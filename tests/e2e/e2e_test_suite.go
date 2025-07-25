package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"lsp-gateway/tests/framework"
	"lsp-gateway/tests/mocks"
)

// E2ETestSuite coordinates comprehensive end-to-end testing
type E2ETestSuite struct {
	// Core components
	framework       *framework.MultiLanguageTestFramework
	mcpMockClient   *mocks.MockMcpClient
	httpTest        *HTTPProtocolE2ETest
	mcpTest         *MCPProtocolE2ETest
	workflowTest    *WorkflowE2ETest
	integrationTest *IntegrationE2ETest

	// Configuration
	ResultsDirectory    string
	BaselineResultsFile string
	TestTimeout         time.Duration
	MaxConcurrentTests  int

	// Test orchestration
	testState        *E2ETestState
	testDependencies *TestDependencyGraph
	resourceManager  *TestResourceManager

	// Results tracking
	currentResults  *E2ETestResults
	baselineResults *E2ETestResults

	// Synchronization
	mu              sync.RWMutex
	testCoordinator *TestCoordinator

	// Context management
	ctx    context.Context
	cancel context.CancelFunc
}

// E2ETestResults contains comprehensive E2E test results
type E2ETestResults struct {
	Timestamp           time.Time            `json:"timestamp"`
	SystemInfo          *SystemInfo          `json:"system_info"`
	HTTPProtocolResults *HTTPProtocolResults `json:"http_protocol_results"`
	MCPProtocolResults  *MCPProtocolResults  `json:"mcp_protocol_results"`
	WorkflowResults     *WorkflowResults     `json:"workflow_results"`
	IntegrationResults  *IntegrationResults  `json:"integration_results"`
	OverallE2EScore     float64              `json:"overall_e2e_score"`
	TestsExecuted       int                  `json:"tests_executed"`
	TestsPassed         int                  `json:"tests_passed"`
	TestsFailed         int                  `json:"tests_failed"`
	CriticalFailures    []string             `json:"critical_failures,omitempty"`
	TestExecutionPlan   *TestExecutionPlan   `json:"test_execution_plan"`
}

// SystemInfo contains system information for E2E test context
type SystemInfo struct {
	OS              string    `json:"os"`
	Architecture    string    `json:"architecture"`
	NumCPU          int       `json:"num_cpu"`
	GoVersion       string    `json:"go_version"`
	TotalMemoryMB   int64     `json:"total_memory_mb"`
	TestStartTime   time.Time `json:"test_start_time"`
	TestEnvironment string    `json:"test_environment"`
}

// HTTPProtocolResults contains HTTP JSON-RPC protocol test results
type HTTPProtocolResults struct {
	GatewayStartupTime      int64   `json:"gateway_startup_time_ms"`
	LSPMethodsCovered       int     `json:"lsp_methods_covered"`
	RequestResponseLatency  int64   `json:"avg_request_response_latency_ms"`
	ConcurrentRequestTests  int     `json:"concurrent_request_tests"`
	ErrorHandlingTests      int     `json:"error_handling_tests"`
	ProtocolComplianceScore float64 `json:"protocol_compliance_score"`
}

// MCPProtocolResults contains MCP protocol test results
type MCPProtocolResults struct {
	MCPServerStartupTime   int64   `json:"mcp_server_startup_time_ms"`
	ToolsAvailable         int     `json:"tools_available"`
	AIAssistantIntegration bool    `json:"ai_assistant_integration_success"`
	McpToolsExecuted       int     `json:"mcp_tools_executed"`
	CrossProtocolRequests  int     `json:"cross_protocol_requests"`
	MCPComplianceScore     float64 `json:"mcp_compliance_score"`
}

// WorkflowResults contains workflow test results
type WorkflowResults struct {
	DeveloperWorkflows    int     `json:"developer_workflows_tested"`
	MultiLanguageSupport  bool    `json:"multi_language_support"`
	ProjectDetectionTests int     `json:"project_detection_tests"`
	LanguageServerPools   int     `json:"language_server_pools_tested"`
	WorkflowSuccessRate   float64 `json:"workflow_success_rate"`
	RealWorldScenarios    int     `json:"real_world_scenarios"`
}

// IntegrationResults contains integration test results
type IntegrationResults struct {
	ComponentIntegrations  int     `json:"component_integrations_tested"`
	ConfigurationTemplates int     `json:"configuration_templates_tested"`
	CircuitBreakerTests    int     `json:"circuit_breaker_tests"`
	HealthMonitoringTests  int     `json:"health_monitoring_tests"`
	LoadBalancingTests     int     `json:"load_balancing_tests"`
	IntegrationScore       float64 `json:"integration_score"`
}

