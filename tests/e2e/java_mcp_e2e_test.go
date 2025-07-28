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

// JavaMCPE2ETestSuite tests Java LSP functionality through MCP protocol using in28minutes/clean-code repository
type JavaMCPE2ETestSuite struct {
	suite.Suite
	
	// Core infrastructure
	mcpCmd        *exec.Cmd
	mcpStdin      io.WriteCloser
	mcpReader     *bufio.Reader
	configPath    string
	tempDir       string
	projectRoot   string
	testTimeout   time.Duration
	
	// Java repository management with fixed commit
	repoManager   testutils.RepositoryManager
	repoDir       string
	javaFiles     []string
	
	// Server state tracking
	serverStarted bool
}

// SetupSuite initializes the test suite for Java MCP testing using in28minutes/clean-code repository
func (suite *JavaMCPE2ETestSuite) SetupSuite() {
	suite.testTimeout = 5 * time.Minute
	
	var err error
	suite.projectRoot, err = testutils.GetProjectRoot()
	suite.Require().NoError(err, "Failed to get project root")
	
	suite.tempDir, err = os.MkdirTemp("", "java-mcp-e2e-*")
	suite.Require().NoError(err, "Failed to create temp directory")
	
	// Initialize Java repository manager with in28minutes/clean-code repository
	suite.setupJavaRepositoryManager()
	
	// Setup clean-code repository
	suite.repoDir, err = suite.repoManager.SetupRepository()
	suite.Require().NoError(err, "Failed to setup in28minutes/clean-code repository")
	
	// Discover Java files in clean-code repository
	suite.discoverJavaFiles()
	
	// Create test configuration for Java MCP
	suite.createJavaMCPConfig()
}

// SetupTest initializes fresh components for each test
func (suite *JavaMCPE2ETestSuite) SetupTest() {
	suite.serverStarted = false
}

// TearDownTest cleans up per-test resources
func (suite *JavaMCPE2ETestSuite) TearDownTest() {
	suite.stopMCPServer()
}

// TearDownSuite performs final cleanup
func (suite *JavaMCPE2ETestSuite) TearDownSuite() {
	if suite.repoManager != nil {
		if err := suite.repoManager.Cleanup(); err != nil {
			suite.T().Logf("Warning: Failed to cleanup in28minutes/clean-code repository: %v", err)
		}
	}
	
	if suite.tempDir != "" {
		if err := os.RemoveAll(suite.tempDir); err != nil {
			suite.T().Logf("Warning: Failed to remove temp directory: %v", err)
		}
	}
}

// TestJavaMCPServerLifecycle tests the complete MCP server lifecycle for Java
func (suite *JavaMCPE2ETestSuite) TestJavaMCPServerLifecycle() {
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	suite.verifyMCPServerReadiness()
	suite.testBasicMCPOperations()
}

// TestJavaMCPDefinitionFeature tests textDocument/definition via MCP for Java files
func (suite *JavaMCPE2ETestSuite) TestJavaMCPDefinitionFeature() {
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	// Test definition on clean-code Java files
	if len(suite.javaFiles) > 0 {
		testFile := suite.javaFiles[0]
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
					"line":      5,  // Test at line 5 for potential class/method definitions
					"character": 10,
				},
			},
		}
		
		response, err := testutils.SendMCPStdioMessage(suite.mcpStdin, suite.mcpReader, definitionRequest)
		suite.NoError(err, "MCP definition request should not fail")
		suite.NotNil(response, "MCP response should not be nil")
		suite.Equal("2.0", response.Jsonrpc, "Response should be JSON-RPC 2.0")
		
		suite.T().Logf("MCP definition response for Java file %s: %+v", testFile, response.Result)
	}
}

