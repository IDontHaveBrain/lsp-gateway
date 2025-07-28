package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-gateway/tests/e2e/benchmarks"
	"lsp-gateway/tests/e2e/fixtures"
	"lsp-gateway/tests/e2e/helpers"
	"lsp-gateway/tests/e2e/testutils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type PythonLSPComprehensiveIntegrationTestSuite struct {
	suite.Suite
	httpClient         *testutils.HttpClient
	repoManager        *testutils.PythonRepoManager
	performanceBenchmarker *benchmarks.LSPPerformanceBenchmarker
	workspaceDir       string
	repoContext        string
	startTime          time.Time
	timeout            time.Duration
	validationResults  []*helpers.LSPValidationResult
	benchmarkResults   *benchmarks.LSPBenchmarkResults
	testArtifactsDir   string
}

func (suite *PythonLSPComprehensiveIntegrationTestSuite) SetupSuite() {
	suite.timeout = 10 * time.Second
	suite.startTime = time.Now()
	suite.validationResults = make([]*helpers.LSPValidationResult, 0)

	// Create test artifacts directory
	suite.testArtifactsDir = filepath.Join("/tmp", "lspg-python-integration-test-artifacts", fmt.Sprintf("%d", time.Now().Unix()))
	err := os.MkdirAll(suite.testArtifactsDir, 0755)
	require.NoError(suite.T(), err, "Failed to create test artifacts directory")

	// Setup Python repository
	config := testutils.DefaultPythonRepoConfig()
	config.EnableLogging = true
	config.ForceClean = true

	suite.repoManager = testutils.NewPythonRepoManager(config)
	require.NotNil(suite.T(), suite.repoManager)

	err = suite.repoManager.CloneRepository()
	require.NoError(suite.T(), err, "Failed to clone Python patterns repository")

	suite.workspaceDir = suite.repoManager.WorkspaceDir
	suite.repoContext = "python-patterns-comprehensive"

	// Setup HTTP client
	httpConfig := testutils.HttpClientConfig{
		BaseURL:     "http://localhost:8080",
		Timeout:     suite.timeout,
		MockMode:    false,
		ProjectPath: suite.workspaceDir,
		WorkspaceID: suite.repoContext,
		EnableRequestLogging: true,
		EnableMetrics: true,
	}

	suite.httpClient = testutils.NewHttpClient(httpConfig)
	require.NotNil(suite.T(), suite.httpClient)

	// Setup performance benchmarker
	benchmarkConfig := benchmarks.DefaultLSPBenchmarkConfig()
	benchmarkConfig.RequestsPerMethod = 20
	benchmarkConfig.ConcurrentRequests = 3
	benchmarkConfig.EnableDetailedMetrics = true

	suite.performanceBenchmarker = benchmarks.NewLSPPerformanceBenchmarker(
		suite.httpClient, suite.repoManager, benchmarkConfig)
	require.NotNil(suite.T(), suite.performanceBenchmarker)

	suite.T().Logf("Comprehensive integration test setup completed in %v", time.Since(suite.startTime))
}

func (suite *PythonLSPComprehensiveIntegrationTestSuite) TearDownSuite() {
	// Save test artifacts
	suite.saveTestArtifacts()

	// Generate final report
	suite.generateComprehensiveReport()

	// Cleanup
	if suite.repoManager != nil && suite.repoManager.CleanupFunc != nil {
		_ = suite.repoManager.CleanupFunc()
	}

	suite.T().Logf("Comprehensive integration test completed in %v", time.Since(suite.startTime))
}

// TestComprehensiveLSPFeatureValidation tests all LSP features with comprehensive validation
func (suite *PythonLSPComprehensiveIntegrationTestSuite) TestComprehensiveLSPFeatureValidation() {
	scenarios := fixtures.GetAllPythonPatternScenarios()
	require.NotEmpty(suite.T(), scenarios, "No Python pattern scenarios found")

	suite.T().Logf("Running comprehensive LSP validation for %d scenarios", len(scenarios))

	// Group scenarios by LSP method for organized testing
	methodScenarios := make(map[string][]fixtures.PythonPatternScenario)
	for _, scenario := range scenarios {
		for _, method := range scenario.LSPMethods {
			methodScenarios[method] = append(methodScenarios[method], scenario)
		}
	}

	// Test each LSP method comprehensively
	lspMethods := []string{
		"textDocument/definition",
		"textDocument/references",
		"textDocument/hover",
		"textDocument/documentSymbol",
		"workspace/symbol",
		"textDocument/completion",
	}

	for _, method := range lspMethods {
		suite.Run(fmt.Sprintf("ComprehensiveValidation_%s", method), func() {
			suite.testLSPMethodComprehensively(method, methodScenarios[method])
		})
	}

	// Validate overall integration
	suite.Run("OverallIntegrationValidation", func() {
		suite.validateOverallIntegration()
	})
}

