package client

import (
	"bufio"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/constants"
	"lsp-gateway/src/internal/platform"
	"lsp-gateway/src/internal/registry"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/capabilities"
	"lsp-gateway/src/server/documents"
	"lsp-gateway/src/server/errors"
	"lsp-gateway/src/server/process"
	"lsp-gateway/src/server/protocol"
	"lsp-gateway/src/utils"
)

type LSPClient struct {
	config              types.ClientConfig
	language            string
	transport           Transport
	stateMgr            *ClientStateManager
	docManager          *documents.DocumentLifecycleManager
	processManager      process.ProcessManager
	processInfo         *process.ProcessInfo
	capabilities        capabilities.ServerCapabilities
	errorTranslator     errors.ErrorTranslator
	capDetector         capabilities.CapabilityDetector
	jsonrpcProtocol     protocol.JSONRPCProtocol
	initializationOptions interface{}
	workspaceFolders    map[string]bool
	workspaceMu         sync.RWMutex
}

func NewStdioClient(config types.ClientConfig, language string, docManager *documents.DocumentLifecycleManager) (types.LSPClient, error) {
	client := &LSPClient{
		config:                config,
		language:              language,
		stateMgr:              NewClientStateManager(),
		docManager:            docManager,
		processManager:        process.NewLSPProcessManager(),
		errorTranslator:       errors.NewLSPErrorTranslator(),
		capDetector:           capabilities.NewLSPCapabilityDetector(),
		jsonrpcProtocol:       protocol.NewLSPJSONRPCProtocol(language),
		initializationOptions: config.InitializationOptions,
		workspaceFolders:      make(map[string]bool),
	}
	return client, nil
}

func NewSocketClient(config types.ClientConfig, language string, addr string, docManager *documents.DocumentLifecycleManager) (types.LSPClient, error) {
	client := &LSPClient{
		config:                config,
		language:              language,
		stateMgr:              NewClientStateManager(),
		docManager:            docManager,
		processManager:        process.NewLSPProcessManager(),
		errorTranslator:       errors.NewLSPErrorTranslator(),
		capDetector:           capabilities.NewLSPCapabilityDetector(),
		jsonrpcProtocol:       protocol.NewLSPJSONRPCProtocol(language),
		initializationOptions: config.InitializationOptions,
		workspaceFolders:      make(map[string]bool),
		transport:             NewSocketTransport(addr),
	}
	return client, nil
}

func (c *LSPClient) Start(ctx context.Context) error {
	if c.stateMgr.IsActive() {
		return fmt.Errorf("client already active")
	}

	processConfig := types.ClientConfig{
		Command:    c.config.Command,
		Args:       c.config.Args,
		WorkingDir: c.config.WorkingDir,
	}

	var err error
	c.processInfo, err = c.processManager.StartProcess(processConfig, c.language)
	if err != nil {
		return fmt.Errorf("failed to start LSP server: %w", err)
	}
	c.processInfo.Active = true

	if c.transport == nil {
		c.transport = NewStdioTransport(c.processInfo)
	}

	if err := c.transport.Connect(ctx); err != nil {
		c.processManager.CleanupProcess(c.processInfo)
		return fmt.Errorf("failed to connect transport: %w", err)
	}

	go func() {
		if err := c.jsonrpcProtocol.HandleResponses(c.transport.Reader(), c, c.processInfo.StopCh); err != nil {
			if !c.processInfo.IntentionalStop && !stderrors.Is(err, io.EOF) {
				common.LSPLogger.Error("Error handling responses for %s: %v", c.language, err)
			}
		}
	}()

	go c.logStderr()

	go c.processManager.MonitorProcess(c.processInfo, func(err error) {
		c.stateMgr.SetActive(false)
		if !c.processInfo.IntentionalStop {
			if err != nil {
				errStr := err.Error()
				if !strings.Contains(errStr, "signal: killed") &&
					!strings.Contains(errStr, "waitid: no child processes") &&
					!strings.Contains(errStr, "process already finished") &&
					!strings.Contains(errStr, "exit status 1") &&
					!strings.Contains(errStr, "exit status 0xc000013a") {
					common.LSPLogger.Error("LSP server process exited with error: language=%s, error=%v", c.language, err)
				}
			}
		}
	})

	if err := c.initializeLSP(ctx); err != nil {
		_ = c.processManager.StopProcess(c.processInfo, c)
		_ = c.transport.Disconnect()
		return fmt.Errorf("failed to initialize LSP server: %w", err)
	}

	c.stateMgr.SetActive(true)
	return nil
}

