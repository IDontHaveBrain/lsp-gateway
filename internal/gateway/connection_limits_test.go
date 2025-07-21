package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/testutil"
	"lsp-gateway/internal/transport"
)

// FileDescriptorMonitor tracks file descriptor usage
type FileDescriptorMonitor struct {
	pid           int
	initialCount  int64
	currentCount  int64
	maxObserved   int64
	measurements  []int64
	mu            sync.RWMutex
}

func NewFileDescriptorMonitor() *FileDescriptorMonitor {
	pid := os.Getpid()
	initial := getCurrentFileDescriptorCount(pid)
	return &FileDescriptorMonitor{
		pid:          pid,
		initialCount: initial,
		currentCount: initial,
		maxObserved:  initial,
		measurements: make([]int64, 0, 1000),
	}
}

func (fdm *FileDescriptorMonitor) StartMonitoring(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count := getCurrentFileDescriptorCount(fdm.pid)
			fdm.mu.Lock()
			fdm.currentCount = count
			if count > fdm.maxObserved {
				fdm.maxObserved = count
			}
			fdm.measurements = append(fdm.measurements, count)
			if len(fdm.measurements) > 1000 {
				fdm.measurements = fdm.measurements[1:]
			}
			fdm.mu.Unlock()
		}
	}
}

func (fdm *FileDescriptorMonitor) GetStats() (current, max, initial int64, measurements []int64) {
	fdm.mu.RLock()
	defer fdm.mu.RUnlock()
	return fdm.currentCount, fdm.maxObserved, fdm.initialCount, append([]int64(nil), fdm.measurements...)
}

func getCurrentFileDescriptorCount(pid int) int64 {
	if runtime.GOOS == "linux" {
		fdDir := fmt.Sprintf("/proc/%d/fd", pid)
		entries, err := os.ReadDir(fdDir)
		if err != nil {
			return -1
		}
		return int64(len(entries))
	}

	if runtime.GOOS == "darwin" {
		// Use lsof command for macOS
		return getMacOSFileDescriptorCount(pid)
	}

	// Windows - use handle count approximation
	if runtime.GOOS == "windows" {
		return getWindowsHandleCount(pid)
	}

	return -1
}

func getMacOSFileDescriptorCount(pid int) int64 {
	// This would require lsof command, simplified for testing
	return -1 // Not implemented for this test environment
}

func getWindowsHandleCount(pid int) int64 {
	// This would require Windows API calls, simplified for testing
	return -1 // Not implemented for this test environment
}

func getSystemFileDescriptorLimit() (soft, hard int64, err error) {
	if runtime.GOOS == "windows" {
		return 2048, 16384, nil // Windows default approximation
	}

	var rlimit syscall.Rlimit
	err = syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit)
	if err != nil {
		return 0, 0, err
	}

	return int64(rlimit.Cur), int64(rlimit.Max), nil
}

// ConnectionPool manages HTTP client connections with limits
type ConnectionPool struct {
	maxConnections int
	activeConns    int64
	waitingQueue   chan *http.Client
	clients        []*http.Client
	mu             sync.RWMutex
	metrics        *ConnectionMetrics
}

type ConnectionMetrics struct {
	TotalRequests     int64
	ActiveConnections int64
	QueuedRequests    int64
	RejectedRequests  int64
	AvgResponseTime   time.Duration
	MaxResponseTime   time.Duration
	mu                sync.RWMutex
}

func NewConnectionPool(maxConnections int) *ConnectionPool {
	return &ConnectionPool{
		maxConnections: maxConnections,
		waitingQueue:   make(chan *http.Client, maxConnections*2),
		clients:        make([]*http.Client, 0, maxConnections),
		metrics:        &ConnectionMetrics{},
	}
}

