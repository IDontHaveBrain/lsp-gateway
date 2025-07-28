package integration_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	
	"lsp-gateway/internal/config"
	"lsp-gateway/mcp"
	"lsp-gateway/tests/e2e/testutils"
	"lsp-gateway/tests/testdata"
)

// JavaScriptMCPIntegrationTestSuite tests comprehensive MCP server integration for JavaScript language server using chalk repository
type JavaScriptMCPIntegrationTestSuite struct {
	suite.Suite
	
	// MCP server components
	mcpServer     *mcp.Server
	mcpConfig     *mcp.ServerConfig
	tempDir       string
	configPath    string
	testTimeout   time.Duration
	
	// Chalk repository management with fixed commit hash
	repoManager   *testutils.GenericRepoManager
	repoDir       string
	jsFiles       []string
	testFiles     []string
	
	// Test infrastructure
	testCtx       *testdata.TestContext
	serverStarted bool
	
	// MCP communication
	mcpReader     *bufio.Reader
	mcpWriter     io.WriteCloser
	readerPipe    *io.PipeReader
	writerPipe    *io.PipeWriter
	
	// Test metrics and results
	testResults   map[string]*MCPTestResult
	messageCount  int
}

type MCPTestResult struct {
	Method       string
	TestCase     string
	Success      bool
	Duration     time.Duration
	Error        error
	ResponseSize int
	MessageID    interface{}
}

// SetupSuite initializes the comprehensive test suite for JavaScript MCP integration testing using chalk repository
func (suite *JavaScriptMCPIntegrationTestSuite) SetupSuite() {
	suite.testTimeout = 10 * time.Minute
	suite.testResults = make(map[string]*MCPTestResult)
	suite.messageCount = 0
	
	var err error
	suite.tempDir, err = os.MkdirTemp("", "js-mcp-comprehensive-integration-*")
	suite.Require().NoError(err, "Failed to create temp directory")
	
	suite.testCtx = testdata.NewTestContext(suite.testTimeout)
	
	// Setup chalk repository manager with fixed commit hash
	suite.setupChalkRepository()
	
	// Create comprehensive MCP server configuration for JavaScript
	suite.createComprehensiveMCPServerConfig()
	
	suite.T().Logf("JavaScript MCP Integration Test Suite initialized with chalk repository (commit: 5dbc1e2)")
	suite.T().Logf("Found %d JavaScript files for MCP testing", len(suite.jsFiles))
}

// SetupTest initializes fresh components for each test
func (suite *JavaScriptMCPIntegrationTestSuite) SetupTest() {
	suite.serverStarted = false
	
	// Create pipes for MCP communication
	var err error
	suite.readerPipe, suite.writerPipe = io.Pipe()
	suite.mcpReader = bufio.NewReader(suite.readerPipe)
	suite.mcpWriter = suite.writerPipe
	
	suite.Require().NoError(err, "Failed to create communication pipes")
}

// TearDownTest cleans up per-test resources
func (suite *JavaScriptMCPIntegrationTestSuite) TearDownTest() {
	suite.stopMCPServer()
	
	if suite.writerPipe != nil {
		suite.writerPipe.Close()
	}
	if suite.readerPipe != nil {
		suite.readerPipe.Close()
	}
}

// TearDownSuite performs final cleanup and reports comprehensive test results
func (suite *JavaScriptMCPIntegrationTestSuite) TearDownSuite() {
	// Report comprehensive test results
	suite.reportComprehensiveMCPTestResults()
	
	if suite.repoManager != nil {
		if err := suite.repoManager.Cleanup(); err != nil {
			suite.T().Logf("Warning: Failed to cleanup chalk repository: %v", err)
		}
	}
	
	if suite.tempDir != "" {
		if err := os.RemoveAll(suite.tempDir); err != nil {
			suite.T().Logf("Warning: Failed to remove temp directory: %v", err)
		}
	}
	
	if suite.testCtx != nil {
		suite.testCtx.Cleanup()
	}
}

