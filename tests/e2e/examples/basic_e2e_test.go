package e2e_test

import (
	"context"
	"testing"
	"time"

	"lsp-gateway/tests/e2e/helpers"
	"lsp-gateway/tests/framework"
	"lsp-gateway/tests/mocks"
)

// TestBasicE2EWorkflow demonstrates basic usage of the E2E testing framework
func TestBasicE2EWorkflow(t *testing.T) {
	// Create E2E test runner with 5-minute timeout
	runner := NewE2ETestRunner(&E2ETestConfig{
		Timeout:        5 * time.Minute,
		MaxConcurrency: 10,
		RetryAttempts:  3,
		CleanupTimeout: 30 * time.Second,
	})
	defer runner.Cleanup()

	// Setup test environment
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := runner.SetupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}

	// Execute basic MCP workflow scenario
	result, err := runner.ExecuteScenario(ctx, ScenarioBasicMCPWorkflow)
	if err != nil {
		t.Fatalf("Failed to execute basic MCP workflow: %v", err)
	}

	// Validate results
	if !result.Success {
		t.Errorf("Basic MCP workflow failed: %v", result.Error)
	}

	// Check performance metrics
	metrics := result.Metrics
	if metrics.TotalRequests == 0 {
		t.Error("No requests were processed")
	}

	if metrics.SuccessRate < 0.95 {
		t.Errorf("Success rate too low: %.2f < 0.95", metrics.SuccessRate)
	}

	t.Logf("Basic E2E workflow completed successfully:")
	t.Logf("  Total requests: %d", metrics.TotalRequests)
	t.Logf("  Success rate: %.2f", metrics.SuccessRate)
	t.Logf("  Average latency: %v", metrics.AverageLatency)
	t.Logf("  Duration: %v", result.Duration)
}

// TestLSPMethodValidation demonstrates testing individual LSP methods
func TestLSPMethodValidation(t *testing.T) {
	runner := NewE2ETestRunner(&E2ETestConfig{
		Timeout:        3 * time.Minute,
		MaxConcurrency: 5,
	})
	defer runner.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if err := runner.SetupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}

	// Execute LSP method validation scenario
	result, err := runner.ExecuteScenario(ctx, ScenarioLSPMethodValidation)
	if err != nil {
		t.Fatalf("Failed to execute LSP method validation: %v", err)
	}

	// Validate that all LSP methods were tested
	if !result.Success {
		t.Errorf("LSP method validation failed: %v", result.Error)
	}

	// Check that we tested the expected LSP methods
	expectedMethods := []string{
		"workspace/symbol",
		"textDocument/definition",
		"textDocument/references",
		"textDocument/hover",
		"textDocument/documentSymbol",
	}

	if len(result.TestedMethods) != len(expectedMethods) {
		t.Errorf("Expected %d methods, got %d", len(expectedMethods), len(result.TestedMethods))
	}

	t.Logf("LSP method validation completed:")
	t.Logf("  Methods tested: %v", result.TestedMethods)
	t.Logf("  Success rate: %.2f", result.Metrics.SuccessRate)
}

// TestMultiLanguageSupport demonstrates multi-language project testing
func TestMultiLanguageSupport(t *testing.T) {
	runner := NewE2ETestRunner(&E2ETestConfig{
		Timeout:        4 * time.Minute,
		MaxConcurrency: 8,
	})
	defer runner.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	if err := runner.SetupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}

	// Create a multi-language test project
	project, err := runner.Framework.CreateMultiLanguageProject(
		framework.ProjectTypeMonorepo,
		[]string{"go", "python", "typescript", "java"},
	)
	if err != nil {
		t.Fatalf("Failed to create multi-language project: %v", err)
	}

	// Execute multi-language support scenario
	result, err := runner.ExecuteScenario(ctx, ScenarioMultiLanguageSupport)
	if err != nil {
		t.Fatalf("Failed to execute multi-language support scenario: %v", err)
	}

	if !result.Success {
		t.Errorf("Multi-language support test failed: %v", result.Error)
	}

	// Validate that all languages were tested
	expectedLanguages := []string{"go", "python", "typescript", "java"}
	if len(result.TestedLanguages) != len(expectedLanguages) {
		t.Errorf("Expected %d languages, got %d", len(expectedLanguages), len(result.TestedLanguages))
	}

	t.Logf("Multi-language support test completed:")
	t.Logf("  Project: %s", project.Name)
	t.Logf("  Languages tested: %v", result.TestedLanguages)
	t.Logf("  Cross-language navigation: %v", result.CrossLanguageNavigation)
}

// TestWithMockConfiguration demonstrates custom mock client configuration
func TestWithMockConfiguration(t *testing.T) {
	// Create mock MCP client
	mockClient := mocks.NewMockMcpClient()

	// Configure mock for specific test scenario
	generator := e2e_test.NewTestDataGenerator()
	if err := generator.ConfigureMockClientForScenario(mockClient, "basic-workflow"); err != nil {
		t.Fatalf("Failed to configure mock client: %v", err)
	}

	// Create runner with custom mock client
	runner := NewE2ETestRunner(&E2ETestConfig{
		Timeout:        2 * time.Minute,
		MaxConcurrency: 5,
		MockClient:     mockClient, // Use custom mock client
	})
	defer runner.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := runner.SetupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}

	// Execute scenario with custom mock configuration
	result, err := runner.ExecuteScenario(ctx, ScenarioBasicMCPWorkflow)
	if err != nil {
		t.Fatalf("Failed to execute scenario with custom mock: %v", err)
	}

	if !result.Success {
		t.Errorf("Custom mock scenario failed: %v", result.Error)
	}

	// Verify mock client was used
	metrics := mockClient.GetMetrics()
	if metrics.TotalRequests == 0 {
		t.Error("Mock client was not used - no requests recorded")
	}

	t.Logf("Custom mock scenario completed:")
	t.Logf("  Mock requests: %d", metrics.TotalRequests)
	t.Logf("  Mock success rate: %.2f", float64(metrics.SuccessfulReqs)/float64(metrics.TotalRequests))
}

