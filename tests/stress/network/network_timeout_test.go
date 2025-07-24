package network_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/gateway"
	"lsp-gateway/internal/transport"
	gatewayTestUtils "lsp-gateway/tests/utils/gateway"
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
			w, duration := executeTimeoutTest(t, tt.requestTimeout, tt.clientDelay)

			if tt.expectedError {
				validateTimeoutErrorResponse(t, w, tt.errorContains, duration, tt.requestTimeout)
			} else {
				validateSuccessResponse(t, w)
			}
		})
	}
}

func executeTimeoutTest(t *testing.T, requestTimeout, clientDelay time.Duration) (*httptest.ResponseRecorder, time.Duration) {
	mockClient := NewMockLSPClientWithDelay(clientDelay)

	// Start the mock client
	ctx := context.Background()
	if err := mockClient.Start(ctx); err != nil {
		t.Fatalf("Failed to start mock client: %v", err)
	}

	gw := createTimeoutTestGateway(t, map[string]transport.LSPClient{
		"test-server": mockClient,
	})

	req := createTimeoutTestRequest(t, requestTimeout)
	w := httptest.NewRecorder()

	start := time.Now()
	gw.HandleJSONRPC(w, req)
	duration := time.Since(start)

	return w, duration
}

func createTimeoutTestRequest(t *testing.T, timeout time.Duration) *http.Request {
	requestBody := gateway.JSONRPCRequest{
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

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewReader(bodyBytes))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	return req
}

func validateTimeoutErrorResponse(t *testing.T, w *httptest.ResponseRecorder, errorContains string, duration, requestTimeout time.Duration) {
	var response gateway.JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Error == nil {
		t.Error("Expected error response due to timeout")
		return
	}

	if !strings.Contains(strings.ToLower(response.Error.Message), strings.ToLower(errorContains)) {
		t.Errorf("Expected error containing '%s', got: %s", errorContains, response.Error.Message)
	}

	if duration < requestTimeout-500*time.Millisecond || duration > requestTimeout+2*time.Second {
		t.Errorf("Timeout occurred at %v, expected around %v", duration, requestTimeout)
	}
}

func validateSuccessResponse(t *testing.T, w *httptest.ResponseRecorder) {
	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response gateway.JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Error != nil {
		t.Errorf("Expected success, got error: %s", response.Error.Message)
	}
}