func (c *LSPClient) Stop() error {
	if !c.stateMgr.IsActive() {
		return nil
	}

	err := c.processManager.StopProcess(c.processInfo, c)
	if err != nil {
		common.LSPLogger.Error("Error stopping process: %v", err)
	}

	_ = c.transport.Disconnect()
	c.stateMgr.SetActive(false)
	return err
}

func (c *LSPClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	if !c.stateMgr.IsActive() && method != types.MethodInitialize {
		return nil, fmt.Errorf("client not active")
	}

	if c.processInfo != nil && c.processInfo.Cmd != nil && c.processInfo.Cmd.Process != nil {
		if processState := c.processInfo.Cmd.ProcessState; processState != nil && processState.Exited() {
			c.stateMgr.SetActive(false)
			return nil, fmt.Errorf("LSP server process has exited")
		}
	}

	c.stateMgr.mu.Lock()
	c.stateMgr.nextID++
	idVal := c.stateMgr.nextID
	id := fmt.Sprintf("%d", idVal)
	c.stateMgr.mu.Unlock()

	request := &pendingRequest{
		respCh: make(chan json.RawMessage, 1),
		done:   make(chan struct{}),
	}

	c.stateMgr.AddPendingRequest(id, request)
	defer func() {
		c.stateMgr.RemovePendingRequest(id)
		close(request.done)
	}()

	msg := protocol.CreateMessage(method, idVal, params)

	c.stateMgr.LockWrite()
	writeErr := c.jsonrpcProtocol.WriteMessage(c.transport.Writer(), msg)
	c.stateMgr.UnlockWrite()

	if writeErr != nil {
		isConnectionError := false

		if stderrors.Is(writeErr, syscall.EPIPE) || stderrors.Is(writeErr, io.ErrClosedPipe) {
			isConnectionError = true
		}

		var opErr *net.OpError
		if stderrors.As(writeErr, &opErr) {
			var syscallErr *os.SyscallError
			if stderrors.As(opErr.Err, &syscallErr) {
				if stderrors.Is(syscallErr.Err, syscall.ECONNRESET) {
					isConnectionError = true
				}
			}
		}

		if stderrors.Is(writeErr, io.EOF) {
			isConnectionError = true
		}

		if isConnectionError {
			c.stateMgr.SetActive(false)
			common.LSPLogger.Warn("LSP client connection lost, marking as inactive: method=%s, id=%s, error=%v", method, id, writeErr)
		}
		common.LSPLogger.Error("Failed to send LSP request: method=%s, id=%s, error=%v", method, id, writeErr)
		return nil, fmt.Errorf("failed to send request: %w", writeErr)
	}

	timeoutDuration := c.getRequestTimeout(method)
	if method == types.MethodInitialize {
		timeoutDuration = c.getInitializeTimeout()
	}

	var cancel context.CancelFunc
	if deadline, ok := ctx.Deadline(); ok {
		remainingTime := time.Until(deadline)
		if remainingTime < timeoutDuration {
			cancel = func() {}
			common.LSPLogger.Debug("Using existing context deadline (%v) for %s request %s", remainingTime, method, id)
		} else {
			ctx, cancel = common.WithTimeout(ctx, timeoutDuration)
		}
	} else {
		ctx, cancel = common.WithTimeout(ctx, timeoutDuration)
	}
	defer cancel()

	select {
	case response := <-request.respCh:
		return response, nil
	case <-ctx.Done():
		c.stateMgr.AddRecentTimeout(id)
		c.stateMgr.CleanupOldTimeouts()

		cancelParams := map[string]interface{}{"id": idVal}
		if cancelErr := c.SendNotification(context.Background(), "$/cancelRequest", cancelParams); cancelErr != nil {
			common.LSPLogger.Debug("Failed to send cancel request for id=%s: %v", id, cancelErr)
		}

		common.LSPLogger.Error("LSP request timeout: method=%s, id=%s, timeout=%v", method, id, timeoutDuration)
		return nil, fmt.Errorf("request timeout after %v for method %s", timeoutDuration, method)
	case <-c.processInfo.StopCh:
		if method == "shutdown" || c.processInfo.IntentionalStop {
			common.LSPLogger.Debug("LSP client stopped during request: method=%s, id=%s", method, id)
		} else {
			common.LSPLogger.Warn("LSP client stopped during request: method=%s, id=%s", method, id)
		}
		return nil, fmt.Errorf("client stopped")
	}
}

