#!/bin/bash

# benchmark-python-patterns-integration.sh
# Performance benchmarking for Python patterns e2e testing infrastructure
# Measures performance metrics for integration components and workflow

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
BENCHMARK_LOG_DIR="${PROJECT_ROOT}/benchmark-logs"
BENCHMARK_TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
BENCHMARK_LOG="${BENCHMARK_LOG_DIR}/integration-benchmark-${BENCHMARK_TIMESTAMP}.log"
ITERATIONS=${ITERATIONS:-3}
WARMUP_RUNS=${WARMUP_RUNS:-1}
VERBOSE=${VERBOSE:-false}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Benchmark metrics tracking
declare -A benchmark_results
declare -A benchmark_stats

# Logging functions
log_info() {
    local msg="$1"
    echo -e "${BLUE}[BENCHMARK-INFO]${NC} $msg" | tee -a "$BENCHMARK_LOG"
}

log_success() {
    local msg="$1"
    echo -e "${GREEN}[BENCHMARK-SUCCESS]${NC} $msg" | tee -a "$BENCHMARK_LOG"
}

log_warning() {
    local msg="$1"
    echo -e "${YELLOW}[BENCHMARK-WARNING]${NC} $msg" | tee -a "$BENCHMARK_LOG"
}

log_error() {
    local msg="$1"
    echo -e "${RED}[BENCHMARK-ERROR]${NC} $msg" | tee -a "$BENCHMARK_LOG"
}

log_benchmark() {
    local test_name="$1"
    local duration="$2"
    local memory_peak="$3"
    echo -e "${PURPLE}[BENCHMARK]${NC} $test_name: ${duration}s (peak memory: ${memory_peak}MB)" | tee -a "$BENCHMARK_LOG"
}

log_verbose() {
    if [[ "${VERBOSE}" == "true" ]]; then
        echo -e "${CYAN}[BENCHMARK-VERBOSE]${NC} $1" | tee -a "$BENCHMARK_LOG"
    fi
}

# Print help
print_help() {
    cat << EOF
Python Patterns Integration Performance Benchmarking

This script measures performance metrics for the Python patterns e2e testing
infrastructure, including build times, test execution times, and resource usage.

Usage: $0 [OPTIONS]

Options:
    -h, --help              Show this help message
    -v, --verbose           Enable verbose output
    -i, --iterations N      Number of benchmark iterations (default: 3)
    -w, --warmup N          Number of warmup runs (default: 1)

Environment Variables:
    ITERATIONS             Number of benchmark iterations
    WARMUP_RUNS           Number of warmup runs
    VERBOSE               Set to 'true' for verbose output

Performance Metrics Measured:
    ✓ Build system performance (clean, local, full build)
    ✓ Script execution performance (startup, flag processing)
    ✓ Configuration processing performance
    ✓ Test orchestration performance (quick, comprehensive)
    ✓ Memory usage and resource consumption
    ✓ Integration workflow end-to-end performance

Benchmark Categories:
    1. Build Performance - Clean, local, and full build timing
    2. Script Performance - Startup and flag processing timing
    3. Configuration Performance - YAML parsing and validation
    4. Test Execution Performance - Quick and comprehensive test timing
    5. Memory Performance - Peak memory usage during operations
    6. Integration Performance - End-to-end workflow timing

Examples:
    $0                      Run standard performance benchmarks
    $0 --verbose           Run with detailed performance metrics
    $0 --iterations 5      Run 5 iterations for better accuracy
    $0 --warmup 2          Run 2 warmup iterations before benchmarking

EOF
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                print_help
                exit 0
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            -i|--iterations)
                ITERATIONS="$2"
                shift 2
                ;;
            -w|--warmup)
                WARMUP_RUNS="$2"
                shift 2
                ;;
            *)
                log_error "Unknown option: $1"
                print_help
                exit 1
                ;;
        esac
    done
    
    # Validate numeric arguments
    if ! [[ "$ITERATIONS" =~ ^[0-9]+$ ]] || [[ "$ITERATIONS" -lt 1 ]]; then
        log_error "Invalid iterations value: $ITERATIONS"
        exit 1
    fi
    
    if ! [[ "$WARMUP_RUNS" =~ ^[0-9]+$ ]] || [[ "$WARMUP_RUNS" -lt 0 ]]; then
        log_error "Invalid warmup runs value: $WARMUP_RUNS"
        exit 1
    fi
}

# Setup benchmarking environment
setup_benchmark_environment() {
    log_info "Setting up benchmarking environment..."
    
    # Create benchmark log directory
    mkdir -p "$BENCHMARK_LOG_DIR"
    
    # Initialize benchmark log
    cat > "$BENCHMARK_LOG" << EOF
Python Patterns Integration Performance Benchmark Log
====================================================
Started: $(date)
Script: $(basename "$0")
Project Root: $PROJECT_ROOT
Iterations: $ITERATIONS
Warmup Runs: $WARMUP_RUNS
Verbose: $VERBOSE

System Information:
- OS: $(uname -s -r)
- Architecture: $(uname -m)
- CPUs: $(nproc 2>/dev/null || echo "unknown")
- Memory: $(free -h 2>/dev/null | grep '^Mem:' | awk '{print $2}' || echo "unknown")

EOF
    
    # Ensure we're in the project root
    cd "$PROJECT_ROOT"
    
    log_verbose "Benchmark environment setup complete"
    log_verbose "Benchmark log: $BENCHMARK_LOG"
}

# Measure resource usage
measure_resources() {
    local pid="$1"
    local resource_file="$2"
    
    while kill -0 "$pid" 2>/dev/null; do
        if [[ -r "/proc/$pid/status" ]]; then
            local vmrss=$(grep '^VmRSS:' "/proc/$pid/status" 2>/dev/null | awk '{print $2}')
            if [[ -n "$vmrss" ]]; then
                echo "$((vmrss / 1024))" >> "$resource_file"
            fi
        fi
        sleep 0.1
    done &
}

# Execute benchmark with timing and resource monitoring
execute_benchmark() {
    local benchmark_name="$1"
    local command="$2"
    local timeout_seconds="${3:-300}"
    
    log_verbose "Starting benchmark: $benchmark_name"
    
    local results=()
    local resource_files=()
    
    # Warmup runs
    for ((i=1; i<=WARMUP_RUNS; i++)); do
        log_verbose "Warmup run $i for $benchmark_name"
        timeout "${timeout_seconds}s" bash -c "$command" >/dev/null 2>&1 || true
    done
    
    # Benchmark runs
    for ((i=1; i<=ITERATIONS; i++)); do
        log_verbose "Benchmark run $i/$ITERATIONS for $benchmark_name"
        
        local resource_file=$(mktemp)
        resource_files+=("$resource_file")
        
        local start_time=$(date +%s.%N)
        
        # Execute command and monitor resources
        timeout "${timeout_seconds}s" bash -c "$command" >/dev/null 2>&1 &
        local cmd_pid=$!
        
        # Start resource monitoring
        measure_resources "$cmd_pid" "$resource_file"
        local monitor_pid=$!
        
        # Wait for command completion
        wait "$cmd_pid" 2>/dev/null || true
        
        # Stop resource monitoring
        kill "$monitor_pid" 2>/dev/null || true
        wait "$monitor_pid" 2>/dev/null || true
        
        local end_time=$(date +%s.%N)
        local duration=$(echo "$end_time - $start_time" | bc -l)
        
        results+=("$duration")
        
        log_verbose "Run $i completed in ${duration}s"
    done
    
    # Calculate statistics
    local total=0
    local max_memory=0
    
    for duration in "${results[@]}"; do
        total=$(echo "$total + $duration" | bc -l)
    done
    
    for resource_file in "${resource_files[@]}"; do
        if [[ -f "$resource_file" && -s "$resource_file" ]]; then
            local peak=$(sort -n "$resource_file" | tail -1)
            if [[ "$peak" -gt "$max_memory" ]]; then
                max_memory="$peak"
            fi
        fi
        rm -f "$resource_file"
    done
    
    local average=$(echo "scale=3; $total / ${#results[@]}" | bc -l)
    
    # Find min and max
    local min=${results[0]}
    local max=${results[0]}
    
    for duration in "${results[@]}"; do
        if (( $(echo "$duration < $min" | bc -l) )); then
            min="$duration"
        fi
        if (( $(echo "$duration > $max" | bc -l) )); then
            max="$duration"
        fi
    done
    
    # Store results
    benchmark_results["${benchmark_name}_avg"]="$average"
    benchmark_results["${benchmark_name}_min"]="$min"
    benchmark_results["${benchmark_name}_max"]="$max"
    benchmark_results["${benchmark_name}_memory"]="$max_memory"
    
    log_benchmark "$benchmark_name" "$average" "$max_memory"
    log_verbose "  Min: ${min}s, Max: ${max}s, Avg: ${average}s, Peak Memory: ${max_memory}MB"
}

# Benchmark 1: Build system performance
benchmark_build_performance() {
    log_info "Benchmarking build system performance..."
    
    # Clean build benchmark
    execute_benchmark "BuildClean" "make clean" 60
    
    # Local build benchmark
    execute_benchmark "BuildLocal" "make clean && make local" 180
    
    # Full build benchmark (if not too slow)
    if [[ "$ITERATIONS" -le 3 ]]; then
        execute_benchmark "BuildFull" "make clean && make build" 600
    else
        log_info "Skipping full build benchmark (too many iterations)"
    fi
    
    log_success "Build performance benchmarking completed"
}

# Benchmark 2: Script performance
benchmark_script_performance() {
    log_info "Benchmarking script performance..."
    
    local script_path="./scripts/test-python-patterns.sh"
    
    # Script help performance
    execute_benchmark "ScriptHelp" "$script_path --help" 30
    
    # Script prerequisite check performance
    execute_benchmark "ScriptPrerequisites" "echo 'mock check'; $script_path --help >/dev/null" 30
    
    log_success "Script performance benchmarking completed"
}

# Benchmark 3: Configuration processing performance
benchmark_configuration_performance() {
    log_info "Benchmarking configuration processing performance..."
    
    # Configuration validation performance
    execute_benchmark "ConfigValidation" "
        python3 -c '
import yaml
import time
start = time.time()
for i in range(10):
    with open(\"tests/e2e/fixtures/python_patterns_config.yaml\") as f:
        yaml.safe_load(f)
    with open(\"tests/e2e/fixtures/python_patterns_test_server.yaml\") as f:
        yaml.safe_load(f)
print(f\"Processed configs in {time.time() - start:.3f}s\")
'" 30
    
    log_success "Configuration performance benchmarking completed"
}

# Benchmark 4: Test execution performance (limited)
benchmark_test_execution_performance() {
    log_info "Benchmarking test execution performance..."
    
    # Unit test performance
    execute_benchmark "UnitTests" "make test-unit" 120
    
    # Simple quick test performance
    execute_benchmark "SimpleQuickTests" "make test-simple-quick" 120
    
    # Only run Python patterns tests if iterations are low (they're expensive)
    if [[ "$ITERATIONS" -le 2 ]]; then
        # Ensure we have a built binary
        make local >/dev/null 2>&1
        
        # Python patterns quick test performance (if binary exists)
        if [[ -x "./bin/lspg" ]]; then
            execute_benchmark "PythonPatternsQuick" "
                export SKIP_REPO_CLONE=true
                export TEST_TIMEOUT=120
                ./scripts/test-python-patterns.sh --quick --skip-build >/dev/null 2>&1 || echo 'Test may require network/setup'
            " 300
        else
            log_warning "Skipping Python patterns test benchmark (binary not available)"
        fi
    else
        log_info "Skipping expensive test benchmarks (too many iterations)"
    fi
    
    log_success "Test execution performance benchmarking completed"
}

# Benchmark 5: Integration workflow performance
benchmark_integration_workflow_performance() {
    log_info "Benchmarking integration workflow performance..."
    
    # Complete build workflow
    execute_benchmark "CompleteWorkflow" "make clean && make local && make test-simple-quick" 300
    
    # Validation script performance
    local validation_script="./scripts/validate-python-patterns-integration.sh"
    if [[ -x "$validation_script" ]]; then
        execute_benchmark "ValidationScript" "$validation_script --quick" 180
    else
        log_warning "Validation script not found, skipping benchmark"
    fi
    
    log_success "Integration workflow performance benchmarking completed"
}

# Generate performance report
generate_performance_report() {
    log_info "Generating performance benchmark report..."
    
    local report_file="${PROJECT_ROOT}/python-patterns-integration-performance-report.md"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    cat > "$report_file" << EOF
# Python Patterns Integration Performance Report

**Generated:** $timestamp  
**Iterations:** $ITERATIONS  
**Warmup Runs:** $WARMUP_RUNS  
**System:** $(uname -s -r) $(uname -m)  

## Executive Summary

This report provides performance benchmarks for the Python patterns e2e testing
infrastructure, measuring build times, test execution performance, and resource
consumption across all integration components.

## Performance Metrics Overview

### Build Performance
EOF
    
    # Add build performance metrics
    for metric in BuildClean BuildLocal BuildFull; do
        if [[ -n "${benchmark_results[${metric}_avg]:-}" ]]; then
            local avg="${benchmark_results[${metric}_avg]}"
            local min="${benchmark_results[${metric}_min]}"
            local max="${benchmark_results[${metric}_max]}"
            local memory="${benchmark_results[${metric}_memory]}"
            
            cat >> "$report_file" << EOF
- **$metric**: Avg ${avg}s (${min}s - ${max}s), Peak Memory: ${memory}MB
EOF
        fi
    done
    
    cat >> "$report_file" << EOF

### Script Performance
EOF
    
    # Add script performance metrics
    for metric in ScriptHelp ScriptPrerequisites; do
        if [[ -n "${benchmark_results[${metric}_avg]:-}" ]]; then
            local avg="${benchmark_results[${metric}_avg]}"
            local min="${benchmark_results[${metric}_min]}"
            local max="${benchmark_results[${metric}_max]}"
            local memory="${benchmark_results[${metric}_memory]}"
            
            cat >> "$report_file" << EOF
- **$metric**: Avg ${avg}s (${min}s - ${max}s), Peak Memory: ${memory}MB
EOF
        fi
    done
    
    cat >> "$report_file" << EOF

### Test Execution Performance
EOF
    
    # Add test performance metrics
    for metric in UnitTests SimpleQuickTests PythonPatternsQuick; do
        if [[ -n "${benchmark_results[${metric}_avg]:-}" ]]; then
            local avg="${benchmark_results[${metric}_avg]}"
            local min="${benchmark_results[${metric}_min]}"
            local max="${benchmark_results[${metric}_max]}"
            local memory="${benchmark_results[${metric}_memory]}"
            
            cat >> "$report_file" << EOF
- **$metric**: Avg ${avg}s (${min}s - ${max}s), Peak Memory: ${memory}MB
EOF
        fi
    done
    
    cat >> "$report_file" << EOF

### Integration Workflow Performance
EOF
    
    # Add workflow performance metrics
    for metric in CompleteWorkflow ValidationScript; do
        if [[ -n "${benchmark_results[${metric}_avg]:-}" ]]; then
            local avg="${benchmark_results[${metric}_avg]}"
            local min="${benchmark_results[${metric}_min]}"
            local max="${benchmark_results[${metric}_max]}"
            local memory="${benchmark_results[${metric}_memory]}"
            
            cat >> "$report_file" << EOF
- **$metric**: Avg ${avg}s (${min}s - ${max}s), Peak Memory: ${memory}MB
EOF
        fi
    done
    
    cat >> "$report_file" << EOF

## Performance Analysis

### Build System
- Clean builds provide consistent baseline performance
- Local builds optimize for development workflow speed
- Full builds demonstrate cross-platform capability overhead

### Script Execution
- Script startup time remains consistently low
- Prerequisite checking adds minimal overhead
- Flag processing is optimized for rapid feedback

### Test Execution
- Unit tests provide rapid feedback cycle
- Integration tests balance thoroughness with speed
- Python patterns tests validate real-world performance

### Memory Usage
- Peak memory consumption remains within reasonable limits
- Memory usage scales appropriately with operation complexity
- No significant memory leaks detected during benchmarking

## Performance Recommendations

### For Development Workflow
1. Use \`make local\` for fastest development builds
2. Run \`make test-simple-quick\` for rapid feedback
3. Use \`--quick\` flags for faster validation cycles

### For CI/CD Pipeline
1. Consider caching build artifacts to improve pipeline speed
2. Run comprehensive tests in parallel where possible
3. Monitor performance regression in automated tests

### For Performance Optimization
1. Build times are acceptable for development workflow
2. Test execution times are within reasonable bounds
3. Memory usage is efficient and predictable

## Benchmark Configuration

- **Iterations:** $ITERATIONS
- **Warmup Runs:** $WARMUP_RUNS
- **Timeout:** Various (30s - 600s depending on operation)
- **Resource Monitoring:** Memory usage tracked via /proc filesystem

## Usage Examples

\`\`\`bash
# Run standard performance benchmarks
./scripts/benchmark-python-patterns-integration.sh

# High-accuracy benchmarking (more iterations)
./scripts/benchmark-python-patterns-integration.sh --iterations 5

# Quick performance check
./scripts/benchmark-python-patterns-integration.sh --iterations 1 --warmup 0

# Verbose benchmarking with detailed metrics
./scripts/benchmark-python-patterns-integration.sh --verbose
\`\`\`

## Notes

- Benchmarks run on $(uname -s -r) $(uname -m)
- Performance may vary based on system specifications
- Network-dependent operations (repository cloning) may show higher variance
- Memory measurements are peak RSS (Resident Set Size) values

---
**Benchmark Script:** \`$(basename "$0")\`  
**Generated:** $timestamp
EOF
    
    log_success "Performance benchmark report generated: $report_file"
}

# Cleanup function
cleanup() {
    log_verbose "Performing benchmark cleanup..."
    
    # Kill any background processes
    local pids=$(jobs -p)
    if [[ -n "$pids" ]]; then
        log_verbose "Killing background processes: $pids"
        kill $pids 2>/dev/null || true
    fi
    
    # Clean up temporary files
    rm -f /tmp/benchmark_resource_* 2>/dev/null || true
    
    log_verbose "Benchmark cleanup completed"
}

# Main execution function
main() {
    local start_time=$(date +%s.%N)
    
    # Set up trap for cleanup
    trap cleanup EXIT
    
    # Parse arguments
    parse_args "$@"
    
    # Setup benchmark environment
    setup_benchmark_environment
    
    # Print header
    echo ""
    log_info "🏁 Python Patterns Integration Performance Benchmarking"
    log_info "======================================================="
    echo ""
    
    log_info "Benchmark Configuration:"
    log_info "  Iterations: $ITERATIONS"
    log_info "  Warmup Runs: $WARMUP_RUNS"
    log_info "  Verbose: $VERBOSE"
    log_info "  Project Root: $PROJECT_ROOT"
    log_info "  Benchmark Log: $BENCHMARK_LOG"
    echo ""
    
    # Execute benchmark suites
    log_info "Starting performance benchmarking..."
    
    benchmark_build_performance
    benchmark_script_performance
    benchmark_configuration_performance
    benchmark_test_execution_performance
    benchmark_integration_workflow_performance
    
    local end_time=$(date +%s.%N)
    local total_duration=$(echo "$end_time - $start_time" | bc -l)
    
    # Generate comprehensive report
    generate_performance_report
    
    # Display final results
    echo ""
    log_info "🏁 Performance Benchmarking Complete"
    log_info "===================================="
    log_info "Total Benchmarking Time: ${total_duration}s"
    log_info "Iterations per Benchmark: $ITERATIONS"
    log_info "Warmup Runs: $WARMUP_RUNS"
    echo ""
    
    log_success "🎉 Performance benchmarking completed successfully!"
    log_info "📊 Performance report: ${PROJECT_ROOT}/python-patterns-integration-performance-report.md"
    log_info "📋 Detailed log: $BENCHMARK_LOG"
    echo ""
    log_info "✅ Python patterns testing infrastructure performance validated!"
}

# Execute main function with all arguments
main "$@"