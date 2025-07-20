package transport

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// MemoryProfile tracks memory usage over time
type MemoryProfile struct {
	mu           sync.RWMutex
	samples      []runtime.MemStats
	timestamps   []time.Time
	maxSamples   int
	sampleRate   time.Duration
	monitoring   bool
	stopCh       chan struct{}
	monitoringWG sync.WaitGroup
}

// NewMemoryProfile creates a new memory profiler
func NewMemoryProfile(maxSamples int, sampleRate time.Duration) *MemoryProfile {
	return &MemoryProfile{
		samples:    make([]runtime.MemStats, 0, maxSamples),
		timestamps: make([]time.Time, 0, maxSamples),
		maxSamples: maxSamples,
		sampleRate: sampleRate,
		stopCh:     make(chan struct{}),
	}
}

// StartMonitoring begins continuous memory monitoring
func (mp *MemoryProfile) StartMonitoring() {
	mp.mu.Lock()
	if mp.monitoring {
		mp.mu.Unlock()
		return
	}
	mp.monitoring = true
	mp.mu.Unlock()

	mp.monitoringWG.Add(1)
	go func() {
		defer mp.monitoringWG.Done()
		ticker := time.NewTicker(mp.sampleRate)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				mp.takeSample()
			case <-mp.stopCh:
				return
			}
		}
	}()
}

// StopMonitoring stops memory monitoring
func (mp *MemoryProfile) StopMonitoring() {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	if !mp.monitoring {
		return
	}

	mp.monitoring = false
	close(mp.stopCh)
	mp.monitoringWG.Wait()
}

// takeSample captures current memory statistics
func (mp *MemoryProfile) takeSample() {
	runtime.GC() // Force GC for consistent measurements
	
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	mp.mu.Lock()
	defer mp.mu.Unlock()

	if len(mp.samples) >= mp.maxSamples {
		// Remove oldest sample
		copy(mp.samples, mp.samples[1:])
		copy(mp.timestamps, mp.timestamps[1:])
		mp.samples = mp.samples[:len(mp.samples)-1]
		mp.timestamps = mp.timestamps[:len(mp.timestamps)-1]
	}

	mp.samples = append(mp.samples, memStats)
	mp.timestamps = append(mp.timestamps, time.Now())
}

