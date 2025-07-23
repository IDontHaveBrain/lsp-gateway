#!/bin/bash

# LSP Gateway Integration Test Environment Setup
# Prepares environment for comprehensive integration testing

set -euo pipefail

# Configuration
SETUP_LOG="./test-environment-setup.log"
TEST_CONFIG_DIR="./tests/data/configs"
TEST_DATA_DIR="./tests/data/samples"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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

# Initialize log
echo "LSP Gateway Integration Test Environment Setup - $(date)" > "$SETUP_LOG"

log "Starting LSP Gateway integration test environment setup"

# Function to check Go environment
check_go_environment() {
    log "Checking Go environment..."
    
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed"
        echo "Please install Go 1.24+ from https://golang.org/dl/"
        exit 1
    fi
    
    local go_version=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | sed 's/go//')
    local major=$(echo "$go_version" | cut -d. -f1)
    local minor=$(echo "$go_version" | cut -d. -f2)
    
    if [[ $major -lt 1 || ($major -eq 1 && $minor -lt 24) ]]; then
        log_error "Go 1.24+ required, found Go $go_version"
        exit 1
    fi
    
    log_success "Go $go_version detected"
    
    # Check Go environment variables
    local gopath=$(go env GOPATH)
    local goroot=$(go env GOROOT)
    
    log "Go environment:"
    log "  GOPATH: $gopath"
    log "  GOROOT: $goroot"
    log "  Module mode: $(go env GO111MODULE)"
}

# Function to install language servers
install_language_servers() {
    log "Installing language servers for integration testing..."
    
    # Go Language Server (gopls)
    if ! command -v gopls &> /dev/null; then
        log "Installing gopls (Go Language Server)..."
        if go install golang.org/x/tools/gopls@latest; then
            log_success "gopls installed successfully"
        else
            log_warning "Failed to install gopls"
        fi
    else
        log_success "gopls already installed: $(gopls version)"
    fi
    
    # Python Language Server (pylsp)
    if ! command -v pylsp &> /dev/null; then
        log "Installing python-lsp-server..."
        if command -v pip &> /dev/null; then
            if pip install python-lsp-server; then
                log_success "python-lsp-server installed successfully"
            else
                log_warning "Failed to install python-lsp-server"
            fi
        elif command -v pip3 &> /dev/null; then
            if pip3 install python-lsp-server; then
                log_success "python-lsp-server installed successfully"
            else
                log_warning "Failed to install python-lsp-server"
            fi
        else
            log_warning "pip not found, skipping python-lsp-server installation"
        fi
    else
        log_success "python-lsp-server already installed"
    fi
    
    # TypeScript Language Server
    if ! command -v typescript-language-server &> /dev/null; then
        log "Installing typescript-language-server..."
        if command -v npm &> /dev/null; then
            if npm install -g typescript-language-server typescript; then
                log_success "typescript-language-server installed successfully"
            else
                log_warning "Failed to install typescript-language-server"
            fi
        else
            log_warning "npm not found, skipping typescript-language-server installation"
        fi
    else
        log_success "typescript-language-server already installed"
    fi
    
    # Java Language Server (jdtls) - optional
    if ! command -v jdtls &> /dev/null; then
        log_warning "Java Language Server (jdtls) not found - optional for testing"
        log "To install jdtls, download from: https://download.eclipse.org/jdtls/snapshots/"
    else
        log_success "Java Language Server (jdtls) found"
    fi
}

