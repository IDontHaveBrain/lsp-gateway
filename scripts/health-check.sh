#!/bin/bash

# LSP Gateway Health Check Monitor
# Automated health monitoring for the LSP validation test environment

set -euo pipefail

# Script metadata
readonly SCRIPT_NAME="$(basename "$0")"
readonly SCRIPT_VERSION="1.0.0"
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Configuration
readonly HEALTH_CHECK_LOG="$PROJECT_ROOT/health-check.log"
readonly ALERT_LOG="$PROJECT_ROOT/health-alerts.log"
readonly MIN_DISK_SPACE_GB=5
readonly MAX_MEMORY_USAGE_PERCENT=80

# Colors for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly NC='\033[0m'

# Global variables
QUIET=false
VERBOSE=false
ALERT_ON_FAILURE=false
LOG_RESULTS=true

# Logging functions
log() {
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    local message="$*"
    
    if [[ "$LOG_RESULTS" == true ]]; then
        echo "[$timestamp] $message" >> "$HEALTH_CHECK_LOG"
    fi
    
    if [[ "$QUIET" != true ]]; then
        echo -e "${BLUE}[$timestamp]${NC} $message"
    fi
}

log_success() {
    local message="$*"
    log "✓ SUCCESS: $message"
    if [[ "$QUIET" != true ]]; then
        echo -e "${GREEN}✓ $message${NC}"
    fi
}

log_warning() {
    local message="$*"
    log "⚠ WARNING: $message"
    if [[ "$QUIET" != true ]]; then
        echo -e "${YELLOW}⚠ $message${NC}"
    fi
}

log_error() {
    local message="$*"
    log "✗ ERROR: $message"
    if [[ "$LOG_RESULTS" == true ]]; then
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] HEALTH CHECK FAILURE: $message" >> "$ALERT_LOG"
    fi
    if [[ "$QUIET" != true ]]; then
        echo -e "${RED}✗ $message${NC}"
    fi
}

# Health check functions
check_disk_space() {
    log "Checking disk space..."
    
    local available_kb
    available_kb=$(df "$PROJECT_ROOT" | awk 'NR==2 {print $4}')
    local available_gb=$((available_kb / 1024 / 1024))
    
    if [[ $available_gb -lt $MIN_DISK_SPACE_GB ]]; then
        log_error "Insufficient disk space: ${available_gb}GB available, ${MIN_DISK_SPACE_GB}GB required"
        return 1
    fi
    
    log_success "Disk space check passed: ${available_gb}GB available"
    return 0
}

check_memory_usage() {
    log "Checking memory usage..."
    
    if ! command -v free >/dev/null 2>&1; then
        log_warning "Memory check skipped: 'free' command not available"
        return 0
    fi
    
    local memory_usage
    memory_usage=$(free | awk 'NR==2{printf "%.0f", $3*100/$2}')
    
    if [[ $memory_usage -gt $MAX_MEMORY_USAGE_PERCENT ]]; then
        log_warning "High memory usage: ${memory_usage}%"
    else
        log_success "Memory usage check passed: ${memory_usage}%"
    fi
    
    return 0
}

check_go_environment() {
    log "Checking Go environment..."
    
    if ! command -v go >/dev/null 2>&1; then
        log_error "Go not found in PATH"
        return 1
    fi
    
    local go_version
    go_version=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | sed 's/go//')
    local major=$(echo "$go_version" | cut -d. -f1)
    local minor=$(echo "$go_version" | cut -d. -f2)
    
    if [[ $major -lt 1 || ($major -eq 1 && $minor -lt 24) ]]; then
        log_error "Go version $go_version is too old, need 1.24+"
        return 1
    fi
    
    log_success "Go environment check passed: version $go_version"
    return 0
}

