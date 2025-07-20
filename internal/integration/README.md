# Integration Test Infrastructure

This package provides comprehensive integration testing infrastructure for the LSP Gateway project, supporting end-to-end testing, performance testing, and shared utilities for realistic testing scenarios.

## Overview

The integration testing infrastructure consists of several key components:

- **Test Environment Management**: Automated setup/teardown of gateway and mock servers
- **Mock LSP Servers**: Configurable mock language servers for testing
- **Performance Monitoring**: Comprehensive performance metrics collection
- **Request Generation**: Realistic LSP request generation and validation
- **HTTP Client Pool**: Concurrent HTTP client management
- **Test Fixtures**: Test data and configuration management

## Quick Start

### Basic Integration Test

```go
func TestBasicIntegration(t *testing.T) {
    suite := testutil.NewIntegrationTestSuite(t, &testutil.IntegrationTestConfig{
        TestName: "basic_test",
        Environment: &testutil.TestEnvironmentConfig{
            Languages:         []string{"go", "python"},
            EnableMockServers: true,
        },
    })
    defer suite.Cleanup()

    if err := suite.RunBasicFunctionalTests(); err != nil {
        t.Fatalf("Tests failed: %v", err)
    }
}
```

### Load Testing

```go
func TestLoadTest(t *testing.T) {
    suite := testutil.NewIntegrationTestSuite(t, &testutil.IntegrationTestConfig{
        TestName:        "load_test",
        Performance:     true,
        ConcurrentUsers: 20,
        TestDuration:    60 * time.Second,
        FailureThresholds: &testutil.FailureThresholds{
            MaxErrorRate:  0.05, // 5%
            MaxLatencyP95: 100 * time.Millisecond,
            MinThroughput: 50.0, // 50 RPS
        },
    })
    defer suite.Cleanup()

    if err := suite.RunLoadTest(); err != nil {
        t.Fatalf("Load test failed: %v", err)
    }
}
```

## Components

### 1. Test Environment (`environment.go`)

Manages complete test environments with gateway and mock servers.

**Key Features:**
- Automatic port allocation
- Mock LSP server management
- Gateway lifecycle management
- Resource cleanup
- Configuration templating

**Usage:**
```go
env := SetupTestEnvironment(t, &TestEnvironmentConfig{
    Languages:         []string{"go", "python"},
    EnableMockServers: true,
    EnablePerformance: true,
})
defer env.Cleanup()

baseURL := env.BaseURL()
gateway := env.Gateway()
```

### 2. Mock LSP Server (`mock_lsp_server.go`)

Configurable mock language servers that simulate real LSP servers.

**Key Features:**
- Multiple language support (Go, Python, TypeScript, Java)
- Configurable response latency and error rates
- Request/response logging
- Custom response patterns
- Failure mode simulation

**Usage:**
```go
mockServer := NewMockLSPServer(t, &MockLSPServerConfig{
    Language:        "go",
    Transport:       "stdio",
    ResponseLatency: 10 * time.Millisecond,
    ErrorRate:       0.05, // 5% error rate
    LogRequests:     true,
})
defer mockServer.Stop()
```

### 3. Performance Monitoring (`performance.go`)

Comprehensive performance metrics collection and analysis.

**Metrics Collected:**
- Request latency (min, max, average, percentiles)
- Throughput (requests per second)
- Memory usage and peak memory
- Goroutine count
- GC statistics
- Error rates

**Usage:**
```go
monitor := NewPerformanceMonitor(t)
monitor.Start()

// Execute tests...
measurement := monitor.StartLatencyMeasurement()
// ... perform operation ...
measurement.End()

metrics := monitor.Stop()
fmt.Printf("Performance Report:\n%s", monitor.Report())
```

### 4. Request Generation (`requests.go`)

Realistic LSP request generation with validation.

**Supported LSP Methods:**
- `textDocument/definition`
- `textDocument/hover`
- `textDocument/references`
- `textDocument/documentSymbol`
- `workspace/symbol`

**Usage:**
```go
generator := NewRequestGenerator("http://localhost:8080")

// Generate single request
patterns := generator.GetDefaultRequestPatterns()
request := generator.GenerateRequest(patterns)
response, err := generator.SendRequest(ctx, request)

// Generate realistic sequence
sequence := generator.GenerateRealisticSequence()
responses, err := generator.SendSequence(ctx, sequence)
```

### 5. HTTP Client Pool (`http_client.go`)

Concurrent HTTP client management with statistics.

**Features:**
- Connection pooling and reuse
- Automatic retry with backoff
- Request/response statistics
- Concurrent request execution
- Batch request processing

**Usage:**
```go
pool := NewHTTPClientPool(&HTTPClientPoolConfig{
    BaseURL:  "http://localhost:8080",
    PoolSize: 10,
    Timeout:  30 * time.Second,
})

// Send single request
response, err := pool.SendJSONRPCRequest(ctx, request)

// Execute batch
results, err := pool.ExecuteBatch(ctx, &BatchRequestConfig{
    Requests:       requests,
    ConcurrentReqs: 20,
    Timeout:        5 * time.Second,
})
```

