package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestL1MemoryCache_BasicOperations(t *testing.T) {
	cache := NewL1MemoryCache(nil)
	ctx := context.Background()

	err := cache.Initialize(ctx, TierConfig{
		TierType: TierL1Memory,
		BackendConfig: BackendConfig{
			Type: BackendMemory,
		},
	})
	if err != nil {
		t.Fatalf("Failed to initialize cache: %v", err)
	}
	defer cache.Close()

	// Test basic Put/Get operations
	entry := &CacheEntry{
		Method:     "textDocument/definition",
		Params:     `{"textDocument":{"uri":"file:///test.go"},"position":{"line":10,"character":5}}`,
		Response:   json.RawMessage(`{"uri":"file:///test.go","range":{"start":{"line":5,"character":0}}}`),
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		TTL:        time.Hour,
	}

	// Test Put
	err = cache.Put(ctx, "test-key-1", entry)
	if err != nil {
		t.Errorf("Put failed: %v", err)
	}

	// Test Get
	retrieved, err := cache.Get(ctx, "test-key-1")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if retrieved == nil {
		t.Error("Get returned nil entry")
		return
	}
	if retrieved.Method != entry.Method {
		t.Errorf("Expected method %s, got %s", entry.Method, retrieved.Method)
	}
	if retrieved.Params != entry.Params {
		t.Errorf("Expected params %s, got %s", entry.Params, retrieved.Params)
	}

	// Test Exists
	exists, err := cache.Exists(ctx, "test-key-1")
	if err != nil {
		t.Errorf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("Entry should exist")
	}

	// Test Delete
	err = cache.Delete(ctx, "test-key-1")
	if err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	exists, err = cache.Exists(ctx, "test-key-1")
	if err != nil {
		t.Errorf("Exists check after delete failed: %v", err)
	}
	if exists {
		t.Error("Entry should not exist after deletion")
	}
}

func TestL1MemoryCache_TTLExpiration(t *testing.T) {
	cache := NewL1MemoryCache(nil)
	ctx := context.Background()

	err := cache.Initialize(ctx, TierConfig{
		TierType: TierL1Memory,
		BackendConfig: BackendConfig{
			Type: BackendMemory,
		},
	})
	if err != nil {
		t.Fatalf("Failed to initialize cache: %v", err)
	}
	defer cache.Close()

	// Create entry with short TTL
	entry := &CacheEntry{
		Method:     "textDocument/hover",
		Params:     `{"textDocument":{"uri":"file:///test.go"},"position":{"line":5,"character":10}}`,
		Response:   json.RawMessage(`{"contents":"test function"}`),
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		TTL:        10 * time.Millisecond,
	}

	err = cache.Put(ctx, "ttl-test-key", entry)
	if err != nil {
		t.Errorf("Put failed: %v", err)
	}

	// Wait for TTL to expire
	time.Sleep(20 * time.Millisecond)

	// Entry should be expired
	retrieved, err := cache.Get(ctx, "ttl-test-key")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if retrieved != nil {
		t.Error("Entry should have expired")
	}
}

func TestL1MemoryCache_NonExistentKey(t *testing.T) {
	cache := NewL1MemoryCache(nil)
	ctx := context.Background()

	err := cache.Initialize(ctx, TierConfig{
		TierType: TierL1Memory,
		BackendConfig: BackendConfig{
			Type: BackendMemory,
		},
	})
	if err != nil {
		t.Fatalf("Failed to initialize cache: %v", err)
	}
	defer cache.Close()

	// Test Get with non-existent key
	retrieved, err := cache.Get(ctx, "non-existent-key")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if retrieved != nil {
		t.Error("Get should return nil for non-existent key")
	}

	// Test Exists with non-existent key
	exists, err := cache.Exists(ctx, "non-existent-key")
	if err != nil {
		t.Errorf("Exists failed: %v", err)
	}
	if exists {
		t.Error("Exists should return false for non-existent key")
	}

	// Test Delete with non-existent key (should not error)
	err = cache.Delete(ctx, "non-existent-key")
	if err != nil {
		t.Errorf("Delete failed: %v", err)
	}
}

func TestL1MemoryCache_ConcurrentAccess(t *testing.T) {
	cache := NewL1MemoryCache(nil)
	ctx := context.Background()

	err := cache.Initialize(ctx, TierConfig{
		TierType: TierL1Memory,
		BackendConfig: BackendConfig{
			Type: BackendMemory,
		},
	})
	if err != nil {
		t.Fatalf("Failed to initialize cache: %v", err)
	}
	defer cache.Close()

	// Test concurrent operations
	const numGoroutines = 50
	const numOperations = 5

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numOperations)

	// Concurrent puts
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				entry := &CacheEntry{
					Method:     "textDocument/completion",
					Params:     fmt.Sprintf(`{"textDocument":{"uri":"file:///test_%d_%d.go"}}`, id, j),
					Response:   json.RawMessage(`{"items":[{"label":"test","kind":1}]}`),
					CreatedAt:  time.Now(),
					AccessedAt: time.Now(),
					TTL:        time.Hour,
				}

				key := fmt.Sprintf("concurrent-key-%d-%d", id, j)
				if err := cache.Put(ctx, key, entry); err != nil {
					errors <- err
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent operation failed: %v", err)
	}

	// Verify entries were stored
	stats := cache.GetStats()
	expectedEntries := int64(numGoroutines * numOperations)
	if stats.EntryCount != expectedEntries {
		t.Errorf("Expected %d entries, got %d", expectedEntries, stats.EntryCount)
	}
}

