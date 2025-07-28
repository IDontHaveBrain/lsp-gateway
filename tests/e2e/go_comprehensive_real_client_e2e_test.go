package e2e_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/tests/e2e/testutils"
)

// GoComprehensiveRealClientE2ETestSuite provides comprehensive testing
// of all 6 supported LSP methods using real golang/example repository
type GoComprehensiveRealClientE2ETestSuite struct {
	suite.Suite
	
	// Core infrastructure
	httpClient      *testutils.HttpClient
	gatewayCmd      *exec.Cmd
	gatewayPort     int
	configPath      string
	tempDir         string
	projectRoot     string
	testTimeout     time.Duration
	
	// Repository management using testutils
	repoManager     testutils.RepositoryManager
	repoDir         string
	goFiles         []string
	
	// Server state
	serverStarted   bool
	
	// Test results tracking
	testResults     map[string]bool
	resultsMutex    sync.RWMutex
}

func (suite *GoComprehensiveRealClientE2ETestSuite) SetupSuite() {
	suite.testTimeout = 15 * time.Second
	suite.testResults = make(map[string]bool)
	
	var err error
	suite.projectRoot, err = testutils.GetProjectRoot()
	suite.Require().NoError(err, "Failed to get project root")
	
	suite.tempDir, err = os.MkdirTemp("", "go-comprehensive-e2e-*")
	suite.Require().NoError(err, "Failed to create temp directory")
	
	// Initialize Go repository manager with golang/example
	suite.repoManager = testutils.NewGoRepositoryManager()
	
	// Setup golang/example repository
	suite.repoDir, err = suite.repoManager.SetupRepository()
	suite.Require().NoError(err, "Failed to setup golang/example repository")
	
	// Discover Go files for comprehensive testing
	suite.discoverGoFiles()
	suite.createGoTestConfig()
	
	suite.T().Logf("Successfully setup golang/example repository with %d Go files", len(suite.goFiles))
}

func (suite *GoComprehensiveRealClientE2ETestSuite) SetupTest() {
	var err error
	suite.gatewayPort, err = testutils.FindAvailablePort()
	suite.Require().NoError(err, "Failed to find available port")

	suite.updateConfigPort()

	// Configure HttpClient for comprehensive testing
	config := testutils.HttpClientConfig{
		BaseURL:            fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
		Timeout:            5 * time.Second,
		MaxRetries:         5,
		RetryDelay:         500 * time.Millisecond,
		EnableLogging:      true,
		EnableRecording:    true,
		WorkspaceID:        fmt.Sprintf("go-comprehensive-%d", time.Now().UnixNano()),
		ProjectPath:        suite.repoDir,
		UserAgent:          "LSP-Gateway-Go-Comprehensive-E2E/1.0",
		MaxResponseSize:    100 * 1024 * 1024, // 100MB for large responses
		ConnectionPoolSize: 20,
		KeepAlive:          30 * time.Second,
	}

	suite.httpClient = testutils.NewHttpClient(config)
	suite.serverStarted = false
}

func (suite *GoComprehensiveRealClientE2ETestSuite) TearDownTest() {
	suite.stopGatewayServer()
	
	if suite.httpClient != nil {
		suite.httpClient.Close()
		suite.httpClient = nil
	}
}

func (suite *GoComprehensiveRealClientE2ETestSuite) TearDownSuite() {
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
	
	// Log test results summary
	suite.logTestSummary()
}

// TestGoAllLSPMethods tests all 6 supported LSP methods comprehensively
func (suite *GoComprehensiveRealClientE2ETestSuite) TestGoAllLSPMethods() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.verifyServerReadiness(ctx)
	
	// Test all LSP methods on multiple Go files
	suite.testDefinitionOnAllFiles(ctx)
	suite.testReferencesOnAllFiles(ctx)
	suite.testHoverOnAllFiles(ctx)
	suite.testDocumentSymbolOnAllFiles(ctx)
	suite.testWorkspaceSymbol(ctx)
	suite.testCompletionOnAllFiles(ctx)
}

// TestGoDefinitionComprehensive tests textDocument/definition extensively
func (suite *GoComprehensiveRealClientE2ETestSuite) TestGoDefinitionComprehensive() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.testDefinitionOnAllFiles(ctx)
}

// TestGoReferencesComprehensive tests textDocument/references extensively
func (suite *GoComprehensiveRealClientE2ETestSuite) TestGoReferencesComprehensive() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.testReferencesOnAllFiles(ctx)
}

// TestGoHoverComprehensive tests textDocument/hover extensively
func (suite *GoComprehensiveRealClientE2ETestSuite) TestGoHoverComprehensive() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.testHoverOnAllFiles(ctx)
}

