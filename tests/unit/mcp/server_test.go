package mcp_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"lsp-gateway/mcp"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type LSPClientInterface interface {
	SendLSPRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error)
	GetMetrics() mcp.ConnectionMetrics
	GetHealth(ctx context.Context) error
	IsHealthy() bool
}

type ToolHandlerInterface interface {
	ListTools() []mcp.Tool
	CallTool(ctx context.Context, call mcp.ToolCall) (*mcp.ToolResult, error)
}

type MockLSPGatewayClient struct {
	mu           sync.RWMutex
	requestCount int64
	responses    map[string]json.RawMessage
	errors       map[string]error
	delays       map[string]time.Duration
	metrics      mcp.ConnectionMetrics
	healthy      bool
	failCount    int
	maxFails     int
}

func NewMockLSPGatewayClient() *MockLSPGatewayClient {
	return &MockLSPGatewayClient{
		responses: make(map[string]json.RawMessage),
		errors:    make(map[string]error),
		delays:    make(map[string]time.Duration),
		healthy:   true,
		maxFails:  -1, // No limit by default
	}
}

func (m *MockLSPGatewayClient) SendLSPRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	atomic.AddInt64(&m.requestCount, 1)

	m.mu.RLock()
	defer m.mu.RUnlock()

	if delay, exists := m.delays[method]; exists {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.maxFails >= 0 && m.failCount < m.maxFails {
		m.failCount++
		return nil, errors.New("mock failure")
	}

	if err, exists := m.errors[method]; exists {
		return nil, err
	}

	if response, exists := m.responses[method]; exists {
		return response, nil
	}

	defaultResponse := map[string]interface{}{
		"success": true,
		"method":  method,
		"params":  params,
	}
	data, _ := json.Marshal(defaultResponse)
	return json.RawMessage(data), nil
}

func (m *MockLSPGatewayClient) SetResponse(method string, response interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, _ := json.Marshal(response)
	m.responses[method] = json.RawMessage(data)
}

func (m *MockLSPGatewayClient) SetError(method string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.errors[method] = err
}

func (m *MockLSPGatewayClient) SetDelay(method string, delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.delays[method] = delay
}

func (m *MockLSPGatewayClient) GetRequestCount() int64 {
	return atomic.LoadInt64(&m.requestCount)
}

func (m *MockLSPGatewayClient) GetMetrics() mcp.ConnectionMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return mcp.ConnectionMetrics{
		TotalRequests:    m.metrics.TotalRequests,
		SuccessfulReqs:   m.metrics.SuccessfulReqs,
		FailedRequests:   m.metrics.FailedRequests,
		TimeoutCount:     m.metrics.TimeoutCount,
		ConnectionErrors: m.metrics.ConnectionErrors,
		AverageLatency:   m.metrics.AverageLatency,
		LastRequestTime:  m.metrics.LastRequestTime,
		LastSuccessTime:  m.metrics.LastSuccessTime,
	}
}

func (m *MockLSPGatewayClient) GetHealth(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.healthy {
		return errors.New("mock unhealthy")
	}
	return nil
}

func (m *MockLSPGatewayClient) IsHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.healthy
}

func (m *MockLSPGatewayClient) SetHealthy(healthy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthy = healthy
}

func (m *MockLSPGatewayClient) SetMaxFails(maxFails int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxFails = maxFails
	m.failCount = 0
}

func createTestServer() *mcp.Server {
	config := &mcp.ServerConfig{
		Name:          "test-server",
		Version:       "1.0.0",
		LSPGatewayURL: "http://localhost:8080",
		Timeout:       30 * time.Second,
		MaxRetries:    3,
	}
	return mcp.NewServer(config)
}

func createTestMessage(id interface{}, method string, params interface{}) string {
	msg := mcp.MCPMessage{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      id,
		Method:  method,
		Params:  params,
	}
	data, _ := json.Marshal(msg)
	content := string(data)
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(content), content)
}

