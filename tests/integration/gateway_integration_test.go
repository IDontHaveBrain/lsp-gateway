package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server"
	"lsp-gateway/src/utils"

	"github.com/stretchr/testify/require"
)

func TestGatewayIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	_, err := exec.LookPath("gopls")
	if err != nil {
		t.Skip("Go LSP server (gopls) not installed, skipping test")
	}

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "main.go")
	testContent := []byte(`package main

import (
	"fmt"
	"net/http"
)

type Server struct {
	port int
	handler http.Handler
}

func NewServer(port int) *Server {
	return &Server{
		port: port,
	}
}

func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	return http.ListenAndServe(addr, s.handler)
}

func main() {
	server := NewServer(8080)
	server.Start()
}
`)
	err = os.WriteFile(testFile, testContent, 0644)
	require.NoError(t, err)

	cacheDir := filepath.Join(tempDir, "gateway-cache")
	cfg := &config.Config{
		Cache: &config.CacheConfig{
			Enabled:         true,
			StoragePath:     cacheDir,
			MaxMemoryMB:     128,
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

	gateway, err := server.NewHTTPGateway(":18888", cfg, false)
	require.NoError(t, err)
	require.NotNil(t, gateway)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = gateway.Start(ctx)
	require.NoError(t, err)
	time.Sleep(2 * time.Second) // Wait for server to start
	serverAddr := "localhost:18888"
	defer gateway.Stop()

	baseURL := fmt.Sprintf("http://%s", serverAddr)
	jsonrpcURL := fmt.Sprintf("%s/jsonrpc", baseURL)

	time.Sleep(3 * time.Second)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	t.Run("Health check", func(t *testing.T) {
		resp, err := client.Get(fmt.Sprintf("%s/health", baseURL))
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var health map[string]interface{}
		err = json.Unmarshal(body, &health)
		require.NoError(t, err)
		require.Equal(t, "healthy", health["status"])
	})

	t.Run("TextDocument Definition via JSON-RPC", func(t *testing.T) {
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "textDocument/definition",
			"params": map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": utils.FilePathToURI(testFile),
				},
				"position": map[string]interface{}{
					"line":      24,
					"character": 10,
				},
			},
			"id": 1,
		}

		response := sendJSONRPCRequest(t, client, jsonrpcURL, request)
		require.NotNil(t, response)
		require.Contains(t, response, "result")
		require.NotContains(t, response, "error")
	})

	t.Run("TextDocument Hover via JSON-RPC", func(t *testing.T) {
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "textDocument/hover",
			"params": map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": utils.FilePathToURI(testFile),
				},
				"position": map[string]interface{}{
					"line":      13,
					"character": 15,
				},
			},
			"id": 2,
		}

		response := sendJSONRPCRequest(t, client, jsonrpcURL, request)
		require.NotNil(t, response)
		require.Contains(t, response, "result")

		result, ok := response["result"].(map[string]interface{})
		if ok && result != nil {
			contents, ok := result["contents"].(map[string]interface{})
			require.True(t, ok, "Hover should have contents")
			require.NotNil(t, contents)
		}
	})

	t.Run("TextDocument DocumentSymbol via JSON-RPC", func(t *testing.T) {
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "textDocument/documentSymbol",
			"params": map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": utils.FilePathToURI(testFile),
				},
			},
			"id": 3,
		}

		response := sendJSONRPCRequest(t, client, jsonrpcURL, request)
		require.NotNil(t, response)
		require.Contains(t, response, "result")

		result, ok := response["result"].([]interface{})
		require.True(t, ok, "Result should be array of symbols")
		require.Greater(t, len(result), 0, "Should find document symbols")
	})

	t.Run("Workspace Symbol via JSON-RPC", func(t *testing.T) {
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "workspace/symbol",
			"params": map[string]interface{}{
				"query": "Server",
			},
			"id": 4,
		}

		response := sendJSONRPCRequest(t, client, jsonrpcURL, request)
		require.NotNil(t, response)
		require.Contains(t, response, "result")

		_, ok := response["result"].([]interface{})
		require.True(t, ok, "Result should be array of symbols")
	})

	t.Run("TextDocument References via JSON-RPC", func(t *testing.T) {
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "textDocument/references",
			"params": map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": utils.FilePathToURI(testFile),
				},
				"position": map[string]interface{}{
					"line":      8,
					"character": 5,
				},
				"context": map[string]interface{}{
					"includeDeclaration": true,
				},
			},
			"id": 5,
		}

		response := sendJSONRPCRequest(t, client, jsonrpcURL, request)
		require.NotNil(t, response)
		require.Contains(t, response, "result")

		result, ok := response["result"].([]interface{})
		require.True(t, ok, "Result should be array of locations")
		require.GreaterOrEqual(t, len(result), 1, "Should find at least one reference")
	})

	t.Run("TextDocument Completion via JSON-RPC", func(t *testing.T) {
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "textDocument/completion",
			"params": map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": utils.FilePathToURI(testFile),
				},
				"position": map[string]interface{}{
					"line":      24,
					"character": 8,
				},
			},
			"id": 6,
		}

		response := sendJSONRPCRequest(t, client, jsonrpcURL, request)
		require.NotNil(t, response)
		require.Contains(t, response, "result")

		result := response["result"]
		if result != nil {
			switch v := result.(type) {
			case []interface{}:
				require.NotNil(t, v, "Completion items should not be nil")
			case map[string]interface{}:
				if items, ok := v["items"].([]interface{}); ok {
					require.NotNil(t, items)
				}
			}
		}
	})

	t.Run("Batch JSON-RPC requests not supported", func(t *testing.T) {
		// Server only handles single requests, not batch arrays
		batchRequest := []map[string]interface{}{
			{
				"jsonrpc": "2.0",
				"method":  "textDocument/hover",
				"params": map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": utils.FilePathToURI(testFile),
					},
					"position": map[string]interface{}{
						"line":      13,
						"character": 15,
					},
				},
				"id": 100,
			},
		}

		requestBody, err := json.Marshal(batchRequest)
		require.NoError(t, err)

		resp, err := client.Post(jsonrpcURL, "application/json", bytes.NewReader(requestBody))
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		// Server should return a JSON-RPC error for unsupported batch format
		var errorResponse map[string]interface{}
		err = json.Unmarshal(body, &errorResponse)
		require.NoError(t, err)
		require.Contains(t, errorResponse, "error", "Should return error for batch request")
	})

	t.Run("Invalid method handling", func(t *testing.T) {
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "invalid/method",
			"params":  map[string]interface{}{},
			"id":      999,
		}

		response := sendJSONRPCRequest(t, client, jsonrpcURL, request)
		require.NotNil(t, response)
		require.Contains(t, response, "error")

		errorObj, ok := response["error"].(map[string]interface{})
		require.True(t, ok, "Error should be an object")
		require.Contains(t, errorObj, "code")
		require.Contains(t, errorObj, "message")
	})

	t.Run("Concurrent requests", func(t *testing.T) {
		done := make(chan bool, 5)

		for i := 0; i < 5; i++ {
			go func(id int) {
				defer func() { done <- true }()

				request := map[string]interface{}{
					"jsonrpc": "2.0",
					"method":  "textDocument/hover",
					"params": map[string]interface{}{
						"textDocument": map[string]interface{}{
							"uri": utils.FilePathToURI(testFile),
						},
						"position": map[string]interface{}{
							"line":      13,
							"character": 15,
						},
					},
					"id": 1000 + id,
				}

				response := sendJSONRPCRequest(t, client, jsonrpcURL, request)
				require.NotNil(t, response)
				require.Contains(t, response, "result")
			}(i)
		}

		for i := 0; i < 5; i++ {
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("Concurrent request timeout")
			}
		}
	})
}

func sendJSONRPCRequest(t *testing.T, client *http.Client, url string, request map[string]interface{}) map[string]interface{} {
	requestBody, err := json.Marshal(request)
	require.NoError(t, err)

	resp, err := client.Post(url, "application/json", bytes.NewReader(requestBody))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	return response
}
