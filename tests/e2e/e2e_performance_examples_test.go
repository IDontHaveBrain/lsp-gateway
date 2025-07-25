package e2e_test

import (
	"context"
	"testing"
	"time"

	"lsp-gateway/mcp"
	"lsp-gateway/tests/framework"
)

// TestPerformanceMethodsExamples demonstrates practical usage of all performance testing methods
func TestPerformanceMethodsExamples(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance method examples in short mode")
	}

	// Setup test environment
	config := &E2ETestConfig{
		TestTimeout:       10 * time.Minute,
		MaxConcurrency:    5,
		RetryAttempts:     2,
		CleanupOnFailure:  true,
		DetailedLogging:   true,
		MetricsCollection: true,
		Languages:         []string{"go", "python", "typescript"},
		Scenarios: []E2EScenario{
			E2EScenarioPerformanceValidation,
		},
	}

	runner, err := NewE2ETestRunner(config)
	if err != nil {
		t.Fatalf("Failed to create E2E test runner: %v", err)
	}
	defer runner.Cleanup()

	if err := runner.SetupTestEnvironment(context.Background()); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}

	// Create performance testing methods instance
	perfMethods := NewPerformanceTestMethods(
		runner.MockMcpClient,
		runner.TestFramework.PerformanceProfiler,
		runner.TestFramework,
		runner.ctx,
	)

	// Example 1: Load Testing Scenario
	t.Run("LoadTestingScenario", func(t *testing.T) {
		t.Log("=== Load Testing Scenario Example ===")

		result := perfMethods.executeLoadTestingScenario(
			runner.ctx,
			runner.MockMcpClient,
			50,             // 50 concurrent users
			60*time.Second, // 1 minute duration
		)

		// Validate results
		t.Logf("Load Test Results:")
		t.Logf("  Total Requests: %d", result.TotalRequests)
		t.Logf("  Successful: %d (%.2f%%)", result.SuccessfulRequests,
			float64(result.SuccessfulRequests)/float64(result.TotalRequests)*100)
		t.Logf("  Failed: %d (%.2f%%)", result.FailedRequests, result.ErrorRatePercent)
		t.Logf("  Average Throughput: %.2f req/sec", result.AverageThroughput)
		t.Logf("  Peak Throughput: %.2f req/sec", result.PeakThroughput)
		t.Logf("  Average Latency: %d ms", result.AverageLatencyMs)
		t.Logf("  P95 Latency: %d ms", result.P95LatencyMs)
		t.Logf("  P99 Latency: %d ms", result.P99LatencyMs)
		t.Logf("  Memory Growth: %d MB", result.MemoryUsageGrowthMB)
		t.Logf("  Success: %t", result.Success)

		if !result.Success {
			t.Logf("  Threshold Violations: %v", result.ThresholdViolations)
		}

		// Validate key metrics
		if result.TotalRequests < 100 {
			t.Errorf("Expected at least 100 requests, got %d", result.TotalRequests)
		}

		if result.ErrorRatePercent > 10.0 {
			t.Errorf("Error rate %.2f%% too high", result.ErrorRatePercent)
		}

		if result.AverageThroughput < 10.0 {
			t.Errorf("Throughput %.2f req/sec too low", result.AverageThroughput)
		}

		// Log method-specific performance
		t.Logf("Method Performance Breakdown:")
		for method, methodPerf := range result.MethodPerformance {
			t.Logf("  %s:", method)
			t.Logf("    Requests: %d", methodPerf.TotalRequests)
			t.Logf("    Throughput: %.2f req/sec", methodPerf.ThroughputReqPerSec)
			t.Logf("    Avg Latency: %d ms", methodPerf.AverageLatencyMs)
			t.Logf("    P95 Latency: %d ms", methodPerf.P95LatencyMs)
			t.Logf("    Error Rate: %.2f%%", methodPerf.ErrorRatePercent)
		}
	})

	// Example 2: LSP Method Benchmarking
	t.Run("LSPMethodBenchmarking", func(t *testing.T) {
		t.Log("=== LSP Method Benchmarking Example ===")

		methods := []string{
			mcp.LSP_METHOD_WORKSPACE_SYMBOL,
			mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION,
			mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES,
			mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER,
		}

		for _, method := range methods {
			t.Logf("Benchmarking method: %s", method)

			result := perfMethods.benchmarkLSPMethodPerformance(
				runner.ctx,
				runner.MockMcpClient,
				method,
				100, // 100 iterations
			)

			t.Logf("  Benchmark Results for %s:", method)
			t.Logf("    Total Requests: %d", result.TotalRequests)
			t.Logf("    Successful: %d", result.SuccessfulRequests)
			t.Logf("    Failed: %d", result.FailedRequests)
			t.Logf("    Throughput: %.2f req/sec", result.ThroughputReqPerSec)
			t.Logf("    Average Latency: %d ms", result.AverageLatencyMs)
			t.Logf("    Min Latency: %d ms", result.MinLatencyMs)
			t.Logf("    Max Latency: %d ms", result.MaxLatencyMs)
			t.Logf("    P95 Latency: %d ms", result.P95LatencyMs)
			t.Logf("    P99 Latency: %d ms", result.P99LatencyMs)
			t.Logf("    Error Rate: %.2f%%", result.ErrorRatePercent)
			t.Logf("    Memory Delta: %d MB", result.MemoryUsageDelta)
			t.Logf("    Duration: %v", result.BenchmarkDuration)

			// Validate benchmark results
			if result.TotalRequests != 100 {
				t.Errorf("Expected 100 requests for %s, got %d", method, result.TotalRequests)
			}

			if result.ErrorRatePercent > 5.0 {
				t.Errorf("Error rate for %s too high: %.2f%%", method, result.ErrorRatePercent)
			}

			if result.ThroughputReqPerSec < 5.0 {
				t.Errorf("Throughput for %s too low: %.2f req/sec", method, result.ThroughputReqPerSec)
			}

			// Log error details if any
			if len(result.Errors) > 0 {
				t.Logf("    Errors encountered:")
				for i, errorOcc := range result.Errors[:min(5, len(result.Errors))] {
					t.Logf("      %d. %s: %s", i+1, errorOcc.ErrorType, errorOcc.ErrorMessage)
				}
			}
		}
	})

	// Example 3: Memory Usage Pattern Testing
	t.Run("MemoryUsagePatterns", func(t *testing.T) {
		t.Log("=== Memory Usage Pattern Testing Example ===")

		// Create a test project for memory testing
		testProject, err := runner.TestFramework.CreateMultiLanguageProject(
			framework.ProjectTypeMultiLanguage,
			[]string{"go", "python", "typescript"},
		)
		if err != nil {
			t.Fatalf("Failed to create test project: %v", err)
		}

		result := perfMethods.testMemoryUsagePatterns(
			runner.ctx,
			runner.MockMcpClient,
			testProject,
		)

		t.Logf("Memory Usage Analysis Results:")
		t.Logf("  Test Duration: %v", result.EndTime.Sub(result.StartTime))
		t.Logf("  Initial Memory: %d MB", result.InitialMemoryMB)
		t.Logf("  Peak Memory: %d MB", result.PeakMemoryMB)
		t.Logf("  Final Memory: %d MB", result.FinalMemoryMB)
		t.Logf("  Memory Growth: %d MB", result.MemoryGrowthMB)
		t.Logf("  Net Memory Change: %d MB", result.NetMemoryChangeMB)
		t.Logf("  Memory Leak Detected: %t", result.MemoryLeakDetected)
		t.Logf("  GC Efficiency: %.2f", result.GCEfficiency)
		t.Logf("  Heap Allocations: %d", result.HeapAllocations)
		t.Logf("  Heap Releases: %d", result.HeapReleases)
		t.Logf("  GC Count: %d", result.GCCount)
		t.Logf("  GC Pause Total: %d ms", result.GCPauseTotalMs)
		t.Logf("  Memory Stability Score: %.2f", result.MemoryStabilityScore)

		// Log memory spikes
		if len(result.MemorySpikes) > 0 {
			t.Logf("  Memory Spikes Detected: %d", len(result.MemorySpikes))
			for i, spike := range result.MemorySpikes[:min(3, len(result.MemorySpikes))] {
				t.Logf("    %d. %s: %d MB spike magnitude",
					i+1, spike.Timestamp.Format("15:04:05"), spike.SpikeMagnitude)
			}
		}

		// Log large request handling analysis
		if result.LargeRequestHandling != nil {
			t.Logf("  Large Request Analysis:")
			t.Logf("    Large Requests: %d", result.LargeRequestHandling.LargeRequestCount)
			t.Logf("    Avg Memory Spike: %d MB", result.LargeRequestHandling.AverageMemorySpikeMB)
			t.Logf("    Max Memory Spike: %d MB", result.LargeRequestHandling.MaxMemorySpikeMB)
			t.Logf("    Recovery Time: %d ms", result.LargeRequestHandling.RecoveryTimeMs)
			t.Logf("    Timeout Occurrences: %d", result.LargeRequestHandling.TimeoutOccurrences)
		}

		// Validate memory usage
		if result.MemoryLeakDetected {
			t.Errorf("Memory leak detected: %d MB growth", result.MemoryGrowthMB)
		}

		if result.PeakMemoryMB > 2048 { // 2GB threshold
			t.Errorf("Peak memory usage too high: %d MB", result.PeakMemoryMB)
		}

		if result.MemoryStabilityScore < 0.7 {
			t.Errorf("Memory stability score too low: %.2f", result.MemoryStabilityScore)
		}

		// Log threshold violations
		if len(result.ThresholdViolations) > 0 {
			t.Logf("  Threshold Violations:")
			for i, violation := range result.ThresholdViolations {
				t.Logf("    %d. %s", i+1, violation)
			}
		}
	})

	// Example 4: Latency Threshold Validation
	t.Run("LatencyThresholdValidation", func(t *testing.T) {
		t.Log("=== Latency Threshold Validation Example ===")

		expectedLatency := 1000 * time.Millisecond // 1 second threshold

		result := perfMethods.validateLatencyThresholds(
			runner.ctx,
			runner.MockMcpClient,
			expectedLatency,
		)

		t.Logf("Latency Validation Results:")
		t.Logf("  Method: %s", result.Method)
		t.Logf("  Sample Count: %d", result.SampleCount)
		t.Logf("  Expected Latency: %d ms", result.ExpectedLatencyMs)
		t.Logf("  Actual Average Latency: %d ms", result.ActualAverageLatencyMs)
		t.Logf("  P95 Latency: %d ms", result.P95LatencyMs)
		t.Logf("  P99 Latency: %d ms", result.P99LatencyMs)
		t.Logf("  Max Latency: %d ms", result.MaxLatencyMs)
		t.Logf("  Latency Violations: %d", result.LatencyViolations)
		t.Logf("  Violation Percentage: %.2f%%", result.ViolationPercent)
		t.Logf("  Validation Success: %t", result.ValidationSuccess)

		// Log latency distribution
		t.Logf("  Latency Distribution:")
		for bucket, count := range result.LatencyDistribution {
			t.Logf("    %s: %d requests", bucket, count)
		}

		// Log threshold exceedances
		if len(result.ThresholdExceedances) > 0 {
			t.Logf("  Threshold Exceedances (first 3):")
			for i, exceedance := range result.ThresholdExceedances[:min(3, len(result.ThresholdExceedances))] {
				t.Logf("    %d. Request %s: %d ms (%.1f%% over threshold)",
					i+1, exceedance.RequestID, exceedance.ActualLatencyMs, exceedance.ExceedancePercent)
			}
		}

		// Validate latency results
		if !result.ValidationSuccess {
			t.Errorf("Latency validation failed: %.1f%% violations", result.ViolationPercent)
		}

		if result.P95LatencyMs > result.ExpectedLatencyMs {
			t.Errorf("P95 latency %d ms exceeds threshold %d ms",
				result.P95LatencyMs, result.ExpectedLatencyMs)
		}
	})

	// Example 5: Throughput Limits Testing
	t.Run("ThroughputLimitsTesting", func(t *testing.T) {
		t.Log("=== Throughput Limits Testing Example ===")

		targetTPS := 100.0 // Target 100 requests per second

		result := perfMethods.testThroughputLimits(
			runner.ctx,
			runner.MockMcpClient,
			targetTPS,
		)

		t.Logf("Throughput Testing Results:")
		t.Logf("  Target Throughput: %.2f req/sec", result.TargetThroughputReqPerSec)
		t.Logf("  Actual Throughput: %.2f req/sec", result.ActualThroughputReqPerSec)
		t.Logf("  Peak Throughput: %.2f req/sec", result.PeakThroughputReqPerSec)
		t.Logf("  Throughput Efficiency: %.2f", result.ThroughputEfficiency)
		t.Logf("  Sustained Throughput: %.2f req/sec", result.SustainedThroughput)
		t.Logf("  Throughput Variability: %.2f", result.ThroughputVariability)
		t.Logf("  Optimal Concurrency: %d", result.OptimalConcurrency)
		t.Logf("  Max Sustainable Throughput: %.2f req/sec", result.MaxSustainableThroughput)
		t.Logf("  Threshold Validation: %t", result.ThresholdValidation)

		// Log scaling behavior analysis
		if result.ScalingBehavior != nil {
			t.Logf("  Scaling Behavior:")
			t.Logf("    Linear Scaling Limit: %d", result.ScalingBehavior.LinearScalingLimit)
			t.Logf("    Degradation Point: %d", result.ScalingBehavior.DegradationPoint)
			t.Logf("    Optimal Concurrency: %d", result.ScalingBehavior.OptimalConcurrency)
			t.Logf("    Scaling Efficiency: %.2f", result.ScalingBehavior.ScalingEfficiency)
			t.Logf("    Throughput Ceiling: %.2f req/sec", result.ScalingBehavior.ThroughputCeiling)

			// Log concurrency points
			t.Logf("    Concurrency Test Points:")
			for i, point := range result.ScalingBehavior.ConcurrencyPoints[:min(3, len(result.ScalingBehavior.ConcurrencyPoints))] {
				t.Logf("      %d. Concurrency %d: %.2f req/sec, %d ms latency, %.2f%% errors",
					i+1, point.Concurrency, point.ThroughputReqPerSec,
					point.LatencyMs, point.ErrorRatePercent)
			}
		}

		// Log bottleneck points
		if len(result.BottleneckPoints) > 0 {
			t.Logf("  Bottleneck Points:")
			for i, bottleneck := range result.BottleneckPoints {
				t.Logf("    %d. Limiting Factor: %s at %.2f req/sec",
					i+1, bottleneck.LimitingFactor, bottleneck.ThroughputReqPerSec)
			}
		}

		// Log resource limitations
		if len(result.ResourceLimitations) > 0 {
			t.Logf("  Resource Limitations:")
			for i, limitation := range result.ResourceLimitations {
				t.Logf("    %d. %s: %.1f%% utilization (limit reached: %v)",
					i+1, limitation.ResourceType, limitation.UtilizationPercent, limitation.LimitReached)
				t.Logf("       Impact: %.1f%% throughput loss", limitation.ImpactOnThroughput)
				t.Logf("       Recommended Action: %s", limitation.RecommendedAction)
			}
		}

		// Log error impact analysis
		if result.ErrorImpact != nil {
			t.Logf("  Error Impact Analysis:")
			t.Logf("    Error Rate: %.2f%%", result.ErrorImpact.ErrorRatePercent)
			t.Logf("    Throughput Impact: %.2f%%", result.ErrorImpact.ThroughputImpactPercent)
			t.Logf("    Error-Throughput Correlation: %.2f", result.ErrorImpact.ErrorThroughputCorrelation)
			t.Logf("    Recovery Time: %d ms", result.ErrorImpact.RecoveryTimeMs)
		}

		// Validate throughput results
		if result.ThroughputEfficiency < 0.8 {
			t.Errorf("Throughput efficiency %.2f too low", result.ThroughputEfficiency)
		}

		if result.ActualThroughputReqPerSec < targetTPS*0.7 {
			t.Errorf("Actual throughput %.2f req/sec significantly below target %.2f req/sec",
				result.ActualThroughputReqPerSec, targetTPS)
		}
	})

	// Example 6: Performance Regression Testing
	t.Run("PerformanceRegressionTesting", func(t *testing.T) {
		t.Log("=== Performance Regression Testing Example ===")

		// Create a baseline for comparison
		baseline := &PerformanceRegressionBaseline{
			Version:   "baseline-v1.0",
			Timestamp: time.Now().Add(-24 * time.Hour),
			MethodLatencies: map[string]int64{
				mcp.LSP_METHOD_WORKSPACE_SYMBOL:         80,
				mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION: 40,
				mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES: 60,
				mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER:      25,
				mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS:    35,
			},
			ThroughputData: map[string]float64{
				"overall":                               120.0,
				mcp.LSP_METHOD_WORKSPACE_SYMBOL:         20.0,
				mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION: 35.0,
				mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES: 25.0,
				mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER:      30.0,
				mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS:    15.0,
			},
			MemoryUsageData: map[string]int64{
				"peak":   400,
				"growth": 30,
			},
			ErrorRates: map[string]float64{
				mcp.LSP_METHOD_WORKSPACE_SYMBOL:         1.5,
				mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION: 0.8,
				mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES: 1.2,
				mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER:      0.3,
				mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS:    0.7,
			},
			ValidationResults: map[string]bool{
				"memory-leak-detected": false,
			},
		}

		result := perfMethods.executePerformanceRegressionTest(
			runner.ctx,
			runner.MockMcpClient,
			baseline,
		)

		t.Logf("Performance Regression Analysis Results:")
		t.Logf("  Baseline Version: %s", result.BaselineVersion)
		t.Logf("  Current Version: %s", result.CurrentVersion)
		t.Logf("  Comparison Timestamp: %s", result.ComparisonTimestamp.Format("2006-01-02 15:04:05"))
		t.Logf("  Overall Regression Score: %.2f", result.OverallRegressionScore)
		t.Logf("  Regression Detected: %t", result.RegressionDetected)
		t.Logf("  Regression Severity: %s", result.RegressionSeverity)

		// Log method comparisons
		t.Logf("  Method Performance Comparisons:")
		for method, methodRegression := range result.MethodComparisons {
			t.Logf("    %s:", method)
			t.Logf("      Latency: %d ms -> %d ms (%.2f%% change)",
				methodRegression.BaselineLatencyMs, methodRegression.CurrentLatencyMs,
				methodRegression.LatencyChangePercent)
			t.Logf("      Throughput: %.2f -> %.2f req/sec (%.2f%% change)",
				methodRegression.BaselineThroughput, methodRegression.CurrentThroughput,
				methodRegression.ThroughputChangePercent)
			t.Logf("      Error Rate: %.2f%% -> %.2f%% (%.2f%% change)",
				methodRegression.BaselineErrorRate, methodRegression.CurrentErrorRate,
				methodRegression.ErrorRateChangePercent)
			t.Logf("      Regression Detected: %t (Severity: %s)",
				methodRegression.RegressionDetected, methodRegression.Severity)
		}

		// Log regression analysis results
		if result.ThroughputRegression != nil {
			t.Logf("  Throughput Regression Analysis:")
			t.Logf("    Baseline: %.2f req/sec", result.ThroughputRegression.BaselineThroughput)
			t.Logf("    Current: %.2f req/sec", result.ThroughputRegression.CurrentThroughput)
			t.Logf("    Change: %.2f%%", result.ThroughputRegression.ThroughputChangePercent)
			t.Logf("    Regression Detected: %t", result.ThroughputRegression.RegressionDetected)
		}

		if result.MemoryRegression != nil {
			t.Logf("  Memory Regression Analysis:")
			t.Logf("    Baseline: %d MB", result.MemoryRegression.BaselineMemoryMB)
			t.Logf("    Current: %d MB", result.MemoryRegression.CurrentMemoryMB)
			t.Logf("    Change: %.2f%%", result.MemoryRegression.MemoryChangePercent)
			t.Logf("    New Memory Leaks: %t", result.MemoryRegression.NewMemoryLeaksDetected)
			t.Logf("    Regression Detected: %t", result.MemoryRegression.RegressionDetected)
		}

		if result.LatencyRegression != nil {
			t.Logf("  Latency Regression Analysis:")
			t.Logf("    P95 Baseline: %d ms", result.LatencyRegression.BaselineP95LatencyMs)
			t.Logf("    P95 Current: %d ms", result.LatencyRegression.CurrentP95LatencyMs)
			t.Logf("    P95 Change: %.2f%%", result.LatencyRegression.P95LatencyChangePercent)
			t.Logf("    Distribution Shift: %t", result.LatencyRegression.LatencyDistributionShift)
			t.Logf("    Regression Detected: %t", result.LatencyRegression.RegressionDetected)
		}

		// Log recommended actions
		if len(result.RecommendedActions) > 0 {
			t.Logf("  Recommended Actions:")
			for i, action := range result.RecommendedActions {
				t.Logf("    %d. %s", i+1, action)
			}
		}

		// Log detailed report
		t.Logf("  Detailed Report:\n%s", result.DetailedReport)

		// Validate regression results
		if result.RegressionDetected && result.RegressionSeverity == "critical" {
			t.Errorf("Critical performance regression detected: score %.2f", result.OverallRegressionScore)
		}

		if result.ThroughputRegression != nil && result.ThroughputRegression.ThroughputChangePercent < -30.0 {
			t.Errorf("Severe throughput regression: %.2f%% decrease",
				result.ThroughputRegression.ThroughputChangePercent)
		}
	})
}

