package cache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/models/lsp"
)

// CachedLSPManager wraps the existing LSP manager with SCIP caching
type CachedLSPManager struct {
	lspManager      LSPFallback
	indexingManager *IndexingManager
	enabled         bool
}

// NewCachedLSPManager creates a new cached LSP manager
func NewCachedLSPManager(lspManager LSPFallback) *CachedLSPManager {
	indexingManager := NewIndexingManager(lspManager, IncrementalIndex)

	return &CachedLSPManager{
		lspManager:      lspManager,
		indexingManager: indexingManager,
		enabled:         true,
	}
}

// Start initializes the caching system
func (c *CachedLSPManager) Start() error {
	if !c.enabled {
		return nil
	}

	common.LSPLogger.Info("Starting SCIP caching system")

	err := c.indexingManager.Start()
	if err != nil {
		return fmt.Errorf("failed to start indexing manager: %w", err)
	}

	common.LSPLogger.Info("SCIP caching system started successfully")
	return nil
}

// Stop shuts down the caching system
func (c *CachedLSPManager) Stop() error {
	if !c.enabled {
		return nil
	}

	common.LSPLogger.Info("Stopping SCIP caching system")

	err := c.indexingManager.Stop()
	if err != nil {
		return fmt.Errorf("failed to stop indexing manager: %w", err)
	}

	common.LSPLogger.Info("SCIP caching system stopped")
	return nil
}

// ProcessRequest processes LSP requests with caching
func (c *CachedLSPManager) ProcessRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	if !c.enabled || !c.isMethodSupported(method) {
		// Fall back to original LSP manager for unsupported methods
		return c.lspManager.ProcessRequest(ctx, method, params)
	}

	// Try to handle with cached query manager first
	queryManager := c.indexingManager.GetQueryManager()

	// Trigger indexing for the file if needed
	c.triggerIndexingIfNeeded(method, params)

	// Route to appropriate cached method
	switch method {
	case "textDocument/definition":
		if definitionParams, ok := params.(*lsp.DefinitionParams); ok {
			return queryManager.GetDefinition(ctx, definitionParams)
		}
	case "textDocument/references":
		if refParams, ok := params.(*lsp.ReferenceParams); ok {
			return queryManager.GetReferences(ctx, refParams)
		}
	case "textDocument/hover":
		if hoverParams, ok := params.(*lsp.HoverParams); ok {
			return queryManager.GetHover(ctx, hoverParams)
		}
	case "textDocument/documentSymbol":
		if docSymbolParams, ok := params.(*lsp.DocumentSymbolParams); ok {
			return queryManager.GetDocumentSymbols(ctx, docSymbolParams)
		}
	case "workspace/symbol":
		if wsSymbolParams, ok := params.(*lsp.WorkspaceSymbolParams); ok {
			return queryManager.GetWorkspaceSymbols(ctx, wsSymbolParams)
		}
	case "textDocument/completion":
		if completionParams, ok := params.(*lsp.CompletionParams); ok {
			return queryManager.GetCompletion(ctx, completionParams)
		}
	}

	// Fallback to original LSP manager
	common.LSPLogger.Debug("Falling back to LSP manager for method: %s", method)
	return c.lspManager.ProcessRequest(ctx, method, params)
}

// Enable enables or disables the caching system
func (c *CachedLSPManager) Enable(enabled bool) {
	c.enabled = enabled
	if enabled {
		common.LSPLogger.Info("SCIP caching enabled")
	} else {
		common.LSPLogger.Info("SCIP caching disabled")
	}
}

// IsEnabled returns whether caching is enabled
func (c *CachedLSPManager) IsEnabled() bool {
	return c.enabled
}

// GetHealthStatus returns detailed health information
func (c *CachedLSPManager) GetHealthStatus() map[string]interface{} {
	if !c.enabled {
		return map[string]interface{}{
			"enabled": false,
			"status":  "disabled",
		}
	}

	health := c.indexingManager.GetHealthStatus()
	health["enabled"] = true
	health["ready"] = c.indexingManager.IsReady()

	return health
}

