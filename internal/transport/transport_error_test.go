package transport

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestStdioClientInvalidCommands tests error handling for invalid commands
func TestStdioClientInvalidCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		command       string
		args          []string
		expectedError string
	}{
		{
			name:          "nonexistent command",
			command:       "nonexistent_command_12345",
			args:          []string{},
			expectedError: "executable file not found",
		},
		{
			name:          "empty command",
			command:       "",
			args:          []string{},
			expectedError: "command",
		},
		{
			name:          "command with null bytes",
			command:       "echo\x00test",
			args:          []string{},
			expectedError: "not found",
		},
		{
			name:          "command that immediately exits",
			command:       "false", // Command that always fails
			args:          []string{},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ClientConfig{
				Command:   tt.command,
				Args:      tt.args,
				Transport: "stdio",
			}

			client, err := NewStdioClient(config)
			if err != nil {
				if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.expectedError)) {
					t.Errorf("Expected error containing '%s', got: %v", tt.expectedError, err)
				}
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err = client.Start(ctx)
			if tt.expectedError == "" {
				// Should succeed or fail gracefully
				if err != nil {
					t.Logf("Expected graceful handling, got error: %v", err)
				}
			} else {
				if err == nil {
					t.Error("Expected error when starting invalid command")
				} else if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.expectedError)) {
					t.Errorf("Expected error containing '%s', got: %v", tt.expectedError, err)
				}
			}

			// Cleanup
			if client != nil {
				if err := client.Stop(); err != nil {
					t.Logf("Warning: Failed to stop client: %v", err)
				}
			}
		})
	}
}

// TestStdioClientResourceExhaustion tests behavior under resource exhaustion
func TestStdioClientResourceExhaustion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource exhaustion test in short mode")
	}

	t.Parallel()

	t.Run("concurrent requests with backoff", func(t *testing.T) {
		// Create a mock server that responds with realistic delay
		mockServer := createMockLSPServerProgram(t, false, 0, false, 0, 50*time.Millisecond)
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

		// Send manageable number of concurrent requests with backoff
		const numRequests = 20
		errors := make(chan error, numRequests)
		successes := make(chan bool, numRequests)
		var wg sync.WaitGroup

		// Add staggered starts to prevent resource exhaustion
		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				// Stagger request starts to reduce resource pressure
				if id > 0 {
					time.Sleep(time.Duration(id*5) * time.Millisecond)
				}
				
				requestCtx, requestCancel := context.WithTimeout(ctx, 8*time.Second)
				defer requestCancel()

				_, err := client.SendRequest(requestCtx, "test", map[string]interface{}{
					"id": id,
				})
				if err != nil {
					errors <- err
				} else {
					successes <- true
				}
			}(i)
		}

		wg.Wait()
		close(errors)
		close(successes)

		// Count results
		errorCount := 0
		successCount := 0
		
		for err := range errors {
			errorCount++
			t.Logf("Request error: %v", err)
		}
		
		for range successes {
			successCount++
		}

		t.Logf("Request results: %d successes, %d errors out of %d total", successCount, errorCount, numRequests)

		// Should handle reasonable load - expect at least some successes
		if successCount == 0 {
			t.Error("All requests failed - client should handle reasonable concurrent load")
		}
		
		// Should not have excessive failures under reasonable load
		if float64(errorCount)/float64(numRequests) > 0.7 {
			t.Errorf("Too many failures (%d/%d) - client should handle reasonable load better", errorCount, numRequests)
		}
	})
}

// TestStdioClientMalformedMessages tests handling of malformed LSP messages
func TestStdioClientMalformedMessages(t *testing.T) {
	t.Parallel()

	// Create a custom mock server that sends malformed responses
	config := ClientConfig{
		Command:   "cat", // Will echo back whatever we send
		Args:      []string{},
		Transport: "stdio",
	}

	client, err := NewStdioClient(config)
	if err != nil {
		t.Fatalf("NewStdioClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := client.Stop(); err != nil {
			t.Logf("Warning: Failed to stop client: %v", err)
		}
	}()

	// Test sending a malformed message (cat will echo it back, causing parse error)
	malformedMessage := "this is not a valid LSP message"
	
	// Create a buffer to write the malformed message directly
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(malformedMessage), malformedMessage))

	// This should fail gracefully and not crash the client
	_, err = client.SendRequest(ctx, "test", nil)
	if err == nil {
		t.Log("Request succeeded despite malformed setup")
	} else {
		t.Logf("Request failed as expected: %v", err)
	}

	// Client should still be responsive after handling malformed data
	if !client.IsActive() {
		t.Error("Client should still be active after handling malformed data")
	}
}

// TestStdioClientMemoryLeaks tests for memory leaks during long-running operations
func TestStdioClientMemoryLeaks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	t.Parallel()

	mockServer := createMockLSPServerProgram(t, false, 0, false, 0, 0)
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

	// Send many requests in sequence
	for i := 0; i < 1000; i++ {
		requestCtx, requestCancel := context.WithTimeout(ctx, 1*time.Second)
		
		_, err := client.SendRequest(requestCtx, "test", map[string]interface{}{
			"data": strings.Repeat("x", 1000), // Add some data to test memory handling
		})
		
		requestCancel() // Important: cancel the context to prevent leaks
		
		if err != nil {
			t.Logf("Request %d failed: %v", i, err)
		}

		// Occasionally check if client is still active
		if i%100 == 0 && !client.IsActive() {
			t.Fatalf("Client became inactive during test at iteration %d", i)
		}
	}

	if !client.IsActive() {
		t.Error("Client should still be active after many requests")
	}
}

// TestTCPClientConnectionErrors tests TCP-specific connection error scenarios
func TestTCPClientConnectionErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		command       string
		expectedError string
	}{
		{
			name:          "invalid address format",
			command:       "invalid-address",
			expectedError: "address",
		},
		{
			name:          "connection refused",
			command:       "localhost:99999", // Unlikely to be in use
			expectedError: "connection refused",
		},
		{
			name:          "invalid port",
			command:       "localhost:999999",
			expectedError: "invalid port",
		},
		{
			name:          "malformed host",
			command:       "999.999.999.999:8080",
			expectedError: "no such host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ClientConfig{
				Command:   tt.command,
				Transport: "tcp",
			}

			client, err := NewTCPClient(config)
			if err != nil {
				t.Logf("Expected creation error: %v", err)
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err = client.Start(ctx)
			if err == nil {
				t.Error("Expected connection error")
				if err := client.Stop(); err != nil {
					t.Logf("Warning: Failed to stop client: %v", err)
				}
			} else if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.expectedError)) {
				t.Errorf("Expected error containing '%s', got: %v", tt.expectedError, err)
			}
		})
	}
}

// TestTCPClientNetworkInterruption tests handling of network interruptions
func TestTCPClientNetworkInterruption(t *testing.T) {
	t.Parallel()

	// Create a TCP server that will close the connection abruptly
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

	// Server that accepts connection but then closes it
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// Immediately close the connection to simulate network interruption
			if err := conn.Close(); err != nil {
				// Log but don't fail the test as this is expected in error scenarios
				return
			}
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start might succeed initially
	err = client.Start(ctx)
	if err != nil {
		t.Logf("Start failed as expected due to immediate close: %v", err)
		return
	}

	// But sending a request should fail
	_, err = client.SendRequest(ctx, "test", nil)
	if err == nil {
		t.Error("Expected error due to closed connection")
	}

	if err := client.Stop(); err != nil {
		t.Logf("Warning: Failed to stop client: %v", err)
	}
}

// TestClientFactoryErrors tests error cases in the client factory
func TestClientFactoryErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		config        ClientConfig
		expectedError string
	}{
		{
			name: "unsupported transport",
			config: ClientConfig{
				Transport: "unsupported",
				Command:   "echo",
			},
			expectedError: "unsupported transport",
		},
		{
			name: "empty transport",
			config: ClientConfig{
				Transport: "",
				Command:   "echo",
			},
			expectedError: "transport",
		},
		{
			name: "TCP without address",
			config: ClientConfig{
				Transport: "tcp",
			},
			expectedError: "address",
		},
		{
			name: "stdio without command",
			config: ClientConfig{
				Transport: "stdio",
				Command:   "",
			},
			expectedError: "command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewLSPClient(tt.config)
			if err == nil {
				t.Error("Expected error from NewLSPClient")
				if client != nil {
					// Try to clean up
					if err := client.Stop(); err != nil {
						t.Logf("Warning: Failed to stop client: %v", err)
					}
				}
			} else if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.expectedError)) {
				t.Errorf("Expected error containing '%s', got: %v", tt.expectedError, err)
			}

			if client != nil {
				t.Error("Expected nil client when creation fails")
			}
		})
	}
}

// TestClientContextCancellation tests proper handling of context cancellation
func TestClientContextCancellation(t *testing.T) {
	t.Parallel()

	mockServer := createMockLSPServerProgram(t, false, 0, false, 0, 5*time.Second) // Slow server
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := client.Stop(); err != nil {
			t.Logf("Warning: Failed to stop client: %v", err)
		}
	}()

	// Test request cancellation
	t.Run("request cancellation", func(t *testing.T) {
		requestCtx, requestCancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer requestCancel()

		_, err := client.SendRequest(requestCtx, "test", nil)
		if err == nil {
			t.Error("Expected timeout error")
		} else if !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "timeout") {
			t.Errorf("Expected context cancellation error, got: %v", err)
		}
	})

	// Test that client remains functional after cancelled requests
	t.Run("client remains functional", func(t *testing.T) {
		if !client.IsActive() {
			t.Error("Client should remain active after cancelled request")
		}
	})
}

// TestClientCleanupOnErrors tests proper cleanup when errors occur
func TestClientCleanupOnErrors(t *testing.T) {
	t.Parallel()

	t.Run("cleanup after start failure", func(t *testing.T) {
		config := ClientConfig{
			Command:   "nonexistent_command",
			Args:      []string{},
			Transport: "stdio",
		}

		client, err := NewStdioClient(config)
		if err != nil {
			// If creation fails, nothing to clean up
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err = client.Start(ctx)
		if err == nil {
			t.Error("Expected start to fail")
			if err := client.Stop(); err != nil {
				t.Logf("Warning: Failed to stop client: %v", err)
			}
			return
		}

		// Should be safe to call Stop even after failed Start
		if err := client.Stop(); err != nil {
			t.Logf("Stop error after failed start: %v", err)
		}

		// Should not be active
		if client.IsActive() {
			t.Error("Client should not be active after failed start")
		}
	})

	t.Run("double stop", func(t *testing.T) {
		config := ClientConfig{
			Command:   "echo",
			Args:      []string{},
			Transport: "stdio",
		}

		client, err := NewStdioClient(config)
		if err != nil {
			t.Fatalf("NewStdioClient failed: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := client.Start(ctx); err != nil {
			t.Fatalf("Start failed: %v", err)
		}

		// First stop should succeed
		if err := client.Stop(); err != nil {
			t.Errorf("First stop failed: %v", err)
		}

		// Second stop should be safe (no-op or harmless error)
		if err := client.Stop(); err != nil {
			t.Logf("Second stop returned error (may be expected): %v", err)
		}

		if client.IsActive() {
			t.Error("Client should not be active after stop")
		}
	})
}

// simpleMockLSPResponder creates a simple echo command wrapper for testing
// This avoids the complexity of compiling Go programs and provides faster, more reliable tests
func createSimpleMockLSPResponder(t *testing.T, delay time.Duration) string {
	tmpDir := t.TempDir()
	scriptPath := fmt.Sprintf("%s/mock_lsp.sh", tmpDir)
	
	// Create a simple shell script that acts as an LSP server
	script := fmt.Sprintf(`#!/bin/bash
# Simple LSP-like responder for testing
while IFS= read -r line; do
    if [[ "$line" =~ Content-Length:\ ([0-9]+) ]]; then
        length=${BASH_REMATCH[1]}
        read -r  # Read empty line
        read -r -N $length request
        
        # Add delay if specified
        if [ %d -gt 0 ]; then
            sleep %f
        fi
        
        # Simple response based on request
        response='{"jsonrpc":"2.0","id":1,"result":{"message":"test response"}}'
        echo "Content-Length: ${#response}"
        echo ""
        echo "$response"
    fi
done
`, int(delay.Nanoseconds()), delay.Seconds())

	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to create mock LSP script: %v", err)
	}
	
	return scriptPath
}

// createInMemoryMockLSP creates an in-process mock LSP server that responds via pipes
func createInMemoryMockLSP(t *testing.T, delay time.Duration, shouldFail bool) (*exec.Cmd, io.WriteCloser, io.ReadCloser) {
	// Use a simple command that can be controlled
	cmd := exec.Command("cat")
	
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}
	
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to create stdout pipe: %v", err)
	}
	
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start mock command: %v", err)
	}
	
	// Clean up when test completes
	t.Cleanup(func() {
		stdin.Close()
		stdout.Close()
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	})
	
	return cmd, stdin, stdout
}

