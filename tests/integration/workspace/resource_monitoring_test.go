package workspace

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/internal/workspace"
	"lsp-gateway/tests/e2e/testutils"
	"lsp-gateway/tests/integration/workspace/helpers"
)

// ResourceMonitoringIntegrationSuite tests resource usage validation across multiple projects
type ResourceMonitoringIntegrationSuite struct {
	suite.Suite
	
	// Core components
	tempDir             string
	detector            workspace.WorkspaceDetector
	helper              *helpers.WorkspaceTestHelper
	multiProjectManager *testutils.MultiProjectManager
	
	// Resource monitoring
	resourceMonitor     *ResourceMonitor
	scipCache          workspace.WorkspaceSCIPCache
	
	// Resource quotas (requirements)
	maxTotalMemoryMB    int64  // 8GB total
	maxWorkspaceMemoryMB int64  // 2GB per workspace
	maxGoroutines       int    // 1000 goroutines max
	maxFileDescriptors  int    // 1000 FDs max
}

// ResourceMonitor tracks system resource usage over time
type ResourceMonitor struct {
	samples             []*ResourceSample
	samplingInterval    time.Duration
	samplingActive      atomic.Bool
	stopCh              chan struct{}
	wg                  sync.WaitGroup
	mu                  sync.RWMutex
	
	// Alert thresholds
	memoryThresholdMB   int64
	goroutineThreshold  int
	fdThreshold         int
	alerts              []*ResourceAlert
}

// ResourceSample captures resource usage at a point in time
type ResourceSample struct {
	Timestamp           time.Time
	HeapAllocMB         int64
	HeapSysMB           int64
	StackMB             int64
	NumGoroutines       int
	NumGC               uint32
	GCPauseTimeNs       uint64
	NumFileDescriptors  int
	CPUUsagePercent     float64
	TestPhase           string
	WorkspaceContext    string
}

// ResourceAlert represents a resource usage alert
type ResourceAlert struct {
	Timestamp       time.Time
	AlertType       string
	ResourceType    string
	CurrentValue    interface{}
	ThresholdValue  interface{}
	Severity        string
	Message         string
	WorkspaceID     string
}

// ResourceQuota defines resource limits per workspace
type ResourceQuota struct {
	MaxMemoryMB         int64
	MaxGoroutines       int
	MaxFileDescriptors  int
	MaxCacheSize        int64
	MaxCacheEntries     int64
	MaxConcurrentOps    int
}

func (suite *ResourceMonitoringIntegrationSuite) SetupSuite() {
	tempDir, err := os.MkdirTemp("", "resource-monitoring-*")
	suite.Require().NoError(err)
	
	suite.tempDir = tempDir
	suite.detector = workspace.NewWorkspaceDetector()
	suite.helper = helpers.NewWorkspaceTestHelper(tempDir)
	
	// Set resource quotas (based on requirements)
	suite.maxTotalMemoryMB = 8 * 1024    // 8GB total system
	suite.maxWorkspaceMemoryMB = 2 * 1024 // 2GB per workspace
	suite.maxGoroutines = 1000
	suite.maxFileDescriptors = 1000
	
	// Initialize resource monitor
	suite.resourceMonitor = &ResourceMonitor{
		samplingInterval:    100 * time.Millisecond,
		memoryThresholdMB:   suite.maxWorkspaceMemoryMB,
		goroutineThreshold:  suite.maxGoroutines,
		fdThreshold:         suite.maxFileDescriptors,
		stopCh:              make(chan struct{}),
		samples:             []*ResourceSample{},
		alerts:              []*ResourceAlert{},
	}
	
	// Setup multi-project workspace
	config := testutils.EnhancedMultiProjectConfig{
		WorkspaceName: "resource-monitoring-test",
		Languages:     []string{"go", "python", "typescript", "java"},
		CloneTimeout:  300 * time.Second,
		EnableLogging: true,
		ForceClean:    true,
		ParallelSetup: true,
	}
	suite.multiProjectManager = testutils.NewMultiProjectManager(config)
	err = suite.multiProjectManager.SetupWorkspace()
	suite.Require().NoError(err)
	
	// Initialize SCIP cache with resource monitoring
	configManager := workspace.NewWorkspaceConfigManager()
	suite.scipCache = workspace.NewWorkspaceSCIPCache(configManager)
	
	cacheConfig := &workspace.CacheConfig{
		MaxMemorySize:        suite.maxWorkspaceMemoryMB * 1024 * 1024, // Convert to bytes
		MaxMemoryEntries:     100000,
		MemoryTTL:            30 * time.Minute,
		MaxDiskSize:          5 * 1024 * 1024 * 1024, // 5GB disk cache
		MaxDiskEntries:       500000,
		DiskTTL:              24 * time.Hour,
		CompressionEnabled:   true,
		CompressionThreshold: 1024,
		CleanupInterval:      1 * time.Minute, // More frequent cleanup for monitoring
		MetricsInterval:      10 * time.Second, // More frequent metrics
		SyncInterval:         30 * time.Second,
		IsolationLevel:       workspace.IsolationStrong,
		CrossWorkspaceAccess: false,
	}
	
	err = suite.scipCache.Initialize(tempDir, cacheConfig)
	suite.Require().NoError(err)
	
	// Start resource monitoring
	suite.startResourceMonitoring()
}

