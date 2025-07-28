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

### ❌ NOT Supported
Code formatting, Diagnostics, Code actions, Refactoring, Signature help, Semantic tokens, and all other LSP features.

## Quick Start

**Development Quick Start**:
```bash
make local && make test-simple-quick    # Build + global link + validate
lspg setup all                  # Complete setup
lspg server --config config.yaml       # Start server
```
**📖 Docs**: [README.md](README.md) (setup), [docs/troubleshooting.md](docs/troubleshooting.md) (diagnostics), [docs/test_guide.md](docs/test_guide.md) (testing)

## Architecture

### Request Flow
```
HTTP → Gateway → Router → LSPClient → LSP Server
MCP → ToolHandler → LSPGatewayClient → HTTP Gateway → Router → LSPClient → LSP Server
```

### SCIP Caching (v0.5.2)
- **L1 Memory**: <10ms, 8GB capacity, LRU
- **L2 Disk**: <50ms, memory-mapped files
- **Performance**: 60-87% faster, 85-90% cache hits, ~65-75MB memory

### Key Components
- **Gateway** (`internal/gateway/`): HTTP routing, JSON-RPC
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

### Testing
```bash
# Quick tests
make test-unit                    # Unit tests (<60s)
make test-simple-quick            # E2E validation (1min)
make test-python-patterns-quick   # Quick Python patterns (5-10min)

# Development workflow
make test-unit && make test-simple-quick         # Quick (2-5min)
make test-lsp-validation-short && make test-e2e-quick  # Pre-commit (5-10min)
make test-python-patterns && make test-python-comprehensive  # Python validation (20-25min)
make test                                        # Full suite
```

**Philosophy**: Essential tests only, focusing on real development workflows.

## Development Workflow

### Build & Quality
```bash
make local                    # Build + global link
make unlink                   # Remove global link
make build                    # Build all platforms
make quality                  # Format + lint + security
make release VERSION=v1.0.0   # Release build
```

### Development Cycle
```bash
make local && make test-simple-quick              # Quick cycle
make test-unit && make test-lsp-validation-short  # Pre-commit  
make test-python-patterns-quick                   # Python validation
make test && make quality                         # Before PR
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

**Transport**: `internal/transport/client.go:30`, `internal/transport/stdio.go:28`, `internal/transport/tcp.go:36`

**SCIP System**: `internal/indexing/scip_store.go:16`, `internal/storage/memory_cache.go:21`, `internal/indexing/watcher.go:37`

**MCP**: `mcp/server.go:139`, `mcp/tools.go:122`, `mcp/tools_scip_enhanced.go:215`

**Config/Setup**: `internal/config/multi_language_generator.go:570`, `internal/setup/orchestrator.go:228`, `internal/project/detector.go:132`

## Notes

**SCIP Performance**: 85-90% cache hits, 60-87% faster than pure LSP, ~65-75MB memory usage

**MCP Version**: 2025-06-18