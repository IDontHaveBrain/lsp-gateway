package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"

	"lsp-gateway/internal/testing/lsp/cases"
	"lsp-gateway/internal/testing/lsp/config"
	"lsp-gateway/internal/testing/lsp/runner"
)

// LSPBenchmarkFramework provides performance benchmarking for LSP validation
type LSPBenchmarkFramework struct {
	framework *LSPTestFramework
	config    *BenchmarkConfig
	logger    TestLogger
}

// BenchmarkConfig defines performance benchmarking configuration
type BenchmarkConfig struct {
	// Performance targets
	LatencyThresholds    LatencyThresholds `yaml:"latency_thresholds"`
	ThroughputThresholds ThroughputThresholds `yaml:"throughput_thresholds"`
	MemoryThresholds     MemoryThresholds     `yaml:"memory_thresholds"`
	
	// Test configuration
	WarmupIterations   int           `yaml:"warmup_iterations"`
	BenchmarkDuration  time.Duration `yaml:"benchmark_duration"`
	ConcurrencyLevels  []int         `yaml:"concurrency_levels"`
	SamplingRate       time.Duration `yaml:"sampling_rate"`
	
	// Reporting
	EnableCSVOutput    bool   `yaml:"enable_csv_output"`
	EnableReporting    bool   `yaml:"enable_reporting"`
	OutputDir          string `yaml:"output_dir"`
	DetectRegressions  bool   `yaml:"detect_regressions"`
}

// LatencyThresholds defines acceptable latency limits for each LSP method
type LatencyThresholds struct {
	Definition      MethodThresholds `yaml:"definition"`
	References      MethodThresholds `yaml:"references"`
	Hover          MethodThresholds `yaml:"hover"`
	DocumentSymbol MethodThresholds `yaml:"document_symbol"`
	WorkspaceSymbol MethodThresholds `yaml:"workspace_symbol"`
}

// MethodThresholds defines latency thresholds for a specific method
type MethodThresholds struct {
	P50 time.Duration `yaml:"p50"`
	P95 time.Duration `yaml:"p95"`
	P99 time.Duration `yaml:"p99"`
	Max time.Duration `yaml:"max"`
}

// ThroughputThresholds defines minimum acceptable throughput
type ThroughputThresholds struct {
	MinRequestsPerSecond float64 `yaml:"min_requests_per_second"`
	MaxErrorRate         float64 `yaml:"max_error_rate"`
}

// MemoryThresholds defines acceptable memory usage limits
type MemoryThresholds struct {
	MaxAllocPerRequest int64   `yaml:"max_alloc_per_request"` // bytes
	MaxMemoryGrowth    float64 `yaml:"max_memory_growth"`     // percentage
	MaxGCPause         time.Duration `yaml:"max_gc_pause"`
}

// BenchmarkResult contains performance benchmark results
type BenchmarkResult struct {
	Method              string                `json:"method"`
	TotalRequests       int64                 `json:"total_requests"`
	Duration            time.Duration         `json:"duration"`
	ThroughputRPS       float64              `json:"throughput_rps"`
	ErrorCount          int64                 `json:"error_count"`
	ErrorRate           float64              `json:"error_rate"`
	LatencyMetrics      LatencyMetrics       `json:"latency_metrics"`
	MemoryMetrics       MemoryMetrics        `json:"memory_metrics"`
	ConcurrencyResults  []ConcurrencyResult  `json:"concurrency_results"`
	ThresholdResults    ThresholdResults     `json:"threshold_results"`
	Timestamp           time.Time            `json:"timestamp"`
}

// LatencyMetrics contains latency statistics
type LatencyMetrics struct {
	Average time.Duration `json:"average"`
	Min     time.Duration `json:"min"`
	Max     time.Duration `json:"max"`
	P50     time.Duration `json:"p50"`
	P95     time.Duration `json:"p95"`
	P99     time.Duration `json:"p99"`
}

// MemoryMetrics contains memory usage statistics
type MemoryMetrics struct {
	AllocPerRequest     int64   `json:"alloc_per_request"`    // bytes
	TotalAlloc          int64   `json:"total_alloc"`          // bytes
	PeakMemoryUsage     int64   `json:"peak_memory_usage"`    // bytes
	MemoryGrowthPercent float64 `json:"memory_growth_percent"`
	GCCount             uint32  `json:"gc_count"`
	TotalGCPause        time.Duration `json:"total_gc_pause"`
	AvgGCPause          time.Duration `json:"avg_gc_pause"`
}

