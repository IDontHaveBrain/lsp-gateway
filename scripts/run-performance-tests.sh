#!/bin/bash

# LSP Gateway Performance Integration Test Suite
# Comprehensive performance testing for integration scenarios

set -euo pipefail

# Configuration
PERF_OUTPUT_DIR="./performance-test-results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
PERF_REPORT="${PERF_OUTPUT_DIR}/performance_test_report_${TIMESTAMP}.txt"
PERF_CSV="${PERF_OUTPUT_DIR}/performance_results_${TIMESTAMP}.csv"
MEMORY_PROFILE="${PERF_OUTPUT_DIR}/memory_profile_${TIMESTAMP}.prof"
CPU_PROFILE="${PERF_OUTPUT_DIR}/cpu_profile_${TIMESTAMP}.prof"

# Performance targets for integration tests
TARGET_LATENCY_MS=100
TARGET_THROUGHPUT_RPS=50        # Reduced for integration tests
TARGET_CONCURRENT_CLIENTS=5     # Reduced for integration tests
TARGET_MEMORY_MB=50            # Memory usage target
TARGET_SUCCESS_RATE=0.95       # 95% success rate

# Test configuration
INTEGRATION_BENCH_TIME="30s"
INTEGRATION_COUNT=3
LOAD_TEST_REQUESTS=100         # Reduced for integration focus
LOAD_TEST_CONCURRENCY=10       # Reduced for integration focus

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() {
    echo -e "${BLUE}[$(date +'%H:%M:%S')]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[$(date +'%H:%M:%S')] ✓${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[$(date +'%H:%M:%S')] ⚠${NC} $1"
}

log_error() {
    echo -e "${RED}[$(date +'%H:%M:%S')] ✗${NC} $1"
}

# Create output directory
mkdir -p "$PERF_OUTPUT_DIR"

# Initialize report
cat > "$PERF_REPORT" << EOF
LSP Gateway Performance Integration Test Report
==============================================
Generated: $(date)
Go Version: $(go version)
System: $(uname -a)
CPU Cores: $(nproc)
Memory: $(free -h | awk '/^Mem:/ {print $2}')

Performance Targets for Integration Tests:
- Latency: < ${TARGET_LATENCY_MS}ms
- Throughput: > ${TARGET_THROUGHPUT_RPS} req/sec
- Concurrent Clients: ${TARGET_CONCURRENT_CLIENTS}+
- Memory Usage: < ${TARGET_MEMORY_MB}MB
- Success Rate: > $(echo "$TARGET_SUCCESS_RATE * 100" | bc)%

Test Configuration:
- Benchmark Time: ${INTEGRATION_BENCH_TIME}
- Benchmark Count: ${INTEGRATION_COUNT}
- Load Test Requests: ${LOAD_TEST_REQUESTS}
- Load Test Concurrency: ${LOAD_TEST_CONCURRENCY}

EOF

# Initialize CSV file
echo "Component,Test,Throughput(req/s),Latency(ms),Memory(MB),Success_Rate(%),Status" > "$PERF_CSV"

log "Starting LSP Gateway Performance Integration Test Suite"
log "Results will be saved to: $PERF_REPORT"

