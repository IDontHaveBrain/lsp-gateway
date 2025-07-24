#!/bin/bash
set -euo pipefail

# CI Test Execution Script for LSP Gateway
# Comprehensive test runner with reporting and error handling

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
RESULTS_DIR="${PROJECT_ROOT}/test-results"
COVERAGE_DIR="${RESULTS_DIR}/coverage"
REPORTS_DIR="${RESULTS_DIR}/reports"

# Configuration
DEFAULT_TIMEOUT="300s"
UNIT_TEST_TIMEOUT="60s"
INTEGRATION_TEST_TIMEOUT="600s"
PERFORMANCE_TEST_TIMEOUT="1800s"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Help function
show_help() {
    cat << EOF
LSP Gateway CI Test Runner

Usage: $0 [OPTIONS] [TEST_TYPE]

TEST_TYPE:
    all             Run all tests (default)
    unit            Run unit tests only
    integration     Run integration tests only
    performance     Run performance tests only
    lsp-validation  Run LSP validation tests
    quick           Run quick validation tests

OPTIONS:
    -h, --help      Show this help message
    -v, --verbose   Enable verbose output
    -c, --coverage  Enable coverage reporting
    -r, --race      Enable race detection
    -p, --parallel  Set parallel test count (default: auto)
    -t, --timeout   Set test timeout (default: $DEFAULT_TIMEOUT)
    --xml           Generate JUnit XML reports
    --html          Generate HTML coverage reports
    --clean         Clean previous test results
    --fail-fast     Stop on first test failure

Examples:
    $0 unit -c --xml                 # Run unit tests with coverage and XML reports
    $0 integration -v -t 10m         # Run integration tests with 10 minute timeout
    $0 performance --clean           # Run performance tests after cleaning results
    $0 all -c --html --xml           # Run all tests with full reporting

EOF
}

