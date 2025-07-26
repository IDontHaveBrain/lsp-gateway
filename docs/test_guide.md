# Test Guide

This guide covers essential testing strategies for LSP Gateway, focusing on E2E and unit tests for critical functionality.

## Testing Philosophy

LSP Gateway uses a **minimal, essential-only** testing approach:
- **Unit Tests**: Core logic and critical components only
- **E2E Tests**: Real-world usage scenarios and workflows
- **Focus**: Essential functionality with minimal overhead

## Quick Start

### Unit Tests (Fast - <60s)
```bash
make test-unit                # Fast unit tests only
go test ./internal/...        # Direct unit test execution
```

### E2E Tests (Comprehensive)
```bash
make test-simple-quick        # Quick E2E validation (1min)
make test-lsp-validation-short # Short LSP validation (2min)
make test-lsp-validation      # Full LSP validation (5min)
```

## Test Categories

### Unit Tests
Essential unit tests cover:
- **Core Gateway Logic** (`internal/gateway/`): Request routing and protocol handling
- **Configuration System** (`internal/config/`): Config validation and loading
- **Transport Layer** (`internal/transport/`): Connection management and circuit breakers
- **CLI Commands** (`internal/cli/`): Command parsing and execution

**Location**: Co-located with source code (`*_test.go` files)

### E2E Tests
Essential E2E scenarios:
- **Basic LSP Workflow**: Definition, references, hover
- **Multi-Language Support**: Go, Python, TypeScript integration
- **Circuit Breaker Behavior**: Failure handling and recovery
- **HTTP/MCP Protocol**: Dual protocol validation

**Location**: `tests/` directory

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

# Circuit breaker tests
make test-circuit-breaker       # 5 minutes

# Java integration (if needed)
make test-jdtls-integration     # 10 minutes
```

## Performance Thresholds

Essential performance requirements:
- **Response Time**: <5 seconds
- **Throughput**: >100 requests/second  
- **Error Rate**: <5%
- **Memory Usage**: <3GB total, <1GB growth

## Test Infrastructure

### Test Framework
- **Unit Tests**: Standard Go testing with table-driven tests
- **E2E Tests**: Custom framework with MockMcpClient integration
- **Assertions**: Minimal, focused on critical paths

### Test Data
- **Synthetic Projects**: Generated realistic code structures
- **Mock Responses**: Simulated LSP server responses
- **Error Scenarios**: Controlled failure injection

## Development Guidelines

### Writing Unit Tests
- Test **essential logic only** - avoid testing trivial functions
- Use **table-driven tests** for multiple scenarios
- Focus on **error conditions** and **edge cases**
- Keep tests **fast and independent**

### Writing E2E Tests
- Test **real user workflows** only
- Use **MockMcpClient** for controlled scenarios  
- Validate **critical paths** and **error recovery**
- Include **performance validation** where needed

### Test Maintenance
- **Remove obsolete tests** when refactoring
- **Update tests** when behavior changes
- **Keep test data minimal** and focused
- **Avoid over-testing** trivial functionality

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

This streamlined approach ensures reliable testing while minimizing maintenance overhead and focusing on essential functionality.