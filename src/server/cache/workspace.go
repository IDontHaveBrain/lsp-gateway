package cache

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/constants"
	"lsp-gateway/src/internal/types"
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

	// Use current directory if not provided
	if workspaceDir == "" {
		var err error
		workspaceDir, err = os.Getwd()
		if err != nil {
			workspaceDir = "."
		}
	}

	// Get file extensions for the configured languages
	extensions := w.GetLanguageExtensions(languages)

	// Scan for source files
	files := w.ScanWorkspaceSourceFiles(workspaceDir, extensions, maxFiles)

	if len(files) == 0 {
		common.LSPLogger.Warn("Workspace indexer: No source files found for indexing")
		return nil
	}

	// Index each file's document symbols
	indexedCount := 0
	for _, file := range files {
		// Convert to file URI
		absPath, err := filepath.Abs(file)
		if err != nil {
			continue
		}
		uri := "file://" + absPath

		// Get document symbols for this file
		params := map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": uri,
			},
		}

		result, err := w.lspFallback.ProcessRequest(ctx, types.MethodTextDocumentDocumentSymbol, params)
		if err != nil {
			common.LSPLogger.Debug("Failed to get document symbols for %s: %v", file, err)
			continue
		}

		// The ProcessRequest will automatically trigger indexing via indexDocumentSymbols
		// But let's log the result to see what we got
		if result != nil {
			indexedCount++
		}
	}

	return nil
}

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

	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			// Skip hidden directories and common non-source directories
			if d != nil && d.IsDir() {
				name := d.Name()
				if strings.HasPrefix(name, ".") || constants.SkipDirectories[name] {
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

	return files
}
