package server

import (
	"bufio"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/constants"
	"lsp-gateway/src/internal/registry"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/capabilities"
	"lsp-gateway/src/server/errors"
	"lsp-gateway/src/server/process"
	"lsp-gateway/src/server/protocol"
)

type SocketClient struct {
	config                types.ClientConfig
	language              string
	capabilities          capabilities.ServerCapabilities
	errorTranslator       errors.ErrorTranslator
	capDetector           capabilities.CapabilityDetector
	processManager        process.ProcessManager
	processInfo           *process.ProcessInfo
	jsonrpcProtocol       protocol.JSONRPCProtocol
	initializationOptions interface{}

	addr string
	conn net.Conn

	mu               sync.RWMutex
	writeMu          sync.Mutex
	active           bool
	requests         map[string]*pendingRequest
	nextID           int
	openDocs         map[string]bool
	recentTimeouts   map[string]time.Time
	timeoutsMu       sync.RWMutex
	workspaceFolders map[string]bool
}

func NewSocketClient(config types.ClientConfig, language, addr string) (types.LSPClient, error) {
	c := &SocketClient{
		config:                config,
		language:              language,
		addr:                  addr,
		requests:              make(map[string]*pendingRequest),
		openDocs:              make(map[string]bool),
		workspaceFolders:      make(map[string]bool),
		recentTimeouts:        make(map[string]time.Time),
		errorTranslator:       errors.NewLSPErrorTranslator(),
		capDetector:           capabilities.NewLSPCapabilityDetector(),
		processManager:        process.NewLSPProcessManager(),
		jsonrpcProtocol:       protocol.NewLSPJSONRPCProtocol(language),
		initializationOptions: config.InitializationOptions,
	}
	return c, nil
}

