package e2e_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-gateway/mcp"
	"lsp-gateway/tests/framework"
	"lsp-gateway/tests/mocks"
)

// TestReportingSystemIntegration demonstrates the comprehensive reporting system integration
func TestReportingSystemIntegration(t *testing.T) {
	// Setup temporary directory for reports
	tempDir, err := os.MkdirTemp("", "e2e-reporting-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Configure reporting system
	config := &ReportingConfig{
		OutputDirectory:           tempDir,
		EnableRealtimeReporting:   true,
		EnableHistoricalTracking:  false, // Disabled for test
		ReportFormats:             []ReportFormat{FormatConsole, FormatJSON, FormatHTML, FormatCSV},
		MetricsCollectionInterval: 100 * time.Millisecond,
		ProgressReportingInterval: 500 * time.Millisecond,
		RetentionDays:             7,
		DetailLevel:               DetailLevelDetailed,
		PerformanceThresholds: &PerformanceThresholds{
			MaxAverageResponseTime: 2 * time.Second,
			MinThroughputPerSecond: 50.0,
			MaxErrorRatePercent:    10.0,
			MaxMemoryUsageMB:       2048,
			MaxCPUUsagePercent:     70.0,
			MaxDurationSeconds:     300,
		},
	}

	// Create reporting system
	reportingSystem, err := NewReportingSystem(config)
	if err != nil {
		t.Fatalf("Failed to create reporting system: %v", err)
	}
	defer reportingSystem.Cleanup()

	// Start testing session
	totalTests := int64(15)
	if err := reportingSystem.StartSession(totalTests); err != nil {
		t.Fatalf("Failed to start session: %v", err)
	}

	// Simulate comprehensive E2E test execution
	t.Run("SimulateE2ETestExecution", func(t *testing.T) {
		runSimulatedE2ETests(t, reportingSystem, totalTests)
	})

	// Generate comprehensive report
	t.Run("GenerateComprehensiveReport", func(t *testing.T) {
		report, err := reportingSystem.GenerateReport()
		if err != nil {
			t.Fatalf("Failed to generate report: %v", err)
		}

		// Validate report structure
		validateReportStructure(t, report)

		// Verify report files were created
		verifyReportFiles(t, tempDir)
	})
}

// TestReportingSystemWithRealE2ERunner demonstrates integration with actual E2E test runner
func TestReportingSystemWithRealE2ERunner(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comprehensive integration test in short mode")
	}

	// Setup
	tempDir, err := os.MkdirTemp("", "e2e-reporting-integration-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := getDefaultReportingConfig()
	config.OutputDirectory = tempDir
	config.EnableRealtimeReporting = true
	config.EnableHistoricalTracking = false

	reportingSystem, err := NewReportingSystem(config)
	if err != nil {
		t.Fatalf("Failed to create reporting system: %v", err)
	}
	defer reportingSystem.Cleanup()

	// Create E2E test runner
	e2eConfig := &E2ETestConfig{
		TestTimeout:       5 * time.Minute,
		MaxConcurrency:    3,
		RetryAttempts:     2,
		CleanupOnFailure:  true,
		DetailedLogging:   true,
		MetricsCollection: true,
		ProjectTypes: []framework.ProjectType{
			framework.ProjectTypeMultiLanguage,
		},
		Languages: []string{"go", "python", "typescript"},
		Scenarios: []E2EScenario{
			E2EScenarioBasicMCPWorkflow,
			E2EScenarioLSPMethodValidation,
			E2EScenarioMultiLanguageSupport,
		},
	}

	e2eRunner, err := NewE2ETestRunner(e2eConfig)
	if err != nil {
		t.Fatalf("Failed to create E2E test runner: %v", err)
	}
	defer e2eRunner.Cleanup()

	// Setup test environment
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := e2eRunner.SetupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup E2E test environment: %v", err)
	}

	// Start reporting session
	if err := reportingSystem.StartSession(int64(len(e2eConfig.Scenarios))); err != nil {
		t.Fatalf("Failed to start reporting session: %v", err)
	}

	// Execute E2E scenarios with reporting integration
	for _, scenario := range e2eConfig.Scenarios {
		t.Run(string(scenario), func(t *testing.T) {
			// Execute scenario
			result, err := e2eRunner.ExecuteScenario(scenario)
			if err != nil {
				t.Logf("Scenario %s failed: %v", scenario, err)
			}

			// Convert E2E result to reporting system format
			testResult := convertE2EResultToTestResult(result)

			// Record result in reporting system
			reportingSystem.RecordTestResult(testResult)

			// Record MCP metrics if available
			if e2eRunner.MockMcpClient != nil {
				reportingSystem.RecordMCPMetrics(e2eRunner.MockMcpClient)
			}

			// Record system metrics
			reportingSystem.RecordSystemMetrics()
		})
	}

	// Generate final report
	report, err := reportingSystem.GenerateReport()
	if err != nil {
		t.Fatalf("Failed to generate final report: %v", err)
	}

	// Validate comprehensive report
	if report.ExecutiveSummary == nil {
		t.Error("Executive summary should not be nil")
	}

	if report.TestMetrics.TotalTests != int64(len(e2eConfig.Scenarios)) {
		t.Errorf("Expected %d tests, got %d", len(e2eConfig.Scenarios), report.TestMetrics.TotalTests)
	}

	t.Logf("Integration test completed successfully. Report generated with %d tests", report.TestMetrics.TotalTests)
}

