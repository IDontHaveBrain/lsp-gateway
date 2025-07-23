package reporters

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"lsp-gateway/tests/utils/framework/types"
)

// PerformanceReporter generates comprehensive performance reports
type PerformanceReporter struct {
	outputDir    string
	enableCSV    bool
	enableJSON   bool
	enableHTML   bool
	colorEnabled bool
	verbose      bool
}

// PerformanceReportConfig configures performance reporting
type PerformanceReportConfig struct {
	OutputDir    string `yaml:"output_dir"`
	EnableCSV    bool   `yaml:"enable_csv"`
	EnableJSON   bool   `yaml:"enable_json"`
	EnableHTML   bool   `yaml:"enable_html"`
	ColorEnabled bool   `yaml:"color_enabled"`
	Verbose      bool   `yaml:"verbose"`
}

// BenchmarkSummary contains aggregated benchmark results
type BenchmarkSummary struct {
	TotalMethods    int                               `json:"total_methods"`
	PassedMethods   int                               `json:"passed_methods"`
	FailedMethods   int                               `json:"failed_methods"`
	OverallPassRate float64                           `json:"overall_pass_rate"`
	Results         map[string]*types.BenchmarkResult `json:"results"`
	Aggregated      AggregatedMetrics                 `json:"aggregated"`
	Regressions     []RegressionAlert                 `json:"regressions"`
	Timestamp       time.Time                         `json:"timestamp"`
	TestDuration    time.Duration                     `json:"test_duration"`
}

// AggregatedMetrics contains metrics aggregated across all methods
type AggregatedMetrics struct {
	TotalRequests     int64                `json:"total_requests"`
	TotalErrors       int64                `json:"total_errors"`
	OverallErrorRate  float64              `json:"overall_error_rate"`
	AverageThroughput float64              `json:"average_throughput"`
	AverageLatency    types.LatencyMetrics `json:"average_latency"`
	TotalMemoryUsage  int64                `json:"total_memory_usage"`
	PeakMemoryUsage   int64                `json:"peak_memory_usage"`
}

