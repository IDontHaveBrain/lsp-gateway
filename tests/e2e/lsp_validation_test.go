package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/mcp"
	"lsp-gateway/tests/e2e/helpers"
	"lsp-gateway/tests/e2e/testutils"
)

type LSPValidationTestSuite struct {
	suite.Suite
	httpClient     *testutils.HttpClient
	assertHelper   *e2e_test.AssertionHelper
	gatewayCmd     *exec.Cmd
	gatewayPort    int
	configPath     string
	tempDir        string
	projectRoot    string
	testTimeout    time.Duration
}

type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type LSPResponseValidation struct {
	Method           string
	ExpectedFields   []string
	OptionalFields   []string  
	ArrayFields      []string
	ValidationFunc   func(response json.RawMessage) error
}

func (suite *LSPValidationTestSuite) SetupSuite() {
	suite.testTimeout = 60 * time.Second
	
	var err error
	suite.projectRoot, err = testutils.GetProjectRoot()
	suite.Require().NoError(err)
	
	suite.tempDir, err = os.MkdirTemp("", "lsp-validation-test-*")
	suite.Require().NoError(err)
	
	suite.gatewayPort, err = testutils.FindAvailablePort()
	suite.Require().NoError(err)

	suite.assertHelper = e2e_test.NewAssertionHelper(suite.T())
	
	suite.createTestConfig()
	suite.setupTestFiles()
}

func (suite *LSPValidationTestSuite) SetupTest() {
	config := testutils.HttpClientConfig{
		BaseURL:         fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
		Timeout:         15 * time.Second,
		MaxRetries:      3,
		RetryDelay:      500 * time.Millisecond,
		EnableLogging:   true,
		EnableRecording: true,
		WorkspaceID:     "validation-test-workspace",
		ProjectPath:     suite.tempDir,
	}
	suite.httpClient = testutils.NewHttpClient(config)
}

func (suite *LSPValidationTestSuite) TearDownTest() {
	if suite.httpClient != nil {
		suite.httpClient.Close()
	}
}

func (suite *LSPValidationTestSuite) TearDownSuite() {
	if suite.gatewayCmd != nil && suite.gatewayCmd.Process != nil {
		suite.gatewayCmd.Process.Signal(syscall.SIGTERM)
		suite.gatewayCmd.Wait()
	}
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
}

func (suite *LSPValidationTestSuite) createTestConfig() {
	configContent := fmt.Sprintf(`
servers:
- name: go-lsp
  languages:
  - go
  command: gopls
  args: []
  transport: stdio
  root_markers:
  - go.mod
  priority: 1
  weight: 1.0
- name: python-lsp
  languages:
  - python
  command: pylsp
  args: []
  transport: stdio
  root_markers:
  - setup.py
  priority: 1
  weight: 1.0
port: %d
timeout: 30s
max_concurrent_requests: 100
`, suite.gatewayPort)

	var err error
	suite.configPath, _, err = testutils.CreateTempConfig(configContent)
	suite.Require().NoError(err)
}

func (suite *LSPValidationTestSuite) setupTestFiles() {
	// Create a simple Go file for testing
	goContent := `package main

import "fmt"

type Server struct {
	Name string
	Port int
}

func (s *Server) Start() error {
	fmt.Printf("Starting %s on port %d\n", s.Name, s.Port)
	return nil
}

func NewServer(name string, port int) *Server {
	return &Server{Name: name, Port: port}
}

func main() {
	server := NewServer("test", 8080)
	server.Start()
}`

	goFile := filepath.Join(suite.tempDir, "main.go")
	err := os.WriteFile(goFile, []byte(goContent), 0644)
	suite.Require().NoError(err)

	// Create go.mod file
	goModContent := "module test\n\ngo 1.21\n"
	goModFile := filepath.Join(suite.tempDir, "go.mod")
	err = os.WriteFile(goModFile, []byte(goModContent), 0644)
	suite.Require().NoError(err)
}

func (suite *LSPValidationTestSuite) startGatewayServer() {
	binaryPath := filepath.Join(suite.projectRoot, "bin", "lsp-gateway")
	suite.gatewayCmd = exec.Command(binaryPath, "server", "--config", suite.configPath)
	suite.gatewayCmd.Dir = suite.projectRoot
	
	err := suite.gatewayCmd.Start()
	suite.Require().NoError(err)
	
	time.Sleep(3 * time.Second)
}

func (suite *LSPValidationTestSuite) stopGatewayServer() {
	if suite.gatewayCmd != nil && suite.gatewayCmd.Process != nil {
		suite.gatewayCmd.Process.Signal(syscall.SIGTERM)
		suite.gatewayCmd.Wait()
		suite.gatewayCmd = nil
	}
}

func (suite *LSPValidationTestSuite) waitForServerReady(ctx context.Context) {
	suite.Eventually(func() bool {
		err := suite.httpClient.HealthCheck(ctx)
		return err == nil
	}, 30*time.Second, 500*time.Millisecond, "Server should become ready")
}

func (suite *LSPValidationTestSuite) TestJSONRPCCompliance() {
	suite.T().Run("JSONRPCRequestCompliance", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		// Clear previous recordings
		suite.httpClient.ClearRecordings()

		// Make various LSP requests
		testFile := fmt.Sprintf("file://%s/main.go", suite.tempDir)
		position := testutils.Position{Line: 8, Character: 5}

		// Execute different LSP methods
		suite.httpClient.Definition(ctx, testFile, position)
		suite.httpClient.References(ctx, testFile, position, true)
		suite.httpClient.Hover(ctx, testFile, position)
		suite.httpClient.DocumentSymbol(ctx, testFile)
		suite.httpClient.WorkspaceSymbol(ctx, "Server")
		suite.httpClient.Completion(ctx, testFile, position)

		// Validate recorded requests
		recordings := suite.httpClient.GetRecordings()
		suite.Greater(len(recordings), 0, "Should have recorded requests")

		for _, recording := range recordings {
			suite.T().Run(fmt.Sprintf("Request_%s", recording.Metadata["lsp_method"].(string)), func(t *testing.T) {
				// Validate HTTP method and headers
				suite.Equal("POST", recording.Method, "Should use POST method")
				suite.Contains(recording.URL, "/jsonrpc", "Should use JSON-RPC endpoint")
				suite.Equal("application/json", recording.Headers["Content-Type"], "Should have correct content type")
				suite.NotEmpty(recording.Headers["User-Agent"], "Should have User-Agent header")
				suite.NotEmpty(recording.Headers["X-Request-ID"], "Should have X-Request-ID header")

				// Validate JSON-RPC request structure
				if bodyMap, ok := recording.Body.(map[string]interface{}); ok {
					suite.Equal("2.0", bodyMap["jsonrpc"], "Should use JSON-RPC 2.0")
					suite.NotEmpty(bodyMap["id"], "Should have request ID")
					suite.NotEmpty(bodyMap["method"], "Should have method")
					suite.NotNil(bodyMap["params"], "Should have params")

					// Validate method is one of the supported LSP methods
					method, ok := bodyMap["method"].(string)
					suite.True(ok, "Method should be a string")
					supportedMethods := []string{
						mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION,
						mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES,
						mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER,
						mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS,
						mcp.LSP_METHOD_WORKSPACE_SYMBOL,
						mcp.LSP_METHOD_TEXT_DOCUMENT_COMPLETION,
					}
					suite.Contains(supportedMethods, method, "Method should be supported")

					// Validate parameters structure
					params, ok := bodyMap["params"].(map[string]interface{})
					suite.True(ok, "Params should be an object")
					
					suite.validateLSPParameters(method, params)
				} else {
					suite.Fail("Request body should be a valid JSON object")
				}
			})
		}
	})
}

