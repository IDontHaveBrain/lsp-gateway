# LSP Gateway

**LSP Gateway** is a dual-protocol Language Server Protocol gateway for local development:
- **HTTP JSON-RPC Gateway**: REST API at `localhost:8080/jsonrpc` for IDEs
- **MCP Server**: AI assistant integration for Claude, GPT, etc.
- **Cross-platform CLI**: Essential commands for LSP server management

**Focus**: 6 core LSP features • 4 languages • Local development tool

📖 **Complete Guide**: [CLAUDE.md](CLAUDE.md) for architecture and development workflows

---

## 🚀 Quick Start (5 Minutes)

**Requirements**: Go 1.24+, language servers installed

```bash
# 1. Clone and build
git clone <repository-url>
cd lsp-gateway
make local                    # Builds + creates global 'lsp-gateway' command

# 2. Install language servers (choose what you need)
go install golang.org/x/tools/gopls@latest                    # Go
pip install python-lsp-server                                 # Python  
npm install -g typescript-language-server typescript         # TypeScript/JS
# Java: Install Eclipse JDT Language Server manually

# 3. Start using
lsp-gateway server            # HTTP Gateway (port 8080) - uses config.yaml
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
make test-quick   # Run quick validation tests
make quality      # Essential checks (format + vet)
```

## 🎯 Supported LSP Features

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

Default `config.yaml` supports all 4 languages:

```yaml
servers:
  go:
    command: "gopls"
    args: []
  python:
    command: "pylsp"
    args: []
  javascript:
    command: "typescript-language-server"
    args: ["--stdio"]
  typescript:
    command: "typescript-language-server"
    args: ["--stdio"]
  java:
    command: "jdtls"
    args: []
```

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

## Requirements

- **Go 1.24+** for building from source
- **Language servers**: Install manually for each language you need:
  - Go: `go install golang.org/x/tools/gopls@latest`
  - Python: `pip install python-lsp-server`
  - TypeScript/JS: `npm install -g typescript-language-server typescript`
  - Java: Install Eclipse JDT Language Server
- **Platforms**: Linux, macOS (x64/arm64), Windows (x64)

## Integration Examples

### IDE Integration
Point your IDE's LSP client to `http://localhost:8080/jsonrpc`

### AI Assistant Integration  
```bash
lsp-gateway mcp    # Start MCP server, configure in your AI tool
```

### Node.js Integration
```bash
npm install -g lsp-gateway
lsp-gateway server    # Start HTTP Gateway
# Or using npm scripts:
npm run server        # Start HTTP Gateway
npm run mcp           # Start MCP Server
```

---

**Get Started**: `make local && lsp-gateway server` 🚀