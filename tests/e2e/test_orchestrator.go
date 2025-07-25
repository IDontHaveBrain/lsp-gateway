package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"lsp-gateway/tests/framework"
	"lsp-gateway/tests/mocks"
)

// E2ETestOrchestrator coordinates comprehensive E2E test discovery and execution
type E2ETestOrchestrator struct {
	// Core engines
	discoveryEngine  *TestDiscoveryEngine
	executionEngine  *TestExecutionEngine
	
	// Framework integration
	testFramework    *framework.MultiLanguageTestFramework
	mcpMockClient    *mocks.MockMcpClient
	
	// Configuration
	config           *OrchestratorConfig
	workingDirectory string
	resultsDirectory string
	
	// State management
	currentPlan      *ExecutionPlan
	executionHistory []*OrchestratorExecution
	
	// Reporting
	reportGenerator  *TestReportGenerator
	
	// Synchronization
	mu               sync.RWMutex
	
	// Context management
	ctx              context.Context
	cancel           context.CancelFunc
}

// OrchestratorConfig defines configuration for the E2E test orchestrator
type OrchestratorConfig struct {
	// Discovery settings
	DiscoveryPackages    []string
	TestPatterns         []string
	ExcludePatterns      []string
	MaxDiscoveryDepth    int
	
	// Execution settings
	DefaultStrategy      ExecutionStrategy
	MaxConcurrency       int
	DefaultTimeout       time.Duration
	EnableRetries        bool
	MaxRetries           int
	
	// Resource settings
	MemoryLimitMB        int64
	CPULimit             int
	NetworkPorts         []int
	
	// Reporting settings
	GenerateReports      bool
	ReportFormats        []ReportFormat
	SaveBaseline         bool
	CompareBaseline      bool
	
	// Integration settings
	UseRealServers       bool
	ServerConfigs        map[string]string
	TestDataDirectory    string
	CleanupAfterRun      bool
	
	// Monitoring settings
	EnableMonitoring     bool
	MonitoringInterval   time.Duration
	AlertThresholds      *MonitoringThresholds
}

// OrchestratorExecution represents a complete orchestrated test execution
type OrchestratorExecution struct {
	ID                   string
	StartTime            time.Time
	EndTime              time.Time
	Duration             time.Duration
	
	// Discovery results
	DiscoveryResults     *TestDiscoveryMetrics
	TestsDiscovered      int
	
	// Execution plan
	ExecutionPlan        *ExecutionPlan
	
	// Execution results
	ExecutionResults     *ExecutionSummary
	
	// Reports
	GeneratedReports     []string
	
	// Configuration used
	ConfigSnapshot       *OrchestratorConfig
	
	// Status
	Status               ExecutionStatus
	Error                error
}

// ExecutionSummary provides a summary of test execution results
type ExecutionSummary struct {
	TotalTests           int
	TestsPassed          int
	TestsFailed          int
	TestsSkipped         int
	TestsRetried         int
	
	// Timing
	TotalDuration        time.Duration
	AverageTestDuration  time.Duration
	SlowestTest          *TestExecutionResult
	FastestTest          *TestExecutionResult
	
	// Categories
	CategoryResults      map[TestCategory]*CategorySummary
	PriorityResults      map[TestPriority]*PrioritySummary
	
	// Resources
	ResourceUtilization  *ResourceUtilizationMetrics
	
	// Failures
	CriticalFailures     []string
	FailuresByCategory   map[TestCategory][]string
	
	// Performance
	ThroughputPerSecond  float64
	MemoryPeakMB         int64
	CPUPeakPercent       float64
}

// CategorySummary provides summary for a test category
type CategorySummary struct {
	Category         TestCategory
	TotalTests       int
	Passed           int
	Failed           int
	Duration         time.Duration
	SuccessRate      float64
}

// PrioritySummary provides summary for a test priority level
type PrioritySummary struct {
	Priority         TestPriority
	TotalTests       int
	Passed           int
	Failed           int
	Duration         time.Duration
	SuccessRate      float64
}

// ExecutionStatus represents the status of an orchestrated execution
type ExecutionStatus string

const (
	StatusInitializing   ExecutionStatus = "initializing"
	StatusDiscovering    ExecutionStatus = "discovering"
	StatusPlanning       ExecutionStatus = "planning"
	StatusExecuting      ExecutionStatus = "executing"
	StatusReporting      ExecutionStatus = "reporting"
	StatusCompleted      ExecutionStatus = "completed"
	StatusFailed         ExecutionStatus = "failed"
	StatusCancelled      ExecutionStatus = "cancelled"
)

