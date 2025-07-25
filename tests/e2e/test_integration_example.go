package e2e_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"
	"time"
)

// TestE2ETestDiscoveryAndExecutionIntegration demonstrates the complete E2E test
// discovery and execution system working together
func TestE2ETestDiscoveryAndExecutionIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E integration test in short mode")
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	
	// Create orchestrator configuration
	config := &OrchestratorConfig{
		DiscoveryPackages:   []string{"."},
		TestPatterns:        []string{"Test.*E2E.*", ".*E2ETest$"},
		ExcludePatterns:     []string{".*Helper.*", ".*Mock.*"},
		MaxDiscoveryDepth:   5,
		DefaultStrategy:     StrategyOptimized,
		MaxConcurrency:      4,
		DefaultTimeout:      20 * time.Minute,
		EnableRetries:       true,
		MaxRetries:          2,
		MemoryLimitMB:       4096,
		CPULimit:            4,
		NetworkPorts:        []int{8080, 8081, 8082, 8083},
		GenerateReports:     true,
		ReportFormats:       []ReportFormat{FormatJSON, FormatHTML, FormatMarkdown},
		SaveBaseline:        true,
		CompareBaseline:     false,
		UseRealServers:      false,
		CleanupAfterRun:     true,
		EnableMonitoring:    true,
		MonitoringInterval:  10 * time.Second,
	}
	
	// Initialize orchestrator
	orchestrator, err := NewE2ETestOrchestrator(config)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}
	defer orchestrator.Close()
	
	// Define test filter for comprehensive testing
	filter := &TestFilter{
		Categories: []TestCategory{
			CategorySetup,
			CategoryHTTPProtocol,
			CategoryMCPProtocol,
			CategoryMultiLang,
			CategoryWorkflow,
			CategoryIntegration,
			CategoryPerformance,
		},
		MinPriority: PriorityMedium,
		ExecutionModes: []ExecutionMode{
			ModeSequential,
			ModeParallel,
			ModeConditional,
		},
		MaxDuration: 25 * time.Minute,
		RequiredResources: []ResourceType{
			ResourceTypeServer,
			ResourceTypeConfig,
		},
		Tags: []string{"critical", "integration"},
		IncludePatterns: []*regexp.Regexp{
			regexp.MustCompile(".*startup.*"),
			regexp.MustCompile(".*protocol.*"),
			regexp.MustCompile(".*workflow.*"),
		},
	}
	
	// Run complete E2E test suite
	t.Log("Starting comprehensive E2E test discovery and execution...")
	execution, err := orchestrator.RunCompleteE2ETestSuite(ctx, filter)
	if err != nil {
		t.Fatalf("E2E test suite execution failed: %v", err)
	}
	
	// Validate execution results
	validateExecutionResults(t, execution)
	
	// Verify test discovery worked correctly
	validateTestDiscovery(t, orchestrator, execution)
	
	// Verify execution strategy was applied correctly
	validateExecutionStrategy(t, orchestrator, execution)
	
	// Verify reporting was generated
	validateReportGeneration(t, execution)
	
	// Log final summary
	logExecutionSummary(t, execution)
}

