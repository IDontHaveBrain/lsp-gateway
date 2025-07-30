# Test Guide

LSP Gateway testing focused exclusively on E2E and integration tests for essential functionality validation.

## Testing Philosophy

**E2E and Integration Testing Only:**
- **E2E Tests**: Real-world developer workflows with actual language servers
- **Integration Tests**: Component interaction validation and protocol testing
- **Focus**: Essential functionality for local development - no unit tests, no over-engineering
- **Approach**: Test what users actually experience in practice

**Principles:**
- Test real usage scenarios, not implementation details
- Use actual language servers whenever possible
- Validate core LSP functionality across multiple languages
- Keep test scenarios simple and maintainable
- Progressive removal of non-essential test infrastructure

## Quick Reference

### Development Workflows

```bash
# Quick development cycle (2-5min)
make local && make test-simple-quick

# Pre-commit validation (5-15min)
make test-unit && make test-lsp-validation-short && make test-parallel-validation

# Full validation before PR (30-60min)
make test && make test-workspace-integration && make quality
```

### Essential Test Commands

#### Quick Validation (2-5min)
```bash
make test-simple-quick            # Basic E2E validation
make test-lsp-validation-short    # Short LSP validation
make test-parallel-validation     # Standard parallel validation
```

#### Language-Specific Real Server Testing (10-20min each)
```bash
make test-go-real-client          # Go with golang/example repo
make test-java-real-client        # Java with clean-code repo
make test-javascript-real-client  # JavaScript with chalk repo
make test-typescript-real         # TypeScript comprehensive
make test-python-patterns         # Python with real patterns
```

#### Multi-Project & Performance (15-30min)
```bash
make test-multi-project-workspace # Cross-language workspace testing
make test-parallel-performance    # Performance comparison
make test-workspace-integration   # Workspace component integration
```

#### Parallel Execution Optimization
```bash
make test-parallel-fast              # 3x CPU cores (fastest)
make test-parallel-memory-optimized  # 1x CPU cores (memory efficient)
```

## Test Categories

### E2E Tests
**Purpose**: Validate complete developer workflows with real language servers
**Location**: `tests/e2e/`
**Coverage**: 
- All 6 supported LSP methods (definition, references, hover, documentSymbol, workspaceSymbol, completion)
- Multi-language support (Go, Python, JavaScript/TypeScript, Java)
- Real repository testing with actual open-source projects
- Performance validation under realistic conditions

**Key Test Suites:**
- `python_basic_e2e_test.go` - Python E2E tests with pylsp
- `go_basic_e2e_test.go` - Go language server integration with gopls
- `javascript_basic_e2e_test.go` - JavaScript/TypeScript basic validation
- `typescript_basic_e2e_test.go` - TypeScript E2E tests
- `java_real_jdtls_e2e_test.go` - Java integration with JDTLS
- `multi_project_workspace_e2e_test.go` - Multi-project workspace testing
- Real client comprehensive tests for each language

### Integration Tests  
**Purpose**: Validate component interactions and protocol compliance
**Location**: `tests/integration/`
**Coverage**:
- LSP protocol validation
- Configuration system integration
- MCP server integration
- Transport layer validation (STDIO/TCP)
- Workspace management integration
- SCIP caching integration

### Multi-Project Workspace Tests
**Purpose**: Validate multi-project workspace functionality with sub-project routing
**Location**: `tests/e2e/multi_project_workspace_e2e_test.go` and `tests/integration/workspace/`
**Coverage**:
- Multi-project workspace detection (Go, Python, TypeScript, Java)
- Sub-project LSP request routing and client management
- Workspace-isolated SCIP cache performance
- Cross-project symbol resolution
- Resource allocation and isolation between sub-projects

**Key Test Areas:**
- **Workspace Detection**: Automatic detection of all sub-projects within workspace
- **Request Routing**: LSP requests routed to correct sub-project based on file path
- **Client Management**: Independent LSP client instances per sub-project per language
- **Cache Isolation**: Workspace-specific SCIP cache with 60-87% performance improvement
- **Performance**: Sub-project resolution <1ms, LSP responses <5s, memory <3GB

## Test Infrastructure

### testutils Package
The `tests/e2e/testutils` package provides unified test infrastructure:

```go
// Repository management for real project testing
repoManager := testutils.NewPythonRepositoryManager()
workspaceDir, err := repoManager.SetupRepository()
defer repoManager.Cleanup()

// Multi-project workspace testing
multiManager := testutils.NewMultiProjectManager(config)
workspaceDir, err := multiManager.SetupMultiProjectWorkspace([]string{"go", "python", "java"})
defer multiManager.Cleanup()
```

**Key Components:**
- **Repository Management**: Automated Git cloning and cleanup for real projects
- **Multi-Project Support**: Cross-language workspace creation and testing
- **HTTP Client Testing**: Production-like testing against running gateway
- **Performance Utilities**: Latency, throughput, and resource usage measurement

### Real Project Testing

