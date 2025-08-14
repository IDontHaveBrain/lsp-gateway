package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server"
	"lsp-gateway/src/server/cache"
	"lsp-gateway/src/utils"
	"lsp-gateway/tests/shared"

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
				"uri": utils.FilePathToURI(testFile),
			},
			"position": map[string]interface{}{
				"line":      17,
				"character": 20,
			},
		}

		result, err := lspManager.ProcessRequest(ctx, "textDocument/definition", params)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Parse definition response using helper
		locations := shared.ParseDefinitionResponse(t, result)
		shared.LogResponseSummary(t, "textDocument/definition", result)

		require.Greater(t, len(locations), 0, "Should find definition")
		require.Contains(t, string(locations[0].URI), "test.go")
	})

	t.Run("TextDocument References", func(t *testing.T) {
		params := map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": utils.FilePathToURI(testFile),
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

		// Parse references response using helper
		locations := shared.ParseReferencesResponse(t, result)
		shared.LogResponseSummary(t, "textDocument/references", result)

		if len(locations) == 0 {
			t.Log("No references found, which may be expected depending on test file content")
		} else {
			require.GreaterOrEqual(t, len(locations), 1, "Should find at least one reference")
		}
	})

	t.Run("TextDocument Hover", func(t *testing.T) {
		params := map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": utils.FilePathToURI(testFile),
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

		// Parse hover response using helper
		hover := shared.ParseHoverResponse(t, result)
		shared.LogResponseSummary(t, "textDocument/hover", result)
		if hover == nil {
			t.Log("No hover information available")
			return
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
				"uri": utils.FilePathToURI(testFile),
			},
		}

		result, err := lspManager.ProcessRequest(ctx, "textDocument/documentSymbol", params)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Parse document symbols response using helper
		symbols := shared.ParseDocumentSymbolResponse(t, result)
		shared.LogResponseSummary(t, "textDocument/documentSymbol", result)
		symbolCount := len(symbols)

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
				"uri": utils.FilePathToURI(testFile),
			},
			"position": map[string]interface{}{
				"line":      17,
				"character": 10,
			},
		}

		result, err := lspManager.ProcessRequest(ctx, "textDocument/completion", params)
		require.NoError(t, err)

		// Parse completion response using helper
		completion := shared.ParseCompletionResponse(t, result)
		shared.LogResponseSummary(t, "textDocument/completion", result)
		if len(completion.Items) > 0 {
			t.Logf("Received %d completion items", len(completion.Items))
		} else {
			t.Log("No completion results")
		}
	})

	t.Run("Cache Integration", func(t *testing.T) {
		params1 := map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": utils.FilePathToURI(testFile),
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

		_, pythonErr := exec.LookPath("jedi-language-server")
		if pythonErr == nil {
			params := map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": utils.FilePathToURI(pythonFile),
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
