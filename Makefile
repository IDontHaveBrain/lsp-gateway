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

.PHONY: deps tidy format test
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
	$(GOTEST) -v -short ./...

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
	@echo "Running dead code analysis..."
	@command -v deadcode >/dev/null 2>&1 || { echo "deadcode not found. Install: go install golang.org/x/tools/cmd/deadcode@latest"; exit 1; }
	deadcode -test ./...

# Combined quality check
quality: format lint security
	@echo "All quality checks completed"

# =============================================================================
# TESTING TARGETS
# =============================================================================

.PHONY: test-simple-quick test-lsp-validation test-jdtls-integration test-circuit-breaker test-circuit-breaker-comprehensive test-e2e-quick test-e2e-full test-e2e-mcp test-e2e-http test-e2e-performance test-e2e-workflow setup-simple-repos
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

test-circuit-breaker:
	@echo "Running circuit breaker E2E tests..."
	$(GOTEST) -v -timeout 300s -run "TestCircuitBreakerE2ESuite" ./tests/e2e/...

test-circuit-breaker-comprehensive:
	@echo "Running comprehensive circuit breaker scenarios..."
	$(GOTEST) -v -timeout 600s -run "TestCircuitBreakerComprehensiveScenarios" ./tests/e2e/...

# E2E Test Suite Targets
test-e2e-quick:
	@echo "Running quick E2E validation tests..."
	$(GOTEST) -v -short -timeout 300s -run "TestE2EQuickValidation" ./tests/e2e/...

test-e2e-full:
	@echo "Running full E2E test suite..."
	$(GOTEST) -v -timeout 1800s -run "TestFullE2ETestSuite" ./tests/e2e/...

test-e2e-mcp:
	@echo "Running MCP protocol E2E tests..."
	$(GOTEST) -v -timeout 600s -run "TestMCPProtocol" ./tests/e2e/...

test-e2e-http:
	@echo "Running HTTP JSON-RPC E2E tests..."
	$(GOTEST) -v -timeout 600s -run "TestHTTPProtocol" ./tests/e2e/...

test-e2e-performance:
	@echo "Running E2E performance tests..."
	$(GOTEST) -v -timeout 1200s -run "TestE2EPerformance" ./tests/e2e/...

test-e2e-workflow:
	@echo "Running E2E workflow tests..."
	$(GOTEST) -v -timeout 900s -run "TestE2EWorkflow" ./tests/e2e/...

# =============================================================================
# PHASE 2 TESTING TARGETS
# =============================================================================

.PHONY: test-phase2-quick test-phase2-full test-phase2-performance test-phase2-production
.PHONY: test-phase2-smart-router test-phase2-three-tier test-phase2-incremental test-phase2-enhanced-mcp test-phase2-load

# Quick Phase 2 validation (1-2 minutes)
test-phase2-quick:
	@echo "Running Phase 2 quick validation tests..."
	$(GOTEST) -v -short -timeout 120s ./tests/phase2/integration/... ./tests/phase2/performance/...

# Comprehensive Phase 2 testing (10-15 minutes)
test-phase2-full:
	@echo "Running comprehensive Phase 2 tests..."
	$(GOTEST) -v -timeout 900s ./tests/phase2/integration/... ./tests/phase2/performance/...

# Phase 2 performance benchmarking (20-30 minutes)
test-phase2-performance:
	@echo "Running Phase 2 performance benchmarks..."
	$(GOTEST) -v -timeout 1800s -bench=. ./tests/phase2/performance/...
	$(GOTEST) -v -timeout 900s -run "Performance" ./tests/phase2/integration/...

# Phase 2 production readiness validation (45-60 minutes)
test-phase2-production:
	@echo "Running Phase 2 production readiness tests..."
	$(GOTEST) -v -timeout 3600s ./tests/phase2/production/...

# Individual Phase 2 component tests
test-phase2-smart-router:
	@echo "Running Smart Router integration tests..."
	$(GOTEST) -v -timeout 600s ./tests/phase2/integration/smart_router_integration_test.go

test-phase2-three-tier:
	@echo "Running Three-Tier Storage performance tests..."
	$(GOTEST) -v -timeout 900s ./tests/phase2/performance/three_tier_storage_test.go

test-phase2-incremental:
	@echo "Running Incremental Pipeline performance tests..."
	$(GOTEST) -v -timeout 900s ./tests/phase2/performance/incremental_pipeline_test.go

test-phase2-enhanced-mcp:
	@echo "Running Enhanced MCP Tools integration tests..."
	$(GOTEST) -v -timeout 600s ./tests/phase2/integration/enhanced_mcp_tools_test.go

test-phase2-load:
	@echo "Running Phase 2 load testing scenarios..."
	$(GOTEST) -v -timeout 1800s -run "Concurrent\|Load\|Scale" ./tests/phase2/...

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
	@echo "  lint      - Run linter"
	@echo "  security  - Run security analysis"
	@echo "  quality   - Run all quality checks"
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
	@echo "  test-e2e-mcp           - MCP protocol E2E tests"
	@echo "  test-e2e-http          - HTTP JSON-RPC E2E tests"
	@echo "  test-e2e-performance   - E2E performance tests"
	@echo "  test-e2e-workflow      - E2E workflow tests"
	@echo ""
	@echo "Phase 2 Testing:"
	@echo "  test-phase2-quick         - Phase 2 quick validation (1-2 min)"
	@echo "  test-phase2-full          - Comprehensive Phase 2 tests (10-15 min)"
	@echo "  test-phase2-performance   - Phase 2 performance benchmarks (20-30 min)"
	@echo "  test-phase2-production    - Production readiness validation (45-60 min)"
	@echo "  test-phase2-smart-router  - Smart Router integration tests"
	@echo "  test-phase2-three-tier    - Three-Tier Storage performance tests"
	@echo "  test-phase2-incremental   - Incremental Pipeline performance tests"
	@echo "  test-phase2-enhanced-mcp  - Enhanced MCP Tools integration tests"
	@echo "  test-phase2-load          - Phase 2 load testing scenarios"
	@echo ""
	@echo "Utility:"
	@echo "  install   - Install binary to GOPATH"
	@echo "  release   - Create release build"
	@echo "  info      - Show build information"
	@echo "  help      - Show this help"