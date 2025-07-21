#!/bin/bash

# LSP Gateway Commit Hash Update Script
# Automated commit hash updates for test repositories with validation

set -euo pipefail

# Script metadata
readonly SCRIPT_NAME="$(basename "$0")"
readonly SCRIPT_VERSION="1.0.0"
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Configuration
readonly CONFIG_FILE="$PROJECT_ROOT/test-repositories.yaml"
readonly BACKUP_DIR="$PROJECT_ROOT/.maintenance-backups"
readonly UPDATE_LOG="$PROJECT_ROOT/commit-update.log"
readonly TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Colors for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly PURPLE='\033[0;35m'
readonly NC='\033[0m'

# Global variables
DRY_RUN=false
INTERACTIVE=true
FORCE_UPDATE=false
VALIDATE_ONLY=false
UPDATE_TYPE="auto"  # auto, latest, stable, manual
FILTER_LANGUAGES=()
FILTER_REPOS=()

# Repository update strategies
declare -A UPDATE_STRATEGIES=(
    ["kubernetes"]="stable"
    ["golang"]="stable" 
    ["django"]="stable"
    ["flask"]="latest"
    ["vscode"]="stable"
    ["typescript"]="stable"
    ["spring-boot"]="stable"
    ["kafka"]="stable"
)

# Logging functions
log() {
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    local message="$*"
    echo "[$timestamp] $message" >> "$UPDATE_LOG"
    echo -e "${BLUE}[$timestamp]${NC} $message"
}

log_success() {
    local message="$*"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] SUCCESS: $message" >> "$UPDATE_LOG"
    echo -e "${GREEN}✓ $message${NC}"
}

log_warning() {
    local message="$*"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] WARNING: $message" >> "$UPDATE_LOG"
    echo -e "${YELLOW}⚠ $message${NC}"
}

log_error() {
    local message="$*"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: $message" >> "$UPDATE_LOG"
    echo -e "${RED}✗ $message${NC}"
}

log_progress() {
    local message="$*"
    echo -e "${PURPLE}⚡ $message${NC}"
}

# Utility functions
create_backup() {
    log "Creating configuration backup..."
    mkdir -p "$BACKUP_DIR"
    
    local backup_file="$BACKUP_DIR/test-repositories.yaml.backup.$TIMESTAMP"
    cp "$CONFIG_FILE" "$backup_file"
    
    log_success "Configuration backed up to: $backup_file"
    echo "$backup_file"
}

get_latest_release() {
    local repo_url="$1"
    local strategy="$2"
    
    # Extract owner/repo from URL
    local owner_repo=$(echo "$repo_url" | sed 's|https://github.com/||' | sed 's|\.git||')
    
    case "$strategy" in
        "stable")
            # Get latest stable release (not pre-release)
            curl -s "https://api.github.com/repos/$owner_repo/releases/latest" | \
                jq -r '.tag_name // empty' 2>/dev/null || echo ""
            ;;
        "latest")
            # Get latest release including pre-releases
            curl -s "https://api.github.com/repos/$owner_repo/releases" | \
                jq -r '.[0].tag_name // empty' 2>/dev/null || echo ""
            ;;
        "commit")
            # Get latest commit on default branch
            curl -s "https://api.github.com/repos/$owner_repo/commits/HEAD" | \
                jq -r '.sha // empty' 2>/dev/null || echo ""
            ;;
        *)
            echo ""
            ;;
    esac
}

