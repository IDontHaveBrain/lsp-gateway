package testutil

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// MockLSPServer provides a configurable mock LSP server for testing
type MockLSPServer struct {
	t            *testing.T
	config       *MockLSPServerConfig
	binaryPath   string
	listener     net.Listener
	connections  []*MockConnection
	requestLog   []LSPMessage
	responseLog  []LSPMessage
	requestCount int64
	errorCount   int64
	mu           sync.RWMutex
	isRunning    bool
	stopCh       chan struct{}
}

// MockLSPServerConfig configures mock server behavior
type MockLSPServerConfig struct {
	Language        string
	Transport       string // "stdio", "tcp"
	Port            int    // For TCP transport
	ResponseLatency time.Duration
	ErrorRate       float64 // 0.0 to 1.0
	CustomResponses map[string]interface{}
	LogRequests     bool
	MaxConnections  int
	EnableMetrics   bool
	FailureMode     FailureMode
}

// FailureMode defines how the server should fail
type FailureMode int

const (
	FailureModeNone FailureMode = iota
	FailureModeTimeout
	FailureModeConnectionRefused
	FailureModeInvalidResponse
	FailureModeRandomDisconnect
)

// MockConnection represents a connection to the mock server
type MockConnection struct {
	conn      net.Conn
	reader    *bufio.Reader
	writer    *bufio.Writer
	connected bool
	mu        sync.RWMutex
}

// LSPMessage represents an LSP JSON-RPC message
type LSPMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

// NewMockLSPServer creates a new mock LSP server with specified configuration
func NewMockLSPServer(t *testing.T, config *MockLSPServerConfig) *MockLSPServer {
	if config == nil {
		config = &MockLSPServerConfig{
			Language:        "go",
			Transport:       "stdio",
			ResponseLatency: 1 * time.Millisecond, // Reduce from 10ms to 1ms
			ErrorRate:       0.0,
			LogRequests:     true,
			MaxConnections:  10,
			EnableMetrics:   true,
			FailureMode:     FailureModeNone,
		}
	}

	server := &MockLSPServer{
		t:           t,
		config:      config,
		connections: make([]*MockConnection, 0),
		requestLog:  make([]LSPMessage, 0),
		responseLog: make([]LSPMessage, 0),
		stopCh:      make(chan struct{}),
	}

	server.createBinary()
	return server
}

// createBinary creates the mock LSP server binary
func (s *MockLSPServer) createBinary() {
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("mock-lsp-%s-*", s.config.Language))
	if err != nil {
		s.t.Fatalf("Failed to create temp directory: %v", err)
	}

	source := s.generateMockServerSource()
	sourceFile := filepath.Join(tempDir, "mock_server.go")

	if err := os.WriteFile(sourceFile, []byte(source), 0644); err != nil {
		s.t.Fatalf("Failed to write mock server source: %v", err)
	}

	s.binaryPath = filepath.Join(tempDir, "mock_server")
	cmd := exec.Command("go", "build", "-o", s.binaryPath, sourceFile)
	if err := cmd.Run(); err != nil {
		s.t.Fatalf("Failed to compile mock LSP server: %v", err)
	}
}

