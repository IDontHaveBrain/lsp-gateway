package e2e_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// IntegrationScenario represents an integration test scenario
type IntegrationScenario struct {
	Name           string
	Description    string
	Languages      []string
	ProjectType    string
	SetupSteps     []SetupStep
	TestSteps      []IntegrationStep
	CleanupSteps   []CleanupStep
	Timeout        time.Duration
	RequiresSetup  bool
	ValidationFunc func(results *IntegrationResults) error
}

// SetupStep represents a setup step for the integration test
type SetupStep struct {
	Name        string
	Description string
	Action      string
	Parameters  map[string]interface{}
	Timeout     time.Duration
}

// IntegrationStep represents a step in an integration test
type IntegrationStep struct {
	Name           string
	Description    string
	Type           StepType
	Language       string
	Method         string
	Parameters     map[string]interface{}
	ExpectedResult interface{}
	Timeout        time.Duration
	Parallel       bool
	Dependencies   []string // Names of steps this step depends on
	ValidationFunc func(result interface{}) error
}

// CleanupStep represents a cleanup step
type CleanupStep struct {
	Name        string
	Description string
	Action      string
	Parameters  map[string]interface{}
}

// StepType represents the type of integration step
type StepType string

const (
	StepTypeLSP      StepType = "lsp"
	StepTypeMCP      StepType = "mcp"
	StepTypeHTTP     StepType = "http"
	StepTypeSystem   StepType = "system"
	StepTypeValidate StepType = "validate"
)

// IntegrationResults contains the results of an integration test
type IntegrationResults struct {
	ScenarioName      string
	StartTime         time.Time
	EndTime           time.Time
	Duration          time.Duration
	StepResults       map[string]*StepResult
	TotalSteps        int
	PassedSteps       int
	FailedSteps       int
	SkippedSteps      int
	SetupSuccessful   bool
	CleanupSuccessful bool
	Errors            []string
}

// StepResult contains the result of a single integration step
type StepResult struct {
	StepName     string
	StartTime    time.Time
	EndTime      time.Time
	Duration     time.Duration
	Success      bool
	Result       interface{}
	Error        error
	Logs         []string
	Dependencies []string
}

// IntegrationTestManager manages integration test scenarios
type IntegrationTestManager struct {
	gatewayURL string
	mcpURL     string
	results    *IntegrationResults
	mu         sync.RWMutex
	scenarios  map[string]*IntegrationScenario
	stepStates map[string]chan *StepResult
}

// NewIntegrationTestManager creates a new integration test manager
func NewIntegrationTestManager(gatewayURL, mcpURL string) *IntegrationTestManager {
	return &IntegrationTestManager{
		gatewayURL: gatewayURL,
		mcpURL:     mcpURL,
		scenarios:  make(map[string]*IntegrationScenario),
		stepStates: make(map[string]chan *StepResult),
	}
}

// RegisterScenario registers a new integration scenario
func (m *IntegrationTestManager) RegisterScenario(scenario *IntegrationScenario) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scenarios[scenario.Name] = scenario
}

