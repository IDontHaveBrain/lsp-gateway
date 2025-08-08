package integration

import (
	"testing"
	"time"

	"lsp-gateway/src/server"
	"lsp-gateway/tests/shared"
)

func TestFindReferencesWithSCIPCache(t *testing.T) {
	// Create test workspace with Go project
	workspace := shared.CreateBasicGoProject(t, "testproject")
	defer workspace.Cleanup()

	// Create LSP manager with cache
	setup := shared.CreateLSPManagerWithCache(t, workspace.TempDir)
	setup.Start(t)
	defer setup.Stop()

	ctx := setup.Context

	// Get main file URI and content
	fileURI := workspace.GetFileURI("main.go")
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

	// Open the document
	setup.OpenDocument(t, fileURI, "go", 1, mainContent)

	// Request document symbols to populate cache
	symbolResult := setup.IndexDocument(t, fileURI)
	t.Logf("Document symbols indexed: %v", symbolResult != nil)

	// Wait for indexing
	time.Sleep(1 * time.Second)

	// Test SearchSymbolReferences for "Router"
	query := server.SymbolReferenceQuery{
		Pattern:     "Router",
		FilePattern: "*.go",
		MaxResults:  100,
	}

	result, err := setup.Manager.SearchSymbolReferences(ctx, query)
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
	if setup.Cache != nil {
		stats := setup.Cache.GetIndexStats()
		t.Logf("Cache stats: %d documents, %d symbols, %d references",
			stats.DocumentCount, stats.SymbolCount, stats.ReferenceCount)
	}

	// Close the document
	setup.CloseDocument(t, fileURI)
}
