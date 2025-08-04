#!/bin/bash
set -euo pipefail

# CI Test Environment Setup Script for LSP Gateway
# Sets up comprehensive testing environment with all dependencies

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Configuration
DEFAULT_GO_VERSION="1.24.x"
DEFAULT_NODE_VERSION="18"
DEFAULT_PYTHON_VERSION="3.11"
DEFAULT_JAVA_VERSION="17"

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

# Help function
show_help() {
    cat << EOF
LSP Gateway CI Test Environment Setup

Usage: $0 [OPTIONS] [COMPONENT]

COMPONENT:
    all             Setup all components (default)
    runtimes        Setup language runtimes only
    lsp-servers     Setup LSP servers only
    tools           Setup testing tools only
    workspace       Setup test workspace only

OPTIONS:
    -h, --help          Show this help message
    -v, --verbose       Enable verbose output
    --go-version VER    Go version to use (default: $DEFAULT_GO_VERSION)
    --node-version VER  Node.js version to use (default: $DEFAULT_NODE_VERSION)
    --python-version VER Python version to use (default: $DEFAULT_PYTHON_VERSION)
    --java-version VER  Java version to use (default: $DEFAULT_JAVA_VERSION)
    --skip-cache        Skip using cached installations
    --force             Force reinstallation of components
    --ci                Optimized setup for CI environments
    --local             Setup for local development

Examples:
    $0 all --ci                         # Full CI setup
    $0 runtimes --local                 # Setup runtimes for local dev
    $0 lsp-servers --force              # Force reinstall LSP servers

EOF
}

# Detect operating system and architecture
detect_platform() {
    local os arch
    
    case "$(uname -s)" in
        Linux*)     os="linux" ;;
        Darwin*)    os="darwin" ;;
        CYGWIN*|MINGW*|MSYS*) os="windows" ;;
        *)          os="unknown" ;;
    esac
    
    case "$(uname -m)" in
        x86_64)     arch="amd64" ;;
        arm64|aarch64) arch="arm64" ;;
        *)          arch="unknown" ;;
    esac
    
    echo "${os}-${arch}"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Setup Go runtime
setup_go_runtime() {
    log_info "Setting up Go runtime..."
    
    if command_exists go && [[ "$FORCE" != "true" ]]; then
        local current_version
        current_version=$(go version | grep -o 'go[0-9]\+\.[0-9]\+\.[0-9]\+' | sed 's/go//')
        log_info "Go $current_version already installed"
        
        # Verify minimum version
        if [[ "$(printf '%s\n' "1.20" "$current_version" | sort -V | head -n1)" = "1.20" ]]; then
            log_success "Go version is compatible"
            return 0
        else
            log_warning "Go version $current_version is too old, need 1.20+"
        fi
    fi
    
    if [[ "$CI" == "true" ]]; then
        log_info "In CI environment, Go should be pre-installed"
        if ! command_exists go; then
            log_error "Go not found in CI environment"
            exit 1
        fi
    else
        log_info "Setting up Go for local development..."
        
        local platform
        platform=$(detect_platform)
        
        case "$platform" in
            linux-amd64|darwin-amd64|darwin-arm64)
                # Download and install Go
                local go_version="1.24.5"  # Latest stable
                local go_archive="go${go_version}.${platform%-*}-${platform#*-}.tar.gz"
                local go_url="https://golang.org/dl/${go_archive}"
                
                log_info "Downloading Go $go_version for $platform..."
                curl -fsSL "$go_url" -o "/tmp/$go_archive"
                
                # Install to /usr/local or ~/go
                if [[ -w "/usr/local" ]]; then
                    sudo rm -rf /usr/local/go
                    sudo tar -C /usr/local -xzf "/tmp/$go_archive"
                    export PATH="/usr/local/go/bin:$PATH"
                else
                    rm -rf "$HOME/go-install"
                    mkdir -p "$HOME/go-install"
                    tar -C "$HOME/go-install" -xzf "/tmp/$go_archive"
                    export PATH="$HOME/go-install/go/bin:$PATH"
                fi
                
                rm "/tmp/$go_archive"
                ;;
            *)
                log_error "Unsupported platform for Go installation: $platform"
                exit 1
                ;;
        esac
    fi
    
    # Verify installation
    if command_exists go; then
        log_success "Go $(go version | cut -d' ' -f3) installed successfully"
    else
        log_error "Go installation failed"
        exit 1
    fi
}

