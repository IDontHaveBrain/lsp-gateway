package e2e_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestResultPersistenceManager tests the result persistence functionality
func TestResultPersistenceManager(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "e2e_persistence_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test configuration
	config := &ResultPersistenceConfig{
		DatabasePath:         filepath.Join(tempDir, "test.db"),
		JSONResultsDirectory: filepath.Join(tempDir, "json"),
		ArchiveDirectory:     filepath.Join(tempDir, "archive"),
		RetentionDays:        30,
		CompressionEnabled:   true,
		MaxResultsInMemory:   100,
		BackupInterval:       time.Hour,
	}

	// Create persistence manager
	rpm, err := NewResultPersistenceManager(config)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}
	defer rpm.Close()

	t.Run("SaveAndRetrieveTestRun", func(t *testing.T) {
		testSaveAndRetrieveTestRun(t, rpm)
	})

	t.Run("HistoricalTrends", func(t *testing.T) {
		testHistoricalTrends(t, rpm)
	})

	t.Run("RegressionDetection", func(t *testing.T) {
		testRegressionDetection(t, rpm)
	})

	t.Run("DateRangeQuery", func(t *testing.T) {
		testDateRangeQuery(t, rpm)
	})

	t.Run("BaselineManagement", func(t *testing.T) {
		testBaselineManagement(t, rpm)
	})

	t.Run("ComprehensiveReport", func(t *testing.T) {
		testComprehensiveReport(t, rpm)
	})
}

func testSaveAndRetrieveTestRun(t *testing.T, rpm *ResultPersistenceManager) {
	// Create test run record
	record := &TestRunRecord{
		ID:            "test_run_001",
		Timestamp:     time.Now(),
		SuiteVersion:  "1.0.0",
		ExecutionMode: "full",
		TotalTests:    10,
		PassedTests:   8,
		FailedTests:   2,
		SkippedTests:  0,
		TotalDuration: 5 * time.Minute,
		E2EScore:      85.5,
		SystemInfo: &SystemInfo{
			OS:           "linux",
			Architecture: "amd64",
			NumCPU:       4,
			GoVersion:    "go1.21",
		},
		TestResults: []*TestResult{
			{
				TestName:  "TestHTTPProtocol_BasicStartup",
				Category:  "http_protocol",
				Status:    "passed",
				Duration:  30 * time.Second,
				Timestamp: time.Now(),
			},
			{
				TestName:     "TestMCPProtocol_ToolAvailability",
				Category:     "mcp_protocol",
				Status:       "failed",
				Duration:     15 * time.Second,
				ErrorMessage: "timeout waiting for tool response",
				Timestamp:    time.Now(),
			},
		},
		MCPMetrics: &MCPClientMetrics{
			TotalRequests:  100,
			SuccessfulReqs: 85,
			FailedRequests: 15,
			AverageLatency: 150 * time.Millisecond,
			TimeoutCount:   3,
		},
		PerformanceData: &PerformanceData{
			ThroughputReqPerSec:     120.5,
			AverageResponseTime:     100 * time.Millisecond,
			P95ResponseTime:         250 * time.Millisecond,
			P99ResponseTime:         400 * time.Millisecond,
			PeakMemoryUsage:         512 * 1024 * 1024, // 512MB
			ResourceEfficiencyScore: 78.5,
		},
		RegressionStatus: "no_regression",
		Tags:             []string{"automated", "full_suite"},
		Environment: map[string]string{
			"CI":     "true",
			"BRANCH": "main",
			"COMMIT": "abc123",
		},
	}

	// Save test run
	if err := rpm.SaveTestRun(record); err != nil {
		t.Fatalf("Failed to save test run: %v", err)
	}

	// Retrieve test run
	retrieved, err := rpm.GetTestRun("test_run_001")
	if err != nil {
		t.Fatalf("Failed to retrieve test run: %v", err)
	}

	// Validate retrieved data
	if retrieved.ID != record.ID {
		t.Errorf("Expected ID %s, got %s", record.ID, retrieved.ID)
	}
	if retrieved.TotalTests != record.TotalTests {
		t.Errorf("Expected %d total tests, got %d", record.TotalTests, retrieved.TotalTests)
	}
	if retrieved.E2EScore != record.E2EScore {
		t.Errorf("Expected E2E score %.2f, got %.2f", record.E2EScore, retrieved.E2EScore)
	}
	if len(retrieved.TestResults) != len(record.TestResults) {
		t.Errorf("Expected %d test results, got %d", len(record.TestResults), len(retrieved.TestResults))
	}

	t.Logf("Successfully saved and retrieved test run: %s", retrieved.ID)
}

