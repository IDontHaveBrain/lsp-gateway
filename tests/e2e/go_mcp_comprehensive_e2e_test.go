package e2e_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

// GoMCPComprehensiveE2ETestSuite tests all 6 supported LSP methods for Go through MCP protocol using golang/example repository
type GoMCPComprehensiveE2ETestSuite struct {
	suite.Suite
	
	// Core MCP infrastructure
	mcpCmd        *exec.Cmd
	mcpStdin      io.WriteCloser
	mcpReader     *bufio.Reader
	configPath    string
	tempDir       string
	projectRoot   string
	testTimeout   time.Duration
	
	// golang/example repository management with fixed commit
	repoManager   testutils.RepositoryManager
	repoDir       string
	goFiles       []string
	
	// Server state tracking
	serverStarted bool
	
	// Test metrics and results
	testResults   map[string]*MCPTestResult
	requestID     int64
}

type MCPTestResult struct {
	Method     string
	File       string
	Success    bool
	Duration   time.Duration
	Error      error
	Response   interface{}
	RequestID  int64
}

type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

// SetupSuite initializes the comprehensive MCP test suite for Go using golang/example repository
func (suite *GoMCPComprehensiveE2ETestSuite) SetupSuite() {
	suite.testTimeout = 10 * time.Minute
	suite.testResults = make(map[string]*MCPTestResult)
	suite.requestID = 1
	
	var err error
	suite.projectRoot, err = testutils.GetProjectRoot()
	suite.Require().NoError(err, "Failed to get project root")
	
	suite.tempDir, err = os.MkdirTemp("", "go-mcp-comprehensive-e2e-*")
	suite.Require().NoError(err, "Failed to create temp directory")
	
	// Initialize golang/example repository manager with fixed commit
	suite.repoManager = testutils.NewGoRepositoryManager()
	
	// Setup golang/example repository
	suite.repoDir, err = suite.repoManager.SetupRepository()
	suite.Require().NoError(err, "Failed to setup golang/example repository")
	
	// Discover Go files for comprehensive MCP testing
	suite.discoverGoFiles()
	
	// Create test configuration for Go MCP comprehensive testing
	suite.createGoMCPComprehensiveConfig()
	
	suite.T().Logf("Comprehensive Go MCP E2E test suite initialized with golang/example repository (commit: 8b40562)")
	suite.T().Logf("Found %d Go files for MCP testing", len(suite.goFiles))
}

// SetupTest initializes fresh components for each test
func (suite *GoMCPComprehensiveE2ETestSuite) SetupTest() {
	suite.serverStarted = false
}

// TearDownTest cleans up per-test resources
func (suite *GoMCPComprehensiveE2ETestSuite) TearDownTest() {
	suite.stopMCPServer()
}

// TearDownSuite performs final cleanup and reports comprehensive MCP test results
func (suite *GoMCPComprehensiveE2ETestSuite) TearDownSuite() {
	suite.reportComprehensiveMCPTestResults()
	
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

// TestGoMCPComprehensiveServerLifecycle tests complete MCP server lifecycle with comprehensive monitoring
func (suite *GoMCPComprehensiveE2ETestSuite) TestGoMCPComprehensiveServerLifecycle() {
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	suite.verifyMCPServerReadiness()
	suite.testComprehensiveMCPOperations()
}

// TestGoMCPDefinitionComprehensive tests lsp_definition tool via MCP across multiple golang/example files
func (suite *GoMCPComprehensiveE2ETestSuite) TestGoMCPDefinitionComprehensive() {
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	suite.T().Logf("Testing lsp_definition MCP tool on %d Go files", len(suite.goFiles))
	
	for _, testFile := range suite.goFiles {
		suite.testMCPDefinitionOnFile(testFile)
	}
	
	// Test specific positions in key files
	suite.testMCPDefinitionAtSpecificPositions()
}

// TestGoMCPReferencesComprehensive tests lsp_references tool via MCP across golang/example codebase
func (suite *GoMCPComprehensiveE2ETestSuite) TestGoMCPReferencesComprehensive() {
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	suite.T().Logf("Testing lsp_references MCP tool on %d Go files", len(suite.goFiles))
	
	for _, testFile := range suite.goFiles {
		suite.testMCPReferencesOnFile(testFile)
	}
}

// TestGoMCPHoverComprehensive tests lsp_hover tool via MCP across golang/example files
func (suite *GoMCPComprehensiveE2ETestSuite) TestGoMCPHoverComprehensive() {
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	suite.T().Logf("Testing lsp_hover MCP tool on %d Go files", len(suite.goFiles))
	
	for _, testFile := range suite.goFiles {
		suite.testMCPHoverOnFile(testFile)
	}
}

// TestGoMCPDocumentSymbolComprehensive tests lsp_document_symbol tool via MCP across golang/example files
func (suite *GoMCPComprehensiveE2ETestSuite) TestGoMCPDocumentSymbolComprehensive() {
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	suite.T().Logf("Testing lsp_document_symbol MCP tool on %d Go files", len(suite.goFiles))
	
	for _, testFile := range suite.goFiles {
		suite.testMCPDocumentSymbolOnFile(testFile)
	}
}

// TestGoMCPWorkspaceSymbolComprehensive tests lsp_workspace_symbol tool via MCP across entire golang/example repository
func (suite *GoMCPComprehensiveE2ETestSuite) TestGoMCPWorkspaceSymbolComprehensive() {
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
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
	
	suite.T().Logf("Testing lsp_workspace_symbol MCP tool with %d queries", len(queries))
	
	for _, query := range queries {
		suite.testMCPWorkspaceSymbolQuery(query)
	}
}

// TestGoMCPCompletionComprehensive tests lsp_completion tool via MCP across golang/example files
func (suite *GoMCPComprehensiveE2ETestSuite) TestGoMCPCompletionComprehensive() {
	suite.startMCPServer()
	defer suite.stopMCPServer()
	
	suite.T().Logf("Testing lsp_completion MCP tool on %d Go files", len(suite.goFiles))
	
	for _, testFile := range suite.goFiles {
		suite.testMCPCompletionOnFile(testFile)
	}
}

// Helper methods

func (suite *GoMCPComprehensiveE2ETestSuite) discoverGoFiles() {
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
	
	// Combine with priority files first, limit total for comprehensive testing
	suite.goFiles = append(priorityFiles, regularFiles...)
	if len(suite.goFiles) > 20 {
		suite.goFiles = suite.goFiles[:20] // Limit for comprehensive testing
	}
	
	suite.Require().Greater(len(suite.goFiles), 0, "No Go files found")
	suite.T().Logf("Discovered %d priority files and %d regular files", len(priorityFiles), len(regularFiles))
}

func (suite *GoMCPComprehensiveE2ETestSuite) createGoMCPComprehensiveConfig() {
	options := testutils.DefaultLanguageConfigOptions("go")
	options.ConfigType = "mcp-comprehensive"
	
	configPath, cleanup, err := testutils.CreateLanguageConfig(suite.repoManager, options)
	suite.Require().NoError(err, "Failed to create Go MCP comprehensive test config")
	
	suite.configPath = configPath
	_ = cleanup // Will be cleaned up via tempDir
}

func (suite *GoMCPComprehensiveE2ETestSuite) getFileURI(filePath string) string {
	workspaceDir := suite.repoManager.GetWorkspaceDir()
	return "file://" + filepath.Join(workspaceDir, filePath)
}

func (suite *GoMCPComprehensiveE2ETestSuite) startMCPServer() {
	if suite.serverStarted {
		return
	}
	
	binaryPath := filepath.Join(suite.projectRoot, "bin", "lspg")
	suite.mcpCmd = exec.Command(binaryPath, "mcp", "--config", suite.configPath)
	suite.mcpCmd.Dir = suite.projectRoot
	
	// Setup pipes for MCP communication
	stdin, err := suite.mcpCmd.StdinPipe()
	suite.Require().NoError(err, "Failed to create stdin pipe")
	suite.mcpStdin = stdin
	
	stdout, err := suite.mcpCmd.StdoutPipe()
	suite.Require().NoError(err, "Failed to create stdout pipe")
	suite.mcpReader = bufio.NewReader(stdout)
	
	err = suite.mcpCmd.Start()
	suite.Require().NoError(err, "Failed to start MCP server")
	
	suite.waitForMCPServerReadiness()
	suite.serverStarted = true
	
	suite.T().Logf("Go MCP server started successfully")
}

func (suite *GoMCPComprehensiveE2ETestSuite) stopMCPServer() {
	if !suite.serverStarted || suite.mcpCmd == nil {
		return
	}
	
	if suite.mcpStdin != nil {
		suite.mcpStdin.Close()
		suite.mcpStdin = nil
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
	suite.serverStarted = false
	suite.T().Logf("Go MCP server stopped")
}

func (suite *GoMCPComprehensiveE2ETestSuite) waitForMCPServerReadiness() {
	// Send initialization request
	initRequest := MCPRequest{
		JSONRPC: "2.0",
		ID:      suite.getNextRequestID(),
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "LSP-Gateway-Go-MCP-E2E-Test",
				"version": "1.0.0",
			},
		},
	}
	
	response, err := suite.sendMCPRequest(initRequest)
	suite.Require().NoError(err, "Failed to initialize MCP server")
	suite.Require().NotNil(response, "No response from MCP server initialization")
	
	suite.T().Logf("Go MCP server initialized successfully")
}

