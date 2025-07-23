#!/bin/bash

# LSP Gateway Coverage Baseline Measurement Script
# Usage: ./scripts/coverage-baseline.sh

set -e

echo "==========================="
echo "LSP Gateway Coverage Baseline"
echo "==========================="
echo

# Clean any existing coverage files
rm -f coverage*.out coverage*.html

# Package-specific coverage measurement
declare -A PACKAGES_COVERAGE
declare -A PACKAGES_STATUS

# List of packages to test
PACKAGES=(
    "internal/config"
    "internal/gateway" 
    "internal/setup"
    "internal/platform"
    "mcp"
    "internal/common"
    "internal/types"
    "internal/testutil"
    "internal/cli"
    "internal/installer"
    "internal/transport"
    "internal/integration"
)

echo "📊 Testing packages individually for coverage..."
echo

for pkg in "${PACKAGES[@]}"; do
    echo -n "Testing $pkg... "
    
    # Run test with timeout and capture coverage
    if timeout 30s go test -cover -short "./$pkg" 2>&1 | grep -E "coverage:|no test files" > /tmp/coverage_output.tmp; then
        # Extract coverage percentage
        coverage=$(grep "coverage:" /tmp/coverage_output.tmp | tail -1 | sed 's/.*coverage: //' | sed 's/ of statements//')
        
        if [ -n "$coverage" ]; then
            PACKAGES_COVERAGE["$pkg"]="$coverage"
            PACKAGES_STATUS["$pkg"]="MEASURED"
            echo "✅ $coverage"
        else
            # Check for no test files
            if grep -q "no test files" /tmp/coverage_output.tmp; then
                PACKAGES_COVERAGE["$pkg"]="N/A (no tests)"
                PACKAGES_STATUS["$pkg"]="NO_TESTS"
                echo "➖ No test files"
            else
                PACKAGES_COVERAGE["$pkg"]="FAILED"
                PACKAGES_STATUS["$pkg"]="FAILED"
                echo "❌ Failed to measure"
            fi
        fi
    else
        # Test failed or timed out
        PACKAGES_COVERAGE["$pkg"]="TIMEOUT/ERROR"
        PACKAGES_STATUS["$pkg"]="ERROR"
        echo "⏱️ Timeout or error"
    fi
done

echo
echo "==========================="
echo "COVERAGE BASELINE REPORT"
echo "==========================="
echo

# Sort packages by coverage percentage (numeric sort)
printf "%-25s %-20s %-10s\n" "PACKAGE" "COVERAGE" "STATUS"
printf "%-25s %-20s %-10s\n" "-------" "--------" "------"

total_measured=0
total_coverage=0
coverage_count=0

for pkg in "${PACKAGES[@]}"; do
    coverage="${PACKAGES_COVERAGE[$pkg]}"
    status="${PACKAGES_STATUS[$pkg]}"
    
    printf "%-25s %-20s %-10s\n" "$pkg" "$coverage" "$status"
    
    # Calculate averages for numeric coverage
    if [[ "$coverage" =~ ^[0-9]+\.[0-9]+% ]] || [[ "$coverage" =~ ^[0-9]+% ]]; then
        numeric_coverage=$(echo "$coverage" | sed 's/%//')
        total_coverage=$(echo "$total_coverage + $numeric_coverage" | bc -l)
        ((coverage_count++))
    fi
    
    if [ "$status" = "MEASURED" ]; then
        ((total_measured++))
    fi
done

echo
echo "==========================="
echo "BASELINE SUMMARY"
echo "==========================="

if [ $coverage_count -gt 0 ]; then
    avg_coverage=$(echo "scale=1; $total_coverage / $coverage_count" | bc -l)
    echo "📈 Average Coverage: ${avg_coverage}%"
else
    echo "📈 Average Coverage: Unable to calculate"
fi

echo "📦 Packages Measured: $total_measured/${#PACKAGES[@]}"
echo "📊 Packages with Tests: $coverage_count/${#PACKAGES[@]}"

# Identify high and low coverage packages
echo
echo "🎯 HIGH COVERAGE (>80%):"
for pkg in "${PACKAGES[@]}"; do
    coverage="${PACKAGES_COVERAGE[$pkg]}"
    if [[ "$coverage" =~ ^[0-9]+\.[0-9]+% ]] || [[ "$coverage" =~ ^[0-9]+% ]]; then
        numeric_coverage=$(echo "$coverage" | sed 's/%//')
        if (( $(echo "$numeric_coverage > 80" | bc -l) )); then
            printf "  ✅ %-25s %s\n" "$pkg" "$coverage"
        fi
    fi
done

echo
echo "⚠️  LOW COVERAGE (<60%):"
for pkg in "${PACKAGES[@]}"; do
    coverage="${PACKAGES_COVERAGE[$pkg]}"
    if [[ "$coverage" =~ ^[0-9]+\.[0-9]+% ]] || [[ "$coverage" =~ ^[0-9]+% ]]; then
        numeric_coverage=$(echo "$coverage" | sed 's/%//')
        if (( $(echo "$numeric_coverage < 60" | bc -l) )); then
            printf "  ❌ %-25s %s\n" "$pkg" "$coverage"
        fi
    fi
done

echo
echo "🚧 MEASUREMENT ISSUES:"
for pkg in "${PACKAGES[@]}"; do
    status="${PACKAGES_STATUS[$pkg]}"
    coverage="${PACKAGES_COVERAGE[$pkg]}"
    if [ "$status" = "ERROR" ] || [ "$status" = "FAILED" ]; then
        printf "  🔧 %-25s %s\n" "$pkg" "$coverage"
    fi
done

echo
echo "==========================="
echo "RECOMMENDATIONS"
echo "==========================="
echo

echo "🔧 INFRASTRUCTURE FIXES NEEDED:"
echo "  1. Fix test compilation errors in packages with FAILED status"
echo "  2. Optimize slow-running tests causing timeouts"  
echo "  3. Add timeout handling for network-dependent tests"

echo
echo "📈 COVERAGE IMPROVEMENT TARGETS:"
if [ $coverage_count -gt 0 ]; then
    echo "  1. Target 60%+ coverage for all packages with <60%"
    echo "  2. Add test files for packages with 'N/A (no tests)'"
    echo "  3. Focus on critical business logic in core packages"
else
    echo "  1. First fix measurement infrastructure issues"
    echo "  2. Then establish coverage targets"
fi

echo
echo "✅ Coverage baseline measurement complete!"
echo "📄 Next: Fix measurement issues and establish improvement plan"