// TestReportGenerator generates comprehensive test reports
type TestReportGenerator struct {
	outputDirectory  string
	formats          []ReportFormat
	templateDirectory string
	baselineData     *BaselineData
}

// ReportFormat defines different report output formats
type ReportFormat string

const (
	FormatJSON       ReportFormat = "json"
	FormatHTML       ReportFormat = "html"
	FormatXML        ReportFormat = "xml"
	FormatMarkdown   ReportFormat = "markdown"
	FormatJUnit      ReportFormat = "junit"
)

// BaselineData contains baseline test data for comparison
type BaselineData struct {
	Version          string
	Timestamp        time.Time
	TestResults      map[string]*TestExecutionResult
	SystemInfo       *SystemInfo
	PerformanceData  *PerformanceBaseline
}

// PerformanceBaseline contains performance baseline data
type PerformanceBaseline struct {
	AverageTestDuration  map[string]time.Duration
	MemoryUsage          map[string]int64
	CPUUsage             map[string]float64
	ThroughputTargets    map[TestCategory]float64
	SuccessRateTargets   map[TestCategory]float64
}

// NewE2ETestOrchestrator creates a new E2E test orchestrator
func NewE2ETestOrchestrator(config *OrchestratorConfig) (*E2ETestOrchestrator, error) {
	if config == nil {
		config = getDefaultOrchestratorConfig()
	}
	
	// Validate configuration
	if err := validateOrchestratorConfig(config); err != nil {
		return nil, fmt.Errorf("invalid orchestrator config: %w", err)
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create working and results directories
	workingDir, err := os.MkdirTemp("", "e2e-orchestrator-*")
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create working directory: %w", err)
	}
	
	resultsDir := filepath.Join(workingDir, "results")
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		cancel()
		os.RemoveAll(workingDir)
		return nil, fmt.Errorf("failed to create results directory: %w", err)
	}
	
	orchestrator := &E2ETestOrchestrator{
		config:           config,
		workingDirectory: workingDir,
		resultsDirectory: resultsDir,
		executionHistory: make([]*OrchestratorExecution, 0),
		ctx:              ctx,
		cancel:           cancel,
	}
	
	// Initialize discovery engine
	orchestrator.discoveryEngine = NewTestDiscoveryEngine()
	
	// Initialize execution engine (will be created after discovery)
	
	// Initialize test framework
	frameworkTimeout := config.DefaultTimeout
	if frameworkTimeout > 2*time.Hour {
		frameworkTimeout = 2 * time.Hour
	}
	orchestrator.testFramework = framework.NewMultiLanguageTestFramework(frameworkTimeout)
	
	// Initialize mock MCP client
	orchestrator.mcpMockClient = mocks.NewMockMcpClient()
	
	// Initialize report generator
	orchestrator.reportGenerator = NewTestReportGenerator(resultsDir, config.ReportFormats)
	
	return orchestrator, nil
}

