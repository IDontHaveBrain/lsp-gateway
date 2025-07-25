package e2e_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ResultPersistenceConfig configures the result persistence system
type ResultPersistenceConfig struct {
	DatabasePath         string        `json:"database_path"`
	JSONResultsDirectory string        `json:"json_results_directory"`
	ArchiveDirectory     string        `json:"archive_directory"`
	RetentionDays        int           `json:"retention_days"`
	CompressionEnabled   bool          `json:"compression_enabled"`
	MaxResultsInMemory   int           `json:"max_results_in_memory"`
	BackupInterval       time.Duration `json:"backup_interval"`
}

// DefaultResultPersistenceConfig returns default configuration
func DefaultResultPersistenceConfig() *ResultPersistenceConfig {
	return &ResultPersistenceConfig{
		DatabasePath:         "./tests/e2e/results/e2e_results.db",
		JSONResultsDirectory: "./tests/e2e/results/json",
		ArchiveDirectory:     "./tests/e2e/results/archive",
		RetentionDays:        90,
		CompressionEnabled:   true,
		MaxResultsInMemory:   1000,
		BackupInterval:       24 * time.Hour,
	}
}

// TestRunRecord represents a complete test execution record
type TestRunRecord struct {
	ID               string            `json:"id" db:"id"`
	Timestamp        time.Time         `json:"timestamp" db:"timestamp"`
	SuiteVersion     string            `json:"suite_version" db:"suite_version"`
	ExecutionMode    string            `json:"execution_mode" db:"execution_mode"`
	TotalTests       int               `json:"total_tests" db:"total_tests"`
	PassedTests      int               `json:"passed_tests" db:"passed_tests"`
	FailedTests      int               `json:"failed_tests" db:"failed_tests"`
	SkippedTests     int               `json:"skipped_tests" db:"skipped_tests"`
	TotalDuration    time.Duration     `json:"total_duration" db:"total_duration"`
	E2EScore         float64           `json:"e2e_score" db:"e2e_score"`
	SystemInfo       *SystemInfo       `json:"system_info"`
	TestResults      []*TestResult     `json:"test_results"`
	MCPMetrics       *MCPClientMetrics `json:"mcp_metrics"`
	PerformanceData  *PerformanceData  `json:"performance_data"`
	Errors           []string          `json:"errors"`
	Warnings         []string          `json:"warnings"`
	RegressionStatus string            `json:"regression_status" db:"regression_status"`
	BaselineID       string            `json:"baseline_id,omitempty" db:"baseline_id"`
	Tags             []string          `json:"tags"`
	Environment      map[string]string `json:"environment"`
}

