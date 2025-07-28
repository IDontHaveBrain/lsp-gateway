#!/bin/bash

# test-python-patterns.sh
# Comprehensive test runner for Python patterns e2e functionality
# Tests Python LSP features using PythonRepoManager and real server integration

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
TESTS_DIR="${PROJECT_ROOT}/tests"
E2E_DIR="${TESTS_DIR}/e2e"
VERBOSE=${VERBOSE:-false}
SKIP_BUILD=${SKIP_BUILD:-false}
SKIP_REPO_CLONE=${SKIP_REPO_CLONE:-false}
TEST_TIMEOUT=${TEST_TIMEOUT:-900}

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
Python Patterns E2E Test Runner

Usage: $0 [OPTIONS]

Options:
    -h, --help              Show this help message
    -v, --verbose           Enable verbose output
    -s, --skip-build        Skip binary build step
    -r, --skip-repo-clone   Skip repository cloning (use existing if available)
    --quick                 Run quick tests only (skip long-running tests)
    --comprehensive         Run comprehensive Python test suite (includes all Python tests)
    --timeout SECONDS       Test timeout in seconds (default: 900)

Environment Variables:
    VERBOSE                 Set to 'true' for verbose output
    SKIP_BUILD             Set to 'true' to skip binary build
    SKIP_REPO_CLONE        Set to 'true' to skip repository cloning
    TEST_TIMEOUT           Test timeout in seconds

Examples:
    $0                      Run Python patterns tests
    $0 --verbose            Run with verbose output
    $0 --quick             Run quick patterns tests only
    $0 --comprehensive      Run complete Python test suite
    $0 --skip-repo-clone   Use existing repository if available

EOF
}

