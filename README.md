# LSP Gateway

> **⚠️ Development Status**: MVP/ALPHA - Use with caution for development and testing purposes.

LSP Gateway provides dual protocol access to language servers:

- **LSP Gateway**: HTTP JSON-RPC server for IDEs and development tools
- **MCP Server**: Model Context Protocol server for AI models (Claude, GPT, etc.)

**Features:**
- Routes LSP methods to appropriate language servers
- AI integration via MCP protocol
- 4 languages supported: Go, Python, JavaScript/TypeScript, Java
- Cross-platform binaries (Linux, Windows, macOS Intel/ARM)

## Installation

### NPM Package (Recommended)

```bash
# Install globally
npm install -g lsp-gateway

# Or install locally in your project
npm install lsp-gateway
```

The NPM package automatically detects your platform (Linux, Windows, macOS Intel/ARM) and downloads the appropriate binary.

### Manual Build

```bash
# Build from source
go build -o lsp-gateway cmd/lsp-gateway/main.go

# Run
./lsp-gateway server --config config.yaml --port 8080
```

## Quick Start

### LSP Gateway Server

```bash
# Start LSP server with default config
lsp-gateway server

# Start with custom config and port
lsp-gateway server --config config.yaml --port 8080

# Check version
lsp-gateway version

# Get help
lsp-gateway --help
```

### MCP Server for AI Integration

```bash
# Start MCP server (for Claude, GPT, etc.)
lsp-gateway mcp

# Start MCP server with custom config
lsp-gateway mcp --config config.yaml
```

### Run Both Servers

```bash
# Run separately in different terminals
lsp-gateway server &
lsp-gateway mcp &
```

### Programmatic Usage

```javascript
const { LSPGateway } = require('lsp-gateway');

// Create gateway instance
const gateway = new LSPGateway({
  port: 8080,
  config: './config.yaml'
});

// Start the server
await gateway.start();

// Stop the server
await gateway.stop();
```

## Requirements

- **Node.js** 18.0.0 or higher (for NPM package)
- **Language Servers** (install as needed):
  - Go: `go install golang.org/x/tools/gopls@latest`
  - Python: `pip install python-lsp-server`
  - TypeScript/JavaScript: `npm install -g typescript-language-server`
  - Java: Download from [Eclipse JDT Language Server](https://github.com/eclipse/eclipse.jdt.ls)

## Configuration

Create `config.yaml`:

```yaml
port: 8080
servers:
  - name: "go-lsp"
    languages: ["go"]
    command: "gopls"
    args: []
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
    args: []
    transport: "stdio"
```

**Language Support**: Currently supports 4 languages: Go, Python, JavaScript/TypeScript, and Java.

## Usage

### LSP Gateway Usage

Send JSON-RPC requests to `http://localhost:8080/jsonrpc`:

```bash
# Go to definition
curl -X POST http://localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"textDocument/definition","params":{"textDocument":{"uri":"file:///test.go"},"position":{"line":10,"character":5}}}'

# Find references
curl -X POST http://localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"textDocument/references","params":{"textDocument":{"uri":"file:///test.go"},"position":{"line":10,"character":5},"context":{"includeDeclaration":true}}}'

# Document symbols
curl -X POST http://localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":3,"method":"textDocument/documentSymbol","params":{"textDocument":{"uri":"file:///test.go"}}}'

# Workspace symbols
curl -X POST http://localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":4,"method":"workspace/symbol","params":{"query":"MyFunction"}}'

# Java example
curl -X POST http://localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":5,"method":"textDocument/definition","params":{"textDocument":{"uri":"file:///test.java"},"position":{"line":15,"character":8}}}'
```

### MCP Server Usage (AI Integration)

The MCP server exposes LSP functions to AI models. Connect your AI tool (Claude, GPT, etc.) to the MCP server:

**MCP Server Configuration** for AI tools:
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

**Available MCP Tools** for AI models:
- `goto_definition` - Navigate to symbol definitions
- `find_references` - Find all references to a symbol
- `get_hover_info` - Get hover information for symbols
- `get_document_symbols` - List symbols in a document
- `search_workspace_symbols` - Search workspace symbols

**Example AI Integration**:
```bash
# AI models can now call LSP functions directly:
# "Find the definition of MyClass in main.go"
# "Show me all references to the calculateTotal function" 
# "What symbols are available in utils.py?"
```

## Build System

The project includes comprehensive build tools:

```bash
# Build for all platforms
make build

# Build for specific platform
make linux    # Linux AMD64
make windows  # Windows AMD64
make macos    # macOS Intel
make macos-arm64  # macOS Apple Silicon

# Run tests
make test

# Clean build artifacts
make clean
```

## Current Limitations

- MVP/ALPHA stage - not production-ready
- No caching or connection pooling
- Limited to 4 languages currently
- STDIO transport primary (TCP available)

## Troubleshooting

**LSP Gateway not starting:**
```bash
# Check if port is available
netstat -tlnp | grep :8080

# Try different port
lsp-gateway server --port 8081
```

**Language server not found:**
```bash
# Check if language server is installed
which gopls    # For Go
which pylsp    # For Python
which typescript-language-server  # For TypeScript
```