// TestMCPServerInitialization tests MCP server initialization with JavaScript configuration
func (suite *JavaScriptMCPIntegrationTestSuite) TestMCPServerInitialization() {
	if testing.Short() {
		suite.T().Skip("Skipping integration tests in short mode")
	}
	
	// Test MCP server creation
	mcpServer, err := suite.createMCPServer()
	suite.NoError(err, "MCP server creation should succeed")
	suite.NotNil(mcpServer, "MCP server should not be nil")
	
	// Verify server configuration
	suite.Equal("lsp-gateway-mcp", mcpServer.Config.Name, "Server name should be correct")
	suite.Equal("1.0.0", mcpServer.Config.Version, "Server version should be correct")
	suite.NotNil(mcpServer.ToolHandler, "Tool handler should be initialized")
	
	// Test server initialization state
	suite.verifyServerInitializationState(mcpServer)
}

// TestJavaScriptLanguageServerIntegration tests JavaScript language server integration via MCP
func (suite *JavaScriptMCPIntegrationTestSuite) TestJavaScriptLanguageServerIntegration() {
	if testing.Short() {
		suite.T().Skip("Skipping integration tests in short mode")
	}
	
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	// Test MCP initialization protocol
	suite.testMCPInitialization()
	
	// Test JavaScript language server communication
	suite.testJavaScriptLSPCommunication()
}

// TestRepositorySetupAndManagement tests repository setup with JavaScript files
func (suite *JavaScriptMCPIntegrationTestSuite) TestRepositorySetupAndManagement() {
	suite.Require().NotNil(suite.repoManager, "Repository manager should be initialized")
	
	// Verify repository setup
	workspaceDir := suite.repoManager.GetWorkspaceDir()
	suite.NotEmpty(workspaceDir, "Workspace directory should be set")
	
	// Verify JavaScript files are discovered
	suite.Greater(len(suite.jsFiles), 0, "Should have discovered JavaScript files")
	
	// Test file access
	for _, jsFile := range suite.jsFiles[:min(3, len(suite.jsFiles))] { // Test first 3 files
		fullPath := filepath.Join(workspaceDir, jsFile)
		info, err := os.Stat(fullPath)
		suite.NoError(err, "JavaScript file should exist: %s", jsFile)
		suite.False(info.IsDir(), "Should be a file, not directory: %s", jsFile)
	}
}

// TestMCPServerLifecycleManagement tests complete MCP server lifecycle
func (suite *JavaScriptMCPIntegrationTestSuite) TestMCPServerLifecycleManagement() {
	if testing.Short() {
		suite.T().Skip("Skipping integration tests in short mode")
	}
	
	// Test server startup
	suite.startMCPServer()
	suite.True(suite.serverStarted, "Server should be marked as started")
	suite.NotNil(suite.mcpServer, "MCP server should be available")
	
	// Test server responsiveness
	suite.verifyMCPServerResponsiveness()
	
	// Test server shutdown
	suite.stopMCPServer()
	suite.False(suite.serverStarted, "Server should be marked as stopped")
}

// TestSCIPIntegration tests SCIP integration if available
func (suite *JavaScriptMCPIntegrationTestSuite) TestSCIPIntegration() {
	if testing.Short() {
		suite.T().Skip("Skipping integration tests in short mode")
	}
	
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	// Test SCIP-enhanced operations if JavaScript files are available
	if len(suite.jsFiles) > 0 {
		testFile := suite.jsFiles[0]
		fileURI := suite.getFileURI(testFile)
		
		// Test document symbol request (should benefit from SCIP if available)
		symbolRequest := testutils.MCPMessage{
			Jsonrpc: "2.0",
			ID:      "scip-test",
			Method:  "textDocument/documentSymbol",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": fileURI,
				},
			},
		}
		
		response, err := testutils.SendMCPStdioMessage(suite.mcpWriter, suite.mcpReader, symbolRequest)
		suite.NoError(err, "Document symbol request should not fail")
		suite.NotNil(response, "SCIP-enhanced response should not be nil")
		
		suite.T().Logf("SCIP integration test completed for file: %s", testFile)
	}
}