// RunCompleteE2ETestSuite runs the complete E2E test suite with discovery and execution
func (orchestrator *E2ETestOrchestrator) RunCompleteE2ETestSuite(ctx context.Context, filter *TestFilter) (*OrchestratorExecution, error) {
	orchestrator.mu.Lock()
	defer orchestrator.mu.Unlock()
	
	execution := &OrchestratorExecution{
		ID:               fmt.Sprintf("exec_%d", time.Now().Unix()),
		StartTime:        time.Now(),
		Status:           StatusInitializing,
		ConfigSnapshot:   orchestrator.copyConfig(),
		GeneratedReports: make([]string, 0),
	}
	
	defer func() {
		execution.EndTime = time.Now()
		execution.Duration = execution.EndTime.Sub(execution.StartTime)
		orchestrator.executionHistory = append(orchestrator.executionHistory, execution)
	}()
	
	// Phase 1: Initialize environment
	if err := orchestrator.initializeEnvironment(ctx, execution); err != nil {
		execution.Status = StatusFailed
		execution.Error = fmt.Errorf("environment initialization failed: %w", err)
		return execution, execution.Error
	}
	
	// Phase 2: Discover tests
	execution.Status = StatusDiscovering
	if err := orchestrator.discoverTests(ctx, execution); err != nil {
		execution.Status = StatusFailed
		execution.Error = fmt.Errorf("test discovery failed: %w", err)
		return execution, execution.Error
	}
	
	// Phase 3: Create execution plan
	execution.Status = StatusPlanning
	if err := orchestrator.createExecutionPlan(ctx, execution, filter); err != nil {
		execution.Status = StatusFailed
		execution.Error = fmt.Errorf("execution planning failed: %w", err)
		return execution, execution.Error
	}
	
	// Phase 4: Execute tests
	execution.Status = StatusExecuting
	if err := orchestrator.executeTests(ctx, execution); err != nil {
		execution.Status = StatusFailed
		execution.Error = fmt.Errorf("test execution failed: %w", err)
		return execution, execution.Error
	}
	
	// Phase 5: Generate reports
	execution.Status = StatusReporting
	if err := orchestrator.generateReports(ctx, execution); err != nil {
		// Don't fail the entire execution for reporting errors
		fmt.Printf("Warning: Report generation failed: %v\n", err)
	}
	
	// Phase 6: Cleanup
	if orchestrator.config.CleanupAfterRun {
		if err := orchestrator.cleanup(ctx); err != nil {
			fmt.Printf("Warning: Cleanup failed: %v\n", err)
		}
	}
	
	execution.Status = StatusCompleted
	return execution, nil
}

// initializeEnvironment initializes the test environment
func (orchestrator *E2ETestOrchestrator) initializeEnvironment(ctx context.Context, execution *OrchestratorExecution) error {
	// Initialize test framework
	if err := orchestrator.testFramework.SetupTestEnvironment(ctx); err != nil {
		return fmt.Errorf("failed to setup test framework: %w", err)
	}
	
	// Configure mock MCP client
	orchestrator.mcpMockClient.SetTimeout(orchestrator.config.DefaultTimeout)
	orchestrator.mcpMockClient.SetMaxRetries(orchestrator.config.MaxRetries)
	orchestrator.mcpMockClient.SetHealthy(true)
	
	// Initialize directories
	if err := os.MkdirAll(orchestrator.resultsDirectory, 0755); err != nil {
		return fmt.Errorf("failed to create results directory: %w", err)
	}
	
	return nil
}

// discoverTests discovers all available E2E tests
func (orchestrator *E2ETestOrchestrator) discoverTests(ctx context.Context, execution *OrchestratorExecution) error {
	// Run test discovery
	packages := orchestrator.config.DiscoveryPackages
	if len(packages) == 0 {
		packages = []string{"."} // Default to current package
	}
	
	if err := orchestrator.discoveryEngine.DiscoverTests(ctx, packages...); err != nil {
		return fmt.Errorf("test discovery failed: %w", err)
	}
	
	// Get discovery results
	execution.DiscoveryResults = orchestrator.discoveryEngine.GetDiscoveryMetrics()
	execution.TestsDiscovered = execution.DiscoveryResults.TotalTestsDiscovered
	
	if execution.TestsDiscovered == 0 {
		return fmt.Errorf("no tests discovered")
	}
	
	return nil
}

// createExecutionPlan creates an optimized execution plan
func (orchestrator *E2ETestOrchestrator) createExecutionPlan(ctx context.Context, execution *OrchestratorExecution, filter *TestFilter) error {
	// Initialize execution engine with discovery results
	orchestrator.executionEngine = NewTestExecutionEngine(orchestrator.discoveryEngine)
	
	// Create execution plan
	plan, err := orchestrator.executionEngine.CreateExecutionPlan(ctx, filter, orchestrator.config.DefaultStrategy)
	if err != nil {
		return fmt.Errorf("failed to create execution plan: %w", err)
	}
	
	execution.ExecutionPlan = plan
	orchestrator.currentPlan = plan
	
	return nil
}

// executeTests executes the test plan
func (orchestrator *E2ETestOrchestrator) executeTests(ctx context.Context, execution *OrchestratorExecution) error {
	if orchestrator.currentPlan == nil {
		return fmt.Errorf("no execution plan available")
	}
	
	// Execute the plan
	if err := orchestrator.executionEngine.ExecutePlan(ctx, orchestrator.currentPlan); err != nil {
		return fmt.Errorf("execution plan failed: %w", err)
	}
	
	// Collect execution results
	execution.ExecutionResults = orchestrator.buildExecutionSummary()
	
	return nil
}

