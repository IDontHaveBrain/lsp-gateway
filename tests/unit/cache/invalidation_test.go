package cache_test

import (
	"context"
	"testing"

	"lsp-gateway/src/server/cache"
	"lsp-gateway/src/server/documents"
)

// MockStorage implements SCIPStorage for testing
type MockStorage struct {
	entries map[cache.CacheKey]*cache.CacheEntry
	size    int64
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		entries: make(map[cache.CacheKey]*cache.CacheEntry),
	}
}

func (ms *MockStorage) Store(key cache.CacheKey, entry *cache.CacheEntry) error {
	ms.entries[key] = entry
	ms.size += entry.Size
	return nil
}

func (ms *MockStorage) Retrieve(key cache.CacheKey) (*cache.CacheEntry, error) {
	entry, exists := ms.entries[key]
	if !exists {
		return nil, nil
	}
	return entry, nil
}

func (ms *MockStorage) Delete(key cache.CacheKey) error {
	if entry, exists := ms.entries[key]; exists {
		ms.size -= entry.Size
		delete(ms.entries, key)
	}
	return nil
}

func (ms *MockStorage) Clear() error {
	ms.entries = make(map[cache.CacheKey]*cache.CacheEntry)
	ms.size = 0
	return nil
}

func (ms *MockStorage) Size() int64 {
	return ms.size
}

func (ms *MockStorage) EntryCount() int64 {
	return int64(len(ms.entries))
}

// Test basic invalidation manager creation
func TestNewSimpleInvalidationManager(t *testing.T) {
	storage := NewMockStorage()
	docManager := documents.NewLSPDocumentManager()

	manager := cache.NewSimpleInvalidationManager(storage, docManager)

	if manager == nil {
		t.Fatal("Expected non-nil invalidation manager")
	}
}

// Test manager start and stop
func TestInvalidationManagerLifecycle(t *testing.T) {
	storage := NewMockStorage()
	docManager := documents.NewLSPDocumentManager()
	manager := cache.NewSimpleInvalidationManager(storage, docManager)

	ctx := context.Background()

	// Test start
	err := manager.Start(ctx)
	if err != nil {
		t.Errorf("Failed to start invalidation manager: %v", err)
	}

	// Test stop
	err = manager.Stop()
	if err != nil {
		t.Errorf("Failed to stop invalidation manager: %v", err)
	}
}

// Test document invalidation
func TestDocumentInvalidation(t *testing.T) {
	storage := NewMockStorage()
	docManager := documents.NewLSPDocumentManager()
	manager := cache.NewSimpleInvalidationManager(storage, docManager)

	testURI := "file:///test.go"

	err := manager.InvalidateDocument(testURI)
	if err != nil {
		t.Errorf("Failed to invalidate document: %v", err)
	}
}

// Test symbol invalidation (no-op in simple implementation)
func TestSymbolInvalidation(t *testing.T) {
	storage := NewMockStorage()
	docManager := documents.NewLSPDocumentManager()
	manager := cache.NewSimpleInvalidationManager(storage, docManager)

	symbolID := "test-symbol-123"

	err := manager.InvalidateSymbol(symbolID)
	if err != nil {
		t.Errorf("Failed to invalidate symbol: %v", err)
	}
}

// Test pattern invalidation (no-op in simple implementation)
func TestPatternInvalidation(t *testing.T) {
	storage := NewMockStorage()
	docManager := documents.NewLSPDocumentManager()
	manager := cache.NewSimpleInvalidationManager(storage, docManager)

	pattern := "*.go"

	err := manager.InvalidatePattern(pattern)
	if err != nil {
		t.Errorf("Failed to invalidate pattern: %v", err)
	}
}

// Test getting stats
func TestGetStats(t *testing.T) {
	storage := NewMockStorage()
	docManager := documents.NewLSPDocumentManager()
	manager := cache.NewSimpleInvalidationManager(storage, docManager)

	stats := manager.GetStats()
	if stats == nil {
		t.Error("Expected non-nil stats")
	}

	// Verify basic stats structure
	if stats["type"] != "simple" {
		t.Errorf("Expected type 'simple', got %v", stats["type"])
	}

	t.Logf("Invalidation manager stats: %+v", stats)
}