// GetStats returns performance statistics
func (c *CachedLSPManager) GetStats() map[string]interface{} {
	if !c.enabled {
		return map[string]interface{}{
			"enabled": false,
		}
	}

	queryManager := c.indexingManager.GetQueryManager()
	indexer := c.indexingManager.GetIndexer()

	queryMetrics := queryManager.GetMetrics()
	indexStats := indexer.GetStats()

	return map[string]interface{}{
		"enabled": true,
		"query_performance": map[string]interface{}{
			"total_queries":     queryMetrics.TotalQueries,
			"cache_hits":        queryMetrics.CacheHits,
			"cache_misses":      queryMetrics.CacheMisses,
			"hit_ratio":         calculateHitRatio(queryMetrics.CacheHits, queryMetrics.TotalQueries),
			"avg_response_time": queryMetrics.AvgResponseTime.String(),
		},
		"indexing_performance": map[string]interface{}{
			"documents_indexed": indexStats.DocumentsIndexed,
			"symbols_extracted": indexStats.SymbolsExtracted,
			"avg_index_time":    indexStats.AverageIndexTime.String(),
			"total_jobs":        indexStats.TotalJobs,
			"completed_jobs":    indexStats.CompletedJobs,
			"failed_jobs":       indexStats.FailedJobs,
		},
		"hot_files": indexer.GetHotFiles(10),
	}
}

// Private helper methods

// isMethodSupported checks if the method is supported by SCIP caching
func (c *CachedLSPManager) isMethodSupported(method string) bool {
	supportedMethods := map[string]bool{
		"textDocument/definition":     true,
		"textDocument/references":     true,
		"textDocument/hover":          true,
		"textDocument/documentSymbol": true,
		"workspace/symbol":            true,
		"textDocument/completion":     true,
	}

	return supportedMethods[method]
}

// triggerIndexingIfNeeded triggers indexing for files that need it
func (c *CachedLSPManager) triggerIndexingIfNeeded(method string, params interface{}) {
	var uri string
	var language string

	// Extract URI from different parameter types
	switch p := params.(type) {
	case *lsp.DefinitionParams:
		uri = p.TextDocument.URI
	case *lsp.ReferenceParams:
		uri = p.TextDocument.URI
	case *lsp.HoverParams:
		uri = p.TextDocument.URI
	case *lsp.DocumentSymbolParams:
		uri = p.TextDocument.URI
	case *lsp.CompletionParams:
		uri = p.TextDocument.URI
	default:
		return // No URI to extract
	}

	if uri == "" {
		return
	}

	// Detect language from URI
	language = c.detectLanguageFromURI(uri)
	if language == "" {
		return
	}

	// Determine priority based on method
	priority := c.getMethodPriority(method)

	// Request indexing
	c.indexingManager.IndexFile(uri, language, priority)

	common.LSPLogger.Debug("Triggered indexing for %s (language: %s, priority: %d)", uri, language, priority)
}

// detectLanguageFromURI detects programming language from file URI
func (c *CachedLSPManager) detectLanguageFromURI(uri string) string {
	// Simple language detection based on file extension
	lowerURI := strings.ToLower(uri)

	switch {
	case strings.HasSuffix(lowerURI, ".go"):
		return "go"
	case strings.HasSuffix(lowerURI, ".py"):
		return "python"
	case strings.HasSuffix(lowerURI, ".js") || strings.HasSuffix(lowerURI, ".jsx"):
		return "javascript"
	case strings.HasSuffix(lowerURI, ".ts") || strings.HasSuffix(lowerURI, ".tsx"):
		return "typescript"
	case strings.HasSuffix(lowerURI, ".java"):
		return "java"
	default:
		return ""
	}
}

