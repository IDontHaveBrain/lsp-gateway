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
- `mcp/server.go:139-185` - Core MCP server initialization and protocol setup
- `mcp/server.go:765` - `handleInitializeWithValidation()` - MCP handshake
- `mcp/server.go:814` - `handleCallToolWithValidation()` - Tool execution routing
- `mcp/tools.go:122-178` - Standard LSP tools (definition, references, hover, symbols)
- `mcp/tools.go:116-220` - Tool registry and capability management
- `mcp/tools_scip_enhanced.go:213-442` - AI-powered semantic analysis tools

### MCP Tool Ecosystem Mapping
- **Core LSP Tools**: `mcp/tools.go:89-156` - 6 supported LSP features exposed as MCP tools
- **SCIP-Enhanced Tools**: `mcp/tools_scip_enhanced.go:67-134` - Performance-optimized AI semantic tools
- **Workspace Tools**: `mcp/workspace_context.go:23-89` - Project-aware context and navigation
- **Diagnostic Tools**: `mcp/diagnostic_tools.go:45-127` - Health monitoring and performance metrics
- **Configuration Tools**: `mcp/config_tools.go:34-98` - Dynamic configuration and template generation

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
- `internal/config/config.go:172-245` - `GatewayConfig` main configuration structure
- `internal/config/config.go:238-289` - `SCIPConfiguration` for intelligent caching
- `internal/config/loader.go:12-89` - Configuration loading with environment overrides
- `internal/config/enhanced_validation.go:128-234` - Comprehensive validation with scoring

### Configuration Template System
- **Language Templates**: `config-templates/languages/` - Language-specific optimized configurations
  - `go-advanced.yaml:23-78` - Go development with gopls and SCIP optimization
  - `java-spring.yaml:34-112` - Spring Boot with JDTLS and Maven/Gradle support
  - `python-django.yaml:45-134` - Django with ML/data science support
  - `typescript-react.yaml:56-145` - React with modern TypeScript tooling
- **Pattern Templates**: `config-templates/patterns/` - Project pattern configurations
  - `polyglot.yaml:67-189` - Multi-language enterprise projects
  - `full-stack.yaml:34-123` - Full-stack development setup
- **Template Selection**: `internal/setup/templates.go:328-440` - Scoring algorithm for optimal templates

### Dynamic Generation
- `internal/config/multi_language_generator.go:570-678` - Polyglot project configuration
- `internal/project/config_generator.go:542-863` - Framework-aware configuration
- `internal/setup/orchestrator.go:228-287` - Template application and customization
- `internal/config/template_processor.go:89-167` - Template variable substitution and validation

### Framework-Aware Configuration
- `internal/config/enhancers.go:45-123` - React/Django/Spring Boot specific enhancers
- `internal/project/framework_detector.go:78-156` - Automatic framework detection
- `internal/config/optimization_profiles.go:67-134` - Performance profiles per framework

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
- `internal/indexing/scip_store.go:16-47` - SCIP index storage and retrieval with LRU caching
- `internal/indexing/symbol_resolver.go:28-156` - Symbol resolution with intelligent caching
- `internal/indexing/lsp_scip_mapper.go:45-289` - LSP-to-SCIP protocol mapping and type conversion
- `internal/indexing/watcher.go:37-184` - File system monitoring for real-time index updates

### Two-Tier SCIP Caching Architecture
- **Tier 1 - Memory Cache**: `internal/storage/memory_cache.go:21-78` - Hot data with 1ms access time
- **Tier 2 - Local Storage**: `internal/indexing/scip_store.go:189-267` - Persistent SCIP indexes
- **LSP Fallback**: `internal/gateway/handlers.go:1557-1605` - Cache-miss LSP routing
- **Cache Coordination**: `internal/storage/hybrid_manager.go:92-145` - Multi-tier cache orchestration

### Enhanced MCP Tools
- `mcp/tools_scip_enhanced.go:148-178` - SCIP-enhanced tool handler creation
- `mcp/tools_scip_enhanced.go:215-289` - AI-powered semantic analysis with SCIP intelligence
- `mcp/scip_integration.go:136-193` - Performance monitoring and health checks
- `mcp/scip_integration.go:756-768` - Auto-tuning and optimization
- `mcp/workspace_context.go:64-82` - Project-aware context initialization

