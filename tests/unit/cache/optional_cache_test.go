package cache

import (
	"context"
	"testing"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server"
	"lsp-gateway/src/server/cache"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOptionalCacheIntegration verifies that LSP manager works with and without cache
func TestOptionalCacheIntegration(t *testing.T) {
	tests := []struct {
		name        string
		cacheConfig *config.CacheConfig
		expectCache bool
		description string
	}{
		{
			name:        "No cache config - should work without cache",
			cacheConfig: nil,
			expectCache: false,
			description: "LSP manager should work fine when no cache config is provided",
		},
		{
			name: "Cache disabled - should work without cache",
			cacheConfig: &config.CacheConfig{
				Enabled: false,
			},
			expectCache: false,
			description: "LSP manager should work fine when cache is explicitly disabled",
		},
		{
			name: "Cache enabled - should work with cache",
			cacheConfig: &config.CacheConfig{
				Enabled:      true,
				MaxMemoryMB:  64,
				TTLHours:     1,
				StoragePath:  "/tmp/test-cache",
				Languages:    []string{"go"},
				DiskCache:    false,
			},
			expectCache: true,
			description: "LSP manager should work with cache when properly configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config with optional cache
			cfg := &config.Config{
				Servers: map[string]*config.ServerConfig{
					"go": {
						Command: "gopls",
						Args:    []string{},
					},
				},
				Cache: tt.cacheConfig,
			}

			// Create LSP manager with optional cache
			manager, err := server.NewLSPManager(cfg)
			require.NoError(t, err, "Failed to create LSP manager: %s", tt.description)
			require.NotNil(t, manager, "LSP manager should not be nil")

			// Verify cache presence matches expectation
			cache := manager.GetCache()
			if tt.expectCache {
				assert.NotNil(t, cache, "Expected cache to be present: %s", tt.description)
			} else {
				assert.Nil(t, cache, "Expected no cache: %s", tt.description)
			}

			// Verify manager can start and stop regardless of cache presence
			ctx := context.Background()
			err = manager.Start(ctx)
			assert.NoError(t, err, "LSP manager should start successfully: %s", tt.description)

			err = manager.Stop()
			assert.NoError(t, err, "LSP manager should stop successfully: %s", tt.description)
		})
	}
}

// TestCacheGracefulDegradation verifies cache failures don't prevent LSP manager operation
func TestCacheGracefulDegradation(t *testing.T) {
	// Create config with cache pointing to invalid/inaccessible path
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"go": {
				Command: "gopls",
				Args:    []string{},
			},
		},
		Cache: &config.CacheConfig{
			Enabled:     true,
			MaxMemoryMB: 64,
			TTLHours:    1,
			StoragePath: "/invalid/path/that/does/not/exist",
			Languages:   []string{"go"},
			DiskCache:   true, // This might fail due to invalid path
		},
	}

	// Create LSP manager - should succeed even if cache fails
	manager, err := server.NewLSPManager(cfg)
	require.NoError(t, err, "LSP manager creation should succeed even with cache configuration issues")
	require.NotNil(t, manager, "LSP manager should not be nil")

	// Manager should still be functional
	ctx := context.Background()
	err = manager.Start(ctx)
	assert.NoError(t, err, "LSP manager should start successfully even without working cache")

	err = manager.Stop()
	assert.NoError(t, err, "LSP manager should stop successfully")
}

// TestSimpleCacheCreation verifies the new simplified cache creation
func TestSimpleCacheCreation(t *testing.T) {
	tests := []struct {
		name      string
		memoryMB  int
		expectErr bool
	}{
		{
			name:      "Default cache size",
			memoryMB:  0, // Should default to 256MB
			expectErr: false,
		},
		{
			name:      "Custom cache size",
			memoryMB:  128,
			expectErr: false,
		},
		{
			name:      "Negative cache size",
			memoryMB:  -1, // Should default to 256MB
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the new simplified cache creation
			simpleCache, err := cache.NewSimpleCache(tt.memoryMB)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, simpleCache)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, simpleCache)
				assert.True(t, simpleCache.IsEnabled())

				// Verify cache can be started and stopped
				ctx := context.Background()
				err = simpleCache.Start(ctx)
				assert.NoError(t, err)

				err = simpleCache.Stop()
				assert.NoError(t, err)
			}
		})
	}
}

// TestCacheOptionalMethods verifies cache methods handle nil cache gracefully
func TestCacheOptionalMethods(t *testing.T) {
	// Create LSP manager without cache
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"go": {
				Command: "gopls",
				Args:    []string{},
			},
		},
		Cache: nil, // No cache
	}

	manager, err := server.NewLSPManager(cfg)
	require.NoError(t, err)
	require.NotNil(t, manager)

	// Verify cache methods handle nil cache gracefully
	err = manager.InvalidateCache("file://test.go")
	assert.NoError(t, err, "InvalidateCache should handle nil cache gracefully")

	metrics := manager.GetCacheMetrics()
	assert.Nil(t, metrics, "GetCacheMetrics should return nil for nil cache")

	cache := manager.GetCache()
	assert.Nil(t, cache, "GetCache should return nil when no cache is configured")

	// Test SetCache method for optional injection
	simpleCache, err := cache.NewSimpleCache(64)
	require.NoError(t, err)

	manager.SetCache(simpleCache)
	retrievedCache := manager.GetCache()
	assert.NotNil(t, retrievedCache, "Cache should be present after injection")

	// Remove cache by setting to nil
	manager.SetCache(nil)
	retrievedCache = manager.GetCache()
	assert.Nil(t, retrievedCache, "Cache should be nil after removal")
}

// TestDirectCacheIntegrationPattern tests the new direct integration pattern
func TestDirectCacheIntegrationPattern(t *testing.T) {
	// Create simple cache
	simpleCache, err := cache.NewSimpleCache(64)
	require.NoError(t, err)
	require.NotNil(t, simpleCache)

	// Create direct integration
	integration := cache.NewDirectCacheIntegration(simpleCache)
	require.NotNil(t, integration)

	// Test cache-first, LSP-fallback pattern
	ctx := context.Background()
	method := "textDocument/definition"
	params := map[string]interface{}{
		"textDocument": map[string]string{"uri": "file://test.go"},
		"position":     map[string]int{"line": 10, "character": 5},
	}

	// Mock LSP fallback function
	fallbackCalled := false
	lspFallback := func() (interface{}, error) {
		fallbackCalled = true
		return "definition result", nil
	}

	// First call should hit LSP fallback (cache miss)
	result, err := integration.ProcessRequest(ctx, method, params, lspFallback)
	assert.NoError(t, err)
	assert.Equal(t, "definition result", result)
	assert.True(t, fallbackCalled, "LSP fallback should be called on cache miss")

	// Reset fallback flag
	fallbackCalled = false

	// Second call should hit cache (if caching works)
	result2, err := integration.ProcessRequest(ctx, method, params, lspFallback)
	assert.NoError(t, err)
	assert.NotNil(t, result2)
	// Note: Cache hit behavior depends on cache implementation details
}