// E2ETestState tracks test execution state
type E2ETestState struct {
	CurrentPhase    TestPhase            `json:"current_phase"`
	TestsInProgress map[string]*TestInfo `json:"tests_in_progress"`
	CompletedTests  map[string]*TestInfo `json:"completed_tests"`
	FailedTests     map[string]*TestInfo `json:"failed_tests"`
	ResourcesInUse  map[string]bool      `json:"resources_in_use"`
	StartTime       time.Time            `json:"start_time"`
	LastUpdate      time.Time            `json:"last_update"`
}

// TestPhase represents different phases of E2E testing
type TestPhase string

const (
	PhaseInitialization TestPhase = "initialization"
	PhaseSetup          TestPhase = "setup"
	PhaseHTTPProtocol   TestPhase = "http_protocol"
	PhaseMCPProtocol    TestPhase = "mcp_protocol"
	PhaseWorkflow       TestPhase = "workflow"
	PhaseIntegration    TestPhase = "integration"
	PhaseCleanup        TestPhase = "cleanup"
	PhaseComplete       TestPhase = "complete"
)

// TestInfo contains information about individual tests
type TestInfo struct {
	Name         string                 `json:"name"`
	Phase        TestPhase              `json:"phase"`
	StartTime    time.Time              `json:"start_time"`
	EndTime      time.Time              `json:"end_time"`
	Duration     time.Duration          `json:"duration"`
	Status       TestStatus             `json:"status"`
	Dependencies []string               `json:"dependencies"`
	Resources    []string               `json:"resources"`
	Metrics      map[string]interface{} `json:"metrics,omitempty"`
	Error        string                 `json:"error,omitempty"`
}

// TestStatus represents test execution status
type TestStatus string

const (
	StatusPending   TestStatus = "pending"
	StatusRunning   TestStatus = "running"
	StatusCompleted TestStatus = "completed"
	StatusFailed    TestStatus = "failed"
	StatusSkipped   TestStatus = "skipped"
)

// TestExecutionPlan defines the execution strategy
type TestExecutionPlan struct {
	ExecutionStrategy ExecutionStrategy        `json:"execution_strategy"`
	TestCategories    []TestCategory           `json:"test_categories"`
	Dependencies      map[string][]string      `json:"dependencies"`
	ResourceRequests  map[string][]string      `json:"resource_requests"`
	Timeouts          map[string]time.Duration `json:"timeouts"`
	ParallelGroups    [][]string               `json:"parallel_groups"`
}

// ExecutionStrategy defines how tests are executed
type ExecutionStrategy string

const (
	StrategySequential ExecutionStrategy = "sequential"
	StrategyParallel   ExecutionStrategy = "parallel"
	StrategyDependency ExecutionStrategy = "dependency_based"
	StrategyOptimized  ExecutionStrategy = "optimized"
)

// TestCategory groups related tests
type TestCategory struct {
	Name       string        `json:"name"`
	Tests      []string      `json:"tests"`
	Required   bool          `json:"required"`
	Timeout    time.Duration `json:"timeout"`
	MaxRetries int           `json:"max_retries"`
}

// TestDependencyGraph manages test dependencies
type TestDependencyGraph struct {
	dependencies map[string][]string
	mu           sync.RWMutex
}

// TestResourceManager manages shared test resources
type TestResourceManager struct {
	resources   map[string]*TestResource
	allocations map[string]string
	mu          sync.RWMutex
}

// TestResource represents a shared test resource
type TestResource struct {
	Name      string
	Type      ResourceType
	Available bool
	InUse     bool
	Owner     string
	CreatedAt time.Time
	LastUsed  time.Time
	Cleanup   func() error
}

// ResourceType defines types of test resources
type ResourceType string

const (
	ResourceTypeServer    ResourceType = "server"
	ResourceTypeProject   ResourceType = "project"
	ResourceTypeClient    ResourceType = "client"
	ResourceTypeConfig    ResourceType = "config"
	ResourceTypeWorkspace ResourceType = "workspace"
)

// TestCoordinator coordinates test execution
type TestCoordinator struct {
	activeTests    map[string]*TestExecution
	waitingTests   []*TestExecution
	completedTests []*TestExecution
	maxConcurrent  int
	currentRunning int
	mu             sync.RWMutex
}

// TestExecution represents a test being executed
type TestExecution struct {
	TestInfo *TestInfo
	Context  context.Context
	Cancel   context.CancelFunc
	Done     chan struct{}
	Result   interface{}
	Error    error
}

