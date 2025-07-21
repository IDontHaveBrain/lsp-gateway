#!/bin/bash

# LSP Gateway Test Repository Cloning Script
# Clones and manages test repositories for LSP server validation
# Supports selective cloning, commit verification, and comprehensive error handling

set -euo pipefail

# Script metadata
readonly SCRIPT_NAME="$(basename "$0")"
readonly SCRIPT_VERSION="1.0.0"
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Default configuration
readonly DEFAULT_CONFIG_FILE="$PROJECT_ROOT/test-repositories.yaml"
readonly DEFAULT_BASE_DIR="$PROJECT_ROOT/test-repositories"
readonly DEFAULT_LOG_LEVEL="INFO"
readonly DEFAULT_MAX_CONCURRENT=3
readonly DEFAULT_TIMEOUT_MINUTES=30

# Colors for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly PURPLE='\033[0;35m'
readonly CYAN='\033[0;36m'
readonly WHITE='\033[1;37m'
readonly NC='\033[0m' # No Color

# Global variables
CONFIG_FILE="$DEFAULT_CONFIG_FILE"
BASE_DIR="$DEFAULT_BASE_DIR"
LOG_LEVEL="$DEFAULT_LOG_LEVEL"
MAX_CONCURRENT="$DEFAULT_MAX_CONCURRENT"
TIMEOUT_MINUTES="$DEFAULT_TIMEOUT_MINUTES"
SHALLOW_CLONE=true
VERIFY_COMMITS=true
CLEANUP_ON_FAILURE=true
FORCE_RECLONE=false
DRY_RUN=false
QUIET=false
VERBOSE=false

# Language filters
FILTER_LANGUAGES=()
FILTER_REPOS=()
EXCLUDE_LANGUAGES=()
EXCLUDE_REPOS=()

# Logging functions
log_level_to_num() {
    case "$1" in
        "ERROR") echo 1 ;;
        "WARN")  echo 2 ;;
        "INFO")  echo 3 ;;
        "DEBUG") echo 4 ;;
        *) echo 3 ;;
    esac
}

should_log() {
    local level="$1"
    local current_level_num=$(log_level_to_num "$LOG_LEVEL")
    local msg_level_num=$(log_level_to_num "$level")
    [[ $msg_level_num -le $current_level_num ]]
}

log() {
    local level="$1"
    shift
    local message="$*"
    
    if ! should_log "$level"; then
        return 0
    fi
    
    if [[ "$QUIET" == true && "$level" != "ERROR" ]]; then
        return 0
    fi
    
    local timestamp=$(date '+%H:%M:%S')
    local color=""
    local symbol=""
    
    case "$level" in
        "ERROR") color="$RED"; symbol="✗" ;;
        "WARN")  color="$YELLOW"; symbol="⚠" ;;
        "INFO")  color="$BLUE"; symbol="ℹ" ;;
        "DEBUG") color="$CYAN"; symbol="🐛" ;;
    esac
    
    echo -e "${color}[$timestamp] $symbol ${WHITE}$message${NC}" >&2
}

log_error() { log "ERROR" "$@"; }
log_warn()  { log "WARN" "$@"; }
log_info()  { log "INFO" "$@"; }
log_debug() { log "DEBUG" "$@"; }

log_success() {
    local message="$*"
    if [[ "$QUIET" != true ]]; then
        echo -e "${GREEN}[$(date '+%H:%M:%S')] ✓ ${WHITE}$message${NC}" >&2
    fi
}

log_progress() {
    local message="$*"
    if [[ "$QUIET" != true ]]; then
        echo -e "${PURPLE}[$(date '+%H:%M:%S')] ⚡ ${WHITE}$message${NC}" >&2
    fi
}

# Utility functions
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

is_git_repo() {
    local dir="$1"
    [[ -d "$dir/.git" ]] || git -C "$dir" rev-parse --git-dir >/dev/null 2>&1
}

get_git_commit() {
    local dir="$1"
    git -C "$dir" rev-parse HEAD 2>/dev/null || echo ""
}

get_git_remote_url() {
    local dir="$1"
    git -C "$dir" config --get remote.origin.url 2>/dev/null || echo ""
}