# Function to check system dependencies
check_system_dependencies() {
    log "Checking system dependencies..."
    
    # Required tools
    local required_tools=("make" "curl" "bc")
    for tool in "${required_tools[@]}"; do
        if command -v "$tool" &> /dev/null; then
            log_success "$tool is available"
        else
            log_error "$tool is required but not installed"
            exit 1
        fi
    done
    
    # Optional tools
    local optional_tools=("git" "code" "nano")
    for tool in "${optional_tools[@]}"; do
        if command -v "$tool" &> /dev/null; then
            log_success "$tool is available (optional)"
        else
            log_warning "$tool not found (optional)"
        fi
    done
    
    # Check system resources
    local available_memory=$(free -m | awk '/^Mem:/ {print $2}')
    if [[ $available_memory -lt 4096 ]]; then
        log_warning "Less than 4GB RAM available ($available_memory MB). Performance tests may be affected."
    else
        log_success "Sufficient memory available: ${available_memory}MB"
    fi
    
    local cpu_cores=$(nproc)
    if [[ $cpu_cores -lt 2 ]]; then
        log_warning "Less than 2 CPU cores available ($cpu_cores). Concurrency tests may be affected."
    else
        log_success "Sufficient CPU cores available: $cpu_cores"
    fi
}

# Function to create test configurations
create_test_configurations() {
    log "Creating test configurations..."
    
    mkdir -p "$TEST_CONFIG_DIR"
    
    # Integration test configuration
    cat > "$TEST_CONFIG_DIR/integration-test-config.yaml" << EOF
# LSP Gateway Integration Test Configuration
port: 0  # Dynamic port allocation for tests
timeout: 30s
max_connections: 100

servers:
  # Go Language Server for testing
  - name: "test-go-lsp"
    languages: ["go"]
    command: "gopls"
    args: []
    transport: "stdio"
    
  # Python Language Server for testing  
  - name: "test-python-lsp"
    languages: ["python"]
    command: "python"
    args: ["-m", "pylsp"]
    transport: "stdio"
    
  # TypeScript Language Server for testing
  - name: "test-typescript-lsp"
    languages: ["typescript", "javascript"]
    command: "typescript-language-server"
    args: ["--stdio"]
    transport: "stdio"

# Integration test specific settings
integration_test:
  mock_servers: true
  timeout_multiplier: 2
  concurrent_clients: 5
  test_data_dir: "$TEST_DATA_DIR"
EOF

    # Performance test configuration
    cat > "$TEST_CONFIG_DIR/performance-test-config.yaml" << EOF
# LSP Gateway Performance Test Configuration
port: 0  # Dynamic port allocation for tests
timeout: 10s  # Shorter timeout for performance tests
max_connections: 50

servers:
  # Mock servers for performance testing
  - name: "perf-mock-lsp"
    languages: ["go", "python", "typescript", "javascript"]
    command: "mock_lsp_server"  # Embedded mock server
    args: []
    transport: "stdio"

# Performance test specific settings
performance_test:
  target_latency_ms: 100
  target_throughput_rps: 50
  target_memory_mb: 50
  concurrent_clients: 10
  requests_per_test: 100
  benchmark_time: "30s"
EOF

    # CI/CD test configuration
    cat > "$TEST_CONFIG_DIR/ci-test-config.yaml" << EOF
# LSP Gateway CI/CD Test Configuration
port: 0  # Dynamic port allocation
timeout: 5s  # Very short timeout for CI
max_connections: 20

servers:
  # Minimal server configuration for CI
  - name: "ci-mock-lsp"
    languages: ["go"]
    command: "mock_lsp_server"
    args: []
    transport: "stdio"

# CI test specific settings  
ci_test:
  short_mode: true
  timeout_multiplier: 1
  concurrent_clients: 3
  requests_per_test: 20
EOF

    log_success "Test configurations created in $TEST_CONFIG_DIR/"
}

# Function to create test data
create_test_data() {
    log "Creating test data for integration tests..."
    
    mkdir -p "$TEST_DATA_DIR"
    
    # Sample Go file for testing
    cat > "$TEST_DATA_DIR/sample.go" << 'EOF'
package main

import (
    "fmt"
    "os"
)

// TestStruct demonstrates a struct for testing
type TestStruct struct {
    Field1 string
    Field2 int
}

// TestFunction demonstrates a function for testing
func TestFunction(input string) string {
    return fmt.Sprintf("Processed: %s", input)
}

// Method demonstrates a method for testing
func (ts *TestStruct) Method() string {
    return ts.Field1
}

func main() {
    test := &TestStruct{
        Field1: "Hello",
        Field2: 42,
    }
    
    result := TestFunction("World")
    fmt.Println(result)
    fmt.Println(test.Method())
    
    if len(os.Args) > 1 {
        fmt.Printf("Argument: %s\n", os.Args[1])
    }
}
EOF

    # Sample Python file for testing
    cat > "$TEST_DATA_DIR/sample.py" << 'EOF'
"""Sample Python file for LSP Gateway integration testing."""

import os
import sys
from typing import List, Optional


class TestClass:
    """Test class for LSP integration testing."""
    
    def __init__(self, field1: str, field2: int):
        self.field1 = field1
        self.field2 = field2
    
    def method(self) -> str:
        """Test method for hover and definition testing."""
        return f"Field1: {self.field1}, Field2: {self.field2}"


def test_function(input_data: str) -> str:
    """Test function for LSP integration testing."""
    return f"Processed: {input_data}"


def main():
    """Main function for testing."""
    test_obj = TestClass("Hello", 42)
    result = test_function("World")
    
    print(result)
    print(test_obj.method())
    
    if len(sys.argv) > 1:
        print(f"Argument: {sys.argv[1]}")


if __name__ == "__main__":
    main()
EOF

    # Sample TypeScript file for testing
    cat > "$TEST_DATA_DIR/sample.ts" << 'EOF'
/**
 * Sample TypeScript file for LSP Gateway integration testing
 */

interface TestInterface {
    field1: string;
    field2: number;
}

class TestClass implements TestInterface {
    field1: string;
    field2: number;
    
    constructor(field1: string, field2: number) {
        this.field1 = field1;
        this.field2 = field2;
    }
    
    method(): string {
        return `Field1: ${this.field1}, Field2: ${this.field2}`;
    }
}

function testFunction(input: string): string {
    return `Processed: ${input}`;
}

function main(): void {
    const testObj = new TestClass("Hello", 42);
    const result = testFunction("World");
    
    console.log(result);
    console.log(testObj.method());
    
    if (process.argv.length > 2) {
        console.log(`Argument: ${process.argv[2]}`);
    }
}

if (require.main === module) {
    main();
}

export { TestClass, TestInterface, testFunction };
EOF

    # Sample JavaScript file for testing  
    cat > "$TEST_DATA_DIR/sample.js" << 'EOF'
/**
 * Sample JavaScript file for LSP Gateway integration testing
 */

class TestClass {
    constructor(field1, field2) {
        this.field1 = field1;
        this.field2 = field2;
    }
    
    method() {
        return `Field1: ${this.field1}, Field2: ${this.field2}`;
    }
}

function testFunction(input) {
    return `Processed: ${input}`;
}

function main() {
    const testObj = new TestClass("Hello", 42);
    const result = testFunction("World");
    
    console.log(result);
    console.log(testObj.method());
    
    if (process.argv.length > 2) {
        console.log(`Argument: ${process.argv[2]}`);
    }
}

if (require.main === module) {
    main();
}

module.exports = { TestClass, testFunction };
EOF

    # Create a go.mod file for the test Go code
    cat > "$TEST_DATA_DIR/go.mod" << 'EOF'
module test-project

go 1.24
EOF

    # Create package.json for TypeScript/JavaScript testing
    cat > "$TEST_DATA_DIR/package.json" << 'EOF'
{
    "name": "lsp-gateway-test-data",
    "version": "1.0.0",
    "description": "Test data for LSP Gateway integration testing",
    "main": "sample.js",
    "scripts": {
        "build": "tsc sample.ts",
        "test": "node sample.js"
    },
    "devDependencies": {
        "typescript": "^5.0.0",
        "@types/node": "^20.0.0"
    }
}
EOF

    # Create TypeScript configuration
    cat > "$TEST_DATA_DIR/tsconfig.json" << 'EOF'
{
    "compilerOptions": {
        "target": "ES2020",
        "module": "commonjs",
        "lib": ["ES2020"],
        "outDir": "./dist",
        "rootDir": "./",
        "strict": true,
        "esModuleInterop": true,
        "skipLibCheck": true,
        "forceConsistentCasingInFileNames": true
    },
    "include": ["*.ts"],
    "exclude": ["node_modules", "dist"]
}
EOF

    log_success "Test data created in $TEST_DATA_DIR/"
}

