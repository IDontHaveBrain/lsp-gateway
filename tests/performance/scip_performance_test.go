package performance

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-gateway/internal/indexing"
	"lsp-gateway/tests/framework"
)

// SCIPPerformanceTest validates SCIP integration performance requirements
type SCIPPerformanceTest struct {
	framework    *framework.MultiLanguageTestFramework
	profiler     *framework.PerformanceProfiler
	testProject  *framework.TestProject
	scipStore    indexing.SCIPStore

	// Performance thresholds (Phase 1 targets)
	MinCacheHitRate           float64       // >10% cache hit rate target
	MaxSCIPOverhead           time.Duration // <50ms overhead for SCIP infrastructure
	MaxMemoryImpact           int64         // <100MB additional memory for SCIP
	MaxResponseTimeRegression time.Duration // No performance regression in LSP-only mode
	MinThroughputMaintained   float64       // Maintain 100+ req/sec with mixed protocols

	// Test configuration
	BaselineConcurrency       int
	TestConcurrency           []int
	TestDurations             []time.Duration
	SCIPSupportedMethods      []string
	LSPOnlyMethods            []string
	MixedProtocolRatios       []float64

	// Metrics tracking
	baselineMetrics           *SCIPBaselineMetrics
	scipMetrics               *SCIPPerformanceMetrics
	performanceComparisons    []*SCIPPerformanceComparison

	mu sync.RWMutex
}

// SCIPBaselineMetrics contains LSP-only baseline performance metrics
type SCIPBaselineMetrics struct {
	AverageResponseTime    time.Duration            `json:"average_response_time"`
	P95ResponseTime        time.Duration            `json:"p95_response_time"`
	ThroughputReqPerSec    float64                  `json:"throughput_req_per_sec"`
	MemoryUsageMB          int64                    `json:"memory_usage_mb"`
	MethodResponseTimes    map[string]time.Duration `json:"method_response_times"`
	ErrorRate              float64                  `json:"error_rate"`
	ConcurrentCapacity     int                      `json:"concurrent_capacity"`
}

// SCIPPerformanceMetrics contains SCIP-enabled performance metrics
type SCIPPerformanceMetrics struct {
	CacheHitRate           float64                  `json:"cache_hit_rate"`
	CacheMissRate          float64                  `json:"cache_miss_rate"`
	SCIPOverheadMs         int64                    `json:"scip_overhead_ms"`
	MemoryImpactMB         int64                    `json:"memory_impact_mb"`
	AverageResponseTime    time.Duration            `json:"average_response_time"`
	P95ResponseTime        time.Duration            `json:"p95_response_time"`
	ThroughputReqPerSec    float64                  `json:"throughput_req_per_sec"`
	MethodPerformance      map[string]*MethodMetrics `json:"method_performance"`
	IndexingPerformance    *IndexingMetrics         `json:"indexing_performance"`
	CacheOperationMetrics  *CacheOperationMetrics   `json:"cache_operation_metrics"`
}

// MethodMetrics contains performance metrics for specific LSP methods
type MethodMetrics struct {
	Method                string        `json:"method"`
	TotalRequests         int64         `json:"total_requests"`
	CacheHits             int64         `json:"cache_hits"`
	CacheMisses           int64         `json:"cache_misses"`
	AverageResponseTime   time.Duration `json:"average_response_time"`
	CacheHitResponseTime  time.Duration `json:"cache_hit_response_time"`
	CacheMissResponseTime time.Duration `json:"cache_miss_response_time"`
	ErrorCount            int64         `json:"error_count"`
	PerformanceImprovement float64      `json:"performance_improvement_percent"`
}

// IndexingMetrics contains SCIP indexing operation metrics
type IndexingMetrics struct {
	IndexLoadTime          time.Duration `json:"index_load_time"`
	IndexMemoryUsageMB     int64         `json:"index_memory_usage_mb"`
	QueryAverageTime       time.Duration `json:"query_average_time"`
	IndexSize              int64         `json:"index_size_bytes"`
	SymbolCount            int           `json:"symbol_count"`
	FileCount              int           `json:"file_count"`
}

// CacheOperationMetrics contains cache-specific performance metrics
type CacheOperationMetrics struct {
	CacheSize              int           `json:"cache_size"`
	CacheMemoryUsageMB     int64         `json:"cache_memory_usage_mb"`
	EvictionCount          int64         `json:"eviction_count"`
	InvalidationCount      int64         `json:"invalidation_count"`
	AverageCacheLatency    time.Duration `json:"average_cache_latency"`
	CacheEfficiency        float64       `json:"cache_efficiency_percent"`
}

// SCIPPerformanceComparison contains comparison between baseline and SCIP-enabled performance
type SCIPPerformanceComparison struct {
	Scenario                  string                   `json:"scenario"`
	Concurrency              int                      `json:"concurrency"`
	MixedProtocolRatio       float64                  `json:"mixed_protocol_ratio"`
	BaselineMetrics          *SCIPBaselineMetrics     `json:"baseline_metrics"`
	SCIPMetrics              *SCIPPerformanceMetrics  `json:"scip_metrics"`
	PerformanceRegression    bool                     `json:"performance_regression"`
	CacheEffectiveness       float64                  `json:"cache_effectiveness"`
	MemoryEfficiency         float64                  `json:"memory_efficiency"`
	RequirementsMet          map[string]bool          `json:"requirements_met"`
}

// SCIPRequestResult contains the result of a SCIP-enabled request
type SCIPRequestResult struct {
	Method           string        `json:"method"`
	Success          bool          `json:"success"`
	ResponseTime     time.Duration `json:"response_time"`
	CacheHit         bool          `json:"cache_hit"`
	SCIPUsed         bool          `json:"scip_used"`
	Error            error         `json:"error,omitempty"`
	Timestamp        time.Time     `json:"timestamp"`
	MemoryAllocated  int64         `json:"memory_allocated"`
}

