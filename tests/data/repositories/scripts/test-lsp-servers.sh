#!/bin/bash

# test-lsp-servers.sh - Test LSP servers against cloned repositories
# Usage: ./test-lsp-servers.sh [language] [test-type]
# language: go, python, typescript, java, or all (default: all)
# test-type: basic, full (default: basic)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BASE_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(dirname "$BASE_DIR")"

LANGUAGE="${1:-all}"
TEST_TYPE="${2:-basic}"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

test_lsp_methods() {
    local repo_dir="$1"
    local language="$2"
    local repo_name="$3"
    
    if [[ ! -d "$repo_dir" ]]; then
        log "WARNING: Repository $repo_dir not found, skipping"
        return
    fi
    
    log "Testing LSP methods for $language/$repo_name"
    
    # Basic LSP method tests using LSP Gateway
    local gateway_cmd="$PROJECT_ROOT/bin/lsp-gateway"
    
    if [[ ! -x "$gateway_cmd" ]]; then
        log "ERROR: LSP Gateway binary not found at $gateway_cmd"
        log "Run 'make local' from project root to build it"
        return 1
    fi
    
    # Test basic LSP methods if files exist
    local test_files=()
    case "$language" in
        "go")
            readarray -t test_files < <(find "$repo_dir" -name "*.go" -not -path "*/vendor/*" | head -5)
            ;;
        "python")
            readarray -t test_files < <(find "$repo_dir" -name "*.py" -not -path "*/venv/*" -not -path "*/.venv/*" | head -5)
            ;;
        "typescript")
            readarray -t test_files < <(find "$repo_dir" -name "*.ts" -not -path "*/node_modules/*" | head -5)
            ;;
        "java")
            readarray -t test_files < <(find "$repo_dir" -name "*.java" -not -path "*/target/*" | head -5)
            ;;
    esac
    
    if [[ ${#test_files[@]} -eq 0 ]]; then
        log "No test files found for $language in $repo_name"
        return
    fi
    
    log "Found ${#test_files[@]} test files for $repo_name"
    
    # Test document symbols on first file
    if [[ ${#test_files[@]} -gt 0 ]]; then
        local test_file="${test_files[0]}"
        log "Testing document symbols on $(basename "$test_file")"
        
        # This would require LSP Gateway to be running
        # For now, just log what we would test
        log "Would test: textDocument/documentSymbol on $test_file"
        log "Would test: textDocument/hover on $test_file"
        if [[ "$TEST_TYPE" == "full" ]]; then
            log "Would test: textDocument/definition on $test_file"
            log "Would test: textDocument/references on $test_file"
            log "Would test: workspace/symbol on $repo_dir"
        fi
    fi
}

test_go_repos() {
    log "Testing Go repositories..."
    test_lsp_methods "$BASE_DIR/go/kubernetes" "go" "kubernetes"
    test_lsp_methods "$BASE_DIR/go/docker" "go" "docker"
    test_lsp_methods "$BASE_DIR/go/golang" "go" "golang"
    test_lsp_methods "$BASE_DIR/go/prometheus" "go" "prometheus"
}

test_python_repos() {
    log "Testing Python repositories..."
    test_lsp_methods "$BASE_DIR/python/django" "python" "django"
    test_lsp_methods "$BASE_DIR/python/flask" "python" "flask"
    test_lsp_methods "$BASE_DIR/python/numpy" "python" "numpy"
    test_lsp_methods "$BASE_DIR/python/pandas" "python" "pandas"
}

test_typescript_repos() {
    log "Testing TypeScript repositories..."
    test_lsp_methods "$BASE_DIR/typescript/vscode" "typescript" "vscode"
    test_lsp_methods "$BASE_DIR/typescript/angular" "typescript" "angular"
    test_lsp_methods "$BASE_DIR/typescript/react" "typescript" "react"
    test_lsp_methods "$BASE_DIR/typescript/typescript" "typescript" "typescript"
}

test_java_repos() {
    log "Testing Java repositories..."
    test_lsp_methods "$BASE_DIR/java/spring-boot" "java" "spring-boot"
    test_lsp_methods "$BASE_DIR/java/elasticsearch" "java" "elasticsearch"
    test_lsp_methods "$BASE_DIR/java/kafka" "java" "kafka"
    test_lsp_methods "$BASE_DIR/java/maven" "java" "maven"
}

main() {
    log "Starting LSP server testing process"
    log "Language: $LANGUAGE, Test type: $TEST_TYPE"
    
    case "$LANGUAGE" in
        "go")
            test_go_repos
            ;;
        "python")
            test_python_repos
            ;;
        "typescript")
            test_typescript_repos
            ;;
        "java")
            test_java_repos
            ;;
        "all")
            test_go_repos
            test_python_repos
            test_typescript_repos
            test_java_repos
            ;;
        *)
            log "ERROR: Unknown language '$LANGUAGE'"
            log "Supported languages: go, python, typescript, java, all"
            exit 1
            ;;
    esac
    
    log "LSP server testing completed"
}

main "$@"