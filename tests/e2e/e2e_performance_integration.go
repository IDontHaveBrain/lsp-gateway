package e2e_test

import (
	"fmt"
	"testing"
	"time"

	"lsp-gateway/mcp"
	"lsp-gateway/tests/framework"
)

// E2EPerformanceTestSuite integrates performance testing methods with E2E framework
type E2EPerformanceTestSuite struct {
	*E2ETestRunner
	performanceMethods *PerformanceTestMethods
}

// NewE2EPerformanceTestSuite creates a new performance E2E test suite
func NewE2EPerformanceTestSuite(config *E2ETestConfig) (*E2EPerformanceTestSuite, error) {
	runner, err := NewE2ETestRunner(config)
	if err != nil {
		return nil, err
	}

	performanceMethods := NewPerformanceTestMethods(
		runner.MockMcpClient,
		runner.TestFramework.PerformanceProfiler,
		runner.TestFramework,
		runner.ctx,
	)

	return &E2EPerformanceTestSuite{
		E2ETestRunner:      runner,
		performanceMethods: performanceMethods,
	}, nil
}

// Enhanced performance test implementations for E2E scenarios

// executePerformanceValidation implements comprehensive performance validation
func (r *E2ETestRunner) executePerformanceValidation(result *E2ETestResult) error {
	r.Logger.Printf("Executing comprehensive performance validation")

	// Create performance methods instance
	performanceMethods := NewPerformanceTestMethods(
		r.MockMcpClient,
		r.TestFramework.PerformanceProfiler,
		r.TestFramework,
		r.ctx,
	)

	// Capture initial metrics as baseline
	baseline := performanceMethods.captureBaselineMetrics()
	r.Logger.Printf("Captured baseline metrics: %dMB memory, %d goroutines",
		baseline.MemoryUsageMB, baseline.GoroutineCount)

	var allErrors []error

	// 1. Load Testing Scenario
	r.Logger.Printf("Running load testing scenario...")
	loadResult := performanceMethods.executeLoadTestingScenario(
		r.ctx,
		r.MockMcpClient,
		75,             // concurrency
		60*time.Second, // duration
	)

	if !loadResult.Success {
		allErrors = append(allErrors, fmt.Errorf("load testing failed: %v", loadResult.ThresholdViolations))
	}

	result.TotalRequests += loadResult.TotalRequests
	result.SuccessfulReqs += loadResult.SuccessfulRequests
	result.FailedRequests += loadResult.FailedRequests

	// 2. Method Benchmarking
	r.Logger.Printf("Running LSP method benchmarking...")
	testMethods := []string{
		mcp.LSP_METHOD_WORKSPACE_SYMBOL,
		mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION,
		mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES,
		mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER,
		mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS,
	}

	methodResults := make(map[string]*MethodBenchmarkResult)
	for _, method := range testMethods {
		benchmarkResult := performanceMethods.benchmarkLSPMethodPerformance(
			r.ctx,
			r.MockMcpClient,
			method,
			50, // iterations
		)
		methodResults[method] = benchmarkResult

		if benchmarkResult.ErrorRatePercent > 5.0 {
			allErrors = append(allErrors, fmt.Errorf("method %s error rate %.2f%% exceeds threshold",
				method, benchmarkResult.ErrorRatePercent))
		}

		r.Logger.Printf("Method %s: %.2f req/sec, %dms avg latency, %.2f%% errors",
			method, benchmarkResult.ThroughputReqPerSec,
			benchmarkResult.AverageLatencyMs, benchmarkResult.ErrorRatePercent)
	}

	// 3. Memory Usage Pattern Testing
	r.Logger.Printf("Running memory usage pattern testing...")

	// Create a test project for memory testing
	testProject, err := r.TestFramework.CreateMultiLanguageProject(
		framework.ProjectTypeMultiLanguage,
		[]string{"go", "python", "typescript"},
	)
	if err != nil {
		r.Logger.Printf("Warning: Failed to create test project for memory testing: %v", err)
	} else {
		memoryResult := performanceMethods.testMemoryUsagePatterns(
			r.ctx,
			r.MockMcpClient,
			testProject,
		)

		if memoryResult.MemoryLeakDetected {
			allErrors = append(allErrors, fmt.Errorf("memory leak detected: %dMB growth",
				memoryResult.MemoryGrowthMB))
		}

		result.MemoryUsageMB = float64(memoryResult.PeakMemoryMB)

		r.Logger.Printf("Memory analysis: %dMB peak, %dMB growth, leak detected: %v, stability: %.2f",
			memoryResult.PeakMemoryMB, memoryResult.MemoryGrowthMB,
			memoryResult.MemoryLeakDetected, memoryResult.MemoryStabilityScore)
	}

	// 4. Latency Threshold Validation
	r.Logger.Printf("Running latency threshold validation...")
	latencyResult := performanceMethods.validateLatencyThresholds(
		r.ctx,
		r.MockMcpClient,
		2000*time.Millisecond, // 2s expected max latency
	)

	if !latencyResult.ValidationSuccess {
		allErrors = append(allErrors, fmt.Errorf("latency validation failed: %.1f%% violations",
			latencyResult.ViolationPercent))
	}

	if len(latencyResult.LatencyTrend) > 0 {
		lastTrend := latencyResult.LatencyTrend[len(latencyResult.LatencyTrend)-1]
		result.AverageLatency = time.Duration(lastTrend.P95LatencyMs) * time.Millisecond
	}

	r.Logger.Printf("Latency validation: avg %dms, P95 %dms, %.1f%% violations, success: %v",
		latencyResult.ActualAverageLatencyMs, latencyResult.P95LatencyMs,
		latencyResult.ViolationPercent, latencyResult.ValidationSuccess)

	// 5. Throughput Limits Testing
	r.Logger.Printf("Running throughput limits testing...")
	throughputResult := performanceMethods.testThroughputLimits(
		r.ctx,
		r.MockMcpClient,
		100.0, // target 100 req/sec
	)

	if !throughputResult.ThresholdValidation {
		allErrors = append(allErrors, fmt.Errorf("throughput validation failed: actual %.2f req/sec below target %.2f req/sec",
			throughputResult.ActualThroughputReqPerSec, throughputResult.TargetThroughputReqPerSec))
	}

	result.ThroughputPerSecond = throughputResult.ActualThroughputReqPerSec

	r.Logger.Printf("Throughput analysis: peak %.2f req/sec, sustained %.2f req/sec, efficiency %.2f",
		throughputResult.PeakThroughputReqPerSec, throughputResult.SustainedThroughput,
		throughputResult.ThroughputEfficiency)

	// 6. Performance Regression Testing (if baseline exists)
	r.Logger.Printf("Running performance regression testing...")

	// Create a sample baseline for demonstration
	regressionBaseline := &PerformanceRegressionBaseline{
		Version:   "baseline-v1.0",
		Timestamp: time.Now().Add(-24 * time.Hour),
		MethodLatencies: map[string]int64{
			mcp.LSP_METHOD_WORKSPACE_SYMBOL:         100,
			mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION: 50,
			mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES: 80,
			mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER:      30,
			mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS:    40,
		},
		ThroughputData: map[string]float64{
			"overall":                               95.0,
			mcp.LSP_METHOD_WORKSPACE_SYMBOL:         15.0,
			mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION: 30.0,
			mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES: 20.0,
			mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER:      25.0,
			mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS:    10.0,
		},
		MemoryUsageData: map[string]int64{
			"peak":   512,
			"growth": 50,
		},
		ErrorRates: map[string]float64{
			mcp.LSP_METHOD_WORKSPACE_SYMBOL:         2.0,
			mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION: 1.0,
			mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES: 1.5,
			mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER:      0.5,
			mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS:    1.0,
		},
		ValidationResults: map[string]bool{
			"memory-leak-detected": false,
		},
	}

	regressionResult := performanceMethods.executePerformanceRegressionTest(
		r.ctx,
		r.MockMcpClient,
		regressionBaseline,
	)

	if regressionResult.RegressionDetected {
		allErrors = append(allErrors, fmt.Errorf("performance regression detected: severity %s, score %.2f",
			regressionResult.RegressionSeverity, regressionResult.OverallRegressionScore))
	}

	r.Logger.Printf("Regression analysis: regression detected: %v, severity: %s, score: %.2f",
		regressionResult.RegressionDetected, regressionResult.RegressionSeverity,
		regressionResult.OverallRegressionScore)

	// Log detailed regression report
	if regressionResult.RegressionDetected {
		r.Logger.Printf("Regression Report:\n%s", regressionResult.DetailedReport)

		r.Logger.Printf("Recommended Actions:")
		for i, action := range regressionResult.RecommendedActions {
			r.Logger.Printf("  %d. %s", i+1, action)
		}
	}

	// Update overall result
	if len(allErrors) > 0 {
		result.Errors = append(result.Errors, allErrors...)
		r.Logger.Printf("Performance validation completed with %d errors", len(allErrors))
		return fmt.Errorf("performance validation failed with %d issues", len(allErrors))
	}

	r.Logger.Printf("Performance validation completed successfully")
	return nil
}