// NewE2ETestSuite creates a new E2E test suite
func NewE2ETestSuite(t *testing.T) *E2ETestSuite {
	resultsDir := filepath.Join("tests", "e2e", "results")
	os.MkdirAll(resultsDir, 0755)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)

	suite := &E2ETestSuite{
		framework:           framework.NewMultiLanguageTestFramework(2 * time.Hour),
		mcpMockClient:       mocks.NewMockMcpClient(),
		ResultsDirectory:    resultsDir,
		BaselineResultsFile: filepath.Join(resultsDir, "baseline_e2e_results.json"),
		TestTimeout:         2 * time.Hour,
		MaxConcurrentTests:  4,
		ctx:                 ctx,
		cancel:              cancel,
	}

	// Initialize components
	suite.initializeComponents()

	return suite
}

// initializeComponents initializes all test suite components
func (suite *E2ETestSuite) initializeComponents() {
	// Initialize test state
	suite.testState = &E2ETestState{
		CurrentPhase:    PhaseInitialization,
		TestsInProgress: make(map[string]*TestInfo),
		CompletedTests:  make(map[string]*TestInfo),
		FailedTests:     make(map[string]*TestInfo),
		ResourcesInUse:  make(map[string]bool),
		StartTime:       time.Now(),
		LastUpdate:      time.Now(),
	}

	// Initialize dependency graph
	suite.testDependencies = &TestDependencyGraph{
		dependencies: make(map[string][]string),
	}

	// Initialize resource manager
	suite.resourceManager = &TestResourceManager{
		resources:   make(map[string]*TestResource),
		allocations: make(map[string]string),
	}

	// Initialize test coordinator
	suite.testCoordinator = &TestCoordinator{
		activeTests:    make(map[string]*TestExecution),
		waitingTests:   make([]*TestExecution, 0),
		completedTests: make([]*TestExecution, 0),
		maxConcurrent:  suite.MaxConcurrentTests,
	}

	// Initialize individual test components
	suite.httpTest = NewHTTPProtocolE2ETest(suite.framework, suite.mcpMockClient)
	suite.mcpTest = NewMCPProtocolE2ETest(suite.framework, suite.mcpMockClient)
	suite.workflowTest = NewWorkflowE2ETest(suite.framework, suite.mcpMockClient)
	suite.integrationTest = NewIntegrationE2ETest(suite.framework, suite.mcpMockClient)
}

// RunFullE2ETestSuite runs the complete end-to-end test suite
func (suite *E2ETestSuite) RunFullE2ETestSuite(t *testing.T) *E2ETestResults {
	t.Log("Starting comprehensive E2E test suite...")

	// Initialize test suite
	if err := suite.initializeTestSuite(t); err != nil {
		t.Fatalf("Failed to initialize E2E test suite: %v", err)
	}
	defer suite.cleanup()

	// Load baseline results
	suite.loadBaselineResults(t)

	// Initialize current results
	suite.currentResults = &E2ETestResults{
		Timestamp:  time.Now(),
		SystemInfo: suite.collectSystemInfo(),
	}

	// Create test execution plan
	executionPlan := suite.createTestExecutionPlan()
	suite.currentResults.TestExecutionPlan = executionPlan

	// Execute test phases based on strategy
	switch executionPlan.ExecutionStrategy {
	case StrategySequential:
		suite.executeTestsSequentially(t)
	case StrategyParallel:
		suite.executeTestsParallel(t)
	case StrategyOptimized:
		suite.executeTestsOptimized(t)
	default:
		suite.executeTestsSequentially(t)
	}

	// Calculate overall E2E score
	suite.calculateOverallE2EScore()

	// Save results
	if err := suite.saveResults(); err != nil {
		t.Errorf("Failed to save E2E results: %v", err)
	}

	// Generate E2E report
	suite.generateE2EReport(t)

	return suite.currentResults
}

// initializeTestSuite initializes the E2E test suite
func (suite *E2ETestSuite) initializeTestSuite(t *testing.T) error {
	suite.updateTestPhase(PhaseSetup)

	ctx := context.Background()

	// Setup framework
	if err := suite.framework.SetupTestEnvironment(ctx); err != nil {
		return fmt.Errorf("failed to setup framework: %w", err)
	}

	// Initialize mock MCP client
	suite.mcpMockClient.Reset()
	suite.mcpMockClient.SetHealthy(true)

	// Setup test dependencies
	suite.setupTestDependencies()

	// Allocate initial resources
	if err := suite.allocateInitialResources(); err != nil {
		return fmt.Errorf("failed to allocate initial resources: %w", err)
	}

	t.Log("E2E test suite initialization completed")
	return nil
}

