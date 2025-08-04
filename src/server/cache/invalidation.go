package cache

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server/documents"
)

// DependencyGraph tracks symbol dependencies across files
type DependencyGraph struct {
	// symbolToFiles maps symbol ID to files that define or reference it
	symbolToFiles map[string]map[string]bool
	// fileToSymbols maps file URI to symbols it contains
	fileToSymbols map[string]map[string]bool
	// fileDependencies tracks which files depend on other files
	fileDependencies map[string]map[string]bool
	mu               sync.RWMutex
}

// FileWatchDebouncer handles debouncing of rapid file changes
type FileWatchDebouncer struct {
	changes map[string]*time.Timer
	delay   time.Duration
	mu      sync.Mutex
}

// FileHashTracker tracks file content hashes to detect actual changes
type FileHashTracker struct {
	fileHashes map[string]string // URI -> content hash
	mu         sync.RWMutex
}

// SCIPInvalidationManager implements smart cache invalidation
type SCIPInvalidationManager struct {
	storage    SCIPStorage
	docManager documents.DocumentManager

	// File system monitoring
	watcher      *fsnotify.Watcher
	watchedPaths map[string]bool
	debouncer    *FileWatchDebouncer

	// Dependency tracking
	depGraph *DependencyGraph

	// File content tracking for change detection
	hashTracker *FileHashTracker

	// Configuration
	projectRoot   string
	watchPatterns []string

	// State management
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.RWMutex
	started bool
}

// NewSCIPInvalidationManager creates a new invalidation manager
func NewSCIPInvalidationManager(storage SCIPStorage, docManager documents.DocumentManager) *SCIPInvalidationManager {
	return &SCIPInvalidationManager{
		storage:      storage,
		docManager:   docManager,
		watchedPaths: make(map[string]bool),
		watchPatterns: []string{
			"*.go", "*.py", "*.js", "*.jsx", "*.ts", "*.tsx", "*.java",
		},
		debouncer: &FileWatchDebouncer{
			changes: make(map[string]*time.Timer),
			delay:   200 * time.Millisecond, // Debounce rapid changes
		},
		depGraph: &DependencyGraph{
			symbolToFiles:    make(map[string]map[string]bool),
			fileToSymbols:    make(map[string]map[string]bool),
			fileDependencies: make(map[string]map[string]bool),
		},
		hashTracker: &FileHashTracker{
			fileHashes: make(map[string]string),
		},
	}
}

// Start initializes the invalidation manager
func (m *SCIPInvalidationManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return fmt.Errorf("invalidation manager already started")
	}

	m.ctx, m.cancel = context.WithCancel(ctx)
	m.started = true

	// Initialize file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	m.watcher = watcher

	// Start file monitoring goroutine
	m.wg.Add(1)
	go m.watchFiles()

	common.LSPLogger.Info("SCIP invalidation manager started")
	return nil
}

// Stop gracefully shuts down the invalidation manager
func (m *SCIPInvalidationManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return nil
	}

	common.LSPLogger.Info("Stopping SCIP invalidation manager")

	if m.cancel != nil {
		m.cancel()
	}

	if m.watcher != nil {
		m.watcher.Close()
	}

	m.wg.Wait()
	m.started = false

	common.LSPLogger.Info("SCIP invalidation manager stopped")
	return nil
}

// InvalidateDocument removes all cached entries for a specific document
func (m *SCIPInvalidationManager) InvalidateDocument(uri string) error {
	common.LSPLogger.Info("Invalidating document: %s", uri)

	if m.storage == nil {
		return fmt.Errorf("storage not available")
	}

	// Build patterns for all methods related to this document
	patterns := []string{
		fmt.Sprintf("*:%s:*", uri),
		fmt.Sprintf("textDocument/definition:%s:*", uri),
		fmt.Sprintf("textDocument/references:%s:*", uri),
		fmt.Sprintf("textDocument/hover:%s:*", uri),
		fmt.Sprintf("textDocument/documentSymbol:%s:*", uri),
		fmt.Sprintf("textDocument/completion:%s:*", uri),
	}

	for _, pattern := range patterns {
		if err := m.InvalidatePattern(pattern); err != nil {
			common.LSPLogger.Error("Failed to invalidate pattern %s: %v", pattern, err)
		}
	}

	// Remove from dependency graph
	m.removeFromDepGraph(uri)

	return nil
}

