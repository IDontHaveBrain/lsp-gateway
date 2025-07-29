package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"

	"lsp-gateway/mcp"
)

// MockLSPServer provides a configurable mock LSP server for testing workspace functionality
type MockLSPServer struct {
	Language        string
	Port            int
	RequestHistory  []LSPRequest
	ResponseDelay   time.Duration
	FailureRate     float64
	IsRunning       bool
	server          *http.Server
	listener        net.Listener
	responses       map[string]interface{}
	capabilities    map[string]interface{}
	mu              sync.RWMutex
	logger          *mcp.StructuredLogger
	requestCount    int
	startTime       time.Time
	requestHandlers map[string]func(interface{}) (interface{}, error)
}

// MockResponse represents a configurable response for specific LSP methods
type MockResponse struct {
	Method   string      `json:"method"`
	Response interface{} `json:"response"`
	Delay    time.Duration `json:"delay,omitempty"`
	Error    *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// MockServerConfig configures mock LSP server behavior
type MockServerConfig struct {
	Language        string
	Port            int
	ResponseDelay   time.Duration
	FailureRate     float64
	CustomResponses map[string]interface{}
	Capabilities    map[string]interface{}
	EnableLogging   bool
	LogLevel        string
}

// DefaultMockServerConfig returns sensible defaults for mock LSP server testing
func DefaultMockServerConfig(language string) MockServerConfig {
	return MockServerConfig{
		Language:        language,
		Port:            0, // Auto-assign
		ResponseDelay:   10 * time.Millisecond,
		FailureRate:     0.0,
		CustomResponses: make(map[string]interface{}),
		Capabilities:    CreateDefaultCapabilities(language),
		EnableLogging:   true,
		LogLevel:        "debug",
	}
}

// NewMockLSPServer creates a new mock LSP server with default configuration
func NewMockLSPServer(language string) *MockLSPServer {
	return NewMockLSPServerWithConfig(DefaultMockServerConfig(language))
}

// NewMockLSPServerWithConfig creates a new mock LSP server with custom configuration
func NewMockLSPServerWithConfig(config MockServerConfig) *MockLSPServer {
	var logger *mcp.StructuredLogger
	if config.EnableLogging {
		logLevel := parseLogLevel(config.LogLevel)
		loggerConfig := &mcp.LoggerConfig{
			Level:     logLevel,
			Component: fmt.Sprintf("mock-lsp-%s", config.Language),
		}
		logger = mcp.NewStructuredLogger(loggerConfig)
	}
	
	server := &MockLSPServer{
		Language:        config.Language,
		Port:            config.Port,
		RequestHistory:  make([]LSPRequest, 0),
		ResponseDelay:   config.ResponseDelay,
		FailureRate:     config.FailureRate,
		IsRunning:       false,
		responses:       config.CustomResponses,
		capabilities:    config.Capabilities,
		logger:          logger,
		requestHandlers: make(map[string]func(interface{}) (interface{}, error)),
	}
	
	// Setup default request handlers
	server.setupDefaultHandlers()
	
	return server
}

// Start starts the mock LSP server on the configured or auto-assigned port
func (m *MockLSPServer) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.IsRunning {
		return fmt.Errorf("mock LSP server for %s is already running", m.Language)
	}
	
	// Create listener with port auto-assignment if needed
	var err error
	addr := fmt.Sprintf("localhost:%d", m.Port)
	m.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	
	// Update port if auto-assigned
	if m.Port == 0 {
		m.Port = m.listener.Addr().(*net.TCPAddr).Port
	}
	
	// Create HTTP server with JSON-RPC handler
	mux := http.NewServeMux()
	mux.HandleFunc("/", m.handleJSONRPC)
	mux.HandleFunc("/health", m.handleHealth)
	mux.HandleFunc("/capabilities", m.handleCapabilities)
	mux.HandleFunc("/history", m.handleRequestHistory)
	
	m.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	
	// Start server in goroutine
	go func() {
		if err := m.server.Serve(m.listener); err != nil && err != http.ErrServerClosed {
			if m.logger != nil {
				m.logger.WithError(err).Error("Mock LSP server error")
			}
		}
	}()
	
	m.IsRunning = true
	m.startTime = time.Now()
	
	if m.logger != nil {
		m.logger.WithFields(map[string]interface{}{
			"language": m.Language,
			"port":     m.Port,
			"addr":     m.listener.Addr().String(),
		}).Info("Mock LSP server started")
	}
	
	return nil
}