// PerformanceRegressionResult contains results of regression analysis
type PerformanceRegressionResult struct {
	HasRegression                 bool     `json:"has_regression"`
	ThroughputRegressionPercent   float64  `json:"throughput_regression_percent"`
	ResponseTimeRegressionPercent float64  `json:"response_time_regression_percent"`
	RegressionDetails            []string `json:"regression_details"`
}

// SCIPHighLoadMetrics contains high-load performance metrics
type SCIPHighLoadMetrics struct {
	ThroughputReqPerSec float64                 `json:"throughput_req_per_sec"`
	P95ResponseTime     time.Duration           `json:"p95_response_time"`
	CacheHitRate        float64                 `json:"cache_hit_rate"`
	SCIPMetrics         *SCIPPerformanceMetrics `json:"scip_metrics"`
}

// NewSCIPPerformanceTest creates a new SCIP performance test
func NewSCIPPerformanceTest(t *testing.T) *SCIPPerformanceTest {
	return &SCIPPerformanceTest{
		framework: framework.NewMultiLanguageTestFramework(60 * time.Minute),
		profiler:  framework.NewPerformanceProfiler(),

		// Phase 1 performance requirements
		MinCacheHitRate:           0.10, // >10% cache hit rate
		MaxSCIPOverhead:           50 * time.Millisecond, // <50ms SCIP overhead
		MaxMemoryImpact:           100 * 1024 * 1024, // <100MB additional memory
		MaxResponseTimeRegression: 0 * time.Millisecond, // No regression allowed
		MinThroughputMaintained:   100.0, // Maintain 100+ req/sec

		// Test configuration
		BaselineConcurrency: 50,
		TestConcurrency:     []int{25, 50, 100, 150, 200},
		TestDurations:       []time.Duration{30 * time.Second, 60 * time.Second, 120 * time.Second},
		
		// SCIP-supported methods for Phase 1
		SCIPSupportedMethods: []string{
			"textDocument/definition",
			"textDocument/references",
			"textDocument/hover",
			"textDocument/documentSymbol",
			"workspace/symbol",
		},
		
		// LSP-only methods (not supported by SCIP in Phase 1)
		LSPOnlyMethods: []string{
			"textDocument/completion",
			"textDocument/signatureHelp",
			"textDocument/formatting",
			"textDocument/codeAction",
		},
		
		// Mixed protocol testing ratios (SCIP-enabled vs LSP-only requests)
		MixedProtocolRatios: []float64{0.2, 0.5, 0.8},

		// Initialize metrics tracking
		performanceComparisons: make([]*SCIPPerformanceComparison, 0),
	}
}

// TestSCIPPerformanceBaseline establishes LSP-only baseline performance
func (test *SCIPPerformanceTest) TestSCIPPerformanceBaseline(t *testing.T) {
	ctx := context.Background()

	if err := test.setupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	t.Log("Establishing LSP-only baseline performance metrics...")

	// Measure baseline performance without SCIP
	baselineMetrics, err := test.measureBaselinePerformance(ctx, test.BaselineConcurrency, 60*time.Second)
	if err != nil {
		t.Fatalf("Failed to measure baseline performance: %v", err)
	}

	test.baselineMetrics = baselineMetrics

	t.Logf("LSP-only baseline: Throughput=%.2f req/sec, P95=%v, Memory=%dMB, Error rate=%.2f%%",
		baselineMetrics.ThroughputReqPerSec, baselineMetrics.P95ResponseTime,
		baselineMetrics.MemoryUsageMB, baselineMetrics.ErrorRate*100)

	// Validate baseline performance meets enterprise requirements
	test.validateBaselinePerformance(t, baselineMetrics)
}

// TestSCIPCacheHitRate validates SCIP cache hit rate requirements
func (test *SCIPPerformanceTest) TestSCIPCacheHitRate(t *testing.T) {
	ctx := context.Background()

	if err := test.setupSCIPEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup SCIP environment: %v", err)
	}
	defer test.cleanup()

	t.Log("Testing SCIP cache hit rate performance...")

	// Test cache effectiveness with different request patterns
	cacheTestScenarios := []struct {
		name            string
		requestPattern  string
		expectedHitRate float64
	}{
		{"repeated_queries", "repeated", 0.15}, // Expect >15% for repeated queries
		{"similar_queries", "similar", 0.12},   // Expect >12% for similar queries
		{"mixed_queries", "mixed", 0.10},       // Expect >10% for mixed queries
	}

	for _, scenario := range cacheTestScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			hitRate, metrics := test.measureCacheHitRate(t, scenario.requestPattern, 50, 90*time.Second)

			if hitRate < scenario.expectedHitRate {
				t.Errorf("Cache hit rate too low for %s: %.2f%% < %.2f%%", 
					scenario.name, hitRate*100, scenario.expectedHitRate*100)
			}

			// Validate cache effectiveness
			if hitRate > 0 && metrics.CacheHitResponseTime >= metrics.CacheMissResponseTime {
				t.Errorf("Cache hits should be faster than misses: %v >= %v",
					metrics.CacheHitResponseTime, metrics.CacheMissResponseTime)
			}

			t.Logf("Cache performance (%s): Hit rate=%.2f%%, Hit time=%v, Miss time=%v",
				scenario.name, hitRate*100, metrics.CacheHitResponseTime, metrics.CacheMissResponseTime)
		})
	}
}

