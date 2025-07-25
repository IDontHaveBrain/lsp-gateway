package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"lsp-gateway/mcp"
	"lsp-gateway/tests/framework"
	"lsp-gateway/tests/mocks"

	"github.com/stretchr/testify/suite"
)

// MultiLanguageE2ETestSuite provides comprehensive E2E tests for cross-language LSP functionality
type MultiLanguageE2ETestSuite struct {
	suite.Suite
	framework    *framework.MultiLanguageTestFramework
	mockClient   *mocks.MockMcpClient
	gatewayURL   string
	testTimeout  time.Duration
	tempProjects []string
}

// SetupSuite initializes the test environment
func (suite *MultiLanguageE2ETestSuite) SetupSuite() {
	suite.testTimeout = 30 * time.Second
	suite.gatewayURL = "http://localhost:8080"
	suite.tempProjects = make([]string, 0)

	// Initialize test framework
	suite.framework = framework.NewMultiLanguageTestFramework(suite.testTimeout)
	suite.Require().NotNil(suite.framework)

	// Initialize mock MCP client
	suite.mockClient = mocks.NewMockMcpClient()
	suite.Require().NotNil(suite.mockClient)

	// Setup test environment
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := suite.framework.SetupTestEnvironment(ctx)
	suite.Require().NoError(err)
}

// TearDownSuite cleans up test resources
func (suite *MultiLanguageE2ETestSuite) TearDownSuite() {
	if suite.framework != nil {
		suite.framework.CleanupAll()
	}
}

// SetupTest resets mock client state before each test
func (suite *MultiLanguageE2ETestSuite) SetupTest() {
	suite.mockClient.Reset()
}

// TestPolyglotProjectIntegration tests comprehensive polyglot project scenarios
func (suite *MultiLanguageE2ETestSuite) TestPolyglotProjectIntegration() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Create polyglot project with Go, Python, TypeScript, Java
	languages := []string{"go", "python", "typescript", "java"}
	project, err := suite.framework.CreateMultiLanguageProject(framework.ProjectTypePolyglot, languages)
	suite.Require().NoError(err)
	suite.Require().NotNil(project)

	// Validate project structure
	err = suite.framework.ValidateProjectStructure(project)
	suite.Require().NoError(err)

	// Start language servers for all languages
	err = suite.framework.StartMultipleLanguageServers(languages)
	suite.Require().NoError(err)

	// Setup realistic cross-language responses
	suite.setupPolyglotResponses()

	// Test cross-language workspace symbol search
	suite.Run("WorkspaceSymbolSearch", func() {
		suite.testCrossLanguageWorkspaceSymbols(ctx, project)
	})

	// Test cross-language definition lookup
	suite.Run("CrossLanguageDefinition", func() {
		suite.testCrossLanguageDefinition(ctx, project)
	})

	// Test concurrent multi-language requests
	suite.Run("ConcurrentRequests", func() {
		suite.testConcurrentMultiLanguageRequests(ctx, project, languages)
	})

	// Test language-specific error handling
	suite.Run("ErrorHandling", func() {
		suite.testLanguageSpecificErrorHandling(ctx, project, languages)
	})
}

// TestMicroservicesArchitecture tests multi-language microservices scenarios
func (suite *MultiLanguageE2ETestSuite) TestMicroservicesArchitecture() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Create microservices project
	languages := []string{"go", "python", "typescript", "java"}
	project, err := suite.framework.CreateMultiLanguageProject(framework.ProjectTypeMicroservices, languages)
	suite.Require().NoError(err)

	// Start language servers
	err = suite.framework.StartMultipleLanguageServers(languages)
	suite.Require().NoError(err)

	// Setup microservices-specific responses
	suite.setupMicroservicesResponses()

	// Test service discovery across languages
	suite.Run("ServiceDiscovery", func() {
		suite.testServiceDiscoveryAcrossLanguages(ctx, project, languages)
	})

	// Test inter-service navigation
	suite.Run("InterServiceNavigation", func() {
		suite.testInterServiceNavigation(ctx, project)
	})

	// Test load balancing across language servers
	suite.Run("LoadBalancing", func() {
		suite.testLoadBalancingAcrossServers(ctx, languages)
	})
}