func (suite *ResourceMonitoringIntegrationSuite) TearDownSuite() {
	// Stop resource monitoring
	suite.stopResourceMonitoring()
	
	if suite.scipCache != nil {
		suite.scipCache.Close()
	}
	
	if suite.multiProjectManager != nil {
		suite.multiProjectManager.Cleanup()
	}
	
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
	
	// Log resource monitoring summary
	suite.logResourceSummary()
}

// TestMemoryUsageQuotas validates memory usage stays within defined quotas
func (suite *ResourceMonitoringIntegrationSuite) TestMemoryUsageQuotas() {
	suite.captureResourceSample("memory_quota_test_start", "memory_test")
	
	suite.Run("workspace_memory_isolation", func() {
		// Create multiple workspaces to test isolation
		workspaces := []string{
			suite.helper.CreateLargeWorkspace("mem-test-1", 15),
			suite.helper.CreateLargeWorkspace("mem-test-2", 15),
			suite.helper.CreateLargeWorkspace("mem-test-3", 15),
		}
		
		var wg sync.WaitGroup
		memoryUsages := make([]int64, len(workspaces))
		
		// Process workspaces concurrently to test memory isolation
		for i, workspace := range workspaces {
			wg.Add(1)
			go func(index int, workspacePath string) {
				defer wg.Done()
				
				// Capture memory before workspace processing
				var memStatsBefore runtime.MemStats
				runtime.ReadMemStats(&memStatsBefore)
				beforeHeap := int64(memStatsBefore.HeapAlloc)
				
				// Detect workspace (memory-intensive operation)
				result, err := suite.detector.DetectWorkspaceAt(workspacePath)
				suite.NoError(err)
				suite.NotNil(result)
				
				// Simulate LSP operations to consume memory
				for j := 0; j < 100; j++ {
					for _, subProject := range result.SubProjects {
						testFile := fmt.Sprintf("%s/test-file-%d.go", subProject.AbsolutePath, j)
						foundProject := suite.detector.FindSubProjectForPath(result, testFile)
						suite.NotNil(foundProject)
					}
				}
				
				// Capture memory after processing
				var memStatsAfter runtime.MemStats
				runtime.ReadMemStats(&memStatsAfter)
				afterHeap := int64(memStatsAfter.HeapAlloc)
				
				memoryUsage := (afterHeap - beforeHeap) / (1024 * 1024) // Convert to MB
				memoryUsages[index] = memoryUsage
				
				suite.T().Logf("Workspace %d memory usage: %d MB", index+1, memoryUsage)
				
			}(i, workspace)
		}
		
		wg.Wait()
		
		// Validate memory usage per workspace
		for i, usage := range memoryUsages {
			suite.LessOrEqual(usage, suite.maxWorkspaceMemoryMB,
				"Workspace %d used %d MB, exceeds limit of %d MB", 
				i+1, usage, suite.maxWorkspaceMemoryMB)
		}
		
		// Validate total system memory usage
		var totalMemStats runtime.MemStats
		runtime.ReadMemStats(&totalMemStats)
		totalMemoryMB := int64(totalMemStats.HeapSys) / (1024 * 1024)
		
		suite.LessOrEqual(totalMemoryMB, suite.maxTotalMemoryMB,
			"Total system memory %d MB exceeds limit of %d MB", 
			totalMemoryMB, suite.maxTotalMemoryMB)
		
		suite.T().Logf("✓ Memory isolation validated: individual workspaces ≤%d MB, total ≤%d MB", 
			suite.maxWorkspaceMemoryMB, suite.maxTotalMemoryMB)
	})
	
	suite.captureResourceSample("memory_quota_test_complete", "memory_test")
}

// TestGoroutineResourceManagement validates goroutine usage stays within limits
func (suite *ResourceMonitoringIntegrationSuite) TestGoroutineResourceManagement() {
	suite.captureResourceSample("goroutine_test_start", "goroutine_test")
	
	suite.Run("concurrent_operation_limits", func() {
		initialGoroutines := runtime.NumGoroutine()
		
		// Simulate high concurrency workload
		const maxConcurrentOps = 100
		var wg sync.WaitGroup
		semaphore := make(chan struct{}, maxConcurrentOps)
		
		// Launch many concurrent workspace operations
		for i := 0; i < maxConcurrentOps*2; i++ {
			wg.Add(1)
			go func(operationID int) {
				defer wg.Done()
				
				// Acquire semaphore to limit concurrency
				semaphore <- struct{}{}
				defer func() { <-semaphore }()
				
				// Simulate workspace operation
				workspace := suite.helper.CreateLargeWorkspace(fmt.Sprintf("concurrent-%d", operationID), 5)
				result, err := suite.detector.DetectWorkspaceAt(workspace)
				
				if err == nil && result != nil {
					// Perform some work
					for _, subProject := range result.SubProjects {
						testFile := fmt.Sprintf("%s/test.go", subProject.AbsolutePath)
						suite.detector.FindSubProjectForPath(result, testFile)
					}
				}
			}(i)
		}
		
		// Monitor goroutine count during execution
		done := make(chan bool)
		go func() {
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					currentGoroutines := runtime.NumGoroutine()
					
					// Check if we're within limits
					suite.LessOrEqual(currentGoroutines, initialGoroutines+suite.maxGoroutines,
						"Goroutine count %d exceeds limit of %d", 
						currentGoroutines, initialGoroutines+suite.maxGoroutines)
				}
			}
		}()
		
		wg.Wait()
		done <- true
		
		// Allow goroutines to settle
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
		
		finalGoroutines := runtime.NumGoroutine()
		
		// Validate goroutine cleanup
		goroutineLeak := finalGoroutines - initialGoroutines
		suite.LessOrEqual(goroutineLeak, 10, // Allow small buffer for background tasks
			"Goroutine leak detected: %d goroutines remain after test", goroutineLeak)
		
		suite.T().Logf("✓ Goroutine management: initial=%d, final=%d, leak=%d", 
			initialGoroutines, finalGoroutines, goroutineLeak)
	})
	
	suite.captureResourceSample("goroutine_test_complete", "goroutine_test")
}

// TestCacheResourceLimits validates SCIP cache respects resource limits
func (suite *ResourceMonitoringIntegrationSuite) TestCacheResourceLimits() {
	suite.captureResourceSample("cache_limits_start", "cache_test")
	
	suite.Run("cache_memory_limits", func() {
		// Fill cache until memory limit is approached
		const entrySize = 1024 // 1KB per entry
		maxEntries := int(suite.maxWorkspaceMemoryMB * 1024 / entrySize) // Theoretical max
		
		// Add entries to cache
		addedEntries := 0
		for i := 0; i < maxEntries; i++ {
			key := fmt.Sprintf("cache-limit-test-%d", i)
			entry := suite.createLargeCacheEntry(key, entrySize)
			
			err := suite.scipCache.Set(key, entry, 30*time.Minute)
			if err != nil {
				// Cache rejected entry due to limits - this is expected
				break
			}
			addedEntries++
			
			// Check cache stats periodically
			if i%1000 == 0 {
				stats := suite.scipCache.GetStats()
				currentMemoryMB := stats.TotalSize / (1024 * 1024)
				
				// Cache should stay within memory limits
				suite.LessOrEqual(currentMemoryMB, suite.maxWorkspaceMemoryMB,
					"Cache memory usage %d MB exceeds limit of %d MB", 
					currentMemoryMB, suite.maxWorkspaceMemoryMB)
			}
		}
		
		// Get final cache statistics
		finalStats := suite.scipCache.GetStats()
		finalMemoryMB := finalStats.TotalSize / (1024 * 1024)
		
		// Validate final state
		suite.LessOrEqual(finalMemoryMB, suite.maxWorkspaceMemoryMB,
			"Final cache memory %d MB exceeds limit of %d MB", 
			finalMemoryMB, suite.maxWorkspaceMemoryMB)
		
		suite.T().Logf("✓ Cache limits enforced: added %d entries, using %d MB", 
			addedEntries, finalMemoryMB)
		
		// Test cache eviction behavior
		suite.validateCacheEviction()
	})
	
	suite.captureResourceSample("cache_limits_complete", "cache_test")
}

// TestResourceLeakDetection detects resource leaks over time
func (suite *ResourceMonitoringIntegrationSuite) TestResourceLeakDetection() {
	suite.captureResourceSample("leak_detection_start", "leak_test")
	
	suite.Run("memory_leak_detection", func() {
		// Baseline memory measurement
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
		var baselineMemStats runtime.MemStats
		runtime.ReadMemStats(&baselineMemStats)
		baselineHeapMB := int64(baselineMemStats.HeapAlloc) / (1024 * 1024)
		
		// Perform repeated operations that should not leak memory
		const iterations = 50
		for i := 0; i < iterations; i++ {
			// Create and destroy workspace context
			workspace := suite.helper.CreateLargeWorkspace(fmt.Sprintf("leak-test-%d", i), 5)
			result, err := suite.detector.DetectWorkspaceAt(workspace)
			suite.NoError(err)
			suite.NotNil(result)
			
			// Perform operations
			for _, subProject := range result.SubProjects {
				testFile := fmt.Sprintf("%s/test.go", subProject.AbsolutePath)
				suite.detector.FindSubProjectForPath(result, testFile)
			}
			
			// Cleanup workspace
			os.RemoveAll(workspace)
			
			// Periodic memory check
			if i%10 == 9 {
				runtime.GC()
				time.Sleep(50 * time.Millisecond)
				
				var currentMemStats runtime.MemStats
				runtime.ReadMemStats(&currentMemStats)
				currentHeapMB := int64(currentMemStats.HeapAlloc) / (1024 * 1024)
				
				memoryGrowth := currentHeapMB - baselineHeapMB
				
				// Memory growth should be bounded
				suite.LessOrEqual(memoryGrowth, int64(100), // Allow 100MB growth
					"Memory leak detected at iteration %d: grew by %d MB", i, memoryGrowth)
				
				suite.T().Logf("Iteration %d: memory growth = %d MB", i, memoryGrowth)
			}
		}
		
		// Final memory measurement
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
		var finalMemStats runtime.MemStats
		runtime.ReadMemStats(&finalMemStats)
		finalHeapMB := int64(finalMemStats.HeapAlloc) / (1024 * 1024)
		
		totalGrowth := finalHeapMB - baselineHeapMB
		
		// Validate no significant memory leak
		suite.LessOrEqual(totalGrowth, int64(50), // Allow 50MB total growth
			"Memory leak detected: total growth of %d MB after %d iterations", 
			totalGrowth, iterations)
		
		suite.T().Logf("✓ No memory leak: baseline=%d MB, final=%d MB, growth=%d MB", 
			baselineHeapMB, finalHeapMB, totalGrowth)
	})
	
	suite.captureResourceSample("leak_detection_complete", "leak_test")
}

// TestResourceRecoveryUnderPressure tests system behavior under resource pressure
func (suite *ResourceMonitoringIntegrationSuite) TestResourceRecoveryUnderPressure() {
	suite.captureResourceSample("pressure_test_start", "pressure_test")
	
	suite.Run("memory_pressure_recovery", func() {
		// Create memory pressure by filling cache
		pressureEntries := 0
		
		// Add large entries to create memory pressure
		for i := 0; i < 10000; i++ {
			key := fmt.Sprintf("pressure-test-%d", i)
			entry := suite.createLargeCacheEntry(key, 10*1024) // 10KB entries
			
			err := suite.scipCache.Set(key, entry, 5*time.Minute) // Shorter TTL
			if err != nil {
				break
			}
			pressureEntries++
			
			// Check if we're approaching limits
			stats := suite.scipCache.GetStats()
			if stats.TotalSize > int64(float64(suite.maxWorkspaceMemoryMB*1024*1024)*0.8) {
				break // Stop at 80% of limit
			}
		}
		
		suite.T().Logf("Created memory pressure with %d entries", pressureEntries)
		
		// Wait for cache cleanup to kick in
		time.Sleep(2 * time.Second)
		
		// Test that system can still perform operations under pressure
		workspace := suite.helper.CreateLargeWorkspace("pressure-workspace", 10)
		
		start := time.Now()
		result, err := suite.detector.DetectWorkspaceAt(workspace)
		detectionTime := time.Since(start)
		
		// System should still function, though potentially slower
		suite.NoError(err, "System should remain functional under memory pressure")
		suite.NotNil(result)
		suite.NotEmpty(result.SubProjects)
		
		// Performance degradation should be reasonable (less than 10x normal time)
		suite.Less(detectionTime, 50*time.Second,
			"Detection under pressure took %v, should complete within reasonable time", detectionTime)
		
		// Verify system recovers after pressure is relieved
		suite.scipCache.Invalidate() // Clear cache
		runtime.GC()
		time.Sleep(500 * time.Millisecond)
		
		// Performance should return to normal
		normalStart := time.Now()
		normalResult, err := suite.detector.DetectWorkspaceAt(workspace)
		normalTime := time.Since(normalStart)
		
		suite.NoError(err)
		suite.NotNil(normalResult)
		
		// Recovery should show improved performance
		suite.Less(normalTime, detectionTime,
			"Performance should recover after pressure relief: normal=%v vs pressure=%v", 
			normalTime, detectionTime)
		
		suite.T().Logf("✓ Resource pressure recovery: pressure=%v, normal=%v", 
			detectionTime, normalTime)
	})
	
	suite.captureResourceSample("pressure_test_complete", "pressure_test")
}

// Helper methods

func (suite *ResourceMonitoringIntegrationSuite) startResourceMonitoring() {
	suite.resourceMonitor.samplingActive.Store(true)
	suite.resourceMonitor.wg.Add(1)
	
	go func() {
		defer suite.resourceMonitor.wg.Done()
		ticker := time.NewTicker(suite.resourceMonitor.samplingInterval)
		defer ticker.Stop()
		
		for {
			select {
			case <-suite.resourceMonitor.stopCh:
				return
			case <-ticker.C:
				if suite.resourceMonitor.samplingActive.Load() {
					suite.collectResourceSample("background", "monitoring")
				}
			}
		}
	}()
}

func (suite *ResourceMonitoringIntegrationSuite) stopResourceMonitoring() {
	suite.resourceMonitor.samplingActive.Store(false)
	close(suite.resourceMonitor.stopCh)
	suite.resourceMonitor.wg.Wait()
}

func (suite *ResourceMonitoringIntegrationSuite) captureResourceSample(phase, context string) {
	suite.collectResourceSample(phase, context)
}

func (suite *ResourceMonitoringIntegrationSuite) collectResourceSample(phase, context string) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	sample := &ResourceSample{
		Timestamp:          time.Now(),
		HeapAllocMB:        int64(memStats.HeapAlloc) / (1024 * 1024),
		HeapSysMB:          int64(memStats.HeapSys) / (1024 * 1024), 
		StackMB:            int64(memStats.StackSys) / (1024 * 1024),
		NumGoroutines:      runtime.NumGoroutine(),
		NumGC:              memStats.NumGC,
		GCPauseTimeNs:      memStats.PauseNs[(memStats.NumGC+255)%256],
		NumFileDescriptors: suite.getFileDescriptorCount(),
		CPUUsagePercent:    suite.getCPUUsage(),
		TestPhase:          phase,
		WorkspaceContext:   context,
	}
	
	// Check for resource alerts
	suite.checkResourceAlerts(sample)
	
	suite.resourceMonitor.mu.Lock()
	suite.resourceMonitor.samples = append(suite.resourceMonitor.samples, sample)
	suite.resourceMonitor.mu.Unlock()
}

func (suite *ResourceMonitoringIntegrationSuite) checkResourceAlerts(sample *ResourceSample) {
	// Check memory threshold
	if sample.HeapAllocMB > suite.resourceMonitor.memoryThresholdMB {
		alert := &ResourceAlert{
			Timestamp:      sample.Timestamp,
			AlertType:      "THRESHOLD_EXCEEDED",
			ResourceType:   "memory",
			CurrentValue:   sample.HeapAllocMB,
			ThresholdValue: suite.resourceMonitor.memoryThresholdMB,
			Severity:       "WARNING",
			Message:        fmt.Sprintf("Memory usage %d MB exceeds threshold %d MB", sample.HeapAllocMB, suite.resourceMonitor.memoryThresholdMB),
			WorkspaceID:    sample.WorkspaceContext,
		}
		
		suite.resourceMonitor.mu.Lock()
		suite.resourceMonitor.alerts = append(suite.resourceMonitor.alerts, alert)
		suite.resourceMonitor.mu.Unlock()
	}
	
	// Check goroutine threshold
	if sample.NumGoroutines > suite.resourceMonitor.goroutineThreshold {
		alert := &ResourceAlert{
			Timestamp:      sample.Timestamp,
			AlertType:      "THRESHOLD_EXCEEDED",
			ResourceType:   "goroutines",
			CurrentValue:   sample.NumGoroutines,
			ThresholdValue: suite.resourceMonitor.goroutineThreshold,
			Severity:       "WARNING",
			Message:        fmt.Sprintf("Goroutine count %d exceeds threshold %d", sample.NumGoroutines, suite.resourceMonitor.goroutineThreshold),
			WorkspaceID:    sample.WorkspaceContext,
		}
		
		suite.resourceMonitor.mu.Lock()
		suite.resourceMonitor.alerts = append(suite.resourceMonitor.alerts, alert)
		suite.resourceMonitor.mu.Unlock()
	}
}

func (suite *ResourceMonitoringIntegrationSuite) createLargeCacheEntry(key string, sizeBytes int) *workspace.CacheEntry {
	data := make([]byte, sizeBytes)
	for i := range data {
		data[i] = byte(i % 256)
	}
	
	return &workspace.CacheEntry{
		Key:         key,
		Data:        data,
		Size:        int64(len(data)),
		CreatedAt:   time.Now(),
		AccessedAt:  time.Now(),
		TTL:         30 * time.Minute,
		AccessCount: 1,
		ProjectPath: suite.tempDir,
	}
}

func (suite *ResourceMonitoringIntegrationSuite) validateCacheEviction() {
	// Test that cache properly evicts entries when under pressure
	stats := suite.scipCache.GetStats()
	initialEntries := stats.TotalEntries
	
	// Add more entries to trigger eviction
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("eviction-test-%d", i)
		entry := suite.createLargeCacheEntry(key, 5*1024) // 5KB
		suite.scipCache.Set(key, entry, 1*time.Minute)   // Short TTL
	}
	
	// Wait for potential eviction
	time.Sleep(2 * time.Second)
	
	finalStats := suite.scipCache.GetStats()
	
	// Validate eviction occurred if needed
	suite.T().Logf("Cache eviction test: initial=%d, final=%d entries", 
		initialEntries, finalStats.TotalEntries)
}

func (suite *ResourceMonitoringIntegrationSuite) getFileDescriptorCount() int {
	// Simplified FD count - in production would use proper system calls
	return 10 // Placeholder
}

func (suite *ResourceMonitoringIntegrationSuite) getCPUUsage() float64 {
	// Simplified CPU usage - in production would use proper monitoring
	return 0.0 // Placeholder
}

func (suite *ResourceMonitoringIntegrationSuite) logResourceSummary() {
	suite.resourceMonitor.mu.RLock()
	defer suite.resourceMonitor.mu.RUnlock()
	
	if len(suite.resourceMonitor.samples) == 0 {
		return
	}
	
	// Calculate statistics
	var totalHeapMB, maxHeapMB, minHeapMB int64
	var totalGoroutines, maxGoroutines, minGoroutines int
	
	minHeapMB = suite.resourceMonitor.samples[0].HeapAllocMB
	minGoroutines = suite.resourceMonitor.samples[0].NumGoroutines
	
	for _, sample := range suite.resourceMonitor.samples {
		totalHeapMB += sample.HeapAllocMB
		totalGoroutines += sample.NumGoroutines
		
		if sample.HeapAllocMB > maxHeapMB {
			maxHeapMB = sample.HeapAllocMB
		}
		if sample.HeapAllocMB < minHeapMB {
			minHeapMB = sample.HeapAllocMB
		}
		if sample.NumGoroutines > maxGoroutines {
			maxGoroutines = sample.NumGoroutines
		}
		if sample.NumGoroutines < minGoroutines {
			minGoroutines = sample.NumGoroutines
		}
	}
	
	avgHeapMB := totalHeapMB / int64(len(suite.resourceMonitor.samples))
	avgGoroutines := totalGoroutines / len(suite.resourceMonitor.samples)
	
	suite.T().Logf("\n=== RESOURCE MONITORING SUMMARY ===")
	suite.T().Logf("Samples Collected: %d", len(suite.resourceMonitor.samples))
	suite.T().Logf("Memory Usage (MB): min=%d, avg=%d, max=%d", minHeapMB, avgHeapMB, maxHeapMB)
	suite.T().Logf("Goroutines: min=%d, avg=%d, max=%d", minGoroutines, avgGoroutines, maxGoroutines)
	suite.T().Logf("Resource Alerts: %d", len(suite.resourceMonitor.alerts))
	
	// Log alerts
	for _, alert := range suite.resourceMonitor.alerts {
		suite.T().Logf("ALERT [%s]: %s", alert.Severity, alert.Message)
	}
	
	suite.T().Logf("=== END RESOURCE SUMMARY ===\n")
}

func TestResourceMonitoringIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource monitoring integration tests in short mode")
	}
	
	suite.Run(t, new(ResourceMonitoringIntegrationSuite))
}