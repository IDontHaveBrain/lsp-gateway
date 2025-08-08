# LSP Gateway

Dual-protocol Language Server Protocol gateway for local development.

## Quick Start

```bash
# Requirements: Go 1.24+, Node.js 18+
git clone https://github.com/IDontHaveBrain/lsp-gateway
cd lsp-gateway
make local                   # Build + npm link globally
lsp-gateway install all      # Install language servers
lsp-gateway server           # Start HTTP Gateway on :8080
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
lsp-gateway mcp             # MCP Server for AI assistants (2 enhanced tools)
lsp-gateway status          # Check LSP server availability
lsp-gateway test            # Test connections

# Installation
lsp-gateway install all     # Install all language servers
lsp-gateway install go      # Install specific (go/python/typescript/javascript/java)

# Cache
lsp-gateway cache info      # Cache statistics
lsp-gateway cache clear     # Clear cache
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
**MCP Tools**: `findSymbols`, `findReferences` - Enhanced SCIP-based symbol search with regex patterns and role filtering.

## Development

```bash
make local                  # Build + npm link
make quality                # Format + vet
make quality-full           # Format + vet + lint + security

# Testing
make test                   # Run ALL tests (unit + integration + e2e)
make test-fast              # Quick tests only (unit + integration)
go test -v ./tests/unit/... # Unit tests only
go test -v ./tests/e2e/...  # E2E tests only (30min, uses real GitHub repos)
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

```bash
lsp-gateway status               # Verify LSP servers
lsp-gateway install all          # Reinstall if needed
export LSP_GATEWAY_DEBUG=true    # Enable debug logs
lsp-gateway server --port 8081   # Use different port
```

## License

MIT