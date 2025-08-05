package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/security"
	"lsp-gateway/src/server/aggregators"
	"lsp-gateway/src/server/cache"
	"lsp-gateway/src/server/capabilities"
	"lsp-gateway/src/server/documents"
	"lsp-gateway/src/server/errors"
	"lsp-gateway/src/server/process"
	"lsp-gateway/src/server/protocol"
)

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
	Supports(method string) bool // NEW: Check if server supports a specific method
}

// ServerCapabilities represents LSP server capabilities

// MethodNotSupportedError represents a user-friendly error for unsupported LSP methods
type MethodNotSupportedError struct {
	Server     string
	Method     string
	Suggestion string
}

func (e *MethodNotSupportedError) Error() string {
	return fmt.Sprintf("LSP server '%s' does not support '%s'. %s",
		e.Server, e.Method, e.Suggestion)
}

// Timeout constants for basic operation
const (
	DefaultRequestTimeout = 30 * time.Second
	JavaRequestTimeout    = 60 * time.Second
	PythonRequestTimeout  = 30 * time.Second
	GoTSRequestTimeout    = 15 * time.Second
	WriteTimeout          = 10 * time.Second
)

// getRequestTimeout returns language-specific timeout for LSP requests
func (c *StdioClient) getRequestTimeout(method string) time.Duration {
	switch c.language {
	case "java":
		return JavaRequestTimeout
	case "python":
		return PythonRequestTimeout
	case "go", "javascript", "typescript":
		return GoTSRequestTimeout
	default:
		return DefaultRequestTimeout
	}
}

// getInitializeTimeout returns language-specific timeout for initialize requests
func (c *StdioClient) getInitializeTimeout() time.Duration {
	switch c.language {
	case "java":
		// Java LSP server needs significantly more time to initialize
		return 60 * time.Second
	case "python":
		return 30 * time.Second
	default:
		return 15 * time.Second
	}
}

// getClientActiveWaitIterations returns language-specific wait iterations for client to become active
func (m *LSPManager) getClientActiveWaitIterations(language string) int {
	switch language {
	case "java":
		// Java LSP server needs up to 15 seconds (150 iterations * 100ms)
		return 150
	case "python":
		// Python LSP server needs moderate time (50 iterations * 100ms = 5s)
		return 50
	default:
		// Default 3 seconds (30 iterations * 100ms)
		return 30
	}
}

// pendingRequest stores context for pending LSP requests
type pendingRequest struct {
	respCh chan json.RawMessage
	done   chan struct{}
}

// StdioClient implements LSP communication over STDIO
type StdioClient struct {
	config          ClientConfig
	language        string // Language identifier for unique request IDs
	capabilities    capabilities.ServerCapabilities
	errorTranslator errors.ErrorTranslator
	capDetector     capabilities.CapabilityDetector
	processManager  process.ProcessManager
	processInfo     *process.ProcessInfo
	jsonrpcProtocol protocol.JSONRPCProtocol

	mu       sync.RWMutex
	active   bool
	requests map[string]*pendingRequest
	nextID   int
	openDocs map[string]bool // Track opened documents to prevent duplicate didOpen
}

// LSPManager manages LSP clients for different languages
type LSPManager struct {
	clients             map[string]LSPClient
	clientErrors        map[string]error
	config              *config.Config
	ctx                 context.Context
	cancel              context.CancelFunc
	mu                  sync.RWMutex
	documentManager     documents.DocumentManager
	workspaceAggregator aggregators.WorkspaceSymbolAggregator

	// Optional SCIP cache integration - can be nil for simple usage
	scipCache cache.SCIPCache
}

// Removed convertConfigToCache - now using unified config.CacheConfig directly

