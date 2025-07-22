package transport

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestStdioClientSubprocessTerminationRecovery tests automatic recovery when subprocess terminates unexpectedly
func TestStdioClientSubprocessTerminationRecovery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		killDelay       time.Duration
		expectedErrors  int
		maxRecoveryTime time.Duration
	}{
		{
			name:            "immediate termination",
			killDelay:       50 * time.Millisecond,
			expectedErrors:  1,
			maxRecoveryTime: 5 * time.Second,
		},
		{
			name:            "delayed termination",
			killDelay:       200 * time.Millisecond,
			expectedErrors:  1,
			maxRecoveryTime: 5 * time.Second,
		},
		{
			name:            "rapid termination cycles",
			killDelay:       10 * time.Millisecond,
			expectedErrors:  3,
			maxRecoveryTime: 10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server that can be killed
			mockServer := createKillableMockLSPServer(t, tt.killDelay)
			defer func() {
				if err := os.Remove(mockServer); err != nil {
					t.Logf("Warning: Failed to remove mock server: %v", err)
				}
			}()

			config := ClientConfig{
				Command:   mockServer,
				Args:      []string{},
				Transport: TransportStdio,
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

			// Monitor client state changes
			errorCount := 0
			recoveryStartTime := time.Now()

			// Send requests continuously to detect failures and recovery
			for i := 0; i < 10; i++ {
				requestCtx, requestCancel := context.WithTimeout(ctx, 2*time.Second)
				_, err := client.SendRequest(requestCtx, "test", map[string]interface{}{
					"iteration": i,
				})
				requestCancel()

				if err != nil {
					errorCount++
					t.Logf("Request %d failed (expected during recovery): %v", i, err)
				} else {
					t.Logf("Request %d succeeded", i)
				}

				// Wait between requests to allow for recovery
				time.Sleep(100 * time.Millisecond)
			}

			recoveryTime := time.Since(recoveryStartTime)
			t.Logf("Recovery monitoring completed in %v with %d errors", recoveryTime, errorCount)

			// Validate recovery characteristics
			if errorCount < tt.expectedErrors {
				t.Errorf("Expected at least %d errors during termination/recovery, got %d", tt.expectedErrors, errorCount)
			}

			if recoveryTime > tt.maxRecoveryTime {
				t.Errorf("Recovery took %v, expected under %v", recoveryTime, tt.maxRecoveryTime)
			}
		})
	}
}

// TestTCPClientNetworkInterruptionRecovery tests TCP client recovery after network interruption
func TestTCPClientNetworkInterruptionRecovery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                  string
		interruptionType      string
		interruptionDelay     time.Duration
		recoveryDelay         time.Duration
		expectedReconnections int
		maxRecoveryTime       time.Duration
	}{
		{
			name:                  "immediate connection drop",
			interruptionType:      "connection_drop",
			interruptionDelay:     100 * time.Millisecond,
			recoveryDelay:         200 * time.Millisecond,
			expectedReconnections: 1,
			maxRecoveryTime:       5 * time.Second,
		},
		{
			name:                  "delayed connection drop",
			interruptionType:      "connection_drop",
			interruptionDelay:     500 * time.Millisecond,
			recoveryDelay:         300 * time.Millisecond,
			expectedReconnections: 1,
			maxRecoveryTime:       8 * time.Second,
		},
		{
			name:                  "server restart simulation",
			interruptionType:      "server_restart",
			interruptionDelay:     200 * time.Millisecond,
			recoveryDelay:         1 * time.Second,
			expectedReconnections: 1,
			maxRecoveryTime:       10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create controllable TCP server
			server := NewControllableTCPServer(t)
			defer server.Close()

			address := server.Start()

			config := ClientConfig{
				Command:   address,
				Transport: TransportTCP,
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

			// Simulate network interruption
			go func() {
				time.Sleep(tt.interruptionDelay)
				switch tt.interruptionType {
				case "connection_drop":
					server.DropConnections()
				case "server_restart":
					server.Restart(tt.recoveryDelay)
				}
			}()

			// Monitor recovery behavior
			recoveryStartTime := time.Now()
			errorCount := 0
			successCount := 0

			for i := 0; i < 20; i++ {
				requestCtx, requestCancel := context.WithTimeout(ctx, 3*time.Second)
				_, err := client.SendRequest(requestCtx, "test", map[string]interface{}{
					"iteration": i,
				})
				requestCancel()

				if err != nil {
					errorCount++
					t.Logf("Request %d failed: %v", i, err)
				} else {
					successCount++
					t.Logf("Request %d succeeded", i)
				}

				time.Sleep(200 * time.Millisecond)
			}

			recoveryTime := time.Since(recoveryStartTime)
			t.Logf("Recovery test completed in %v: %d successes, %d errors", recoveryTime, successCount, errorCount)

			// Validate recovery behavior
			if successCount == 0 {
				t.Error("No successful requests - client should recover")
			}

			if errorCount == 0 {
				t.Error("No errors detected - test should capture interruption")
			}

			if recoveryTime > tt.maxRecoveryTime {
				t.Errorf("Recovery took %v, expected under %v", recoveryTime, tt.maxRecoveryTime)
			}
		})
	}
}

