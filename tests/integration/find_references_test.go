package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server"
	"lsp-gateway/src/server/cache"
)

func TestFindReferencesWithSCIPCache(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "lsp-gateway-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test Go files
	mainFile := filepath.Join(tmpDir, "main.go")
	mainContent := `package main

import "fmt"

type Router struct {
	routes map[string]string
}

func NewRouter() *Router {
	return &Router{
		routes: make(map[string]string),
	}
}

func (r *Router) Handle(path string) {
	fmt.Println("Handling", path)
}

func main() {
	r := NewRouter()
	r.Handle("/home")
}
`
	if err := os.WriteFile(mainFile, []byte(mainContent), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	// Create go.mod for language detection
	goModFile := filepath.Join(tmpDir, "go.mod")
	goModContent := `module testproject

go 1.21
`
	if err := os.WriteFile(goModFile, []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Change to test directory
	originalWD, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalWD)

	// Create configuration
	cfg := &config.Config{
		Cache: &config.CacheConfig{
			Enabled:     true,
			MaxMemoryMB: 128,
			TTLHours:    1,
		},
		Servers: map[string]*config.ServerConfig{
			"go": {
				Command: "gopls",
				Args:    []string{"serve"},
			},
		},
	}

	// Create cache manager
	cacheManager, err := cache.NewSCIPCacheManager(cfg.Cache)
	if err != nil {
		t.Fatalf("Failed to create cache manager: %v", err)
	}

	// Start cache manager
	ctx := context.Background()
	if err := cacheManager.Start(ctx); err != nil {
		t.Fatalf("Failed to start cache manager: %v", err)
	}
	defer cacheManager.Stop()

	// Create LSP manager
	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create LSP manager: %v", err)
	}
	manager.SetCache(cacheManager)

	// Start LSP manager
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start LSP manager: %v", err)
	}
	defer manager.Stop()

	// Wait for initialization
	time.Sleep(2 * time.Second)

	// Index the test file
	fileURI := "file://" + mainFile

	// Open the document
	didOpenParams := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        fileURI,
			"languageId": "go",
			"version":    1,
			"text":       mainContent,
		},
	}
	_, err = manager.ProcessRequest(ctx, "textDocument/didOpen", didOpenParams)
	if err != nil {
		t.Logf("Warning: didOpen failed: %v", err)
	}

	// Request document symbols to populate cache
	symbolParams := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fileURI,
		},
	}

	symbolResult, err := manager.ProcessRequest(ctx, "textDocument/documentSymbol", symbolParams)
	if err != nil {
		t.Logf("Warning: documentSymbol failed: %v", err)
	} else {
		t.Logf("Document symbols indexed: %v", symbolResult != nil)
	}

	// Wait for indexing
	time.Sleep(1 * time.Second)

	// Test SearchSymbolReferences for "Router"
	query := server.SymbolReferenceQuery{
		Pattern:     "Router",
		FilePattern: "*.go",
		MaxResults:  100,
	}

	result, err := manager.SearchSymbolReferences(ctx, query)
	if err != nil {
		t.Logf("SearchSymbolReferences error: %v", err)
	}

	// Verify results
	if result == nil {
		t.Fatalf("SearchSymbolReferences returned nil result")
	}

	t.Logf("Found %d references to 'Router'", len(result.References))
	t.Logf("Search metadata: %+v", result.SearchMetadata)

	// We should find at least 2 references:
	// 1. The type definition at line 5
	// 2. The NewRouter function return type at line 10
	// 3. Usage in main() at line 20
	if len(result.References) < 2 {
		t.Errorf("Expected at least 2 references to Router, got %d", len(result.References))
		for i, ref := range result.References {
			t.Logf("  Reference %d: %s:%d:%d", i+1, ref.FilePath, ref.LineNumber, ref.Column)
		}
	}

	// Check cache statistics
	stats := cacheManager.GetIndexStats()
	t.Logf("Cache stats: %d documents, %d symbols, %d references",
		stats.DocumentCount, stats.SymbolCount, stats.ReferenceCount)

	// Close the document
	didCloseParams := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fileURI,
		},
	}
	_, _ = manager.ProcessRequest(ctx, "textDocument/didClose", didCloseParams)
}