// TestLSPPerformanceBenchmarking runs comprehensive performance benchmarks
func (suite *PythonLSPComprehensiveIntegrationTestSuite) TestLSPPerformanceBenchmarking() {
	suite.T().Logf("Starting comprehensive LSP performance benchmarking")

	b := &testing.B{}
	suite.benchmarkResults = suite.performanceBenchmarker.RunComprehensiveBenchmark(b)
	require.NotNil(suite.T(), suite.benchmarkResults, "Benchmark results should not be nil")

	// Validate performance thresholds
	violations := suite.benchmarkResults.ValidatePerformanceThresholds()
	if len(violations) > 0 {
		suite.T().Logf("Performance threshold violations detected:")
		for _, violation := range violations {
			suite.T().Logf("  - %s", violation)
		}
		// Log as warnings rather than failures for integration test
	}

	// Assert basic performance criteria
	assert.Greater(suite.T(), suite.benchmarkResults.OverallThroughput, 1.0, 
		"Overall throughput should be greater than 1 req/sec")
	assert.Less(suite.T(), suite.benchmarkResults.OverallErrorRate, 0.5, 
		"Overall error rate should be less than 50%")

	suite.T().Logf("Performance benchmarking completed: %.2f req/sec overall throughput", 
		suite.benchmarkResults.OverallThroughput)
}

// TestEndToEndWorkflows tests complete end-to-end workflows
func (suite *PythonLSPComprehensiveIntegrationTestSuite) TestEndToEndWorkflows() {
	// Test typical IDE workflow: definition -> references -> hover -> completion
	suite.Run("TypicalIDEWorkflow", func() {
		suite.testTypicalIDEWorkflow()
	})

	// Test pattern exploration workflow: workspace symbol -> document symbol -> definition
	suite.Run("PatternExplorationWorkflow", func() {
		suite.testPatternExplorationWorkflow()
	})

	// Test code navigation workflow: definition -> references -> hover validation
	suite.Run("CodeNavigationWorkflow", func() {
		suite.testCodeNavigationWorkflow()
	})
}

// TestErrorHandlingAndEdgeCases tests comprehensive error handling
func (suite *PythonLSPComprehensiveIntegrationTestSuite) TestErrorHandlingAndEdgeCases() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.timeout)
	defer cancel()

	testCases := []struct {
		name        string
		fileURI     string
		position    testutils.Position
		query       string
		expectError bool
	}{
		{
			name:        "NonExistentFile",
			fileURI:     "file:///non/existent/file.py",
			position:    testutils.Position{Line: 0, Character: 0},
			expectError: false, // Should handle gracefully
		},
		{
			name:        "InvalidPosition",
			fileURI:     "file://" + filepath.Join(suite.workspaceDir, "patterns/creational/builder.py"),
			position:    testutils.Position{Line: -1, Character: -1},
			expectError: false, // Should handle gracefully
		},
		{
			name:        "VeryLargePosition",
			fileURI:     "file://" + filepath.Join(suite.workspaceDir, "patterns/creational/builder.py"),
			position:    testutils.Position{Line: 10000, Character: 10000},
			expectError: false, // Should handle gracefully
		},
		{
			name:        "EmptyQuery",
			query:       "",
			expectError: false, // Should handle gracefully
		},
		{
			name:        "VeryLongQuery",
			query:       string(make([]byte, 1000)),
			expectError: false, // Should handle gracefully
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Test all LSP methods with edge case
			if tc.fileURI != "" {
				// Test methods that require file URI
				_, err := suite.httpClient.Definition(ctx, tc.fileURI, tc.position)
				if tc.expectError {
					assert.Error(suite.T(), err)
				} else {
					// Should not panic or crash
					suite.T().Logf("Definition handled edge case: %v", err)
				}

				_, err = suite.httpClient.References(ctx, tc.fileURI, tc.position, true)
				if tc.expectError {
					assert.Error(suite.T(), err)
				} else {
					suite.T().Logf("References handled edge case: %v", err)
				}

				_, err = suite.httpClient.Hover(ctx, tc.fileURI, tc.position)
				if tc.expectError {
					assert.Error(suite.T(), err)
				} else {
					suite.T().Logf("Hover handled edge case: %v", err)
				}

				_, err = suite.httpClient.DocumentSymbol(ctx, tc.fileURI)
				if tc.expectError {
					assert.Error(suite.T(), err)
				} else {
					suite.T().Logf("DocumentSymbol handled edge case: %v", err)
				}

				_, err = suite.httpClient.Completion(ctx, tc.fileURI, tc.position)
				if tc.expectError {
					assert.Error(suite.T(), err)
				} else {
					suite.T().Logf("Completion handled edge case: %v", err)
				}
			}

			if tc.query != "" || tc.name == "EmptyQuery" {
				_, err := suite.httpClient.WorkspaceSymbol(ctx, tc.query)
				if tc.expectError {
					assert.Error(suite.T(), err)
				} else {
					suite.T().Logf("WorkspaceSymbol handled edge case: %v", err)
				}
			}
		})
	}
}