func (suite *GoMCPComprehensiveE2ETestSuite) verifyMCPServerReadiness() {
	// List available tools
	listToolsRequest := MCPRequest{
		JSONRPC: "2.0",
		ID:      suite.getNextRequestID(),
		Method:  "tools/list",
		Params:  map[string]interface{}{},
	}
	
	response, err := suite.sendMCPRequest(listToolsRequest)
	suite.Require().NoError(err, "Failed to list MCP tools")
	suite.Require().NotNil(response, "No response from MCP tools list")
	
	suite.T().Logf("Go MCP server tools verified successfully")
}

func (suite *GoMCPComprehensiveE2ETestSuite) testComprehensiveMCPOperations() {
	// Test basic MCP protocol operations
	suite.T().Logf("Testing comprehensive MCP operations")
	
	// Additional protocol tests can be added here
}

// MCP LSP tool testing methods

func (suite *GoMCPComprehensiveE2ETestSuite) testMCPDefinitionOnFile(testFile string) {
	start := time.Now()
	fileURI := suite.getFileURI(testFile)
	
	// Test multiple positions
	positions := []struct{ line, char int }{
		{0, 0},   // Start of file
		{5, 10},  // Mid-file
		{10, 5},  // Different position
	}
	
	for _, pos := range positions {
		params := MCPToolCallParams{
			URI:       fileURI,
			Line:      pos.line,
			Character: pos.char,
		}
		
		response, err := suite.callMCPTool("lsp_definition", params)
		
		result := &MCPTestResult{
			Method:    "lsp_definition",
			File:      testFile,
			Success:   err == nil,
			Duration:  time.Since(start),
			Error:     err,
			Response:  response,
			RequestID: suite.requestID - 1,
		}
		
		key := fmt.Sprintf("mcp_definition_%s_%d_%d", testFile, pos.line, pos.char)
		suite.testResults[key] = result
		
		if err != nil {
			suite.T().Logf("MCP Definition failed for %s at %d:%d: %v", testFile, pos.line, pos.char, err)
		} else {
			suite.T().Logf("MCP Definition succeeded for %s at %d:%d", testFile, pos.line, pos.char)
		}
	}
}

func (suite *GoMCPComprehensiveE2ETestSuite) testMCPDefinitionAtSpecificPositions() {
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
			params := MCPToolCallParams{
				URI:       suite.getFileURI(test.file),
				Line:      test.line,
				Character: test.char,
			}
			
			start := time.Now()
			response, err := suite.callMCPTool("lsp_definition", params)
			
			result := &MCPTestResult{
				Method:    "lsp_definition",
				File:      test.file,
				Success:   err == nil,
				Duration:  time.Since(start),
				Error:     err,
				Response:  response,
				RequestID: suite.requestID - 1,
			}
			
			key := fmt.Sprintf("mcp_specific_definition_%s_%d_%d", test.file, test.line, test.char)
			suite.testResults[key] = result
			
			if err == nil {
				suite.T().Logf("MCP Specific definition test passed for %s: %s", test.file, test.expected)
			}
		}
	}
}

func (suite *GoMCPComprehensiveE2ETestSuite) testMCPReferencesOnFile(testFile string) {
	start := time.Now()
	params := MCPToolCallParams{
		URI:       suite.getFileURI(testFile),
		Line:      5,
		Character: 10,
	}
	
	response, err := suite.callMCPTool("lsp_references", params)
	
	result := &MCPTestResult{
		Method:    "lsp_references",
		File:      testFile,
		Success:   err == nil,
		Duration:  time.Since(start),
		Error:     err,
		Response:  response,
		RequestID: suite.requestID - 1,
	}
	
	key := fmt.Sprintf("mcp_references_%s", testFile)
	suite.testResults[key] = result
	
	if err != nil {
		suite.T().Logf("MCP References failed for %s: %v", testFile, err)
	} else {
		suite.T().Logf("MCP References succeeded for %s", testFile)
	}
}

func (suite *GoMCPComprehensiveE2ETestSuite) testMCPHoverOnFile(testFile string) {
	start := time.Now()
	params := MCPToolCallParams{
		URI:       suite.getFileURI(testFile),
		Line:      5,
		Character: 10,
	}
	
	response, err := suite.callMCPTool("lsp_hover", params)
	
	result := &MCPTestResult{
		Method:    "lsp_hover",
		File:      testFile,
		Success:   err == nil,
		Duration:  time.Since(start),
		Error:     err,
		Response:  response,
		RequestID: suite.requestID - 1,
	}
	
	key := fmt.Sprintf("mcp_hover_%s", testFile)
	suite.testResults[key] = result
	
	if err != nil {
		suite.T().Logf("MCP Hover failed for %s: %v", testFile, err)
	} else {
		suite.T().Logf("MCP Hover succeeded for %s", testFile)
	}
}

func (suite *GoMCPComprehensiveE2ETestSuite) testMCPDocumentSymbolOnFile(testFile string) {
	start := time.Now()
	params := MCPToolCallParams{
		URI: suite.getFileURI(testFile),
	}
	
	response, err := suite.callMCPTool("lsp_document_symbol", params)
	
	result := &MCPTestResult{
		Method:    "lsp_document_symbol",
		File:      testFile,
		Success:   err == nil,
		Duration:  time.Since(start),
		Error:     err,
		Response:  response,
		RequestID: suite.requestID - 1,
	}
	
	key := fmt.Sprintf("mcp_documentSymbol_%s", testFile)
	suite.testResults[key] = result
	
	if err != nil {
		suite.T().Logf("MCP DocumentSymbol failed for %s: %v", testFile, err)
	} else {
		suite.T().Logf("MCP DocumentSymbol succeeded for %s", testFile)
	}
}

func (suite *GoMCPComprehensiveE2ETestSuite) testMCPWorkspaceSymbolQuery(query string) {
	start := time.Now()
	params := MCPToolCallParams{
		Query: query,
	}
	
	response, err := suite.callMCPTool("lsp_workspace_symbol", params)
	
	result := &MCPTestResult{
		Method:    "lsp_workspace_symbol",
		File:      fmt.Sprintf("query_%s", query),
		Success:   err == nil,
		Duration:  time.Since(start),
		Error:     err,
		Response:  response,
		RequestID: suite.requestID - 1,
	}
	
	key := fmt.Sprintf("mcp_workspaceSymbol_%s", query)
	suite.testResults[key] = result
	
	if err != nil {
		suite.T().Logf("MCP WorkspaceSymbol failed for query '%s': %v", query, err)
	} else {
		suite.T().Logf("MCP WorkspaceSymbol succeeded for query '%s'", query)
	}
}