// Stop gracefully stops the mock LSP server
func (m *MockLSPServer) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if !m.IsRunning {
		return nil
	}
	
	if m.logger != nil {
		m.logger.WithField("language", m.Language).Info("Stopping mock LSP server")
	}
	
	// Stop HTTP server with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	var err error
	if m.server != nil {
		err = m.server.Shutdown(ctx)
		m.server = nil
	}
	
	if m.listener != nil {
		m.listener.Close()
		m.listener = nil
	}
	
	m.IsRunning = false
	
	if m.logger != nil {
		uptime := time.Since(m.startTime)
		m.logger.WithFields(map[string]interface{}{
			"language":      m.Language,
			"uptime":        uptime,
			"request_count": m.requestCount,
		}).Info("Mock LSP server stopped")
	}
	
	return err
}

// HandleRequest processes a JSON-RPC request and returns appropriate response
func (m *MockLSPServer) HandleRequest(method string, params interface{}) (interface{}, error) {
	m.mu.Lock()
	
	// Record request
	request := LSPRequest{
		Method:    method,
		Params:    params,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("req-%d", m.requestCount),
	}
	m.RequestHistory = append(m.RequestHistory, request)
	m.requestCount++
	
	m.mu.Unlock()
	
	if m.logger != nil {
		m.logger.WithFields(map[string]interface{}{
			"method":   method,
			"language": m.Language,
		}).Debug("Handling LSP request")
	}
	
	// Simulate response delay
	if m.ResponseDelay > 0 {
		time.Sleep(m.ResponseDelay)
	}
	
	// Simulate random failures
	if m.FailureRate > 0 && rand.Float64() < m.FailureRate {
		return nil, fmt.Errorf("simulated failure for method %s", method)
	}
	
	// Check for custom response first
	m.mu.RLock()
	if customResponse, exists := m.responses[method]; exists {
		m.mu.RUnlock()
		return customResponse, nil
	}
	
	// Use request handler if available
	if handler, exists := m.requestHandlers[method]; exists {
		m.mu.RUnlock()
		return handler(params)
	}
	m.mu.RUnlock()
	
	// Fallback to default responses
	return m.getDefaultResponse(method, params)
}

// GetRequestHistory returns a copy of all recorded requests
func (m *MockLSPServer) GetRequestHistory() []LSPRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	history := make([]LSPRequest, len(m.RequestHistory))
	copy(history, m.RequestHistory)
	return history
}

// ClearRequestHistory clears the request history
func (m *MockLSPServer) ClearRequestHistory() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.RequestHistory = m.RequestHistory[:0]
	m.requestCount = 0
}

// SetFailureRate configures the simulated failure rate (0.0 to 1.0)
func (m *MockLSPServer) SetFailureRate(rate float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if rate < 0.0 {
		rate = 0.0
	} else if rate > 1.0 {
		rate = 1.0
	}
	
	m.FailureRate = rate
	
	if m.logger != nil {
		m.logger.WithFields(map[string]interface{}{
			"language":     m.Language,
			"failure_rate": rate,
		}).Info("Updated mock server failure rate")
	}
}

// SimulateLatency configures response delay simulation
func (m *MockLSPServer) SimulateLatency(delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.ResponseDelay = delay
	
	if m.logger != nil {
		m.logger.WithFields(map[string]interface{}{
			"language": m.Language,
			"delay":    delay,
		}).Info("Updated mock server response delay")
	}
}

// SetCustomResponse sets a custom response for a specific LSP method
func (m *MockLSPServer) SetCustomResponse(method string, response interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.responses[method] = response
	
	if m.logger != nil {
		m.logger.WithFields(map[string]interface{}{
			"language": m.Language,
			"method":   method,
		}).Debug("Set custom response for LSP method")
	}
}

// SetRequestHandler sets a custom handler function for a specific LSP method
func (m *MockLSPServer) SetRequestHandler(method string, handler func(interface{}) (interface{}, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.requestHandlers[method] = handler
	
	if m.logger != nil {
		m.logger.WithFields(map[string]interface{}{
			"language": m.Language,
			"method":   method,
		}).Debug("Set custom handler for LSP method")
	}
}

// GetServerMetrics returns comprehensive server metrics
func (m *MockLSPServer) GetServerMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	uptime := time.Duration(0)
	if m.IsRunning {
		uptime = time.Since(m.startTime)
	}
	
	return map[string]interface{}{
		"language":       m.Language,
		"port":           m.Port,
		"is_running":     m.IsRunning,
		"request_count":  m.requestCount,
		"uptime":         uptime,
		"failure_rate":   m.FailureRate,
		"response_delay": m.ResponseDelay,
		"history_size":   len(m.RequestHistory),
	}
}

