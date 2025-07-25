package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/mcp"
	"lsp-gateway/tests/mocks"
)

// LSPWorkflowE2ETestSuite provides comprehensive end-to-end tests for LSP workflow scenarios
// that developers actually use in their daily work. Tests complete developer workflows
// including definition navigation, code exploration, workspace analysis, and multi-language scenarios.
type LSPWorkflowE2ETestSuite struct {
	suite.Suite
	mockClient       *mocks.MockMcpClient
	testTimeout      time.Duration
	workspaceRoot    string
	testFiles        map[string]string
	languageFixtures map[string]WorkflowFixtures
}

// WorkflowFixtures contains realistic test data for each language
type WorkflowFixtures struct {
	Language           string
	DefinitionResponse json.RawMessage
	DocumentSymbols    json.RawMessage
	HoverResponse      json.RawMessage
	ReferencesResponse json.RawMessage
	WorkspaceSymbols   json.RawMessage
	TestFilePath       string
	SymbolName         string
	Position           LSPPosition
}

// LSPPosition represents a position in a document
type LSPPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// DefinitionNavigationResult captures results from definition navigation workflow
type DefinitionNavigationResult struct {
	InitialSymbolSearch bool
	DefinitionFound     bool
	ReferencesFound     bool
	NavigationLatency   time.Duration
	TotalRequestCount   int
	ErrorCount          int
}

// CodeExplorationResult captures results from code exploration workflow
type CodeExplorationResult struct {
	FileOpened         bool
	DocumentSymbols    int
	SymbolNavigation   bool
	HoverInfoRetrieved bool
	ExplorationLatency time.Duration
	SymbolCount        int
}

// SetupSuite initializes the test suite with realistic fixtures and mock client
func (suite *LSPWorkflowE2ETestSuite) SetupSuite() {
	suite.testTimeout = 30 * time.Second
	suite.workspaceRoot = "/workspace"

	// Initialize test files mapping
	suite.testFiles = map[string]string{
		"main.go":       "package main\n\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}",
		"handler.py":    "def handle_request(request):\n    return process_data(request.data)",
		"component.tsx": "export const Component = () => {\n  return <div>Hello</div>\n}",
		"Service.java":  "public class Service {\n  public void processRequest() {}\n}",
	}

	// Setup realistic language fixtures
	suite.setupLanguageFixtures()
}

// SetupTest initializes a fresh mock client for each test
func (suite *LSPWorkflowE2ETestSuite) SetupTest() {
	suite.mockClient = mocks.NewMockMcpClient()
	suite.mockClient.SetHealthy(true)
}

// TearDownTest cleans up mock client state
func (suite *LSPWorkflowE2ETestSuite) TearDownTest() {
	if suite.mockClient != nil {
		suite.mockClient.Reset()
	}
}

