#!/bin/bash

# Simple LSP Test Repository Cloning Script
# Works with simple-test-repositories.yaml for single repository per language

set -euo pipefail

# Script configuration
readonly SCRIPT_NAME="$(basename "$0")"
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
readonly CONFIG_FILE="$PROJECT_ROOT/simple-test-repositories.yaml"
readonly BASE_DIR="$PROJECT_ROOT/test-repositories"

# Colors for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly WHITE='\033[1;37m'
readonly NC='\033[0m'

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${WHITE} $*${NC}" >&2
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${WHITE} $*${NC}" >&2
}

log_warn() {
    echo -e "${YELLOW}[WARN]${WHITE} $*${NC}" >&2
}

log_error() {
    echo -e "${RED}[ERROR]${WHITE} $*${NC}" >&2
}

# Check if required commands exist
check_dependencies() {
    local missing_deps=()
    
    if ! command -v git >/dev/null 2>&1; then
        missing_deps+=("git")
    fi
    
    if ! command -v yq >/dev/null 2>&1; then
        # If yq is not available, we'll use a simple parser
        log_warn "yq not found, using simple YAML parser"
    fi
    
    if [[ ${#missing_deps[@]} -gt 0 ]]; then
        log_error "Missing required dependencies: ${missing_deps[*]}"
        return 1
    fi
    
    return 0
}

# Simple YAML parser for our specific format
get_repo_value() {
    local language="$1"
    local key="$2"
    
    # Use yq if available, otherwise use grep/awk
    if command -v yq >/dev/null 2>&1; then
        yq eval ".repositories.$language.$key" "$CONFIG_FILE" 2>/dev/null || echo "null"
    else
        # Simple grep-based parser
        awk -v lang="$language" -v key="$key" '
        BEGIN { 
            in_lang = 0
            found = 0
        }
        /^repositories:/ { next }
        $0 ~ "^  " lang ":$" { 
            in_lang = 1
            next 
        }
        /^  [a-z]/ && in_lang { in_lang = 0 }
        in_lang && $0 ~ "^    " key ":" {
            sub(/^    [^:]+: */, "")
            gsub(/^"|"$/, "")
            print
            found = 1
            exit
        }
        END { if (!found) print "null" }
        ' "$CONFIG_FILE"
    fi
}

# Clone a single repository
clone_repository() {
    local language="$1"
    
    log_info "Processing $language repository..."
    
    # Extract repository information
    local name=$(get_repo_value "$language" "name")
    local url=$(get_repo_value "$language" "url")
    local commit=$(get_repo_value "$language" "commit")
    local commit_hash=$(get_repo_value "$language" "commit_hash")
    
    if [[ "$name" == "null" || "$url" == "null" || "$commit" == "null" ]]; then
        log_error "Missing configuration for $language repository"
        return 1
    fi
    
    local repo_dir="$BASE_DIR/$language/$name"
    
    log_info "Cloning $name to $repo_dir"
    log_info "  URL: $url"
    log_info "  Commit: $commit"
    
    # Create language directory
    mkdir -p "$BASE_DIR/$language"
    
    # Check if repository already exists
    if [[ -d "$repo_dir" ]]; then
        if [[ -d "$repo_dir/.git" ]]; then
            log_info "Repository already exists, checking status..."
            
            # Check if it's the correct repository
            local current_url=$(git -C "$repo_dir" config --get remote.origin.url 2>/dev/null || echo "")
            if [[ "$current_url" == "$url" ]]; then
                log_success "Repository already cloned: $name"
                return 0
            else
                log_warn "Repository exists but URL mismatch, removing..."
                rm -rf "$repo_dir"
            fi
        else
            log_warn "Directory exists but is not a git repository, removing..."
            rm -rf "$repo_dir"
        fi
    fi
    
    # Clone the repository
    if git clone --depth 1 "$url" "$repo_dir"; then
        log_success "Successfully cloned: $name"
        
        # Checkout specific commit if different from HEAD
        if [[ -n "$commit_hash" && "$commit_hash" != "null" ]]; then
            log_info "Checking out specific commit: $commit"
            
            # For shallow clones, we might need to fetch more history
            if ! git -C "$repo_dir" checkout "$commit" 2>/dev/null; then
                log_info "Shallow clone doesn't contain commit, fetching more history..."
                git -C "$repo_dir" fetch --depth=50
                if git -C "$repo_dir" checkout "$commit"; then
                    log_success "Successfully checked out: $commit"
                else
                    log_error "Failed to checkout commit: $commit"
                    return 1
                fi
            fi
        fi
        
        # Get repository size
        local size_kb=$(du -sk "$repo_dir" 2>/dev/null | cut -f1 || echo "0")
        local size_mb=$((size_kb / 1024))
        log_success "Repository ready: $name (${size_mb}MB)"
        
        return 0
    else
        log_error "Failed to clone: $name"
        return 1
    fi
}

# Setup repositories for testing
setup_repositories() {
    local languages=("$@")
    
    # If no languages specified, use all
    if [[ ${#languages[@]} -eq 0 ]]; then
        languages=("go" "python" "typescript" "java")
    fi
    
    log_info "Setting up test repositories for: ${languages[*]}"
    
    # Create base directory
    mkdir -p "$BASE_DIR"
    
    local success_count=0
    local total_count=${#languages[@]}
    
    for lang in "${languages[@]}"; do
        if clone_repository "$lang"; then
            ((success_count++))
        else
            log_error "Failed to setup $lang repository"
        fi
    done
    
    log_info "Repository setup complete: $success_count/$total_count successful"
    
    if [[ $success_count -eq $total_count ]]; then
        log_success "All repositories set up successfully!"
        return 0
    else
        log_error "Some repositories failed to set up"
        return 1
    fi
}

# Show repository status
show_status() {
    log_info "Repository Status:"
    echo
    
    printf "%-12s %-20s %-12s %-50s\n" "LANGUAGE" "NAME" "STATUS" "PATH"
    printf "%-12s %-20s %-12s %-50s\n" "--------" "----" "------" "----"
    
    local languages=("go" "python" "typescript" "java")
    
    for lang in "${languages[@]}"; do
        local name=$(get_repo_value "$lang" "name")
        local repo_dir="$BASE_DIR/$lang/$name"
        
        if [[ "$name" == "null" ]]; then
            printf "%-12s %-20s %-12s %-50s\n" "$lang" "NOT CONFIGURED" "ERROR" "N/A"
            continue
        fi
        
        if [[ -d "$repo_dir" && -d "$repo_dir/.git" ]]; then
            printf "%-12s %-20s %-12s %-50s\n" "$lang" "$name" "CLONED" "$repo_dir"
        else
            printf "%-12s %-20s %-12s %-50s\n" "$lang" "$name" "NOT CLONED" "$repo_dir"
        fi
    done
    echo
}

# Clean up repositories
cleanup_repositories() {
    local target="${1:-}"
    
    if [[ -z "$target" ]]; then
        log_warn "This will remove ALL test repositories in $BASE_DIR"
        read -p "Are you sure? (y/N): " -r
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_info "Cleanup cancelled"
            return 0
        fi
        rm -rf "$BASE_DIR"
        log_success "All repositories cleaned up"
    elif [[ "$target" == "go" || "$target" == "python" || "$target" == "typescript" || "$target" == "java" ]]; then
        local lang_dir="$BASE_DIR/$target"
        if [[ -d "$lang_dir" ]]; then
            log_warn "This will remove $target repositories in $lang_dir"
            read -p "Are you sure? (y/N): " -r
            if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                log_info "Cleanup cancelled"
                return 0
            fi
            rm -rf "$lang_dir"
            log_success "Cleaned up $target repositories"
        else
            log_warn "No $target repositories found"
        fi
    else
        log_error "Unknown cleanup target: $target"
        return 1
    fi
}

# Show usage
show_usage() {
    cat << EOF
$SCRIPT_NAME - Simple LSP Test Repository Management

USAGE:
    $SCRIPT_NAME [COMMAND] [LANGUAGES...]

COMMANDS:
    setup [LANGUAGES...]    Clone repositories for specified languages (default: all)
    status                  Show status of repositories
    cleanup [LANGUAGE]      Clean up repositories (all or specific language)
    help                    Show this help

LANGUAGES:
    go, python, typescript, java

EXAMPLES:
    $SCRIPT_NAME                        # Setup all repositories
    $SCRIPT_NAME setup go python        # Setup only Go and Python repositories
    $SCRIPT_NAME status                  # Show repository status
    $SCRIPT_NAME cleanup                 # Remove all repositories
    $SCRIPT_NAME cleanup go              # Remove only Go repositories

CONFIGURATION:
    Uses $CONFIG_FILE

EOF
}

# Main function
main() {
    local command="${1:-setup}"
    
    case "$command" in
        setup)
            shift
            if ! check_dependencies; then
                exit 1
            fi
            
            if [[ ! -f "$CONFIG_FILE" ]]; then
                log_error "Configuration file not found: $CONFIG_FILE"
                exit 1
            fi
            
            setup_repositories "$@"
            ;;
        
        status)
            if [[ ! -f "$CONFIG_FILE" ]]; then
                log_error "Configuration file not found: $CONFIG_FILE"
                exit 1
            fi
            show_status
            ;;
        
        cleanup)
            cleanup_repositories "${2:-}"
            ;;
        
        help|--help|-h)
            show_usage
            ;;
        
        *)
            log_error "Unknown command: $command"
            show_usage
            exit 1
            ;;
    esac
}

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi