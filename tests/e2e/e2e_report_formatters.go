package e2e_test

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ConsoleFormatter formats reports for console output
type ConsoleFormatter struct{}

func (f *ConsoleFormatter) Format(data *ComprehensiveReport) ([]byte, error) {
	var buf bytes.Buffer

	// Header
	buf.WriteString("================================================================================\n")
	buf.WriteString("                        E2E Test Comprehensive Report\n")
	buf.WriteString("================================================================================\n\n")

	// Report metadata
	buf.WriteString(fmt.Sprintf("Report Generated: %s\n", data.GeneratedAt.Format("2006-01-02 15:04:05")))
	buf.WriteString(fmt.Sprintf("Report ID: %s\n", data.ReportID))
	buf.WriteString(fmt.Sprintf("Test Run ID: %s\n", data.TestRunID))
	buf.WriteString("\n")

	// Executive Summary
	if data.ExecutiveSummary != nil {
		buf.WriteString("EXECUTIVE SUMMARY\n")
		buf.WriteString("================================================================================\n")
		buf.WriteString(fmt.Sprintf("Overall Status: %s\n", f.colorizeStatus(string(data.ExecutiveSummary.OverallStatus))))
		buf.WriteString(fmt.Sprintf("Total Tests: %d\n", data.ExecutiveSummary.TotalTests))
		buf.WriteString(fmt.Sprintf("Pass Rate: %.2f%%\n", data.ExecutiveSummary.PassRate))
		buf.WriteString(fmt.Sprintf("Duration: %v\n", data.ExecutiveSummary.Duration))
		buf.WriteString(fmt.Sprintf("Performance Score: %.1f/100\n", data.ExecutiveSummary.PerformanceScore))
		buf.WriteString(fmt.Sprintf("Quality Score: %.1f/100\n", data.ExecutiveSummary.QualityScore))
		buf.WriteString(fmt.Sprintf("Coverage Score: %.1f%%\n", data.ExecutiveSummary.CoverageScore))
		buf.WriteString("\n")

		if len(data.ExecutiveSummary.KeyFindings) > 0 {
			buf.WriteString("Key Findings:\n")
			for _, finding := range data.ExecutiveSummary.KeyFindings {
				buf.WriteString(fmt.Sprintf("  • %s\n", finding))
			}
			buf.WriteString("\n")
		}

		if len(data.ExecutiveSummary.CriticalIssues) > 0 {
			buf.WriteString("Critical Issues:\n")
			for _, issue := range data.ExecutiveSummary.CriticalIssues {
				buf.WriteString(fmt.Sprintf("  ⚠️  %s\n", issue))
			}
			buf.WriteString("\n")
		}

		if len(data.ExecutiveSummary.RecommendedActions) > 0 {
			buf.WriteString("Recommended Actions:\n")
			for _, action := range data.ExecutiveSummary.RecommendedActions {
				buf.WriteString(fmt.Sprintf("  → %s\n", action))
			}
			buf.WriteString("\n")
		}
	}

	// Test Metrics
	if data.TestMetrics != nil {
		buf.WriteString("TEST EXECUTION METRICS\n")
		buf.WriteString("================================================================================\n")
		buf.WriteString(fmt.Sprintf("Total Tests: %d\n", data.TestMetrics.TotalTests))
		buf.WriteString(fmt.Sprintf("Passed: %d\n", data.TestMetrics.PassedTests))
		buf.WriteString(fmt.Sprintf("Failed: %d\n", data.TestMetrics.FailedTests))
		buf.WriteString(fmt.Sprintf("Skipped: %d\n", data.TestMetrics.SkippedTests))
		buf.WriteString(fmt.Sprintf("Success Rate: %.2f%%\n", data.TestMetrics.SuccessRate))
		buf.WriteString(fmt.Sprintf("Total Duration: %v\n", data.TestMetrics.TotalDuration))
		buf.WriteString(fmt.Sprintf("Average Duration: %v\n", data.TestMetrics.AverageDuration))
		buf.WriteString(fmt.Sprintf("Fastest Test: %v\n", data.TestMetrics.FastestTest))
		buf.WriteString(fmt.Sprintf("Slowest Test: %v\n", data.TestMetrics.SlowestTest))
		buf.WriteString(fmt.Sprintf("Tests Per Second: %.2f\n", data.TestMetrics.TestsPerSecond))
		buf.WriteString(fmt.Sprintf("Timeouts: %d\n", data.TestMetrics.TimeoutCount))
		buf.WriteString(fmt.Sprintf("Retries: %d\n", data.TestMetrics.RetryCount))
		buf.WriteString("\n")

		if len(data.TestMetrics.TestsByScenario) > 0 {
			buf.WriteString("Tests by Scenario:\n")
			for scenario, count := range data.TestMetrics.TestsByScenario {
				buf.WriteString(fmt.Sprintf("  %s: %d\n", scenario, count))
			}
			buf.WriteString("\n")
		}

		if len(data.TestMetrics.TestsByLanguage) > 0 {
			buf.WriteString("Tests by Language:\n")
			for language, count := range data.TestMetrics.TestsByLanguage {
				buf.WriteString(fmt.Sprintf("  %s: %d\n", language, count))
			}
			buf.WriteString("\n")
		}
	}

	// System Metrics
	if data.SystemMetrics != nil {
		buf.WriteString("SYSTEM RESOURCE METRICS\n")
		buf.WriteString("================================================================================\n")
		buf.WriteString(fmt.Sprintf("Test Duration: %v\n", data.SystemMetrics.EndTime.Sub(data.SystemMetrics.StartTime)))
		buf.WriteString(fmt.Sprintf("Initial Memory: %.1f MB\n", data.SystemMetrics.InitialMemoryMB))
		buf.WriteString(fmt.Sprintf("Peak Memory: %.1f MB\n", data.SystemMetrics.PeakMemoryMB))
		buf.WriteString(fmt.Sprintf("Final Memory: %.1f MB\n", data.SystemMetrics.FinalMemoryMB))
		buf.WriteString(fmt.Sprintf("Memory Growth: %.1f MB\n", data.SystemMetrics.MemoryGrowthMB))
		buf.WriteString(fmt.Sprintf("Initial Goroutines: %d\n", data.SystemMetrics.InitialGoroutines))
		buf.WriteString(fmt.Sprintf("Peak Goroutines: %d\n", data.SystemMetrics.PeakGoroutines))
		buf.WriteString(fmt.Sprintf("Final Goroutines: %d\n", data.SystemMetrics.FinalGoroutines))
		buf.WriteString(fmt.Sprintf("Goroutine Growth: %d\n", data.SystemMetrics.GoroutineGrowth))
		buf.WriteString(fmt.Sprintf("GC Count: %d\n", data.SystemMetrics.GCCount))
		buf.WriteString(fmt.Sprintf("GC Total Pause: %v\n", time.Duration(data.SystemMetrics.GCTotalPauseNS)))
		buf.WriteString(fmt.Sprintf("GC Average Pause: %v\n", time.Duration(data.SystemMetrics.GCAveragePauseNS)))
		buf.WriteString(fmt.Sprintf("Memory Efficiency: %.2f\n", data.SystemMetrics.MemoryEfficiency))
		buf.WriteString(fmt.Sprintf("GC Efficiency: %.2f\n", data.SystemMetrics.GCEfficiency))
		buf.WriteString(fmt.Sprintf("System: %s %s\n", data.SystemMetrics.GOOS, data.SystemMetrics.GOARCH))
		buf.WriteString(fmt.Sprintf("CPUs: %d\n", data.SystemMetrics.NumCPU))
		buf.WriteString(fmt.Sprintf("Go Version: %s\n", data.SystemMetrics.GoVersion))
		buf.WriteString("\n")
	}

	// MCP Client Metrics
	if data.MCPMetrics != nil {
		buf.WriteString("MCP CLIENT METRICS\n")
		buf.WriteString("================================================================================\n")
		buf.WriteString(fmt.Sprintf("Total Requests: %d\n", data.MCPMetrics.TotalRequests))
		buf.WriteString(fmt.Sprintf("Successful Requests: %d\n", data.MCPMetrics.SuccessfulRequests))
		buf.WriteString(fmt.Sprintf("Failed Requests: %d\n", data.MCPMetrics.FailedRequests))
		buf.WriteString(fmt.Sprintf("Timeout Requests: %d\n", data.MCPMetrics.TimeoutRequests))
		buf.WriteString(fmt.Sprintf("Connection Errors: %d\n", data.MCPMetrics.ConnectionErrors))
		buf.WriteString(fmt.Sprintf("Circuit Breaker Triggers: %d\n", data.MCPMetrics.CircuitBreakerTriggers))
		buf.WriteString(fmt.Sprintf("Retry Attempts: %d\n", data.MCPMetrics.RetryAttempts))
		buf.WriteString(fmt.Sprintf("Average Latency: %v\n", data.MCPMetrics.AverageLatency))
		buf.WriteString(fmt.Sprintf("Min Latency: %v\n", data.MCPMetrics.MinLatency))
		buf.WriteString(fmt.Sprintf("Max Latency: %v\n", data.MCPMetrics.MaxLatency))
		buf.WriteString(fmt.Sprintf("P95 Latency: %v\n", data.MCPMetrics.P95Latency))
		buf.WriteString(fmt.Sprintf("P99 Latency: %v\n", data.MCPMetrics.P99Latency))
		buf.WriteString(fmt.Sprintf("Throughput: %.2f req/sec\n", data.MCPMetrics.ThroughputPerSecond))
		buf.WriteString(fmt.Sprintf("Error Rate: %.2f%%\n", data.MCPMetrics.ErrorRatePercent))
		buf.WriteString("\n")

		if len(data.MCPMetrics.RequestsByMethod) > 0 {
			buf.WriteString("Requests by Method:\n")
			// Sort methods by request count
			type methodCount struct {
				method string
				count  int64
			}
			var methods []methodCount
			for method, count := range data.MCPMetrics.RequestsByMethod {
				methods = append(methods, methodCount{method, count})
			}
			sort.Slice(methods, func(i, j int) bool {
				return methods[i].count > methods[j].count
			})
			for _, mc := range methods {
				buf.WriteString(fmt.Sprintf("  %s: %d\n", mc.method, mc.count))
			}
			buf.WriteString("\n")
		}
	}

	// Error Analysis
	if data.ErrorAnalysis != nil && data.ErrorAnalysis.TotalErrors > 0 {
		buf.WriteString("ERROR ANALYSIS\n")
		buf.WriteString("================================================================================\n")
		buf.WriteString(fmt.Sprintf("Total Errors: %d\n", data.ErrorAnalysis.TotalErrors))
		buf.WriteString("\n")

		if len(data.ErrorAnalysis.ErrorsByType) > 0 {
			buf.WriteString("Errors by Type:\n")
			for errorType, count := range data.ErrorAnalysis.ErrorsByType {
				buf.WriteString(fmt.Sprintf("  %s: %d\n", errorType, count))
			}
			buf.WriteString("\n")
		}

		if len(data.ErrorAnalysis.ErrorsByCategory) > 0 {
			buf.WriteString("Errors by Category:\n")
			for category, count := range data.ErrorAnalysis.ErrorsByCategory {
				buf.WriteString(fmt.Sprintf("  %s: %d\n", category, count))
			}
			buf.WriteString("\n")
		}

		if len(data.ErrorAnalysis.ErrorsBySeverity) > 0 {
			buf.WriteString("Errors by Severity:\n")
			for severity, count := range data.ErrorAnalysis.ErrorsBySeverity {
				buf.WriteString(fmt.Sprintf("  %s: %d\n", severity, count))
			}
			buf.WriteString("\n")
		}

		if len(data.ErrorAnalysis.FrequentErrors) > 0 {
			buf.WriteString("Most Frequent Errors:\n")
			for i, pattern := range data.ErrorAnalysis.FrequentErrors {
				if i >= 5 { // Show top 5
					break
				}
				buf.WriteString(fmt.Sprintf("  %d. %s (count: %d, severity: %s)\n",
					i+1, pattern.Pattern, pattern.Count, pattern.Severity))
			}
			buf.WriteString("\n")
		}
	}

	// Coverage Analysis
	if data.CoverageAnalysis != nil && data.CoverageAnalysis.OverallCoverage != nil {
		buf.WriteString("COVERAGE ANALYSIS\n")
		buf.WriteString("================================================================================\n")
		buf.WriteString(fmt.Sprintf("Total Coverage: %.2f%%\n", data.CoverageAnalysis.OverallCoverage.TotalCoveragePercent))
		buf.WriteString(fmt.Sprintf("Scenario Coverage: %.2f%%\n", data.CoverageAnalysis.OverallCoverage.ScenarioCoveragePercent))
		buf.WriteString(fmt.Sprintf("Language Coverage: %.2f%%\n", data.CoverageAnalysis.OverallCoverage.LanguageCoveragePercent))
		buf.WriteString(fmt.Sprintf("Method Coverage: %.2f%%\n", data.CoverageAnalysis.OverallCoverage.MethodCoveragePercent))
		buf.WriteString(fmt.Sprintf("Feature Coverage: %.2f%%\n", data.CoverageAnalysis.OverallCoverage.FeatureCoveragePercent))
		buf.WriteString(fmt.Sprintf("Quality Score: %.2f\n", data.CoverageAnalysis.OverallCoverage.QualityScore))
		buf.WriteString(fmt.Sprintf("Coverage Trend: %s\n", data.CoverageAnalysis.OverallCoverage.CoverageTrend))
		buf.WriteString("\n")

		if len(data.CoverageAnalysis.CoverageGaps) > 0 {
			buf.WriteString("Coverage Gaps:\n")
			for i, gap := range data.CoverageAnalysis.CoverageGaps {
				if i >= 5 { // Show top 5
					break
				}
				buf.WriteString(fmt.Sprintf("  %d. %s: %s (Priority: %s, Impact: %s)\n",
					i+1, gap.GapType, gap.Description, gap.Priority, gap.Impact))
			}
			buf.WriteString("\n")
		}
	}

	// Performance Summary
	if data.PerformanceData != nil {
		buf.WriteString("PERFORMANCE SUMMARY\n")
		buf.WriteString("================================================================================\n")
		buf.WriteString(fmt.Sprintf("Performance Score: %.1f/100\n", data.PerformanceData.PerformanceScore))

		if data.PerformanceData.RegressionDetection != nil {
			buf.WriteString(fmt.Sprintf("Regression Risk: %s\n", data.PerformanceData.RegressionDetection.RegressionRisk))
			buf.WriteString(fmt.Sprintf("Regressions Detected: %d\n", len(data.PerformanceData.RegressionDetection.RegressionsDetected)))
			buf.WriteString(fmt.Sprintf("Improvements Detected: %d\n", len(data.PerformanceData.RegressionDetection.ImprovementsDetected)))
		}

		if len(data.PerformanceData.ScenarioMetrics) > 0 {
			buf.WriteString("\nTop Scenarios by Performance:\n")
			// Sort scenarios by performance score
			type scenarioPerf struct {
				name  string
				score float64
			}
			var scenarios []scenarioPerf
			for name, metrics := range data.PerformanceData.ScenarioMetrics {
				scenarios = append(scenarios, scenarioPerf{name, metrics.PerformanceScore})
			}
			sort.Slice(scenarios, func(i, j int) bool {
				return scenarios[i].score > scenarios[j].score
			})
			for i, sp := range scenarios {
				if i >= 5 { // Show top 5
					break
				}
				buf.WriteString(fmt.Sprintf("  %d. %s: %.1f/100\n", i+1, sp.name, sp.score))
			}
		}
		buf.WriteString("\n")
	}

	// Recommendations
	if len(data.Recommendations) > 0 {
		buf.WriteString("RECOMMENDATIONS\n")
		buf.WriteString("================================================================================\n")
		for i, rec := range data.Recommendations {
			if i >= 10 { // Show top 10
				break
			}
			buf.WriteString(fmt.Sprintf("%d. %s (Priority: %s)\n", i+1, rec.Title, rec.Priority))
			buf.WriteString(fmt.Sprintf("   %s\n", rec.Description))
			if rec.Implementation != "" {
				buf.WriteString(fmt.Sprintf("   Implementation: %s\n", rec.Implementation))
			}
			buf.WriteString("\n")
		}
	}

	// Footer
	buf.WriteString("================================================================================\n")
	buf.WriteString("End of E2E Test Comprehensive Report\n")
	buf.WriteString("================================================================================\n")

	return buf.Bytes(), nil
}