// ConcurrencyResult contains results for a specific concurrency level
type ConcurrencyResult struct {
	ConcurrentUsers int             `json:"concurrent_users"`
	ThroughputRPS   float64         `json:"throughput_rps"`
	LatencyMetrics  LatencyMetrics  `json:"latency_metrics"`
	ErrorRate       float64         `json:"error_rate"`
}

// ThresholdResults indicates which thresholds passed/failed
type ThresholdResults struct {
	LatencyPassed    bool   `json:"latency_passed"`
	ThroughputPassed bool   `json:"throughput_passed"`
	MemoryPassed     bool   `json:"memory_passed"`
	OverallPassed    bool   `json:"overall_passed"`
	FailureReasons   []string `json:"failure_reasons"`
}

// BenchmarkRunner executes performance benchmarks for a specific LSP method
type BenchmarkRunner struct {
	method       string
	framework    *LSPTestFramework
	config       *BenchmarkConfig
	logger       TestLogger
	latencies    []time.Duration
	memSamples   []MemorySample
	errors       int64
	mu           sync.RWMutex
}

// MemorySample represents a memory usage sample
type MemorySample struct {
	Timestamp time.Time
	Alloc     uint64
	TotalAlloc uint64
	Sys       uint64
	GCCount   uint32
}

// NewLSPBenchmarkFramework creates a new benchmark framework
func NewLSPBenchmarkFramework(options *FrameworkOptions, benchmarkConfig *BenchmarkConfig) (*LSPBenchmarkFramework, error) {
	// Create underlying LSP test framework
	framework, err := NewLSPTestFramework(options)
	if err != nil {
		return nil, fmt.Errorf("failed to create LSP framework: %w", err)
	}
	
	// Set default benchmark config if not provided
	if benchmarkConfig == nil {
		benchmarkConfig = DefaultBenchmarkConfig()
	}
	
	logger := NewSimpleTestLogger(options.Verbose)
	if options.LogTiming {
		logger = NewTimedLogger(logger)
	}
	
	return &LSPBenchmarkFramework{
		framework: framework,
		config:    benchmarkConfig,
		logger:    logger,
	}, nil
}

// DefaultBenchmarkConfig returns default benchmark configuration
func DefaultBenchmarkConfig() *BenchmarkConfig {
	return &BenchmarkConfig{
		LatencyThresholds: LatencyThresholds{
			Definition: MethodThresholds{
				P50: 50 * time.Millisecond,
				P95: 100 * time.Millisecond,
				P99: 200 * time.Millisecond,
				Max: 500 * time.Millisecond,
			},
			References: MethodThresholds{
				P50: 100 * time.Millisecond,
				P95: 200 * time.Millisecond,
				P99: 500 * time.Millisecond,
				Max: 1000 * time.Millisecond,
			},
			Hover: MethodThresholds{
				P50: 30 * time.Millisecond,
				P95: 50 * time.Millisecond,
				P99: 100 * time.Millisecond,
				Max: 200 * time.Millisecond,
			},
			DocumentSymbol: MethodThresholds{
				P50: 100 * time.Millisecond,
				P95: 300 * time.Millisecond,
				P99: 500 * time.Millisecond,
				Max: 1000 * time.Millisecond,
			},
			WorkspaceSymbol: MethodThresholds{
				P50: 200 * time.Millisecond,
				P95: 500 * time.Millisecond,
				P99: 1000 * time.Millisecond,
				Max: 2000 * time.Millisecond,
			},
		},
		ThroughputThresholds: ThroughputThresholds{
			MinRequestsPerSecond: 10.0,
			MaxErrorRate:         0.05, // 5%
		},
		MemoryThresholds: MemoryThresholds{
			MaxAllocPerRequest: 1024 * 1024,      // 1MB per request
			MaxMemoryGrowth:    20.0,             // 20% growth
			MaxGCPause:         10 * time.Millisecond,
		},
		WarmupIterations:  100,
		BenchmarkDuration: 30 * time.Second,
		ConcurrencyLevels: []int{1, 5, 10, 20},
		SamplingRate:      100 * time.Millisecond,
		EnableCSVOutput:   true,
		EnableReporting:   true,
		OutputDir:        "benchmark-results",
		DetectRegressions: true,
	}
}