// GetMemoryTrend analyzes memory usage trend
func (mp *MemoryProfile) GetMemoryTrend() (slope float64, r2 float64, leak bool) {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	if len(mp.samples) < 10 {
		return 0, 0, false
	}

	// Linear regression to detect trend
	n := float64(len(mp.samples))
	var sumX, sumY, sumXY, sumX2 float64

	for i, sample := range mp.samples {
		x := float64(i)
		y := float64(sample.HeapAlloc)
		
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// Calculate slope
	slope = (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	
	// Calculate R-squared
	meanY := sumY / n
	var ssRes, ssTot float64
	
	for i, sample := range mp.samples {
		x := float64(i)
		y := float64(sample.HeapAlloc)
		predicted := slope*x + (sumY-slope*sumX)/n
		
		ssRes += (y - predicted) * (y - predicted)
		ssTot += (y - meanY) * (y - meanY)
	}
	
	if ssTot != 0 {
		r2 = 1 - ssRes/ssTot
	}

	// Consider it a leak if there's a strong positive trend
	leak = slope > 1000 && r2 > 0.7 // Growing by 1KB+ per sample with good correlation

	return slope, r2, leak
}

// GetLeakReport generates a comprehensive leak analysis report
func (mp *MemoryProfile) GetLeakReport() map[string]interface{} {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	if len(mp.samples) == 0 {
		return map[string]interface{}{"error": "no samples available"}
	}

	first := mp.samples[0]
	last := mp.samples[len(mp.samples)-1]
	slope, r2, leak := mp.GetMemoryTrend()

	return map[string]interface{}{
		"sample_count":        len(mp.samples),
		"duration_seconds":    mp.timestamps[len(mp.timestamps)-1].Sub(mp.timestamps[0]).Seconds(),
		"initial_heap_alloc":  first.HeapAlloc,
		"final_heap_alloc":    last.HeapAlloc,
		"heap_growth":         int64(last.HeapAlloc) - int64(first.HeapAlloc),
		"total_allocations":   last.TotalAlloc - first.TotalAlloc,
		"gc_runs":            last.NumGC - first.NumGC,
		"goroutines":         runtime.NumGoroutine(),
		"memory_trend_slope":  slope,
		"correlation_r2":     r2,
		"leak_detected":      leak,
		"heap_objects":       last.HeapObjects,
		"stack_in_use":       last.StackInuse,
		"heap_sys":           last.HeapSys,
	}
}

// LeakTestMockServer simulates a server for testing memory leaks
type LeakTestMockServer struct {
	mu           sync.RWMutex
	connections  map[string]net.Conn
	buffers      [][]byte
	active       bool
	leakMemory   bool
	listener     net.Listener
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

// NewLeakTestMockServer creates a mock server for leak testing
func NewLeakTestMockServer(leakMemory bool) *LeakTestMockServer {
	return &LeakTestMockServer{
		connections: make(map[string]net.Conn),
		buffers:     make([][]byte, 0),
		leakMemory:  leakMemory,
		stopCh:      make(chan struct{}),
	}
}

// Start starts the mock server
func (s *LeakTestMockServer) Start(addr string) error {
	listener, err := net.Listen("tcp", addr)
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

			s.wg.Add(1)
			go s.handleConnection(conn)
		}
	}()

	return nil
}

// Stop stops the mock server
func (s *LeakTestMockServer) Stop() error {
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

	// Close all connections
	for _, conn := range s.connections {
		conn.Close()
	}
	s.mu.Unlock()

	s.wg.Wait()

	// Clean buffers if not leaking
	if !s.leakMemory {
		s.mu.Lock()
		s.buffers = nil
		s.mu.Unlock()
	}

	return nil
}

// handleConnection processes client connections
func (s *LeakTestMockServer) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	connID := fmt.Sprintf("%p", conn)
	
	s.mu.Lock()
	s.connections[connID] = conn
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.connections, connID)
		s.mu.Unlock()
	}()

	reader := bufio.NewReader(conn)
	
	for {
		select {
		case <-s.stopCh:
			return
		default:
		}

		// Read message
		message, err := s.readMessage(reader)
		if err != nil {
			if err == io.EOF {
				return
			}
			continue
		}

		// Simulate memory leak if enabled
		if s.leakMemory {
			// Allocate buffer that won't be freed
			buffer := make([]byte, 1024*1024) // 1MB
			rand.Read(buffer)
			
			s.mu.Lock()
			s.buffers = append(s.buffers, buffer)
			s.mu.Unlock()
		}

		// Send response
		response := JSONRPCMessage{
			JSONRPC: "2.0",
			ID:      message.ID,
			Result:  "mock_response",
		}

		s.writeMessage(conn, response)
	}
}

// readMessage reads a JSON-RPC message
func (s *LeakTestMockServer) readMessage(reader *bufio.Reader) (JSONRPCMessage, error) {
	var msg JSONRPCMessage
	
	// Read headers
	var contentLength int
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return msg, err
		}
		
		if line == "\r\n" || line == "\n" {
			break
		}
		
		if len(line) > 16 && line[:16] == "Content-Length: " {
			fmt.Sscanf(line, "Content-Length: %d", &contentLength)
		}
	}

	if contentLength == 0 {
		return msg, fmt.Errorf("no content length")
	}

	// Read body
	body := make([]byte, contentLength)
	_, err := io.ReadFull(reader, body)
	if err != nil {
		return msg, err
	}

	err = json.Unmarshal(body, &msg)
	return msg, err
}

// writeMessage writes a JSON-RPC message
func (s *LeakTestMockServer) writeMessage(conn net.Conn, msg JSONRPCMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	_, err = conn.Write([]byte(header))
	if err != nil {
		return err
	}

	_, err = conn.Write(data)
	return err
}

// GetBufferCount returns the number of leaked buffers
func (s *LeakTestMockServer) GetBufferCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.buffers)
}