// executeConcurrentRequests implements concurrent request handling testing
func (r *E2ETestRunner) executeConcurrentRequests(result *E2ETestResult) error {
	r.Logger.Printf("Executing concurrent requests testing")

	// Create performance methods instance
	performanceMethods := NewPerformanceTestMethods(
		r.MockMcpClient,
		r.TestFramework.PerformanceProfiler,
		r.TestFramework,
		r.ctx,
	)

	// Define concurrency test scenarios
	concurrencyScenarios := []struct {
		name        string
		concurrency int
		duration    time.Duration
		description string
	}{
		{
			name:        "Low Concurrency",
			concurrency: 25,
			duration:    30 * time.Second,
			description: "Test system behavior with low concurrent load",
		},
		{
			name:        "Medium Concurrency",
			concurrency: 75,
			duration:    45 * time.Second,
			description: "Test system behavior with medium concurrent load",
		},
		{
			name:        "High Concurrency",
			concurrency: 150,
			duration:    60 * time.Second,
			description: "Test system behavior with high concurrent load",
		},
		{
			name:        "Stress Concurrency",
			concurrency: 300,
			duration:    30 * time.Second,
			description: "Test system limits under stress conditions",
		},
	}

	var allErrors []error
	var totalRequests, totalSuccessful, totalFailed int64

	for _, scenario := range concurrencyScenarios {
		r.Logger.Printf("Running concurrent scenario: %s (%d concurrent requests)",
			scenario.name, scenario.concurrency)

		loadResult := performanceMethods.executeLoadTestingScenario(
			r.ctx,
			r.MockMcpClient,
			scenario.concurrency,
			scenario.duration,
		)

		totalRequests += loadResult.TotalRequests
		totalSuccessful += loadResult.SuccessfulRequests
		totalFailed += loadResult.FailedRequests

		r.Logger.Printf("Scenario %s results: %d requests, %.2f req/sec, %.2f%% errors, %dms P95",
			scenario.name, loadResult.TotalRequests, loadResult.AverageThroughput,
			loadResult.ErrorRatePercent, loadResult.P95LatencyMs)

		// Validate scenario results
		if !loadResult.Success {
			allErrors = append(allErrors, fmt.Errorf("concurrent scenario %s failed: %v",
				scenario.name, loadResult.ThresholdViolations))
		}

		// Check specific thresholds for each scenario
		switch scenario.name {
		case "Low Concurrency":
			if loadResult.ErrorRatePercent > 1.0 {
				allErrors = append(allErrors, fmt.Errorf("low concurrency error rate %.2f%% too high",
					loadResult.ErrorRatePercent))
			}
			if loadResult.AverageThroughput < 50.0 {
				allErrors = append(allErrors, fmt.Errorf("low concurrency throughput %.2f req/sec too low",
					loadResult.AverageThroughput))
			}

		case "Medium Concurrency":
			if loadResult.ErrorRatePercent > 3.0 {
				allErrors = append(allErrors, fmt.Errorf("medium concurrency error rate %.2f%% too high",
					loadResult.ErrorRatePercent))
			}
			if loadResult.P95LatencyMs > 1000 {
				allErrors = append(allErrors, fmt.Errorf("medium concurrency P95 latency %dms too high",
					loadResult.P95LatencyMs))
			}

		case "High Concurrency":
			if loadResult.ErrorRatePercent > 5.0 {
				allErrors = append(allErrors, fmt.Errorf("high concurrency error rate %.2f%% too high",
					loadResult.ErrorRatePercent))
			}
			if loadResult.P95LatencyMs > 2000 {
				allErrors = append(allErrors, fmt.Errorf("high concurrency P95 latency %dms too high",
					loadResult.P95LatencyMs))
			}

		case "Stress Concurrency":
			// More lenient thresholds for stress testing
			if loadResult.ErrorRatePercent > 10.0 {
				allErrors = append(allErrors, fmt.Errorf("stress concurrency error rate %.2f%% too high",
					loadResult.ErrorRatePercent))
			}
		}

		// Add brief recovery time between scenarios
		time.Sleep(5 * time.Second)
	}

	// Update result with aggregated metrics
	result.TotalRequests = totalRequests
	result.SuccessfulReqs = totalSuccessful
	result.FailedRequests = totalFailed

	if totalRequests > 0 {
		result.AverageLatency = time.Duration(float64(totalSuccessful)) * time.Millisecond // Simplified
		errorRate := float64(totalFailed) / float64(totalRequests) * 100

		r.Logger.Printf("Concurrent requests testing summary: %d total requests, %.2f%% overall error rate",
			totalRequests, errorRate)
	}

	if len(allErrors) > 0 {
		result.Errors = append(result.Errors, allErrors...)
		return fmt.Errorf("concurrent requests testing failed with %d issues", len(allErrors))
	}

	r.Logger.Printf("Concurrent requests testing completed successfully")
	return nil
}

