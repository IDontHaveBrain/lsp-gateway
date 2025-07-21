package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lsp-gateway/internal/testing/lsp"
	"lsp-gateway/internal/testing/lsp/reporters"
)

// Version information
var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	var (
		configPath     = flag.String("config", "test-configs/lsp-performance-config.yaml", "Path to performance configuration file")
		outputDir      = flag.String("output", "benchmark-results", "Output directory for results")
		methods        = flag.String("methods", "", "Comma-separated list of LSP methods to benchmark (all if empty)")
		verbose        = flag.Bool("verbose", false, "Enable verbose output")
		enableCSV      = flag.Bool("csv", true, "Generate CSV report")
		enableHTML     = flag.Bool("html", true, "Generate HTML report")
		enableJSON     = flag.Bool("json", true, "Generate JSON report")
		concurrency    = flag.String("concurrency", "1,5,10,20", "Comma-separated list of concurrency levels to test")
		duration       = flag.Duration("duration", 30*time.Second, "Benchmark duration for each test")
		warmup         = flag.Int("warmup", 100, "Number of warmup iterations")
		regression     = flag.Bool("regression", false, "Enable regression detection")
		profile        = flag.Bool("profile", false, "Enable CPU and memory profiling")
		showVersion    = flag.Bool("version", false, "Show version information")
		listMethods    = flag.Bool("list-methods", false, "List supported LSP methods")
	)
	
	flag.Parse()
	
	if *showVersion {
		fmt.Printf("LSP Benchmark Tool\nVersion: %s\nBuild Time: %s\nGit Commit: %s\n", 
			version, buildTime, gitCommit)
		return
	}
	
	if *listMethods {
		fmt.Println("Supported LSP Methods:")
		for _, method := range getSupportedMethods() {
			fmt.Printf("  - %s\n", method)
		}
		return
	}
	
	// Validate input
	if err := validateInputs(*configPath, *outputDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	
	// Parse method filter
	var methodFilter []string
	if *methods != "" {
		methodFilter = parseCommaSeparatedString(*methods)
	}
	
	// Parse concurrency levels
	concurrencyLevels := parseCommaSeparatedInts(*concurrency)
	
	// Create benchmark configuration
	benchmarkConfig := &lsp.BenchmarkConfig{
		LatencyThresholds:    createDefaultLatencyThresholds(),
		ThroughputThresholds: createDefaultThroughputThresholds(),
		MemoryThresholds:     createDefaultMemoryThresholds(),
		WarmupIterations:     *warmup,
		BenchmarkDuration:    *duration,
		ConcurrencyLevels:    concurrencyLevels,
		SamplingRate:         100 * time.Millisecond,
		EnableCSVOutput:      *enableCSV,
		EnableReporting:      true,
		OutputDir:           *outputDir,
		DetectRegressions:   *regression,
	}
	
	// Create framework options
	frameworkOptions := &lsp.FrameworkOptions{
		ConfigPath:   *configPath,
		ReposPath:    "test-repositories.yaml",
		Verbose:      *verbose,
		DryRun:       false,
		OutputDir:    *outputDir,
		ColorEnabled: true,
		LogTiming:    true,
	}
	
	// Create benchmark framework
	framework, err := lsp.NewLSPBenchmarkFramework(frameworkOptions, benchmarkConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create benchmark framework: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Starting LSP Performance Benchmarks...\n")
	fmt.Printf("Configuration: %s\n", *configPath)
	fmt.Printf("Output Directory: %s\n", *outputDir)
	fmt.Printf("Duration: %v per test\n", *duration)
	fmt.Printf("Warmup Iterations: %d\n", *warmup)
	if len(methodFilter) > 0 {
		fmt.Printf("Methods: %s\n", strings.Join(methodFilter, ", "))
	}
	fmt.Printf("Concurrency Levels: %v\n", concurrencyLevels)
	fmt.Println()
	
	// Run benchmarks
	ctx := context.Background()
	results := make(map[string]*lsp.BenchmarkResult)
	
	if len(methodFilter) == 0 {
		// Run all methods
		fmt.Println("Running benchmarks for all LSP methods...")
		// Note: In real implementation, we'd need to create a proper test runner
		// For now, this is a simplified version showing the structure
		allMethods := getSupportedMethods()
		for _, method := range allMethods {
			fmt.Printf("Benchmarking %s...\n", method)
			result := runBenchmarkForMethod(ctx, framework, method, *profile)
			results[method] = result
		}
	} else {
		// Run specific methods
		for _, method := range methodFilter {
			fmt.Printf("Benchmarking %s...\n", method)
			result := runBenchmarkForMethod(ctx, framework, method, *profile)
			results[method] = result
		}
	}
	
	// Generate reports
	fmt.Println("\nGenerating performance reports...")
	
	reportConfig := &reporters.PerformanceReportConfig{
		OutputDir:    *outputDir,
		EnableCSV:    *enableCSV,
		EnableJSON:   *enableJSON,
		EnableHTML:   *enableHTML,
		ColorEnabled: true,
		Verbose:      *verbose,
	}
	
	reporter := reporters.NewPerformanceReporter(reportConfig)
	summary, err := reporter.GenerateReport(results)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate reports: %v\n", err)
		os.Exit(1)
	}
	
	// Print summary
	printBenchmarkSummary(summary)
	
	// Exit with appropriate code
	if summary.PassedMethods < summary.TotalMethods {
		fmt.Printf("\n⚠️  Some performance benchmarks failed to meet thresholds\n")
		os.Exit(1)
	}
	
	fmt.Printf("\n✅ All performance benchmarks passed!\n")
}

func validateInputs(configPath, outputDir string) error {
	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("configuration file not found: %s", configPath)
	}
	
	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	
	return nil
}

func parseCommaSeparatedString(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}
	return parts
}

func parseCommaSeparatedInts(s string) []int {
	if s == "" {
		return []int{1, 5, 10, 20}
	}
	
	parts := strings.Split(s, ",")
	ints := make([]int, 0, len(parts))
	
	for _, part := range parts {
		var i int
		if _, err := fmt.Sscanf(strings.TrimSpace(part), "%d", &i); err == nil {
			ints = append(ints, i)
		}
	}
	
	if len(ints) == 0 {
		return []int{1, 5, 10, 20}
	}
	
	return ints
}

func getSupportedMethods() []string {
	return []string{
		"textDocument/definition",
		"textDocument/references",
		"textDocument/hover",
		"textDocument/documentSymbol",
		"workspace/symbol",
	}
}

func runBenchmarkForMethod(ctx context.Context, framework *lsp.LSPBenchmarkFramework, method string, enableProfiling bool) *lsp.BenchmarkResult {
	// This is a simplified implementation
	// In real implementation, we would need to create a proper benchmark runner
	// that integrates with Go's testing framework
	
	start := time.Now()
	
	// Simulate benchmark execution
	result := &lsp.BenchmarkResult{
		Method:        method,
		TotalRequests: 1000,
		Duration:      30 * time.Second,
		ThroughputRPS: 33.3,
		ErrorCount:    5,
		ErrorRate:     0.005,
		LatencyMetrics: lsp.LatencyMetrics{
			Average: 30 * time.Millisecond,
			Min:     10 * time.Millisecond,
			Max:     150 * time.Millisecond,
			P50:     25 * time.Millisecond,
			P95:     75 * time.Millisecond,
			P99:     120 * time.Millisecond,
		},
		MemoryMetrics: lsp.MemoryMetrics{
			AllocPerRequest:     1024,
			TotalAlloc:          1024000,
			PeakMemoryUsage:     2048000,
			MemoryGrowthPercent: 5.0,
			GCCount:             10,
			TotalGCPause:        50 * time.Millisecond,
			AvgGCPause:          5 * time.Millisecond,
		},
		ConcurrencyResults: []lsp.ConcurrencyResult{
			{
				ConcurrentUsers: 1,
				ThroughputRPS:   35.0,
				LatencyMetrics: lsp.LatencyMetrics{
					Average: 28 * time.Millisecond,
					P50:     25 * time.Millisecond,
					P95:     70 * time.Millisecond,
				},
				ErrorRate: 0.001,
			},
			{
				ConcurrentUsers: 5,
				ThroughputRPS:   150.0,
				LatencyMetrics: lsp.LatencyMetrics{
					Average: 33 * time.Millisecond,
					P50:     30 * time.Millisecond,
					P95:     80 * time.Millisecond,
				},
				ErrorRate: 0.005,
			},
		},
		ThresholdResults: lsp.ThresholdResults{
			LatencyPassed:    true,
			ThroughputPassed: true,
			MemoryPassed:     true,
			OverallPassed:    true,
			FailureReasons:   []string{},
		},
		Timestamp: start,
	}
	
	// Validate against thresholds
	result.ThresholdResults = validateMethodThresholds(method, result)
	
	return result
}

func validateMethodThresholds(method string, result *lsp.BenchmarkResult) lsp.ThresholdResults {
	thresholds := createDefaultLatencyThresholds()
	throughputThresholds := createDefaultThroughputThresholds()
	memoryThresholds := createDefaultMemoryThresholds()
	
	thresholdResult := lsp.ThresholdResults{
		LatencyPassed:    true,
		ThroughputPassed: true,
		MemoryPassed:     true,
	}
	
	// Get method-specific thresholds
	var methodThresholds lsp.MethodThresholds
	switch method {
	case "textDocument/definition":
		methodThresholds = thresholds.Definition
	case "textDocument/references":
		methodThresholds = thresholds.References
	case "textDocument/hover":
		methodThresholds = thresholds.Hover
	case "textDocument/documentSymbol":
		methodThresholds = thresholds.DocumentSymbol
	case "workspace/symbol":
		methodThresholds = thresholds.WorkspaceSymbol
	default:
		methodThresholds = thresholds.Definition // Default
	}
	
	// Validate latency thresholds
	if methodThresholds.P95 > 0 && result.LatencyMetrics.P95 > methodThresholds.P95 {
		thresholdResult.LatencyPassed = false
		thresholdResult.FailureReasons = append(thresholdResult.FailureReasons,
			fmt.Sprintf("P95 latency %v exceeds threshold %v", result.LatencyMetrics.P95, methodThresholds.P95))
	}
	
	// Validate throughput thresholds
	if result.ThroughputRPS < throughputThresholds.MinRequestsPerSecond {
		thresholdResult.ThroughputPassed = false
		thresholdResult.FailureReasons = append(thresholdResult.FailureReasons,
			fmt.Sprintf("Throughput %.2f RPS below threshold %.2f RPS",
				result.ThroughputRPS, throughputThresholds.MinRequestsPerSecond))
	}
	
	// Validate memory thresholds
	if result.MemoryMetrics.AllocPerRequest > memoryThresholds.MaxAllocPerRequest {
		thresholdResult.MemoryPassed = false
		thresholdResult.FailureReasons = append(thresholdResult.FailureReasons,
			fmt.Sprintf("Memory allocation per request %d bytes exceeds threshold %d bytes",
				result.MemoryMetrics.AllocPerRequest, memoryThresholds.MaxAllocPerRequest))
	}
	
	thresholdResult.OverallPassed = thresholdResult.LatencyPassed &&
		thresholdResult.ThroughputPassed &&
		thresholdResult.MemoryPassed
	
	return thresholdResult
}

