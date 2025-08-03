# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

LSP Gateway is a dual-protocol Language Server Protocol gateway for local development that provides:

- **HTTP JSON-RPC Gateway** at `localhost:8080/jsonrpc` for IDE integration
- **MCP Server** for AI assistant integration (Claude, GPT, etc.)  
- **Cross-platform CLI** with essential setup and management commands
- **Auto-Configuration** - automatically detects project languages when no config provided

**Supported Languages**: Go, Python, JavaScript/TypeScript, Java (4 languages)

**Supported LSP Features**: Exactly 6 essential features - `textDocument/definition`, `textDocument/references`, `textDocument/hover`, `textDocument/documentSymbol`, `workspace/symbol`, `textDocument/completion`

## Essential Commands

### Build Commands
```bash
make local          # Build for current platform + npm global link (most common)
make build          # Build for all platforms 
make clean          # Remove build artifacts
make tidy           # Tidy go modules
```

### Quality Commands
```bash
make format         # Format Go code
make vet            # Run go vet (built-in, may have import warnings)
make quality        # Essential checks (format + vet)
```

**Note**: `make lint`, `make security`, `make test` reference outdated paths and are not currently functional.

### Development Commands
```bash
lsp-gateway server --config config.yaml    # Start HTTP Gateway (port 8080)
lsp-gateway mcp                             # Start MCP Server, auto-detects languages
lsp-gateway mcp --config config.yaml       # Start MCP with explicit config
lsp-gateway status                          # Show LSP server availability (fast)
lsp-gateway test                           # Test LSP server connections
```

## Current Architecture

### Core Package Architecture
Clean package organization under `src/`:

- **`src/server/lsp_manager.go`** - LSP client management with graceful shutdown and process lifecycle
- **`src/server/mcp_server.go`** - MCP server implementation for AI assistant integration
- **`src/server/gateway.go`** - HTTP JSON-RPC gateway with clean request handling
- **`src/cli/commands.go`** - CLI command implementations and server lifecycle
- **`src/config/config.go`** - YAML configuration with auto-generation and language detection

**3-Layer Flow**: HTTP Gateway → LSP Manager → LSP Client  
**MCP Flow**: MCP Protocol → LSP Manager → LSP Client

### Package Structure
```
src/
├── cmd/lsp-gateway/     # Application entry point
├── server/              # Core server implementations (HTTP, MCP, LSP)
├── cli/                 # Command-line interface and commands
├── config/              # Configuration types, loading, and auto-generation
└── internal/            # Internal supporting packages
    ├── common/          # STDIO-safe logging utilities
    ├── models/lsp/      # LSP protocol definitions (test validation)
    ├── project/         # Language detection and project analysis
    ├── security/        # Command validation (active security)
    └── types/          # Shared types
tests/                   # All test code (E2E tests)
```

### Key Internal Components
- **`src/internal/common/logging.go`** - STDIO-safe logging with levels
- **`src/internal/project/detector.go`** - Automatic language detection based on file extensions and project structure
- **`src/internal/security/command_validation.go`** - LSP server command validation
- **`src/internal/models/lsp/protocol.go`** - LSP protocol definitions for validation

## Project-Specific Development Patterns

### Code Conventions
- **Manager Pattern**: All managers use `Initialize()`, `Start()`, `Stop()` methods
- **Constructor Pattern**: Use `New*` prefix returning `(*Type, error)`
- **Error Handling**: Structured errors with wrapping and graceful degradation
- **Untyped JSON-RPC**: Uses `interface{}` for LSP parameters/results (intentional simplicity)
- **Safe Logging**: Use `common.LSPLogger`, `common.GatewayLogger`, `common.CLILogger` for STDIO-safe output

### Key Design Decisions
- **Auto-Configuration**: MCP mode automatically detects languages in current directory when no config provided
- **Graceful Process Management**: LSP servers follow proper shutdown sequence (`shutdown` → `exit` → 5s timeout)
- **Security First**: All LSP server commands validated through `internal/security/command_validation.go`
- **Local Focus**: No enterprise features (auth, monitoring, distributed systems)
- **STDIO Safety**: All logging uses stderr-only to avoid LSP protocol interference

### Configuration Approach
- **YAML-based**: Simple configuration with language server defaults
- **Auto-Detection**: Intelligent language detection from file extensions and project structure in MCP mode
- **Dynamic Generation**: `config.GenerateAutoConfig()` creates configs for detected languages
- **Default Servers**: `gopls serve`, `pylsp`, `typescript-language-server --stdio`, `jdtls`

### LSP Communication
- **STDIO Protocol**: Full LSP JSON-RPC over stdin/stdout with Content-Length headers
- **Graceful Shutdown**: 5-second timeout for proper LSP shutdown sequence before force termination
- **Process Management**: Manual process control using `exec.Command` (not `exec.CommandContext`)
- **Error Recovery**: Enhanced error reporting distinguishing crashes vs initialization failures

## Important Constraints

### Language Support
- **Exactly 4 languages**: Go, Python, JavaScript/TypeScript, Java
- **6 LSP features only**: Essential developer productivity features
- **Local development focus**: No authentication/monitoring complexity

### Development Guidelines
- **Simplified architecture**: All core functionality in `src/` package
- **Clean separation**: `cmd/` (entry), `server/` (core), `cli/` (commands), `internal/` (support)
- **No auto-installation**: Users must manually install language servers
- **Security active**: Command validation prevents malicious LSP server execution

### **IMPORTANT: Local Developer Tool Identity**
- **Target Users**: Individual developers working locally
- **Use Cases**: IDE integration and AI assistant integration via MCP
- **Scope**: Source code analysis through LSP server integration
- **NO Enterprise Features**: Avoid authentication, monitoring, distributed systems
- **Keep It Simple**: Focus on core LSP-to-HTTP and LSP-to-MCP bridging

## Key Files for Reference

### Core Server Package
- **`src/server/lsp_manager.go`** - LSP client management with graceful shutdown and process lifecycle
- **`src/server/mcp_server.go`** - MCP server for AI assistant integration with auto-configuration
- **`src/server/gateway.go`** - HTTP JSON-RPC request processing

### CLI and Configuration
- **`src/cli/commands.go`** - Server lifecycle and CLI commands with non-destructive status checks
- **`src/config/config.go`** - YAML configuration with auto-generation and language detection

### Supporting Infrastructure
- **`src/internal/project/detector.go`** - Automatic language detection based on file extensions and project structure
- **`src/internal/common/logging.go`** - STDIO-safe logging utilities with levels
- **`src/internal/security/command_validation.go`** - Command validation for security

### Entry Point and Build
- **`src/cmd/lsp-gateway/main.go`** - Application entry point
- **`Makefile`** - Build automation with working targets (`make local`, `make build`, `make quality`)
- **`package.json`** - NPM integration for global command linking

## Development Workflow

1. **Setup**: `make tidy && make local` to build and install globally
2. **Development**: Use working make targets for validation (`make quality`)
3. **Testing**: Manual testing with `lsp-gateway status` and `lsp-gateway test`

### Server Testing
```bash
# Test server availability (fast, non-destructive)
lsp-gateway status                        # Check LSP server availability

# Test functionality
lsp-gateway test --config config.yaml    # Test LSP server connections

# Start servers
lsp-gateway server --config config.yaml  # HTTP Gateway (port 8080)
lsp-gateway mcp                          # MCP Server with auto-detection
lsp-gateway mcp --config config.yaml    # MCP Server with explicit config

# Health check
curl http://localhost:8080/health
```

### Working Make Targets
```bash
make local           # Build and npm link (most common)
make build          # Cross-platform build
make clean          # Clean artifacts  
make tidy           # Go mod tidy
make format         # Go fmt
make vet            # Go vet (may show import warnings)
make quality        # Format + vet
```

## Auto-Configuration Features

### MCP Mode Auto-Detection
When `lsp-gateway mcp` is run without `--config`, it automatically:
1. **Scans current directory** for language-specific files and project structures
2. **Detects languages** based on file extensions (`.go`, `.py`, `.js/.jsx`, `.ts/.tsx`, `.java`)
3. **Analyzes project files** (`go.mod`, `package.json`, `requirements.txt`, `tsconfig.json`, `pom.xml`)
4. **Checks LSP server availability** using `exec.LookPath` and security validation
5. **Generates configuration** for available languages only
6. **Starts LSP servers** for detected languages

### Detection Examples
```bash
cd go-project && lsp-gateway mcp
# 🔍 Auto-detecting languages: [go]

cd full-stack-project && lsp-gateway mcp  
# 🔍 Auto-detecting languages: [typescript javascript python go]

lsp-gateway mcp --config config.yaml
# Uses explicit configuration (no auto-detection)
```

## Recent Improvements

### Process Management Fixes
- **Resolved "signal: killed" crashes**: Fixed aggressive process termination
- **Proper LSP shutdown**: Implements `shutdown` → `exit` → 5s timeout sequence
- **Status command safety**: No longer starts/stops servers for status checks
- **Graceful termination**: Uses `exec.Command` instead of `exec.CommandContext` for manual control

### Enhanced Configuration
- **Auto-generation**: `config.GenerateAutoConfig()` creates configs for detected languages
- **Language detection**: `project.DetectLanguages()` and `project.GetAvailableLanguages()`
- **Availability checking**: `project.IsLSPServerAvailable()` validates server installation

## Current System Status

- **Build System**: Functional with core targets (`make local`, `make build`, `make quality`)
- **LSP Communication**: All 4 language servers working (Go, Python, JavaScript/TypeScript) when installed
- **Auto-Configuration**: MCP mode automatically detects and configures available languages
- **Process Management**: Stable with proper shutdown sequences and error handling
- **Security**: Command validation active, prevents malicious LSP server execution

### Working with This Codebase
- **Start Here**: `src/server/lsp_manager.go` - contains LSP client management and graceful shutdown
- **Auto-Detection**: `src/internal/project/detector.go` - language detection logic
- **HTTP Gateway**: `src/server/gateway.go` - handles JSON-RPC over HTTP
- **MCP Integration**: `src/server/mcp_server.go` - AI assistant protocol with auto-configuration
- **CLI Commands**: `src/cli/commands.go` - server lifecycle and command implementations
- **Configuration**: `src/config/config.go` - YAML loading with auto-generation

### Architecture Philosophy
The project prioritizes simplicity and reliability. Auto-configuration eliminates manual setup for MCP mode, graceful process management prevents LSP server crashes, and the entire system focuses on 4 languages with 6 essential LSP features. STDIO communication is carefully managed to avoid interference with LSP protocol messages.

This LSP Gateway provides a clean, focused solution for local LSP functionality with both HTTP and MCP protocol support, featuring automatic language detection and robust process management for seamless IDE and AI assistant integration.