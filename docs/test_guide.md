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
make test-python-patterns-quick   # Quick Python patterns validation (5-10min)
make test-typescript-real   # Real tsserver integration
```

#### Comprehensive Tests (20-30min)
```bash
make test-e2e-advanced      # Advanced scenarios
make test-e2e-full         # Full test suite
make test-python-patterns   # Comprehensive Python patterns (15-20min)
make test-python-comprehensive # Complete Python test suite (20-25min)
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

### Python Patterns E2E Testing

Test Python language server integration using real repositories and comprehensive LSP method validation:

```bash
make test-python-patterns-quick    # Quick validation (5-10min)
make test-python-patterns          # Comprehensive patterns testing (15-20min)
make test-python-comprehensive     # Full Python test suite (20-25min)
```

**Features**:
- Real repository analysis using Git-based test projects
- Comprehensive LSP method validation across different Python project types
- Performance benchmarking with realistic codebases
- Integration with existing Python test infrastructure
- Repository-based testing with automated setup and cleanup

**Test Coverage**:
- All 6 supported LSP methods (definition, references, hover, symbols, completion)
- Multiple Python project patterns (Django, Flask, data science, CLI tools)
- Error handling and edge cases with real-world scenarios
- Performance validation under realistic loads

**Troubleshooting**:
- Test failures: Check `./scripts/test-python-patterns.sh --verbose` output
- Repository setup issues: Verify Git access and network connectivity
- Language server problems: Use `./bin/lspg diagnose --language python`
- Timeout issues: Adjust test timeouts in configuration files

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
./bin/lspg diagnose
./bin/lspg verify
go test -v -run TestSpecificScenario ./tests/
```

### Common Issues

**Language Server Issues**
```bash
# Reinstall language server
./bin/lspg install <server> --force

# Language-specific diagnostics
./bin/lspg diagnose --language <language>
```

**Test Failures**
- Configuration: `./bin/lspg config validate`
- Performance: `./bin/lspg performance`
- Verbose output: `go test -v -run TestName`

## Summary

LSP Gateway testing focuses on essential functionality for local development:
- Fast unit tests (<60s) for core logic
- Practical E2E tests with real language servers
- Simple infrastructure without over-engineering
- Clear workflows for different development stages

The test suite provides comprehensive coverage while maintaining simplicity and fast feedback loops.

## HttpClient Testing Infrastructure

**Real server HTTP testing infrastructure for comprehensive LSP Gateway validation.**

### HttpClient Overview

The HttpClient provides production-like testing against running LSP Gateway instances:
- **Real Connections**: Direct HTTP/JSON-RPC to gateway server
- **Performance Metrics**: Latency, throughput, error rates
- **Request Recording**: Full request/response capture for debugging
- **Retry Logic**: Exponential backoff with configurable attempts
- **Connection Pooling**: Efficient resource management

### Quick Start

#### Basic Usage
```go
// Create client with default config
config := testutils.DefaultHttpClientConfig()
client := testutils.NewHttpClient(config)
defer client.Close()

// Health check
err := client.HealthCheck(ctx)

// LSP method calls
locations, err := client.Definition(ctx, "file:///path/to/file.go", testutils.Position{Line: 10, Character: 5})
symbols, err := client.WorkspaceSymbol(ctx, "myFunction")
```

#### Custom Configuration
```go
config := testutils.HttpClientConfig{
    BaseURL:         "http://localhost:8080",
    Timeout:         10 * time.Second,
    MaxRetries:      3,
    RetryDelay:      500 * time.Millisecond,
    EnableLogging:   true,
    EnableRecording: true,
    WorkspaceID:     "test-workspace",
    ProjectPath:     "/path/to/project",
}
client := testutils.NewHttpClient(config)
```

### Test Commands

#### Real Server E2E Tests
```bash
# HTTP connection testing
make test-e2e-http-connection     # Connection validation and health checks

# LSP API method testing
make test-e2e-lsp-api-methods     # All 6 supported LSP methods
make test-e2e-lsp-edge-cases      # Error handling and edge cases  
make test-e2e-lsp-validation      # Request/response validation

# Performance testing
make test-e2e-lsp-performance     # Latency and throughput benchmarks
```

#### Individual Test Files
```bash
# Connection tests
go test -v ./tests/e2e -run TestHttpConnection

# API method tests  
go test -v ./tests/e2e -run TestLSPAPIMethods

# Performance benchmarks
go test -v ./tests/e2e -run TestLSPPerformance -bench=.
```

### Supported LSP Methods

The HttpClient supports all 6 LSP Gateway methods:

#### 1. Go to Definition
```go
positions, err := client.Definition(ctx, fileURI, testutils.Position{Line: 10, Character: 5})
```

#### 2. Find References  
```go
locations, err := client.References(ctx, fileURI, position, true) // includeDeclaration
```

#### 3. Hover Information
```go
hover, err := client.Hover(ctx, fileURI, position)
if hover != nil {
    fmt.Println("Hover content:", hover.Contents)
}
```

#### 4. Document Symbols
```go
symbols, err := client.DocumentSymbol(ctx, fileURI)
for _, symbol := range symbols {
    fmt.Printf("Symbol: %s (kind: %d)\n", symbol.Name, symbol.Kind)
}
```

#### 5. Workspace Symbol Search
```go
symbols, err := client.WorkspaceSymbol(ctx, "function")
```

#### 6. Code Completion
```go
completion, err := client.Completion(ctx, fileURI, position)
if completion != nil {
    for _, item := range completion.Items {
        fmt.Printf("Completion: %s\n", item.Label)
    }
}
```

### Performance Testing

#### Benchmark Execution
```go
// Run concurrent requests
func BenchmarkDefinitionRequests(b *testing.B) {
    client := testutils.NewHttpClient(testutils.DefaultHttpClientConfig())
    defer client.Close()
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            _, err := client.Definition(ctx, fileURI, position)
            if err != nil {
                b.Error(err)
            }
        }
    })
}
```

#### Metrics Collection
```go
// Get performance metrics
metrics := client.GetMetrics()
fmt.Printf("Average latency: %v\n", metrics.AverageLatency)
fmt.Printf("Success rate: %.2f%%\n", 
    float64(metrics.SuccessfulReqs)/float64(metrics.TotalRequests)*100)
```

#### Load Testing Pattern
```go
func TestConcurrentRequests(t *testing.T) {
    client := testutils.NewHttpClient(config)
    defer client.Close()
    
    // Clear metrics
    client.ClearMetrics()
    
    // Run concurrent load
    concurrency := 10
    requestsPerWorker := 50
    
    var wg sync.WaitGroup
    for i := 0; i < concurrency; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := 0; j < requestsPerWorker; j++ {
                client.Definition(ctx, fileURI, position)
            }
        }()
    }
    wg.Wait()
    
    // Validate metrics
    metrics := client.GetMetrics()
    assert.True(t, metrics.AverageLatency < 100*time.Millisecond)
    assert.True(t, metrics.SuccessfulReqs >= concurrency*requestsPerWorker*0.95) // 95% success
}
```

### Request Recording & Debugging

#### Enable Recording
```go
config := testutils.DefaultHttpClientConfig()
config.EnableRecording = true
client := testutils.NewHttpClient(config)

// Make requests
client.Definition(ctx, fileURI, position)

// Get recordings for debugging
recordings := client.GetRecordings()
for _, req := range recordings {
    fmt.Printf("Method: %s, Duration: %v, Error: %s\n", 
        req.Metadata["lsp_method"], req.Duration, req.Error)
}
```

#### Mock Mode Testing
```go
config := testutils.DefaultHttpClientConfig()
config.MockMode = true
client := testutils.NewHttpClient(config)

// Set mock responses
mockLocations := []testutils.Location{
    {URI: "file:///test.go", Range: testutils.Range{...}},
}
client.SetMockResponse("textDocument/definition", mockLocations)

// Test with mocks
locations, err := client.Definition(ctx, fileURI, position)
// Returns mock response
```

### Integration with Test Suites

#### Suite Setup Pattern
```go
type HttpTestSuite struct {
    suite.Suite
    httpClient  *testutils.HttpClient
    gatewayCmd  *exec.Cmd
    tempDir     string
}

func (s *HttpTestSuite) SetupTest() {
    config := testutils.HttpClientConfig{
        BaseURL:         fmt.Sprintf("http://localhost:%d", s.gatewayPort),
        Timeout:         10 * time.Second,
        EnableRecording: true,
    }
    s.httpClient = testutils.NewHttpClient(config)
}

func (s *HttpTestSuite) TearDownTest() {
    if s.httpClient != nil {
        s.httpClient.Close()
    }
}
```

#### Validation Helpers
```go
func (s *HttpTestSuite) ValidateSuccessfulRequest(locations []testutils.Location, err error) {
    s.Require().NoError(err)
    s.Assert().NotEmpty(locations)
    
    // Check metrics
    metrics := s.httpClient.GetMetrics()
    s.Assert().True(metrics.SuccessfulReqs > 0)
    s.Assert().True(metrics.AverageLatency < 5*time.Second)
}
```

### Performance Benchmarks

#### Expected Performance
- **Response Time**: <100ms for definition/references
- **Throughput**: >100 requests/second  
- **Success Rate**: >95% under normal load
- **Memory Growth**: <10MB per 1000 requests

#### Benchmark Commands
```bash
# Run all performance tests
make test-e2e-lsp-performance

# Specific benchmarks
go test -bench=BenchmarkDefinition ./tests/e2e
go test -bench=BenchmarkConcurrent ./tests/e2e
```

### Troubleshooting

#### Connection Issues
```bash
# Test gateway health directly
curl http://localhost:8080/health

# Validate configuration
./bin/lsp-gateway config validate

# Check logs with verbose client
config.EnableLogging = true
```

#### Performance Issues
```go
// Check client metrics
metrics := client.GetMetrics()
fmt.Printf("Connection errors: %d\n", metrics.ConnectionErrors)
fmt.Printf("Timeout errors: %d\n", metrics.TimeoutErrors)
fmt.Printf("Average latency: %v\n", metrics.AverageLatency)
```

The HttpClient testing infrastructure enables comprehensive validation of LSP Gateway under realistic conditions, providing confidence in production behavior through real server testing.