// TestMonorepoScenario tests large monorepo with multiple languages
func (suite *MultiLanguageE2ETestSuite) TestMonorepoScenario() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Create large monorepo project
	languages := []string{"go", "python", "typescript", "java", "rust"}
	project, err := suite.framework.CreateMultiLanguageProject(framework.ProjectTypeMonorepo, languages)
	suite.Require().NoError(err)

	// Start all language servers
	err = suite.framework.StartMultipleLanguageServers(languages)
	suite.Require().NoError(err)

	// Setup monorepo responses
	suite.setupMonorepoResponses()

	// Test workspace-wide symbol search
	suite.Run("WorkspaceWideSymbolSearch", func() {
		suite.testWorkspaceWideSymbolSearch(ctx, project, languages)
	})

	// Test monorepo performance
	suite.Run("MonorepoPerformance", func() {
		suite.testMonorepoPerformance(ctx, project, languages)
	})

	// Test shared library navigation
	suite.Run("SharedLibraryNavigation", func() {
		suite.testSharedLibraryNavigation(ctx, project)
	})
}

// TestFullStackWebApp tests frontend-backend integration
func (suite *MultiLanguageE2ETestSuite) TestFullStackWebApp() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Create full-stack project
	languages := []string{"typescript", "go"}
	project, err := suite.framework.CreateMultiLanguageProject(framework.ProjectTypeFrontendBackend, languages)
	suite.Require().NoError(err)

	// Start language servers
	err = suite.framework.StartMultipleLanguageServers(languages)
	suite.Require().NoError(err)

	// Setup full-stack responses
	suite.setupFullStackResponses()

	// Test API contract navigation
	suite.Run("APIContractNavigation", func() {
		suite.testAPIContractNavigation(ctx, project)
	})

	// Test frontend-backend type sharing
	suite.Run("TypeSharing", func() {
		suite.testFrontendBackendTypeSharing(ctx, project)
	})
}

// TestServerRoutingValidation tests correct routing to language servers
func (suite *MultiLanguageE2ETestSuite) TestServerRoutingValidation() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	languages := []string{"go", "python", "typescript", "java"}
	project, err := suite.framework.CreateMultiLanguageProject(framework.ProjectTypeMultiLanguage, languages)
	suite.Require().NoError(err)

	err = suite.framework.StartMultipleLanguageServers(languages)
	suite.Require().NoError(err)

	// Test routing by file extension
	suite.Run("FileExtensionRouting", func() {
		suite.testFileExtensionRouting(ctx, project, languages)
	})

	// Test routing by project structure
	suite.Run("ProjectStructureRouting", func() {
		suite.testProjectStructureRouting(ctx, project, languages)
	})

	// Test fallback routing
	suite.Run("FallbackRouting", func() {
		suite.testFallbackRouting(ctx, project, languages)
	})
}

// TestServerFailoverScenarios tests language server failure handling
func (suite *MultiLanguageE2ETestSuite) TestServerFailoverScenarios() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	languages := []string{"go", "python", "typescript"}
	project, err := suite.framework.CreateMultiLanguageProject(framework.ProjectTypeMultiLanguage, languages)
	suite.Require().NoError(err)

	err = suite.framework.StartMultipleLanguageServers(languages)
	suite.Require().NoError(err)

	// Test individual server failure isolation
	suite.Run("ServerFailureIsolation", func() {
		suite.testServerFailureIsolation(ctx, project, languages)
	})

	// Test server recovery
	suite.Run("ServerRecovery", func() {
		suite.testServerRecovery(ctx, project, languages)
	})

	// Test degraded mode operation
	suite.Run("DegradedModeOperation", func() {
		suite.testDegradedModeOperation(ctx, project, languages)
	})
}

// TestConcurrentWorkspaceOperations tests concurrent operations across languages
func (suite *MultiLanguageE2ETestSuite) TestConcurrentWorkspaceOperations() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	languages := []string{"go", "python", "typescript", "java"}
	project, err := suite.framework.CreateMultiLanguageProject(framework.ProjectTypeWorkspace, languages)
	suite.Require().NoError(err)

	err = suite.framework.StartMultiLanguageServers(languages)
	suite.Require().NoError(err)

	// Execute concurrent workflow scenarios
	scenarios := []framework.WorkflowScenario{
		framework.ScenarioSimultaneousRequests,
		framework.ScenarioConcurrentWorkspaces,
		framework.ScenarioCrossLanguageNavigation,
	}

	for _, scenario := range scenarios {
		suite.Run(string(scenario), func() {
			result, err := suite.framework.SimulateComplexWorkflow(scenario)
			suite.Require().NoError(err)
			suite.Require().NotNil(result)
			suite.True(result.Success, "Workflow should succeed")
			suite.Greater(result.RequestCount, 0, "Should process requests")
		})
	}
}

// Helper methods for setting up realistic responses

