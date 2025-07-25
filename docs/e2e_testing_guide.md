# E2E Testing Guide

This guide provides comprehensive End-to-End testing strategies for LSP Gateway, including both automated test frameworks and manual testing approaches.

## Overview

LSP Gateway supports dual-protocol architecture (HTTP JSON-RPC + MCP) with comprehensive E2E testing capabilities:

- **Automated E2E Framework**: MockMcpClient-based testing with comprehensive scenarios
- **Manual Testing**: HTTP and MCP protocol validation
- **Performance Testing**: Load testing and performance benchmarking
- **Multi-Language Testing**: Go, Python, TypeScript, Java integration testing

## Quick Start

### Automated E2E Testing (Recommended)

Use the comprehensive E2E test framework with MockMcpClient integration:

```bash
# Quick validation (5 minutes)
make test-e2e-quick

# Full comprehensive testing (30 minutes)
make test-e2e-full

# Protocol-specific testing
make test-e2e-mcp          # MCP protocol tests (10 min)
make test-e2e-http         # HTTP JSON-RPC tests (10 min)

# Specialized testing
make test-e2e-performance  # Performance tests (20 min)
make test-e2e-workflow     # Workflow tests (15 min)
```

### Manual Testing

For manual validation and debugging:

```bash
# Basic setup and health check
make local
./bin/lsp-gateway setup all
./bin/lsp-gateway server --config config.yaml &
curl -f http://localhost:8080/health

# Test LSP methods
curl -X POST http://localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "textDocument/definition",
    "params": {
      "textDocument": {"uri": "file:///path/to/main.go"},
      "position": {"line": 10, "character": 5}
    }
  }'
```

## E2E Test Framework

### Test Categories

The automated E2E framework includes:

1. **LSP Workflow Tests** (`lsp_workflow_e2e_test.go`)
   - Definition navigation workflows
   - Code exploration scenarios
   - Multi-language navigation
   - Error recovery testing

2. **Circuit Breaker Tests** (`circuit_breaker_e2e_test.go`)
   - State transition testing (Closed → Open → HalfOpen)
   - Error categorization and retry logic
   - Performance impact measurement

3. **Performance Tests** (`performance_e2e_test.go`)
   - Load testing with concurrent users
   - Latency analysis (P50, P95, P99)
   - Memory usage and leak detection
   - Throughput scaling analysis

4. **Multi-Language Tests** (`multi_language_e2e_test.go`)
   - Cross-language navigation
   - Polyglot project testing
   - Server failover scenarios

### MockMcpClient Integration

The E2E framework uses MockMcpClient for:
- **Response Simulation**: Realistic LSP server responses
- **Error Injection**: Network, timeout, server errors
- **Circuit Breaker Testing**: State management and recovery
- **Metrics Validation**: Connection and performance metrics

### Key Features

- **Comprehensive Coverage**: 16+ test scenarios covering real-world workflows
- **Concurrent Testing**: Up to 500 concurrent users for load testing
- **Performance Validation**: Response time <5s, throughput >100 req/sec
- **Multi-Language Support**: Go, Python, TypeScript, Java
- **Advanced Reporting**: JSON, HTML, CSV reports with historical tracking

## Performance Thresholds

The E2E tests validate against these production thresholds:

- **Response Time**: Maximum 5 seconds per request
- **Throughput**: Minimum 100 requests/second
- **Error Rate**: Maximum 5% error rate
- **Memory Usage**: Maximum 3GB, 1GB growth limit
- **Circuit Breaker**: <10% performance overhead

## Test Execution Strategies

### Development Workflow
```bash
# Quick validation during development
make test-e2e-quick

# Comprehensive testing before PR
make test-e2e-full
```

### CI/CD Integration
```bash
# Parallel execution with timeout
go test -v -timeout 30m -parallel 4 ./tests/e2e/...

# JSON reporting for CI
go test -json ./tests/e2e/... > e2e_results.json
```

### Debugging Failed Tests
```bash
# System diagnostics
./bin/lsp-gateway diagnose

# Verbose test execution
go test -v -run TestSpecificScenario ./tests/e2e/

# Log analysis
./bin/lsp-gateway server --verbose
```

## Multi-Language Testing

### Supported Languages
- **Go**: Definition, references, hover, symbols
- **Python**: Complete LSP method support
- **TypeScript**: Frontend/backend integration
- **Java**: Enterprise application testing

### Cross-Language Scenarios
- Polyglot project navigation
- Microservices communication
- Full-stack application workflows
- Language server failover testing

## Troubleshooting

### Common Issues

**Test Failures:**
```bash
# Check system status
./bin/lsp-gateway status
./bin/lsp-gateway verify

# Validate configuration
./bin/lsp-gateway config validate
```

**Performance Issues:**
```bash
# Run focused performance tests
make test-e2e-performance

# Analyze specific scenarios
go test -bench=. ./tests/e2e/
```

**Circuit Breaker Issues:**
```bash
# Test circuit breaker behavior
go test -run TestCircuitBreaker ./tests/e2e/
```

## Integration with Existing Tests

The E2E framework integrates with existing test infrastructure:

```bash
# Full test suite (includes E2E)
make test

# Legacy test commands still work
make test-simple-quick
make test-lsp-validation
make test-integration
```

## Development and Extension

### Adding New Test Scenarios

1. Create test in appropriate E2E test file
2. Use MockMcpClient for response simulation
3. Add to test suite execution
4. Update documentation

### Custom MockMcpClient Configuration

```go
mockClient := mocks.NewMockMcpClient()
mockClient.QueueResponse(customResponse)
mockClient.SetCircuitBreakerState(mcp.CircuitOpen)
mockClient.UpdateMetrics(total, success, failed, timeouts, errors, latency)
```

## Best Practices

- **Use automated E2E tests** for regular validation
- **Run performance tests** before major releases
- **Test multi-language scenarios** for polyglot projects
- **Validate circuit breaker behavior** under load
- **Monitor test execution times** and optimize as needed

This comprehensive E2E testing approach ensures LSP Gateway reliability across all supported protocols, languages, and usage scenarios.