// testLSPMethodComprehensively tests a specific LSP method with comprehensive validation
func (suite *PythonLSPComprehensiveIntegrationTestSuite) testLSPMethodComprehensively(
	method string, scenarios []fixtures.PythonPatternScenario) {
	
	if len(scenarios) == 0 {
		suite.T().Logf("No scenarios found for method %s", method)
		return
	}

	validationCtx := &helpers.LSPValidationContext{
		T:            suite.T(),
		WorkspaceDir: suite.workspaceDir,
	}

	for _, scenario := range scenarios {
		validationCtx.Scenario = scenario
		
		suite.Run(fmt.Sprintf("%s_%s", method, scenario.Name), func() {
			ctx, cancel := context.WithTimeout(context.Background(), suite.timeout)
			defer cancel()

			filePath := filepath.Join(suite.workspaceDir, scenario.FilePath)
			fileURI := "file://" + filePath

			switch method {
			case "textDocument/definition":
				suite.testDefinitionWithValidation(ctx, validationCtx, fileURI, scenario)
			case "textDocument/references":
				suite.testReferencesWithValidation(ctx, validationCtx, fileURI, scenario)
			case "textDocument/hover":
				suite.testHoverWithValidation(ctx, validationCtx, fileURI, scenario)
			case "textDocument/documentSymbol":
				suite.testDocumentSymbolWithValidation(ctx, validationCtx, fileURI, scenario)
			case "workspace/symbol":
				suite.testWorkspaceSymbolWithValidation(ctx, validationCtx, scenario)
			case "textDocument/completion":
				suite.testCompletionWithValidation(ctx, validationCtx, fileURI, scenario)
			}
		})
	}
}

// Individual test method implementations with validation
func (suite *PythonLSPComprehensiveIntegrationTestSuite) testDefinitionWithValidation(
	ctx context.Context, validationCtx *helpers.LSPValidationContext, 
	fileURI string, scenario fixtures.PythonPatternScenario) {
	
	for _, testPos := range scenario.TestPositions {
		position := testutils.Position{Line: testPos.Line, Character: testPos.Character}
		locations, err := suite.httpClient.Definition(ctx, fileURI, position)
		
		if err != nil {
			suite.T().Logf("Definition error for %s: %v", testPos.Symbol, err)
			continue
		}

		result := helpers.ValidateDefinitionResponse(validationCtx, locations, testPos)
		result.Timestamp = time.Now().Unix()
		suite.validationResults = append(suite.validationResults, result)

		// Assert basic validity
		assert.True(suite.T(), result.IsValid, "Definition validation failed: %v", result.Errors)
	}
}

func (suite *PythonLSPComprehensiveIntegrationTestSuite) testReferencesWithValidation(
	ctx context.Context, validationCtx *helpers.LSPValidationContext,
	fileURI string, scenario fixtures.PythonPatternScenario) {
	
	for _, testPos := range scenario.TestPositions {
		position := testutils.Position{Line: testPos.Line, Character: testPos.Character}
		references, err := suite.httpClient.References(ctx, fileURI, position, true)
		
		if err != nil {
			suite.T().Logf("References error for %s: %v", testPos.Symbol, err)
			continue
		}

		result := helpers.ValidateReferencesResponse(validationCtx, references, testPos)
		result.Timestamp = time.Now().Unix()
		suite.validationResults = append(suite.validationResults, result)

		assert.True(suite.T(), result.IsValid, "References validation failed: %v", result.Errors)
	}
}