func (suite *LSPValidationTestSuite) validateLSPParameters(method string, params map[string]interface{}) {
	switch method {
	case mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION:
		suite.validateTextDocumentPositionParams(params)
	case mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES:
		suite.validateTextDocumentPositionParams(params)
		suite.validateReferencesParams(params)
	case mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER:
		suite.validateTextDocumentPositionParams(params)
	case mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS:
		suite.validateTextDocumentParams(params)
	case mcp.LSP_METHOD_WORKSPACE_SYMBOL:
		suite.validateWorkspaceSymbolParams(params)
	case mcp.LSP_METHOD_TEXT_DOCUMENT_COMPLETION:
		suite.validateTextDocumentPositionParams(params)
	}
}

func (suite *LSPValidationTestSuite) validateTextDocumentPositionParams(params map[string]interface{}) {
	// Validate textDocument parameter
	textDoc, exists := params["textDocument"]
	suite.True(exists, "Should have textDocument parameter")
	
	if textDocMap, ok := textDoc.(map[string]interface{}); ok {
		uri, exists := textDocMap["uri"]
		suite.True(exists, "textDocument should have uri")
		suite.IsType("", uri, "uri should be a string")
		suite.True(strings.HasPrefix(uri.(string), "file://"), "uri should be a file URI")
	}

	// Validate position parameter
	position, exists := params["position"]
	suite.True(exists, "Should have position parameter")
	
	if posMap, ok := position.(map[string]interface{}); ok {
		line, exists := posMap["line"]
		suite.True(exists, "position should have line")
		suite.IsType(float64(0), line, "line should be a number")
		suite.GreaterOrEqual(line.(float64), 0.0, "line should be non-negative")

		character, exists := posMap["character"]
		suite.True(exists, "position should have character")
		suite.IsType(float64(0), character, "character should be a number")
		suite.GreaterOrEqual(character.(float64), 0.0, "character should be non-negative")
	}
}

func (suite *LSPValidationTestSuite) validateTextDocumentParams(params map[string]interface{}) {
	textDoc, exists := params["textDocument"]
	suite.True(exists, "Should have textDocument parameter")
	
	if textDocMap, ok := textDoc.(map[string]interface{}); ok {
		uri, exists := textDocMap["uri"]
		suite.True(exists, "textDocument should have uri")
		suite.IsType("", uri, "uri should be a string")
		suite.True(strings.HasPrefix(uri.(string), "file://"), "uri should be a file URI")
	}
}

func (suite *LSPValidationTestSuite) validateReferencesParams(params map[string]interface{}) {
	context, exists := params["context"]
	suite.True(exists, "References should have context parameter")
	
	if contextMap, ok := context.(map[string]interface{}); ok {
		includeDecl, exists := contextMap["includeDeclaration"]
		suite.True(exists, "context should have includeDeclaration")
		suite.IsType(true, includeDecl, "includeDeclaration should be boolean")
	}
}

func (suite *LSPValidationTestSuite) validateWorkspaceSymbolParams(params map[string]interface{}) {
	query, exists := params["query"]
	suite.True(exists, "WorkspaceSymbol should have query parameter")
	suite.IsType("", query, "query should be a string")
}

