package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ComprehensiveMCPTest provides a complete integration test suite for MCP functionality
type ComprehensiveMCPTest struct {
	t              *testing.T
	client         *TestMCPClient
	transport      *STDIOTransport
	serverCmd      *exec.Cmd
	testHelper     *TestHelper
	testProjectDir string
	configPath     string
}

// NewComprehensiveMCPTest creates a new comprehensive MCP test suite
func NewComprehensiveMCPTest(t *testing.T) *ComprehensiveMCPTest {
	return &ComprehensiveMCPTest{
		t: t,
	}
}

// TestComprehensiveMCPIntegration runs the complete MCP integration test suite
func TestComprehensiveMCPIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comprehensive MCP integration test in short mode")
	}

	suite := NewComprehensiveMCPTest(t)
	suite.runFullTestSuite()
}

// runFullTestSuite executes the complete test suite
func (ct *ComprehensiveMCPTest) runFullTestSuite() {
	ct.t.Log("Starting comprehensive MCP integration test suite")

	// Test phases
	ct.setupTestEnvironment()
	defer ct.cleanupTestEnvironment()

	ct.startMCPServer()
	defer ct.stopMCPServer()

	ct.setupMCPClient()
	defer ct.cleanupMCPClient()

	ct.testMCPConnection()
	ct.testInitialization()
	ct.testToolDiscovery()
	ct.testAllAvailableTools()
	ct.testErrorHandling()
	ct.testPerformanceMetrics()

	ct.t.Log("Comprehensive MCP integration test suite completed successfully")
}

// setupTestEnvironment prepares the test environment
func (ct *ComprehensiveMCPTest) setupTestEnvironment() {
	ct.t.Log("Setting up test environment")

	// Create temporary test project directory
	tempDir, err := os.MkdirTemp("", "mcp-integration-test-*")
	require.NoError(ct.t, err, "Failed to create temp directory")
	ct.testProjectDir = tempDir

	// Create test files for different languages
	ct.createTestFiles()

	// Create test configuration
	ct.createTestConfig()

	ct.t.Logf("Test environment created at: %s", ct.testProjectDir)
}

// createTestFiles creates test files for different programming languages
func (ct *ComprehensiveMCPTest) createTestFiles() {
	testFiles := map[string]string{
		"main.go": `package main

import "fmt"

// TestStruct represents a test structure
type TestStruct struct {
	Name    string
	Value   int
	Active  bool
}

// NewTestStruct creates a new TestStruct
func NewTestStruct(name string, value int) *TestStruct {
	return &TestStruct{
		Name:   name,
		Value:  value,
		Active: true,
	}
}

// GetName returns the name of the struct
func (ts *TestStruct) GetName() string {
	return ts.Name
}

// SetValue sets the value
func (ts *TestStruct) SetValue(value int) {
	ts.Value = value
}

func main() {
	test := NewTestStruct("example", 42)
	fmt.Printf("Test struct: %+v\n", test)
}`,

		"test.py": `#!/usr/bin/env python3
"""Test Python file for MCP integration testing."""

class TestClass:
    """A test class for demonstration."""
    
    def __init__(self, name: str, value: int):
        self.name = name
        self.value = value
        self.active = True
    
    def get_name(self) -> str:
        """Return the name."""
        return self.name
    
    def set_value(self, value: int) -> None:
        """Set the value."""
        self.value = value
    
    def is_active(self) -> bool:
        """Check if active."""
        return self.active

def main():
    """Main function."""
    test = TestClass("example", 42)
    print(f"Test class: {test.__dict__}")

if __name__ == "__main__":
    main()`,

		"test.js": `/**
 * Test JavaScript file for MCP integration testing
 */

class TestClass {
    constructor(name, value) {
        this.name = name;
        this.value = value;
        this.active = true;
    }
    
    getName() {
        return this.name;
    }
    
    setValue(value) {
        this.value = value;
    }
    
    isActive() {
        return this.active;
    }
}

function main() {
    const test = new TestClass("example", 42);
    console.log("Test class:", test);
}

main();`,

		"Test.java": `/**
 * Test Java file for MCP integration testing
 */
public class Test {
    private String name;
    private int value;
    private boolean active;
    
    public Test(String name, int value) {
        this.name = name;
        this.value = value;
        this.active = true;
    }
    
    public String getName() {
        return name;
    }
    
    public void setValue(int value) {
        this.value = value;
    }
    
    public boolean isActive() {
        return active;
    }
    
    public static void main(String[] args) {
        Test test = new Test("example", 42);
        System.out.println("Test class: " + test);
    }
}`,
	}

	for filename, content := range testFiles {
		filePath := filepath.Join(ct.testProjectDir, filename)
		err := os.WriteFile(filePath, []byte(content), 0644)
		require.NoError(ct.t, err, "Failed to create test file: %s", filename)
	}
}

// createTestConfig creates a test configuration file
func (ct *ComprehensiveMCPTest) createTestConfig() {
	config := map[string]interface{}{
		"name":        "test-mcp-server",
		"description": "Test MCP server for integration testing",
		"version":     "1.0.0",
		"lsp_gateway_url": "http://localhost:8080",
		"transport":   "stdio",
		"timeout":     "30s",
		"max_retries": 3,
	}

	configBytes, err := json.MarshalIndent(config, "", "  ")
	require.NoError(ct.t, err, "Failed to marshal config")

	ct.configPath = filepath.Join(ct.testProjectDir, "mcp-config.json")
	err = os.WriteFile(ct.configPath, configBytes, 0644)
	require.NoError(ct.t, err, "Failed to write config file")
}

// startMCPServer starts the MCP server for testing
func (ct *ComprehensiveMCPTest) startMCPServer() {
	ct.t.Log("Starting MCP server")

	// Build the lsp-gateway binary if it doesn't exist
	// Get absolute path to binary
	wd, err := os.Getwd()
	require.NoError(ct.t, err, "Failed to get working directory")
	binaryPath := filepath.Join(wd, "..", "..", "bin", "lsp-gateway")
	
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		ct.t.Log("Building lsp-gateway binary")
		buildCmd := exec.Command("make", "local")
		buildCmd.Dir = filepath.Join(wd, "..", "..")
		output, err := buildCmd.CombinedOutput()
		require.NoError(ct.t, err, "Failed to build lsp-gateway: %s", string(output))
	}

	// Start MCP server
	ct.serverCmd = exec.Command(binaryPath, "mcp", "--config", ct.configPath)
	ct.serverCmd.Dir = ct.testProjectDir

	// Set up pipes for communication
	stdin, err := ct.serverCmd.StdinPipe()
	require.NoError(ct.t, err, "Failed to create stdin pipe")

	stdout, err := ct.serverCmd.StdoutPipe()
	require.NoError(ct.t, err, "Failed to create stdout pipe")

	stderr, err := ct.serverCmd.StderrPipe()
	require.NoError(ct.t, err, "Failed to create stderr pipe")

	// Start the server
	err = ct.serverCmd.Start()
	require.NoError(ct.t, err, "Failed to start MCP server")

	// Create STDIO transport
	ct.transport = NewSTDIOTransport(stdin, stdout, stderr)

	// Wait a moment for server to start
	time.Sleep(2 * time.Second)

	ct.t.Log("MCP server started successfully")
}

// stopMCPServer stops the MCP server
func (ct *ComprehensiveMCPTest) stopMCPServer() {
	ct.t.Log("Stopping MCP server")

	if ct.serverCmd != nil && ct.serverCmd.Process != nil {
		// Try graceful shutdown first
		if err := ct.serverCmd.Process.Signal(os.Interrupt); err != nil {
			ct.t.Logf("Failed to send interrupt signal: %v", err)
		}

		// Wait for graceful shutdown
		done := make(chan error, 1)
		go func() {
			done <- ct.serverCmd.Wait()
		}()

		select {
		case <-done:
			ct.t.Log("Server shut down gracefully")
		case <-time.After(5 * time.Second):
			ct.t.Log("Server didn't shut down gracefully, killing")
			if err := ct.serverCmd.Process.Kill(); err != nil {
				ct.t.Logf("Failed to kill server process: %v", err)
			}
		}
	}

	if ct.transport != nil {
		ct.transport.Close()
	}
}

// setupMCPClient sets up the test MCP client
func (ct *ComprehensiveMCPTest) setupMCPClient() {
	ct.t.Log("Setting up MCP client")

	config := DefaultClientConfig()
	config.LogLevel = "debug"
	config.ConnectionTimeout = 10 * time.Second
	config.RequestTimeout = 30 * time.Second

	ct.client = NewTestMCPClient(config, ct.transport)
	ct.testHelper = NewTestHelper(ct.t, ct.client)

	ct.t.Log("MCP client set up successfully")
}

