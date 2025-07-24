#!/bin/bash

# LSP Gateway Comprehensive Environment Setup
# Prepares complete environment for LSP server validation and repository testing
# Integrates with LSP Gateway's internal setup infrastructure

set -euo pipefail

# Script metadata
readonly SCRIPT_VERSION="1.0.0"
readonly SCRIPT_NAME="LSP Gateway Environment Setup"
readonly MIN_GO_VERSION="1.24"
readonly MIN_PYTHON_VERSION="3.8"
readonly MIN_NODE_VERSION="18.0"
readonly MIN_JAVA_VERSION="17"

# Configuration
readonly SETUP_LOG="./env-setup.log"
readonly TEST_CONFIG_DIR="./tests/data/configs"
readonly TEST_DATA_DIR="./tests/data/samples"
readonly REPO_CACHE_DIR="./tests/data/repositories"
readonly LSP_CONFIG_DIR="./lsp-configs"
readonly HEALTH_CHECK_DIR="./health-checks"

# Environment variables (not readonly to allow command line overrides)
PARALLEL_OPERATIONS="${PARALLEL_OPERATIONS:-true}"
CLEANUP_ON_FAILURE="${CLEANUP_ON_FAILURE:-true}"
VERBOSE_OUTPUT="${VERBOSE_OUTPUT:-false}"

# Colors and formatting
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly CYAN='\033[0;36m'
readonly PURPLE='\033[0;35m'
readonly BOLD='\033[1m'
readonly NC='\033[0m'

# Logging and output functions
log() {
    echo -e "${BLUE}[$(date +'%H:%M:%S')]${NC} $1" | tee -a "$SETUP_LOG"
}

log_success() {
    echo -e "${GREEN}[$(date +'%H:%M:%S')] ✓${NC} $1" | tee -a "$SETUP_LOG"
}

log_warning() {
    echo -e "${YELLOW}[$(date +'%H:%M:%S')] ⚠${NC} $1" | tee -a "$SETUP_LOG"
}

log_error() {
    echo -e "${RED}[$(date +'%H:%M:%S')] ✗${NC} $1" | tee -a "$SETUP_LOG"
}

log_info() {
    echo -e "${CYAN}[$(date +'%H:%M:%S')] ℹ${NC} $1" | tee -a "$SETUP_LOG"
}

log_step() {
    echo -e "${PURPLE}[$(date +'%H:%M:%S')] ${BOLD}▶${NC} $1" | tee -a "$SETUP_LOG"
}

log_verbose() {
    if [[ "$VERBOSE_OUTPUT" == "true" ]]; then
        echo -e "${CYAN}[DEBUG]${NC} $1" | tee -a "$SETUP_LOG"
    fi
}

# Error handling and cleanup
cleanup_on_failure() {
    if [[ "$CLEANUP_ON_FAILURE" == "true" ]]; then
        log_warning "Cleaning up partial setup due to failure..."
        
        # Remove partially created directories
        local cleanup_dirs=("$REPO_CACHE_DIR" "$LSP_CONFIG_DIR" "$HEALTH_CHECK_DIR")
        for dir in "${cleanup_dirs[@]}"; do
            if [[ -d "$dir" && ! -f "$dir/.setup_complete" ]]; then
                rm -rf "$dir" 2>/dev/null || true
                log_verbose "Removed partial directory: $dir"
            fi
        done
    fi
}

trap 'cleanup_on_failure' ERR

# Initialize logging
initialize_logging() {
    local log_header="$SCRIPT_NAME v$SCRIPT_VERSION - $(date)"
    echo "$log_header" > "$SETUP_LOG"
    echo "========================================" >> "$SETUP_LOG"
    echo "Environment Setup Started: $(date)" >> "$SETUP_LOG"
    echo "Working Directory: $(pwd)" >> "$SETUP_LOG"
    echo "User: $(whoami)" >> "$SETUP_LOG"
    echo "Platform: $(uname -s) $(uname -m)" >> "$SETUP_LOG"
    echo "Shell: $SHELL" >> "$SETUP_LOG"
    echo "PATH: $PATH" >> "$SETUP_LOG"
    echo "========================================" >> "$SETUP_LOG"
    
    log_step "$SCRIPT_NAME v$SCRIPT_VERSION"
    log "Environment setup started in $(pwd)"
}

# Version comparison utility
version_gte() {
    local version1=$1
    local version2=$2
    
    # Handle empty versions
    [[ -z "$version1" || -z "$version2" ]] && return 1
    
    # Extract numeric parts (handle versions like "go1.21.0")
    local v1=$(echo "$version1" | sed 's/[^0-9.]//g')
    local v2=$(echo "$version2" | sed 's/[^0-9.]//g')
    
    # Compare versions using sort -V
    [[ "$(printf '%s\n' "$v2" "$v1" | sort -V | head -n1)" == "$v2" ]]
}

