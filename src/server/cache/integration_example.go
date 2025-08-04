package cache

import (
	"context"
	"fmt"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server/documents"
)

// IntegrationExample demonstrates how to integrate the SCIP invalidation manager
// with the existing cache system for smart cache updates
func IntegrationExample() error {
	common.LSPLogger.Info("Starting SCIP Cache Integration Example")

	// 1. Create cache manager with default configuration
	config := DefaultSCIPConfig()
	cacheManager, err := NewSCIPCacheManager(config)
	if err != nil {
		return fmt.Errorf("failed to create cache manager: %w", err)
	}

	// 2. Create document manager
	docManager := documents.NewLSPDocumentManager()

	// 3. Create mock storage for this example
	storage := &mockStorageForExample{}

	// 4. Create invalidation manager
	invalidationManager := NewSCIPInvalidationManager(storage, docManager)

	// 5. Set up the integration
	cacheManager.SetInvalidation(invalidationManager)

	// 6. Initialize and start all components
	ctx := context.Background()

	// Initialize cache manager
	if err := cacheManager.Initialize(config); err != nil {
		return fmt.Errorf("failed to initialize cache manager: %w", err)
	}

	// Start cache manager
	if err := cacheManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start cache manager: %w", err)
	}
	defer cacheManager.Stop()

	// Start invalidation manager
	if err := invalidationManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start invalidation manager: %w", err)
	}
	defer invalidationManager.Stop()

	// 7. Set up file watcher for project
	projectRoot := "/example/project"
	if err := invalidationManager.SetupFileWatcher(projectRoot); err != nil {
		common.LSPLogger.Warn("Failed to setup file watcher: %v", err)
	}

	// 8. Demonstrate usage scenarios
	if err := demonstrateInvalidationScenarios(cacheManager, invalidationManager); err != nil {
		return fmt.Errorf("demonstration failed: %w", err)
	}

	common.LSPLogger.Info("SCIP Cache Integration Example completed successfully")
	return nil
}

// demonstrateInvalidationScenarios shows various invalidation use cases
func demonstrateInvalidationScenarios(cacheManager *SCIPCacheManager, invalidationManager *SCIPInvalidationManager) error {
	common.LSPLogger.Info("Demonstrating invalidation scenarios")

	// Scenario 1: Single file invalidation
	uri := "file:///example/main.go"
	common.LSPLogger.Info("Scenario 1: Invalidating single file: %s", uri)

	if err := invalidationManager.InvalidateDocument(uri); err != nil {
		return fmt.Errorf("failed to invalidate document: %w", err)
	}

	// Scenario 2: Symbol-based invalidation
	symbolID := "main.ExampleFunction"
	common.LSPLogger.Info("Scenario 2: Invalidating symbol: %s", symbolID)

	if err := invalidationManager.InvalidateSymbol(symbolID); err != nil {
		common.LSPLogger.Warn("Symbol invalidation failed (expected for demo): %v", err)
	}

	// Scenario 3: Pattern-based invalidation
	pattern := "textDocument/definition:file:///example/*.go:*"
	common.LSPLogger.Info("Scenario 3: Invalidating pattern: %s", pattern)

	if err := invalidationManager.InvalidatePattern(pattern); err != nil {
		return fmt.Errorf("failed to invalidate pattern: %w", err)
	}

	// Scenario 4: Dependency tracking and cascade invalidation
	common.LSPLogger.Info("Scenario 4: Setting up dependencies and cascade invalidation")

	// Simulate dependency relationships
	mainFile := "file:///example/main.go"
	utilsFile := "file:///example/utils.go"
	handlerFile := "file:///example/handler.go"

	// Main file depends on utils and handler
	mainSymbols := []string{"main", "init"}
	mainRefs := map[string][]string{
		"main": {utilsFile, handlerFile},
		"init": {utilsFile},
	}
	invalidationManager.UpdateDependencies(mainFile, mainSymbols, mainRefs)

	// Utils file has some dependencies
	utilsSymbols := []string{"UtilFunc", "Helper"}
	utilsRefs := map[string][]string{
		"UtilFunc": {handlerFile},
	}
	invalidationManager.UpdateDependencies(utilsFile, utilsSymbols, utilsRefs)

	// Get dependencies
	deps, err := invalidationManager.GetDependencies(mainFile)
	if err != nil {
		return fmt.Errorf("failed to get dependencies: %w", err)
	}
	common.LSPLogger.Info("Dependencies for %s: %v", mainFile, deps)

	// Perform cascade invalidation
	if err := invalidationManager.CascadeInvalidate([]string{mainFile}); err != nil {
		return fmt.Errorf("cascade invalidation failed: %w", err)
	}

	// Scenario 5: Monitor performance and health
	common.LSPLogger.Info("Scenario 5: Monitoring system health and performance")

	// Get cache metrics
	metrics := cacheManager.GetMetrics()
	common.LSPLogger.Info("Cache metrics: hits=%d, misses=%d, size=%d",
		metrics.HitCount, metrics.MissCount, metrics.TotalSize)

	// Get invalidation stats
	stats := invalidationManager.GetStats()
	common.LSPLogger.Info("Invalidation stats: watched_paths=%v, tracked_files=%v",
		stats["watched_paths"], stats["tracked_files"])

	return nil
}

