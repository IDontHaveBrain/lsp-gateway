package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/transport"
)

// TestGatewayHTTPTimeouts tests HTTP request timeout scenarios
func TestGatewayHTTPTimeouts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		requestTimeout time.Duration
		clientDelay    time.Duration
		expectedError  bool
		errorContains  string
	}{
		{
			name:           "fast response within timeout",
			requestTimeout: 5 * time.Second,
			clientDelay:    100 * time.Millisecond,
			expectedError:  false,
		},
		{
			name:           "slow response exceeds 1s timeout",
			requestTimeout: 1 * time.Second,
			clientDelay:    2 * time.Second,
			expectedError:  true,
			errorContains:  "context deadline exceeded",
		},
		{
			name:           "slow response exceeds 3s timeout",
			requestTimeout: 3 * time.Second,
			clientDelay:    5 * time.Second,
			expectedError:  true,
			errorContains:  "context deadline exceeded",
		},
		{
			name:           "very slow response exceeds 10s timeout",
			requestTimeout: 10 * time.Second,
			clientDelay:    15 * time.Second,
			expectedError:  true,
			errorContains:  "context deadline exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock client with configurable delay
			mockClient := NewMockLSPClientWithDelay(tt.clientDelay)
			
			gateway := createTimeoutTestGateway(t, map[string]transport.LSPClient{
				"test-server": mockClient,
			})

			// Create JSON-RPC request
			requestBody := JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      "test-1",
				Method:  "textDocument/definition",
				Params: map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": "file:///test.go",
					},
					"position": map[string]interface{}{
						"line":      10,
						"character": 5,
					},
				},
			}

			bodyBytes, err := json.Marshal(requestBody)
			if err != nil {
				t.Fatalf("Failed to marshal request: %v", err)
			}

			// Create HTTP request with timeout context
			ctx, cancel := context.WithTimeout(context.Background(), tt.requestTimeout)
			defer cancel()

			req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewReader(bodyBytes))
			req = req.WithContext(ctx)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			// Record start time
			start := time.Now()
			gateway.HandleJSONRPC(w, req)
			duration := time.Since(start)

			if tt.expectedError {
				// Should return an error response
				var response JSONRPCResponse
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if response.Error == nil {
					t.Error("Expected error response due to timeout")
				} else if !strings.Contains(strings.ToLower(response.Error.Message), strings.ToLower(tt.errorContains)) {
					t.Errorf("Expected error containing '%s', got: %s", tt.errorContains, response.Error.Message)
				}

				// Verify timeout occurred around expected time
				if duration < tt.requestTimeout-500*time.Millisecond || duration > tt.requestTimeout+2*time.Second {
					t.Errorf("Timeout occurred at %v, expected around %v", duration, tt.requestTimeout)
				}
			} else {
				// Should succeed
				if w.Code != 200 {
					t.Errorf("Expected status 200, got %d", w.Code)
				}

				var response JSONRPCResponse
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if response.Error != nil {
					t.Errorf("Expected success, got error: %s", response.Error.Message)
				}
			}
		})
	}
}

// TestGatewayTimeoutWithMultipleServers tests timeout behavior with multiple LSP servers
func TestGatewayTimeoutWithMultipleServers(t *testing.T) {
	t.Parallel()

	// Create multiple mock clients with different delays
	fastClient := NewMockLSPClientWithDelay(100 * time.Millisecond)
	slowClient := NewMockLSPClientWithDelay(3 * time.Second)
	verySlowClient := NewMockLSPClientWithDelay(10 * time.Second)

	gateway := createTimeoutTestGateway(t, map[string]transport.LSPClient{
		"fast-server":      fastClient,
		"slow-server":      slowClient,
		"very-slow-server": verySlowClient,
	})

	// Configure router for different file types
	gateway.router.RegisterServer("fast-server", []string{"go"})
	gateway.router.RegisterServer("slow-server", []string{"python"})
	gateway.router.RegisterServer("very-slow-server", []string{"javascript"})

	tests := []struct {
		name        string
		fileURI     string
		timeout     time.Duration
		expectError bool
	}{
		{
			name:        "fast server within timeout",
			fileURI:     "file:///test.go",
			timeout:     2 * time.Second,
			expectError: false,
		},
		{
			name:        "slow server within timeout",
			fileURI:     "file:///test.py",
			timeout:     5 * time.Second,
			expectError: false,
		},
		{
			name:        "slow server exceeds timeout",
			fileURI:     "file:///test.py",
			timeout:     1 * time.Second,
			expectError: true,
		},
		{
			name:        "very slow server exceeds timeout",
			fileURI:     "file:///test.js",
			timeout:     5 * time.Second,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestBody := JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      "test-multi",
				Method:  "textDocument/definition",
				Params: map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": tt.fileURI,
					},
					"position": map[string]interface{}{
						"line":      10,
						"character": 5,
					},
				},
			}

			bodyBytes, err := json.Marshal(requestBody)
			if err != nil {
				t.Fatalf("Failed to marshal request: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewReader(bodyBytes))
			req = req.WithContext(ctx)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			start := time.Now()
			gateway.HandleJSONRPC(w, req)
			duration := time.Since(start)

			var response JSONRPCResponse
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if tt.expectError {
				if response.Error == nil {
					t.Error("Expected timeout error")
				}
				// Verify timeout timing
				if duration > tt.timeout+2*time.Second {
					t.Errorf("Request took too long: %v, expected around %v", duration, tt.timeout)
				}
			} else {
				if response.Error != nil {
					t.Errorf("Expected success, got error: %s", response.Error.Message)
				}
			}
		})
	}
}

// TestGatewayConcurrentTimeouts tests concurrent requests with timeouts
func TestGatewayConcurrentTimeouts(t *testing.T) {
	t.Parallel()

	// Create a mock client with moderate delay
	mockClient := NewMockLSPClientWithDelay(2 * time.Second)
	gateway := createTimeoutTestGateway(t, map[string]transport.LSPClient{
		"test-server": mockClient,
	})

	const numConcurrentRequests = 10
	var wg sync.WaitGroup
	results := make(chan struct {
		timeout  time.Duration
		duration time.Duration
		hasError bool
	}, numConcurrentRequests)

	// Send concurrent requests with different timeouts
	timeouts := []time.Duration{
		500 * time.Millisecond,  // Should timeout
		1 * time.Second,         // Should timeout  
		3 * time.Second,         // Should succeed
		5 * time.Second,         // Should succeed
		500 * time.Millisecond,  // Should timeout
		1 * time.Second,         // Should timeout
		3 * time.Second,         // Should succeed
		5 * time.Second,         // Should succeed
		750 * time.Millisecond,  // Should timeout
		4 * time.Second,         // Should succeed
	}

	for i, timeout := range timeouts {
		wg.Add(1)
		go func(reqID int, to time.Duration) {
			defer wg.Done()

			requestBody := JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      fmt.Sprintf("concurrent-%d", reqID),
				Method:  "textDocument/definition",
				Params: map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": "file:///test.go",
					},
					"position": map[string]interface{}{
						"line":      reqID,
						"character": 5,
					},
				},
			}

			bodyBytes, err := json.Marshal(requestBody)
			if err != nil {
				t.Errorf("Failed to marshal request %d: %v", reqID, err)
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), to)
			defer cancel()

			req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewReader(bodyBytes))
			req = req.WithContext(ctx)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			start := time.Now()
			gateway.HandleJSONRPC(w, req)
			duration := time.Since(start)

			var response JSONRPCResponse
			hasError := false
			if err := json.NewDecoder(w.Body).Decode(&response); err == nil {
				hasError = (response.Error != nil)
			}

			results <- struct {
				timeout  time.Duration
				duration time.Duration
				hasError bool
			}{to, duration, hasError}
		}(i, timeout)
	}

	wg.Wait()
	close(results)

	timeoutCount := 0
	successCount := 0

	for result := range results {
		if result.timeout < 2*time.Second {
			// Should timeout
			if !result.hasError {
				t.Errorf("Expected timeout for %v duration", result.timeout)
			} else {
				timeoutCount++
			}
		} else {
			// Should succeed
			if result.hasError {
				t.Errorf("Expected success for %v timeout", result.timeout)
			} else {
				successCount++
			}
		}
	}

	t.Logf("Concurrent timeout test: %d timeouts, %d successes", timeoutCount, successCount)

	// Verify we got expected counts
	expectedTimeouts := 5
	expectedSuccesses := 5
	if timeoutCount != expectedTimeouts {
		t.Errorf("Expected %d timeouts, got %d", expectedTimeouts, timeoutCount)
	}
	if successCount != expectedSuccesses {
		t.Errorf("Expected %d successes, got %d", expectedSuccesses, successCount)
	}
}

// TestGatewayTimeoutRecovery tests that gateway recovers properly after timeouts
func TestGatewayTimeoutRecovery(t *testing.T) {
	t.Parallel()

	mockClient := NewMockLSPClientWithDelay(2 * time.Second)
	gateway := createTimeoutTestGateway(t, map[string]transport.LSPClient{
		"test-server": mockClient,
	})

	// First request: timeout
	t.Run("timeout request", func(t *testing.T) {
		requestBody := JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "timeout-test",
			Method:  "textDocument/definition",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file:///test.go",
				},
				"position": map[string]interface{}{
					"line":      10,
					"character": 5,
				},
			},
		}

		bodyBytes, err := json.Marshal(requestBody)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewReader(bodyBytes))
		req = req.WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		gateway.HandleJSONRPC(w, req)

		var response JSONRPCResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Error == nil {
			t.Error("Expected timeout error")
		}
	})

	// Second request: success with longer timeout
	t.Run("recovery request", func(t *testing.T) {
		requestBody := JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "recovery-test",
			Method:  "textDocument/definition",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file:///test.go",
				},
				"position": map[string]interface{}{
					"line":      20,
					"character": 10,
				},
			},
		}

		bodyBytes, err := json.Marshal(requestBody)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewReader(bodyBytes))
		req = req.WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		gateway.HandleJSONRPC(w, req)

		var response JSONRPCResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Error != nil {
			t.Errorf("Expected success after recovery, got error: %s", response.Error.Message)
		}
	})

	// Third request: timeout again
	t.Run("timeout again", func(t *testing.T) {
		requestBody := JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "timeout-again-test",
			Method:  "textDocument/references",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file:///test.go",
				},
				"position": map[string]interface{}{
					"line":      30,
					"character": 15,
				},
				"context": map[string]interface{}{
					"includeDeclaration": true,
				},
			},
		}

		bodyBytes, err := json.Marshal(requestBody)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewReader(bodyBytes))
		req = req.WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		gateway.HandleJSONRPC(w, req)

		var response JSONRPCResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Error == nil {
			t.Error("Expected timeout error again")
		}
	})
}

// TestGatewayNotificationTimeouts tests timeout handling for notifications
func TestGatewayNotificationTimeouts(t *testing.T) {
	t.Parallel()

	mockClient := NewMockLSPClientWithDelay(3 * time.Second)
	gateway := createTimeoutTestGateway(t, map[string]transport.LSPClient{
		"test-server": mockClient,
	})

	// Notification (no ID field)
	requestBody := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "textDocument/didOpen",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri":        "file:///test.go",
				"languageId": "go",
				"version":    1,
				"text":       "package main\n\nfunc main() {\n    println(\"Hello\")\n}",
			},
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewReader(bodyBytes))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	start := time.Now()
	gateway.HandleJSONRPC(w, req)
	duration := time.Since(start)

	// Notification should timeout but gateway should still respond appropriately
	if w.Code != 200 {
		t.Errorf("Expected status 200 for notification, got %d", w.Code)
	}

	// Check if timeout occurred around expected time
	if duration > 2*time.Second {
		t.Errorf("Notification handling took too long: %v", duration)
	}
}

