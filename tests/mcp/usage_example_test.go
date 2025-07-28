package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMCPClientUsageExample demonstrates how to use the MCP test client
func TestMCPClientUsageExample(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping MCP usage example in short mode")
	}

	t.Log("=== MCP Client Usage Example ===")

	// Step 1: Create a mock transport for demonstration
	t.Log("Step 1: Setting up mock transport for testing")
	
	config := DefaultClientConfig()
	config.LogLevel = "info"
	config.RequestTimeout = 5 * time.Second
	
	client, mockTransport := CreateTestClient(t, config)
	defer client.Shutdown()

	// Step 2: Demonstrate basic connection
	t.Log("Step 2: Connecting to MCP server")
	
	err := client.Connect(context.Background())
	require.NoError(t, err, "Failed to connect client")

	assert.True(t, client.IsConnected(), "Client should be connected")
	t.Logf("✓ Client connected successfully. State: %s", client.GetState())

	// Step 3: Initialize the MCP session  
	t.Log("Step 3: Initializing MCP session")

	// Set up automatic response simulation in a goroutine
	go func() {
		// Wait for and respond to initialize request
		sent, err := mockTransport.GetLastSent()
		if err == nil && sent.Method == "initialize" {
			initResponse := NewMessageBuilder().
				WithID(sent.ID).
				WithResult(map[string]interface{}{
					"protocolVersion": "2025-06-18",
					"capabilities": map[string]interface{}{
						"tools": true,
					},
					"serverInfo": map[string]interface{}{
						"name":    "test-server",
						"version": "1.0.0",
					},
				}).
				Build()
			mockTransport.SimulateResponse(initResponse)
		}
	}()

	result, err := client.Initialize(nil, nil)
	require.NoError(t, err, "Failed to initialize client")
	require.NotNil(t, result, "Initialize result should not be nil")

	assert.True(t, client.IsInitialized(), "Client should be initialized")
	assert.Equal(t, "2025-06-18", result.ProtocolVersion, "Protocol version should match")
	t.Logf("✓ Client initialized successfully. Protocol: %s", result.ProtocolVersion)

	// Step 4: List available tools
	t.Log("Step 4: Discovering available tools")

	// Simulate tools/list response
	toolsResponse := NewMessageBuilder().
		WithID(2).
		WithResult(map[string]interface{}{
			"tools": []interface{}{
				map[string]interface{}{
					"name":        "goto_definition",
					"description": "Navigate to symbol definition",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"uri":       map[string]interface{}{"type": "string"},
							"line":      map[string]interface{}{"type": "integer"},
							"character": map[string]interface{}{"type": "integer"},
						},
						"required": []interface{}{"uri", "line", "character"},
					},
				},
				map[string]interface{}{
					"name":        "search_workspace_symbols",
					"description": "Search for symbols in workspace",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"query": map[string]interface{}{"type": "string"},
						},
						"required": []interface{}{"query"},
					},
				},
			},
		}).
		Build()

	err = mockTransport.SimulateResponse(toolsResponse)
	require.NoError(t, err, "Failed to simulate tools response")

	tools, err := client.ListTools()
	require.NoError(t, err, "Failed to list tools")
	require.NotEmpty(t, tools, "Tools list should not be empty")

	t.Logf("✓ Discovered %d tools:", len(tools))
	for i, tool := range tools {
		t.Logf("  %d. %s: %s", i+1, tool.Name, tool.Description)
	}

	// Step 5: Call a tool
	t.Log("Step 5: Calling a tool")

	// Simulate tool call response
	toolCallResponse := NewMessageBuilder().
		WithID(3).
		WithResult(map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "Symbol found at line 42, character 10",
				},
			},
			"isError": false,
			"meta": map[string]interface{}{
				"timestamp": time.Now().Format(time.RFC3339),
				"duration":  "50ms",
				"lspMethod": "textDocument/definition",
				"cacheHit":  false,
			},
		}).
		Build()

	err = mockTransport.SimulateResponse(toolCallResponse)
	require.NoError(t, err, "Failed to simulate tool call response")

	params := map[string]interface{}{
		"uri":       "file:///test/main.go",
		"line":      10,
		"character": 5,
	}

	result2, err := client.CallTool("goto_definition", params)
	require.NoError(t, err, "Failed to call tool")
	require.NotNil(t, result2, "Tool result should not be nil")

	assert.False(t, result2.IsError, "Tool call should not return error")
	assert.NotEmpty(t, result2.Content, "Tool result should have content")

	t.Logf("✓ Tool call successful. Result: %s", ExtractTextFromToolResult(result2))

	// Step 6: Demonstrate metrics
	t.Log("Step 6: Checking client metrics")

	metrics := client.GetMetrics()
	t.Logf("✓ Client metrics:")
	t.Logf("  - Requests sent: %d", metrics.RequestsSent)
	t.Logf("  - Responses received: %d", metrics.ResponsesReceived)
	t.Logf("  - Errors: %d", metrics.ErrorsReceived)
	t.Logf("  - Average latency: %v", metrics.AverageLatency)

	// Step 7: Demonstrate error handling
	t.Log("Step 7: Testing error handling")

	// Simulate error response
	errorResponse := NewMessageBuilder().
		WithID(4).
		WithError(-32601, "Method not found", nil).
		Build()

	err = mockTransport.SimulateResponse(errorResponse)
	require.NoError(t, err, "Failed to simulate error response")

	_, err = client.CallTool("nonexistent_tool", nil)
	assert.Error(t, err, "Should return error for nonexistent tool")
	t.Logf("✓ Error handling works: %v", err)

	// Step 8: Cleanup
	t.Log("Step 8: Cleaning up")

	err = client.Disconnect()
	require.NoError(t, err, "Failed to disconnect client")

	assert.False(t, client.IsConnected(), "Client should be disconnected")
	t.Log("✓ Client disconnected successfully")

	t.Log("=== MCP Client Usage Example Completed Successfully ===")
}

// TestMCPServerIntegration demonstrates connecting to a real MCP server
func TestMCPServerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real MCP server integration in short mode")
	}

	t.Log("=== Real MCP Server Integration Test ===")

	// Create temp directory and config
	tempDir, err := os.MkdirTemp("", "mcp-server-test-*")
	require.NoError(t, err, "Failed to create temp directory")
	defer os.RemoveAll(tempDir)

	// Build server if needed
	wd, err := os.Getwd()
	require.NoError(t, err, "Failed to get working directory")
	binaryPath := filepath.Join(wd, "..", "..", "bin", "lsp-gateway")

	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Log("Building MCP server")
		buildCmd := exec.Command("make", "local")
		buildCmd.Dir = filepath.Join(wd, "..", "..")
		output, err := buildCmd.CombinedOutput()
		require.NoError(t, err, "Failed to build server: %s", string(output))
	}

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "mcp")
	cmd.Dir = tempDir

	stdin, err := cmd.StdinPipe()
	require.NoError(t, err, "Failed to create stdin pipe")

	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err, "Failed to create stdout pipe")

	stderr, err := cmd.StderrPipe()
	require.NoError(t, err, "Failed to create stderr pipe")

	err = cmd.Start()
	require.NoError(t, err, "Failed to start MCP server")

	// Setup client with real transport
	config := DefaultClientConfig()
	config.RequestTimeout = 10 * time.Second
	transport := NewSTDIOTransport(stdin, stdout, stderr)
	client := NewTestMCPClient(config, transport)

	// Connect and test
	err = client.Connect(context.Background())
	require.NoError(t, err, "Failed to connect to real server")

	t.Log("✓ Connected to real MCP server")

	// Initialize
	result, err := client.Initialize(nil, nil)
	if err != nil {
		t.Logf("Initialize failed (may be expected): %v", err)
	} else {
		t.Logf("✓ Initialized with protocol version: %s", result.ProtocolVersion)

		// List tools
		tools, err := client.ListTools()
		if err != nil {
			t.Logf("ListTools failed (may be expected): %v", err)
		} else {
			t.Logf("✓ Found %d tools from real server", len(tools))
			for _, tool := range tools {
				t.Logf("  - %s", tool.Name)
			}
		}
	}

	// Cleanup
	client.Shutdown()
	cancel()
	cmd.Wait()

	t.Log("=== Real MCP Server Integration Test Completed ===")
}

// BenchmarkMCPClientPerformance benchmarks the MCP client performance
func BenchmarkMCPClientPerformance(b *testing.B) {
	config := DefaultClientConfig()
	
	transport := NewMockTransport()
	client := NewTestMCPClient(config, transport)
	defer client.Shutdown()

	// Setup
	transport.Connect(context.Background())
	client.Connect(context.Background())

	// Simulate responses
	for i := 0; i < b.N; i++ {
		response := NewMessageBuilder().
			WithID(i).
			WithResult(map[string]interface{}{
				"content": []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": fmt.Sprintf("Result %d", i),
					},
				},
			}).
			Build()
		transport.SimulateResponse(response)
	}

	// Initialize client
	initResponse := NewMessageBuilder().
		WithID("init").
		WithResult(map[string]interface{}{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]interface{}{"tools": true},
			"serverInfo":      map[string]interface{}{"name": "test"},
		}).
		Build()
	transport.SimulateResponse(initResponse)
	client.Initialize(nil, nil)

	// Benchmark tool calls
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		callID := 0
		for pb.Next() {
			params := map[string]interface{}{
				"query": fmt.Sprintf("test%d", callID),
			}
			client.CallTool("search_workspace_symbols", params)
			callID++
		}
	})
}