# Parse command line arguments
parse_args() {
    local quick=false
    local comprehensive=false

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
            -r|--skip-repo-clone)
                SKIP_REPO_CLONE=true
                shift
                ;;
            --quick)
                quick=true
                shift
                ;;
            --comprehensive)
                comprehensive=true
                shift
                ;;
            --timeout)
                TEST_TIMEOUT="$2"
                shift 2
                ;;
            *)
                log_error "Unknown option: $1"
                print_help
                exit 1
                ;;
        esac
    done

    # Export parsed options for use in functions
    export QUICK="${quick}"
    export COMPREHENSIVE="${comprehensive}"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    local missing_tools=()

    # Check for Go
    if ! command -v go &> /dev/null; then
        missing_tools+=("go")
    fi

    # Check for Python (required for pylsp)
    if ! command -v python3 &> /dev/null && ! command -v python &> /dev/null; then
        missing_tools+=("python3 or python")
    fi

    # Check for Git (required for repository cloning)
    if ! command -v git &> /dev/null; then
        missing_tools+=("git")
    fi

    if [[ ${#missing_tools[@]} -gt 0 ]]; then
        log_error "Missing required tools: ${missing_tools[*]}"
        log_error "Please install the missing tools and try again"
        exit 1
    fi

    # Determine Python command
    local python_cmd
    if command -v python3 &> /dev/null; then
        python_cmd="python3"
    else
        python_cmd="python"
    fi

    # Check Python version (3.11+ recommended)
    local python_version
    python_version=$($python_cmd --version 2>&1 | grep -oE '[0-9]+\.[0-9]+' | head -1)
    local required_major=3
    local required_minor=8
    local current_major
    local current_minor
    current_major=$(echo "${python_version}" | cut -d. -f1)
    current_minor=$(echo "${python_version}" | cut -d. -f2)

    if [[ ${current_major} -lt ${required_major} ]] || [[ ${current_major} -eq ${required_major} && ${current_minor} -lt ${required_minor} ]]; then
        log_warning "Python version ${python_version} detected. Recommended: ${required_major}.${required_minor}+ or higher"
    fi

    # Check for pylsp (Python LSP server)
    if ! command -v pylsp &> /dev/null; then
        log_warning "pylsp (Python LSP server) not found in PATH"
        log_info "Attempting to find pylsp in common locations..."
        
        # Check common pylsp installation paths
        local pylsp_paths=(
            "$HOME/.local/bin/pylsp"
            "$($python_cmd -m site --user-base)/bin/pylsp"
            "/usr/local/bin/pylsp"
            "/opt/homebrew/bin/pylsp"
        )
        
        local pylsp_found=false
        for pylsp_path in "${pylsp_paths[@]}"; do
            if [[ -x "$pylsp_path" ]]; then
                log_info "Found pylsp at: $pylsp_path"
                export PATH="$(dirname "$pylsp_path"):$PATH"
                pylsp_found=true
                break
            fi
        done
        
        if [[ "$pylsp_found" == "false" ]]; then
            log_error "pylsp not found. Please install python-lsp-server:"
            log_error "  pip install python-lsp-server[all]"
            log_error "  or: pip install --user python-lsp-server[all]"
            exit 1
        fi
    fi

    # Verify pylsp functionality
    if ! pylsp --help &> /dev/null; then
        log_error "pylsp found but not functional. Please reinstall python-lsp-server"
        exit 1
    fi

    log_verbose "Go version: $(go version)"
    log_verbose "Python version: $($python_cmd --version 2>&1)"
    log_verbose "Git version: $(git --version)"
    log_verbose "pylsp version: $(pylsp --version 2>/dev/null || echo 'version not available')"

    log_success "Prerequisites check passed"
}

# Setup test environment
setup_test_environment() {
    log_info "Setting up test environment..."

    # Ensure we're in the project root
    cd "${PROJECT_ROOT}"

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

    # Set up Python environment variables for testing
    export PYTHONPATH="${PROJECT_ROOT}:${PYTHONPATH:-}"
    export GO_TEST_TIMEOUT="${TEST_TIMEOUT}s"

    # Create temporary directory for test isolation
    local temp_base="/tmp/lspg-python-e2e-tests"
    mkdir -p "$temp_base"
    log_verbose "Test workspace base directory: $temp_base"

    # Set environment variables for Go tests to use
    export PYTHON_TEST_SKIP_REPO_CLONE="${SKIP_REPO_CLONE}"
    export PYTHON_TEST_VERBOSE="${VERBOSE}"

    # Validate that lspg binary exists and is functional
    if [[ -x "${PROJECT_ROOT}/bin/lspg" ]]; then
        log_verbose "lspg binary found at: ${PROJECT_ROOT}/bin/lspg"
        
        # Quick validation that binary works
        if "${PROJECT_ROOT}/bin/lspg" --version &> /dev/null; then
            log_verbose "lspg binary is functional"
        else
            log_warning "lspg binary exists but may not be functional"
        fi
    else
        log_error "lspg binary not found. Please run 'make local' first"
        exit 1
    fi

    log_success "Test environment setup complete"
}

# Run Python patterns tests
run_python_patterns_tests() {
    log_info "Running Python patterns e2e tests..."

    local test_args=("-v" "-timeout" "${TEST_TIMEOUT}s")
    
    if [[ "${QUICK}" == "true" ]]; then
        test_args+=("-short")
        log_info "Running in quick mode (short tests only)"
    fi

    if [[ "${VERBOSE}" == "true" ]]; then
        test_args+=("-test.v")
    fi

    # Run the specific Python patterns test that uses PythonRepoManager
    local test_pattern="TestPythonLSPFeaturesComprehensiveTestSuite"
    
    log_verbose "Executing: go test ${test_args[*]} -run ${test_pattern} ${E2E_DIR}"
    log_info "Test timeout: ${TEST_TIMEOUT} seconds"
    
    # Set additional environment variables for the test
    export PYTHON_PATTERNS_TEST_MODE="patterns_only"
    
    local start_time
    start_time=$(date +%s)
    
    if go test "${test_args[@]}" -run "${test_pattern}" "${E2E_DIR}"; then
        local end_time
        end_time=$(date +%s)
        local duration=$((end_time - start_time))
        log_success "Python patterns e2e tests passed (duration: ${duration}s)"
        return 0
    else
        local exit_code=$?
        local end_time
        end_time=$(date +%s)
        local duration=$((end_time - start_time))
        log_error "Python patterns e2e tests failed with exit code ${exit_code} (duration: ${duration}s)"
        return 1
    fi
}

# Run comprehensive Python test suite
run_comprehensive_python_tests() {
    log_info "Running comprehensive Python test suite..."

    local test_args=("-v" "-timeout" "${TEST_TIMEOUT}s")
    
    if [[ "${QUICK}" == "true" ]]; then
        test_args+=("-short")
        log_info "Running in quick mode (short tests only)"
    fi

    if [[ "${VERBOSE}" == "true" ]]; then
        test_args+=("-test.v")
    fi

    # Run all Python-related tests
    local test_patterns=(
        "TestPythonBasicWorkflow.*"
        "TestPythonAdvanced.*"
        "TestPythonLSP.*"
        "TestPythonReal.*"
    )
    
    local overall_result=0
    local test_count=0
    local passed_count=0
    
    export PYTHON_PATTERNS_TEST_MODE="comprehensive"
    
    for pattern in "${test_patterns[@]}"; do
        log_info "Running test pattern: $pattern"
        log_verbose "Executing: go test ${test_args[*]} -run ${pattern} ${E2E_DIR}"
        
        local start_time
        start_time=$(date +%s)
        
        if go test "${test_args[@]}" -run "${pattern}" "${E2E_DIR}"; then
            local end_time
            end_time=$(date +%s)
            local duration=$((end_time - start_time))
            log_success "✅ $pattern passed (duration: ${duration}s)"
            ((passed_count++))
        else
            local exit_code=$?
            local end_time
            end_time=$(date +%s)
            local duration=$((end_time - start_time))
            log_error "❌ $pattern failed with exit code ${exit_code} (duration: ${duration}s)"
            overall_result=1
        fi
        ((test_count++))
    done
    
    log_info "Comprehensive test results: ${passed_count}/${test_count} test patterns passed"
    
    if [[ $overall_result -eq 0 ]]; then
        log_success "Comprehensive Python test suite passed"
        return 0
    else
        log_error "Comprehensive Python test suite failed"
        return 1
    fi
}

# Run tests based on mode
run_tests() {
    log_info "Test execution summary:"
    log_info "  Mode: $(if [[ "${COMPREHENSIVE}" == "true" ]]; then echo "Comprehensive"; else echo "Python Patterns"; fi)"
    log_info "  Quick: $(if [[ "${QUICK}" == "true" ]]; then echo "Yes"; else echo "No"; fi)"
    log_info "  Timeout: ${TEST_TIMEOUT}s"
    log_info "  Skip Repo Clone: $(if [[ "${SKIP_REPO_CLONE}" == "true" ]]; then echo "Yes"; else echo "No"; fi)"
    echo ""

    local start_time
    start_time=$(date +%s)
    
    if [[ "${COMPREHENSIVE}" == "true" ]]; then
        if run_comprehensive_python_tests; then
            local end_time
            end_time=$(date +%s)
            local duration=$((end_time - start_time))
            log_success "✅ Comprehensive Python tests passed (total duration: ${duration}s)"
            return 0
        else
            local end_time
            end_time=$(date +%s)
            local duration=$((end_time - start_time))
            log_error "❌ Comprehensive Python tests failed (total duration: ${duration}s)"
            return 1
        fi
    else
        if run_python_patterns_tests; then
            local end_time
            end_time=$(date +%s)
            local duration=$((end_time - start_time))
            log_success "✅ Python patterns tests passed (total duration: ${duration}s)"
            return 0
        else
            local end_time
            end_time=$(date +%s)
            local duration=$((end_time - start_time))
            log_error "❌ Python patterns tests failed (total duration: ${duration}s)"
            return 1
        fi
    fi
}

# Run benchmarks (optional performance testing)
run_benchmarks() {
    log_info "Running Python patterns benchmarks..."

    local benchmark_args=("-bench=BenchmarkPython" "-benchmem" "-timeout" "${TEST_TIMEOUT}s")
    
    if [[ "${VERBOSE}" == "true" ]]; then
        benchmark_args+=("-v")
    fi

    log_verbose "Executing: go test ${benchmark_args[*]} ${E2E_DIR}"
    
    if go test "${benchmark_args[@]}" "${E2E_DIR}" 2>/dev/null; then
        log_success "Python benchmarks completed"
        return 0
    else
        log_warning "Python benchmarks failed or not available"
        return 1
    fi
}

# Generate test report
generate_test_report() {
    log_info "Generating test report..."

    local report_file="${PROJECT_ROOT}/python-patterns-test-report.md"
    local timestamp
    timestamp=$(date '+%Y-%m-%d %H:%M:%S')

    local python_cmd
    if command -v python3 &> /dev/null; then
        python_cmd="python3"
    else
        python_cmd="python"
    fi

    # Collect system information
    local pylsp_version
    pylsp_version=$(pylsp --version 2>/dev/null || echo "version not available")
    
    local git_version
    git_version=$(git --version)
    
    local platform_info
    platform_info="$(uname -s -m)"

    cat > "${report_file}" << EOF
# Python Patterns E2E Test Report

**Generated:** ${timestamp}
**Test Suite:** Python LSP Features and Patterns Comprehensive Tests
**Test Mode:** $(if [[ "${COMPREHENSIVE}" == "true" ]]; then echo "Comprehensive (All Python Tests)"; else echo "Python Patterns Only"; fi)

## Test Environment

- **Go Version:** $(go version)
- **Python Version:** $($python_cmd --version 2>&1)
- **Python LSP Server:** pylsp ${pylsp_version}
- **Git Version:** ${git_version}
- **Platform:** ${platform_info}
- **Project Root:** ${PROJECT_ROOT}
- **Test Timeout:** ${TEST_TIMEOUT} seconds
- **Quick Mode:** $(if [[ "${QUICK}" == "true" ]]; then echo "Enabled"; else echo "Disabled"; fi)
- **Repository Cloning:** $(if [[ "${SKIP_REPO_CLONE}" == "true" ]]; then echo "Skipped"; else echo "Enabled"; fi)

## Test Categories Executed

EOF

    if [[ "${COMPREHENSIVE}" == "true" ]]; then
        cat >> "${report_file}" << EOF
- ✅ **Comprehensive Python Test Suite**
  - Basic Python workflow tests
  - Advanced Python language features
  - Real pylsp integration tests
  - Pattern-specific LSP functionality
EOF
    else
        cat >> "${report_file}" << EOF
- ✅ **Python Patterns E2E Tests**
  - Python design patterns repository integration
  - Real-world Python codebase testing
  - LSP feature validation with actual Python files
EOF
    fi

    if [[ "${QUICK}" == "true" ]]; then
        echo "- ⚡ **Quick Mode** (Long-running tests skipped)" >> "${report_file}"
    fi

    cat >> "${report_file}" << EOF

## Test Infrastructure

### Repository Management
- **Repository:** https://github.com/faif/python-patterns.git
- **Management System:** PythonRepoManager with error recovery
- **Test Isolation:** Unique temporary workspaces for each test run
- **Cleanup:** Automatic workspace cleanup with emergency recovery

### LSP Integration Testing
- **LSP Server:** python-lsp-server (pylsp)
- **Communication:** HTTP JSON-RPC gateway
- **Features Tested:**
  - textDocument/definition (Go to definition)
  - textDocument/references (Find references)
  - textDocument/hover (Hover information)
  - textDocument/documentSymbol (Document symbols)
  - workspace/symbol (Workspace symbol search)
  - textDocument/completion (Code completion)

### Test Scenarios
- **Creational Patterns:** Factory Method, Builder, Singleton, Abstract Factory
- **Structural Patterns:** Adapter, Decorator, Facade, Proxy
- **Behavioral Patterns:** Observer, Strategy, Command, State
- **Each Pattern Tests:**
  - Symbol definition lookup
  - Reference finding across files
  - Hover information display
  - Code completion in context
  - Document and workspace symbol search

## Performance and Reliability

### Error Handling
- Network connectivity validation
- Repository health checks
- LSP server availability verification
- Comprehensive error recovery mechanisms
- Emergency cleanup procedures

### Test Isolation
- Unique workspace directories per test run
- Proper cleanup after test completion
- No interference between test executions
- Temporary file management

## Usage Examples

\`\`\`bash
# Run standard Python patterns tests
$0

# Run with detailed output
$0 --verbose

# Quick test execution (skip long-running tests)
$0 --quick

# Comprehensive Python test suite
$0 --comprehensive

# Use existing repository if available
$0 --skip-repo-clone

# Custom timeout (15 minutes)
$0 --timeout 900
\`\`\`

## Troubleshooting

### Common Issues
1. **pylsp not found:** Install with \`pip install python-lsp-server[all]\`
2. **Git clone failures:** Check network connectivity and repository access
3. **Test timeouts:** Increase timeout with \`--timeout\` flag
4. **LSP server issues:** Verify pylsp installation and functionality

### Environment Setup
- Ensure Python 3.8+ is installed
- Install python-lsp-server: \`pip install python-lsp-server[all]\`
- Verify Git is available and functional
- Build LSP Gateway binary: \`make local\`

## Notes

This test suite validates the Python LSP integration using real Python design pattern
implementations from the community-maintained python-patterns repository. It ensures
that all LSP features work correctly with actual Python codebases, providing confidence
in the LSP Gateway's Python language support.

The test infrastructure uses the PythonRepoManager system for reliable repository
management, comprehensive error handling, and proper test isolation.

For detailed test output and debugging information, run with the \`--verbose\` flag.

EOF

    log_success "Test report generated: ${report_file}"
}

# Cleanup function
cleanup() {
    log_verbose "Performing cleanup..."
    
    # Kill any background processes if needed
    local pids=$(jobs -p)
    if [[ -n "$pids" ]]; then
        log_verbose "Killing background processes: $pids"
        kill $pids 2>/dev/null || true
    fi
    
    # Clean up any temporary files or processes related to testing
    # Note: PythonRepoManager handles its own cleanup automatically
    
    # Clean up any lingering lspg processes (in case of abnormal termination)
    if command -v pkill &> /dev/null; then
        pkill -f "lspg.*server" 2>/dev/null || true
        log_verbose "Cleaned up any lingering lspg server processes"
    fi
    
    log_verbose "Cleanup completed"
}

# Validate test results and provide detailed feedback
validate_test_results() {
    local exit_code=$1
    local duration=$2
    
    if [[ $exit_code -eq 0 ]]; then
        log_success "🎉 All tests completed successfully!"
        log_info "Test execution summary:"
        log_info "  Total duration: ${duration} seconds"
        log_info "  Mode: $(if [[ "${COMPREHENSIVE}" == "true" ]]; then echo "Comprehensive"; else echo "Python Patterns"; fi)"
        log_info "  Quick mode: $(if [[ "${QUICK}" == "true" ]]; then echo "Enabled"; else echo "Disabled"; fi)"
        log_info "  Repository cloning: $(if [[ "${SKIP_REPO_CLONE}" == "true" ]]; then echo "Skipped"; else echo "Enabled"; fi)"
        return 0
    else
        log_error "❌ Test suite failed!"
        log_error "Troubleshooting suggestions:"
        log_error "  1. Check that pylsp is properly installed: pip install python-lsp-server[all]"
        log_error "  2. Verify network connectivity for repository cloning"
        log_error "  3. Ensure lspg binary is built: make local"
        log_error "  4. Try running with --verbose for detailed output"
        log_error "  5. Check available disk space in /tmp for test workspaces"
        
        if [[ "${QUICK}" != "true" ]]; then
            log_error "  6. Try running with --quick to skip long-running tests"
        fi
        
        if [[ "${SKIP_REPO_CLONE}" != "true" ]]; then
            log_error "  7. Try running with --skip-repo-clone if repository cloning fails"
        fi
        
        return 1
    fi
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
    log_info "🐍 Python Patterns E2E Test Suite Runner"
    log_info "=========================================="
    echo ""

    # Display configuration
    log_info "Configuration:"
    log_info "  Script: $(basename "$0")"
    log_info "  Project Root: ${PROJECT_ROOT}"
    log_info "  Test Directory: ${E2E_DIR}"
    log_info "  Verbose: ${VERBOSE}"
    log_info "  Skip Build: ${SKIP_BUILD}"
    log_info "  Skip Repo Clone: ${SKIP_REPO_CLONE}"
    log_info "  Timeout: ${TEST_TIMEOUT}s"
    echo ""

    # Check prerequisites
    check_prerequisites

    # Setup test environment  
    setup_test_environment

    # Run tests
    local test_exit_code=0
    if ! run_tests; then
        test_exit_code=1
    fi
    
    local end_time
    end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    # Validate results and provide feedback
    if validate_test_results $test_exit_code $duration; then
        # Generate test report on success
        generate_test_report
        
        # Run benchmarks if not in quick mode and tests passed
        if [[ "${QUICK}" != "true" ]] && [[ $test_exit_code -eq 0 ]]; then
            echo ""
            log_info "Running optional benchmarks..."
            run_benchmarks
        fi
        
        echo ""
        log_info "📊 Test report generated: ${PROJECT_ROOT}/python-patterns-test-report.md"
        log_info "✅ Python patterns E2E testing completed successfully!"
        
        exit 0
    else
        echo ""
        log_info "Test duration: ${duration} seconds"
        log_error "❌ Python patterns E2E testing failed!"
        
        exit 1
    fi
}

# Execute main function with all arguments
main "$@"