func (suite *LSPValidationTestSuite) TestLSPResponseStructureValidation() {
	suite.T().Run("ResponseStructureValidation", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		testFile := fmt.Sprintf("file://%s/main.go", suite.tempDir)
		position := testutils.Position{Line: 15, Character: 8}

		// Test Definition Response
		suite.T().Run("DefinitionResponse", func(t *testing.T) {
			locations, err := suite.httpClient.Definition(ctx, testFile, position)
			if err != nil {
				suite.T().Logf("Definition request failed: %v", err)
				return
			}

			for i, location := range locations {
				suite.NotEmpty(location.URI, fmt.Sprintf("Location %d should have URI", i))
				suite.Contains(location.URI, "file://", fmt.Sprintf("Location %d URI should be file URI", i))
				
				// Validate range structure
				suite.GreaterOrEqual(location.Range.Start.Line, 0, fmt.Sprintf("Location %d start line should be valid", i))
				suite.GreaterOrEqual(location.Range.Start.Character, 0, fmt.Sprintf("Location %d start character should be valid", i))
				suite.GreaterOrEqual(location.Range.End.Line, location.Range.Start.Line, fmt.Sprintf("Location %d end line should be >= start line", i))
				
				if location.Range.End.Line == location.Range.Start.Line {
					suite.GreaterOrEqual(location.Range.End.Character, location.Range.Start.Character, fmt.Sprintf("Location %d end character should be >= start character", i))
				}
			}
		})

		// Test References Response
		suite.T().Run("ReferencesResponse", func(t *testing.T) {
			references, err := suite.httpClient.References(ctx, testFile, position, true)
			if err != nil {
				suite.T().Logf("References request failed: %v", err)
				return
			}

			for i, ref := range references {
				suite.NotEmpty(ref.URI, fmt.Sprintf("Reference %d should have URI", i))
				suite.Contains(ref.URI, "file://", fmt.Sprintf("Reference %d URI should be file URI", i))
				
				// Validate range structure
				suite.GreaterOrEqual(ref.Range.Start.Line, 0, fmt.Sprintf("Reference %d start line should be valid", i))
				suite.GreaterOrEqual(ref.Range.Start.Character, 0, fmt.Sprintf("Reference %d start character should be valid", i))
				suite.GreaterOrEqual(ref.Range.End.Line, ref.Range.Start.Line, fmt.Sprintf("Reference %d end line should be >= start line", i))
			}
		})

		// Test Hover Response
		suite.T().Run("HoverResponse", func(t *testing.T) {
			hoverResult, err := suite.httpClient.Hover(ctx, testFile, position)
			if err != nil {
				suite.T().Logf("Hover request failed: %v", err)
				return
			}

			if hoverResult != nil {
				suite.NotNil(hoverResult.Contents, "Hover should have contents")
				
				// If range is present, validate it
				if hoverResult.Range != nil {
					suite.GreaterOrEqual(hoverResult.Range.Start.Line, 0, "Hover range start line should be valid")
					suite.GreaterOrEqual(hoverResult.Range.Start.Character, 0, "Hover range start character should be valid")
					suite.GreaterOrEqual(hoverResult.Range.End.Line, hoverResult.Range.Start.Line, "Hover range end line should be >= start line")
				}
			}
		})

		// Test Document Symbols Response
		suite.T().Run("DocumentSymbolsResponse", func(t *testing.T) {
			symbols, err := suite.httpClient.DocumentSymbol(ctx, testFile)
			if err != nil {
				suite.T().Logf("DocumentSymbol request failed: %v", err)
				return
			}

			for i, symbol := range symbols {
				suite.NotEmpty(symbol.Name, fmt.Sprintf("Symbol %d should have name", i))
				suite.Greater(symbol.Kind, 0, fmt.Sprintf("Symbol %d should have valid kind", i))
				suite.LessOrEqual(symbol.Kind, 26, fmt.Sprintf("Symbol %d kind should be within LSP range", i))
				
				// Validate ranges
				suite.GreaterOrEqual(symbol.Range.Start.Line, 0, fmt.Sprintf("Symbol %d range start line should be valid", i))
				suite.GreaterOrEqual(symbol.Range.Start.Character, 0, fmt.Sprintf("Symbol %d range start character should be valid", i))
				suite.GreaterOrEqual(symbol.SelectionRange.Start.Line, 0, fmt.Sprintf("Symbol %d selection range start line should be valid", i))
				suite.GreaterOrEqual(symbol.SelectionRange.Start.Character, 0, fmt.Sprintf("Symbol %d selection range start character should be valid", i))
				
				// Validate children if present
				for j, child := range symbol.Children {
					suite.NotEmpty(child.Name, fmt.Sprintf("Symbol %d child %d should have name", i, j))
					suite.Greater(child.Kind, 0, fmt.Sprintf("Symbol %d child %d should have valid kind", i, j))
				}
			}
		})

		// Test Workspace Symbols Response
		suite.T().Run("WorkspaceSymbolsResponse", func(t *testing.T) {
			symbols, err := suite.httpClient.WorkspaceSymbol(ctx, "Server")
			if err != nil {
				suite.T().Logf("WorkspaceSymbol request failed: %v", err)
				return
			}

			for i, symbol := range symbols {
				suite.NotEmpty(symbol.Name, fmt.Sprintf("Workspace symbol %d should have name", i))
				suite.Greater(symbol.Kind, 0, fmt.Sprintf("Workspace symbol %d should have valid kind", i))
				suite.LessOrEqual(symbol.Kind, 26, fmt.Sprintf("Workspace symbol %d kind should be within LSP range", i))
				
				// Validate location
				suite.NotEmpty(symbol.Location.URI, fmt.Sprintf("Workspace symbol %d should have URI", i))
				suite.Contains(symbol.Location.URI, "file://", fmt.Sprintf("Workspace symbol %d URI should be file URI", i))
				
				// Validate location range
				suite.GreaterOrEqual(symbol.Location.Range.Start.Line, 0, fmt.Sprintf("Workspace symbol %d location start line should be valid", i))
				suite.GreaterOrEqual(symbol.Location.Range.Start.Character, 0, fmt.Sprintf("Workspace symbol %d location start character should be valid", i))
			}
		})

		// Test Completion Response
		suite.T().Run("CompletionResponse", func(t *testing.T) {
			completion, err := suite.httpClient.Completion(ctx, testFile, position)
			if err != nil {
				suite.T().Logf("Completion request failed: %v", err)
				return
			}

			if completion != nil {
				for i, item := range completion.Items {
					suite.NotEmpty(item.Label, fmt.Sprintf("Completion item %d should have label", i))
					suite.Greater(item.Kind, 0, fmt.Sprintf("Completion item %d should have valid kind", i))
					suite.LessOrEqual(item.Kind, 25, fmt.Sprintf("Completion item %d kind should be within LSP range", i))
				}
			}
		})
	})
}

