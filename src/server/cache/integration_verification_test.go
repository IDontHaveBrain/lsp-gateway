package cache_test

import (
	"context"
	"testing"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server"
	"lsp-gateway/src/server/cache"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOptionalCacheIntegrationVerification verifies the core optional cache functionality
func TestOptionalCacheIntegrationVerification(t *testing.T) {
	t.Run("No cache config - LSP manager works without cache", func(t *testing.T) {
		cfg := config.NewTestConfig(
			config.WithLanguages("go"),
			config.WithCacheEnabled(false),
		)
		cfg.Cache = nil

		manager, err := server.NewLSPManager(cfg)
		require.NoError(t, err, "Should create LSP manager without cache")
		require.NotNil(t, manager, "LSP manager should not be nil")

		cacheInstance := manager.GetCache()
		assert.Nil(t, cacheInstance, "Should have no cache when none configured")
	})

	t.Run("Disabled cache - LSP manager works without cache", func(t *testing.T) {
		cfg := config.NewTestConfig(
			config.WithLanguages("go"),
			config.WithCacheEnabled(false),
			config.WithCachePath("/tmp/test-cache"),
			config.WithCacheMemory(64),
			config.WithCacheTTL(1),
		)

		manager, err := server.NewLSPManager(cfg)
		require.NoError(t, err, "Should create LSP manager with disabled cache")
		require.NotNil(t, manager, "LSP manager should not be nil")

		cacheInstance := manager.GetCache()
		assert.Nil(t, cacheInstance, "Should have no cache when disabled")
	})

	t.Run("Enabled cache - LSP manager works with cache", func(t *testing.T) {
		cfg := config.BasicGoTestConfig(t.TempDir())

		manager, err := server.NewLSPManager(cfg)
		require.NoError(t, err, "Should create LSP manager with enabled cache")
		require.NotNil(t, manager, "LSP manager should not be nil")

		cacheInstance := manager.GetCache()
		assert.NotNil(t, cacheInstance, "Should have cache when enabled")
	})

	t.Run("Simple cache creation and lifecycle", func(t *testing.T) {
		// Test simple cache creation
		cfg := config.NewTestConfig(
			config.WithCachePath(t.TempDir()),
		)
		simpleCache, err := cache.NewSCIPCacheManager(cfg.Cache)
		require.NoError(t, err, "Should create simple cache")
		require.NotNil(t, simpleCache, "Simple cache should not be nil")

		// Test lifecycle
		ctx := context.Background()
		err = simpleCache.Start(ctx)
		assert.NoError(t, err, "Should start simple cache")

		err = simpleCache.Stop()
		assert.NoError(t, err, "Should stop simple cache")
	})

	t.Run("Optional cache injection", func(t *testing.T) {
		// Create manager without cache
		cfg := config.NewTestConfig(
			config.WithLanguages("go"),
			config.WithCacheEnabled(false),
		)
		cfg.Cache = nil

		manager, err := server.NewLSPManager(cfg)
		require.NoError(t, err, "Should create LSP manager")

		// Initially no cache
		cacheInstance := manager.GetCache()
		assert.Nil(t, cacheInstance, "Should have no cache initially")

		// Inject cache
		cacheCfg := config.NewTestConfig(
			config.WithCachePath(t.TempDir()),
			config.WithCacheMemory(64),
			config.WithCacheTTL(1),
		)
		simpleCache, err := cache.NewSCIPCacheManager(cacheCfg.Cache)
		require.NoError(t, err, "Should create simple cache for injection")

		manager.SetCache(simpleCache)
		injectedCache := manager.GetCache()
		assert.NotNil(t, injectedCache, "Should have cache after injection")

		// Remove cache
		manager.SetCache(nil)
		removedCache := manager.GetCache()
		assert.Nil(t, removedCache, "Should have no cache after removal")
	})
}
