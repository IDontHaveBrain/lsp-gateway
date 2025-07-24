#!/bin/bash

# LSP Gateway LSP Validation Test Suite
# Comprehensive LSP server validation testing with detailed reporting

set -euo pipefail

# Configuration
TEST_OUTPUT_DIR="./lsp-validation-test-results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
TEST_REPORT="${TEST_OUTPUT_DIR}/lsp_validation_test_report_${TIMESTAMP}.txt"
COVERAGE_FILE="${TEST_OUTPUT_DIR}/lsp_validation_coverage_${TIMESTAMP}.out"
COVERAGE_HTML="${TEST_OUTPUT_DIR}/lsp_validation_coverage_${TIMESTAMP}.html"

# Test timeouts (in seconds)
LSP_VALIDATION_TIMEOUT=60
LSP_REPOSITORY_TIMEOUT=120
LSP_INTEGRATION_TIMEOUT=180

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
mkdir -p "$TEST_OUTPUT_DIR"

# Initialize report
cat > "$TEST_REPORT" << EOF
LSP Gateway LSP Validation Test Report
======================================
Generated: $(date)
Go Version: $(go version)
System: $(uname -a)
Working Directory: $(pwd)

Test Configuration:
- LSP Validation Tests: ${LSP_VALIDATION_TIMEOUT}s timeout
- LSP Repository Tests: ${LSP_REPOSITORY_TIMEOUT}s timeout  
- LSP Integration Tests: ${LSP_INTEGRATION_TIMEOUT}s timeout

EOF

log "Starting LSP Gateway LSP Validation Test Suite"
log "Results will be saved to: $TEST_REPORT"

# Function to run test suite and capture results
run_test_suite() {
    local component=$1
    local package=$2
    local timeout=$3
    local test_pattern=${4:-"LSPValidation"}
    local additional_flags=${5:-""}
    
    log "Running $component LSP validation tests..."
    
    local output_file="${TEST_OUTPUT_DIR}/${component}_${TIMESTAMP}.txt"
    local test_flags="-v -timeout=${timeout}s -run=$test_pattern $additional_flags"
    
    # Run tests and capture output
    if go test $package $test_flags > "$output_file" 2>&1; then
        log_success "$component LSP validation tests passed"
        
        # Extract test statistics
        local total_tests=$(grep -c "=== RUN" "$output_file" || echo "0")
        local passed_tests=$(grep -c "--- PASS:" "$output_file" || echo "0") 
        local failed_tests=$(grep -c "--- FAIL:" "$output_file" || echo "0")
        local skipped_tests=$(grep -c "--- SKIP:" "$output_file" || echo "0")
        
        # Add to report
        {
            echo ""
            echo "=== $component LSP Validation Tests ==="
            echo "Status: PASS"
            echo "Total Tests: $total_tests"
            echo "Passed: $passed_tests"
            echo "Failed: $failed_tests" 
            echo "Skipped: $skipped_tests"
            echo ""
            echo "Detailed Output:"
            cat "$output_file"
            echo ""
        } >> "$TEST_REPORT"
        
        return 0
    else
        log_error "$component LSP validation tests failed"
        
        # Extract error information
        local total_tests=$(grep -c "=== RUN" "$output_file" || echo "0")
        local passed_tests=$(grep -c "--- PASS:" "$output_file" || echo "0")
        local failed_tests=$(grep -c "--- FAIL:" "$output_file" || echo "0")
        
        # Add failure to report
        {
            echo ""
            echo "=== $component LSP Validation Tests ==="
            echo "Status: FAIL"
            echo "Total Tests: $total_tests"
            echo "Passed: $passed_tests"
            echo "Failed: $failed_tests"
            echo ""
            echo "Error Output:"
            cat "$output_file"
            echo ""
        } >> "$TEST_REPORT"
        
        return 1
    fi
}

