# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

LSP Gateway is a dual-protocol Language Server Protocol gateway with integrated cache for local development that provides:

- **HTTP JSON-RPC Gateway** at `localhost:8080/jsonrpc` for IDE integration
- **MCP Server** for AI assistant integration (Claude, GPT, etc.)  
- **Simple Cache System** - Sub-millisecond symbol/definition/reference lookups for local development
- **Cross-platform CLI** with essential setup and management commands
- **Auto-Configuration** - automatically detects project languages when no config provided

**Local Development Tool**: Designed for individual developers working locally, not enterprise features.

**Supported Languages**: Go, Python, JavaScript/TypeScript, Java (4 languages)

**Supported LSP Features**: Exactly 6 essential features:

| LSP Method | Description |
|------------|-------------|
| `textDocument/definition` | Go to definition |
| `textDocument/references` | Find all references |
| `textDocument/hover` | Show documentation on hover |
| `textDocument/documentSymbol` | Document outline/symbols |
| `workspace/symbol` | Workspace-wide symbol search |
| `textDocument/completion` | Code completion |

## Requirements

- **Go 1.24.0+** - Required for building LSP Gateway
- **Node.js 18+** - For npm global linking and scripts
- **Platform Support**: Linux, macOS (x64/arm64), Windows (x64)

**Language Servers** (manual installation required):
```bash
go install golang.org/x/tools/gopls@latest                    # Go
pip install python-lsp-server                               # Python
npm install -g typescript-language-server typescript         # TypeScript/JS
# Java: Eclipse JDT Language Server - install manually via Eclipse downloads
```

## Quick Setup

```bash
# 1. Clone and build
git clone <repository-url>
cd lsp-gateway
make local                    # Build and install globally

# 2. Install language servers (optional - for specific languages)
go install golang.org/x/tools/gopls@latest
pip install python-lsp-server
npm install -g typescript-language-server typescript

# 3. Test availability  
lsp-gateway status           # Check which LSP servers are available

# 4. Start using
lsp-gateway mcp              # Auto-detect languages and start MCP server
# OR
lsp-gateway server           # Start HTTP gateway on port 8080
```

## Essential Commands

### Build Commands (Working)
```bash
make local          # Build for current platform + npm global link (most common)
make build          # Build for all platforms 
make clean          # Remove build artifacts
make tidy           # Tidy go modules
make unlink         # Remove npm global link
```

### Quality Commands (Working)
```bash
make format         # Format Go code
make vet            # Run go vet (built-in, may have import warnings)
make quality        # Essential checks (format + vet)
make quality-full   # All checks (format + vet + lint + security)
```

### Testing Commands (Working)
```bash
# Direct Go test commands (recommended)
go test -v ./src/internal/project/...    # Unit tests for language detection
go test -v ./tests/unit/...              # Config unit tests
go test -v ./src/server/cache/...        # Cache system tests

# E2E tests with real GitHub repositories (slow - requires LSP servers)
cd tests/e2e
go test -v -run "TestGoRealClientComprehensiveE2ETestSuite" .
go test -v -run "TestPythonRealClientComprehensiveE2ETestSuite" .
go test -v -run "TestTypeScriptRealClientComprehensiveE2ETestSuite" .
go test -v -run "TestJavaRealClientComprehensiveE2ETestSuite" .
```

### Development Commands
```bash
lsp-gateway server --config config.yaml    # Start HTTP Gateway (port 8080)
lsp-gateway mcp                             # Start MCP Server, auto-detects languages
lsp-gateway mcp --config config.yaml       # Start MCP with explicit config
lsp-gateway status                          # Show LSP server availability + cache status
lsp-gateway test                           # Test LSP server connections

# Cache Management Commands
lsp-gateway cache status                   # Cache status and basic metrics
lsp-gateway cache clear                    # Clear cache contents

# Alternative via npm (after make local):
npm run server                             # Start HTTP Gateway
npm run mcp                                # Start MCP Server
npm run status                             # Show LSP server status
npm run test                               # Test LSP server connections
npm run build                              # Build via make local
npm run clean                              # Clean build artifacts
```

## Configuration

**Complete Configuration Guide**: [docs/configuration.md](docs/configuration.md)

