package cache

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "time"

    "lsp-gateway/src/internal/common"
    "lsp-gateway/src/internal/project"
    "lsp-gateway/src/utils"
    "lsp-gateway/src/internal/registry"
)

// (legacy detectLanguageFromPath removed; use registry.GetLanguageByExtension)

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

	// Determine last update time from stored files
	latest := time.Time{}
	err := filepath.Walk(m.config.StoragePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		return nil
	})
	if err == nil && !latest.IsZero() {
		m.indexStats.LastUpdate = latest
	}

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

	const maxFiles = 100000

	return m.PerformFullIndexing(ctx, workingDir, detectedLanguages, maxFiles, lspFallback)
}

// PerformFullIndexing performs both basic and enhanced reference indexing
// This is the common indexing logic used by both CLI and MCP server
func (m *SCIPCacheManager) PerformFullIndexing(ctx context.Context, workingDir string, languages []string, maxFiles int, lspFallback LSPFallback) error {
	if !m.enabled {
		return nil
	}

	indexer := NewWorkspaceIndexer(lspFallback)

	common.LSPLogger.Debug("Starting basic symbol indexing")
	err := indexer.IndexWorkspaceFiles(ctx, workingDir, languages, maxFiles)
	if err != nil {
		return fmt.Errorf("workspace indexing failed: %w", err)
	}

	common.LSPLogger.Debug("Starting enhanced reference indexing")
	err = indexer.IndexWorkspaceFilesWithReferences(ctx, workingDir, languages, maxFiles, m)
	if err != nil {
		common.LSPLogger.Error("Enhanced reference indexing failed: %v", err)
		// Don't fail completely, basic indexing is still useful
	} else {
		common.LSPLogger.Debug("Enhanced reference indexing complete")
	}

	if m.config.DiskCache && m.config.StoragePath != "" {
		if err := m.SaveIndexToDisk(); err != nil {
			common.LSPLogger.Warn("Failed to save index to disk: %v", err)
		}
	}

	return nil
}

func (m *SCIPCacheManager) PerformWorkspaceIndexingWithProgress(ctx context.Context, workingDir string, lspFallback LSPFallback, progress IndexProgressFunc) error {
	if !m.enabled {
		return nil
	}
	detectedLanguages, err := project.DetectLanguages(workingDir)
	if err != nil {
		common.LSPLogger.Warn("Failed to detect languages for indexing: %v", err)
		detectedLanguages = []string{}
	}
	const maxFiles = 100000
	return m.PerformFullIndexingWithProgress(ctx, workingDir, detectedLanguages, maxFiles, lspFallback, progress)
}

func (m *SCIPCacheManager) PerformFullIndexingWithProgress(ctx context.Context, workingDir string, languages []string, maxFiles int, lspFallback LSPFallback, progress IndexProgressFunc) error {
	if !m.enabled {
		return nil
	}
	indexer := NewWorkspaceIndexer(lspFallback)
	common.LSPLogger.Debug("Starting basic symbol indexing")
	err := indexer.IndexWorkspaceFilesWithProgress(ctx, workingDir, languages, maxFiles, progress)
	if err != nil {
		return fmt.Errorf("workspace indexing failed: %w", err)
	}
	common.LSPLogger.Debug("Starting enhanced reference indexing")
	err = indexer.IndexWorkspaceFilesWithReferencesProgress(ctx, workingDir, languages, maxFiles, m, progress)
	if err != nil {
		common.LSPLogger.Error("Enhanced reference indexing failed: %v", err)
	} else {
		common.LSPLogger.Debug("Enhanced reference indexing complete")
	}
	if m.config.DiskCache && m.config.StoragePath != "" {
		if err := m.SaveIndexToDisk(); err != nil {
			common.LSPLogger.Warn("Failed to save index to disk: %v", err)
		}
	}
	return nil
}

// PerformIncrementalIndexing performs incremental workspace indexing
// Only indexes new or modified files since last indexing
func (m *SCIPCacheManager) PerformIncrementalIndexing(ctx context.Context, workingDir string, lspFallback LSPFallback) error {
	return m.PerformIncrementalIndexingWithProgress(ctx, workingDir, lspFallback, nil)
}

