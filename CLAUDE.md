# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Quick Reference

```bash
# Most common workflow
make local                     # Build and install globally
lsp-gateway mcp               # Run MCP server (auto-detects languages)

# Quality checks before commit
make quality                   # Format + vet
make quality-full             # Format + vet + lint + security
go test -v ./tests/unit/...   # Run unit tests

# Debugging
export LSP_GATEWAY_DEBUG=true # Enable debug logging
lsp-gateway status            # Check LSP server availability
```

## Project Overview

LSP Gateway is a dual-protocol Language Server Protocol gateway for local development:
- **HTTP JSON-RPC Gateway** at `localhost:8080/jsonrpc` for IDE integration
- **MCP Server** for AI assistant integration (Claude, GPT, etc.)  
- **SCIP Cache System** - Sub-millisecond lookups with LRU eviction via SCIP protocol
- **Auto-Configuration** - Detects project languages and available LSP servers

**Supported Languages**: Go, Python, JavaScript, TypeScript, Java (5 languages)
**Supported LSP Methods**: 6 core features - definition, references, hover, documentSymbol, workspace/symbol, completion

## Essential Commands

### Build & Install
```bash
make local          # Build + npm global link (most common)
make build          # Build for all platforms  
make clean          # Clean build artifacts
make tidy           # Tidy go modules
make unlink         # Remove npm global link
```

### Quality & Testing
```bash
# Quality checks
make format         # Format Go code
make vet            # Run go vet
make quality        # format + vet (essential)
make quality-full   # format + vet + lint + security

# Testing (uses real GitHub repositories)
go test -v ./src/internal/project/...    # Language detection tests
go test -v ./tests/unit/...              # Unit tests (includes cache tests)

# E2E tests with real LSP servers and GitHub repos
cd tests/e2e
go test -v -run "TestGoRealClientComprehensiveE2ETestSuite" .
go test -v -run "TestPythonRealClientComprehensiveE2ETestSuite" .
go test -v -run "TestJavaScriptRealClientComprehensiveE2ETestSuite" .
go test -v -run "TestTypeScriptRealClientComprehensiveE2ETestSuite" .
go test -v -run "TestJavaRealClientComprehensiveE2ETestSuite" .
```

### Runtime Commands
```bash
# Server operations
lsp-gateway server --config config.yaml    # HTTP Gateway
lsp-gateway mcp                             # MCP Server (auto-detects languages)
lsp-gateway status                          # Show LSP server availability
lsp-gateway test                           # Test LSP connections

# LSP server installation
lsp-gateway install all                    # Install all language servers
lsp-gateway install go                     # Install specific language server
lsp-gateway install python
lsp-gateway install javascript
lsp-gateway install typescript  
lsp-gateway install java

# Cache management
lsp-gateway cache status                   # Cache metrics
lsp-gateway cache clear                    # Clear cache
make cache-clean                          # Clean cache data
```

## Architecture

### Core Components
```
src/
├── server/                    # Core server implementations
│   ├── lsp_manager.go        # LSP orchestration with SCIP cache interface
│   ├── gateway.go            # HTTP JSON-RPC gateway  
│   ├── mcp_server.go         # MCP server implementation (Normal/Enhanced modes)
│   ├── cache/                # SCIP-based LRU cache system
│   │   ├── manager.go        # SCIPCacheManager (256MB default)
│   │   ├── utils.go          # Cache utilities
│   │   └── simple_interface.go # SimpleCache interface
│   ├── scip/                 # SCIP protocol implementation
│   ├── documents/            # Document management for LSP
│   ├── aggregators/          # Workspace symbol aggregation
│   ├── capabilities/         # LSP capability detection
│   ├── process/              # LSP server lifecycle (5s graceful shutdown)
│   ├── protocol/             # JSON-RPC with Content-Length headers
│   └── errors/               # Error translation with user-friendly messages
├── cli/                      # Command-line interface
├── config/                   # YAML config loading and auto-detection
└── internal/
    ├── installer/            # LSP server installers
    ├── project/              # Language detection
    ├── security/             # Command validation whitelist
    ├── common/               # STDIO-safe logging (CRITICAL)
    ├── models/               # LSP protocol definitions
    ├── types/                # Shared types (MCP Enhanced mode)
    └── version/              # Version management
```

### Key Design Patterns

**Module Integration**:
```go
manager := &LSPManager{
    documentManager:     documents.NewLSPDocumentManager(),
    workspaceAggregator: aggregators.NewWorkspaceSymbolAggregator(),
    scipCache:           nil, // Interface-based, handles nil gracefully
}

if cacheConfig.Enabled {
    scipCache, err := cache.NewSCIPCacheManager(cacheConfig)
    manager.SetCache(scipCache) // Runtime cache injection
}
```

**Constructor Pattern**: Use `New*` prefix returning `(*Type, error)`

### Critical: STDIO-Safe Logging

**MANDATORY**: Use ONLY `src/internal/common/logging.go` loggers:

```go
import "lsp-gateway/src/internal/common"

// Context-specific loggers
common.LSPLogger.Info("message")        // LSP/MCP operations
common.GatewayLogger.Error("error")     // HTTP gateway  
common.CLILogger.Warn("warning")        // CLI commands

// NEVER USE (breaks LSP protocol):
fmt.Print*, log.Print*, log.New()       // ❌ Uses stdout
```

## Testing Strategy

**E2E Tests Use Real GitHub Repositories**:
- Go: `gorilla/mux` (v1.8.0)
- Python: `psf/requests` (v2.28.1)  
- JavaScript: `ramda/ramda` (v0.31.3)
- TypeScript: `sindresorhus/is` (v5.4.1)
- Java: `spring-projects/spring-petclinic` (main branch)

Repository manager (`tests/e2e/testutils/repo_manager.go`) handles cloning and test positions.

## Configuration

### Auto-Detection (Recommended)
```bash
lsp-gateway mcp    # Scans for go.mod, package.json, *.py, pom.xml
```

### Manual Configuration
```yaml
servers:
  go:
    command: "gopls"
    args: ["serve"]
    
cache:
  enabled: true
  max_memory_mb: 256
  ttl_hours: 24
  storage_path: ~/.lsp-gateway/cache
  eviction_policy: "lru"
```

## Key Implementation Details

**Language Detection** (`src/internal/project/detector.go`):
- Scans file extensions and project files
- Only includes languages with available LSP servers
- Used by MCP mode for auto-configuration

**Process Management** (`src/server/process/manager.go`):
- 5-second graceful shutdown sequence
- Distinguishes crashes vs normal shutdown
- Interface-based with `ShutdownSender` for LSP integration

**Cache System** (`src/server/cache/manager.go`):
- SCIP-based LRU cache with configurable memory limits  
- Interface-based design via SCIPCache
- Optional disk persistence with JSON serialization
- File modification time checking for invalidation
- All operations handle nil scipCache gracefully

**Security** (`src/internal/security/command_validation.go`):
- Whitelist-based LSP command validation
- Prevents shell injection and malicious execution

## Development Guidelines

1. **Local Focus**: No enterprise features (auth, monitoring, distributed systems)
2. **Error Handling**: Structured errors with wrapping (`fmt.Errorf("context: %w", err)`)
3. **Untyped JSON-RPC**: Uses `interface{}` for LSP params/results (intentional simplicity)
4. **Cache Design**: SCIP interface-based, cache effectively required for HTTP/MCP modes
5. **STDIO Safety**: All logging must use stderr-only loggers from `src/internal/common/logging.go`

### MCP Server Modes
- **Normal Mode**: Standard MCP protocol implementation
- **Enhanced Mode**: Adds extra tools for AI assistants (configured via `--mode enhanced`)

### Interface Patterns
- **ShutdownSender**: Process lifecycle management (`src/server/process/manager.go:32-35`)
- **SimpleCache**: Cache abstraction (`src/server/cache/simple_interface.go:10-34`)
- **LanguageInstaller**: LSP installer abstraction (`src/internal/installer/interfaces.go:8-29`)

## Common Development Tasks

### Adding a New LSP Method
1. Update capability detection in `src/server/capabilities/detector.go`
2. Add error translation in `src/server/errors/translator.go`
3. Update MCP tools in `src/server/mcp_server.go`
4. Add E2E test in `tests/e2e/`

### Debugging LSP Communication
```bash
# Enable debug logging
export LSP_GATEWAY_DEBUG=true
lsp-gateway server

# Check LSP server directly
gopls serve                    # Test LSP server standalone
```

### Performance Profiling
```bash
# Run with profiling
go test -bench=. -cpuprofile=cpu.prof ./src/server/cache/...
go tool pprof cpu.prof
```

## Important Constraints

- **Exactly 5 languages**: Go, Python, JavaScript, TypeScript, Java
- **6 LSP methods only**: Core developer productivity features
- **Manual LSP server installation**: Use `lsp-gateway install` helpers
- **Local development only**: Not designed for production/enterprise use
- **SCIP cache**: LRU eviction, no distributed features

## Common Gotchas

1. **STDIO Logging**: Never use `fmt.Print*` or `log.Print*` - breaks LSP protocol. Use `common.LSPLogger`, `common.GatewayLogger`, or `common.CLILogger` only.
2. **Cache Nil Checks**: Always check `if m.scipCache != nil` before cache operations - cache is optional at interface level but effectively required for HTTP/MCP modes.
3. **Test Repositories**: E2E tests clone real GitHub repos. First run may be slow due to cloning.
4. **LSP Server Installation**: Must manually install LSP servers before use (`lsp-gateway install all`).
5. **Process Management**: LSP servers have 5-second graceful shutdown. Don't force-kill processes.