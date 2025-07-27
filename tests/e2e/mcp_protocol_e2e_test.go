package e2e_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lsp-gateway/tests/e2e/testutils"
)


// TestMCPStdioProtocol tests MCP server STDIO communication
func TestMCPStdioProtocol(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping MCP E2E test in short mode")
	}

	// Build binary
	projectRoot, err := testutils.GetProjectRoot()
	require.NoError(t, err)
	binaryPath := filepath.Join(projectRoot, "bin", "lsp-gateway")
	
	// Find available port for gateway
	gatewayPort, err := testutils.FindAvailablePort()
	require.NoError(t, err)
	gatewayURL := fmt.Sprintf("http://localhost:%d", gatewayPort)
	
	// Create temporary config file
	configContent := fmt.Sprintf(`
servers:
- name: go-lsp
  languages:
  - go
  command: gopls
  args: []
  transport: stdio
  root_markers:
  - go.mod
  - go.sum
  priority: 1
  weight: 1.0
port: %d
timeout: 45s
max_concurrent_requests: 150
multi_server_config:
  primary: null
  secondary: []
  selection_strategy: load_balance
  concurrent_limit: 3
  resource_sharing: true
  health_check_interval: 30s
  max_retries: 3
enable_concurrent_servers: true
max_concurrent_servers_per_language: 2
enable_smart_routing: true
smart_router_config:
  default_strategy: single_target_with_fallback
  method_strategies:
    textDocument/definition: single_target_with_fallback
`, gatewayPort)
	configPath, configCleanup, err := testutils.CreateTempConfig(configContent)
	require.NoError(t, err)
	defer configCleanup()
	
	// Start LSP Gateway server first
	gatewayCmd := exec.Command(binaryPath, "server", "--config", configPath)
	gatewayCmd.Dir = projectRoot
	err = gatewayCmd.Start()
	require.NoError(t, err)
	
	defer func() {
		if gatewayCmd.Process != nil {
			gatewayCmd.Process.Signal(syscall.SIGTERM)
			gatewayCmd.Wait()
		}
	}()
	
	// Wait for gateway to start
	time.Sleep(3 * time.Second)
	
	// Start MCP server via STDIO (default transport)
	cmd := exec.Command(binaryPath, "mcp", "--gateway", gatewayURL)
	cmd.Dir = projectRoot
	
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	
	// Capture stderr for debugging
	stderr, err := cmd.StderrPipe()
	require.NoError(t, err)
	
	err = cmd.Start()
	require.NoError(t, err)
	
	// Log any stderr output
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			t.Logf("MCP stderr: %s", scanner.Text())
		}
	}()
	
	defer func() {
		stdin.Close()
		if cmd.Process != nil {
			cmd.Process.Signal(syscall.SIGTERM)
			cmd.Wait()
		}
	}()

	reader := bufio.NewReader(stdout)

	// Test MCP initialize
	initMsg := testutils.MCPMessage{
		Jsonrpc: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
		},
	}

	response, err := sendMCPStdioMessage(stdin, reader, initMsg)
	require.NoError(t, err)
	assert.Equal(t, "2.0", response.Jsonrpc)
	assert.Equal(t, 1, int(response.ID.(float64)))
	assert.NotNil(t, response.Result)

	t.Logf("MCP initialize successful")
}

// TestMCPTCPProtocol tests MCP server TCP communication
func TestMCPTCPProtocol(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping MCP E2E test in short mode")
	}

	// Build binary
	projectRoot, err := testutils.GetProjectRoot()
	require.NoError(t, err)
	binaryPath := filepath.Join(projectRoot, "bin", "lsp-gateway")
	
	// Find available ports
	gatewayPort, err := testutils.FindAvailablePort()
	require.NoError(t, err)
	mcpPort, err := testutils.FindAvailablePort()
	require.NoError(t, err)
	gatewayURL := fmt.Sprintf("http://localhost:%d", gatewayPort)
	
	// Create temporary config file
	configContent := fmt.Sprintf(`
servers:
- name: go-lsp
  languages:
  - go
  command: gopls
  args: []
  transport: stdio
  root_markers:
  - go.mod
  - go.sum
  priority: 1
  weight: 1.0
port: %d
timeout: 45s
max_concurrent_requests: 150
multi_server_config:
  primary: null
  secondary: []
  selection_strategy: load_balance
  concurrent_limit: 3
  resource_sharing: true
  health_check_interval: 30s
  max_retries: 3
enable_concurrent_servers: true
max_concurrent_servers_per_language: 2
enable_smart_routing: true
smart_router_config:
  default_strategy: single_target_with_fallback
  method_strategies:
    textDocument/definition: single_target_with_fallback
`, gatewayPort)
	configPath, configCleanup, err := testutils.CreateTempConfig(configContent)
	require.NoError(t, err)
	defer configCleanup()
	
	// Start LSP Gateway server first
	gatewayCmd := exec.Command(binaryPath, "server", "--config", configPath)
	gatewayCmd.Dir = projectRoot
	err = gatewayCmd.Start()
	require.NoError(t, err)
	
	defer func() {
		if gatewayCmd.Process != nil {
			gatewayCmd.Process.Signal(syscall.SIGTERM)
			gatewayCmd.Wait()
		}
	}()
	
	// Wait for gateway to start
	time.Sleep(3 * time.Second)
	
	// Start MCP server via TCP
	cmd := exec.Command(binaryPath, "mcp", "--transport", "tcp", "--port", strconv.Itoa(mcpPort), "--gateway", gatewayURL)
	cmd.Dir = projectRoot
	
	// Capture stderr for debugging
	stderr, err := cmd.StderrPipe()
	require.NoError(t, err)
	
	err = cmd.Start()
	require.NoError(t, err)
	
	// Log any stderr output
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			t.Logf("MCP TCP stderr: %s", scanner.Text())
		}
	}()
	
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Signal(syscall.SIGTERM)
			cmd.Wait()
		}
	}()

	// Wait for server to start (increase timeout for TCP)
	time.Sleep(5 * time.Second)

	// Connect to MCP TCP server
	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", mcpPort))
	require.NoError(t, err)
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Test MCP initialize
	initMsg := testutils.MCPMessage{
		Jsonrpc: "2.0",
		ID:      1,
		Method:  "initialize", 
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
		},
	}

	response, err := sendMCPTCPMessage(conn, reader, initMsg)
	require.NoError(t, err)
	assert.Equal(t, "2.0", response.Jsonrpc)
	assert.Equal(t, 1, int(response.ID.(float64)))
	assert.NotNil(t, response.Result)

	t.Logf("MCP TCP initialize successful")
}

// TestMCPToolsList tests MCP tools/list functionality
func TestMCPToolsList(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping MCP E2E test in short mode")
	}

	// Build binary
	projectRoot, err := testutils.GetProjectRoot()
	require.NoError(t, err)
	binaryPath := filepath.Join(projectRoot, "bin", "lsp-gateway")
	
	// Find available port for gateway
	gatewayPort, err := testutils.FindAvailablePort()
	require.NoError(t, err)
	gatewayURL := fmt.Sprintf("http://localhost:%d", gatewayPort)
	
	// Create temporary config file
	configContent := fmt.Sprintf(`
servers:
- name: go-lsp
  languages:
  - go
  command: gopls
  args: []
  transport: stdio
  root_markers:
  - go.mod
  - go.sum
  priority: 1
  weight: 1.0
port: %d
timeout: 45s
max_concurrent_requests: 150
multi_server_config:
  primary: null
  secondary: []
  selection_strategy: load_balance
  concurrent_limit: 3
  resource_sharing: true
  health_check_interval: 30s
  max_retries: 3
enable_concurrent_servers: true
max_concurrent_servers_per_language: 2
enable_smart_routing: true
smart_router_config:
  default_strategy: single_target_with_fallback
  method_strategies:
    textDocument/definition: single_target_with_fallback
`, gatewayPort)
	configPath, configCleanup, err := testutils.CreateTempConfig(configContent)
	require.NoError(t, err)
	defer configCleanup()
	
	// Start LSP Gateway server first
	gatewayCmd := exec.Command(binaryPath, "server", "--config", configPath)
	gatewayCmd.Dir = projectRoot
	err = gatewayCmd.Start()
	require.NoError(t, err)
	
	defer func() {
		if gatewayCmd.Process != nil {
			gatewayCmd.Process.Signal(syscall.SIGTERM)
			gatewayCmd.Wait()
		}
	}()
	
	// Wait for gateway to start
	time.Sleep(3 * time.Second)
	
	// Start MCP server
	cmd := exec.Command(binaryPath, "mcp", "--gateway", gatewayURL)
	cmd.Dir = projectRoot
	
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	
	err = cmd.Start()
	require.NoError(t, err)
	
	defer func() {
		stdin.Close()
		if cmd.Process != nil {
			cmd.Process.Signal(syscall.SIGTERM)
			cmd.Wait()
		}
	}()

	reader := bufio.NewReader(stdout)

	// Initialize first
	initMsg := testutils.MCPMessage{
		Jsonrpc: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
		},
	}

	_, err = sendMCPStdioMessage(stdin, reader, initMsg)
	require.NoError(t, err)

	// Test tools/list
	listMsg := testutils.MCPMessage{
		Jsonrpc: "2.0",
		ID:      2,
		Method:  "tools/list",
	}

	response, err := sendMCPStdioMessage(stdin, reader, listMsg)
	require.NoError(t, err)
	assert.Equal(t, "2.0", response.Jsonrpc)
	assert.Equal(t, 2, int(response.ID.(float64)))
	assert.NotNil(t, response.Result)

	// Verify tools are available
	result := response.Result.(map[string]interface{})
	tools, exists := result["tools"]
	require.True(t, exists)
	
	toolsList := tools.([]interface{})
	assert.Greater(t, len(toolsList), 0)

	// Check for expected tools
	expectedTools := []string{
		"goto_definition",
		"find_references", 
		"get_hover_info",
		"get_document_symbols",
		"search_workspace_symbols",
	}

	foundTools := make(map[string]bool)
	for _, tool := range toolsList {
		toolMap := tool.(map[string]interface{})
		name := toolMap["name"].(string)
		foundTools[name] = true
	}

	for _, expectedTool := range expectedTools {
		assert.True(t, foundTools[expectedTool], "Expected tool %s not found", expectedTool)
	}

	t.Logf("MCP tools/list returned %d tools", len(toolsList))
}

