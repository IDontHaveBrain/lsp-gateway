# Makefile for LSP Gateway
# Simplified build system for solo development

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
GOCLEAN := $(GOCMD) clean
GOMOD := $(GOCMD) mod

# Platform definitions
PLATFORMS := linux/amd64 darwin/amd64 darwin/arm64 windows/amd64

# =============================================================================
# BUILD TARGETS
# =============================================================================

.PHONY: all build local clean
all: build

# Build for current platform (most common use case)
local: $(BUILD_DIR)
	@echo "Building for current platform..."
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)

# Build for all platforms
build: $(BUILD_DIR)
	@echo "Building for all platforms..."
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d'/' -f1); \
		arch=$$(echo $$platform | cut -d'/' -f2); \
		output=$(BUILD_DIR)/$(BINARY_NAME)-$$os; \
		if [ $$os = "windows" ]; then output=$$output.exe; fi; \
		if [ $$os = "darwin" ] && [ $$arch = "arm64" ]; then output=$(BUILD_DIR)/$(BINARY_NAME)-macos-arm64; fi; \
		if [ $$os = "darwin" ] && [ $$arch = "amd64" ]; then output=$(BUILD_DIR)/$(BINARY_NAME)-macos; fi; \
		echo "Building $$os/$$arch -> $$output"; \
		GOOS=$$os GOARCH=$$arch $(GOBUILD) -ldflags "$(LDFLAGS)" -o $$output $(MAIN_PATH); \
	done

# Individual platform builds (for convenience)
.PHONY: linux windows macos macos-arm64
linux: $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux $(MAIN_PATH)

windows: $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows.exe $(MAIN_PATH)

macos: $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-macos $(MAIN_PATH)

macos-arm64: $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-macos-arm64 $(MAIN_PATH)

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	$(GOCLEAN)

# =============================================================================
# DEVELOPMENT TARGETS
# =============================================================================

.PHONY: deps tidy format test test-unit test-internal
deps:
	@echo "Downloading dependencies..."
	$(GOCMD) get -v ./...

tidy:
	@echo "Tidying go modules..."
	$(GOMOD) tidy

format:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

test-unit:
	@echo "Running unit tests..."
	$(GOTEST) -v -short -timeout 60s ./tests/unit/... ./internal/...

test-internal:
	@echo "Running internal package unit tests..."
	$(GOTEST) -v -short -timeout 60s ./internal/...

test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -v -run Integration ./...

# =============================================================================
# CODE QUALITY
# =============================================================================

.PHONY: lint security quality
lint:
	@echo "Running linter..."
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not found. Install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; exit 1; }
	golangci-lint run ./...

security:
	@echo "Running security analysis..."
	@command -v gosec >/dev/null 2>&1 || { echo "gosec not found. Install: go install github.com/securego/gosec/v2/cmd/gosec@latest"; exit 1; }
	gosec -conf .gosec.json ./...

check-deadcode:
	@echo "Running dead code analysis (production code only)..."
	@command -v deadcode >/dev/null 2>&1 || { echo "deadcode not found. Install: go install golang.org/x/tools/cmd/deadcode@latest"; exit 1; }
	deadcode -filter="github.com/.*|cmd/.*|internal/.*" ./...

check-deadcode-strict:
	@echo "Running strict dead code analysis (including tests)..."
	@command -v deadcode >/dev/null 2>&1 || { echo "deadcode not found. Install: go install golang.org/x/tools/cmd/deadcode@latest"; exit 1; }
	deadcode -test -filter="github.com/.*|cmd/.*|internal/.*" ./...

check-deadcode-main:
	@echo "Running dead code analysis (main packages only)..."
	@command -v deadcode >/dev/null 2>&1 || { echo "deadcode not found. Install: go install golang.org/x/tools/cmd/deadcode@latest"; exit 1; }
	deadcode -filter="cmd/.*" ./cmd/...

check-deadcode-internal:
	@echo "Running dead code analysis (internal packages only)..."
	@command -v deadcode >/dev/null 2>&1 || { echo "deadcode not found. Install: go install golang.org/x/tools/cmd/deadcode@latest"; exit 1; }
	deadcode -filter="internal/.*" ./internal/...

# Combined quality check
quality: format lint security
	@echo "All quality checks completed"

# =============================================================================
# TESTING TARGETS
# =============================================================================

.PHONY: test-simple-quick test-lsp-validation test-jdtls-integration test-circuit-breaker test-circuit-breaker-comprehensive test-e2e-quick test-e2e-full test-e2e-java test-e2e-python test-e2e-typescript test-e2e-go test-java-real test-python-real test-typescript-real test-e2e-advanced test-e2e-workflow test-e2e-setup-cli test-e2e-mcp test-mcp-stdio test-mcp-tcp test-mcp-tools test-mcp-scip test-npm-cli test-npm-mcp test-npm-mcp-quick test-npm-mcp-js test-npm-mcp-go setup-simple-repos
setup-simple-repos:
	@echo "Setting up test repositories..."
	./scripts/setup-simple-repos.sh || echo "Setup script not found, skipping..."