func parseResponseMessage(output string) (*mcp.MCPMessage, error) {
	jsonStart := strings.Index(output, "\r\n\r\n")
	if jsonStart == -1 {
		return nil, errors.New("no double CRLF found")
	}

	jsonContent := output[jsonStart+4:]
	var msg mcp.MCPMessage
	err := json.Unmarshal([]byte(jsonContent), &msg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &msg, nil
}

func TestNewServer(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		config := &mcp.ServerConfig{
			Name:          "test-server",
			Version:       "1.0.0",
			LSPGatewayURL: "http://localhost:8080",
			Timeout:       30 * time.Second,
			MaxRetries:    3,
		}

		server := mcp.NewServer(config)

		if server == nil {
			t.Fatal("Expected server to be created")
		}

		if server.Config != config {
			t.Error("Expected config to be set")
		}

		if server.Client == nil {
			t.Error("Expected client to be initialized")
		}

		if server.ToolHandler == nil {
			t.Error("Expected tool handler to be initialized")
		}

		if server.ProtocolLimits == nil {
			t.Error("Expected protocol limits to be initialized")
		}

		if server.RecoveryContext == nil {
			t.Error("Expected recovery context to be initialized")
		}

		if server.Logger == nil {
			t.Error("Expected logger to be initialized")
		}

		if server.Input != os.Stdin {
			t.Error("Expected default input to be os.Stdin")
		}

		if server.Output != os.Stdout {
			t.Error("Expected default output to be os.Stdout")
		}
	})

	t.Run("NilConfig", func(t *testing.T) {
		server := mcp.NewServer(nil)

		if server == nil {
			t.Fatal("Expected server to be created with default config")
		}

		if server.Config == nil {
			t.Error("Expected default config to be set")
		}

		if server.Config.Name != "lsp-gateway-mcp" {
			t.Errorf("Expected default name, got: %s", server.Config.Name)
		}
	})

	t.Run("ProtocolLimits", func(t *testing.T) {
		server := createTestServer()

		limits := server.ProtocolLimits
		if limits.MaxMessageSize != 10*1024*1024 {
			t.Errorf("Expected MaxMessageSize 10MB, got: %d", limits.MaxMessageSize)
		}

		if limits.MaxHeaderSize != 4096 {
			t.Errorf("Expected MaxHeaderSize 4KB, got: %d", limits.MaxHeaderSize)
		}

		if limits.MaxMethodLength != 256 {
			t.Errorf("Expected MaxMethodLength 256, got: %d", limits.MaxMethodLength)
		}

		if limits.MessageTimeout != 30*time.Second {
			t.Errorf("Expected MessageTimeout 30s, got: %v", limits.MessageTimeout)
		}
	})
}

func TestServerLifecycle(t *testing.T) {
	t.Run("StartStop", func(t *testing.T) {
		server := createTestServer()

		if !server.IsRunning() {
			t.Error("Expected server to be running initially")
		}

		inputReader := strings.NewReader("")
		outputBuffer := &bytes.Buffer{}
		server.SetIO(inputReader, outputBuffer)

		done := make(chan error, 1)
		go func() {
			done <- server.Start()
		}()

		time.Sleep(10 * time.Millisecond)

		err := server.Stop()
		if err != nil {
			t.Errorf("Unexpected error stopping server: %v", err)
		}

		select {
		case startErr := <-done:
			if startErr != nil {
				t.Errorf("Unexpected error from Start(): %v", startErr)
			}
		case <-time.After(1 * time.Second):
			t.Error("Server did not stop within timeout")
		}

		if server.IsRunning() {
			t.Error("Expected server to be stopped")
		}
	})

	t.Run("IsRunning", func(t *testing.T) {
		server := createTestServer()

		if !server.IsRunning() {
			t.Error("Expected new server to be running")
		}

		server.Cancel()

		time.Sleep(10 * time.Millisecond)

		if server.IsRunning() {
			t.Error("Expected server to be stopped after cancellation")
		}
	})

	t.Run("MultipleStopCalls", func(t *testing.T) {
		server := createTestServer()
		server.SetIO(strings.NewReader(""), &bytes.Buffer{})

		go func() {
			_ = server.Start()
		}()

		time.Sleep(10 * time.Millisecond)

		err1 := server.Stop()
		err2 := server.Stop()
		err3 := server.Stop()

		if err1 != nil || err2 != nil || err3 != nil {
			t.Error("Multiple stop calls should not return errors")
		}
	})
}