// generateReports generates comprehensive test reports
func (orchestrator *E2ETestOrchestrator) generateReports(ctx context.Context, execution *OrchestratorExecution) error {
	if !orchestrator.config.GenerateReports {
		return nil
	}
	
	// Generate reports in all configured formats
	for _, format := range orchestrator.config.ReportFormats {
		reportPath, err := orchestrator.reportGenerator.GenerateReport(execution, format)
		if err != nil {
			return fmt.Errorf("failed to generate %s report: %w", format, err)
		}
		execution.GeneratedReports = append(execution.GeneratedReports, reportPath)
	}
	
	// Save baseline data if configured
	if orchestrator.config.SaveBaseline {
		if err := orchestrator.saveBaselineData(execution); err != nil {
			return fmt.Errorf("failed to save baseline data: %w", err)
		}
	}
	
	return nil
}

// buildExecutionSummary builds a comprehensive execution summary
func (orchestrator *E2ETestOrchestrator) buildExecutionSummary() *ExecutionSummary {
	completed, failed := orchestrator.executionEngine.GetExecutionResults()
	metrics := orchestrator.executionEngine.GetExecutionMetrics()
	
	summary := &ExecutionSummary{
		TotalTests:          len(completed) + len(failed),
		TestsPassed:         len(completed),
		TestsFailed:         len(failed),
		TestsRetried:        int(metrics.RetriedTests),
		TotalDuration:       metrics.TotalDuration,
		CategoryResults:     make(map[TestCategory]*CategorySummary),
		PriorityResults:     make(map[TestPriority]*PrioritySummary),
		ResourceUtilization: metrics.ResourceUtilization,
		FailuresByCategory:  make(map[TestCategory][]string),
		CriticalFailures:    make([]string, 0),
		ThroughputPerSecond: metrics.Throughput,
		MemoryPeakMB:        metrics.ResourceUtilization.PeakMemoryMB,
		CPUPeakPercent:      metrics.ResourceUtilization.PeakCPUPercent,
	}
	
	// Calculate average test duration
	if summary.TotalTests > 0 {
		summary.AverageTestDuration = time.Duration(int64(summary.TotalDuration) / int64(summary.TotalTests))
	}
	
	// Find slowest and fastest tests
	var slowest, fastest *TestExecutionResult
	for _, result := range completed {
		if slowest == nil || result.Duration > slowest.Duration {
			slowest = result
		}
		if fastest == nil || result.Duration < fastest.Duration {
			fastest = result
		}
	}
	for _, result := range failed {
		if slowest == nil || result.Duration > slowest.Duration {
			slowest = result
		}
		if fastest == nil || result.Duration < fastest.Duration {
			fastest = result
		}
	}
	summary.SlowestTest = slowest
	summary.FastestTest = fastest
	
	// Build category and priority summaries
	orchestrator.buildCategorySummaries(summary, completed, failed)
	orchestrator.buildPrioritySummaries(summary, completed, failed)
	
	// Identify critical failures
	for testName, result := range failed {
		if test, exists := orchestrator.discoveryEngine.GetTestByName(testName); exists {
			if test.Priority == PriorityCritical {
				summary.CriticalFailures = append(summary.CriticalFailures, 
					fmt.Sprintf("%s: %v", testName, result.Error))
			}
			
			// Group failures by category
			if summary.FailuresByCategory[test.Category] == nil {
				summary.FailuresByCategory[test.Category] = make([]string, 0)
			}
			summary.FailuresByCategory[test.Category] = append(
				summary.FailuresByCategory[test.Category], testName)
		}
	}
	
	return summary
}

// buildCategorySummaries builds category-specific summaries
func (orchestrator *E2ETestOrchestrator) buildCategorySummaries(summary *ExecutionSummary, completed, failed map[string]*TestExecutionResult) {
	categoryData := make(map[TestCategory]*CategorySummary)
	
	// Process completed tests
	for testName, result := range completed {
		if test, exists := orchestrator.discoveryEngine.GetTestByName(testName); exists {
			if categoryData[test.Category] == nil {
				categoryData[test.Category] = &CategorySummary{
					Category: test.Category,
				}
			}
			cat := categoryData[test.Category]
			cat.TotalTests++
			cat.Passed++
			cat.Duration += result.Duration
		}
	}
	
	// Process failed tests
	for testName, result := range failed {
		if test, exists := orchestrator.discoveryEngine.GetTestByName(testName); exists {
			if categoryData[test.Category] == nil {
				categoryData[test.Category] = &CategorySummary{
					Category: test.Category,
				}
			}
			cat := categoryData[test.Category]
			cat.TotalTests++
			cat.Failed++
			cat.Duration += result.Duration
		}
	}
	
	// Calculate success rates
	for category, catSummary := range categoryData {
		if catSummary.TotalTests > 0 {
			catSummary.SuccessRate = float64(catSummary.Passed) / float64(catSummary.TotalTests)
		}
		summary.CategoryResults[category] = catSummary
	}
}

