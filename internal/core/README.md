# Shared Core Module - Architecture & Implementation Guide

## Overview

The Shared Core module provides a unified foundation for both HTTP and MCP servers in the LSP Gateway system. It implements a dependency injection pattern with hierarchical configuration management, intelligent caching, and comprehensive resource management.

## Architecture Components

### 1. Core Interfaces (`interfaces.go`)

**SharedCore Interface**
- Central orchestrator managing all core components
- Provides unified access to configuration, registry, cache, and project management
- Implements health monitoring and metrics collection
- Thread-safe with proper lifecycle management

**Component Interfaces**
- `ConfigManager`: Hierarchical configuration (global → project → client)
- `LSPServerRegistry`: LSP server lifecycle and connection pooling  
- `SCIPCache`: Intelligent symbol/reference caching with L1/L2 strategy
- `ProjectManager`: Project detection and workspace management

### 2. Dependency Injection (`dependency_injection.go`)

**Container-Based DI Pattern**
- Factory-based component creation with dependency resolution
- Topological sorting for proper initialization order
- Support for custom factory registration
- Graceful shutdown with cleanup orchestration

**Component Factories**
- Configurable implementation selection (file/memory/hybrid configs)
- Automatic dependency injection based on factory declarations
- Error handling with rollback capabilities

### 3. Implementation (`shared_core.go`)

**DefaultSharedCore**
- Coordinates all components with proper state management
- Background monitoring with health checks and metrics collection
- Periodic cleanup and memory pressure management
- Circuit breaker pattern for component failure handling

**Lifecycle Management**
- Initialize → Start → Stop sequence with proper error handling
- Component shutdown in reverse dependency order
- Resource cleanup with timeout handling

### 4. Memory Management (`memory_management.go`)

**Memory Manager**
- Automatic memory pressure detection and cleanup
- Object pooling for frequently allocated types (Symbol, Reference, Location)
- Multiple cleanup strategies with priority ordering
- Runtime memory monitoring with GC triggering

**Cleanup Strategies**
1. Cache eviction (highest priority)
2. Idle workspace cleanup
3. Idle server cleanup  
4. Object pool cleanup (lowest priority)

## Configuration Schema

### Hierarchical Configuration

```
Global Config (system-wide defaults)
├── Project Config (workspace-specific overrides)
    └── Client Config (client-specific preferences)
        → Resolved Config (final merged configuration)
```

**Configuration Inheritance**
- Client settings override project settings
- Project settings override global settings  
- Validation ensures configuration consistency
- Real-time configuration watching with change notifications

### Configuration Types

**Global Configuration**
- Server defaults and timeout settings
- Cache configuration and resource limits
- Feature flags and logging settings
- System-wide memory and connection limits

**Project Configuration**  
- Project-specific server overrides
- Source/exclude directory specifications
- Language-specific settings and build commands
- Workspace-scoped feature toggles

**Client Configuration**
- Preferred/disabled server lists
- Performance settings and concurrency limits  
- Feature preferences and UI settings
- Client-specific timeout overrides

## Implementation Guidelines

### 1. Component Development

**Interface Implementation**
```go
// Example: Implementing ConfigManager
type FileConfigManager struct {
    mu         sync.RWMutex
    globalCfg  *GlobalConfig
    projectCfgs map[string]*ProjectConfig
    clientCfgs  map[string]*ClientConfig
    // ... implementation
}

func (f *FileConfigManager) GetServerConfig(serverName, workspaceID string) (*ServerConfig, error) {
    // Implement hierarchical resolution logic
    // 1. Get global config
    // 2. Get project config (if exists)
    // 3. Get client config (if exists) 
    // 4. Merge with proper precedence
}
```

**Factory Registration**
```go
// Register custom factory
container.Register("config", NewCustomConfigManagerFactory("hybrid"))

// Or use global registration
RegisterGlobalFactory("cache", NewRedisCacheFactory())
```

### 2. Integration Patterns

**HTTP Server Integration**
```go
// In HTTP gateway initialization
container := core.NewContainer(containerConfig)
if err := container.Initialize(ctx); err != nil {
    return fmt.Errorf("failed to initialize shared core: %w", err)
}

sharedCore, _ := container.GetSharedCore()
gateway := NewHTTPGateway(sharedCore, httpConfig)
```

**MCP Server Integration**
```go
// In MCP server initialization  
sharedCore, _ := container.GetSharedCore()
mcpServer := NewMCPServer(sharedCore, mcpConfig)

// Access components through shared core
configMgr := sharedCore.Config()
projectMgr := sharedCore.Projects()
```

### 3. Thread Safety Patterns

**Component Access**
- All component interfaces are thread-safe
- Use RWMutex for read-heavy operations
- Atomic operations for counters and flags
- Channel-based communication for background processes

**Resource Management**
- Proper cleanup in defer statements
- Context propagation for cancellation
- Timeout handling for long-running operations
- Resource pooling to minimize allocations

### 4. Memory Management

**Object Pooling**
```go
// Use memory manager for frequent allocations
symbol := memoryMgr.GetSymbol()
defer memoryMgr.PutSymbol(symbol)

// Populate and use symbol
symbol.ID = "symbol_id"  
symbol.Name = "function_name"
```

**Memory Pressure Handling**
- Automatic cleanup when memory usage > 85%
- GC triggering when memory usage > 75%
- Component-specific memory estimation
- Configurable cleanup strategies

## Deployment Configuration

### Container Configuration
```go
containerConfig := &ContainerConfig{
    ConfigManagerType:  "hybrid",     // file + memory caching
    RegistryType:       "pooled",     // connection pooling
    CacheType:          "hybrid",     // L1 memory + L2 disk
    ProjectManagerType: "enhanced",   // full feature set
    
    MaxMemoryUsage:     1024 * 1024 * 1024, // 1GB
    MaxWorkspaces:      100,
    MaxServers:         50,
    
    InitTimeout:        30 * time.Second,
    ShutdownTimeout:    10 * time.Second,
}
```

### Component Configurations
```go
componentConfigs := map[string]interface{}{
    "cache": &CacheConfig{
        MemoryCacheSize:   256 * 1024 * 1024, // 256MB
        MemoryTTL:         30 * time.Minute,
        DiskCacheEnabled:  true,
        DiskCacheSize:     1024 * 1024 * 1024, // 1GB
        DiskCachePath:     "/tmp/lsp-cache",
    },
    "registry": &RegistryConfig{
        MaxConnectionsPerServer: 10,
        ConnectionTimeout:       30 * time.Second,
        HealthCheckInterval:     60 * time.Second,
    },
}
```

## Testing Strategy

### Unit Testing
- Mock interfaces for component isolation
- Factory testing with dependency verification
- Configuration merging and validation tests
- Memory management and cleanup verification

### Integration Testing
- Component interaction testing
- Configuration hierarchy validation
- Resource cleanup and lifecycle testing
- Memory pressure and recovery testing

### Performance Testing
- Cache hit rate optimization
- Memory usage benchmarking
- Concurrent access performance
- Cleanup strategy effectiveness

## Migration Path

### From Existing Architecture
1. **Phase 1**: Implement core interfaces and container
2. **Phase 2**: Migrate configuration management
3. **Phase 3**: Integrate LSP server registry
4. **Phase 4**: Add SCIP caching layer
5. **Phase 5**: Complete project management integration

### Backward Compatibility
- Gradual migration with feature flags
- Legacy adapter patterns for existing code
- Configuration conversion utilities
- Rollback capabilities for each phase

## Monitoring & Observability

### Health Checks
- Component-level health monitoring
- Cascading health status reporting
- Configurable health check intervals
- Health endpoint for external monitoring

### Metrics Collection
- Memory usage and allocation patterns
- Cache hit rates and performance metrics
- Server connection pool utilization
- Workspace activity and cleanup metrics

### Logging Integration
- Structured logging with context propagation
- Configurable log levels per component
- Performance logging for optimization
- Error tracking with stack traces

## Production Considerations

### Resource Limits
- Configurable memory limits with enforcement
- Connection pool sizing based on workload
- Disk space monitoring for cache storage
- CPU usage tracking for background processes

### Error Handling
- Circuit breaker pattern for component failures
- Graceful degradation with fallback modes
- Error propagation with context preservation
- Recovery mechanisms for transient failures

### Security
- Configuration validation and sanitization
- Resource access control and isolation
- Secure cleanup of sensitive data
- Audit logging for configuration changes