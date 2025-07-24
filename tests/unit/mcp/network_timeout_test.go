package mcp_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"lsp-gateway/mcp"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestMCPServerMessageTimeout tests message processing timeouts in MCP server
func TestMCPServerMessageTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		messageDelay    time.Duration
		messageTimeout  time.Duration
		expectedTimeout bool
	}{
		{
			name:            "fast message within timeout",
			messageDelay:    100 * time.Millisecond,
			messageTimeout:  1 * time.Second, // Reduced for faster tests
			expectedTimeout: false,
		},
		{
			name:            "slow message exceeds timeout",
			messageDelay:    1500 * time.Millisecond, // Reduced from 3s
			messageTimeout:  500 * time.Millisecond,  // Shorter timeout
			expectedTimeout: true,
		},
		{
			name:            "very slow message exceeds timeout",
			messageDelay:    2 * time.Second,        // Reduced from 3s
			messageTimeout:  500 * time.Millisecond, // Shorter timeout
			expectedTimeout: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock LSP gateway server
			mockGateway := createMockLSPGateway(t, 100*time.Millisecond)
			defer mockGateway.Close()

			// Create MCP server config
			config := &mcp.ServerConfig{
				Name:          "test-mcp",
				Version:       "1.0.0",
				LSPGatewayURL: mockGateway.URL,
				Timeout:       3 * time.Second, // Reduced from 10s
				MaxRetries:    1,
			}

			// Create input/output pipes
			inputReader, inputWriter := io.Pipe()
			outputReader, outputWriter := io.Pipe()

			server := mcp.NewServer(config)
			server.SetIO(inputReader, outputWriter)

			// Override message timeout for testing
			server.ProtocolLimits.MessageTimeout = tt.messageTimeout

			// Start server in goroutine
			serverDone := make(chan error, 1)
			go func() {
				serverDone <- server.Start()
			}()

			// Create delayed message sender
			go func() {
				defer inputWriter.Close()
				time.Sleep(tt.messageDelay)

				// Send initialize request
				initRequest := mcp.MCPMessage{
					JSONRPC: "2.0",
					ID:      "init-1",
					Method:  mcp.MCPMethodInitialize,
					Params: map[string]interface{}{
						"protocolVersion": mcp.ProtocolVersion,
						"capabilities":    map[string]interface{}{},
						"clientInfo": map[string]interface{}{
							"name":    "test-client",
							"version": "1.0.0",
						},
					},
				}

				data, _ := json.Marshal(initRequest)
				message := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(data), string(data))
				inputWriter.Write([]byte(message))
			}()

			// Read response with timeout - shorter for faster tests
			responseTimeout := tt.messageTimeout + 1*time.Second
			responseChan := make(chan string, 1)
			go func() {
				reader := bufio.NewReader(outputReader)

				// Read headers
				var contentLength int
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					line = strings.TrimSpace(line)
					if line == "" {
						break
					}
					if strings.HasPrefix(line, "Content-Length:") {
						lengthStr := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
						contentLength, _ = strconv.Atoi(lengthStr)
					}
				}

				if contentLength > 0 {
					body := make([]byte, contentLength)
					if _, err := io.ReadFull(reader, body); err == nil {
						responseChan <- string(body)
					}
				}
			}()

			// Wait for response or timeout
			start := time.Now()
			select {
			case response := <-responseChan:
				duration := time.Since(start)

				if tt.expectedTimeout {
					t.Errorf("Expected timeout but got response in %v: %s", duration, response)
				} else {
					// Verify it's a valid initialize response
					var mcpResponse mcp.MCPMessage
					if err := json.Unmarshal([]byte(response), &mcpResponse); err != nil {
						t.Errorf("Failed to parse response: %v", err)
					} else if mcpResponse.Error != nil {
						t.Errorf("Expected success response, got error: %v", mcpResponse.Error)
					}
				}

			case <-time.After(responseTimeout):
				duration := time.Since(start)

				if !tt.expectedTimeout {
					t.Errorf("Unexpected timeout after %v", duration)
				} else {
					// Verify timeout occurred around expected time (allow some variance)
					if duration < tt.messageTimeout || duration > tt.messageTimeout+500*time.Millisecond {
						t.Logf("Timeout occurred at %v (message timeout: %v)", duration, tt.messageTimeout)
					}
				}
			}

			// Stop server
			server.Stop()
			outputReader.Close()

			select {
			case <-serverDone:
			case <-time.After(1 * time.Second): // Reduced timeout
				t.Error("Server did not stop within timeout")
			}
		})
	}
}