// TestReportingSystemPerformanceMetrics tests performance metrics collection
func TestReportingSystemPerformanceMetrics(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "e2e-reporting-perf-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := getDefaultReportingConfig()
	config.OutputDirectory = tempDir
	config.MetricsCollectionInterval = 50 * time.Millisecond

	reportingSystem, err := NewReportingSystem(config)
	if err != nil {
		t.Fatalf("Failed to create reporting system: %v", err)
	}
	defer reportingSystem.Cleanup()

	// Start session
	if err := reportingSystem.StartSession(10); err != nil {
		t.Fatalf("Failed to start session: %v", err)
	}

	// Simulate performance-intensive test scenarios
	scenarios := []string{
		"high-throughput-scenario",
		"memory-intensive-scenario",
		"cpu-intensive-scenario",
		"latency-sensitive-scenario",
	}

	for i, scenario := range scenarios {
		testResult := &TestResult{
			TestID:    generateTestID(),
			TestName:  fmt.Sprintf("Performance Test %d", i+1),
			Scenario:  scenario,
			Language:  "go",
			StartTime: time.Now().Add(-time.Duration(i+1) * time.Second),
			EndTime:   time.Now(),
			Duration:  time.Duration(i+1) * time.Second,
			Status:    TestStatusPassed,
			Success:   true,
			Errors:    []TestError{},
			Warnings:  []TestWarning{},
			Metrics: &TestResultMetrics{
				RequestCount:        int64((i + 1) * 100),
				SuccessfulRequests:  int64((i + 1) * 95),
				FailedRequests:      int64((i + 1) * 5),
				AverageLatency:      time.Duration(i+1) * 10 * time.Millisecond,
				ThroughputPerSecond: float64((i + 1) * 50),
				ErrorRate:           5.0,
				DataProcessed:       int64((i + 1) * 1024 * 1024),
				CacheHitRate:        0.85,
			},
			PerformanceData: &TestPerformanceData{
				ResponseTimes:     generateResponseTimes(100, time.Duration(i+1)*10*time.Millisecond),
				ThroughputSamples: generateThroughputSamples(50, float64((i+1)*50)),
				MemorySamples:     generateMemorySamples(50, float64((i+1)*100)),
				CPUSamples:        generateCPUSamples(50, float64((i+1)*10)),
				LatencyPercentiles: map[string]time.Duration{
					"p50":  time.Duration(i+1) * 8 * time.Millisecond,
					"p95":  time.Duration(i+1) * 15 * time.Millisecond,
					"p99":  time.Duration(i+1) * 25 * time.Millisecond,
					"p999": time.Duration(i+1) * 50 * time.Millisecond,
				},
				PerformanceScore: float64(95 - i*5),
			},
			ResourceUsage: &ResourceUsage{
				MemoryUsedMB:        float64((i + 1) * 50),
				CPUTimeMS:           int64((i + 1) * 1000),
				GoroutinesCreated:   (i + 1) * 10,
				FileDescriptorsUsed: (i + 1) * 5,
				NetworkBytesSent:    int64((i + 1) * 1024 * 100),
				NetworkBytesRecv:    int64((i + 1) * 1024 * 150),
			},
		}

		reportingSystem.RecordTestResult(testResult)
		reportingSystem.RecordSystemMetrics()

		// Small delay to simulate test execution
		time.Sleep(100 * time.Millisecond)
	}

	// Generate report
	report, err := reportingSystem.GenerateReport()
	if err != nil {
		t.Fatalf("Failed to generate report: %v", err)
	}

	// Validate performance data
	if report.PerformanceData == nil {
		t.Error("Performance data should not be nil")
	}

	if len(report.PerformanceData.ScenarioMetrics) != len(scenarios) {
		t.Errorf("Expected %d scenario metrics, got %d", len(scenarios), len(report.PerformanceData.ScenarioMetrics))
	}

	// Validate system metrics
	if report.SystemMetrics == nil {
		t.Error("System metrics should not be nil")
	}

	if report.SystemMetrics.PeakMemoryMB <= 0 {
		t.Error("Peak memory should be greater than 0")
	}

	t.Logf("Performance metrics test completed successfully")
}

