# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

LSP Gateway is a dual-protocol Language Server Protocol gateway written in Go that provides:
- **HTTP JSON-RPC Gateway**: REST API at `localhost:8080/jsonrpc` for IDEs
- **Model Context Protocol (MCP) Server**: AI assistant integration interface
- **Cross-platform CLI**: 20+ commands for setup, management, and diagnostics

**Status**: MVP/ALPHA - Use caution in production
**Languages**: Go, Python, JavaScript/TypeScript, Java
**Platforms**: Linux, Windows, macOS (x64/arm64)

## Installation and Setup

### Automated Setup (Recommended)
```bash
make local                           # Build for current platform
./bin/lsp-gateway setup all          # Installs runtimes + language servers
./bin/lsp-gateway server --config config.yaml
```

### Manual Installation
```bash
make local
# Install language servers manually
go install golang.org/x/tools/gopls@latest
pip install python-lsp-server
npm install -g typescript-language-server
```

## Common Development Commands

### Build Commands
```bash
make local                    # Build for current platform
make build                    # Build all platforms
make clean                    # Clean build artifacts
```

### Testing Commands
```bash
make test                     # Run all tests
make test-unit               # Fast unit tests only (<60s)
make test-integration        # Integration + performance tests
make test-lsp-validation     # Comprehensive LSP validation
make test-simple-quick       # Quick validation for development
```

### Code Quality
```bash
make format                  # Format code
make lint                    # Run golangci-lint
make security               # Run gosec security analysis
make check-deadcode         # Dead code analysis
```

### LSP Testing Setup
```bash
./bin/lsp-gateway setup all # Install LSP servers and runtimes (automated setup)
make setup-simple-repos     # Setup test repositories
make test-lsp-repos         # Repository validation tests
```

### Development Workflow
```bash
# Quick development cycle
make local && make test-unit && make format && make lint

# Full validation before PR
make test && make test-lsp-validation-short && make security
```

## Architecture Overview

### Core Request Flow
```
HTTP → Gateway → Router → LSPClient → LSP Server
MCP → ToolHandler → LSPGatewayClient → HTTP Gateway → Router → LSPClient → LSP Server
```

### Key Components
- **Gateway Layer** (`internal/gateway/`): HTTP routing, JSON-RPC protocol, server management
- **Transport Layer** (`internal/transport/`): STDIO/TCP communication with circuit breakers
- **CLI Interface** (`internal/cli/`): Comprehensive command system with 11 main subcommands
- **Setup System** (`internal/setup/`): Cross-platform runtime detection and auto-installation
- **Platform Abstraction** (`internal/platform/`): Multi-platform package manager integration
- **MCP Integration** (`mcp/`): Model Context Protocol server exposing LSP as MCP tools

### CLI Command Structure
- **`server`**: Start HTTP gateway server
- **`mcp`**: Start MCP server for AI assistants
- **`install runtime <name|all>`**: Install language runtimes
- **`install servers`**: Install LSP servers
- **`setup all`**: Complete automated setup (installs runtimes, servers, and generates config)
- **`status`**: System status and health
- **`diagnose`**: System diagnostics
- **`verify runtime <name|all>`**: Verify installations
- **`config generate/validate`**: Configuration management

## MCP Integration Deep Dive

### MCP Tool Mappings
The MCP server exposes LSP functionality through these tools:

- **`goto_definition`** → `textDocument/definition`: Navigate to symbol definitions
- **`find_references`** → `textDocument/references`: Find all symbol references
- **`get_hover_info`** → `textDocument/hover`: Get documentation and type info
- **`get_document_symbols`** → `textDocument/documentSymbol`: Extract file symbols
- **`search_workspace_symbols`** → `workspace/symbol`: Search symbols across workspace

### MCP Server Configuration
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

### MCP vs HTTP Gateway Use Cases
- **MCP Server**: AI assistant integration (Claude, GPT), automated code analysis
- **HTTP Gateway**: IDE integration, traditional development tools, custom LSP clients

## Configuration System