**Supported Test Repositories:**
- **Python**: faif/python-patterns (design patterns)
- **Go**: golang/example (standard library examples)
- **JavaScript**: chalk/chalk (terminal string styling library)
- **Java**: spring-projects/spring-boot (enterprise patterns)
- **TypeScript**: Real-world TypeScript projects

**Usage:**
```go
// Single-language testing
repoManager := testutils.NewPythonRepositoryManager()
workspaceDir, err := repoManager.SetupRepository()
defer repoManager.Cleanup()

// Multi-project workspace testing
multiManager := testutils.NewMultiProjectManager(config)
workspaceDir, err := multiManager.SetupMultiProjectWorkspace([]string{"go", "python", "java"})
defer multiManager.Cleanup()
```

## Performance Requirements

**E2E Test Performance:**
- LSP Response Time: <5 seconds
- Test Suite Execution: E2E <30min, Integration <15min
- Repository Setup: <60 seconds per language
- Memory Usage: <3GB total during testing
- Success Rate: >95% for essential functionality

**Multi-Project Performance:**
- Sub-project Detection: <5 seconds for typical workspaces
- Sub-project Routing: <1ms per request routing decision  
- Cross-project Symbol Resolution: <10 seconds
- SCIP Cache Performance: 60-87% improvement, 85-90% hit rates

## Development Guidelines

### Writing E2E Tests

**Essential Functionality Focus:**
- Test actual developer workflows (go-to-definition, find references, hover)
- Use real language servers with realistic codebases
- Validate core LSP methods across multiple file types
- Test error conditions that users encounter

```go
// Basic E2E test pattern
repoManager := testutils.NewPythonRepositoryManager()
workspaceDir, err := repoManager.SetupRepository()
defer repoManager.Cleanup()

testFiles, err := repoManager.GetTestFiles()
locations, err := client.Definition(ctx, fileURI, position)
assert.NoError(err)
assert.NotEmpty(locations)
```

### Writing Multi-Project Tests

**Multi-Project Workspace Focus:**
- Test sub-project detection across multiple languages
- Validate request routing to correct sub-project clients
- Test cross-project symbol resolution and references  
- Verify workspace isolation and resource management

```go
// Multi-project test pattern
multiManager := testutils.NewMultiProjectManager(config)
workspaceDir, err := multiManager.SetupMultiProjectWorkspace([]string{"go", "python", "java"})
defer multiManager.Cleanup()

// Test routing and performance
router := testutils.NewRoutingTestHelper(workspaceDir)
routingResults, err := router.ValidateSubProjectRouting(ctx, subProjects)
assert.True(routingResults.AllRoutedCorrectly)
```

### Test Maintenance

**Quality Gates:**
- All E2E tests must use real language servers
- Integration tests must validate actual component interactions
- No testing of implementation details
- Test scenarios must reflect real usage patterns

## Language Server Integration

**Supported Language Servers:**
- **Python**: `pylsp` with faif/python-patterns repository
- **Go**: `gopls` with golang/example repository  
- **JavaScript/TypeScript**: `typescript-language-server` with chalk repository
- **Java**: `jdtls` with Spring Boot repository

**Integration Approach:**
- **Real Project Testing**: Use actual open-source projects for realistic validation
- **Automated Setup**: Repository cloning, validation, and cleanup managed by testutils
- **Performance Testing**: Realistic load testing with actual codebases

## Troubleshooting

### Common Issues
```bash
# Repository setup issues
lspg diagnose --language python
git config --list  # Verify Git access

# Language server problems  
lspg setup all --force
lspg diagnose

# Verbose test output
go test -v -run TestPythonE2E ./tests/e2e/

# Performance diagnostics
go test -bench=. -memprofile=mem.prof ./tests/e2e/
go tool pprof mem.prof
```

### Configuration Validation
```bash
# Configuration validation
lspg config validate

# Protocol testing
lspg diagnose --transport stdio

# Component interaction
go test -v -run TestIntegration ./tests/integration/
```

## Summary

LSP Gateway testing focuses on essential functionality with streamlined infrastructure:

**Core Testing:**
- **E2E Tests**: Real language servers with actual open-source repositories
- **Integration Tests**: Component interactions and protocol compliance  
- **Multi-Project Tests**: Cross-language workspace testing with sub-project routing
- **Performance Tests**: SCIP caching, parallel execution, memory management

**Key Features:**
- **Real Project Testing**: Python (faif/python-patterns), Go (golang/example), JavaScript (chalk), Java (Spring Boot)
- **Multi-Project Support**: Cross-language workspace detection, routing, and isolation
- **Performance Requirements**: <5s LSP responses, <1ms routing, 60-87% SCIP performance improvement
- **Parallel Execution**: Optimized test execution with configurable CPU core usage

**Testing Infrastructure:**
- **testutils Package**: Unified repository management and multi-project workspace creation
- **HTTP Client Testing**: Production-like testing against running gateway
- **Performance Utilities**: Latency, throughput, and resource usage measurement