// createTestExecutionPlan creates a comprehensive test execution plan
func (suite *E2ETestSuite) createTestExecutionPlan() *TestExecutionPlan {
	plan := &TestExecutionPlan{
		ExecutionStrategy: StrategyOptimized,
		TestCategories: []TestCategory{
			{
				Name:       "HTTP Protocol Tests",
				Tests:      []string{"gateway_startup", "lsp_methods", "error_handling", "concurrent_requests"},
				Required:   true,
				Timeout:    15 * time.Minute,
				MaxRetries: 2,
			},
			{
				Name:       "MCP Protocol Tests",
				Tests:      []string{"mcp_startup", "tool_availability", "ai_integration", "cross_protocol"},
				Required:   true,
				Timeout:    15 * time.Minute,
				MaxRetries: 2,
			},
			{
				Name:       "Workflow Tests",
				Tests:      []string{"developer_workflows", "multi_language", "project_detection", "real_world"},
				Required:   true,
				Timeout:    30 * time.Minute,
				MaxRetries: 1,
			},
			{
				Name:       "Integration Tests",
				Tests:      []string{"component_integration", "config_templates", "circuit_breaker", "health_monitoring"},
				Required:   false,
				Timeout:    20 * time.Minute,
				MaxRetries: 1,
			},
		},
		Dependencies: map[string][]string{
			"mcp_startup":           {"gateway_startup"},
			"ai_integration":        {"mcp_startup", "tool_availability"},
			"cross_protocol":        {"gateway_startup", "mcp_startup"},
			"developer_workflows":   {"gateway_startup", "mcp_startup"},
			"multi_language":        {"developer_workflows"},
			"component_integration": {"gateway_startup", "mcp_startup"},
		},
		ResourceRequests: map[string][]string{
			"gateway_startup":       {"server", "config"},
			"mcp_startup":           {"server", "config"},
			"concurrent_requests":   {"server", "client"},
			"developer_workflows":   {"server", "project", "workspace"},
			"component_integration": {"server", "config", "project"},
		},
		Timeouts: map[string]time.Duration{
			"gateway_startup":     5 * time.Minute,
			"mcp_startup":         5 * time.Minute,
			"lsp_methods":         10 * time.Minute,
			"ai_integration":      10 * time.Minute,
			"developer_workflows": 20 * time.Minute,
		},
		ParallelGroups: [][]string{
			{"lsp_methods", "error_handling"},
			{"tool_availability", "ai_integration"},
			{"project_detection", "real_world"},
			{"circuit_breaker", "health_monitoring"},
		},
	}

	return plan
}

// executeTestsOptimized executes tests using optimized strategy
func (suite *E2ETestSuite) executeTestsOptimized(t *testing.T) {
	plan := suite.currentResults.TestExecutionPlan

	// Execute tests in dependency order with parallelization
	for _, category := range plan.TestCategories {
		t.Run(category.Name, func(t *testing.T) {
			suite.executeCategoryTests(t, &category)
		})
	}
}

// executeTestsSequentially executes tests in sequential order
func (suite *E2ETestSuite) executeTestsSequentially(t *testing.T) {
	// HTTP Protocol Tests
	suite.updateTestPhase(PhaseHTTPProtocol)
	t.Run("HTTPProtocolTests", func(t *testing.T) {
		suite.executeHTTPProtocolTests(t)
	})

	// MCP Protocol Tests
	suite.updateTestPhase(PhaseMCPProtocol)
	t.Run("MCPProtocolTests", func(t *testing.T) {
		suite.executeMCPProtocolTests(t)
	})

	// Workflow Tests
	suite.updateTestPhase(PhaseWorkflow)
	t.Run("WorkflowTests", func(t *testing.T) {
		suite.executeWorkflowTests(t)
	})

	// Integration Tests
	suite.updateTestPhase(PhaseIntegration)
	t.Run("IntegrationTests", func(t *testing.T) {
		suite.executeIntegrationTests(t)
	})
}

// executeTestsParallel executes tests in parallel where possible
func (suite *E2ETestSuite) executeTestsParallel(t *testing.T) {
	var wg sync.WaitGroup

	// Start HTTP and MCP protocol tests in parallel (after setup dependencies)
	wg.Add(2)
	go func() {
		defer wg.Done()
		suite.updateTestPhase(PhaseHTTPProtocol)
		suite.executeHTTPProtocolTests(t)
	}()

	go func() {
		defer wg.Done()
		suite.updateTestPhase(PhaseMCPProtocol)
		suite.executeMCPProtocolTests(t)
	}()

	wg.Wait()

	// Execute workflow and integration tests after protocol tests complete
	wg.Add(2)
	go func() {
		defer wg.Done()
		suite.updateTestPhase(PhaseWorkflow)
		suite.executeWorkflowTests(t)
	}()

	go func() {
		defer wg.Done()
		suite.updateTestPhase(PhaseIntegration)
		suite.executeIntegrationTests(t)
	}()

	wg.Wait()
}