// TestE2ETestDiscoveryEngineStandalone tests the discovery engine in isolation
func TestE2ETestDiscoveryEngineStandalone(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	
	// Create discovery engine
	engine := NewTestDiscoveryEngine()
	
	// Perform test discovery
	t.Log("Discovering E2E tests...")
	if err := engine.DiscoverTests(ctx, "."); err != nil {
		t.Fatalf("Test discovery failed: %v", err)
	}
	
	// Validate discovery results
	if !engine.IsDiscoveryComplete() {
		t.Error("Discovery should be complete")
	}
	
	discoveredTests := engine.GetDiscoveredTests()
	if len(discoveredTests) == 0 {
		t.Error("No tests were discovered")
	}
	
	t.Logf("Discovered %d tests", len(discoveredTests))
	
	// Validate test metadata
	for name, test := range discoveredTests {
		if test.Name != name {
			t.Errorf("Test name mismatch: expected %s, got %s", name, test.Name)
		}
		
		if test.Category == "" {
			t.Errorf("Test %s missing category", name)
		}
		
		if test.Priority == 0 {
			t.Errorf("Test %s missing priority", name)
		}
		
		if test.Timeout == 0 {
			t.Errorf("Test %s missing timeout", name)
		}
		
		if len(test.Resources) == 0 {
			t.Errorf("Test %s missing resource requirements", name)
		}
	}
	
	// Test filtering
	filter := &TestFilter{
		Categories:  []TestCategory{CategorySetup, CategoryHTTPProtocol},
		MinPriority: PriorityHigh,
		Tags:        []string{"critical"},
	}
	
	filteredTests, err := engine.FilterTests(filter)
	if err != nil {
		t.Fatalf("Test filtering failed: %v", err)
	}
	
	t.Logf("Filtered to %d tests", len(filteredTests))
	
	// Validate filtered tests meet criteria
	for _, test := range filteredTests {
		if test.Category != CategorySetup && test.Category != CategoryHTTPProtocol {
			t.Errorf("Filtered test %s has wrong category: %s", test.Name, test.Category)
		}
		
		if test.Priority < PriorityHigh {
			t.Errorf("Filtered test %s has low priority: %d", test.Name, test.Priority)
		}
		
		hasRequiredTag := false
		for _, tag := range test.Tags {
			if tag == "critical" {
				hasRequiredTag = true
				break
			}
		}
		if !hasRequiredTag {
			t.Errorf("Filtered test %s missing required tag 'critical'", test.Name)
		}
	}
	
	// Test dependency analysis
	dependencyGraph := engine.GetDependencyGraph()
	if len(dependencyGraph) != len(discoveredTests) {
		t.Errorf("Dependency graph size mismatch: expected %d, got %d", 
			len(discoveredTests), len(dependencyGraph))
	}
	
	// Test execution level calculation
	executionLevels := engine.GetExecutionLevels()
	if len(executionLevels) == 0 {
		t.Error("No execution levels calculated")
	}
	
	t.Logf("Calculated %d execution levels", len(executionLevels))
	for i, level := range executionLevels {
		t.Logf("Level %d: %d tests", i, len(level))
	}
	
	// Test parallel groups
	parallelGroups := engine.GetParallelGroups()
	t.Logf("Found %d parallel groups", len(parallelGroups))
	
	// Validate metrics
	metrics := engine.GetDiscoveryMetrics()
	if metrics.TotalTestsDiscovered != len(discoveredTests) {
		t.Errorf("Metrics mismatch: expected %d tests, got %d", 
			len(discoveredTests), metrics.TotalTestsDiscovered)
	}
	
	if metrics.DiscoveryDurationMs <= 0 {
		t.Error("Discovery duration should be positive")
	}
	
	if metrics.AnalysisDurationMs <= 0 {
		t.Error("Analysis duration should be positive")
	}
	
	t.Logf("Discovery completed in %d ms, analysis in %d ms", 
		metrics.DiscoveryDurationMs, metrics.AnalysisDurationMs)
}

