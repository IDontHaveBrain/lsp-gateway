package config_test

import (
	"testing"

	"lsp-gateway/src/config"
)

func TestGetDefaultConfig_CacheEnabledByDefault(t *testing.T) {
	cfg := config.GetDefaultConfig()

	// Verify cache is not nil
	if cfg.Cache == nil {
		t.Fatal("Expected cache to be included by default, but it was nil")
	}

	// Verify cache is enabled by default
	if !cfg.Cache.Enabled {
		t.Error("Expected cache to be enabled by default, but it was disabled")
	}

	// Verify default cache settings
	expectedStoragePath := ".lsp-gateway/scip-cache"
	if !containsPath(cfg.Cache.StoragePath, expectedStoragePath) {
		t.Errorf("Expected storage path to contain %s, got %s", expectedStoragePath, cfg.Cache.StoragePath)
	}

	if cfg.Cache.MaxMemoryMB != 512 {
		t.Errorf("Expected max memory to be 512MB, got %d", cfg.Cache.MaxMemoryMB)
	}

	if cfg.Cache.TTLHours != 24 {
		t.Errorf("Expected TTL to be 24 hours, got %d", cfg.Cache.TTLHours)
	}

	if !cfg.Cache.BackgroundIndex {
		t.Error("Expected background index to be enabled by default")
	}

	if cfg.Cache.HealthCheckMinutes != 5 {
		t.Errorf("Expected health check interval to be 5 minutes, got %d", cfg.Cache.HealthCheckMinutes)
	}
}

func TestGetDefaultCacheConfig_EnabledByDefault(t *testing.T) {
	cacheConfig := config.GetDefaultCacheConfig()

	if !cacheConfig.Enabled {
		t.Error("Expected cache config to be enabled by default")
	}

	// Check languages is set to wildcard for all supported languages
	if len(cacheConfig.Languages) != 1 || cacheConfig.Languages[0] != "*" {
		t.Errorf("Expected languages to be [\"*\"], got %v", cacheConfig.Languages)
	}
}

func TestConfig_IsCacheEnabled(t *testing.T) {
	cfg := config.GetDefaultConfig()

	// Should be enabled by default now
	if !cfg.IsCacheEnabled() {
		t.Error("Expected IsCacheEnabled() to return true for default config")
	}

	// Test with disabled cache
	cfg.Cache.Enabled = false
	if cfg.IsCacheEnabled() {
		t.Error("Expected IsCacheEnabled() to return false when cache is disabled")
	}

	// Test with nil cache
	cfg.Cache = nil
	if cfg.IsCacheEnabled() {
		t.Error("Expected IsCacheEnabled() to return false when cache is nil")
	}
}

func TestConfig_HasCache(t *testing.T) {
	cfg := config.GetDefaultConfig()

	// Should have cache by default now
	if !cfg.HasCache() {
		t.Error("Expected HasCache() to return true for default config")
	}

	// Test with nil cache
	cfg.Cache = nil
	if cfg.HasCache() {
		t.Error("Expected HasCache() to return false when cache is nil")
	}
}

func TestGetDefaultConfigWithCache_ReturnsDefaultConfig(t *testing.T) {
	defaultConfig := config.GetDefaultConfig()
	cacheConfig := config.GetDefaultConfigWithCache()

	// Since GetDefaultConfig() now includes cache, these should be equivalent
	if defaultConfig.Cache == nil || cacheConfig.Cache == nil {
		t.Fatal("Both configs should have cache")
	}

	if defaultConfig.Cache.Enabled != cacheConfig.Cache.Enabled {
		t.Error("Expected both configs to have same cache enabled state")
	}
}

// Helper function to check if a path contains the expected substring
func containsPath(actualPath, expectedSubstring string) bool {
	return len(actualPath) > 0 && (actualPath == expectedSubstring ||
		len(actualPath) > len(expectedSubstring) &&
			actualPath[len(actualPath)-len(expectedSubstring):] == expectedSubstring)
}
