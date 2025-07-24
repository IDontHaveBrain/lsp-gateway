#!/bin/bash
set -euo pipefail

# Performance Regression Detection Script for LSP Gateway
# Compares current performance against baseline and detects regressions

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
RESULTS_DIR="${PROJECT_ROOT}/performance-results"
BASELINES_DIR="${RESULTS_DIR}/baselines"
REPORTS_DIR="${RESULTS_DIR}/reports"

# Configuration
DEFAULT_REGRESSION_THRESHOLD=20  # 20% performance degradation threshold
DEFAULT_MEMORY_THRESHOLD=15      # 15% memory increase threshold
BENCHMARK_RUNS=5                 # Number of benchmark runs for statistical significance
BASELINE_RETENTION_DAYS=90       # How long to keep baseline data

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
LSP Gateway Performance Regression Detection

Usage: $0 [OPTIONS] [COMMAND]

COMMANDS:
    baseline        Create performance baseline from current code
    check           Check current performance against baseline
    compare         Compare two specific baselines
    clean           Clean old baseline data
    report          Generate performance trend report

OPTIONS:
    -h, --help              Show this help message
    -v, --verbose           Enable verbose output
    -t, --threshold PERCENT Set regression threshold (default: $DEFAULT_REGRESSION_THRESHOLD%)
    -m, --memory-threshold PERCENT Set memory threshold (default: $DEFAULT_MEMORY_THRESHOLD%)
    -r, --runs COUNT        Number of benchmark runs (default: $BENCHMARK_RUNS)
    -b, --baseline FILE     Specific baseline file to compare against
    -o, --output FILE       Output report file
    --fail-on-regression    Exit with error code if regression detected
    --json                  Output results in JSON format
    --html                  Generate HTML report

Examples:
    $0 baseline                              # Create baseline from current code
    $0 check -t 15 --fail-on-regression     # Check with 15% threshold, fail on regression
    $0 compare baseline1.json baseline2.json # Compare two baselines
    $0 report --html -o performance-report.html # Generate HTML trend report

EOF
}

# Initialize directories
init_dirs() {
    log_info "Initializing performance check directories..."
    mkdir -p "$RESULTS_DIR" "$BASELINES_DIR" "$REPORTS_DIR"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking performance check prerequisites..."
    
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed or not in PATH"
        exit 1
    fi
    
    # Check if binary exists
    if [[ ! -f "$PROJECT_ROOT/bin/lsp-gateway" ]]; then
        log_warning "LSP Gateway binary not found, building..."
        cd "$PROJECT_ROOT"
        make local
    fi
    
    # Install benchstat if needed for statistical analysis
    if ! command -v benchstat &> /dev/null; then
        log_info "Installing benchstat for statistical analysis..."
        go install golang.org/x/perf/cmd/benchstat@latest
    fi
}

# Setup test environment for performance testing
setup_performance_environment() {
    log_info "Setting up performance test environment..."
    
    cd "$PROJECT_ROOT"
    
    # Set optimal environment for consistent benchmarking
    export GOGC=off
    export GOMAXPROCS=1
    
    # Add binary to PATH
    export PATH="$PROJECT_ROOT/bin:$PATH"
    
    # Create performance test workspace
    PERF_WORKSPACE="$PROJECT_ROOT/perf-workspace"
    mkdir -p "$PERF_WORKSPACE"
    
    # Create larger test files for realistic performance testing
    log_info "Creating performance test files..."
    
    # Large Go file with multiple functions
    cat > "$PERF_WORKSPACE/large_project.go" << 'EOF'
package main

import (
    "fmt"
    "time"
    "sync"
    "context"
)

// Large struct for testing
type LargeStruct struct {
    ID          int64
    Name        string
    Description string
    Metadata    map[string]interface{}
    Timestamps  []time.Time
    Active      bool
}

// Service interface
type Service interface {
    ProcessData(ctx context.Context, data *LargeStruct) error
    GetStats() map[string]int64
    Close() error
}

// Implementation
type ServiceImpl struct {
    mu    sync.RWMutex
    data  map[int64]*LargeStruct
    stats map[string]int64
}

func NewService() Service {
    return &ServiceImpl{
        data:  make(map[int64]*LargeStruct),
        stats: make(map[string]int64),
    }
}

func (s *ServiceImpl) ProcessData(ctx context.Context, data *LargeStruct) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    s.data[data.ID] = data
    s.stats["processed"]++
    
    return nil
}