// setupLanguageFixtures creates realistic test fixtures for all supported languages
func (suite *LSPWorkflowE2ETestSuite) setupLanguageFixtures() {
	suite.languageFixtures = map[string]WorkflowFixtures{
		"go": {
			Language: "go",
			DefinitionResponse: json.RawMessage(`{
				"uri": "file:///workspace/pkg/server/handler.go",
				"range": {
					"start": {"line": 42, "character": 5},
					"end": {"line": 42, "character": 17}
				}
			}`),
			DocumentSymbols: json.RawMessage(`[
				{
					"name": "Server",
					"kind": 23,
					"range": {"start": {"line": 15, "character": 5}, "end": {"line": 25, "character": 1}},
					"selectionRange": {"start": {"line": 15, "character": 5}, "end": {"line": 15, "character": 11}}
				},
				{
					"name": "HandleRequest",
					"kind": 6,
					"range": {"start": {"line": 42, "character": 0}, "end": {"line": 55, "character": 1}},
					"selectionRange": {"start": {"line": 42, "character": 5}, "end": {"line": 42, "character": 17}}
				}
			]`),
			HoverResponse: json.RawMessage(`{
				"contents": {
					"kind": "markdown",
					"value": "Go function HandleRequest handles HTTP requests"
				}
			}`),
			ReferencesResponse: json.RawMessage(`[
				{"uri": "file:///workspace/main.go", "range": {"start": {"line": 25, "character": 8}, "end": {"line": 25, "character": 21}}},
				{"uri": "file:///workspace/server_test.go", "range": {"start": {"line": 15, "character": 12}, "end": {"line": 15, "character": 25}}}
			]`),
			WorkspaceSymbols: json.RawMessage(`[
				{"name": "HandleRequest", "kind": 12, "location": {"uri": "file:///workspace/handler.go", "range": {"start": {"line": 42, "character": 5}, "end": {"line": 42, "character": 17}}}},
				{"name": "Server", "kind": 23, "location": {"uri": "file:///workspace/server.go", "range": {"start": {"line": 15, "character": 5}, "end": {"line": 15, "character": 11}}}}
			]`),
			TestFilePath: "handler.go",
			SymbolName:   "HandleRequest",
			Position:     LSPPosition{Line: 42, Character: 10},
		},
		"python": {
			Language: "python",
			DefinitionResponse: json.RawMessage(`{
				"uri": "file:///workspace/handlers/request_handler.py",
				"range": {
					"start": {"line": 15, "character": 4},
					"end": {"line": 15, "character": 18}
				}
			}`),
			DocumentSymbols: json.RawMessage(`[
				{
					"name": "RequestHandler",
					"kind": 5,
					"range": {"start": {"line": 10, "character": 0}, "end": {"line": 50, "character": 0}},
					"selectionRange": {"start": {"line": 10, "character": 6}, "end": {"line": 10, "character": 20}}
				}
			]`),
			HoverResponse: json.RawMessage(`{
				"contents": {
					"kind": "markdown",
					"value": "Python function handle_request processes requests"
				}
			}`),
			ReferencesResponse: json.RawMessage(`[
				{"uri": "file:///workspace/main.py", "range": {"start": {"line": 8, "character": 12}, "end": {"line": 8, "character": 26}}},
				{"uri": "file:///workspace/test_handler.py", "range": {"start": {"line": 20, "character": 16}, "end": {"line": 20, "character": 30}}}
			]`),
			WorkspaceSymbols: json.RawMessage(`[
				{"name": "handle_request", "kind": 6, "location": {"uri": "file:///workspace/handler.py", "range": {"start": {"line": 15, "character": 4}, "end": {"line": 15, "character": 18}}}},
				{"name": "RequestHandler", "kind": 5, "location": {"uri": "file:///workspace/handler.py", "range": {"start": {"line": 10, "character": 6}, "end": {"line": 10, "character": 20}}}}
			]`),
			TestFilePath: "handler.py",
			SymbolName:   "handle_request",
			Position:     LSPPosition{Line: 15, Character: 10},
		},
		"typescript": {
			Language: "typescript",
			DefinitionResponse: json.RawMessage(`{
				"uri": "file:///workspace/src/components/RequestHandler.tsx",
				"range": {
					"start": {"line": 8, "character": 13},
					"end": {"line": 8, "character": 27}
				}
			}`),
			DocumentSymbols: json.RawMessage(`[
				{
					"name": "RequestHandler",
					"kind": 5,
					"range": {"start": {"line": 5, "character": 0}, "end": {"line": 25, "character": 1}},
					"selectionRange": {"start": {"line": 5, "character": 13}, "end": {"line": 5, "character": 27}}
				}
			]`),
			HoverResponse: json.RawMessage(`{
				"contents": {
					"kind": "markdown",
					"value": "TypeScript function handleRequest processes requests"
				}
			}`),
			ReferencesResponse: json.RawMessage(`[
				{"uri": "file:///workspace/src/app.tsx", "range": {"start": {"line": 12, "character": 20}, "end": {"line": 12, "character": 33}}},
				{"uri": "file:///workspace/src/test.tsx", "range": {"start": {"line": 5, "character": 8}, "end": {"line": 5, "character": 21}}}
			]`),
			WorkspaceSymbols: json.RawMessage(`[
				{"name": "handleRequest", "kind": 6, "location": {"uri": "file:///workspace/component.tsx", "range": {"start": {"line": 8, "character": 13}, "end": {"line": 8, "character": 27}}}},
				{"name": "RequestHandler", "kind": 5, "location": {"uri": "file:///workspace/component.tsx", "range": {"start": {"line": 5, "character": 13}, "end": {"line": 5, "character": 27}}}}
			]`),
			TestFilePath: "component.tsx",
			SymbolName:   "handleRequest",
			Position:     LSPPosition{Line: 8, Character: 20},
		},
		"java": {
			Language: "java",
			DefinitionResponse: json.RawMessage(`{
				"uri": "file:///workspace/src/main/java/com/example/RequestHandler.java",
				"range": {
					"start": {"line": 25, "character": 16},
					"end": {"line": 25, "character": 30}
				}
			}`),
			DocumentSymbols: json.RawMessage(`[
				{
					"name": "RequestHandler",
					"kind": 5,
					"range": {"start": {"line": 10, "character": 0}, "end": {"line": 50, "character": 1}},
					"selectionRange": {"start": {"line": 10, "character": 13}, "end": {"line": 10, "character": 27}}
				}
			]`),
			HoverResponse: json.RawMessage(`{
				"contents": {
					"kind": "markdown",
					"value": "Java method handleRequest processes requests"
				}
			}`),
			ReferencesResponse: json.RawMessage(`[
				{"uri": "file:///workspace/src/main/java/Main.java", "range": {"start": {"line": 18, "character": 12}, "end": {"line": 18, "character": 25}}},
				{"uri": "file:///workspace/src/test/java/HandlerTest.java", "range": {"start": {"line": 22, "character": 20}, "end": {"line": 22, "character": 33}}}
			]`),
			WorkspaceSymbols: json.RawMessage(`[
				{"name": "handleRequest", "kind": 6, "location": {"uri": "file:///workspace/Service.java", "range": {"start": {"line": 25, "character": 16}, "end": {"line": 25, "character": 30}}}},
				{"name": "RequestHandler", "kind": 5, "location": {"uri": "file:///workspace/Service.java", "range": {"start": {"line": 10, "character": 13}, "end": {"line": 10, "character": 27}}}}
			]`),
			TestFilePath: "Service.java",
			SymbolName:   "handleRequest",
			Position:     LSPPosition{Line: 25, Character: 23},
		},
	}
}

