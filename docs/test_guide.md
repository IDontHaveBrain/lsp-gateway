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
- Real repository testing with modular repository management system
- Performance validation under realistic conditions

**Key Test Suites:**
- `python_e2e_comprehensive_test.go` - Comprehensive Python testing with unified repository system
- `python_workflow_validation_test.go` - Python workflow validation using modular infrastructure
- `go_basic_e2e_test.go` - Go language server integration
- `javascript_basic_e2e_test.go` - JavaScript/TypeScript validation
- Language-agnostic tests using unified modular repository system

### Integration Tests  
**Purpose**: Validate component interactions and protocol compliance
**Location**: `tests/integration/`
**Coverage**:
- LSP protocol validation
- Configuration system integration
- MCP server integration
- Transport layer validation
- Error handling and recovery

## Modular Repository Management

**Multi-Language E2E Testing System:**

The modular repository system enables consistent testing across programming languages:

```bash
# Language-specific repository managers
make test-python-modular     # Python with faif/python-patterns
make test-go-modular        # Go with golang/example  
make test-js-modular        # JavaScript with microsoft/TypeScript
make test-java-modular      # Java with spring-projects/spring-boot
make test-rust-modular      # Rust with rust-lang/cargo
```

**Benefits:**
- Unified interface for all language testing
- Real repository analysis with Git-based test projects
- Consistent test patterns across languages
- Easy extension to new programming languages
- Automatic repository setup and cleanup

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
// Use modular repository system
repoManager := testutils.NewPythonRepositoryManager()
workspaceDir, err := repoManager.SetupRepository()

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

**Supported Language Servers:**
- **Python**: `pylsp` with real repository testing
- **Go**: `gopls` with golang/example repository
- **JavaScript/TypeScript**: `typescript-language-server` with TypeScript repository
- **Java**: `jdtls` with Spring Boot repository
- **Rust**: `rust-analyzer` with Cargo repository

**Integration Approach:**
- Use actual language server binaries
- Test with real project repositories
- Validate language-specific features
- Performance testing under realistic loads

## Real Repository Testing

**Python Patterns E2E:**
```bash
make test-python-patterns-quick    # Quick validation (5-10min)
make test-python-patterns          # Comprehensive testing (15-20min)
```

**Features:**
- Real Git repository analysis (faif/python-patterns)
- Comprehensive LSP method validation across Python patterns
- Performance benchmarking with realistic codebases
- Automatic repository setup, testing, and cleanup

**Other Language Repositories:**
- Go: golang/example for standard library patterns
- JavaScript: microsoft/TypeScript for complex TypeScript codebase
- Java: spring-projects/spring-boot for enterprise patterns
- Rust: rust-lang/cargo for systems programming patterns

## HttpClient Testing Infrastructure

**Real Server HTTP Testing:**
```go
// Production-like testing against running gateway
config := testutils.HttpClientConfig{
    BaseURL:         "http://localhost:8080",
    EnableLogging:   true,
    EnableRecording: true,
}
client := testutils.NewHttpClient(config)

// Test all LSP methods
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
# Repository setup issues
./bin/lspg diagnose --language python
git config --list  # Verify Git access

# Language server problems  
./bin/lspg install pylsp --force
./bin/lspg verify --language python

# Verbose test output
go test -v -run TestPythonPatternsE2E ./tests/e2e/
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

**Progressive Removal of Unit Tests:**
1. **Phase 1**: Focus new development on E2E/integration tests only
2. **Phase 2**: Remove unit tests for components covered by E2E tests
3. **Phase 3**: Eliminate mock infrastructure and test utilities
4. **Phase 4**: Consolidate remaining tests into essential E2E scenarios

**Timeline:**
- Immediate: Stop writing new unit tests
- Next release: Remove 50% of existing unit tests
- Following releases: Progressive elimination of remaining unit tests
- Final state: E2E and integration tests only

## Summary

LSP Gateway testing is streamlined to focus exclusively on what users experience:

- **E2E Tests**: Real language servers, real repositories, real workflows
- **Integration Tests**: Component interactions and protocol compliance  
- **Essential Focus**: Core LSP functionality that developers depend on
- **Modular System**: Consistent testing across all supported languages
- **Performance Validation**: Realistic load testing with actual codebases

The simplified approach provides comprehensive coverage while eliminating testing overhead and complexity.