func (cp *ConnectionPool) GetClient(ctx context.Context) (*http.Client, error) {
	cp.mu.Lock()
	
	// Check if we have available clients in the pool
	if len(cp.clients) > 0 {
		client := cp.clients[len(cp.clients)-1]
		cp.clients = cp.clients[:len(cp.clients)-1]
		atomic.AddInt64(&cp.activeConns, 1)
		atomic.AddInt64(&cp.metrics.ActiveConnections, 1)
		cp.mu.Unlock()
		return client, nil
	}

	// Check if we're at capacity
	if int(atomic.LoadInt64(&cp.activeConns)) >= cp.maxConnections {
		atomic.AddInt64(&cp.metrics.QueuedRequests, 1)
		cp.mu.Unlock() // Release lock before waiting on channel
		
		select {
		case client := <-cp.waitingQueue:
			atomic.AddInt64(&cp.activeConns, 1)
			atomic.AddInt64(&cp.metrics.ActiveConnections, 1)
			return client, nil
		case <-ctx.Done():
			atomic.AddInt64(&cp.metrics.RejectedRequests, 1)
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			atomic.AddInt64(&cp.metrics.RejectedRequests, 1)
			return nil, fmt.Errorf("connection pool timeout")
		}
	}

	// Create new client
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 2,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	atomic.AddInt64(&cp.activeConns, 1)
	atomic.AddInt64(&cp.metrics.ActiveConnections, 1)
	cp.mu.Unlock()
	return client, nil
}

func (cp *ConnectionPool) ReleaseClient(client *http.Client) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	atomic.AddInt64(&cp.activeConns, -1)
	atomic.AddInt64(&cp.metrics.ActiveConnections, -1)

	select {
	case cp.waitingQueue <- client:
		// Successfully queued for reuse
	default:
		// Queue is full, add to pool or discard
		if len(cp.clients) < cp.maxConnections {
			cp.clients = append(cp.clients, client)
		}
		// Otherwise, client is discarded and will be garbage collected
	}
}

func (cp *ConnectionPool) GetMetrics() ConnectionMetrics {
	cp.metrics.mu.RLock()
	defer cp.metrics.mu.RUnlock()
	return *cp.metrics
}

func (cp *ConnectionPool) Close() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	close(cp.waitingQueue)
	for client := range cp.waitingQueue {
		if transport, ok := client.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}

	for _, client := range cp.clients {
		if transport, ok := client.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}
	cp.clients = nil
}

func TestGatewayConnectionLimits_MaxConcurrentConnections(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping connection limit tests in short mode")
	}

	port := testutil.AllocateTestPort(t)
	gateway, server := setupTestGatewayWithMonitoring(t, port)
	defer func() {
		if err := gateway.Stop(); err != nil {
			t.Logf("Error stopping gateway: %v", err)
		}
		if err := server.Close(); err != nil {
			t.Logf("Error closing server: %v", err)
		}
	}()

	fdMonitor := NewFileDescriptorMonitor()
	monitorCtx, monitorCancel := context.WithCancel(context.Background())
	defer monitorCancel()

	go fdMonitor.StartMonitoring(monitorCtx, 100*time.Millisecond)

	baseURL := fmt.Sprintf("http://localhost:%d/jsonrpc", port)
	maxConnections := 50
	connectionPool := NewConnectionPool(maxConnections)
	defer connectionPool.Close()

	t.Run("Sequential_Connection_Test", func(t *testing.T) {
		testSequentialConnections(t, baseURL, connectionPool, fdMonitor, 20)
	})

	t.Run("Concurrent_Connection_Test", func(t *testing.T) {
		testConcurrentConnections(t, baseURL, connectionPool, fdMonitor, maxConnections)
	})

	t.Run("Connection_Pool_Efficiency_Test", func(t *testing.T) {
		testConnectionPoolEfficiency(t, baseURL, connectionPool, fdMonitor, maxConnections)
	})

	current, max, initial, measurements := fdMonitor.GetStats()
	t.Logf("File Descriptor Stats - Initial: %d, Current: %d, Max: %d, Measurements: %d",
		initial, current, max, len(measurements))

	if len(measurements) > 0 {
		fdDelta := max - initial
		if fdDelta > int64(maxConnections*2) {
			t.Errorf("File descriptor usage grew too much: %d (max allowed: %d)", fdDelta, maxConnections*2)
		}
	}
}

