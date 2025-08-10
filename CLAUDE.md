# CLAUDE.md

LSP Gateway development reference for Claude Code.

## Getting Started

```bash
# Prerequisites: Go 1.24.0+, Node.js 18+
git clone https://github.com/IDontHaveBrain/lsp-gateway
cd lsp-gateway
make local                    # Build + npm link globally
lsp-gateway install all       # Install all 6 language servers (Go, Python, JS, TS, Java, Rust)
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
make help                    # Show all available make targets

# Test commands
go test -v ./tests/unit/...         # Unit tests only (2 min)
go test -v ./tests/integration/...  # Integration tests (10 min)
go test -v ./tests/e2e/...          # E2E tests (30 min)
go test -v -run TestName ./tests/...  # Run specific test
```

## Project Overview

LSP Gateway bridges Language Server Protocol servers through dual protocols:
- **HTTP Gateway**: `localhost:8080/jsonrpc` - JSON-RPC interface
- **MCP Server**: STDIO protocol for AI assistants with SCIP cache (512MB, 1hr TTL)
- **Languages**: Go, Python, JavaScript, TypeScript, Java, Rust
- **LSP Methods**: definition, references, hover, documentSymbol, workspace/symbol, completion
- **MCP Tools**: findSymbols, findReferences (SCIP-enhanced)

## Build & Development

```bash
# Primary commands
make local          # Build + npm link (main workflow)
make clean          # Clean artifacts
make quality        # Format + vet
make test-fast      # Unit + integration tests (8-10 min)
make test           # Full test suite (30+ min)

# LSP server installation
lsp-gateway install all              # Install all language servers
lsp-gateway install [language]       # Install specific: go, python, typescript, javascript, java, rust

# Runtime
lsp-gateway server                   # HTTP Gateway on :8080
lsp-gateway server --port 8081       # Alternative port
lsp-gateway mcp                      # MCP Server (STDIO)
lsp-gateway status                   # Check LSP availability
lsp-gateway cache index              # Index workspace for faster lookups
```

## Development Workflow

1. **Build**: `make local` - primary workflow for development
2. **Test**: `make test-fast` during development, `make test` before commits
3. **Quality**: `make quality` before pushing changes
4. **Debug**: Set `LSP_GATEWAY_DEBUG=true` for verbose logging

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
    ├── installer/    # Language-specific installer logic (including rust_installer.go)
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

### Known Issues

1. **Cache Invalidation**: File change notifications not fully implemented
2. **Monolithic Cache Module**: `src/server/cache/manager.go` (2000+ lines) - refactoring candidate

### Key Design Patterns

- **Interface abstraction**: `LSPClient` and `CacheIntegrator` for clean separation
- **Parallel processing**: Concurrent queries across language servers with timeout protection
- **SCIP indexing**: Automatic symbol indexing for sub-millisecond lookups
- **Graceful degradation**: System continues without cache if unavailable

## System Constants

**Timeouts** (`src/internal/constants/constants.go`):
- Java: 90s initialization/operations
- Python: 30s initialization/operations
- Go/JS/TS/Rust: 15s initialization/operations

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

## Testing

```bash
make test-fast      # Unit + integration (8-10 min) - primary development cycle
make test           # Full suite including E2E (30+ min)
go test -v -run TestName ./tests/...  # Run specific test
```

E2E tests clone real repositories: gorilla/mux (Go), psf/requests (Python), ramda/ramda (JS), sindresorhus/is (TS), spring-petclinic (Java), tokio-rs/tokio (Rust)

## Configuration

### MCP Setup
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

**Auto-detection**: Scans for `go.mod`, `package.json`, `*.py`, `pom.xml`, `build.gradle`, `Cargo.toml`
**Config**: `~/.lsp-gateway/config.yaml` (optional - has sensible defaults)

## Common Tasks

### Testing LSP Methods
```bash
curl -X POST localhost:8080/jsonrpc -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"textDocument/definition","params":{"textDocument":{"uri":"file:///path/to/file.go"},"position":{"line":10,"character":5}}}'
```

### Adding New LSP Method
1. Define types in `src/internal/models/`
2. Update `src/server/capabilities/detector.go`
3. Implement in `src/server/lsp_manager.go`
4. Add tests in `tests/e2e/`

## Development Constraints

- **STDIO-safe logging mandatory**: Never use fmt.Print*/log.Print* - breaks LSP/MCP protocol
- **Dynamic typing**: JSON-RPC uses `interface{}` for flexibility
- **Constructor pattern**: All use `New*` prefix returning `(*Type, error)`
- **Cache checks**: Always verify `if m.scipCache != nil` before use

## Troubleshooting

| Issue | Solution |
|-------|----------|
| STDIO pollution | Use `common.LSPLogger` from `src/internal/common/logging.go` |
| Cache nil panic | Always check `if m.scipCache != nil` before use |
| LSP not found | Run `lsp-gateway install all` first |
| Port 8080 busy | Use `lsp-gateway server --port 8081` |
| Slow E2E tests | Expected - clones real repos |
| Java slow init | Java has 90s timeout for initialization |