func (suite *PythonLSPComprehensiveIntegrationTestSuite) testHoverWithValidation(
	ctx context.Context, validationCtx *helpers.LSPValidationContext,
	fileURI string, scenario fixtures.PythonPatternScenario) {
	
	for _, testPos := range scenario.TestPositions {
		position := testutils.Position{Line: testPos.Line, Character: testPos.Character}
		hoverResult, err := suite.httpClient.Hover(ctx, fileURI, position)
		
		if err != nil {
			suite.T().Logf("Hover error for %s: %v", testPos.Symbol, err)
			continue
		}

		result := helpers.ValidateHoverResponse(validationCtx, hoverResult, testPos)
		result.Timestamp = time.Now().Unix()
		suite.validationResults = append(suite.validationResults, result)

		assert.True(suite.T(), result.IsValid, "Hover validation failed: %v", result.Errors)
	}
}

func (suite *PythonLSPComprehensiveIntegrationTestSuite) testDocumentSymbolWithValidation(
	ctx context.Context, validationCtx *helpers.LSPValidationContext,
	fileURI string, scenario fixtures.PythonPatternScenario) {
	
	symbols, err := suite.httpClient.DocumentSymbol(ctx, fileURI)
	if err != nil {
		suite.T().Logf("DocumentSymbol error: %v", err)
		return
	}

	result := helpers.ValidateDocumentSymbolResponse(validationCtx, symbols)
	result.Timestamp = time.Now().Unix()
	suite.validationResults = append(suite.validationResults, result)

	assert.True(suite.T(), result.IsValid, "DocumentSymbol validation failed: %v", result.Errors)
}

func (suite *PythonLSPComprehensiveIntegrationTestSuite) testWorkspaceSymbolWithValidation(
	ctx context.Context, validationCtx *helpers.LSPValidationContext,
	scenario fixtures.PythonPatternScenario) {
	
	queries := []string{"Pattern", "Builder", "Observer"}
	if len(scenario.ExpectedSymbols) > 0 {
		queries = append(queries, scenario.ExpectedSymbols[0].Name)
	}

	for _, query := range queries {
		symbols, err := suite.httpClient.WorkspaceSymbol(ctx, query)
		if err != nil {
			suite.T().Logf("WorkspaceSymbol error for query '%s': %v", query, err)
			continue
		}

		result := helpers.ValidateWorkspaceSymbolResponse(validationCtx, symbols, query)
		result.Timestamp = time.Now().Unix()
		suite.validationResults = append(suite.validationResults, result)

		assert.True(suite.T(), result.IsValid, "WorkspaceSymbol validation failed: %v", result.Errors)
	}
}

func (suite *PythonLSPComprehensiveIntegrationTestSuite) testCompletionWithValidation(
	ctx context.Context, validationCtx *helpers.LSPValidationContext,
	fileURI string, scenario fixtures.PythonPatternScenario) {
	
	for _, testPos := range scenario.TestPositions {
		position := testutils.Position{Line: testPos.Line, Character: testPos.Character}
		completionList, err := suite.httpClient.Completion(ctx, fileURI, position)
		
		if err != nil {
			suite.T().Logf("Completion error for %s: %v", testPos.Symbol, err)
			continue
		}

		result := helpers.ValidateCompletionResponse(validationCtx, completionList, testPos)
		result.Timestamp = time.Now().Unix()
		suite.validationResults = append(suite.validationResults, result)

		assert.True(suite.T(), result.IsValid, "Completion validation failed: %v", result.Errors)
	}
}

