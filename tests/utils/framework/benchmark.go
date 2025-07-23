package framework

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"

	"lsp-gateway/tests/utils/framework/cases"
	"lsp-gateway/tests/utils/framework/runner"
	"lsp-gateway/tests/utils/framework/types"
)

// LSPBenchmarkFramework provides performance benchmarking for LSP validation
type LSPBenchmarkFramework struct {
	Framework *LSPTestFramework
	Config    *types.BenchmarkConfig
	Logger    TestLogger
}

// BenchmarkRunner executes performance benchmarks for a specific LSP method
type BenchmarkRunner struct {
	Method     string
	Framework  *LSPTestFramework
	Config     *types.BenchmarkConfig
	Logger     TestLogger
	latencies  []time.Duration
	memSamples []types.MemorySample
	errors     int64
	mu         sync.RWMutex
}

// NewLSPBenchmarkFramework creates a new benchmark framework
func NewLSPBenchmarkFramework(options *FrameworkOptions, benchmarkConfig *types.BenchmarkConfig) (*LSPBenchmarkFramework, error) {
	// Create underlying LSP test framework
	framework, err := NewLSPTestFramework(options)
	if err != nil {
		return nil, fmt.Errorf("failed to create LSP framework: %w", err)
	}

	// Set default benchmark config if not provided
	if benchmarkConfig == nil {
		benchmarkConfig = DefaultBenchmarkConfig()
	}

	var logger TestLogger
	logger = NewSimpleTestLogger(options.Verbose)
	if options.LogTiming {
		logger = NewTimedLogger(logger)
	}

	return &LSPBenchmarkFramework{
		Framework: framework,
		Config:    benchmarkConfig,
		Logger:    logger,
	}, nil
}

// DefaultBenchmarkConfig returns default benchmark configuration
func DefaultBenchmarkConfig() *types.BenchmarkConfig {
	return &types.BenchmarkConfig{
		LatencyThresholds: types.LatencyThresholds{
			Definition: types.MethodThresholds{
				P50: 50 * time.Millisecond,
				P95: 100 * time.Millisecond,
				P99: 200 * time.Millisecond,
				Max: 500 * time.Millisecond,
			},
			References: types.MethodThresholds{
				P50: 100 * time.Millisecond,
				P95: 200 * time.Millisecond,
				P99: 500 * time.Millisecond,
				Max: 1000 * time.Millisecond,
			},
			Hover: types.MethodThresholds{
				P50: 30 * time.Millisecond,
				P95: 50 * time.Millisecond,
				P99: 100 * time.Millisecond,
				Max: 200 * time.Millisecond,
			},
			DocumentSymbol: types.MethodThresholds{
				P50: 100 * time.Millisecond,
				P95: 300 * time.Millisecond,
				P99: 500 * time.Millisecond,
				Max: 1000 * time.Millisecond,
			},
			WorkspaceSymbol: types.MethodThresholds{
				P50: 200 * time.Millisecond,
				P95: 500 * time.Millisecond,
				P99: 1000 * time.Millisecond,
				Max: 2000 * time.Millisecond,
			},
		},
		ThroughputThresholds: types.ThroughputThresholds{
			MinRequestsPerSecond: 10.0,
			MaxErrorRate:         0.05, // 5%
		},
		MemoryThresholds: types.MemoryThresholds{
			MaxAllocPerRequest: 1024 * 1024, // 1MB per request
			MaxMemoryGrowth:    20.0,        // 20% growth
			MaxGCPause:         10 * time.Millisecond,
		},
		WarmupIterations:  100,
		BenchmarkDuration: 30 * time.Second,
		ConcurrencyLevels: []int{1, 5, 10, 20},
		SamplingRate:      100 * time.Millisecond,
		EnableCSVOutput:   true,
		EnableReporting:   true,
		OutputDir:         "benchmark-results",
		DetectRegressions: true,
	}
}

