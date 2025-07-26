# AI/Agent Code Reference Guide

This document provides critical file locations and code references for quick AI/Agent navigation of the LSP Gateway codebase.

## 🎯 Core Entry Points

### Primary Application Entry
- `cmd/lsp-gateway/main.go:20-25` - Main CLI launcher, delegates to internal/cli
- `internal/cli/root.go:64-66` - Cobra CLI root command with 20+ subcommands
- `internal/cli/server.go:52-76` - HTTP JSON-RPC gateway server startup
- `internal/cli/mcp.go:82-95` - MCP server for AI assistant integration

### Key Business Logic
- `internal/gateway/handlers.go:889-920` - **CRITICAL** - Primary HTTP JSON-RPC request handler
- `internal/gateway/handlers.go:523-716` - Gateway initialization with project awareness
- `internal/gateway/handlers.go:1557-1605` - SCIP cache-first routing (60-87% performance boost)
- `mcp/server.go:444-463` - MCP protocol bridge to LSP capabilities

## 🔄 Request Processing Flow

### HTTP Gateway Flow
- `internal/gateway/handlers.go:889` - `HandleJSONRPC()` - Main HTTP entry point
- `internal/gateway/handlers.go:132-199` - Request tracking and lifecycle management
- `internal/gateway/smart_router.go` - 6 routing strategies with performance metrics
- `internal/gateway/scip_smart_router.go` - Cache-first routing with SCIP intelligence

### MCP Processing Flow
- `mcp/server.go:765` - `handleInitializeWithValidation()` - MCP handshake
- `mcp/server.go:814` - `handleCallToolWithValidation()` - Tool execution routing
- `mcp/tools.go:116-220` - Standard LSP tools (definition, references, hover, symbols)
- `mcp/tools_scip_enhanced.go:213-442` - AI-powered semantic analysis tools

## 🚚 Transport Layer

### STDIO Transport (Process-based)
- `internal/transport/stdio.go:28-677` - Process-based LSP communication
- `internal/transport/stdio.go:601-615` - Circuit breaker with exponential backoff
- `internal/transport/stdio.go` - SCIP background caching integration

### TCP Transport (Network-based) 
- `internal/transport/tcp.go:36-879` - Network-based LSP communication
- `internal/transport/tcp.go:589-817` - Connection pooling (5 connections per pool)
- `internal/transport/tcp.go:66` - Atomic circuit breaker implementation

### Circuit Breaker Protection
- `internal/gateway/circuit_breaker.go:109-543` - Three-state circuit breaker (Closed/Open/Half-Open)
- `internal/transport/stdio.go:601` - Process transport circuit breaker
- `internal/transport/tcp.go` - Network transport fault tolerance

## ⚙️ Configuration System

### Core Configuration
- `internal/config/config.go:172` - `GatewayConfig` main configuration structure
- `internal/config/config.go:238` - `SCIPConfiguration` for intelligent caching
- `internal/config/loader.go:12` - Configuration loading with environment overrides
- `internal/config/enhanced_validation.go:128` - Comprehensive validation with scoring

### Configuration Templates
- `config-templates/languages/go-advanced.yaml` - Go development with SCIP optimization
- `config-templates/languages/java-spring.yaml` - Spring Boot with JDTLS
- `config-templates/languages/python-django.yaml` - Django with ML/data science support
- `config-templates/languages/typescript-react.yaml` - React with modern tooling
- `config-templates/patterns/polyglot.yaml` - Multi-language enterprise projects

### Dynamic Generation
- `internal/config/multi_language_generator.go:570` - Polyglot project configuration
- `internal/project/config_generator.go:542-863` - Framework-aware configuration

## 🔧 Setup and Installation

### Installation Orchestration
- `internal/setup/orchestrator.go:244-295` - 5-stage installation workflow
- `internal/setup/detector.go:167-172` - Runtime detection (Go, Python, Node.js, Java)
- `internal/setup/templates.go:328-440` - Template selection with scoring algorithm

### Platform Support
- `internal/platform/platform.go:32-43` - Cross-platform detection (Windows, Linux, macOS)
- `internal/platform/packagemgr.go` - Package manager integration (Homebrew, APT, Winget, etc.)
- `internal/installer/runtime_strategies.go` - Platform-specific installation strategies

### Version Requirements
- `internal/setup/version.go:35-47` - Minimum versions (Go 1.19+, Python 3.9+, Node.js 22+, Java 17+)

## 🔍 Project Detection

### Multi-Language Detection
- `internal/project/detector.go:132-195` - 6-stage project analysis pipeline
- `internal/project/detectors/typescript_detector.go:829-873` - React/Next.js/Vue detection
- `internal/project/detectors/python_detector.go:661-669` - Django/Flask/FastAPI detection
- `internal/project/detectors/java_detector.go` - Spring Boot and Maven/Gradle detection

### Framework Auto-Detection
- `internal/config/enhancers.go:12` - Framework-specific enhancers (React, Django, Spring Boot)
- `internal/project/language_detector.go` - Language runtime validation
- `internal/project/workspace.go` - Workspace context management

