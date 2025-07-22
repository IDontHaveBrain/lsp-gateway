package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestStdioClientRequestTimeouts tests various request timeout scenarios for stdio clients
func TestStdioClientRequestTimeouts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		serverDelay    time.Duration
		requestTimeout time.Duration
		expectedError  string
	}{
		{
			name:           "1s timeout with immediate response",
			serverDelay:    0,
			requestTimeout: 1 * time.Second,
			expectedError:  "",
		},
		{
			name:           "1s timeout with 2s server delay",
			serverDelay:    2 * time.Second,
			requestTimeout: 1 * time.Second,
			expectedError:  "context deadline exceeded",
		},
		{
			name:           "5s timeout with 3s server delay",
			serverDelay:    3 * time.Second,
			requestTimeout: 5 * time.Second,
			expectedError:  "",
		},
		{
			name:           "10s timeout with 15s server delay",
			serverDelay:    15 * time.Second,
			requestTimeout: 10 * time.Second,
			expectedError:  "context deadline exceeded",
		},
		{
			name:           "30s timeout with 45s server delay",
			serverDelay:    45 * time.Second,
			requestTimeout: 30 * time.Second,
			expectedError:  "context deadline exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockServer := createDelayedMockLSPServer(t, tt.serverDelay)
			defer func() {
				if err := os.Remove(mockServer); err != nil {
					t.Logf("Warning: Failed to remove mock server: %v", err)
				}
			}()

			config := ClientConfig{
				Command:   mockServer,
				Args:      []string{},
				Transport: "stdio",
			}

			client, err := NewStdioClient(config)
			if err != nil {
				t.Fatalf("NewStdioClient failed: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			if err := client.Start(ctx); err != nil {
				t.Fatalf("Start failed: %v", err)
			}
			defer func() {
				if err := client.Stop(); err != nil {
					t.Logf("Warning: Failed to stop client: %v", err)
				}
			}()

			requestCtx, requestCancel := context.WithTimeout(ctx, tt.requestTimeout)
			defer requestCancel()

			startTime := time.Now()
			_, err = client.SendRequest(requestCtx, "test", map[string]interface{}{
				"message": "timeout test",
			})
			duration := time.Since(startTime)

			if tt.expectedError == "" {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				// Verify response came within reasonable time (server delay + buffer)
				expectedMax := tt.serverDelay + 2*time.Second
				if duration > expectedMax {
					t.Errorf("Response took too long: %v, expected max: %v", duration, expectedMax)
				}
			} else {
				if err == nil {
					t.Error("Expected timeout error")
				} else if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing '%s', got: %v", tt.expectedError, err)
				}
				// Verify timeout occurred around the expected time
				expectedTimeout := tt.requestTimeout
				if duration < expectedTimeout-500*time.Millisecond || duration > expectedTimeout+2*time.Second {
					t.Errorf("Timeout occurred at %v, expected around %v", duration, expectedTimeout)
				}
			}
		})
	}
}

// TestStdioClientConcurrentTimeouts tests concurrent requests with different timeout values
func TestStdioClientConcurrentTimeouts(t *testing.T) {
	t.Parallel()

	mockServer := createDelayedMockLSPServer(t, 3*time.Second)
	defer func() {
		if err := os.Remove(mockServer); err != nil {
			t.Logf("Warning: Failed to remove mock server: %v", err)
		}
	}()

	config := ClientConfig{
		Command:   mockServer,
		Args:      []string{},
		Transport: "stdio",
	}

	client, err := NewStdioClient(config)
	if err != nil {
		t.Fatalf("NewStdioClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := client.Stop(); err != nil {
			t.Logf("Warning: Failed to stop client: %v", err)
		}
	}()

	// Test multiple concurrent requests with different timeouts
	timeouts := []time.Duration{1 * time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second}
	var wg sync.WaitGroup
	results := make(chan struct {
		timeout time.Duration
		err     error
		start   time.Time
		end     time.Time
	}, len(timeouts))

	for _, timeout := range timeouts {
		wg.Add(1)
		go func(to time.Duration) {
			defer wg.Done()
			requestCtx, requestCancel := context.WithTimeout(ctx, to)
			defer requestCancel()

			start := time.Now()
			_, err := client.SendRequest(requestCtx, "test", map[string]interface{}{
				"timeout": to.String(),
			})
			end := time.Now()

			results <- struct {
				timeout time.Duration
				err     error
				start   time.Time
				end     time.Time
			}{to, err, start, end}
		}(timeout)
	}

	wg.Wait()
	close(results)

	timeoutCount := 0
	successCount := 0

	for result := range results {
		duration := result.end.Sub(result.start)
		if result.timeout < 3*time.Second {
			// Should timeout
			if result.err == nil {
				t.Errorf("Expected timeout for %v duration", result.timeout)
			} else {
				timeoutCount++
				// Verify timeout occurred around expected time
				if duration < result.timeout-500*time.Millisecond || duration > result.timeout+2*time.Second {
					t.Errorf("Timeout at %v for %v timeout, expected around %v", duration, result.timeout, result.timeout)
				}
			}
		} else {
			// Should succeed
			if result.err != nil {
				t.Errorf("Expected success for %v timeout, got: %v", result.timeout, result.err)
			} else {
				successCount++
			}
		}
	}

	t.Logf("Concurrent timeout test: %d timeouts, %d successes", timeoutCount, successCount)
}

