package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"lsp-gateway/internal/gateway"
	"lsp-gateway/tests/framework"

	"github.com/stretchr/testify/suite"
)

// MultiLanguageWorkflowTestSuite provides comprehensive end-to-end integration tests
// for complete multi-language workflows, testing the integration of all major components:
// MultiServerManager, SmartRouter, ProjectDetector, and response aggregation systems.
type MultiLanguageWorkflowTestSuite struct {
	suite.Suite
	framework       *framework.MultiLanguageTestFramework
	testTimeout     time.Duration
	createdProjects []*framework.TestProject
}

// SetupSuite initializes the test suite with framework components
func (suite *MultiLanguageWorkflowTestSuite) SetupSuite() {
	suite.testTimeout = 5 * time.Minute
	suite.framework = framework.NewMultiLanguageTestFramework(suite.testTimeout)
	suite.createdProjects = make([]*framework.TestProject, 0)

	// Setup test environment with all components
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := suite.framework.SetupTestEnvironment(ctx)
	suite.Require().NoError(err, "Failed to setup test environment")
}

// TearDownSuite cleans up all test resources
func (suite *MultiLanguageWorkflowTestSuite) TearDownSuite() {
	if suite.framework != nil {
		err := suite.framework.CleanupAll()
		suite.Require().NoError(err, "Failed to cleanup test framework")
	}
}

// SetupTest prepares each individual test
func (suite *MultiLanguageWorkflowTestSuite) SetupTest() {
	// Reset framework state for each test
	suite.createdProjects = make([]*framework.TestProject, 0)
}

// TearDownTest cleans up after each test
func (suite *MultiLanguageWorkflowTestSuite) TearDownTest() {
	// Individual test cleanup is handled by framework
}