// TestSCIPMemoryImpact validates memory usage impact of SCIP components
func (test *SCIPPerformanceTest) TestSCIPMemoryImpact(t *testing.T) {
	ctx := context.Background()

	// First measure baseline memory usage
	if err := test.setupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup baseline environment: %v", err)
	}

	baselineMemory := test.measureCurrentMemoryUsage()
	test.cleanup()

	// Then measure with SCIP enabled
	if err := test.setupSCIPEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup SCIP environment: %v", err)
	}
	defer test.cleanup()

	scipMemory := test.measureCurrentMemoryUsage()
	memoryImpact := scipMemory - baselineMemory

	t.Logf("Memory impact: Baseline=%dMB, SCIP=%dMB, Impact=%dMB",
		baselineMemory/1024/1024, scipMemory/1024/1024, memoryImpact/1024/1024)

	// Validate memory impact is within acceptable limits
	if memoryImpact > test.MaxMemoryImpact {
		t.Errorf("SCIP memory impact too high: %dMB > %dMB",
			memoryImpact/1024/1024, test.MaxMemoryImpact/1024/1024)
	}

	// Test memory usage under load
	memoryMetrics := test.measureMemoryUsageUnderLoad(ctx, 100, 120*time.Second)
	
	// Validate no memory leaks
	if memoryMetrics.MemoryLeakDetected {
		t.Errorf("Memory leak detected in SCIP components: %s", memoryMetrics.MemoryLeakSeverity)
	}

	// Validate peak memory usage
	if memoryMetrics.PeakMemoryUsageMB > (baselineMemory/1024/1024)+100 {
		t.Errorf("Peak memory usage too high: %dMB > %dMB",
			memoryMetrics.PeakMemoryUsageMB, (baselineMemory/1024/1024)+100)
	}
}

// TestSCIPMixedProtocolPerformance validates performance with mixed SCIP/LSP requests
func (test *SCIPPerformanceTest) TestSCIPMixedProtocolPerformance(t *testing.T) {
	ctx := context.Background()

	if err := test.setupSCIPEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup SCIP environment: %v", err)
	}
	defer test.cleanup()

	// Ensure baseline is established
	if test.baselineMetrics == nil {
		baselineMetrics, err := test.measureBaselinePerformance(ctx, test.BaselineConcurrency, 60*time.Second)
		if err != nil {
			t.Fatalf("Failed to establish baseline: %v", err)
		}
		test.baselineMetrics = baselineMetrics
	}

	t.Log("Testing mixed protocol performance with 100+ concurrent requests...")

	// Test different concurrency levels and protocol ratios
	for _, concurrency := range test.TestConcurrency {
		for _, scipRatio := range test.MixedProtocolRatios {
			t.Run(fmt.Sprintf("Concurrency_%d_SCIPRatio_%.0f", concurrency, scipRatio*100), func(t *testing.T) {
				comparison := test.measureMixedProtocolPerformance(t, concurrency, scipRatio, 90*time.Second)
				
				test.validateMixedProtocolPerformance(t, comparison)
				test.performanceComparisons = append(test.performanceComparisons, comparison)

				t.Logf("Mixed protocol (C=%d, SCIP=%.0f%%): Throughput=%.2f req/sec, Cache hit=%.2f%%, Regression=%t",
					concurrency, scipRatio*100, comparison.SCIPMetrics.ThroughputReqPerSec,
					comparison.SCIPMetrics.CacheHitRate*100, comparison.PerformanceRegression)
			})
		}
	}
}

// TestSCIPPerformanceRegression validates no performance regression in LSP-only mode
func (test *SCIPPerformanceTest) TestSCIPPerformanceRegression(t *testing.T) {
	ctx := context.Background()

	// Test with SCIP enabled but using only LSP-only methods
	if err := test.setupSCIPEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup SCIP environment: %v", err)
	}
	defer test.cleanup()

	t.Log("Testing performance regression with SCIP enabled but using LSP-only methods...")

	scipLSPOnlyMetrics, err := test.measureLSPOnlyPerformanceWithSCIP(ctx, test.BaselineConcurrency, 60*time.Second)
	if err != nil {
		t.Fatalf("Failed to measure LSP-only performance with SCIP: %v", err)
	}

	// Compare with baseline
	if test.baselineMetrics == nil {
		t.Fatal("Baseline metrics not available - run baseline test first")
	}

	regression := test.detectPerformanceRegression(test.baselineMetrics, scipLSPOnlyMetrics)
	
	if regression.HasRegression {
		t.Errorf("Performance regression detected in LSP-only mode with SCIP enabled:")
		for _, detail := range regression.RegressionDetails {
			t.Errorf("  - %s", detail)
		}
	}

	t.Logf("LSP-only with SCIP overhead: Throughput regression=%.2f%%, Response time regression=%.2f%%",
		regression.ThroughputRegressionPercent, regression.ResponseTimeRegressionPercent)
}

// TestSCIPConcurrentLoadCapacity tests SCIP performance under high concurrent load
func (test *SCIPPerformanceTest) TestSCIPConcurrentLoadCapacity(t *testing.T) {
	ctx := context.Background()

	if err := test.setupSCIPEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup SCIP environment: %v", err)
	}
	defer test.cleanup()

	t.Log("Testing SCIP performance under high concurrent load (200+ requests)...")

	// Test escalating concurrency levels
	concurrencyLevels := []int{100, 150, 200, 300, 500}
	
	for _, concurrency := range concurrencyLevels {
		t.Run(fmt.Sprintf("HighLoad_%d", concurrency), func(t *testing.T) {
			loadMetrics, err := test.measureHighConcurrencyLoad(ctx, concurrency, 120*time.Second)
			if err != nil {
				t.Errorf("High concurrency test failed at %d concurrent requests: %v", concurrency, err)
				return
			}

			// Validate performance requirements under load
			test.validateHighConcurrencyPerformance(t, concurrency, loadMetrics)

			errorCount := int64(0)
			if overallMetrics, exists := loadMetrics.SCIPMetrics.MethodPerformance["overall"]; exists {
				errorCount = overallMetrics.ErrorCount
			}

			t.Logf("High load (C=%d): Throughput=%.2f req/sec, P95=%v, Cache hit=%.2f%%, Errors=%d",
				concurrency, loadMetrics.ThroughputReqPerSec, loadMetrics.P95ResponseTime,
				loadMetrics.CacheHitRate*100, errorCount)
		})
	}
}

