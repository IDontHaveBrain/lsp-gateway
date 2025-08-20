# LSP Gateway

Language Server Protocol gateway with HTTP and MCP interfaces for 7 languages.

## Quick Start

```bash
# Prerequisites: Go 1.24.0+, Node.js 18+
git clone https://github.com/IDontHaveBrain/lsp-gateway
cd lsp-gateway
make local                   # Build + npm link globally
lsp-gateway install all      # Install all language servers
lsp-gateway server           # Start HTTP Gateway on :8080
```

Verify installation:
```bash
lsp-gateway status           # Check installed servers
curl localhost:8080/jsonrpc  # Test HTTP gateway
```

## Features

- **7 Languages**: Go, Python, JavaScript, TypeScript, Java, Rust, C#
- **Dual Protocols**: HTTP Gateway (:8080) + MCP Server (STDIO)
- **Auto-detection**: Scans for go.mod, package.json, *.py, pom.xml, Cargo.toml, *.csproj
- **SCIP Cache**: Sub-millisecond symbol lookups with 512MB LRU cache
- **LSP Methods**: definition, references, hover, documentSymbol, workspace/symbol, completion

## Installation

### Language Servers

```bash
lsp-gateway install all        # Install all supported servers
lsp-gateway install go         # Install gopls
lsp-gateway install python     # Install jedi-language-server
lsp-gateway install typescript # Install typescript-language-server
lsp-gateway install javascript # Install typescript-language-server
lsp-gateway install java       # Install jdtls
lsp-gateway install rust       # Install rust-analyzer
lsp-gateway install csharp     # Install omnisharp
```

## Usage

### Commands

```bash
# Servers
lsp-gateway server          # HTTP Gateway at localhost:8080/jsonrpc
lsp-gateway server --port 8081  # Alternative port
lsp-gateway mcp             # MCP Server for AI assistants
lsp-gateway status          # Check LSP server availability
lsp-gateway test            # Test server connections
lsp-gateway version         # Show version info

# Cache Management
lsp-gateway cache info      # Show cache statistics
lsp-gateway cache clear     # Clear all cache
lsp-gateway cache index     # Index workspace for faster lookups
```

### HTTP Gateway

```bash
# Find symbol definition
curl -X POST localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"textDocument/definition","params":{"textDocument":{"uri":"file:///path/to/file.go"},"position":{"line":10,"character":5}}}'

# Search workspace symbols
curl -X POST localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"workspace/symbol","params":{"query":"Router"}}'
```

Additional endpoints:

- GET `/languages`: Supported language names and extensions

```bash
curl -s localhost:8080/languages | jq
# {
#   "languages": ["go","python","javascript","typescript","java","rust","csharp"],
#   "extensions": {
#     "go": [".go"],
#     "python": [".py", ".pyi"],
#     "javascript": [".js", ".jsx", ".mjs"],
#     "typescript": [".ts", ".tsx", ".d.ts"],
#     "java": [".java"],
#     "rust": [".rs"],
#     "csharp": [".cs"]
#   }
# }
```

### MCP Server

Add to your AI assistant configuration:
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

**MCP Tools**: 
- `findSymbols` - Search symbols with regex patterns
- `findReferences` - Find all references to symbols

## Development

### Build

```bash
make local                  # Primary workflow: build + npm link
make build                  # Build for all platforms
make clean                  # Clean build artifacts

# Platform-specific
make linux                  # Linux amd64
make macos                  # Darwin amd64  
make macos-arm64           # Darwin arm64 (M1/M2)
make windows               # Windows amd64
```

### Test

```bash
make test                   # Complete suite (30+ min)
make test-fast              # Unit + integration only (8-10 min)
go test -v ./tests/unit/... # Unit tests only (2 min)
go test -v -run TestName ./tests/... # Specific test
```

### Code Quality

```bash
make quality                # Format + vet
make quality-full           # Format + vet + lint + security
```

## Configuration

Optional config at `~/.lsp-gateway/config.yaml`:
```yaml
cache:
  enabled: true
  max_memory_mb: 512
  ttl_hours: 24         # MCP mode uses 1hr
servers:
  go:
    command: "gopls"
    args: ["serve"]
  python:
    command: "jedi-language-server"
    args: []
  rust:
    command: "rust-analyzer"
```

## Architecture

```
src/
├── server/         # Core services (LSP manager, HTTP gateway, MCP server)
├── cli/            # Command implementations
├── internal/       # Shared types and utilities
└── tests/          # Test suites
```

Key components:
- **LSP Manager**: Orchestrates language servers with SCIP cache
- **HTTP Gateway**: JSON-RPC endpoint at `:8080/jsonrpc`  
- **MCP Server**: STDIO protocol for AI assistants
- **SCIP Cache**: Memory-efficient symbol indexing

## Troubleshooting

| Issue | Solution |
|-------|----------|
| LSP server not found | `lsp-gateway install all` |
| Port 8080 in use | `lsp-gateway server --port 8081` |
| Debug mode | `export LSP_GATEWAY_DEBUG=true` |
| Cache issues | `lsp-gateway cache clear` |
| Check server status | `lsp-gateway status` |
| Java timeout errors | Normal - Java requires 90s initialization |

## License

MIT
