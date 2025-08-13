package testutils

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server/cache"

	"github.com/stretchr/testify/require"
)

// CacheTestSetup provides comprehensive cache setup with automatic cleanup
type CacheTestSetup struct {
	Manager *cache.SCIPCacheManager
	Config  *config.CacheConfig
	TempDir string
	t       *testing.T
}

// Close performs cleanup and stops the cache manager
func (setup *CacheTestSetup) Close() {
	if setup.Manager != nil {
		setup.Manager.Stop()
	}
}

// CacheConfigBuilder provides fluent interface for building cache configurations
type CacheConfigBuilder struct {
	config *config.CacheConfig
}

// NewCacheConfigBuilder creates a new builder with sensible defaults
func NewCacheConfigBuilder(tempDir string) *CacheConfigBuilder {
	cacheDir := filepath.Join(tempDir, "test-cache")
	return &CacheConfigBuilder{
		config: &config.CacheConfig{
			Enabled:            true,
			StoragePath:        cacheDir,
			MaxMemoryMB:        64,
			TTLHours:           1,
			Languages:          []string{"go"},
			BackgroundIndex:    false,
			DiskCache:          true,
			EvictionPolicy:     "lru",
			HealthCheckMinutes: 2,
		},
	}
}

// WithMemoryLimit sets the memory limit in MB
func (b *CacheConfigBuilder) WithMemoryLimit(memoryMB int) *CacheConfigBuilder {
	b.config.MaxMemoryMB = memoryMB
	return b
}

// WithTTL sets the cache TTL in hours
func (b *CacheConfigBuilder) WithTTL(hours int) *CacheConfigBuilder {
	b.config.TTLHours = hours
	return b
}

// WithLanguages sets the supported languages
func (b *CacheConfigBuilder) WithLanguages(languages ...string) *CacheConfigBuilder {
	b.config.Languages = languages
	return b
}

// WithDiskCache enables or disables disk persistence
func (b *CacheConfigBuilder) WithDiskCache(enabled bool) *CacheConfigBuilder {
	b.config.DiskCache = enabled
	return b
}

// WithBackgroundIndex enables or disables background indexing
func (b *CacheConfigBuilder) WithBackgroundIndex(enabled bool) *CacheConfigBuilder {
	b.config.BackgroundIndex = enabled
	return b
}

// WithEvictionPolicy sets the eviction policy
func (b *CacheConfigBuilder) WithEvictionPolicy(policy string) *CacheConfigBuilder {
	b.config.EvictionPolicy = policy
	return b
}

// WithHealthCheckInterval sets health check interval in minutes
func (b *CacheConfigBuilder) WithHealthCheckInterval(minutes int) *CacheConfigBuilder {
	b.config.HealthCheckMinutes = minutes
	return b
}

// WithStoragePath sets custom storage path
func (b *CacheConfigBuilder) WithStoragePath(path string) *CacheConfigBuilder {
	b.config.StoragePath = path
	return b
}

// Build returns the constructed configuration
func (b *CacheConfigBuilder) Build() *config.CacheConfig {
	return b.config
}

// CreateTestCacheConfig creates a basic cache configuration for testing
func CreateTestCacheConfig(storageDir string, memoryMB int) *config.CacheConfig {
	return NewCacheConfigBuilder(storageDir).
		WithMemoryLimit(memoryMB).
		Build()
}

// CreateAndStartCacheManager creates and starts a cache manager with lifecycle management
func CreateAndStartCacheManager(t *testing.T, cfg *config.CacheConfig) *CacheTestSetup {
	scipCache, err := cache.NewSCIPCacheManager(cfg)
	require.NoError(t, err)
	require.NotNil(t, scipCache)

	ctx := context.Background()
	err = scipCache.Start(ctx)
	require.NoError(t, err)

	setup := &CacheTestSetup{
		Manager: scipCache,
		Config:  cfg,
		TempDir: cfg.StoragePath,
		t:       t,
	}

	// Register cleanup
	t.Cleanup(func() {
		setup.Close()
	})

	return setup
}