func TestSetIO(t *testing.T) {
	server := createTestServer()

	inputReader := strings.NewReader("test input")
	outputBuffer := &bytes.Buffer{}

	server.SetIO(inputReader, outputBuffer)

	if server.Input != inputReader {
		t.Error("Expected input to be set")
	}

	if server.Output != outputBuffer {
		t.Error("Expected output to be set")
	}
}

func TestMessageReading(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectError   bool
		errorContains string
	}{
		{
			name:  "ValidMessage",
			input: createTestMessage(1, "initialize", nil),
		},
		{
			name:          "InvalidContentLength",
			input:         "Content-Length: invalid\r\n\r\n{}",
			expectError:   true,
			errorContains: "invalid Content-Length value",
		},
		{
			name:          "NegativeContentLength",
			input:         "Content-Length: -1\r\n\r\n{}",
			expectError:   true,
			errorContains: "negative Content-Length",
		},
		{
			name:          "MissingContentLength",
			input:         "\r\n{}",
			expectError:   true,
			errorContains: "missing or zero Content-Length header",
		},
		{
			name:          "TooLargeMessage",
			input:         "Content-Length: 20971520\r\n\r\n{}",
			expectError:   true,
			errorContains: "message size too large",
		},
		{
			name:          "TooManyHeaderLines",
			input:         strings.Repeat("Header: value\r\n", 25) + "\r\n{}",
			expectError:   true,
			errorContains: "too many header lines",
		},
		{
			name:          "InvalidUTF8InHeader",
			input:         "Content-Length: 2\r\nInvalid: \xff\xff\r\n\r\n{}",
			expectError:   true,
			errorContains: "invalid UTF-8 encoding in header",
		},
		{
			name:          "InvalidUTF8InContent",
			input:         "Content-Length: 2\r\n\r\n\xff\xff",
			expectError:   true,
			errorContains: "invalid UTF-8 encoding in message content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createTestServer()
			reader := bufio.NewReader(strings.NewReader(tt.input))

			message, err := server.ReadMessageWithRecovery(reader)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if message == "" {
					t.Error("Expected non-empty message")
				}
			}
		})
	}
}

func TestMessageValidation(t *testing.T) {
	server := createTestServer()

	tests := []struct {
		name        string
		message     mcp.MCPMessage
		expectError bool
		errorField  string
	}{
		{
			name: "ValidRequest",
			message: mcp.MCPMessage{
				JSONRPC: mcp.JSONRPCVersion,
				ID:      1,
				Method:  "initialize",
				Params:  map[string]interface{}{"test": true},
			},
		},
		{
			name: "ValidResponse",
			message: mcp.MCPMessage{
				JSONRPC: mcp.JSONRPCVersion,
				ID:      1,
				Result:  map[string]interface{}{"success": true},
			},
		},
		{
			name: "InvalidJSONRPCVersion",
			message: mcp.MCPMessage{
				JSONRPC: "1.0",
				ID:      1,
				Method:  "test",
			},
			expectError: true,
			errorField:  "jsonrpc",
		},
		{
			name: "MethodTooLong",
			message: mcp.MCPMessage{
				JSONRPC: mcp.JSONRPCVersion,
				ID:      1,
				Method:  strings.Repeat("a", 300),
			},
			expectError: true,
			errorField:  "method",
		},
		{
			name: "InvalidMethodName",
			message: mcp.MCPMessage{
				JSONRPC: mcp.JSONRPCVersion,
				ID:      1,
				Method:  "invalid@method!name",
			},
			expectError: true,
			errorField:  "method",
		},
		{
			name: "BothRequestAndResponse",
			message: mcp.MCPMessage{
				JSONRPC: mcp.JSONRPCVersion,
				ID:      1,
				Method:  "test",
				Result:  map[string]interface{}{"test": true},
			},
			expectError: true,
			errorField:  "message_type",
		},
		{
			name: "NeitherRequestNorResponse",
			message: mcp.MCPMessage{
				JSONRPC: mcp.JSONRPCVersion,
				ID:      1,
			},
			expectError: true,
			errorField:  "message_type",
		},
		{
			name: "MissingRequiredID",
			message: mcp.MCPMessage{
				JSONRPC: mcp.JSONRPCVersion,
				Method:  "initialize",
			},
			expectError: true,
			errorField:  "id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := server.ValidateMessageStructure(&tt.message)

			if tt.expectError {
				if err == nil {
					t.Error("Expected validation error but got none")
				} else if validationErr, ok := err.(*MessageValidationError); ok {
					if validationErr.Field != tt.errorField {
						t.Errorf("Expected error field '%s', got: '%s'", tt.errorField, validationErr.Field)
					}
				} else {
					t.Errorf("Expected MessageValidationError, got: %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected validation error: %v", err)
				}
			}
		})
	}
}

