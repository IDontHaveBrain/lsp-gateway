# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

LSP Gateway development reference - v0.0.21

## Core Commands

```bash
# Build & Install
make local                       # Primary dev: build + npm link (includes cache)
lsp-gateway install all          # Install all 8 language servers

# Run Servers
lsp-gateway server               # HTTP Gateway on :8080/jsonrpc
lsp-gateway server --port 8081   # Alternative port
lsp-gateway mcp                  # MCP Server (STDIO for AI assistants)

# Status & Testing
lsp-gateway status               # Check LSP server availability
lsp-gateway test                 # Test server connections
lsp-gateway version              # Show version info

# Cache Management
lsp-gateway cache info           # Show cache statistics and configuration
lsp-gateway cache clear          # Clear SCIP cache
lsp-gateway cache index          # Index workspace for faster lookups

# Quality & Testing
make quality                     # Format + vet (essential checks, no external deps)
make quality-full                # Format + vet + lint + security
make test-fast                   # Unit + integration (8-10 min, recommended for dev)
make test                        # All tests including E2E (30+ min)
make test-unit                   # Unit tests only (2 min)
make test-integration            # Integration tests only
make test-e2e                    # E2E tests only
go test -v -run TestName ./...   # Run specific test
make cache-test                  # Cache-specific tests (5 min)
make help                        # Show all available targets
```

## Architecture Overview

### Core Components
```
src/
├── server/                      # Core server implementations
│   ├── lsp_manager.go           # Central LSP orchestration (Manager of Managers pattern)
│   ├── gateway.go               # HTTP JSON-RPC gateway
│   ├── mcp_server.go            # MCP STDIO server
│   ├── lsp_client_manager.go    # Language client lifecycle
│   ├── cache/                   # SCIP cache (512MB LRU, 30+ implementation files)
│   ├── capabilities/            # Dynamic LSP capability detection
│   ├── aggregators/             # Parallel workspace operations
│   ├── process/                 # LSP server process management
│   ├── documents/               # Document lifecycle management
│   ├── watcher/                 # Real-time file change detection
│   ├── errors/                  # Unified error translation
│   ├── scip/                    # SCIP protocol support
│   └── protocol/                # JSON-RPC protocol layer
├── cli/                         # Command implementations
├── internal/                    # Shared components
│   ├── registry/languages.go    # Language configurations
│   ├── security/                # Command injection prevention
│   └── common/logging.go        # STDIO-safe logging
└── tests/                       # Test suites (unit, integration, e2e)
```

## Critical Architectural Patterns

### 1. Graceful Degradation (MOST IMPORTANT)
**Everything fails safely** - the system continues functioning despite partial failures:
```go
// Always check availability before use
if m.scipCache != nil {
    // Cache operations
}
// Continue with LSP processing even if cache unavailable
```

### 2. Parallel Aggregation Framework
Language-aware concurrent processing with sophisticated timeout management:
```go
aggregator := NewParallelAggregator[RequestType, ResponseType](
    individualTimeout,  // Per-language timeout
    overallTimeout,     // Total operation timeout
)
results, errors := aggregator.Execute(ctx, clients, request, executor)
```

### 3. STDIO-Safe Logging (MANDATORY)
**Critical for LSP/MCP protocols** - Use only stderr-based loggers:
```go
import "lsp-gateway/src/internal/common"

common.LSPLogger.Info("message")     // LSP/MCP operations
common.GatewayLogger.Error("error")  // HTTP gateway
common.CLILogger.Warn("warning")     // CLI commands

// NEVER USE: fmt.Print*, log.Print*, log.New() - breaks protocol
```

### 4. Security-First Validation
Whitelist-based command validation at every execution point:
```go
if err := security.ValidateCommand(command, args); err != nil {
    return fmt.Errorf("security validation failed: %w", err)
}
```
- Only pre-approved LSP commands in registry
- Blocks shell metacharacters (`|`, `&`, `;`, backticks, `$()`)
- Rejects path traversal (`..`)

## Language Support

8 languages with auto-detection and language-specific timeouts:

| Language | Extensions | LSP Server | Timeout | Detection |
|----------|------------|------------|---------|-----------|
| Go | .go | gopls | 15s | go.mod |
| Python | .py, .pyi | basedpyright (default) | 30s/45s init | *.py |
| JavaScript | .js, .jsx, .mjs | typescript-language-server | 15s/30s init | package.json |
| TypeScript | .ts, .tsx, .d.ts | typescript-language-server | 15s/30s init | package.json |
| Java | .java | jdtls | 90s | pom.xml, build.gradle |
| Rust | .rs | rust-analyzer | 15s | Cargo.toml |
| C# | .cs | omnisharp | 30s/45s init | *.csproj, *.sln |
| Kotlin | .kt, .kts | kotlin-lsp (JetBrains/fwcd) | 30s/150s init | build.gradle.kts, *.kt |

