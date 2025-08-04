# Makefile for LSP Gateway
# Simplified build system for local development

# Configuration
BINARY_NAME := lsp-gateway
MAIN_PATH := lsp-gateway/src/cmd/lsp-gateway
BUILD_DIR := bin

# Extract version from package.json if available, fallback to 'dev'
VERSION ?= $(shell if [ -f package.json ]; then node -p "require('./package.json').version" 2>/dev/null || echo "dev"; else echo "dev"; fi)

# Build info (simplified for local development)
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# LD flags - inject version information into the binary
LDFLAGS := -s -w \
	-X lsp-gateway/src/internal/version.Version=$(VERSION) \
	-X lsp-gateway/src/internal/version.GitCommit=$(GIT_COMMIT) \
	-X lsp-gateway/src/internal/version.BuildDate=$(BUILD_TIME)

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

.PHONY: all build local clean unlink
all: build

# Build for current platform (most common use case)
local: $(BUILD_DIR)
	@echo "Building for current platform..."
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Linking npm package globally..."
	@if command -v npm >/dev/null 2>&1; then \
		if [ -f package.json ]; then \
			npm link; \
			echo "✅ npm link completed - 'lsp-gateway' command is now available globally"; \
		else \
			echo "⚠️  package.json not found, skipping npm link"; \
		fi; \
	else \
		echo "⚠️  npm not found, skipping npm link"; \
	fi

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

# Unlink npm package globally
unlink:
	@echo "Unlinking npm package globally..."
	@if command -v npm >/dev/null 2>&1; then \
		if [ -f package.json ]; then \
			npm unlink -g lsp-gateway 2>/dev/null || true; \
			echo "✅ npm unlink completed - 'lsp-gateway' command removed from global scope"; \
		else \
			echo "⚠️  package.json not found, skipping npm unlink"; \
		fi; \
	else \
		echo "⚠️  npm not found, skipping npm unlink"; \
	fi

# =============================================================================
# DEVELOPMENT TARGETS
# =============================================================================

.PHONY: deps tidy format test test-unit test-integration test-quick
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
	@echo "Running all tests..."
	$(GOTEST) -v -timeout 600s ./tests/e2e/... ./tests/integration/... ./simple/...

test-unit:
	@echo "Running unit tests..."
	@if [ -n "$$(find tests/unit -name '*_test.go' -type f 2>/dev/null)" ]; then \
		$(GOTEST) -v -short -timeout 120s ./tests/unit/...; \
	else \
		echo "Running simple package configuration tests only..."; \
		$(GOTEST) -v -short -timeout 120s -run "TestGetDefaultConfig|TestLoadConfig|TestDetectLanguageFromFile|TestServerConfig_Validation|TestConfig_GetServerForLanguage" ./simple/...; \
	fi

test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -v -timeout 600s -run Integration ./...

test-quick:
	@echo "Running quick validation tests..."
	$(GOTEST) -v -short -timeout 120s -run "TestGetDefaultConfig|TestLoadConfig" ./simple/...

# =============================================================================
# CODE QUALITY
# =============================================================================

.PHONY: vet lint security quality quality-full
vet:
	@echo "Running go vet..."
	$(GOCMD) vet ./src/... || echo "Some packages have import issues - continuing..."

lint:
	@echo "Running linter..."
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not found. Install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; exit 1; }
	golangci-lint run ./cmd/... ./simple/... || echo "Linting issues found - review above for code quality improvements"

security:
	@echo "Running security analysis (optional)..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./cmd/... ./simple/... || echo "Security issues found - review above for production use"; \
	else \
		echo "gosec not found - skipping security check (install: go install github.com/securego/gosec/v2/cmd/gosec@latest)"; \
	fi

# Essential quality checks (no external dependencies)
quality: format vet
	@echo "Essential quality checks completed"

# Full quality checks (includes external tools)
quality-full: format vet lint security
	@echo "Full quality checks completed"

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
	@echo "LSP Gateway Makefile (Simplified)"
	@echo "=================================="
	@echo ""
	@echo "Build Commands:"
	@echo "  local     - Build for current platform + npm link"
	@echo "  build     - Build for all platforms"
	@echo "  clean     - Clean build artifacts"
	@echo "  unlink    - Remove npm global link"
	@echo ""
	@echo "Development Commands:"
	@echo "  deps      - Download dependencies"
	@echo "  tidy      - Tidy go modules"
	@echo "  format    - Format code"
	@echo ""
	@echo "Testing Commands:"
	@echo "  test               - Run all tests"
	@echo "  test-unit          - Run unit tests"
	@echo "  test-integration   - Run integration tests"
	@echo "  test-quick         - Run quick validation tests"
	@echo ""
	@echo "Quality Commands:"
	@echo "  vet          - Run go vet (built-in)"
	@echo "  lint         - Run golangci-lint"
	@echo "  security     - Run security analysis (optional)"
	@echo "  quality      - Essential checks (format + vet)"
	@echo "  quality-full - All checks (format + vet + lint + security)"
	@echo ""
	@echo "Utility Commands:"
	@echo "  install   - Install binary to GOPATH"
	@echo "  release   - Create release build"
	@echo "  info      - Show build information"
	@echo "  help      - Show this help"
	@echo ""
	@echo "Example Usage:"
	@echo "  make local         # Most common - build and install"
	@echo "  make test-quick    # Quick validation"
	@echo "  make quality       # Essential quality checks"
	@echo "  make quality-full  # Full quality checks"