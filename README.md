# LSP Gateway

> **⚠️ Development Status**: MVP/ALPHA - Use with caution for development and testing purposes.

LSP Gateway provides dual protocol access to language servers:
- **LSP Gateway**: HTTP JSON-RPC server for IDEs and development tools
- **MCP Server**: Model Context Protocol server for AI models (Claude, GPT, etc.)

**Quick Features**: Routes LSP methods to language servers | AI integration via MCP | 4 languages supported | Cross-platform (Linux, Windows, macOS)

## Quick Start (5 minutes)

**Requirements**: Go 1.24+

```bash
# 1. Build and setup
git clone [repository-url]
cd lsp-gateway
make local
./bin/lsp-gateway setup all

# 2. Start using
./bin/lsp-gateway server --config config.yaml    # HTTP Gateway (port 8080)
./bin/lsp-gateway mcp --config config.yaml       # MCP Server for AI tools
```

## Installation Methods

### Method 1: Automated Setup (Recommended)
```bash
make local
./bin/lsp-gateway setup all                   # Installs runtimes + language servers
./bin/lsp-gateway server --config config.yaml
```

### Method 2: Manual Dependencies
```bash
make local

# Install language servers manually
go install golang.org/x/tools/gopls@latest
pip install python-lsp-server
npm install -g typescript-language-server

./bin/lsp-gateway server --config config.yaml
```

## Essential Commands

### Server Operations
```bash
./bin/lsp-gateway server                      # Start HTTP Gateway (default port 8080)
./bin/lsp-gateway server --port 8081          # Custom port
./bin/lsp-gateway mcp                         # Start MCP Server (stdio)
./bin/lsp-gateway mcp --transport tcp --port 9090  # MCP TCP mode
```

### Setup & Management
```bash
./bin/lsp-gateway setup all                   # Complete automated setup
./bin/lsp-gateway setup wizard                # Interactive setup
./bin/lsp-gateway status                      # System status
./bin/lsp-gateway diagnose                    # System diagnostics
./bin/lsp-gateway config validate             # Validate configuration
./bin/lsp-gateway workflows                   # Usage examples
```

### Build System
```bash
make local        # Build for current platform
make build        # Build for all platforms (Linux, Windows, macOS x64/arm64)
make test         # Run component tests
make lint         # Code quality check
make clean        # Clean artifacts
make help         # Show all targets
```

## Configuration

Auto-generated `config.yaml`:
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
    
  - name: "java-lsp"
    languages: ["java"]
    command: "jdtls"
    transport: "stdio"
```

**Supported Languages**: Go, Python, JavaScript/TypeScript, Java

## Usage

### HTTP Gateway API
Send JSON-RPC requests to `http://localhost:8080/jsonrpc`:

```bash
# Go to definition
curl -X POST http://localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"textDocument/definition","params":{"textDocument":{"uri":"file:///test.go"},"position":{"line":10,"character":5}}}'
```

**Available methods**: `textDocument/definition`, `textDocument/references`, `textDocument/documentSymbol`, `workspace/symbol`, `textDocument/hover`

### MCP Server for AI Integration

**Configure AI tools** (Claude Desktop, etc.):
```json
{
  "mcpServers": {
    "lsp-gateway": {
      "command": "lsp-gateway",
      "args": ["mcp"],
      "env": {}
    }
  }
}
```

**Available MCP tools**:
- `goto_definition` - Navigate to symbol definitions  
- `find_references` - Find all references to a symbol
- `get_hover_info` - Get hover information for symbols
- `get_document_symbols` - List symbols in a document
- `search_workspace_symbols` - Search workspace symbols

### Programmatic Usage (Node.js)

```javascript
const { LSPGateway } = require('lsp-gateway');

const gateway = new LSPGateway({
  port: 8080,
  config: './config.yaml'
});

await gateway.start();
await gateway.stop();
```

**Available npm scripts**:
```bash
npm run server      # Start HTTP Gateway
npm run diagnose    # System diagnostics  
npm run version     # Show version
```

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
./bin/lsp-gateway config generate --auto-detect  # Generate config with auto-detection
./bin/lsp-gateway config validate       # Validate existing config

# Build problems
make clean && make local                # Clean rebuild

# Port conflicts
./bin/lsp-gateway server --port 8081    # Use different port
```

### Language Server Installation

**Auto-install via CLI** (recommended):
```bash
./bin/lsp-gateway setup all
```

**Manual installation**:
```bash
# Go
go install golang.org/x/tools/gopls@latest

# Python  
pip install python-lsp-server

# TypeScript/JavaScript
npm install -g typescript-language-server

# Java
# Download Eclipse JDT Language Server from official site
```

## Requirements

- **Go 1.24+** (primary requirement)
- **Node.js 18.0.0+** (optional, for npm package wrapper)
- **Language runtimes** (auto-installed by setup system): Go, Python 3.8+, Java 17+

## Dependencies

**Minimal Go dependencies**:
- `github.com/spf13/cobra v1.9.1` - CLI framework
- `github.com/spf13/pflag v1.0.6` - CLI flag parsing  
- `golang.org/x/text v0.27.0` - Text processing
- `gopkg.in/yaml.v3 v3.0.1` - YAML configuration parsing
- Go 1.24+ standard library

## Architecture

**Request Flow**:
```
HTTP → Gateway → Router → LSPClient → LSP Server
MCP → ToolHandler → LSPGatewayClient → HTTP Gateway → Router → LSPClient → LSP Server
```

**Core Components**:
- **Router**: Thread-safe request routing (80+ file extensions supported)
- **Transport**: Pluggable LSP communication (stdio/TCP) with circuit breakers
- **Auto-Setup**: Cross-platform installation system
- **CLI**: Comprehensive management interface with 20+ commands

## Integration Examples

### IDE Integration
Configure your IDE to send LSP requests to `http://localhost:8080/jsonrpc`

### AI Assistant Integration  
Start MCP server: `./bin/lsp-gateway mcp` and configure your AI tool

### Custom Integration
Use the HTTP API or Node.js package for custom tooling

---

**Get Started**: `./bin/lsp-gateway setup all` 🚀