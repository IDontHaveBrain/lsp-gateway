package scip

import (
	"context"
	"path/filepath"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/types"
)

// ExampleUsage demonstrates how to use the simple SCIP storage
func ExampleUsage() error {
	// Create configuration
	config := SCIPStorageConfig{
		MemoryLimit:   256 * 1024 * 1024, // 256MB
		DiskCacheDir:  "/tmp/lsp-gateway-scip",
		EnableMetrics: true,
	}

	// Create simple storage
	storage, err := NewSimpleSCIPStorage(config)
	if err != nil {
		return err
	}

	// Start the storage manager
	ctx := context.Background()
	if err := storage.Start(ctx); err != nil {
		return err
	}
	defer storage.Stop(ctx)

	// Create a sample occurrence-centric SCIP document
	doc := &SCIPDocument{
		URI:      "file:///example/main.go",
		Language: "go",
		Content:  []byte("package main\n\nfunc main() {}\n"),
		Occurrences: []SCIPOccurrence{
			{
				Range: types.Range{
					Start: types.Position{Line: 2, Character: 5},
					End:   types.Position{Line: 2, Character: 9},
				},
				Symbol:      "main.main",
				SymbolRoles: types.SymbolRoleDefinition,
				SyntaxKind:  types.SyntaxKindIdentifierFunction,
			},
		},
		SymbolInformation: []SCIPSymbolInformation{
			{
				Symbol:        "main.main",
				Kind:          SCIPSymbolKindFunction,
				DisplayName:   "main",
				Documentation: []string{"Main function"},
				SignatureDocumentation: SCIPSignatureDocumentation{
					Text:     "func main()",
					Language: "go",
				},
			},
		},
		LastModified: time.Now(),
		Size:         25, // Size of content
	}

	// Store the document
	if err := storage.StoreDocument(ctx, doc); err != nil {
		common.LSPLogger.Error("Failed to store document: %v", err)
		return err
	}

	common.LSPLogger.Info("Successfully stored SCIP document: %s", doc.URI)

	// Retrieve the document
	retrievedDoc, err := storage.GetDocument(ctx, doc.URI)
	if err != nil {
		common.LSPLogger.Error("Failed to retrieve document: %v", err)
		return err
	}

	common.LSPLogger.Info("Successfully retrieved document: %s", retrievedDoc.URI)

	// Get occurrences in the document
	occurrences, err := storage.GetOccurrences(ctx, doc.URI)
	if err != nil {
		common.LSPLogger.Error("Failed to get occurrences: %v", err)
		return err
	}

	common.LSPLogger.Info("Found %d occurrences in document", len(occurrences))

	// Get symbol information by name
	symbolInfo, err := storage.GetSymbolInformationByName(ctx, "main")
	if err != nil {
		common.LSPLogger.Error("Failed to get symbol information by name: %v", err)
		return err
	}

	common.LSPLogger.Info("Found %d symbol information entries for 'main'", len(symbolInfo))

	// Get definition occurrence
	definition, err := storage.GetDefinitionOccurrence(ctx, "main.main")
	if err != nil {
		common.LSPLogger.Error("Failed to get definition occurrence: %v", err)
		return err
	}

	common.LSPLogger.Info("Found definition for symbol 'main.main' at line %d", definition.Range.Start.Line)

	// Search symbols
	searchResults, err := storage.SearchSymbols(ctx, "main", 10)
	if err != nil {
		common.LSPLogger.Error("Failed to search symbols: %v", err)
		return err
	}

	common.LSPLogger.Info("Found %d symbols matching 'main'", len(searchResults))

	// Get storage statistics
	stats, err := storage.GetStats(ctx)
	if err != nil {
		common.LSPLogger.Error("Failed to get storage stats: %v", err)
		return err
	}

	common.LSPLogger.Info("Storage stats - Memory: %d MB, Disk: %d MB, Documents: %d, Occurrences: %d, Symbols: %d, Hit Rate: %.2f%%",
		stats.MemoryUsage/(1024*1024),
		stats.DiskUsage/(1024*1024),
		stats.CachedDocuments,
		stats.TotalOccurrences,
		stats.TotalSymbols,
		stats.HitRate*100)

	// Perform health check
	if err := storage.HealthCheck(ctx); err != nil {
		common.LSPLogger.Warn("Health check failed: %v", err)
	} else {
		common.LSPLogger.Info("Storage health check passed")
	}

	// Trigger compaction (no-op for simple storage)
	if err := storage.Compact(ctx); err != nil {
		common.LSPLogger.Error("Compaction failed: %v", err)
	} else {
		common.LSPLogger.Info("Storage compaction completed (no-op)")
	}

	return nil
}

// DefaultSCIPStorageConfig returns a default configuration suitable for most use cases
func DefaultSCIPStorageConfig() SCIPStorageConfig {
	return SCIPStorageConfig{
		MemoryLimit:   256 * 1024 * 1024, // 256MB
		DiskCacheDir:  filepath.Join("/tmp", "lsp-gateway-scip-cache"),
		EnableMetrics: true,
	}
}