// TestReportingSystemErrorAnalysis tests error analysis capabilities
func TestReportingSystemErrorAnalysis(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "e2e-reporting-errors-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := getDefaultReportingConfig()
	config.OutputDirectory = tempDir

	reportingSystem, err := NewReportingSystem(config)
	if err != nil {
		t.Fatalf("Failed to create reporting system: %v", err)
	}
	defer reportingSystem.Cleanup()

	// Start session
	if err := reportingSystem.StartSession(8); err != nil {
		t.Fatalf("Failed to start session: %v", err)
	}

	// Simulate tests with various error types
	errorScenarios := []struct {
		name     string
		errors   []TestError
		warnings []TestWarning
		success  bool
	}{
		{
			name: "Connection Error Test",
			errors: []TestError{
				{
					Type:        ErrorTypeConnection,
					Category:    ErrorCategoryInfrastructure,
					Message:     "Connection refused to localhost:8080",
					Timestamp:   time.Now(),
					Severity:    ErrorSeverityHigh,
					Recoverable: true,
				},
			},
			success: false,
		},
		{
			name: "Timeout Error Test",
			errors: []TestError{
				{
					Type:        ErrorTypeTimeout,
					Category:    ErrorCategoryEnvironment,
					Message:     "Request timeout after 5 seconds",
					Timestamp:   time.Now(),
					Severity:    ErrorSeverityMedium,
					Recoverable: true,
				},
			},
			success: false,
		},
		{
			name: "Protocol Error Test",
			errors: []TestError{
				{
					Type:        ErrorTypeProtocol,
					Category:    ErrorCategoryApplication,
					Message:     "Invalid JSON-RPC response format",
					Timestamp:   time.Now(),
					Severity:    ErrorSeverityHigh,
					Recoverable: false,
				},
			},
			success: false,
		},
		{
			name: "Assertion Error Test",
			errors: []TestError{
				{
					Type:        ErrorTypeAssertion,
					Category:    ErrorCategoryTest,
					Message:     "Expected 5 symbols, got 3",
					Timestamp:   time.Now(),
					Severity:    ErrorSeverityMedium,
					Recoverable: false,
				},
			},
			success: false,
		},
		{
			name: "Warning Test",
			warnings: []TestWarning{
				{
					Type:      WarningTypePerformance,
					Message:   "Response time exceeded expected threshold",
					Timestamp: time.Now(),
				},
				{
					Type:      WarningTypeConfiguration,
					Message:   "Using deprecated configuration option",
					Timestamp: time.Now(),
				},
			},
			success: true,
		},
		{
			name:    "Successful Test 1",
			success: true,
		},
		{
			name:    "Successful Test 2",
			success: true,
		},
		{
			name:    "Successful Test 3",
			success: true,
		},
	}

	for i, scenario := range errorScenarios {
		testResult := &TestResult{
			TestID:    generateTestID(),
			TestName:  scenario.name,
			Scenario:  "error-analysis-scenario",
			Language:  "go",
			StartTime: time.Now().Add(-time.Second),
			EndTime:   time.Now(),
			Duration:  time.Second,
			Success:   scenario.success,
			Errors:    scenario.errors,
			Warnings:  scenario.warnings,
			Metrics: &TestResultMetrics{
				RequestCount: 10,
				SuccessfulRequests: func() int64 {
					if scenario.success {
						return 10
					} else {
						return 5
					}
				}(),
				FailedRequests: func() int64 {
					if scenario.success {
						return 0
					} else {
						return 5
					}
				}(),
				AverageLatency: 100 * time.Millisecond,
				ErrorRate: func() float64 {
					if scenario.success {
						return 0.0
					} else {
						return 50.0
					}
				}(),
			},
		}

		if scenario.success {
			testResult.Status = TestStatusPassed
		} else {
			testResult.Status = TestStatusFailed
		}

		reportingSystem.RecordTestResult(testResult)

		// Delay to simulate realistic timing
		time.Sleep(10 * time.Millisecond)
	}

	// Generate report
	report, err := reportingSystem.GenerateReport()
	if err != nil {
		t.Fatalf("Failed to generate report: %v", err)
	}

	// Validate error analysis
	if report.ErrorAnalysis == nil {
		t.Error("Error analysis should not be nil")
	}

	if report.ErrorAnalysis.TotalErrors == 0 {
		t.Error("Should have recorded some errors")
	}

	if len(report.ErrorAnalysis.ErrorsByType) == 0 {
		t.Error("Should have errors categorized by type")
	}

	if len(report.ErrorAnalysis.ErrorsByCategory) == 0 {
		t.Error("Should have errors categorized by category")
	}

	if len(report.ErrorAnalysis.ErrorsBySeverity) == 0 {
		t.Error("Should have errors categorized by severity")
	}

	// Validate test metrics
	expectedTotal := int64(len(errorScenarios))
	if report.TestMetrics.TotalTests != expectedTotal {
		t.Errorf("Expected %d total tests, got %d", expectedTotal, report.TestMetrics.TotalTests)
	}

	expectedFailed := int64(4) // 4 scenarios have errors
	if report.TestMetrics.FailedTests != expectedFailed {
		t.Errorf("Expected %d failed tests, got %d", expectedFailed, report.TestMetrics.FailedTests)
	}

	expectedPassed := expectedTotal - expectedFailed
	if report.TestMetrics.PassedTests != expectedPassed {
		t.Errorf("Expected %d passed tests, got %d", expectedPassed, report.TestMetrics.PassedTests)
	}

	t.Logf("Error analysis test completed successfully. Analyzed %d errors across %d tests",
		report.ErrorAnalysis.TotalErrors, report.TestMetrics.TotalTests)
}

