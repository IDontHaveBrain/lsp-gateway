# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Quick Reference

```bash
# Prerequisites
go version   # Requires Go 1.24.0+ (toolchain: go1.24.5)
node --version  # Requires Node.js 18+

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
- **MCP Server** for AI assistant integration (Claude, GPT, etc.) - Enhanced mode with 512MB cache, 1hr TTL
- **SCIP Cache System** - Sub-millisecond lookups with LRU eviction, dual ID format support
- **Project Intelligence** - Language detection, package info extraction, workspace scanning
- **Auto-Configuration** - Detects go.mod, package.json, *.py, pom.xml, etc.

**Supported Languages**: Go, Python, JavaScript, TypeScript, Java (exactly 5)
**Supported LSP Methods**: 6 methods - definition, references, hover, documentSymbol, workspace/symbol, completion
**Platform Support**: Linux, macOS (x64/arm64), Windows (x64)

## Essential Commands

### LSP Server Setup (Required)
```bash
# Install LSP servers before first use
lsp-gateway install all                   # Install all language servers

# Or manually:
go install golang.org/x/tools/gopls@latest
pip install python-lsp-server
npm install -g typescript-language-server typescript
```

### Build & Install
```bash
make local          # Build + npm global link (most common)
make build          # Build for all platforms  
make clean          # Clean build artifacts
make tidy           # Tidy go modules
make deps           # Download dependencies

# Platform-specific builds
make linux          # Build for Linux x64
make windows        # Build for Windows x64
make macos          # Build for macOS x64
make macos-arm64    # Build for macOS ARM64
```

### Quality & Testing
```bash
# Quality checks
make quality        # format + vet (essential)
make quality-full   # format + vet + lint + security

# Testing (uses real GitHub repositories)
go test -v ./tests/unit/...              # Unit tests
go test -v ./tests/integration/...       # Integration tests
make test-unit                           # Unit tests with fallback
make test-e2e                            # All E2E tests
make cache-test                          # Cache-specific tests
```

### Runtime Commands
```bash
# Server operations
lsp-gateway server --config config.yaml    # HTTP Gateway
lsp-gateway mcp                            # MCP Server (auto-detects languages)
lsp-gateway status                         # Show LSP server availability
lsp-gateway test                           # Test LSP connections

# LSP server installation
lsp-gateway install all                    # Install all language servers
lsp-gateway install go                     # Install specific language server

# Cache management
lsp-gateway cache info                    # Show cache information and statistics
lsp-gateway cache clear                   # Clear all cached entries
lsp-gateway cache index                   # Proactively index files for cache
```

## Architecture

### Core Components
```
src/
├── server/                    # Core server implementations
│   ├── lsp_manager.go        # LSP orchestration with SCIP cache interface
│   ├── gateway.go            # HTTP JSON-RPC gateway (forces cache enabled)
│   ├── mcp_server.go         # MCP server (always enhanced, 512MB cache)
│   ├── mcp_tools.go          # MCP tool implementations (4 tools)
│   ├── cache/                # SCIP-based LRU cache system
│   │   ├── manager.go        # SCIPCacheManager with dual ID formats
│   │   ├── indexer.go        # Background file indexing
│   │   └── workspace.go      # Workspace operations
│   ├── scip/                 # SCIP protocol implementation
│   │   └── simple_storage.go # In-memory SCIP storage
│   ├── process/              # LSP server lifecycle (5s graceful shutdown)
│   └── protocol/             # JSON-RPC with Content-Length headers
├── cli/                      # Command-line interface
│   ├── commands.go           # Main command registry
│   ├── server_commands.go    # Server, MCP, status, test commands
│   ├── install_commands.go   # LSP server installers
│   └── cache_commands.go     # Cache info, clear, index commands
├── config/                   # YAML config loading and auto-detection
└── internal/
    ├── installer/            # LSP server installers for 5 languages
    ├── project/              # Project intelligence system
    │   ├── detector.go       # Language detection with confidence scoring
    │   ├── package_info.go   # Extract metadata from package files
    │   └── workspace_scanner.go # 3-level depth source file scanning
    ├── constants/            # System constants and limits
    ├── errors/               # Structured error types
    ├── common/               # STDIO-safe logging (CRITICAL)
    └── types/                # Shared types and pattern matching
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

### Default Configuration (~/.lsp-gateway/config.yaml)
```yaml
cache:
  enabled: true                           # Always enabled
  storage_path: ~/.lsp-gateway/scip-cache  # Cache storage directory
  max_memory_mb: 512                      # Default memory limit
  ttl_hours: 24                           # MCP mode overrides to 1hr
  background_index: true                  # Auto-index on startup
  health_check_minutes: 5                 # File modification checking
  eviction_policy: "lru"                  # LRU with timestamps
  disk_cache: true                        # JSON persistence

servers:
  go:
    command: "gopls"
    args: ["serve"]
```

## Key Implementation Details

**System Constants** (`src/internal/constants/constants.go`):
- Cache defaults: 512MB memory, 24h TTL (1h for MCP), 5min health checks
- MCP auto-indexing: 50 files limit on startup
- Process shutdown: 5-second graceful timeout
- Directory scanning: 3-level depth limit
- Language timeouts: Java 60s, Python 30s, others 15s

**Project Intelligence** (`src/internal/project/`):
- **Language Detection**: Multi-indicator confidence scoring, only returns languages with available LSP servers
- **Package Info Extraction**: Extracts name, version, repository from go.mod, package.json, pyproject.toml, setup.py, pom.xml, build.gradle
- **Workspace Scanner**: 3-level depth limit, skips hidden/vendor/node_modules directories

**Cache System** (`src/server/cache/`):
- SCIP-based LRU cache with 512MB default
- Dual ID format support: simple (`"go:TestFunction"`) and SCIP format
- File watching with 5-second intervals for modification detection
- JSON persistence with separate `scip_index.json` and `document_index.json`
- Method priority system: definition=10, references=9, hover=8

### MCP Server Mode
- **Always Enhanced**: MCP server operates exclusively in enhanced mode with 512MB cache, 1hr TTL
- **Background Indexing**: Auto-indexes common patterns, 50 files on startup
- **Available Tools**: 4 MCP tools - `findSymbols`, `findReferences`, `findDefinitions`, `getSymbolInfo`
- **Protocol**: MCP version 2025-06-18, STDIO transport

## Development Guidelines

1. **Local Focus**: No enterprise features (auth, monitoring, distributed systems)
2. **Error Handling**: Structured errors with wrapping (`fmt.Errorf("context: %w", err)`)
3. **Untyped JSON-RPC**: Uses `interface{}` for LSP params/results (intentional simplicity)
4. **Cache Design**: SCIP interface-based, cache effectively required for HTTP/MCP modes
5. **STDIO Safety**: All logging must use stderr-only loggers from `src/internal/common/logging.go`

## Common Development Tasks

### Adding a New LSP Method
1. Update capability detection in `src/server/capabilities/detector.go`
2. Add error translation in `src/server/errors/translator.go`
3. Update MCP tools in `src/server/mcp_tools.go`
4. Add E2E test in `tests/e2e/`

### Debugging LSP Communication
```bash
# Enable debug logging
export LSP_GATEWAY_DEBUG=true
lsp-gateway server

# Test HTTP gateway with curl
curl -X POST localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"textDocument/definition","params":{"textDocument":{"uri":"file:///path/to/file.go"},"position":{"line":10,"character":5}},"id":1}'
```

## Important Constraints

- **Exactly 5 languages**: Go, Python, JavaScript, TypeScript, Java
- **6 LSP methods only**: Core developer productivity features
- **4 MCP tools**: findSymbols, findReferences, findDefinitions, getSymbolInfo
- **Manual LSP server installation**: Use `lsp-gateway install` helpers
- **Local development only**: Not designed for production/enterprise use

## Common Gotchas

1. **STDIO Logging**: Never use `fmt.Print*` or `log.Print*` - breaks LSP protocol. Use `common.LSPLogger`, `common.GatewayLogger`, or `common.CLILogger` only.
2. **Cache Nil Checks**: Always check `if m.scipCache != nil` before cache operations - cache is optional at interface level but effectively required for HTTP/MCP modes.
3. **Test Repositories**: E2E tests clone real GitHub repos. First run may be slow (~30s per repo).
4. **LSP Server Installation**: Must manually install LSP servers before use (`lsp-gateway install all`). Check with `lsp-gateway status`.
5. **Process Management**: LSP servers have 5-second graceful shutdown. Don't force-kill processes.

## Troubleshooting

**LSP server not found**: Run `lsp-gateway install all` or check PATH for manual installations
**Port 8080 in use**: Use `lsp-gateway server --port 8081` or update config.yaml
**MCP connection fails**: Check STDIO output isn't polluted, use stderr-only logging
**Cache not working**: Verify write permissions on `~/.lsp-gateway/scip-cache/`, check disk space
**Slow initial requests**: First cache population takes time, subsequent lookups are sub-millisecond