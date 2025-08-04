package cache

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestSCIPInvalidationManager_BasicOperations(t *testing.T) {
	storage := NewMockStorage()
	docManager := documents.NewLSPDocumentManager()

	manager := NewSCIPInvalidationManager(storage, docManager)
	require.NotNil(t, manager)

	ctx := context.Background()

	// Test starting the manager
	err := manager.Start(ctx)
	require.NoError(t, err)
	assert.True(t, manager.started)

	// Test invalidating a document
	uri := "file:///test.go"
	err = manager.InvalidateDocument(uri)
	assert.NoError(t, err)

	// Test getting dependencies (should be empty initially)
	deps, err := manager.GetDependencies(uri)
	assert.NoError(t, err)
	assert.Empty(t, deps)

	// Test updating dependencies
	symbols := []string{"symbol1", "symbol2"}
	references := map[string][]string{
		"symbol1": {"file:///ref1.go", "file:///ref2.go"},
		"symbol2": {"file:///ref3.go"},
	}

	manager.UpdateDependencies(uri, symbols, references)

	// Test getting dependencies after update
	deps, err = manager.GetDependencies(uri)
	assert.NoError(t, err)
	assert.Contains(t, deps, "file:///ref1.go")
	assert.Contains(t, deps, "file:///ref2.go")
	assert.Contains(t, deps, "file:///ref3.go")

	// Test cascade invalidation
	err = manager.CascadeInvalidate([]string{uri})
	assert.NoError(t, err)

	// Test getting stats
	stats := manager.GetStats()
	assert.NotNil(t, stats)
	assert.Contains(t, stats, "tracked_files")
	assert.Contains(t, stats, "tracked_symbols")

	// Stop the manager
	err = manager.Stop()
	assert.NoError(t, err)
	assert.False(t, manager.started)
}

func TestSCIPInvalidationManager_FileWatcher(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "scip_test_")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage := NewMockStorage()
	docManager := documents.NewLSPDocumentManager()

	manager := NewSCIPInvalidationManager(storage, docManager)
	require.NotNil(t, manager)

	ctx := context.Background()

	// Start the manager
	err = manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop()

	// Setup file watcher
	err = manager.SetupFileWatcher(tmpDir)
	assert.NoError(t, err)

	// Verify the project root was set
	assert.Equal(t, tmpDir, manager.projectRoot)

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.go")
	err = os.WriteFile(testFile, []byte("package main\n\nfunc main() {}\n"), 0644)
	require.NoError(t, err)

	// Give the file watcher a moment to process
	time.Sleep(100 * time.Millisecond)

	// Check that the file is considered relevant
	assert.True(t, manager.isRelevantFile(testFile))

	// Test with irrelevant file
	irrelevantFile := filepath.Join(tmpDir, "test.txt")
	assert.False(t, manager.isRelevantFile(irrelevantFile))
}

func TestSCIPInvalidationManager_DependencyGraph(t *testing.T) {
	storage := NewMockStorage()
	docManager := documents.NewLSPDocumentManager()

	manager := NewSCIPInvalidationManager(storage, docManager)
	require.NotNil(t, manager)

	// Test dependency graph operations
	uri1 := "file:///main.go"
	uri2 := "file:///utils.go"
	uri3 := "file:///handler.go"

	// Add dependencies
	symbols1 := []string{"main", "init"}
	references1 := map[string][]string{
		"main": {uri2, uri3},
		"init": {uri2},
	}

	manager.UpdateDependencies(uri1, symbols1, references1)

	symbols2 := []string{"UtilFunc", "Helper"}
	references2 := map[string][]string{
		"UtilFunc": {uri3},
	}

	manager.UpdateDependencies(uri2, symbols2, references2)

	// Test getting dependencies
	deps1, err := manager.GetDependencies(uri1)
	assert.NoError(t, err)
	assert.Contains(t, deps1, uri2)
	assert.Contains(t, deps1, uri3)

	deps2, err := manager.GetDependencies(uri2)
	assert.NoError(t, err)
	assert.Contains(t, deps2, uri3)
	assert.Contains(t, deps2, uri1) // Reverse dependency

	// Test removing from dependency graph
	manager.removeFromDepGraph(uri1)

	// Verify uri1 is removed
	deps1After, err := manager.GetDependencies(uri1)
	assert.NoError(t, err)
	assert.Empty(t, deps1After)

	// Verify other files no longer reference uri1
	deps2After, err := manager.GetDependencies(uri2)
	assert.NoError(t, err)
	assert.NotContains(t, deps2After, uri1)
}

func TestSCIPInvalidationManager_PatternMatching(t *testing.T) {
	storage := NewMockStorage()
	docManager := documents.NewLSPDocumentManager()

	manager := NewSCIPInvalidationManager(storage, docManager)
	require.NotNil(t, manager)

	// Test file relevance checking
	testCases := []struct {
		filename string
		relevant bool
	}{
		{"test.go", true},
		{"main.py", true},
		{"app.js", true},
		{"component.jsx", true},
		{"types.ts", true},
		{"Component.tsx", true},
		{"Main.java", true},
		{"readme.txt", false},
		{"config.yaml", false},
		{"Dockerfile", false},
		{".gitignore", false},
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			result := manager.isRelevantFile(tc.filename)
			assert.Equal(t, tc.relevant, result, "File %s relevance mismatch", tc.filename)
		})
	}
}

func TestSCIPInvalidationManager_CascadeInvalidation(t *testing.T) {
	storage := NewMockStorage()
	docManager := documents.NewLSPDocumentManager()

	manager := NewSCIPInvalidationManager(storage, docManager)
	require.NotNil(t, manager)

	// Create a dependency chain: A -> B -> C
	uriA := "file:///a.go"
	uriB := "file:///b.go"
	uriC := "file:///c.go"

	// A depends on B
	manager.UpdateDependencies(uriA, []string{"funcA"}, map[string][]string{
		"funcA": {uriB},
	})

	// B depends on C
	manager.UpdateDependencies(uriB, []string{"funcB"}, map[string][]string{
		"funcB": {uriC},
	})

	// Test cascade invalidation starting from A
	err := manager.CascadeInvalidate([]string{uriA})
	assert.NoError(t, err)

	// Test with empty list
	err = manager.CascadeInvalidate([]string{})
	assert.NoError(t, err)

	// Test with multiple starting points
	err = manager.CascadeInvalidate([]string{uriA, uriB})
	assert.NoError(t, err)
}

func TestSCIPInvalidationManager_Statistics(t *testing.T) {
	storage := NewMockStorage()
	docManager := documents.NewLSPDocumentManager()

	manager := NewSCIPInvalidationManager(storage, docManager)
	require.NotNil(t, manager)

	// Add some test data
	manager.projectRoot = "/test/project"
	manager.watchedPaths["/test/project"] = true
	manager.watchedPaths["/test/project/src"] = true

	// Add some dependencies
	manager.UpdateDependencies("file:///test.go", []string{"symbol1"}, map[string][]string{
		"symbol1": {"file:///dep.go"},
	})

	// Get statistics
	stats := manager.GetStats()

	// Verify expected fields are present
	assert.Contains(t, stats, "watched_paths")
	assert.Contains(t, stats, "tracked_files")
	assert.Contains(t, stats, "tracked_symbols")
	assert.Contains(t, stats, "dependency_edges")
	assert.Contains(t, stats, "project_root")
	assert.Contains(t, stats, "watch_patterns")
	assert.Contains(t, stats, "debounce_delay_ms")

	// Verify values
	assert.Equal(t, 2, stats["watched_paths"])
	assert.Equal(t, 1, stats["tracked_files"])
	assert.Equal(t, 1, stats["tracked_symbols"])
	assert.Equal(t, "/test/project", stats["project_root"])
	assert.Equal(t, int64(200), stats["debounce_delay_ms"])
}
