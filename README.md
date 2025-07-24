# LSP Gateway

> **⚠️ Status: MVP/ALPHA** - Development and testing use only

**LSP Gateway** provides dual-protocol access to Language Server Protocol servers:
- **HTTP JSON-RPC Gateway**: REST API for IDEs and development tools
- **MCP Server**: Model Context Protocol server for AI assistants (Claude, GPT, etc.)

**Features**: 4 languages supported • Cross-platform • Auto-setup • 20+ CLI commands

---

## 🚀 Quick Start

**Requirements**: Go 1.24+

```bash
# 1. Build and setup (5 minutes)
git clone [repository-url]
cd lsp-gateway
make local
./bin/lsp-gateway setup all

# 2. Start using
./bin/lsp-gateway server --config config.yaml    # HTTP Gateway (port 8080)
./bin/lsp-gateway mcp --config config.yaml       # MCP Server for AI
```

## Installation

### Automated Setup (Recommended)
```bash
make local                    # Build for current platform
./bin/lsp-gateway setup all   # Install runtimes + language servers
```

### Manual Dependencies
```bash
make local

# Install language servers manually
go install golang.org/x/tools/gopls@latest
pip install python-lsp-server
npm install -g typescript-language-server
```

## Usage

### HTTP Gateway
Send JSON-RPC requests to `http://localhost:8080/jsonrpc`:

```bash
# Example: Go to definition
curl -X POST http://localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "textDocument/definition",
    "params": {
      "textDocument": {"uri": "file:///test.go"},
      "position": {"line": 10, "character": 5}
    }
  }'
```

**Available methods**: `textDocument/definition`, `textDocument/references`, `textDocument/documentSymbol`, `workspace/symbol`, `textDocument/hover`

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

**MCP Tools Available**:
- `goto_definition` - Navigate to symbol definitions  
- `find_references` - Find all references to a symbol
- `get_hover_info` - Get hover information for symbols
- `get_document_symbols` - List symbols in a document
- `search_workspace_symbols` - Search workspace symbols

### Node.js Integration

```javascript
const { LSPGateway } = require('lsp-gateway');

const gateway = new LSPGateway({
  port: 8080,
  config: './config.yaml'
});

await gateway.start();
await gateway.stop();
```

**NPM Scripts**:
```bash
npm run server      # Start HTTP Gateway
npm run diagnose    # System diagnostics
npm run version     # Show version
```

## Commands

### Server Operations
```bash
./bin/lsp-gateway server                      # Start HTTP Gateway (port 8080)
./bin/lsp-gateway server --port 8081          # Custom port
./bin/lsp-gateway mcp                         # Start MCP Server
./bin/lsp-gateway mcp --transport tcp --port 9090  # MCP TCP mode
```

### Setup & Management
```bash
./bin/lsp-gateway setup all          # Complete automated setup
./bin/lsp-gateway setup wizard       # Interactive setup
./bin/lsp-gateway status             # System status
./bin/lsp-gateway diagnose           # System diagnostics
./bin/lsp-gateway config validate    # Validate configuration
```

### Build System
```bash
make local        # Build for current platform
make build        # Build all platforms (Linux, Windows, macOS x64/arm64)
make test         # Run tests
make lint         # Code quality check
make clean        # Clean artifacts
```

## Configuration

Auto-generated `config.yaml` supports **Go, Python, JavaScript/TypeScript, Java**:

```yaml
port: 8080
servers:
  - name: "go-lsp"
    languages: ["go"]
    command: "gopls"
    transport: "stdio"
    
  - name: "python-lsp"
    languages: ["python"] 
    command: "python"
    args: ["-m", "pylsp"]
    transport: "stdio"
    
  - name: "typescript-lsp"
    languages: ["typescript", "javascript"]
    command: "typescript-language-server"
    args: ["--stdio"]
    transport: "stdio"
```

## Architecture

**Request Flow**:
```
HTTP → Gateway → Router → LSPClient → LSP Server
MCP → ToolHandler → LSPGatewayClient → HTTP Gateway → Router → LSPClient → LSP Server
```

**Core Components**:
- **Router**: Thread-safe request routing (80+ file extensions)
- **Transport**: Pluggable LSP communication (stdio/TCP) with circuit breakers
- **Auto-Setup**: Cross-platform installation system
- **CLI**: 20+ management commands

## Troubleshooting

### Quick Diagnostics
```bash
./bin/lsp-gateway diagnose              # Comprehensive diagnostics
./bin/lsp-gateway status runtimes       # Check runtime installations
./bin/lsp-gateway status servers        # Check language server status
```

### Common Issues
```bash
# Installation problems
./bin/lsp-gateway install servers       # Install missing language servers
./bin/lsp-gateway verify runtime all    # Verify runtime installations

# Configuration issues  
./bin/lsp-gateway config generate --auto-detect  # Regenerate config
./bin/lsp-gateway config validate       # Validate existing config

# Build problems
make clean && make local                # Clean rebuild

# Port conflicts
./bin/lsp-gateway server --port 8081    # Use different port
```

## Requirements & Dependencies

- **Go 1.24+** (primary requirement)
- **Node.js 18.0.0+** (optional, for npm package wrapper)
- **Language runtimes** (auto-installed): Go, Python 3.8+, Java 17+

**Minimal Go dependencies**:
- `github.com/spf13/cobra v1.9.1` - CLI framework
- `golang.org/x/text v0.27.0` - Text processing
- `gopkg.in/yaml.v3 v3.0.1` - YAML configuration

## Integration Examples

### IDE Integration
Configure your IDE to send LSP requests to `http://localhost:8080/jsonrpc`

### AI Assistant Integration  
Start MCP server: `./bin/lsp-gateway mcp` and configure your AI tool

### Custom Integration
Use the HTTP API or Node.js package for custom tooling

---

**Get Started**: `./bin/lsp-gateway setup all` 🚀