package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/transport"
)

// TestGatewayHTTPClientConnectionFailures tests HTTP client connection failures when gateway is unavailable
func TestGatewayHTTPClientConnectionFailures(t *testing.T) {
	t.Parallel()

	t.Run("gateway server not started", func(t *testing.T) {
		// Test HTTP client behavior when gateway server is not running
		url := "http://localhost:65523/jsonrpc" // Valid high port unlikely to be in use

		request := JSONRPCRequest{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "textDocument/definition",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": "file:///test.go"},
				"position":     map[string]interface{}{"line": 0, "character": 5},
			},
		}

		body, err := json.Marshal(request)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Post(url, "application/json", bytes.NewBuffer(body))

		if err == nil {
			t.Error("Expected connection error when gateway is not running")
			if resp != nil {
				resp.Body.Close()
			}
			return
		}

		if !strings.Contains(strings.ToLower(err.Error()), "connection refused") &&
		   !strings.Contains(strings.ToLower(err.Error()), "connect") {
			t.Errorf("Expected connection refused error, got: %v", err)
		}
	})

	t.Run("gateway server refuses connection", func(t *testing.T) {
		// Create a server that immediately closes connections
		listener, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			t.Fatalf("Failed to create listener: %v", err)
		}
		defer listener.Close()

		address := listener.Addr().String()
		url := fmt.Sprintf("http://%s/jsonrpc", address)

		// Server that immediately closes connections
		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				// Immediately close to simulate connection refused
				conn.Close()
			}
		}()

		request := JSONRPCRequest{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "textDocument/definition",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": "file:///test.go"},
				"position":     map[string]interface{}{"line": 0, "character": 5},
			},
		}

		body, err := json.Marshal(request)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Post(url, "application/json", bytes.NewBuffer(body))

		if err == nil {
			t.Error("Expected connection error when server refuses connection")
			if resp != nil {
				resp.Body.Close()
			}
			return
		}

		// Should get connection reset or EOF error
		errStr := strings.ToLower(err.Error())
		if !strings.Contains(errStr, "connection") && !strings.Contains(errStr, "eof") {
			t.Errorf("Expected connection/EOF error, got: %v", err)
		}
	})
}

// TestGatewayBackendLSPServerUnavailable tests gateway behavior when backend LSP servers are unavailable
func TestGatewayBackendLSPServerUnavailable(t *testing.T) {
	t.Parallel()

	t.Run("single LSP server connection refused", func(t *testing.T) {
		// Create gateway with client that will fail to connect
		mockClient := NewMockLSPClient()
		mockClient.SetStartError(fmt.Errorf("connection refused: dial tcp [::1]:65522: connect: connection refused"))

		cfg := &config.GatewayConfig{
			Port: 8080,
			Servers: []config.ServerConfig{
				{Name: "failing-server", Languages: []string{"go"}, Command: "localhost:65522", Transport: "tcp"},
			},
		}

		gateway := &Gateway{
			config:  cfg,
			clients: make(map[string]transport.LSPClient),
			router:  NewRouter(),
		}
		gateway.clients["failing-server"] = mockClient

		request := JSONRPCRequest{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "textDocument/definition",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": "file:///test.go"},
				"position":     map[string]interface{}{"line": 0, "character": 5},
			},
		}

		body, err := json.Marshal(request)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		gateway.HandleJSONRPC(w, req)

		var response JSONRPCResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Error == nil {
			t.Error("Expected error response when LSP server connection is refused")
		} else if response.Error.Code != InternalError {
			t.Errorf("Expected internal error code %d, got %d", InternalError, response.Error.Code)
		}
	})

	t.Run("multiple LSP servers all unavailable", func(t *testing.T) {
		// Create gateway with multiple failing clients
		mockClient1 := NewMockLSPClient()
		mockClient2 := NewMockLSPClient()
		
		mockClient1.SetStartError(fmt.Errorf("connection refused"))
		mockClient2.SetStartError(fmt.Errorf("connection refused"))

		cfg := &config.GatewayConfig{
			Port: 8080,
			Servers: []config.ServerConfig{
				{Name: "go-server", Languages: []string{"go"}, Command: "localhost:65521", Transport: "tcp"},
				{Name: "python-server", Languages: []string{"python"}, Command: "localhost:65520", Transport: "tcp"},
			},
		}

		gateway := &Gateway{
			config:  cfg,
			clients: make(map[string]transport.LSPClient),
			router:  NewRouter(),
		}
		gateway.clients["go-server"] = mockClient1
		gateway.clients["python-server"] = mockClient2

		// Test Go request
		goRequest := JSONRPCRequest{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "textDocument/definition",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": "file:///test.go"},
				"position":     map[string]interface{}{"line": 0, "character": 5},
			},
		}

		body, err := json.Marshal(goRequest)
		if err != nil {
			t.Fatalf("Failed to marshal Go request: %v", err)
		}

		req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		gateway.HandleJSONRPC(w, req)

		var response JSONRPCResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode Go response: %v", err)
		}

		if response.Error == nil {
			t.Error("Expected error response when Go LSP server is unavailable")
		}

		// Test Python request
		pythonRequest := JSONRPCRequest{
			JSONRPC: JSONRPCVersion,
			ID:      2,
			Method:  "textDocument/definition",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": "file:///test.py"},
				"position":     map[string]interface{}{"line": 0, "character": 5},
			},
		}

		body, err = json.Marshal(pythonRequest)
		if err != nil {
			t.Fatalf("Failed to marshal Python request: %v", err)
		}

		req = httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()

		gateway.HandleJSONRPC(w, req)

		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode Python response: %v", err)
		}

		if response.Error == nil {
			t.Error("Expected error response when Python LSP server is unavailable")
		}
	})
}