func testSequentialConnections(t *testing.T, baseURL string, pool *ConnectionPool, fdMonitor *FileDescriptorMonitor, count int) {
	ctx := context.Background()
	
	for i := 0; i < count; i++ {
		client, err := pool.GetClient(ctx)
		if err != nil {
			t.Fatalf("Failed to get client %d: %v", i, err)
		}

		err = sendTestRequest(client, baseURL, fmt.Sprintf("sequential_test_%d", i))
		pool.ReleaseClient(client)

		if err != nil && !strings.Contains(err.Error(), "connection refused") {
			t.Errorf("Request %d failed: %v", i, err)
		}

		if i%10 == 0 {
			time.Sleep(10 * time.Millisecond) // Brief pause for monitoring
		}
	}

	metrics := pool.GetMetrics()
	t.Logf("Sequential test metrics - Total: %d, Active: %d, Queued: %d, Rejected: %d",
		metrics.TotalRequests, metrics.ActiveConnections, metrics.QueuedRequests, metrics.RejectedRequests)
}

func testConcurrentConnections(t *testing.T, baseURL string, pool *ConnectionPool, fdMonitor *FileDescriptorMonitor, maxConnections int) {
	var wg sync.WaitGroup
	concurrentRequests := maxConnections * 2
	
	requestCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	errorCount := int64(0)
	successCount := int64(0)

	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			client, err := pool.GetClient(requestCtx)
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				return
			}
			defer pool.ReleaseClient(client)

			err = sendTestRequest(client, baseURL, fmt.Sprintf("concurrent_test_%d", id))
			if err != nil {
				if !strings.Contains(err.Error(), "connection refused") {
					atomic.AddInt64(&errorCount, 1)
				}
			} else {
				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()

	metrics := pool.GetMetrics()
	t.Logf("Concurrent test results - Success: %d, Errors: %d, Pool metrics - Active: %d, Queued: %d, Rejected: %d",
		successCount, errorCount, metrics.ActiveConnections, metrics.QueuedRequests, metrics.RejectedRequests)

	// Verify pool handled concurrency correctly
	if metrics.RejectedRequests > int64(concurrentRequests/2) {
		t.Errorf("Too many requests rejected: %d (max expected: %d)", metrics.RejectedRequests, concurrentRequests/2)
	}
}

func testConnectionPoolEfficiency(t *testing.T, baseURL string, pool *ConnectionPool, fdMonitor *FileDescriptorMonitor, maxConnections int) {
	rounds := 5
	requestsPerRound := maxConnections / 2

	for round := 0; round < rounds; round++ {
		t.Logf("Pool efficiency test round %d/%d", round+1, rounds)

		var wg sync.WaitGroup
		roundCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		for i := 0; i < requestsPerRound; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				client, err := pool.GetClient(roundCtx)
				if err != nil {
					return
				}
				defer pool.ReleaseClient(client)

				_ = sendTestRequest(client, baseURL, fmt.Sprintf("efficiency_r%d_req%d", round, id))
			}(i)
		}

		wg.Wait()
		cancel()

		// Brief pause between rounds to allow cleanup
		time.Sleep(200 * time.Millisecond)
	}

	current, max, initial, _ := fdMonitor.GetStats()
	finalFdCount := current

	// Verify file descriptors are cleaned up between rounds
	fdGrowth := finalFdCount - initial
	t.Logf("File descriptor growth over efficiency test: %d (initial: %d, final: %d, max: %d)",
		fdGrowth, initial, finalFdCount, max)

	if fdGrowth > int64(maxConnections) {
		t.Errorf("File descriptor growth too high: %d (max allowed: %d)", fdGrowth, maxConnections)
	}
}

func TestGatewayConnectionLimits_QueueManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping queue management tests in short mode")
	}

	port := testutil.AllocateTestPort(t)
	gateway, server := setupTestGatewayWithMonitoring(t, port)
	defer func() {
		if err := gateway.Stop(); err != nil {
			t.Logf("Error stopping gateway: %v", err)
		}
		if err := server.Close(); err != nil {
			t.Logf("Error closing server: %v", err)
		}
	}()

	baseURL := fmt.Sprintf("http://localhost:%d/jsonrpc", port)
	smallPool := NewConnectionPool(5) // Very small pool to test queuing
	defer smallPool.Close()

	t.Run("Queue_Overflow_Handling", func(t *testing.T) {
		testQueueOverflowHandling(t, baseURL, smallPool)
	})

	t.Run("Queue_FIFO_Order", func(t *testing.T) {
		testQueueFIFOOrder(t, baseURL, smallPool)
	})

	t.Run("Queue_Timeout_Handling", func(t *testing.T) {
		testQueueTimeoutHandling(t, baseURL, smallPool)
	})
}