func TestMessageHandling(t *testing.T) {
	server := createTestServer()

	tests := []struct {
		name           string
		messageData    string
		expectResponse bool
		setupMocks     func()
	}{
		{
			name:           "Initialize",
			messageData:    `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
			expectResponse: true,
		},
		{
			name:           "ListTools",
			messageData:    `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
			expectResponse: true,
		},
		{
			name:           "CallTool",
			messageData:    `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"goto_definition","arguments":{"uri":"file:///test.go","line":1,"character":1}}}`,
			expectResponse: true,
		},
		{
			name:           "Ping",
			messageData:    `{"jsonrpc":"2.0","id":4,"method":"ping"}`,
			expectResponse: true,
		},
		{
			name:           "InitializedNotification",
			messageData:    `{"jsonrpc":"2.0","method":"notifications/initialized"}`,
			expectResponse: false,
		},
		{
			name:           "UnknownMethod",
			messageData:    `{"jsonrpc":"2.0","id":5,"method":"unknown"}`,
			expectResponse: true,
		},
		{
			name:           "InvalidJSON",
			messageData:    `{"jsonrpc":"2.0","id":6,"method":}`,
			expectResponse: true,
		},
		{
			name:           "EmptyMessage",
			messageData:    ``,
			expectResponse: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputBuffer := &bytes.Buffer{}
			server.SetIO(nil, outputBuffer)

			if tt.setupMocks != nil {
				tt.setupMocks()
			}

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			err := server.HandleMessageWithValidation(ctx, tt.messageData)

			if err != nil {
				t.Logf("Handler returned error (may be expected): %v", err)
			}

			output := outputBuffer.String()
			hasResponse := strings.Contains(output, "Content-Length:")

			if tt.expectResponse && !hasResponse {
				t.Errorf("Expected response but got none. Output: %s", output)
			} else if !tt.expectResponse && hasResponse {
				t.Errorf("Expected no response but got one. Output: %s", output)
			}
		})
	}
}

func TestResponseSending(t *testing.T) {
	server := createTestServer()

	t.Run("SendResponse", func(t *testing.T) {
		outputBuffer := &bytes.Buffer{}
		server.SetIO(nil, outputBuffer)

		result := map[string]interface{}{
			"test": "value",
		}

		err := server.SendResponse(123, result)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		output := outputBuffer.String()
		response, err := parseResponseMessage(output)
		if err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response.JSONRPC != mcp.JSONRPCVersion {
			t.Errorf("Expected JSONRPC %s, got: %s", mcp.JSONRPCVersion, response.JSONRPC)
		}

		if response.ID != 123 {
			t.Errorf("Expected ID 123, got: %v", response.ID)
		}

		if response.Result == nil {
			t.Error("Expected result to be set")
		}
	})

	t.Run("SendError", func(t *testing.T) {
		outputBuffer := &bytes.Buffer{}
		server.SetIO(nil, outputBuffer)

		err := server.SendError(456, -32603, "Internal error", "Test error data")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		output := outputBuffer.String()
		response, err := parseResponseMessage(output)
		if err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response.Error == nil {
			t.Fatal("Expected error to be set")
		}

		if response.Error.Code != -32603 {
			t.Errorf("Expected error code -32603, got: %d", response.Error.Code)
		}

		if response.Error.Message != "Internal error" {
			t.Errorf("Expected error message 'Internal error', got: %s", response.Error.Message)
		}
	})

	t.Run("SendMessageTooLarge", func(t *testing.T) {
		outputBuffer := &bytes.Buffer{}
		server.SetIO(nil, outputBuffer)

		largeData := strings.Repeat("a", 15*1024*1024) // 15MB
		result := map[string]interface{}{
			"data": largeData,
		}

		err := server.SendResponse(1, result)
		if err == nil {
			t.Error("Expected error for message too large")
		}

		if !strings.Contains(err.Error(), "outgoing message too large") {
			t.Errorf("Expected 'outgoing message too large' error, got: %v", err)
		}
	})
}