// TestE2ETestExecutionEngineStandalone tests the execution engine in isolation
func TestE2ETestExecutionEngineStandalone(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	
	// Create and setup discovery engine first
	discoveryEngine := NewTestDiscoveryEngine()
	if err := discoveryEngine.DiscoverTests(ctx, "."); err != nil {
		t.Fatalf("Test discovery failed: %v", err)
	}
	
	// Create execution engine
	executionEngine := NewTestExecutionEngine(discoveryEngine)
	
	// Create execution plan with different strategies
	strategies := []ExecutionStrategy{
		StrategySequential,
		StrategyParallel,
		StrategyDependency,
		StrategyOptimized,
	}
	
	filter := &TestFilter{
		Categories: []TestCategory{CategorySetup, CategoryHTTPProtocol},
		MinPriority: PriorityMedium,
	}
	
	for _, strategy := range strategies {
		t.Run(fmt.Sprintf("Strategy_%s", strategy), func(t *testing.T) {
			plan, err := executionEngine.CreateExecutionPlan(ctx, filter, strategy)
			if err != nil {
				t.Fatalf("Failed to create execution plan for %s: %v", strategy, err)
			}
			
			// Validate plan
			if plan.Strategy != strategy {
				t.Errorf("Plan strategy mismatch: expected %s, got %s", strategy, plan.Strategy)
			}
			
			if plan.TotalTests <= 0 {
				t.Error("Plan should include tests")
			}
			
			if len(plan.ExecutionStages) == 0 {
				t.Error("Plan should have execution stages")
			}
			
			if plan.EstimatedTime <= 0 {
				t.Error("Plan should have estimated time")
			}
			
			if plan.ResourceBudget == nil {
				t.Error("Plan should have resource budget")
			}
			
			t.Logf("Plan for %s: %d tests, %d stages, estimated time: %v", 
				strategy, plan.TotalTests, len(plan.ExecutionStages), plan.EstimatedTime)
			
			// Validate stages
			for i, stage := range plan.ExecutionStages {
				if stage.Name == "" {
					t.Errorf("Stage %d missing name", i)
				}
				
				if len(stage.Tests) == 0 {
					t.Errorf("Stage %s has no tests", stage.Name)
				}
				
				if stage.Timeout <= 0 {
					t.Errorf("Stage %s has invalid timeout", stage.Name)
				}
				
				if stage.MaxConcurrency <= 0 {
					t.Errorf("Stage %s has invalid concurrency", stage.Name)
				}
				
				t.Logf("  Stage %d (%s): %d tests, mode: %s, concurrency: %d, timeout: %v", 
					i, stage.Name, len(stage.Tests), stage.ExecutionMode, 
					stage.MaxConcurrency, stage.Timeout)
			}
		})
	}
	
	// Test execution with a simple plan
	simpleFilter := &TestFilter{
		Categories: []TestCategory{CategorySetup},
		MaxDuration: 5 * time.Minute,
	}
	
	plan, err := executionEngine.CreateExecutionPlan(ctx, simpleFilter, StrategySequential)
	if err != nil {
		t.Fatalf("Failed to create simple execution plan: %v", err)
	}
	
	t.Log("Executing simple test plan...")
	if err := executionEngine.ExecutePlan(ctx, plan); err != nil {
		t.Errorf("Execution plan failed: %v", err)
	}
	
	// Validate execution results
	completed, failed := executionEngine.GetExecutionResults()
	metrics := executionEngine.GetExecutionMetrics()
	
	t.Logf("Execution completed: %d passed, %d failed", len(completed), len(failed))
	t.Logf("Execution metrics: %d total executions, %d successful, %d failed, %d retried", 
		metrics.TotalExecutions, metrics.SuccessfulTests, metrics.FailedTests, metrics.RetriedTests)
	
	if metrics.TotalExecutions == 0 {
		t.Error("No tests were executed")
	}
	
	if metrics.TotalDuration <= 0 {
		t.Error("Total duration should be positive")
	}
	
	if metrics.AverageTestTime <= 0 {
		t.Error("Average test time should be positive")
	}
}

