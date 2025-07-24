# Makefile for LSP Gateway
# Multi-platform build system

# Configuration
BINARY_NAME := lsp-gateway
MAIN_PATH := cmd/lsp-gateway/main.go
BUILD_DIR := bin
VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOCLEAN := $(GOCMD) clean
GOMOD := $(GOCMD) mod

# Build targets
PLATFORMS := linux/amd64 windows/amd64 darwin/amd64 darwin/arm64
BINARIES := $(BUILD_DIR)/$(BINARY_NAME)-linux \
            $(BUILD_DIR)/$(BINARY_NAME)-windows.exe \
            $(BUILD_DIR)/$(BINARY_NAME)-macos \
            $(BUILD_DIR)/$(BINARY_NAME)-macos-arm64

# Default target
.PHONY: all
all: build

# Help target
.PHONY: help
help:
	@echo "LSP Gateway Build System"
	@echo "========================"
	@echo ""
	@echo "Available targets:"
	@echo "  all          - Build for all platforms (default)"
	@echo "  build        - Build for all platforms"
	@echo "  linux        - Build for Linux (amd64)"
	@echo "  windows      - Build for Windows (amd64)"
	@echo "  macos        - Build for macOS (amd64)"
	@echo "  macos-arm64  - Build for macOS (arm64)"
	@echo "  local        - Build for current platform"
	@echo "  clean        - Clean build artifacts"
	@echo "  test         - Run all tests"
	@echo "  test-unit    - Run fast unit tests (<60s, excludes integration/performance)"
	@echo "  test-integration - Run integration and performance tests"
	@echo "  test-cover   - Run tests with coverage"
	@echo "  lsp-help     - Show LSP testing targets and options"
	@echo "  deps         - Download dependencies"
	@echo "  tidy         - Tidy go modules"
	@echo "  format       - Format code"
	@echo "  lint         - Run linter (requires golangci-lint)"
	@echo "  deadcode     - Run dead code analysis (requires deadcode tool)"
	@echo "  lint-unused  - Run only unused code detection"
	@echo "  check-deadcode - Complete dead code analysis (both tools)"
	@echo "  deadcode-report - Generate timestamped dead code report"
	@echo "  security     - Run security analysis (requires gosec)"
	@echo "  security-report - Generate timestamped security report"
	@echo "  security-check - Quick security check (strict mode for CI)"
	@echo "  security-full - Comprehensive security analysis (all formats)"
	@echo "  install      - Install binary to GOPATH/bin"
	@echo "  release      - Create release build with version"
	@echo "  help         - Show this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  VERSION      - Set version (default: dev)"
	@echo ""
	@echo "Quick Start - LSP Testing (Simplified):"
	@echo "  make setup-simple-repos          # Setup simple test repositories"
	@echo "  make test-simple-quick           # Quick LSP tests with simple config"
	@echo "  make test-simple-full            # Full LSP tests with simple config"
	@echo "  make simple-status               # Show simple repository status"
	@echo "  make simple-clean                # Clean simple test repositories"
	@echo "  make lsp-help                    # Show LSP testing help"
	@echo ""
	@echo "Examples:"
	@echo "  make build VERSION=v1.0.0"
	@echo "  make linux"
	@echo "  make clean && make all"

# Create build directory
$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

# Build for all platforms
.PHONY: build
build: $(BINARIES)

# Individual platform targets
.PHONY: linux
linux: $(BUILD_DIR)/$(BINARY_NAME)-linux

.PHONY: windows
windows: $(BUILD_DIR)/$(BINARY_NAME)-windows.exe

.PHONY: macos
macos: $(BUILD_DIR)/$(BINARY_NAME)-macos

.PHONY: macos-arm64
macos-arm64: $(BUILD_DIR)/$(BINARY_NAME)-macos-arm64

# Build rules for each platform
$(BUILD_DIR)/$(BINARY_NAME)-linux: $(BUILD_DIR)
	@echo "Building for Linux (amd64)..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $@ $(MAIN_PATH)