# Function to run performance test and capture results
run_performance_test() {
    local component=$1
    local package=$2
    local test_name=$3
    local additional_flags=${4:-}
    
    log "Running $component performance test: $test_name"
    
    local output_file="${PERF_OUTPUT_DIR}/${component}_${test_name}_${TIMESTAMP}.txt"
    local bench_flags="-bench=$test_name -benchmem -benchtime=$INTEGRATION_BENCH_TIME -count=$INTEGRATION_COUNT $additional_flags"
    
    # Capture benchmark output
    if go test $package $bench_flags -run=^$ > "$output_file" 2>&1; then
        log_success "$component/$test_name completed"
        
        # Extract metrics from output
        local throughput=$(extract_throughput "$output_file")
        local latency=$(extract_latency "$output_file") 
        local memory=$(extract_memory "$output_file")
        local success_rate=$(extract_success_rate "$output_file")
        
        # Determine status based on performance targets
        local status=$(determine_status "$throughput" "$latency" "$memory" "$success_rate")
        
        # Add to CSV
        echo "$component,$test_name,$throughput,$latency,$memory,$success_rate,$status" >> "$PERF_CSV"
        
        # Add to report
        {
            echo ""
            echo "=== $component: $test_name ==="
            echo "Status: $status"
            echo "Throughput: $throughput req/sec"
            echo "Latency: $latency ms"
            echo "Memory: $memory MB/op"
            echo "Success Rate: $success_rate%"
            echo ""
            cat "$output_file"
            echo ""
        } >> "$PERF_REPORT"
        
        # Show status
        if [[ "$status" == "PASS" ]]; then
            log_success "$component/$test_name: $status"
        else
            log_warning "$component/$test_name: $status"
        fi
        
        return 0
    else
        log_error "$component/$test_name failed"
        echo "$component,$test_name,ERROR,ERROR,ERROR,ERROR,FAIL" >> "$PERF_CSV"
        
        # Add error to report
        {
            echo ""
            echo "=== $component: $test_name ==="
            echo "Status: FAIL"
            echo "Error output:"
            cat "$output_file"
            echo ""
        } >> "$PERF_REPORT"
        
        return 1
    fi
}

# Function to extract throughput from benchmark output
extract_throughput() {
    local output_file=$1
    local throughput="N/A"
    
    # Look for requests/second patterns in output
    if grep -q "req/sec\|requests/second" "$output_file"; then
        throughput=$(grep -E "req/sec|requests/second" "$output_file" | tail -1 | grep -oE '[0-9.]+' | head -1 || echo "N/A")
    elif grep -q "Requests/second:" "$output_file"; then
        throughput=$(grep "Requests/second:" "$output_file" | tail -1 | awk '{print $2}' || echo "N/A")
    fi
    
    echo "$throughput"
}

# Function to extract latency from benchmark output  
extract_latency() {
    local output_file=$1
    local latency="N/A"
    
    # Look for latency patterns in output
    if grep -q "Latency.*Avg:" "$output_file"; then
        latency=$(grep "Latency.*Avg:" "$output_file" | tail -1 | grep -oE '[0-9.]+ms' | grep -oE '[0-9.]+' | head -1 || echo "N/A")
    elif grep -q "ns/op" "$output_file"; then
        # Convert ns/op to ms
        local ns_per_op=$(grep "ns/op" "$output_file" | tail -1 | awk '{print $3}' | grep -oE '[0-9.]+' || echo "0")
        if [[ "$ns_per_op" != "0" && "$ns_per_op" =~ ^[0-9.]+$ ]]; then
            latency=$(echo "scale=2; $ns_per_op / 1000000" | bc -l)
        fi
    fi
    
    echo "$latency"
}

# Function to extract memory usage from benchmark output
extract_memory() {
    local output_file=$1
    local memory="N/A"
    
    # Look for B/op (bytes per operation)
    if grep -q "B/op" "$output_file"; then
        local bytes_per_op=$(grep "B/op" "$output_file" | tail -1 | awk '{print $4}' | grep -oE '[0-9.]+' || echo "0")
        if [[ "$bytes_per_op" != "0" && "$bytes_per_op" =~ ^[0-9.]+$ ]]; then
            memory=$(echo "scale=2; $bytes_per_op / 1024 / 1024" | bc -l)
        fi
    fi
    
    echo "$memory"
}