// BenchmarkMethod runs performance benchmark for a specific LSP method
func (bf *LSPBenchmarkFramework) BenchmarkMethod(ctx context.Context, b *testing.B, method string) *types.BenchmarkResult {
	runner := &BenchmarkRunner{
		Method:     method,
		Framework:  bf.Framework,
		Config:     bf.Config,
		Logger:     bf.Logger,
		latencies:  make([]time.Duration, 0),
		memSamples: make([]types.MemorySample, 0),
	}

	return runner.Run(ctx, b)
}

// Run executes the benchmark for a specific method
func (br *BenchmarkRunner) Run(ctx context.Context, b *testing.B) *types.BenchmarkResult {
	result := &types.BenchmarkResult{
		Method:    br.Method,
		Timestamp: time.Now(),
	}

	// Warmup phase
	br.Logger.Info("Starting warmup phase for %s (%d iterations)", br.Method, br.Config.WarmupIterations)
	br.warmup(ctx)

	// Main benchmark phase
	br.Logger.Info("Starting main benchmark phase for %s", br.Method)
	br.runMainBenchmark(ctx, b, result)

	// Concurrency benchmarks
	br.Logger.Info("Starting concurrency benchmarks for %s", br.Method)
	result.ConcurrencyResults = br.runConcurrencyBenchmarks(ctx)

	// Calculate final metrics
	br.calculateMetrics(result)

	// Validate against thresholds
	result.ThresholdResults = br.validateThresholds(result)

	return result
}

// warmup performs warmup iterations to stabilize performance
func (br *BenchmarkRunner) warmup(ctx context.Context) {
	for i := 0; i < br.Config.WarmupIterations; i++ {
		_, _ = br.ExecuteTestCase(ctx)
	}

	// Clear memory and force GC after warmup
	runtime.GC()
}

// runMainBenchmark executes the main benchmark phase
func (br *BenchmarkRunner) runMainBenchmark(ctx context.Context, b *testing.B, result *types.BenchmarkResult) {
	start := time.Now()
	var requestCount int64

	// Start memory sampling
	stopSampling := br.startMemorySampling()
	defer stopSampling()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		latency, err := br.ExecuteTestCase(ctx)

		br.mu.Lock()
		br.latencies = append(br.latencies, latency)
		if err != nil {
			br.errors++
		}
		br.mu.Unlock()

		requestCount++
	}

	result.Duration = time.Since(start)
	result.TotalRequests = requestCount
	result.ErrorCount = br.errors
	result.ThroughputRPS = float64(requestCount) / result.Duration.Seconds()
	result.ErrorRate = float64(br.errors) / float64(requestCount)
}

// runConcurrencyBenchmarks tests different concurrency levels
func (br *BenchmarkRunner) runConcurrencyBenchmarks(ctx context.Context) []types.ConcurrencyResult {
	var results []types.ConcurrencyResult

	for _, concurrency := range br.Config.ConcurrencyLevels {
		br.Logger.Info("Testing concurrency level: %d", concurrency)
		result := br.RunConcurrencyTest(ctx, concurrency)
		results = append(results, result)
	}

	return results
}

// RunConcurrencyTest tests performance at a specific concurrency level
func (br *BenchmarkRunner) RunConcurrencyTest(ctx context.Context, concurrency int) types.ConcurrencyResult {
	var wg sync.WaitGroup
	var requestCount, errorCount int64
	var latencies []time.Duration
	var mu sync.Mutex

	duration := 10 * time.Second // Fixed duration for concurrency tests
	testCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-testCtx.Done():
					return
				default:
					latency, err := br.ExecuteTestCase(testCtx)

					mu.Lock()
					latencies = append(latencies, latency)
					requestCount++
					if err != nil {
						errorCount++
					}
					mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()
	actualDuration := time.Since(start)

	return types.ConcurrencyResult{
		ConcurrentUsers: concurrency,
		ThroughputRPS:   float64(requestCount) / actualDuration.Seconds(),
		LatencyMetrics:  CalculateLatencyMetrics(latencies),
		ErrorRate:       float64(errorCount) / float64(requestCount),
	}
}

