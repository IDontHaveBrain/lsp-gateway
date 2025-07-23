#!/bin/bash

# LSP Performance Dashboard Generator
# Generates HTML and JSON performance dashboards from LSP test results

set -euo pipefail

# Configuration
INPUT_FILE=""
OUTPUT_DIR=""
FORMAT="html,json"
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

Generate LSP performance dashboard from test results

OPTIONS:
    --input=FILE        Input JSON results file (required)
    --output=DIR        Output directory for dashboard (required)
    --format=FORMATS    Output formats: html,json,csv (default: html,json)
    --verbose           Enable verbose output
    -h, --help          Show this help message

EXAMPLES:
    $0 --input=results.json --output=dashboard --format=html
    $0 --input=results.json --output=dashboard --format=html,json,csv --verbose

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
        --input=*)
            INPUT_FILE="${1#*=}"
            shift
            ;;
        --output=*)
            OUTPUT_DIR="${1#*=}"
            shift
            ;;
        --format=*)
            FORMAT="${1#*=}"
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
if [[ -z "$INPUT_FILE" ]]; then
    error "Input file is required. Use --input=FILE"
fi

if [[ -z "$OUTPUT_DIR" ]]; then
    error "Output directory is required. Use --output=DIR"
fi

if [[ ! -f "$INPUT_FILE" ]]; then
    error "Input file does not exist: $INPUT_FILE"
fi

# Validate JSON format
if ! jq empty "$INPUT_FILE" 2>/dev/null; then
    error "Invalid JSON format in input file: $INPUT_FILE"
fi

# Create output directory
mkdir -p "$OUTPUT_DIR"
log "Created output directory: $OUTPUT_DIR"

# Extract performance data
log "Extracting performance data from $INPUT_FILE"

TOTAL_TESTS=$(jq -r '.summary.total_tests // 0' "$INPUT_FILE")
PASSED_TESTS=$(jq -r '.summary.passed_tests // 0' "$INPUT_FILE")
FAILED_TESTS=$(jq -r '.summary.failed_tests // 0' "$INPUT_FILE")
TOTAL_DURATION=$(jq -r '.performance.total_duration // "0s"' "$INPUT_FILE")
AVG_RESPONSE_TIME=$(jq -r '.performance.avg_response_time // "0"' "$INPUT_FILE")
SERVER_STARTUP_TIME=$(jq -r '.performance.server_startup_time // "0"' "$INPUT_FILE")
TOTAL_REQUESTS=$(jq -r '.performance.total_requests // 0' "$INPUT_FILE")

# Calculate success rate
SUCCESS_RATE=0
if [[ "$TOTAL_TESTS" -gt 0 ]]; then
    SUCCESS_RATE=$(echo "scale=2; $PASSED_TESTS * 100 / $TOTAL_TESTS" | bc)
fi

log "Performance summary: $PASSED_TESTS/$TOTAL_TESTS tests passed (${SUCCESS_RATE}%)"

