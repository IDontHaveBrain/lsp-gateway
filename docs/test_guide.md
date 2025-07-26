# Test Guide

This guide covers the streamlined testing strategies for LSP Gateway, focusing on essential functionality with minimal overhead.

## Testing Philosophy

LSP Gateway uses a **simplified, essential-only** testing approach:
- **Unit Tests**: Core logic only - no integration testing disguised as unit tests
- **E2E Tests**: Real-world usage scenarios only - no enterprise-scale simulation
- **Simplified Infrastructure**: Basic mocks and fixtures without over-engineering
- **Focus**: Essential functionality that supports local development workflows

## Quick Start

### Unit Tests (Fast - <60s)
```bash
make test-unit                # Fast unit tests only
go test ./internal/...        # Direct unit test execution
```

### E2E Tests (Essential Scenarios)
```bash
make test-simple-quick        # Quick E2E validation (1min)
make test-lsp-validation-short # Short LSP validation (2min)
make test-lsp-validation      # Full LSP validation (5min)
make test-e2e-setup-cli       # Setup CLI E2E tests (5min)
```

## Test Categories

### Unit Tests
Simplified unit tests cover only essential logic:
- **Core Gateway Logic** (`internal/gateway/`): Request routing and protocol handling
- **Configuration System** (`internal/config/`): Config validation and loading  
- **Transport Layer** (`internal/transport/`): Basic connection management
- **Project Detection** (`internal/project/`): Language detection core functionality

**Location**: Co-located with source code (`*_test.go` files)
**Approach**: Standard Go testing, no complex test suites or frameworks

### E2E Tests
Essential E2E scenarios only:
- **Basic LSP Workflow**: Definition, references, hover for real development scenarios
- **Multi-Language Support**: Go, Python, TypeScript integration with actual language servers
- **Protocol Validation**: HTTP JSON-RPC and MCP protocol basics
- **Setup CLI Testing**: Comprehensive binary execution tests for setup automation
- **Simple Error Handling**: Basic failure recovery without complex simulation

**Location**: `tests/e2e/` and `tests/integration/` directories
**Approach**: Real language server integration, no synthetic project generation

### Setup CLI E2E Tests
Comprehensive binary execution tests for setup automation:
- **Real Binary Testing**: Execute actual compiled binary with various command combinations
- **JSON Output Validation**: Parse and validate complex JSON response structures  
- **Command Coverage**: Tests setup all, detect, wizard, template commands with real scenarios
- **Error Handling**: Validates proper error responses for invalid inputs and edge cases
- **Project Integration**: Tests with real project structures (Go, Python, etc.)

**Location**: `tests/e2e/setup_cli_e2e_test.go`
**Approach**: Direct binary execution with comprehensive JSON parsing and validation

## Test Commands

### Development Workflow
```bash
# Quick development validation
make test-unit && make test-simple-quick

# Pre-commit validation
make test-unit && make test-lsp-validation-short

# Full validation before PR
make test-unit && make test-lsp-validation
```

### Specific Test Types
```bash
# Unit tests only
make test-unit

# LSP validation tests
make test-lsp-validation-short  # 2 minutes
make test-lsp-validation        # 5 minutes

# Setup CLI E2E tests
make test-e2e-setup-cli         # 5 minutes

# Basic integration tests
make test-integration           # 3-5 minutes
```

## Performance Thresholds

Essential performance requirements:
- **Response Time**: <5 seconds
- **Throughput**: >100 requests/second  
- **Error Rate**: <5%
- **Memory Usage**: <3GB total, <1GB growth

## Test Infrastructure

### Simplified Test Framework
- **Unit Tests**: Standard Go testing without complex test suites
- **E2E Tests**: Direct integration with real language servers
- **Mocking**: Simple mock implementations with basic functionality
- **Assertions**: Standard Go testing assertions

### Test Data
- **Real Projects**: Simple temporary directories with actual project files
- **Basic Mock Responses**: Minimal LSP response simulation
- **Essential Fixtures**: Small set of realistic LSP responses in JSON files

## Development Guidelines

### Writing Unit Tests
- Test **core business logic only** - skip getters, setters, and trivial functions
- Use **standard Go testing** - no complex test frameworks or suites
- Focus on **error conditions** and **edge cases** that matter
- Keep tests **fast (<60s total)** and **independent**
- Avoid testing implementation details - test behavior

### Writing E2E Tests
- Test **actual developer workflows** only
- Use **real language servers** when possible
- Test **essential LSP methods** - definition, hover, references
- Keep scenarios **simple and realistic**
- Avoid performance testing unless critical

### Writing Setup CLI E2E Tests
- Test **real binary execution** with actual command combinations
- Validate **JSON output structure** and content accuracy
- Cover **error scenarios** and edge cases with proper response validation
- Use **real project structures** for authentic testing environments
- Focus on **command coverage** - setup all, detect, wizard, template scenarios
- Ensure **cross-platform compatibility** where applicable

### Test Maintenance
- **Delete tests** that don't add value or test trivial functionality  
- **Simplify over-engineered tests** - prefer standard Go testing patterns
- **Remove redundant test infrastructure** - avoid custom frameworks
- **Keep test data minimal** - use simple fixtures and real temporary files
- **Update tests only when behavior changes** - not implementation details

## Troubleshooting

### Test Failures
```bash
# System diagnostics
./bin/lsp-gateway diagnose
./bin/lsp-gateway verify

# Verbose test execution
go test -v -run TestSpecificScenario ./tests/
```

### Common Issues
- **Configuration errors**: `./bin/lsp-gateway config validate`
- **Language server issues**: `./bin/lsp-gateway install <server> --force`
- **Performance issues**: `./bin/lsp-gateway performance`

## CI/CD Integration

```bash
# Parallel execution for CI
go test -v -timeout 10m -parallel 4 ./...

# JSON output for reporting
go test -json ./... > test_results.json
```

## Recent Cleanup (2025)

The test suite has been significantly simplified to align with the minimal testing philosophy:

### Removed Components
- **Phase 2 Testing Framework**: Deleted entire enterprise-scale testing infrastructure
- **Over-engineered Mock Infrastructure**: Simplified mock LSP server (2017→87 lines), MCP client mock (567→50 lines)  
- **Synthetic Project Generation**: Removed complex project generators and content generators
- **Performance Testing**: Removed enterprise performance validation and benchmarking
- **Complex Test Frameworks**: Deleted multi-language test framework and performance profilers

### Simplified Components  
- **Unit Tests**: Reduced detector tests (1062→148 lines), memory cache tests (849→162 lines)
- **Makefile**: Removed 20+ Phase 2 test targets and complex test commands
- **Test Infrastructure**: Replaced complex test suites with standard Go testing

### Result
- **70-75% reduction** in test code complexity
- **Faster test execution** - unit tests run in <60 seconds
- **Easier maintenance** - standard Go testing patterns only
- **Clear focus** - essential functionality for local development workflows

This streamlined approach ensures reliable testing while minimizing maintenance overhead and focusing on essential functionality that supports LSP Gateway as a local development tool.