func testHistoricalTrends(t *testing.T, rpm *ResultPersistenceManager) {
	// Create multiple test runs over time to establish trends
	baseTime := time.Now().AddDate(0, 0, -30)

	testRuns := []*TestRunRecord{
		{
			ID:            "trend_run_001",
			Timestamp:     baseTime,
			SuiteVersion:  "1.0.0",
			ExecutionMode: "full",
			TotalTests:    10,
			PassedTests:   8,
			FailedTests:   2,
			TotalDuration: 5 * time.Minute,
			E2EScore:      80.0,
		},
		{
			ID:            "trend_run_002",
			Timestamp:     baseTime.AddDate(0, 0, 10),
			SuiteVersion:  "1.1.0",
			ExecutionMode: "full",
			TotalTests:    12,
			PassedTests:   10,
			FailedTests:   2,
			TotalDuration: 4*time.Minute + 30*time.Second,
			E2EScore:      85.0,
		},
		{
			ID:            "trend_run_003",
			Timestamp:     baseTime.AddDate(0, 0, 20),
			SuiteVersion:  "1.2.0",
			ExecutionMode: "full",
			TotalTests:    15,
			PassedTests:   13,
			FailedTests:   2,
			TotalDuration: 4 * time.Minute,
			E2EScore:      88.5,
		},
	}

	// Save all test runs
	for _, run := range testRuns {
		if err := rpm.SaveTestRun(run); err != nil {
			t.Fatalf("Failed to save trend test run %s: %v", run.ID, err)
		}
	}

	// Analyze trends
	trends, err := rpm.AnalyzeHistoricalTrends(35)
	if err != nil {
		t.Fatalf("Failed to analyze historical trends: %v", err)
	}

	// Validate trends
	if len(trends) == 0 {
		t.Error("Expected historical trends, got none")
	}

	for _, trend := range trends {
		t.Logf("Trend for %s: %s (slope: %.2f)", trend.Metric, trend.Direction, trend.TrendSlope)

		if len(trend.DataPoints) == 0 {
			t.Errorf("Expected data points for trend %s", trend.Metric)
		}

		// E2E score trend should be improving based on our test data
		if trend.Metric == "e2e_score" && trend.Direction != "improving" {
			t.Logf("Warning: E2E score trend is %s, expected improving", trend.Direction)
		}
	}
}

func testRegressionDetection(t *testing.T, rpm *ResultPersistenceManager) {
	// Create baseline run
	baseline := &TestRunRecord{
		ID:            "baseline_run",
		Timestamp:     time.Now().AddDate(0, 0, -7),
		SuiteVersion:  "1.0.0",
		ExecutionMode: "full",
		TotalTests:    10,
		PassedTests:   9,
		FailedTests:   1,
		TotalDuration: 5 * time.Minute,
		E2EScore:      90.0,
	}

	// Create current run with regression
	current := &TestRunRecord{
		ID:            "current_run_regression",
		Timestamp:     time.Now(),
		SuiteVersion:  "1.1.0",
		ExecutionMode: "full",
		TotalTests:    10,
		PassedTests:   7,               // Worse than baseline
		FailedTests:   3,               // Worse than baseline
		TotalDuration: 7 * time.Minute, // Slower than baseline
		E2EScore:      75.0,            // Lower than baseline (>10% regression)
	}

	// Save both runs
	if err := rpm.SaveTestRun(baseline); err != nil {
		t.Fatalf("Failed to save baseline run: %v", err)
	}
	if err := rpm.SaveTestRun(current); err != nil {
		t.Fatalf("Failed to save current run: %v", err)
	}

	// Detect regressions
	analysis, err := rpm.DetectRegressions(current, baseline.ID)
	if err != nil {
		t.Fatalf("Failed to detect regressions: %v", err)
	}

	// Validate regression detection
	if !analysis.RegressionsDetected {
		t.Error("Expected regressions to be detected")
	}

	if len(analysis.RegressionDetails) == 0 {
		t.Error("Expected regression details")
	}

	if analysis.BaselineComparison.OverallStatus != "degraded" {
		t.Errorf("Expected overall status 'degraded', got '%s'", analysis.BaselineComparison.OverallStatus)
	}

	t.Logf("Detected %d regressions with overall status: %s",
		len(analysis.RegressionDetails), analysis.BaselineComparison.OverallStatus)

	for _, regression := range analysis.RegressionDetails {
		t.Logf("Regression in %s: %.2f%% change (%s)",
			regression.Metric, regression.PercentChange, regression.Severity)
	}
}

