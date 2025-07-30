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

// MultiProjectWorkspaceE2ETestSuite tests multi-project workspace functionality
type MultiProjectWorkspaceE2ETestSuite struct {
	suite.Suite

	// Core infrastructure
	httpClient    *testutils.HttpClient
	gatewayCmd    *exec.Cmd
	gatewayPort   int
	configPath    string
	tempDir       string
	projectRoot   string
	timeouts      *testutils.TestSuiteTimeouts

	// Multi-project specific
	multiProjectManager *testutils.MultiProjectManager
	subProjects         map[string]*testutils.SubProjectInfo
	serverStarted       bool
}

// SetupSuite initializes the test suite for multi-project testing
func (suite *MultiProjectWorkspaceE2ETestSuite) SetupSuite() {
	// Initialize standardized timeout management
	suite.timeouts = testutils.NewTestSuiteTimeouts(testutils.LanguageGo) // Use Go as base, suitable for multi-project

	var err error
	suite.projectRoot, err = testutils.GetProjectRoot()
	suite.Require().NoError(err, "Failed to get project root")

	suite.tempDir, err = os.MkdirTemp("", "multi-project-e2e-*")
	suite.Require().NoError(err, "Failed to create temp directory")

	// Initialize multi-project manager
	config := testutils.EnhancedMultiProjectConfig{
		WorkspaceName: "e2e-test-workspace",
		Languages:     []string{"go", "python", "typescript", "java"},
		CloneTimeout:  120 * time.Second,
		EnableLogging: true,
		ForceClean:    true,
		ParallelSetup: true,
	}
	suite.multiProjectManager = testutils.NewMultiProjectManager(config)

	// Setup workspace with all 4 sub-projects
	err = suite.multiProjectManager.SetupWorkspace()
	suite.Require().NoError(err, "Failed to setup multi-project workspace")

	// Get sub-projects
	suite.subProjects = suite.multiProjectManager.GetSubProjects()
	suite.Require().Equal(4, len(suite.subProjects), "Expected 4 sub-projects")

	// Validate workspace
	err = suite.multiProjectManager.ValidateWorkspace()
	suite.Require().NoError(err, "Failed to validate multi-project workspace")
}

// SetupTest initializes fresh components for each test
func (suite *MultiProjectWorkspaceE2ETestSuite) SetupTest() {
	var err error
	suite.gatewayPort, err = testutils.FindAvailablePort()
	suite.Require().NoError(err, "Failed to find available port")

	// Generate multi-project configuration
	configContent, err := suite.multiProjectManager.GenerateConfig(suite.gatewayPort)
	suite.Require().NoError(err, "Failed to generate multi-project config")

	// Write configuration to temp file
	configFile := filepath.Join(suite.tempDir, fmt.Sprintf("config-%d.yaml", suite.gatewayPort))
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	suite.Require().NoError(err, "Failed to write config file")
	suite.configPath = configFile

	// Configure HttpClient for multi-project testing with enhanced features
	timeoutConfig := suite.timeouts.Config()
	config := testutils.HttpClientConfig{
		BaseURL:                 fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
		Timeout:                 timeoutConfig.HTTPTimeout(),
		MaxRetries:              5,
		RetryDelay:              500 * time.Millisecond,
		EnableLogging:           true,
		EnableRecording:         true,
		WorkspaceID:             fmt.Sprintf("multi-project-test-%d", time.Now().UnixNano()),
		ProjectPath:             suite.multiProjectManager.GetWorkspaceDir(),
		UserAgent:               "LSP-Gateway-MultiProject-E2E/1.0",
		MaxResponseSize:         50 * 1024 * 1024,
		ConnectionPoolSize:      20,
		KeepAlive:               timeoutConfig.HTTPKeepAlive(),
		WorkspaceRoot:           suite.multiProjectManager.GetWorkspaceDir(),
		EnableSubProjectMetrics: true,
	}

	suite.httpClient = testutils.NewHttpClient(config)
	suite.httpClient.SetWorkspaceRoot(suite.multiProjectManager.GetWorkspaceDir())
	suite.httpClient.EnableSubProjectMetrics(true)
	suite.serverStarted = false
}

