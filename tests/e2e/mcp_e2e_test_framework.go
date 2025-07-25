package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/gateway"
	"lsp-gateway/mcp"
	"lsp-gateway/tests/framework"
	"lsp-gateway/tests/mocks"
)

// E2EScenario defines different E2E test scenarios for MCP integration
type E2EScenario string

const (
	// Basic MCP Integration Scenarios
	E2EScenarioBasicMCPWorkflow        E2EScenario = "basic-mcp-workflow"
	E2EScenarioLSPMethodValidation     E2EScenario = "lsp-method-validation"
	E2EScenarioMultiLanguageSupport    E2EScenario = "multi-language-support"
	E2EScenarioWorkspaceManagement     E2EScenario = "workspace-management"
	
	// Circuit Breaker & Resilience Scenarios
	E2EScenarioCircuitBreakerTesting   E2EScenario = "circuit-breaker-testing"
	E2EScenarioRetryPolicyValidation   E2EScenario = "retry-policy-validation"
	E2EScenarioErrorHandlingWorkflow   E2EScenario = "error-handling-workflow"
	E2EScenarioFailoverRecovery        E2EScenario = "failover-recovery"
	
	// Performance & Load Scenarios
	E2EScenarioPerformanceValidation   E2EScenario = "performance-validation"
	E2EScenarioConcurrentRequests      E2EScenario = "concurrent-requests"
	E2EScenarioLoadBalancingMCP        E2EScenario = "load-balancing-mcp"
	E2EScenarioHighThroughputTesting   E2EScenario = "high-throughput-testing"
	
	// Integration & Real-world Scenarios
	E2EScenarioProjectAnalysisWorkflow E2EScenario = "project-analysis-workflow"
	E2EScenarioCodeNavigationChain     E2EScenario = "code-navigation-chain"
	E2EScenarioFullStackIntegration    E2EScenario = "full-stack-integration"
	E2EScenarioEndToEndDeveloperUse    E2EScenario = "end-to-end-developer-use"
)

// E2ETestConfig defines configuration for E2E test execution
type E2ETestConfig struct {
	TestTimeout       time.Duration
	MaxConcurrency    int
	RetryAttempts     int
	CleanupOnFailure  bool
	DetailedLogging   bool
	MetricsCollection bool
	ProjectTypes      []framework.ProjectType
	Languages         []string
	Scenarios         []E2EScenario
}

// E2ETestResult contains comprehensive results from E2E test execution
type E2ETestResult struct {
	Scenario          E2EScenario
	Success           bool
	Duration          time.Duration
	StartTime         time.Time
	EndTime           time.Time
	
	// Request and Response Metrics
	TotalRequests     int64
	SuccessfulReqs    int64
	FailedRequests    int64
	AverageLatency    time.Duration
	MinLatency        time.Duration
	MaxLatency        time.Duration
	
	// MCP Client Metrics
	MCPMetrics        *mcp.ConnectionMetrics
	CircuitBreaker    *CircuitBreakerTestResult
	RetryPolicy       *RetryPolicyTestResult
	
	// Framework Integration Results
	FrameworkResults  *framework.WorkflowResult
	ProjectResults    []*ProjectTestResult
	
	// Error Details
	Errors            []error
	Warnings          []string
	
	// Performance Data
	MemoryUsageMB     float64
	CPUUsagePercent   float64
	GoroutineCount    int
	
	// Test Environment Details
	TestEnvironment   *TestEnvironmentInfo
	ConfigUsed        *E2ETestConfig
}

// CircuitBreakerTestResult contains circuit breaker specific test results
type CircuitBreakerTestResult struct {
	State                 mcp.CircuitBreakerState
	TriggeredCount        int
	RecoveryTime          time.Duration
	FalsePositiveCount    int
	FalseNegativeCount    int
	StateTransitions      []StateTransitionEvent
	FailureThreshold      float64
	MaxFailures           int
	Timeout               time.Duration
}

// RetryPolicyTestResult contains retry policy specific test results
type RetryPolicyTestResult struct {
	MaxRetries        int
	TotalRetryAttempts int
	SuccessfulRetries int
	FailedRetries     int
	BackoffTimes      []time.Duration
	AverageBackoff    time.Duration
	JitterEnabled     bool
	RetryableErrors   map[mcp.ErrorCategory]int
}

// ProjectTestResult contains project-specific test results
type ProjectTestResult struct {
	Project           *framework.TestProject
	LanguageResults   map[string]*LanguageTestResult
	Success           bool
	Duration          time.Duration
	RequestCount      int
	ErrorCount        int
	Errors            []error
}

// LanguageTestResult contains language-specific test results
type LanguageTestResult struct {
	Language          string
	LSPMethods        map[string]*LSPMethodTestResult
	Success           bool
	AverageLatency    time.Duration
	RequestCount      int
	ErrorCount        int
}

// LSPMethodTestResult contains LSP method specific test results
type LSPMethodTestResult struct {
	Method            string
	Invocations       int
	SuccessCount      int
	FailureCount      int
	AverageLatency    time.Duration
	Errors            []error
}

// StateTransitionEvent captures circuit breaker state transitions
type StateTransitionEvent struct {
	Timestamp     time.Time
	FromState     mcp.CircuitBreakerState
	ToState       mcp.CircuitBreakerState
	TriggerReason string
}

// TestEnvironmentInfo contains information about the test environment
type TestEnvironmentInfo struct {
	TempDirectory     string
	ProjectsCreated   []string
	ServersStarted    []string
	GatewayInstances  []string
	ConfigFiles       []string
	CreatedAt         time.Time
}

// E2ETestRunner is the main orchestrator for MCP E2E integration testing
type E2ETestRunner struct {
	// Core Components
	MockMcpClient     *mocks.MockMcpClient
	TestFramework     *framework.MultiLanguageTestFramework
	Config            *E2ETestConfig
	
	// Test Environment
	TempDir           string
	TestEnvironment   *TestEnvironmentInfo
	
	// State Management
	mu                sync.RWMutex
	ctx               context.Context
	cancel            context.CancelFunc
	
	// Results & Metrics
	TestResults       []*E2ETestResult
	GlobalMetrics     *GlobalTestMetrics
	
	// Logging & Debug
	Logger            *log.Logger
	DebugMode         bool
	
	// Cleanup Management
	CleanupFunctions  []func() error
	
	// Concurrency Control
	semaphore         chan struct{}
	activeTests       int32
}

// GlobalTestMetrics tracks metrics across all E2E tests
type GlobalTestMetrics struct {
	TotalTestsRun        int64
	TotalTestsPassed     int64
	TotalTestsFailed     int64
	TotalDuration        time.Duration
	TotalRequests        int64
	TotalErrors          int64
	AverageTestDuration  time.Duration
	PeakMemoryUsage      int64
	PeakCPUUsage         float64
	PeakGoroutineCount   int
	CircuitBreakerTriggers int64
	RetryAttempts        int64
}

// NewE2ETestRunner creates a new E2E test runner with comprehensive configuration
func NewE2ETestRunner(config *E2ETestConfig) (*E2ETestRunner, error) {
	// Validate configuration
	if config == nil {
		config = getDefaultE2ETestConfig()
	}
	
	if err := validateE2ETestConfig(config); err != nil {
		return nil, fmt.Errorf("invalid E2E test config: %w", err)
	}
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), config.TestTimeout)
	
	// Create temporary directory for test artifacts
	tempDir, err := os.MkdirTemp("", "e2e-mcp-test-*")
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	
	// Initialize logger
	logLevel := log.LstdFlags
	if config.DetailedLogging {
		logLevel |= log.Lshortfile
	}
	logger := log.New(os.Stdout, "[E2E-MCP-Test] ", logLevel)
	
	// Create semaphore for concurrency control
	semaphore := make(chan struct{}, config.MaxConcurrency)
	
	runner := &E2ETestRunner{
		Config:           config,
		TempDir:          tempDir,
		ctx:              ctx,
		cancel:           cancel,
		TestResults:      make([]*E2ETestResult, 0),
		GlobalMetrics:    &GlobalTestMetrics{},
		Logger:           logger,
		DebugMode:        config.DetailedLogging,
		CleanupFunctions: make([]func() error, 0),
		semaphore:        semaphore,
	}
	
	// Initialize test environment info
	runner.TestEnvironment = &TestEnvironmentInfo{
		TempDirectory:    tempDir,
		ProjectsCreated:  make([]string, 0),
		ServersStarted:   make([]string, 0),
		GatewayInstances: make([]string, 0),
		ConfigFiles:      make([]string, 0),
		CreatedAt:        time.Now(),
	}
	
	// Add cleanup for temp directory
	runner.CleanupFunctions = append(runner.CleanupFunctions, func() error {
		return os.RemoveAll(tempDir)
	})
	
	return runner, nil
}