// Helper functions for test simulation

func runSimulatedE2ETests(t *testing.T, reportingSystem *ReportingSystem, totalTests int64) {
	mockClient := mocks.NewMockMcpClient()
	defer mockClient.Reset()

	scenarios := []string{
		"basic-mcp-workflow",
		"lsp-method-validation",
		"multi-language-support",
		"workspace-management",
		"circuit-breaker-testing",
	}

	languages := []string{"go", "python", "typescript", "java"}

	for i := int64(0); i < totalTests; i++ {
		scenario := scenarios[i%int64(len(scenarios))]
		language := languages[i%int64(len(languages))]

		// Simulate test execution with realistic timing
		startTime := time.Now()

		// Simulate some work
		time.Sleep(time.Duration(50+i*10) * time.Millisecond)

		endTime := time.Now()
		duration := endTime.Sub(startTime)

		// Simulate occasional failures
		success := i%7 != 0 // Fail every 7th test

		var errors []TestError
		var warnings []TestWarning

		if !success {
			errors = append(errors, TestError{
				Type:        ErrorTypeAssertion,
				Category:    ErrorCategoryTest,
				Message:     fmt.Sprintf("Simulated test failure for test %d", i),
				Timestamp:   time.Now(),
				Severity:    ErrorSeverityMedium,
				Recoverable: true,
			})
		}

		// Add occasional warnings
		if i%5 == 0 {
			warnings = append(warnings, TestWarning{
				Type:      WarningTypePerformance,
				Message:   "Simulated performance warning",
				Timestamp: time.Now(),
			})
		}

		// Create test result
		testResult := &TestResult{
			TestID:    generateTestID(),
			TestName:  fmt.Sprintf("E2E Test %d - %s", i+1, scenario),
			Scenario:  scenario,
			Language:  language,
			StartTime: startTime,
			EndTime:   endTime,
			Duration:  duration,
			Status: func() TestStatus {
				if success {
					return TestStatusPassed
				} else {
					return TestStatusFailed
				}
			}(),
			Success:  success,
			Errors:   errors,
			Warnings: warnings,
			Metrics: &TestResultMetrics{
				RequestCount: int64(10 + i*2),
				SuccessfulRequests: func() int64 {
					if success {
						return int64(10 + i*2)
					} else {
						return int64(5 + i)
					}
				}(),
				FailedRequests: func() int64 {
					if success {
						return 0
					} else {
						return int64(5 + i)
					}
				}(),
				AverageLatency:      time.Duration(50+i*5) * time.Millisecond,
				ThroughputPerSecond: float64(100 - i*2),
				ErrorRate: func() float64 {
					if success {
						return 0.0
					} else {
						return 10.0 + float64(i)
					}
				}(),
				DataProcessed: int64(1024 * (i + 1)),
				CacheHitRate:  0.85,
			},
			ResourceUsage: &ResourceUsage{
				MemoryUsedMB:        float64(50 + i*5),
				CPUTimeMS:           int64(100 + i*50),
				GoroutinesCreated:   int(10 + i),
				FileDescriptorsUsed: int(5 + i/2),
				NetworkBytesSent:    int64(1024 * (100 + i*10)),
				NetworkBytesRecv:    int64(1024 * (150 + i*15)),
			},
			PerformanceData: &TestPerformanceData{
				ResponseTimes:     generateResponseTimes(50, time.Duration(50+i*5)*time.Millisecond),
				ThroughputSamples: generateThroughputSamples(30, float64(100-i*2)),
				MemorySamples:     generateMemorySamples(30, float64(50+i*5)),
				CPUSamples:        generateCPUSamples(30, float64(20+i*2)),
				LatencyPercentiles: map[string]time.Duration{
					"p50":  time.Duration(40+i*3) * time.Millisecond,
					"p95":  time.Duration(80+i*5) * time.Millisecond,
					"p99":  time.Duration(120+i*8) * time.Millisecond,
					"p999": time.Duration(200+i*10) * time.Millisecond,
				},
				PerformanceScore: float64(95 - i*0.5),
			},
		}

		// Record test result
		reportingSystem.RecordTestResult(testResult)

		// Simulate MCP client activity
		simulateMCPClientActivity(mockClient, int(i)+1)
		reportingSystem.RecordMCPMetrics(mockClient)

		// Record system metrics
		reportingSystem.RecordSystemMetrics()

		// Log progress
		if (i+1)%5 == 0 {
			t.Logf("Completed %d/%d tests", i+1, totalTests)
		}
	}
}