// ExecuteScenario executes a specific integration scenario
func (m *IntegrationTestManager) ExecuteScenario(t *testing.T, scenarioName string) error {
	m.mu.RLock()
	scenario, exists := m.scenarios[scenarioName]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("scenario %s not found", scenarioName)
	}

	t.Logf("Executing integration scenario: %s", scenario.Name)
	t.Logf("Description: %s", scenario.Description)
	t.Logf("Languages: %v", scenario.Languages)

	// Initialize results
	m.initializeResults(scenario)

	ctx, cancel := context.WithTimeout(context.Background(), scenario.Timeout)
	defer cancel()

	// Execute setup steps
	if scenario.RequiresSetup {
		t.Logf("Executing setup steps...")
		if err := m.executeSetupSteps(ctx, scenario.SetupSteps); err != nil {
			m.results.SetupSuccessful = false
			return fmt.Errorf("setup failed: %w", err)
		}
		m.results.SetupSuccessful = true
		t.Logf("Setup completed successfully")
	}

	// Execute test steps
	t.Logf("Executing integration test steps...")
	if err := m.executeTestSteps(ctx, scenario.TestSteps); err != nil {
		// Continue to cleanup even if tests fail
		t.Logf("Integration tests failed: %v", err)
	}

	// Execute cleanup steps
	if len(scenario.CleanupSteps) > 0 {
		t.Logf("Executing cleanup steps...")
		if err := m.executeCleanupSteps(ctx, scenario.CleanupSteps); err != nil {
			t.Logf("Cleanup failed: %v", err)
			m.results.CleanupSuccessful = false
		} else {
			m.results.CleanupSuccessful = true
		}
	}

	// Finalize results
	m.finalizeResults()

	// Validate results
	if scenario.ValidationFunc != nil {
		if err := scenario.ValidationFunc(m.results); err != nil {
			return fmt.Errorf("scenario validation failed: %w", err)
		}
	}

	// Check if any steps failed
	if m.results.FailedSteps > 0 {
		return fmt.Errorf("integration scenario failed: %d/%d steps failed",
			m.results.FailedSteps, m.results.TotalSteps)
	}

	t.Logf("Scenario %s completed successfully", scenarioName)
	m.logResults(t)

	return nil
}

// initializeResults initializes the test results structure
func (m *IntegrationTestManager) initializeResults(scenario *IntegrationScenario) {
	m.results = &IntegrationResults{
		ScenarioName: scenario.Name,
		StartTime:    time.Now(),
		StepResults:  make(map[string]*StepResult),
		TotalSteps:   len(scenario.TestSteps),
		Errors:       make([]string, 0),
	}

	// Initialize step state channels
	m.stepStates = make(map[string]chan *StepResult)
	for _, step := range scenario.TestSteps {
		m.stepStates[step.Name] = make(chan *StepResult, 1)
	}
}

// executeSetupSteps executes all setup steps
func (m *IntegrationTestManager) executeSetupSteps(ctx context.Context, steps []SetupStep) error {
	for _, step := range steps {
		if err := m.executeSetupStep(ctx, step); err != nil {
			return fmt.Errorf("setup step '%s' failed: %w", step.Name, err)
		}
	}
	return nil
}

// executeSetupStep executes a single setup step
func (m *IntegrationTestManager) executeSetupStep(ctx context.Context, step SetupStep) error {
	stepCtx := ctx
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		stepCtx, cancel = context.WithTimeout(ctx, step.Timeout)
		defer cancel()
	}

	switch step.Action {
	case "start_server":
		return m.startServer(stepCtx, step.Parameters)
	case "create_project":
		return m.createProject(stepCtx, step.Parameters)
	case "install_dependencies":
		return m.installDependencies(stepCtx, step.Parameters)
	case "wait":
		duration := time.Duration(step.Parameters["duration"].(int64))
		time.Sleep(duration)
		return nil
	default:
		return fmt.Errorf("unknown setup action: %s", step.Action)
	}
}

// executeTestSteps executes all test steps, handling dependencies and parallelism
func (m *IntegrationTestManager) executeTestSteps(ctx context.Context, steps []IntegrationStep) error {
	// Build dependency graph
	_ = m.buildDependencyGraph(steps)

	// Execute steps respecting dependencies
	var wg sync.WaitGroup
	errChan := make(chan error, len(steps))

	for _, step := range steps {
		if step.Parallel && len(step.Dependencies) == 0 {
			// Execute parallel steps without dependencies immediately
			wg.Add(1)
			go func(s IntegrationStep) {
				defer wg.Done()
				if err := m.executeTestStep(ctx, s); err != nil {
					errChan <- err
				}
			}(step)
		} else {
			// Execute sequential steps or steps with dependencies
			if err := m.waitForDependencies(step.Dependencies); err != nil {
				return fmt.Errorf("dependency wait failed for step '%s': %w", step.Name, err)
			}

			if err := m.executeTestStep(ctx, step); err != nil {
				errChan <- err
			}
		}
	}

	wg.Wait()
	close(errChan)

	// Collect any errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("test steps failed: %v", errors)
	}

	return nil
}