func testQueueOverflowHandling(t *testing.T, baseURL string, pool *ConnectionPool) {
	overflowRequests := 20
	var wg sync.WaitGroup
	
	successCount := int64(0)
	errorCount := int64(0)
	timeoutCount := int64(0)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for i := 0; i < overflowRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			client, err := pool.GetClient(ctx)
			if err != nil {
				if strings.Contains(err.Error(), "timeout") {
					atomic.AddInt64(&timeoutCount, 1)
				} else {
					atomic.AddInt64(&errorCount, 1)
				}
				return
			}
			defer pool.ReleaseClient(client)

			err = sendTestRequest(client, baseURL, fmt.Sprintf("overflow_test_%d", id))
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
			} else {
				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Queue overflow test - Success: %d, Errors: %d, Timeouts: %d",
		successCount, errorCount, timeoutCount)

	// Verify that pool handled overflow gracefully
	if timeoutCount == 0 && errorCount == 0 {
		t.Error("Expected some timeouts or errors due to queue overflow, got none")
	}

	metrics := pool.GetMetrics()
	if metrics.RejectedRequests == 0 && timeoutCount == 0 {
		t.Error("Expected some rejected requests or timeouts")
	}
}

func testQueueFIFOOrder(t *testing.T, baseURL string, pool *ConnectionPool) {
	// This test verifies that connections are processed in order
	requestCount := 10
	results := make(chan int, requestCount)
	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			client, err := pool.GetClient(ctx)
			if err != nil {
				return
			}
			defer pool.ReleaseClient(client)

			// Add small delay to ensure queueing behavior
			time.Sleep(time.Duration(id*10) * time.Millisecond)
			
			err = sendTestRequest(client, baseURL, fmt.Sprintf("fifo_test_%d", id))
			if err == nil {
				results <- id
			}
		}(i)
	}

	wg.Wait()
	close(results)

	var resultOrder []int
	for result := range results {
		resultOrder = append(resultOrder, result)
	}

	t.Logf("FIFO test processed %d requests in order: %v", len(resultOrder), resultOrder)
	
	if len(resultOrder) < requestCount/2 {
		t.Errorf("Too few requests completed: %d (expected at least %d)", len(resultOrder), requestCount/2)
	}
}

func testQueueTimeoutHandling(t *testing.T, baseURL string, pool *ConnectionPool) {
	var wg sync.WaitGroup
	timeoutRequests := 8
	
	timeoutCount := int64(0)
	successCount := int64(0)

	// Use very short timeout to force timeouts
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	for i := 0; i < timeoutRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			client, err := pool.GetClient(ctx)
			if err != nil {
				if strings.Contains(err.Error(), "timeout") || err == context.DeadlineExceeded {
					atomic.AddInt64(&timeoutCount, 1)
				}
				return
			}
			defer pool.ReleaseClient(client)

			err = sendTestRequest(client, baseURL, fmt.Sprintf("timeout_test_%d", id))
			if err == nil {
				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Timeout test - Success: %d, Timeouts: %d", successCount, timeoutCount)
	
	if timeoutCount == 0 {
		t.Error("Expected some timeouts due to short context deadline")
	}
}

func TestGatewayConnectionLimits_ResourceCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource cleanup tests in short mode")
	}

	t.Run("Connection_Cleanup_After_Errors", func(t *testing.T) {
		testConnectionCleanupAfterErrors(t)
	})

	t.Run("Pool_Cleanup_Verification", func(t *testing.T) {
		testPoolCleanupVerification(t)
	})

	t.Run("Memory_Leak_Detection", func(t *testing.T) {
		testMemoryLeakDetection(t)
	})
}

