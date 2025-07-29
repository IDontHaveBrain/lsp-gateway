# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Project Overview

LSP Gateway is a dual-protocol Language Server Protocol gateway written in Go:
- **HTTP JSON-RPC Gateway**: REST API at `localhost:8080/jsonrpc` for IDEs
- **Model Context Protocol (MCP) Server**: AI assistant integration  
- **Cross-platform CLI**: 20+ commands for setup/management
- **SCIP Intelligent Caching**: 60-87% performance improvements

**Status**: MVP/Alpha | **Languages**: Go, Python, JS/TS, Java | **Platforms**: Linux, Windows, macOS

## LSP Development Philosophy

**LSP Gateway is a LOCAL DEVELOPMENT TOOL**

**❌ AVOID:** Monitoring, Security/Auth, Enterprise scaling, Production infrastructure  
**✅ FOCUS:** Local performance, Developer experience, Multi-language support, Resource management

**Design Principle**: Keep it simple, local, and focused on developer productivity.

## Supported LSP Features (6 ONLY)

### ✅ Supported
1. **`textDocument/definition`** - Go to definition
2. **`textDocument/references`** - Find references
3. **`textDocument/hover`** - Hover info
4. **`textDocument/documentSymbol`** - Document symbols
5. **`workspace/symbol`** - Workspace search
6. **`textDocument/completion`** - Code completion

### 🚀 Enhanced Capabilities
- **Multi-project workspace support** - Cross-language symbol resolution
- **Intelligent sub-project routing** - Context-aware request routing
- **Performance-optimized caching** - SCIP + workspace-aware caching

### ❌ NOT Supported
Code formatting, Diagnostics, Code actions, Refactoring, Signature help, Semantic tokens, and all other LSP features.

## Quick Start

**Development Quick Start**:
```bash
make local && make test-simple-quick    # Build + global link + validate
lspg setup all                          # Complete setup + workspace detection
lspg server --config config.yaml       # Start with workspace support
```
**📖 Docs**: [README.md](README.md) (setup), [docs/troubleshooting.md](docs/troubleshooting.md) (diagnostics), [docs/test_guide.md](docs/test_guide.md) (testing)

## Architecture

### Request Flow
```
HTTP → Gateway → WorkspaceManager → Router → LSPClient → LSP Server
MCP → ToolHandler → LSPGatewayClient → HTTP Gateway → WorkspaceManager → Router → LSPClient → LSP Server
```

### Workspace Management System
- **Multi-Project Support** (`internal/workspace/`): Hierarchical workspace configuration, cross-language project detection, sub-project routing
- **Workspace Gateway** (`internal/workspace/gateway.go`): Simplified single workspace operation with sub-project routing capabilities
- **Client Management** (`internal/workspace/client_manager.go`): Language-specific LSP client lifecycle management
- **Configuration Manager** (`internal/workspace/config_manager.go`): Workspace-specific configuration loading with global → workspace → environment hierarchy
- **Sub-Project Resolution** (`internal/workspace/subproject_resolver.go`): Intelligent routing for complex project structures

### SCIP Caching (v0.5.2)
- **L1 Memory**: <10ms, 8GB capacity, LRU
- **L2 Disk**: <50ms, memory-mapped files
- **Performance**: 60-87% faster, 85-90% cache hits, ~65-75MB memory

### Key Components
- **Gateway** (`internal/gateway/`): HTTP routing, JSON-RPC
- **Workspace** (`internal/workspace/`): Multi-project management, configuration hierarchy, sub-project routing
- **Transport** (`internal/transport/`): STDIO/TCP with circuit breakers
- **CLI** (`internal/cli/`): 20+ command system
- **Setup** (`internal/setup/`): Auto-installation
- **MCP** (`mcp/`): Model Context Protocol server
- **Config** (`internal/config/`): Hierarchical templates

## Commands & Testing

### Key Commands
```bash
# Server Operations
./bin/lspg server --config config.yaml    # HTTP gateway
./bin/lspg mcp --config config.yaml       # MCP server
./bin/lspg diagnose                       # Diagnostics
./bin/lspg setup all                      # Auto setup
```

