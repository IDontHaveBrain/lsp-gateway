# Test Guide

LSP Gateway testing guide focused on essential functionality with minimal overhead.

## Testing Philosophy

**Simplified, essential-only testing approach:**
- **Unit Tests**: Core logic only - no integration testing disguised as unit tests
- **E2E Tests**: Real-world usage scenarios only - no enterprise-scale simulation
- **Focus**: Essential functionality for local development workflows

## Quick Reference

### Development Workflows

```bash
# Quick development cycle (<5 minutes)
make test-unit && make test-simple-quick

# Pre-commit validation (5-10 minutes)
make test-unit && make test-lsp-validation-short

# Full validation before PR (15-30 minutes)
make test-unit && make test-lsp-validation && make test-e2e-advanced
```

### Test Commands by Category

#### Unit Tests (<60s)
```bash
make test-unit                # All unit tests
go test ./internal/...        # Direct execution
```

#### Quick Tests (<5min)
```bash
make test-simple-quick        # Basic E2E validation
make test-e2e-quick          # Quick E2E suite
make test-lsp-validation-short # Short LSP validation
make test-mcp-stdio          # MCP STDIO tests
make test-npm-mcp-quick      # Quick NPM-MCP tests
```

#### Standard Tests (5-15min)
```bash
make test-lsp-validation     # Full LSP validation
make test-e2e-setup-cli      # Setup CLI E2E tests
make test-e2e-python         # Python E2E tests
make test-e2e-typescript     # TypeScript E2E tests
make test-e2e-go            # Go E2E tests
make test-e2e-mcp           # Comprehensive MCP tests
make test-npm-mcp           # Full NPM-MCP tests
```

#### Language-Specific Tests (10-15min)
```bash
make test-e2e-java          # Java with JDTLS
make test-java-real         # Real JDTLS integration
make test-python-real       # Real pylsp integration
make test-typescript-real   # Real tsserver integration
```

#### Comprehensive Tests (20-30min)
```bash
make test-e2e-advanced      # Advanced scenarios
make test-e2e-full         # Full test suite
```

## Test Categories

### Unit Tests
**Coverage**: Core gateway logic, configuration, transport, project detection
**Location**: `*_test.go` files co-located with source
**Approach**: Standard Go testing, no complex frameworks

### E2E Tests
**Scenarios**: LSP workflow, multi-language support, protocol validation, error handling
**Location**: `tests/e2e/`, `tests/integration/`
**Approach**: Real language server integration when possible

### Language Server Integration
- **JDTLS** (Java): Eclipse JDT Language Server
- **pylsp** (Python): Python LSP server
- **tsserver** (TypeScript): TypeScript Language Server
- **gopls** (Go): Go language server

## Performance Requirements

- Response Time: <5 seconds
- Throughput: >100 requests/second
- Error Rate: <5%
- Memory Usage: <3GB total, <1GB growth
- Test Suite: Unit tests <60s, E2E <30min

## Test Infrastructure

### Organization (55+ test files)
- **Unit Tests**: 35+ files in `internal/**/*_test.go`
- **Integration Tests**: 8+ files in `tests/integration/`
- **E2E Tests**: 12+ files in `tests/e2e/`

### Mock Infrastructure (8+ mocks)
Streamlined implementations for essential testing:
- Mock LSP Server (87 lines)
- Mock MCP Client (50 lines)
- Mock Transport, Project Detector, Config
- Mock Circuit Breaker, NPM Client, SCIP Cache

### Real Server Testing
Complete integration with actual language servers for production validation.

## Development Guidelines

### Writing Tests

**Unit Tests**
- Test core business logic only
- Use standard Go testing
- Focus on error conditions and edge cases
- Keep tests fast and independent

**E2E Tests**
- Test actual developer workflows
- Use real language servers when possible
- Test essential LSP methods (definition, hover, references)
- Keep scenarios simple and realistic

**Language-Specific Tests**
- Focus on real language servers
- Test common development scenarios
- Validate language-specific features
- Keep test projects minimal

### Test Maintenance
- Delete tests that don't add value
- Simplify over-engineered tests
- Remove redundant infrastructure
- Update tests only when behavior changes

## Troubleshooting

### Diagnostics
```bash
./bin/lsp-gateway diagnose
./bin/lsp-gateway verify
go test -v -run TestSpecificScenario ./tests/
```

### Common Issues

**Language Server Issues**
```bash
# Reinstall language server
./bin/lsp-gateway install <server> --force

# Language-specific diagnostics
./bin/lsp-gateway diagnose --language <language>
```

**Test Failures**
- Configuration: `./bin/lsp-gateway config validate`
- Performance: `./bin/lsp-gateway performance`
- Verbose output: `go test -v -run TestName`

## Summary

LSP Gateway testing focuses on essential functionality for local development:
- Fast unit tests (<60s) for core logic
- Practical E2E tests with real language servers
- Simple infrastructure without over-engineering
- Clear workflows for different development stages

The test suite provides comprehensive coverage while maintaining simplicity and fast feedback loops.