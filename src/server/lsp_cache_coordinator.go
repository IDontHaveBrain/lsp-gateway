package server

import (
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/cache"
)

// isCacheableMethod determines if an LSP method should be cached
func (m *LSPManager) isCacheableMethod(method string) bool {
	// Cache the 6 supported LSP methods that benefit from caching
	cacheableMethods := map[string]bool{
		types.MethodTextDocumentDefinition:     true,
		types.MethodTextDocumentReferences:     true,
		types.MethodTextDocumentHover:          true,
		types.MethodTextDocumentDocumentSymbol: true,
		types.MethodWorkspaceSymbol:            true,
		types.MethodTextDocumentCompletion:     true,
	}

	return cacheableMethods[method]
}

// InvalidateCache invalidates cached entries for a document (optional cache)
func (m *LSPManager) InvalidateCache(uri string) error {
	if m.scipCache == nil {
		return nil // No cache to invalidate
	}
	return m.scipCache.InvalidateDocument(uri)
}

// GetCacheMetrics returns cache performance metrics including SCIP index stats (optional cache)
func (m *LSPManager) GetCacheMetrics() interface{} {
	if m.scipCache == nil {
		return nil
	}

	cacheMetrics := m.scipCache.GetMetrics()
	indexStats := m.scipCache.GetIndexStats()

	return map[string]interface{}{
		"cache":      cacheMetrics,
		"scip_index": indexStats,
		"integrated": true,
	}
}

// GetCache returns the SCIP cache instance for external access (optional cache)
func (m *LSPManager) GetCache() cache.SCIPCache {
	return m.scipCache // Can be nil
}

// SetCache allows optional cache injection for flexible integration
func (m *LSPManager) SetCache(cache cache.SCIPCache) {
	m.scipCache = cache
	if cache != nil {
		common.LSPLogger.Debug("Cache injected into LSP manager")
	} else {
		common.LSPLogger.Debug("Cache removed from LSP manager")
	}
}

// ensureDocumentOpen sends a textDocument/didOpen notification if needed
func (m *LSPManager) ensureDocumentOpen(client types.LSPClient, uri string, params interface{}) {
	// Check if this is a StdioClient with document tracking capability
	if stdioClient, ok := client.(*StdioClient); ok {
		// Check if document is already open to prevent duplicate didOpen
		stdioClient.mu.Lock()
		if stdioClient.openDocs[uri] {
			stdioClient.mu.Unlock()
			return
		}
		stdioClient.mu.Unlock()
	}

	// Use document manager to ensure document is open
	err := m.documentManager.EnsureOpen(client, uri, params)
	if err != nil {
		common.LSPLogger.Error("Failed to ensure document open for %s: %v", uri, err)
		return
	}

	// Mark document as open AFTER successful didOpen (preserve tracking functionality)
	if stdioClient, ok := client.(*StdioClient); ok {
		stdioClient.mu.Lock()
		stdioClient.openDocs[uri] = true
		stdioClient.mu.Unlock()
	}
}