// TestCompleteMonorepoWorkflow tests end-to-end workflow for a large monorepo
// with multiple languages, testing complete integration from project detection
// through server management to response aggregation.
func (suite *MultiLanguageWorkflowTestSuite) TestCompleteMonorepoWorkflow() {
	// Create a realistic large monorepo project
	project, err := suite.framework.CreateMultiLanguageProject(
		framework.ProjectTypeMonorepo,
		[]string{"go", "python", "typescript", "java", "rust"},
	)
	suite.Require().NoError(err, "Failed to create monorepo project")
	suite.createdProjects = append(suite.createdProjects, project)

	// Validate project structure
	err = suite.framework.ValidateProjectStructure(project)
	suite.Require().NoError(err, "Project structure validation failed")

	// Start language servers for all languages in the project
	err = suite.framework.StartMultipleLanguageServers(project.Languages)
	suite.Require().NoError(err, "Failed to start language servers")

	// Create and configure the project-aware gateway
	projectGateway, err := suite.framework.CreateGatewayWithProject(project)
	suite.Require().NoError(err, "Failed to create project gateway")
	suite.NotNil(projectGateway, "Project gateway should not be nil")

	// Test complete workflow scenarios
	ctx := suite.framework.Context()

	// Scenario 1: Project Detection → Server Pool Setup → Request Routing
	suite.Run("ProjectDetection_To_ServerPoolSetup", func() {
		// Simulate project detection
		detector := gateway.NewProjectDetector()
		projectInfo, err := detector.DetectProject(ctx, project.RootPath)
		suite.Require().NoError(err, "Project detection failed")
		suite.NotNil(projectInfo, "Project info should not be nil")

		// Validate detected languages match project languages
		suite.GreaterOrEqual(len(projectInfo.Languages), 4, "Should detect multiple languages")

		for _, lang := range project.Languages {
			suite.True(projectInfo.HasLanguage(lang), fmt.Sprintf("Should detect %s language", lang))
		}

		// Simulate server pool initialization based on detected project
		multiServerManager := gateway.NewMultiServerManager(nil)
		err = multiServerManager.InitializeFromProject(ctx, projectInfo)
		suite.Require().NoError(err, "Server pool initialization failed")

		// Validate server instances are created for each language
		serverInstances := multiServerManager.GetActiveServers()
		suite.GreaterOrEqual(len(serverInstances), 3, "Should have server instances for multiple languages")
	})

	// Scenario 2: Smart Routing with Language Detection
	suite.Run("SmartRouting_With_LanguageDetection", func() {
		smartRouter := gateway.NewSmartRouter(nil)

		// Test routing for different file types
		testCases := []struct {
			filePath     string
			expectedLang string
		}{
			{filepath.Join(project.RootPath, "services/auth-service/main.go"), "go"},
			{filepath.Join(project.RootPath, "services/user-service/main.py"), "python"},
			{filepath.Join(project.RootPath, "frontend/src/App.tsx"), "typescript"},
			{filepath.Join(project.RootPath, "services/notification/Main.java"), "java"},
			{filepath.Join(project.RootPath, "services/search/src/main.rs"), "rust"},
		}

		for _, tc := range testCases {
			routingDecision, err := smartRouter.DetermineRouting(ctx, tc.filePath, "textDocument/definition")
			suite.Require().NoError(err, fmt.Sprintf("Routing failed for %s", tc.filePath))
			suite.Equal(tc.expectedLang, routingDecision.TargetLanguage,
				fmt.Sprintf("Wrong language detected for %s", tc.filePath))
		}
	})

	// Scenario 3: Complete LSP Request Lifecycle
	suite.Run("Complete_LSP_Request_Lifecycle", func() {
		// Test various LSP methods through the complete pipeline
		lspMethods := []string{
			"textDocument/definition",
			"textDocument/references",
			"textDocument/hover",
			"textDocument/documentSymbol",
			"workspace/symbol",
		}

		for _, method := range lspMethods {
			for _, lang := range project.Languages {
				testFile := suite.getTestFileForLanguage(project, lang)
				if testFile == "" {
					continue
				}

				params := suite.createLSPParams(method, testFile)

				// Execute request through complete pipeline
				response, err := projectGateway.HandleLSPRequest(ctx, method, params)
				suite.Require().NoError(err,
					fmt.Sprintf("LSP request failed: %s for %s", method, lang))
				suite.NotNil(response,
					fmt.Sprintf("Response should not be nil for %s %s", method, lang))

				// Validate response structure
				suite.validateLSPResponse(method, response)
			}
		}
	})

	// Scenario 4: Response Aggregation from Multiple Servers
	suite.Run("Response_Aggregation_MultipleServers", func() {
		responseAggregator := gateway.NewResponseAggregator()

		// Simulate workspace symbol search across all languages
		symbolQuery := "User"

		// Create requests for all language servers
		responses := make(map[string]interface{})
		for _, lang := range project.Languages {
			mockResponse := suite.createMockSymbolResponse(lang, symbolQuery)
			responses[lang] = mockResponse
		}

		// Aggregate responses
		aggregatedResponse, err := responseAggregator.AggregateWorkspaceSymbols(ctx, responses)
		suite.Require().NoError(err, "Response aggregation failed")
		suite.NotNil(aggregatedResponse, "Aggregated response should not be nil")

		// Validate aggregation results
		symbols, ok := aggregatedResponse.([]interface{})
		suite.True(ok, "Aggregated response should be symbol array")
		suite.Greater(len(symbols), len(project.Languages)-1,
			"Should aggregate symbols from multiple languages")
	})

	// Test performance under realistic load
	suite.Run("Performance_Under_Load", func() {
		metrics, err := suite.framework.MeasurePerformance(func() error {
			return suite.simulateRealisticWorkload(ctx, projectGateway, project)
		})
		suite.Require().NoError(err, "Performance test failed")

		// Validate performance expectations
		suite.Less(metrics.OperationDuration, 30*time.Second,
			"Complete workflow should complete within 30 seconds")
		suite.Equal(0, metrics.ErrorCount, "Should have no errors during performance test")
	})
}

