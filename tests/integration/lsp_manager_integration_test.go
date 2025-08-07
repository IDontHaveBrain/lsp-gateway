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
		params := protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: protocol.DocumentURI("file://" + testFile),
				},
				Position: protocol.Position{
					Line:      17,
					Character: 20,
				},
			},
		}

		result, err := lspManager.ProcessRequest(ctx, "textDocument/definition", params)
		require.NoError(t, err)
		require.NotNil(t, result)

		locations, ok := result.([]protocol.Location)
		if !ok {
			location, ok := result.(protocol.Location)
			require.True(t, ok, "Result should be Location or []Location")
			locations = []protocol.Location{location}
		}

		require.Greater(t, len(locations), 0, "Should find definition")
		require.Contains(t, string(locations[0].URI), "test.go")
	})

	t.Run("TextDocument References", func(t *testing.T) {
		params := protocol.ReferenceParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: protocol.DocumentURI("file://" + testFile),
				},
				Position: protocol.Position{
					Line:      23,
					Character: 5,
				},
			},
			Context: protocol.ReferenceContext{
				IncludeDeclaration: true,
			},
		}

		result, err := lspManager.ProcessRequest(ctx, "textDocument/references", params)
		require.NoError(t, err)
		require.NotNil(t, result)

		locations, ok := result.([]protocol.Location)
		require.True(t, ok, "Result should be []Location")
		require.GreaterOrEqual(t, len(locations), 1, "Should find at least one reference")
	})

	t.Run("TextDocument Hover", func(t *testing.T) {
		params := protocol.HoverParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: protocol.DocumentURI("file://" + testFile),
				},
				Position: protocol.Position{
					Line:      14,
					Character: 10,
				},
			},
		}

		result, err := lspManager.ProcessRequest(ctx, "textDocument/hover", params)
		require.NoError(t, err)
		require.NotNil(t, result)

		hover, ok := result.(*protocol.Hover)
		require.True(t, ok, "Result should be *Hover")
		require.NotNil(t, hover.Contents)
	})

	t.Run("TextDocument DocumentSymbol", func(t *testing.T) {
		params := protocol.DocumentSymbolParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: protocol.DocumentURI("file://" + testFile),
			},
		}

		result, err := lspManager.ProcessRequest(ctx, "textDocument/documentSymbol", params)
		require.NoError(t, err)
		require.NotNil(t, result)

		symbols, ok := result.([]interface{})
		require.True(t, ok, "Result should be array of symbols")
		require.Greater(t, len(symbols), 0, "Should find document symbols")
	})

	t.Run("Workspace Symbol", func(t *testing.T) {
		params := protocol.WorkspaceSymbolParams{
			Query: "Test",
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
		params := protocol.CompletionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: protocol.DocumentURI("file://" + testFile),
				},
				Position: protocol.Position{
					Line:      17,
					Character: 10,
				},
			},
		}

		result, err := lspManager.ProcessRequest(ctx, "textDocument/completion", params)
		require.NoError(t, err)

		if result != nil {
			completionList, ok := result.(*protocol.CompletionList)
			if ok {
				require.NotNil(t, completionList)
			} else {
				items, ok := result.([]protocol.CompletionItem)
				require.True(t, ok, "Result should be CompletionList or []CompletionItem")
				require.NotNil(t, items)
			}
		}
	})

	t.Run("Cache Integration", func(t *testing.T) {
		params1 := protocol.HoverParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: protocol.DocumentURI("file://" + testFile),
				},
				Position: protocol.Position{
					Line:      14,
					Character: 10,
				},
			},
		}

		result1, err := lspManager.ProcessRequest(ctx, "textDocument/hover", params1)
		require.NoError(t, err)
		require.NotNil(t, result1)

		result2, err := lspManager.ProcessRequest(ctx, "textDocument/hover", params1)
		require.NoError(t, err)
		require.NotNil(t, result2)

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
			params := protocol.DocumentSymbolParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: protocol.DocumentURI("file://" + pythonFile),
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
