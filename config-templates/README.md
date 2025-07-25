# LSP Gateway Configuration Templates

## Overview

This directory contains a comprehensive collection of configuration templates for the LSP Gateway system. These templates provide optimized configurations for different project types, team sizes, architectural patterns, and development environments.

The template collection is designed to help teams quickly adopt the LSP Gateway with battle-tested configurations that have been optimized for specific use cases, reducing setup time and providing best-practice configurations out of the box.

### Template Categories

The templates are organized into four main categories:

1. **Multi-Language Templates** - Complete feature demonstrations and production-ready configurations
2. **Language-Specific Examples** - Detailed configurations for individual languages and frameworks
3. **Architecture Patterns** - Common architectural patterns and project structures
4. **Specialized Configurations** - Performance, monitoring, and specialized use cases

### Version Compatibility

All templates are compatible with LSP Gateway v1.0+ and support the enhanced configuration system including:

- Multi-server language pools
- Enhanced connection pooling
- Circuit breaker patterns
- Health monitoring
- Dynamic resource scaling
- Project-aware routing

### Integration Features

These templates integrate seamlessly with the LSP Gateway's existing features:

- **Auto-detection System**: Templates can be auto-selected based on project analysis
- **CLI Integration**: Use with `lsp-gateway config generate --template <name>`
- **Migration Support**: Automatic migration from legacy configurations
- **Validation**: Built-in validation for all template configurations
- **Hot Reloading**: Configuration changes without server restart

## Quick Start

### Template Selection Decision Tree

```
Start Here: What type of project are you working with?

┌─ Single Language Project
│  ├─ Small team (1-5 developers) → patterns/single-language.yaml
│  ├─ Medium team (5-15 developers) → languages/{language}-advanced.yaml
│  └─ Large team (15+ developers) → production-template.yaml
│
├─ Multi-Language Project
│  ├─ Full-stack web app → patterns/full-stack.yaml
│  ├─ Microservices architecture → microservices-template.yaml
│  ├─ Monorepo → monorepo-template.yaml
│  └─ Enterprise polyglot → patterns/enterprise.yaml
│
├─ Development Environment
│  ├─ Local development → development-template.yaml
│  ├─ Code analysis focus → analysis-template.yaml
│  └─ Production deployment → production-template.yaml
│
└─ Special Requirements
   ├─ High performance → production-template.yaml
   ├─ Resource constrained → patterns/single-language.yaml
   └─ Learning/demo → multi-language-template.yaml
```

### Getting Started

1. **Select a Template**: Use the decision tree above to choose the appropriate template
2. **Copy and Customize**: Copy the template to your project directory as `config.yaml`
3. **Validate**: Run `lsp-gateway config validate` to ensure correctness
4. **Start Server**: Run `lsp-gateway server --config config.yaml`

#### Example Usage

```bash
# Copy a template
cp config-templates/development-template.yaml ./config.yaml

# Customize for your project
vim config.yaml

# Validate the configuration
lsp-gateway config validate

# Start the server
lsp-gateway server --config config.yaml
```

## Template Categories

### Multi-Language Templates

These templates demonstrate the full capabilities of the LSP Gateway system and are designed for comprehensive multi-language development environments.

#### `multi-language-template.yaml`
**Purpose**: Complete feature demonstration and default multi-language setup
**Best For**: Learning the system, proof-of-concepts, general multi-language projects
**Languages**: Go, Python, TypeScript/JavaScript, Java, Rust
**Features**:
- All major language servers configured
- Balanced performance and feature settings
- Comprehensive settings examples
- Framework detection and optimization
- Standard resource limits

**Use Cases**:
- New team evaluation of LSP Gateway
- Educational environments
- Multi-language prototyping
- Standard development workflows

#### `monorepo-template.yaml`
**Purpose**: Optimized for monorepo architectures with multiple projects
**Best For**: Large monorepos with multiple languages and services
**Languages**: All supported languages with workspace awareness
**Features**:
- Multi-root workspace configuration
- Cross-language reference support
- Project boundary detection
- Shared dependency management
- Workspace-aware indexing

**Use Cases**:
- Large enterprise monorepos
- Multi-service repositories
- Shared library development
- Complex project hierarchies

#### `microservices-template.yaml`
**Purpose**: Service-oriented architecture with distributed language servers
**Best For**: Microservices architectures with service-specific language optimization
**Languages**: Service-specific language configurations
**Features**:
- Service isolation boundaries
- Independent language server pools
- Service-aware routing
- Resource isolation
- Health monitoring per service

**Use Cases**:
- Kubernetes deployments
- Docker-based development
- Service mesh architectures
- Distributed development teams

#### `production-template.yaml`
**Purpose**: Production-hardened configuration with maximum reliability
**Best For**: Production deployments requiring high availability
**Languages**: Production-optimized for all languages
**Features**:
- Circuit breaker patterns
- Enhanced health monitoring
- Resource limits and throttling
- Comprehensive error handling
- Performance monitoring
- Security hardening