### 6. Test Fixtures (`fixtures/manager.go`)

Test data and configuration management.

**Features:**
- Temporary workspace creation
- Source code file generation
- Configuration templates
- Expected response datasets
- Test scenario definitions

**Usage:**
```go
fixtures := NewFixtureManager(t, "")
workspace := fixtures.CreateTempWorkspace()
err := fixtures.CreateSourceFiles(workspace)

config := fixtures.GetGatewayConfig(port, mockServers)
scenarios := fixtures.GetStandardTestScenarios()
```

### 7. Integration Test Suite (`integration.go`)

High-level test orchestration combining all components.

**Test Types:**
- Basic functional tests
- Load tests
- Reliability tests
- Concurrency tests
- Comprehensive tests

**Usage:**
```go
suite := NewIntegrationTestSuite(t, &IntegrationTestConfig{
    TestName:        "comprehensive",
    Performance:     true,
    ConcurrentUsers: 15,
    TestDuration:    60 * time.Second,
})
defer suite.Cleanup()

err := suite.RunComprehensiveTest()
```

## Configuration

### Environment Configuration

```go
type TestEnvironmentConfig struct {
    Languages          []string          // Languages to support
    HTTPPort           int               // Custom HTTP port
    EnableMockServers  bool              // Enable mock LSP servers
    EnablePerformance  bool              // Enable performance monitoring
    CustomConfig       *config.GatewayConfig // Custom gateway config
    ResourceLimits     *ResourceLimits   // Resource constraints
    NetworkConditions  *NetworkConditions // Network simulation
}
```

### Performance Thresholds

```go
type FailureThresholds struct {
    MaxErrorRate        float64       // Maximum acceptable error rate (0.0-1.0)
    MaxLatencyP95       time.Duration // Maximum P95 latency
    MaxLatencyP99       time.Duration // Maximum P99 latency
    MinThroughput       float64       // Minimum RPS
    MaxMemoryUsageMB    float64       // Maximum memory usage
    MaxGoroutines       int           // Maximum goroutine count
}
```

## Running Tests

### Component Tests
```bash
# Run all integration tests
go test ./internal/integration -v

# Run specific test types
go test ./internal/integration -v -run TestBasic
go test ./internal/integration -v -run TestLoad
go test ./internal/integration -v -run TestComprehensive

# Skip long-running tests
go test ./internal/integration -v -short
```

### Performance Tests
```bash
# Run performance benchmarks
go test ./internal/integration -bench=. -benchmem

# Run with CPU profiling
go test ./internal/integration -bench=. -cpuprofile=cpu.prof

# Run with memory profiling
go test ./internal/integration -bench=. -memprofile=mem.prof
```

### Load Tests
```bash
# Light load test (30s, 5 users)
go test ./internal/integration -v -run TestLoadTest

# Custom load test parameters
LOAD_DURATION=120s LOAD_USERS=50 go test ./internal/integration -v -run TestLoadTest
```

## Best Practices

### 1. Test Isolation
- Each test should use its own TestEnvironment
- Always call `defer suite.Cleanup()` or `defer env.Cleanup()`
- Use temporary directories for file operations

### 2. Resource Management
- Set appropriate timeouts for operations
- Monitor resource usage in long-running tests
- Use performance thresholds to catch regressions

### 3. Error Handling
- Test both success and error scenarios
- Validate error responses are properly formatted
- Test edge cases and boundary conditions

### 4. Performance Testing
- Establish baseline performance metrics
- Use realistic request patterns
- Test under various load conditions
- Monitor for memory leaks and resource exhaustion

### 5. Mock Server Configuration
- Configure realistic response latencies
- Test with various error rates
- Validate protocol compliance
- Use different mock configurations for different test scenarios

## Troubleshooting

### Common Issues

1. **Port Conflicts**: Use `testutil.AllocateTestPort(t)` for automatic port allocation
2. **Resource Leaks**: Always use cleanup functions and defer statements
3. **Timeouts**: Adjust timeouts based on test complexity and CI environment
4. **Mock Server Issues**: Check binary compilation and configuration

### Debug Information

```go
// Enable detailed logging
t.Logf("Environment URL: %s", env.BaseURL())
t.Logf("HTTP Stats: %s", httpClient.GetStats().String())
t.Logf("Performance: %s", perfMonitor.Report())

// Check mock server logs
requestLog := mockServer.GetRequestLog()
responseLog := mockServer.GetResponseLog()
```

## Example Test Scenarios

See `integration_test.go` for comprehensive examples including:
- Basic functional testing
- Load testing with performance validation
- Error scenario testing
- Concurrency testing
- Workspace simulation
- Custom request patterns
- Resource monitoring

## Extension Points

The infrastructure is designed for extensibility:

1. **Custom Mock Servers**: Implement additional language servers
2. **Custom Request Patterns**: Add new LSP methods or validation
3. **Custom Metrics**: Extend performance monitoring
4. **Custom Test Scenarios**: Create domain-specific test patterns
5. **Network Simulation**: Add network condition simulation
6. **Failure Injection**: Add chaos engineering capabilities