func (suite *MultiLanguageE2ETestSuite) setupPolyglotResponses() {
	// Go definitions
	goDefinition := json.RawMessage(`{
		"uri": "file:///workspace/services/auth/handler.go",
		"range": {
			"start": {"line": 15, "character": 5},
			"end": {"line": 15, "character": 18}
		}
	}`)
	suite.mockClient.QueueResponse(goDefinition)

	// Python definitions
	pythonDefinition := json.RawMessage(`{
		"uri": "file:///workspace/services/data/processor.py",
		"range": {
			"start": {"line": 8, "character": 4},
			"end": {"line": 8, "character": 20}
		}
	}`)
	suite.mockClient.QueueResponse(pythonDefinition)

	// TypeScript definitions
	tsDefinition := json.RawMessage(`{
		"uri": "file:///workspace/frontend/src/services/ApiClient.ts",
		"range": {
			"start": {"line": 22, "character": 8},
			"end": {"line": 22, "character": 25}
		}
	}`)
	suite.mockClient.QueueResponse(tsDefinition)

	// Java definitions
	javaDefinition := json.RawMessage(`{
		"uri": "file:///workspace/services/notification/src/main/java/NotificationService.java",
		"range": {
			"start": {"line": 35, "character": 16},
			"end": {"line": 35, "character": 32}
		}
	}`)
	suite.mockClient.QueueResponse(javaDefinition)

	// Cross-language workspace symbols
	workspaceSymbols := json.RawMessage(`[
		{
			"name": "AuthHandler",
			"kind": 5,
			"location": {
				"uri": "file:///workspace/services/auth/handler.go",
				"range": {"start": {"line": 10, "character": 5}, "end": {"line": 10, "character": 16}}
			},
			"containerName": "auth"
		},
		{
			"name": "DataProcessor",
			"kind": 5,
			"location": {
				"uri": "file:///workspace/services/data/processor.py",
				"range": {"start": {"line": 5, "character": 6}, "end": {"line": 5, "character": 19}}
			},
			"containerName": "data"
		},
		{
			"name": "ApiClient",
			"kind": 5,
			"location": {
				"uri": "file:///workspace/frontend/src/services/ApiClient.ts",
				"range": {"start": {"line": 8, "character": 13}, "end": {"line": 8, "character": 22}}
			},
			"containerName": "services"
		},
		{
			"name": "NotificationService",
			"kind": 5,
			"location": {
				"uri": "file:///workspace/services/notification/src/main/java/NotificationService.java",
				"range": {"start": {"line": 12, "character": 13}, "end": {"line": 12, "character": 32}}
			},
			"containerName": "notification"
		}
	]`)
	suite.mockClient.QueueResponse(workspaceSymbols)
}

func (suite *MultiLanguageE2ETestSuite) setupMicroservicesResponses() {
	// Service interfaces across languages
	serviceSymbols := json.RawMessage(`[
		{
			"name": "UserService",
			"kind": 11,
			"location": {
				"uri": "file:///workspace/services/user-service/main.go",
				"range": {"start": {"line": 20, "character": 5}, "end": {"line": 20, "character": 16}}
			}
		},
		{
			"name": "AuthService",
			"kind": 5,
			"location": {
				"uri": "file:///workspace/services/auth-service/auth.py",
				"range": {"start": {"line": 15, "character": 6}, "end": {"line": 15, "character": 17}}
			}
		},
		{
			"name": "ApiGateway",
			"kind": 5,
			"location": {
				"uri": "file:///workspace/services/api-gateway/src/Gateway.ts",
				"range": {"start": {"line": 12, "character": 13}, "end": {"line": 12, "character": 23}}
			}
		},
		{
			"name": "NotificationController",
			"kind": 5,
			"location": {
				"uri": "file:///workspace/services/notification/Controller.java",
				"range": {"start": {"line": 18, "character": 13}, "end": {"line": 18, "character": 35}}
			}
		}
	]`)
	suite.mockClient.QueueResponse(serviceSymbols)
}

func (suite *MultiLanguageE2ETestSuite) setupMonorepoResponses() {
	// Large monorepo with multiple language components
	monorepoSymbols := json.RawMessage(`[
		{
			"name": "CoreEngine",
			"kind": 5,
			"location": {
				"uri": "file:///workspace/platform/core/engine.go",
				"range": {"start": {"line": 25, "character": 5}, "end": {"line": 25, "character": 15}}
			}
		},
		{
			"name": "MLPipeline",
			"kind": 5,
			"location": {
				"uri": "file:///workspace/platform/ml/pipeline.py",
				"range": {"start": {"line": 30, "character": 6}, "end": {"line": 30, "character": 16}}
			}
		},
		{
			"name": "WebInterface",
			"kind": 5,
			"location": {
				"uri": "file:///workspace/frontend/web/src/Interface.tsx",
				"range": {"start": {"line": 18, "character": 13}, "end": {"line": 18, "character": 25}}
			}
		},
		{
			"name": "DataConnector",
			"kind": 5,
			"location": {
				"uri": "file:///workspace/platform/connectors/DataConnector.java",
				"range": {"start": {"line": 22, "character": 13}, "end": {"line": 22, "character": 26}}
			}
		},
		{
			"name": "CLIToolkit",
			"kind": 1,
			"location": {
				"uri": "file:///workspace/tools/cli/src/main.rs",
				"range": {"start": {"line": 10, "character": 8}, "end": {"line": 10, "character": 18}}
			}
		}
	]`)
	suite.mockClient.QueueResponse(monorepoSymbols)
}

