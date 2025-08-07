# LSP Gateway

Language Server Protocol gateway for local development with dual-protocol support:
- **HTTP JSON-RPC Gateway** at `localhost:8080/jsonrpc` for IDE integration
- **MCP Server** for AI assistants (Claude, GPT, etc.) with enhanced caching
- **SCIP Cache** for sub-millisecond symbol lookups

## Quick Start

**Requirements**: Go 1.24+, Node.js 18+

```bash
# Clone and build
git clone https://github.com/yourusername/lsp-gateway
cd lsp-gateway
make local                    # Build + install globally

# Install language servers
lsp-gateway install all       # Install all 5 language servers

# Start using
lsp-gateway server           # HTTP Gateway (port 8080)
lsp-gateway mcp             # MCP Server for AI assistants
```

## Core Commands

```bash
# Server operations
lsp-gateway server          # HTTP Gateway at localhost:8080/jsonrpc
lsp-gateway mcp             # MCP Server (STDIO, auto-detects languages)
lsp-gateway status          # Check LSP server availability
lsp-gateway test            # Test LSP connections

# Setup & management
lsp-gateway install all     # Install all language servers
lsp-gateway install go      # Install specific language (go/python/typescript/javascript/java)
lsp-gateway cache info      # Show cache statistics
lsp-gateway cache clear     # Clear cache entries

# Development
make local                  # Build + npm global link
make quality               # Format + vet checks
make test-unit             # Run unit tests
```

## Features

- **Languages**: Go, Python, JavaScript, TypeScript, Java
- **LSP Methods**: definition, references, hover, documentSymbol, workspace/symbol, completion
- **MCP Tools**: findSymbols, findReferences, findDefinitions, getSymbolInfo
- **Cache**: 512MB SCIP-based LRU cache with sub-millisecond lookups
- **Auto-detection**: Scans for go.mod, package.json, *.py, pom.xml

## Usage Examples

### HTTP Gateway
```bash
curl -X POST http://localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"workspace/symbol","params":{"query":"main"}}'
```

### MCP Server Configuration
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

## Configuration

LSP Gateway works out-of-the-box with auto-detection. For custom settings, create `~/.lsp-gateway/config.yaml`:

```yaml
cache:
  max_memory_mb: 512
  ttl_hours: 24
servers:
  go:
    command: "gopls"
    args: ["serve"]
```

See [docs/configuration.md](docs/configuration.md) for full options.

## Troubleshooting

```bash
# Diagnostics
lsp-gateway status               # Check LSP servers
lsp-gateway test                 # Test connections
export LSP_GATEWAY_DEBUG=true    # Enable debug logging

# Common fixes
lsp-gateway install all          # Reinstall LSP servers
lsp-gateway server --port 8081   # Use different port
make clean && make local         # Clean rebuild
```

## Development

```bash
# Build commands
make build          # Build for all platforms
make linux          # Linux build
make macos          # macOS build
make windows        # Windows build

# Quality & testing
make quality-full   # Complete checks (format + vet + lint + security)
make test          # Run all tests
make cache-test    # Cache-specific tests
```

## Requirements

- Go 1.24.0+ (with toolchain go1.24.5)
- Node.js 18+
- Platform: Linux, macOS (x64/arm64), Windows (x64)

## License

MIT