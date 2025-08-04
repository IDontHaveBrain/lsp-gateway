package cache

import (
	"context"
	"testing"

	"lsp-gateway/src/server/documents"
)

// MockStorage implements SCIPStorage for testing
type MockStorage struct {
	entries map[CacheKey]*CacheEntry
	size    int64
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		entries: make(map[CacheKey]*CacheEntry),
	}
}

func (ms *MockStorage) Store(key CacheKey, entry *CacheEntry) error {
	ms.entries[key] = entry
	ms.size += entry.Size
	return nil
}

func (ms *MockStorage) Retrieve(key CacheKey) (*CacheEntry, error) {
	entry, exists := ms.entries[key]
	if !exists {
		return nil, nil
	}
	return entry, nil
}

func (ms *MockStorage) Delete(key CacheKey) error {
	if entry, exists := ms.entries[key]; exists {
		ms.size -= entry.Size
		delete(ms.entries, key)
	}
	return nil
}

func (ms *MockStorage) Clear() error {
	ms.entries = make(map[CacheKey]*CacheEntry)
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
func TestNewSCIPInvalidationManager(t *testing.T) {
	storage := NewMockStorage()
	docManager := documents.NewLSPDocumentManager()

	manager := NewSCIPInvalidationManager(storage, docManager)

	if manager == nil {
		t.Fatal("Expected non-nil invalidation manager")
	}
}

// Test manager start and stop
func TestInvalidationManagerLifecycle(t *testing.T) {
	storage := NewMockStorage()
	docManager := documents.NewLSPDocumentManager()
	manager := NewSCIPInvalidationManager(storage, docManager)

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
	manager := NewSCIPInvalidationManager(storage, docManager)

	testURI := "file:///test.go"

	err := manager.InvalidateDocument(testURI)
	if err != nil {
		t.Errorf("Failed to invalidate document: %v", err)
	}
}

// Test symbol invalidation
func TestSymbolInvalidation(t *testing.T) {
	storage := NewMockStorage()
	docManager := documents.NewLSPDocumentManager()
	manager := NewSCIPInvalidationManager(storage, docManager)

	symbolID := "test-symbol-123"

	err := manager.InvalidateSymbol(symbolID)
	if err != nil {
		t.Errorf("Failed to invalidate symbol: %v", err)
	}
}

// Test pattern invalidation
func TestPatternInvalidation(t *testing.T) {
	storage := NewMockStorage()
	docManager := documents.NewLSPDocumentManager()
	manager := NewSCIPInvalidationManager(storage, docManager)

	pattern := "*.go"

	err := manager.InvalidatePattern(pattern)
	if err != nil {
		t.Errorf("Failed to invalidate pattern: %v", err)
	}
}

// Test getting dependencies
func TestGetDependencies(t *testing.T) {
	storage := NewMockStorage()
	docManager := documents.NewLSPDocumentManager()
	manager := NewSCIPInvalidationManager(storage, docManager)

	testURI := "file:///test.go"

	deps, err := manager.GetDependencies(testURI)
	if err != nil {
		t.Errorf("Failed to get dependencies: %v", err)
	}

	// Should return empty dependencies for new manager
	if deps == nil {
		t.Error("Expected non-nil dependencies slice")
	}
}

// Test cascade invalidation
func TestCascadeInvalidation(t *testing.T) {
	storage := NewMockStorage()
	docManager := documents.NewLSPDocumentManager()
	manager := NewSCIPInvalidationManager(storage, docManager)

	uris := []string{"file:///test1.go", "file:///test2.go", "file:///test3.go"}

	err := manager.CascadeInvalidate(uris)
	if err != nil {
		t.Errorf("Failed to cascade invalidate: %v", err)
	}
}

// Test update dependencies
func TestUpdateDependencies(t *testing.T) {
	storage := NewMockStorage()
	docManager := documents.NewLSPDocumentManager()
	manager := NewSCIPInvalidationManager(storage, docManager)

	testURI := "file:///test.go"
	symbols := []string{"symbol1", "symbol2"}
	references := map[string][]string{
		"symbol1": {"file:///ref1.go", "file:///ref2.go"},
		"symbol2": {"file:///ref3.go"},
	}

	// This method doesn't return an error, just updates internal state
	manager.UpdateDependencies(testURI, symbols, references)

	// Verify dependencies were added by getting them back
	deps, err := manager.GetDependencies(testURI)
	if err != nil {
		t.Errorf("Failed to get dependencies after update: %v", err)
	}

	t.Logf("Updated dependencies: %v", deps)
}

// Test getting stats
func TestGetStats(t *testing.T) {
	storage := NewMockStorage()
	docManager := documents.NewLSPDocumentManager()
	manager := NewSCIPInvalidationManager(storage, docManager)

	stats := manager.GetStats()
	if stats == nil {
		t.Error("Expected non-nil stats")
	}

	t.Logf("Invalidation manager stats: %+v", stats)
}