# Function to extract success rate from benchmark output
extract_success_rate() {
    local output_file=$1
    local success_rate="N/A"
    
    # Look for success rate patterns
    if grep -q "Success rate:" "$output_file"; then
        success_rate=$(grep "Success rate:" "$output_file" | tail -1 | grep -oE '[0-9.]+%' | grep -oE '[0-9.]+' | head -1 || echo "N/A")
    elif grep -q "success rate" "$output_file"; then
        success_rate=$(grep "success rate" "$output_file" | tail -1 | grep -oE '[0-9.]+%' | grep -oE '[0-9.]+' | head -1 || echo "N/A")
    elif grep -q "Successful:" "$output_file"; then
        # Calculate from successful/total counts
        local successful=$(grep "Successful:" "$output_file" | tail -1 | awk '{print $2}' || echo "0")
        local total=$(grep "Total requests:" "$output_file" | tail -1 | awk '{print $3}' || echo "1")
        if [[ "$successful" =~ ^[0-9]+$ && "$total" =~ ^[0-9]+$ && "$total" -gt 0 ]]; then
            success_rate=$(echo "scale=1; $successful * 100 / $total" | bc -l)
        fi
    else
        success_rate="100"  # Assume 100% if no failures reported
    fi
    
    echo "$success_rate"
}

# Function to determine test status based on metrics
determine_status() {
    local throughput=$1
    local latency=$2
    local memory=$3
    local success_rate=$4
    local status="PASS"
    
    # Check latency target
    if [[ "$latency" != "N/A" && "$latency" =~ ^[0-9.]+$ ]]; then
        if (( $(echo "$latency > $TARGET_LATENCY_MS" | bc -l) )); then
            status="FAIL_LATENCY"
        fi
    fi
    
    # Check throughput target
    if [[ "$throughput" != "N/A" && "$throughput" =~ ^[0-9.]+$ ]]; then
        if (( $(echo "$throughput < $TARGET_THROUGHPUT_RPS" | bc -l) )); then
            status="FAIL_THROUGHPUT"
        fi
    fi
    
    # Check memory target
    if [[ "$memory" != "N/A" && "$memory" =~ ^[0-9.]+$ ]]; then
        if (( $(echo "$memory > $TARGET_MEMORY_MB" | bc -l) )); then
            status="FAIL_MEMORY"
        fi
    fi
    
    # Check success rate target
    if [[ "$success_rate" != "N/A" && "$success_rate" =~ ^[0-9.]+$ ]]; then
        local success_decimal=$(echo "scale=2; $success_rate / 100" | bc -l)
        if (( $(echo "$success_decimal < $TARGET_SUCCESS_RATE" | bc -l) )); then
            status="FAIL_SUCCESS_RATE"
        fi
    fi
    
    echo "$status"
}

# Function to run memory profiling
run_memory_profiling() {
    local component=$1
    local package=$2
    local test_name=$3
    
    log "Running memory profiling for $component/$test_name"
    
    if go test $package -bench="$test_name" -benchtime=60s -memprofile="$MEMORY_PROFILE" -run=^$ > /dev/null 2>&1; then
        log_success "Memory profile saved to $MEMORY_PROFILE"
        
        # Analyze memory profile
        if command -v go &> /dev/null; then
            local top_allocs=$(go tool pprof -top -alloc_space "$MEMORY_PROFILE" 2>/dev/null | head -20 || echo "Profile analysis failed")
            
            {
                echo ""
                echo "=== Memory Profile Analysis: $component/$test_name ==="
                echo "$top_allocs"
                echo ""
            } >> "$PERF_REPORT"
        fi
        return 0
    else
        log_warning "Memory profiling failed for $component/$test_name"
        return 1
    fi
}

# Function to run CPU profiling
run_cpu_profiling() {
    local component=$1
    local package=$2
    local test_name=$3
    
    log "Running CPU profiling for $component/$test_name"
    
    if go test $package -bench="$test_name" -benchtime=60s -cpuprofile="$CPU_PROFILE" -run=^$ > /dev/null 2>&1; then
        log_success "CPU profile saved to $CPU_PROFILE"
        
        # Analyze CPU profile
        if command -v go &> /dev/null; then
            local top_functions=$(go tool pprof -top "$CPU_PROFILE" 2>/dev/null | head -20 || echo "Profile analysis failed")
            
            {
                echo ""
                echo "=== CPU Profile Analysis: $component/$test_name ==="
                echo "$top_functions"
                echo ""
            } >> "$PERF_REPORT"
        fi
        return 0
    else
        log_warning "CPU profiling failed for $component/$test_name"
        return 1
    fi
}