### Auto-Generated Configuration Example
```yaml
port: 8080
timeout: 30s
max_concurrent_requests: 100

servers:
  - name: "go-lsp"
    languages: ["go"]
    command: "gopls"
    transport: "stdio"
    settings:
      "gopls": {
        "analyses": {
          "unusedparams": true
        }
      }
  
  - name: "python-lsp"
    languages: ["python"]
    command: "python"
    args: ["-m", "pylsp"]
    transport: "stdio"
    root_markers: ["pyproject.toml", "setup.py"]
    
  - name: "typescript-lsp"
    languages: ["typescript", "javascript"]
    command: "typescript-language-server"
    args: ["--stdio"]
    transport: "stdio"
    root_markers: ["tsconfig.json", "package.json"]
```

### Configuration Auto-Detection
```bash
./bin/lsp-gateway config generate --auto-detect  # Generate config with runtime detection
./bin/lsp-gateway config validate               # Validate configuration
./bin/lsp-gateway status runtimes              # Check detected runtimes
```

## API Usage Examples

### HTTP JSON-RPC Gateway
```bash
# Go to definition
curl -X POST http://localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "textDocument/definition",
    "params": {
      "textDocument": {"uri": "file:///path/to/file.go"},
      "position": {"line": 10, "character": 5}
    }
  }'

# Find references
curl -X POST http://localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "textDocument/references",
    "params": {
      "textDocument": {"uri": "file:///path/to/file.py"},
      "position": {"line": 15, "character": 8},
      "context": {"includeDeclaration": true}
    }
  }'
```

### Supported HTTP Methods
- `textDocument/definition`: Symbol definition lookup
- `textDocument/references`: Reference finding
- `textDocument/documentSymbol`: Document symbol extraction
- `workspace/symbol`: Workspace-wide symbol search
- `textDocument/hover`: Hover information

## Code Quality Standards

### Go Standards (MVP/ALPHA Relaxed)
- Cyclomatic complexity: 15 (relaxed from 10)
- Cognitive complexity: 20 (relaxed from 15)
- Function length: 100 lines (relaxed from 50)
- Line length: 140 characters

### Security Standards
- Comprehensive gosec rules enabled
- Whitelisted development tools: `go`, `make`, `npm`, `python`, `pip`, LSP servers
- File permissions: directories 0750, files 0600

### Architecture Patterns
- **Strategy Pattern**: Platform-specific installers, transport implementations
- **Registry Pattern**: Server and runtime definition management
- **Gateway/Proxy Pattern**: Request routing to language servers
- **Circuit Breaker Pattern**: LSP server resilience

## Testing Infrastructure

### Test Categories
- **Unit** (`tests/unit/`): Fast, isolated component tests
- **Integration** (`tests/integration/`): End-to-end workflow validation
- **Benchmark** (`tests/benchmark/`): Performance measurement
- **Stress** (`tests/stress/`): Reliability under high load
- **Cross-platform** (`tests/crossplatform/`): Platform compatibility

### Test Configurations
Multiple YAML configs in `tests/data/configs/`:
- `simple-lsp-test-config.yaml`: Basic LSP testing
- Language-specific configs for Go, Python, TypeScript, Java

### LSP Validation Testing
- Repository-based testing against real codebases (Kubernetes, Django, VS Code)
- Language server functionality validation
- Performance profiling and coverage analysis

### Repository-Based Testing
```bash
make setup-simple-repos     # Setup test repositories (Kubernetes, Django, VS Code)
make test-lsp-repos         # Validate against real codebases
make simple-status          # Check repository status
make simple-clean           # Clean test repositories
```

## Performance and Benchmarking

### Performance Testing Commands
```bash
make test-integration          # Integration + performance tests
make bench-lsp-validation      # LSP performance benchmarks
make test-lsp-validation-full  # Full LSP validation with performance metrics
```

### Performance Monitoring
- Concurrent request handling validation
- Memory allocation tracking
- Response time measurement
- Circuit breaker performance
- Transport layer efficiency

### Benchmark Configuration
Performance expectations configured in `tests/data/configs/performance-test-config.yaml`:
- Request timeout limits
- Concurrent client limits
- Memory usage thresholds
- Response time expectations

## Troubleshooting and Debugging

### Common Diagnostics
```bash
./bin/lsp-gateway diagnose              # Comprehensive system diagnostics
./bin/lsp-gateway status                # Overall system status
./bin/lsp-gateway status runtimes       # Check runtime installations
./bin/lsp-gateway status servers        # Check language server status
```

