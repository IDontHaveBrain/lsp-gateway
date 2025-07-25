#!/bin/bash
set -euo pipefail

# Test Report Generation Script for LSP Gateway
# Generates comprehensive test reports from CI results

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
RESULTS_DIR="${PROJECT_ROOT}/test-results"
REPORTS_DIR="${RESULTS_DIR}/reports"
COVERAGE_DIR="${RESULTS_DIR}/coverage"

# Configuration
REPORT_TITLE="LSP Gateway Test Report"
MAX_LOG_LINES=100

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Help function
show_help() {
    cat << EOF
LSP Gateway Test Report Generator

Usage: $0 [OPTIONS] [REPORT_TYPE]

REPORT_TYPE:
    summary         Generate test summary report (default)
    detailed        Generate detailed test report with logs
    coverage        Generate coverage report
    performance     Generate performance report
    all             Generate all reports

OPTIONS:
    -h, --help      Show this help message
    -v, --verbose   Enable verbose output
    -o, --output DIR Set output directory (default: $REPORTS_DIR)
    -f, --format FORMAT Set output format (html, md, json, xml)
    -t, --title TITLE Set report title
    --include-logs  Include full test logs in report
    --timestamp     Add timestamp to report filenames
    --upload        Prepare reports for upload to CI artifacts

Examples:
    $0 summary                           # Generate summary report
    $0 detailed --include-logs          # Generate detailed report with logs
    $0 coverage -f html                 # Generate HTML coverage report
    $0 all -o /tmp/reports --timestamp  # Generate all reports with timestamps

EOF
}

# Initialize directories
init_dirs() {
    log_info "Initializing report directories..."
    mkdir -p "$RESULTS_DIR" "$REPORTS_DIR" "$COVERAGE_DIR"
}

# Get Git information
get_git_info() {
    cat << EOF
{
    "commit": "$(git rev-parse HEAD 2>/dev/null || echo "unknown")",
    "short_commit": "$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")",
    "branch": "$(git branch --show-current 2>/dev/null || echo "unknown")",
    "author": "$(git log -1 --format='%an' 2>/dev/null || echo "unknown")",
    "message": "$(git log -1 --format='%s' 2>/dev/null || echo "unknown")",
    "timestamp": "$(git log -1 --format='%ci' 2>/dev/null || echo "unknown")"
}
EOF
}

# Get environment information
get_env_info() {
    cat << EOF
{
    "go_version": "$(go version 2>/dev/null | cut -d' ' -f3 || echo "unknown")",
    "os": "$(uname -s)",
    "arch": "$(uname -m)",
    "hostname": "$(hostname)",
    "user": "$(whoami)",
    "pwd": "$PWD",
    "ci": "${CI:-false}",
    "github_actions": "${GITHUB_ACTIONS:-false}",
    "runner_os": "${RUNNER_OS:-unknown}"
}
EOF
}

