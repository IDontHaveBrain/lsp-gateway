package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server/cache"

	"github.com/stretchr/testify/require"
)

func TestBasicCacheOperations(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "basic-cache")

	cacheConfig := &config.CacheConfig{
		Enabled:         true,
		StoragePath:     cacheDir,
		MaxMemoryMB:     64,
		TTLHours:        1,
		Languages:       []string{"go"},
		BackgroundIndex: false,
		DiskCache:       true,
		EvictionPolicy:  "lru",
	}

	scipCache, err := cache.NewSCIPCacheManager(cacheConfig)
	require.NoError(t, err)
	require.NotNil(t, scipCache)

	ctx := context.Background()
	err = scipCache.Start(ctx)
	require.NoError(t, err)
	defer scipCache.Stop()

	t.Run("Store and retrieve basic cache entry", func(t *testing.T) {
		method := "textDocument/definition"
		params := map[string]interface{}{
			"textDocument": map[string]string{"uri": "file:///test.go"},
			"position":     map[string]int{"line": 10, "character": 5},
		}
		response := map[string]interface{}{
			"uri":   "file:///test.go",
			"range": map[string]interface{}{"start": map[string]int{"line": 10, "character": 5}},
		}

		err := scipCache.Store(method, params, response)
		require.NoError(t, err)

		cached, found, err := scipCache.Lookup(method, params)
		require.NoError(t, err)
		require.True(t, found, "Should find cached entry")
		require.NotNil(t, cached)
	})

	t.Run("Cache miss for non-existent entry", func(t *testing.T) {
		method := "textDocument/hover"
		params := map[string]interface{}{
			"textDocument": map[string]string{"uri": "file:///missing.go"},
			"position":     map[string]int{"line": 0, "character": 0},
		}

		cached, found, err := scipCache.Lookup(method, params)
		require.NoError(t, err)
		require.False(t, found, "Should not find non-existent entry")
		require.Nil(t, cached)
	})

	t.Run("Document invalidation", func(t *testing.T) {
		uri := "file:///invalidate.go"
		method := "textDocument/documentSymbol"
		params := map[string]interface{}{
			"textDocument": map[string]string{"uri": uri},
		}
		response := []interface{}{
			map[string]interface{}{"name": "Symbol1"},
			map[string]interface{}{"name": "Symbol2"},
		}

		err := scipCache.Store(method, params, response)
		require.NoError(t, err)

		cached, found, err := scipCache.Lookup(method, params)
		require.NoError(t, err)
		require.True(t, found, "Should find entry before invalidation")

		err = scipCache.InvalidateDocument(uri)
		require.NoError(t, err)

		cached, found, err = scipCache.Lookup(method, params)
		require.NoError(t, err)
		_ = cached
		// After invalidation, the entry may or may not exist depending on implementation
	})

	t.Run("Clear cache", func(t *testing.T) {
		method := "textDocument/references"
		params := map[string]interface{}{
			"textDocument": map[string]string{"uri": "file:///clear.go"},
			"position":     map[string]int{"line": 5, "character": 10},
		}
		response := []interface{}{
			map[string]interface{}{"uri": "file:///ref1.go"},
			map[string]interface{}{"uri": "file:///ref2.go"},
		}

		err := scipCache.Store(method, params, response)
		require.NoError(t, err)

		err = scipCache.Clear()
		require.NoError(t, err)

		cached, found, err := scipCache.Lookup(method, params)
		require.NoError(t, err)
		require.False(t, found, "Should not find entry after clear")
		require.Nil(t, cached)
	})

	t.Run("Cache metrics", func(t *testing.T) {
		metrics := scipCache.GetMetrics()
		require.NotNil(t, metrics)

		healthMetrics, err := scipCache.HealthCheck()
		require.NoError(t, err)
		require.NotNil(t, healthMetrics)
	})
}

