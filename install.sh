#!/bin/bash
set -e

# LSP Gateway Installation Script
# This script compiles the Go binary and installs the npm package locally
#
# Usage:
#   ./install.sh           - Full installation
#   ./install.sh --dry-run - Show what would be done without executing
#   ./install.sh --help    - Show this help

# Parse command line arguments
DRY_RUN=false
SHOW_HELP=false

for arg in "$@"; do
    case $arg in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --help|-h)
            SHOW_HELP=true
            shift
            ;;
        *)
            echo "Unknown option: $arg"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

if [ "$SHOW_HELP" = true ]; then
    echo "LSP Gateway Installation Script"
    echo "==============================="
    echo ""
    echo "This script builds the Go binary and installs the npm package globally."
    echo ""
    echo "Usage:"
    echo "  ./install.sh           - Full installation"
    echo "  ./install.sh --dry-run - Show what would be done without executing"
    echo "  ./install.sh --help    - Show this help"
    echo ""
    echo "Prerequisites:"
    echo "  - Go 1.21 or later"
    echo "  - Node.js 22.0.0 or later"
    echo "  - npm"
    echo "  - make"
    echo ""
    echo "After installation, you can use 'lspg' command globally."
    exit 0
fi

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_dry_run() {
    echo -e "${YELLOW}[DRY-RUN]${NC} Would execute: $1"
}

# Function to execute command or show what would be executed
execute_or_dry_run() {
    if [ "$DRY_RUN" = true ]; then
        print_dry_run "$1"
    else
        eval "$1"
    fi
}

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to get version of a command
get_version() {
    $1 --version 2>/dev/null | head -n1 || echo "unknown"
}

print_info "🚀 LSP Gateway Installation Script"
print_info "==================================="

if [ "$DRY_RUN" = true ]; then
    print_warning "🔍 DRY-RUN MODE: Showing what would be done without executing"
fi
echo

# Check prerequisites
print_info "Checking prerequisites..."

# Check Go
if ! command_exists go; then
    print_error "Go is not installed. Please install Go 1.21 or later."
    print_info "Visit: https://golang.org/dl/"
    exit 1
fi

GO_VERSION=$(get_version go)
print_success "Go found: $GO_VERSION"

# Check Node.js
if ! command_exists node; then
    print_error "Node.js is not installed. Please install Node.js 22.0.0 or later."
    print_info "Visit: https://nodejs.org/"
    exit 1
fi

NODE_VERSION=$(get_version node)
print_success "Node.js found: $NODE_VERSION"

# Check npm
if ! command_exists npm; then
    print_error "npm is not installed. Please install npm."
    exit 1
fi

NPM_VERSION=$(get_version npm)
print_success "npm found: $NPM_VERSION"

# Check make
if ! command_exists make; then
    print_error "make is not installed. Please install make."
    exit 1
fi

print_success "make found"
echo

# Step 1: Clean previous builds
print_info "🧹 Cleaning previous builds..."
if [ -d "bin" ]; then
    execute_or_dry_run "rm -rf bin"
    if [ "$DRY_RUN" = false ]; then
        print_success "Cleaned bin directory"
    fi
fi

# Step 2: Download Go dependencies
print_info "📦 Downloading Go dependencies..."
execute_or_dry_run "go mod tidy"
if [ "$DRY_RUN" = false ]; then
    print_success "Go dependencies updated"
fi

# Step 3: Build Go binary
print_info "🔨 Building Go binary..."
execute_or_dry_run "make local VERSION=0.0.1"
if [ "$DRY_RUN" = false ]; then
    if [ ! -f "bin/lsp-gateway" ]; then
        print_error "Failed to build Go binary"
        exit 1
    fi
    print_success "Go binary built successfully"
fi

# Step 4: Make binary executable
execute_or_dry_run "chmod +x bin/lsp-gateway"
if [ "$DRY_RUN" = false ]; then
    print_success "Binary made executable"
fi

# Step 5: Test binary
print_info "🧪 Testing Go binary..."
if [ "$DRY_RUN" = true ]; then
    print_dry_run "./bin/lsp-gateway version"
else
    if ./bin/lsp-gateway version >/dev/null 2>&1; then
        BINARY_VERSION=$(./bin/lsp-gateway version 2>/dev/null | grep "Version:" | awk '{print $2}' || echo "0.0.1")
        print_success "Binary test passed - Version: $BINARY_VERSION"
    else
        print_error "Binary test failed"
        exit 1
    fi
fi

# Step 6: Install npm package globally
print_info "📥 Installing npm package globally..."

if [ "$DRY_RUN" = true ]; then
    print_dry_run "npm list -g lsp-gateway (check for existing installation)"
    print_dry_run "npm uninstall -g lsp-gateway (if needed)"
    print_dry_run "npm install -g ."
else
    # Check if package is already installed and uninstall it
    if npm list -g lsp-gateway >/dev/null 2>&1; then
        print_info "Uninstalling previous version..."
        npm uninstall -g lsp-gateway
    fi

    # Install the package globally
    npm install -g .
    if [ $? -ne 0 ]; then
        print_error "Failed to install npm package globally"
        print_info "You might need to run this script with sudo:"
        print_info "sudo ./install.sh"
        exit 1
    fi

    print_success "npm package installed globally"
fi

# Step 7: Test installation
print_info "🧪 Testing installation..."

if [ "$DRY_RUN" = true ]; then
    print_dry_run "command -v lspg (check if lspg command is available)"
    print_dry_run "lspg version (test lspg command)"
else
    # Check if lspg command is available
    if command_exists lspg; then
        print_success "lspg command is available"
        
        # Test lspg version command
        if lspg version >/dev/null 2>&1; then
            INSTALLED_VERSION=$(lspg version 2>/dev/null | grep "Version:" | awk '{print $2}' || echo "unknown")
            print_success "lspg version test passed - Version: $INSTALLED_VERSION"
        else
            print_warning "lspg command exists but version test failed"
        fi
    else
        print_error "lspg command not found in PATH"
        print_info "You may need to restart your terminal or source your shell profile"
        print_info "Alternatively, you can run: source ~/.bashrc or source ~/.zshrc"
        exit 1
    fi
fi

echo
print_success "🎉 Installation completed successfully!"
print_info "You can now use the 'lspg' command:"
print_info "  lspg version      - Show version information"
print_info "  lspg server       - Start LSP Gateway server"
print_info "  lspg diagnose     - Run diagnostics"
print_info "  lspg setup all    - Setup language servers"
echo
print_info "For more commands, run: lspg --help"
print_info "📖 Documentation: https://github.com/lsp-gateway/lsp-gateway"