// TestMicroservicesArchitectureWorkflow tests end-to-end workflow for microservices
// architecture with different languages per service
func (suite *MultiLanguageWorkflowTestSuite) TestMicroservicesArchitectureWorkflow() {
	// Create microservices project with different languages per service
	project, err := suite.framework.CreateMultiLanguageProject(
		framework.ProjectTypeMicroservices,
		[]string{"go", "python", "typescript", "java"},
	)
	suite.Require().NoError(err, "Failed to create microservices project")
	suite.createdProjects = append(suite.createdProjects, project)

	// Start servers for all languages
	err = suite.framework.StartMultipleLanguageServers(project.Languages)
	suite.Require().NoError(err, "Failed to start language servers")

	// Create multi-server manager
	multiServerManager := gateway.NewMultiServerManager(nil)

	ctx := suite.framework.Context()

	// Test service-specific routing
	suite.Run("Service_Specific_Routing", func() {
		// Define service-language mappings typical of microservices
		serviceMappings := map[string]string{
			"auth-service":         "go",
			"user-service":         "python",
			"api-gateway":          "typescript",
			"notification-service": "java",
		}

		for service, expectedLang := range serviceMappings {
			servicePath := filepath.Join(project.RootPath, "services", service)

			// Test that requests to service files are routed to correct language server
			routing, err := multiServerManager.DetermineServerForPath(ctx, servicePath)
			suite.Require().NoError(err, fmt.Sprintf("Failed to determine routing for %s", service))
			suite.Equal(expectedLang, routing.Language,
				fmt.Sprintf("Service %s should route to %s server", service, expectedLang))
		}
	})

	// Test cross-service reference resolution
	suite.Run("CrossService_Reference_Resolution", func() {
		// Simulate finding references across service boundaries
		referenceAggregator := gateway.NewResponseAggregator()

		// Mock references from multiple services
		crossServiceRefs := make(map[string]interface{})
		for _, lang := range project.Languages {
			crossServiceRefs[lang] = suite.createMockCrossServiceReferences(lang)
		}

		aggregated, err := referenceAggregator.AggregateCrossServiceReferences(ctx, crossServiceRefs)
		suite.Require().NoError(err, "Cross-service reference aggregation failed")

		// Validate cross-service references are properly linked
		refs, ok := aggregated.([]interface{})
		suite.True(ok, "Should return reference array")
		suite.Greater(len(refs), 1, "Should find cross-service references")
	})

	// Test load balancing across services
	suite.Run("LoadBalancing_Across_Services", func() {
		result, err := suite.framework.SimulateComplexWorkflow(framework.ScenarioLoadBalancing)
		suite.Require().NoError(err, "Load balancing simulation failed")
		suite.True(result.Success, "Load balancing should succeed")

		// Validate load distribution
		suite.NotNil(result.LoadBalancing, "Should have load balancing results")
		suite.Greater(result.LoadBalancing.EfficiencyScore, 0.7,
			"Load balancing should be efficient")
	})
}

