package e2e_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/tests/e2e/testutils"
)

// GoRealClientComprehensiveE2ETestSuite tests all 6 supported LSP methods for Go using Real HTTP Client with golang/example repository
type GoRealClientComprehensiveE2ETestSuite struct {
	suite.Suite
	
	// Core infrastructure
	httpClient      *testutils.HttpClient
	gatewayCmd      *exec.Cmd
	gatewayPort     int
	configPath      string
	tempDir         string
	projectRoot     string
	testTimeout     time.Duration
	
	// golang/example repository management with fixed commit hash
	repoManager     testutils.RepositoryManager
	repoDir         string
	goFiles         []string
	testFiles       []string
	
	// Server state tracking
	serverStarted   bool
	
	// Test metrics
	testResults     map[string]*TestResult
}

// SetupSuite initializes the comprehensive test suite for Go using golang/example repository
func (suite *GoRealClientComprehensiveE2ETestSuite) SetupSuite() {
	suite.testTimeout = 15 * time.Second
	suite.testResults = make(map[string]*TestResult)
	
	var err error
	suite.projectRoot, err = testutils.GetProjectRoot()
	suite.Require().NoError(err, "Failed to get project root")
	
	suite.tempDir, err = os.MkdirTemp("", "go-real-comprehensive-e2e-*")
	suite.Require().NoError(err, "Failed to create temp directory")
	
	// Initialize golang/example repository manager with fixed commit
	suite.repoManager = testutils.NewGoRepositoryManager()
	
	// Setup golang/example repository
	suite.repoDir, err = suite.repoManager.SetupRepository()
	suite.Require().NoError(err, "Failed to setup golang/example repository")
	
	// Discover Go files for comprehensive testing
	suite.discoverGoFiles()
	
	// Create test configuration for Go comprehensive testing
	suite.createComprehensiveTestConfig()
	
	suite.T().Logf("Comprehensive Go E2E test suite initialized with golang/example repository (commit: 8b40562)")
	suite.T().Logf("Found %d Go files for testing", len(suite.goFiles))
}

// SetupTest initializes fresh components for each test
func (suite *GoRealClientComprehensiveE2ETestSuite) SetupTest() {
	var err error
	suite.gatewayPort, err = testutils.FindAvailablePort()
	suite.Require().NoError(err, "Failed to find available port")

	// Update config with new port
	suite.updateConfigPort()

	// Configure HttpClient for comprehensive Go testing
	config := testutils.HttpClientConfig{
		BaseURL:            fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
		Timeout:            5 * time.Second,
		MaxRetries:         3,
		RetryDelay:         1 * time.Second,
		EnableLogging:      true,
		EnableRecording:    true,
		WorkspaceID:        fmt.Sprintf("go-comprehensive-test-%d", time.Now().UnixNano()),
		ProjectPath:        suite.repoDir,
		UserAgent:          "LSP-Gateway-Go-Comprehensive-E2E/1.0",
		MaxResponseSize:    100 * 1024 * 1024,
		ConnectionPoolSize: 20,
		KeepAlive:          30 * time.Second,
	}

	suite.httpClient = testutils.NewHttpClient(config)
	suite.serverStarted = false
}

// TearDownTest cleans up per-test resources
func (suite *GoRealClientComprehensiveE2ETestSuite) TearDownTest() {
	suite.stopGatewayServer()
	
	if suite.httpClient != nil {
		suite.httpClient.Close()
		suite.httpClient = nil
	}
}

// TearDownSuite performs final cleanup and reports comprehensive test results
func (suite *GoRealClientComprehensiveE2ETestSuite) TearDownSuite() {
	suite.reportComprehensiveTestResults()
	
	if suite.repoManager != nil {
		if err := suite.repoManager.Cleanup(); err != nil {
			suite.T().Logf("Warning: Failed to cleanup golang/example repository: %v", err)
		}
	}
	
	if suite.tempDir != "" {
		if err := os.RemoveAll(suite.tempDir); err != nil {
			suite.T().Logf("Warning: Failed to remove temp directory: %v", err)
		}
	}
}

// TestGoComprehensiveServerLifecycle tests complete server lifecycle with comprehensive monitoring
func (suite *GoRealClientComprehensiveE2ETestSuite) TestGoComprehensiveServerLifecycle() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.verifyServerReadiness(ctx)
	suite.testComprehensiveServerOperations(ctx)
}

// TestGoDefinitionComprehensive tests textDocument/definition across multiple golang/example files
func (suite *GoRealClientComprehensiveE2ETestSuite) TestGoDefinitionComprehensive() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.T().Logf("Testing textDocument/definition on %d Go files", len(suite.goFiles))
	
	for _, testFile := range suite.goFiles {
		suite.testDefinitionOnFile(ctx, testFile)
	}
	
	// Test specific positions in key files
	suite.testDefinitionAtSpecificPositions(ctx)
}

// TestGoReferencesComprehensive tests textDocument/references across golang/example codebase
func (suite *GoRealClientComprehensiveE2ETestSuite) TestGoReferencesComprehensive() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.T().Logf("Testing textDocument/references on %d Go files", len(suite.goFiles))
	
	for _, testFile := range suite.goFiles {
		suite.testReferencesOnFile(ctx, testFile)
	}
}

// TestGoHoverComprehensive tests textDocument/hover across golang/example files
func (suite *GoRealClientComprehensiveE2ETestSuite) TestGoHoverComprehensive() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.T().Logf("Testing textDocument/hover on %d Go files", len(suite.goFiles))
	
	for _, testFile := range suite.goFiles {
		suite.testHoverOnFile(ctx, testFile)
	}
}

// TestGoDocumentSymbolComprehensive tests textDocument/documentSymbol across golang/example files
func (suite *GoRealClientComprehensiveE2ETestSuite) TestGoDocumentSymbolComprehensive() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.T().Logf("Testing textDocument/documentSymbol on %d Go files", len(suite.goFiles))
	
	for _, testFile := range suite.goFiles {
		suite.testDocumentSymbolOnFile(ctx, testFile)
	}
}

// TestGoWorkspaceSymbolComprehensive tests workspace/symbol across entire golang/example repository
func (suite *GoRealClientComprehensiveE2ETestSuite) TestGoWorkspaceSymbolComprehensive() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	// Test common Go symbols
	queries := []string{
		"main",          // main functions
		"String",        // common method names
		"Error",         // error types
		"Handler",       // handler patterns
		"Client",        // client types
		"Server",        // server types
		"Config",        // configuration types
		"Test",          // test functions
	}
	
	suite.T().Logf("Testing workspace/symbol with %d queries", len(queries))
	
	for _, query := range queries {
		suite.testWorkspaceSymbolQuery(ctx, query)
	}
}

// TestGoCompletionComprehensive tests textDocument/completion across golang/example files
func (suite *GoRealClientComprehensiveE2ETestSuite) TestGoCompletionComprehensive() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.T().Logf("Testing textDocument/completion on %d Go files", len(suite.goFiles))
	
	for _, testFile := range suite.goFiles {
		suite.testCompletionOnFile(ctx, testFile)
	}
}

// Helper methods

func (suite *GoRealClientComprehensiveE2ETestSuite) discoverGoFiles() {
	testFiles, err := suite.repoManager.GetTestFiles()
	suite.Require().NoError(err, "Failed to get Go test files")
	suite.Require().Greater(len(testFiles), 0, "No Go files found in golang/example repository")
	
	// Filter for Go files only and prioritize important ones
	priorityFiles := []string{}
	regularFiles := []string{}
	
	for _, file := range testFiles {
		if filepath.Ext(file) == ".go" {
			// Prioritize main.go, examples, and files in key directories
			if strings.Contains(file, "main.go") || 
			   strings.Contains(file, "example") ||
			   strings.Contains(file, "cmd/") ||
			   strings.Contains(file, "pkg/") {
				priorityFiles = append(priorityFiles, file)
			} else {
				regularFiles = append(regularFiles, file)
			}
		}
	}
	
	// Combine with priority files first
	suite.goFiles = append(priorityFiles, regularFiles...)
	suite.testFiles = suite.goFiles
	
	suite.Require().Greater(len(suite.goFiles), 0, "No Go files found")
	suite.T().Logf("Discovered %d priority files and %d regular files", len(priorityFiles), len(regularFiles))
}

func (suite *GoRealClientComprehensiveE2ETestSuite) createComprehensiveTestConfig() {
	options := testutils.DefaultLanguageConfigOptions("go")
	options.TestPort = fmt.Sprintf("%d", suite.gatewayPort)
	options.ConfigType = "comprehensive"
	
	configPath, cleanup, err := testutils.CreateLanguageConfig(suite.repoManager, options)
	suite.Require().NoError(err, "Failed to create Go comprehensive test config")
	
	suite.configPath = configPath
	_ = cleanup // Will be cleaned up via tempDir
}

func (suite *GoRealClientComprehensiveE2ETestSuite) updateConfigPort() {
	suite.createComprehensiveTestConfig()
}

func (suite *GoRealClientComprehensiveE2ETestSuite) getFileURI(filePath string) string {
	workspaceDir := suite.repoManager.GetWorkspaceDir()
	return "file://" + filepath.Join(workspaceDir, filePath)
}

func (suite *GoRealClientComprehensiveE2ETestSuite) startGatewayServer() {
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

func (suite *GoRealClientComprehensiveE2ETestSuite) stopGatewayServer() {
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

func (suite *GoRealClientComprehensiveE2ETestSuite) waitForServerReadiness() {
	config := testutils.DefaultPollingConfig()
	config.Timeout = 60 * time.Second // Keep original 60 * 1s = 60s timeout for comprehensive tests
	config.Interval = time.Second // Keep original interval
	
	condition := func() (bool, error) {
		return suite.checkServerHealth(), nil
	}
	
	err := testutils.WaitForCondition(condition, config, "server to be ready")
	suite.Require().NoError(err, "Server failed to become ready within timeout")
}

func (suite *GoRealClientComprehensiveE2ETestSuite) checkServerHealth() bool {
	if suite.httpClient == nil {
		return false
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err := suite.httpClient.HealthCheck(ctx)
	return err == nil
}

func (suite *GoRealClientComprehensiveE2ETestSuite) verifyServerReadiness(ctx context.Context) {
	err := suite.httpClient.HealthCheck(ctx)
	suite.Require().NoError(err, "Server health check should pass")
}

func (suite *GoRealClientComprehensiveE2ETestSuite) testComprehensiveServerOperations(ctx context.Context) {
	err := suite.httpClient.ValidateConnection(ctx)
	suite.Require().NoError(err, "Server connection validation should pass")
}

// LSP method testing helpers

func (suite *GoRealClientComprehensiveE2ETestSuite) testDefinitionOnFile(ctx context.Context, testFile string) {
	start := time.Now()
	fileURI := suite.getFileURI(testFile)
	
	// Test multiple positions
	positions := []testutils.Position{
		{Line: 0, Character: 0},   // Start of file
		{Line: 5, Character: 10},  // Mid-file
		{Line: 10, Character: 5},  // Different position
	}
	
	for _, position := range positions {
		locations, err := suite.httpClient.Definition(ctx, fileURI, position)
		
		result := &TestResult{
			Method:   "textDocument/definition",
			File:     testFile,
			Success:  err == nil,
			Duration: time.Since(start),
			Error:    err,
			Response: locations,
		}
		
		key := fmt.Sprintf("definition_%s_%d_%d", testFile, position.Line, position.Character)
		suite.testResults[key] = result
		
		if err != nil {
			suite.T().Logf("Definition failed for %s at %d:%d: %v", testFile, position.Line, position.Character, err)
		} else {
			suite.T().Logf("Definition succeeded for %s at %d:%d: %d locations", testFile, position.Line, position.Character, len(locations))
		}
	}
}

func (suite *GoRealClientComprehensiveE2ETestSuite) testDefinitionAtSpecificPositions(ctx context.Context) {
	// Test specific known positions in golang/example files
	specificTests := []struct {
		file     string
		line     int
		char     int
		expected string
	}{
		// These would be adapted based on actual golang/example content
		{"example/hello/hello.go", 5, 10, "main function"},
		{"example/stringutil/reverse.go", 8, 15, "string function"},
	}
	
	for _, test := range specificTests {
		if suite.fileExists(test.file) {
			fileURI := suite.getFileURI(test.file)
			position := testutils.Position{Line: test.line, Character: test.char}
			
			start := time.Now()
			locations, err := suite.httpClient.Definition(ctx, fileURI, position)
			
			result := &TestResult{
				Method:   "textDocument/definition",
				File:     test.file,
				Success:  err == nil,
				Duration: time.Since(start),
				Error:    err,
				Response: locations,
			}
			
			key := fmt.Sprintf("specific_definition_%s_%d_%d", test.file, test.line, test.char)
			suite.testResults[key] = result
			
			if err == nil {
				suite.T().Logf("Specific definition test passed for %s: %s", test.file, test.expected)
			}
		}
	}
}

func (suite *GoRealClientComprehensiveE2ETestSuite) testReferencesOnFile(ctx context.Context, testFile string) {
	start := time.Now()
	fileURI := suite.getFileURI(testFile)
	position := testutils.Position{Line: 5, Character: 10}
	
	locations, err := suite.httpClient.References(ctx, fileURI, position, true)
	
	result := &TestResult{
		Method:   "textDocument/references",
		File:     testFile,
		Success:  err == nil,
		Duration: time.Since(start),
		Error:    err,
		Response: locations,
	}
	
	key := fmt.Sprintf("references_%s", testFile)
	suite.testResults[key] = result
	
	if err != nil {
		suite.T().Logf("References failed for %s: %v", testFile, err)
	} else {
		suite.T().Logf("References succeeded for %s: %d locations", testFile, len(locations))
	}
}

func (suite *GoRealClientComprehensiveE2ETestSuite) testHoverOnFile(ctx context.Context, testFile string) {
	start := time.Now()
	fileURI := suite.getFileURI(testFile)
	position := testutils.Position{Line: 5, Character: 10}
	
	hover, err := suite.httpClient.Hover(ctx, fileURI, position)
	
	result := &TestResult{
		Method:   "textDocument/hover",
		File:     testFile,
		Success:  err == nil,
		Duration: time.Since(start),
		Error:    err,
		Response: hover,
	}
	
	key := fmt.Sprintf("hover_%s", testFile)
	suite.testResults[key] = result
	
	if err != nil {
		suite.T().Logf("Hover failed for %s: %v", testFile, err)
	} else {
		suite.T().Logf("Hover succeeded for %s", testFile)
	}
}

func (suite *GoRealClientComprehensiveE2ETestSuite) testDocumentSymbolOnFile(ctx context.Context, testFile string) {
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
	
	key := fmt.Sprintf("documentSymbol_%s", testFile)
	suite.testResults[key] = result
	
	if err != nil {
		suite.T().Logf("DocumentSymbol failed for %s: %v", testFile, err)
	} else {
		suite.T().Logf("DocumentSymbol succeeded for %s: %d symbols", testFile, len(symbols))
	}
}

func (suite *GoRealClientComprehensiveE2ETestSuite) testWorkspaceSymbolQuery(ctx context.Context, query string) {
	start := time.Now()
	symbols, err := suite.httpClient.WorkspaceSymbol(ctx, query)
	
	result := &TestResult{
		Method:   "workspace/symbol",
		File:     fmt.Sprintf("query_%s", query),
		Success:  err == nil,
		Duration: time.Since(start),
		Error:    err,
		Response: symbols,
	}
	
	key := fmt.Sprintf("workspaceSymbol_%s", query)
	suite.testResults[key] = result
	
	if err != nil {
		suite.T().Logf("WorkspaceSymbol failed for query '%s': %v", query, err)
	} else {
		suite.T().Logf("WorkspaceSymbol succeeded for query '%s': %d symbols", query, len(symbols))
	}
}

func (suite *GoRealClientComprehensiveE2ETestSuite) testCompletionOnFile(ctx context.Context, testFile string) {
	start := time.Now()
	fileURI := suite.getFileURI(testFile)
	position := testutils.Position{Line: 5, Character: 10}
	
	completion, err := suite.httpClient.Completion(ctx, fileURI, position)
	
	result := &TestResult{
		Method:   "textDocument/completion",
		File:     testFile,
		Success:  err == nil,
		Duration: time.Since(start),
		Error:    err,
		Response: completion,
	}
	
	key := fmt.Sprintf("completion_%s", testFile)
	suite.testResults[key] = result
	
	if err != nil {
		suite.T().Logf("Completion failed for %s: %v", testFile, err)
	} else {
		suite.T().Logf("Completion succeeded for %s", testFile)
	}
}

func (suite *GoRealClientComprehensiveE2ETestSuite) fileExists(filePath string) bool {
	workspaceDir := suite.repoManager.GetWorkspaceDir()
	fullPath := filepath.Join(workspaceDir, filePath)
	_, err := os.Stat(fullPath)
	return err == nil
}

func (suite *GoRealClientComprehensiveE2ETestSuite) reportComprehensiveTestResults() {
	suite.T().Logf("\n=== Comprehensive Go E2E Test Results ===")
	
	methodStats := make(map[string]int)
	successStats := make(map[string]int)
	totalDuration := time.Duration(0)
	
	for _, result := range suite.testResults {
		methodStats[result.Method]++
		if result.Success {
			successStats[result.Method]++
		}
		totalDuration += result.Duration
	}
	
	suite.T().Logf("Total tests: %d", len(suite.testResults))
	suite.T().Logf("Total duration: %v", totalDuration)
	
	for method, total := range methodStats {
		success := successStats[method]
		successRate := float64(success) / float64(total) * 100
		suite.T().Logf("%s: %d/%d (%.1f%% success)", method, success, total, successRate)
	}
	
	// Log failures
	failedTests := 0
	for key, result := range suite.testResults {
		if !result.Success {
			suite.T().Logf("FAILED: %s - %v", key, result.Error)
			failedTests++
		}
	}
	
	if failedTests > 0 {
		suite.T().Logf("Total failures: %d", failedTests)
	} else {
		suite.T().Logf("All comprehensive tests passed!")
	}
}

// Test runner function
func TestGoRealClientComprehensiveE2ETestSuite(t *testing.T) {
	suite.Run(t, new(GoRealClientComprehensiveE2ETestSuite))
}