# Setup Node.js runtime
setup_node_runtime() {
    log_info "Setting up Node.js runtime..."
    
    if command_exists node && [[ "$FORCE" != "true" ]]; then
        local current_version
        current_version=$(node --version | sed 's/v//')
        log_info "Node.js $current_version already installed"
        
        # Verify minimum version
        if [[ "$(printf '%s\n' "16.0.0" "$current_version" | sort -V | head -n1)" = "16.0.0" ]]; then
            log_success "Node.js version is compatible"
            return 0
        else
            log_warning "Node.js version $current_version is too old, need 16+"
        fi
    fi
    
    if [[ "$CI" == "true" ]]; then
        log_info "In CI environment, Node.js should be pre-installed"
        if ! command_exists node; then
            log_error "Node.js not found in CI environment"
            exit 1
        fi
    else
        log_info "Setting up Node.js for local development..."
        
        # Use nvm if available, otherwise direct installation
        if [[ -f "$HOME/.nvm/nvm.sh" ]]; then
            source "$HOME/.nvm/nvm.sh"
            nvm install "$NODE_VERSION"
            nvm use "$NODE_VERSION"
        else
            log_warning "nvm not found, attempting direct installation..."
            # This is platform-specific and complex, recommend using nvm
            log_error "Please install Node.js $NODE_VERSION manually or use nvm"
            exit 1
        fi
    fi
    
    # Verify installation
    if command_exists node && command_exists npm; then
        log_success "Node.js $(node --version) and npm $(npm --version) installed successfully"
    else
        log_error "Node.js installation failed"
        exit 1
    fi
}

# Setup Python runtime
setup_python_runtime() {
    log_info "Setting up Python runtime..."
    
    local python_cmd="python3"
    
    if command_exists "$python_cmd" && [[ "$FORCE" != "true" ]]; then
        local current_version
        current_version=$($python_cmd --version 2>&1 | grep -o '[0-9]\+\.[0-9]\+\.[0-9]\+')
        log_info "Python $current_version already installed"
        
        # Verify minimum version
        if [[ "$(printf '%s\n' "3.8.0" "$current_version" | sort -V | head -n1)" = "3.8.0" ]]; then
            log_success "Python version is compatible"
            return 0
        else
            log_warning "Python version $current_version is too old, need 3.8+"
        fi
    fi
    
    if [[ "$CI" == "true" ]]; then
        log_info "In CI environment, Python should be pre-installed"
        if ! command_exists "$python_cmd"; then
            log_error "Python not found in CI environment"
            exit 1
        fi
    else
        log_info "Setting up Python for local development..."
        log_warning "Python installation varies by platform. Please install Python $PYTHON_VERSION manually."
        
        # Check if pyenv is available
        if command_exists pyenv; then
            log_info "Using pyenv to install Python $PYTHON_VERSION"
            pyenv install "$PYTHON_VERSION" || log_warning "pyenv install failed, continuing with system Python"
            pyenv global "$PYTHON_VERSION" || true
        fi
    fi
    
    # Verify installation
    if command_exists "$python_cmd" && command_exists pip; then
        log_success "Python $($python_cmd --version 2>&1) and pip installed successfully"
    else
        log_error "Python installation verification failed"
        exit 1
    fi
}

