#!/bin/bash
# Deadcode Configuration Script for LSP Gateway
# Provides different deadcode analysis configurations to reduce false positives

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

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

# Check if deadcode is installed
check_deadcode() {
    if ! command -v deadcode >/dev/null 2>&1; then
        log_error "deadcode not found. Install with: go install golang.org/x/tools/cmd/deadcode@latest"
        exit 1
    fi
}

# Exclusion patterns for different analysis types
get_production_filter() {
    echo "github.com/.*|cmd/.*|internal/.*"
}

get_main_filter() {
    echo "cmd/.*"
}

get_internal_filter() {
    echo "internal/.*"
}

# Build tags to exclude test-specific or platform-specific code
get_common_build_tags() {
    echo "!test,!integration,!e2e"
}

# Run deadcode with production-focused settings (reduces false positives)
run_production_analysis() {
    log_info "Running production code analysis (excludes tests to reduce false positives)..."
    
    local filter
    filter=$(get_production_filter)
    
    deadcode -filter="$filter" ./... 2>&1 | grep -E "(unreachable func|error|warning)" || {
        log_success "No dead code found in production packages"
        return 0
    }
}

# Run deadcode with strict settings (includes tests, may have more false positives)
run_strict_analysis() {
    log_info "Running strict analysis (includes tests - may have false positives)..."
    
    local filter
    filter=$(get_production_filter)
    
    deadcode -test -filter="$filter" ./... 2>&1 | grep -E "(unreachable func|error|warning)" || {
        log_success "No dead code found with strict analysis"
        return 0
    }
}

# Run deadcode on main packages only
run_main_analysis() {
    log_info "Running main packages analysis..."
    
    local filter
    filter=$(get_main_filter)
    
    deadcode -filter="$filter" ./cmd/... 2>&1 | grep -E "(unreachable func|error|warning)" || {
        log_success "No dead code found in main packages"
        return 0
    }
}

# Run deadcode on internal packages only
run_internal_analysis() {
    log_info "Running internal packages analysis..."
    
    local filter
    filter=$(get_internal_filter)
    
    deadcode -filter="$filter" ./internal/... 2>&1 | grep -E "(unreachable func|error|warning)" || {
        log_success "No dead code found in internal packages"
        return 0
    }
}

# Run deadcode with JSON output for parsing
run_json_analysis() {
    log_info "Running deadcode analysis with JSON output..."
    
    local filter
    filter=$(get_production_filter)
    
    deadcode -json -filter="$filter" ./... > /tmp/deadcode-analysis.json
    
    if [[ -s /tmp/deadcode-analysis.json ]] && [[ "$(cat /tmp/deadcode-analysis.json)" != "null" ]] && [[ "$(cat /tmp/deadcode-analysis.json)" != "[]" ]]; then
        log_warning "Dead code found. Results saved to /tmp/deadcode-analysis.json"
        cat /tmp/deadcode-analysis.json | jq '.[] | .Path + ": " + (.Funcs | map(.Name) | join(", "))'
        return 1
    else
        log_success "No dead code found"
        rm -f /tmp/deadcode-analysis.json
        return 0
    fi
}

# Explain why a function is not dead
explain_function() {
    local func_name="$1"
    
    if [[ -z "$func_name" ]]; then
        log_error "Please provide a function name"
        return 1
    fi
    
    log_info "Analyzing why function '$func_name' is not dead..."
    deadcode -whylive="$func_name" ./...
}

# Show help
show_help() {
    cat << 'EOF'
Deadcode Configuration Script for LSP Gateway

Usage: ./scripts/deadcode-config.sh [COMMAND] [OPTIONS]

COMMANDS:
    production    Run production-focused analysis (default, fewer false positives)
    strict        Run strict analysis including tests (may have false positives)
    main          Run analysis on main packages only
    internal      Run analysis on internal packages only
    json          Run analysis with JSON output
    explain FUNC  Explain why a function is not considered dead code
    help          Show this help message

EXAMPLES:
    ./scripts/deadcode-config.sh production
    ./scripts/deadcode-config.sh strict
    ./scripts/deadcode-config.sh explain "main.validateConfig"
    ./scripts/deadcode-config.sh json

REDUCING FALSE POSITIVES:
    - Use 'production' mode to exclude test code
    - Use 'main' or 'internal' for focused analysis
    - Functions called via reflection may appear as false positives
    - Build constraints and platform-specific code may cause false positives
    - Consider using 'explain' to understand why code is kept

EOF
}

# Main execution
main() {
    check_deadcode
    
    case "${1:-production}" in
        production)
            run_production_analysis
            ;;
        strict)
            run_strict_analysis
            ;;
        main)
            run_main_analysis
            ;;
        internal)
            run_internal_analysis
            ;;
        json)
            run_json_analysis
            ;;
        explain)
            if [[ $# -lt 2 ]]; then
                log_error "Please provide a function name to explain"
                show_help
                exit 1
            fi
            explain_function "$2"
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            log_error "Unknown command: $1"
            show_help
            exit 1
            ;;
    esac
}

# Run main function with all arguments
main "$@"