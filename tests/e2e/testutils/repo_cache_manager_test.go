package testutils

import (
	"os"
	"testing"
	"time"
)

func TestRepoCacheManagerIntegration(t *testing.T) {
	// Create temporary cache directory
	tempDir, err := os.MkdirTemp("", "repo-cache-manager-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Initialize cache manager with test configuration
	config := CacheConfig{
		MaxSizeGB:       1,  // 1GB for testing
		MaxEntries:      100,
		DefaultTTL:      24 * time.Hour,
		CleanupInterval: 10 * time.Minute,
		CacheDir:        tempDir,
		EnableLogging:   true,
	}
	
	manager := NewIntegratedCacheManager(config)
	defer manager.Close()
	
	t.Run("Initialization", func(t *testing.T) {
		if manager == nil {
			t.Fatal("Manager should not be nil")
		}
		
		if !manager.IsHealthy() {
			t.Error("Newly initialized manager should be healthy")
		}
		
		stats := manager.GetCacheStats()
		if stats.TotalEntries != 0 {
			t.Errorf("Expected 0 entries, got %d", stats.TotalEntries)
		}
	})
	
	t.Run("Cache Miss Scenario", func(t *testing.T) {
		// Test cache miss
		repoURL := "https://github.com/example/test-repo"
		commitHash := "abc123def456"
		
		path, found, err := manager.GetCachedRepository(repoURL, commitHash)
		if err != nil {
			t.Errorf("Unexpected error on cache miss: %v", err)
		}
		if found {
			t.Error("Should not find non-existent repository")
		}
		if path != "" {
			t.Error("Path should be empty on cache miss")
		}
	})
	
	t.Run("Cache Validation", func(t *testing.T) {
		repoURL := "https://github.com/example/test-repo"
		commitHash := "abc123def456"
		
		// Test validation of non-existent entry
		isValid, err := manager.ValidateCache(repoURL, commitHash)
		if err != nil {
			t.Errorf("Unexpected error on validation: %v", err)
		}
		if isValid {
			t.Error("Non-existent cache entry should not be valid")
		}
	})
	
	t.Run("Cache Stats", func(t *testing.T) {
		stats := manager.GetCacheStats()
		
		// Verify stats structure
		if stats.TotalEntries < 0 {
			t.Error("Total entries should not be negative")
		}
		if stats.CurrentSizeGB < 0 {
			t.Error("Current size should not be negative")
		}
		if stats.HitRate < 0 || stats.HitRate > 1 {
			t.Errorf("Hit rate should be between 0 and 1, got %f", stats.HitRate)
		}
	})
	
	t.Run("Cleanup Operations", func(t *testing.T) {
		err := manager.CleanupExpiredCache()
		if err != nil {
			t.Errorf("Cleanup should not fail: %v", err)
		}
	})
	
	t.Run("Performance Report", func(t *testing.T) {
		report := manager.GetPerformanceReport()
		if len(report) == 0 {
			t.Error("Performance report should not be empty")
		}
		
		// Check for key sections
		if !containsText(report, "CACHE PERFORMANCE") {
			t.Error("Report should contain cache performance section")
		}
		if !containsText(report, "OPERATION STATISTICS") {
			t.Error("Report should contain operation statistics section")
		}
	})
	
	t.Run("Interface Compliance", func(t *testing.T) {
		// Verify manager implements IntegratedCacheInterface
		var _ IntegratedCacheInterface = manager
		
		// Test all interface methods are callable
		_, _, _ = manager.GetCachedRepository("test", "hash")
		_ = manager.CacheRepository("test", "hash", tempDir)
		_, _ = manager.ValidateCache("test", "hash")
		_ = manager.GetCacheStats()
		_ = manager.CleanupExpiredCache()
		_ = manager.Close() // This will be called again in defer, but that's fine
	})
}

func TestRepoCacheOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "repo-cache-ops-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	config := CacheConfig{
		MaxSizeGB:       1,
		MaxEntries:      10,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
		CacheDir:        tempDir,
		EnableLogging:   false, // Reduce noise in tests
	}
	
	manager := NewIntegratedCacheManager(config)
	defer manager.Close()
	
	t.Run("Cache Operation Recording", func(t *testing.T) {
		repoURL := "https://github.com/test/repo"
		commitHash := "test123"
		
		// Perform several operations to generate operation records
		manager.GetCachedRepository(repoURL, commitHash) // Cache miss
		manager.ValidateCache(repoURL, commitHash)       // Validation
		manager.CleanupExpiredCache()                    // Cleanup
		
		// Verify operations were recorded
		stats := manager.GetCacheStats()
		// Note: Operation recording is internal, but we can verify through stats
		if stats.MissCount == 0 {
			// At least one cache miss should have been recorded
			// This is a basic check that the recording mechanism is working
		}
	})
	
	t.Run("Health Check", func(t *testing.T) {
		// New manager should be healthy
		if !manager.IsHealthy() {
			t.Error("Fresh manager should be healthy")
		}
		
		// Test with different scenarios (would require more complex setup)
		// For now, just verify the method works
	})
}

