#!/bin/bash

# LSP Gateway Performance Benchmark Suite
# Runs comprehensive performance tests and generates reports

set -euo pipefail

# Configuration
BENCHMARK_OUTPUT_DIR="./benchmark-results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
REPORT_FILE="${BENCHMARK_OUTPUT_DIR}/benchmark_report_${TIMESTAMP}.txt"
CSV_FILE="${BENCHMARK_OUTPUT_DIR}/benchmark_results_${TIMESTAMP}.csv"
MEMORY_PROFILE="${BENCHMARK_OUTPUT_DIR}/memory_profile_${TIMESTAMP}.prof"
CPU_PROFILE="${BENCHMARK_OUTPUT_DIR}/cpu_profile_${TIMESTAMP}.prof"

# Performance targets
TARGET_LATENCY_MS=100
TARGET_THROUGHPUT_RPS=100
TARGET_CONCURRENT_CLIENTS=10

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
mkdir -p "$BENCHMARK_OUTPUT_DIR"

# Initialize report
cat > "$REPORT_FILE" << EOF
LSP Gateway Performance Benchmark Report
========================================
Generated: $(date)
Go Version: $(go version)
System: $(uname -a)
CPU Cores: $(nproc)
Memory: $(free -h | awk '/^Mem:/ {print $2}')

Performance Targets:
- Latency: < ${TARGET_LATENCY_MS}ms
- Throughput: > ${TARGET_THROUGHPUT_RPS} req/sec
- Concurrent Clients: ${TARGET_CONCURRENT_CLIENTS}+

EOF

# Initialize CSV file
echo "Component,Test,Throughput(req/s),Latency(ms),Memory(MB),Status" > "$CSV_FILE"

log "Starting LSP Gateway Performance Benchmark Suite"
log "Results will be saved to: $REPORT_FILE"

# Function to run benchmark and capture results
run_benchmark() {
    local component=$1
    local package=$2
    local test_name=$3
    local additional_flags=${4:-}
    
    log "Running $component benchmarks: $test_name"
    
    local output_file="${BENCHMARK_OUTPUT_DIR}/${component}_${test_name}_${TIMESTAMP}.txt"
    local benchmark_flags="-bench=$test_name -benchmem -benchtime=30s -count=3 $additional_flags"
    
    # Capture benchmark output
    if go test $package $benchmark_flags > "$output_file" 2>&1; then
        log_success "$component/$test_name completed"
        
        # Extract metrics from output
        local throughput=$(grep -E "req/sec|requests/second" "$output_file" | tail -1 | grep -oE '[0-9.]+' | head -1 || echo "N/A")
        local latency=$(grep -E "Latency.*Avg:" "$output_file" | grep -oE '[0-9.]+ms' | grep -oE '[0-9.]+' | head -1 || echo "N/A")
        local memory=$(grep -E "B/op" "$output_file" | tail -1 | awk '{print $4}' | grep -oE '[0-9.]+' || echo "N/A")
        
        # Convert memory from bytes to MB if available
        if [[ "$memory" != "N/A" && "$memory" =~ ^[0-9.]+$ ]]; then
            memory=$(echo "scale=2; $memory / 1024 / 1024" | bc -l)
        fi
        
        # Determine status based on performance targets
        local status="PASS"
        if [[ "$latency" != "N/A" && "$latency" =~ ^[0-9.]+$ ]] && (( $(echo "$latency > $TARGET_LATENCY_MS" | bc -l) )); then
            status="FAIL_LATENCY"
        fi
        if [[ "$throughput" != "N/A" && "$throughput" =~ ^[0-9.]+$ ]] && (( $(echo "$throughput < $TARGET_THROUGHPUT_RPS" | bc -l) )); then
            status="FAIL_THROUGHPUT"
        fi
        
        # Add to CSV
        echo "$component,$test_name,$throughput,$latency,$memory,$status" >> "$CSV_FILE"
        
        # Add to report
        {
            echo ""
            echo "=== $component: $test_name ==="
            echo "Status: $status"
            echo "Throughput: $throughput req/sec"
            echo "Latency: $latency ms"
            echo "Memory: $memory MB/op"
            echo ""
            cat "$output_file"
            echo ""
        } >> "$REPORT_FILE"
        
        # Show status
        if [[ "$status" == "PASS" ]]; then
            log_success "$component/$test_name: $status"
        else
            log_warning "$component/$test_name: $status"
        fi
        
    else
        log_error "$component/$test_name failed"
        echo "$component,$test_name,ERROR,ERROR,ERROR,FAIL" >> "$CSV_FILE"
        
        # Add error to report
        {
            echo ""
            echo "=== $component: $test_name ==="
            echo "Status: FAIL"
            echo "Error output:"
            cat "$output_file"
            echo ""
        } >> "$REPORT_FILE"
    fi
}

