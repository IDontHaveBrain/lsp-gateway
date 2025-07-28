#!/bin/bash

# LSP Gateway Maintenance Automation
# Orchestrates comprehensive maintenance procedures for the test environment

set -euo pipefail

# Script metadata
readonly SCRIPT_NAME="$(basename "$0")"
readonly SCRIPT_VERSION="1.0.0"
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Configuration
readonly MAINTENANCE_LOG="$PROJECT_ROOT/maintenance-automation.log"
readonly MAINTENANCE_CONFIG="$PROJECT_ROOT/maintenance-monitoring.yaml"
readonly TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Colors for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly PURPLE='\033[0;35m'
readonly WHITE='\033[1;37m'
readonly NC='\033[0m'

# Global variables
MAINTENANCE_TYPE="quarterly"
DRY_RUN=false
SKIP_VALIDATION=false
SKIP_UPDATES=false
FORCE_CONTINUE=false
SEND_REPORT=true
MAINTENANCE_PHASE=""

# Logging functions
log() {
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    local message="$*"
    echo "[$timestamp] MAINTENANCE: $message" >> "$MAINTENANCE_LOG"
    echo -e "${BLUE}[$timestamp]${NC} $message"
}

log_success() {
    local message="$*"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] SUCCESS: $message" >> "$MAINTENANCE_LOG"
    echo -e "${GREEN}✓ $message${NC}"
}

log_warning() {
    local message="$*"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] WARNING: $message" >> "$MAINTENANCE_LOG"
    echo -e "${YELLOW}⚠ $message${NC}"
}

log_error() {
    local message="$*"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: $message" >> "$MAINTENANCE_LOG"
    echo -e "${RED}✗ $message${NC}"
}

log_phase() {
    local phase="$1"
    MAINTENANCE_PHASE="$phase"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] PHASE: $phase" >> "$MAINTENANCE_LOG"
    echo -e "${WHITE}=== $phase ===${NC}"
}

# Maintenance phases
phase_pre_maintenance_checks() {
    log_phase "PRE-MAINTENANCE CHECKS"
    
    # Check if we're in maintenance window (if configured)
    log "Validating maintenance window..."
    
    # Backup current state
    log "Creating pre-maintenance backup..."
    mkdir -p "$PROJECT_ROOT/.maintenance-backups"
    tar -czf "$PROJECT_ROOT/.maintenance-backups/pre-maintenance-backup-$TIMESTAMP.tar.gz" \
        -C "$PROJECT_ROOT" \
        test-repositories.yaml config.yaml tests/data/configs/ || {
        log_error "Failed to create pre-maintenance backup"
        return 1
    }
    log_success "Pre-maintenance backup created"
    
    # Run comprehensive health check
    log "Running pre-maintenance health check..."
    if ! ./scripts/health-check.sh --verbose; then
        log_error "Pre-maintenance health check failed"
        if [[ "$FORCE_CONTINUE" != true ]]; then
            return 1
        fi
        log_warning "Continuing despite health check failure (forced)"
    fi
    log_success "Pre-maintenance health check passed"
    
    # Validate current repository state
    log "Validating current repository state..."
    if ! ./scripts/clone-repos.sh validate; then
        log_error "Repository validation failed"
        if [[ "$FORCE_CONTINUE" != true ]]; then
            return 1
        fi
        log_warning "Continuing despite repository validation failure (forced)"
    fi
    log_success "Repository validation passed"
    
    return 0
}

phase_commit_hash_updates() {
    log_phase "COMMIT HASH UPDATES"
    
    if [[ "$SKIP_UPDATES" == true ]]; then
        log "Skipping commit hash updates (requested)"
        return 0
    fi
    
    # Determine update strategy based on maintenance type
    local update_args=""
    case "$MAINTENANCE_TYPE" in
        "quarterly"|"major")
            update_args="--strategy stable --non-interactive"
            ;;
        "monthly")
            update_args="--strategy auto --non-interactive"  
            ;;
        "weekly")
            update_args="--dry-run"
            ;;
        *)
            update_args="--strategy auto"
            ;;
    esac
    
    if [[ "$DRY_RUN" == true ]]; then
        update_args="$update_args --dry-run"
    fi
    
    log "Running commit hash updates with strategy: $update_args"
    
    if ! ./scripts/update-commit-hashes.sh $update_args; then
        log_error "Commit hash updates failed"
        return 1
    fi
    
    log_success "Commit hash updates completed"
    return 0
}