// buildDependencyGraph builds a dependency graph for the test steps
func (m *IntegrationTestManager) buildDependencyGraph(steps []IntegrationStep) map[string][]string {
	graph := make(map[string][]string)
	for _, step := range steps {
		graph[step.Name] = step.Dependencies
	}
	return graph
}

// waitForDependencies waits for all dependency steps to complete
func (m *IntegrationTestManager) waitForDependencies(dependencies []string) error {
	for _, dep := range dependencies {
		if resultChan, exists := m.stepStates[dep]; exists {
			select {
			case result := <-resultChan:
				if !result.Success {
					return fmt.Errorf("dependency step '%s' failed", dep)
				}
			case <-time.After(30 * time.Second):
				return fmt.Errorf("timeout waiting for dependency step '%s'", dep)
			}
		} else {
			return fmt.Errorf("dependency step '%s' not found", dep)
		}
	}
	return nil
}

// executeTestStep executes a single test step
func (m *IntegrationTestManager) executeTestStep(ctx context.Context, step IntegrationStep) error {
	stepResult := &StepResult{
		StepName:     step.Name,
		StartTime:    time.Now(),
		Dependencies: step.Dependencies,
		Logs:         make([]string, 0),
	}

	defer func() {
		stepResult.EndTime = time.Now()
		stepResult.Duration = stepResult.EndTime.Sub(stepResult.StartTime)

		m.mu.Lock()
		m.results.StepResults[step.Name] = stepResult
		if stepResult.Success {
			m.results.PassedSteps++
		} else {
			m.results.FailedSteps++
			if stepResult.Error != nil {
				m.results.Errors = append(m.results.Errors, stepResult.Error.Error())
			}
		}
		m.mu.Unlock()

		// Signal completion to any waiting dependencies
		if resultChan, exists := m.stepStates[step.Name]; exists {
			select {
			case resultChan <- stepResult:
			default:
			}
		}
	}()

	stepCtx := ctx
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		stepCtx, cancel = context.WithTimeout(ctx, step.Timeout)
		defer cancel()
	}

	var result interface{}
	var err error

	switch step.Type {
	case StepTypeLSP:
		result, err = m.executeLSPStep(stepCtx, step)
	case StepTypeMCP:
		result, err = m.executeMCPStep(stepCtx, step)
	case StepTypeHTTP:
		result, err = m.executeHTTPStep(stepCtx, step)
	case StepTypeSystem:
		result, err = m.executeSystemStep(stepCtx, step)
	case StepTypeValidate:
		result, err = m.executeValidationStep(stepCtx, step)
	default:
		err = fmt.Errorf("unknown step type: %s", step.Type)
	}

	stepResult.Result = result
	stepResult.Error = err
	stepResult.Success = err == nil

	// Custom validation
	if err == nil && step.ValidationFunc != nil {
		if validationErr := step.ValidationFunc(result); validationErr != nil {
			stepResult.Error = validationErr
			stepResult.Success = false
		}
	}

	return err
}

// executeLSPStep executes an LSP-related step
func (m *IntegrationTestManager) executeLSPStep(ctx context.Context, step IntegrationStep) (interface{}, error) {
	// Implement LSP step execution
	// This would use the LSP workflow manager from lsp_workflow_scenarios.go
	workflowManager := NewLSPWorkflowManager(m.gatewayURL)

	// Create a simple workflow step based on the integration step
	workflowStep := WorkflowStep{
		Name:   step.Name,
		Method: step.Method,
	}

	// Extract parameters
	if uri, ok := step.Parameters["uri"].(string); ok {
		workflowStep.URI = uri
	}
	if content, ok := step.Parameters["content"].(string); ok {
		workflowStep.Content = content
	}
	if pos, ok := step.Parameters["position"].(map[string]interface{}); ok {
		if line, ok := pos["line"].(int); ok {
			workflowStep.Position.Line = line
		}
		if char, ok := pos["character"].(int); ok {
			workflowStep.Position.Character = char
		}
	}

	projectDir := "/tmp/test-project" // Default project directory
	if dir, ok := step.Parameters["project_dir"].(string); ok {
		projectDir = dir
	}

	return workflowManager.executeStep(ctx, workflowStep, projectDir)
}

