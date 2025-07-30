# AI/Agent Code Reference Guide

This document provides critical file locations and code references for quick AI/Agent navigation of the LSP Gateway codebase.

## 🎯 Core Entry Points

### Primary Application Entry
- `cmd/lsp-gateway/main.go` - Main CLI launcher, delegates to internal/cli
- `internal/cli/root.go` - Cobra CLI root command with 20+ subcommands
- `internal/cli/server.go` - HTTP JSON-RPC gateway server startup 
- `internal/cli/mcp.go` - MCP server for AI assistant integration

### Key Business Logic
- `internal/gateway/handlers.go` - **CRITICAL** - Primary HTTP JSON-RPC request handler (`HandleJSONRPC`)
- `internal/gateway/handlers.go` - Gateway initialization with project awareness
- `internal/gateway/handlers.go` - SCIP cache-first routing (60-87% performance boost)
- `mcp/server.go` - MCP protocol bridge to LSP capabilities

## 🔄 Request Processing Flow

### HTTP Gateway Flow
- `internal/gateway/handlers.go` - `HandleJSONRPC()` - Main HTTP entry point
- `internal/gateway/handlers.go` - Request tracking and lifecycle management
- `internal/gateway/smart_router.go` - 6 routing strategies with performance metrics
- `internal/gateway/scip_smart_router.go` - Cache-first routing with SCIP intelligence

### MCP Processing Flow
- `mcp/server.go` - Core MCP server initialization and protocol setup
- `mcp/server.go` - `handleInitializeWithValidation()` - MCP handshake
- `mcp/server.go` - `handleCallToolWithValidation()` - Tool execution routing
- `mcp/tools.go` - Standard LSP tools registration and implementation
- `mcp/tools.go` - `fallbackWorkspaceSymbolSearch()` - Fallback for pylsp workspace/symbol
- `mcp/tools_scip_enhanced.go` - AI-powered semantic analysis tools

### MCP Tool Ecosystem
- **Core LSP Tools**: `mcp/tools.go` - 6 supported LSP features exposed as MCP tools
- **SCIP-Enhanced Tools**: `mcp/tools_scip_enhanced.go` - Performance-optimized AI semantic tools
- **Workspace Tools**: `mcp/workspace_context.go` - Project-aware context and navigation
- **Project Tools**: `mcp/project_tools.go` - Project detection and analysis tools
- **Enhanced Tools**: `mcp/tools_enhanced.go` - Advanced MCP tool implementations

## 🚚 Transport Layer

### STDIO Transport (Process-based)
- `internal/transport/stdio.go` - Process-based LSP communication
- `internal/transport/stdio.go` - Circuit breaker with exponential backoff
- `internal/transport/stdio.go` - SCIP background caching integration

### TCP Transport (Network-based) 
- `internal/transport/tcp.go` - Network-based LSP communication
- `internal/transport/tcp.go` - Connection pooling (5 connections per pool)
- `internal/transport/tcp.go` - Atomic circuit breaker implementation

### Circuit Breaker Protection
- `internal/gateway/circuit_breaker.go` - Three-state circuit breaker (Closed/Open/Half-Open)
- `internal/transport/stdio.go` - Process transport circuit breaker
- `internal/transport/tcp.go` - Network transport fault tolerance

## ⚙️ Configuration System

### Core Configuration
- `internal/config/config.go` - `GatewayConfig` main configuration structure
- `internal/config/config.go` - `SCIPConfiguration` for intelligent caching
- `internal/config/loader.go` - Configuration loading with environment overrides
- `internal/config/enhanced_validation.go` - Comprehensive validation with scoring

### Configuration Template System
- **Language Templates**: `config-templates/languages/` - Language-specific optimized configurations
  - `go-advanced.yaml` - Go development with gopls and SCIP optimization
  - `java-spring.yaml` - Spring Boot with JDTLS and Maven/Gradle support
  - `python-django.yaml` - Django with ML/data science support
  - `typescript-react.yaml` - React with modern TypeScript tooling
- **Pattern Templates**: `config-templates/patterns/` - Project pattern configurations
  - `polyglot.yaml` - Multi-language enterprise projects
  - `full-stack.yaml` - Full-stack development setup
- **Template Selection**: `internal/setup/templates.go` - Scoring algorithm for optimal templates

