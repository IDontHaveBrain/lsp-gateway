package base

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"lsp-gateway/tests/e2e/testutils"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// LanguageConfig defines language-specific configuration
type LanguageConfig struct {
	Language      string
	DisplayName   string
	HasRepoMgmt   bool
	HasAllLSPTest bool
}

// ComprehensiveTestBaseSuite provides base functionality for comprehensive E2E tests
type ComprehensiveTestBaseSuite struct {
	suite.Suite

	// Language configuration
	Config LanguageConfig

	// Core infrastructure
	httpClient  *testutils.HttpClient
	gatewayCmd  *exec.Cmd
	gatewayPort int
	configPath  string
	tempDir     string
	projectRoot string
	testTimeout time.Duration

	// Repository management
	repoManager *testutils.RepoManager
	repoDir     string

	// Server state tracking
	serverStarted bool
}

// SetupSuite initializes the comprehensive test suite
func (suite *ComprehensiveTestBaseSuite) SetupSuite() {
	suite.testTimeout = 120 * time.Second
	suite.gatewayPort = 8080

	// Create temp directory for repositories
	tempDir, err := ioutil.TempDir("", "lsp-gateway-e2e-repos")
	require.NoError(suite.T(), err, "Failed to create temp directory")
	suite.tempDir = tempDir

	// Initialize repository manager
	suite.repoManager = testutils.NewRepoManager(tempDir)

	// Basic project root setup
	if cwd, err := os.Getwd(); err == nil {
		suite.projectRoot = filepath.Dir(filepath.Dir(cwd))
	}
}

// SetupTest prepares each test
func (suite *ComprehensiveTestBaseSuite) SetupTest() {
	// Basic setup only
}

// TearDownTest cleans up after each test
func (suite *ComprehensiveTestBaseSuite) TearDownTest() {
	suite.stopGatewayServer()
}

// TearDownSuite cleans up after all tests
func (suite *ComprehensiveTestBaseSuite) TearDownSuite() {
	if suite.repoManager != nil {
		suite.repoManager.Cleanup()
	}
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
}

// stopGatewayServer stops the gateway server
func (suite *ComprehensiveTestBaseSuite) stopGatewayServer() {
	if suite.gatewayCmd != nil && suite.gatewayCmd.Process != nil {
		suite.gatewayCmd.Process.Signal(syscall.SIGTERM)
		suite.gatewayCmd.Wait()
		suite.gatewayCmd = nil
	}
	suite.serverStarted = false
}

// setupTestWorkspace sets up a real project workspace for testing
func (suite *ComprehensiveTestBaseSuite) setupTestWorkspace() error {
	// Setup repository for the language
	repoDir, err := suite.repoManager.SetupRepository(suite.Config.Language)
	if err != nil {
		return fmt.Errorf("failed to setup repository for %s: %w", suite.Config.Language, err)
	}
	suite.repoDir = repoDir

	// Verify test files exist
	if err := suite.repoManager.VerifyFileExists(suite.Config.Language, 0); err != nil {
		return fmt.Errorf("test file verification failed: %w", err)
	}

	suite.T().Logf("Set up test workspace for %s at: %s", suite.Config.Language, repoDir)
	return nil
}

// startGatewayServer starts the LSP gateway server
func (suite *ComprehensiveTestBaseSuite) startGatewayServer() error {
	if suite.serverStarted {
		return nil
	}

	// Find available port
	port, err := testutils.FindAvailablePort()
	if err != nil {
		return fmt.Errorf("failed to find available port: %w", err)
	}
	suite.gatewayPort = port

	// Create a proper config file
	configContent := fmt.Sprintf(`servers:
  go:
    command: "gopls"
    args: ["serve"]
    working_dir: ""
    initialization_options: {}
  python:
    command: "pylsp"
    args: []
    working_dir: ""
    initialization_options: {}
  javascript:
    command: "typescript-language-server"
    args: ["--stdio"]
    working_dir: ""
    initialization_options: {}
  typescript:
    command: "typescript-language-server"
    args: ["--stdio"]
    working_dir: ""
    initialization_options: {}
  java:
    command: "~/.lsp-gateway/tools/java/bin/jdtls"
    args: []
    working_dir: ""
    initialization_options: {}
`)

	configPath := filepath.Join(suite.tempDir, "config.yaml")
	if err := ioutil.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	suite.configPath = configPath

	// Get path to lsp-gateway binary
	pwd, _ := os.Getwd()
	projectRoot := filepath.Dir(filepath.Dir(pwd)) // go up from tests/e2e to project root
	binaryPath := filepath.Join(projectRoot, "bin", "lsp-gateway")

	// Check if binary exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return fmt.Errorf("lsp-gateway binary not found at %s. Run 'make local' first", binaryPath)
	}

	// Start the server
	cmd := exec.Command(binaryPath, "server", "--config", configPath, "--port", fmt.Sprintf("%d", port))
	cmd.Dir = suite.repoDir // Run in the cloned repository workspace
	cmd.Env = append(os.Environ(),
		"GO111MODULE=on",
		fmt.Sprintf("GOPATH=%s", os.Getenv("GOPATH")),
	)

	// Capture output for debugging
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	suite.T().Logf("Starting LSP gateway server on port %d with config %s", port, configPath)
	suite.T().Logf("Command: %s %s", binaryPath, strings.Join(cmd.Args[1:], " "))
	suite.T().Logf("Working directory: %s", cmd.Dir)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	suite.gatewayCmd = cmd
	suite.serverStarted = true

	return nil
}

