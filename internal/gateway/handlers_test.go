package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/transport"
)

// MockLSPClient implements transport.LSPClient for testing
type MockLSPClient struct {
	active          bool
	sendRequestFunc func(ctx context.Context, method string, params interface{}) (json.RawMessage, error)
	sendNotifyFunc  func(ctx context.Context, method string, params interface{}) error
	startFunc       func(ctx context.Context) error
	stopFunc        func() error
	mu              sync.RWMutex
}

func NewMockLSPClient() *MockLSPClient {
	return &MockLSPClient{
		active: true,
		sendRequestFunc: func(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
			// Default successful response
			result := map[string]interface{}{
				"uri": "file:///test.go",
				"range": map[string]interface{}{
					"start": map[string]interface{}{"line": 0, "character": 0},
					"end":   map[string]interface{}{"line": 0, "character": 10},
				},
			}
			return json.Marshal(result)
		},
		sendNotifyFunc: func(ctx context.Context, method string, params interface{}) error {
			return nil
		},
		startFunc: func(ctx context.Context) error {
			return nil
		},
		stopFunc: func() error {
			return nil
		},
	}
}

func (m *MockLSPClient) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startFunc(ctx)
}

func (m *MockLSPClient) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active = false
	return m.stopFunc()
}

func (m *MockLSPClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sendRequestFunc(ctx, method, params)
}

func (m *MockLSPClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sendNotifyFunc(ctx, method, params)
}

func (m *MockLSPClient) IsActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

func (m *MockLSPClient) SetSendRequestFunc(f func(ctx context.Context, method string, params interface{}) (json.RawMessage, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendRequestFunc = f
}

func (m *MockLSPClient) SetActive(active bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active = active
}

// Ensure MockLSPClient implements transport.LSPClient interface
var _ transport.LSPClient = (*MockLSPClient)(nil)

// Test helper functions
func createTestGateway(t *testing.T) *Gateway {
	config := &config.GatewayConfig{
		Servers: []config.ServerConfig{
			{
				Name:      "go-server",
				Languages: []string{"go"},
				Command:   "echo", // Use echo as a mock command that exists
				Transport: "stdio",
			},
			{
				Name:      "python-server", 
				Languages: []string{"python"},
				Command:   "cat", // Use cat as a mock command that exists
				Transport: "stdio",
			},
		},
		Timeout: "30s",
	}

	gateway, err := NewGateway(config)
	if err != nil {
		t.Fatalf("Failed to create test gateway: %v", err)
	}

	// Replace real clients with mock clients
	gateway.Clients["go-server"] = NewMockLSPClient()
	gateway.Clients["python-server"] = NewMockLSPClient()

	return gateway
}

func createJSONRPCRequest(method string, params interface{}, id interface{}) JSONRPCRequest {
	return JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		Method:  method,
		Params:  params,
		ID:      id,
	}
}

func createTextDocumentParams(uri string, line, character int) map[string]interface{} {
	return map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": character,
		},
	}
}

// TestHandleJSONRPC_ValidRequests tests the main HTTP entry point with valid requests
func TestHandleJSONRPC_ValidRequests(t *testing.T) {
	gateway := createTestGateway(t)

	tests := []struct {
		name       string
		method     string
		params     interface{}
		expectCode int
	}{
		{
			name:       "textDocument/definition",
			method:     LSPMethodDefinition,
			params:     createTextDocumentParams("file:///test.go", 10, 5),
			expectCode: http.StatusOK,
		},
		{
			name:       "textDocument/references", 
			method:     LSPMethodReferences,
			params:     createTextDocumentParams("file:///test.py", 20, 10),
			expectCode: http.StatusOK,
		},
		{
			name:       "textDocument/hover",
			method:     LSPMethodHover,
			params:     createTextDocumentParams("file:///test.go", 5, 15),
			expectCode: http.StatusOK,
		},
		{
			name:   "textDocument/documentSymbol",
			method: LSPMethodDocumentSymbol,
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file:///test.go",
				},
			},
			expectCode: http.StatusOK,
		},
		{
			name:   "workspace/symbol",
			method: LSPMethodWorkspaceSymbol,
			params: map[string]interface{}{
				"query": "TestFunction",
			},
			expectCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := createJSONRPCRequest(tt.method, tt.params, 1)
			reqBody, _ := json.Marshal(req)

			httpReq := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(reqBody))
			httpReq.Header.Set("Content-Type", "application/json")
			
			recorder := httptest.NewRecorder()
			gateway.HandleJSONRPC(recorder, httpReq)

			if recorder.Code != tt.expectCode {
				t.Errorf("Expected status code %d, got %d", tt.expectCode, recorder.Code)
			}

			// Verify response is valid JSON-RPC
			var response JSONRPCResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Errorf("Failed to unmarshal response: %v", err)
			}

			if response.JSONRPC != JSONRPCVersion {
				t.Errorf("Expected JSON-RPC version %s, got %s", JSONRPCVersion, response.JSONRPC)
			}

			// Convert both IDs to strings for comparison since JSON unmarshaling can change types
			expectedID := fmt.Sprintf("%v", req.ID)
			actualID := fmt.Sprintf("%v", response.ID)
			if actualID != expectedID {
				t.Errorf("Expected ID %v, got %v", req.ID, response.ID)
			}

			if response.Error != nil {
				t.Errorf("Expected no error, got: %+v", response.Error)
			}
		})
	}
}

