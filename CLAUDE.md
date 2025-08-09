# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with this LSP Gateway repository.

## Getting Started

```bash
# Initial setup
git clone https://github.com/IDontHaveBrain/lsp-gateway
cd lsp-gateway
make local                    # Build + npm link globally
lsp-gateway install all       # Install all 5 language servers

# Prerequisites
go version      # Requires Go 1.24.0+ (toolchain 1.24.5)
node --version  # Requires Node.js 18+
```

## Quick Reference

```bash
# Build and install
make local                     # Primary workflow: build + npm link
make clean                     # Clean build artifacts
make tidy                      # Tidy go modules

# Run servers
lsp-gateway server            # HTTP Gateway (port 8080)
lsp-gateway server --port 8081 # Use different port if 8080 busy
lsp-gateway mcp              # MCP Server (STDIO protocol)
lsp-gateway status           # Check LSP server availability

# Quality checks
make quality                  # Essential: format + vet
make quality-full            # Complete: format + vet + lint + security
make test-fast               # Unit + integration (8-10 min)
make test                    # All tests including E2E (30+ min)

# Test commands
go test -v ./tests/unit/...         # Unit tests only (2 min)
go test -v ./tests/integration/...  # Integration tests (10 min)
go test -v ./tests/e2e/...          # E2E tests (30 min)
go test -v -run TestName ./tests/...  # Run specific test
```

## Project Overview

LSP Gateway provides dual-protocol access to Language Server Protocol servers:
- **HTTP JSON-RPC Gateway** at `localhost:8080/jsonrpc` for IDE integration
- **MCP Server** for AI assistants (STDIO protocol, 512MB cache, 1hr TTL)
- **SCIP Cache** - Sub-millisecond symbol lookups with LRU eviction
- **Auto-detection** - Scans for go.mod, package.json, *.py, pom.xml

**Languages**: Go, Python, JavaScript, TypeScript, Java (5 total)
**LSP Methods**: definition, references, hover, documentSymbol, workspace/symbol, completion (6 total)
**MCP Tools**: findSymbols, findReferences (2 enhanced SCIP-based tools)

## Build & Development

```bash
# Build commands
make local          # Primary workflow: build + npm link with cache enabled
make build          # Build all platforms (linux/darwin/windows)
make clean          # Clean artifacts and go clean
make unlink         # Remove npm global link

# Platform-specific builds
make linux          # Linux amd64
make macos          # Darwin amd64
make macos-arm64    # Darwin arm64 (M1/M2)
make windows        # Windows amd64

# Quality & maintenance
make deps           # Download dependencies
make tidy           # Tidy go modules
make format         # Format code with go fmt
make vet            # Run go vet
make quality        # Essential: format + vet
make quality-full   # Complete: format + vet + lint + security

# Testing hierarchy
make test           # Complete suite: unit + integration + E2E (30+ min)
make test-fast      # Development: unit + integration only (8-10 min)
make test-unit      # Unit tests with 120s timeout
make test-integration # Integration tests with 600s timeout
make test-e2e       # E2E tests with 1800s (30min) timeout
make test-quick     # Quick validation tests
make cache-test     # Cache-specific tests with 300s timeout

# LSP server installation (required before first use)
lsp-gateway install all              # Install all 5 language servers
lsp-gateway install go               # Install gopls
lsp-gateway install python           # Install Python LSP (pylsp)
lsp-gateway install typescript       # Install TypeScript LSP
lsp-gateway install javascript       # Install JavaScript LSP
lsp-gateway install java             # Install Java LSP (jdtls)

# Runtime commands
lsp-gateway server                   # HTTP Gateway on :8080
lsp-gateway server --port 8081       # Use alternate port
lsp-gateway mcp                      # MCP Server (STDIO protocol)
lsp-gateway status                   # Check LSP server availability
lsp-gateway test                     # Test LSP connections
lsp-gateway version                  # Show version info
lsp-gateway cache info               # Display cache statistics
lsp-gateway cache clear              # Clear SCIP cache
lsp-gateway cache index              # Index workspace for faster lookups
```

## Architecture

### Core Structure
```
src/
├── cmd/lsp-gateway/  # CLI entry point (main.go)
├── server/           # Server implementations
│   ├── lsp_manager.go       # Central LSP orchestration with SCIP cache
│   ├── gateway.go           # HTTP JSON-RPC gateway (:8080/jsonrpc)
│   ├── mcp_server.go        # MCP server (STDIO, enhanced mode)
│   ├── mcp_tools.go         # MCP tool handlers (findSymbols, findReferences)
│   ├── lsp_client_manager.go # Language-specific client management
│   ├── aggregators/         # Parallel workspace symbol aggregation
│   ├── capabilities/        # Dynamic LSP capability detection
│   ├── documents/           # Document state management
│   ├── errors/              # Error code translation
│   ├── cache/               # SCIP cache system (manager.go - 2000+ lines)
│   ├── scip/                # SCIP protocol implementation
│   ├── process/             # LSP server lifecycle management
│   └── protocol/            # JSON-RPC protocol handling
├── cli/              # CLI command implementations
│   ├── server_commands.go   # server, mcp, status, test, version
│   ├── install_commands.go  # Language server installers
│   └── cache_commands.go    # cache info, clear, index
├── config/           # YAML configuration and auto-detection
├── utils/            # URI conversion utilities
└── internal/
    ├── installer/    # Language-specific installer logic
    ├── project/      # Language detection, package discovery
    ├── models/       # LSP protocol type definitions
    ├── types/        # Shared interfaces and types
    ├── errors/       # Error type definitions
    ├── security/     # Command injection prevention
    ├── version/      # Version extraction from package.json
    ├── constants/    # Timeouts and system limits
    └── common/       # STDIO-safe logging infrastructure
```