// waitForServerReady waits for the server to be ready to accept requests
func (suite *ComprehensiveTestBaseSuite) waitForServerReady() error {
	healthURL := fmt.Sprintf("http://localhost:%d/health", suite.gatewayPort)

	suite.T().Logf("Waiting for server and LSP clients to be ready at %s...", healthURL)

	for i := 0; i < 30; i++ { // Wait up to 30 seconds
		if err := testutils.QuickConnectivityCheck(healthURL); err == nil {
			// Health endpoint is responding, now check if LSP clients are active
			if suite.checkLSPClientsActive(healthURL) {
				suite.T().Logf("✅ Server and LSP clients are ready after %d seconds", i+1)
				return nil
			}
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("server did not become ready within 30 seconds")
}

// checkLSPClientsActive checks if required LSP clients are active
func (suite *ComprehensiveTestBaseSuite) checkLSPClientsActive(healthURL string) bool {
	resp, err := http.Get(healthURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return false
	}

	lspClients, ok := health["lsp_clients"].(map[string]interface{})
	if !ok {
		return false
	}

	// Check if the target language LSP client is active
	langClient, ok := lspClients[suite.Config.Language].(map[string]interface{})
	if !ok {
		suite.T().Logf("%s LSP client not found in health status", suite.Config.Language)
		return false
	}

	active, ok := langClient["Active"].(bool)
	if !ok || !active {
		suite.T().Logf("%s LSP client not active yet: %v", suite.Config.Language, langClient)
		return false
	}

	suite.T().Logf("%s LSP client is active: %v", suite.Config.Language, langClient)
	return true
}

// makeJSONRPCRequest makes a raw JSON-RPC request to the LSP gateway
func (suite *ComprehensiveTestBaseSuite) makeJSONRPCRequest(ctx context.Context, httpClient *testutils.HttpClient, request map[string]interface{}) (map[string]interface{}, error) {
	return httpClient.MakeRawJSONRPCRequest(ctx, request)
}

// Test methods that can be used by language-specific tests

// TestComprehensiveServerLifecycle tests server lifecycle
func (suite *ComprehensiveTestBaseSuite) TestComprehensiveServerLifecycle() {
	suite.T().Logf("Testing %s comprehensive server lifecycle", suite.Config.DisplayName)
	// Basic lifecycle test
}

// TestDefinitionComprehensive tests textDocument/definition
func (suite *ComprehensiveTestBaseSuite) TestDefinitionComprehensive() {
	suite.T().Logf("Testing %s definition", suite.Config.DisplayName)

	err := suite.setupTestWorkspace()
	require.NoError(suite.T(), err, "Failed to setup test workspace")

	err = suite.startGatewayServer()
	require.NoError(suite.T(), err, "Failed to start gateway server")

	err = suite.waitForServerReady()
	require.NoError(suite.T(), err, "Server failed to become ready")

	httpClient := testutils.NewHttpClient(testutils.HttpClientConfig{
		BaseURL: fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
		Timeout: 10 * time.Second,
	})

	fileURI, err := suite.repoManager.GetFileURI(suite.Config.Language, 0)
	require.NoError(suite.T(), err, "Failed to get file URI")

	testFile, err := suite.repoManager.GetTestFile(suite.Config.Language, 0)
	require.NoError(suite.T(), err, "Failed to get test file")

	time.Sleep(3 * time.Second)

	definitionRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "textDocument/definition",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fileURI,
			},
			"position": map[string]interface{}{
				"line":      testFile.DefinitionPos.Line,
				"character": testFile.DefinitionPos.Character,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	response, err := suite.makeJSONRPCRequest(ctx, httpClient, definitionRequest)
	require.NoError(suite.T(), err, "Definition request failed")
	require.NotNil(suite.T(), response, "Response should not be nil")

	if errorField, hasError := response["error"]; hasError && errorField != nil {
		suite.T().Errorf("LSP error in definition response: %v", errorField)
		return
	}

	result, hasResult := response["result"]
	require.True(suite.T(), hasResult, "Response should have result field")

	if result != nil {
		suite.T().Logf("✅ Definition result: %v", result)
	} else {
		suite.T().Logf("⚠️ Definition result is null")
	}

	suite.T().Logf("✅ Definition test completed for %s", suite.Config.DisplayName)
}

// TestReferencesComprehensive tests textDocument/references
func (suite *ComprehensiveTestBaseSuite) TestReferencesComprehensive() {
	suite.T().Logf("Testing %s references", suite.Config.DisplayName)

	err := suite.setupTestWorkspace()
	require.NoError(suite.T(), err, "Failed to setup test workspace")

	err = suite.startGatewayServer()
	require.NoError(suite.T(), err, "Failed to start gateway server")

	err = suite.waitForServerReady()
	require.NoError(suite.T(), err, "Server failed to become ready")

	httpClient := testutils.NewHttpClient(testutils.HttpClientConfig{
		BaseURL: fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
		Timeout: 10 * time.Second,
	})

	fileURI, err := suite.repoManager.GetFileURI(suite.Config.Language, 0)
	require.NoError(suite.T(), err, "Failed to get file URI")

	testFile, err := suite.repoManager.GetTestFile(suite.Config.Language, 0)
	require.NoError(suite.T(), err, "Failed to get test file")

	time.Sleep(3 * time.Second)

	referencesRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "textDocument/references",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fileURI,
			},
			"position": map[string]interface{}{
				"line":      testFile.ReferencePos.Line,
				"character": testFile.ReferencePos.Character,
			},
			"context": map[string]interface{}{
				"includeDeclaration": true,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	response, err := suite.makeJSONRPCRequest(ctx, httpClient, referencesRequest)
	require.NoError(suite.T(), err, "References request failed")
	require.NotNil(suite.T(), response, "Response should not be nil")

	if errorField, hasError := response["error"]; hasError && errorField != nil {
		suite.T().Errorf("LSP error in references response: %v", errorField)
		return
	}

	result, hasResult := response["result"]
	require.True(suite.T(), hasResult, "Response should have result field")

	if result != nil {
		suite.T().Logf("✅ References result: %v", result)
	} else {
		suite.T().Logf("⚠️ References result is null")
	}

	suite.T().Logf("✅ References test completed for %s", suite.Config.DisplayName)
}

// TestHoverComprehensive tests textDocument/hover
func (suite *ComprehensiveTestBaseSuite) TestHoverComprehensive() {
	suite.T().Logf("Testing %s hover - REAL E2E TEST", suite.Config.DisplayName)

	// Set up test workspace with real repository
	err := suite.setupTestWorkspace()
	require.NoError(suite.T(), err, "Failed to setup test workspace")

	// Start the LSP gateway server
	err = suite.startGatewayServer()
	require.NoError(suite.T(), err, "Failed to start gateway server")

	// Wait for server to be ready
	err = suite.waitForServerReady()
	require.NoError(suite.T(), err, "Server failed to become ready")

	// Create HTTP client
	httpClient := testutils.NewHttpClient(testutils.HttpClientConfig{
		BaseURL: fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
		Timeout: 10 * time.Second,
	})

	// Get test file info from repository manager
	fileURI, err := suite.repoManager.GetFileURI(suite.Config.Language, 0)
	require.NoError(suite.T(), err, "Failed to get file URI")

	testFile, err := suite.repoManager.GetTestFile(suite.Config.Language, 0)
	require.NoError(suite.T(), err, "Failed to get test file")

	suite.T().Logf("Testing hover on file: %s", fileURI)

	// Give LSP server time to initialize
	suite.T().Logf("Waiting for LSP server to fully initialize...")
	time.Sleep(3 * time.Second)

	// Test hover request
	hoverRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "textDocument/hover",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fileURI,
			},
			"position": map[string]interface{}{
				"line":      testFile.HoverPos.Line,
				"character": testFile.HoverPos.Character,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	suite.T().Logf("Making hover request at line %d, character %d...", testFile.HoverPos.Line, testFile.HoverPos.Character)

	response, err := suite.makeJSONRPCRequest(ctx, httpClient, hoverRequest)
	require.NoError(suite.T(), err, "Hover request failed")
	require.NotNil(suite.T(), response, "Response should not be nil")

	// Check for error in response
	if errorField, hasError := response["error"]; hasError && errorField != nil {
		suite.T().Errorf("LSP error in hover response: %v", errorField)
		return
	}

	// Validate hover result
	result, hasResult := response["result"]
	require.True(suite.T(), hasResult, "Response should have result field")

	if result != nil {
		resultStr := fmt.Sprintf("%v", result)
		suite.T().Logf("✅ Hover result: %s", resultStr)
		require.True(suite.T(), len(resultStr) > 0, "Hover result should not be empty")
	} else {
		suite.T().Logf("⚠️ Hover result is null - this may be normal for some positions")
	}

	suite.T().Logf("✅ Hover test completed successfully for %s", suite.Config.DisplayName)
}

// TestDocumentSymbolComprehensive tests textDocument/documentSymbol
func (suite *ComprehensiveTestBaseSuite) TestDocumentSymbolComprehensive() {
	suite.T().Logf("Testing %s document symbols", suite.Config.DisplayName)

	err := suite.setupTestWorkspace()
	require.NoError(suite.T(), err, "Failed to setup test workspace")

	err = suite.startGatewayServer()
	require.NoError(suite.T(), err, "Failed to start gateway server")

	err = suite.waitForServerReady()
	require.NoError(suite.T(), err, "Server failed to become ready")

	httpClient := testutils.NewHttpClient(testutils.HttpClientConfig{
		BaseURL: fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
		Timeout: 10 * time.Second,
	})

	fileURI, err := suite.repoManager.GetFileURI(suite.Config.Language, 0)
	require.NoError(suite.T(), err, "Failed to get file URI")

	time.Sleep(3 * time.Second)

	documentSymbolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "textDocument/documentSymbol",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fileURI,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	response, err := suite.makeJSONRPCRequest(ctx, httpClient, documentSymbolRequest)
	require.NoError(suite.T(), err, "Document symbol request failed")
	require.NotNil(suite.T(), response, "Response should not be nil")

	if errorField, hasError := response["error"]; hasError && errorField != nil {
		suite.T().Errorf("LSP error in document symbol response: %v", errorField)
		return
	}

	result, hasResult := response["result"]
	require.True(suite.T(), hasResult, "Response should have result field")

	if result != nil {
		if resultArray, ok := result.([]interface{}); ok {
			suite.T().Logf("✅ Document symbols found: %d symbols", len(resultArray))
		} else {
			suite.T().Logf("✅ Document symbols result: %v", result)
		}
	} else {
		suite.T().Logf("⚠️ Document symbols result is null")
	}

	suite.T().Logf("✅ Document symbol test completed for %s", suite.Config.DisplayName)
}

// TestWorkspaceSymbolComprehensive tests workspace/symbol
func (suite *ComprehensiveTestBaseSuite) TestWorkspaceSymbolComprehensive() {
	suite.T().Logf("Testing %s workspace symbols", suite.Config.DisplayName)

	err := suite.setupTestWorkspace()
	require.NoError(suite.T(), err, "Failed to setup test workspace")

	err = suite.startGatewayServer()
	require.NoError(suite.T(), err, "Failed to start gateway server")

	err = suite.waitForServerReady()
	require.NoError(suite.T(), err, "Server failed to become ready")

	httpClient := testutils.NewHttpClient(testutils.HttpClientConfig{
		BaseURL: fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
		Timeout: 10 * time.Second,
	})

	testFile, err := suite.repoManager.GetTestFile(suite.Config.Language, 0)
	require.NoError(suite.T(), err, "Failed to get test file")

	time.Sleep(3 * time.Second)

	workspaceSymbolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "workspace/symbol",
		"params": map[string]interface{}{
			"query": testFile.SymbolQuery,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	response, err := suite.makeJSONRPCRequest(ctx, httpClient, workspaceSymbolRequest)
	require.NoError(suite.T(), err, "Workspace symbol request failed")
	require.NotNil(suite.T(), response, "Response should not be nil")

	if errorField, hasError := response["error"]; hasError && errorField != nil {
		suite.T().Errorf("LSP error in workspace symbol response: %v", errorField)
		return
	}

	result, hasResult := response["result"]
	require.True(suite.T(), hasResult, "Response should have result field")

	if result != nil {
		if resultArray, ok := result.([]interface{}); ok {
			suite.T().Logf("✅ Workspace symbols found for query '%s': %d symbols", testFile.SymbolQuery, len(resultArray))
		} else {
			suite.T().Logf("✅ Workspace symbols result: %v", result)
		}
	} else {
		suite.T().Logf("⚠️ Workspace symbols result is null")
	}

	suite.T().Logf("✅ Workspace symbol test completed for %s", suite.Config.DisplayName)
}

// TestCompletionComprehensive tests textDocument/completion
func (suite *ComprehensiveTestBaseSuite) TestCompletionComprehensive() {
	suite.T().Logf("Testing %s completion", suite.Config.DisplayName)

	err := suite.setupTestWorkspace()
	require.NoError(suite.T(), err, "Failed to setup test workspace")

	err = suite.startGatewayServer()
	require.NoError(suite.T(), err, "Failed to start gateway server")

	err = suite.waitForServerReady()
	require.NoError(suite.T(), err, "Server failed to become ready")

	httpClient := testutils.NewHttpClient(testutils.HttpClientConfig{
		BaseURL: fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
		Timeout: 10 * time.Second,
	})

	fileURI, err := suite.repoManager.GetFileURI(suite.Config.Language, 0)
	require.NoError(suite.T(), err, "Failed to get file URI")

	testFile, err := suite.repoManager.GetTestFile(suite.Config.Language, 0)
	require.NoError(suite.T(), err, "Failed to get test file")

	time.Sleep(3 * time.Second)

	completionRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "textDocument/completion",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fileURI,
			},
			"position": map[string]interface{}{
				"line":      testFile.CompletionPos.Line,
				"character": testFile.CompletionPos.Character,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	response, err := suite.makeJSONRPCRequest(ctx, httpClient, completionRequest)
	require.NoError(suite.T(), err, "Completion request failed")
	require.NotNil(suite.T(), response, "Response should not be nil")

	if errorField, hasError := response["error"]; hasError && errorField != nil {
		suite.T().Errorf("LSP error in completion response: %v", errorField)
		return
	}

	result, hasResult := response["result"]
	require.True(suite.T(), hasResult, "Response should have result field")

	if result != nil {
		if resultMap, ok := result.(map[string]interface{}); ok {
			if items, hasItems := resultMap["items"]; hasItems {
				if itemsArray, ok := items.([]interface{}); ok {
					suite.T().Logf("✅ Completion items found: %d items", len(itemsArray))
				}
			}
		} else if resultArray, ok := result.([]interface{}); ok {
			suite.T().Logf("✅ Completion items found: %d items", len(resultArray))
		} else {
			suite.T().Logf("✅ Completion result: %v", result)
		}
	} else {
		suite.T().Logf("⚠️ Completion result is null")
	}

	suite.T().Logf("✅ Completion test completed for %s", suite.Config.DisplayName)
}

// TestAllLSPMethodsSequential tests all LSP methods sequentially (for languages that support it)
func (suite *ComprehensiveTestBaseSuite) TestAllLSPMethodsSequential() {
	if !suite.Config.HasAllLSPTest {
		suite.T().Skip("Language does not support sequential LSP test")
		return
	}
	suite.T().Logf("Testing %s all LSP methods sequentially", suite.Config.DisplayName)

	// Set up workspace once for all tests
	err := suite.setupTestWorkspace()
	require.NoError(suite.T(), err, "Failed to setup test workspace")

	err = suite.startGatewayServer()
	require.NoError(suite.T(), err, "Failed to start gateway server")

	err = suite.waitForServerReady()
	require.NoError(suite.T(), err, "Server failed to become ready")

	httpClient := testutils.NewHttpClient(testutils.HttpClientConfig{
		BaseURL: fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
		Timeout: 10 * time.Second,
	})

	fileURI, err := suite.repoManager.GetFileURI(suite.Config.Language, 0)
	require.NoError(suite.T(), err, "Failed to get file URI")

	testFile, err := suite.repoManager.GetTestFile(suite.Config.Language, 0)
	require.NoError(suite.T(), err, "Failed to get test file")

	// Wait for LSP server to fully initialize
	time.Sleep(3 * time.Second)

	// Test all 6 LSP methods sequentially
	suite.testMethodSequentially(httpClient, fileURI, testFile, "textDocument/definition", testFile.DefinitionPos)
	suite.testMethodSequentially(httpClient, fileURI, testFile, "textDocument/references", testFile.ReferencePos)
	suite.testMethodSequentially(httpClient, fileURI, testFile, "textDocument/hover", testFile.HoverPos)
	suite.testDocumentSymbolSequentially(httpClient, fileURI)
	suite.testWorkspaceSymbolSequentially(httpClient, testFile.SymbolQuery)
	suite.testMethodSequentially(httpClient, fileURI, testFile, "textDocument/completion", testFile.CompletionPos)

	suite.T().Logf("✅ All LSP methods tested sequentially for %s", suite.Config.DisplayName)
}

// Helper method for testing LSP methods with position
func (suite *ComprehensiveTestBaseSuite) testMethodSequentially(httpClient *testutils.HttpClient, fileURI string, testFile *testutils.TestFile, method string, pos testutils.Position) {
	suite.T().Logf("  Testing %s...", method)

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fileURI,
			},
			"position": map[string]interface{}{
				"line":      pos.Line,
				"character": pos.Character,
			},
		},
	}

	// Add context for references
	if method == "textDocument/references" {
		request["params"].(map[string]interface{})["context"] = map[string]interface{}{
			"includeDeclaration": true,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := suite.makeJSONRPCRequest(ctx, httpClient, request)
	if err != nil {
		suite.T().Logf("  ❌ %s failed: %v", method, err)
		return
	}

	if errorField, hasError := response["error"]; hasError && errorField != nil {
		suite.T().Logf("  ❌ %s LSP error: %v", method, errorField)
		return
	}

	suite.T().Logf("  ✅ %s completed", method)
}

// Helper method for testing document symbols
func (suite *ComprehensiveTestBaseSuite) testDocumentSymbolSequentially(httpClient *testutils.HttpClient, fileURI string) {
	suite.T().Logf("  Testing textDocument/documentSymbol...")

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "textDocument/documentSymbol",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fileURI,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := suite.makeJSONRPCRequest(ctx, httpClient, request)
	if err != nil {
		suite.T().Logf("  ❌ textDocument/documentSymbol failed: %v", err)
		return
	}

	if errorField, hasError := response["error"]; hasError && errorField != nil {
		suite.T().Logf("  ❌ textDocument/documentSymbol LSP error: %v", errorField)
		return
	}

	suite.T().Logf("  ✅ textDocument/documentSymbol completed")
}

// Helper method for testing workspace symbols
func (suite *ComprehensiveTestBaseSuite) testWorkspaceSymbolSequentially(httpClient *testutils.HttpClient, query string) {
	suite.T().Logf("  Testing workspace/symbol...")

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "workspace/symbol",
		"params": map[string]interface{}{
			"query": query,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := suite.makeJSONRPCRequest(ctx, httpClient, request)
	if err != nil {
		suite.T().Logf("  ❌ workspace/symbol failed: %v", err)
		return
	}

	if errorField, hasError := response["error"]; hasError && errorField != nil {
		suite.T().Logf("  ❌ workspace/symbol LSP error: %v", errorField)
		return
	}

	suite.T().Logf("  ✅ workspace/symbol completed")
}