Python alternatives:
```bash
lsp-gateway install python --server pyright   # Microsoft's pyright via npm
lsp-gateway install python --server jedi      # Jedi-language-server via pip
```

## Development Patterns

### Cache Integration Pattern
```go
// Cache is always optional - never assume availability
if m.scipCache != nil && m.isCacheableMethod(method) {
    if result, found, err := m.scipCache.Lookup(method, params); err == nil && found {
        return result, nil
    }
}
// Graceful degradation to direct LSP
```

### Error Handling Philosophy
- **Fail-safe defaults**: Unknown capabilities assumed supported
- **Progressive enhancement**: Core functionality works, cache/advanced features enhance
- **Error isolation**: Component failures contained, don't cascade
- **Comprehensive fallbacks**: Multiple strategies for every operation

## Testing Strategy

### Development Workflow
```bash
make local          # Build + npm link (primary dev command)
make test-fast      # Unit + integration (8-10 min, use during dev)
make quality        # Essential checks (fast, no external deps)
make deps           # Download Go dependencies
```

### Pre-commit Validation
```bash
make quality-full   # Complete quality suite including linting
make test-fast      # Comprehensive but time-efficient testing
make cache-test     # Verify cache functionality
```

### Full Validation (CI-equivalent)
```bash
make test           # All tests including E2E (30+ min)
make quality-full   # Complete quality analysis
```

## Common Development Tasks

### Adding New LSP Method
1. Define types in `src/internal/models/lsp/protocol.go`
2. Update capability detection in `src/server/capabilities/detector.go`
3. Implement in `src/server/lsp_manager.go::ProcessRequest`
4. Add MCP tool handler in `src/server/mcp_tools.go` if needed
5. Add tests in `tests/e2e/`

### Additional CLI Commands
```bash
lsp-gateway context map <file>       # Print referenced files code
lsp-gateway context symbols [files]  # Extract symbol definitions
```

### Testing LSP Methods
```bash
# Definition request
curl -X POST localhost:8080/jsonrpc -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"textDocument/definition","params":{"textDocument":{"uri":"file:///path/to/file.go"},"position":{"line":10,"character":5}}}'

# Check supported languages
curl localhost:8080/languages
```

### Debug Mode
```bash
export LSP_GATEWAY_DEBUG=true
lsp-gateway server
```

## Configuration

### Config File (`~/.lsp-gateway/config.yaml`)
```yaml
cache:
  enabled: true
  max_memory_mb: 512
  ttl_hours: 24         # MCP mode: 1hr
servers:
  python:
    command: "basedpyright-langserver"  # Default
    args: ["--stdio"]
```

### MCP Integration
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

## Build System

### Platform Builds
```bash
make linux          # Linux amd64
make macos          # Darwin amd64
make macos-arm64    # Darwin arm64 (M1/M2)
make windows        # Windows amd64
```

- Automatic binary naming: `lsp-gateway-{platform}[.exe]`
- npm wrapper: `bin/lsp-gateway.js` with fallback
- Build tags: `cache_enabled` for SCIP cache

## Troubleshooting

| Issue | Solution |
|-------|----------|
| STDIO pollution | Use `common.LSPLogger` from logging.go |
| Cache nil panic | Check `if m.scipCache != nil` |
| LSP not found | Run `lsp-gateway install all` |
| Port 8080 busy | Use `--port 8081` |
| Java slow init | Expected - 90s timeout |
| Build cache issues | `make cache-clean && make local` |
| View all commands | `make help` |

## Module & Import Paths
- Go module: `lsp-gateway` (not github.com/...)
- Import: `lsp-gateway/src/...`

## Critical Development Guidelines

### Must-Follow Patterns:
1. **Always check for nil**: `if component != nil { /* use component */ }`
2. **Use STDIO-safe logging**: Import `common` package loggers only
3. **Validate all commands**: Use `security.ValidateCommand()` before execution
4. **Handle partial failures**: Design for graceful degradation
5. **Use parallel aggregation**: For multi-language operations
6. **Cache is optional**: Never assume cache availability

### Development Philosophy
1. **Fail-Safe**: Graceful degradation on partial failures
2. **Language-Aware**: Different timeouts per language
3. **Cache-Optional**: Full functionality without cache
4. **Security-First**: Whitelist-based validation
5. **Performance-Conscious**: Parallel processing with language-specific optimizations