// executeHighThroughputTesting implements high throughput testing
func (r *E2ETestRunner) executeHighThroughputTesting(result *E2ETestResult) error {
	r.Logger.Printf("Executing high throughput testing")

	// Create performance methods instance
	performanceMethods := NewPerformanceTestMethods(
		r.MockMcpClient,
		r.TestFramework.PerformanceProfiler,
		r.TestFramework,
		r.ctx,
	)

	// Define throughput test scenarios
	throughputTargets := []float64{50.0, 100.0, 200.0, 500.0, 1000.0}
	var allErrors []error
	maxAchievedThroughput := 0.0

	for _, targetTPS := range throughputTargets {
		r.Logger.Printf("Testing throughput target: %.2f req/sec", targetTPS)

		throughputResult := performanceMethods.testThroughputLimits(
			r.ctx,
			r.MockMcpClient,
			targetTPS,
		)

		r.Logger.Printf("Throughput test results for %.2f req/sec target:", targetTPS)
		r.Logger.Printf("  - Actual throughput: %.2f req/sec", throughputResult.ActualThroughputReqPerSec)
		r.Logger.Printf("  - Peak throughput: %.2f req/sec", throughputResult.PeakThroughputReqPerSec)
		r.Logger.Printf("  - Efficiency: %.2f", throughputResult.ThroughputEfficiency)
		r.Logger.Printf("  - Optimal concurrency: %d", throughputResult.OptimalConcurrency)

		if throughputResult.PeakThroughputReqPerSec > maxAchievedThroughput {
			maxAchievedThroughput = throughputResult.PeakThroughputReqPerSec
		}

		// Validate throughput results
		if !throughputResult.ThresholdValidation {
			r.Logger.Printf("Warning: Failed to achieve throughput target %.2f req/sec (achieved %.2f req/sec)",
				targetTPS, throughputResult.ActualThroughputReqPerSec)
		}

		// Check for bottlenecks
		if len(throughputResult.BottleneckPoints) > 0 {
			r.Logger.Printf("Bottlenecks detected:")
			for i, bottleneck := range throughputResult.BottleneckPoints {
				r.Logger.Printf("  %d. Limiting factor: %s at %.2f req/sec",
					i+1, bottleneck.LimitingFactor, bottleneck.ThroughputReqPerSec)
			}
		}

		// Check for resource limitations
		if len(throughputResult.ResourceLimitations) > 0 {
			r.Logger.Printf("Resource limitations detected:")
			for i, limitation := range throughputResult.ResourceLimitations {
				r.Logger.Printf("  %d. %s: %.1f%% utilization (limit reached: %v)",
					i+1, limitation.ResourceType, limitation.UtilizationPercent, limitation.LimitReached)
			}
		}

		// Stop testing if we've clearly hit the system limit
		if throughputResult.ThroughputEfficiency < 0.5 && targetTPS > 100.0 {
			r.Logger.Printf("Throughput efficiency dropped below 50%%, stopping throughput testing")
			break
		}
	}

	// Update result
	result.ThroughputPerSecond = maxAchievedThroughput

	r.Logger.Printf("High throughput testing completed: max achieved throughput %.2f req/sec",
		maxAchievedThroughput)

	if len(allErrors) > 0 {
		result.Errors = append(result.Errors, allErrors...)
		return fmt.Errorf("high throughput testing failed with %d issues", len(allErrors))
	}

	return nil
}