### Dynamic Generation
- `internal/config/multi_language_generator.go` - Polyglot project configuration
- `internal/project/config_generator.go` - Framework-aware configuration
- `internal/setup/orchestrator.go` - Template application and customization

### Framework-Aware Configuration
- `internal/config/enhancers.go` - React/Django/Spring Boot specific enhancers
- `internal/project/language_detector.go` - Language runtime validation
- `internal/project/workspace.go` - Workspace context management

## 🔧 Setup and Installation

### Installation Orchestration
- `internal/setup/orchestrator.go` - 5-stage installation workflow
- `internal/setup/detector.go` - Runtime detection (Go, Python, Node.js, Java)
- `internal/setup/templates.go` - Template selection with scoring algorithm

### Platform Support
- `internal/platform/platform.go` - Cross-platform detection (Windows, Linux, macOS)
- `internal/platform/packagemgr.go` - Package manager integration (Homebrew, APT, Winget, etc.)
- `internal/installer/runtime_strategies.go` - Platform-specific installation strategies

### Version Requirements
- `internal/setup/version.go` - Minimum versions (Go 1.24+, Python 3.9+, Node.js 20+, Java 17+)

## 🔍 Project Detection

### Multi-Language Detection
- `internal/project/detector.go` - 6-stage project analysis pipeline
- `internal/project/detectors/typescript_detector.go` - React/Next.js/Vue detection
- `internal/project/detectors/python_detector.go` - Django/Flask/FastAPI detection
- `internal/project/detectors/java_detector.go` - Spring Boot and Maven/Gradle detection
- `internal/project/detectors/go_detector.go` - Go module and workspace detection

### Framework Auto-Detection
- `internal/config/enhancers.go` - Framework-specific enhancers (React, Django, Spring Boot)
- `internal/project/language_detector.go` - Language runtime validation
- `internal/project/workspace.go` - Workspace context management

## 🧠 SCIP Intelligence System

### Core SCIP Integration
- `internal/indexing/scip_store.go` - SCIP index storage and retrieval with LRU caching
- `internal/indexing/symbol_resolver.go` - Symbol resolution with intelligent caching
- `internal/indexing/lsp_scip_mapper.go` - LSP-to-SCIP protocol mapping and type conversion
- `internal/indexing/watcher.go` - File system monitoring for real-time index updates

### Two-Tier SCIP Caching Architecture
- **Tier 1 - Memory Cache**: `internal/storage/memory_cache.go` - Hot data with 1ms access time
- **Tier 2 - Local Storage**: `internal/indexing/scip_store.go` - Persistent SCIP indexes
- **LSP Fallback**: `internal/gateway/handlers.go` - Cache-miss LSP routing
- **Cache Coordination**: `internal/storage/hybrid_manager.go` - Two-tier cache orchestration

### Enhanced MCP Tools
- `mcp/tools_scip_enhanced.go` - SCIP-enhanced tool handler creation
- `mcp/tools_scip_enhanced.go` - AI-powered semantic analysis with SCIP intelligence
- `mcp/scip_integration.go` - Performance monitoring and health checks
- `mcp/scip_integration.go` - Auto-tuning and optimization
- `mcp/workspace_context.go` - Project-aware context initialization

### Performance Optimization
- `internal/gateway/performance_cache.go` - Response caching layer with TTL management
- `internal/storage/hybrid_manager.go` - Two-tier caching strategy with LRU eviction
- `internal/indexing/performance_cache.go` - SCIP-specific caching optimizations

## 🎛️ CLI Commands

### Essential Commands
- `internal/cli/server.go` - `lspg server` - HTTP gateway startup with SCIP caching
- `internal/cli/mcp.go` - `lspg mcp` - MCP server for AI integration  
- `internal/cli/setup.go` - `lspg setup all` - Orchestrated automated installation
- `internal/cli/diagnose.go` - `lspg diagnose` - Comprehensive system diagnostics

### Configuration Commands
- `internal/cli/config.go` - `lspg config generate` - Dynamic config creation  
- `internal/cli/config.go` - Template-based configuration with framework detection
- `internal/cli/validation.go` - `lspg config validate` - Configuration validation
- `internal/cli/install.go` - `lspg install <server>` - Language server installation

### Status and Verification
- `internal/cli/status.go` - `lspg status` - Server health monitoring with metrics
- `internal/cli/verify.go` - `lspg verify` - Installation verification with scoring
- `internal/cli/version.go` - `lspg version` - Version information with dependencies