$(BUILD_DIR)/$(BINARY_NAME)-windows.exe: $(BUILD_DIR)
	@echo "Building for Windows (amd64)..."
	GOOS=windows GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $@ $(MAIN_PATH)

$(BUILD_DIR)/$(BINARY_NAME)-macos: $(BUILD_DIR)
	@echo "Building for macOS (amd64)..."
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $@ $(MAIN_PATH)

$(BUILD_DIR)/$(BINARY_NAME)-macos-arm64: $(BUILD_DIR)
	@echo "Building for macOS (arm64)..."
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $@ $(MAIN_PATH)

# Build for current platform
.PHONY: local
local: $(BUILD_DIR)
	@echo "Building for current platform..."
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	$(GOCLEAN)

# Test targets
.PHONY: test
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

.PHONY: test-unit
test-unit:
	@echo "Running fast unit tests (core functionality only)..."
	$(GOTEST) -v -short -timeout=30s ./tests/unit/internal/config -run "^Test(Default|Load|Validate|Server)"
	$(GOTEST) -v -short -timeout=30s ./tests/unit/internal/cli -run "^Test(Root|Config|Version|Help|Status|Completion)"
	$(GOTEST) -v -short -timeout=30s ./tests/unit/internal/setup -run "^Test(Config|Detector|Error)" 
	@echo "Running basic component tests..."
	$(GOTEST) -v -short -timeout=30s ./tests/unit/internal/gateway -run "^Test(Router|Handler|Extract)$$"
	$(GOTEST) -v -short -timeout=30s ./tests/unit/internal/platform -run "^Test(Platform|Executor|Package)$$" 
	$(GOTEST) -v -short -timeout=30s ./tests/unit/internal/transport -run "^Test(STDIO|TCP)$$" 
	$(GOTEST) -v -short -timeout=30s ./tests/unit/mcp -run "^Test(Client|Server|Protocol)$$"

.PHONY: test-integration
test-integration:
	@echo "Running integration and performance tests..."
	$(GOTEST) -v -timeout=300s ./tests/unit/internal/platform ./tests/integration/packagemgr_comprehensive_test.go -run "GCBehavior|AllocationRate|GCPause|MemoryAllocation|Comprehensive|Coverage|Extended|Final|GCEfficiency|ResourcePool"
	$(GOTEST) -v -timeout=300s ./tests/unit/internal/transport ./tests/integration/transport ./tests/stress/memory ./tests/stress/network -run "LongRunning|MemoryGrowth|MemoryLeak|Performance|Integration|NetworkTimeout|FileHandle|CircuitBreaker"
	$(GOTEST) -v -timeout=300s ./tests/unit/mcp ./tests/integration/mcp ./tests/benchmark/mcp -run "Performance|Throughput|Integration|NetworkTimeout"
	$(GOTEST) -v -timeout=300s ./tests/unit/internal/gateway ./tests/integration/gateway ./tests/benchmark/gateway ./tests/stress/memory ./tests/stress/network ./tests/stress/reliability -run "Integration|E2E|Performance|Memory|Connection|Network|Retry"

.PHONY: test-cover
test-cover:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -cover ./...

# LSP validation test targets
.PHONY: test-lsp-validation
test-lsp-validation:
	@echo "Running comprehensive LSP validation tests..."
	$(GOTEST) -v -timeout=180s ./tests/unit/internal/gateway ./tests/integration/gateway ./tests/unit/internal/transport ./tests/integration/transport ./tests/unit/mcp ./tests/integration/mcp ./tests/unit/internal/setup ./tests/integration/setup -run "LSPValidation"

.PHONY: test-lsp-validation-short
test-lsp-validation-short:
	@echo "Running quick LSP validation tests for CI..."
	$(GOTEST) -v -short -timeout=60s ./tests/unit/internal/gateway ./tests/integration/gateway ./tests/unit/internal/transport ./tests/integration/transport ./tests/unit/mcp ./tests/integration/mcp -run "LSPValidation"