// TestGoDocumentSymbolComprehensive tests textDocument/documentSymbol extensively
func (suite *GoComprehensiveRealClientE2ETestSuite) TestGoDocumentSymbolComprehensive() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.testDocumentSymbolOnAllFiles(ctx)
}

// TestGoWorkspaceSymbolComprehensive tests workspace/symbol extensively
func (suite *GoComprehensiveRealClientE2ETestSuite) TestGoWorkspaceSymbolComprehensive() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.testWorkspaceSymbol(ctx)
}

// TestGoCompletionComprehensive tests textDocument/completion extensively
func (suite *GoComprehensiveRealClientE2ETestSuite) TestGoCompletionComprehensive() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.testCompletionOnAllFiles(ctx)
}

// TestGoConcurrentRequests tests multiple concurrent LSP requests
func (suite *GoComprehensiveRealClientE2ETestSuite) TestGoConcurrentRequests() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.testConcurrentLSPRequests(ctx)
}

// LSP method implementations

func (suite *GoComprehensiveRealClientE2ETestSuite) testDefinitionOnAllFiles(ctx context.Context) {
	suite.T().Log("Testing textDocument/definition on all Go files...")
	successCount := 0
	
	for i, testFile := range suite.goFiles {
		if i >= 10 { // Limit to first 10 files for performance
			break
		}
		
		fileURI := suite.getFileURI(testFile)
		positions := []testutils.Position{
			{Line: 0, Character: 0},   // Start of file
			{Line: 5, Character: 10},  // Mid file
			{Line: 10, Character: 5},  // Another position
		}
		
		for _, position := range positions {
			locations, err := suite.httpClient.Definition(ctx, fileURI, position)
			if err == nil && len(locations) >= 0 {
				successCount++
				suite.T().Logf("Definition successful for %s at %+v: %d locations", 
					testFile, position, len(locations))
			} else if err != nil {
				suite.T().Logf("Definition error for %s at %+v: %v", testFile, position, err)
			}
		}
	}
	
	suite.recordTestResult("definition", successCount > 0)
	suite.Greater(successCount, 0, "At least some definition requests should succeed")
}

func (suite *GoComprehensiveRealClientE2ETestSuite) testReferencesOnAllFiles(ctx context.Context) {
	suite.T().Log("Testing textDocument/references on all Go files...")
	successCount := 0
	
	for i, testFile := range suite.goFiles {
		if i >= 8 { // Limit for performance
			break
		}
		
		fileURI := suite.getFileURI(testFile)
		positions := []testutils.Position{
			{Line: 0, Character: 0},
			{Line: 5, Character: 10},
		}
		
		for _, position := range positions {
			references, err := suite.httpClient.References(ctx, fileURI, position, true)
			if err == nil && len(references) >= 0 {
				successCount++
				suite.T().Logf("References successful for %s at %+v: %d references", 
					testFile, position, len(references))
			} else if err != nil {
				suite.T().Logf("References error for %s at %+v: %v", testFile, position, err)
			}
		}
	}
	
	suite.recordTestResult("references", successCount > 0)
	suite.Greater(successCount, 0, "At least some references requests should succeed")
}

func (suite *GoComprehensiveRealClientE2ETestSuite) testHoverOnAllFiles(ctx context.Context) {
	suite.T().Log("Testing textDocument/hover on all Go files...")
	successCount := 0
	
	for i, testFile := range suite.goFiles {
		if i >= 8 { // Limit for performance
			break
		}
		
		fileURI := suite.getFileURI(testFile)
		positions := []testutils.Position{
			{Line: 0, Character: 0},
			{Line: 3, Character: 5},
		}
		
		for _, position := range positions {
			hover, err := suite.httpClient.Hover(ctx, fileURI, position)
			if err == nil {
				successCount++
				hoverContent := ""
				if hover != nil && hover.Contents != nil {
					hoverContent = fmt.Sprintf("%v", hover.Contents)
				}
				suite.T().Logf("Hover successful for %s at %+v: %s", 
					testFile, position, hoverContent)
			} else if err != nil {
				suite.T().Logf("Hover error for %s at %+v: %v", testFile, position, err)
			}
		}
	}
	
	suite.recordTestResult("hover", successCount > 0)
	suite.Greater(successCount, 0, "At least some hover requests should succeed")
}

