package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"lsp-gateway/mcp"
	"lsp-gateway/tests/mocks"
)

// IntegrationUtilities provides comprehensive utilities for E2E test integration
type IntegrationUtilities struct {
	reportGenerator    *TestReportGenerator
	healthValidator    *HealthMonitoringValidator
	resultAggregator   *ResultAggregator
	cleanupManager     *CleanupManager
}

// NewIntegrationUtilities creates a new integration utilities instance
func NewIntegrationUtilities() *IntegrationUtilities {
	return &IntegrationUtilities{
		reportGenerator:  NewTestReportGenerator(),
		healthValidator:  NewHealthMonitoringValidator(),
		resultAggregator: NewResultAggregator(),
		cleanupManager:   NewCleanupManager(),
	}
}

// TestReportGenerator creates comprehensive test reports
type TestReportGenerator struct {
	startTime time.Time
	reports   map[string]*TestReport
}

// TestReport contains detailed test execution information
type TestReport struct {
	TestSuite        string                         `json:"test_suite"`
	StartTime        time.Time                      `json:"start_time"`
	EndTime          time.Time                      `json:"end_time"`
	Duration         time.Duration                  `json:"duration"`
	TotalScenarios   int                           `json:"total_scenarios"`
	PassedScenarios  int                           `json:"passed_scenarios"`
	FailedScenarios  int                           `json:"failed_scenarios"`
	SuccessRate      float64                       `json:"success_rate"`
	ScenarioResults  map[E2EScenario]*E2ETestResult `json:"scenario_results"`
	GlobalMetrics    *GlobalTestMetrics            `json:"global_metrics"`
	ErrorAnalysis    *ErrorAnalysis                `json:"error_analysis"`
	Recommendations  []string                      `json:"recommendations"`
}

// GlobalTestMetrics contains overall test execution metrics
type GlobalTestMetrics struct {
	TotalRequests      int64         `json:"total_requests"`
	SuccessfulRequests int64         `json:"successful_requests"`
	FailedRequests     int64         `json:"failed_requests"`
	AverageLatency     time.Duration `json:"average_latency"`
	ErrorRate          float64       `json:"error_rate"`
}

// ErrorAnalysis contains comprehensive error analysis
type ErrorAnalysis struct {
	ErrorCategories      map[mcp.ErrorCategory]int `json:"error_categories"`
	CircuitBreakerEvents int                       `json:"circuit_breaker_events"`
	RecoveryTimes        []time.Duration           `json:"recovery_times"`
}

// NewTestReportGenerator creates a new test report generator
func NewTestReportGenerator() *TestReportGenerator {
	return &TestReportGenerator{
		startTime: time.Now(),
		reports:   make(map[string]*TestReport),
	}
}

// GenerateComprehensiveReport creates a detailed test execution report
func (g *TestReportGenerator) GenerateComprehensiveReport(testSuite string, results map[E2EScenario]*E2ETestResult) *TestReport {
	endTime := time.Now()
	
	report := &TestReport{
		TestSuite:       testSuite,
		StartTime:       g.startTime,
		EndTime:         endTime,
		Duration:        endTime.Sub(g.startTime),
		TotalScenarios:  len(results),
		ScenarioResults: results,
	}
	
	// Calculate success metrics
	passedCount := 0
	for _, result := range results {
		if result.Success {
			passedCount++
		}
	}
	
	report.PassedScenarios = passedCount
	report.FailedScenarios = report.TotalScenarios - passedCount
	report.SuccessRate = float64(passedCount) / float64(report.TotalScenarios)
	
	// Generate analytics
	report.GlobalMetrics = g.calculateGlobalMetrics(results)
	report.ErrorAnalysis = g.analyzeErrors(results)
	report.Recommendations = g.generateRecommendations(report)
	
	g.reports[testSuite] = report
	return report
}

// ExportReport exports the report in various formats
func (g *TestReportGenerator) ExportReport(report *TestReport, format string) ([]byte, error) {
	switch format {
	case "json":
		return json.MarshalIndent(report, "", "  ")
	case "summary":
		return g.generateSummary(report), nil
	default:
		return nil, fmt.Errorf("unsupported report format: %s", format)
	}
}