.PHONY: test-lsp-repositories
test-lsp-repositories:
	@echo "Running repository-focused LSP validation tests..."
	$(GOTEST) -v -timeout=120s ./tests/unit/internal/gateway ./tests/integration/gateway ./tests/unit/internal/transport ./tests/integration/transport ./tests/unit/mcp ./tests/integration/mcp -run "LSPRepository"

.PHONY: bench-lsp-validation
bench-lsp-validation:
	@echo "Running LSP validation performance benchmarks..."
	$(GOTEST) -v -bench=LSPValidation -benchmem -timeout=300s ./tests/benchmark/gateway ./tests/benchmark/transport ./tests/benchmark/mcp

# LSP validation test script targets
.PHONY: test-lsp-validation-full
test-lsp-validation-full:
	@echo "Running full LSP validation test suite with reporting..."
	./tests/scripts/execution/run-lsp-validation-tests.sh full

.PHONY: test-lsp-validation-ci
test-lsp-validation-ci:
	@echo "Running CI-friendly LSP validation test suite..."
	./tests/scripts/execution/run-lsp-validation-tests.sh ci

# Coverage targets
.PHONY: test-coverage-threshold
test-coverage-threshold:
	@echo "Running coverage check with threshold..."
	./tests/scripts/coverage/coverage-simple.sh

.PHONY: coverage-ci
coverage-ci:
	@echo "Generating CI coverage reports..."
	./tests/scripts/coverage/coverage-simple.sh

# Dependency management
.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	$(GOGET) -v ./...

.PHONY: tidy
tidy:
	@echo "Tidying go modules..."
	$(GOMOD) tidy

# Code quality
.PHONY: format
format:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

.PHONY: lint
lint:
	@echo "Running linter..."
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not found. Install it with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; exit 1; }
	golangci-lint run ./...

# Enhanced dead code detection
.PHONY: deadcode
deadcode:
	@echo "Running comprehensive dead code analysis..."
	@command -v deadcode >/dev/null 2>&1 || { echo "deadcode not found. Install it with: go install golang.org/x/tools/cmd/deadcode@latest"; exit 1; }
	deadcode -test ./...

# Dead code detection with detailed output
.PHONY: deadcode-report
deadcode-report:
	@echo "Generating dead code analysis report..."
	@command -v deadcode >/dev/null 2>&1 || { echo "deadcode not found. Install it with: go install golang.org/x/tools/cmd/deadcode@latest"; exit 1; }
	@mkdir -p reports
	deadcode -test ./... > reports/deadcode-$(shell date +%Y%m%d-%H%M%S).txt 2>&1 || true
	@echo "Report saved to reports/deadcode-$(shell date +%Y%m%d-%H%M%S).txt"

# Focused unused code detection via golangci-lint
.PHONY: lint-unused
lint-unused:
	@echo "Running unused code detection..."
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not found. Install it with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; exit 1; }
	golangci-lint run --enable-only unused,ineffassign ./...

# Complete dead code analysis (combines both tools)
.PHONY: check-deadcode
check-deadcode:
	@echo "=== Comprehensive Dead Code Analysis ==="
	@echo "1. Running golangci-lint unused detection..."
	@$(MAKE) lint-unused || true
	@echo ""
	@echo "2. Running official deadcode analysis..."
	@$(MAKE) deadcode || true
	@echo ""
	@echo "=== Analysis Complete ==="

# Security analysis
.PHONY: security
security:
	@echo "Running security analysis..."
	@command -v gosec >/dev/null 2>&1 || { echo "gosec not found. Install it with: go install github.com/securego/gosec/v2/cmd/gosec@latest"; exit 1; }
	gosec -conf .gosec.json ./cmd/lsp-gateway ./internal/cli ./internal/common ./internal/config ./internal/gateway ./internal/installer ./internal/platform ./internal/setup ./internal/transport ./internal/types ./mcp

