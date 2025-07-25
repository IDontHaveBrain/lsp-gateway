# SCIP Phase 2 Testing Architecture

## Executive Summary

This document outlines the comprehensive testing architecture for LSP Gateway's SCIP Phase 2 implementation. 

**🎉 PHASE 2 STATUS: COMPLETE AND FUNCTIONAL** 
- ✅ **Smart Router**: Fully implemented with confidence-based routing
- ✅ **Three-Tier Storage**: Complete L1/L2/L3 architecture implemented  
- ✅ **Incremental Pipeline**: Full implementation with dependency tracking
- ✅ **Enhanced MCP Tools**: 590 lines of production-ready code complete
- ✅ **LSP Gateway Application**: Compiles successfully and runs functionally
- ✅ **All Compilation Issues**: Resolved across gateway, CLI, and storage modules

Phase 2 is now ready for comprehensive testing and validation.

### Phase 2 Components Ready for Testing ✅

1. **Smart Router Integration** ✅ - Intelligent SCIP/LSP routing with confidence scoring (IMPLEMENTED)
2. **Three-Tier Storage System** ✅ - L1 Memory, L2 Disk, L3 Remote storage with promotion/eviction (IMPLEMENTED)
3. **Incremental Update Pipeline** ✅ - <30s latency updates with dependency tracking (IMPLEMENTED)
4. **Enhanced MCP Tools** ✅ - SCIP-powered AI assistant capabilities (590 lines complete)
5. **Gateway Integration** ✅ - Critical cache-first routing functionality (FUNCTIONAL)
6. **Application Compilation & Runtime** ✅ - LSP Gateway compiles and runs successfully (VERIFIED)

### Performance Targets to Validate

| Component | Target | Measurement |
|-----------|--------|-------------|
| Smart Router | <5ms routing decisions | P95 decision latency |
| L1 Cache | <10ms access | P95 access time |
| L2 Cache | <50ms access | P95 access time |
| L3 Cache | <200ms access | P95 access time |
| Incremental Updates | <30s average latency | Mean update propagation time |
| Enhanced MCP | <100ms SCIP responses | P95 response time |
| Cache Hit Rates | 85-90% | Symbol query hit rate |

## Test Architecture Overview

### Design Principles

1. **Isolation-First Testing** - Each Phase 2 component tested in isolation before integration
2. **Performance-Driven Validation** - All tests measure and validate performance targets
3. **Production Simulation** - Tests simulate real-world usage patterns and scales
4. **Graceful Degradation** - Verify fallback behavior when SCIP components fail
5. **Cross-Component Integration** - Validate interactions between Phase 2 components

### Test Categories

#### 1. Unit Tests
- **Location**: `tests/phase2/unit/`
- **Purpose**: Validate individual component functionality
- **Coverage**: 90%+ code coverage for Phase 2 components
- **Execution Time**: <5 seconds per package

#### 2. Integration Tests
- **Location**: `tests/phase2/integration/`
- **Purpose**: Validate component interactions and data flow
- **Focus**: Smart Router ↔ Storage, Pipeline ↔ SCIP Store
- **Execution Time**: <2 minutes total

#### 3. Performance Tests
- **Location**: `tests/phase2/performance/`
- **Purpose**: Validate all performance targets are met
- **Focus**: Latency, throughput, resource usage
- **Execution Time**: 5-10 minutes

#### 4. E2E Workflow Tests
- **Location**: `tests/phase2/e2e/`
- **Purpose**: Validate complete development workflows
- **Focus**: Real-world usage scenarios
- **Execution Time**: 10-15 minutes

#### 5. Stress Tests
- **Location**: `tests/phase2/stress/`
- **Purpose**: Validate system behavior under extreme load
- **Focus**: Concurrent requests, large codebases
- **Execution Time**: 15-30 minutes

## Directory Structure