// BenchmarkSCIPPerformance benchmarks SCIP performance
func (test *SCIPPerformanceTest) BenchmarkSCIPPerformance(b *testing.B) {
	ctx := context.Background()

	if err := test.setupSCIPEnvironment(ctx); err != nil {
		b.Fatalf("Failed to setup SCIP environment: %v", err)
	}
	defer test.cleanup()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := test.performBenchmarkRequests(50, 100)
		if err != nil {
			b.Fatalf("Benchmark iteration %d failed: %v", i, err)
		}
	}
}

// Helper methods

func (test *SCIPPerformanceTest) setupTestEnvironment(ctx context.Context) error {
	if err := test.framework.SetupTestEnvironment(ctx); err != nil {
		return fmt.Errorf("failed to setup framework: %w", err)
	}

	// Create test project
	project, err := test.framework.CreateMultiLanguageProject(
		framework.ProjectTypeMultiLanguage,
		[]string{"go", "python", "typescript", "java"})
	if err != nil {
		return fmt.Errorf("failed to create test project: %w", err)
	}
	test.testProject = project

	// Start language servers
	if err := test.framework.StartMultipleLanguageServers(project.Languages); err != nil {
		return fmt.Errorf("failed to start language servers: %w", err)
	}

	return nil
}

func (test *SCIPPerformanceTest) setupSCIPEnvironment(ctx context.Context) error {
	if err := test.setupTestEnvironment(ctx); err != nil {
		return err
	}

	// Configure SCIP integration
	scipConfig := &indexing.SCIPConfig{
		CacheConfig: indexing.CacheConfig{
			Enabled: true,
			MaxSize: 1000,
			TTL:     5 * time.Minute,
		},
		Logging: indexing.LoggingConfig{
			LogQueries:         false, // Disable to avoid test noise
			LogCacheOperations: false,
			LogIndexOperations: false,
		},
		Performance: indexing.PerformanceConfig{
			QueryTimeout:           5 * time.Second,
			MaxConcurrentQueries:   50,
			IndexLoadTimeout:       30 * time.Second,
		},
	}

	// Create SCIP store
	test.scipStore = indexing.NewSCIPIndexStore(scipConfig)

	// Load mock SCIP index for testing
	indexPath := "/mock/scip/index"
	if err := test.scipStore.LoadIndex(indexPath); err != nil {
		return fmt.Errorf("failed to load SCIP index: %w", err)
	}

	return nil
}

func (test *SCIPPerformanceTest) measureBaselinePerformance(ctx context.Context, concurrency int, duration time.Duration) (*SCIPBaselineMetrics, error) {
	results := make(chan *SCIPRequestResult, concurrency*100)
	var wg sync.WaitGroup

	startTime := time.Now()
	endTime := startTime.Add(duration)
	initialMemory := test.measureCurrentMemoryUsage()

	// Start concurrent workers using only LSP methods
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			test.baselineWorker(workerID, endTime, results)
		}(i)
	}

	// Collect results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Process results
	metrics := test.collectBaselineMetrics(results)
	metrics.MemoryUsageMB = (test.measureCurrentMemoryUsage() - initialMemory) / 1024 / 1024
	metrics.ConcurrentCapacity = concurrency

	return metrics, nil
}

func (test *SCIPPerformanceTest) baselineWorker(workerID int, endTime time.Time, results chan<- *SCIPRequestResult) {
	allMethods := append(test.SCIPSupportedMethods, test.LSPOnlyMethods...)
	
	for time.Now().Before(endTime) {
		method := allMethods[rand.Intn(len(allMethods))]
		
		startTime := time.Now()
		err := test.simulateLSPRequest(method, false) // No SCIP
		responseTime := time.Since(startTime)

		result := &SCIPRequestResult{
			Method:          method,
			Success:         err == nil,
			ResponseTime:    responseTime,
			CacheHit:        false, // No cache in baseline
			SCIPUsed:        false,
			Error:           err,
			Timestamp:       startTime,
			MemoryAllocated: test.measureCurrentMemoryUsage(),
		}

		select {
		case results <- result:
		case <-time.After(1 * time.Second):
			// Avoid blocking
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func (test *SCIPPerformanceTest) measureCacheHitRate(t *testing.T, requestPattern string, concurrency int, duration time.Duration) (float64, *MethodMetrics) {
	results := make(chan *SCIPRequestResult, concurrency*200)
	var wg sync.WaitGroup

	endTime := time.Now().Add(duration)

	// Start workers with specific request pattern
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			test.cacheTestWorker(workerID, requestPattern, endTime, results)
		}(i)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect and analyze cache metrics
	return test.analyzeCachePerformance(results)
}

func (test *SCIPPerformanceTest) cacheTestWorker(workerID int, pattern string, endTime time.Time, results chan<- *SCIPRequestResult) {
	switch pattern {
	case "repeated":
		// Use same method repeatedly to maximize cache hits
		method := "textDocument/definition"
		for time.Now().Before(endTime) {
			test.performSCIPRequest(method, results)
			time.Sleep(20 * time.Millisecond)
		}
	case "similar":
		// Use similar methods that might share cache entries
		methods := []string{"textDocument/definition", "textDocument/references"}
		for time.Now().Before(endTime) {
			method := methods[rand.Intn(len(methods))]
			test.performSCIPRequest(method, results)
			time.Sleep(20 * time.Millisecond)
		}
	case "mixed":
		// Mixed pattern with all SCIP-supported methods
		for time.Now().Before(endTime) {
			method := test.SCIPSupportedMethods[rand.Intn(len(test.SCIPSupportedMethods))]
			test.performSCIPRequest(method, results)
			time.Sleep(15 * time.Millisecond)
		}
	}
}

func (test *SCIPPerformanceTest) performSCIPRequest(method string, results chan<- *SCIPRequestResult) {
	startTime := time.Now()
	
	// Query SCIP store first
	queryResult := test.scipStore.Query(method, map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": "file:///test.go"},
		"position":     map[string]interface{}{"line": 10, "character": 5},
	})
	
	responseTime := time.Since(startTime)

	result := &SCIPRequestResult{
		Method:          method,
		Success:         queryResult.Error == "",
		ResponseTime:    responseTime,
		CacheHit:        queryResult.CacheHit,
		SCIPUsed:        queryResult.Found,
		Timestamp:       startTime,
		MemoryAllocated: test.measureCurrentMemoryUsage(),
	}

	if !queryResult.Found {
		// Fallback to LSP if SCIP doesn't have result
		err := test.simulateLSPRequest(method, true)
		result.Success = err == nil
		if err != nil {
			result.Error = err
		}
	}

	select {
	case results <- result:
	case <-time.After(500 * time.Millisecond):
		// Avoid blocking
	}
}

func (test *SCIPPerformanceTest) measureMixedProtocolPerformance(t *testing.T, concurrency int, scipRatio float64, duration time.Duration) *SCIPPerformanceComparison {
	results := make(chan *SCIPRequestResult, concurrency*200)
	var wg sync.WaitGroup

	endTime := time.Now().Add(duration)
	initialMemory := test.measureCurrentMemoryUsage()

	// Start mixed protocol workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			test.mixedProtocolWorker(workerID, scipRatio, endTime, results)
		}(i)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect and analyze results
	scipMetrics := test.collectSCIPMetrics(results)
	scipMetrics.MemoryImpactMB = (test.measureCurrentMemoryUsage() - initialMemory) / 1024 / 1024

	// Create performance comparison
	comparison := &SCIPPerformanceComparison{
		Scenario:           fmt.Sprintf("Mixed_C%d_S%.0f", concurrency, scipRatio*100),
		Concurrency:        concurrency,
		MixedProtocolRatio: scipRatio,
		BaselineMetrics:    test.baselineMetrics,
		SCIPMetrics:        scipMetrics,
		RequirementsMet:    make(map[string]bool),
	}

	// Analyze performance
	comparison.PerformanceRegression = test.detectRegressionInComparison(comparison)
	comparison.CacheEffectiveness = scipMetrics.CacheHitRate
	comparison.MemoryEfficiency = test.calculateMemoryEfficiency(comparison)

	// Check requirements
	comparison.RequirementsMet["cache_hit_rate"] = scipMetrics.CacheHitRate >= test.MinCacheHitRate
	comparison.RequirementsMet["memory_impact"] = scipMetrics.MemoryImpactMB*1024*1024 <= test.MaxMemoryImpact
	comparison.RequirementsMet["throughput_maintained"] = scipMetrics.ThroughputReqPerSec >= test.MinThroughputMaintained
	comparison.RequirementsMet["no_regression"] = !comparison.PerformanceRegression

	return comparison
}

