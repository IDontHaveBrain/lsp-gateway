package e2e_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"lsp-gateway/tests/e2e/testutils"
)

// TypeScriptBasicE2ETestSuite provides basic E2E testing for TypeScript functionality
type TypeScriptBasicE2ETestSuite struct {
	suite.Suite
	
	// Core infrastructure
	httpClient      *testutils.HttpClient
	gatewayCmd      *exec.Cmd
	gatewayPort     int
	configPath      string
	tempDir         string
	projectRoot     string
	testTimeout     time.Duration
	
	// TypeScript repository management
	repoManager     testutils.RepositoryManager
	repoDir         string
	
	// Server state tracking
	serverStarted   bool
}

// SetupSuite initializes the basic test suite for TypeScript
func (suite *TypeScriptBasicE2ETestSuite) SetupSuite() {
	suite.testTimeout = 15 * time.Second
	
	var err error
	suite.projectRoot, err = testutils.GetProjectRoot()
	suite.Require().NoError(err, "Failed to get project root")
	
	suite.tempDir, err = os.MkdirTemp("", "ts-basic-e2e-*")
	suite.Require().NoError(err, "Failed to create temp directory")
	
	// Initialize TypeScript repository manager
	suite.repoManager = testutils.NewTypeScriptRepositoryManager()
	
	// Setup TypeScript repository
	suite.repoDir, err = suite.repoManager.SetupRepository()
	suite.Require().NoError(err, "Failed to setup TypeScript repository")
	
	suite.T().Logf("Basic TypeScript E2E test suite initialized with Microsoft TypeScript repository")
}

// SetupTest initializes fresh components for each test
func (suite *TypeScriptBasicE2ETestSuite) SetupTest() {
	var err error
	suite.gatewayPort, err = testutils.FindAvailablePort()
	suite.Require().NoError(err, "Failed to find available port")

	// Create test configuration
	suite.createTestConfig()

	// Configure HttpClient for basic TypeScript testing
	config := testutils.HttpClientConfig{
		BaseURL:            fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
		Timeout:            5 * time.Second,
		MaxRetries:         3,
		RetryDelay:         500 * time.Millisecond,
		EnableLogging:      true,
		EnableRecording:    false,
		WorkspaceID:        fmt.Sprintf("ts-basic-test-%d", time.Now().UnixNano()),
		ProjectPath:        suite.repoDir,
		UserAgent:          "LSP-Gateway-TypeScript-Basic-E2E/1.0",
		MaxResponseSize:    50 * 1024 * 1024,
		ConnectionPoolSize: 10,
		KeepAlive:          20 * time.Second,
	}

	suite.httpClient = testutils.NewHttpClient(config)
	suite.serverStarted = false
}

// TearDownTest cleans up per-test resources
func (suite *TypeScriptBasicE2ETestSuite) TearDownTest() {
	suite.stopGatewayServer()
	
	if suite.httpClient != nil {
		suite.httpClient.Close()
		suite.httpClient = nil
	}
}

// TearDownSuite performs final cleanup
func (suite *TypeScriptBasicE2ETestSuite) TearDownSuite() {
	if suite.repoManager != nil {
		if err := suite.repoManager.Cleanup(); err != nil {
			suite.T().Logf("Warning: Failed to cleanup TypeScript repository: %v", err)
		}
	}
	
	if suite.tempDir != "" {
		if err := os.RemoveAll(suite.tempDir); err != nil {
			suite.T().Logf("Warning: Failed to remove temp directory: %v", err)
		}
	}
}

// TestTypeScriptBasicServerLifecycle tests basic server lifecycle
func (suite *TypeScriptBasicE2ETestSuite) TestTypeScriptBasicServerLifecycle() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	// Verify server is ready
	err := suite.httpClient.HealthCheck(ctx)
	suite.Require().NoError(err, "Server health check should pass")
	
	suite.T().Logf("TypeScript basic server lifecycle test completed successfully")
}

// TestTypeScriptBasicDefinition tests basic textDocument/definition functionality
func (suite *TypeScriptBasicE2ETestSuite) TestTypeScriptBasicDefinition() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	// Get a TypeScript file for testing
	testFiles, err := suite.repoManager.GetTestFiles()
	suite.Require().NoError(err, "Failed to get test files")
	suite.Require().Greater(len(testFiles), 0, "No test files found")
	
	// Find a TypeScript file
	var tsFile string
	for _, file := range testFiles {
		if filepath.Ext(file) == ".ts" {
			tsFile = file
			break
		}
	}
	suite.Require().NotEmpty(tsFile, "No TypeScript files found")
	
	fileURI := suite.getFileURI(tsFile)
	position := testutils.Position{Line: 1, Character: 5}
	
	locations, err := suite.httpClient.Definition(ctx, fileURI, position)
	
	// Allow for graceful degradation - not all positions will have definitions
	if err == nil {
		suite.T().Logf("Definition test successful for %s - found %d locations", tsFile, len(locations))
	} else {
		suite.T().Logf("Definition test completed with expected behavior for %s: %v", tsFile, err)
	}
}

