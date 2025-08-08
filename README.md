# LSP Gateway

Local Language Server Protocol gateway providing HTTP and MCP interfaces to 5 language servers.

## Quick Start

```bash
# Requirements: Go 1.24+, Node.js 18+
git clone https://github.com/IDontHaveBrain/lsp-gateway
cd lsp-gateway
make local                   # Build + npm link globally
lsp-gateway install all      # Install language servers
lsp-gateway server           # Start HTTP Gateway on :8080
```

Test it's working:
```bash
lsp-gateway status           # Check installed servers
curl localhost:8080/jsonrpc  # Verify HTTP gateway
```

## Features

- **5 Languages**: Go, Python, JavaScript, TypeScript, Java
- **Dual Protocol**: HTTP Gateway (port 8080) + MCP Server (STDIO)
- **Auto-detection**: Scans for go.mod, package.json, *.py, pom.xml
- **SCIP Cache**: 512MB LRU cache, sub-millisecond lookups
- **LSP Methods**: definition, references, hover, documentSymbol, workspace/symbol, completion

## Commands

```bash
# Servers
lsp-gateway server          # HTTP Gateway at localhost:8080/jsonrpc
lsp-gateway mcp             # MCP Server for AI assistants
lsp-gateway status          # Check LSP server availability
lsp-gateway test            # Test connections
lsp-gateway version         # Show version info

# Installation
lsp-gateway install all        # Install all 5 language servers
lsp-gateway install go         # Install gopls
lsp-gateway install python     # Install python-lsp-server
lsp-gateway install typescript # Install typescript-language-server
lsp-gateway install javascript # Install typescript-language-server
lsp-gateway install java       # Install jdtls

# Cache Management
lsp-gateway cache info      # Show cache statistics
lsp-gateway cache clear     # Clear all cache
lsp-gateway cache index     # Index files for faster lookups
```

## Usage

### HTTP Gateway
```bash
curl -X POST localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"workspace/symbol","params":{"query":"main"}}'
```

### MCP Server
Configure in your AI assistant:
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
**MCP Tools**: `findSymbols`, `findReferences` - Enhanced symbol search with SCIP caching.

## Development

```bash
make local                  # Build + npm link
make clean                  # Clean build artifacts
make quality                # Format + vet
make quality-full           # Format + vet + lint + security

# Testing
make test                   # All tests (unit + integration + e2e)
make test-fast              # Quick tests (skip e2e)
go test -v ./tests/unit/... # Unit tests only
go test -v -run TestName ./tests/... # Run specific test
```

## Configuration

Auto-detects projects. Optional config at `~/.lsp-gateway/config.yaml`:

```yaml
cache:
  enabled: true
  max_memory_mb: 512
  ttl_hours: 24         # MCP overrides to 1hr
servers:
  go:
    command: "gopls"
    args: ["serve"]
```

## Troubleshooting

| Issue | Solution |
|-------|----------|
| LSP server not found | `lsp-gateway install all` |
| Port 8080 in use | `lsp-gateway server --port 8081` |
| Debug issues | `export LSP_GATEWAY_DEBUG=true` |
| Cache problems | `lsp-gateway cache clear` |
| Check status | `lsp-gateway status` |

## License

MIT