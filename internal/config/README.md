# LSP Gateway Configuration System

This directory contains a comprehensive multi-language configuration system for the LSP Gateway project. The system provides flexible, optimized configurations for different environments and project types.

## Overview

The configuration system consists of several key components:

1. **Core Configuration (`config.go`)** - Main configuration structures and methods
2. **Multi-Server Support (`multi_server.go`)** - Language server pool management
3. **Multi-Language Support (`multi_language.go`, `multi_language_generator.go`)** - Multi-language project configuration
4. **Optimization Strategies (`optimization.go`)** - Environment-specific optimizations
5. **Integration Utilities (`integration.go`)** - Configuration migration and integration
6. **Framework Enhancers (`enhancers.go`)** - Framework-specific enhancements
7. **Validation (`validation.go`)** - Configuration validation and consistency checks
8. **Constants (`constants.go`)** - System constants and defaults

## Key Features

### Multi-Language Project Support

- **Automatic Language Detection**: Detects languages in project directories
- **Framework-Aware Configuration**: Optimizes settings based on detected frameworks
- **Cross-Language Integration**: Supports polyglot projects with cross-language references
- **Project Type Classification**: Handles monorepos, microservices, frontend-backend, etc.

### Optimization Strategies

Three main optimization strategies are available:

#### Production Optimization
- **Memory Conservative**: Reduces memory usage with degraded features
- **Performance Focused**: Optimizes for speed and responsiveness
- **Reduced Analysis**: Disables expensive analyses for better performance
- **Resource Limits**: Sets conservative resource limits

```go
prodOpt := config.NewProductionOptimization()
err := prodOpt.ApplyOptimizations(mlConfig)
```

#### Analysis Optimization
- **Comprehensive Analysis**: Enables all available analyses
- **Deep Inspection**: Maximum diagnostic capabilities
- **Strict Mode**: Enforces strict type checking and validation
- **Extended Memory**: Allows higher memory usage for thorough analysis

```go
analysisOpt := config.NewAnalysisOptimization()
err := analysisOpt.ApplyOptimizations(mlConfig)
```

#### Development Optimization
- **Developer-Friendly**: Balances features with performance
- **Interactive Features**: Enables helpful code lenses and hover info
- **Responsive UI**: Optimizes for development workflow
- **Moderate Resource Usage**: Reasonable resource limits for development

```go
devOpt := config.NewDevelopmentOptimization()
err := devOpt.ApplyOptimizations(mlConfig)
```

### Multi-Server Architecture

The system supports multiple language servers per language for:

- **Load Balancing**: Distribute requests across multiple servers
- **Fallback Support**: Automatic failover to secondary servers
- **Feature Specialization**: Use different servers for different capabilities
- **Resource Management**: Pool-based resource allocation

### Framework Enhancement

Built-in enhancers for popular frameworks:

- **React**: TypeScript/JavaScript optimization for React projects
- **Django**: Python-specific Django settings and completions
- **Spring Boot**: Java enterprise development optimizations
- **Kubernetes**: Go/YAML configurations for K8s development

### Configuration Migration

Automatic migration from legacy configuration formats:

- **Legacy Gateway Config**: Migrates from old GatewayConfig format
- **Simple Config**: Handles basic language list configurations
- **Version Upgrades**: Seamless configuration format upgrades

## Usage Examples

### Basic Usage

```go
// Generate configuration from project path
config, err := config.AutoGenerateConfigFromPath("/path/to/project")
if err != nil {
    log.Fatal(err)
}

// Apply optimization for production
manager := config.NewOptimizationManager()
err = manager.ApplyOptimization(config, "production")
if err != nil {
    log.Fatal(err)
}

// Save configuration
err = config.WriteYAML("lsp-gateway-config.yaml")
if err != nil {
    log.Fatal(err)
}
```

### Advanced Integration

```go
// Load and migrate legacy configuration
integrator := config.NewConfigurationIntegrator()
migratedConfig, err := integrator.MigrateConfiguration("legacy-config.yaml")
if err != nil {
    log.Fatal(err)
}

// Generate enhanced configuration with optimization
enhancedConfig, err := integrator.GenerateEnhancedConfiguration(
    "/path/to/project", 
    "development",
)
if err != nil {
    log.Fatal(err)
}

// Integrate multiple configurations
integrated, err := integrator.IntegrateConfigurations(
    migratedConfig, 
    enhancedConfig,
)
if err != nil {
    log.Fatal(err)
}

// Convert to gateway config for runtime use
gatewayConfig, err := integrator.ConvertToGatewayConfig(integrated)
if err != nil {
    log.Fatal(err)
}
```

### Custom Optimization

```go
// Create custom optimization strategy
type CustomOptimization struct {
    TargetLanguage string
    CustomSettings map[string]interface{}
}

func (c *CustomOptimization) ApplyOptimizations(config *config.MultiLanguageConfig) error {
    // Custom optimization logic
    for _, serverConfig := range config.ServerConfigs {
        if contains(serverConfig.Languages, c.TargetLanguage) {
            // Apply custom settings
            for key, value := range c.CustomSettings {
                serverConfig.Settings[key] = value
            }
        }
    }
    return nil
}

// Register custom strategy
manager := config.NewOptimizationManager()
manager.RegisterStrategy("custom", &CustomOptimization{
    TargetLanguage: "rust",
    CustomSettings: map[string]interface{}{
        "custom_setting": "custom_value",
    },
})
```

## Configuration File Formats

### YAML Configuration

```yaml
project_info:
  project_type: "multi-language"
  root_directory: "/path/to/project"
  language_contexts:
    - language: "go"
      file_patterns: ["*.go"]
      file_count: 25
      root_markers: ["go.mod"]
      frameworks: ["gin"]
    - language: "typescript" 
      file_patterns: ["*.ts", "*.tsx"]
      file_count: 40
      root_markers: ["tsconfig.json"]
      frameworks: ["react"]

servers:
  - name: "gopls"
    languages: ["go"]
    command: "gopls"
    transport: "stdio"
    priority: 10
    settings:
      gopls:
        analyses:
          unusedparams: true
          shadow: true

workspace:
  multi_root: true
  cross_language_references: true
  indexing_strategy: "incremental"

optimized_for: "development"
```

### JSON Configuration

```json
{
  "project_info": {
    "project_type": "multi-language",
    "root_directory": "/path/to/project",
    "language_contexts": [
      {
        "language": "go",
        "file_patterns": ["*.go"],
        "file_count": 25,
        "root_markers": ["go.mod"],
        "frameworks": ["gin"]
      }
    ]
  },
  "servers": [
    {
      "name": "gopls",
      "languages": ["go"],
      "command": "gopls",
      "transport": "stdio",
      "priority": 10
    }
  ],
  "optimized_for": "development"
}
```

## Architecture

### Configuration Layers

1. **Base Configuration**: Core server definitions and basic settings
2. **Language Context**: Language-specific detection and metadata
3. **Framework Enhancement**: Framework-specific optimizations
4. **Optimization Strategy**: Environment-specific tuning
5. **Project Integration**: Multi-project and workspace configuration
6. **Runtime Gateway**: Final gateway configuration for server execution

### Data Flow

```
Project Detection → Language Contexts → Server Generation → Framework Enhancement → Optimization → Gateway Config
```

### Extension Points

The system provides several extension points:

- **Custom Optimization Strategies**: Implement `OptimizationStrategy` interface
- **Framework Enhancers**: Implement `FrameworkEnhancer` interface  
- **Migration Handlers**: Implement `MigrationHandler` interface
- **Validators**: Implement `ConfigValidator` interface
- **Monorepo Strategies**: Implement `MonorepoStrategy` interface

## Best Practices

### Configuration Management

1. **Use Optimization Strategies**: Always apply appropriate optimization for your environment
2. **Enable Framework Enhancement**: Let the system auto-detect and enhance framework settings
3. **Validate Configurations**: Use built-in validators to ensure configuration integrity
4. **Version Control Configs**: Store generated configurations in version control
5. **Migration-Friendly**: Use the migration system for configuration updates

### Performance Considerations

1. **Production Optimization**: Use production optimization for deployed systems
2. **Resource Limits**: Set appropriate resource limits based on system capacity  
3. **Pool Configuration**: Configure language server pools for high-load scenarios
4. **Monitoring**: Enable performance monitoring in production environments

### Development Workflow

1. **Auto-Generation**: Start with auto-generated configurations
2. **Incremental Enhancement**: Apply framework and optimization enhancements
3. **Testing**: Use analysis optimization for comprehensive code analysis
4. **Integration**: Use configuration integration for complex project setups

## Testing

Comprehensive test coverage is provided in:

- `tests/unit/optimization_integration_test.go` - Integration and optimization tests
- `internal/config/multi_server_test.go` - Multi-server functionality tests
- Individual unit tests for each component

Run tests with:

```bash
go test ./internal/config/...
go test ./tests/unit/...
```

## Migration Guide

### From Legacy Gateway Config

```go
// Automatic migration
integrator := config.NewConfigurationIntegrator()
newConfig, err := integrator.MigrateConfiguration("old-gateway-config.yaml")
```

### From Simple Language Lists

```go
// For configurations with just language arrays
simpleConfig := map[string]interface{}{
    "languages": []string{"go", "python", "typescript"},
}

handler := &config.SimpleConfigMigrationHandler{}
migratedConfig, err := handler.Migrate(simpleConfig)
```

## Future Enhancements

Planned improvements include:

- **AI-Powered Optimization**: Machine learning-based configuration tuning
- **Cloud Integration**: Support for cloud-based language servers
- **Container Support**: Docker and Kubernetes integration
- **Real-time Adaptation**: Dynamic configuration adjustment based on usage patterns
- **Advanced Analytics**: Detailed performance and usage analytics

## Contributing

When contributing to the configuration system:

1. **Follow Patterns**: Use existing patterns for new components
2. **Add Tests**: Include comprehensive tests for new functionality
3. **Update Documentation**: Keep documentation current with changes
4. **Backward Compatibility**: Maintain compatibility with existing configurations
5. **Performance**: Consider performance impact of new features

## Support

For questions or issues:

1. Check existing tests for usage examples
2. Review configuration validation error messages
3. Use the migration system for format compatibility
4. Consult the integration utilities for complex scenarios