func simulateMCPClientActivity(mockClient *mocks.MockMcpClient, requestCount int) {
	ctx := context.Background()

	methods := []string{
		mcp.LSP_METHOD_WORKSPACE_SYMBOL,
		mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION,
		mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES,
		mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER,
		mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS,
	}

	for i := 0; i < requestCount; i++ {
		method := methods[i%len(methods)]

		// Simulate some requests failing
		if i%10 == 0 {
			mockClient.QueueError(fmt.Errorf("simulated error for request %d", i))
		}

		_, _ = mockClient.SendLSPRequest(ctx, method, map[string]interface{}{
			"query": fmt.Sprintf("test-query-%d", i),
		})
	}
}

func convertE2EResultToTestResult(e2eResult *E2ETestResult) *TestResult {
	if e2eResult == nil {
		return nil
	}

	var errors []TestError
	for _, err := range e2eResult.Errors {
		errors = append(errors, TestError{
			Type:        ErrorTypeApplication,
			Category:    ErrorCategoryTest,
			Message:     err.Error(),
			Timestamp:   time.Now(),
			Severity:    ErrorSeverityMedium,
			Recoverable: true,
		})
	}

	var warnings []TestWarning
	for _, warning := range e2eResult.Warnings {
		warnings = append(warnings, TestWarning{
			Type:      WarningTypePerformance,
			Message:   warning,
			Timestamp: time.Now(),
		})
	}

	return &TestResult{
		TestID:    generateTestID(),
		TestName:  string(e2eResult.Scenario),
		Scenario:  string(e2eResult.Scenario),
		Language:  "multi", // E2E tests typically cover multiple languages
		StartTime: e2eResult.StartTime,
		EndTime:   e2eResult.EndTime,
		Duration:  e2eResult.Duration,
		Status: func() TestStatus {
			if e2eResult.Success {
				return TestStatusPassed
			} else {
				return TestStatusFailed
			}
		}(),
		Success:  e2eResult.Success,
		Errors:   errors,
		Warnings: warnings,
		Metrics: &TestResultMetrics{
			RequestCount:        e2eResult.TotalRequests,
			SuccessfulRequests:  e2eResult.SuccessfulReqs,
			FailedRequests:      e2eResult.FailedRequests,
			AverageLatency:      e2eResult.AverageLatency,
			ThroughputPerSecond: 0.0, // Calculate if needed
			ErrorRate:           float64(e2eResult.FailedRequests) / float64(e2eResult.TotalRequests) * 100,
		},
		ResourceUsage: &ResourceUsage{
			MemoryUsedMB:      e2eResult.MemoryUsageMB,
			CPUTimeMS:         int64(e2eResult.CPUUsagePercent * 1000), // Rough approximation
			GoroutinesCreated: e2eResult.GoroutineCount,
		},
	}
}

