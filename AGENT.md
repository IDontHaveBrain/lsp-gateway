# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

LSP Gateway is a dual-protocol Language Server Protocol gateway written in Go that provides:
- **HTTP JSON-RPC Gateway**: REST API at `localhost:8080/jsonrpc` for IDEs
- **Model Context Protocol (MCP) Server**: AI assistant integration interface  
- **Cross-platform CLI**: 20+ commands for setup, management, and diagnostics
- **SCIP Intelligent Caching**: High-performance indexing with 60-87% performance improvements

**Status**: MVP/Alpha - Development and testing use only
**Languages**: Go, Python, JavaScript/TypeScript, Java
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

## Supported LSP Features

**CRITICAL: LSP Gateway supports ONLY 6 specific LSP features**

The LSP Gateway is intentionally limited to core development features that provide maximum developer productivity with minimal complexity. All development, testing, and feature work must focus exclusively on these 6 features:

### ✅ Supported Features (6 Total)
1. **`textDocument/definition`** - Go to definition functionality
2. **`textDocument/references`** - Find all references to symbols
3. **`textDocument/hover`** - Hover information and documentation
4. **`textDocument/documentSymbol`** - Document symbol outline/tree
5. **`workspace/symbol`** - Workspace-wide symbol search
6. **`textDocument/completion`** - Code completion and IntelliSense

### ❌ Unsupported Features
**All other LSP features are explicitly NOT supported**, including but not limited to:
- Code formatting (`textDocument/formatting`)
- Diagnostics (`textDocument/publishDiagnostics`)
- Code actions (`textDocument/codeAction`)
- Refactoring (`textDocument/rename`)
- Signature help (`textDocument/signatureHelp`)
- Semantic tokens (`textDocument/semanticTokens`)

### Development Constraints
- **Feature Development**: Only implement and enhance the 6 supported features
- **Testing Focus**: All tests must validate only the 6 supported features
- **API Design**: Gateway APIs should only expose the 6 supported operations
- **Performance Optimization**: SCIP caching and performance improvements target only supported features
- **Error Handling**: Gracefully reject requests for unsupported LSP methods

**Rationale**: This focused approach ensures reliable, high-performance operation for essential development tasks while avoiding the complexity of maintaining dozens of LSP features.

## Development Guidelines

### Documentation Updates
**CRITICAL**: Always update related documentation after changes:
- **CLAUDE.md**: Commands, architecture, or workflow changes
- **README.md**: Installation, setup, or usage changes  
- **Config examples**: Schema or option changes
- **Test documentation**: Framework or procedure changes
- **AI Code Reference**: Keep `docs/ai_code_reference.md` updated with new critical files and code locations

### AI/Agent Code Navigation
**📖 AI Code Reference Guide**: See [docs/ai_code_reference.md](docs/ai_code_reference.md)

For AI/Agent quick navigation, this project maintains a comprehensive code reference document that includes:
- Critical file locations with `path:line` references
- Core entry points and business logic locations
- Key architecture components and their roles
- Important configuration and setup files
- Performance-critical code sections

**IMPORTANT**: Always update `docs/ai_code_reference.md` when:
- Adding new core functionality or entry points
- Modifying critical business logic locations
- Changing key configuration structures
- Refactoring major architectural components
- Adding new CLI commands or MCP tools

Verify all examples work before completing tasks.

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

### Essential Commands
```bash
# Setup and Installation
./bin/lsp-gateway setup all           # Complete automated setup
./bin/lsp-gateway setup wizard        # Interactive setup wizard
./bin/lsp-gateway install <server>     # Install specific language server

# Server Operations  
./bin/lsp-gateway server             # Start HTTP JSON-RPC gateway
./bin/lsp-gateway mcp                # Start MCP server (stdio/tcp)
./bin/lsp-gateway status             # Show server and language server status

# Configuration
./bin/lsp-gateway config generate    # Generate config from project detection
./bin/lsp-gateway config validate    # Validate configuration files

# Diagnostics
./bin/lsp-gateway diagnose           # Comprehensive system diagnostics
./bin/lsp-gateway verify             # Verify installation and configuration
```


## Testing Strategy

**📖 Complete Testing Guide**: See [docs/test_guide.md](docs/test_guide.md)

LSP Gateway uses a **streamlined, essential-only** testing approach focusing on real development workflows.

### Quick Test Commands
```bash
# Unit tests (fast - <60s)
make test-unit

# Quick E2E validation (1min)
make test-simple-quick

# LSP validation tests  
make test-lsp-validation-short  # 2 minutes
make test-lsp-validation        # 5 minutes

# Integration tests
make test-integration

# All tests
make test
```

### Testing Philosophy
- **Unit Tests**: Core logic only - no over-engineering
- **E2E Tests**: Real-world usage scenarios with actual language servers
- **Simplified Infrastructure**: Basic mocks and fixtures without complexity
- **Focus**: Essential functionality for local development workflows

For detailed testing guidelines, test infrastructure, and troubleshooting, see [docs/test_guide.md](docs/test_guide.md).

## Development Commands

### Build Commands
```bash
make local                    # Build for current platform  
make build                    # Build all platforms (Linux, Windows, macOS)
make clean                    # Clean build artifacts
make install                  # Install to GOPATH
make release VERSION=v1.0.0   # Release build
```

### Development Workflow
```bash
make deps                     # Download dependencies
make tidy                     # Tidy go modules  
make format                   # Format code
make lint                     # Run golangci-lint
make security                 # Run gosec security analysis
make quality                  # Format + lint + security
```

### Development Workflow
```bash
# Quick development cycle
make local && make test-simple-quick

# Pre-commit validation  
make test-unit && make test-lsp-validation-short

# Full validation before PR
make test && make quality
```

## Development Workflows

### Key Development Areas
- **Core Logic**: `internal/` packages with interface-based design
- **Configuration**: Templates in `config-templates/`, schema in `internal/config/`
- **Transport Layer**: STDIO/TCP implementations in `internal/transport/`
- **MCP Integration**: Protocol and tools in `mcp/`

## Installation and Setup

### Automated Setup (Recommended)
```bash
make local                           # Build for current platform
./bin/lsp-gateway setup all          # Installs runtimes + language servers
./bin/lsp-gateway server --config config.yaml
```

### Manual Setup
```bash
./bin/lsp-gateway setup runtimes                     # Install runtimes
./bin/lsp-gateway install <server>                   # Install language servers
./bin/lsp-gateway config generate --auto-detect      # Generate configuration
./bin/lsp-gateway verify                            # Validate setup
```

### Requirements
- **Go**: 1.24+ (core requirement)
- **Make**: Build system orchestration
- **Node.js**: 22.0.0+ (for npm integration)
- **Platform**: Linux, macOS (x64/arm64), Windows (x64)

## Troubleshooting

**📖 Complete Guide**: See [docs/troubleshooting.md](docs/troubleshooting.md)

### Quick Diagnostics
```bash
./bin/lsp-gateway diagnose    # Comprehensive diagnostics
./bin/lsp-gateway status      # Server status
./bin/lsp-gateway verify      # Verify installation
```

### Common Quick Fixes
- **Config issues**: `./bin/lsp-gateway config validate`
- **Language server issues**: `./bin/lsp-gateway install <server> --force`
- **Build issues**: `make clean && make local`

## Architecture Notes

### SCIP Caching Performance
- **Cache Hit Rate**: 85-90% for repeated symbol queries
- **Response Time Improvement**: 60-87% faster than pure LSP
- **Memory Usage**: ~65-75MB additional memory for cache