# LSP Gateway

**LSP Gateway** is a dual-protocol Language Server Protocol gateway for local development:
- **HTTP JSON-RPC Gateway**: REST API at `localhost:8080/jsonrpc` for IDEs
- **MCP Server**: AI assistant integration for Claude, GPT, etc.
- **Cross-platform CLI**: Essential commands for LSP server management

**Focus**: 6 core LSP features • 4 languages • Local development tool

**Complete Guide**: [CLAUDE.md](CLAUDE.md) for architecture and development workflows

---

## Quick Start (5 Minutes)

**Requirements**: Go 1.24+, Node.js 18+, language servers installed

```bash
# 1. Clone and build
git clone <repository-url>
cd lsp-gateway
make local                    # Builds + creates global 'lsp-gateway' command

# 2. Install language servers (choose what you need)
go install golang.org/x/tools/gopls@latest                    # Go
npm install -g pyright                                        # Python  
npm install -g typescript-language-server typescript         # TypeScript/JS
# Java: Install Eclipse JDT Language Server manually

# 3. Start using
lsp-gateway server            # HTTP Gateway (port 8080)
lsp-gateway mcp               # MCP Server for AI assistants
```

## Core Commands

### Server Operations
```bash
lsp-gateway server             # Start HTTP Gateway (port 8080)
lsp-gateway mcp                # Start MCP Server for AI assistants
lsp-gateway status             # Show LSP server status  
lsp-gateway test               # Test LSP server connections
```

### Development
```bash
make local        # Build + create global 'lsp-gateway' command
make unlink       # Remove global 'lsp-gateway' command  
make quality      # Essential checks (format + vet)
```

## Supported LSP Features

**Exactly 6 core LSP features** for essential developer productivity:

| **LSP Method** | **Description** |
|----------------|-----------------|
| `textDocument/definition` | Go to definition |
| `textDocument/references` | Find all references |
| `textDocument/hover` | Hover information |
| `textDocument/documentSymbol` | Document symbols |
| `workspace/symbol` | Workspace symbol search |
| `textDocument/completion` | Code completion |

**Languages**: Go, Python, JavaScript/TypeScript, Java

## Usage

### HTTP Gateway
Send JSON-RPC requests to `http://localhost:8080/jsonrpc`:

```bash
curl -X POST http://localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "workspace/symbol",
    "params": {"query": "main"}
  }'
```

### MCP Server for AI Integration
Configure AI tools (Claude Desktop, etc.):
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

**Auto-detection mode** (recommended): `lsp-gateway mcp` automatically detects languages and available LSP servers.

**Custom configuration**: Create `config.yaml` for advanced settings, cache configuration, and server customization.

**Complete guide**: [docs/configuration.md](docs/configuration.md) covers all configuration options, templates, and examples.

## Troubleshooting

### Quick Diagnostics
```bash
lsp-gateway status                # Check LSP server status
lsp-gateway test                  # Test LSP server connections
curl http://localhost:8080/health # HTTP Gateway health check
```

### Common Issues
```bash
# Build problems
make clean && make local          # Clean rebuild

# Port conflicts  
lsp-gateway server --port 8081    # Use different port

# Language server not found
which gopls                       # Check if language server is in PATH
```

### IDE Integration
Point your IDE's LSP client to `http://localhost:8080/jsonrpc` after starting the HTTP gateway.

### Additional Endpoints
```bash
curl http://localhost:8080/health       # Health check
curl http://localhost:8080/cache/stats  # Cache performance metrics
```

**Platforms**: Linux, macOS (x64/arm64), Windows (x64)