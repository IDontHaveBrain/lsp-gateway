package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-gateway/internal/testing/lsp/cases"
	"lsp-gateway/internal/testing/lsp/types"
)

// BenchmarkLSPDefinition benchmarks textDocument/definition performance
func BenchmarkLSPDefinition(b *testing.B) {
	benchmarkLSPMethod(b, cases.LSPMethodDefinition)
}

// BenchmarkLSPReferences benchmarks textDocument/references performance
func BenchmarkLSPReferences(b *testing.B) {
	benchmarkLSPMethod(b, cases.LSPMethodReferences)
}

// BenchmarkLSPHover benchmarks textDocument/hover performance
func BenchmarkLSPHover(b *testing.B) {
	benchmarkLSPMethod(b, cases.LSPMethodHover)
}

// BenchmarkLSPDocumentSymbol benchmarks textDocument/documentSymbol performance
func BenchmarkLSPDocumentSymbol(b *testing.B) {
	benchmarkLSPMethod(b, cases.LSPMethodDocumentSymbol)
}

// BenchmarkLSPWorkspaceSymbol benchmarks workspace/symbol performance
func BenchmarkLSPWorkspaceSymbol(b *testing.B) {
	benchmarkLSPMethod(b, cases.LSPMethodWorkspaceSymbol)
}

// BenchmarkLSPAllMethods benchmarks all LSP methods in sequence
func BenchmarkLSPAllMethods(b *testing.B) {
	methods := []string{
		cases.LSPMethodDefinition,
		cases.LSPMethodReferences,
		cases.LSPMethodHover,
		cases.LSPMethodDocumentSymbol,
		cases.LSPMethodWorkspaceSymbol,
	}

	for _, method := range methods {
		b.Run(method, func(b *testing.B) {
			benchmarkLSPMethod(b, method)
		})
	}
}

// BenchmarkLSPConcurrency benchmarks LSP methods under different concurrency levels
func BenchmarkLSPConcurrency(b *testing.B) {
	concurrencyLevels := []int{1, 5, 10, 20, 50}
	methods := []string{
		cases.LSPMethodDefinition,
		cases.LSPMethodReferences,
		cases.LSPMethodHover,
	}

	for _, method := range methods {
		for _, concurrency := range concurrencyLevels {
			name := fmt.Sprintf("%s_Concurrent_%d", method, concurrency)
			b.Run(name, func(b *testing.B) {
				benchmarkLSPMethodConcurrency(b, method, concurrency)
			})
		}
	}
}

// BenchmarkLSPMemoryProfile benchmarks LSP methods with detailed memory profiling
func BenchmarkLSPMemoryProfile(b *testing.B) {
	methods := []string{
		cases.LSPMethodDefinition,
		cases.LSPMethodReferences,
		cases.LSPMethodHover,
		cases.LSPMethodDocumentSymbol,
		cases.LSPMethodWorkspaceSymbol,
	}

	for _, method := range methods {
		b.Run(fmt.Sprintf("%s_MemoryProfile", method), func(b *testing.B) {
			benchmarkLSPMethodMemory(b, method)
		})
	}
}

// BenchmarkLSPThroughput measures maximum throughput for each LSP method
func BenchmarkLSPThroughput(b *testing.B) {
	methods := []string{
		cases.LSPMethodDefinition,
		cases.LSPMethodReferences,
		cases.LSPMethodHover,
		cases.LSPMethodDocumentSymbol,
		cases.LSPMethodWorkspaceSymbol,
	}

	for _, method := range methods {
		b.Run(fmt.Sprintf("%s_Throughput", method), func(b *testing.B) {
			benchmarkLSPMethodThroughput(b, method)
		})
	}
}

// BenchmarkLSPLatencyProfile measures detailed latency characteristics
func BenchmarkLSPLatencyProfile(b *testing.B) {
	methods := []string{
		cases.LSPMethodDefinition,
		cases.LSPMethodReferences,
		cases.LSPMethodHover,
		cases.LSPMethodDocumentSymbol,
		cases.LSPMethodWorkspaceSymbol,
	}

	for _, method := range methods {
		b.Run(fmt.Sprintf("%s_LatencyProfile", method), func(b *testing.B) {
			benchmarkLSPMethodLatency(b, method)
		})
	}
}

// BenchmarkLSPLargeFiles tests performance with large files
func BenchmarkLSPLargeFiles(b *testing.B) {
	fileSizes := []string{"small", "medium", "large", "xlarge"}
	methods := []string{
		cases.LSPMethodDefinition,
		cases.LSPMethodDocumentSymbol,
	}

	for _, method := range methods {
		for _, size := range fileSizes {
			name := fmt.Sprintf("%s_%s_file", method, size)
			b.Run(name, func(b *testing.B) {
				benchmarkLSPMethodWithFileSize(b, method, size)
			})
		}
	}
}

// BenchmarkLSPRegressionDetection runs regression detection benchmarks
func BenchmarkLSPRegressionDetection(b *testing.B) {
	if !shouldRunRegressionTests() {
		b.Skip("Regression detection benchmarks skipped (set LSP_BENCHMARK_REGRESSION=1 to enable)")
		return
	}

	methods := []string{
		cases.LSPMethodDefinition,
		cases.LSPMethodReferences,
		cases.LSPMethodHover,
	}

	for _, method := range methods {
		b.Run(fmt.Sprintf("%s_Regression", method), func(b *testing.B) {
			benchmarkLSPMethodRegression(b, method)
		})
	}
}

// benchmarkLSPMethod is the core benchmark function for a single LSP method
func benchmarkLSPMethod(b *testing.B, method string) {
	framework := setupLSPBenchmarkFramework(b)

	result := framework.BenchmarkMethod(context.Background(), b, method)

	// Report results
	reportBenchmarkResult(b, result)

	// Validate performance targets
	validatePerformanceTargets(b, result)
}

// benchmarkLSPMethodConcurrency benchmarks with specified concurrency level
func benchmarkLSPMethodConcurrency(b *testing.B, method string, concurrency int) {
	framework := setupLSPBenchmarkFramework(b)

	// Create a runner with custom configuration for concurrency testing
	runner := &BenchmarkRunner{
		method:    method,
		framework: framework.framework,
		config:    framework.config,
		logger:    framework.logger,
	}

	// Run concurrency test
	result := runner.runConcurrencyTest(context.Background(), concurrency)

	// Report concurrency-specific metrics
	b.ReportMetric(result.ThroughputRPS, "req/sec")
	b.ReportMetric(float64(result.LatencyMetrics.P50.Nanoseconds()), "p50-latency-ns")
	b.ReportMetric(float64(result.LatencyMetrics.P95.Nanoseconds()), "p95-latency-ns")
	b.ReportMetric(result.ErrorRate*100, "error-rate-%")

	if result.ErrorRate > 0.05 {
		b.Errorf("High error rate in concurrency test: %.2f%%", result.ErrorRate*100)
	}
}

// benchmarkLSPMethodMemory performs detailed memory profiling
func benchmarkLSPMethodMemory(b *testing.B, method string) {
	framework := setupLSPBenchmarkFramework(b)

	// Force GC before starting
	setupMemoryProfiling(b)

	result := framework.BenchmarkMethod(context.Background(), b, method)

	// Report memory metrics
	b.ReportMetric(float64(result.MemoryMetrics.AllocPerRequest), "bytes/op")
	b.ReportMetric(float64(result.MemoryMetrics.PeakMemoryUsage)/1024/1024, "peak-memory-MB")
	b.ReportMetric(result.MemoryMetrics.MemoryGrowthPercent, "memory-growth-%")
	b.ReportMetric(float64(result.MemoryMetrics.GCCount), "gc-count")

	// Validate memory thresholds
	if result.MemoryMetrics.AllocPerRequest > framework.config.MemoryThresholds.MaxAllocPerRequest {
		b.Errorf("Memory allocation per request %d bytes exceeds threshold %d bytes",
			result.MemoryMetrics.AllocPerRequest, framework.config.MemoryThresholds.MaxAllocPerRequest)
	}

	if result.MemoryMetrics.MemoryGrowthPercent > framework.config.MemoryThresholds.MaxMemoryGrowth {
		b.Errorf("Memory growth %.1f%% exceeds threshold %.1f%%",
			result.MemoryMetrics.MemoryGrowthPercent, framework.config.MemoryThresholds.MaxMemoryGrowth)
	}
}

// benchmarkLSPMethodThroughput measures maximum sustainable throughput
func benchmarkLSPMethodThroughput(b *testing.B, method string) {
	framework := setupLSPBenchmarkFramework(b)

	// Run for fixed duration to measure throughput
	duration := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var requestCount int64
	start := time.Now()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			select {
			case <-ctx.Done():
				return
			default:
				runner := &BenchmarkRunner{
					method:    method,
					framework: framework.framework,
					config:    framework.config,
					logger:    framework.logger,
				}

				_, err := runner.executeTestCase(context.Background())
				if err == nil {
					requestCount++
				}
			}
		}
	})

	actualDuration := time.Since(start)
	throughput := float64(requestCount) / actualDuration.Seconds()

	b.ReportMetric(throughput, "req/sec")

	if throughput < framework.config.ThroughputThresholds.MinRequestsPerSecond {
		b.Errorf("Throughput %.2f req/sec below threshold %.2f req/sec",
			throughput, framework.config.ThroughputThresholds.MinRequestsPerSecond)
	}

	b.Logf("Sustained throughput: %.2f req/sec over %v", throughput, actualDuration)
}

// benchmarkLSPMethodLatency measures detailed latency characteristics
func benchmarkLSPMethodLatency(b *testing.B, method string) {
	framework := setupLSPBenchmarkFramework(b)

	latencies := make([]time.Duration, 0, b.N)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		runner := &BenchmarkRunner{
			method:    method,
			framework: framework.framework,
			config:    framework.config,
			logger:    framework.logger,
		}

		latency, err := runner.executeTestCase(context.Background())
		if err == nil {
			latencies = append(latencies, latency)
		}
	}

	if len(latencies) > 0 {
		metrics := calculateLatencyMetrics(latencies)

		b.ReportMetric(float64(metrics.Average.Nanoseconds()), "avg-latency-ns")
		b.ReportMetric(float64(metrics.P50.Nanoseconds()), "p50-latency-ns")
		b.ReportMetric(float64(metrics.P95.Nanoseconds()), "p95-latency-ns")
		b.ReportMetric(float64(metrics.P99.Nanoseconds()), "p99-latency-ns")
		b.ReportMetric(float64(metrics.Max.Nanoseconds()), "max-latency-ns")

		// Log detailed latency information
		b.Logf("Latency Profile - Min: %v, Avg: %v, P50: %v, P95: %v, P99: %v, Max: %v",
			metrics.Min, metrics.Average, metrics.P50, metrics.P95, metrics.P99, metrics.Max)
	}
}

// benchmarkLSPMethodWithFileSize tests performance with different file sizes
func benchmarkLSPMethodWithFileSize(b *testing.B, method, fileSize string) {
	framework := setupLSPBenchmarkFramework(b)

	// Create test files of different sizes if they don't exist
	testFile := createTestFileOfSize(b, fileSize)
	defer cleanupTestFile(testFile)

	// Update framework configuration to use the specific test file
	// This is simplified - in real implementation, we'd modify the test case configuration

	result := framework.BenchmarkMethod(context.Background(), b, method)

	// Report file size impact on performance
	b.ReportMetric(result.ThroughputRPS, "req/sec")
	b.ReportMetric(float64(result.LatencyMetrics.P95.Nanoseconds()), "p95-latency-ns")

	b.Logf("File size %s: Throughput %.2f req/sec, P95 Latency %v",
		fileSize, result.ThroughputRPS, result.LatencyMetrics.P95)
}

// benchmarkLSPMethodRegression performs regression detection
func benchmarkLSPMethodRegression(b *testing.B, method string) {
	framework := setupLSPBenchmarkFramework(b)

	// Load historical performance data
	historical := loadHistoricalPerformanceData(method)

	// Run current benchmark
	result := framework.BenchmarkMethod(context.Background(), b, method)

	// Compare with historical data and detect regressions
	regressions := detectPerformanceRegressions(historical, result)

	if len(regressions) > 0 {
		for _, regression := range regressions {
			b.Errorf("Performance regression detected: %s", regression)
		}
	}

	// Save current results for future regression detection
	savePerformanceData(method, result)
}

// Helper functions

func setupLSPBenchmarkFramework(b *testing.B) *LSPBenchmarkFramework {
	options := &FrameworkOptions{
		ConfigPath:   "test-configs/performance-test-config.yaml",
		ReposPath:    "test-repositories.yaml",
		Verbose:      false,
		DryRun:       false,
		OutputDir:    "benchmark-results",
		ColorEnabled: false,
		LogTiming:    true,
	}

	benchmarkConfig := DefaultBenchmarkConfig()
	// Override for testing
	benchmarkConfig.WarmupIterations = 50
	benchmarkConfig.BenchmarkDuration = 10 * time.Second

	framework, err := NewLSPBenchmarkFramework(options, benchmarkConfig)
	if err != nil {
		b.Fatalf("Failed to create LSP benchmark framework: %v", err)
	}

	return framework
}

func setupMemoryProfiling(b *testing.B) {
	// Force garbage collection before memory profiling
	setupGCForProfiling()
	b.ReportAllocs()
}

func setupGCForProfiling() {
	// Implementation would set up GC for accurate memory profiling
	// This is a placeholder for the actual implementation
}

func reportBenchmarkResult(b *testing.B, result *types.BenchmarkResult) {
	// Report key metrics to Go's benchmark framework
	b.ReportMetric(result.ThroughputRPS, "req/sec")
	b.ReportMetric(float64(result.LatencyMetrics.P50.Nanoseconds()), "p50-latency-ns")
	b.ReportMetric(float64(result.LatencyMetrics.P95.Nanoseconds()), "p95-latency-ns")
	b.ReportMetric(float64(result.LatencyMetrics.P99.Nanoseconds()), "p99-latency-ns")
	b.ReportMetric(result.ErrorRate*100, "error-rate-%")
	b.ReportMetric(float64(result.MemoryMetrics.AllocPerRequest), "bytes/op")
}

func validatePerformanceTargets(b *testing.B, result *types.BenchmarkResult) {
	if !result.ThresholdResults.OverallPassed {
		for _, reason := range result.ThresholdResults.FailureReasons {
			b.Error(reason)
		}
	}
}

func shouldRunRegressionTests() bool {
	return os.Getenv("LSP_BENCHMARK_REGRESSION") == "1"
}

func createTestFileOfSize(b *testing.B, fileSize string) string {
	// This would create test files of different sizes for benchmarking
	// Implementation depends on the specific file size requirements
	testFile := filepath.Join(os.TempDir(), fmt.Sprintf("lsp_test_%s.go", fileSize))

	var content string
	switch fileSize {
	case "small":
		content = generateGoCode(100) // 100 lines
	case "medium":
		content = generateGoCode(1000) // 1000 lines
	case "large":
		content = generateGoCode(5000) // 5000 lines
	case "xlarge":
		content = generateGoCode(10000) // 10000 lines
	default:
		content = generateGoCode(100)
	}

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		b.Fatalf("Failed to create test file: %v", err)
	}

	return testFile
}

func generateGoCode(lines int) string {
	// Generate Go code with specified number of lines
	content := "package main\n\nimport \"fmt\"\n\n"

	for i := 0; i < lines; i++ {
		content += fmt.Sprintf("func function%d() {\n", i)
		content += fmt.Sprintf("    fmt.Printf(\"Function %d called\\n\")\n", i)
		content += "}\n\n"
	}

	content += "func main() {\n"
	for i := 0; i < min(lines, 10); i++ {
		content += fmt.Sprintf("    function%d()\n", i)
	}
	content += "}\n"

	return content
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func cleanupTestFile(testFile string) {
	os.Remove(testFile)
}

func loadHistoricalPerformanceData(method string) *types.BenchmarkResult {
	// Load historical performance data from previous benchmark runs
	// This would typically read from a JSON file or database
	historyFile := filepath.Join("benchmark-results", fmt.Sprintf("history_%s.json", method))

	data, err := os.ReadFile(historyFile)
	if err != nil {
		return nil // No historical data available
	}

	var result types.BenchmarkResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}

	return &result
}

func detectPerformanceRegressions(historical, current *types.BenchmarkResult) []string {
	var regressions []string

	if historical == nil {
		return regressions // No historical data to compare against
	}

	// Check for throughput regression (>20% decrease)
	if current.ThroughputRPS < historical.ThroughputRPS*0.8 {
		regressions = append(regressions,
			fmt.Sprintf("Throughput regression: %.2f -> %.2f req/sec (%.1f%% decrease)",
				historical.ThroughputRPS, current.ThroughputRPS,
				(historical.ThroughputRPS-current.ThroughputRPS)/historical.ThroughputRPS*100))
	}

	// Check for latency regression (>50% increase)
	if current.LatencyMetrics.P95 > historical.LatencyMetrics.P95*3/2 {
		regressions = append(regressions,
			fmt.Sprintf("P95 latency regression: %v -> %v (%.1f%% increase)",
				historical.LatencyMetrics.P95, current.LatencyMetrics.P95,
				float64(current.LatencyMetrics.P95-historical.LatencyMetrics.P95)/float64(historical.LatencyMetrics.P95)*100))
	}

	// Check for memory regression (>30% increase)
	if current.MemoryMetrics.AllocPerRequest > historical.MemoryMetrics.AllocPerRequest*13/10 {
		regressions = append(regressions,
			fmt.Sprintf("Memory allocation regression: %d -> %d bytes/op (%.1f%% increase)",
				historical.MemoryMetrics.AllocPerRequest, current.MemoryMetrics.AllocPerRequest,
				float64(current.MemoryMetrics.AllocPerRequest-historical.MemoryMetrics.AllocPerRequest)/float64(historical.MemoryMetrics.AllocPerRequest)*100))
	}

	return regressions
}

func savePerformanceData(method string, result *types.BenchmarkResult) {
	// Save current performance data for future regression detection
	historyFile := filepath.Join("benchmark-results", fmt.Sprintf("history_%s.json", method))

	// Create directory if it doesn't exist
	os.MkdirAll(filepath.Dir(historyFile), 0755)

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return // Silently fail
	}

	os.WriteFile(historyFile, data, 0644)
}
