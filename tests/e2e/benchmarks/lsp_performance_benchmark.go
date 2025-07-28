package benchmarks

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"lsp-gateway/tests/e2e/fixtures"
	"lsp-gateway/tests/e2e/testutils"
)

// LSPBenchmarkConfig defines configuration for LSP performance benchmarks
type LSPBenchmarkConfig struct {
	ConcurrentRequests   int
	RequestsPerMethod    int
	TimeoutPerRequest    time.Duration
	WarmupRequests       int
	MeasurementDuration  time.Duration
	EnableDetailedMetrics bool
}

// DefaultLSPBenchmarkConfig returns a default benchmark configuration
func DefaultLSPBenchmarkConfig() LSPBenchmarkConfig {
	return LSPBenchmarkConfig{
		ConcurrentRequests:   5,
		RequestsPerMethod:    50,
		TimeoutPerRequest:    5 * time.Second,
		WarmupRequests:       10,
		MeasurementDuration:  30 * time.Second,
		EnableDetailedMetrics: true,
	}
}

// LSPMethodBenchmark represents benchmark results for a specific LSP method
type LSPMethodBenchmark struct {
	Method            string        `json:"method"`
	TotalRequests     int           `json:"total_requests"`
	SuccessfulReqs    int           `json:"successful_requests"`
	FailedRequests    int           `json:"failed_requests"`
	AverageLatency    time.Duration `json:"average_latency"`
	MinLatency        time.Duration `json:"min_latency"`
	MaxLatency        time.Duration `json:"max_latency"`
	P95Latency        time.Duration `json:"p95_latency"`
	P99Latency        time.Duration `json:"p99_latency"`
	Throughput        float64       `json:"throughput"` // requests per second
	ErrorRate         float64       `json:"error_rate"`
	ResponseSizes     []int         `json:"response_sizes,omitempty"`
	LatencyHistogram  []time.Duration `json:"latency_histogram,omitempty"`
}

// LSPBenchmarkResults aggregates all LSP method benchmark results
type LSPBenchmarkResults struct {
	OverallDuration    time.Duration                    `json:"overall_duration"`
	TotalRequests      int                              `json:"total_requests"`
	OverallThroughput  float64                          `json:"overall_throughput"`
	OverallErrorRate   float64                          `json:"overall_error_rate"`
	MethodBenchmarks   map[string]*LSPMethodBenchmark   `json:"method_benchmarks"`
	ConcurrentLoad     int                              `json:"concurrent_load"`
	WorkspaceSize      int                              `json:"workspace_size"`
	BenchmarkTimestamp time.Time                        `json:"benchmark_timestamp"`
}

// LSPPerformanceBenchmarker provides comprehensive LSP performance benchmarking
type LSPPerformanceBenchmarker struct {
	httpClient   *testutils.HttpClient
	repoManager  *testutils.PythonRepoManager
	workspaceDir string
	config       LSPBenchmarkConfig
	mu           sync.RWMutex
}

// NewLSPPerformanceBenchmarker creates a new LSP performance benchmarker
func NewLSPPerformanceBenchmarker(httpClient *testutils.HttpClient, repoManager *testutils.PythonRepoManager, config LSPBenchmarkConfig) *LSPPerformanceBenchmarker {
	return &LSPPerformanceBenchmarker{
		httpClient:   httpClient,
		repoManager:  repoManager,
		workspaceDir: repoManager.WorkspaceDir,
		config:       config,
	}
}