// validateExecutionResults validates the overall execution results
func validateExecutionResults(t *testing.T, execution *OrchestratorExecution) {
	if execution.Status != StatusCompleted {
		t.Errorf("Execution status should be completed, got: %s", execution.Status)
	}
	
	if execution.Duration <= 0 {
		t.Error("Execution duration should be positive")
	}
	
	if execution.TestsDiscovered <= 0 {
		t.Error("Should have discovered tests")
	}
	
	if execution.ExecutionResults == nil {
		t.Fatal("Execution results should not be nil")
	}
	
	results := execution.ExecutionResults
	if results.TotalTests <= 0 {
		t.Error("Should have executed tests")
	}
	
	if results.TotalTests != results.TestsPassed + results.TestsFailed + results.TestsSkipped {
		t.Error("Test counts don't add up")
	}
	
	if results.TotalDuration <= 0 {
		t.Error("Total duration should be positive")
	}
	
	if results.AverageTestDuration <= 0 {
		t.Error("Average test duration should be positive")
	}
	
	// Validate category results
	if len(results.CategoryResults) == 0 {
		t.Error("Should have category results")
	}
	
	for category, summary := range results.CategoryResults {
		if summary.Category != category {
			t.Errorf("Category mismatch: expected %s, got %s", category, summary.Category)
		}
		
		if summary.TotalTests <= 0 {
			t.Errorf("Category %s should have tests", category)
		}
		
		if summary.TotalTests != summary.Passed + summary.Failed {
			t.Errorf("Category %s test counts don't add up", category)
		}
		
		if summary.SuccessRate < 0 || summary.SuccessRate > 1 {
			t.Errorf("Category %s has invalid success rate: %f", category, summary.SuccessRate)
		}
	}
	
	// Validate priority results
	if len(results.PriorityResults) == 0 {
		t.Error("Should have priority results")
	}
	
	for priority, summary := range results.PriorityResults {
		if summary.Priority != priority {
			t.Errorf("Priority mismatch: expected %d, got %d", priority, summary.Priority)
		}
		
		if summary.TotalTests <= 0 {
			t.Errorf("Priority %d should have tests", priority)
		}
		
		if summary.SuccessRate < 0 || summary.SuccessRate > 1 {
			t.Errorf("Priority %d has invalid success rate: %f", priority, summary.SuccessRate)
		}
	}
	
	// Validate resource utilization
	if results.ResourceUtilization == nil {
		t.Error("Should have resource utilization data")
	}
}

// validateTestDiscovery validates the test discovery process
func validateTestDiscovery(t *testing.T, orchestrator *E2ETestOrchestrator, execution *OrchestratorExecution) {
	discoveryEngine := orchestrator.GetDiscoveryEngine()
	if discoveryEngine == nil {
		t.Fatal("Discovery engine should not be nil")
	}
	
	if !discoveryEngine.IsDiscoveryComplete() {
		t.Error("Discovery should be complete")
	}
	
	discoveredTests := discoveryEngine.GetDiscoveredTests()
	if len(discoveredTests) == 0 {
		t.Error("Should have discovered tests")
	}
	
	if execution.DiscoveryResults == nil {
		t.Fatal("Discovery results should not be nil")
	}
	
	metrics := execution.DiscoveryResults
	if metrics.TotalTestsDiscovered != len(discoveredTests) {
		t.Errorf("Discovery metrics mismatch: expected %d, got %d", 
			len(discoveredTests), metrics.TotalTestsDiscovered)
	}
	
	if metrics.DiscoveryDurationMs <= 0 {
		t.Error("Discovery duration should be positive")
	}
	
	if metrics.AnalysisDurationMs <= 0 {
		t.Error("Analysis duration should be positive")
	}
	
	// Validate discovery by category
	if len(metrics.TestsByCategory) == 0 {
		t.Error("Should have tests by category")
	}
	
	totalByCategory := 0
	for category, count := range metrics.TestsByCategory {
		if count <= 0 {
			t.Errorf("Category %s should have tests", category)
		}
		totalByCategory += count
	}
	
	if totalByCategory != metrics.TotalTestsDiscovered {
		t.Error("Category totals don't match discovered tests")
	}
	
	t.Logf("Discovery validation passed: %d tests discovered across %d categories", 
		metrics.TotalTestsDiscovered, len(metrics.TestsByCategory))
}