func (f *ConsoleFormatter) Extension() string {
	return "txt"
}

func (f *ConsoleFormatter) ContentType() string {
	return "text/plain"
}

func (f *ConsoleFormatter) colorizeStatus(status string) string {
	// ANSI color codes for console output
	switch strings.ToLower(status) {
	case "passed":
		return "\033[32m" + status + "\033[0m" // Green
	case "failed":
		return "\033[31m" + status + "\033[0m" // Red
	case "skipped":
		return "\033[33m" + status + "\033[0m" // Yellow
	case "timeout":
		return "\033[35m" + status + "\033[0m" // Magenta
	default:
		return status
	}
}

// JSONFormatter formats reports as JSON
type JSONFormatter struct{}

func (f *JSONFormatter) Format(data *ComprehensiveReport) ([]byte, error) {
	return json.MarshalIndent(data, "", "  ")
}

func (f *JSONFormatter) Extension() string {
	return "json"
}

func (f *JSONFormatter) ContentType() string {
	return "application/json"
}

// HTMLFormatter formats reports as HTML
type HTMLFormatter struct{}

func (f *HTMLFormatter) Format(data *ComprehensiveReport) ([]byte, error) {
	tmpl := template.Must(template.New("report").Funcs(template.FuncMap{
		"formatDuration": func(d time.Duration) string {
			return d.String()
		},
		"formatPercent": func(v float64) string {
			return fmt.Sprintf("%.2f%%", v)
		},
		"formatFloat": func(v float64) string {
			return fmt.Sprintf("%.2f", v)
		},
		"formatTime": func(t time.Time) string {
			return t.Format("2006-01-02 15:04:05")
		},
		"colorizeStatus": func(status string) string {
			switch strings.ToLower(status) {
			case "passed":
				return "success"
			case "failed":
				return "danger"
			case "skipped":
				return "warning"
			case "timeout":
				return "info"
			default:
				return "secondary"
			}
		},
		"progressBarClass": func(percent float64) string {
			if percent >= 95 {
				return "bg-success"
			} else if percent >= 80 {
				return "bg-warning"
			} else {
				return "bg-danger"
			}
		},
	}).Parse(htmlTemplate))

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to execute HTML template: %w", err)
	}

	return buf.Bytes(), nil
}