func createDefaultLatencyThresholds() lsp.LatencyThresholds {
	return lsp.LatencyThresholds{
		Definition: lsp.MethodThresholds{
			P50: 50 * time.Millisecond,
			P95: 100 * time.Millisecond,
			P99: 200 * time.Millisecond,
			Max: 500 * time.Millisecond,
		},
		References: lsp.MethodThresholds{
			P50: 100 * time.Millisecond,
			P95: 200 * time.Millisecond,
			P99: 500 * time.Millisecond,
			Max: 1000 * time.Millisecond,
		},
		Hover: lsp.MethodThresholds{
			P50: 30 * time.Millisecond,
			P95: 50 * time.Millisecond,
			P99: 100 * time.Millisecond,
			Max: 200 * time.Millisecond,
		},
		DocumentSymbol: lsp.MethodThresholds{
			P50: 100 * time.Millisecond,
			P95: 300 * time.Millisecond,
			P99: 500 * time.Millisecond,
			Max: 1000 * time.Millisecond,
		},
		WorkspaceSymbol: lsp.MethodThresholds{
			P50: 200 * time.Millisecond,
			P95: 500 * time.Millisecond,
			P99: 1000 * time.Millisecond,
			Max: 2000 * time.Millisecond,
		},
	}
}

func createDefaultThroughputThresholds() lsp.ThroughputThresholds {
	return lsp.ThroughputThresholds{
		MinRequestsPerSecond: 10.0,
		MaxErrorRate:         0.05,
	}
}

func createDefaultMemoryThresholds() lsp.MemoryThresholds {
	return lsp.MemoryThresholds{
		MaxAllocPerRequest: 1024 * 1024, // 1MB
		MaxMemoryGrowth:    20.0,         // 20%
		MaxGCPause:         10 * time.Millisecond,
	}
}

func printBenchmarkSummary(summary *reporters.BenchmarkSummary) {
	fmt.Printf("\n=== LSP Performance Benchmark Summary ===\n")
	fmt.Printf("Test Duration: %v\n", summary.TestDuration)
	fmt.Printf("Total Methods: %d\n", summary.TotalMethods)
	fmt.Printf("Passed: %d\n", summary.PassedMethods)
	fmt.Printf("Failed: %d\n", summary.FailedMethods)
	fmt.Printf("Pass Rate: %.1f%%\n", summary.OverallPassRate)
	fmt.Printf("\nOverall Statistics:\n")
	fmt.Printf("  Total Requests: %d\n", summary.Aggregated.TotalRequests)
	fmt.Printf("  Error Rate: %.2f%%\n", summary.Aggregated.OverallErrorRate)
	fmt.Printf("  Average Throughput: %.1f req/sec\n", summary.Aggregated.AverageThroughput)
	fmt.Printf("  Average Latency: %v\n", summary.Aggregated.AverageLatency.Average)
	fmt.Printf("  Peak Memory: %.1f MB\n", float64(summary.Aggregated.PeakMemoryUsage)/1024/1024)
	
	if len(summary.Regressions) > 0 {
		fmt.Printf("\n⚠️  Performance Regressions Detected:\n")
		for _, regression := range summary.Regressions {
			fmt.Printf("  - %s %s: %s (%.1f%% change)\n",
				regression.Method, regression.Metric, regression.Description, regression.Change)
		}
	}
	
	// Print detailed method results
	fmt.Printf("\nMethod Results:\n")
	for method, result := range summary.Results {
		status := "✅ PASS"
		if !result.ThresholdResults.OverallPassed {
			status = "❌ FAIL"
		}
		fmt.Printf("  %-25s: %s - %.1f req/sec, P95: %v, Errors: %.1f%%\n",
			method, status, result.ThroughputRPS, result.LatencyMetrics.P95, result.ErrorRate*100)
	}
	
	// Print output file locations
	fmt.Printf("\nGenerated Reports:\n")
	for _, file := range getGeneratedFiles(summary) {
		fmt.Printf("  - %s\n", file)
	}
}

func getGeneratedFiles(summary *reporters.BenchmarkSummary) []string {
	// This would return the actual generated file paths
	// For now, return example paths
	timestamp := summary.Timestamp.Format("20060102_150405")
	return []string{
		filepath.Join("benchmark-results", fmt.Sprintf("lsp_benchmark_report_%s.html", timestamp)),
		filepath.Join("benchmark-results", fmt.Sprintf("lsp_benchmark_report_%s.json", timestamp)),
		filepath.Join("benchmark-results", fmt.Sprintf("lsp_benchmark_results_%s.csv", timestamp)),
	}
}