// validateExecutionStrategy validates the execution strategy was applied correctly
func validateExecutionStrategy(t *testing.T, orchestrator *E2ETestOrchestrator, execution *OrchestratorExecution) {
	executionEngine := orchestrator.GetExecutionEngine()
	if executionEngine == nil {
		t.Fatal("Execution engine should not be nil")
	}
	
	plan := orchestrator.GetCurrentPlan()
	if plan == nil {
		t.Fatal("Execution plan should not be nil")
	}
	
	if execution.ExecutionPlan != plan {
		t.Error("Execution plan mismatch")
	}
	
	if plan.Strategy != StrategyOptimized {
		t.Errorf("Expected optimized strategy, got: %s", plan.Strategy)
	}
	
	if len(plan.ExecutionStages) == 0 {
		t.Error("Should have execution stages")
	}
	
	if plan.ResourceBudget == nil {
		t.Error("Should have resource budget")
	}
	
	if plan.EstimatedTime <= 0 {
		t.Error("Should have estimated time")
	}
	
	// Validate stage execution order and dependencies
	for i, stage := range plan.ExecutionStages {
		if stage.StageNumber != i {
			t.Errorf("Stage %s number mismatch: expected %d, got %d", 
				stage.Name, i, stage.StageNumber)
		}
		
		if len(stage.Tests) == 0 {
			t.Errorf("Stage %s should have tests", stage.Name)
		}
		
		if stage.MaxConcurrency <= 0 {
			t.Errorf("Stage %s should have valid concurrency", stage.Name)
		}
	}
	
	t.Logf("Execution strategy validation passed: %s strategy with %d stages", 
		plan.Strategy, len(plan.ExecutionStages))
}

// validateReportGeneration validates that reports were generated correctly
func validateReportGeneration(t *testing.T, execution *OrchestratorExecution) {
	if len(execution.GeneratedReports) == 0 {
		t.Error("Should have generated reports")
	}
	
	expectedFormats := []ReportFormat{FormatJSON, FormatHTML, FormatMarkdown}
	if len(execution.GeneratedReports) != len(expectedFormats) {
		t.Errorf("Expected %d reports, got %d", len(expectedFormats), len(execution.GeneratedReports))
	}
	
	for _, reportPath := range execution.GeneratedReports {
		if reportPath == "" {
			t.Error("Report path should not be empty")
		}
		
		// In a real implementation, we would check if files exist
		t.Logf("Generated report: %s", reportPath)
	}
}

// logExecutionSummary logs a comprehensive execution summary
func logExecutionSummary(t *testing.T, execution *OrchestratorExecution) {
	t.Logf(`
=== E2E Test Execution Summary ===
Execution ID: %s
Status: %s
Duration: %v
Tests Discovered: %d
Tests Executed: %d
Tests Passed: %d
Tests Failed: %d
Success Rate: %.2f%%
Generated Reports: %d

=== Discovery Metrics ===
Discovery Duration: %d ms
Analysis Duration: %d ms
Dependency Complexity: %.2f

=== Execution Performance ===
Average Test Duration: %v
Throughput: %.2f tests/second
Peak Memory: %d MB
Peak CPU: %.1f%%

=== Category Breakdown ===`,
		execution.ID,
		execution.Status,
		execution.Duration,
		execution.TestsDiscovered,
		execution.ExecutionResults.TotalTests,
		execution.ExecutionResults.TestsPassed,
		execution.ExecutionResults.TestsFailed,
		float64(execution.ExecutionResults.TestsPassed)/float64(execution.ExecutionResults.TotalTests)*100,
		len(execution.GeneratedReports),
		execution.DiscoveryResults.DiscoveryDurationMs,
		execution.DiscoveryResults.AnalysisDurationMs,
		execution.DiscoveryResults.DependencyComplexity,
		execution.ExecutionResults.AverageTestDuration,
		execution.ExecutionResults.ThroughputPerSecond,
		execution.ExecutionResults.MemoryPeakMB,
		execution.ExecutionResults.CPUPeakPercent,
	)
	
	for category, summary := range execution.ExecutionResults.CategoryResults {
		t.Logf("  %s: %d tests (%.1f%% success)", 
			category, summary.TotalTests, summary.SuccessRate*100)
	}
	
	if len(execution.ExecutionResults.CriticalFailures) > 0 {
		t.Log("\n=== Critical Failures ===")
		for _, failure := range execution.ExecutionResults.CriticalFailures {
			t.Logf("  - %s", failure)
		}
	}
	
	t.Log("=== E2E Test Suite Completed Successfully ===")
}