func (f *HTMLFormatter) Extension() string {
	return "html"
}

func (f *HTMLFormatter) ContentType() string {
	return "text/html"
}

// CSVFormatter formats reports as CSV
type CSVFormatter struct{}

func (f *CSVFormatter) Format(data *ComprehensiveReport) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write test results CSV
	if err := writer.Write([]string{
		"Test ID", "Test Name", "Scenario", "Language", "Status", "Duration (ms)",
		"Success", "Errors", "Warnings", "Request Count", "Success Rate", "Average Latency (ms)",
	}); err != nil {
		return nil, fmt.Errorf("failed to write CSV header: %w", err)
	}

	for _, result := range data.TestResults {
		record := []string{
			result.TestID,
			result.TestName,
			result.Scenario,
			result.Language,
			string(result.Status),
			strconv.FormatInt(result.Duration.Milliseconds(), 10),
			strconv.FormatBool(result.Success),
			strconv.Itoa(len(result.Errors)),
			strconv.Itoa(len(result.Warnings)),
		}

		if result.Metrics != nil {
			record = append(record,
				strconv.FormatInt(result.Metrics.RequestCount, 10),
				fmt.Sprintf("%.2f", (float64(result.Metrics.SuccessfulRequests)/float64(result.Metrics.RequestCount))*100),
				strconv.FormatInt(result.Metrics.AverageLatency.Milliseconds(), 10),
			)
		} else {
			record = append(record, "", "", "")
		}

		if err := writer.Write(record); err != nil {
			return nil, fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("CSV writer error: %w", err)
	}

	return buf.Bytes(), nil
}

