package cli

import (
	"fmt"
	"sort"
	"time"

	clicommon "lsp-gateway/src/cli/common"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server/cache"
	"lsp-gateway/src/server/scip"
)

// IndexCache rebuilds the cache index by processing workspace files
func IndexCache(configPath string) error {
	cmdCtx, err := clicommon.NewCommandContext(configPath, 5*time.Minute)
	if err != nil {
		return err
	}
	defer cmdCtx.Cleanup()

	// Get working directory
	wd, err := cmdCtx.GetWorkingDir()
	if err != nil {
		common.CLILogger.Error("Failed to get working directory: %v", err)
		return err
	}

	_, err = cmdCtx.CheckCacheHealth()
	if err != nil {
		return nil // CheckCacheHealth already logged the appropriate message
	}

	common.CLILogger.Info("Cache Index")
	common.CLILogger.Info("Starting cache index rebuild...")
	common.CLILogger.Info("Scanning workspace: %s", wd)

	// Wait for LSP servers to initialize
	cmdCtx.WaitForInitialization()

	// Get initial index stats to track actual indexing success
	var initialSymbolCount, initialDocCount int64
	initialStats := cmdCtx.Cache.GetIndexStats()
	if initialStats != nil {
		initialSymbolCount = initialStats.SymbolCount
		initialDocCount = initialStats.DocumentCount
	}

	if cacheManager, ok := cmdCtx.Cache.(*cache.SCIPCacheManager); ok {
		progress := func(phase string, current, total int, detail string) {
			switch phase {
			case "index_start":
				common.CLILogger.Info("Indexing %d files", total)
			case "index_file":
				if verbose {
					common.CLILogger.Info("Indexed %d/%d: %s", current, total, detail)
				} else if current%50 == 0 || current == total {
					common.CLILogger.Info("Indexed %d/%d files", current, total)
				}
			case "index_complete":
				common.CLILogger.Info("Indexed %d/%d files", current, total)
			case "references_start":
				common.CLILogger.Info("Processing references for %d symbols", total)
			case "references":
				if verbose {
					common.CLILogger.Info("References %d/%d", current, total)
				} else if current%100 == 0 || current == total {
					common.CLILogger.Info("References %d/%d", current, total)
				}
			case "references_complete":
				common.CLILogger.Info("References processed: %d", current)
			}
		}
		// Use incremental indexing by default
		common.CLILogger.Info("Performing incremental indexing (checking for changes)...")
		if err := cacheManager.PerformIncrementalIndexingWithProgress(cmdCtx.Context, wd, cmdCtx.Manager, progress); err != nil {
			common.CLILogger.Error("❌ Failed to perform incremental indexing: %v", err)
			return err
		}
		common.CLILogger.Info("Index saved to disk")
	} else {
		common.CLILogger.Error("Cache manager doesn't support workspace indexing")
		return fmt.Errorf("cache manager doesn't support workspace indexing")
	}

	// Get final index stats to check actual indexing results
	var actuallyIndexedSymbols int64 = 0
	var actuallyIndexedDocs int64 = 0
	finalStats := cmdCtx.Cache.GetIndexStats()
	if finalStats != nil {
		actuallyIndexedSymbols = finalStats.SymbolCount - initialSymbolCount
		actuallyIndexedDocs = finalStats.DocumentCount - initialDocCount
	}

	// Report results based on actual indexing success

	if actuallyIndexedSymbols > 0 {
		common.CLILogger.Info("Cache index rebuilt successfully - indexed %d symbols from %d documents", actuallyIndexedSymbols, actuallyIndexedDocs)
	} else {
		// Check if this was incremental indexing with no changes
		if cacheManager, ok := cmdCtx.Cache.(*cache.SCIPCacheManager); ok {
			trackedCount := cacheManager.GetTrackedFileCount()
			if trackedCount > 0 {
				// Files are already indexed, no changes detected
				common.CLILogger.Info("Incremental indexing complete - no changes detected in %d tracked files", trackedCount)

				// Show current cache stats
				if finalStats != nil {
					common.CLILogger.Info("Current index contains %d symbols from %d documents",
						finalStats.SymbolCount, finalStats.DocumentCount)
				}
			} else {
				// No files tracked and no symbols indexed - likely an error
				common.CLILogger.Warn("Cache indexing completed but no symbols were indexed")
				common.CLILogger.Error("No symbols indexed - check:")
				common.CLILogger.Error("   • LSP servers are returning document symbols (enable LSP_GATEWAY_DEBUG=true)")
				common.CLILogger.Error("   • Cache is properly enabled and configured")
				common.CLILogger.Error("   • Files exist in the workspace")

				return fmt.Errorf("cache indexing failed: no symbols were indexed")
			}
		}
	}

	// Show updated stats
	if updatedMetrics, err := cmdCtx.Cache.HealthCheck(); err == nil && updatedMetrics != nil {
		common.CLILogger.Info("Updated cache stats: %d entries", updatedMetrics.EntryCount)
	}

	return nil
}