// TestGatewayLSPMethodTimeouts tests different LSP methods with timeouts
func TestGatewayLSPMethodTimeouts(t *testing.T) {
	t.Parallel()

	mockClient := NewMockLSPClientWithDelay(2 * time.Second)
	gateway := createTimeoutTestGateway(t, map[string]transport.LSPClient{
		"test-server": mockClient,
	})

	methods := []struct {
		method string
		params interface{}
	}{
		{
			method: "textDocument/definition",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": "file:///test.go"},
				"position":     map[string]interface{}{"line": 10, "character": 5},
			},
		},
		{
			method: "textDocument/references",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": "file:///test.go"},
				"position":     map[string]interface{}{"line": 10, "character": 5},
				"context":      map[string]interface{}{"includeDeclaration": true},
			},
		},
		{
			method: "textDocument/hover",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": "file:///test.go"},
				"position":     map[string]interface{}{"line": 10, "character": 5},
			},
		},
		{
			method: "textDocument/documentSymbol",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": "file:///test.go"},
			},
		},
		{
			method: "workspace/symbol",
			params: map[string]interface{}{
				"query": "test",
			},
		},
	}

	for _, method := range methods {
		t.Run(method.method, func(t *testing.T) {
			// Test with short timeout (should fail)
			t.Run("short timeout", func(t *testing.T) {
				requestBody := JSONRPCRequest{
					JSONRPC: "2.0",
					ID:      fmt.Sprintf("timeout-%s", method.method),
					Method:  method.method,
					Params:  method.params,
				}

				bodyBytes, err := json.Marshal(requestBody)
				if err != nil {
					t.Fatalf("Failed to marshal request: %v", err)
				}

				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer cancel()

				req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewReader(bodyBytes))
				req = req.WithContext(ctx)
				req.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()
				gateway.HandleJSONRPC(w, req)

				var response JSONRPCResponse
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if response.Error == nil {
					t.Error("Expected timeout error")
				}
			})

			// Test with long timeout (should succeed)
			t.Run("long timeout", func(t *testing.T) {
				requestBody := JSONRPCRequest{
					JSONRPC: "2.0",
					ID:      fmt.Sprintf("success-%s", method.method),
					Method:  method.method,
					Params:  method.params,
				}

				bodyBytes, err := json.Marshal(requestBody)
				if err != nil {
					t.Fatalf("Failed to marshal request: %v", err)
				}

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewReader(bodyBytes))
				req = req.WithContext(ctx)
				req.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()
				gateway.HandleJSONRPC(w, req)

				var response JSONRPCResponse
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if response.Error != nil {
					t.Errorf("Expected success, got error: %s", response.Error.Message)
				}
			})
		})
	}
}

// TestGatewayTimeoutWithInactiveServer tests timeout when server becomes inactive
func TestGatewayTimeoutWithInactiveServer(t *testing.T) {
	t.Parallel()

	mockClient := NewMockLSPClient()
	// Create an inactive client by calling Stop first
	if err := mockClient.Stop(); err != nil {
		t.Fatalf("Failed to stop mock client: %v", err)
	}

	gateway := createTimeoutTestGateway(t, map[string]transport.LSPClient{
		"inactive-server": mockClient,
	})

	requestBody := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "inactive-test",
		Method:  "textDocument/definition",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///test.go",
			},
			"position": map[string]interface{}{
				"line":      10,
				"character": 5,
			},
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewReader(bodyBytes))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	gateway.HandleJSONRPC(w, req)

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Error == nil {
		t.Error("Expected error for inactive server")
	} else if !strings.Contains(strings.ToLower(response.Error.Message), "not active") {
		t.Errorf("Expected 'not active' error, got: %s", response.Error.Message)
	}
}

// Helper functions and mock implementations

// MockLSPClientWithDelay simulates an LSP client with configurable response delay
type MockLSPClientWithDelay struct {
	delay  time.Duration
	active bool
	mu     sync.RWMutex
}

func NewMockLSPClientWithDelay(delay time.Duration) *MockLSPClientWithDelay {
	return &MockLSPClientWithDelay{
		delay:  delay,
		active: true,
	}
}

func (m *MockLSPClientWithDelay) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active = true
	return nil
}

func (m *MockLSPClientWithDelay) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active = false
	return nil
}

func (m *MockLSPClientWithDelay) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	m.mu.RLock()
	delay := m.delay
	m.mu.RUnlock()

	// Simulate delay - but respect context cancellation
	select {
	case <-time.After(delay):
		// Delay completed
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Return mock response
	response := map[string]interface{}{
		"method": method,
		"delay":  delay.String(),
		"result": "mock response",
	}

	return json.Marshal(response)
}