// TestGatewayRetryLogicConnectionFailures tests retry behavior with connection failures
func TestGatewayRetryLogicConnectionFailures(t *testing.T) {
	t.Parallel()

	t.Run("no retry on connection refused", func(t *testing.T) {
		mockClient := NewMockLSPClient()
		
		// Simulate connection refused error that should not trigger retries
		mockClient.SetRequestError(fmt.Errorf("connection refused"))

		cfg := &config.GatewayConfig{
			Port: 8080,
			Servers: []config.ServerConfig{
				{Name: "test-server", Languages: []string{"go"}, Command: "localhost:65519", Transport: "tcp"},
			},
		}

		gateway := &Gateway{
			config:  cfg,
			clients: make(map[string]transport.LSPClient),
			router:  NewRouter(),
		}
		gateway.clients["test-server"] = mockClient

		request := JSONRPCRequest{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "textDocument/definition",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": "file:///test.go"},
				"position":     map[string]interface{}{"line": 0, "character": 5},
			},
		}

		body, err := json.Marshal(request)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		startTime := time.Now()
		gateway.HandleJSONRPC(w, req)
		elapsed := time.Since(startTime)

		// Should fail quickly without retries
		if elapsed > 2*time.Second {
			t.Errorf("Request took too long (%v), possible retry attempts", elapsed)
		}

		var response JSONRPCResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Error == nil {
			t.Error("Expected error response for connection refused")
		}

		// Should only have attempted request once (no retries)
		if mockClient.GetRequestCount() > 1 {
			t.Errorf("Expected 1 request attempt, got %d", mockClient.GetRequestCount())
		}
	})
}

// TestGatewayCircuitBreakerConnectionFailures tests circuit breaker behavior with connection failures
func TestGatewayCircuitBreakerConnectionFailures(t *testing.T) {
	t.Parallel()

	t.Run("circuit breaker on repeated connection failures", func(t *testing.T) {
		mockClient := NewMockLSPClient()

		cfg := &config.GatewayConfig{
			Port: 8080,
			Servers: []config.ServerConfig{
				{Name: "test-server", Languages: []string{"go"}, Command: "localhost:65518", Transport: "tcp"},
			},
		}

		gateway := &Gateway{
			config:  cfg,
			clients: make(map[string]transport.LSPClient),
			router:  NewRouter(),
		}
		gateway.clients["test-server"] = mockClient

		// Send multiple requests that will fail due to connection refused
		const numRequests = 10
		for i := 0; i < numRequests; i++ {
			mockClient.SetRequestError(fmt.Errorf("connection refused"))

			request := JSONRPCRequest{
				JSONRPC: JSONRPCVersion,
				ID:      i,
				Method:  "textDocument/definition",
				Params: map[string]interface{}{
					"textDocument": map[string]interface{}{"uri": "file:///test.go"},
					"position":     map[string]interface{}{"line": 0, "character": 5},
				},
			}

			body, err := json.Marshal(request)
			if err != nil {
				t.Fatalf("Failed to marshal request %d: %v", i, err)
			}

			req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			gateway.HandleJSONRPC(w, req)

			var response JSONRPCResponse
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response %d: %v", i, err)
			}

			if response.Error == nil {
				t.Errorf("Expected error response for request %d", i)
			}
		}

		t.Logf("Sent %d requests, total request count: %d", numRequests, mockClient.GetRequestCount())

		// All requests should have failed
		if mockClient.GetRequestCount() != numRequests {
			t.Errorf("Expected %d request attempts, got %d", numRequests, mockClient.GetRequestCount())
		}
	})
}

// TestGatewayErrorResponseFormattingConnectionFailures tests proper error response formatting for connection failures
func TestGatewayErrorResponseFormattingConnectionFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		mockError       error
		expectedCode    int
		expectedMessage string
	}{
		{
			name:            "TCP connection refused",
			mockError:       fmt.Errorf("dial tcp [::1]:8080: connect: connection refused"),
			expectedCode:    InternalError,
			expectedMessage: "connection refused",
		},
		{
			name:            "network unreachable",
			mockError:       fmt.Errorf("dial tcp 192.0.2.1:80: connect: network is unreachable"),
			expectedCode:    InternalError,
			expectedMessage: "network",
		},
		{
			name:            "timeout error",
			mockError:       fmt.Errorf("dial tcp 198.51.100.1:80: i/o timeout"),
			expectedCode:    InternalError,
			expectedMessage: "timeout",
		},
		{
			name:            "no such host",
			mockError:       fmt.Errorf("dial tcp: lookup nonexistent.example: no such host"),
			expectedCode:    InternalError,
			expectedMessage: "no such host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := NewMockLSPClient()
			mockClient.SetRequestError(tt.mockError)

			cfg := &config.GatewayConfig{
				Port: 8080,
				Servers: []config.ServerConfig{
					{Name: "test-server", Languages: []string{"go"}, Command: "test", Transport: "tcp"},
				},
			}

			gateway := &Gateway{
				config:  cfg,
				clients: make(map[string]transport.LSPClient),
				router:  NewRouter(),
			}
			gateway.clients["test-server"] = mockClient

			request := JSONRPCRequest{
				JSONRPC: JSONRPCVersion,
				ID:      1,
				Method:  "textDocument/definition",
				Params: map[string]interface{}{
					"textDocument": map[string]interface{}{"uri": "file:///test.go"},
					"position":     map[string]interface{}{"line": 0, "character": 5},
				},
			}

			body, err := json.Marshal(request)
			if err != nil {
				t.Fatalf("Failed to marshal request: %v", err)
			}

			req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			gateway.HandleJSONRPC(w, req)

			var response JSONRPCResponse
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if response.Error == nil {
				t.Error("Expected error response")
				return
			}

			if response.Error.Code != tt.expectedCode {
				t.Errorf("Expected error code %d, got %d", tt.expectedCode, response.Error.Code)
			}

			if !strings.Contains(strings.ToLower(response.Error.Message), strings.ToLower(tt.expectedMessage)) {
				t.Errorf("Expected error message containing '%s', got: %s", tt.expectedMessage, response.Error.Message)
			}

			// Response should have matching ID
			if response.ID != request.ID {
				t.Errorf("Expected response ID %v, got %v", request.ID, response.ID)
			}

			// Response should be valid JSON-RPC
			if response.JSONRPC != JSONRPCVersion {
				t.Errorf("Expected JSON-RPC version %s, got %s", JSONRPCVersion, response.JSONRPC)
			}
		})
	}
}