// getMethodPriority returns priority level for different LSP methods
func (c *CachedLSPManager) getMethodPriority(method string) int {
	priorities := map[string]int{
		"textDocument/definition":     10, // High priority - user navigation
		"textDocument/references":     9,  // High priority - user navigation
		"textDocument/hover":          8,  // High priority - immediate feedback
		"textDocument/completion":     7,  // Medium-high priority - typing assistance
		"textDocument/documentSymbol": 5,  // Medium priority - outline view
		"workspace/symbol":            3,  // Lower priority - search functionality
	}

	if priority, exists := priorities[method]; exists {
		return priority
	}
	return 1 // Default low priority
}

// calculateHitRatio calculates cache hit ratio
func calculateHitRatio(hits, total int64) float64 {
	if total == 0 {
		return 0.0
	}
	return float64(hits) / float64(total)
}

// CacheWarmupManager handles proactive cache warming
type CacheWarmupManager struct {
	cachedManager  *CachedLSPManager
	warmupEnabled  bool
	warmupInterval time.Duration
	stopChan       chan struct{}
}

// NewCacheWarmupManager creates a new cache warmup manager
func NewCacheWarmupManager(cachedManager *CachedLSPManager) *CacheWarmupManager {
	return &CacheWarmupManager{
		cachedManager:  cachedManager,
		warmupEnabled:  true,
		warmupInterval: 5 * time.Minute,
		stopChan:       make(chan struct{}),
	}
}

// Start begins the cache warmup process
func (w *CacheWarmupManager) Start() {
	if !w.warmupEnabled {
		return
	}

	go w.warmupLoop()
	common.LSPLogger.Info("Cache warmup manager started")
}

// Stop stops the cache warmup process
func (w *CacheWarmupManager) Stop() {
	close(w.stopChan)
	common.LSPLogger.Info("Cache warmup manager stopped")
}

// warmupLoop runs the periodic cache warmup
func (w *CacheWarmupManager) warmupLoop() {
	ticker := time.NewTicker(w.warmupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.performWarmup()
		case <-w.stopChan:
			return
		}
	}
}

// performWarmup performs proactive cache warming for hot files
func (w *CacheWarmupManager) performWarmup() {
	if !w.cachedManager.IsEnabled() {
		return
	}

	indexer := w.cachedManager.indexingManager.GetIndexer()
	hotFiles := indexer.GetHotFiles(20) // Get top 20 hot files

	common.LSPLogger.Debug("Performing cache warmup for %d hot files", len(hotFiles))

	for _, uri := range hotFiles {
		language := w.cachedManager.detectLanguageFromURI(uri)
		if language != "" {
			// Request background indexing with low priority
			w.cachedManager.indexingManager.IndexFile(uri, language, 1)
		}
	}
}

// Example integration with existing LSP manager

// IntegrateWithLSPManager shows how to integrate SCIP caching with existing LSP manager
func IntegrateWithLSPManager(existingLSPManager LSPFallback) (*CachedLSPManager, error) {
	// Create cached LSP manager
	cachedManager := NewCachedLSPManager(existingLSPManager)

	// Start the caching system
	err := cachedManager.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start SCIP caching: %w", err)
	}

	// Optional: Start cache warmup manager
	warmupManager := NewCacheWarmupManager(cachedManager)
	warmupManager.Start()

	common.LSPLogger.Info("SCIP caching successfully integrated with LSP manager")

	return cachedManager, nil
}

// GetPerformanceReport generates a detailed performance report
func GetPerformanceReport(cachedManager *CachedLSPManager) map[string]interface{} {
	stats := cachedManager.GetStats()
	health := cachedManager.GetHealthStatus()

	report := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"enabled":   cachedManager.IsEnabled(),
		"ready":     cachedManager.indexingManager.IsReady(),
		"health":    health,
		"stats":     stats,
	}

	// Add performance analysis
	if queryPerf, ok := stats["query_performance"].(map[string]interface{}); ok {
		if hitRatio, ok := queryPerf["hit_ratio"].(float64); ok {
			var performance string
			switch {
			case hitRatio >= 0.9:
				performance = "excellent"
			case hitRatio >= 0.7:
				performance = "good"
			case hitRatio >= 0.5:
				performance = "fair"
			default:
				performance = "poor"
			}
			report["performance_rating"] = performance
		}
	}

	return report
}