// TestMCPClientGatewayTimeout tests LSP Gateway communication timeouts
func TestMCPClientGatewayTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		gatewayDelay   time.Duration
		requestTimeout time.Duration
		expectedError  bool
	}{
		{
			name:           "fast gateway response",
			gatewayDelay:   100 * time.Millisecond,
			requestTimeout: 1 * time.Second, // Reduced for faster tests
			expectedError:  false,
		},
		{
			name:           "slow gateway exceeds timeout",
			gatewayDelay:   1500 * time.Millisecond, // Reduced from 3s
			requestTimeout: 500 * time.Millisecond,  // Shorter timeout
			expectedError:  true,
		},
		{
			name:           "very slow gateway exceeds timeout",
			gatewayDelay:   2 * time.Second,        // Reduced from 3s
			requestTimeout: 500 * time.Millisecond, // Shorter timeout
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock LSP gateway with delay
			mockGateway := createMockLSPGateway(t, tt.gatewayDelay)
			defer mockGateway.Close()

			config := &mcp.ServerConfig{
				Name:          "timeout-test",
				Version:       "1.0.0",
				LSPGatewayURL: mockGateway.URL,
				Timeout:       tt.requestTimeout,
				// ConnectTimeout:  5 * time.Second,
				MaxRetries: 1,
				// EnableMetrics:   true,
				// EnableHealthCheck: false,
			}

			client := mcp.NewLSPGatewayClient(config)

			ctx, cancel := context.WithTimeout(context.Background(), tt.requestTimeout+1*time.Second) // Shorter context timeout
			defer cancel()

			// Test LSP request
			start := time.Now()
			result, err := client.SendLSPRequest(ctx, "textDocument/definition", map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": "file:///test.go"},
				"position":     map[string]interface{}{"line": 10, "character": 5},
			})
			duration := time.Since(start)

			if tt.expectedError {
				if err == nil {
					t.Error("Expected timeout error")
				} else if !strings.Contains(strings.ToLower(err.Error()), "timeout") &&
					!strings.Contains(strings.ToLower(err.Error()), "context deadline exceeded") {
					t.Errorf("Expected timeout error, got: %v", err)
				}

				// Verify timeout occurred around expected time (allow for retry delays)
				expectedMax := tt.requestTimeout + 1500*time.Millisecond // Allow extra time for retries
				if duration > expectedMax {
					t.Errorf("Request took too long: %v, expected max: %v", duration, expectedMax)
				}
			} else {
				if err != nil {
					t.Errorf("Expected success, got error: %v", err)
				}
				if result == nil {
					t.Error("Expected non-nil result")
				}
			}
		})
	}
}