// ExecuteTestCase executes a single test case and measures performance
func (br *BenchmarkRunner) ExecuteTestCase(ctx context.Context) (time.Duration, error) {
	start := time.Now()

	// Execute the test case (simplified version)
	runOptions := runner.DefaultRunOptions()
	runOptions.Filter = &cases.TestCaseFilter{
		Methods: []string{br.Method},
	}

	_, err := br.Framework.runner.Run(ctx, runOptions)

	latency := time.Since(start)

	return latency, err
}

// startMemorySampling starts periodic memory sampling
func (br *BenchmarkRunner) startMemorySampling() func() {
	stopCh := make(chan struct{})

	go func() {
		ticker := time.NewTicker(br.Config.SamplingRate)
		defer ticker.Stop()

		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				br.takeSample()
			}
		}
	}()

	return func() { close(stopCh) }
}

// takeSample takes a memory usage sample
func (br *BenchmarkRunner) takeSample() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	sample := types.MemorySample{
		Timestamp:  time.Now(),
		Alloc:      m.Alloc,
		TotalAlloc: m.TotalAlloc,
		Sys:        m.Sys,
		GCCount:    m.NumGC,
	}

	br.mu.Lock()
	br.memSamples = append(br.memSamples, sample)
	br.mu.Unlock()
}

// calculateMetrics calculates final performance metrics
func (br *BenchmarkRunner) calculateMetrics(result *types.BenchmarkResult) {
	// Calculate latency metrics
	result.LatencyMetrics = CalculateLatencyMetrics(br.latencies)

	// Calculate memory metrics
	result.MemoryMetrics = br.calculateMemoryMetrics()
}

// CalculateLatencyMetrics calculates latency statistics
func CalculateLatencyMetrics(latencies []time.Duration) types.LatencyMetrics {
	if len(latencies) == 0 {
		return types.LatencyMetrics{}
	}

	// Sort latencies for percentile calculation
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	var total time.Duration
	for _, lat := range latencies {
		total += lat
	}

	return types.LatencyMetrics{
		Average: total / time.Duration(len(latencies)),
		Min:     latencies[0],
		Max:     latencies[len(latencies)-1],
		P50:     latencies[len(latencies)*50/100],
		P95:     latencies[len(latencies)*95/100],
		P99:     latencies[len(latencies)*99/100],
	}
}

// calculateMemoryMetrics calculates memory usage statistics
func (br *BenchmarkRunner) calculateMemoryMetrics() types.MemoryMetrics {
	if len(br.memSamples) == 0 {
		return types.MemoryMetrics{}
	}

	first := br.memSamples[0]
	last := br.memSamples[len(br.memSamples)-1]

	var peak uint64
	for _, sample := range br.memSamples {
		if sample.Alloc > peak {
			peak = sample.Alloc
		}
	}

	totalGCPause := time.Duration(0) // Simplified - would need more detailed GC tracking
	avgGCPause := time.Duration(0)
	gcCount := last.GCCount - first.GCCount

	if gcCount > 0 {
		avgGCPause = totalGCPause / time.Duration(gcCount)
	}

	var allocPerRequest int64
	if len(br.latencies) > 0 {
		allocPerRequest = int64(last.TotalAlloc-first.TotalAlloc) / int64(len(br.latencies))
	}

	var memoryGrowthPercent float64
	if first.Alloc > 0 {
		memoryGrowthPercent = float64(last.Alloc-first.Alloc) / float64(first.Alloc) * 100
	}

	return types.MemoryMetrics{
		AllocPerRequest:     allocPerRequest,
		TotalAlloc:          int64(last.TotalAlloc - first.TotalAlloc),
		PeakMemoryUsage:     int64(peak),
		MemoryGrowthPercent: memoryGrowthPercent,
		GCCount:             gcCount,
		TotalGCPause:        totalGCPause,
		AvgGCPause:          avgGCPause,
	}
}