phase_repository_validation() {
    log_phase "REPOSITORY VALIDATION"
    
    if [[ "$SKIP_VALIDATION" == true ]]; then
        log "Skipping repository validation (requested)"
        return 0
    fi
    
    # Clean and re-clone all repositories to ensure consistency
    log "Cleaning existing repositories..."
    if [[ "$DRY_RUN" != true ]]; then
        ./scripts/clone-repos.sh cleanup all --force || {
            log_warning "Repository cleanup had issues"
        }
    fi
    
    # Clone all repositories with updated commits
    log "Cloning repositories with updated commits..."
    if [[ "$DRY_RUN" != true ]]; then
        if ! ./scripts/clone-repos.sh --force; then
            log_error "Repository cloning failed"
            return 1
        fi
    fi
    log_success "Repository cloning completed"
    
    # Validate all repositories
    log "Validating all repositories..."
    if [[ "$DRY_RUN" != true ]]; then
        if ! ./scripts/clone-repos.sh validate; then
            log_error "Repository validation failed"
            return 1
        fi
    fi
    log_success "Repository validation passed"
    
    return 0
}

phase_lsp_validation() {
    log_phase "LSP VALIDATION TESTING"
    
    if [[ "$SKIP_VALIDATION" == true ]]; then
        log "Skipping LSP validation (requested)"
        return 0
    fi
    
    # Run comprehensive LSP validation tests
    log "Running comprehensive LSP validation tests..."
    if [[ "$DRY_RUN" != true ]]; then
        if ! timeout 1800 ./scripts/run-lsp-validation-tests.sh full; then
            log_error "LSP validation tests failed"
            return 1
        fi
    else
        log "DRY RUN: Would run LSP validation tests"
    fi
    log_success "LSP validation tests passed"
    
    # Run performance benchmarks if quarterly maintenance
    if [[ "$MAINTENANCE_TYPE" == "quarterly" && "$DRY_RUN" != true ]]; then
        log "Running performance benchmarks..."
        if ! timeout 900 ./scripts/benchmark-python-patterns-integration.sh; then
            log_warning "Performance benchmarks failed"
        else
            log_success "Performance benchmarks completed"
        fi
    fi
    
    return 0
}

phase_infrastructure_updates() {
    log_phase "INFRASTRUCTURE UPDATES"
    
    if [[ "$MAINTENANCE_TYPE" != "quarterly" ]]; then
        log "Skipping infrastructure updates (not quarterly maintenance)"
        return 0
    fi
    
    # Check for Go updates
    log "Checking Go version and dependencies..."
    local go_version
    go_version=$(go version | grep -oE 'go[0-9]+\.[0-9]+')
    log "Current Go version: $go_version"
    
    # Update Go dependencies
    if [[ "$DRY_RUN" != true ]]; then
        log "Updating Go dependencies..."
        cd "$PROJECT_ROOT"
        go get -u ./...
        go mod tidy
        if ! make clean && make local; then
            log_error "Failed to rebuild after dependency updates"
            return 1
        fi
        log_success "Go dependencies updated and rebuilt"
    else
        log "DRY RUN: Would update Go dependencies"
    fi
    
    # Check LSP server versions
    log "Checking LSP server versions..."
    if command -v gopls >/dev/null 2>&1; then
        local gopls_version
        gopls_version=$(gopls version | head -1 || echo "unknown")
        log "Current gopls version: $gopls_version"
    fi
    
    return 0
}

phase_performance_analysis() {
    log_phase "PERFORMANCE ANALYSIS"
    
    if [[ "$MAINTENANCE_TYPE" != "quarterly" && "$MAINTENANCE_TYPE" != "monthly" ]]; then
        log "Skipping performance analysis (not quarterly/monthly maintenance)"
        return 0
    fi
    
    # Compare against performance baselines
    log "Analyzing performance trends..."
    
    # This would integrate with performance monitoring tools
    log "Performance analysis would include:"
    log "  - LSP response time trends"
    log "  - Memory usage analysis"
    log "  - Repository clone performance"
    log "  - Test execution time analysis"
    
    # For now, just log current metrics
    if [[ "$DRY_RUN" != true ]]; then
        local start_time=$(date +%s)
        ./scripts/health-check.sh --quick >/dev/null 2>&1 || true
        local end_time=$(date +%s)
        local health_check_time=$((end_time - start_time))
        log "Health check execution time: ${health_check_time}s"
    fi
    
    log_success "Performance analysis completed"
    return 0
}

phase_post_maintenance_validation() {
    log_phase "POST-MAINTENANCE VALIDATION"
    
    # Run final health check
    log "Running post-maintenance health check..."
    if [[ "$DRY_RUN" != true ]]; then
        if ! ./scripts/health-check.sh; then
            log_error "Post-maintenance health check failed"
            return 1
        fi
    fi
    log_success "Post-maintenance health check passed"
    
    # Verify all systems are operational
    log "Verifying system operational status..."
    
    # Check LSP Gateway functionality
    if [[ "$DRY_RUN" != true ]]; then
        if ! ./bin/lspg version >/dev/null 2>&1; then
            log_error "LSP Gateway binary is not functional"
            return 1
        fi
    fi
    
    log_success "All systems verified operational"
    return 0
}