// buildPrioritySummaries builds priority-specific summaries
func (orchestrator *E2ETestOrchestrator) buildPrioritySummaries(summary *ExecutionSummary, completed, failed map[string]*TestExecutionResult) {
	priorityData := make(map[TestPriority]*PrioritySummary)
	
	// Process completed tests
	for testName, result := range completed {
		if test, exists := orchestrator.discoveryEngine.GetTestByName(testName); exists {
			if priorityData[test.Priority] == nil {
				priorityData[test.Priority] = &PrioritySummary{
					Priority: test.Priority,
				}
			}
			pri := priorityData[test.Priority]
			pri.TotalTests++
			pri.Passed++
			pri.Duration += result.Duration
		}
	}
	
	// Process failed tests
	for testName, result := range failed {
		if test, exists := orchestrator.discoveryEngine.GetTestByName(testName); exists {
			if priorityData[test.Priority] == nil {
				priorityData[test.Priority] = &PrioritySummary{
					Priority: test.Priority,
				}
			}
			pri := priorityData[test.Priority]
			pri.TotalTests++
			pri.Failed++
			pri.Duration += result.Duration
		}
	}
	
	// Calculate success rates
	for priority, priSummary := range priorityData {
		if priSummary.TotalTests > 0 {
			priSummary.SuccessRate = float64(priSummary.Passed) / float64(priSummary.TotalTests)
		}
		summary.PriorityResults[priority] = priSummary
	}
}

// saveBaselineData saves current execution data as baseline
func (orchestrator *E2ETestOrchestrator) saveBaselineData(execution *OrchestratorExecution) error {
	completed, failed := orchestrator.executionEngine.GetExecutionResults()
	
	baseline := &BaselineData{
		Version:     "1.0.0", // This would come from version control
		Timestamp:   execution.StartTime,
		TestResults: make(map[string]*TestExecutionResult),
		SystemInfo:  &SystemInfo{
			OS:           "linux", // This would be detected
			Architecture: "amd64",
			NumCPU:       4,
			GoVersion:    "go1.21",
		},
		PerformanceData: &PerformanceBaseline{
			AverageTestDuration: make(map[string]time.Duration),
			MemoryUsage:         make(map[string]int64),
			CPUUsage:           make(map[string]float64),
			ThroughputTargets:   make(map[TestCategory]float64),
			SuccessRateTargets:  make(map[TestCategory]float64),
		},
	}
	
	// Include all test results in baseline
	for name, result := range completed {
		baseline.TestResults[name] = result
		baseline.PerformanceData.AverageTestDuration[name] = result.Duration
		baseline.PerformanceData.MemoryUsage[name] = result.Memory
	}
	for name, result := range failed {
		baseline.TestResults[name] = result
	}
	
	// Set category targets based on current performance
	for category, summary := range execution.ExecutionResults.CategoryResults {
		baseline.PerformanceData.SuccessRateTargets[category] = summary.SuccessRate
		// Set throughput target (simplified calculation)
		if summary.Duration > 0 {
			baseline.PerformanceData.ThroughputTargets[category] = float64(summary.TotalTests) / summary.Duration.Seconds()
		}
	}
	
	// Save baseline to file
	baselineFile := filepath.Join(orchestrator.resultsDirectory, "baseline.json")
	data, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal baseline data: %w", err)
	}
	
	if err := os.WriteFile(baselineFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write baseline file: %w", err)
	}
	
	return nil
}

