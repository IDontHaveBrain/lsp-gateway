package cache

import (
	"runtime"
	"strings"

	"lsp-gateway/src/internal/errors"
	"lsp-gateway/src/server/documents"
)

// ExtractURIFromParams extracts the URI from LSP method parameters
func ExtractURIFromParams(method string, params interface{}) (string, error) {
	if params == nil {
		return "", errors.NewValidationError("params", "no parameters provided")
	}
	dm := documents.NewDocumentManager()
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

// computeWorkers returns a bounded worker count with Java/Windows handling
func computeWorkers(hasJava bool) int {
	workers := runtime.NumCPU()
	if hasJava && runtime.GOOS == "windows" {
		return 1
	}
	if hasJava {
		if workers > 2 {
			return 2
		}
		if workers < 1 {
			return 1
		}
		return workers
	}
	if workers < 2 {
		workers = 2
	}
	if workers > 16 {
		workers = 16
	}
	return workers
}

func hasJavaInLangs(languages []string) bool {
	for _, lang := range languages {
		if lang == "java" {
			return true
		}
	}
	return false
}

func hasJavaInFiles(files []string) bool {
	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(f), ".java") {
			return true
		}
	}
	return false
}

func hasJavaInURIs(uris []string) bool {
	for _, u := range uris {
		if strings.HasSuffix(strings.ToLower(u), ".java") {
			return true
		}
	}
	return false
}