// SetupTestEnvironment initializes the complete E2E test environment
func (r *E2ETestRunner) SetupTestEnvironment(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.Logger.Printf("Setting up E2E test environment in %s", r.TempDir)
	
	// Initialize MockMcpClient
	r.MockMcpClient = mocks.NewMockMcpClient()
	if r.MockMcpClient == nil {
		return fmt.Errorf("failed to create MockMcpClient")
	}
	
	// Configure MockMcpClient with test-specific settings
	r.MockMcpClient.SetBaseURL("http://localhost:8080")
	r.MockMcpClient.SetTimeout(30 * time.Second)
	r.MockMcpClient.SetMaxRetries(3)
	
	// Initialize MultiLanguageTestFramework
	frameworkTimeout := r.Config.TestTimeout
	if frameworkTimeout > 60*time.Second {
		frameworkTimeout = 60 * time.Second // Cap framework timeout
	}
	
	r.TestFramework = framework.NewMultiLanguageTestFramework(frameworkTimeout)
	if r.TestFramework == nil {
		return fmt.Errorf("failed to create MultiLanguageTestFramework")
	}
	
	// Setup test framework environment
	if err := r.TestFramework.SetupTestEnvironment(ctx); err != nil {
		return fmt.Errorf("failed to setup test framework environment: %w", err)
	}
	
	// Add cleanup for test framework
	r.CleanupFunctions = append(r.CleanupFunctions, func() error {
		return r.TestFramework.CleanupAll()
	})
	
	r.Logger.Printf("E2E test environment setup completed successfully")
	return nil
}

// ExecuteScenario executes a specific E2E test scenario
func (r *E2ETestRunner) ExecuteScenario(scenario E2EScenario) (*E2ETestResult, error) {
	// Acquire semaphore for concurrency control
	r.semaphore <- struct{}{}
	defer func() { <-r.semaphore }()
	
	atomic.AddInt32(&r.activeTests, 1)
	defer atomic.AddInt32(&r.activeTests, -1)
	
	startTime := time.Now()
	
	result := &E2ETestResult{
		Scenario:         scenario,
		StartTime:        startTime,
		TestEnvironment:  r.TestEnvironment,
		ConfigUsed:       r.Config,
		Errors:           make([]error, 0),
		Warnings:         make([]string, 0),
		ProjectResults:   make([]*ProjectTestResult, 0),
	}
	
	r.Logger.Printf("Executing E2E scenario: %s", scenario)
	
	// Execute scenario-specific logic
	var err error
	switch scenario {
	case E2EScenarioBasicMCPWorkflow:
		err = r.executeBasicMCPWorkflow(result)
	case E2EScenarioLSPMethodValidation:
		err = r.executeLSPMethodValidation(result)
	case E2EScenarioMultiLanguageSupport:
		err = r.executeMultiLanguageSupport(result)
	case E2EScenarioWorkspaceManagement:
		err = r.executeWorkspaceManagement(result)
	case E2EScenarioCircuitBreakerTesting:
		err = r.executeCircuitBreakerTesting(result)
	case E2EScenarioRetryPolicyValidation:
		err = r.executeRetryPolicyValidation(result)
	case E2EScenarioErrorHandlingWorkflow:
		err = r.executeErrorHandlingWorkflow(result)
	case E2EScenarioFailoverRecovery:
		err = r.executeFailoverRecovery(result)
	case E2EScenarioPerformanceValidation:
		err = r.executePerformanceValidation(result)
	case E2EScenarioConcurrentRequests:
		err = r.executeConcurrentRequests(result)
	case E2EScenarioLoadBalancingMCP:
		err = r.executeLoadBalancingMCP(result)
	case E2EScenarioHighThroughputTesting:
		err = r.executeHighThroughputTesting(result)
	case E2EScenarioProjectAnalysisWorkflow:
		err = r.executeProjectAnalysisWorkflow(result)
	case E2EScenarioCodeNavigationChain:
		err = r.executeCodeNavigationChain(result)
	case E2EScenarioFullStackIntegration:
		err = r.executeFullStackIntegration(result)
	case E2EScenarioEndToEndDeveloperUse:
		err = r.executeEndToEndDeveloperUse(result)
	default:
		err = fmt.Errorf("unsupported E2E scenario: %s", scenario)
	}
	
	// Finalize result
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Success = err == nil && len(result.Errors) == 0
	
	if err != nil {
		result.Errors = append(result.Errors, err)
	}
	
	// Capture MCP client metrics
	if r.MockMcpClient != nil {
		metrics := r.MockMcpClient.GetMetrics()
		result.MCPMetrics = &metrics
	}
	
	// Update global metrics
	r.updateGlobalMetrics(result)
	
	// Add result to test results
	r.mu.Lock()
	r.TestResults = append(r.TestResults, result)
	r.mu.Unlock()
	
	r.Logger.Printf("E2E scenario %s completed: success=%t, duration=%v", 
					scenario, result.Success, result.Duration)
	
	return result, err
}

// ExecuteAllScenarios executes all configured E2E test scenarios
func (r *E2ETestRunner) ExecuteAllScenarios() ([]*E2ETestResult, error) {
	r.Logger.Printf("Executing all E2E scenarios: %d scenarios", len(r.Config.Scenarios))
	
	results := make([]*E2ETestResult, 0, len(r.Config.Scenarios))
	var errors []error
	
	for _, scenario := range r.Config.Scenarios {
		result, err := r.ExecuteScenario(scenario)
		results = append(results, result)
		
		if err != nil {
			errors = append(errors, fmt.Errorf("scenario %s failed: %w", scenario, err))
			
			if r.Config.CleanupOnFailure {
				r.Logger.Printf("Cleaning up after failed scenario: %s", scenario)
				if cleanupErr := r.Cleanup(); cleanupErr != nil {
					r.Logger.Printf("Cleanup error: %v", cleanupErr)
				}
			}
		}
	}
	
	if len(errors) > 0 {
		return results, fmt.Errorf("E2E scenarios failed: %v", errors)
	}
	
	return results, nil
}

// Cleanup performs comprehensive cleanup of all test resources
func (r *E2ETestRunner) Cleanup() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.Logger.Printf("Starting comprehensive E2E test cleanup")
	
	var errors []error
	
	// Execute cleanup functions in reverse order
	for i := len(r.CleanupFunctions) - 1; i >= 0; i-- {
		if err := r.CleanupFunctions[i](); err != nil {
			errors = append(errors, err)
		}
	}
	
	// Reset MockMcpClient
	if r.MockMcpClient != nil {
		r.MockMcpClient.Reset()
	}
	
	// Cancel context
	if r.cancel != nil {
		r.cancel()
	}
	
	r.Logger.Printf("E2E test cleanup completed")
	
	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}
	
	return nil
}

// GetTestResults returns all test results
func (r *E2ETestRunner) GetTestResults() []*E2ETestResult {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	results := make([]*E2ETestResult, len(r.TestResults))
	copy(results, r.TestResults)
	return results
}

// GetGlobalMetrics returns global test metrics
func (r *E2ETestRunner) GetGlobalMetrics() *GlobalTestMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	return &GlobalTestMetrics{
		TotalTestsRun:        r.GlobalMetrics.TotalTestsRun,
		TotalTestsPassed:     r.GlobalMetrics.TotalTestsPassed,
		TotalTestsFailed:     r.GlobalMetrics.TotalTestsFailed,
		TotalDuration:        r.GlobalMetrics.TotalDuration,
		TotalRequests:        r.GlobalMetrics.TotalRequests,
		TotalErrors:          r.GlobalMetrics.TotalErrors,
		AverageTestDuration:  r.GlobalMetrics.AverageTestDuration,
		PeakMemoryUsage:      r.GlobalMetrics.PeakMemoryUsage,
		PeakCPUUsage:         r.GlobalMetrics.PeakCPUUsage,
		PeakGoroutineCount:   r.GlobalMetrics.PeakGoroutineCount,
		CircuitBreakerTriggers: r.GlobalMetrics.CircuitBreakerTriggers,
		RetryAttempts:        r.GlobalMetrics.RetryAttempts,
	}
}

// IsActive returns whether the test runner is still active
func (r *E2ETestRunner) IsActive() bool {
	select {
	case <-r.ctx.Done():
		return false
	default:
		return true
	}
}

// GetActiveTestCount returns the number of currently active tests
func (r *E2ETestRunner) GetActiveTestCount() int {
	return int(atomic.LoadInt32(&r.activeTests))
}

// Helper Methods

// getDefaultE2ETestConfig returns default configuration for E2E tests
func getDefaultE2ETestConfig() *E2ETestConfig {
	return &E2ETestConfig{
		TestTimeout:       30 * time.Minute,
		MaxConcurrency:    5,
		RetryAttempts:     3,
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
		},
	}
}

// validateE2ETestConfig validates E2E test configuration
func validateE2ETestConfig(config *E2ETestConfig) error {
	if config.TestTimeout <= 0 {
		return fmt.Errorf("test timeout must be positive")
	}
	if config.MaxConcurrency <= 0 {
		return fmt.Errorf("max concurrency must be positive")
	}
	if config.RetryAttempts < 0 {
		return fmt.Errorf("retry attempts cannot be negative")
	}
	if len(config.Languages) == 0 {
		return fmt.Errorf("at least one language must be specified")
	}
	if len(config.Scenarios) == 0 {
		return fmt.Errorf("at least one scenario must be specified")
	}
	return nil
}

// updateGlobalMetrics updates global test metrics with result data
func (r *E2ETestRunner) updateGlobalMetrics(result *E2ETestResult) {
	atomic.AddInt64(&r.GlobalMetrics.TotalTestsRun, 1)
	
	if result.Success {
		atomic.AddInt64(&r.GlobalMetrics.TotalTestsPassed, 1)
	} else {
		atomic.AddInt64(&r.GlobalMetrics.TotalTestsFailed, 1)
	}
	
	atomic.AddInt64(&r.GlobalMetrics.TotalRequests, result.TotalRequests)
	atomic.AddInt64(&r.GlobalMetrics.TotalErrors, int64(len(result.Errors)))
	
	// Update average test duration (simplified atomic operation)
	r.mu.Lock()
	r.GlobalMetrics.TotalDuration += result.Duration
	if r.GlobalMetrics.TotalTestsRun > 0 {
		r.GlobalMetrics.AverageTestDuration = time.Duration(
			int64(r.GlobalMetrics.TotalDuration) / r.GlobalMetrics.TotalTestsRun)
	}
	r.mu.Unlock()
}

// LSP Method Workflow Testing Results and Types

// LSPWorkflowResult contains results from LSP workflow testing
type LSPWorkflowResult struct {
	Method            string
	ProjectLanguages  []string
	RequestsExecuted  int
	SuccessfulReqs    int
	FailedRequests    int
	AverageLatency    time.Duration
	MinLatency        time.Duration
	MaxLatency        time.Duration
	ResponseSizes     []int
	ErrorDetails      []error
	StartTime         time.Time
	EndTime           time.Time
}

// LSPRequestParams contains parameters for LSP method requests
type LSPRequestParams struct {
	URI      string                 `json:"uri,omitempty"`
	Position map[string]interface{} `json:"position,omitempty"`
	Query    string                 `json:"query,omitempty"`
	Context  map[string]interface{} `json:"context,omitempty"`
}

// LSPWorkflowScenario defines different LSP workflow testing patterns
type LSPWorkflowScenario string

const (
	LSPWorkflowDefinitionLookup     LSPWorkflowScenario = "definition-lookup"
	LSPWorkflowReferenceFinding     LSPWorkflowScenario = "reference-finding"
	LSPWorkflowHoverInformation     LSPWorkflowScenario = "hover-information"
	LSPWorkflowSymbolSearch         LSPWorkflowScenario = "symbol-search"
	LSPWorkflowCrossLanguage        LSPWorkflowScenario = "cross-language"
	LSPWorkflowConcurrentRequests   LSPWorkflowScenario = "concurrent-requests"
)

// Scenario execution methods (stubs for actual implementation)

func (r *E2ETestRunner) executeBasicMCPWorkflow(result *E2ETestResult) error {
	// Implementation placeholder for basic MCP workflow testing
	return nil
}

func (r *E2ETestRunner) executeLSPMethodValidation(result *E2ETestResult) error {
	ctx, cancel := context.WithTimeout(r.ctx, 10*time.Minute)
	defer cancel()

	r.Logger.Printf("Starting comprehensive LSP method validation")

	// Generate test projects for different languages
	languages := []string{"go", "python", "typescript", "java"}
	project, err := r.TestFramework.CreateMultiLanguageProject(framework.ProjectTypeMultiLanguage, languages)
	if err != nil {
		return fmt.Errorf("failed to create multi-language project: %w", err)
	}

	// Track overall result
	projectResult := &ProjectTestResult{
		Project:         project,
		LanguageResults: make(map[string]*LanguageTestResult),
		Success:         true,
		RequestCount:    0,
		ErrorCount:      0,
		Errors:          make([]error, 0),
	}

	startTime := time.Now()

	// Execute all LSP workflow scenarios in parallel
	var wg sync.WaitGroup
	scenarios := []LSPWorkflowScenario{
		LSPWorkflowDefinitionLookup,
		LSPWorkflowReferenceFinding,
		LSPWorkflowHoverInformation,
		LSPWorkflowSymbolSearch,
		LSPWorkflowCrossLanguage,
		LSPWorkflowConcurrentRequests,
	}

	workflowResults := make(map[LSPWorkflowScenario]*LSPWorkflowResult)
	var resultMutex sync.Mutex

	for _, scenario := range scenarios {
		wg.Add(1)
		go func(s LSPWorkflowScenario) {
			defer wg.Done()
			workflowResult := r.executeSpecificLSPWorkflow(ctx, s, project)
			
			resultMutex.Lock()
			workflowResults[s] = workflowResult
			resultMutex.Unlock()
		}(scenario)
	}

	wg.Wait()

	// Aggregate results
	totalRequests := int64(0)
	totalSuccessful := int64(0)
	totalFailed := int64(0)
	var totalDuration time.Duration

	for scenario, workflowResult := range workflowResults {
		if workflowResult == nil {
			continue
		}

		totalRequests += int64(workflowResult.RequestsExecuted)
		totalSuccessful += int64(workflowResult.SuccessfulReqs)
		totalFailed += int64(workflowResult.FailedRequests)
		totalDuration += workflowResult.EndTime.Sub(workflowResult.StartTime)

		projectResult.RequestCount += workflowResult.RequestsExecuted
		projectResult.ErrorCount += len(workflowResult.ErrorDetails)
		projectResult.Errors = append(projectResult.Errors, workflowResult.ErrorDetails...)

		r.Logger.Printf("LSP workflow %s completed: %d requests, %d successful, %d failed", 
			scenario, workflowResult.RequestsExecuted, workflowResult.SuccessfulReqs, workflowResult.FailedRequests)
	}

	projectResult.Duration = time.Since(startTime)
	projectResult.Success = totalFailed == 0

	// Update result metrics
	result.TotalRequests = totalRequests
	result.SuccessfulReqs = totalSuccessful
	result.FailedRequests = totalFailed
	result.ProjectResults = append(result.ProjectResults, projectResult)

	if totalDuration > 0 {
		result.AverageLatency = time.Duration(int64(totalDuration) / totalRequests)
	}

	r.Logger.Printf("LSP method validation completed: %d total requests, %d successful, %d failed", 
		totalRequests, totalSuccessful, totalFailed)

	if totalFailed > 0 {
		return fmt.Errorf("LSP method validation failed with %d failed requests", totalFailed)
	}

	return nil
}

func (r *E2ETestRunner) executeMultiLanguageSupport(result *E2ETestResult) error {
	// Implementation placeholder for multi-language support testing
	return nil
}

func (r *E2ETestRunner) executeWorkspaceManagement(result *E2ETestResult) error {
	// Implementation placeholder for workspace management testing
	return nil
}

func (r *E2ETestRunner) executeCircuitBreakerTesting(result *E2ETestResult) error {
	// Implementation placeholder for circuit breaker testing
	return nil
}

func (r *E2ETestRunner) executeRetryPolicyValidation(result *E2ETestResult) error {
	// Implementation placeholder for retry policy validation
	return nil
}

func (r *E2ETestRunner) executeErrorHandlingWorkflow(result *E2ETestResult) error {
	// Implementation placeholder for error handling workflow testing
	return nil
}

func (r *E2ETestRunner) executeFailoverRecovery(result *E2ETestResult) error {
	// Implementation placeholder for failover recovery testing
	return nil
}

func (r *E2ETestRunner) executePerformanceValidation(result *E2ETestResult) error {
	// Implementation placeholder for performance validation testing
	return nil
}

func (r *E2ETestRunner) executeConcurrentRequests(result *E2ETestResult) error {
	// Implementation placeholder for concurrent requests testing
	return nil
}

func (r *E2ETestRunner) executeLoadBalancingMCP(result *E2ETestResult) error {
	// Implementation placeholder for load balancing MCP testing
	return nil
}

func (r *E2ETestRunner) executeHighThroughputTesting(result *E2ETestResult) error {
	// Implementation placeholder for high throughput testing
	return nil
}

func (r *E2ETestRunner) executeProjectAnalysisWorkflow(result *E2ETestResult) error {
	// Implementation placeholder for project analysis workflow testing
	return nil
}

func (r *E2ETestRunner) executeCodeNavigationChain(result *E2ETestResult) error {
	// Implementation placeholder for code navigation chain testing
	return nil
}

func (r *E2ETestRunner) executeFullStackIntegration(result *E2ETestResult) error {
	// Implementation placeholder for full stack integration testing
	return nil
}

func (r *E2ETestRunner) executeEndToEndDeveloperUse(result *E2ETestResult) error {
	// Implementation placeholder for end-to-end developer use testing
	return nil
}

// LSP Workflow Execution Methods

// executeSpecificLSPWorkflow executes a specific LSP workflow scenario
func (r *E2ETestRunner) executeSpecificLSPWorkflow(ctx context.Context, scenario LSPWorkflowScenario, project *framework.TestProject) *LSPWorkflowResult {
	startTime := time.Now()
	
	result := &LSPWorkflowResult{
		ProjectLanguages: project.Languages,
		RequestsExecuted: 0,
		SuccessfulReqs:   0,
		FailedRequests:   0,
		ResponseSizes:    make([]int, 0),
		ErrorDetails:     make([]error, 0),
		StartTime:        startTime,
	}

	r.Logger.Printf("Executing LSP workflow scenario: %s", scenario)

	switch scenario {
	case LSPWorkflowDefinitionLookup:
		r.executeDefinitionWorkflow(ctx, r.MockMcpClient, project, result)
	case LSPWorkflowReferenceFinding:
		r.executeReferencesWorkflow(ctx, r.MockMcpClient, project, result)
	case LSPWorkflowHoverInformation:
		r.executeHoverWorkflow(ctx, r.MockMcpClient, project, result)
	case LSPWorkflowSymbolSearch:
		r.executeSymbolsWorkflow(ctx, r.MockMcpClient, project, result)
	case LSPWorkflowCrossLanguage:
		r.executeCrossLanguageWorkflow(ctx, r.MockMcpClient, project, result)
	case LSPWorkflowConcurrentRequests:
		r.executeSimultaneousLSPRequests(ctx, r.MockMcpClient, project, result)
	default:
		result.ErrorDetails = append(result.ErrorDetails, fmt.Errorf("unsupported LSP workflow scenario: %s", scenario))
		result.FailedRequests++
	}

	result.EndTime = time.Now()
	
	// Calculate latency metrics
	if result.RequestsExecuted > 0 {
		totalDuration := result.EndTime.Sub(result.StartTime)
		result.AverageLatency = time.Duration(int64(totalDuration) / int64(result.RequestsExecuted))
	}

	r.Logger.Printf("LSP workflow %s completed: %d requests, %d successful, %d failed, avg latency: %v", 
		scenario, result.RequestsExecuted, result.SuccessfulReqs, result.FailedRequests, result.AverageLatency)

	return result
}

// executeDefinitionWorkflow tests definition lookup workflows across languages
func (r *E2ETestRunner) executeDefinitionWorkflow(ctx context.Context, mockClient *mocks.MockMcpClient, project *framework.TestProject, result *LSPWorkflowResult) {
	result.Method = mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION

	// Setup realistic definition responses for different languages
	r.setupDefinitionResponses(mockClient, project)

	// Test definition lookup scenarios
	definitionScenarios := []struct {
		name     string
		language string
		uri      string
		position map[string]interface{}
	}{
		{
			name:     "Go function definition",
			language: "go",
			uri:      "file://" + filepath.Join(project.RootPath, "main.go"),
			position: map[string]interface{}{"line": 10, "character": 15},
		},
		{
			name:     "Python class definition",
			language: "python",
			uri:      "file://" + filepath.Join(project.RootPath, "main.py"),
			position: map[string]interface{}{"line": 5, "character": 10},
		},
		{
			name:     "TypeScript interface definition",
			language: "typescript",
			uri:      "file://" + filepath.Join(project.RootPath, "main.ts"),
			position: map[string]interface{}{"line": 3, "character": 8},
		},
		{
			name:     "Java method definition",
			language: "java",
			uri:      "file://" + filepath.Join(project.RootPath, "Main.java"),
			position: map[string]interface{}{"line": 12, "character": 20},
		},
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, scenario := range definitionScenarios {
		wg.Add(1)
		go func(s struct {
			name     string
			language string
			uri      string
			position map[string]interface{}
		}) {
			defer wg.Done()

			requestStart := time.Now()
			params := LSPRequestParams{
				URI:      s.uri,
				Position: s.position,
			}

			response, err := mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, params)
			requestDuration := time.Since(requestStart)

			mu.Lock()
			result.RequestsExecuted++
			
			if err != nil {
				result.FailedRequests++
				result.ErrorDetails = append(result.ErrorDetails, fmt.Errorf("definition request failed for %s: %w", s.name, err))
				r.Logger.Printf("Definition lookup failed for %s: %v", s.name, err)
			} else if response != nil {
				result.SuccessfulReqs++
				result.ResponseSizes = append(result.ResponseSizes, len(response))
				
				// Validate response structure
				if err := r.validateDefinitionResponse(response, s.language); err != nil {
					result.ErrorDetails = append(result.ErrorDetails, fmt.Errorf("invalid definition response for %s: %w", s.name, err))
					r.Logger.Printf("Invalid definition response for %s: %v", s.name, err)
				} else {
					r.Logger.Printf("Definition lookup successful for %s (duration: %v)", s.name, requestDuration)
				}
				
				// Update latency tracking
				if result.MinLatency == 0 || requestDuration < result.MinLatency {
					result.MinLatency = requestDuration
				}
				if requestDuration > result.MaxLatency {
					result.MaxLatency = requestDuration
				}
			} else {
				result.FailedRequests++
				result.ErrorDetails = append(result.ErrorDetails, fmt.Errorf("empty definition response for %s", s.name))
			}
			mu.Unlock()
		}(scenario)
	}

	wg.Wait()
}

// executeReferencesWorkflow tests reference finding workflows
func (r *E2ETestRunner) executeReferencesWorkflow(ctx context.Context, mockClient *mocks.MockMcpClient, project *framework.TestProject, result *LSPWorkflowResult) {
	result.Method = mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES

	// Setup realistic references responses
	r.setupReferencesResponses(mockClient, project)

	// Test references finding scenarios
	referencesScenarios := []struct {
		name          string
		language      string
		uri           string
		position      map[string]interface{}
		includeDecl   bool
		expectedMin   int
	}{
		{
			name:        "Go function references",
			language:    "go",
			uri:         "file://" + filepath.Join(project.RootPath, "main.go"),
			position:    map[string]interface{}{"line": 10, "character": 15},
			includeDecl: true,
			expectedMin: 2,
		},
		{
			name:        "Python class references",
			language:    "python", 
			uri:         "file://" + filepath.Join(project.RootPath, "main.py"),
			position:    map[string]interface{}{"line": 20, "character": 10},
			includeDecl: false,
			expectedMin: 1,
		},
		{
			name:        "TypeScript variable references",
			language:    "typescript",
			uri:         "file://" + filepath.Join(project.RootPath, "main.ts"),
			position:    map[string]interface{}{"line": 15, "character": 8},
			includeDecl: true,
			expectedMin: 3,
		},
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, scenario := range referencesScenarios {
		wg.Add(1)
		go func(s struct {
			name          string
			language      string
			uri           string
			position      map[string]interface{}
			includeDecl   bool
			expectedMin   int
		}) {
			defer wg.Done()

			requestStart := time.Now()
			params := LSPRequestParams{
				URI:      s.uri,
				Position: s.position,
				Context: map[string]interface{}{
					"includeDeclaration": s.includeDecl,
				},
			}

			response, err := mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES, params)
			requestDuration := time.Since(requestStart)

			mu.Lock()
			result.RequestsExecuted++
			
			if err != nil {
				result.FailedRequests++
				result.ErrorDetails = append(result.ErrorDetails, fmt.Errorf("references request failed for %s: %w", s.name, err))
			} else if response != nil {
				result.SuccessfulReqs++
				result.ResponseSizes = append(result.ResponseSizes, len(response))
				
				// Validate response structure and count
				if err := r.validateReferencesResponse(response, s.language, s.expectedMin); err != nil {
					result.ErrorDetails = append(result.ErrorDetails, fmt.Errorf("invalid references response for %s: %w", s.name, err))
				}
				
				// Update latency tracking
				if result.MinLatency == 0 || requestDuration < result.MinLatency {
					result.MinLatency = requestDuration
				}
				if requestDuration > result.MaxLatency {
					result.MaxLatency = requestDuration
				}
			} else {
				result.FailedRequests++
				result.ErrorDetails = append(result.ErrorDetails, fmt.Errorf("empty references response for %s", s.name))
			}
			mu.Unlock()
		}(scenario)
	}

	wg.Wait()
}

