# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

LSP Gateway is a dual-protocol Language Server Protocol gateway written in Go that provides:
- **HTTP JSON-RPC Gateway**: REST API at `localhost:8080/jsonrpc` for IDEs
- **Model Context Protocol (MCP) Server**: AI assistant integration interface
- **Cross-platform CLI**: 20+ commands for setup, management, and diagnostics
- **Multi-Server Architecture**: Language pools with load balancing and circuit breakers
- **Enterprise Configuration**: Advanced optimization strategies and framework detection

**Status**: MVP/ALPHA - Use caution in production
**Languages**: Go, Python, JavaScript/TypeScript, Java, Rust
**Platforms**: Linux, Windows, macOS (x64/arm64)

## Development Guidelines

### General Development Practices

**CRITICAL: Always Update Related Documentation After Any Work**

After completing any development task, you **MUST** update all related documentation to ensure accuracy and completeness. This is a non-negotiable requirement for maintaining project quality.

#### Documentation Update Requirements

**When Any Change is Made:**
1. **CLAUDE.md**: Update if commands, architecture, or development workflows change
2. **README.md**: Update if installation, setup, or basic usage changes  
3. **Code Comments**: Update inline documentation for modified functions/classes
4. **API Documentation**: Update if endpoints, parameters, or responses change
5. **Configuration Examples**: Update if config schema or options change
6. **Test Documentation**: Update if testing procedures or frameworks change

#### Mandatory Documentation Updates For:

**Code Changes:**
- New CLI commands → Update CLAUDE.md CLI command structure
- New make targets → Update CLAUDE.md build commands section
- Architecture changes → Update CLAUDE.md architecture overview
- New dependencies → Update CLAUDE.md requirements section
- API changes → Update CLAUDE.md API usage examples

**Feature Additions:**
- New language servers → Update installation guides and supported languages
- New transport methods → Update configuration examples and architecture docs
- New MCP tools → Update MCP integration section
- New testing categories → Update testing infrastructure documentation

**Configuration Changes:**
- New config options → Update configuration examples and schema documentation
- Changed default values → Update all example configurations
- New environment variables → Update setup and deployment documentation

#### Documentation Verification Checklist

Before considering any task complete, verify:
- [ ] All command examples in documentation still work correctly
- [ ] Version numbers and requirements are current
- [ ] Architecture diagrams reflect actual implementation
- [ ] Code examples compile and execute successfully
- [ ] Installation instructions produce working setup
- [ ] API examples return expected responses
- [ ] Configuration examples are valid and complete

#### Documentation Standards
- **Accuracy**: All examples must be tested and functional
- **Completeness**: Include all necessary context and prerequisites
- **Clarity**: Write for developers unfamiliar with the codebase
- **Consistency**: Maintain formatting and style standards
- **Currency**: Remove outdated information immediately

**Remember: Outdated documentation is worse than no documentation. Always keep it current.**

## Quick Start (5 Minutes)

```bash
# 1. Clone and build (2 minutes)
git clone [repository-url]
cd lsp-gateway
make local

# 2. Automated setup (2 minutes)
./bin/lsp-gateway setup all          # Installs runtimes + language servers + config

# 3. Start using (30 seconds)
./bin/lsp-gateway server --config config.yaml    # HTTP Gateway (port 8080)
./bin/lsp-gateway mcp --config config.yaml       # MCP Server for AI assistants
```

## Architecture Overview

### Core Request Flow
```
HTTP → Gateway → Router → LSPClient → LSP Server
MCP → ToolHandler → LSPGatewayClient → HTTP Gateway → Router → LSPClient → LSP Server
```

### Advanced Architecture Components

#### Multi-Server Language Pools
The gateway supports sophisticated multi-server configurations with:
- **Language Pools**: Multiple language servers per language for load distribution
- **Circuit Breakers**: Automatic failure recovery with exponential backoff
- **Health Monitoring**: Real-time server health checks and automatic failover
- **Connection Pooling**: Dynamic connection pool sizing with utilization-based scaling

#### Transport Layer (`internal/transport/`)
- **STDIO Transport**: Process-based communication with circuit breakers
- **TCP Transport**: Connection pooling with three-state circuit breaker (Closed → Open → Half-Open)
- **Enhanced Pool**: Dynamic sizing with auto-scaling based on utilization
- **Performance Monitoring**: Real-time metrics with atomic operations

