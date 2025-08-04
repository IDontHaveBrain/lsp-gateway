# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

LSP Gateway is a dual-protocol Language Server Protocol gateway with integrated SCIP cache for local development that provides:

- **HTTP JSON-RPC Gateway** at `localhost:8080/jsonrpc` for IDE integration
- **MCP Server** for AI assistant integration (Claude, GPT, etc.)  
- **SCIP Cache System** - Sub-millisecond symbol/definition/reference lookups for LLM queries
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

- **Go 1.24.0+** - Required for building LSP Gateway (current: 1.24.5)
- **Node.js 18+** - For npm global linking and scripts
- **Platform Support**: Linux, macOS (x64/arm64), Windows (x64)

**Language Servers** (manual installation required):
```bash
go install golang.org/x/tools/gopls@latest                    # Go
npm install -g pyright                                        # Python  
npm install -g typescript-language-server typescript         # TypeScript/JS
# Java: Eclipse JDT Language Server - install manually via Eclipse downloads
```

## Quick Setup

**5-minute setup for immediate use:**

```bash
# 1. Clone and build
git clone <repository-url>
cd lsp-gateway
make local                    # Build and install globally

# Alternative: Install via npm (if available)
npm install -g lsp-gateway

# 2. Install language servers (optional - for specific languages)
go install golang.org/x/tools/gopls@latest
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

# Cache-specific build commands
make cache-verify   # Verify cache dependencies are available
make cache-test     # Run cache tests
make cache-clean    # Clean cache data
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
lsp-gateway test                           # Test LSP server connections + cache performance

# Cache Management Commands
lsp-gateway cache status                   # Cache status and configuration
lsp-gateway cache health                   # Detailed cache health diagnostics  
lsp-gateway cache stats                    # Cache performance statistics
lsp-gateway cache clear                    # Clear cache contents
lsp-gateway cache index                    # Proactively index files for faster responses

# Alternative via npm (after make local):
npm run server                             # Start HTTP Gateway
npm run mcp                                # Start MCP Server
npm run status                             # Show LSP server status
npm run test                               # Test LSP server connections
npm run build                              # Build via make local
npm run clean                              # Clean build artifacts
npm run cache-verify                       # Verify cache dependencies
npm run cache-test                         # Run cache tests  
npm run cache-clean                        # Clean cache data
```

## Configuration

### YAML Configuration Format
```yaml
# config.yaml - Complete configuration with SCIP cache
servers:
  go:
    command: "gopls"
    args: ["serve"]
    working_dir: ""
    initialization_options: {}
  python:
    command: "pyright-langserver"
    args: ["--stdio"]
    working_dir: ""
    initialization_options: {}
  typescript:
    command: "typescript-language-server"
    args: ["--stdio"]
    working_dir: ""
    initialization_options: {}
  java:
    command: "jdtls"
    args: []
    working_dir: ""
    initialization_options: {}

# SCIP Cache Configuration (enabled by default)
cache:
  enabled: true                               # Always enabled for optimal performance
  storage_path: ".lsp-gateway/scip-cache"     # Cache storage (auto-generates project-specific paths)
  max_memory_mb: 256                          # Memory limit in MB
  ttl: "24h"                                  # Cache time-to-live (24h for daily dev workflow)
  languages: ["go", "python", "typescript", "java"]  # Cached languages
  background_index: true                      # Background cache optimization
  health_check_interval: "5m"                # Health monitoring frequency
```

## Usage Examples

### HTTP Gateway Integration
```bash
# Start HTTP gateway
lsp-gateway server --config config.yaml

# Test with curl
curl -X POST http://localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "textDocument/hover",
    "params": {
      "textDocument": {"uri": "file:///path/to/file.go"},
      "position": {"line": 10, "character": 5}
    }
  }'

# Health check (includes cache status)
curl http://localhost:8080/health

# Cache monitoring endpoints
curl http://localhost:8080/cache/stats    # Cache statistics
curl http://localhost:8080/cache/health   # Cache health status
```

### MCP Server Integration  
```bash
# Auto-detect languages in current directory
lsp-gateway mcp

# Use with explicit config
lsp-gateway mcp --config config.yaml
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

## SCIP Cache System

### Performance Benefits
- **Sub-millisecond responses** for cached symbols (< 1ms vs 10-100ms LSP calls)
- **90%+ cache hit rates** for typical LLM usage patterns
- **Memory-efficient storage** with configurable limits (256MB default)
- **Intelligent invalidation** based on file changes and dependencies

### Cache Architecture
- **Three-tier storage**: Hot memory cache, warm LRU cache, cold disk storage
- **Project isolation**: Auto-generates unique cache paths per project to prevent conflicts
- **Background indexing**: Automatic symbol extraction and relationship building
- **Smart invalidation**: File system monitoring with cascade dependency updates
- **Health monitoring**: Continuous performance and health tracking

### LLM Optimization
The SCIP cache is specifically optimized for AI assistant usage patterns:
- **Predictive caching**: Pre-warms adjacent code positions based on LLM exploration patterns
- **Query normalization**: Improves cache hit rates for symbol searches
- **Performance reporting**: Every MCP response includes cache performance metrics
- **Fault resilience**: Graceful degradation to LSP servers when cache fails

## Architecture

### Modular Package Structure
```
src/
├── cmd/lsp-gateway/     # Application entry point
├── server/              # Core server implementations (HTTP, MCP, LSP)
│   ├── lsp_manager.go   # Core LSP orchestration with mandatory SCIP cache
│   ├── gateway.go       # HTTP JSON-RPC gateway with cache monitoring endpoints
│   ├── mcp_server.go    # MCP server implementation with cache optimization
│   ├── cache/           # SCIP cache system
│   │   ├── interfaces.go    # Cache interfaces and types
│   │   ├── manager.go       # Core cache manager with lifecycle management
│   │   ├── storage.go       # Three-tier storage (memory/LRU/disk)
│   │   ├── query.go         # Sub-millisecond symbol query engine
│   │   └── invalidation.go  # Smart cache invalidation with file monitoring
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
tests/                   # All test code (E2E tests with real LSP servers and cache)
```

### Core Architecture Components

**Core Files:**
- **`src/server/lsp_manager.go`** - LSP client management and orchestration
- **`src/server/errors/translator.go`** - Error translation with method-specific suggestions
- **`src/server/capabilities/detector.go`** - LSP server capability detection
- **`src/server/documents/manager.go`** - Document state management
- **`src/server/process/manager.go`** - Process lifecycle with 5s graceful shutdown
- **`src/server/protocol/jsonrpc.go`** - JSON-RPC protocol with LSP Content-Length headers
- **`src/server/aggregators/workspace_symbol.go`** - Multi-client workspace symbol aggregation

**Design Patterns:**
- **Interface-Based Design**: Each module has clean interfaces for dependency injection
- **Single Responsibility**: Each module handles one specific concern
- **Manager Pattern**: All managers use `Initialize()`, `Start()`, `Stop()` methods
- **Constructor Pattern**: Use `New*` prefix returning `(*Type, error)`

**Flow Architecture:**
- **HTTP Flow**: HTTP Gateway → LSP Manager → Module Interfaces → LSP Client
- **MCP Flow**: MCP Protocol → LSP Manager → Module Interfaces → LSP Client

## Development Patterns

### Code Conventions
- **Error Handling**: Structured errors with wrapping and graceful degradation
- **Untyped JSON-RPC**: Uses `interface{}` for LSP parameters/results (intentional simplicity)
- **Dependency Injection**: All modules injected via interfaces

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

**Critical**: LSP servers communicate via stdin/stdout. Any logging to stdout breaks the protocol.

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

**Modular Interface Design** - All modules use dependency injection:
```go
type DocumentManager interface { ... }
type ProcessManager interface { ... } 
type JSONRPCProtocol interface { ... }
type WorkspaceSymbolAggregator interface { ... }
type ErrorTranslator interface { ... }
type CapabilityDetector interface { ... }
```

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
- **Real LSP Integration**: Tests with actual LSP servers (gopls, pyright, typescript-language-server)
- **Server Lifecycle**: Graceful shutdown testing and health monitoring
- **Reproducible**: Fixed commit hashes ensure consistent test environments

**Repository-Based Testing Architecture:**
```go
// testutils/repo_manager.go provides:
type TestRepository struct {
    Language   string
    URL        string
    CommitHash string
    TestFiles  []TestFile // Predefined test positions
}

// Automatic repository setup and cleanup
repoManager := testutils.NewRepoManager(tempDir)
repoDir, err := repoManager.SetupRepository("go")
```

### Unit Testing
```bash
go test -v ./src/internal/project/...    # Language detection tests
```

**Test-Driven Development:** All E2E tests use real LSP servers and real GitHub repositories - no mocks. Tests validate actual LSP protocol communication in realistic environments.

## Troubleshooting

### Quick Diagnostics
```bash
lsp-gateway status           # Check LSP server availability (fast)
lsp-gateway test            # Test LSP server connections
lsp-gateway cache status    # Check cache status and configuration
lsp-gateway cache health    # Detailed cache health diagnostics
make quality                # Validate code format and vet
curl http://localhost:8080/health  # Test HTTP gateway (includes cache status)
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

**Process Issues:**
- LSP servers auto-terminate after 5s timeout during graceful shutdown
- Check logs in stderr for STDIO-safe error messages
- Use `lsp-gateway status` for non-destructive server checks

## Important Constraints

### Language Support
- **Exactly 4 languages**: Go, Python, JavaScript/TypeScript, Java
- **6 LSP features only**: Essential developer productivity features
- **Local development focus**: No authentication/monitoring complexity

### Development Guidelines
- **Modular architecture**: 7 focused modules with clear interfaces
- **Clean separation**: Each module has single responsibility
- **No auto-installation**: Users must manually install language servers
- **Security active**: Command validation prevents malicious LSP server execution

### **IMPORTANT: Local Developer Tool Identity**
- **Target Users**: Individual developers working locally
- **Use Cases**: IDE integration and AI assistant integration via MCP
- **Scope**: Source code analysis through LSP server integration
- **NO Enterprise Features**: Avoid authentication, monitoring, distributed systems
- **Keep It Simple**: Focus on core LSP-to-HTTP and LSP-to-MCP bridging

## Working with This Codebase

### Start Here for Development:
- **`src/server/lsp_manager.go`** - Core LSP orchestration and client management
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
}

client := &StdioClient{
    processManager:   process.NewLSPProcessManager(),
    jsonrpcProtocol:  protocol.NewLSPJSONRPCProtocol(language),
    errorTranslator:  errors.NewLSPErrorTranslator(),
    capDetector:      capabilities.NewLSPCapabilityDetector(),
}
```

### Architecture Philosophy
The project prioritizes **simplicity, modularity, and reliability**. Auto-configuration eliminates manual setup for MCP mode, graceful process management prevents LSP server crashes, and the modular system focuses on 4 languages with 6 essential LSP features.

Each module has a clear interface and single responsibility, making the codebase easier to understand, test, and extend. STDIO communication is carefully managed to avoid interference with LSP protocol messages.

This LSP Gateway provides a clean, focused solution for local LSP functionality with both HTTP and MCP protocol support, featuring automatic language detection, mandatory SCIP cache for sub-millisecond responses, and robust process management for seamless IDE and AI assistant integration.