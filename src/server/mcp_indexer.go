package server

import (
	"context"
	"fmt"
	"os"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/constants"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/cache"
)

// performInitialIndexing performs initial workspace symbol indexing for enhanced MCP performance
// This method runs asynchronously after LSP servers are initialized to populate the cache
// with commonly accessed symbols, reducing query latency for LLM interactions
func (m *MCPServer) performInitialIndexing() {
	// Wait a bit for LSP servers to fully initialize
	time.Sleep(constants.MCPInitialIndexingDelay)

	ctx, cancel := context.WithTimeout(context.Background(), constants.MCPIndexingTimeout)
	defer cancel()

	// First, try to get all workspace symbols using a broad query
	// Use multiple common patterns to get more symbols
	patterns := []string{"", "a", "e", "s", "t", "i", "n", "r", "o"}
	allSymbols := make(map[string]bool) // Track unique symbols by key

	for _, pattern := range patterns {
		query := types.SymbolPatternQuery{
			Pattern:     pattern,
			FilePattern: "**/*",
			MaxResults:  constants.MCPMaxSymbolResults,
		}

		result, err := m.lspManager.SearchSymbolPattern(ctx, query)
		if err != nil {
			continue
		}

		// Track unique symbols
		for _, sym := range result.Symbols {
			key := fmt.Sprintf("%s:%s:%d", sym.FilePath, sym.Name, sym.LineNumber)
			allSymbols[key] = true
		}

	}

	// Additionally, scan workspace files and index them directly
	// This ensures we get document symbols with full ranges
	m.performWorkspaceIndexing(ctx)

	// Save the index to disk for persistence
	if cacheManager, ok := m.scipCache.(*cache.SCIPCacheManager); ok {
		if err := cacheManager.SaveIndexToDisk(); err != nil {
			common.LSPLogger.Error("MCP server: Failed to save index to disk: %v", err)
		} else {
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

	// Delegate to cache module's workspace indexing with MCP-specific limits
	if _, ok := m.scipCache.(*cache.SCIPCacheManager); ok {
		// Create a limited workspace indexer for MCP mode
		indexer := cache.NewWorkspaceIndexer(m.lspManager)

		// Get configured languages for extension mapping
		configuredLangs := []string{}
		for lang := range m.lspManager.GetConfiguredServers() {
			configuredLangs = append(configuredLangs, lang)
		}

		// Use MCP-specific file limit for initial indexing
		err := indexer.IndexWorkspaceFiles(ctx, workspaceDir, configuredLangs, constants.MCPMaxIndexFiles)
		if err != nil {
			common.LSPLogger.Error("MCP server: Workspace indexing failed: %v", err)
		}
	} else {
		common.LSPLogger.Warn("MCP server: Cache not available for workspace indexing")
	}
}