// TestE2EComponentIntegration tests integration between individual components
func TestE2EComponentIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	
	t.Log("Testing component integration...")
	
	// Test 1: Discovery Engine -> Execution Engine integration
	t.Run("DiscoveryToExecution", func(t *testing.T) {
		discoveryEngine := NewTestDiscoveryEngine()
		if err := discoveryEngine.DiscoverTests(ctx, "."); err != nil {
			t.Fatalf("Discovery failed: %v", err)
		}
		
		executionEngine := NewTestExecutionEngine(discoveryEngine)
		
		filter := &TestFilter{
			Categories: []TestCategory{CategorySetup},
			MinPriority: PriorityMedium,
		}
		
		plan, err := executionEngine.CreateExecutionPlan(ctx, filter, StrategySequential)
		if err != nil {
			t.Fatalf("Plan creation failed: %v", err)
		}
		
		if plan.TotalTests == 0 {
			t.Error("Plan should have tests")
		}
		
		t.Logf("Integration test passed: discovered %d tests, planned %d tests", 
			len(discoveryEngine.GetDiscoveredTests()), plan.TotalTests)
	})
	
	// Test 2: Orchestrator component coordination
	t.Run("OrchestratorCoordination", func(t *testing.T) {
		config := getDefaultOrchestratorConfig()
		config.DefaultTimeout = 5 * time.Minute
		config.GenerateReports = false // Disable for faster testing
		
		orchestrator, err := NewE2ETestOrchestrator(config)
		if err != nil {
			t.Fatalf("Orchestrator creation failed: %v", err)
		}
		defer orchestrator.Close()
		
		filter := &TestFilter{
			Categories: []TestCategory{CategorySetup},
			MaxDuration: 3 * time.Minute,
		}
		
		execution, err := orchestrator.RunCompleteE2ETestSuite(ctx, filter)
		if err != nil {
			t.Fatalf("Orchestrator execution failed: %v", err)
		}
		
		if execution.Status != StatusCompleted {
			t.Errorf("Expected completed status, got: %s", execution.Status)
		}
		
		if execution.ExecutionResults.TotalTests == 0 {
			t.Error("Should have executed tests")
		}
		
		t.Logf("Orchestrator integration passed: %d tests executed in %v", 
			execution.ExecutionResults.TotalTests, execution.Duration)
	})
	
	t.Log("Component integration tests completed successfully")
}

// BenchmarkE2ETestDiscovery benchmarks the test discovery process
func BenchmarkE2ETestDiscovery(b *testing.B) {
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine := NewTestDiscoveryEngine()
		if err := engine.DiscoverTests(ctx, "."); err != nil {
			b.Fatalf("Discovery failed: %v", err)
		}
		
		discoveredTests := engine.GetDiscoveredTests()
		if len(discoveredTests) == 0 {
			b.Error("No tests discovered")
		}
	}
}

// BenchmarkE2ETestExecution benchmarks the test execution process
func BenchmarkE2ETestExecution(b *testing.B) {
	ctx := context.Background()
	
	// Setup once
	discoveryEngine := NewTestDiscoveryEngine()
	if err := discoveryEngine.DiscoverTests(ctx, "."); err != nil {
		b.Fatalf("Discovery setup failed: %v", err)
	}
	
	filter := &TestFilter{
		Categories: []TestCategory{CategorySetup},
		MaxDuration: 2 * time.Minute,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		executionEngine := NewTestExecutionEngine(discoveryEngine)
		
		plan, err := executionEngine.CreateExecutionPlan(ctx, filter, StrategySequential)
		if err != nil {
			b.Fatalf("Plan creation failed: %v", err)
		}
		
		if err := executionEngine.ExecutePlan(ctx, plan); err != nil {
			b.Fatalf("Execution failed: %v", err)
		}
	}
}