// mockStorageForExample provides a simple storage implementation for demonstration
type mockStorageForExample struct {
	entries map[CacheKey]*CacheEntry
}

func (m *mockStorageForExample) Store(key CacheKey, entry *CacheEntry) error {
	if m.entries == nil {
		m.entries = make(map[CacheKey]*CacheEntry)
	}
	m.entries[key] = entry
	common.LSPLogger.Debug("Stored cache entry: %s", key.Method)
	return nil
}

func (m *mockStorageForExample) Retrieve(key CacheKey) (*CacheEntry, error) {
	if m.entries == nil {
		return nil, nil
	}
	entry, exists := m.entries[key]
	if !exists {
		return nil, nil
	}
	common.LSPLogger.Debug("Retrieved cache entry: %s", key.Method)
	return entry, nil
}

func (m *mockStorageForExample) Delete(key CacheKey) error {
	if m.entries != nil {
		delete(m.entries, key)
		common.LSPLogger.Debug("Deleted cache entry: %s", key.Method)
	}
	return nil
}

func (m *mockStorageForExample) Clear() error {
	m.entries = make(map[CacheKey]*CacheEntry)
	common.LSPLogger.Debug("Cleared all cache entries")
	return nil
}

func (m *mockStorageForExample) Size() int64 {
	var size int64
	for _, entry := range m.entries {
		size += entry.Size
	}
	return size
}

func (m *mockStorageForExample) EntryCount() int64 {
	return int64(len(m.entries))
}

// Usage recommendations and best practices
const UsageDocumentation = `
SCIP Invalidation Manager Usage Guide
=====================================

## Integration Steps

1. **Create Cache Manager**
   config := DefaultSCIPConfig()
   cacheManager, err := NewSCIPCacheManager(config)

2. **Create Document Manager**
   docManager := documents.NewLSPDocumentManager()

3. **Create Invalidation Manager**
   invalidationManager := NewSCIPInvalidationManager(storage, docManager)

4. **Connect Components**
   cacheManager.SetInvalidation(invalidationManager)

5. **Start Systems**
   cacheManager.Start(ctx)
   invalidationManager.Start(ctx)
   invalidationManager.SetupFileWatcher(projectRoot)

## Key Features

### File System Monitoring
- Automatically watches project directories for changes
- Filters relevant file types (go, py, js, ts, java)
- Debounces rapid file changes (200ms default)
- Handles file renames, moves, and deletions

### Dependency Tracking
- Tracks symbol dependencies across files
- Builds dependency graph for cascade invalidation
- Supports cross-language dependencies
- Memory-efficient graph storage

### Smart Invalidation Strategies
- **File-level**: Invalidate all symbols from changed file
- **Symbol-level**: Invalidate specific symbols and dependents
- **Cascade**: Invalidate dependent files and references
- **Pattern**: Bulk invalidation using patterns

### Performance Features
- File change detection: < 100ms
- Dependency analysis: < 1s for typical projects
- Cascade invalidation: < 500ms
- Memory efficient dependency graphs

## Best Practices

1. **Set up file watcher early** - Call SetupFileWatcher() after starting
2. **Update dependencies regularly** - Call UpdateDependencies() when indexing
3. **Monitor health** - Use GetStats() and GetMetrics() for monitoring
4. **Handle cascades carefully** - Cascade invalidation can be expensive
5. **Use appropriate patterns** - Pattern invalidation is powerful but broad

## Error Handling

- File system permission errors are logged but don't stop operation
- Missing file notifications are handled gracefully
- Corrupted dependency graphs are recovered automatically
- Resource exhaustion triggers emergency cleanup

## Integration Points

- **Document Manager**: Receives change notifications
- **Git Hooks**: Can trigger bulk invalidation on branch changes
- **IDE Integration**: VSCode/IDE file change notifications
- **Manual Triggers**: API endpoints for manual invalidation

## Performance Monitoring

Use these methods to monitor performance:
- invalidationManager.GetStats() - Dependency graph statistics
- cacheManager.GetMetrics() - Cache performance metrics
- cacheManager.HealthCheck() - Overall system health
`