func (test *SCIPPerformanceTest) mixedProtocolWorker(workerID int, scipRatio float64, endTime time.Time, results chan<- *SCIPRequestResult) {
	for time.Now().Before(endTime) {
		var method string
		useSCIP := rand.Float64() < scipRatio

		if useSCIP {
			method = test.SCIPSupportedMethods[rand.Intn(len(test.SCIPSupportedMethods))]
			test.performSCIPRequest(method, results)
		} else {
			method = test.LSPOnlyMethods[rand.Intn(len(test.LSPOnlyMethods))]
			startTime := time.Now()
			err := test.simulateLSPRequest(method, true)
			responseTime := time.Since(startTime)

			result := &SCIPRequestResult{
				Method:          method,
				Success:         err == nil,
				ResponseTime:    responseTime,
				CacheHit:        false,
				SCIPUsed:        false,
				Error:           err,
				Timestamp:       startTime,
				MemoryAllocated: test.measureCurrentMemoryUsage(),
			}

			select {
			case results <- result:
			case <-time.After(500 * time.Millisecond):
				// Avoid blocking
			}
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func (test *SCIPPerformanceTest) simulateLSPRequest(method string, withSCIP bool) error {
	// Simulate request processing time and potential overhead
	var processingTime time.Duration

	baseTime := test.getMethodBaseProcessingTime(method)
	
	if withSCIP {
		// Add SCIP overhead for infrastructure
		scipOverhead := time.Duration(rand.Intn(int(test.MaxSCIPOverhead.Nanoseconds()))) * time.Nanosecond
		processingTime = baseTime + scipOverhead
	} else {
		processingTime = baseTime
	}

	time.Sleep(processingTime)

	// Simulate occasional failures
	if rand.Float32() < 0.02 { // 2% failure rate
		return fmt.Errorf("simulated LSP request failure for %s", method)
	}

	return nil
}

func (test *SCIPPerformanceTest) getMethodBaseProcessingTime(method string) time.Duration {
	switch method {
	case "textDocument/definition":
		return time.Duration(10+rand.Intn(30)) * time.Millisecond
	case "textDocument/references":
		return time.Duration(20+rand.Intn(50)) * time.Millisecond
	case "textDocument/hover":
		return time.Duration(5+rand.Intn(15)) * time.Millisecond
	case "textDocument/documentSymbol":
		return time.Duration(15+rand.Intn(40)) * time.Millisecond
	case "workspace/symbol":
		return time.Duration(30+rand.Intn(80)) * time.Millisecond
	case "textDocument/completion":
		return time.Duration(25+rand.Intn(60)) * time.Millisecond
	case "textDocument/signatureHelp":
		return time.Duration(8+rand.Intn(20)) * time.Millisecond
	case "textDocument/formatting":
		return time.Duration(50+rand.Intn(100)) * time.Millisecond
	case "textDocument/codeAction":
		return time.Duration(20+rand.Intn(60)) * time.Millisecond
	default:
		return time.Duration(15+rand.Intn(30)) * time.Millisecond
	}
}

func (test *SCIPPerformanceTest) measureCurrentMemoryUsage() int64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.Alloc)
}

func (test *SCIPPerformanceTest) collectBaselineMetrics(results <-chan *SCIPRequestResult) *SCIPBaselineMetrics {
	var (
		totalRequests     int64
		successfulRequests int64
		totalResponseTime time.Duration
		responseTimes     []time.Duration
		methodTimes       = make(map[string][]time.Duration)
	)

	for result := range results {
		totalRequests++
		totalResponseTime += result.ResponseTime
		responseTimes = append(responseTimes, result.ResponseTime)

		if result.Success {
			successfulRequests++
		}

		// Track per-method performance
		if _, exists := methodTimes[result.Method]; !exists {
			methodTimes[result.Method] = make([]time.Duration, 0)
		}
		methodTimes[result.Method] = append(methodTimes[result.Method], result.ResponseTime)
	}

	// Calculate metrics
	var avgResponseTime time.Duration
	if totalRequests > 0 {
		avgResponseTime = totalResponseTime / time.Duration(totalRequests)
	}

	p95ResponseTime := test.calculatePercentile(responseTimes, 95)
	throughput := float64(totalRequests) / 60.0 // requests per second over 60s
	errorRate := float64(totalRequests-successfulRequests) / float64(totalRequests)

	// Calculate per-method averages
	methodResponseTimes := make(map[string]time.Duration)
	for method, times := range methodTimes {
		var total time.Duration
		for _, t := range times {
			total += t
		}
		if len(times) > 0 {
			methodResponseTimes[method] = total / time.Duration(len(times))
		}
	}

	return &SCIPBaselineMetrics{
		AverageResponseTime: avgResponseTime,
		P95ResponseTime:     p95ResponseTime,
		ThroughputReqPerSec: throughput,
		MethodResponseTimes: methodResponseTimes,
		ErrorRate:           errorRate,
	}
}

func (test *SCIPPerformanceTest) collectSCIPMetrics(results <-chan *SCIPRequestResult) *SCIPPerformanceMetrics {
	var (
		totalRequests     int64
		cacheHits         int64
		cacheMisses       int64
		scipUsed          int64
		totalResponseTime time.Duration
		responseTimes     []time.Duration
		methodMetrics     = make(map[string]*MethodMetrics)
	)

	for result := range results {
		totalRequests++
		totalResponseTime += result.ResponseTime
		responseTimes = append(responseTimes, result.ResponseTime)

		if result.CacheHit {
			cacheHits++
		} else {
			cacheMisses++
		}

		if result.SCIPUsed {
			scipUsed++
		}

		// Track per-method metrics
		if _, exists := methodMetrics[result.Method]; !exists {
			methodMetrics[result.Method] = &MethodMetrics{
				Method: result.Method,
			}
		}

		metrics := methodMetrics[result.Method]
		metrics.TotalRequests++
		if result.CacheHit {
			metrics.CacheHits++
		} else {
			metrics.CacheMisses++
		}

		if !result.Success {
			metrics.ErrorCount++
		}
	}

	// Calculate overall metrics
	var avgResponseTime time.Duration
	if totalRequests > 0 {
		avgResponseTime = totalResponseTime / time.Duration(totalRequests)
	}

	cacheHitRate := float64(cacheHits) / float64(totalRequests)
	cacheMissRate := float64(cacheMisses) / float64(totalRequests)
	p95ResponseTime := test.calculatePercentile(responseTimes, 95)
	throughput := float64(totalRequests) / 60.0 // requests per second

	// Get SCIP store stats
	scipStats := test.scipStore.GetStats()

	return &SCIPPerformanceMetrics{
		CacheHitRate:        cacheHitRate,
		CacheMissRate:       cacheMissRate,
		SCIPOverheadMs:      0, // Would be calculated from comparison
		AverageResponseTime: avgResponseTime,
		P95ResponseTime:     p95ResponseTime,
		ThroughputReqPerSec: throughput,
		MethodPerformance:   methodMetrics,
		IndexingPerformance: &IndexingMetrics{
			QueryAverageTime: scipStats.AverageQueryTime,
			IndexMemoryUsageMB: scipStats.MemoryUsage / 1024 / 1024,
		},
		CacheOperationMetrics: &CacheOperationMetrics{
			CacheSize:           scipStats.CacheSize,
			CacheEfficiency:     cacheHitRate * 100,
		},
	}
}

func (test *SCIPPerformanceTest) analyzeCachePerformance(results <-chan *SCIPRequestResult) (float64, *MethodMetrics) {
	var (
		totalRequests         int64
		cacheHits             int64
		totalCacheHitTime     time.Duration
		totalCacheMissTime    time.Duration
		cacheHitCount         int64
		cacheMissCount        int64
	)

	for result := range results {
		totalRequests++
		
		if result.CacheHit {
			cacheHits++
			totalCacheHitTime += result.ResponseTime
			cacheHitCount++
		} else {
			totalCacheMissTime += result.ResponseTime
			cacheMissCount++
		}
	}

	hitRate := float64(cacheHits) / float64(totalRequests)

	var avgCacheHitTime, avgCacheMissTime time.Duration
	if cacheHitCount > 0 {
		avgCacheHitTime = totalCacheHitTime / time.Duration(cacheHitCount)
	}
	if cacheMissCount > 0 {
		avgCacheMissTime = totalCacheMissTime / time.Duration(cacheMissCount)
	}

	metrics := &MethodMetrics{
		Method:                "cache_analysis",
		TotalRequests:         totalRequests,
		CacheHits:             cacheHits,
		CacheMisses:           cacheMissCount,
		CacheHitResponseTime:  avgCacheHitTime,
		CacheMissResponseTime: avgCacheMissTime,
	}

	return hitRate, metrics
}

func (test *SCIPPerformanceTest) calculatePercentile(times []time.Duration, percentile int) time.Duration {
	if len(times) == 0 {
		return 0
	}

	// Simple sorting for percentile calculation
	sorted := make([]time.Duration, len(times))
	copy(sorted, times)

	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	index := (len(sorted) * percentile) / 100
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

// Additional helper methods for comprehensive testing

func (test *SCIPPerformanceTest) measureLSPOnlyPerformanceWithSCIP(ctx context.Context, concurrency int, duration time.Duration) (*SCIPBaselineMetrics, error) {
	// This simulates using LSP-only methods with SCIP infrastructure enabled
	// to detect any overhead from the SCIP system
	results := make(chan *SCIPRequestResult, concurrency*100)
	var wg sync.WaitGroup

	endTime := time.Now().Add(duration)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			test.lspOnlyWithSCIPWorker(workerID, endTime, results)
		}(i)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return test.collectBaselineMetrics(results), nil
}

func (test *SCIPPerformanceTest) lspOnlyWithSCIPWorker(workerID int, endTime time.Time, results chan<- *SCIPRequestResult) {
	for time.Now().Before(endTime) {
		// Use only LSP-only methods that SCIP doesn't support
		method := test.LSPOnlyMethods[rand.Intn(len(test.LSPOnlyMethods))]
		
		startTime := time.Now()
		err := test.simulateLSPRequest(method, true) // With SCIP infrastructure overhead
		responseTime := time.Since(startTime)

		result := &SCIPRequestResult{
			Method:          method,
			Success:         err == nil,
			ResponseTime:    responseTime,
			CacheHit:        false, // LSP-only methods don't use cache
			SCIPUsed:        false,
			Error:           err,
			Timestamp:       startTime,
			MemoryAllocated: test.measureCurrentMemoryUsage(),
		}

		select {
		case results <- result:
		case <-time.After(1 * time.Second):
			// Avoid blocking
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func (test *SCIPPerformanceTest) detectPerformanceRegression(baseline, withSCIP *SCIPBaselineMetrics) *PerformanceRegressionResult {
	throughputRegression := (baseline.ThroughputReqPerSec - withSCIP.ThroughputReqPerSec) / baseline.ThroughputReqPerSec * 100
	responseTimeRegression := float64(withSCIP.P95ResponseTime - baseline.P95ResponseTime) / float64(baseline.P95ResponseTime) * 100

	regressionDetails := make([]string, 0)
	hasRegression := false

	if throughputRegression > 5.0 { // >5% throughput regression
		regressionDetails = append(regressionDetails, fmt.Sprintf("Throughput regression: %.2f%%", throughputRegression))
		hasRegression = true
	}

	if responseTimeRegression > 10.0 { // >10% response time regression
		regressionDetails = append(regressionDetails, fmt.Sprintf("Response time regression: %.2f%%", responseTimeRegression))
		hasRegression = true
	}

	return &PerformanceRegressionResult{
		HasRegression:                 hasRegression,
		ThroughputRegressionPercent:   throughputRegression,
		ResponseTimeRegressionPercent: responseTimeRegression,
		RegressionDetails:            regressionDetails,
	}
}

func (test *SCIPPerformanceTest) measureMemoryUsageUnderLoad(ctx context.Context, concurrency int, duration time.Duration) *MemoryUsageResults {
	initialMemory := test.measureCurrentMemoryUsage()
	var peakMemory int64 = initialMemory

	// Start memory monitoring
	done := make(chan bool)
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				currentMemory := test.measureCurrentMemoryUsage()
				if currentMemory > peakMemory {
					atomic.StoreInt64(&peakMemory, currentMemory)
				}
			case <-done:
				return
			}
		}
	}()

	// Simulate load
	test.measureMixedProtocolPerformance(nil, concurrency, 0.5, duration)

	close(done)

	finalMemory := test.measureCurrentMemoryUsage()
	memoryGrowth := finalMemory - initialMemory

	return &MemoryUsageResults{
		PeakMemoryUsageMB:  atomic.LoadInt64(&peakMemory) / 1024 / 1024,
		MemoryLeakDetected: memoryGrowth > test.MaxMemoryImpact,
		MemoryLeakSeverity: test.classifyMemoryLeakSeverity(memoryGrowth),
	}
}

func (test *SCIPPerformanceTest) classifyMemoryLeakSeverity(growth int64) string {
	if growth <= test.MaxMemoryImpact {
		return ""
	} else if growth <= test.MaxMemoryImpact*2 {
		return "moderate"
	} else {
		return "severe"
	}
}

func (test *SCIPPerformanceTest) measureHighConcurrencyLoad(ctx context.Context, concurrency int, duration time.Duration) (*SCIPHighLoadMetrics, error) {
	results := make(chan *SCIPRequestResult, concurrency*300)
	var wg sync.WaitGroup

	endTime := time.Now().Add(duration)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			test.highLoadWorker(workerID, endTime, results)
		}(i)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return test.collectHighLoadMetrics(results), nil
}