test-simple-quick:
	@echo "Running quick validation tests..."
	$(GOTEST) -v -short -timeout 60s ./tests/unit/...

test-lsp-validation:
	@echo "Running LSP validation tests..."
	$(GOTEST) -v -timeout 300s ./tests/integration/...

test-lsp-validation-short:
	@echo "Running short LSP validation..."
	$(GOTEST) -v -short -timeout 120s ./tests/integration/...

test-jdtls-integration:
	@echo "Running JDTLS integration tests..."
	$(GOTEST) -v -timeout 600s -run "TestJDTLS" ./tests/integration/...
	@echo "Running Java E2E tests with real JDTLS..."
	$(GOTEST) -v -timeout 900s -run "TestJava.*E2ETestSuite" ./tests/e2e/...

test-circuit-breaker:
	@echo "Running circuit breaker E2E tests..."
	$(GOTEST) -v -timeout 300s -run "TestCircuitBreakerE2ESuite" ./tests/e2e/...

test-circuit-breaker-comprehensive:
	@echo "Running comprehensive circuit breaker scenarios..."
	$(GOTEST) -v -timeout 600s -run "TestCircuitBreakerComprehensiveScenarios" ./tests/e2e/...

# E2E Test Suite Targets
test-e2e-quick:
	@echo "Running quick E2E validation tests..."
	$(GOTEST) -v -short -timeout 300s ./tests/e2e/...

test-e2e-full:
	@echo "Running full E2E test suite..."
	$(GOTEST) -v -timeout 1800s ./tests/e2e/...

# Language-Specific E2E Test Targets
test-e2e-java:
	@echo "Running Java E2E tests (mock and real JDTLS)..."
	$(GOTEST) -v -timeout 900s -run "TestJava.*E2ETestSuite|TestJavaRealJDTLSE2ETestSuite" ./tests/e2e/...

test-e2e-python:
	@echo "Running Python E2E tests..."
	$(GOTEST) -v -timeout 600s -run "TestPython.*E2ETestSuite" ./tests/e2e/...

test-e2e-typescript:
	@echo "Running TypeScript E2E tests..."
	$(GOTEST) -v -timeout 600s -run "TestTypeScript.*E2ETestSuite" ./tests/e2e/...

test-e2e-go:
	@echo "Running Go E2E tests..."
	$(GOTEST) -v -timeout 600s -run "TestGo.*E2ETestSuite" ./tests/e2e/...

# Real Language Server Integration Tests
test-java-real:
	@echo "Running Java real JDTLS integration tests..."
	$(GOTEST) -v -timeout 900s -run "TestJavaRealJDTLSE2ETestSuite" ./tests/e2e/...

test-python-real:
	@echo "Running Python real pylsp integration tests..."
	$(GOTEST) -v -timeout 600s -run "TestPythonReal.*IntegrationTestSuite" ./tests/e2e/...

test-typescript-real:
	@echo "Running TypeScript real server integration tests..."
	$(GOTEST) -v -timeout 600s -run "TestTypeScriptReal.*IntegrationTestSuite" ./tests/e2e/...

# Advanced E2E Test Targets
test-e2e-advanced:
	@echo "Running advanced E2E test scenarios..."
	$(GOTEST) -v -timeout 1200s -run ".*Advanced.*E2ETestSuite" ./tests/e2e/...

test-e2e-workflow:
	@echo "Running E2E workflow tests..."
	$(GOTEST) -v -timeout 900s -run ".*BasicWorkflow.*E2ETestSuite" ./tests/e2e/...

test-e2e-setup-cli:
	@echo "Running Setup CLI E2E tests..."
	$(GOTEST) -v -timeout 300s -run "TestSetupCliE2ETestSuite" ./tests/e2e/setup_cli_e2e_test.go

# MCP E2E Test Targets
test-e2e-mcp:
	@echo "Running comprehensive MCP E2E tests..."
	$(GOTEST) -v -timeout 600s -run "TestMCP" ./tests/e2e/mcp_protocol_e2e_test.go ./tests/e2e/mcp_tools_e2e_test.go ./tests/e2e/mcp_scip_enhanced_e2e_test.go

test-mcp-stdio:
	@echo "Running MCP STDIO protocol tests..."
	$(GOTEST) -v -timeout 300s -run "TestMCPStdioProtocol" ./tests/e2e/mcp_protocol_e2e_test.go

test-mcp-tcp:
	@echo "Running MCP TCP protocol tests..."
	$(GOTEST) -v -timeout 300s -run "TestMCPTCPProtocol" ./tests/e2e/mcp_protocol_e2e_test.go

test-mcp-tools:
	@echo "Running comprehensive MCP tools E2E tests..."
	$(GOTEST) -v -timeout 600s ./tests/e2e/mcp_tools_e2e_test.go

test-mcp-scip:
	@echo "Running SCIP-enhanced MCP E2E tests..."
	$(GOTEST) -v -timeout 900s ./tests/e2e/mcp_scip_enhanced_e2e_test.go

test-npm-cli:
	@echo "Running npm-cli E2E tests..."
	$(GOTEST) -v -timeout 600s -run "TestNpmCliE2ETestSuite" ./tests/e2e/npm_cli_e2e_test.go