// RegressionAlert represents a detected performance regression
type RegressionAlert struct {
	Method      string    `json:"method"`
	Metric      string    `json:"metric"`
	Previous    float64   `json:"previous"`
	Current     float64   `json:"current"`
	Change      float64   `json:"change"`
	Severity    string    `json:"severity"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
}

// NewPerformanceReporter creates a new performance reporter
func NewPerformanceReporter(config *PerformanceReportConfig) *PerformanceReporter {
	if config == nil {
		config = &PerformanceReportConfig{
			OutputDir:    "benchmark-results",
			EnableCSV:    true,
			EnableJSON:   true,
			EnableHTML:   true,
			ColorEnabled: true,
			Verbose:      false,
		}
	}

	return &PerformanceReporter{
		outputDir:    config.OutputDir,
		enableCSV:    config.EnableCSV,
		enableJSON:   config.EnableJSON,
		enableHTML:   config.EnableHTML,
		colorEnabled: config.ColorEnabled,
		verbose:      config.Verbose,
	}
}

// GenerateReport creates a comprehensive performance report
func (pr *PerformanceReporter) GenerateReport(results map[string]*types.BenchmarkResult) (*BenchmarkSummary, error) {
	if err := os.MkdirAll(pr.outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create benchmark summary
	summary := pr.createBenchmarkSummary(results)

	// Generate different report formats
	if pr.enableJSON {
		if err := pr.generateJSONReport(summary); err != nil {
			return nil, fmt.Errorf("failed to generate JSON report: %w", err)
		}
	}

	if pr.enableCSV {
		if err := pr.generateCSVReport(summary); err != nil {
			return nil, fmt.Errorf("failed to generate CSV report: %w", err)
		}
	}

	if pr.enableHTML {
		if err := pr.generateHTMLReport(summary); err != nil {
			return nil, fmt.Errorf("failed to generate HTML report: %w", err)
		}
	}

	// Generate console report
	pr.generateConsoleReport(summary)

	return summary, nil
}

// createBenchmarkSummary aggregates benchmark results into a summary
func (pr *PerformanceReporter) createBenchmarkSummary(results map[string]*types.BenchmarkResult) *BenchmarkSummary {
	summary := &BenchmarkSummary{
		Results:   results,
		Timestamp: time.Now(),
	}

	var totalMethods, passedMethods int
	var totalRequests, totalErrors int64
	var totalThroughput float64
	var totalLatency, minLatency, maxLatency time.Duration
	var totalMemoryUsage, peakMemoryUsage int64

	minLatency = time.Hour // Initialize to high value

	for method, result := range results {
		totalMethods++
		totalRequests += result.TotalRequests
		totalErrors += result.ErrorCount
		totalThroughput += result.ThroughputRPS

		// Aggregate latency metrics
		totalLatency += result.LatencyMetrics.Average
		if result.LatencyMetrics.Min < minLatency {
			minLatency = result.LatencyMetrics.Min
		}
		if result.LatencyMetrics.Max > maxLatency {
			maxLatency = result.LatencyMetrics.Max
		}

		// Aggregate memory metrics
		totalMemoryUsage += result.MemoryMetrics.TotalAlloc
		if result.MemoryMetrics.PeakMemoryUsage > peakMemoryUsage {
			peakMemoryUsage = result.MemoryMetrics.PeakMemoryUsage
		}

		// Check if method passed all thresholds
		if result.ThresholdResults.OverallPassed {
			passedMethods++
		}

		// Detect regressions
		regressions := pr.detectRegressions(method, result)
		summary.Regressions = append(summary.Regressions, regressions...)
	}

	summary.TotalMethods = totalMethods
	summary.PassedMethods = passedMethods
	summary.FailedMethods = totalMethods - passedMethods
	summary.OverallPassRate = float64(passedMethods) / float64(totalMethods) * 100

	// Calculate aggregated metrics
	summary.Aggregated = AggregatedMetrics{
		TotalRequests:     totalRequests,
		TotalErrors:       totalErrors,
		OverallErrorRate:  float64(totalErrors) / float64(totalRequests) * 100,
		AverageThroughput: totalThroughput / float64(totalMethods),
		AverageLatency: types.LatencyMetrics{
			Average: totalLatency / time.Duration(totalMethods),
			Min:     minLatency,
			Max:     maxLatency,
		},
		TotalMemoryUsage: totalMemoryUsage,
		PeakMemoryUsage:  peakMemoryUsage,
	}

	return summary
}

// generateJSONReport creates a detailed JSON report
func (pr *PerformanceReporter) generateJSONReport(summary *BenchmarkSummary) error {
	filename := filepath.Join(pr.outputDir, fmt.Sprintf("lsp_benchmark_report_%s.json",
		summary.Timestamp.Format("20060102_150405")))

	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return os.WriteFile(filename, data, 0644)
}

// generateCSVReport creates a CSV report for easy data analysis
func (pr *PerformanceReporter) generateCSVReport(summary *BenchmarkSummary) error {
	filename := filepath.Join(pr.outputDir, fmt.Sprintf("lsp_benchmark_results_%s.csv",
		summary.Timestamp.Format("20060102_150405")))

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write CSV header
	header := []string{
		"Method", "TotalRequests", "Duration", "ThroughputRPS", "ErrorCount", "ErrorRate",
		"AvgLatency", "MinLatency", "MaxLatency", "P50Latency", "P95Latency", "P99Latency",
		"AllocPerRequest", "TotalAlloc", "PeakMemory", "MemoryGrowth", "GCCount",
		"LatencyPassed", "ThroughputPassed", "MemoryPassed", "OverallPassed",
	}

	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write benchmark results
	for method, result := range summary.Results {
		row := []string{
			method,
			fmt.Sprintf("%d", result.TotalRequests),
			result.Duration.String(),
			fmt.Sprintf("%.2f", result.ThroughputRPS),
			fmt.Sprintf("%d", result.ErrorCount),
			fmt.Sprintf("%.2f", result.ErrorRate*100),
			result.LatencyMetrics.Average.String(),
			result.LatencyMetrics.Min.String(),
			result.LatencyMetrics.Max.String(),
			result.LatencyMetrics.P50.String(),
			result.LatencyMetrics.P95.String(),
			result.LatencyMetrics.P99.String(),
			fmt.Sprintf("%d", result.MemoryMetrics.AllocPerRequest),
			fmt.Sprintf("%d", result.MemoryMetrics.TotalAlloc),
			fmt.Sprintf("%d", result.MemoryMetrics.PeakMemoryUsage),
			fmt.Sprintf("%.1f", result.MemoryMetrics.MemoryGrowthPercent),
			fmt.Sprintf("%d", result.MemoryMetrics.GCCount),
			fmt.Sprintf("%t", result.ThresholdResults.LatencyPassed),
			fmt.Sprintf("%t", result.ThresholdResults.ThroughputPassed),
			fmt.Sprintf("%t", result.ThresholdResults.MemoryPassed),
			fmt.Sprintf("%t", result.ThresholdResults.OverallPassed),
		}

		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return nil
}

// generateHTMLReport creates an HTML report with charts and visualizations
func (pr *PerformanceReporter) generateHTMLReport(summary *BenchmarkSummary) error {
	filename := filepath.Join(pr.outputDir, fmt.Sprintf("lsp_benchmark_report_%s.html",
		summary.Timestamp.Format("20060102_150405")))

	html := pr.generateHTMLContent(summary)

	return os.WriteFile(filename, []byte(html), 0644)
}

// generateHTMLContent creates the HTML content for the report
func (pr *PerformanceReporter) generateHTMLContent(summary *BenchmarkSummary) string {
	var sb strings.Builder

	// HTML header
	sb.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>LSP Performance Benchmark Report</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 20px; }
        .header { background: #f8f9fa; padding: 20px; border-radius: 8px; margin-bottom: 20px; }
        .summary { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 15px; margin-bottom: 30px; }
        .metric { background: white; padding: 15px; border: 1px solid #dee2e6; border-radius: 6px; text-align: center; }
        .metric-value { font-size: 24px; font-weight: bold; color: #007bff; }
        .metric-label { font-size: 12px; color: #6c757d; text-transform: uppercase; margin-top: 5px; }
        .chart-container { margin: 20px 0; }
        .table { width: 100%; border-collapse: collapse; margin: 20px 0; }
        .table th, .table td { padding: 8px 12px; text-align: left; border-bottom: 1px solid #dee2e6; }
        .table th { background: #f8f9fa; font-weight: 600; }
        .status-pass { color: #28a745; font-weight: bold; }
        .status-fail { color: #dc3545; font-weight: bold; }
        .regression { background: #f8d7da; border: 1px solid #f5c6cb; border-radius: 4px; padding: 10px; margin: 5px 0; }
        .regression-severe { background: #d1ecf1; border-color: #bee5eb; }
    </style>
</head>
<body>`)

	// Report header
	sb.WriteString(fmt.Sprintf(`
    <div class="header">
        <h1>LSP Performance Benchmark Report</h1>
        <p><strong>Generated:</strong> %s</p>
        <p><strong>Test Duration:</strong> %v</p>
        <p><strong>Overall Pass Rate:</strong> %.1f%% (%d/%d methods passed)</p>
    </div>`,
		summary.Timestamp.Format("2006-01-02 15:04:05"),
		summary.TestDuration,
		summary.OverallPassRate,
		summary.PassedMethods,
		summary.TotalMethods))

	// Summary metrics
	sb.WriteString(`<div class="summary">`)

	summaryMetrics := []struct {
		label string
		value string
	}{
		{"Total Requests", fmt.Sprintf("%d", summary.Aggregated.TotalRequests)},
		{"Total Errors", fmt.Sprintf("%d", summary.Aggregated.TotalErrors)},
		{"Error Rate", fmt.Sprintf("%.2f%%", summary.Aggregated.OverallErrorRate)},
		{"Avg Throughput", fmt.Sprintf("%.1f req/sec", summary.Aggregated.AverageThroughput)},
		{"Avg Latency", summary.Aggregated.AverageLatency.Average.String()},
		{"Peak Memory", fmt.Sprintf("%.1f MB", float64(summary.Aggregated.PeakMemoryUsage)/1024/1024)},
	}

	for _, metric := range summaryMetrics {
		sb.WriteString(fmt.Sprintf(`
        <div class="metric">
            <div class="metric-value">%s</div>
            <div class="metric-label">%s</div>
        </div>`, metric.value, metric.label))
	}

	sb.WriteString(`</div>`)

	// Performance charts
	pr.addChartSection(&sb, summary)

	// Detailed results table
	pr.addResultsTable(&sb, summary)

	// Regression alerts
	if len(summary.Regressions) > 0 {
		pr.addRegressionSection(&sb, summary)
	}

	// HTML footer
	sb.WriteString(`
</body>
</html>`)

	return sb.String()
}

// addChartSection adds performance charts to HTML report
func (pr *PerformanceReporter) addChartSection(sb *strings.Builder, summary *BenchmarkSummary) {
	sb.WriteString(`
    <h2>Performance Charts</h2>
    <div class="chart-container">
        <h3>Throughput by Method</h3>
        <canvas id="throughputChart" width="400" height="200"></canvas>
    </div>
    
    <div class="chart-container">
        <h3>Latency Comparison</h3>
        <canvas id="latencyChart" width="400" height="200"></canvas>
    </div>
    
    <div class="chart-container">
        <h3>Memory Usage</h3>
        <canvas id="memoryChart" width="400" height="200"></canvas>
    </div>`)

	// Add JavaScript for charts
	sb.WriteString(`<script>`)

	// Prepare data for charts
	var methods []string
	var throughputs, p95Latencies, memoryUsages []float64

	// Sort methods for consistent ordering
	methodNames := make([]string, 0, len(summary.Results))
	for method := range summary.Results {
		methodNames = append(methodNames, method)
	}
	sort.Strings(methodNames)

	for _, method := range methodNames {
		result := summary.Results[method]
		methods = append(methods, method)
		throughputs = append(throughputs, result.ThroughputRPS)
		p95Latencies = append(p95Latencies, float64(result.LatencyMetrics.P95.Nanoseconds())/1000000) // Convert to ms
		memoryUsages = append(memoryUsages, float64(result.MemoryMetrics.AllocPerRequest)/1024)       // Convert to KB
	}

	// Generate chart data
	sb.WriteString(fmt.Sprintf(`
    const methods = %s;
    const throughputs = %s;
    const p95Latencies = %s;
    const memoryUsages = %s;`,
		toJSONArray(methods),
		toJSONFloatArray(throughputs),
		toJSONFloatArray(p95Latencies),
		toJSONFloatArray(memoryUsages)))

	// Throughput chart
	sb.WriteString(`
    new Chart(document.getElementById('throughputChart'), {
        type: 'bar',
        data: {
            labels: methods,
            datasets: [{
                label: 'Throughput (req/sec)',
                data: throughputs,
                backgroundColor: 'rgba(54, 162, 235, 0.5)',
                borderColor: 'rgba(54, 162, 235, 1)',
                borderWidth: 1
            }]
        },
        options: {
            responsive: true,
            scales: {
                y: { beginAtZero: true }
            }
        }
    });`)

	// Latency chart
	sb.WriteString(`
    new Chart(document.getElementById('latencyChart'), {
        type: 'bar',
        data: {
            labels: methods,
            datasets: [{
                label: 'P95 Latency (ms)',
                data: p95Latencies,
                backgroundColor: 'rgba(255, 99, 132, 0.5)',
                borderColor: 'rgba(255, 99, 132, 1)',
                borderWidth: 1
            }]
        },
        options: {
            responsive: true,
            scales: {
                y: { beginAtZero: true }
            }
        }
    });`)

	// Memory chart
	sb.WriteString(`
    new Chart(document.getElementById('memoryChart'), {
        type: 'bar',
        data: {
            labels: methods,
            datasets: [{
                label: 'Memory per Request (KB)',
                data: memoryUsages,
                backgroundColor: 'rgba(75, 192, 192, 0.5)',
                borderColor: 'rgba(75, 192, 192, 1)',
                borderWidth: 1
            }]
        },
        options: {
            responsive: true,
            scales: {
                y: { beginAtZero: true }
            }
        }
    });`)

	sb.WriteString(`</script>`)
}

