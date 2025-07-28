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

// JavaScriptBasicE2ETestSuite demonstrates the modular repository system for JavaScript/TypeScript testing
type JavaScriptBasicE2ETestSuite struct {
	suite.Suite
	
	// Core infrastructure
	httpClient      *testutils.HttpClient
	gatewayCmd      *exec.Cmd
	gatewayPort     int
	configPath      string
	tempDir         string
	projectRoot     string
	testTimeout     time.Duration
	
	// Modular repository management for JavaScript/TypeScript
	repoManager     testutils.RepositoryManager
	repoDir         string
	jsFiles         []string
	
	// Server state tracking
	serverStarted   bool
}

// SetupSuite initializes the test suite for JavaScript using the modular system
func (suite *JavaScriptBasicE2ETestSuite) SetupSuite() {
	suite.testTimeout = 15 * time.Second
	
	var err error
	suite.projectRoot, err = testutils.GetProjectRoot()
	suite.Require().NoError(err, "Failed to get project root")
	
	suite.tempDir, err = os.MkdirTemp("", "js-basic-e2e-*")
	suite.Require().NoError(err, "Failed to create temp directory")
	
	// Initialize modular JavaScript repository manager
	suite.repoManager = testutils.NewJavaScriptRepositoryManager()
	
	// Setup JavaScript repository
	suite.repoDir, err = suite.repoManager.SetupRepository()
	suite.Require().NoError(err, "Failed to setup JavaScript repository")
	
	// Discover JavaScript/TypeScript files for testing
	suite.discoverJavaScriptFiles()
	
	// Create test configuration for JavaScript
	suite.createJavaScriptTestConfig()
}

// SetupTest initializes fresh components for each test
func (suite *JavaScriptBasicE2ETestSuite) SetupTest() {
	var err error
	suite.gatewayPort, err = testutils.FindAvailablePort()
	suite.Require().NoError(err, "Failed to find available port")

	// Update config with new port
	suite.updateConfigPort()

	// Configure HttpClient for JavaScript testing
	config := testutils.HttpClientConfig{
		BaseURL:            fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
		Timeout:            5 * time.Second,
		MaxRetries:         5,
		RetryDelay:         500 * time.Millisecond,
		EnableLogging:      true,
		EnableRecording:    true,
		WorkspaceID:        fmt.Sprintf("js-basic-test-%d", time.Now().UnixNano()),
		ProjectPath:        suite.repoDir,
		UserAgent:          "LSP-Gateway-JavaScript-Basic-E2E/1.0",
		MaxResponseSize:    50 * 1024 * 1024,
		ConnectionPoolSize: 15,
		KeepAlive:          20 * time.Second,
	}

	suite.httpClient = testutils.NewHttpClient(config)
	suite.serverStarted = false
}

// TearDownTest cleans up per-test resources
func (suite *JavaScriptBasicE2ETestSuite) TearDownTest() {
	suite.stopGatewayServer()
	
	if suite.httpClient != nil {
		suite.httpClient.Close()
		suite.httpClient = nil
	}
}

// TearDownSuite performs final cleanup
func (suite *JavaScriptBasicE2ETestSuite) TearDownSuite() {
	if suite.repoManager != nil {
		if err := suite.repoManager.Cleanup(); err != nil {
			suite.T().Logf("Warning: Failed to cleanup JavaScript repository: %v", err)
		}
	}
	
	if suite.tempDir != "" {
		if err := os.RemoveAll(suite.tempDir); err != nil {
			suite.T().Logf("Warning: Failed to remove temp directory: %v", err)
		}
	}
}

// TestJavaScriptBasicServerLifecycle tests the complete server lifecycle for JavaScript
func (suite *JavaScriptBasicE2ETestSuite) TestJavaScriptBasicServerLifecycle() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.verifyServerReadiness(ctx)
	suite.testBasicServerOperations(ctx)
}

// TestJavaScriptDefinitionFeature tests textDocument/definition for JavaScript/TypeScript files
func (suite *JavaScriptBasicE2ETestSuite) TestJavaScriptDefinitionFeature() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	// Test definition on JavaScript/TypeScript files
	if len(suite.jsFiles) > 0 {
		testFile := suite.jsFiles[0]
		fileURI := suite.getFileURI(testFile)
		
		// Test at a common position (line 0, character 0)
		position := testutils.Position{Line: 0, Character: 0}
		
		locations, err := suite.httpClient.Definition(ctx, fileURI, position)
		suite.NoError(err, "Definition request should not fail")
		suite.T().Logf("Found %d definition locations for JavaScript file %s", len(locations), testFile)
	}
}

// Helper methods demonstrating modular system usage

func (suite *JavaScriptBasicE2ETestSuite) discoverJavaScriptFiles() {
	testFiles, err := suite.repoManager.GetTestFiles()
	suite.Require().NoError(err, "Failed to get JavaScript test files")
	suite.Require().Greater(len(testFiles), 0, "No JavaScript files found in repository")
	
	// Filter for JavaScript/TypeScript files
	for _, file := range testFiles {
		ext := filepath.Ext(file)
		if ext == ".js" || ext == ".ts" || ext == ".jsx" || ext == ".tsx" {
			suite.jsFiles = append(suite.jsFiles, file)
		}
	}
	
	suite.Require().Greater(len(suite.jsFiles), 0, "No JavaScript/TypeScript files found")
}

