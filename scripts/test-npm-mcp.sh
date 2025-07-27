#!/bin/bash

# test-npm-mcp.sh
# Comprehensive test runner for NPM-MCP functionality
# Tests both Node.js wrapper and Go integration components

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
TESTS_DIR="${PROJECT_ROOT}/tests"
E2E_DIR="${TESTS_DIR}/e2e"
VERBOSE=${VERBOSE:-false}
SKIP_BUILD=${SKIP_BUILD:-false}
SKIP_NPM_INSTALL=${SKIP_NPM_INSTALL:-false}

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

log_verbose() {
    if [[ "${VERBOSE}" == "true" ]]; then
        echo -e "${BLUE}[VERBOSE]${NC} $1"
    fi
}

# Print help
print_help() {
    cat << EOF
NPM-MCP Test Runner

Usage: $0 [OPTIONS]

Options:
    -h, --help              Show this help message
    -v, --verbose           Enable verbose output
    -s, --skip-build        Skip binary build step
    -n, --skip-npm-install  Skip npm install step
    --js-only               Run only JavaScript tests
    --go-only               Run only Go tests
    --quick                 Run quick tests only (skip long-running tests)

Environment Variables:
    VERBOSE                 Set to 'true' for verbose output
    SKIP_BUILD             Set to 'true' to skip binary build
    SKIP_NPM_INSTALL       Set to 'true' to skip npm install

Examples:
    $0                      Run all tests
    $0 --verbose            Run with verbose output
    $0 --js-only           Run only JavaScript tests
    $0 --quick             Run quick tests only

EOF
}