## 🧠 SCIP Intelligence System

### Core SCIP Integration
- `internal/indexing/scip_store.go` - SCIP index storage and retrieval
- `internal/indexing/symbol_resolver.go` - Symbol resolution with caching
- `internal/indexing/lsp_scip_mapper.go` - LSP-to-SCIP protocol mapping
- `internal/indexing/watcher.go` - File system monitoring for index updates

### Enhanced MCP Tools
- `mcp/tools_scip_enhanced.go:148-178` - SCIP-enhanced tool handler creation
- `mcp/scip_integration.go:136-193` - Performance monitoring and health checks
- `mcp/scip_integration.go:756-768` - Auto-tuning and optimization
- `mcp/workspace_context.go:64-82` - Project-aware context initialization

### Performance Optimization
- `internal/gateway/performance_cache.go` - Response caching layer
- `internal/storage/hybrid_manager.go` - Multi-tier caching strategy
- `internal/indexing/performance_cache.go` - SCIP-specific caching optimizations

## 🎛️ CLI Commands

### Essential Commands
- `internal/cli/server.go` - `lsp-gateway server` - HTTP gateway startup
- `internal/cli/mcp.go` - `lsp-gateway mcp` - MCP server for AI integration  
- `internal/cli/setup.go` - `lsp-gateway setup all` - Automated installation
- `internal/cli/diagnose.go` - `lsp-gateway diagnose` - Comprehensive diagnostics

### Configuration Commands
- `internal/cli/config.go` - `lsp-gateway config generate` - Dynamic config creation
- `internal/cli/validation.go` - `lsp-gateway config validate` - Configuration validation
- `internal/cli/install.go` - `lsp-gateway install <server>` - Language server installation

### Status and Verification
- `internal/cli/status.go` - `lsp-gateway status` - Server health monitoring
- `internal/cli/verify.go` - `lsp-gateway verify` - Installation verification
- `internal/cli/version.go` - `lsp-gateway version` - Version information

## 🏗️ Multi-Server Management

### Server Orchestration
- `internal/gateway/multi_server_manager.go` - Multi-server coordination and health tracking
- `internal/gateway/server_instance.go` - Individual server lifecycle management
- `internal/gateway/load_balancer.go` - Request distribution across servers
- `internal/gateway/response_aggregator.go` - Multi-server response merging

### Workspace Management
- `internal/gateway/workspace_manager.go` - Project workspace coordination
- `internal/gateway/workspace_server_manager.go` - Workspace-specific server management
- `internal/project/workspace.go` - Workspace context and metadata

## 🧪 Testing Infrastructure

### E2E Testing
- `tests/e2e/setup_cli_e2e_test.go` - Comprehensive setup CLI E2E test suite with JSON validation
- `tests/e2e/typescript_advanced_e2e_test.go` - Advanced TypeScript workflow validation
- `tests/e2e/scenarios/lsp_workflow_scenarios.go` - Real-world LSP usage scenarios
- `tests/e2e/scenarios/performance_scenarios.go` - Performance benchmark scenarios

### Integration Testing
- `tests/integration/scip/scip_integration_test_suite.go` - SCIP system integration tests
- `tests/integration/scip_mcp_integration_test.go` - MCP-SCIP integration validation
- `tests/performance/scip_performance_test.go` - SCIP performance benchmarks

### Test Data
- `tests/fixtures/lsp_responses/` - Mock LSP responses for testing
- `tests/fixtures/project_structures/` - Multi-language project examples
- `tests/integration/scip/testdata/scip/scip_indices/` - SCIP test indexes

## 🔧 Build and Development

### Build System
- `Makefile` - Cross-platform build system (Linux, macOS, Windows)
- `go.mod` - Go 1.24+ dependencies with SCIP, Cobra, AWS SDK
- `package.json` - Node.js wrapper for npm distribution

### Development Commands
- `make local` - Build for current platform
- `make test-unit` - Fast unit tests (<60s)
- `make test-simple-quick` - Quick E2E validation (1 minute)
- `make test-integration` - Integration test suite
- `make test-lsp-validation-short` - Quick LSP validation (2min)
- `make quality` - Format + lint + security analysis

## 🚨 Critical Performance Points

### High-Performance Components
- SCIP caching provides 60-87% response time improvements
- Cache hit rates of 85-90% for repeated symbol queries
- Connection pooling with 5 connections per TCP pool
- Circuit breakers prevent cascade failures

### Memory Optimization
- ~65-75MB additional memory for SCIP cache
- LRU cache with TTL for optimal memory usage
- Background indexing to minimize UI blocking
- Graceful degradation when resources are constrained

## 📍 Key Architecture Decisions

- **Local Development Focus** - No authentication/monitoring complexity
- **Dual Protocol Support** - HTTP JSON-RPC + MCP for different integration patterns  
- **Template-Driven Configuration** - Rapid setup for common development scenarios
- **Interface-Based Design** - Clean abstractions for testing and extensibility
- **Performance-First Architecture** - SCIP integration with intelligent caching

---

*This reference guide covers the most critical files and functions for AI/Agent navigation. For complete details, refer to the full source files.*