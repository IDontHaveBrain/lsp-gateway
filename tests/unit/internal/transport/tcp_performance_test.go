package transport_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-gateway/internal/transport"
)

// MockTCPServer creates a simple mock TCP server for testing
type MockTCPServer struct {
	listener    net.Listener
	connections []net.Conn
	mu          sync.Mutex
	active      bool
	stopCh      chan struct{}
	wg          sync.WaitGroup
	reqCount    int64
}

func NewMockTCPServer() *MockTCPServer {
	return &MockTCPServer{
		stopCh: make(chan struct{}),
	}
}

func (s *MockTCPServer) Start(address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.listener = listener
	s.active = true
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-s.stopCh:
					return
				default:
					continue
				}
			}

			s.mu.Lock()
			s.connections = append(s.connections, conn)
			s.mu.Unlock()

			s.wg.Add(1)
			go s.handleConnection(conn)
		}
	}()

	return nil
}

func (s *MockTCPServer) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	buf := make([]byte, 4096)
	for {
		select {
		case <-s.stopCh:
			return
		default:
		}

		n, err := conn.Read(buf)
		if err != nil {
			return
		}

		// Parse the message to extract ID for response
		var msg transport.JSONRPCMessage

		// Simple parsing to find the content after headers
		lines := string(buf[:n])
		if contentStart := findContentStart(lines); contentStart != -1 {
			jsonContent := lines[contentStart:]
			if len(jsonContent) > 0 {
				json.Unmarshal([]byte(jsonContent), &msg)
			}
		}

		atomic.AddInt64(&s.reqCount, 1)

		// Send response
		response := transport.JSONRPCMessage{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Result:  "test_result",
		}

		responseData, _ := json.Marshal(response)
		responseMsg := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(responseData), string(responseData))
		conn.Write([]byte(responseMsg))
	}
}

func findContentStart(content string) int {
	// Find the end of headers (double CRLF)
	if idx := findString(content, "\r\n\r\n"); idx != -1 {
		return idx + 4
	}
	if idx := findString(content, "\n\n"); idx != -1 {
		return idx + 2
	}
	return -1
}

func findString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func (s *MockTCPServer) Stop() error {
	s.mu.Lock()
	if !s.active {
		s.mu.Unlock()
		return nil
	}
	s.active = false
	close(s.stopCh)

	if s.listener != nil {
		s.listener.Close()
	}

	for _, conn := range s.connections {
		conn.Close()
	}
	s.connections = nil
	s.mu.Unlock()

	s.wg.Wait()
	return nil
}

func (s *MockTCPServer) GetAddress() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return ""
}

func (s *MockTCPServer) GetRequestCount() int64 {
	return atomic.LoadInt64(&s.reqCount)
}

// TestTCPClientCircuitBreakerPerformance tests the high-performance circuit breaker
func TestTCPClientCircuitBreakerPerformance(t *testing.T) {
	server := NewMockTCPServer()
	err := server.Start("localhost:0")
	if err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer server.Stop()

	address := server.GetAddress()
	client, err := transport.NewTCPClient(transport.ClientConfig{
		Command:   address,
		Transport: "tcp",
	})
	if err != nil {
		t.Fatalf("Failed to create TCP client: %v", err)
	}

	tcpClient := client.(*transport.TCPClient)
	tcpClient.SetMaxRetriesForTesting(5)

	ctx := context.Background()
	err = client.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}
	defer client.Stop()

	t.Run("AtomicCircuitBreakerOperations", func(t *testing.T) {
		// Test that circuit breaker operations are atomic and fast
		start := time.Now()

		// Record many errors concurrently
		var wg sync.WaitGroup
		errorCount := 1000

		for i := 0; i < errorCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				tcpClient.RecordErrorForTesting()
			}()
		}

		wg.Wait()
		duration := time.Since(start)

		t.Logf("Recorded %d errors in %v (avg: %v per error)",
			errorCount, duration, duration/time.Duration(errorCount))

		// Should be very fast with atomic operations
		if duration > 100*time.Millisecond {
			t.Errorf("Circuit breaker operations too slow: %v", duration)
		}

		// Verify error count
		finalErrorCount := tcpClient.GetErrorCountForTesting()
		if finalErrorCount != int64(errorCount) {
			t.Errorf("Expected error count %d, got %d", errorCount, finalErrorCount)
		}

		// Circuit should be open
		if !tcpClient.IsCircuitOpenForTesting() {
			t.Error("Circuit should be open after many errors")
		}
	})

	t.Run("CircuitBreakerStateTransitions", func(t *testing.T) {
		// Reset circuit for this test
		tcpClient.ResetCircuitForTesting()

		// Initially closed
		state := tcpClient.GetCircuitStateForTesting()
		if state != transport.CircuitClosed {
			t.Errorf("Expected circuit closed, got %v", state)
		}

		// Record errors to open circuit
		for i := 0; i < 10; i++ {
			tcpClient.RecordErrorForTesting()
		}

		state = tcpClient.GetCircuitStateForTesting()
		if state != transport.CircuitOpen {
			t.Errorf("Expected circuit open, got %v", state)
		}

		// Set last error time to past for transition test
		tcpClient.SetLastErrorTimeForTesting(time.Now().Add(-35 * time.Second))

		// Check should transition to half-open
		isOpen := tcpClient.IsCircuitOpenForTesting()
		if isOpen {
			t.Error("Circuit should transition to half-open after timeout")
		}

		state = tcpClient.GetCircuitStateForTesting()
		if state != transport.CircuitHalfOpen {
			t.Errorf("Expected circuit half-open, got %v", state)
		}

		// Record successes to close circuit
		for i := 0; i < 5; i++ {
			tcpClient.RecordSuccessForTesting()
		}

		state = tcpClient.GetCircuitStateForTesting()
		if state != transport.CircuitClosed {
			t.Errorf("Expected circuit closed after successes, got %v", state)
		}
	})
}