func (c *LSPClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	if !c.stateMgr.IsActive() && method != "initialized" {
		return fmt.Errorf("client not active")
	}

	msg := protocol.CreateNotification(method, params)

	c.stateMgr.LockWrite()
	defer c.stateMgr.UnlockWrite()
	return c.jsonrpcProtocol.WriteMessage(c.transport.Writer(), msg)
}

func (c *LSPClient) IsActive() bool {
	return c.stateMgr.IsActive()
}

func (c *LSPClient) Supports(method string) bool {
	c.stateMgr.mu.RLock()
	defer c.stateMgr.mu.RUnlock()
	return c.capDetector.SupportsMethod(c.capabilities, method)
}

func (c *LSPClient) SendShutdownRequest(ctx context.Context) error {
	_, err := c.SendRequest(ctx, "shutdown", nil)
	return err
}

func (c *LSPClient) SendExitNotification(ctx context.Context) error {
	return c.SendNotification(ctx, "exit", nil)
}

func (c *LSPClient) HandleRequest(method string, id interface{}, params interface{}) error {
	if method == "workspace/configuration" {
		response := protocol.CreateResponse(id, []interface{}{map[string]interface{}{}}, nil)
		c.stateMgr.LockWrite()
		defer c.stateMgr.UnlockWrite()
		return c.jsonrpcProtocol.WriteMessage(c.transport.Writer(), response)
	} else {
		var nullResult = json.RawMessage("null")
		response := protocol.CreateResponse(id, nullResult, nil)
		c.stateMgr.LockWrite()
		defer c.stateMgr.UnlockWrite()
		return c.jsonrpcProtocol.WriteMessage(c.transport.Writer(), response)
	}
}

func (c *LSPClient) HandleResponse(id interface{}, result json.RawMessage, err *protocol.RPCError) error {
	idStr := fmt.Sprintf("%v", id)

	req, exists := c.stateMgr.GetPendingRequest(idStr)
	if exists {
		var responseData json.RawMessage
		if err != nil {
			errorData, _ := json.Marshal(err)
			responseData = errorData
			if !protocol.IsExpectedSuppressibleError(err) {
				sanitizedError := common.SanitizeErrorForLogging(err)
				common.LSPLogger.Warn("LSP response contains error: id=%s, error=%s", idStr, sanitizedError)
			}
		} else {
			responseData = result
		}
		select {
		case req.respCh <- responseData:
		case <-req.done:
			common.LSPLogger.Warn("Request already completed when trying to deliver response: id=%s", idStr)
		case <-c.processInfo.StopCh:
			common.LSPLogger.Warn("Client stopped when trying to deliver response: id=%s", idStr)
		}
		return nil
	}

	if c.stateMgr.IsRecentTimeout(idStr) {
		common.LSPLogger.Debug("Received late response for previously timed-out request: id=%s", idStr)
	} else {
		common.LSPLogger.Warn("No matching request found for response: id=%s", idStr)
	}
	return nil
}

func (c *LSPClient) HandleNotification(method string, params interface{}) error {
	return nil
}