// TestFrontendBackendIntegration tests complete workflow for frontend-backend projects
func (suite *MultiLanguageWorkflowTestSuite) TestFrontendBackendIntegration() {
	// Create frontend-backend project
	project, err := suite.framework.CreateMultiLanguageProject(
		framework.ProjectTypeFrontendBackend,
		[]string{"typescript", "go"},
	)
	suite.Require().NoError(err, "Failed to create frontend-backend project")
	suite.createdProjects = append(suite.createdProjects, project)

	// Start servers
	err = suite.framework.StartMultipleLanguageServers(project.Languages)
	suite.Require().NoError(err, "Failed to start language servers")

	ctx := suite.framework.Context()

	// Test frontend-backend API integration
	suite.Run("Frontend_Backend_API_Integration", func() {
		// Create project-aware gateway
		projectGateway, err := suite.framework.CreateGatewayWithProject(project)
		suite.Require().NoError(err, "Failed to create project gateway")

		// Test API endpoint discovery from frontend to backend
		frontendFile := filepath.Join(project.RootPath, "frontend/src/api/client.ts")
		backendFile := filepath.Join(project.RootPath, "backend/api/handlers.go")

		// Simulate finding API references between frontend and backend
		frontendRefs, err := projectGateway.HandleLSPRequest(ctx, "textDocument/references",
			suite.createLSPParams("textDocument/references", frontendFile))
		suite.Require().NoError(err, "Frontend reference lookup failed")

		backendRefs, err := projectGateway.HandleLSPRequest(ctx, "textDocument/references",
			suite.createLSPParams("textDocument/references", backendFile))
		suite.Require().NoError(err, "Backend reference lookup failed")

		// Validate both frontend and backend references are found
		suite.NotNil(frontendRefs, "Frontend references should be found")
		suite.NotNil(backendRefs, "Backend references should be found")
	})

	// Test shared type definitions
	suite.Run("Shared_Type_Definitions", func() {
		// Test that type definitions are consistent between frontend and backend
		smartRouter := gateway.NewSmartRouter(nil)

		// Test hover information for shared types
		sharedTypes := []string{"User", "ApiResponse", "ErrorResponse"}

		for _, typeName := range sharedTypes {
			// Check type definition in both frontend and backend contexts
			frontendContext := suite.createTypeContext("typescript", typeName)
			backendContext := suite.createTypeContext("go", typeName)

			frontendRoute, err := smartRouter.RouteTypeQuery(ctx, frontendContext)
			suite.Require().NoError(err, fmt.Sprintf("Frontend type routing failed for %s", typeName))

			backendRoute, err := smartRouter.RouteTypeQuery(ctx, backendContext)
			suite.Require().NoError(err, fmt.Sprintf("Backend type routing failed for %s", typeName))

			// Validate routing to appropriate language servers
			suite.Equal("typescript", frontendRoute.TargetLanguage)
			suite.Equal("go", backendRoute.TargetLanguage)
		}
	})
}

// TestCrossLanguageNavigation tests navigation across different languages
func (suite *MultiLanguageWorkflowTestSuite) TestCrossLanguageNavigation() {
	// Create polyglot project with cross-language dependencies
	project, err := suite.framework.CreateMultiLanguageProject(
		framework.ProjectTypePolyglot,
		[]string{"go", "python", "typescript", "java"},
	)
	suite.Require().NoError(err, "Failed to create polyglot project")
	suite.createdProjects = append(suite.createdProjects, project)

	// Start all language servers
	err = suite.framework.StartMultipleLanguageServers(project.Languages)
	suite.Require().NoError(err, "Failed to start language servers")

	ctx := suite.framework.Context()

	// Execute cross-language navigation scenario
	result, err := suite.framework.SimulateComplexWorkflow(framework.ScenarioCrossLanguageNavigation)
	suite.Require().NoError(err, "Cross-language navigation simulation failed")
	suite.True(result.Success, "Cross-language navigation should succeed")

	// Test specific cross-language scenarios
	suite.Run("Go_To_Python_API_Calls", func() {
		// Simulate Go code calling Python API
		smartRouter := gateway.NewSmartRouter(nil)

		// Create cross-language reference context
		crossLangContext := suite.createCrossLanguageContext("go", "python", "api_call")

		routing, err := smartRouter.RouteCrossLanguageQuery(ctx, crossLangContext)
		suite.Require().NoError(err, "Cross-language routing failed")

		// Validate routing handles cross-language dependencies
		suite.NotNil(routing.MultiLanguageTargets, "Should identify multiple language targets")
		suite.Contains(routing.MultiLanguageTargets, "go", "Should include Go target")
		suite.Contains(routing.MultiLanguageTargets, "python", "Should include Python target")
	})

	suite.Run("TypeScript_To_Java_Type_References", func() {
		// Test TypeScript frontend referencing Java backend types
		responseAggregator := gateway.NewResponseAggregator()

		// Mock responses from both TypeScript and Java servers
		responses := map[string]interface{}{
			"typescript": suite.createMockTypeScriptResponse("UserModel"),
			"java":       suite.createMockJavaResponse("UserModel"),
		}

		aggregated, err := responseAggregator.AggregateCrossLanguageTypes(ctx, responses)
		suite.Require().NoError(err, "Cross-language type aggregation failed")

		// Validate aggregation preserves type information from both languages
		suite.NotNil(aggregated, "Aggregated response should not be nil")
	})
}