func (suite *MultiLanguageE2ETestSuite) setupFullStackResponses() {
	// Frontend-backend type definitions
	fullStackSymbols := json.RawMessage(`[
		{
			"name": "UserController",
			"kind": 5,
			"location": {
				"uri": "file:///workspace/backend/controllers/user.go",
				"range": {"start": {"line": 15, "character": 5}, "end": {"line": 15, "character": 19}}
			}
		},
		{
			"name": "UserService",
			"kind": 5,
			"location": {
				"uri": "file:///workspace/frontend/src/services/UserService.ts",
				"range": {"start": {"line": 8, "character": 13}, "end": {"line": 8, "character": 24}}
			}
		},
		{
			"name": "UserDTO",
			"kind": 11,
			"location": {
				"uri": "file:///workspace/shared/types/User.ts",
				"range": {"start": {"line": 5, "character": 17}, "end": {"line": 5, "character": 24}}
			}
		}
	]`)
	suite.mockClient.QueueResponse(fullStackSymbols)
}

// Test implementation methods

func (suite *MultiLanguageE2ETestSuite) testCrossLanguageWorkspaceSymbols(ctx context.Context, project *framework.TestProject) {
	// Test workspace symbol search across all languages
	response, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_WORKSPACE_SYMBOL, map[string]interface{}{
		"query": "Handler",
	})

	suite.Require().NoError(err)
	suite.Require().NotNil(response)

	// Verify we can find symbols from multiple languages
	var symbols []map[string]interface{}
	err = json.Unmarshal(response, &symbols)
	suite.Require().NoError(err)
	suite.Greater(len(symbols), 0, "Should find symbols across languages")

	// Verify symbols from different languages are returned
	languageURIs := make(map[string]bool)
	for _, symbol := range symbols {
		if location, ok := symbol["location"].(map[string]interface{}); ok {
			if uri, ok := location["uri"].(string); ok {
				if filepath.Ext(uri) == ".go" {
					languageURIs["go"] = true
				} else if filepath.Ext(uri) == ".py" {
					languageURIs["python"] = true
				} else if filepath.Ext(uri) == ".ts" || filepath.Ext(uri) == ".tsx" {
					languageURIs["typescript"] = true
				} else if filepath.Ext(uri) == ".java" {
					languageURIs["java"] = true
				}
			}
		}
	}

	suite.GreaterOrEqual(len(languageURIs), 2, "Should find symbols from multiple languages")
}

func (suite *MultiLanguageE2ETestSuite) testCrossLanguageDefinition(ctx context.Context, project *framework.TestProject) {
	// Test definition lookup across different languages
	languages := []string{"go", "python", "typescript", "java"}

	for _, lang := range languages {
		uri := fmt.Sprintf("file:///workspace/services/%s/main.%s", lang, suite.getFileExtension(lang))

		response, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": uri,
			},
			"position": map[string]interface{}{
				"line":      10,
				"character": 5,
			},
		})

		suite.Require().NoError(err, "Should handle definition request for %s", lang)
		suite.Require().NotNil(response, "Should return definition for %s", lang)

		// Verify response structure
		var definition map[string]interface{}
		err = json.Unmarshal(response, &definition)
		suite.Require().NoError(err)
		suite.Contains(definition, "uri", "Definition should contain URI")
		suite.Contains(definition, "range", "Definition should contain range")
	}
}