// NewLSPManager creates a new LSP manager with unified cache configuration
func NewLSPManager(cfg *config.Config) (*LSPManager, error) {
	if cfg == nil {
		cfg = config.GetDefaultConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	manager := &LSPManager{
		clients:             make(map[string]LSPClient),
		clientErrors:        make(map[string]error),
		config:              cfg,
		ctx:                 ctx,
		cancel:              cancel,
		documentManager:     documents.NewLSPDocumentManager(),
		workspaceAggregator: aggregators.NewWorkspaceSymbolAggregator(),
		scipCache:           nil, // Optional cache - set to nil initially
	}

	// Try to create cache with unified config - graceful degradation if it fails
	if cfg.Cache != nil && cfg.Cache.Enabled {
		scipCache, err := cache.NewSCIPCacheManager(cfg.Cache)
		if err != nil {
			common.LSPLogger.Warn("Failed to create cache (continuing without cache): %v", err)
		} else {
			manager.scipCache = scipCache
			common.LSPLogger.Info("LSP manager initialized with unified cache integration")
		}
	}

	if manager.scipCache == nil {
		common.LSPLogger.Info("LSP manager initialized without cache - using direct LSP mode")
	}

	return manager, nil
}

// Start initializes and starts all configured LSP clients
func (m *LSPManager) Start(ctx context.Context) error {
	// Start cache if available - optional integration
	if m.scipCache != nil {
		if err := m.scipCache.Start(ctx); err != nil {
			common.LSPLogger.Warn("Failed to start SCIP cache (continuing without cache): %v", err)
			m.scipCache = nil // Disable cache on start failure
		} else {
			common.LSPLogger.Info("SCIP cache started successfully")
		}
	}

	common.LSPLogger.Info("Starting LSP manager with %d servers", len(m.config.Servers))

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
				common.LSPLogger.Error("Failed to start %s client: %v", result.language, result.err)
				m.mu.Lock()
				m.clientErrors[result.language] = result.err
				m.mu.Unlock()
			} else {
				common.LSPLogger.Info("Started %s LSP client", result.language)
			}
		case <-timeout:
			common.LSPLogger.Warn("Timeout reached, %d/%d clients started", completed, len(m.config.Servers))
			return nil
		case <-ctx.Done():
			common.LSPLogger.Warn("Context cancelled, %d/%d clients started", completed, len(m.config.Servers))
			return nil
		}
	}

	return nil
}

// Stop stops all LSP clients
func (m *LSPManager) Stop() error {
	m.cancel()

	// Stop cache if available - optional integration
	if m.scipCache != nil {
		if err := m.scipCache.Stop(); err != nil {
			common.LSPLogger.Warn("Failed to stop SCIP cache: %v", err)
		} else {
			common.LSPLogger.Info("SCIP cache stopped successfully")
		}
	}

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

	// Wait for all clients to stop with timeout (allow for 5s graceful shutdown + buffer)
	timeout := time.After(6 * time.Second)
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
				Active:    false,
				Available: false,
				Error:     fmt.Errorf("invalid command: %w", err),
			}
			continue
		}

		// Check if command exists in PATH
		if _, err := exec.LookPath(serverConfig.Command); err != nil {
			status[language] = ClientStatus{
				Active:    false,
				Available: false,
				Error:     fmt.Errorf("command not found: %s", serverConfig.Command),
			}
			continue
		}

		// Command is available but not running
		status[language] = ClientStatus{
			Active:    false,
			Available: true,
			Error:     nil,
		}
	}

	return status
}

