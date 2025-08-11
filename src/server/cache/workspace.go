package cache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/constants"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/utils"
)

// WorkspaceIndexer provides common workspace file scanning and indexing functionality
// This component consolidates workspace indexing operations to be shared between
// MCP server and HTTP gateway implementations
type WorkspaceIndexer struct {
	lspFallback LSPFallback
}

// NewWorkspaceIndexer creates a new workspace indexer with LSP fallback capability
func NewWorkspaceIndexer(lspFallback LSPFallback) *WorkspaceIndexer {
	return &WorkspaceIndexer{
		lspFallback: lspFallback,
	}
}

// IndexWorkspaceFiles scans workspace files and indexes document symbols with full ranges
// This method provides comprehensive symbol indexing by analyzing individual source files
// and extracting their complete symbol hierarchies for enhanced cache population
func (w *WorkspaceIndexer) IndexWorkspaceFiles(ctx context.Context, workspaceDir string, languages []string, maxFiles int) error {
	return w.indexWorkspaceFilesCore(ctx, workspaceDir, languages, maxFiles, nil)
}

func (w *WorkspaceIndexer) IndexWorkspaceFilesWithProgress(ctx context.Context, workspaceDir string, languages []string, maxFiles int, progress IndexProgressFunc) error {
	return w.indexWorkspaceFilesCore(ctx, workspaceDir, languages, maxFiles, progress)
}

func (w *WorkspaceIndexer) indexWorkspaceFilesCore(ctx context.Context, workspaceDir string, languages []string, maxFiles int, progress IndexProgressFunc) error {
	common.LSPLogger.Info("IndexWorkspaceFiles called: workspaceDir=%s, languages=%v, maxFiles=%d", workspaceDir, languages, maxFiles)
	if w.lspFallback == nil {
		common.LSPLogger.Error("IndexWorkspaceFiles: lspFallback is nil!")
		return fmt.Errorf("lspFallback is nil")
	}
	if workspaceDir == "" {
		var err error
		workspaceDir, err = os.Getwd()
		if err != nil {
			workspaceDir = "."
		}
	}
	extensions := w.GetLanguageExtensions(languages)
	common.LSPLogger.Info("Workspace indexer: Detected languages: %v", languages)
	common.LSPLogger.Info("Workspace indexer: Looking for extensions: %v", extensions)
	files := w.ScanWorkspaceSourceFiles(workspaceDir, extensions, maxFiles)
	common.LSPLogger.Info("Workspace indexer: Found %d source files in %s", len(files), workspaceDir)
	// Log first few files found
	for i, file := range files {
		if i < 5 {
			common.LSPLogger.Info("  File %d: %s", i, file)
		}
	}
	if len(files) == 0 {
		common.LSPLogger.Warn("Workspace indexer: No source files found for indexing")
		return nil
	}
	if progress != nil {
		progress("index_start", 0, len(files), "")
	}
	indexedCount := 0
	failedFiles := []string{}
	common.LSPLogger.Debug("Workspace indexer: Starting to process %d files", len(files))

	// Determine worker count based on environment and project type
	workers := computeWorkers(hasJavaInLangs(languages))
	var mu sync.Mutex
	jobs := make(chan int, workers)
	total := len(files)

	var wg sync.WaitGroup
	for wkr := 0; wkr < workers; wkr++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				file := files[idx]
				absPath, err := filepath.Abs(file)
				if err != nil {
					mu.Lock()
					failedFiles = append(failedFiles, file)
					if progress != nil {
						progress("index_file", idx+1, total, file)
					}
					mu.Unlock()
					continue
				}
				uri := utils.FilePathToURI(absPath)
				// Use shared context to avoid missing indexes due to per-file timeouts
				fctx, cancel := context.WithCancel(ctx)
				params := map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": uri,
					},
				}
				result, err := w.lspFallback.ProcessRequest(fctx, types.MethodTextDocumentDocumentSymbol, params)
				cancel()
				mu.Lock()
				if err != nil {
					failedFiles = append(failedFiles, file)
				} else if result != nil {
					indexedCount++
				}
				if progress != nil {
					progress("index_file", idx+1, total, file)
				}
				mu.Unlock()
			}
		}()
	}
	for i := range files {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	if len(failedFiles) > 0 {
		common.LSPLogger.Warn("Failed to index %d out of %d files", len(failedFiles), len(files))
		if len(failedFiles) <= 10 {
			for _, file := range failedFiles {
				common.LSPLogger.Debug("  Failed: %s", file)
			}
		} else {
			common.LSPLogger.Debug("  First few failed files: %v...", failedFiles[:5])
		}
	}
	common.LSPLogger.Debug("Workspace indexer: Completed processing. Indexed %d files, %d failed", indexedCount, len(failedFiles))
	if progress != nil {
		progress("index_complete", indexedCount, len(files), "")
	}
	return nil
}