// Helper function to get minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestPerformanceMethodsStressTest runs a comprehensive stress test using all performance methods
func TestPerformanceMethodsStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance stress test in short mode")
	}

	t.Log("=== Comprehensive Performance Stress Test ===")

	// Setup with higher limits for stress testing
	config := &E2ETestConfig{
		TestTimeout:       20 * time.Minute,
		MaxConcurrency:    10,
		RetryAttempts:     1,
		CleanupOnFailure:  true,
		DetailedLogging:   false, // Reduce logging for stress test
		MetricsCollection: true,
		Languages:         []string{"go", "python", "typescript", "java"},
		Scenarios: []E2EScenario{
			E2EScenarioPerformanceValidation,
		},
	}

	runner, err := NewE2ETestRunner(config)
	if err != nil {
		t.Fatalf("Failed to create E2E test runner: %v", err)
	}
	defer runner.Cleanup()

	if err := runner.SetupTestEnvironment(context.Background()); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}

	perfMethods := NewPerformanceTestMethods(
		runner.MockMcpClient,
		runner.TestFramework.PerformanceProfiler,
		runner.TestFramework,
		runner.ctx,
	)

	// Stress test parameters
	stressScenarios := []struct {
		name        string
		concurrency int
		duration    time.Duration
		targetTPS   float64
	}{
		{"Light Load", 25, 2 * time.Minute, 50.0},
		{"Medium Load", 100, 3 * time.Minute, 200.0},
		{"Heavy Load", 200, 2 * time.Minute, 400.0},
		{"Extreme Load", 500, 1 * time.Minute, 800.0},
	}

	stressResults := make(map[string]*LoadTestingResult)

	for _, scenario := range stressScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			t.Logf("Running stress scenario: %s (%d concurrent, %v duration, %.0f target TPS)",
				scenario.name, scenario.concurrency, scenario.duration, scenario.targetTPS)

			startTime := time.Now()

			// Run load test
			loadResult := perfMethods.executeLoadTestingScenario(
				runner.ctx,
				runner.MockMcpClient,
				scenario.concurrency,
				scenario.duration,
			)

			stressResults[scenario.name] = loadResult

			t.Logf("Stress scenario %s completed in %v:", scenario.name, time.Since(startTime))
			t.Logf("  Requests: %d (%.2f%% success)", loadResult.TotalRequests,
				float64(loadResult.SuccessfulRequests)/float64(loadResult.TotalRequests)*100)
			t.Logf("  Throughput: %.2f req/sec (target: %.2f)",
				loadResult.AverageThroughput, scenario.targetTPS)
			t.Logf("  Latency: %dms avg, %dms P95, %dms P99",
				loadResult.AverageLatencyMs, loadResult.P95LatencyMs, loadResult.P99LatencyMs)
			t.Logf("  Memory: %dMB peak, %dMB growth",
				loadResult.PeakMemoryUsageMB, loadResult.MemoryUsageGrowthMB)
			t.Logf("  Error Rate: %.2f%%", loadResult.ErrorRatePercent)

			// Validate stress test results
			if loadResult.ErrorRatePercent > 15.0 {
				t.Errorf("Error rate %.2f%% too high for stress scenario %s",
					loadResult.ErrorRatePercent, scenario.name)
			}

			if scenario.name != "Extreme Load" && loadResult.P95LatencyMs > 5000 {
				t.Errorf("P95 latency %dms too high for stress scenario %s",
					loadResult.P95LatencyMs, scenario.name)
			}

			// Check for system stability
			if loadResult.PeakMemoryUsageMB > 4096 { // 4GB limit
				t.Errorf("Memory usage %dMB too high for stress scenario %s",
					loadResult.PeakMemoryUsageMB, scenario.name)
			}
		})

		// Recovery period between stress scenarios
		time.Sleep(30 * time.Second)
	}

	// Generate stress test summary
	t.Logf("\n=== Stress Test Summary ===")
	totalRequests := int64(0)
	totalDuration := time.Duration(0)
	maxThroughput := 0.0
	maxMemory := int64(0)

	for name, result := range stressResults {
		totalRequests += result.TotalRequests
		totalDuration += result.ActualDuration
		if result.PeakThroughput > maxThroughput {
			maxThroughput = result.PeakThroughput
		}
		if result.PeakMemoryUsageMB > maxMemory {
			maxMemory = result.PeakMemoryUsageMB
		}

		t.Logf("  %s: %d requests, %.2f req/sec peak, %.2f%% errors",
			name, result.TotalRequests, result.PeakThroughput, result.ErrorRatePercent)
	}

	t.Logf("Overall Stress Test Results:")
	t.Logf("  Total Requests: %d", totalRequests)
	t.Logf("  Total Duration: %v", totalDuration)
	t.Logf("  Peak Throughput: %.2f req/sec", maxThroughput)
	t.Logf("  Peak Memory Usage: %d MB", maxMemory)
	t.Logf("  System Stability: %s", func() string {
		if maxMemory < 2048 && maxThroughput > 100 {
			return "Good"
		} else if maxMemory < 4096 && maxThroughput > 50 {
			return "Acceptable"
		} else {
			return "Needs Improvement"
		}
	}())
}
