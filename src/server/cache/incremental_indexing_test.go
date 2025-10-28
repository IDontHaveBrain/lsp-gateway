package cache_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server/cache"
)

func TestIncrementalIndexing(t *testing.T) {
	// Create temp directory for test
	tempDir, err := os.MkdirTemp("", "incremental-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	file1 := filepath.Join(tempDir, "file1.go")
	file2 := filepath.Join(tempDir, "file2.go")

	// Write initial content
	content1 := `package main
func Hello() string {
	return "Hello"
}`
	content2 := `package main
func World() string {
	return "World"
}`

	if err := os.WriteFile(file1, []byte(content1), 0644); err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte(content2), 0644); err != nil {
		t.Fatalf("Failed to write file2: %v", err)
	}

	// Create cache manager with storage
	cacheDir := filepath.Join(tempDir, ".cache")
	cfg := &config.CacheConfig{
		Enabled:     true,
		MaxMemoryMB: 100,
		TTLHours:    1,
		DiskCache:   true,
		StoragePath: cacheDir,
	}

	cacheManager, err := cache.NewSCIPCacheManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create cache manager: %v", err)
	}

	ctx := context.Background()
	if err := cacheManager.Start(ctx); err != nil {
		t.Fatalf("Failed to start cache manager: %v", err)
	}
	defer cacheManager.Stop()

	// Mock LSP fallback
	mockLSP := &mockLSPFallback{
		responses: make(map[string]interface{}),
	}

	// First indexing - should index both files
	t.Run("InitialIndexing", func(t *testing.T) {
		err := cacheManager.PerformIncrementalIndexing(ctx, tempDir, mockLSP)
		if err != nil {
			t.Errorf("Initial indexing failed: %v", err)
		}

		// Check that files are tracked
		trackedCount := cacheManager.GetTrackedFileCount()
		if trackedCount != 2 {
			t.Errorf("Expected 2 tracked files, got %d", trackedCount)
		}
	})

	// Wait a bit to ensure file modification time difference
	time.Sleep(100 * time.Millisecond)

	// Modify one file
	t.Run("FileModification", func(t *testing.T) {
		newContent := `package main
func Hello() string {
	return "Hello, Modified!"
}`
		if err := os.WriteFile(file1, []byte(newContent), 0644); err != nil {
			t.Fatalf("Failed to modify file1: %v", err)
		}

		// Re-index - should only index the modified file
		err := cacheManager.PerformIncrementalIndexing(ctx, tempDir, mockLSP)
		if err != nil {
			t.Errorf("Incremental indexing after modification failed: %v", err)
		}

		// Files should still be tracked
		trackedCount := cacheManager.GetTrackedFileCount()
		if trackedCount != 2 {
			t.Errorf("Expected 2 tracked files after modification, got %d", trackedCount)
		}
	})

	// Add a new file
	t.Run("NewFile", func(t *testing.T) {
		file3 := filepath.Join(tempDir, "file3.go")
		content3 := `package main
func NewFunc() string {
	return "New"
}`
		if err := os.WriteFile(file3, []byte(content3), 0644); err != nil {
			t.Fatalf("Failed to create file3: %v", err)
		}

		// Re-index - should index only the new file
		err := cacheManager.PerformIncrementalIndexing(ctx, tempDir, mockLSP)
		if err != nil {
			t.Errorf("Incremental indexing after new file failed: %v", err)
		}

		// Should now track 3 files
		trackedCount := cacheManager.GetTrackedFileCount()
		if trackedCount != 3 {
			t.Errorf("Expected 3 tracked files after adding new file, got %d", trackedCount)
		}
	})

	// Delete a file
	t.Run("FileDelection", func(t *testing.T) {
		if err := os.Remove(file2); err != nil {
			t.Fatalf("Failed to delete file2: %v", err)
		}

		// Re-index - should detect deletion
		err := cacheManager.PerformIncrementalIndexing(ctx, tempDir, mockLSP)
		if err != nil {
			t.Errorf("Incremental indexing after deletion failed: %v", err)
		}

		// Should now track 2 files
		trackedCount := cacheManager.GetTrackedFileCount()
		if trackedCount != 2 {
			t.Errorf("Expected 2 tracked files after deletion, got %d", trackedCount)
		}
	})
}

// mockLSPFallback is a mock implementation for testing
type mockLSPFallback struct {
	responses map[string]interface{}
}

func (m *mockLSPFallback) ProcessRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	// Return mock document symbols
	return []interface{}{
		map[string]interface{}{
			"name": "MockSymbol",
			"kind": 12,
			"range": map[string]interface{}{
				"start": map[string]interface{}{"line": 0, "character": 0},
				"end":   map[string]interface{}{"line": 1, "character": 0},
			},
		},
	}, nil
}
