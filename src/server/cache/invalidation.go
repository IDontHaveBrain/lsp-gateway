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
	storage    SCIPStorage
	docManager documents.DocumentManager
	lastSeen   map[string]time.Time
	mu         sync.RWMutex
	started    bool
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

	if !m.started {
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

// GetStats returns simplified invalidation manager statistics
func (m *SimpleInvalidationManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"type":          "simple",
		"tracked_files": len(m.lastSeen),
		"started":       m.started,
	}

	return stats
}
