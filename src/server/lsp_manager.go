package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/security"
)

// JSONRPCMessage represents a JSON-RPC 2.0 message
type JSONRPCMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// ClientConfig contains configuration for an LSP client
type ClientConfig struct {
	Command string
	Args    []string
}

// LSPClient interface for LSP communication
type LSPClient interface {
	Start(ctx context.Context) error
	Stop() error
	SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error)
	SendNotification(ctx context.Context, method string, params interface{}) error
	IsActive() bool
}

// Timeout constants for basic operation
const (
	RequestTimeout = 30 * time.Second
	WriteTimeout   = 10 * time.Second
	JSONRPCVersion = "2.0"
)

// pendingRequest stores context for pending LSP requests
type pendingRequest struct {
	respCh chan json.RawMessage
	done   chan struct{}
}

// StdioClient implements LSP communication over STDIO
type StdioClient struct {
	config ClientConfig
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	mu       sync.RWMutex
	active   bool
	requests map[string]*pendingRequest
	nextID   int

	stopCh chan struct{}
	done   chan struct{}
}

// LSPManager manages LSP clients for different languages
type LSPManager struct {
	clients      map[string]LSPClient
	clientErrors map[string]error
	config       *config.Config
	logger       *log.Logger
	ctx          context.Context
	cancel       context.CancelFunc
	mu           sync.RWMutex
}

// NewLSPManager creates a new LSP manager
func NewLSPManager(cfg *config.Config) (*LSPManager, error) {
	if cfg == nil {
		cfg = config.GetDefaultConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	manager := &LSPManager{
		clients:      make(map[string]LSPClient),
		clientErrors: make(map[string]error),
		config:       cfg,
		logger:       log.New(log.Writer(), "[LSPManager] ", log.LstdFlags),
		ctx:          ctx,
		cancel:       cancel,
	}

	return manager, nil
}

// Start initializes and starts all configured LSP clients
func (m *LSPManager) Start(ctx context.Context) error {
	m.logger.Printf("Starting LSP manager with %d servers", len(m.config.Servers))

	// Start clients with individual timeouts to prevent hanging
	results := make(chan struct {
		language string
		err      error
	}, len(m.config.Servers))

	// Start each client in a separate goroutine
	for language, serverConfig := range m.config.Servers {
		go func(lang string, cfg *config.ServerConfig) {
			// Individual timeout per server
			clientCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			err := m.startClientWithTimeout(clientCtx, lang, cfg)
			results <- struct {
				language string
				err      error
			}{lang, err}
		}(language, serverConfig)
	}

	// Collect results with overall timeout
	timeout := time.After(15 * time.Second)
	completed := 0

	for completed < len(m.config.Servers) {
		select {
		case result := <-results:
			completed++
			if result.err != nil {
				m.logger.Printf("Failed to start %s client: %v", result.language, result.err)
				m.mu.Lock()
				m.clientErrors[result.language] = result.err
				m.mu.Unlock()
			} else {
				m.logger.Printf("Started %s LSP client", result.language)
			}
		case <-timeout:
			m.logger.Printf("Timeout reached, %d/%d clients started", completed, len(m.config.Servers))
			return nil
		case <-ctx.Done():
			m.logger.Printf("Context cancelled, %d/%d clients started", completed, len(m.config.Servers))
			return nil
		}
	}

	return nil
}

// Stop stops all LSP clients
func (m *LSPManager) Stop() error {
	m.cancel()

	m.mu.Lock()
	clients := make(map[string]LSPClient)
	for k, v := range m.clients {
		clients[k] = v
	}
	m.clients = make(map[string]LSPClient)
	m.mu.Unlock()

	// Stop clients in parallel for faster shutdown
	done := make(chan error, len(clients))
	for language, client := range clients {
		go func(lang string, c LSPClient) {
			err := c.Stop()
			if err != nil {
				common.LSPLogger.Error("Error stopping %s client: %v", lang, err)
			}
			done <- err
		}(language, client)
	}

	// Wait for all clients to stop with timeout
	timeout := time.After(2 * time.Second)
	completed := 0
	var lastErr error

	for completed < len(clients) {
		select {
		case err := <-done:
			completed++
			if err != nil {
				lastErr = err
			}
		case <-timeout:
			common.LSPLogger.Warn("Timeout stopping LSP clients, %d/%d completed", completed, len(clients))
			return fmt.Errorf("timeout stopping LSP clients")
		}
	}

	return lastErr
}

// CheckServerAvailability checks if LSP server commands are available without starting them
func (m *LSPManager) CheckServerAvailability() map[string]ClientStatus {
	status := make(map[string]ClientStatus)
	
	for language, serverConfig := range m.config.Servers {
		// Validate command using security module
		if err := security.ValidateCommand(serverConfig.Command, serverConfig.Args); err != nil {
			status[language] = ClientStatus{
				Active: false,
				Available: false,
				Error: fmt.Errorf("invalid command: %w", err),
			}
			continue
		}
		
		// Check if command exists in PATH
		if _, err := exec.LookPath(serverConfig.Command); err != nil {
			status[language] = ClientStatus{
				Active: false,
				Available: false,
				Error: fmt.Errorf("command not found: %s", serverConfig.Command),
			}
			continue
		}
		
		// Command is available but not running
		status[language] = ClientStatus{
			Active: false,
			Available: true,
			Error: nil,
		}
	}
	
	return status
}

// ProcessRequest processes a JSON-RPC request by routing it to the appropriate LSP client
func (m *LSPManager) ProcessRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	// Extract file URI from params to determine language
	uri, err := m.extractURI(params)
	if err != nil {
		// For methods that don't require URI (like workspace/symbol), try all clients
		if method == "workspace/symbol" {
			return m.processWorkspaceSymbol(ctx, params)
		}
		return nil, fmt.Errorf("failed to extract URI from params: %w", err)
	}

	language := m.detectLanguage(uri)
	if language == "" {
		return nil, fmt.Errorf("unsupported file type: %s", uri)
	}

	client, err := m.getClient(language)
	if err != nil {
		return nil, fmt.Errorf("no LSP client for language %s: %w", language, err)
	}

	// Send textDocument/didOpen notification if needed
	if method != "workspace/symbol" {
		m.ensureDocumentOpen(client, uri, params)
	}

	return client.SendRequest(ctx, method, params)
}

