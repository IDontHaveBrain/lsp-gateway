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

// JavaScriptRealClientComprehensiveE2ETestSuite tests all 6 supported LSP methods for JavaScript using Real HTTP Client with chalk repository
type JavaScriptRealClientComprehensiveE2ETestSuite struct {
	suite.Suite
	
	// Core infrastructure
	httpClient      *testutils.HttpClient
	gatewayCmd      *exec.Cmd
	gatewayPort     int
	configPath      string
	tempDir         string
	projectRoot     string
	testTimeout     time.Duration
	
	// Chalk repository management with fixed commit hash
	repoManager     testutils.RepositoryManager
	repoDir         string
	jsFiles         []string
	testFiles       []string
	
	// Server state tracking
	serverStarted   bool
	
	// Test metrics
	testResults     map[string]*TestResult
}

// SetupSuite initializes the comprehensive test suite for JavaScript using chalk repository
func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) SetupSuite() {
	suite.testTimeout = 10 * time.Minute
	suite.testResults = make(map[string]*TestResult)
	
	var err error
	suite.projectRoot, err = testutils.GetProjectRoot()
	suite.Require().NoError(err, "Failed to get project root")
	
	suite.tempDir, err = os.MkdirTemp("", "js-real-comprehensive-e2e-*")
	suite.Require().NoError(err, "Failed to create temp directory")
	
	// Initialize chalk repository manager with fixed commit
	suite.repoManager = testutils.NewJavaScriptRepositoryManager()
	
	// Setup chalk repository
	suite.repoDir, err = suite.repoManager.SetupRepository()
	suite.Require().NoError(err, "Failed to setup chalk repository")
	
	// Discover JavaScript/TypeScript files for comprehensive testing
	suite.discoverJavaScriptFiles()
	
	// Create test configuration for JavaScript comprehensive testing
	suite.createComprehensiveTestConfig()
	
	suite.T().Logf("Comprehensive JavaScript E2E test suite initialized with chalk repository (commit: 5dbc1e2)")
	suite.T().Logf("Found %d JavaScript files for testing", len(suite.jsFiles))
}

// SetupTest initializes fresh components for each test
func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) SetupTest() {
	var err error
	suite.gatewayPort, err = testutils.FindAvailablePort()
	suite.Require().NoError(err, "Failed to find available port")

	// Update config with new port
	suite.updateConfigPort()

	// Configure HttpClient for comprehensive JavaScript testing
	config := testutils.HttpClientConfig{
		BaseURL:            fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
		Timeout:            60 * time.Second,
		MaxRetries:         3,
		RetryDelay:         3 * time.Second,
		EnableLogging:      true,
		EnableRecording:    true,
		WorkspaceID:        fmt.Sprintf("js-comprehensive-test-%d", time.Now().UnixNano()),
		ProjectPath:        suite.repoDir,
		UserAgent:          "LSP-Gateway-JavaScript-Comprehensive-E2E/1.0",
		MaxResponseSize:    100 * 1024 * 1024,
		ConnectionPoolSize: 20,
		KeepAlive:          120 * time.Second,
	}

	suite.httpClient = testutils.NewHttpClient(config)
	suite.serverStarted = false
}

// TearDownTest cleans up per-test resources
func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) TearDownTest() {
	suite.stopGatewayServer()
	
	if suite.httpClient != nil {
		suite.httpClient.Close()
		suite.httpClient = nil
	}
}

// TearDownSuite performs final cleanup and reports comprehensive test results
func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) TearDownSuite() {
	suite.reportComprehensiveTestResults()
	
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

// TestJavaScriptComprehensiveServerLifecycle tests complete server lifecycle with comprehensive monitoring
func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) TestJavaScriptComprehensiveServerLifecycle() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.verifyServerReadiness(ctx)
	suite.testComprehensiveServerOperations(ctx)
}

// TestJavaScriptDefinitionComprehensive tests textDocument/definition across multiple chalk files
func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) TestJavaScriptDefinitionComprehensive() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	// Test definition on multiple JavaScript files from chalk repository
	for i, testFile := range suite.getTestFilesSubset(5) {
		testName := fmt.Sprintf("definition-test-%d", i)
		start := time.Now()
		
		fileURI := suite.getFileURI(testFile)
		
		// Test multiple positions in the file
		positions := []testutils.Position{
			{Line: 0, Character: 0},
			{Line: 1, Character: 0},
			{Line: 5, Character: 10},
		}
		
		for j, position := range positions {
			posTestName := fmt.Sprintf("%s-pos-%d", testName, j)
			
			locations, err := suite.httpClient.Definition(ctx, fileURI, position)
			
			result := &TestResult{
				Method:   "textDocument/definition",
				File:     testFile,
				Success:  err == nil,
				Duration: time.Since(start),
				Error:    err,
				Response: locations,
			}
			suite.testResults[posTestName] = result
			
			if err == nil {
				suite.T().Logf("Definition test successful for %s at position %+v - found %d locations", 
					testFile, position, len(locations))
			} else {
				suite.T().Logf("Definition test failed for %s at position %+v: %v", 
					testFile, position, err)
			}
		}
	}
}