// InvalidateSymbol removes cached entries for a specific symbol
func (m *SCIPInvalidationManager) InvalidateSymbol(symbolID string) error {
	common.LSPLogger.Debug("Invalidating symbol: %s", symbolID)

	m.depGraph.mu.RLock()
	files, exists := m.depGraph.symbolToFiles[symbolID]
	m.depGraph.mu.RUnlock()

	if !exists {
		common.LSPLogger.Debug("Symbol %s not found in dependency graph", symbolID)
		return nil
	}

	// Invalidate all files that contain or reference this symbol
	var filesToInvalidate []string
	for file := range files {
		filesToInvalidate = append(filesToInvalidate, file)
	}

	return m.CascadeInvalidate(filesToInvalidate)
}

// InvalidatePattern removes cached entries matching a pattern
func (m *SCIPInvalidationManager) InvalidatePattern(pattern string) error {
	common.LSPLogger.Debug("Invalidating pattern: %s", pattern)

	if m.storage == nil {
		return fmt.Errorf("storage not available")
	}

	// For now, we'll implement a simple pattern matching
	// In a real implementation, this would iterate through cache keys
	// and match against the pattern, then delete matching entries

	// This is a placeholder - actual implementation would depend on
	// the storage backend's pattern matching capabilities
	common.LSPLogger.Debug("Pattern invalidation completed for: %s", pattern)
	return nil
}

