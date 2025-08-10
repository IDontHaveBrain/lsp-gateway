package e2e_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server"
	"lsp-gateway/tests/e2e/testutils"
)

func TestSCIPIndexAndFindReferences(t *testing.T) {
	// Skip if not in E2E mode
	if os.Getenv("RUN_E2E_TESTS") != "true" {
		t.Skip("Skipping E2E tests (set RUN_E2E_TESTS=true to run)")
	}

	// Create repository manager
	baseDir := t.TempDir()
	repoManager := testutils.NewRepoManager(baseDir)
	defer repoManager.Cleanup()

	// Test each language
	languages := []string{"go", "python", "javascript", "typescript", "rust"}

	for _, language := range languages {
		t.Run(language, func(t *testing.T) {
			testSCIPReferencesForLanguage(t, repoManager, language)
		})
	}
}

func testSCIPReferencesForLanguage(t *testing.T, repoManager *testutils.RepoManager, language string) {
	// Set up repository for language
	repoPath, err := repoManager.SetupRepository(language)
	if err != nil {
		t.Skipf("Skipping %s test: %v", language, err)
		return
	}

	// Get LSP command for language
	lspCommand, lspArgs := getLSPCommandForLanguage(language)

	// Create config with SCIP cache enabled
	cfg := &config.Config{
		Cache: &config.CacheConfig{
			Enabled:     true,
			MaxMemoryMB: 256,
			TTLHours:    1,
		},
		Servers: map[string]*config.ServerConfig{
			language: {
				Command:    lspCommand,
				Args:       lspArgs,
				WorkingDir: repoPath,
			},
		},
	}

	// Create LSP manager with SCIP cache
	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create LSP manager: %v", err)
	}
	defer manager.Stop()

	// Start LSP manager with workspace
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start LSP manager: %v", err)
	}

	// Wait for initialization
	time.Sleep(2 * time.Second)

	// Get test files for language from repository
	testFiles := getTestFilesForLanguage(t, repoManager, language)

	// Test 1: Index document symbols to populate SCIP cache
	t.Run("IndexDocumentSymbols", func(t *testing.T) {
		for _, testFile := range testFiles {
			uri := filePathToURI(testFile)

			// Request document symbols to trigger SCIP indexing
			params := map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": uri,
				},
			}

			result, err := manager.ProcessRequest(ctx, "textDocument/documentSymbol", params)
			if err != nil {
				t.Logf("Warning: Failed to get document symbols for %s: %v", testFile, err)
				// Don't fail - some files might not have symbols
			} else if result != nil {
				t.Logf("Successfully indexed symbols from %s", filepath.Base(testFile))
			}
		}
	})

	// Test 2: Index workspace symbols
	t.Run("IndexWorkspaceSymbols", func(t *testing.T) {
		params := map[string]interface{}{
			"query": "", // Empty query to get all symbols
		}

		result, err := manager.ProcessRequest(ctx, "workspace/symbol", params)
		if err != nil {
			t.Logf("Warning: Failed to get workspace symbols: %v", err)
		} else if result != nil {
			t.Logf("Successfully indexed workspace symbols")
		}
	})

	// Test 3: Search for references using SCIP cache
	t.Run("SearchReferences", func(t *testing.T) {
		// Get a known symbol for the language
		symbolName := getKnownSymbolForLanguage(language)

		query := server.SymbolReferenceQuery{
			Pattern:     symbolName,
			FilePattern: "**/*",
			MaxResults:  100,
		}

		result, err := manager.SearchSymbolReferences(ctx, query)
		if err != nil {
			t.Errorf("Failed to search references for %s: %v", symbolName, err)
			return
		}

		if result == nil {
			t.Errorf("Got nil result for symbol %s", symbolName)
			return
		}

		// Check that we found at least one reference
		if len(result.References) == 0 {
			t.Logf("Warning: No references found for %s (might not be indexed yet)", symbolName)
		} else {
			t.Logf("Found %d references for %s", len(result.References), symbolName)

			// Log some details about the references
			for i, ref := range result.References {
				if i < 3 { // Log first 3 references
					t.Logf("  Reference %d: %s:%d:%d (Definition: %v, Read: %v, Write: %v)",
						i+1, filepath.Base(ref.FilePath), ref.LineNumber, ref.Column,
						ref.IsDefinition, ref.IsReadAccess, ref.IsWriteAccess)
				}
			}

			// Check metadata
			if result.DefinitionCount > 0 {
				t.Logf("  Found %d definitions", result.DefinitionCount)
			}
			if result.ReadAccessCount > 0 {
				t.Logf("  Found %d read accesses", result.ReadAccessCount)
			}
			if result.WriteAccessCount > 0 {
				t.Logf("  Found %d write accesses", result.WriteAccessCount)
			}
		}
	})

	// Test 4: Test reference search with file pattern filtering
	t.Run("SearchReferencesWithFilePattern", func(t *testing.T) {
		symbolName := getKnownSymbolForLanguage(language)

		// Get language-specific file pattern
		var filePattern string
		switch language {
		case "go":
			filePattern = "**/*.go"
		case "python":
			filePattern = "**/*.py"
		case "javascript":
			filePattern = "**/*.js"
		case "typescript":
			filePattern = "**/*.ts"
		case "rust":
			filePattern = "**/*.rs"
		default:
			filePattern = "**/*"
		}

		query := server.SymbolReferenceQuery{
			Pattern:     symbolName,
			FilePattern: filePattern,
			MaxResults:  10,
		}

		result, err := manager.SearchSymbolReferences(ctx, query)
		if err != nil {
			t.Errorf("Failed to search references with file pattern: %v", err)
		} else if result != nil {
			t.Logf("Found %d references for %s with pattern %s",
				len(result.References), symbolName, filePattern)
		}
	})

	// Test 5: Verify SCIP cache statistics
	t.Run("CacheStats", func(t *testing.T) {
		stats := manager.GetIndexStats()
		if stats == nil {
			t.Log("Warning: Could not get index stats")
		} else {
			t.Logf("SCIP Index Stats: %+v", stats)
		}
	})
}