func TestRecoveryContext(t *testing.T) {
	server := createTestServer()

	t.Run("UpdateRecoveryContext", func(t *testing.T) {
		server.UpdateRecoveryContext("malformed")
		server.UpdateRecoveryContext("malformed")
		server.UpdateRecoveryContext("malformed")

		metrics := server.GetRecoveryMetrics()
		if metrics["malformed_count"] != 3 {
			t.Errorf("Expected malformed_count 3, got: %v", metrics["malformed_count"])
		}

		if !metrics["recovery_mode"].(bool) {
			t.Error("Expected recovery mode to be enabled after 3 malformed errors")
		}
	})

	t.Run("ParseErrorTracking", func(t *testing.T) {
		server := createTestServer()

		for i := 0; i < 6; i++ {
			server.UpdateRecoveryContext("parse_error")
		}

		metrics := server.GetRecoveryMetrics()
		if metrics["parse_errors"] != 6 {
			t.Errorf("Expected parse_errors 6, got: %v", metrics["parse_errors"])
		}

		if !metrics["recovery_mode"].(bool) {
			t.Error("Expected recovery mode to be enabled after 5+ parse errors")
		}
	})

	t.Run("RecoveryModeExit", func(t *testing.T) {
		server := createTestServer()

		server.RecoveryContext.RecoveryMode = true
		server.RecoveryContext.RecoveryStart = time.Now().Add(-70 * time.Second) // 70 seconds ago
		server.RecoveryContext.MalformedCount = 5
		server.RecoveryContext.ParseErrors = 5

		server.UpdateRecoveryContext("malformed")

		metrics := server.GetRecoveryMetrics()
		if metrics["recovery_mode"].(bool) {
			t.Error("Expected recovery mode to be disabled after timeout")
		}

		if metrics["malformed_count"] != 0 {
			t.Error("Expected malformed_count to be reset after recovery")
		}
	})
}

func TestAttemptRecovery(t *testing.T) {
	server := createTestServer()

	t.Run("SuccessfulRecovery", func(t *testing.T) {
		input := "garbage data\nmore garbage\nContent-Length: 20\r\n\r\nsomething"
		reader := bufio.NewReader(strings.NewReader(input))

		originalErr := errors.New("test error")
		success := server.AttemptRecovery(reader, originalErr)

		if !success {
			t.Error("Expected recovery to succeed when Content-Length is found")
		}
	})

	t.Run("FailedRecovery", func(t *testing.T) {
		input := "garbage data\nmore garbage\nno content length here"
		reader := bufio.NewReader(strings.NewReader(input))

		originalErr := errors.New("test error")
		success := server.AttemptRecovery(reader, originalErr)

		if success {
			t.Error("Expected recovery to fail when Content-Length is not found")
		}
	})
}

func TestMessageLoop(t *testing.T) {
	t.Run("NormalOperation", func(t *testing.T) {
		server := createTestServer()

		input := createTestMessage(1, "ping", nil)
		inputReader := strings.NewReader(input)
		outputBuffer := &bytes.Buffer{}
		server.SetIO(inputReader, outputBuffer)

		done := make(chan struct{})
		go func() {
			defer close(done)
			_ = server.Start()
		}()

		select {
		case <-done:
		case <-time.After(1 * time.Second):
			t.Error("Server did not exit on EOF within timeout")
		}

		output := outputBuffer.String()
		if !strings.Contains(output, "Content-Length:") {
			t.Error("Expected response to be sent")
		}
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		server := createTestServer()

		pr, pw := io.Pipe()
		outputBuffer := &bytes.Buffer{}
		server.SetIO(pr, outputBuffer)

		done := make(chan struct{})
		go func() {
			defer close(done)
			_ = server.Start()
		}()

		time.Sleep(10 * time.Millisecond)

		server.Cancel()

		_ = pw.Close()

		select {
		case <-done:
		case <-time.After(1 * time.Second):
			t.Error("Server did not exit on context cancellation within timeout")
		}
	})

	t.Run("ConsecutiveErrors", func(t *testing.T) {
		server := createTestServer()

		malformedInput := "Content-Length: invalid\r\n\r\n{}\n" +
			"Content-Length: -1\r\n\r\n{}\n" +
			"garbage\n" +
			strings.Repeat("bad data\n", 20)

		inputReader := strings.NewReader(malformedInput)
		outputBuffer := &bytes.Buffer{}
		server.SetIO(inputReader, outputBuffer)

		done := make(chan struct{})
		go func() {
			defer close(done)
			_ = server.Start()
		}()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("Server did not exit after consecutive errors within timeout")
			server.Cancel()
		}
	})
}