// Test long-running LSP client connections for memory growth
func TestLongRunningLSPClientMemoryGrowth(t *testing.T) {
	testCases := []struct {
		name          string
		clientType    string
		duration      time.Duration
		requestRate   time.Duration
		expectedLeak  bool
	}{
		{"StdioClient_ShortTerm", "stdio", 10 * time.Second, 100 * time.Millisecond, false},
		{"StdioClient_MediumTerm", "stdio", 30 * time.Second, 200 * time.Millisecond, false},
		{"TCPClient_ShortTerm", "tcp", 10 * time.Second, 100 * time.Millisecond, false},
		{"TCPClient_MediumTerm", "tcp", 30 * time.Second, 200 * time.Millisecond, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			profile := NewMemoryProfile(1000, 100*time.Millisecond)
			profile.StartMonitoring()
			defer profile.StopMonitoring()

			var client LSPClient
			var err error
			var mockServer *LeakTestMockServer

			if tc.clientType == "tcp" {
				// Start mock TCP server
				mockServer = NewLeakTestMockServer(false)
				err = mockServer.Start("localhost:0")
				if err != nil {
					t.Fatalf("Failed to start mock server: %v", err)
				}
				defer mockServer.Stop()

				addr := mockServer.listener.Addr().String()
				client, err = NewTCPClient(ClientConfig{
					Command:   addr,
					Transport: "tcp",
				})
			} else {
				// Use echo command for stdio testing
				client, err = NewStdioClient(ClientConfig{
					Command:   "cat", // Simple echo-like behavior
					Args:      []string{},
					Transport: "stdio",
				})
			}

			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			ctx := context.Background()
			err = client.Start(ctx)
			if err != nil {
				t.Fatalf("Failed to start client: %v", err)
			}

			var requestCount int64
			var errorCount int64

			// Run requests for specified duration
			stopTime := time.Now().Add(tc.duration)
			requestTicker := time.NewTicker(tc.requestRate)
			defer requestTicker.Stop()

			for time.Now().Before(stopTime) {
				select {
				case <-requestTicker.C:
					go func() {
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()

						_, err := client.SendRequest(ctx, "test_method", map[string]interface{}{
							"test": "data",
						})
						
						atomic.AddInt64(&requestCount, 1)
						if err != nil {
							atomic.AddInt64(&errorCount, 1)
						}
					}()
				default:
					time.Sleep(10 * time.Millisecond)
				}
			}

			// Allow time for pending requests to complete
			time.Sleep(2 * time.Second)

			err = client.Stop()
			if err != nil {
				t.Logf("Warning: error stopping client: %v", err)
			}

			// Allow cleanup time
			runtime.GC()
			runtime.GC()
			time.Sleep(1 * time.Second)

			// Analyze memory profile
			report := profile.GetLeakReport()
			
			t.Logf("Requests sent: %d, Errors: %d", requestCount, errorCount)
			t.Logf("Memory report: %+v", report)

			// Check for memory leaks
			if leakDetected, ok := report["leak_detected"].(bool); ok {
				if leakDetected != tc.expectedLeak {
					if leakDetected {
						t.Errorf("Unexpected memory leak detected")
					} else {
						t.Errorf("Expected memory leak not detected")
					}
				}
			}

			// Verify reasonable memory growth
			if growth, ok := report["heap_growth"].(int64); ok {
				maxGrowth := int64(requestCount * 10000) // 10KB per request max
				if growth > maxGrowth {
					t.Errorf("Excessive memory growth: %d bytes > %d bytes", growth, maxGrowth)
				}
			}

			// Verify error rate
			errorRate := float64(errorCount) / float64(requestCount) * 100
			if errorRate > 20.0 {
				t.Errorf("High error rate: %.2f%%", errorRate)
			}
		})
	}
}