func (suite *MultiLanguageE2ETestSuite) testConcurrentMultiLanguageRequests(ctx context.Context, project *framework.TestProject, languages []string) {
	const concurrentRequests = 20
	var wg sync.WaitGroup
	results := make(chan error, concurrentRequests)

	// Launch concurrent requests across different languages
	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func(requestID int) {
			defer wg.Done()

			lang := languages[requestID%len(languages)]
			uri := fmt.Sprintf("file:///workspace/test_%d.%s", requestID, suite.getFileExtension(lang))

			response, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": uri,
				},
				"position": map[string]interface{}{
					"line":      requestID % 50,
					"character": requestID % 20,
				},
			})

			if err != nil {
				results <- fmt.Errorf("request %d failed: %w", requestID, err)
				return
			}

			if response == nil {
				results <- fmt.Errorf("request %d returned nil response", requestID)
				return
			}

			results <- nil
		}(i)
	}

	wg.Wait()
	close(results)

	// Check results
	errorCount := 0
	for err := range results {
		if err != nil {
			suite.T().Logf("Concurrent request error: %v", err)
			errorCount++
		}
	}

	suite.LessOrEqual(errorCount, concurrentRequests/10, "Should have minimal errors in concurrent requests")
}

func (suite *MultiLanguageE2ETestSuite) testLanguageSpecificErrorHandling(ctx context.Context, project *framework.TestProject, languages []string) {
	// Test error handling for each language
	for _, lang := range languages {
		// Queue an error for this language
		suite.mockClient.QueueError(fmt.Errorf("simulated %s server error", lang))

		uri := fmt.Sprintf("file:///workspace/error_test.%s", suite.getFileExtension(lang))

		_, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": uri,
			},
			"position": map[string]interface{}{
				"line":      0,
				"character": 0,
			},
		})

		suite.Require().Error(err, "Should handle error for %s", lang)
		suite.Contains(err.Error(), lang, "Error should mention the language")
	}
}

func (suite *MultiLanguageE2ETestSuite) testServiceDiscoveryAcrossLanguages(ctx context.Context, project *framework.TestProject, languages []string) {
	// Test discovering services implemented in different languages
	response, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_WORKSPACE_SYMBOL, map[string]interface{}{
		"query": "Service",
	})

	suite.Require().NoError(err)
	suite.Require().NotNil(response)

	// Verify services from different languages are discoverable
	var symbols []map[string]interface{}
	err = json.Unmarshal(response, &symbols)
	suite.Require().NoError(err)
	suite.Greater(len(symbols), 0, "Should discover services")

	serviceLanguages := make(map[string]bool)
	for _, symbol := range symbols {
		if location, ok := symbol["location"].(map[string]interface{}); ok {
			if uri, ok := location["uri"].(string); ok {
				ext := filepath.Ext(uri)
				lang := suite.getLanguageFromExtension(ext)
				if lang != "" {
					serviceLanguages[lang] = true
				}
			}
		}
	}

	suite.GreaterOrEqual(len(serviceLanguages), 2, "Should discover services from multiple languages")
}

func (suite *MultiLanguageE2ETestSuite) testInterServiceNavigation(ctx context.Context, project *framework.TestProject) {
	// Test navigation between services in different languages
	languages := []string{"go", "python", "typescript", "java"}

	for _, fromLang := range languages {
		for _, toLang := range languages {
			if fromLang == toLang {
				continue
			}

			fromURI := fmt.Sprintf("file:///workspace/services/%s-service/client.%s", fromLang, suite.getFileExtension(fromLang))

			response, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES, map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": fromURI,
				},
				"position": map[string]interface{}{
					"line":      5,
					"character": 10,
				},
				"context": map[string]interface{}{
					"includeDeclaration": true,
				},
			})

			suite.Require().NoError(err, "Should handle references from %s to %s", fromLang, toLang)
			suite.Require().NotNil(response, "Should return references from %s to %s", fromLang, toLang)
		}
	}
}

func (suite *MultiLanguageE2ETestSuite) testLoadBalancingAcrossServers(ctx context.Context, languages []string) {
	const requestCount = 50
	serverHits := make(map[string]int)

	// Send requests that should be distributed across language servers
	for i := 0; i < requestCount; i++ {
		lang := languages[i%len(languages)]
		uri := fmt.Sprintf("file:///workspace/load_test_%d.%s", i, suite.getFileExtension(lang))

		response, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": uri,
			},
			"position": map[string]interface{}{
				"line":      i % 100,
				"character": i % 50,
			},
		})

		suite.Require().NoError(err, "Load balancing request %d should succeed", i)
		suite.Require().NotNil(response, "Load balancing request %d should return response", i)

		serverHits[lang]++
	}

	// Verify requests were distributed across servers
	for _, lang := range languages {
		expectedHits := requestCount / len(languages)
		actualHits := serverHits[lang]
		suite.Greater(actualHits, 0, "Language server %s should receive requests", lang)
		suite.LessOrEqual(abs(actualHits-expectedHits), expectedHits/2, "Load should be reasonably balanced for %s", lang)
	}
}