# NPM-MCP Test Targets  
test-npm-mcp:
	@echo "Running comprehensive NPM-MCP tests..."
	./scripts/test-npm-mcp.sh --verbose

test-npm-mcp-quick:
	@echo "Running quick NPM-MCP tests..."
	./scripts/test-npm-mcp.sh --quick --verbose

test-npm-mcp-js:
	@echo "Running NPM-MCP JavaScript tests only..."
	./scripts/test-npm-mcp.sh --js-only --verbose

test-npm-mcp-go:
	@echo "Running NPM-MCP Go integration tests only..."
	./scripts/test-npm-mcp.sh --go-only --verbose

# =============================================================================
# UTILITY TARGETS
# =============================================================================

.PHONY: install release info help
install:
	@echo "Installing binary..."
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(GOPATH)/bin/$(BINARY_NAME) $(MAIN_PATH)

release:
	@if [ "$(VERSION)" = "dev" ]; then echo "Set VERSION for release: make release VERSION=v1.0.0"; exit 1; fi
	$(MAKE) clean && $(MAKE) build VERSION=$(VERSION)
	@echo "Release $(VERSION) completed"

info:
	@echo "LSP Gateway Build Info:"
	@echo "  Version: $(VERSION)"
	@echo "  Binary:  $(BINARY_NAME)"
	@echo "  Platforms: $(PLATFORMS)"

help:
	@echo "LSP Gateway Makefile"
	@echo "====================="
	@echo ""
	@echo "Build:"
	@echo "  local     - Build for current platform"
	@echo "  build     - Build for all platforms"
	@echo "  clean     - Clean build artifacts"
	@echo ""
	@echo "Development:"
	@echo "  deps      - Download dependencies"
	@echo "  tidy      - Tidy go modules"
	@echo "  format    - Format code"
	@echo "  test      - Run all tests"
	@echo "  test-unit - Run unit tests only"
	@echo ""
	@echo "Quality:"
	@echo "  lint                  - Run linter"
	@echo "  security              - Run security analysis"
	@echo "  check-deadcode        - Run dead code analysis (production only, fewer false positives)"
	@echo "  check-deadcode-strict - Run strict dead code analysis (including tests)"
	@echo "  check-deadcode-main   - Run dead code analysis (main packages only)"
	@echo "  check-deadcode-internal - Run dead code analysis (internal packages only)"
	@echo "  quality               - Run all quality checks"
	@echo ""
	@echo "Testing:"
	@echo "  test-simple-quick      - Quick validation tests"
	@echo "  test-lsp-validation    - Full LSP validation"
	@echo "  test-circuit-breaker   - Circuit breaker E2E tests"
	@echo "  test-circuit-breaker-comprehensive - Comprehensive circuit breaker scenarios"
	@echo "  setup-simple-repos     - Setup test repositories"
	@echo ""
	@echo "E2E Testing:"
	@echo "  test-e2e-quick         - Quick E2E validation tests"
	@echo "  test-e2e-full          - Full E2E test suite"
	@echo "  test-e2e-workflow      - E2E workflow tests"
	@echo "  test-e2e-advanced      - Advanced E2E test scenarios"
	@echo "  test-e2e-setup-cli     - Setup CLI E2E tests (binary execution)"
	@echo "  test-e2e-mcp           - Comprehensive MCP E2E tests (all MCP functionality)"
	@echo ""
	@echo "MCP Protocol Testing:"
	@echo "  test-mcp-stdio         - MCP STDIO protocol tests"
	@echo "  test-mcp-tcp           - MCP TCP protocol tests"
	@echo "  test-mcp-tools         - MCP tools E2E tests (all 5 LSP tools with real servers)"
	@echo "  test-mcp-scip          - SCIP-enhanced MCP E2E tests (performance and intelligence)"
	@echo ""
	@echo "Language-Specific E2E Tests:"
	@echo "  test-e2e-java          - Java E2E tests (mock and real JDTLS)"
	@echo "  test-e2e-python        - Python E2E tests"
	@echo "  test-e2e-typescript    - TypeScript E2E tests"
	@echo "  test-e2e-go            - Go E2E tests"
	@echo ""
	@echo "Real Language Server Integration:"
	@echo "  test-java-real         - Java real JDTLS integration"
	@echo "  test-python-real       - Python real pylsp integration"
	@echo "  test-typescript-real   - TypeScript real server integration"
	@echo ""
	@echo "NPM-MCP Integration Testing:"
	@echo "  test-npm-mcp           - Comprehensive NPM-MCP tests (JavaScript + Go)"
	@echo "  test-npm-mcp-quick     - Quick NPM-MCP tests"
	@echo "  test-npm-mcp-js        - JavaScript E2E tests only"
	@echo "  test-npm-mcp-go        - Go integration tests only"
	@echo ""
	@echo "Utility:"
	@echo "  install   - Install binary to GOPATH"
	@echo "  release   - Create release build"
	@echo "  info      - Show build information"
	@echo "  help      - Show this help"