// generateMockServerSource generates the Go source code for the mock server
func (s *MockLSPServer) generateMockServerSource() string {
	return fmt.Sprintf(`package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const JSONRPCVersion = "2.0"

type LSPMessage struct {
	JSONRPC string      %sjson:"jsonrpc"%s
	ID      interface{} %sjson:"id,omitempty"%s
	Method  string      %sjson:"method,omitempty"%s
	Params  interface{} %sjson:"params,omitempty"%s
	Result  interface{} %sjson:"result,omitempty"%s
	Error   interface{} %sjson:"error,omitempty"%s
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "tcp" {
		runTCPServer()
	} else {
		runStdioServer()
	}
}

func runStdioServer() {
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()

	for {
		msg, err := readMessage(reader)
		if err != nil {
			if err == io.EOF {
				return
			}
			continue
		}

		response := handleMessage(msg)
		if response != nil {
			writeMessage(writer, *response)
		}
	}
}

func runTCPServer() {
	port := "%d"
	if port == "0" {
		port = "0" // Let system choose
	}
	
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to listen: %%v\n", err)
		os.Exit(1)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	defer writer.Flush()

	for {
		msg, err := readMessage(reader)
		if err != nil {
			return
		}

		response := handleMessage(msg)
		if response != nil {
			writeMessage(writer, *response)
		}
	}
}

func readMessage(reader *bufio.Reader) (*LSPMessage, error) {
	var contentLength int
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			break
		}

		if strings.HasPrefix(line, "Content-Length: ") {
			lengthStr := strings.TrimPrefix(line, "Content-Length: ")
			if n, err := strconv.Atoi(lengthStr); err == nil {
				contentLength = n
			}
		}
	}

	if contentLength <= 0 {
		return nil, fmt.Errorf("no content length")
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, err
	}

	var msg LSPMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

func writeMessage(writer *bufio.Writer, msg LSPMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("Content-Length: %%d\r\n\r\n", len(data))
	if _, err := writer.WriteString(header); err != nil {
		return err
	}
	if _, err := writer.Write(data); err != nil {
		return err
	}
	return writer.Flush()
}

func handleMessage(msg *LSPMessage) *LSPMessage {
	// Simulate response latency
	if latency := %d; latency > 0 {
		time.Sleep(time.Duration(latency) * time.Millisecond)
	}

	// Simulate error rate
	if errorRate := %.2f; errorRate > 0 && rand.Float64() < errorRate {
		return &LSPMessage{
			JSONRPC: JSONRPCVersion,
			ID:      msg.ID,
			Error: map[string]interface{}{
				"code":    -32603,
				"message": "Internal error (simulated)",
			},
		}
	}

	switch msg.Method {
	case "initialize":
		return &LSPMessage{
			JSONRPC: JSONRPCVersion,
			ID:      msg.ID,
			Result: map[string]interface{}{
				"capabilities": map[string]interface{}{
					"definitionProvider": true,
					"referencesProvider": true,
					"documentSymbolProvider": true,
					"workspaceSymbolProvider": true,
					"hoverProvider": true,
				},
				"serverInfo": map[string]interface{}{
					"name":    "mock-%s-lsp",
					"version": "1.0.0",
				},
			},
		}
	case "initialized":
		return nil // No response for notification
	case "textDocument/definition":
		return &LSPMessage{
			JSONRPC: JSONRPCVersion,
			ID:      msg.ID,
			Result: map[string]interface{}{
				"uri": extractURI(msg.Params),
				"method": msg.Method,
				"range": map[string]interface{}{
					"start": map[string]interface{}{"line": 5, "character": 0},
					"end":   map[string]interface{}{"line": 5, "character": 10},
				},
			},
		}
	case "textDocument/hover":
		return &LSPMessage{
			JSONRPC: JSONRPCVersion,
			ID:      msg.ID,
			Result: map[string]interface{}{
				"uri": extractURI(msg.Params),
				"method": msg.Method,
				"contents": map[string]interface{}{
					"kind":  "markdown",
					"value": fmt.Sprintf("Mock hover information for %%s", "%s"),
				},
				"range": map[string]interface{}{
					"start": map[string]interface{}{"line": 7, "character": 12},
					"end":   map[string]interface{}{"line": 7, "character": 25},
				},
			},
		}
	case "textDocument/references":
		return &LSPMessage{
			JSONRPC: JSONRPCVersion,
			ID:      msg.ID,
			Result: []map[string]interface{}{
				{
					"uri": extractURI(msg.Params),
					"range": map[string]interface{}{
						"start": map[string]interface{}{"line": 5, "character": 0},
						"end":   map[string]interface{}{"line": 5, "character": 10},
					},
				},
			},
		}
	case "textDocument/documentSymbol":
		return &LSPMessage{
			JSONRPC: JSONRPCVersion,
			ID:      msg.ID,
			Result: []map[string]interface{}{
				{
					"name": fmt.Sprintf("Mock%%sSymbol", "%s"),
					"kind": 12, // Function
					"range": map[string]interface{}{
						"start": map[string]interface{}{"line": 0, "character": 0},
						"end":   map[string]interface{}{"line": 10, "character": 0},
					},
					"selectionRange": map[string]interface{}{
						"start": map[string]interface{}{"line": 0, "character": 0},
						"end":   map[string]interface{}{"line": 0, "character": 10},
					},
				},
			},
		}
	case "workspace/symbol":
		return &LSPMessage{
			JSONRPC: JSONRPCVersion,
			ID:      msg.ID,
			Result: []map[string]interface{}{
				{
					"name": fmt.Sprintf("Mock%%sWorkspaceSymbol", "%s"),
					"kind": 12, // Function
					"location": map[string]interface{}{
						"uri": fmt.Sprintf("file:///mock.%%s", "%s"),
						"range": map[string]interface{}{
							"start": map[string]interface{}{"line": 0, "character": 0},
							"end":   map[string]interface{}{"line": 0, "character": 10},
						},
					},
				},
			},
		}
	default:
		return &LSPMessage{
			JSONRPC: JSONRPCVersion,
			ID:      msg.ID,
			Error: map[string]interface{}{
				"code":    -32601,
				"message": "Method not found",
			},
		}
	}
}

func extractURI(params interface{}) string {
	if params == nil {
		return ""
	}
	
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return ""
	}
	
	if textDoc, exists := paramsMap["textDocument"]; exists {
		if textDocMap, ok := textDoc.(map[string]interface{}); ok {
			if uri, exists := textDocMap["uri"]; exists {
				if uriStr, ok := uri.(string); ok {
					return uriStr
				}
			}
		}
	}
	
	return ""
}
`, "`", "`", "`", "`", "`", "`", "`", "`", "`", "`", "`", "`",
		s.config.Port,
		int(s.config.ResponseLatency.Milliseconds()),
		s.config.ErrorRate,
		s.config.Language,
		s.config.Language,
		cases.Title(language.Und).String(s.config.Language),
		s.config.Language,
		getFileExtension(s.config.Language))
}

