package server

import (
	"context"
	"fmt"
	"os"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/constants"
	"lsp-gateway/src/server/cache"
)

// performInitialIndexing performs initial workspace symbol indexing for enhanced MCP performance
// This method runs asynchronously after LSP servers are initialized to populate the cache
// with commonly accessed symbols, reducing query latency for LLM interactions
func (m *MCPServer) performInitialIndexing() {
	common.LSPLogger.Info("MCP: Starting initial indexing")

	// Wait a bit for LSP servers to fully initialize
	time.Sleep(constants.MCPInitialIndexingDelay)

	ctx, cancel := common.CreateContext(constants.MCPIndexingTimeout)
	defer cancel()

	// First, try to get all workspace symbols using a broad query
	// Use multiple common patterns to get more symbols
	patterns := []string{"", "a", "e", "s", "t", "i", "n", "r", "o"}
	allSymbols := make(map[string]bool) // Track unique symbols by key

	common.LSPLogger.Info("MCP: Searching workspace symbols with patterns")
	for _, pattern := range patterns {
		found := 0
		// Prefer cache search service when available
		if m.lspManager != nil && m.lspManager.scipCache != nil {
			// Try enhanced symbols via concrete manager if possible
			if cm, ok := m.lspManager.scipCache.(*cache.SCIPCacheManager); ok {
				if res, err := cm.SearchSymbolsEnhanced(ctx, &cache.EnhancedSymbolQuery{Pattern: pattern, MaxResults: constants.MCPMaxSymbolResults}); err == nil && res != nil {
					for _, s := range res.Symbols {
						key := fmt.Sprintf("%s:%s", s.SymbolID, s.DisplayName)
						allSymbols[key] = true
						found++
					}
					common.LSPLogger.Debug("MCP: Pattern '%s' found %d symbols (enhanced)", pattern, found)
					continue
				}
			}
			// Fallback to simple symbol search
			if syms, err := m.lspManager.scipCache.SearchSymbols(ctx, pattern, "**/*", constants.MCPMaxSymbolResults); err == nil && syms != nil {
				for _, it := range syms {
					if m, ok := it.(map[string]interface{}); ok {
						name, _ := m["displayName"].(string)
						if name == "" {
							if info, ok2 := m["symbol_info"].(map[string]interface{}); ok2 {
								if dn, ok3 := info["displayName"].(string); ok3 {
									name = dn
								}
							}
						}
						id, _ := m["symbol"].(string)
						key := fmt.Sprintf("%s:%s", id, name)
						allSymbols[key] = true
						found++
					}
				}
				common.LSPLogger.Debug("MCP: Pattern '%s' found %d symbols (simple)", pattern, found)
				continue
			}
		}
		// No cache available; skip LSP fallback to maintain single search pathway
		common.LSPLogger.Debug("MCP: Cache unavailable; skipping LSP fallback for pattern '%s'", pattern)
	}
	common.LSPLogger.Info("MCP: Found %d unique symbols from pattern search", len(allSymbols))

	// Additionally, scan workspace files and index them directly
	// This ensures we get document symbols with full ranges
	common.LSPLogger.Info("MCP: Starting workspace file indexing")
	m.performWorkspaceIndexing(ctx)

	// Save the index to disk for persistence
	if cacheManager, ok := m.lspManager.scipCache.(*cache.SCIPCacheManager); ok {
		if err := cacheManager.SaveIndexToDisk(); err != nil {
			common.LSPLogger.Error("MCP server: Failed to save index to disk: %v", err)
		}
	}
}

// performWorkspaceIndexing delegates workspace file indexing to the cache module
// This method uses the shared WorkspaceIndexer infrastructure while maintaining
// MCP-specific file count limits for optimized initial indexing performance
func (m *MCPServer) performWorkspaceIndexing(ctx context.Context) {
	// Get workspace directory
	workspaceDir, err := os.Getwd()
	if err != nil {
		workspaceDir = "."
	}
	common.LSPLogger.Info("MCP: Workspace directory: %s", workspaceDir)

	// Delegate to cache module's workspace indexing with MCP-specific limits
	if cacheManager, ok := m.lspManager.scipCache.(*cache.SCIPCacheManager); ok {
		// Get configured languages for extension mapping
		configuredLangs := []string{}
		for lang := range m.lspManager.GetConfiguredServers() {
			configuredLangs = append(configuredLangs, lang)
		}

		// Use the common full indexing method (basic + enhanced references)
		common.LSPLogger.Debug("MCP server: Starting full workspace indexing")
		err := cacheManager.PerformFullIndexing(ctx, workspaceDir, configuredLangs, constants.MCPMaxIndexFiles, m.lspManager)
		if err != nil {
			common.LSPLogger.Error("MCP server: Full workspace indexing failed: %v", err)
		} else {
			common.LSPLogger.Debug("MCP server: Full workspace indexing complete")
		}
	} else {
		common.LSPLogger.Warn("MCP server: Cache not available for workspace indexing")
	}
}