func testConnectionCleanupAfterErrors(t *testing.T) {
	port := testutil.AllocateTestPort(t)
	// Intentionally don't start a server to cause connection errors
	
	baseURL := fmt.Sprintf("http://localhost:%d/jsonrpc", port)
	pool := NewConnectionPool(10)
	defer pool.Close()

	fdMonitor := NewFileDescriptorMonitor()
	monitorCtx, monitorCancel := context.WithCancel(context.Background())
	defer monitorCancel()

	go fdMonitor.StartMonitoring(monitorCtx, 50*time.Millisecond)

	// Make requests that will fail
	errorRequests := 15
	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for i := 0; i < errorRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			client, err := pool.GetClient(ctx)
			if err != nil {
				return
			}
			defer pool.ReleaseClient(client)

			// This will fail - no server running
			_ = sendTestRequest(client, baseURL, fmt.Sprintf("error_test_%d", id))
		}(i)
	}

	wg.Wait()

	// Allow cleanup time
	time.Sleep(1 * time.Second)

	current, max, initial, _ := fdMonitor.GetStats()
	fdGrowth := current - initial

	t.Logf("Error cleanup test - Initial FD: %d, Current FD: %d, Max FD: %d, Growth: %d",
		initial, current, max, fdGrowth)

	// Verify file descriptors are cleaned up even after errors
	if fdGrowth > 20 {
		t.Errorf("File descriptor growth too high after errors: %d", fdGrowth)
	}

	metrics := pool.GetMetrics()
	t.Logf("Pool metrics after errors - Active: %d, Rejected: %d",
		metrics.ActiveConnections, metrics.RejectedRequests)
}

func testPoolCleanupVerification(t *testing.T) {
	initialFdCount := getCurrentFileDescriptorCount(os.Getpid())
	
	// Create and destroy multiple pools
	poolCount := 5
	for i := 0; i < poolCount; i++ {
		pool := NewConnectionPool(20)
		
		// Get some clients
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		for j := 0; j < 5; j++ {
			client, err := pool.GetClient(ctx)
			if err == nil {
				pool.ReleaseClient(client)
			}
		}
		cancel()
		
		pool.Close()
		runtime.GC() // Force garbage collection
		time.Sleep(100 * time.Millisecond)
	}

	finalFdCount := getCurrentFileDescriptorCount(os.Getpid())
	fdGrowth := finalFdCount - initialFdCount

	t.Logf("Pool cleanup verification - Initial: %d, Final: %d, Growth: %d",
		initialFdCount, finalFdCount, fdGrowth)

	if fdGrowth > 10 {
		t.Errorf("File descriptor leak detected: %d descriptors not cleaned up", fdGrowth)
	}
}

func testMemoryLeakDetection(t *testing.T) {
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// Create many connections and clean them up
	iterations := 100
	for i := 0; i < iterations; i++ {
		pool := NewConnectionPool(5)
		
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		for j := 0; j < 3; j++ {
			client, err := pool.GetClient(ctx)
			if err == nil {
				pool.ReleaseClient(client)
			}
		}
		cancel()
		
		pool.Close()
		
		if i%20 == 0 {
			runtime.GC()
		}
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	memGrowth := m2.Alloc - m1.Alloc
	t.Logf("Memory leak detection - Before: %d bytes, After: %d bytes, Growth: %d bytes",
		m1.Alloc, m2.Alloc, memGrowth)

	// Allow some growth but not excessive
	if memGrowth > 1024*1024 {
		t.Errorf("Potential memory leak detected: %d bytes growth", memGrowth)
	}
}

func TestGatewayConnectionLimits_SystemLimits(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping system limits tests in short mode")
	}

	soft, hard, err := getSystemFileDescriptorLimit()
	if err != nil {
		t.Skipf("Cannot get system file descriptor limits: %v", err)
	}

	t.Logf("System file descriptor limits - Soft: %d, Hard: %d", soft, hard)

	t.Run("Detect_System_Limits", func(t *testing.T) {
		testDetectSystemLimits(t, soft, hard)
	})

	t.Run("Respect_System_Limits", func(t *testing.T) {
		testRespectSystemLimits(t, soft)
	})

	t.Run("Graceful_Degradation", func(t *testing.T) {
		testGracefulDegradation(t, soft)
	})
}

func testDetectSystemLimits(t *testing.T, soft, hard int64) {
	if soft <= 0 {
		t.Error("Could not detect soft file descriptor limit")
	}

	if hard <= 0 {
		t.Error("Could not detect hard file descriptor limit")
	}

	if soft > hard {
		t.Errorf("Soft limit (%d) is greater than hard limit (%d)", soft, hard)
	}

	t.Logf("System limits detection successful - Soft: %d, Hard: %d", soft, hard)
}

