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
	"lsp-gateway/tests/e2e/fixtures"
)

// PythonRealClientComprehensiveE2ETestSuite tests all 6 supported LSP methods for Python using Real HTTP Client with design pattern scenarios
type PythonRealClientComprehensiveE2ETestSuite struct {
	suite.Suite
	
	// Core infrastructure
	httpClient      *testutils.HttpClient
	gatewayCmd      *exec.Cmd
	gatewayPort     int
	configPath      string
	tempDir         string
	projectRoot     string
	testTimeout     time.Duration
	
	// Python design patterns repository management
	repoManager     testutils.RepositoryManager
	repoDir         string
	pyFiles         []string
	patternScenarios []fixtures.PythonPatternScenario
	
	// Server state tracking
	serverStarted   bool
	
	// Test metrics
	testResults     map[string]*TestResult
}

// SetupSuite initializes the comprehensive test suite for Python using design pattern scenarios
func (suite *PythonRealClientComprehensiveE2ETestSuite) SetupSuite() {
	suite.testTimeout = 15 * time.Second
	suite.testResults = make(map[string]*TestResult)
	
	var err error
	suite.projectRoot, err = testutils.GetProjectRoot()
	suite.Require().NoError(err, "Failed to get project root")
	
	suite.tempDir, err = os.MkdirTemp("", "py-real-comprehensive-e2e-*")
	suite.Require().NoError(err, "Failed to create temp directory")
	
	// Initialize Python repository manager with design patterns
	suite.repoManager = testutils.NewPythonRepositoryManager()
	
	// Setup Python repository with design pattern scenarios
	suite.repoDir, err = suite.repoManager.SetupRepository()
	suite.Require().NoError(err, "Failed to setup Python repository")
	
	// Load all 13 Python design pattern scenarios
	suite.patternScenarios = fixtures.GetAllPythonPatternScenarios()
	suite.Require().Equal(13, len(suite.patternScenarios), "Expected 13 Python design pattern scenarios")
	
	// Discover Python files for comprehensive testing
	suite.discoverPythonFiles()
	
	// Create test configuration for Python comprehensive testing
	suite.createComprehensiveTestConfig()
	
	suite.T().Logf("Comprehensive Python E2E test suite initialized with %d design pattern scenarios", len(suite.patternScenarios))
	suite.T().Logf("Found %d Python files for testing", len(suite.pyFiles))
}

// SetupTest initializes fresh components for each test
func (suite *PythonRealClientComprehensiveE2ETestSuite) SetupTest() {
	var err error
	suite.gatewayPort, err = testutils.FindAvailablePort()
	suite.Require().NoError(err, "Failed to find available port")

	// Update config with new port
	suite.updateConfigPort()

	// Configure HttpClient for comprehensive Python testing
	config := testutils.HttpClientConfig{
		BaseURL:            fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
		Timeout:            5 * time.Second,
		MaxRetries:         3,
		RetryDelay:         1 * time.Second,
		EnableLogging:      true,
		EnableRecording:    true,
		WorkspaceID:        fmt.Sprintf("py-comprehensive-test-%d", time.Now().UnixNano()),
		ProjectPath:        suite.repoDir,
		UserAgent:          "LSP-Gateway-Python-Comprehensive-E2E/1.0",
		MaxResponseSize:    100 * 1024 * 1024,
		ConnectionPoolSize: 20,
		KeepAlive:          30 * time.Second,
	}

	suite.httpClient = testutils.NewHttpClient(config)
	suite.serverStarted = false
}

// TearDownTest cleans up per-test resources
func (suite *PythonRealClientComprehensiveE2ETestSuite) TearDownTest() {
	suite.stopGatewayServer()
	
	if suite.httpClient != nil {
		suite.httpClient.Close()
		suite.httpClient = nil
	}
}

// TearDownSuite performs final cleanup and reports comprehensive test results
func (suite *PythonRealClientComprehensiveE2ETestSuite) TearDownSuite() {
	suite.reportComprehensiveTestResults()
	
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

// TestPythonComprehensiveServerLifecycle tests complete server lifecycle with comprehensive monitoring
func (suite *PythonRealClientComprehensiveE2ETestSuite) TestPythonComprehensiveServerLifecycle() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.verifyServerReadiness(ctx)
	suite.testComprehensiveServerOperations(ctx)
}

// TestPythonComprehensiveDefinition tests textDocument/definition across multiple Python design pattern files
func (suite *PythonRealClientComprehensiveE2ETestSuite) TestPythonComprehensiveDefinition() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	// Test definition on multiple Python files using design pattern scenarios
	definitionScenarios := fixtures.GetScenariosForLSPMethod("textDocument/definition")
	testFiles := suite.getTestFilesSubset(5)
	
	for i, testFile := range testFiles {
		testName := fmt.Sprintf("definition-test-%d", i)
		start := time.Now()
		
		fileURI := suite.getFileURI(testFile)
		
		// Use scenario-specific test positions if available
		var positions []testutils.Position
		if i < len(definitionScenarios) {
			scenario := definitionScenarios[i]
			for _, pos := range scenario.TestPositions {
				positions = append(positions, testutils.Position{
					Line:      pos.Line,
					Character: pos.Character,
				})
			}
		} else {
			// Fallback to default positions
			positions = []testutils.Position{
				{Line: 0, Character: 0},
				{Line: 1, Character: 0},
				{Line: 5, Character: 10},
			}
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

// TestPythonComprehensiveReferences tests textDocument/references across multiple Python design pattern files
func (suite *PythonRealClientComprehensiveE2ETestSuite) TestPythonComprehensiveReferences() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	// Test references on multiple Python files using design pattern scenarios
	referencesScenarios := fixtures.GetScenariosForLSPMethod("textDocument/references")
	testFiles := suite.getTestFilesSubset(5)
	
	for i, testFile := range testFiles {
		testName := fmt.Sprintf("references-test-%d", i)
		start := time.Now()
		
		fileURI := suite.getFileURI(testFile)
		
		// Use scenario-specific test position if available
		var position testutils.Position
		if i < len(referencesScenarios) {
			scenario := referencesScenarios[i]
			if len(scenario.TestPositions) > 0 {
				pos := scenario.TestPositions[0]
				position = testutils.Position{Line: pos.Line, Character: pos.Character}
			} else {
				position = testutils.Position{Line: 0, Character: 0}
			}
		} else {
			position = testutils.Position{Line: 0, Character: 0}
		}
		
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

// TestPythonComprehensiveHover tests textDocument/hover across multiple Python design pattern files
func (suite *PythonRealClientComprehensiveE2ETestSuite) TestPythonComprehensiveHover() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	// Test hover on multiple Python files using design pattern scenarios
	hoverScenarios := fixtures.GetScenariosForLSPMethod("textDocument/hover")
	testFiles := suite.getTestFilesSubset(5)
	
	for i, testFile := range testFiles {
		testName := fmt.Sprintf("hover-test-%d", i)
		start := time.Now()
		
		fileURI := suite.getFileURI(testFile)
		
		// Use scenario-specific test position if available
		var position testutils.Position
		if i < len(hoverScenarios) {
			scenario := hoverScenarios[i]
			if len(scenario.TestPositions) > 0 {
				pos := scenario.TestPositions[0]
				position = testutils.Position{Line: pos.Line, Character: pos.Character}
			} else {
				position = testutils.Position{Line: 0, Character: 0}
			}
		} else {
			position = testutils.Position{Line: 0, Character: 0}
		}
		
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

// TestPythonComprehensiveDocumentSymbol tests textDocument/documentSymbol across multiple Python design pattern files
func (suite *PythonRealClientComprehensiveE2ETestSuite) TestPythonComprehensiveDocumentSymbol() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	// Test document symbols on multiple Python files using design pattern scenarios
	documentSymbolScenarios := fixtures.GetScenariosForLSPMethod("textDocument/documentSymbol")
	testFiles := suite.getTestFilesSubset(5)
	
	for i, testFile := range testFiles {
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
			expectedSymbolCount := 0
			if i < len(documentSymbolScenarios) {
				expectedSymbolCount = len(documentSymbolScenarios[i].ExpectedSymbols)
			}
			suite.T().Logf("Document symbol test successful for %s - found %d symbols (expected ~%d)", 
				testFile, len(symbols), expectedSymbolCount)
		} else {
			suite.T().Logf("Document symbol test failed for %s: %v", testFile, err)
		}
	}
}

// TestPythonComprehensiveWorkspaceSymbol tests workspace/symbol with Python-specific design pattern queries
func (suite *PythonRealClientComprehensiveE2ETestSuite) TestPythonComprehensiveWorkspaceSymbol() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	// Test workspace symbols with Python design pattern-specific queries
	queries := []string{
		"pattern",      // Generic pattern-related symbols
		"class",        // Python class symbols
		"def",          // Python function/method definitions
		"import",       // Python import statements
		"decorator",    // Decorator pattern symbols
		"singleton",    // Singleton pattern symbols
		"factory",      // Factory pattern symbols
		"observer",     // Observer pattern symbols
		"adapter",      // Adapter pattern symbols
		"command",      // Command pattern symbols
		"",             // Empty query to get all symbols
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

// TestPythonComprehensiveCompletion tests textDocument/completion across multiple Python design pattern files
func (suite *PythonRealClientComprehensiveE2ETestSuite) TestPythonComprehensiveCompletion() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	// Test completion on multiple Python files using design pattern scenarios
	completionScenarios := fixtures.GetScenariosForLSPMethod("textDocument/completion")
	testFiles := suite.getTestFilesSubset(3)
	
	for i, testFile := range testFiles {
		testName := fmt.Sprintf("completion-test-%d", i)
		start := time.Now()
		
		fileURI := suite.getFileURI(testFile)
		
		// Use scenario-specific test position if available
		var position testutils.Position
		if i < len(completionScenarios) {
			scenario := completionScenarios[i]
			if len(scenario.TestPositions) > 0 {
				pos := scenario.TestPositions[0]
				position = testutils.Position{Line: pos.Line, Character: pos.Character}
			} else {
				position = testutils.Position{Line: 0, Character: 0}
			}
		} else {
			position = testutils.Position{Line: 0, Character: 0}
		}
		
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

// TestPythonAllLSPMethodsSequential tests all 6 LSP methods sequentially on same files
func (suite *PythonRealClientComprehensiveE2ETestSuite) TestPythonAllLSPMethodsSequential() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	// Test all 6 LSP methods on the same file for comprehensive validation
	testFile := suite.getTestFilesSubset(1)[0]
	fileURI := suite.getFileURI(testFile)
	
	// Use first design pattern scenario position if available
	var position testutils.Position
	if len(suite.patternScenarios) > 0 && len(suite.patternScenarios[0].TestPositions) > 0 {
		pos := suite.patternScenarios[0].TestPositions[0]
		position = testutils.Position{Line: pos.Line, Character: pos.Character}
	} else {
		position = testutils.Position{Line: 0, Character: 0}
	}
	
	suite.T().Logf("Testing all 6 LSP methods sequentially on Python file: %s", testFile)
	
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
	wsSymbols, err := suite.httpClient.WorkspaceSymbol(ctx, "pattern")
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

func (suite *PythonRealClientComprehensiveE2ETestSuite) discoverPythonFiles() {
	testFiles, err := suite.repoManager.GetTestFiles()
	suite.Require().NoError(err, "Failed to get Python test files from repository")
	suite.Require().Greater(len(testFiles), 0, "No Python files found in repository")
	
	// Filter and categorize Python files
	for _, file := range testFiles {
		ext := filepath.Ext(file)
		if ext == ".py" {
			suite.pyFiles = append(suite.pyFiles, file)
		}
	}
	
	suite.Require().Greater(len(suite.pyFiles), 0, "No Python files found in repository")
	suite.T().Logf("Discovered %d Python files in repository with %d design pattern scenarios", 
		len(suite.pyFiles), len(suite.patternScenarios))
}

func (suite *PythonRealClientComprehensiveE2ETestSuite) getTestFilesSubset(count int) []string {
	if len(suite.pyFiles) <= count {
		return suite.pyFiles
	}
	return suite.pyFiles[:count]
}

func (suite *PythonRealClientComprehensiveE2ETestSuite) createComprehensiveTestConfig() {
	options := testutils.DefaultLanguageConfigOptions("python")
	options.TestPort = fmt.Sprintf("%d", suite.gatewayPort)
	options.ConfigType = "main"
	
	// Add comprehensive Python-specific custom variables
	options.CustomVariables["PYTHONPATH"] = suite.repoDir
	options.CustomVariables["PYTHON_LSP_SERVER"] = "pylsp"
	options.CustomVariables["REPOSITORY"] = "python-patterns"
	options.CustomVariables["DESIGN_PATTERNS"] = "13"
	options.CustomVariables["TEST_MODE"] = "comprehensive"
	options.CustomVariables["LSP_TIMEOUT"] = "60"
	options.CustomVariables["PATTERN_TYPES"] = "creational,structural,behavioral"
	
	configPath, cleanup, err := testutils.CreateLanguageConfig(suite.repoManager, options)
	suite.Require().NoError(err, "Failed to create Python comprehensive test config")
	
	suite.configPath = configPath
	_ = cleanup // Will be cleaned up via tempDir
}

func (suite *PythonRealClientComprehensiveE2ETestSuite) updateConfigPort() {
	suite.createComprehensiveTestConfig()
}

func (suite *PythonRealClientComprehensiveE2ETestSuite) getFileURI(filePath string) string {
	workspaceDir := suite.repoManager.GetWorkspaceDir()
	return "file://" + filepath.Join(workspaceDir, filePath)
}

func (suite *PythonRealClientComprehensiveE2ETestSuite) startGatewayServer() {
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
	
	suite.T().Logf("Gateway server started for comprehensive Python testing on port %d", suite.gatewayPort)
}

func (suite *PythonRealClientComprehensiveE2ETestSuite) stopGatewayServer() {
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

func (suite *PythonRealClientComprehensiveE2ETestSuite) waitForServerReadiness() {
	config := testutils.DefaultPollingConfig()
	config.Timeout = 120 * time.Second // Keep original 60 * 2s = 120s timeout
	config.Interval = 2 * time.Second // Keep original interval
	
	condition := func() (bool, error) {
		return suite.checkServerHealth(), nil
	}
	
	err := testutils.WaitForCondition(condition, config, "server to be ready")
	suite.Require().NoError(err, "Server failed to become ready within timeout")
}

func (suite *PythonRealClientComprehensiveE2ETestSuite) checkServerHealth() bool {
	if suite.httpClient == nil {
		return false
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	err := suite.httpClient.HealthCheck(ctx)
	return err == nil
}

func (suite *PythonRealClientComprehensiveE2ETestSuite) verifyServerReadiness(ctx context.Context) {
	err := suite.httpClient.HealthCheck(ctx)
	suite.Require().NoError(err, "Server health check should pass")
}

func (suite *PythonRealClientComprehensiveE2ETestSuite) testComprehensiveServerOperations(ctx context.Context) {
	err := suite.httpClient.ValidateConnection(ctx)
	suite.Require().NoError(err, "Server connection validation should pass")
}

func (suite *PythonRealClientComprehensiveE2ETestSuite) reportComprehensiveTestResults() {
	suite.T().Logf("=== Comprehensive Python E2E Test Results ===")
	
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
	
	// Python design pattern specific metrics
	suite.T().Logf("=== Design Pattern Integration ===")
	suite.T().Logf("Design pattern scenarios: %d", len(suite.patternScenarios))
	suite.T().Logf("Creational patterns: %d", len(fixtures.GetCreationalPatternScenarios()))
	suite.T().Logf("Structural patterns: %d", len(fixtures.GetStructuralPatternScenarios()))
	suite.T().Logf("Behavioral patterns: %d", len(fixtures.GetBehavioralPatternScenarios()))
	
	// Verify comprehensive test quality requirements
	suite.GreaterOrEqual(overallSuccessRate, 80.0, "Overall success rate should be at least 80%")
	suite.LessOrEqual(totalDuration.Minutes(), 15.0, "Total test duration should be under 15 minutes")
	
	suite.T().Logf("Python comprehensive E2E test completed with %.1f%% success rate using %d design pattern scenarios", 
		overallSuccessRate, len(suite.patternScenarios))
}

// Test runner function
func TestPythonRealClientComprehensiveE2ETestSuite(t *testing.T) {
	suite.Run(t, new(PythonRealClientComprehensiveE2ETestSuite))
}