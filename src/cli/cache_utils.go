package cli

import (
	"fmt"
	"strings"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server"
	"lsp-gateway/src/server/cache"
)

// Cache display constants
const (
	CacheIconHealthy  = "✅"
	CacheIconWarning  = "⚠️"
	CacheIconError    = "❌"
	CacheIconDisabled = "⚫"
	CacheIconUnknown  = "❓"
	CacheSeparator    = "="
	CacheSubSeparator = "-"
	CacheSeparatorLen = 50
	CacheSubSepLen    = 30
)

// GetCacheStatusIcon returns appropriate status icons based on health status
func GetCacheStatusIcon(health string) string {
	switch strings.ToUpper(health) {
	case "OK":
		return CacheIconHealthy
	case "DEGRADED":
		return CacheIconWarning
	case "FAILING", "CRITICAL":
		return CacheIconError
	case "DISABLED":
		return CacheIconDisabled
	default:
		return CacheIconUnknown
	}
}

// GetCacheMetricsFromManager safely extracts cache metrics from LSP manager
func GetCacheMetricsFromManager(manager *server.LSPManager) (*cache.CacheMetrics, error) {
	if manager == nil {
		return nil, fmt.Errorf("LSP manager is nil")
	}

	// Get cache metrics through the public interface
	metricsInterface := manager.GetCacheMetrics()
	if metricsInterface == nil {
		return nil, fmt.Errorf("cache metrics are nil")
	}

	// Type assert to CacheMetrics
	metrics, ok := metricsInterface.(*cache.CacheMetrics)
	if !ok {
		return nil, fmt.Errorf("cache metrics type assertion failed")
	}

	return metrics, nil
}

// Helper functions

// formatBytes converts bytes to human-readable format
func formatBytes(bytes int64) string {
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

// GetCacheStatusText returns a human-readable cache status description
func GetCacheStatusText(metrics *cache.CacheMetrics) string {
	if metrics == nil {
		return "Unavailable"
	}

	switch metrics.HealthStatus {
	case "OK":
		return "Operating normally"
	case "DEGRADED":
		return "Performance degraded"
	case "FAILING":
		return "Experiencing failures"
	case "CRITICAL":
		return "Critical issues detected"
	case "DISABLED":
		return "Disabled by configuration"
	default:
		return "Unknown status"
	}
}

// DisplayCacheStatus shows comprehensive cache status for CLI commands
// This is the main function called by CLI commands like status, test, etc.
func DisplayCacheStatus(manager *server.LSPManager) {
	if manager == nil {
		common.CLILogger.Info("")
		common.CLILogger.Info("🗄️  Cache Status:")
		common.CLILogger.Info("%s", strings.Repeat(CacheSubSeparator, CacheSubSepLen))
		common.CLILogger.Info("%s Cache: Manager not available", CacheIconError)
		return
	}

	common.CLILogger.Info("")
	common.CLILogger.Info("🗄️  Cache Status:")
	common.CLILogger.Info("%s", strings.Repeat(CacheSubSeparator, CacheSubSepLen))

	metrics, err := GetCacheMetricsFromManager(manager)
	if err != nil {
		common.CLILogger.Info("%s Cache: Unable to get metrics (%v)", CacheIconError, err)
		return
	}

	if metrics == nil {
		common.CLILogger.Info("%s Cache: No metrics available", CacheIconUnknown)
		return
	}

	// Show basic health status
	icon := GetCacheStatusIcon(metrics.HealthStatus)
	statusText := GetCacheStatusText(metrics)
	common.CLILogger.Info("%s Cache: %s", icon, statusText)

	// Show basic metrics if cache is enabled
	if metrics.HealthStatus != "DISABLED" {
		totalRequests := metrics.HitCount + metrics.MissCount
		if totalRequests > 0 {
			hitRatio := float64(metrics.HitCount) / float64(totalRequests) * 100
			common.CLILogger.Info("   📊 Stats: %d entries, %.1f%% hit rate (%d/%d requests)",
				metrics.EntryCount, hitRatio, metrics.HitCount, totalRequests)
		} else {
			common.CLILogger.Info("   📊 Stats: %d entries, no requests processed yet", metrics.EntryCount)
		}

		if metrics.TotalSize > 0 {
			common.CLILogger.Info("   💾 Size: %s", formatBytes(metrics.TotalSize))
		}

		// Show any errors or evictions
		if metrics.ErrorCount > 0 {
			common.CLILogger.Info("   ⚠️  Errors: %d", metrics.ErrorCount)
		}
		if metrics.EvictionCount > 0 {
			common.CLILogger.Info("   🔄 Evictions: %d", metrics.EvictionCount)
		}
	}
}

// displayCacheStatus is a convenience wrapper for DisplayCacheStatus to match
// the naming convention used in commands.go
func displayCacheStatus(manager *server.LSPManager) {
	DisplayCacheStatus(manager)
}