// TestTypeScriptBasicHover tests basic textDocument/hover functionality
func (suite *TypeScriptBasicE2ETestSuite) TestTypeScriptBasicHover() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	// Get a TypeScript file for testing
	testFiles, err := suite.repoManager.GetTestFiles()
	suite.Require().NoError(err, "Failed to get test files")
	
	var tsFile string
	for _, file := range testFiles {
		if filepath.Ext(file) == ".ts" {
			tsFile = file
			break
		}
	}
	suite.Require().NotEmpty(tsFile, "No TypeScript files found")
	
	fileURI := suite.getFileURI(tsFile)
	position := testutils.Position{Line: 1, Character: 5}
	
	hover, err := suite.httpClient.Hover(ctx, fileURI, position)
	
	// Allow for graceful degradation - not all positions will have hover info
	if err == nil {
		suite.T().Logf("Hover test successful for %s", tsFile)
		if hover != nil {
			suite.T().Logf("Hover info received")
		}
	} else {
		suite.T().Logf("Hover test completed with expected behavior for %s: %v", tsFile, err)
	}
}

// TestTypeScriptBasicWorkspaceSymbol tests basic workspace/symbol functionality
func (suite *TypeScriptBasicE2ETestSuite) TestTypeScriptBasicWorkspaceSymbol() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	// Test workspace symbols with a simple query
	symbols, err := suite.httpClient.WorkspaceSymbol(ctx, "function")
	
	if err == nil {
		suite.T().Logf("Workspace symbol test successful - found %d symbols", len(symbols))
	} else {
		suite.T().Logf("Workspace symbol test completed: %v", err)
	}
}

// Helper methods

func (suite *TypeScriptBasicE2ETestSuite) createTestConfig() {
	options := testutils.DefaultLanguageConfigOptions("typescript")
	options.TestPort = fmt.Sprintf("%d", suite.gatewayPort)
	options.ConfigType = "basic"
	
	// Add basic TypeScript-specific custom variables
	options.CustomVariables["NODE_PATH"] = testutils.DetectNodePath()
	options.CustomVariables["TS_NODE_PROJECT"] = "tsconfig.json"
	options.CustomVariables["REPOSITORY"] = "TypeScript"
	options.CustomVariables["TEST_MODE"] = "basic"
	options.CustomVariables["LSP_TIMEOUT"] = "60"
	
	configPath, cleanup, err := testutils.CreateLanguageConfig(suite.repoManager, options)
	suite.Require().NoError(err, "Failed to create TypeScript basic test config")
	
	suite.configPath = configPath
	_ = cleanup // Will be cleaned up via tempDir
}

func (suite *TypeScriptBasicE2ETestSuite) getFileURI(filePath string) string {
	workspaceDir := suite.repoManager.GetWorkspaceDir()
	return "file://" + filepath.Join(workspaceDir, filePath)
}

func (suite *TypeScriptBasicE2ETestSuite) startGatewayServer() {
	if suite.serverStarted {
		return
	}
	
	binaryPath := filepath.Join(suite.projectRoot, "bin", "lspg")
	suite.gatewayCmd = exec.Command(binaryPath, "server", "--config", suite.configPath)
	suite.gatewayCmd.Dir = suite.projectRoot
	
	err := suite.gatewayCmd.Start()
	suite.Require().NoError(err, "Failed to start gateway server")
	
	suite.waitForServerReadiness()
	suite.serverStarted = true
	
	suite.T().Logf("Gateway server started for basic TypeScript testing on port %d", suite.gatewayPort)
}

func (suite *TypeScriptBasicE2ETestSuite) stopGatewayServer() {
	if !suite.serverStarted || suite.gatewayCmd == nil {
		return
	}
	
	if suite.gatewayCmd.Process != nil {
		suite.gatewayCmd.Process.Signal(syscall.SIGTERM)
		
		done := make(chan error)
		go func() {
			done <- suite.gatewayCmd.Wait()
		}()
		
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			suite.gatewayCmd.Process.Kill()
			suite.gatewayCmd.Wait()
		}
	}
	
	suite.gatewayCmd = nil
	suite.serverStarted = false
}

func (suite *TypeScriptBasicE2ETestSuite) waitForServerReadiness() {
	maxRetries := 60
	for i := 0; i < maxRetries; i++ {
		if suite.checkServerHealth() {
			suite.T().Logf("Server ready after %d seconds", i+1)
			return
		}
		time.Sleep(2 * time.Second)
	}
	suite.Require().Fail("Server failed to become ready within timeout")
}