// ClearCache clears all cache entries
func ClearCache(configPath string) error {
	cmdCtx, err := clicommon.NewCacheOnlyContext(configPath, 30*time.Second)
	if err != nil {
		return err
	}
	defer cmdCtx.Cleanup()

	_, err = cmdCtx.CheckCacheHealth()
	if err != nil {
		return nil // CheckCacheHealth already logged the appropriate message
	}

	common.CLILogger.Info("Cache Clear")

	// Clear the cache
	if err := cmdCtx.Cache.Clear(); err != nil {
		common.CLILogger.Error("Failed to clear cache: %v", err)
		return err
	}

	common.CLILogger.Info("Cache cleared successfully")

	return nil
}

// ShowCacheInfo displays brief statistics about cached data
func ShowCacheInfo(configPath string) error {
	cmdCtx, err := clicommon.NewCacheOnlyContext(configPath, 30*time.Second)
	if err != nil {
		return err
	}
	defer cmdCtx.Cleanup()

	metrics, err := cmdCtx.CheckCacheHealth()
	if err != nil {
		return nil // CheckCacheHealth already logged the appropriate message
	}

	common.CLILogger.Info("Cache Info")

	if metrics == nil {
		common.CLILogger.Info("No cache metrics available")
		return nil
	}

	// Display cache status
	if metrics == nil {
		common.CLILogger.Info("Status: Disabled")
		return nil
	}

	// Basic statistics
	common.CLILogger.Info("Cache Statistics:")
	common.CLILogger.Info("  • Entries: %d", metrics.EntryCount)
	common.CLILogger.Info("  • Memory: %s", clicommon.FormatBytes(metrics.TotalSize))

	// Hit/Miss ratio
	totalRequests := metrics.HitCount + metrics.MissCount
	if totalRequests > 0 {
		hitRatio := cache.HitRate(metrics)
		common.CLILogger.Info("  • Hit Rate: %.1f%% (%d/%d)", hitRatio, metrics.HitCount, totalRequests)
	} else {
		common.CLILogger.Info("  • Hit Rate: No requests yet")
	}

	// Evictions if any
	if metrics.EvictionCount > 0 {
		common.CLILogger.Info("  • Evictions: %d", metrics.EvictionCount)
	}

	// Get and display index statistics
	indexStats := cmdCtx.Cache.GetIndexStats()
	if indexStats != nil && indexStats.Status != "disabled" {
		common.CLILogger.Info("")
		common.CLILogger.Info("Index Statistics:")
		common.CLILogger.Info("  • Status: %s", indexStats.Status)
		common.CLILogger.Info("  • Indexed Documents: %d", indexStats.DocumentCount)
		common.CLILogger.Info("  • Indexed Symbols: %d", indexStats.SymbolCount)
		common.CLILogger.Info("  • Indexed References: %d", indexStats.ReferenceCount)

		if indexStats.IndexSize > 0 {
			common.CLILogger.Info("  • Index Size: %s", clicommon.FormatBytes(indexStats.IndexSize))
		}

		// Derived metrics
		if indexStats.DocumentCount > 0 && indexStats.SymbolCount > 0 {
			avgSymsPerDoc := float64(indexStats.SymbolCount) / float64(indexStats.DocumentCount)
			common.CLILogger.Info("  • Symbols per Doc: %.1f", avgSymsPerDoc)
		}
		if indexStats.SymbolCount > 0 && indexStats.ReferenceCount > 0 {
			avgRefsPerSym := float64(indexStats.ReferenceCount) / float64(indexStats.SymbolCount)
			common.CLILogger.Info("  • Refs per Symbol: %.1f", avgRefsPerSym)
		}
		if indexStats.DocumentCount > 0 && indexStats.ReferenceCount > 0 {
			avgRefsPerDoc := float64(indexStats.ReferenceCount) / float64(indexStats.DocumentCount)
			common.CLILogger.Info("  • Refs per Doc: %.1f", avgRefsPerDoc)
		}

		// Display per-language statistics if available
		if len(indexStats.LanguageStats) > 0 {
			// Sort languages by symbol count desc
			type langCount struct {
				lang  string
				count int64
			}
			list := make([]langCount, 0, len(indexStats.LanguageStats))
			for lang, count := range indexStats.LanguageStats {
				list = append(list, langCount{lang, count})
			}
			sort.Slice(list, func(i, j int) bool { return list[i].count > list[j].count })
			common.CLILogger.Info("  • Languages (%d):", len(list))
			for _, lc := range list {
				common.CLILogger.Info("    - %s: %d symbols", lc.lang, lc.count)
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

		// Storage details and index hit rate
		if cmdCtx.Config != nil && cmdCtx.Config.Cache != nil {
			disk := "disabled"
			if cmdCtx.Config.Cache.DiskCache {
				disk = "enabled"
			}
			if cmdCtx.Config.Cache.StoragePath != "" {
				common.CLILogger.Info("  • Disk Cache: %s (%s)", disk, cmdCtx.Config.Cache.StoragePath)
			} else {
				common.CLILogger.Info("  • Disk Cache: %s", disk)
			}
		}

		if scipStorage := cmdCtx.Cache.GetSCIPStorage(); scipStorage != nil {
			if sstats, ok := scipStorage.(scip.SCIPDocumentStorage); ok {
				s := sstats.GetIndexStats()
				if s.HitRate > 0 {
					common.CLILogger.Info("  • Index Hit Rate: %.1f%%", s.HitRate*100)
				}
			}
		}
	}

	return nil
}