func (c *LSPClient) initializeLSP(ctx context.Context) error {
	var wd string
	if c.config.WorkingDir != "" {
		wd = c.config.WorkingDir
	} else {
		var err error
		wd, err = os.Getwd()
		if err != nil {
			if platform.IsWindows() {
				wd = "C:\\temp"
			} else {
				wd = "/tmp"
			}
		}
	}

	wd, _ = filepath.Abs(wd)
	if platform.IsWindows() {
		wd = utils.URIToFilePathCached(utils.FilePathToURI(wd))
	}

	initOptions := c.getInitializationOptions()

	if c.language == "rust" {
		if optionsJSON, err := json.Marshal(initOptions); err == nil {
			common.LSPLogger.Info("rust-analyzer initialization options: %s", string(optionsJSON))
		}
	}

	initParams := map[string]interface{}{
		"processId": os.Getpid(),
		"clientInfo": map[string]interface{}{
			"name":    "lsp-gateway",
			"version": "1.0.0",
		},
		"rootUri":  utils.FilePathToURI(wd),
		"rootPath": wd,
		"workspaceFolders": []map[string]interface{}{
			{
				"uri":  utils.FilePathToURI(wd),
				"name": filepath.Base(wd),
			},
		},
		"initializationOptions": initOptions,
		"capabilities": map[string]interface{}{
			"workspace": map[string]interface{}{
				"applyEdit":              true,
				"workspaceEdit":          map[string]interface{}{"documentChanges": true},
				"didChangeConfiguration": map[string]interface{}{"dynamicRegistration": true},
				"didChangeWatchedFiles":  map[string]interface{}{"dynamicRegistration": true},
				"symbol":                 map[string]interface{}{"dynamicRegistration": true},
				"executeCommand":         map[string]interface{}{"dynamicRegistration": true},
				"configuration":          true,
				"workspaceFolders":       true,
			},
			"textDocument": map[string]interface{}{
				"publishDiagnostics": map[string]interface{}{
					"relatedInformation": true,
					"versionSupport":     false,
					"tagSupport":         map[string]interface{}{"valueSet": []int{1, 2}},
				},
				"synchronization": map[string]interface{}{
					"dynamicRegistration": true,
					"willSave":            true,
					"willSaveWaitUntil":   true,
					"didSave":             true,
				},
				"completion": map[string]interface{}{
					"dynamicRegistration": true,
					"contextSupport":      true,
					"completionItem": map[string]interface{}{
						"snippetSupport":          true,
						"commitCharactersSupport": true,
						"documentationFormat":     []string{"markdown", "plaintext"},
						"preselectSupport":        true,
					},
					"completionItemKind": map[string]interface{}{
						"valueSet": []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
					},
				},
				"hover": map[string]interface{}{
					"dynamicRegistration": true,
					"contentFormat":       []string{"markdown", "plaintext"},
				},
				"signatureHelp": map[string]interface{}{
					"dynamicRegistration": true,
					"signatureInformation": map[string]interface{}{
						"documentationFormat": []string{"markdown", "plaintext"},
					},
				},
				"definition": map[string]interface{}{
					"dynamicRegistration": true,
					"linkSupport":         true,
				},
				"references": map[string]interface{}{
					"dynamicRegistration": true,
				},
				"documentHighlight": map[string]interface{}{
					"dynamicRegistration": true,
				},
				"documentSymbol": map[string]interface{}{
					"dynamicRegistration":               true,
					"hierarchicalDocumentSymbolSupport": true,
					"symbolKind": map[string]interface{}{
						"valueSet": []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26},
					},
				},
				"codeAction": map[string]interface{}{
					"dynamicRegistration": true,
					"codeActionLiteralSupport": map[string]interface{}{
						"codeActionKind": map[string]interface{}{
							"valueSet": []string{"", "quickfix", "refactor", "refactor.extract", "refactor.inline", "refactor.rewrite", "source", "source.organizeImports"},
						},
					},
				},
				"formatting": map[string]interface{}{
					"dynamicRegistration": true,
				},
				"rangeFormatting": map[string]interface{}{
					"dynamicRegistration": true,
				},
				"onTypeFormatting": map[string]interface{}{
					"dynamicRegistration": true,
				},
				"rename": map[string]interface{}{
					"dynamicRegistration": true,
					"prepareSupport":      true,
				},
			},
		},
		"trace": "off",
	}

	result, err := c.SendRequest(ctx, types.MethodInitialize, initParams)
	if err != nil {
		return err
	}

	if err := c.parseServerCapabilities(result); err != nil {
		common.LSPLogger.Warn("Failed to parse server capabilities for %s: %v", c.config.Command, err)
	}

	if c.language == "rust" {
		common.LSPLogger.Info("rust-analyzer capabilities: %s", string(result))
	}

	if err := c.SendNotification(ctx, "initialized", map[string]interface{}{}); err != nil {
		common.LSPLogger.Error("Failed to send initialized notification for %s: %v", c.language, err)
		return err
	}

	isPyright := c.config.Command == "pyright-langserver" || (c.config.Command == "uvx" && len(c.config.Args) > 0 && c.config.Args[0] == "pyright-langserver")
	isBasedPyright := c.config.Command == "basedpyright-langserver" || (c.config.Command == "uvx" && len(c.config.Args) > 0 && c.config.Args[0] == "basedpyright-langserver")
	if isPyright || isBasedPyright {
		c.stateMgr.SetActive(true)

		serverName := "pyright"
		configPrefix := "python"
		if isBasedPyright {
			serverName = "basedpyright"
			configPrefix = "basedpyright"
		}

		common.LSPLogger.Debug("Sending workspace/didChangeConfiguration for %s", serverName)
		configParams := map[string]interface{}{
			"settings": map[string]interface{}{
				configPrefix: map[string]interface{}{
					"analysis": map[string]interface{}{
						"autoImportCompletions":  true,
						"autoSearchPaths":        true,
						"diagnosticMode":         "openFilesOnly",
						"typeCheckingMode":       "basic",
						"useLibraryCodeForTypes": true,
					},
				},
			},
		}
		if err := c.SendNotification(ctx, "workspace/didChangeConfiguration", configParams); err != nil {
			common.LSPLogger.Warn("Failed to send workspace/didChangeConfiguration for %s: %v", serverName, err)
		}

		c.stateMgr.SetActive(false)
	}

	return nil
}