// getFileExtension returns the file extension for a language
func getFileExtension(language string) string {
	switch language {
	case "go":
		return "go"
	case "python":
		return "py"
	case "typescript":
		return "ts"
	case "javascript":
		return "js"
	case "java":
		return "java"
	default:
		return "txt"
	}
}

// BinaryPath returns the path to the compiled mock server binary
func (s *MockLSPServer) BinaryPath() string {
	return s.binaryPath
}

// Start starts the mock server (if using TCP transport)
func (s *MockLSPServer) Start() error {
	if s.config.Transport != "tcp" {
		return nil // stdio servers are started by the gateway
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.config.Port))
	if err != nil {
		return fmt.Errorf("failed to start TCP server: %w", err)
	}

	s.listener = listener
	s.isRunning = true

	go s.acceptConnections()
	return nil
}

// acceptConnections handles incoming TCP connections
func (s *MockLSPServer) acceptConnections() {
	for {
		select {
		case <-s.stopCh:
			return
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			continue
		}

		mockConn := &MockConnection{
			conn:      conn,
			reader:    bufio.NewReader(conn),
			writer:    bufio.NewWriter(conn),
			connected: true,
		}

		s.mu.Lock()
		s.connections = append(s.connections, mockConn)
		s.mu.Unlock()

		go s.handleConnection(mockConn)
	}
}

// handleConnection processes messages from a single connection
func (s *MockLSPServer) handleConnection(conn *MockConnection) {
	defer func() {
		conn.mu.Lock()
		conn.connected = false
		_ = conn.conn.Close()
		conn.mu.Unlock()
	}()

	for {
		select {
		case <-s.stopCh:
			return
		default:
		}

		msg, err := s.readMessage(conn.reader)
		if err != nil {
			return
		}

		if s.config.LogRequests {
			s.mu.Lock()
			s.requestLog = append(s.requestLog, *msg)
			s.mu.Unlock()
		}

		atomic.AddInt64(&s.requestCount, 1)

		response := s.handleMessage(msg)
		if response != nil {
			if s.config.LogRequests {
				s.mu.Lock()
				s.responseLog = append(s.responseLog, *response)
				s.mu.Unlock()
			}

			if err := s.writeMessage(conn.writer, *response); err != nil {
				atomic.AddInt64(&s.errorCount, 1)
				return
			}
		}
	}
}

