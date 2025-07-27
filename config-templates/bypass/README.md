# LSP Gateway Bypass Configuration Templates

This directory contains comprehensive bypass configuration templates for LSP Gateway, providing sensible defaults and language-specific strategies for handling LSP server failures gracefully.

## Overview

The bypass system in LSP Gateway provides resilient handling of LSP server failures through multiple strategies:

- **Fail Gracefully**: Return empty responses instead of errors
- **Fallback Server**: Switch to backup servers when primary fails
- **Cache Response**: Use cached data from previous successful requests
- **Circuit Breaker**: Temporarily stop requests to allow server recovery
- **Retry with Backoff**: Retry failed requests with exponential delays
- **Adaptive Strategy**: Dynamically choose strategies based on failure patterns

## Template Structure

```
config-templates/bypass/
├── README.md                           # This documentation
├── bypass-defaults.yaml                # Default bypass configuration
├── bypass-strategies.yaml              # Strategy definitions and guidelines
└── language-specific/
    ├── go-bypass.yaml                  # Go language-specific configuration
    ├── python-bypass.yaml              # Python language-specific configuration
    ├── typescript-bypass.yaml          # TypeScript language-specific configuration
    └── java-bypass.yaml                # Java language-specific configuration
```

## Configuration Files

### 1. `bypass-defaults.yaml`

The main bypass configuration template providing:

- **Global Settings**: Default strategies, timeouts, and recovery behavior
- **Server-Specific Settings**: Per-server bypass configurations
- **Language-Specific Settings**: Language-aware bypass strategies
- **Circuit Breaker Configuration**: Advanced failure detection
- **Monitoring and Notifications**: Bypass event tracking

**Key Features:**
- Comprehensive default settings for all bypass strategies
- Integration with existing LSP Gateway infrastructure
- Configurable failure thresholds and recovery mechanisms
- Support for interactive and automated bypass decisions

### 2. `bypass-strategies.yaml`

Complete documentation of all available bypass strategies:

- **Strategy Definitions**: Detailed explanations of each strategy
- **Use Cases**: When to use each strategy
- **Configuration Options**: All available settings per strategy
- **Performance Characteristics**: Latency and resource impact
- **Selection Guidelines**: How to choose the right strategy

**Strategies Covered:**
- Fail Gracefully
- Fallback Server
- Cache Response
- Circuit Breaker
- Retry with Backoff
- Adaptive Strategy

### 3. Language-Specific Templates

#### `go-bypass.yaml` - Go Language Configuration

Optimized for Go language servers (gopls, go-langserver):

- **Go-Specific Failures**: Module resolution, build errors, workspace timeouts
- **Performance Optimizations**: Large Go project handling, memory management
- **Project Patterns**: Standard layout, monorepo, legacy GOPATH projects
- **Method Configurations**: Optimized timeouts for Go analysis
- **Version Compatibility**: Go 1.17+ specific optimizations

**Key Optimizations:**
- Module-aware bypass strategies
- Vendor directory handling
- Go build integration
- Memory optimization for large codebases

#### `python-bypass.yaml` - Python Language Configuration

Tailored for Python language servers (pylsp, pyright, jedi-language-server):

- **Python-Specific Failures**: Import resolution, virtual environment issues
- **Framework Support**: Django, Flask, FastAPI optimizations
- **Virtual Environment**: Comprehensive venv, conda, poetry support
- **Performance Tuning**: Large Python project handling
- **Version Compatibility**: Python 3.7+ specific configurations

**Key Features:**
- Virtual environment detection and handling
- Framework-specific optimizations
- Import resolution strategies
- Data science project support

#### `typescript-bypass.yaml` - TypeScript Language Configuration

Designed for TypeScript servers (tsserver, typescript-language-server):

- **TypeScript-Specific Failures**: Compilation errors, type checking timeouts
- **Framework Integration**: React, Angular, Vue.js support
- **Build Tool Support**: Webpack, Vite, Next.js optimizations
- **Performance Tuning**: Large TypeScript project handling
- **Version Compatibility**: TypeScript 3.x-5.x configurations

**Key Features:**
- Incremental compilation support
- Framework-aware bypass strategies
- Memory optimization for large projects
- Type checking timeout handling

#### `java-bypass.yaml` - Java Language Configuration

Optimized for Java language servers (eclipse.jdt.ls, java-language-server):

- **Java-Specific Failures**: JVM startup, classpath resolution, build tool sync
- **Build Tool Integration**: Maven, Gradle optimization
- **Framework Support**: Spring Boot, Jakarta EE configurations
- **Memory Management**: JVM heap optimization
- **Version Compatibility**: Java 8-21+ configurations

**Key Features:**
- JVM memory optimization
- Build tool integration
- Enterprise framework support
- Large project workspace handling

## Usage Instructions

### 1. Basic Setup

Copy the default configuration to your LSP Gateway config directory:

```bash
cp config-templates/bypass/bypass-defaults.yaml ~/.lsp-gateway/bypass-config.yaml
```

### 2. Language-Specific Configuration

For language-specific optimizations, merge relevant language templates:

