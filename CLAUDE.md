# CLAUDE.md

LSP Gateway development reference for Claude Code - v0.0.10

## Core Commands

```bash
# Primary workflow
make local                     # Build + npm link (main development command)
lsp-gateway install all        # Install all 6 language servers

# Run servers
lsp-gateway server             # HTTP Gateway (port 8080/jsonrpc)
lsp-gateway server --port 8081 # Alternative port
lsp-gateway mcp                # MCP Server (STDIO protocol)
lsp-gateway status             # Check LSP server availability

# Quality & Testing
make quality                   # Format + vet (essential checks)
make quality-full              # Format + vet + lint + security
make test-fast                 # Unit + integration (8-10 min)
make test                      # All tests including E2E (30+ min)

# Specific tests
go test -v ./tests/unit/...         # Unit tests (2 min)
go test -v ./tests/integration/...  # Integration tests (10 min)
go test -v -run TestName ./tests/... # Run specific test
```

## Architecture

### Core Components
- **LSP Manager** (`src/server/lsp_manager.go`): Central orchestrator with SCIP cache integration
- **HTTP Gateway** (`src/server/gateway.go`): JSON-RPC endpoint at `:8080/jsonrpc`
- **MCP Server** (`src/server/mcp_server.go`): STDIO protocol for AI assistants
- **SCIP Cache** (`src/server/cache/manager.go`): 512MB LRU cache, sub-millisecond lookups

### Directory Structure
```
src/
├── cmd/lsp-gateway/         # CLI entry (main.go)
├── server/                  # Core server implementations
│   ├── lsp_manager.go       # Central LSP orchestration
│   ├── gateway.go           # HTTP JSON-RPC gateway
│   ├── mcp_server.go        # MCP server (STDIO)
│   ├── mcp_tools.go         # MCP tool handlers
│   ├── lsp_client_manager.go# Language client management
│   ├── cache/               # SCIP cache (manager.go - 2000+ lines)
│   ├── scip/                # SCIP protocol implementation
│   ├── process/             # LSP server lifecycle
│   ├── capabilities/        # Dynamic LSP capability detection
│   ├── aggregators/         # Parallel workspace symbol aggregation
│   └── watcher/             # File change monitoring
├── cli/                     # CLI commands
│   ├── server_commands.go   # server, mcp, status, test
│   ├── install_commands.go  # Language server installers
│   └── cache_commands.go    # cache management
├── internal/
│   ├── installer/           # Language-specific installers
│   ├── project/             # Language detection
│   ├── models/lsp/          # LSP protocol types
│   ├── types/               # Shared interfaces
│   ├── constants/           # System timeouts and limits
│   ├── common/              # STDIO-safe logging
│   └── security/            # Command injection prevention
├── config/                  # YAML configuration
└── tests/                   # Test suites
```

### Critical: STDIO-Safe Logging

**MANDATORY**: Use only stderr-based loggers from `src/internal/common/logging.go`:
```go
import "lsp-gateway/src/internal/common"

common.LSPLogger.Info("message")     // LSP/MCP operations
common.GatewayLogger.Error("error")  // HTTP gateway
common.CLILogger.Warn("warning")     // CLI commands

// NEVER USE (breaks LSP/MCP protocol):
fmt.Print*, log.Print*, log.New()    // ❌ Writes to stdout
```

## System Constants (`src/internal/constants/constants.go`)

### Timeouts
- **Java**: 90s initialization/operations
- **Python**: 30s initialization/operations  
- **Go/TS/JS**: 15s initialization, 30s operations
- **Process shutdown**: 5s graceful timeout

### Cache Limits
- **Memory**: 512MB default (MCP mode)
- **TTL**: 24h standard, 1h for MCP mode
- **Health checks**: 5min standard, 2min for MCP
- **Query limit**: 100 default, 5000 for MCP symbols

### File Scanning
- **Max depth**: 3 directory levels
- **Skip patterns**: vendor/, node_modules/, .git/, build/, dist/, target/, __pycache__

## Development Patterns

### Interface Abstractions
```go
// Core interfaces in src/internal/types/
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

### Error Handling
- Always check cache availability: `if m.scipCache != nil`
- Use context for timeout control
- Graceful degradation when cache unavailable

### Constructor Pattern
All constructors use `New*` prefix returning `(*Type, error)`:
```go
func NewLSPManager(cfg *config.Config) (*LSPManager, error)
func NewMCPServer(cfg *config.Config) (*MCPServer, error)
func NewGateway(manager *LSPManager, port int) (*Gateway, error)
```

## LSP Method Support

**Implemented Methods**:
- `textDocument/definition` - Find symbol definition
- `textDocument/references` - Find all references
- `textDocument/hover` - Show hover info
- `textDocument/documentSymbol` - Document symbols
- `textDocument/completion` - Code completion
- `workspace/symbol` - Search workspace symbols

**MCP Tools** (SCIP-enhanced):
- `findSymbols` - Regex pattern search across workspace
- `findReferences` - Find all references with context

## Configuration

### Auto-detection
Scans for: `go.mod`, `package.json`, `*.py`, `pom.xml`, `build.gradle`, `Cargo.toml`

### Config File (`~/.lsp-gateway/config.yaml`)
```yaml
cache:
  enabled: true
  max_memory_mb: 512
  ttl_hours: 24         # MCP overrides to 1hr
servers:
  go:
    command: "gopls"
    args: ["serve"]
  python:
    command: "pylsp"
  rust:
    command: "rust-analyzer"
```

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

## Testing Strategy

### E2E Tests Clone Real Repos
- **Go**: gorilla/mux
- **Python**: psf/requests
- **JavaScript**: ramda/ramda
- **TypeScript**: sindresorhus/is
- **Java**: spring-petclinic
- **Rust**: tokio-rs/tokio

### Test Execution
```bash
make test-fast      # Development cycle (8-10 min)
make test           # Full suite (30+ min)
```

## Common Tasks

### Adding New LSP Method
1. Define types in `src/internal/models/lsp/`
2. Update `src/server/capabilities/detector.go`
3. Implement in `src/server/lsp_manager.go::ProcessRequest`
4. Add MCP tool handler in `src/server/mcp_tools.go` if needed
5. Add tests in `tests/e2e/`

### Testing LSP Methods
```bash
# Test definition request
curl -X POST localhost:8080/jsonrpc -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"textDocument/definition","params":{"textDocument":{"uri":"file:///path/to/file.go"},"position":{"line":10,"character":5}}}'

# Search workspace symbols
curl -X POST localhost:8080/jsonrpc -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"workspace/symbol","params":{"query":"Router"}}'
```

### Debug Mode
```bash
export LSP_GATEWAY_DEBUG=true
lsp-gateway server
```

## Known Issues & Solutions

| Issue | Solution |
|-------|----------|
| STDIO pollution | Use `common.LSPLogger` from `src/internal/common/logging.go` |
| Cache nil panic | Always check `if m.scipCache != nil` before use |
| LSP not found | Run `lsp-gateway install all` |
| Port 8080 busy | Use `lsp-gateway server --port 8081` |
| Java slow init | Expected - Java has 90s timeout |
| Cache module size | `src/server/cache/manager.go` (2000+ lines) - refactoring candidate |

## Performance Characteristics

- **Parallel Processing**: Concurrent queries across language servers
- **SCIP Indexing**: Automatic background indexing for fast lookups
- **Response Buffer**: 1MB buffer for large workspace/symbol results
- **Graceful Degradation**: Continues without cache if unavailable
- **File Watch Debounce**: 500ms delay for file change events

## Development Philosophy

1. **Dynamic Typing**: JSON-RPC uses `interface{}` for flexibility
2. **Interface-First**: Clean separation through interfaces
3. **Fail-Safe**: System continues with degraded functionality
4. **Language-Aware**: Different timeouts and behaviors per language
5. **Cache-Optional**: Full functionality without cache, enhanced with it