#### Configuration System (`internal/config/`)
Four-tier hierarchical configuration:
- **Template-driven Setup**: 15+ pre-configured templates for enterprise, full-stack, polyglot patterns
- **Framework Detection**: Auto-detection and optimization for React, Django, Spring Boot
- **Optimization Strategies**: Production, Development, Analysis modes with different resource allocations
- **Migration System**: Automatic configuration upgrades and validation

### Key Components
- **Gateway Layer** (`internal/gateway/`): HTTP routing, JSON-RPC protocol, multi-server management
- **Transport Layer** (`internal/transport/`): STDIO/TCP communication with circuit breakers
- **CLI Interface** (`internal/cli/`): Comprehensive command system with 20+ commands
- **Setup System** (`internal/setup/`): Cross-platform runtime detection and auto-installation
- **Platform Abstraction** (`internal/platform/`): Multi-platform package manager integration
- **MCP Integration** (`mcp/`): Model Context Protocol server exposing LSP as MCP tools
- **Project Detection** (`internal/project/`): Advanced multi-language project analysis
- **Configuration Management** (`internal/config/`): Hierarchical config with templates

## Advanced Configuration System

### Configuration Templates

The project includes 15+ configuration templates in `config-templates/`:

**Language-Specific Templates:**
- `go-advanced.yaml` - Advanced Go development with multiple gopls instances
- `java-spring.yaml` - Spring Boot optimized configuration with JDTLS
- `python-django.yaml` - Django development with pylsp and mypy integration
- `typescript-react.yaml` - React development with TypeScript language server
- `rust-workspace.yaml` - Rust workspace with multiple cargo projects

**Architecture Patterns:**
- `enterprise.yaml` - Production-ready multi-server configuration
- `full-stack.yaml` - Full-stack development with frontend/backend optimization
- `polyglot.yaml` - Multi-language monorepo configuration
- `monorepo-template.yaml` - Large monorepo with framework detection

### Multi-Server Configuration Example

```yaml
language_pools:
  python:
    servers:
      - name: "pylsp-primary"
        command: ["pylsp"]
        transport: "stdio"
        priority: 100
        max_concurrent_requests: 50
      - name: "pylsp-secondary" 
        command: ["pylsp", "--debug"]
        transport: "stdio"
        priority: 50
        max_concurrent_requests: 25
    circuit_breaker:
      error_threshold: 10
      timeout_duration: "30s"
      max_half_open_requests: 5
    load_balancing:
      strategy: "round_robin"
      health_check_interval: "10s"
```

### Framework Detection and Optimization

The configuration system includes automatic framework detection:
- **React Projects**: TypeScript optimization, JSX support, ESLint integration
- **Django Projects**: Python path configuration, database schema validation
- **Spring Boot**: Java classpath optimization, annotation processing
- **Kubernetes Projects**: YAML validation, schema completion

## Comprehensive CLI Commands

### Setup and Installation Commands
```bash
./bin/lsp-gateway setup all           # Install runtimes + servers + config
./bin/lsp-gateway setup runtimes     # Install language runtimes only
./bin/lsp-gateway setup servers      # Install language servers only
./bin/lsp-gateway setup config       # Generate configuration files
./bin/lsp-gateway install <server>   # Install specific language server
```

### Server Management Commands
```bash
./bin/lsp-gateway server             # Start HTTP JSON-RPC gateway
./bin/lsp-gateway mcp                # Start MCP server (stdio/tcp)
./bin/lsp-gateway status             # Show server and language server status
./bin/lsp-gateway verify             # Verify installation and configuration
```

### Configuration Commands
```bash
./bin/lsp-gateway config generate    # Generate config from project detection
./bin/lsp-gateway config validate    # Validate configuration files
./bin/lsp-gateway config upgrade     # Upgrade config to latest schema
./bin/lsp-gateway config templates   # List available templates
```

### Diagnostic Commands
```bash
./bin/lsp-gateway diagnose           # Comprehensive system diagnostics
./bin/lsp-gateway detect             # Detect project languages and frameworks
./bin/lsp-gateway health             # Health check all components
./bin/lsp-gateway performance        # Performance analysis and benchmarking
```

