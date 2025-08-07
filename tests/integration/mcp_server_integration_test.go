package integration

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server"

	"github.com/stretchr/testify/require"
)

func TestMCPServerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	_, err := exec.LookPath("gopls")
	if err != nil {
		t.Skip("Go LSP server (gopls) not installed, skipping test")
	}

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")
	err = os.WriteFile(testFile, []byte(`package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}

func testFunction() string {
	return "test"
}
`), 0644)
	require.NoError(t, err)

	cacheDir := filepath.Join(tempDir, "mcp-cache")
	cfg := &config.Config{
		Cache: &config.CacheConfig{
			Enabled:         true,
			StoragePath:     cacheDir,
			MaxMemoryMB:     64,
			TTLHours:        1,
			Languages:       []string{"go"},
			BackgroundIndex: false,
			DiskCache:       true,
			EvictionPolicy:  "lru",
		},
		Servers: map[string]*config.ServerConfig{
			"go": {
				Command: "gopls",
				Args:    []string{"serve"},
			},
		},
	}

	mcpServer, err := server.NewMCPServer(cfg)
	require.NoError(t, err)
	require.NotNil(t, mcpServer)

	err = mcpServer.Start()
	require.NoError(t, err)
	defer mcpServer.Stop()

	time.Sleep(2 * time.Second)

	t.Run("Initialize MCP connection", func(t *testing.T) {
		initRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "initialize",
			"params": map[string]interface{}{
				"clientInfo": map[string]string{
					"name":    "test-client",
					"version": "1.0.0",
				},
				"protocolVersion": "2025-06-18",
			},
			"id": 1,
		}

		response := sendMCPRequest(t, mcpServer, initRequest)
		require.NotNil(t, response)

		result, ok := response["result"].(map[string]interface{})
		require.True(t, ok, "Response should have result field")

		serverInfo, ok := result["serverInfo"].(map[string]interface{})
		require.True(t, ok, "Result should have serverInfo")
		require.Equal(t, "lsp-gateway-mcp", serverInfo["name"])

		tools, ok := result["tools"].([]interface{})
		require.True(t, ok, "Result should have tools")
		require.GreaterOrEqual(t, len(tools), 4, "Should have at least 4 tools")

		initializedNotification := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "notifications/initialized",
		}
		sendMCPRequest(t, mcpServer, initializedNotification)
	})

	t.Run("Find symbols in file", func(t *testing.T) {
		findSymbolsRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name": "findSymbols",
				"arguments": map[string]interface{}{
					"pattern":     "test",
					"filePattern": tempDir,
				},
			},
			"id": 2,
		}

		response := sendMCPRequest(t, mcpServer, findSymbolsRequest)
		require.NotNil(t, response)

		result, ok := response["result"].(map[string]interface{})
		require.True(t, ok, "Response should have result field")

		content, ok := result["content"].([]interface{})
		require.True(t, ok, "Result should have content array")
		require.Greater(t, len(content), 0, "Should find at least one symbol")

		firstResult := content[0].(map[string]interface{})
		text, ok := firstResult["text"].(string)
		require.True(t, ok, "Content should have text field")
		require.Contains(t, text, "testFunction", "Should find testFunction symbol")
	})

	t.Run("Get symbol info", func(t *testing.T) {
		getSymbolInfoRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name": "getSymbolInfo",
				"arguments": map[string]interface{}{
					"symbolName":  "main",
					"filePattern": tempDir,
				},
			},
			"id": 3,
		}

		response := sendMCPRequest(t, mcpServer, getSymbolInfoRequest)
		require.NotNil(t, response)

		result, ok := response["result"].(map[string]interface{})
		require.True(t, ok, "Response should have result field")

		content, ok := result["content"].([]interface{})
		require.True(t, ok, "Result should have content array")
		require.Greater(t, len(content), 0, "Should find symbol info")

		firstResult := content[0].(map[string]interface{})
		text, ok := firstResult["text"].(string)
		require.True(t, ok, "Content should have text field")
		require.Contains(t, text, "main", "Should contain main function info")
	})

	t.Run("Find references", func(t *testing.T) {
		findReferencesRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name": "findReferences",
				"arguments": map[string]interface{}{
					"symbolName":  "fmt",
					"filePattern": tempDir,
				},
			},
			"id": 4,
		}

		response := sendMCPRequest(t, mcpServer, findReferencesRequest)
		require.NotNil(t, response)

		result, ok := response["result"].(map[string]interface{})
		require.True(t, ok, "Response should have result field")

		_, ok = result["content"].([]interface{})
		require.True(t, ok, "Result should have content array")
	})

	t.Run("Find definitions", func(t *testing.T) {
		findDefinitionsRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name": "findDefinitions",
				"arguments": map[string]interface{}{
					"symbolName":  "testFunction",
					"filePattern": tempDir,
				},
			},
			"id": 5,
		}

		response := sendMCPRequest(t, mcpServer, findDefinitionsRequest)
		require.NotNil(t, response)

		result, ok := response["result"].(map[string]interface{})
		require.True(t, ok, "Response should have result field")

		content, ok := result["content"].([]interface{})
		require.True(t, ok, "Result should have content array")
		require.Greater(t, len(content), 0, "Should find definition")
	})
}

func sendMCPRequest(t *testing.T, server *server.MCPServer, request map[string]interface{}) map[string]interface{} {
	requestBytes, err := json.Marshal(request)
	require.NoError(t, err)

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(requestBytes)))
	buf.Write(requestBytes)

	reader := bufio.NewReader(&buf)

	contentLengthLine, err := reader.ReadString('\n')
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(contentLengthLine, "Content-Length:"))

	var contentLength int
	fmt.Sscanf(contentLengthLine, "Content-Length: %d", &contentLength)

	reader.ReadString('\n')

	content := make([]byte, contentLength)
	_, err = reader.Read(content)
	require.NoError(t, err)

	var response map[string]interface{}
	err = json.Unmarshal(content, &response)
	if err == nil && response["method"] == nil {
		return response
	}

	return nil
}