// addResultsTable adds detailed results table to HTML report
func (pr *PerformanceReporter) addResultsTable(sb *strings.Builder, summary *BenchmarkSummary) {
	sb.WriteString(`
    <h2>Detailed Results</h2>
    <table class="table">
        <thead>
            <tr>
                <th>Method</th>
                <th>Throughput</th>
                <th>P95 Latency</th>
                <th>Error Rate</th>
                <th>Memory/Req</th>
                <th>Status</th>
            </tr>
        </thead>
        <tbody>`)

	// Sort methods for consistent ordering
	methodNames := make([]string, 0, len(summary.Results))
	for method := range summary.Results {
		methodNames = append(methodNames, method)
	}
	sort.Strings(methodNames)

	for _, method := range methodNames {
		result := summary.Results[method]
		statusClass := "status-pass"
		statusText := "PASS"
		if !result.ThresholdResults.OverallPassed {
			statusClass = "status-fail"
			statusText = "FAIL"
		}

		sb.WriteString(fmt.Sprintf(`
            <tr>
                <td>%s</td>
                <td>%.1f req/sec</td>
                <td>%v</td>
                <td>%.2f%%</td>
                <td>%.1f KB</td>
                <td class="%s">%s</td>
            </tr>`,
			method,
			result.ThroughputRPS,
			result.LatencyMetrics.P95,
			result.ErrorRate*100,
			float64(result.MemoryMetrics.AllocPerRequest)/1024,
			statusClass,
			statusText))
	}

	sb.WriteString(`</tbody></table>`)
}