validate_commit_format() {
    local commit="$1"
    # Accept full SHA, short SHA, tag, or branch name
    if [[ "$commit" =~ ^[a-fA-F0-9]{7,40}$ ]] || [[ "$commit" =~ ^[a-zA-Z0-9][a-zA-Z0-9._/-]+$ ]]; then
        return 0
    fi
    return 1
}

format_size() {
    local size_mb="$1"
    if [[ "$size_mb" =~ ^~?([0-9]+)$ ]]; then
        local num="${BASH_REMATCH[1]}"
        if [[ $num -gt 1024 ]]; then
            echo "$(($num / 1024))GB"
        else
            echo "${num}MB"
        fi
    else
        echo "$size_mb"
    fi
}

# Simple YAML parsing functions (no external dependencies)
parse_yaml() {
    local yaml_file="$1"
    local prefix="$2"
    
    if [[ ! -f "$yaml_file" ]]; then
        log_error "Configuration file not found: $yaml_file"
        return 1
    fi
    
    # Simple YAML parser using awk for our specific structure
    # This handles the basic structure we need without requiring PyYAML
    awk '
        BEGIN { 
            in_repositories = 0
            in_config = 0
            current_lang = ""
            current_type = ""
        }
        
        # Skip comments and empty lines
        /^#/ || /^[[:space:]]*$/ { next }
        
        # Main sections
        /^repositories:/ { in_repositories = 1; in_config = 0; next }
        /^config:/ { in_config = 1; in_repositories = 0; next }
        /^validation:/ { in_repositories = 0; in_config = 0; next }
        
        # Language level (go:, python:, etc.)
        in_repositories && /^  [a-z]+:$/ {
            current_lang = substr($1, 1, length($1)-1)
            next
        }
        
        # Type level (primary:, alternative:)
        in_repositories && /^    (primary|alternative):$/ {
            current_type = substr($1, 1, length($1)-1)
            next
        }
        
        # Repository properties
        in_repositories && /^      [a-z_]+:/ {
            key = substr($1, 1, length($1)-1)
            value = $0
            gsub(/^      [a-z_]+:[[:space:]]*/, "", value)
            # Remove quotes if present
            gsub(/^"/, "", value)
            gsub(/"$/, "", value)
            printf "repositories_%s_%s_%s=\"%s\"\n", current_lang, current_type, key, value
        }
        
        # Config properties
        in_config && /^  [a-z_]+:/ {
            key = substr($1, 1, length($1)-1)
            value = $0
            gsub(/^  [a-z_]+:[[:space:]]*/, "", value)
            # Remove quotes and handle booleans
            gsub(/^"/, "", value)
            gsub(/"$/, "", value)
            printf "config_%s=\"%s\"\n", key, value
        }
    ' "$yaml_file"
}

# Configuration loading
load_config() {
    log_debug "Loading configuration from: $CONFIG_FILE"
    
    if [[ ! -f "$CONFIG_FILE" ]]; then
        log_error "Configuration file not found: $CONFIG_FILE"
        return 1
    fi
    
    # Parse configuration using our simple YAML parser
    local config_vars
    if ! config_vars=$(parse_yaml "$CONFIG_FILE" ""); then
        log_error "Failed to parse configuration file"
        return 1
    fi
    
    # Load global configuration if present
    if echo "$config_vars" | grep -q "^config_"; then
        eval "$(echo "$config_vars" | grep '^config_')"
        
        # Override defaults with config file values
        BASE_DIR="${config_base_directory:-$BASE_DIR}"
        MAX_CONCURRENT="${config_max_concurrent_clones:-$MAX_CONCURRENT}"
        TIMEOUT_MINUTES="${config_clone_timeout_minutes:-$TIMEOUT_MINUTES}"
        SHALLOW_CLONE="${config_shallow_clone:-$SHALLOW_CLONE}"
        VERIFY_COMMITS="${config_verify_commits:-$VERIFY_COMMITS}"
        CLEANUP_ON_FAILURE="${config_cleanup_on_failure:-$CLEANUP_ON_FAILURE}"
        
        # Convert relative paths to absolute paths from project root
        if [[ "$BASE_DIR" == "./"* ]]; then
            BASE_DIR="$PROJECT_ROOT/${BASE_DIR#./}"
        fi
    fi
    
    log_debug "Configuration loaded successfully"
    return 0
}

# Repository information extraction
get_repository_info() {
    local lang="$1"
    local type="$2"  # primary or alternative
    
    local config_vars
    if ! config_vars=$(parse_yaml "$CONFIG_FILE" ""); then
        return 1
    fi
    
    # Extract repository information
    eval "$(echo "$config_vars" | grep "^repositories_${lang}_${type}_")"
    
    # Check if we have the required information
    local name_var="repositories_${lang}_${type}_name"
    local url_var="repositories_${lang}_${type}_url"
    local commit_var="repositories_${lang}_${type}_commit"
    
    eval "local name=\${$name_var:-}"
    eval "local url=\${$url_var:-}"
    eval "local commit=\${$commit_var:-}"
    
    if [[ -z "$name" || -z "$url" || -z "$commit" ]]; then
        return 1
    fi
    
    echo "$name|$url|$commit"
}

# Repository listing
list_repositories() {
    local format="${1:-table}"
    
    log_info "Available test repositories:"
    
    if [[ "$format" == "table" ]]; then
        printf "%-12s %-12s %-20s %-50s %-12s\n" "LANGUAGE" "TYPE" "NAME" "URL" "COMMIT"
        printf "%-12s %-12s %-20s %-50s %-12s\n" "--------" "----" "----" "---" "------"
    fi
    
    local languages=("go" "python" "typescript" "java")
    local types=("primary" "alternative")
    
    for lang in "${languages[@]}"; do
        for type in "${types[@]}"; do
            local repo_info
            if repo_info=$(get_repository_info "$lang" "$type"); then
                IFS='|' read -r name url commit <<< "$repo_info"
                
                if [[ "$format" == "table" ]]; then
                    printf "%-12s %-12s %-20s %-50s %-12s\n" "$lang" "$type" "$name" "$url" "$commit"
                elif [[ "$format" == "simple" ]]; then
                    echo "$lang/$name"
                fi
            fi
        done
    done
}

# Repository validation
validate_repository() {
    local repo_dir="$1"
    local lang="$2"
    local expected_commit="$3"
    local url="$4"
    
    log_debug "Validating repository: $repo_dir"
    
    # Check if directory exists and is a git repository
    if [[ ! -d "$repo_dir" ]]; then
        log_error "Repository directory does not exist: $repo_dir"
        return 1
    fi
    
    if ! is_git_repo "$repo_dir"; then
        log_error "Directory is not a git repository: $repo_dir"
        return 1
    fi
    
    # Verify remote URL
    local actual_url
    actual_url=$(get_git_remote_url "$repo_dir")
    if [[ "$actual_url" != "$url" ]]; then
        log_warn "Remote URL mismatch for $repo_dir: expected '$url', got '$actual_url'"
    fi
    
    # Verify commit if required
    if [[ "$VERIFY_COMMITS" == true ]]; then
        local actual_commit
        actual_commit=$(get_git_commit "$repo_dir")
        
        # For tag/branch names, resolve to commit hash
        local expected_commit_hash
        if ! expected_commit_hash=$(git -C "$repo_dir" rev-parse "$expected_commit" 2>/dev/null); then
            log_error "Cannot resolve commit '$expected_commit' in $repo_dir"
            return 1
        fi
        
        if [[ "$actual_commit" != "$expected_commit_hash" ]]; then
            log_error "Commit mismatch for $repo_dir: expected '$expected_commit_hash', got '$actual_commit'"
            return 1
        fi
    fi
    
    # Validate language-specific files if configured
    local config_vars
    if config_vars=$(parse_yaml "$CONFIG_FILE" ""); then
        eval "$(echo "$config_vars" | grep '^validation_required_files_')"
        
        local required_files_var="validation_required_files_${lang}"
        eval "local required_files=(\${$required_files_var[@]:-})" 2>/dev/null || true
        
        for file in "${required_files[@]:-}"; do
            if [[ -n "$file" && ! -f "$repo_dir/$file" ]]; then
                log_warn "Expected file not found in $repo_dir: $file"
            fi
        done
    fi
    
    log_debug "Repository validation successful: $repo_dir"
    return 0
}

# Repository cloning
clone_repository() {
    local name="$1"
    local url="$2" 
    local commit="$3"
    local target_dir="$4"
    local lang="$5"
    
    local repo_dir="$target_dir/$name"
    
    log_info "Cloning $name ($lang) to $repo_dir"
    
    if [[ "$DRY_RUN" == true ]]; then
        log_info "[DRY-RUN] Would clone: $url -> $repo_dir (commit: $commit)"
        return 0
    fi
    
    # Check if repository already exists
    if [[ -d "$repo_dir" ]]; then
        if [[ "$FORCE_RECLONE" == true ]]; then
            log_info "Removing existing repository: $repo_dir"
            rm -rf "$repo_dir"
        elif is_git_repo "$repo_dir"; then
            log_info "Repository already exists, validating: $repo_dir"
            if validate_repository "$repo_dir" "$lang" "$commit" "$url"; then
                log_success "Repository already cloned and valid: $name"
                return 0
            else
                log_warn "Existing repository is invalid, re-cloning: $repo_dir"
                rm -rf "$repo_dir"
            fi
        else
            log_error "Directory exists but is not a git repository: $repo_dir"
            return 1
        fi
    fi
    
    # Prepare clone command
    local clone_args=("clone")
    
    if [[ "$SHALLOW_CLONE" == true ]]; then
        clone_args+=("--depth" "1")
    fi
    
    clone_args+=("$url" "$repo_dir")
    
    # Clone with timeout
    local clone_timeout=$((TIMEOUT_MINUTES * 60))
    
    log_progress "Cloning $name... (timeout: ${TIMEOUT_MINUTES}m)"
    
    if timeout "$clone_timeout" git "${clone_args[@]}" >/dev/null 2>&1; then
        log_success "Successfully cloned: $name"
    else
        local exit_code=$?
        log_error "Failed to clone $name (exit code: $exit_code)"
        
        # Cleanup on failure
        if [[ "$CLEANUP_ON_FAILURE" == true && -d "$repo_dir" ]]; then
            log_debug "Cleaning up failed clone: $repo_dir"
            rm -rf "$repo_dir"
        fi
        
        return 1
    fi
    
    # Checkout specific commit/tag/branch
    if [[ -n "$commit" ]]; then
        log_progress "Checking out commit: $commit"
        
        if ! git -C "$repo_dir" checkout "$commit" >/dev/null 2>&1; then
            log_error "Failed to checkout commit '$commit' for $name"
            
            # Try fetching if it's a shallow clone
            if [[ "$SHALLOW_CLONE" == true ]]; then
                log_info "Attempting to fetch commit for shallow clone..."
                if git -C "$repo_dir" fetch --depth=50 origin "$commit" >/dev/null 2>&1; then
                    if git -C "$repo_dir" checkout "$commit" >/dev/null 2>&1; then
                        log_success "Successfully checked out commit after fetch: $commit"
                    else
                        log_error "Still failed to checkout after fetch: $commit"
                        return 1
                    fi
                else
                    log_error "Failed to fetch commit: $commit"
                    return 1
                fi
            else
                return 1
            fi
        fi
    fi
    
    # Final validation
    if ! validate_repository "$repo_dir" "$lang" "$commit" "$url"; then
        log_error "Repository validation failed after clone: $name"
        return 1
    fi
    
    # Get repository size for reporting
    local size_kb
    size_kb=$(du -sk "$repo_dir" 2>/dev/null | cut -f1 || echo "0")
    local size_mb=$((size_kb / 1024))
    
    log_success "Repository cloned and validated: $name (${size_mb}MB)"
    return 0
}

# Main cloning logic
clone_repositories() {
    log_info "Starting repository cloning process"
    log_info "Base directory: $BASE_DIR"
    log_info "Max concurrent clones: $MAX_CONCURRENT"
    log_info "Shallow clone: $SHALLOW_CLONE"
    log_info "Verify commits: $VERIFY_COMMITS"
    
    # Create base directory
    if [[ "$DRY_RUN" != true ]]; then
        mkdir -p "$BASE_DIR"
    fi
    
    local languages=("go" "python" "typescript" "java")
    local types=("primary" "alternative")
    
    # Collect repositories to clone
    local repos_to_clone=()
    local total_repos=0
    
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
                log_debug "Skipping language (not in filter): $lang"
                continue
            fi
        fi
        
        # Check language exclusions
        local lang_excluded=false
        for exclude_lang in "${EXCLUDE_LANGUAGES[@]}"; do
            if [[ "$lang" == "$exclude_lang" ]]; then
                lang_excluded=true
                break
            fi
        done
        if [[ "$lang_excluded" == true ]]; then
            log_debug "Skipping language (excluded): $lang"
            continue
        fi
        
        # Create language directory
        local lang_dir="$BASE_DIR/$lang"
        if [[ "$DRY_RUN" != true ]]; then
            mkdir -p "$lang_dir"
        fi
        
        for type in "${types[@]}"; do
            local repo_info
            if repo_info=$(get_repository_info "$lang" "$type"); then
                IFS='|' read -r name url commit <<< "$repo_info"
                
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
                        log_debug "Skipping repository (not in filter): $lang/$name"
                        continue
                    fi
                fi
                
                # Check repository exclusions
                local repo_excluded=false
                for exclude_repo in "${EXCLUDE_REPOS[@]}"; do
                    if [[ "$name" == "$exclude_repo" || "$lang/$name" == "$exclude_repo" ]]; then
                        repo_excluded=true
                        break
                    fi
                done
                if [[ "$repo_excluded" == true ]]; then
                    log_debug "Skipping repository (excluded): $lang/$name"
                    continue
                fi
                
                repos_to_clone+=("$name|$url|$commit|$lang_dir|$lang")
                ((total_repos++))
            else
                log_warn "Could not load repository info for: $lang/$type"
            fi
        done
    done
    
    if [[ $total_repos -eq 0 ]]; then
        log_warn "No repositories to clone based on current filters"
        return 0
    fi
    
    log_info "Found $total_repos repositories to clone"
    
    # Clone repositories with concurrency control
    local success_count=0
    local failure_count=0
    local active_jobs=()
    local job_id=0
    
    for repo_spec in "${repos_to_clone[@]}"; do
        IFS='|' read -r name url commit target_dir lang <<< "$repo_spec"
        
        # Wait for available slot
        while [[ ${#active_jobs[@]} -ge $MAX_CONCURRENT ]]; do
            local new_active_jobs=()
            for job_pid in "${active_jobs[@]}"; do
                if kill -0 "$job_pid" 2>/dev/null; then
                    new_active_jobs+=("$job_pid")
                else
                    wait "$job_pid"
                    local job_status=$?
                    if [[ $job_status -eq 0 ]]; then
                        ((success_count++))
                    else
                        ((failure_count++))
                    fi
                fi
            done
            active_jobs=("${new_active_jobs[@]}")
            
            if [[ ${#active_jobs[@]} -ge $MAX_CONCURRENT ]]; then
                sleep 1
            fi
        done
        
        # Start clone job in background
        {
            clone_repository "$name" "$url" "$commit" "$target_dir" "$lang"
        } &
        
        local bg_pid=$!
        active_jobs+=("$bg_pid")
        ((job_id++))
        
        log_debug "Started clone job $job_id (PID: $bg_pid): $name"
    done
    
    # Wait for all remaining jobs
    log_info "Waiting for remaining clone operations to complete..."
    
    for job_pid in "${active_jobs[@]}"; do
        if wait "$job_pid"; then
            ((success_count++))
        else
            ((failure_count++))
        fi
    done
    
    # Final summary
    log_info "Repository cloning completed"
    log_info "Successfully cloned: $success_count repositories"
    
    if [[ $failure_count -gt 0 ]]; then
        log_error "Failed to clone: $failure_count repositories"
        return 1
    else
        log_success "All repositories cloned successfully!"
        return 0
    fi
}

# Cleanup operations
cleanup_repositories() {
    local target="${1:-all}"
    
    log_info "Starting cleanup operation: $target"
    
    if [[ ! -d "$BASE_DIR" ]]; then
        log_warn "Base directory does not exist: $BASE_DIR"
        return 0
    fi
    
    case "$target" in
        "all")
            log_warn "This will remove ALL test repositories in $BASE_DIR"
            if [[ "$FORCE_RECLONE" != true ]]; then
                read -p "Are you sure? (y/N): " -r
                if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                    log_info "Cleanup cancelled"
                    return 0
                fi
            fi
            
            if [[ "$DRY_RUN" == true ]]; then
                log_info "[DRY-RUN] Would remove: $BASE_DIR"
            else
                # Remove only the language directories, not the scripts
                for lang_dir in "$BASE_DIR"/{go,python,typescript,java}; do
                    if [[ -d "$lang_dir" ]]; then
                        rm -rf "$lang_dir"
                    fi
                done
                log_success "All repositories cleaned up"
            fi
            ;;
        
        *)
            # Clean specific language or repository
            local cleanup_path
            if [[ -d "$BASE_DIR/$target" ]]; then
                cleanup_path="$BASE_DIR/$target"
            elif [[ "$target" == *"/"* ]]; then
                IFS='/' read -r lang name <<< "$target"
                cleanup_path="$BASE_DIR/$lang/$name"
            else
                log_error "Unknown cleanup target: $target"
                return 1
            fi
            
            if [[ ! -d "$cleanup_path" ]]; then
                log_warn "Cleanup target does not exist: $cleanup_path"
                return 0
            fi
            
            log_warn "This will remove: $cleanup_path"
            if [[ "$FORCE_RECLONE" != true && "$DRY_RUN" != true ]]; then
                read -p "Are you sure? (y/N): " -r
                if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                    log_info "Cleanup cancelled"
                    return 0
                fi
            fi
            
            if [[ "$DRY_RUN" == true ]]; then
                log_info "[DRY-RUN] Would remove: $cleanup_path"
            else
                rm -rf "$cleanup_path"
                log_success "Cleaned up: $cleanup_path"
            fi
            ;;
    esac
}

# Status reporting
show_status() {
    log_info "Repository Status Report"
    echo
    
    if [[ ! -d "$BASE_DIR" ]]; then
        log_warn "Base directory does not exist: $BASE_DIR"
        return 0
    fi
    
    local languages=("go" "python" "typescript" "java")
    local types=("primary" "alternative")
    
    printf "%-12s %-12s %-20s %-12s %-12s %-15s\n" "LANGUAGE" "TYPE" "NAME" "STATUS" "COMMIT" "SIZE"
    printf "%-12s %-12s %-20s %-12s %-12s %-15s\n" "--------" "----" "----" "------" "------" "----"
    
    local total_repos=0
    local cloned_repos=0
    local total_size_mb=0
    
    for lang in "${languages[@]}"; do
        for type in "${types[@]}"; do
            local repo_info
            if repo_info=$(get_repository_info "$lang" "$type"); then
                IFS='|' read -r name url commit <<< "$repo_info"
                ((total_repos++))
                
                local repo_dir="$BASE_DIR/$lang/$name"
                local status="NOT CLONED"
                local actual_commit="N/A"
                local size_str="N/A"
                
                if [[ -d "$repo_dir" ]]; then
                    if is_git_repo "$repo_dir"; then
                        if validate_repository "$repo_dir" "$lang" "$commit" "$url" >/dev/null 2>&1; then
                            status="VALID"
                            ((cloned_repos++))
                        else
                            status="INVALID"
                        fi
                        
                        actual_commit=$(get_git_commit "$repo_dir")
                        actual_commit="${actual_commit:0:8}"  # Show short hash
                        
                        # Get size
                        local size_kb
                        size_kb=$(du -sk "$repo_dir" 2>/dev/null | cut -f1 || echo "0")
                        local size_mb=$((size_kb / 1024))
                        total_size_mb=$((total_size_mb + size_mb))
                        size_str="${size_mb}MB"
                    else
                        status="NOT GIT REPO"
                    fi
                fi
                
                printf "%-12s %-12s %-20s %-12s %-12s %-15s\n" "$lang" "$type" "$name" "$status" "$actual_commit" "$size_str"
            fi
        done
    done
    
    echo
    log_info "Summary: $cloned_repos/$total_repos repositories cloned (Total size: ${total_size_mb}MB)"
}

# Validation of all repositories
validate_all_repositories() {
    log_info "Validating all repositories..."
    
    if [[ ! -d "$BASE_DIR" ]]; then
        log_error "Base directory does not exist: $BASE_DIR"
        return 1
    fi
    
    local languages=("go" "python" "typescript" "java")
    local types=("primary" "alternative")
    
    local total_repos=0
    local valid_repos=0
    local invalid_repos=0
    
    for lang in "${languages[@]}"; do
        for type in "${types[@]}"; do
            local repo_info
            if repo_info=$(get_repository_info "$lang" "$type"); then
                IFS='|' read -r name url commit <<< "$repo_info"
                ((total_repos++))
                
                local repo_dir="$BASE_DIR/$lang/$name"
                
                if [[ ! -d "$repo_dir" ]]; then
                    log_warn "Repository not cloned: $lang/$name"
                    ((invalid_repos++))
                    continue
                fi
                
                if validate_repository "$repo_dir" "$lang" "$commit" "$url"; then
                    log_success "Valid: $lang/$name"
                    ((valid_repos++))
                else
                    log_error "Invalid: $lang/$name"
                    ((invalid_repos++))
                fi
            fi
        done
    done
    
    log_info "Validation complete: $valid_repos/$total_repos repositories valid"
    
    if [[ $invalid_repos -gt 0 ]]; then
        return 1
    fi
    
    return 0
}

# Help and usage
show_usage() {
    cat << EOF
$SCRIPT_NAME v$SCRIPT_VERSION - LSP Gateway Test Repository Management

USAGE:
    $SCRIPT_NAME [COMMAND] [OPTIONS]

COMMANDS:
    clone       Clone repositories (default)
    list        List available repositories
    status      Show status of cloned repositories
    validate    Validate all cloned repositories
    cleanup     Clean up repositories
    help        Show this help message

OPTIONS:
    -c, --config FILE       Configuration file (default: $DEFAULT_CONFIG_FILE)
    -d, --dir DIR          Base directory for repositories (default: $DEFAULT_BASE_DIR)
    -l, --languages LANGS  Comma-separated list of languages to include
    -r, --repos REPOS      Comma-separated list of repositories to include
    -x, --exclude-langs    Comma-separated list of languages to exclude
    -X, --exclude-repos    Comma-separated list of repositories to exclude
    -j, --jobs N           Maximum concurrent clones (default: $DEFAULT_MAX_CONCURRENT)
    -t, --timeout MINS     Clone timeout in minutes (default: $DEFAULT_TIMEOUT_MINUTES)
    -f, --force            Force re-clone existing repositories
    -s, --shallow          Use shallow clone (--depth=1)
    -S, --no-shallow       Use full clone
    -v, --verify           Verify commit hashes (default: true)
    -V, --no-verify        Skip commit verification
    --cleanup-on-failure   Clean up on clone failure (default: true)
    --no-cleanup-on-failure  Keep failed clones for debugging
    --dry-run              Show what would be done without doing it
    -q, --quiet            Quiet output (errors only)
    --verbose              Verbose output (debug level)
    --log-level LEVEL      Set log level (ERROR, WARN, INFO, DEBUG)
    -h, --help             Show this help message

EXAMPLES:
    # Clone all repositories
    $SCRIPT_NAME

    # Clone only Go repositories
    $SCRIPT_NAME --languages go

    # Clone specific repositories
    $SCRIPT_NAME --repos kubernetes,django

    # Force re-clone with full history
    $SCRIPT_NAME --force --no-shallow

    # Dry run to see what would be cloned
    $SCRIPT_NAME --dry-run --verbose

    # List available repositories
    $SCRIPT_NAME list

    # Show status of cloned repositories
    $SCRIPT_NAME status

    # Clean up all repositories
    $SCRIPT_NAME cleanup all

    # Clean up specific language
    $SCRIPT_NAME cleanup go

    # Validate all repositories
    $SCRIPT_NAME validate

CONFIGURATION:
    The script reads repository configuration from a YAML file.
    See $DEFAULT_CONFIG_FILE for the expected format.

EXIT CODES:
    0    Success
    1    General error
    2    Configuration error
    3    Clone/validation failure
    4    Missing dependencies

For more information, see the project documentation.
EOF
}

# Command line parsing
parse_arguments() {
    local command=""
    
    while [[ $# -gt 0 ]]; do
        case "$1" in
            clone|list|status|validate|cleanup|help)
                command="$1"
                shift
                ;;
            
            -c|--config)
                CONFIG_FILE="$2"
                shift 2
                ;;
            
            -d|--dir)
                BASE_DIR="$2"
                shift 2
                ;;
            
            -l|--languages)
                IFS=',' read -ra FILTER_LANGUAGES <<< "$2"
                shift 2
                ;;
            
            -r|--repos)
                IFS=',' read -ra FILTER_REPOS <<< "$2"
                shift 2
                ;;
            
            -x|--exclude-langs)
                IFS=',' read -ra EXCLUDE_LANGUAGES <<< "$2"
                shift 2
                ;;
            
            -X|--exclude-repos)
                IFS=',' read -ra EXCLUDE_REPOS <<< "$2"
                shift 2
                ;;
            
            -j|--jobs)
                MAX_CONCURRENT="$2"
                shift 2
                ;;
            
            -t|--timeout)
                TIMEOUT_MINUTES="$2"
                shift 2
                ;;
            
            -f|--force)
                FORCE_RECLONE=true
                shift
                ;;
            
            -s|--shallow)
                SHALLOW_CLONE=true
                shift
                ;;
            
            -S|--no-shallow)
                SHALLOW_CLONE=false
                shift
                ;;
            
            -v|--verify)
                VERIFY_COMMITS=true
                shift
                ;;
            
            -V|--no-verify)
                VERIFY_COMMITS=false
                shift
                ;;
            
            --cleanup-on-failure)
                CLEANUP_ON_FAILURE=true
                shift
                ;;
            
            --no-cleanup-on-failure)
                CLEANUP_ON_FAILURE=false
                shift
                ;;
            
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            
            -q|--quiet)
                QUIET=true
                shift
                ;;
            
            --verbose)
                LOG_LEVEL="DEBUG"
                VERBOSE=true
                shift
                ;;
            
            --log-level)
                LOG_LEVEL="$2"
                shift 2
                ;;
            
            -h|--help)
                show_usage
                exit 0
                ;;
            
            -*)
                log_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
            
            *)
                if [[ -z "$command" ]]; then
                    command="$1"
                else
                    log_error "Unexpected argument: $1"
                    show_usage
                    exit 1
                fi
                shift
                ;;
        esac
    done
    
    # Set default command
    if [[ -z "$command" ]]; then
        command="clone"
    fi
    
    echo "$command"
}