// Example test function showing how to use the enhanced E2E performance testing
func TestE2EPerformanceValidationIntegrated(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integrated E2E performance validation in short mode")
	}

	// Create E2E test configuration
	config := &E2ETestConfig{
		TestTimeout:       15 * time.Minute,
		MaxConcurrency:    10,
		RetryAttempts:     3,
		CleanupOnFailure:  true,
		DetailedLogging:   true,
		MetricsCollection: true,
		Languages:         []string{"go", "python", "typescript"},
		Scenarios: []E2EScenario{
			E2EScenarioPerformanceValidation,
			E2EScenarioConcurrentRequests,
			E2EScenarioHighThroughputTesting,
		},
	}

	// Create performance test suite
	suite, err := NewE2EPerformanceTestSuite(config)
	if err != nil {
		t.Fatalf("Failed to create E2E performance test suite: %v", err)
	}
	defer suite.Cleanup()

	// Setup test environment
	if err := suite.SetupTestEnvironment(suite.ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}

	// Execute performance validation scenarios
	t.Run("PerformanceValidation", func(t *testing.T) {
		result, err := suite.ExecuteScenario(E2EScenarioPerformanceValidation)
		if err != nil {
			t.Errorf("Performance validation scenario failed: %v", err)
		}

		// Validate key performance metrics
		if result.TotalRequests < 100 {
			t.Errorf("Expected at least 100 requests, got %d", result.TotalRequests)
		}

		if result.ThroughputPerSecond < 10.0 {
			t.Errorf("Expected throughput at least 10 req/sec, got %.2f", result.ThroughputPerSecond)
		}

		errorRate := float64(result.FailedRequests) / float64(result.TotalRequests) * 100
		if errorRate > 5.0 {
			t.Errorf("Error rate %.2f%% exceeds 5%% threshold", errorRate)
		}

		t.Logf("Performance validation completed: %d requests, %.2f req/sec, %.2f%% errors",
			result.TotalRequests, result.ThroughputPerSecond, errorRate)
	})

	t.Run("ConcurrentRequests", func(t *testing.T) {
		result, err := suite.ExecuteScenario(E2EScenarioConcurrentRequests)
		if err != nil {
			t.Errorf("Concurrent requests scenario failed: %v", err)
		}

		if result.TotalRequests < 500 {
			t.Errorf("Expected at least 500 concurrent requests, got %d", result.TotalRequests)
		}

		t.Logf("Concurrent requests completed: %d requests, %.2f%% success rate",
			result.TotalRequests, float64(result.SuccessfulReqs)/float64(result.TotalRequests)*100)
	})

	t.Run("HighThroughputTesting", func(t *testing.T) {
		result, err := suite.ExecuteScenario(E2EScenarioHighThroughputTesting)
		if err != nil {
			t.Errorf("High throughput testing scenario failed: %v", err)
		}

		if result.ThroughputPerSecond < 50.0 {
			t.Errorf("Expected peak throughput at least 50 req/sec, got %.2f", result.ThroughputPerSecond)
		}

		t.Logf("High throughput testing completed: peak %.2f req/sec", result.ThroughputPerSecond)
	})

	// Generate comprehensive performance report
	results := suite.GetTestResults()
	generatePerformanceTestReport(t, results)
}

// generatePerformanceTestReport generates a comprehensive performance test report
func generatePerformanceTestReport(t *testing.T, results []*E2ETestResult) {
	report := `
=================================================
E2E Performance Test Report
=================================================
`

	totalRequests := int64(0)
	totalSuccessful := int64(0)
	totalFailed := int64(0)
	totalDuration := time.Duration(0)
	maxThroughput := 0.0
	totalErrors := 0

	for _, result := range results {
		totalRequests += result.TotalRequests
		totalSuccessful += result.SuccessfulReqs
		totalFailed += result.FailedRequests
		totalDuration += result.Duration
		totalErrors += len(result.Errors)

		if result.ThroughputPerSecond > maxThroughput {
			maxThroughput = result.ThroughputPerSecond
		}

		report += fmt.Sprintf(`
Scenario: %s
- Duration: %v
- Requests: %d (Success: %d, Failed: %d)
- Throughput: %.2f req/sec
- Memory Usage: %.2f MB
- Errors: %d
- Success: %t
`,
			result.Scenario,
			result.Duration,
			result.TotalRequests,
			result.SuccessfulReqs,
			result.FailedRequests,
			result.ThroughputPerSecond,
			result.MemoryUsageMB,
			len(result.Errors),
			result.Success)
	}

	report += fmt.Sprintf(`
=================================================
Summary
=================================================
Total Test Duration: %v
Total Requests: %d
Success Rate: %.2f%%
Peak Throughput: %.2f req/sec
Total Errors: %d
Overall Success Rate: %.2f%%
`,
		totalDuration,
		totalRequests,
		float64(totalSuccessful)/float64(totalRequests)*100,
		maxThroughput,
		totalErrors,
		float64(len(results)-totalErrors)/float64(len(results))*100)

	t.Log(report)
}