// executeCategoryTests executes tests for a specific category
func (suite *E2ETestSuite) executeCategoryTests(t *testing.T, category *TestCategory) {
	for _, testName := range category.Tests {
		suite.executeIndividualTest(t, testName, category)
	}
}

// executeIndividualTest executes a single test with resource management
func (suite *E2ETestSuite) executeIndividualTest(t *testing.T, testName string, category *TestCategory) {
	testInfo := &TestInfo{
		Name:      testName,
		Phase:     suite.testState.CurrentPhase,
		StartTime: time.Now(),
		Status:    StatusRunning,
	}

	suite.trackTestStart(testName, testInfo)

	// Allocate required resources
	resources, err := suite.allocateTestResources(testName)
	if err != nil {
		suite.trackTestFailure(testName, testInfo, fmt.Sprintf("Resource allocation failed: %v", err))
		return
	}
	defer suite.releaseTestResources(testName, resources)

	// Execute the test
	err = suite.executeTestByName(testName, testInfo)

	testInfo.EndTime = time.Now()
	testInfo.Duration = testInfo.EndTime.Sub(testInfo.StartTime)

	if err != nil {
		suite.trackTestFailure(testName, testInfo, err.Error())
		t.Errorf("Test %s failed: %v", testName, err)
	} else {
		suite.trackTestCompletion(testName, testInfo)
		t.Logf("Test %s completed successfully in %v", testName, testInfo.Duration)
	}
}

// executeTestByName executes a specific test by name
func (suite *E2ETestSuite) executeTestByName(testName string, testInfo *TestInfo) error {
	switch testName {
	case "gateway_startup":
		return suite.httpTest.TestGatewayStartup()
	case "lsp_methods":
		return suite.httpTest.TestLSPMethods()
	case "error_handling":
		return suite.httpTest.TestErrorHandling()
	case "concurrent_requests":
		return suite.httpTest.TestConcurrentRequests()
	case "mcp_startup":
		return suite.mcpTest.TestMCPStartup()
	case "tool_availability":
		return suite.mcpTest.TestToolAvailability()
	case "ai_integration":
		return suite.mcpTest.TestAIIntegration()
	case "cross_protocol":
		return suite.mcpTest.TestCrossProtocol()
	case "developer_workflows":
		return suite.workflowTest.TestDeveloperWorkflows()
	case "multi_language":
		return suite.workflowTest.TestMultiLanguage()
	case "project_detection":
		return suite.workflowTest.TestProjectDetection()
	case "real_world":
		return suite.workflowTest.TestRealWorldScenarios()
	case "component_integration":
		return suite.integrationTest.TestComponentIntegration()
	case "config_templates":
		return suite.integrationTest.TestConfigTemplates()
	case "circuit_breaker":
		return suite.integrationTest.TestCircuitBreaker()
	case "health_monitoring":
		return suite.integrationTest.TestHealthMonitoring()
	default:
		return fmt.Errorf("unknown test: %s", testName)
	}
}

// HTTP Protocol Tests
func (suite *E2ETestSuite) executeHTTPProtocolTests(t *testing.T) {
	results, err := suite.httpTest.RunHTTPProtocolTests()
	if err != nil {
		t.Errorf("HTTP protocol tests failed: %v", err)
		return
	}

	suite.currentResults.HTTPProtocolResults = results
	suite.currentResults.TestsExecuted += results.LSPMethodsCovered + results.ConcurrentRequestTests + results.ErrorHandlingTests
	if results.ProtocolComplianceScore >= 0.8 {
		suite.currentResults.TestsPassed += results.LSPMethodsCovered + results.ConcurrentRequestTests + results.ErrorHandlingTests
	} else {
		suite.currentResults.TestsFailed += results.LSPMethodsCovered + results.ConcurrentRequestTests + results.ErrorHandlingTests
	}
}

// MCP Protocol Tests
func (suite *E2ETestSuite) executeMCPProtocolTests(t *testing.T) {
	results, err := suite.mcpTest.RunMCPProtocolTests()
	if err != nil {
		t.Errorf("MCP protocol tests failed: %v", err)
		return
	}

	suite.currentResults.MCPProtocolResults = results
	suite.currentResults.TestsExecuted += results.ToolsAvailable + results.McpToolsExecuted + results.CrossProtocolRequests
	if results.MCPComplianceScore >= 0.8 {
		suite.currentResults.TestsPassed += results.ToolsAvailable + results.McpToolsExecuted + results.CrossProtocolRequests
	} else {
		suite.currentResults.TestsFailed += results.ToolsAvailable + results.McpToolsExecuted + results.CrossProtocolRequests
	}
}

