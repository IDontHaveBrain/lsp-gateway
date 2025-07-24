package platform_test

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// ResourcePool manages a pool of generic resources with limits
type ResourcePool struct {
	maxResources    int
	activeCount     int64
	waitingQueue    chan Resource
	resources       []Resource
	mu              sync.RWMutex
	metrics         *PoolMetrics
	cleanupCallback func(Resource)
	closed          bool
	closeOnce       sync.Once
}

type Resource interface {
	ID() string
	IsValid() bool
	Close() error
	LastUsed() time.Time
	MarkUsed()
}

type PoolMetrics struct {
	TotalRequests    int64
	ActiveResources  int64
	QueuedRequests   int64
	RejectedRequests int64
	PoolHits         int64
	PoolMisses       int64
	CleanupCount     int64
	MaxPoolSize      int64
	mu               sync.RWMutex
}

// FileResource implements Resource interface for file handles
type FileResource struct {
	id       string
	file     *os.File
	lastUsed time.Time
	valid    bool
}

func (fr *FileResource) ID() string {
	return fr.id
}

func (fr *FileResource) IsValid() bool {
	return fr.valid && fr.file != nil
}

func (fr *FileResource) Close() error {
	fr.valid = false
	if fr.file != nil {
		return fr.file.Close()
	}
	return nil
}

func (fr *FileResource) LastUsed() time.Time {
	return fr.lastUsed
}

func (fr *FileResource) MarkUsed() {
	fr.lastUsed = time.Now()
}

// ConnectionResource implements Resource interface for network connections
type ConnectionResource struct {
	id       string
	address  string
	lastUsed time.Time
	valid    bool
}

func (cr *ConnectionResource) ID() string {
	return cr.id
}

func (cr *ConnectionResource) IsValid() bool {
	return cr.valid
}

func (cr *ConnectionResource) Close() error {
	cr.valid = false
	return nil
}

func (cr *ConnectionResource) LastUsed() time.Time {
	return cr.lastUsed
}

func (cr *ConnectionResource) MarkUsed() {
	cr.lastUsed = time.Now()
}

func NewResourcePool(maxResources int, cleanupCallback func(Resource)) *ResourcePool {
	return &ResourcePool{
		maxResources:    maxResources,
		waitingQueue:    make(chan Resource, maxResources*2),
		resources:       make([]Resource, 0, maxResources),
		metrics:         &PoolMetrics{MaxPoolSize: int64(maxResources)},
		cleanupCallback: cleanupCallback,
	}
}

func (rp *ResourcePool) AcquireResource(ctx context.Context, factory func() (Resource, error)) (Resource, error) {
	atomic.AddInt64(&rp.metrics.TotalRequests, 1)

	// Try to get from pool first (prioritize resources slice for better reuse)
	rp.mu.Lock()
	if len(rp.resources) > 0 {
		resource := rp.resources[len(rp.resources)-1]
		rp.resources = rp.resources[:len(rp.resources)-1]
		rp.mu.Unlock()

		if resource.IsValid() {
			resource.MarkUsed()
			atomic.AddInt64(&rp.activeCount, 1)
			atomic.AddInt64(&rp.metrics.ActiveResources, 1)
			atomic.AddInt64(&rp.metrics.PoolHits, 1)
			return resource, nil
		}
		// If resource was invalid, try waiting queue next
	} else {
		rp.mu.Unlock()
	}

	// Try waiting queue for immediate availability
	select {
	case resource := <-rp.waitingQueue:
		if resource.IsValid() {
			resource.MarkUsed()
			atomic.AddInt64(&rp.activeCount, 1)
			atomic.AddInt64(&rp.metrics.ActiveResources, 1)
			atomic.AddInt64(&rp.metrics.PoolHits, 1)
			return resource, nil
		}
	default:
		// No resources immediately available in queue
	}

	// Check if we can create new resource
	if int(atomic.LoadInt64(&rp.activeCount)) >= rp.maxResources {
		atomic.AddInt64(&rp.metrics.QueuedRequests, 1)

		// Wait for resource to become available or timeout
		select {
		case resource := <-rp.waitingQueue:
			if resource.IsValid() {
				resource.MarkUsed()
				atomic.AddInt64(&rp.activeCount, 1)
				atomic.AddInt64(&rp.metrics.ActiveResources, 1)
				atomic.AddInt64(&rp.metrics.PoolHits, 1)
				return resource, nil
			}
		case <-ctx.Done():
			atomic.AddInt64(&rp.metrics.RejectedRequests, 1)
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			atomic.AddInt64(&rp.metrics.RejectedRequests, 1)
			return nil, fmt.Errorf("resource pool timeout")
		}
	}

	// Create new resource
	resource, err := factory()
	if err != nil {
		atomic.AddInt64(&rp.metrics.RejectedRequests, 1)
		return nil, err
	}

	resource.MarkUsed()
	atomic.AddInt64(&rp.activeCount, 1)
	atomic.AddInt64(&rp.metrics.ActiveResources, 1)
	atomic.AddInt64(&rp.metrics.PoolMisses, 1)
	return resource, nil
}