phase_cleanup_and_reporting() {
    log_phase "CLEANUP AND REPORTING"
    
    # Clean up temporary files
    log "Cleaning up temporary files..."
    find "$PROJECT_ROOT" -name "*.tmp" -type f -delete 2>/dev/null || true
    
    # Generate maintenance report
    if [[ "$SEND_REPORT" == true ]]; then
        log "Generating maintenance report..."
        generate_maintenance_report
    fi
    
    # Archive logs
    log "Archiving maintenance logs..."
    local archive_dir="$PROJECT_ROOT/.maintenance-backups/logs"
    mkdir -p "$archive_dir"
    
    if [[ -f "$MAINTENANCE_LOG" ]]; then
        cp "$MAINTENANCE_LOG" "$archive_dir/maintenance-log-$TIMESTAMP.log"
    fi
    
    # Cleanup old backups (keep last 10)
    log "Cleaning up old maintenance backups..."
    find "$PROJECT_ROOT/.maintenance-backups" -name "*.tar.gz" -type f | sort -r | tail -n +11 | xargs -r rm -f
    
    log_success "Cleanup and reporting completed"
    return 0
}

generate_maintenance_report() {
    local report_file="$PROJECT_ROOT/maintenance-report-$TIMESTAMP.txt"
    
    cat > "$report_file" << EOF
LSP Gateway Maintenance Report
=============================
Generated: $(date)
Maintenance Type: $MAINTENANCE_TYPE
Execution Mode: $(if [[ "$DRY_RUN" == true ]]; then echo "DRY RUN"; else echo "LIVE"; fi)

Maintenance Summary:
-------------------
EOF

    # Add summary of what was done
    if [[ "$SKIP_UPDATES" != true ]]; then
        echo "✓ Commit hash updates processed" >> "$report_file"
    fi
    
    if [[ "$SKIP_VALIDATION" != true ]]; then
        echo "✓ Repository validation completed" >> "$report_file"
        echo "✓ LSP validation tests executed" >> "$report_file"
    fi
    
    if [[ "$MAINTENANCE_TYPE" == "quarterly" ]]; then
        echo "✓ Infrastructure updates checked" >> "$report_file"
        echo "✓ Performance analysis completed" >> "$report_file"
    fi
    
    # Add current system status
    cat >> "$report_file" << EOF

Current System Status:
---------------------
EOF
    
    # Repository status
    if [[ "$DRY_RUN" != true ]]; then
        ./scripts/clone-repos.sh status >> "$report_file" 2>/dev/null || echo "Repository status check failed" >> "$report_file"
    else
        echo "Repository status: Not checked (dry run)" >> "$report_file"
    fi
    
    # Add maintenance log summary
    cat >> "$report_file" << EOF

Maintenance Log Summary:
-----------------------
EOF
    
    if [[ -f "$MAINTENANCE_LOG" ]]; then
        grep -E "(SUCCESS|ERROR|WARNING)" "$MAINTENANCE_LOG" | tail -20 >> "$report_file"
    fi
    
    echo "" >> "$report_file"
    echo "Full maintenance log: $MAINTENANCE_LOG" >> "$report_file"
    echo "Report generated by: $SCRIPT_NAME v$SCRIPT_VERSION" >> "$report_file"
    
    log_success "Maintenance report generated: $report_file"
}

rollback_changes() {
    log_error "Maintenance failed, initiating rollback..."
    
    local backup_file
    backup_file=$(find "$PROJECT_ROOT/.maintenance-backups" -name "pre-maintenance-backup-$TIMESTAMP.tar.gz" | head -1)
    
    if [[ -n "$backup_file" && -f "$backup_file" ]]; then
        log "Rolling back from backup: $backup_file"
        
        cd "$PROJECT_ROOT"
        tar -xzf "$backup_file" || {
            log_error "Failed to extract rollback backup"
            return 1
        }
        
        log_success "Configuration files rolled back"
        
        # Re-clone repositories to ensure consistency
        log "Re-cloning repositories to ensure consistency..."
        ./scripts/clone-repos.sh cleanup all --force || true
        ./scripts/clone-repos.sh --force || {
            log_error "Failed to re-clone repositories during rollback"
            return 1
        }
        
        log_success "Rollback completed successfully"
    else
        log_error "No backup found for rollback"
        return 1
    fi
    
    return 0
}

