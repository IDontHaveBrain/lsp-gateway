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
	@echo "  test         - Run tests"
	@echo "  test-cover   - Run tests with coverage"
	@echo "  deps         - Download dependencies"
	@echo "  tidy         - Tidy go modules"
	@echo "  format       - Format code"
	@echo "  lint         - Run linter (requires golangci-lint)"
	@echo "  install      - Install binary to GOPATH/bin"
	@echo "  release      - Create release build with version"
	@echo "  help         - Show this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  VERSION      - Set version (default: dev)"
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

.PHONY: test-cover
test-cover:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -cover ./...

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