# Security analysis with detailed report
.PHONY: security-report
security-report:
	@echo "Generating security analysis report..."
	@command -v gosec >/dev/null 2>&1 || { echo "gosec not found. Install it with: go install github.com/securego/gosec/v2/cmd/gosec@latest"; exit 1; }
	@mkdir -p reports
	gosec -conf .gosec.json -fmt json -out reports/security-$(shell date +%Y%m%d-%H%M%S).json ./... || true
	gosec -conf .gosec.json -fmt text -out reports/security-$(shell date +%Y%m%d-%H%M%S).txt ./... || true
	@echo "Reports saved to reports/security-$(shell date +%Y%m%d-%H%M%S).{json,txt}"

# Quick security check (strict mode for CI)
.PHONY: security-check
security-check:
	@echo "Running strict security check..."
	@command -v gosec >/dev/null 2>&1 || { echo "gosec not found. Install it with: go install github.com/securego/gosec/v2/cmd/gosec@latest"; exit 1; }
	gosec -conf .gosec.json -severity high -confidence high -quiet ./...


# Comprehensive security analysis
.PHONY: security-full
security-full:
	@echo "=== Comprehensive Security Analysis ==="
	@$(MAKE) security-report
	@echo "=== Security Analysis Complete ==="

# Install binary
.PHONY: install
install:
	@echo "Installing binary..."
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(GOPATH)/bin/$(BINARY_NAME) $(MAIN_PATH)

# Release build
.PHONY: release
release:
	@echo "Creating release build..."
	@if [ "$(VERSION)" = "dev" ]; then \
		echo "Please set VERSION for release build. Example: make release VERSION=v1.0.0"; \
		exit 1; \
	fi
	$(MAKE) clean
	$(MAKE) build VERSION=$(VERSION)
	@echo "Release build completed with version: $(VERSION)"

# Show build info
.PHONY: info
info:
	@echo "Build Information:"
	@echo "  Binary Name: $(BINARY_NAME)"
	@echo "  Main Path:   $(MAIN_PATH)"
	@echo "  Build Dir:   $(BUILD_DIR)"
	@echo "  Version:     $(VERSION)"
	@echo "  LDFLAGS:     $(LDFLAGS)"
	@echo "  Platforms:   $(PLATFORMS)"

# Check if all binaries exist
.PHONY: check
check:
	@echo "Checking built binaries..."
	@for binary in $(BINARIES); do \
		if [ -f "$$binary" ]; then \
			echo "✓ $$binary exists ($(shell ls -lh $$binary | awk '{print $$5}'))"; \
		else \
			echo "✗ $$binary missing"; \
		fi; \
	done

# ========================================
# LSP Testing and Validation Targets
# ========================================

# LSP test dependencies
.PHONY: lsp-deps
lsp-deps:
	@echo "Installing LSP test dependencies..."
	@command -v jq >/dev/null 2>&1 || { echo "jq is required for LSP testing. Please install jq."; exit 1; }
	@command -v bc >/dev/null 2>&1 || { echo "bc is required for LSP testing. Please install bc."; exit 1; }

# Setup LSP servers for testing
.PHONY: lsp-setup
lsp-setup: lsp-deps
	@echo "Setting up LSP servers for testing..."
	@echo "Installing Go language server (gopls)..."
	@go install golang.org/x/tools/gopls@latest || { echo "Failed to install gopls"; exit 1; }
	@echo "Installing Python language server..."
	@pip3 install python-lsp-server || echo "Warning: Failed to install python-lsp-server"
	@echo "Installing TypeScript language server..."
	@npm install -g typescript-language-server typescript || echo "Warning: Failed to install typescript-language-server"
	@echo "✓ LSP servers setup completed"

# Quick LSP validation (for PRs)
.PHONY: test-lsp-quick
test-lsp-quick: local lsp-setup
	@echo "Running quick LSP validation..."
	@mkdir -p lsp-results
	./bin/$(BINARY_NAME) test-lsp \
		--config=tests/data/configs/ci-test-config.yaml \
		--format=json \
		--output-dir=lsp-results \
		--filter="method=textDocument/definition,textDocument/hover" \
		--timeout=60s \
		--max-concurrency=2 \
		--quick-mode || { echo "Quick LSP validation failed"; exit 1; }
	@echo "✓ Quick LSP validation completed"


# Repository validation against real codebases
.PHONY: test-lsp-repos
test-lsp-repos: local lsp-setup
	@echo "Running repository validation tests..."
	@for repo_type in golang python typescript; do \
		echo "Testing against $$repo_type repositories..."; \
		mkdir -p repo-validation-$$repo_type; \
		./bin/$(BINARY_NAME) test-lsp \
			--config=tests/data/configs/$$repo_type-repo-config.yaml \
			--format=json \
			--output-dir=repo-validation-$$repo_type \
			--timeout=300s \
			--repository-mode || echo "Warning: $$repo_type repo validation failed"; \
	done
	@echo "✓ Repository validation completed"

# Clean LSP test artifacts
.PHONY: lsp-clean
lsp-clean:
	@echo "Cleaning LSP test artifacts..."
	@rm -rf lsp-results/ repo-validation-*/ 
	@echo "✓ LSP test artifacts cleaned"


# LSP help target
.PHONY: lsp-help
lsp-help:
	@echo "LSP Testing Targets:"
	@echo "==================="
	@echo ""
	@echo "Setup and Dependencies:"
	@echo "  lsp-setup             - Install and setup LSP servers"
	@echo "  lsp-deps              - Install LSP testing dependencies (jq, bc)"
	@echo ""  
	@echo "Testing Targets:"
	@echo "  test-lsp-simple       - Simple LSP validation (quick, one test per feature per language)"
	@echo "  test-lsp-full         - Full LSP test suite (all test scenarios)" 
	@echo "  test-lsp-quick        - Quick LSP validation (legacy, for PRs)"
	@echo "  test-lsp-validation   - LSP validation tests"
	@echo "  test-lsp-repos        - Repository validation against real codebases"
	@echo ""
	@echo "Maintenance:"
	@echo "  lsp-clean             - Clean LSP test artifacts and results"
	@echo "  lsp-help              - Show this help"
	@echo ""
	@echo "Examples:"
	@echo "  make lsp-setup        # Setup LSP testing environment"
	@echo "  make test-lsp-simple  # Run quick validation"
	@echo "  make test-lsp-full    # Run complete test suite"


# ========================================
# Simple LSP Testing Targets
# ========================================

# Simple LSP test - quick validation (one test per feature per language)
.PHONY: test-lsp-simple
test-lsp-simple: local lsp-setup
	@echo "Running simple LSP validation tests..."
	@mkdir -p lsp-results
	$(GOTEST) -v -timeout=120s ./tests/unit/internal/gateway ./tests/integration/gateway ./tests/unit/internal/transport ./tests/integration/transport ./tests/unit/mcp ./tests/integration/mcp -run "LSPValidation.*Simple" || \
	$(GOTEST) -v -short -timeout=60s ./tests/unit/internal/gateway ./tests/integration/gateway ./tests/unit/internal/transport ./tests/integration/transport ./tests/unit/mcp ./tests/integration/mcp -run "LSPValidation"
	@echo "✓ Simple LSP validation completed"

# Full LSP test - all test scenarios  
.PHONY: test-lsp-full
test-lsp-full: local lsp-setup
	@echo "Running full LSP test suite..."
	@mkdir -p lsp-results
	$(GOTEST) -v -timeout=300s ./tests/unit/internal/gateway ./tests/integration/gateway ./tests/unit/internal/transport ./tests/integration/transport ./tests/unit/mcp ./tests/integration/mcp -run "LSPValidation"
	$(GOTEST) -v -timeout=180s ./tests/unit/internal/gateway ./tests/integration/gateway ./tests/unit/internal/transport ./tests/integration/transport ./tests/unit/mcp ./tests/integration/mcp -run "LSPRepository"  
	@echo "✓ Full LSP test suite completed"