// Helper methods for comprehensive MCP server integration

func (suite *JavaScriptMCPIntegrationTestSuite) setupChalkRepository() {
	// Create custom repository manager for chalk with fixed commit hash
	chalkConfig := testutils.NewCustomLanguageConfig("javascript", "https://github.com/chalk/chalk.git").
		WithTestPaths("source", "test", ".").
		WithFilePatterns("*.js", "*.ts", "*.mjs").
		WithRootMarkers("package.json", "tsconfig.json").
		WithExcludePaths("node_modules", ".git", "dist", "build", "coverage").
		WithRepoSubDir("chalk").
		WithCustomVariable("lsp_server", "typescript-language-server").
		WithCustomVariable("transport", "stdio").
		WithCustomVariable("commit_hash", "5dbc1e2").  // Fixed commit hash for v5.4.1
		Build()
	
	// Create custom repo config with commit hash and MCP settings
	repoConfig := testutils.GenericRepoConfig{
		CloneTimeout:   300 * time.Second,
		EnableLogging:  true,
		ForceClean:     false,
		PreserveGitDir: true,  // Keep .git for commit checkout
		CommitHash:     "5dbc1e2",  // Checkout specific commit
	}
	
	suite.repoManager = testutils.NewCustomRepositoryManager(chalkConfig, repoConfig)
	
	var err error
	suite.repoDir, err = suite.repoManager.SetupRepository()
	suite.Require().NoError(err, "Failed to setup chalk repository for MCP integration testing")
	
	// Discover JavaScript files from chalk repository
	testFiles, err := suite.repoManager.GetTestFiles()
	suite.Require().NoError(err, "Failed to get test files from chalk repository")
	
	// Filter and categorize JavaScript/TypeScript files
	for _, file := range testFiles {
		ext := filepath.Ext(file)
		if ext == ".js" || ext == ".mjs" {
			suite.jsFiles = append(suite.jsFiles, file)
		}
		if ext == ".js" || ext == ".ts" || ext == ".mjs" || ext == ".jsx" || ext == ".tsx" {
			suite.testFiles = append(suite.testFiles, file)
		}
	}
	
	suite.Require().Greater(len(suite.jsFiles), 0, "No JavaScript files found in chalk repository")
	suite.T().Logf("Discovered %d JavaScript files and %d total test files in chalk repository for MCP integration testing", 
		len(suite.jsFiles), len(suite.testFiles))
}

func (suite *JavaScriptMCPIntegrationTestSuite) createComprehensiveMCPServerConfig() {
	// Create comprehensive language configuration for JavaScript MCP
	options := testutils.DefaultLanguageConfigOptions("javascript")
	options.ConfigType = "mcp"
	options.CustomVariables["REPOSITORY"] = "chalk"
	options.CustomVariables["MCP_MODE"] = "stdio"
	options.CustomVariables["COMMIT_HASH"] = "5dbc1e2"
	options.CustomVariables["LSP_TIMEOUT"] = "60"
	options.CustomVariables["MCP_INTEGRATION_TEST"] = "true"
	options.CustomVariables["NODE_PATH"] = "/usr/local/lib/node_modules"
	options.CustomVariables["TS_NODE_PROJECT"] = "tsconfig.json"
	
	var err error
	suite.configPath, _, err = testutils.CreateLanguageConfig(suite.repoManager, options)
	suite.Require().NoError(err, "Failed to create comprehensive MCP language config")
	
	// Load configuration using internal config system
	configManager := config.NewManager()
	loadedConfig, err := configManager.LoadConfig(suite.configPath)
	suite.Require().NoError(err, "Failed to load comprehensive MCP configuration")
	
	// Create comprehensive MCP server configuration
	suite.mcpConfig = &mcp.ServerConfig{
		Name:            "lsp-gateway-js-mcp-integration",
		Version:         "1.0.0",
		LSPGatewayURL:   "http://localhost:8080/jsonrpc",
		WorkspaceDir:    suite.repoManager.GetWorkspaceDir(),
		ConfigPath:      suite.configPath,
		LoadedConfig:    loadedConfig,
	}
}