// Test TCP connection memory cleanup after client disconnection
func TestTCPConnectionMemoryCleanup(t *testing.T) {
	profile := NewMemoryProfile(500, 50*time.Millisecond)
	profile.StartMonitoring()
	defer profile.StopMonitoring()

	// Start mock server
	mockServer := NewLeakTestMockServer(false)
	err := mockServer.Start("localhost:0")
	if err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer mockServer.Stop()

	addr := mockServer.listener.Addr().String()
	
	const clientCount = 20
	const requestsPerClient = 10

	var wg sync.WaitGroup

	// Create and use multiple clients
	for i := 0; i < clientCount; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			client, err := NewTCPClient(ClientConfig{
				Command:   addr,
				Transport: "tcp",
			})
			if err != nil {
				t.Errorf("Client %d: failed to create: %v", clientID, err)
				return
			}

			ctx := context.Background()
			err = client.Start(ctx)
			if err != nil {
				t.Errorf("Client %d: failed to start: %v", clientID, err)
				return
			}

			// Send requests
			for j := 0; j < requestsPerClient; j++ {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_, err := client.SendRequest(ctx, "test_method", map[string]interface{}{
					"client": clientID,
					"request": j,
				})
				cancel()

				if err != nil {
					t.Logf("Client %d request %d failed: %v", clientID, j, err)
				}

				time.Sleep(10 * time.Millisecond)
			}

			// Stop client
			err = client.Stop()
			if err != nil {
				t.Logf("Client %d: error stopping: %v", clientID, err)
			}
		}(i)
	}

	wg.Wait()

	// Force cleanup
	runtime.GC()
	runtime.GC()
	time.Sleep(2 * time.Second)

	// Analyze memory
	report := profile.GetLeakReport()
	t.Logf("Memory cleanup report: %+v", report)

	// Check for leaks
	if leakDetected, ok := report["leak_detected"].(bool); ok && leakDetected {
		t.Error("Memory leak detected after TCP client cleanup")
	}

	// Verify reasonable memory usage
	if growth, ok := report["heap_growth"].(int64); ok {
		maxGrowth := int64(clientCount * requestsPerClient * 5000) // 5KB per request
		if growth > maxGrowth {
			t.Errorf("Excessive memory retention: %d bytes > %d bytes", growth, maxGrowth)
		}
	}
}