// Workflow Tests
func (suite *E2ETestSuite) executeWorkflowTests(t *testing.T) {
	results, err := suite.workflowTest.RunWorkflowTests()
	if err != nil {
		t.Errorf("Workflow tests failed: %v", err)
		return
	}

	suite.currentResults.WorkflowResults = results
	suite.currentResults.TestsExecuted += results.DeveloperWorkflows + results.ProjectDetectionTests + results.RealWorldScenarios
	if results.WorkflowSuccessRate >= 0.8 {
		suite.currentResults.TestsPassed += results.DeveloperWorkflows + results.ProjectDetectionTests + results.RealWorldScenarios
	} else {
		suite.currentResults.TestsFailed += results.DeveloperWorkflows + results.ProjectDetectionTests + results.RealWorldScenarios
	}
}

// Integration Tests
func (suite *E2ETestSuite) executeIntegrationTests(t *testing.T) {
	results, err := suite.integrationTest.RunIntegrationTests()
	if err != nil {
		t.Errorf("Integration tests failed: %v", err)
		return
	}

	suite.currentResults.IntegrationResults = results
	suite.currentResults.TestsExecuted += results.ComponentIntegrations + results.ConfigurationTemplates + results.CircuitBreakerTests
	if results.IntegrationScore >= 0.8 {
		suite.currentResults.TestsPassed += results.ComponentIntegrations + results.ConfigurationTemplates + results.CircuitBreakerTests
	} else {
		suite.currentResults.TestsFailed += results.ComponentIntegrations + results.ConfigurationTemplates + results.CircuitBreakerTests
	}
}

// calculateOverallE2EScore calculates the overall E2E test score
func (suite *E2ETestSuite) calculateOverallE2EScore() {
	score := 0.0
	weights := map[string]float64{
		"http_protocol": 0.25,
		"mcp_protocol":  0.25,
		"workflow":      0.30,
		"integration":   0.20,
	}

	// HTTP Protocol score
	httpScore := 100.0
	if suite.currentResults.HTTPProtocolResults != nil {
		httpScore = suite.currentResults.HTTPProtocolResults.ProtocolComplianceScore * 100
	}

	// MCP Protocol score
	mcpScore := 100.0
	if suite.currentResults.MCPProtocolResults != nil {
		mcpScore = suite.currentResults.MCPProtocolResults.MCPComplianceScore * 100
	}

	// Workflow score
	workflowScore := 100.0
	if suite.currentResults.WorkflowResults != nil {
		workflowScore = suite.currentResults.WorkflowResults.WorkflowSuccessRate * 100
	}

	// Integration score
	integrationScore := 100.0
	if suite.currentResults.IntegrationResults != nil {
		integrationScore = suite.currentResults.IntegrationResults.IntegrationScore * 100
	}

	// Calculate weighted average
	score = httpScore*weights["http_protocol"] +
		mcpScore*weights["mcp_protocol"] +
		workflowScore*weights["workflow"] +
		integrationScore*weights["integration"]

	suite.currentResults.OverallE2EScore = score
}

// Test state management
func (suite *E2ETestSuite) updateTestPhase(phase TestPhase) {
	suite.mu.Lock()
	defer suite.mu.Unlock()

	suite.testState.CurrentPhase = phase
	suite.testState.LastUpdate = time.Now()
}

func (suite *E2ETestSuite) trackTestStart(testName string, testInfo *TestInfo) {
	suite.mu.Lock()
	defer suite.mu.Unlock()

	suite.testState.TestsInProgress[testName] = testInfo
	suite.testState.LastUpdate = time.Now()
}

func (suite *E2ETestSuite) trackTestCompletion(testName string, testInfo *TestInfo) {
	suite.mu.Lock()
	defer suite.mu.Unlock()

	testInfo.Status = StatusCompleted
	suite.testState.CompletedTests[testName] = testInfo
	delete(suite.testState.TestsInProgress, testName)
	suite.testState.LastUpdate = time.Now()
}

func (suite *E2ETestSuite) trackTestFailure(testName string, testInfo *TestInfo, errorMsg string) {
	suite.mu.Lock()
	defer suite.mu.Unlock()

	testInfo.Status = StatusFailed
	testInfo.Error = errorMsg
	suite.testState.FailedTests[testName] = testInfo
	delete(suite.testState.TestsInProgress, testName)
	suite.testState.LastUpdate = time.Now()

	// Track critical failures
	if suite.currentResults != nil {
		suite.currentResults.CriticalFailures = append(suite.currentResults.CriticalFailures,
			fmt.Sprintf("%s: %s", testName, errorMsg))
	}
}