func (s *ServiceImpl) GetStats() map[string]int64 {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    result := make(map[string]int64)
    for k, v := range s.stats {
        result[k] = v
    }
    return result
}

func (s *ServiceImpl) Close() error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    s.data = nil
    s.stats = nil
    return nil
}

func main() {
    service := NewService()
    defer service.Close()
    
    for i := 0; i < 1000; i++ {
        data := &LargeStruct{
            ID:          int64(i),
            Name:        fmt.Sprintf("Item_%d", i),
            Description: fmt.Sprintf("Description for item %d", i),
            Metadata:    map[string]interface{}{"key": "value"},
            Timestamps:  []time.Time{time.Now()},
            Active:      i%2 == 0,
        }
        
        ctx := context.Background()
        service.ProcessData(ctx, data)
    }
    
    stats := service.GetStats()
    fmt.Printf("Processed: %d items\n", stats["processed"])
}
EOF
    
    # Python file with realistic complexity
    cat > "$PERF_WORKSPACE/large_project.py" << 'EOF'
import time
import threading
from typing import Dict, List, Optional, Any
from dataclasses import dataclass
from contextlib import contextmanager

@dataclass
class DataModel:
    id: int
    name: str
    description: str
    metadata: Dict[str, Any]
    timestamps: List[float]
    active: bool

class DataProcessor:
    def __init__(self):
        self._data: Dict[int, DataModel] = {}
        self._stats: Dict[str, int] = {"processed": 0}
        self._lock = threading.RLock()
    
    def process_data(self, data: DataModel) -> None:
        with self._lock:
            self._data[data.id] = data
            self._stats["processed"] += 1
    
    def get_stats(self) -> Dict[str, int]:
        with self._lock:
            return self._stats.copy()
    
    @contextmanager
    def transaction(self):
        self._lock.acquire()
        try:
            yield self
        finally:
            self._lock.release()

def main():
    processor = DataProcessor()
    
    for i in range(1000):
        data = DataModel(
            id=i,
            name=f"Item_{i}",
            description=f"Description for item {i}",
            metadata={"key": "value", "index": i},
            timestamps=[time.time()],
            active=i % 2 == 0
        )
        
        processor.process_data(data)
    
    stats = processor.get_stats()
    print(f"Processed: {stats['processed']} items")

if __name__ == "__main__":
    main()
EOF
    
    log_success "Performance test environment setup completed"
}

# Run performance benchmarks
run_benchmarks() {
    local runs=$1
    local output_file="$2"
    
    log_info "Running performance benchmarks ($runs runs)..."
    
    cd "$PROJECT_ROOT"
    
    # Clear output file
    > "$output_file"
    
    for ((i=1; i<=runs; i++)); do
        log_info "Benchmark run $i/$runs"
        echo "=== Run $i ===" >> "$output_file"
        
        # Run benchmarks with various settings
        go test -bench=. -benchmem -count=1 -timeout=30m ./tests/performance/... >> "$output_file" 2>&1
        echo "" >> "$output_file"
        
        # Brief pause between runs to allow system recovery
        sleep 2
    done
    
    log_success "Benchmarks completed"
}