// HealthMonitoringValidator validates health monitoring functionality
type HealthMonitoringValidator struct {
	healthChecks []HealthCheckRecord
}

// HealthCheckRecord records health check results
type HealthCheckRecord struct {
	Timestamp    time.Time              `json:"timestamp"`
	Component    string                 `json:"component"`
	Status       HealthStatus           `json:"status"`
	ResponseTime time.Duration          `json:"response_time"`
	Details      map[string]interface{} `json:"details"`
}

type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// NewHealthMonitoringValidator creates a new health monitoring validator
func NewHealthMonitoringValidator() *HealthMonitoringValidator {
	return &HealthMonitoringValidator{
		healthChecks: make([]HealthCheckRecord, 0),
	}
}

// ValidateHealthMonitoring tests health monitoring functionality
func (v *HealthMonitoringValidator) ValidateHealthMonitoring(ctx context.Context, mockClient *mocks.MockMcpClient) (*HealthValidationResult, error) {
	result := &HealthValidationResult{
		StartTime:    time.Now(),
		HealthChecks: make([]HealthCheckRecord, 0),
	}
	
	// Test basic health check
	healthStatus := v.checkBasicHealth(ctx, mockClient)
	result.HealthChecks = append(result.HealthChecks, healthStatus)
	
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	
	// Calculate overall health score
	result.OverallHealthScore = v.calculateHealthScore(result.HealthChecks)
	result.Success = result.OverallHealthScore >= 0.8
	
	return result, nil
}

// HealthValidationResult contains health validation results
type HealthValidationResult struct {
	StartTime          time.Time           `json:"start_time"`
	EndTime            time.Time           `json:"end_time"`
	Duration           time.Duration       `json:"duration"`
	Success            bool                `json:"success"`
	OverallHealthScore float64             `json:"overall_health_score"`
	HealthChecks       []HealthCheckRecord `json:"health_checks"`
	Recommendations    []string            `json:"recommendations"`
}

// ResultAggregator aggregates and analyzes test results
type ResultAggregator struct {
	aggregatedResults map[string]*AggregatedResult
}

// AggregatedResult contains aggregated test results
type AggregatedResult struct {
	TestSuites      []string                         `json:"test_suites"`
	TotalScenarios  int                             `json:"total_scenarios"`
	OverallSuccess  bool                            `json:"overall_success"`
	ScenarioSummary map[E2EScenario]*ScenarioSummary `json:"scenario_summary"`
}

type ScenarioSummary struct {
	TotalRuns      int           `json:"total_runs"`
	SuccessfulRuns int           `json:"successful_runs"`
	FailedRuns     int           `json:"failed_runs"`
	SuccessRate    float64       `json:"success_rate"`
	AverageLatency time.Duration `json:"average_latency"`
}

// NewResultAggregator creates a new result aggregator
func NewResultAggregator() *ResultAggregator {
	return &ResultAggregator{
		aggregatedResults: make(map[string]*AggregatedResult),
	}
}

// CleanupManager manages resource cleanup for E2E tests
type CleanupManager struct {
	cleanupFunctions []CleanupFunction
	resources        []TestResource
}

// CleanupFunction represents a cleanup function
type CleanupFunction func() error