// TestJavaScriptReferencesComprehensive tests textDocument/references across multiple chalk files
func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) TestJavaScriptReferencesComprehensive() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	// Test references on multiple JavaScript files from chalk repository
	for i, testFile := range suite.getTestFilesSubset(5) {
		testName := fmt.Sprintf("references-test-%d", i)
		start := time.Now()
		
		fileURI := suite.getFileURI(testFile)
		position := testutils.Position{Line: 0, Character: 0}
		
		references, err := suite.httpClient.References(ctx, fileURI, position, true)
		
		result := &TestResult{
			Method:   "textDocument/references",
			File:     testFile,
			Success:  err == nil,
			Duration: time.Since(start),
			Error:    err,
			Response: references,
		}
		suite.testResults[testName] = result
		
		if err == nil {
			suite.T().Logf("References test successful for %s - found %d references", 
				testFile, len(references))
		} else {
			suite.T().Logf("References test failed for %s: %v", testFile, err)
		}
	}
}

// TestJavaScriptHoverComprehensive tests textDocument/hover across multiple chalk files
func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) TestJavaScriptHoverComprehensive() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	// Test hover on multiple JavaScript files from chalk repository
	for i, testFile := range suite.getTestFilesSubset(5) {
		testName := fmt.Sprintf("hover-test-%d", i)
		start := time.Now()
		
		fileURI := suite.getFileURI(testFile)
		position := testutils.Position{Line: 0, Character: 0}
		
		hover, err := suite.httpClient.Hover(ctx, fileURI, position)
		
		result := &TestResult{
			Method:   "textDocument/hover",
			File:     testFile,
			Success:  err == nil,
			Duration: time.Since(start),
			Error:    err,
			Response: hover,
		}
		suite.testResults[testName] = result
		
		if err == nil {
			suite.T().Logf("Hover test successful for %s", testFile)
		} else {
			suite.T().Logf("Hover test failed for %s: %v", testFile, err)
		}
	}
}

// TestJavaScriptDocumentSymbolComprehensive tests textDocument/documentSymbol across multiple chalk files
func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) TestJavaScriptDocumentSymbolComprehensive() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	// Test document symbols on multiple JavaScript files from chalk repository
	for i, testFile := range suite.getTestFilesSubset(5) {
		testName := fmt.Sprintf("document-symbol-test-%d", i)
		start := time.Now()
		
		fileURI := suite.getFileURI(testFile)
		
		symbols, err := suite.httpClient.DocumentSymbol(ctx, fileURI)
		
		result := &TestResult{
			Method:   "textDocument/documentSymbol",
			File:     testFile,
			Success:  err == nil,
			Duration: time.Since(start),
			Error:    err,
			Response: symbols,
		}
		suite.testResults[testName] = result
		
		if err == nil {
			suite.T().Logf("Document symbol test successful for %s - found %d symbols", 
				testFile, len(symbols))
		} else {
			suite.T().Logf("Document symbol test failed for %s: %v", testFile, err)
		}
	}
}

// TestJavaScriptWorkspaceSymbolComprehensive tests workspace/symbol with various queries
func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) TestJavaScriptWorkspaceSymbolComprehensive() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	// Test workspace symbols with chalk-specific queries
	queries := []string{
		"chalk",
		"color",
		"style",
		"ansi",
		"export",
		"function",
		"const",
		"",  // Empty query to get all symbols
	}
	
	for i, query := range queries {
		testName := fmt.Sprintf("workspace-symbol-test-%d", i)
		start := time.Now()
		
		symbols, err := suite.httpClient.WorkspaceSymbol(ctx, query)
		
		result := &TestResult{
			Method:   "workspace/symbol",
			File:     fmt.Sprintf("query:'%s'", query),
			Success:  err == nil,
			Duration: time.Since(start),
			Error:    err,
			Response: symbols,
		}
		suite.testResults[testName] = result
		
		if err == nil {
			suite.T().Logf("Workspace symbol test successful for query '%s' - found %d symbols", 
				query, len(symbols))
		} else {
			suite.T().Logf("Workspace symbol test failed for query '%s': %v", query, err)
		}
	}
}

// TestJavaScriptCompletionComprehensive tests textDocument/completion across multiple chalk files
func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) TestJavaScriptCompletionComprehensive() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	// Test completion on multiple JavaScript files from chalk repository
	for i, testFile := range suite.getTestFilesSubset(3) {
		testName := fmt.Sprintf("completion-test-%d", i)
		start := time.Now()
		
		fileURI := suite.getFileURI(testFile)
		position := testutils.Position{Line: 0, Character: 0}
		
		completions, err := suite.httpClient.Completion(ctx, fileURI, position)
		
		result := &TestResult{
			Method:   "textDocument/completion",
			File:     testFile,
			Success:  err == nil,
			Duration: time.Since(start),
			Error:    err,
			Response: completions,
		}
		suite.testResults[testName] = result
		
		if err == nil {
			suite.T().Logf("Completion test successful for %s - found %d completions", 
				testFile, len(completions.Items))
		} else {
			suite.T().Logf("Completion test failed for %s: %v", testFile, err)
		}
	}
}

// TestJavaScriptAllLSPMethodsSequential tests all 6 LSP methods sequentially on same files
func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) TestJavaScriptAllLSPMethodsSequential() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	// Test all 6 LSP methods on the same file for comprehensive validation
	testFile := suite.getTestFilesSubset(1)[0]
	fileURI := suite.getFileURI(testFile)
	position := testutils.Position{Line: 0, Character: 0}
	
	suite.T().Logf("Testing all 6 LSP methods sequentially on chalk file: %s", testFile)
	
	// 1. textDocument/definition
	start := time.Now()
	definitions, err := suite.httpClient.Definition(ctx, fileURI, position)
	suite.testResults["sequential-definition"] = &TestResult{
		Method: "textDocument/definition", File: testFile, Success: err == nil, 
		Duration: time.Since(start), Error: err, Response: definitions,
	}
	
	// 2. textDocument/references
	start = time.Now()
	references, err := suite.httpClient.References(ctx, fileURI, position, true)
	suite.testResults["sequential-references"] = &TestResult{
		Method: "textDocument/references", File: testFile, Success: err == nil,
		Duration: time.Since(start), Error: err, Response: references,
	}
	
	// 3. textDocument/hover
	start = time.Now()
	hover, err := suite.httpClient.Hover(ctx, fileURI, position)
	suite.testResults["sequential-hover"] = &TestResult{
		Method: "textDocument/hover", File: testFile, Success: err == nil,
		Duration: time.Since(start), Error: err, Response: hover,
	}
	
	// 4. textDocument/documentSymbol
	start = time.Now()
	docSymbols, err := suite.httpClient.DocumentSymbol(ctx, fileURI)
	suite.testResults["sequential-document-symbol"] = &TestResult{
		Method: "textDocument/documentSymbol", File: testFile, Success: err == nil,
		Duration: time.Since(start), Error: err, Response: docSymbols,
	}
	
	// 5. workspace/symbol
	start = time.Now()
	wsSymbols, err := suite.httpClient.WorkspaceSymbol(ctx, "chalk")
	suite.testResults["sequential-workspace-symbol"] = &TestResult{
		Method: "workspace/symbol", File: testFile, Success: err == nil,
		Duration: time.Since(start), Error: err, Response: wsSymbols,
	}
	
	// 6. textDocument/completion
	start = time.Now()
	completions, err := suite.httpClient.Completion(ctx, fileURI, position)
	suite.testResults["sequential-completion"] = &TestResult{
		Method: "textDocument/completion", File: testFile, Success: err == nil,
		Duration: time.Since(start), Error: err, Response: completions,
	}
	
	// Verify all methods completed
	sequentialTests := []string{"sequential-definition", "sequential-references", "sequential-hover", 
		"sequential-document-symbol", "sequential-workspace-symbol", "sequential-completion"}
	
	successCount := 0
	for _, testName := range sequentialTests {
		if result, exists := suite.testResults[testName]; exists && result.Success {
			successCount++
		}
	}
	
	suite.T().Logf("Sequential LSP methods test completed: %d/6 methods successful", successCount)
	suite.GreaterOrEqual(successCount, 4, "At least 4 out of 6 LSP methods should succeed")
}

// Helper methods for comprehensive testing

func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) discoverJavaScriptFiles() {
	testFiles, err := suite.repoManager.GetTestFiles()
	suite.Require().NoError(err, "Failed to get JavaScript test files from chalk repository")
	suite.Require().Greater(len(testFiles), 0, "No JavaScript files found in chalk repository")
	
	// Filter and categorize JavaScript/TypeScript files
	for _, file := range testFiles {
		ext := filepath.Ext(file)
		if ext == ".js" || ext == ".mjs" {
			suite.jsFiles = append(suite.jsFiles, file)
		}
		if ext == ".js" || ext == ".ts" || ext == ".mjs" || ext == ".jsx" || ext == ".tsx" {
			suite.testFiles = append(suite.testFiles, file)
		}
	}
	
	suite.Require().Greater(len(suite.jsFiles), 0, "No JavaScript files found in chalk repository")
	suite.T().Logf("Discovered %d JavaScript files and %d total test files in chalk repository", 
		len(suite.jsFiles), len(suite.testFiles))
}

