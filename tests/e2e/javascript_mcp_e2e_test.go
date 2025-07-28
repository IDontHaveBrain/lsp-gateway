package e2e_test

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/tests/e2e/testutils"
)

// JavaScriptMCPE2ETestSuite tests JavaScript LSP functionality through MCP protocol using chalk repository
type JavaScriptMCPE2ETestSuite struct {
	suite.Suite
	
	// Core infrastructure
	mcpCmd        *exec.Cmd
	mcpStdin      io.WriteCloser
	mcpReader     *bufio.Reader
	configPath    string
	tempDir       string
	projectRoot   string
	testTimeout   time.Duration
	
	// Chalk repository management with fixed commit
	repoManager   testutils.RepositoryManager
	repoDir       string
	jsFiles       []string
	
	// Server state tracking
	serverStarted bool
}

// SetupSuite initializes the test suite for JavaScript MCP testing using chalk repository
func (suite *JavaScriptMCPE2ETestSuite) SetupSuite() {
	suite.testTimeout = 5 * time.Minute
	
	var err error
	suite.projectRoot, err = testutils.GetProjectRoot()
	suite.Require().NoError(err, "Failed to get project root")
	
	suite.tempDir, err = os.MkdirTemp("", "js-mcp-e2e-*")
	suite.Require().NoError(err, "Failed to create temp directory")
	
	// Initialize custom repository manager for chalk with fixed commit
	suite.setupChalkRepositoryManager()
	
	// Setup chalk repository
	suite.repoDir, err = suite.repoManager.SetupRepository()
	suite.Require().NoError(err, "Failed to setup chalk repository")
	
	// Discover JavaScript/TypeScript files in chalk repository
	suite.discoverJavaScriptFiles()
	
	// Create test configuration for JavaScript MCP
	suite.createJavaScriptMCPConfig()
}

// SetupTest initializes fresh components for each test
func (suite *JavaScriptMCPE2ETestSuite) SetupTest() {
	suite.serverStarted = false
}

// TearDownTest cleans up per-test resources
func (suite *JavaScriptMCPE2ETestSuite) TearDownTest() {
	suite.stopMCPServer()
}

// TearDownSuite performs final cleanup
func (suite *JavaScriptMCPE2ETestSuite) TearDownSuite() {
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
}

// TestJavaScriptMCPServerLifecycle tests the complete MCP server lifecycle for JavaScript
func (suite *JavaScriptMCPE2ETestSuite) TestJavaScriptMCPServerLifecycle() {
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	suite.verifyMCPServerReadiness()
	suite.testBasicMCPOperations()
}