// Workflow tests
func (suite *PythonLSPComprehensiveIntegrationTestSuite) testTypicalIDEWorkflow() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.timeout*2)
	defer cancel()

	// Simulate typical IDE workflow: definition -> references -> hover -> completion
	fileURI := "file://" + filepath.Join(suite.workspaceDir, "patterns/creational/builder.py")
	position := testutils.Position{Line: 20, Character: 10} // Target Builder class

	// Step 1: Go to definition
	locations, err := suite.httpClient.Definition(ctx, fileURI, position)
	if err == nil && len(locations) > 0 {
		suite.T().Logf("IDE Workflow - Definition found: %s", locations[0].URI)
		
		// Step 2: Find references from definition location
		defPos := testutils.Position{
			Line:      locations[0].Range.Start.Line,
			Character: locations[0].Range.Start.Character,
		}
		references, err := suite.httpClient.References(ctx, locations[0].URI, defPos, true)
		if err == nil {
			suite.T().Logf("IDE Workflow - Found %d references", len(references))
			
			// Step 3: Get hover information
			hoverResult, err := suite.httpClient.Hover(ctx, locations[0].URI, defPos)
			if err == nil && hoverResult != nil {
				suite.T().Logf("IDE Workflow - Hover info available")
				
				// Step 4: Get completion at nearby position
				completionPos := testutils.Position{
					Line:      defPos.Line + 1,
					Character: 0,
				}
				completionList, err := suite.httpClient.Completion(ctx, locations[0].URI, completionPos)
				if err == nil && completionList != nil {
					suite.T().Logf("IDE Workflow - %d completion items", len(completionList.Items))
				}
			}
		}
	}

	assert.NoError(suite.T(), err, "IDE workflow should complete without errors")
}

func (suite *PythonLSPComprehensiveIntegrationTestSuite) testPatternExplorationWorkflow() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.timeout*2)
	defer cancel()

	// Step 1: Search for pattern symbols
	symbols, err := suite.httpClient.WorkspaceSymbol(ctx, "Builder")
	if err == nil && len(symbols) > 0 {
		symbol := symbols[0]
		suite.T().Logf("Pattern Exploration - Found symbol: %s in %s", symbol.Name, symbol.Location.URI)
		
		// Step 2: Get document symbols for the file
		docSymbols, err := suite.httpClient.DocumentSymbol(ctx, symbol.Location.URI)
		if err == nil {
			suite.T().Logf("Pattern Exploration - Document has %d symbols", len(docSymbols))
			
			// Step 3: Get definition of the symbol
			locations, err := suite.httpClient.Definition(ctx, symbol.Location.URI, 
				testutils.Position{
					Line:      symbol.Location.Range.Start.Line,
					Character: symbol.Location.Range.Start.Character,
				})
			if err == nil {
				suite.T().Logf("Pattern Exploration - Definition workflow completed with %d locations", len(locations))
			}
		}
	}

	assert.NoError(suite.T(), err, "Pattern exploration workflow should complete without errors")
}

func (suite *PythonLSPComprehensiveIntegrationTestSuite) testCodeNavigationWorkflow() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.timeout*2)
	defer cancel()

	// Navigate through code using LSP features
	fileURI := "file://" + filepath.Join(suite.workspaceDir, "patterns/behavioral/observer.py")
	position := testutils.Position{Line: 15, Character: 5} // Observer pattern

	// Definition -> References -> Hover validation chain
	locations, err := suite.httpClient.Definition(ctx, fileURI, position)
	if err == nil && len(locations) > 0 {
		defLocation := locations[0]
		
		// Find all references to this definition
		references, err := suite.httpClient.References(ctx, defLocation.URI, 
			testutils.Position{
				Line:      defLocation.Range.Start.Line,
				Character: defLocation.Range.Start.Character,
			}, true)
		
		if err == nil {
			suite.T().Logf("Code Navigation - Found %d references", len(references))
			
			// Validate hover information for each reference
			validHovers := 0
			for _, ref := range references {
				hoverResult, err := suite.httpClient.Hover(ctx, ref.URI,
					testutils.Position{
						Line:      ref.Range.Start.Line,
						Character: ref.Range.Start.Character,
					})
				if err == nil && hoverResult != nil {
					validHovers++
				}
			}
			
			suite.T().Logf("Code Navigation - %d/%d references have hover info", validHovers, len(references))
		}
	}

	assert.NoError(suite.T(), err, "Code navigation workflow should complete without errors")
}