// TestLargeProjectPerformance tests performance with enterprise-scale projects
func (suite *MultiLanguageWorkflowTestSuite) TestLargeProjectPerformance() {
	// Create large complex project
	project, err := suite.framework.CreateMultiLanguageProject(
		framework.ProjectTypeMonorepo,
		[]string{"go", "python", "typescript", "java", "rust", "cpp"},
	)
	suite.Require().NoError(err, "Failed to create large project")
	suite.createdProjects = append(suite.createdProjects, project)

	// Start all language servers
	err = suite.framework.StartMultipleLanguageServers(project.Languages)
	suite.Require().NoError(err, "Failed to start language servers")

	// Execute large project performance scenario
	result, err := suite.framework.SimulateComplexWorkflow(framework.ScenarioLargeProjectPerformance)
	suite.Require().NoError(err, "Large project performance simulation failed")
	suite.True(result.Success, "Large project performance test should succeed")

	// Validate performance metrics
	suite.NotNil(result.Metrics, "Should have performance metrics")
	suite.Less(result.Metrics.AverageResponseTime, 5*time.Second,
		"Average response time should be reasonable for large projects")
	suite.Greater(result.Metrics.ThroughputPerSecond, 10.0,
		"Should maintain good throughput")
	suite.Less(result.Metrics.MemoryUsageMB, 1000.0,
		"Memory usage should be controlled")
}

// TestCircuitBreakerIntegration tests circuit breaker functionality across components
func (suite *MultiLanguageWorkflowTestSuite) TestCircuitBreakerIntegration() {
	// Create project for circuit breaker testing
	project, err := suite.framework.CreateMultiLanguageProject(
		framework.ProjectTypeMultiLanguage,
		[]string{"go", "python", "typescript"},
	)
	suite.Require().NoError(err, "Failed to create project for circuit breaker test")
	suite.createdProjects = append(suite.createdProjects, project)

	// Start language servers
	err = suite.framework.StartMultipleLanguageServers(project.Languages)
	suite.Require().NoError(err, "Failed to start language servers")

	// Execute circuit breaker testing scenario
	result, err := suite.framework.SimulateComplexWorkflow(framework.ScenarioCircuitBreakerTesting)
	suite.Require().NoError(err, "Circuit breaker simulation failed")
	suite.True(result.Success, "Circuit breaker test should succeed")

	// Validate circuit breaker results
	suite.NotNil(result.CircuitBreaker, "Should have circuit breaker results")
	suite.GreaterOrEqual(result.CircuitBreaker.TriggeredCount, 0,
		"Circuit breaker should have trigger count")
	suite.Less(result.CircuitBreaker.RecoveryTime, 30*time.Second,
		"Recovery time should be reasonable")
}

// TestHealthMonitoringIntegration tests health monitoring across all components
func (suite *MultiLanguageWorkflowTestSuite) TestHealthMonitoringIntegration() {
	// Create project for health monitoring
	project, err := suite.framework.CreateMultiLanguageProject(
		framework.ProjectTypeMultiLanguage,
		[]string{"go", "python", "java"},
	)
	suite.Require().NoError(err, "Failed to create project for health monitoring")
	suite.createdProjects = append(suite.createdProjects, project)

	// Start language servers
	err = suite.framework.StartMultipleLanguageServers(project.Languages)
	suite.Require().NoError(err, "Failed to start language servers")

	// Execute health monitoring scenario
	result, err := suite.framework.SimulateComplexWorkflow(framework.ScenarioHealthMonitoring)
	suite.Require().NoError(err, "Health monitoring simulation failed")
	suite.True(result.Success, "Health monitoring test should succeed")

	// Validate health monitoring results
	suite.NotNil(result.Health, "Should have health monitoring results")
	suite.Greater(result.Health.HealthyServers, 0, "Should have healthy servers")
	suite.GreaterOrEqual(result.Health.AverageHealthScore, 0.5,
		"Average health score should be reasonable")
}

// Helper methods for test execution