func (c *SocketClient) getRequestTimeout(method string) time.Duration {
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

func (c *SocketClient) getInitializeTimeout() time.Duration {
	return constants.GetInitializeTimeout(c.language)
}

func (c *SocketClient) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.active {
		c.mu.Unlock()
		return fmt.Errorf("client already active")
	}
	c.mu.Unlock()

	procCfg := types.ClientConfig{
		Command:    c.config.Command,
		Args:       c.config.Args,
		WorkingDir: c.config.WorkingDir,
	}

	var err error
	c.processInfo, err = c.processManager.StartProcess(procCfg, c.language)
	if err != nil {
		return fmt.Errorf("failed to start LSP server: %w", err)
	}
	c.processInfo.Active = true

	go c.processManager.MonitorProcess(c.processInfo, func(err error) {
		c.mu.Lock()
		c.active = false
		c.mu.Unlock()
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

	go c.logStderr()

	// Kotlin on Windows needs time to initialize before opening socket
	if c.language == "kotlin" && runtime.GOOS == "windows" {
		initialDelay := 5 * time.Second
		if common.IsCI() {
			initialDelay = 10 * time.Second
		}
		common.LSPLogger.Info("Waiting %v for Kotlin LSP to initialize before connecting...", initialDelay)
		time.Sleep(initialDelay)
	}

	// Kotlin needs more time to start, similar to Java
	timeout := constants.ProcessStartTimeout
	if c.language == "kotlin" || c.language == "java" {
		timeout = 90 * time.Second
		if common.IsCI() {
			if runtime.GOOS == "windows" {
				timeout = time.Duration(float64(timeout) * 3.0) // Windows CI: 3x multiplier for JVM languages
			} else {
				timeout = time.Duration(float64(timeout) * 1.2) // Regular CI: 1.2x multiplier
			}
		}
	}

	dialCtx, cancel := common.WithTimeout(ctx, timeout)
	defer cancel()

	var conn net.Conn
	var lastErr error
	retryInterval := 200 * time.Millisecond
	if c.language == "kotlin" {
		retryInterval = 500 * time.Millisecond // Slower retry for Kotlin
	}

	for {
		if dialCtx.Err() != nil {
			break
		}
		conn, lastErr = net.DialTimeout("tcp", c.addr, 2*time.Second)
		if lastErr == nil {
			break
		}
		time.Sleep(retryInterval)
	}
	if lastErr != nil {
		c.processManager.CleanupProcess(c.processInfo)
		return fmt.Errorf("failed to connect to server socket %s: %w", c.addr, lastErr)
	}
	c.conn = conn

	go func() {
		if err := c.jsonrpcProtocol.HandleResponses(c.conn, c, c.processInfo.StopCh); err != nil {
			if !c.processInfo.IntentionalStop && err != io.EOF {
				common.LSPLogger.Error("Error handling responses for %s: %v", c.language, err)
			}
		}
	}()

	if err := c.initializeLSP(ctx); err != nil {
		c.processManager.CleanupProcess(c.processInfo)
		c.conn.Close()
		return fmt.Errorf("failed to initialize LSP server: %w", err)
	}

	c.mu.Lock()
	c.active = true
	c.mu.Unlock()
	return nil
}

func (c *SocketClient) Stop() error {
	c.mu.Lock()
	if !c.active {
		c.mu.Unlock()
		if c.conn != nil {
			c.conn.Close()
		}
		return nil
	}
	c.mu.Unlock()

	err := c.processManager.StopProcess(c.processInfo, c)
	if c.conn != nil {
		c.conn.Close()
	}
	c.mu.Lock()
	c.active = false
	c.mu.Unlock()
	return err
}

func (c *SocketClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	c.mu.RLock()
	active := c.active
	processInfo := c.processInfo
	c.mu.RUnlock()
	if !active && method != types.MethodInitialize {
		return nil, fmt.Errorf("client not active")
	}
	if processInfo != nil && processInfo.Cmd != nil && processInfo.Cmd.Process != nil {
		if processState := processInfo.Cmd.ProcessState; processState != nil && processState.Exited() {
			c.mu.Lock()
			c.active = false
			c.mu.Unlock()
			return nil, fmt.Errorf("LSP server process has exited")
		}
	}

	c.mu.Lock()
	c.nextID++
	idVal := c.nextID
	id := fmt.Sprintf("%d", idVal)
	c.mu.Unlock()

	request := &pendingRequest{respCh: make(chan json.RawMessage, 1), done: make(chan struct{})}
	c.mu.Lock()
	c.requests[id] = request
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.requests, id)
		c.mu.Unlock()
		close(request.done)
	}()

	msg := protocol.CreateMessage(method, idVal, params)
	c.writeMu.Lock()
	writeErr := c.jsonrpcProtocol.WriteMessage(c.conn, msg)
	c.writeMu.Unlock()
	if writeErr != nil {
		isConnectionError := false
		if stderrors.Is(writeErr, syscall.EPIPE) || stderrors.Is(writeErr, io.ErrClosedPipe) {
			isConnectionError = true
		}
		var opErr *net.OpError
		if stderrors.As(writeErr, &opErr) {
			if runtime.GOOS == "windows" {
				isConnectionError = true
			}
			if opErr.Timeout() {
				isConnectionError = true
			}
		}
		if stderrors.Is(writeErr, io.EOF) {
			isConnectionError = true
		}
		if isConnectionError {
			c.mu.Lock()
			c.active = false
			c.mu.Unlock()
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
		remaining := time.Until(deadline)
		if remaining < timeoutDuration {
			cancel = func() {}
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
		c.timeoutsMu.Lock()
		c.recentTimeouts[id] = time.Now()
		for reqID, t := range c.recentTimeouts {
			if time.Since(t) > 30*time.Second {
				delete(c.recentTimeouts, reqID)
			}
		}
		c.timeoutsMu.Unlock()
		cancelParams := map[string]interface{}{"id": idVal}
		if cancelErr := c.SendNotification(context.Background(), "$/cancelRequest", cancelParams); cancelErr != nil {
			common.LSPLogger.Debug("Failed to send cancel request for id=%s: %v", id, cancelErr)
		}
		common.LSPLogger.Error("LSP request timeout: method=%s, id=%s, timeout=%v", method, id, timeoutDuration)
		return nil, fmt.Errorf("request timeout after %v for method %s", timeoutDuration, method)
	case <-processInfo.StopCh:
		if method == "shutdown" || processInfo.IntentionalStop {
			common.LSPLogger.Debug("LSP client stopped during request: method=%s, id=%s", method, id)
		} else {
			common.LSPLogger.Warn("LSP client stopped during request: method=%s, id=%s", method, id)
		}
		return nil, fmt.Errorf("client stopped")
	}
}

func (c *SocketClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	c.mu.RLock()
	if !c.active && method != "initialized" {
		c.mu.RUnlock()
		return fmt.Errorf("client not active")
	}
	c.mu.RUnlock()
	msg := protocol.CreateNotification(method, params)
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.jsonrpcProtocol.WriteMessage(c.conn, msg)
}

func (c *SocketClient) IsActive() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.active
}

func (c *SocketClient) Supports(method string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.capDetector.SupportsMethod(c.capabilities, method)
}

func (c *SocketClient) SendShutdownRequest(ctx context.Context) error {
	_, err := c.SendRequest(ctx, "shutdown", nil)
	return err
}

func (c *SocketClient) SendExitNotification(ctx context.Context) error {
	return c.SendNotification(ctx, "exit", nil)
}

func (c *SocketClient) HandleRequest(method string, id interface{}, params interface{}) error {
	if method == "workspace/configuration" {
		response := protocol.CreateResponse(id, []interface{}{map[string]interface{}{}}, nil)
		c.writeMu.Lock()
		defer c.writeMu.Unlock()
		return c.jsonrpcProtocol.WriteMessage(c.conn, response)
	} else {
		var nullResult json.RawMessage = json.RawMessage("null")
		response := protocol.CreateResponse(id, nullResult, nil)
		c.writeMu.Lock()
		defer c.writeMu.Unlock()
		return c.jsonrpcProtocol.WriteMessage(c.conn, response)
	}
}

func (c *SocketClient) HandleResponse(id interface{}, result json.RawMessage, err *protocol.RPCError) error {
	idStr := fmt.Sprintf("%v", id)
	c.mu.RLock()
	req, exists := c.requests[idStr]
	processInfo := c.processInfo
	c.mu.RUnlock()
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
		case <-processInfo.StopCh:
			common.LSPLogger.Warn("Client stopped when trying to deliver response: id=%s", idStr)
		}
	} else {
		c.timeoutsMu.RLock()
		timeoutTime, wasTimeout := c.recentTimeouts[idStr]
		c.timeoutsMu.RUnlock()
		if wasTimeout && time.Since(timeoutTime) < 30*time.Second {
			common.LSPLogger.Debug("Received late response for previously timed-out request: id=%s (timed out %v ago)", idStr, time.Since(timeoutTime))
		} else {
			common.LSPLogger.Warn("No matching request found for response: id=%s", idStr)
		}
	}
	return nil
}

