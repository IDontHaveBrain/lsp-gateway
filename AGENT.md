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

LSP Gateway는 **E2E 테스트 우선** 접근법을 사용합니다. 실제 개발 워크플로우와 사용 시나리오를 중심으로 테스트합니다.

### E2E 테스트 핵심 원칙
- **Real Workflow Testing**: 개발자가 실제 사용하는 시나리오 테스트
- **Dual Protocol Coverage**: HTTP JSON-RPC와 MCP 프로토콜 모두 검증
- **Language Server Integration**: 실제 언어 서버와의 완전한 통합 테스트
- **Real Codebase Validation**: Kubernetes, Django, VS Code 등 실제 프로젝트 대상 테스트

### 주요 E2E 테스트 명령어
```bash
# Quick E2E validation (1분)
make test-simple-quick

# Full LSP validation (5분)
make test-lsp-validation

# Integration + performance tests (10분)
make test-integration

# Java LSP integration tests (10분)
make test-jdtls-integration

# Repository-based testing
make setup-simple-repos     # Setup Kubernetes, Django, VS Code repos
make test-lsp-repos         # Validate against real codebases
```

### E2E 테스트 시나리오
1. **기본 설정 및 시작**: 완전한 설정부터 서버 시작까지
2. **HTTP JSON-RPC Protocol**: 모든 LSP 메소드 검증
3. **MCP Protocol**: AI 어시스턴트 통합 시나리오
4. **Multi-Language**: Go, Python, TypeScript, Java 통합 테스트
5. **Performance & Load**: 동시 요청 처리 및 Circuit Breaker 테스트
6. **Real Codebase**: 실제 프로젝트 대상 포괄적 검증

## Common Development Commands

### Build Commands
```bash
make local                    # Build for current platform
make build                    # Build all platforms
make clean                    # Clean build artifacts
```

### Testing Commands (E2E 중심)
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