# Function to run memory profiling
run_memory_profile() {
    local component=$1
    local package=$2
    local test_name=$3
    
    log "Profiling memory usage for $component/$test_name"
    
    if go test $package -bench="$test_name" -benchtime=60s -memprofile="$MEMORY_PROFILE" -run=^$ > /dev/null 2>&1; then
        log_success "Memory profile saved to $MEMORY_PROFILE"
        
        # Analyze memory profile
        if command -v go &> /dev/null; then
            local top_allocs=$(go tool pprof -top -alloc_space "$MEMORY_PROFILE" 2>/dev/null | head -20 || echo "Profile analysis failed")
            
            {
                echo ""
                echo "=== Memory Profile Analysis ==="
                echo "$top_allocs"
                echo ""
            } >> "$REPORT_FILE"
        fi
    else
        log_warning "Memory profiling failed for $component/$test_name"
    fi
}

# Function to run CPU profiling
run_cpu_profile() {
    local component=$1
    local package=$2
    local test_name=$3
    
    log "Profiling CPU usage for $component/$test_name"
    
    if go test $package -bench="$test_name" -benchtime=60s -cpuprofile="$CPU_PROFILE" -run=^$ > /dev/null 2>&1; then
        log_success "CPU profile saved to $CPU_PROFILE"
        
        # Analyze CPU profile
        if command -v go &> /dev/null; then
            local top_functions=$(go tool pprof -top "$CPU_PROFILE" 2>/dev/null | head -20 || echo "Profile analysis failed")
            
            {
                echo ""
                echo "=== CPU Profile Analysis ==="
                echo "$top_functions"
                echo ""
            } >> "$REPORT_FILE"
        fi
    else
        log_warning "CPU profiling failed for $component/$test_name"
    fi
}

# Build the project first
log "Building LSP Gateway..."
if make clean && make local; then
    log_success "Build completed"
else
    log_error "Build failed"
    exit 1
fi

# Run Gateway Benchmarks
log "=== Running Gateway Performance Benchmarks ==="
run_benchmark "Gateway" "./internal/gateway" "BenchmarkGatewayHTTPHandler"
run_benchmark "Gateway" "./internal/gateway" "BenchmarkRouterPerformance"
run_benchmark "Gateway" "./internal/gateway" "BenchmarkConcurrentClients"
run_benchmark "Gateway" "./internal/gateway" "BenchmarkLatencyProfile"
run_benchmark "Gateway" "./internal/gateway" "BenchmarkMemoryLeakDetection"
run_benchmark "Gateway" "./internal/gateway" "BenchmarkThroughput"
run_benchmark "Gateway" "./internal/gateway" "BenchmarkErrorHandling"

# Run MCP Benchmarks
log "=== Running MCP Performance Benchmarks ==="
run_benchmark "MCP" "./mcp" "BenchmarkMCPServerThroughput"
run_benchmark "MCP" "./mcp" "BenchmarkMCPMessageProcessing"
run_benchmark "MCP" "./mcp" "BenchmarkMCPConcurrentClients"
run_benchmark "MCP" "./mcp" "BenchmarkMCPProtocolParsing"
run_benchmark "MCP" "./mcp" "BenchmarkMCPMemoryUsage"
run_benchmark "MCP" "./mcp" "BenchmarkMCPLatencyProfile"
run_benchmark "MCP" "./mcp" "BenchmarkMCPErrorHandling"
run_benchmark "MCP" "./mcp" "BenchmarkMCPToolExecution"