func (c *SocketClient) HandleNotification(method string, params interface{}) error { return nil }

func (c *SocketClient) initializeLSP(ctx context.Context) error {
	var wd string
	if c.config.WorkingDir != "" {
		wd = c.config.WorkingDir
	} else {
		var err error
		wd, err = os.Getwd()
		if err != nil {
			if runtime.GOOS == "windows" {
				wd = os.TempDir()
			} else {
				wd = "/tmp"
			}
		}
	}

	initParams := map[string]interface{}{
		"processId": os.Getpid(),
		"rootUri":   nil,
		"rootPath":  nil,
		"workspaceFolders": []interface{}{
			map[string]interface{}{"uri": "file://" + wd, "name": filepathBase(wd)},
		},
		"capabilities": map[string]interface{}{
			"workspace": map[string]interface{}{
				"applyEdit": true,
				"workspaceEdit": map[string]interface{}{
					"documentChanges":    true,
					"resourceOperations": []string{"create", "rename", "delete"},
				},
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

	if opts := c.getInitializationOptions(); opts != nil {
		initParams["initializationOptions"] = opts
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
		c.active = true
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
		c.active = false
	}
	return nil
}

func (c *SocketClient) getInitializationOptions() map[string]interface{} {
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
		return map[string]interface{}{
			"usePlaceholders":    false,
			"completeUnimported": true,
		}
	}
	return langInfo.GetInitOptions()
}

func (c *SocketClient) parseServerCapabilities(result json.RawMessage) error {
	caps, err := c.capDetector.ParseCapabilities(result, c.config.Command)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.capabilities = caps
	c.mu.Unlock()
	return nil
}

func (c *SocketClient) logStderr() {
	c.mu.RLock()
	processInfo := c.processInfo
	c.mu.RUnlock()
	if processInfo == nil || processInfo.Stderr == nil {
		return
	}
	scanner := bufio.NewScanner(processInfo.Stderr)
	var errorContext []string
	for scanner.Scan() {
		select {
		case <-processInfo.StopCh:
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

func filepathBase(p string) string {
	i := strings.LastIndex(p, "/")
	j := strings.LastIndex(p, "\\")
	if j > i {
		i = j
	}
	if i >= 0 && i+1 < len(p) {
		return p[i+1:]
	}
	return p
}