// BenchmarkAllMethods runs performance benchmarks for all LSP methods
func (bf *LSPBenchmarkFramework) BenchmarkAllMethods(ctx context.Context, b *testing.B) map[string]*BenchmarkResult {
	results := make(map[string]*BenchmarkResult)
	
	methods := []string{
		cases.LSPMethodDefinition,
		cases.LSPMethodReferences,
		cases.LSPMethodHover,
		cases.LSPMethodDocumentSymbol,
		cases.LSPMethodWorkspaceSymbol,
	}
	
	for _, method := range methods {
		bf.logger.Info("Starting benchmark for method: %s", method)
		result := bf.BenchmarkMethod(ctx, b, method)
		results[method] = result
		bf.logger.Info("Completed benchmark for method: %s", method)
	}
	
	return results
}

// BenchmarkMethod runs performance benchmark for a specific LSP method
func (bf *LSPBenchmarkFramework) BenchmarkMethod(ctx context.Context, b *testing.B, method string) *BenchmarkResult {
	runner := &BenchmarkRunner{
		method:    method,
		framework: bf.framework,
		config:    bf.config,
		logger:    bf.logger,
		latencies: make([]time.Duration, 0),
		memSamples: make([]MemorySample, 0),
	}
	
	return runner.Run(ctx, b)
}

// Run executes the benchmark for a specific method
func (br *BenchmarkRunner) Run(ctx context.Context, b *testing.B) *BenchmarkResult {
	result := &BenchmarkResult{
		Method:    br.method,
		Timestamp: time.Now(),
	}
	
	// Warmup phase
	br.logger.Info("Starting warmup phase for %s (%d iterations)", br.method, br.config.WarmupIterations)
	br.warmup(ctx)
	
	// Main benchmark phase
	br.logger.Info("Starting main benchmark phase for %s", br.method)
	br.runMainBenchmark(ctx, b, result)
	
	// Concurrency benchmarks
	br.logger.Info("Starting concurrency benchmarks for %s", br.method)
	result.ConcurrencyResults = br.runConcurrencyBenchmarks(ctx)
	
	// Calculate final metrics
	br.calculateMetrics(result)
	
	// Validate against thresholds
	result.ThresholdResults = br.validateThresholds(result)
	
	return result
}

// warmup performs warmup iterations to stabilize performance
func (br *BenchmarkRunner) warmup(ctx context.Context) {
	for i := 0; i < br.config.WarmupIterations; i++ {
		_, _ = br.executeTestCase(ctx)
	}
	
	// Clear memory and force GC after warmup
	runtime.GC()
}