check_lsp_servers() {
    log "Checking LSP servers..."
    
    local lsp_check_passed=true
    
    # Check gopls
    if command -v gopls >/dev/null 2>&1; then
        local gopls_version
        gopls_version=$(gopls version 2>/dev/null | head -1 || echo "unknown")
        log_success "gopls available: $gopls_version"
    else
        log_error "gopls not found"
        lsp_check_passed=false
    fi
    
    # Check pyright
    if command -v pyright-langserver >/dev/null 2>&1; then
        local pyright_version
        pyright_version=$(pyright --version 2>/dev/null || echo "unknown")
        log_success "pyright available: $pyright_version"
    else
        log_warning "pyright not found (optional)"
    fi
    
    # Check typescript-language-server
    if command -v typescript-language-server >/dev/null 2>&1; then
        local tsls_version
        tsls_version=$(typescript-language-server --version 2>/dev/null || echo "unknown")
        log_success "typescript-language-server available: $tsls_version"
    else
        log_warning "typescript-language-server not found (optional)"
    fi
    
    if [[ "$lsp_check_passed" != true ]]; then
        log_error "Critical LSP servers missing"
        return 1
    fi
    
    return 0
}

check_lsp_gateway_binary() {
    log "Checking LSP Gateway binary..."
    
    local binary_path="$PROJECT_ROOT/bin/lsp-gateway"
    
    if [[ ! -f "$binary_path" ]]; then
        log_error "LSP Gateway binary not found at: $binary_path"
        return 1
    fi
    
    if [[ ! -x "$binary_path" ]]; then
        log_error "LSP Gateway binary is not executable: $binary_path"
        return 1
    fi
    
    # Test basic functionality
    if ! "$binary_path" version >/dev/null 2>&1; then
        log_error "LSP Gateway binary is not functional"
        return 1
    fi
    
    local version
    version=$("$binary_path" version 2>/dev/null | head -1 || echo "unknown")
    log_success "LSP Gateway binary check passed: $version"
    return 0
}

check_repositories() {
    log "Checking repository status..."
    
    cd "$PROJECT_ROOT"
    
    # Check if repositories exist
    if [[ ! -d "test-repositories" ]]; then
        log_warning "Test repositories not cloned"
        return 0
    fi
    
    # Run repository status check
    if ! ./scripts/clone-repos.sh status >/dev/null 2>&1; then
        log_error "Repository status check failed"
        return 1
    fi
    
    log_success "Repository status check passed"
    return 0
}

run_basic_lsp_tests() {
    log "Running basic LSP validation tests..."
    
    cd "$PROJECT_ROOT"
    
    # Run short validation tests with timeout
    if timeout 300 ./scripts/run-lsp-validation-tests.sh short >/dev/null 2>&1; then
        log_success "Basic LSP validation tests passed"
        return 0
    else
        local exit_code=$?
        if [[ $exit_code -eq 124 ]]; then
            log_error "LSP validation tests timed out after 5 minutes"
        else
            log_error "LSP validation tests failed with exit code: $exit_code"
        fi
        return 1
    fi
}

check_configuration_files() {
    log "Checking configuration files..."
    
    local required_files=(
        "$PROJECT_ROOT/test-repositories.yaml"
        "$PROJECT_ROOT/config.yaml"
    )
    
    for file in "${required_files[@]}"; do
        if [[ ! -f "$file" ]]; then
            log_error "Required configuration file missing: $file"
            return 1
        fi
        
        # Basic YAML syntax check (if yq or python is available)
        if command -v python3 >/dev/null 2>&1; then
            if ! python3 -c "import yaml; yaml.safe_load(open('$file'))" 2>/dev/null; then
                log_error "Configuration file has syntax errors: $file"
                return 1
            fi
        fi
    done
    
    log_success "Configuration files check passed"
    return 0
}

check_network_connectivity() {
    log "Checking network connectivity..."
    
    # Test GitHub connectivity (where test repositories are hosted)
    if ! curl -s --connect-timeout 10 https://github.com >/dev/null; then
        log_error "Cannot connect to GitHub - repository cloning may fail"
        return 1
    fi
    
    log_success "Network connectivity check passed"
    return 0
}

