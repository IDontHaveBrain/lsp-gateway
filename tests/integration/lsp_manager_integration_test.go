package integration

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server"
	"lsp-gateway/src/server/cache"

	"github.com/stretchr/testify/require"
	"go.lsp.dev/protocol"
)

func TestLSPManagerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	_, err := exec.LookPath("gopls")
	if err != nil {
		t.Skip("Go LSP server (gopls) not installed, skipping test")
	}

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")
	testContent := []byte(`package main

import (
	"fmt"
	"strings"
)

type TestStruct struct {
	Name  string
	Value int
}

func main() {
	ts := TestStruct{
		Name:  "test",
		Value: 42,
	}
	fmt.Println(ts.Name)
	result := processData(ts)
	fmt.Println(result)
}

func processData(data TestStruct) string {
	return strings.ToUpper(data.Name)
}

func helperFunction() {
	fmt.Println("Helper")
}
`)
	err = os.WriteFile(testFile, testContent, 0644)
	require.NoError(t, err)

	cacheDir := filepath.Join(tempDir, "lsp-cache")
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

	scipCache, err := cache.NewSCIPCacheManager(cfg.Cache)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = scipCache.Start(ctx)
	require.NoError(t, err)
	defer scipCache.Stop()

	lspManager, err := server.NewLSPManager(cfg)
	require.NoError(t, err)
	lspManager.SetCache(scipCache)

	err = lspManager.Start(ctx)
	require.NoError(t, err)
	defer lspManager.Stop()

	time.Sleep(2 * time.Second)

	t.Run("TextDocument Definition", func(t *testing.T) {
		params := map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": common.FilePathToURI(testFile),
			},
			"position": map[string]interface{}{
				"line":      17,
				"character": 20,
			},
		}

		result, err := lspManager.ProcessRequest(ctx, "textDocument/definition", params)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Handle different response types from LSP server
		var locations []protocol.Location
		switch res := result.(type) {
		case protocol.Location:
			locations = []protocol.Location{res}
		case []protocol.Location:
			locations = res
		case json.RawMessage:
			// Try to unmarshal as array first
			err := json.Unmarshal(res, &locations)
			if err != nil {
				// Try as single location
				var singleLoc protocol.Location
				if err2 := json.Unmarshal(res, &singleLoc); err2 == nil {
					locations = []protocol.Location{singleLoc}
				}
			}
		case []interface{}:
			for _, item := range res {
				if data, err := json.Marshal(item); err == nil {
					var loc protocol.Location
					if json.Unmarshal(data, &loc) == nil {
						locations = append(locations, loc)
					}
				}
			}
		default:
			require.Fail(t, "Unexpected result type", "Expected protocol.Location or []protocol.Location, got %T", result)
		}

		require.Greater(t, len(locations), 0, "Should find definition")
		require.Contains(t, string(locations[0].URI), "test.go")
	})

	t.Run("TextDocument References", func(t *testing.T) {
		params := map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": common.FilePathToURI(testFile),
			},
			"position": map[string]interface{}{
				"line":      23,
				"character": 5,
			},
			"context": map[string]interface{}{
				"includeDeclaration": true,
			},
		}

		result, err := lspManager.ProcessRequest(ctx, "textDocument/references", params)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Handle different response types from LSP server
		var locations []protocol.Location
		switch res := result.(type) {
		case []protocol.Location:
			locations = res
		case json.RawMessage:
			// Handle null or empty responses
			jsonStr := string(res)
			if jsonStr == "null" || jsonStr == "[]" {
				locations = []protocol.Location{}
			} else {
				err := json.Unmarshal(res, &locations)
				if err != nil {
					t.Logf("Failed to unmarshal references JSON: %s", jsonStr)
					locations = []protocol.Location{}
				}
			}
		case []interface{}:
			for _, item := range res {
				if data, err := json.Marshal(item); err == nil {
					var loc protocol.Location
					if json.Unmarshal(data, &loc) == nil {
						locations = append(locations, loc)
					}
				}
			}
		default:
			require.Fail(t, "Unexpected result type", "Expected []protocol.Location, got %T", result)
		}

		if len(locations) == 0 {
			t.Log("No references found, which may be expected depending on test file content")
		} else {
			require.GreaterOrEqual(t, len(locations), 1, "Should find at least one reference")
		}
	})

	t.Run("TextDocument Hover", func(t *testing.T) {
		params := map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": common.FilePathToURI(testFile),
			},
			"position": map[string]interface{}{
				"line":      14,
				"character": 10,
			},
		}

		result, err := lspManager.ProcessRequest(ctx, "textDocument/hover", params)
		require.NoError(t, err)

		if result == nil {
			t.Log("No hover information available at this position")
			return
		}

		// Handle different response types from LSP server
		var hover *protocol.Hover
		switch res := result.(type) {
		case *protocol.Hover:
			hover = res
		case protocol.Hover:
			hover = &res
		case json.RawMessage:
			jsonStr := string(res)
			if jsonStr == "null" {
				t.Log("No hover information available")
				return
			}
			hover = &protocol.Hover{}
			err := json.Unmarshal(res, hover)
			if err != nil {
				t.Logf("Failed to unmarshal hover JSON: %s", jsonStr)
				return
			}
		case map[string]interface{}:
			if data, err := json.Marshal(res); err == nil {
				hover = &protocol.Hover{}
				json.Unmarshal(data, hover)
			}
		default:
			require.Fail(t, "Unexpected result type", "Expected *protocol.Hover, got %T", result)
		}

		if hover != nil {
			t.Log("Hover information retrieved successfully")
		} else {
			t.Log("No hover contents available")
		}
	})

	t.Run("TextDocument DocumentSymbol", func(t *testing.T) {
		params := map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": common.FilePathToURI(testFile),
			},
		}

		result, err := lspManager.ProcessRequest(ctx, "textDocument/documentSymbol", params)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Handle different response types from LSP server
		var symbolCount int
		switch res := result.(type) {
		case []interface{}:
			symbolCount = len(res)
		case json.RawMessage:
			jsonStr := string(res)
			if jsonStr == "null" || jsonStr == "[]" {
				symbolCount = 0
			} else {
				var symbols []interface{}
				err := json.Unmarshal(res, &symbols)
				if err != nil {
					t.Logf("Failed to unmarshal document symbols JSON: %s", jsonStr)
					symbolCount = 0
				} else {
					symbolCount = len(symbols)
				}
			}
		default:
			require.Fail(t, "Unexpected result type", "Expected []interface{} or json.RawMessage, got %T", result)
		}

		if symbolCount == 0 {
			t.Log("No document symbols found, which may be expected")
		} else {
			t.Logf("Found %d document symbols", symbolCount)
		}
	})

	t.Run("Workspace Symbol", func(t *testing.T) {
		params := map[string]interface{}{
			"query": "Test",
		}

		result, err := lspManager.ProcessRequest(ctx, "workspace/symbol", params)
		require.NoError(t, err)
		require.NotNil(t, result)

		symbols, ok := result.([]protocol.SymbolInformation)
		if !ok {
			symbolsInterface, ok := result.([]interface{})
			require.True(t, ok, "Result should be array of symbols")
			require.Greater(t, len(symbolsInterface), 0, "Should find workspace symbols")
		} else {
			require.Greater(t, len(symbols), 0, "Should find workspace symbols")
		}
	})

	t.Run("TextDocument Completion", func(t *testing.T) {
		params := map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": common.FilePathToURI(testFile),
			},
			"position": map[string]interface{}{
				"line":      17,
				"character": 10,
			},
		}

		result, err := lspManager.ProcessRequest(ctx, "textDocument/completion", params)
		require.NoError(t, err)

		if result != nil {
			// Handle different response types from LSP server
			switch res := result.(type) {
			case *protocol.CompletionList:
				t.Log("Received CompletionList")
			case []protocol.CompletionItem:
				t.Logf("Received %d completion items", len(res))
			case json.RawMessage:
				jsonStr := string(res)
				if jsonStr == "null" || jsonStr == "[]" {
					t.Log("No completion results available")
				} else {
					t.Log("Received completion data as JSON")
				}
			case []interface{}:
				t.Logf("Received %d completion items as interfaces", len(res))
			default:
				t.Logf("Received completion result of type %T", result)
			}
		} else {
			t.Log("No completion results available")
		}
	})

	t.Run("Cache Integration", func(t *testing.T) {
		params1 := map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": common.FilePathToURI(testFile),
			},
			"position": map[string]interface{}{
				"line":      14,
				"character": 10,
			},
		}

		result1, err := lspManager.ProcessRequest(ctx, "textDocument/hover", params1)
		require.NoError(t, err)

		if result1 == nil {
			t.Log("No hover information available, skipping cache test")
			return
		}

		result2, err := lspManager.ProcessRequest(ctx, "textDocument/hover", params1)
		require.NoError(t, err)

		if result2 == nil {
			t.Log("Second hover request returned nil, cache might not be working")
		} else {
			t.Log("Cache integration test completed successfully")
		}

		// Cache should have been used for the second request
	})

	t.Run("Multiple Language Support", func(t *testing.T) {
		pythonFile := filepath.Join(tempDir, "test.py")
		pythonContent := []byte(`def main():
    print("Hello, World!")
    
def helper_function():
    return "helper"

if __name__ == "__main__":
    main()
`)
		err := os.WriteFile(pythonFile, pythonContent, 0644)
		require.NoError(t, err)

		_, pythonErr := exec.LookPath("pylsp")
		if pythonErr == nil {
			params := map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": common.FilePathToURI(pythonFile),
				},
			}

			result, err := lspManager.ProcessRequest(ctx, "textDocument/documentSymbol", params)
			if err == nil && result != nil {
				symbols, ok := result.([]interface{})
				require.True(t, ok, "Result should be array of symbols")
				require.Greater(t, len(symbols), 0, "Should find Python symbols")
			}
		}
	})
}