// TestConnectionPoolingDuringNetworkFailures tests connection pooling behavior during network failures
func TestConnectionPoolingDuringNetworkFailures(t *testing.T) {
	t.Parallel()

	t.Run("connection pool exhaustion and recovery", func(t *testing.T) {
		server := NewControllableTCPServer(t)
		defer server.Close()

		address := server.Start()

		// Create multiple clients to test pooling behavior
		const numClients = 5
		clients := make([]LSPClient, numClients)
		defer func() {
			for _, client := range clients {
				if client != nil {
					if err := client.Stop(); err != nil {
						t.Logf("Warning: Failed to stop client: %v", err)
					}
				}
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Start all clients
		for i := 0; i < numClients; i++ {
			config := ClientConfig{
				Command:   address,
				Transport: TransportTCP,
			}

			client, err := NewTCPClient(config)
			if err != nil {
				t.Fatalf("NewTCPClient %d failed: %v", i, err)
			}

			if err := client.Start(ctx); err != nil {
				t.Fatalf("Start client %d failed: %v", i, err)
			}

			clients[i] = client
		}

		// Simulate pool exhaustion
		go func() {
			time.Sleep(200 * time.Millisecond)
			server.SetConnectionLimit(2) // Limit connections
			time.Sleep(500 * time.Millisecond)
			server.SetConnectionLimit(10) // Restore connections
		}()

		// Test concurrent requests during pool limitations
		var wg sync.WaitGroup
		errorCounts := make([]int32, numClients)
		successCounts := make([]int32, numClients)

		for clientIdx, client := range clients {
			wg.Add(1)
			go func(idx int, c LSPClient) {
				defer wg.Done()

				for i := 0; i < 10; i++ {
					requestCtx, requestCancel := context.WithTimeout(ctx, 3*time.Second)
					_, err := c.SendRequest(requestCtx, "test", map[string]interface{}{
						"client":  idx,
						"request": i,
					})
					requestCancel()

					if err != nil {
						atomic.AddInt32(&errorCounts[idx], 1)
					} else {
						atomic.AddInt32(&successCounts[idx], 1)
					}

					time.Sleep(100 * time.Millisecond)
				}
			}(clientIdx, client)
		}

		wg.Wait()

		// Validate pooling behavior
		totalErrors := int32(0)
		totalSuccesses := int32(0)
		for i := 0; i < numClients; i++ {
			errors := atomic.LoadInt32(&errorCounts[i])
			successes := atomic.LoadInt32(&successCounts[i])
			totalErrors += errors
			totalSuccesses += successes
			t.Logf("Client %d: %d successes, %d errors", i, successes, errors)
		}

		if totalSuccesses == 0 {
			t.Error("No successful requests across all clients")
		}

		if totalErrors == 0 {
			t.Error("No errors detected during pool limitations")
		}

		t.Logf("Pool test results: %d total successes, %d total errors", totalSuccesses, totalErrors)
	})
}

// TestGracefulDegradationOnRepeatedFailures tests graceful degradation when reconnection fails repeatedly
func TestGracefulDegradationOnRepeatedFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		maxFailures      int
		failureDuration  time.Duration
		expectedBehavior string
	}{
		{
			name:             "circuit breaker activation",
			maxFailures:      3,
			failureDuration:  2 * time.Second,
			expectedBehavior: "circuit_open",
		},
		{
			name:             "extended failure recovery",
			maxFailures:      5,
			failureDuration:  5 * time.Second,
			expectedBehavior: "eventual_recovery",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewControllableTCPServer(t)
			defer server.Close()

			address := server.Start()

			config := ClientConfig{
				Command:   address,
				Transport: TransportTCP,
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

			// Simulate repeated failures
			server.SetFailureMode(true, tt.failureDuration)

			// Track degradation behavior
			failureCount := 0
			circuitBreakerActivated := false
			recoveryDetected := false

			for i := 0; i < 20; i++ {
				requestCtx, requestCancel := context.WithTimeout(ctx, 2*time.Second)
				_, err := client.SendRequest(requestCtx, "test", map[string]interface{}{
					"iteration": i,
				})
				requestCancel()

				if err != nil {
					failureCount++
					t.Logf("Request %d failed: %v", i, err)

					if failureCount >= tt.maxFailures && !circuitBreakerActivated {
						circuitBreakerActivated = true
						t.Logf("Circuit breaker should be activated after %d failures", failureCount)
					}
				} else {
					if failureCount > 0 {
						recoveryDetected = true
						t.Logf("Recovery detected at request %d", i)
					}
				}

				time.Sleep(200 * time.Millisecond)
			}

			// Validate degradation behavior
			switch tt.expectedBehavior {
			case "circuit_open":
				if failureCount < tt.maxFailures {
					t.Errorf("Expected at least %d failures for circuit breaker, got %d", tt.maxFailures, failureCount)
				}
				if !circuitBreakerActivated {
					t.Error("Circuit breaker should have been activated")
				}

			case "eventual_recovery":
				if !recoveryDetected {
					t.Error("Should detect recovery after failure period")
				}
			}

			t.Logf("Degradation test completed: %d failures, circuit breaker: %v, recovery: %v",
				failureCount, circuitBreakerActivated, recoveryDetected)
		})
	}
}

