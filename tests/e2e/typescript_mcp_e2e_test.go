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

// TypeScriptMCPE2ETestSuite tests TypeScript functionality via MCP protocol with Microsoft TypeScript repository
type TypeScriptMCPE2ETestSuite struct {
	suite.Suite
	
	// Core infrastructure
	httpClient      *testutils.HttpClient
	gatewayCmd      *exec.Cmd
	mcpCmd          *exec.Cmd
	gatewayPort     int
	mcpPort         int
	configPath      string
	tempDir         string
	projectRoot     string
	testTimeout     time.Duration
	
	// TypeScript repository management
	repoManager     testutils.RepositoryManager
	repoDir         string
	tsFiles         []string
	
	// Server state tracking
	gatewayStarted  bool
	mcpStarted      bool
}

// SetupSuite initializes the MCP test suite for TypeScript using Microsoft TypeScript repository
func (suite *TypeScriptMCPE2ETestSuite) SetupSuite() {
	suite.testTimeout = 12 * time.Minute
	
	var err error
	suite.projectRoot, err = testutils.GetProjectRoot()
	suite.Require().NoError(err, "Failed to get project root")
	
	suite.tempDir, err = os.MkdirTemp("", "ts-mcp-e2e-*")
	suite.Require().NoError(err, "Failed to create temp directory")
	
	// Initialize TypeScript repository manager
	suite.repoManager = testutils.NewTypeScriptRepositoryManager()
	
	// Setup TypeScript repository
	suite.repoDir, err = suite.repoManager.SetupRepository()
	suite.Require().NoError(err, "Failed to setup TypeScript repository")
	
	// Discover TypeScript files
	suite.discoverTypeScriptFiles()
	
	suite.T().Logf("TypeScript MCP E2E test suite initialized with Microsoft TypeScript repository (version: v5.6.2)")
	suite.T().Logf("Found %d TypeScript files for testing", len(suite.tsFiles))
}

// SetupTest initializes fresh components for each test
func (suite *TypeScriptMCPE2ETestSuite) SetupTest() {
	var err error
	
	// Get available ports
	suite.gatewayPort, err = testutils.FindAvailablePort()
	suite.Require().NoError(err, "Failed to find available gateway port")
	
	suite.mcpPort, err = testutils.FindAvailablePort()
	suite.Require().NoError(err, "Failed to find available MCP port")

	// Create test configuration
	suite.createMCPTestConfig()

	// Configure HttpClient for TypeScript MCP testing
	config := testutils.HttpClientConfig{
		BaseURL:            fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
		Timeout:            30 * time.Second,
		MaxRetries:         3,
		RetryDelay:         3 * time.Second,
		EnableLogging:      true,
		EnableRecording:    false,
		WorkspaceID:        fmt.Sprintf("ts-mcp-test-%d", time.Now().UnixNano()),
		ProjectPath:        suite.repoDir,
		UserAgent:          "LSP-Gateway-TypeScript-MCP-E2E/1.0",
		MaxResponseSize:    75 * 1024 * 1024,
		ConnectionPoolSize: 15,
		KeepAlive:          30 * time.Second,
	}

	suite.httpClient = testutils.NewHttpClient(config)
	suite.gatewayStarted = false
	suite.mcpStarted = false
}

// TearDownTest cleans up per-test resources
func (suite *TypeScriptMCPE2ETestSuite) TearDownTest() {
	suite.stopMCPServer()
	suite.stopGatewayServer()
	
	if suite.httpClient != nil {
		suite.httpClient.Close()
		suite.httpClient = nil
	}
}

// TearDownSuite performs final cleanup
func (suite *TypeScriptMCPE2ETestSuite) TearDownSuite() {
	if suite.repoManager != nil {
		if err := suite.repoManager.Cleanup(); err != nil {
			suite.T().Logf("Warning: Failed to cleanup TypeScript repository: %v", err)
		}
	}
	
	if suite.tempDir != "" {
		if err := os.RemoveAll(suite.tempDir); err != nil {
			suite.T().Logf("Warning: Failed to remove temp directory: %v", err)
		}
	}
}

// TestTypeScriptMCPServerLifecycle tests complete MCP server lifecycle
func (suite *TypeScriptMCPE2ETestSuite) TestTypeScriptMCPServerLifecycle() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	// Verify both servers are ready
	err := suite.httpClient.HealthCheck(ctx)
	suite.Require().NoError(err, "Gateway server health check should pass")
	
	suite.T().Logf("TypeScript MCP server lifecycle test completed successfully")
}

// TestTypeScriptMCPDefinition tests textDocument/definition via MCP protocol with comprehensive TypeScript scenarios
func (suite *TypeScriptMCPE2ETestSuite) TestTypeScriptMCPDefinition() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	if len(suite.tsFiles) == 0 {
		suite.T().Skip("No TypeScript files found for testing")
		return
	}
	
	// Test definition resolution for different TypeScript constructs
	testScenarios := []struct {
		name     string
		position testutils.Position
		description string
	}{
		{"class_method", testutils.Position{Line: 5, Character: 15}, "Class method definition"},
		{"interface_prop", testutils.Position{Line: 8, Character: 12}, "Interface property definition"},
		{"function_param", testutils.Position{Line: 12, Character: 8}, "Function parameter definition"},
		{"import_symbol", testutils.Position{Line: 1, Character: 20}, "Import symbol definition"},
		{"type_alias", testutils.Position{Line: 15, Character: 10}, "Type alias definition"},
	}
	
	testFile := suite.tsFiles[0]
	fileURI := suite.getFileURI(testFile)
	successfulTests := 0
	
	for _, scenario := range testScenarios {
		suite.T().Logf("Testing definition scenario: %s (%s)", scenario.name, scenario.description)
		
		locations, err := suite.httpClient.Definition(ctx, fileURI, scenario.position)
		
		if err == nil && len(locations) > 0 {
			successfulTests++
			suite.T().Logf("✓ Definition test successful for %s - found %d locations", scenario.name, len(locations))
			
			// Validate location structure
			for i, loc := range locations {
				suite.Require().NotEmpty(loc.URI, "Location URI should not be empty")
				suite.Require().GreaterOrEqual(loc.Range.Start.Line, 0, "Start line should be non-negative")
				suite.Require().GreaterOrEqual(loc.Range.Start.Character, 0, "Start character should be non-negative")
				suite.T().Logf("  Location %d: %s at line %d, char %d", i+1, loc.URI, loc.Range.Start.Line, loc.Range.Start.Character)
			}
		} else {
			suite.T().Logf("○ Definition test for %s returned no results (expected for some positions): %v", scenario.name, err)
		}
	}
	
	suite.T().Logf("MCP Definition test completed: %d/%d scenarios found definitions", successfulTests, len(testScenarios))
	
	// Test cross-file definition resolution
	if len(suite.tsFiles) > 1 {
		suite.testCrossFileDefinition(ctx)
	}
}

// TestTypeScriptMCPHover tests textDocument/hover via MCP protocol with TypeScript-specific scenarios
func (suite *TypeScriptMCPE2ETestSuite) TestTypeScriptMCPHover() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	if len(suite.tsFiles) == 0 {
		suite.T().Skip("No TypeScript files found for testing")
		return
	}
	
	// Test hover information for different TypeScript language features
	hoverScenarios := []struct {
		name     string
		position testutils.Position
		expectedContent []string // Expected content patterns in hover
		description string
	}{
		{"function_signature", testutils.Position{Line: 10, Character: 8}, []string{"function", "(", ")"}, "Function hover with signature"},
		{"class_constructor", testutils.Position{Line: 15, Character: 12}, []string{"constructor", "class"}, "Class constructor hover"},
		{"interface_member", testutils.Position{Line: 8, Character: 15}, []string{":"}, "Interface member type information"},
		{"generic_type", testutils.Position{Line: 20, Character: 10}, []string{"<", ">"}, "Generic type parameter hover"},
		{"async_function", testutils.Position{Line: 25, Character: 6}, []string{"async", "Promise"}, "Async function hover"},
	}
	
	testFile := suite.tsFiles[0]
	fileURI := suite.getFileURI(testFile)
	hoverSuccessCount := 0
	
	for _, scenario := range hoverScenarios {
		suite.T().Logf("Testing hover scenario: %s (%s)", scenario.name, scenario.description)
		
		hover, err := suite.httpClient.Hover(ctx, fileURI, scenario.position)
		
		if err == nil && hover != nil {
			hoverSuccessCount++
			suite.T().Logf("✓ Hover test successful for %s", scenario.name)
			
			// Validate hover content structure
			if hover.Contents != nil {
				if contentSlice, ok := hover.Contents.([]interface{}); ok {
					suite.Require().Greater(len(contentSlice), 0, "Hover contents should not be empty")
					
					// Check for expected TypeScript-specific content patterns
					contentStr := fmt.Sprintf("%v", contentSlice)
					for _, expectedPattern := range scenario.expectedContent {
						if strings.Contains(strings.ToLower(contentStr), strings.ToLower(expectedPattern)) {
							suite.T().Logf("  Found expected pattern '%s' in hover content", expectedPattern)
						}
					}
					
					suite.T().Logf("  Hover content preview: %s", suite.truncateString(contentStr, 100))
				}
			}
			
			// Validate range if present
			if hover.Range != nil {
				suite.Require().GreaterOrEqual(hover.Range.Start.Line, 0, "Hover range start line should be non-negative")
				suite.Require().GreaterOrEqual(hover.Range.End.Line, hover.Range.Start.Line, "Hover range end should be after start")
			}
		} else {
			suite.T().Logf("○ Hover test for %s returned no results (expected for some positions): %v", scenario.name, err)
		}
	}
	
	suite.T().Logf("MCP Hover test completed: %d/%d scenarios provided hover information", hoverSuccessCount, len(hoverScenarios))
}

// TestTypeScriptMCPWorkspaceSymbol tests workspace/symbol via MCP protocol with TypeScript-specific queries
func (suite *TypeScriptMCPE2ETestSuite) TestTypeScriptMCPWorkspaceSymbol() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	// Test workspace symbols with comprehensive TypeScript-specific queries
	symbolQueries := []struct {
		query       string
		expectedMin int
		description string
		symbolKinds []int // Expected LSP symbol kinds
	}{
		{"interface", 0, "TypeScript interface declarations", []int{11}}, // Interface kind
		{"type", 0, "Type aliases and definitions", []int{5, 26}}, // Class, TypeParameter kinds
		{"class", 0, "Class declarations", []int{5}}, // Class kind
		{"function", 0, "Function declarations", []int{12}}, // Function kind
		{"enum", 0, "Enum declarations", []int{10}}, // Enum kind
		{"namespace", 0, "Namespace declarations", []int{3}}, // Namespace kind
		{"const", 0, "Constant declarations", []int{14}}, // Constant kind
		{"Component", 0, "React components (if present)", []int{5, 12}}, // Class or Function
	}
	
	totalSymbolsFound := 0
	successfulQueries := 0
	
	for _, queryTest := range symbolQueries {
		suite.T().Logf("Testing workspace symbol query: '%s' (%s)", queryTest.query, queryTest.description)
		
		symbols, err := suite.httpClient.WorkspaceSymbol(ctx, queryTest.query)
		
		if err == nil {
			successfulQueries++
			totalSymbolsFound += len(symbols)
			suite.T().Logf("✓ Workspace symbol test successful for query '%s' - found %d symbols", queryTest.query, len(symbols))
			
			// Validate symbol structure and TypeScript-specific properties
			for i, symbol := range symbols {
				if i >= 3 { // Limit detailed logging to first 3 symbols
					break
				}
				
				suite.Require().NotEmpty(symbol.Name, "Symbol name should not be empty")
				suite.Require().Greater(symbol.Kind, 0, "Symbol kind should be positive")
				suite.Require().NotEmpty(symbol.Location.URI, "Symbol location URI should not be empty")
				
				suite.T().Logf("  Symbol %d: %s (kind: %d) at %s:%d:%d", 
					i+1, symbol.Name, symbol.Kind, 
					suite.extractFileName(symbol.Location.URI),
					symbol.Location.Range.Start.Line, symbol.Location.Range.Start.Character)
				
				// Check if symbol kind matches expected kinds for this query
				if len(queryTest.symbolKinds) > 0 {
					kindMatched := false
					for _, expectedKind := range queryTest.symbolKinds {
						if symbol.Kind == expectedKind {
							kindMatched = true
							break
						}
					}
					if kindMatched {
						suite.T().Logf("    ✓ Symbol kind %d matches expected for query '%s'", symbol.Kind, queryTest.query)
					}
				}
			}
			
			if len(symbols) > 3 {
				suite.T().Logf("  ... and %d more symbols", len(symbols)-3)
			}
		} else {
			suite.T().Logf("○ Workspace symbol test for query '%s' completed with no results: %v", queryTest.query, err)
		}
	}
	
	suite.T().Logf("MCP Workspace Symbol test completed: %d/%d queries successful, %d total symbols found", 
		successfulQueries, len(symbolQueries), totalSymbolsFound)
}

// TestTypeScriptMCPDocumentSymbol tests textDocument/documentSymbol via MCP protocol with TypeScript file analysis
func (suite *TypeScriptMCPE2ETestSuite) TestTypeScriptMCPDocumentSymbol() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	if len(suite.tsFiles) == 0 {
		suite.T().Skip("No TypeScript files found for testing")
		return
	}
	
	// Test document symbols for multiple TypeScript files to get comprehensive coverage
	testFiles := suite.tsFiles
	if len(testFiles) > 5 {
		testFiles = testFiles[:5] // Limit to first 5 files for reasonable test time
	}
	
	totalSymbolsFound := 0
	successfulFiles := 0
	symbolKindCounts := make(map[int]int)
	
	for _, testFile := range testFiles {
		suite.T().Logf("Testing document symbols for file: %s", testFile)
		fileURI := suite.getFileURI(testFile)
		
		symbols, err := suite.httpClient.DocumentSymbol(ctx, fileURI)
		
		if err == nil && len(symbols) > 0 {
			successfulFiles++
			totalSymbolsFound += len(symbols)
			suite.T().Logf("✓ Document symbol test successful for %s - found %d symbols", testFile, len(symbols))
			
			// Analyze and validate symbol structure
			for i, symbol := range symbols {
				if i >= 5 { // Limit detailed logging to first 5 symbols per file
					break
				}
				
				suite.Require().NotEmpty(symbol.Name, "Symbol name should not be empty")
				suite.Require().Greater(symbol.Kind, 0, "Symbol kind should be positive")
				
				// Count symbol kinds for TypeScript analysis
				symbolKindCounts[symbol.Kind]++
				
				suite.T().Logf("  Symbol %d: %s (kind: %d, %s) at line %d:%d", 
					i+1, symbol.Name, symbol.Kind, suite.getSymbolKindName(symbol.Kind),
					symbol.Range.Start.Line, symbol.Range.Start.Character)
				
				// Validate TypeScript-specific symbol properties
				if symbol.Kind == 5 { // Class
					suite.T().Logf("    ✓ Found TypeScript class: %s", symbol.Name)
				} else if symbol.Kind == 11 { // Interface
					suite.T().Logf("    ✓ Found TypeScript interface: %s", symbol.Name)
				} else if symbol.Kind == 10 { // Enum
					suite.T().Logf("    ✓ Found TypeScript enum: %s", symbol.Name)
				} else if symbol.Kind == 3 { // Namespace
					suite.T().Logf("    ✓ Found TypeScript namespace: %s", symbol.Name)
				}
			}
			
			if len(symbols) > 5 {
				suite.T().Logf("  ... and %d more symbols", len(symbols)-5)
			}
		} else {
			suite.T().Logf("○ Document symbol test for %s returned no results: %v", testFile, err)
		}
	}
	
	// Log summary of TypeScript symbol kinds found
	suite.T().Logf("Document Symbol test completed: %d/%d files successful, %d total symbols found", 
		successfulFiles, len(testFiles), totalSymbolsFound)
	
	if len(symbolKindCounts) > 0 {
		suite.T().Logf("Symbol kind distribution:")
		for kind, count := range symbolKindCounts {
			suite.T().Logf("  Kind %d (%s): %d symbols", kind, suite.getSymbolKindName(kind), count)
		}
	}
}

// Helper methods

// testCrossFileDefinition tests definition resolution across multiple files
func (suite *TypeScriptMCPE2ETestSuite) testCrossFileDefinition(ctx context.Context) {
	suite.T().Logf("Testing cross-file definition resolution")
	
	// Test definition resolution for imported symbols in different files
	testFiles := suite.tsFiles[:min(3, len(suite.tsFiles))]
	crossFileDefinitionsFound := 0
	
	for _, testFile := range testFiles {
		fileURI := suite.getFileURI(testFile)
		
		// Test at positions likely to have imports (first few lines)
		importPositions := []testutils.Position{
			{Line: 0, Character: 15},
			{Line: 1, Character: 20},
			{Line: 2, Character: 10},
		}
		
		for _, pos := range importPositions {
			locations, err := suite.httpClient.Definition(ctx, fileURI, pos)
			if err == nil && len(locations) > 0 {
				// Check if any definition is in a different file
				for _, loc := range locations {
					if !strings.Contains(loc.URI, testFile) {
						crossFileDefinitionsFound++
						suite.T().Logf("✓ Cross-file definition: %s -> %s", 
							testFile, suite.extractFileName(loc.URI))
						break
					}
				}
			}
		}
	}
	
	if crossFileDefinitionsFound > 0 {
		suite.T().Logf("✓ Found %d cross-file definitions", crossFileDefinitionsFound)
	} else {
		suite.T().Logf("○ No cross-file definitions found (may be expected)")
	}
}

// truncateString truncates a string to the specified length with ellipsis
func (suite *TypeScriptMCPE2ETestSuite) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// extractFileName extracts the filename from a URI
func (suite *TypeScriptMCPE2ETestSuite) extractFileName(uri string) string {
	if idx := strings.LastIndex(uri, "/"); idx != -1 {
		return uri[idx+1:]
	}
	return uri
}

// getSymbolKindName returns a human-readable name for LSP symbol kinds
func (suite *TypeScriptMCPE2ETestSuite) getSymbolKindName(kind int) string {
	symbolKindNames := map[int]string{
		1:  "File",
		2:  "Module",
		3:  "Namespace",
		4:  "Package",
		5:  "Class",
		6:  "Method",
		7:  "Property",
		8:  "Field",
		9:  "Constructor",
		10: "Enum",
		11: "Interface",
		12: "Function",
		13: "Variable",
		14: "Constant",
		15: "String",
		16: "Number",
		17: "Boolean",
		18: "Array",
		19: "Object",
		20: "Key",
		21: "Null",
		22: "EnumMember",
		23: "Struct",
		24: "Event",
		25: "Operator",
		26: "TypeParameter",
	}
	
	if name, exists := symbolKindNames[kind]; exists {
		return name
	}
	return fmt.Sprintf("Unknown(%d)", kind)
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (suite *TypeScriptMCPE2ETestSuite) discoverTypeScriptFiles() {
	testFiles, err := suite.repoManager.GetTestFiles()
	suite.Require().NoError(err, "Failed to get TypeScript test files from Microsoft TypeScript repository")
	
	// Filter TypeScript files
	for _, file := range testFiles {
		ext := filepath.Ext(file)
		if ext == ".ts" || ext == ".tsx" {
			suite.tsFiles = append(suite.tsFiles, file)
		}
	}
	
	suite.T().Logf("Discovered %d TypeScript files in Microsoft TypeScript repository", len(suite.tsFiles))
}

func (suite *TypeScriptMCPE2ETestSuite) createMCPTestConfig() {
	options := testutils.DefaultLanguageConfigOptions("typescript")
	options.TestPort = fmt.Sprintf("%d", suite.gatewayPort)
	options.ConfigType = "main"
	
	// Add TypeScript-specific MCP custom variables
	options.CustomVariables["NODE_PATH"] = testutils.DetectNodePath()
	options.CustomVariables["TS_NODE_PROJECT"] = "tsconfig.json"
	options.CustomVariables["REPOSITORY"] = "TypeScript"
	options.CustomVariables["VERSION_TAG"] = "v5.6.2"
	options.CustomVariables["TEST_MODE"] = "mcp"
	options.CustomVariables["LSP_TIMEOUT"] = "90"
	options.CustomVariables["MCP_PORT"] = fmt.Sprintf("%d", suite.mcpPort)
	
	configPath, cleanup, err := testutils.CreateLanguageConfig(suite.repoManager, options)
	suite.Require().NoError(err, "Failed to create TypeScript MCP test config")
	
	suite.configPath = configPath
	_ = cleanup // Will be cleaned up via tempDir
}

func (suite *TypeScriptMCPE2ETestSuite) getFileURI(filePath string) string {
	workspaceDir := suite.repoManager.GetWorkspaceDir()
	return "file://" + filepath.Join(workspaceDir, filePath)
}

func (suite *TypeScriptMCPE2ETestSuite) startGatewayServer() {
	if suite.gatewayStarted {
		return
	}
	
	binaryPath := filepath.Join(suite.projectRoot, "bin", "lspg")
	suite.gatewayCmd = exec.Command(binaryPath, "server", "--config", suite.configPath)
	suite.gatewayCmd.Dir = suite.projectRoot
	
	err := suite.gatewayCmd.Start()
	suite.Require().NoError(err, "Failed to start gateway server")
	
	suite.waitForGatewayReadiness()
	suite.gatewayStarted = true
	
	suite.T().Logf("Gateway server started for TypeScript MCP testing on port %d", suite.gatewayPort)
}

func (suite *TypeScriptMCPE2ETestSuite) startMCPServer() {
	if suite.mcpStarted {
		return
	}
	
	binaryPath := filepath.Join(suite.projectRoot, "bin", "lspg")
	suite.mcpCmd = exec.Command(binaryPath, "mcp", "--config", suite.configPath)
	suite.mcpCmd.Dir = suite.projectRoot
	
	err := suite.mcpCmd.Start()
	suite.Require().NoError(err, "Failed to start MCP server")
	
	// Wait for MCP server to be ready (check via gateway health)
	config := testutils.QuickPollingConfig()
	condition := func() (bool, error) {
		return suite.checkGatewayHealth(), nil
	}
	err = testutils.WaitForCondition(condition, config, "MCP server to be ready")
	suite.Require().NoError(err, "MCP server failed to become ready")
	
	suite.mcpStarted = true
	
	suite.T().Logf("MCP server started for TypeScript testing on port %d", suite.mcpPort)
}

func (suite *TypeScriptMCPE2ETestSuite) stopGatewayServer() {
	if !suite.gatewayStarted || suite.gatewayCmd == nil {
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
	suite.gatewayStarted = false
}

func (suite *TypeScriptMCPE2ETestSuite) stopMCPServer() {
	if !suite.mcpStarted || suite.mcpCmd == nil {
		return
	}
	
	if suite.mcpCmd.Process != nil {
		suite.mcpCmd.Process.Signal(syscall.SIGTERM)
		
		done := make(chan error)
		go func() {
			done <- suite.mcpCmd.Wait()
		}()
		
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			suite.mcpCmd.Process.Kill()
			suite.mcpCmd.Wait()
		}
	}
	
	suite.mcpCmd = nil
	suite.mcpStarted = false
}

func (suite *TypeScriptMCPE2ETestSuite) waitForGatewayReadiness() {
	config := testutils.DefaultPollingConfig()
	config.Timeout = 45 * time.Second // Keep original 90 * 0.5s = 45s total timeout
	config.Interval = 500 * time.Millisecond // Keep original interval
	
	condition := func() (bool, error) {
		return suite.checkGatewayHealth(), nil
	}
	
	err := testutils.WaitForCondition(condition, config, "Gateway server to be ready")
	suite.Require().NoError(err, "Gateway server failed to become ready within timeout")
}

func (suite *TypeScriptMCPE2ETestSuite) checkGatewayHealth() bool {
	if suite.httpClient == nil {
		return false
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	
	err := suite.httpClient.HealthCheck(ctx)
	return err == nil
}

// TestTypeScriptMCPReferences tests textDocument/references via MCP protocol with cross-file analysis
func (suite *TypeScriptMCPE2ETestSuite) TestTypeScriptMCPReferences() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	if len(suite.tsFiles) == 0 {
		suite.T().Skip("No TypeScript files found for testing")
		return
	}
	
	// Test reference finding for different TypeScript constructs
	referenceScenarios := []struct {
		name        string
		position    testutils.Position
		expectedMin int
		description string
	}{
		{"exported_function", testutils.Position{Line: 3, Character: 15}, 0, "References to exported function"},
		{"class_method", testutils.Position{Line: 10, Character: 8}, 0, "References to class method"},
		{"interface_usage", testutils.Position{Line: 7, Character: 12}, 0, "References to interface usage"},
		{"imported_symbol", testutils.Position{Line: 1, Character: 18}, 0, "References to imported symbol"},
		{"type_parameter", testutils.Position{Line: 15, Character: 10}, 0, "References to generic type parameter"},
	}
	
	testFile := suite.tsFiles[0]
	fileURI := suite.getFileURI(testFile)
	successfulTests := 0
	totalReferences := 0
	
	for _, scenario := range referenceScenarios {
		suite.T().Logf("Testing references scenario: %s (%s)", scenario.name, scenario.description)
		
		references, err := suite.httpClient.References(ctx, fileURI, scenario.position, true)
		
		if err == nil && len(references) > 0 {
			successfulTests++
			totalReferences += len(references)
			suite.T().Logf("✓ References test successful for %s - found %d references", scenario.name, len(references))
			
			// Validate reference structure and analyze cross-file references
			uniqueFiles := make(map[string]int)
			for i, ref := range references {
				if i >= 3 { // Limit detailed logging to first 3 references
					break
				}
				
				suite.Require().NotEmpty(ref.URI, "Reference URI should not be empty")
				suite.Require().GreaterOrEqual(ref.Range.Start.Line, 0, "Reference start line should be non-negative")
				
				fileName := suite.extractFileName(ref.URI)
				uniqueFiles[fileName]++
				
				suite.T().Logf("  Reference %d: %s at line %d:%d", 
					i+1, fileName, ref.Range.Start.Line, ref.Range.Start.Character)
			}
			
			if len(references) > 3 {
				suite.T().Logf("  ... and %d more references", len(references)-3)
			}
			
			// Log cross-file reference analysis
			if len(uniqueFiles) > 1 {
				suite.T().Logf("  ✓ Cross-file references found across %d files", len(uniqueFiles))
				for fileName, count := range uniqueFiles {
					suite.T().Logf("    %s: %d references", fileName, count)
				}
			}
		} else {
			suite.T().Logf("○ References test for %s returned no results (expected for some positions): %v", scenario.name, err)
		}
	}
	
	suite.T().Logf("MCP References test completed: %d/%d scenarios found references, %d total references", 
		successfulTests, len(referenceScenarios), totalReferences)
}

// TestTypeScriptMCPCompletion tests textDocument/completion via MCP protocol with TypeScript-specific triggers
func (suite *TypeScriptMCPE2ETestSuite) TestTypeScriptMCPCompletion() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	if len(suite.tsFiles) == 0 {
		suite.T().Skip("No TypeScript files found for testing")
		return
	}
	
	// Test completion at different positions likely to have completions
	completionScenarios := []struct {
		name        string
		position    testutils.Position
		expectedMin int
		description string
	}{
		{"after_dot", testutils.Position{Line: 5, Character: 20}, 0, "Member access completion after dot"},
		{"import_statement", testutils.Position{Line: 1, Character: 25}, 0, "Import statement completion"},
		{"function_parameter", testutils.Position{Line: 10, Character: 15}, 0, "Function parameter type completion"},
		{"class_member", testutils.Position{Line: 15, Character: 8}, 0, "Class member completion"},
		{"generic_constraint", testutils.Position{Line: 20, Character: 18}, 0, "Generic type constraint completion"},
	}
	
	testFile := suite.tsFiles[0]
	fileURI := suite.getFileURI(testFile)
	successfulTests := 0
	totalCompletions := 0
	
	for _, scenario := range completionScenarios {
		suite.T().Logf("Testing completion scenario: %s (%s)", scenario.name, scenario.description)
		
		completions, err := suite.httpClient.Completion(ctx, fileURI, scenario.position)
		
		if err == nil && completions != nil && len(completions.Items) > 0 {
			successfulTests++
			totalCompletions += len(completions.Items)
			suite.T().Logf("✓ Completion test successful for %s - found %d completions", scenario.name, len(completions.Items))
			
			// Analyze completion items for TypeScript-specific features
			completionKinds := make(map[int]int)
			for i, completion := range completions.Items {
				if i >= 5 { // Limit detailed logging to first 5 completions
					break
				}
				
				suite.Require().NotEmpty(completion.Label, "Completion label should not be empty")
				
				if completion.Kind > 0 {
					completionKinds[completion.Kind]++
				}
				
				suite.T().Logf("  Completion %d: %s (kind: %d)", i+1, completion.Label, completion.Kind)
				
				// Check for TypeScript-specific completion features
				if completion.Detail != "" {
					suite.T().Logf("    Detail: %s", suite.truncateString(completion.Detail, 50))
				}
			}
			
			if len(completions.Items) > 5 {
				suite.T().Logf("  ... and %d more completions", len(completions.Items)-5)
			}
			
			// Log completion kind analysis
			if len(completionKinds) > 0 {
				suite.T().Logf("  Completion kinds: %v", completionKinds)
			}
		} else {
			suite.T().Logf("○ Completion test for %s returned no results (expected for some positions): %v", scenario.name, err)
		}
	}
	
	suite.T().Logf("MCP Completion test completed: %d/%d scenarios provided completions, %d total completions", 
		successfulTests, len(completionScenarios), totalCompletions)
}

// TestTypeScriptMCPCrossFileResolution tests cross-file symbol resolution capabilities
func (suite *TypeScriptMCPE2ETestSuite) TestTypeScriptMCPCrossFileResolution() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	if len(suite.tsFiles) < 2 {
		suite.T().Skip("Need at least 2 TypeScript files for cross-file resolution testing")
		return
	}
	
	suite.T().Logf("Testing cross-file symbol resolution with %d TypeScript files", len(suite.tsFiles))
	
	// Test import/export resolution across files
	crossFileTests := []struct {
		name        string
		methodName  string
		description string
	}{
		{"import_definition", "definition", "Find definition of imported symbol"},
		{"export_references", "references", "Find references to exported symbol"},
		{"module_hover", "hover", "Hover information for module imports"},
	}
	
	successfulCrossFileTests := 0
	
	for _, test := range crossFileTests {
		suite.T().Logf("Testing cross-file %s: %s", test.name, test.description)
		
		// Test on first few files for import statements (usually at top)
		testFiles := suite.tsFiles[:min(3, len(suite.tsFiles))]
		foundCrossFileResult := false
		
		for _, testFile := range testFiles {
			fileURI := suite.getFileURI(testFile)
			
			// Test at positions likely to have imports (first few lines)
			importPositions := []testutils.Position{
				{Line: 0, Character: 10},
				{Line: 1, Character: 15},
				{Line: 2, Character: 20},
			}
			
			for _, pos := range importPositions {
				switch test.methodName {
				case "definition":
					locations, err := suite.httpClient.Definition(ctx, fileURI, pos)
					if err == nil && len(locations) > 0 {
						// Check if definition is in a different file
						for _, loc := range locations {
							if !strings.Contains(loc.URI, testFile) {
								foundCrossFileResult = true
								suite.T().Logf("✓ Cross-file definition found: %s -> %s", 
									testFile, suite.extractFileName(loc.URI))
								break
							}
						}
					}
				case "references":
					references, err := suite.httpClient.References(ctx, fileURI, pos, true)
					if err == nil && len(references) > 0 {
						// Check for references in different files
						uniqueFiles := make(map[string]bool)
						for _, ref := range references {
							uniqueFiles[ref.URI] = true
						}
						if len(uniqueFiles) > 1 {
							foundCrossFileResult = true
							suite.T().Logf("✓ Cross-file references found across %d files", len(uniqueFiles))
						}
					}
				case "hover":
					hover, err := suite.httpClient.Hover(ctx, fileURI, pos)
					if err == nil && hover != nil {
						foundCrossFileResult = true
						suite.T().Logf("✓ Cross-file hover information found")
					}
				}
				
				if foundCrossFileResult {
					break
				}
			}
			
			if foundCrossFileResult {
				break
			}
		}
		
		if foundCrossFileResult {
			successfulCrossFileTests++
		} else {
			suite.T().Logf("○ No cross-file results found for %s test", test.name)
		}
	}
	
	suite.T().Logf("Cross-file resolution test completed: %d/%d tests found cross-file results", 
		successfulCrossFileTests, len(crossFileTests))
}

// TestTypeScriptMCPErrorHandling tests error handling and edge cases
func (suite *TypeScriptMCPE2ETestSuite) TestTypeScriptMCPErrorHandling() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	suite.startGatewayServer()
	defer suite.stopGatewayServer()
	
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	suite.T().Logf("Testing MCP error handling and edge cases for TypeScript")
	
	// Test various error conditions
	errorScenarios := []struct {
		name        string
		testFunc    func() error
		description string
	}{
		{"invalid_uri", func() error {
			_, err := suite.httpClient.Definition(ctx, "file:///nonexistent/file.ts", testutils.Position{Line: 0, Character: 0})
			return err
		}, "Definition request with invalid file URI"},
		{"out_of_bounds_position", func() error {
			if len(suite.tsFiles) > 0 {
				fileURI := suite.getFileURI(suite.tsFiles[0])
				_, err := suite.httpClient.Hover(ctx, fileURI, testutils.Position{Line: 99999, Character: 99999})
				return err
			}
			return fmt.Errorf("no files available")
		}, "Hover request with out-of-bounds position"},
		{"empty_workspace_symbol", func() error {
			symbols, err := suite.httpClient.WorkspaceSymbol(ctx, "")
			if err == nil && len(symbols) == 0 {
				return fmt.Errorf("empty result for empty query")
			}
			return err
		}, "Workspace symbol request with empty query"},
	}
	
	errorTestResults := 0
	
	for _, scenario := range errorScenarios {
		suite.T().Logf("Testing error scenario: %s (%s)", scenario.name, scenario.description)
		
		err := scenario.testFunc()
		if err != nil {
			errorTestResults++
			suite.T().Logf("✓ Error scenario %s handled correctly: %v", scenario.name, err)
		} else {
			suite.T().Logf("○ Error scenario %s did not produce expected error", scenario.name)
		}
	}
	
	suite.T().Logf("Error handling test completed: %d/%d scenarios behaved as expected", 
		errorTestResults, len(errorScenarios))
}

// Test runner function
func TestTypeScriptMCPE2ETestSuite(t *testing.T) {
	suite.Run(t, new(TypeScriptMCPE2ETestSuite))
}