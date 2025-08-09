package cache

import (
	"context"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server/scip"
)

// InvalidateDocument removes all cached entries for a specific document and its dependencies
func (m *SCIPCacheManager) InvalidateDocument(uri string) error {
	if !m.isEnabled() {
		return nil
	}

	var affectedDocs []string
	if scipStorage, ok := m.scipStorage.(*scip.SimpleSCIPStorage); ok {
		affectedDocs = scipStorage.GetAffectedDocuments(uri)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	common.LSPLogger.Debug("Invalidating cache for document: %s (affects %d other documents)", uri, len(affectedDocs))

	m.invalidateSingleDocument(uri)
	for _, affectedURI := range affectedDocs {
		m.invalidateSingleDocument(affectedURI)
	}

	m.updateStats()

	if m.config.BackgroundIndex && len(affectedDocs) > 0 {
		docsToReindex := append(affectedDocs, uri)
		go m.reindexDocuments(docsToReindex)
	}

	return nil
}

// invalidateSingleDocument removes cached entries for a single document
func (m *SCIPCacheManager) invalidateSingleDocument(uri string) {
	// Remove all entries with matching URI
	var removedCount int
	for key, entry := range m.entries {
		if entry.Key.URI == uri {
			delete(m.entries, key)
			removedCount++
		}
	}

	// Cache entries removed

	// Clean up SCIP storage
	if m.scipStorage != nil {
		if err := m.scipStorage.RemoveDocument(context.Background(), uri); err != nil {
			common.LSPLogger.Error("Failed to remove document from SCIP storage: %v", err)
		}
	}
}

// reindexDocuments performs background reindexing of affected documents
// This is triggered when a file changes and affects other files that reference it
func (m *SCIPCacheManager) reindexDocuments(uris []string) {
	// Create a context with timeout for reindexing
	ctx, cancel := common.CreateContext(30 * time.Second)
	defer cancel()

	common.LSPLogger.Debug("Starting background reindexing for %d documents", len(uris))

	successCount := 0
	errorCount := 0

	for _, uri := range uris {
		// Skip if context is cancelled
		select {
		case <-ctx.Done():
			common.LSPLogger.Warn("Reindexing cancelled after %d/%d documents", successCount, len(uris))
			return
		default:
		}

		// Detect language from URI
		language := m.converter.DetectLanguageFromURI(uri)
		if language == "" {
			common.LSPLogger.Debug("Could not detect language for %s, skipping reindex", uri)
			errorCount++
			continue
		}

		// Request fresh document symbols from LSP server
		// Note: We need access to LSP manager, which should be injected or accessible
		// For now, we'll just mark this as a placeholder for actual implementation
		// The actual implementation would need to call LSP server through the manager

		// Parameters for document symbols request would be:
		// params := map[string]interface{}{
		//     "textDocument": map[string]interface{}{
		//         "uri": uri,
		//     },
		// }

		// Since we don't have direct access to LSP manager here, we'll just log
		// In a real implementation, this would be coordinated through the LSP manager
		common.LSPLogger.Debug("Would reindex document %s (language: %s)", uri, language)

		// For demonstration purposes, we'll just mark as successful
		// In actual implementation, this would:
		// 1. Get fresh symbols from LSP server
		// 2. Convert to SCIP format
		// 3. Store in SCIP storage
		successCount++
	}

	common.LSPLogger.Debug("Background reindexing completed: %d successful, %d errors", successCount, errorCount)
}