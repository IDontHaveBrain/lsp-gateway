package integration

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server"
)

func TestNewLSPManagerReferences(t *testing.T) {
	// Get the project root directory (two levels up from tests/integration)
	_, currentFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(currentFile), "..", "..")

	// Create config with SCIP cache enabled
	cfg := &config.Config{
		Cache: &config.CacheConfig{
			Enabled:     true,
			MaxMemoryMB: 256,
			TTLHours:    1,
		},
		Servers: map[string]*config.ServerConfig{
			"go": {
				Command:    "gopls",
				Args:       []string{"serve"},
				WorkingDir: projectRoot,
			},
		},
	}

	// Create LSP manager
	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create LSP manager: %v", err)
	}
	defer manager.Stop()

	// Start LSP manager
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start LSP manager: %v", err)
	}

	// Wait for gopls to initialize
	time.Sleep(2 * time.Second)

	// First, index the file containing NewLSPManager definition
	t.Run("IndexDefinition", func(t *testing.T) {
		uri := "file://" + filepath.Join(projectRoot, "src", "server", "lsp_manager.go")

		// Request document symbols to trigger SCIP indexing
		params := map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": uri,
			},
		}

		result, err := manager.ProcessRequest(ctx, "textDocument/documentSymbol", params)
		if err != nil {
			t.Logf("Warning: Failed to index lsp_manager.go: %v", err)
		} else if result != nil {
			t.Logf("Successfully indexed lsp_manager.go")
		}
	})

	// Index files that use NewLSPManager
	t.Run("IndexUsages", func(t *testing.T) {
		files := []string{
			filepath.Join(projectRoot, "src", "server", "gateway.go"),
			filepath.Join(projectRoot, "src", "server", "mcp_server.go"),
			filepath.Join(projectRoot, "src", "cli", "server_commands.go"),
		}

		for _, file := range files {
			uri := "file://" + file
			params := map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": uri,
				},
			}

			result, err := manager.ProcessRequest(ctx, "textDocument/documentSymbol", params)
			if err != nil {
				t.Logf("Warning: Failed to index %s: %v", filepath.Base(file), err)
			} else if result != nil {
				t.Logf("Successfully indexed %s", filepath.Base(file))
			}
		}
	})

	// Also try workspace symbol indexing
	t.Run("IndexWorkspace", func(t *testing.T) {
		params := map[string]interface{}{
			"query": "NewLSPManager",
		}

		result, err := manager.ProcessRequest(ctx, "workspace/symbol", params)
		if err != nil {
			t.Logf("Warning: workspace/symbol failed: %v", err)
		} else if result != nil {
			t.Logf("Workspace symbols found for NewLSPManager")
		}
	})

	// Index references using LSP textDocument/references
	t.Run("IndexReferencesViaLSP", func(t *testing.T) {
		// The NewLSPManager function is at line 52 (1-based), which is line 51 (0-based)
		uri := "file://" + filepath.Join(projectRoot, "src", "server", "lsp_manager.go")

		params := map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": uri,
			},
			"position": map[string]interface{}{
				"line":      51, // Line 52 in 0-based indexing
				"character": 5,  // Position on 'N' of 'NewLSPManager'
			},
			"context": map[string]interface{}{
				"includeDeclaration": true,
			},
		}

		result, err := manager.ProcessRequest(ctx, "textDocument/references", params)
		if err != nil {
			t.Logf("Warning: textDocument/references failed: %v", err)
		} else if result != nil {
			t.Logf("LSP references found: %+v", result)
			// This should trigger SCIP indexing of references
		}
	})

	// Wait for indexing to complete
	time.Sleep(1 * time.Second)

	// Test findReferences - this is failing
	t.Run("FindReferences", func(t *testing.T) {
		query := server.SymbolReferenceQuery{
			Pattern:     "NewLSPManager",
			FilePattern: "**/*.go",
			MaxResults:  5,
		}

		result, err := manager.SearchSymbolReferences(ctx, query)
		if err != nil {
			t.Fatalf("Failed to search references: %v", err)
		}

		if result == nil {
			t.Fatal("Got nil result")
		}

		// Log cache stats for debugging
		stats := manager.GetIndexStats()
		t.Logf("Cache stats: %+v", stats)

		if len(result.References) == 0 {
			t.Error("✗ Expected to find references to NewLSPManager, but got none")

			// Debug: Try different patterns
			t.Log("Debug: Trying alternative searches...")

			// Try searching for just "LSPManager"
			altQuery := server.SymbolReferenceQuery{
				Pattern:     "LSPManager",
				FilePattern: "**/*.go",
				MaxResults:  5,
			}
			altResult, _ := manager.SearchSymbolReferences(ctx, altQuery)
			if altResult != nil && len(altResult.References) > 0 {
				t.Logf("Found %d references for 'LSPManager'", len(altResult.References))
			}

			// Try exact match
			exactQuery := server.SymbolReferenceQuery{
				Pattern:     "func NewLSPManager",
				FilePattern: "**/*.go",
				MaxResults:  5,
			}
			exactResult, _ := manager.SearchSymbolReferences(ctx, exactQuery)
			if exactResult != nil && len(exactResult.References) > 0 {
				t.Logf("Found %d references for 'func NewLSPManager'", len(exactResult.References))
			}
		} else {
			t.Logf("✓ Found %d references to NewLSPManager", len(result.References))
			for i, ref := range result.References {
				if i < 5 {
					t.Logf("  Reference %d: %s:%d:%d (Def: %v, Read: %v, Write: %v)",
						i+1, ref.FilePath, ref.LineNumber, ref.Column,
						ref.IsDefinition, ref.IsReadAccess, ref.IsWriteAccess)
				}
			}
		}
	})
}