// RunComprehensiveBenchmark runs benchmarks for all LSP methods
func (lpb *LSPPerformanceBenchmarker) RunComprehensiveBenchmark(b *testing.B) *LSPBenchmarkResults {
	startTime := time.Now()
	
	results := &LSPBenchmarkResults{
		MethodBenchmarks:   make(map[string]*LSPMethodBenchmark),
		ConcurrentLoad:     lpb.config.ConcurrentRequests,
		BenchmarkTimestamp: startTime,
	}

	// Get test scenarios
	scenarios := fixtures.GetAllPythonPatternScenarios()
	
	// Run warmup requests
	lpb.runWarmupRequests(scenarios)

	// Benchmark each LSP method
	lspMethods := []string{
		"textDocument/definition",
		"textDocument/references",
		"textDocument/hover",
		"textDocument/documentSymbol",
		"workspace/symbol",
		"textDocument/completion",
	}

	totalRequests := 0
	totalSuccessful := 0
	
	for _, method := range lspMethods {
		b.Run(fmt.Sprintf("LSP_%s", method), func(b *testing.B) {
			benchmark := lpb.benchmarkLSPMethod(b, method, scenarios)
			results.MethodBenchmarks[method] = benchmark
			totalRequests += benchmark.TotalRequests
			totalSuccessful += benchmark.SuccessfulReqs
		})
	}

	// Calculate overall metrics
	results.OverallDuration = time.Since(startTime)
	results.TotalRequests = totalRequests
	results.OverallThroughput = float64(totalRequests) / results.OverallDuration.Seconds()
	if totalRequests > 0 {
		results.OverallErrorRate = float64(totalRequests-totalSuccessful) / float64(totalRequests)
	}

	return results
}

// benchmarkLSPMethod benchmarks a specific LSP method
func (lpb *LSPPerformanceBenchmarker) benchmarkLSPMethod(b *testing.B, method string, scenarios []fixtures.PythonPatternScenario) *LSPMethodBenchmark {
	benchmark := &LSPMethodBenchmark{
		Method:        method,
		MinLatency:    time.Hour, // Initialize to high value
		MaxLatency:    0,
		ResponseSizes: []int{},
		LatencyHistogram: []time.Duration{},
	}

	// Filter scenarios for this method
	relevantScenarios := make([]fixtures.PythonPatternScenario, 0)
	for _, scenario := range scenarios {
		for _, lspMethod := range scenario.LSPMethods {
			if lspMethod == method {
				relevantScenarios = append(relevantScenarios, scenario)
				break
			}
		}
	}

	if len(relevantScenarios) == 0 {
		return benchmark
	}

	// Run benchmark
	b.ResetTimer()
	
	var wg sync.WaitGroup
	results := make(chan benchmarkResult, lpb.config.RequestsPerMethod)
	
	// Launch concurrent workers
	for i := 0; i < lpb.config.ConcurrentRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lpb.benchmarkWorker(method, relevantScenarios, results)
		}()
	}

	// Collect results
	go func() {
		wg.Wait()
		close(results)
	}()

	totalLatency := time.Duration(0)
	latencies := make([]time.Duration, 0)

	for result := range results {
		benchmark.TotalRequests++
		if result.Success {
			benchmark.SuccessfulReqs++
			totalLatency += result.Latency
			latencies = append(latencies, result.Latency)
			
			if result.Latency < benchmark.MinLatency {
				benchmark.MinLatency = result.Latency
			}
			if result.Latency > benchmark.MaxLatency {
				benchmark.MaxLatency = result.Latency
			}
			
			if lpb.config.EnableDetailedMetrics {
				benchmark.ResponseSizes = append(benchmark.ResponseSizes, result.ResponseSize)
				benchmark.LatencyHistogram = append(benchmark.LatencyHistogram, result.Latency)
			}
		} else {
			benchmark.FailedRequests++
		}
	}

	// Calculate statistics
	if benchmark.SuccessfulReqs > 0 {
		benchmark.AverageLatency = totalLatency / time.Duration(benchmark.SuccessfulReqs)
		benchmark.P95Latency = calculatePercentile(latencies, 95)
		benchmark.P99Latency = calculatePercentile(latencies, 99)
	}

	if benchmark.TotalRequests > 0 {
		benchmark.ErrorRate = float64(benchmark.FailedRequests) / float64(benchmark.TotalRequests)
		benchmark.Throughput = float64(benchmark.SuccessfulReqs) / b.Elapsed().Seconds()
	}

	return benchmark
}

// benchmarkWorker performs benchmark requests for a specific method
func (lpb *LSPPerformanceBenchmarker) benchmarkWorker(method string, scenarios []fixtures.PythonPatternScenario, results chan<- benchmarkResult) {
	requestsPerWorker := lpb.config.RequestsPerMethod / lpb.config.ConcurrentRequests
	
	for i := 0; i < requestsPerWorker; i++ {
		scenario := scenarios[i%len(scenarios)]
		result := lpb.executeLSPRequest(method, scenario)
		results <- result
	}
}