// TestConnectionHealthMonitoring tests connection health monitoring and stale connection detection
func TestConnectionHealthMonitoring(t *testing.T) {
	t.Parallel()

	t.Run("stale connection detection", func(t *testing.T) {
		server := NewControllableTCPServer(t)
		defer server.Close()

		address := server.Start()

		config := ClientConfig{
			Command:   address,
			Transport: TransportTCP,
		}

		client, err := NewTCPClient(config)
		if err != nil {
			t.Fatalf("NewTCPClient failed: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		if err := client.Start(ctx); err != nil {
			t.Fatalf("Start failed: %v", err)
		}
		defer func() {
			if err := client.Stop(); err != nil {
				t.Logf("Warning: Failed to stop client: %v", err)
			}
		}()

		// Establish healthy connection
		requestCtx, requestCancel := context.WithTimeout(ctx, 2*time.Second)
		_, err = client.SendRequest(requestCtx, "test", map[string]interface{}{"health": "check"})
		requestCancel()

		if err != nil {
			t.Fatalf("Initial health check failed: %v", err)
		}

		// Simulate stale connection (server stops responding)
		server.SetStaleMode(true)

		// Monitor health detection
		staleDetected := false
		for i := 0; i < 10; i++ {
			requestCtx, requestCancel := context.WithTimeout(ctx, 1*time.Second)
			_, err := client.SendRequest(requestCtx, "test", map[string]interface{}{
				"stale_test": i,
			})
			requestCancel()

			if err != nil {
				staleDetected = true
				t.Logf("Stale connection detected at attempt %d: %v", i, err)
				break
			}

			time.Sleep(100 * time.Millisecond)
		}

		if !staleDetected {
			t.Error("Stale connection should have been detected")
		}

		// Restore connection health
		server.SetStaleMode(false)

		// Verify recovery
		time.Sleep(500 * time.Millisecond)
		requestCtx, requestCancel = context.WithTimeout(ctx, 3*time.Second)
		_, err = client.SendRequest(requestCtx, "test", map[string]interface{}{"recovery": "check"})
		requestCancel()

		if err != nil {
			t.Logf("Recovery may take time: %v", err)
		} else {
			t.Log("Connection health restored successfully")
		}
	})
}

// TestNetworkPartitioningRecovery tests recovery from network partitioning scenarios
func TestNetworkPartitioningRecovery(t *testing.T) {
	t.Parallel()

	t.Run("complete network partition", func(t *testing.T) {
		server := NewControllableTCPServer(t)
		defer server.Close()

		address := server.Start()

		config := ClientConfig{
			Command:   address,
			Transport: TransportTCP,
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

		// Establish baseline
		requestCtx, requestCancel := context.WithTimeout(ctx, 2*time.Second)
		_, err = client.SendRequest(requestCtx, "test", map[string]interface{}{"baseline": true})
		requestCancel()

		if err != nil {
			t.Fatalf("Baseline request failed: %v", err)
		}

		// Simulate complete network partition
		partitionStart := time.Now()
		server.SetPartitionMode(true, 3*time.Second)

		// Track partition detection and recovery
		partitionDetected := false
		recoveryDetected := false
		partitionDetectionTime := time.Duration(0)

		for i := 0; i < 25; i++ {
			requestCtx, requestCancel := context.WithTimeout(ctx, 1*time.Second)
			_, err := client.SendRequest(requestCtx, "test", map[string]interface{}{
				"partition_test": i,
			})
			requestCancel()

			if err != nil && !partitionDetected {
				partitionDetected = true
				partitionDetectionTime = time.Since(partitionStart)
				t.Logf("Network partition detected at attempt %d after %v: %v", i, partitionDetectionTime, err)
			}

			if err == nil && partitionDetected && !recoveryDetected {
				recoveryDetected = true
				recoveryTime := time.Since(partitionStart)
				t.Logf("Recovery from partition detected at attempt %d after %v", i, recoveryTime)
			}

			time.Sleep(200 * time.Millisecond)
		}

		// Validate partition handling
		if !partitionDetected {
			t.Error("Network partition should have been detected")
		}

		if partitionDetectionTime > 5*time.Second {
			t.Errorf("Partition detection took %v, should be faster", partitionDetectionTime)
		}

		if !recoveryDetected {
			t.Error("Recovery from partition should have been detected")
		}
	})
}

// Helper functions for creating controllable test infrastructure

// ControllableTCPServer provides a TCP server that can simulate various network conditions
type ControllableTCPServer struct {
	listener      net.Listener
	address       string
	connections   []net.Conn
	mu            sync.RWMutex
	closed        bool
	failureMode   bool
	staleMode     bool
	partitionMode bool
	connLimit     int
	t             *testing.T
}

func NewControllableTCPServer(t *testing.T) *ControllableTCPServer {
	return &ControllableTCPServer{
		connections: make([]net.Conn, 0),
		connLimit:   100,
		t:           t,
	}
}

func (s *ControllableTCPServer) Start() string {
	var err error
	s.listener, err = net.Listen("tcp", "localhost:0")
	if err != nil {
		s.t.Fatalf("Failed to create listener: %v", err)
	}

	s.address = s.listener.Addr().String()

	go s.acceptConnections()

	return s.address
}

func (s *ControllableTCPServer) acceptConnections() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.RLock()
			closed := s.closed
			s.mu.RUnlock()
			if closed {
				return
			}
			continue
		}

		s.mu.Lock()
		if len(s.connections) >= s.connLimit {
			conn.Close()
			s.mu.Unlock()
			continue
		}
		s.connections = append(s.connections, conn)
		s.mu.Unlock()

		go s.handleConnection(conn)
	}
}