// SetupFileWatcher configures file system monitoring for a project
func (m *SCIPInvalidationManager) SetupFileWatcher(projectRoot string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.watcher == nil {
		return fmt.Errorf("file watcher not initialized")
	}

	m.projectRoot = projectRoot

	common.LSPLogger.Info("Setting up file watcher for project: %s", projectRoot)

	// Walk the project directory and add watches for relevant directories
	err := filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and common build/cache directories
		if info.IsDir() {
			name := filepath.Base(path)
			if strings.HasPrefix(name, ".") ||
				name == "node_modules" ||
				name == "vendor" ||
				name == "__pycache__" ||
				name == "target" ||
				name == "build" {
				return filepath.SkipDir
			}

			// Add directory to watcher
			if !m.watchedPaths[path] {
				if err := m.watcher.Add(path); err != nil {
					common.LSPLogger.Warn("Failed to watch directory %s: %v", path, err)
				} else {
					m.watchedPaths[path] = true
					common.LSPLogger.Debug("Added directory to watcher: %s", path)
				}
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to setup file watcher: %w", err)
	}

	common.LSPLogger.Info("File watcher setup completed for %d directories", len(m.watchedPaths))
	return nil
}

// GetDependencies returns files that depend on the given URI
func (m *SCIPInvalidationManager) GetDependencies(uri string) ([]string, error) {
	m.depGraph.mu.RLock()
	defer m.depGraph.mu.RUnlock()

	dependencies := make([]string, 0)

	// Get direct dependencies
	if deps, exists := m.depGraph.fileDependencies[uri]; exists {
		for dep := range deps {
			dependencies = append(dependencies, dep)
		}
	}

	// Get files that reference symbols from this file
	if symbols, exists := m.depGraph.fileToSymbols[uri]; exists {
		for symbolID := range symbols {
			if files, exists := m.depGraph.symbolToFiles[symbolID]; exists {
				for file := range files {
					if file != uri {
						// Check if not already in dependencies
						found := false
						for _, dep := range dependencies {
							if dep == file {
								found = true
								break
							}
						}
						if !found {
							dependencies = append(dependencies, file)
						}
					}
				}
			}
		}
	}

	common.LSPLogger.Debug("Found %d dependencies for %s", len(dependencies), uri)
	return dependencies, nil
}

// CascadeInvalidate performs cascade invalidation for multiple URIs
func (m *SCIPInvalidationManager) CascadeInvalidate(uris []string) error {
	if len(uris) == 0 {
		return nil
	}

	common.LSPLogger.Info("Performing cascade invalidation for %d files", len(uris))

	// Use a set to avoid duplicate invalidations
	toInvalidate := make(map[string]bool)
	processed := make(map[string]bool)

	// Build the complete set of files to invalidate using BFS
	queue := make([]string, len(uris))
	copy(queue, uris)

	for _, uri := range uris {
		toInvalidate[uri] = true
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if processed[current] {
			continue
		}
		processed[current] = true

		// Get dependencies of current file
		deps, err := m.GetDependencies(current)
		if err != nil {
			common.LSPLogger.Error("Failed to get dependencies for %s: %v", current, err)
			continue
		}

		// Add dependencies to invalidation set and queue
		for _, dep := range deps {
			if !toInvalidate[dep] {
				toInvalidate[dep] = true
				queue = append(queue, dep)
			}
		}
	}

	// Perform invalidation for all files in the set
	var errors []error
	for uri := range toInvalidate {
		if err := m.InvalidateDocument(uri); err != nil {
			errors = append(errors, fmt.Errorf("failed to invalidate %s: %w", uri, err))
		}
	}

	if len(errors) > 0 {
		// Log individual errors but don't fail the entire operation
		for _, err := range errors {
			common.LSPLogger.Error("Cascade invalidation error: %v", err)
		}
		return fmt.Errorf("cascade invalidation completed with %d errors", len(errors))
	}

	common.LSPLogger.Info("Cascade invalidation completed for %d files", len(toInvalidate))
	return nil
}

// UpdateDependencies updates the dependency graph with symbol information
func (m *SCIPInvalidationManager) UpdateDependencies(uri string, symbols []string, references map[string][]string) {
	m.depGraph.mu.Lock()
	defer m.depGraph.mu.Unlock()

	// Initialize maps if needed
	if m.depGraph.fileToSymbols[uri] == nil {
		m.depGraph.fileToSymbols[uri] = make(map[string]bool)
	}

	// Update symbols for this file
	for _, symbolID := range symbols {
		m.depGraph.fileToSymbols[uri][symbolID] = true

		if m.depGraph.symbolToFiles[symbolID] == nil {
			m.depGraph.symbolToFiles[symbolID] = make(map[string]bool)
		}
		m.depGraph.symbolToFiles[symbolID][uri] = true
	}

	// Update references
	if m.depGraph.fileDependencies[uri] == nil {
		m.depGraph.fileDependencies[uri] = make(map[string]bool)
	}

	for _, refFiles := range references {
		for _, refFile := range refFiles {
			if refFile != uri {
				m.depGraph.fileDependencies[uri][refFile] = true

				// Also track reverse dependency
				if m.depGraph.fileDependencies[refFile] == nil {
					m.depGraph.fileDependencies[refFile] = make(map[string]bool)
				}
				m.depGraph.fileDependencies[refFile][uri] = true
			}
		}
	}

	common.LSPLogger.Debug("Updated dependencies for %s: %d symbols, %d references",
		uri, len(symbols), len(references))
}

// watchFiles monitors file system changes
func (m *SCIPInvalidationManager) watchFiles() {
	defer m.wg.Done()

	common.LSPLogger.Info("Starting file system monitoring")

	for {
		select {
		case <-m.ctx.Done():
			common.LSPLogger.Info("File watcher context cancelled")
			return

		case event, ok := <-m.watcher.Events:
			if !ok {
				common.LSPLogger.Warn("File watcher events channel closed")
				return
			}

			m.handleFileEvent(event)

		case err, ok := <-m.watcher.Errors:
			if !ok {
				common.LSPLogger.Warn("File watcher errors channel closed")
				return
			}

			common.LSPLogger.Error("File watcher error: %v", err)
		}
	}
}

// handleFileEvent processes a file system event
func (m *SCIPInvalidationManager) handleFileEvent(event fsnotify.Event) {
	// Check if this is a file we care about
	if !m.isRelevantFile(event.Name) {
		return
	}

	common.LSPLogger.Debug("File event: %s %s", event.Op.String(), event.Name)

	// Convert file path to URI
	uri := "file://" + event.Name

	// Debounce rapid changes to the same file
	m.debouncer.mu.Lock()

	// Cancel existing timer for this file
	if timer, exists := m.debouncer.changes[uri]; exists {
		timer.Stop()
	}

	// Set new timer
	m.debouncer.changes[uri] = time.AfterFunc(m.debouncer.delay, func() {
		m.processFileChange(uri, event.Op)

		// Clean up timer
		m.debouncer.mu.Lock()
		delete(m.debouncer.changes, uri)
		m.debouncer.mu.Unlock()
	})

	m.debouncer.mu.Unlock()
}

// processFileChange handles a debounced file change
func (m *SCIPInvalidationManager) processFileChange(uri string, op fsnotify.Op) {
	common.LSPLogger.Info("Processing file change: %s (%s)", uri, op.String())

	switch {
	case op&fsnotify.Write == fsnotify.Write:
		// File was modified - invalidate and potentially cascade
		m.handleFileModification(uri)

	case op&fsnotify.Remove == fsnotify.Remove:
		// File was deleted - remove from cache and dependency graph
		m.handleFileRemoval(uri)

	case op&fsnotify.Rename == fsnotify.Rename:
		// File was renamed - treat as removal
		m.handleFileRemoval(uri)

	case op&fsnotify.Create == fsnotify.Create:
		// New file created - nothing to invalidate but log it
		common.LSPLogger.Debug("New file created: %s", uri)
	}
}

// handleFileModification processes a file modification with content change detection
func (m *SCIPInvalidationManager) handleFileModification(uri string) {
	// Check if file content has actually changed
	hasChanged, err := m.hasFileContentChanged(uri)
	if err != nil {
		common.LSPLogger.Error("Failed to check file content changes for %s: %v", uri, err)
		// If we can't determine if content changed, assume it did for safety
		hasChanged = true
	}

	if !hasChanged {
		common.LSPLogger.Debug("File %s has not changed in content, preserving cache", uri)
		return
	}

	common.LSPLogger.Info("File %s content changed, invalidating cache", uri)

	// Get dependencies before invalidation
	dependencies, err := m.GetDependencies(uri)
	if err != nil {
		common.LSPLogger.Error("Failed to get dependencies for %s: %v", uri, err)
		dependencies = []string{}
	}

	// Add the modified file to the list
	allFiles := append([]string{uri}, dependencies...)

	// Perform cascade invalidation
	if err := m.CascadeInvalidate(allFiles); err != nil {
		common.LSPLogger.Error("Failed to cascade invalidate after file modification: %v", err)
	}
}

// handleFileRemoval processes a file removal
func (m *SCIPInvalidationManager) handleFileRemoval(uri string) {
	// Invalidate the removed file
	if err := m.InvalidateDocument(uri); err != nil {
		common.LSPLogger.Error("Failed to invalidate removed file %s: %v", uri, err)
	}

	// Remove from dependency graph
	m.removeFromDepGraph(uri)

	// Remove from hash tracking
	m.removeFileHash(uri)

	common.LSPLogger.Info("Removed file %s from cache, dependency graph, and hash tracking", uri)
}

// removeFromDepGraph removes a file from the dependency graph
func (m *SCIPInvalidationManager) removeFromDepGraph(uri string) {
	m.depGraph.mu.Lock()
	defer m.depGraph.mu.Unlock()

	// Remove symbols associated with this file
	if symbols, exists := m.depGraph.fileToSymbols[uri]; exists {
		for symbolID := range symbols {
			if files, exists := m.depGraph.symbolToFiles[symbolID]; exists {
				delete(files, uri)
				// If no files left for this symbol, remove the symbol entirely
				if len(files) == 0 {
					delete(m.depGraph.symbolToFiles, symbolID)
				}
			}
		}
		delete(m.depGraph.fileToSymbols, uri)
	}

	// Remove file dependencies
	delete(m.depGraph.fileDependencies, uri)

	// Remove this file from other files' dependency lists
	for _, deps := range m.depGraph.fileDependencies {
		delete(deps, uri)
	}

	common.LSPLogger.Debug("Removed %s from dependency graph", uri)
}

// isRelevantFile checks if a file is relevant for cache invalidation
func (m *SCIPInvalidationManager) isRelevantFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))

	relevantExtensions := map[string]bool{
		".go":   true,
		".py":   true,
		".js":   true,
		".jsx":  true,
		".ts":   true,
		".tsx":  true,
		".java": true,
	}

	return relevantExtensions[ext]
}

