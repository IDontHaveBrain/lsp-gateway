package cache

import (
    "lsp-gateway/src/internal/common"
    "lsp-gateway/src/server/documents"
)

// ExtractURIFromParams extracts the URI from LSP method parameters
// This provides centralized URI extraction logic for cache operations
func ExtractURIFromParams(method string, params interface{}) (string, error) {
    if params == nil {
        return "", common.NoParametersError()
    }
    dm := documents.NewLSPDocumentManager()
    return dm.ExtractURI(params)
}

// HitRate calculates the cache hit rate percentage from metrics
func HitRate(m *CacheMetrics) float64 {
	if m == nil {
		return 0
	}
	total := m.HitCount + m.MissCount
	if total == 0 {
		return 0
	}
	return float64(m.HitCount) / float64(total) * 100
}
