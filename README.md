# LSP Gateway MVP (ALPHA)

⚠️ **This is an MVP/ALPHA version** - Basic functionality only.

## What is this?

A **generic LSP gateway** that provides a single endpoint for any Language Server Protocol method across multiple language servers. Routes requests based on file extensions with automatic server selection.

**Architecture:**
- **Generic LSP Proxy**: Forwards any LSP method to appropriate servers
- **Multi-Language Support**: 4 languages currently supported (Go, Python, JavaScript/TypeScript, Java)
- **Hybrid Go+Node.js**: High-performance Go core with NPM package distribution
- **Cross-Platform**: Linux, Windows, macOS (Intel/ARM) binaries

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

### CLI Usage

```bash
# Start server with default config
lsp-gateway server

# Start with custom config and port
lsp-gateway server --config config.yaml --port 8080

# Check version
lsp-gateway --version

# Get help
lsp-gateway --help
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

## Platform Support

The LSP Gateway supports the following platforms:

- **Linux x64** - Linux AMD64 systems
- **Windows x64** - Windows AMD64 systems  
- **macOS Intel** - macOS AMD64 systems
- **macOS Apple Silicon** - macOS ARM64 systems

The NPM package automatically detects your platform and installs the appropriate binary.

## Requirements

- **Node.js** 14.0.0 or higher (for NPM package)
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

**Language Support**: The gateway currently supports 4 languages: Go, Python, TypeScript/JavaScript, and Java. The architecture is extensible for future language additions.

## Usage

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

## MVP Limitations

- Basic error handling only
- No authentication/authorization
- No advanced features (caching, pooling, etc.)
- STDIO transport primary (TCP available but less mature)
- No performance optimizations

## Status

### Core Features
- ✅ Generic LSP gateway (forwards any LSP method)
- ✅ Multi-language server support (4 languages: Go, Python, TypeScript/JavaScript, Java)
- ✅ LSP 1:1 protocol mapping
- ✅ File extension-based routing
- ✅ JSON-RPC 2.0 compliant
- ✅ Cross-platform build system

### NPM Package Features
- ✅ Automatic platform detection (Linux, Windows, macOS Intel/ARM)
- ✅ Binary download and installation
- ✅ CLI wrapper for easy usage
- ✅ Programmatic API for Node.js integration
- ✅ Configuration helpers and validation
- ✅ Comprehensive test suite

### Limitations (MVP/ALPHA)
- ❌ Advanced features not implemented
- ❌ No authentication/authorization
- ❌ No caching or connection pooling
- ❌ No performance optimizations

## Troubleshooting

### Common Issues

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

**Configuration errors:**
```bash
# Validate configuration
lsp-gateway server --config config.yaml --dry-run
```

## LSP Protocol Compliance

The gateway implements exact 1:1 LSP protocol mapping:
- Forwards any LSP method without modification
- Maintains standard JSON-RPC 2.0 format
- Routes based on file extensions
- Preserves all LSP semantics