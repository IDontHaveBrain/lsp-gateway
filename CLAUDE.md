# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with this LSP Gateway repository.

## Quick Reference

```bash
# Prerequisites
go version      # Requires Go 1.24.0+ (toolchain 1.24.5)
node --version  # Requires Node.js 18+

# Build and install
make local                     # Build + npm link globally
make clean                     # Clean build artifacts

# Run servers
lsp-gateway server            # HTTP Gateway (port 8080)
lsp-gateway mcp              # MCP Server (auto-detects languages)
lsp-gateway status           # Check LSP server availability

# Quality checks
make quality                  # Format + vet
make quality-full            # Format + vet + lint + security  
go test -v ./tests/unit/...  # Unit tests
```

## Project Overview

LSP Gateway provides dual-protocol access to Language Server Protocol servers:
- **HTTP JSON-RPC Gateway** at `localhost:8080/jsonrpc` for IDE integration
- **MCP Server** for AI assistants (STDIO protocol, 512MB cache, 1hr TTL)
- **SCIP Cache** - Sub-millisecond symbol lookups with LRU eviction
- **Auto-detection** - Scans for go.mod, package.json, *.py, pom.xml

**Languages**: Go, Python, JavaScript, TypeScript, Java (5 total)
**LSP Methods**: definition, references, hover, documentSymbol, workspace/symbol, completion (6 total)
**MCP Tools**: findSymbols, findReferences (2 enhanced tools)

## Build & Development

```bash
# Build commands
make local          # Build + npm link (primary workflow)
make build          # Build all platforms  
make clean          # Clean artifacts
make tidy           # Tidy go modules

# Quality checks
make quality        # format + vet
make quality-full   # format + vet + lint + security

# Testing - Automatically discovers and runs all tests
make test                            # Run ALL tests (unit + integration + e2e)
make test-fast                       # Unit + integration only (quick feedback)
go test -v ./tests/unit/...         # Unit tests only  
go test -v ./tests/integration/...  # Integration tests only
go test -v ./tests/e2e/...          # E2E tests only (30min timeout)
make cache-test                      # Cache tests

# Run a single test
go test -v -run TestFunctionName ./tests/unit/...  # Single unit test
go test -v -run TestFunctionName ./tests/e2e/...   # Single E2E test

# LSP server installation (required)
lsp-gateway install all              # Install all 5 language servers
lsp-gateway install go               # Install Go LSP (gopls)
lsp-gateway install python           # Install Python LSP
lsp-gateway install typescript       # Install TypeScript LSP
lsp-gateway install javascript       # Install JavaScript LSP
lsp-gateway install java             # Install Java LSP (jdtls)

# Runtime commands
lsp-gateway server                   # HTTP Gateway on :8080
lsp-gateway mcp                      # MCP Server (STDIO)
lsp-gateway status                   # Check LSP availability
lsp-gateway test                     # Test connections
lsp-gateway version                  # Show version information
lsp-gateway cache info               # Cache statistics
lsp-gateway cache clear              # Clear cache
lsp-gateway cache index              # Index cache for faster lookups
```

## Architecture

### Core Structure
```
src/
├── cmd/lsp-gateway/  # CLI entry point (main.go)
├── server/           # Server implementations
│   ├── lsp_manager.go       # LSP orchestration with optional SCIP cache
│   ├── gateway.go           # HTTP JSON-RPC gateway
│   ├── mcp_server.go        # MCP server (enhanced mode)
│   ├── mcp_tools.go         # 2 MCP tool handlers (findSymbols, findReferences)
│   ├── aggregators/         # Workspace symbol aggregation
│   ├── capabilities/        # LSP capability detection
│   ├── documents/           # Document management
│   ├── errors/              # Error translation
│   ├── cache/               # SCIP cache system
│   ├── scip/                # SCIP protocol implementation
│   ├── process/             # LSP server lifecycle
│   └── protocol/            # JSON-RPC handling
├── cli/              # CLI commands
│   ├── server_commands.go   # server, mcp, status, test, version
│   ├── install_commands.go  # LSP server installers
│   └── cache_commands.go    # cache info, clear, index
├── config/           # YAML config and auto-detection
├── utils/            # URI utilities
└── internal/
    ├── installer/    # Language-specific installers
    ├── project/      # Language detection, package info
    ├── models/       # LSP protocol models
    ├── types/        # Shared type definitions
    ├── errors/       # Error type definitions
    ├── security/     # Command validation
    ├── version/      # Version management
    ├── constants/    # System limits and timeouts
    └── common/       # STDIO-safe logging

```

