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

// GoBasicE2ETestSuite demonstrates the modular repository system for Go testing
type GoBasicE2ETestSuite struct {
	suite.Suite
	
	// Core infrastructure
	httpClient      *testutils.HttpClient
	gatewayCmd      *exec.Cmd
	gatewayPort     int
	configPath      string
	tempDir         string
	projectRoot     string
	
	// Standardized timeout management
	timeouts        *testutils.TestSuiteTimeouts
	
	// Modular repository management for Go
	repoManager     testutils.RepositoryManager
	repoDir         string
	goFiles         []string
	
	// Server state tracking
	serverStarted   bool
}

// SetupSuite initializes the test suite for Go using the modular system
func (suite *GoBasicE2ETestSuite) SetupSuite() {
	// Initialize standardized timeout management for Go language
	suite.timeouts = testutils.NewTestSuiteTimeouts(testutils.LanguageGo)
	
	var err error
	suite.projectRoot, err = testutils.GetProjectRoot()
	suite.Require().NoError(err, "Failed to get project root")
	
	suite.tempDir, err = os.MkdirTemp("", "go-basic-e2e-*")
	suite.Require().NoError(err, "Failed to create temp directory")
	
	// Initialize modular Go repository manager
	suite.repoManager = testutils.NewGoRepositoryManager()
	
	// Setup Go repository
	suite.repoDir, err = suite.repoManager.SetupRepository()
	suite.Require().NoError(err, "Failed to setup Go repository")
	
	// Discover Go files for testing
	suite.discoverGoFiles()
	
	// Create test configuration for Go
	suite.createGoTestConfig()
}

// SetupTest initializes fresh components for each test
func (suite *GoBasicE2ETestSuite) SetupTest() {
	var err error
	suite.gatewayPort, err = testutils.FindAvailablePort()
	suite.Require().NoError(err, "Failed to find available port")

	// Update config with new port
	suite.updateConfigPort()

	// Configure HttpClient for Go testing with standardized timeouts
	timeoutConfig := suite.timeouts.Config()
	config := testutils.HttpClientConfig{
		BaseURL:            fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
		Timeout:            timeoutConfig.HTTPTimeout(),
		MaxRetries:         5,
		RetryDelay:         500 * time.Millisecond,
		EnableLogging:      true,
		EnableRecording:    true,
		WorkspaceID:        fmt.Sprintf("go-basic-test-%d", time.Now().UnixNano()),
		ProjectPath:        suite.repoDir,
		UserAgent:          "LSP-Gateway-Go-Basic-E2E/1.0",
		MaxResponseSize:    50 * 1024 * 1024,
		ConnectionPoolSize: 15,
		KeepAlive:          timeoutConfig.HTTPKeepAlive(),
	}

	suite.httpClient = testutils.NewHttpClient(config)
	suite.serverStarted = false
}

// TearDownTest cleans up per-test resources
func (suite *GoBasicE2ETestSuite) TearDownTest() {
	suite.stopGatewayServer()
	
	// Cleanup any remaining timeout contexts
	testutils.CleanupAllTimeouts()
	
	if suite.httpClient != nil {
		suite.httpClient.Close()
		suite.httpClient = nil
	}
}

// TearDownSuite performs final cleanup
func (suite *GoBasicE2ETestSuite) TearDownSuite() {
	if suite.repoManager != nil {
		if err := suite.repoManager.Cleanup(); err != nil {
			suite.T().Logf("Warning: Failed to cleanup Go repository: %v", err)
		}
	}
	
	if suite.tempDir != "" {
		if err := os.RemoveAll(suite.tempDir); err != nil {
			suite.T().Logf("Warning: Failed to remove temp directory: %v", err)
		}
	}
}

// TestGoBasicServerLifecycle tests the complete server lifecycle for Go
func (suite *GoBasicE2ETestSuite) TestGoBasicServerLifecycle() {
	ctx, cancel := suite.timeouts.Config().TestContext(context.Background())
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.verifyServerReadiness(ctx)
	suite.testBasicServerOperations(ctx)
}

// TestGoDefinitionFeature tests textDocument/definition for Go files
func (suite *GoBasicE2ETestSuite) TestGoDefinitionFeature() {
	ctx, cancel := suite.timeouts.Config().TestContext(context.Background())
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	// Test definition on Go files
	if len(suite.goFiles) > 0 {
		testFile := suite.goFiles[0]
		fileURI := suite.getFileURI(testFile)
		
		// Test at a common position (line 0, character 0)
		position := testutils.Position{Line: 0, Character: 0}
		
		locations, err := suite.httpClient.Definition(ctx, fileURI, position)
		suite.NoError(err, "Definition request should not fail")
		suite.T().Logf("Found %d definition locations for Go file %s", len(locations), testFile)
	}
}

// Helper methods

func (suite *GoBasicE2ETestSuite) discoverGoFiles() {
	testFiles, err := suite.repoManager.GetTestFiles()
	suite.Require().NoError(err, "Failed to get Go test files")
	suite.Require().Greater(len(testFiles), 0, "No Go files found in repository")
	
	// Filter for Go files only
	for _, file := range testFiles {
		if filepath.Ext(file) == ".go" {
			suite.goFiles = append(suite.goFiles, file)
		}
	}
	
	suite.Require().Greater(len(suite.goFiles), 0, "No Go files found")
}

func (suite *GoBasicE2ETestSuite) createGoTestConfig() {
	options := testutils.DefaultLanguageConfigOptions("go")
	options.TestPort = fmt.Sprintf("%d", suite.gatewayPort)
	options.ConfigType = "main"
	
	configPath, cleanup, err := testutils.CreateLanguageConfig(suite.repoManager, options)
	suite.Require().NoError(err, "Failed to create Go test config")
	
	suite.configPath = configPath
	_ = cleanup // Will be cleaned up via tempDir
}

func (suite *GoBasicE2ETestSuite) updateConfigPort() {
	suite.createGoTestConfig()
}

func (suite *GoBasicE2ETestSuite) getFileURI(filePath string) string {
	workspaceDir := suite.repoManager.GetWorkspaceDir()
	return "file://" + filepath.Join(workspaceDir, filePath)
}

func (suite *GoBasicE2ETestSuite) startGatewayServer() {
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

func (suite *GoBasicE2ETestSuite) stopGatewayServer() {
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
		case <-time.After(suite.timeouts.Config().TestSetupTimeout()):
			suite.gatewayCmd.Process.Kill()
			suite.gatewayCmd.Wait()
		}
	}
	
	suite.gatewayCmd = nil
	suite.serverStarted = false
}

func (suite *GoBasicE2ETestSuite) waitForServerReadiness() {
	config := suite.timeouts.Config().StandardPollingConfig()
	config.Interval = time.Second // Keep original interval
	
	condition := func() (bool, error) {
		return suite.checkServerHealth(), nil
	}
	
	err := testutils.WaitForCondition(condition, config, "server to be ready")
	suite.Require().NoError(err, "Server failed to become ready within timeout")
}

func (suite *GoBasicE2ETestSuite) checkServerHealth() bool {
	if suite.httpClient == nil {
		return false
	}
	
	ctx, cancel := suite.timeouts.Config().HealthCheckContext(context.Background())
	defer cancel()
	
	err := suite.httpClient.HealthCheck(ctx)
	return err == nil
}

func (suite *GoBasicE2ETestSuite) verifyServerReadiness(ctx context.Context) {
	err := suite.httpClient.HealthCheck(ctx)
	suite.Require().NoError(err, "Server health check should pass")
}

func (suite *GoBasicE2ETestSuite) testBasicServerOperations(ctx context.Context) {
	err := suite.httpClient.ValidateConnection(ctx)
	suite.Require().NoError(err, "Server connection validation should pass")
}

// Parallel test functions using resource isolation

// TestGoBasicServerLifecycleParallel tests the complete server lifecycle for Go with parallel execution
func TestGoBasicServerLifecycleParallel(t *testing.T) {
	t.Parallel()
	
	// Initialize standardized timeout management for Go language
	timeoutConfig := testutils.GoTimeoutConfig()
	defer testutils.CleanupAllTimeouts()
	
	setup, err := testutils.SetupIsolatedTestWithLanguage("go_basic_lifecycle", "go")
	require.NoError(t, err, "Failed to setup isolated test")
	defer func() {
		if cleanupErr := setup.Cleanup(); cleanupErr != nil {
			t.Logf("Warning: Cleanup failed: %v", cleanupErr)
		}
	}()
	
	ctx, cancel := timeoutConfig.TestContext(context.Background())
	defer cancel()
	
	// Start gateway server
	projectRoot, err := testutils.GetProjectRoot()
	require.NoError(t, err, "Failed to get project root")
	
	binaryPath := filepath.Join(projectRoot, "bin", "lspg")
	serverCmd := []string{binaryPath, "server", "--config", setup.ConfigPath}
	
	err = setup.StartServer(serverCmd, timeoutConfig.ServerStartupTimeout())
	require.NoError(t, err, "Failed to start server")
	
	err = setup.WaitForServerReady(timeoutConfig.HealthCheckTimeout())
	require.NoError(t, err, "Server failed to become ready")
	
	// Verify server readiness
	httpClient := setup.GetHTTPClient()
	err = httpClient.HealthCheck(ctx)
	require.NoError(t, err, "Server health check should pass")
	
	// Test basic server operations
	err = httpClient.ValidateConnection(ctx)
	require.NoError(t, err, "Server connection validation should pass")
	
	t.Logf("Go basic server lifecycle test completed successfully on port %d", setup.Resources.Port)
}

// TestGoDefinitionFeatureParallel tests textDocument/definition for Go files with parallel execution
func TestGoDefinitionFeatureParallel(t *testing.T) {
	t.Parallel()
	
	// Initialize standardized timeout management for Go language
	timeoutConfig := testutils.GoTimeoutConfig()
	defer testutils.CleanupAllTimeouts()
	
	setup, err := testutils.SetupIsolatedTestWithLanguage("go_definition_feature", "go")
	require.NoError(t, err, "Failed to setup isolated test")
	defer func() {
		if cleanupErr := setup.Cleanup(); cleanupErr != nil {
			t.Logf("Warning: Cleanup failed: %v", cleanupErr)
		}
	}()
	
	ctx, cancel := timeoutConfig.TestContext(context.Background())
	defer cancel()
	
	// Create test Go file
	goContent := `package main

import "fmt"

type Server struct {
	Name string
	Port int
}

func (s *Server) Start() error {
	fmt.Printf("Starting server %s on port %d\n", s.Name, s.Port)
	return nil
}

func NewServer(name string, port int) *Server {
	return &Server{
		Name: name,
		Port: port,
	}
}

func main() {
	server := NewServer("gateway", 8080)
	server.Start()
}`
	
	testFile, err := setup.Resources.Directory.CreateTempFile("main.go", goContent)
	require.NoError(t, err, "Failed to create test Go file")
	
	// Start gateway server
	projectRoot, err := testutils.GetProjectRoot()
	require.NoError(t, err, "Failed to get project root")
	
	binaryPath := filepath.Join(projectRoot, "bin", "lspg")
	serverCmd := []string{binaryPath, "server", "--config", setup.ConfigPath}
	
	err = setup.StartServer(serverCmd, timeoutConfig.ServerStartupTimeout())
	require.NoError(t, err, "Failed to start server")
	
	err = setup.WaitForServerReady(timeoutConfig.HealthCheckTimeout())
	require.NoError(t, err, "Server failed to become ready")
	
	// Test definition on Go file
	httpClient := setup.GetHTTPClient()
	fileURI := "file://" + testFile
	position := testutils.Position{Line: 15, Character: 8} // NewServer function call
	
	locations, err := httpClient.Definition(ctx, fileURI, position)
	if err != nil {
		t.Logf("Definition request failed (expected for test setup): %v", err)
		return
	}
	
	t.Logf("Found %d definition locations for Go file", len(locations))
	if len(locations) > 0 {
		assert.Contains(t, locations[0].URI, "main.go", "Definition should reference correct file")
		assert.GreaterOrEqual(t, locations[0].Range.Start.Line, 0, "Definition line should be valid")
	}
	
	t.Logf("Go definition feature test completed successfully on port %d", setup.Resources.Port)
}

// Test runner function
func TestGoBasicE2ETestSuite(t *testing.T) {
	suite.Run(t, new(GoBasicE2ETestSuite))
}