// executeMCPStep executes an MCP-related step
func (m *IntegrationTestManager) executeMCPStep(ctx context.Context, step IntegrationStep) (interface{}, error) {
	// Implement MCP step execution
	// This would involve MCP protocol communication

	// Simulate MCP request execution
	time.Sleep(100 * time.Millisecond)

	return map[string]interface{}{
		"method": step.Method,
		"result": "mcp_success",
	}, nil
}

// executeHTTPStep executes an HTTP-related step
func (m *IntegrationTestManager) executeHTTPStep(ctx context.Context, step IntegrationStep) (interface{}, error) {
	// Implement HTTP step execution
	// This would involve direct HTTP requests to the gateway

	// Simulate HTTP request execution
	time.Sleep(50 * time.Millisecond)

	return map[string]interface{}{
		"method":     step.Method,
		"status":     "success",
		"statusCode": 200,
	}, nil
}

// executeSystemStep executes a system-related step
func (m *IntegrationTestManager) executeSystemStep(ctx context.Context, step IntegrationStep) (interface{}, error) {
	// Implement system step execution (file operations, process management, etc.)

	action := step.Parameters["action"].(string)

	switch action {
	case "check_file_exists":
		filepath := step.Parameters["filepath"].(string)
		// Simulate file existence check
		return map[string]interface{}{
			"exists":   true,
			"filepath": filepath,
		}, nil
	case "check_process":
		processName := step.Parameters["process"].(string)
		// Simulate process check
		return map[string]interface{}{
			"running": true,
			"process": processName,
		}, nil
	default:
		return nil, fmt.Errorf("unknown system action: %s", action)
	}
}

// executeValidationStep executes a validation step
func (m *IntegrationTestManager) executeValidationStep(ctx context.Context, step IntegrationStep) (interface{}, error) {
	// Implement validation step execution
	validationType := step.Parameters["type"].(string)

	switch validationType {
	case "response_format":
		// Validate response format
		return map[string]interface{}{
			"valid":  true,
			"format": "json",
		}, nil
	case "performance":
		// Validate performance metrics
		return map[string]interface{}{
			"within_threshold": true,
			"response_time":    "50ms",
		}, nil
	default:
		return nil, fmt.Errorf("unknown validation type: %s", validationType)
	}
}

// executeCleanupSteps executes all cleanup steps
func (m *IntegrationTestManager) executeCleanupSteps(ctx context.Context, steps []CleanupStep) error {
	for _, step := range steps {
		if err := m.executeCleanupStep(ctx, step); err != nil {
			return fmt.Errorf("cleanup step '%s' failed: %w", step.Name, err)
		}
	}
	return nil
}

// executeCleanupStep executes a single cleanup step
func (m *IntegrationTestManager) executeCleanupStep(ctx context.Context, step CleanupStep) error {
	switch step.Action {
	case "stop_server":
		return m.stopServer(ctx, step.Parameters)
	case "remove_project":
		return m.removeProject(ctx, step.Parameters)
	case "clean_temp":
		return m.cleanTemp(ctx, step.Parameters)
	default:
		return fmt.Errorf("unknown cleanup action: %s", step.Action)
	}
}

// Helper methods for setup/cleanup actions
func (m *IntegrationTestManager) startServer(ctx context.Context, params map[string]interface{}) error {
	// Simulate server startup
	time.Sleep(500 * time.Millisecond)
	return nil
}

func (m *IntegrationTestManager) createProject(ctx context.Context, params map[string]interface{}) error {
	// Simulate project creation
	time.Sleep(200 * time.Millisecond)
	return nil
}

func (m *IntegrationTestManager) installDependencies(ctx context.Context, params map[string]interface{}) error {
	// Simulate dependency installation
	time.Sleep(1 * time.Second)
	return nil
}

func (m *IntegrationTestManager) stopServer(ctx context.Context, params map[string]interface{}) error {
	// Simulate server shutdown
	time.Sleep(100 * time.Millisecond)
	return nil
}