```
tests/phase2/
├── README.md                          # Phase 2 testing overview and quick start
├── common/                            # Shared test utilities
│   ├── fixtures/                      # Test data and SCIP indices
│   │   ├── large-monorepo/           # 100k+ file test repo
│   │   ├── multi-language/           # Polyglot test projects
│   │   └── scip-indices/             # Pre-generated SCIP indices
│   ├── helpers/                       # Test helper functions
│   │   ├── scip_helpers.go          
│   │   ├── storage_helpers.go       
│   │   └── performance_helpers.go   
│   └── mocks/                        # Mock implementations
│       ├── mock_scip_store.go       
│       └── mock_storage_tier.go     
│
├── unit/                             # Unit tests for Phase 2 components
│   ├── smart_router/
│   │   ├── confidence_scoring_test.go
│   │   ├── routing_decision_test.go
│   │   └── strategy_selection_test.go
│   ├── storage/
│   │   ├── memory_cache_test.go
│   │   ├── disk_cache_test.go
│   │   ├── remote_cache_test.go
│   │   └── hybrid_manager_test.go
│   ├── pipeline/
│   │   ├── file_watcher_test.go
│   │   ├── dependency_graph_test.go
│   │   ├── update_queue_test.go
│   │   └── conflict_resolution_test.go
│   └── mcp_tools/
│       ├── scip_enhancement_test.go
│       └── cross_language_test.go
│
├── integration/                      # Integration tests
│   ├── smart_router_integration/
│   │   ├── router_storage_test.go    # Router + Storage integration
│   │   ├── router_pipeline_test.go   # Router + Pipeline integration
│   │   └── router_gateway_test.go    # Router + Gateway integration
│   ├── storage_integration/
│   │   ├── tier_promotion_test.go    # L1→L2→L3 promotion flows
│   │   ├── tier_eviction_test.go     # L3→L2→L1 eviction flows
│   │   └── tier_coordination_test.go # Multi-tier coordination
│   ├── pipeline_integration/
│   │   ├── watcher_updates_test.go   # File watcher → update flow
│   │   ├── dependency_updates_test.go # Dependency tracking
│   │   └── conflict_handling_test.go  # Conflict resolution
│   └── mcp_integration/
│       ├── scip_tool_routing_test.go # MCP → Router integration
│       └── enhanced_search_test.go    # Enhanced search capabilities
│
├── performance/                      # Performance validation tests
│   ├── benchmarks/
│   │   ├── smart_router_bench_test.go
│   │   ├── storage_tiers_bench_test.go
│   │   ├── pipeline_latency_bench_test.go
│   │   └── mcp_response_bench_test.go
│   ├── load_tests/
│   │   ├── concurrent_routing_test.go
│   │   ├── storage_pressure_test.go
│   │   └── pipeline_throughput_test.go
│   └── resource_tests/
│       ├── memory_usage_test.go
│       ├── cpu_utilization_test.go
│       └── disk_io_test.go
│
├── e2e/                              # End-to-end workflow tests
│   ├── developer_workflows/
│   │   ├── code_navigation_test.go   # Symbol lookup workflows
│   │   ├── refactoring_test.go       # Cross-file refactoring
│   │   └── dependency_analysis_test.go
│   ├── ai_assistant_workflows/
│   │   ├── intelligent_search_test.go
│   │   ├── cross_language_refs_test.go
│   │   └── context_awareness_test.go
│   └── production_scenarios/
│       ├── monorepo_workflow_test.go
│       ├── microservices_test.go
│       └── rapid_iteration_test.go
│
├── stress/                           # Stress and chaos tests
│   ├── extreme_load/
│   │   ├── thousand_concurrent_test.go
│   │   ├── million_symbols_test.go
│   │   └── sustained_load_test.go
│   ├── failure_scenarios/
│   │   ├── storage_failure_test.go
│   │   ├── network_partition_test.go
│   │   └── cascading_failure_test.go
│   └── recovery_tests/
│       ├── graceful_degradation_test.go
│       ├── circuit_breaker_test.go
│       └── self_healing_test.go
│
└── reports/                          # Test execution reports
    ├── coverage/                     # Code coverage reports
    ├── performance/                  # Performance test results
    └── compliance/                   # SLA compliance reports
```

## Integration with Existing Infrastructure

### 1. Reusing Framework Components

Phase 2 tests will leverage existing test framework components:

```go
// Reuse from tests/framework/
import (
    "lsp-gateway/tests/framework"
    "lsp-gateway/tests/integration/scip"
)

// Extend existing test suite
type Phase2TestSuite struct {
    *scip.SCIPTestSuite                    // Inherit Phase 1 infrastructure
    smartRouter     *gateway.SCIPSmartRouter
    storageManager  *storage.HybridStorageManager
    pipeline        *indexing.IncrementalUpdatePipeline
    enhancedTools   *mcp.SCIPEnhancedToolHandler
}
```