// Test stdio subprocess memory cleanup after process termination  
func TestStdioSubprocessMemoryCleanup(t *testing.T) {
	profile := NewMemoryProfile(500, 100*time.Millisecond)
	profile.StartMonitoring()
	defer profile.StopMonitoring()

	const processCount = 10
	const operationsPerProcess = 5

	var wg sync.WaitGroup

	for i := 0; i < processCount; i++ {
		wg.Add(1)
		go func(processID int) {
			defer wg.Done()

			// Use a simple command that echoes input
			client, err := NewStdioClient(ClientConfig{
				Command:   "cat",
				Args:      []string{},
				Transport: "stdio",
			})
			if err != nil {
				t.Errorf("Process %d: failed to create client: %v", processID, err)
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			err = client.Start(ctx)
			if err != nil {
				t.Errorf("Process %d: failed to start: %v", processID, err)
				return
			}

			// Perform operations
			for j := 0; j < operationsPerProcess; j++ {
				opCtx, opCancel := context.WithTimeout(ctx, 5*time.Second)
				
				err := client.SendNotification(opCtx, "test_notification", map[string]interface{}{
					"process":   processID,
					"operation": j,
					"data":      "test_data_string",
				})
				opCancel()

				if err != nil {
					t.Logf("Process %d operation %d failed: %v", processID, j, err)
				}

				time.Sleep(50 * time.Millisecond)
			}

			// Stop client and verify cleanup
			err = client.Stop()
			if err != nil {
				t.Logf("Process %d: error during stop: %v", processID, err)
			}
		}(i)
	}

	wg.Wait()

	// Force cleanup
	runtime.GC()
	runtime.GC()
	time.Sleep(2 * time.Second)

	// Check for process leaks
	initialProcCount := 5 // Approximate baseline
	currentProcCount := countChildProcesses()
	
	if currentProcCount > initialProcCount*2 {
		t.Errorf("Potential process leak: %d processes running", currentProcCount)
	}

	// Analyze memory
	report := profile.GetLeakReport()
	t.Logf("Subprocess cleanup report: %+v", report)

	// Check for memory leaks
	if leakDetected, ok := report["leak_detected"].(bool); ok && leakDetected {
		t.Error("Memory leak detected after subprocess cleanup")
	}

	// Verify reasonable memory usage
	if growth, ok := report["heap_growth"].(int64); ok {
		maxGrowth := int64(processCount * 1024 * 1024) // 1MB per process
		if growth > maxGrowth {
			t.Errorf("Excessive memory retention: %d bytes > %d bytes", growth, maxGrowth)
		}
	}
}

// Test goroutine leak detection during connection lifecycle
func TestGoroutineLeakDetection(t *testing.T) {
	initialGoroutines := runtime.NumGoroutine()
	t.Logf("Initial goroutines: %d", initialGoroutines)

	const clientCount = 50
	const cycleCount = 3

	for cycle := 0; cycle < cycleCount; cycle++ {
		t.Logf("Starting cycle %d", cycle+1)
		
		var clients []LSPClient
		var wg sync.WaitGroup

		// Create clients
		for i := 0; i < clientCount; i++ {
			client, err := NewStdioClient(ClientConfig{
				Command:   "cat",
				Args:      []string{},
				Transport: "stdio",
			})
			if err != nil {
				t.Fatalf("Failed to create client %d: %v", i, err)
			}
			clients = append(clients, client)
		}

		// Start all clients
		for i, client := range clients {
			wg.Add(1)
			go func(clientIdx int, c LSPClient) {
				defer wg.Done()
				
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				
				err := c.Start(ctx)
				if err != nil {
					t.Logf("Client %d failed to start: %v", clientIdx, err)
				}
			}(i, client)
		}

		wg.Wait()
		time.Sleep(100 * time.Millisecond)

		currentGoroutines := runtime.NumGoroutine()
		t.Logf("Goroutines after starting %d clients: %d", clientCount, currentGoroutines)

		// Stop all clients
		for i, client := range clients {
			wg.Add(1)
			go func(clientIdx int, c LSPClient) {
				defer wg.Done()
				
				err := c.Stop()
				if err != nil {
					t.Logf("Client %d failed to stop: %v", clientIdx, err)
				}
			}(i, client)
		}

		wg.Wait()
		
		// Allow cleanup time
		runtime.GC()
		time.Sleep(500 * time.Millisecond)

		finalGoroutines := runtime.NumGoroutine()
		t.Logf("Goroutines after stopping clients: %d", finalGoroutines)

		// Check for goroutine leaks
		leakedGoroutines := finalGoroutines - initialGoroutines
		if leakedGoroutines > clientCount/10 { // Allow some tolerance
			t.Errorf("Cycle %d: potential goroutine leak: %d leaked goroutines", cycle+1, leakedGoroutines)
		}
	}

	finalGoroutines := runtime.NumGoroutine()
	totalLeaked := finalGoroutines - initialGoroutines
	
	t.Logf("Final goroutines: %d, Total leaked: %d", finalGoroutines, totalLeaked)

	if totalLeaked > clientCount/5 { // Allow some tolerance for test infrastructure
		t.Errorf("Total goroutine leak detected: %d goroutines", totalLeaked)
	}
}

// Test memory usage patterns during request/response cycles
func TestRequestResponseMemoryPatterns(t *testing.T) {
	profile := NewMemoryProfile(1000, 50*time.Millisecond)
	profile.StartMonitoring()
	defer profile.StopMonitoring()

	// Start mock server with controlled memory behavior
	mockServer := NewLeakTestMockServer(false)
	err := mockServer.Start("localhost:0")
	if err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer mockServer.Stop()

	addr := mockServer.listener.Addr().String()
	client, err := NewTCPClient(ClientConfig{
		Command:   addr,
		Transport: "tcp",
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()
	err = client.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}
	defer client.Stop()

	const requestBatches = 10
	const requestsPerBatch = 20
	const batchDelay = 500 * time.Millisecond

	var totalRequests int64
	var totalErrors int64

	for batch := 0; batch < requestBatches; batch++ {
		var batchWG sync.WaitGroup

		// Send batch of requests
		for i := 0; i < requestsPerBatch; i++ {
			batchWG.Add(1)
			go func(batchID, reqID int) {
				defer batchWG.Done()

				reqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				_, err := client.SendRequest(reqCtx, "test_method", map[string]interface{}{
					"batch":   batchID,
					"request": reqID,
					"data":    fmt.Sprintf("batch_%d_request_%d_data", batchID, reqID),
				})

				atomic.AddInt64(&totalRequests, 1)
				if err != nil {
					atomic.AddInt64(&totalErrors, 1)
				}
			}(batch, i)
		}

		batchWG.Wait()

		// Force GC between batches
		runtime.GC()
		time.Sleep(batchDelay)

		// Log progress
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		t.Logf("Batch %d completed, current heap: %d bytes", batch+1, memStats.HeapAlloc)
	}

	// Final analysis
	report := profile.GetLeakReport()
	
	t.Logf("Total requests: %d, Errors: %d", totalRequests, totalErrors)
	t.Logf("Memory pattern analysis: %+v", report)

	// Verify memory usage patterns
	if growth, ok := report["heap_growth"].(int64); ok {
		avgGrowthPerRequest := growth / totalRequests
		if avgGrowthPerRequest > 1000 { // 1KB per request
			t.Errorf("High memory usage per request: %d bytes", avgGrowthPerRequest)
		}
	}

	// Check for memory leaks
	if leakDetected, ok := report["leak_detected"].(bool); ok && leakDetected {
		t.Error("Memory leak detected during request/response cycles")
	}

	// Verify error rate
	errorRate := float64(totalErrors) / float64(totalRequests) * 100
	if errorRate > 5.0 {
		t.Errorf("High error rate: %.2f%%", errorRate)
	}
}

// countChildProcesses estimates the number of child processes
func countChildProcesses() int {
	cmd := exec.Command("pgrep", "-P", fmt.Sprintf("%d", os.Getpid()))
	output, err := cmd.Output()
	if err != nil {
		return 0
	}
	
	lines := len(strings.Split(strings.TrimSpace(string(output)), "\n"))
	if len(strings.TrimSpace(string(output))) == 0 {
		return 0
	}
	return lines
}

// Benchmark memory efficiency during client lifecycle
func BenchmarkClientLifecycleMemoryEfficiency(b *testing.B) {
	clientTypes := []string{"stdio", "tcp"}

	for _, clientType := range clientTypes {
		b.Run(fmt.Sprintf("ClientType_%s", clientType), func(b *testing.B) {
			profile := NewMemoryProfile(b.N, 100*time.Millisecond)
			profile.StartMonitoring()
			defer profile.StopMonitoring()

			var mockServer *LeakTestMockServer
			var serverAddr string

			if clientType == "tcp" {
				mockServer = NewLeakTestMockServer(false)
				err := mockServer.Start("localhost:0")
				if err != nil {
					b.Fatalf("Failed to start mock server: %v", err)
				}
				defer mockServer.Stop()
				serverAddr = mockServer.listener.Addr().String()
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				var client LSPClient
				var err error

				if clientType == "tcp" {
					client, err = NewTCPClient(ClientConfig{
						Command:   serverAddr,
						Transport: "tcp",
					})
				} else {
					client, err = NewStdioClient(ClientConfig{
						Command:   "cat",
						Args:      []string{},
						Transport: "stdio",
					})
				}

				if err != nil {
					b.Fatalf("Failed to create client: %v", err)
				}

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				err = client.Start(ctx)
				cancel()

				if err != nil {
					b.Logf("Failed to start client %d: %v", i, err)
					continue
				}

				err = client.Stop()
				if err != nil {
					b.Logf("Failed to stop client %d: %v", i, err)
				}
			}

			report := profile.GetLeakReport()
			if growth, ok := report["heap_growth"].(int64); ok {
				b.Logf("Average memory per lifecycle: %d bytes", growth/int64(b.N))
			}
		})
	}
}

// Benchmark concurrent request memory efficiency
func BenchmarkConcurrentRequestMemoryEfficiency(b *testing.B) {
	concurrencyLevels := []int{1, 5, 10, 20}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(b *testing.B) {
			profile := NewMemoryProfile(1000, 100*time.Millisecond)
			profile.StartMonitoring()
			defer profile.StopMonitoring()

			mockServer := NewLeakTestMockServer(false)
			err := mockServer.Start("localhost:0")
			if err != nil {
				b.Fatalf("Failed to start mock server: %v", err)
			}
			defer mockServer.Stop()

			addr := mockServer.listener.Addr().String()
			client, err := NewTCPClient(ClientConfig{
				Command:   addr,
				Transport: "tcp",
			})
			if err != nil {
				b.Fatalf("Failed to create client: %v", err)
			}

			ctx := context.Background()
			err = client.Start(ctx)
			if err != nil {
				b.Fatalf("Failed to start client: %v", err)
			}
			defer client.Stop()

			b.ResetTimer()
			b.ReportAllocs()

			requests := make(chan int, b.N)
			for i := 0; i < b.N; i++ {
				requests <- i
			}
			close(requests)

			var wg sync.WaitGroup
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for reqID := range requests {
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						_, err := client.SendRequest(ctx, "test_method", map[string]interface{}{
							"request_id": reqID,
						})
						cancel()

						if err != nil {
							b.Logf("Request %d failed: %v", reqID, err)
						}
					}
				}()
			}

			wg.Wait()

			report := profile.GetLeakReport()
			if growth, ok := report["heap_growth"].(int64); ok {
				b.Logf("Average memory per request: %d bytes", growth/int64(b.N))
			}
		})
	}
}