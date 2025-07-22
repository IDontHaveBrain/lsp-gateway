package transport

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestStdioClientServerUnavailable tests behavior when LSP server executables are not available
func TestStdioClientServerUnavailable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		command        string
		args           []string
		expectedError  string
		shouldFailFast bool
	}{
		{
			name:           "nonexistent executable",
			command:        "definitely_does_not_exist_12345",
			args:           []string{},
			expectedError:  "executable file not found",
			shouldFailFast: true,
		},
		{
			name:           "invalid executable path",
			command:        "/invalid/path/to/executable",
			args:           []string{},
			expectedError:  "no such file",
			shouldFailFast: true,
		},
		{
			name:           "permission denied executable",
			command:        createNonExecutableFile(t),
			args:           []string{},
			expectedError:  "permission denied",
			shouldFailFast: true,
		},
		{
			name:           "directory instead of executable",
			command:        t.TempDir(),
			args:           []string{},
			expectedError:  "is a directory",
			shouldFailFast: true,
		},
		{
			name:           "empty command",
			command:        "",
			args:           []string{},
			expectedError:  "command",
			shouldFailFast: true,
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
					t.Errorf("Expected creation error containing '%s', got: %v", tt.expectedError, err)
				}
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			startTime := time.Now()
			err = client.Start(ctx)
			elapsed := time.Since(startTime)

			if err == nil {
				t.Error("Expected error when starting unavailable server")
				if err := client.Stop(); err != nil {
					t.Logf("Warning: Failed to stop client: %v", err)
				}
				return
			}

			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.expectedError)) {
				t.Errorf("Expected error containing '%s', got: %v", tt.expectedError, err)
			}

			// Should fail fast for certain error types
			if tt.shouldFailFast && elapsed > 500*time.Millisecond {
				t.Errorf("Expected fast failure, but took %v", elapsed)
			}

			// Client should not be active after failure
			if client.IsActive() {
				t.Error("Client should not be active after failed start")
			}

			// Should be safe to call Stop even after failed Start
			if err := client.Stop(); err != nil {
				t.Logf("Stop error after failed start: %v", err)
			}
		})
	}
}

// TestStdioClientServerCrashScenarios tests behavior when LSP server processes crash
func TestStdioClientServerCrashScenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		crashType   string
		expectedErr string
		crashAfter  time.Duration
	}{
		{
			name:        "immediate exit",
			crashType:   "exit_immediate",
			expectedErr: "exit",
			crashAfter:  0,
		},
		{
			name:        "exit after delay",
			crashType:   "exit_delayed",
			expectedErr: "exit",
			crashAfter:  100 * time.Millisecond,
		},
		{
			name:        "segmentation fault",
			crashType:   "kill_self",
			expectedErr: "signal",
			crashAfter:  50 * time.Millisecond,
		},
		{
			name:        "invalid command line",
			crashType:   "invalid_args",
			expectedErr: "exit",
			crashAfter:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockServer := createCrashingMockLSPServer(t, tt.crashType, tt.crashAfter)
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

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Start may fail immediately or succeed initially
			err = client.Start(ctx)
			if err != nil {
				if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.expectedErr)) {
					t.Errorf("Expected error containing '%s', got: %v", tt.expectedErr, err)
				}
				return
			}

			// If start succeeded, client should become inactive after crash
			time.Sleep(tt.crashAfter + 200*time.Millisecond)

			// Try to send a request - should fail due to crashed process
			_, err = client.SendRequest(ctx, "test", nil)
			if err == nil {
				t.Error("Expected error when sending request to crashed server")
			}

			// Client should eventually become inactive
			maxWait := 2 * time.Second
			checkInterval := 50 * time.Millisecond
			inactive := false

			for elapsed := time.Duration(0); elapsed < maxWait; elapsed += checkInterval {
				if !client.IsActive() {
					inactive = true
					break
				}
				time.Sleep(checkInterval)
			}

			if !inactive {
				t.Error("Client should become inactive after server crash")
			}

			if err := client.Stop(); err != nil {
				t.Logf("Warning: Failed to stop client: %v", err)
			}
		})
	}
}