func (rp *ResourcePool) ReleaseResource(resource Resource) {
	if resource == nil {
		return
	}

	atomic.AddInt64(&rp.activeCount, -1)
	atomic.AddInt64(&rp.metrics.ActiveResources, -1)

	if !resource.IsValid() {
		if rp.cleanupCallback != nil {
			rp.cleanupCallback(resource)
		}
		atomic.AddInt64(&rp.metrics.CleanupCount, 1)
		return
	}

	rp.mu.Lock()
	defer rp.mu.Unlock()

	// Check if pool is closed to prevent sending on closed channel
	if rp.closed {
		if rp.cleanupCallback != nil {
			rp.cleanupCallback(resource)
		}
		atomic.AddInt64(&rp.metrics.CleanupCount, 1)
		return
	}

	// Try waiting queue first to serve waiting goroutines immediately
	select {
	case rp.waitingQueue <- resource:
		// Successfully sent to waiting goroutine
		return
	default:
		// No one waiting, try to add to resources slice for later reuse
		if len(rp.resources) < rp.maxResources {
			rp.resources = append(rp.resources, resource)
			return
		}

		// Pool is full, cleanup the resource
		if rp.cleanupCallback != nil {
			rp.cleanupCallback(resource)
		}
		atomic.AddInt64(&rp.metrics.CleanupCount, 1)
	}
}

func (rp *ResourcePool) GetMetrics() PoolMetrics {
	rp.metrics.mu.RLock()
	defer rp.metrics.mu.RUnlock()

	// Return copy without mutex to avoid copying lock value
	return PoolMetrics{
		TotalRequests:    rp.metrics.TotalRequests,
		ActiveResources:  rp.metrics.ActiveResources,
		QueuedRequests:   rp.metrics.QueuedRequests,
		RejectedRequests: rp.metrics.RejectedRequests,
		PoolHits:         rp.metrics.PoolHits,
		PoolMisses:       rp.metrics.PoolMisses,
		CleanupCount:     rp.metrics.CleanupCount,
		MaxPoolSize:      rp.metrics.MaxPoolSize,
	}
}

func (rp *ResourcePool) Cleanup() {
	rp.closeOnce.Do(func() {
		rp.mu.Lock()
		defer rp.mu.Unlock()

		// Mark as closed to prevent new releases
		rp.closed = true

		// Close waiting queue safely
		close(rp.waitingQueue)

		// Drain any resources from the waiting queue
		for resource := range rp.waitingQueue {
			if rp.cleanupCallback != nil {
				rp.cleanupCallback(resource)
			}
			atomic.AddInt64(&rp.metrics.CleanupCount, 1)
		}

		// Cleanup pooled resources
		for _, resource := range rp.resources {
			if rp.cleanupCallback != nil {
				rp.cleanupCallback(resource)
			}
			atomic.AddInt64(&rp.metrics.CleanupCount, 1)
		}
		rp.resources = nil
	})
}

func (rp *ResourcePool) Size() int {
	rp.mu.RLock()
	defer rp.mu.RUnlock()
	return len(rp.resources)
}

// SystemResourceMonitor tracks system-wide resource usage
type SystemResourceMonitor struct {
	pid               int
	initialMemory     uint64
	initialFDs        int64
	currentMemory     uint64
	currentFDs        int64
	maxMemoryObserved uint64
	maxFDsObserved    int64
	measurements      []ResourceMeasurement
	mu                sync.RWMutex
}

type ResourceMeasurement struct {
	Timestamp time.Time
	Memory    uint64
	FDs       int64
}