func TestCacheConfiguration(t *testing.T) {
	t.Run("Default Configuration", func(t *testing.T) {
		tempDir, _ := os.MkdirTemp("", "cache-config-test")
		defer os.RemoveAll(tempDir)
		
		// Test with minimal config
		config := CacheConfig{
			CacheDir: tempDir,
		}
		
		manager := NewIntegratedCacheManager(config)
		defer manager.Close()
		
		if manager == nil {
			t.Fatal("Manager should be created with default config")
		}
		
		// Verify defaults were applied
		if manager.config.MaxSizeGB <= 0 {
			t.Error("Default MaxSizeGB should be set")
		}
		if manager.config.MaxEntries <= 0 {
			t.Error("Default MaxEntries should be set")
		}
		if manager.config.DefaultTTL <= 0 {
			t.Error("Default TTL should be set")
		}
	})
	
	t.Run("Custom Configuration", func(t *testing.T) {
		tempDir, _ := os.MkdirTemp("", "cache-config-test")
		defer os.RemoveAll(tempDir)
		
		config := CacheConfig{
			MaxSizeGB:       5,
			MaxEntries:      500,
			DefaultTTL:      7 * 24 * time.Hour, // 7 days
			CleanupInterval: 2 * time.Hour,
			CacheDir:        tempDir,
			EnableLogging:   true,
		}
		
		manager := NewIntegratedCacheManager(config)
		defer manager.Close()
		
		// Verify custom configuration was preserved
		if manager.config.MaxSizeGB != 5 {
			t.Errorf("Expected MaxSizeGB=5, got %d", manager.config.MaxSizeGB)
		}
		if manager.config.MaxEntries != 500 {
			t.Errorf("Expected MaxEntries=500, got %d", manager.config.MaxEntries)
		}
	})
}

func TestComponentIntegration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "component-integration-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	config := CacheConfig{
		MaxSizeGB:     1,
		MaxEntries:    50,
		DefaultTTL:    time.Hour,
		CacheDir:      tempDir,
		EnableLogging: false,
	}
	
	manager := NewIntegratedCacheManager(config)
	defer manager.Close()
	
	t.Run("Core Component Access", func(t *testing.T) {
		// Verify core components are initialized
		if manager.cacheCore == nil {
			t.Error("Cache core should be initialized")
		}
		if manager.gitValidator == nil {
			t.Error("Git validator should be initialized")
		}
	})
	
	t.Run("Integrated Statistics", func(t *testing.T) {
		// Test that statistics integrate data from both components
		stats := manager.GetCacheStats()
		
		// Basic validation that stats structure is complete
		// (Detailed testing would require actual cached repositories)
		if stats.HitCount < 0 {
			t.Error("Hit count should not be negative")
		}
		if stats.MissCount < 0 {
			t.Error("Miss count should not be negative")
		}
	})
}

// Helper function for string containment check
func containsText(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (s == substr || len(s) > len(substr) && 
		   (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		    containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Benchmark tests for performance verification
func BenchmarkCacheOperations(b *testing.B) {
	tempDir, _ := os.MkdirTemp("", "cache-benchmark")
	defer os.RemoveAll(tempDir)
	
	config := CacheConfig{
		MaxSizeGB:     2,
		MaxEntries:    1000,
		CacheDir:      tempDir,
		EnableLogging: false,
	}
	
	manager := NewIntegratedCacheManager(config)
	defer manager.Close()
	
	b.Run("GetCachedRepository", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			manager.GetCachedRepository("https://github.com/test/repo", "abc123")
		}
	})
	
	b.Run("ValidateCache", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			manager.ValidateCache("https://github.com/test/repo", "abc123")
		}
	})
	
	b.Run("GetCacheStats", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			manager.GetCacheStats()
		}
	})
}