func testDateRangeQuery(t *testing.T, rpm *ResultPersistenceManager) {
	// Create test runs across different dates
	now := time.Now()
	testRuns := []*TestRunRecord{
		{
			ID:        "date_run_001",
			Timestamp: now.AddDate(0, 0, -5),
			E2EScore:  80.0,
		},
		{
			ID:        "date_run_002",
			Timestamp: now.AddDate(0, 0, -3),
			E2EScore:  85.0,
		},
		{
			ID:        "date_run_003",
			Timestamp: now.AddDate(0, 0, -1),
			E2EScore:  90.0,
		},
	}

	// Save test runs
	for _, run := range testRuns {
		run.SuiteVersion = "1.0.0"
		run.ExecutionMode = "quick"
		run.TotalTests = 5
		run.PassedTests = 5
		run.TotalDuration = 2 * time.Minute

		if err := rpm.SaveTestRun(run); err != nil {
			t.Fatalf("Failed to save date range test run %s: %v", run.ID, err)
		}
	}

	// Query date range
	start := now.AddDate(0, 0, -4)
	end := now

	runs, err := rpm.GetTestRunsInDateRange(start, end)
	if err != nil {
		t.Fatalf("Failed to get test runs in date range: %v", err)
	}

	// Should get runs from the last 4 days (2 runs)
	expectedRuns := 2
	if len(runs) != expectedRuns {
		t.Errorf("Expected %d runs in date range, got %d", expectedRuns, len(runs))
	}

	t.Logf("Retrieved %d runs in date range", len(runs))
}

func testBaselineManagement(t *testing.T, rpm *ResultPersistenceManager) {
	// Create test run to set as baseline
	baseline := &TestRunRecord{
		ID:            "new_baseline",
		Timestamp:     time.Now(),
		SuiteVersion:  "1.0.0",
		ExecutionMode: "full",
		TotalTests:    10,
		PassedTests:   10,
		FailedTests:   0,
		TotalDuration: 4 * time.Minute,
		E2EScore:      95.0,
	}

	// Save test run
	if err := rpm.SaveTestRun(baseline); err != nil {
		t.Fatalf("Failed to save baseline test run: %v", err)
	}

	// Set as baseline
	if err := rpm.UpdateBaseline(baseline.ID); err != nil {
		t.Fatalf("Failed to update baseline: %v", err)
	}

	// Retrieve to verify baseline was set
	retrieved, err := rpm.GetTestRun(baseline.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve baseline run: %v", err)
	}

	if retrieved.BaselineID != baseline.ID {
		t.Errorf("Expected baseline ID %s, got %s", baseline.ID, retrieved.BaselineID)
	}

	t.Logf("Successfully set baseline: %s", baseline.ID)
}

func testComprehensiveReport(t *testing.T, rpm *ResultPersistenceManager) {
	// Generate comprehensive report
	report, err := rpm.GenerateComprehensiveReport(30)
	if err != nil {
		t.Fatalf("Failed to generate comprehensive report: %v", err)
	}

	// Validate report structure
	if summary, ok := report["summary"].(map[string]interface{}); ok {
		if totalRuns, ok := summary["total_runs"].(int); !ok || totalRuns == 0 {
			t.Error("Expected total_runs in summary")
		}
		if avgScore, ok := summary["average_e2e_score"].(float64); !ok || avgScore == 0 {
			t.Error("Expected average_e2e_score in summary")
		}
	} else {
		t.Error("Expected summary section in report")
	}

	if recentRuns, ok := report["recent_runs"].([]*TestRunRecord); ok {
		t.Logf("Report includes %d recent runs", len(recentRuns))
	} else {
		t.Error("Expected recent_runs section in report")
	}

	if trends, ok := report["trends"].([]HistoricalTrend); ok {
		t.Logf("Report includes %d trend analyses", len(trends))
	} else {
		t.Error("Expected trends section in report")
	}

	t.Log("Comprehensive report generated successfully")
}