### Cache Configuration (Simplified)
```yaml
cache:
  enabled: true                  # Enable cache (default: true)
  max_memory_mb: 256            # Memory limit in MB (default: 256)
  ttl_hours: 24                 # Cache TTL in hours (default: 24)
  storage_path: ~/.lsp-gateway/cache  # Cache directory
  eviction_policy: "lru"        # LRU eviction (default)
  disk_cache: false             # Optional disk persistence (default: false)
  languages: ["go", "python", "typescript", "java"]  # Languages to cache (uses pylsp for Python)
```

## Usage Examples

### HTTP Gateway Integration
```bash
# Start HTTP gateway (auto-detects languages)
lsp-gateway server

# Health check (includes cache status)
curl http://localhost:8080/health
```

### MCP Server Integration  
```bash
# Auto-detect languages in current directory (recommended)
lsp-gateway mcp
```

**MCP Tools Available**: `goto_definition`, `find_references`, `get_hover_info`, `get_document_symbols`, `search_workspace_symbols`, `get_completion`

### Claude Desktop Integration
Add to your Claude Desktop configuration:
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

### IDE Integration
Point your IDE's LSP client to `http://localhost:8080/jsonrpc` after starting the HTTP gateway.

## Simplified Cache System

### Performance Benefits
- **Sub-millisecond responses** for cached symbols (< 1ms vs 10-100ms LSP calls)
- **Memory-efficient storage** with configurable limits (256MB default)
- **Optional cache** - LSP manager works with or without cache
- **Simple LRU eviction** when memory limits are reached

### Cache Architecture (Simplified)
- **Single LRU cache** with configurable memory limits
- **Optional disk persistence** with simple JSON serialization
- **File modification time checking** for basic invalidation
- **Direct LSP fallback** when cache misses or fails

### Local Development Optimized
The cache system is specifically designed for local development:
- **Simple configuration** with MB/hours instead of complex units
- **Optional operation** - graceful degradation when cache is unavailable
- **Basic invalidation** using file modification times
- **No background processes** - synchronous operation only

## Architecture

### Modular Package Structure
```
src/
├── cmd/lsp-gateway/     # Application entry point
├── server/              # Core server implementations (HTTP, MCP, LSP)
│   ├── lsp_manager.go   # Core LSP orchestration with optional cache
│   ├── gateway.go       # HTTP JSON-RPC gateway
│   ├── mcp_server.go    # MCP server implementation
│   ├── cache/           # Simplified cache system
│   │   ├── manager.go       # Simple cache manager with LRU eviction
│   │   ├── utils.go         # Shared cache utilities
│   │   └── integration.go   # LSP manager integration
│   ├── errors/          # LSP error translation and user-friendly messaging
│   ├── capabilities/    # LSP server capability detection
│   ├── documents/       # Document state management and didOpen notifications
│   ├── process/         # LSP server process lifecycle with graceful shutdown
│   ├── protocol/        # JSON-RPC protocol handling with Content-Length headers
│   └── aggregators/     # Workspace symbol aggregation across multiple clients
├── cli/                 # Command-line interface and commands
├── config/              # Configuration types, loading, and auto-generation
└── internal/            # Internal supporting packages
    ├── common/          # STDIO-safe logging utilities
    ├── project/         # Language detection and project analysis
    ├── security/        # Command validation (active security)
    └── types/          # Shared types
tests/                   # All test code (E2E tests with real LSP servers)
```

### Core Architecture Components

**Core Files:**
- **`src/server/lsp_manager.go`** - LSP client management with optional cache integration
- **`src/server/cache/manager.go`** - Simple LRU cache manager
- **`src/server/errors/translator.go`** - Error translation with method-specific suggestions
- **`src/server/capabilities/detector.go`** - LSP server capability detection
- **`src/server/process/manager.go`** - Process lifecycle with 5s graceful shutdown
- **`src/server/protocol/jsonrpc.go`** - JSON-RPC protocol with LSP Content-Length headers

**Design Patterns:**
- **Interface-Based Design**: Clean interfaces for dependency injection
- **Single Responsibility**: Each module handles one specific concern
- **Optional Cache**: Cache can be disabled without affecting LSP functionality
- **Constructor Pattern**: Use `New*` prefix returning `(*Type, error)`