// Resource management
func (suite *E2ETestSuite) allocateInitialResources() error {
	// Allocate core resources needed for E2E testing
	resources := []TestResource{
		{Name: "http_server", Type: ResourceTypeServer, Available: true},
		{Name: "mcp_server", Type: ResourceTypeServer, Available: true},
		{Name: "test_client", Type: ResourceTypeClient, Available: true},
		{Name: "test_config", Type: ResourceTypeConfig, Available: true},
		{Name: "test_workspace", Type: ResourceTypeWorkspace, Available: true},
	}

	for _, resource := range resources {
		suite.resourceManager.resources[resource.Name] = &resource
	}

	return nil
}

func (suite *E2ETestSuite) allocateTestResources(testName string) ([]string, error) {
	plan := suite.currentResults.TestExecutionPlan
	requestedResources, exists := plan.ResourceRequests[testName]
	if !exists {
		return []string{}, nil
	}

	suite.resourceManager.mu.Lock()
	defer suite.resourceManager.mu.Unlock()

	allocated := []string{}
	for _, resourceType := range requestedResources {
		// Find available resource of requested type
		for name, resource := range suite.resourceManager.resources {
			if string(resource.Type) == resourceType && resource.Available && !resource.InUse {
				resource.InUse = true
				resource.Owner = testName
				resource.LastUsed = time.Now()
				suite.resourceManager.allocations[name] = testName
				allocated = append(allocated, name)
				break
			}
		}
	}

	return allocated, nil
}

func (suite *E2ETestSuite) releaseTestResources(testName string, resources []string) {
	suite.resourceManager.mu.Lock()
	defer suite.resourceManager.mu.Unlock()

	for _, resourceName := range resources {
		if resource, exists := suite.resourceManager.resources[resourceName]; exists {
			resource.InUse = false
			resource.Owner = ""
			delete(suite.resourceManager.allocations, resourceName)
		}
	}
}

// Dependency management
func (suite *E2ETestSuite) setupTestDependencies() {
	dependencies := map[string][]string{
		"mcp_startup":           {"gateway_startup"},
		"ai_integration":        {"mcp_startup", "tool_availability"},
		"cross_protocol":        {"gateway_startup", "mcp_startup"},
		"developer_workflows":   {"gateway_startup", "mcp_startup"},
		"multi_language":        {"developer_workflows"},
		"component_integration": {"gateway_startup", "mcp_startup"},
	}

	suite.testDependencies.mu.Lock()
	defer suite.testDependencies.mu.Unlock()

	suite.testDependencies.dependencies = dependencies
}

// Results management
func (suite *E2ETestSuite) loadBaselineResults(t *testing.T) {
	if _, err := os.Stat(suite.BaselineResultsFile); os.IsNotExist(err) {
		t.Log("No baseline E2E results file found - this will become the new baseline")
		return
	}

	data, err := os.ReadFile(suite.BaselineResultsFile)
	if err != nil {
		t.Logf("Failed to read baseline E2E results: %v", err)
		return
	}

	if err := json.Unmarshal(data, &suite.baselineResults); err != nil {
		t.Logf("Failed to parse baseline E2E results: %v", err)
		return
	}

	t.Logf("Loaded baseline E2E results from %s", suite.BaselineResultsFile)
}

func (suite *E2ETestSuite) saveResults() error {
	currentFile := filepath.Join(suite.ResultsDirectory,
		fmt.Sprintf("e2e_results_%s.json", time.Now().Format("20060102_150405")))

	data, err := json.MarshalIndent(suite.currentResults, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal E2E results: %w", err)
	}

	if err := os.WriteFile(currentFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write E2E results file: %w", err)
	}

	// Update baseline if score is better
	if suite.baselineResults == nil ||
		suite.currentResults.OverallE2EScore > suite.baselineResults.OverallE2EScore {

		if err := os.WriteFile(suite.BaselineResultsFile, data, 0644); err != nil {
			return fmt.Errorf("failed to update baseline E2E results: %w", err)
		}
	}

	return nil
}

