# CLAUDE.md

LSP Gateway development reference - v0.0.20

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
lsp-gateway cache status         # Show cache configuration and status
lsp-gateway cache clear          # Clear SCIP cache
lsp-gateway cache stats          # Performance statistics
lsp-gateway cache health         # Cache health diagnostics

# Quality & Testing
make quality                     # Format + vet (essential checks)
make quality-full                # Format + vet + lint + security
make test-fast                   # Unit + integration (8-10 min)
make test                        # All tests including E2E (30+ min)
go test -v ./tests/unit/...      # Unit tests only (2 min)
go test -v -run TestName ./...   # Run specific test
make test-quick                  # Quick validation tests
```

## Architecture Overview

### Core Components
```
src/
├── server/                      # Core server implementations
│   ├── lsp_manager.go           # Central LSP orchestration
│   ├── gateway.go               # HTTP JSON-RPC gateway
│   ├── mcp_server.go            # MCP STDIO server
│   ├── lsp_client_manager.go    # Language client lifecycle
│   ├── cache/                   # SCIP cache (512MB LRU)
│   ├── capabilities/            # Dynamic LSP capability detection
│   ├── aggregators/             # Parallel workspace operations
│   └── process/                 # LSP server process management
├── cli/                         # Command implementations
├── internal/                    # Shared components
│   ├── registry/languages.go    # Language configurations
│   ├── security/                # Command injection prevention
│   └── common/logging.go        # STDIO-safe logging
└── tests/                       # Test suites
```

### Key Components
- **LSP Manager** (`src/server/lsp_manager.go`): Central orchestration for all language servers
- **Gateway** (`src/server/gateway.go`): HTTP JSON-RPC server on port 8080
- **MCP Server** (`src/server/mcp_server.go`): STDIO protocol for AI assistants
- **SCIP Cache** (`src/server/cache/`): 512MB LRU cache for fast symbol lookups
- **Parallel Aggregator** (`src/server/aggregators/`): Concurrent multi-language operations

## Language Support

8 languages with auto-detection:

| Language | Extensions | LSP Server | Timeout | Detection |
|----------|------------|------------|---------|-----------|
| Go | .go | gopls | 15s | go.mod |
| Python | .py, .pyi | basedpyright (default) | 30s/45s init | *.py |
| JavaScript | .js, .jsx, .mjs | typescript-language-server | 15s/30s init | package.json |
| TypeScript | .ts, .tsx, .d.ts | typescript-language-server | 15s/30s init | package.json |
| Java | .java | jdtls | 90s | pom.xml, build.gradle |
| Rust | .rs | rust-analyzer | 15s | Cargo.toml |
| C# | .cs | omnisharp | 30s/45s init | *.csproj, *.sln |
| Kotlin | .kt, .kts | kotlin-lsp (JetBrains/fwcd) | 15s | build.gradle.kts, *.kt |

### Python LSP Server Options

Default: **basedpyright** (enhanced pyright fork via pip)

Alternatives available:
```bash
lsp-gateway install python --server pyright   # Microsoft's pyright via npm
lsp-gateway install python --server jedi      # Jedi-language-server via pip
```

### Kotlin LSP Server

Platform-specific selection:
- macOS/Linux: JetBrains kotlin-lsp (official)
- Windows: fwcd/kotlin-language-server (community)

## Critical: STDIO-Safe Logging

**MANDATORY for LSP/MCP protocols** - Use only stderr-based loggers:
```go
import "lsp-gateway/src/internal/common"

common.LSPLogger.Info("message")     // LSP/MCP operations
common.GatewayLogger.Error("error")  // HTTP gateway
common.CLILogger.Warn("warning")     // CLI commands

// NEVER USE: fmt.Print*, log.Print*, log.New() - breaks protocol
```

## Security Model

Whitelist-based command validation (`src/internal/security/command_validation.go`):
- Only pre-approved LSP commands in registry
- Blocks shell metacharacters (`|`, `&`, `;`, backticks, `$()`)
- Rejects path traversal (`..`)

## Development Patterns

### Parallel Processing
```go
aggregator := NewParallelAggregator[RequestType, ResponseType](
    individualTimeout,  // Per-language timeout
    overallTimeout,     // Total operation timeout
)
results, errors := aggregator.Execute(ctx, clients, request, executor)
```

### Cache Integration
```go
// Always check availability before use
if m.scipCache != nil {
    // Cache operations
}
// Graceful degradation when unavailable
```

## LSP Methods & MCP Tools

**LSP Methods**: definition, references, hover, documentSymbol, completion, workspace/symbol

**MCP Tools** (SCIP-enhanced):
- `findSymbols` - Regex pattern search across workspace
- `findReferences` - Find all references with context

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

## Platform-Specific Tuning

### Timeouts
- Java: 90s (all operations)
- Python: 30s standard, 45s initialization
- Go/TypeScript: 15s init, 30s operations
- CI multipliers: Windows 3x (Java), 1.5x (others)

### Cache Configuration
- Memory: 512MB LRU
- TTL: 24h (1h MCP mode)
- Query limit: 100 (5000 MCP symbols)

## Testing Strategy

### Test Execution
```bash
make test-fast      # Dev cycle (8-10 min)
make test           # Full suite (30+ min)
make cache-test     # Cache-specific tests
make test-quick     # Quick validation tests
```

E2E tests use real repositories for validation.

## Common Development Tasks

### Adding New LSP Method
1. Define types in `src/internal/models/lsp/protocol.go`
2. Update capability detection in `src/server/capabilities/detector.go`
3. Implement in `src/server/lsp_manager.go::ProcessRequest`
4. Add MCP tool handler in `src/server/mcp_tools.go` if needed
5. Add tests in `tests/e2e/`

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

## Module & Import Paths
- Go module: `lsp-gateway` (not github.com/...)
- Import: `lsp-gateway/src/...`

## Development Philosophy
1. **Fail-Safe**: Graceful degradation on partial failures
2. **Language-Aware**: Different timeouts per language
3. **Cache-Optional**: Full functionality without cache
4. **Security-First**: Whitelist-based validation