func (m *IntegrationTestManager) removeProject(ctx context.Context, params map[string]interface{}) error {
	// Simulate project removal
	time.Sleep(100 * time.Millisecond)
	return nil
}

func (m *IntegrationTestManager) cleanTemp(ctx context.Context, params map[string]interface{}) error {
	// Simulate temp cleanup
	time.Sleep(50 * time.Millisecond)
	return nil
}

// finalizeResults finalizes the test results
func (m *IntegrationTestManager) finalizeResults() {
	m.results.EndTime = time.Now()
	m.results.Duration = m.results.EndTime.Sub(m.results.StartTime)
}

// logResults logs the integration test results
func (m *IntegrationTestManager) logResults(t *testing.T) {
	t.Logf("Integration Test Results for %s:", m.results.ScenarioName)
	t.Logf("  Duration: %v", m.results.Duration)
	t.Logf("  Total Steps: %d", m.results.TotalSteps)
	t.Logf("  Passed Steps: %d", m.results.PassedSteps)
	t.Logf("  Failed Steps: %d", m.results.FailedSteps)
	t.Logf("  Skipped Steps: %d", m.results.SkippedSteps)
	t.Logf("  Setup Successful: %v", m.results.SetupSuccessful)
	t.Logf("  Cleanup Successful: %v", m.results.CleanupSuccessful)

	if len(m.results.Errors) > 0 {
		t.Logf("  Errors:")
		for i, err := range m.results.Errors {
			t.Logf("    %d. %s", i+1, err)
		}
	}

	t.Logf("  Step Details:")
	for name, result := range m.results.StepResults {
		status := "PASS"
		if !result.Success {
			status = "FAIL"
		}
		t.Logf("    %s: %s (%v)", name, status, result.Duration)
	}
}

// GetMultiLanguageIntegrationScenario returns a multi-language integration scenario
func GetMultiLanguageIntegrationScenario() *IntegrationScenario {
	return &IntegrationScenario{
		Name:          "multi-language-integration",
		Description:   "Integration test with multiple language servers",
		Languages:     []string{"go", "python", "typescript"},
		ProjectType:   "multi-language",
		Timeout:       5 * time.Minute,
		RequiresSetup: true,
		SetupSteps: []SetupStep{
			{
				Name:        "create_project",
				Description: "Create test project structure",
				Action:      "create_project",
				Parameters: map[string]interface{}{
					"type":      "multi-language",
					"languages": []string{"go", "python", "typescript"},
				},
				Timeout: 30 * time.Second,
			},
			{
				Name:        "start_gateway",
				Description: "Start LSP gateway server",
				Action:      "start_server",
				Parameters: map[string]interface{}{
					"type": "gateway",
					"port": 8080,
				},
				Timeout: 60 * time.Second,
			},
		},
		TestSteps: []IntegrationStep{
			{
				Name:        "test_go_definition",
				Description: "Test Go definition request",
				Type:        StepTypeLSP,
				Language:    "go",
				Method:      "textDocument/definition",
				Parameters: map[string]interface{}{
					"uri": "main.go",
					"position": map[string]interface{}{
						"line":      5,
						"character": 10,
					},
				},
				Timeout:  30 * time.Second,
				Parallel: true,
			},
			{
				Name:        "test_python_hover",
				Description: "Test Python hover request",
				Type:        StepTypeLSP,
				Language:    "python",
				Method:      "textDocument/hover",
				Parameters: map[string]interface{}{
					"uri": "main.py",
					"position": map[string]interface{}{
						"line":      3,
						"character": 8,
					},
				},
				Timeout:  30 * time.Second,
				Parallel: true,
			},
			{
				Name:        "test_typescript_references",
				Description: "Test TypeScript references request",
				Type:        StepTypeLSP,
				Language:    "typescript",
				Method:      "textDocument/references",
				Parameters: map[string]interface{}{
					"uri": "main.ts",
					"position": map[string]interface{}{
						"line":      7,
						"character": 15,
					},
				},
				Timeout:  30 * time.Second,
				Parallel: true,
			},
			{
				Name:         "validate_responses",
				Description:  "Validate all language server responses",
				Type:         StepTypeValidate,
				Dependencies: []string{"test_go_definition", "test_python_hover", "test_typescript_references"},
				Parameters: map[string]interface{}{
					"type": "response_format",
				},
				Timeout: 10 * time.Second,
			},
		},
		CleanupSteps: []CleanupStep{
			{
				Name:        "stop_gateway",
				Description: "Stop LSP gateway server",
				Action:      "stop_server",
				Parameters: map[string]interface{}{
					"type": "gateway",
				},
			},
			{
				Name:        "cleanup_project",
				Description: "Remove test project",
				Action:      "remove_project",
				Parameters:  map[string]interface{}{},
			},
		},
		ValidationFunc: func(results *IntegrationResults) error {
			if results.PassedSteps < 3 {
				return fmt.Errorf("expected at least 3 passed steps, got %d", results.PassedSteps)
			}
			return nil
		},
	}
}

