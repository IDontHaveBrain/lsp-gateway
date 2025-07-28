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
# Quick development cycle (<5 minutes)
make test-simple-quick

# Pre-commit validation (5-10 minutes)  
make test-e2e-quick && make test-integration-quick

# Full validation before PR (15-30 minutes)
make test-e2e-full && make test-integration
```

### Essential Test Commands

#### Quick E2E Tests (<5min)
```bash
make test-simple-quick        # Basic E2E validation
make test-e2e-quick          # Quick E2E suite
make test-python-patterns-quick   # Quick Python validation (5-10min)
```

#### Language-Specific E2E (10-15min)
```bash
make test-python-real        # Real pylsp integration
make test-go-e2e            # Go with gopls
make test-javascript-e2e     # JavaScript/TypeScript
make test-java-real         # Real JDTLS integration
```

#### Comprehensive E2E (20-30min)
```bash
make test-e2e-full          # Full E2E test suite
make test-python-patterns   # Comprehensive Python patterns (15-20min)
make test-python-comprehensive # Complete Python validation (20-25min)
```

#### Integration Tests
```bash
make test-integration       # Core integration tests
make test-mcp-integration   # MCP protocol integration
make test-config-integration # Configuration validation
```

## Test Categories

### E2E Tests
**Purpose**: Validate complete developer workflows with real language servers
**Location**: `tests/e2e/`
**Coverage**: 
- All 6 supported LSP methods (definition, references, hover, documentSymbol, workspaceSymbol, completion)
- Multi-language support (Python, Go, JavaScript, Java, Rust)
- Real repository testing with unified testutils repository management system
- Performance validation under realistic conditions

**Key Test Suites:**
- `python_patterns_real_server_e2e_test.go` - Python testing using testutils.GenericRepoManager
- `go_basic_e2e_test.go` - Go language server integration with testutils
- `javascript_basic_e2e_test.go` - JavaScript/TypeScript validation with testutils
- All tests use unified testutils repository management system

### Integration Tests  
**Purpose**: Validate component interactions and protocol compliance
**Location**: `tests/integration/`
**Coverage**:
- LSP protocol validation
- Configuration system integration
- MCP server integration
- Transport layer validation
- Error handling and recovery

## Unified Repository Management System

**testutils Package - Common Repository Management:**

The `tests/e2e/testutils` package provides a unified repository management system for all language testing:

```go
// Generic repository manager for any language
repoManager := testutils.NewPythonRepositoryManager()
workspaceDir, err := repoManager.SetupRepository()

// Language-specific factory functions available:
// - testutils.NewPythonRepositoryManager()
// - testutils.NewGoRepositoryManager() 
// - testutils.NewJavaScriptRepositoryManager()
// - testutils.NewJavaRepositoryManager()
// - testutils.NewRustRepositoryManager()
```

**Key Components:**
- **GenericRepoManager**: Unified interface implementing RepositoryManager
- **LanguageConfig**: Language-specific configurations (repo URLs, file patterns, test paths)
- **Language Integration Functions**: High-level setup for complete test environments
- **Backward Compatibility**: Adapters maintain compatibility with existing tests

**Benefits:**
- Single codebase for all language repository management
- Predefined configurations for common test repositories
- Automatic Git cloning, validation, and cleanup
- Consistent interface across all programming languages
- Easy extension to new languages via configuration

## testutils Package Structure

**Core Components:**

- **`repository_manager.go`**: GenericRepoManager implementing unified RepositoryManager interface
- **`language_configs.go`**: Predefined configurations and factory functions for all supported languages
- **`language_integration.go`**: High-level functions for complete test environment setup
- **`python_repo_adapter.go`**: Backward compatibility adapter for existing Python tests

**Key Interfaces:**
```go
type RepositoryManager interface {
    SetupRepository() (string, error)
    GetTestFiles() ([]string, error)
    GetWorkspaceDir() string
    Cleanup() error
    ValidateRepository() error
    GetLastError() error
}
```

**Usage Pattern:**
```go
// 1. Create language-specific repository manager
repoManager := testutils.NewPythonRepositoryManager()

// 2. Setup repository (handles Git operations)
workspaceDir, err := repoManager.SetupRepository()
if err != nil {
    return err
}
defer repoManager.Cleanup()

// 3. Get test files automatically
testFiles, err := repoManager.GetTestFiles()

// 4. Use files for LSP testing
for _, testFile := range testFiles {
    fileURI := "file://" + filepath.Join(workspaceDir, testFile)
    // Perform LSP operations...
}
```

## Performance Requirements

**E2E Test Performance:**
- LSP Response Time: <5 seconds
- Test Suite Execution: E2E <30min, Integration <15min
- Repository Setup: <60 seconds per language
- Memory Usage: <3GB total during testing
- Success Rate: >95% for essential functionality

## Development Guidelines

### Writing E2E Tests

**Essential Functionality Focus:**
- Test actual developer workflows (go-to-definition, find references, hover)
- Use real language servers with realistic codebases
- Validate core LSP methods across multiple file types
- Test error conditions that users encounter

**Implementation Patterns:**
```go
// Using the unified repository management system
repoManager := testutils.NewPythonRepositoryManager()
workspaceDir, err := repoManager.SetupRepository()
defer repoManager.Cleanup()

// Get test files automatically
testFiles, err := repoManager.GetTestFiles()