func (c *LSPClient) getRequestTimeout(method string) time.Duration {
	baseTimeout := constants.GetRequestTimeout(c.language)

	if c.language == "java" {
		switch method {
		case types.MethodTextDocumentReferences:
			return baseTimeout * 2
		case types.MethodWorkspaceSymbol:
			return time.Duration(float64(baseTimeout) * 1.5)
		}
	}

	return baseTimeout
}

func (c *LSPClient) getInitializeTimeout() time.Duration {
	return constants.GetInitializeTimeout(c.language)
}

func (c *LSPClient) getInitializationOptions() map[string]interface{} {
	if c.initializationOptions != nil {
		switch opts := c.initializationOptions.(type) {
		case map[string]interface{}:
			return convertToStringMap(opts)
		case map[interface{}]interface{}:
			return convertInterfaceMap(opts)
		}
	}

	langInfo, exists := registry.GetLanguageByName(c.language)
	if !exists {
		common.LSPLogger.Warn("Unknown language %s, using default initialization options", c.language)
		return map[string]interface{}{
			"usePlaceholders":    false,
			"completeUnimported": true,
		}
	}
	return langInfo.GetInitOptions()
}

func (c *LSPClient) parseServerCapabilities(result json.RawMessage) error {
	caps, err := c.capDetector.ParseCapabilities(result, c.config.Command)
	if err != nil {
		return err
	}

	c.stateMgr.mu.Lock()
	c.capabilities = caps
	c.stateMgr.mu.Unlock()

	return nil
}

func (c *LSPClient) logStderr() {
	if c.processInfo == nil || c.processInfo.Stderr == nil {
		return
	}

	scanner := bufio.NewScanner(c.processInfo.Stderr)
	var errorContext []string

	for scanner.Scan() {
		select {
		case <-c.processInfo.StopCh:
			return
		default:
			line := scanner.Text()

			if strings.Contains(line, "Traceback") {
				errorContext = []string{line}
				continue
			}

			if len(errorContext) > 0 && (strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t")) {
				errorContext = append(errorContext, line)
				continue
			}

			if c.errorTranslator.TranslateAndLogError(c.config.Command, line, errorContext) {
				errorContext = nil
				continue
			}

			if strings.Contains(line, "error") || strings.Contains(line, "Error") ||
				strings.Contains(line, "fatal") || strings.Contains(line, "Fatal") ||
				strings.Contains(line, "Exception") {
				common.LSPLogger.Error("LSP %s stderr ERROR: %s", c.config.Command, line)
			}

			errorContext = nil
		}
	}
}

func convertInterfaceMap(m map[interface{}]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		if key, ok := k.(string); ok {
			result[key] = convertValue(v)
		}
	}
	return result
}

func convertToStringMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		result[k] = convertValue(v)
	}
	return result
}

func convertValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[interface{}]interface{}:
		return convertInterfaceMap(val)
	case map[string]interface{}:
		return convertToStringMap(val)
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, item := range val {
			result[i] = convertValue(item)
		}
		return result
	default:
		return v
	}
}