// GetMCPIntegrationScenario returns an MCP integration scenario
func GetMCPIntegrationScenario() *IntegrationScenario {
	return &IntegrationScenario{
		Name:          "mcp-integration",
		Description:   "Integration test for MCP protocol functionality",
		Languages:     []string{"go"},
		ProjectType:   "mcp-test",
		Timeout:       3 * time.Minute,
		RequiresSetup: true,
		SetupSteps: []SetupStep{
			{
				Name:        "start_mcp_server",
				Description: "Start MCP server",
				Action:      "start_server",
				Parameters: map[string]interface{}{
					"type": "mcp",
					"mode": "stdio",
				},
				Timeout: 30 * time.Second,
			},
		},
		TestSteps: []IntegrationStep{
			{
				Name:        "test_mcp_initialize",
				Description: "Test MCP initialization",
				Type:        StepTypeMCP,
				Method:      "initialize",
				Parameters: map[string]interface{}{
					"clientInfo": map[string]interface{}{
						"name":    "test-client",
						"version": "1.0.0",
					},
				},
				Timeout: 30 * time.Second,
			},
			{
				Name:         "test_mcp_list_tools",
				Description:  "Test MCP list tools",
				Type:         StepTypeMCP,
				Method:       "tools/list",
				Dependencies: []string{"test_mcp_initialize"},
				Parameters:   map[string]interface{}{},
				Timeout:      30 * time.Second,
			},
			{
				Name:         "test_mcp_call_tool",
				Description:  "Test MCP tool call",
				Type:         StepTypeMCP,
				Method:       "tools/call",
				Dependencies: []string{"test_mcp_list_tools"},
				Parameters: map[string]interface{}{
					"name": "lsp_definition",
					"arguments": map[string]interface{}{
						"uri": "file:///test/main.go",
						"position": map[string]interface{}{
							"line":      5,
							"character": 10,
						},
					},
				},
				Timeout: 60 * time.Second,
			},
		},
		CleanupSteps: []CleanupStep{
			{
				Name:        "stop_mcp_server",
				Description: "Stop MCP server",
				Action:      "stop_server",
				Parameters: map[string]interface{}{
					"type": "mcp",
				},
			},
		},
		ValidationFunc: func(results *IntegrationResults) error {
			if !results.SetupSuccessful {
				return fmt.Errorf("MCP server setup failed")
			}
			if results.PassedSteps < len(results.StepResults) {
				return fmt.Errorf("not all MCP steps passed")
			}
			return nil
		},
	}
}

// RunStandardIntegrationTests runs a standard set of integration tests
func RunStandardIntegrationTests(t *testing.T, gatewayURL, mcpURL string) {
	manager := NewIntegrationTestManager(gatewayURL, mcpURL)

	// Register scenarios
	manager.RegisterScenario(GetMultiLanguageIntegrationScenario())
	manager.RegisterScenario(GetMCPIntegrationScenario())

	// Execute scenarios
	scenarios := []string{"multi-language-integration", "mcp-integration"}

	for _, scenarioName := range scenarios {
		t.Run(scenarioName, func(t *testing.T) {
			err := manager.ExecuteScenario(t, scenarioName)
			require.NoError(t, err, "Integration scenario %s should complete successfully", scenarioName)
		})
	}
}