// TestConcurrentScenarios demonstrates running multiple scenarios concurrently
func TestConcurrentScenarios(t *testing.T) {
	runner := NewE2ETestRunner(&E2ETestConfig{
		Timeout:        6 * time.Minute,
		MaxConcurrency: 15,
	})
	defer runner.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	if err := runner.SetupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}

	// Define scenarios to run concurrently
	scenarios := []E2EScenario{
		ScenarioBasicMCPWorkflow,
		ScenarioLSPMethodValidation,
		ScenarioConcurrentRequests,
		ScenarioWorkspaceManagement,
	}

	// Execute all scenarios
	results, err := runner.ExecuteAllScenarios(ctx, scenarios)
	if err != nil {
		t.Fatalf("Failed to execute concurrent scenarios: %v", err)
	}

	// Validate all scenarios passed
	successCount := 0
	for scenario, result := range results {
		if result.Success {
			successCount++
			t.Logf("Scenario %s: SUCCESS (duration: %v)", scenario, result.Duration)
		} else {
			t.Errorf("Scenario %s: FAILED - %v", scenario, result.Error)
		}
	}

	if successCount != len(scenarios) {
		t.Errorf("Only %d out of %d scenarios passed", successCount, len(scenarios))
	}

	// Check global metrics
	globalMetrics := runner.GetGlobalMetrics()
	t.Logf("Global test metrics:")
	t.Logf("  Total requests: %d", globalMetrics.TotalRequests)
	t.Logf("  Overall success rate: %.2f", globalMetrics.OverallSuccessRate)
	t.Logf("  Total duration: %v", globalMetrics.TotalDuration)
}

// Placeholder types and functions (these would be defined in the actual framework)

type E2ETestRunner struct {
	Config     *E2ETestConfig
	Framework  *framework.MultiLanguageTestFramework
	MockClient *mocks.MockMcpClient
}

type E2ETestConfig struct {
	Timeout        time.Duration
	MaxConcurrency int
	RetryAttempts  int
	CleanupTimeout time.Duration
	MockClient     *mocks.MockMcpClient
}

type E2EScenario string

const (
	ScenarioBasicMCPWorkflow     E2EScenario = "basic-mcp-workflow"
	ScenarioLSPMethodValidation  E2EScenario = "lsp-method-validation"
	ScenarioMultiLanguageSupport E2EScenario = "multi-language-support"
	ScenarioConcurrentRequests   E2EScenario = "concurrent-requests"
	ScenarioWorkspaceManagement  E2EScenario = "workspace-management"
)

type E2ETestResult struct {
	Scenario                E2EScenario
	Success                 bool
	Error                   error
	Duration                time.Duration
	Metrics                 *E2EMetrics
	TestedMethods           []string
	TestedLanguages         []string
	CrossLanguageNavigation bool
}

type E2EMetrics struct {
	TotalRequests  int64
	SuccessfulReqs int64
	FailedRequests int64
	SuccessRate    float64
	AverageLatency time.Duration
}

type GlobalMetrics struct {
	TotalRequests      int64
	OverallSuccessRate float64
	TotalDuration      time.Duration
}

// Placeholder functions (these would be implemented in the actual framework)

func NewE2ETestRunner(config *E2ETestConfig) *E2ETestRunner {
	return &E2ETestRunner{
		Config:     config,
		Framework:  framework.NewMultiLanguageTestFramework(config.Timeout),
		MockClient: config.MockClient,
	}
}

func (r *E2ETestRunner) SetupTestEnvironment(ctx context.Context) error {
	return r.Framework.SetupTestEnvironment(ctx)
}

func (r *E2ETestRunner) ExecuteScenario(ctx context.Context, scenario E2EScenario) (*E2ETestResult, error) {
	// This would be implemented in the actual framework
	return &E2ETestResult{
		Scenario: scenario,
		Success:  true,
		Duration: 30 * time.Second,
		Metrics: &E2EMetrics{
			TotalRequests:  50,
			SuccessfulReqs: 48,
			FailedRequests: 2,
			SuccessRate:    0.96,
			AverageLatency: 150 * time.Millisecond,
		},
		TestedMethods:   []string{"workspace/symbol", "textDocument/definition"},
		TestedLanguages: []string{"go", "python"},
	}, nil
}

func (r *E2ETestRunner) ExecuteAllScenarios(ctx context.Context, scenarios []E2EScenario) (map[E2EScenario]*E2ETestResult, error) {
	results := make(map[E2EScenario]*E2ETestResult)
	for _, scenario := range scenarios {
		result, err := r.ExecuteScenario(ctx, scenario)
		if err != nil {
			return nil, err
		}
		results[scenario] = result
	}
	return results, nil
}

func (r *E2ETestRunner) GetGlobalMetrics() *GlobalMetrics {
	return &GlobalMetrics{
		TotalRequests:      200,
		OverallSuccessRate: 0.94,
		TotalDuration:      2 * time.Minute,
	}
}

func (r *E2ETestRunner) Cleanup() {
	if r.Framework != nil {
		r.Framework.CleanupAll()
	}
}