// TestStdioClientLongRunningRequestTimeout tests timeout handling for long-running requests
func TestStdioClientLongRunningRequestTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}
	t.Parallel()

	mockServer := createDelayedMockLSPServer(t, 45*time.Second)
	defer func() {
		if err := os.Remove(mockServer); err != nil {
			t.Logf("Warning: Failed to remove mock server: %v", err)
		}
	}()

	config := ClientConfig{
		Command:   mockServer,
		Args:      []string{},
		Transport: "stdio",
	}

	client, err := NewStdioClient(config)
	if err != nil {
		t.Fatalf("NewStdioClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := client.Stop(); err != nil {
			t.Logf("Warning: Failed to stop client: %v", err)
		}
	}()

	// Test 30-second timeout on 45-second operation
	requestCtx, requestCancel := context.WithTimeout(ctx, 30*time.Second)
	defer requestCancel()

	start := time.Now()
	_, err = client.SendRequest(requestCtx, "long_operation", map[string]interface{}{
		"duration": "45s",
	})
	duration := time.Since(start)

	if err == nil {
		t.Error("Expected timeout error for long-running request")
	} else if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected context deadline exceeded, got: %v", err)
	}

	// Verify timeout occurred around 30 seconds
	expectedTimeout := 30 * time.Second
	if duration < expectedTimeout-2*time.Second || duration > expectedTimeout+5*time.Second {
		t.Errorf("Timeout occurred at %v, expected around %v", duration, expectedTimeout)
	}

	// Verify client remains functional after timeout
	if !client.IsActive() {
		t.Error("Client should remain active after request timeout")
	}
}

// TestTCPClientConnectionTimeout tests TCP connection establishment timeouts
func TestTCPClientConnectionTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		address       string
		expectedError string
	}{
		{
			name:          "connection to non-existent server",
			address:       "localhost:59999", // Unlikely to be in use
			expectedError: "connection refused",
		},
		{
			name:          "connection to unreachable address",
			address:       "192.0.2.1:8080", // RFC5737 test address
			expectedError: "timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ClientConfig{
				Command:   tt.address,
				Transport: "tcp",
			}

			client, err := NewTCPClient(config)
			if err != nil {
				t.Fatalf("NewTCPClient failed: %v", err)
			}

			// Use short timeout for connection attempt
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			start := time.Now()
			err = client.Start(ctx)
			duration := time.Since(start)

			if err == nil {
				t.Error("Expected connection error")
				if err := client.Stop(); err != nil {
					t.Logf("Warning: Failed to stop client: %v", err)
				}
			} else if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.expectedError)) {
				t.Errorf("Expected error containing '%s', got: %v", tt.expectedError, err)
			}

			// Verify timeout occurred within reasonable time
			if duration > 15*time.Second {
				t.Errorf("Connection attempt took too long: %v", duration)
			}
		})
	}
}