### Development Commands
```bash
./bin/lsp-gateway symbols <file>     # Extract symbols from file
./bin/lsp-gateway serve              # Development server with hot reload
./bin/lsp-gateway benchmark          # Run performance benchmarks
```

## E2E Testing Strategy

**📖 Complete E2E Testing Guide**: See [docs/e2e-testing.md](docs/e2e-testing.md)

LSP Gateway uses an **E2E test-first** approach, focusing on actual development workflows and usage scenarios.

### E2E Testing Core Principles
- **Real Workflow Testing**: Testing scenarios that developers actually use
- **Dual Protocol Coverage**: Verification of both HTTP JSON-RPC and MCP protocols
- **Language Server Integration**: Complete integration testing with actual language servers
- **Synthetic Project Validation**: Testing against generated realistic project structures

### Key E2E Testing Commands (Verified)
```bash
# Quick E2E validation (1min)
make test-simple-quick

# Unit tests only (<60s)
make test-unit

# Full LSP validation (5min)
make test-lsp-validation

# Short LSP validation (2min)
make test-lsp-validation-short

# Integration + performance tests
make test-integration

# Java LSP integration tests (10min)
make test-jdtls-integration

# All tests
make test
```

### E2E Testing Scenarios
1. **Basic Setup and Startup**: From complete setup to server startup
2. **HTTP JSON-RPC Protocol**: Verification of all LSP methods
3. **MCP Protocol**: AI assistant integration scenarios
4. **Multi-Language**: Go, Python, TypeScript, Java integration tests
5. **Performance & Load**: Concurrent request processing and Circuit Breaker testing
6. **Synthetic Projects**: Validation against generated realistic codebases

### Performance Testing
```bash
# Performance validation (30m-2h)
go test -v ./tests/performance -timeout 2h

# Benchmark testing
go test -v ./tests/performance -bench=BenchmarkFullPerformanceSuite -benchtime=10s
```

## Common Development Commands

### Build Commands
```bash
make local                    # Build for current platform
make build                    # Build all platforms
make clean                    # Clean build artifacts
make install                  # Install to GOPATH
make release VERSION=v1.0.0   # Release build
```

### Development Workflow Commands
```bash
make deps                     # Download dependencies
make tidy                     # Tidy go modules
make format                   # Format code
make lint                     # Run golangci-lint
make security                 # Run gosec security analysis
make check-deadcode          # Dead code analysis
make quality                 # Format + lint + security
```

### Testing Commands (E2E Focused)
```bash
make test                     # Run all tests
make test-unit               # Fast unit tests only (<60s)
make test-integration        # Integration + performance tests
make test-lsp-validation     # Comprehensive LSP validation (5min)
make test-lsp-validation-short # Short LSP validation (2min)
make test-simple-quick       # Quick validation for development (1min)
make test-jdtls-integration  # Java LSP integration tests (10min)
```

### Development Workflow Patterns
```bash
# Quick development cycle with E2E validation
make local && make test-simple-quick && make format

# Pre-commit validation
make test-unit && make test-lsp-validation-short && make security

# Full validation before PR
make test && make test-lsp-validation && make security
```

## Multi-Language Development Workflows

### Go Development
- **Entry Point**: `cmd/lsp-gateway/main.go`
- **Core Logic**: `internal/` packages with interface-based design
- **Testing**: Unit tests co-located with packages
- **Tools**: golangci-lint, gosec, go vet

### Configuration Development
- **Templates**: Add new templates to `config-templates/`
- **Schema**: Update `internal/config/schema.go` for new options
- **Validation**: Add validation rules in `internal/config/validation.go`
- **Migration**: Update `internal/config/migration.go` for schema changes

### Transport Layer Development
- **STDIO**: Modify `internal/transport/stdio.go`
- **TCP**: Modify `internal/transport/tcp.go`
- **Circuit Breakers**: Configure in `internal/transport/circuit_breaker.go`
- **Performance**: Monitor with `internal/transport/pool_metrics.go`

### MCP Integration Development
- **Protocol**: Update `mcp/server.go` for new MCP methods
- **Tools**: Add new LSP tools in `mcp/tools.go`
- **Transport**: Support both stdio and tcp in `mcp/transport.go`