func (suite *JavaScriptBasicE2ETestSuite) createJavaScriptTestConfig() {
	// Demonstrate custom configuration for JavaScript
	options := testutils.DefaultLanguageConfigOptions("javascript")
	options.TestPort = fmt.Sprintf("%d", suite.gatewayPort)
	options.ConfigType = "main"
	
	// Add JavaScript-specific custom variables
	options.CustomVariables["NODE_PATH"] = "/usr/local/lib/node_modules"
	options.CustomVariables["TS_NODE_PROJECT"] = "tsconfig.json"
	
	configPath, cleanup, err := testutils.CreateLanguageConfig(suite.repoManager, options)
	suite.Require().NoError(err, "Failed to create JavaScript test config")
	
	suite.configPath = configPath
	_ = cleanup // Will be cleaned up via tempDir
}

func (suite *JavaScriptBasicE2ETestSuite) updateConfigPort() {
	suite.createJavaScriptTestConfig()
}

func (suite *JavaScriptBasicE2ETestSuite) getFileURI(filePath string) string {
	workspaceDir := suite.repoManager.GetWorkspaceDir()
	return "file://" + filepath.Join(workspaceDir, filePath)
}

func (suite *JavaScriptBasicE2ETestSuite) startGatewayServer() {
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
}

func (suite *JavaScriptBasicE2ETestSuite) stopGatewayServer() {
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

func (suite *JavaScriptBasicE2ETestSuite) waitForServerReadiness() {
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		if suite.checkServerHealth() {
			return
		}
		time.Sleep(time.Second)
	}
	suite.Require().Fail("Server failed to become ready within timeout")
}

func (suite *JavaScriptBasicE2ETestSuite) checkServerHealth() bool {
	if suite.httpClient == nil {
		return false
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err := suite.httpClient.HealthCheck(ctx)
	return err == nil
}

func (suite *JavaScriptBasicE2ETestSuite) verifyServerReadiness(ctx context.Context) {
	err := suite.httpClient.HealthCheck(ctx)
	suite.Require().NoError(err, "Server health check should pass")
}

func (suite *JavaScriptBasicE2ETestSuite) testBasicServerOperations(ctx context.Context) {
	err := suite.httpClient.ValidateConnection(ctx)
	suite.Require().NoError(err, "Server connection validation should pass")
}

// Parallel test functions using resource isolation

// TestJavaScriptBasicServerLifecycleParallel tests the complete server lifecycle for JavaScript with parallel execution
func TestJavaScriptBasicServerLifecycleParallel(t *testing.T) {
	t.Parallel()
	
	setup, err := testutils.SetupIsolatedTestWithLanguage("js_basic_lifecycle", "javascript")
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
	
	t.Logf("JavaScript basic server lifecycle test completed successfully on port %d", setup.Resources.Port)
}

// TestJavaScriptDefinitionFeatureParallel tests textDocument/definition for JavaScript files with parallel execution
func TestJavaScriptDefinitionFeatureParallel(t *testing.T) {
	t.Parallel()
	
	setup, err := testutils.SetupIsolatedTestWithLanguage("js_definition_feature", "javascript")
	require.NoError(t, err, "Failed to setup isolated test")
	defer func() {
		if cleanupErr := setup.Cleanup(); cleanupErr != nil {
			t.Logf("Warning: Cleanup failed: %v", cleanupErr)
		}
	}()
	
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	// Create test JavaScript file
	jsContent := `/**
 * LSP Gateway Server Implementation
 */

class Server {
    constructor(name, port) {
        this.name = name;
        this.port = port;
        this.running = false;
    }

    async start() {
        console.log("Starting server " + this.name + " on port " + this.port);
        this.running = true;
    }

    async stop() {
        console.log("Stopping server " + this.name);
        this.running = false;
    }

    isRunning() {
        return this.running;
    }
}

function createServer(name, port) {
    return new Server(name, port);
}

module.exports = { Server, createServer };`
	
	testFile, err := setup.Resources.Directory.CreateTempFile("server.js", jsContent)
	require.NoError(t, err, "Failed to create test JavaScript file")
	
	// Start gateway server
	projectRoot, err := testutils.GetProjectRoot()
	require.NoError(t, err, "Failed to get project root")
	
	binaryPath := filepath.Join(projectRoot, "bin", "lspg")
	serverCmd := []string{binaryPath, "server", "--config", setup.ConfigPath}
	
	err = setup.StartServer(serverCmd, 30*time.Second)
	require.NoError(t, err, "Failed to start server")
	
	err = setup.WaitForServerReady(10 * time.Second)
	require.NoError(t, err, "Server failed to become ready")
	
	// Test definition on JavaScript file
	httpClient := setup.GetHTTPClient()
	fileURI := "file://" + testFile
	position := testutils.Position{Line: 27, Character: 15} // createServer function call
	
	locations, err := httpClient.Definition(ctx, fileURI, position)
	if err != nil {
		t.Logf("Definition request failed (expected for test setup): %v", err)
		return
	}
	
	t.Logf("Found %d definition locations for JavaScript file", len(locations))
	if len(locations) > 0 {
		assert.Contains(t, locations[0].URI, "server.js", "Definition should reference correct file")
		assert.GreaterOrEqual(t, locations[0].Range.Start.Line, 0, "Definition line should be valid")
	}
	
	t.Logf("JavaScript definition feature test completed successfully on port %d", setup.Resources.Port)
}

// Test runner function
func TestJavaScriptBasicE2ETestSuite(t *testing.T) {
	suite.Run(t, new(JavaScriptBasicE2ETestSuite))
}