```bash
# For Go projects
cp config-templates/bypass/language-specific/go-bypass.yaml ~/.lsp-gateway/go-bypass.yaml

# For Python projects  
cp config-templates/bypass/language-specific/python-bypass.yaml ~/.lsp-gateway/python-bypass.yaml
```

### 3. Integration with Main Configuration

Add bypass configuration to your main LSP Gateway config:

```yaml
# config.yaml
bypass_config_file: "~/.lsp-gateway/bypass-config.yaml"

# Or embed directly
bypass_config:
  enabled: true
  # ... bypass settings
```

### 4. Customization

Customize templates based on your specific needs:

1. **Adjust Timeouts**: Modify timeouts based on your project size and server performance
2. **Configure Fallback Servers**: Set up backup servers for critical languages
3. **Tune Failure Thresholds**: Adjust thresholds based on your tolerance for failures
4. **Enable/Disable Strategies**: Choose strategies that fit your development workflow

## Configuration Examples

### Basic Bypass Configuration

```yaml
bypass_config:
  enabled: true
  global_bypass:
    enabled: true
    default_strategy: "fail_gracefully"
    failure_timeout: "30s"
    max_retry_attempts: 3
```

### Language-Specific Configuration

```yaml
language_bypass:
  go:
    strategy: "fallback_server"
    timeout: "15s"
    conditions: ["timeout", "memory_limit"]
  
  python:
    strategy: "retry_with_backoff"
    timeout: "20s"
    conditions: ["import_resolution", "virtual_env_issues"]
```

### Server-Specific Configuration

```yaml
server_bypass:
  - server: "gopls"
    bypass_enabled: true
    bypass_strategy: "fallback_server"
    fallback_server: "go-langserver"
    recovery_attempts: 3
    cooldown_period: "2m"
```

## Best Practices

### 1. Strategy Selection

- **Development**: Use `fail_gracefully` or `cache_response` for fast feedback
- **Production**: Use `fallback_server` or `adaptive` for maximum reliability
- **CI/CD**: Use `circuit_breaker` or aggressive timeouts for fast failures

### 2. Timeout Configuration

- **Small Projects**: Use shorter timeouts (5-15s)
- **Large Projects**: Use longer timeouts (30-60s)
- **Remote Development**: Increase timeouts by 50-100%

### 3. Memory Management

- **Go Projects**: 1-2GB for medium projects, 4GB+ for large
- **Python Projects**: 2-3GB for data science, 1-2GB for web apps
- **TypeScript Projects**: 1-2GB for most projects, 3GB+ for large monorepos
- **Java Projects**: 4GB minimum, 8GB+ for enterprise applications

### 4. Monitoring

Enable monitoring to track bypass effectiveness:

```yaml
monitoring:
  enabled: true
  retention_period: "24h"
  export:
    enabled: true
    format: "prometheus"
```

## Troubleshooting

### Common Issues

1. **High Bypass Rate**: Check server health, adjust timeouts, or add more fallback servers
2. **Slow Recovery**: Reduce cooldown periods or increase recovery attempts
3. **Memory Issues**: Increase JVM heap size or enable memory optimization
4. **Build Tool Failures**: Configure longer timeouts for dependency resolution

### Diagnostic Commands

```bash
# Check bypass status
lsp-gateway status bypass

# View bypass statistics
lsp-gateway bypass stats

# Test bypass configuration
lsp-gateway bypass test --config bypass-config.yaml

# Monitor bypass events
lsp-gateway bypass monitor --realtime
```

## Integration Notes

These templates are designed to integrate with the existing LSP Gateway infrastructure:

- **BypassDecisionType**: Compatible with existing bypass decision types
- **ServerState**: Works with `ServerStateBypassed` and `ServerStateBypassedRecovering`
- **FailureDetector**: Integrates with the comprehensive failure detection system
- **CircuitBreaker**: Uses existing circuit breaker infrastructure
- **HealthMonitor**: Compatible with health monitoring systems

## Performance Considerations

### Strategy Performance Impact

| Strategy | Latency | Memory | CPU | Use Case |
|----------|---------|--------|-----|----------|
| Fail Gracefully | < 1ms | None | None | Development |
| Fallback Server | Variable | Low | Low | Production |
| Cache Response | < 5ms | Medium | Low | Stable code |
| Circuit Breaker | < 1ms | Low | Low | Overloaded servers |
| Retry with Backoff | High | Low | Low | Transient failures |
| Adaptive | Variable | Medium | Medium | Complex environments |

### Resource Requirements

- **Memory Overhead**: 10-50MB depending on cache size and strategy
- **CPU Overhead**: < 5% additional CPU usage in most cases
- **Network Impact**: Reduced network usage with caching strategies
- **Storage Impact**: < 100MB for bypass state persistence

## Contributing

To contribute new language-specific templates:

1. Follow the existing template structure
2. Include comprehensive failure patterns for the language
3. Add framework-specific optimizations
4. Provide version compatibility information
5. Include monitoring and alerting configurations
6. Document best practices and troubleshooting

## Support

For issues with bypass configuration:

1. Check the troubleshooting section
2. Review LSP Gateway logs for bypass events
3. Test configuration with `lsp-gateway bypass test`
4. Consult language-specific optimization guides