**Use Cases**:
- Production LSP Gateway deployments
- High-availability requirements
- Enterprise production environments
- Critical development infrastructure

#### `development-template.yaml`
**Purpose**: Developer experience optimized configuration
**Best For**: Local development environments
**Languages**: Developer-friendly settings for all languages
**Features**:
- Fast startup and response times
- Rich IDE integration features
- Comprehensive code analysis
- Interactive debugging support
- Hot reload capabilities

**Use Cases**:
- Local developer workstations
- IDE integration
- Interactive development
- Rapid prototyping

#### `analysis-template.yaml`
**Purpose**: Code quality and analysis focused configuration
**Best For**: Code review, static analysis, and quality assurance
**Languages**: Analysis-optimized settings for all languages
**Features**:
- Maximum diagnostic capabilities
- Comprehensive linting and analysis
- Security scanning integration
- Code quality metrics
- Deep inspection modes

**Use Cases**:
- Continuous integration pipelines
- Code review workflows
- Static analysis systems
- Quality assurance processes

### Language-Specific Examples

These templates provide detailed, optimized configurations for specific programming languages and their popular frameworks.

#### `languages/go-advanced.yaml`
**Purpose**: Advanced Go development with enterprise features
**Languages**: Go
**Features**:
- Advanced gopls configuration
- Module and workspace support
- Build tag awareness
- Comprehensive analysis rules
- Performance optimization for large Go codebases

**Frameworks Supported**:
- Gin web framework
- Echo framework
- Kubernetes controllers
- gRPC services
- CLI applications

**Use Cases**:
- Large Go microservices
- Kubernetes operator development
- Enterprise Go applications
- High-performance Go services

#### `languages/python-django.yaml`
**Purpose**: Python development optimized for Django projects
**Languages**: Python
**Features**:
- Django-specific settings and plugins
- Virtual environment support
- Database model awareness
- Template language support
- Django REST framework integration

**Frameworks Supported**:
- Django web framework
- Django REST framework
- Celery task queues
- pytest testing
- FastAPI (secondary)

**Use Cases**:
- Django web applications
- Django REST APIs
- Full-stack Python development
- Python web services

#### `languages/typescript-react.yaml`
**Purpose**: TypeScript/JavaScript with React ecosystem optimization
**Languages**: TypeScript, JavaScript
**Features**:
- React-specific completions and analysis
- JSX/TSX support
- Modern ES6+ features
- Build tool integration (Webpack, Vite)
- Testing framework support

**Frameworks Supported**:
- React and React Router
- Next.js applications
- Create React App
- Vite build tool
- Jest and React Testing Library

**Use Cases**:
- React web applications
- Next.js full-stack applications
- Modern frontend development
- Single-page applications

#### `languages/java-spring.yaml`
**Purpose**: Java development with Spring Boot ecosystem
**Languages**: Java
**Features**:
- Spring Boot specific configuration
- Annotation processing
- Maven/Gradle build integration
- JPA and database support
- Microservices patterns

**Frameworks Supported**:
- Spring Boot and Spring Framework
- Spring Security
- Spring Data JPA
- Maven and Gradle
- JUnit testing

**Use Cases**:
- Spring Boot microservices
- Enterprise Java applications
- REST API development
- Java web services

#### `languages/rust-workspace.yaml`
**Purpose**: Rust development with workspace and cargo integration
**Languages**: Rust
**Features**:
- Cargo workspace support
- Comprehensive Clippy integration
- Performance-focused settings
- Proc macro support
- Cross-compilation awareness

**Frameworks Supported**:
- Actix web framework
- Tokio async runtime
- Serde serialization
- Diesel ORM
- Wasm development

**Use Cases**:
- Rust web services
- System programming
- WebAssembly projects
- Performance-critical applications

### Architecture Patterns

These templates are designed for common architectural patterns and project structures.

#### `patterns/single-language.yaml`
**Purpose**: Simple, single-language project configuration
**Best For**: Small teams, simple projects, learning environments
**Languages**: Configurable (template parameter)
**Features**:
- Minimal resource usage
- Simple configuration
- Fast startup
- Easy customization
- Single language server

**Use Cases**:
- Startup projects
- Individual development
- Learning environments
- Simple applications

#### `patterns/full-stack.yaml`
**Purpose**: Full-stack web application development
**Best For**: Frontend + Backend development teams
**Languages**: TypeScript/JavaScript (frontend), Python/Go/Java (backend)
**Features**:
- Frontend/backend separation
- API development support
- Database integration
- Build pipeline awareness
- Development server integration

**Use Cases**:
- Web application development
- API + frontend projects
- Full-stack development teams
- Modern web architectures

#### `patterns/polyglot.yaml`
**Purpose**: Multi-language projects with cross-language integration
**Best For**: Complex projects using multiple programming languages
**Languages**: All supported languages with cross-references
**Features**:
- Cross-language symbol resolution
- Multi-language debugging
- Shared library support
- Language interop features
- Complex dependency management

**Use Cases**:
- Legacy system modernization
- Multi-language data pipelines
- Complex enterprise systems
- Research and experimental projects

