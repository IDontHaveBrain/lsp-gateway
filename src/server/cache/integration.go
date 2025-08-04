package cache

import (
	"context"
	"lsp-gateway/src/internal/common"
)

// SimpleCacheIntegration - DEPRECATED: Use direct LSPManager cache integration instead
// This wrapper is maintained for backward compatibility but adds unnecessary abstraction
type SimpleCacheIntegration struct {
	lspManager LSPFallback
	cache      SCIPCache
	enabled    bool
}

// NewSimpleCacheIntegration - DEPRECATED: Use LSPManager.SetCache() for direct integration
func NewSimpleCacheIntegration(lspManager LSPFallback, cache SCIPCache) *SimpleCacheIntegration {
	common.LSPLogger.Warn("SimpleCacheIntegration is deprecated - use direct LSPManager cache integration")
	return &SimpleCacheIntegration{
		lspManager: lspManager,
		cache:      cache,
		enabled:    true,
	}
}

// ProcessRequest - DEPRECATED: Direct delegation to LSP manager (wrapper removed)
func (c *SimpleCacheIntegration) ProcessRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	// Simplified implementation - direct delegation without wrapper logic
	return c.lspManager.ProcessRequest(ctx, method, params)
}

// Enable - DEPRECATED: No-op method (cache control moved to LSPManager)
func (c *SimpleCacheIntegration) Enable(enabled bool) {
	common.LSPLogger.Warn("SimpleCacheIntegration.Enable() is deprecated - use LSPManager cache methods")
}

// IsEnabled - DEPRECATED: Always returns true (cache control moved to LSPManager)
func (c *SimpleCacheIntegration) IsEnabled() bool {
	return true
}

// GetStatus - DEPRECATED: Returns minimal status (detailed metrics moved to LSPManager)
func (c *SimpleCacheIntegration) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"status":     "deprecated",
		"message":    "Use LSPManager cache methods instead",
		"migration":  "Replace with direct LSPManager.SetCache() usage",
	}
}