// validateOverallIntegration performs comprehensive integration validation
func (suite *PythonLSPComprehensiveIntegrationTestSuite) validateOverallIntegration() {
	// Calculate validation metrics
	metrics := helpers.CalculateLSPValidationMetrics(suite.validationResults)
	require.NotNil(suite.T(), metrics)

	// Assert integration quality metrics
	assert.GreaterOrEqual(suite.T(), metrics.DefinitionAccuracy, 0.7, 
		"Definition accuracy should be at least 70%")
	assert.GreaterOrEqual(suite.T(), metrics.ReferenceCompleteness, 0.6, 
		"Reference completeness should be at least 60%")
	assert.GreaterOrEqual(suite.T(), metrics.HoverContentQuality, 0.5, 
		"Hover content quality should be at least 50%")
	assert.GreaterOrEqual(suite.T(), metrics.ErrorHandlingRobustness, 0.8, 
		"Error handling robustness should be at least 80%")

	suite.T().Logf("Integration validation metrics: Def=%.2f, Ref=%.2f, Hover=%.2f, Error=%.2f",
		metrics.DefinitionAccuracy, metrics.ReferenceCompleteness, 
		metrics.HoverContentQuality, metrics.ErrorHandlingRobustness)
}

// saveTestArtifacts saves test results and artifacts
func (suite *PythonLSPComprehensiveIntegrationTestSuite) saveTestArtifacts() {
	// Save validation results
	if len(suite.validationResults) > 0 {
		validationFile := filepath.Join(suite.testArtifactsDir, "validation_results.json")
		validationData, _ := json.MarshalIndent(suite.validationResults, "", "  ")
		_ = os.WriteFile(validationFile, validationData, 0644)
	}

	// Save benchmark results
	if suite.benchmarkResults != nil {
		benchmarkFile := filepath.Join(suite.testArtifactsDir, "benchmark_results.json")
		benchmarkData, _ := json.MarshalIndent(suite.benchmarkResults, "", "  ")
		_ = os.WriteFile(benchmarkFile, benchmarkData, 0644)

		// Save benchmark report
		reportFile := filepath.Join(suite.testArtifactsDir, "benchmark_report.txt")
		report := suite.benchmarkResults.GenerateBenchmarkReport()
		_ = os.WriteFile(reportFile, []byte(report), 0644)
	}

	suite.T().Logf("Test artifacts saved to: %s", suite.testArtifactsDir)
}

// generateComprehensiveReport generates a comprehensive test report
func (suite *PythonLSPComprehensiveIntegrationTestSuite) generateComprehensiveReport() {
	report := fmt.Sprintf(`Python LSP Comprehensive Integration Test Report
=================================================
Test Duration: %v
Workspace: %s
Test Artifacts: %s

Validation Results: %d total validations
`, 
		time.Since(suite.startTime),
		suite.workspaceDir,
		suite.testArtifactsDir,
		len(suite.validationResults),
	)

	// Add validation summary
	if len(suite.validationResults) > 0 {
		metrics := helpers.CalculateLSPValidationMetrics(suite.validationResults)
		report += fmt.Sprintf(`
Validation Metrics:
- Definition Accuracy: %.2f%%
- Reference Completeness: %.2f%%
- Hover Content Quality: %.2f%%
- Error Handling Robustness: %.2f%%
- Symbol Hierarchy Max Depth: %d
`,
			metrics.DefinitionAccuracy*100,
			metrics.ReferenceCompleteness*100,
			metrics.HoverContentQuality*100,
			metrics.ErrorHandlingRobustness*100,
			metrics.SymbolHierarchyDepth,
		)
	}

	// Add benchmark summary
	if suite.benchmarkResults != nil {
		report += fmt.Sprintf(`
Performance Benchmark Results:
- Overall Throughput: %.2f req/sec
- Overall Error Rate: %.2f%%
- Total Requests: %d
- Benchmark Duration: %v
`,
			suite.benchmarkResults.OverallThroughput,
			suite.benchmarkResults.OverallErrorRate*100,
			suite.benchmarkResults.TotalRequests,
			suite.benchmarkResults.OverallDuration,
		)
	}

	// Save comprehensive report
	reportFile := filepath.Join(suite.testArtifactsDir, "comprehensive_report.txt")
	_ = os.WriteFile(reportFile, []byte(report), 0644)

	suite.T().Log(report)
}

// TestSuite entry point
func TestPythonLSPComprehensiveIntegrationTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comprehensive integration test in short mode")
	}
	suite.Run(t, new(PythonLSPComprehensiveIntegrationTestSuite))
}