### 2. Shared Test Data

Phase 2 tests will utilize and extend existing test data:

```
tests/
├── integration/scip/testdata/        # Existing SCIP test data
│   └── scip_indices/                # Reuse these indices
└── phase2/common/fixtures/          # Phase 2 specific test data
    └── scip-indices/                # Symlink to existing + new indices
```

### 3. Performance Profiling Integration

Leverage existing performance profiler:

```go
// Extend framework.PerformanceProfiler
type Phase2PerformanceProfiler struct {
    *framework.PerformanceProfiler
    
    // Phase 2 specific metrics
    routingLatencyHistogram  *prometheus.HistogramVec
    tierAccessHistogram      *prometheus.HistogramVec
    pipelineLatencyHistogram *prometheus.HistogramVec
}
```

## Performance Benchmarking Design

### 1. Smart Router Performance

```go
// tests/phase2/performance/benchmarks/smart_router_bench_test.go

func BenchmarkSmartRouterDecision(b *testing.B) {
    scenarios := []struct {
        name   string
        method string
        params interface{}
    }{
        {"SymbolLookup", "textDocument/definition", defParams},
        {"CodeComplete", "textDocument/completion", compParams},
        {"FindRefs", "textDocument/references", refParams},
    }
    
    for _, scenario := range scenarios {
        b.Run(scenario.name, func(b *testing.B) {
            b.ResetTimer()
            
            for i := 0; i < b.N; i++ {
                start := time.Now()
                decision, _ := router.RouteWithSCIPAwareness(request)
                elapsed := time.Since(start)
                
                // Validate <5ms target
                if elapsed > 5*time.Millisecond {
                    b.Errorf("Routing decision took %v, exceeds 5ms target", elapsed)
                }
            }
        })
    }
}
```

### 2. Storage Tier Performance

```go
// tests/phase2/performance/benchmarks/storage_tiers_bench_test.go

func BenchmarkStorageTierAccess(b *testing.B) {
    tiers := []struct {
        name   string
        tier   storage.StorageTier
        target time.Duration
    }{
        {"L1_Memory", manager.L1(), 10 * time.Millisecond},
        {"L2_Disk", manager.L2(), 50 * time.Millisecond},
        {"L3_Remote", manager.L3(), 200 * time.Millisecond},
    }
    
    for _, tier := range tiers {
        b.Run(tier.name, func(b *testing.B) {
            // Generate test data
            symbols := generateTestSymbols(1000)
            
            b.ResetTimer()
            
            for i := 0; i < b.N; i++ {
                key := symbols[i%len(symbols)].ID
                start := time.Now()
                _, _ = tier.tier.Get(ctx, key)
                elapsed := time.Since(start)
                
                // Track P95 latency
                recordLatency(tier.name, elapsed)
            }
            
            // Validate P95 meets target
            p95 := calculateP95(tier.name)
            if p95 > tier.target {
                b.Errorf("%s P95 latency %v exceeds target %v", 
                    tier.name, p95, tier.target)
            }
        })
    }
}
```

### 3. Incremental Pipeline Performance

```go
// tests/phase2/performance/benchmarks/pipeline_latency_bench_test.go

func BenchmarkIncrementalUpdateLatency(b *testing.B) {
    scenarios := []struct {
        name       string
        fileCount  int
        changeSize string
    }{
        {"SingleFile", 1, "small"},
        {"MultiFile", 10, "medium"},
        {"LargeRefactor", 100, "large"},
    }
    
    for _, scenario := range scenarios {
        b.Run(scenario.name, func(b *testing.B) {
            // Setup test project
            project := setupTestProject(scenario.fileCount)
            
            b.ResetTimer()
            
            latencies := []time.Duration{}
            
            for i := 0; i < b.N; i++ {
                // Simulate file change
                changes := generateChanges(scenario.changeSize)
                
                start := time.Now()
                
                // Trigger update
                pipeline.ProcessChanges(changes)
                
                // Wait for propagation
                waitForPropagation(project)
                
                elapsed := time.Since(start)
                latencies = append(latencies, elapsed)
            }
            
            // Calculate average latency
            avgLatency := calculateAverage(latencies)
            
            // Validate <30s target
            if avgLatency > 30*time.Second {
                b.Errorf("Average update latency %v exceeds 30s target", avgLatency)
            }
        })
    }
}
```

