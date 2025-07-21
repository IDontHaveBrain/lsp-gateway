#!/bin/bash

# cleanup-repos.sh - Clean up cloned test repositories
# Usage: ./cleanup-repos.sh [language] [confirm]
# language: go, python, typescript, java, or all (default: all)
# confirm: yes to skip confirmation (default: prompt user)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BASE_DIR="$(dirname "$SCRIPT_DIR")"

LANGUAGE="${1:-all}"
CONFIRM="${2:-prompt}"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

cleanup_dir() {
    local target_dir="$1"
    local repo_name="$2"
    
    if [[ ! -d "$target_dir" ]]; then
        log "Directory $target_dir does not exist, skipping"
        return
    fi
    
    local size=$(du -sh "$target_dir" 2>/dev/null | cut -f1 || echo "unknown")
    log "Removing $repo_name ($size): $target_dir"
    rm -rf "$target_dir"
}

cleanup_go_repos() {
    log "Cleaning up Go repositories..."
    cleanup_dir "$BASE_DIR/go/kubernetes" "kubernetes"
    cleanup_dir "$BASE_DIR/go/docker" "docker"
    cleanup_dir "$BASE_DIR/go/golang" "golang"
    cleanup_dir "$BASE_DIR/go/prometheus" "prometheus"
}

cleanup_python_repos() {
    log "Cleaning up Python repositories..."
    cleanup_dir "$BASE_DIR/python/django" "django"
    cleanup_dir "$BASE_DIR/python/flask" "flask"
    cleanup_dir "$BASE_DIR/python/numpy" "numpy"
    cleanup_dir "$BASE_DIR/python/pandas" "pandas"
}

cleanup_typescript_repos() {
    log "Cleaning up TypeScript repositories..."
    cleanup_dir "$BASE_DIR/typescript/vscode" "vscode"
    cleanup_dir "$BASE_DIR/typescript/angular" "angular"
    cleanup_dir "$BASE_DIR/typescript/react" "react"
    cleanup_dir "$BASE_DIR/typescript/typescript" "typescript"
}

cleanup_java_repos() {
    log "Cleaning up Java repositories..."
    cleanup_dir "$BASE_DIR/java/spring-boot" "spring-boot"
    cleanup_dir "$BASE_DIR/java/elasticsearch" "elasticsearch"
    cleanup_dir "$BASE_DIR/java/kafka" "kafka"
    cleanup_dir "$BASE_DIR/java/maven" "maven"
}

confirm_cleanup() {
    if [[ "$CONFIRM" == "yes" ]]; then
        return 0
    fi
    
    echo
    echo "This will permanently delete cloned repositories for: $LANGUAGE"
    echo "This action cannot be undone."
    echo
    read -p "Are you sure you want to continue? (y/N): " -n 1 -r
    echo
    
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log "Cleanup cancelled by user"
        exit 0
    fi
}

main() {
    log "Starting repository cleanup process"
    log "Language: $LANGUAGE"
    
    confirm_cleanup
    
    case "$LANGUAGE" in
        "go")
            cleanup_go_repos
            ;;
        "python")
            cleanup_python_repos
            ;;
        "typescript")
            cleanup_typescript_repos
            ;;
        "java")
            cleanup_java_repos
            ;;
        "all")
            cleanup_go_repos
            cleanup_python_repos
            cleanup_typescript_repos
            cleanup_java_repos
            ;;
        *)
            log "ERROR: Unknown language '$LANGUAGE'"
            log "Supported languages: go, python, typescript, java, all"
            exit 1
            ;;
    esac
    
    log "Repository cleanup completed"
}

main "$@"