#### `patterns/enterprise.yaml`
**Purpose**: Enterprise-scale configuration with governance and compliance
**Best For**: Large enterprises with complex requirements
**Languages**: All languages with enterprise governance
**Features**:
- Compliance and audit logging
- Role-based access control
- Enterprise security policies
- Scalability and performance monitoring
- Integration with enterprise tools

**Use Cases**:
- Large enterprise development
- Regulated industries
- Government projects
- Complex organizational structures

## Configuration Customization Guide

### Basic Customization

#### Port and Network Settings

```yaml
# Change the server port
port: 8080  # Default: 8080

# Adjust timeout settings
timeout: "30s"  # Default: 30s

# Set concurrent request limits
max_concurrent_requests: 200  # Default: 100
```

#### Language Server Configuration

```yaml
servers:
  - name: "custom-go-server"
    languages: ["go"]
    command: "gopls"
    args: ["--logfile", "/tmp/gopls.log"]  # Add custom arguments
    transport: "stdio"  # or "tcp", "http"
    
    # Custom settings for the language server
    settings:
      gopls:
        analyses:
          unusedparams: true
          shadow: true
        staticcheck: true
```

#### Project Context Customization

```yaml
# Update project paths
project_info:
  root_directory: "/path/to/your/project"
  project_type: "multi-language"  # or "single-language", "monorepo", etc.
  
# Configure workspace roots
workspace:
  multi_root: true
  roots:
    - "/path/to/frontend"
    - "/path/to/backend"
    - "/path/to/shared"
```

### Advanced Customization

#### Performance Optimization

```yaml
# Enhanced pool configuration
servers:
  - name: "high-perf-go"
    languages: ["go"]
    command: "gopls"
    transport: "tcp"
    
    pool_config:
      min_size: 2
      max_size: 15
      warmup_size: 3
      enable_dynamic_sizing: true
      target_utilization: 0.75
      
      # Resource limits
      memory_limit_mb: 150
      cpu_limit_percent: 85.0
      
      # Health monitoring
      health_check_interval: 30s
      max_retries: 4
      circuit_timeout: 15s
```

#### Multi-Server Load Balancing

```yaml
# Language pools with multiple servers
language_pools:
  - language: "python"
    default_server: "pylsp-fast"
    servers:
      pylsp-fast:
        name: "pylsp-fast"
        command: "pylsp"
        transport: "tcp"
        priority: 3
        weight: 3.0
        max_concurrent_requests: 80
        
      pylsp-complete:
        name: "pylsp-complete"
        command: "pylsp"
        transport: "stdio"
        priority: 2
        weight: 2.0
        max_concurrent_requests: 60
        
    load_balancing:
      strategy: "response_time"  # or "round_robin", "least_connections"
      health_threshold: 0.9
```

#### Framework-Specific Enhancement

```yaml
# React-specific TypeScript configuration
servers:
  - name: "typescript-react"
    languages: ["typescript", "javascript"]
    command: "typescript-language-server"
    frameworks: ["react"]  # Enables framework enhancements
    
    settings:
      typescript:
        preferences:
          includeCompletionsForModuleExports: true
          includeCompletionsWithInsertText: true
        suggest:
          autoImports: true
        jsx: "react-jsx"
```

### Environment-Specific Customization

#### Development Environment

```yaml
# Development-optimized settings
optimized_for: "development"

# Enable development features
development:
  enable_debug_logging: true
  hot_reload: true
  comprehensive_analysis: true
  interactive_features: true
  
# Relaxed resource limits for development
global_resource_limits:
  max_memory_mb: 4096
  max_concurrent_requests: 150
```

#### Production Environment

```yaml
# Production-hardened settings
optimized_for: "production"

# Production safety features
production:
  enable_circuit_breakers: true
  comprehensive_monitoring: true
  security_hardening: true
  resource_throttling: true
  
# Strict resource limits
global_resource_limits:
  max_memory_mb: 2048
  max_concurrent_requests: 100
  
# Enhanced error handling
error_handling:
  max_retries: 3
  backoff_strategy: "exponential"
  circuit_breaker_threshold: 0.5
```

## Migration Guide

### From Legacy Gateway Config

If you have an existing LSP Gateway configuration, you can automatically migrate it:

```bash
# Automatic migration
lsp-gateway config migrate --input legacy-config.yaml --output new-config.yaml

# Validate the migrated configuration
lsp-gateway config validate --config new-config.yaml
```

#### Manual Migration Steps

1. **Identify Configuration Format**: Determine if you have a legacy `GatewayConfig` or simple server list
2. **Copy Base Template**: Start with the closest template from this collection
3. **Transfer Settings**: Copy your custom settings to the new format
4. **Add New Features**: Leverage new features like pooling and multi-server support
5. **Validate**: Ensure the new configuration works correctly

#### Example Migration

**Legacy Configuration:**
```yaml
port: 8080
servers:
  - name: "go-server"
    command: "gopls"
    languages: ["go"]
```

**Migrated Configuration:**
```yaml
port: 8080
timeout: "30s"
max_concurrent_requests: 100

servers:
  - name: "go-server"
    languages: ["go"]
    command: "gopls"
    transport: "stdio"
    
    # New features added during migration
    pool_config:
      min_size: 1
      max_size: 5
      enable_dynamic_sizing: true
      
    health_check_settings:
      enabled: true
      interval: 30s
```

### From Simple Language Lists

For configurations that just specified a list of languages:

```bash
# Generate from project analysis
lsp-gateway config generate --auto-detect --output config.yaml

# Use template as starting point
cp config-templates/multi-language-template.yaml config.yaml
```

### Breaking Changes and Compatibility

#### v1.0 to v1.1
- **Pool Configuration**: Added enhanced pool management
- **Multi-Server Support**: New language pool syntax
- **Health Monitoring**: Enhanced health check configuration

#### Backward Compatibility
- Legacy configurations continue to work
- Automatic migration available
- Gradual adoption of new features possible

## Examples and Use Cases

### Startup Team (2-5 developers)

**Recommended Template**: `patterns/single-language.yaml` or `patterns/full-stack.yaml`

**Characteristics**:
- Limited resources
- Rapid development focus
- Simple architecture
- Learning-oriented

**Configuration**:
```yaml
# Optimized for small teams
port: 8080
timeout: "30s"
max_concurrent_requests: 50

servers:
  - name: "primary-language-server"
    languages: ["go"]  # or your primary language
    command: "gopls"
    transport: "stdio"
    
    # Minimal resource configuration
    pool_config:
      min_size: 1
      max_size: 3
      enable_dynamic_sizing: false
```

**Benefits**:
- Fast startup and low resource usage
- Simple configuration and maintenance
- Focus on core development features
- Easy to understand and modify

### Growing Team (5-20 developers)

**Recommended Template**: `multi-language-template.yaml` or specific language templates

**Characteristics**:
- Multi-language requirements
- Performance becomes important
- Scalability considerations
- Team coordination needs

**Configuration**:
```yaml
# Balanced configuration for growing teams
port: 8080
timeout: "30s"
max_concurrent_requests: 150

# Multiple language servers
servers:
  - name: "go-server"
    languages: ["go"]
    command: "gopls"
    transport: "stdio"
    pool_config:
      min_size: 2
      max_size: 8
      enable_dynamic_sizing: true
      
  - name: "typescript-server"
    languages: ["typescript", "javascript"]
    command: "typescript-language-server"
    args: ["--stdio"]
    transport: "stdio"
    pool_config:
      min_size: 2
      max_size: 6
      enable_dynamic_sizing: true
```

**Benefits**:
- Multi-language support
- Performance optimization
- Scalable architecture
- Team productivity features

### Enterprise Team (20+ developers)

**Recommended Template**: `production-template.yaml` or `patterns/enterprise.yaml`

**Characteristics**:
- Maximum performance and reliability required
- Complex multi-language projects
- High availability requirements
- Advanced monitoring and observability

**Configuration**:
```yaml
# Enterprise-scale configuration
port: 8080
timeout: "30s"
max_concurrent_requests: 300

# Advanced multi-server pools
language_pools:
  - language: "go"
    default_server: "gopls-primary"
    servers:
      gopls-primary:
        name: "gopls-primary"
        command: "gopls"
        transport: "tcp"
        priority: 2
        weight: 2.0
        max_concurrent_requests: 100
        pool_config:
          min_size: 3
          max_size: 15
          enable_dynamic_sizing: true
          
      gopls-secondary:
        name: "gopls-secondary"
        command: "gopls"
        args: ["--mode=lightweight"]
        transport: "stdio"
        priority: 1
        weight: 1.0
        max_concurrent_requests: 50
        
    load_balancing:
      strategy: "round_robin"
      health_threshold: 0.8

# Global monitoring and limits
pool_management:
  enable_global_monitoring: true
  max_total_connections: 200
  max_total_memory_mb: 8192
  
  global_circuit_breaker:
    enabled: true
    failure_threshold: 0.50
    recovery_timeout: 30s
```

**Benefits**:
- Maximum performance and reliability
- Advanced monitoring and observability
- High availability features
- Enterprise-grade security

### Framework-Specific Examples

#### React + Node.js Full-Stack

```yaml
# Full-stack React + Node.js configuration
servers:
  - name: "typescript-frontend"
    languages: ["typescript", "javascript"]
    command: "typescript-language-server"
    frameworks: ["react"]
    settings:
      typescript:
        preferences:
          includeCompletionsForModuleExports: true
        jsx: "react-jsx"
        
  - name: "node-backend"
    languages: ["javascript"]
    command: "typescript-language-server"
    frameworks: ["node", "express"]
    settings:
      typescript:
        preferences:
          includeCompletionsForModuleExports: true
```

#### Django + React

```yaml
# Django backend + React frontend
servers:
  - name: "python-django"
    languages: ["python"]
    command: "pylsp"
    frameworks: ["django"]
    settings:
      pylsp:
        plugins:
          django:
            enabled: true
          pycodestyle:
            enabled: true
            
  - name: "typescript-react"
    languages: ["typescript", "javascript"]
    command: "typescript-language-server"
    frameworks: ["react"]
    settings:
      typescript:
        jsx: "react-jsx"
```

#### Spring Boot + Angular

```yaml
# Spring Boot backend + Angular frontend
servers:
  - name: "java-spring"
    languages: ["java"]
    command: "jdtls"
    frameworks: ["spring"]
    settings:
      java:
        configuration:
          maven:
            userSettings: "/path/to/maven/settings.xml"
            
  - name: "typescript-angular"
    languages: ["typescript"]
    command: "typescript-language-server"
    frameworks: ["angular"]
    settings:
      typescript:
        preferences:
          includeCompletionsForModuleExports: true
```

## Troubleshooting Guide

### Common Configuration Issues

#### Language Server Not Found

**Symptoms**:
- "Command not found" errors
- Server fails to start
- No completions or diagnostics

**Solutions**:
```bash
# Install missing language servers
lsp-gateway install servers

# Verify installations
lsp-gateway verify runtime all

# Check server paths
which gopls
which pylsp
which typescript-language-server
```

**Configuration Fix**:
```yaml
# Specify full path if needed
servers:
  - name: "go-server"
    command: "/usr/local/bin/gopls"  # Full path
    languages: ["go"]
```

#### Port Already in Use

**Symptoms**:
- "Address already in use" error
- Server fails to bind to port

**Solutions**:
```yaml
# Change port in configuration
port: 8081  # Use different port

# Or find and kill existing process
lsof -ti:8080 | xargs kill -9
```

#### High Memory Usage

**Symptoms**:
- System slowdown
- Out of memory errors
- Server crashes

**Solutions**:
```yaml
# Add resource limits
servers:
  - name: "memory-limited-server"
    languages: ["java"]
    command: "jdtls"
    pool_config:
      memory_limit_mb: 512  # Limit memory usage
      max_size: 3           # Reduce pool size
      
# Global resource limits
pool_management:
  max_total_memory_mb: 2048
```

#### Circuit Breaker Triggering

**Symptoms**:
- Intermittent service failures
- "Circuit breaker open" errors
- Degraded performance

**Solutions**:
```yaml
# Adjust circuit breaker thresholds
servers:
  - name: "reliable-server"
    languages: ["go"]
    command: "gopls"
    pool_config:
      max_retries: 5          # Increase retries
      circuit_timeout: 30s    # Longer timeout
      base_delay: 200ms       # Longer base delay
      
# Health check tuning
health_check_settings:
  failure_threshold: 5      # Allow more failures
  success_threshold: 2      # Require fewer successes
```

### Performance Issues

#### Slow Response Times

**Check List**:
1. **Resource Limits**: Ensure adequate memory and CPU limits
2. **Pool Size**: Increase pool sizes for high load
3. **Transport**: Consider TCP transport for better performance
4. **Health Checks**: Reduce health check frequency

**Optimization**:
```yaml
# Performance-optimized configuration
servers:
  - name: "fast-server"
    languages: ["go"]
    command: "gopls"
    transport: "tcp"  # Faster than stdio for high load
    
    pool_config:
      min_size: 3
      max_size: 20          # Larger pool
      target_utilization: 0.60  # Lower utilization target
      memory_limit_mb: 300  # Higher memory limit
      
    health_check_settings:
      interval: 60s         # Less frequent health checks
```

#### High CPU Usage

**Investigation**:
```bash
# Monitor resource usage
top -p $(pgrep lsp-gateway)

# Check language server processes
ps aux | grep -E "(gopls|pylsp|typescript-language-server)"
```

**Solutions**:
```yaml
# CPU optimization
servers:
  - name: "cpu-optimized"
    languages: ["python"]
    command: "pylsp"
    
    pool_config:
      cpu_limit_percent: 70.0  # Limit CPU usage
      max_size: 4              # Reduce concurrent processes
      
    settings:
      pylsp:
        plugins:
          pylint:
            enabled: false     # Disable CPU-intensive plugins
```

#### Network Timeouts

**Symptoms**:
- Connection timeouts
- Request failures
- Intermittent connectivity

**Solutions**:
```yaml
# Network timeout configuration
timeout: "60s"              # Increase global timeout

servers:
  - name: "network-resilient"
    languages: ["go"]
    command: "gopls"
    transport: "tcp"
    
    connection_settings:
      connect_timeout: 30s
      read_timeout: 60s
      write_timeout: 60s
      keep_alive: true
      keep_alive_period: 30s
```

### Language Server Specific Problems

#### Go (gopls) Issues

**Common Problems**:
- Module resolution failures
- Slow indexing
- Memory leaks