// DefaultCacheConfig returns default cache configuration for tests
func DefaultCacheConfig() *config.CacheConfig {
	return &config.CacheConfig{
		Enabled:            true,
		MaxMemoryMB:        64,
		TTLHours:           1,
		Languages:          []string{"go"},
		BackgroundIndex:    false,
		DiskCache:          true,
		EvictionPolicy:     "lru",
		HealthCheckMinutes: 2,
	}
}

// CreateMemOnlyCacheConfig creates memory-only cache configuration for fast tests
func CreateMemOnlyCacheConfig() *config.CacheConfig {
	return &config.CacheConfig{
		Enabled:            true,
		MaxMemoryMB:        32,
		TTLHours:           1,
		Languages:          []string{"go"},
		BackgroundIndex:    false,
		DiskCache:          false,
		EvictionPolicy:     "lru",
		HealthCheckMinutes: 1,
	}
}

// CreateMultiLangCacheConfig creates cache configuration for multiple languages
func CreateMultiLangCacheConfig() *config.CacheConfig {
	return NewCacheConfigBuilder("").
		WithLanguages("go", "python", "typescript", "javascript").
		WithMemoryLimit(128).
		WithTTL(24).
		WithBackgroundIndex(true).
		Build()
}

// CreateLargeCacheConfig creates cache configuration with larger limits for performance tests
func CreateLargeCacheConfig() *config.CacheConfig {
	return NewCacheConfigBuilder("").
		WithMemoryLimit(256).
		WithTTL(48).
		WithLanguages("go", "python", "typescript", "javascript", "java", "rust").
		WithBackgroundIndex(true).
		WithHealthCheckInterval(5).
		Build()
}

// StartCacheManager creates and starts a cache manager with the given configuration
func StartCacheManager(t *testing.T, cfg *config.CacheConfig) *cache.SCIPCacheManager {
	scipCache, err := cache.NewSCIPCacheManager(cfg)
	require.NoError(t, err)
	require.NotNil(t, scipCache)

	ctx := context.Background()
	err = scipCache.Start(ctx)
	require.NoError(t, err)

	// Register cleanup
	t.Cleanup(func() {
		scipCache.Stop()
	})

	return scipCache
}

// CreateCacheTestData creates comprehensive sample test data for cache operations
func CreateCacheTestData() (string, map[string]interface{}, interface{}) {
	method := "textDocument/definition"
	params := map[string]interface{}{
		"textDocument": map[string]string{"uri": "file:///test.go"},
		"position":     map[string]int{"line": 10, "character": 5},
	}
	response := []interface{}{
		map[string]interface{}{
			"uri": "file:///test.go",
			"range": map[string]interface{}{
				"start": map[string]int{"line": 10, "character": 5},
				"end":   map[string]int{"line": 10, "character": 15},
			},
		},
	}
	return method, params, response
}

// WaitForCacheReady waits for cache to be ready with timeout
func WaitForCacheManagerReady(t *testing.T, cacheManager cache.SCIPCache, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Fatal("Timeout waiting for cache to be ready")
		case <-ticker.C:
			if metrics, err := cacheManager.HealthCheck(); err == nil && metrics != nil {
				return // Cache is ready
			}
		}
	}
}

// ClearCacheAndWait clears cache and waits for operation to complete
func ClearCacheAndWait(t *testing.T, cacheManager cache.SCIPCache) {
	err := cacheManager.Clear()
	require.NoError(t, err)

	// Wait for clear to take effect
	time.Sleep(50 * time.Millisecond)

	// Verify cache is empty
	metrics := cacheManager.GetMetrics()
	require.NotNil(t, metrics)
	require.Equal(t, int64(0), metrics.EntryCount, "Cache should be empty after clear")
}

// ValidateCacheMetrics validates cache metrics for basic consistency
func ValidateCacheMetrics(t *testing.T, cacheManager cache.SCIPCache) {
	metrics := cacheManager.GetMetrics()
	require.NotNil(t, metrics, "Cache metrics should not be nil")

	// Basic sanity checks
	require.GreaterOrEqual(t, metrics.HitCount, int64(0), "Hit count should be non-negative")
	require.GreaterOrEqual(t, metrics.MissCount, int64(0), "Miss count should be non-negative")
	require.GreaterOrEqual(t, metrics.ErrorCount, int64(0), "Error count should be non-negative")
	require.GreaterOrEqual(t, metrics.EntryCount, int64(0), "Entry count should be non-negative")
	require.GreaterOrEqual(t, metrics.TotalSize, int64(0), "Total size should be non-negative")

	// Health check should work
	healthMetrics, err := cacheManager.HealthCheck()
	require.NoError(t, err, "Health check should not error")
	require.NotNil(t, healthMetrics, "Health metrics should not be nil")
}