func NewSystemResourceMonitor() *SystemResourceMonitor {
	pid := os.Getpid()
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	initialFDs := getCurrentFileDescriptorCount(pid)

	return &SystemResourceMonitor{
		pid:               pid,
		initialMemory:     memStats.Alloc,
		initialFDs:        initialFDs,
		currentMemory:     memStats.Alloc,
		currentFDs:        initialFDs,
		maxMemoryObserved: memStats.Alloc,
		maxFDsObserved:    initialFDs,
		measurements:      make([]ResourceMeasurement, 0, 1000),
	}
}

func (srm *SystemResourceMonitor) StartMonitoring(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			srm.takeMeasurement()
		}
	}
}

func (srm *SystemResourceMonitor) takeMeasurement() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	fds := getCurrentFileDescriptorCount(srm.pid)

	srm.mu.Lock()
	defer srm.mu.Unlock()

	srm.currentMemory = memStats.Alloc
	srm.currentFDs = fds

	if memStats.Alloc > srm.maxMemoryObserved {
		srm.maxMemoryObserved = memStats.Alloc
	}

	if fds > srm.maxFDsObserved {
		srm.maxFDsObserved = fds
	}

	measurement := ResourceMeasurement{
		Timestamp: time.Now(),
		Memory:    memStats.Alloc,
		FDs:       fds,
	}

	srm.measurements = append(srm.measurements, measurement)
	if len(srm.measurements) > 1000 {
		srm.measurements = srm.measurements[1:]
	}
}

func (srm *SystemResourceMonitor) GetStats() (currentMem, maxMem, initialMem uint64, currentFDs, maxFDs, initialFDs int64) {
	srm.mu.RLock()
	defer srm.mu.RUnlock()
	return srm.currentMemory, srm.maxMemoryObserved, srm.initialMemory,
		srm.currentFDs, srm.maxFDsObserved, srm.initialFDs
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

	// For other platforms, return a mock value for testing
	return 10
}

func getSystemResourceLimits() (memoryLimit, fdLimit int64, err error) {
	// Get file descriptor limit
	var rlimit syscall.Rlimit
	err = syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit)
	if err != nil {
		return 0, 0, err
	}

	fdLimit = int64(rlimit.Cur)

	// Memory limit (simplified - would need more complex logic for real implementation)
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	memoryLimit = int64(memStats.Sys) * 2 // Allow 2x current system memory as a rough limit

	return memoryLimit, fdLimit, nil
}

func TestResourcePool_MaxPoolSize(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource pool tests in short mode")
	}

	monitor := NewSystemResourceMonitor()
	monitorCtx, monitorCancel := context.WithCancel(context.Background())
	defer monitorCancel()

	go monitor.StartMonitoring(monitorCtx, 100*time.Millisecond)

	t.Run("Pool_Size_Limits", func(t *testing.T) {
		testPoolSizeLimits(t, monitor)
	})

	t.Run("Pool_Resource_Reuse", func(t *testing.T) {
		testPoolResourceReuse(t, monitor)
	})

	t.Run("Pool_Cleanup_Efficiency", func(t *testing.T) {
		testPoolCleanupEfficiency(t, monitor)
	})
}

func testPoolSizeLimits(t *testing.T, monitor *SystemResourceMonitor) {
	maxResources := 10
	cleanupCount := int64(0)

	pool := NewResourcePool(maxResources, func(r Resource) {
		atomic.AddInt64(&cleanupCount, 1)
		_ = r.Close()
	})
	defer pool.Cleanup()

	factory := func() (Resource, error) {
		tempFile, err := os.CreateTemp("", "pool-test-*")
		if err != nil {
			return nil, err
		}

		return &FileResource{
			id:       fmt.Sprintf("file_%p", tempFile),
			file:     tempFile,
			lastUsed: time.Now(),
			valid:    true,
		}, nil
	}

	// Acquire resources up to the limit
	resources := make([]Resource, 0, maxResources*2)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Acquire more than the limit to test rejection
	for i := 0; i < maxResources*2; i++ {
		resource, err := pool.AcquireResource(ctx, factory)
		if err != nil {
			if i < maxResources {
				t.Errorf("Failed to acquire resource %d (within limit): %v", i, err)
			}
			// Expected failures beyond the limit
			continue
		}
		resources = append(resources, resource)
	}

	// Verify pool size constraints
	metrics := pool.GetMetrics()
	t.Logf("Pool limits test - Acquired: %d, Active: %d, Rejected: %d, Max: %d",
		len(resources), metrics.ActiveResources, metrics.RejectedRequests, metrics.MaxPoolSize)

	if len(resources) > maxResources+2 {
		t.Errorf("Pool allowed too many resources: %d (max: %d)", len(resources), maxResources)
	}

	if metrics.RejectedRequests == 0 {
		t.Error("Expected some requests to be rejected due to pool limits")
	}

	// Release all resources
	for _, resource := range resources {
		pool.ReleaseResource(resource)
	}

	// Allow cleanup time
	time.Sleep(200 * time.Millisecond)

	finalMetrics := pool.GetMetrics()
	t.Logf("After release - Active: %d, Cleanup count: %d, Pool size: %d",
		finalMetrics.ActiveResources, finalMetrics.CleanupCount, pool.Size())

	if finalMetrics.ActiveResources > 0 {
		t.Errorf("Some resources not properly released: %d still active", finalMetrics.ActiveResources)
	}
}

func testPoolResourceReuse(t *testing.T, monitor *SystemResourceMonitor) {
	maxResources := 5
	pool := NewResourcePool(maxResources, func(r Resource) {
		_ = r.Close()
	})
	defer pool.Cleanup()

	factory := func() (Resource, error) {
		return &ConnectionResource{
			id:       fmt.Sprintf("conn_%d", time.Now().UnixNano()),
			address:  "localhost:8080",
			lastUsed: time.Now(),
			valid:    true,
		}, nil
	}

	// Test resource reuse pattern
	reuseCycles := 3
	resourcesPerCycle := maxResources

	for cycle := 0; cycle < reuseCycles; cycle++ {
		t.Logf("Resource reuse cycle %d/%d", cycle+1, reuseCycles)

		resources := make([]Resource, 0, resourcesPerCycle)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		// Acquire resources
		for i := 0; i < resourcesPerCycle; i++ {
			resource, err := pool.AcquireResource(ctx, factory)
			if err != nil {
				t.Errorf("Cycle %d: Failed to acquire resource %d: %v", cycle, i, err)
				continue
			}
			resources = append(resources, resource)
		}

		// Use resources briefly
		for _, resource := range resources {
			resource.MarkUsed()
		}

		// Release resources
		for _, resource := range resources {
			pool.ReleaseResource(resource)
		}

		cancel()

		// Allow time for resources to be properly pooled before next cycle
		time.Sleep(200 * time.Millisecond)

		metrics := pool.GetMetrics()
		t.Logf("Cycle %d metrics - Hits: %d, Misses: %d, Pool size: %d",
			cycle, metrics.PoolHits, metrics.PoolMisses, pool.Size())

		// Brief pause between cycles
		time.Sleep(100 * time.Millisecond)
	}

	finalMetrics := pool.GetMetrics()

	// Verify reuse efficiency
	if finalMetrics.PoolHits == 0 && reuseCycles > 1 {
		t.Error("No pool hits detected - resource reuse not working")
	}

	reuseRatio := float64(finalMetrics.PoolHits) / float64(finalMetrics.TotalRequests)
	t.Logf("Resource reuse efficiency - Total requests: %d, Pool hits: %d, Reuse ratio: %.2f",
		finalMetrics.TotalRequests, finalMetrics.PoolHits, reuseRatio)

	if reuseRatio < 0.3 && reuseCycles > 1 {
		t.Errorf("Low resource reuse ratio: %.2f (expected > 0.3)", reuseRatio)
	}
}

func testPoolCleanupEfficiency(t *testing.T, monitor *SystemResourceMonitor) {
	currentMem, _, initialMem, currentFDs, _, initialFDs := monitor.GetStats()
	t.Logf("Starting cleanup test - Memory: %d bytes (delta: %d), FDs: %d (delta: %d)",
		currentMem, int64(currentMem-initialMem), currentFDs, currentFDs-initialFDs)

	cleanupRounds := 5
	resourcesPerRound := 8

	for round := 0; round < cleanupRounds; round++ {
		t.Logf("Cleanup efficiency round %d/%d", round+1, cleanupRounds)

		pool := NewResourcePool(resourcesPerRound, func(r Resource) {
			_ = r.Close()
		})

		factory := func() (Resource, error) {
			tempFile, err := os.CreateTemp("", "cleanup-test-*")
			if err != nil {
				return nil, err
			}

			// Write some data to make the file handle more "real"
			_, _ = tempFile.WriteString("test data")

			return &FileResource{
				id:       fmt.Sprintf("file_%d_%p", round, tempFile),
				file:     tempFile,
				lastUsed: time.Now(),
				valid:    true,
			}, nil
		}

		resources := make([]Resource, 0, resourcesPerRound)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		// Create and use resources
		for i := 0; i < resourcesPerRound; i++ {
			resource, err := pool.AcquireResource(ctx, factory)
			if err != nil {
				t.Errorf("Round %d: Failed to acquire resource %d: %v", round, i, err)
				continue
			}
			resources = append(resources, resource)
		}

		// Release resources
		for _, resource := range resources {
			pool.ReleaseResource(resource)
		}

		cancel()

		// Cleanup pool
		pool.Cleanup()

		// Force garbage collection multiple times for better cleanup
		for i := 0; i < 3; i++ {
			runtime.GC()
			time.Sleep(50 * time.Millisecond)
		}

		// Additional wait to ensure cleanup completion
		time.Sleep(200 * time.Millisecond)

		// Check resource usage
		roundMem, _, _, roundFDs, _, _ := monitor.GetStats()
		t.Logf("Round %d cleanup - Memory: %d bytes, FDs: %d", round, roundMem, roundFDs)
	}

	// Final check
	time.Sleep(500 * time.Millisecond)
	runtime.GC()

	finalMem, maxMem, _, finalFDs, maxFDs, _ := monitor.GetStats()
	memGrowth := int64(finalMem - initialMem)
	fdGrowth := finalFDs - initialFDs

	t.Logf("Cleanup efficiency final - Memory growth: %d bytes (max: %d), FD growth: %d (max: %d)",
		memGrowth, maxMem, fdGrowth, maxFDs)

	if memGrowth > 1024*1024 {
		t.Errorf("Excessive memory growth: %d bytes", memGrowth)
	}

	if fdGrowth > 10 {
		t.Errorf("Excessive file descriptor growth: %d", fdGrowth)
	}
}

func TestResourcePool_ExhaustionHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource exhaustion tests in short mode")
	}

	monitor := NewSystemResourceMonitor()
	monitorCtx, monitorCancel := context.WithCancel(context.Background())
	defer monitorCancel()

	go monitor.StartMonitoring(monitorCtx, 50*time.Millisecond)

	t.Run("Resource_Exhaustion_Recovery", func(t *testing.T) {
		testResourceExhaustionRecovery(t, monitor)
	})

	t.Run("Pool_Waiting_Queue", func(t *testing.T) {
		testPoolWaitingQueue(t, monitor)
	})

	t.Run("Resource_Timeout_Handling", func(t *testing.T) {
		testResourceTimeoutHandling(t, monitor)
	})
}

func testResourceExhaustionRecovery(t *testing.T, monitor *SystemResourceMonitor) {
	smallPool := 3 // Very small pool to test exhaustion
	pool := NewResourcePool(smallPool, func(r Resource) {
		_ = r.Close()
	})
	defer pool.Cleanup()

	factory := func() (Resource, error) {
		return &ConnectionResource{
			id:       fmt.Sprintf("conn_%d", time.Now().UnixNano()),
			address:  "localhost:9999",
			lastUsed: time.Now(),
			valid:    true,
		}, nil
	}

	// Exhaust the pool
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exhaustionRequests := smallPool * 3
	resources := make([]Resource, 0, exhaustionRequests)
	successes := 0
	failures := 0

	for i := 0; i < exhaustionRequests; i++ {
		resource, err := pool.AcquireResource(ctx, factory)
		if err != nil {
			failures++
		} else {
			resources = append(resources, resource)
			successes++
		}
	}

	t.Logf("Exhaustion test - Successes: %d, Failures: %d, Pool size: %d",
		successes, failures, smallPool)

	if failures == 0 {
		t.Error("Expected some failures due to pool exhaustion")
	}

	if successes == 0 {
		t.Error("Expected some successes before exhaustion")
	}

	// Release some resources
	releaseCount := len(resources) / 2
	for i := 0; i < releaseCount; i++ {
		pool.ReleaseResource(resources[i])
	}

	// Verify recovery - should be able to acquire resources again
	recoveryRequests := releaseCount
	recoverySuccesses := 0

	for i := 0; i < recoveryRequests; i++ {
		resource, err := pool.AcquireResource(ctx, factory)
		if err == nil {
			recoverySuccesses++
			resources = append(resources, resource)
		}
	}

	t.Logf("Recovery test - Recovery successes: %d out of %d attempts", recoverySuccesses, recoveryRequests)

	if recoverySuccesses == 0 {
		t.Error("Pool did not recover after releasing resources")
	}

	// Cleanup
	for _, resource := range resources {
		if resource != nil {
			pool.ReleaseResource(resource)
		}
	}
}

func testPoolWaitingQueue(t *testing.T, monitor *SystemResourceMonitor) {
	poolSize := 2
	pool := NewResourcePool(poolSize, func(r Resource) {
		r.Close()
	})
	defer pool.Cleanup()

	factory := func() (Resource, error) {
		return &ConnectionResource{
			id:       fmt.Sprintf("queue_conn_%d", time.Now().UnixNano()),
			address:  "localhost:8888",
			lastUsed: time.Now(),
			valid:    true,
		}, nil
	}

	// Fill the pool
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	heldResources := make([]Resource, poolSize)
	for i := 0; i < poolSize; i++ {
		resource, err := pool.AcquireResource(ctx, factory)
		if err != nil {
			t.Fatalf("Failed to acquire initial resource %d: %v", i, err)
		}
		heldResources[i] = resource
	}

	// Start goroutines that will wait in queue
	waitingCount := 5
	var wg sync.WaitGroup
	results := make(chan bool, waitingCount)

	// Start all waiting goroutines
	for i := 0; i < waitingCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			waitCtx, waitCancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer waitCancel()

			resource, err := pool.AcquireResource(waitCtx, factory)
			if err != nil {
				results <- false
			} else {
				results <- true
				pool.ReleaseResource(resource)
			}
		}(i)
	}

	// Give goroutines time to start waiting
	time.Sleep(500 * time.Millisecond)

	// Release resources with delays to test queue behavior
	releaseWg := sync.WaitGroup{}
	releaseWg.Add(1)
	go func() {
		defer releaseWg.Done()
		for i, resource := range heldResources {
			time.Sleep(1 * time.Second)
			t.Logf("Releasing held resource %d", i)
			pool.ReleaseResource(resource)
		}
	}()

	wg.Wait()
	releaseWg.Wait() // Ensure all releases complete before cleanup
	close(results)

	successCount := 0
	for success := range results {
		if success {
			successCount++
		}
	}

	metrics := pool.GetMetrics()
	t.Logf("Queue test - Waiting requests: %d, Successes: %d, Queued: %d",
		waitingCount, successCount, metrics.QueuedRequests)

	if successCount == 0 {
		t.Error("No waiting requests succeeded")
	}

	if metrics.QueuedRequests == 0 {
		t.Error("No requests were queued")
	}
}

func testResourceTimeoutHandling(t *testing.T, monitor *SystemResourceMonitor) {
	poolSize := 1
	pool := NewResourcePool(poolSize, func(r Resource) {
		r.Close()
	})
	defer pool.Cleanup()

	factory := func() (Resource, error) {
		return &ConnectionResource{
			id:       fmt.Sprintf("timeout_conn_%d", time.Now().UnixNano()),
			address:  "localhost:7777",
			lastUsed: time.Now(),
			valid:    true,
		}, nil
	}

	// Acquire the only resource
	longCtx, longCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer longCancel()

	heldResource, err := pool.AcquireResource(longCtx, factory)
	if err != nil {
		t.Fatalf("Failed to acquire resource: %v", err)
	}

	// Try to acquire with short timeout
	shortCtx, shortCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer shortCancel()

	start := time.Now()
	_, err = pool.AcquireResource(shortCtx, factory)
	duration := time.Since(start)

	if err == nil {
		t.Error("Expected timeout error")
	}

	if duration > 2*time.Second {
		t.Errorf("Timeout took too long: %v (expected ~500ms)", duration)
	}

	if !timeoutError(err) {
		t.Errorf("Expected timeout error, got: %v", err)
	}

	t.Logf("Timeout test - Duration: %v, Error: %v", duration, err)

	// Release the held resource
	pool.ReleaseResource(heldResource)

	// Verify pool works after timeout
	quickCtx, quickCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer quickCancel()

	resource, err := pool.AcquireResource(quickCtx, factory)
	if err != nil {
		t.Errorf("Pool not working after timeout: %v", err)
	} else {
		pool.ReleaseResource(resource)
	}

	metrics := pool.GetMetrics()
	t.Logf("Timeout handling metrics - Rejected: %d, Total: %d",
		metrics.RejectedRequests, metrics.TotalRequests)

	if metrics.RejectedRequests == 0 {
		t.Error("Expected some rejected requests due to timeout")
	}
}

func TestResourcePool_SystemLimits(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping system limits tests in short mode")
	}

	memoryLimit, fdLimit, err := getSystemResourceLimits()
	if err != nil {
		t.Skipf("Cannot get system resource limits: %v", err)
	}

	t.Logf("System limits - Memory: %d bytes, File descriptors: %d", memoryLimit, fdLimit)

	t.Run("Respect_File_Descriptor_Limits", func(t *testing.T) {
		testRespectFileDescriptorLimits(t, fdLimit)
	})

	t.Run("Pool_Size_Auto_Adjustment", func(t *testing.T) {
		testPoolSizeAutoAdjustment(t, fdLimit)
	})

	t.Run("System_Resource_Monitoring", func(t *testing.T) {
		testSystemResourceMonitoring(t, memoryLimit, fdLimit)
	})
}

func testRespectFileDescriptorLimits(t *testing.T, systemFDLimit int64) {
	// Use a conservative limit for testing
	testFDLimit := min(systemFDLimit/4, 50)
	if testFDLimit < 5 {
		t.Skip("System FD limit too low for testing")
	}

	monitor := NewSystemResourceMonitor()
	monitorCtx, monitorCancel := context.WithCancel(context.Background())
	defer monitorCancel()

	go monitor.StartMonitoring(monitorCtx, 100*time.Millisecond)

	pool := NewResourcePool(int(testFDLimit), func(r Resource) {
		r.Close()
	})
	defer pool.Cleanup()

	factory := func() (Resource, error) {
		tempFile, err := os.CreateTemp("", "fd-limit-test-*")
		if err != nil {
			return nil, err
		}

		return &FileResource{
			id:       fmt.Sprintf("fd_test_%p", tempFile),
			file:     tempFile,
			lastUsed: time.Now(),
			valid:    true,
		}, nil
	}

	// Try to exceed the test limit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resources := make([]Resource, 0, int(testFDLimit*2))
	successCount := 0
	errorCount := 0

	for i := 0; i < int(testFDLimit*2); i++ {
		resource, err := pool.AcquireResource(ctx, factory)
		if err != nil {
			errorCount++
		} else {
			resources = append(resources, resource)
			successCount++
		}
	}

	currentMem, _, _, currentFDs, maxFDs, initialFDs := monitor.GetStats()
	fdGrowth := currentFDs - initialFDs

	t.Logf("FD limits test - Success: %d, Errors: %d, FD growth: %d, Max FDs: %d, Test limit: %d",
		successCount, errorCount, fdGrowth, maxFDs, testFDLimit)

	// Release resources
	for _, resource := range resources {
		pool.ReleaseResource(resource)
	}

	// Verify we respected the limits
	if errorCount == 0 && successCount > int(testFDLimit) {
		t.Errorf("Pool didn't respect FD limits: %d successes (limit: %d)", successCount, testFDLimit)
	}

	if fdGrowth > testFDLimit+10 {
		t.Errorf("File descriptor usage exceeded expected limit: %d (max allowed: %d)", fdGrowth, testFDLimit+10)
	}

	t.Logf("Memory usage: %d bytes", currentMem)
}

func testPoolSizeAutoAdjustment(t *testing.T, systemFDLimit int64) {
	monitor := NewSystemResourceMonitor()
	_ = monitor // Prevent unused variable error

	// Test different pool sizes based on system limits
	testSizes := []int{
		int(systemFDLimit / 20),
		int(systemFDLimit / 10),
		int(systemFDLimit / 5),
	}

	for _, poolSize := range testSizes {
		if poolSize < 2 {
			continue
		}

		t.Logf("Testing pool size: %d", poolSize)

		pool := NewResourcePool(poolSize, func(r Resource) {
			r.Close()
		})

		factory := func() (Resource, error) {
			return &ConnectionResource{
				id:       fmt.Sprintf("auto_adjust_%d", time.Now().UnixNano()),
				address:  "localhost:6666",
				lastUsed: time.Now(),
				valid:    true,
			}, nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		// Test pool at its configured size
		resources := make([]Resource, 0, poolSize)
		for i := 0; i < poolSize; i++ {
			resource, err := pool.AcquireResource(ctx, factory)
			if err != nil {
				t.Errorf("Pool size %d: Failed to acquire resource %d: %v", poolSize, i, err)
				break
			}
			resources = append(resources, resource)
		}

		// Release resources
		for _, resource := range resources {
			pool.ReleaseResource(resource)
		}

		cancel()
		pool.Cleanup()

		metrics := pool.GetMetrics()
		efficiency := float64(metrics.PoolHits) / float64(metrics.TotalRequests)

		t.Logf("Pool size %d metrics - Total requests: %d, Efficiency: %.2f",
			poolSize, metrics.TotalRequests, efficiency)
	}

	// Test optimal pool size calculation
	optimalSize := calculateOptimalPoolSize(systemFDLimit)
	t.Logf("Calculated optimal pool size: %d (system FD limit: %d)", optimalSize, systemFDLimit)

	if optimalSize <= 0 || optimalSize > int(systemFDLimit/2) {
		t.Errorf("Optimal pool size out of reasonable range: %d", optimalSize)
	}
}

func testSystemResourceMonitoring(t *testing.T, memoryLimit, fdLimit int64) {
	monitor := NewSystemResourceMonitor()
	monitorCtx, monitorCancel := context.WithCancel(context.Background())
	defer monitorCancel()

	go monitor.StartMonitoring(monitorCtx, 50*time.Millisecond)

	// Perform resource-intensive operations
	poolSize := min(fdLimit/10, 20)
	pool := NewResourcePool(int(poolSize), func(r Resource) {
		r.Close()
	})
	defer pool.Cleanup()

	factory := func() (Resource, error) {
		tempFile, err := os.CreateTemp("", "monitor-test-*")
		if err != nil {
			return nil, err
		}

		// Write data to increase memory usage
		data := make([]byte, 1024)
		_, _ = tempFile.Write(data)

		return &FileResource{
			id:       fmt.Sprintf("monitor_%p", tempFile),
			file:     tempFile,
			lastUsed: time.Now(),
			valid:    true,
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create and use resources
	resources := make([]Resource, 0, int(poolSize))
	for i := 0; i < int(poolSize); i++ {
		resource, err := pool.AcquireResource(ctx, factory)
		if err != nil {
			continue
		}
		resources = append(resources, resource)
	}

	// Let monitor collect data
	time.Sleep(1 * time.Second)

	// Release resources
	for _, resource := range resources {
		pool.ReleaseResource(resource)
	}

	// Final monitoring
	time.Sleep(500 * time.Millisecond)

	currentMem, maxMem, initialMem, currentFDs, maxFDs, initialFDs := monitor.GetStats()

	t.Logf("Resource monitoring results:")
	t.Logf("  Memory - Initial: %d, Current: %d, Max: %d, Growth: %d",
		initialMem, currentMem, maxMem, currentMem-initialMem)
	t.Logf("  File Descriptors - Initial: %d, Current: %d, Max: %d, Growth: %d",
		initialFDs, currentFDs, maxFDs, currentFDs-initialFDs)

	// Verify monitoring detected resource usage
	if maxMem <= initialMem {
		t.Error("Monitor did not detect memory usage increase")
	}

	if maxFDs <= initialFDs && runtime.GOOS == "linux" {
		t.Error("Monitor did not detect file descriptor usage increase")
	}

	// Verify resources are within reasonable bounds
	memGrowth := maxMem - initialMem
	fdGrowth := maxFDs - initialFDs

	if memGrowth > uint64(memoryLimit/4) {
		t.Errorf("Excessive memory growth detected: %d bytes", memGrowth)
	}

	if fdGrowth > poolSize*2 {
		t.Errorf("Excessive file descriptor growth: %d", fdGrowth)
	}
}

// Helper functions

func timeoutError(err error) bool {
	return err != nil && (err.Error() == "context deadline exceeded" ||
		err.Error() == "resource pool timeout")
}

func calculateOptimalPoolSize(systemFDLimit int64) int {
	// Simple heuristic: use 10% of system FD limit, with min 5 and max 100
	optimal := int(systemFDLimit / 10)
	if optimal < 5 {
		optimal = 5
	}
	if optimal > 100 {
		optimal = 100
	}
	return optimal
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
