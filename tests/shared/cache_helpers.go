package shared

import (
	"context"
	"path/filepath"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server/cache"

	"github.com/stretchr/testify/require"
	"testing"
)

// CacheConfigOptions holds options for creating cache configurations
type CacheConfigOptions struct {
	TempDir         string
	MaxMemoryMB     int
	TTLHours        int
	Languages       []string
	BackgroundIndex bool
	DiskCache       bool
	EvictionPolicy  string
}

// DefaultCacheConfigOptions returns commonly used cache configuration options
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

// CreateCacheConfig creates a cache configuration with the given options
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

// CreateBasicCacheConfig creates a basic cache configuration for simple tests
func CreateBasicCacheConfig(tempDir string) *config.CacheConfig {
	return CreateCacheConfig(DefaultCacheConfigOptions(tempDir))
}

// CreateMemOnlyCacheConfig creates a memory-only cache configuration (no disk)
func CreateMemOnlyCacheConfig(tempDir string) *config.CacheConfig {
	opts := DefaultCacheConfigOptions(tempDir)
	opts.DiskCache = false
	return CreateCacheConfig(opts)
}

// CreateMultiLangCacheConfig creates a cache configuration for multiple languages
func CreateMultiLangCacheConfig(tempDir string) *config.CacheConfig {
	opts := DefaultCacheConfigOptions(tempDir)
	opts.Languages = []string{"go", "python"}
	opts.MaxMemoryMB = 128
	opts.TTLHours = 24
	return CreateCacheConfig(opts)
}

// CreateLargeCacheConfig creates a cache configuration with larger limits
func CreateLargeCacheConfig(tempDir string) *config.CacheConfig {
	opts := DefaultCacheConfigOptions(tempDir)
	opts.MaxMemoryMB = 128
	opts.TTLHours = 24
	return CreateCacheConfig(opts)
}

// StartCacheManager creates and starts a SCIP cache manager with the given configuration
func StartCacheManager(t *testing.T, cacheConfig *config.CacheConfig) *cache.SCIPCacheManager {
	scipCache, err := cache.NewSCIPCacheManager(cacheConfig)
	require.NoError(t, err)
	require.NotNil(t, scipCache)

	ctx := context.Background()
	err = scipCache.Start(ctx)
	require.NoError(t, err)

	return scipCache
}

// CreateAndStartSimpleCache creates and starts a simple cache for testing
func CreateAndStartSimpleCache(t *testing.T, memoryMB int) cache.SCIPCache {
	if memoryMB <= 0 {
		memoryMB = 64
	}

	simpleCache, err := cache.NewSimpleCache(memoryMB)
	require.NoError(t, err)
	require.NotNil(t, simpleCache)

	ctx := context.Background()
	err = simpleCache.Start(ctx)
	require.NoError(t, err)

	return simpleCache
}

// CreateCacheTestData creates sample cache data for testing
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

// CreateHoverTestData creates sample hover data for testing
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

// CreateReferencesTestData creates sample references data for testing
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

// CleanupCache stops and cleans up a cache manager
func CleanupCache(scipCache *cache.SCIPCacheManager) {
	if scipCache != nil {
		scipCache.Stop()
	}
}