// Test real LSP functionality
locations, err := client.Definition(ctx, fileURI, position)
assert.NoError(err)
assert.NotEmpty(locations)
```

### Writing Integration Tests

**Component Interaction Validation:**
- Test protocol compliance (JSON-RPC, MCP)
- Validate configuration loading and validation
- Test transport layer reliability
- Verify error handling and recovery mechanisms

**Focus Areas:**
- Configuration system integration
- Protocol message handling
- Connection management
- Resource cleanup

### Test Maintenance

**Continuous Simplification:**
- Remove unit tests progressively
- Eliminate redundant test infrastructure
- Consolidate similar test scenarios
- Focus on user-facing functionality only

**Quality Gates:**
- All E2E tests must use real language servers
- Integration tests must validate actual component interactions
- No testing of implementation details
- Test scenarios must reflect real usage patterns

## Language Server Integration

**Language Server Integration via testutils:**
- **Python**: `pylsp` with faif/python-patterns repository
- **Go**: `gopls` with golang/example repository  
- **JavaScript/TypeScript**: `typescript-language-server` with TypeScript repository
- **Java**: `jdtls` with Spring Boot repository
- **Rust**: `rust-analyzer` with Cargo repository

**Integration Approach:**
- **Unified Repository Management**: testutils.GenericRepoManager handles all repository operations
- **Language-Specific Configurations**: Predefined settings for each language server
- **Automatic Setup**: Repository cloning, validation, and cleanup managed by testutils
- **Real Project Testing**: Use actual open-source projects for realistic validation
- **Performance Testing**: Realistic load testing with actual codebases

## Real Repository Testing with testutils

**Unified Repository Testing:**
```bash
# Language-specific E2E tests using common testutils system
make test-python-patterns-quick    # Python with faif/python-patterns
make test-go-e2e                   # Go with golang/example
make test-javascript-e2e           # JavaScript with microsoft/TypeScript
```

**testutils Repository Management Features:**
- **Automatic Repository Setup**: Git cloning with configurable timeouts
- **Language-Specific Configurations**: Predefined configs for Python, Go, JS, Java, Rust
- **File Discovery**: Automatic test file detection based on patterns and paths
- **Validation**: Repository structure and content validation
- **Cleanup Management**: Automatic workspace cleanup with error handling

**Supported Test Repositories:**
- **Python**: faif/python-patterns (design patterns)
- **Go**: golang/example (standard library examples)
- **JavaScript**: microsoft/TypeScript (complex TypeScript codebase)
- **Java**: spring-projects/spring-boot (enterprise patterns)
- **Rust**: rust-lang/cargo (systems programming)

**Usage Example:**
```go
// Create repository manager with predefined configuration
repoManager := testutils.NewPythonRepositoryManager()
workspaceDir, err := repoManager.SetupRepository()
defer repoManager.Cleanup()

// Repository manager handles all Git operations automatically
testFiles, err := repoManager.GetTestFiles()
// Test files are discovered based on language configuration
```

## HttpClient Testing Infrastructure

**Real Server HTTP Testing with testutils:**
```go
// Setup repository using unified system
repoManager := testutils.NewPythonRepositoryManager()
workspaceDir, err := repoManager.SetupRepository()
defer repoManager.Cleanup()

// Production-like testing against running gateway
config := testutils.HttpClientConfig{
    BaseURL:         "http://localhost:8080",
    EnableLogging:   true,
    EnableRecording: true,
}
client := testutils.NewHttpClient(config)

// Test all LSP methods with real repository files
testFiles, _ := repoManager.GetTestFiles()
fileURI := "file://" + filepath.Join(workspaceDir, testFiles[0])
locations, err := client.Definition(ctx, fileURI, position)
symbols, err := client.WorkspaceSymbol(ctx, "query")
hover, err := client.Hover(ctx, fileURI, position)
```

**Performance Testing:**
- Concurrent request validation
- Latency and throughput measurement  
- Success rate monitoring
- Resource usage tracking

## Troubleshooting

### E2E Test Failures
```bash
# Repository setup issues - testutils handles Git operations
./bin/lspg diagnose --language python
git config --list  # Verify Git access for testutils repository cloning

# Language server problems  
./bin/lspg install pylsp --force
./bin/lspg verify --language python

# Verbose test output with testutils debugging
go test -v -run TestPythonPatternsE2E ./tests/e2e/

# Debug repository manager issues
# testutils.GenericRepoManager provides detailed logging when EnableLogging=true
```

### Integration Test Issues
```bash
# Configuration validation
./bin/lspg config validate

# Protocol testing
./bin/lspg diagnose --transport stdio

# Component interaction
go test -v -run TestIntegration ./tests/integration/
```

### Performance Issues
```bash
# Performance diagnostics
./bin/lspg performance

# Resource monitoring
go test -bench=. -memprofile=mem.prof ./tests/e2e/
go tool pprof mem.prof
```

## Migration Path

**Unified Testing with testutils:**
1. **Current State**: All E2E tests use unified testutils repository management
2. **Repository Management**: Single GenericRepoManager interface for all languages
3. **Configuration System**: Predefined language configurations eliminate custom setup
4. **Backward Compatibility**: Existing tests continue working with adapter pattern

**Benefits:**
- Consistent repository management across all languages
- Reduced code duplication in test setup
- Easier maintenance and extension to new languages
- Automatic Git operations with proper error handling
- Standardized test file discovery and validation

## Summary

LSP Gateway testing is streamlined with unified testutils repository management:

- **E2E Tests**: Real language servers, real repositories via testutils.GenericRepoManager
- **Integration Tests**: Component interactions and protocol compliance  
- **Unified Repository Management**: Single testutils package handles all language repository operations
- **Consistent Interface**: RepositoryManager interface across Python, Go, JavaScript, Java, Rust
- **Automated Operations**: Git cloning, file discovery, validation, and cleanup handled by testutils
- **Performance Validation**: Realistic load testing with actual codebases from open-source projects

The testutils approach provides comprehensive coverage with consistent repository management across all languages.