func TestConcurrentAccess(t *testing.T) {
	server := createTestServer()

	t.Run("ConcurrentIsRunning", func(t *testing.T) {
		var wg sync.WaitGroup
		results := make([]bool, 100)

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				results[idx] = server.IsRunning()
			}(i)
		}

		wg.Wait()

		for i, result := range results {
			if !result {
				t.Errorf("Expected result %d to be true", i)
			}
		}
	})

	t.Run("ConcurrentRecoveryUpdates", func(t *testing.T) {
		var wg sync.WaitGroup

		for i := 0; i < 50; i++ {
			wg.Add(2)
			go func() {
				defer wg.Done()
				server.UpdateRecoveryContext("malformed")
			}()
			go func() {
				defer wg.Done()
				server.UpdateRecoveryContext("parse_error")
			}()
		}

		wg.Wait()

		metrics := server.GetRecoveryMetrics()
		malformedCount := metrics["malformed_count"].(int)
		parseErrors := metrics["parse_errors"].(int)

		if malformedCount != 50 {
			t.Errorf("Expected malformed_count 50, got: %d", malformedCount)
		}

		if parseErrors != 50 {
			t.Errorf("Expected parse_errors 50, got: %d", parseErrors)
		}
	})
}

func TestIntegration(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jsonrpc" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result":  map[string]interface{}{"test": "success"},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer mockServer.Close()

	t.Run("EndToEndFlow", func(t *testing.T) {
		config := &mcp.ServerConfig{
			Name:          "test-server",
			Version:       "1.0.0",
			LSPGatewayURL: mockServer.URL,
			Timeout:       5 * time.Second,
			MaxRetries:    3,
		}

		server := mcp.NewServer(config)

		messages := []string{
			createTestMessage(1, "initialize", map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{},
			}),
			createTestMessage(2, "tools/list", nil),
			createTestMessage(3, "tools/call", map[string]interface{}{
				"name":      "goto_definition",
				"arguments": map[string]interface{}{"uri": "file:///test.go", "line": 10, "character": 5},
			}),
		}

		input := strings.Join(messages, "")
		inputReader := strings.NewReader(input)
		outputBuffer := &bytes.Buffer{}
		server.SetIO(inputReader, outputBuffer)

		done := make(chan struct{})
		go func() {
			defer close(done)
			_ = server.Start()
		}()

		select {
		case <-done:
			output := outputBuffer.String()
			responseCount := strings.Count(output, "Content-Length:")
			if responseCount < 3 {
				t.Errorf("Expected at least 3 responses, got %d. Output: %s", responseCount, output)
			}
		case <-time.After(5 * time.Second):
			t.Error("Integration test did not complete within timeout")
			server.Cancel()
		}
	})
}

