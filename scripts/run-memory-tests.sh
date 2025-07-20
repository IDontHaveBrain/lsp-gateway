#!/bin/bash

# LSP Gateway Memory Leak Testing Script
# This script demonstrates how to run the comprehensive memory leak detection tests

set -e

echo "🧪 LSP Gateway Memory Leak Detection Test Suite"
echo "================================================"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
TIMEOUT_SHORT="5m"
TIMEOUT_MEDIUM="30m"
TIMEOUT_LONG="2h"
TEST_DIR="./internal/integration/"

echo -e "${BLUE}Available Memory Tests:${NC}"
echo "1. Demo Tests (Quick validation)"
echo "2. Short Memory Tests (5 minutes)"
echo "3. Medium Memory Tests (30 minutes)"
echo "4. Long-Running Memory Tests (1+ hours)"
echo "5. Memory Benchmarks"
echo ""

# Function to run a test with nice output
run_test() {
    local test_name="$1"
    local test_pattern="$2"
    local timeout="$3"
    local additional_flags="$4"
    
    echo -e "${YELLOW}Running: ${test_name}${NC}"
    echo "Command: go test ${TEST_DIR} -run ${test_pattern} -timeout=${timeout} ${additional_flags} -v"
    echo ""
    
    if go test ${TEST_DIR} -run "${test_pattern}" -timeout="${timeout}" ${additional_flags} -v; then
        echo -e "${GREEN}✅ ${test_name} PASSED${NC}"
    else
        echo -e "${RED}❌ ${test_name} FAILED${NC}"
        return 1
    fi
    echo ""
}

# Function to run benchmarks
run_benchmark() {
    local bench_name="$1"
    local bench_pattern="$2"
    local timeout="$3"
    
    echo -e "${YELLOW}Running: ${bench_name}${NC}"
    echo "Command: go test ${TEST_DIR} -bench=${bench_pattern} -timeout=${timeout} -benchmem"
    echo ""
    
    if go test ${TEST_DIR} -bench="${bench_pattern}" -timeout="${timeout}" -benchmem; then
        echo -e "${GREEN}✅ ${bench_name} COMPLETED${NC}"
    else
        echo -e "${RED}❌ ${bench_name} FAILED${NC}"
        return 1
    fi
    echo ""
}

