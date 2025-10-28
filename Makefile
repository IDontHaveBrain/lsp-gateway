# Makefile for LSP Gateway

BINARY_NAME := lsp-gateway
MAIN_PATH := lsp-gateway/src/cmd/lsp-gateway
BUILD_DIR := bin
BUILD_TAGS := cache_enabled

VERSION ?= $(shell if [ -f package.json ]; then node -p "require('./package.json').version" 2>/dev/null || echo "dev"; else echo "dev"; fi)
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

LDFLAGS := -s -w \
	-X lsp-gateway/src/internal/version.Version=$(VERSION) \
	-X lsp-gateway/src/internal/version.GitCommit=$(GIT_COMMIT) \
	-X lsp-gateway/src/internal/version.BuildDate=$(BUILD_TIME)

GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOCLEAN := $(GOCMD) clean
GOMOD := $(GOCMD) mod

PLATFORMS := linux/amd64 darwin/amd64 darwin/arm64 windows/amd64

# =============================================================================
# BUILD
# =============================================================================

.PHONY: all local build clean
all: local

local: $(BUILD_DIR)
	@echo "Building for current platform..."
	$(GOBUILD) -tags "$(BUILD_TAGS)" -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Creating npm wrapper script..."
	@echo '#!/usr/bin/env node' > $(BUILD_DIR)/$(BINARY_NAME).js
	@echo '' >> $(BUILD_DIR)/$(BINARY_NAME).js
	@echo 'const { spawn } = require("child_process");' >> $(BUILD_DIR)/$(BINARY_NAME).js
	@echo 'const path = require("path");' >> $(BUILD_DIR)/$(BINARY_NAME).js
	@echo 'const fs = require("fs");' >> $(BUILD_DIR)/$(BINARY_NAME).js
	@echo '' >> $(BUILD_DIR)/$(BINARY_NAME).js
	@echo 'const binaryName = process.platform === "win32" ? "lsp-gateway.exe" : "lsp-gateway";' >> $(BUILD_DIR)/$(BINARY_NAME).js
	@echo 'const binaryPath = path.join(__dirname, binaryName);' >> $(BUILD_DIR)/$(BINARY_NAME).js
	@echo '' >> $(BUILD_DIR)/$(BINARY_NAME).js
	@echo 'if (!fs.existsSync(binaryPath)) {' >> $(BUILD_DIR)/$(BINARY_NAME).js
	@echo '  console.error("Binary not found at:", binaryPath);' >> $(BUILD_DIR)/$(BINARY_NAME).js
	@echo '  console.error("Run \"make local\" to build");' >> $(BUILD_DIR)/$(BINARY_NAME).js
	@echo '  process.exit(1);' >> $(BUILD_DIR)/$(BINARY_NAME).js
	@echo '}' >> $(BUILD_DIR)/$(BINARY_NAME).js
	@echo '' >> $(BUILD_DIR)/$(BINARY_NAME).js
	@echo 'const child = spawn(binaryPath, process.argv.slice(2), { stdio: "inherit" });' >> $(BUILD_DIR)/$(BINARY_NAME).js
	@echo 'process.on("SIGINT", () => child.kill("SIGINT"));' >> $(BUILD_DIR)/$(BINARY_NAME).js
	@echo 'process.on("SIGTERM", () => child.kill("SIGTERM"));' >> $(BUILD_DIR)/$(BINARY_NAME).js
	@echo 'child.on("close", (code) => process.exit(code || 0));' >> $(BUILD_DIR)/$(BINARY_NAME).js
	@echo 'child.on("error", (err) => { console.error("Failed to start:", err.message); process.exit(1); });' >> $(BUILD_DIR)/$(BINARY_NAME).js
	@chmod +x $(BUILD_DIR)/$(BINARY_NAME).js
	@if command -v npm >/dev/null 2>&1 && [ -f package.json ]; then \
		npm link && echo "npm link completed"; \
	fi

build: $(BUILD_DIR)
	@echo "Building for all platforms..."
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d'/' -f1); \
		arch=$$(echo $$platform | cut -d'/' -f2); \
		output=$(BUILD_DIR)/$(BINARY_NAME)-$$os; \
		[ $$os = "windows" ] && output=$$output.exe; \
		[ $$os = "darwin" ] && [ $$arch = "arm64" ] && output=$(BUILD_DIR)/$(BINARY_NAME)-macos-arm64; \
		[ $$os = "darwin" ] && [ $$arch = "amd64" ] && output=$(BUILD_DIR)/$(BINARY_NAME)-macos; \
		echo "Building $$os/$$arch -> $$output"; \
		GOOS=$$os GOARCH=$$arch $(GOBUILD) -tags "$(BUILD_TAGS)" -ldflags "$(LDFLAGS)" -o $$output $(MAIN_PATH); \
	done

$(BUILD_DIR):
	@mkdir -p $(BUILD_DIR)

clean:
	@rm -rf $(BUILD_DIR)
	@$(GOCLEAN)

# =============================================================================
# TESTING
# =============================================================================

.PHONY: test test-unit test-integration
test:
	@echo "Running all tests..."
	@$(GOTEST) -v -short -timeout 120s ./src/...
	@$(GOTEST) -v -timeout 600s -p=2 ./tests/integration/...

test-unit:
	@$(GOTEST) -v -short -timeout 120s ./src/...

test-integration:
	@$(GOTEST) -v -timeout 600s -p=2 ./tests/integration/...

# =============================================================================
# QUALITY
# =============================================================================

.PHONY: lint
lint:
	@golangci-lint fmt
	@golangci-lint run

# =============================================================================
# UTILITY
# =============================================================================

.PHONY: release help
release:
	@[ "$(VERSION)" != "dev" ] || { echo "Set VERSION for release: make release VERSION=v1.0.0"; exit 1; }
	@$(MAKE) clean && $(MAKE) build VERSION=$(VERSION)

help:
	@echo "LSP Gateway Makefile"
	@echo ""
	@echo "Build:"
	@echo "  local        Build for current platform + npm link"
	@echo "  build        Build for all platforms"
	@echo "  clean        Clean build artifacts"
	@echo ""
	@echo "Testing:"
	@echo "  test             Run all tests"
	@echo "  test-unit        Run unit tests only"
	@echo "  test-integration Run integration tests only"
	@echo ""
	@echo "Quality:"
	@echo "  lint         Run golangci-lint fmt and run"
	@echo ""
	@echo "Utility:"
	@echo "  release      Create release build (set VERSION)"
	@echo "  help         Show this help"