# Run Transport Benchmarks
log "=== Running Transport Performance Benchmarks ==="
run_benchmark "Transport" "./internal/transport" "BenchmarkLSPClientLatency"
run_benchmark "Transport" "./internal/transport" "BenchmarkLSPClientThroughput"
run_benchmark "Transport" "./internal/transport" "BenchmarkLSPClientConcurrency"
run_benchmark "Transport" "./internal/transport" "BenchmarkLSPClientNotifications"
run_benchmark "Transport" "./internal/transport" "BenchmarkLSPClientMemoryUsage"
run_benchmark "Transport" "./internal/transport" "BenchmarkLSPMessageSerialization"
run_benchmark "Transport" "./internal/transport" "BenchmarkLSPClientErrorHandling"
run_benchmark "Transport" "./internal/transport" "BenchmarkLSPClientStartupShutdown"

# Run Load Tests
log "=== Running Load Test Scenarios ==="
run_benchmark "LoadTest" "./cmd/lsp-gateway" "BenchmarkProductionLoadTest"
run_benchmark "LoadTest" "./cmd/lsp-gateway" "BenchmarkMemoryLeakDetection"

# Run profiling on critical paths
log "=== Running Performance Profiling ==="
run_memory_profile "Gateway" "./internal/gateway" "BenchmarkGatewayHTTPHandler"
run_cpu_profile "Gateway" "./internal/gateway" "BenchmarkGatewayHTTPHandler"

# Generate summary
log "Generating performance summary..."

{
    echo ""
    echo "=== PERFORMANCE SUMMARY ==="
    echo ""
    
    # Count passes and failures
    local total_tests=$(grep -c "," "$CSV_FILE" | tail -1)
    local passed_tests=$(grep -c "PASS" "$CSV_FILE" || echo "0")
    local failed_tests=$(grep -c "FAIL" "$CSV_FILE" || echo "0")
    
    echo "Total Tests: $total_tests"
    echo "Passed: $passed_tests"
    echo "Failed: $failed_tests"
    
    if [[ $failed_tests -gt 0 ]]; then
        echo ""
        echo "FAILED TESTS:"
        grep "FAIL" "$CSV_FILE" | while IFS=',' read -r component test throughput latency memory status; do
            echo "  - $component/$test: $status"
        done
    fi
    
    echo ""
    echo "=== RECOMMENDATIONS ==="
    echo ""
    
    # Generate recommendations based on failures
    if grep -q "FAIL_LATENCY" "$CSV_FILE"; then
        echo "⚠ LATENCY ISSUES DETECTED:"
        echo "  - Some operations exceed ${TARGET_LATENCY_MS}ms target"
        echo "  - Consider optimizing critical path performance"
        echo "  - Review CPU profile for bottlenecks"
        echo ""
    fi
    
    if grep -q "FAIL_THROUGHPUT" "$CSV_FILE"; then
        echo "⚠ THROUGHPUT ISSUES DETECTED:"
        echo "  - Some operations below ${TARGET_THROUGHPUT_RPS} req/sec target"
        echo "  - Consider increasing concurrency"
        echo "  - Review connection pooling strategies"
        echo ""
    fi
    
    if [[ $failed_tests -eq 0 ]]; then
        echo "✓ ALL PERFORMANCE TARGETS MET"
        echo "  - System ready for production deployment"
        echo "  - Consider stress testing with higher loads"
        echo ""
    fi
    
    echo "Files generated:"
    echo "  - Full report: $REPORT_FILE"
    echo "  - CSV data: $CSV_FILE"
    echo "  - Memory profile: $MEMORY_PROFILE"
    echo "  - CPU profile: $CPU_PROFILE"
    
} >> "$REPORT_FILE"

# Final status
if [[ $failed_tests -eq 0 ]]; then
    log_success "All performance benchmarks completed successfully!"
    log "🎯 Performance targets met: Latency < ${TARGET_LATENCY_MS}ms, Throughput > ${TARGET_THROUGHPUT_RPS} req/sec"
else
    log_warning "Some performance benchmarks failed or below targets"
    log "⚠️  Check $REPORT_FILE for details and recommendations"
fi

log "Complete benchmark report available at: $REPORT_FILE"
log "CSV data available at: $CSV_FILE"

# Optional: Open report in default text editor if available
if command -v code &> /dev/null; then
    log "Opening report in VS Code..."
    code "$REPORT_FILE"
elif command -v nano &> /dev/null; then
    log "Press any key to view report in nano..."
    read -n 1 -s
    nano "$REPORT_FILE"
fi