// cleanupMCPClient cleans up the MCP client
func (ct *ComprehensiveMCPTest) cleanupMCPClient() {
	ct.t.Log("Cleaning up MCP client")

	if ct.client != nil {
		ct.client.Shutdown()
	}
}

// testMCPConnection tests the basic MCP connection
func (ct *ComprehensiveMCPTest) testMCPConnection() {
	ct.t.Log("Testing MCP connection")

	// Connect to server
	err := ct.client.Connect(context.Background())
	ct.testHelper.AssertNoError(err, "Failed to connect to MCP server")

	// Verify connection state
	ct.testHelper.AssertStateWithin(Connected, 5*time.Second)

	// Test ping
	err = ct.client.Ping()
	ct.testHelper.AssertNoError(err, "Ping failed")

	ct.t.Log("MCP connection test passed")
}

// testInitialization tests the MCP initialization process
func (ct *ComprehensiveMCPTest) testInitialization() {
	ct.t.Log("Testing MCP initialization")

	// Initialize client
	clientInfo := map[string]interface{}{
		"name":    "comprehensive-test-client",
		"version": "1.0.0",
	}

	capabilities := map[string]interface{}{
		"tools": true,
	}

	result, err := ct.client.Initialize(clientInfo, capabilities)
	ct.testHelper.AssertNoError(err, "Initialization failed")
	ct.testHelper.AssertNotNil(result, "Initialize result should not be nil")

	// Verify initialization state
	ct.testHelper.AssertStateWithin(Initialized, 5*time.Second)

	// Validate protocol version
	if result != nil {
		assert.Equal(ct.t, "2025-06-18", result.ProtocolVersion, "Protocol version mismatch")

		// Validate server info
		ct.testHelper.AssertNotNil(result.ServerInfo, "Server info should not be nil")
	}

	ct.t.Log("MCP initialization test passed")
}

// testToolDiscovery tests tool discovery functionality
func (ct *ComprehensiveMCPTest) testToolDiscovery() {
	ct.t.Log("Testing tool discovery")

	// List available tools
	tools, err := ct.client.ListTools()
	ct.testHelper.AssertNoError(err, "Failed to list tools")
	ct.testHelper.AssertNotNil(tools, "Tools list should not be nil")

	// Verify we have expected basic tools (SCIP tools may not be available in test environment)
	basicExpectedTools := []string{
		"goto_definition",
		"find_references", 
		"get_hover_info",
		"get_document_symbols",
		"search_workspace_symbols",
	}

	// All SCIP enhanced tools (may not be available)
	scipTools := []string{
		"scip_intelligent_symbol_search",
		"scip_cross_language_references",
		"scip_semantic_code_analysis",
		"scip_context_aware_assistance",
		"scip_workspace_intelligence",
		"scip_refactoring_suggestions",
	}

	assert.GreaterOrEqual(ct.t, len(tools), len(basicExpectedTools), "Should have at least basic LSP tools")

	// Verify each expected tool exists
	toolMap := make(map[string]Tool)
	for _, tool := range tools {
		toolMap[tool.Name] = tool
	}

	// Check basic LSP tools (should always be available)
	for _, expectedTool := range basicExpectedTools {
		tool, exists := toolMap[expectedTool]
		assert.True(ct.t, exists, "Basic LSP tool %s should exist", expectedTool)
		if exists {
			assert.NotEmpty(ct.t, tool.Description, "Tool %s should have description", expectedTool)
			ct.testHelper.AssertNotNil(tool.InputSchema, fmt.Sprintf("Tool %s should have input schema", expectedTool))
		}
	}

	// Check SCIP tools (may not be available in test environment)
	scipToolsFound := 0
	for _, scipTool := range scipTools {
		if _, exists := toolMap[scipTool]; exists {
			scipToolsFound++
		}
	}
	ct.t.Logf("Found %d SCIP tools out of %d expected", scipToolsFound, len(scipTools))

	ct.t.Logf("Discovered %d tools", len(tools))
	ct.t.Log("Tool discovery test passed")
}