// TestStdioClientReconnectionAttempts tests reconnection behavior and backoff
func TestStdioClientReconnectionAttempts(t *testing.T) {
	t.Parallel()

	t.Run("no reconnection on initial failure", func(t *testing.T) {
		config := ClientConfig{
			Command:   "nonexistent_command_reconnect_test",
			Args:      []string{},
			Transport: "stdio",
		}

		client, err := NewStdioClient(config)
		if err != nil {
			// If creation fails, that's expected
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		startTime := time.Now()
		err = client.Start(ctx)
		elapsed := time.Since(startTime)

		if err == nil {
			t.Error("Expected error for nonexistent command")
			if err := client.Stop(); err != nil {
				t.Logf("Warning: Failed to stop client: %v", err)
			}
			return
		}

		// Should fail quickly without retry attempts
		if elapsed > 500*time.Millisecond {
			t.Errorf("Expected fast failure without retries, took %v", elapsed)
		}
	})
}

// TestTCPClientConnectionRefused tests TCP-specific connection refused scenarios
func TestTCPClientConnectionRefused(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		address       string
		expectedError string
		shouldTimeout bool
	}{
		{
			name:          "localhost connection refused",
			address:       "localhost:65533", // Valid high port unlikely to be in use
			expectedError: "connection refused",
			shouldTimeout: false,
		},
		{
			name:          "127.0.0.1 connection refused",
			address:       "127.0.0.1:65532",
			expectedError: "connection refused",
			shouldTimeout: false,
		},
		{
			name:          "explicit port connection refused",
			address:       fmt.Sprintf("localhost:%d", allocateUnusedPort(t)),
			expectedError: "connection refused",
			shouldTimeout: false,
		},
		{
			name:          "unreachable host timeout",
			address:       "192.0.2.1:80", // RFC 5737 test address - should be unreachable
			expectedError: "timeout",
			shouldTimeout: true,
		},
		{
			name:          "invalid host no such host",
			address:       "definitely.does.not.exist.example:8080",
			expectedError: "no such host",
			shouldTimeout: false,
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

			timeout := 3 * time.Second
			if tt.shouldTimeout {
				timeout = 15 * time.Second // Longer timeout for network unreachable
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			startTime := time.Now()
			err = client.Start(ctx)
			elapsed := time.Since(startTime)

			if err == nil {
				t.Error("Expected connection error")
				if err := client.Stop(); err != nil {
					t.Logf("Warning: Failed to stop client: %v", err)
				}
				return
			}

			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.expectedError)) {
				t.Errorf("Expected error containing '%s', got: %v", tt.expectedError, err)
			}

			// Connection refused should fail relatively quickly
			if !tt.shouldTimeout && elapsed > 12*time.Second {
				t.Errorf("Connection refused took too long: %v", elapsed)
			}

			// Client should not be active after connection failure
			if client.IsActive() {
				t.Error("Client should not be active after connection failure")
			}

			// Should be safe to call Stop after failed Start
			if err := client.Stop(); err != nil {
				t.Logf("Stop error after failed start: %v", err)
			}
		})
	}
}

// TestTCPClientNetworkUnreachable tests network unreachable scenarios
func TestTCPClientNetworkUnreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network unreachable test in short mode")
	}

	t.Parallel()

	tests := []struct {
		name          string
		address       string
		expectedError string
		maxDuration   time.Duration
	}{
		{
			name:          "blackhole address",
			address:       "198.51.100.1:80", // RFC 5737 test address
			expectedError: "timeout",
			maxDuration:   15 * time.Second,
		},
		{
			name:          "private network unreachable",
			address:       "172.16.0.1:22",
			expectedError: "unreachable",
			maxDuration:   10 * time.Second,
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

			ctx, cancel := context.WithTimeout(context.Background(), tt.maxDuration)
			defer cancel()

			err = client.Start(ctx)
			if err == nil {
				t.Error("Expected network unreachable error")
				if err := client.Stop(); err != nil {
					t.Logf("Warning: Failed to stop client: %v", err)
				}
				return
			}

			// Should contain timeout or unreachable in error
			errStr := strings.ToLower(err.Error())
			if !strings.Contains(errStr, "timeout") && !strings.Contains(errStr, "unreachable") &&
				!strings.Contains(errStr, "context deadline exceeded") {
				t.Errorf("Expected timeout/unreachable error, got: %v", err)
			}

			if client.IsActive() {
				t.Error("Client should not be active after network error")
			}
		})
	}
}

// TestTCPClientConnectionReset tests connection reset scenarios
func TestTCPClientConnectionReset(t *testing.T) {
	t.Parallel()

	t.Run("immediate connection reset", func(t *testing.T) {
		// Create a server that accepts and immediately closes connections
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

		// Server that immediately closes connections
		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				// Immediately close to simulate connection reset
				if err := conn.Close(); err != nil {
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

		// Start might succeed initially or fail
		err = client.Start(ctx)
		if err != nil {
			t.Logf("Start failed as expected due to immediate close: %v", err)
			return
		}

		// Try to send a request - should fail
		_, err = client.SendRequest(ctx, "test", nil)
		if err == nil {
			t.Error("Expected error due to connection reset")
		}

		if err := client.Stop(); err != nil {
			t.Logf("Warning: Failed to stop client: %v", err)
		}
	})

	t.Run("connection reset during communication", func(t *testing.T) {
		// Create a server that accepts, then closes after receiving data
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

		// Server that closes after receiving some data
		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				}

				go func(c net.Conn) {
					defer func() { _ = c.Close() }()

					// Read some data then close
					buffer := make([]byte, 1024)
					if _, err := c.Read(buffer); err == nil {
						// Close connection after reading
						return
					}
				}(conn)
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

		if err := client.Start(ctx); err != nil {
			t.Fatalf("Start failed: %v", err)
		}

		// Send a request - connection should be reset
		_, err = client.SendRequest(ctx, "test", map[string]string{"key": "value"})
		if err == nil {
			t.Error("Expected error due to connection reset during communication")
		}

		if err := client.Stop(); err != nil {
			t.Logf("Warning: Failed to stop client: %v", err)
		}
	})
}

// TestTCPClientPortBindingFailures tests port binding and address-in-use scenarios
func TestTCPClientPortBindingFailures(t *testing.T) {
	t.Parallel()

	t.Run("address already in use", func(t *testing.T) {
		// Create a server to occupy a port
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

		// Try to connect to the occupied port (should succeed for client)
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

		// This should actually succeed since we're connecting to an existing listener
		err = client.Start(ctx)
		if err != nil {
			// Connection might fail if the server doesn't speak LSP protocol
			t.Logf("Expected connection failure to non-LSP server: %v", err)
		} else {
			if err := client.Stop(); err != nil {
				t.Logf("Warning: Failed to stop client: %v", err)
			}
		}
	})
}

// TestTCPClientIPv4IPv6Scenarios tests IPv4/IPv6 connection differences
func TestTCPClientIPv4IPv6Scenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		address       string
		expectedError string
	}{
		{
			name:          "IPv4 loopback connection refused",
			address:       "127.0.0.1:65530",
			expectedError: "connection refused",
		},
		{
			name:          "IPv6 loopback connection refused",
			address:       "[::1]:65529",
			expectedError: "connection refused",
		},
		{
			name:          "IPv4 zero address invalid",
			address:       "0.0.0.0:80",
			expectedError: "connection refused",
		},
		{
			name:          "IPv6 invalid address format",
			address:       "::1:80", // Missing brackets
			expectedError: "address",
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
				if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.expectedError)) {
					t.Errorf("Expected creation error containing '%s', got: %v", tt.expectedError, err)
				}
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			err = client.Start(ctx)
			if err == nil {
				t.Error("Expected connection error")
				if err := client.Stop(); err != nil {
					t.Logf("Warning: Failed to stop client: %v", err)
				}
				return
			}

			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.expectedError)) {
				t.Errorf("Expected error containing '%s', got: %v", tt.expectedError, err)
			}
		})
	}
}

// TestLSPClientFactoryConnectionErrors tests error cases in the client factory with connection issues
func TestLSPClientFactoryConnectionErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		config        ClientConfig
		expectedError string
	}{
		{
			name: "TCP with invalid address format",
			config: ClientConfig{
				Transport: "tcp",
				Command:   "invalid_address_format",
			},
			expectedError: "address",
		},
		{
			name: "TCP with port out of range",
			config: ClientConfig{
				Transport: "tcp",
				Command:   "localhost:99999",
			},
			expectedError: "invalid port",
		},
		{
			name: "stdio with invalid executable",
			config: ClientConfig{
				Transport: "stdio",
				Command:   "/dev/null", // Not executable
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewLSPClient(tt.config)
			if err != nil && tt.expectedError != "" {
				if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.expectedError)) {
					t.Errorf("Expected error containing '%s', got: %v", tt.expectedError, err)
				}
				return
			}

			if client == nil {
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err = client.Start(ctx)
			if err == nil && tt.expectedError != "" {
				t.Error("Expected start error")
				if err := client.Stop(); err != nil {
					t.Logf("Warning: Failed to stop client: %v", err)
				}
			} else if err != nil && tt.expectedError == "" {
				t.Logf("Got expected start error: %v", err)
			}

			if client != nil {
				if err := client.Stop(); err != nil {
					t.Logf("Warning: Failed to stop client: %v", err)
				}
			}
		})
	}
}

// TestConcurrentConnectionFailures tests concurrent connection failure scenarios
func TestConcurrentConnectionFailures(t *testing.T) {
	t.Parallel()

	const numClients = 10

	t.Run("concurrent connection refused", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make(chan error, numClients)

		for i := 0; i < numClients; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				config := ClientConfig{
					Command:   fmt.Sprintf("localhost:%d", 65520-id), // Valid high ports unlikely to be in use
					Transport: "tcp",
				}

				client, err := NewTCPClient(config)
				if err != nil {
					errors <- err
					return
				}

				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()

				err = client.Start(ctx)
				if err != nil {
					errors <- err
				}

				if err := client.Stop(); err != nil {
					// Log but don't fail test for cleanup errors
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		errorCount := 0
		for err := range errors {
			errorCount++
			if !strings.Contains(strings.ToLower(err.Error()), "connection refused") {
				t.Errorf("Expected connection refused error, got: %v", err)
			}
		}

		if errorCount != numClients {
			t.Errorf("Expected %d connection errors, got %d", numClients, errorCount)
		}
	})
}

// Helper functions

func createNonExecutableFile(t *testing.T) string {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "non_executable")

	if err := os.WriteFile(filePath, []byte("#!/bin/bash\necho test"), 0644); err != nil {
		t.Fatalf("Failed to create non-executable file: %v", err)
	}

	return filePath
}

func createCrashingMockLSPServer(t *testing.T, crashType string, crashAfter time.Duration) string {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "crashing_mock_lsp.sh")

	var script string
	switch crashType {
	case "exit_immediate":
		script = "#!/bin/bash\nexit 1\n"
	case "exit_delayed":
		script = fmt.Sprintf("#!/bin/bash\nsleep %f\nexit 1\n", crashAfter.Seconds())
	case "kill_self":
		script = fmt.Sprintf("#!/bin/bash\nsleep %f\nkill -9 $$\n", crashAfter.Seconds())
	case "invalid_args":
		script = "#!/bin/bash\necho 'Invalid arguments' >&2\nexit 2\n"
	default:
		script = "#!/bin/bash\nexit 1\n"
	}

	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to create crashing mock LSP script: %v", err)
	}

	return scriptPath
}

func allocateUnusedPort(t *testing.T) int {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to allocate unused port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Logf("Warning: Failed to close port allocator: %v", err)
	}
	return port
}
