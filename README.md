# LSP Gateway

HTTP and MCP gateway for Language Server Protocol with 8 languages.

## Quick Start

### Install from npm

```bash
# Prerequisites: Node.js 18+
npm install -g lsp-gateway
lsp-gateway install all      # Install all language servers
lsp-gateway server           # Start HTTP Gateway on :8080
```

### Build from source

```bash
# Prerequisites: Go 1.23+, Node.js 18+
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

- **8 Languages**: Go, Python, JavaScript, TypeScript, Java, Rust, C#, Kotlin
- **HTTP Gateway**: JSON-RPC endpoint on port 8080
- **MCP Server**: STDIO protocol for AI assistants
- **Auto-detection**: Language detection via project files
- **SCIP Cache**: Fast symbol lookups with 512MB LRU cache
- **LSP Methods**: definition, references, hover, symbols, completion

## Installation

### Language Servers

```bash
lsp-gateway install all        # Install all supported servers
lsp-gateway install go         # Install gopls
lsp-gateway install python     # Install basedpyright
lsp-gateway install typescript # Install typescript-language-server
lsp-gateway install javascript # Install typescript-language-server
lsp-gateway install java       # Install jdtls
lsp-gateway install rust       # Install rust-analyzer
lsp-gateway install csharp     # Install omnisharp
lsp-gateway install kotlin     # Install kotlin-lsp (official JetBrains)
```

## Usage

### Commands

```bash
# Servers
lsp-gateway server          # HTTP Gateway at localhost:8080/jsonrpc
lsp-gateway server --port 8081  # Alternative port
lsp-gateway mcp             # MCP Server for AI assistants

# Status & Info
lsp-gateway status          # Check LSP server availability
lsp-gateway test            # Test server connections
lsp-gateway version         # Show version info

# Cache
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
```bash
curl localhost:8080/languages   # List supported languages and extensions
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

MCP tools: `findSymbols`, `findReferences`

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

### Test & Quality

```bash
make test-fast              # Unit + integration (8-10 min, recommended)
make test                   # Complete suite (30+ min)
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
  python:
    command: "basedpyright-langserver"
    args: ["--stdio"]
  go:
    command: "gopls"
    args: ["serve"]
```

## Architecture

- **LSP Manager**: Orchestrates language servers with SCIP cache
- **HTTP Gateway**: JSON-RPC endpoint at `:8080/jsonrpc`  
- **MCP Server**: STDIO protocol for AI assistants
- **SCIP Cache**: Memory-efficient symbol indexing with 512MB LRU

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