# Parse command line arguments
parse_args() {
    local js_only=false
    local go_only=false
    local quick=false

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
            -s|--skip-build)
                SKIP_BUILD=true
                shift
                ;;
            -n|--skip-npm-install)
                SKIP_NPM_INSTALL=true
                shift
                ;;
            --js-only)
                js_only=true
                shift
                ;;
            --go-only)
                go_only=true
                shift
                ;;
            --quick)
                quick=true
                shift
                ;;
            *)
                log_error "Unknown option: $1"
                print_help
                exit 1
                ;;
        esac
    done

    # Export parsed options for use in functions
    export JS_ONLY="${js_only}"
    export GO_ONLY="${go_only}"
    export QUICK="${quick}"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    local missing_tools=()

    # Check for Go
    if ! command -v go &> /dev/null; then
        missing_tools+=("go")
    fi

    # Check for Node.js (required for npm-mcp tests)
    if ! command -v node &> /dev/null; then
        missing_tools+=("node")
    fi

    # Check for npm
    if ! command -v npm &> /dev/null; then
        missing_tools+=("npm")
    fi

    if [[ ${#missing_tools[@]} -gt 0 ]]; then
        log_error "Missing required tools: ${missing_tools[*]}"
        log_error "Please install the missing tools and try again"
        exit 1
    fi

    # Check Node.js version
    local node_version
    node_version=$(node --version | sed 's/v//')
    local required_major=22
    local current_major
    current_major=$(echo "${node_version}" | cut -d. -f1)

    if [[ ${current_major} -lt ${required_major} ]]; then
        log_warning "Node.js version ${node_version} detected. Recommended: ${required_major}.x.x or higher"
    fi

    log_verbose "Go version: $(go version)"
    log_verbose "Node.js version: $(node --version)"
    log_verbose "npm version: $(npm --version)"

    log_success "Prerequisites check passed"
}

# Setup test environment
setup_test_environment() {
    log_info "Setting up test environment..."

    # Ensure we're in the project root
    cd "${PROJECT_ROOT}"

    # Install npm dependencies if not skipped
    if [[ "${SKIP_NPM_INSTALL}" != "true" ]]; then
        log_info "Installing npm dependencies..."
        if [[ "${VERBOSE}" == "true" ]]; then
            npm install
        else
            npm install --silent
        fi
        log_success "npm dependencies installed"
    else
        log_verbose "Skipping npm install"
    fi

    # Build Go binary if not skipped
    if [[ "${SKIP_BUILD}" != "true" ]]; then
        log_info "Building Go binary..."
        if [[ "${VERBOSE}" == "true" ]]; then
            make local
        else
            make local > /dev/null 2>&1
        fi
        log_success "Binary built successfully"
    else
        log_verbose "Skipping binary build"
    fi

    # Ensure test directories exist
    mkdir -p "${E2E_DIR}"

    log_success "Test environment setup complete"
}

# Run JavaScript E2E tests
run_javascript_tests() {
    log_info "Running JavaScript E2E tests..."

    local js_test_file="${E2E_DIR}/npm_mcp_e2e_test.js"

    if [[ ! -f "${js_test_file}" ]]; then
        log_error "JavaScript test file not found: ${js_test_file}"
        return 1
    fi

    # Make sure the test file is executable
    chmod +x "${js_test_file}"

    # Set environment variables for the test
    export VERBOSE="${VERBOSE}"
    
    local test_env=()
    if [[ "${VERBOSE}" == "true" ]]; then
        test_env+=("VERBOSE=true")
    fi

    if [[ "${QUICK}" == "true" ]]; then
        test_env+=("QUICK=true")
    fi

    # Run the JavaScript tests
    log_verbose "Executing: node ${js_test_file}"
    
    if env "${test_env[@]}" node "${js_test_file}"; then
        log_success "JavaScript E2E tests passed"
        return 0
    else
        local exit_code=$?
        log_error "JavaScript E2E tests failed with exit code ${exit_code}"
        return 1
    fi
}

# Run Go integration tests
run_go_tests() {
    log_info "Running Go integration tests..."

    local test_args=("-v")
    
    if [[ "${QUICK}" == "true" ]]; then
        test_args+=("-short")
    fi

    if [[ "${VERBOSE}" == "true" ]]; then
        test_args+=("-test.v")
    fi

    # Run the specific NPM-MCP integration test
    local test_pattern="TestNPMMCPIntegration"
    
    log_verbose "Executing: go test ${test_args[*]} -run ${test_pattern} ${E2E_DIR}"
    
    if go test "${test_args[@]}" -run "${test_pattern}" "${E2E_DIR}"; then
        log_success "Go integration tests passed"
        return 0
    else
        local exit_code=$?
        log_error "Go integration tests failed with exit code ${exit_code}"
        return 1
    fi
}

# Run comprehensive tests
run_comprehensive_tests() {
    log_info "Running comprehensive NPM-MCP tests..."

    local js_result=0
    local go_result=0

    # Run JavaScript tests unless go-only
    if [[ "${GO_ONLY}" != "true" ]]; then
        if run_javascript_tests; then
            log_success "✅ JavaScript tests passed"
        else
            js_result=1
            log_error "❌ JavaScript tests failed"
        fi
    fi

    # Run Go tests unless js-only
    if [[ "${JS_ONLY}" != "true" ]]; then
        if run_go_tests; then
            log_success "✅ Go integration tests passed"
        else
            go_result=1
            log_error "❌ Go integration tests failed"
        fi
    fi

    # Summary
    echo ""
    log_info "Test Results Summary:"
    
    if [[ "${GO_ONLY}" != "true" ]]; then
        if [[ ${js_result} -eq 0 ]]; then
            echo -e "  JavaScript E2E Tests: ${GREEN}PASSED${NC}"
        else
            echo -e "  JavaScript E2E Tests: ${RED}FAILED${NC}"
        fi
    fi

    if [[ "${JS_ONLY}" != "true" ]]; then
        if [[ ${go_result} -eq 0 ]]; then
            echo -e "  Go Integration Tests: ${GREEN}PASSED${NC}"
        else
            echo -e "  Go Integration Tests: ${RED}FAILED${NC}"
        fi
    fi

    echo ""

    # Return appropriate exit code
    if [[ ${js_result} -eq 0 && ${go_result} -eq 0 ]]; then
        log_success "All NPM-MCP tests passed! 🎉"
        return 0
    else
        log_error "Some NPM-MCP tests failed. Please check the output above."
        return 1
    fi
}

# Run benchmark tests
run_benchmarks() {
    log_info "Running NPM-MCP benchmarks..."

    if [[ "${JS_ONLY}" != "true" ]]; then
        log_info "Running Go benchmarks..."
        if go test -bench=BenchmarkNPM -benchmem "${E2E_DIR}"; then
            log_success "Go benchmarks completed"
        else
            log_warning "Go benchmarks failed or incomplete"
        fi
    fi

    log_success "Benchmarks completed"
}

# Generate test report
generate_test_report() {
    log_info "Generating test report..."

    local report_file="${PROJECT_ROOT}/npm-mcp-test-report.md"
    local timestamp
    timestamp=$(date '+%Y-%m-%d %H:%M:%S')

    cat > "${report_file}" << EOF
# NPM-MCP Test Report

**Generated:** ${timestamp}
**Test Suite:** NPM-MCP E2E and Integration Tests

## Test Environment

- **Go Version:** $(go version)
- **Node.js Version:** $(node --version)
- **npm Version:** $(npm --version)
- **Platform:** $(uname -s -m)
- **Project Root:** ${PROJECT_ROOT}

## Test Categories Executed

EOF

    if [[ "${GO_ONLY}" != "true" ]]; then
        echo "- ✅ JavaScript E2E Tests" >> "${report_file}"
    fi

    if [[ "${JS_ONLY}" != "true" ]]; then
        echo "- ✅ Go Integration Tests" >> "${report_file}"
    fi

    if [[ "${QUICK}" == "true" ]]; then
        echo "- ⚡ Quick Mode (Long tests skipped)" >> "${report_file}"
    fi

    cat >> "${report_file}" << EOF

## Test Coverage

### JavaScript API Testing
- NPM package structure validation
- JavaScript wrapper functionality
- Binary integration through Node.js
- Error handling and recovery
- Cross-platform compatibility

### Go Integration Testing  
- NPM package installation flow
- Binary communication integration
- MCP server lifecycle via Node.js
- Performance benchmarking

## Notes

This test suite validates the integration between the NPM package wrapper
and the underlying Go binary, ensuring that the LSP Gateway can be properly
used as both a standalone binary and as a Node.js package with MCP support.

For detailed test output, run with --verbose flag.

EOF

    log_success "Test report generated: ${report_file}"
}

# Cleanup function
cleanup() {
    log_verbose "Performing cleanup..."
    
    # Kill any background processes if needed
    # (Currently not needed as tests are synchronous)
    
    log_verbose "Cleanup completed"
}

# Main execution
main() {
    local start_time
    start_time=$(date +%s)

    # Set up trap for cleanup
    trap cleanup EXIT

    # Parse arguments
    parse_args "$@"

    # Print header
    echo ""
    log_info "🚀 NPM-MCP Test Suite Runner"
    log_info "================================"
    echo ""

    # Check prerequisites
    check_prerequisites

    # Setup test environment  
    setup_test_environment

    # Run tests
    if run_comprehensive_tests; then
        log_success "🎉 All tests completed successfully!"
        
        # Run benchmarks if not in quick mode
        if [[ "${QUICK}" != "true" ]]; then
            run_benchmarks
        fi
        
        # Generate test report
        generate_test_report
        
        local end_time
        end_time=$(date +%s)
        local duration=$((end_time - start_time))
        
        log_info "Total test duration: ${duration} seconds"
        
        exit 0
    else
        log_error "❌ Test suite failed!"
        
        local end_time
        end_time=$(date +%s)
        local duration=$((end_time - start_time))
        
        log_info "Test duration: ${duration} seconds"
        
        exit 1
    fi
}

# Execute main function with all arguments
main "$@"