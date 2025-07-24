# Performance Testing Infrastructure

This directory contains comprehensive performance tests for the LSP Gateway system, designed to validate enterprise-scale performance and detect regressions.

## Overview

The performance testing infrastructure validates three critical areas:

1. **Large Project Performance** - Handling of enterprise-scale projects with 10,000+ files
2. **Concurrent Request Performance** - 100+ concurrent LSP requests with load balancing
3. **Memory Usage Performance** - Memory optimization, leak detection, and GC efficiency

## Test Structure

```
tests/performance/
├── README.md                           # This documentation
├── large_project_performance_test.go   # Large project handling tests
├── concurrent_request_performance_test.go # Concurrent request load tests  
├── memory_usage_performance_test.go    # Memory usage and leak detection
├── performance_test_suite.go           # Comprehensive test suite coordinator
└── results/                            # Test results and baselines
    ├── baseline_results.json           # Baseline performance metrics
    ├── results_YYYYMMDD_HHMMSS.json   # Historical test results
    └── report_YYYYMMDD_HHMMSS.txt     # Human-readable reports
```

## Enterprise-Scale Performance Requirements

The tests validate the following enterprise performance requirements:

### Large Project Performance
- **Project Detection Time**: ≤ 30 seconds for 10,000+ file projects
- **Memory Usage**: ≤ 2GB for large project operations
- **Concurrent Projects**: Handle 20+ concurrent large projects
- **Cache Efficiency**: ≥ 75% cache hit ratio
- **Server Pool Management**: Efficient server creation/destruction

### Concurrent Request Performance
- **Throughput**: ≥ 100 requests/second sustained
- **Concurrency**: Handle 100+ concurrent requests
- **Response Time**: P95 ≤ 5 seconds, P99 ≤ 10 seconds
- **Error Rate**: ≤ 5% under normal load
- **Load Balancing**: Efficient distribution across servers
- **Circuit Breaker**: Effective failure handling

### Memory Usage Performance
- **Peak Memory**: ≤ 3GB under high load
- **Memory Leaks**: No severe leaks in long-running scenarios
- **GC Efficiency**: ≥ 70% garbage collection efficiency
- **Resource Cleanup**: Proper cleanup of project resources
- **Memory Pressure**: Graceful handling of memory constraints

## Running Performance Tests

### Full Performance Suite

Run the comprehensive performance test suite:

```bash
# Run complete performance validation (90+ minutes)
go test -v ./tests/performance -run TestEnterpriseScalePerformanceValidation -timeout 2h

# Run with short mode for faster validation
go test -v ./tests/performance -short
```

### Individual Test Categories

Run specific performance test categories:

```bash
# Large project performance tests
go test -v ./tests/performance -run TestLargeProjectPerformanceSuite -timeout 45m

# Concurrent request performance tests  
go test -v ./tests/performance -run TestConcurrentRequestPerformanceSuite -timeout 30m

# Memory usage performance tests
go test -v ./tests/performance -run TestMemoryUsagePerformanceSuite -timeout 60m
```

### Performance Regression Testing

Detect performance regressions against baseline:

```bash
# Run regression detection
go test -v ./tests/performance -run TestPerformanceRegression -timeout 2h
```

### Benchmarking

Run performance benchmarks:

```bash
# Benchmark full performance suite
go test -v ./tests/performance -bench=BenchmarkFullPerformanceSuite -benchtime=10s

# Benchmark specific components
go test -v ./tests/performance -bench=BenchmarkLargeProjectPerformance -benchtime=30s
go test -v ./tests/performance -bench=BenchmarkConcurrentRequestPerformance -benchtime=30s
go test -v ./tests/performance -bench=BenchmarkMemoryUsagePerformance -benchtime=30s
```

## Test Scenarios

### Large Project Performance Scenarios

1. **Project Detection Performance**
   - 5,000-12,000 file projects across multiple languages
   - Deep directory structures (8-12 levels)
   - Complex project types (monorepo, microservices, polyglot)

2. **Memory Usage During Large Operations**
   - Memory consumption patterns for different project sizes
   - Memory leak detection over extended periods
   - Resource cleanup validation

3. **Concurrent Project Handling**
   - 20+ concurrent large projects
   - Cross-language project detection
   - Server pool management efficiency

4. **Cache Performance**
   - Cache hit/miss ratios with large datasets
   - Cache efficiency under memory pressure
   - Multi-project cache coordination

### Concurrent Request Performance Scenarios

1. **Load Testing Profiles**
   - **Spike Load**: 200 concurrent users, 5s ramp-up
   - **Sustained Load**: 100 concurrent users, 120s duration
   - **Stress Load**: 500 concurrent users, high pressure

2. **Request Type Distribution**
   - `textDocument/definition` (10ms avg)
   - `textDocument/references` (20ms avg)  
   - `textDocument/documentSymbol` (15ms avg)
   - `workspace/symbol` (30ms avg)
   - `textDocument/hover` (5ms avg)
   - `textDocument/completion` (25ms avg)

3. **Load Balancing Validation**
   - Round-robin server distribution
   - Response time consistency across servers
   - Failover and recovery testing

4. **Circuit Breaker Testing**
   - Failure threshold configuration
   - Recovery time measurement
   - False positive detection

### Memory Usage Performance Scenarios

