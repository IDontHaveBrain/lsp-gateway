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

// TestGatewayResourceCleanupErrors tests error conditions during resource cleanup
func TestGatewayResourceCleanupErrors(t *testing.T) {
	t.Parallel()

	t.Run("cleanup with failing clients", func(t *testing.T) {
		cfg := &config.GatewayConfig{
			Port: 8080,
			Servers: []config.ServerConfig{
				{Name: "test1", Languages: []string{"go"}, Command: "gopls", Transport: "stdio"},
				{Name: "test2", Languages: []string{"python"}, Command: "pylsp", Transport: "stdio"},
			},
		}

		gateway := &Gateway{
			config:  cfg,
			clients: make(map[string]transport.LSPClient),
			router:  NewRouter(),
		}

		// Add mock clients that will fail during stop
		failingClient1 := NewMockLSPClient()
		failingClient2 := NewMockLSPClient()

		failingClient1.SetStopError(fmt.Errorf("client1 stop failed"))
		failingClient2.SetStopError(fmt.Errorf("client2 stop failed"))

		gateway.clients["test1"] = failingClient1
		gateway.clients["test2"] = failingClient2

		// Start both clients
		ctx := context.Background()
		if err := failingClient1.Start(ctx); err != nil {
			t.Fatalf("Failed to start client1: %v", err)
		}
		if err := failingClient2.Start(ctx); err != nil {
			t.Fatalf("Failed to start client2: %v", err)
		}

		// Stop should handle errors gracefully
		err := gateway.Stop()
		if err == nil {
			t.Error("Expected error during stop with failing clients")
		}

		// Should still have attempted to stop all clients
		if failingClient1.GetStopCount() == 0 {
			t.Error("Expected stop to be called on client1")
		}
		if failingClient2.GetStopCount() == 0 {
			t.Error("Expected stop to be called on client2")
		}
	})
}

// TestGatewayMemoryLeakPrevention tests scenarios that could cause memory leaks
func TestGatewayMemoryLeakPrevention(t *testing.T) {
	t.Parallel()

	t.Run("repeated start/stop cycles", func(t *testing.T) {
		cfg := &config.GatewayConfig{
			Port: 8080,
			Servers: []config.ServerConfig{
				{Name: "test", Languages: []string{"go"}, Command: "gopls", Transport: "stdio"},
			},
		}

		gateway := &Gateway{
			config:  cfg,
			clients: make(map[string]transport.LSPClient),
			router:  NewRouter(),
		}

		mockClient := NewMockLSPClient()
		gateway.clients["test"] = mockClient

		// Perform multiple start/stop cycles
		for i := 0; i < 10; i++ {
			err := gateway.Start(context.Background())
			if err != nil {
				t.Fatalf("Failed to start gateway on iteration %d: %v", i, err)
			}

			err = gateway.Stop()
			if err != nil {
				t.Fatalf("Failed to stop gateway on iteration %d: %v", i, err)
			}
		}

		// Verify client was properly managed
		if mockClient.GetStartCount() != 10 {
			t.Errorf("Expected 10 start calls, got %d", mockClient.GetStartCount())
		}
		if mockClient.GetStopCount() != 10 {
			t.Errorf("Expected 10 stop calls, got %d", mockClient.GetStopCount())
		}
	})
}

// TestGatewayHighLoadErrorHandling tests error handling under high load
func TestGatewayHighLoadErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high load test in short mode")
	}

	t.Parallel()

	gateway, mockClients := createTestGatewayForHandlers(t)

	// Set up a mock client that occasionally fails
	mockClient := mockClients["gopls"]

	// Test concurrent requests with some failures
	const numRequests = 100
	errors := make(chan error, numRequests)
	successes := make(chan bool, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(requestID int) {
			// Every 10th request should fail
			if requestID%10 == 0 {
				mockClient.SetRequestError(fmt.Errorf("simulated failure %d", requestID))
			} else {
				mockClient.SetRequestError(nil)
			}

			request := JSONRPCRequest{
				JSONRPC: JSONRPCVersion,
				ID:      requestID,
				Method:  "textDocument/definition",
				Params: map[string]interface{}{
					"textDocument": map[string]interface{}{"uri": "file:///test.go"},
					"position":     map[string]interface{}{"line": 0, "character": 5},
				},
			}

			body, err := json.Marshal(request)
			if err != nil {
				errors <- fmt.Errorf("failed to marshal JSON request: %v", err)
				return
			}
			req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			gateway.HandleJSONRPC(w, req)

			var response JSONRPCResponse
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				errors <- fmt.Errorf("decode error for request %d: %v", requestID, err)
				return
			}

			if response.Error != nil {
				errors <- fmt.Errorf("request %d failed: %v", requestID, response.Error)
			} else {
				successes <- true
			}
		}(i)
	}

	// Collect results
	errorCount := 0
	successCount := 0
	timeout := time.After(30 * time.Second)

	for i := 0; i < numRequests; i++ {
		select {
		case <-errors:
			errorCount++
		case <-successes:
			successCount++
		case <-timeout:
			t.Fatal("Test timed out waiting for requests to complete")
		}
	}

	t.Logf("High load test results: %d successes, %d errors", successCount, errorCount)

	// Should have some successes and some failures
	if successCount == 0 {
		t.Error("Expected some successful requests")
	}
	if errorCount == 0 {
		t.Error("Expected some failed requests")
	}
}