// ProcessRequest processes a JSON-RPC request by routing it to the appropriate LSP client
func (m *LSPManager) ProcessRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	// Try cache lookup first if cache is available and method is cacheable
	if m.scipCache != nil && m.isCacheableMethod(method) {
		if result, found, err := m.scipCache.Lookup(method, params); err == nil && found {
			common.LSPLogger.Debug("Cache hit for method=%s", method)
			return result, nil
		} else if err != nil {
			// Log cache error but continue with LSP fallback
			common.LSPLogger.Debug("Cache lookup failed for method=%s: %v", method, err)
		}
	}

	// Extract file URI from params to determine language
	uri, err := m.documentManager.ExtractURI(params)
	if err != nil {
		// For methods that don't require URI (like workspace/symbol), try all clients
		if method == "workspace/symbol" {
			m.mu.RLock()
			clients := make(map[string]interface{})
			for k, v := range m.clients {
				clients[k] = v
			}
			m.mu.RUnlock()
			result, err := m.workspaceAggregator.ProcessWorkspaceSymbol(ctx, clients, params)

			// Cache the result if successful and cache is available
			if err == nil && m.scipCache != nil && m.isCacheableMethod(method) {
				if cacheErr := m.scipCache.Store(method, params, result); cacheErr != nil {
					common.LSPLogger.Debug("Failed to cache workspace/symbol result: %v", cacheErr)
				} else {
					common.LSPLogger.Debug("Cached result for workspace/symbol")
				}
			}

			return result, err
		}
		return nil, fmt.Errorf("failed to extract URI from params: %w", err)
	}

	language := m.documentManager.DetectLanguage(uri)
	if language == "" {
		return nil, fmt.Errorf("unsupported file type: %s", uri)
	}

	client, err := m.getClient(language)
	if err != nil {
		return nil, fmt.Errorf("no LSP client for language %s: %w", language, err)
	}

	// NEW: Check if server supports the requested method
	if !client.Supports(method) {
		errorTranslator := errors.NewLSPErrorTranslator()
		return nil, &MethodNotSupportedError{
			Server:     language,
			Method:     method,
			Suggestion: errorTranslator.GetMethodSuggestion(language, method),
		}
	}

	// Send textDocument/didOpen notification if needed for methods that require opened documents
	// documentSymbol works on file system and doesn't need didOpen
	needsDidOpen := method != "workspace/symbol" && method != "textDocument/documentSymbol"
	if needsDidOpen {
		m.ensureDocumentOpen(client, uri, params)
	}

	// Send request to LSP server
	result, err := client.SendRequest(ctx, method, params)

	// Cache the result and perform SCIP indexing if successful and cache is available
	if err == nil && m.scipCache != nil && m.isCacheableMethod(method) {
		if cacheErr := m.scipCache.Store(method, params, result); cacheErr != nil {
			common.LSPLogger.Debug("Failed to cache LSP response for method=%s: %v", method, cacheErr)
		} else {
			common.LSPLogger.Debug("Cached LSP response for method=%s", method)
		}

		// Perform SCIP indexing for document-related operations
		m.performSCIPIndexing(ctx, method, uri, language, params, result)
	} else {
		// Debug why caching didn't happen
		if err != nil {
			common.LSPLogger.Debug("LSP request failed for method=%s: %v", method, err)
		} else if m.scipCache == nil {
			common.LSPLogger.Debug("Cache is nil for method=%s", method)
		} else if !m.isCacheableMethod(method) {
			common.LSPLogger.Debug("Method %s is not cacheable", method)
		}
	}

	return result, err
}