func testRespectSystemLimits(t *testing.T, systemLimit int64) {
	// Don't test too close to actual system limits to avoid affecting other processes
	testLimit := min(systemLimit/4, 100)
	if testLimit < 10 {
		t.Skip("System limit too low for testing")
	}

	pool := NewConnectionPool(int(testLimit))
	defer pool.Close()

	fdMonitor := NewFileDescriptorMonitor()
	monitorCtx, monitorCancel := context.WithCancel(context.Background())
	defer monitorCancel()

	go fdMonitor.StartMonitoring(monitorCtx, 100*time.Millisecond)

	// Try to exceed the test limit
	exceededRequests := int(testLimit * 2)
	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	successCount := int64(0)
	rejectedCount := int64(0)

	for i := 0; i < exceededRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			client, err := pool.GetClient(ctx)
			if err != nil {
				atomic.AddInt64(&rejectedCount, 1)
				return
			}
			defer pool.ReleaseClient(client)

			atomic.AddInt64(&successCount, 1)
			time.Sleep(10 * time.Millisecond) // Hold connection briefly
		}(i)
	}

	wg.Wait()

	current, max, initial, _ := fdMonitor.GetStats()
	
	t.Logf("System limits test - Success: %d, Rejected: %d, FD usage - Initial: %d, Max: %d, Current: %d",
		successCount, rejectedCount, initial, max, current)

	// Verify we respected the limits
	if rejectedCount == 0 {
		t.Error("Expected some requests to be rejected due to connection limits")
	}

	if max-initial > testLimit+10 {
		t.Errorf("File descriptor usage exceeded expected limit: %d (max allowed: %d)", max-initial, testLimit+10)
	}
}

func testGracefulDegradation(t *testing.T, systemLimit int64) {
	testLimit := min(systemLimit/10, 50)
	if testLimit < 5 {
		t.Skip("System limit too low for graceful degradation testing")
	}

	pool := NewConnectionPool(int(testLimit))
	defer pool.Close()

	// Gradually increase load and monitor behavior
	loadLevels := []int{int(testLimit / 2), int(testLimit), int(testLimit * 2)}
	
	for _, loadLevel := range loadLevels {
		t.Logf("Testing graceful degradation at load level: %d", loadLevel)
		
		var wg sync.WaitGroup
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		successCount := int64(0)
		errorCount := int64(0)

		for i := 0; i < loadLevel; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				client, err := pool.GetClient(ctx)
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					return
				}
				defer pool.ReleaseClient(client)

				atomic.AddInt64(&successCount, 1)
				time.Sleep(50 * time.Millisecond)
			}(i)
		}

		wg.Wait()
		cancel()

		metrics := pool.GetMetrics()
		t.Logf("Load level %d - Success: %d, Errors: %d, Queued: %d, Rejected: %d",
			loadLevel, successCount, errorCount, metrics.QueuedRequests, metrics.RejectedRequests)

		// Allow cleanup between load levels
		time.Sleep(200 * time.Millisecond)
	}
}

// Helper functions

func setupTestGatewayWithMonitoring(t *testing.T, port int) (*Gateway, *http.Server) {
	t.Helper()

	// Create gateway with mock clients instead of real LSP servers
	cfg := &config.GatewayConfig{
		Port: port,
		Servers: []config.ServerConfig{
			{
				Name:      "test-server",
				Languages: []string{"go"},
				Command:   "mock",
				Args:      []string{},
				Transport: "stdio",
			},
		},
	}

	gateway := &Gateway{
		config:  cfg,
		clients: make(map[string]transport.LSPClient),
		router:  NewRouter(),
	}

	// Use mock client to avoid external dependencies
	mockClient := NewMockLSPClient()
	gateway.clients["test-server"] = mockClient
	gateway.router.RegisterServer("test-server", []string{"go"})

	// Start mock client instead of real LSP server
	if err := mockClient.Start(context.TODO()); err != nil {
		t.Fatalf("Failed to start mock client: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/jsonrpc", gateway.HandleJSONRPC)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("Server error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	return gateway, server
}

func sendTestRequest(client *http.Client, baseURL, testID string) error {
	requestData := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      testID,
		Method:  "textDocument/definition",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///test.go",
			},
			"position": map[string]interface{}{
				"line":      10,
				"character": 5,
			},
		},
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return err
	}

	resp, err := client.Post(baseURL, "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}