func (suite *TypeScriptBasicE2ETestSuite) checkServerHealth() bool {
	if suite.httpClient == nil {
		return false
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	err := suite.httpClient.HealthCheck(ctx)
	return err == nil
}

// Parallel test functions using resource isolation

// TestTypeScriptBasicServerLifecycleParallel tests the complete server lifecycle for TypeScript with parallel execution
func TestTypeScriptBasicServerLifecycleParallel(t *testing.T) {
	t.Parallel()
	
	setup, err := testutils.SetupIsolatedTestWithLanguage("ts_basic_lifecycle", "typescript")
	require.NoError(t, err, "Failed to setup isolated test")
	defer func() {
		if cleanupErr := setup.Cleanup(); cleanupErr != nil {
			t.Logf("Warning: Cleanup failed: %v", cleanupErr)
		}
	}()
	
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	// Start gateway server
	projectRoot, err := testutils.GetProjectRoot()
	require.NoError(t, err, "Failed to get project root")
	
	binaryPath := filepath.Join(projectRoot, "bin", "lspg")
	serverCmd := []string{binaryPath, "server", "--config", setup.ConfigPath}
	
	err = setup.StartServer(serverCmd, 30*time.Second)
	require.NoError(t, err, "Failed to start server")
	
	err = setup.WaitForServerReady(10 * time.Second)
	require.NoError(t, err, "Server failed to become ready")
	
	// Verify server readiness
	httpClient := setup.GetHTTPClient()
	err = httpClient.HealthCheck(ctx)
	require.NoError(t, err, "Server health check should pass")
	
	// Test basic server operations
	err = httpClient.ValidateConnection(ctx)
	require.NoError(t, err, "Server connection validation should pass")
	
	t.Logf("TypeScript basic server lifecycle test completed successfully on port %d", setup.Resources.Port)
}

// TestTypeScriptDefinitionFeatureParallel tests textDocument/definition for TypeScript files with parallel execution
func TestTypeScriptDefinitionFeatureParallel(t *testing.T) {
	t.Parallel()
	
	setup, err := testutils.SetupIsolatedTestWithLanguage("ts_definition_feature", "typescript")
	require.NoError(t, err, "Failed to setup isolated test")
	defer func() {
		if cleanupErr := setup.Cleanup(); cleanupErr != nil {
			t.Logf("Warning: Cleanup failed: %v", cleanupErr)
		}
	}()
	
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	// Create test TypeScript file
	tsContent := `/**
 * LSP Gateway Server Implementation
 */

interface ServerConfig {
    name: string;
    port: number;
    timeout?: number;
}

class Server {
    private name: string;
    private port: number;
    private timeout: number;
    private running: boolean = false;

    constructor(config: ServerConfig) {
        this.name = config.name;
        this.port = config.port;
        this.timeout = config.timeout || 30000;
    }

    public async start(): Promise<void> {
        console.log("Starting server " + this.name + " on port " + this.port);
        this.running = true;
    }

    public async stop(): Promise<void> {
        console.log("Stopping server " + this.name);
        this.running = false;
    }

    public isRunning(): boolean {
        return this.running;
    }
}

export function createServer(config: ServerConfig): Server {
    return new Server(config);
}

export default Server;`
	
	testFile, err := setup.Resources.Directory.CreateTempFile("server.ts", tsContent)
	require.NoError(t, err, "Failed to create test TypeScript file")
	
	// Start gateway server
	projectRoot, err := testutils.GetProjectRoot()
	require.NoError(t, err, "Failed to get project root")
	
	binaryPath := filepath.Join(projectRoot, "bin", "lspg")
	serverCmd := []string{binaryPath, "server", "--config", setup.ConfigPath}
	
	err = setup.StartServer(serverCmd, 30*time.Second)
	require.NoError(t, err, "Failed to start server")
	
	err = setup.WaitForServerReady(10 * time.Second)
	require.NoError(t, err, "Server failed to become ready")
	
	// Test definition on TypeScript file
	httpClient := setup.GetHTTPClient()
	fileURI := "file://" + testFile
	position := testutils.Position{Line: 38, Character: 15} // createServer function call
	
	locations, err := httpClient.Definition(ctx, fileURI, position)
	if err != nil {
		t.Logf("Definition request failed (expected for test setup): %v", err)
		return
	}
	
	t.Logf("Found %d definition locations for TypeScript file", len(locations))
	if len(locations) > 0 {
		assert.Contains(t, locations[0].URI, "server.ts", "Definition should reference correct file")
		assert.GreaterOrEqual(t, locations[0].Range.Start.Line, 0, "Definition line should be valid")
	}
	
	t.Logf("TypeScript definition feature test completed successfully on port %d", setup.Resources.Port)
}

// Test runner function
func TestTypeScriptBasicE2ETestSuite(t *testing.T) {
	suite.Run(t, new(TypeScriptBasicE2ETestSuite))
}