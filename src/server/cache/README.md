# Simple Cache System

Simple in-memory cache system for LSP responses, designed for local development tool usage with basic caching functionality.

## Overview

The Simple Cache System implements basic in-memory caching for LSP responses to improve performance for repeated queries. It provides essential caching functionality without enterprise complexity, suitable for individual developer usage.

## Performance Characteristics

The simple cache system provides:

- **Cache Hit Performance**: Sub-millisecond response for cached entries
- **Memory Usage**: Basic in-memory storage with simple LRU eviction
- **TTL Management**: 24-hour default time-to-live for cached responses
- **File Change Detection**: Simple modification time checking for invalidation

## Architecture

### Core Components

1. **SimpleCacheManager** - Basic in-memory cache with TTL support
2. **SimpleIndexer** - Direct synchronous document indexing
3. **SimpleInvalidationManager** - File change detection and cache invalidation
4. **SimpleCacheIntegration** - Cache-first, LSP-fallback pattern

### Storage Structure

```go
type SimpleCacheManager struct {
    entries map[string]*CacheEntry    // Simple key-value storage
    config  *config.CacheConfig      // Unified cache configuration
    stats   *SimpleCacheStats        // Hit/miss counters
    mu      sync.RWMutex             // Thread safety
}

type CacheEntry struct {
    Key        CacheKey             // Method + URI + parameter hash
    Response   interface{}          // Cached LSP response
    Timestamp  time.Time            // Creation time
    AccessedAt time.Time            // Last access time
    Size       int64                // Entry size for eviction
}
```

### Supported LSP Methods

All 6 essential LSP methods are supported with basic caching:

1. **textDocument/definition** - Go to definition
2. **textDocument/references** - Find all references  
3. **textDocument/hover** - Show documentation on hover
4. **textDocument/documentSymbol** - Document outline/symbols
5. **workspace/symbol** - Workspace-wide symbol search
6. **textDocument/completion** - Code completion

## Usage

### Basic Integration

```go
// Create simple cache manager with unified config
cacheManager, err := cache.NewSCIPCacheManager(config.GetDefaultCacheConfig())
if err != nil {
    log.Fatal(err)
}

// Initialize and start cache
err = cacheManager.Initialize(nil)
if err != nil {
    log.Fatal(err)
}

err = cacheManager.Start(ctx)
if err != nil {
    log.Fatal(err)
}

// Use with LSP manager
result, found, err := cacheManager.Lookup("textDocument/definition", params)
if !found {
    // Fallback to LSP server and cache result
    result, err = lspManager.ProcessRequest(ctx, "textDocument/definition", params)
    if err == nil {
        cacheManager.Store("textDocument/definition", params, result)
    }
}
```

### Integration with SimpleCacheIntegration

```go
// Create cache integration wrapper
cacheIntegration := &cache.SimpleCacheIntegration{
    lspManager: existingLSPManager,
    cache:      cacheManager,
    enabled:    true,
}

// Use as drop-in replacement with cache-first fallback
result, err := cacheIntegration.ProcessRequest(ctx, "textDocument/definition", params)
```

### Performance Monitoring

```go
// Get basic performance metrics
metrics := cacheManager.GetMetrics()
fmt.Printf("Cache hits: %d\n", metrics.HitCount)
fmt.Printf("Cache misses: %d\n", metrics.MissCount)
fmt.Printf("Total entries: %d\n", metrics.EntryCount)
fmt.Printf("Total size: %d bytes\n", metrics.TotalSize)

// Check if cache is enabled
if cacheManager.IsEnabled() {
    fmt.Println("Cache is enabled and running")
}

// Get health status
health, err := cacheManager.HealthCheck()
if err == nil {
    fmt.Printf("Health status: %s\n", health.HealthStatus)
}
```

## Configuration

### Basic Configuration Options

```go
config := &config.CacheConfig{
    Enabled:            true,               // Enable/disable caching
    MaxMemoryMB:        256,                // 256MB cache size limit (simple units)
    TTLHours:           24,                 // Cache entry time-to-live in hours (simple units)
    EvictionPolicy:     "lru",              // LRU eviction policy
    BackgroundIndex:    true,               // Background indexing enabled
    HealthCheckMinutes: 5,                  // Health check interval in minutes (simple units)
    DiskCache:          false,              // In-memory cache only
}
```

### File Change Detection

```go
// Simple invalidation manager setup
invalidationMgr := cache.NewSimpleInvalidationManager(storage, docManager)

// Check if file changed and invalidate if needed
invalidationMgr.InvalidateIfChanged("file:///path/to/file.go")
```

## Error Handling

The simple system includes basic error handling:

- **Graceful Degradation**: Falls back to LSP servers on cache miss
- **Simple Validation**: Basic TTL and size-based cache management  
- **File Change Detection**: Simple modification time checking
- **Memory Management**: Basic LRU eviction when cache is full

## Implementation Details

### Cache Operation Flow

1. **Request Processing**: Check cache for existing response
2. **Cache Miss Handling**: Query LSP server and store result
3. **File Change Detection**: Invalidate cache on file modifications
4. **Simple Eviction**: Remove oldest entries when cache is full

### Memory Management

- **In-Memory Storage**: Simple map-based storage for fast access
- **LRU Eviction**: Remove oldest entries when size limit is reached
- **TTL Validation**: Automatic expiry of stale cache entries
- **Size Tracking**: Basic size monitoring for eviction decisions

### Thread Safety

- **Read-Write Locks**: Safe concurrent access to cache entries
- **Simple Synchronization**: Basic mutex protection for critical sections
- **No Background Threads**: Synchronous operations only

## Testing

Basic tests are included for the simple cache system:

```bash
# Run cache tests
go test -v ./src/server/cache/...

# Run basic functionality tests
go test -v ./src/server/cache/... -run TestSimple
```

### Test Coverage

- ✅ Basic cache operations (store, lookup, eviction)
- ✅ TTL validation and expiry
- ✅ File change detection and invalidation
- ✅ Simple memory management
- ✅ Thread safety for concurrent access

## Integration with LSP Gateway

The Simple Cache System integrates with the LSP Gateway architecture:

1. **Cache-First Pattern**: Try cache first, fallback to LSP on miss
2. **Method Support**: Supports all 6 essential LSP methods
3. **Language Support**: Works with Go, Python, JavaScript/TypeScript, Java
4. **MCP Integration**: Compatible with MCP server for AI assistant usage
5. **HTTP Gateway**: Works with existing HTTP JSON-RPC gateway
6. **Local Development Focus**: Designed for individual developer usage

## Architecture Philosophy

The simple cache system prioritizes:

- **Simplicity**: No complex enterprise patterns or background processing
- **Local Development**: Focused on individual developer productivity
- **Essential Features**: Core caching without unnecessary complexity
- **Maintainability**: Easy to understand and modify code
- **Resource Efficiency**: Minimal overhead for local development usage