// executeHoverWorkflow tests hover information workflows
func (r *E2ETestRunner) executeHoverWorkflow(ctx context.Context, mockClient *mocks.MockMcpClient, project *framework.TestProject, result *LSPWorkflowResult) {
	result.Method = mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER

	// Setup realistic hover responses
	r.setupHoverResponses(mockClient, project)

	// Test hover scenarios for different symbol types
	hoverScenarios := []struct {
		name       string
		language   string
		uri        string
		position   map[string]interface{}
		symbolType string
	}{
		{
			name:       "Go function hover",
			language:   "go",
			uri:        "file://" + filepath.Join(project.RootPath, "main.go"),
			position:   map[string]interface{}{"line": 8, "character": 12},
			symbolType: "function",
		},
		{
			name:       "Python class hover",
			language:   "python",
			uri:        "file://" + filepath.Join(project.RootPath, "main.py"),
			position:   map[string]interface{}{"line": 15, "character": 6},
			symbolType: "class",
		},
		{
			name:       "TypeScript interface hover",
			language:   "typescript",
			uri:        "file://" + filepath.Join(project.RootPath, "main.ts"),
			position:   map[string]interface{}{"line": 3, "character": 10},
			symbolType: "interface",
		},
		{
			name:       "Java method hover",
			language:   "java",
			uri:        "file://" + filepath.Join(project.RootPath, "Main.java"),
			position:   map[string]interface{}{"line": 12, "character": 18},
			symbolType: "method",
		},
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, scenario := range hoverScenarios {
		wg.Add(1)
		go func(s struct {
			name       string
			language   string
			uri        string
			position   map[string]interface{}
			symbolType string
		}) {
			defer wg.Done()

			requestStart := time.Now()
			params := LSPRequestParams{
				URI:      s.uri,
				Position: s.position,
			}

			response, err := mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, params)
			requestDuration := time.Since(requestStart)

			mu.Lock()
			result.RequestsExecuted++
			
			if err != nil {
				result.FailedRequests++
				result.ErrorDetails = append(result.ErrorDetails, fmt.Errorf("hover request failed for %s: %w", s.name, err))
			} else if response != nil {
				result.SuccessfulReqs++
				result.ResponseSizes = append(result.ResponseSizes, len(response))
				
				// Validate hover response content
				if err := r.validateHoverResponse(response, s.language, s.symbolType); err != nil {
					result.ErrorDetails = append(result.ErrorDetails, fmt.Errorf("invalid hover response for %s: %w", s.name, err))
				}
				
				// Update latency tracking
				if result.MinLatency == 0 || requestDuration < result.MinLatency {
					result.MinLatency = requestDuration
				}
				if requestDuration > result.MaxLatency {
					result.MaxLatency = requestDuration
				}
			} else {
				result.FailedRequests++
				result.ErrorDetails = append(result.ErrorDetails, fmt.Errorf("empty hover response for %s", s.name))
			}
			mu.Unlock()
		}(scenario)
	}

	wg.Wait()
}

// executeSymbolsWorkflow tests symbol search workflows
func (r *E2ETestRunner) executeSymbolsWorkflow(ctx context.Context, mockClient *mocks.MockMcpClient, project *framework.TestProject, result *LSPWorkflowResult) {
	result.Method = mcp.LSP_METHOD_WORKSPACE_SYMBOL

	// Setup realistic symbol responses
	r.setupSymbolsResponses(mockClient, project)

	// Test workspace symbol search scenarios
	symbolScenarios := []struct {
		name         string
		query        string
		expectedMin  int
		symbolTypes  []string
	}{
		{
			name:        "Function search",
			query:       "main",
			expectedMin: 2,
			symbolTypes: []string{"function", "method"},
		},
		{
			name:        "Class search",
			query:       "User",
			expectedMin: 1,
			symbolTypes: []string{"class", "interface"},
		},
		{
			name:        "Variable search",
			query:       "config",
			expectedMin: 1,
			symbolTypes: []string{"variable", "constant"},
		},
		{
			name:        "Wildcard search",
			query:       "*Handler",
			expectedMin: 3,
			symbolTypes: []string{"class", "function", "method"},
		},
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, scenario := range symbolScenarios {
		wg.Add(1)
		go func(s struct {
			name         string
			query        string
			expectedMin  int
			symbolTypes  []string
		}) {
			defer wg.Done()

			requestStart := time.Now()
			params := LSPRequestParams{
				Query: s.query,
			}

			response, err := mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_WORKSPACE_SYMBOL, params)
			requestDuration := time.Since(requestStart)

			mu.Lock()
			result.RequestsExecuted++
			
			if err != nil {
				result.FailedRequests++
				result.ErrorDetails = append(result.ErrorDetails, fmt.Errorf("symbol search failed for %s: %w", s.name, err))
			} else if response != nil {
				result.SuccessfulReqs++
				result.ResponseSizes = append(result.ResponseSizes, len(response))
				
				// Validate symbol response
				if err := r.validateSymbolsResponse(response, s.expectedMin, s.symbolTypes); err != nil {
					result.ErrorDetails = append(result.ErrorDetails, fmt.Errorf("invalid symbols response for %s: %w", s.name, err))
				}
				
				// Update latency tracking
				if result.MinLatency == 0 || requestDuration < result.MinLatency {
					result.MinLatency = requestDuration
				}
				if requestDuration > result.MaxLatency {
					result.MaxLatency = requestDuration
				}
			} else {
				result.FailedRequests++
				result.ErrorDetails = append(result.ErrorDetails, fmt.Errorf("empty symbols response for %s", s.name))
			}
			mu.Unlock()
		}(scenario)
	}

	wg.Wait()

	// Also test document symbols for each language
	for _, language := range project.Languages {
		wg.Add(1)
		go func(lang string) {
			defer wg.Done()

			var uri string
			switch lang {
			case "go":
				uri = "file://" + filepath.Join(project.RootPath, "main.go")
			case "python":
				uri = "file://" + filepath.Join(project.RootPath, "main.py")
			case "typescript":
				uri = "file://" + filepath.Join(project.RootPath, "main.ts")
			case "java":
				uri = "file://" + filepath.Join(project.RootPath, "Main.java")
			default:
				return
			}

			requestStart := time.Now()
			params := LSPRequestParams{
				URI: uri,
			}

			response, err := mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS, params)
			requestDuration := time.Since(requestStart)

			mu.Lock()
			result.RequestsExecuted++
			
			if err != nil {
				result.FailedRequests++
				result.ErrorDetails = append(result.ErrorDetails, fmt.Errorf("document symbols failed for %s: %w", lang, err))
			} else if response != nil {
				result.SuccessfulReqs++
				result.ResponseSizes = append(result.ResponseSizes, len(response))
				
				// Update latency tracking
				if result.MinLatency == 0 || requestDuration < result.MinLatency {
					result.MinLatency = requestDuration
				}
				if requestDuration > result.MaxLatency {
					result.MaxLatency = requestDuration
				}
			} else {
				result.FailedRequests++
				result.ErrorDetails = append(result.ErrorDetails, fmt.Errorf("empty document symbols response for %s", lang))
			}
			mu.Unlock()
		}(language)
	}

	wg.Wait()
}

