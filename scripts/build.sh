#!/bin/bash

# Build script for LSP Gateway - Multi-platform compilation
# This script builds binaries for Linux, Windows, and macOS (amd64 and arm64)

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
BINARY_NAME="lsp-gateway"
MAIN_PATH="cmd/lsp-gateway/main.go"
BUILD_DIR="bin"
VERSION=${VERSION:-"dev"}
LDFLAGS="-s -w -X main.version=${VERSION}"

# Create build directory
mkdir -p "${BUILD_DIR}"

echo -e "${YELLOW}Building LSP Gateway for multiple platforms...${NC}"

# Build targets
declare -a TARGETS=(
    "linux/amd64:${BINARY_NAME}-linux"
    "windows/amd64:${BINARY_NAME}-windows.exe"
    "darwin/amd64:${BINARY_NAME}-macos"
    "darwin/arm64:${BINARY_NAME}-macos-arm64"
)

# Build function
build_target() {
    local target=$1
    local output=$2
    local goos=${target%/*}
    local goarch=${target#*/}
    
    echo -e "${YELLOW}Building ${output} (${goos}/${goarch})...${NC}"
    
    GOOS=${goos} GOARCH=${goarch} go build \
        -ldflags "${LDFLAGS}" \
        -o "${BUILD_DIR}/${output}" \
        "${MAIN_PATH}"
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Successfully built ${output}${NC}"
        
        # Display file info
        if [ -f "${BUILD_DIR}/${output}" ]; then
            size=$(ls -lh "${BUILD_DIR}/${output}" | awk '{print $5}')
            echo -e "  Size: ${size}"
        fi
    else
        echo -e "${RED}✗ Failed to build ${output}${NC}"
        exit 1
    fi
    echo
}

# Clean previous builds
echo -e "${YELLOW}Cleaning previous builds...${NC}"
rm -f "${BUILD_DIR}/${BINARY_NAME}-"*

# Build all targets
for target_config in "${TARGETS[@]}"; do
    IFS=':' read -r target output <<< "${target_config}"
    build_target "${target}" "${output}"
done

echo -e "${GREEN}Build completed successfully!${NC}"
echo -e "${YELLOW}Binaries are available in the ${BUILD_DIR}/ directory:${NC}"
ls -la "${BUILD_DIR}/"