// validateThresholds validates results against configured thresholds
func (br *BenchmarkRunner) validateThresholds(result *types.BenchmarkResult) types.ThresholdResults {
	thresholdResult := types.ThresholdResults{
		LatencyPassed:    true,
		ThroughputPassed: true,
		MemoryPassed:     true,
	}

	// Get method-specific thresholds
	var methodThresholds types.MethodThresholds
	switch br.Method {
	case cases.LSPMethodDefinition:
		methodThresholds = br.Config.LatencyThresholds.Definition
	case cases.LSPMethodReferences:
		methodThresholds = br.Config.LatencyThresholds.References
	case cases.LSPMethodHover:
		methodThresholds = br.Config.LatencyThresholds.Hover
	case cases.LSPMethodDocumentSymbol:
		methodThresholds = br.Config.LatencyThresholds.DocumentSymbol
	case cases.LSPMethodWorkspaceSymbol:
		methodThresholds = br.Config.LatencyThresholds.WorkspaceSymbol
	}

	// Validate latency thresholds
	if methodThresholds.P50 > 0 && result.LatencyMetrics.P50 > methodThresholds.P50 {
		thresholdResult.LatencyPassed = false
		thresholdResult.FailureReasons = append(thresholdResult.FailureReasons,
			fmt.Sprintf("P50 latency %v exceeds threshold %v", result.LatencyMetrics.P50, methodThresholds.P50))
	}

	if methodThresholds.P95 > 0 && result.LatencyMetrics.P95 > methodThresholds.P95 {
		thresholdResult.LatencyPassed = false
		thresholdResult.FailureReasons = append(thresholdResult.FailureReasons,
			fmt.Sprintf("P95 latency %v exceeds threshold %v", result.LatencyMetrics.P95, methodThresholds.P95))
	}

	if methodThresholds.P99 > 0 && result.LatencyMetrics.P99 > methodThresholds.P99 {
		thresholdResult.LatencyPassed = false
		thresholdResult.FailureReasons = append(thresholdResult.FailureReasons,
			fmt.Sprintf("P99 latency %v exceeds threshold %v", result.LatencyMetrics.P99, methodThresholds.P99))
	}

	// Validate throughput thresholds
	if result.ThroughputRPS < br.Config.ThroughputThresholds.MinRequestsPerSecond {
		thresholdResult.ThroughputPassed = false
		thresholdResult.FailureReasons = append(thresholdResult.FailureReasons,
			fmt.Sprintf("Throughput %.2f RPS below threshold %.2f RPS",
				result.ThroughputRPS, br.Config.ThroughputThresholds.MinRequestsPerSecond))
	}

	if result.ErrorRate > br.Config.ThroughputThresholds.MaxErrorRate {
		thresholdResult.ThroughputPassed = false
		thresholdResult.FailureReasons = append(thresholdResult.FailureReasons,
			fmt.Sprintf("Error rate %.2f%% exceeds threshold %.2f%%",
				result.ErrorRate*100, br.Config.ThroughputThresholds.MaxErrorRate*100))
	}

	// Validate memory thresholds
	if result.MemoryMetrics.AllocPerRequest > br.Config.MemoryThresholds.MaxAllocPerRequest {
		thresholdResult.MemoryPassed = false
		thresholdResult.FailureReasons = append(thresholdResult.FailureReasons,
			fmt.Sprintf("Memory allocation per request %d bytes exceeds threshold %d bytes",
				result.MemoryMetrics.AllocPerRequest, br.Config.MemoryThresholds.MaxAllocPerRequest))
	}

	if result.MemoryMetrics.MemoryGrowthPercent > br.Config.MemoryThresholds.MaxMemoryGrowth {
		thresholdResult.MemoryPassed = false
		thresholdResult.FailureReasons = append(thresholdResult.FailureReasons,
			fmt.Sprintf("Memory growth %.1f%% exceeds threshold %.1f%%",
				result.MemoryMetrics.MemoryGrowthPercent, br.Config.MemoryThresholds.MaxMemoryGrowth))
	}

	thresholdResult.OverallPassed = thresholdResult.LatencyPassed &&
		thresholdResult.ThroughputPassed &&
		thresholdResult.MemoryPassed

	return thresholdResult
}