// TestGatewayMalformedRequestHandling tests handling of various malformed requests
func TestGatewayMalformedRequestHandling(t *testing.T) {
	t.Parallel()

	gateway, _ := createTestGatewayForHandlers(t)

	tests := []struct {
		name         string
		body         string
		contentType  string
		expectedCode int
	}{
		{
			name:         "non-JSON content type",
			body:         `{"jsonrpc": "2.0", "method": "test", "id": 1}`,
			contentType:  "text/plain",
			expectedCode: http.StatusOK, // Should still process but may fail JSON parsing
		},
		{
			name:         "extremely large JSON",
			body:         `{"jsonrpc": "2.0", "method": "test", "id": 1, "data": "` + strings.Repeat("a", 1000000) + `"}`,
			contentType:  "application/json",
			expectedCode: http.StatusOK,
		},
		{
			name:         "deeply nested JSON",
			body:         createDeeplyNestedJSON(100),
			contentType:  "application/json",
			expectedCode: http.StatusOK,
		},
		{
			name:         "null bytes in JSON",
			body:         "{\"jsonrpc\": \"2.0\", \"method\": \"test\x00\", \"id\": 1}",
			contentType:  "application/json",
			expectedCode: http.StatusOK,
		},
		{
			name:         "invalid UTF-8 in JSON",
			body:         "{\"jsonrpc\": \"2.0\", \"method\": \"test\xff\", \"id\": 1}",
			contentType:  "application/json",
			expectedCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/jsonrpc", strings.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			w := httptest.NewRecorder()

			// Should not panic or crash
			gateway.HandleJSONRPC(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status %d, got %d", tt.expectedCode, w.Code)
			}

			// Response should be valid JSON even for malformed input
			body := w.Body.String()
			if body != "" && !isValidJSON(body) {
				t.Errorf("Response is not valid JSON: %s", body)
			}
		})
	}
}

// TestGatewayTimeoutHandling tests various timeout scenarios
func TestGatewayTimeoutHandling(t *testing.T) {
	t.Parallel()

	gateway, mockClients := createTestGatewayForHandlers(t)

	t.Run("client timeout during request", func(t *testing.T) {
		mockClient := mockClients["gopls"]

		// Set up a client that will have request errors on timeout
		mockClient.SetRequestError(fmt.Errorf("timeout error"))

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
			t.Fatalf("Failed to marshal JSON request: %v", err)
		}
		req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		// Add a short timeout context
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		gateway.HandleJSONRPC(w, req)

		// Should handle timeout gracefully
		var response JSONRPCResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Should return an error for timeout
		if response.Error == nil {
			t.Error("Expected error response for timeout")
		}
	})
}

// TestGatewayRaceConditions tests for race conditions in concurrent access
func TestGatewayRaceConditions(t *testing.T) {
	t.Parallel()

	gateway, _ := createTestGatewayForHandlers(t)

	// Test concurrent start/stop with request handling
	var wg sync.WaitGroup
	const numGoroutines = 20

	// Start/stop operations
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()

			// Randomly start and stop
			if i%2 == 0 {
				if err := gateway.Start(context.Background()); err != nil {
					// Log but don't fail as this is expected in race condition test
					t.Logf("Start failed in race test: %v", err)
				}
			} else {
				if err := gateway.Stop(); err != nil {
					// Log but don't fail as this is expected in race condition test
					t.Logf("Stop failed in race test: %v", err)
				}
			}
		}()
	}

	// Concurrent requests
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
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
				// Cannot use t.Fatalf in goroutine, so just return
				return
			}
			req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Should not panic or deadlock
			gateway.HandleJSONRPC(w, req)
		}(i)
	}

	// Wait for all operations to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out - possible deadlock")
	}
}

// TestGatewayClientLifecycleErrors tests client lifecycle error scenarios
func TestGatewayClientLifecycleErrors(t *testing.T) {
	t.Parallel()

	t.Run("client fails during initialization", func(t *testing.T) {
		cfg := &config.GatewayConfig{
			Port: 8080,
			Servers: []config.ServerConfig{
				{Name: "failing", Languages: []string{"go"}, Command: "nonexistent", Transport: "stdio"},
			},
		}

		gateway, err := NewGateway(cfg)
		if err == nil {
			t.Error("Expected error when creating gateway with invalid client")
		}
		if gateway != nil {
			t.Error("Expected nil gateway when creation fails")
		}
	})

	t.Run("client becomes inactive during request", func(t *testing.T) {
		gateway, mockClients := createTestGatewayForHandlers(t)
		mockClient := mockClients["gopls"]

		// Stop the client mid-request
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
			t.Fatalf("Failed to marshal JSON request: %v", err)
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
			t.Error("Expected error when client is inactive")
		}
	})
}

// Helper functions

func createDeeplyNestedJSON(depth int) string {
	var result strings.Builder
	result.WriteString(`{"jsonrpc": "2.0", "method": "test", "id": 1, "params": `)

	for i := 0; i < depth; i++ {
		result.WriteString(`{"nested": `)
	}
	result.WriteString(`"value"`)
	for i := 0; i < depth; i++ {
		result.WriteString(`}`)
	}
	result.WriteString(`}`)

	return result.String()
}

func isValidJSON(s string) bool {
	var js interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}