### Critical: STDIO-Safe Logging

**MANDATORY**: Use only stderr-based loggers from `src/internal/common/logging.go`:
```go
import "lsp-gateway/src/internal/common"

common.LSPLogger.Info("message")     // LSP/MCP operations
common.GatewayLogger.Error("error")  // HTTP gateway  
common.CLILogger.Warn("warning")     // CLI commands

// NEVER USE (breaks LSP protocol):
fmt.Print*, log.Print*, log.New()    // ❌ Uses stdout
```

### Key Design Patterns

**Cache Integration** (interface-based, graceful degradation):
```go
if cfg.Cache != nil && cfg.Cache.Enabled {
    scipCache, err := cache.NewSCIPCacheManager(cfg.Cache)
    if err != nil {
        common.LSPLogger.Warn("Continuing without cache: %v", err)
    } else {
        manager.scipCache = scipCache
    }
}
```

**Constructor Pattern**: Use `New*` prefix returning `(*Type, error)`

## System Constants

- **Timeouts**: Java 60s, Python 30s, Go/JS/TS 15s
- **Cache**: 512MB memory, 24h TTL (1h for MCP), 5min health checks (2min for MCP)
- **Process**: 5s graceful shutdown
- **Scanning**: 3-level directory depth, skips vendor/node_modules
- **MCP**: 50 files auto-index limit, enhanced mode only

## Testing

E2E tests use real GitHub repositories:
- Go: `gorilla/mux` (v1.8.0)
- Python: `psf/requests` (v2.28.1)
- JavaScript: `ramda/ramda` (v0.31.3)
- TypeScript: `sindresorhus/is` (v5.4.1)
- Java: `spring-projects/spring-petclinic` (main)

## Configuration

Auto-detection scans for: go.mod, package.json, *.py, pom.xml

Config file: `~/.lsp-gateway/config.yaml`
```yaml
cache:
  enabled: true
  max_memory_mb: 512
  ttl_hours: 24          # MCP overrides to 1hr
servers:
  go:
    command: "gopls"
    args: ["serve"]
```

## Common Development Tasks

### Debug LSP Communication
```bash
export LSP_GATEWAY_DEBUG=true
lsp-gateway server

# Test with curl
curl -X POST localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"textDocument/definition","params":{"textDocument":{"uri":"file:///path/to/file.go"},"position":{"line":10,"character":5}},"id":1}'
```

### Add New LSP Method
1. Update `src/server/capabilities/detector.go`
2. Add error translation in `src/server/errors/translator.go`
3. Update MCP tools in `src/server/mcp_tools.go`
4. Add E2E test

## Important Constraints

- **5 languages**: Go, Python, JavaScript, TypeScript, Java
- **6 LSP methods**: definition, references, hover, documentSymbol, workspace/symbol, completion
- **2 MCP tools**: findSymbols, findReferences (enhanced SCIP-based tools)
- **Local only**: No enterprise features (auth, monitoring, distributed)
- **Untyped JSON-RPC**: Uses `interface{}` for params/results

## Common Issues

1. **STDIO Pollution**: Never use `fmt.Print*` - breaks LSP protocol
2. **Cache Nil Checks**: Always check `if m.scipCache != nil`
3. **LSP Not Found**: Run `lsp-gateway install all` first
4. **Port Conflict**: Use `--port 8081` flag
5. **Slow First Run**: E2E tests clone repos (~30s per repo)