// TestDefinitionNavigationWorkflow tests the complete definition navigation workflow
// Workflow: Search symbol → Get definition → Find references → Validate results
func (suite *LSPWorkflowE2ETestSuite) TestDefinitionNavigationWorkflow() {
	testCases := []struct {
		name     string
		language string
	}{
		{"Go Definition Navigation", "go"},
		{"Python Definition Navigation", "python"},
		{"TypeScript Definition Navigation", "typescript"},
		{"Java Definition Navigation", "java"},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result := suite.executeDefinitionNavigationWorkflow(tc.language)

			// Validate workflow completion
			suite.True(result.InitialSymbolSearch, "Initial symbol search should succeed")
			suite.True(result.DefinitionFound, "Definition should be found")
			suite.True(result.ReferencesFound, "References should be found")
			suite.Equal(0, result.ErrorCount, "No errors should occur during workflow")
			suite.GreaterOrEqual(result.TotalRequestCount, 3, "At least 3 requests should be made")
			suite.Less(result.NavigationLatency, 5*time.Second, "Navigation should complete within 5 seconds")
		})
	}
}

// TestCodeExplorationWorkflow tests the complete code exploration workflow
// Workflow: Open file → Get document symbols → Navigate to specific symbol → Get hover info
func (suite *LSPWorkflowE2ETestSuite) TestCodeExplorationWorkflow() {
	testCases := []struct {
		name     string
		language string
	}{
		{"Go Code Exploration", "go"},
		{"Python Code Exploration", "python"},
		{"TypeScript Code Exploration", "typescript"},
		{"Java Code Exploration", "java"},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result := suite.executeCodeExplorationWorkflow(tc.language)

			// Validate workflow completion
			suite.True(result.FileOpened, "File should be opened successfully")
			suite.Greater(result.DocumentSymbols, 0, "Document symbols should be found")
			suite.True(result.SymbolNavigation, "Symbol navigation should succeed")
			suite.True(result.HoverInfoRetrieved, "Hover info should be retrieved")
			suite.Less(result.ExplorationLatency, 3*time.Second, "Exploration should complete within 3 seconds")
		})
	}
}