// TestHandleJSONRPC_InvalidHTTPMethod tests HTTP method validation
func TestHandleJSONRPC_InvalidHTTPMethod(t *testing.T) {
	gateway := createTestGateway(t)

	methods := []string{"GET", "PUT", "DELETE", "PATCH"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			httpReq := httptest.NewRequest(method, "/jsonrpc", nil)
			recorder := httptest.NewRecorder()
			
			gateway.HandleJSONRPC(recorder, httpReq)
			
			if recorder.Code != http.StatusMethodNotAllowed {
				t.Errorf("Expected status %d for %s method, got %d", 
					http.StatusMethodNotAllowed, method, recorder.Code)
			}
		})
	}
}

// TestHandleJSONRPC_MalformedJSON tests JSON parsing error handling
func TestHandleJSONRPC_MalformedJSON(t *testing.T) {
	gateway := createTestGateway(t)

	tests := []struct {
		name string
		body string
	}{
		{"Invalid JSON", `{"invalid": json}`},
		{"Empty body", ``},
		{"Non-JSON", `this is not json`},
		{"Incomplete JSON", `{"jsonrpc": "2.0", "method"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpReq := httptest.NewRequest("POST", "/jsonrpc", strings.NewReader(tt.body))
			httpReq.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			gateway.HandleJSONRPC(recorder, httpReq)

			var response JSONRPCResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Errorf("Failed to unmarshal error response: %v", err)
			}

			if response.Error == nil {
				t.Error("Expected error in response")
			}

			if response.Error.Code != ParseError {
				t.Errorf("Expected parse error code %d, got %d", ParseError, response.Error.Code)
			}
		})
	}
}

// TestHandleJSONRPC_InvalidJSONRPC tests JSON-RPC validation
func TestHandleJSONRPC_InvalidJSONRPC(t *testing.T) {
	gateway := createTestGateway(t)

	tests := []struct {
		name     string
		request  JSONRPCRequest
		errorCode int
	}{
		{
			name: "Invalid JSON-RPC version",
			request: JSONRPCRequest{
				JSONRPC: "1.0",
				Method:  "test",
				ID:      1,
			},
			errorCode: InvalidRequest,
		},
		{
			name: "Missing method",
			request: JSONRPCRequest{
				JSONRPC: JSONRPCVersion,
				ID:      1,
			},
			errorCode: InvalidRequest,
		},
		{
			name: "Empty method",
			request: JSONRPCRequest{
				JSONRPC: JSONRPCVersion,
				Method:  "",
				ID:      1,
			},
			errorCode: InvalidRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody, _ := json.Marshal(tt.request)
			httpReq := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(reqBody))
			httpReq.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			gateway.HandleJSONRPC(recorder, httpReq)

			var response JSONRPCResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Errorf("Failed to unmarshal error response: %v", err)
			}

			if response.Error == nil {
				t.Error("Expected error in response")
			}

			if response.Error.Code != tt.errorCode {
				t.Errorf("Expected error code %d, got %d", tt.errorCode, response.Error.Code)
			}
		})
	}
}

// TestRouteRequest tests the request routing logic
func TestRouteRequest(t *testing.T) {
	gateway := createTestGateway(t)

	tests := []struct {
		name           string
		request        JSONRPCRequest
		expectedServer string
		expectError    bool
	}{
		{
			name: "Go file routing",
			request: createJSONRPCRequest(
				LSPMethodDefinition,
				createTextDocumentParams("file:///test.go", 10, 5),
				1,
			),
			expectedServer: "go-server",
			expectError:    false,
		},
		{
			name: "Python file routing",
			request: createJSONRPCRequest(
				LSPMethodDefinition,
				createTextDocumentParams("file:///test.py", 10, 5),
				1,
			),
			expectedServer: "python-server",
			expectError:    false,
		},
		{
			name: "Unsupported file extension",
			request: createJSONRPCRequest(
				LSPMethodDefinition,
				createTextDocumentParams("file:///test.xyz", 10, 5),
				1,
			),
			expectError: true,
		},
		{
			name: "No file extension",
			request: createJSONRPCRequest(
				LSPMethodDefinition,
				createTextDocumentParams("file:///README", 10, 5),
				1,
			),
			expectError: true,
		},
		{
			name: "Workspace symbol - any server",
			request: createJSONRPCRequest(
				LSPMethodWorkspaceSymbol,
				map[string]interface{}{"query": "test"},
				1,
			),
			expectError: false, // Should return any available server
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serverName, err := gateway.routeRequest(tt.request)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				if tt.expectedServer != "" && serverName != tt.expectedServer {
					t.Errorf("Expected server %s, got %s", tt.expectedServer, serverName)
				}
			}
		})
	}
}

// TestProcessLSPRequest tests the core LSP request processing logic
func TestProcessLSPRequest(t *testing.T) {
	gateway := createTestGateway(t)

	t.Run("Successful single server request", func(t *testing.T) {
		req := createJSONRPCRequest(
			LSPMethodDefinition,
			createTextDocumentParams("file:///test.go", 10, 5),
			1,
		)

		recorder := httptest.NewRecorder()
		httpReq := httptest.NewRequest("POST", "/jsonrpc", nil)
		
		startTime := time.Now()
		gateway.processLSPRequest(recorder, httpReq, req, "go-server", nil, startTime)

		var response JSONRPCResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if response.Error != nil {
			t.Errorf("Expected no error, got: %+v", response.Error)
		}

		if response.Result == nil {
			t.Error("Expected result in response")
		}
	})

	t.Run("Server not found", func(t *testing.T) {
		req := createJSONRPCRequest(
			LSPMethodDefinition,
			createTextDocumentParams("file:///test.go", 10, 5),
			1,
		)

		recorder := httptest.NewRecorder()
		httpReq := httptest.NewRequest("POST", "/jsonrpc", nil)
		
		startTime := time.Now()
		gateway.processLSPRequest(recorder, httpReq, req, "nonexistent-server", nil, startTime)

		var response JSONRPCResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if response.Error == nil {
			t.Error("Expected error for nonexistent server")
		}

		if response.Error.Code != InternalError {
			t.Errorf("Expected internal error code %d, got %d", InternalError, response.Error.Code)
		}
	})

	t.Run("Inactive server", func(t *testing.T) {
		// Set the mock client as inactive
		mockClient := gateway.Clients["go-server"].(*MockLSPClient)
		mockClient.SetActive(false)

		req := createJSONRPCRequest(
			LSPMethodDefinition,
			createTextDocumentParams("file:///test.go", 10, 5),
			1,
		)

		recorder := httptest.NewRecorder()
		httpReq := httptest.NewRequest("POST", "/jsonrpc", nil)
		
		startTime := time.Now()
		gateway.processLSPRequest(recorder, httpReq, req, "go-server", nil, startTime)

		var response JSONRPCResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if response.Error == nil {
			t.Error("Expected error for inactive server")
		}

		// Reset for other tests
		mockClient.SetActive(true)
	})
}

// TestHandleNotification tests LSP notification handling
func TestHandleNotification(t *testing.T) {
	gateway := createTestGateway(t)
	mockClient := gateway.Clients["go-server"].(*MockLSPClient)

	notificationCalled := false
	mockClient.sendNotifyFunc = func(ctx context.Context, method string, params interface{}) error {
		notificationCalled = true
		return nil
	}

	req := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		Method:  "textDocument/didOpen",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///test.go",
			},
		},
		// No ID for notifications
	}

	recorder := httptest.NewRecorder()
	httpReq := httptest.NewRequest("POST", "/jsonrpc", nil)
	startTime := time.Now()

	gateway.handleNotification(recorder, httpReq, req, mockClient, nil, startTime)

	if !notificationCalled {
		t.Error("Expected notification to be sent to client")
	}

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, recorder.Code)
	}
}

// TestHandleRequest tests LSP request handling
func TestHandleRequest(t *testing.T) {
	gateway := createTestGateway(t)
	mockClient := gateway.Clients["go-server"].(*MockLSPClient)

	t.Run("Successful request", func(t *testing.T) {
		expectedResult := map[string]interface{}{
			"uri": "file:///test.go",
			"range": map[string]interface{}{
				"start": map[string]interface{}{"line": 10, "character": 5},
				"end":   map[string]interface{}{"line": 10, "character": 15},
			},
		}

		mockClient.SetSendRequestFunc(func(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
			return json.Marshal(expectedResult)
		})

		req := createJSONRPCRequest(
			LSPMethodDefinition,
			createTextDocumentParams("file:///test.go", 10, 5),
			1,
		)

		recorder := httptest.NewRecorder()
		httpReq := httptest.NewRequest("POST", "/jsonrpc", nil)
		startTime := time.Now()

		gateway.handleRequest(recorder, httpReq, req, mockClient, nil, startTime)

		var response JSONRPCResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if response.Error != nil {
			t.Errorf("Expected no error, got: %+v", response.Error)
		}

		if response.Result == nil {
			t.Error("Expected result in response")
		}
	})

	t.Run("Request timeout", func(t *testing.T) {
		mockClient.SetSendRequestFunc(func(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
			return nil, context.DeadlineExceeded
		})

		req := createJSONRPCRequest(
			LSPMethodDefinition,
			createTextDocumentParams("file:///test.go", 10, 5),
			1,
		)

		recorder := httptest.NewRecorder()
		httpReq := httptest.NewRequest("POST", "/jsonrpc", nil)
		startTime := time.Now()

		gateway.handleRequest(recorder, httpReq, req, mockClient, nil, startTime)

		var response JSONRPCResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if response.Error == nil {
			t.Error("Expected error for timeout")
		}

		if !strings.Contains(response.Error.Message, "timeout") {
			t.Error("Expected timeout error message")
		}
	})

	t.Run("Server error", func(t *testing.T) {
		mockClient.SetSendRequestFunc(func(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
			return nil, fmt.Errorf("server internal error")
		})

		req := createJSONRPCRequest(
			LSPMethodDefinition,
			createTextDocumentParams("file:///test.go", 10, 5),
			1,
		)

		recorder := httptest.NewRecorder()
		httpReq := httptest.NewRequest("POST", "/jsonrpc", nil)
		startTime := time.Now()

		gateway.handleRequest(recorder, httpReq, req, mockClient, nil, startTime)

		var response JSONRPCResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if response.Error == nil {
			t.Error("Expected error for server failure")
		}

		if response.Error.Code != InternalError {
			t.Errorf("Expected internal error code %d, got %d", InternalError, response.Error.Code)
		}
	})
}

// TestExtractURI tests URI extraction from request parameters
func TestExtractURI(t *testing.T) {
	gateway := createTestGateway(t)

	tests := []struct {
		name        string
		request     JSONRPCRequest
		expectedURI string
		expectError bool
	}{
		{
			name: "TextDocument parameters",
			request: createJSONRPCRequest(
				LSPMethodDefinition,
				createTextDocumentParams("file:///test.go", 10, 5),
				1,
			),
			expectedURI: "file:///test.go",
			expectError: false,
		},
		{
			name: "DocumentSymbol parameters",
			request: createJSONRPCRequest(
				LSPMethodDocumentSymbol,
				map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": "file:///test.py",
					},
				},
				1,
			),
			expectedURI: "file:///test.py",
			expectError: false,
		},
		{
			name: "Missing parameters",
			request: createJSONRPCRequest(
				LSPMethodDefinition,
				nil,
				1,
			),
			expectError: true,
		},
		{
			name: "Invalid parameters format",
			request: createJSONRPCRequest(
				LSPMethodDefinition,
				"invalid params",
				1,
			),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uri, err := gateway.extractURI(tt.request)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				if uri != tt.expectedURI {
					t.Errorf("Expected URI %s, got %s", tt.expectedURI, uri)
				}
			}
		})
	}
}

// TestWriteError tests error response formatting
func TestWriteError(t *testing.T) {
	gateway := createTestGateway(t)

	tests := []struct {
		name     string
		id       interface{}
		code     int
		message  string
		err      error
	}{
		{
			name:    "Parse error",
			id:      nil,
			code:    ParseError,
			message: "Parse error",
			err:     fmt.Errorf("invalid JSON"),
		},
		{
			name:    "Method not found",
			id:      1,
			code:    MethodNotFound,
			message: "Method not found",
			err:     fmt.Errorf("unsupported method"),
		},
		{
			name:    "Internal error",
			id:      "test-123",
			code:    InternalError,
			message: "Internal error",
			err:     fmt.Errorf("server failure"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			gateway.writeError(recorder, tt.id, tt.code, tt.message, tt.err)

			var response JSONRPCResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("Failed to unmarshal error response: %v", err)
			}

			if response.JSONRPC != JSONRPCVersion {
				t.Errorf("Expected JSON-RPC version %s, got %s", JSONRPCVersion, response.JSONRPC)
			}

			// Convert both IDs to strings for comparison since JSON unmarshaling can change types
			expectedID := fmt.Sprintf("%v", tt.id)
			actualID := fmt.Sprintf("%v", response.ID)
			if actualID != expectedID {
				t.Errorf("Expected ID %v, got %v", tt.id, response.ID)
			}

			if response.Error == nil {
				t.Error("Expected error in response")
			}

			if response.Error.Code != tt.code {
				t.Errorf("Expected error code %d, got %d", tt.code, response.Error.Code)
			}

			if response.Error.Message != tt.message {
				t.Errorf("Expected error message %s, got %s", tt.message, response.Error.Message)
			}

			if tt.err != nil && response.Error.Data != tt.err.Error() {
				t.Errorf("Expected error data %s, got %v", tt.err.Error(), response.Error.Data)
			}
		})
	}
}

// TestConcurrentRequests tests concurrent request handling
func TestConcurrentRequests(t *testing.T) {
	gateway := createTestGateway(t)

	numRequests := 10
	var wg sync.WaitGroup
	errors := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			req := createJSONRPCRequest(
				LSPMethodDefinition,
				createTextDocumentParams("file:///test.go", id, 5),
				id,
			)

			reqBody, _ := json.Marshal(req)
			httpReq := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(reqBody))
			httpReq.Header.Set("Content-Type", "application/json")
			
			recorder := httptest.NewRecorder()
			gateway.HandleJSONRPC(recorder, httpReq)

			if recorder.Code != http.StatusOK {
				errors <- fmt.Errorf("request %d failed with status %d", id, recorder.Code)
				return
			}

			var response JSONRPCResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				errors <- fmt.Errorf("request %d: failed to unmarshal response: %v", id, err)
				return
			}

			if response.Error != nil {
				errors <- fmt.Errorf("request %d: got error response: %+v", id, response.Error)
				return
			}

			// Convert both IDs to strings for comparison since JSON unmarshaling can change types
			expectedID := fmt.Sprintf("%v", id)
			actualID := fmt.Sprintf("%v", response.ID)
			if actualID != expectedID {
				errors <- fmt.Errorf("request %d: expected ID %d, got %v", id, id, response.ID)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

// TestContentTypeValidation tests Content-Type header handling
func TestContentTypeValidation(t *testing.T) {
	gateway := createTestGateway(t)

	req := createJSONRPCRequest(
		LSPMethodDefinition,
		createTextDocumentParams("file:///test.go", 10, 5),
		1,
	)
	reqBody, _ := json.Marshal(req)

	// Test without Content-Type header
	httpReq := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(reqBody))
	recorder := httptest.NewRecorder()
	
	gateway.HandleJSONRPC(recorder, httpReq)
	
	// Should still work (Content-Type is not strictly required for JSON-RPC)
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	// Verify response Content-Type is set correctly
	contentType := recorder.Header().Get("Content-Type")
	if contentType != HTTPContentTypeJSON {
		t.Errorf("Expected Content-Type %s, got %s", HTTPContentTypeJSON, contentType)
	}
}

// BenchmarkHandleJSONRPC benchmarks the main handler function
func BenchmarkHandleJSONRPC(b *testing.B) {
	gateway := createTestGateway(&testing.T{})

	req := createJSONRPCRequest(
		LSPMethodDefinition,
		createTextDocumentParams("file:///test.go", 10, 5),
		1,
	)
	reqBody, _ := json.Marshal(req)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		httpReq := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(reqBody))
		httpReq.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		
		gateway.HandleJSONRPC(recorder, httpReq)
	}
}

// BenchmarkRouteRequest benchmarks the request routing function
func BenchmarkRouteRequest(b *testing.B) {
	gateway := createTestGateway(&testing.T{})

	req := createJSONRPCRequest(
		LSPMethodDefinition,
		createTextDocumentParams("file:///test.go", 10, 5),
		1,
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := gateway.routeRequest(req)
		if err != nil {
			b.Fatalf("Routing failed: %v", err)
		}
	}
}