func (suite *JavaScriptMCPIntegrationTestSuite) createMCPServer() (*mcp.Server, error) {
	// Create LSP Gateway client for MCP server
	client, err := mcp.NewLSPGatewayClient(suite.mcpConfig.LSPGatewayURL, 30*time.Second)
	if err != nil {
		return nil, err
	}
	
	// Create MCP server with configuration
	server, err := mcp.NewServer(suite.mcpConfig, client)
	if err != nil {
		return nil, err
	}
	
	// Configure IO for testing
	server.SetIO(suite.readerPipe, suite.writerPipe)
	
	return server, nil
}

func (suite *JavaScriptMCPIntegrationTestSuite) startMCPServer() {
	if suite.serverStarted {
		return
	}
	
	var err error
	suite.mcpServer, err = suite.createMCPServer()
	suite.Require().NoError(err, "Failed to create MCP server")
	
	// Start server in goroutine
	go func() {
		if startErr := suite.mcpServer.Start(); startErr != nil {
			suite.T().Logf("MCP server error: %v", startErr)
		}
	}()
	
	// Wait for server initialization
	time.Sleep(2 * time.Second)
	suite.serverStarted = true
	
	suite.T().Logf("MCP server started for JavaScript integration testing")
}

func (suite *JavaScriptMCPIntegrationTestSuite) stopMCPServer() {
	if !suite.serverStarted || suite.mcpServer == nil {
		return
	}
	
	// Stop server
	suite.mcpServer.Stop()
	suite.mcpServer = nil
	suite.serverStarted = false
	
	suite.T().Logf("MCP server stopped")
}

func (suite *JavaScriptMCPIntegrationTestSuite) verifyServerInitializationState(server *mcp.Server) {
	// Verify server components are properly initialized
	suite.NotNil(server.Config, "Server config should be initialized")
	suite.NotNil(server.ToolHandler, "Tool handler should be initialized")
	suite.Equal(server.Config.WorkspaceDir, suite.repoManager.GetWorkspaceDir(), "Workspace directory should match")
}

func (suite *JavaScriptMCPIntegrationTestSuite) testMCPInitialization() {
	// Send MCP initialize request
	initRequest := testutils.MCPMessage{
		Jsonrpc: "2.0",
		ID:      "init-test",
		Method:  "initialize",
		Params: map[string]interface{}{
			"capabilities": map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "lspg-integration-test",
				"version": "1.0.0",
			},
		},
	}
	
	response, err := testutils.SendMCPStdioMessage(suite.mcpWriter, suite.mcpReader, initRequest)
	suite.NoError(err, "MCP initialization should succeed")
	suite.NotNil(response, "MCP initialization response should not be nil")
	suite.Equal("2.0", response.Jsonrpc, "Response should be JSON-RPC 2.0")
	
	suite.T().Logf("MCP initialization test completed successfully")
}

func (suite *JavaScriptMCPIntegrationTestSuite) testJavaScriptLSPCommunication() {
	if len(suite.jsFiles) == 0 {
		suite.T().Skip("No JavaScript files available for LSP communication test")
		return
	}
	
	testFile := suite.jsFiles[0]
	fileURI := suite.getFileURI(testFile)
	
	// Test hover request
	hoverRequest := testutils.MCPMessage{
		Jsonrpc: "2.0",
		ID:      "hover-test",
		Method:  "textDocument/hover",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fileURI,
			},
			"position": map[string]interface{}{
				"line":      0,
				"character": 0,
			},
		},
	}
	
	response, err := testutils.SendMCPStdioMessage(suite.mcpWriter, suite.mcpReader, hoverRequest)
	suite.NoError(err, "JavaScript LSP hover request should not fail")
	suite.NotNil(response, "JavaScript LSP response should not be nil")
	
	suite.T().Logf("JavaScript LSP communication test completed for file: %s", testFile)
}