func (suite *GoComprehensiveRealClientE2ETestSuite) testDocumentSymbolOnAllFiles(ctx context.Context) {
	suite.T().Log("Testing textDocument/documentSymbol on all Go files...")
	successCount := 0
	
	for i, testFile := range suite.goFiles {
		if i >= 8 { // Limit for performance
			break
		}
		
		fileURI := suite.getFileURI(testFile)
		symbols, err := suite.httpClient.DocumentSymbol(ctx, fileURI)
		if err == nil && len(symbols) >= 0 {
			successCount++
			suite.T().Logf("DocumentSymbol successful for %s: %d symbols found", 
				testFile, len(symbols))
		} else if err != nil {
			suite.T().Logf("DocumentSymbol error for %s: %v", testFile, err)
		}
	}
	
	suite.recordTestResult("documentSymbol", successCount > 0)
	suite.Greater(successCount, 0, "At least some documentSymbol requests should succeed")
}

func (suite *GoComprehensiveRealClientE2ETestSuite) testWorkspaceSymbol(ctx context.Context) {
	suite.T().Log("Testing workspace/symbol with various queries...")
	
	queries := []string{
		"main",
		"func",
		"http",
		"server",
		"example",
		"", // Empty query for all symbols
	}
	
	successCount := 0
	for _, query := range queries {
		symbols, err := suite.httpClient.WorkspaceSymbol(ctx, query)
		if err == nil && len(symbols) >= 0 {
			successCount++
			suite.T().Logf("WorkspaceSymbol successful for query '%s': %d symbols found", 
				query, len(symbols))
		} else if err != nil {
			suite.T().Logf("WorkspaceSymbol error for query '%s': %v", query, err)
		}
	}
	
	suite.recordTestResult("workspaceSymbol", successCount > 0)
	suite.Greater(successCount, 0, "At least some workspaceSymbol requests should succeed")
}

func (suite *GoComprehensiveRealClientE2ETestSuite) testCompletionOnAllFiles(ctx context.Context) {
	suite.T().Log("Testing textDocument/completion on all Go files...")
	successCount := 0
	
	for i, testFile := range suite.goFiles {
		if i >= 6 { // Limit for performance
			break
		}
		
		fileURI := suite.getFileURI(testFile)
		positions := []testutils.Position{
			{Line: 5, Character: 0},   // Beginning of line
			{Line: 10, Character: 10}, // Mid line
		}
		
		for _, position := range positions {
			completions, err := suite.httpClient.Completion(ctx, fileURI, position)
			if err == nil && completions != nil {
				successCount++
				itemCount := 0
				if completions.Items != nil {
					itemCount = len(completions.Items)
				}
				suite.T().Logf("Completion successful for %s at %+v: %d items", 
					testFile, position, itemCount)
			} else if err != nil {
				suite.T().Logf("Completion error for %s at %+v: %v", testFile, position, err)
			}
		}
	}
	
	suite.recordTestResult("completion", successCount > 0)
	suite.Greater(successCount, 0, "At least some completion requests should succeed")
}

func (suite *GoComprehensiveRealClientE2ETestSuite) testConcurrentLSPRequests(ctx context.Context) {
	suite.T().Log("Testing concurrent LSP requests...")
	
	if len(suite.goFiles) == 0 {
		suite.T().Skip("No Go files available for concurrent testing")
		return
	}
	
	testFile := suite.goFiles[0]
	fileURI := suite.getFileURI(testFile)
	position := testutils.Position{Line: 5, Character: 5}
	
	concurrency := 5
	requestsPerWorker := 3
	
	var wg sync.WaitGroup
	successCount := int32(0)
	
	// Test concurrent requests
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for j := 0; j < requestsPerWorker; j++ {
				// Rotate through different LSP methods
				switch (workerID*requestsPerWorker + j) % 6 {
				case 0:
					_, err := suite.httpClient.Definition(ctx, fileURI, position)
					if err == nil {
						successCount++
					}
				case 1:
					_, err := suite.httpClient.References(ctx, fileURI, position, true)
					if err == nil {
						successCount++
					}
				case 2:
					_, err := suite.httpClient.Hover(ctx, fileURI, position)
					if err == nil {
						successCount++
					}
				case 3:
					_, err := suite.httpClient.DocumentSymbol(ctx, fileURI)
					if err == nil {
						successCount++
					}
				case 4:
					_, err := suite.httpClient.WorkspaceSymbol(ctx, "main")
					if err == nil {
						successCount++
					}
				case 5:
					_, err := suite.httpClient.Completion(ctx, fileURI, position)
					if err == nil {
						successCount++
					}
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	totalRequests := concurrency * requestsPerWorker
	suite.T().Logf("Concurrent test completed: %d/%d requests succeeded", successCount, totalRequests)
	suite.recordTestResult("concurrent", successCount > 0)
	suite.Greater(int(successCount), 0, "At least some concurrent requests should succeed")
}

// Helper methods

func (suite *GoComprehensiveRealClientE2ETestSuite) discoverGoFiles() {
	testFiles, err := suite.repoManager.GetTestFiles()
	suite.Require().NoError(err, "Failed to get Go test files from golang/example")
	suite.Require().Greater(len(testFiles), 0, "No files found in golang/example repository")
	
	// Filter for Go files and exclude test files for cleaner testing
	for _, file := range testFiles {
		if filepath.Ext(file) == ".go" && !strings.Contains(file, "_test") {
			suite.goFiles = append(suite.goFiles, file)
		}
	}
	
	suite.Require().Greater(len(suite.goFiles), 0, "No Go source files found in golang/example")
	suite.T().Logf("Discovered %d Go files in golang/example repository", len(suite.goFiles))
}

func (suite *GoComprehensiveRealClientE2ETestSuite) createGoTestConfig() {
	options := testutils.DefaultLanguageConfigOptions("go")
	options.TestPort = fmt.Sprintf("%d", suite.gatewayPort)
	options.ConfigType = "main"
	
	configPath, cleanup, err := testutils.CreateLanguageConfig(suite.repoManager, options)
	suite.Require().NoError(err, "Failed to create Go test config")
	
	suite.configPath = configPath
	_ = cleanup // Will be cleaned up via tempDir
}

func (suite *GoComprehensiveRealClientE2ETestSuite) updateConfigPort() {
	suite.createGoTestConfig()
}

func (suite *GoComprehensiveRealClientE2ETestSuite) getFileURI(filePath string) string {
	workspaceDir := suite.repoManager.GetWorkspaceDir()
	return "file://" + filepath.Join(workspaceDir, filePath)
}

func (suite *GoComprehensiveRealClientE2ETestSuite) startGatewayServer() {
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
	suite.T().Log("LSP Gateway server started successfully")
}

func (suite *GoComprehensiveRealClientE2ETestSuite) stopGatewayServer() {
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
		case <-time.After(15 * time.Second):
			suite.gatewayCmd.Process.Kill()
			suite.gatewayCmd.Wait()
		}
	}
	
	suite.gatewayCmd = nil
	suite.serverStarted = false
	suite.T().Log("LSP Gateway server stopped")
}

func (suite *GoComprehensiveRealClientE2ETestSuite) waitForServerReadiness() {
	config := testutils.DefaultPollingConfig()
	config.Timeout = 120 * time.Second // Keep original 60 * 2s = 120s timeout
	config.Interval = 2 * time.Second // Keep original interval
	
	condition := func() (bool, error) {
		return suite.checkServerHealth(), nil
	}
	
	err := testutils.WaitForCondition(condition, config, "server to be ready")
	suite.Require().NoError(err, "Server failed to become ready within timeout")
}

func (suite *GoComprehensiveRealClientE2ETestSuite) checkServerHealth() bool {
	if suite.httpClient == nil {
		return false
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	
	err := suite.httpClient.HealthCheck(ctx)
	return err == nil
}

func (suite *GoComprehensiveRealClientE2ETestSuite) verifyServerReadiness(ctx context.Context) {
	err := suite.httpClient.HealthCheck(ctx)
	suite.Require().NoError(err, "Server health check should pass")
	
	err = suite.httpClient.ValidateConnection(ctx)
	suite.Require().NoError(err, "Server connection validation should pass")
}

func (suite *GoComprehensiveRealClientE2ETestSuite) recordTestResult(testName string, success bool) {
	suite.resultsMutex.Lock()
	defer suite.resultsMutex.Unlock()
	suite.testResults[testName] = success
}

func (suite *GoComprehensiveRealClientE2ETestSuite) logTestSummary() {
	suite.resultsMutex.RLock()
	defer suite.resultsMutex.RUnlock()
	
	suite.T().Log("=== Go Comprehensive E2E Test Summary ===")
	for testName, success := range suite.testResults {
		status := "PASS"
		if !success {
			status = "FAIL"
		}
		suite.T().Logf("%s: %s", testName, status)
	}
	
	// Log HTTP client metrics
	if suite.httpClient != nil {
		metrics := suite.httpClient.GetMetrics()
		suite.T().Logf("HTTP Client Metrics:")
		suite.T().Logf("  Total Requests: %d", metrics.TotalRequests)
		suite.T().Logf("  Successful Requests: %d", metrics.SuccessfulReqs)
		suite.T().Logf("  Failed Requests: %d", metrics.FailedRequests)
		suite.T().Logf("  Average Latency: %v", metrics.AverageLatency)
	}
}

// Test runner function
func TestGoComprehensiveRealClientE2ETestSuite(t *testing.T) {
	suite.Run(t, new(GoComprehensiveRealClientE2ETestSuite))
}