func (test *SCIPPerformanceTest) highLoadWorker(workerID int, endTime time.Time, results chan<- *SCIPRequestResult) {
	for time.Now().Before(endTime) {
		// Mix of SCIP and LSP-only requests
		if rand.Float64() < 0.6 { // 60% SCIP requests
			method := test.SCIPSupportedMethods[rand.Intn(len(test.SCIPSupportedMethods))]
			test.performSCIPRequest(method, results)
		} else {
			method := test.LSPOnlyMethods[rand.Intn(len(test.LSPOnlyMethods))]
			startTime := time.Now()
			err := test.simulateLSPRequest(method, true)
			responseTime := time.Since(startTime)

			result := &SCIPRequestResult{
				Method:          method,
				Success:         err == nil,
				ResponseTime:    responseTime,
				CacheHit:        false,
				SCIPUsed:        false,
				Error:           err,
				Timestamp:       startTime,
				MemoryAllocated: test.measureCurrentMemoryUsage(),
			}

			select {
			case results <- result:
			case <-time.After(500 * time.Millisecond):
				// Avoid blocking under high load
			}
		}

		time.Sleep(5 * time.Millisecond) // Minimal delay for high load
	}
}

func (test *SCIPPerformanceTest) collectHighLoadMetrics(results <-chan *SCIPRequestResult) *SCIPHighLoadMetrics {
	scipMetrics := test.collectSCIPMetrics(results)

	return &SCIPHighLoadMetrics{
		ThroughputReqPerSec: scipMetrics.ThroughputReqPerSec,
		P95ResponseTime:     scipMetrics.P95ResponseTime,
		CacheHitRate:        scipMetrics.CacheHitRate,
		SCIPMetrics:         scipMetrics,
	}
}

