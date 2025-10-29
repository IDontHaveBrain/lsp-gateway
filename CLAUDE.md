# CLAUDE.md

Development reference working with this repository.

Multi-language LSP orchestration with HTTP Gateway and MCP Server

## Purpose

This document provides essential architecture, patterns, and conventions for developing LSP Gateway.

## Quick Reference

```bash
make local          # Build + npm link (primary development workflow)
make test           # Unit + integration tests
make lint           # Format + lint (golangci-lint)
```

## Architecture Overview

### System Design

**LSP Gateway** orchestrates language servers through a unified interface with two independent server modes:

- **HTTP Gateway**: JSON-RPC endpoint for IDE/editor integration
- **MCP Server**: Model Context Protocol for AI assistant integration

### Core Components

```
LSPManager (Manager of Managers)
├── Language Clients (8 languages: Go, Python, JS, TS, Java, Rust, C#, Kotlin)
│   └── StdioClient (per-language LSP server communication)
├── Cache System (optional SCIP cache with graceful degradation)
├── Document Manager (lifecycle tracking)
├── Workspace Aggregator (parallel multi-language operations)
└── File Watcher (change detection and cache invalidation)

Gateway Servers (independent)
├── HTTPGateway (JSON-RPC on port 8080)
└── MCPServer (STDIO protocol for AI assistants)
```

### Module Organization

```
src/
├── server/           # Core server implementations
│   ├── cache/        # Optional SCIP cache (50+ files)
│   ├── aggregators/  # Parallel execution framework
│   ├── capabilities/ # Dynamic LSP capability detection
│   ├── documents/    # Document lifecycle management
│   ├── watcher/      # File change monitoring
│   ├── errors/       # Unified error translation
│   ├── scip/         # SCIP protocol support
│   └── protocol/     # JSON-RPC protocol layer
├── cli/              # Command implementations
├── config/           # Configuration management
├── internal/         # Shared utilities and types
│   ├── registry/     # Language definitions and metadata
│   ├── security/     # Command validation (whitelist-based)
│   ├── common/       # STDIO-safe logging and utilities
│   ├── project/      # Project detection and analysis
│   └── installer/    # Language server installation
└── cmd/              # Main entry point
```

## Critical Architectural Patterns

### 1. Graceful Degradation (MANDATORY)

All optional components must fail safely without breaking core functionality.

**Cache Pattern:**
```go
// ALWAYS check for nil before using optional components
if m.scipCache != nil {
    if result, found := m.scipCache.Lookup(method, params); found {
        return result
    }
}
// Continue with direct LSP if cache unavailable
```

**Never assume component availability.** The system must work fully without cache, advanced features, or optional services.

### 2. STDIO-Safe Logging (MANDATORY)

**Required Pattern:**
```go
import "lsp-gateway/src/internal/common"

// Use only these loggers
common.LSPLogger.Info("message")      // LSP/MCP operations
common.GatewayLogger.Error("error")   // HTTP gateway
common.CLILogger.Warn("warning")      // CLI commands

// NEVER USE:
// fmt.Print*, log.Print*, log.New()
```

### 3. Parallel Aggregation Framework

Multi-language operations use concurrent execution with language-specific timeouts.

**Concept:**
```go
// ParallelAggregator manages concurrent LSP operations
aggregator := NewParallelAggregator[Request, Response](
    individualTimeout,  // Per-language timeout (15s-90s)
    overallTimeout,     // Total timeout (max individual + 25%)
)
results := aggregator.Execute(ctx, clients, request, executor)
```

## Development Conventions

### Module Paths

- **Go module**: `lsp-gateway`
- **Import paths**: `lsp-gateway/src/...`
- **Not** `github.com/...` - local module

### Build System

- **Build tags**: `cache_enabled` for SCIP cache support
- **Platform targets**: linux, macos, macos-arm64, windows
- **Binary wrapper**: `bin/lsp-gateway.js` for npm integration
- **Version injection**: Via LDFLAGS at build time

### Testing Strategy

**Two test levels:**

1. **Unit Tests**
   - Location: `src/**/*_test.go`
   - Command: `make test-unit`
   - Scope: Component-level, utilities

2. **Integration Tests**
   - Location: `tests/integration/*_test.go`
   - Command: `make test-integration`
   - Scope: Component interaction, cache, gateway
   - Parallelism: Limited to `p=2`

**Fast workflow**: `make test` runs unit + integration tests

### Configuration

- **Default path**: `~/.lsp-gateway/config.yaml`
- **Auto-detection**: Languages detected from project structure
- **Cache enabled**: By default (512MB, 24hr TTL)
- **MCP mode**: Always uses enhanced mode with optimized cache settings

## Must-Follow Rules

1. **Always check for nil** before using optional components (`if cache != nil`)
2. **Use STDIO-safe logging** exclusively (import `common` package loggers)
3. **Never assume cache availability** - it's optional by design

---

**Note**: This document focuses on architecture and patterns. For usage, installation, and configuration details, refer to README.md and docs/configuration.md.
