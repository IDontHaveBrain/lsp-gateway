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

// TestOptionalCacheIntegrationVerification verifies the core optional cache functionality
func TestOptionalCacheIntegrationVerification(t *testing.T) {
	t.Run("No cache config - LSP manager works without cache", func(t *testing.T) {
		cfg := &config.Config{
			Servers: map[string]*config.ServerConfig{
				"go": {Command: "gopls", Args: []string{}},
			},
			Cache: nil, // No cache
		}

		manager, err := server.NewLSPManager(cfg)
		require.NoError(t, err, "Should create LSP manager without cache")
		require.NotNil(t, manager, "LSP manager should not be nil")

		cacheInstance := manager.GetCache()
		assert.Nil(t, cacheInstance, "Should have no cache when none configured")
	})

	t.Run("Disabled cache - LSP manager works without cache", func(t *testing.T) {
		cfg := &config.Config{
			Servers: map[string]*config.ServerConfig{
				"go": {Command: "gopls", Args: []string{}},
			},
			Cache: &config.CacheConfig{
				Enabled: false,
			},
		}

		manager, err := server.NewLSPManager(cfg)
		require.NoError(t, err, "Should create LSP manager with disabled cache")
		require.NotNil(t, manager, "LSP manager should not be nil")

		cacheInstance := manager.GetCache()
		assert.Nil(t, cacheInstance, "Should have no cache when disabled")
	})

	t.Run("Enabled cache - LSP manager works with cache", func(t *testing.T) {
		cfg := &config.Config{
			Servers: map[string]*config.ServerConfig{
				"go": {Command: "gopls", Args: []string{}},
			},
			Cache: &config.CacheConfig{
				Enabled:     true,
				MaxMemoryMB: 64,
				TTLHours:    1,
				StoragePath: "/tmp/test-cache",
				Languages:   []string{"go"},
				DiskCache:   false,
			},
		}

		manager, err := server.NewLSPManager(cfg)
		require.NoError(t, err, "Should create LSP manager with enabled cache")
		require.NotNil(t, manager, "LSP manager should not be nil")

		cacheInstance := manager.GetCache()
		assert.NotNil(t, cacheInstance, "Should have cache when enabled")
	})

	t.Run("Simple cache creation and lifecycle", func(t *testing.T) {
		// Test simple cache creation
		simpleCache, err := cache.NewSimpleCache(128)
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
		cfg := &config.Config{
			Servers: map[string]*config.ServerConfig{
				"go": {Command: "gopls", Args: []string{}},
			},
			Cache: nil,
		}

		manager, err := server.NewLSPManager(cfg)
		require.NoError(t, err, "Should create LSP manager")

		// Initially no cache
		cacheInstance := manager.GetCache()
		assert.Nil(t, cacheInstance, "Should have no cache initially")

		// Inject cache
		simpleCache, err := cache.NewSimpleCache(64)
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