func (suite *LSPValidationTestSuite) TestWorkspaceContextValidation() {
	suite.T().Run("WorkspaceContextHandling", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		// Clear recordings and make requests
		suite.httpClient.ClearRecordings()

		testFile := fmt.Sprintf("file://%s/main.go", suite.tempDir)
		position := testutils.Position{Line: 8, Character: 5}

		// Make a request to check workspace context
		suite.httpClient.Definition(ctx, testFile, position)

		recordings := suite.httpClient.GetRecordings()
		suite.Greater(len(recordings), 0, "Should have recorded requests")

		for _, recording := range recordings {
			// Check headers for workspace context
			if workspaceID, exists := recording.Headers["X-Workspace-ID"]; exists {
				suite.NotEmpty(workspaceID, "Workspace ID header should not be empty if present")
			}

			// Check request body for workspace parameters
			if bodyMap, ok := recording.Body.(map[string]interface{}); ok {
				if params, ok := bodyMap["params"].(map[string]interface{}); ok {
					// Check for workspace_id parameter
					if workspaceID, exists := params["workspace_id"]; exists {
						suite.IsType("", workspaceID, "workspace_id should be string")
						suite.NotEmpty(workspaceID.(string), "workspace_id should not be empty")
					}

					// Check for project_path parameter
					if projectPath, exists := params["project_path"]; exists {
						suite.IsType("", projectPath, "project_path should be string")
						suite.NotEmpty(projectPath.(string), "project_path should not be empty")
					}
				}
			}
		}
	})
}

