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
	"path/filepath"
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
	"lsp-gateway/src/utils"
)

// getRequestTimeout returns language-specific timeout for LSP requests
func (c *StdioClient) getRequestTimeout(method string) time.Duration {
	baseTimeout := constants.GetRequestTimeout(c.language)

	// Method-specific overrides for Java
	if c.language == "java" {
		switch method {
		case types.MethodTextDocumentReferences:
			// Java references can be particularly slow due to project-wide search
			// On Windows CI, this becomes 270s * 2 = 540s (9 minutes)
			return baseTimeout * 2
		case types.MethodWorkspaceSymbol:
			// Workspace symbol search can also be slow for large Java projects
			return time.Duration(float64(baseTimeout) * 1.5)
		}
	}

	return baseTimeout
}

// getInitializeTimeout returns language-specific timeout for initialize requests
func (c *StdioClient) getInitializeTimeout() time.Duration {
	return constants.GetInitializeTimeout(c.language)
}

// pendingRequest stores context for pending LSP requests
type pendingRequest struct {
	respCh chan json.RawMessage
	done   chan struct{}
}

// StdioClient implements LSP communication over STDIO
type StdioClient struct {
	config                types.ClientConfig
	language              string // Language identifier for unique request IDs
	capabilities          capabilities.ServerCapabilities
	errorTranslator       errors.ErrorTranslator
	capDetector           capabilities.CapabilityDetector
	processManager        process.ProcessManager
	processInfo           *process.ProcessInfo
	jsonrpcProtocol       protocol.JSONRPCProtocol
	initializationOptions interface{} // Initialization options from config

	mu               sync.RWMutex
	writeMu          sync.Mutex
	active           bool
	requests         map[string]*pendingRequest
	nextID           int
	openDocs         map[string]bool      // Track opened documents to prevent duplicate didOpen
	recentTimeouts   map[string]time.Time // Track recently timed-out requests
	timeoutsMu       sync.RWMutex
	workspaceFolders map[string]bool // Track added workspace folders
}

// NewStdioClient creates a new STDIO LSP client
func NewStdioClient(config types.ClientConfig, language string) (types.LSPClient, error) {
	client := &StdioClient{
		config:                config,
		language:              language,
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
	return client, nil
}

// Start initializes and starts the LSP server process
func (c *StdioClient) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.active {
		c.mu.Unlock()
		return fmt.Errorf("client already active")
	}
	c.mu.Unlock()

	// Use shared ClientConfig type
	processConfig := types.ClientConfig{
		Command:    c.config.Command,
		Args:       c.config.Args,
		WorkingDir: c.config.WorkingDir,
	}

	// Start process using process manager
	var err error
	c.processInfo, err = c.processManager.StartProcess(processConfig, c.language)
	if err != nil {
		return fmt.Errorf("failed to start LSP server: %w", err)
	}

	// Mark process as active
	c.processInfo.Active = true

	// Start response handler using protocol module
	go func() {
		if err := c.jsonrpcProtocol.HandleResponses(c.processInfo.Stdout, c, c.processInfo.StopCh); err != nil {
			// Only log errors if this wasn't an intentional stop
			if !c.processInfo.IntentionalStop && err != io.EOF {
				common.LSPLogger.Error("Error handling responses for %s: %v", c.language, err)
			}
		}
	}()

	// Start stderr logger
	go c.logStderr()

	// Start process monitor using process manager
	go c.processManager.MonitorProcess(c.processInfo, func(err error) {
		// Mark as inactive when process exits
		c.mu.Lock()
		c.active = false
		c.mu.Unlock()

		// Log process exit for debugging EOF issues
		// Only log errors if this wasn't an intentional stop
		if !c.processInfo.IntentionalStop {
			if err != nil {
				// Filter out common shutdown errors
				errStr := err.Error()
				if !strings.Contains(errStr, "signal: killed") &&
					!strings.Contains(errStr, "waitid: no child processes") &&
					!strings.Contains(errStr, "process already finished") &&
					!strings.Contains(errStr, "exit status 1") && // Common on Windows
					!strings.Contains(errStr, "exit status 0xc000013a") { // Windows CTRL_C_EVENT
					common.LSPLogger.Error("LSP server process exited with error: language=%s, error=%v", c.language, err)
				}
			}
		}
	})

	// Initialize LSP server
	if err := c.initializeLSP(ctx); err != nil {
		c.processManager.CleanupProcess(c.processInfo)
		return fmt.Errorf("failed to initialize LSP server: %w", err)
	}

	// Mark as active after successful initialization
	c.mu.Lock()
	c.active = true
	c.mu.Unlock()

	return nil
}

// Stop terminates the LSP server process
func (c *StdioClient) Stop() error {
	c.mu.Lock()
	if !c.active {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	// Use process manager to stop the process
	err := c.processManager.StopProcess(c.processInfo, c)
	if err != nil {
		common.LSPLogger.Error("Error stopping process: %v", err)
	}

	// Mark as inactive
	c.mu.Lock()
	c.active = false
	c.mu.Unlock()

	return err
}

// SendRequest sends a JSON-RPC request and waits for response
func (c *StdioClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	c.mu.RLock()
	active := c.active
	processInfo := c.processInfo
	c.mu.RUnlock()

	// Check if client is active and process is still running
	if !active && method != types.MethodInitialize {
		return nil, fmt.Errorf("client not active")
	}

	// Additional check: verify process is still alive
	if processInfo != nil && processInfo.Cmd != nil && processInfo.Cmd.Process != nil {
		if processState := processInfo.Cmd.ProcessState; processState != nil && processState.Exited() {
			c.mu.Lock()
			c.active = false
			c.mu.Unlock()
			return nil, fmt.Errorf("LSP server process has exited")
		}
	}

	// Generate numeric request ID (per-client) to maximize compatibility
	c.mu.Lock()
	c.nextID++
	idVal := c.nextID
	id := fmt.Sprintf("%d", idVal)
	c.mu.Unlock()

	// Create request
	request := &pendingRequest{
		respCh: make(chan json.RawMessage, 1),
		done:   make(chan struct{}),
	}

	// Store request
	c.mu.Lock()
	c.requests[id] = request
	c.mu.Unlock()

	// Cleanup on exit
	defer func() {
		c.mu.Lock()
		delete(c.requests, id)
		c.mu.Unlock()
		close(request.done)
	}()

	// Create and send message using protocol module
	// Use numeric ID in wire protocol
	msg := protocol.CreateMessage(method, idVal, params)

	c.writeMu.Lock()
	writeErr := c.jsonrpcProtocol.WriteMessage(c.processInfo.Stdin, msg)
	c.writeMu.Unlock()
	if writeErr != nil {
		// If we get connection errors, mark client as inactive
		isConnectionError := false

		// Check for broken pipe
		if stderrors.Is(writeErr, syscall.EPIPE) || stderrors.Is(writeErr, io.ErrClosedPipe) {
			isConnectionError = true
		}

		// Check for connection reset errors
		var opErr *net.OpError
		if stderrors.As(writeErr, &opErr) {
			if syscallErr, ok := opErr.Err.(*os.SyscallError); ok {
				if syscallErr.Err == syscall.ECONNRESET {
					isConnectionError = true
				}
			}
		}

		// Check for EOF errors
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

	// Wait for response with appropriate timeout
	timeoutDuration := c.getRequestTimeout(method)
	if method == types.MethodInitialize {
		// Use longer timeout for initialize - LSP servers can be slow to start
		timeoutDuration = c.getInitializeTimeout()
	}

	// Respect existing context deadline if it's shorter than our timeout
	var cancel context.CancelFunc
	if deadline, ok := ctx.Deadline(); ok {
		// Context already has a deadline, use the shorter timeout
		remainingTime := time.Until(deadline)
		if remainingTime < timeoutDuration {
			// Use existing deadline as is
			cancel = func() {} // No-op since we're not creating a new context
			common.LSPLogger.Debug("Using existing context deadline (%v) for %s request %s", remainingTime, method, id)
		} else {
			// Our timeout is shorter, create new context with our timeout
			ctx, cancel = common.WithTimeout(ctx, timeoutDuration)
		}
	} else {
		// No existing deadline, use our timeout
		ctx, cancel = common.WithTimeout(ctx, timeoutDuration)
	}
	defer cancel()

	select {
	case response := <-request.respCh:
		return response, nil
	case <-ctx.Done():
		// Track this as a recently timed-out request
		c.timeoutsMu.Lock()
		c.recentTimeouts[id] = time.Now()
		// Clean up old timeout entries (older than 30 seconds)
		for reqID, timeoutTime := range c.recentTimeouts {
			if time.Since(timeoutTime) > 30*time.Second {
				delete(c.recentTimeouts, reqID)
			}
		}
		c.timeoutsMu.Unlock()

		// Send cancellation request to LSP server
		cancelParams := map[string]interface{}{"id": idVal}
		if cancelErr := c.SendNotification(context.Background(), "$/cancelRequest", cancelParams); cancelErr != nil {
			common.LSPLogger.Debug("Failed to send cancel request for id=%s: %v", id, cancelErr)
		}

		common.LSPLogger.Error("LSP request timeout: method=%s, id=%s, timeout=%v", method, id, timeoutDuration)
		return nil, fmt.Errorf("request timeout after %v for method %s", timeoutDuration, method)
	case <-processInfo.StopCh:
		// During shutdown, this is expected behavior
		if method == "shutdown" || processInfo.IntentionalStop {
			common.LSPLogger.Debug("LSP client stopped during request: method=%s, id=%s", method, id)
		} else {
			common.LSPLogger.Warn("LSP client stopped during request: method=%s, id=%s", method, id)
		}
		return nil, fmt.Errorf("client stopped")
	}
}

// SendNotification sends a JSON-RPC notification (no response expected)
func (c *StdioClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	c.mu.RLock()
	if !c.active && method != "initialized" {
		c.mu.RUnlock()
		return fmt.Errorf("client not active")
	}
	c.mu.RUnlock()

	msg := protocol.CreateNotification(method, params)

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.jsonrpcProtocol.WriteMessage(c.processInfo.Stdin, msg)
}

// IsActive returns true if the client is active
func (c *StdioClient) IsActive() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.active
}

// Supports checks if the LSP server supports a specific method
func (c *StdioClient) Supports(method string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.capDetector.SupportsMethod(c.capabilities, method)
}

// SendShutdownRequest sends a shutdown request to the LSP server (ShutdownSender interface)
func (c *StdioClient) SendShutdownRequest(ctx context.Context) error {
	_, err := c.SendRequest(ctx, "shutdown", nil)
	return err
}

// SendExitNotification sends an exit notification to the LSP server (ShutdownSender interface)
func (c *StdioClient) SendExitNotification(ctx context.Context) error {
	return c.SendNotification(ctx, "exit", nil)
}

// HandleRequest implements the MessageHandler interface for server-initiated requests
func (c *StdioClient) HandleRequest(method string, id interface{}, params interface{}) error {

	// Handle specific server requests
	if method == "workspace/configuration" {
		// Respond with empty configuration
		response := protocol.CreateResponse(id, []interface{}{map[string]interface{}{}}, nil)
		c.writeMu.Lock()
		defer c.writeMu.Unlock()
		return c.jsonrpcProtocol.WriteMessage(c.processInfo.Stdin, response)
	} else {
		// For other server requests, send explicit null result (result: null)
		var nullResult json.RawMessage = json.RawMessage("null")
		response := protocol.CreateResponse(id, nullResult, nil)
		c.writeMu.Lock()
		defer c.writeMu.Unlock()
		return c.jsonrpcProtocol.WriteMessage(c.processInfo.Stdin, response)
	}
}

// HandleResponse implements the MessageHandler interface for client responses
func (c *StdioClient) HandleResponse(id interface{}, result json.RawMessage, err *protocol.RPCError) error {
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
			// Suppress expected errors during indexing operations
			if !protocol.IsExpectedSuppressibleError(err) {
				sanitizedError := common.SanitizeErrorForLogging(err)
				common.LSPLogger.Warn("LSP response contains error: id=%s, error=%s", idStr, sanitizedError)
			}
		} else {
			responseData = result
		}

		select {
		case req.respCh <- responseData:
			// Response delivered successfully
		case <-req.done:
			common.LSPLogger.Warn("Request already completed when trying to deliver response: id=%s", idStr)
		case <-processInfo.StopCh:
			common.LSPLogger.Warn("Client stopped when trying to deliver response: id=%s", idStr)
		}
	} else {
		// Check if this was a recently timed-out request
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

// HandleNotification implements the MessageHandler interface for server-initiated notifications
func (c *StdioClient) HandleNotification(method string, params interface{}) error {
	// Log and safely ignore notifications without stalling message processing
	return nil
}

// initializeLSP sends the initialize request to start LSP communication
func (c *StdioClient) initializeLSP(ctx context.Context) error {
	// Use configured working directory if specified, otherwise use current directory
	var wd string
	if c.config.WorkingDir != "" {
		wd = c.config.WorkingDir
	} else {
		var err error
		wd, err = os.Getwd()
		if err != nil {
			if runtime.GOOS == "windows" {
				wd = "C:\\temp"
			} else {
				wd = "/tmp"
			}
		}
	}

	// Ensure path is absolute and clean
	wd, _ = filepath.Abs(wd)
	if runtime.GOOS == "windows" {
		wd = utils.URIToFilePathCached(utils.FilePathToURI(wd))
	}

	// Get initialization options
	initOptions := c.getInitializationOptions()

	// Log initialization options for rust to debug cargo issues
	if c.language == "rust" {
		if optionsJSON, err := json.Marshal(initOptions); err == nil {
			common.LSPLogger.Info("rust-analyzer initialization options: %s", string(optionsJSON))
		}
	}

	// Send initialize request according to LSP specification
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

	// Parse server capabilities from initialize response
	if err := c.parseServerCapabilities(result); err != nil {
		common.LSPLogger.Warn("Failed to parse server capabilities for %s: %v", c.config.Command, err)
		// Continue anyway - capability detection failure shouldn't prevent initialization
	}

	// Log capabilities for rust-analyzer
	if c.language == "rust" {
		common.LSPLogger.Info("rust-analyzer capabilities: %s", string(result))
	}

	// Send initialized notification
	if err := c.SendNotification(ctx, "initialized", map[string]interface{}{}); err != nil {
		common.LSPLogger.Error("Failed to send initialized notification for %s: %v", c.language, err)
		return err
	}
	return nil
}

// getInitializationOptions returns language-specific initialization options
func (c *StdioClient) getInitializationOptions() map[string]interface{} {
	// Use initialization options from config if provided
	if c.initializationOptions != nil {
		switch opts := c.initializationOptions.(type) {
		case map[string]interface{}:
			// Recursively convert any nested map[interface{}]interface{} values
			return convertToStringMap(opts)
		case map[interface{}]interface{}:
			// Convert map[interface{}]interface{} to map[string]interface{} (YAML unmarshaling)
			return convertInterfaceMap(opts)
		}
	}

	// Fall back to defaults from language registry
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

// convertInterfaceMap recursively converts map[interface{}]interface{} to map[string]interface{}
func convertInterfaceMap(m map[interface{}]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		if key, ok := k.(string); ok {
			result[key] = convertValue(v)
		}
	}
	return result
}

// convertToStringMap recursively ensures all nested maps are map[string]interface{}
func convertToStringMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		result[k] = convertValue(v)
	}
	return result
}