// detectLanguageID detects the language ID from file extension
// (legacy detectLanguageID removed; use internal/registry or DocumentManager for detection)

// GetLanguageExtensions returns file extensions for the specified languages
// This method determines which file types should be included in workspace scanning
// based on the provided language list and constants mapping
func (w *WorkspaceIndexer) GetLanguageExtensions(languages []string) []string {
	if len(languages) == 0 {
		// Return all supported extensions if no languages specified
		return constants.GetAllSupportedExtensions()
	}

	extensions := []string{}

	// Get extensions for specified languages
	for _, lang := range languages {
		if exts, ok := constants.SupportedExtensions[lang]; ok {
			extensions = append(extensions, exts...)
		}
	}

	// Default to common extensions if none found
	if len(extensions) == 0 {
		extensions = []string{".go", ".py", ".js", ".ts", ".java"}
	}

	return extensions
}

// ScanWorkspaceSourceFiles scans directory for source files with given extensions
// This method performs a recursive directory traversal to locate source files
// for indexing, respecting common exclusion patterns and file count limits
func (w *WorkspaceIndexer) ScanWorkspaceSourceFiles(dir string, extensions []string, maxFiles int) []string {
	var files []string
	count := 0

	common.LSPLogger.Debug("ScanWorkspaceSourceFiles: Starting scan of %s, looking for extensions: %v, maxFiles: %d", dir, extensions, maxFiles)

	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			// Skip hidden directories and common non-source directories
			if d != nil && d.IsDir() {
				name := d.Name()
				if strings.HasPrefix(name, ".") || constants.SkipDirectories[name] {
					common.LSPLogger.Debug("ScanWorkspaceSourceFiles: Skipping directory: %s", path)
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Check if we've reached the limit
		if count >= maxFiles {
			return filepath.SkipDir
		}

		// Check file extension
		ext := filepath.Ext(path)
		for _, validExt := range extensions {
			if ext == validExt {
				files = append(files, path)
				count++
				break
			}
		}

		return nil
	})

	common.LSPLogger.Debug("ScanWorkspaceSourceFiles: Scan complete. Found %d files", len(files))
	return files
}

// IndexSpecificFiles indexes only the specified files
func (w *WorkspaceIndexer) IndexSpecificFiles(ctx context.Context, files []string, progress IndexProgressFunc) error {
	if w.lspFallback == nil {
		return fmt.Errorf("lspFallback is nil")
	}

	if len(files) == 0 {
		common.LSPLogger.Debug("IndexSpecificFiles: No files to index")
		return nil
	}

	common.LSPLogger.Debug("IndexSpecificFiles: Indexing %d specific files", len(files))

	if progress != nil {
		progress("index_start", 0, len(files), "")
	}

	indexedCount := 0
	failedFiles := []string{}

	// Determine worker count based on environment and file types
	workers := computeWorkers(hasJavaInFiles(files))

	var mu sync.Mutex
	jobs := make(chan int, workers)
	total := len(files)

	var wg sync.WaitGroup
	for wkr := 0; wkr < workers; wkr++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				file := files[idx]
				absPath, err := filepath.Abs(file)
				if err != nil {
					mu.Lock()
					failedFiles = append(failedFiles, file)
					if progress != nil {
						progress("index_file", idx+1, total, file)
					}
					mu.Unlock()
					continue
				}

				uri := utils.FilePathToURI(absPath)
				fctx, cancel := context.WithCancel(ctx)
				params := map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": uri,
					},
				}

				result, err := w.lspFallback.ProcessRequest(fctx, types.MethodTextDocumentDocumentSymbol, params)
				cancel()

				mu.Lock()
				if err != nil {
					failedFiles = append(failedFiles, file)
				} else if result != nil {
					indexedCount++
				}
				if progress != nil {
					progress("index_file", idx+1, total, file)
				}
				mu.Unlock()
			}
		}()
	}

	for i := range files {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	if len(failedFiles) > 0 {
		common.LSPLogger.Warn("Failed to index %d out of %d files", len(failedFiles), len(files))
	}

	common.LSPLogger.Debug("IndexSpecificFiles: Completed. Indexed %d files, %d failed", indexedCount, len(failedFiles))

	if progress != nil {
		progress("index_complete", indexedCount, len(files), "")
	}

	return nil
}