// TestGatewayTimeoutWithMultipleServers tests timeout behavior with multiple LSP servers
func TestGatewayTimeoutWithMultipleServers(t *testing.T) {
	t.Parallel()

	// Create multiple mock clients with different delays
	fastClient := NewMockLSPClientWithDelay(100 * time.Millisecond)
	slowClient := NewMockLSPClientWithDelay(3 * time.Second)
	verySlowClient := NewMockLSPClientWithDelay(10 * time.Second)

	gw := createTimeoutTestGateway(t, map[string]transport.LSPClient{
		"fast-server":      fastClient,
		"slow-server":      slowClient,
		"very-slow-server": verySlowClient,
	})

	// Router registration is now handled during gateway initialization
	// The servers are automatically registered based on the config

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
			requestBody := gateway.JSONRPCRequest{
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
			gw.HandleJSONRPC(w, req)
			duration := time.Since(start)

			var response gateway.JSONRPCResponse
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
	gw := createTimeoutTestGateway(t, map[string]transport.LSPClient{
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
		500 * time.Millisecond, // Should timeout
		1 * time.Second,        // Should timeout
		3 * time.Second,        // Should succeed
		5 * time.Second,        // Should succeed
		500 * time.Millisecond, // Should timeout
		1 * time.Second,        // Should timeout
		3 * time.Second,        // Should succeed
		5 * time.Second,        // Should succeed
		750 * time.Millisecond, // Should timeout
		4 * time.Second,        // Should succeed
	}

	for i, timeout := range timeouts {
		wg.Add(1)
		go func(reqID int, to time.Duration) {
			defer wg.Done()

			requestBody := gateway.JSONRPCRequest{
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
			gw.HandleJSONRPC(w, req)
			duration := time.Since(start)

			var response gateway.JSONRPCResponse
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
	gw := createTimeoutTestGateway(t, map[string]transport.LSPClient{
		"test-server": mockClient,
	})

	// First request: timeout
	t.Run("timeout request", func(t *testing.T) {
		requestBody := gateway.JSONRPCRequest{
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
		gw.HandleJSONRPC(w, req)

		var response gateway.JSONRPCResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Error == nil {
			t.Error("Expected timeout error")
		}
	})

	// Second request: success with longer timeout
	t.Run("recovery request", func(t *testing.T) {
		requestBody := gateway.JSONRPCRequest{
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
		gw.HandleJSONRPC(w, req)

		var response gateway.JSONRPCResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Error != nil {
			t.Errorf("Expected success after recovery, got error: %s", response.Error.Message)
		}
	})

	// Third request: timeout again
	t.Run("timeout again", func(t *testing.T) {
		requestBody := gateway.JSONRPCRequest{
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
		gw.HandleJSONRPC(w, req)

		var response gateway.JSONRPCResponse
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
	gw := createTimeoutTestGateway(t, map[string]transport.LSPClient{
		"test-server": mockClient,
	})

	// Notification (no ID field)
	requestBody := gateway.JSONRPCRequest{
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
	gw.HandleJSONRPC(w, req)
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
	gw := createTimeoutTestGateway(t, map[string]transport.LSPClient{
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
				requestBody := gateway.JSONRPCRequest{
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
				gw.HandleJSONRPC(w, req)

				var response gateway.JSONRPCResponse
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if response.Error == nil {
					t.Error("Expected timeout error")
				}
			})

			// Test with long timeout (should succeed)
			t.Run("long timeout", func(t *testing.T) {
				requestBody := gateway.JSONRPCRequest{
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
				gw.HandleJSONRPC(w, req)

				var response gateway.JSONRPCResponse
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

	mockClient := gatewayTestUtils.NewMockLSPClient()
	// Create an inactive client by calling Stop first
	if err := mockClient.Stop(); err != nil {
		t.Fatalf("Failed to stop mock client: %v", err)
	}

	gw := createTimeoutTestGateway(t, map[string]transport.LSPClient{
		"inactive-server": mockClient,
	})

	requestBody := gateway.JSONRPCRequest{
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
	gw.HandleJSONRPC(w, req)

	var response gateway.JSONRPCResponse
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
	active := m.active
	delay := m.delay
	m.mu.RUnlock()

	if !active {
		return nil, errors.New("client not active")
	}

	// Simulate delay with proper context cancellation handling
	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()

		select {
		case <-timer.C:
			// Delay completed successfully
		case <-ctx.Done():
			// Context cancelled during delay
			return nil, ctx.Err()
		}
	}

	// Check if client is still active after delay
	m.mu.RLock()
	active = m.active
	m.mu.RUnlock()

	if !active {
		return nil, errors.New("client became inactive during request")
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
	active := m.active
	delay := m.delay
	m.mu.RUnlock()

	if !active {
		return errors.New("client not active")
	}

	// Simulate delay with proper context cancellation handling
	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()

		select {
		case <-timer.C:
			// Delay completed successfully
		case <-ctx.Done():
			// Context cancelled during delay
			return ctx.Err()
		}
	}

	// Check if client is still active after delay
	m.mu.RLock()
	active = m.active
	m.mu.RUnlock()

	if !active {
		return errors.New("client became inactive during notification")
	}

	return nil
}

func (m *MockLSPClientWithDelay) IsActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

func createTimeoutTestGateway(t interface{}, clients map[string]transport.LSPClient) *gateway.Gateway {
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

	// Create mock client factory that returns pre-created clients
	clientMap := make(map[string]transport.LSPClient)
	for name, client := range clients {
		clientMap[name] = client
	}

	mockClientFactory := func(clientConfig transport.ClientConfig) (transport.LSPClient, error) {
		// Match by server name from config
		for serverName, client := range clientMap {
			// Find matching server config
			for _, serverCfg := range cfg.Servers {
				if serverCfg.Name == serverName && serverCfg.Command == clientConfig.Command {
					return client, nil
				}
			}
		}
		// Fallback: return any available client
		for _, client := range clientMap {
			return client, nil
		}
		return nil, fmt.Errorf("no mock client available")
	}

	// Use testable gateway pattern
	testableGateway, err := gatewayTestUtils.NewTestableGateway(cfg, mockClientFactory)
	if err != nil {
		// For stress tests, we need to handle this gracefully
		if tester, ok := t.(*testing.T); ok {
			tester.Fatalf("Failed to create testable gateway: %v", err)
		} else if tester, ok := t.(*testing.B); ok {
			tester.Fatalf("Failed to create testable gateway: %v", err)
		}
		panic(fmt.Sprintf("Failed to create testable gateway: %v", err))
	}

	return testableGateway.Gateway
}

// Benchmark tests for gateway timeout performance
func BenchmarkGatewayTimeout(b *testing.B) {
	mockClient := NewMockLSPClientWithDelay(100 * time.Millisecond)
	gw := createTimeoutTestGateway(b, map[string]transport.LSPClient{
		"bench-server": mockClient,
	})

	requestBody := gateway.JSONRPCRequest{
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
		gw.HandleJSONRPC(w, req)

		cancel()
	}
}

func BenchmarkGatewaySuccess(b *testing.B) {
	mockClient := NewMockLSPClientWithDelay(10 * time.Millisecond)
	gw := createTimeoutTestGateway(b, map[string]transport.LSPClient{
		"bench-server": mockClient,
	})

	requestBody := gateway.JSONRPCRequest{
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
		gw.HandleJSONRPC(w, req)

		cancel()
	}
}

// TestGatewayTimeoutErrorMessages tests that proper error messages are returned for timeouts
func TestGatewayTimeoutErrorMessages(t *testing.T) {
	t.Parallel()

	mockClient := NewMockLSPClientWithDelay(3 * time.Second)
	gw := createTimeoutTestGateway(t, map[string]transport.LSPClient{
		"test-server": mockClient,
	})

	requestBody := gateway.JSONRPCRequest{
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
	gw.HandleJSONRPC(w, req)

	// Verify response structure
	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response gateway.JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify error structure
	if response.Error == nil {
		t.Fatal("Expected error response")
	}

	if response.Error.Code != gateway.InternalError {
		t.Errorf("Expected error code %d, got %d", gateway.InternalError, response.Error.Code)
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
	gw := createTimeoutTestGateway(t, map[string]transport.LSPClient{
		"test-server": mockClient,
	})

	requestBody := gateway.JSONRPCRequest{
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
	gw.HandleJSONRPC(w, req)
	duration := time.Since(start)

	// Should cancel around 2 seconds
	if duration < 1500*time.Millisecond || duration > 3*time.Second {
		t.Errorf("Cancellation occurred at %v, expected around 2s", duration)
	}

	var response gateway.JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Error == nil {
		t.Error("Expected error due to context cancellation")
	}
}
