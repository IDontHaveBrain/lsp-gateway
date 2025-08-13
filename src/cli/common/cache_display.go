package common

import (
	"fmt"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server/cache"
)

// DisplayCacheStatus displays cache status in different modes for various contexts
// mode options: "gateway", "mcp", "cli"
func DisplayCacheStatus(logger *common.SafeLogger, scipCache cache.SCIPCache, mode string) {
	if scipCache == nil {
		switch mode {
		case "gateway":
			logger.Warn("SCIP Cache: Not available")
		case "mcp":
			logger.Warn("SCIP Cache: Not available")
		case "cli":
			logger.Info("")
			logger.Info("Cache Status:")
			logger.Info("Cache: Manager not available")
		}
		return
	}

	metrics, healthErr := scipCache.HealthCheck()
	if healthErr != nil {
		switch mode {
		case "gateway":
			logger.Error("SCIP Cache: Health check failed (%v)", healthErr)
		case "mcp":
			logger.Error("SCIP Cache: Health check failed (%v)", healthErr)
		case "cli":
			logger.Info("")
			logger.Info("Cache Status:")
			logger.Info("Cache: Unable to get metrics (%v)", healthErr)
		}
		return
	}

	switch mode {
	case "gateway":
		displayGatewayCache(logger, metrics)
	case "mcp":
		displayMCPCache(logger, metrics)
	case "cli":
		displayCLICache(logger, metrics)
	}
}

// displayGatewayCache displays cache status for HTTP Gateway startup
func displayGatewayCache(logger *common.SafeLogger, metrics *cache.CacheMetrics) {
	if metrics != nil {
		logger.Info("SCIP Cache: Initialized and Ready (100MB limit, 30min TTL)")
		logger.Info("Health: OK, Stats: %d entries, %s used", metrics.EntryCount, FormatBytes(metrics.TotalSize))
	} else {
		logger.Info("SCIP Cache: Initialized and Ready")
		logger.Info("Health: OK, Stats: 0 entries, 0MB used")
	}
}

// displayMCPCache displays cache status for MCP server startup
func displayMCPCache(logger *common.SafeLogger, metrics *cache.CacheMetrics) {
	if metrics != nil {
		logger.Info("SCIP Cache: Initialized and Ready")
		logger.Info("   Health: OK, Stats: %d entries, %s used", metrics.EntryCount, FormatBytes(metrics.TotalSize))
	} else {
		logger.Info("SCIP Cache: Initialized and Ready")
		logger.Info("   Health: OK, Stats: 0 entries, 0MB used")
	}
}

// displayCLICache displays comprehensive cache status for CLI commands
func displayCLICache(logger *common.SafeLogger, metrics *cache.CacheMetrics) {
	logger.Info("")
	logger.Info("Cache Status:")

	if metrics == nil {
		logger.Info("Cache: No metrics available")
		return
	}

	logger.Info("Cache: Operating normally")

	totalRequests := metrics.HitCount + metrics.MissCount
	if totalRequests > 0 {
		hitRatio := cache.HitRate(metrics)
		logger.Info("   Stats: %d entries, %.1f%% hit rate (%d/%d requests)",
			metrics.EntryCount, hitRatio, metrics.HitCount, totalRequests)
	} else {
		logger.Info("   Stats: %d entries, no requests processed yet", metrics.EntryCount)
	}

	if metrics.TotalSize > 0 {
		logger.Info("   Size: %s", FormatBytes(metrics.TotalSize))
	}

	if metrics.ErrorCount > 0 {
		logger.Info("   Errors: %d", metrics.ErrorCount)
	}
	if metrics.EvictionCount > 0 {
		logger.Info("   Evictions: %d", metrics.EvictionCount)
	}
}

// FormatCacheMetrics formats cache metrics as a string for external use
func FormatCacheMetrics(metrics *cache.CacheMetrics) string {
	if metrics == nil {
		return "No metrics available"
	}

	totalRequests := metrics.HitCount + metrics.MissCount
	if totalRequests > 0 {
		hitRatio := cache.HitRate(metrics)
		return fmt.Sprintf("%d entries, %.1f%% hit rate (%d/%d requests), %s",
			metrics.EntryCount, hitRatio, metrics.HitCount, totalRequests, FormatBytes(metrics.TotalSize))
	}

	return fmt.Sprintf("%d entries, no requests processed yet, %s",
		metrics.EntryCount, FormatBytes(metrics.TotalSize))
}

// FormatBytes converts bytes to human-readable format
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
