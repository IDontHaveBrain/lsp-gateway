package cli

import (
	"fmt"
	"strings"

	"lsp-gateway/src/server"
	"lsp-gateway/src/server/cache"
)

// Cache display constants
const (
	CacheSeparator    = "="
	CacheSubSeparator = "-"
	CacheSeparatorLen = 50
	CacheSubSepLen    = 30
)

// GetHealthStatusText returns appropriate status text based on health status
func GetHealthStatusText(health string) string {
	switch strings.ToUpper(health) {
	case "OK":
		return "Healthy"
	case "DEGRADED":
		return "Warning"
	case "FAILING", "CRITICAL":
		return "Error"
	case "DISABLED":
		return "Disabled"
	default:
		return "Unknown"
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

// GetCacheStatusText returns a human-readable cache status description
func GetCacheStatusText(metrics *cache.CacheMetrics) string {
	if metrics == nil {
		return "Unavailable"
	}

	return "Operating normally"
}
