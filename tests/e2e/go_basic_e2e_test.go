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
	testTimeout     time.Duration
	
	// Modular repository management for Go
	repoManager     testutils.RepositoryManager
	repoDir         string
	goFiles         []string
	
	// Server state tracking
	serverStarted   bool
}

// SetupSuite initializes the test suite for Go using the modular system
func (suite *GoBasicE2ETestSuite) SetupSuite() {
	suite.testTimeout = 5 * time.Minute
	
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

	// Configure HttpClient for Go testing
	config := testutils.HttpClientConfig{
		BaseURL:            fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
		Timeout:            30 * time.Second,
		MaxRetries:         5,
		RetryDelay:         2 * time.Second,
		EnableLogging:      true,
		EnableRecording:    true,
		WorkspaceID:        fmt.Sprintf("go-basic-test-%d", time.Now().UnixNano()),
		ProjectPath:        suite.repoDir,
		UserAgent:          "LSP-Gateway-Go-Basic-E2E/1.0",
		MaxResponseSize:    50 * 1024 * 1024,
		ConnectionPoolSize: 15,
		KeepAlive:          60 * time.Second,
	}

	suite.httpClient = testutils.NewHttpClient(config)
	suite.serverStarted = false
}

// TearDownTest cleans up per-test resources
func (suite *GoBasicE2ETestSuite) TearDownTest() {
	suite.stopGatewayServer()
	
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
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.verifyServerReadiness(ctx)
	suite.testBasicServerOperations(ctx)
}

// TestGoDefinitionFeature tests textDocument/definition for Go files
func (suite *GoBasicE2ETestSuite) TestGoDefinitionFeature() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
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
		case <-time.After(10 * time.Second):
			suite.gatewayCmd.Process.Kill()
			suite.gatewayCmd.Wait()
		}
	}
	
	suite.gatewayCmd = nil
	suite.serverStarted = false
}

func (suite *GoBasicE2ETestSuite) waitForServerReadiness() {
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		if suite.checkServerHealth() {
			return
		}
		time.Sleep(time.Second)
	}
	suite.Require().Fail("Server failed to become ready within timeout")
}

func (suite *GoBasicE2ETestSuite) checkServerHealth() bool {
	if suite.httpClient == nil {
		return false
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

// Test runner function
func TestGoBasicE2ETestSuite(t *testing.T) {
	suite.Run(t, new(GoBasicE2ETestSuite))
}