#!/bin/bash

# LSP Benchmark Comparison Tool
# Compares current LSP test results against baseline for performance regression detection

set -euo pipefail

# Configuration
BASELINE_FILE=""
CURRENT_FILE=""
OUTPUT_FILE=""
THRESHOLD=10  # Performance regression threshold percentage
VERBOSE=false

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Compare LSP performance results against baseline for regression detection

OPTIONS:
    --baseline=FILE     Baseline results JSON file (required)
    --current=FILE      Current results JSON file (required)
    --output=FILE       Output comparison JSON file (required)
    --threshold=NUM     Regression threshold percentage (default: 10)
    --verbose           Enable verbose output
    -h, --help          Show this help message

EXAMPLES:
    $0 --baseline=baseline.json --current=current.json --output=comparison.json
    $0 --baseline=old.json --current=new.json --output=diff.json --threshold=15

EOF
}

log() {
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "${BLUE}[INFO]${NC} $1" >&2
    fi
}

error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
    exit 1
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" >&2
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1" >&2
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --baseline=*)
            BASELINE_FILE="${1#*=}"
            shift
            ;;
        --current=*)
            CURRENT_FILE="${1#*=}"
            shift
            ;;
        --output=*)
            OUTPUT_FILE="${1#*=}"
            shift
            ;;
        --threshold=*)
            THRESHOLD="${1#*=}"
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            error "Unknown option: $1"
            ;;
    esac
done

# Validate required parameters
if [[ -z "$BASELINE_FILE" ]]; then
    error "Baseline file is required. Use --baseline=FILE"
fi

if [[ -z "$CURRENT_FILE" ]]; then
    error "Current file is required. Use --current=FILE"
fi

if [[ -z "$OUTPUT_FILE" ]]; then
    error "Output file is required. Use --output=FILE"
fi

if [[ ! -f "$BASELINE_FILE" ]]; then
    error "Baseline file does not exist: $BASELINE_FILE"
fi

if [[ ! -f "$CURRENT_FILE" ]]; then
    error "Current file does not exist: $CURRENT_FILE"
fi

# Validate JSON format
if ! jq empty "$BASELINE_FILE" 2>/dev/null; then
    error "Invalid JSON format in baseline file: $BASELINE_FILE"
fi

if ! jq empty "$CURRENT_FILE" 2>/dev/null; then
    error "Invalid JSON format in current file: $CURRENT_FILE"
fi

# Extract key metrics
log "Extracting performance metrics from baseline and current results"

# Baseline metrics
BASELINE_AVG_RESPONSE=$(jq -r '.performance.avg_response_time // 0' "$BASELINE_FILE" | sed 's/ms//')
BASELINE_STARTUP_TIME=$(jq -r '.performance.server_startup_time // 0' "$BASELINE_FILE" | sed 's/ms//')
BASELINE_TOTAL_DURATION=$(jq -r '.performance.total_duration // "0s"' "$BASELINE_FILE" | sed 's/s$//')
BASELINE_SUCCESS_RATE=$(jq -r '.summary.passed_tests / .summary.total_tests * 100 // 0' "$BASELINE_FILE")

# Current metrics
CURRENT_AVG_RESPONSE=$(jq -r '.performance.avg_response_time // 0' "$CURRENT_FILE" | sed 's/ms//')
CURRENT_STARTUP_TIME=$(jq -r '.performance.server_startup_time // 0' "$CURRENT_FILE" | sed 's/ms//')
CURRENT_TOTAL_DURATION=$(jq -r '.performance.total_duration // "0s"' "$CURRENT_FILE" | sed 's/s$//')
CURRENT_SUCCESS_RATE=$(jq -r '.summary.passed_tests / .summary.total_tests * 100 // 0' "$CURRENT_FILE")

log "Baseline - Avg Response: ${BASELINE_AVG_RESPONSE}ms, Startup: ${BASELINE_STARTUP_TIME}ms"
log "Current - Avg Response: ${CURRENT_AVG_RESPONSE}ms, Startup: ${CURRENT_STARTUP_TIME}ms"

# Calculate percentage changes
calculate_change() {
    local baseline=$1
    local current=$2
    
    if [[ "$baseline" == "0" ]] || [[ -z "$baseline" ]]; then
        echo "N/A"
        return
    fi
    
    echo "scale=2; ($current - $baseline) / $baseline * 100" | bc
}

AVG_RESPONSE_CHANGE=$(calculate_change "$BASELINE_AVG_RESPONSE" "$CURRENT_AVG_RESPONSE")
STARTUP_TIME_CHANGE=$(calculate_change "$BASELINE_STARTUP_TIME" "$CURRENT_STARTUP_TIME")
SUCCESS_RATE_CHANGE=$(calculate_change "$BASELINE_SUCCESS_RATE" "$CURRENT_SUCCESS_RATE")

log "Performance changes: Response: ${AVG_RESPONSE_CHANGE}%, Startup: ${STARTUP_TIME_CHANGE}%"

# Determine regression status
check_regression() {
    local change=$1
    local metric_name=$2
    
    if [[ "$change" == "N/A" ]]; then
        echo "unknown"
        return
    fi
    
    # Use bc for float comparison
    if echo "$change > $THRESHOLD" | bc -l | grep -q 1; then
        warn "Performance regression detected in $metric_name: ${change}% (threshold: ${THRESHOLD}%)"
        echo "regression"
    elif echo "$change < -$THRESHOLD" | bc -l | grep -q 1; then
        success "Performance improvement in $metric_name: ${change}%"
        echo "improvement"
    else
        log "Performance stable in $metric_name: ${change}%"
        echo "stable"
    fi
}

AVG_RESPONSE_STATUS=$(check_regression "$AVG_RESPONSE_CHANGE" "Average Response Time")
STARTUP_TIME_STATUS=$(check_regression "$STARTUP_TIME_CHANGE" "Startup Time")

# Overall status determination
OVERALL_STATUS="stable"
if [[ "$AVG_RESPONSE_STATUS" == "regression" ]] || [[ "$STARTUP_TIME_STATUS" == "regression" ]]; then
    OVERALL_STATUS="regression"
elif [[ "$AVG_RESPONSE_STATUS" == "improvement" ]] || [[ "$STARTUP_TIME_STATUS" == "improvement" ]]; then
    OVERALL_STATUS="improvement"
fi

# Generate comparison report
log "Generating benchmark comparison report"

cat > "$OUTPUT_FILE" << EOF
{
    "comparison": {
        "generated_at": "$(date -u '+%Y-%m-%dT%H:%M:%SZ')",
        "threshold_percent": $THRESHOLD,
        "overall_status": "$OVERALL_STATUS",
        "baseline_file": "$BASELINE_FILE",
        "current_file": "$CURRENT_FILE",
        "metrics": {
            "avg_response_time": {
                "baseline_ms": $BASELINE_AVG_RESPONSE,
                "current_ms": $CURRENT_AVG_RESPONSE,
                "change_percent": $(echo "$AVG_RESPONSE_CHANGE" | sed 's/N\/A/null/'),
                "status": "$AVG_RESPONSE_STATUS"
            },
            "server_startup_time": {
                "baseline_ms": $BASELINE_STARTUP_TIME,
                "current_ms": $CURRENT_STARTUP_TIME,
                "change_percent": $(echo "$STARTUP_TIME_CHANGE" | sed 's/N\/A/null/'),
                "status": "$STARTUP_TIME_STATUS"
            },
            "success_rate": {
                "baseline_percent": $BASELINE_SUCCESS_RATE,
                "current_percent": $CURRENT_SUCCESS_RATE,
                "change_percent": $(echo "$SUCCESS_RATE_CHANGE" | sed 's/N\/A/null/')
            }
        },
        "summary": {
            "regressions_detected": $(if [[ "$OVERALL_STATUS" == "regression" ]]; then echo "true"; else echo "false"; fi),
            "improvements_detected": $(if [[ "$OVERALL_STATUS" == "improvement" ]]; then echo "true"; else echo "false"; fi),
            "stable_performance": $(if [[ "$OVERALL_STATUS" == "stable" ]]; then echo "true"; else echo "false"; fi)
        }
    }
}
EOF

# Add detailed test comparison if available
if jq -e '.test_results' "$BASELINE_FILE" >/dev/null 2>&1 && jq -e '.test_results' "$CURRENT_FILE" >/dev/null 2>&1; then
    log "Adding detailed test comparison"
    
    # Create temporary files for test comparison
    TEMP_BASELINE=$(mktemp)
    TEMP_CURRENT=$(mktemp)
    
    # Extract test results with durations
    jq -r '.test_results[] | select(.duration != null) | [.name, (.duration | gsub("ms|s"; "") | tonumber)] | @tsv' "$BASELINE_FILE" > "$TEMP_BASELINE"
    jq -r '.test_results[] | select(.duration != null) | [.name, (.duration | gsub("ms|s"; "") | tonumber)] | @tsv' "$CURRENT_FILE" > "$TEMP_CURRENT"
    
    # Generate test comparison
    TEST_COMPARISON_JSON=$(mktemp)
    echo '{"test_comparisons": []}' > "$TEST_COMPARISON_JSON"
    
    while IFS=$'\t' read -r test_name baseline_duration; do
        if grep -q "^$test_name" "$TEMP_CURRENT"; then
            current_duration=$(grep "^$test_name" "$TEMP_CURRENT" | cut -f2)
            
            if [[ "$baseline_duration" != "0" ]]; then
                change=$(echo "scale=2; ($current_duration - $baseline_duration) / $baseline_duration * 100" | bc)
                
                # Determine test status
                test_status="stable"
                if echo "$change > $THRESHOLD" | bc -l | grep -q 1; then
                    test_status="regression"
                elif echo "$change < -$THRESHOLD" | bc -l | grep -q 1; then
                    test_status="improvement"
                fi
                
                # Add to comparison JSON
                jq --arg name "$test_name" \
                   --argjson baseline "$baseline_duration" \
                   --argjson current "$current_duration" \
                   --argjson change "$change" \
                   --arg status "$test_status" \
                   '.test_comparisons += [{
                       name: $name,
                       baseline_duration_ms: $baseline,
                       current_duration_ms: $current,
                       change_percent: $change,
                       status: $status
                   }]' "$TEST_COMPARISON_JSON" > "${TEST_COMPARISON_JSON}.tmp"
                mv "${TEST_COMPARISON_JSON}.tmp" "$TEST_COMPARISON_JSON"
            fi
        fi
    done < "$TEMP_BASELINE"
    
    # Merge test comparison into main output
    jq -s '.[0] * .[1]' "$OUTPUT_FILE" "$TEST_COMPARISON_JSON" > "${OUTPUT_FILE}.tmp"
    mv "${OUTPUT_FILE}.tmp" "$OUTPUT_FILE"
    
    # Cleanup
    rm -f "$TEMP_BASELINE" "$TEMP_CURRENT" "$TEST_COMPARISON_JSON"
fi

success "Benchmark comparison completed: $OUTPUT_FILE"

# Generate summary output for CI
echo
echo "=================================="
echo "LSP Benchmark Comparison Summary"
echo "=================================="
echo
echo "Overall Status: $OVERALL_STATUS"
echo "Regression Threshold: ${THRESHOLD}%"
echo
echo "Performance Metrics:"
echo "- Average Response Time: ${BASELINE_AVG_RESPONSE}ms -> ${CURRENT_AVG_RESPONSE}ms (${AVG_RESPONSE_CHANGE}%) [$AVG_RESPONSE_STATUS]"
echo "- Server Startup Time: ${BASELINE_STARTUP_TIME}ms -> ${CURRENT_STARTUP_TIME}ms (${STARTUP_TIME_CHANGE}%) [$STARTUP_TIME_STATUS]"
echo "- Success Rate: ${BASELINE_SUCCESS_RATE}% -> ${CURRENT_SUCCESS_RATE}% (${SUCCESS_RATE_CHANGE}%)"
echo

# Exit with appropriate code for CI
if [[ "$OVERALL_STATUS" == "regression" ]]; then
    error "Performance regression detected above ${THRESHOLD}% threshold"
else
    success "No significant performance regression detected"
fi