// TestJavaMCPWorkspaceSymbolFeature tests workspace/symbol via MCP for Java files  
func (suite *JavaMCPE2ETestSuite) TestJavaMCPWorkspaceSymbolFeature() {
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	// Create MCP request for workspace/symbol searching for common Java patterns
	symbolRequest := testutils.MCPMessage{
		Jsonrpc: "2.0",
		ID:      2,
		Method:  "workspace/symbol",
		Params: map[string]interface{}{
			"query": "class",  // Search for class symbols in clean-code repository
		},
	}
	
	response, err := testutils.SendMCPStdioMessage(suite.mcpStdin, suite.mcpReader, symbolRequest)
	suite.NoError(err, "MCP workspace symbol request should not fail")
	suite.NotNil(response, "MCP response should not be nil")
	suite.Equal("2.0", response.Jsonrpc, "Response should be JSON-RPC 2.0")
	
	suite.T().Logf("MCP workspace symbol response for query 'class': %+v", response.Result)
}

// TestJavaMCPHoverFeature tests textDocument/hover via MCP for Java files
func (suite *JavaMCPE2ETestSuite) TestJavaMCPHoverFeature() {
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	// Test hover on Java files
	if len(suite.javaFiles) > 0 {
		testFile := suite.javaFiles[0]
		fileURI := suite.getFileURI(testFile)
		
		// Create MCP request for textDocument/hover
		hoverRequest := testutils.MCPMessage{
			Jsonrpc: "2.0",
			ID:      3,
			Method:  "textDocument/hover",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": fileURI,
				},
				"position": map[string]interface{}{
					"line":      3,  // Test hover on early lines for imports/declarations
					"character": 8,
				},
			},
		}
		
		response, err := testutils.SendMCPStdioMessage(suite.mcpStdin, suite.mcpReader, hoverRequest)
		suite.NoError(err, "MCP hover request should not fail")
		suite.NotNil(response, "MCP response should not be nil")
		suite.Equal("2.0", response.Jsonrpc, "Response should be JSON-RPC 2.0")
		
		suite.T().Logf("MCP hover response for Java file %s: %+v", testFile, response.Result)
	}
}

// TestJavaMCPDocumentSymbolFeature tests textDocument/documentSymbol via MCP for Java files
func (suite *JavaMCPE2ETestSuite) TestJavaMCPDocumentSymbolFeature() {
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	// Test document symbols on Java files
	if len(suite.javaFiles) > 0 {
		testFile := suite.javaFiles[0]
		fileURI := suite.getFileURI(testFile)
		
		// Create MCP request for textDocument/documentSymbol
		documentSymbolRequest := testutils.MCPMessage{
			Jsonrpc: "2.0",
			ID:      4,
			Method:  "textDocument/documentSymbol",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": fileURI,
				},
			},
		}
		
		response, err := testutils.SendMCPStdioMessage(suite.mcpStdin, suite.mcpReader, documentSymbolRequest)
		suite.NoError(err, "MCP document symbol request should not fail")
		suite.NotNil(response, "MCP response should not be nil")
		suite.Equal("2.0", response.Jsonrpc, "Response should be JSON-RPC 2.0")
		
		suite.T().Logf("MCP document symbol response for Java file %s: %+v", testFile, response.Result)
	}
}

// TestJavaMCPReferencesFeature tests textDocument/references via MCP for Java files
func (suite *JavaMCPE2ETestSuite) TestJavaMCPReferencesFeature() {
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	// Test references on Java files
	if len(suite.javaFiles) > 0 {
		testFile := suite.javaFiles[0]
		fileURI := suite.getFileURI(testFile)
		
		// Create MCP request for textDocument/references
		referencesRequest := testutils.MCPMessage{
			Jsonrpc: "2.0",
			ID:      5,
			Method:  "textDocument/references",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": fileURI,
				},
				"position": map[string]interface{}{
					"line":      5,  // Test references at line 5
					"character": 10,
				},
				"context": map[string]interface{}{
					"includeDeclaration": true,
				},
			},
		}
		
		response, err := testutils.SendMCPStdioMessage(suite.mcpStdin, suite.mcpReader, referencesRequest)
		suite.NoError(err, "MCP references request should not fail")
		suite.NotNil(response, "MCP response should not be nil")
		suite.Equal("2.0", response.Jsonrpc, "Response should be JSON-RPC 2.0")
		
		suite.T().Logf("MCP references response for Java file %s: %+v", testFile, response.Result)
	}
}