func (f *CSVFormatter) Extension() string {
	return "csv"
}

func (f *CSVFormatter) ContentType() string {
	return "text/csv"
}

// HTML template for the HTML formatter
const htmlTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>E2E Test Comprehensive Report</title>
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css" rel="stylesheet">
    <link href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0/css/all.min.css" rel="stylesheet">
    <style>
        .metric-card {
            border-left: 4px solid #007bff;
            margin-bottom: 1rem;
        }
        .status-badge {
            font-size: 0.9rem;
        }
        .progress-sm {
            height: 8px;
        }
        .table-sm th {
            background-color: #f8f9fa;
        }
        .error-badge {
            font-size: 0.8rem;
        }
        .recommendation-card {
            border-left: 4px solid #28a745;
        }
        .summary-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 1rem;
        }
        .chart-container {
            height: 300px;
            margin: 1rem 0;
        }
    </style>
</head>
<body>
    <div class="container-fluid py-4">
        <!-- Header -->
        <div class="row mb-4">
            <div class="col-12">
                <h1 class="display-4 text-center">
                    <i class="fas fa-chart-line me-3"></i>
                    E2E Test Comprehensive Report
                </h1>
                <hr>
                <div class="text-center text-muted">
                    <p>Report Generated: {{formatTime .GeneratedAt}} | Report ID: {{.ReportID}}</p>
                </div>
            </div>
        </div>

        <!-- Executive Summary -->
        {{if .ExecutiveSummary}}
        <div class="row mb-4">
            <div class="col-12">
                <div class="card">
                    <div class="card-header bg-primary text-white">
                        <h3 class="card-title mb-0">
                            <i class="fas fa-clipboard-check me-2"></i>
                            Executive Summary
                        </h3>
                    </div>
                    <div class="card-body">
                        <div class="summary-grid">
                            <div class="metric-card card p-3">
                                <h5>Overall Status</h5>
                                <span class="badge bg-{{colorizeStatus (string .ExecutiveSummary.OverallStatus)}} status-badge">
                                    {{.ExecutiveSummary.OverallStatus}}
                                </span>
                            </div>
                            <div class="metric-card card p-3">
                                <h5>Total Tests</h5>
                                <h3 class="text-primary">{{.ExecutiveSummary.TotalTests}}</h3>
                            </div>
                            <div class="metric-card card p-3">
                                <h5>Pass Rate</h5>
                                <h3 class="text-success">{{formatPercent .ExecutiveSummary.PassRate}}</h3>
                                <div class="progress progress-sm">
                                    <div class="progress-bar {{progressBarClass .ExecutiveSummary.PassRate}}" 
                                         style="width: {{formatPercent .ExecutiveSummary.PassRate}}"></div>
                                </div>
                            </div>
                            <div class="metric-card card p-3">
                                <h5>Duration</h5>
                                <h3 class="text-info">{{formatDuration .ExecutiveSummary.Duration}}</h3>
                            </div>
                            <div class="metric-card card p-3">
                                <h5>Performance Score</h5>
                                <h3 class="text-warning">{{formatFloat .ExecutiveSummary.PerformanceScore}}/100</h3>
                                <div class="progress progress-sm">
                                    <div class="progress-bar bg-warning" 
                                         style="width: {{formatFloat .ExecutiveSummary.PerformanceScore}}%"></div>
                                </div>
                            </div>
                            <div class="metric-card card p-3">
                                <h5>Coverage Score</h5>
                                <h3 class="text-success">{{formatPercent .ExecutiveSummary.CoverageScore}}</h3>
                                <div class="progress progress-sm">
                                    <div class="progress-bar bg-success" 
                                         style="width: {{formatPercent .ExecutiveSummary.CoverageScore}}"></div>
                                </div>
                            </div>
                        </div>

                        {{if .ExecutiveSummary.KeyFindings}}
                        <div class="mt-4">
                            <h5><i class="fas fa-star text-warning me-2"></i>Key Findings</h5>
                            <ul class="list-group">
                                {{range .ExecutiveSummary.KeyFindings}}
                                <li class="list-group-item">
                                    <i class="fas fa-check-circle text-success me-2"></i>{{.}}
                                </li>
                                {{end}}
                            </ul>
                        </div>
                        {{end}}

                        {{if .ExecutiveSummary.CriticalIssues}}
                        <div class="mt-4">
                            <h5><i class="fas fa-exclamation-triangle text-danger me-2"></i>Critical Issues</h5>
                            <ul class="list-group">
                                {{range .ExecutiveSummary.CriticalIssues}}
                                <li class="list-group-item list-group-item-danger">
                                    <i class="fas fa-times-circle text-danger me-2"></i>{{.}}
                                </li>
                                {{end}}
                            </ul>
                        </div>
                        {{end}}

                        {{if .ExecutiveSummary.RecommendedActions}}
                        <div class="mt-4">
                            <h5><i class="fas fa-lightbulb text-info me-2"></i>Recommended Actions</h5>
                            <ul class="list-group">
                                {{range .ExecutiveSummary.RecommendedActions}}
                                <li class="list-group-item list-group-item-info">
                                    <i class="fas fa-arrow-right text-info me-2"></i>{{.}}
                                </li>
                                {{end}}
                            </ul>
                        </div>
                        {{end}}
                    </div>
                </div>
            </div>
        </div>
        {{end}}

        <!-- Test Metrics -->
        {{if .TestMetrics}}
        <div class="row mb-4">
            <div class="col-12">
                <div class="card">
                    <div class="card-header bg-info text-white">
                        <h3 class="card-title mb-0">
                            <i class="fas fa-vial me-2"></i>
                            Test Execution Metrics
                        </h3>
                    </div>
                    <div class="card-body">
                        <div class="row">
                            <div class="col-md-6">
                                <table class="table table-sm">
                                    <tr><th>Total Tests</th><td>{{.TestMetrics.TotalTests}}</td></tr>
                                    <tr><th>Passed</th><td class="text-success">{{.TestMetrics.PassedTests}}</td></tr>
                                    <tr><th>Failed</th><td class="text-danger">{{.TestMetrics.FailedTests}}</td></tr>
                                    <tr><th>Skipped</th><td class="text-warning">{{.TestMetrics.SkippedTests}}</td></tr>
                                    <tr><th>Success Rate</th><td>{{formatPercent .TestMetrics.SuccessRate}}</td></tr>
                                    <tr><th>Total Duration</th><td>{{formatDuration .TestMetrics.TotalDuration}}</td></tr>
                                </table>
                            </div>
                            <div class="col-md-6">
                                <table class="table table-sm">
                                    <tr><th>Average Duration</th><td>{{formatDuration .TestMetrics.AverageDuration}}</td></tr>
                                    <tr><th>Fastest Test</th><td>{{formatDuration .TestMetrics.FastestTest}}</td></tr>
                                    <tr><th>Slowest Test</th><td>{{formatDuration .TestMetrics.SlowestTest}}</td></tr>
                                    <tr><th>Tests Per Second</th><td>{{formatFloat .TestMetrics.TestsPerSecond}}</td></tr>
                                    <tr><th>Timeouts</th><td>{{.TestMetrics.TimeoutCount}}</td></tr>
                                    <tr><th>Retries</th><td>{{.TestMetrics.RetryCount}}</td></tr>
                                </table>
                            </div>
                        </div>

                        {{if .TestMetrics.TestsByScenario}}
                        <div class="mt-4">
                            <h5>Tests by Scenario</h5>
                            <div class="row">
                                {{range $scenario, $count := .TestMetrics.TestsByScenario}}
                                <div class="col-md-4 mb-2">
                                    <div class="card">
                                        <div class="card-body py-2">
                                            <strong>{{$scenario}}</strong>
                                            <span class="badge bg-primary float-end">{{$count}}</span>
                                        </div>
                                    </div>
                                </div>
                                {{end}}
                            </div>
                        </div>
                        {{end}}

                        {{if .TestMetrics.TestsByLanguage}}
                        <div class="mt-4">
                            <h5>Tests by Language</h5>
                            <div class="row">
                                {{range $language, $count := .TestMetrics.TestsByLanguage}}
                                <div class="col-md-3 mb-2">
                                    <div class="card">
                                        <div class="card-body py-2">
                                            <strong>{{$language}}</strong>
                                            <span class="badge bg-secondary float-end">{{$count}}</span>
                                        </div>
                                    </div>
                                </div>
                                {{end}}
                            </div>
                        </div>
                        {{end}}
                    </div>
                </div>
            </div>
        </div>
        {{end}}

        <!-- System Metrics -->
        {{if .SystemMetrics}}
        <div class="row mb-4">
            <div class="col-12">
                <div class="card">
                    <div class="card-header bg-success text-white">
                        <h3 class="card-title mb-0">
                            <i class="fas fa-server me-2"></i>
                            System Resource Metrics
                        </h3>
                    </div>
                    <div class="card-body">
                        <div class="row">
                            <div class="col-md-6">
                                <h5>Memory Usage</h5>
                                <table class="table table-sm">
                                    <tr><th>Initial Memory</th><td>{{formatFloat .SystemMetrics.InitialMemoryMB}} MB</td></tr>
                                    <tr><th>Peak Memory</th><td>{{formatFloat .SystemMetrics.PeakMemoryMB}} MB</td></tr>
                                    <tr><th>Final Memory</th><td>{{formatFloat .SystemMetrics.FinalMemoryMB}} MB</td></tr>
                                    <tr><th>Memory Growth</th><td>{{formatFloat .SystemMetrics.MemoryGrowthMB}} MB</td></tr>
                                    <tr><th>Memory Efficiency</th><td>{{formatFloat .SystemMetrics.MemoryEfficiency}}</td></tr>
                                </table>
                            </div>
                            <div class="col-md-6">
                                <h5>System Information</h5>
                                <table class="table table-sm">
                                    <tr><th>OS</th><td>{{.SystemMetrics.GOOS}} {{.SystemMetrics.GOARCH}}</td></tr>
                                    <tr><th>CPUs</th><td>{{.SystemMetrics.NumCPU}}</td></tr>
                                    <tr><th>Go Version</th><td>{{.SystemMetrics.GoVersion}}</td></tr>
                                    <tr><th>GC Count</th><td>{{.SystemMetrics.GCCount}}</td></tr>
                                    <tr><th>GC Efficiency</th><td>{{formatFloat .SystemMetrics.GCEfficiency}}</td></tr>
                                </table>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
        {{end}}

        <!-- MCP Client Metrics -->
        {{if .MCPMetrics}}
        <div class="row mb-4">
            <div class="col-12">
                <div class="card">
                    <div class="card-header bg-warning text-dark">
                        <h3 class="card-title mb-0">
                            <i class="fas fa-exchange-alt me-2"></i>
                            MCP Client Metrics
                        </h3>
                    </div>
                    <div class="card-body">
                        <div class="row">
                            <div class="col-md-6">
                                <h5>Request Statistics</h5>
                                <table class="table table-sm">
                                    <tr><th>Total Requests</th><td>{{.MCPMetrics.TotalRequests}}</td></tr>
                                    <tr><th>Successful</th><td class="text-success">{{.MCPMetrics.SuccessfulRequests}}</td></tr>
                                    <tr><th>Failed</th><td class="text-danger">{{.MCPMetrics.FailedRequests}}</td></tr>
                                    <tr><th>Timeouts</th><td class="text-warning">{{.MCPMetrics.TimeoutRequests}}</td></tr>
                                    <tr><th>Connection Errors</th><td class="text-danger">{{.MCPMetrics.ConnectionErrors}}</td></tr>
                                    <tr><th>Error Rate</th><td>{{formatPercent .MCPMetrics.ErrorRatePercent}}</td></tr>
                                </table>
                            </div>
                            <div class="col-md-6">
                                <h5>Performance Statistics</h5>
                                <table class="table table-sm">
                                    <tr><th>Average Latency</th><td>{{formatDuration .MCPMetrics.AverageLatency}}</td></tr>
                                    <tr><th>Min Latency</th><td>{{formatDuration .MCPMetrics.MinLatency}}</td></tr>
                                    <tr><th>Max Latency</th><td>{{formatDuration .MCPMetrics.MaxLatency}}</td></tr>
                                    <tr><th>P95 Latency</th><td>{{formatDuration .MCPMetrics.P95Latency}}</td></tr>
                                    <tr><th>P99 Latency</th><td>{{formatDuration .MCPMetrics.P99Latency}}</td></tr>
                                    <tr><th>Throughput</th><td>{{formatFloat .MCPMetrics.ThroughputPerSecond}} req/sec</td></tr>
                                </table>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
        {{end}}

        <!-- Error Analysis -->
        {{if and .ErrorAnalysis (gt .ErrorAnalysis.TotalErrors 0)}}
        <div class="row mb-4">
            <div class="col-12">
                <div class="card">
                    <div class="card-header bg-danger text-white">
                        <h3 class="card-title mb-0">
                            <i class="fas fa-bug me-2"></i>
                            Error Analysis
                        </h3>
                    </div>
                    <div class="card-body">
                        <div class="alert alert-danger">
                            <strong>Total Errors: {{.ErrorAnalysis.TotalErrors}}</strong>
                        </div>

                        <div class="row">
                            {{if .ErrorAnalysis.ErrorsByType}}
                            <div class="col-md-4">
                                <h5>By Type</h5>
                                <ul class="list-group">
                                    {{range $type, $count := .ErrorAnalysis.ErrorsByType}}
                                    <li class="list-group-item d-flex justify-content-between">
                                        {{$type}}
                                        <span class="badge bg-danger">{{$count}}</span>
                                    </li>
                                    {{end}}
                                </ul>
                            </div>
                            {{end}}

                            {{if .ErrorAnalysis.ErrorsByCategory}}
                            <div class="col-md-4">
                                <h5>By Category</h5>
                                <ul class="list-group">
                                    {{range $category, $count := .ErrorAnalysis.ErrorsByCategory}}
                                    <li class="list-group-item d-flex justify-content-between">
                                        {{$category}}
                                        <span class="badge bg-warning">{{$count}}</span>
                                    </li>
                                    {{end}}
                                </ul>
                            </div>
                            {{end}}

                            {{if .ErrorAnalysis.ErrorsBySeverity}}
                            <div class="col-md-4">
                                <h5>By Severity</h5>
                                <ul class="list-group">
                                    {{range $severity, $count := .ErrorAnalysis.ErrorsBySeverity}}
                                    <li class="list-group-item d-flex justify-content-between">
                                        {{$severity}}
                                        <span class="badge bg-info">{{$count}}</span>
                                    </li>
                                    {{end}}
                                </ul>
                            </div>
                            {{end}}
                        </div>
                    </div>
                </div>
            </div>
        </div>
        {{end}}

        <!-- Recommendations -->
        {{if .Recommendations}}
        <div class="row mb-4">
            <div class="col-12">
                <div class="card">
                    <div class="card-header bg-primary text-white">
                        <h3 class="card-title mb-0">
                            <i class="fas fa-lightbulb me-2"></i>
                            Recommendations
                        </h3>
                    </div>
                    <div class="card-body">
                        {{range $index, $rec := .Recommendations}}
                        {{if lt $index 10}}
                        <div class="recommendation-card card mb-3">
                            <div class="card-body">
                                <h5 class="card-title">
                                    {{$rec.Title}}
                                    <span class="badge bg-{{if eq $rec.Priority "critical"}}danger{{else if eq $rec.Priority "high"}}warning{{else}}info{{end}} ms-2">
                                        {{$rec.Priority}}
                                    </span>
                                </h5>
                                <p class="card-text">{{$rec.Description}}</p>
                                {{if $rec.Implementation}}
                                <div class="alert alert-info">
                                    <strong>Implementation:</strong> {{$rec.Implementation}}
                                </div>
                                {{end}}
                                {{if $rec.EstimatedEffort}}
                                <small class="text-muted">Estimated Effort: {{$rec.EstimatedEffort}}</small>
                                {{end}}
                            </div>
                        </div>
                        {{end}}
                        {{end}}
                    </div>
                </div>
            </div>
        </div>
        {{end}}

        <!-- Footer -->
        <div class="row mt-5">
            <div class="col-12 text-center text-muted">
                <hr>
                <p>Report generated by E2E Test Reporting System v{{.ReportVersion}}</p>
                <p>Generated at {{formatTime .GeneratedAt}}</p>
            </div>
        </div>
    </div>

    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/js/bootstrap.bundle.min.js"></script>
</body>
</html>
`