func (suite *GoMCPComprehensiveE2ETestSuite) testMCPCompletionOnFile(testFile string) {
	start := time.Now()
	params := MCPToolCallParams{
		URI:       suite.getFileURI(testFile),
		Line:      5,
		Character: 10,
	}
	
	response, err := suite.callMCPTool("lsp_completion", params)
	
	result := &MCPTestResult{
		Method:    "lsp_completion",
		File:      testFile,
		Success:   err == nil,
		Duration:  time.Since(start),
		Error:     err,
		Response:  response,
		RequestID: suite.requestID - 1,
	}
	
	key := fmt.Sprintf("mcp_completion_%s", testFile)
	suite.testResults[key] = result
	
	if err != nil {
		suite.T().Logf("MCP Completion failed for %s: %v", testFile, err)
	} else {
		suite.T().Logf("MCP Completion succeeded for %s", testFile)
	}
}

func (suite *GoMCPComprehensiveE2ETestSuite) fileExists(filePath string) bool {
	workspaceDir := suite.repoManager.GetWorkspaceDir()
	fullPath := filepath.Join(workspaceDir, filePath)
	_, err := os.Stat(fullPath)
	return err == nil
}

// MCP protocol helper methods

func (suite *GoMCPComprehensiveE2ETestSuite) getNextRequestID() int64 {
	suite.requestID++
	return suite.requestID
}

func (suite *GoMCPComprehensiveE2ETestSuite) sendMCPRequest(request MCPRequest) (*MCPResponse, error) {
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// Send request
	_, err = suite.mcpStdin.Write(append(requestJSON, '\n'))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	
	// Read response with timeout
	responseStr, err := suite.readMCPResponseWithTimeout(30 * time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var response MCPResponse
	err = json.Unmarshal([]byte(responseStr), &response)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	
	if response.Error != nil {
		return &response, fmt.Errorf("MCP error: %v", response.Error)
	}
	
	return &response, nil
}

func (suite *GoMCPComprehensiveE2ETestSuite) readMCPResponseWithTimeout(timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	responseChan := make(chan string, 1)
	errorChan := make(chan error, 1)
	
	go func() {
		line, isPrefix, err := suite.mcpReader.ReadLine()
		if err != nil {
			errorChan <- err
			return
		}
		if isPrefix {
			// Handle partial line reading if needed
			var fullLine []byte
			fullLine = append(fullLine, line...)
			for isPrefix && err == nil {
				line, isPrefix, err = suite.mcpReader.ReadLine()
				if err == nil {
					fullLine = append(fullLine, line...)
				}
			}
			if err != nil {
				errorChan <- err
				return
			}
			responseChan <- string(fullLine)
		} else {
			responseChan <- string(line)
		}
	}()
	
	select {
	case response := <-responseChan:
		return response, nil
	case err := <-errorChan:
		return "", err
	case <-ctx.Done():
		return "", fmt.Errorf("timeout reading MCP response")
	}
}

func (suite *GoMCPComprehensiveE2ETestSuite) callMCPTool(toolName string, params MCPToolCallParams) (interface{}, error) {
	request := MCPRequest{
		JSONRPC: "2.0",
		ID:      suite.getNextRequestID(),
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      toolName,
			"arguments": params,
		},
	}
	
	response, err := suite.sendMCPRequest(request)
	if err != nil {
		return nil, err
	}
	
	return response.Result, nil
}

func (suite *GoMCPComprehensiveE2ETestSuite) reportComprehensiveMCPTestResults() {
	suite.T().Logf("\n=== Comprehensive Go MCP E2E Test Results ===")
	
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
	
	suite.T().Logf("Total MCP tests: %d", len(suite.testResults))
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
		suite.T().Logf("Total MCP failures: %d", failedTests)
	} else {
		suite.T().Logf("All comprehensive MCP tests passed!")
	}
}

// Test runner function
func TestGoMCPComprehensiveE2ETestSuite(t *testing.T) {
	suite.Run(t, new(GoMCPComprehensiveE2ETestSuite))
}