// TestGatewayClientInactiveConnectionFailures tests handling when clients become inactive due to connection failures
func TestGatewayClientInactiveConnectionFailures(t *testing.T) {
	t.Parallel()

	t.Run("client becomes inactive during request", func(t *testing.T) {
		mockClient := NewMockLSPClient()

		cfg := &config.GatewayConfig{
			Port: 8080,
			Servers: []config.ServerConfig{
				{Name: "test-server", Languages: []string{"go"}, Command: "test", Transport: "tcp"},
			},
		}

		gateway := &Gateway{
			config:  cfg,
			clients: make(map[string]transport.LSPClient),
			router:  NewRouter(),
		}
		gateway.clients["test-server"] = mockClient

		// Start the client
		ctx := context.Background()
		if err := mockClient.Start(ctx); err != nil {
			t.Fatalf("Failed to start mock client: %v", err)
		}

		// Stop the client to simulate connection failure
		if err := mockClient.Stop(); err != nil {
			t.Logf("Warning: Failed to stop mock client: %v", err)
		}

		request := JSONRPCRequest{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "textDocument/definition",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": "file:///test.go"},
				"position":     map[string]interface{}{"line": 0, "character": 5},
			},
		}

		body, err := json.Marshal(request)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		gateway.HandleJSONRPC(w, req)

		var response JSONRPCResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Error == nil {
			t.Error("Expected error response when client is inactive")
		}
	})
}

// TestGatewayConcurrentConnectionFailures tests concurrent connection failure scenarios under load
func TestGatewayConcurrentConnectionFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent connection failure test in short mode")
	}

	t.Parallel()

	t.Run("concurrent requests to unavailable servers", func(t *testing.T) {
		const numConcurrent = 50

		mockClient := NewMockLSPClient()
		mockClient.SetRequestError(fmt.Errorf("connection refused"))

		cfg := &config.GatewayConfig{
			Port: 8080,
			Servers: []config.ServerConfig{
				{Name: "test-server", Languages: []string{"go"}, Command: "localhost:65517", Transport: "tcp"},
			},
		}

		gateway := &Gateway{
			config:  cfg,
			clients: make(map[string]transport.LSPClient),
			router:  NewRouter(),
		}
		gateway.clients["test-server"] = mockClient

		var wg sync.WaitGroup
		errors := make(chan error, numConcurrent)
		successes := make(chan bool, numConcurrent)

		for i := 0; i < numConcurrent; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				request := JSONRPCRequest{
					JSONRPC: JSONRPCVersion,
					ID:      id,
					Method:  "textDocument/definition",
					Params: map[string]interface{}{
						"textDocument": map[string]interface{}{"uri": "file:///test.go"},
						"position":     map[string]interface{}{"line": 0, "character": 5},
					},
				}

				body, err := json.Marshal(request)
				if err != nil {
					errors <- fmt.Errorf("marshal error for request %d: %w", id, err)
					return
				}

				req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()

				// Should not panic or deadlock
				gateway.HandleJSONRPC(w, req)

				var response JSONRPCResponse
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					errors <- fmt.Errorf("decode error for request %d: %w", id, err)
					return
				}

				if response.Error != nil {
					successes <- true // Error response is expected and successful handling
				} else {
					errors <- fmt.Errorf("request %d: expected error response", id)
				}
			}(i)
		}

		// Wait for all requests to complete
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(30 * time.Second):
			t.Fatal("Test timed out - possible deadlock in concurrent connection failure handling")
		}

		close(errors)
		close(successes)

		errorCount := 0
		successCount := 0

		for err := range errors {
			errorCount++
			t.Logf("Concurrent test error: %v", err)
		}

		for range successes {
			successCount++
		}

		t.Logf("Concurrent connection failure test: %d successes, %d errors out of %d total", successCount, errorCount, numConcurrent)

		// All requests should have been handled (either success or controlled error)
		totalHandled := successCount + errorCount
		if totalHandled != numConcurrent {
			t.Errorf("Expected %d total handled requests, got %d", numConcurrent, totalHandled)
		}

		// Should have mostly successful error handling
		if errorCount > numConcurrent/10 {
			t.Errorf("Too many unhandled errors (%d/%d)", errorCount, numConcurrent)
		}
	})
}

