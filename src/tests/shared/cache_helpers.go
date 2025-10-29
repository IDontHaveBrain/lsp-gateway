package shared

import (
	"path/filepath"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server/cache"
)

type CacheConfigOptions struct {
	TempDir         string
	MaxMemoryMB     int
	TTLHours        int
	Languages       []string
	BackgroundIndex bool
	DiskCache       bool
	EvictionPolicy  string
}

func DefaultCacheConfigOptions(tempDir string) *CacheConfigOptions {
	return &CacheConfigOptions{
		TempDir:         tempDir,
		MaxMemoryMB:     64,
		TTLHours:        1,
		Languages:       []string{"go"},
		BackgroundIndex: false,
		DiskCache:       true,
		EvictionPolicy:  "lru",
	}
}

func CreateCacheConfig(opts *CacheConfigOptions) *config.CacheConfig {
	if opts == nil {
		opts = DefaultCacheConfigOptions("")
	}

	cacheDir := filepath.Join(opts.TempDir, "test-cache")
	return &config.CacheConfig{
		Enabled:         true,
		StoragePath:     cacheDir,
		MaxMemoryMB:     opts.MaxMemoryMB,
		TTLHours:        opts.TTLHours,
		Languages:       opts.Languages,
		BackgroundIndex: opts.BackgroundIndex,
		DiskCache:       opts.DiskCache,
		EvictionPolicy:  opts.EvictionPolicy,
	}
}

func CreateMultiLangCacheConfig(tempDir string) *config.CacheConfig {
	opts := DefaultCacheConfigOptions(tempDir)
	opts.Languages = []string{"go", "python"}
	opts.MaxMemoryMB = 128
	opts.TTLHours = 24
	return CreateCacheConfig(opts)
}

func CreateLargeCacheConfig(tempDir string) *config.CacheConfig {
	opts := DefaultCacheConfigOptions(tempDir)
	opts.MaxMemoryMB = 128
	opts.TTLHours = 24
	return CreateCacheConfig(opts)
}

func CreateCacheTestData() (string, map[string]interface{}, interface{}) {
	method := "textDocument/definition"
	params := map[string]interface{}{
		"textDocument": map[string]string{"uri": "file:///test.go"},
		"position":     map[string]int{"line": 10, "character": 5},
	}
	response := map[string]interface{}{
		"uri":   "file:///test.go",
		"range": map[string]interface{}{"start": map[string]int{"line": 10, "character": 5}},
	}
	return method, params, response
}

func CreateHoverTestData() (string, map[string]interface{}, interface{}) {
	method := "textDocument/hover"
	params := map[string]interface{}{
		"textDocument": map[string]string{"uri": "file:///test.go"},
		"position":     map[string]int{"line": 5, "character": 10},
	}
	response := map[string]interface{}{
		"contents": "Test hover content",
	}
	return method, params, response
}

func CreateReferencesTestData() (string, map[string]interface{}, interface{}) {
	method := "textDocument/references"
	params := map[string]interface{}{
		"textDocument": map[string]string{"uri": "file:///test.go"},
		"position":     map[string]int{"line": 5, "character": 10},
		"context":      map[string]bool{"includeDeclaration": true},
	}
	response := []interface{}{
		map[string]interface{}{"uri": "file:///ref1.go"},
		map[string]interface{}{"uri": "file:///ref2.go"},
	}
	return method, params, response
}

func CleanupCache(scipCache cache.SCIPCache) {
	if scipCache != nil {
		scipCache.Stop()
	}
}