get_current_commit() {
    local lang="$1"
    local type="$2"
    
    # Simple YAML parsing for current commit
    awk -v lang="$lang" -v type="$type" '
        BEGIN { found_lang = 0; found_type = 0 }
        $0 ~ "^  " lang ":$" { found_lang = 1; next }
        found_lang && $0 ~ "^    " type ":$" { found_type = 1; next }
        found_type && /^      commit:/ { 
            gsub(/^[ ]*commit:[ ]*"?/, ""); 
            gsub(/"?[ ]*$/, ""); 
            print $0; 
            exit 
        }
        /^  [a-z]+:$/ { found_lang = 0; found_type = 0 }
        /^    (primary|alternative):$/ { if (!found_lang) found_type = 0 }
    ' "$CONFIG_FILE"
}

get_repository_info() {
    local lang="$1"
    local type="$2"
    
    awk -v lang="$lang" -v type="$type" '
        BEGIN { 
            found_lang = 0; found_type = 0
            name = ""; url = ""; commit = ""
        }
        $0 ~ "^  " lang ":$" { found_lang = 1; next }
        found_lang && $0 ~ "^    " type ":$" { found_type = 1; next }
        found_type && /^      name:/ { 
            gsub(/^[ ]*name:[ ]*"?/, ""); gsub(/"?[ ]*$/, ""); 
            name = $0; next 
        }
        found_type && /^      url:/ { 
            gsub(/^[ ]*url:[ ]*"?/, ""); gsub(/"?[ ]*$/, ""); 
            url = $0; next 
        }
        found_type && /^      commit:/ { 
            gsub(/^[ ]*commit:[ ]*"?/, ""); gsub(/"?[ ]*$/, ""); 
            commit = $0; next 
        }
        /^  [a-z]+:$/ { 
            if (found_lang && found_type && name && url && commit) {
                print name "|" url "|" commit
                exit
            }
            found_lang = 0; found_type = 0 
        }
        /^    (primary|alternative):$/ { 
            if (!found_lang) found_type = 0 
        }
        END {
            if (found_lang && found_type && name && url && commit) {
                print name "|" url "|" commit
            }
        }
    ' "$CONFIG_FILE"
}

validate_commit_exists() {
    local repo_url="$1"
    local commit="$2"
    
    # Extract owner/repo from URL
    local owner_repo=$(echo "$repo_url" | sed 's|https://github.com/||' | sed 's|\.git||')
    
    # Check if commit exists via GitHub API
    local http_code
    http_code=$(curl -s -o /dev/null -w "%{http_code}" \
        "https://api.github.com/repos/$owner_repo/commits/$commit")
    
    [[ "$http_code" == "200" ]]
}

update_commit_in_config() {
    local lang="$1"
    local type="$2" 
    local new_commit="$3"
    
    if [[ "$DRY_RUN" == true ]]; then
        log "DRY RUN: Would update $lang/$type commit to: $new_commit"
        return 0
    fi
    
    # Create temporary file for sed operation
    local temp_file=$(mktemp)
    
    # Use awk to update the specific commit
    awk -v lang="$lang" -v type="$type" -v new_commit="$new_commit" '
        BEGIN { found_lang = 0; found_type = 0; updated = 0 }
        $0 ~ "^  " lang ":$" { found_lang = 1; print; next }
        found_lang && $0 ~ "^    " type ":$" { found_type = 1; print; next }
        found_type && /^      commit:/ && !updated { 
            sub(/commit:.*/, "commit: \"" new_commit "\"")
            updated = 1
        }
        /^  [a-z]+:$/ { found_lang = 0; found_type = 0 }
        /^    (primary|alternative):$/ { if (!found_lang) found_type = 0 }
        { print }
    ' "$CONFIG_FILE" > "$temp_file"
    
    # Replace original file
    mv "$temp_file" "$CONFIG_FILE"
    
    log_success "Updated $lang/$type commit to: $new_commit"
}

update_timestamp_in_config() {
    if [[ "$DRY_RUN" == true ]]; then
        log "DRY RUN: Would update timestamp in configuration"
        return 0
    fi
    
    local current_date=$(date +%Y-%m-%d)
    sed -i "s/^# Last updated: .*/# Last updated: $current_date/" "$CONFIG_FILE"
    
    log_success "Updated configuration timestamp to: $current_date"
}

check_repository_update() {
    local lang="$1"
    local type="$2"
    local name="$3"
    local url="$4"  
    local current_commit="$5"
    
    log_progress "Checking updates for $lang/$name..."
    
    # Determine update strategy
    local strategy="${UPDATE_STRATEGIES[$name]:-auto}"
    if [[ "$UPDATE_TYPE" != "auto" ]]; then
        strategy="$UPDATE_TYPE"
    fi
    
    # Get suggested new commit
    local suggested_commit
    suggested_commit=$(get_latest_release "$url" "$strategy")
    
    if [[ -z "$suggested_commit" ]]; then
        log_warning "Could not determine latest version for $name, keeping current: $current_commit"
        return 0
    fi
    
    # Skip if no change
    if [[ "$suggested_commit" == "$current_commit" ]]; then
        log "No update needed for $name (current: $current_commit)"
        return 0
    fi
    
    # Validate new commit exists
    if ! validate_commit_exists "$url" "$suggested_commit"; then
        log_error "Suggested commit does not exist: $suggested_commit for $name"
        return 1
    fi
    
    log "Update available for $name:"
    log "  Current: $current_commit"  
    log "  Latest:  $suggested_commit"
    log "  Strategy: $strategy"
    
    # Interactive confirmation
    if [[ "$INTERACTIVE" == true && "$FORCE_UPDATE" != true ]]; then
        echo -n "Update $name to $suggested_commit? [y/N]: "
        read -r response
        if [[ ! "$response" =~ ^[Yy]$ ]]; then
            log "Skipped update for $name"
            return 0
        fi
    fi
    
    # Perform update
    update_commit_in_config "$lang" "$type" "$suggested_commit"
    
    return 0
}

validate_updated_repository() {
    local lang="$1"
    local name="$2"
    
    log_progress "Validating updated repository: $lang/$name"
    
    # Clean and re-clone repository to test new commit
    if ! ./scripts/clone-repos.sh cleanup "$lang/$name" --force >/dev/null 2>&1; then
        log_warning "Could not clean existing repository: $lang/$name"
    fi
    
    # Clone with new commit
    if ! timeout 300 ./scripts/clone-repos.sh --repos "$name" --force >/dev/null 2>&1; then
        log_error "Failed to clone repository with new commit: $lang/$name"
        return 1
    fi
    
    # Validate repository
    if ! ./scripts/clone-repos.sh validate >/dev/null 2>&1; then
        log_error "Repository validation failed: $lang/$name"  
        return 1
    fi
    
    log_success "Repository validation passed: $lang/$name"
    return 0
}

run_lsp_validation_after_updates() {
    log_progress "Running LSP validation tests after updates..."
    
    if [[ "$DRY_RUN" == true ]]; then
        log "DRY RUN: Would run LSP validation tests"
        return 0
    fi
    
    # Run short validation tests
    if timeout 600 ./scripts/run-lsp-validation-tests.sh short >/dev/null 2>&1; then
        log_success "LSP validation tests passed"
        return 0
    else
        log_error "LSP validation tests failed after updates"
        return 1
    fi
}

process_repository_updates() {
    log "Starting repository commit hash updates..."
    
    local languages=("go" "python" "typescript" "java")
    local types=("primary" "alternative")
    
    local total_repos=0
    local updated_repos=0
    local failed_repos=0
    local updated_repo_list=()
    
    for lang in "${languages[@]}"; do
        # Check language filters
        if [[ ${#FILTER_LANGUAGES[@]} -gt 0 ]]; then
            local lang_included=false
            for filter_lang in "${FILTER_LANGUAGES[@]}"; do
                if [[ "$lang" == "$filter_lang" ]]; then
                    lang_included=true
                    break
                fi
            done
            if [[ "$lang_included" != true ]]; then
                continue
            fi
        fi
        
        for type in "${types[@]}"; do
            local repo_info
            if repo_info=$(get_repository_info "$lang" "$type"); then
                IFS='|' read -r name url current_commit <<< "$repo_info"
                ((total_repos++))
                
                # Check repository filters
                if [[ ${#FILTER_REPOS[@]} -gt 0 ]]; then
                    local repo_included=false
                    for filter_repo in "${FILTER_REPOS[@]}"; do
                        if [[ "$name" == "$filter_repo" || "$lang/$name" == "$filter_repo" ]]; then
                            repo_included=true
                            break
                        fi
                    done
                    if [[ "$repo_included" != true ]]; then
                        continue
                    fi
                fi
                
                # Process repository update
                if check_repository_update "$lang" "$type" "$name" "$url" "$current_commit"; then
                    # Check if actually updated
                    local new_commit
                    new_commit=$(get_current_commit "$lang" "$type")
                    if [[ "$new_commit" != "$current_commit" ]]; then
                        ((updated_repos++))
                        updated_repo_list+=("$lang/$name")
                        
                        # Validate updated repository (if not dry run)
                        if [[ "$DRY_RUN" != true ]]; then
                            if ! validate_updated_repository "$lang" "$name"; then
                                ((failed_repos++))
                                log_error "Validation failed for $lang/$name"
                            fi
                        fi
                    fi
                else
                    ((failed_repos++))
                    log_error "Update failed for $lang/$name"
                fi
            fi
        done
    done
    
    # Update timestamp if any updates were made
    if [[ $updated_repos -gt 0 ]]; then
        update_timestamp_in_config
    fi
    
    # Summary
    log "Repository update summary:"
    log "  Total repositories: $total_repos"
    log "  Updated repositories: $updated_repos"
    log "  Failed repositories: $failed_repos"
    
    if [[ $updated_repos -gt 0 ]]; then
        log "Updated repositories:"
        for repo in "${updated_repo_list[@]}"; do
            log "  - $repo"
        done
    fi
    
    # Return appropriate exit code
    if [[ $failed_repos -gt 0 ]]; then
        return 1
    fi
    
    return 0
}

validate_current_configuration() {
    log "Validating current configuration..."
    
    local languages=("go" "python" "typescript" "java")
    local types=("primary" "alternative")
    
    local validation_passed=true
    local total_repos=0
    local valid_repos=0
    
    for lang in "${languages[@]}"; do
        for type in "${types[@]}"; do
            local repo_info
            if repo_info=$(get_repository_info "$lang" "$type"); then
                IFS='|' read -r name url commit <<< "$repo_info"
                ((total_repos++))
                
                log_progress "Validating $lang/$name commit: $commit"
                
                if validate_commit_exists "$url" "$commit"; then
                    log_success "Valid commit: $lang/$name -> $commit"
                    ((valid_repos++))
                else
                    log_error "Invalid commit: $lang/$name -> $commit"
                    validation_passed=false
                fi
            else
                log_error "Could not parse repository info for: $lang/$type"
                validation_passed=false
            fi
        done
    done
    
    log "Configuration validation summary:"
    log "  Total repositories: $total_repos"
    log "  Valid repositories: $valid_repos"
    
    if [[ "$validation_passed" == true ]]; then
        log_success "All repository commits are valid"
        return 0
    else
        log_error "Configuration validation failed"
        return 1
    fi
}

show_usage() {
    cat << EOF
$SCRIPT_NAME v$SCRIPT_VERSION - LSP Gateway Commit Hash Update Tool

USAGE:
    $SCRIPT_NAME [COMMAND] [OPTIONS]

COMMANDS:
    update      Update repository commit hashes (default)
    validate    Validate current commit hashes only
    status      Show current repository status

OPTIONS:
    --dry-run              Show what would be updated without making changes
    --non-interactive      Don't prompt for confirmation
    --force                Force update without confirmation
    --strategy STRATEGY    Update strategy: auto, latest, stable, commit
    --languages LANGS      Comma-separated list of languages to update
    --repos REPOS          Comma-separated list of repositories to update
    -h, --help             Show this help message

UPDATE STRATEGIES:
    auto       Use repository-specific strategy (default)
    latest     Use latest release (including pre-releases)  
    stable     Use latest stable release only
    commit     Use latest commit on default branch

EXAMPLES:
    # Update all repositories interactively
    $SCRIPT_NAME

    # Dry run to see what would be updated  
    $SCRIPT_NAME --dry-run

    # Update only Go repositories without prompts
    $SCRIPT_NAME --languages go --non-interactive

    # Update specific repositories to latest stable
    $SCRIPT_NAME --repos kubernetes,django --strategy stable

    # Validate current configuration
    $SCRIPT_NAME validate

    # Non-interactive update for automation
    $SCRIPT_NAME --non-interactive --force

The script will:
1. Create backup of current configuration
2. Check for available updates using GitHub API
3. Validate new commits exist
4. Update configuration file
5. Test updated repositories
6. Run LSP validation tests

Results are logged to: $UPDATE_LOG
Backups are stored in: $BACKUP_DIR
EOF
}

parse_arguments() {
    local command=""
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            update|validate|status)
                command="$1"
                shift
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --non-interactive)
                INTERACTIVE=false
                shift
                ;;
            --force)
                FORCE_UPDATE=true
                shift
                ;;
            --strategy)
                UPDATE_TYPE="$2"
                shift 2
                ;;
            --languages)
                IFS=',' read -ra FILTER_LANGUAGES <<< "$2"
                shift 2
                ;;
            --repos)
                IFS=',' read -ra FILTER_REPOS <<< "$2"
                shift 2
                ;;
            -h|--help)
                show_usage
                exit 0
                ;;
            *)
                if [[ -z "$command" ]]; then
                    command="$1"
                else
                    log_error "Unknown argument: $1"
                    show_usage
                    exit 1
                fi
                shift
                ;;
        esac
    done
    
    # Default command
    if [[ -z "$command" ]]; then
        command="update"
    fi
    
    echo "$command"
}