// ClientStatus represents the status of an LSP client
type ClientStatus struct {
	Active bool
	Error  error
	Available bool // Whether the server command is available on system
}

// GetClientStatus returns the status of all LSP clients
func (m *LSPManager) GetClientStatus() map[string]ClientStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]ClientStatus)

	// Add all configured servers to status
	for language := range m.config.Servers {
		if client, exists := m.clients[language]; exists {
			// Client exists, check if it's active
			if activeClient, ok := client.(interface{ IsActive() bool }); ok {
				status[language] = ClientStatus{
					Active: activeClient.IsActive(),
					Available: true, // If client exists, command was available
					Error:  nil,
				}
			} else {
				status[language] = ClientStatus{
					Active: true, // Assume active if we can't check
					Available: true,
					Error:  nil,
				}
			}
		} else {
			// Client doesn't exist, check if there's an error
			if err, hasError := m.clientErrors[language]; hasError {
				status[language] = ClientStatus{
					Active: false,
					Available: false,
					Error:  err,
				}
			} else {
				status[language] = ClientStatus{
					Active: false,
					Available: false,
					Error:  fmt.Errorf("client not started"),
				}
			}
		}
	}

	return status
}

// startClientWithTimeout starts a single LSP client with timeout
func (m *LSPManager) startClientWithTimeout(ctx context.Context, language string, cfg *config.ServerConfig) error {
	// Validate LSP server command for security
	if err := security.ValidateCommand(cfg.Command, cfg.Args); err != nil {
		return fmt.Errorf("invalid LSP server command for %s: %w", language, err)
	}

	// Check if LSP server executable is available
	if _, err := exec.LookPath(cfg.Command); err != nil {
		return fmt.Errorf("LSP server executable not found for %s: %s", language, cfg.Command)
	}

	clientConfig := ClientConfig{
		Command: cfg.Command,
		Args:    cfg.Args,
	}

	client, err := NewStdioClient(clientConfig)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if err := client.Start(ctx); err != nil {
		return fmt.Errorf("failed to start client: %w", err)
	}

	m.mu.Lock()
	m.clients[language] = client
	m.mu.Unlock()

	// Wait for client to become active with timeout
	if activeClient, ok := client.(interface{ IsActive() bool }); ok {
		for i := 0; i < 30; i++ { // Wait up to 3 seconds
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled while waiting for client to become active")
			default:
				if activeClient.IsActive() {
					return nil
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
		return fmt.Errorf("client did not become active within timeout")
	}

	return nil
}

// startClient starts a single LSP client for a language
func (m *LSPManager) startClient(language string, cfg *config.ServerConfig) error {
	// Validate LSP server command for security
	if err := security.ValidateCommand(cfg.Command, cfg.Args); err != nil {
		return fmt.Errorf("invalid LSP server command for %s: %w", language, err)
	}

	clientConfig := ClientConfig{
		Command: cfg.Command,
		Args:    cfg.Args,
	}

	client, err := NewStdioClient(clientConfig)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if err := client.Start(m.ctx); err != nil {
		return fmt.Errorf("failed to start client: %w", err)
	}

	m.mu.Lock()
	m.clients[language] = client
	m.mu.Unlock()

	// Wait for client to become active
	if activeClient, ok := client.(interface{ IsActive() bool }); ok {
		for i := 0; i < 50; i++ { // Wait up to 5 seconds
			if activeClient.IsActive() {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	return nil
}

// getClient returns the LSP client for a given language
func (m *LSPManager) getClient(language string) (LSPClient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, exists := m.clients[language]
	if !exists {
		return nil, fmt.Errorf("no client for language: %s", language)
	}
	return client, nil
}

// detectLanguage detects the programming language from a file URI
func (m *LSPManager) detectLanguage(uri string) string {
	// Remove file:// prefix and get extension
	path := strings.TrimPrefix(uri, "file://")
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".java":
		return "java"
	default:
		return ""
	}
}

// extractURI extracts the file URI from request parameters
func (m *LSPManager) extractURI(params interface{}) (string, error) {
	if params == nil {
		return "", fmt.Errorf("no parameters provided")
	}

	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("params is not a map")
	}

	// Try textDocument.uri first
	if textDoc, ok := paramsMap["textDocument"].(map[string]interface{}); ok {
		if uri, ok := textDoc["uri"].(string); ok {
			return uri, nil
		}
	}

	// Try direct uri parameter
	if uri, ok := paramsMap["uri"].(string); ok {
		return uri, nil
	}

	return "", fmt.Errorf("no URI found in parameters")
}

// processWorkspaceSymbol processes workspace/symbol requests across all clients
func (m *LSPManager) processWorkspaceSymbol(ctx context.Context, params interface{}) (interface{}, error) {
	m.mu.RLock()
	clients := make([]LSPClient, 0, len(m.clients))
	languages := make([]string, 0, len(m.clients))
	for lang, client := range m.clients {
		clients = append(clients, client)
		languages = append(languages, lang)
	}
	m.mu.RUnlock()

	if len(clients) == 0 {
		return nil, fmt.Errorf("no active LSP clients available")
	}

	// Collect errors from all clients
	var errors []string

	// Try each client until we get a successful response
	for i, client := range clients {
		result, err := client.SendRequest(ctx, "workspace/symbol", params)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", languages[i], err))
			continue // Try next client
		}
		return result, nil
	}

	// If all clients failed, return detailed error information
	return nil, fmt.Errorf("no workspace symbol results found - all clients failed: %s", strings.Join(errors, "; "))
}

// ensureDocumentOpen sends a textDocument/didOpen notification if needed
func (m *LSPManager) ensureDocumentOpen(client LSPClient, uri string, params interface{}) {
	// For simplicity, we'll skip content reading and just send a basic didOpen
	// In a production system, you might want to read the actual file content

	didOpenParams := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        uri,
			"languageId": m.detectLanguage(uri),
			"version":    1,
			"text":       "", // Empty for now - LSP servers can usually handle this
		},
	}

	// Send notification (ignore errors as this is optional)
	if notifyClient, ok := client.(interface {
		SendNotification(ctx context.Context, method string, params interface{}) error
	}); ok {
		notifyClient.SendNotification(context.Background(), "textDocument/didOpen", didOpenParams)
	}
}

// NewStdioClient creates a new STDIO LSP client
func NewStdioClient(config ClientConfig) (LSPClient, error) {
	client := &StdioClient{
		config:   config,
		requests: make(map[string]*pendingRequest),
		stopCh:   make(chan struct{}),
		done:     make(chan struct{}),
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

	// Create command
	c.cmd = exec.Command(c.config.Command, c.config.Args...)
	// Use current working directory, but fallback to /tmp if needed
	if wd, err := os.Getwd(); err == nil {
		c.cmd.Dir = wd
	} else {
		c.cmd.Dir = "/tmp"
	}

	// Setup pipes
	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		c.stdin.Close()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	c.stderr, err = c.cmd.StderrPipe()
	if err != nil {
		c.stdin.Close()
		c.stdout.Close()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start process
	if err := c.cmd.Start(); err != nil {
		c.cleanup()
		return fmt.Errorf("failed to start LSP server: %w", err)
	}

	// Mark as active so STDIO communication can proceed
	c.mu.Lock()
	c.active = true
	c.mu.Unlock()

	// Start response handler
	go c.handleResponses()

	// Start stderr logger
	go c.logStderr()

	// Start process monitor
	go c.monitorProcess()

	// Initialize LSP server
	if err := c.initializeLSP(ctx); err != nil {
		c.mu.Lock()
		c.active = false
		c.mu.Unlock()
		c.cleanup()
		return fmt.Errorf("failed to initialize LSP server: %w", err)
	}

	return nil
}

// Stop terminates the LSP server process
func (c *StdioClient) Stop() error {
	c.mu.Lock()
	if !c.active {
		c.mu.Unlock()
		return nil
	}

	c.active = false
	c.mu.Unlock()

	// Close stop channel to signal goroutines to stop
	select {
	case <-c.stopCh:
		// Already closed
	default:
		close(c.stopCh)
	}

	// Send shutdown and exit notifications with timeout
	c.sendShutdown()

	// Wait for graceful shutdown with proper timeout
	if c.cmd != nil && c.cmd.Process != nil {
		// Create channel to track process completion
		done := make(chan error, 1)
		go func() {
			done <- c.cmd.Wait()
		}()

		// Wait up to 5 seconds for graceful exit
		select {
		case err := <-done:
			// Process exited gracefully
			if err != nil && !strings.Contains(err.Error(), "signal: killed") {
				common.LSPLogger.Debug("LSP server %s exited with error: %v", c.config.Command, err)
			}
		case <-time.After(5 * time.Second):
			// Timeout - force kill only after waiting
			common.LSPLogger.Warn("LSP server %s did not exit gracefully within 5 seconds, force killing", c.config.Command)
			if err := c.cmd.Process.Kill(); err != nil {
				common.LSPLogger.Error("Failed to kill LSP server %s: %v", c.config.Command, err)
			}
			// Wait for process to actually die
			<-done
		}
	}

	// Close pipes
	c.cleanup()

	// Wait for handlers to finish with timeout
	select {
	case <-c.done:
		// Handlers finished
	case <-time.After(500 * time.Millisecond):
		// Timeout - force cleanup
		common.LSPLogger.Warn("Timeout waiting for client %s handlers to finish", c.config.Command)
	}

	return nil
}

// SendRequest sends a JSON-RPC request and waits for response
func (c *StdioClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	c.mu.RLock()
	active := c.active
	cmd := c.cmd
	c.mu.RUnlock()

	// Check if client is active and process is still running
	if !active && method != "initialize" {
		return nil, fmt.Errorf("client not active")
	}

	// Additional check: verify process is still alive
	if cmd != nil && cmd.Process != nil {
		if processState := cmd.ProcessState; processState != nil && processState.Exited() {
			c.mu.Lock()
			c.active = false
			c.mu.Unlock()
			return nil, fmt.Errorf("LSP server process has exited")
		}
	}

	// Generate request ID
	c.mu.Lock()
	c.nextID++
	id := strconv.Itoa(c.nextID)
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

	// Send message
	msg := JSONRPCMessage{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Method:  method,
		Params:  params,
	}

	if err := c.writeMessage(msg); err != nil {
		// If we get a broken pipe error, mark client as inactive
		if strings.Contains(err.Error(), "broken pipe") || strings.Contains(err.Error(), "write: connection reset by peer") {
			c.mu.Lock()
			c.active = false
			c.mu.Unlock()
		}
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response with appropriate timeout
	timeoutDuration := RequestTimeout
	if method == "initialize" {
		// Use longer timeout for initialize - LSP servers can be slow to start
		timeoutDuration = 15 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeoutDuration)
	defer cancel()

	select {
	case response := <-request.respCh:
		return response, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("request timeout after %v for method %s", timeoutDuration, method)
	case <-c.stopCh:
		return nil, fmt.Errorf("client stopped")
	}
}

// SendNotification sends a JSON-RPC notification (no response expected)
func (c *StdioClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	c.mu.RLock()
	if !c.active {
		c.mu.RUnlock()
		return fmt.Errorf("client not active")
	}
	c.mu.RUnlock()

	msg := JSONRPCMessage{
		JSONRPC: JSONRPCVersion,
		Method:  method,
		Params:  params,
	}

	return c.writeMessage(msg)
}

// IsActive returns true if the client is active
func (c *StdioClient) IsActive() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.active
}

// handleResponses processes responses from the LSP server
func (c *StdioClient) handleResponses() {
	defer func() {
		// Ensure done channel is always closed
		select {
		case <-c.done:
			// Already closed
		default:
			close(c.done)
		}
	}()

	if c.stdout == nil {
		return
	}

	reader := bufio.NewReader(c.stdout)
	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		// Read Content-Length header with timeout
		var contentLength int
		headerDone := make(chan bool, 1)

		go func() {
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					headerDone <- false
					return
				}

				line = strings.TrimSpace(line)
				if line == "" {
					// Empty line indicates end of headers
					headerDone <- true
					return
				}

				if strings.HasPrefix(line, "Content-Length:") {
					lengthStr := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
					length, err := strconv.Atoi(lengthStr)
					if err != nil {
						continue
					}
					contentLength = length
				}
			}
		}()

		select {
		case success := <-headerDone:
			if !success {
				return
			}
		case <-c.stopCh:
			return
		case <-time.After(1 * time.Second):
			return // Timeout reading headers
		}

		if contentLength > 0 {
			// Read the JSON body
			body := make([]byte, contentLength)
			_, err := io.ReadFull(reader, body)
			if err != nil {
				return
			}

			c.handleMessage(body)
		}
	}
}

// handleMessage processes a single JSON-RPC message
func (c *StdioClient) handleMessage(data []byte) {
	var msg JSONRPCMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		// Silently handle JSON errors to avoid STDIO interference
		return
	}

	// Handle response
	if msg.ID != nil {
		idStr := fmt.Sprintf("%v", msg.ID)
		c.mu.RLock()
		req, exists := c.requests[idStr]
		c.mu.RUnlock()

		if exists {
			// Extract result or error
			var result json.RawMessage
			if msg.Error != nil {
				errorData, _ := json.Marshal(msg.Error)
				result = errorData
			} else if msg.Result != nil {
				result, _ = json.Marshal(msg.Result)
			}

			select {
			case req.respCh <- result:
			case <-req.done:
			case <-c.stopCh:
			}
		}
	}
	// Ignore notifications for simplicity
}

// writeMessage sends a JSON-RPC message to the LSP server
func (c *StdioClient) writeMessage(msg JSONRPCMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Format with Content-Length header
	content := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(data), data)

	c.mu.RLock()
	stdin := c.stdin
	c.mu.RUnlock()

	if stdin == nil {
		return fmt.Errorf("stdin not available")
	}

	_, err = stdin.Write([]byte(content))
	return err
}

// initializeLSP sends the initialize request to start LSP communication
func (c *StdioClient) initializeLSP(ctx context.Context) error {
	// Use current working directory, but fallback to /tmp if needed
	wd, err := os.Getwd()
	if err != nil {
		wd = "/tmp"
	}

	// Send initialize request according to LSP specification
	initParams := map[string]interface{}{
		"processId": os.Getpid(),
		"clientInfo": map[string]interface{}{
			"name":    "lsp-gateway",
			"version": "1.0.0",
		},
		"rootPath":              wd, // Deprecated but still supported
		"rootUri":               "file://" + wd,
		"initializationOptions": nil,
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
						"deprecatedSupport":       true,
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
					"dynamicRegistration": true,
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
		"workspaceFolders": []map[string]interface{}{
			{
				"uri":  "file://" + wd,
				"name": "workspace",
			},
		},
	}

	_, err = c.SendRequest(ctx, "initialize", initParams)
	if err != nil {
		return err
	}

	// Send initialized notification
	return c.SendNotification(ctx, "initialized", map[string]interface{}{})
}

// sendShutdown sends shutdown sequence to LSP server
func (c *StdioClient) sendShutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Send shutdown request
	_, err := c.SendRequest(ctx, "shutdown", nil)
	if err != nil {
		common.LSPLogger.Debug("Failed to send shutdown to %s: %v", c.config.Command, err)
	}

	// Send exit notification
	err = c.SendNotification(ctx, "exit", nil)
	if err != nil {
		common.LSPLogger.Debug("Failed to send exit to %s: %v", c.config.Command, err)
	}
}

// logStderr logs stderr output from the LSP server
func (c *StdioClient) logStderr() {
	if c.stderr == nil {
		return
	}

	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		select {
		case <-c.stopCh:
			return
		default:
			line := scanner.Text()
			// Only log critical errors to avoid noise
			if strings.Contains(line, "error") || strings.Contains(line, "Error") ||
				strings.Contains(line, "fatal") || strings.Contains(line, "Fatal") {
				common.LSPLogger.Error("LSP %s stderr: %s", c.config.Command, line)
			}
		}
	}
}

// monitorProcess watches the LSP server process and updates client status
func (c *StdioClient) monitorProcess() {
	if c.cmd == nil || c.cmd.Process == nil {
		common.LSPLogger.Error("monitorProcess called with nil command or process")
		return
	}

	// Wait for process to finish
	err := c.cmd.Wait()

	// Check if client was active when process exited (helps identify crashes vs normal shutdown)
	c.mu.Lock()
	wasActive := c.active
	c.active = false
	c.mu.Unlock()

	// Enhanced error reporting based on client state
	if err != nil {
		if wasActive {
			common.LSPLogger.Error("LSP server %s crashed unexpectedly: %v", c.config.Command, err)
		} else {
			common.LSPLogger.Warn("LSP server %s failed to start: %v", c.config.Command, err)
		}
	} else {
		if wasActive {
			common.LSPLogger.Info("LSP server %s exited normally", c.config.Command)
		} else {
			common.LSPLogger.Info("LSP server %s stopped during initialization", c.config.Command)
		}
	}

	// Signal stop to other goroutines
	select {
	case <-c.stopCh:
		// Already stopped
	default:
		close(c.stopCh)
	}
}

// cleanup closes all pipes and resources
func (c *StdioClient) cleanup() {
	if c.stdin != nil {
		c.stdin.Close()
		c.stdin = nil
	}
	if c.stdout != nil {
		c.stdout.Close()
		c.stdout = nil
	}
	if c.stderr != nil {
		c.stderr.Close()
		c.stderr = nil
	}
}