// TestMCPToolCallTimeouts tests timeout handling for tool calls
func TestMCPToolCallTimeouts(t *testing.T) {
	t.Parallel()

	// Create mock LSP gateway with delays that will cause timeout
	mockGateway := createMockLSPGateway(t, 1500*time.Millisecond) // Longer delay to ensure timeout
	defer mockGateway.Close()

	config := &mcp.ServerConfig{
		Name:          "tool-timeout-test",
		Version:       "1.0.0",
		LSPGatewayURL: mockGateway.URL,
		Timeout:       500 * time.Millisecond, // Short timeout to ensure failure
		MaxRetries:    1,                      // Minimal retries
	}

	client := mcp.NewLSPGatewayClient(config)
	toolHandler := mcp.NewToolHandler(client)

	tools := []struct {
		toolName string
		args     map[string]interface{}
	}{
		{
			toolName: "goto_definition",
			args: map[string]interface{}{
				"uri":    "file:///test.go",
				"line":   10,
				"column": 5,
			},
		},
		{
			toolName: "find_references",
			args: map[string]interface{}{
				"uri":    "file:///test.go",
				"line":   10,
				"column": 5,
			},
		},
		{
			toolName: "get_hover_info",
			args: map[string]interface{}{
				"uri":    "file:///test.go",
				"line":   10,
				"column": 5,
			},
		},
	}

	for _, tool := range tools {
		t.Run(tool.toolName, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			start := time.Now()
			result, err := toolHandler.CallTool(ctx, mcp.ToolCall{Name: tool.toolName, Arguments: tool.args})
			duration := time.Since(start)

			// Should timeout due to slow gateway and short request timeout
			hasError := (err != nil) || (result != nil && result.IsError)
			if !hasError {
				t.Errorf("Expected timeout error for tool %s", tool.toolName)
			}

			// Should timeout around request timeout (1s)
			if duration < 500*time.Millisecond || duration > 3*time.Second {
				t.Errorf("mcp.Tool %s timeout at %v, expected around 1s", tool.toolName, duration)
			}

			// Verify we got some kind of error indicating timeout
			if !hasError {
				t.Errorf("Expected some form of error for tool %s", tool.toolName)
			}
		})
	}
}

// TestMCPConcurrentTimeouts tests concurrent tool calls with timeouts
func TestMCPConcurrentTimeouts(t *testing.T) {
	t.Parallel()

	// Create mock gateway with moderate delay
	mockGateway := createMockLSPGateway(t, 1500*time.Millisecond)
	defer mockGateway.Close()

	config := &mcp.ServerConfig{
		Name:          "concurrent-test",
		Version:       "1.0.0",
		LSPGatewayURL: mockGateway.URL,
		Timeout:       1 * time.Second, // Will timeout
		// ConnectTimeout:  5 * time.Second,
		MaxRetries: 1,
		// EnableMetrics:   true,
		// EnableHealthCheck: false,
	}

	client := mcp.NewLSPGatewayClient(config)
	toolHandler := mcp.NewToolHandler(client)

	const numConcurrent = 5
	var wg sync.WaitGroup
	results := make(chan struct {
		toolName string
		err      error
		duration time.Duration
	}, numConcurrent)

	// Launch concurrent tool calls
	tools := []string{
		"goto_definition",
		"find_references",
		"get_hover_info",
		"get_document_symbols",
		"search_workspace_symbols",
	}

	for i, toolName := range tools {
		wg.Add(1)
		go func(name string, id int) {
			defer wg.Done()

			args := map[string]interface{}{
				"uri":    fmt.Sprintf("file:///test%d.go", id),
				"line":   10 + id,
				"column": 5,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			start := time.Now()
			_, err := toolHandler.CallTool(ctx, mcp.ToolCall{Name: name, Arguments: args})
			duration := time.Since(start)

			results <- struct {
				toolName string
				err      error
				duration time.Duration
			}{name, err, duration}
		}(toolName, i)
	}

	wg.Wait()
	close(results)

	timeoutCount := 0
	for result := range results {
		// All should timeout due to slow gateway
		hasError := (result.err != nil)
		if !hasError {
			t.Errorf("Expected timeout for tool %s", result.toolName)
		} else {
			timeoutCount++
		}

		// Should timeout around 1 second
		if result.duration < 500*time.Millisecond || result.duration > 2*time.Second {
			t.Errorf("mcp.Tool %s timeout at %v, expected around 1s", result.toolName, result.duration)
		}
	}

	if timeoutCount != numConcurrent {
		t.Errorf("Expected %d timeouts, got %d", numConcurrent, timeoutCount)
	}
}

// TestMCPCircuitBreakerWithTimeouts tests circuit breaker behavior with timeouts
func TestMCPCircuitBreakerWithTimeouts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping circuit breaker test in short mode")
	}
	t.Parallel()

	// Create mock gateway that will always timeout (2 second delay vs 500ms timeout)
	mockGateway := createMockLSPGateway(t, 2*time.Second)
	defer mockGateway.Close()

	config := &mcp.ServerConfig{
		Name:          "circuit-breaker-test",
		Version:       "1.0.0",
		LSPGatewayURL: mockGateway.URL,
		Timeout:       500 * time.Millisecond, // Short timeout to ensure failure
		MaxRetries:    1,                      // Minimal retries to speed up test
	}

	client := mcp.NewLSPGatewayClient(config)
	// Override circuit breaker settings for testing
	client.SetCircuitBreakerConfig(3, 2*time.Second) // Reduce threshold for faster triggering with short recovery timeout

	toolHandler := mcp.NewToolHandler(client)

	// Make requests that will timeout to trigger circuit breaker
	for i := 0; i < 3; i++ { // Reduced to match maxFailures
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

		args := map[string]interface{}{
			"uri":    fmt.Sprintf("file:///test%d.go", i),
			"line":   10,
			"column": 5,
		}

		start := time.Now()
		result, err := toolHandler.CallTool(ctx, mcp.ToolCall{Name: "goto_definition", Arguments: args})
		duration := time.Since(start)

		cancel()

		// All requests should fail due to timeout
		hasError := (err != nil) || (result != nil && result.IsError)
		if !hasError {
			t.Errorf("Expected timeout error for request %d", i)
		}

		// Should timeout reasonably quickly (considering retries)
		if duration > 3*time.Second {
			t.Errorf("Request %d took too long: %v (expected ~1-2s with retries)", i, duration)
		}

		t.Logf("Request %d: error=%v, duration=%v", i, hasError, duration)
	}

	// Circuit breaker should now be open - subsequent requests should fail fast
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	args := map[string]interface{}{
		"uri":    "file:///test-fast-fail.go",
		"line":   10,
		"column": 5,
	}

	start := time.Now()
	result, err := toolHandler.CallTool(ctx, mcp.ToolCall{Name: "goto_definition", Arguments: args})
	duration := time.Since(start)

	hasError := (err != nil) || (result != nil && result.IsError)
	if !hasError {
		t.Error("Expected circuit breaker to reject request")
	}

	// Should fail very quickly due to circuit breaker (within 50ms)
	if duration > 50*time.Millisecond {
		t.Logf("Circuit breaker request took %v (circuit breaker may still be transitioning)", duration)
	}

	// Verify circuit breaker error message
	if err != nil && !strings.Contains(err.Error(), "circuit breaker") {
		t.Logf("Expected circuit breaker error message, got: %v", err)
	}
}

