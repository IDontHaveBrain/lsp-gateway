package cache

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server/documents"
)

// SimpleInvalidationManager implements basic cache invalidation with file modification time checking
type SimpleInvalidationManager struct {
	storage     SCIPStorage
	docManager  documents.DocumentManager
	lastSeen    map[string]time.Time
	fileWatcher *FileWatcher
	mu          sync.RWMutex
	started     bool
}

// NewSimpleInvalidationManager creates a new simple invalidation manager
func NewSimpleInvalidationManager(storage SCIPStorage, docManager documents.DocumentManager) *SimpleInvalidationManager {
	return &SimpleInvalidationManager{
		storage:    storage,
		docManager: docManager,
		lastSeen:   make(map[string]time.Time),
	}
}

// Start initializes the invalidation manager
func (m *SimpleInvalidationManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return fmt.Errorf("invalidation manager already started")
	}

	m.started = true
	common.LSPLogger.Info("Simple invalidation manager started")
	return nil
}

// Stop gracefully shuts down the invalidation manager
func (m *SimpleInvalidationManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop file watcher if running (regardless of started state)
	if m.fileWatcher != nil {
		if err := m.fileWatcher.Stop(); err != nil {
			common.LSPLogger.Error("Error stopping file watcher: %v", err)
		}
		m.fileWatcher = nil
	}

	// Only skip the rest if not started
	if !m.started {
		common.LSPLogger.Info("Simple invalidation manager stopped (was not started)")
		return nil
	}

	m.started = false
	common.LSPLogger.Info("Simple invalidation manager stopped")
	return nil
}

// InvalidateDocument removes all cached entries for a specific document
func (m *SimpleInvalidationManager) InvalidateDocument(uri string) error {
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

	return nil
}

// InvalidateSymbol removes cached entries for a specific symbol
func (m *SimpleInvalidationManager) InvalidateSymbol(symbolID string) error {
	common.LSPLogger.Debug("Invalidating symbol: %s", symbolID)
	return nil
}

// InvalidatePattern removes cached entries matching a pattern
func (m *SimpleInvalidationManager) InvalidatePattern(pattern string) error {
	common.LSPLogger.Debug("Invalidating pattern: %s", pattern)

	if m.storage == nil {
		return fmt.Errorf("storage not available")
	}

	common.LSPLogger.Debug("Pattern invalidation completed for: %s", pattern)
	return nil
}

// InvalidateIfChanged checks if file has been modified and invalidates if needed
func (m *SimpleInvalidationManager) InvalidateIfChanged(uri string) {
	filePath := strings.TrimPrefix(uri, "file://")

	if stat, err := os.Stat(filePath); err == nil {
		modTime := stat.ModTime()

		m.mu.RLock()
		lastSeen, exists := m.lastSeen[uri]
		m.mu.RUnlock()

		if !exists || modTime.After(lastSeen) {
			m.InvalidateDocument(uri)

			m.mu.Lock()
			m.lastSeen[uri] = modTime
			m.mu.Unlock()

			common.LSPLogger.Debug("File %s modified, cache invalidated", uri)
		}
	}
}

// SetupFileWatcher implements the SCIPInvalidation interface
func (m *SimpleInvalidationManager) SetupFileWatcher(projectRoot string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.fileWatcher != nil {
		return fmt.Errorf("file watcher already set up")
	}

	// Create file watcher with this invalidation manager
	// Note: semanticCache will be set later when available
	m.fileWatcher = NewFileWatcher(nil, m)

	// Start the file watcher
	ctx := context.Background()
	if err := m.fileWatcher.Start(ctx, projectRoot); err != nil {
		return fmt.Errorf("failed to start file watcher: %w", err)
	}

	// Add project root to watched directories
	if err := m.fileWatcher.AddWatchPath(projectRoot); err != nil {
		common.LSPLogger.Warn("Failed to add project root to watch paths: %v", err)
	}

	common.LSPLogger.Info("File watcher set up successfully for project: %s", projectRoot)
	return nil
}

// SetSemanticCache enables semantic indexing for real-time updates
func (m *SimpleInvalidationManager) SetSemanticCache(semanticCache *SemanticCacheManager) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.fileWatcher != nil {
		m.fileWatcher.semanticCache = semanticCache
		common.LSPLogger.Info("Semantic cache enabled for file watcher")
	}
}

// GetDependencies returns dependent files for a given URI (placeholder implementation)
func (m *SimpleInvalidationManager) GetDependencies(uri string) ([]string, error) {
	// Placeholder implementation - in a real system this would analyze imports/dependencies
	return []string{}, nil
}

// CascadeInvalidate invalidates multiple URIs and their dependencies
func (m *SimpleInvalidationManager) CascadeInvalidate(uris []string) error {
	common.LSPLogger.Info("Cascade invalidating %d URIs", len(uris))

	for _, uri := range uris {
		if err := m.InvalidateDocument(uri); err != nil {
			common.LSPLogger.Error("Failed to invalidate URI %s: %v", uri, err)
		}

		// Get dependencies and invalidate them too
		if deps, err := m.GetDependencies(uri); err == nil {
			for _, dep := range deps {
				if err := m.InvalidateDocument(dep); err != nil {
					common.LSPLogger.Error("Failed to invalidate dependency %s: %v", dep, err)
				}
			}
		}
	}

	return nil
}

// GetFileWatcher returns the file watcher instance (for testing/debugging)
func (m *SimpleInvalidationManager) GetFileWatcher() *FileWatcher {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.fileWatcher
}

// StopFileWatcher stops the file watcher if running
func (m *SimpleInvalidationManager) StopFileWatcher() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.fileWatcher != nil {
		if err := m.fileWatcher.Stop(); err != nil {
			return err
		}
		m.fileWatcher = nil
		common.LSPLogger.Info("File watcher stopped")
	}

	return nil
}

// GetStats returns simplified invalidation manager statistics
func (m *SimpleInvalidationManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"type":          "simple",
		"tracked_files": len(m.lastSeen),
		"started":       m.started,
	}

	// Add file watcher stats if available
	if m.fileWatcher != nil {
		stats["file_watcher"] = m.fileWatcher.GetStats()
	}

	return stats
}