# Function to run coverage analysis
run_coverage_analysis() {
    log "Running LSP validation test coverage analysis..."
    
    local packages="./internal/gateway ./internal/transport ./mcp ./internal/setup"
    local coverage_flags="-coverprofile=$COVERAGE_FILE -covermode=atomic"
    
    if go test $packages -run=LSPValidation -timeout=180s $coverage_flags > /dev/null 2>&1; then
        log_success "Coverage analysis completed"
        
        # Generate HTML coverage report
        if go tool cover -html="$COVERAGE_FILE" -o "$COVERAGE_HTML"; then
            log_success "Coverage HTML report generated: $COVERAGE_HTML"
        fi
        
        # Extract coverage statistics
        local coverage_percent=$(go tool cover -func="$COVERAGE_FILE" | tail -1 | awk '{print $3}' || echo "N/A")
        
        {
            echo ""
            echo "=== LSP Validation Test Coverage ==="
            echo "Overall Coverage: $coverage_percent"
            echo "Coverage Report: $COVERAGE_HTML"
            echo ""
            echo "Detailed Coverage:"
            go tool cover -func="$COVERAGE_FILE" 2>/dev/null || echo "Coverage analysis failed"
            echo ""
        } >> "$TEST_REPORT"
        
    else
        log_warning "Coverage analysis failed"
        {
            echo ""
            echo "=== LSP Validation Test Coverage ==="
            echo "Status: FAILED"
            echo "Coverage analysis could not be completed"
            echo ""
        } >> "$TEST_REPORT"
    fi
}

# Function to validate environment
validate_environment() {
    log "Validating test environment..."
    
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
    
    # Verify binary works
    if ! ./bin/lsp-gateway version > /dev/null 2>&1; then
        log_error "LSP Gateway binary is not functional"
        exit 1
    fi
    
    log_success "Environment validation passed"
}

# Function to run repository-focused tests
run_repository_tests() {
    log "=== Running LSP Repository Validation Tests ==="
    
    local packages="./internal/gateway ./internal/transport ./mcp"
    local repo_flags="-timeout=${LSP_REPOSITORY_TIMEOUT}s -run=LSPRepository"
    
    if go test $packages $repo_flags -v > "${TEST_OUTPUT_DIR}/repository_tests_${TIMESTAMP}.txt" 2>&1; then
        log_success "LSP repository validation tests passed"
        return 0
    else
        log_error "LSP repository validation tests failed"
        return 1
    fi
}

# Function to run short validation tests
run_short_tests() {
    log "=== Running Short LSP Validation Tests (CI Mode) ==="
    
    local packages="./internal/gateway ./internal/transport ./mcp"
    local short_flags="-short -timeout=60s -run=LSPValidation"
    
    if go test $packages $short_flags -v > "${TEST_OUTPUT_DIR}/short_validation_tests_${TIMESTAMP}.txt" 2>&1; then
        log_success "Short LSP validation tests passed"
        return 0
    else
        log_error "Short LSP validation tests failed"
        return 1
    fi
}

# Function to run benchmark tests
run_benchmark_tests() {
    log "=== Running LSP Validation Benchmark Tests ==="
    
    local packages="./internal/gateway ./internal/transport ./mcp"
    local bench_flags="-bench=LSPValidation -benchmem -timeout=300s"
    
    if go test $packages $bench_flags -v > "${TEST_OUTPUT_DIR}/benchmark_tests_${TIMESTAMP}.txt" 2>&1; then
        log_success "LSP validation benchmark tests completed"
        
        # Add benchmark results to report
        {
            echo ""
            echo "=== LSP Validation Benchmark Results ==="
            cat "${TEST_OUTPUT_DIR}/benchmark_tests_${TIMESTAMP}.txt"
            echo ""
        } >> "$TEST_REPORT"
        
        return 0
    else
        log_error "LSP validation benchmark tests failed"
        return 1
    fi
}

