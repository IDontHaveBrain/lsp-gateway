# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

LSP Gateway is a dual-protocol Language Server Protocol gateway written in Go that provides:
- **HTTP JSON-RPC Gateway**: REST API at `localhost:8080/jsonrpc` for IDEs
- **Model Context Protocol (MCP) Server**: AI assistant integration interface
- **Cross-platform CLI**: 20+ commands for setup, management, and diagnostics
- **Multi-Server Architecture**: Language pools with load balancing and circuit breakers
- **SCIP Intelligent Caching**: High-performance indexing with 60-87% performance improvements

**Status**: Functional - Core SCIP caching features completed
**Languages**: Go, Python, JavaScript/TypeScript, Java, Rust
**Platforms**: Linux, Windows, macOS (x64/arm64)

## LSP Development Philosophy

**IMPORTANT: LSP Gateway is a LOCAL DEVELOPMENT TOOL**

LSP servers are inherently local development tools that run on developers' machines without authentication or network access. Therefore:

**❌ AVOID THESE UNNECESSARY FEATURES:**
- **Monitoring & Observability**: No metrics collection, distributed tracing, or monitoring dashboards
- **Security & Authentication**: No authentication, authorization, or security scanning
- **Enterprise Scalability**: No distributed systems, horizontal scaling, or enterprise management
- **Production Infrastructure**: No containerization, service mesh, or deployment pipelines

**✅ FOCUS ON THESE CORE FEATURES:**
- **Local Performance**: Fast LSP responses and intelligent caching
- **Developer Experience**: Simple setup, clear diagnostics, and reliable operation
- **Multi-Language Support**: Seamless integration with popular language servers
- **Local Resource Management**: Efficient memory and CPU usage on developer machines

**Design Principle**: Keep it simple, local, and focused on developer productivity.

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

### Core Architecture Components

#### SCIP Intelligent Caching
The gateway includes SCIP v0.5.2 integration for high-performance caching:
- **Cache-First Routing**: Check SCIP cache before forwarding to LSP servers
- **Background Caching**: Store LSP responses in SCIP indexes for future queries
- **Cache Invalidation**: Update cache when source files change
- **Performance**: 60-87% response time improvements with 85-90% cache hit rates

#### Transport Layer (`internal/transport/`)
- **STDIO Transport**: Process-based communication with circuit breakers
- **TCP Transport**: Connection pooling with circuit breaker support
- **Connection Management**: Basic pooling and error recovery

#### Configuration System (`internal/config/`)
Simple hierarchical configuration:
- **Template-driven Setup**: Pre-configured templates for common development setups
- **Framework Detection**: Auto-detection for React, Django, Spring Boot
- **Local Optimization**: Development-focused resource allocation

### Key Components
- **Gateway Layer** (`internal/gateway/`): HTTP routing, JSON-RPC protocol, multi-server management
- **Transport Layer** (`internal/transport/`): STDIO/TCP communication with circuit breakers
- **CLI Interface** (`internal/cli/`): Comprehensive command system with 20+ commands
- **Setup System** (`internal/setup/`): Cross-platform runtime detection and auto-installation
- **Platform Abstraction** (`internal/platform/`): Multi-platform package manager integration
- **MCP Integration** (`mcp/`): Model Context Protocol server exposing LSP as MCP tools
- **Project Detection** (`internal/project/`): Advanced multi-language project analysis
- **Configuration Management** (`internal/config/`): Hierarchical config with templates

## Configuration System

### Configuration Templates

The project includes configuration templates in `config-templates/`:

**Language-Specific Templates:**
- `go-advanced.yaml` - Go development with gopls
- `java-spring.yaml` - Spring Boot development with JDTLS
- `python-django.yaml` - Django development with pylsp
- `typescript-react.yaml` - React development with TypeScript language server
- `rust-workspace.yaml` - Rust workspace configuration

**Project Patterns:**
- `full-stack.yaml` - Full-stack development setup
- `polyglot.yaml` - Multi-language project configuration

### Basic Configuration Example