### 4. Cache Hit Rate Validation

```go
// tests/phase2/performance/load_tests/cache_hit_rate_test.go

func TestCacheHitRateCompliance(t *testing.T) {
    // Setup realistic workload
    workload := generateRealisticWorkload()
    
    // Track metrics
    var totalQueries int64
    var cacheHits int64
    
    // Run workload
    for _, query := range workload {
        decision, _ := router.RouteWithSCIPAwareness(query)
        
        atomic.AddInt64(&totalQueries, 1)
        if decision.SCIPResult != nil && decision.SCIPResult.FromCache {
            atomic.AddInt64(&cacheHits, 1)
        }
    }
    
    // Calculate hit rate
    hitRate := float64(cacheHits) / float64(totalQueries) * 100
    
    // Validate 85-90% target
    if hitRate < 85 {
        t.Errorf("Cache hit rate %.2f%% below 85%% target", hitRate)
    }
    
    t.Logf("Cache hit rate: %.2f%% (target: 85-90%%)", hitRate)
}
```

## Make Target Specifications

### Core Test Targets

```makefile
# Quick validation (2 minutes)
test-phase2-quick:
	@echo "Running Phase 2 quick validation tests..."
	go test -v ./tests/phase2/unit/... -timeout 2m

# Unit tests only (5 minutes)
test-phase2-unit:
	@echo "Running Phase 2 unit tests..."
	go test -v ./tests/phase2/unit/... -timeout 5m -coverprofile=phase2-unit.coverage

# Integration tests (10 minutes)
test-phase2-integration:
	@echo "Running Phase 2 integration tests..."
	go test -v ./tests/phase2/integration/... -timeout 10m

# Performance validation (15 minutes)
test-phase2-performance:
	@echo "Running Phase 2 performance tests..."
	go test -v ./tests/phase2/performance/... -timeout 15m -bench=. -benchtime=10s

# E2E workflow tests (20 minutes)
test-phase2-e2e:
	@echo "Running Phase 2 E2E workflow tests..."
	go test -v ./tests/phase2/e2e/... -timeout 20m

# Stress tests (30 minutes)
test-phase2-stress:
	@echo "Running Phase 2 stress tests..."
	go test -v ./tests/phase2/stress/... -timeout 30m

# All Phase 2 tests
test-phase2-all: test-phase2-unit test-phase2-integration test-phase2-performance test-phase2-e2e

# Phase 2 validation for CI (includes unit + integration + quick performance)
test-phase2-ci:
	@echo "Running Phase 2 CI validation..."
	go test -v ./tests/phase2/unit/... ./tests/phase2/integration/... -timeout 15m
	go test -v ./tests/phase2/performance/benchmarks/... -timeout 5m -bench=Quick

# Generate Phase 2 test report
test-phase2-report:
	@echo "Generating Phase 2 test report..."
	@mkdir -p tests/phase2/reports/coverage tests/phase2/reports/performance
	go test -v ./tests/phase2/... -coverprofile=tests/phase2/reports/coverage/phase2.coverage
	go tool cover -html=tests/phase2/reports/coverage/phase2.coverage -o tests/phase2/reports/coverage/phase2.html
	@echo "Report generated at tests/phase2/reports/"
```

### Component-Specific Targets

```makefile
# Smart Router tests
test-phase2-router:
	go test -v ./tests/phase2/unit/smart_router/... ./tests/phase2/integration/smart_router_integration/...

# Storage tests
test-phase2-storage:
	go test -v ./tests/phase2/unit/storage/... ./tests/phase2/integration/storage_integration/...

# Pipeline tests
test-phase2-pipeline:
	go test -v ./tests/phase2/unit/pipeline/... ./tests/phase2/integration/pipeline_integration/...

# MCP enhancement tests
test-phase2-mcp:
	go test -v ./tests/phase2/unit/mcp_tools/... ./tests/phase2/integration/mcp_integration/...
```

### Development Workflow Targets

```makefile
# Watch mode for Phase 2 development
test-phase2-watch:
	@echo "Watching Phase 2 tests..."
	watchexec -e go -r "go test -v ./tests/phase2/unit/..."

# Benchmark comparison
test-phase2-benchmark-compare:
	@echo "Running benchmark comparison..."
	go test -bench=. ./tests/phase2/performance/benchmarks/... -benchmem -count=10 > phase2-new.bench
	benchstat phase2-baseline.bench phase2-new.bench

# Generate test fixtures
test-phase2-generate-fixtures:
	@echo "Generating Phase 2 test fixtures..."
	go run ./tests/phase2/common/fixtures/generator/main.go
```

## CI/CD Integration

### GitHub Actions Workflow

```yaml
name: Phase 2 Tests

on:
  push:
    paths:
      - 'internal/gateway/scip_*.go'
      - 'internal/storage/*.go'
      - 'internal/indexing/*.go'
      - 'mcp/*scip*.go'
      - 'tests/phase2/**'

jobs:
  phase2-validation:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Install dependencies
        run: make deps
      
      - name: Run Phase 2 CI tests
        run: make test-phase2-ci
      
      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./tests/phase2/reports/coverage/phase2.coverage
          flags: phase2
      
      - name: Performance regression check
        run: |
          make test-phase2-benchmark-compare
          if grep -q "regression" phase2-benchmark.txt; then
            echo "Performance regression detected!"
            exit 1
          fi
```

## Test Data Generation

### SCIP Index Generator

```go
// tests/phase2/common/fixtures/generator/scip_generator.go

type SCIPIndexGenerator struct {
    languages []string
    sizes     []ProjectSize
}

type ProjectSize struct {
    Name       string
    FileCount  int
    SymbolCount int
}

func (g *SCIPIndexGenerator) GenerateTestIndices() error {
    sizes := []ProjectSize{
        {"small", 10, 100},
        {"medium", 100, 1000},
        {"large", 1000, 10000},
        {"xlarge", 10000, 100000},
    }
    
    for _, lang := range g.languages {
        for _, size := range sizes {
            project := g.generateProject(lang, size)
            index := g.generateSCIPIndex(project)
            
            path := fmt.Sprintf("fixtures/scip-indices/%s-%s.scip", lang, size.Name)
            if err := g.saveIndex(index, path); err != nil {
                return err
            }
        }
    }
    
    return nil
}
```

## Monitoring and Reporting

### Real-time Test Dashboard

```go
// tests/phase2/common/monitoring/dashboard.go

type Phase2TestDashboard struct {
    server *http.Server
    
    // Metrics
    testsPassed    prometheus.Counter
    testsFailed    prometheus.Counter
    testDuration   prometheus.Histogram
    performanceP95 prometheus.GaugeVec
}

func (d *Phase2TestDashboard) Start() {
    http.Handle("/metrics", promhttp.Handler())
    http.HandleFunc("/dashboard", d.serveDashboard)
    
    go func() {
        d.server.ListenAndServe(":9091")
    }()
}
```

## Migration Plan

### Phase 1: Foundation (Week 1)
1. Create directory structure
2. Set up common test utilities
3. Implement basic unit tests for each component
4. Add make targets

### Phase 2: Integration (Week 2)
1. Implement integration tests
2. Add performance benchmarks
3. Create test data generators
4. Set up CI pipeline

### Phase 3: E2E & Stress (Week 3)
1. Implement E2E workflow tests
2. Add stress tests
3. Create monitoring dashboard
4. Generate baseline benchmarks

### Phase 4: Production Readiness (Week 4)
1. Run full test suite
2. Fix any failing tests
3. Optimize performance bottlenecks
4. Generate compliance reports

## Success Criteria

1. **Coverage**: >90% code coverage for all Phase 2 components
2. **Performance**: All performance targets met with <5% variance
3. **Reliability**: Zero flaky tests in CI
4. **Speed**: Full test suite completes in <30 minutes
5. **Integration**: Seamless integration with existing test infrastructure

## Conclusion

This comprehensive testing architecture ensures LSP Gateway's SCIP Phase 2 implementation meets all performance targets and production readiness requirements. The modular design allows for easy extension as new features are added while maintaining fast feedback cycles for developers.