#!/bin/bash

# Simple LSP Test Runner
# Simplified test execution script for LSP server testing

set -euo pipefail

# Configuration
CONFIG_FILE="${CONFIG_FILE:-./simple-test-config.yaml}"
OUTPUT_DIR="${OUTPUT_DIR:-./test-results}"
MODE="${MODE:-full}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

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

# Show usage
show_usage() {
    cat << EOF
Simple LSP Test Runner

Usage: $0 [OPTIONS]

Options:
  -c, --config FILE    Configuration file (default: ./simple-test-config.yaml)
  -o, --output DIR     Output directory (default: ./test-results)
  -m, --mode MODE      Test mode: quick|full (default: full)
  -v, --verbose        Enable verbose output
  -h, --help           Show this help

Test Modes:
  quick    Run basic tests only
  full     Run all configured tests

Examples:
  $0                           # Run full tests with defaults
  $0 --mode quick              # Run quick tests
  $0 --config custom.yaml      # Use custom configuration
  $0 --output ./results        # Custom output directory
EOF
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -c|--config)
                CONFIG_FILE="$2"
                shift 2
                ;;
            -o|--output)
                OUTPUT_DIR="$2"
                shift 2
                ;;
            -m|--mode)
                MODE="$2"
                shift 2
                ;;
            -v|--verbose)
                set -x
                shift
                ;;
            -h|--help)
                show_usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done
}

# Check dependencies
check_dependencies() {
    log_info "Checking dependencies..."
    
    local missing=()
    
    # Check required tools
    for tool in go make; do
        if ! command -v "$tool" &> /dev/null; then
            missing+=("$tool")
        fi
    done
    
    if [[ ${#missing[@]} -gt 0 ]]; then
        log_error "Missing required dependencies: ${missing[*]}"
        return 1
    fi
    
    log_success "Dependencies check passed"
}

# Setup output directory
setup_output() {
    log_info "Setting up output directory: $OUTPUT_DIR"
    
    mkdir -p "$OUTPUT_DIR"
    
    # Clean previous results
    rm -f "$OUTPUT_DIR"/*.json "$OUTPUT_DIR"/*.log
    
    log_success "Output directory ready"
}

# Build test runner
build_runner() {
    log_info "Building simple test runner..."
    
    if [[ ! -f "./cmd/simple-lsp-test/main.go" ]]; then
        log_error "Simple test runner source not found"
        return 1
    fi
    
    mkdir -p ./bin
    
    if go build -o ./bin/simple-lsp-test ./cmd/simple-lsp-test/; then
        log_success "Test runner built successfully"
    else
        log_error "Failed to build test runner"
        return 1
    fi
}

# Run tests
run_tests() {
    log_info "Running LSP tests (mode: $MODE)..."
    
    local runner="./bin/simple-lsp-test"
    
    if [[ ! -f "$runner" ]]; then
        log_error "Test runner not found: $runner"
        return 1
    fi
    
    if [[ ! -f "$CONFIG_FILE" ]]; then
        log_error "Configuration file not found: $CONFIG_FILE"
        return 1
    fi
    
    # Run the test runner
    local start_time=$(date +%s)
    local exit_code=0
    
    "$runner" \
        --config "$CONFIG_FILE" \
        --output "$OUTPUT_DIR" \
        --mode "$MODE" || exit_code=$?
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    if [[ $exit_code -eq 0 ]]; then
        log_success "Tests completed successfully in ${duration}s"
    else
        log_error "Tests failed with exit code: $exit_code"
    fi
    
    return $exit_code
}

# Show results summary
show_results() {
    log_info "Test Results:"
    
    local results_file="$OUTPUT_DIR/test-results.json"
    
    if [[ -f "$results_file" ]]; then
        if command -v python3 &> /dev/null; then
            python3 -c "
import json
try:
    with open('$results_file') as f:
        data = json.load(f)
        print(f\"  Total Tests: {data.get('total', 0)}\")
        print(f\"  Passed: {data.get('passed', 0)}\")
        print(f\"  Failed: {data.get('failed', 0)}\")
        if data.get('total', 0) > 0:
            pass_rate = (data.get('passed', 0) / data.get('total', 1)) * 100
            print(f\"  Pass Rate: {pass_rate:.1f}%\")
except Exception as e:
    print(f\"  Could not parse results: {e}\")
"
        else
            log_info "  Results file: $results_file"
        fi
    else
        log_warning "No results file found"
    fi
    
    log_info "Output directory: $OUTPUT_DIR"
}

# Main execution
main() {
    echo "Simple LSP Test Runner"
    echo "====================="
    echo
    
    parse_args "$@"
    
    check_dependencies || exit 1
    setup_output || exit 1
    build_runner || exit 1
    run_tests || exit 1
    show_results
    
    log_success "Test execution completed!"
}

# Execute main function
main "$@"