// Helper methods for Java repository and MCP testing

func (suite *JavaMCPE2ETestSuite) setupJavaRepositoryManager() {
	// Use the configured Java repository manager for in28minutes/clean-code
	suite.repoManager = testutils.NewJavaRepositoryManager()
}

func (suite *JavaMCPE2ETestSuite) discoverJavaFiles() {
	testFiles, err := suite.repoManager.GetTestFiles()
	suite.Require().NoError(err, "Failed to get Java test files from in28minutes/clean-code repository")
	suite.Require().Greater(len(testFiles), 0, "No Java files found in in28minutes/clean-code repository")
	
	// Filter for Java files
	for _, file := range testFiles {
		ext := filepath.Ext(file)
		if ext == ".java" {
			suite.javaFiles = append(suite.javaFiles, file)
		}
	}
	
	suite.Require().Greater(len(suite.javaFiles), 0, "No Java files found in in28minutes/clean-code repository")
	suite.T().Logf("Found %d Java files in in28minutes/clean-code repository", len(suite.javaFiles))
}

func (suite *JavaMCPE2ETestSuite) createJavaMCPConfig() {
	// Create MCP-specific configuration for Java
	options := testutils.DefaultLanguageConfigOptions("java")
	options.ConfigType = "mcp"
	
	// Add Java-specific custom variables for MCP
	options.CustomVariables["JAVA_HOME"] = "/usr/lib/jvm/default-java"
	options.CustomVariables["MAVEN_HOME"] = "/usr/share/maven"
	options.CustomVariables["MCP_MODE"] = "stdio"
	options.CustomVariables["REPOSITORY"] = "clean-code"
	options.CustomVariables["COMMIT_HASH"] = "9a55768b26988766e4413019ed4cd23055a66709"
	options.CustomVariables["LSP_SERVER"] = "jdtls"
	options.CustomVariables["TRANSPORT"] = "stdio"
	
	configPath, cleanup, err := testutils.CreateLanguageConfig(suite.repoManager, options)
	suite.Require().NoError(err, "Failed to create Java MCP test config")
	
	suite.configPath = configPath
	_ = cleanup // Will be cleaned up via tempDir
}

func (suite *JavaMCPE2ETestSuite) getFileURI(filePath string) string {
	workspaceDir := suite.repoManager.GetWorkspaceDir()
	return "file://" + filepath.Join(workspaceDir, filePath)
}

func (suite *JavaMCPE2ETestSuite) startMCPServer() {
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

func (suite *JavaMCPE2ETestSuite) stopMCPServer() {
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

func (suite *JavaMCPE2ETestSuite) waitForMCPServerReadiness() {
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

func (suite *JavaMCPE2ETestSuite) verifyMCPServerReadiness() {
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

func (suite *JavaMCPE2ETestSuite) testBasicMCPOperations() {
	// Test basic MCP operations with in28minutes/clean-code repository
	if len(suite.javaFiles) > 0 {
		testFile := suite.javaFiles[0]
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
					"line":      1,  // Test hover on early line for class declaration
					"character": 5,
				},
			},
		}
		
		response, err := testutils.SendMCPStdioMessage(suite.mcpStdin, suite.mcpReader, hoverRequest)
		suite.NoError(err, "Basic MCP hover operation should not fail")
		suite.NotNil(response, "Basic MCP hover response should not be nil")
		
		suite.T().Logf("Basic MCP operations test completed for Java file: %s", testFile)
	}
}

// Test runner function
func TestJavaMCPE2ETestSuite(t *testing.T) {
	suite.Run(t, new(JavaMCPE2ETestSuite))
}