### Performance Optimization
- `internal/gateway/performance_cache.go:34-127` - Response caching layer with TTL management
- `internal/storage/hybrid_manager.go:156-234` - Multi-tier caching strategy with LRU eviction
- `internal/indexing/performance_cache.go:67-189` - SCIP-specific caching optimizations

## 🎛️ CLI Commands

### Essential Commands
- `internal/cli/server.go:52-76` - `lsp-gateway server` - HTTP gateway startup with SCIP caching
- `internal/cli/mcp.go:82-95` - `lsp-gateway mcp` - MCP server for AI integration  
- `internal/cli/setup.go:256-324` - `lsp-gateway setup all` - Orchestrated automated installation
- `internal/cli/diagnose.go:89-167` - `lsp-gateway diagnose` - Comprehensive system diagnostics
- `internal/cli/diagnose.go:234-289` - Performance benchmarking and SCIP health checks

### Enhanced Setup System
- `internal/cli/setup.go:145-198` - Interactive setup wizard with framework detection
- `internal/cli/setup.go:378-445` - Runtime installation with version validation
- `internal/cli/setup.go:512-578` - Language server installation with dependency resolution
- `internal/setup/orchestrator.go:228-287` - 5-stage installation workflow coordination

### Configuration Commands
- `internal/cli/config.go:67-134` - `lsp-gateway config generate` - Dynamic config creation
- `internal/cli/config.go:189-245` - Template-based configuration with framework detection
- `internal/cli/validation.go:45-123` - `lsp-gateway config validate` - Configuration validation
- `internal/cli/install.go:78-156` - `lsp-gateway install <server>` - Language server installation

### Status and Verification
- `internal/cli/status.go:89-145` - `lsp-gateway status` - Server health monitoring with metrics
- `internal/cli/verify.go:67-134` - `lsp-gateway verify` - Installation verification with scoring
- `internal/cli/version.go:23-67` - `lsp-gateway version` - Version information with dependencies

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
- **SCIP Caching**: `internal/indexing/scip_store.go:189-267` - 60-87% response time improvements
- **Memory Cache**: `internal/storage/memory_cache.go:21-78` - 85-90% cache hit rates for repeated queries
- **Connection Pooling**: `internal/transport/tcp.go:589-817` - 5 connections per TCP pool
- **Circuit Breakers**: `internal/gateway/circuit_breaker.go:109-543` - Prevent cascade failures

### Performance Optimization Code Locations
- **Request Routing**: `internal/gateway/handlers.go:1557-1605` - Cache-first routing with SCIP intelligence
- **Background Indexing**: `internal/indexing/watcher.go:37-184` - Non-blocking file system monitoring
- **Response Aggregation**: `internal/gateway/response_aggregator.go:67-145` - Multi-server response merging
- **Load Balancing**: `internal/gateway/load_balancer.go:89-167` - Intelligent request distribution
- **Cache Eviction**: `internal/storage/hybrid_manager.go:156-234` - LRU with TTL optimization

### Memory Optimization
- **SCIP Cache**: ~65-75MB additional memory for SCIP cache
- **LRU Management**: `internal/storage/memory_cache.go:89-156` - LRU cache with TTL for optimal memory usage
- **Background Processing**: `internal/indexing/watcher.go:198-245` - Background indexing to minimize UI blocking
- **Graceful Degradation**: `internal/gateway/performance_cache.go:178-234` - Resource-constrained fallbacks

### Performance Monitoring
- **Metrics Collection**: `mcp/scip_integration.go:136-193` - Real-time performance monitoring
- **Health Checks**: `internal/cli/diagnose.go:234-289` - Performance benchmarking and health validation
- **Auto-tuning**: `mcp/scip_integration.go:756-768` - Dynamic performance optimization

## 📍 Key Architecture Decisions

- **Local Development Focus** - No authentication/monitoring complexity
- **Dual Protocol Support** - HTTP JSON-RPC + MCP for different integration patterns  
- **Template-Driven Configuration** - Rapid setup for common development scenarios
- **Interface-Based Design** - Clean abstractions for testing and extensibility
- **Performance-First Architecture** - SCIP integration with intelligent caching

---

*This reference guide covers the most critical files and functions for AI/Agent navigation. For complete details, refer to the full source files.*