**Flow Architecture:**
- **HTTP Flow**: HTTP Gateway → LSP Manager → Cache (optional) → LSP Client
- **MCP Flow**: MCP Protocol → LSP Manager → Cache (optional) → LSP Client

## Development Patterns

### Code Conventions
- **Error Handling**: Structured errors with wrapping and graceful degradation
- **Untyped JSON-RPC**: Uses `interface{}` for LSP parameters/results (intentional simplicity)
- **Dependency Injection**: All modules injected via interfaces
- **Optional Cache**: All cache operations handle nil cache gracefully

### **MANDATORY Logging Standards**
**CRITICAL**: Use ONLY the unified logging system from `src/internal/common/logging.go`

#### **Required Usage:**
```go
import "lsp-gateway/src/internal/common"

// Use appropriate logger for context:
common.LSPLogger.Info("LSP server started for %s", language)
common.GatewayLogger.Error("HTTP request failed: %v", err) 
common.CLILogger.Warn("Config file not found, using defaults")
```

#### **Logger Selection by Context:**
- **`common.LSPLogger`** - LSP client management, MCP server operations, language detection
- **`common.GatewayLogger`** - HTTP gateway operations, JSON-RPC processing, web server lifecycle
- **`common.CLILogger`** - CLI commands, configuration loading, user-facing operations

#### **STRICTLY FORBIDDEN:**
```go
// ❌ NEVER USE - Interferes with LSP protocol STDIO communication
fmt.Print*, fmt.Printf, fmt.Println
log.Print*, log.Printf, log.Println
logger := log.New(...) // Custom loggers

// ✅ ALWAYS USE - STDIO-safe, structured logging
common.CLILogger.Info("message")
common.LSPLogger.Error("error: %v", err)
common.GatewayLogger.Debug("debug info")
```

#### **STDIO Safety Requirement:**
All logging MUST use stderr-only to prevent interference with LSP JSON-RPC protocol communication over stdin/stdout.

### Key Design Decisions

**Auto-Configuration** - `src/config/config.go`, `src/internal/project/detector.go`
- MCP mode automatically detects languages when no config provided
- Scans file extensions, project files (go.mod, package.json, etc.)
- Only includes languages with installed/available LSP servers

**Graceful Process Management** - `src/server/process/manager.go`
- 5-second shutdown sequence: `shutdown` → `exit` → 5s timeout → force kill
- Process monitoring distinguishes crashes vs normal shutdown
- Interface-based design with `ShutdownSender` for LSP protocol integration

**Security First** - `src/internal/security/command_validation.go`
- All LSP server commands validated through whitelist approach
- Prevents malicious LSP server execution and shell injection

**Optional Cache Design** - `src/server/cache/manager.go`
- Cache can be nil - all operations handle graceful degradation
- Simple LRU eviction with configurable memory limits
- Direct LSP fallback when cache is unavailable or fails

**Local Focus**: No enterprise features (auth, monitoring, distributed systems)

### LSP Communication
- **STDIO Protocol**: Full LSP JSON-RPC over stdin/stdout with Content-Length headers
- **Capability Detection**: Parses server capabilities from `initialize` response
- **Method Validation**: Checks if server supports requested LSP method before routing
- **Error Recovery**: Enhanced error reporting with user-friendly alternatives
- **Multi-Client Aggregation**: Workspace symbols collected from all active clients in parallel

## Testing

### E2E Testing with Real GitHub Repositories
**Architecture**: E2E tests use real GitHub repositories instead of fixtures for realistic testing.

```bash
# Run comprehensive E2E tests with real LSP servers and GitHub projects
cd tests/e2e
go test -v -run "TestGoRealClientComprehensiveE2ETestSuite" .
go test -v -run "TestPythonRealClientComprehensiveE2ETestSuite" .
go test -v -run "TestTypeScriptRealClientComprehensiveE2ETestSuite" .

# Test all 6 LSP methods sequentially
go test -v -run "TestGoAllLSPMethodsSequential" .

# All E2E tests (requires LSP servers installed)
go test -v ./tests/e2e/...
```

**E2E Test Features:**
- **Real GitHub Projects**: Uses actual open-source repositories
  - Go: `gorilla/mux` (v1.8.0)
  - Python: `psf/requests` (v2.28.2)
  - TypeScript: `sindresorhus/is` (v5.4.1)
  - Java: `spring-projects/spring-petclinic` (v2.7.3)