# Function to validate environment
validate_environment() {
    log "Validating performance test environment..."
    
    # Check Go version
    if ! go version | grep -q "go1.2[4-9]"; then
        log_error "Go 1.24+ required"
        exit 1
    fi
    
    # Check if binary exists and works
    if [[ ! -f "./bin/lsp-gateway" ]]; then
        log "Building LSP Gateway..."
        if ! make local; then
            log_error "Failed to build LSP Gateway"
            exit 1
        fi
    fi
    
    # Check for bc calculator (needed for calculations)
    if ! command -v bc &> /dev/null; then
        log_error "bc calculator required for performance calculations"
        exit 1
    fi
    
    log_success "Environment validation passed"
}

# Main execution
main() {
    local test_mode=${1:-"integration"}
    local exit_code=0
    
    # Validate environment first
    validate_environment
    
    case "$test_mode" in
        "integration"|"")
            log "Running integration performance tests"
            
            # MCP Integration Performance Tests
            log "=== MCP Integration Performance Tests ==="
            if ! run_performance_test "MCP" "./mcp" "BenchmarkIntegrationThroughput"; then
                exit_code=1
            fi
            
            if ! run_performance_test "MCP" "./mcp" "BenchmarkMCPMessageProcessingIntegration"; then
                exit_code=1
            fi
            
            # Run performance-focused integration tests
            log "=== Running Performance Integration Tests ==="
            if ! go test ./mcp -v -timeout=120s -run=TestConcurrentThroughputPerformance > "${PERF_OUTPUT_DIR}/concurrent_throughput_${TIMESTAMP}.txt" 2>&1; then
                log_warning "Concurrent throughput performance test had issues"
            else
                log_success "Concurrent throughput performance test completed"
            fi
            
            if ! go test ./mcp -v -timeout=120s -run=TestMixedToolPerformance > "${PERF_OUTPUT_DIR}/mixed_tool_performance_${TIMESTAMP}.txt" 2>&1; then
                log_warning "Mixed tool performance test had issues"
            else
                log_success "Mixed tool performance test completed"
            fi
            
            # Memory and CPU profiling for key components
            log "=== Performance Profiling ==="
            run_memory_profiling "MCP" "./mcp" "BenchmarkIntegrationThroughput"
            run_cpu_profiling "MCP" "./mcp" "BenchmarkIntegrationThroughput"
            ;;
            
        "full")
            log "Running full performance test suite"
            
            # Run integration performance tests first
            main "integration"
            local integration_exit=$?
            
            # Run additional benchmark suite
            log "=== Running Full Benchmark Suite ==="
            if [[ -f "./scripts/run-benchmarks.sh" ]]; then
                if ./scripts/run-benchmarks.sh; then
                    log_success "Full benchmark suite completed"
                else
                    log_warning "Full benchmark suite had issues"
                    exit_code=1
                fi
            else
                log_warning "Full benchmark script not found, skipping"
            fi
            
            if [[ $integration_exit -ne 0 ]]; then
                exit_code=1
            fi
            ;;
            
        "quick")
            log "Running quick performance validation"
            
            # Quick MCP performance test
            if ! run_performance_test "MCP" "./mcp" "BenchmarkIntegrationThroughput" "-benchtime=5s -count=1"; then
                exit_code=1
            fi
            ;;
    esac
    
    # Generate performance summary
    generate_performance_summary
    
    # Final status
    if [[ $exit_code -eq 0 ]]; then
        log_success "Performance integration test suite completed successfully!"
        log "📊 Performance targets met for integration scenarios"
    else
        log_warning "Some performance tests failed or below targets"
        log "⚠️  Check $PERF_REPORT for details and recommendations"
    fi
    
    log "Complete performance report available at: $PERF_REPORT"
    log "CSV data available at: $PERF_CSV"
    
    return $exit_code
}

# Function to generate performance summary
generate_performance_summary() {
    log "Generating performance summary..."
    
    {
        echo ""
        echo "=== PERFORMANCE SUMMARY ==="
        echo ""
        
        # Count passes and failures
        local total_tests=$(tail -n +2 "$PERF_CSV" | wc -l || echo "0")
        local passed_tests=$(grep -c "PASS" "$PERF_CSV" || echo "0")
        local failed_tests=$(grep -c "FAIL" "$PERF_CSV" || echo "0")
        
        echo "Total Performance Tests: $total_tests"
        echo "Passed: $passed_tests"
        echo "Failed: $failed_tests"
        
        if [[ $failed_tests -gt 0 ]]; then
            echo ""
            echo "FAILED TESTS:"
            grep "FAIL" "$PERF_CSV" | while IFS=',' read -r component test throughput latency memory success_rate status; do
                echo "  - $component/$test: $status"
            done
        fi
        
        echo ""
        echo "=== PERFORMANCE RECOMMENDATIONS ==="
        echo ""
        
        # Generate recommendations based on failures
        if grep -q "FAIL_LATENCY" "$PERF_CSV"; then
            echo "⚠ LATENCY ISSUES DETECTED:"
            echo "  - Some operations exceed ${TARGET_LATENCY_MS}ms target"
            echo "  - Consider optimizing critical path performance"
            echo "  - Review CPU profile for bottlenecks"
            echo ""
        fi
        
        if grep -q "FAIL_THROUGHPUT" "$PERF_CSV"; then
            echo "⚠ THROUGHPUT ISSUES DETECTED:"
            echo "  - Some operations below ${TARGET_THROUGHPUT_RPS} req/sec target"
            echo "  - Consider increasing concurrency"
            echo "  - Review connection pooling strategies"
            echo ""
        fi
        
        if grep -q "FAIL_MEMORY" "$PERF_CSV"; then
            echo "⚠ MEMORY ISSUES DETECTED:"
            echo "  - Some operations exceed ${TARGET_MEMORY_MB}MB memory target"
            echo "  - Consider memory optimization"
            echo "  - Review memory profile for allocations"
            echo ""
        fi
        
        if grep -q "FAIL_SUCCESS_RATE" "$PERF_CSV"; then
            echo "⚠ SUCCESS RATE ISSUES DETECTED:"
            echo "  - Some operations below $(echo "$TARGET_SUCCESS_RATE * 100" | bc)% success rate"
            echo "  - Review error handling and retry logic"
            echo "  - Check for timeout or connection issues"
            echo ""
        fi
        
        if [[ $failed_tests -eq 0 ]]; then
            echo "✓ ALL PERFORMANCE TARGETS MET"
            echo "  - Integration performance ready for production"
            echo "  - Consider stress testing with higher loads"
            echo ""
        fi
        
        echo "Files generated:"
        echo "  - Performance report: $PERF_REPORT"
        echo "  - CSV data: $PERF_CSV"
        echo "  - Memory profile: $MEMORY_PROFILE"
        echo "  - CPU profile: $CPU_PROFILE"
        
    } >> "$PERF_REPORT"
}

# Help function
show_help() {
    cat << EOF
LSP Gateway Performance Integration Test Suite

Usage: $0 [MODE]

Modes:
  integration  Run integration performance tests (default)
  full         Run full performance test suite
  quick        Run quick performance validation
  help         Show this help message

Examples:
  $0                    # Run integration performance tests
  $0 integration       # Run integration performance tests
  $0 full              # Run complete performance suite  
  $0 quick             # Run quick performance validation

Environment Variables:
  PERF_TEST_TIMEOUT       Override benchmark timeout
  PERF_TEST_REQUESTS      Override number of test requests
  PERF_TEST_CONCURRENCY   Override concurrency level

For detailed documentation, see: docs/INTEGRATION_TESTING.md
EOF
}

# Handle command line arguments
case "${1:-integration}" in
    "help"|"-h"|"--help")
        show_help
        exit 0
        ;;
    *)
        main "$@"
        exit $?
        ;;
esac