// TestTCPClientRequestTimeout tests TCP request timeout scenarios
func TestTCPClientRequestTimeout(t *testing.T) {
	t.Parallel()

	// Create a TCP server that accepts connections but responds slowly
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			t.Logf("Warning: Failed to close listener: %v", err)
		}
	}()

	address := listener.Addr().String()

	// Server that responds after a delay
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleSlowTCPConnection(conn, 5*time.Second)
		}
	}()

	config := ClientConfig{
		Command:   address,
		Transport: "tcp",
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("NewTCPClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := client.Stop(); err != nil {
			t.Logf("Warning: Failed to stop client: %v", err)
		}
	}()

	// Test 2-second timeout on 5-second response
	requestCtx, requestCancel := context.WithTimeout(ctx, 2*time.Second)
	defer requestCancel()

	start := time.Now()
	_, err = client.SendRequest(requestCtx, "test", map[string]interface{}{
		"message": "tcp timeout test",
	})
	duration := time.Since(start)

	if err == nil {
		t.Error("Expected timeout error")
	} else if !strings.Contains(err.Error(), "context deadline exceeded") && !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected timeout error, got: %v", err)
	}

	// Verify timeout occurred around 2 seconds
	if duration < 1500*time.Millisecond || duration > 4*time.Second {
		t.Errorf("Timeout occurred at %v, expected around 2s", duration)
	}
}

// TestTCPClientNetworkDelayTimeout tests timeout handling with network delays
func TestTCPClientNetworkDelayTimeout(t *testing.T) {
	t.Parallel()

	// Create a TCP server that simulates network delays
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			t.Logf("Warning: Failed to close listener: %v", err)
		}
	}()

	address := listener.Addr().String()

	// Server that introduces random delays
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleDelayedTCPConnection(conn, 100*time.Millisecond, 2*time.Second)
		}
	}()

	config := ClientConfig{
		Command:   address,
		Transport: "tcp",
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("NewTCPClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := client.Stop(); err != nil {
			t.Logf("Warning: Failed to stop client: %v", err)
		}
	}()

	// Test multiple requests with 1-second timeout
	timeoutCount := 0
	successCount := 0

	for i := 0; i < 10; i++ {
		requestCtx, requestCancel := context.WithTimeout(ctx, 1*time.Second)
		_, err := client.SendRequest(requestCtx, "test", map[string]interface{}{
			"request": i,
		})
		requestCancel()

		if err != nil {
			if strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "timeout") {
				timeoutCount++
			} else {
				t.Errorf("Unexpected error on request %d: %v", i, err)
			}
		} else {
			successCount++
		}
	}

	t.Logf("Network delay test: %d timeouts, %d successes out of 10 requests", timeoutCount, successCount)

	// Should have some timeouts due to network delays
	if timeoutCount == 0 {
		t.Error("Expected some timeouts due to network delays")
	}
}

// TestClientTimeoutRecovery tests that clients recover properly after timeout scenarios
func TestClientTimeoutRecovery(t *testing.T) {
	t.Parallel()

	mockServer := createDelayedMockLSPServer(t, 2*time.Second)
	defer func() {
		if err := os.Remove(mockServer); err != nil {
			t.Logf("Warning: Failed to remove mock server: %v", err)
		}
	}()

	config := ClientConfig{
		Command:   mockServer,
		Args:      []string{},
		Transport: "stdio",
	}

	client, err := NewStdioClient(config)
	if err != nil {
		t.Fatalf("NewStdioClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := client.Stop(); err != nil {
			t.Logf("Warning: Failed to stop client: %v", err)
		}
	}()

	// First request: timeout
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 1*time.Second)
	_, err = client.SendRequest(timeoutCtx, "test1", nil)
	timeoutCancel()

	if err == nil {
		t.Error("Expected timeout on first request")
	}

	// Verify client is still active
	if !client.IsActive() {
		t.Error("Client should remain active after timeout")
	}

	// Second request: should succeed with longer timeout
	successCtx, successCancel := context.WithTimeout(ctx, 5*time.Second)
	_, err = client.SendRequest(successCtx, "test2", nil)
	successCancel()

	if err != nil {
		t.Errorf("Second request should succeed, got: %v", err)
	}

	// Third request: timeout again
	timeoutCtx2, timeoutCancel2 := context.WithTimeout(ctx, 1*time.Second)
	_, err = client.SendRequest(timeoutCtx2, "test3", nil)
	timeoutCancel2()

	if err == nil {
		t.Error("Expected timeout on third request")
	}

	// Verify client remains functional
	if !client.IsActive() {
		t.Error("Client should remain active after multiple timeouts")
	}
}