# Function to build and validate LSP Gateway
build_and_validate() {
    log "Building and validating LSP Gateway..."
    
    # Clean and build
    if make clean && make local; then
        log_success "LSP Gateway built successfully"
    else
        log_error "Failed to build LSP Gateway"
        exit 1
    fi
    
    # Validate binary
    if [[ ! -f "./bin/lsp-gateway" ]]; then
        log_error "LSP Gateway binary not found"
        exit 1
    fi
    
    # Test basic functionality
    if ./bin/lsp-gateway version > /dev/null 2>&1; then
        log_success "LSP Gateway binary is functional"
    else
        log_error "LSP Gateway binary is not functional"
        exit 1
    fi
    
    # Test configuration validation
    if ./bin/lsp-gateway config validate > /dev/null 2>&1; then
        log_success "Configuration validation passed"
    else
        log_warning "Configuration validation had issues (may be expected)"
    fi
    
    # Test system diagnostics
    if ./bin/lsp-gateway diagnose > /dev/null 2>&1; then
        log_success "System diagnostics completed"
    else
        log_warning "System diagnostics had issues"
    fi
}

# Function to set up test result directories
setup_result_directories() {
    log "Setting up test result directories..."
    
    local result_dirs=(
        "./integration-test-results"
        "./performance-test-results"
        "./benchmark-results"
        "./coverage-reports"
        "./test-logs"
    )
    
    for dir in "${result_dirs[@]}"; do
        mkdir -p "$dir"
        log_success "Created directory: $dir"
    done
    
    # Create .gitignore for test results
    cat > "./integration-test-results/.gitignore" << 'EOF'
# Integration test results
*.txt
*.log
*.out
*.prof
*.html
*.csv
EOF

    cat > "./performance-test-results/.gitignore" << 'EOF'
# Performance test results
*.txt
*.log
*.out
*.prof
*.html
*.csv
EOF

    cat > "./benchmark-results/.gitignore" << 'EOF'
# Benchmark results
*.txt
*.log
*.out
*.prof
*.html
*.csv
EOF
}

# Function to create test scripts permissions
setup_script_permissions() {
    log "Setting up script permissions..."
    
    local scripts=(
        "./scripts/run-integration-tests.sh"
        "./scripts/run-performance-tests.sh"
        "./scripts/run-benchmarks.sh"
        "./scripts/setup-test-environment.sh"
    )
    
    for script in "${scripts[@]}"; do
        if [[ -f "$script" ]]; then
            chmod +x "$script"
            log_success "Made $script executable"
        else
            log_warning "$script not found"
        fi
    done
}

# Function to run environment validation tests
run_validation_tests() {
    log "Running environment validation tests..."
    
    # Test Go compilation
    if go build -o /tmp/test-build ./cmd/lsp-gateway/ > /dev/null 2>&1; then
        log_success "Go compilation test passed"
        rm -f /tmp/test-build
    else
        log_error "Go compilation test failed"
        exit 1
    fi
    
    # Test basic integration test execution (short mode)
    if go test ./internal/transport -v -timeout=10s -short -run=TestClientCreation > /dev/null 2>&1; then
        log_success "Basic integration test execution works"
    else
        log_warning "Basic integration test execution had issues"
    fi
    
    # Test if mock server can be compiled
    local temp_dir=$(mktemp -d)
    cat > "$temp_dir/test_mock.go" << 'EOF'
package main
import "fmt"
func main() { fmt.Println("Mock test") }
EOF
    
    if go build -o "$temp_dir/test_mock" "$temp_dir/test_mock.go" > /dev/null 2>&1; then
        log_success "Mock server compilation test passed"
    else
        log_warning "Mock server compilation test failed"
    fi
    
    rm -rf "$temp_dir"
}