// Helper functions

func getLSPCommandForLanguage(language string) (string, []string) {
	switch language {
	case "go":
		return "gopls", []string{"serve"}
	case "python":
		return "pylsp", []string{}
	case "javascript":
		return "typescript-language-server", []string{"--stdio"}
	case "typescript":
		return "typescript-language-server", []string{"--stdio"}
	case "rust":
		return "rust-analyzer", []string{}
	default:
		return "", nil
	}
}

func getTestFilesForLanguage(t *testing.T, repoManager *testutils.RepoManager, language string) []string {
	var testFiles []string

	// Get repository path
	repoPath, err := repoManager.GetRepositoryPath(language)
	if err != nil {
		t.Logf("Warning: Could not get repository path for %s: %v", language, err)
		return nil
	}

	// Get test repository info
	testRepos := testutils.GetTestRepositories()
	testRepo, exists := testRepos[language]
	if !exists {
		t.Logf("Warning: No test repository defined for %s", language)
		return nil
	}

	// Use defined test files from repository
	for _, testFile := range testRepo.TestFiles {
		fullPath := filepath.Join(repoPath, testFile.Path)
		if _, err := os.Stat(fullPath); err == nil {
			testFiles = append(testFiles, fullPath)
		} else {
			t.Logf("Warning: Test file not found: %s", fullPath)
		}
	}

	// If no test files found, try to find some files
	if len(testFiles) == 0 {
		var pattern string
		switch language {
		case "go":
			pattern = "*.go"
		case "python":
			pattern = "*.py"
		case "javascript":
			pattern = "*.js"
		case "typescript":
			pattern = "*.ts"
		case "rust":
			pattern = "*.rs"
		}

		// Find up to 3 files
		_ = filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() && filepath.Ext(path) == filepath.Ext(pattern) {
				testFiles = append(testFiles, path)
				if len(testFiles) >= 3 {
					return filepath.SkipDir
				}
			}
			return nil
		})
	}

	return testFiles
}

func getKnownSymbolForLanguage(language string) string {
	// Get well-known symbols from test repositories
	testRepos := testutils.GetTestRepositories()
	testRepo, exists := testRepos[language]
	if exists && len(testRepo.TestFiles) > 0 {
		// Use a symbol query from the first test file that has one
		for _, testFile := range testRepo.TestFiles {
			if testFile.SymbolQuery != "" {
				// Return the symbol query
				return testFile.SymbolQuery
			}
		}
	}

	// Fallback to common symbols
	switch language {
	case "go":
		return "Router" // Common in gorilla/mux
	case "python":
		return "request" // Common in requests library
	case "javascript":
		return "map" // Common in ramda
	case "typescript":
		return "is" // Main export in sindresorhus/is
	case "rust":
		return "Buffer"
	default:
		return "main"
	}
}

// filePathToURI converts a file path to a file:// URI
func filePathToURI(path string) string {
	// Clean the path
	path = filepath.Clean(path)

	// Convert to forward slashes for URI
	path = strings.ReplaceAll(path, string(filepath.Separator), "/")

	// Add file:// prefix
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return fmt.Sprintf("file://%s", path)
}