# Main execution
main() {
    local test_mode=${1:-"full"}
    local exit_code=0
    
    # Validate environment first
    validate_environment
    
    case "$test_mode" in
        "short"|"ci")
            log "Running LSP validation tests in SHORT mode"
            if ! run_short_tests; then
                exit_code=1
            fi
            ;;
        "repository"|"repo")
            log "Running LSP validation tests in REPOSITORY mode"
            if ! run_repository_tests; then
                exit_code=1
            fi
            ;;
        "benchmark"|"bench")
            log "Running LSP validation tests in BENCHMARK mode"
            if ! run_benchmark_tests; then
                exit_code=1
            fi
            ;;
        "full"|*)
            log "Running LSP validation tests in FULL mode"
            
            # Run each component's LSP validation tests
            if ! run_test_suite "Gateway" "./internal/gateway" "$LSP_VALIDATION_TIMEOUT" "LSPValidation"; then
                exit_code=1
            fi
            
            if ! run_test_suite "Transport" "./internal/transport" "$LSP_VALIDATION_TIMEOUT" "LSPValidation"; then
                exit_code=1
            fi
            
            if ! run_test_suite "MCP" "./mcp" "$LSP_VALIDATION_TIMEOUT" "LSPValidation"; then
                exit_code=1
            fi
            
            if ! run_test_suite "Setup" "./internal/setup" "$LSP_VALIDATION_TIMEOUT" "LSPValidation"; then
                exit_code=1
            fi
            
            # Run coverage analysis
            run_coverage_analysis
            
            # Run repository tests as part of full suite
            if ! run_repository_tests; then
                log_warning "Repository tests failed but continuing with full suite"
            fi
            ;;
    esac
    
    # Generate final summary
    {
        echo ""
        echo "=== LSP VALIDATION TEST SUMMARY ==="
        echo ""
        echo "Test Mode: $test_mode"
        echo "Completion Time: $(date)"
        echo ""
        
        if [[ $exit_code -eq 0 ]]; then
            echo "✓ ALL LSP VALIDATION TESTS PASSED"
            echo ""
            echo "The LSP Gateway LSP validation test suite completed successfully."
            echo "All LSP server integrations are functioning correctly."
        else
            echo "✗ SOME LSP VALIDATION TESTS FAILED"
            echo ""
            echo "Please review the detailed output above for specific failures."
            echo "Common issues and solutions can be found in docs/LSP_VALIDATION.md"
        fi
        
        echo ""
        echo "Generated Files:"
        echo "  - Test Report: $TEST_REPORT"
        if [[ -f "$COVERAGE_FILE" ]]; then
            echo "  - Coverage Data: $COVERAGE_FILE"
            echo "  - Coverage HTML: $COVERAGE_HTML"
        fi
        
        local log_files=($(find "$TEST_OUTPUT_DIR" -name "*_${TIMESTAMP}.txt" -type f))
        if [[ ${#log_files[@]} -gt 0 ]]; then
            echo "  - Component Logs:"
            for log_file in "${log_files[@]}"; do
                echo "    - $(basename "$log_file")"
            done
        fi
        
    } >> "$TEST_REPORT"
    
    # Final status
    if [[ $exit_code -eq 0 ]]; then
        log_success "LSP validation test suite completed successfully!"
        log "📋 Full report available at: $TEST_REPORT"
        if [[ -f "$COVERAGE_HTML" ]]; then
            log "📊 Coverage report available at: $COVERAGE_HTML"
        fi
    else
        log_error "LSP validation test suite completed with failures"
        log "📋 Check detailed report at: $TEST_REPORT"
    fi
    
    return $exit_code
}

# Help function
show_help() {
    cat << EOF
LSP Gateway LSP Validation Test Suite

Usage: $0 [MODE]

Modes:
  full         Run complete LSP validation test suite (default)
  short        Run abbreviated test suite  
  ci           Alias for 'short'
  repository   Run repository-focused validation tests
  repo         Alias for 'repository'
  benchmark    Run LSP validation benchmark tests
  bench        Alias for 'benchmark'
  help         Show this help message

Examples:
  $0                    # Run full LSP validation test suite
  $0 full              # Run full LSP validation test suite  
  $0 short             # Run abbreviated test suite
  $0 ci                # Run CI-friendly test suite
  $0 repository        # Run repository validation tests
  $0 benchmark         # Run benchmark tests

Environment Variables:
  LSP_VALIDATION_TEST_TIMEOUT    Override default test timeouts
  LSP_VALIDATION_TEST_VERBOSE    Enable verbose test output
  LSP_VALIDATION_TEST_COVERAGE   Force coverage analysis

For detailed documentation, see: docs/LSP_VALIDATION.md
EOF
}

# Handle command line arguments
case "${1:-full}" in
    "help"|"-h"|"--help")
        show_help
        exit 0
        ;;
    *)
        main "$@"
        exit $?
        ;;
esac