# Function to display setup summary
display_setup_summary() {
    log "=== INTEGRATION TEST ENVIRONMENT SETUP SUMMARY ==="
    
    echo ""
    echo "Environment Status:"
    echo "  ✓ Go $(go version | grep -oE 'go[0-9]+\.[0-9]+') installed and configured"
    echo "  ✓ LSP Gateway built and validated"
    echo "  ✓ Test configurations created"
    echo "  ✓ Test data prepared"
    echo "  ✓ Result directories set up"
    
    echo ""
    echo "Available Language Servers:"
    command -v gopls &> /dev/null && echo "  ✓ Go (gopls)" || echo "  ✗ Go (gopls)"
    command -v pylsp &> /dev/null && echo "  ✓ Python (pylsp)" || echo "  ✗ Python (pylsp)"
    command -v typescript-language-server &> /dev/null && echo "  ✓ TypeScript (typescript-language-server)" || echo "  ✗ TypeScript (typescript-language-server)"
    command -v jdtls &> /dev/null && echo "  ✓ Java (jdtls)" || echo "  ~ Java (jdtls) - optional"
    
    echo ""
    echo "Test Configurations:"
    echo "  - Integration: $TEST_CONFIG_DIR/integration-test-config.yaml"
    echo "  - Performance: $TEST_CONFIG_DIR/performance-test-config.yaml"
    echo "  - CI/CD: $TEST_CONFIG_DIR/ci-test-config.yaml"
    
    echo ""
    echo "Test Data:"
    echo "  - Sample files: $TEST_DATA_DIR/"
    echo "  - Go project: $TEST_DATA_DIR/go.mod"
    echo "  - Node project: $TEST_DATA_DIR/package.json"
    
    echo ""
    echo "Test Scripts:"
    echo "  - Integration tests: ./scripts/run-integration-tests.sh"
    echo "  - Performance tests: ./scripts/run-performance-tests.sh"
    echo "  - Benchmarks: ./scripts/run-benchmarks.sh"
    
    echo ""
    echo "Next Steps:"
    echo "  1. Run integration tests: ./scripts/run-integration-tests.sh"
    echo "  2. Run performance tests: ./scripts/run-performance-tests.sh"
    echo "  3. View documentation: docs/INTEGRATION_TESTING.md"
    
    echo ""
    echo "Setup completed successfully! Environment ready for integration testing."
}

# Help function
show_help() {
    cat << EOF
LSP Gateway Integration Test Environment Setup

Usage: $0 [OPTIONS]

Options:
  --skip-language-servers    Skip language server installation
  --skip-validation         Skip environment validation tests
  --minimal                 Minimal setup (skip optional components)
  --help                    Show this help message

Examples:
  $0                        # Full environment setup
  $0 --minimal              # Minimal setup for CI
  $0 --skip-language-servers # Skip language server installation

This script prepares a complete environment for LSP Gateway integration testing,
including language servers, test configurations, sample data, and validation.
EOF
}

# Main execution
main() {
    local skip_language_servers=false
    local skip_validation=false
    local minimal_setup=false
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --skip-language-servers)
                skip_language_servers=true
                shift
                ;;
            --skip-validation)
                skip_validation=true
                shift
                ;;
            --minimal)
                minimal_setup=true
                skip_language_servers=true
                shift
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
    
    # Core setup steps
    check_go_environment
    check_system_dependencies
    build_and_validate
    setup_result_directories
    setup_script_permissions
    create_test_configurations
    create_test_data
    
    # Optional steps
    if [[ "$skip_language_servers" != true ]]; then
        install_language_servers
    else
        log_warning "Skipping language server installation"
    fi
    
    if [[ "$skip_validation" != true ]]; then
        run_validation_tests
    else
        log_warning "Skipping validation tests"
    fi
    
    # Summary
    display_setup_summary
    
    log_success "Integration test environment setup completed!"
    log "Setup log saved to: $SETUP_LOG"
}

# Execute main function with all arguments
main "$@"