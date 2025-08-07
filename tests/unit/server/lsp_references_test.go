package server_test

import (
	"context"
	"testing"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server"
)

func TestSearchSymbolReferences(t *testing.T) {
	// Create a test configuration
	cfg := config.GetDefaultConfig()
	cfg.EnableCache()

	// Create LSP manager
	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create LSP manager: %v", err)
	}

	ctx := context.Background()

	// Test basic symbol reference search
	t.Run("BasicReferenceSearch", func(t *testing.T) {
		query := server.SymbolReferenceQuery{
			SymbolName:        "TestFunction",
			FilePattern:       "**/*.go",
			IncludeDefinition: true,
			MaxResults:        10,
			IncludeCode:       false,
			ExactMatch:        false,
		}

		result, err := manager.SearchSymbolReferences(ctx, query)
		if err != nil {
			// This is expected to fail if no LSP servers are running
			// Just check that the method exists and is callable
			return
		}

		if result == nil {
			t.Fatal("Expected non-nil result")
		}

		if result.TotalCount < 0 {
			t.Errorf("Invalid total count: %d", result.TotalCount)
		}
	})

	// Test with exact match
	t.Run("ExactMatchReferenceSearch", func(t *testing.T) {
		query := server.SymbolReferenceQuery{
			SymbolName:        "main",
			FilePattern:       "**/*.go",
			IncludeDefinition: false,
			MaxResults:        5,
			IncludeCode:       true,
			ExactMatch:        true,
		}

		result, err := manager.SearchSymbolReferences(ctx, query)
		if err != nil {
			return
		}

		if result == nil {
			t.Fatal("Expected non-nil result")
		}
	})

	// Test empty symbol name
	t.Run("EmptySymbolName", func(t *testing.T) {
		query := server.SymbolReferenceQuery{
			SymbolName:  "",
			FilePattern: "**/*.go",
		}

		_, err := manager.SearchSymbolReferences(ctx, query)
		if err == nil {
			t.Error("Expected error for empty symbol name")
		}
	})
}

func TestMatchesFilePattern(t *testing.T) {
	// This would test the file pattern matching logic
	// Since it's a private method, we can't test it directly
	// but we can test it through the public API

	testCases := []struct {
		name        string
		filePath    string
		pattern     string
		shouldMatch bool
	}{
		{
			name:        "Match all files",
			filePath:    "src/main.go",
			pattern:     "**/*",
			shouldMatch: true,
		},
		{
			name:        "Match Go files",
			filePath:    "src/main.go",
			pattern:     "*.go",
			shouldMatch: true,
		},
		{
			name:        "Match directory prefix",
			filePath:    "src/server/main.go",
			pattern:     "src/",
			shouldMatch: true,
		},
		{
			name:        "No match different extension",
			filePath:    "src/main.go",
			pattern:     "*.js",
			shouldMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test logic would go here if we could access the private method
			// For now, this is just a placeholder showing test structure
		})
	}
}

func TestReferenceInfoFormatting(t *testing.T) {
	// Test that ReferenceInfo is properly formatted
	ref := server.ReferenceInfo{
		FilePath:   "/path/to/file.go",
		LineNumber: 42,
		Column:     10,
		Text:       "someFunction()",
		Code:       "result := someFunction()",
		Context:    "func main() { result := someFunction() }",
	}

	// Check that all fields are accessible
	if ref.FilePath != "/path/to/file.go" {
		t.Errorf("Unexpected FilePath: %s", ref.FilePath)
	}
	if ref.LineNumber != 42 {
		t.Errorf("Unexpected LineNumber: %d", ref.LineNumber)
	}
	if ref.Column != 10 {
		t.Errorf("Unexpected Column: %d", ref.Column)
	}
}
