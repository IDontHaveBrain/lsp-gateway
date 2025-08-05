package cli

import (
	"fmt"
	"strings"
	"time"

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

// DisplayCacheHealth shows cache health status with appropriate icons and colors
func DisplayCacheHealth(metrics *cache.CacheMetrics) {
	if metrics == nil {
		common.CLILogger.Info("%s Cache: Unavailable", CacheIconUnknown)
		return
	}

	icon := GetCacheStatusIcon(metrics.HealthStatus)
	common.CLILogger.Info("%s Cache Health: %s", icon, metrics.HealthStatus)

	// Show additional health details if not OK
	if metrics.HealthStatus != "OK" && metrics.HealthStatus != "DISABLED" {
		totalRequests := metrics.HitCount + metrics.MissCount
		if totalRequests > 0 {
			errorRate := float64(metrics.ErrorCount) / float64(totalRequests) * 100
			common.CLILogger.Info("   Error Rate: %.1f%% (%d/%d requests)", errorRate, metrics.ErrorCount, totalRequests)
		}

		if !metrics.LastHealthCheck.IsZero() {
			timeSince := time.Since(metrics.LastHealthCheck)
			common.CLILogger.Info("   Last Check: %s ago", formatDuration(timeSince))
		}
	}
}

// FormatCacheMetrics displays detailed cache statistics in a user-friendly format
func FormatCacheMetrics(metrics *cache.CacheMetrics) {
	if metrics == nil {
		common.CLILogger.Info("📊 Cache Metrics: Unavailable")
		return
	}

	common.CLILogger.Info("📊 Cache Performance Metrics")
	common.CLILogger.Info("%s", strings.Repeat(CacheSubSeparator, CacheSubSepLen))

	// Performance metrics
	totalRequests := metrics.HitCount + metrics.MissCount
	if totalRequests > 0 {
		hitRatio := float64(metrics.HitCount) / float64(totalRequests) * 100

		common.CLILogger.Info("📈 Request Statistics:")
		common.CLILogger.Info("   • Total Requests: %d", totalRequests)
		common.CLILogger.Info("   • Cache Hits: %d (%.1f%%)", metrics.HitCount, hitRatio)
		common.CLILogger.Info("   • Cache Misses: %d (%.1f%%)", metrics.MissCount, 100-hitRatio)

		if metrics.ErrorCount > 0 {
			errorRate := float64(metrics.ErrorCount) / float64(totalRequests) * 100
			common.CLILogger.Info("   • Errors: %d (%.1f%%)", metrics.ErrorCount, errorRate)
		}

		// Performance rating
		performanceIcon := "🎯"
		performanceText := getPerformanceRating(hitRatio)
		common.CLILogger.Info("   %s Performance: %s", performanceIcon, performanceText)
	} else {
		common.CLILogger.Info("📈 Request Statistics: No requests processed yet")
	}

	// Storage metrics
	common.CLILogger.Info("")
	common.CLILogger.Info("💾 Storage Information:")
	common.CLILogger.Info("   • Entries: %d", metrics.EntryCount)
	common.CLILogger.Info("   • Total Size: %s", formatBytes(metrics.TotalSize))

	if metrics.EvictionCount > 0 {
		common.CLILogger.Info("   • Evictions: %d", metrics.EvictionCount)
	}

	// Timing information
	if metrics.AverageHitTime > 0 || metrics.AverageMissTime > 0 {
		common.CLILogger.Info("")
		common.CLILogger.Info("⏱️  Response Times:")
		if metrics.AverageHitTime > 0 {
			common.CLILogger.Info("   • Average Hit Time: %s", metrics.AverageHitTime)
		}
		if metrics.AverageMissTime > 0 {
			common.CLILogger.Info("   • Average Miss Time: %s", metrics.AverageMissTime)
		}
	}
}

// ShowCacheInitStatus displays cache initialization status from LSP manager
func ShowCacheInitStatus(manager *server.LSPManager) {
	if manager == nil {
		common.CLILogger.Info("%s Cache: Manager not initialized", CacheIconError)
		return
	}

	// Try to get cache health to determine if cache is properly initialized
	metrics, err := GetCacheMetricsFromManager(manager)
	if err != nil {
		common.CLILogger.Info("%s Cache Initialization: Failed (%v)", CacheIconError, err)
		return
	}

	if metrics == nil {
		common.CLILogger.Info("%s Cache Initialization: Unknown status", CacheIconUnknown)
		return
	}

	if metrics.HealthStatus == "DISABLED" {
		common.CLILogger.Info("%s Cache Initialization: Disabled by configuration", CacheIconDisabled)
		return
	}

	icon := GetCacheStatusIcon(metrics.HealthStatus)
	common.CLILogger.Info("%s Cache Initialization: Ready (%s)", icon, metrics.HealthStatus)

	// Show basic metrics for initialized cache
	if metrics.EntryCount > 0 || metrics.HitCount > 0 || metrics.MissCount > 0 {
		common.CLILogger.Info("   • %d entries, %d hits, %d misses",
			metrics.EntryCount, metrics.HitCount, metrics.MissCount)
	}
}

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

// IsCacheHealthy determines if cache is in good health
func IsCacheHealthy(metrics *cache.CacheMetrics) bool {
	if metrics == nil {
		return false
	}

	// Disabled cache is considered "healthy" in the sense that it's working as expected
	if metrics.HealthStatus == "DISABLED" {
		return true
	}

	// Only OK status is considered healthy for active caches
	return metrics.HealthStatus == "OK"
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

// DisplayCacheSummary shows a compact cache status summary
func DisplayCacheSummary(metrics *cache.CacheMetrics) {
	if metrics == nil {
		common.CLILogger.Info("🔄 Cache: Unavailable")
		return
	}

	icon := GetCacheStatusIcon(metrics.HealthStatus)

	if metrics.HealthStatus == "DISABLED" {
		common.CLILogger.Info("%s Cache: Disabled", icon)
		return
	}

	totalRequests := metrics.HitCount + metrics.MissCount
	if totalRequests > 0 {
		hitRatio := float64(metrics.HitCount) / float64(totalRequests) * 100
		common.CLILogger.Info("%s Cache: %s (%.1f%% hit rate, %d entries)",
			icon, metrics.HealthStatus, hitRatio, metrics.EntryCount)
	} else {
		common.CLILogger.Info("%s Cache: %s (%d entries, no requests yet)",
			icon, metrics.HealthStatus, metrics.EntryCount)
	}
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

// formatDuration formats duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	} else {
		return fmt.Sprintf("%.1fd", d.Hours()/24)
	}
}

// getPerformanceRating returns a performance rating based on hit ratio
func getPerformanceRating(hitRatio float64) string {
	switch {
	case hitRatio >= 90:
		return "Excellent"
	case hitRatio >= 70:
		return "Good"
	case hitRatio >= 50:
		return "Fair"
	case hitRatio >= 30:
		return "Poor"
	default:
		return "Very Poor"
	}
}

// DisplayCacheHeader shows a formatted header for cache information
func DisplayCacheHeader(title string) {
	common.CLILogger.Info("🔄 %s", title)
	common.CLILogger.Info("%s", strings.Repeat(CacheSeparator, CacheSeparatorLen))
}

// DisplayCacheError shows cache-related errors in a consistent format
func DisplayCacheError(operation string, err error) {
	common.CLILogger.Error("%s Cache %s: %v", CacheIconError, operation, err)
}

// IsCacheEnabled determines if cache is enabled based on metrics
func IsCacheEnabled(metrics *cache.CacheMetrics) bool {
	return metrics != nil && metrics.HealthStatus != "DISABLED"
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

// DisplayCacheHealthFromManager shows cache health directly from LSP manager
func DisplayCacheHealthFromManager(manager *server.LSPManager) {
	if manager == nil {
		common.CLILogger.Info("%s Cache: Manager not available", CacheIconError)
		return
	}

	metrics, err := GetCacheMetricsFromManager(manager)
	if err != nil {
		common.CLILogger.Info("%s Cache: Unable to get metrics (%v)", CacheIconError, err)
		return
	}

	DisplayCacheHealth(metrics)
}

// DisplayCacheMetricsFromManager shows detailed cache metrics directly from LSP manager
func DisplayCacheMetricsFromManager(manager *server.LSPManager) {
	if manager == nil {
		common.CLILogger.Info("📊 Cache Metrics: Manager not available")
		return
	}

	metrics, err := GetCacheMetricsFromManager(manager)
	if err != nil {
		common.CLILogger.Info("📊 Cache Metrics: Unable to get metrics (%v)", err)
		return
	}

	FormatCacheMetrics(metrics)
}

// DisplayCacheSummaryFromManager shows cache summary directly from LSP manager
func DisplayCacheSummaryFromManager(manager *server.LSPManager) {
	if manager == nil {
		common.CLILogger.Info("🔄 Cache: Manager not available")
		return
	}

	metrics, err := GetCacheMetricsFromManager(manager)
	if err != nil {
		common.CLILogger.Info("🔄 Cache: Unable to get metrics (%v)", err)
		return
	}

	DisplayCacheSummary(metrics)
}

// GetCacheMetricsFromInterface safely converts interface{} to CacheMetrics
func GetCacheMetricsFromInterface(metricsInterface interface{}) (*cache.CacheMetrics, error) {
	if metricsInterface == nil {
		return nil, fmt.Errorf("metrics interface is nil")
	}

	metrics, ok := metricsInterface.(*cache.CacheMetrics)
	if !ok {
		return nil, fmt.Errorf("failed to convert interface{} to *cache.CacheMetrics")
	}

	return metrics, nil
}

// DisplayCacheFromInterface shows cache metrics from interface{} type
func DisplayCacheFromInterface(metricsInterface interface{}) {
	metrics, err := GetCacheMetricsFromInterface(metricsInterface)
	if err != nil {
		common.CLILogger.Info("🔄 Cache: Unable to parse metrics (%v)", err)
		return
	}

	DisplayCacheSummary(metrics)
}

// DisplayDetailedCacheFromInterface shows detailed cache metrics from interface{} type
func DisplayDetailedCacheFromInterface(metricsInterface interface{}) {
	metrics, err := GetCacheMetricsFromInterface(metricsInterface)
	if err != nil {
		common.CLILogger.Error("📊 Cache Metrics: Unable to parse metrics (%v)", err)
		return
	}

	FormatCacheMetrics(metrics)
}

// GetCacheHealthIconFromManager returns cache health icon directly from manager
func GetCacheHealthIconFromManager(manager *server.LSPManager) string {
	if manager == nil {
		return CacheIconError
	}

	metrics, err := GetCacheMetricsFromManager(manager)
	if err != nil {
		return CacheIconError
	}

	return GetCacheStatusIcon(metrics.HealthStatus)
}

// IsCacheHealthyFromManager checks if cache is healthy directly from manager
func IsCacheHealthyFromManager(manager *server.LSPManager) bool {
	if manager == nil {
		return false
	}

	metrics, err := GetCacheMetricsFromManager(manager)
	if err != nil {
		return false
	}

	return IsCacheHealthy(metrics)
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

// FormatCacheHealthForStatus returns formatted cache health for status display
func FormatCacheHealthForStatus(manager *server.LSPManager) string {
	if manager == nil {
		return fmt.Sprintf("%s Cache: Unavailable", CacheIconError)
	}

	metrics, err := GetCacheMetricsFromManager(manager)
	if err != nil {
		return fmt.Sprintf("%s Cache: Error (%v)", CacheIconError, err)
	}

	if metrics == nil {
		return fmt.Sprintf("%s Cache: No metrics", CacheIconUnknown)
	}

	icon := GetCacheStatusIcon(metrics.HealthStatus)
	return fmt.Sprintf("%s Cache: %s", icon, metrics.HealthStatus)
}

// GetCachePerformanceRating returns a performance rating based on metrics
func GetCachePerformanceRating(metrics *cache.CacheMetrics) string {
	if metrics == nil {
		return "Unknown"
	}

	totalRequests := metrics.HitCount + metrics.MissCount
	if totalRequests == 0 {
		return "No Activity"
	}

	hitRatio := float64(metrics.HitCount) / float64(totalRequests) * 100
	return getPerformanceRating(hitRatio)
}

// DisplayCacheWarningsAndErrors shows any cache warnings or errors
func DisplayCacheWarningsAndErrors(metrics *cache.CacheMetrics) {
	if metrics == nil {
		return
	}

	hasWarnings := false

	// Check for degraded performance
	if metrics.HealthStatus == "DEGRADED" || metrics.HealthStatus == "FAILING" || metrics.HealthStatus == "CRITICAL" {
		common.CLILogger.Warn("⚠️  Cache health is %s", metrics.HealthStatus)
		hasWarnings = true
	}

	// Check for high error rate
	totalRequests := metrics.HitCount + metrics.MissCount
	if totalRequests > 0 {
		errorRate := float64(metrics.ErrorCount) / float64(totalRequests) * 100
		if errorRate > 10 {
			common.CLILogger.Warn("⚠️  High error rate: %.1f%% (%d/%d requests)", errorRate, metrics.ErrorCount, totalRequests)
			hasWarnings = true
		}
	}

	// Check for excessive evictions
	if metrics.EvictionCount > 100 {
		common.CLILogger.Warn("⚠️  High eviction count: %d (may indicate memory pressure)", metrics.EvictionCount)
		hasWarnings = true
	}

	// Check for very low hit rate
	if totalRequests > 10 {
		hitRatio := float64(metrics.HitCount) / float64(totalRequests) * 100
		if hitRatio < 30 {
			common.CLILogger.Warn("⚠️  Low cache hit rate: %.1f%% (cache may not be effective)", hitRatio)
			hasWarnings = true
		}
	}

	if hasWarnings {
		common.CLILogger.Info("💡 Consider checking cache configuration if performance is suboptimal")
	}
}

// displayCacheStatus is a convenience wrapper for DisplayCacheStatus to match
// the naming convention used in commands.go
func displayCacheStatus(manager *server.LSPManager) {
	DisplayCacheStatus(manager)
}