// IndexTestFiles indexes a list of test files in the cache
func IndexTestFiles(t *testing.T, cacheManager cache.SCIPCache, files []string) {
	ctx := context.Background()

	err := cacheManager.UpdateIndex(ctx, files)
	require.NoError(t, err, "Failed to index test files")

	// Wait for indexing to complete
	time.Sleep(200 * time.Millisecond)

	// Verify index stats show some progress
	stats := cacheManager.GetIndexStats()
	require.NotNil(t, stats, "Index stats should be available after indexing")
}

// StoreTestCacheEntries stores multiple test entries in cache for testing
func StoreTestCacheEntries(t *testing.T, cacheManager cache.SCIPCache, count int) {
	for i := 0; i < count; i++ {
		method := "textDocument/hover"
		params := map[string]interface{}{
			"textDocument": map[string]string{"uri": fmt.Sprintf("file:///test%d.go", i)},
			"position":     map[string]int{"line": i, "character": 5},
		}
		response := map[string]interface{}{
			"contents": fmt.Sprintf("Test hover content %d", i),
		}

		err := cacheManager.Store(method, params, response)
		require.NoError(t, err, "Failed to store test cache entry %d", i)
	}
}

// VerifyIndexedContent verifies that content was properly indexed
func VerifyIndexedContent(t *testing.T, cacheManager cache.SCIPCache, symbolName string) {
	ctx := context.Background()

	query := &cache.IndexQuery{
		Type:   "symbol",
		Symbol: symbolName,
	}

	result, err := cacheManager.QueryIndex(ctx, query)
	require.NoError(t, err, "Failed to query index for symbol %s", symbolName)
	require.NotNil(t, result, "Query result should not be nil")
	require.NotEmpty(t, result.Results, "Should find results for symbol %s", symbolName)
}

// WaitForIndexingComplete waits for background indexing to complete
func WaitForIndexingComplete(t *testing.T, cacheManager cache.SCIPCache, maxWait time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), maxWait)
	defer cancel()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	initialStats := cacheManager.GetIndexStats()
	if initialStats == nil {
		return // No indexing in progress
	}

	for {
		select {
		case <-ctx.Done():
			t.Log("Warning: Timeout waiting for indexing to complete")
			return
		case <-ticker.C:
			stats := cacheManager.GetIndexStats()
			if stats != nil && stats.Status == "ready" {
				return
			}
		}
	}
}

// CreateCacheConfigForLanguage creates optimized cache config for specific language
func CreateCacheConfigForLanguage(language string, tempDir string) *config.CacheConfig {
	builder := NewCacheConfigBuilder(tempDir).WithLanguages(language)

	// Language-specific optimizations
	switch language {
	case "java":
		return builder.WithMemoryLimit(128).WithTTL(24).WithBackgroundIndex(true).Build()
	case "python":
		return builder.WithMemoryLimit(96).WithTTL(12).WithBackgroundIndex(true).Build()
	case "typescript", "javascript":
		return builder.WithMemoryLimit(80).WithTTL(6).WithBackgroundIndex(false).Build()
	case "rust":
		return builder.WithMemoryLimit(112).WithTTL(18).WithBackgroundIndex(true).Build()
	default:
		return builder.Build()
	}
}

// CreateFastTestCacheConfig creates minimal cache config for fast unit tests
func CreateFastTestCacheConfig() *config.CacheConfig {
	return &config.CacheConfig{
		Enabled:            true,
		MaxMemoryMB:        16,
		TTLHours:           1,
		Languages:          []string{"go"},
		BackgroundIndex:    false,
		DiskCache:          false,
		EvictionPolicy:     "simple",
		HealthCheckMinutes: 1,
	}
}