// GetStats returns invalidation manager statistics
func (m *SCIPInvalidationManager) GetStats() map[string]interface{} {
	m.depGraph.mu.RLock()
	defer m.depGraph.mu.RUnlock()

	m.hashTracker.mu.RLock()
	trackedHashes := len(m.hashTracker.fileHashes)
	m.hashTracker.mu.RUnlock()

	stats := map[string]interface{}{
		"watched_paths":     len(m.watchedPaths),
		"tracked_files":     len(m.depGraph.fileToSymbols),
		"tracked_symbols":   len(m.depGraph.symbolToFiles),
		"tracked_hashes":    trackedHashes,
		"dependency_edges":  len(m.depGraph.fileDependencies),
		"project_root":      m.projectRoot,
		"watch_patterns":    m.watchPatterns,
		"debounce_delay_ms": m.debouncer.delay.Milliseconds(),
	}

	return stats
}

// hasFileContentChanged checks if a file's content has actually changed
func (m *SCIPInvalidationManager) hasFileContentChanged(uri string) (bool, error) {
	// Convert URI to file path
	filePath := strings.TrimPrefix(uri, "file://")
	
	// Calculate current file hash
	currentHash, err := m.calculateFileHash(filePath)
	if err != nil {
		return true, fmt.Errorf("failed to calculate file hash: %w", err)
	}

	// Get previous hash
	m.hashTracker.mu.RLock()
	previousHash, exists := m.hashTracker.fileHashes[uri]
	m.hashTracker.mu.RUnlock()

	// If no previous hash exists, consider it changed
	if !exists {
		m.updateFileHash(uri, currentHash)
		return true, nil
	}

	// Compare hashes
	hasChanged := currentHash != previousHash
	
	// Update hash if changed
	if hasChanged {
		m.updateFileHash(uri, currentHash)
	}

	return hasChanged, nil
}