// handleJSONRPC handles incoming JSON-RPC requests
func (m *MockLSPServer) handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	
	var request struct {
		JSONRPC string      `json:"jsonrpc"`
		ID      interface{} `json:"id,omitempty"`
		Method  string      `json:"method"`
		Params  interface{} `json:"params,omitempty"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		m.writeErrorResponse(w, nil, -32700, "Parse error", err.Error())
		return
	}
	
	if request.JSONRPC != "2.0" {
		m.writeErrorResponse(w, request.ID, -32600, "Invalid Request", "Invalid JSON-RPC version")
		return
	}
	
	result, err := m.HandleRequest(request.Method, request.Params)
	if err != nil {
		m.writeErrorResponse(w, request.ID, -32603, "Internal error", err.Error())
		return
	}
	
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      request.ID,
		"result":  result,
	}
	
	json.NewEncoder(w).Encode(response)
}

// handleHealth handles health check requests
func (m *MockLSPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	health := map[string]interface{}{
		"status":    "healthy",
		"language":  m.Language,
		"uptime":    time.Since(m.startTime).String(),
		"requests":  m.requestCount,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	
	json.NewEncoder(w).Encode(health)
}

// handleCapabilities handles LSP capabilities requests
func (m *MockLSPServer) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	m.mu.RLock()
	capabilities := m.capabilities
	m.mu.RUnlock()
	
	json.NewEncoder(w).Encode(capabilities)
}

// handleRequestHistory handles request history retrieval
func (m *MockLSPServer) handleRequestHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	history := m.GetRequestHistory()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"language": m.Language,
		"count":    len(history),
		"requests": history,
	})
}

// writeErrorResponse writes a JSON-RPC error response
func (m *MockLSPServer) writeErrorResponse(w http.ResponseWriter, id interface{}, code int, message, data string) {
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
			"data":    data,
		},
	}
	
	json.NewEncoder(w).Encode(response)
}

// setupDefaultHandlers configures default request handlers for common LSP methods
func (m *MockLSPServer) setupDefaultHandlers() {
	// Initialize capabilities
	m.requestHandlers["initialize"] = func(params interface{}) (interface{}, error) {
		return map[string]interface{}{
			"capabilities": m.capabilities,
			"serverInfo": map[string]interface{}{
				"name":    fmt.Sprintf("Mock LSP Server (%s)", m.Language),
				"version": "1.0.0-test",
			},
		}, nil
	}
	
	// Initialized notification (no response)
	m.requestHandlers["initialized"] = func(params interface{}) (interface{}, error) {
		return nil, nil
	}
	
	// Shutdown
	m.requestHandlers["shutdown"] = func(params interface{}) (interface{}, error) {
		return nil, nil
	}
	
	// Exit notification
	m.requestHandlers["exit"] = func(params interface{}) (interface{}, error) {
		go func() {
			time.Sleep(100 * time.Millisecond)
			m.Stop()
		}()
		return nil, nil
	}
}

// getDefaultResponse provides fallback responses for LSP methods
func (m *MockLSPServer) getDefaultResponse(method string, params interface{}) (interface{}, error) {
	switch method {
	case mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION:
		return m.createMockDefinition(params)
		
	case mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES:
		return m.createMockReferences(params)
		
	case mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER:
		return m.createMockHover(params)
		
	case mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS:
		return m.createMockDocumentSymbols(params)
		
	case mcp.LSP_METHOD_WORKSPACE_SYMBOL:
		return m.createMockWorkspaceSymbols(params)
		
	case mcp.LSP_METHOD_TEXT_DOCUMENT_COMPLETION:
		return m.createMockCompletion(params)
		
	default:
		return nil, fmt.Errorf("unsupported method: %s", method)
	}
}

// createMockDefinition creates a mock definition response
func (m *MockLSPServer) createMockDefinition(params interface{}) (interface{}, error) {
	return []map[string]interface{}{
		{
			"uri": fmt.Sprintf("file:///mock/%s/definition.%s", m.Language, getFileExtension(m.Language)),
			"range": map[string]interface{}{
				"start": map[string]interface{}{"line": 10, "character": 5},
				"end":   map[string]interface{}{"line": 10, "character": 15},
			},
		},
	}, nil
}

// createMockReferences creates a mock references response
func (m *MockLSPServer) createMockReferences(params interface{}) (interface{}, error) {
	return []map[string]interface{}{
		{
			"uri": fmt.Sprintf("file:///mock/%s/reference1.%s", m.Language, getFileExtension(m.Language)),
			"range": map[string]interface{}{
				"start": map[string]interface{}{"line": 5, "character": 10},
				"end":   map[string]interface{}{"line": 5, "character": 20},
			},
		},
		{
			"uri": fmt.Sprintf("file:///mock/%s/reference2.%s", m.Language, getFileExtension(m.Language)),
			"range": map[string]interface{}{
				"start": map[string]interface{}{"line": 15, "character": 8},
				"end":   map[string]interface{}{"line": 15, "character": 18},
			},
		},
	}, nil
}


// createMockDocumentSymbols creates mock document symbols
func (m *MockLSPServer) createMockDocumentSymbols(params interface{}) (interface{}, error) {
	return []map[string]interface{}{
		{
			"name": fmt.Sprintf("MockFunction_%s", m.Language),
			"kind": 12, // Function symbol kind
			"range": map[string]interface{}{
				"start": map[string]interface{}{"line": 0, "character": 0},
				"end":   map[string]interface{}{"line": 10, "character": 0},
			},
			"selectionRange": map[string]interface{}{
				"start": map[string]interface{}{"line": 0, "character": 0},
				"end":   map[string]interface{}{"line": 0, "character": 20},
			},
		},
	}, nil
}

// createMockWorkspaceSymbols creates mock workspace symbols
func (m *MockLSPServer) createMockWorkspaceSymbols(params interface{}) (interface{}, error) {
	return []map[string]interface{}{
		{
			"name": fmt.Sprintf("GlobalSymbol_%s", m.Language),
			"kind": 13, // Variable symbol kind
			"location": map[string]interface{}{
				"uri": fmt.Sprintf("file:///mock/%s/global.%s", m.Language, getFileExtension(m.Language)),
				"range": map[string]interface{}{
					"start": map[string]interface{}{"line": 0, "character": 0},
					"end":   map[string]interface{}{"line": 0, "character": 15},
				},
			},
		},
	}, nil
}

// createMockCompletion creates mock completion response
func (m *MockLSPServer) createMockCompletion(params interface{}) (interface{}, error) {
	return map[string]interface{}{
		"isIncomplete": false,
		"items": []map[string]interface{}{
			{
				"label":  fmt.Sprintf("mockFunction_%s", m.Language),
				"kind":   3, // Function completion kind
				"detail": fmt.Sprintf("Mock function for %s", m.Language),
			},
			{
				"label":  fmt.Sprintf("mockVariable_%s", m.Language),
				"kind":   6, // Variable completion kind
				"detail": fmt.Sprintf("Mock variable for %s", m.Language),
			},
		},
	}, nil
}

// CreateDefaultCapabilities creates default LSP capabilities for a language
func CreateDefaultCapabilities(language string) map[string]interface{} {
	return map[string]interface{}{
		"textDocumentSync": 1,
		"definitionProvider": true,
		"referencesProvider": true,
		"hoverProvider": true,
		"documentSymbolProvider": true,
		"workspaceSymbolProvider": true,
		"completionProvider": map[string]interface{}{
			"resolveProvider": false,
			"triggerCharacters": getTriggerCharacters(language),
		},
	}
}

// Helper functions
func parseLogLevel(level string) mcp.LogLevel {
	switch level {
	case "trace":
		return mcp.LogLevelTrace
	case "debug":
		return mcp.LogLevelDebug
	case "info":
		return mcp.LogLevelInfo
	case "warn", "warning":
		return mcp.LogLevelWarn
	case "error":
		return mcp.LogLevelError
	case "fatal":
		return mcp.LogLevelFatal
	default:
		return mcp.LogLevelInfo
	}
}

func getFileExtension(language string) string {
	extensions := map[string]string{
		"go":         "go",
		"python":     "py",
		"javascript": "js",
		"typescript": "ts",
		"java":       "java",
		"rust":       "rs",
		"c":          "c",
		"cpp":        "cpp",
	}
	
	if ext, exists := extensions[language]; exists {
		return ext
	}
	return "txt"
}

func getTriggerCharacters(language string) []string {
	triggers := map[string][]string{
		"go":         {".", ":"},
		"python":     {".", ":"},
		"javascript": {".", ":", "(", "["},
		"typescript": {".", ":", "(", "[", "<"},
		"java":       {".", "(", "["},
		"rust":       {".", "::", "("},
		"c":          {".", "->", "("},
		"cpp":        {".", "::", "->", "(", "<"},
	}
	
	if chars, exists := triggers[language]; exists {
		return chars
	}
	return []string{"."}
}

// Fix the typo in createMockHover method receiver
func (m *MockLSPServer) createMockHover(params interface{}) (interface{}, error) {
	return map[string]interface{}{
		"contents": fmt.Sprintf("Mock hover information for %s", m.Language),
		"range": map[string]interface{}{
			"start": map[string]interface{}{"line": 0, "character": 0},
			"end":   map[string]interface{}{"line": 0, "character": 10},
		},
	}, nil
}