1. **Memory Pressure Testing**
   - **Low Pressure**: 5 projects, 20 requests
   - **Medium Pressure**: 15 projects, 50 requests
   - **High Pressure**: 25 projects, 100 requests

2. **Long-Running Leak Detection**
   - 5-45 minute continuous operations
   - Memory growth pattern analysis
   - Severity classification (none/minor/moderate/severe)

3. **Garbage Collection Analysis**
   - GC pause time measurement
   - Memory reclamation efficiency
   - GC frequency under different loads

4. **Resource Cleanup Validation**
   - Project resource lifecycle management
   - Server connection cleanup
   - File handle and memory cleanup

## Performance Metrics

### Collected Metrics

The tests collect comprehensive performance metrics:

```go
type PerformanceTestResults struct {
    Timestamp                   time.Time
    SystemInfo                  *SystemInfo
    LargeProjectResults         *LargeProjectResults
    ConcurrentRequestResults    *ConcurrentRequestResults  
    MemoryUsageResults          *MemoryUsageResults
    OverallPerformanceScore     float64
    RegressionDetected          bool
    RegressionDetails           []string
}
```

### Key Performance Indicators (KPIs)

1. **Overall Performance Score**: Weighted composite score (0-100)
   - Large Project Performance: 30% weight
   - Concurrent Request Performance: 40% weight  
   - Memory Usage Performance: 30% weight

2. **Response Time Percentiles**: P50, P95, P99 response times
3. **Throughput Metrics**: Requests per second sustained
4. **Resource Utilization**: Memory, CPU, goroutines
5. **Error Rates**: Request failure percentages
6. **Efficiency Metrics**: Cache hit ratios, GC efficiency

### Regression Detection

The system automatically detects performance regressions with configurable thresholds:

- **Default Threshold**: 10% performance degradation
- **Baseline Comparison**: Against previous best results
- **Multi-Metric Analysis**: Comprehensive regression detection across all metrics
- **Severity Classification**: Minor/moderate/severe regression levels

## Results and Reporting

### Test Results

Results are automatically saved in JSON format:

- **Current Results**: `results_YYYYMMDD_HHMMSS.json`
- **Baseline Results**: `baseline_results.json` (auto-updated)
- **Historical Tracking**: All results preserved for trend analysis

### Performance Reports

Human-readable reports are generated automatically:

```
Performance Test Report
=======================
Timestamp: 2024-01-15 14:30:45
Overall Performance Score: 87.3/100

System Information:
- OS: linux amd64
- CPUs: 8
- Go Version: go1.21.5
- Total Memory: 16384MB

Large Project Performance:
- Project Detection Time: 18500ms
- Memory Usage: 1847MB
- Cache Efficiency: 78.5%
- Server Pool Efficiency: 85.2%

...
```

### Continuous Integration Integration

Performance tests can be integrated into CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
- name: Run Performance Tests
  run: |
    go test -v ./tests/performance -run TestEnterpriseScalePerformanceValidation -timeout 2h
    if [ $? -ne 0 ]; then
      echo "Performance tests failed or regressions detected"
      exit 1
    fi
```

## Customization and Extension

### Adding New Performance Tests

1. **Create Test File**: Add new `*_performance_test.go` file
2. **Implement Test Structure**: Follow existing patterns
3. **Use Framework**: Leverage `framework.MultiLanguageTestFramework`
4. **Add to Suite**: Register in `performance_test_suite.go`

### Configuring Thresholds

Modify performance thresholds in test constructors:

```go
test := &LargeProjectPerformanceTest{
    MaxProjectDetectionTime:    30 * time.Second,  // Customize
    MaxMemoryUsageBytes:        2 * GB,            // Customize
    MinThroughputReqPerSec:     100.0,             // Customize
}
```

### Custom Load Profiles

Define custom load testing profiles:

```go
customProfile := LoadProfile{
    Name:               "custom_load",
    ConcurrentUsers:    150,
    RequestsPerUser:    75,
    RampUpTime:         15 * time.Second,
    SustainTime:        90 * time.Second,
    RampDownTime:       15 * time.Second,
    ThinkTime:          150 * time.Millisecond,
}
```

## Troubleshooting

### Common Issues

1. **Test Timeouts**: Increase timeout values for slower systems
2. **Memory Limits**: Ensure sufficient system memory (≥8GB recommended)
3. **File Limits**: Check ulimit settings for large project tests
4. **Concurrency Limits**: Verify system can handle high goroutine counts

### Performance Debugging

1. **Enable Profiling**: Use Go's built-in profiler
2. **Memory Analysis**: Use `go tool pprof` for memory analysis
3. **Trace Analysis**: Enable execution tracing for detailed analysis
4. **Metrics Collection**: Leverage built-in performance profiler

### Environment Requirements

- **Memory**: Minimum 8GB RAM, 16GB+ recommended
- **CPU**: Multi-core processor recommended for concurrency tests
- **Disk**: SSD recommended for large project operations
- **OS**: Linux/macOS/Windows supported

## Integration with Existing Tests

The performance tests integrate seamlessly with the existing test framework:

- **Shared Utilities**: Leverages `tests/framework/` components
- **Mock Servers**: Uses existing LSP server mocks
- **Project Generation**: Extends existing project generators
- **Test Patterns**: Follows established testing conventions

This comprehensive performance testing infrastructure ensures the LSP Gateway system can handle enterprise-scale workloads while maintaining optimal performance characteristics.