func (suite *JavaScriptMCPIntegrationTestSuite) verifyMCPServerResponsiveness() {
	// Simple ping test to verify server responsiveness
	pingRequest := testutils.MCPMessage{
		Jsonrpc: "2.0",
		ID:      "ping-test",
		Method:  "workspace/symbol",
		Params: map[string]interface{}{
			"query": "",
		},
	}
	
	response, err := testutils.SendMCPStdioMessage(suite.mcpWriter, suite.mcpReader, pingRequest)
	suite.NoError(err, "MCP server responsiveness check should pass")
	suite.NotNil(response, "MCP responsiveness response should not be nil")
	
	suite.T().Logf("MCP server responsiveness verified")
}

// TestJavaScriptMCPComprehensiveLSPMethods tests all 6 LSP methods via MCP with chalk repository
func (suite *JavaScriptMCPIntegrationTestSuite) TestJavaScriptMCPComprehensiveLSPMethods() {
	if testing.Short() {
		suite.T().Skip("Skipping comprehensive integration tests in short mode")
	}
	
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	// Initialize MCP protocol
	suite.testMCPInitialization()
	
	if len(suite.jsFiles) == 0 {
		suite.T().Skip("No JavaScript files available for comprehensive LSP methods test")
		return
	}
	
	testFile := suite.jsFiles[0]
	fileURI := suite.getFileURI(testFile)
	
	suite.T().Logf("Testing all 6 LSP methods via MCP with chalk file: %s", testFile)
	
	// Test all 6 supported LSP methods via MCP
	suite.testMCPDefinition(fileURI, testFile)
	suite.testMCPReferences(fileURI, testFile)
	suite.testMCPHover(fileURI, testFile)
	suite.testMCPDocumentSymbol(fileURI, testFile)
	suite.testMCPWorkspaceSymbol()
	suite.testMCPCompletion(fileURI, testFile)
}

// TestJavaScriptMCPPerformanceAndReliability tests MCP performance and reliability with multiple requests
func (suite *JavaScriptMCPIntegrationTestSuite) TestJavaScriptMCPPerformanceAndReliability() {
	if testing.Short() {
		suite.T().Skip("Skipping performance integration tests in short mode")
	}
	
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	// Initialize MCP protocol
	suite.testMCPInitialization()
	
	if len(suite.jsFiles) < 2 {
		suite.T().Skip("Need at least 2 JavaScript files for performance testing")
		return
	}
	
	// Test multiple concurrent requests
	for i := 0; i < 3; i++ {
		testFile := suite.jsFiles[i%len(suite.jsFiles)]
		fileURI := suite.getFileURI(testFile)
		
		testName := fmt.Sprintf("performance-test-%d", i)
		start := time.Now()
		
		// Test workspace symbol (should be fast with SCIP)
		symbolRequest := testutils.MCPMessage{
			Jsonrpc: "2.0",
			ID:      fmt.Sprintf("perf-%d", i),
			Method:  "workspace/symbol",
			Params: map[string]interface{}{
				"query": "chalk",
			},
		}
		
		response, err := testutils.SendMCPStdioMessage(suite.mcpWriter, suite.mcpReader, symbolRequest)
		duration := time.Since(start)
		
		result := &MCPTestResult{
			Method:   "workspace/symbol",
			TestCase: testName,
			Success:  err == nil,
			Duration: duration,
			Error:    err,
			MessageID: symbolRequest.ID,
		}
		
		if response != nil {
			result.ResponseSize = len(fmt.Sprintf("%v", response))
		}
		
		suite.testResults[testName] = result
		suite.messageCount++
		
		suite.T().Logf("Performance test %d completed in %v", i, duration)
	}
}

// Individual MCP LSP method test functions

