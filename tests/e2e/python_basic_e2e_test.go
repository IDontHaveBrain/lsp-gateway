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

	"github.com/stretchr/testify/suite"
	"lsp-gateway/tests/e2e/testutils"
)

// PythonBasicE2ETestSuite demonstrates the modular repository system for Python testing
type PythonBasicE2ETestSuite struct {
	suite.Suite
	
	// Core infrastructure
	httpClient      *testutils.HttpClient
	gatewayCmd      *exec.Cmd
	gatewayPort     int
	configPath      string
	tempDir         string
	projectRoot     string
	
	// Standardized timeout management for Python (longer timeouts than Go)
	timeouts        *testutils.TestSuiteTimeouts
	
	// Modular repository management for Python
	repoManager     testutils.RepositoryManager
	repoDir         string
	pyFiles         []string
	
	// Server state tracking
	serverStarted   bool
}

// SetupSuite initializes the test suite for Python using the modular system
func (suite *PythonBasicE2ETestSuite) SetupSuite() {
	// Initialize standardized timeout management for Python language
	// Python timeouts are longer than Go due to dependency loading and slower startup
	suite.timeouts = testutils.NewTestSuiteTimeouts(testutils.LanguagePython)
	
	var err error
	suite.projectRoot, err = testutils.GetProjectRoot()
	suite.Require().NoError(err, "Failed to get project root")
	
	suite.tempDir, err = os.MkdirTemp("", "py-basic-e2e-*")
	suite.Require().NoError(err, "Failed to create temp directory")
	
	// Initialize modular Python repository manager
	suite.repoManager = testutils.NewPythonRepositoryManager()
	
	// Setup Python repository
	suite.repoDir, err = suite.repoManager.SetupRepository()
	suite.Require().NoError(err, "Failed to setup Python repository")
	
	// Discover Python files for testing
	suite.discoverPythonFiles()
	
	// Create test configuration for Python
	suite.createPythonTestConfig()
}

// SetupTest initializes fresh components for each test
func (suite *PythonBasicE2ETestSuite) SetupTest() {
	var err error
	suite.gatewayPort, err = testutils.FindAvailablePort()
	suite.Require().NoError(err, "Failed to find available port")

	// Update config with new port
	suite.updateConfigPort()

	// Configure HttpClient for Python testing with language-specific timeouts
	// Python requires longer timeouts due to dependency loading and slower LSP server startup
	timeoutConfig := suite.timeouts.Config()
	config := testutils.HttpClientConfig{
		BaseURL:            fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
		Timeout:            timeoutConfig.HTTPTimeout(),
		MaxRetries:         5,
		RetryDelay:         500 * time.Millisecond,
		EnableLogging:      true,
		EnableRecording:    true,
		WorkspaceID:        fmt.Sprintf("py-basic-test-%d", time.Now().UnixNano()),
		ProjectPath:        suite.repoDir,
		UserAgent:          "LSP-Gateway-Python-Basic-E2E/1.0",
		MaxResponseSize:    50 * 1024 * 1024,
		ConnectionPoolSize: 15,
		KeepAlive:          timeoutConfig.HTTPKeepAlive(),
	}

	suite.httpClient = testutils.NewHttpClient(config)
	suite.serverStarted = false
}

// TearDownTest cleans up per-test resources
func (suite *PythonBasicE2ETestSuite) TearDownTest() {
	suite.stopGatewayServer()
	
	// Cleanup any remaining timeout contexts
	testutils.CleanupAllTimeouts()
	
	if suite.httpClient != nil {
		suite.httpClient.Close()
		suite.httpClient = nil
	}
}

// TearDownSuite performs final cleanup
func (suite *PythonBasicE2ETestSuite) TearDownSuite() {
	if suite.repoManager != nil {
		if err := suite.repoManager.Cleanup(); err != nil {
			suite.T().Logf("Warning: Failed to cleanup Python repository: %v", err)
		}
	}
	
	if suite.tempDir != "" {
		if err := os.RemoveAll(suite.tempDir); err != nil {
			suite.T().Logf("Warning: Failed to remove temp directory: %v", err)
		}
	}
}

// TestPythonBasicServerLifecycle tests the complete server lifecycle for Python
func (suite *PythonBasicE2ETestSuite) TestPythonBasicServerLifecycle() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.timeouts.Config().TestMethodTimeout())
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.verifyServerReadiness(ctx)
	suite.testBasicServerOperations(ctx)
}

// TestPythonDefinitionFeature tests textDocument/definition for Python files
func (suite *PythonBasicE2ETestSuite) TestPythonDefinitionFeature() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.timeouts.Config().TestMethodTimeout())
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	// Test definition on Python files
	if len(suite.pyFiles) > 0 {
		testFile := suite.pyFiles[0]
		fileURI := suite.getFileURI(testFile)
		
		// Test at a common position (line 0, character 0)
		position := testutils.Position{Line: 0, Character: 0}
		
		locations, err := suite.httpClient.Definition(ctx, fileURI, position)
		suite.NoError(err, "Definition request should not fail")
		suite.T().Logf("Found %d definition locations for Python file %s", len(locations), testFile)
	}
}

// Helper methods demonstrating modular system usage

func (suite *PythonBasicE2ETestSuite) discoverPythonFiles() {
	testFiles, err := suite.repoManager.GetTestFiles()
	suite.Require().NoError(err, "Failed to get Python test files")
	suite.Require().Greater(len(testFiles), 0, "No Python files found in repository")
	
	// Filter for Python files
	for _, file := range testFiles {
		ext := filepath.Ext(file)
		if ext == ".py" {
			suite.pyFiles = append(suite.pyFiles, file)
		}
	}
	
	suite.Require().Greater(len(suite.pyFiles), 0, "No Python files found")
}

func (suite *PythonBasicE2ETestSuite) createPythonTestConfig() {
	// Demonstrate custom configuration for Python
	options := testutils.DefaultLanguageConfigOptions("python")
	options.TestPort = fmt.Sprintf("%d", suite.gatewayPort)
	options.ConfigType = "main"
	
	// Add Python-specific custom variables
	options.CustomVariables["PYTHONPATH"] = suite.repoDir
	options.CustomVariables["PYTHON_LSP_SERVER"] = "pylsp"
	
	configPath, cleanup, err := testutils.CreateLanguageConfig(suite.repoManager, options)
	suite.Require().NoError(err, "Failed to create Python test config")
	
	suite.configPath = configPath
	_ = cleanup // Will be cleaned up via tempDir
}

func (suite *PythonBasicE2ETestSuite) updateConfigPort() {
	suite.createPythonTestConfig()
}

func (suite *PythonBasicE2ETestSuite) getFileURI(filePath string) string {
	workspaceDir := suite.repoManager.GetWorkspaceDir()
	return "file://" + filepath.Join(workspaceDir, filePath)
}

func (suite *PythonBasicE2ETestSuite) startGatewayServer() {
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

func (suite *PythonBasicE2ETestSuite) stopGatewayServer() {
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

func (suite *PythonBasicE2ETestSuite) waitForServerReadiness() {
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		if suite.checkServerHealth() {
			return
		}
		time.Sleep(time.Second)
	}
	suite.Require().Fail("Server failed to become ready within timeout")
}

func (suite *PythonBasicE2ETestSuite) checkServerHealth() bool {
	if suite.httpClient == nil {
		return false
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err := suite.httpClient.HealthCheck(ctx)
	return err == nil
}

func (suite *PythonBasicE2ETestSuite) verifyServerReadiness(ctx context.Context) {
	err := suite.httpClient.HealthCheck(ctx)
	suite.Require().NoError(err, "Server health check should pass")
}

func (suite *PythonBasicE2ETestSuite) testBasicServerOperations(ctx context.Context) {
	err := suite.httpClient.ValidateConnection(ctx)
	suite.Require().NoError(err, "Server connection validation should pass")
}

// Test runner function
func TestPythonBasicE2ETestSuite(t *testing.T) {
	suite.Run(t, new(PythonBasicE2ETestSuite))
}