func (test *SCIPPerformanceTest) performBenchmarkRequests(concurrency int, requestsPerWorker int) error {
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerWorker; j++ {
				method := test.SCIPSupportedMethods[j%len(test.SCIPSupportedMethods)]
				test.simulateLSPRequest(method, true)
			}
		}()
	}

	wg.Wait()
	return nil
}

// Validation methods

func (test *SCIPPerformanceTest) validateBaselinePerformance(t *testing.T, metrics *SCIPBaselineMetrics) {
	// Validate baseline meets enterprise requirements
	if metrics.ThroughputReqPerSec < test.MinThroughputMaintained {
		t.Errorf("Baseline throughput too low: %.2f req/sec < %.2f req/sec",
			metrics.ThroughputReqPerSec, test.MinThroughputMaintained)
	}

	if metrics.P95ResponseTime > 5*time.Second {
		t.Errorf("Baseline P95 response time too high: %v > 5s", metrics.P95ResponseTime)
	}

	if metrics.ErrorRate > 0.05 {
		t.Errorf("Baseline error rate too high: %.2f%% > 5%%", metrics.ErrorRate*100)
	}
}

func (test *SCIPPerformanceTest) validateMixedProtocolPerformance(t *testing.T, comparison *SCIPPerformanceComparison) {
	// Check all requirements are met
	allRequirementsMet := true
	for requirement, met := range comparison.RequirementsMet {
		if !met {
			t.Errorf("SCIP requirement not met: %s", requirement)
			allRequirementsMet = false
		}
	}

	if !allRequirementsMet {
		t.Errorf("SCIP performance requirements not satisfied for scenario: %s", comparison.Scenario)
	}

	// Specific validations
	if comparison.SCIPMetrics.CacheHitRate < test.MinCacheHitRate {
		t.Errorf("Cache hit rate too low: %.2f%% < %.2f%%",
			comparison.SCIPMetrics.CacheHitRate*100, test.MinCacheHitRate*100)
	}

	if comparison.PerformanceRegression {
		t.Errorf("Performance regression detected in scenario: %s", comparison.Scenario)
	}
}