// getTestFileForLanguage returns a test file path for the given language
func (suite *MultiLanguageWorkflowTestSuite) getTestFileForLanguage(project *framework.TestProject, language string) string {
	languageFiles := map[string]string{
		"go":         "main.go",
		"python":     "main.py",
		"typescript": "src/index.ts",
		"java":       "src/main/java/Main.java",
		"rust":       "src/main.rs",
		"cpp":        "main.cpp",
	}

	if fileName, exists := languageFiles[language]; exists {
		return filepath.Join(project.RootPath, fileName)
	}
	return ""
}

// createLSPParams creates LSP request parameters for testing
func (suite *MultiLanguageWorkflowTestSuite) createLSPParams(method, filePath string) interface{} {
	switch method {
	case "textDocument/definition", "textDocument/references", "textDocument/hover":
		return map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fmt.Sprintf("file://%s", filePath),
			},
			"position": map[string]interface{}{
				"line":      10,
				"character": 5,
			},
		}
	case "textDocument/documentSymbol":
		return map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fmt.Sprintf("file://%s", filePath),
			},
		}
	case "workspace/symbol":
		return map[string]interface{}{
			"query": "test",
		}
	default:
		return map[string]interface{}{}
	}
}

// validateLSPResponse validates the structure of LSP responses
func (suite *MultiLanguageWorkflowTestSuite) validateLSPResponse(method string, response interface{}) {
	suite.NotNil(response, "Response should not be nil")

	// Convert to JSON for structure validation
	jsonBytes, err := json.Marshal(response)
	suite.Require().NoError(err, "Response should be JSON serializable")

	var responseObj interface{}
	err = json.Unmarshal(jsonBytes, &responseObj)
	suite.Require().NoError(err, "Response should be valid JSON")

	// Method-specific validation
	switch method {
	case "textDocument/definition":
		// Should have location information
		suite.validateLocationResponse(responseObj)
	case "textDocument/references":
		// Should have array of locations
		suite.validateReferencesResponse(responseObj)
	case "textDocument/hover":
		// Should have hover contents
		suite.validateHoverResponse(responseObj)
	case "textDocument/documentSymbol":
		// Should have array of symbols
		suite.validateSymbolsResponse(responseObj)
	case "workspace/symbol":
		// Should have array of symbol information
		suite.validateWorkspaceSymbolsResponse(responseObj)
	}
}

// validateLocationResponse validates location response structure
func (suite *MultiLanguageWorkflowTestSuite) validateLocationResponse(response interface{}) {
	responseMap, ok := response.(map[string]interface{})
	if ok {
		suite.Contains(responseMap, "uri", "Location should have URI")
		suite.Contains(responseMap, "range", "Location should have range")
	}
}

// validateReferencesResponse validates references response structure
func (suite *MultiLanguageWorkflowTestSuite) validateReferencesResponse(response interface{}) {
	if responseArray, ok := response.([]interface{}); ok {
		suite.Greater(len(responseArray), 0, "References should not be empty")
		for _, ref := range responseArray {
			suite.validateLocationResponse(ref)
		}
	}
}

// validateHoverResponse validates hover response structure
func (suite *MultiLanguageWorkflowTestSuite) validateHoverResponse(response interface{}) {
	responseMap, ok := response.(map[string]interface{})
	if ok {
		suite.Contains(responseMap, "contents", "Hover should have contents")
	}
}

// validateSymbolsResponse validates document symbols response
func (suite *MultiLanguageWorkflowTestSuite) validateSymbolsResponse(response interface{}) {
	if responseArray, ok := response.([]interface{}); ok {
		for _, symbol := range responseArray {
			if symbolMap, ok := symbol.(map[string]interface{}); ok {
				suite.Contains(symbolMap, "name", "Symbol should have name")
				suite.Contains(symbolMap, "kind", "Symbol should have kind")
			}
		}
	}
}