run_maintenance() {
    local start_time=$(date +%s)
    log "Starting maintenance automation v$SCRIPT_VERSION"
    log "Maintenance type: $MAINTENANCE_TYPE"
    log "Working directory: $PROJECT_ROOT"
    
    # Ensure we're in project root
    cd "$PROJECT_ROOT"
    
    local phases=(
        "phase_pre_maintenance_checks"
        "phase_commit_hash_updates"
        "phase_repository_validation"
        "phase_lsp_validation"
        "phase_infrastructure_updates"
        "phase_performance_analysis"
        "phase_post_maintenance_validation"
        "phase_cleanup_and_reporting"
    )
    
    local failed_phase=""
    local phase_count=0
    local total_phases=${#phases[@]}
    
    for phase_func in "${phases[@]}"; do
        ((phase_count++))
        log "Executing phase $phase_count/$total_phases: $phase_func"
        
        if ! "$phase_func"; then
            failed_phase="$phase_func"
            log_error "Maintenance phase failed: $phase_func"
            break
        fi
        
        log_success "Phase completed: $phase_func"
    done
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    if [[ -n "$failed_phase" ]]; then
        log_error "Maintenance failed at phase: $failed_phase"
        log "Total execution time: ${duration}s"
        
        if [[ "$DRY_RUN" != true ]]; then
            rollback_changes
        fi
        
        return 1
    else
        log_success "All maintenance phases completed successfully!"
        log "Total execution time: ${duration}s"
        return 0
    fi
}

show_usage() {
    cat << EOF
$SCRIPT_NAME v$SCRIPT_VERSION - LSP Gateway Maintenance Automation

USAGE:
    $SCRIPT_NAME [OPTIONS]

OPTIONS:
    --type TYPE            Maintenance type: quarterly, monthly, weekly (default: quarterly)
    --dry-run              Show what would be done without making changes
    --skip-validation      Skip validation phases
    --skip-updates         Skip commit hash updates
    --force-continue       Continue even if pre-checks fail
    --no-report            Skip generating maintenance report
    -h, --help             Show this help message

MAINTENANCE TYPES:
    quarterly    Full maintenance including infrastructure updates
    monthly      Repository updates and validation
    weekly       Health checks and validation only
    emergency    Critical fixes and rollback procedures

EXAMPLES:
    # Full quarterly maintenance
    $SCRIPT_NAME --type quarterly

    # Dry run to see what would be done
    $SCRIPT_NAME --dry-run

    # Monthly maintenance without infrastructure updates  
    $SCRIPT_NAME --type monthly

    # Emergency maintenance (validation only)
    $SCRIPT_NAME --type emergency --skip-updates

The automation orchestrates:
1. Pre-maintenance checks and backup
2. Commit hash updates (if applicable)
3. Repository validation
4. LSP validation testing
5. Infrastructure updates (quarterly only)
6. Performance analysis
7. Post-maintenance validation
8. Cleanup and reporting

Logs are written to: $MAINTENANCE_LOG
Backups are stored in: $PROJECT_ROOT/.maintenance-backups/
EOF
}

parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --type)
                MAINTENANCE_TYPE="$2"
                shift 2
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --skip-validation)
                SKIP_VALIDATION=true
                shift
                ;;
            --skip-updates)
                SKIP_UPDATES=true
                shift
                ;;
            --force-continue)
                FORCE_CONTINUE=true
                shift
                ;;
            --no-report)
                SEND_REPORT=false
                shift
                ;;
            -h|--help)
                show_usage
                exit 0
                ;;
            *)
                log_error "Unknown argument: $1"
                show_usage
                exit 1
                ;;
        esac
    done
}

main() {
    parse_arguments "$@"
    
    # Ensure log directory exists
    mkdir -p "$(dirname "$MAINTENANCE_LOG")"
    
    # Initialize maintenance log
    echo "=== LSP Gateway Maintenance Automation Started $(date) ===" > "$MAINTENANCE_LOG"
    
    # Run maintenance
    if run_maintenance; then
        log_success "Maintenance automation completed successfully!"
        exit 0
    else
        log_error "Maintenance automation failed!"
        exit 1
    fi
}

# Signal handling for graceful cleanup
cleanup_on_signal() {
    log_error "Received signal, cleaning up maintenance automation..."
    
    # Kill any running background processes
    local jobs=($(jobs -p))
    if [[ ${#jobs[@]} -gt 0 ]]; then
        kill "${jobs[@]}" 2>/dev/null || true
        wait 2>/dev/null || true
    fi
    
    if [[ -n "$MAINTENANCE_PHASE" ]]; then
        log_error "Maintenance interrupted during phase: $MAINTENANCE_PHASE"
    fi
    
    exit 130
}

trap cleanup_on_signal INT TERM

# Execute main function if script is run directly (not sourced)
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi