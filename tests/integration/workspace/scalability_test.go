package workspace

import (
	"context"
	"fmt"
	"math"
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

// ScalabilityIntegrationSuite validates scalability with increasing numbers of sub-projects
type ScalabilityIntegrationSuite struct {
	suite.Suite
	
	// Core components
	tempDir             string
	detector            workspace.WorkspaceDetector
	helper              *helpers.WorkspaceTestHelper
	multiProjectManager *testutils.MultiProjectManager
	
	// Scalability tracking
	scalabilityMetrics  *ScalabilityMetrics
	scipCache          workspace.WorkspaceSCIPCache
	
	// Scalability limits (requirements)
	maxSubProjects      int           // 50 sub-projects per workspace
	maxDetectionTime    time.Duration // 5 seconds
	maxRoutingTime      time.Duration // 1ms per routing operation
	minThroughput       float64       // Minimum requests per second
}

// ScalabilityMetrics tracks performance across different scales
type ScalabilityMetrics struct {
	dataPoints          []*ScalabilityDataPoint
	throughputTests     []*ThroughputDataPoint
	degradationTests    []*DegradationDataPoint
	concurrencyTests    []*ConcurrencyDataPoint
	mu                  sync.RWMutex
}

// ScalabilityDataPoint captures performance at a specific scale
type ScalabilityDataPoint struct {
	SubProjectCount     int
	DetectionTime       time.Duration
	MemoryUsageMB       int64
	RoutingTimePerOp    time.Duration
	CacheHitRate        float64
	ThroughputReqPerSec float64
	ErrorRate           float64
	Timestamp           time.Time
}

// ThroughputDataPoint captures throughput at different scales
type ThroughputDataPoint struct {
	SubProjectCount     int
	ConcurrentUsers     int
	RequestsPerUser     int
	TotalRequests       int
	Duration            time.Duration
	ThroughputReqPerSec float64
	AverageLatency      time.Duration
	P95Latency          time.Duration
	P99Latency          time.Duration
	ErrorRate           float64
	MemoryPeakMB        int64
}

// DegradationDataPoint captures performance degradation patterns
type DegradationDataPoint struct {
	BaselineSubProjects int
	ScaledSubProjects   int
	BaselinePerformance time.Duration
	ScaledPerformance   time.Duration
	DegradationFactor   float64
	DegradationType     string
	AcceptableDegradation bool
}

// ConcurrencyDataPoint captures concurrent access performance
type ConcurrencyDataPoint struct {
	SubProjectCount     int
	ConcurrentLevel     int
	OperationsPerThread int
	TotalOperations     int
	Duration            time.Duration
	SuccessfulOps       int64
	FailedOps           int64
	AverageLatency      time.Duration
	ContentionEvents    int64
}

func (suite *ScalabilityIntegrationSuite) SetupSuite() {
	tempDir, err := os.MkdirTemp("", "scalability-integration-*")
	suite.Require().NoError(err)
	
	suite.tempDir = tempDir
	suite.detector = workspace.NewWorkspaceDetector()
	suite.helper = helpers.NewWorkspaceTestHelper(tempDir)
	
	// Set scalability limits (requirements)
	suite.maxSubProjects = 50
	suite.maxDetectionTime = 5 * time.Second
	suite.maxRoutingTime = time.Millisecond
	suite.minThroughput = 100.0 // 100 req/s minimum
	
	// Initialize scalability metrics
	suite.scalabilityMetrics = &ScalabilityMetrics{
		dataPoints:       []*ScalabilityDataPoint{},
		throughputTests:  []*ThroughputDataPoint{},
		degradationTests: []*DegradationDataPoint{},
		concurrencyTests: []*ConcurrencyDataPoint{},
	}
	
	// Setup multi-language workspace for realistic testing
	config := testutils.EnhancedMultiProjectConfig{
		WorkspaceName: "scalability-test-workspace",
		Languages:     []string{"go", "python", "typescript", "java"},
		CloneTimeout:  300 * time.Second,
		EnableLogging: false, // Reduce noise for scalability tests
		ForceClean:    true,
		ParallelSetup: true,
	}
	suite.multiProjectManager = testutils.NewMultiProjectManager(config)
	err = suite.multiProjectManager.SetupWorkspace()
	suite.Require().NoError(err)
	
	// Initialize SCIP cache for scalability testing
	configManager := workspace.NewWorkspaceConfigManager()
	suite.scipCache = workspace.NewWorkspaceSCIPCache(configManager)
	
	cacheConfig := &workspace.CacheConfig{
		MaxMemorySize:        8 * 1024 * 1024 * 1024, // 8GB for scalability testing
		MaxMemoryEntries:     1000000,                 // 1M entries
		MemoryTTL:            60 * time.Minute,
		MaxDiskSize:          20 * 1024 * 1024 * 1024, // 20GB disk cache
		MaxDiskEntries:       5000000,                  // 5M entries
		DiskTTL:              24 * time.Hour,
		CompressionEnabled:   true,
		CompressionThreshold: 1024,
		CleanupInterval:      10 * time.Minute,
		MetricsInterval:      30 * time.Second,
		SyncInterval:         60 * time.Second,
		IsolationLevel:       workspace.IsolationStrong,
		CrossWorkspaceAccess: false,
	}
	
	err = suite.scipCache.Initialize(tempDir, cacheConfig)
	suite.Require().NoError(err)
}

func (suite *ScalabilityIntegrationSuite) TearDownSuite() {
	if suite.scipCache != nil {
		suite.scipCache.Close()
	}
	
	if suite.multiProjectManager != nil {
		suite.multiProjectManager.Cleanup()
	}
	
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
	
	// Log scalability analysis
	suite.logScalabilityAnalysis()
}

// TestSubProjectScalability validates performance across different project counts
func (suite *ScalabilityIntegrationSuite) TestSubProjectScalability() {
	suite.Run("project_count_scaling", func() {
		// Test scales: 5, 10, 15, 25, 35, 50 sub-projects
		scales := []int{5, 10, 15, 25, 35, 50}
		
		for _, scale := range scales {
			suite.T().Run(fmt.Sprintf("scale_%d_projects", scale), func(t *testing.T) {
				// Create workspace with specified scale
				workspacePath := suite.helper.CreateLargeWorkspace(fmt.Sprintf("scale-%d", scale), scale)
				
				// Measure detection performance
				start := time.Now()
				var memStatsBefore runtime.MemStats
				runtime.ReadMemStats(&memStatsBefore)
				
				result, err := suite.detector.DetectWorkspaceAt(workspacePath)
				detectionTime := time.Since(start)
				
				var memStatsAfter runtime.MemStats
				runtime.ReadMemStats(&memStatsAfter)
				memoryUsedMB := int64(memStatsAfter.HeapAlloc-memStatsBefore.HeapAlloc) / (1024 * 1024)
				
				suite.NoError(err, "Detection should succeed at scale %d", scale)
				suite.NotNil(result)
				
				// Validate scalability requirements
				suite.Less(detectionTime, suite.maxDetectionTime,
					"Detection time %v exceeds limit %v for %d projects", 
					detectionTime, suite.maxDetectionTime, scale)
				
				// Validate project detection accuracy
				expectedMinProjects := int(float64(scale) * 0.8) // At least 80% detected
				suite.GreaterOrEqual(len(result.SubProjects), expectedMinProjects,
					"Detected %d projects, expected at least %d for scale %d", 
					len(result.SubProjects), expectedMinProjects, scale)
				
				// Test routing performance at scale
				routingTime := suite.measureRoutingPerformanceAtScale(result, 1000)
				suite.Less(routingTime, suite.maxRoutingTime,
					"Routing time %v exceeds limit %v for %d projects", 
					routingTime, suite.maxRoutingTime, scale)
				
				// Measure cache performance if available
				var cacheHitRate float64
				var throughput float64
				if scale <= 25 { // Only test throughput for smaller scales to avoid timeout
					throughput = suite.measureThroughputAtScale(result, 100, 10)
				}
				
				// Record scalability data point
				dataPoint := &ScalabilityDataPoint{
					SubProjectCount:     scale,
					DetectionTime:       detectionTime,
					MemoryUsageMB:       memoryUsedMB,
					RoutingTimePerOp:    routingTime,
					CacheHitRate:        cacheHitRate,
					ThroughputReqPerSec: throughput,
					ErrorRate:           0.0, // Would calculate from actual errors
					Timestamp:           time.Now(),
				}
				
				suite.scalabilityMetrics.mu.Lock()
				suite.scalabilityMetrics.dataPoints = append(suite.scalabilityMetrics.dataPoints, dataPoint)
				suite.scalabilityMetrics.mu.Unlock()
				
				t.Logf("✓ Scale %d: detection=%v, memory=%dMB, routing=%v, throughput=%.1f req/s", 
					scale, detectionTime, memoryUsedMB, routingTime, throughput)
			})
		}
	})
}

// TestConcurrentScalability validates performance with concurrent access at scale
func (suite *ScalabilityIntegrationSuite) TestConcurrentScalability() {
	suite.Run("concurrent_access_scaling", func() {
		// Test concurrent access at different scales
		concurrencyLevels := []struct {
			subProjects   int
			concurrentOps int
			opsPerThread  int
		}{
			{10, 5, 20},   // Light concurrency
			{25, 10, 15},  // Medium concurrency  
			{50, 20, 10},  // High concurrency
		}
		
		for _, level := range concurrencyLevels {
			suite.T().Run(fmt.Sprintf("concurrent_%dp_%dc", level.subProjects, level.concurrentOps), func(t *testing.T) {
				// Create workspace at scale
				workspacePath := suite.helper.CreateLargeWorkspace(
					fmt.Sprintf("concurrent-%d-%d", level.subProjects, level.concurrentOps), 
					level.subProjects)
				
				result, err := suite.detector.DetectWorkspaceAt(workspacePath)
				suite.NoError(err)
				suite.NotNil(result)
				
				// Run concurrent operations
				dataPoint := suite.runConcurrentTest(result, level.concurrentOps, level.opsPerThread)
				
				// Validate concurrent performance
				averageLatencyLimit := 100 * time.Millisecond // Should handle ops quickly even under concurrency
				suite.Less(dataPoint.AverageLatency, averageLatencyLimit,
					"Average latency %v exceeds limit %v for %d concurrent ops on %d projects", 
					dataPoint.AverageLatency, averageLatencyLimit, level.concurrentOps, level.subProjects)
				
				// Error rate should be low even under concurrency
				errorRate := float64(dataPoint.FailedOps) / float64(dataPoint.TotalOperations)
				suite.Less(errorRate, 0.05, // Less than 5% errors
					"Error rate %.2f%% too high for concurrent access", errorRate*100)
				
				suite.scalabilityMetrics.mu.Lock()
				suite.scalabilityMetrics.concurrencyTests = append(suite.scalabilityMetrics.concurrencyTests, dataPoint)
				suite.scalabilityMetrics.mu.Unlock()
				
				t.Logf("✓ Concurrent %dp/%dc: latency=%v, success=%d, failed=%d", 
					level.subProjects, level.concurrentOps, dataPoint.AverageLatency, 
					dataPoint.SuccessfulOps, dataPoint.FailedOps)
			})
		}
	})
}

// TestThroughputScaling validates throughput maintains acceptable levels at scale
func (suite *ScalabilityIntegrationSuite) TestThroughputScaling() {
	suite.Run("throughput_scaling", func() {
		// Test throughput at different scales with varying user loads
		throughputTests := []struct {
			subProjects     int
			concurrentUsers int
			requestsPerUser int
			minThroughput   float64
		}{
			{10, 5, 50, 150.0},   // Small scale - high throughput expected
			{25, 10, 30, 100.0},  // Medium scale - good throughput expected
			{50, 15, 20, 50.0},   // Large scale - acceptable throughput expected
		}
		
		for _, test := range throughputTests {
			suite.T().Run(fmt.Sprintf("throughput_%dp_%du", test.subProjects, test.concurrentUsers), func(t *testing.T) {
				// Create workspace
				workspaceName := fmt.Sprintf("throughput-%d-%d", test.subProjects, test.concurrentUsers)
				workspacePath := suite.helper.CreateLargeWorkspace(workspaceName, test.subProjects)
				
				result, err := suite.detector.DetectWorkspaceAt(workspacePath)
				suite.NoError(err)
				suite.NotNil(result)
				
				// Run throughput test
				dataPoint := suite.runThroughputTest(result, test.concurrentUsers, test.requestsPerUser)
				
				// Validate throughput requirements
				suite.GreaterOrEqual(dataPoint.ThroughputReqPerSec, test.minThroughput,
					"Throughput %.1f req/s below minimum %.1f for %d projects with %d users", 
					dataPoint.ThroughputReqPerSec, test.minThroughput, test.subProjects, test.concurrentUsers)
				
				// Validate latency requirements
				maxAcceptableLatency := 1 * time.Second
				suite.Less(dataPoint.P95Latency, maxAcceptableLatency,
					"P95 latency %v exceeds limit %v", dataPoint.P95Latency, maxAcceptableLatency)
				
				// Memory usage should be reasonable
				maxMemoryMB := int64(4 * 1024) // 4GB max
				suite.LessOrEqual(dataPoint.MemoryPeakMB, maxMemoryMB,
					"Memory usage %d MB exceeds limit %d MB", dataPoint.MemoryPeakMB, maxMemoryMB)
				
				suite.scalabilityMetrics.mu.Lock()
				suite.scalabilityMetrics.throughputTests = append(suite.scalabilityMetrics.throughputTests, dataPoint)
				suite.scalabilityMetrics.mu.Unlock()
				
				t.Logf("✓ Throughput %dp/%du: %.1f req/s, P95=%v, mem=%dMB", 
					test.subProjects, test.concurrentUsers, dataPoint.ThroughputReqPerSec, 
					dataPoint.P95Latency, dataPoint.MemoryPeakMB)
			})
		}
	})
}

// TestPerformanceDegradation validates graceful performance degradation at scale
func (suite *ScalabilityIntegrationSuite) TestPerformanceDegradation() {
	suite.Run("performance_degradation_analysis", func() {
		// Test degradation patterns
		degradationTests := []struct {
			baselineProjects int
			scaledProjects   int
			operationType    string
			maxDegradation   float64 // Maximum acceptable degradation factor
		}{
			{10, 25, "detection", 2.5},    // 2.5x degradation acceptable for 2.5x scale
			{10, 50, "detection", 5.0},    // 5x degradation acceptable for 5x scale
			{10, 25, "routing", 1.5},      // Routing should scale better - 1.5x degradation max
			{10, 50, "routing", 2.0},      // Even at 5x scale, routing should be <2x slower
		}
		
		for _, test := range degradationTests {
			suite.T().Run(fmt.Sprintf("degradation_%s_%dto%d", test.operationType, test.baselineProjects, test.scaledProjects), func(t *testing.T) {
				// Create baseline workspace
				baselineWorkspace := suite.helper.CreateLargeWorkspace(
					fmt.Sprintf("baseline-%d", test.baselineProjects), test.baselineProjects)
				
				// Create scaled workspace  
				scaledWorkspace := suite.helper.CreateLargeWorkspace(
					fmt.Sprintf("scaled-%d", test.scaledProjects), test.scaledProjects)
				
				// Measure baseline performance
				baselinePerf := suite.measureOperationPerformance(baselineWorkspace, test.operationType)
				
				// Measure scaled performance
				scaledPerf := suite.measureOperationPerformance(scaledWorkspace, test.operationType)
				
				// Calculate degradation factor
				degradationFactor := float64(scaledPerf) / float64(baselinePerf)
				acceptable := degradationFactor <= test.maxDegradation
				
				// Record degradation data
				dataPoint := &DegradationDataPoint{
					BaselineSubProjects:   test.baselineProjects,
					ScaledSubProjects:     test.scaledProjects,
					BaselinePerformance:   baselinePerf,
					ScaledPerformance:     scaledPerf,
					DegradationFactor:     degradationFactor,
					DegradationType:       test.operationType,
					AcceptableDegradation: acceptable,
				}
				
				suite.scalabilityMetrics.mu.Lock()
				suite.scalabilityMetrics.degradationTests = append(suite.scalabilityMetrics.degradationTests, dataPoint)
				suite.scalabilityMetrics.mu.Unlock()
				
				// Validate degradation is acceptable
				suite.LessOrEqual(degradationFactor, test.maxDegradation,
					"%s degradation factor %.2fx exceeds limit %.2fx (%dp->%dp: %v->%v)", 
					test.operationType, degradationFactor, test.maxDegradation,
					test.baselineProjects, test.scaledProjects, baselinePerf, scaledPerf)
				
				t.Logf("✓ %s degradation %dp->%dp: %.2fx (baseline=%v, scaled=%v, limit=%.2fx)", 
					test.operationType, test.baselineProjects, test.scaledProjects, 
					degradationFactor, baselinePerf, scaledPerf, test.maxDegradation)
			})
		}
	})
}

// TestScalabilityLimitValidation validates system behavior at maximum scale
func (suite *ScalabilityIntegrationSuite) TestScalabilityLimitValidation() {
	suite.Run("maximum_scale_validation", func() {
		// Test at maximum supported scale (50 sub-projects)
		maxScale := suite.maxSubProjects
		
		suite.T().Run(fmt.Sprintf("max_scale_%d_projects", maxScale), func(t *testing.T) {
			// Create workspace at maximum scale
			maxWorkspace := suite.helper.CreateLargeWorkspace("max-scale-test", maxScale)
			
			// Test basic functionality at max scale
			start := time.Now()
			result, err := suite.detector.DetectWorkspaceAt(maxWorkspace)
			detectionTime := time.Since(start)
			
			// Should still function at max scale
			suite.NoError(err, "System should function at maximum scale %d", maxScale)
			suite.NotNil(result)
			suite.NotEmpty(result.SubProjects)
			
			// Should meet performance requirements even at max scale
			suite.Less(detectionTime, suite.maxDetectionTime,
				"Detection time %v exceeds limit %v at maximum scale %d", 
				detectionTime, suite.maxDetectionTime, maxScale)
			
			// Should detect reasonable percentage of projects
			minDetectedProjects := int(float64(maxScale) * 0.75) // At least 75% at max scale
			suite.GreaterOrEqual(len(result.SubProjects), minDetectedProjects,
				"Detected %d projects, expected at least %d at maximum scale", 
				len(result.SubProjects), minDetectedProjects)
			
			// Test routing performance at max scale
			routingTime := suite.measureRoutingPerformanceAtScale(result, 500)
			suite.Less(routingTime, suite.maxRoutingTime,
				"Routing time %v exceeds limit %v at maximum scale", routingTime, suite.maxRoutingTime)
			
			// Test memory usage at max scale
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			memoryUsageMB := int64(memStats.HeapAlloc) / (1024 * 1024)
			maxMemoryMB := int64(6 * 1024) // 6GB limit at max scale
			
			suite.LessOrEqual(memoryUsageMB, maxMemoryMB,
				"Memory usage %d MB exceeds limit %d MB at maximum scale", 
				memoryUsageMB, maxMemoryMB)
			
			t.Logf("✓ Maximum scale validation: %d projects, detection=%v, routing=%v, memory=%dMB", 
				len(result.SubProjects), detectionTime, routingTime, memoryUsageMB)
		})
		
		// Test beyond maximum scale to validate graceful handling
		suite.T().Run("beyond_max_scale_graceful_handling", func(t *testing.T) {
			beyondMaxScale := maxScale + 10 // Try 60 projects (beyond 50 limit)
			
			// This test validates graceful degradation/rejection rather than success
			beyondMaxWorkspace := suite.helper.CreateLargeWorkspace("beyond-max-scale", beyondMaxScale)
			
			start := time.Now()
			result, err := suite.detector.DetectWorkspaceAt(beyondMaxWorkspace)
			detectionTime := time.Since(start)
			
			if err != nil {
				// Graceful rejection is acceptable
				suite.Contains(err.Error(), "scale", "Error should mention scale limits")
				t.Logf("✓ Graceful rejection beyond maximum scale: %v", err)
			} else {
				// If it succeeds, it should still meet performance requirements
				suite.NotNil(result)
				suite.Less(detectionTime, suite.maxDetectionTime*2, // Allow 2x time beyond limit
					"Detection time %v excessive beyond maximum scale", detectionTime)
				
				t.Logf("✓ Functions beyond maximum scale: %d projects detected in %v", 
					len(result.SubProjects), detectionTime)
			}
		})
	})
}

// Helper methods

func (suite *ScalabilityIntegrationSuite) measureRoutingPerformanceAtScale(result *workspace.WorkspaceContext, numTests int) time.Duration {
	if len(result.SubProjects) == 0 {
		return 0
	}
	
	var totalTime time.Duration
	
	for i := 0; i < numTests; i++ {
		projectIndex := i % len(result.SubProjects)
		testPath := fmt.Sprintf("%s/test-file-%d.go", result.SubProjects[projectIndex].AbsolutePath, i)
		
		start := time.Now()
		project := suite.detector.FindSubProjectForPath(result, testPath)
		routingTime := time.Since(start)
		
		totalTime += routingTime
		
		if project == nil {
			suite.T().Logf("Warning: routing failed for path %s", testPath)
		}
	}
	
	return totalTime / time.Duration(numTests)
}

func (suite *ScalabilityIntegrationSuite) measureThroughputAtScale(result *workspace.WorkspaceContext, requestCount, concurrency int) float64 {
	if len(result.SubProjects) == 0 {
		return 0
	}
	
	var wg sync.WaitGroup
	var successfulOps int64
	
	start := time.Now()
	
	// Simulate concurrent requests
	requestsPerWorker := requestCount / concurrency
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for j := 0; j < requestsPerWorker; j++ {
				projectIndex := (workerID*requestsPerWorker + j) % len(result.SubProjects)
				testPath := fmt.Sprintf("%s/test-%d-%d.go", 
					result.SubProjects[projectIndex].AbsolutePath, workerID, j)
				
				project := suite.detector.FindSubProjectForPath(result, testPath)
				if project != nil {
					atomic.AddInt64(&successfulOps, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	return float64(successfulOps) / duration.Seconds()
}

func (suite *ScalabilityIntegrationSuite) runConcurrentTest(result *workspace.WorkspaceContext, concurrentOps, opsPerThread int) *ConcurrencyDataPoint {
	var wg sync.WaitGroup
	var successfulOps, failedOps int64
	var totalLatency int64
	var contentionEvents int64
	
	start := time.Now()
	
	for i := 0; i < concurrentOps; i++ {
		wg.Add(1)
		go func(threadID int) {
			defer wg.Done()
			
			for j := 0; j < opsPerThread; j++ {
				opStart := time.Now()
				
				if len(result.SubProjects) > 0 {
					projectIndex := (threadID*opsPerThread + j) % len(result.SubProjects)
					testPath := fmt.Sprintf("%s/concurrent-test-%d-%d.go", 
						result.SubProjects[projectIndex].AbsolutePath, threadID, j)
					
					project := suite.detector.FindSubProjectForPath(result, testPath)
					
					opLatency := time.Since(opStart)
					atomic.AddInt64(&totalLatency, opLatency.Nanoseconds())
					
					if project != nil {
						atomic.AddInt64(&successfulOps, 1)
					} else {
						atomic.AddInt64(&failedOps, 1)
					}
					
					// Simulate contention detection (simplified)
					if opLatency > 10*time.Millisecond {
						atomic.AddInt64(&contentionEvents, 1)
					}
				} else {
					atomic.AddInt64(&failedOps, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	totalOps := successfulOps + failedOps
	var averageLatency time.Duration
	if totalOps > 0 {
		averageLatency = time.Duration(totalLatency / totalOps)
	}
	
	return &ConcurrencyDataPoint{
		SubProjectCount:     len(result.SubProjects),
		ConcurrentLevel:     concurrentOps,
		OperationsPerThread: opsPerThread,
		TotalOperations:     int(totalOps),
		Duration:            duration,
		SuccessfulOps:       successfulOps,
		FailedOps:           failedOps,
		AverageLatency:      averageLatency,
		ContentionEvents:    contentionEvents,
	}
}

func (suite *ScalabilityIntegrationSuite) runThroughputTest(result *workspace.WorkspaceContext, concurrentUsers, requestsPerUser int) *ThroughputDataPoint {
	var wg sync.WaitGroup
	latencies := make([]time.Duration, 0, concurrentUsers*requestsPerUser)
	var latenciesMu sync.Mutex
	var successfulOps, failedOps int64
	
	// Monitor memory during test
	var memStatsBefore, memStatsAfter runtime.MemStats
	runtime.ReadMemStats(&memStatsBefore)
	
	start := time.Now()
	
	for i := 0; i < concurrentUsers; i++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()
			
			userLatencies := make([]time.Duration, 0, requestsPerUser)
			
			for j := 0; j < requestsPerUser; j++ {
				opStart := time.Now()
				
				if len(result.SubProjects) > 0 {
					projectIndex := (userID*requestsPerUser + j) % len(result.SubProjects)
					testPath := fmt.Sprintf("%s/throughput-test-%d-%d.go", 
						result.SubProjects[projectIndex].AbsolutePath, userID, j)
					
					project := suite.detector.FindSubProjectForPath(result, testPath)
					
					opLatency := time.Since(opStart)
					userLatencies = append(userLatencies, opLatency)
					
					if project != nil {
						atomic.AddInt64(&successfulOps, 1)
					} else {
						atomic.AddInt64(&failedOps, 1)
					}
				} else {
					atomic.AddInt64(&failedOps, 1)
				}
			}
			
			// Add user latencies to global collection
			latenciesMu.Lock()
			latencies = append(latencies, userLatencies...)
			latenciesMu.Unlock()
		}(i)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	runtime.ReadMemStats(&memStatsAfter)
	memoryPeakMB := int64(memStatsAfter.HeapAlloc) / (1024 * 1024)
	
	totalRequests := concurrentUsers * requestsPerUser
	throughput := float64(successfulOps) / duration.Seconds()
	errorRate := float64(failedOps) / float64(totalRequests)
	
	// Calculate latency percentiles
	averageLatency, p95Latency, p99Latency := suite.calculateLatencyPercentiles(latencies)
	
	return &ThroughputDataPoint{
		SubProjectCount:     len(result.SubProjects),
		ConcurrentUsers:     concurrentUsers,
		RequestsPerUser:     requestsPerUser,
		TotalRequests:       totalRequests,
		Duration:            duration,
		ThroughputReqPerSec: throughput,
		AverageLatency:      averageLatency,
		P95Latency:          p95Latency,
		P99Latency:          p99Latency,
		ErrorRate:           errorRate,
		MemoryPeakMB:        memoryPeakMB,
	}
}

func (suite *ScalabilityIntegrationSuite) measureOperationPerformance(workspacePath, operationType string) time.Duration {
	switch operationType {
	case "detection":
		start := time.Now()
		result, err := suite.detector.DetectWorkspaceAt(workspacePath)
		if err != nil || result == nil {
			return time.Hour // Return high value for failed operations
		}
		return time.Since(start)
		
	case "routing":
		result, err := suite.detector.DetectWorkspaceAt(workspacePath)
		if err != nil || result == nil || len(result.SubProjects) == 0 {
			return time.Hour
		}
		
		// Measure average routing time
		const testCount = 100
		testPath := fmt.Sprintf("%s/test.go", result.SubProjects[0].AbsolutePath)
		
		start := time.Now()
		for i := 0; i < testCount; i++ {
			suite.detector.FindSubProjectForPath(result, testPath)
		}
		return time.Since(start) / testCount
		
	default:
		return time.Hour
	}
}

func (suite *ScalabilityIntegrationSuite) calculateLatencyPercentiles(latencies []time.Duration) (avg, p95, p99 time.Duration) {
	if len(latencies) == 0 {
		return 0, 0, 0
	}
	
	// Calculate average
	var total time.Duration
	for _, latency := range latencies {
		total += latency
	}
	avg = total / time.Duration(len(latencies))
	
	// Sort for percentiles (simplified sorting)
	sortedLatencies := make([]time.Duration, len(latencies))
	copy(sortedLatencies, latencies)
	
	// Simple bubble sort for this test
	for i := 0; i < len(sortedLatencies); i++ {
		for j := i + 1; j < len(sortedLatencies); j++ {
			if sortedLatencies[i] > sortedLatencies[j] {
				sortedLatencies[i], sortedLatencies[j] = sortedLatencies[j], sortedLatencies[i]
			}
		}
	}
	
	// Calculate percentiles
	p95Index := int(math.Ceil(float64(len(sortedLatencies)) * 0.95)) - 1
	p99Index := int(math.Ceil(float64(len(sortedLatencies)) * 0.99)) - 1
	
	if p95Index >= len(sortedLatencies) {
		p95Index = len(sortedLatencies) - 1
	}
	if p99Index >= len(sortedLatencies) {
		p99Index = len(sortedLatencies) - 1
	}
	
	p95 = sortedLatencies[p95Index]
	p99 = sortedLatencies[p99Index]
	
	return avg, p95, p99
}

func (suite *ScalabilityIntegrationSuite) logScalabilityAnalysis() {
	suite.scalabilityMetrics.mu.RLock()
	defer suite.scalabilityMetrics.mu.RUnlock()
	
	suite.T().Logf("\n=== SCALABILITY ANALYSIS ===")
	
	// Project count scaling analysis
	if len(suite.scalabilityMetrics.dataPoints) > 0 {
		suite.T().Logf("\nProject Count Scaling:")
		for _, dp := range suite.scalabilityMetrics.dataPoints {
			suite.T().Logf("  %d projects: detection=%v, memory=%dMB, routing=%v, throughput=%.1f", 
				dp.SubProjectCount, dp.DetectionTime, dp.MemoryUsageMB, dp.RoutingTimePerOp, dp.ThroughputReqPerSec)
		}
	}
	
	// Throughput analysis
	if len(suite.scalabilityMetrics.throughputTests) > 0 {
		suite.T().Logf("\nThroughput Analysis:")
		for _, tp := range suite.scalabilityMetrics.throughputTests {
			suite.T().Logf("  %dp/%du: %.1f req/s, P95=%v, P99=%v, mem=%dMB", 
				tp.SubProjectCount, tp.ConcurrentUsers, tp.ThroughputReqPerSec, 
				tp.P95Latency, tp.P99Latency, tp.MemoryPeakMB)
		}
	}
	
	// Degradation analysis
	if len(suite.scalabilityMetrics.degradationTests) > 0 {
		suite.T().Logf("\nPerformance Degradation:")
		for _, deg := range suite.scalabilityMetrics.degradationTests {
			acceptableStr := "✓"
			if !deg.AcceptableDegradation {
				acceptableStr = "✗"
			}
			suite.T().Logf("  %s %s %dp->%dp: %.2fx degradation (%v->%v)", 
				acceptableStr, deg.DegradationType, deg.BaselineSubProjects, deg.ScaledSubProjects,
				deg.DegradationFactor, deg.BaselinePerformance, deg.ScaledPerformance)
		}
	}
	
	// Concurrency analysis
	if len(suite.scalabilityMetrics.concurrencyTests) > 0 {
		suite.T().Logf("\nConcurrency Analysis:")
		for _, ct := range suite.scalabilityMetrics.concurrencyTests {
			suite.T().Logf("  %dp/%dc: avg_latency=%v, success=%d, failed=%d, contention=%d", 
				ct.SubProjectCount, ct.ConcurrentLevel, ct.AverageLatency, 
				ct.SuccessfulOps, ct.FailedOps, ct.ContentionEvents)
		}
	}
	
	suite.T().Logf("=== END SCALABILITY ANALYSIS ===\n")
}

func TestScalabilityIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping scalability integration tests in short mode")
	}
	
	suite.Run(t, new(ScalabilityIntegrationSuite))
}