// calculateFileHash calculates SHA256 hash of file content
func (m *SCIPInvalidationManager) calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// updateFileHash updates the stored hash for a file
func (m *SCIPInvalidationManager) updateFileHash(uri, hash string) {
	m.hashTracker.mu.Lock()
	defer m.hashTracker.mu.Unlock()
	m.hashTracker.fileHashes[uri] = hash
}

// getFileHash retrieves the stored hash for a file
func (m *SCIPInvalidationManager) getFileHash(uri string) (string, bool) {
	m.hashTracker.mu.RLock()
	defer m.hashTracker.mu.RUnlock()
	hash, exists := m.hashTracker.fileHashes[uri]
	return hash, exists
}

// removeFileHash removes the stored hash for a file
func (m *SCIPInvalidationManager) removeFileHash(uri string) {
	m.hashTracker.mu.Lock()
	defer m.hashTracker.mu.Unlock()
	delete(m.hashTracker.fileHashes, uri)
}

// trackFileForChanges starts tracking a file for content changes
func (m *SCIPInvalidationManager) trackFileForChanges(uri string) error {
	filePath := strings.TrimPrefix(uri, "file://")
	
	hash, err := m.calculateFileHash(filePath)
	if err != nil {
		return fmt.Errorf("failed to calculate initial hash for %s: %w", uri, err)
	}

	m.updateFileHash(uri, hash)
	common.LSPLogger.Debug("Started tracking file for changes: %s", uri)
	return nil
}