// convertValue recursively converts values to ensure proper types
func convertValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[interface{}]interface{}:
		return convertInterfaceMap(val)
	case map[string]interface{}:
		return convertToStringMap(val)
	case []interface{}:
		// Convert slices recursively
		result := make([]interface{}, len(val))
		for i, item := range val {
			result[i] = convertValue(item)
		}
		return result
	default:
		return v
	}
}

// parseServerCapabilities parses the server capabilities from initialize response
func (c *StdioClient) parseServerCapabilities(result json.RawMessage) error {
	caps, err := c.capDetector.ParseCapabilities(result, c.config.Command)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.capabilities = caps
	c.mu.Unlock()

	return nil
}

// logStderr logs stderr output from the LSP server with intelligent error translation
func (c *StdioClient) logStderr() {
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

			// Collect error context for better diagnosis
			if strings.Contains(line, "Traceback") {
				errorContext = []string{line}
				continue
			}

			if len(errorContext) > 0 && (strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t")) {
				errorContext = append(errorContext, line)
				continue
			}

			// Process specific error patterns with user-friendly messages
			if c.errorTranslator.TranslateAndLogError(c.config.Command, line, errorContext) {
				errorContext = nil // Reset context after processing
				continue
			}

			// Log other errors normally
			if strings.Contains(line, "error") || strings.Contains(line, "Error") ||
				strings.Contains(line, "fatal") || strings.Contains(line, "Fatal") ||
				strings.Contains(line, "Exception") {
				common.LSPLogger.Error("LSP %s stderr ERROR: %s", c.config.Command, line)
			}

			errorContext = nil // Reset context if line doesn't match error patterns
		}
	}
}