func (suite *JavaScriptMCPIntegrationTestSuite) testMCPDefinition(fileURI, testFile string) {
	start := time.Now()
	definitionRequest := testutils.MCPMessage{
		Jsonrpc: "2.0",
		ID:      "definition-test",
		Method:  "textDocument/definition",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fileURI,
			},
			"position": map[string]interface{}{
				"line":      0,
				"character": 0,
			},
		},
	}
	
	response, err := testutils.SendMCPStdioMessage(suite.mcpWriter, suite.mcpReader, definitionRequest)
	suite.recordMCPTestResult("textDocument/definition", "definition-chalk", start, err, response, definitionRequest.ID)
	
	suite.NoError(err, "MCP definition request should not fail for chalk file")
	suite.NotNil(response, "MCP definition response should not be nil")
}

func (suite *JavaScriptMCPIntegrationTestSuite) testMCPReferences(fileURI, testFile string) {
	start := time.Now()
	referencesRequest := testutils.MCPMessage{
		Jsonrpc: "2.0",
		ID:      "references-test",
		Method:  "textDocument/references",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fileURI,
			},
			"position": map[string]interface{}{
				"line":      0,
				"character": 0,
			},
			"context": map[string]interface{}{
				"includeDeclaration": true,
			},
		},
	}
	
	response, err := testutils.SendMCPStdioMessage(suite.mcpWriter, suite.mcpReader, referencesRequest)
	suite.recordMCPTestResult("textDocument/references", "references-chalk", start, err, response, referencesRequest.ID)
	
	suite.NoError(err, "MCP references request should not fail for chalk file")
	suite.NotNil(response, "MCP references response should not be nil")
}

func (suite *JavaScriptMCPIntegrationTestSuite) testMCPHover(fileURI, testFile string) {
	start := time.Now()
	hoverRequest := testutils.MCPMessage{
		Jsonrpc: "2.0",
		ID:      "hover-test",
		Method:  "textDocument/hover",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fileURI,
			},
			"position": map[string]interface{}{
				"line":      0,
				"character": 0,
			},
		},
	}
	
	response, err := testutils.SendMCPStdioMessage(suite.mcpWriter, suite.mcpReader, hoverRequest)
	suite.recordMCPTestResult("textDocument/hover", "hover-chalk", start, err, response, hoverRequest.ID)
	
	suite.NoError(err, "MCP hover request should not fail for chalk file")
	suite.NotNil(response, "MCP hover response should not be nil")
}

func (suite *JavaScriptMCPIntegrationTestSuite) testMCPDocumentSymbol(fileURI, testFile string) {
	start := time.Now()
	docSymbolRequest := testutils.MCPMessage{
		Jsonrpc: "2.0",
		ID:      "document-symbol-test",
		Method:  "textDocument/documentSymbol",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fileURI,
			},
		},
	}
	
	response, err := testutils.SendMCPStdioMessage(suite.mcpWriter, suite.mcpReader, docSymbolRequest)
	suite.recordMCPTestResult("textDocument/documentSymbol", "document-symbol-chalk", start, err, response, docSymbolRequest.ID)
	
	suite.NoError(err, "MCP document symbol request should not fail for chalk file")
	suite.NotNil(response, "MCP document symbol response should not be nil")
}

func (suite *JavaScriptMCPIntegrationTestSuite) testMCPWorkspaceSymbol() {
	start := time.Now()
	wsSymbolRequest := testutils.MCPMessage{
		Jsonrpc: "2.0",
		ID:      "workspace-symbol-test",
		Method:  "workspace/symbol",
		Params: map[string]interface{}{
			"query": "chalk",
		},
	}
	
	response, err := testutils.SendMCPStdioMessage(suite.mcpWriter, suite.mcpReader, wsSymbolRequest)
	suite.recordMCPTestResult("workspace/symbol", "workspace-symbol-chalk", start, err, response, wsSymbolRequest.ID)
	
	suite.NoError(err, "MCP workspace symbol request should not fail")
	suite.NotNil(response, "MCP workspace symbol response should not be nil")
}