# Parse command line arguments
case "${1:-demo}" in
    "demo"|"1")
        echo -e "${BLUE}Running Demo Tests (Quick Validation)${NC}"
        echo "These tests verify that the memory leak detection infrastructure works correctly."
        echo ""
        
        run_test "Memory Profiler Basic Functionality" "TestMemoryProfilerBasicFunctionality" "${TIMEOUT_SHORT}" ""
        run_test "Memory Leak Detection Algorithm" "TestMemoryLeakDetectionAlgorithm" "${TIMEOUT_SHORT}" ""
        run_test "Test Harness Basic Setup" "TestTestHarnessBasicSetup" "${TIMEOUT_SHORT}" ""
        
        echo -e "${GREEN}✅ All demo tests completed successfully!${NC}"
        echo "The memory leak detection infrastructure is working correctly."
        ;;
        
    "short"|"2")
        echo -e "${BLUE}Running Short Memory Tests (5 minutes)${NC}"
        echo "These tests run memory analysis for a short duration with aggressive sampling."
        echo ""
        
        # Set short test duration
        export LSP_GATEWAY_LONG_TEST_DURATION="2m"
        
        run_test "Short Memory Stability Test" "TestLongRunningGatewayMemoryStability" "${TIMEOUT_SHORT}" ""
        run_test "Short Server Restart Cycles" "TestServerRestartCycles" "${TIMEOUT_SHORT}" "-short"
        
        echo -e "${GREEN}✅ Short memory tests completed!${NC}"
        ;;
        
    "medium"|"3")
        echo -e "${BLUE}Running Medium Memory Tests (30 minutes)${NC}"
        echo "These tests run extended memory analysis to catch gradual leaks."
        echo ""
        
        # Set medium test duration
        export LSP_GATEWAY_LONG_TEST_DURATION="15m"
        
        run_test "Medium Memory Stability Test" "TestLongRunningGatewayMemoryStability" "${TIMEOUT_MEDIUM}" ""
        run_test "Server Restart Cycles Test" "TestServerRestartCycles" "${TIMEOUT_MEDIUM}" ""
        run_test "Large Document Processing Test" "TestLargeDocumentProcessing" "${TIMEOUT_MEDIUM}" ""
        
        echo -e "${GREEN}✅ Medium memory tests completed!${NC}"
        ;;
        
    "long"|"4")
        echo -e "${BLUE}Running Long-Running Memory Tests (1+ hours)${NC}"
        echo "⚠️  These tests run for extended periods to detect subtle memory leaks."
        echo "Use these for comprehensive memory safety validation."
        echo ""
        
        # Check if user really wants to run long tests
        read -p "Are you sure you want to run 1+ hour tests? (y/N): " -n 1 -r
        echo ""
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            echo "Long tests cancelled."
            exit 0
        fi
        
        # Set long test duration (default 1 hour, can be overridden)
        export LSP_GATEWAY_LONG_TEST_DURATION="${LSP_GATEWAY_LONG_TEST_DURATION:-1h}"
        
        echo "Test duration: ${LSP_GATEWAY_LONG_TEST_DURATION}"
        echo "Starting long-running tests..."
        echo ""
        
        run_test "Long-Running Memory Stability Test" "TestLongRunningGatewayMemoryStability" "${TIMEOUT_LONG}" ""
        run_test "Extended Server Restart Cycles" "TestServerRestartCycles" "${TIMEOUT_LONG}" ""
        run_test "Large Document Processing Test" "TestLargeDocumentProcessing" "${TIMEOUT_LONG}" ""
        
        echo -e "${GREEN}✅ Long-running memory tests completed!${NC}"
        echo "Check the test output above for any memory leak warnings."
        ;;
        
    "bench"|"5")
        echo -e "${BLUE}Running Memory Benchmarks${NC}"
        echo "These benchmarks measure memory efficiency and performance characteristics."
        echo ""
        
        run_benchmark "Memory Efficiency Benchmark" "BenchmarkMemoryEfficiency" "${TIMEOUT_MEDIUM}"
        
        echo -e "${GREEN}✅ Memory benchmarks completed!${NC}"
        ;;
        
    "all")
        echo -e "${BLUE}Running All Memory Tests${NC}"
        echo "⚠️  This will run all tests including long-running ones!"
        echo ""
        
        read -p "Are you sure you want to run ALL tests including 1+ hour tests? (y/N): " -n 1 -r
        echo ""
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            echo "Full test suite cancelled."
            exit 0
        fi
        
        # Run all test categories
        "$0" demo
        "$0" short
        "$0" medium
        "$0" bench
        
        echo -e "${GREEN}✅ Complete memory test suite finished!${NC}"
        ;;
        
    "help"|"-h"|"--help")
        echo "Usage: $0 [test_type]"
        echo ""
        echo "Available test types:"
        echo "  demo, 1    - Quick demo tests (30 seconds)"
        echo "  short, 2   - Short memory tests (5 minutes)"
        echo "  medium, 3  - Medium memory tests (30 minutes)"
        echo "  long, 4    - Long-running tests (1+ hours)"
        echo "  bench, 5   - Memory benchmarks"
        echo "  all        - Run all tests (interactive confirmation)"
        echo "  help       - Show this help message"
        echo ""
        echo "Environment Variables:"
        echo "  LSP_GATEWAY_LONG_TEST_DURATION - Override test duration (e.g., '2h', '30m')"
        echo ""
        echo "Examples:"
        echo "  $0 demo                                    # Quick validation"
        echo "  $0 short                                   # 5-minute tests"
        echo "  LSP_GATEWAY_LONG_TEST_DURATION=2h $0 long # 2-hour tests"
        ;;
        
    *)
        echo -e "${RED}Unknown test type: $1${NC}"
        echo "Use '$0 help' to see available options."
        exit 1
        ;;
esac

echo ""
echo -e "${BLUE}Memory Testing Complete!${NC}"
echo ""
echo "📊 For detailed analysis:"
echo "  - Check test output above for memory growth statistics"
echo "  - Look for any 'MEMORY LEAK DETECTED' warnings"
echo "  - Review heap growth and goroutine growth numbers"
echo ""
echo "🔍 Memory profiles (if generated):"
echo "  - Use: go tool pprof memory_profile_*.pprof"
echo "  - View with: go tool pprof -http=:8080 profile.pprof"
echo ""
echo "📈 Key metrics to monitor:"
echo "  - Heap growth < 50MB for stability tests"
echo "  - Goroutine growth < 50"
echo "  - Error rate < 5%"
echo "  - Memory leak confidence < 70%"