- **Repository Manager**: `testutils/repo_manager.go` handles cloning and test position management
- **All 6 LSP Methods**: Comprehensive testing of definition, references, hover, documentSymbol, workspace/symbol, completion
- **Real LSP Integration**: Tests with actual LSP servers (gopls, pylsp, typescript-language-server)
- **Server Lifecycle**: Graceful shutdown testing and health monitoring
- **Reproducible**: Fixed commit hashes ensure consistent test environments

### Unit Testing
```bash
go test -v ./src/internal/project/...    # Language detection tests
go test -v ./src/server/cache/...        # Cache system tests
```

**Test-Driven Development:** All E2E tests use real LSP servers and real GitHub repositories - no mocks. Tests validate actual LSP protocol communication in realistic environments.

## Troubleshooting

### Quick Diagnostics
```bash
lsp-gateway status           # Check LSP server availability (fast)
lsp-gateway test            # Test LSP server connections
lsp-gateway cache status    # Check cache status and configuration
make quality                # Validate code format and vet
curl http://localhost:8080/health  # Test HTTP gateway
```

### Common Issues

**Build Problems:**
```bash
make clean && make local    # Clean rebuild and install
make tidy                   # Update Go dependencies
```

**Port Conflicts:**
```bash
lsp-gateway server --port 8081  # Use different port
```

**Missing Language Servers:**
```bash
which gopls                 # Check if language server is installed
lsp-gateway status          # See which servers are available
```

**Cache Issues:**
- Cache is optional - LSP functionality works without cache
- Check `lsp-gateway cache status` for cache diagnostics
- Use `lsp-gateway cache clear` to reset cache state

## Important Constraints

### Language Support
- **Exactly 4 languages**: Go, Python, JavaScript/TypeScript, Java
- **6 LSP features only**: Essential developer productivity features
- **Local development focus**: No authentication/monitoring complexity

### Development Guidelines
- **Modular architecture**: Clean interfaces with single responsibility
- **No auto-installation**: Users must manually install language servers
- **Security active**: Command validation prevents malicious LSP server execution
- **Optional cache**: All functionality works without cache

### **IMPORTANT: Local Developer Tool Identity**
- **Target Users**: Individual developers working locally
- **Use Cases**: IDE integration and AI assistant integration via MCP
- **Scope**: Source code analysis through LSP server integration
- **NO Enterprise Features**: Avoid authentication, monitoring, distributed systems
- **Keep It Simple**: Focus on core LSP-to-HTTP and LSP-to-MCP bridging

## Working with This Codebase

### Start Here for Development:
- **`src/server/lsp_manager.go`** - Core LSP orchestration and client management
- **`src/server/cache/manager.go`** - Simple cache implementation
- **`src/server/process/manager.go`** - Process lifecycle and graceful shutdown
- **`src/server/protocol/jsonrpc.go`** - JSON-RPC protocol handling
- **`src/internal/project/detector.go`** - Language detection logic
- **`src/config/config.go`** - YAML configuration loading with auto-generation

### Module Integration Pattern:
```go
// Example of how modules are integrated
manager := &LSPManager{
    documentManager:     documents.NewLSPDocumentManager(),
    workspaceAggregator: aggregators.NewWorkspaceSymbolAggregator(),
    cache:               nil, // Optional cache - can be nil
}

// Optional cache injection
if cacheConfig.Enabled {
    cache := cache.NewSimpleCache(cacheConfig.MaxMemoryMB)
    manager.SetCache(cache)
}
```

### Architecture Philosophy
The project prioritizes **simplicity, modularity, and reliability**. Auto-configuration eliminates manual setup for MCP mode, graceful process management prevents LSP server crashes, and the modular system focuses on 4 languages with 6 essential LSP features.

Each module has a clear interface and single responsibility, making the codebase easier to understand, test, and extend. STDIO communication is carefully managed to avoid interference with LSP protocol messages.

The optional cache system provides performance benefits for local development without adding complexity or enterprise overhead. The system works reliably with or without cache, maintaining focus on the core LSP functionality.

This LSP Gateway provides a clean, focused solution for local LSP functionality with both HTTP and MCP protocol support, featuring automatic language detection, optional simple cache for improved performance, and robust process management for seamless IDE and AI assistant integration.