// cleanup performs comprehensive cleanup
func (orchestrator *E2ETestOrchestrator) cleanup(ctx context.Context) error {
	var errors []error
	
	// Cleanup test framework
	if orchestrator.testFramework != nil {
		if err := orchestrator.testFramework.CleanupAll(); err != nil {
			errors = append(errors, fmt.Errorf("test framework cleanup failed: %w", err))
		}
	}
	
	// Reset mock client
	if orchestrator.mcpMockClient != nil {
		orchestrator.mcpMockClient.Reset()
	}
	
	// Cleanup working directory (but preserve results)
	tempDirs := []string{
		filepath.Join(orchestrator.workingDirectory, "temp"),
		filepath.Join(orchestrator.workingDirectory, "projects"),
		filepath.Join(orchestrator.workingDirectory, "servers"),
	}
	
	for _, dir := range tempDirs {
		if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
			errors = append(errors, fmt.Errorf("failed to cleanup %s: %w", dir, err))
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}
	
	return nil
}

// copyConfig creates a deep copy of the orchestrator configuration
func (orchestrator *E2ETestOrchestrator) copyConfig() *OrchestratorConfig {
	return &OrchestratorConfig{
		DiscoveryPackages:   append([]string(nil), orchestrator.config.DiscoveryPackages...),
		TestPatterns:        append([]string(nil), orchestrator.config.TestPatterns...),
		ExcludePatterns:     append([]string(nil), orchestrator.config.ExcludePatterns...),
		MaxDiscoveryDepth:   orchestrator.config.MaxDiscoveryDepth,
		DefaultStrategy:     orchestrator.config.DefaultStrategy,
		MaxConcurrency:      orchestrator.config.MaxConcurrency,
		DefaultTimeout:      orchestrator.config.DefaultTimeout,
		EnableRetries:       orchestrator.config.EnableRetries,
		MaxRetries:          orchestrator.config.MaxRetries,
		MemoryLimitMB:       orchestrator.config.MemoryLimitMB,
		CPULimit:            orchestrator.config.CPULimit,
		NetworkPorts:        append([]int(nil), orchestrator.config.NetworkPorts...),
		GenerateReports:     orchestrator.config.GenerateReports,
		ReportFormats:       append([]ReportFormat(nil), orchestrator.config.ReportFormats...),
		SaveBaseline:        orchestrator.config.SaveBaseline,
		CompareBaseline:     orchestrator.config.CompareBaseline,
		UseRealServers:      orchestrator.config.UseRealServers,
		TestDataDirectory:   orchestrator.config.TestDataDirectory,
		CleanupAfterRun:     orchestrator.config.CleanupAfterRun,
		EnableMonitoring:    orchestrator.config.EnableMonitoring,
		MonitoringInterval:  orchestrator.config.MonitoringInterval,
	}
}

// GetExecutionHistory returns the execution history
func (orchestrator *E2ETestOrchestrator) GetExecutionHistory() []*OrchestratorExecution {
	orchestrator.mu.RLock()
	defer orchestrator.mu.RUnlock()
	
	// Return a copy
	history := make([]*OrchestratorExecution, len(orchestrator.executionHistory))
	copy(history, orchestrator.executionHistory)
	return history
}

// GetCurrentPlan returns the current execution plan
func (orchestrator *E2ETestOrchestrator) GetCurrentPlan() *ExecutionPlan {
	orchestrator.mu.RLock()
	defer orchestrator.mu.RUnlock()
	return orchestrator.currentPlan
}

// GetDiscoveryEngine returns the discovery engine
func (orchestrator *E2ETestOrchestrator) GetDiscoveryEngine() *TestDiscoveryEngine {
	return orchestrator.discoveryEngine
}

// GetExecutionEngine returns the execution engine
func (orchestrator *E2ETestOrchestrator) GetExecutionEngine() *TestExecutionEngine {
	return orchestrator.executionEngine
}

// Close cleans up the orchestrator
func (orchestrator *E2ETestOrchestrator) Close() error {
	if orchestrator.cancel != nil {
		orchestrator.cancel()
	}
	
	return orchestrator.cleanup(context.Background())
}

// NewTestReportGenerator creates a new test report generator
func NewTestReportGenerator(outputDir string, formats []ReportFormat) *TestReportGenerator {
	return &TestReportGenerator{
		outputDirectory: outputDir,
		formats:         formats,
		templateDirectory: filepath.Join(outputDir, "templates"),
	}
}