# Main health check function
run_health_checks() {
    local start_time=$(date +%s)
    local check_count=0
    local passed_count=0
    local failed_count=0
    
    log "Starting LSP Gateway health check v$SCRIPT_VERSION"
    log "Working directory: $PROJECT_ROOT"
    
    # Array of health check functions
    local health_checks=(
        "check_disk_space"
        "check_memory_usage"
        "check_go_environment"
        "check_configuration_files"
        "check_lsp_gateway_binary"
        "check_lsp_servers"
        "check_network_connectivity"
        "check_repositories"
    )
    
    # Run optional LSP tests if not in quick mode
    if [[ "${QUICK_CHECK:-false}" != "true" ]]; then
        health_checks+=("run_basic_lsp_tests")
    fi
    
    # Execute each health check
    for check_func in "${health_checks[@]}"; do
        ((check_count++))
        if [[ "$VERBOSE" == true ]]; then
            log "Running check: $check_func"
        fi
        
        if "$check_func"; then
            ((passed_count++))
        else
            ((failed_count++))
        fi
    done
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    # Generate summary
    log "Health check completed in ${duration}s"
    log "Results: $passed_count passed, $failed_count failed, $check_count total"
    
    if [[ $failed_count -eq 0 ]]; then
        log_success "All health checks passed!"
        return 0
    else
        log_error "Health check failed: $failed_count checks failed"
        
        # Send alert if configured
        if [[ "$ALERT_ON_FAILURE" == true ]]; then
            send_alert "LSP Gateway health check failed: $failed_count/$check_count checks failed"
        fi
        
        return 1
    fi
}

# Alert function (can be extended to integrate with monitoring systems)
send_alert() {
    local message="$1"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    echo "[$timestamp] ALERT: $message" >> "$ALERT_LOG"
    
    # Add integration with monitoring systems here:
    # - Email notifications
    # - Slack/Teams webhooks
    # - PagerDuty/Opsgenie
    # - Custom monitoring dashboards
    
    if [[ "$VERBOSE" == true ]]; then
        log "Alert sent: $message"
    fi
}

# Usage and help
show_usage() {
    cat << EOF
$SCRIPT_NAME v$SCRIPT_VERSION - LSP Gateway Health Check Monitor

USAGE:
    $SCRIPT_NAME [OPTIONS]

OPTIONS:
    --quick             Skip LSP validation tests (faster check)
    --quiet             Suppress output except errors
    --verbose           Enable verbose logging
    --alert             Send alerts on failure
    --no-log            Don't write to log files
    -h, --help          Show this help message

EXAMPLES:
    # Standard health check
    $SCRIPT_NAME

    # Quick check (skip LSP tests)
    $SCRIPT_NAME --quick

    # Verbose check with alerting
    $SCRIPT_NAME --verbose --alert

    # Quiet check for cron
    $SCRIPT_NAME --quiet --no-log

EXIT CODES:
    0    All health checks passed
    1    One or more health checks failed
    2    Configuration error
    3    Missing dependencies

The health check validates:
- Disk space and memory usage
- Go environment and version
- LSP Gateway binary functionality
- LSP server availability
- Configuration file integrity
- Network connectivity
- Repository status
- Basic LSP validation tests (unless --quick)

Results are logged to: $HEALTH_CHECK_LOG
Alerts are logged to: $ALERT_LOG
EOF
}

# Command line parsing
parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --quick)
                QUICK_CHECK=true
                shift
                ;;
            --quiet)
                QUIET=true
                shift
                ;;
            --verbose)
                VERBOSE=true
                shift
                ;;
            --alert)
                ALERT_ON_FAILURE=true
                shift
                ;;
            --no-log)
                LOG_RESULTS=false
                shift
                ;;
            -h|--help)
                show_usage
                exit 0
                ;;
            *)
                echo "Unknown option: $1" >&2
                show_usage
                exit 2
                ;;
        esac
    done
}

# Main execution
main() {
    parse_arguments "$@"
    
    # Ensure log directory exists
    mkdir -p "$(dirname "$HEALTH_CHECK_LOG")"
    mkdir -p "$(dirname "$ALERT_LOG")"
    
    # Run health checks
    if run_health_checks; then
        exit 0
    else
        exit 1
    fi
}

# Execute main function if script is run directly (not sourced)
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi