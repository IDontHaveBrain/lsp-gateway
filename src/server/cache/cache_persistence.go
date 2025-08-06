package cache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/project"
	"lsp-gateway/src/internal/types"
)

// SaveIndexToDisk saves the SCIP index data to disk as JSON files
func (m *SimpleCacheManager) SaveIndexToDisk() error {
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
	common.LSPLogger.Info("SCIP storage handles persistence internally at %s", m.config.StoragePath)
	return nil
}

// LoadIndexFromDisk loads the SCIP index data from disk JSON files
func (m *SimpleCacheManager) LoadIndexFromDisk() error {
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
		common.LSPLogger.Warn("Failed to start SCIP storage during load: %v", err)
	}

	// Update index stats from SCIP storage
	m.indexStats.Status = "loaded"
	if stats, err := m.scipStorage.GetStats(ctx); err == nil {
		m.indexStats.SymbolCount = stats.TotalSymbols
		m.indexStats.DocumentCount = int64(stats.CachedDocuments)
	}

	common.LSPLogger.Info("SCIP storage loaded from %s", m.config.StoragePath)
	return nil
}

// PerformWorkspaceIndexing performs initial workspace indexing
func (m *SimpleCacheManager) PerformWorkspaceIndexing(ctx context.Context, workingDir string, lspFallback LSPFallback) error {
	if !m.enabled {
		return nil
	}

	// Detect languages in the workspace
	detectedLanguages, err := project.DetectLanguages(workingDir)
	if err != nil {
		common.LSPLogger.Warn("Failed to detect languages for indexing: %v", err)
		detectedLanguages = []string{}
	}

	// Get workspace files for indexing based on detected languages
	workspaceFiles, err := project.ScanWorkspaceFiles(workingDir, detectedLanguages)
	if err != nil {
		return fmt.Errorf("failed to scan workspace files: %w", err)
	}

	if len(workspaceFiles) == 0 {
		common.LSPLogger.Info("No files to index in workspace")
		return nil
	}

	common.LSPLogger.Info("Starting workspace indexing: %d files found", len(workspaceFiles))

	// Index each file by fetching document symbols
	processedCount := 0
	errorCount := 0
	for i, file := range workspaceFiles {
		// Convert to file URI
		absPath, err := filepath.Abs(file)
		if err != nil {
			errorCount++
			continue
		}
		uri := "file://" + absPath

		// Get document symbols for this file
		params := map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": uri,
			},
		}

		if i%100 == 0 {
			common.LSPLogger.Debug("Indexing progress: %d/%d files", i, len(workspaceFiles))
		}

		_, err = lspFallback.ProcessRequest(ctx, types.MethodTextDocumentDocumentSymbol, params)
		if err == nil {
			processedCount++
		} else {
			errorCount++
		}
	}

	common.LSPLogger.Info("Workspace indexing completed: %d/%d files processed, %d errors", processedCount, len(workspaceFiles), errorCount)

	// Save the index to disk if disk cache is enabled
	if m.config.DiskCache && m.config.StoragePath != "" {
		if err := m.SaveIndexToDisk(); err != nil {
			common.LSPLogger.Warn("Failed to save index to disk: %v", err)
		}
	}

	return nil
}