// GenerateReport generates a test report in the specified format
func (generator *TestReportGenerator) GenerateReport(execution *OrchestratorExecution, format ReportFormat) (string, error) {
	switch format {
	case FormatJSON:
		return generator.generateJSONReport(execution)
	case FormatHTML:
		return generator.generateHTMLReport(execution)
	case FormatMarkdown:
		return generator.generateMarkdownReport(execution)
	case FormatXML:
		return generator.generateXMLReport(execution)
	case FormatJUnit:
		return generator.generateJUnitReport(execution)
	default:
		return "", fmt.Errorf("unsupported report format: %s", format)
	}
}

// generateJSONReport generates a JSON report
func (generator *TestReportGenerator) generateJSONReport(execution *OrchestratorExecution) (string, error) {
	reportPath := filepath.Join(generator.outputDirectory, fmt.Sprintf("report_%s.json", execution.ID))
	
	data, err := json.MarshalIndent(execution, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON report: %w", err)
	}
	
	if err := os.WriteFile(reportPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write JSON report: %w", err)
	}
	
	return reportPath, nil
}

// generateHTMLReport generates an HTML report
func (generator *TestReportGenerator) generateHTMLReport(execution *OrchestratorExecution) (string, error) {
	reportPath := filepath.Join(generator.outputDirectory, fmt.Sprintf("report_%s.html", execution.ID))
	
	// Create a comprehensive HTML report
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>E2E Test Report - %s</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .header { background: #f4f4f4; padding: 20px; border-radius: 5px; }
        .summary { margin: 20px 0; }
        .passed { color: green; }
        .failed { color: red; }
        .category { margin: 10px 0; padding: 10px; border-left: 3px solid #ccc; }
        table { border-collapse: collapse; width: 100%%; margin: 20px 0; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
    </style>
</head>
<body>
    <div class="header">
        <h1>E2E Test Report</h1>
        <p><strong>Execution ID:</strong> %s</p>
        <p><strong>Start Time:</strong> %s</p>
        <p><strong>Duration:</strong> %v</p>
        <p><strong>Status:</strong> %s</p>
    </div>
    
    <div class="summary">
        <h2>Summary</h2>
        <p><span class="passed">✓ Passed: %d</span></p>
        <p><span class="failed">✗ Failed: %d</span></p>
        <p><strong>Total Tests:</strong> %d</p>
        <p><strong>Success Rate:</strong> %.2f%%</p>
    </div>
</body>
</html>`,
		execution.ID,
		execution.ID,
		execution.StartTime.Format("2006-01-02 15:04:05"),
		execution.Duration,
		execution.Status,
		execution.ExecutionResults.TestsPassed,
		execution.ExecutionResults.TestsFailed,
		execution.ExecutionResults.TotalTests,
		float64(execution.ExecutionResults.TestsPassed)/float64(execution.ExecutionResults.TotalTests)*100,
	)
	
	if err := os.WriteFile(reportPath, []byte(html), 0644); err != nil {
		return "", fmt.Errorf("failed to write HTML report: %w", err)
	}
	
	return reportPath, nil
}

// generateMarkdownReport generates a Markdown report
func (generator *TestReportGenerator) generateMarkdownReport(execution *OrchestratorExecution) (string, error) {
	reportPath := filepath.Join(generator.outputDirectory, fmt.Sprintf("report_%s.md", execution.ID))
	
	markdown := fmt.Sprintf(`# E2E Test Report

## Execution Details
- **ID**: %s
- **Start Time**: %s
- **Duration**: %v
- **Status**: %s

## Summary
- **Total Tests**: %d
- **Passed**: %d ✓
- **Failed**: %d ✗
- **Success Rate**: %.2f%%

## Test Categories
`,
		execution.ID,
		execution.StartTime.Format("2006-01-02 15:04:05"),
		execution.Duration,
		execution.Status,
		execution.ExecutionResults.TotalTests,
		execution.ExecutionResults.TestsPassed,
		execution.ExecutionResults.TestsFailed,
		float64(execution.ExecutionResults.TestsPassed)/float64(execution.ExecutionResults.TotalTests)*100,
	)
	
	// Add category details
	for category, summary := range execution.ExecutionResults.CategoryResults {
		markdown += fmt.Sprintf(`
### %s
- Tests: %d
- Passed: %d
- Failed: %d
- Success Rate: %.2f%%
- Duration: %v
`,
			string(category),
			summary.TotalTests,
			summary.Passed,
			summary.Failed,
			summary.SuccessRate*100,
			summary.Duration,
		)
	}
	
	if err := os.WriteFile(reportPath, []byte(markdown), 0644); err != nil {
		return "", fmt.Errorf("failed to write Markdown report: %w", err)
	}
	
	return reportPath, nil
}

// generateXMLReport generates an XML report
func (generator *TestReportGenerator) generateXMLReport(execution *OrchestratorExecution) (string, error) {
	reportPath := filepath.Join(generator.outputDirectory, fmt.Sprintf("report_%s.xml", execution.ID))
	
	xml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<testexecution id="%s" starttime="%s" duration="%v" status="%s">
    <summary>
        <totaltests>%d</totaltests>
        <passed>%d</passed>
        <failed>%d</failed>
        <successrate>%.2f</successrate>
    </summary>
</testexecution>`,
		execution.ID,
		execution.StartTime.Format("2006-01-02T15:04:05Z"),
		execution.Duration,
		execution.Status,
		execution.ExecutionResults.TotalTests,
		execution.ExecutionResults.TestsPassed,
		execution.ExecutionResults.TestsFailed,
		float64(execution.ExecutionResults.TestsPassed)/float64(execution.ExecutionResults.TotalTests)*100,
	)
	
	if err := os.WriteFile(reportPath, []byte(xml), 0644); err != nil {
		return "", fmt.Errorf("failed to write XML report: %w", err)
	}
	
	return reportPath, nil
}

// generateJUnitReport generates a JUnit-style XML report
func (generator *TestReportGenerator) generateJUnitReport(execution *OrchestratorExecution) (string, error) {
	reportPath := filepath.Join(generator.outputDirectory, fmt.Sprintf("junit_%s.xml", execution.ID))
	
	junit := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="E2E Test Suite" tests="%d" failures="%d" time="%.3f">
`,
		execution.ExecutionResults.TotalTests,
		execution.ExecutionResults.TestsFailed,
		execution.Duration.Seconds(),
	)
	
	// Add test cases (simplified - would iterate through actual test results)
	junit += `</testsuite>`
	
	if err := os.WriteFile(reportPath, []byte(junit), 0644); err != nil {
		return "", fmt.Errorf("failed to write JUnit report: %w", err)
	}
	
	return reportPath, nil
}

// Helper functions

// getDefaultOrchestratorConfig returns default orchestrator configuration
func getDefaultOrchestratorConfig() *OrchestratorConfig {
	return &OrchestratorConfig{
		DiscoveryPackages:   []string{"."},
		TestPatterns:        []string{"Test.*E2E.*", ".*E2ETest$"},
		ExcludePatterns:     []string{".*_test\\.go$", ".*Helper.*", ".*Mock.*"},
		MaxDiscoveryDepth:   5,
		DefaultStrategy:     StrategyOptimized,
		MaxConcurrency:      4,
		DefaultTimeout:      2 * time.Hour,
		EnableRetries:       true,
		MaxRetries:          3,
		MemoryLimitMB:       8192,
		CPULimit:            4,
		NetworkPorts:        []int{8080, 8081, 8082, 8083, 8084},
		GenerateReports:     true,
		ReportFormats:       []ReportFormat{FormatJSON, FormatHTML, FormatMarkdown},
		SaveBaseline:        true,
		CompareBaseline:     false,
		UseRealServers:      false,
		TestDataDirectory:   "testdata",
		CleanupAfterRun:     true,
		EnableMonitoring:    true,
		MonitoringInterval:  30 * time.Second,
		AlertThresholds: &MonitoringThresholds{
			MaxMemoryMB:           6144,
			MaxCPUPercent:         80.0,
			MaxExecutionTime:      2 * time.Hour,
			MaxFailureRate:        0.1,
			MaxResourceWaitTime:   5 * time.Minute,
			MaxConcurrentFailures: 5,
		},
	}
}

// validateOrchestratorConfig validates orchestrator configuration
func validateOrchestratorConfig(config *OrchestratorConfig) error {
	if config.MaxConcurrency <= 0 {
		return fmt.Errorf("max concurrency must be positive")
	}
	if config.DefaultTimeout <= 0 {
		return fmt.Errorf("default timeout must be positive")
	}
	if config.MemoryLimitMB <= 0 {
		return fmt.Errorf("memory limit must be positive")
	}
	if config.CPULimit <= 0 {
		return fmt.Errorf("CPU limit must be positive")
	}
	if len(config.ReportFormats) == 0 && config.GenerateReports {
		return fmt.Errorf("report formats must be specified when generating reports")
	}
	return nil
}