### Common Issues and Solutions

#### LSP Server Not Found
```bash
./bin/lsp-gateway install servers       # Install missing language servers
./bin/lsp-gateway verify runtime all    # Verify runtime installations
```

#### Configuration Issues
```bash
./bin/lsp-gateway config validate       # Validate configuration
./bin/lsp-gateway config generate --auto-detect  # Regenerate with auto-detection
```

#### Gateway Connection Issues
```bash
# Check server status and logs
./bin/lsp-gateway diagnose

# Clean rebuild
make clean && make local

# Test basic connectivity
curl -X POST http://localhost:8080/jsonrpc -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":1,"method":"ping"}'
```

#### Performance Issues
```bash
make bench-lsp-validation      # Run performance benchmarks
make test-simple-quick         # Quick performance validation
# Check circuit breaker status in logs
```

## Development Guidelines

### Working with LSP Servers
- LSP servers auto-installed: gopls (Go), pylsp (Python), typescript-language-server, jdtls (Java)
- Configuration auto-generated based on detected runtimes
- Test against real repositories using `make test-lsp-repos`

### Configuration System
- YAML-based with auto-detection capabilities
- Multi-platform server definitions
- Validation with detailed error reporting

### Error Handling Patterns
- Structured errors with recovery suggestions
- Circuit breaking for LSP server failures  
- Platform-specific error handling

## Extension Development

### Adding New Language Servers

1. **Add Server Definition** (`internal/config/servers.go`):
```go
&ServerConfig{
    Name:      "rust-lsp",
    Languages: []string{"rust"},
    Command:   "rust-analyzer",
    Transport: "stdio",
    RootMarkers: []string{"Cargo.toml"},
}
```

2. **Add Runtime Detection** (`internal/setup/rust_detector.go`):
```go
func (d *RustDetector) DetectRuntime() (*RuntimeInfo, error) {
    // Implement rust runtime detection
}
```

3. **Add Platform Strategy** (`internal/platform/rust_strategy.go`):
```go
func (s *RustStrategy) Install() error {
    // Implement rust installation
}
```

4. **Add Test Configuration** (`tests/data/configs/rust-config.yaml`)
5. **Add CLI Command Support** in `internal/cli/constants.go`

### Adding New CLI Commands

1. **Create Command File** (`internal/cli/newcommand.go`):
```go
var newCommandCmd = &cobra.Command{
    Use:   "newcommand",
    Short: "Brief description",
    Long:  `Detailed description with examples`,
    RunE:  runNewCommand,
}

func init() {
    newCommandCmd.Flags().StringVarP(&flag, "flag", "f", "default", "description")
    rootCmd.AddCommand(newCommandCmd)
}
```

2. **Add Constants** (`internal/cli/constants.go`)
3. **Implement Command Logic**
4. **Add Tests** (`tests/unit/internal/cli/newcommand_test.go`)

### Adding New Transport Methods

1. **Implement LSPClient Interface** (`internal/transport/`):
```go
type NewTransport struct {
    // implementation
}

func (t *NewTransport) SendRequest(method string, params interface{}) (interface{}, error) {
    // implement transport logic
}
```

2. **Add Circuit Breaker Support**
3. **Add Transport Tests**
4. **Update Server Configuration Schema**

## Security Best Practices

### Development Security
- Use gosec for vulnerability scanning: `make security`
- Whitelist only necessary development tools in gosec configuration
- File permissions: directories 0750, files 0600
- No sensitive information in configuration files

### Production Security
- Run security analysis before deployment: `make security-full`
- Use HTTPS for HTTP gateway in production
- Implement authentication/authorization for MCP server
- Monitor and log all gateway requests
- Use minimal file permissions for deployed binaries

## Key File Locations
- Main entry: `cmd/lsp-gateway/main.go`
- CLI root: `internal/cli/root.go`
- Gateway logic: `internal/gateway/handlers.go`  
- Configuration: `internal/config/config.go`
- Test framework: `tests/utils/framework/`
- MCP server: `mcp/server.go`
- Transport layer: `internal/transport/`