// TestContextCancellationDuringTimeout tests proper handling of context cancellation
func TestContextCancellationDuringTimeout(t *testing.T) {
	t.Parallel()

	mockServer := createDelayedMockLSPServer(t, 10*time.Second)
	defer func() {
		if err := os.Remove(mockServer); err != nil {
			t.Logf("Warning: Failed to remove mock server: %v", err)
		}
	}()

	config := ClientConfig{
		Command:   mockServer,
		Args:      []string{},
		Transport: "stdio",
	}

	client, err := NewStdioClient(config)
	if err != nil {
		t.Fatalf("NewStdioClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := client.Stop(); err != nil {
			t.Logf("Warning: Failed to stop client: %v", err)
		}
	}()

	// Start a request and cancel the context after 2 seconds
	requestCtx, requestCancel := context.WithCancel(ctx)

	start := time.Now()
	go func() {
		time.Sleep(2 * time.Second)
		requestCancel()
	}()

	_, err = client.SendRequest(requestCtx, "test", nil)
	duration := time.Since(start)

	if err == nil {
		t.Error("Expected cancellation error")
	} else if !strings.Contains(err.Error(), "context canceled") && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected context cancellation error, got: %v", err)
	}

	// Verify cancellation occurred around 2 seconds
	if duration < 1500*time.Millisecond || duration > 4*time.Second {
		t.Errorf("Cancellation occurred at %v, expected around 2s", duration)
	}
}

// Helper functions for creating mock servers with controlled delays

func createDelayedMockLSPServer(t interface{}, delay time.Duration) string {
	var tmpDir string
	switch v := t.(type) {
	case *testing.T:
		tmpDir = v.TempDir()
	case *testing.B:
		tmpDir = v.TempDir()
	default:
		panic("unsupported test type")
	}
	scriptPath := fmt.Sprintf("%s/delayed_mock_lsp.sh", tmpDir)

	delaySeconds := int(delay.Seconds())
	delayNanos := int(delay.Nanoseconds() % 1000000000)

	script := fmt.Sprintf(`#!/bin/bash
# Mock LSP server with configurable delay
while IFS= read -r line; do
    if [[ "$line" =~ Content-Length:\ ([0-9]+) ]]; then
        length=${BASH_REMATCH[1]}
        read -r  # Read empty line
        read -r -N $length request
        
        # Add specified delay
        if [ %d -gt 0 ]; then
            sleep %d
        fi
        if [ %d -gt 0 ]; then
            sleep 0.%09d
        fi
        
        # Parse request and create appropriate response
        if echo "$request" | grep -q '"method"'; then
            method=$(echo "$request" | sed -n 's/.*"method"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
            id=$(echo "$request" | sed -n 's/.*"id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
            if [ -z "$id" ]; then
                id=$(echo "$request" | sed -n 's/.*"id"[[:space:]]*:[[:space:]]*\([0-9]*\).*/\1/p')
            fi
            if [ -z "$id" ]; then
                id="1"
            fi
            
            response="{\"jsonrpc\":\"2.0\",\"id\":\"$id\",\"result\":{\"message\":\"response to $method\",\"delay\":\"%ds\"}}"
        else
            response='{"jsonrpc":"2.0","id":"1","result":{"message":"default response"}}'
        fi
        
        echo "Content-Length: ${#response}"
        echo ""
        echo "$response"
    fi
done
`, delaySeconds, delaySeconds, delayNanos, delayNanos, delaySeconds)

	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		switch v := t.(type) {
		case *testing.T:
			v.Fatalf("Failed to create delayed mock LSP script: %v", err)
		case *testing.B:
			v.Fatalf("Failed to create delayed mock LSP script: %v", err)
		}
	}

	return scriptPath
}