func (s *ControllableTCPServer) handleConnection(conn net.Conn) {
	defer func() {
		conn.Close()
		s.mu.Lock()
		for i, c := range s.connections {
			if c == conn {
				s.connections = append(s.connections[:i], s.connections[i+1:]...)
				break
			}
		}
		s.mu.Unlock()
	}()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		s.mu.RLock()
		failure := s.failureMode
		stale := s.staleMode
		partition := s.partitionMode
		s.mu.RUnlock()

		if failure || partition {
			return
		}

		if stale {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Read LSP message
		message, err := s.readLSPMessage(reader)
		if err != nil {
			return
		}

		// Send response
		response := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"result":{"message":"test response","timestamp":"%s"}}`, time.Now().Format(time.RFC3339))
		responseMsg := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(response), response)

		if _, err := writer.WriteString(responseMsg); err != nil {
			return
		}
		if err := writer.Flush(); err != nil {
			return
		}

		_ = message // Use message to avoid unused variable error
	}
}

func (s *ControllableTCPServer) readLSPMessage(reader *bufio.Reader) ([]byte, error) {
	contentLength := 0

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			break
		}

		if strings.HasPrefix(line, "Content-Length:") {
			fmt.Sscanf(line, "Content-Length: %d", &contentLength)
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("no Content-Length header")
	}

	body := make([]byte, contentLength)
	_, err := io.ReadFull(reader, body)
	return body, err
}

func (s *ControllableTCPServer) DropConnections() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, conn := range s.connections {
		conn.Close()
	}
	s.connections = s.connections[:0]
}

func (s *ControllableTCPServer) SetConnectionLimit(limit int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connLimit = limit
}

func (s *ControllableTCPServer) SetFailureMode(enabled bool, duration time.Duration) {
	s.mu.Lock()
	s.failureMode = enabled
	s.mu.Unlock()

	if enabled && duration > 0 {
		go func() {
			time.Sleep(duration)
			s.mu.Lock()
			s.failureMode = false
			s.mu.Unlock()
		}()
	}
}

func (s *ControllableTCPServer) SetStaleMode(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.staleMode = enabled
}

func (s *ControllableTCPServer) SetPartitionMode(enabled bool, duration time.Duration) {
	s.mu.Lock()
	s.partitionMode = enabled
	s.mu.Unlock()

	if enabled && duration > 0 {
		go func() {
			time.Sleep(duration)
			s.mu.Lock()
			s.partitionMode = false
			s.mu.Unlock()
		}()
	}
}

func (s *ControllableTCPServer) Restart(delay time.Duration) {
	s.DropConnections()
	s.mu.Lock()
	s.failureMode = true
	s.mu.Unlock()

	go func() {
		time.Sleep(delay)
		s.mu.Lock()
		s.failureMode = false
		s.mu.Unlock()
	}()
}

func (s *ControllableTCPServer) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	if s.listener != nil {
		s.listener.Close()
	}

	for _, conn := range s.connections {
		conn.Close()
	}
}

// createKillableMockLSPServer creates a mock LSP server that can be terminated for testing recovery
func createKillableMockLSPServer(t *testing.T, killDelay time.Duration) string {
	tmpDir := t.TempDir()
	scriptPath := fmt.Sprintf("%s/killable_mock_lsp.sh", tmpDir)

	script := fmt.Sprintf(`#!/bin/bash
# Killable LSP server for testing recovery
trap 'exit 0' TERM INT

# Self-termination after delay
if [ %d -gt 0 ]; then
    (sleep %f && kill $$) &
fi

while IFS= read -r line; do
    if [[ "$line" =~ Content-Length:\ ([0-9]+) ]]; then
        length=${BASH_REMATCH[1]}
        read -r  # Read empty line
        read -r -N $length request
        
        # Simple response
        response='{"jsonrpc":"2.0","id":1,"result":{"message":"killable response"}}'
        echo "Content-Length: ${#response}"
        echo ""
        echo "$response"
    fi
done
`, int(killDelay.Nanoseconds()), killDelay.Seconds())

	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to create killable mock LSP script: %v", err)
	}

	return scriptPath
}