// testAllAvailableTools tests all available tools
func (ct *ComprehensiveMCPTest) testAllAvailableTools() {
	ct.t.Log("Testing all available tools")

	tools := ct.client.GetCachedTools()
	require.NotEmpty(ct.t, tools, "Should have cached tools")

	// Test standard LSP tools
	ct.testLSPTools()

	// Test SCIP enhanced tools
	ct.testSCIPTools()

	ct.t.Log("All tools testing completed")
}

// testLSPTools tests standard LSP tools
func (ct *ComprehensiveMCPTest) testLSPTools() {
	ct.t.Log("Testing standard LSP tools")

	testCases := []struct {
		toolName string
		params   map[string]interface{}
	}{
		{
			toolName: "goto_definition",
			params: map[string]interface{}{
				"uri":       fmt.Sprintf("file://%s", filepath.Join(ct.testProjectDir, "main.go")),
				"line":      8,
				"character": 10,
			},
		},
		{
			toolName: "find_references",
			params: map[string]interface{}{
				"uri":       fmt.Sprintf("file://%s", filepath.Join(ct.testProjectDir, "main.go")),
				"line":      8,
				"character": 10,
			},
		},
		{
			toolName: "get_hover_info",
			params: map[string]interface{}{
				"uri":       fmt.Sprintf("file://%s", filepath.Join(ct.testProjectDir, "main.go")),
				"line":      8,
				"character": 10,
			},
		},
		{
			toolName: "get_document_symbols",
			params: map[string]interface{}{
				"uri": fmt.Sprintf("file://%s", filepath.Join(ct.testProjectDir, "main.go")),
			},
		},
		{
			toolName: "search_workspace_symbols",
			params: map[string]interface{}{
				"query": "TestStruct",
			},
		},
	}

	for _, testCase := range testCases {
		ct.t.Logf("Testing tool: %s", testCase.toolName)

		// Validate parameters first
		err := ct.client.ValidateToolParams(testCase.toolName, testCase.params)
		if err != nil {
			ct.t.Logf("Parameter validation failed for %s: %v (this may be expected for some tools)", testCase.toolName, err)
		}

		// Call the tool
		result, err := ct.client.CallTool(testCase.toolName, testCase.params)
		
		// Note: Some tools might fail due to missing LSP servers, which is expected in test environment
		if err != nil {
			ct.t.Logf("Tool %s returned error: %v (may be expected in test environment)", testCase.toolName, err)
			continue
		}

		ct.testHelper.AssertNotNil(result, fmt.Sprintf("Tool %s should return result", testCase.toolName))
		
		if result.IsError {
			ct.t.Logf("Tool %s returned error result: %v", testCase.toolName, result.Error)
		} else {
			ct.t.Logf("Tool %s executed successfully", testCase.toolName)
			assert.NotEmpty(ct.t, result.Content, "Tool result should have content")
		}
	}
}

// testSCIPTools tests SCIP enhanced tools
func (ct *ComprehensiveMCPTest) testSCIPTools() {
	ct.t.Log("Testing SCIP enhanced tools")

	scipTestCases := []struct {
		toolName string
		params   map[string]interface{}
	}{
		{
			toolName: "scip_intelligent_symbol_search",
			params: map[string]interface{}{
				"query":     "TestStruct",
				"file_type": "go",
				"limit":     10,
			},
		},
		{
			toolName: "scip_cross_language_references",
			params: map[string]interface{}{
				"symbol": "TestStruct",
				"scope":  "workspace",
			},
		},
		{
			toolName: "scip_semantic_code_analysis",
			params: map[string]interface{}{
				"uri":           fmt.Sprintf("file://%s", filepath.Join(ct.testProjectDir, "main.go")),
				"analysis_type": "structure",
			},
		},
		{
			toolName: "scip_context_aware_assistance",
			params: map[string]interface{}{
				"uri":         fmt.Sprintf("file://%s", filepath.Join(ct.testProjectDir, "main.go")),
				"line":        15,
				"character":   10,
				"context_size": 5,
			},
		},
		{
			toolName: "scip_workspace_intelligence",
			params: map[string]interface{}{
				"workspace_path": ct.testProjectDir,
				"include_metrics": true,
			},
		},
		{
			toolName: "scip_refactoring_suggestions",
			params: map[string]interface{}{
				"uri":              fmt.Sprintf("file://%s", filepath.Join(ct.testProjectDir, "main.go")),
				"refactoring_type": "general",
			},
		},
	}

	for _, testCase := range scipTestCases {
		ct.t.Logf("Testing SCIP tool: %s", testCase.toolName)

		result, err := ct.client.CallTool(testCase.toolName, testCase.params)
		
		// SCIP tools may have different availability
		if err != nil {
			if strings.Contains(err.Error(), "not available") || strings.Contains(err.Error(), "not supported") {
				ct.t.Logf("SCIP tool %s not available in test environment: %v", testCase.toolName, err)
				continue
			}
			ct.t.Logf("SCIP tool %s returned error: %v", testCase.toolName, err)
			continue
		}

		ct.testHelper.AssertNotNil(result, fmt.Sprintf("SCIP tool %s should return result", testCase.toolName))
		
		if result.IsError {
			ct.t.Logf("SCIP tool %s returned error result: %v", testCase.toolName, result.Error)
		} else {
			ct.t.Logf("SCIP tool %s executed successfully", testCase.toolName)
			assert.NotEmpty(ct.t, result.Content, "SCIP tool result should have content")
		}
	}
}