func (m *MockLSPClientWithDelay) SendNotification(ctx context.Context, method string, params interface{}) error {
	m.mu.RLock()
	delay := m.delay
	m.mu.RUnlock()

	// Simulate delay - but respect context cancellation
	select {
	case <-time.After(delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *MockLSPClientWithDelay) IsActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

func createTimeoutTestGateway(t interface{}, clients map[string]transport.LSPClient) *Gateway {
	// Create test config
	var servers []config.ServerConfig
	for name := range clients {
		servers = append(servers, config.ServerConfig{
			Name:      name,
			Languages: []string{"go", "python", "javascript"},
			Command:   "mock",
			Transport: "stdio",
		})
	}

	cfg := &config.GatewayConfig{
		Port:    8080,
		Servers: servers,
	}

	// Create gateway with mock clients
	gateway := &Gateway{
		config:  cfg,
		clients: make(map[string]transport.LSPClient),
		router:  NewRouter(),
	}

	// Add mock clients
	for name, client := range clients {
		gateway.clients[name] = client
		gateway.router.RegisterServer(name, []string{"go", "python", "javascript"})
	}

	return gateway
}

// Benchmark tests for gateway timeout performance
func BenchmarkGatewayTimeout(b *testing.B) {
	mockClient := NewMockLSPClientWithDelay(100 * time.Millisecond)
	gateway := createTimeoutTestGateway(b, map[string]transport.LSPClient{
		"bench-server": mockClient,
	})

	requestBody := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "bench",
		Method:  "textDocument/definition",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///test.go",
			},
			"position": map[string]interface{}{
				"line":      10,
				"character": 5,
			},
		},
	}

	bodyBytes, _ := json.Marshal(requestBody)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		
		req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewReader(bodyBytes))
		req = req.WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		gateway.HandleJSONRPC(w, req)
		
		cancel()
	}
}

func BenchmarkGatewaySuccess(b *testing.B) {
	mockClient := NewMockLSPClientWithDelay(10 * time.Millisecond)
	gateway := createTimeoutTestGateway(b, map[string]transport.LSPClient{
		"bench-server": mockClient,
	})

	requestBody := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "bench",
		Method:  "textDocument/definition",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///test.go",
			},
			"position": map[string]interface{}{
				"line":      10,
				"character": 5,
			},
		},
	}

	bodyBytes, _ := json.Marshal(requestBody)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		
		req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewReader(bodyBytes))
		req = req.WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		gateway.HandleJSONRPC(w, req)
		
		cancel()
	}
}

// TestGatewayTimeoutErrorMessages tests that proper error messages are returned for timeouts
func TestGatewayTimeoutErrorMessages(t *testing.T) {
	t.Parallel()

	mockClient := NewMockLSPClientWithDelay(3 * time.Second)
	gateway := createTimeoutTestGateway(t, map[string]transport.LSPClient{
		"test-server": mockClient,
	})

	requestBody := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "error-test",
		Method:  "textDocument/definition",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///test.go",
			},
			"position": map[string]interface{}{
				"line":      10,
				"character": 5,
			},
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewReader(bodyBytes))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	gateway.HandleJSONRPC(w, req)

	// Verify response structure
	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify error structure
	if response.Error == nil {
		t.Fatal("Expected error response")
	}

	if response.Error.Code != InternalError {
		t.Errorf("Expected error code %d, got %d", InternalError, response.Error.Code)
	}

	if response.Error.Message == "" {
		t.Error("Expected non-empty error message")
	}

	if response.ID != requestBody.ID {
		t.Errorf("Expected response ID %v, got %v", requestBody.ID, response.ID)
	}

	if response.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC version 2.0, got %s", response.JSONRPC)
	}
}

// TestGatewayTimeoutWithContextCancellation tests proper handling when context is explicitly cancelled
func TestGatewayTimeoutWithContextCancellation(t *testing.T) {
	t.Parallel()

	mockClient := NewMockLSPClientWithDelay(5 * time.Second)
	gateway := createTimeoutTestGateway(t, map[string]transport.LSPClient{
		"test-server": mockClient,
	})

	requestBody := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "cancel-test",
		Method:  "textDocument/definition",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///test.go",
			},
			"position": map[string]interface{}{
				"line":      10,
				"character": 5,
			},
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewReader(bodyBytes))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	// Cancel context after 2 seconds
	go func() {
		time.Sleep(2 * time.Second)
		cancel()
	}()

	start := time.Now()
	gateway.HandleJSONRPC(w, req)
	duration := time.Since(start)

	// Should cancel around 2 seconds
	if duration < 1500*time.Millisecond || duration > 3*time.Second {
		t.Errorf("Cancellation occurred at %v, expected around 2s", duration)
	}

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Error == nil {
		t.Error("Expected error due to context cancellation")
	}
}