// runMainBenchmark executes the main benchmark phase
func (br *BenchmarkRunner) runMainBenchmark(ctx context.Context, b *testing.B, result *BenchmarkResult) {
	start := time.Now()
	var requestCount int64
	
	// Start memory sampling
	stopSampling := br.startMemorySampling()
	defer stopSampling()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		latency, err := br.executeTestCase(ctx)
		
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
func (br *BenchmarkRunner) runConcurrencyBenchmarks(ctx context.Context) []ConcurrencyResult {
	var results []ConcurrencyResult
	
	for _, concurrency := range br.config.ConcurrencyLevels {
		br.logger.Info("Testing concurrency level: %d", concurrency)
		result := br.runConcurrencyTest(ctx, concurrency)
		results = append(results, result)
	}
	
	return results
}

// runConcurrencyTest tests performance at a specific concurrency level
func (br *BenchmarkRunner) runConcurrencyTest(ctx context.Context, concurrency int) ConcurrencyResult {
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
					latency, err := br.executeTestCase(testCtx)
					
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
	
	return ConcurrencyResult{
		ConcurrentUsers: concurrency,
		ThroughputRPS:   float64(requestCount) / actualDuration.Seconds(),
		LatencyMetrics:  calculateLatencyMetrics(latencies),
		ErrorRate:       float64(errorCount) / float64(requestCount),
	}
}

// executeTestCase executes a single test case and measures performance
func (br *BenchmarkRunner) executeTestCase(ctx context.Context) (time.Duration, error) {
	// Create a simple test case for the method
	testCase := br.createTestCase()
	
	start := time.Now()
	
	// Execute the test case (simplified version)
	runOptions := runner.DefaultRunOptions()
	runOptions.Filter = &cases.TestCaseFilter{
		Methods: []string{br.method},
	}
	
	_, err := br.framework.runner.Run(ctx, runOptions)
	
	latency := time.Since(start)
	
	return latency, err
}

// createTestCase creates a simple test case for benchmarking
func (br *BenchmarkRunner) createTestCase() *cases.TestCase {
	return &cases.TestCase{
		ID:       fmt.Sprintf("benchmark_%s", br.method),
		Name:     fmt.Sprintf("Benchmark %s", br.method),
		Method:   br.method,
		Language: "go",
		Position: &config.Position{Line: 10, Character: 5},
		Config: &config.TestCaseConfig{
			File: "sample.go",
		},
	}
}

// startMemorySampling starts periodic memory sampling
func (br *BenchmarkRunner) startMemorySampling() func() {
	stopCh := make(chan struct{})
	
	go func() {
		ticker := time.NewTicker(br.config.SamplingRate)
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
	
	sample := MemorySample{
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
func (br *BenchmarkRunner) calculateMetrics(result *BenchmarkResult) {
	// Calculate latency metrics
	result.LatencyMetrics = calculateLatencyMetrics(br.latencies)
	
	// Calculate memory metrics
	result.MemoryMetrics = br.calculateMemoryMetrics()
}

// calculateLatencyMetrics calculates latency statistics
func calculateLatencyMetrics(latencies []time.Duration) LatencyMetrics {
	if len(latencies) == 0 {
		return LatencyMetrics{}
	}
	
	// Sort latencies for percentile calculation
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})
	
	var total time.Duration
	for _, lat := range latencies {
		total += lat
	}
	
	return LatencyMetrics{
		Average: total / time.Duration(len(latencies)),
		Min:     latencies[0],
		Max:     latencies[len(latencies)-1],
		P50:     latencies[len(latencies)*50/100],
		P95:     latencies[len(latencies)*95/100],
		P99:     latencies[len(latencies)*99/100],
	}
}

// calculateMemoryMetrics calculates memory usage statistics
func (br *BenchmarkRunner) calculateMemoryMetrics() MemoryMetrics {
	if len(br.memSamples) == 0 {
		return MemoryMetrics{}
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
	
	return MemoryMetrics{
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
func (br *BenchmarkRunner) validateThresholds(result *BenchmarkResult) ThresholdResults {
	thresholdResult := ThresholdResults{
		LatencyPassed:    true,
		ThroughputPassed: true,
		MemoryPassed:     true,
	}
	
	// Get method-specific thresholds
	var methodThresholds MethodThresholds
	switch br.method {
	case cases.LSPMethodDefinition:
		methodThresholds = br.config.LatencyThresholds.Definition
	case cases.LSPMethodReferences:
		methodThresholds = br.config.LatencyThresholds.References
	case cases.LSPMethodHover:
		methodThresholds = br.config.LatencyThresholds.Hover
	case cases.LSPMethodDocumentSymbol:
		methodThresholds = br.config.LatencyThresholds.DocumentSymbol
	case cases.LSPMethodWorkspaceSymbol:
		methodThresholds = br.config.LatencyThresholds.WorkspaceSymbol
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
	if result.ThroughputRPS < br.config.ThroughputThresholds.MinRequestsPerSecond {
		thresholdResult.ThroughputPassed = false
		thresholdResult.FailureReasons = append(thresholdResult.FailureReasons, 
			fmt.Sprintf("Throughput %.2f RPS below threshold %.2f RPS", 
				result.ThroughputRPS, br.config.ThroughputThresholds.MinRequestsPerSecond))
	}
	
	if result.ErrorRate > br.config.ThroughputThresholds.MaxErrorRate {
		thresholdResult.ThroughputPassed = false
		thresholdResult.FailureReasons = append(thresholdResult.FailureReasons, 
			fmt.Sprintf("Error rate %.2f%% exceeds threshold %.2f%%", 
				result.ErrorRate*100, br.config.ThroughputThresholds.MaxErrorRate*100))
	}
	
	// Validate memory thresholds
	if result.MemoryMetrics.AllocPerRequest > br.config.MemoryThresholds.MaxAllocPerRequest {
		thresholdResult.MemoryPassed = false
		thresholdResult.FailureReasons = append(thresholdResult.FailureReasons, 
			fmt.Sprintf("Memory allocation per request %d bytes exceeds threshold %d bytes", 
				result.MemoryMetrics.AllocPerRequest, br.config.MemoryThresholds.MaxAllocPerRequest))
	}
	
	if result.MemoryMetrics.MemoryGrowthPercent > br.config.MemoryThresholds.MaxMemoryGrowth {
		thresholdResult.MemoryPassed = false
		thresholdResult.FailureReasons = append(thresholdResult.FailureReasons, 
			fmt.Sprintf("Memory growth %.1f%% exceeds threshold %.1f%%", 
				result.MemoryMetrics.MemoryGrowthPercent, br.config.MemoryThresholds.MaxMemoryGrowth))
	}
	
	thresholdResult.OverallPassed = thresholdResult.LatencyPassed && 
		thresholdResult.ThroughputPassed && 
		thresholdResult.MemoryPassed
	
	return thresholdResult
}