// readMessage reads an LSP message from the connection
func (s *MockLSPServer) readMessage(reader *bufio.Reader) (*LSPMessage, error) {
	var contentLength int
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			break
		}

		if strings.HasPrefix(line, "Content-Length: ") {
			lengthStr := strings.TrimPrefix(line, "Content-Length: ")
			if n, err := strconv.Atoi(lengthStr); err == nil {
				contentLength = n
			}
		}
	}

	if contentLength <= 0 {
		return nil, fmt.Errorf("no content length")
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, err
	}

	var msg LSPMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

// writeMessage writes an LSP message to the connection
func (s *MockLSPServer) writeMessage(writer *bufio.Writer, msg LSPMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := writer.WriteString(header); err != nil {
		return err
	}
	if _, err := writer.Write(data); err != nil {
		return err
	}
	return writer.Flush()
}

// handleMessage processes an LSP message and returns a response
func (s *MockLSPServer) handleMessage(msg *LSPMessage) *LSPMessage {
	// Simulate response latency
	if s.config.ResponseLatency > 0 {
		time.Sleep(s.config.ResponseLatency)
	}

	// Check for custom responses
	if s.config.CustomResponses != nil {
		if customResp, exists := s.config.CustomResponses[msg.Method]; exists {
			return &LSPMessage{
				JSONRPC: "2.0",
				ID:      msg.ID,
				Result:  customResp,
			}
		}
	}

	// Apply failure modes
	switch s.config.FailureMode {
	case FailureModeTimeout:
		time.Sleep(time.Minute) // Simulate timeout
	case FailureModeInvalidResponse:
		return &LSPMessage{
			JSONRPC: "1.0", // Invalid version
			ID:      "invalid",
			Result:  "not a valid response",
		}
	}

	// Default handling (same as generated source)
	switch msg.Method {
	case "initialize":
		return &LSPMessage{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Result: map[string]interface{}{
				"capabilities": map[string]interface{}{
					"definitionProvider":      true,
					"referencesProvider":      true,
					"documentSymbolProvider":  true,
					"workspaceSymbolProvider": true,
					"hoverProvider":           true,
				},
			},
		}
	case "initialized":
		return nil
	default:
		return &LSPMessage{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Error: map[string]interface{}{
				"code":    -32601,
				"message": "Method not found",
			},
		}
	}
}

// Stop stops the mock server and cleans up resources
func (s *MockLSPServer) Stop() {
	if !s.isRunning {
		return
	}

	close(s.stopCh)

	if s.listener != nil {
		_ = s.listener.Close()
	}

	s.mu.Lock()
	for _, conn := range s.connections {
		conn.mu.Lock()
		if conn.connected {
			_ = conn.conn.Close()
		}
		conn.mu.Unlock()
	}
	s.connections = nil
	s.mu.Unlock()

	s.isRunning = false
}

// GetRequestCount returns the total number of requests received
func (s *MockLSPServer) GetRequestCount() int64 {
	return atomic.LoadInt64(&s.requestCount)
}

// GetErrorCount returns the total number of errors encountered
func (s *MockLSPServer) GetErrorCount() int64 {
	return atomic.LoadInt64(&s.errorCount)
}

// GetRequestLog returns a copy of all logged requests
func (s *MockLSPServer) GetRequestLog() []LSPMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	log := make([]LSPMessage, len(s.requestLog))
	copy(log, s.requestLog)
	return log
}

// GetResponseLog returns a copy of all logged responses
func (s *MockLSPServer) GetResponseLog() []LSPMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	log := make([]LSPMessage, len(s.responseLog))
	copy(log, s.responseLog)
	return log
}

// ClearLogs clears all request and response logs
func (s *MockLSPServer) ClearLogs() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.requestLog = s.requestLog[:0]
	s.responseLog = s.responseLog[:0]
}