func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) getTestFilesSubset(count int) []string {
	if len(suite.jsFiles) <= count {
		return suite.jsFiles
	}
	return suite.jsFiles[:count]
}

func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) createComprehensiveTestConfig() {
	options := testutils.DefaultLanguageConfigOptions("javascript")
	options.TestPort = fmt.Sprintf("%d", suite.gatewayPort)
	options.ConfigType = "main"
	
	// Add comprehensive JavaScript-specific custom variables
	options.CustomVariables["NODE_PATH"] = "/usr/local/lib/node_modules"
	options.CustomVariables["TS_NODE_PROJECT"] = "tsconfig.json"
	options.CustomVariables["REPOSITORY"] = "chalk"
	options.CustomVariables["COMMIT_HASH"] = "5dbc1e2"
	options.CustomVariables["TEST_MODE"] = "comprehensive"
	options.CustomVariables["LSP_TIMEOUT"] = "60"
	
	configPath, cleanup, err := testutils.CreateLanguageConfig(suite.repoManager, options)
	suite.Require().NoError(err, "Failed to create JavaScript comprehensive test config")
	
	suite.configPath = configPath
	_ = cleanup // Will be cleaned up via tempDir
}

func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) updateConfigPort() {
	suite.createComprehensiveTestConfig()
}

func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) getFileURI(filePath string) string {
	workspaceDir := suite.repoManager.GetWorkspaceDir()
	return "file://" + filepath.Join(workspaceDir, filePath)
}

func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) startGatewayServer() {
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
	
	suite.T().Logf("Gateway server started for comprehensive JavaScript testing on port %d", suite.gatewayPort)
}

func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) stopGatewayServer() {
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
}

func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) waitForServerReadiness() {
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

func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) checkServerHealth() bool {
	if suite.httpClient == nil {
		return false
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	err := suite.httpClient.HealthCheck(ctx)
	return err == nil
}

func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) verifyServerReadiness(ctx context.Context) {
	err := suite.httpClient.HealthCheck(ctx)
	suite.Require().NoError(err, "Server health check should pass")
}

func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) testComprehensiveServerOperations(ctx context.Context) {
	err := suite.httpClient.ValidateConnection(ctx)
	suite.Require().NoError(err, "Server connection validation should pass")
}

func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) reportComprehensiveTestResults() {
	suite.T().Logf("=== Comprehensive JavaScript E2E Test Results ===")
	
	methodCounts := make(map[string]int)
	methodSuccesses := make(map[string]int)
	totalDuration := time.Duration(0)
	
	for testName, result := range suite.testResults {
		methodCounts[result.Method]++
		if result.Success {
			methodSuccesses[result.Method]++
		}
		totalDuration += result.Duration
		
		status := "PASS"
		if !result.Success {
			status = "FAIL"
		}
		
		suite.T().Logf("[%s] %s - %s (%v)", status, result.Method, testName, result.Duration)
		if !result.Success && result.Error != nil {
			suite.T().Logf("  Error: %v", result.Error)
		}
	}
	
	suite.T().Logf("=== Method Summary ===")
	for method := range methodCounts {
		successRate := float64(methodSuccesses[method]) / float64(methodCounts[method]) * 100
		suite.T().Logf("%s: %d/%d (%.1f%%)", method, methodSuccesses[method], methodCounts[method], successRate)
	}
	
	totalTests := len(suite.testResults)
	totalSuccesses := 0
	for _, count := range methodSuccesses {
		totalSuccesses += count
	}
	
	overallSuccessRate := float64(totalSuccesses) / float64(totalTests) * 100
	suite.T().Logf("=== Overall Results ===")
	suite.T().Logf("Total tests: %d", totalTests)
	suite.T().Logf("Successful: %d", totalSuccesses)
	suite.T().Logf("Success rate: %.1f%%", overallSuccessRate)
	suite.T().Logf("Total duration: %v", totalDuration)
	suite.T().Logf("Average duration: %v", totalDuration/time.Duration(totalTests))
	
	// Verify comprehensive test quality requirements
	suite.GreaterOrEqual(overallSuccessRate, 80.0, "Overall success rate should be at least 80%")
	suite.LessOrEqual(totalDuration.Minutes(), 15.0, "Total test duration should be under 15 minutes")
}

// Test runner function
func TestJavaScriptRealClientComprehensiveE2ETestSuite(t *testing.T) {
	suite.Run(t, new(JavaScriptRealClientComprehensiveE2ETestSuite))
}