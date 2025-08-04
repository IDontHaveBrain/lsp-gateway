package scip

import (
	"context"
	"path/filepath"
	"time"

	"lsp-gateway/src/internal/common"
)

// ExampleUsage demonstrates how to use the SCIP storage manager
func ExampleUsage() error {
	// Create configuration
	config := SCIPStorageConfig{
		MemoryLimit:        256 * 1024 * 1024, // 256MB
		DiskCacheDir:       "/tmp/lsp-gateway-scip",
		CompressionType:    "gzip",
		CompactionInterval: 30 * time.Minute,
		MaxDocumentAge:     24 * time.Hour,
		EnableMetrics:      true,
	}

	// Create storage manager
	storage, err := NewSCIPStorageManager(config)
	if err != nil {
		return err
	}

	// Start the storage manager
	ctx := context.Background()
	if err := storage.Start(ctx); err != nil {
		return err
	}
	defer storage.Stop(ctx)

	// Create a sample SCIP document
	doc := &SCIPDocument{
		URI:      "file:///example/main.go",
		Language: "go",
		Content:  []byte("package main\n\nfunc main() {}\n"),
		Symbols: []SCIPSymbol{
			{
				ID:   "main.main",
				Name: "main",
				Kind: 12, // Function kind
				Range: SCIPRange{
					Start: SCIPPosition{Line: 2, Character: 5},
					End:   SCIPPosition{Line: 2, Character: 9},
				},
				Definition: SCIPRange{
					Start: SCIPPosition{Line: 2, Character: 5},
					End:   SCIPPosition{Line: 2, Character: 9},
				},
				Documentation: "Main function",
			},
		},
		References: []SCIPReference{
			{
				Symbol: "main.main",
				Range: SCIPRange{
					Start: SCIPPosition{Line: 2, Character: 5},
					End:   SCIPPosition{Line: 2, Character: 9},
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

	// Get symbols by name
	symbols, err := storage.GetSymbolsByName(ctx, "main")
	if err != nil {
		common.LSPLogger.Error("Failed to get symbols by name: %v", err)
		return err
	}

	common.LSPLogger.Info("Found %d symbols named 'main'", len(symbols))

	// Get references
	references, err := storage.GetReferences(ctx, "main.main")
	if err != nil {
		common.LSPLogger.Error("Failed to get references: %v", err)
		return err
	}

	common.LSPLogger.Info("Found %d references for symbol 'main.main'", len(references))

	// Get storage statistics
	stats, err := storage.GetStats(ctx)
	if err != nil {
		common.LSPLogger.Error("Failed to get storage stats: %v", err)
		return err
	}

	common.LSPLogger.Info("Storage stats - Memory: %d MB, Disk: %d MB, Documents: %d, Hit Rate: %.2f%%",
		stats.MemoryUsage/(1024*1024),
		stats.DiskUsage/(1024*1024),
		stats.CachedDocuments,
		stats.HitRate*100)

	// Perform health check
	if err := storage.HealthCheck(ctx); err != nil {
		common.LSPLogger.Warn("Health check failed: %v", err)
	} else {
		common.LSPLogger.Info("Storage health check passed")
	}

	// Trigger compaction
	if err := storage.Compact(ctx); err != nil {
		common.LSPLogger.Error("Compaction failed: %v", err)
	} else {
		common.LSPLogger.Info("Storage compaction completed")
	}

	return nil
}

// DefaultSCIPStorageConfig returns a default configuration suitable for most use cases
func DefaultSCIPStorageConfig() SCIPStorageConfig {
	return SCIPStorageConfig{
		MemoryLimit:        256 * 1024 * 1024, // 256MB
		DiskCacheDir:       filepath.Join("/tmp", "lsp-gateway-scip-cache"),
		CompressionType:    "gzip",
		CompactionInterval: 30 * time.Minute,
		MaxDocumentAge:     24 * time.Hour,
		EnableMetrics:      true,
	}
}