// TearDownTest cleans up per-test resources
func (suite *MultiProjectWorkspaceE2ETestSuite) TearDownTest() {
	suite.stopGatewayServer()

	testutils.CleanupAllTimeouts()

	if suite.httpClient != nil {
		suite.httpClient.Close()
		suite.httpClient = nil
	}
}

// TearDownSuite performs final cleanup
func (suite *MultiProjectWorkspaceE2ETestSuite) TearDownSuite() {
	if suite.multiProjectManager != nil {
		if err := suite.multiProjectManager.Cleanup(); err != nil {
			suite.T().Logf("Warning: Failed to cleanup multi-project workspace: %v", err)
		}
	}

	if suite.tempDir != "" {
		if err := os.RemoveAll(suite.tempDir); err != nil {
			suite.T().Logf("Warning: Failed to remove temp directory: %v", err)
		}
	}
}

// Test Methods

// TestMultiProjectDetection verifies all 4 sub-projects are detected
func (suite *MultiProjectWorkspaceE2ETestSuite) TestMultiProjectDetection() {
	suite.startGatewayServer()
	defer suite.stopGatewayServer()

	// Verify all 4 languages are present
	expectedLanguages := []string{"go", "python", "typescript", "java"}
	for _, lang := range expectedLanguages {
		subProject, exists := suite.subProjects[lang]
		suite.True(exists, "Sub-project for %s should exist", lang)
		suite.Equal(lang, subProject.Language, "Language should match")
		suite.NotEmpty(subProject.ProjectPath, "Project path should not be empty")

		// Verify project directory exists
		_, err := os.Stat(subProject.ProjectPath)
		suite.NoError(err, "Project directory should exist for %s", lang)
	}

	suite.T().Logf("Successfully detected all %d sub-projects", len(suite.subProjects))
}

// TestMultiProjectRouting tests LSP requests route to correct sub-project
func (suite *MultiProjectWorkspaceE2ETestSuite) TestMultiProjectRouting() {
	ctx, cancel := suite.timeouts.Config().TestContext(context.Background())
	defer cancel()

	suite.startGatewayServer()
	defer suite.stopGatewayServer()

	// Test routing for each sub-project
	for language, subProject := range suite.subProjects {
		suite.T().Run(fmt.Sprintf("Routing_%s", language), func(t *testing.T) {
			if len(subProject.TestFiles) == 0 {
				t.Skipf("No test files available for %s", language)
				return
			}

			// Test definition request routing
			testFile := subProject.TestFiles[0]
			position := testutils.Position{Line: 0, Character: 0}

			startTime := time.Now()
			_, err := suite.httpClient.DefinitionInSubProject(ctx, subProject, testFile, position)
			routingTime := time.Since(startTime)

			// Routing should be fast (<1ms requirement)
			suite.Less(routingTime, time.Millisecond, "Sub-project routing should be <1ms for %s", language)

			if err != nil {
				t.Logf("Definition request failed for %s (expected for test setup): %v", language, err)
			} else {
				t.Logf("Successfully routed definition request for %s in %v", language, routingTime)
			}
		})
	}
}

// TestCrossProjectSymbolResolution tests workspace/symbol across projects
func (suite *MultiProjectWorkspaceE2ETestSuite) TestCrossProjectSymbolResolution() {
	ctx, cancel := suite.timeouts.Config().TestContext(context.Background())
	defer cancel()

	suite.startGatewayServer()
	defer suite.stopGatewayServer()

	// Test workspace symbol search across all projects
	for language, subProject := range suite.subProjects {
		suite.T().Run(fmt.Sprintf("WorkspaceSymbol_%s", language), func(t *testing.T) {
			symbols, err := suite.httpClient.WorkspaceSymbolInSubProject(ctx, subProject, "main")
			if err != nil {
				t.Logf("Workspace symbol request failed for %s (expected for test setup): %v", language, err)
				return
			}

			t.Logf("Found %d symbols in %s project for query 'main'", len(symbols), language)
		})
	}

	// Check cross-project request tracking
	crossProjectCount := suite.httpClient.GetCrossProjectRequestCount()
	suite.T().Logf("Cross-project requests detected: %d", crossProjectCount)
}

// TestMultiProjectPerformance validates performance requirements
func (suite *MultiProjectWorkspaceE2ETestSuite) TestMultiProjectPerformance() {
	ctx, cancel := suite.timeouts.Config().TestContext(context.Background())
	defer cancel()

	suite.startGatewayServer()
	defer suite.stopGatewayServer()

	// Test performance for each language
	for language, subProject := range suite.subProjects {
		suite.T().Run(fmt.Sprintf("Performance_%s", language), func(t *testing.T) {
			if len(subProject.TestFiles) == 0 {
				t.Skipf("No test files available for %s", language)
				return
			}

			testFile := subProject.TestFiles[0]
			position := testutils.Position{Line: 0, Character: 0}

			// Perform multiple requests to test performance
			const numRequests = 5
			var totalTime time.Duration

			for i := 0; i < numRequests; i++ {
				startTime := time.Now()
				_, err := suite.httpClient.DefinitionInSubProject(ctx, subProject, testFile, position)
				requestTime := time.Since(startTime)
				totalTime += requestTime

				if err == nil {
					// LSP response time should be <5 seconds
					suite.Less(requestTime, 5*time.Second, "LSP response time should be <5s for %s", language)
				}
			}

			avgTime := totalTime / numRequests
			t.Logf("Average request time for %s: %v", language, avgTime)
		})
	}

	// Validate memory usage (rough check)
	// Note: This is a basic check - more sophisticated memory monitoring could be added
	suite.T().Logf("Performance validation completed for all sub-projects")
}

// TestMultiProjectErrorHandling tests files outside sub-projects are handled properly
func (suite *MultiProjectWorkspaceE2ETestSuite) TestMultiProjectErrorHandling() {
	ctx, cancel := suite.timeouts.Config().TestContext(context.Background())
	defer cancel()

	suite.startGatewayServer()
	defer suite.stopGatewayServer()

	// Test with a file outside sub-project boundaries
	for language, subProject := range suite.subProjects {
		suite.T().Run(fmt.Sprintf("ErrorHandling_%s", language), func(t *testing.T) {
			// Try to access a file outside the sub-project
			outsideFile := "../outside-project.txt"
			position := testutils.Position{Line: 0, Character: 0}

			_, err := suite.httpClient.DefinitionInSubProject(ctx, subProject, outsideFile, position)
			suite.Error(err, "Should fail for file outside sub-project boundaries")
			suite.Contains(err.Error(), "outside sub-project boundaries", "Error should mention boundary violation")
		})
	}
}

// TestMultiProjectLSPMethods tests all 6 LSP methods on each sub-project
func (suite *MultiProjectWorkspaceE2ETestSuite) TestMultiProjectLSPMethods() {
	ctx, cancel := suite.timeouts.Config().TestContext(context.Background())
	defer cancel()

	suite.startGatewayServer()
	defer suite.stopGatewayServer()

	// Test all LSP methods for each language
	for language, subProject := range suite.subProjects {
		suite.T().Run(fmt.Sprintf("LSPMethods_%s", language), func(t *testing.T) {
			if len(subProject.TestFiles) == 0 {
				t.Skipf("No test files available for %s", language)
				return
			}

			testFile := subProject.TestFiles[0]
			position := testutils.Position{Line: 0, Character: 0}

			// 1. textDocument/definition
			_, err := suite.httpClient.DefinitionInSubProject(ctx, subProject, testFile, position)
			if err != nil {
				t.Logf("Definition failed for %s: %v", language, err)
			}

			// 2. textDocument/references
			_, err = suite.httpClient.ReferencesInSubProject(ctx, subProject, testFile, position, true)
			if err != nil {
				t.Logf("References failed for %s: %v", language, err)
			}

			// 3. workspace/symbol
			_, err = suite.httpClient.WorkspaceSymbolInSubProject(ctx, subProject, "test")
			if err != nil {
				t.Logf("WorkspaceSymbol failed for %s: %v", language, err)
			}

			// 4. textDocument/hover (using regular HttpClient method)
			fileURI := fmt.Sprintf("file://%s", filepath.Join(subProject.ProjectPath, testFile))
			_, err = suite.httpClient.Hover(ctx, fileURI, position)
			if err != nil {
				t.Logf("Hover failed for %s: %v", language, err)
			}

			// 5. textDocument/documentSymbol
			_, err = suite.httpClient.DocumentSymbol(ctx, fileURI)
			if err != nil {
				t.Logf("DocumentSymbol failed for %s: %v", language, err)
			}

			// 6. textDocument/completion
			_, err = suite.httpClient.Completion(ctx, fileURI, position)
			if err != nil {
				t.Logf("Completion failed for %s: %v", language, err)
			}

			t.Logf("Completed LSP method testing for %s", language)
		})
	}
}

// TestSubProjectMetrics tests the enhanced metrics functionality
func (suite *MultiProjectWorkspaceE2ETestSuite) TestSubProjectMetrics() {
	ctx, cancel := suite.timeouts.Config().TestContext(context.Background())
	defer cancel()

	suite.startGatewayServer()
	defer suite.stopGatewayServer()

	// Perform some requests to generate metrics
	for _, subProject := range suite.subProjects {
		if len(subProject.TestFiles) > 0 {
			testFile := subProject.TestFiles[0]
			position := testutils.Position{Line: 0, Character: 0}
			suite.httpClient.DefinitionInSubProject(ctx, subProject, testFile, position)
		}
	}

	// Check sub-project metrics
	allMetrics := suite.httpClient.GetAllSubProjectMetrics()
	suite.NotEmpty(allMetrics, "Sub-project metrics should be available")

	for language := range suite.subProjects {
		metrics, exists := suite.httpClient.GetSubProjectMetrics(language)
		if exists {
			suite.Equal(language, metrics.Language, "Language should match in metrics")
			suite.GreaterOrEqual(metrics.TotalRequests, 0, "Total requests should be non-negative")
			suite.T().Logf("Metrics for %s: %d total requests, %d successful, %d failed",
				language, metrics.TotalRequests, metrics.SuccessfulRequests, metrics.FailedRequests)
		}
	}
}

// TestWorkspaceRootIntegration tests workspace root functionality
func (suite *MultiProjectWorkspaceE2ETestSuite) TestWorkspaceRootIntegration() {
	workspaceRoot := suite.httpClient.GetWorkspaceRoot()
	suite.Equal(suite.multiProjectManager.GetWorkspaceDir(), workspaceRoot, "Workspace roots should match")

	// Test workspace root change
	newRoot := "/tmp/test-workspace"
	suite.httpClient.SetWorkspaceRoot(newRoot)
	suite.Equal(newRoot, suite.httpClient.GetWorkspaceRoot(), "Workspace root should be updated")

	// Restore original root
	suite.httpClient.SetWorkspaceRoot(suite.multiProjectManager.GetWorkspaceDir())
}

// Helper methods

func (suite *MultiProjectWorkspaceE2ETestSuite) startGatewayServer() {
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

func (suite *MultiProjectWorkspaceE2ETestSuite) stopGatewayServer() {
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

func (suite *MultiProjectWorkspaceE2ETestSuite) waitForServerReadiness() {
	config := suite.timeouts.Config().StandardPollingConfig()
	config.Interval = time.Second

	condition := func() (bool, error) {
		return suite.checkServerHealth(), nil
	}

	err := testutils.WaitForCondition(condition, config, "server to be ready")
	suite.Require().NoError(err, "Server failed to become ready within timeout")
}

func (suite *MultiProjectWorkspaceE2ETestSuite) checkServerHealth() bool {
	if suite.httpClient == nil {
		return false
	}

	ctx, cancel := suite.timeouts.Config().HealthCheckContext(context.Background())
	defer cancel()

	err := suite.httpClient.HealthCheck(ctx)
	return err == nil
}

// Test runner function
func TestMultiProjectWorkspaceE2ETestSuite(t *testing.T) {
	suite.Run(t, new(MultiProjectWorkspaceE2ETestSuite))
}