func validateReportStructure(t *testing.T, report *ComprehensiveReport) {
	if report == nil {
		t.Fatal("Report should not be nil")
	}

	if report.GeneratedAt.IsZero() {
		t.Error("Report generation time should be set")
	}

	if report.ReportID == "" {
		t.Error("Report ID should not be empty")
	}

	if report.ExecutiveSummary == nil {
		t.Error("Executive summary should not be nil")
	}

	if report.TestMetrics == nil {
		t.Error("Test metrics should not be nil")
	}

	if report.SystemMetrics == nil {
		t.Error("System metrics should not be nil")
	}

	if report.MCPMetrics == nil {
		t.Error("MCP metrics should not be nil")
	}

	if report.Configuration == nil {
		t.Error("Configuration should not be nil")
	}

	// Validate executive summary
	if report.ExecutiveSummary.TotalTests <= 0 {
		t.Error("Total tests should be greater than 0")
	}

	if report.ExecutiveSummary.PerformanceScore < 0 || report.ExecutiveSummary.PerformanceScore > 100 {
		t.Error("Performance score should be between 0 and 100")
	}

	// Validate test metrics
	if report.TestMetrics.TotalTests != report.TestMetrics.PassedTests+report.TestMetrics.FailedTests+report.TestMetrics.SkippedTests {
		t.Error("Total tests should equal sum of passed, failed, and skipped tests")
	}

	if report.TestMetrics.TotalDuration <= 0 {
		t.Error("Total duration should be greater than 0")
	}
}

func verifyReportFiles(t *testing.T, outputDir string) {
	expectedFiles := []string{
		"*.txt",  // Console format
		"*.json", // JSON format
		"*.html", // HTML format
		"*.csv",  // CSV format
	}

	for _, pattern := range expectedFiles {
		matches, err := filepath.Glob(filepath.Join(outputDir, pattern))
		if err != nil {
			t.Errorf("Failed to check for %s files: %v", pattern, err)
			continue
		}

		if len(matches) == 0 {
			t.Errorf("No %s files found in output directory", pattern)
		} else {
			t.Logf("Found %d %s files", len(matches), pattern)
		}
	}
}

// Helper functions for generating test data

func generateTestID() string {
	return fmt.Sprintf("test-%d", time.Now().UnixNano())
}

func generateResponseTimes(count int, baseLatency time.Duration) []time.Duration {
	times := make([]time.Duration, count)
	for i := 0; i < count; i++ {
		variation := time.Duration(i%20-10) * time.Millisecond // ±10ms variation
		times[i] = baseLatency + variation
		if times[i] < 0 {
			times[i] = baseLatency / 2
		}
	}
	return times
}

func generateThroughputSamples(count int, baseThroughput float64) []float64 {
	samples := make([]float64, count)
	for i := 0; i < count; i++ {
		variation := float64(i%20-10) * 2.0 // ±20 req/sec variation
		samples[i] = baseThroughput + variation
		if samples[i] < 0 {
			samples[i] = baseThroughput / 2
		}
	}
	return samples
}

func generateMemorySamples(count int, baseMemory float64) []float64 {
	samples := make([]float64, count)
	for i := 0; i < count; i++ {
		variation := float64(i%10-5) * 5.0 // ±25MB variation
		samples[i] = baseMemory + variation
		if samples[i] < 0 {
			samples[i] = baseMemory / 2
		}
	}
	return samples
}

func generateCPUSamples(count int, baseCPU float64) []float64 {
	samples := make([]float64, count)
	for i := 0; i < count; i++ {
		variation := float64(i%10-5) * 2.0 // ±10% variation
		samples[i] = baseCPU + variation
		if samples[i] < 0 {
			samples[i] = baseCPU / 2
		}
		if samples[i] > 100 {
			samples[i] = 100
		}
	}
	return samples
}