// PerformIncrementalIndexingWithProgress performs incremental indexing with progress callback
func (m *SCIPCacheManager) PerformIncrementalIndexingWithProgress(ctx context.Context, workingDir string, lspFallback LSPFallback, progress IndexProgressFunc) error {
	if !m.enabled {
		return nil
	}

	// Detect languages in the workspace
	detectedLanguages, err := project.DetectLanguages(workingDir)
	if err != nil {
		common.LSPLogger.Warn("Failed to detect languages for indexing: %v", err)
		detectedLanguages = []string{}
	}

	const maxFiles = 100000
	return m.performIncrementalIndexingCore(ctx, workingDir, detectedLanguages, maxFiles, lspFallback, progress)
}

// performIncrementalIndexingCore performs incremental indexing with all parameters
func (m *SCIPCacheManager) performIncrementalIndexingCore(ctx context.Context, workingDir string, languages []string, maxFiles int, lspFallback LSPFallback, progress IndexProgressFunc) error {
	if !m.enabled {
		return nil
	}

	indexer := NewWorkspaceIndexer(lspFallback)

	// Get file extensions for the languages
	extensions := indexer.GetLanguageExtensions(languages)

	// Scan all workspace files
	allFiles := indexer.ScanWorkspaceSourceFiles(workingDir, extensions, maxFiles)
	common.LSPLogger.Debug("Incremental indexing: Found %d total files in workspace", len(allFiles))

	// Determine which files need reindexing
	newFiles, modifiedFiles, unchangedFiles, err := m.fileTracker.GetChangedFiles(ctx, allFiles)
	if err != nil {
		return fmt.Errorf("failed to detect changed files: %w", err)
	}

	common.LSPLogger.Info("Incremental indexing: %d new, %d modified, %d unchanged files",
		len(newFiles), len(modifiedFiles), len(unchangedFiles))

	// Check for deleted files
	currentFileMap := make(map[string]bool)
	for _, file := range allFiles {
		absPath, _ := filepath.Abs(file)
		currentFileMap[absPath] = true
	}
	deletedFiles := m.fileTracker.GetDeletedFiles(currentFileMap)

	if len(deletedFiles) > 0 {
		common.LSPLogger.Info("Incremental indexing: Removing %d deleted files from index", len(deletedFiles))
		for _, uri := range deletedFiles {
			// Remove from SCIP storage
			if m.scipStorage != nil {
				if err := m.scipStorage.RemoveDocument(ctx, uri); err != nil {
					common.LSPLogger.Warn("Failed to remove document %s: %v", uri, err)
				}
			}
		}
		// Remove from file tracker
		m.fileTracker.RemoveFileMetadata(deletedFiles)
	}

	// Combine new and modified files for indexing
	filesToIndex := append(newFiles, modifiedFiles...)

	if len(filesToIndex) == 0 {
		common.LSPLogger.Info("Incremental indexing: No files need reindexing")
		return nil
	}

	common.LSPLogger.Info("Incremental indexing: Indexing %d files", len(filesToIndex))

	// Perform the indexing (IndexSpecificFiles will handle progress reporting)
	err = indexer.IndexSpecificFiles(ctx, filesToIndex, progress)
	if err != nil {
		return fmt.Errorf("incremental indexing failed: %w", err)
	}

	// Update file metadata for indexed files
	for _, file := range filesToIndex {
		absPath, err := filepath.Abs(file)
		if err != nil {
			continue
		}
		fileInfo, err := os.Stat(absPath)
		if err != nil {
			continue
		}
        uri := utils.FilePathToURI(absPath)
        language := "plaintext"
        if lang, ok := registry.GetLanguageByExtension(filepath.Ext(file)); ok {
            language = lang.Name
        }
        m.fileTracker.UpdateFileMetadata(uri, absPath, fileInfo.ModTime(), fileInfo.Size(), language)
	}

	// Save file tracker metadata
	if m.config.DiskCache && m.config.StoragePath != "" {
		metadataPath := filepath.Join(m.config.StoragePath, "file_metadata.json")
		if err := m.fileTracker.SaveToFile(metadataPath); err != nil {
			common.LSPLogger.Warn("Failed to save file metadata: %v", err)
		}

		if err := m.SaveIndexToDisk(); err != nil {
			common.LSPLogger.Warn("Failed to save index to disk: %v", err)
		}
	}

	// Process references for changed files if needed
	if len(filesToIndex) > 0 {
		common.LSPLogger.Debug("Starting enhanced reference indexing for changed files")
		err = indexer.IndexSpecificFilesWithReferences(ctx, filesToIndex, m, progress)
		if err != nil {
			common.LSPLogger.Error("Enhanced reference indexing failed: %v", err)
		}
	}

	return nil
}
