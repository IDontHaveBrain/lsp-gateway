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

## Development Guidelines

### General Development Practices

**CRITICAL: Always Update Related Documentation After Any Work**

After completing any development task, you **MUST** update all related documentation to ensure accuracy and completeness. This is a non-negotiable requirement for maintaining project quality.

#### Documentation Update Requirements

**When Any Change is Made:**
1. **CLAUDE.md**: Update if commands, architecture, or development workflows change
2. **README.md**: Update if installation, setup, or basic usage changes  
3. **Code Comments**: Update inline documentation for modified functions/classes
4. **API Documentation**: Update if endpoints, parameters, or responses change
5. **Configuration Examples**: Update if config schema or options change
6. **Test Documentation**: Update if testing procedures or frameworks change

#### Mandatory Documentation Updates For:

**Code Changes:**
- New CLI commands → Update CLAUDE.md CLI command structure
- New make targets → Update CLAUDE.md build commands section
- Architecture changes → Update CLAUDE.md architecture overview
- New dependencies → Update CLAUDE.md requirements section
- API changes → Update CLAUDE.md API usage examples

**Feature Additions:**
- New language servers → Update installation guides and supported languages
- New transport methods → Update configuration examples and architecture docs
- New MCP tools → Update MCP integration section
- New testing categories → Update testing infrastructure documentation

**Configuration Changes:**
- New config options → Update configuration examples and schema documentation
- Changed default values → Update all example configurations
- New environment variables → Update setup and deployment documentation

#### Documentation Verification Checklist

Before considering any task complete, verify:
- [ ] All command examples in documentation still work correctly
- [ ] Version numbers and requirements are current
- [ ] Architecture diagrams reflect actual implementation
- [ ] Code examples compile and execute successfully
- [ ] Installation instructions produce working setup
- [ ] API examples return expected responses
- [ ] Configuration examples are valid and complete

#### Documentation Standards
- **Accuracy**: All examples must be tested and functional
- **Completeness**: Include all necessary context and prerequisites
- **Clarity**: Write for developers unfamiliar with the codebase
- **Consistency**: Maintain formatting and style standards
- **Currency**: Remove outdated information immediately

**Remember: Outdated documentation is worse than no documentation. Always keep it current.**

## Quick Start (5 Minutes)

```bash
# 1. Clone and build (2 minutes)
git clone [repository-url]
cd lsp-gateway
make local

# 2. Automated setup (2 minutes)
./bin/lsp-gateway setup all          # Installs runtimes + language servers + config

# 3. Start using (30 seconds)
./bin/lsp-gateway server --config config.yaml    # HTTP Gateway (port 8080)
./bin/lsp-gateway mcp --config config.yaml       # MCP Server for AI assistants
```

## E2E Testing Strategy

**📖 Complete E2E Testing Guide**: See [docs/e2e-testing.md](docs/e2e-testing.md)

LSP Gateway uses an **E2E test-first** approach, focusing on actual development workflows and usage scenarios.

### E2E Testing Core Principles
- **Real Workflow Testing**: Testing scenarios that developers actually use
- **Dual Protocol Coverage**: Verification of both HTTP JSON-RPC and MCP protocols
- **Language Server Integration**: Complete integration testing with actual language servers
- **Real Codebase Validation**: Testing against real projects like Kubernetes, Django, VS Code

### Key E2E Testing Commands
```bash
# Quick E2E validation (1min)
make test-simple-quick

# Full LSP validation (5min)
make test-lsp-validation

# Integration + performance tests (10min)
make test-integration

# Java LSP integration tests (10min)
make test-jdtls-integration

# Repository-based testing
make setup-simple-repos     # Setup Kubernetes, Django, VS Code repos
make test-lsp-repos         # Validate against real codebases
```

### E2E Testing Scenarios
1. **Basic Setup and Startup**: From complete setup to server startup
2. **HTTP JSON-RPC Protocol**: Verification of all LSP methods
3. **MCP Protocol**: AI assistant integration scenarios
4. **Multi-Language**: Go, Python, TypeScript, Java integration tests
5. **Performance & Load**: Concurrent request processing and Circuit Breaker testing
6. **Real Codebase**: Comprehensive validation against real projects

## Common Development Commands

### Build Commands
```bash
make local                    # Build for current platform
make build                    # Build all platforms
make clean                    # Clean build artifacts
```

### Testing Commands (E2E Focused)
```bash
make test                     # Run all tests
make test-unit               # Fast unit tests only (<60s)
make test-integration        # Integration + performance tests
make test-lsp-validation     # Comprehensive LSP validation (5min)
make test-lsp-validation-short # Short LSP validation (2min)
make test-simple-quick       # Quick validation for development (1min)
make test-jdtls-integration  # Java LSP integration tests (10min)
```

### Code Quality
```bash
make format                  # Format code
make lint                    # Run golangci-lint
make security               # Run gosec security analysis
make check-deadcode         # Dead code analysis
```

### Development Workflow
```bash
# Quick development cycle with E2E validation
make local && make test-simple-quick && make format

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
- **CLI Interface** (`internal/cli/`): Comprehensive command system with 20+ commands
- **Setup System** (`internal/setup/`): Cross-platform runtime detection and auto-installation
- **Platform Abstraction** (`internal/platform/`): Multi-platform package manager integration
- **MCP Integration** (`mcp/`): Model Context Protocol server exposing LSP as MCP tools

## Installation and Setup

### Automated Setup (Recommended)
```bash
make local                           # Build for current platform
./bin/lsp-gateway setup all          # Installs runtimes + language servers
./bin/lsp-gateway server --config config.yaml
```

### Requirements
- **Go**: 1.24+ (core requirement)
- **Make**: Build system orchestration
- **Platform**: Linux, macOS (x64/arm64), Windows (x64)

## Key File Locations
- Main entry: `cmd/lsp-gateway/main.go`
- CLI root: `internal/cli/root.go`
- Gateway logic: `internal/gateway/handlers.go`  
- Configuration: `internal/config/config.go`
- MCP server: `mcp/server.go`
- Transport layer: `internal/transport/`
- **E2E Testing Guide**: `docs/e2e-testing.md`