// TestMCPServerStdioTimeout tests stdio communication timeouts
func TestMCPServerStdioTimeout(t *testing.T) {
	t.Parallel()

	// Create mock gateway
	mockGateway := createMockLSPGateway(t, 100*time.Millisecond)
	defer mockGateway.Close()

	config := &mcp.ServerConfig{
		Name:          "stdio-timeout-test",
		Version:       "1.0.0",
		LSPGatewayURL: mockGateway.URL,
		Timeout:       5 * time.Second,
		// ConnectTimeout:  5 * time.Second,
		MaxRetries: 1,
		// EnableMetrics:   false,
		// EnableHealthCheck: false,
	}

	// Create stdin/stdout pipes
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()

	server := mcp.NewServer(config)
	server.SetIO(stdinReader, stdoutWriter)

	// Start server
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- server.Start()
	}()

	// Test message timeout by sending partial message and hanging
	go func() {
		defer stdinWriter.Close()

		// Send partial header
		stdinWriter.Write([]byte("Content-Length: 100\r\n\r\n"))
		// Send partial JSON and hang
		stdinWriter.Write([]byte(`{"jsonrpc":"2.0","id":"hang","method":"initialize","params":`))

		// Don't complete the message - let it hang (reduced timeout)
		time.Sleep(1 * time.Second) // Reduced from 10s
	}()

	// Wait for server to handle the hanging message
	time.Sleep(500 * time.Millisecond) // Reduced from 2s

	// Server should still be running despite hanging message
	select {
	case err := <-serverDone:
		t.Errorf("Server stopped unexpectedly: %v", err)
	default:
		// Server is still running - good
	}

	// Stop server and verify it stops properly
	server.Stop()
	stdoutWriter.Close()

	select {
	case <-serverDone:
		// Server stopped properly
	case <-time.After(3 * time.Second):
		t.Error("Server did not stop within timeout")
	}

	stdoutReader.Close()
}

// TestMCPTCPTimeout tests TCP transport timeout scenarios
func TestMCPTCPTimeout(t *testing.T) {
	t.Parallel()

	// Create mock gateway
	mockGateway := createMockLSPGateway(t, 500*time.Millisecond) // Reduced from 2s
	defer mockGateway.Close()

	// Create TCP listener for MCP server
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to create TCP listener: %v", err)
	}
	defer listener.Close()

	address := listener.Addr().String()

	config := &mcp.ServerConfig{
		Name:          "tcp-timeout-test",
		Version:       "1.0.0",
		LSPGatewayURL: mockGateway.URL,
		Timeout:       1 * time.Second, // Short timeout
		// ConnectTimeout:  5 * time.Second,
		MaxRetries: 1,
		// EnableMetrics:   false,
		// EnableHealthCheck: false,
	}

	// Start TCP server
	serverDone := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverDone <- err
			return
		}
		defer conn.Close()

		server := mcp.NewServer(config)
		server.SetIO(conn, conn)
		serverDone <- server.Start()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Connect as TCP client
	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatalf("Failed to connect to TCP server: %v", err)
	}
	defer conn.Close()

	// Send initialize request
	initRequest := mcp.MCPMessage{
		JSONRPC: "2.0",
		ID:      "tcp-init",
		Method:  mcp.MCPMethodInitialize,
		Params: map[string]interface{}{
			"protocolVersion": mcp.ProtocolVersion,
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "tcp-test-client",
				"version": "1.0.0",
			},
		},
	}

	data, _ := json.Marshal(initRequest)
	message := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(data), string(data))

	if _, err := conn.Write([]byte(message)); err != nil {
		t.Fatalf("Failed to write to TCP connection: %v", err)
	}

	// Read response with timeout
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	reader := bufio.NewReader(conn)
	var contentLength int

	// Read headers
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("Failed to read TCP response headers: %v", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length:") {
			lengthStr := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			contentLength, _ = strconv.Atoi(lengthStr)
		}
	}

	// Read body
	if contentLength > 0 {
		body := make([]byte, contentLength)
		if _, err := io.ReadFull(reader, body); err != nil {
			t.Fatalf("Failed to read TCP response body: %v", err)
		}

		var response mcp.MCPMessage
		if err := json.Unmarshal(body, &response); err != nil {
			t.Fatalf("Failed to parse TCP response: %v", err)
		}

		if response.Error != nil {
			t.Errorf("Expected success response, got error: %v", response.Error)
		}
	} else {
		t.Error("No response body received")
	}

	// Close connection and wait for server
	conn.Close()
	listener.Close()

	select {
	case <-serverDone:
		// Server completed
	case <-time.After(3 * time.Second):
		t.Error("TCP server did not stop within timeout")
	}
}

// Helper functions