## Installation and Setup

### Automated Setup (Recommended)
```bash
make local                           # Build for current platform
./bin/lsp-gateway setup all          # Installs runtimes + language servers
./bin/lsp-gateway server --config config.yaml
```

### Manual Setup
```bash
# 1. Install runtimes
./bin/lsp-gateway setup runtimes

# 2. Install language servers
./bin/lsp-gateway install gopls
./bin/lsp-gateway install pylsp
./bin/lsp-gateway install typescript-language-server
./bin/lsp-gateway install jdtls

# 3. Generate configuration
./bin/lsp-gateway config generate --template enterprise

# 4. Validate setup
./bin/lsp-gateway verify
```

### Requirements
- **Go**: 1.24+ (core requirement)
- **Make**: Build system orchestration
- **Node.js**: 22.0.0+ (for npm integration)
- **Platform**: Linux, macOS (x64/arm64), Windows (x64)

## Troubleshooting

**📖 Complete Troubleshooting Guide**: See [docs/troubleshooting.md](docs/troubleshooting.md)

For quick issue resolution, start with these diagnostic commands:

```bash
# Overall system health check
./bin/lsp-gateway health

# Comprehensive diagnostics
./bin/lsp-gateway diagnose

# Check server status
./bin/lsp-gateway status

# Verify installation
./bin/lsp-gateway verify
```

### Common Quick Fixes

**Configuration Issues**: `./bin/lsp-gateway config validate`
**Language Server Problems**: `./bin/lsp-gateway install <server> --force`
**Performance Issues**: `./bin/lsp-gateway performance`
**Connection Problems**: `./bin/lsp-gateway health`
**Build Issues**: `make clean && make local`
**Test Failures**: `make test-simple-quick`

For detailed troubleshooting steps, environment-specific issues, and advanced debugging techniques, see the [complete troubleshooting guide](docs/troubleshooting.md).

## Key File Locations

### Entry Points
- **Main CLI**: `cmd/lsp-gateway/main.go`
- **Multi-language CLI**: `cmd/multi-language-cli/main.go`
- **Test runners**: `cmd/test-multi-language/main.go`, `cmd/simple-multi-test/main.go`

### Core Components
- **CLI root**: `internal/cli/root.go`
- **Gateway logic**: `internal/gateway/handlers.go`
- **Configuration**: `internal/config/config.go`
- **Transport layer**: `internal/transport/`
- **MCP server**: `mcp/server.go`

### Configuration and Templates
- **Config templates**: `config-templates/`
- **Config documentation**: `config-templates/README.md`
- **Config system docs**: `internal/config/README.md`

### Testing Infrastructure
- **E2E Testing Guide**: `docs/e2e-testing.md`
- **Troubleshooting Guide**: `docs/troubleshooting.md`
- **Test framework**: `tests/framework/`


## Important Implementation Notes

### Circuit Breaker Configuration
Default circuit breaker settings across the system:
- **Error Threshold**: 10 errors before opening
- **Timeout Duration**: 30 seconds
- **Max Half-Open Requests**: 5 requests in half-open state
- **Rolling Window**: 60 seconds for error counting

### Performance Thresholds
- **Response Time**: 5s max response time
- **Throughput**: 100 req/sec minimum
- **Error Rate**: 5% maximum error rate
- **Memory Usage**: 3GB max memory, 1GB max growth

### Multi-Server Management
- **Health Check Interval**: 10 seconds
- **Load Balancing**: Round-robin with priority weighting
- **Connection Pool**: Dynamic sizing with utilization-based scaling
- **Failover**: Automatic failover to secondary servers

## Development Entry Points Summary

The project has **6 distinct entry points** for different use cases:
1. **`cmd/lsp-gateway/main.go`** - Production LSP Gateway
2. **`cmd/multi-language-cli/main.go`** - Development CLI tool
3. **`cmd/test-multi-language/main.go`** - E2E integration testing
4. **`cmd/basic-scanner-test/main.go`** - Scanner testing
5. **`cmd/simple-multi-test/main.go`** - Basic validation
6. **`cmd/standalone-test/main.go`** - Independent diagnostics

Choose the appropriate entry point based on your development needs.