func TestL1MemoryCache_CacheInvalidation(t *testing.T) {
	cache := NewL1MemoryCache(nil)
	ctx := context.Background()

	err := cache.Initialize(ctx, TierConfig{
		TierType: TierL1Memory,
		BackendConfig: BackendConfig{
			Type: BackendMemory,
		},
	})
	if err != nil {
		t.Fatalf("Failed to initialize cache: %v", err)
	}
	defer cache.Close()

	// Add test entries with file associations
	testEntries := map[string]*CacheEntry{
		"file1-def": {
			Method:      "textDocument/definition",
			Params:      `{"textDocument":{"uri":"file:///project/file1.go"}}`,
			Response:    json.RawMessage(`{"uri":"file:///project/file1.go","range":{"start":{"line":10,"character":0}}}`),
			FilePaths:   []string{"/project/file1.go"},
			ProjectPath: "/project",
			CreatedAt:   time.Now(),
			AccessedAt:  time.Now(),
			TTL:         time.Hour,
		},
		"file1-hover": {
			Method:      "textDocument/hover",
			Params:      `{"textDocument":{"uri":"file:///project/file1.go"}}`,
			Response:    json.RawMessage(`{"contents":"Hover info for file1"}`),
			FilePaths:   []string{"/project/file1.go"},
			ProjectPath: "/project",
			CreatedAt:   time.Now(),
			AccessedAt:  time.Now(),
			TTL:         time.Hour,
		},
		"file2-def": {
			Method:      "textDocument/definition",
			Params:      `{"textDocument":{"uri":"file:///project/file2.go"}}`,
			Response:    json.RawMessage(`{"uri":"file:///project/file2.go","range":{"start":{"line":5,"character":0}}}`),
			FilePaths:   []string{"/project/file2.go"},
			ProjectPath: "/project",
			CreatedAt:   time.Now(),
			AccessedAt:  time.Now(),
			TTL:         time.Hour,
		},
	}

	// Put all entries
	for key, entry := range testEntries {
		err = cache.Put(ctx, key, entry)
		if err != nil {
			t.Errorf("Put failed for key %s: %v", key, err)
		}
	}

	// Test invalidation by file
	invalidated, err := cache.InvalidateByFile(ctx, "/project/file1.go")
	if err != nil {
		t.Errorf("InvalidateByFile failed: %v", err)
	}
	if invalidated != 2 {
		t.Errorf("Expected 2 invalidated entries, got %d", invalidated)
	}

	// Verify file1 entries are gone
	exists, err := cache.Exists(ctx, "file1-def")
	if err != nil {
		t.Errorf("Exists check failed: %v", err)
	}
	if exists {
		t.Error("file1-def should have been invalidated")
	}

	exists, err = cache.Exists(ctx, "file1-hover")
	if err != nil {
		t.Errorf("Exists check failed: %v", err)
	}
	if exists {
		t.Error("file1-hover should have been invalidated")
	}

	// Verify other entries still exist
	exists, err = cache.Exists(ctx, "file2-def")
	if err != nil {
		t.Errorf("Exists check failed: %v", err)
	}
	if !exists {
		t.Error("file2-def should still exist")
	}

	// Test invalidation by project
	invalidated, err = cache.InvalidateByProject(ctx, "/project")
	if err != nil {
		t.Errorf("InvalidateByProject failed: %v", err)
	}
	if invalidated != 1 {
		t.Errorf("Expected 1 invalidated entry, got %d", invalidated)
	}

	// Verify project entries are gone
	exists, err = cache.Exists(ctx, "file2-def")
	if err != nil {
		t.Errorf("Exists check failed: %v", err)
	}
	if exists {
		t.Error("file2-def should have been invalidated")
	}
}

func TestL1MemoryCache_BasicStats(t *testing.T) {
	cache := NewL1MemoryCache(nil)
	ctx := context.Background()

	err := cache.Initialize(ctx, TierConfig{
		TierType: TierL1Memory,
		BackendConfig: BackendConfig{
			Type: BackendMemory,
		},
	})
	if err != nil {
		t.Fatalf("Failed to initialize cache: %v", err)
	}
	defer cache.Close()

	// Add some test data
	entry := &CacheEntry{
		Method:     "textDocument/references",
		Params:     `{"textDocument":{"uri":"file:///test.go"}}`,
		Response:   json.RawMessage(`[{"uri":"file:///test.go","range":{"start":{"line":1,"character":1}}}]`),
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		TTL:        time.Hour,
	}

	for i := 0; i < 5; i++ {
		err = cache.Put(ctx, fmt.Sprintf("stats-test-key-%d", i), entry)
		if err != nil {
			t.Errorf("Put failed: %v", err)
		}
	}

	// Perform some gets
	for i := 0; i < 3; i++ {
		_, err = cache.Get(ctx, fmt.Sprintf("stats-test-key-%d", i))
		if err != nil {
			t.Errorf("Get failed: %v", err)
		}
	}

	// Test stats
	stats := cache.GetStats()
	if stats.TierType != TierL1Memory {
		t.Errorf("Expected tier type %v, got %v", TierL1Memory, stats.TierType)
	}
	if stats.EntryCount != 5 {
		t.Errorf("Expected 5 entries, got %d", stats.EntryCount)
	}
	if stats.TotalRequests <= 0 {
		t.Error("Total requests should be greater than 0")
	}
	if stats.CacheHits <= 0 {
		t.Error("Cache hits should be greater than 0")
	}
	if stats.UsedCapacity <= 0 {
		t.Error("Used capacity should be greater than 0")
	}
}