// TestResource represents a test resource that needs cleanup
type TestResource struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	CreatedAt   time.Time              `json:"created_at"`
	CleanupFunc CleanupFunction        `json:"-"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// NewCleanupManager creates a new cleanup manager
func NewCleanupManager() *CleanupManager {
	return &CleanupManager{
		cleanupFunctions: make([]CleanupFunction, 0),
		resources:        make([]TestResource, 0),
	}
}

// RegisterCleanup registers a cleanup function
func (c *CleanupManager) RegisterCleanup(fn CleanupFunction) {
	c.cleanupFunctions = append(c.cleanupFunctions, fn)
}

// PerformCleanup executes all registered cleanup functions
func (c *CleanupManager) PerformCleanup(ctx context.Context) error {
	var errors []error
	
	// Execute cleanup functions in reverse order
	for i := len(c.cleanupFunctions) - 1; i >= 0; i-- {
		if err := c.cleanupFunctions[i](); err != nil {
			errors = append(errors, fmt.Errorf("cleanup function failed: %w", err))
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}
	
	return nil
}

// Private helper methods

func (g *TestReportGenerator) calculateGlobalMetrics(results map[E2EScenario]*E2ETestResult) *GlobalTestMetrics {
	metrics := &GlobalTestMetrics{}
	
	totalDuration := time.Duration(0)
	requestCount := int64(0)
	
	for _, result := range results {
		if result.Metrics != nil {
			metrics.TotalRequests += result.Metrics.TotalRequests
			metrics.SuccessfulRequests += result.Metrics.SuccessfulReqs
			metrics.FailedRequests += result.Metrics.FailedRequests
			totalDuration += result.Metrics.AverageLatency
			requestCount++
		}
	}
	
	if metrics.TotalRequests > 0 {
		metrics.ErrorRate = float64(metrics.FailedRequests) / float64(metrics.TotalRequests)
	}
	
	if requestCount > 0 {
		metrics.AverageLatency = totalDuration / time.Duration(requestCount)
	}
	
	return metrics
}

func (g *TestReportGenerator) analyzeErrors(results map[E2EScenario]*E2ETestResult) *ErrorAnalysis {
	analysis := &ErrorAnalysis{
		ErrorCategories: make(map[mcp.ErrorCategory]int),
		RecoveryTimes:   make([]time.Duration, 0),
	}
	
	// This would analyze errors from the results
	// For now, return basic structure
	
	return analysis
}

func (g *TestReportGenerator) generateRecommendations(report *TestReport) []string {
	recommendations := make([]string, 0)
	
	if report.SuccessRate < 0.95 {
		recommendations = append(recommendations, "Consider investigating failed scenarios to improve overall success rate")
	}
	
	if report.GlobalMetrics.ErrorRate > 0.05 {
		recommendations = append(recommendations, "High error rate detected - review error handling and retry policies")
	}
	
	if report.GlobalMetrics.AverageLatency > 2*time.Second {
		recommendations = append(recommendations, "Average latency is high - consider performance optimization")
	}
	
	return recommendations
}

func (g *TestReportGenerator) generateSummary(report *TestReport) []byte {
	summary := fmt.Sprintf(`
E2E Test Report Summary
======================
Test Suite: %s
Duration: %v
Total Scenarios: %d
Passed: %d
Failed: %d
Success Rate: %.2f%%

Performance Summary:
- Total Requests: %d
- Average Latency: %v
- Error Rate: %.2f%%

Top Recommendations:
%s
`, 
		report.TestSuite,
		report.Duration,
		report.TotalScenarios,
		report.PassedScenarios,
		report.FailedScenarios,
		report.SuccessRate*100,
		report.GlobalMetrics.TotalRequests,
		report.GlobalMetrics.AverageLatency,
		report.GlobalMetrics.ErrorRate*100,
		formatRecommendations(report.Recommendations),
	)
	
	return []byte(summary)
}

func formatRecommendations(recommendations []string) string {
	if len(recommendations) == 0 {
		return "- No specific recommendations"
	}
	
	result := ""
	for i, rec := range recommendations {
		result += fmt.Sprintf("- %s", rec)
		if i < len(recommendations)-1 {
			result += "\n"
		}
	}
	return result
}

// Health monitoring helper methods

func (v *HealthMonitoringValidator) checkBasicHealth(ctx context.Context, mockClient *mocks.MockMcpClient) HealthCheckRecord {
	start := time.Now()
	err := mockClient.GetHealth(ctx)
	responseTime := time.Since(start)
	
	status := HealthStatusHealthy
	if err != nil {
		status = HealthStatusUnhealthy
	}
	
	return HealthCheckRecord{
		Timestamp:    start,
		Component:    "mcp-client",
		Status:       status,
		ResponseTime: responseTime,
		Details: map[string]interface{}{
			"healthy": err == nil,
			"error":   fmt.Sprintf("%v", err),
		},
	}
}

func (v *HealthMonitoringValidator) calculateHealthScore(checks []HealthCheckRecord) float64 {
	if len(checks) == 0 {
		return 0.0
	}
	
	healthyCount := 0
	for _, check := range checks {
		if check.Status == HealthStatusHealthy {
			healthyCount++
		}
	}
	
	return float64(healthyCount) / float64(len(checks))
}