```yaml
language_pools:
  python:
    servers:
      - name: "pylsp-primary"
        command: ["pylsp"]
        transport: "stdio"
    circuit_breaker:
      error_threshold: 10
      timeout_duration: "30s"
```

### Framework Detection

The configuration system includes automatic framework detection:
- **React Projects**: TypeScript optimization, JSX support
- **Django Projects**: Python path configuration
- **Spring Boot**: Java classpath optimization

## CLI Commands

### Setup and Installation Commands
```bash
./bin/lsp-gateway setup all           # Complete automated setup
./bin/lsp-gateway setup wizard        # Interactive setup wizard
./bin/lsp-gateway setup detect        # Auto-detect and setup for current project
./bin/lsp-gateway setup multi-language # Multi-language project setup
./bin/lsp-gateway setup template       # Template-based setup
./bin/lsp-gateway install <server>     # Install specific language server
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
./bin/lsp-gateway config migrate     # Migrate legacy config to latest schema
./bin/lsp-gateway config show        # Display current configuration
./bin/lsp-gateway config optimize    # Optimize configuration for performance
```

### Diagnostic Commands
```bash
./bin/lsp-gateway diagnose           # Comprehensive system diagnostics
./bin/lsp-gateway setup detect       # Detect project languages and frameworks
./bin/lsp-gateway diagnose performance # Performance analysis and benchmarking
```


## E2E Testing Strategy

**📖 Complete E2E Testing Guide**: See [docs/e2e_testing_guide.md](docs/e2e_testing_guide.md)

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

# Circuit breaker E2E tests (5min)
make test-circuit-breaker

# Comprehensive circuit breaker scenarios (10min)
make test-circuit-breaker-comprehensive

# All tests
make test
```

### E2E Testing Scenarios
1. **Basic Setup and Startup**: From complete setup to server startup
2. **HTTP JSON-RPC Protocol**: Verification of all LSP methods
3. **MCP Protocol**: AI assistant integration scenarios
4. **Multi-Language**: Go, Python, TypeScript, Java integration tests
5. **Performance & Load**: Concurrent request processing and Circuit Breaker testing
6. **Circuit Breaker Scenarios**: Comprehensive failure handling, recovery, error categorization, and performance impact testing
7. **Synthetic Projects**: Validation against generated realistic codebases

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
./bin/lsp-gateway config generate --auto-detect

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
# Comprehensive diagnostics and health check
./bin/lsp-gateway diagnose

# Check server status
./bin/lsp-gateway status

# Verify installation
./bin/lsp-gateway verify
```

### Common Quick Fixes

**Configuration Issues**: `./bin/lsp-gateway config validate`
**Language Server Problems**: `./bin/lsp-gateway install <server> --force`
**Performance Issues**: `./bin/lsp-gateway diagnose performance`
**System Problems**: `./bin/lsp-gateway diagnose`
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

### Testing Infrastructure
- **E2E Testing Guide**: `docs/e2e_testing_guide.md`
- **Troubleshooting Guide**: `docs/troubleshooting.md`
- **Test framework**: `tests/framework/`


## Important Implementation Notes

### Circuit Breaker Configuration
Default circuit breaker settings:
- **Error Threshold**: 10 errors before opening
- **Timeout Duration**: 30 seconds
- **Max Half-Open Requests**: 5 requests in half-open state

### SCIP Caching Performance
- **Cache Hit Rate**: 85-90% for repeated symbol queries
- **Response Time Improvement**: 60-87% faster than pure LSP
- **Memory Usage**: ~65-75MB additional memory for cache

## Development Entry Points Summary

The project has **6 distinct entry points** for different use cases:
1. **`cmd/lsp-gateway/main.go`** - Production LSP Gateway
2. **`cmd/multi-language-cli/main.go`** - Development CLI tool
3. **`cmd/test-multi-language/main.go`** - E2E integration testing
4. **`cmd/basic-scanner-test/main.go`** - Scanner testing
5. **`cmd/simple-multi-test/main.go`** - Basic validation
6. **`cmd/standalone-test/main.go`** - Independent diagnostics

Choose the appropriate entry point based on your development needs.