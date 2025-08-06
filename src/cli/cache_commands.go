package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server"
	"lsp-gateway/src/server/cache"
)

// IndexCache rebuilds the cache index by processing workspace files
func IndexCache(configPath string) error {
	cfg := LoadConfigWithFallback(configPath)

	// Create LSP manager to access cache
	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LSP manager: %w", err)
	}

	cacheInstance := manager.GetCache()
	if cacheInstance == nil {
		common.CLILogger.Info("❌ Cache: Not available")
		return nil
	}

	metrics, err := cacheInstance.HealthCheck()
	if err != nil {
		common.CLILogger.Error("❌ Cache: Unable to get status (%v)", err)
		return err
	}

	if metrics == nil {
		common.CLILogger.Info("⚫ Cache: Disabled by configuration")
		return nil
	}

	common.CLILogger.Info("🔄 Cache Index")
	common.CLILogger.Info("%s", strings.Repeat("=", 50))
	common.CLILogger.Info("📋 Starting cache index rebuild...")

	// Get working directory
	wd, err := os.Getwd()
	if err != nil {
		common.CLILogger.Error("❌ Failed to get working directory: %v", err)
		return err
	}
	common.CLILogger.Info("🔍 Scanning workspace: %s", wd)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Start LSP manager to fetch document symbols
	if err := manager.Start(ctx); err != nil {
		common.CLILogger.Error("❌ Failed to start LSP manager: %v", err)
		return err
	}
	defer manager.Stop()

	// Wait for LSP servers to initialize
	time.Sleep(2 * time.Second)

	// Get initial index stats to track actual indexing success
	var initialSymbolCount, initialDocCount int64
	initialStats := cacheInstance.GetIndexStats()
	if initialStats != nil {
		initialSymbolCount = initialStats.SymbolCount
		initialDocCount = initialStats.DocumentCount
		common.CLILogger.Debug("Initial index stats - symbols: %d, documents: %d, status: %s",
			initialSymbolCount, initialDocCount, initialStats.Status)
	}

	// Use the cache manager's workspace indexing method
	if cacheManager, ok := cacheInstance.(*cache.SimpleCacheManager); ok {
		if err := cacheManager.PerformWorkspaceIndexing(ctx, wd, manager); err != nil {
			common.CLILogger.Error("❌ Failed to perform workspace indexing: %v", err)
			return err
		}
		common.CLILogger.Info("💾 Index saved to disk")
	} else {
		common.CLILogger.Error("❌ Cache manager doesn't support workspace indexing")
		return fmt.Errorf("cache manager doesn't support workspace indexing")
	}

	// Get final index stats to check actual indexing results
	var actuallyIndexedSymbols int64 = 0
	var actuallyIndexedDocs int64 = 0
	finalStats := cacheInstance.GetIndexStats()
	if finalStats != nil {
		actuallyIndexedSymbols = finalStats.SymbolCount - initialSymbolCount
		actuallyIndexedDocs = finalStats.DocumentCount - initialDocCount
	}

	// Report results based on actual indexing success
	if finalStats != nil {
		common.CLILogger.Debug("Final index stats - symbols: %d, documents: %d, status: %s",
			finalStats.SymbolCount, finalStats.DocumentCount, finalStats.Status)
	}

	if actuallyIndexedSymbols > 0 {
		common.CLILogger.Info("✅ Cache index rebuilt successfully - indexed %d symbols from %d documents", actuallyIndexedSymbols, actuallyIndexedDocs)
	} else {
		common.CLILogger.Warn("⚠️  Cache indexing completed but no symbols were indexed")
		common.CLILogger.Error("❌ No symbols indexed - check:")
		common.CLILogger.Error("   • LSP servers are returning document symbols (enable LSP_GATEWAY_DEBUG=true)")
		common.CLILogger.Error("   • Cache is properly enabled and configured")
		common.CLILogger.Error("   • Files exist in the workspace")
	}

	// Show updated stats
	if updatedMetrics, err := cacheInstance.HealthCheck(); err == nil && updatedMetrics != nil {
		common.CLILogger.Info("📊 Updated cache stats: %d entries", updatedMetrics.EntryCount)
	}

	return nil
}

// ClearCache clears all cache entries
func ClearCache(configPath string) error {
	cfg := LoadConfigWithFallback(configPath)

	// Create LSP manager to access cache
	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LSP manager: %w", err)
	}

	cache := manager.GetCache()
	if cache == nil {
		common.CLILogger.Info("❌ Cache: Not available")
		return nil
	}

	metrics, err := cache.HealthCheck()
	if err != nil {
		common.CLILogger.Error("❌ Cache: Unable to get status (%v)", err)
		return err
	}

	if metrics == nil {
		common.CLILogger.Info("⚫ Cache: Disabled by configuration")
		return nil
	}

	common.CLILogger.Info("🧹 Cache Clear")
	common.CLILogger.Info("%s", strings.Repeat("=", 50))

	// Clear the cache
	if err := cache.Clear(); err != nil {
		common.CLILogger.Error("❌ Failed to clear cache: %v", err)
		return err
	}

	common.CLILogger.Info("✅ Cache cleared successfully")

	return nil
}

// ShowCacheInfo displays brief statistics about cached data
func ShowCacheInfo(configPath string) error {
	cfg := LoadConfigWithFallback(configPath)

	// Create LSP manager to access cache
	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LSP manager: %w", err)
	}

	cacheInstance := manager.GetCache()
	if cacheInstance == nil {
		common.CLILogger.Info("❌ Cache: Not available")
		return nil
	}

	metrics, err := cacheInstance.HealthCheck()
	if err != nil {
		common.CLILogger.Error("❌ Cache: Unable to get status (%v)", err)
		return err
	}

	common.CLILogger.Info("📊 Cache Info")
	common.CLILogger.Info("%s", strings.Repeat("=", 50))

	if metrics == nil {
		common.CLILogger.Info("❌ No cache metrics available")
		return nil
	}

	// Display cache status
	if metrics == nil {
		common.CLILogger.Info("⚫ Status: Disabled")
		return nil
	}

	// Basic statistics
	common.CLILogger.Info("📈 Cache Statistics:")
	common.CLILogger.Info("  • Entries: %d", metrics.EntryCount)
	common.CLILogger.Info("  • Memory: %s", formatBytes(metrics.TotalSize))

	// Hit/Miss ratio
	totalRequests := metrics.HitCount + metrics.MissCount
	if totalRequests > 0 {
		hitRatio := float64(metrics.HitCount) / float64(totalRequests) * 100
		common.CLILogger.Info("  • Hit Rate: %.1f%% (%d/%d)", hitRatio, metrics.HitCount, totalRequests)
	} else {
		common.CLILogger.Info("  • Hit Rate: No requests yet")
	}

	// Evictions if any
	if metrics.EvictionCount > 0 {
		common.CLILogger.Info("  • Evictions: %d", metrics.EvictionCount)
	}

	// Get and display index statistics
	indexStats := cacheInstance.GetIndexStats()
	if indexStats != nil && indexStats.Status != "disabled" {
		common.CLILogger.Info("")
		common.CLILogger.Info("📑 Index Statistics:")
		common.CLILogger.Info("  • Indexed Documents: %d", indexStats.DocumentCount)
		common.CLILogger.Info("  • Indexed Symbols: %d", indexStats.SymbolCount)

		if indexStats.IndexSize > 0 {
			common.CLILogger.Info("  • Index Size: %s", formatBytes(indexStats.IndexSize))
		}

		// Display per-language statistics if available
		if len(indexStats.LanguageStats) > 0 {
			common.CLILogger.Info("  • Languages:")
			for lang, count := range indexStats.LanguageStats {
				common.CLILogger.Info("    - %s: %d symbols", lang, count)
			}
		}

		// Display last update time if available
		if !indexStats.LastUpdate.IsZero() {
			timeSinceUpdate := time.Since(indexStats.LastUpdate)
			if timeSinceUpdate < time.Minute {
				common.CLILogger.Info("  • Last Updated: %d seconds ago", int(timeSinceUpdate.Seconds()))
			} else if timeSinceUpdate < time.Hour {
				common.CLILogger.Info("  • Last Updated: %d minutes ago", int(timeSinceUpdate.Minutes()))
			} else {
				common.CLILogger.Info("  • Last Updated: %d hours ago", int(timeSinceUpdate.Hours()))
			}
		}
	}

	return nil
}
