# LSP Gateway

> **⚠️ Status: MVP/ALPHA** - Development and testing use only

**LSP Gateway** is a dual-protocol Language Server Protocol gateway that provides:
- **HTTP JSON-RPC Gateway**: REST API at `localhost:8080/jsonrpc` for IDEs
- **MCP Server**: AI assistant integration interface for Claude, GPT, etc.
- **Cross-platform CLI**: Essential commands for setup and management
- **SCIP Intelligent Caching**: 60-87% performance improvements

**Focus**: 6 core LSP features • 4 languages supported • Local development tool

📖 **Complete Developer Guide**: See [CLAUDE.md](CLAUDE.md) for architecture, development workflows, and comprehensive documentation.

---

## 🚀 Quick Start (5 Minutes)

**Requirements**: Go 1.24+

```bash
# 1. Clone and build (2 minutes)
git clone [repository-url]
cd lsp-gateway
make local

# 2. Automated setup (2 minutes) 
./bin/lsp-gateway setup all          # Installs runtimes + language servers + config

# 3. Start using (30 seconds)
./bin/lsp-gateway server --config config.yaml    # HTTP Gateway (port 8080)
./bin/lsp-gateway mcp --config config.yaml       # MCP Server for AI assistants
```

## Usage

### HTTP Gateway
Send JSON-RPC requests to `http://localhost:8080/jsonrpc` for IDE integration.

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

### Node.js Integration
```bash
npm install lsp-gateway
npm run server      # Start HTTP Gateway
```

## 🎯 Supported LSP Features

> **⚠️ IMPORTANT**: LSP Gateway supports **exactly 6 core LSP features** only, focused on essential developer productivity.

| **LSP Method** | **Description** | **Use Case** |
|----------------|-----------------|--------------|
| `textDocument/definition` | Go to definition | Navigate to symbol definitions |
| `textDocument/references` | Find all references | Find all uses of a symbol |
| `textDocument/hover` | Hover information | Get documentation and type info |
| `textDocument/documentSymbol` | Document symbols | Get document outline/structure |
| `workspace/symbol` | Workspace symbol search | Search symbols across files |
| `textDocument/completion` | Code completion | IntelliSense and autocompletion |

**✅ Supported**: These 6 features provide the core IDE experience for local development.

**❌ Not Supported**: Advanced features like formatting, diagnostics, code actions, refactoring, etc. are intentionally excluded to keep the gateway simple and focused.

## Essential Commands

### Server Operations
```bash
./bin/lsp-gateway server             # Start HTTP Gateway (port 8080)
./bin/lsp-gateway mcp                # Start MCP Server for AI assistants
```

### Setup & Management
```bash
./bin/lsp-gateway setup all          # Complete automated setup
./bin/lsp-gateway status             # System status
./bin/lsp-gateway diagnose           # System diagnostics
./bin/lsp-gateway config validate    # Validate configuration
```

### Build
```bash
make local        # Build for current platform
make test         # Run tests
```

📖 **Complete CLI Reference**: See [CLAUDE.md](CLAUDE.md) for all 20+ commands and advanced options.

## Configuration

Auto-generated `config.yaml` supports **Go, Python, JavaScript/TypeScript, Java**:

```yaml
port: 8080
servers:
  - name: "go-lsp"
    languages: ["go"]
    command: "gopls"
    transport: "stdio"
```

Generate configuration: `./bin/lsp-gateway config generate --auto-detect`

📖 **Configuration Guide**: See [CLAUDE.md](CLAUDE.md) for templates, framework detection, and advanced options.

## Troubleshooting

### Quick Diagnostics
```bash
./bin/lsp-gateway diagnose              # Comprehensive diagnostics
./bin/lsp-gateway status                # System status
./bin/lsp-gateway config validate       # Validate configuration
```

### Common Issues
```bash
# Installation problems
./bin/lsp-gateway setup all             # Reinstall everything

# Configuration issues  
./bin/lsp-gateway config generate --auto-detect  # Regenerate config

# Build problems
make clean && make local                # Clean rebuild

# Port conflicts
./bin/lsp-gateway server --port 8081    # Use different port
```

📖 **Troubleshooting Guide**: See [docs/troubleshooting.md](docs/troubleshooting.md) for comprehensive troubleshooting.

## Requirements

- **Go 1.24+** (primary requirement)
- **Node.js 22.0.0+** (optional, for npm integration)
- **Platforms**: Linux, macOS (x64/arm64), Windows (x64)

**Language runtimes** (auto-installed): Go, Python 3.9+, Java 17+

## Integration

### IDE Integration
Configure your IDE to send LSP requests to `http://localhost:8080/jsonrpc`

### AI Assistant Integration  
Start MCP server: `./bin/lsp-gateway mcp` and configure your AI tool

📖 **Testing Guide**: See [docs/test_guide.md](docs/test_guide.md) for testing infrastructure and procedures.

📖 **AI Code Reference**: See [docs/ai_code_reference.md](docs/ai_code_reference.md) for AI/Agent navigation.

---

**Get Started**: `./bin/lsp-gateway setup all` 🚀