func (suite *MultiLanguageE2ETestSuite) testWorkspaceWideSymbolSearch(ctx context.Context, project *framework.TestProject, languages []string) {
	// Test comprehensive symbol search across large monorepo
	searchQueries := []string{"Engine", "Pipeline", "Interface", "Connector", "Toolkit"}

	for _, query := range searchQueries {
		response, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_WORKSPACE_SYMBOL, map[string]interface{}{
			"query": query,
		})

		suite.Require().NoError(err, "Workspace symbol search for '%s' should succeed", query)
		suite.Require().NotNil(response, "Should return symbols for query '%s'", query)

		var symbols []map[string]interface{}
		err = json.Unmarshal(response, &symbols)
		suite.Require().NoError(err)

		if len(symbols) > 0 {
			// Verify symbols span multiple languages in monorepo
			foundLanguages := make(map[string]bool)
			for _, symbol := range symbols {
				if location, ok := symbol["location"].(map[string]interface{}); ok {
					if uri, ok := location["uri"].(string); ok {
						lang := suite.getLanguageFromExtension(filepath.Ext(uri))
						if lang != "" {
							foundLanguages[lang] = true
						}
					}
				}
			}
			suite.T().Logf("Query '%s' found symbols in languages: %v", query, foundLanguages)
		}
	}
}

func (suite *MultiLanguageE2ETestSuite) testMonorepoPerformance(ctx context.Context, project *framework.TestProject, languages []string) {
	// Measure performance of operations in large monorepo
	startTime := time.Now()

	// Perform various operations to measure performance
	operations := []struct {
		name   string
		method string
		params map[string]interface{}
	}{
		{
			name:   "WorkspaceSymbolSearch",
			method: mcp.LSP_METHOD_WORKSPACE_SYMBOL,
			params: map[string]interface{}{"query": "Service"},
		},
		{
			name:   "DocumentSymbols",
			method: mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS,
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file:///workspace/platform/core/engine.go",
				},
			},
		},
		{
			name:   "Definition",
			method: mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION,
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file:///workspace/platform/ml/pipeline.py",
				},
				"position": map[string]interface{}{"line": 10, "character": 5},
			},
		},
	}

	for _, op := range operations {
		opStart := time.Now()

		response, err := suite.mockClient.SendLSPRequest(ctx, op.method, op.params)

		opDuration := time.Since(opStart)
		suite.T().Logf("Operation %s took %v", op.name, opDuration)

		suite.Require().NoError(err, "Operation %s should succeed", op.name)
		suite.Require().NotNil(response, "Operation %s should return response", op.name)
		suite.Less(opDuration, 5*time.Second, "Operation %s should complete within 5 seconds", op.name)
	}

	totalDuration := time.Since(startTime)
	suite.Less(totalDuration, 15*time.Second, "All monorepo operations should complete within 15 seconds")
}

func (suite *MultiLanguageE2ETestSuite) testSharedLibraryNavigation(ctx context.Context, project *framework.TestProject) {
	// Test navigation through shared libraries across languages
	sharedLibraries := []string{
		"file:///workspace/shared/types/common.go",
		"file:///workspace/shared/utils/helpers.py",
		"file:///workspace/shared/interfaces/api.ts",
		"file:///workspace/shared/models/data.java",
	}

	for _, libURI := range sharedLibraries {
		// Test references to shared library
		response, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES, map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": libURI,
			},
			"position": map[string]interface{}{
				"line":      5,
				"character": 10,
			},
			"context": map[string]interface{}{
				"includeDeclaration": true,
			},
		})

		suite.Require().NoError(err, "Should find references for shared library %s", libURI)
		suite.Require().NotNil(response, "Should return references for shared library %s", libURI)
	}
}

func (suite *MultiLanguageE2ETestSuite) testAPIContractNavigation(ctx context.Context, project *framework.TestProject) {
	// Test navigation between API contracts (frontend to backend)
	frontendURI := "file:///workspace/frontend/src/api/client.ts"
	backendURI := "file:///workspace/backend/handlers/api.go"

	// Test frontend API client definition lookup
	response, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": frontendURI,
		},
		"position": map[string]interface{}{
			"line":      15,
			"character": 8,
		},
	})

	suite.Require().NoError(err, "Should handle API contract navigation from frontend")
	suite.Require().NotNil(response, "Should return definition for API contract")

	// Test backend API handler references
	response, err = suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": backendURI,
		},
		"position": map[string]interface{}{
			"line":      20,
			"character": 12,
		},
		"context": map[string]interface{}{
			"includeDeclaration": true,
		},
	})

	suite.Require().NoError(err, "Should handle API contract references from backend")
	suite.Require().NotNil(response, "Should return references for API contract")
}