# Process benchmark results and extract statistics
process_benchmark_results() {
    local input_file="$1"
    local output_file="$2"
    
    log_info "Processing benchmark results..."
    
    python3 << EOF
import re
import json
import sys
from statistics import mean, stdev
from collections import defaultdict

def parse_benchmark_results(filename):
    results =defaultdict(lambda: {'ns_per_op': [], 'bytes_per_op': [], 'allocs_per_op': []})
    
    with open(filename, 'r') as f:
        content = f.read()
    
    # Pattern to match Go benchmark output
    # BenchmarkName-N    iterations   ns/op   MB/s   B/op   allocs/op
    pattern = r'Benchmark(\w+)-\d+\s+(\d+)\s+(\d+(?:\.\d+)?)\s+ns/op(?:\s+[\d.]+\s+MB/s)?\s+(\d+)\s+B/op\s+(\d+)\s+allocs/op'
    
    matches = re.findall(pattern, content)
    
    for match in matches:
        name, iterations, ns_per_op, bytes_per_op, allocs_per_op = match
        
        results[name]['ns_per_op'].append(float(ns_per_op))
        results[name]['bytes_per_op'].append(int(bytes_per_op))
        results[name]['allocs_per_op'].append(int(allocs_per_op))
    
    # Calculate statistics
    processed_results = {}
    for name, data in results.items():
        if data['ns_per_op']:  # Only process if we have data
            processed_results[name] = {
                'ns_per_op': {
                    'mean': mean(data['ns_per_op']),
                    'stdev': stdev(data['ns_per_op']) if len(data['ns_per_op']) > 1 else 0,
                    'min': min(data['ns_per_op']),
                    'max': max(data['ns_per_op']),
                    'samples': len(data['ns_per_op'])
                },
                'bytes_per_op': {
                    'mean': mean(data['bytes_per_op']),
                    'stdev': stdev(data['bytes_per_op']) if len(data['bytes_per_op']) > 1 else 0,
                    'min': min(data['bytes_per_op']),
                    'max': max(data['bytes_per_op']),
                    'samples': len(data['bytes_per_op'])
                },
                'allocs_per_op': {
                    'mean': mean(data['allocs_per_op']),
                    'stdev': stdev(data['allocs_per_op']) if len(data['allocs_per_op']) > 1 else 0,
                    'min': min(data['allocs_per_op']),
                    'max': max(data['allocs_per_op']),
                    'samples': len(data['allocs_per_op'])
                }
            }
    
    return processed_results

# Process the results
results = parse_benchmark_results('$input_file')

# Create output structure
output = {
    'timestamp': '$(date -Iseconds)',
    'git_commit': '$(git rev-parse HEAD 2>/dev/null || echo "unknown")',
    'git_branch': '$(git branch --show-current 2>/dev/null || echo "unknown")',
    'go_version': '$(go version | cut -d" " -f3)',
    'benchmarks': results
}

# Write processed results
with open('$output_file', 'w') as f:
    json.dump(output, f, indent=2)

print(f"Processed {len(results)} benchmarks")
EOF
    
    log_success "Benchmark results processed"
}

# Create performance baseline
create_baseline() {
    log_info "Creating performance baseline..."
    
    setup_performance_environment
    
    local timestamp=$(date +"%Y%m%d_%H%M%S")
    local commit_hash=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    local raw_file="$RESULTS_DIR/baseline_raw_${timestamp}.txt"
    local baseline_file="$BASELINES_DIR/baseline_${timestamp}_${commit_hash}.json"
    
    run_benchmarks "$BENCHMARK_RUNS" "$raw_file"
    process_benchmark_results "$raw_file" "$baseline_file"
    
    # Create symlink to latest baseline
    ln -sf "$(basename "$baseline_file")" "$BASELINES_DIR/latest.json"
    
    log_success "Baseline created: $baseline_file"
    echo "BASELINE_FILE=$baseline_file"
}

# Find latest baseline
find_latest_baseline() {
    if [[ -n "${BASELINE_FILE:-}" ]]; then
        echo "$BASELINE_FILE"
        return
    fi
    
    if [[ -f "$BASELINES_DIR/latest.json" ]]; then
        echo "$BASELINES_DIR/latest.json"
        return
    fi
    
    # Find most recent baseline file
    local latest_baseline
    latest_baseline=$(find "$BASELINES_DIR" -name "baseline_*.json" -type f -exec ls -t {} + | head -n1)
    
    if [[ -n "$latest_baseline" ]]; then
        echo "$latest_baseline"
    else
        log_error "No baseline found. Create one with: $0 baseline"
        exit 1
    fi
}

# Check performance against baseline
check_performance() {
    log_info "Checking performance against baseline..."
    
    setup_performance_environment
    
    local baseline_file
    baseline_file=$(find_latest_baseline)
    
    log_info "Using baseline: $baseline_file"
    
    local timestamp=$(date +"%Y%m%d_%H%M%S")
    local raw_file="$RESULTS_DIR/check_raw_${timestamp}.txt"
    local results_file="$RESULTS_DIR/check_results_${timestamp}.json"
    local comparison_file="$REPORTS_DIR/performance_comparison_${timestamp}.json"
    local report_file="$REPORTS_DIR/performance_report_${timestamp}.md"
    
    run_benchmarks "$BENCHMARK_RUNS" "$raw_file"
    process_benchmark_results "$raw_file" "$results_file"
    
    # Compare with baseline
    python3 << EOF
import json
import sys

# Load baseline and current results
with open('$baseline_file', 'r') as f:
    baseline = json.load(f)

with open('$results_file', 'r') as f:
    current = json.load(f)

# Comparison thresholds
performance_threshold = $REGRESSION_THRESHOLD / 100.0
memory_threshold = $MEMORY_THRESHOLD / 100.0

# Compare benchmarks
comparison = {
    'baseline_info': {
        'file': '$baseline_file',
        'timestamp': baseline.get('timestamp', 'unknown'),
        'commit': baseline.get('git_commit', 'unknown')
    },
    'current_info': {
        'timestamp': current.get('timestamp', 'unknown'),
        'commit': current.get('git_commit', 'unknown')
    },
    'thresholds': {
        'performance': performance_threshold,
        'memory': memory_threshold
    },
    'comparisons': {},
    'regressions': [],
    'improvements': []
}

baseline_benchmarks = baseline.get('benchmarks', {})
current_benchmarks = current.get('benchmarks', {})

for bench_name in current_benchmarks:
    if bench_name not in baseline_benchmarks:
        comparison['comparisons'][bench_name] = {
            'status': 'new',
            'message': 'New benchmark'
        }
        continue
    
    baseline_perf = baseline_benchmarks[bench_name]
    current_perf = current_benchmarks[bench_name]
    
    # Calculate changes
    time_change = (current_perf['ns_per_op']['mean'] - baseline_perf['ns_per_op']['mean']) / baseline_perf['ns_per_op']['mean']
    memory_change = (current_perf['bytes_per_op']['mean'] - baseline_perf['bytes_per_op']['mean']) / baseline_perf['bytes_per_op']['mean']
    
    # Determine status
    status = 'stable'
    issues = []
    
    if time_change > performance_threshold:
        status = 'regression'
        issues.append(f'Performance regression: {time_change*100:+.1f}%')
        comparison['regressions'].append({
            'benchmark': bench_name,
            'type': 'performance',
            'change': time_change
        })
    elif time_change < -0.1:  # 10% improvement
        if status == 'stable':
            status = 'improvement'
        issues.append(f'Performance improvement: {time_change*100:+.1f}%')
        comparison['improvements'].append({
            'benchmark': bench_name,
            'type': 'performance',
            'change': time_change
        })
    
    if memory_change > memory_threshold:
        if status == 'stable':
            status = 'regression'
        issues.append(f'Memory regression: {memory_change*100:+.1f}%')
        comparison['regressions'].append({
            'benchmark': bench_name,
            'type': 'memory',
            'change': memory_change
        })
    elif memory_change < -0.1:  # 10% improvement
        if status == 'stable':
            status = 'improvement'
        issues.append(f'Memory improvement: {memory_change*100:+.1f}%')
        comparison['improvements'].append({
            'benchmark': bench_name,
            'type': 'memory',
            'change': memory_change
        })
    
    comparison['comparisons'][bench_name] = {
        'status': status,
        'time_change': time_change,
        'memory_change': memory_change,
        'issues': issues,
        'baseline': {
            'ns_per_op': baseline_perf['ns_per_op']['mean'],
            'bytes_per_op': baseline_perf['bytes_per_op']['mean']
        },
        'current': {
            'ns_per_op': current_perf['ns_per_op']['mean'],
            'bytes_per_op': current_perf['bytes_per_op']['mean']
        }
    }

# Save comparison results
with open('$comparison_file', 'w') as f:
    json.dump(comparison, f, indent=2)

# Generate markdown report
report_lines = [
    '# Performance Comparison Report\\n\\n',
    f'**Generated:** {comparison["current_info"]["timestamp"]}\\n',
    f'**Current Commit:** {comparison["current_info"]["commit"]}\\n',
    f'**Baseline Commit:** {comparison["baseline_info"]["commit"]}\\n\\n'
]

if comparison['regressions']:
    report_lines.append(f'## ❌ Regressions Detected ({len(comparison["regressions"])})\\n\\n')
    for reg in comparison['regressions']:
        report_lines.append(f'- **{reg["benchmark"]}** ({reg["type"]}): {reg["change"]*100:+.1f}%\\n')
    report_lines.append('\\n')

if comparison['improvements']:
    report_lines.append(f'## ✅ Improvements Detected ({len(comparison["improvements"])})\\n\\n')
    for imp in comparison['improvements']:
        report_lines.append(f'- **{imp["benchmark"]}** ({imp["type"]}): {imp["change"]*100:+.1f}%\\n')
    report_lines.append('\\n')

report_lines.append('## Detailed Results\\n\\n')
for bench_name, comp in comparison['comparisons'].items():
    status_emoji = {'regression': '❌', 'improvement': '✅', 'stable': '➡️', 'new': '🆕'}
    report_lines.append(f'### {status_emoji.get(comp["status"], "❓")} {bench_name}\\n\\n')
    
    if comp['status'] != 'new':
        report_lines.append(f'- **Performance**: {comp["time_change"]*100:+.1f}% ({comp["current"]["ns_per_op"]:.0f} vs {comp["baseline"]["ns_per_op"]:.0f} ns/op)\\n')
        report_lines.append(f'- **Memory**: {comp["memory_change"]*100:+.1f}% ({comp["current"]["bytes_per_op"]} vs {comp["baseline"]["bytes_per_op"]} B/op)\\n')
    
    if comp.get('issues'):
        for issue in comp['issues']:
            report_lines.append(f'- ⚠️ {issue}\\n')
    
    report_lines.append('\\n')

# Write report
with open('$report_file', 'w') as f:
    f.write(''.join(report_lines))

# Print summary
has_regressions = len(comparison['regressions']) > 0
has_improvements = len(comparison['improvements']) > 0

print(f"Performance check completed:")
print(f"  Regressions: {len(comparison['regressions'])}")
print(f"  Improvements: {len(comparison['improvements'])}")
print(f"  Report: $report_file")

# Exit with appropriate code
if has_regressions and $FAIL_ON_REGRESSION:
    print("Performance regressions detected!")
    sys.exit(1)
else:
    sys.exit(0)
EOF
    
    local exit_code=$?
    
    if [[ "${OUTPUT_FILE:-}" ]]; then
        cp "$report_file" "$OUTPUT_FILE"
        log_info "Report copied to: $OUTPUT_FILE"
    fi
    
    log_success "Performance check completed"
    return $exit_code
}

# Clean old baseline data
clean_baselines() {
    log_info "Cleaning old baseline data..."
    
    # Remove baselines older than retention period
    find "$BASELINES_DIR" -name "baseline_*.json" -type f -mtime +$BASELINE_RETENTION_DAYS -delete
    
    # Clean old result files
    find "$RESULTS_DIR" -name "*_raw_*.txt" -type f -mtime +7 -delete
    find "$RESULTS_DIR" -name "check_results_*.json" -type f -mtime +30 -delete
    
    log_success "Baseline cleanup completed"
}

# Generate performance trend report
generate_trend_report() {
    log_info "Generating performance trend report..."
    
    local output_file="${OUTPUT_FILE:-$REPORTS_DIR/performance_trend_$(date +%Y%m%d).html}"
    
    python3 << EOF
import json
import glob
import os
from datetime import datetime

# Find all baseline files
baseline_files = glob.glob('$BASELINES_DIR/baseline_*.json')
baseline_files.sort()

if len(baseline_files) < 2:
    print("Need at least 2 baselines for trend analysis")
    exit(1)

trend_data = []
for baseline_file in baseline_files:
    try:
        with open(baseline_file, 'r') as f:
            data = json.load(f)
            trend_data.append(data)
    except Exception as e:
        print(f"Error reading {baseline_file}: {e}")

# Generate HTML report
html_content = '''
<!DOCTYPE html>
<html>
<head>
    <title>LSP Gateway Performance Trend Report</title>
    <script src="https://cdn.plot.ly/plotly-latest.min.js"></script>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .benchmark { margin-bottom: 30px; }
        .chart { width: 100%; height: 400px; }
    </style>
</head>
<body>
    <h1>LSP Gateway Performance Trend Report</h1>
    <p>Generated: ''' + datetime.now().isoformat() + '''</p>
    <p>Baselines analyzed: ''' + str(len(trend_data)) + '''</p>
'''

# Get all benchmark names
all_benchmarks = set()
for data in trend_data:
    all_benchmarks.update(data.get('benchmarks', {}).keys())

for benchmark in sorted(all_benchmarks):
    html_content += f'<div class="benchmark"><h2>{benchmark}</h2>'
    
    # Extract data for this benchmark
    timestamps = []
    ns_per_op = []
    bytes_per_op = []
    
    for data in trend_data:
        if benchmark in data.get('benchmarks', {}):
            timestamps.append(data.get('timestamp', ''))
            benchmark_data = data['benchmarks'][benchmark]
            ns_per_op.append(benchmark_data['ns_per_op']['mean'])
            bytes_per_op.append(benchmark_data['bytes_per_op']['mean'])
    
    if len(timestamps) > 1:
        # Generate chart
        chart_id = f"chart_{benchmark.replace(' ', '_')}"
        html_content += f'<div id="{chart_id}" class="chart"></div>'
        html_content += f'''
        <script>
        var trace1 = {{
            x: {timestamps},
            y: {ns_per_op},
            type: 'scatter',
            mode: 'lines+markers',
            name: 'ns/op',
            yaxis: 'y'
        }};
        
        var trace2 = {{
            x: {timestamps},
            y: {bytes_per_op},
            type: 'scatter',
            mode: 'lines+markers',
            name: 'B/op',
            yaxis: 'y2'
        }};
        
        var layout = {{
            title: '{benchmark} Performance Trend',
            xaxis: {{title: 'Time'}},
            yaxis: {{title: 'ns/op', side: 'left'}},
            yaxis2: {{title: 'B/op', side: 'right', overlaying: 'y'}}
        }};
        
        Plotly.newPlot('{chart_id}', [trace1, trace2], layout);
        </script>
        '''
    else:
        html_content += '<p>Insufficient data for trend analysis</p>'
    
    html_content += '</div>'

html_content += '''
</body>
</html>
'''

# Write HTML report
with open('$output_file', 'w') as f:
    f.write(html_content)

print(f"Trend report generated: $output_file")
EOF
    
    log_success "Performance trend report generated: $output_file"
}

# Main execution function
main() {
    case "$COMMAND" in
        "baseline")
            create_baseline
            ;;
        "check")
            check_performance
            ;;
        "clean")
            clean_baselines
            ;;
        "report")
            generate_trend_report
            ;;
        *)
            log_error "Unknown command: $COMMAND"
            show_help
            exit 1
            ;;
    esac
}

# Parse command line arguments
COMMAND=""
REGRESSION_THRESHOLD=$DEFAULT_REGRESSION_THRESHOLD
MEMORY_THRESHOLD=$DEFAULT_MEMORY_THRESHOLD
BENCHMARK_RUNS=$BENCHMARK_RUNS
BASELINE_FILE=""
OUTPUT_FILE=""
FAIL_ON_REGRESSION=false
JSON_OUTPUT=false
HTML_OUTPUT=false
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
        -t|--threshold)
            REGRESSION_THRESHOLD="$2"
            shift 2
            ;;
        -m|--memory-threshold)
            MEMORY_THRESHOLD="$2"
            shift 2
            ;;
        -r|--runs)
            BENCHMARK_RUNS="$2"
            shift 2
            ;;
        -b|--baseline)
            BASELINE_FILE="$2"
            shift 2
            ;;
        -o|--output)
            OUTPUT_FILE="$2"
            shift 2
            ;;
        --fail-on-regression)
            FAIL_ON_REGRESSION=true
            shift
            ;;
        --json)
            JSON_OUTPUT=true
            shift
            ;;
        --html)
            HTML_OUTPUT=true
            shift
            ;;
        baseline|check|clean|report)
            COMMAND="$1"
            shift
            ;;
        *)
            log_error "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Default command
if [[ -z "$COMMAND" ]]; then
    COMMAND="check"
fi

# Initialize and run
init_dirs
check_prerequisites
main "$@"