# Parse test logs to extract test results
parse_test_results() {
    local log_file="$1"
    local test_type="$2"
    
    if [[ ! -f "$log_file" ]]; then
        echo '{"status": "not_run", "tests": 0, "passed": 0, "failed": 0, "skipped": 0}'
        return
    fi
    
    python3 << EOF
import re
import json

def parse_go_test_output(filename):
    results = {
        "status": "unknown",
        "tests": 0,
        "passed": 0,
        "failed": 0,
        "skipped": 0,
        "duration": "0s",
        "failures": [],
        "coverage": None
    }
    
    with open(filename, 'r') as f:
        content = f.read()
    
    # Check overall result
    if "PASS" in content and not re.search(r'FAIL.*\n.*FAIL', content):
        results["status"] = "passed"
    elif "FAIL" in content:
        results["status"] = "failed"
    elif content.strip() == "":
        results["status"] = "not_run"
    else:
        results["status"] = "unknown"
    
    # Extract test counts
    # Look for patterns like: "--- PASS: TestName (0.00s)"
    pass_matches = re.findall(r'--- PASS: (Test\w+)', content)
    fail_matches = re.findall(r'--- FAIL: (Test\w+)', content)
    skip_matches = re.findall(r'--- SKIP: (Test\w+)', content)
    
    results["passed"] = len(pass_matches)
    results["failed"] = len(fail_matches)
    results["skipped"] = len(skip_matches)
    results["tests"] = results["passed"] + results["failed"] + results["skipped"]
    
    # Extract failures with details
    for fail_match in fail_matches:
        # Try to find the failure reason
        failure_pattern = rf'--- FAIL: {re.escape(fail_match)}.*?\n(.*?)(?=--- |$)'
        failure_detail = re.search(failure_pattern, content, re.DOTALL)
        
        failure_info = {
            "test": fail_match,
            "detail": failure_detail.group(1).strip() if failure_detail else "No details available"
        }
        results["failures"].append(failure_info)
    
    # Extract duration
    duration_match = re.search(r'PASS|FAIL.*in ([\d.]+s)', content)
    if duration_match:
        results["duration"] = duration_match.group(1)
    
    # Extract coverage if present
    coverage_match = re.search(r'coverage: ([\d.]+)% of statements', content)
    if coverage_match:
        results["coverage"] = float(coverage_match.group(1))
    
    return results

result = parse_go_test_output('$log_file')
print(json.dumps(result, indent=2))
EOF
}

# Generate summary report
generate_summary_report() {
    log_info "Generating test summary report..."
    
    local output_file
    if [[ "$FORMAT" == "html" ]]; then
        output_file="$OUTPUT_DIR/test-summary.html"
    elif [[ "$FORMAT" == "json" ]]; then
        output_file="$OUTPUT_DIR/test-summary.json"
    else
        output_file="$OUTPUT_DIR/test-summary.md"
    fi
    
    if [[ "$TIMESTAMP" == "true" ]]; then
        local timestamp=$(date +"%Y%m%d_%H%M%S")
        output_file="${output_file%.*}_${timestamp}.${output_file##*.}"
    fi
    
    log_info "Creating summary report: $output_file"
    
    # Collect test results
    local unit_results integration_results performance_results lsp_results
    
    unit_results=$(parse_test_results "$RESULTS_DIR/unit-tests.log" "unit")
    integration_results=$(parse_test_results "$RESULTS_DIR/integration-tests.log" "integration")
    performance_results=$(parse_test_results "$RESULTS_DIR/performance-tests.log" "performance")
    lsp_results=$(parse_test_results "$RESULTS_DIR/lsp-validation.log" "lsp")
    
    # Get coverage information
    local coverage_percent="N/A"
    if [[ -f "$RESULTS_DIR/coverage-env.txt" ]]; then
        coverage_percent=$(grep "COVERAGE_PERCENT=" "$RESULTS_DIR/coverage-env.txt" | cut -d'=' -f2)
    fi
    
    if [[ "$FORMAT" == "json" ]]; then
        # Generate JSON report
        cat > "$output_file" << EOF
{
    "title": "$REPORT_TITLE",
    "generated": "$(date -Iseconds)",
    "git": $(get_git_info),
    "environment": $(get_env_info),
    "coverage": {
        "total_percent": "$coverage_percent"
    },
    "tests": {
        "unit": $unit_results,
        "integration": $integration_results,
        "performance": $performance_results,
        "lsp_validation": $lsp_results
    }
}
EOF
    elif [[ "$FORMAT" == "html" ]]; then
        # Generate HTML report
        generate_html_summary_report "$output_file" "$unit_results" "$integration_results" "$performance_results" "$lsp_results" "$coverage_percent"
    else
        # Generate Markdown report
        generate_markdown_summary_report "$output_file" "$unit_results" "$integration_results" "$performance_results" "$lsp_results" "$coverage_percent"
    fi
    
    log_success "Summary report generated: $output_file"
}

# Generate Markdown summary report
generate_markdown_summary_report() {
    local output_file="$1"
    local unit_results="$2"
    local integration_results="$3"
    local performance_results="$4"
    local lsp_results="$5"
    local coverage_percent="$6"
    
    cat > "$output_file" << EOF
# $REPORT_TITLE

**Generated:** $(date)
**Commit:** $(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
**Branch:** $(git branch --show-current 2>/dev/null || echo "unknown")

## Summary

| Test Suite | Status | Tests | Passed | Failed | Skipped | Duration |
|------------|--------|-------|--------|--------|---------|----------|
EOF
    
    # Add test results to table
    python3 << 'PYTHON_EOF'
import json
import sys

def format_status(status):
    if status == "passed":
        return "✅ PASSED"
    elif status == "failed":
        return "❌ FAILED"
    elif status == "not_run":
        return "⏭️ NOT RUN"
    else:
        return "❓ UNKNOWN"

test_suites = [
    ("Unit Tests", '$unit_results'),
    ("Integration Tests", '$integration_results'),
    ("Performance Tests", '$performance_results'),
    ("LSP Validation", '$lsp_results')
]

for name, results_json in test_suites:
    try:
        results = json.loads(results_json)
        status = format_status(results["status"])
        tests = results["tests"]
        passed = results["passed"]
        failed = results["failed"]
        skipped = results["skipped"]
        duration = results["duration"]
        
        print(f"| {name} | {status} | {tests} | {passed} | {failed} | {skipped} | {duration} |")
    except Exception as e:
        print(f"| {name} | ❓ ERROR | - | - | - | - | - |")
PYTHON_EOF
    
    cat >> "$output_file" << EOF

## Code Coverage

**Total Coverage:** $coverage_percent%

## Environment

- **Go Version:** $(go version 2>/dev/null | cut -d' ' -f3 || echo "unknown")
- **OS:** $(uname -s) $(uname -m)
- **CI:** ${GITHUB_ACTIONS:-false}

EOF
    
    # Add failure details if any
    python3 << 'PYTHON_EOF'
import json

test_suites = [
    ("Unit Tests", '$unit_results'),
    ("Integration Tests", '$integration_results'),
    ("Performance Tests", '$performance_results'),
    ("LSP Validation", '$lsp_results')
]

has_failures = False
for name, results_json in test_suites:
    try:
        results = json.loads(results_json)
        if results["failures"]:
            if not has_failures:
                print("## Failures")
                print("")
                has_failures = True
            
            print(f"### {name}")
            print("")
            for failure in results["failures"]:
                test_name = failure["test"]
                detail = failure["detail"]
                print(f"**{test_name}:**")
                print("```")
                # Limit detail length
                if len(detail) > 1000:
                    detail = detail[:1000] + "... (truncated)"
                print(detail)
                print("```")
                print("")
    except:
        pass
PYTHON_EOF
}

# Generate HTML summary report
generate_html_summary_report() {
    local output_file="$1"
    local unit_results="$2"
    local integration_results="$3"
    local performance_results="$4"
    local lsp_results="$5"
    local coverage_percent="$6"
    
    cat > "$output_file" << EOF
<!DOCTYPE html>
<html>
<head>
    <title>$REPORT_TITLE</title>
    <meta charset="UTF-8">
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            margin: 0;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background: white;
            padding: 30px;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        .header {
            border-bottom: 2px solid #eee;
            padding-bottom: 20px;
            margin-bottom: 30px;
        }
        .status-passed { color: #28a745; }
        .status-failed { color: #dc3545; }
        .status-not-run { color: #6c757d; }
        .status-unknown { color: #ffc107; }
        table {
            width: 100%;
            border-collapse: collapse;
            margin: 20px 0;
        }
        th, td {
            text-align: left;
            padding: 12px;
            border-bottom: 1px solid #ddd;
        }
        th {
            background-color: #f8f9fa;
            font-weight: 600;
        }
        .metric-card {
            display: inline-block;
            margin: 10px;
            padding: 20px;
            background: #f8f9fa;
            border-radius: 6px;
            text-align: center;
            min-width: 120px;
        }
        .metric-value {
            font-size: 2em;
            font-weight: bold;
            color: #007bff;
        }
        .failure-detail {
            background: #f8f9fa;
            border-left: 4px solid #dc3545;
            padding: 15px;
            margin: 10px 0;
            border-radius: 4px;
        }
        .failure-detail pre {
            background: #fff;
            padding: 10px;
            border-radius: 4px;
            overflow-x: auto;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>$REPORT_TITLE</h1>
            <p><strong>Generated:</strong> $(date)</p>
            <p><strong>Commit:</strong> $(git rev-parse --short HEAD 2>/dev/null || echo "unknown")</p>
            <p><strong>Branch:</strong> $(git branch --show-current 2>/dev/null || echo "unknown")</p>
        </div>
        
        <h2>Summary</h2>
        <table>
            <thead>
                <tr>
                    <th>Test Suite</th>
                    <th>Status</th>
                    <th>Tests</th>
                    <th>Passed</th>
                    <th>Failed</th>
                    <th>Skipped</th>
                    <th>Duration</th>
                </tr>
            </thead>
            <tbody>
EOF
    
    # Add test results to HTML table
    python3 << 'PYTHON_EOF'
import json

def format_status(status):
    if status == "passed":
        return '<span class="status-passed">✅ PASSED</span>'
    elif status == "failed":
        return '<span class="status-failed">❌ FAILED</span>'
    elif status == "not_run":
        return '<span class="status-not-run">⏭️ NOT RUN</span>'
    else:
        return '<span class="status-unknown">❓ UNKNOWN</span>'

test_suites = [
    ("Unit Tests", '$unit_results'),
    ("Integration Tests", '$integration_results'),
    ("Performance Tests", '$performance_results'),
    ("LSP Validation", '$lsp_results')
]

for name, results_json in test_suites:
    try:
        results = json.loads(results_json)
        status = format_status(results["status"])
        tests = results["tests"]
        passed = results["passed"]
        failed = results["failed"]
        skipped = results["skipped"]
        duration = results["duration"]
        
        print(f"                <tr>")
        print(f"                    <td>{name}</td>")
        print(f"                    <td>{status}</td>")
        print(f"                    <td>{tests}</td>")
        print(f"                    <td>{passed}</td>")
        print(f"                    <td>{failed}</td>")
        print(f"                    <td>{skipped}</td>")
        print(f"                    <td>{duration}</td>")
        print(f"                </tr>")
    except Exception as e:
        print(f"                <tr>")
        print(f"                    <td>{name}</td>")
        print(f"                    <td><span class=\"status-unknown\">❓ ERROR</span></td>")
        print(f"                    <td>-</td>")
        print(f"                    <td>-</td>")
        print(f"                    <td>-</td>")
        print(f"                    <td>-</td>")
        print(f"                    <td>-</td>")
        print(f"                </tr>")
PYTHON_EOF
    
    cat >> "$output_file" << EOF
            </tbody>
        </table>
        
        <h2>Code Coverage</h2>
        <div class="metric-card">
            <div class="metric-value">$coverage_percent%</div>
            <div>Total Coverage</div>
        </div>
        
        <h2>Environment</h2>
        <ul>
            <li><strong>Go Version:</strong> $(go version 2>/dev/null | cut -d' ' -f3 || echo "unknown")</li>
            <li><strong>OS:</strong> $(uname -s) $(uname -m)</li>
            <li><strong>CI:</strong> ${GITHUB_ACTIONS:-false}</li>
        </ul>
EOF
    
    # Add failure details if any
    python3 << 'PYTHON_EOF'
import json
import html

test_suites = [
    ("Unit Tests", '$unit_results'),
    ("Integration Tests", '$integration_results'),
    ("Performance Tests", '$performance_results'),
    ("LSP Validation", '$lsp_results')
]

has_failures = False
for name, results_json in test_suites:
    try:
        results = json.loads(results_json)
        if results["failures"]:
            if not has_failures:
                print("        <h2>Failures</h2>")
                has_failures = True
            
            print(f"        <h3>{name}</h3>")
            for failure in results["failures"]:
                test_name = html.escape(failure["test"])
                detail = html.escape(failure["detail"])
                
                # Limit detail length
                if len(detail) > 2000:
                    detail = detail[:2000] + "... (truncated)"
                
                print(f'        <div class="failure-detail">')
                print(f'            <h4>{test_name}</h4>')
                print(f'            <pre>{detail}</pre>')
                print(f'        </div>')
    except:
        pass
PYTHON_EOF
    
    cat >> "$output_file" << EOF
    </div>
</body>
</html>
EOF
}

# Generate detailed report with logs
generate_detailed_report() {
    log_info "Generating detailed test report..."
    
    local output_file="$OUTPUT_DIR/test-detailed.md"
    
    if [[ "$TIMESTAMP" == "true" ]]; then
        local timestamp=$(date +"%Y%m%d_%H%M%S")
        output_file="${output_file%.*}_${timestamp}.${output_file##*.}"
    fi
    
    cat > "$output_file" << EOF
# Detailed Test Report - $REPORT_TITLE

**Generated:** $(date)
**Commit:** $(git rev-parse HEAD 2>/dev/null || echo "unknown")
**Branch:** $(git branch --show-current 2>/dev/null || echo "unknown")

EOF
    
    # Add detailed logs for each test type
    local log_files=(
        "unit-tests.log:Unit Tests"
        "integration-tests.log:Integration Tests"
        "performance-tests.log:Performance Tests"
        "lsp-validation.log:LSP Validation"
    )
    
    for log_entry in "${log_files[@]}"; do
        local log_file="${log_entry%%:*}"
        local log_title="${log_entry##*:}"
        local full_path="$RESULTS_DIR/$log_file"
        
        echo "## $log_title" >> "$output_file"
        echo "" >> "$output_file"
        
        if [[ -f "$full_path" ]]; then
            echo '```' >> "$output_file"
            if [[ "$INCLUDE_LOGS" == "true" ]]; then
                cat "$full_path" >> "$output_file"
            else
                tail -n "$MAX_LOG_LINES" "$full_path" >> "$output_file"
                echo "" >> "$output_file"
                echo "... (showing last $MAX_LOG_LINES lines, use --include-logs for full output)" >> "$output_file"
            fi
            echo '```' >> "$output_file"
        else
            echo "Log file not found: $log_file" >> "$output_file"
        fi
        echo "" >> "$output_file"
    done
    
    log_success "Detailed report generated: $output_file"
}

# Generate coverage report
generate_coverage_report() {
    log_info "Generating coverage report..."
    
    if [[ ! -f "$COVERAGE_DIR/coverage.out" ]]; then
        log_warning "Coverage data not found, skipping coverage report"
        return
    fi
    
    local output_file
    if [[ "$FORMAT" == "html" ]]; then
        output_file="$OUTPUT_DIR/coverage.html"
        go tool cover -html="$COVERAGE_DIR/coverage.out" -o "$output_file"
    else
        output_file="$OUTPUT_DIR/coverage.txt"
        go tool cover -func="$COVERAGE_DIR/coverage.out" > "$output_file"
    fi
    
    if [[ "$TIMESTAMP" == "true" ]]; then
        local timestamp=$(date +"%Y%m%d_%H%M%S")
        local ext="${output_file##*.}"
        local base="${output_file%.*}"
        output_file="${base}_${timestamp}.${ext}"
        cp "${OUTPUT_DIR}/coverage.${ext}" "$output_file"
    fi
    
    log_success "Coverage report generated: $output_file"
}

# Generate performance report
generate_performance_report() {
    log_info "Generating performance report..."
    
    local output_file="$OUTPUT_DIR/performance-report.md"
    
    if [[ "$TIMESTAMP" == "true" ]]; then
        local timestamp=$(date +"%Y%m%d_%H%M%S")
        output_file="${output_file%.*}_${timestamp}.${output_file##*.}"
    fi
    
    cat > "$output_file" << EOF
# Performance Test Report

**Generated:** $(date)
**Commit:** $(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

## Benchmark Results

EOF
    
    if [[ -f "$RESULTS_DIR/benchmark-results.txt" ]]; then
        echo '```' >> "$output_file"
        cat "$RESULTS_DIR/benchmark-results.txt" >> "$output_file"
        echo '```' >> "$output_file"
    else
        echo "No benchmark results found." >> "$output_file"
    fi
    
    log_success "Performance report generated: $output_file"
}

# Prepare reports for upload
prepare_upload() {
    log_info "Preparing reports for CI upload..."
    
    local upload_dir="$OUTPUT_DIR/upload"
    mkdir -p "$upload_dir"
    
    # Copy all reports to upload directory
    find "$OUTPUT_DIR" -name "*.html" -o -name "*.md" -o -name "*.json" -o -name "*.xml" | while read -r file; do
        cp "$file" "$upload_dir/"
    done
    
    # Create upload manifest
    cat > "$upload_dir/manifest.json" << EOF
{
    "generated": "$(date -Iseconds)",
    "commit": "$(git rev-parse HEAD 2>/dev/null || echo "unknown")",
    "files": [
EOF
    
    first=true
    find "$upload_dir" -type f ! -name "manifest.json" | while read -r file; do
        if [[ "$first" != "true" ]]; then
            echo "," >> "$upload_dir/manifest.json"
        fi
        echo "        \"$(basename "$file")\"" >> "$upload_dir/manifest.json"
        first=false
    done
    
    cat >> "$upload_dir/manifest.json" << EOF
    ]
}
EOF
    
    log_success "Reports prepared for upload in: $upload_dir"
}

# Main execution function
main() {
    case "$REPORT_TYPE" in
        "summary")
            generate_summary_report
            ;;
        "detailed")
            generate_detailed_report
            ;;
        "coverage")
            generate_coverage_report
            ;;
        "performance")
            generate_performance_report
            ;;
        "all")
            generate_summary_report
            generate_detailed_report
            generate_coverage_report
            generate_performance_report
            ;;
        *)
            log_error "Unknown report type: $REPORT_TYPE"
            show_help
            exit 1
            ;;
    esac
    
    if [[ "$UPLOAD" == "true" ]]; then
        prepare_upload
    fi
}

# Parse command line arguments
REPORT_TYPE="summary"
FORMAT="md"
OUTPUT_DIR="$REPORTS_DIR"
INCLUDE_LOGS=false
TIMESTAMP=false
UPLOAD=false
VERBOSE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -v|--verbose)
            VERBOSE=true
            set -x
            shift
            ;;
        -o|--output)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        -f|--format)
            FORMAT="$2"
            shift 2
            ;;
        -t|--title)
            REPORT_TITLE="$2"
            shift 2
            ;;
        --include-logs)
            INCLUDE_LOGS=true
            shift
            ;;
        --timestamp)
            TIMESTAMP=true
            shift
            ;;
        --upload)
            UPLOAD=true
            shift
            ;;
        summary|detailed|coverage|performance|all)
            REPORT_TYPE="$1"
            shift
            ;;
        *)
            log_error "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Initialize and run
init_dirs
mkdir -p "$OUTPUT_DIR"
main "$@"