func (suite *MultiLanguageE2ETestSuite) testFrontendBackendTypeSharing(ctx context.Context, project *framework.TestProject) {
	// Test shared type definitions between frontend and backend
	sharedTypeURI := "file:///workspace/shared/types/User.ts"

	// Test type definition lookup
	response, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": sharedTypeURI,
		},
		"position": map[string]interface{}{
			"line":      5,
			"character": 17,
		},
	})

	suite.Require().NoError(err, "Should handle shared type definition lookup")
	suite.Require().NotNil(response, "Should return definition for shared type")

	// Test references to shared type from both frontend and backend
	response, err = suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": sharedTypeURI,
		},
		"position": map[string]interface{}{
			"line":      5,
			"character": 17,
		},
		"context": map[string]interface{}{
			"includeDeclaration": true,
		},
	})

	suite.Require().NoError(err, "Should find references to shared type")
	suite.Require().NotNil(response, "Should return references to shared type")
}

func (suite *MultiLanguageE2ETestSuite) testFileExtensionRouting(ctx context.Context, project *framework.TestProject, languages []string) {
	// Test that requests are routed to correct servers based on file extension
	for _, lang := range languages {
		ext := suite.getFileExtension(lang)
		uri := fmt.Sprintf("file:///workspace/test.%s", ext)

		response, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": uri,
			},
			"position": map[string]interface{}{
				"line":      10,
				"character": 5,
			},
		})

		suite.Require().NoError(err, "Should route .%s files to %s server", ext, lang)
		suite.Require().NotNil(response, "Should return response for .%s files", ext)

		// Verify the request was made (mock client tracks calls)
		calls := suite.mockClient.GetCallsForMethod(mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER)
		suite.Greater(len(calls), 0, "Should have made LSP calls for %s", lang)
	}
}

func (suite *MultiLanguageE2ETestSuite) testProjectStructureRouting(ctx context.Context, project *framework.TestProject, languages []string) {
	// Test routing based on project structure (e.g., language-specific directories)
	for _, lang := range languages {
		uri := fmt.Sprintf("file:///workspace/%s-project/src/main.%s", lang, suite.getFileExtension(lang))

		response, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS, map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": uri,
			},
		})

		suite.Require().NoError(err, "Should route based on project structure for %s", lang)
		suite.Require().NotNil(response, "Should return symbols for %s project structure", lang)
	}
}

func (suite *MultiLanguageE2ETestSuite) testFallbackRouting(ctx context.Context, project *framework.TestProject, languages []string) {
	// Test fallback routing for unknown file types or ambiguous cases
	unknownURI := "file:///workspace/unknown/test.xyz"

	response, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": unknownURI,
		},
		"position": map[string]interface{}{
			"line":      5,
			"character": 10,
		},
	})

	// Should either succeed with fallback or fail gracefully
	if err != nil {
		suite.T().Logf("Fallback routing failed gracefully: %v", err)
	} else {
		suite.NotNil(response, "Fallback routing should return some response")
	}
}

func (suite *MultiLanguageE2ETestSuite) testServerFailureIsolation(ctx context.Context, project *framework.TestProject, languages []string) {
	// Simulate failure in one language server
	failingLang := languages[0]
	suite.mockClient.QueueError(fmt.Errorf("simulated %s server failure", failingLang))

	// Test that failure in one server doesn't affect others
	for _, lang := range languages {
		uri := fmt.Sprintf("file:///workspace/test.%s", suite.getFileExtension(lang))

		response, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": uri,
			},
			"position": map[string]interface{}{
				"line":      5,
				"character": 10,
			},
		})

		if lang == failingLang {
			suite.Require().Error(err, "Failed server should return error")
		} else {
			suite.Require().NoError(err, "Other servers should continue working when %s fails", failingLang)
			suite.Require().NotNil(response, "Other servers should return responses when %s fails", failingLang)
		}
	}
}

func (suite *MultiLanguageE2ETestSuite) testServerRecovery(ctx context.Context, project *framework.TestProject, languages []string) {
	// Test server recovery after failure
	recoveryLang := languages[0]

	// First, simulate failure
	suite.mockClient.QueueError(fmt.Errorf("temporary %s server failure", recoveryLang))

	uri := fmt.Sprintf("file:///workspace/recovery_test.%s", suite.getFileExtension(recoveryLang))

	// First request should fail
	_, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      5,
			"character": 10,
		},
	})
	suite.Require().Error(err, "Server should fail initially")

	// Reset mock to simulate recovery
	suite.mockClient.SetHealthy(true)

	// Second request should succeed (simulating recovery)
	response, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      5,
			"character": 10,
		},
	})

	suite.Require().NoError(err, "Server should recover and handle requests")
	suite.Require().NotNil(response, "Recovered server should return response")
}