### Critical: STDIO-Safe Logging

**MANDATORY**: Use only stderr-based loggers from `src/internal/common/logging.go`:
```go
import "lsp-gateway/src/internal/common"

common.LSPLogger.Info("message")     // LSP/MCP operations
common.GatewayLogger.Error("error")  // HTTP gateway operations
common.CLILogger.Warn("warning")     // CLI commands

// NEVER USE (breaks LSP/MCP protocol):
fmt.Print*, log.Print*, log.New()    // ❌ Writes to stdout
```

### Known Critical Issues

1. **Timeout Mismatch**: `src/server/lsp_manager.go:116-121` wraps client init in 10s timeout but Java needs 60s
2. **Cache Invalidation Missing**: No file change notifications handled (`src/server/lsp_client_manager.go:368-371`)
3. **Monolithic Cache**: `src/server/cache/manager.go` combines too many responsibilities (2000+ lines)

### Key Design Patterns

**Interface-Based Abstraction** (`src/internal/types/interfaces.go:12-37`):
- Clean `LSPClient` interface enables multiple implementations
- `CacheIntegrator` interface for optional cache enhancement
- Graceful degradation when cache unavailable

**Parallel Processing** (`src/server/aggregators/workspace_symbol.go`):
- Concurrent queries across multiple language servers
- Result aggregation with timeout protection
- Channel-based coordination

**Constructor Pattern**: All constructors use `New*` prefix returning `(*Type, error)`

## System Constants

**Timeouts** (`src/internal/constants/constants.go:19-22`):
- Java: 60s initialization
- Python: 30s initialization  
- Go/JS/TS: 15s initialization
- Default operations: 30s

**Cache Limits** (`src/internal/constants/constants.go:29-40`):
- Memory: 512MB default, 2GB max
- TTL: 24h standard, 1h for MCP mode
- Health checks: 5min standard, 2min for MCP
- Index batch: 100 files
- Query limit: 1000 results

**Process Management**:
- Graceful shutdown: 5s
- Directory scan depth: 3 levels
- Skip patterns: vendor/, node_modules/, .git/

## Testing Strategy

### Test Organization (56 test files total)
```
tests/
├── unit/         # 7 files - Component isolation tests (2 min)
├── integration/  # 15 files - Component interaction tests (10 min)
├── e2e/          # 16 files - Full system validation (30 min)
├── shared/       # 6 files - Test utilities and helpers
└── testdata/     # 1 file - Test context management
```

### E2E Test Repositories (`tests/e2e/testutils/repo_manager.go:52-134`)
- **Go**: `gorilla/mux` v1.8.0 - HTTP router
- **Python**: `psf/requests` v2.28.1 - HTTP client
- **JavaScript**: `ramda/ramda` v0.31.3 - Functional library
- **TypeScript**: `sindresorhus/is` v5.4.1 - Type checking
- **Java**: `spring-projects/spring-petclinic` main - Spring Boot app

### Running Tests
```bash
# Quick validation (2 min)
go test -v -short ./tests/unit/...

# Development cycle (8-10 min)
make test-fast

# Full validation (30+ min)
make test

# Specific test
go test -v -run TestDefinition ./tests/e2e/...
```

## Configuration

### MCP Server Setup (for AI assistants)
Add to your AI assistant's configuration:
```json
{
  "mcpServers": {
    "lsp-gateway": {
      "command": "lsp-gateway",
      "args": ["mcp"]
    }
  }
}
```

### Auto-Detection
Scans project root for: `go.mod`, `package.json`, `*.py`, `pom.xml`, `build.gradle`

### Config File: `~/.lsp-gateway/config.yaml`
```yaml
cache:
  enabled: true
  max_memory_mb: 512
  ttl_hours: 24          # MCP mode overrides to 1hr
servers:
  go:
    command: "gopls"
    args: ["serve"]
  python:
    command: "pylsp"
```

## Common Development Tasks

### Debug LSP Communication
```bash
export LSP_GATEWAY_DEBUG=true
lsp-gateway server

# Test textDocument/definition
curl -X POST localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"textDocument/definition","params":{"textDocument":{"uri":"file:///path/to/file.go"},"position":{"line":10,"character":5}}}'

# Test workspace/symbol
curl -X POST localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"workspace/symbol","params":{"query":"Router"}}'
```

### Add New LSP Method
1. Define types in `src/internal/models/`
2. Update capability detection in `src/server/capabilities/detector.go`
3. Add error translation in `src/server/errors/translator.go`
4. Implement in `src/server/lsp_manager.go`
5. Update MCP tools in `src/server/mcp_tools.go` if applicable
6. Add tests in `tests/e2e/`

## Important Constraints

- **5 languages only**: Go, Python, JavaScript, TypeScript, Java
- **6 LSP methods**: definition, references, hover, documentSymbol, workspace/symbol, completion
- **2 MCP tools**: findSymbols, findReferences (SCIP-enhanced)
- **Local operation**: No enterprise features (auth, monitoring, distributed)
- **Dynamic typing**: JSON-RPC uses `interface{}` for flexibility

## Troubleshooting

| Issue | Solution |
|-------|----------|
| STDIO pollution | Use `common.LSPLogger` from `src/internal/common/logging.go` |
| Cache nil panic | Always check `if m.scipCache != nil` before use |
| LSP not found | Run `lsp-gateway install all` first |
| Port 8080 busy | Use `lsp-gateway server --port 8081` |
| Slow E2E tests | Normal - clones real repos (30s each) |
| Java timeout | Known issue - `lsp_manager.go:116` has 10s limit vs 60s requirement |