// validateWorkspaceSymbolsResponse validates workspace symbols response
func (suite *MultiLanguageWorkflowTestSuite) validateWorkspaceSymbolsResponse(response interface{}) {
	if responseArray, ok := response.([]interface{}); ok {
		for _, symbol := range responseArray {
			if symbolMap, ok := symbol.(map[string]interface{}); ok {
				suite.Contains(symbolMap, "name", "Workspace symbol should have name")
				suite.Contains(symbolMap, "location", "Workspace symbol should have location")
			}
		}
	}
}

// createMockSymbolResponse creates mock symbol response for testing
func (suite *MultiLanguageWorkflowTestSuite) createMockSymbolResponse(language, query string) interface{} {
	return []interface{}{
		map[string]interface{}{
			"name": fmt.Sprintf("%s_%s_Symbol", query, language),
			"kind": 12, // Function
			"location": map[string]interface{}{
				"uri": fmt.Sprintf("file:///mock/%s_file", language),
				"range": map[string]interface{}{
					"start": map[string]interface{}{"line": 10, "character": 0},
					"end":   map[string]interface{}{"line": 10, "character": 10},
				},
			},
		},
	}
}

// createMockCrossServiceReferences creates mock cross-service references
func (suite *MultiLanguageWorkflowTestSuite) createMockCrossServiceReferences(language string) interface{} {
	return []interface{}{
		map[string]interface{}{
			"uri": fmt.Sprintf("file:///services/%s-service/main", language),
			"range": map[string]interface{}{
				"start": map[string]interface{}{"line": 5, "character": 0},
				"end":   map[string]interface{}{"line": 5, "character": 10},
			},
		},
	}
}

// createTypeContext creates type context for cross-language testing
func (suite *MultiLanguageWorkflowTestSuite) createTypeContext(language, typeName string) interface{} {
	return map[string]interface{}{
		"language": language,
		"typeName": typeName,
		"context":  "type_query",
	}
}

// createCrossLanguageContext creates cross-language context for testing
func (suite *MultiLanguageWorkflowTestSuite) createCrossLanguageContext(fromLang, toLang, operation string) interface{} {
	return map[string]interface{}{
		"fromLanguage": fromLang,
		"toLanguage":   toLang,
		"operation":    operation,
		"context":      "cross_language_query",
	}
}

// createMockTypeScriptResponse creates mock TypeScript response
func (suite *MultiLanguageWorkflowTestSuite) createMockTypeScriptResponse(typeName string) interface{} {
	return map[string]interface{}{
		"name": typeName,
		"type": "interface",
		"properties": []interface{}{
			map[string]interface{}{
				"name": "id",
				"type": "number",
			},
			map[string]interface{}{
				"name": "name",
				"type": "string",
			},
		},
	}
}

// createMockJavaResponse creates mock Java response
func (suite *MultiLanguageWorkflowTestSuite) createMockJavaResponse(typeName string) interface{} {
	return map[string]interface{}{
		"name": typeName,
		"type": "class",
		"fields": []interface{}{
			map[string]interface{}{
				"name": "id",
				"type": "Long",
			},
			map[string]interface{}{
				"name": "name",
				"type": "String",
			},
		},
	}
}

// simulateRealisticWorkload simulates realistic workload for performance testing
func (suite *MultiLanguageWorkflowTestSuite) simulateRealisticWorkload(ctx context.Context, gateway *gateway.ProjectAwareGateway, project *framework.TestProject) error {
	var wg sync.WaitGroup
	numRequests := 20
	errChan := make(chan error, numRequests)

	// Simulate concurrent requests across different languages and methods
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// Rotate through languages and methods
			lang := project.Languages[index%len(project.Languages)]
			methods := []string{"textDocument/definition", "textDocument/references", "textDocument/hover"}
			method := methods[index%len(methods)]

			testFile := suite.getTestFileForLanguage(project, lang)
			if testFile == "" {
				errChan <- fmt.Errorf("no test file for language %s", lang)
				return
			}

			params := suite.createLSPParams(method, testFile)

			_, err := gateway.HandleLSPRequest(ctx, method, params)
			if err != nil {
				errChan <- fmt.Errorf("request failed for %s %s: %w", lang, method, err)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// Run the test suite
func TestMultiLanguageWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(MultiLanguageWorkflowTestSuite))
}