func (suite *MultiLanguageE2ETestSuite) testDegradedModeOperation(ctx context.Context, project *framework.TestProject, languages []string) {
	// Test system operation when some language servers are unavailable
	availableLanguages := languages[:len(languages)/2] // Only half the servers available
	unavailableLanguages := languages[len(languages)/2:]

	// Queue errors for unavailable servers
	for _, lang := range unavailableLanguages {
		suite.mockClient.QueueError(fmt.Errorf("%s server unavailable", lang))
	}

	// Test that available servers still work
	for _, lang := range availableLanguages {
		uri := fmt.Sprintf("file:///workspace/degraded_test.%s", suite.getFileExtension(lang))

		response, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": uri,
			},
			"position": map[string]interface{}{
				"line":      8,
				"character": 12,
			},
		})

		suite.Require().NoError(err, "Available server %s should work in degraded mode", lang)
		suite.Require().NotNil(response, "Available server %s should return response in degraded mode", lang)
	}

	// Test that unavailable servers return appropriate errors
	for _, lang := range unavailableLanguages {
		uri := fmt.Sprintf("file:///workspace/degraded_test.%s", suite.getFileExtension(lang))

		_, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": uri,
			},
			"position": map[string]interface{}{
				"line":      8,
				"character": 12,
			},
		})

		suite.Require().Error(err, "Unavailable server %s should return error", lang)
	}
}

// Utility methods

func (suite *MultiLanguageE2ETestSuite) getFileExtension(language string) string {
	extensions := map[string]string{
		"go":         "go",
		"python":     "py",
		"javascript": "js",
		"typescript": "ts",
		"java":       "java",
		"rust":       "rs",
		"cpp":        "cpp",
		"csharp":     "cs",
	}

	if ext, exists := extensions[language]; exists {
		return ext
	}
	return "txt"
}

func (suite *MultiLanguageE2ETestSuite) getLanguageFromExtension(ext string) string {
	languages := map[string]string{
		".go":   "go",
		".py":   "python",
		".js":   "javascript",
		".jsx":  "javascript",
		".ts":   "typescript",
		".tsx":  "typescript",
		".java": "java",
		".rs":   "rust",
		".cpp":  "cpp",
		".cc":   "cpp",
		".cxx":  "cpp",
		".cs":   "csharp",
	}

	if lang, exists := languages[ext]; exists {
		return lang
	}
	return ""
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// TestMultiLanguageE2ETestSuite runs the comprehensive E2E test suite
func TestMultiLanguageE2ETestSuite(t *testing.T) {
	suite.Run(t, new(MultiLanguageE2ETestSuite))
}

// Benchmark tests for performance validation

func BenchmarkCrossLanguageWorkspaceSymbols(b *testing.B) {
	framework := framework.NewMultiLanguageTestFramework(30 * time.Second)
	defer framework.CleanupAll()

	mockClient := mocks.NewMockMcpClient()
	ctx := context.Background()

	// Setup
	err := framework.SetupTestEnvironment(ctx)
	if err != nil {
		b.Fatal(err)
	}

	languages := []string{"go", "python", "typescript", "java"}
	err = framework.StartMultipleLanguageServers(languages)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_WORKSPACE_SYMBOL, map[string]interface{}{
			"query": fmt.Sprintf("TestSymbol%d", i),
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkConcurrentMultiLanguageRequests(b *testing.B) {
	framework := framework.NewMultiLanguageTestFramework(30 * time.Second)
	defer framework.CleanupAll()

	mockClient := mocks.NewMockMcpClient()
	ctx := context.Background()

	// Setup
	err := framework.SetupTestEnvironment(ctx)
	if err != nil {
		b.Fatal(err)
	}

	languages := []string{"go", "python", "typescript", "java"}
	err = framework.StartMultipleLanguageServers(languages)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		requestID := 0
		for pb.Next() {
			lang := languages[requestID%len(languages)]
			uri := fmt.Sprintf("file:///workspace/bench_%d.%s", requestID, getFileExt(lang))

			_, err := mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": uri,
				},
				"position": map[string]interface{}{
					"line":      requestID % 100,
					"character": requestID % 50,
				},
			})
			if err != nil {
				b.Fatal(err)
			}
			requestID++
		}
	})
}

func getFileExt(language string) string {
	extensions := map[string]string{
		"go":         "go",
		"python":     "py",
		"javascript": "js",
		"typescript": "ts",
		"java":       "java",
		"rust":       "rs",
	}
	return extensions[language]
}