func handleSlowTCPConnection(conn net.Conn, delay time.Duration) {
	defer func() { _ = conn.Close() }()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		// Read LSP message
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
			if _, err := io.ReadFull(reader, body); err != nil {
				return
			}

			// Parse request
			var request map[string]interface{}
			if err := json.Unmarshal(body, &request); err != nil {
				return
			}

			// Add delay before responding
			time.Sleep(delay)

			// Send response
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      request["id"],
				"result": map[string]interface{}{
					"message": "slow response",
					"delay":   delay.String(),
				},
			}

			responseData, _ := json.Marshal(response)
			responseStr := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(responseData), string(responseData))

			if _, err := writer.WriteString(responseStr); err != nil {
				return
			}
			if err := writer.Flush(); err != nil {
				return
			}
		}
	}
}

func handleDelayedTCPConnection(conn net.Conn, baseDelay, maxDelay time.Duration) {
	defer func() { _ = conn.Close() }()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		// Read LSP message
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
			if _, err := io.ReadFull(reader, body); err != nil {
				return
			}

			// Parse request
			var request map[string]interface{}
			if err := json.Unmarshal(body, &request); err != nil {
				return
			}

			// Random delay between baseDelay and maxDelay
			delayRange := maxDelay - baseDelay
			randomDelay := baseDelay + time.Duration(float64(delayRange)*0.5) // Use fixed delay for predictable testing
			time.Sleep(randomDelay)

			// Send response
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      request["id"],
				"result": map[string]interface{}{
					"message": "delayed response",
					"delay":   randomDelay.String(),
				},
			}

			responseData, _ := json.Marshal(response)
			responseStr := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(responseData), string(responseData))

			if _, err := writer.WriteString(responseStr); err != nil {
				return
			}
			if err := writer.Flush(); err != nil {
				return
			}
		}
	}
}

// TestStdioClientHangingServerTimeout tests timeout when server hangs without responding
func TestStdioClientHangingServerTimeout(t *testing.T) {
	t.Parallel()

	mockServer := createHangingMockLSPServer(t)
	defer func() {
		if err := os.Remove(mockServer); err != nil {
			t.Logf("Warning: Failed to remove mock server: %v", err)
		}
	}()

	config := ClientConfig{
		Command:   mockServer,
		Args:      []string{},
		Transport: "stdio",
	}

	client, err := NewStdioClient(config)
	if err != nil {
		t.Fatalf("NewStdioClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := client.Stop(); err != nil {
			t.Logf("Warning: Failed to stop client: %v", err)
		}
	}()

	// Test 5-second timeout on hanging server
	requestCtx, requestCancel := context.WithTimeout(ctx, 5*time.Second)
	defer requestCancel()

	start := time.Now()
	_, err = client.SendRequest(requestCtx, "test", nil)
	duration := time.Since(start)

	if err == nil {
		t.Error("Expected timeout error for hanging server")
	} else if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected context deadline exceeded, got: %v", err)
	}

	// Verify timeout occurred around 5 seconds
	if duration < 4*time.Second || duration > 7*time.Second {
		t.Errorf("Timeout occurred at %v, expected around 5s", duration)
	}
}

func createHangingMockLSPServer(t interface{}) string {
	var tmpDir string
	switch v := t.(type) {
	case *testing.T:
		tmpDir = v.TempDir()
	case *testing.B:
		tmpDir = v.TempDir()
	default:
		panic("unsupported test type")
	}
	scriptPath := fmt.Sprintf("%s/hanging_mock_lsp.sh", tmpDir)

	script := `#!/bin/bash
# Mock LSP server that hangs (never responds)
while IFS= read -r line; do
    if [[ "$line" =~ Content-Length:\ ([0-9]+) ]]; then
        length=${BASH_REMATCH[1]}
        read -r  # Read empty line
        read -r -N $length request
        
        # Read the request but never respond - simulate hanging server
        # Just hang forever
        sleep infinity
    fi
done
`

	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		switch v := t.(type) {
		case *testing.T:
			v.Fatalf("Failed to create hanging mock LSP script: %v", err)
		case *testing.B:
			v.Fatalf("Failed to create hanging mock LSP script: %v", err)
		}
	}

	return scriptPath
}