// TestJavaScriptMCPDefinitionFeature tests textDocument/definition via MCP for JavaScript files
func (suite *JavaScriptMCPE2ETestSuite) TestJavaScriptMCPDefinitionFeature() {
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	// Test definition on chalk JavaScript files
	if len(suite.jsFiles) > 0 {
		testFile := suite.jsFiles[0]
		fileURI := suite.getFileURI(testFile)
		
		// Create MCP request for textDocument/definition
		definitionRequest := testutils.MCPMessage{
			Jsonrpc: "2.0",
			ID:      1,
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
		
		response, err := testutils.SendMCPStdioMessage(suite.mcpStdin, suite.mcpReader, definitionRequest)
		suite.NoError(err, "MCP definition request should not fail")
		suite.NotNil(response, "MCP response should not be nil")
		suite.Equal("2.0", response.Jsonrpc, "Response should be JSON-RPC 2.0")
		
		suite.T().Logf("MCP definition response for JavaScript file %s: %+v", testFile, response.Result)
	}
}

// TestJavaScriptMCPWorkspaceSymbolFeature tests workspace/symbol via MCP for JavaScript files  
func (suite *JavaScriptMCPE2ETestSuite) TestJavaScriptMCPWorkspaceSymbolFeature() {
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	// Create MCP request for workspace/symbol
	symbolRequest := testutils.MCPMessage{
		Jsonrpc: "2.0",
		ID:      2,
		Method:  "workspace/symbol",
		Params: map[string]interface{}{
			"query": "chalk",
		},
	}
	
	response, err := testutils.SendMCPStdioMessage(suite.mcpStdin, suite.mcpReader, symbolRequest)
	suite.NoError(err, "MCP workspace symbol request should not fail")
	suite.NotNil(response, "MCP response should not be nil")
	suite.Equal("2.0", response.Jsonrpc, "Response should be JSON-RPC 2.0")
	
	suite.T().Logf("MCP workspace symbol response for query 'chalk': %+v", response.Result)
}

// Helper methods for chalk repository and MCP testing

func (suite *JavaScriptMCPE2ETestSuite) setupChalkRepositoryManager() {
	// Create custom language config for chalk repository with fixed commit
	chalkConfig := testutils.NewCustomLanguageConfig("javascript", "https://github.com/chalk/chalk.git").
		WithTestPaths("source", "test").
		WithFilePatterns("*.js", "*.ts", "*.mjs").
		WithRootMarkers("package.json", "tsconfig.json").
		WithExcludePaths("node_modules", ".git", "dist", "build", "coverage").
		WithRepoSubDir("chalk").
		WithCustomVariable("lsp_server", "typescript-language-server").
		WithCustomVariable("transport", "stdio").
		WithCustomVariable("commit_hash", "5dbc1e2").  // Fixed commit hash for v5.4.1
		Build()
	
	// Create custom repo config with commit hash
	repoConfig := testutils.GenericRepoConfig{
		CloneTimeout:   300 * time.Second,
		EnableLogging:  true,
		ForceClean:     false,
		PreserveGitDir: true,  // Keep .git for commit checkout
		CommitHash:     "5dbc1e2",  // Checkout specific commit
	}
	
	suite.repoManager = testutils.NewCustomRepositoryManager(chalkConfig, repoConfig)
}

func (suite *JavaScriptMCPE2ETestSuite) discoverJavaScriptFiles() {
	testFiles, err := suite.repoManager.GetTestFiles()
	suite.Require().NoError(err, "Failed to get JavaScript test files from chalk repository")
	suite.Require().Greater(len(testFiles), 0, "No JavaScript files found in chalk repository")
	
	// Filter for JavaScript/TypeScript files
	for _, file := range testFiles {
		ext := filepath.Ext(file)
		if ext == ".js" || ext == ".ts" || ext == ".mjs" || ext == ".jsx" || ext == ".tsx" {
			suite.jsFiles = append(suite.jsFiles, file)
		}
	}
	
	suite.Require().Greater(len(suite.jsFiles), 0, "No JavaScript/TypeScript files found in chalk repository")
	suite.T().Logf("Found %d JavaScript files in chalk repository", len(suite.jsFiles))
}

func (suite *JavaScriptMCPE2ETestSuite) createJavaScriptMCPConfig() {
	// Create MCP-specific configuration for JavaScript
	options := testutils.DefaultLanguageConfigOptions("javascript")
	options.ConfigType = "mcp"
	
	// Add JavaScript-specific custom variables for MCP
	options.CustomVariables["NODE_PATH"] = "/usr/local/lib/node_modules"
	options.CustomVariables["TS_NODE_PROJECT"] = "tsconfig.json"
	options.CustomVariables["MCP_MODE"] = "stdio"
	options.CustomVariables["REPOSITORY"] = "chalk"
	options.CustomVariables["COMMIT_HASH"] = "5dbc1e2"
	
	configPath, cleanup, err := testutils.CreateLanguageConfig(suite.repoManager, options)
	suite.Require().NoError(err, "Failed to create JavaScript MCP test config")
	
	suite.configPath = configPath
	_ = cleanup // Will be cleaned up via tempDir
}

func (suite *JavaScriptMCPE2ETestSuite) getFileURI(filePath string) string {
	workspaceDir := suite.repoManager.GetWorkspaceDir()
	return "file://" + filepath.Join(workspaceDir, filePath)
}

func (suite *JavaScriptMCPE2ETestSuite) startMCPServer() {
	if suite.serverStarted {
		return
	}
	
	binaryPath := filepath.Join(suite.projectRoot, "bin", "lspg")
	suite.mcpCmd = exec.Command(binaryPath, "mcp", "--config", suite.configPath)
	suite.mcpCmd.Dir = suite.projectRoot
	
	// Setup STDIO pipes for MCP communication
	var err error
	suite.mcpStdin, err = suite.mcpCmd.StdinPipe()
	suite.Require().NoError(err, "Failed to create MCP stdin pipe")
	
	stdout, err := suite.mcpCmd.StdoutPipe()  
	suite.Require().NoError(err, "Failed to create MCP stdout pipe")
	
	suite.mcpReader = bufio.NewReader(stdout)
	
	err = suite.mcpCmd.Start()
	suite.Require().NoError(err, "Failed to start MCP server")
	
	suite.waitForMCPServerReadiness()
	suite.serverStarted = true
}

func (suite *JavaScriptMCPE2ETestSuite) stopMCPServer() {
	if !suite.serverStarted || suite.mcpCmd == nil {
		return
	}
	
	// Close stdin to signal shutdown
	if suite.mcpStdin != nil {
		suite.mcpStdin.Close()
	}
	
	if suite.mcpCmd.Process != nil {
		suite.mcpCmd.Process.Signal(syscall.SIGTERM)
		
		done := make(chan error)
		go func() {
			done <- suite.mcpCmd.Wait()
		}()
		
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			suite.mcpCmd.Process.Kill()
			suite.mcpCmd.Wait()
		}
	}
	
	suite.mcpCmd = nil
	suite.serverStarted = false
}

func (suite *JavaScriptMCPE2ETestSuite) waitForMCPServerReadiness() {
	// Wait a bit for MCP server to initialize
	time.Sleep(2 * time.Second)
	
	// Send initialize request to verify MCP server is ready
	initRequest := testutils.MCPMessage{
		Jsonrpc: "2.0",
		ID:      0,
		Method:  "initialize",
		Params: map[string]interface{}{
			"capabilities": map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "lspg-mcp-test",
				"version": "1.0.0",
			},
		},
	}
	
	response, err := testutils.SendMCPStdioMessage(suite.mcpStdin, suite.mcpReader, initRequest)
	suite.Require().NoError(err, "MCP initialize request should succeed")
	suite.Require().NotNil(response, "MCP initialize response should not be nil")
	suite.Require().Equal("2.0", response.Jsonrpc, "Initialize response should be JSON-RPC 2.0")
}

func (suite *JavaScriptMCPE2ETestSuite) verifyMCPServerReadiness() {
	// Verify server is responsive with a simple request
	pingRequest := testutils.MCPMessage{
		Jsonrpc: "2.0",
		ID:      999,
		Method:  "workspace/symbol",
		Params: map[string]interface{}{
			"query": "",
		},
	}
	
	response, err := testutils.SendMCPStdioMessage(suite.mcpStdin, suite.mcpReader, pingRequest)
	suite.Require().NoError(err, "MCP server readiness check should pass")
	suite.Require().NotNil(response, "MCP readiness response should not be nil")
}

func (suite *JavaScriptMCPE2ETestSuite) testBasicMCPOperations() {
	// Test basic MCP operations with chalk repository
	if len(suite.jsFiles) > 0 {
		testFile := suite.jsFiles[0]
		fileURI := suite.getFileURI(testFile)
		
		// Test hover operation
		hoverRequest := testutils.MCPMessage{
			Jsonrpc: "2.0",
			ID:      100,
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
		
		response, err := testutils.SendMCPStdioMessage(suite.mcpStdin, suite.mcpReader, hoverRequest)
		suite.NoError(err, "Basic MCP hover operation should not fail")
		suite.NotNil(response, "Basic MCP hover response should not be nil")
		
		suite.T().Logf("Basic MCP operations test completed for chalk file: %s", testFile)
	}
}

// Test runner function
func TestJavaScriptMCPE2ETestSuite(t *testing.T) {
	suite.Run(t, new(JavaScriptMCPE2ETestSuite))
}