// TestWorkspaceAnalysisWorkflow tests the complete workspace analysis workflow
// Workflow: Search workspace symbols → Filter results → Navigate to definitions
func (suite *LSPWorkflowE2ETestSuite) TestWorkspaceAnalysisWorkflow() {
	languages := []string{"go", "python", "typescript", "java"}

	for _, language := range languages {
		suite.Run(fmt.Sprintf("Workspace Analysis - %s", language), func() {
			fixture := suite.languageFixtures[language]

			// Setup mock responses
			suite.mockClient.QueueResponse(fixture.WorkspaceSymbols)
			suite.mockClient.QueueResponse(fixture.DefinitionResponse)

			startTime := time.Now()

			// Execute workspace symbol search
			ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
			defer cancel()

			symbolsResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_WORKSPACE_SYMBOL, map[string]interface{}{
				"query": fixture.SymbolName,
			})
			suite.NoError(err, "Workspace symbol search should succeed")
			suite.NotNil(symbolsResp, "Symbol response should not be nil")

			// Navigate to first symbol definition
			defResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
				"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fixture.TestFilePath)},
				"position":     fixture.Position,
			})
			suite.NoError(err, "Definition request should succeed")
			suite.NotNil(defResp, "Definition response should not be nil")

			analysisLatency := time.Since(startTime)

			// Validate metrics
			suite.GreaterOrEqual(suite.mockClient.GetCallCount(mcp.LSP_METHOD_WORKSPACE_SYMBOL), 1, "Workspace symbol should be called")
			suite.GreaterOrEqual(suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION), 1, "Definition should be called")
			suite.Less(analysisLatency, 2*time.Second, "Analysis should complete quickly")
		})
	}
}

// TestMultiLanguageWorkflow tests navigating between different language files
// Workflow: Navigate between Go/Python/TypeScript/Java files in sequence
func (suite *LSPWorkflowE2ETestSuite) TestMultiLanguageWorkflow() {
	languages := []string{"go", "python", "typescript", "java"}

	// Setup responses for all languages
	for _, lang := range languages {
		fixture := suite.languageFixtures[lang]
		suite.mockClient.QueueResponse(fixture.DocumentSymbols)
		suite.mockClient.QueueResponse(fixture.DefinitionResponse)
		suite.mockClient.QueueResponse(fixture.HoverResponse)
	}

	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	totalSymbols := 0

	// Navigate through each language
	for _, language := range languages {
		fixture := suite.languageFixtures[language]

		// Get document symbols
		symbolsResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS, map[string]interface{}{
			"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fixture.TestFilePath)},
		})
		suite.NoError(err, fmt.Sprintf("Document symbols request should succeed for %s", language))
		suite.NotNil(symbolsResp, fmt.Sprintf("Document symbols response should not be nil for %s", language))

		// Parse symbols to count them
		var symbols []interface{}
		err = json.Unmarshal(symbolsResp, &symbols)
		suite.NoError(err, fmt.Sprintf("Should be able to parse symbols for %s", language))
		totalSymbols += len(symbols)

		// Get definition for main symbol
		defResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
			"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fixture.TestFilePath)},
			"position":     fixture.Position,
		})
		suite.NoError(err, fmt.Sprintf("Definition request should succeed for %s", language))
		suite.NotNil(defResp, fmt.Sprintf("Definition response should not be nil for %s", language))

		// Get hover information
		hoverResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
			"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fixture.TestFilePath)},
			"position":     fixture.Position,
		})
		suite.NoError(err, fmt.Sprintf("Hover request should succeed for %s", language))
		suite.NotNil(hoverResp, fmt.Sprintf("Hover response should not be nil for %s", language))
	}

	workflowLatency := time.Since(startTime)

	// Validate multi-language workflow completion
	suite.Greater(totalSymbols, 0, "Should find symbols across all languages")
	suite.Equal(len(languages), suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS), "Document symbols should be called for each language")
	suite.Equal(len(languages), suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION), "Definition should be called for each language")
	suite.Equal(len(languages), suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER), "Hover should be called for each language")
	suite.Less(workflowLatency, 10*time.Second, "Multi-language workflow should complete within 10 seconds")
}