// TestStdioClientPartialResponseTimeout tests timeout during partial response reading
func TestStdioClientPartialResponseTimeout(t *testing.T) {
	t.Parallel()

	mockServer := createPartialResponseMockLSPServer(t)
	defer func() {
		if err := os.Remove(mockServer); err != nil {
			t.Logf("Warning: Failed to remove mock server: %v", err)
		}
	}()

	config := ClientConfig{
		Command:   mockServer,
		Args:      []string{},
		Transport: "stdio",
	}

	client, err := NewStdioClient(config)
	if err != nil {
		t.Fatalf("NewStdioClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := client.Stop(); err != nil {
			t.Logf("Warning: Failed to stop client: %v", err)
		}
	}()

	// Test 3-second timeout on server that sends partial response
	requestCtx, requestCancel := context.WithTimeout(ctx, 3*time.Second)
	defer requestCancel()

	start := time.Now()
	_, err = client.SendRequest(requestCtx, "test", nil)
	duration := time.Since(start)

	if err == nil {
		t.Error("Expected timeout or parse error for partial response")
	}

	// Should timeout within reasonable time
	if duration > 5*time.Second {
		t.Errorf("Operation took too long: %v", duration)
	}
}

func createPartialResponseMockLSPServer(t interface{}) string {
	var tmpDir string
	switch v := t.(type) {
	case *testing.T:
		tmpDir = v.TempDir()
	case *testing.B:
		tmpDir = v.TempDir()
	default:
		panic("unsupported test type")
	}
	scriptPath := fmt.Sprintf("%s/partial_mock_lsp.sh", tmpDir)

	script := `#!/bin/bash
# Mock LSP server that sends partial responses
while IFS= read -r line; do
    if [[ "$line" =~ Content-Length:\ ([0-9]+) ]]; then
        length=${BASH_REMATCH[1]}
        read -r  # Read empty line
        read -r -N $length request
        
        # Send a partial response (headers but incomplete body)
        echo "Content-Length: 100"
        echo ""
        echo '{"jsonrpc":"2.0","id":"1","result":{"partial":'
        # Don't complete the JSON - hang after sending partial data
        sleep infinity
    fi
done
`

	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		switch v := t.(type) {
		case *testing.T:
			v.Fatalf("Failed to create partial response mock LSP script: %v", err)
		case *testing.B:
			v.Fatalf("Failed to create partial response mock LSP script: %v", err)
		}
	}

	return scriptPath
}

// Benchmark tests for timeout performance
func BenchmarkStdioClientTimeout(b *testing.B) {
	mockServer := createDelayedMockLSPServer(b, 100*time.Millisecond)
	defer func() {
		if err := os.Remove(mockServer); err != nil {
			b.Logf("Warning: Failed to remove mock server: %v", err)
		}
	}()

	config := ClientConfig{
		Command:   mockServer,
		Args:      []string{},
		Transport: "stdio",
	}

	client, err := NewStdioClient(config)
	if err != nil {
		b.Fatalf("NewStdioClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		b.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := client.Stop(); err != nil {
			b.Logf("Warning: Failed to stop client: %v", err)
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		requestCtx, requestCancel := context.WithTimeout(ctx, 50*time.Millisecond)
		_, err := client.SendRequest(requestCtx, "test", map[string]interface{}{
			"iteration": i,
		})
		requestCancel()

		if err != nil && !strings.Contains(err.Error(), "context deadline exceeded") {
			b.Errorf("Unexpected error: %v", err)
		}
	}
}

func BenchmarkTCPClientTimeout(b *testing.B) {
	// Create a TCP server for benchmarking
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		b.Fatalf("Failed to create listener: %v", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			b.Logf("Warning: Failed to close listener: %v", err)
		}
	}()

	address := listener.Addr().String()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleSlowTCPConnection(conn, 100*time.Millisecond)
		}
	}()

	config := ClientConfig{
		Command:   address,
		Transport: "tcp",
	}

	client, err := NewTCPClient(config)
	if err != nil {
		b.Fatalf("NewTCPClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		b.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := client.Stop(); err != nil {
			b.Logf("Warning: Failed to stop client: %v", err)
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		requestCtx, requestCancel := context.WithTimeout(ctx, 50*time.Millisecond)
		_, err := client.SendRequest(requestCtx, "test", map[string]interface{}{
			"iteration": i,
		})
		requestCancel()

		if err != nil && !strings.Contains(err.Error(), "context deadline exceeded") && !strings.Contains(err.Error(), "timeout") {
			b.Errorf("Unexpected error: %v", err)
		}
	}
}
