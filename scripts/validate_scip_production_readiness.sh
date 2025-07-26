#!/bin/bash

# SCIP Production Readiness Validation Script
# This script runs comprehensive tests to validate SCIP Phase 1 implementation
# is production-ready with robust error handling and configuration

set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"

echo "===================================="
echo "SCIP Production Readiness Validation"
echo "===================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Track validation results
VALIDATION_PASSED=true
VALIDATION_RESULTS=()

# Function to run a test category
run_test_category() {
    local category=$1
    local description=$2
    local test_command=$3
    
    echo -e "${YELLOW}Testing: ${description}${NC}"
    echo "Command: $test_command"
    echo "----------------------------------------"
    
    if eval "$test_command"; then
        echo -e "${GREEN}✓ ${description} - PASSED${NC}"
        VALIDATION_RESULTS+=("✓ ${description}")
    else
        echo -e "${RED}✗ ${description} - FAILED${NC}"
        VALIDATION_RESULTS+=("✗ ${description}")
        VALIDATION_PASSED=false
    fi
    echo ""
}

# Change to project root
cd "$PROJECT_ROOT"

# 1. Configuration Validation
echo "1. Configuration Validation"
echo "=========================="

# Test configuration schema validation
run_test_category "config_validation" \
    "Configuration Schema Validation" \
    "go test -v ./internal/config -run TestSCIPConfig -timeout 5m"

# Test environment variable support
run_test_category "env_validation" \
    "Environment Variable Support" \
    "SCIP_CACHE_ENABLED=true SCIP_CACHE_MAX_SIZE=1000 go test -v ./internal/config -run TestEnvironmentOverrides -timeout 5m"

# 2. Error Handling Validation
echo -e "\n2. Error Handling Validation"
echo "============================"

# Run production readiness tests
run_test_category "error_handling" \
    "Error Handling Scenarios" \
    "go test -v ./tests/integration/scip -run TestSCIPProductionReadiness/ErrorHandling -timeout 10m"

# 3. Fallback Mechanism Testing
echo -e "\n3. Fallback Mechanism Testing"
echo "============================="

run_test_category "fallback_mechanisms" \
    "Graceful Fallback to LSP" \
    "go test -v ./tests/integration/scip -run TestSCIPProductionReadiness/FallbackMechanisms -timeout 10m"

# 4. Resource Management Testing
echo -e "\n4. Resource Management Testing"
echo "=============================="

run_test_category "resource_management" \
    "Memory and Resource Management" \
    "go test -v ./tests/integration/scip -run TestSCIPProductionReadiness/ResourceManagement -timeout 15m"

# 5. Concurrent Access Testing
echo -e "\n5. Concurrent Access Testing"
echo "==========================="

# Run with race detector
run_test_category "concurrent_access" \
    "Thread Safety and Concurrent Access" \
    "go test -race -v ./tests/integration/scip -run TestSCIPProductionReadiness/ConcurrentAccess -timeout 15m"

# 6. Operational Features Testing
echo -e "\n6. Operational Features Testing"
echo "=============================="

run_test_category "operational_features" \
    "Monitoring, Logging, and Observability" \
    "go test -v ./tests/integration/scip -run TestSCIPProductionReadiness/OperationalFeatures -timeout 10m"

# 7. Performance Regression Testing
echo -e "\n7. Performance Regression Testing"
echo "================================"

run_test_category "performance_regression" \
    "No Performance Regression in LSP-only Mode" \
    "go test -v ./tests/performance -run TestSCIPPerformanceRegression -timeout 10m"

# 8. Cache Hit Rate Validation
echo -e "\n8. Cache Hit Rate Validation"
echo "==========================="

run_test_category "cache_hit_rate" \
    "SCIP Cache Hit Rate (>10% target)" \
    "go test -v ./tests/performance -run TestSCIPCacheHitRate -timeout 10m"

# 9. Memory Impact Testing
echo -e "\n9. Memory Impact Testing"
echo "======================="

run_test_category "memory_impact" \
    "SCIP Memory Impact (<100MB target)" \
    "go test -v ./tests/performance -run TestSCIPMemoryImpact -timeout 10m"

# 10. High Load Testing
echo -e "\n10. High Load Testing"
echo "===================="

run_test_category "high_load" \
    "Concurrent Load Capacity (200+ requests)" \
    "go test -v ./tests/performance -run TestSCIPConcurrentLoadCapacity -timeout 15m"

# 11. Integration Tests
echo -e "\n11. Integration Tests"
echo "===================="

# Test all LSP methods with SCIP
run_test_category "lsp_integration" \
    "LSP Method Integration with SCIP" \
    "go test -v ./tests/integration/scip -run TestLSPMethodIntegration -timeout 10m"

# 12. Build and Lint Validation
echo -e "\n12. Build and Lint Validation"
echo "============================"

# Ensure code compiles without errors
run_test_category "build_validation" \
    "Build Validation" \
    "go build -v ./internal/indexing/..."

# Run linter on SCIP code
run_test_category "lint_validation" \
    "Code Quality (golangci-lint)" \
    "golangci-lint run ./internal/indexing/... --timeout=5m"

# Summary Report
echo -e "\n===================================="
echo "VALIDATION SUMMARY"
echo "===================================="

for result in "${VALIDATION_RESULTS[@]}"; do
    echo "$result"
done

echo ""

if [ "$VALIDATION_PASSED" = true ]; then
    echo -e "${GREEN}✓ ALL VALIDATION TESTS PASSED${NC}"
    echo -e "${GREEN}SCIP Phase 1 implementation is PRODUCTION READY${NC}"
    exit 0
else
    echo -e "${RED}✗ SOME VALIDATION TESTS FAILED${NC}"
    echo -e "${RED}SCIP Phase 1 implementation needs fixes before production${NC}"
    exit 1
fi