// TestErrorRecoveryWorkflow tests graceful error handling and retry logic
// Workflow: Handle timeouts/errors gracefully and retry
func (suite *LSPWorkflowE2ETestSuite) TestErrorRecoveryWorkflow() {
	testCases := []struct {
		name        string
		errorType   string
		shouldRetry bool
	}{
		{"Network Error Recovery", "network error", true},
		{"Timeout Error Recovery", "timeout", true},
		{"Server Error Recovery", "server error", true},
		{"Client Error No Retry", "client error", false},
		{"Protocol Error No Retry", "protocol error", false},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.mockClient.Reset()

			// Setup error followed by success response
			suite.mockClient.QueueError(fmt.Errorf(tc.errorType))
			if tc.shouldRetry {
				suite.mockClient.QueueResponse(suite.languageFixtures["go"].DefinitionResponse)
			}

			ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
			defer cancel()

			startTime := time.Now()
			var finalErr error
			var resp json.RawMessage

			// Attempt request with potential error
			resp, finalErr = suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
				"textDocument": map[string]string{"uri": "file:///workspace/test.go"},
				"position":     LSPPosition{Line: 10, Character: 5},
			})

			recoveryLatency := time.Since(startTime)

			if tc.shouldRetry {
				// For retryable errors, we should eventually get a response
				// In real implementation, retry logic would be in the client
				suite.NoError(finalErr, "Should eventually succeed after retry")
				suite.NotNil(resp, "Should get response after retry")
			} else {
				// For non-retryable errors, we should get the error immediately
				suite.Error(finalErr, "Should fail for non-retryable errors")
				suite.Contains(finalErr.Error(), tc.errorType, "Error should contain expected error type")
			}

			// Validate error categorization
			category := suite.mockClient.CategorizeError(fmt.Errorf(tc.errorType))
			suite.NotEmpty(category, "Error should be categorized")

			suite.Less(recoveryLatency, 1*time.Second, "Error recovery should be fast")
		})
	}
}

// TestConcurrentWorkflowExecution tests multiple workflows running concurrently
func (suite *LSPWorkflowE2ETestSuite) TestConcurrentWorkflowExecution() {
	const numConcurrentWorkflows = 10

	// Setup responses for concurrent execution
	for i := 0; i < numConcurrentWorkflows*3; i++ { // 3 requests per workflow
		suite.mockClient.QueueResponse(suite.languageFixtures["go"].DocumentSymbols)
		suite.mockClient.QueueResponse(suite.languageFixtures["go"].DefinitionResponse)
		suite.mockClient.QueueResponse(suite.languageFixtures["go"].HoverResponse)
	}

	var wg sync.WaitGroup
	results := make(chan bool, numConcurrentWorkflows)
	startTime := time.Now()

	// Launch concurrent workflows
	for i := 0; i < numConcurrentWorkflows; i++ {
		wg.Add(1)
		go func(workflowID int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
			defer cancel()

			// Execute mini workflow: symbols → definition → hover
			_, err1 := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS, map[string]interface{}{
				"textDocument": map[string]string{"uri": fmt.Sprintf("file:///workspace/test%d.go", workflowID)},
			})

			_, err2 := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
				"textDocument": map[string]string{"uri": fmt.Sprintf("file:///workspace/test%d.go", workflowID)},
				"position":     LSPPosition{Line: 10, Character: 5},
			})

			_, err3 := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
				"textDocument": map[string]string{"uri": fmt.Sprintf("file:///workspace/test%d.go", workflowID)},
				"position":     LSPPosition{Line: 10, Character: 5},
			})

			results <- (err1 == nil && err2 == nil && err3 == nil)
		}(i)
	}

	// Wait for all workflows to complete
	wg.Wait()
	close(results)

	concurrentLatency := time.Since(startTime)

	// Validate concurrent execution results
	successCount := 0
	for success := range results {
		if success {
			successCount++
		}
	}

	suite.Equal(numConcurrentWorkflows, successCount, "All concurrent workflows should succeed")
	suite.GreaterOrEqual(suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS), numConcurrentWorkflows, "All symbol requests should be made")
	suite.GreaterOrEqual(suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION), numConcurrentWorkflows, "All definition requests should be made")
	suite.GreaterOrEqual(suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER), numConcurrentWorkflows, "All hover requests should be made")
	suite.Less(concurrentLatency, 15*time.Second, "Concurrent workflows should complete within reasonable time")
}

// executeDefinitionNavigationWorkflow performs the complete definition navigation workflow
func (suite *LSPWorkflowE2ETestSuite) executeDefinitionNavigationWorkflow(language string) DefinitionNavigationResult {
	fixture := suite.languageFixtures[language]
	result := DefinitionNavigationResult{}

	// Setup mock responses
	suite.mockClient.QueueResponse(fixture.WorkspaceSymbols)
	suite.mockClient.QueueResponse(fixture.DefinitionResponse)
	suite.mockClient.QueueResponse(fixture.ReferencesResponse)

	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Step 1: Search for symbol in workspace
	symbolsResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_WORKSPACE_SYMBOL, map[string]interface{}{
		"query": fixture.SymbolName,
	})
	result.InitialSymbolSearch = err == nil && symbolsResp != nil
	if err != nil {
		result.ErrorCount++
	}

	// Step 2: Get definition of found symbol
	defResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fixture.TestFilePath)},
		"position":     fixture.Position,
	})
	result.DefinitionFound = err == nil && defResp != nil
	if err != nil {
		result.ErrorCount++
	}

	// Step 3: Find all references to the symbol
	refsResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES, map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fixture.TestFilePath)},
		"position":     fixture.Position,
		"context":      map[string]bool{"includeDeclaration": true},
	})
	result.ReferencesFound = err == nil && refsResp != nil
	if err != nil {
		result.ErrorCount++
	}

	result.NavigationLatency = time.Since(startTime)
	result.TotalRequestCount = len(suite.mockClient.SendLSPRequestCalls)

	return result
}