# System dependency validation
validate_system_dependencies() {
    log_step "Validating system dependencies..."
    
    local required_tools=("make" "curl" "git" "bc" "which")
    local optional_tools=("wget" "jq" "docker" "docker-compose")
    local missing_required=()
    local missing_optional=()
    
    # Check required tools
    for tool in "${required_tools[@]}"; do
        if command -v "$tool" &> /dev/null; then
            log_success "$tool is available"
        else
            missing_required+=("$tool")
            log_error "$tool is required but not installed"
        fi
    done
    
    # Check optional tools
    for tool in "${optional_tools[@]}"; do
        if command -v "$tool" &> /dev/null; then
            log_success "$tool is available (optional)"
        else
            missing_optional+=("$tool")
            log_warning "$tool not found (optional)"
        fi
    done
    
    # Fail if required tools are missing
    if [[ ${#missing_required[@]} -gt 0 ]]; then
        log_error "Missing required tools: ${missing_required[*]}"
        log_error "Please install required tools before continuing"
        return 1
    fi
    
    # Check system resources
    validate_system_resources
    
    log_success "System dependencies validated"
}

validate_system_resources() {
    log_info "Checking system resources..."
    
    # Memory check
    local available_memory
    if command -v free &> /dev/null; then
        available_memory=$(free -m | awk '/^Mem:/ {print $2}')
        if [[ $available_memory -lt 2048 ]]; then
            log_warning "Limited memory available: ${available_memory}MB (recommended: 2GB+)"
        else
            log_success "Sufficient memory available: ${available_memory}MB"
        fi
    fi
    
    # CPU cores check
    local cpu_cores=$(nproc 2>/dev/null || echo "1")
    if [[ $cpu_cores -lt 2 ]]; then
        log_warning "Limited CPU cores: $cpu_cores (recommended: 2+)"
    else
        log_success "Sufficient CPU cores: $cpu_cores"
    fi
    
    # Disk space check
    local available_space_gb
    if command -v df &> /dev/null; then
        available_space_gb=$(df . | awk 'NR==2 {print int($4/1024/1024)}')
        if [[ $available_space_gb -lt 2 ]]; then
            log_warning "Limited disk space: ${available_space_gb}GB (recommended: 2GB+)"
        else
            log_success "Sufficient disk space: ${available_space_gb}GB"
        fi
    fi
}

# Runtime environment validation
validate_runtime_environments() {
    log_step "Validating runtime environments..."
    
    local validation_results=()
    
    # Validate Go
    if validate_go_environment; then
        validation_results+=("Go: ✓")
    else
        validation_results+=("Go: ✗")
    fi
    
    # Validate Python
    if validate_python_environment; then
        validation_results+=("Python: ✓")
    else
        validation_results+=("Python: ✗")
    fi
    
    # Validate Node.js
    if validate_nodejs_environment; then
        validation_results+=("Node.js: ✓")
    else
        validation_results+=("Node.js: ✗")
    fi
    
    # Validate Java (optional)
    if validate_java_environment; then
        validation_results+=("Java: ✓")
    else
        validation_results+=("Java: ⚠")
    fi
    
    log_info "Runtime validation summary:"
    for result in "${validation_results[@]}"; do
        log "  $result"
    done
    
    log_success "Runtime environments validated"
}

validate_go_environment() {
    log_info "Validating Go environment..."
    
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed"
        log_info "Please install Go $MIN_GO_VERSION+ from https://golang.org/dl/"
        return 1
    fi
    
    local go_version
    go_version=$(go version 2>/dev/null | grep -oE 'go[0-9]+\.[0-9]+(\.[0-9]+)?' | sed 's/go//')
    
    if ! version_gte "$go_version" "$MIN_GO_VERSION"; then
        log_error "Go $MIN_GO_VERSION+ required, found Go $go_version"
        return 1
    fi
    
    # Check Go environment variables
    local gopath=$(go env GOPATH 2>/dev/null || echo "")
    local goroot=$(go env GOROOT 2>/dev/null || echo "")
    local go111module=$(go env GO111MODULE 2>/dev/null || echo "")
    
    log_success "Go $go_version detected"
    log_verbose "GOPATH: $gopath"
    log_verbose "GOROOT: $goroot"
    log_verbose "GO111MODULE: $go111module"
    
    # Test Go compilation
    local temp_go_file=$(mktemp --suffix=.go)
    cat > "$temp_go_file" << 'EOF'
package main
import "fmt"
func main() { fmt.Println("Go compilation test") }
EOF
    
    if go build -o /tmp/go-test "$temp_go_file" &>/dev/null; then
        log_success "Go compilation test passed"
        rm -f /tmp/go-test "$temp_go_file"
    else
        log_warning "Go compilation test failed"
        rm -f "$temp_go_file"
        return 1
    fi
    
    return 0
}

validate_python_environment() {
    log_info "Validating Python environment..."
    
    local python_cmd=""
    local python_version=""
    
    # Try python3 first, then python
    for cmd in python3 python; do
        if command -v "$cmd" &> /dev/null; then
            python_version=$($cmd --version 2>&1 | grep -oE '[0-9]+\.[0-9]+(\.[0-9]+)?')
            if version_gte "$python_version" "$MIN_PYTHON_VERSION"; then
                python_cmd=$cmd
                break
            fi
        fi
    done
    
    if [[ -z "$python_cmd" ]]; then
        log_warning "Python $MIN_PYTHON_VERSION+ not found"
        log_info "Python is optional but recommended for complete LSP testing"
        return 1
    fi
    
    log_success "Python $python_version detected ($python_cmd)"
    
    # Check pip availability
    local pip_cmd=""
    for cmd in pip3 pip; do
        if command -v "$cmd" &> /dev/null; then
            pip_cmd=$cmd
            break
        fi
    done
    
    if [[ -n "$pip_cmd" ]]; then
        log_success "Pip available ($pip_cmd)"
    else
        log_warning "Pip not found - LSP server installation may be limited"
    fi
    
    return 0
}

validate_nodejs_environment() {
    log_info "Validating Node.js environment..."
    
    if ! command -v node &> /dev/null; then
        log_warning "Node.js not found"
        log_info "Node.js is optional but recommended for JavaScript/TypeScript LSP testing"
        return 1
    fi
    
    local node_version
    node_version=$(node --version 2>/dev/null | sed 's/v//')
    
    if ! version_gte "$node_version" "$MIN_NODE_VERSION"; then
        log_warning "Node.js $MIN_NODE_VERSION+ recommended, found $node_version"
        return 1
    fi
    
    log_success "Node.js $node_version detected"
    
    # Check npm availability
    if command -v npm &> /dev/null; then
        local npm_version
        npm_version=$(npm --version 2>/dev/null || echo "unknown")
        log_success "npm $npm_version available"
    else
        log_warning "npm not found - TypeScript LSP installation may be limited"
        return 1
    fi
    
    return 0
}

validate_java_environment() {
    log_info "Validating Java environment..."
    
    if ! command -v java &> /dev/null; then
        log_warning "Java not found"
        log_info "Java is optional - only needed for Java LSP testing"
        return 1
    fi
    
    local java_version
    java_version=$(java -version 2>&1 | head -n1 | grep -oE '[0-9]+(\.[0-9]+)*' | head -n1)
    
    if ! version_gte "$java_version" "$MIN_JAVA_VERSION"; then
        log_warning "Java $MIN_JAVA_VERSION+ recommended, found $java_version"
        return 1
    fi
    
    log_success "Java $java_version detected"
    return 0
}

# Repository cloning and setup using existing clone-repos.sh script
setup_test_repositories() {
    log_step "Setting up test repositories..."
    
    local clone_script="./scripts/clone-repos.sh"
    
    # Check if clone-repos.sh script exists
    if [[ ! -f "$clone_script" ]]; then
        log_error "Repository cloning script not found: $clone_script"
        log_info "Falling back to basic repository setup..."
        setup_test_repositories_fallback
        return $?
    fi
    
    # Check if configuration file exists
    local config_file="./test-repositories.yaml"
    if [[ ! -f "$config_file" ]]; then
        log_warning "Repository configuration file not found: $config_file"
        log_info "Creating default configuration file..."
        create_default_repository_config
    fi
    
    # Use existing clone-repos.sh script with appropriate options
    log_info "Using existing repository cloning script: $clone_script"
    log_info "Repository cache directory: $REPO_CACHE_DIR"
    
    local clone_args=()
    
    # Configure clone script arguments
    clone_args+=("clone")
    clone_args+=("--dir" "$REPO_CACHE_DIR")
    clone_args+=("--config" "$config_file")
    
    if [[ "$VERBOSE_OUTPUT" == "true" ]]; then
        clone_args+=("--verbose")
    else
        clone_args+=("--log-level" "INFO")
    fi
    
    # Set appropriate concurrency for setup
    clone_args+=("--jobs" "2")
    clone_args+=("--timeout" "10")
    clone_args+=("--shallow")
    clone_args+=("--cleanup-on-failure")
    
    log_verbose "Executing: $clone_script ${clone_args[*]}"
    
    # Execute the repository cloning script
    if "$clone_script" "${clone_args[@]}"; then
        log_success "Repository cloning completed successfully"
        
        # Mark setup complete
        touch "$REPO_CACHE_DIR/.setup_complete"
        
        # Show status summary
        log_info "Repository setup summary:"
        "$clone_script" status --dir "$REPO_CACHE_DIR" --config "$config_file" | tail -n 5
        
        return 0
    else
        local exit_code=$?
        log_error "Repository cloning failed (exit code: $exit_code)"
        
        # Try to show what went wrong
        if [[ -f "$clone_script" ]]; then
            "$clone_script" status --dir "$REPO_CACHE_DIR" --config "$config_file" 2>/dev/null || true
        fi
        
        return 1
    fi
}

# Fallback repository setup for when clone-repos.sh is not available
setup_test_repositories_fallback() {
    log_info "Setting up test repositories using fallback method..."
    
    mkdir -p "$REPO_CACHE_DIR"
    cd "$REPO_CACHE_DIR"
    
    # Define basic test repositories for different languages
    local repos=(
        "https://github.com/golang/example.git:go-example:go:--depth=1"
        "https://github.com/psf/requests.git:python-example:python:--depth=1"
        "https://github.com/microsoft/TypeScript-Node-Starter.git:typescript-example:typescript:--depth=1"
        "https://github.com/spring-projects/spring-boot.git:java-example:java:--depth=1"
    )
    
    local cloned_repos=()
    local failed_repos=()
    
    for repo_spec in "${repos[@]}"; do
        IFS=':' read -r repo_url dir_name language clone_options <<< "$repo_spec"
        
        if clone_test_repository_fallback "$repo_url" "$dir_name" "$language" "$clone_options"; then
            cloned_repos+=("$dir_name ($language)")
        else
            failed_repos+=("$dir_name ($language)")
        fi
    done
    
    cd - > /dev/null
    
    log_info "Repository setup summary:"
    log "  Cloned: ${#cloned_repos[@]}"
    log "  Failed: ${#failed_repos[@]}"
    
    for repo in "${cloned_repos[@]}"; do
        log_success "  ✓ $repo"
    done
    
    for repo in "${failed_repos[@]}"; do
        log_warning "  ✗ $repo"
    done
    
    # Mark setup complete
    touch "$REPO_CACHE_DIR/.setup_complete"
    
    if [[ ${#failed_repos[@]} -eq 0 ]]; then
        log_success "Test repositories setup completed successfully"
        return 0
    else
        log_warning "Test repositories setup completed with some failures"
        return 1
    fi
}

clone_test_repository_fallback() {
    local repo_url=$1
    local dir_name=$2
    local language=$3
    local clone_options=${4:-"--depth=1"}
    
    log_info "Cloning $language repository: $dir_name"
    
    # Skip if already exists and is valid
    if [[ -d "$dir_name" && -d "$dir_name/.git" ]]; then
        log_success "$dir_name already exists and appears valid"
        return 0
    fi
    
    # Clone with timeout protection
    local clone_cmd="git clone $clone_options $repo_url $dir_name"
    log_verbose "Executing: $clone_cmd"
    
    if timeout 300 $clone_cmd &>/dev/null; then
        log_success "Successfully cloned $dir_name"
        
        # Create a marker file with metadata
        cat > "$dir_name/.lsp_test_metadata" << EOF
language=$language
cloned_at=$(date -Iseconds)
repo_url=$repo_url
setup_script=$0
cloned_by=fallback_method
EOF
        return 0
    else
        log_error "Failed to clone $dir_name (timeout or error)"
        # Clean up partial clone
        rm -rf "$dir_name" 2>/dev/null || true
        return 1
    fi
}

# Create default repository configuration if it doesn't exist
create_default_repository_config() {
    log_info "Creating default repository configuration..."
    
    local config_file="./test-repositories.yaml"
    
    cat > "$config_file" << 'EOF'
# LSP Gateway Test Repository Configuration
# This file defines repositories used for testing different language servers

# Global configuration
config:
  base_directory: "./test-repositories"
  max_concurrent_clones: 3
  clone_timeout_minutes: 10
  shallow_clone: true
  verify_commits: true
  cleanup_on_failure: true

# Test repositories for each supported language
repositories:
  go:
    primary:
      name: "go-example"
      url: "https://github.com/golang/example.git"
      commit: "master"
      description: "Official Go example repository"
      
    alternative:
      name: "kubernetes"
      url: "https://github.com/kubernetes/kubernetes.git"  
      commit: "master"
      description: "Kubernetes main repository (large Go project)"

  python:
    primary:
      name: "python-requests"
      url: "https://github.com/psf/requests.git"
      commit: "main"
      description: "Popular Python HTTP library"
      
    alternative:
      name: "django"
      url: "https://github.com/django/django.git"
      commit: "main"
      description: "Django web framework"

  typescript:
    primary:
      name: "typescript-starter"
      url: "https://github.com/microsoft/TypeScript-Node-Starter.git"
      commit: "master"
      description: "TypeScript Node.js starter project"
      
    alternative:
      name: "vscode"
      url: "https://github.com/microsoft/vscode.git"
      commit: "main"
      description: "Visual Studio Code (large TypeScript project)"

  java:
    primary:
      name: "spring-boot"
      url: "https://github.com/spring-projects/spring-boot.git"
      commit: "main"
      description: "Spring Boot framework"
      
    alternative:
      name: "elasticsearch"
      url: "https://github.com/elastic/elasticsearch.git"
      commit: "main"
      description: "Elasticsearch search engine"

# Validation rules for cloned repositories
validation:
  required_files:
    go: ["go.mod", "*.go"]
    python: ["setup.py", "pyproject.toml", "requirements.txt", "*.py"]
    typescript: ["package.json", "tsconfig.json", "*.ts"]
    java: ["pom.xml", "build.gradle", "*.java"]
    
  minimum_files:
    go: 5
    python: 10
    typescript: 5
    java: 10
EOF
    
    log_success "Created default repository configuration: $config_file"
}

# LSP server installation and setup
setup_lsp_servers() {
    log_step "Setting up LSP servers..."
    
    local server_setup_results=()
    
    # Setup Go LSP server
    if setup_go_lsp_server; then
        server_setup_results+=("gopls: ✓")
    else
        server_setup_results+=("gopls: ✗")
    fi
    
    # Setup Python LSP server
    if setup_python_lsp_server; then
        server_setup_results+=("pylsp: ✓")
    else
        server_setup_results+=("pylsp: ✗")
    fi
    
    # Setup TypeScript LSP server
    if setup_typescript_lsp_server; then
        server_setup_results+=("typescript-language-server: ✓")
    else
        server_setup_results+=("typescript-language-server: ✗")
    fi
    
    # Setup Java LSP server (optional)
    if setup_java_lsp_server; then
        server_setup_results+=("jdtls: ✓")
    else
        server_setup_results+=("jdtls: ⚠")
    fi
    
    log_info "LSP server setup summary:"
    for result in "${server_setup_results[@]}"; do
        log "  $result"
    done
    
    log_success "LSP server setup completed"
}

setup_go_lsp_server() {
    log_info "Setting up Go LSP server (gopls)..."
    
    if command -v gopls &> /dev/null; then
        local gopls_version
        gopls_version=$(gopls version 2>/dev/null | head -n1 || echo "unknown")
        log_success "gopls already installed: $gopls_version"
        return 0
    fi
    
    if ! command -v go &> /dev/null; then
        log_warning "Go not available, skipping gopls installation"
        return 1
    fi
    
    log_info "Installing gopls via go install..."
    if timeout 120 go install golang.org/x/tools/gopls@latest; then
        log_success "gopls installed successfully"
        
        # Verify installation
        if command -v gopls &> /dev/null; then
            local version
            version=$(gopls version 2>/dev/null | head -n1 || echo "unknown")
            log_success "gopls verification passed: $version"
            return 0
        fi
    fi
    
    log_error "Failed to install or verify gopls"
    return 1
}

setup_python_lsp_server() {
    log_info "Setting up Python LSP server (python-lsp-server)..."
    
    if command -v pylsp &> /dev/null; then
        local pylsp_version
        pylsp_version=$(pylsp --version 2>/dev/null | head -n1 || echo "unknown")
        log_success "python-lsp-server already installed: $pylsp_version"
        return 0
    fi
    
    # Find appropriate pip command
    local pip_cmd=""
    for cmd in pip3 pip; do
        if command -v "$cmd" &> /dev/null; then
            pip_cmd=$cmd
            break
        fi
    done
    
    if [[ -z "$pip_cmd" ]]; then
        log_warning "pip not available, skipping python-lsp-server installation"
        return 1
    fi
    
    log_info "Installing python-lsp-server via $pip_cmd..."
    if timeout 120 $pip_cmd install python-lsp-server; then
        log_success "python-lsp-server installed successfully"
        
        # Verify installation
        if command -v pylsp &> /dev/null; then
            local version
            version=$(pylsp --version 2>/dev/null | head -n1 || echo "unknown")
            log_success "pylsp verification passed: $version"
            return 0
        fi
    fi
    
    log_error "Failed to install or verify python-lsp-server"
    return 1
}

setup_typescript_lsp_server() {
    log_info "Setting up TypeScript LSP server..."
    
    if command -v typescript-language-server &> /dev/null; then
        local ts_version
        ts_version=$(typescript-language-server --version 2>/dev/null || echo "unknown")
        log_success "typescript-language-server already installed: $ts_version"
        return 0
    fi
    
    if ! command -v npm &> /dev/null; then
        log_warning "npm not available, skipping typescript-language-server installation"
        return 1
    fi
    
    log_info "Installing typescript-language-server via npm..."
    if timeout 180 npm install -g typescript-language-server typescript; then
        log_success "typescript-language-server installed successfully"
        
        # Verify installation
        if command -v typescript-language-server &> /dev/null; then
            local version
            version=$(typescript-language-server --version 2>/dev/null || echo "unknown")
            log_success "typescript-language-server verification passed: $version"
            return 0
        fi
    fi
    
    log_error "Failed to install or verify typescript-language-server"
    return 1
}

setup_java_lsp_server() {
    log_info "Setting up Java LSP server (jdtls)..."
    
    if ! command -v java &> /dev/null; then
        log_warning "Java not available, skipping jdtls setup"
        return 1
    fi
    
    # Java LSP (jdtls) requires manual setup - just verify Java is available
    log_warning "Java LSP server (jdtls) requires manual installation"
    log_info "Download from: https://download.eclipse.org/jdtls/snapshots/"
    log_info "Java runtime is available for manual jdtls setup"
    
    return 1  # Not an error, but indicates manual setup required
}

# Configuration file generation
create_configuration_files() {
    log_step "Creating configuration files..."
    
    create_test_configurations
    create_lsp_configurations
    create_health_check_configurations
    
    log_success "Configuration files created"
}

create_test_configurations() {
    log_info "Creating test configurations..."
    
    mkdir -p "$TEST_CONFIG_DIR"
    
    # Enhanced integration test configuration
    cat > "$TEST_CONFIG_DIR/integration-test-config.yaml" << EOF
# LSP Gateway Integration Test Configuration
# Generated by $0 on $(date)

port: 0  # Dynamic port allocation for tests
timeout: 30s
max_connections: 100
log_level: info

servers:
  # Go Language Server for testing
  - name: "test-go-lsp"
    languages: ["go"]
    command: "gopls"
    args: []
    transport: "stdio"
    working_directory: "$REPO_CACHE_DIR/go-example"
    
  # Python Language Server for testing  
  - name: "test-python-lsp"
    languages: ["python"]
    command: "python"
    args: ["-m", "pylsp"]
    transport: "stdio"
    working_directory: "$REPO_CACHE_DIR/python-example"
    
  # TypeScript Language Server for testing
  - name: "test-typescript-lsp"
    languages: ["typescript", "javascript"]
    command: "typescript-language-server"
    args: ["--stdio"]
    transport: "stdio"
    working_directory: "$REPO_CACHE_DIR/typescript-example"

# Integration test specific settings
integration_test:
  mock_servers: true
  timeout_multiplier: 2
  concurrent_clients: 5
  test_data_dir: "$TEST_DATA_DIR"
  repository_cache_dir: "$REPO_CACHE_DIR"
  health_check_enabled: true
  
# Test execution settings
execution:
  parallel_tests: true
  retry_on_failure: true
  max_retries: 3
  cleanup_after_test: false
  
# Logging configuration
logging:
  level: debug
  file: "./integration-test-results/test.log"
  console: true
  
# Environment validation
validation:
  runtime_check: true
  server_availability_check: true
  repository_validation: true
EOF

    # Performance test configuration
    cat > "$TEST_CONFIG_DIR/performance-test-config.yaml" << EOF
# LSP Gateway Performance Test Configuration
# Generated by $0 on $(date)

port: 0  # Dynamic port allocation for tests
timeout: 10s  # Shorter timeout for performance tests
max_connections: 200

servers:
  # High-performance mock servers for testing
  - name: "perf-mock-lsp"
    languages: ["go", "python", "typescript", "javascript"]
    command: "mock_lsp_server"  # Embedded mock server
    args: ["--performance-mode"]
    transport: "stdio"

# Performance test specific settings
performance_test:
  target_latency_ms: 50
  target_throughput_rps: 100
  target_memory_mb: 100
  concurrent_clients: 20
  requests_per_test: 1000
  benchmark_time: "60s"
  warmup_time: "10s"
  
# Load testing configuration  
load_test:
  ramp_up_time: "30s"
  steady_state_time: "120s"
  ramp_down_time: "30s"
  max_concurrent_requests: 500
  
# Resource monitoring
monitoring:
  cpu_threshold_percent: 80
  memory_threshold_mb: 500
  disk_io_threshold_mbps: 100
EOF


    log_success "Test configurations created in $TEST_CONFIG_DIR/"
}

create_lsp_configurations() {
    log_info "Creating LSP server configurations..."
    
    mkdir -p "$LSP_CONFIG_DIR"
    
    # Go LSP configuration
    cat > "$LSP_CONFIG_DIR/gopls.yaml" << EOF
# Go Language Server Configuration
# Generated by $0 on $(date)

server:
  name: "gopls"
  display_name: "Go Language Server"
  command: "gopls"
  args: []
  transport: "stdio"
  languages: ["go"]
  
settings:
  go.buildFlags: []
  go.env: {}
  go.formatTool: "goimports"
  go.lintTool: "golangci-lint"
  go.vetOnSave: "package"
  
initialization_options:
  usePlaceholders: true
  completionDocumentation: true
  deepCompletion: true
  
working_directory: "$REPO_CACHE_DIR/go-example"
timeout: 30s
max_memory_mb: 512

health_check:
  enabled: true
  interval: 30s
  timeout: 5s
EOF

    # Python LSP configuration
    cat > "$LSP_CONFIG_DIR/pylsp.yaml" << EOF
# Python Language Server Configuration
# Generated by $0 on $(date)

server:
  name: "pylsp"
  display_name: "Python Language Server"
  command: "python"
  args: ["-m", "pylsp"]
  transport: "stdio"
  languages: ["python"]
  
settings:
  pylsp:
    plugins:
      pycodestyle:
        enabled: true
        maxLineLength: 88
      pydocstyle:
        enabled: false
      pyflakes:
        enabled: true
      pylint:
        enabled: false
      rope_completion:
        enabled: true
        
working_directory: "$REPO_CACHE_DIR/python-example"
timeout: 30s
max_memory_mb: 256

health_check:
  enabled: true
  interval: 30s
  timeout: 5s
EOF

    # TypeScript LSP configuration
    cat > "$LSP_CONFIG_DIR/typescript-language-server.yaml" << EOF
# TypeScript Language Server Configuration
# Generated by $0 on $(date)

server:
  name: "typescript-language-server"
  display_name: "TypeScript Language Server"
  command: "typescript-language-server"
  args: ["--stdio"]
  transport: "stdio"
  languages: ["typescript", "javascript"]
  
settings:
  typescript:
    suggest:
      completeFunctionCalls: true
      includeCompletionsForModuleExports: true
    preferences:
      includePackageJsonAutoImports: "auto"
      
working_directory: "$REPO_CACHE_DIR/typescript-example"
timeout: 30s
max_memory_mb: 512

health_check:
  enabled: true
  interval: 30s
  timeout: 5s
EOF

    log_success "LSP configurations created in $LSP_CONFIG_DIR/"
}

create_health_check_configurations() {
    log_info "Creating health check configurations..."
    
    mkdir -p "$HEALTH_CHECK_DIR"
    
    # Health check test suite configuration
    cat > "$HEALTH_CHECK_DIR/health-check-config.yaml" << EOF
# LSP Gateway Health Check Configuration
# Generated by $0 on $(date)

health_checks:
  - name: "system_health"
    type: "system"
    interval: 60s
    timeout: 10s
    checks:
      - memory_usage
      - cpu_usage
      - disk_space
      - file_descriptors
      
  - name: "runtime_health"
    type: "runtime"  
    interval: 120s
    timeout: 30s
    checks:
      - go_availability
      - python_availability
      - nodejs_availability
      - java_availability
      
  - name: "lsp_server_health"
    type: "lsp_servers"
    interval: 180s
    timeout: 45s
    checks:
      - gopls_responsive
      - pylsp_responsive
      - typescript_language_server_responsive
      
  - name: "repository_health"
    type: "repositories"
    interval: 300s
    timeout: 60s
    checks:
      - repository_integrity
      - git_status
      - file_permissions

thresholds:
  memory_usage_percent: 80
  cpu_usage_percent: 70
  disk_usage_percent: 90
  file_descriptor_usage_percent: 75
  
notifications:
  enabled: true
  log_file: "$HEALTH_CHECK_DIR/health-check.log"
  alert_on_failure: true
  
recovery:
  auto_restart_lsp_servers: true
  cleanup_temp_files: true
  max_restart_attempts: 3
EOF

    # Create health check scripts
    create_health_check_scripts
    
    log_success "Health check configurations created in $HEALTH_CHECK_DIR/"
}

create_health_check_scripts() {
    # System health check script
    cat > "$HEALTH_CHECK_DIR/system-health-check.sh" << 'EOF'
#!/bin/bash
# System Health Check Script

check_memory() {
    local usage=$(free | awk '/^Mem:/ {printf "%.1f", $3/$2 * 100}')
    echo "Memory usage: ${usage}%"
    return $(echo "$usage < 80" | bc -l)
}

check_cpu() {
    local usage=$(top -bn1 | grep "Cpu(s)" | awk '{print $2}' | cut -d'%' -f1)
    echo "CPU usage: ${usage}%"
    return $(echo "$usage < 70" | bc -l)
}

check_disk() {
    local usage=$(df . | awk 'NR==2 {print $5}' | cut -d'%' -f1)
    echo "Disk usage: ${usage}%"
    return $(test "$usage" -lt 90)
}

main() {
    echo "System Health Check - $(date)"
    
    local checks=(check_memory check_cpu check_disk)
    local failed_checks=()
    
    for check in "${checks[@]}"; do
        if ! $check; then
            failed_checks+=("$check")
        fi
    done
    
    if [[ ${#failed_checks[@]} -eq 0 ]]; then
        echo "All system health checks passed"
        exit 0
    else
        echo "Failed health checks: ${failed_checks[*]}"
        exit 1
    fi
}

main "$@"
EOF

    # LSP server health check script
    cat > "$HEALTH_CHECK_DIR/lsp-health-check.sh" << 'EOF'
#!/bin/bash
# LSP Server Health Check Script

check_gopls() {
    if command -v gopls &>/dev/null; then
        if timeout 5 gopls version &>/dev/null; then
            echo "gopls: healthy"
            return 0
        else
            echo "gopls: unresponsive"
            return 1
        fi
    else
        echo "gopls: not installed"
        return 1
    fi
}

check_pylsp() {
    if command -v pylsp &>/dev/null; then
        if timeout 5 pylsp --version &>/dev/null; then
            echo "pylsp: healthy"
            return 0
        else
            echo "pylsp: unresponsive"
            return 1
        fi
    else
        echo "pylsp: not installed"
        return 1
    fi
}

check_typescript_language_server() {
    if command -v typescript-language-server &>/dev/null; then
        if timeout 5 typescript-language-server --version &>/dev/null; then
            echo "typescript-language-server: healthy"
            return 0
        else
            echo "typescript-language-server: unresponsive"
            return 1
        fi
    else
        echo "typescript-language-server: not installed"
        return 1
    fi
}

main() {
    echo "LSP Server Health Check - $(date)"
    
    local checks=(check_gopls check_pylsp check_typescript_language_server)
    local failed_checks=()
    
    for check in "${checks[@]}"; do
        if ! $check; then
            failed_checks+=("$check")
        fi
    done
    
    if [[ ${#failed_checks[@]} -eq 0 ]]; then
        echo "All LSP server health checks passed"
        exit 0
    else
        echo "Failed LSP server checks: ${failed_checks[*]}"
        exit 1
    fi
}

main "$@"
EOF

    chmod +x "$HEALTH_CHECK_DIR/system-health-check.sh"
    chmod +x "$HEALTH_CHECK_DIR/lsp-health-check.sh"
}

# Enhanced test data creation
create_enhanced_test_data() {
    log_step "Creating enhanced test data..."
    
    mkdir -p "$TEST_DATA_DIR"
    
    create_sample_go_project
    create_sample_python_project
    create_sample_typescript_project
    create_sample_javascript_project
    create_project_configurations
    
    log_success "Enhanced test data created in $TEST_DATA_DIR/"
}

create_sample_go_project() {
    log_info "Creating sample Go project..."
    
    # Main Go file with comprehensive examples
    cat > "$TEST_DATA_DIR/sample.go" << 'EOF'
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// TestStruct demonstrates a struct for LSP testing
type TestStruct struct {
	Field1 string    `json:"field1"`
	Field2 int       `json:"field2"`
	Field3 time.Time `json:"field3"`
}

// TestInterface demonstrates an interface for LSP testing
type TestInterface interface {
	Method() string
	Process(input string) (string, error)
}

// TestFunction demonstrates a function for LSP testing
func TestFunction(input string) string {
	return fmt.Sprintf("Processed: %s", input)
}

// Method demonstrates a method for LSP testing
func (ts *TestStruct) Method() string {
	return fmt.Sprintf("%s (%d) at %s", ts.Field1, ts.Field2, ts.Field3.Format(time.RFC3339))
}

// Process implements TestInterface
func (ts *TestStruct) Process(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("empty input not allowed")
	}
	return fmt.Sprintf("%s -> %s", input, ts.Method()), nil
}

// HTTPHandler demonstrates an HTTP handler for LSP testing
func HTTPHandler(w http.ResponseWriter, r *http.Request) {
	test := &TestStruct{
		Field1: "HTTP Handler",
		Field2: 200,
		Field3: time.Now(),
	}
	
	result, err := test.Process(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	fmt.Fprintf(w, "Response: %s", result)
}

// ContextFunction demonstrates context usage for LSP testing
func ContextFunction(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	select {
	case <-time.After(timeout / 2):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func main() {
	test := &TestStruct{
		Field1: "Hello World",
		Field2: 42,
		Field3: time.Now(),
	}
	
	// Test basic functionality
	result := TestFunction("LSP Gateway Test")
	fmt.Println(result)
	fmt.Println(test.Method())
	
	// Test interface implementation
	var iface TestInterface = test
	processed, err := iface.Process("test input")
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("Processed: %s\n", processed)
	}
	
	// Test context function
	ctx := context.Background()
	if err := ContextFunction(ctx, 1*time.Second); err != nil {
		log.Printf("Context function error: %v", err)
	}
	
	// Test command line arguments
	if len(os.Args) > 1 {
		fmt.Printf("Arguments: %v\n", os.Args[1:])
	}
	
	// Optional HTTP server for testing
	if len(os.Args) > 1 && os.Args[1] == "server" {
		fmt.Println("Starting HTTP server on :8080")
		http.HandleFunc("/", HTTPHandler)
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatal(err)
		}
	}
}
EOF

    # Go module file
    cat > "$TEST_DATA_DIR/go.mod" << 'EOF'
module lsp-gateway-test-data

go 1.24

require (
    golang.org/x/sync v0.3.0
    golang.org/x/time v0.3.0
)
EOF

    # Go sum file (placeholder)
    cat > "$TEST_DATA_DIR/go.sum" << 'EOF'
golang.org/x/sync v0.3.0 h1:ftCYgMx6zT/asHUrPw8BLLscYtGznsLAnjq5RH9P66E=
golang.org/x/sync v0.3.0/go.mod h1:FU7BRWz2tNW+3quACPkgCx/L+uEAv1htQ0V83Z9Rj+Y=
golang.org/x/time v0.3.0 h1:rg5rLMjNzMS1RkNLzCG38eapWhnYLFYXDXj2gOlr8j4=
golang.org/x/time v0.3.0/go.mod h1:tRJNPiyCQ0inRvYxbN9jk5I+vvW/OXSQhTDSoE431IQ=
EOF
}

create_sample_python_project() {
    log_info "Creating sample Python project..."
    
    # Main Python file with comprehensive examples
    cat > "$TEST_DATA_DIR/sample.py" << 'EOF'
"""
Sample Python file for LSP Gateway integration testing.
Provides comprehensive examples for Python LSP functionality.
"""

import asyncio
import json
import os
import sys
import time
from abc import ABC, abstractmethod
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Dict, List, Optional, Protocol, Union


@dataclass
class TestDataClass:
    """Test dataclass for LSP integration testing."""
    field1: str
    field2: int
    field3: datetime
    
    def to_dict(self) -> Dict[str, Union[str, int]]:
        """Convert to dictionary representation."""
        return {
            'field1': self.field1,
            'field2': self.field2,
            'field3': self.field3.isoformat()
        }


class TestProtocol(Protocol):
    """Test protocol for LSP integration testing."""
    
    def method(self) -> str:
        """Protocol method definition."""
        ...
        
    def process(self, input_data: str) -> str:
        """Protocol process method definition."""
        ...


class TestAbstractClass(ABC):
    """Test abstract class for LSP integration testing."""
    
    @abstractmethod
    def abstract_method(self) -> str:
        """Abstract method that must be implemented."""
        pass
    
    def concrete_method(self) -> str:
        """Concrete method with default implementation."""
        return "concrete implementation"


class TestClass(TestAbstractClass):
    """Test class for LSP integration testing."""
    
    def __init__(self, field1: str, field2: int):
        self.field1 = field1
        self.field2 = field2
        self._private_field = "private"
        self.__very_private_field = "very private"
    
    @property
    def field1(self) -> str:
        """Field1 property getter."""
        return self._field1
    
    @field1.setter
    def field1(self, value: str) -> None:
        """Field1 property setter."""
        if not isinstance(value, str):
            raise TypeError("field1 must be a string")
        self._field1 = value
    
    def method(self) -> str:
        """Test method for hover and definition testing."""
        return f"Field1: {self.field1}, Field2: {self.field2}"
    
    def process(self, input_data: str) -> str:
        """Process input data with validation."""
        if not input_data:
            raise ValueError("Input data cannot be empty")
        return f"Processed: {input_data} -> {self.method()}"
    
    def abstract_method(self) -> str:
        """Implementation of abstract method."""
        return f"Abstract method implementation: {self.method()}"
    
    async def async_method(self, delay: float = 1.0) -> str:
        """Asynchronous method for testing async/await."""
        await asyncio.sleep(delay)
        return f"Async result after {delay}s: {self.method()}"
    
    def __str__(self) -> str:
        """String representation."""
        return f"TestClass({self.field1}, {self.field2})"
    
    def __repr__(self) -> str:
        """Developer representation."""
        return f"TestClass(field1={self.field1!r}, field2={self.field2!r})"


def test_function(input_data: str) -> str:
    """Test function for LSP integration testing."""
    if not isinstance(input_data, str):
        raise TypeError("Input must be a string")
    return f"Function processed: {input_data}"


def test_function_with_defaults(
    required_param: str,
    optional_param: str = "default",
    *args: str,
    **kwargs: Union[str, int]
) -> Dict[str, Union[str, List[str], Dict[str, Union[str, int]]]]:
    """Function with various parameter types for LSP testing."""
    return {
        'required': required_param,
        'optional': optional_param,
        'args': list(args),
        'kwargs': kwargs
    }


async def async_test_function(items: List[str]) -> List[str]:
    """Asynchronous function for testing async LSP features."""
    results = []
    for item in items:
        await asyncio.sleep(0.1)  # Simulate async work
        results.append(f"async_processed_{item}")
    return results


def context_manager_example():
    """Example using context managers for LSP testing."""
    with open(__file__, 'r') as f:
        lines = f.readlines()
        return len(lines)


class TestContextManager:
    """Custom context manager for LSP testing."""
    
    def __init__(self, name: str):
        self.name = name
        self.start_time: Optional[datetime] = None
    
    def __enter__(self) -> 'TestContextManager':
        self.start_time = datetime.now(timezone.utc)
        print(f"Entering context: {self.name}")
        return self
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        if self.start_time:
            duration = datetime.now(timezone.utc) - self.start_time
            print(f"Exiting context: {self.name} (duration: {duration})")


def decorator_example(func):
    """Decorator function for LSP testing."""
    def wrapper(*args, **kwargs):
        print(f"Calling function: {func.__name__}")
        start = time.time()
        result = func(*args, **kwargs)
        end = time.time()
        print(f"Function {func.__name__} took {end - start:.4f}s")
        return result
    return wrapper


@decorator_example
def decorated_function(message: str) -> str:
    """Function with decorator for LSP testing."""
    time.sleep(0.1)  # Simulate some work
    return f"Decorated result: {message}"


async def main():
    """Main async function for testing."""
    # Test basic classes and functions
    test_obj = TestClass("Hello", 42)
    result = test_function("World")
    
    print(result)
    print(test_obj.method())
    print(str(test_obj))
    print(repr(test_obj))
    
    # Test dataclass
    data_obj = TestDataClass("Test", 123, datetime.now(timezone.utc))
    print(f"DataClass: {data_obj}")
    print(f"DataClass dict: {data_obj.to_dict()}")
    
    # Test async functionality
    async_result = await test_obj.async_method(0.5)
    print(f"Async result: {async_result}")
    
    # Test async function
    items = ["item1", "item2", "item3"]
    async_items = await async_test_function(items)
    print(f"Async processed items: {async_items}")
    
    # Test function with defaults
    defaults_result = test_function_with_defaults(
        "required", "optional", "arg1", "arg2", 
        keyword1="value1", keyword2=42
    )
    print(f"Defaults result: {json.dumps(defaults_result, indent=2)}")
    
    # Test context manager
    with TestContextManager("test_context"):
        time.sleep(0.2)
        print("Inside context manager")
    
    # Test decorator
    decorated_result = decorated_function("test message")
    print(f"Decorated result: {decorated_result}")
    
    # Test command line arguments
    if len(sys.argv) > 1:
        print(f"Command line arguments: {sys.argv[1:]}")


if __name__ == "__main__":
    # Check if running in asyncio mode
    if len(sys.argv) > 1 and sys.argv[1] == "async":
        asyncio.run(main())
    else:
        # Synchronous version
        test_obj = TestClass("Hello", 42)
        result = test_function("World")
        print(result)
        print(test_obj.method())
        
        if len(sys.argv) > 1:
            print(f"Argument: {sys.argv[1]}")
EOF

    # Python requirements file
    cat > "$TEST_DATA_DIR/requirements.txt" << 'EOF'
# Requirements for LSP Gateway test data
asyncio-mqtt==0.11.1
dataclasses-json==0.5.9
python-dateutil==2.8.2
requests==2.31.0
typing-extensions==4.7.1
EOF

    # Python setup configuration
    cat > "$TEST_DATA_DIR/setup.cfg" << 'EOF'
[metadata]
name = lsp-gateway-test-data
version = 1.0.0
description = Test data for LSP Gateway Python integration testing
long_description = file: README.md
long_description_content_type = text/markdown
author = LSP Gateway Test Suite
classifiers =
    Development Status :: 4 - Beta
    Intended Audience :: Developers
    License :: OSI Approved :: MIT License
    Programming Language :: Python :: 3
    Programming Language :: Python :: 3.8
    Programming Language :: Python :: 3.9
    Programming Language :: Python :: 3.10
    Programming Language :: Python :: 3.11

[options]
packages = find:
python_requires = >=3.8
install_requires =
    asyncio-mqtt>=0.11.1
    dataclasses-json>=0.5.9
    python-dateutil>=2.8.2
    requests>=2.31.0
    typing-extensions>=4.7.1

[options.packages.find]
where = .
include = *
EOF
}

create_sample_typescript_project() {
    log_info "Creating sample TypeScript project..."
    
    # Main TypeScript file with comprehensive examples
    cat > "$TEST_DATA_DIR/sample.ts" << 'EOF'
/**
 * Sample TypeScript file for LSP Gateway integration testing
 * Provides comprehensive examples for TypeScript LSP functionality
 */

import { EventEmitter } from 'events';

// Type definitions and interfaces
interface TestInterface {
    field1: string;
    field2: number;
    field3?: Date;
}

interface TestProtocol {
    method(): string;
    process(input: string): Promise<string>;
}

// Union and intersection types
type StringOrNumber = string | number;
type TestIntersection = TestInterface & { additionalField: boolean };

// Generic interfaces
interface Repository<T> {
    findById(id: string): Promise<T | null>;
    save(entity: T): Promise<T>;
    delete(id: string): Promise<boolean>;
}

// Enum definitions
enum TestEnum {
    VALUE_ONE = "value_one",
    VALUE_TWO = "value_two",
    VALUE_THREE = "value_three"
}

enum NumericEnum {
    FIRST = 1,
    SECOND = 2,
    THIRD = 3
}

// Abstract class
abstract class AbstractTestClass {
    protected name: string;
    
    constructor(name: string) {
        this.name = name;
    }
    
    abstract abstractMethod(): string;
    
    concreteMethod(): string {
        return `Concrete implementation for ${this.name}`;
    }
}

// Implementation class
class TestClass extends AbstractTestClass implements TestInterface, TestProtocol {
    public field1: string;
    public field2: number;
    public field3?: Date;
    private _privateField: string;
    readonly readonlyField: string;
    
    // Static properties and methods
    static staticField: string = "static value";
    static instances: TestClass[] = [];
    
    constructor(field1: string, field2: number, field3?: Date) {
        super(`TestClass_${field2}`);
        this.field1 = field1;
        this.field2 = field2;
        this.field3 = field3 || new Date();
        this._privateField = "private value";
        this.readonlyField = `readonly_${field1}_${field2}`;
        
        TestClass.instances.push(this);
    }
    
    // Getter and setter
    get privateField(): string {
        return this._privateField;
    }
    
    set privateField(value: string) {
        if (value.length === 0) {
            throw new Error("Private field cannot be empty");
        }
        this._privateField = value;
    }
    
    // Method implementations
    method(): string {
        return `Field1: ${this.field1}, Field2: ${this.field2}, Field3: ${this.field3?.toISOString()}`;
    }
    
    async process(input: string): Promise<string> {
        if (!input) {
            throw new Error("Input cannot be empty");
        }
        
        // Simulate async work
        await new Promise(resolve => setTimeout(resolve, 100));
        
        return `Processed: ${input} -> ${this.method()}`;
    }
    
    abstractMethod(): string {
        return `Abstract method implementation: ${this.method()}`;
    }
    
    // Method with optional parameters and defaults
    methodWithDefaults(
        required: string,
        optional?: string,
        withDefault: string = "default",
        ...rest: string[]
    ): string {
        return `Required: ${required}, Optional: ${optional || 'undefined'}, Default: ${withDefault}, Rest: [${rest.join(', ')}]`;
    }
    
    // Generic method
    genericMethod<T>(input: T): T {
        console.log(`Generic method called with: ${input}`);
        return input;
    }
    
    // Static method
    static createInstance(field1: string, field2: number): TestClass {
        return new TestClass(field1, field2);
    }
    
    static getInstanceCount(): number {
        return TestClass.instances.length;
    }
    
    // Method overloads
    overloadedMethod(input: string): string;
    overloadedMethod(input: number): number;
    overloadedMethod(input: string | number): string | number {
        if (typeof input === 'string') {
            return `String: ${input}`;
        } else {
            return input * 2;
        }
    }
    
    toString(): string {
        return `TestClass(${this.field1}, ${this.field2})`;
    }
}

// Generic class
class GenericRepository<T> implements Repository<T> {
    private data: Map<string, T> = new Map();
    
    async findById(id: string): Promise<T | null> {
        return this.data.get(id) || null;
    }
    
    async save(entity: T): Promise<T> {
        const id = Math.random().toString(36).substr(2, 9);
        this.data.set(id, entity);
        return entity;
    }
    
    async delete(id: string): Promise<boolean> {
        return this.data.delete(id);
    }
    
    getAll(): T[] {
        return Array.from(this.data.values());
    }
}

// Function declarations and implementations
function testFunction(input: string): string {
    return `Processed: ${input}`;
}

const arrowFunction = (input: string): string => `Arrow function: ${input}`;

const asyncArrowFunction = async (items: string[]): Promise<string[]> => {
    return Promise.all(items.map(async (item) => {
        await new Promise(resolve => setTimeout(resolve, 50));
        return `async_${item}`;
    }));
};

// Higher-order functions
function higherOrderFunction<T>(
    transformer: (input: T) => T
): (input: T) => T {
    return (input: T) => {
        console.log('Higher-order function called');
        return transformer(input);
    };
}

// Decorator example
function methodDecorator(target: any, propertyKey: string, descriptor: PropertyDescriptor) {
    const originalMethod = descriptor.value;
    
    descriptor.value = function(...args: any[]) {
        console.log(`Calling method: ${propertyKey}`);
        const start = performance.now();
        const result = originalMethod.apply(this, args);
        const end = performance.now();
        console.log(`Method ${propertyKey} took ${end - start} milliseconds`);
        return result;
    };
    
    return descriptor;
}

class DecoratedClass {
    @methodDecorator
    decoratedMethod(input: string): string {
        return `Decorated: ${input}`;
    }
}

// Event emitter example
class TestEventEmitter extends EventEmitter {
    private counter: number = 0;
    
    incrementCounter(): void {
        this.counter++;
        this.emit('increment', this.counter);
    }
    
    getCounter(): number {
        return this.counter;
    }
}

// Module-level functions
export function exportedFunction(input: string): string {
    return `Exported: ${input}`;
}

export { TestClass, TestInterface, testFunction };

// Async main function
async function main(): Promise<void> {
    // Test class instantiation and methods
    const testObj = new TestClass("Hello", 42, new Date());
    console.log(testFunction("World"));
    console.log(testObj.method());
    console.log(testObj.toString());
    
    // Test async methods
    try {
        const processResult = await testObj.process("test input");
        console.log(`Process result: ${processResult}`);
    } catch (error) {
        console.error(`Process error: ${error}`);
    }
    
    // Test generic methods
    const stringResult = testObj.genericMethod<string>("generic string");
    const numberResult = testObj.genericMethod<number>(123);
    console.log(`Generic results: ${stringResult}, ${numberResult}`);
    
    // Test method overloads
    const overloadString = testObj.overloadedMethod("overload test");
    const overloadNumber = testObj.overloadedMethod(21);
    console.log(`Overload results: ${overloadString}, ${overloadNumber}`);
    
    // Test static methods
    const staticInstance = TestClass.createInstance("Static", 99);
    console.log(`Static instance: ${staticInstance.toString()}`);
    console.log(`Total instances: ${TestClass.getInstanceCount()}`);
    
    // Test generic repository
    const repository = new GenericRepository<TestInterface>();
    const testEntity: TestInterface = { field1: "Entity", field2: 456 };
    await repository.save(testEntity);
    console.log(`Repository entities: ${repository.getAll().length}`);
    
    // Test arrow functions
    console.log(arrowFunction("arrow test"));
    const asyncResults = await asyncArrowFunction(["item1", "item2", "item3"]);
    console.log(`Async arrow results: ${asyncResults.join(', ')}`);
    
    // Test higher-order function
    const transformer = higherOrderFunction<string>((input: string) => input.toUpperCase());
    const transformedResult = transformer("transform me");
    console.log(`Transformed: ${transformedResult}`);
    
    // Test decorator
    const decorated = new DecoratedClass();
    const decoratedResult = decorated.decoratedMethod("decorator test");
    console.log(`Decorated result: ${decoratedResult}`);
    
    // Test event emitter
    const eventEmitter = new TestEventEmitter();
    eventEmitter.on('increment', (count) => {
        console.log(`Counter incremented to: ${count}`);
    });
    
    eventEmitter.incrementCounter();
    eventEmitter.incrementCounter();
    
    // Test command line arguments
    if (process.argv.length > 2) {
        console.log(`Command line arguments: ${process.argv.slice(2).join(', ')}`);
    }
}

// Execute main function if this is the main module
if (require.main === module) {
    main().catch(console.error);
}
EOF

    # TypeScript configuration
    cat > "$TEST_DATA_DIR/tsconfig.json" << 'EOF'
{
    "compilerOptions": {
        "target": "ES2020",
        "lib": ["ES2020", "DOM"],
        "module": "commonjs",
        "moduleResolution": "node",
        "outDir": "./dist",
        "rootDir": "./",
        "sourceMap": true,
        "declaration": true,
        "declarationMap": true,
        "strict": true,
        "noImplicitAny": true,
        "strictNullChecks": true,
        "strictFunctionTypes": true,
        "strictBindCallApply": true,
        "strictPropertyInitialization": true,
        "noImplicitReturns": true,
        "noFallthroughCasesInSwitch": true,
        "noUncheckedIndexedAccess": true,
        "exactOptionalPropertyTypes": true,
        "noImplicitOverride": true,
        "noPropertyAccessFromIndexSignature": true,
        "allowUnusedLabels": false,
        "allowUnreachableCode": false,
        "experimentalDecorators": true,
        "emitDecoratorMetadata": true,
        "esModuleInterop": true,
        "allowSyntheticDefaultImports": true,
        "skipLibCheck": true,
        "forceConsistentCasingInFileNames": true,
        "resolveJsonModule": true,
        "isolatedModules": true,
        "incremental": true,
        "tsBuildInfoFile": "./dist/.tsbuildinfo"
    },
    "include": [
        "*.ts",
        "**/*.ts"
    ],
    "exclude": [
        "node_modules",
        "dist",
        "**/*.spec.ts",
        "**/*.test.ts"
    ],
    "ts-node": {
        "esm": true
    }
}
EOF
}

create_sample_javascript_project() {
    log_info "Creating sample JavaScript project..."
    
    # Main JavaScript file with comprehensive examples
    cat > "$TEST_DATA_DIR/sample.js" << 'EOF'
/**
 * Sample JavaScript file for LSP Gateway integration testing
 * Provides comprehensive examples for JavaScript LSP functionality
 */

const { EventEmitter } = require('events');
const fs = require('fs').promises;
const path = require('path');

// Class definitions
class TestClass {
    constructor(field1, field2, field3) {
        this.field1 = field1;
        this.field2 = field2;
        this.field3 = field3 || new Date();
        this._privateField = "private value";
        
        // Static property tracking
        TestClass.instances = TestClass.instances || [];
        TestClass.instances.push(this);
    }
    
    // Getter and setter
    get privateField() {
        return this._privateField;
    }
    
    set privateField(value) {
        if (typeof value !== 'string' || value.length === 0) {
            throw new Error("Private field must be a non-empty string");
        }
        this._privateField = value;
    }
    
    method() {
        return `Field1: ${this.field1}, Field2: ${this.field2}, Field3: ${this.field3.toISOString()}`;
    }
    
    async process(input) {
        if (!input) {
            throw new Error("Input cannot be empty");
        }
        
        // Simulate async work
        await new Promise(resolve => setTimeout(resolve, 100));
        
        return `Processed: ${input} -> ${this.method()}`;
    }
    
    // Method with default parameters
    methodWithDefaults(required, optional = 'default', ...rest) {
        return `Required: ${required}, Optional: ${optional}, Rest: [${rest.join(', ')}]`;
    }
    
    // Static methods
    static createInstance(field1, field2) {
        return new TestClass(field1, field2);
    }
    
    static getInstanceCount() {
        return TestClass.instances ? TestClass.instances.length : 0;
    }
    
    toString() {
        return `TestClass(${this.field1}, ${this.field2})`;
    }
}

// Inheritance example
class ExtendedTestClass extends TestClass {
    constructor(field1, field2, field3, extraField) {
        super(field1, field2, field3);
        this.extraField = extraField;
    }
    
    method() {
        const baseMethod = super.method();
        return `${baseMethod}, Extra: ${this.extraField}`;
    }
    
    extraMethod() {
        return `Extended functionality: ${this.extraField}`;
    }
}

// Function declarations
function testFunction(input) {
    if (typeof input !== 'string') {
        throw new TypeError("Input must be a string");
    }
    return `Function processed: ${input}`;
}

// Arrow functions
const arrowFunction = (input) => `Arrow function: ${input}`;

const asyncArrowFunction = async (items) => {
    const results = await Promise.all(items.map(async (item) => {
        await new Promise(resolve => setTimeout(resolve, 50));
        return `async_${item}`;
    }));
    return results;
};

// Higher-order functions
const higherOrderFunction = (transformer) => {
    return (input) => {
        console.log('Higher-order function called');
        return transformer(input);
    };
};

const mapFunction = (array, callback) => {
    const result = [];
    for (let i = 0; i < array.length; i++) {
        result.push(callback(array[i], i, array));
    }
    return result;
};

// Closure example
function createCounter(initialValue = 0) {
    let count = initialValue;
    
    return {
        increment: () => ++count,
        decrement: () => --count,
        getValue: () => count,
        reset: () => { count = initialValue; }
    };
}

// Prototype extension
TestClass.prototype.prototypeMethod = function() {
    return `Prototype method called on ${this.toString()}`;
};

// Object factory pattern
const objectFactory = {
    createTestObject(type, ...args) {
        switch (type) {
            case 'basic':
                return new TestClass(...args);
            case 'extended':
                return new ExtendedTestClass(...args);
            default:
                throw new Error(`Unknown type: ${type}`);
        }
    },
    
    createCounter(initial) {
        return createCounter(initial);
    }
};

// Event emitter example
class TestEventEmitter extends EventEmitter {
    constructor() {
        super();
        this.counter = 0;
    }
    
    incrementCounter() {
        this.counter++;
        this.emit('increment', this.counter);
    }
    
    decrementCounter() {
        this.counter--;
        this.emit('decrement', this.counter);
    }
    
    getCounter() {
        return this.counter;
    }
}

// Promise-based utility functions
const promiseUtilities = {
    delay: (ms) => new Promise(resolve => setTimeout(resolve, ms)),
    
    timeout: (promise, ms) => {
        return Promise.race([
            promise,
            new Promise((_, reject) => 
                setTimeout(() => reject(new Error('Timeout')), ms)
            )
        ]);
    },
    
    retry: async (fn, maxAttempts = 3, delay = 1000) => {
        let lastError;
        
        for (let attempt = 1; attempt <= maxAttempts; attempt++) {
            try {
                return await fn();
            } catch (error) {
                lastError = error;
                if (attempt < maxAttempts) {
                    await promiseUtilities.delay(delay);
                }
            }
        }
        
        throw lastError;
    }
};

// File system utilities
const fileUtilities = {
    async readJsonFile(filePath) {
        try {
            const content = await fs.readFile(filePath, 'utf8');
            return JSON.parse(content);
        } catch (error) {
            throw new Error(`Failed to read JSON file ${filePath}: ${error.message}`);
        }
    },
    
    async writeJsonFile(filePath, data, indent = 2) {
        try {
            const content = JSON.stringify(data, null, indent);
            await fs.writeFile(filePath, content, 'utf8');
        } catch (error) {
            throw new Error(`Failed to write JSON file ${filePath}: ${error.message}`);
        }
    },
    
    async fileExists(filePath) {
        try {
            await fs.access(filePath);
            return true;
        } catch {
            return false;
        }
    }
};

// Mixin pattern
const LoggerMixin = {
    log(message) {
        console.log(`[${new Date().toISOString()}] ${this.constructor.name}: ${message}`);
    },
    
    error(message) {
        console.error(`[${new Date().toISOString()}] ${this.constructor.name} ERROR: ${message}`);
    }
};

// Apply mixin to class
Object.assign(TestClass.prototype, LoggerMixin);
Object.assign(ExtendedTestClass.prototype, LoggerMixin);

// Module pattern
const modulePattern = (function() {
    let privateVariable = "private";
    
    function privateFunction() {
        return `Private function called with ${privateVariable}`;
    }
    
    return {
        publicMethod() {
            return privateFunction();
        },
        
        setPrivateVariable(value) {
            privateVariable = value;
        },
        
        getPrivateVariable() {
            return privateVariable;
        }
    };
})();

// Async main function
async function main() {
    try {
        // Test class instantiation and methods
        const testObj = new TestClass("Hello", 42, new Date());
        console.log(testFunction("World"));
        console.log(testObj.method());
        console.log(testObj.toString());
        
        // Test inheritance
        const extendedObj = new ExtendedTestClass("Extended", 99, new Date(), "extra data");
        console.log(extendedObj.method());
        console.log(extendedObj.extraMethod());
        
        // Test async methods
        const processResult = await testObj.process("test input");
        console.log(`Process result: ${processResult}`);
        
        // Test static methods
        const staticInstance = TestClass.createInstance("Static", 123);
        console.log(`Static instance: ${staticInstance.toString()}`);
        console.log(`Total instances: ${TestClass.getInstanceCount()}`);
        
        // Test arrow functions
        console.log(arrowFunction("arrow test"));
        const asyncResults = await asyncArrowFunction(["item1", "item2", "item3"]);
        console.log(`Async arrow results: ${asyncResults.join(', ')}`);
        
        // Test higher-order function
        const transformer = higherOrderFunction((input) => input.toUpperCase());
        const transformedResult = transformer("transform me");
        console.log(`Transformed: ${transformedResult}`);
        
        // Test closure
        const counter = createCounter(10);
        console.log(`Counter: ${counter.getValue()}`);
        console.log(`After increment: ${counter.increment()}`);
        console.log(`After decrement: ${counter.decrement()}`);
        
        // Test object factory
        const factoryObject = objectFactory.createTestObject('extended', "Factory", 456, new Date(), "factory extra");
        console.log(`Factory object: ${factoryObject.method()}`);
        
        // Test event emitter
        const eventEmitter = new TestEventEmitter();
        eventEmitter.on('increment', (count) => {
            console.log(`Counter incremented to: ${count}`);
        });
        
        eventEmitter.incrementCounter();
        eventEmitter.incrementCounter();
        
        // Test promise utilities
        await promiseUtilities.delay(100);
        console.log("Delay completed");
        
        // Test mixin methods
        testObj.log("Testing logger mixin");
        
        // Test module pattern
        console.log(modulePattern.publicMethod());
        modulePattern.setPrivateVariable("updated private");
        console.log(modulePattern.publicMethod());
        
        // Test command line arguments
        if (process.argv.length > 2) {
            console.log(`Command line arguments: ${process.argv.slice(2).join(', ')}`);
        }
        
    } catch (error) {
        console.error(`Error in main: ${error.message}`);
    }
}

// Module exports
module.exports = {
    TestClass,
    ExtendedTestClass,
    testFunction,
    arrowFunction,
    asyncArrowFunction,
    higherOrderFunction,
    createCounter,
    objectFactory,
    TestEventEmitter,
    promiseUtilities,
    fileUtilities,
    modulePattern
};

// Execute main function if this is the main module
if (require.main === module) {
    main().catch(console.error);
}
EOF
}

create_project_configurations() {
    log_info "Creating project configuration files..."
    
    # Enhanced package.json
    cat > "$TEST_DATA_DIR/package.json" << 'EOF'
{
    "name": "lsp-gateway-test-data",
    "version": "1.0.0",
    "description": "Comprehensive test data for LSP Gateway integration testing",
    "main": "sample.js",
    "types": "sample.d.ts",
    "scripts": {
        "start": "node sample.js",
        "start:ts": "ts-node sample.ts",
        "build": "tsc",
        "build:watch": "tsc --watch",
        "test": "jest",
        "test:watch": "jest --watch",
        "lint": "eslint *.js *.ts",
        "lint:fix": "eslint *.js *.ts --fix",
        "format": "prettier --write *.js *.ts *.json",
        "clean": "rm -rf dist node_modules/.cache"
    },
    "keywords": [
        "lsp",
        "language-server-protocol",
        "testing",
        "integration",
        "typescript",
        "javascript",
        "nodejs"
    ],
    "author": "LSP Gateway Test Suite",
    "license": "MIT",
    "engines": {
        "node": ">=18.0.0",
        "npm": ">=8.0.0"
    },
    "dependencies": {
        "events": "^3.3.0"
    },
    "devDependencies": {
        "@types/node": "^20.0.0",
        "@typescript-eslint/eslint-plugin": "^6.0.0",
        "@typescript-eslint/parser": "^6.0.0",
        "eslint": "^8.45.0",
        "eslint-config-prettier": "^8.8.0",
        "eslint-plugin-prettier": "^5.0.0",
        "jest": "^29.6.0",
        "prettier": "^3.0.0",
        "ts-node": "^10.9.0",
        "typescript": "^5.1.0"
    },
    "jest": {
        "testEnvironment": "node",
        "collectCoverage": true,
        "coverageDirectory": "./coverage",
        "coverageReporters": ["text", "lcov", "html"]
    },
    "prettier": {
        "semi": true,
        "trailingComma": "es5",
        "singleQuote": true,
        "printWidth": 100,
        "tabWidth": 4
    },
    "eslintConfig": {
        "extends": [
            "eslint:recommended",
            "@typescript-eslint/recommended",
            "prettier"
        ],
        "parser": "@typescript-eslint/parser",
        "plugins": ["@typescript-eslint", "prettier"],
        "rules": {
            "prettier/prettier": "error",
            "@typescript-eslint/no-unused-vars": "error",
            "@typescript-eslint/no-explicit-any": "warn"
        }
    }
}
EOF

    # Create additional configuration files
    create_additional_configs
}

create_additional_configs() {
    # EditorConfig for consistent formatting
    cat > "$TEST_DATA_DIR/.editorconfig" << 'EOF'
# EditorConfig is awesome: https://EditorConfig.org

root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
trim_trailing_whitespace = true
indent_style = space
indent_size = 4

[*.{js,ts,json}]
indent_size = 4

[*.{yml,yaml}]
indent_size = 2

[*.md]
trim_trailing_whitespace = false
EOF

    # Prettier configuration
    cat > "$TEST_DATA_DIR/.prettierrc" << 'EOF'
{
    "semi": true,
    "trailingComma": "es5",
    "singleQuote": true,
    "printWidth": 100,
    "tabWidth": 4,
    "useTabs": false,
    "bracketSpacing": true,
    "arrowParens": "avoid",
    "endOfLine": "lf"
}
EOF

    # ESLint configuration
    cat > "$TEST_DATA_DIR/.eslintrc.js" << 'EOF'
module.exports = {
    env: {
        node: true,
        es2020: true,
        jest: true
    },
    extends: [
        'eslint:recommended',
        '@typescript-eslint/recommended',
        'prettier'
    ],
    parser: '@typescript-eslint/parser',
    parserOptions: {
        ecmaVersion: 2020,
        sourceType: 'module',
        project: './tsconfig.json'
    },
    plugins: ['@typescript-eslint', 'prettier'],
    rules: {
        'prettier/prettier': 'error',
        '@typescript-eslint/no-unused-vars': 'error',
        '@typescript-eslint/no-explicit-any': 'warn',
        '@typescript-eslint/explicit-function-return-type': 'off',
        '@typescript-eslint/explicit-module-boundary-types': 'off',
        'no-console': 'off'
    },
    ignorePatterns: [
        'dist/',
        'node_modules/',
        'coverage/',
        '*.d.ts'
    ]
};
EOF

    # Simple README for test data
    cat > "$TEST_DATA_DIR/README.md" << 'EOF'
# LSP Gateway Test Data

This directory contains comprehensive test data for LSP Gateway integration testing.

## Files

- `sample.go` - Go language examples
- `sample.py` - Python language examples  
- `sample.ts` - TypeScript language examples
- `sample.js` - JavaScript language examples
- Various configuration files for each language

## Usage

These files are used by the LSP Gateway test suite to validate language server functionality across different programming languages.

## Running Examples

```bash
# Go
go run sample.go

# Python  
python sample.py

# TypeScript
npx ts-node sample.ts

# JavaScript
node sample.js
```
EOF
}

# Build and validation
build_and_validate_lsp_gateway() {
    log_step "Building and validating LSP Gateway..."
    
    # Clean previous builds
    log_info "Cleaning previous builds..."
    if make clean &>/dev/null; then
        log_success "Clean completed successfully"
    else
        log_warning "Clean had issues (may be expected)"
    fi
    
    # Build for local platform
    log_info "Building LSP Gateway for local platform..."
    if make local; then
        log_success "LSP Gateway built successfully"
    else
        log_error "Failed to build LSP Gateway"
        return 1
    fi
    
    # Validate binary existence
    if [[ ! -f "./bin/lsp-gateway" ]]; then
        log_error "LSP Gateway binary not found at ./bin/lsp-gateway"
        return 1
    fi
    
    # Test basic functionality
    log_info "Testing basic LSP Gateway functionality..."
    
    if ./bin/lsp-gateway version &>/dev/null; then
        log_success "Version command works"
    else
        log_error "Version command failed"
        return 1
    fi
    
    if ./bin/lsp-gateway --help &>/dev/null; then
        log_success "Help command works"
    else
        log_warning "Help command had issues"
    fi
    
    # Test configuration validation
    log_info "Testing configuration validation..."
    if ./bin/lsp-gateway setup --dry-run &>/dev/null; then
        log_success "Setup dry-run works"
    else
        log_warning "Setup dry-run had issues (may be expected)"
    fi
    
    # Test diagnostics
    log_info "Running system diagnostics..."
    if timeout 30 ./bin/lsp-gateway diagnose &>/dev/null; then
        log_success "System diagnostics completed"
    else
        log_warning "System diagnostics had timeout or issues"
    fi
    
    log_success "LSP Gateway validation completed"
}

# Environment health checks
run_comprehensive_health_checks() {
    log_step "Running comprehensive environment health checks..."
    
    local health_check_results=()
    
    # System health check
    if run_system_health_check; then
        health_check_results+=("System: ✓")
    else
        health_check_results+=("System: ✗")
    fi
    
    # Runtime health check
    if run_runtime_health_check; then
        health_check_results+=("Runtimes: ✓")
    else
        health_check_results+=("Runtimes: ✗")
    fi
    
    # LSP server health check
    if run_lsp_server_health_check; then
        health_check_results+=("LSP Servers: ✓")
    else
        health_check_results+=("LSP Servers: ✗")
    fi
    
    # Repository health check
    if run_repository_health_check; then
        health_check_results+=("Repositories: ✓")
    else
        health_check_results+=("Repositories: ✗")
    fi
    
    # Configuration health check
    if run_configuration_health_check; then
        health_check_results+=("Configuration: ✓")
    else
        health_check_results+=("Configuration: ✗")
    fi
    
    log_info "Health check summary:"
    for result in "${health_check_results[@]}"; do
        log "  $result"
    done
    
    log_success "Comprehensive health checks completed"
}

run_system_health_check() {
    log_info "Running system health check..."
    
    if [[ -f "$HEALTH_CHECK_DIR/system-health-check.sh" ]]; then
        if bash "$HEALTH_CHECK_DIR/system-health-check.sh" &>/dev/null; then
            log_success "System health check passed"
            return 0
        else
            log_warning "System health check failed"
            return 1
        fi
    else
        log_warning "System health check script not found"
        return 1
    fi
}

run_runtime_health_check() {
    log_info "Running runtime health check..."
    
    local runtime_issues=()
    
    # Check Go
    if ! command -v go &>/dev/null; then
        runtime_issues+=("Go not available")
    fi
    
    # Check Python  
    if ! command -v python3 &>/dev/null && ! command -v python &>/dev/null; then
        runtime_issues+=("Python not available")
    fi
    
    # Check Node.js
    if ! command -v node &>/dev/null; then
        runtime_issues+=("Node.js not available")
    fi
    
    if [[ ${#runtime_issues[@]} -eq 0 ]]; then
        log_success "Runtime health check passed"
        return 0
    else
        log_warning "Runtime issues: ${runtime_issues[*]}"
        return 1
    fi
}

run_lsp_server_health_check() {
    log_info "Running LSP server health check..."
    
    if [[ -f "$HEALTH_CHECK_DIR/lsp-health-check.sh" ]]; then
        if bash "$HEALTH_CHECK_DIR/lsp-health-check.sh" &>/dev/null; then
            log_success "LSP server health check passed"
            return 0
        else
            log_warning "LSP server health check failed"
            return 1
        fi
    else
        log_warning "LSP server health check script not found"
        return 1
    fi
}

run_repository_health_check() {
    log_info "Running repository health check..."
    
    if [[ ! -d "$REPO_CACHE_DIR" ]]; then
        log_warning "Repository cache directory not found"
        return 1
    fi
    
    local repo_issues=()
    local expected_repos=("go-example" "python-example" "typescript-example" "java-example")
    
    for repo in "${expected_repos[@]}"; do
        if [[ ! -d "$REPO_CACHE_DIR/$repo" ]]; then
            repo_issues+=("$repo missing")
        elif [[ ! -d "$REPO_CACHE_DIR/$repo/.git" ]]; then
            repo_issues+=("$repo invalid (no .git)")
        fi
    done
    
    if [[ ${#repo_issues[@]} -eq 0 ]]; then
        log_success "Repository health check passed"
        return 0
    else
        log_warning "Repository issues: ${repo_issues[*]}"
        return 1
    fi
}

run_configuration_health_check() {
    log_info "Running configuration health check..."
    
    local config_dirs=("$TEST_CONFIG_DIR" "$LSP_CONFIG_DIR" "$HEALTH_CHECK_DIR")
    local config_issues=()
    
    for dir in "${config_dirs[@]}"; do
        if [[ ! -d "$dir" ]]; then
            config_issues+=("$dir missing")
        fi
    done
    
    # Check specific configuration files
    local required_configs=(
        "$TEST_CONFIG_DIR/integration-test-config.yaml"
        "$TEST_CONFIG_DIR/performance-test-config.yaml"
        "$TEST_CONFIG_DIR/ci-test-config.yaml"
        "$LSP_CONFIG_DIR/gopls.yaml"
        "$HEALTH_CHECK_DIR/health-check-config.yaml"
    )
    
    for config in "${required_configs[@]}"; do
        if [[ ! -f "$config" ]]; then
            config_issues+=("$(basename "$config") missing")
        fi
    done
    
    if [[ ${#config_issues[@]} -eq 0 ]]; then
        log_success "Configuration health check passed"
        return 0
    else
        log_warning "Configuration issues: ${config_issues[*]}"
        return 1
    fi
}

# Setup result directories and permissions
setup_result_directories_and_permissions() {
    log_step "Setting up result directories and permissions..."
    
    local result_dirs=(
        "./integration-test-results"
        "./performance-test-results"
        "./benchmark-results"
        "./coverage-reports"
        "./test-logs"
        "./health-check-logs"
        "./lsp-server-logs"
    )
    
    for dir in "${result_dirs[@]}"; do
        mkdir -p "$dir"
        chmod 755 "$dir"
        log_success "Created directory: $dir"
    done
    
    # Create .gitignore files for result directories
    create_gitignore_files "${result_dirs[@]}"
    
    # Set script permissions
    setup_script_permissions
    
    # Create completion markers
    create_completion_markers
    
    log_success "Result directories and permissions setup completed"
}

create_gitignore_files() {
    local dirs=("$@")
    
    for dir in "${dirs[@]}"; do
        cat > "$dir/.gitignore" << 'EOF'
# Test result files
*.txt
*.log
*.out
*.prof
*.html
*.csv
*.json
*.xml
*.junit
*.tap
*.coverage

# Temporary files
*.tmp
*.temp
.DS_Store
Thumbs.db

# IDE files
.vscode/
.idea/
*.swp
*.swo
*~

# Keep directory structure
!.gitignore
!README.md
EOF
        log_verbose "Created .gitignore for $dir"
    done
}

setup_script_permissions() {
    log_info "Setting up script permissions..."
    
    local scripts=(
        "./scripts/setup-env.sh"
        "./scripts/setup-test-environment.sh"
        "./scripts/run-integration-tests.sh"
        "./scripts/run-performance-tests.sh"
        "./scripts/run-benchmarks.sh"
        "./scripts/run-memory-tests.sh"
    )
    
    for script in "${scripts[@]}"; do
        if [[ -f "$script" ]]; then
            chmod +x "$script"
            log_success "Made $script executable"
        else
            log_verbose "$script not found (skipping)"
        fi
    done
    
    # Set health check script permissions
    if [[ -d "$HEALTH_CHECK_DIR" ]]; then
        find "$HEALTH_CHECK_DIR" -name "*.sh" -type f -exec chmod +x {} \;
        log_success "Made health check scripts executable"
    fi
}

create_completion_markers() {
    log_info "Creating completion markers..."
    
    local marker_dirs=("$REPO_CACHE_DIR" "$LSP_CONFIG_DIR" "$HEALTH_CHECK_DIR" "$TEST_CONFIG_DIR" "$TEST_DATA_DIR")
    
    for dir in "${marker_dirs[@]}"; do
        if [[ -d "$dir" ]]; then
            cat > "$dir/.setup_complete" << EOF
setup_completed_at=$(date -Iseconds)
setup_script=$0
setup_version=$SCRIPT_VERSION
setup_user=$(whoami)
setup_hostname=$(hostname)
setup_cwd=$(pwd)
EOF
            log_verbose "Created completion marker for $dir"
        fi
    done
}

# Cleanup and reset functionality
cleanup_environment() {
    log_step "Cleaning up environment..."
    
    local cleanup_options=$1
    
    case "$cleanup_options" in
        "full")
            cleanup_full
            ;;
        "partial")
            cleanup_partial
            ;;
        "repositories")
            cleanup_repositories
            ;;
        "configs")
            cleanup_configurations
            ;;
        *)
            log_warning "Unknown cleanup option: $cleanup_options"
            log_info "Available options: full, partial, repositories, configs"
            return 1
            ;;
    esac
    
    log_success "Cleanup completed"
}

cleanup_full() {
    log_info "Performing full cleanup..."
    
    local cleanup_dirs=(
        "$REPO_CACHE_DIR"
        "$LSP_CONFIG_DIR"
        "$HEALTH_CHECK_DIR"
        "./integration-test-results"
        "./performance-test-results"
        "./benchmark-results"
        "./coverage-reports"
        "./test-logs"
        "./health-check-logs"
        "./lsp-server-logs"
    )
    
    for dir in "${cleanup_dirs[@]}"; do
        if [[ -d "$dir" ]]; then
            rm -rf "$dir"
            log_success "Removed $dir"
        else
            log_verbose "$dir not found (skipping)"
        fi
    done
    
    # Remove log files
    local log_files=("$SETUP_LOG" "./test-environment-setup.log")
    for log_file in "${log_files[@]}"; do
        if [[ -f "$log_file" ]]; then
            rm -f "$log_file"
            log_success "Removed $log_file"
        fi
    done
}

cleanup_partial() {
    log_info "Performing partial cleanup..."
    
    # Only clean result directories, keep configurations
    local cleanup_dirs=(
        "./integration-test-results"
        "./performance-test-results"
        "./benchmark-results"
        "./coverage-reports"
        "./test-logs"
        "./health-check-logs"
        "./lsp-server-logs"
    )
    
    for dir in "${cleanup_dirs[@]}"; do
        if [[ -d "$dir" ]]; then
            find "$dir" -type f -name "*.log" -delete 2>/dev/null || true
            find "$dir" -type f -name "*.tmp" -delete 2>/dev/null || true
            find "$dir" -type f -name "*.temp" -delete 2>/dev/null || true
            log_success "Cleaned temporary files from $dir"
        fi
    done
}

cleanup_repositories() {
    log_info "Cleaning up repositories..."
    
    if [[ -d "$REPO_CACHE_DIR" ]]; then
        rm -rf "$REPO_CACHE_DIR"
        log_success "Removed repository cache directory"
    else
        log_info "Repository cache directory not found"
    fi
}

cleanup_configurations() {
    log_info "Cleaning up configurations..."
    
    local config_dirs=("$LSP_CONFIG_DIR" "$HEALTH_CHECK_DIR")
    
    for dir in "${config_dirs[@]}"; do
        if [[ -d "$dir" ]]; then
            rm -rf "$dir"
            log_success "Removed $dir"
        else
            log_verbose "$dir not found (skipping)"
        fi
    done
}

# Display comprehensive setup summary
display_comprehensive_summary() {
    log_step "=== LSP GATEWAY COMPREHENSIVE ENVIRONMENT SETUP SUMMARY ==="
    
    echo ""
    echo -e "${BOLD}Environment Status:${NC}"
    
    # System information
    echo "  System Information:"
    echo "    Platform: $(uname -s) $(uname -m)"
    echo "    User: $(whoami)"
    echo "    Working Directory: $(pwd)"
    echo "    Setup Time: $(date)"
    
    # Runtime status
    echo ""
    echo "  Runtime Environments:"
    if command -v go &> /dev/null; then
        echo "    ✓ Go $(go version | grep -oE 'go[0-9]+\.[0-9]+(\.[0-9]+)?' | sed 's/go//')"
    else
        echo "    ✗ Go not found"
    fi
    
    if command -v python3 &> /dev/null; then
        echo "    ✓ Python $(python3 --version | grep -oE '[0-9]+\.[0-9]+(\.[0-9]+)?')"
    elif command -v python &> /dev/null; then
        echo "    ✓ Python $(python --version | grep -oE '[0-9]+\.[0-9]+(\.[0-9]+)?')"
    else
        echo "    ✗ Python not found"
    fi
    
    if command -v node &> /dev/null; then
        echo "    ✓ Node.js $(node --version | sed 's/v//')"
    else
        echo "    ✗ Node.js not found"
    fi
    
    if command -v java &> /dev/null; then
        echo "    ✓ Java $(java -version 2>&1 | head -n1 | grep -oE '[0-9]+(\.[0-9]+)*')"
    else
        echo "    ~ Java not found (optional)"
    fi
    
    # LSP server status
    echo ""
    echo "  Available LSP Servers:"
    command -v gopls &> /dev/null && echo "    ✓ Go (gopls)" || echo "    ✗ Go (gopls)"
    command -v pylsp &> /dev/null && echo "    ✓ Python (pylsp)" || echo "    ✗ Python (pylsp)"
    command -v typescript-language-server &> /dev/null && echo "    ✓ TypeScript (typescript-language-server)" || echo "    ✗ TypeScript (typescript-language-server)"
    echo "    ~ Java (jdtls) - requires manual setup"
    
    # Repository status
    echo ""
    echo "  Test Repositories:"
    if [[ -d "$REPO_CACHE_DIR" ]]; then
        echo "    ✓ Repository cache directory created: $REPO_CACHE_DIR"
        
        # Try to get status from clone-repos.sh if available
        local clone_script="./scripts/clone-repos.sh"
        local config_file="./test-repositories.yaml"
        
        if [[ -f "$clone_script" && -f "$config_file" ]]; then
            # Use clone-repos.sh to show detailed status
            local repo_status
            repo_status=$("$clone_script" status --dir "$REPO_CACHE_DIR" --config "$config_file" --quiet 2>/dev/null | tail -n 1 || echo "")
            if [[ -n "$repo_status" ]]; then
                echo "    ✓ Repository status: $repo_status"
            else
                echo "    ⚠ Repository status unavailable"
            fi
        else
            # Fallback to basic directory count
            local repo_count=$(find "$REPO_CACHE_DIR" -maxdepth 2 -name ".git" -type d | wc -l)
            if [[ $repo_count -gt 0 ]]; then
                echo "    ✓ $repo_count test repositories available"
            else
                echo "    ⚠ No repositories found in cache directory"
            fi
        fi
    else
        echo "    ✗ Repository cache directory not found"
    fi
    
    # Configuration status
    echo ""
    echo "  Configuration Files:"
    echo "    Test Configurations: $TEST_CONFIG_DIR/"
    [[ -f "$TEST_CONFIG_DIR/integration-test-config.yaml" ]] && echo "      ✓ Integration test config" || echo "      ✗ Integration test config"
    [[ -f "$TEST_CONFIG_DIR/performance-test-config.yaml" ]] && echo "      ✓ Performance test config" || echo "      ✗ Performance test config"
    
    echo "    LSP Server Configurations: $LSP_CONFIG_DIR/"
    [[ -f "$LSP_CONFIG_DIR/gopls.yaml" ]] && echo "      ✓ Go LSP config" || echo "      ✗ Go LSP config"
    [[ -f "$LSP_CONFIG_DIR/pylsp.yaml" ]] && echo "      ✓ Python LSP config" || echo "      ✗ Python LSP config"
    [[ -f "$LSP_CONFIG_DIR/typescript-language-server.yaml" ]] && echo "      ✓ TypeScript LSP config" || echo "      ✗ TypeScript LSP config"
    
    echo "    Health Check Configurations: $HEALTH_CHECK_DIR/"
    [[ -f "$HEALTH_CHECK_DIR/health-check-config.yaml" ]] && echo "      ✓ Health check config" || echo "      ✗ Health check config"
    [[ -f "$HEALTH_CHECK_DIR/system-health-check.sh" ]] && echo "      ✓ System health check script" || echo "      ✗ System health check script"
    [[ -f "$HEALTH_CHECK_DIR/lsp-health-check.sh" ]] && echo "      ✓ LSP health check script" || echo "      ✗ LSP health check script"
    
    # Test data status
    echo ""
    echo "  Test Data:"
    echo "    Sample files: $TEST_DATA_DIR/"
    [[ -f "$TEST_DATA_DIR/sample.go" ]] && echo "      ✓ Go sample project" || echo "      ✗ Go sample project"
    [[ -f "$TEST_DATA_DIR/sample.py" ]] && echo "      ✓ Python sample project" || echo "      ✗ Python sample project"
    [[ -f "$TEST_DATA_DIR/sample.ts" ]] && echo "      ✓ TypeScript sample project" || echo "      ✗ TypeScript sample project"
    [[ -f "$TEST_DATA_DIR/sample.js" ]] && echo "      ✓ JavaScript sample project" || echo "      ✗ JavaScript sample project"
    
    # LSP Gateway status
    echo ""
    echo "  LSP Gateway Binary:"
    if [[ -f "./bin/lsp-gateway" ]]; then
        echo "    ✓ Binary built and available"
        if ./bin/lsp-gateway version &>/dev/null; then
            echo "    ✓ Basic functionality verified"
        else
            echo "    ⚠ Basic functionality check failed"
        fi
    else
        echo "    ✗ Binary not found"
    fi
    
    # Available commands
    echo ""
    echo -e "${BOLD}Available Commands:${NC}"
    echo "  Testing:"
    echo "    ./scripts/run-integration-tests.sh    # Run integration tests"
    echo "    ./scripts/run-performance-tests.sh    # Run performance tests"  
    echo "    ./scripts/run-benchmarks.sh          # Run benchmarks"
    echo "    ./scripts/run-memory-tests.sh        # Run memory tests"
    
    echo "  LSP Gateway:"
    echo "    ./bin/lsp-gateway server              # Start HTTP Gateway"
    echo "    ./bin/lsp-gateway mcp                 # Start MCP Server"
    echo "    ./bin/lsp-gateway setup all           # Run automated setup"
    echo "    ./bin/lsp-gateway diagnose            # System diagnostics"
    echo "    ./bin/lsp-gateway status              # System status"
    
    echo "  Health Checks:"
    echo "    $HEALTH_CHECK_DIR/system-health-check.sh      # System health"
    echo "    $HEALTH_CHECK_DIR/lsp-health-check.sh         # LSP server health"
    
    echo "  Repository Management:"
    echo "    ./scripts/clone-repos.sh list             # List available repositories"
    echo "    ./scripts/clone-repos.sh status           # Show repository status"
    echo "    ./scripts/clone-repos.sh validate         # Validate repositories"
    echo "    ./scripts/clone-repos.sh cleanup all      # Clean up all repositories"
    
    echo "  Environment Management:"
    echo "    $0 --cleanup full              # Full cleanup"
    echo "    $0 --cleanup partial           # Partial cleanup"
    echo "    $0 --cleanup repositories      # Repository cleanup"
    echo "    $0 --reset                     # Reset and re-run setup"
    
    # Next steps
    echo ""
    echo -e "${BOLD}Next Steps:${NC}"
    echo "  1. Validate setup: $0 --validate"
    echo "  2. Run health checks: $0 --health-check"
    echo "  3. Run integration tests: ./scripts/run-integration-tests.sh"
    echo "  4. Start LSP Gateway server: ./bin/lsp-gateway server"
    echo "  5. View documentation: README.md and docs/"
    
    echo ""
    echo -e "${GREEN}${BOLD}✓ Environment setup completed successfully!${NC}"
    echo "Setup log saved to: $SETUP_LOG"
    
    # Final statistics
    local end_time=$(date +%s)
    local start_time_file="/tmp/lsp_gateway_setup_start_$$"
    if [[ -f "$start_time_file" ]]; then
        local start_time=$(cat "$start_time_file")
        local duration=$((end_time - start_time))
        echo "Total setup time: ${duration}s"
        rm -f "$start_time_file"
    fi
}

# Help function
show_help() {
    cat << EOF
$SCRIPT_NAME v$SCRIPT_VERSION

USAGE:
    $0 [OPTIONS]

DESCRIPTION:
    Comprehensive environment setup for LSP Gateway testing and validation.
    Prepares runtime environments, LSP servers, test repositories, configuration
    files, and health monitoring for complete LSP functionality testing.

OPTIONS:
    --skip-runtimes           Skip runtime environment validation
    --skip-lsp-servers        Skip LSP server installation
    --skip-repositories       Skip test repository cloning
    --skip-configs            Skip configuration file creation
    --skip-health-checks      Skip health check setup
    --skip-validation         Skip environment validation tests
    --minimal                 Minimal setup (skip optional components)
    --parallel               Enable parallel operations (default: true)
    --cleanup OPTION         Clean up environment (full|partial|repositories|configs)
    --reset                  Reset environment and re-run full setup
    --validate               Validate existing environment setup
    --health-check           Run comprehensive health checks
    --verbose                Enable verbose output
    --dry-run                Show what would be done without executing
    --help, -h               Show this help message
    --version                Show script version

EXAMPLES:
    $0                             # Full environment setup
    $0 --minimal                   # Minimal setup
    $0 --skip-repositories         # Setup without repository cloning
    $0 --cleanup full              # Complete environment cleanup
    $0 --reset                     # Reset and re-setup environment
    $0 --validate                  # Validate existing setup
    $0 --health-check              # Run health checks only

ENVIRONMENT VARIABLES:
    PARALLEL_OPERATIONS      Enable/disable parallel operations (true|false)
    CLEANUP_ON_FAILURE       Clean up on failure (true|false)
    VERBOSE_OUTPUT           Enable verbose output (true|false)

SETUP COMPONENTS:
    ✓ System dependency validation
    ✓ Runtime environment setup (Go, Python, Node.js, Java)  
    ✓ LSP server installation and configuration
    ✓ Test repository cloning and setup (via scripts/clone-repos.sh)
    ✓ Configuration file generation
    ✓ Health check and monitoring setup
    ✓ Result directory preparation
    ✓ LSP Gateway build and validation

DIRECTORIES CREATED:
    $REPO_CACHE_DIR          # Test repository cache (managed by clone-repos.sh)
    $LSP_CONFIG_DIR          # LSP server configurations
    $HEALTH_CHECK_DIR        # Health check scripts and configs
    ./integration-test-results   # Integration test outputs
    ./performance-test-results   # Performance test outputs
    ./benchmark-results          # Benchmark outputs
    ./coverage-reports           # Test coverage reports

INTEGRATION WITH EXISTING SCRIPTS:
    - Uses scripts/clone-repos.sh for repository management
    - Creates test-repositories.yaml for clone-repos.sh configuration
    - Integrates with existing LSP Gateway build system
    - Extends setup-test-environment.sh functionality

For more information, see the project documentation and README.md files.
EOF
}

# Version display
show_version() {
    echo "$SCRIPT_NAME v$SCRIPT_VERSION"
    echo "Compatible with LSP Gateway v1.0.0+"
    echo "Minimum requirements:"
    echo "  Go $MIN_GO_VERSION+"
    echo "  Python $MIN_PYTHON_VERSION+ (optional)"
    echo "  Node.js $MIN_NODE_VERSION+ (optional)" 
    echo "  Java $MIN_JAVA_VERSION+ (optional)"
}

# Validation mode
validate_environment() {
    log_step "Validating existing environment setup..."
    
    local validation_passed=true
    
    # Validate system dependencies
    if ! validate_system_dependencies; then
        validation_passed=false
    fi
    
    # Validate runtime environments
    if ! validate_runtime_environments; then
        validation_passed=false
    fi
    
    # Validate LSP Gateway binary
    if [[ ! -f "./bin/lsp-gateway" ]]; then
        log_error "LSP Gateway binary not found"
        validation_passed=false
    elif ! ./bin/lsp-gateway version &>/dev/null; then
        log_error "LSP Gateway binary is not functional"
        validation_passed=false
    else
        log_success "LSP Gateway binary is functional"
    fi
    
    # Validate directories
    local required_dirs=("$TEST_CONFIG_DIR" "$TEST_DATA_DIR")
    for dir in "${required_dirs[@]}"; do
        if [[ ! -d "$dir" ]]; then
            log_error "Required directory missing: $dir"
            validation_passed=false
        else
            log_success "Directory exists: $dir"
        fi
    done
    
    # Validate configuration files
    local required_configs=(
        "$TEST_CONFIG_DIR/integration-test-config.yaml"
        "$TEST_DATA_DIR/sample.go"
        "$TEST_DATA_DIR/package.json"
    )
    
    for config in "${required_configs[@]}"; do
        if [[ ! -f "$config" ]]; then
            log_error "Required configuration missing: $config"
            validation_passed=false
        else
            log_success "Configuration exists: $config"
        fi
    done
    
    if [[ "$validation_passed" == true ]]; then
        log_success "Environment validation passed"
        return 0
    else
        log_error "Environment validation failed"
        return 1
    fi
}

# Main execution function
main() {
    # Record start time
    echo $$ > "/tmp/lsp_gateway_setup_start_$$"
    
    # Parse command line arguments
    local skip_runtimes=false
    local skip_lsp_servers=false
    local skip_repositories=false
    local skip_configs=false
    local skip_health_checks=false
    local skip_validation=false
    local minimal_setup=false
    local cleanup_mode=""
    local reset_mode=false
    local validate_mode=false
    local health_check_mode=false
    local dry_run=false
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            --skip-runtimes)
                skip_runtimes=true
                shift
                ;;
            --skip-lsp-servers)
                skip_lsp_servers=true
                shift
                ;;
            --skip-repositories)
                skip_repositories=true
                shift
                ;;
            --skip-configs)
                skip_configs=true
                shift
                ;;
            --skip-health-checks)
                skip_health_checks=true
                shift
                ;;
            --skip-validation)
                skip_validation=true
                shift
                ;;
            --minimal)
                minimal_setup=true
                skip_repositories=true
                skip_health_checks=true
                shift
                ;;
            --parallel)
                PARALLEL_OPERATIONS="true"
                shift
                ;;
            --cleanup)
                cleanup_mode=$2
                shift 2
                ;;
            --reset)
                reset_mode=true
                shift
                ;;
            --validate)
                validate_mode=true
                shift
                ;;
            --health-check)
                health_check_mode=true
                shift
                ;;
            --verbose)
                VERBOSE_OUTPUT="true"
                shift
                ;;
            --dry-run)
                dry_run=true
                shift
                ;;
            --version)
                show_version
                exit 0
                ;;
            --help|-h)
                show_help
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
    
    # Initialize logging
    initialize_logging
    
    # Handle special modes
    if [[ -n "$cleanup_mode" ]]; then
        cleanup_environment "$cleanup_mode"
        exit $?
    fi
    
    if [[ "$reset_mode" == true ]]; then
        log_info "Resetting environment..."
        cleanup_environment "full"
        log_info "Proceeding with fresh setup..."
    fi
    
    if [[ "$validate_mode" == true ]]; then
        validate_environment
        exit $?
    fi
    
    if [[ "$health_check_mode" == true ]]; then
        run_comprehensive_health_checks
        exit $?
    fi
    
    if [[ "$dry_run" == true ]]; then
        log_info "DRY RUN MODE - No changes will be made"
        log_info "Would execute full environment setup with current options"
        show_help
        exit 0
    fi
    
    # Core setup sequence
    log_info "Starting comprehensive environment setup..."
    log_info "Options: runtimes=$(!$skip_runtimes), lsp-servers=$(!$skip_lsp_servers), repositories=$(!$skip_repositories), configs=$(!$skip_configs), health-checks=$(!$skip_health_checks), validation=$(!$skip_validation), minimal=$minimal_setup"
    
    # System validation (always required)
    validate_system_dependencies
    
    # Runtime environment validation
    if [[ "$skip_runtimes" != true ]]; then
        validate_runtime_environments
    else
        log_warning "Skipping runtime environment validation"
    fi
    
    # Build LSP Gateway (always required)
    build_and_validate_lsp_gateway
    
    # Setup core directories
    setup_result_directories_and_permissions
    
    # Configuration creation
    if [[ "$skip_configs" != true ]]; then
        create_configuration_files
        create_enhanced_test_data
    else
        log_warning "Skipping configuration file creation"
    fi
    
    # Repository setup
    if [[ "$skip_repositories" != true ]]; then
        setup_test_repositories
    else
        log_warning "Skipping test repository setup"
    fi
    
    # LSP server setup
    if [[ "$skip_lsp_servers" != true ]]; then
        setup_lsp_servers
    else
        log_warning "Skipping LSP server setup"
    fi
    
    # Health check setup
    if [[ "$skip_health_checks" != true ]]; then
        # Health check configuration already created in create_configuration_files
        log_info "Health check setup completed"
    else
        log_warning "Skipping health check setup"
    fi
    
    # Environment validation
    if [[ "$skip_validation" != true ]]; then
        run_comprehensive_health_checks
    else
        log_warning "Skipping environment validation"
    fi
    
    # Display comprehensive summary
    display_comprehensive_summary
    
    log_success "Comprehensive environment setup completed successfully!"
    log "Setup log saved to: $SETUP_LOG"
}

# Execute main function with all arguments
main "$@"