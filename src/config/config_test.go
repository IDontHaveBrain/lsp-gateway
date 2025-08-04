package config

import (
	"testing"
	"time"
)

func TestGetDefaultConfig_CacheEnabledByDefault(t *testing.T) {
	config := GetDefaultConfig()

	// Verify cache is not nil
	if config.Cache == nil {
		t.Fatal("Expected cache to be included by default, but it was nil")
	}

	// Verify cache is enabled by default
	if !config.Cache.Enabled {
		t.Error("Expected cache to be enabled by default, but it was disabled")
	}

	// Verify default cache settings
	expectedStoragePath := ".lsp-gateway/scip-cache"
	if !containsPath(config.Cache.StoragePath, expectedStoragePath) {
		t.Errorf("Expected storage path to contain %s, got %s", expectedStoragePath, config.Cache.StoragePath)
	}

	if config.Cache.MaxMemoryMB != 256 {
		t.Errorf("Expected max memory to be 256MB, got %d", config.Cache.MaxMemoryMB)
	}

	if config.Cache.TTL != 24*time.Hour {
		t.Errorf("Expected TTL to be 24 hours, got %v", config.Cache.TTL)
	}

	if !config.Cache.BackgroundIndex {
		t.Error("Expected background index to be enabled by default")
	}

	if config.Cache.HealthCheckInterval != 5*time.Minute {
		t.Errorf("Expected health check interval to be 5 minutes, got %v", config.Cache.HealthCheckInterval)
	}
}

func TestGetDefaultSCIPConfig_EnabledByDefault(t *testing.T) {
	scipConfig := GetDefaultSCIPConfig()

	if !scipConfig.Enabled {
		t.Error("Expected SCIP config to be enabled by default")
	}

	// Check languages include the core supported languages
	expectedLanguages := []string{"go", "python", "typescript", "java"}
	if len(scipConfig.Languages) != len(expectedLanguages) {
		t.Errorf("Expected %d languages, got %d", len(expectedLanguages), len(scipConfig.Languages))
	}

	for _, lang := range expectedLanguages {
		found := false
		for _, configLang := range scipConfig.Languages {
			if configLang == lang {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected language %s to be in default config", lang)
		}
	}
}

func TestConfig_IsCacheEnabled(t *testing.T) {
	config := GetDefaultConfig()

	// Should be enabled by default now
	if !config.IsCacheEnabled() {
		t.Error("Expected IsCacheEnabled() to return true for default config")
	}

	// Test with disabled cache
	config.Cache.Enabled = false
	if config.IsCacheEnabled() {
		t.Error("Expected IsCacheEnabled() to return false when cache is disabled")
	}

	// Test with nil cache
	config.Cache = nil
	if config.IsCacheEnabled() {
		t.Error("Expected IsCacheEnabled() to return false when cache is nil")
	}
}

func TestConfig_HasCache(t *testing.T) {
	config := GetDefaultConfig()

	// Should have cache by default now
	if !config.HasCache() {
		t.Error("Expected HasCache() to return true for default config")
	}

	// Test with nil cache
	config.Cache = nil
	if config.HasCache() {
		t.Error("Expected HasCache() to return false when cache is nil")
	}
}

func TestGetDefaultConfigWithCache_ReturnsDefaultConfig(t *testing.T) {
	defaultConfig := GetDefaultConfig()
	cacheConfig := GetDefaultConfigWithCache()

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