# Setup Java runtime
setup_java_runtime() {
    log_info "Setting up Java runtime..."
    
    if command_exists java && [[ "$FORCE" != "true" ]]; then
        local current_version
        current_version=$(java -version 2>&1 | head -n1 | grep -o '[0-9]\+\.[0-9]\+' | head -n1)
        log_info "Java $current_version already installed"
        
        # Accept Java 11+
        if [[ "$(printf '%s\n' "11.0" "$current_version" | sort -V | head -n1)" = "11.0" ]]; then
            log_success "Java version is compatible"
            return 0
        else
            log_warning "Java version $current_version is too old, need 11+"
        fi
    fi
    
    if [[ "$CI" == "true" ]]; then
        log_info "In CI environment, Java should be pre-installed"
        if ! command_exists java; then
            log_error "Java not found in CI environment"
            exit 1
        fi
    else
        log_info "Setting up Java for local development..."
        
        local platform
        platform=$(detect_platform)
        
        case "$platform" in
            linux-*)
                # Use package manager
                if command_exists apt-get; then
                    sudo apt-get update
                    sudo apt-get install -y openjdk-17-jdk
                elif command_exists yum; then
                    sudo yum install -y java-17-openjdk-devel
                elif command_exists dnf; then
                    sudo dnf install -y java-17-openjdk-devel
                else
                    log_error "No supported package manager found for Java installation"
                    exit 1
                fi
                ;;
            darwin-*)
                # Use Homebrew if available
                if command_exists brew; then
                    brew install openjdk@17
                    sudo ln -sfn /usr/local/opt/openjdk@17/libexec/openjdk.jdk /Library/Java/JavaVirtualMachines/openjdk-17.jdk
                else
                    log_error "Homebrew not found. Please install Java 17 manually on macOS"
                    exit 1
                fi
                ;;
            *)
                log_error "Unsupported platform for Java installation: $platform"
                exit 1
                ;;
        esac
    fi
    
    # Verify installation
    if command_exists java && command_exists javac; then
        log_success "Java $(java -version 2>&1 | head -n1) installed successfully"
    else
        log_error "Java installation failed"
        exit 1
    fi
}

# Setup LSP servers
setup_lsp_servers() {
    log_info "Setting up LSP servers..."
    
    # Go LSP server (gopls)
    log_info "Installing Go LSP server (gopls)..."
    if ! command_exists gopls || [[ "$FORCE" == "true" ]]; then
        go install golang.org/x/tools/gopls@latest
        log_success "gopls installed"
    else
        log_info "gopls already installed"
    fi
    
    # Python LSP server
    log_info "Installing Python LSP server..."
    if ! command_exists pyright-langserver || [[ "$FORCE" == "true" ]]; then
        npm install -g pyright
        log_success "pyright installed"
    else
        log_info "pyright already installed"
    fi
    
    # TypeScript/JavaScript LSP server
    log_info "Installing TypeScript LSP server..."
    if ! command_exists typescript-language-server || [[ "$FORCE" == "true" ]]; then
        npm install -g typescript-language-server typescript
        log_success "typescript-language-server installed"
    else
        log_info "typescript-language-server already installed"
    fi
    
    # Java LSP server (Eclipse JDT LS)
    log_info "Setting up Java LSP server..."
    local jdtls_dir="$HOME/.local/share/eclipse.jdt.ls"
    
    if [[ ! -d "$jdtls_dir" ]] || [[ "$FORCE" == "true" ]]; then
        mkdir -p "$jdtls_dir"
        
        # Download latest JDT LS
        local jdtls_version="1.26.0"  # Update as needed
        local jdtls_url="https://download.eclipse.org/jdtls/milestones/${jdtls_version}/jdt-language-server-${jdtls_version}-202307271613.tar.gz"
        
        log_info "Downloading JDT Language Server..."
        curl -fsSL "$jdtls_url" | tar -xzC "$jdtls_dir"
        
        log_success "Eclipse JDT LS installed to $jdtls_dir"
    else
        log_info "Eclipse JDT LS already installed"
    fi
    
    log_success "All LSP servers setup completed"
}

# Setup testing tools
setup_testing_tools() {
    log_info "Setting up testing tools..."
    
    # Go testing tools
    local go_tools=(
        "github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
        "github.com/securego/gosec/v2/cmd/gosec@latest"
        "golang.org/x/tools/cmd/deadcode@latest"
        "github.com/jstemmer/go-junit-report@latest"
        "golang.org/x/perf/cmd/benchstat@latest"
        "github.com/axw/gocov/gocov@latest"
        "github.com/matm/gocov-html@latest"
    )
    
    for tool in "${go_tools[@]}"; do
        local tool_name
        tool_name=$(basename "$tool" | cut -d'@' -f1)
        
        if ! command_exists "$tool_name" || [[ "$FORCE" == "true" ]]; then
            log_info "Installing $tool_name..."
            go install "$tool"
        else
            log_info "$tool_name already installed"
        fi
    done
    
    log_success "Testing tools setup completed"
}

# Setup test workspace
setup_test_workspace() {
    log_info "Setting up test workspace..."
    
    local workspace_dir="$PROJECT_ROOT/test-workspace"
    
    if [[ "$FORCE" == "true" ]] || [[ ! -d "$workspace_dir" ]]; then
        rm -rf "$workspace_dir"
        mkdir -p "$workspace_dir"
        
        # Create comprehensive test projects
        create_go_test_project "$workspace_dir/go-project"
        create_python_test_project "$workspace_dir/python-project"
        create_typescript_test_project "$workspace_dir/typescript-project"
        create_java_test_project "$workspace_dir/java-project"
        
        log_success "Test workspace created at $workspace_dir"
    else
        log_info "Test workspace already exists"
    fi
}

# Create Go test project
create_go_test_project() {
    local project_dir="$1"
    mkdir -p "$project_dir"
    
    # go.mod
    cat > "$project_dir/go.mod" << 'EOF'
module test-project

go 1.24

require (
    github.com/stretchr/testify v1.8.4
)
EOF
    
    # main.go
    cat > "$project_dir/main.go" << 'EOF'
package main

import (
    "fmt"
    "net/http"
    "log"
)

// Calculator provides mathematical operations
type Calculator struct {
    history []Operation
}

// Operation represents a mathematical operation
type Operation struct {
    Type   string
    A, B   float64
    Result float64
}

// Add performs addition
func (c *Calculator) Add(a, b float64) float64 {
    result := a + b
    c.history = append(c.history, Operation{
        Type: "add", A: a, B: b, Result: result,
    })
    return result
}

// Multiply performs multiplication
func (c *Calculator) Multiply(a, b float64) float64 {
    result := a * b
    c.history = append(c.history, Operation{
        Type: "multiply", A: a, B: b, Result: result,
    })
    return result
}

// GetHistory returns calculation history
func (c *Calculator) GetHistory() []Operation {
    return c.history
}

func main() {
    calc := &Calculator{}
    
    http.HandleFunc("/add", func(w http.ResponseWriter, r *http.Request) {
        result := calc.Add(5, 3)
        fmt.Fprintf(w, "Result: %.2f", result)
    })
    
    http.HandleFunc("/multiply", func(w http.ResponseWriter, r *http.Request) {
        result := calc.Multiply(4, 7)
        fmt.Fprintf(w, "Result: %.2f", result)
    })
    
    log.Println("Server starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
EOF
    
    # calculator_test.go
    cat > "$project_dir/calculator_test.go" << 'EOF'
package main

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestCalculator_Add(t *testing.T) {
    calc := &Calculator{}
    
    result := calc.Add(2, 3)
    assert.Equal(t, 5.0, result)
    
    history := calc.GetHistory()
    assert.Len(t, history, 1)
    assert.Equal(t, "add", history[0].Type)
}

func TestCalculator_Multiply(t *testing.T) {
    calc := &Calculator{}
    
    result := calc.Multiply(4, 5)
    assert.Equal(t, 20.0, result)
    
    history := calc.GetHistory()
    assert.Len(t, history, 1)
    assert.Equal(t, "multiply", history[0].Type)
}

func BenchmarkCalculator_Add(b *testing.B) {
    calc := &Calculator{}
    for i := 0; i < b.N; i++ {
        calc.Add(float64(i), float64(i+1))
    }
}
EOF
}

# Create Python test project
create_python_test_project() {
    local project_dir="$1"
    mkdir -p "$project_dir"
    
    # main.py
    cat > "$project_dir/main.py" << 'EOF'
#!/usr/bin/env python3
"""
Calculator module for testing LSP functionality
"""

from typing import List, Dict, Union
import json

class Operation:
    """Represents a mathematical operation"""
    
    def __init__(self, op_type: str, a: float, b: float, result: float):
        self.type = op_type
        self.a = a
        self.b = b
        self.result = result
    
    def to_dict(self) -> Dict[str, Union[str, float]]:
        return {
            'type': self.type,
            'a': self.a,
            'b': self.b,
            'result': self.result
        }

class Calculator:
    """Calculator with operation history"""
    
    def __init__(self):
        self.history: List[Operation] = []
    
    def add(self, a: float, b: float) -> float:
        """Add two numbers"""
        result = a + b
        self.history.append(Operation('add', a, b, result))
        return result
    
    def multiply(self, a: float, b: float) -> float:
        """Multiply two numbers"""
        result = a * b
        self.history.append(Operation('multiply', a, b, result))
        return result
    
    def get_history(self) -> List[Dict[str, Union[str, float]]]:
        """Get calculation history"""
        return [op.to_dict() for op in self.history]
    
    def clear_history(self) -> None:
        """Clear calculation history"""
        self.history.clear()

def main():
    """Main function for testing"""
    calc = Calculator()
    
    print("Calculator Test")
    print(f"5 + 3 = {calc.add(5, 3)}")
    print(f"4 * 7 = {calc.multiply(4, 7)}")
    
    print("\nHistory:")
    print(json.dumps(calc.get_history(), indent=2))

if __name__ == "__main__":
    main()
EOF
    
    # test_calculator.py
    cat > "$project_dir/test_calculator.py" << 'EOF'
import unittest
from main import Calculator, Operation

class TestCalculator(unittest.TestCase):
    
    def setUp(self):
        self.calc = Calculator()
    
    def test_add(self):
        result = self.calc.add(2, 3)
        self.assertEqual(result, 5.0)
        
        history = self.calc.get_history()
        self.assertEqual(len(history), 1)
        self.assertEqual(history[0]['type'], 'add')
    
    def test_multiply(self):
        result = self.calc.multiply(4, 5)
        self.assertEqual(result, 20.0)
        
        history = self.calc.get_history()
        self.assertEqual(len(history), 1)
        self.assertEqual(history[0]['type'], 'multiply')
    
    def test_clear_history(self):
        self.calc.add(1, 2)
        self.calc.multiply(3, 4)
        self.assertEqual(len(self.calc.get_history()), 2)
        
        self.calc.clear_history()
        self.assertEqual(len(self.calc.get_history()), 0)

if __name__ == '__main__':
    unittest.main()
EOF
    
    # requirements.txt
    cat > "$project_dir/requirements.txt" << 'EOF'
typing-extensions>=4.0.0
EOF
}

# Create TypeScript test project
create_typescript_test_project() {
    local project_dir="$1"
    mkdir -p "$project_dir"
    
    # package.json
    cat > "$project_dir/package.json" << 'EOF'
{
  "name": "test-project",
  "version": "1.0.0",
  "description": "Test project for LSP validation",
  "main": "dist/main.js",
  "scripts": {
    "build": "tsc",
    "start": "node dist/main.js",
    "test": "jest"
  },
  "devDependencies": {
    "@types/node": "^18.0.0",
    "@types/jest": "^29.0.0",
    "typescript": "^5.0.0",
    "jest": "^29.0.0",
    "ts-jest": "^29.0.0"
  }
}
EOF
    
    # tsconfig.json
    cat > "$project_dir/tsconfig.json" << 'EOF'
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "commonjs",
    "outDir": "./dist",
    "rootDir": "./src",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist"]
}
EOF
    
    mkdir -p "$project_dir/src"
    
    # src/main.ts
    cat > "$project_dir/src/main.ts" << 'EOF'
interface Operation {
    type: string;
    a: number;
    b: number;
    result: number;
}

class Calculator {
    private history: Operation[] = [];
    
    add(a: number, b: number): number {
        const result = a + b;
        this.history.push({ type: 'add', a, b, result });
        return result;
    }
    
    multiply(a: number, b: number): number {
        const result = a * b;
        this.history.push({ type: 'multiply', a, b, result });
        return result;
    }
    
    getHistory(): Operation[] {
        return [...this.history];
    }
    
    clearHistory(): void {
        this.history = [];
    }
}

export { Calculator, Operation };

// Main execution
if (require.main === module) {
    const calc = new Calculator();
    
    console.log('Calculator Test');
    console.log(`5 + 3 = ${calc.add(5, 3)}`);
    console.log(`4 * 7 = ${calc.multiply(4, 7)}`);
    
    console.log('\nHistory:');
    console.log(JSON.stringify(calc.getHistory(), null, 2));
}
EOF
    
    # src/calculator.test.ts
    cat > "$project_dir/src/calculator.test.ts" << 'EOF'
import { Calculator } from './main';

describe('Calculator', () => {
    let calc: Calculator;
    
    beforeEach(() => {
        calc = new Calculator();
    });
    
    test('should add two numbers', () => {
        const result = calc.add(2, 3);
        expect(result).toBe(5);
        
        const history = calc.getHistory();
        expect(history).toHaveLength(1);
        expect(history[0].type).toBe('add');
    });
    
    test('should multiply two numbers', () => {
        const result = calc.multiply(4, 5);
        expect(result).toBe(20);
        
        const history = calc.getHistory();
        expect(history).toHaveLength(1);
        expect(history[0].type).toBe('multiply');
    });
    
    test('should clear history', () => {
        calc.add(1, 2);
        calc.multiply(3, 4);
        expect(calc.getHistory()).toHaveLength(2);
        
        calc.clearHistory();
        expect(calc.getHistory()).toHaveLength(0);
    });
});
EOF
}

# Create Java test project
create_java_test_project() {
    local project_dir="$1"
    mkdir -p "$project_dir/src/main/java/com/example"
    mkdir -p "$project_dir/src/test/java/com/example"
    
    # pom.xml (Maven project)
    cat > "$project_dir/pom.xml" << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 
         http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>
    
    <groupId>com.example</groupId>
    <artifactId>test-project</artifactId>
    <version>1.0.0</version>
    <packaging>jar</packaging>
    
    <properties>
        <maven.compiler.source>17</maven.compiler.source>
        <maven.compiler.target>17</maven.compiler.target>
        <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
    </properties>
    
    <dependencies>
        <dependency>
            <groupId>org.junit.jupiter</groupId>
            <artifactId>junit-jupiter</artifactId>
            <version>5.9.0</version>
            <scope>test</scope>
        </dependency>
    </dependencies>
    
    <build>
        <plugins>
            <plugin>
                <groupId>org.apache.maven.plugins</groupId>
                <artifactId>maven-surefire-plugin</artifactId>
                <version>3.0.0-M9</version>
            </plugin>
        </plugins>
    </build>
</project>
EOF
    
    # Main Calculator class
    cat > "$project_dir/src/main/java/com/example/Calculator.java" << 'EOF'
package com.example;

import java.util.ArrayList;
import java.util.List;

/**
 * Calculator with operation history
 */
public class Calculator {
    private final List<Operation> history = new ArrayList<>();
    
    /**
     * Add two numbers
     */
    public double add(double a, double b) {
        double result = a + b;
        history.add(new Operation("add", a, b, result));
        return result;
    }
    
    /**
     * Multiply two numbers
     */
    public double multiply(double a, double b) {
        double result = a * b;
        history.add(new Operation("multiply", a, b, result));
        return result;
    }
    
    /**
     * Get calculation history
     */
    public List<Operation> getHistory() {
        return new ArrayList<>(history);
    }
    
    /**
     * Clear calculation history
     */
    public void clearHistory() {
        history.clear();
    }
    
    /**
     * Main method for testing
     */
    public static void main(String[] args) {
        Calculator calc = new Calculator();
        
        System.out.println("Calculator Test");
        System.out.println("5 + 3 = " + calc.add(5, 3));
        System.out.println("4 * 7 = " + calc.multiply(4, 7));
        
        System.out.println("\nHistory:");
        calc.getHistory().forEach(System.out::println);
    }
}
EOF
    
    # Operation class
    cat > "$project_dir/src/main/java/com/example/Operation.java" << 'EOF'
package com.example;

/**
 * Represents a mathematical operation
 */
public class Operation {
    private final String type;
    private final double a;
    private final double b;
    private final double result;
    
    public Operation(String type, double a, double b, double result) {
        this.type = type;
        this.a = a;
        this.b = b;
        this.result = result;
    }
    
    public String getType() { return type; }
    public double getA() { return a; }
    public double getB() { return b; }
    public double getResult() { return result; }
    
    @Override
    public String toString() {
        return String.format("Operation{type='%s', a=%.2f, b=%.2f, result=%.2f}", 
                           type, a, b, result);
    }
}
EOF
    
    # Test class
    cat > "$project_dir/src/test/java/com/example/CalculatorTest.java" << 'EOF'
package com.example;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

class CalculatorTest {
    private Calculator calc;
    
    @BeforeEach
    void setUp() {
        calc = new Calculator();
    }
    
    @Test
    void testAdd() {
        double result = calc.add(2, 3);
        assertEquals(5.0, result, 0.001);
        
        var history = calc.getHistory();
        assertEquals(1, history.size());
        assertEquals("add", history.get(0).getType());
    }
    
    @Test
    void testMultiply() {
        double result = calc.multiply(4, 5);
        assertEquals(20.0, result, 0.001);
        
        var history = calc.getHistory();
        assertEquals(1, history.size());
        assertEquals("multiply", history.get(0).getType());
    }
    
    @Test
    void testClearHistory() {
        calc.add(1, 2);
        calc.multiply(3, 4);
        assertEquals(2, calc.getHistory().size());
        
        calc.clearHistory();
        assertEquals(0, calc.getHistory().size());
    }
}
EOF
}

# Make all scripts executable
make_scripts_executable() {
    log_info "Making CI scripts executable..."
    
    find "$SCRIPT_DIR" -name "*.sh" -type f -exec chmod +x {} \;
    
    log_success "CI scripts are now executable"
}

# Display setup summary
display_summary() {
    log_info "Setup Summary:"
    echo "=========================="
    
    # Check Go
    if command_exists go; then
        echo "✅ Go: $(go version | cut -d' ' -f3)"
    else
        echo "❌ Go: Not installed"
    fi
    
    # Check Node.js
    if command_exists node; then
        echo "✅ Node.js: $(node --version)"
    else
        echo "❌ Node.js: Not installed"
    fi
    
    # Check Python
    if command_exists python3; then
        echo "✅ Python: $(python3 --version 2>&1)"
    else
        echo "❌ Python: Not installed"
    fi
    
    # Check Java
    if command_exists java; then
        echo "✅ Java: $(java -version 2>&1 | head -n1)"
    else
        echo "❌ Java: Not installed"
    fi
    
    # Check LSP servers
    echo ""
    echo "LSP Servers:"
    if command_exists gopls; then
        echo "✅ gopls: $(gopls version | head -n1)"
    else
        echo "❌ gopls: Not installed"
    fi
    
    if command_exists pyright-langserver; then
        echo "✅ pyright: Installed"
    else
        echo "❌ pyright: Not installed"
    fi
    
    if command_exists typescript-language-server; then
        echo "✅ typescript-language-server: $(typescript-language-server --version)"
    else
        echo "❌ typescript-language-server: Not installed"
    fi
    
    if [[ -d "$HOME/.local/share/eclipse.jdt.ls" ]]; then
        echo "✅ Eclipse JDT LS: Installed"
    else
        echo "❌ Eclipse JDT LS: Not installed"
    fi
    
    echo "=========================="
    log_success "Setup completed!"
}

# Main execution function
main() {
    case "$COMPONENT" in
        "runtimes")
            setup_go_runtime
            setup_node_runtime
            setup_python_runtime
            setup_java_runtime
            ;;
        "lsp-servers")
            setup_lsp_servers
            ;;
        "tools")
            setup_testing_tools
            ;;
        "workspace")
            setup_test_workspace
            ;;
        "all")
            setup_go_runtime
            setup_node_runtime
            setup_python_runtime
            setup_java_runtime
            setup_lsp_servers
            setup_testing_tools
            setup_test_workspace
            make_scripts_executable
            ;;
        *)
            log_error "Unknown component: $COMPONENT"
            show_help
            exit 1
            ;;
    esac
    
    display_summary
}

# Parse command line arguments
COMPONENT="all"
GO_VERSION="$DEFAULT_GO_VERSION"
NODE_VERSION="$DEFAULT_NODE_VERSION"
PYTHON_VERSION="$DEFAULT_PYTHON_VERSION"
JAVA_VERSION="$DEFAULT_JAVA_VERSION"
SKIP_CACHE=false
FORCE=false
CI=false
VERBOSE=false

# Detect CI environment
if [[ "${GITHUB_ACTIONS:-false}" == "true" ]] || [[ "${CI:-false}" == "true" ]]; then
    CI=true
fi

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -v|--verbose)
            VERBOSE=true
            set -x
            shift
            ;;
        --go-version)
            GO_VERSION="$2"
            shift 2
            ;;
        --node-version)
            NODE_VERSION="$2"
            shift 2
            ;;
        --python-version)
            PYTHON_VERSION="$2"
            shift 2
            ;;
        --java-version)
            JAVA_VERSION="$2"
            shift 2
            ;;
        --skip-cache)
            SKIP_CACHE=true
            shift
            ;;
        --force)
            FORCE=true
            shift
            ;;
        --ci)
            CI=true
            shift
            ;;
        --local)
            CI=false
            shift
            ;;
        all|runtimes|lsp-servers|tools|workspace)
            COMPONENT="$1"
            shift
            ;;
        *)
            log_error "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Run main function
main "$@"