// TestResult represents individual test execution result
type TestResult struct {
	TestName     string                 `json:"test_name"`
	Category     string                 `json:"category"`
	Status       string                 `json:"status"`
	Duration     time.Duration          `json:"duration"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Metrics      map[string]interface{} `json:"metrics"`
	Resources    *ResourceUsage         `json:"resources"`
	Timestamp    time.Time              `json:"timestamp"`
}

// ResourceUsage tracks resource consumption during test execution
type ResourceUsage struct {
	MemoryMB     int64   `json:"memory_mb"`
	CPUPercent   float64 `json:"cpu_percent"`
	Goroutines   int     `json:"goroutines"`
	FilesOpened  int     `json:"files_opened"`
	NetworkConns int     `json:"network_connections"`
}

// MCPClientMetrics aggregates MockMcpClient metrics
type MCPClientMetrics struct {
	TotalRequests     int64         `json:"total_requests"`
	SuccessfulReqs    int64         `json:"successful_requests"`
	FailedRequests    int64         `json:"failed_requests"`
	AverageLatency    time.Duration `json:"average_latency"`
	TimeoutCount      int64         `json:"timeout_count"`
	CircuitBreakerOps int64         `json:"circuit_breaker_operations"`
	RetryOperations   int64         `json:"retry_operations"`
}

// PerformanceData aggregates performance metrics across tests
type PerformanceData struct {
	ThroughputReqPerSec     float64       `json:"throughput_req_per_sec"`
	AverageResponseTime     time.Duration `json:"average_response_time"`
	P95ResponseTime         time.Duration `json:"p95_response_time"`
	P99ResponseTime         time.Duration `json:"p99_response_time"`
	PeakMemoryUsage         int64         `json:"peak_memory_usage"`
	ResourceEfficiencyScore float64       `json:"resource_efficiency_score"`
}

// HistoricalTrend represents performance trends over time
type HistoricalTrend struct {
	Metric     string           `json:"metric"`
	DataPoints []TrendDataPoint `json:"data_points"`
	Direction  string           `json:"direction"` // "improving", "degrading", "stable"
	TrendSlope float64          `json:"trend_slope"`
}

// TrendDataPoint represents a single data point in a trend
type TrendDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	RunID     string    `json:"run_id"`
}

// RegressionAnalysis represents regression analysis results
type RegressionAnalysis struct {
	RegressionsDetected bool                `json:"regressions_detected"`
	RegressionDetails   []RegressionDetail  `json:"regression_details"`
	BaselineComparison  *BaselineComparison `json:"baseline_comparison"`
	TrendAnalysis       []HistoricalTrend   `json:"trend_analysis"`
	RecommendedActions  []string            `json:"recommended_actions"`
}

// RegressionDetail represents a specific regression
type RegressionDetail struct {
	Metric        string    `json:"metric"`
	CurrentValue  float64   `json:"current_value"`
	BaselineValue float64   `json:"baseline_value"`
	PercentChange float64   `json:"percent_change"`
	Severity      string    `json:"severity"`
	FirstDetected time.Time `json:"first_detected"`
}

// BaselineComparison compares current run with baseline
type BaselineComparison struct {
	BaselineID      string             `json:"baseline_id"`
	BaselineDate    time.Time          `json:"baseline_date"`
	ScoreComparison float64            `json:"score_comparison"`
	MetricChanges   map[string]float64 `json:"metric_changes"`
	OverallStatus   string             `json:"overall_status"`
}

// ResultPersistenceManager manages test result persistence and historical tracking
type ResultPersistenceManager struct {
	config *ResultPersistenceConfig
	db     *sql.DB
	ctx    context.Context
	cancel context.CancelFunc
}

// NewResultPersistenceManager creates a new persistence manager
func NewResultPersistenceManager(config *ResultPersistenceConfig) (*ResultPersistenceManager, error) {
	if config == nil {
		config = DefaultResultPersistenceConfig()
	}

	// Create necessary directories
	dirs := []string{
		filepath.Dir(config.DatabasePath),
		config.JSONResultsDirectory,
		config.ArchiveDirectory,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Open database connection
	db, err := sql.Open("sqlite3", config.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	manager := &ResultPersistenceManager{
		config: config,
		db:     db,
		ctx:    ctx,
		cancel: cancel,
	}

	// Initialize database schema
	if err := manager.initializeDatabase(); err != nil {
		manager.Close()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Start background maintenance
	go manager.backgroundMaintenance()

	return manager, nil
}

// SaveTestRun saves a complete test run record
func (rpm *ResultPersistenceManager) SaveTestRun(record *TestRunRecord) error {
	// Generate ID if not provided
	if record.ID == "" {
		record.ID = fmt.Sprintf("run_%s", record.Timestamp.Format("20060102_150405"))
	}

	// Save to database
	if err := rpm.saveToDatabase(record); err != nil {
		return fmt.Errorf("failed to save to database: %w", err)
	}

	// Save JSON file
	if err := rpm.saveToJSON(record); err != nil {
		return fmt.Errorf("failed to save JSON file: %w", err)
	}

	// Archive if compression enabled
	if rpm.config.CompressionEnabled {
		if err := rpm.archiveResult(record); err != nil {
			// Log but don't fail on archive error
			fmt.Printf("Warning: failed to archive result: %v\n", err)
		}
	}

	return nil
}

// GetTestRun retrieves a specific test run by ID
func (rpm *ResultPersistenceManager) GetTestRun(runID string) (*TestRunRecord, error) {
	// Try database first
	record, err := rpm.getFromDatabase(runID)
	if err == nil {
		return record, nil
	}

	// Try JSON file
	record, err = rpm.getFromJSON(runID)
	if err == nil {
		return record, nil
	}

	return nil, fmt.Errorf("test run %s not found", runID)
}

// GetRecentTestRuns retrieves recent test runs
func (rpm *ResultPersistenceManager) GetRecentTestRuns(limit int) ([]*TestRunRecord, error) {
	query := `
		SELECT id, timestamp, suite_version, execution_mode, 
		       total_tests, passed_tests, failed_tests, skipped_tests,
		       total_duration, e2e_score, regression_status, baseline_id
		FROM test_runs 
		ORDER BY timestamp DESC 
		LIMIT ?`

	rows, err := rpm.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent test runs: %w", err)
	}
	defer rows.Close()

	var records []*TestRunRecord
	for rows.Next() {
		record := &TestRunRecord{}
		var totalDurationNs int64

		err := rows.Scan(
			&record.ID, &record.Timestamp, &record.SuiteVersion,
			&record.ExecutionMode, &record.TotalTests, &record.PassedTests,
			&record.FailedTests, &record.SkippedTests, &totalDurationNs,
			&record.E2EScore, &record.RegressionStatus, &record.BaselineID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan test run: %w", err)
		}

		record.TotalDuration = time.Duration(totalDurationNs)
		records = append(records, record)
	}

	return records, nil
}

// AnalyzeHistoricalTrends analyzes performance trends over time
func (rpm *ResultPersistenceManager) AnalyzeHistoricalTrends(days int) ([]HistoricalTrend, error) {
	since := time.Now().AddDate(0, 0, -days)

	query := `
		SELECT id, timestamp, e2e_score, total_duration, passed_tests, failed_tests
		FROM test_runs 
		WHERE timestamp >= ? 
		ORDER BY timestamp ASC`

	rows, err := rpm.db.Query(query, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query historical data: %w", err)
	}
	defer rows.Close()

	var dataPoints []struct {
		ID        string
		Timestamp time.Time
		Score     float64
		Duration  time.Duration
		Passed    int
		Failed    int
	}

	for rows.Next() {
		var dp struct {
			ID        string
			Timestamp time.Time
			Score     float64
			Duration  time.Duration
			Passed    int
			Failed    int
		}
		var durationNs int64

		err := rows.Scan(&dp.ID, &dp.Timestamp, &dp.Score, &durationNs, &dp.Passed, &dp.Failed)
		if err != nil {
			return nil, fmt.Errorf("failed to scan historical data: %w", err)
		}

		dp.Duration = time.Duration(durationNs)
		dataPoints = append(dataPoints, dp)
	}

	// Analyze trends for different metrics
	trends := []HistoricalTrend{
		rpm.analyzeTrend("e2e_score", dataPoints, func(dp interface{}) float64 {
			return dp.(struct {
				ID        string
				Timestamp time.Time
				Score     float64
				Duration  time.Duration
				Passed    int
				Failed    int
			}).Score
		}),
		rpm.analyzeTrend("execution_duration", dataPoints, func(dp interface{}) float64 {
			return float64(dp.(struct {
				ID        string
				Timestamp time.Time
				Score     float64
				Duration  time.Duration
				Passed    int
				Failed    int
			}).Duration.Seconds())
		}),
		rpm.analyzeTrend("success_rate", dataPoints, func(dp interface{}) float64 {
			d := dp.(struct {
				ID        string
				Timestamp time.Time
				Score     float64
				Duration  time.Duration
				Passed    int
				Failed    int
			})
			total := d.Passed + d.Failed
			if total == 0 {
				return 0
			}
			return float64(d.Passed) / float64(total) * 100
		}),
	}

	return trends, nil
}

// DetectRegressions detects performance regressions compared to baseline
func (rpm *ResultPersistenceManager) DetectRegressions(currentRun *TestRunRecord, baselineID string) (*RegressionAnalysis, error) {
	// Get baseline run
	baseline, err := rpm.GetTestRun(baselineID)
	if err != nil {
		return nil, fmt.Errorf("failed to get baseline run: %w", err)
	}

	analysis := &RegressionAnalysis{
		BaselineComparison: &BaselineComparison{
			BaselineID:    baselineID,
			BaselineDate:  baseline.Timestamp,
			MetricChanges: make(map[string]float64),
		},
		RegressionDetails:  []RegressionDetail{},
		RecommendedActions: []string{},
	}

	// Compare E2E scores
	scoreChange := (currentRun.E2EScore - baseline.E2EScore) / baseline.E2EScore * 100
	analysis.BaselineComparison.ScoreComparison = scoreChange
	analysis.BaselineComparison.MetricChanges["e2e_score"] = scoreChange

	// Check for significant regressions (>10% degradation)
	if scoreChange < -10 {
		analysis.RegressionsDetected = true
		analysis.RegressionDetails = append(analysis.RegressionDetails, RegressionDetail{
			Metric:        "e2e_score",
			CurrentValue:  currentRun.E2EScore,
			BaselineValue: baseline.E2EScore,
			PercentChange: scoreChange,
			Severity:      rpm.calculateSeverity(scoreChange),
			FirstDetected: currentRun.Timestamp,
		})
		analysis.RecommendedActions = append(analysis.RecommendedActions, "Investigate E2E score regression")
	}

	// Compare execution duration
	durationChange := (float64(currentRun.TotalDuration) - float64(baseline.TotalDuration)) / float64(baseline.TotalDuration) * 100
	analysis.BaselineComparison.MetricChanges["total_duration"] = durationChange

	if durationChange > 20 {
		analysis.RegressionsDetected = true
		analysis.RegressionDetails = append(analysis.RegressionDetails, RegressionDetail{
			Metric:        "total_duration",
			CurrentValue:  float64(currentRun.TotalDuration.Seconds()),
			BaselineValue: float64(baseline.TotalDuration.Seconds()),
			PercentChange: durationChange,
			Severity:      rpm.calculateSeverity(durationChange),
			FirstDetected: currentRun.Timestamp,
		})
		analysis.RecommendedActions = append(analysis.RecommendedActions, "Investigate execution time regression")
	}

	// Determine overall status
	if analysis.RegressionsDetected {
		analysis.BaselineComparison.OverallStatus = "degraded"
	} else if scoreChange > 5 {
		analysis.BaselineComparison.OverallStatus = "improved"
	} else {
		analysis.BaselineComparison.OverallStatus = "stable"
	}

	// Get historical trends
	trends, err := rpm.AnalyzeHistoricalTrends(30)
	if err == nil {
		analysis.TrendAnalysis = trends
	}

	return analysis, nil
}

// GetTestRunsInDateRange retrieves test runs within a specific date range
func (rpm *ResultPersistenceManager) GetTestRunsInDateRange(start, end time.Time) ([]*TestRunRecord, error) {
	query := `
		SELECT id, timestamp, suite_version, execution_mode, 
		       total_tests, passed_tests, failed_tests, skipped_tests,
		       total_duration, e2e_score, regression_status, baseline_id
		FROM test_runs 
		WHERE timestamp BETWEEN ? AND ?
		ORDER BY timestamp DESC`

	rows, err := rpm.db.Query(query, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query test runs in date range: %w", err)
	}
	defer rows.Close()

	var records []*TestRunRecord
	for rows.Next() {
		record := &TestRunRecord{}
		var totalDurationNs int64

		err := rows.Scan(
			&record.ID, &record.Timestamp, &record.SuiteVersion,
			&record.ExecutionMode, &record.TotalTests, &record.PassedTests,
			&record.FailedTests, &record.SkippedTests, &totalDurationNs,
			&record.E2EScore, &record.RegressionStatus, &record.BaselineID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan test run: %w", err)
		}

		record.TotalDuration = time.Duration(totalDurationNs)
		records = append(records, record)
	}

	return records, nil
}

// UpdateBaseline updates the baseline for regression detection
func (rpm *ResultPersistenceManager) UpdateBaseline(runID string) error {
	// Verify run exists
	_, err := rpm.GetTestRun(runID)
	if err != nil {
		return fmt.Errorf("cannot set baseline to non-existent run: %w", err)
	}

	// Update baseline metadata
	query := `UPDATE test_runs SET baseline_id = ? WHERE baseline_id IS NOT NULL OR baseline_id = ''`
	_, err = rpm.db.Exec(query, "")
	if err != nil {
		return fmt.Errorf("failed to clear previous baseline: %w", err)
	}

	query = `UPDATE test_runs SET baseline_id = ? WHERE id = ?`
	_, err = rpm.db.Exec(query, runID, runID)
	if err != nil {
		return fmt.Errorf("failed to set new baseline: %w", err)
	}

	return nil
}

// GenerateComprehensiveReport generates a comprehensive historical report
func (rpm *ResultPersistenceManager) GenerateComprehensiveReport(days int) (map[string]interface{}, error) {
	recentRuns, err := rpm.GetRecentTestRuns(50)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent runs: %w", err)
	}

	trends, err := rpm.AnalyzeHistoricalTrends(days)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze trends: %w", err)
	}

	// Calculate summary statistics
	var totalRuns, totalPassed, totalFailed int
	var avgScore, avgDuration float64

	for _, run := range recentRuns {
		totalRuns++
		totalPassed += run.PassedTests
		totalFailed += run.FailedTests
		avgScore += run.E2EScore
		avgDuration += run.TotalDuration.Seconds()
	}

	if totalRuns > 0 {
		avgScore /= float64(totalRuns)
		avgDuration /= float64(totalRuns)
	}

	report := map[string]interface{}{
		"summary": map[string]interface{}{
			"total_runs":         totalRuns,
			"total_tests_passed": totalPassed,
			"total_tests_failed": totalFailed,
			"average_e2e_score":  avgScore,
			"average_duration_s": avgDuration,
			"success_rate":       float64(totalPassed) / float64(totalPassed+totalFailed) * 100,
		},
		"recent_runs":  recentRuns[:min(10, len(recentRuns))],
		"trends":       trends,
		"generated_at": time.Now(),
		"period_days":  days,
	}

	return report, nil
}

// Close closes the persistence manager and cleans up resources
func (rpm *ResultPersistenceManager) Close() error {
	if rpm.cancel != nil {
		rpm.cancel()
	}

	if rpm.db != nil {
		return rpm.db.Close()
	}

	return nil
}

// Private helper methods

func (rpm *ResultPersistenceManager) initializeDatabase() error {
	schema := `
	CREATE TABLE IF NOT EXISTS test_runs (
		id TEXT PRIMARY KEY,
		timestamp DATETIME NOT NULL,
		suite_version TEXT NOT NULL,
		execution_mode TEXT NOT NULL,
		total_tests INTEGER NOT NULL,
		passed_tests INTEGER NOT NULL,
		failed_tests INTEGER NOT NULL,
		skipped_tests INTEGER NOT NULL,
		total_duration INTEGER NOT NULL,
		e2e_score REAL NOT NULL,
		regression_status TEXT,
		baseline_id TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_test_runs_timestamp ON test_runs(timestamp);
	CREATE INDEX IF NOT EXISTS idx_test_runs_baseline ON test_runs(baseline_id);
	CREATE INDEX IF NOT EXISTS idx_test_runs_score ON test_runs(e2e_score);

	CREATE TABLE IF NOT EXISTS test_results (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		run_id TEXT NOT NULL,
		test_name TEXT NOT NULL,
		category TEXT NOT NULL,
		status TEXT NOT NULL,
		duration INTEGER NOT NULL,
		error_message TEXT,
		timestamp DATETIME NOT NULL,
		FOREIGN KEY (run_id) REFERENCES test_runs(id)
	);

	CREATE INDEX IF NOT EXISTS idx_test_results_run_id ON test_results(run_id);
	CREATE INDEX IF NOT EXISTS idx_test_results_status ON test_results(status);
	`

	_, err := rpm.db.Exec(schema)
	return err
}

func (rpm *ResultPersistenceManager) saveToDatabase(record *TestRunRecord) error {
	tx, err := rpm.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert test run
	query := `
		INSERT OR REPLACE INTO test_runs 
		(id, timestamp, suite_version, execution_mode, total_tests, 
		 passed_tests, failed_tests, skipped_tests, total_duration, 
		 e2e_score, regression_status, baseline_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = tx.Exec(query,
		record.ID, record.Timestamp, record.SuiteVersion, record.ExecutionMode,
		record.TotalTests, record.PassedTests, record.FailedTests, record.SkippedTests,
		int64(record.TotalDuration), record.E2EScore, record.RegressionStatus, record.BaselineID)
	if err != nil {
		return err
	}

	// Insert test results
	for _, result := range record.TestResults {
		query = `
			INSERT INTO test_results 
			(run_id, test_name, category, status, duration, error_message, timestamp)
			VALUES (?, ?, ?, ?, ?, ?, ?)`

		_, err = tx.Exec(query,
			record.ID, result.TestName, result.Category, result.Status,
			int64(result.Duration), result.ErrorMessage, result.Timestamp)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (rpm *ResultPersistenceManager) saveToJSON(record *TestRunRecord) error {
	filename := filepath.Join(rpm.config.JSONResultsDirectory, fmt.Sprintf("%s.json", record.ID))

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

func (rpm *ResultPersistenceManager) getFromDatabase(runID string) (*TestRunRecord, error) {
	query := `
		SELECT id, timestamp, suite_version, execution_mode, 
		       total_tests, passed_tests, failed_tests, skipped_tests,
		       total_duration, e2e_score, regression_status, baseline_id
		FROM test_runs 
		WHERE id = ?`

	row := rpm.db.QueryRow(query, runID)

	record := &TestRunRecord{}
	var totalDurationNs int64

	err := row.Scan(
		&record.ID, &record.Timestamp, &record.SuiteVersion,
		&record.ExecutionMode, &record.TotalTests, &record.PassedTests,
		&record.FailedTests, &record.SkippedTests, &totalDurationNs,
		&record.E2EScore, &record.RegressionStatus, &record.BaselineID,
	)
	if err != nil {
		return nil, err
	}

	record.TotalDuration = time.Duration(totalDurationNs)

	// Load test results
	resultsQuery := `
		SELECT test_name, category, status, duration, error_message, timestamp
		FROM test_results 
		WHERE run_id = ?
		ORDER BY timestamp`

	rows, err := rpm.db.Query(resultsQuery, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		result := &TestResult{}
		var durationNs int64

		err := rows.Scan(&result.TestName, &result.Category, &result.Status,
			&durationNs, &result.ErrorMessage, &result.Timestamp)
		if err != nil {
			return nil, err
		}

		result.Duration = time.Duration(durationNs)
		record.TestResults = append(record.TestResults, result)
	}

	return record, nil
}

func (rpm *ResultPersistenceManager) getFromJSON(runID string) (*TestRunRecord, error) {
	filename := filepath.Join(rpm.config.JSONResultsDirectory, fmt.Sprintf("%s.json", runID))

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var record TestRunRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, err
	}

	return &record, nil
}

func (rpm *ResultPersistenceManager) archiveResult(record *TestRunRecord) error {
	// Simple archival - create compressed JSON
	archiveFile := filepath.Join(rpm.config.ArchiveDirectory,
		fmt.Sprintf("%s_archive.json", record.ID))

	data, err := json.Marshal(record)
	if err != nil {
		return err
	}

	return os.WriteFile(archiveFile, data, 0644)
}

func (rpm *ResultPersistenceManager) backgroundMaintenance() {
	ticker := time.NewTicker(rpm.config.BackupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rpm.ctx.Done():
			return
		case <-ticker.C:
			rpm.performMaintenance()
		}
	}
}

func (rpm *ResultPersistenceManager) performMaintenance() {
	// Clean up old records
	cutoff := time.Now().AddDate(0, 0, -rpm.config.RetentionDays)

	query := `DELETE FROM test_runs WHERE timestamp < ?`
	if _, err := rpm.db.Exec(query, cutoff); err != nil {
		fmt.Printf("Maintenance error: failed to clean old records: %v\n", err)
	}

	// Clean up orphaned test results
	query = `DELETE FROM test_results WHERE run_id NOT IN (SELECT id FROM test_runs)`
	if _, err := rpm.db.Exec(query); err != nil {
		fmt.Printf("Maintenance error: failed to clean orphaned results: %v\n", err)
	}

	// Vacuum database
	if _, err := rpm.db.Exec("VACUUM"); err != nil {
		fmt.Printf("Maintenance error: failed to vacuum database: %v\n", err)
	}
}

func (rpm *ResultPersistenceManager) analyzeTrend(metricName string, dataPoints interface{}, extractor func(interface{}) float64) HistoricalTrend {
	points := dataPoints.([]struct {
		ID        string
		Timestamp time.Time
		Score     float64
		Duration  time.Duration
		Passed    int
		Failed    int
	})

	var trendPoints []TrendDataPoint
	for _, dp := range points {
		trendPoints = append(trendPoints, TrendDataPoint{
			Timestamp: dp.Timestamp,
			Value:     extractor(dp),
			RunID:     dp.ID,
		})
	}

	// Calculate trend direction and slope
	direction := "stable"
	var slope float64

	if len(trendPoints) >= 2 {
		first := trendPoints[0].Value
		last := trendPoints[len(trendPoints)-1].Value

		if last > first*1.05 {
			direction = "improving"
		} else if last < first*0.95 {
			direction = "degrading"
		}

		// Simple linear regression slope
		n := len(trendPoints)
		var sumX, sumY, sumXY, sumX2 float64

		for i, point := range trendPoints {
			x := float64(i)
			y := point.Value
			sumX += x
			sumY += y
			sumXY += x * y
			sumX2 += x * x
		}

		slope = (float64(n)*sumXY - sumX*sumY) / (float64(n)*sumX2 - sumX*sumX)
	}

	return HistoricalTrend{
		Metric:     metricName,
		DataPoints: trendPoints,
		Direction:  direction,
		TrendSlope: slope,
	}
}

func (rpm *ResultPersistenceManager) calculateSeverity(percentChange float64) string {
	abs := percentChange
	if abs < 0 {
		abs = -abs
	}

	if abs > 50 {
		return "critical"
	} else if abs > 25 {
		return "high"
	} else if abs > 10 {
		return "medium"
	}
	return "low"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