// executeLSPRequest executes a single LSP request and measures performance
func (lpb *LSPPerformanceBenchmarker) executeLSPRequest(method string, scenario fixtures.PythonPatternScenario) benchmarkResult {
	ctx, cancel := context.WithTimeout(context.Background(), lpb.config.TimeoutPerRequest)
	defer cancel()

	filePath := filepath.Join(lpb.workspaceDir, scenario.FilePath)
	fileURI := "file://" + filePath

	startTime := time.Now()
	result := benchmarkResult{
		Method: method,
		Success: false,
		Latency: 0,
		ResponseSize: 0,
	}

	switch method {
	case "textDocument/definition":
		if len(scenario.TestPositions) > 0 {
			pos := scenario.TestPositions[0]
			position := testutils.Position{Line: pos.Line, Character: pos.Character}
			locations, err := lpb.httpClient.Definition(ctx, fileURI, position)
			result.Latency = time.Since(startTime)
			result.Success = err == nil
			if locations != nil {
				result.ResponseSize = len(locations) * 100 // Approximate size
			}
		}

	case "textDocument/references":
		if len(scenario.TestPositions) > 0 {
			pos := scenario.TestPositions[0]
			position := testutils.Position{Line: pos.Line, Character: pos.Character}
			references, err := lpb.httpClient.References(ctx, fileURI, position, true)
			result.Latency = time.Since(startTime)
			result.Success = err == nil
			if references != nil {
				result.ResponseSize = len(references) * 100 // Approximate size
			}
		}

	case "textDocument/hover":
		if len(scenario.TestPositions) > 0 {
			pos := scenario.TestPositions[0]
			position := testutils.Position{Line: pos.Line, Character: pos.Character}
			hoverResult, err := lpb.httpClient.Hover(ctx, fileURI, position)
			result.Latency = time.Since(startTime)
			result.Success = err == nil
			if hoverResult != nil && hoverResult.Contents != nil {
				result.ResponseSize = len(fmt.Sprintf("%v", hoverResult.Contents))
			}
		}

	case "textDocument/documentSymbol":
		symbols, err := lpb.httpClient.DocumentSymbol(ctx, fileURI)
		result.Latency = time.Since(startTime)
		result.Success = err == nil
		if symbols != nil {
			result.ResponseSize = len(symbols) * 50 // Approximate size per symbol
		}

	case "workspace/symbol":
		query := "Pattern"
		if len(scenario.ExpectedSymbols) > 0 {
			query = scenario.ExpectedSymbols[0].Name
		}
		symbols, err := lpb.httpClient.WorkspaceSymbol(ctx, query)
		result.Latency = time.Since(startTime)
		result.Success = err == nil
		if symbols != nil {
			result.ResponseSize = len(symbols) * 80 // Approximate size per symbol
		}

	case "textDocument/completion":
		if len(scenario.TestPositions) > 0 {
			pos := scenario.TestPositions[0]
			position := testutils.Position{Line: pos.Line, Character: pos.Character}
			completionList, err := lpb.httpClient.Completion(ctx, fileURI, position)
			result.Latency = time.Since(startTime)
			result.Success = err == nil
			if completionList != nil {
				result.ResponseSize = len(completionList.Items) * 60 // Approximate size per item
			}
		}
	}

	return result
}

// runWarmupRequests performs warmup requests to stabilize performance
func (lpb *LSPPerformanceBenchmarker) runWarmupRequests(scenarios []fixtures.PythonPatternScenario) {
	if lpb.config.WarmupRequests <= 0 || len(scenarios) == 0 {
		return
	}

	for i := 0; i < lpb.config.WarmupRequests; i++ {
		scenario := scenarios[i%len(scenarios)]
		if len(scenario.TestPositions) > 0 {
			filePath := filepath.Join(lpb.workspaceDir, scenario.FilePath)
			fileURI := "file://" + filePath
			pos := scenario.TestPositions[0]
			position := testutils.Position{Line: pos.Line, Character: pos.Character}
			
			ctx, cancel := context.WithTimeout(context.Background(), lpb.config.TimeoutPerRequest)
			// Execute a simple request for warmup
			_, _ = lpb.httpClient.Definition(ctx, fileURI, position)
			cancel()
		}
	}
}