### Testing Philosophy & Capabilities
- **Parallel Execution Optimization**: Up to 3x CPU cores for maximum throughput
- **Multi-Project Testing**: Complete workspace scenarios with cross-language validation
- **Real Server Integration**: Actual language server testing (JDTLS, pylsp, gopls, typescript-language-server)
- **Performance Testing**: Memory usage, cache hit rates, parallel execution benchmarks
- **MCP Protocol Validation**: STDIO/TCP transport, tools integration, SCIP enhancement

### Testing Targets by Category

**Quick Validation (2-5min)**:
```bash
make test-simple-quick            # Basic E2E validation
make test-lsp-validation-short    # Short LSP validation
make test-parallel-validation     # Standard parallel validation
```

**Language-Specific Real Server Testing (10-20min each)**:
```bash
make test-go-real-client          # Go with golang/example repo
make test-java-real-client        # Java with clean-code repo
make test-javascript-real-client  # JavaScript with chalk repo
make test-typescript-real         # TypeScript comprehensive
make test-python-patterns         # Python with real patterns
```

**Multi-Project & Performance (15-30min)**:
```bash
make test-multi-project-workspace # Cross-language workspace testing
make test-parallel-performance    # Performance comparison
make test-workspace-integration   # Workspace component integration
```

**Parallel Execution Optimization**:
```bash
make test-parallel-fast              # 3x CPU cores (fastest)
make test-parallel-memory-optimized  # 1x CPU cores (memory efficient)
```

## Development Workflow

### Build & Quality
```bash
make local                    # Build + global link
make unlink                   # Remove global link
make build                    # Build all platforms
make quality                  # Format + lint + security
make release VERSION=v1.0.0   # Release build
```

### Enhanced Development Workflows
```bash
# Quick development cycle (2-5min)
make local && make test-simple-quick

# Pre-commit validation (5-15min)
make test-unit && make test-lsp-validation-short && make test-parallel-validation

# Language-specific validation (10-20min each)
make test-python-patterns-quick          # Python development
make test-javascript-quick               # JavaScript development
make test-typescript-quick               # TypeScript development

# Performance & integration validation (15-30min)
make test-workspace-integration && make test-parallel-performance

# Full validation before PR (30-60min)
make test && make test-workspace-comprehensive && make quality
```

## Development Guidelines

### Documentation
Always update docs after changes:
- **CLAUDE.md**: Commands, architecture, workflows
- **README.md**: Installation, setup, usage  
- **docs/ai_code_reference.md**: Critical file locations (update when adding core functionality)

### Setup & Troubleshooting
**Requirements**: Go 1.24+, Make, Git

```bash
# Setup
make local && lspg setup all

# Diagnostics
lspg diagnose
make clean && make local     # Clean rebuild
```

**📖 Full Guides**: [README.md](README.md), [docs/troubleshooting.md](docs/troubleshooting.md), [docs/ai_code_reference.md](docs/ai_code_reference.md)

## Critical File Locations

**Entry Points**: `cmd/lsp-gateway/main.go:10`, `internal/cli/root.go:64`, `internal/cli/server.go:52`, `internal/cli/mcp.go:82`

**Gateway Core**: `internal/gateway/interface.go:10`, `internal/gateway/handlers.go:889`, `internal/gateway/smart_router.go:165`

**Workspace System**: `internal/workspace/gateway.go:18`, `internal/workspace/config_manager.go:162`, `internal/workspace/client_manager.go`, `internal/workspace/subproject_resolver.go`

**Transport**: `internal/transport/client.go:30`, `internal/transport/stdio.go:28`, `internal/transport/tcp.go:36`

**SCIP System**: `internal/indexing/scip_store.go:16`, `internal/storage/memory_cache.go:21`, `internal/indexing/watcher.go:37`

**MCP**: `mcp/server.go:139`, `mcp/tools.go:122`, `mcp/tools_scip_enhanced.go:215`

**Config/Setup**: `internal/config/multi_language_generator.go:570`, `internal/setup/orchestrator.go:228`, `internal/project/detector.go:132`

**Testing Infrastructure**: `tests/e2e/multi_project_workspace_e2e_test.go:17`, `tests/e2e/testutils/multi_project_manager.go`, `tests/integration/workspace/`

## Notes

**SCIP Performance**: 85-90% cache hits, 60-87% faster than pure LSP, ~65-75MB memory usage

**MCP Version**: 2025-06-18