check_dependencies() {
    local missing_deps=()
    
    if ! command -v curl >/dev/null 2>&1; then
        missing_deps+=("curl")
    fi
    
    if ! command -v jq >/dev/null 2>&1; then
        missing_deps+=("jq")
    fi
    
    if [[ ${#missing_deps[@]} -gt 0 ]]; then
        log_error "Missing required dependencies:"
        for dep in "${missing_deps[@]}"; do
            log_error "  - $dep"
        done
        return 1
    fi
    
    return 0
}

main() {
    local command
    command=$(parse_arguments "$@")
    
    log "Starting commit hash update tool v$SCRIPT_VERSION"
    log "Command: $command"
    log "Working directory: $PROJECT_ROOT"
    
    # Check dependencies
    if ! check_dependencies; then
        exit 2
    fi
    
    # Ensure we're in the project root
    cd "$PROJECT_ROOT"
    
    # Check configuration file exists
    if [[ ! -f "$CONFIG_FILE" ]]; then
        log_error "Configuration file not found: $CONFIG_FILE"
        exit 1
    fi
    
    # Execute command
    case "$command" in
        "update")
            # Create backup
            local backup_file
            backup_file=$(create_backup)
            
            # Process updates
            if process_repository_updates; then
                # Run validation tests if updates were made
                if [[ "$DRY_RUN" != true ]] && ! run_lsp_validation_after_updates; then
                    log_error "LSP validation failed, consider rolling back"
                    log "Backup available at: $backup_file"
                    exit 3
                fi
                
                log_success "Commit hash update completed successfully!"
                if [[ "$DRY_RUN" != true ]]; then
                    log "Backup available at: $backup_file"
                fi
            else
                log_error "Commit hash update failed"
                exit 1
            fi
            ;;
        
        "validate")
            if validate_current_configuration; then
                log_success "Configuration validation passed"
            else
                exit 1
            fi
            ;;
        
        "status")
            ./scripts/clone-repos.sh status
            ;;
        
        *)
            log_error "Unknown command: $command"
            show_usage
            exit 1
            ;;
    esac
}

# Execute main function if script is run directly (not sourced)
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi