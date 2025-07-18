# LSP Gateway MVP (ALPHA)

⚠️ **This is an MVP/ALPHA version** - Basic functionality only.

## What is this?

A gateway that provides a single endpoint for LSP functions across multiple open-source language servers. Also provides LSP functions via MCP (Model Context Protocol) integration.

## Quick Start

```bash
# Build
go build -o lsp-gateway cmd/lsp-gateway/main.go

# Run
./lsp-gateway server --config config.yaml --port 8080
```

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
```

## Usage

Send JSON-RPC requests to `http://localhost:8080/jsonrpc`:

```bash
curl -X POST http://localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"textDocument/hover","params":{"textDocument":{"uri":"file:///test.go"},"position":{"line":0,"character":0}}}'
```

## MVP Limitations

- Basic error handling only
- No authentication/authorization
- No advanced features (caching, pooling, etc.)
- Limited to essential LSP methods
- STDIO transport only
- MCP server support not yet implemented

## Status

- ✅ Core functionality working
- ✅ Multi-language server support
- ✅ Basic request routing
- ❌ Advanced features not implemented
- ❌ MCP server support not implemented