// executeCrossLanguageWorkflow tests cross-language navigation scenarios
func (r *E2ETestRunner) executeCrossLanguageWorkflow(ctx context.Context, mockClient *mocks.MockMcpClient, project *framework.TestProject, result *LSPWorkflowResult) {
	result.Method = "cross-language-workflow"

	// Setup cross-language responses
	r.setupCrossLanguageResponses(mockClient, project)

	// Test cross-language scenarios
	crossLangScenarios := []struct {
		name           string
		sourceLanguage string
		targetLanguage string
		method         string
		description    string
	}{
		{
			name:           "Go to Python FFI",
			sourceLanguage: "go",
			targetLanguage: "python",
			method:         mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION,
			description:    "Go calling Python function via FFI",
		},
		{
			name:           "TypeScript to Java API",
			sourceLanguage: "typescript",
			targetLanguage: "java",
			method:         mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES,
			description:    "TypeScript calling Java REST API",
		},
		{
			name:           "Python to Go module",
			sourceLanguage: "python",
			targetLanguage: "go",
			method:         mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER,
			description:    "Python importing Go compiled module",
		},
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, scenario := range crossLangScenarios {
		wg.Add(1)
		go func(s struct {
			name           string
			sourceLanguage string
			targetLanguage string
			method         string
			description    string
		}) {
			defer wg.Done()

			requestStart := time.Now()
			
			// Create realistic cross-language request
			var sourceURI string
			switch s.sourceLanguage {
			case "go":
				sourceURI = "file://" + filepath.Join(project.RootPath, "main.go")
			case "python":
				sourceURI = "file://" + filepath.Join(project.RootPath, "main.py")
			case "typescript":
				sourceURI = "file://" + filepath.Join(project.RootPath, "main.ts")
			case "java":
				sourceURI = "file://" + filepath.Join(project.RootPath, "Main.java")
			}

			params := LSPRequestParams{
				URI:      sourceURI,
				Position: map[string]interface{}{"line": 25, "character": 10},
				Context: map[string]interface{}{
					"targetLanguage": s.targetLanguage,
					"crossLanguage":  true,
				},
			}

			response, err := mockClient.SendLSPRequest(ctx, s.method, params)
			requestDuration := time.Since(requestStart)

			mu.Lock()
			result.RequestsExecuted++
			
			if err != nil {
				result.FailedRequests++
				result.ErrorDetails = append(result.ErrorDetails, fmt.Errorf("cross-language request failed for %s: %w", s.name, err))
			} else if response != nil {
				result.SuccessfulReqs++
				result.ResponseSizes = append(result.ResponseSizes, len(response))
				
				r.Logger.Printf("Cross-language workflow successful: %s -> %s (%s)", 
					s.sourceLanguage, s.targetLanguage, s.description)
				
				// Update latency tracking
				if result.MinLatency == 0 || requestDuration < result.MinLatency {
					result.MinLatency = requestDuration
				}
				if requestDuration > result.MaxLatency {
					result.MaxLatency = requestDuration
				}
			} else {
				result.FailedRequests++
				result.ErrorDetails = append(result.ErrorDetails, fmt.Errorf("empty cross-language response for %s", s.name))
			}
			mu.Unlock()
		}(scenario)
	}

	wg.Wait()
}

// executeSimultaneousLSPRequests tests concurrent LSP method execution
func (r *E2ETestRunner) executeSimultaneousLSPRequests(ctx context.Context, mockClient *mocks.MockMcpClient, project *framework.TestProject, result *LSPWorkflowResult) {
	result.Method = "concurrent-requests"

	// Setup responses for concurrent testing
	r.setupConcurrentTestResponses(mockClient, project)

	// Define concurrent request scenarios
	concurrentScenarios := []struct {
		method      string
		count       int
		params      LSPRequestParams
		description string
	}{
		{
			method: mcp.LSP_METHOD_WORKSPACE_SYMBOL,
			count:  10,
			params: LSPRequestParams{Query: "test"},
			description: "10 concurrent workspace symbol searches",
		},
		{
			method: mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER,
			count:  15,
			params: LSPRequestParams{
				URI:      "file://" + filepath.Join(project.RootPath, "main.go"),
				Position: map[string]interface{}{"line": 5, "character": 10},
			},
			description: "15 concurrent hover requests",
		},
		{
			method: mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION,
			count:  8,
			params: LSPRequestParams{
				URI:      "file://" + filepath.Join(project.RootPath, "main.py"),
				Position: map[string]interface{}{"line": 12, "character": 8},
			},
			description: "8 concurrent definition lookups",
		},
		{
			method: mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES,
			count:  12,
			params: LSPRequestParams{
				URI:      "file://" + filepath.Join(project.RootPath, "main.ts"),
				Position: map[string]interface{}{"line": 18, "character": 15},
				Context:  map[string]interface{}{"includeDeclaration": true},
			},
			description: "12 concurrent reference searches",
		},
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Execute all concurrent scenarios in parallel
	for _, scenario := range concurrentScenarios {
		wg.Add(1)
		go func(s struct {
			method      string
			count       int
			params      LSPRequestParams
			description string
		}) {
			defer wg.Done()

			r.Logger.Printf("Starting concurrent test: %s", s.description)
			
			// Launch concurrent requests for this scenario
			var scenarioWg sync.WaitGroup
			startTime := time.Now()

			for i := 0; i < s.count; i++ {
				scenarioWg.Add(1)
				go func(requestID int) {
					defer scenarioWg.Done()

					requestStart := time.Now()
					response, err := mockClient.SendLSPRequest(ctx, s.method, s.params)
					requestDuration := time.Since(requestStart)

					mu.Lock()
					result.RequestsExecuted++
					
					if err != nil {
						result.FailedRequests++
						result.ErrorDetails = append(result.ErrorDetails, 
							fmt.Errorf("concurrent request %d failed for %s: %w", requestID, s.method, err))
					} else if response != nil {
						result.SuccessfulReqs++
						result.ResponseSizes = append(result.ResponseSizes, len(response))
						
						// Update latency tracking
						if result.MinLatency == 0 || requestDuration < result.MinLatency {
							result.MinLatency = requestDuration
						}
						if requestDuration > result.MaxLatency {
							result.MaxLatency = requestDuration
						}
					} else {
						result.FailedRequests++
						result.ErrorDetails = append(result.ErrorDetails, 
							fmt.Errorf("empty response for concurrent request %d (%s)", requestID, s.method))
					}
					mu.Unlock()
				}(i)
			}

			scenarioWg.Wait()
			scenarioDuration := time.Since(startTime)
			
			r.Logger.Printf("Concurrent test completed: %s (duration: %v)", s.description, scenarioDuration)
		}(scenario)
	}

	wg.Wait()

	// Test mixed concurrent requests (different methods simultaneously)
	r.Logger.Printf("Starting mixed concurrent LSP requests test")
	
	mixedMethods := []string{
		mcp.LSP_METHOD_WORKSPACE_SYMBOL,
		mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION,
		mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES,
		mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER,
		mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS,
	}

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(requestID int) {
			defer wg.Done()

			// Select random method and parameters
			method := mixedMethods[requestID%len(mixedMethods)]
			
			var params LSPRequestParams
			switch method {
			case mcp.LSP_METHOD_WORKSPACE_SYMBOL:
				params = LSPRequestParams{Query: fmt.Sprintf("test%d", requestID)}
			default:
				params = LSPRequestParams{
					URI:      "file://" + filepath.Join(project.RootPath, "main.go"),
					Position: map[string]interface{}{"line": requestID % 20, "character": 10},
				}
			}

			requestStart := time.Now()
			response, err := mockClient.SendLSPRequest(ctx, method, params)
			requestDuration := time.Since(requestStart)

			mu.Lock()
			result.RequestsExecuted++
			
			if err != nil {
				result.FailedRequests++
				result.ErrorDetails = append(result.ErrorDetails, 
					fmt.Errorf("mixed concurrent request %d (%s) failed: %w", requestID, method, err))
			} else if response != nil {
				result.SuccessfulReqs++
				result.ResponseSizes = append(result.ResponseSizes, len(response))
				
				// Update latency tracking
				if result.MinLatency == 0 || requestDuration < result.MinLatency {
					result.MinLatency = requestDuration
				}
				if requestDuration > result.MaxLatency {
					result.MaxLatency = requestDuration
				}
			} else {
				result.FailedRequests++
				result.ErrorDetails = append(result.ErrorDetails, 
					fmt.Errorf("empty response for mixed concurrent request %d (%s)", requestID, method))
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()
}

// Helper Methods for LSP Response Setup and Validation

// setupDefinitionResponses configures realistic definition responses for different languages
func (r *E2ETestRunner) setupDefinitionResponses(mockClient *mocks.MockMcpClient, project *framework.TestProject) {
	// Go definition response
	goDefinition := json.RawMessage(`{
		"uri": "file://` + filepath.Join(project.RootPath, "main.go") + `",
		"range": {
			"start": {"line": 10, "character": 15},
			"end": {"line": 10, "character": 25}
		}
	}`)
	mockClient.QueueResponse(goDefinition)

	// Python definition response
	pythonDefinition := json.RawMessage(`{
		"uri": "file://` + filepath.Join(project.RootPath, "main.py") + `",
		"range": {
			"start": {"line": 5, "character": 10},
			"end": {"line": 5, "character": 20}
		}
	}`)
	mockClient.QueueResponse(pythonDefinition)

	// TypeScript definition response
	tsDefinition := json.RawMessage(`{
		"uri": "file://` + filepath.Join(project.RootPath, "main.ts") + `",
		"range": {
			"start": {"line": 3, "character": 8},
			"end": {"line": 3, "character": 18}
		}
	}`)
	mockClient.QueueResponse(tsDefinition)

	// Java definition response
	javaDefinition := json.RawMessage(`{
		"uri": "file://` + filepath.Join(project.RootPath, "Main.java") + `",
		"range": {
			"start": {"line": 12, "character": 20},
			"end": {"line": 12, "character": 30}
		}
	}`)
	mockClient.QueueResponse(javaDefinition)
}

// setupReferencesResponses configures realistic reference responses
func (r *E2ETestRunner) setupReferencesResponses(mockClient *mocks.MockMcpClient, project *framework.TestProject) {
	// Go references response
	goReferences := json.RawMessage(`[
		{
			"uri": "file://` + filepath.Join(project.RootPath, "main.go") + `",
			"range": {
				"start": {"line": 10, "character": 15},
				"end": {"line": 10, "character": 25}
			}
		},
		{
			"uri": "file://` + filepath.Join(project.RootPath, "utils.go") + `",
			"range": {
				"start": {"line": 8, "character": 5},
				"end": {"line": 8, "character": 15}
			}
		}
	]`)
	mockClient.QueueResponse(goReferences)

	// Python references response
	pythonReferences := json.RawMessage(`[
		{
			"uri": "file://` + filepath.Join(project.RootPath, "main.py") + `",
			"range": {
				"start": {"line": 20, "character": 10},
				"end": {"line": 20, "character": 20}
			}
		}
	]`)
	mockClient.QueueResponse(pythonReferences)

	// TypeScript references response
	tsReferences := json.RawMessage(`[
		{
			"uri": "file://` + filepath.Join(project.RootPath, "main.ts") + `",
			"range": {
				"start": {"line": 15, "character": 8},
				"end": {"line": 15, "character": 18}
			}
		},
		{
			"uri": "file://` + filepath.Join(project.RootPath, "types.ts") + `",
			"range": {
				"start": {"line": 5, "character": 12},
				"end": {"line": 5, "character": 22}
			}
		},
		{
			"uri": "file://` + filepath.Join(project.RootPath, "interfaces.ts") + `",
			"range": {
				"start": {"line": 3, "character": 0},
				"end": {"line": 3, "character": 10}
			}
		}
	]`)
	mockClient.QueueResponse(tsReferences)
}

// setupHoverResponses configures realistic hover responses
func (r *E2ETestRunner) setupHoverResponses(mockClient *mocks.MockMcpClient, project *framework.TestProject) {
	// Go hover response
	goHover := json.RawMessage(`{
		"contents": {
			"kind": "markdown",
			"value": "` + "```go\\nfunc main()\\n```\\n\\nMain function that starts the application." + `"
		},
		"range": {
			"start": {"line": 8, "character": 12},
			"end": {"line": 8, "character": 16}
		}
	}`)
	mockClient.QueueResponse(goHover)

	// Python hover response
	pythonHover := json.RawMessage(`{
		"contents": {
			"kind": "markdown",
			"value": "` + "```python\\nclass UserManager:\\n```\\n\\nManages user operations and authentication." + `"
		},
		"range": {
			"start": {"line": 15, "character": 6},
			"end": {"line": 15, "character": 17}
		}
	}`)
	mockClient.QueueResponse(pythonHover)

	// TypeScript hover response
	tsHover := json.RawMessage(`{
		"contents": {
			"kind": "markdown",
			"value": "` + "```typescript\\ninterface ApiResponse<T>\\n```\\n\\nGeneric interface for API responses." + `"
		},
		"range": {
			"start": {"line": 3, "character": 10},
			"end": {"line": 3, "character": 21}
		}
	}`)
	mockClient.QueueResponse(tsHover)

	// Java hover response
	javaHover := json.RawMessage(`{
		"contents": {
			"kind": "markdown",
			"value": "` + "```java\\npublic void processData()\\n```\\n\\nProcesses incoming data and validates format." + `"
		},
		"range": {
			"start": {"line": 12, "character": 18},
			"end": {"line": 12, "character": 29}
		}
	}`)
	mockClient.QueueResponse(javaHover)
}

// setupSymbolsResponses configures realistic symbol search responses
func (r *E2ETestRunner) setupSymbolsResponses(mockClient *mocks.MockMcpClient, project *framework.TestProject) {
	// Workspace symbols response
	workspaceSymbols := json.RawMessage(`[
		{
			"name": "main",
			"kind": 12,
			"location": {
				"uri": "file://` + filepath.Join(project.RootPath, "main.go") + `",
				"range": {"start": {"line": 8, "character": 5}, "end": {"line": 8, "character": 9}}
			},
			"containerName": ""
		},
		{
			"name": "main",
			"kind": 12,
			"location": {
				"uri": "file://` + filepath.Join(project.RootPath, "main.py") + `",
				"range": {"start": {"line": 50, "character": 4}, "end": {"line": 50, "character": 8}}
			},
			"containerName": "__main__"
		},
		{
			"name": "UserService",
			"kind": 5,
			"location": {
				"uri": "file://` + filepath.Join(project.RootPath, "main.py") + `",
				"range": {"start": {"line": 10, "character": 6}, "end": {"line": 10, "character": 17}}
			},
			"containerName": ""
		},
		{
			"name": "ApiClient",
			"kind": 5,
			"location": {
				"uri": "file://` + filepath.Join(project.RootPath, "main.ts") + `",
				"range": {"start": {"line": 5, "character": 13}, "end": {"line": 5, "character": 22}}
			},
			"containerName": ""
		},
		{
			"name": "config",
			"kind": 13,
			"location": {
				"uri": "file://` + filepath.Join(project.RootPath, "main.go") + `",
				"range": {"start": {"line": 3, "character": 4}, "end": {"line": 3, "character": 10}}
			},
			"containerName": ""
		},
		{
			"name": "ConfigHandler",
			"kind": 5,
			"location": {
				"uri": "file://` + filepath.Join(project.RootPath, "main.go") + `",
				"range": {"start": {"line": 20, "character": 5}, "end": {"line": 20, "character": 18}}
			},
			"containerName": ""
		},
		{
			"name": "DataHandler",
			"kind": 5,
			"location": {
				"uri": "file://` + filepath.Join(project.RootPath, "Main.java") + `",
				"range": {"start": {"line": 15, "character": 13}, "end": {"line": 15, "character": 24}}
			},
			"containerName": ""
		},
		{
			"name": "EventHandler",
			"kind": 12,
			"location": {
				"uri": "file://` + filepath.Join(project.RootPath, "main.ts") + `",
				"range": {"start": {"line": 25, "character": 9}, "end": {"line": 25, "character": 21}}
			},
			"containerName": "EventManager"
		}
	]`)
	mockClient.QueueResponse(workspaceSymbols)

	// Document symbols responses for each language
	goDocSymbols := json.RawMessage(`[
		{
			"name": "main",
			"kind": 12,
			"range": {"start": {"line": 8, "character": 5}, "end": {"line": 15, "character": 1}},
			"selectionRange": {"start": {"line": 8, "character": 5}, "end": {"line": 8, "character": 9}}
		},
		{
			"name": "Config",
			"kind": 23,
			"range": {"start": {"line": 3, "character": 5}, "end": {"line": 7, "character": 1}},
			"selectionRange": {"start": {"line": 3, "character": 12}, "end": {"line": 3, "character": 18}}
		}
	]`)
	mockClient.QueueResponse(goDocSymbols)

	pythonDocSymbols := json.RawMessage(`[
		{
			"name": "UserService",
			"kind": 5,
			"range": {"start": {"line": 5, "character": 0}, "end": {"line": 25, "character": 0}},
			"selectionRange": {"start": {"line": 5, "character": 6}, "end": {"line": 5, "character": 17}}
		},
		{
			"name": "__init__",
			"kind": 9,
			"range": {"start": {"line": 7, "character": 4}, "end": {"line": 10, "character": 0}},
			"selectionRange": {"start": {"line": 7, "character": 8}, "end": {"line": 7, "character": 16}}
		}
	]`)
	mockClient.QueueResponse(pythonDocSymbols)

	tsDocSymbols := json.RawMessage(`[
		{
			"name": "ApiClient",
			"kind": 5,
			"range": {"start": {"line": 3, "character": 0}, "end": {"line": 20, "character": 1}},
			"selectionRange": {"start": {"line": 3, "character": 13}, "end": {"line": 3, "character": 22}}
		},
		{
			"name": "sendRequest",
			"kind": 6,
			"range": {"start": {"line": 8, "character": 2}, "end": {"line": 15, "character": 3}},
			"selectionRange": {"start": {"line": 8, "character": 8}, "end": {"line": 8, "character": 19}}
		}
	]`)
	mockClient.QueueResponse(tsDocSymbols)

	javaDocSymbols := json.RawMessage(`[
		{
			"name": "Main",
			"kind": 5,
			"range": {"start": {"line": 5, "character": 0}, "end": {"line": 30, "character": 1}},
			"selectionRange": {"start": {"line": 5, "character": 13}, "end": {"line": 5, "character": 17}}
		},
		{
			"name": "processData",
			"kind": 6,
			"range": {"start": {"line": 12, "character": 4}, "end": {"line": 18, "character": 5}},
			"selectionRange": {"start": {"line": 12, "character": 16}, "end": {"line": 12, "character": 27}}
		}
	]`)
	mockClient.QueueResponse(javaDocSymbols)
}

// setupCrossLanguageResponses configures responses for cross-language scenarios
func (r *E2ETestRunner) setupCrossLanguageResponses(mockClient *mocks.MockMcpClient, project *framework.TestProject) {
	// Cross-language definition (Go to Python FFI)
	crossLangDefinition := json.RawMessage(`{
		"uri": "file://` + filepath.Join(project.RootPath, "ffi", "python_module.py") + `",
		"range": {
			"start": {"line": 15, "character": 4},
			"end": {"line": 15, "character": 20}
		}
	}`)
	mockClient.QueueResponse(crossLangDefinition)

	// Cross-language references (TypeScript to Java API)
	crossLangReferences := json.RawMessage(`[
		{
			"uri": "file://` + filepath.Join(project.RootPath, "api", "ApiController.java") + `",
			"range": {
				"start": {"line": 25, "character": 16},
				"end": {"line": 25, "character": 32}
			}
		},
		{
			"uri": "file://` + filepath.Join(project.RootPath, "client", "api-client.ts") + `",
			"range": {
				"start": {"line": 12, "character": 8},
				"end": {"line": 12, "character": 24}
			}
		}
	]`)
	mockClient.QueueResponse(crossLangReferences)

	// Cross-language hover (Python importing Go module)
	goModulePath := filepath.Join(project.RootPath, "go_module.go")
	crossLangHover := json.RawMessage(`{
		"contents": {
			"kind": "markdown",
			"value": "` + "```python\\nimport go_module\\n```\\n\\nImported Go module compiled to shared library.\\n\\n**Go Source**: " + goModulePath + `"
		},
		"range": {
			"start": {"line": 25, "character": 10},
			"end": {"line": 25, "character": 19}
		}
	}`)
	mockClient.QueueResponse(crossLangHover)
}

// setupConcurrentTestResponses prepares responses for concurrent testing
func (r *E2ETestRunner) setupConcurrentTestResponses(mockClient *mocks.MockMcpClient, project *framework.TestProject) {
	// Queue multiple responses for concurrent requests
	responses := []json.RawMessage{
		json.RawMessage(`{"symbols": [{"name": "ConcurrentSymbol1", "kind": 12}]}`),
		json.RawMessage(`{"symbols": [{"name": "ConcurrentSymbol2", "kind": 5}]}`),
		json.RawMessage(`{"symbols": [{"name": "ConcurrentSymbol3", "kind": 6}]}`),
		json.RawMessage(`{"contents": {"kind": "markdown", "value": "Concurrent hover 1"}}`),
		json.RawMessage(`{"contents": {"kind": "markdown", "value": "Concurrent hover 2"}}`),
		json.RawMessage(`{"contents": {"kind": "markdown", "value": "Concurrent hover 3"}}`),
		json.RawMessage(`{"uri": "file://test1.go", "range": {"start": {"line": 1, "character": 1}, "end": {"line": 1, "character": 10}}}`),
		json.RawMessage(`{"uri": "file://test2.py", "range": {"start": {"line": 2, "character": 2}, "end": {"line": 2, "character": 20}}}`),
		json.RawMessage(`[{"uri": "file://ref1.ts", "range": {"start": {"line": 3, "character": 3}, "end": {"line": 3, "character": 30}}}]`),
		json.RawMessage(`[{"uri": "file://ref2.java", "range": {"start": {"line": 4, "character": 4}, "end": {"line": 4, "character": 40}}}]`),
	}

	// Queue responses multiple times for concurrent testing
	for i := 0; i < 100; i++ {
		for _, response := range responses {
			mockClient.QueueResponse(response)
		}
	}
}

// Validation Methods

// validateDefinitionResponse validates the structure of definition responses
func (r *E2ETestRunner) validateDefinitionResponse(response json.RawMessage, language string) error {
	var definition map[string]interface{}
	if err := json.Unmarshal(response, &definition); err != nil {
		return fmt.Errorf("failed to unmarshal definition response: %w", err)
	}

	// Validate required fields
	if _, ok := definition["uri"]; !ok {
		return fmt.Errorf("definition response missing 'uri' field")
	}

	if _, ok := definition["range"]; !ok {
		return fmt.Errorf("definition response missing 'range' field")
	}

	// Validate range structure
	if rangeObj, ok := definition["range"].(map[string]interface{}); ok {
		if _, ok := rangeObj["start"]; !ok {
			return fmt.Errorf("definition range missing 'start' field")
		}
		if _, ok := rangeObj["end"]; !ok {
			return fmt.Errorf("definition range missing 'end' field")
		}
	} else {
		return fmt.Errorf("definition range is not a valid object")
	}

	// Language-specific validation
	uri, _ := definition["uri"].(string)
	switch language {
	case "go":
		if filepath.Ext(uri) != ".go" && !filepath.Contains(uri, ".go") {
			return fmt.Errorf("Go definition should point to .go file, got: %s", uri)
		}
	case "python":
		if filepath.Ext(uri) != ".py" && !filepath.Contains(uri, ".py") {
			return fmt.Errorf("Python definition should point to .py file, got: %s", uri)
		}
	case "typescript":
		ext := filepath.Ext(uri)
		if ext != ".ts" && ext != ".tsx" && !filepath.Contains(uri, ".ts") {
			return fmt.Errorf("TypeScript definition should point to .ts/.tsx file, got: %s", uri)
		}
	case "java":
		if filepath.Ext(uri) != ".java" && !filepath.Contains(uri, ".java") {
			return fmt.Errorf("Java definition should point to .java file, got: %s", uri)
		}
	}

	return nil
}

// validateReferencesResponse validates the structure of references responses
func (r *E2ETestRunner) validateReferencesResponse(response json.RawMessage, language string, expectedMin int) error {
	var references []map[string]interface{}
	if err := json.Unmarshal(response, &references); err != nil {
		return fmt.Errorf("failed to unmarshal references response: %w", err)
	}

	if len(references) < expectedMin {
		return fmt.Errorf("expected at least %d references, got %d", expectedMin, len(references))
	}

	// Validate each reference
	for i, ref := range references {
		if _, ok := ref["uri"]; !ok {
			return fmt.Errorf("reference %d missing 'uri' field", i)
		}
		if _, ok := ref["range"]; !ok {
			return fmt.Errorf("reference %d missing 'range' field", i)
		}

		// Validate range structure
		if rangeObj, ok := ref["range"].(map[string]interface{}); ok {
			if _, ok := rangeObj["start"]; !ok {
				return fmt.Errorf("reference %d range missing 'start' field", i)
			}
			if _, ok := rangeObj["end"]; !ok {
				return fmt.Errorf("reference %d range missing 'end' field", i)
			}
		} else {
			return fmt.Errorf("reference %d range is not a valid object", i)
		}
	}

	return nil
}

// validateHoverResponse validates the structure of hover responses
func (r *E2ETestRunner) validateHoverResponse(response json.RawMessage, language string, symbolType string) error {
	var hover map[string]interface{}
	if err := json.Unmarshal(response, &hover); err != nil {
		return fmt.Errorf("failed to unmarshal hover response: %w", err)
	}

	// Validate required fields
	if _, ok := hover["contents"]; !ok {
		return fmt.Errorf("hover response missing 'contents' field")
	}

	// Validate contents structure
	if contents, ok := hover["contents"].(map[string]interface{}); ok {
		if _, ok := contents["kind"]; !ok {
			return fmt.Errorf("hover contents missing 'kind' field")
		}
		if _, ok := contents["value"]; !ok {
			return fmt.Errorf("hover contents missing 'value' field")
		}

		// Validate content kind
		kind, _ := contents["kind"].(string)
		if kind != "markdown" && kind != "plaintext" {
			return fmt.Errorf("hover contents kind should be 'markdown' or 'plaintext', got: %s", kind)
		}

		// Validate content includes language information
		value, _ := contents["value"].(string)
		if symbolType == "function" || symbolType == "method" {
			if !filepath.Contains(value, language) && !filepath.Contains(value, "```") {
				return fmt.Errorf("hover content should include language information for %s %s", language, symbolType)
			}
		}
	} else {
		return fmt.Errorf("hover contents is not a valid object")
	}

	return nil
}

// validateSymbolsResponse validates the structure of symbols responses
func (r *E2ETestRunner) validateSymbolsResponse(response json.RawMessage, expectedMin int, symbolTypes []string) error {
	var symbols []map[string]interface{}
	if err := json.Unmarshal(response, &symbols); err != nil {
		return fmt.Errorf("failed to unmarshal symbols response: %w", err)
	}

	if len(symbols) < expectedMin {
		return fmt.Errorf("expected at least %d symbols, got %d", expectedMin, len(symbols))
	}

	// Validate each symbol
	foundSymbolTypes := make(map[string]bool)
	for i, symbol := range symbols {
		if _, ok := symbol["name"]; !ok {
			return fmt.Errorf("symbol %d missing 'name' field", i)
		}
		if _, ok := symbol["kind"]; !ok {
			return fmt.Errorf("symbol %d missing 'kind' field", i)
		}
		if _, ok := symbol["location"]; !ok {
			return fmt.Errorf("symbol %d missing 'location' field", i)
		}

		// Validate location structure
		if location, ok := symbol["location"].(map[string]interface{}); ok {
			if _, ok := location["uri"]; !ok {
				return fmt.Errorf("symbol %d location missing 'uri' field", i)
			}
			if _, ok := location["range"]; !ok {
				return fmt.Errorf("symbol %d location missing 'range' field", i)
			}
		} else {
			return fmt.Errorf("symbol %d location is not a valid object", i)
		}

		// Track symbol kinds found (simplified mapping)
		if kind, ok := symbol["kind"].(float64); ok {
			switch int(kind) {
			case 5:
				foundSymbolTypes["class"] = true
			case 6:
				foundSymbolTypes["method"] = true
			case 12:
				foundSymbolTypes["function"] = true
			case 13:
				foundSymbolTypes["variable"] = true
			case 14:
				foundSymbolTypes["constant"] = true
			case 11:
				foundSymbolTypes["interface"] = true
			}
		}
	}

	// Check if we found expected symbol types
	if len(symbolTypes) > 0 {
		foundCount := 0
		for _, expectedType := range symbolTypes {
			if foundSymbolTypes[expectedType] {
				foundCount++
			}
		}
		if foundCount == 0 {
			return fmt.Errorf("expected to find symbols of types %v, but found none", symbolTypes)
		}
	}

	return nil
}