// benchmarkResult represents the result of a single benchmark request
type benchmarkResult struct {
	Method       string
	Success      bool
	Latency      time.Duration
	ResponseSize int
	Error        error
}

// calculatePercentile calculates the nth percentile from a sorted slice of durations
func calculatePercentile(latencies []time.Duration, percentile int) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	// Sort latencies
	for i := 0; i < len(latencies)-1; i++ {
		for j := 0; j < len(latencies)-i-1; j++ {
			if latencies[j] > latencies[j+1] {
				latencies[j], latencies[j+1] = latencies[j+1], latencies[j]
			}
		}
	}

	index := int(float64(len(latencies)) * float64(percentile) / 100.0)
	if index >= len(latencies) {
		index = len(latencies) - 1
	}

	return latencies[index]
}

// GenerateBenchmarkReport generates a human-readable benchmark report
func (results *LSPBenchmarkResults) GenerateBenchmarkReport() string {
	report := fmt.Sprintf(`LSP Performance Benchmark Report
=====================================
Benchmark Duration: %v
Total Requests: %d
Overall Throughput: %.2f req/sec
Overall Error Rate: %.2f%%
Concurrent Load: %d workers
Timestamp: %s

Method-Specific Results:
`, 
		results.OverallDuration,
		results.TotalRequests,
		results.OverallThroughput,
		results.OverallErrorRate*100,
		results.ConcurrentLoad,
		results.BenchmarkTimestamp.Format(time.RFC3339),
	)

	for method, benchmark := range results.MethodBenchmarks {
		report += fmt.Sprintf(`
%s:
  Requests: %d (%d successful, %d failed)
  Average Latency: %v
  Min/Max Latency: %v / %v
  P95/P99 Latency: %v / %v
  Throughput: %.2f req/sec
  Error Rate: %.2f%%
`,
			method,
			benchmark.TotalRequests, benchmark.SuccessfulReqs, benchmark.FailedRequests,
			benchmark.AverageLatency,
			benchmark.MinLatency, benchmark.MaxLatency,
			benchmark.P95Latency, benchmark.P99Latency,
			benchmark.Throughput,
			benchmark.ErrorRate*100,
		)
	}

	return report
}

// ValidatePerformanceThresholds validates benchmark results against performance thresholds
func (results *LSPBenchmarkResults) ValidatePerformanceThresholds() []string {
	var violations []string

	// Define performance thresholds
	thresholds := map[string]struct {
		maxAvgLatency time.Duration
		minThroughput float64
		maxErrorRate  float64
	}{
		"textDocument/definition":    {2*time.Second, 10.0, 0.05},
		"textDocument/references":    {3*time.Second, 8.0, 0.05},
		"textDocument/hover":         {1*time.Second, 15.0, 0.03},
		"textDocument/documentSymbol": {2*time.Second, 12.0, 0.05},
		"workspace/symbol":           {4*time.Second, 5.0, 0.08},
		"textDocument/completion":    {1500*time.Millisecond, 20.0, 0.05},
	}

	for method, benchmark := range results.MethodBenchmarks {
		if threshold, exists := thresholds[method]; exists {
			if benchmark.AverageLatency > threshold.maxAvgLatency {
				violations = append(violations, 
					fmt.Sprintf("%s: Average latency %v exceeds threshold %v",
						method, benchmark.AverageLatency, threshold.maxAvgLatency))
			}
			
			if benchmark.Throughput < threshold.minThroughput {
				violations = append(violations,
					fmt.Sprintf("%s: Throughput %.2f req/sec below threshold %.2f req/sec",
						method, benchmark.Throughput, threshold.minThroughput))
			}
			
			if benchmark.ErrorRate > threshold.maxErrorRate {
				violations = append(violations,
					fmt.Sprintf("%s: Error rate %.2f%% exceeds threshold %.2f%%",
						method, benchmark.ErrorRate*100, threshold.maxErrorRate*100))
			}
		}
	}

	return violations
}