func (suite *E2ETestSuite) generateE2EReport(t *testing.T) {
	report := fmt.Sprintf(`
E2E Test Report
===============
Timestamp: %s
Overall E2E Score: %.1f/100
Tests Executed: %d
Tests Passed: %d
Tests Failed: %d

System Information:
- OS: %s %s
- CPUs: %d
- Go Version: %s
- Test Environment: %s

HTTP Protocol Results:
- Gateway Startup Time: %dms
- LSP Methods Covered: %d
- Protocol Compliance Score: %.1f%%

MCP Protocol Results:
- MCP Server Startup Time: %dms
- Tools Available: %d
- MCP Compliance Score: %.1f%%

Workflow Results:
- Developer Workflows Tested: %d
- Multi-Language Support: %t
- Workflow Success Rate: %.1f%%

Integration Results:
- Component Integrations: %d
- Configuration Templates: %d
- Integration Score: %.1f%%

`,
		suite.currentResults.Timestamp.Format("2006-01-02 15:04:05"),
		suite.currentResults.OverallE2EScore,
		suite.currentResults.TestsExecuted,
		suite.currentResults.TestsPassed,
		suite.currentResults.TestsFailed,
		suite.currentResults.SystemInfo.OS,
		suite.currentResults.SystemInfo.Architecture,
		suite.currentResults.SystemInfo.NumCPU,
		suite.currentResults.SystemInfo.GoVersion,
		suite.currentResults.SystemInfo.TestEnvironment,
		suite.currentResults.HTTPProtocolResults.GatewayStartupTime,
		suite.currentResults.HTTPProtocolResults.LSPMethodsCovered,
		suite.currentResults.HTTPProtocolResults.ProtocolComplianceScore*100,
		suite.currentResults.MCPProtocolResults.MCPServerStartupTime,
		suite.currentResults.MCPProtocolResults.ToolsAvailable,
		suite.currentResults.MCPProtocolResults.MCPComplianceScore*100,
		suite.currentResults.WorkflowResults.DeveloperWorkflows,
		suite.currentResults.WorkflowResults.MultiLanguageSupport,
		suite.currentResults.WorkflowResults.WorkflowSuccessRate*100,
		suite.currentResults.IntegrationResults.ComponentIntegrations,
		suite.currentResults.IntegrationResults.ConfigurationTemplates,
		suite.currentResults.IntegrationResults.IntegrationScore*100,
	)

	if len(suite.currentResults.CriticalFailures) > 0 {
		report += "Critical Failures:\n"
		for _, failure := range suite.currentResults.CriticalFailures {
			report += fmt.Sprintf("- %s\n", failure)
		}
	}

	// Save report to file
	reportFile := filepath.Join(suite.ResultsDirectory,
		fmt.Sprintf("e2e_report_%s.txt", time.Now().Format("20060102_150405")))

	if err := os.WriteFile(reportFile, []byte(report), 0644); err != nil {
		t.Errorf("Failed to save E2E report: %v", err)
	} else {
		t.Logf("E2E report saved to %s", reportFile)
	}

	t.Log(report)
}

func (suite *E2ETestSuite) collectSystemInfo() *SystemInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return &SystemInfo{
		OS:              runtime.GOOS,
		Architecture:    runtime.GOARCH,
		NumCPU:          runtime.NumCPU(),
		GoVersion:       runtime.Version(),
		TotalMemoryMB:   int64(m.Sys) / 1024 / 1024,
		TestStartTime:   time.Now(),
		TestEnvironment: "development",
	}
}

func (suite *E2ETestSuite) cleanup() {
	suite.updateTestPhase(PhaseCleanup)

	if suite.framework != nil {
		suite.framework.CleanupAll()
	}

	if suite.mcpMockClient != nil {
		suite.mcpMockClient.Reset()
	}

	// Cleanup all allocated resources
	suite.resourceManager.mu.Lock()
	for _, resource := range suite.resourceManager.resources {
		if resource.Cleanup != nil {
			resource.Cleanup()
		}
	}
	suite.resourceManager.mu.Unlock()

	if suite.cancel != nil {
		suite.cancel()
	}

	suite.updateTestPhase(PhaseComplete)
}

// Test main functions for individual test categories
func TestFullE2ETestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping full E2E test suite in short mode")
	}

	suite := NewE2ETestSuite(t)
	results := suite.RunFullE2ETestSuite(t)

	// Validate E2E requirements
	if results.OverallE2EScore < 80.0 {
		t.Errorf("E2E test suite score below threshold: %.1f < 80.0", results.OverallE2EScore)
	}

	if results.TestsFailed > results.TestsExecuted/10 {
		t.Errorf("Too many test failures: %d failed out of %d executed",
			results.TestsFailed, results.TestsExecuted)
	}
}

func TestE2EQuickValidation(t *testing.T) {
	suite := NewE2ETestSuite(t)
	suite.TestTimeout = 10 * time.Minute

	// Run only critical tests for quick validation
	suite.currentResults = &E2ETestResults{
		Timestamp:  time.Now(),
		SystemInfo: suite.collectSystemInfo(),
	}

	if err := suite.initializeTestSuite(t); err != nil {
		t.Fatalf("Failed to initialize E2E test suite: %v", err)
	}
	defer suite.cleanup()

	// Run only gateway startup and basic MCP tests
	t.Run("QuickHTTPValidation", func(t *testing.T) {
		if err := suite.httpTest.TestGatewayStartup(); err != nil {
			t.Errorf("Gateway startup test failed: %v", err)
		}
	})

	t.Run("QuickMCPValidation", func(t *testing.T) {
		if err := suite.mcpTest.TestMCPStartup(); err != nil {
			t.Errorf("MCP startup test failed: %v", err)
		}
	})
}