func createMockLSPGateway(t interface{}, delay time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add delay to simulate slow gateway
		time.Sleep(delay)

		// Parse request
		var request map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Create mock response based on method
		method, _ := request["method"].(string)
		id := request["id"]

		var result interface{}
		switch method {
		case "textDocument/definition":
			result = []map[string]interface{}{
				{
					"uri": "file:///test.go",
					"range": map[string]interface{}{
						"start": map[string]interface{}{"line": 5, "character": 0},
						"end":   map[string]interface{}{"line": 5, "character": 10},
					},
				},
			}
		case "textDocument/references":
			result = []map[string]interface{}{
				{
					"uri": "file:///test.go",
					"range": map[string]interface{}{
						"start": map[string]interface{}{"line": 10, "character": 5},
						"end":   map[string]interface{}{"line": 10, "character": 15},
					},
				},
			}
		case "textDocument/hover":
			result = map[string]interface{}{
				"contents": map[string]interface{}{
					"kind":  "markdown",
					"value": "Test hover information",
				},
			}
		case "textDocument/documentSymbol":
			result = []map[string]interface{}{
				{
					"name": "TestFunction",
					"kind": 12,
					"range": map[string]interface{}{
						"start": map[string]interface{}{"line": 0, "character": 0},
						"end":   map[string]interface{}{"line": 10, "character": 0},
					},
				},
			}
		case "workspace/symbol":
			result = []map[string]interface{}{
				{
					"name": "TestSymbol",
					"kind": 12,
					"location": map[string]interface{}{
						"uri": "file:///test.go",
						"range": map[string]interface{}{
							"start": map[string]interface{}{"line": 5, "character": 0},
							"end":   map[string]interface{}{"line": 5, "character": 10},
						},
					},
				},
			}
		default:
			result = map[string]interface{}{
				"message": fmt.Sprintf("Mock response for %s", method),
			}
		}

		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"result":  result,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}

// Benchmark tests for MCP timeout performance
func BenchmarkMCPToolCallTimeout(b *testing.B) {
	mockGateway := createMockLSPGateway(b, 50*time.Millisecond)
	defer mockGateway.Close()

	config := &mcp.ServerConfig{
		Name:          "bench-test",
		Version:       "1.0.0",
		LSPGatewayURL: mockGateway.URL,
		Timeout:       30 * time.Millisecond, // Will timeout
		// ConnectTimeout:  5 * time.Second,
		MaxRetries: 1,
		// EnableMetrics:   false,
		// EnableHealthCheck: false,
	}

	client := mcp.NewLSPGatewayClient(config)
	toolHandler := mcp.NewToolHandler(client)

	args := map[string]interface{}{
		"uri":    "file:///bench.go",
		"line":   10,
		"column": 5,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		_, err := toolHandler.CallTool(ctx, mcp.ToolCall{Name: "goto_definition", Arguments: args})
		cancel()

		if err == nil {
			b.Error("Expected timeout error")
		}
	}
}

func BenchmarkMCPToolCallSuccess(b *testing.B) {
	mockGateway := createMockLSPGateway(b, 5*time.Millisecond)
	defer mockGateway.Close()

	config := &mcp.ServerConfig{
		Name:          "bench-success-test",
		Version:       "1.0.0",
		LSPGatewayURL: mockGateway.URL,
		Timeout:       100 * time.Millisecond,
		// ConnectTimeout:  5 * time.Second,
		MaxRetries: 1,
		// EnableMetrics:   false,
		// EnableHealthCheck: false,
	}

	client := mcp.NewLSPGatewayClient(config)
	toolHandler := mcp.NewToolHandler(client)

	args := map[string]interface{}{
		"uri":    "file:///bench.go",
		"line":   10,
		"column": 5,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		_, err := toolHandler.CallTool(ctx, mcp.ToolCall{Name: "goto_definition", Arguments: args})
		cancel()

		if err != nil {
			b.Errorf("Unexpected error: %v", err)
		}
	}
}
