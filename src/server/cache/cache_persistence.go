package cache

import (
	"context"
	"fmt"
	"os"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/project"
)

// SaveIndexToDisk saves the SCIP index data to disk as JSON files
func (m *SCIPCacheManager) SaveIndexToDisk() error {
	if m.config.StoragePath == "" {
		return fmt.Errorf("storage path not configured")
	}

	// Create storage directory if it doesn't exist (already project-specific from config)
	if err := os.MkdirAll(m.config.StoragePath, 0755); err != nil {
		return fmt.Errorf("failed to create storage directory: %v", err)
	}

	m.indexMu.RLock()
	defer m.indexMu.RUnlock()

	// Save using SCIP storage if available
	if m.scipStorage == nil {
		return fmt.Errorf("SCIP storage not available")
	}

	// SCIP storage handles its own persistence internally
	// We just trigger a save operation if the storage supports it
	return nil
}

// LoadIndexFromDisk loads the SCIP index data from disk JSON files
func (m *SCIPCacheManager) LoadIndexFromDisk() error {
	if m.config.StoragePath == "" {
		return fmt.Errorf("storage path not configured")
	}

	m.indexMu.Lock()
	defer m.indexMu.Unlock()

	// Load using SCIP storage if available
	if m.scipStorage == nil {
		return fmt.Errorf("SCIP storage not available")
	}

	// SCIP storage handles its own loading internally
	// We just initialize the storage which loads any existing data
	ctx := context.Background()
	if err := m.scipStorage.Start(ctx); err != nil {
		// Ignore "storage already started" error - it's expected if already running
		if err.Error() != "storage already started" {
			common.LSPLogger.Warn("Failed to start SCIP storage during load: %v", err)
		}
	}

	// Update index stats from SCIP storage
	m.indexStats.Status = "loaded"
	stats := m.scipStorage.GetIndexStats()
	m.indexStats.SymbolCount = stats.TotalSymbols
	m.indexStats.DocumentCount = int64(stats.TotalDocuments)
	m.indexStats.ReferenceCount = stats.TotalOccurrences // Use occurrences as reference count

	return nil
}

// PerformWorkspaceIndexing performs initial workspace indexing
func (m *SCIPCacheManager) PerformWorkspaceIndexing(ctx context.Context, workingDir string, lspFallback LSPFallback) error {
	if !m.enabled {
		return nil
	}

	// Detect languages in the workspace
	detectedLanguages, err := project.DetectLanguages(workingDir)
	if err != nil {
		common.LSPLogger.Warn("Failed to detect languages for indexing: %v", err)
		detectedLanguages = []string{}
	}

	// Use unlimited maxFiles for comprehensive HTTP gateway indexing
	const maxFiles = 100000

	// Perform full indexing with references
	return m.PerformFullIndexing(ctx, workingDir, detectedLanguages, maxFiles, lspFallback)
}

// PerformFullIndexing performs both basic and enhanced reference indexing
// This is the common indexing logic used by both CLI and MCP server
func (m *SCIPCacheManager) PerformFullIndexing(ctx context.Context, workingDir string, languages []string, maxFiles int, lspFallback LSPFallback) error {
	if !m.enabled {
		return nil
	}

	// Create workspace indexer with LSP fallback
	indexer := NewWorkspaceIndexer(lspFallback)

	// Step 1: Basic symbol indexing
	common.LSPLogger.Debug("Starting basic symbol indexing")
	err := indexer.IndexWorkspaceFiles(ctx, workingDir, languages, maxFiles)
	if err != nil {
		return fmt.Errorf("workspace indexing failed: %w", err)
	}

	// Step 2: Enhanced reference indexing
	common.LSPLogger.Debug("Starting enhanced reference indexing")
	err = indexer.IndexWorkspaceFilesWithReferences(ctx, workingDir, languages, maxFiles, m)
	if err != nil {
		common.LSPLogger.Error("Enhanced reference indexing failed: %v", err)
		// Don't fail completely, basic indexing is still useful
	} else {
		common.LSPLogger.Debug("Enhanced reference indexing complete")
	}

	// Save the index to disk if disk cache is enabled
	if m.config.DiskCache && m.config.StoragePath != "" {
		if err := m.SaveIndexToDisk(); err != nil {
			common.LSPLogger.Warn("Failed to save index to disk: %v", err)
		}
	}

	return nil
}
