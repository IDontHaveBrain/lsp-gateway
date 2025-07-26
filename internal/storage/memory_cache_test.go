package storage

import (
	"context"
	"encoding/json"
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

func TestL1MemoryCache_TTLExpiry(t *testing.T) {
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