**Solutions**:
```yaml
servers:
  - name: "gopls-optimized"
    languages: ["go"]
    command: "gopls"
    
    settings:
      gopls:
        # Improve module resolution
        gofumpt: true
        staticcheck: true
        
        # Memory optimization
        memoryMode: "DegradeClosed"
        
        # Faster indexing
        experimentalWorkspaceModule: true
        
    environment:
      GOPROXY: "https://proxy.golang.org,direct"
      GOPRIVATE: "*.corp.example.com"
```

#### Python (pylsp) Issues

**Common Problems**:
- Virtual environment detection
- Slow linting
- Import resolution

**Solutions**:
```yaml
servers:
  - name: "pylsp-optimized"
    languages: ["python"]
    command: "python"
    args: ["-m", "pylsp"]
    
    settings:
      pylsp:
        plugins:
          # Optimize linting
          pycodestyle:
            enabled: true
            maxLineLength: 100
          pylint:
            enabled: false  # Disable slow pylint
          flake8:
            enabled: true
          
          # Import resolution
          rope_completion:
            enabled: true
          jedi_completion:
            enabled: true
            
    environment:
      PYTHONPATH: "/path/to/venv/lib/python3.9/site-packages"
```

#### TypeScript/JavaScript Issues

**Common Problems**:
- Large project performance
- Import resolution
- Build integration

**Solutions**:
```yaml
servers:
  - name: "typescript-optimized"
    languages: ["typescript", "javascript"]
    command: "typescript-language-server"
    args: ["--stdio", "--log-level", "info"]
    
    settings:
      typescript:
        preferences:
          includeCompletionsForModuleExports: true
          includeCompletionsWithInsertText: true
          
        # Performance optimization
        maxTsServerMemory: 4096
        
        # Build integration
        npm:
          packageManager: "npm"
          
    root_markers: ["tsconfig.json", "package.json", "yarn.lock"]
```

## Reference Documentation

### Configuration Schema Reference

#### Core Configuration

```yaml
# Gateway-level configuration
port: 8080                    # Server port (default: 8080)
timeout: "30s"               # Request timeout (default: 30s)
max_concurrent_requests: 100  # Concurrent request limit (default: 100)
log_level: "info"            # Logging level: debug, info, warn, error
enable_metrics: true         # Enable metrics collection (default: false)
metrics_port: 9090           # Metrics server port (default: 9090)
```

#### Server Configuration Schema

```yaml
servers:
  - name: "server-name"           # Required: Unique server name
    languages: ["go", "python"]  # Required: Supported languages
    command: "command-name"       # Required: Command to start server
    args: ["--arg1", "--arg2"]   # Optional: Command arguments
    transport: "stdio"           # Required: stdio, tcp, or http
    
    # Optional: Root markers for project detection
    root_markers: ["go.mod", "pyproject.toml"]
    
    # Optional: Language server settings
    settings:
      server_name:
        setting_key: setting_value
        
    # Optional: Multi-server support
    priority: 1                  # Server priority (higher = preferred)
    weight: 1.0                 # Load balancing weight
    max_concurrent_requests: 50  # Server-specific request limit
    
    # Optional: Enhanced pool configuration
    pool_config:
      min_size: 1               # Minimum pool size
      max_size: 5               # Maximum pool size
      warmup_size: 2            # Initial warm-up size
      enable_dynamic_sizing: true  # Enable dynamic scaling
      target_utilization: 0.75    # Target utilization ratio
      max_lifetime: 30m           # Connection lifetime
      idle_timeout: 5m           # Idle connection timeout
      health_check_interval: 30s  # Health check frequency
      memory_limit_mb: 100       # Memory limit per connection
      cpu_limit_percent: 80.0    # CPU limit percentage
      
    # Optional: Health monitoring
    health_check_settings:
      enabled: true             # Enable health checks
      interval: 30s            # Check interval
      timeout: 10s             # Check timeout
      failure_threshold: 3     # Failures before marking unhealthy
      success_threshold: 2     # Successes before marking healthy
      method: "initialize"     # Health check method
      enable_auto_restart: true  # Auto-restart on failure
      
    # Optional: Environment variables
    environment:
      ENV_VAR: "value"
      
    # Optional: Working directory
    working_dir: "/workspace"
```

#### Language Pool Configuration

```yaml
language_pools:
  - language: "go"              # Language identifier
    default_server: "gopls-primary"  # Default server name
    
    servers:
      server-name:              # Server configurations (same as above)
        name: "server-name"
        command: "command"
        # ... other server config
        
    load_balancing:
      strategy: "round_robin"   # round_robin, response_time, least_connections
      health_threshold: 0.8    # Health threshold for load balancing
      weight_factors:          # Custom weight factors
        server1: 2.0
        server2: 1.0
        
    resource_limits:
      max_memory_mb: 1024      # Pool memory limit
      max_concurrent_requests: 100  # Pool request limit
      max_processes: 5         # Maximum processes in pool
      request_timeout_seconds: 30   # Request timeout
```

#### Global Pool Management