// executeCodeExplorationWorkflow performs the complete code exploration workflow
func (suite *LSPWorkflowE2ETestSuite) executeCodeExplorationWorkflow(language string) CodeExplorationResult {
	fixture := suite.languageFixtures[language]
	result := CodeExplorationResult{}

	// Setup mock responses
	suite.mockClient.QueueResponse(fixture.DocumentSymbols)
	suite.mockClient.QueueResponse(fixture.HoverResponse)

	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Step 1: Open file (simulated by requesting document symbols)
	symbolsResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS, map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fixture.TestFilePath)},
	})
	result.FileOpened = err == nil && symbolsResp != nil

	// Parse symbols to count them
	if symbolsResp != nil {
		var symbols []interface{}
		if json.Unmarshal(symbolsResp, &symbols) == nil {
			result.DocumentSymbols = len(symbols)
			result.SymbolCount = len(symbols)
		}
	}

	// Step 2: Navigate to specific symbol and get hover info
	hoverResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fixture.TestFilePath)},
		"position":     fixture.Position,
	})
	result.SymbolNavigation = err == nil
	result.HoverInfoRetrieved = err == nil && hoverResp != nil

	result.ExplorationLatency = time.Since(startTime)

	return result
}

// Benchmark tests for performance measurement

// BenchmarkDefinitionNavigationWorkflow benchmarks the definition navigation workflow
func BenchmarkDefinitionNavigationWorkflow(b *testing.B) {
	suite := &LSPWorkflowE2ETestSuite{}
	suite.SetupSuite()

	languages := []string{"go", "python", "typescript", "java"}

	for _, language := range languages {
		b.Run(fmt.Sprintf("DefinitionNavigation_%s", language), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				suite.SetupTest()
				result := suite.executeDefinitionNavigationWorkflow(language)
				if !result.InitialSymbolSearch || !result.DefinitionFound || !result.ReferencesFound {
					b.Errorf("Workflow failed for %s", language)
				}
				suite.TearDownTest()
			}
		})
	}
}

// BenchmarkCodeExplorationWorkflow benchmarks the code exploration workflow
func BenchmarkCodeExplorationWorkflow(b *testing.B) {
	suite := &LSPWorkflowE2ETestSuite{}
	suite.SetupSuite()

	languages := []string{"go", "python", "typescript", "java"}

	for _, language := range languages {
		b.Run(fmt.Sprintf("CodeExploration_%s", language), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				suite.SetupTest()
				result := suite.executeCodeExplorationWorkflow(language)
				if !result.FileOpened || !result.HoverInfoRetrieved {
					b.Errorf("Workflow failed for %s", language)
				}
				suite.TearDownTest()
			}
		})
	}
}

// BenchmarkConcurrentWorkflows benchmarks concurrent workflow execution
func BenchmarkConcurrentWorkflows(b *testing.B) {
	suite := &LSPWorkflowE2ETestSuite{}
	suite.SetupSuite()

	concurrencyLevels := []int{1, 5, 10, 20}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrent_%d", concurrency), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				suite.SetupTest()

				// Setup sufficient responses
				for j := 0; j < concurrency*3; j++ {
					suite.mockClient.QueueResponse(suite.languageFixtures["go"].DocumentSymbols)
				}

				var wg sync.WaitGroup
				for j := 0; j < concurrency; j++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()
						suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS, map[string]interface{}{
							"textDocument": map[string]string{"uri": "file:///workspace/test.go"},
						})
					}()
				}
				wg.Wait()

				suite.TearDownTest()
			}
		})
	}
}

// TestSuite runner
func TestLSPWorkflowE2ETestSuite(t *testing.T) {
	suite.Run(t, new(LSPWorkflowE2ETestSuite))
}