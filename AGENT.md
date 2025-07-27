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

**Rationale**: This focused approach ensures reliable, high-performance operation for essential development tasks while avoiding the complexity of maintaining dozens of LSP features.

## Quick Start Reference

**📖 User Quick Start Guide**: See [README.md](README.md) for complete 5-minute setup instructions, installation procedures, and basic usage examples.

**Development Quick Start**:
```bash
make local && make test-simple-quick    # Build and validate (3 minutes)
./bin/lsp-gateway setup all            # Complete automated setup
./bin/lsp-gateway server --config config.yaml    # Start development server
```

## Architecture Overview

### Core Request Flow
```
HTTP → Gateway → Router → LSPClient → LSP Server
MCP → ToolHandler → LSPGatewayClient → HTTP Gateway → Router → LSPClient → LSP Server
```

### SCIP Intelligent Caching Architecture
The gateway includes sophisticated SCIP v0.5.2 integration with multi-tier storage:

**Three-Tier Storage System**:
- **L1 Memory**: <10ms access, 8GB capacity, LRU with compression
- **L2 Disk**: SSD/HDD with memory-mapped files, <50ms access  
- **L3 Remote**: Redis/distributed cache, <200ms access

**Performance Characteristics**:
- **Cache-First Routing**: Check SCIP cache before forwarding to LSP servers
- **Background Indexing**: Non-blocking response caching with confidence scoring
- **File Watching**: Real-time cache invalidation with <1s detection
- **Performance**: 60-87% response time improvements with 85-90% cache hit rates
- **Memory Usage**: ~65-75MB additional memory for cache
- **Intelligent Promotion**: Hot data automatically moved to faster tiers

### Key Components
- **Gateway Layer** (`internal/gateway/`): HTTP routing, JSON-RPC protocol, multi-server management
- **Transport Layer** (`internal/transport/`): STDIO/TCP communication with circuit breakers
- **CLI Interface** (`internal/cli/`): Comprehensive command system with 20+ commands
- **Setup System** (`internal/setup/`): Cross-platform runtime detection and auto-installation
- **Platform Abstraction** (`internal/platform/`): Multi-platform package manager integration
- **MCP Integration** (`mcp/`): Model Context Protocol server exposing LSP as MCP tools
- **Project Detection** (`internal/project/`): Advanced multi-language project analysis
- **Configuration Management** (`internal/config/`): Hierarchical config with templates

## Development CLI Commands

**📖 User Command Guide**: See [README.md](README.md) for essential user commands and basic usage.

**📖 Troubleshooting Commands**: See [docs/troubleshooting.md](docs/troubleshooting.md) for comprehensive diagnostic commands, setup troubleshooting, and problem resolution.

**Development-Focused Commands**:
```bash
# Development Server Operations  
./bin/lsp-gateway server --config config.yaml    # HTTP gateway for development
./bin/lsp-gateway mcp --config config.yaml       # MCP server for AI integration
./bin/lsp-gateway diagnose                       # Quick development diagnostics

# Configuration for Development
./bin/lsp-gateway config generate --auto-detect  # Auto-generate development config
./bin/lsp-gateway config validate               # Validate configuration files
./bin/lsp-gateway setup all                     # Complete development setup

# Development Diagnostics
./bin/lsp-gateway verify                        # Verify development environment
./bin/lsp-gateway status                        # Check server status
```

## Testing Strategy

**📖 Complete Testing Guide**: See [docs/test_guide.md](docs/test_guide.md) for comprehensive test commands, timing estimates, language-specific testing, MCP protocol tests, and testing infrastructure details.

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

### Development Testing Workflow
```bash
# Quick development validation (2-5 minutes)
make test-unit && make test-simple-quick

# Pre-commit validation (5-10 minutes)  
make test-lsp-validation-short && make test-e2e-quick

# Comprehensive testing (15-30 minutes)
make test-e2e-full && make test-mcp-scip
```

### Testing Philosophy
- **Unit Tests**: Core logic only - no over-engineering
- **E2E Tests**: Real-world usage scenarios with actual language servers
- **Simplified Infrastructure**: Basic mocks and fixtures without complexity
- **Focus**: Essential functionality for local development workflows

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

## Development Guidelines

### Documentation Updates
**CRITICAL**: Always update related documentation after changes:
- **CLAUDE.md**: Commands, architecture, or workflow changes
- **README.md**: Installation, setup, or usage changes  
- **Config examples**: Schema or option changes
- **Test documentation**: Framework or procedure changes
- **AI Code Reference**: Keep `docs/ai_code_reference.md` updated with new critical files and code locations

### AI/Agent Code Navigation
**📖 AI Code Reference Guide**: See [docs/ai_code_reference.md](docs/ai_code_reference.md) for comprehensive code navigation with precise file:line references, architecture components, SCIP performance system locations, MCP integration details, and critical entry points.

**IMPORTANT**: Always update `docs/ai_code_reference.md` when:
- Adding new core functionality or entry points
- Modifying critical business logic locations
- Changing key configuration structures
- Refactoring major architectural components
- Adding new CLI commands or MCP tools

## Development Setup

**📖 Complete Installation Guide**: See [README.md](README.md) for detailed installation instructions, requirements, and platform support.

**Development Environment Setup**:
```bash
make local                           # Build for development
make test-unit                       # Validate build
./bin/lsp-gateway setup all          # Complete development setup
./bin/lsp-gateway diagnose           # Verify development environment
```

**Development Requirements**:
- **Go**: 1.24+ (project development standard)
- **Make**: Build system for development workflows
- **Git**: Version control and development workflow integration

## Development Troubleshooting

**📖 Complete Troubleshooting Guide**: See [docs/troubleshooting.md](docs/troubleshooting.md) for comprehensive diagnostic procedures, SCIP caching issues, MCP protocol debugging, and platform-specific solutions.

**Quick Development Diagnostics**:
```bash
./bin/lsp-gateway diagnose    # Comprehensive development diagnostics
make test-unit               # Validate core functionality
make clean && make local     # Clean rebuild for development
```

## Critical File Locations (AI Navigation)

### Core Entry Points
- `cmd/lsp-gateway/main.go:10` - Application main entry
- `internal/cli/root.go:64` - CLI framework execution
- `internal/cli/server.go:52` - HTTP gateway server startup
- `internal/cli/mcp.go:82` - MCP server startup

### Gateway Core
- `internal/gateway/interface.go:10` - Gateway interface definition
- `internal/gateway/handlers.go:889` - Main request handling logic
- `internal/gateway/smart_router.go:165` - Intelligent routing strategies

### Transport Layer
- `internal/transport/client.go:30` - LSP client interface
- `internal/transport/stdio.go:28` - STDIO transport implementation
- `internal/transport/tcp.go:36` - TCP transport implementation

### SCIP Performance System
- `internal/indexing/scip_store.go:16` - SCIP storage interface
- `internal/storage/memory_cache.go:21` - L1 memory cache
- `internal/indexing/watcher.go:37` - File system watcher

### MCP Integration
- `mcp/server.go:139` - MCP server implementation
- `mcp/tools.go:122` - Standard LSP tools
- `mcp/tools_scip_enhanced.go:215` - AI-powered semantic tools

### Configuration & Setup
- `internal/config/multi_language_generator.go:570` - Template generation
- `internal/setup/orchestrator.go:228` - Setup automation workflow
- `internal/project/detector.go:132` - Project detection logic

## Architecture Notes

### SCIP Caching Performance
- **Cache Hit Rate**: 85-90% for repeated symbol queries
- **Response Time Improvement**: 60-87% faster than pure LSP
- **Memory Usage**: ~65-75MB additional memory for cache