func (suite *LSPValidationTestSuite) TestErrorResponseValidation() {
	suite.T().Run("ErrorResponseStructure", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		// Make requests that should generate errors
		invalidURI := "file:///completely/nonexistent/file.go"
		invalidPosition := testutils.Position{Line: -1, Character: -1}

		// Test various error scenarios
		errorTests := []struct {
			name     string
			testFunc func() error
		}{
			{
				name: "Invalid URI Definition",
				testFunc: func() error {
					_, err := suite.httpClient.Definition(ctx, invalidURI, invalidPosition)
					return err
				},
			},
			{
				name: "Invalid URI References",
				testFunc: func() error {
					_, err := suite.httpClient.References(ctx, invalidURI, invalidPosition, true)
					return err
				},
			},
			{
				name: "Invalid URI Hover",
				testFunc: func() error {
					_, err := suite.httpClient.Hover(ctx, invalidURI, invalidPosition)
					return err
				},
			},
			{
				name: "Invalid URI DocumentSymbol",
				testFunc: func() error {
					_, err := suite.httpClient.DocumentSymbol(ctx, invalidURI)
					return err
				},
			},
			{
				name: "Invalid URI Completion",
				testFunc: func() error {
					_, err := suite.httpClient.Completion(ctx, invalidURI, invalidPosition)
					return err
				},
			},
		}

		for _, test := range errorTests {
			suite.T().Run(test.name, func(t *testing.T) {
				err := test.testFunc()
				if err != nil {
					// Validate error message structure
					suite.NotEmpty(err.Error(), "Error message should not be empty")
					
					// Check if error contains expected patterns
					errorMsg := err.Error()
					if strings.Contains(errorMsg, "LSP error") {
						// Extract error code if present
						if strings.Contains(errorMsg, "LSP error ") {
							suite.T().Logf("LSP error received: %s", errorMsg)
						}
					}
				}
			})
		}
	})
}

func (suite *LSPValidationTestSuite) TestResponseTimingValidation() {
	suite.T().Run("ResponseTiming", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		testFile := fmt.Sprintf("file://%s/main.go", suite.tempDir)
		position := testutils.Position{Line: 8, Character: 5}

		// Clear metrics before test
		suite.httpClient.ClearMetrics()

		// Make several requests to collect timing data
		methods := []struct {
			name string
			fn   func() error
		}{
			{
				name: "Definition",
				fn: func() error {
					_, err := suite.httpClient.Definition(ctx, testFile, position)
					return err
				},
			},
			{
				name: "References",
				fn: func() error {
					_, err := suite.httpClient.References(ctx, testFile, position, true)
					return err
				},
			},
			{
				name: "Hover",
				fn: func() error {
					_, err := suite.httpClient.Hover(ctx, testFile, position)
					return err
				},
			},
			{
				name: "DocumentSymbol",
				fn: func() error {
					_, err := suite.httpClient.DocumentSymbol(ctx, testFile)
					return err
				},
			},
			{
				name: "WorkspaceSymbol",
				fn: func() error {
					_, err := suite.httpClient.WorkspaceSymbol(ctx, "Server")
					return err
				},
			},
			{
				name: "Completion",
				fn: func() error {
					_, err := suite.httpClient.Completion(ctx, testFile, position)
					return err
				},
			},
		}

		for _, method := range methods {
			suite.T().Run(method.name, func(t *testing.T) {
				startTime := time.Now()
				err := method.fn()
				duration := time.Since(startTime)

				if err == nil {
					suite.Less(duration, 10*time.Second, "Request should complete within reasonable time")
					suite.T().Logf("%s request took %v", method.name, duration)
				} else {
					suite.T().Logf("%s request failed: %v", method.name, err)
				}
			})
		}

		// Validate overall metrics
		metrics := suite.httpClient.GetMetrics()
		suite.Greater(metrics.TotalRequests, 0, "Should have total requests")
		
		if metrics.SuccessfulReqs > 0 {
			suite.Greater(metrics.AverageLatency, time.Duration(0), "Should have positive average latency")
			suite.LessOrEqual(metrics.MinLatency, metrics.AverageLatency, "Min latency should be <= average")
			suite.GreaterOrEqual(metrics.MaxLatency, metrics.AverageLatency, "Max latency should be >= average")
			
			suite.T().Logf("Timing metrics - Avg: %v, Min: %v, Max: %v", 
				metrics.AverageLatency, metrics.MinLatency, metrics.MaxLatency)
		}
	})
}

func TestLSPValidationTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping LSP validation tests in short mode")
	}
	suite.Run(t, new(LSPValidationTestSuite))
}