## 🏗️ Workspace Management System

### Core Workspace Components
- `internal/workspace/` - Complete workspace management system
- `internal/workspace/gateway.go` - Workspace-aware gateway implementation
- `internal/workspace/client_manager.go` - Language-specific LSP client lifecycle management
- `internal/workspace/config_manager.go` - Workspace-specific configuration loading
- `internal/workspace/subproject_resolver.go` - Intelligent sub-project routing

### Multi-Project Support
- `internal/gateway/multi_server_manager.go` - Multi-server coordination and health tracking
- `internal/gateway/server_instance.go` - Individual server lifecycle management
- `internal/gateway/load_balancer.go` - Request distribution across servers
- `internal/gateway/response_aggregator.go` - Multi-server response merging

## 🧪 Testing Infrastructure

### E2E Testing
- `tests/e2e/` - End-to-end test suites for all supported languages
- `tests/e2e/multi_project_workspace_e2e_test.go` - Multi-project workspace testing
- `tests/e2e/testutils/` - Unified testing utilities and repository management
- `tests/e2e/testutils/multi_project_manager.go` - Multi-project workspace test setup

### Integration Testing
- `tests/integration/` - Component integration tests
- `tests/integration/workspace/` - Workspace system integration tests
- `tests/integration/scip/` - SCIP system integration tests

### Test Infrastructure
- `tests/e2e/testutils/repository_manager.go` - Unified repository management for real project testing
- `tests/e2e/testutils/http_client.go` - HTTP client for gateway testing
- `tests/integration/scip/testdata/` - SCIP test data and indices

## 🔧 Build and Development

### Build System
- `Makefile` - Cross-platform build system with parallel execution optimization
- `go.mod` - Go 1.24+ dependencies with SCIP, Cobra, workspace management
- `package.json` - Node.js wrapper for npm distribution

### Development Commands
- `make local` - Build for current platform and create global `lspg` command
- `make test-simple-quick` - Quick E2E validation (2-5min)
- `make test-lsp-validation-short` - Short LSP validation (2-5min)  
- `make test-parallel-validation` - Standard parallel validation
- `make quality` - Format + lint + security analysis

## 🚨 Critical Performance Points

### High-Performance Components
- **SCIP Caching**: `internal/indexing/scip_store.go` - 60-87% response time improvements
- **Memory Cache**: `internal/storage/memory_cache.go` - 85-90% cache hit rates for repeated queries
- **Connection Pooling**: `internal/transport/tcp.go` - 5 connections per TCP pool
- **Circuit Breakers**: `internal/gateway/circuit_breaker.go` - Prevent cascade failures

### Performance Optimization
- **Request Routing**: `internal/gateway/handlers.go` - Cache-first routing with SCIP intelligence
- **Background Indexing**: `internal/indexing/watcher.go` - Non-blocking file system monitoring
- **Response Aggregation**: `internal/gateway/response_aggregator.go` - Multi-server response merging
- **Load Balancing**: `internal/gateway/load_balancer.go` - Intelligent request distribution
- **Cache Eviction**: `internal/storage/hybrid_manager.go` - LRU with TTL optimization

### Memory & Performance Monitoring
- **SCIP Cache**: ~65-75MB additional memory for SCIP cache with 85-90% hit rates
- **Performance Monitoring**: `mcp/scip_integration.go` - Real-time performance monitoring
- **Health Checks**: `internal/cli/diagnose.go` - Performance benchmarking and health validation
- **Workspace Management**: `internal/workspace/` - Multi-project resource isolation and management

## 📍 Key Architecture Decisions

- **Local Development Focus** - No authentication/monitoring complexity
- **Dual Protocol Support** - HTTP JSON-RPC + MCP for different integration patterns  
- **Workspace Management System** - Multi-project support with intelligent sub-project routing
- **Template-Driven Configuration** - Rapid setup for common development scenarios
- **Interface-Based Design** - Clean abstractions for testing and extensibility
- **Performance-First Architecture** - SCIP integration with intelligent caching (60-87% improvement)
- **E2E Testing Philosophy** - Real language servers with actual open-source repositories

---

**Note**: This reference guide focuses on file locations and architectural components rather than specific line numbers, which can change frequently. For exact function locations, use your editor's search functionality or grep tools.

*This reference guide covers the most critical files and functions for AI/Agent navigation. For complete implementation details, refer to the full source files.*