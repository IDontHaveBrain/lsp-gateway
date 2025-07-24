package transport_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"lsp-gateway/internal/transport"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	helpers "lsp-gateway/tests/utils/helpers"
)

type SimpleTCPEchoServer struct {
	listener net.Listener
	port     int
	messages chan []byte
	closed   chan bool
}

func NewSimpleTCPEchoServer() (*SimpleTCPEchoServer, error) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, err
	}

	port := listener.Addr().(*net.TCPAddr).Port

	return &SimpleTCPEchoServer{
		listener: listener,
		port:     port,
		messages: make(chan []byte, 10),
		closed:   make(chan bool, 1),
	}, nil
}

func (s *SimpleTCPEchoServer) Start() {
	go s.acceptConnections()
}

func (s *SimpleTCPEchoServer) Stop() error {
	close(s.closed)
	return s.listener.Close()
}

func (s *SimpleTCPEchoServer) Port() int {
	return s.port
}

func (s *SimpleTCPEchoServer) GetMessages() [][]byte {
	var messages [][]byte
	for {
		select {
		case msg := <-s.messages:
			messages = append(messages, msg)
		default:
			return messages
		}
	}
}

func (s *SimpleTCPEchoServer) acceptConnections() {
	for {
		select {
		case <-s.closed:
			return
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			return
		}

		go s.handleConnection(conn)
	}
}

func (s *SimpleTCPEchoServer) handleConnection(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	reader := bufio.NewReader(conn)

	contentLength := 0
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
			select {
			case s.messages <- body:
			default:
			}
		}
	}
}