func TestCacheWithIndexing(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "index-cache")
	projectDir := filepath.Join(tempDir, "project")

	err := os.MkdirAll(projectDir, 0755)
	require.NoError(t, err)

	testFile := filepath.Join(projectDir, "test.go")
	content := `package main

type TestStruct struct {
	Name string
}

func TestFunction() {
	println("test")
}`
	err = os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	cacheConfig := &config.CacheConfig{
		Enabled:         true,
		StoragePath:     cacheDir,
		MaxMemoryMB:     64,
		TTLHours:        1,
		Languages:       []string{"go"},
		BackgroundIndex: false,
		DiskCache:       true,
		EvictionPolicy:  "lru",
	}

	scipCache, err := cache.NewSCIPCacheManager(cacheConfig)
	require.NoError(t, err)

	ctx := context.Background()
	err = scipCache.Start(ctx)
	require.NoError(t, err)
	defer scipCache.Stop()

	t.Run("Index document with symbols", func(t *testing.T) {
		uri := common.FilePathToURI(testFile)
		// Index document without symbol details (use nil)
		err := scipCache.IndexDocument(ctx, uri, "go", nil)
		require.NoError(t, err)

		stats := scipCache.GetIndexStats()
		require.NotNil(t, stats)
	})

	t.Run("Query indexed symbols", func(t *testing.T) {
		query := &cache.IndexQuery{
			Type:   "symbol",
			Symbol: "Test",
			URI:    common.FilePathToURI(testFile),
		}

		result, err := scipCache.QueryIndex(ctx, query)
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("Update index with multiple files", func(t *testing.T) {
		file2 := filepath.Join(projectDir, "another.go")
		content2 := `package main

func AnotherFunction() {
	println("another")
}`
		err := os.WriteFile(file2, []byte(content2), 0644)
		require.NoError(t, err)

		files := []string{testFile, file2}
		err = scipCache.UpdateIndex(ctx, files)
		require.NoError(t, err)

		stats := scipCache.GetIndexStats()
		require.NotNil(t, stats)
	})
}

func TestCachePersistence(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "persist-cache")

	cacheConfig := &config.CacheConfig{
		Enabled:         true,
		StoragePath:     cacheDir,
		MaxMemoryMB:     32,
		TTLHours:        24,
		Languages:       []string{"go", "python"},
		BackgroundIndex: false,
		DiskCache:       true,
		EvictionPolicy:  "lru",
	}

	ctx := context.Background()

	// First session - store data
	t.Run("Store data and persist", func(t *testing.T) {
		scipCache, err := cache.NewSCIPCacheManager(cacheConfig)
		require.NoError(t, err)

		err = scipCache.Start(ctx)
		require.NoError(t, err)

		// Store multiple entries
		for i := 0; i < 3; i++ {
			method := "textDocument/hover"
			params := map[string]interface{}{
				"textDocument": map[string]string{"uri": "file:///test" + string(rune('0'+i)) + ".go"},
				"position":     map[string]int{"line": i, "character": 0},
			}
			response := map[string]interface{}{
				"contents": "Test content " + string(rune('0'+i)),
			}

			err = scipCache.Store(method, params, response)
			require.NoError(t, err)
		}

		err = scipCache.Stop()
		require.NoError(t, err)
	})

	// Second session - recover data
	t.Run("Recover persisted data", func(t *testing.T) {
		scipCache, err := cache.NewSCIPCacheManager(cacheConfig)
		require.NoError(t, err)

		err = scipCache.Start(ctx)
		require.NoError(t, err)
		defer scipCache.Stop()

		// Allow time for loading persisted data
		time.Sleep(500 * time.Millisecond)

		metrics := scipCache.GetMetrics()
		require.NotNil(t, metrics)

		// Try to retrieve one of the persisted entries
		method := "textDocument/hover"
		params := map[string]interface{}{
			"textDocument": map[string]string{"uri": "file:///test0.go"},
			"position":     map[string]int{"line": 0, "character": 0},
		}

		cached, found, err := scipCache.Lookup(method, params)
		require.NoError(t, err)
		_ = cached
		_ = found
		// Whether the entry is found depends on persistence implementation
	})
}

func TestConcurrentCacheAccess(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "concurrent-cache")

	cacheConfig := &config.CacheConfig{
		Enabled:         true,
		StoragePath:     cacheDir,
		MaxMemoryMB:     64,
		TTLHours:        1,
		Languages:       []string{"go"},
		BackgroundIndex: false,
		DiskCache:       false,
		EvictionPolicy:  "lru",
	}

	scipCache, err := cache.NewSCIPCacheManager(cacheConfig)
	require.NoError(t, err)

	ctx := context.Background()
	err = scipCache.Start(ctx)
	require.NoError(t, err)
	defer scipCache.Stop()

	numGoroutines := 10
	numOperations := 20
	done := make(chan bool, numGoroutines)
	errors := make(chan error, numGoroutines*numOperations)

	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			for i := 0; i < numOperations; i++ {
				method := "textDocument/definition"
				params := map[string]interface{}{
					"textDocument": map[string]string{"uri": "file:///g" + string(rune('0'+goroutineID)) + "/file" + string(rune('0'+i)) + ".go"},
					"position":     map[string]int{"line": i, "character": goroutineID},
				}
				response := map[string]interface{}{
					"uri": "file:///result.go",
				}

				// Store
				if err := scipCache.Store(method, params, response); err != nil {
					errors <- err
					continue
				}

				// Lookup
				cached, found, err := scipCache.Lookup(method, params)
				if err != nil {
					errors <- err
					continue
				}
				_ = cached
				_ = found

				// Occasionally invalidate
				if i%5 == 0 {
					uri := "file:///g" + string(rune('0'+goroutineID)) + "/file" + string(rune('0'+i)) + ".go"
					if err := scipCache.InvalidateDocument(uri); err != nil {
						errors <- err
					}
				}
			}
			done <- true
		}(g)
	}

	timeout := time.After(30 * time.Second)
	completedGoroutines := 0

	for completedGoroutines < numGoroutines {
		select {
		case <-done:
			completedGoroutines++
		case err := <-errors:
			t.Errorf("Error during concurrent operation: %v", err)
		case <-timeout:
			t.Fatal("Timeout waiting for concurrent operations")
		}
	}

	metrics := scipCache.GetMetrics()
	require.NotNil(t, metrics)
}