// TestResultPersistenceIntegration tests integration with E2E test suite
func TestResultPersistenceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary persistence manager
	tempDir, err := os.MkdirTemp("", "e2e_persistence_integration_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultResultPersistenceConfig()
	config.DatabasePath = filepath.Join(tempDir, "integration.db")
	config.JSONResultsDirectory = filepath.Join(tempDir, "json")
	config.ArchiveDirectory = filepath.Join(tempDir, "archive")

	rpm, err := NewResultPersistenceManager(config)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}
	defer rpm.Close()

	// Simulate running E2E test suite and persisting results
	testRecord := &TestRunRecord{
		ID:            "integration_test_001",
		Timestamp:     time.Now(),
		SuiteVersion:  "1.0.0",
		ExecutionMode: "integration_test",
		TotalTests:    25, // Realistic number for E2E suite
		PassedTests:   23,
		FailedTests:   2,
		SkippedTests:  0,
		TotalDuration: 15 * time.Minute, // Realistic E2E execution time
		E2EScore:      88.5,
		SystemInfo: &SystemInfo{
			OS:           "linux",
			Architecture: "amd64",
			NumCPU:       8,
			GoVersion:    "go1.21",
		},
		MCPMetrics: &MCPClientMetrics{
			TotalRequests:     500,
			SuccessfulReqs:    475,
			FailedRequests:    25,
			AverageLatency:    125 * time.Millisecond,
			TimeoutCount:      5,
			CircuitBreakerOps: 3,
			RetryOperations:   12,
		},
		PerformanceData: &PerformanceData{
			ThroughputReqPerSec:     150.0,
			AverageResponseTime:     85 * time.Millisecond,
			P95ResponseTime:         200 * time.Millisecond,
			P99ResponseTime:         350 * time.Millisecond,
			PeakMemoryUsage:         768 * 1024 * 1024, // 768MB
			ResourceEfficiencyScore: 82.5,
		},
		TestResults: []*TestResult{
			{
				TestName:  "TestE2EFullSuite_HTTPProtocol",
				Category:  "http_protocol",
				Status:    "passed",
				Duration:  3 * time.Minute,
				Timestamp: time.Now(),
				Resources: &ResourceUsage{
					MemoryMB:     256,
					CPUPercent:   45.5,
					Goroutines:   120,
					FilesOpened:  15,
					NetworkConns: 8,
				},
			},
			{
				TestName:  "TestE2EFullSuite_MCPProtocol",
				Category:  "mcp_protocol",
				Status:    "passed",
				Duration:  4 * time.Minute,
				Timestamp: time.Now(),
				Resources: &ResourceUsage{
					MemoryMB:     312,
					CPUPercent:   52.0,
					Goroutines:   145,
					FilesOpened:  18,
					NetworkConns: 12,
				},
			},
			{
				TestName:     "TestE2EFullSuite_MultiLanguageWorkflow",
				Category:     "workflow",
				Status:       "failed",
				Duration:     2 * time.Minute,
				ErrorMessage: "timeout in TypeScript language server initialization",
				Timestamp:    time.Now(),
				Resources: &ResourceUsage{
					MemoryMB:     180,
					CPUPercent:   38.0,
					Goroutines:   95,
					FilesOpened:  12,
					NetworkConns: 6,
				},
			},
		},
		RegressionStatus: "no_regression",
		Tags:             []string{"integration", "full_suite", "automated", "ci"},
		Environment: map[string]string{
			"CI":           "true",
			"BRANCH":       "main",
			"COMMIT":       "abc123def456",
			"BUILD_NUMBER": "1234",
			"RUNNER":       "github-actions",
		},
		Errors: []string{
			"timeout in TypeScript language server initialization",
		},
		Warnings: []string{
			"high memory usage detected in MCP protocol tests",
			"circuit breaker activated 3 times during execution",
		},
	}

	// Save the test record
	if err := rpm.SaveTestRun(testRecord); err != nil {
		t.Fatalf("Failed to save integration test record: %v", err)
	}

	// Verify persistence worked
	retrieved, err := rpm.GetTestRun(testRecord.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve integration test record: %v", err)
	}

	// Validate comprehensive data preservation
	if retrieved.ID != testRecord.ID {
		t.Errorf("ID mismatch: expected %s, got %s", testRecord.ID, retrieved.ID)
	}

	if len(retrieved.TestResults) != len(testRecord.TestResults) {
		t.Errorf("Test results count mismatch: expected %d, got %d",
			len(testRecord.TestResults), len(retrieved.TestResults))
	}

	t.Logf("Integration test record successfully persisted and retrieved")
	t.Logf("Test Summary: %d total, %d passed, %d failed, E2E Score: %.1f",
		retrieved.TotalTests, retrieved.PassedTests, retrieved.FailedTests, retrieved.E2EScore)
}

// BenchmarkResultPersistence benchmarks the persistence operations
func BenchmarkResultPersistence(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "e2e_persistence_bench_*")
	if err != nil {
		b.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultResultPersistenceConfig()
	config.DatabasePath = filepath.Join(tempDir, "bench.db")
	config.JSONResultsDirectory = filepath.Join(tempDir, "json")

	rpm, err := NewResultPersistenceManager(config)
	if err != nil {
		b.Fatalf("Failed to create persistence manager: %v", err)
	}
	defer rpm.Close()

	// Create sample test record
	record := &TestRunRecord{
		SuiteVersion:  "1.0.0",
		ExecutionMode: "benchmark",
		TotalTests:    10,
		PassedTests:   8,
		FailedTests:   2,
		TotalDuration: 5 * time.Minute,
		E2EScore:      85.0,
		Timestamp:     time.Now(),
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		record.ID = fmt.Sprintf("bench_run_%d", i)
		record.Timestamp = time.Now()

		if err := rpm.SaveTestRun(record); err != nil {
			b.Fatalf("Failed to save test run: %v", err)
		}
	}
}