// TestGatewayResourceCleanupConnectionFailures tests proper resource cleanup after connection failures
func TestGatewayResourceCleanupConnectionFailures(t *testing.T) {
	t.Parallel()

	t.Run("cleanup after multiple connection failures", func(t *testing.T) {
		mockClient1 := NewMockLSPClient()
		mockClient2 := NewMockLSPClient()

		mockClient1.SetStartError(fmt.Errorf("connection refused"))
		mockClient2.SetStartError(fmt.Errorf("connection refused"))

		cfg := &config.GatewayConfig{
			Port: 8080,
			Servers: []config.ServerConfig{
				{Name: "server1", Languages: []string{"go"}, Command: "localhost:65516", Transport: "tcp"},
				{Name: "server2", Languages: []string{"python"}, Command: "localhost:65515", Transport: "tcp"},
			},
		}

		gateway := &Gateway{
			config:  cfg,
			clients: make(map[string]transport.LSPClient),
			router:  NewRouter(),
		}
		gateway.clients["server1"] = mockClient1
		gateway.clients["server2"] = mockClient2

		// Try to start gateway - should handle connection failures gracefully
		err := gateway.Start(context.Background())
		if err == nil {
			t.Error("Expected error when starting gateway with failing clients")
		}

		// Stop should be safe even after failed start
		err = gateway.Stop()
		if err != nil {
			t.Logf("Stop error after failed start: %v", err)
		}

		// Verify cleanup was attempted
		if mockClient1.GetStopCount() == 0 {
			t.Error("Expected stop to be called on client1")
		}
		if mockClient2.GetStopCount() == 0 {
			t.Error("Expected stop to be called on client2")
		}
	})

	t.Run("graceful degradation with partial connection failures", func(t *testing.T) {
		mockClient1 := NewMockLSPClient() // Working client
		mockClient2 := NewMockLSPClient() // Failing client

		mockClient2.SetStartError(fmt.Errorf("connection refused"))

		cfg := &config.GatewayConfig{
			Port: 8080,
			Servers: []config.ServerConfig{
				{Name: "working-server", Languages: []string{"go"}, Command: "working", Transport: "stdio"},
				{Name: "failing-server", Languages: []string{"python"}, Command: "localhost:65514", Transport: "tcp"},
			},
		}

		gateway := &Gateway{
			config:  cfg,
			clients: make(map[string]transport.LSPClient),
			router:  NewRouter(),
		}
		gateway.clients["working-server"] = mockClient1
		gateway.clients["failing-server"] = mockClient2

		// Gateway might start with partial success
		err := gateway.Start(context.Background())
		if err != nil {
			t.Logf("Gateway start with partial failures: %v", err)
		}

		// Test request to working server
		mockClient1.SetResponse("textDocument/definition", json.RawMessage(`{"result": "working"}`))

		request := JSONRPCRequest{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "textDocument/definition",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": "file:///test.go"},
				"position":     map[string]interface{}{"line": 0, "character": 5},
			},
		}

		body, err := json.Marshal(request)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		gateway.HandleJSONRPC(w, req)

		var response JSONRPCResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Working server should still work despite other server failures
		if mockClient1.IsActive() && response.Error != nil {
			t.Errorf("Expected successful response from working server, got error: %v", response.Error)
		}

		// Cleanup
		if err := gateway.Stop(); err != nil {
			t.Logf("Warning: Failed to stop gateway: %v", err)
		}
	})
}