// TestMCPBasicToolCall tests calling a basic MCP tool
func TestMCPBasicToolCall(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping MCP E2E test in short mode")
	}

	// Create a simple test file for testing
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")
	testContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	// Build binary
	projectRoot, err := testutils.GetProjectRoot()
	require.NoError(t, err)
	binaryPath := filepath.Join(projectRoot, "bin", "lsp-gateway")
	
	// Find available port for gateway
	gatewayPort, err := testutils.FindAvailablePort()
	require.NoError(t, err)
	gatewayURL := fmt.Sprintf("http://localhost:%d", gatewayPort)
	
	// Start LSP Gateway server first
	gatewayCmd := exec.Command(binaryPath, "server", "--port", strconv.Itoa(gatewayPort))
	gatewayCmd.Dir = projectRoot
	err = gatewayCmd.Start()
	require.NoError(t, err)
	
	defer func() {
		if gatewayCmd.Process != nil {
			gatewayCmd.Process.Signal(syscall.SIGTERM)
			gatewayCmd.Wait()
		}
	}()
	
	// Wait for gateway to start
	time.Sleep(3 * time.Second)
	
	// Start MCP server
	cmd := exec.Command(binaryPath, "mcp", "--gateway", gatewayURL)
	cmd.Dir = tempDir // Run in test directory
	
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	
	err = cmd.Start()
	require.NoError(t, err)
	
	defer func() {
		stdin.Close()
		if cmd.Process != nil {
			cmd.Process.Signal(syscall.SIGTERM)
			cmd.Wait()
		}
	}()

	reader := bufio.NewReader(stdout)

	// Initialize
	initMsg := testutils.MCPMessage{
		Jsonrpc: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
		},
	}

	_, err = sendMCPStdioMessage(stdin, reader, initMsg)
	require.NoError(t, err)

	// Test get_document_symbols tool call
	callMsg := testutils.MCPMessage{
		Jsonrpc: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "get_document_symbols",
			"arguments": map[string]interface{}{
				"uri": fmt.Sprintf("file://%s", testFile),
			},
		},
	}

	response, err := sendMCPStdioMessage(stdin, reader, callMsg)
	require.NoError(t, err)
	assert.Equal(t, "2.0", response.Jsonrpc)
	assert.Equal(t, 3, int(response.ID.(float64)))

	// Should have result or graceful error
	if response.Error == nil {
		assert.NotNil(t, response.Result)
		t.Logf("MCP tool call successful")
	} else {
		// Graceful error is acceptable for E2E test
		t.Logf("MCP tool call returned error (expected without LSP): %v", response.Error)
	}
}

// sendMCPStdioMessage sends a message via STDIO and returns the response
func sendMCPStdioMessage(stdin io.WriteCloser, reader *bufio.Reader, msg testutils.MCPMessage) (*testutils.MCPMessage, error) {
	// Serialize message
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	// Send with Content-Length header
	message := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(msgBytes), msgBytes)
	_, err = stdin.Write([]byte(message))
	if err != nil {
		return nil, err
	}

	// Read response
	return readMCPStdioMessage(reader)
}

// sendMCPTCPMessage sends a message via TCP and returns the response
func sendMCPTCPMessage(conn net.Conn, reader *bufio.Reader, msg testutils.MCPMessage) (*testutils.MCPMessage, error) {
	// Serialize message
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	// Send with Content-Length header
	message := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(msgBytes), msgBytes)
	_, err = conn.Write([]byte(message))
	if err != nil {
		return nil, err
	}

	// Read response
	return readMCPTCPMessage(reader)
}

// readMCPStdioMessage reads a JSON-RPC message from STDIO
func readMCPStdioMessage(reader *bufio.Reader) (*testutils.MCPMessage, error) {
	// Read Content-Length header
	line, _, err := reader.ReadLine()
	if err != nil {
		return nil, err
	}

	contentLengthStr := strings.TrimPrefix(string(line), "Content-Length: ")
	contentLength, err := strconv.Atoi(strings.TrimSpace(contentLengthStr))
	if err != nil {
		return nil, fmt.Errorf("invalid Content-Length: %s", contentLengthStr)
	}

	// Read empty line
	_, _, err = reader.ReadLine()
	if err != nil {
		return nil, err
	}

	// Read message body
	msgBytes := make([]byte, contentLength)
	_, err = io.ReadFull(reader, msgBytes)
	if err != nil {
		return nil, err
	}

	// Parse JSON-RPC message
	var msg testutils.MCPMessage
	err = json.Unmarshal(msgBytes, &msg)
	if err != nil {
		return nil, err
	}

	return &msg, nil
}

// readMCPTCPMessage reads a JSON-RPC message from TCP
func readMCPTCPMessage(reader *bufio.Reader) (*testutils.MCPMessage, error) {
	return readMCPStdioMessage(reader) // Same format for both
}