```yaml
pool_management:
  enable_global_monitoring: true    # Enable global monitoring
  monitoring_interval: 30s         # Monitoring frequency
  max_total_connections: 100       # Global connection limit
  max_total_memory_mb: 2048       # Global memory limit
  max_total_cpu_percent: 300.0    # Global CPU limit (multiple cores)
  
  # Cleanup settings
  enable_orphan_cleanup: true     # Clean up orphaned connections
  cleanup_interval: 5m           # Cleanup frequency
  
  # Metrics settings
  enable_detailed_metrics: true   # Detailed metrics collection
  metrics_retention: 24h         # Metrics retention period
  metrics_granularity: "1m"      # Metrics collection granularity
  
  # Global circuit breaker
  global_circuit_breaker:
    enabled: true               # Enable global circuit breaker
    failure_threshold: 0.50     # Failure rate threshold
    recovery_timeout: 30s       # Recovery timeout
    
  # Emergency mode
  emergency_mode:
    enabled: true               # Enable emergency mode
    trigger_error_rate: 0.75   # Error rate trigger
    trigger_memory_percent: 90.0  # Memory usage trigger
    trigger_cpu_percent: 95.0   # CPU usage trigger
    actions: ["reduce_pool_sizes", "disable_dynamic_sizing"]
```

### Environment Variables

The LSP Gateway supports configuration through environment variables:

#### Core Settings
- `LSP_GATEWAY_PORT` - Server port (overrides config)
- `LSP_GATEWAY_TIMEOUT` - Request timeout
- `LSP_GATEWAY_LOG_LEVEL` - Logging level
- `LSP_GATEWAY_CONFIG` - Configuration file path

#### Language Server Paths
- `GOPLS_PATH` - Path to gopls binary
- `PYLSP_PATH` - Path to pylsp binary
- `TYPESCRIPT_LS_PATH` - Path to typescript-language-server
- `JDTLS_PATH` - Path to jdtls binary

#### Resource Limits
- `LSP_GATEWAY_MAX_MEMORY` - Maximum memory usage
- `LSP_GATEWAY_MAX_CPU` - Maximum CPU usage
- `LSP_GATEWAY_MAX_CONNECTIONS` - Maximum connections

#### Example Usage
```bash
export LSP_GATEWAY_PORT=8081
export LSP_GATEWAY_LOG_LEVEL=debug
export GOPLS_PATH=/usr/local/bin/gopls
lsp-gateway server --config config.yaml
```

### Command-Line Flag Overrides

Configuration values can be overridden with command-line flags:

```bash
# Override port and timeout
lsp-gateway server --config config.yaml --port 8081 --timeout 45s

# Override log level
lsp-gateway server --config config.yaml --log-level debug

# Override maximum concurrent requests
lsp-gateway server --config config.yaml --max-concurrent-requests 200

# Enable metrics
lsp-gateway server --config config.yaml --enable-metrics --metrics-port 9091
```

### Integration with Existing Tools

#### IDE Integration

**VS Code Integration**:
```json
{
  "lsp-gateway.serverUrl": "http://localhost:8080/jsonrpc",
  "lsp-gateway.enableLogging": true,
  "lsp-gateway.timeout": 30000
}
```

**Vim/Neovim Integration**:
```lua
require('lspconfig').lsp_gateway.setup{
  cmd = {'curl', '-X', 'POST', 'http://localhost:8080/jsonrpc'},
  filetypes = {'go', 'python', 'typescript', 'javascript', 'java'},
}
```

**Emacs Integration**:
```elisp
(setq lsp-gateway-server-url "http://localhost:8080/jsonrpc")
(add-to-list 'lsp-language-id-configuration '(".*\\.go$" . "go"))
```

#### Build Tool Integration

**Makefile Integration**:
```makefile
.PHONY: lsp-start lsp-stop lsp-status

lsp-start:
	lsp-gateway server --config config.yaml --daemon

lsp-stop:
	pkill lsp-gateway

lsp-status:
	lsp-gateway status
```

**Docker Integration**:
```dockerfile
FROM golang:1.19-alpine
RUN go install lsp-gateway@latest
COPY config.yaml /app/config.yaml
WORKDIR /app
EXPOSE 8080
CMD ["lsp-gateway", "server", "--config", "config.yaml"]
```

**Docker Compose**:
```yaml
version: '3.8'
services:
  lsp-gateway:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./config.yaml:/app/config.yaml
      - ./workspace:/workspace
    environment:
      - LSP_GATEWAY_LOG_LEVEL=info
```

## Best Practices and Recommendations

### Configuration Management Best Practices

#### Version Control
- **Store Configurations**: Always commit configuration files to version control
- **Environment-Specific Configs**: Use separate configurations for dev/staging/prod
- **Template Versioning**: Track which template version you're using
- **Change Documentation**: Document configuration changes in commit messages

#### Configuration Organization
```
project/
├── config/
│   ├── development.yaml      # Development environment
│   ├── production.yaml       # Production environment
│   ├── testing.yaml         # Testing environment
│   └── local.yaml           # Local overrides (gitignored)
├── config.yaml              # Symlink to current environment
└── .lsp-gateway/
    ├── logs/               # Server logs
    └── metrics/           # Metrics data
```

#### Security Considerations

**Secure Configuration**:
```yaml
# Use environment variables for sensitive data
servers:
  - name: "secure-server"
    environment:
      API_KEY: "${LSP_API_KEY}"  # Environment variable
      DB_PASSWORD: "${DB_PASSWORD}"
      
# Restrict network access
network:
  bind_address: "127.0.0.1"  # Localhost only
  enable_tls: true
  tls_cert_file: "/path/to/cert.pem"
  tls_key_file: "/path/to/key.pem"
```

**File Permissions**:
```bash
# Secure configuration files
chmod 600 config.yaml
chmod 700 .lsp-gateway/

# Secure sensitive files
chmod 600 /path/to/cert.pem
chmod 600 /path/to/key.pem
```

### Performance Optimization

#### Resource Planning

**Small Teams (1-5 developers)**:
- CPU: 2-4 cores
- Memory: 2-4 GB
- Pool Sizes: 1-3 per language
- Concurrent Requests: 50-100

**Medium Teams (5-20 developers)**:
- CPU: 4-8 cores
- Memory: 4-8 GB
- Pool Sizes: 2-8 per language
- Concurrent Requests: 100-200

**Large Teams (20+ developers)**:
- CPU: 8+ cores
- Memory: 8+ GB
- Pool Sizes: 5-20 per language
- Concurrent Requests: 200-500

#### Monitoring and Observability

**Essential Metrics**:
- Request rate and response time
- Memory and CPU usage per language server
- Pool utilization and scaling events
- Circuit breaker events and failure rates
- Health check success rates

**Monitoring Configuration**:
```yaml
# Enable comprehensive monitoring
enable_metrics: true
metrics_port: 9090

# Detailed logging
logging:
  pool_events: true
  health_checks: true
  circuit_breaker_events: true
  performance_metrics: true
  
  levels:
    pool_manager: "info"
    health_monitor: "info"
    circuit_breaker: "warn"
```

**Prometheus Integration**:
```yaml
# Prometheus metrics endpoint
metrics:
  prometheus:
    enabled: true
    endpoint: "/metrics"
    port: 9090
```

### Development Workflow Integration

#### Development Environment Setup

**Quick Start Script**:
```bash
#!/bin/bash
# setup-lsp.sh

# Copy appropriate template
cp config-templates/development-template.yaml ./config.yaml

# Install language servers
lsp-gateway setup all

# Validate configuration
lsp-gateway config validate

# Start server
lsp-gateway server --config config.yaml
```

**IDE Configuration**:
```json
{
  "scripts": {
    "lsp:start": "lsp-gateway server --config config.yaml --daemon",
    "lsp:stop": "pkill lsp-gateway",
    "lsp:restart": "npm run lsp:stop && npm run lsp:start",
    "lsp:status": "lsp-gateway status"
  }
}
```

#### Continuous Integration

**CI Pipeline Integration**:
```yaml
# .github/workflows/lsp-validation.yml
name: LSP Configuration Validation

on: [push, pull_request]

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      
      - name: Install LSP Gateway
        run: |
          wget https://github.com/lsp-gateway/releases/latest/lsp-gateway-linux
          chmod +x lsp-gateway-linux
          sudo mv lsp-gateway-linux /usr/local/bin/lsp-gateway
      
      - name: Validate Configuration
        run: lsp-gateway config validate --config config.yaml
        
      - name: Test Configuration
        run: |
          lsp-gateway server --config config.yaml --daemon
          sleep 5
          curl -f http://localhost:8080/health || exit 1
          pkill lsp-gateway
```

### Troubleshooting Workflow

#### Diagnostic Process

1. **Check Configuration**:
   ```bash
   lsp-gateway config validate --config config.yaml
   ```

2. **Verify Language Servers**:
   ```bash
   lsp-gateway verify runtime all
   lsp-gateway status servers
   ```

3. **Check Resource Usage**:
   ```bash
   lsp-gateway status --detailed
   curl http://localhost:9090/metrics
   ```

4. **Review Logs**:
   ```bash
   lsp-gateway server --config config.yaml --log-level debug
   ```

5. **Test Connectivity**:
   ```bash
   curl -X POST http://localhost:8080/jsonrpc \
     -H "Content-Type: application/json" \
     -d '{"jsonrpc":"2.0","id":1,"method":"ping"}'
   ```

#### Common Issue Resolution

**Performance Issues**:
1. Increase pool sizes and resource limits
2. Switch to TCP transport for high-load scenarios
3. Reduce health check frequency
4. Enable circuit breakers to prevent cascading failures

**Reliability Issues**:
1. Enable auto-restart for language servers
2. Configure appropriate retry policies
3. Set up health monitoring and alerting
4. Use multi-server pools for redundancy

**Memory Issues**:
1. Set memory limits on language servers
2. Enable dynamic pool sizing
3. Configure emergency mode triggers
4. Monitor and alert on memory usage

This comprehensive configuration template documentation provides everything needed to successfully deploy and maintain LSP Gateway in any environment, from small development teams to large enterprise deployments.