func TestErrorScenarios(t *testing.T) {
	t.Run("ToolHandlerError", func(t *testing.T) {
		server := createTestServer()
		server.Initialized = true // Ensure server is initialized for tool calls

		outputBuffer := &bytes.Buffer{}
		server.SetIO(nil, outputBuffer)

		ctx := context.Background()
		messageData := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"invalid_tool","arguments":{}}}`

		err := server.handleMessageWithValidation(ctx, messageData)
		if err != nil {
			t.Logf("Handler error (may be expected): %v", err)
		}

		output := outputBuffer.String()
		if !strings.Contains(output, "Unknown tool") {
			t.Error("Expected error response for invalid tool name")
		}
	})

	t.Run("UninitializedServerError", func(t *testing.T) {
		server := createTestServer()
		server.Initialized = false // Ensure not initialized

		outputBuffer := &bytes.Buffer{}
		server.SetIO(nil, outputBuffer)

		ctx := context.Background()
		messageData := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`

		err := server.handleMessageWithValidation(ctx, messageData)
		if err != nil {
			t.Logf("Handler error (expected): %v", err)
		}

		output := outputBuffer.String()
		if !strings.Contains(output, "Server not initialized") {
			t.Error("Expected 'Server not initialized' error")
		}
	})

	t.Run("InvalidToolCallParams", func(t *testing.T) {
		server := createTestServer()
		server.Initialized = true

		outputBuffer := &bytes.Buffer{}
		server.SetIO(nil, outputBuffer)

		ctx := context.Background()
		messageData := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{}}`

		err := server.handleMessageWithValidation(ctx, messageData)
		if err != nil {
			t.Logf("Handler error (expected): %v", err)
		}

		output := outputBuffer.String()
		if !strings.Contains(output, "mcp.Tool name is required") {
			t.Error("Expected 'mcp.Tool name is required' error")
		}
	})
}

func TestUtilityFunctions(t *testing.T) {
	t.Run("isValidMethodName", func(t *testing.T) {
		tests := []struct {
			method string
			valid  bool
		}{
			{"initialize", true},
			{"tools/list", true},
			{"text_document/hover", true},
			{"workspace-symbol", true},
			{"test123", true},
			{"", false},
			{"invalid@method", false},
			{"method with spaces", false},
			{"method#with#hash", false},
		}

		for _, tt := range tests {
			t.Run(tt.method, func(t *testing.T) {
				result := mcp.IsValidMethodName(tt.method)
				if result != tt.valid {
					t.Errorf("Expected isValidMethodName(%q) = %v, got %v", tt.method, tt.valid, result)
				}
			})
		}
	})

	t.Run("isCompatibleProtocolVersion", func(t *testing.T) {
		tests := []struct {
			version    string
			compatible bool
		}{
			{"2024-11-05", true},
			{"2024-10-07", true},
			{"2024-09-01", true},
			{"2023-01-01", false},
			{"invalid", false},
		}

		for _, tt := range tests {
			t.Run(tt.version, func(t *testing.T) {
				result := mcp.IsCompatibleProtocolVersion(tt.version)
				if result != tt.compatible {
					t.Errorf("Expected isCompatibleProtocolVersion(%q) = %v, got %v", tt.version, tt.compatible, result)
				}
			})
		}
	})
}

func TestGetRecoveryMetrics(t *testing.T) {
	server := createTestServer()

	server.UpdateRecoveryContext("malformed")
	server.UpdateRecoveryContext("parse_error")
	server.UpdateRecoveryContext("parse_error")

	metrics := server.GetRecoveryMetrics()

	expectedFields := []string{
		"malformed_count", "parse_errors", "recovery_mode",
		"last_malformed", "last_parse_error", "recovery_start",
	}

	for _, field := range expectedFields {
		if _, exists := metrics[field]; !exists {
			t.Errorf("Expected field %s in metrics", field)
		}
	}

	if metrics["malformed_count"] != 1 {
		t.Errorf("Expected malformed_count 1, got: %v", metrics["malformed_count"])
	}

	if metrics["parse_errors"] != 2 {
		t.Errorf("Expected parse_errors 2, got: %v", metrics["parse_errors"])
	}
}

func BenchmarkMessageValidation(b *testing.B) {
	server := createTestServer()
	message := mcp.MCPMessage{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      1,
		Method:  "initialize",
		Params:  map[string]interface{}{"test": true},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = server.ValidateMessageStructure(&message)
	}
}

func BenchmarkSendMessage(b *testing.B) {
	server := createTestServer()
	server.SetIO(nil, io.Discard)

	message := mcp.MCPMessage{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      1,
		Result:  map[string]interface{}{"test": "value"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = server.SendMessage(message)
	}
}

func BenchmarkRecoveryContextUpdate(b *testing.B) {
	server := createTestServer()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server.UpdateRecoveryContext("malformed")
	}
}
