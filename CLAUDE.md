# CLAUDE.md

LSP Gateway development reference for Claude Code - v0.0.17

## Core Commands

```bash
# Build & Install
make local                       # Primary dev: build + npm link (includes cache)
lsp-gateway install all          # Install all 7 language servers

# Run Servers
lsp-gateway server               # HTTP Gateway on :8080/jsonrpc
lsp-gateway server --port 8081   # Alternative port
lsp-gateway mcp                  # MCP Server (STDIO for AI assistants)

# Status & Testing
lsp-gateway status               # Check LSP server availability
lsp-gateway test                 # Test server connections
lsp-gateway version              # Show version info

# Cache Management
lsp-gateway cache info           # Show cache statistics
lsp-gateway cache clear          # Clear SCIP cache
lsp-gateway cache index          # Index workspace for fast lookups

# Quality & Testing
make quality                     # Format + vet (essential checks)
make quality-full                # Format + vet + lint + security
make test-fast                   # Unit + integration (8-10 min)
make test                        # All tests including E2E (30+ min)
go test -v ./tests/unit/...      # Unit tests only (2 min)
go test -v -run TestName ./...   # Run specific test
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

### Key Interfaces
```go
// Core abstractions in src/internal/types/interfaces.go
type LSPClient interface {
    Start(ctx context.Context) error
    Stop() error
    SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error)
    SendNotification(ctx context.Context, method string, params interface{}) error
    Supports(method string) bool
    IsActive() bool
}

type CacheIntegrator interface {
    IndexWorkspace(ctx context.Context, rootPath string, language string, filePatterns []string) error
    FindDefinitions(ctx context.Context, uri string, position Position) ([]*Location, error)
    FindReferences(ctx context.Context, uri string, position Position) ([]*Location, error)
    SearchSymbols(ctx context.Context, query string) ([]*WorkspaceSymbol, error)
}
```

## Language Support

7 languages with auto-detection and specific configurations:

| Language | Extensions | LSP Server | Timeout | Detection |
|----------|------------|------------|---------|-----------|
| Go | .go | gopls | 15s | go.mod |
| Python | .py, .pyi | jedi-language-server | 30s | *.py |
| JavaScript | .js, .jsx, .mjs | typescript-language-server | 15s/30s init | package.json |
| TypeScript | .ts, .tsx, .d.ts | typescript-language-server | 15s/30s init | package.json |
| Java | .java | jdtls | 90s | pom.xml, build.gradle |
| Rust | .rs | rust-analyzer | 15s | Cargo.toml |
| C# | .cs | omnisharp | 30s/45s init | *.csproj, *.sln |

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
- Only pre-approved LSP commands in `src/internal/registry/languages.go:186-247`
- Blocks shell metacharacters (`|`, `&`, `;`, backticks, `$()`)
- Rejects path traversal (`..`)

## Development Patterns

### Constructor Pattern
```go
func NewLSPManager(cfg *config.Config) (*LSPManager, error)
func NewMCPServer(cfg *config.Config) (*MCPServer, error)
func NewGateway(manager *LSPManager, port int) (*Gateway, error)
```

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

### Dynamic Capability Detection
```go
// Server-specific workarounds in src/server/capabilities/detector.go
if client.Supports(types.MethodWorkspaceSymbol) {
    // Safe to call workspace/symbol
}
// Special handling: jdtls (Java), OmniSharp (C#)
```

## LSP Methods & MCP Tools

**Implemented LSP Methods**:
- textDocument/definition
- textDocument/references
- textDocument/hover
- textDocument/documentSymbol
- textDocument/completion
- workspace/symbol

**MCP Tools** (SCIP-enhanced):
- `findSymbols` - Regex pattern search
- `findReferences` - Find all references with context

## Configuration

### Auto-detection Order
1. go.mod → Go
2. package.json → TypeScript/JavaScript
3. *.py → Python
4. pom.xml/build.gradle → Java
5. Cargo.toml → Rust
6. *.csproj/*.sln → C#

### Config File (`~/.lsp-gateway/config.yaml`)
```yaml
cache:
  enabled: true
  max_memory_mb: 512
  ttl_hours: 24         # MCP mode: 1hr
servers:
  go:
    command: "gopls"
    args: ["serve"]
  python:
    command: "jedi-language-server"
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

### Timeouts (`src/internal/constants/`)
- Java: 90s (all operations)
- Python: 30s
- Go/TS: 15s init, 30s operations
- CI multipliers: Windows 3x (Java), 1.5x (others)

### Background Indexing Delays
- Windows: 12-20s (CI=20s)
- Unix: 3s standard
- Applied after didOpen notifications

### Cache Limits
- Memory: 512MB LRU
- TTL: 24h (1h MCP mode)
- Query limit: 100 (5000 MCP symbols)
- Project isolation: MD5-based paths

## Testing Strategy

### E2E Tests Clone Real Repos
- Go: gorilla/mux
- Python: psf/requests
- JavaScript: ramda/ramda
- TypeScript: sindresorhus/is
- Java: spring-petclinic
- Rust: tokio-rs/tokio
- C#: dotnet/samples

### Test Execution
```bash
make test-fast      # Dev cycle (8-10 min)
make test           # Full suite (30+ min)
make cache-test     # Cache-specific tests
```

## Common Development Tasks

### Adding New LSP Method
1. Define types in `src/internal/models/lsp/protocol.go`
2. Update capability detection in `src/server/capabilities/detector.go`
3. Implement in `src/server/lsp_manager.go::ProcessRequest` (~line 300+)
4. Add MCP tool handler in `src/server/mcp_tools.go` if needed
5. Register in `src/internal/registry/languages.go` if language-specific
6. Add tests in `tests/e2e/`

### Testing LSP Methods
```bash
# Definition request
curl -X POST localhost:8080/jsonrpc -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"textDocument/definition","params":{"textDocument":{"uri":"file:///path/to/file.go"},"position":{"line":10,"character":5}}}'

# Workspace symbols
curl -X POST localhost:8080/jsonrpc -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"workspace/symbol","params":{"query":"Router"}}'
```

### Debug Mode
```bash
export LSP_GATEWAY_DEBUG=true
lsp-gateway server
```

## Build System

### Multi-platform Support
- Automatic binary naming: `lsp-gateway-{platform}[.exe]`
- npm wrapper: `bin/lsp-gateway.js` with fallback
- Build tags: `cache_enabled` for SCIP cache
- Version injection: LDFlags at build time

### Platform Builds
```bash
make linux          # Linux amd64
make macos          # Darwin amd64
make macos-arm64    # Darwin arm64 (M1/M2)
make windows        # Windows amd64
```

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
1. **Interface-First**: Clean separation through interfaces
2. **Fail-Safe**: Graceful degradation on partial failures
3. **Language-Aware**: Different timeouts per language
4. **Cache-Optional**: Full functionality without cache
5. **Security-First**: Whitelist-based validation
6. **Dynamic Typing**: JSON-RPC uses `interface{}` for flexibility