func TestTCPClientSendMessage(t *testing.T) {
	server, err := NewSimpleTCPEchoServer()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer func() { _ = server.Stop() }()

	server.Start()

	config := transport.ClientConfig{
		Command:   fmt.Sprintf("localhost:%d", server.Port()),
		Transport: "tcp",
	}

	client, err := transport.NewTCPClient(config)
	if err != nil {
		t.Fatalf("NewTCPClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = client.Stop() }()

	tcpClient := client.(*transport.TCPClient)

	// Test that the client can send messages through the public interface
	reqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use SendRequest which internally calls sendMessage
	_, reqErr := tcpClient.SendRequest(reqCtx, "test", map[string]interface{}{"test": "value"})
	// Note: This will likely fail since the server is just an echo server, but it tests sendMessage functionality
	if reqErr == nil {
		t.Log("SendRequest succeeded (unexpected but acceptable for this test)")
	} else {
		t.Logf("SendRequest failed as expected: %v", reqErr)
	}

	time.Sleep(100 * time.Millisecond)

	messages := server.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	var receivedMessage transport.JSONRPCMessage
	if err := json.Unmarshal(messages[0], &receivedMessage); err != nil {
		t.Fatalf("Failed to unmarshal received message: %v", err)
	}

	if receivedMessage.Method != "test" {
		t.Errorf("Expected method 'test', got %s", receivedMessage.Method)
	}

	if receivedMessage.ID != "1" {
		t.Errorf("Expected ID '1', got %v", receivedMessage.ID)
	}
}

func TestTCPClientBasicOperation(t *testing.T) {
	// Test basic TCP client creation and configuration
	testPort := helpers.AllocateTestPort(t)
	config := transport.ClientConfig{
		Command:   fmt.Sprintf("localhost:%d", testPort),
		Transport: "tcp",
	}

	client, err := transport.NewTCPClient(config)
	if err != nil {
		t.Fatalf("NewTCPClient failed: %v", err)
	}

	// Test that client is not active initially
	if client.IsActive() {
		t.Error("Client should not be active initially")
	}

	// Test client type assertion
	if _, ok := client.(*transport.TCPClient); !ok {
		t.Error("Client should be of type *TCPClient")
	}

	// Note: We don't test handleMessage directly since it's an unexported method
	// Instead, we rely on integration tests to verify message handling works correctly
	t.Log("Basic TCP client operations test completed")
}

func TestTCPClientSendNotificationBasic(t *testing.T) {
	server, err := NewSimpleTCPEchoServer()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer func() { _ = server.Stop() }()

	server.Start()

	config := transport.ClientConfig{
		Command:   fmt.Sprintf("localhost:%d", server.Port()),
		Transport: "tcp",
	}

	client, err := transport.NewTCPClient(config)
	if err != nil {
		t.Fatalf("NewTCPClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = client.Stop() }()

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":     "file:///test.go",
			"version": 1,
		},
	}

	err = client.SendNotification(ctx, "textDocument/didOpen", params)
	if err != nil {
		t.Fatalf("SendNotification failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	messages := server.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("Expected 1 notification, got %d", len(messages))
	}

	var notification transport.JSONRPCMessage
	if err := json.Unmarshal(messages[0], &notification); err != nil {
		t.Fatalf("Failed to unmarshal notification: %v", err)
	}

	if notification.Method != "textDocument/didOpen" {
		t.Errorf("Expected method 'textDocument/didOpen', got %s", notification.Method)
	}

	if notification.ID != nil {
		t.Error("Notification should not have an ID")
	}
}

func TestTCPClientMessageHandling(t *testing.T) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	port := listener.Addr().(*net.TCPAddr).Port

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		time.Sleep(200 * time.Millisecond)
	}()

	config := transport.ClientConfig{
		Command:   fmt.Sprintf("localhost:%d", port),
		Transport: "tcp",
	}

	client, err := transport.NewTCPClient(config)
	if err != nil {
		t.Fatalf("NewTCPClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if !client.IsActive() {
		t.Error("Client should be active with good connection")
	}

	_ = client.Stop()
}

// TestTCPClientAdvancedConnectionRefused tests advanced connection refused scenarios
func TestTCPClientAdvancedConnectionRefused(t *testing.T) {
	t.Parallel()

	t.Run("connection refused with retry behavior", func(t *testing.T) {
		// Test that client doesn't retry on connection refused
		config := transport.ClientConfig{
			Command:   "localhost:65527", // Valid high port unlikely to be in use
			Transport: "tcp",
		}

		client, err := transport.NewTCPClient(config)
		if err != nil {
			t.Fatalf("NewTCPClient failed: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		startTime := time.Now()
		err = client.Start(ctx)
		elapsed := time.Since(startTime)

		if err == nil {
			t.Error("Expected connection refused error")
			if err := client.Stop(); err != nil {
				t.Logf("Warning: Failed to stop client: %v", err)
			}
			return
		}

		if !strings.Contains(strings.ToLower(err.Error()), "connection refused") {
			t.Errorf("Expected connection refused error, got: %v", err)
		}

		// Should fail quickly without retries for connection refused
		if elapsed > 1*time.Second {
			t.Errorf("Connection refused took too long (possible retries): %v", elapsed)
		}
	})

	t.Run("dns resolution failure", func(t *testing.T) {
		config := transport.ClientConfig{
			Command:   "this-domain-definitely-does-not-exist.invalid:8080",
			Transport: "tcp",
		}

		client, err := transport.NewTCPClient(config)
		if err != nil {
			t.Fatalf("NewTCPClient failed: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = client.Start(ctx)
		if err == nil {
			t.Error("Expected DNS resolution error")
			if err := client.Stop(); err != nil {
				t.Logf("Warning: Failed to stop client: %v", err)
			}
			return
		}

		errStr := strings.ToLower(err.Error())
		if !strings.Contains(errStr, "no such host") && !strings.Contains(errStr, "name resolution") {
			t.Errorf("Expected DNS resolution error, got: %v", err)
		}
	})

	t.Run("connection timeout vs refused distinction", func(t *testing.T) {
		tests := []struct {
			name        string
			address     string
			expectError string
			maxTime     time.Duration
		}{
			{
				name:        "connection refused",
				address:     "localhost:65526",
				expectError: "connection refused",
				maxTime:     2 * time.Second,
			},
			{
				name:        "connection timeout",
				address:     "198.51.100.1:80", // RFC 5737 test address
				expectError: "timeout",
				maxTime:     15 * time.Second,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := transport.ClientConfig{
					Command:   tt.address,
					Transport: "tcp",
				}

				client, err := transport.NewTCPClient(config)
				if err != nil {
					t.Fatalf("NewTCPClient failed: %v", err)
				}

				ctx, cancel := context.WithTimeout(context.Background(), tt.maxTime)
				defer cancel()

				err = client.Start(ctx)
				if err == nil {
					t.Errorf("Expected %s error", tt.expectError)
					if err := client.Stop(); err != nil {
						t.Logf("Warning: Failed to stop client: %v", err)
					}
					return
				}

				errStr := strings.ToLower(err.Error())
				if !strings.Contains(errStr, tt.expectError) &&
					!(tt.expectError == "timeout" && strings.Contains(errStr, "context deadline exceeded")) {
					t.Errorf("Expected error containing '%s', got: %v", tt.expectError, err)
				}
			})
		}
	})
}

// TestTCPClientPortExhaustionScenarios tests behavior under port exhaustion
func TestTCPClientPortExhaustionScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping port exhaustion test in short mode")
	}

	t.Parallel()

	t.Run("multiple connection attempts to same refused port", func(t *testing.T) {
		const numAttempts = 10
		port := "localhost:65525"

		for i := 0; i < numAttempts; i++ {
			config := transport.ClientConfig{
				Command:   port,
				Transport: "tcp",
			}

			client, err := transport.NewTCPClient(config)
			if err != nil {
				t.Fatalf("NewTCPClient failed on attempt %d: %v", i, err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)

			err = client.Start(ctx)
			if err == nil {
				t.Errorf("Expected connection refused on attempt %d", i)
				if err := client.Stop(); err != nil {
					t.Logf("Warning: Failed to stop client: %v", err)
				}
			} else if !strings.Contains(strings.ToLower(err.Error()), "connection refused") {
				t.Errorf("Expected connection refused on attempt %d, got: %v", i, err)
			}

			cancel()
			if err := client.Stop(); err != nil {
				t.Logf("Warning: Failed to stop client on attempt %d: %v", i, err)
			}
		}
	})
}

// TestTCPClientConnectionInterruption tests various connection interruption scenarios
func TestTCPClientConnectionInterruption(t *testing.T) {
	t.Parallel()

	t.Run("server closes connection during handshake", func(t *testing.T) {
		listener, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			t.Fatalf("Failed to create listener: %v", err)
		}
		defer func() { _ = listener.Close() }()

		address := listener.Addr().String()

		// Server that closes connection during handshake
		go func() {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// Close immediately after accept
			if err := conn.Close(); err != nil {
				return
			}
		}()

		config := transport.ClientConfig{
			Command:   address,
			Transport: "tcp",
		}

		client, err := transport.NewTCPClient(config)
		if err != nil {
			t.Fatalf("NewTCPClient failed: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		err = client.Start(ctx)
		if err == nil {
			// If start succeeded, client should fail on first operation
			_, reqErr := client.SendRequest(ctx, "test", nil)
			if reqErr == nil {
				t.Error("Expected error due to closed connection")
			}
		} else {
			// Expected error during start
			t.Logf("Expected start error due to connection closure: %v", err)
		}

		if err := client.Stop(); err != nil {
			t.Logf("Warning: Failed to stop client: %v", err)
		}
	})

	t.Run("server resets connection during request", func(t *testing.T) {
		listener, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			t.Fatalf("Failed to create listener: %v", err)
		}
		defer func() { _ = listener.Close() }()

		address := listener.Addr().String()

		// Server that accepts connection then resets during request
		go func() {
			conn, err := listener.Accept()
			if err != nil {
				return
			}

			// Wait a bit to let client connect, then close abruptly
			time.Sleep(100 * time.Millisecond)
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				if err := tcpConn.SetLinger(0); err == nil {
					_ = tcpConn.Close() // RST instead of FIN
				}
			} else {
				_ = conn.Close()
			}
		}()

		config := transport.ClientConfig{
			Command:   address,
			Transport: "tcp",
		}

		client, err := transport.NewTCPClient(config)
		if err != nil {
			t.Fatalf("NewTCPClient failed: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		if err := client.Start(ctx); err != nil {
			t.Fatalf("Start failed: %v", err)
		}

		// Give server time to reset connection
		time.Sleep(200 * time.Millisecond)

		// Request should fail due to reset connection
		_, err = client.SendRequest(ctx, "test", map[string]string{"key": "value"})
		if err == nil {
			t.Error("Expected error due to connection reset")
		}

		if err := client.Stop(); err != nil {
			t.Logf("Warning: Failed to stop client: %v", err)
		}
	})
}

// TestTCPClientFirewallSimulation tests firewall-like blocking scenarios
func TestTCPClientFirewallSimulation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping firewall simulation test in short mode")
	}

	t.Parallel()

	t.Run("filtered port timeout", func(t *testing.T) {
		// Simulate firewall DROP rule by connecting to filtered port
		config := transport.ClientConfig{
			Command:   "198.51.100.2:443", // RFC 5737 test address
			Transport: "tcp",
		}

		client, err := transport.NewTCPClient(config)
		if err != nil {
			t.Fatalf("NewTCPClient failed: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		startTime := time.Now()
		err = client.Start(ctx)
		elapsed := time.Since(startTime)

		if err == nil {
			t.Error("Expected timeout error for filtered port")
			if err := client.Stop(); err != nil {
				t.Logf("Warning: Failed to stop client: %v", err)
			}
			return
		}

		// Should timeout, not refuse
		errStr := strings.ToLower(err.Error())
		if strings.Contains(errStr, "connection refused") {
			t.Error("Expected timeout, not connection refused for filtered port")
		}

		// Should take close to full timeout duration
		if elapsed < 3*time.Second {
			t.Errorf("Timeout happened too quickly: %v", elapsed)
		}
	})
}

// TestTCPClientResourceCleanupOnFailure tests proper resource cleanup after connection failures
func TestTCPClientResourceCleanupOnFailure(t *testing.T) {
	t.Parallel()

	t.Run("cleanup after connection refused", func(t *testing.T) {
		config := transport.ClientConfig{
			Command:   "localhost:65524",
			Transport: "tcp",
		}

		client, err := transport.NewTCPClient(config)
		if err != nil {
			t.Fatalf("NewTCPClient failed: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err = client.Start(ctx)
		if err == nil {
			t.Error("Expected connection refused error")
			if err := client.Stop(); err != nil {
				t.Logf("Warning: Failed to stop client: %v", err)
			}
			return
		}

		// Verify client is properly cleaned up
		if client.IsActive() {
			t.Error("Client should not be active after connection failure")
		}

		// Multiple stop calls should be safe
		for i := 0; i < 3; i++ {
			if err := client.Stop(); err != nil {
				t.Logf("Stop call %d returned error (may be expected): %v", i, err)
			}
		}
	})

	t.Run("cleanup after network unreachable", func(t *testing.T) {
		config := transport.ClientConfig{
			Command:   "198.51.100.3:80", // RFC 5737 test address
			Transport: "tcp",
		}

		client, err := transport.NewTCPClient(config)
		if err != nil {
			t.Fatalf("NewTCPClient failed: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		err = client.Start(ctx)
		if err == nil {
			t.Error("Expected network error")
			if err := client.Stop(); err != nil {
				t.Logf("Warning: Failed to stop client: %v", err)
			}
			return
		}

		// Should not be active and cleanup should be safe
		if client.IsActive() {
			t.Error("Client should not be active after network failure")
		}

		if err := client.Stop(); err != nil {
			t.Logf("Stop after network error: %v", err)
		}
	})
}

// TestTCPClientConcurrentConnectionFailures tests concurrent connection failure handling
func TestTCPClientConcurrentConnectionFailures(t *testing.T) {
	t.Parallel()

	const numConcurrent = 20
	var wg sync.WaitGroup
	errors := make(chan error, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			config := transport.ClientConfig{
				Command:   fmt.Sprintf("localhost:%d", 65500-id),
				Transport: "tcp",
			}

			client, err := transport.NewTCPClient(config)
			if err != nil {
				errors <- fmt.Errorf("client %d creation failed: %w", id, err)
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err = client.Start(ctx)
			if err == nil {
				errors <- fmt.Errorf("client %d: expected connection error", id)
			} else if !strings.Contains(strings.ToLower(err.Error()), "connection refused") {
				errors <- fmt.Errorf("client %d: expected connection refused, got: %v", id, err)
			}

			if err := client.Stop(); err != nil {
				// Log but don't add to errors channel for cleanup issues
				t.Logf("Client %d stop error: %v", id, err)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	errorCount := 0
	for err := range errors {
		errorCount++
		t.Logf("Concurrent test error: %v", err)
	}

	if errorCount > 0 {
		t.Errorf("Found %d errors in concurrent connection failure test", errorCount)
	}
}