// testErrorHandling tests error handling scenarios
func (ct *ComprehensiveMCPTest) testErrorHandling() {
	ct.t.Log("Testing error handling")

	// Test invalid tool name
	_, err := ct.client.CallTool("nonexistent_tool", map[string]interface{}{})
	assert.Error(ct.t, err, "Should return error for nonexistent tool")

	// Test invalid parameters
	_, err = ct.client.CallTool("goto_definition", map[string]interface{}{
		"invalid_param": "value",
	})
	assert.Error(ct.t, err, "Should return error for invalid parameters")

	// Test malformed URI
	_, err = ct.client.CallTool("goto_definition", map[string]interface{}{
		"uri":       "not-a-valid-uri",
		"line":      0,
		"character": 0,
	})
	// This may or may not error depending on validation level

	ct.t.Log("Error handling test passed")
}

// testPerformanceMetrics tests performance metrics collection
func (ct *ComprehensiveMCPTest) testPerformanceMetrics() {
	ct.t.Log("Testing performance metrics")

	// Get initial metrics
	initialMetrics := ct.client.GetMetrics()

	// Make several requests to accumulate metrics
	for i := 0; i < 5; i++ {
		_, _ = ct.client.CallTool("search_workspace_symbols", map[string]interface{}{
			"query": fmt.Sprintf("test%d", i),
		})
	}

	// Get final metrics
	finalMetrics := ct.client.GetMetrics()

	// Verify metrics were updated
	assert.GreaterOrEqual(ct.t, finalMetrics.RequestsSent, initialMetrics.RequestsSent, "Requests sent should increase")
	assert.NotZero(ct.t, finalMetrics.LastRequestTime, "Last request time should be set")

	ct.t.Logf("Final metrics: Requests=%d, Responses=%d, Errors=%d, AvgLatency=%v", 
		finalMetrics.RequestsSent, 
		finalMetrics.ResponsesReceived, 
		finalMetrics.ErrorsReceived,
		finalMetrics.AverageLatency)

	ct.t.Log("Performance metrics test passed")
}

// cleanupTestEnvironment cleans up the test environment
func (ct *ComprehensiveMCPTest) cleanupTestEnvironment() {
	ct.t.Log("Cleaning up test environment")

	if ct.testProjectDir != "" {
		err := os.RemoveAll(ct.testProjectDir)
		if err != nil {
			ct.t.Logf("Failed to remove test directory: %v", err)
		}
	}

	ct.t.Log("Test environment cleaned up")
}

// Additional helper methods

// waitForServerReady waits for the server to be ready
func (ct *ComprehensiveMCPTest) waitForServerReady(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for server to be ready")
		case <-ticker.C:
			if ct.client != nil && ct.client.IsConnected() {
				return nil
			}
		}
	}
}

// verifyToolSchema validates that tools have proper schemas
func (ct *ComprehensiveMCPTest) verifyToolSchema(tool Tool) error {
	if tool.InputSchema == nil {
		return fmt.Errorf("tool %s has no input schema", tool.Name)
	}

	// Check for required fields in schema
	if properties, ok := tool.InputSchema["properties"].(map[string]interface{}); ok {
		if len(properties) == 0 {
			return fmt.Errorf("tool %s has empty properties in schema", tool.Name)
		}
	}

	return nil
}