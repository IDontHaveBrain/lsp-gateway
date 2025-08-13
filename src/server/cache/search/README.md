# SearchService Architecture

## Overview

The SearchService provides a unified architecture for consolidating duplicate search patterns across the LSP Gateway cache system. It eliminates code duplication from definition_search.go, reference_search.go, and symbol_search.go while maintaining backward compatibility.

## Key Benefits

- **Unified Patterns**: Consolidates guard checks, lock management, and storage access
- **Type Safety**: Clear interfaces and structured request/response models
- **Extensibility**: Modular handler system for different search types
- **Backward Compatibility**: Works with existing SCIPCacheManager interface
- **Clean Separation**: Separates search logic from cache management concerns

## Architecture Components

### Core Types

- `SearchRequest`: Unified request structure for all search operations
- `SearchResult`: Unified response structure with metadata and timing
- `SearchType`: Enum for different search operations (definition, reference, symbol, workspace)

### Main Components

1. **SearchService**: Main service orchestrating search operations
2. **SearchHandler**: Type-specific search implementations
3. **SearchFactory**: Factory for creating service instances
4. **ResultBuilder**: Utilities for building consistent results

### File Structure

```
src/server/cache/search/
├── service.go      # Main SearchService implementation
├── interfaces.go   # Interface definitions
├── factory.go      # Factory and builder patterns
├── handlers.go     # Type-specific search handlers
├── builders.go     # Result builders and utilities
└── README.md       # This documentation
```

## Usage Examples

### Basic Usage

```go
// Create search service configuration
config := &SearchServiceConfig{
    Storage:               scipStorage,
    Enabled:               true,
    IndexMutex:           &indexMutex,
    MatchFilePatternFn:   manager.matchFilePattern,
    BuildOccurrenceInfoFn: manager.buildOccurrenceInfo,
    FormatSymbolDetailFn: manager.formatSymbolDetail,
}

// Create search service
searchService := NewSearchService(config)

// Create search request
request := &SearchRequest{
    Type:        SearchTypeDefinition,
    SymbolName:  "MyFunction",
    FilePattern: "*.go",
    MaxResults:  100,
    Context:     ctx,
}

// Execute search
result, err := searchService.Search(request)
```

### Using Builder Pattern

```go
// Build search service
searchService, err := NewSearchServiceBuilder().
    WithStorage(scipStorage).
    WithEnabled(true).
    WithIndexMutex(&indexMutex).
    WithMatchFilePatternFn(manager.matchFilePattern).
    WithBuildOccurrenceInfoFn(manager.buildOccurrenceInfo).
    WithFormatSymbolDetailFn(manager.formatSymbolDetail).
    Build()

// Build search request
request, err := NewSearchRequestBuilder().
    WithType(SearchTypeReference).
    WithSymbolName("myVariable").
    WithFilePattern("src/**/*.ts").
    WithMaxResults(50).
    WithContext(ctx).
    Build()
```

### Using Factory

```go
factory := GetSearchServiceFactory()
searchService := factory.CreateSearchService(config)
handler := factory.CreateSearchHandler(SearchTypeSymbol)
```

## Integration with SCIPCacheManager

The SearchService is designed to be integrated into the existing SCIPCacheManager without breaking changes:

```go
type SCIPCacheManager struct {
    // existing fields...
    searchService search.SearchServiceInterface
}

func (m *SCIPCacheManager) initializeSearchService() {
    config := &search.SearchServiceConfig{
        Storage:               m.scipStorage,
        Enabled:               m.enabled,
        IndexMutex:           &m.indexMu,
        MatchFilePatternFn:   m.matchFilePattern,
        BuildOccurrenceInfoFn: m.buildOccurrenceInfo,
        FormatSymbolDetailFn: m.formatSymbolDetail,
    }
    m.searchService = search.NewSearchService(config)
}

func (m *SCIPCacheManager) SearchDefinitions(ctx context.Context, symbolName string, filePattern string, maxResults int) ([]interface{}, error) {
    request := &search.SearchRequest{
        Type:        search.SearchTypeDefinition,
        SymbolName:  symbolName,
        FilePattern: filePattern,
        MaxResults:  maxResults,
        Context:     ctx,
    }
    
    result, err := m.searchService.Search(request)
    if err != nil {
        return nil, err
    }
    
    return result.Results, nil
}
```

## Design Patterns Used

### 1. Strategy Pattern
Different search handlers for each search type:
- `DefinitionSearchHandler`
- `ReferenceSearchHandler` 
- `SymbolSearchHandler`
- `WorkspaceSearchHandler`

### 2. Factory Pattern
Centralized creation of search services and handlers:
- `SearchFactory` interface
- `DefaultSearchFactory` implementation
- `SearchServiceFactory` singleton

### 3. Builder Pattern
Fluent interfaces for configuration:
- `SearchServiceBuilder`
- `SearchRequestBuilder`

### 4. Template Method Pattern
Common search execution flow with type-specific implementations:
- `SearchService.Search()` orchestrates
- Individual handlers implement specific logic

### 5. Guard Pattern
Consolidated guard checks:
- Enabled state validation
- Lock management
- Error handling

## Backward Compatibility

The SearchService maintains full backward compatibility with existing methods:

- All existing method signatures remain unchanged
- Return types and error handling stay consistent
- Metadata format preserved
- Performance characteristics maintained

## Performance Considerations

- **Reduced Lock Contention**: Unified lock management
- **Optimized Storage Access**: Consolidated storage operations
- **Memory Efficiency**: Shared result building patterns
- **Concurrent Safety**: Thread-safe by design

## Testing Strategy

- Unit tests for each search handler
- Integration tests with mock storage
- Performance benchmarks vs existing implementation
- Compatibility tests with existing cache manager

## Migration Path

1. **Phase 1**: Create SearchService infrastructure (current)
2. **Phase 2**: Integrate with SCIPCacheManager
3. **Phase 3**: Migrate existing search methods to use SearchService
4. **Phase 4**: Remove duplicate code from original search files
5. **Phase 5**: Optimize and enhance unified implementation

## Future Enhancements

- **Search Middleware**: Request/response processing pipeline
- **Metrics Collection**: Performance and usage monitoring
- **Request Validation**: Input sanitization and validation
- **Result Caching**: Cache frequently accessed search results
- **Async Search**: Non-blocking search operations