# Initialize directories
init_dirs() {
    log_info "Initializing test directories..."
    mkdir -p "$RESULTS_DIR" "$COVERAGE_DIR" "$REPORTS_DIR"
    
    if [[ "${CLEAN:-false}" == "true" ]]; then
        log_info "Cleaning previous test results..."
        rm -rf "${RESULTS_DIR:?}"/*
        mkdir -p "$RESULTS_DIR" "$COVERAGE_DIR" "$REPORTS_DIR"
    fi
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check Go installation
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed or not in PATH"
        exit 1
    fi
    
    # Check Go version
    GO_VERSION=$(go version | grep -o 'go[0-9]\+\.[0-9]\+\.[0-9]\+' | sed 's/go//')
    log_info "Using Go version: $GO_VERSION"
    
    # Check if binary exists for integration tests
    if [[ "$TEST_TYPE" == "integration" || "$TEST_TYPE" == "all" || "$TEST_TYPE" == "lsp-validation" ]]; then
        if [[ ! -f "$PROJECT_ROOT/bin/lsp-gateway" ]]; then
            log_warning "LSP Gateway binary not found, building..."
            cd "$PROJECT_ROOT"
            make local
        fi
    fi
    
    # Install test dependencies if needed
    if [[ "${COVERAGE:-false}" == "true" ]]; then
        if ! command -v gocov &> /dev/null; then
            log_info "Installing gocov for coverage reporting..."
            go install github.com/axw/gocov/gocov@latest
        fi
        
        if [[ "${HTML_COVERAGE:-false}" == "true" ]] && ! command -v gocov-html &> /dev/null; then
            log_info "Installing gocov-html for HTML coverage reports..."
            go install github.com/matm/gocov-html@latest
        fi
    fi
    
    if [[ "${XML_REPORTS:-false}" == "true" ]] && ! command -v go-junit-report &> /dev/null; then
        log_info "Installing go-junit-report for JUnit XML reports..."
        go install github.com/jstemmer/go-junit-report@latest
    fi
}

# Setup test environment
setup_test_environment() {
    log_info "Setting up test environment..."
    
    cd "$PROJECT_ROOT"
    
    # Set environment variables
    export CGO_ENABLED=1
    export GO111MODULE=on
    export GOPROXY=https://proxy.golang.org,direct
    export GOSUMDB=sum.golang.org
    
    # Add binary to PATH for integration tests
    export PATH="$PROJECT_ROOT/bin:$PATH"
    
    # Create test workspace
    TEST_WORKSPACE="$PROJECT_ROOT/test-workspace"
    mkdir -p "$TEST_WORKSPACE"
    
    # Create sample files for different languages
    cat > "$TEST_WORKSPACE/main.go" << 'EOF'
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}

func add(a, b int) int {
    return a + b
}
EOF
    
    cat > "$TEST_WORKSPACE/main.py" << 'EOF'
def main():
    print("Hello, Python!")

def add(a, b):
    return a + b

if __name__ == "__main__":
    main()
EOF
    
    cat > "$TEST_WORKSPACE/main.ts" << 'EOF'
function main(): void {
    console.log("Hello, TypeScript!");
}

function add(a: number, b: number): number {
    return a + b;
}

main();
EOF
    
    cat > "$TEST_WORKSPACE/Main.java" << 'EOF'
public class Main {
    public static void main(String[] args) {
        System.out.println("Hello, Java!");
    }
    
    public static int add(int a, int b) {
        return a + b;
    }
}
EOF
    
    log_success "Test environment setup completed"
}

# Build test command arguments
build_test_args() {
    local test_args=()
    
    # Basic arguments
    test_args+=("-v")
    test_args+=("-timeout" "${TIMEOUT}")
    
    # Coverage
    if [[ "${COVERAGE:-false}" == "true" ]]; then
        test_args+=("-coverprofile=$COVERAGE_DIR/coverage.out")
        test_args+=("-covermode=atomic")
    fi
    
    # Race detection
    if [[ "${RACE:-false}" == "true" ]]; then
        test_args+=("-race")
    fi
    
    # Parallel execution
    if [[ -n "${PARALLEL:-}" ]]; then
        test_args+=("-parallel" "$PARALLEL")
    fi
    
    # Fail fast
    if [[ "${FAIL_FAST:-false}" == "true" ]]; then
        test_args+=("-failfast")
    fi
    
    echo "${test_args[@]}"
}

# Run unit tests
run_unit_tests() {
    log_info "Running unit tests..."
    
    local test_args
    test_args=($(build_test_args))
    test_args+=("-short")
    
    local output_file="$RESULTS_DIR/unit-tests.log"
    local junit_file="$REPORTS_DIR/unit-tests.xml"
    
    cd "$PROJECT_ROOT"
    
    if [[ "${XML_REPORTS:-false}" == "true" ]]; then
        go test "${test_args[@]}" ./... 2>&1 | tee "$output_file" | go-junit-report > "$junit_file"
        local exit_code=${PIPESTATUS[0]}
    else
        go test "${test_args[@]}" ./... 2>&1 | tee "$output_file"
        local exit_code=${PIPESTATUS[0]}
    fi
    
    if [[ $exit_code -eq 0 ]]; then
        log_success "Unit tests passed"
    else
        log_error "Unit tests failed (exit code: $exit_code)"
        return $exit_code
    fi
}

# Run integration tests
run_integration_tests() {
    log_info "Running integration tests..."
    
    local test_args
    test_args=($(build_test_args))
    test_args+=("-run" "Integration")
    
    local output_file="$RESULTS_DIR/integration-tests.log"
    local junit_file="$REPORTS_DIR/integration-tests.xml"
    
    cd "$PROJECT_ROOT"
    
    if [[ "${XML_REPORTS:-false}" == "true" ]]; then
        go test "${test_args[@]}" ./tests/integration/... 2>&1 | tee "$output_file" | go-junit-report > "$junit_file"
        local exit_code=${PIPESTATUS[0]}
    else
        go test "${test_args[@]}" ./tests/integration/... 2>&1 | tee "$output_file"
        local exit_code=${PIPESTATUS[0]}
    fi
    
    if [[ $exit_code -eq 0 ]]; then
        log_success "Integration tests passed"
    else
        log_error "Integration tests failed (exit code: $exit_code)"
        return $exit_code
    fi
}

# Run performance tests
run_performance_tests() {
    log_info "Running performance tests..."
    
    local test_args
    test_args=($(build_test_args))
    test_args+=("-bench=.")
    test_args+=("-benchmem")
    test_args+=("-count=3")
    
    local output_file="$RESULTS_DIR/performance-tests.log"
    local bench_file="$RESULTS_DIR/benchmark-results.txt"
    
    cd "$PROJECT_ROOT"
    
    go test "${test_args[@]}" ./tests/performance/... 2>&1 | tee "$output_file"
    local exit_code=${PIPESTATUS[0]}
    
    # Extract benchmark results
    grep "^Benchmark" "$output_file" > "$bench_file" || true
    
    if [[ $exit_code -eq 0 ]]; then
        log_success "Performance tests completed"
    else
        log_error "Performance tests failed (exit code: $exit_code)"
        return $exit_code
    fi
}

# Run LSP validation tests
run_lsp_validation_tests() {
    log_info "Running LSP validation tests..."
    
    cd "$PROJECT_ROOT"
    
    # Use makefile targets for LSP validation
    local output_file="$RESULTS_DIR/lsp-validation.log"
    
    make test-lsp-validation 2>&1 | tee "$output_file"
    local exit_code=${PIPESTATUS[0]}
    
    if [[ $exit_code -eq 0 ]]; then
        log_success "LSP validation tests passed"
    else
        log_error "LSP validation tests failed (exit code: $exit_code)"
        return $exit_code
    fi
}

# Run quick tests
run_quick_tests() {
    log_info "Running quick validation tests..."
    
    cd "$PROJECT_ROOT"
    
    local output_file="$RESULTS_DIR/quick-tests.log"
    
    make test-simple-quick 2>&1 | tee "$output_file"
    local exit_code=${PIPESTATUS[0]}
    
    if [[ $exit_code -eq 0 ]]; then
        log_success "Quick tests passed"
    else
        log_error "Quick tests failed (exit code: $exit_code)"
        return $exit_code
    fi
}

# Generate coverage reports
generate_coverage_reports() {
    if [[ "${COVERAGE:-false}" != "true" ]]; then
        return 0
    fi
    
    log_info "Generating coverage reports..."
    
    local coverage_file="$COVERAGE_DIR/coverage.out"
    
    if [[ ! -f "$coverage_file" ]]; then
        log_warning "Coverage file not found, skipping coverage reports"
        return 0
    fi
    
    # Generate text coverage summary
    go tool cover -func="$coverage_file" > "$COVERAGE_DIR/coverage-summary.txt"
    
    # Generate HTML coverage report
    if [[ "${HTML_COVERAGE:-false}" == "true" ]]; then
        go tool cover -html="$coverage_file" -o "$COVERAGE_DIR/coverage.html"
        log_success "HTML coverage report generated: $COVERAGE_DIR/coverage.html"
    fi
    
    # Extract coverage percentage
    local coverage_percent
    coverage_percent=$(go tool cover -func="$coverage_file" | grep "^total:" | awk '{print $3}' | sed 's/%//')
    
    echo "COVERAGE_PERCENT=$coverage_percent" > "$RESULTS_DIR/coverage-env.txt"
    log_success "Total coverage: ${coverage_percent}%"
}

# Generate test summary
generate_test_summary() {
    log_info "Generating test summary..."
    
    local summary_file="$REPORTS_DIR/test-summary.md"
    
    cat > "$summary_file" << EOF
# Test Execution Summary

**Date:** $(date)
**Test Type:** $TEST_TYPE
**Git Commit:** $(git rev-parse HEAD 2>/dev/null || echo "unknown")
**Git Branch:** $(git branch --show-current 2>/dev/null || echo "unknown")

## Test Results

EOF
    
    # Add test results
    if [[ -f "$RESULTS_DIR/unit-tests.log" ]]; then
        echo "### Unit Tests" >> "$summary_file"
        if grep -q "PASS" "$RESULTS_DIR/unit-tests.log"; then
            echo "✅ **PASSED**" >> "$summary_file"
        else
            echo "❌ **FAILED**" >> "$summary_file"
        fi
        echo "" >> "$summary_file"
    fi
    
    if [[ -f "$RESULTS_DIR/integration-tests.log" ]]; then
        echo "### Integration Tests" >> "$summary_file"
        if grep -q "PASS" "$RESULTS_DIR/integration-tests.log"; then
            echo "✅ **PASSED**" >> "$summary_file"
        else
            echo "❌ **FAILED**" >> "$summary_file"
        fi
        echo "" >> "$summary_file"
    fi
    
    if [[ -f "$RESULTS_DIR/performance-tests.log" ]]; then
        echo "### Performance Tests" >> "$summary_file"
        if grep -q "PASS" "$RESULTS_DIR/performance-tests.log"; then
            echo "✅ **COMPLETED**" >> "$summary_file"
        else
            echo "❌ **FAILED**" >> "$summary_file"
        fi
        echo "" >> "$summary_file"
    fi
    
    # Add coverage information
    if [[ -f "$COVERAGE_DIR/coverage-summary.txt" ]]; then
        echo "### Code Coverage" >> "$summary_file"
        local coverage_percent
        coverage_percent=$(grep "^total:" "$COVERAGE_DIR/coverage-summary.txt" | awk '{print $3}')
        echo "**Total Coverage:** $coverage_percent" >> "$summary_file"
        echo "" >> "$summary_file"
    fi
    
    log_success "Test summary generated: $summary_file"
}

# Main execution function
main() {
    local start_time
    start_time=$(date +%s)
    
    log_info "Starting LSP Gateway CI test execution..."
    log_info "Test type: $TEST_TYPE"
    log_info "Project root: $PROJECT_ROOT"
    
    init_dirs
    check_prerequisites
    setup_test_environment
    
    local overall_exit_code=0
    
    case "$TEST_TYPE" in
        "unit")
            run_unit_tests || overall_exit_code=$?
            ;;
        "integration")
            run_integration_tests || overall_exit_code=$?
            ;;
        "performance")
            run_performance_tests || overall_exit_code=$?
            ;;
        "lsp-validation")
            run_lsp_validation_tests || overall_exit_code=$?
            ;;
        "quick")
            run_quick_tests || overall_exit_code=$?
            ;;
        "all")
            run_unit_tests || overall_exit_code=$?
            if [[ $overall_exit_code -eq 0 || "${FAIL_FAST:-false}" != "true" ]]; then
                run_integration_tests || overall_exit_code=$?
            fi
            if [[ $overall_exit_code -eq 0 || "${FAIL_FAST:-false}" != "true" ]]; then
                run_performance_tests || overall_exit_code=$?
            fi
            ;;
        *)
            log_error "Unknown test type: $TEST_TYPE"
            show_help
            exit 1
            ;;
    esac
    
    generate_coverage_reports
    generate_test_summary
    
    local end_time
    end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    log_info "Test execution completed in ${duration}s"
    
    if [[ $overall_exit_code -eq 0 ]]; then
        log_success "All tests completed successfully!"
    else
        log_error "Some tests failed (exit code: $overall_exit_code)"
    fi
    
    exit $overall_exit_code
}

# Parse command line arguments
TEST_TYPE="all"
VERBOSE=false
COVERAGE=false
RACE=false
PARALLEL=""
TIMEOUT="$DEFAULT_TIMEOUT"
XML_REPORTS=false
HTML_COVERAGE=false
CLEAN=false
FAIL_FAST=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -v|--verbose)
            VERBOSE=true
            set -x
            shift
            ;;
        -c|--coverage)
            COVERAGE=true
            shift
            ;;
        -r|--race)
            RACE=true
            shift
            ;;
        -p|--parallel)
            PARALLEL="$2"
            shift 2
            ;;
        -t|--timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        --xml)
            XML_REPORTS=true
            shift
            ;;
        --html)
            HTML_COVERAGE=true
            shift
            ;;
        --clean)
            CLEAN=true
            shift
            ;;
        --fail-fast)
            FAIL_FAST=true
            shift
            ;;
        unit|integration|performance|lsp-validation|quick|all)
            TEST_TYPE="$1"
            shift
            ;;
        *)
            log_error "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Adjust timeout based on test type
case "$TEST_TYPE" in
    "unit"|"quick")
        TIMEOUT="${TIMEOUT:-$UNIT_TEST_TIMEOUT}"
        ;;
    "integration"|"lsp-validation")
        TIMEOUT="${TIMEOUT:-$INTEGRATION_TEST_TIMEOUT}"
        ;;
    "performance")
        TIMEOUT="${TIMEOUT:-$PERFORMANCE_TEST_TIMEOUT}"
        ;;
esac

# Run main function
main "$@"