// TestTCPClientConnectionPooling tests the connection pooling functionality
func TestTCPClientConnectionPooling(t *testing.T) {
	server := NewMockTCPServer()
	err := server.Start("localhost:0")
	if err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer server.Stop()

	address := server.GetAddress()
	client, err := transport.NewTCPClient(transport.ClientConfig{
		Command:   address,
		Transport: "tcp",
	})
	if err != nil {
		t.Fatalf("Failed to create TCP client: %v", err)
	}

	tcpClient := client.(*transport.TCPClient)

	ctx := context.Background()
	err = client.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}
	defer client.Stop()

	t.Run("ConnectionReuseEfficiency", func(t *testing.T) {
		// Make multiple requests and verify connection reuse
		requestCount := 50
		var wg sync.WaitGroup

		start := time.Now()

		for i := 0; i < requestCount; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				_, err := client.SendRequest(ctx, "test_method", map[string]interface{}{
					"test_id": id,
				})
				if err != nil {
					t.Logf("Request %d failed: %v", id, err)
				}
			}(i)
		}

		wg.Wait()
		duration := time.Since(start)

		// Check connection pool stats
		active, total, poolSize := tcpClient.GetConnectionPoolStatsForTesting()

		t.Logf("Sent %d requests in %v", requestCount, duration)
		t.Logf("Connection pool - Active: %d, Total created: %d, Pool size: %d",
			active, total, poolSize)

		// Connection reuse should keep total connections low
		if total > int64(requestCount/5) {
			t.Errorf("Too many connections created: %d (requests: %d)", total, requestCount)
		}

		// Performance should be good
		avgTime := duration / time.Duration(requestCount)
		if avgTime > 50*time.Millisecond {
			t.Errorf("Average request time too high: %v", avgTime)
		}
	})
}

// TestTCPClientMemoryEfficiency tests memory usage patterns
func TestTCPClientMemoryEfficiency(t *testing.T) {
	server := NewMockTCPServer()
	err := server.Start("localhost:0")
	if err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer server.Stop()

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	address := server.GetAddress()
	client, err := transport.NewTCPClient(transport.ClientConfig{
		Command:   address,
		Transport: "tcp",
	})
	if err != nil {
		t.Fatalf("Failed to create TCP client: %v", err)
	}

	ctx := context.Background()
	err = client.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}

	// Perform many operations
	requestCount := 100
	var wg sync.WaitGroup

	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			_, err := client.SendRequest(ctx, "test_method", map[string]interface{}{
				"test_id": id,
			})
			if err != nil {
				// Some failures expected in stress test
			}
		}(i)
	}

	wg.Wait()

	err = client.Stop()
	if err != nil {
		t.Logf("Error stopping client: %v", err)
	}

	// Force cleanup
	runtime.GC()
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	runtime.ReadMemStats(&m2)

	memGrowth := m2.Alloc - m1.Alloc
	t.Logf("Memory usage - Before: %d bytes, After: %d bytes, Growth: %d bytes",
		m1.Alloc, m2.Alloc, memGrowth)

	// Memory growth should be reasonable
	maxGrowth := uint64(requestCount * 1024) // 1KB per request max
	if memGrowth > maxGrowth {
		t.Errorf("Excessive memory growth: %d bytes (max: %d)", memGrowth, maxGrowth)
	}
}

// BenchmarkTCPClientCircuitBreaker benchmarks the circuit breaker performance
func BenchmarkTCPClientCircuitBreaker(b *testing.B) {
	client, err := transport.NewTCPClient(transport.ClientConfig{
		Command:   "localhost:9999", // Non-existent server
		Transport: "tcp",
	})
	if err != nil {
		b.Fatalf("Failed to create TCP client: %v", err)
	}

	tcpClient := client.(*transport.TCPClient)

	b.Run("RecordError", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				tcpClient.RecordErrorForTesting()
			}
		})
	})

	b.Run("RecordSuccess", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				tcpClient.RecordSuccessForTesting()
			}
		})
	})

	b.Run("IsCircuitOpen", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = tcpClient.IsCircuitOpenForTesting()
			}
		})
	})
}

// BenchmarkTCPClientRequestProcessing benchmarks request processing
func BenchmarkTCPClientRequestProcessing(b *testing.B) {
	server := NewMockTCPServer()
	err := server.Start("localhost:0")
	if err != nil {
		b.Fatalf("Failed to start mock server: %v", err)
	}
	defer server.Stop()

	address := server.GetAddress()
	client, err := transport.NewTCPClient(transport.ClientConfig{
		Command:   address,
		Transport: "tcp",
	})
	if err != nil {
		b.Fatalf("Failed to create TCP client: %v", err)
	}

	ctx := context.Background()
	err = client.Start(ctx)
	if err != nil {
		b.Fatalf("Failed to start client: %v", err)
	}
	defer client.Stop()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			_, err := client.SendRequest(ctx, "test_method", map[string]interface{}{
				"test": "data",
			})
			cancel()
			if err != nil {
				// Some errors expected in high-concurrency benchmark
			}
		}
	})
}
