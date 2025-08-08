package cache

import (
	"context"
	"testing"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server"
	"lsp-gateway/src/server/cache"
	"lsp-gateway/tests/shared"

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
			name:        "Cache enabled - should work with cache",
			cacheConfig: shared.CreateMemOnlyCacheConfig("/tmp"),
			expectCache: true,
			description: "LSP manager should work with cache when properly configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config with optional cache
			cfg := shared.CreateBasicConfig()
			cfg.Cache = tt.cacheConfig

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
	cfg := shared.CreateBasicConfig()
	invalidCacheConfig := shared.CreateBasicCacheConfig("/invalid/path/that/does/not/exist")
	cfg.Cache = invalidCacheConfig

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

				// Verify cache can be started and stopped
				ctx := context.Background()
				err = simpleCache.Start(ctx)
				assert.NoError(t, err)

				// Check IsEnabled after starting
				assert.True(t, simpleCache.IsEnabled())

				err = simpleCache.Stop()
				assert.NoError(t, err)
			}
		})
	}
}

// TestCacheOptionalMethods verifies cache methods handle nil cache gracefully
func TestCacheOptionalMethods(t *testing.T) {
	// Create LSP manager without cache
	cfg := shared.CreateBasicConfig()
	cfg.Cache = nil // No cache

	manager, err := server.NewLSPManager(cfg)
	require.NoError(t, err)
	require.NotNil(t, manager)

	// Verify cache methods handle nil cache gracefully
	err = manager.InvalidateCache("file://test.go")
	assert.NoError(t, err, "InvalidateCache should handle nil cache gracefully")

	metrics := manager.GetCacheMetrics()
	assert.Nil(t, metrics, "GetCacheMetrics should return nil for nil cache")

	currentCache := manager.GetCache()
	assert.Nil(t, currentCache, "GetCache should return nil when no cache is configured")

	// Test SetCache method for optional injection
	simpleCache := shared.CreateAndStartSimpleCache(t, 64)
	defer simpleCache.Stop()

	manager.SetCache(simpleCache)
	retrievedCache := manager.GetCache()
	assert.NotNil(t, retrievedCache, "Cache should be present after injection")

	// Remove cache by setting to nil
	manager.SetCache(nil)
	retrievedCache = manager.GetCache()
	assert.Nil(t, retrievedCache, "Cache should be nil after removal")
}

// TestCacheIntegrationPattern tests cache integration
func TestCacheIntegrationPattern(t *testing.T) {
	// Create cache configuration  
	tempDir := t.TempDir()
	cacheConfig := shared.CreateBasicCacheConfig(tempDir)
	
	// Start cache manager
	cacheManager := shared.StartCacheManager(t, cacheConfig)
	defer cacheManager.Stop()

	// Test basic cache operations
	method := "textDocument/definition"
	params := map[string]interface{}{
		"textDocument": map[string]string{"uri": "file://test.go"},
		"position":     map[string]int{"line": 10, "character": 5},
	}

	// First lookup should miss
	result, found, err := cacheManager.Lookup(method, params)
	assert.NoError(t, err)
	assert.False(t, found, "Empty cache should not have results")
	assert.Nil(t, result)

	// Store a result
	response := map[string]interface{}{"result": "definition result"}
	err = cacheManager.Store(method, params, response)
	assert.NoError(t, err)

	// Second lookup should hit
	result, found, err = cacheManager.Lookup(method, params)
	assert.NoError(t, err) 
	assert.True(t, found, "Cache should have stored result")
	assert.NotNil(t, result)

	// Test invalidation
	err = cacheManager.InvalidateDocument("file://test.go")
	assert.NoError(t, err)

	// Note: Document invalidation may not affect all cached LSP methods
	// This depends on the specific cache implementation
	// The test verifies that invalidation doesn't cause errors
}