// addRegressionSection adds regression alerts to HTML report
func (pr *PerformanceReporter) addRegressionSection(sb *strings.Builder, summary *BenchmarkSummary) {
	sb.WriteString(`<h2>Performance Regressions</h2>`)

	for _, regression := range summary.Regressions {
		cssClass := "regression"
		if regression.Severity == "severe" {
			cssClass = "regression-severe"
		}

		sb.WriteString(fmt.Sprintf(`
        <div class="%s">
            <strong>%s - %s:</strong> %s<br>
            Previous: %.2f, Current: %.2f, Change: %+.1f%%
        </div>`,
			cssClass,
			regression.Method,
			regression.Metric,
			regression.Description,
			regression.Previous,
			regression.Current,
			regression.Change))
	}
}

// generateConsoleReport prints a summary to the console
func (pr *PerformanceReporter) generateConsoleReport(summary *BenchmarkSummary) {
	fmt.Printf("\n=== LSP Performance Benchmark Results ===\n")
	fmt.Printf("Generated: %s\n", summary.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("Overall Pass Rate: %.1f%% (%d/%d methods passed)\n\n",
		summary.OverallPassRate, summary.PassedMethods, summary.TotalMethods)

	// Summary metrics
	fmt.Printf("SUMMARY:\n")
	fmt.Printf("  Total Requests: %d\n", summary.Aggregated.TotalRequests)
	fmt.Printf("  Error Rate: %.2f%%\n", summary.Aggregated.OverallErrorRate)
	fmt.Printf("  Average Throughput: %.1f req/sec\n", summary.Aggregated.AverageThroughput)
	fmt.Printf("  Average Latency: %v\n", summary.Aggregated.AverageLatency.Average)
	fmt.Printf("  Peak Memory: %.1f MB\n\n", float64(summary.Aggregated.PeakMemoryUsage)/1024/1024)

	// Per-method results
	fmt.Printf("METHOD RESULTS:\n")
	methodNames := make([]string, 0, len(summary.Results))
	for method := range summary.Results {
		methodNames = append(methodNames, method)
	}
	sort.Strings(methodNames)

	for _, method := range methodNames {
		result := summary.Results[method]
		status := "PASS"
		if !result.ThresholdResults.OverallPassed {
			status = "FAIL"
		}

		fmt.Printf("  %-20s: %s - %.1f req/sec, P95: %v, Errors: %.1f%%\n",
			method, status, result.ThroughputRPS, result.LatencyMetrics.P95, result.ErrorRate*100)
	}

	// Regressions
	if len(summary.Regressions) > 0 {
		fmt.Printf("\nREGRESSIONS DETECTED:\n")
		for _, regression := range summary.Regressions {
			fmt.Printf("  %s - %s: %s (%.1f%% change)\n",
				regression.Method, regression.Metric, regression.Description, regression.Change)
		}
	}

	fmt.Printf("\n")
}

// detectRegressions detects performance regressions for a method
func (pr *PerformanceReporter) detectRegressions(method string, result *types.BenchmarkResult) []RegressionAlert {
	var alerts []RegressionAlert

	// This is a simplified implementation
	// In a real implementation, you would load historical data and compare

	// For now, return empty slice - regressions would be detected by comparing
	// with stored historical performance data

	return alerts
}

// Helper functions for HTML generation

func toJSONArray(strings []string) string {
	data, _ := json.Marshal(strings)
	return string(data)
}

func toJSONFloatArray(floats []float64) string {
	data, _ := json.Marshal(floats)
	return string(data)
}