# ========================================
# Simplified LSP Testing System
# ========================================

# Build simple test runner
.PHONY: build-simple-test
build-simple-test:
	@echo "Building simple LSP test runner..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/simple-lsp-test ./cmd/simple-lsp-test/

# Simple unified test execution - Quick mode
.PHONY: test-simple-quick
test-simple-quick: build-simple-test
	@echo "Running simple LSP tests (quick mode)..."
	./$(BUILD_DIR)/simple-lsp-test --mode quick --verbose

# Simple unified test execution - Full mode  
.PHONY: test-simple-full
test-simple-full: build-simple-test
	@echo "Running simple LSP tests (full mode)..."
	./$(BUILD_DIR)/simple-lsp-test --mode full --verbose

# Simple unified test execution - Using bash script
.PHONY: test-simple-script
test-simple-script:
	@echo "Running simple LSP tests via bash script..."
	./tests/scripts/run-simple-lsp-tests.sh --mode full --verbose

# Simple test with custom config
.PHONY: test-simple-custom
test-simple-custom: build-simple-test
	@echo "Running simple LSP tests with custom config..."
	./$(BUILD_DIR)/simple-lsp-test --config tests/data/configs/simple-lsp-test-config.yaml --mode full --verbose

# ========================================
# Simple Repository Management
# ========================================

# Setup simple test repositories (one per language)
.PHONY: setup-simple-repos
setup-simple-repos:
	@echo "Setting up simple test repositories..."
	./tests/scripts/repositories/simple-clone-repos.sh setup
	@echo "✓ Simple repositories setup completed"

# Setup specific language repositories
.PHONY: setup-simple-go
setup-simple-go:
	@echo "Setting up Go repository (Kubernetes)..."
	./tests/scripts/repositories/simple-clone-repos.sh setup go

.PHONY: setup-simple-python
setup-simple-python:
	@echo "Setting up Python repository (Django)..."
	./tests/scripts/repositories/simple-clone-repos.sh setup python

.PHONY: setup-simple-typescript
setup-simple-typescript:
	@echo "Setting up TypeScript repository (VS Code)..."
	./tests/scripts/repositories/simple-clone-repos.sh setup typescript

.PHONY: setup-simple-java
setup-simple-java:
	@echo "Setting up Java repository (Spring Boot)..."
	./tests/scripts/repositories/simple-clone-repos.sh setup java

# Show status of simple repositories
.PHONY: simple-status
simple-status:
	@echo "Checking simple repository status..."
	./tests/scripts/repositories/simple-clone-repos.sh status

# Clean simple test repositories
.PHONY: simple-clean
simple-clean:
	@echo "Cleaning simple test repositories..."
	./tests/scripts/repositories/simple-clone-repos.sh cleanup

# Clean specific language repositories
.PHONY: simple-clean-go
simple-clean-go:
	@echo "Cleaning Go repositories..."
	./tests/scripts/repositories/simple-clone-repos.sh cleanup go

.PHONY: simple-clean-python
simple-clean-python:
	@echo "Cleaning Python repositories..."
	./tests/scripts/repositories/simple-clone-repos.sh cleanup python

.PHONY: simple-clean-typescript
simple-clean-typescript:
	@echo "Cleaning TypeScript repositories..."
	./tests/scripts/repositories/simple-clone-repos.sh cleanup typescript

.PHONY: simple-clean-java
simple-clean-java:
	@echo "Cleaning Java repositories..."
	./tests/scripts/repositories/simple-clone-repos.sh cleanup java

# Simple LSP testing with repository setup
.PHONY: test-simple-with-setup
test-simple-with-setup: setup-simple-repos test-simple-quick
	@echo "✓ Simple LSP testing with repository setup completed"