func (test *SCIPPerformanceTest) detectRegressionInComparison(comparison *SCIPPerformanceComparison) bool {
	// Check for throughput regression
	throughputRegression := comparison.SCIPMetrics.ThroughputReqPerSec < comparison.BaselineMetrics.ThroughputReqPerSec*0.95

	// Check for response time regression
	responseTimeRegression := comparison.SCIPMetrics.P95ResponseTime > comparison.BaselineMetrics.P95ResponseTime+test.MaxResponseTimeRegression

	return throughputRegression || responseTimeRegression
}

func (test *SCIPPerformanceTest) calculateMemoryEfficiency(comparison *SCIPPerformanceComparison) float64 {
	if comparison.BaselineMetrics.MemoryUsageMB == 0 {
		return 100.0
	}

	memoryIncrease := float64(comparison.SCIPMetrics.MemoryImpactMB) / float64(comparison.BaselineMetrics.MemoryUsageMB)
	efficiency := 100.0 - (memoryIncrease * 100.0)

	if efficiency < 0 {
		efficiency = 0
	}

	return efficiency
}

func (test *SCIPPerformanceTest) validateHighConcurrencyPerformance(t *testing.T, concurrency int, metrics *SCIPHighLoadMetrics) {
	// Validate performance degrades gracefully under high load
	if metrics.ThroughputReqPerSec < test.MinThroughputMaintained*0.8 { // Allow 20% degradation
		t.Errorf("High load throughput too low at C=%d: %.2f req/sec < %.2f req/sec",
			concurrency, metrics.ThroughputReqPerSec, test.MinThroughputMaintained*0.8)
	}

	if metrics.P95ResponseTime > 10*time.Second { // Allow higher response time under extreme load
		t.Errorf("High load P95 response time too high at C=%d: %v > 10s", concurrency, metrics.P95ResponseTime)
	}

	// Cache should still be effective under high load
	if metrics.CacheHitRate < test.MinCacheHitRate*0.5 { // Allow 50% cache effectiveness reduction
		t.Errorf("High load cache hit rate too low at C=%d: %.2f%% < %.2f%%",
			concurrency, metrics.CacheHitRate*100, test.MinCacheHitRate*0.5*100)
	}
}

func (test *SCIPPerformanceTest) cleanup() {
	if test.scipStore != nil {
		test.scipStore.Close()
	}
	if test.framework != nil {
		test.framework.CleanupAll()
	}
}

// Additional types for comprehensive testing

type MemoryUsageResults struct {
	PeakMemoryUsageMB  int64  `json:"peak_memory_usage_mb"`
	MemoryLeakDetected bool   `json:"memory_leak_detected"`
	MemoryLeakSeverity string `json:"memory_leak_severity,omitempty"`
}

// Integration tests

func TestSCIPPerformanceSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP performance tests in short mode")
	}

	perfTest := NewSCIPPerformanceTest(t)

	// Test sequence is important - baseline must be established first
	t.Run("SCIPPerformanceBaseline", perfTest.TestSCIPPerformanceBaseline)
	t.Run("SCIPCacheHitRate", perfTest.TestSCIPCacheHitRate)  
	t.Run("SCIPMemoryImpact", perfTest.TestSCIPMemoryImpact)
	t.Run("SCIPMixedProtocolPerformance", perfTest.TestSCIPMixedProtocolPerformance)
	t.Run("SCIPPerformanceRegression", perfTest.TestSCIPPerformanceRegression)
	t.Run("SCIPConcurrentLoadCapacity", perfTest.TestSCIPConcurrentLoadCapacity)
}

func BenchmarkSCIPPerformance(b *testing.B) {
	perfTest := NewSCIPPerformanceTest(nil)
	perfTest.BenchmarkSCIPPerformance(b)
}