func (suite *JavaScriptMCPIntegrationTestSuite) testMCPCompletion(fileURI, testFile string) {
	start := time.Now()
	completionRequest := testutils.MCPMessage{
		Jsonrpc: "2.0",
		ID:      "completion-test",
		Method:  "textDocument/completion",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fileURI,
			},
			"position": map[string]interface{}{
				"line":      0,
				"character": 0,
			},
		},
	}
	
	response, err := testutils.SendMCPStdioMessage(suite.mcpWriter, suite.mcpReader, completionRequest)
	suite.recordMCPTestResult("textDocument/completion", "completion-chalk", start, err, response, completionRequest.ID)
	
	suite.NoError(err, "MCP completion request should not fail for chalk file")
	suite.NotNil(response, "MCP completion response should not be nil")
}

// Helper methods for comprehensive testing

func (suite *JavaScriptMCPIntegrationTestSuite) recordMCPTestResult(method, testCase string, start time.Time, err error, response interface{}, messageID interface{}) {
	result := &MCPTestResult{
		Method:    method,
		TestCase:  testCase,
		Success:   err == nil,
		Duration:  time.Since(start),
		Error:     err,
		MessageID: messageID,
	}
	
	if response != nil {
		result.ResponseSize = len(fmt.Sprintf("%v", response))
	}
	
	suite.testResults[testCase] = result
	suite.messageCount++
}

func (suite *JavaScriptMCPIntegrationTestSuite) reportComprehensiveMCPTestResults() {
	suite.T().Logf("=== Comprehensive JavaScript MCP Integration Test Results ===")
	
	if len(suite.testResults) == 0 {
		suite.T().Logf("No test results to report")
		return
	}
	
	methodCounts := make(map[string]int)
	methodSuccesses := make(map[string]int)
	methodDurations := make(map[string]time.Duration)
	totalDuration := time.Duration(0)
	
	for testName, result := range suite.testResults {
		methodCounts[result.Method]++
		methodDurations[result.Method] += result.Duration
		if result.Success {
			methodSuccesses[result.Method]++
		}
		totalDuration += result.Duration
		
		status := "PASS"
		if !result.Success {
			status = "FAIL"
		}
		
		suite.T().Logf("[%s] %s - %s (%v, %d bytes)", status, result.Method, testName, result.Duration, result.ResponseSize)
		if !result.Success && result.Error != nil {
			suite.T().Logf("  Error: %v", result.Error)
		}
	}
	
	suite.T().Logf("=== MCP Method Summary ===")
	for method := range methodCounts {
		successRate := float64(methodSuccesses[method]) / float64(methodCounts[method]) * 100
		avgDuration := methodDurations[method] / time.Duration(methodCounts[method])
		suite.T().Logf("%s: %d/%d (%.1f%%) - avg: %v", method, methodSuccesses[method], methodCounts[method], successRate, avgDuration)
	}
	
	totalTests := len(suite.testResults)
	totalSuccesses := 0
	for _, count := range methodSuccesses {
		totalSuccesses += count
	}
	
	overallSuccessRate := float64(totalSuccesses) / float64(totalTests) * 100
	suite.T().Logf("=== Overall MCP Integration Results ===")
	suite.T().Logf("Total tests: %d", totalTests)
	suite.T().Logf("Successful: %d", totalSuccesses)
	suite.T().Logf("Messages sent: %d", suite.messageCount)
	suite.T().Logf("Success rate: %.1f%%", overallSuccessRate)
	suite.T().Logf("Total duration: %v", totalDuration)
	suite.T().Logf("Average duration: %v", totalDuration/time.Duration(totalTests))
	
	// Verify MCP integration test quality requirements
	suite.GreaterOrEqual(overallSuccessRate, 75.0, "MCP integration success rate should be at least 75%")
	suite.LessOrEqual(totalDuration.Minutes(), 10.0, "Total MCP integration test duration should be under 10 minutes")
}

func (suite *JavaScriptMCPIntegrationTestSuite) getFileURI(filePath string) string {
	workspaceDir := suite.repoManager.GetWorkspaceDir()
	return "file://" + filepath.Join(workspaceDir, filePath)
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestJavaScriptMCPIntegration runs the comprehensive JavaScript MCP integration test suite
func TestJavaScriptMCPIntegration(t *testing.T) {
	suite.Run(t, new(JavaScriptMCPIntegrationTestSuite))
}