# Generate HTML dashboard if requested
if [[ "$FORMAT" == *"html"* ]]; then
    log "Generating HTML dashboard"
    
    HTML_FILE="$OUTPUT_DIR/index.html"
    
    cat > "$HTML_FILE" << EOF
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>LSP Performance Dashboard</title>
    <style>
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; 
            margin: 0; 
            padding: 20px; 
            background-color: #f5f7fa; 
            color: #2d3748;
        }
        .container { max-width: 1200px; margin: 0 auto; }
        .header { 
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); 
            color: white; 
            padding: 30px; 
            border-radius: 10px; 
            margin-bottom: 30px; 
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
        }
        .metrics-grid { 
            display: grid; 
            grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); 
            gap: 20px; 
            margin-bottom: 30px; 
        }
        .metric-card { 
            background: white; 
            padding: 25px; 
            border-radius: 10px; 
            box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1); 
            border-left: 4px solid #4299e1;
        }
        .metric-card.success { border-left-color: #48bb78; }
        .metric-card.warning { border-left-color: #ed8936; }
        .metric-card.error { border-left-color: #f56565; }
        .metric-value { 
            font-size: 2.5em; 
            font-weight: bold; 
            margin-bottom: 5px; 
        }
        .metric-label { 
            color: #718096; 
            font-weight: 500; 
        }
        .section { 
            background: white; 
            padding: 25px; 
            border-radius: 10px; 
            margin-bottom: 20px; 
            box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
        }
        .section-title { 
            font-size: 1.5em; 
            font-weight: bold; 
            margin-bottom: 20px; 
            color: #2d3748;
        }
        .status-success { color: #48bb78; }
        .status-error { color: #f56565; }
        .progress-bar { 
            width: 100%; 
            height: 20px; 
            background-color: #e2e8f0; 
            border-radius: 10px; 
            overflow: hidden; 
        }
        .progress-fill { 
            height: 100%; 
            background: linear-gradient(90deg, #48bb78 0%, #38a169 100%); 
            transition: width 0.3s ease;
        }
        .timestamp { 
            color: #718096; 
            font-size: 0.9em; 
            margin-top: 20px;
        }
        table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 15px;
        }
        th, td {
            text-align: left;
            padding: 12px;
            border-bottom: 1px solid #e2e8f0;
        }
        th {
            background-color: #f7fafc;
            font-weight: 600;
            color: #4a5568;
        }
        .method-badge {
            background-color: #edf2f7;
            color: #4a5568;
            padding: 4px 8px;
            border-radius: 4px;
            font-size: 0.85em;
            font-weight: 500;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>LSP Performance Dashboard</h1>
            <p>Comprehensive analysis of LSP server performance and validation results</p>
        </div>

        <div class="metrics-grid">
            <div class="metric-card success">
                <div class="metric-value">${SUCCESS_RATE}%</div>
                <div class="metric-label">Success Rate</div>
            </div>
            <div class="metric-card">
                <div class="metric-value">$TOTAL_TESTS</div>
                <div class="metric-label">Total Tests</div>
            </div>
            <div class="metric-card">
                <div class="metric-value">${AVG_RESPONSE_TIME}ms</div>
                <div class="metric-label">Avg Response Time</div>
            </div>
            <div class="metric-card">
                <div class="metric-value">$TOTAL_DURATION</div>
                <div class="metric-label">Total Duration</div>
            </div>
        </div>

        <div class="section">
            <div class="section-title">Test Results Overview</div>
            <div style="margin-bottom: 15px;">
                <strong>Passed:</strong> <span class="status-success">$PASSED_TESTS</span> | 
                <strong>Failed:</strong> <span class="status-error">$FAILED_TESTS</span> |
                <strong>Total Requests:</strong> $TOTAL_REQUESTS
            </div>
            <div class="progress-bar">
                <div class="progress-fill" style="width: ${SUCCESS_RATE}%;"></div>
            </div>
        </div>

        <div class="section">
            <div class="section-title">Performance Metrics</div>
            <table>
                <thead>
                    <tr>
                        <th>Metric</th>
                        <th>Value</th>
                        <th>Description</th>
                    </tr>
                </thead>
                <tbody>
                    <tr>
                        <td>Server Startup Time</td>
                        <td>${SERVER_STARTUP_TIME}ms</td>
                        <td>Time to initialize LSP servers</td>
                    </tr>
                    <tr>
                        <td>Average Response Time</td>
                        <td>${AVG_RESPONSE_TIME}ms</td>
                        <td>Mean response time for LSP requests</td>
                    </tr>
                    <tr>
                        <td>Total Requests</td>
                        <td>$TOTAL_REQUESTS</td>
                        <td>Number of LSP requests processed</td>
                    </tr>
                    <tr>
                        <td>Total Test Duration</td>
                        <td>$TOTAL_DURATION</td>
                        <td>Complete test suite execution time</td>
                    </tr>
                </tbody>
            </table>
        </div>

EOF

    # Add test results table if available
    if jq -e '.test_results' "$INPUT_FILE" >/dev/null 2>&1; then
        cat >> "$HTML_FILE" << EOF
        <div class="section">
            <div class="section-title">Test Results Details</div>
            <table>
                <thead>
                    <tr>
                        <th>Test Name</th>
                        <th>Method</th>
                        <th>Status</th>
                        <th>Duration</th>
                    </tr>
                </thead>
                <tbody>
EOF

        # Extract and format test results
        jq -r '.test_results[] | [.name, .method, (if .passed then "✅ PASS" else "❌ FAIL" end), (.duration // "N/A")] | @tsv' "$INPUT_FILE" | \
        while IFS=$'\t' read -r name method status duration; do
            status_class=""
            if [[ "$status" == *"PASS"* ]]; then
                status_class="status-success"
            else
                status_class="status-error"
            fi
            
            cat >> "$HTML_FILE" << EOF
                    <tr>
                        <td>$name</td>
                        <td><span class="method-badge">$method</span></td>
                        <td class="$status_class">$status</td>
                        <td>$duration</td>
                    </tr>
EOF
        done

        cat >> "$HTML_FILE" << EOF
                </tbody>
            </table>
        </div>
EOF
    fi

    # Close HTML
    cat >> "$HTML_FILE" << EOF
        <div class="timestamp">
            Generated on $(date '+%Y-%m-%d %H:%M:%S UTC')
        </div>
    </div>
</body>
</html>
EOF

    success "HTML dashboard generated: $HTML_FILE"
fi

# Generate JSON dashboard if requested
if [[ "$FORMAT" == *"json"* ]]; then
    log "Generating JSON dashboard data"
    
    JSON_FILE="$OUTPUT_DIR/dashboard.json"
    
    # Create comprehensive dashboard JSON
    jq --argjson success_rate "$SUCCESS_RATE" \
       --arg generated_at "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" \
       '{
           dashboard: {
               generated_at: $generated_at,
               summary: {
                   total_tests: .summary.total_tests,
                   passed_tests: .summary.passed_tests,
                   failed_tests: .summary.failed_tests,
                   success_rate: $success_rate
               },
               performance: .performance,
               metrics: {
                   avg_response_time_ms: (.performance.avg_response_time // 0 | tonumber),
                   server_startup_time_ms: (.performance.server_startup_time // 0 | tonumber),
                   total_requests: (.performance.total_requests // 0),
                   total_duration: .performance.total_duration
               },
               test_breakdown: [
                   .test_results[]? | {
                       name: .name,
                       method: .method,
                       passed: .passed,
                       duration: .duration,
                       error: .error
                   }
               ]
           }
       }' "$INPUT_FILE" > "$JSON_FILE"
    
    success "JSON dashboard data generated: $JSON_FILE"
fi

# Generate CSV if requested
if [[ "$FORMAT" == *"csv"* ]]; then
    log "Generating CSV performance data"
    
    CSV_FILE="$OUTPUT_DIR/performance.csv"
    
    # Create CSV header
    echo "Test Name,Method,Status,Duration,Error" > "$CSV_FILE"
    
    # Extract test results to CSV
    if jq -e '.test_results' "$INPUT_FILE" >/dev/null 2>&1; then
        jq -r '.test_results[] | [.name, .method, (if .passed then "PASS" else "FAIL" end), (.duration // ""), (.error // "")] | @csv' "$INPUT_FILE" >> "$CSV_FILE"
        success "CSV performance data generated: $CSV_FILE"
    else
        warn "No test results found for CSV generation"
    fi
fi

# Generate summary file
SUMMARY_FILE="$OUTPUT_DIR/summary.txt"
cat > "$SUMMARY_FILE" << EOF
LSP Performance Dashboard Summary
=================================

Generated: $(date '+%Y-%m-%d %H:%M:%S UTC')
Input File: $INPUT_FILE

Test Results:
- Total Tests: $TOTAL_TESTS
- Passed: $PASSED_TESTS  
- Failed: $FAILED_TESTS
- Success Rate: ${SUCCESS_RATE}%

Performance Metrics:
- Total Duration: $TOTAL_DURATION
- Average Response Time: ${AVG_RESPONSE_TIME}ms
- Server Startup Time: ${SERVER_STARTUP_TIME}ms
- Total Requests: $TOTAL_REQUESTS

Dashboard Files Generated:
EOF

if [[ "$FORMAT" == *"html"* ]]; then
    echo "- HTML Dashboard: $OUTPUT_DIR/index.html" >> "$SUMMARY_FILE"
fi

if [[ "$FORMAT" == *"json"* ]]; then
    echo "- JSON Data: $OUTPUT_DIR/dashboard.json" >> "$SUMMARY_FILE"
fi

if [[ "$FORMAT" == *"csv"* ]]; then
    echo "- CSV Data: $OUTPUT_DIR/performance.csv" >> "$SUMMARY_FILE"
fi

success "Performance dashboard generation completed successfully"
log "Summary written to: $SUMMARY_FILE"

# Output summary to stdout for CI integration
cat "$SUMMARY_FILE"