# Dependency checking
check_dependencies() {
    local missing_deps=()
    
    # Required commands
    if ! command_exists git; then
        missing_deps+=("git")
    fi
    
    if ! command_exists awk; then
        missing_deps+=("awk")
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

# Main function
main() {
    local command
    command=$(parse_arguments "$@")
    
    # Show header (unless quiet)
    if [[ "$QUIET" != true ]]; then
        echo -e "${WHITE}$SCRIPT_NAME v$SCRIPT_VERSION - LSP Gateway Test Repository Management${NC}"
        echo
    fi
    
    # Check dependencies first
    if ! check_dependencies; then
        exit 4
    fi
    
    # Load configuration (except for help and list commands that don't need it)
    if [[ "$command" != "help" ]]; then
        if ! load_config; then
            exit 2
        fi
    fi
    
    # Execute command
    case "$command" in
        "clone")
            if ! clone_repositories; then
                exit 3
            fi
            ;;
        
        "list")
            if ! load_config; then
                exit 2
            fi
            list_repositories
            ;;
        
        "status")
            show_status
            ;;
        
        "validate")
            if ! validate_all_repositories; then
                exit 3
            fi
            ;;
        
        "cleanup")
            # Get cleanup target from remaining arguments
            local cleanup_target="all"
            if [[ $# -gt 1 ]]; then
                cleanup_target="$2"
            fi
            cleanup_repositories "$cleanup_target"
            ;;
        
        "help")
            show_usage
            ;;
        
        *)
            log_error "Unknown command: $command"
            show_usage
            exit 1
            ;;
    esac
    
    # Final success message
    if [[ "$QUIET" != true && "$command" != "help" && "$command" != "list" ]]; then
        echo
        log_success "Operation completed successfully!"
    fi
}

# Signal handling
cleanup_on_signal() {
    log_error "Received signal, cleaning up..."
    
    # Kill all background jobs
    local jobs=($(jobs -p))
    if [[ ${#jobs[@]} -gt 0 ]]; then
        log_debug "Killing ${#jobs[@]} background jobs..."
        kill "${jobs[@]}" 2>/dev/null || true
        wait 2>/dev/null || true
    fi
    
    exit 130
}

trap cleanup_on_signal INT TERM

# Execute main function if script is run directly (not sourced)
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi