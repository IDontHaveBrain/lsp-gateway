package e2e_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"lsp-gateway/mcp"
	"lsp-gateway/tests/framework"
	"lsp-gateway/tests/mocks"
)

// ExampleE2ETestWithReporting demonstrates how to use the comprehensive reporting system
// with actual E2E tests. This example shows integration patterns and best practices.
func ExampleE2ETestWithReporting() {
	// 1. Setup the reporting system
	reportingConfig := &ReportingConfig{
		OutputDirectory:           "./e2e-reports",
		EnableRealtimeReporting:   true,
		EnableHistoricalTracking:  true,
		ReportFormats:            []ReportFormat{FormatConsole, FormatJSON, FormatHTML},
		MetricsCollectionInterval: 2 * time.Second,
		ProgressReportingInterval: 10 * time.Second,
		RetentionDays:            30,
		DetailLevel:              DetailLevelDetailed,
		PerformanceThresholds: &PerformanceThresholds{
			MaxAverageResponseTime: 3 * time.Second,
			MinThroughputPerSecond: 75.0,
			MaxErrorRatePercent:    8.0,
			MaxMemoryUsageMB:       2048,
			MaxCPUUsagePercent:     85.0,
			MaxDurationSeconds:     1800, // 30 minutes max
		},
	}

	reportingSystem, err := NewReportingSystem(reportingConfig)
	if err != nil {
		log.Fatalf("Failed to create reporting system: %v", err)
	}
	defer reportingSystem.Cleanup()

	// 2. Setup E2E test runner
	e2eConfig := &E2ETestConfig{
		TestTimeout:       15 * time.Minute,
		MaxConcurrency:    4,
		RetryAttempts:     2,
		CleanupOnFailure:  true,
		DetailedLogging:   true,
		MetricsCollection: true,
		ProjectTypes: []framework.ProjectType{
			framework.ProjectTypeMultiLanguage,
			framework.ProjectTypeMonorepo,
		},
		Languages: []string{"go", "python", "typescript", "java"},
		Scenarios: []E2EScenario{
			E2EScenarioBasicMCPWorkflow,
			E2EScenarioLSPMethodValidation,
			E2EScenarioMultiLanguageSupport,
			E2EScenarioWorkspaceManagement,
			E2EScenarioCircuitBreakerTesting,
			E2EScenarioPerformanceValidation,
		},
	}

	e2eRunner, err := NewE2ETestRunner(e2eConfig)
	if err != nil {
		log.Fatalf("Failed to create E2E test runner: %v", err)
	}
	defer e2eRunner.Cleanup()

	// 3. Setup test environment
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	if err := e2eRunner.SetupTestEnvironment(ctx); err != nil {
		log.Fatalf("Failed to setup E2E test environment: %v", err)
	}

	// 4. Start reporting session
	totalScenarios := int64(len(e2eConfig.Scenarios))
	if err := reportingSystem.StartSession(totalScenarios); err != nil {
		log.Fatalf("Failed to start reporting session: %v", err)
	}

	log.Printf("Starting E2E test execution with comprehensive reporting...")
	log.Printf("Running %d scenarios with %d max concurrency", totalScenarios, e2eConfig.MaxConcurrency)

	// 5. Execute all scenarios with integrated reporting
	results := make([]*E2ETestResult, 0, len(e2eConfig.Scenarios))
	
	for i, scenario := range e2eConfig.Scenarios {
		log.Printf("[%d/%d] Executing scenario: %s", i+1, len(e2eConfig.Scenarios), scenario)
		
		// Execute the scenario
		scenarioStartTime := time.Now()
		result, err := e2eRunner.ExecuteScenario(scenario)
		scenarioEndTime := time.Now()
		
		if err != nil {
			log.Printf("Scenario %s failed: %v", scenario, err)
			// Still record the result for analysis
		}
		
		results = append(results, result)
		
		// Convert E2E result to reporting format
		testResult := convertE2EToTestResult(result, scenario, scenarioStartTime, scenarioEndTime)
		
		// Record the result in the reporting system
		reportingSystem.RecordTestResult(testResult)
		
		// Record MCP client metrics
		if e2eRunner.MockMcpClient != nil {
			reportingSystem.RecordMCPMetrics(e2eRunner.MockMcpClient)
		}
		
		// Record system metrics
		reportingSystem.RecordSystemMetrics()
		
		// Log progress
		if result != nil && result.Success {
			log.Printf("✓ Scenario %s completed successfully in %v", scenario, result.Duration)
		} else {
			log.Printf("✗ Scenario %s failed after %v", scenario, time.Since(scenarioStartTime))
		}
		
		// Brief pause between scenarios to allow system to stabilize
		time.Sleep(1 * time.Second)
	}

	// 6. Generate comprehensive report
	log.Printf("Generating comprehensive E2E test report...")
	
	report, err := reportingSystem.GenerateReport()
	if err != nil {
		log.Fatalf("Failed to generate comprehensive report: %v", err)
	}

	// 7. Display summary results
	displayTestSummary(report)

	// 8. Check for critical issues and provide recommendations
	if report.ExecutiveSummary != nil {
		if len(report.ExecutiveSummary.CriticalIssues) > 0 {
			log.Printf("\n🚨 CRITICAL ISSUES DETECTED:")
			for _, issue := range report.ExecutiveSummary.CriticalIssues {
				log.Printf("  • %s", issue)
			}
		}

		if len(report.ExecutiveSummary.RecommendedActions) > 0 {
			log.Printf("\n💡 RECOMMENDED ACTIONS:")
			for _, action := range report.ExecutiveSummary.RecommendedActions {
				log.Printf("  → %s", action)
			}
		}
	}

	log.Printf("\n📊 Comprehensive reports have been generated in: %s", reportingConfig.OutputDirectory)
	log.Printf("E2E test execution with reporting completed successfully!")

	// Output:
	// E2E test execution with comprehensive reporting completed successfully!
}

// ExampleReportingSystemStandalone demonstrates using the reporting system independently
func ExampleReportingSystemStandalone() {
	// Create a simple reporting configuration
	config := &ReportingConfig{
		OutputDirectory:           "./standalone-reports",
		EnableRealtimeReporting:   false, // Disabled for simplicity
		EnableHistoricalTracking:  false,
		ReportFormats:            []ReportFormat{FormatConsole, FormatJSON},
		DetailLevel:              DetailLevelStandard,
	}

	// Initialize the reporting system
	reportingSystem, err := NewReportingSystem(config)
	if err != nil {
		log.Fatalf("Failed to create reporting system: %v", err)
	}
	defer reportingSystem.Cleanup()

	// Start a test session
	testCount := int64(5)
	if err := reportingSystem.StartSession(testCount); err != nil {
		log.Fatalf("Failed to start session: %v", err)
	}

	// Simulate some test results
	for i := int64(0); i < testCount; i++ {
		testResult := &TestResult{
			TestID:    fmt.Sprintf("standalone-test-%d", i+1),
			TestName:  fmt.Sprintf("Standalone Test %d", i+1),
			Scenario:  "standalone-scenario",
			Language:  "go",
			StartTime: time.Now().Add(-time.Second),
			EndTime:   time.Now(),
			Duration:  time.Second,
			Status:    TestStatusPassed,
			Success:   true,
			Errors:    []TestError{},
			Warnings:  []TestWarning{},
			Metrics: &TestResultMetrics{
				RequestCount:        10,
				SuccessfulRequests:  10,
				FailedRequests:      0,
				AverageLatency:      50 * time.Millisecond,
				ThroughputPerSecond: 200.0,
				ErrorRate:           0.0,
			},
		}

		reportingSystem.RecordTestResult(testResult)
		reportingSystem.RecordSystemMetrics()
	}

	// Generate and display report
	report, err := reportingSystem.GenerateReport()
	if err != nil {
		log.Fatalf("Failed to generate report: %v", err)
	}

	fmt.Printf("Standalone Test Report Summary:\n")
	fmt.Printf("  Total Tests: %d\n", report.TestMetrics.TotalTests)
	fmt.Printf("  Success Rate: %.1f%%\n", report.TestMetrics.SuccessRate)
	fmt.Printf("  Total Duration: %v\n", report.TestMetrics.TotalDuration)

	// Output:
	// Standalone Test Report Summary:
	//   Total Tests: 5
	//   Success Rate: 100.0%
	//   Total Duration: 5s
}

// ExampleCustomMetricsCollection demonstrates collecting custom metrics
func ExampleCustomMetricsCollection() {
	config := getDefaultReportingConfig()
	config.OutputDirectory = "./custom-metrics-reports"
	config.CustomMetrics = []string{"custom_latency", "custom_throughput", "business_metric"}

	reportingSystem, err := NewReportingSystem(config)
	if err != nil {
		log.Fatalf("Failed to create reporting system: %v", err)
	}
	defer reportingSystem.Cleanup()

	// Start session
	if err := reportingSystem.StartSession(3); err != nil {
		log.Fatalf("Failed to start session: %v", err)
	}

	// Create test results with custom performance data
	for i := 0; i < 3; i++ {
		testResult := &TestResult{
			TestID:   fmt.Sprintf("custom-test-%d", i+1),
			TestName: fmt.Sprintf("Custom Metrics Test %d", i+1),
			Scenario: "custom-metrics-scenario",
			Language: "go",
			Duration: time.Duration(i+1) * time.Second,
			Success:  true,
			Status:   TestStatusPassed,
			Metadata: map[string]interface{}{
				"custom_latency":    float64((i + 1) * 100), // milliseconds
				"custom_throughput": float64(1000 - i*100),  // requests per second
				"business_metric":   float64((i + 1) * 50),  // arbitrary business value
				"test_complexity":   i + 1,
				"resource_cost":     float64(i+1) * 1.5,
			},
			PerformanceData: &TestPerformanceData{
				PerformanceScore: float64(95 - i*5),
				ComparedToBaseline: &BaselineComparison{
					BaselineExists:     true,
					DurationChange:     float64(i * 10),    // 0%, 10%, 20% slower
					ThroughputChange:   float64(i * -5),    // 0%, -5%, -10% throughput
					MemoryChange:       float64(i * 15),    // 0%, 15%, 30% more memory
					OverallImprovement: float64(-i * 8),    // Getting worse
					RegressionDetected: i > 0,              // Regression after first test
				},
			},
		}

		reportingSystem.RecordTestResult(testResult)
	}

	// Generate report with custom metrics
	report, err := reportingSystem.GenerateReport()
	if err != nil {
		log.Fatalf("Failed to generate report: %v", err)
	}

	// Display custom metrics summary
	fmt.Printf("Custom Metrics Report:\n")
	for _, result := range report.TestResults {
		if result.Metadata != nil {
			fmt.Printf("  Test %s:\n", result.TestName)
			for key, value := range result.Metadata {
				fmt.Printf("    %s: %v\n", key, value)
			}
		}
	}

	// Output:
	// Custom Metrics Report:
	//   Test Custom Metrics Test 1:
	//     custom_latency: 100
	//     custom_throughput: 1000
	//     business_metric: 50
	//     test_complexity: 1
	//     resource_cost: 1.5
}

// Helper functions for the examples

func convertE2EToTestResult(e2eResult *E2ETestResult, scenario E2EScenario, startTime, endTime time.Time) *TestResult {
	if e2eResult == nil {
		// Create a failed result if the scenario didn't return a result
		return &TestResult{
			TestID:    fmt.Sprintf("e2e-%s-%d", scenario, time.Now().Unix()),
			TestName:  fmt.Sprintf("E2E: %s", scenario),
			Scenario:  string(scenario),
			Language:  "multi-language",
			StartTime: startTime,
			EndTime:   endTime,
			Duration:  endTime.Sub(startTime),
			Status:    TestStatusError,
			Success:   false,
			Errors: []TestError{
				{
					Type:        ErrorTypeSystem,
					Category:    ErrorCategoryInfrastructure,
					Message:     "E2E scenario returned nil result",
					Timestamp:   endTime,
					Severity:    ErrorSeverityCritical,
					Recoverable: false,
				},
			},
			Warnings: []TestWarning{},
		}
	}

	// Convert E2E errors to test errors
	var errors []TestError
	for _, err := range e2eResult.Errors {
		errors = append(errors, TestError{
			Type:        determineErrorType(err.Error()),
			Category:    ErrorCategoryTest,
			Message:     err.Error(),
			Timestamp:   endTime,
			Severity:    ErrorSeverityMedium,
			Recoverable: true,
		})
	}

	// Convert warnings
	var warnings []TestWarning
	for _, warning := range e2eResult.Warnings {
		warnings = append(warnings, TestWarning{
			Type:      WarningTypePerformance,
			Message:   warning,
			Timestamp: endTime,
		})
	}

	// Determine status
	status := TestStatusPassed
	if !e2eResult.Success {
		status = TestStatusFailed
	}

	return &TestResult{
		TestID:    fmt.Sprintf("e2e-%s-%d", scenario, e2eResult.StartTime.Unix()),
		TestName:  fmt.Sprintf("E2E: %s", scenario),
		Scenario:  string(scenario),
		Language:  "multi-language",
		StartTime: e2eResult.StartTime,
		EndTime:   e2eResult.EndTime,
		Duration:  e2eResult.Duration,
		Status:    status,
		Success:   e2eResult.Success,
		Errors:    errors,
		Warnings:  warnings,
		Metrics: &TestResultMetrics{
			RequestCount:        e2eResult.TotalRequests,
			SuccessfulRequests:  e2eResult.SuccessfulReqs,
			FailedRequests:      e2eResult.FailedRequests,
			AverageLatency:      e2eResult.AverageLatency,
			ThroughputPerSecond: calculateThroughput(e2eResult.TotalRequests, e2eResult.Duration),
			ErrorRate:           calculateErrorRate(e2eResult.FailedRequests, e2eResult.TotalRequests),
		},
		ResourceUsage: &ResourceUsage{
			MemoryUsedMB:       e2eResult.MemoryUsageMB,
			CPUTimeMS:          int64(e2eResult.CPUUsagePercent * 1000), // Rough approximation
			GoroutinesCreated:  e2eResult.GoroutineCount,
		},
		PerformanceData: &TestPerformanceData{
			PerformanceScore: calculatePerformanceScoreFromE2E(e2eResult),
		},
		Metadata: map[string]interface{}{
			"e2e_scenario":        scenario,
			"test_environment":    e2eResult.TestEnvironment,
			"config_used":         e2eResult.ConfigUsed,
			"mcp_metrics_available": e2eResult.MCPMetrics != nil,
			"framework_results_available": e2eResult.FrameworkResults != nil,
		},
	}
}

func determineErrorType(errorMessage string) ErrorType {
	// Simple error type determination based on message content
	switch {
	case contains(errorMessage, "timeout"):
		return ErrorTypeTimeout
	case contains(errorMessage, "connection"):
		return ErrorTypeConnection
	case contains(errorMessage, "protocol"):
		return ErrorTypeProtocol
	case contains(errorMessage, "assert"):
		return ErrorTypeAssertion
	case contains(errorMessage, "resource"):
		return ErrorTypeResource
	default:
		return ErrorTypeApplication
	}
}

func contains(str, substr string) bool {
	return len(str) >= len(substr) && 
		   (str == substr || 
		    str[:len(substr)] == substr || 
		    str[len(str)-len(substr):] == substr ||
		    len(str) > len(substr) && 
		    (str[1:len(substr)+1] == substr || 
		     str[len(str)-len(substr)-1:len(str)-1] == substr))
}

func calculateThroughput(totalRequests int64, duration time.Duration) float64 {
	if duration <= 0 {
		return 0.0
	}
	return float64(totalRequests) / duration.Seconds()
}

func calculateErrorRate(failedRequests, totalRequests int64) float64 {
	if totalRequests == 0 {
		return 0.0
	}
	return float64(failedRequests) / float64(totalRequests) * 100.0
}

func calculatePerformanceScoreFromE2E(result *E2ETestResult) float64 {
	score := 100.0
	
	// Reduce score based on errors
	if result.TotalRequests > 0 {
		errorRate := float64(result.FailedRequests) / float64(result.TotalRequests) * 100
		score -= errorRate * 2 // Each percent error reduces score by 2
	}
	
	// Reduce score based on duration (if it's unusually long)
	if result.Duration > 5*time.Minute {
		score -= 10.0
	}
	
	// Reduce score based on resource usage
	if result.MemoryUsageMB > 1000 {
		score -= 5.0
	}
	
	if result.CPUUsagePercent > 80 {
		score -= 10.0
	}
	
	return max(0.0, score)
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func displayTestSummary(report *ComprehensiveReport) {
	if report == nil || report.ExecutiveSummary == nil {
		log.Printf("No summary available")
		return
	}

	summary := report.ExecutiveSummary
	
	log.Printf("\n" + "="*80)
	log.Printf("                    E2E TEST EXECUTION SUMMARY")
	log.Printf("="*80)
	
	// Overall status with emoji
	statusEmoji := "✅"
	if summary.OverallStatus == TestStatusFailed {
		statusEmoji = "❌"
	}
	log.Printf("%s Overall Status: %s", statusEmoji, summary.OverallStatus)
	
	// Key metrics
	log.Printf("📊 Test Metrics:")
	log.Printf("   Total Tests: %d", summary.TotalTests)
	log.Printf("   Pass Rate: %.1f%%", summary.PassRate)
	log.Printf("   Duration: %v", summary.Duration)
	
	// Scores
	log.Printf("🎯 Quality Scores:")
	log.Printf("   Performance: %.1f/100", summary.PerformanceScore)
	log.Printf("   Quality: %.1f/100", summary.QualityScore)
	log.Printf("   Coverage: %.1f%%", summary.CoverageScore)
	
	// System metrics
	if report.SystemMetrics != nil {
		log.Printf("💻 System Resources:")
		log.Printf("   Peak Memory: %.1f MB", report.SystemMetrics.PeakMemoryMB)
		log.Printf("   Memory Growth: %.1f MB", report.SystemMetrics.MemoryGrowthMB)
		log.Printf("   Peak Goroutines: %d", report.SystemMetrics.PeakGoroutines)
	}
	
	// MCP metrics
	if report.MCPMetrics != nil {
		log.Printf("🔗 MCP Client Metrics:")
		log.Printf("   Total Requests: %d", report.MCPMetrics.TotalRequests)
		log.Printf("   Success Rate: %.1f%%", 
			float64(report.MCPMetrics.SuccessfulRequests)/float64(report.MCPMetrics.TotalRequests)*100)
		log.Printf("   Average Latency: %v", report.MCPMetrics.AverageLatency)
		log.Printf("   Throughput: %.1f req/sec", report.MCPMetrics.ThroughputPerSecond)
	}
	
	log.Printf("="*80)
}