// ClientStatus represents the status of an LSP client
type ClientStatus struct {
	Active    bool
	Error     error
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
					Active:    activeClient.IsActive(),
					Available: true, // If client exists, command was available
					Error:     nil,
				}
			} else {
				status[language] = ClientStatus{
					Active:    true, // Assume active if we can't check
					Available: true,
					Error:     nil,
				}
			}
		} else {
			// Client doesn't exist, check if there's an error
			if err, hasError := m.clientErrors[language]; hasError {
				status[language] = ClientStatus{
					Active:    false,
					Available: false,
					Error:     err,
				}
			} else {
				status[language] = ClientStatus{
					Active:    false,
					Available: false,
					Error:     fmt.Errorf("client not started"),
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

	client, err := NewStdioClient(clientConfig, language)
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
		maxWaitIterations := m.getClientActiveWaitIterations(language)
		for i := 0; i < maxWaitIterations; i++ {
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

	client, err := NewStdioClient(clientConfig, language)
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

// GetClient returns the LSP client for a given language (public method for testing)
func (m *LSPManager) GetClient(language string) (LSPClient, error) {
	return m.getClient(language)
}

// isCacheableMethod determines if an LSP method should be cached
func (m *LSPManager) isCacheableMethod(method string) bool {
	// Cache the 6 supported LSP methods that benefit from caching
	cacheableMethods := map[string]bool{
		"textDocument/definition":     true,
		"textDocument/references":     true,
		"textDocument/hover":          true,
		"textDocument/documentSymbol": true,
		"workspace/symbol":            true,
		"textDocument/completion":     true,
	}

	return cacheableMethods[method]
}

// InvalidateCache invalidates cached entries for a document (optional cache)
func (m *LSPManager) InvalidateCache(uri string) error {
	if m.scipCache == nil {
		return nil // No cache to invalidate
	}
	common.LSPLogger.Debug("Invalidating cache for document: %s", uri)
	return m.scipCache.InvalidateDocument(uri)
}

// GetCacheMetrics returns cache performance metrics including SCIP index stats (optional cache)
func (m *LSPManager) GetCacheMetrics() interface{} {
	if m.scipCache == nil {
		return nil
	}

	cacheMetrics := m.scipCache.GetMetrics()
	indexStats := m.scipCache.GetIndexStats()

	return map[string]interface{}{
		"cache":      cacheMetrics,
		"scip_index": indexStats,
		"integrated": true,
	}
}

// GetCache returns the SCIP cache instance for external access (optional cache)
func (m *LSPManager) GetCache() cache.SCIPCache {
	return m.scipCache // Can be nil
}

// SetCache allows optional cache injection for flexible integration
func (m *LSPManager) SetCache(cache cache.SCIPCache) {
	m.scipCache = cache
	if cache != nil {
		common.LSPLogger.Info("Cache injected into LSP manager")
	} else {
		common.LSPLogger.Info("Cache removed from LSP manager")
	}
}

// ensureDocumentOpen sends a textDocument/didOpen notification if needed
func (m *LSPManager) ensureDocumentOpen(client LSPClient, uri string, params interface{}) {
	// Check if this is a StdioClient with document tracking capability
	if stdioClient, ok := client.(*StdioClient); ok {
		// Check if document is already open to prevent duplicate didOpen
		stdioClient.mu.Lock()
		if stdioClient.openDocs[uri] {
			stdioClient.mu.Unlock()
			common.LSPLogger.Debug("Document already open, skipping didOpen: %s", uri)
			return
		}
		stdioClient.mu.Unlock()
	}

	// Use document manager to ensure document is open
	err := m.documentManager.EnsureOpen(client, uri, params)
	if err != nil {
		common.LSPLogger.Error("Failed to ensure document open for %s: %v", uri, err)
		return
	}

	// Mark document as open AFTER successful didOpen (preserve tracking functionality)
	if stdioClient, ok := client.(*StdioClient); ok {
		stdioClient.mu.Lock()
		stdioClient.openDocs[uri] = true
		stdioClient.mu.Unlock()
	}
}

// NewStdioClient creates a new STDIO LSP client
func NewStdioClient(config ClientConfig, language string) (LSPClient, error) {
	client := &StdioClient{
		config:          config,
		language:        language,
		requests:        make(map[string]*pendingRequest),
		openDocs:        make(map[string]bool),
		errorTranslator: errors.NewLSPErrorTranslator(),
		capDetector:     capabilities.NewLSPCapabilityDetector(),
		processManager:  process.NewLSPProcessManager(),
		jsonrpcProtocol: protocol.NewLSPJSONRPCProtocol(language),
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

	// Convert ClientConfig to process.ClientConfig
	processConfig := process.ClientConfig{
		Command: c.config.Command,
		Args:    c.config.Args,
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
			common.LSPLogger.Error("Error handling responses for %s: %v", c.language, err)
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
		if err != nil {
			common.LSPLogger.Error("LSP server process exited with error: language=%s, error=%v", c.language, err)
		} else {
			common.LSPLogger.Warn("LSP server process exited normally: language=%s", c.language)
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
	if !active && method != "initialize" {
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

	// Generate request ID with language prefix to ensure uniqueness across all clients
	c.mu.Lock()
	c.nextID++
	id := fmt.Sprintf("%s-%d", c.language, c.nextID)
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
	msg := protocol.CreateMessage(method, id, params)

	// Log request details for debugging
	common.LSPLogger.Debug("Sending LSP request: method=%s, id=%s", method, id)

	if err := c.jsonrpcProtocol.WriteMessage(c.processInfo.Stdin, msg); err != nil {
		// If we get connection errors, mark client as inactive
		if strings.Contains(err.Error(), "broken pipe") ||
			strings.Contains(err.Error(), "write: connection reset by peer") ||
			strings.Contains(err.Error(), "EOF") {
			c.mu.Lock()
			c.active = false
			c.mu.Unlock()
			common.LSPLogger.Warn("LSP client connection lost, marking as inactive: method=%s, id=%s, error=%v", method, id, err)
		}
		common.LSPLogger.Error("Failed to send LSP request: method=%s, id=%s, error=%v", method, id, err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	common.LSPLogger.Debug("LSP request sent successfully: method=%s, id=%s", method, id)

	// Wait for response with appropriate timeout
	timeoutDuration := c.getRequestTimeout(method)
	if method == "initialize" {
		// Use longer timeout for initialize - LSP servers can be slow to start
		timeoutDuration = c.getInitializeTimeout()
	}

	ctx, cancel := context.WithTimeout(ctx, timeoutDuration)
	defer cancel()

	common.LSPLogger.Debug("Waiting for LSP response: method=%s, id=%s, timeout=%v", method, id, timeoutDuration)

	select {
	case response := <-request.respCh:
		common.LSPLogger.Debug("Received LSP response successfully: method=%s, id=%s", method, id)
		return response, nil
	case <-ctx.Done():
		common.LSPLogger.Error("LSP request timeout: method=%s, id=%s, timeout=%v", method, id, timeoutDuration)
		return nil, fmt.Errorf("request timeout after %v for method %s", timeoutDuration, method)
	case <-processInfo.StopCh:
		common.LSPLogger.Warn("LSP client stopped during request: method=%s, id=%s", method, id)
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
	common.LSPLogger.Debug("Received server request: method=%s, id=%v from %s", method, id, c.language)

	// Handle specific server requests
	if method == "workspace/configuration" {
		// Respond with empty configuration
		response := protocol.CreateResponse(id, []interface{}{map[string]interface{}{}}, nil)
		return c.jsonrpcProtocol.WriteMessage(c.processInfo.Stdin, response)
	} else {
		// For other server requests, send null result
		response := protocol.CreateResponse(id, nil, nil)
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
			sanitizedError := common.SanitizeErrorForLogging(err)
			common.LSPLogger.Warn("LSP response contains error: id=%s, error=%s", idStr, sanitizedError)
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
		common.LSPLogger.Warn("No matching request found for response: id=%s", idStr)
	}

	return nil
}

// HandleNotification implements the MessageHandler interface for server-initiated notifications
func (c *StdioClient) HandleNotification(method string, params interface{}) error {
	common.LSPLogger.Debug("Received server notification: method=%s from %s", method, c.language)
	// Log and safely ignore notifications without stalling message processing
	return nil
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

	result, err := c.SendRequest(ctx, "initialize", initParams)
	if err != nil {
		return err
	}

	// Parse server capabilities from initialize response
	if err := c.parseServerCapabilities(result); err != nil {
		common.LSPLogger.Warn("Failed to parse server capabilities for %s: %v", c.config.Command, err)
		// Continue anyway - capability detection failure shouldn't prevent initialization
	}

	// Send initialized notification
	common.LSPLogger.Debug("Sending initialized notification for %s", c.language)
	if err := c.SendNotification(ctx, "initialized", map[string]interface{}{}); err != nil {
		common.LSPLogger.Error("Failed to send initialized notification for %s: %v", c.language, err)
		return err
	}
	common.LSPLogger.Debug("Successfully sent initialized notification for %s", c.language)
	return nil
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

			// Log ALL stderr output for debugging hover issues
			common.LSPLogger.Info("LSP %s stderr: %s", c.language, line)

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

// SCIP Integration Methods - Integrated as core functionality

// performSCIPIndexing performs SCIP indexing based on LSP method and response
func (m *LSPManager) performSCIPIndexing(ctx context.Context, method, uri, language string, params, result interface{}) {
	if m.scipCache == nil {
		return
	}

	// Only index for specific methods that provide useful data
	switch method {
	case "textDocument/documentSymbol":
		m.indexDocumentSymbols(ctx, uri, language, result)
	case "textDocument/definition":
		m.indexDefinitions(ctx, uri, language, result)
	case "textDocument/references":
		m.indexReferences(ctx, uri, language, result)
	case "workspace/symbol":
		m.indexWorkspaceSymbols(ctx, language, result)
	}
}

// indexDocumentSymbols indexes symbols from a document symbol response
func (m *LSPManager) indexDocumentSymbols(ctx context.Context, uri, language string, result interface{}) {
	// In a real implementation, this would read the actual file content
	// For now, we'll use a placeholder approach
	if err := m.scipCache.IndexDocument(ctx, uri, language, []byte("// indexed from LSP response")); err != nil {
		common.LSPLogger.Debug("Failed to index document symbols for %s: %v", uri, err)
	} else {
		common.LSPLogger.Debug("Indexed document symbols for %s", uri)
	}
}

// indexDefinitions indexes definition information
func (m *LSPManager) indexDefinitions(ctx context.Context, uri, language string, result interface{}) {
	// For now, just log - in a full implementation this would extract definition data
	common.LSPLogger.Debug("Processing definition data for SCIP index: %s", uri)
}

// indexReferences indexes reference information
func (m *LSPManager) indexReferences(ctx context.Context, uri, language string, result interface{}) {
	// For now, just log - in a full implementation this would extract reference data
	common.LSPLogger.Debug("Processing reference data for SCIP index: %s", uri)
}

// indexWorkspaceSymbols indexes workspace symbol information
func (m *LSPManager) indexWorkspaceSymbols(ctx context.Context, language string, result interface{}) {
	// For now, just log - in a full implementation this would extract workspace symbol data
	common.LSPLogger.Debug("Processing workspace symbols for SCIP index: %s", language)
}

// ProcessEnhancedQuery processes a query that combines LSP and SCIP data
func (m *LSPManager) ProcessEnhancedQuery(ctx context.Context, queryType, uri, language string, params interface{}) (interface{}, error) {
	if m.scipCache == nil {
		// Fall back to regular LSP processing
		return m.ProcessRequest(ctx, queryType, params)
	}

	// Query SCIP index first for enhanced data
	indexQuery := &cache.IndexQuery{
		Type:     queryType,
		URI:      uri,
		Language: language,
	}

	indexResult, err := m.scipCache.QueryIndex(ctx, indexQuery)
	if err == nil && len(indexResult.Results) > 0 {
		common.LSPLogger.Debug("Using SCIP index data for enhanced query: %s", queryType)
		return indexResult, nil
	}

	// Fall back to LSP
	common.LSPLogger.Debug("Falling back to LSP for query: %s", queryType)
	return m.ProcessRequest(ctx, queryType, params)
}

// GetIndexStats returns SCIP index statistics
func (m *LSPManager) GetIndexStats() interface{} {
	if m.scipCache == nil {
		return map[string]interface{}{"status": "disabled"}
	}

	return m.scipCache.GetIndexStats()
}

// RefreshIndex refreshes the SCIP index for given files
func (m *LSPManager) RefreshIndex(ctx context.Context, files []string) error {
	if m.scipCache == nil {
		return fmt.Errorf("SCIP cache not available")
	}

	return m.scipCache.UpdateIndex(ctx, files)
}

// Enhanced cache methods that include SCIP functionality - no duplicates needed as existing methods are enhanced
