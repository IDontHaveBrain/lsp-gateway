package e2e_test

import (
	"context"
	"encoding/json"
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

// PythonPatternsE2ETestSuite provides comprehensive E2E tests for Python LSP features
// using real lspg server integration with the faif/python-patterns repository
type PythonPatternsE2ETestSuite struct {
	suite.Suite
	
	// Core infrastructure
	httpClient      *testutils.HttpClient
	gatewayCmd      *exec.Cmd
	gatewayPort     int
	configPath      string
	tempDir         string
	projectRoot     string
	testTimeout     time.Duration
	
	// Python-specific components
	repoManager     *testutils.PythonRepoManager
	repoDir         string
	pythonFiles     map[string]string
	
	// Performance and metrics
	performanceMetrics *PythonPatternsPerformanceMetrics
	testResults        *PythonPatternsTestResults
	
	// Server state tracking
	serverStarted   bool
	serverReadyTime time.Duration
	
	// Test coordination
	mu              sync.RWMutex
	testCount       int
	failureCount    int
}

type PythonPatternsPerformanceMetrics struct {
	ServerStartupTime     time.Duration
	RepositoryCloneTime   time.Duration
	FirstResponseTime     time.Duration
	AverageResponseTime   time.Duration
	PeakMemoryUsage      int64
	TotalRequests        int
	SuccessfulRequests   int
	FailedRequests       int
	LSPFeatureMetrics    map[string]*LSPFeatureMetrics
}

type LSPFeatureMetrics struct {
	RequestCount    int
	AverageLatency  time.Duration
	MinLatency      time.Duration
	MaxLatency      time.Duration
	SuccessRate     float64
	ErrorMessages   []string
}

type PythonPatternsTestResults struct {
	ServerLifecycleSuccess    bool
	RepositorySetupSuccess    bool
	DefinitionSuccess         bool
	ReferencesSuccess         bool
	HoverSuccess              bool
	DocumentSymbolSuccess     bool
	WorkspaceSymbolSuccess    bool
	CompletionSuccess         bool
	OverallSuccess            bool
	TestDuration              time.Duration
	TotalErrors               int
	DetailedResults           map[string]interface{}
}

type TestFileData struct {
	FilePath        string
	Classes         []ClassPosition
	Methods         []MethodPosition
	ExpectedSymbols []string
	TestPositions   []testutils.Position
}

type ClassPosition struct {
	Name      string
	Line      int
	Character int
	Kind      int
}

type MethodPosition struct {
	Name      string
	Line      int
	Character int
	Class     string
}

// Python patterns test data mappings
var PythonPatternTestDatabase = map[string]TestFileData{
	"builder": {
		FilePath: "patterns/creational/builder.py",
		Classes: []ClassPosition{
			{Name: "Building", Line: 24, Character: 6, Kind: 5},
			{Name: "House", Line: 43, Character: 6, Kind: 5},
			{Name: "Flat", Line: 51, Character: 6, Kind: 5},
			{Name: "ComplexBuilding", Line: 60, Character: 6, Kind: 5},
		},
		Methods: []MethodPosition{
			{Name: "build_floor", Line: 30, Character: 8, Class: "Building"},
			{Name: "build_size", Line: 33, Character: 8, Class: "Building"},
			{Name: "construct_building", Line: 73, Character: 0, Class: ""},
		},
		ExpectedSymbols: []string{"Building", "House", "build_floor", "build_size"},
		TestPositions: []testutils.Position{
			{Line: 24, Character: 6},
			{Line: 30, Character: 8},
			{Line: 73, Character: 0},
		},
	},
	"adapter": {
		FilePath: "patterns/structural/adapter.py",
		Classes: []ClassPosition{
			{Name: "Dog", Line: 34, Character: 6, Kind: 5},
			{Name: "Cat", Line: 42, Character: 6, Kind: 5},
			{Name: "Adapter", Line: 63, Character: 6, Kind: 5},
		},
		Methods: []MethodPosition{
			{Name: "bark", Line: 38, Character: 8, Class: "Dog"},
			{Name: "meow", Line: 45, Character: 8, Class: "Cat"},
			{Name: "make_noise", Line: 59, Character: 8, Class: "Car"},
			{Name: "__getattr__", Line: 75, Character: 8, Class: "Adapter"},
		},
		ExpectedSymbols: []string{"Adapter", "Dog", "Cat", "make_noise", "__getattr__"},
		TestPositions: []testutils.Position{
			{Line: 63, Character: 6},
			{Line: 75, Character: 8},
			{Line: 59, Character: 8},
		},
	},
	"observer": {
		FilePath: "patterns/behavioral/observer.py",
		Classes: []ClassPosition{
			{Name: "Subject", Line: 15, Character: 6, Kind: 5},
			{Name: "Data", Line: 30, Character: 6, Kind: 5},
			{Name: "HexViewer", Line: 45, Character: 6, Kind: 5},
			{Name: "DecimalViewer", Line: 50, Character: 6, Kind: 5},
		},
		Methods: []MethodPosition{
			{Name: "attach", Line: 19, Character: 8, Class: "Subject"},
			{Name: "detach", Line: 23, Character: 8, Class: "Subject"},
			{Name: "notify", Line: 27, Character: 8, Class: "Subject"},
			{Name: "update", Line: 46, Character: 8, Class: "HexViewer"},
		},
		ExpectedSymbols: []string{"Subject", "Observer", "attach", "detach", "notify", "update"},
		TestPositions: []testutils.Position{
			{Line: 15, Character: 6},
			{Line: 19, Character: 8},
			{Line: 27, Character: 8},
		},
	},
}

// SetupSuite initializes the test suite with Python patterns repository
func (suite *PythonPatternsE2ETestSuite) SetupSuite() {
	suite.testTimeout = 5 * time.Minute
	
	var err error
	suite.projectRoot, err = testutils.GetProjectRoot()
	suite.Require().NoError(err, "Failed to get project root")
	
	suite.tempDir, err = os.MkdirTemp("", "python-patterns-e2e-*")
	suite.Require().NoError(err, "Failed to create temp directory")
	
	// Initialize Python repository manager
	repoConfig := testutils.DefaultPythonRepoConfig()
	repoConfig.TargetDir = filepath.Join(suite.tempDir, "python-patterns")
	repoConfig.CloneTimeout = 300 * time.Second
	repoConfig.EnableLogging = true
	
	suite.repoManager = testutils.NewPythonRepoManager(repoConfig)
	
	// Clone and setup repository
	suite.repoDir, err = suite.repoManager.SetupRepository()
	suite.Require().NoError(err, "Failed to setup Python patterns repository")
	
	// Initialize performance metrics
	suite.performanceMetrics = &PythonPatternsPerformanceMetrics{
		LSPFeatureMetrics: make(map[string]*LSPFeatureMetrics),
	}
	
	// Initialize test results
	suite.testResults = &PythonPatternsTestResults{
		DetailedResults: make(map[string]interface{}),
	}
	
	// Discover Python files for testing
	suite.discoverPythonFiles()
	
	// Create test configuration
	suite.createTestConfig()
}

// SetupTest initializes fresh components for each test
func (suite *PythonPatternsE2ETestSuite) SetupTest() {
	var err error
	suite.gatewayPort, err = testutils.FindAvailablePort()
	suite.Require().NoError(err, "Failed to find available port")
	
	// Update config with new port
	suite.updateConfigPort()
	
	// Configure HttpClient for this test
	config := testutils.HttpClientConfig{
		BaseURL:            fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
		Timeout:            30 * time.Second,
		MaxRetries:         5,
		RetryDelay:         2 * time.Second,
		EnableLogging:      true,
		EnableRecording:    true,
		WorkspaceID:        fmt.Sprintf("python-patterns-test-%d", time.Now().UnixNano()),
		ProjectPath:        suite.repoDir,
		UserAgent:          "LSP-Gateway-Python-Patterns-E2E/1.0",
		MaxResponseSize:    50 * 1024 * 1024,
		ConnectionPoolSize: 15,
		KeepAlive:          60 * time.Second,
	}
	
	suite.httpClient = testutils.NewHttpClient(config)
	
	// Reset per-test state
	suite.serverStarted = false
	suite.serverReadyTime = 0
	suite.testCount++
}

// TearDownTest cleans up per-test resources
func (suite *PythonPatternsE2ETestSuite) TearDownTest() {
	suite.stopGatewayServer()
	
	if suite.httpClient != nil {
		suite.httpClient.Close()
		suite.httpClient = nil
	}
	
	suite.collectTestMetrics()
}

// TearDownSuite performs final cleanup
func (suite *PythonPatternsE2ETestSuite) TearDownSuite() {
	if suite.repoManager != nil {
		if err := suite.repoManager.Cleanup(); err != nil {
			suite.T().Logf("Warning: Failed to cleanup repository: %v", err)
		}
	}
	
	if suite.tempDir != "" {
		if err := os.RemoveAll(suite.tempDir); err != nil {
			suite.T().Logf("Warning: Failed to remove temp directory: %v", err)
		}
	}
	
	suite.reportTestSummary()
}

// TestPythonPatternsServerLifecycle tests the complete server lifecycle
func (suite *PythonPatternsE2ETestSuite) TestPythonPatternsServerLifecycle() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.verifyServerReadiness(ctx)
	suite.testBasicServerOperations(ctx)
	
	suite.testResults.ServerLifecycleSuccess = true
}

// TestDefinitionFeature tests textDocument/definition LSP method
func (suite *PythonPatternsE2ETestSuite) TestDefinitionFeature() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	success := suite.testLSPFeature(ctx, "definition", suite.executeDefinitionTests)
	suite.testResults.DefinitionSuccess = success
	suite.Assert().True(success, "Definition feature tests should pass")
}

// TestReferencesFeature tests textDocument/references LSP method
func (suite *PythonPatternsE2ETestSuite) TestReferencesFeature() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	success := suite.testLSPFeature(ctx, "references", suite.executeReferencesTests)
	suite.testResults.ReferencesSuccess = success
	suite.Assert().True(success, "References feature tests should pass")
}

// TestHoverFeature tests textDocument/hover LSP method
func (suite *PythonPatternsE2ETestSuite) TestHoverFeature() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	success := suite.testLSPFeature(ctx, "hover", suite.executeHoverTests)
	suite.testResults.HoverSuccess = success
	suite.Assert().True(success, "Hover feature tests should pass")
}

// TestDocumentSymbolFeature tests textDocument/documentSymbol LSP method
func (suite *PythonPatternsE2ETestSuite) TestDocumentSymbolFeature() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)  
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	success := suite.testLSPFeature(ctx, "documentSymbol", suite.executeDocumentSymbolTests)
	suite.testResults.DocumentSymbolSuccess = success
	suite.Assert().True(success, "Document symbol feature tests should pass")
}

// TestWorkspaceSymbolFeature tests workspace/symbol LSP method
func (suite *PythonPatternsE2ETestSuite) TestWorkspaceSymbolFeature() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	success := suite.testLSPFeature(ctx, "workspaceSymbol", suite.executeWorkspaceSymbolTests)
	suite.testResults.WorkspaceSymbolSuccess = success
	suite.Assert().True(success, "Workspace symbol feature tests should pass")
}

// TestCompletionFeature tests textDocument/completion LSP method
func (suite *PythonPatternsE2ETestSuite) TestCompletionFeature() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	success := suite.testLSPFeature(ctx, "completion", suite.executeCompletionTests)
	suite.testResults.CompletionSuccess = success
	suite.Assert().True(success, "Completion feature tests should pass")
}

// Server management helper methods
func (suite *PythonPatternsE2ETestSuite) startGatewayServer() {
	if suite.serverStarted {
		return
	}
	
	startTime := time.Now()
	
	binaryPath := filepath.Join(suite.projectRoot, "bin", "lspg")
	suite.gatewayCmd = exec.Command(binaryPath, "server", "--config", suite.configPath)
	suite.gatewayCmd.Dir = suite.projectRoot
	
	err := suite.gatewayCmd.Start()
	suite.Require().NoError(err, "Failed to start gateway server")
	
	suite.waitForServerReadiness()
	
	suite.serverStarted = true
	suite.serverReadyTime = time.Since(startTime)
	suite.performanceMetrics.ServerStartupTime = suite.serverReadyTime
}

func (suite *PythonPatternsE2ETestSuite) stopGatewayServer() {
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

func (suite *PythonPatternsE2ETestSuite) waitForServerReadiness() {
	maxRetries := 30
	backoffDelay := time.Second
	
	for i := 0; i < maxRetries; i++ {
		if suite.checkServerHealth() {
			return
		}
		
		time.Sleep(backoffDelay)
		backoffDelay = time.Duration(float64(backoffDelay) * 1.5)
		if backoffDelay > 10*time.Second {
			backoffDelay = 10 * time.Second
		}
	}
	
	suite.Require().Fail("Server failed to become ready within timeout")
}

func (suite *PythonPatternsE2ETestSuite) checkServerHealth() bool {
	if suite.httpClient == nil {
		return false
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err := suite.httpClient.HealthCheck(ctx)
	return err == nil
}

// LSP feature test implementations
func (suite *PythonPatternsE2ETestSuite) testLSPFeature(ctx context.Context, featureName string, testFunc func(context.Context) bool) bool {
	startTime := time.Now()
	
	metrics := &LSPFeatureMetrics{
		ErrorMessages: make([]string, 0),
	}
	
	defer func() {
		if suite.httpClient != nil {
			clientMetrics := suite.httpClient.GetMetrics()
			metrics.RequestCount = clientMetrics.TotalRequests
			metrics.AverageLatency = clientMetrics.AverageLatency
		}
		suite.performanceMetrics.LSPFeatureMetrics[featureName] = metrics
	}()
	
	return testFunc(ctx)
}

func (suite *PythonPatternsE2ETestSuite) executeDefinitionTests(ctx context.Context) bool {
	successCount := 0
	totalTests := 0
	
	for patternName, testData := range PythonPatternTestDatabase {
		fileURI := suite.getFileURI(testData.FilePath)
		
		for _, position := range testData.TestPositions {
			totalTests++
			
			locations, err := suite.httpClient.Definition(ctx, fileURI, position)
			if err != nil {
				suite.T().Logf("Definition request failed for %s at %v: %v", patternName, position, err)
				continue
			}
			
			if suite.validateDefinitionResponse(locations, testData) {
				successCount++
			}
		}
	}
	
	return successCount == totalTests && totalTests > 0
}

func (suite *PythonPatternsE2ETestSuite) executeReferencesTests(ctx context.Context) bool {
	successCount := 0
	totalTests := 0
	
	for patternName, testData := range PythonPatternTestDatabase {
		fileURI := suite.getFileURI(testData.FilePath)
		
		for _, position := range testData.TestPositions {
			totalTests++
			
			references, err := suite.httpClient.References(ctx, fileURI, position, true)
			if err != nil {
				suite.T().Logf("References request failed for %s at %v: %v", patternName, position, err)
				continue
			}
			
			if suite.validateReferencesResponse(references, testData) {
				successCount++
			}
		}
	}
	
	return successCount > 0 && totalTests > 0
}

func (suite *PythonPatternsE2ETestSuite) executeHoverTests(ctx context.Context) bool {
	successCount := 0
	totalTests := 0
	
	for patternName, testData := range PythonPatternTestDatabase {
		fileURI := suite.getFileURI(testData.FilePath)
		
		for _, position := range testData.TestPositions {
			totalTests++
			
			hoverResult, err := suite.httpClient.Hover(ctx, fileURI, position)
			if err != nil {
				suite.T().Logf("Hover request failed for %s at %v: %v", patternName, position, err)
				continue
			}
			
			if suite.validateHoverResponse(hoverResult, testData) {
				successCount++
			}
		}
	}
	
	return successCount > 0 && totalTests > 0
}

func (suite *PythonPatternsE2ETestSuite) executeDocumentSymbolTests(ctx context.Context) bool {
	successCount := 0
	totalTests := 0
	
	for patternName, testData := range PythonPatternTestDatabase {
		fileURI := suite.getFileURI(testData.FilePath)
		totalTests++
		
		symbols, err := suite.httpClient.DocumentSymbol(ctx, fileURI)
		if err != nil {
			suite.T().Logf("DocumentSymbol request failed for %s: %v", patternName, err)
			continue
		}
		
		if suite.validateDocumentSymbolResponse(symbols, testData) {
			successCount++
		}
	}
	
	return successCount == totalTests && totalTests > 0
}

func (suite *PythonPatternsE2ETestSuite) executeWorkspaceSymbolTests(ctx context.Context) bool {
	queries := []string{"Building", "Adapter", "Subject", "class", "def"}
	successCount := 0
	
	for _, query := range queries {
		symbols, err := suite.httpClient.WorkspaceSymbol(ctx, query)
		if err != nil {
			suite.T().Logf("WorkspaceSymbol request failed for query '%s': %v", query, err)
			continue
		}
		
		if len(symbols) > 0 {
			successCount++
		}
	}
	
	return successCount > 0
}

func (suite *PythonPatternsE2ETestSuite) executeCompletionTests(ctx context.Context) bool {
	successCount := 0
	totalTests := 0
	
	for patternName, testData := range PythonPatternTestDatabase {
		fileURI := suite.getFileURI(testData.FilePath)
		
		// Test completion at various positions
		for _, position := range testData.TestPositions {
			// Adjust position slightly for completion context
			completionPos := testutils.Position{
				Line:      position.Line + 1,
				Character: 0,
			}
			
			totalTests++
			
			completion, err := suite.httpClient.Completion(ctx, fileURI, completionPos)
			if err != nil {
				suite.T().Logf("Completion request failed for %s at %v: %v", patternName, completionPos, err)
				continue
			}
			
			if completion != nil && len(completion.Items) > 0 {
				successCount++
			}
		}
	}
	
	return successCount > 0 && totalTests > 0
}

// Validation helper methods
func (suite *PythonPatternsE2ETestSuite) validateDefinitionResponse(locations []testutils.Location, testData TestFileData) bool {
	if len(locations) == 0 {
		return false
	}
	
	for _, location := range locations {
		if strings.Contains(location.URI, testData.FilePath) {
			return true
		}
	}
	
	return false
}

func (suite *PythonPatternsE2ETestSuite) validateReferencesResponse(references []testutils.Location, testData TestFileData) bool {
	return len(references) > 0
}

func (suite *PythonPatternsE2ETestSuite) validateHoverResponse(hoverResult *testutils.HoverResult, testData TestFileData) bool {
	if hoverResult == nil || hoverResult.Contents == nil {
		return false
	}
	
	return true
}

func (suite *PythonPatternsE2ETestSuite) validateDocumentSymbolResponse(symbols []testutils.DocumentSymbol, testData TestFileData) bool {
	if len(symbols) == 0 {
		return false
	}
	
	foundSymbols := make(map[string]bool)
	for _, symbol := range symbols {
		foundSymbols[symbol.Name] = true
	}
	
	matchCount := 0
	for _, expectedSymbol := range testData.ExpectedSymbols {
		if foundSymbols[expectedSymbol] {
			matchCount++
		}
	}
	
	return matchCount > 0
}

// Configuration and utility methods
func (suite *PythonPatternsE2ETestSuite) createTestConfig() {
	configContent := fmt.Sprintf(`
servers:
- name: python-pylsp
  languages: ["python"]
  command: pylsp
  args: []
  transport: stdio
  root_markers: ["pyproject.toml", "requirements.txt", ".git"]
  priority: 1
  weight: 1.0

port: %d
timeout: 30s
max_concurrent_requests: 100
logging:
  level: debug
`, suite.gatewayPort)

	suite.configPath = filepath.Join(suite.tempDir, "test-config.yaml")
	err := os.WriteFile(suite.configPath, []byte(configContent), 0644)
	suite.Require().NoError(err, "Failed to create test config")
}

func (suite *PythonPatternsE2ETestSuite) updateConfigPort() {
	suite.createTestConfig()
}

func (suite *PythonPatternsE2ETestSuite) discoverPythonFiles() {
	suite.pythonFiles = make(map[string]string)
	
	err := filepath.Walk(suite.repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if strings.HasSuffix(path, ".py") && !strings.Contains(path, "__pycache__") {
			relPath, _ := filepath.Rel(suite.repoDir, path)
			content, readErr := os.ReadFile(path)
			if readErr == nil {
				suite.pythonFiles[relPath] = string(content)
			}
		}
		
		return nil
	})
	
	suite.Require().NoError(err, "Failed to discover Python files")
	suite.Require().Greater(len(suite.pythonFiles), 0, "No Python files found in repository")
}

func (suite *PythonPatternsE2ETestSuite) getFileURI(filePath string) string {
	return "file://" + filepath.Join(suite.repoDir, filePath)
}

func (suite *PythonPatternsE2ETestSuite) verifyServerReadiness(ctx context.Context) {
	err := suite.httpClient.HealthCheck(ctx)
	suite.Require().NoError(err, "Server health check should pass")
}

func (suite *PythonPatternsE2ETestSuite) testBasicServerOperations(ctx context.Context) {
	err := suite.httpClient.ValidateConnection(ctx)
	suite.Require().NoError(err, "Server connection validation should pass")
}

func (suite *PythonPatternsE2ETestSuite) collectTestMetrics() {
	if suite.httpClient != nil {
		metrics := suite.httpClient.GetMetrics()
		suite.performanceMetrics.TotalRequests += metrics.TotalRequests
		suite.performanceMetrics.SuccessfulRequests += metrics.SuccessfulReqs
		suite.performanceMetrics.FailedRequests += metrics.FailedRequests
		
		if metrics.AverageLatency > 0 {
			suite.performanceMetrics.AverageResponseTime = metrics.AverageLatency
		}
	}
}

func (suite *PythonPatternsE2ETestSuite) reportTestSummary() {
	suite.testResults.OverallSuccess = suite.testResults.DefinitionSuccess &&
		suite.testResults.ReferencesSuccess &&
		suite.testResults.HoverSuccess &&
		suite.testResults.DocumentSymbolSuccess &&
		suite.testResults.WorkspaceSymbolSuccess &&
		suite.testResults.CompletionSuccess
	
	summary := map[string]interface{}{
		"server_startup_time": suite.performanceMetrics.ServerStartupTime,
		"total_requests":      suite.performanceMetrics.TotalRequests,
		"success_rate":        float64(suite.performanceMetrics.SuccessfulRequests) / float64(suite.performanceMetrics.TotalRequests),
		"test_results":        suite.testResults,
	}
	
	summaryJSON, _ := json.MarshalIndent(summary, "", "  ")
	suite.T().Logf("Python Patterns E2E Test Summary:\n%s", string(summaryJSON))
}

// Test runner function
func TestPythonPatternsE2ETestSuite(t *testing.T) {
	suite.Run(t, new(PythonPatternsE2ETestSuite))
}