package e2e_test

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// PDFFormatter formats reports as PDF (using HTML to PDF conversion approach)
type PDFFormatter struct {
	config *PDFConfig
}

// PDFConfig configuration for PDF formatting
type PDFConfig struct {
	PageSize      string  // A4, Letter, etc.
	Orientation   string  // Portrait, Landscape
	MarginTop     float64 // in mm
	MarginBottom  float64 // in mm
	MarginLeft    float64 // in mm
	MarginRight   float64 // in mm
	HeaderHeight  float64 // in mm
	FooterHeight  float64 // in mm
	FontSize      int     // Base font size
	IncludeCharts bool    // Include performance charts
	WatermarkText string  // Optional watermark
}

func (f *PDFFormatter) Format(data *ComprehensiveReport) ([]byte, error) {
	// Generate HTML content with PDF-specific styling
	htmlContent, err := f.generatePDFHTML(data)
	if err != nil {
		return nil, fmt.Errorf("failed to generate PDF HTML: %w", err)
	}

	// Note: In a real implementation, you would use a library like wkhtmltopdf or Chrome headless
	// For this example, we'll return the HTML content that can be converted to PDF
	// Example tools: go-wkhtmltopdf, chromedp, or gopdf

	return []byte(htmlContent), nil
}

func (f *PDFFormatter) Extension() string {
	return "pdf"
}

func (f *PDFFormatter) ContentType() string {
	return "application/pdf"
}

func (f *PDFFormatter) generatePDFHTML(data *ComprehensiveReport) (string, error) {
	var buf bytes.Buffer

	// PDF-specific CSS with page breaks and print styling
	buf.WriteString(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>E2E Test Comprehensive Report</title>
    <style>
        @page {
            size: A4;
            margin: 20mm;
            @top-center {
                content: "E2E Test Comprehensive Report";
                font-size: 10pt;
                color: #666;
            }
            @bottom-center {
                content: "Page " counter(page) " of " counter(pages);
                font-size: 10pt;
                color: #666;
            }
        }
        
        body {
            font-family: Arial, sans-serif;
            font-size: 11pt;
            line-height: 1.4;
            color: #333;
            margin: 0;
            padding: 0;
        }
        
        .header {
            text-align: center;
            border-bottom: 2px solid #007bff;
            padding-bottom: 20px;
            margin-bottom: 30px;
        }
        
        .header h1 {
            color: #007bff;
            font-size: 24pt;
            margin: 0;
        }
        
        .header .metadata {
            color: #666;
            font-size: 10pt;
            margin-top: 10px;
        }
        
        .section {
            margin-bottom: 25px;
            page-break-inside: avoid;
        }
        
        .section-title {
            color: #007bff;
            font-size: 16pt;
            font-weight: bold;
            border-bottom: 1px solid #007bff;
            padding-bottom: 5px;
            margin-bottom: 15px;
        }
        
        .subsection-title {
            color: #333;
            font-size: 14pt;
            font-weight: bold;
            margin-top: 20px;
            margin-bottom: 10px;
        }
        
        .metrics-grid {
            display: grid;
            grid-template-columns: repeat(3, 1fr);
            gap: 15px;
            margin-bottom: 20px;
        }
        
        .metric-card {
            border: 1px solid #ddd;
            padding: 15px;
            border-radius: 5px;
            background-color: #f9f9f9;
        }
        
        .metric-value {
            font-size: 18pt;
            font-weight: bold;
            color: #007bff;
        }
        
        .metric-label {
            font-size: 10pt;
            color: #666;
            margin-top: 5px;
        }
        
        .status-success { color: #28a745; }
        .status-failed { color: #dc3545; }
        .status-warning { color: #ffc107; }
        
        table {
            width: 100%;
            border-collapse: collapse;
            margin-bottom: 20px;
            font-size: 10pt;
        }
        
        th, td {
            border: 1px solid #ddd;
            padding: 8px;
            text-align: left;
        }
        
        th {
            background-color: #f8f9fa;
            font-weight: bold;
        }
        
        .page-break {
            page-break-before: always;
        }
        
        .no-break {
            page-break-inside: avoid;
        }
        
        .executive-summary {
            background-color: #e7f3ff;
            padding: 20px;
            border-left: 5px solid #007bff;
            margin-bottom: 30px;
        }
        
        .critical-issues {
            background-color: #f8d7da;
            border-left: 5px solid #dc3545;
            padding: 15px;
            margin-bottom: 20px;
        }
        
        .recommendations {
            background-color: #d1ecf1;
            border-left: 5px solid #17a2b8;
            padding: 15px;
            margin-bottom: 20px;
        }
        
        ul.findings {
            list-style-type: none;
            padding-left: 0;
        }
        
        ul.findings li {
            padding: 5px 0;
            padding-left: 20px;
        }
        
        ul.findings li:before {
            content: "✓ ";
            color: #28a745;
            font-weight: bold;
        }
        
        ul.issues {
            list-style-type: none;
            padding-left: 0;
        }
        
        ul.issues li {
            padding: 5px 0;
            padding-left: 20px;
        }
        
        ul.issues li:before {
            content: "⚠ ";
            color: #dc3545;
            font-weight: bold;
        }
        
        .chart-placeholder {
            width: 100%;
            height: 200px;
            border: 2px dashed #ddd;
            display: flex;
            align-items: center;
            justify-content: center;
            color: #666;
            font-style: italic;
            margin: 20px 0;
        }
        
        .watermark {
            position: fixed;
            top: 50%;
            left: 50%;
            transform: translate(-50%, -50%) rotate(-45deg);
            font-size: 72pt;
            color: rgba(0, 0, 0, 0.1);
            z-index: -1;
            pointer-events: none;
        }
    </style>
</head>
<body>`)

	// Add watermark if configured
	if f.config != nil && f.config.WatermarkText != "" {
		buf.WriteString(fmt.Sprintf(`<div class="watermark">%s</div>`, f.config.WatermarkText))
	}

	// Header
	buf.WriteString(`<div class="header">
        <h1>E2E Test Comprehensive Report</h1>
        <div class="metadata">`)
	buf.WriteString(fmt.Sprintf("Generated: %s<br>", data.GeneratedAt.Format("2006-01-02 15:04:05")))
	buf.WriteString(fmt.Sprintf("Report ID: %s<br>", data.ReportID))
	buf.WriteString(fmt.Sprintf("Test Run ID: %s", data.TestRunID))
	buf.WriteString(`</div></div>`)

	// Executive Summary
	if data.ExecutiveSummary != nil {
		buf.WriteString(`<div class="section executive-summary">
            <div class="section-title">Executive Summary</div>`)

		buf.WriteString(`<div class="metrics-grid">`)

		// Status card
		statusClass := "status-success"
		if string(data.ExecutiveSummary.OverallStatus) == "failed" {
			statusClass = "status-failed"
		}
		buf.WriteString(fmt.Sprintf(`<div class="metric-card">
            <div class="metric-value %s">%s</div>
            <div class="metric-label">Overall Status</div>
        </div>`, statusClass, data.ExecutiveSummary.OverallStatus))

		// Total tests card
		buf.WriteString(fmt.Sprintf(`<div class="metric-card">
            <div class="metric-value">%d</div>
            <div class="metric-label">Total Tests</div>
        </div>`, data.ExecutiveSummary.TotalTests))

		// Pass rate card
		passRateClass := "status-success"
		if data.ExecutiveSummary.PassRate < 90 {
			passRateClass = "status-warning"
		}
		if data.ExecutiveSummary.PassRate < 80 {
			passRateClass = "status-failed"
		}
		buf.WriteString(fmt.Sprintf(`<div class="metric-card">
            <div class="metric-value %s">%.2f%%</div>
            <div class="metric-label">Pass Rate</div>
        </div>`, passRateClass, data.ExecutiveSummary.PassRate))

		// Duration card
		buf.WriteString(fmt.Sprintf(`<div class="metric-card">
            <div class="metric-value">%v</div>
            <div class="metric-label">Duration</div>
        </div>`, data.ExecutiveSummary.Duration))

		// Performance score card
		buf.WriteString(fmt.Sprintf(`<div class="metric-card">
            <div class="metric-value">%.1f/100</div>
            <div class="metric-label">Performance Score</div>
        </div>`, data.ExecutiveSummary.PerformanceScore))

		// Coverage score card
		buf.WriteString(fmt.Sprintf(`<div class="metric-card">
            <div class="metric-value">%.1f%%</div>
            <div class="metric-label">Coverage Score</div>
        </div>`, data.ExecutiveSummary.CoverageScore))

		buf.WriteString(`</div>`)

		// Key findings
		if len(data.ExecutiveSummary.KeyFindings) > 0 {
			buf.WriteString(`<div class="subsection-title">Key Findings</div>
                <ul class="findings">`)
			for _, finding := range data.ExecutiveSummary.KeyFindings {
				buf.WriteString(fmt.Sprintf("<li>%s</li>", finding))
			}
			buf.WriteString(`</ul>`)
		}

		// Critical issues
		if len(data.ExecutiveSummary.CriticalIssues) > 0 {
			buf.WriteString(`<div class="critical-issues">
                <div class="subsection-title">Critical Issues</div>
                <ul class="issues">`)
			for _, issue := range data.ExecutiveSummary.CriticalIssues {
				buf.WriteString(fmt.Sprintf("<li>%s</li>", issue))
			}
			buf.WriteString(`</ul></div>`)
		}

		buf.WriteString(`</div>`)
	}

	// Test Metrics Section
	if data.TestMetrics != nil {
		buf.WriteString(`<div class="section page-break">
            <div class="section-title">Test Execution Metrics</div>`)

		buf.WriteString(`<table>
            <tr><th>Metric</th><th>Value</th></tr>`)
		buf.WriteString(fmt.Sprintf("<tr><td>Total Tests</td><td>%d</td></tr>", data.TestMetrics.TotalTests))
		buf.WriteString(fmt.Sprintf("<tr><td>Passed Tests</td><td class=\"status-success\">%d</td></tr>", data.TestMetrics.PassedTests))
		buf.WriteString(fmt.Sprintf("<tr><td>Failed Tests</td><td class=\"status-failed\">%d</td></tr>", data.TestMetrics.FailedTests))
		buf.WriteString(fmt.Sprintf("<tr><td>Skipped Tests</td><td class=\"status-warning\">%d</td></tr>", data.TestMetrics.SkippedTests))
		buf.WriteString(fmt.Sprintf("<tr><td>Success Rate</td><td>%.2f%%</td></tr>", data.TestMetrics.SuccessRate))
		buf.WriteString(fmt.Sprintf("<tr><td>Total Duration</td><td>%v</td></tr>", data.TestMetrics.TotalDuration))
		buf.WriteString(fmt.Sprintf("<tr><td>Average Duration</td><td>%v</td></tr>", data.TestMetrics.AverageDuration))
		buf.WriteString(fmt.Sprintf("<tr><td>Tests Per Second</td><td>%.2f</td></tr>", data.TestMetrics.TestsPerSecond))
		buf.WriteString(`</table>`)

		// Tests by scenario chart placeholder
		if f.config != nil && f.config.IncludeCharts && len(data.TestMetrics.TestsByScenario) > 0 {
			buf.WriteString(`<div class="chart-placeholder">
                [Tests by Scenario Chart - Would be generated in actual PDF]
            </div>`)
		}

		buf.WriteString(`</div>`)
	}

	// System Metrics Section
	if data.SystemMetrics != nil {
		buf.WriteString(`<div class="section">
            <div class="section-title">System Resource Metrics</div>`)

		buf.WriteString(`<div class="metrics-grid">`)
		buf.WriteString(fmt.Sprintf(`<div class="metric-card">
            <div class="metric-value">%.1f MB</div>
            <div class="metric-label">Peak Memory</div>
        </div>`, data.SystemMetrics.PeakMemoryMB))
		buf.WriteString(fmt.Sprintf(`<div class="metric-card">
            <div class="metric-value">%.1f MB</div>
            <div class="metric-label">Memory Growth</div>
        </div>`, data.SystemMetrics.MemoryGrowthMB))
		buf.WriteString(fmt.Sprintf(`<div class="metric-card">
            <div class="metric-value">%d</div>
            <div class="metric-label">Peak Goroutines</div>
        </div>`, data.SystemMetrics.PeakGoroutines))
		buf.WriteString(`</div>`)

		buf.WriteString(`<table>
            <tr><th>System Information</th><th>Value</th></tr>`)
		buf.WriteString(fmt.Sprintf("<tr><td>Operating System</td><td>%s %s</td></tr>", data.SystemMetrics.GOOS, data.SystemMetrics.GOARCH))
		buf.WriteString(fmt.Sprintf("<tr><td>CPU Cores</td><td>%d</td></tr>", data.SystemMetrics.NumCPU))
		buf.WriteString(fmt.Sprintf("<tr><td>Go Version</td><td>%s</td></tr>", data.SystemMetrics.GoVersion))
		buf.WriteString(fmt.Sprintf("<tr><td>GC Count</td><td>%d</td></tr>", data.SystemMetrics.GCCount))
		buf.WriteString(fmt.Sprintf("<tr><td>Memory Efficiency</td><td>%.2f</td></tr>", data.SystemMetrics.MemoryEfficiency))
		buf.WriteString(`</table>`)

		buf.WriteString(`</div>`)
	}

	// Performance Analysis
	if data.PerformanceData != nil {
		buf.WriteString(`<div class="section page-break">
            <div class="section-title">Performance Analysis</div>`)

		buf.WriteString(fmt.Sprintf(`<div class="metric-card">
            <div class="metric-value">%.1f/100</div>
            <div class="metric-label">Overall Performance Score</div>
        </div>`, data.PerformanceData.PerformanceScore))

		if data.PerformanceData.RegressionDetection != nil {
			buf.WriteString(`<div class="subsection-title">Regression Analysis</div>`)
			buf.WriteString(`<table>
                <tr><th>Metric</th><th>Value</th></tr>`)
			buf.WriteString(fmt.Sprintf("<tr><td>Regression Risk</td><td>%s</td></tr>", data.PerformanceData.RegressionDetection.RegressionRisk))
			buf.WriteString(fmt.Sprintf("<tr><td>Regressions Detected</td><td>%d</td></tr>", len(data.PerformanceData.RegressionDetection.RegressionsDetected)))
			buf.WriteString(fmt.Sprintf("<tr><td>Improvements Detected</td><td>%d</td></tr>", len(data.PerformanceData.RegressionDetection.ImprovementsDetected)))
			buf.WriteString(`</table>`)
		}

		buf.WriteString(`</div>`)
	}

	// Error Analysis
	if data.ErrorAnalysis != nil && data.ErrorAnalysis.TotalErrors > 0 {
		buf.WriteString(`<div class="section page-break">
            <div class="section-title">Error Analysis</div>`)

		buf.WriteString(fmt.Sprintf(`<div class="critical-issues">
            <strong>Total Errors: %d</strong>
        </div>`, data.ErrorAnalysis.TotalErrors))

		if len(data.ErrorAnalysis.ErrorsByType) > 0 {
			buf.WriteString(`<div class="subsection-title">Errors by Type</div>
                <table>
                    <tr><th>Error Type</th><th>Count</th></tr>`)
			for errorType, count := range data.ErrorAnalysis.ErrorsByType {
				buf.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%d</td></tr>", errorType, count))
			}
			buf.WriteString(`</table>`)
		}

		buf.WriteString(`</div>`)
	}

	// Recommendations
	if len(data.Recommendations) > 0 {
		buf.WriteString(`<div class="section page-break">
            <div class="section-title">Recommendations</div>`)

		for i, rec := range data.Recommendations {
			if i >= 10 { // Limit to top 10 for PDF
				break
			}

			priorityClass := "status-success"
			if rec.Priority == "high" || rec.Priority == "critical" {
				priorityClass = "status-failed"
			} else if rec.Priority == "medium" {
				priorityClass = "status-warning"
			}

			buf.WriteString(`<div class="recommendations no-break">`)
			buf.WriteString(fmt.Sprintf(`<div class="subsection-title">%d. %s 
                <span class="%s">(%s priority)</span></div>`, i+1, rec.Title, priorityClass, rec.Priority))
			buf.WriteString(fmt.Sprintf("<p>%s</p>", rec.Description))
			if rec.Implementation != "" {
				buf.WriteString(fmt.Sprintf("<p><strong>Implementation:</strong> %s</p>", rec.Implementation))
			}
			buf.WriteString(`</div>`)
		}

		buf.WriteString(`</div>`)
	}

	// Footer
	buf.WriteString(`</body></html>`)

	return buf.String(), nil
}

// MarkdownFormatter formats reports as Markdown
type MarkdownFormatter struct {
	config *MarkdownConfig
}

// MarkdownConfig configuration for Markdown formatting
type MarkdownConfig struct {
	IncludeTOC     bool
	IncludeCharts  bool
	ChartFormat    string // "ascii", "mermaid", "plotly"
	MaxTableRows   int
	IncludeRawData bool
	GitHubFlavored bool
	IncludeEmojis  bool
}

func (f *MarkdownFormatter) Format(data *ComprehensiveReport) ([]byte, error) {
	var buf bytes.Buffer

	// Front matter for GitHub Pages or Jekyll
	if f.config != nil && f.config.GitHubFlavored {
		buf.WriteString("---\n")
		buf.WriteString("title: E2E Test Comprehensive Report\n")
		buf.WriteString(fmt.Sprintf("date: %s\n", data.GeneratedAt.Format("2006-01-02")))
		buf.WriteString("layout: report\n")
		buf.WriteString("---\n\n")
	}

	// Title
	if f.config != nil && f.config.IncludeEmojis {
		buf.WriteString("# 📊 E2E Test Comprehensive Report\n\n")
	} else {
		buf.WriteString("# E2E Test Comprehensive Report\n\n")
	}

	// Metadata
	buf.WriteString("## Report Information\n\n")
	buf.WriteString(fmt.Sprintf("- **Generated:** %s\n", data.GeneratedAt.Format("2006-01-02 15:04:05")))
	buf.WriteString(fmt.Sprintf("- **Report ID:** %s\n", data.ReportID))
	buf.WriteString(fmt.Sprintf("- **Test Run ID:** %s\n", data.TestRunID))
	buf.WriteString(fmt.Sprintf("- **Report Version:** %s\n\n", data.ReportVersion))

	// Table of Contents
	if f.config != nil && f.config.IncludeTOC {
		buf.WriteString("## Table of Contents\n\n")
		buf.WriteString("- [Executive Summary](#executive-summary)\n")
		buf.WriteString("- [Test Execution Metrics](#test-execution-metrics)\n")
		buf.WriteString("- [System Resource Metrics](#system-resource-metrics)\n")
		buf.WriteString("- [MCP Client Metrics](#mcp-client-metrics)\n")
		buf.WriteString("- [Performance Analysis](#performance-analysis)\n")
		buf.WriteString("- [Error Analysis](#error-analysis)\n")
		buf.WriteString("- [Coverage Analysis](#coverage-analysis)\n")
		buf.WriteString("- [Recommendations](#recommendations)\n\n")
	}

	// Executive Summary
	if data.ExecutiveSummary != nil {
		if f.config != nil && f.config.IncludeEmojis {
			buf.WriteString("## 📋 Executive Summary\n\n")
		} else {
			buf.WriteString("## Executive Summary\n\n")
		}

		// Status badge
		statusEmoji := "✅"
		if string(data.ExecutiveSummary.OverallStatus) == "failed" {
			statusEmoji = "❌"
		}

		buf.WriteString("### Overview\n\n")
		buf.WriteString("| Metric | Value |\n")
		buf.WriteString("|--------|-------|\n")
		buf.WriteString(fmt.Sprintf("| Overall Status | %s %s |\n", statusEmoji, data.ExecutiveSummary.OverallStatus))
		buf.WriteString(fmt.Sprintf("| Total Tests | %d |\n", data.ExecutiveSummary.TotalTests))
		buf.WriteString(fmt.Sprintf("| Pass Rate | %.2f%% |\n", data.ExecutiveSummary.PassRate))
		buf.WriteString(fmt.Sprintf("| Duration | %v |\n", data.ExecutiveSummary.Duration))
		buf.WriteString(fmt.Sprintf("| Performance Score | %.1f/100 |\n", data.ExecutiveSummary.PerformanceScore))
		buf.WriteString(fmt.Sprintf("| Quality Score | %.1f/100 |\n", data.ExecutiveSummary.QualityScore))
		buf.WriteString(fmt.Sprintf("| Coverage Score | %.1f%% |\n\n", data.ExecutiveSummary.CoverageScore))

		// Key findings
		if len(data.ExecutiveSummary.KeyFindings) > 0 {
			buf.WriteString("### ⭐ Key Findings\n\n")
			for _, finding := range data.ExecutiveSummary.KeyFindings {
				buf.WriteString(fmt.Sprintf("- ✅ %s\n", finding))
			}
			buf.WriteString("\n")
		}

		// Critical issues
		if len(data.ExecutiveSummary.CriticalIssues) > 0 {
			buf.WriteString("### ⚠️ Critical Issues\n\n")
			for _, issue := range data.ExecutiveSummary.CriticalIssues {
				buf.WriteString(fmt.Sprintf("- ❌ %s\n", issue))
			}
			buf.WriteString("\n")
		}

		// Recommended actions
		if len(data.ExecutiveSummary.RecommendedActions) > 0 {
			buf.WriteString("### 💡 Recommended Actions\n\n")
			for _, action := range data.ExecutiveSummary.RecommendedActions {
				buf.WriteString(fmt.Sprintf("- ➡️ %s\n", action))
			}
			buf.WriteString("\n")
		}
	}

	// Test Execution Metrics
	if data.TestMetrics != nil {
		if f.config != nil && f.config.IncludeEmojis {
			buf.WriteString("## 🧪 Test Execution Metrics\n\n")
		} else {
			buf.WriteString("## Test Execution Metrics\n\n")
		}

		buf.WriteString("| Metric | Value |\n")
		buf.WriteString("|--------|-------|\n")
		buf.WriteString(fmt.Sprintf("| Total Tests | %d |\n", data.TestMetrics.TotalTests))
		buf.WriteString(fmt.Sprintf("| Passed Tests | %d |\n", data.TestMetrics.PassedTests))
		buf.WriteString(fmt.Sprintf("| Failed Tests | %d |\n", data.TestMetrics.FailedTests))
		buf.WriteString(fmt.Sprintf("| Skipped Tests | %d |\n", data.TestMetrics.SkippedTests))
		buf.WriteString(fmt.Sprintf("| Success Rate | %.2f%% |\n", data.TestMetrics.SuccessRate))
		buf.WriteString(fmt.Sprintf("| Total Duration | %v |\n", data.TestMetrics.TotalDuration))
		buf.WriteString(fmt.Sprintf("| Average Duration | %v |\n", data.TestMetrics.AverageDuration))
		buf.WriteString(fmt.Sprintf("| Fastest Test | %v |\n", data.TestMetrics.FastestTest))
		buf.WriteString(fmt.Sprintf("| Slowest Test | %v |\n", data.TestMetrics.SlowestTest))
		buf.WriteString(fmt.Sprintf("| Tests Per Second | %.2f |\n", data.TestMetrics.TestsPerSecond))
		buf.WriteString(fmt.Sprintf("| Timeouts | %d |\n", data.TestMetrics.TimeoutCount))
		buf.WriteString(fmt.Sprintf("| Retries | %d |\n\n", data.TestMetrics.RetryCount))

		// Tests by scenario
		if len(data.TestMetrics.TestsByScenario) > 0 {
			buf.WriteString("### Tests by Scenario\n\n")
			buf.WriteString("| Scenario | Count |\n")
			buf.WriteString("|----------|-------|\n")

			// Sort scenarios by count (descending)
			type scenarioCount struct {
				name  string
				count int64
			}
			var scenarios []scenarioCount
			for name, count := range data.TestMetrics.TestsByScenario {
				scenarios = append(scenarios, scenarioCount{name, count})
			}
			sort.Slice(scenarios, func(i, j int) bool {
				return scenarios[i].count > scenarios[j].count
			})

			for _, sc := range scenarios {
				buf.WriteString(fmt.Sprintf("| %s | %d |\n", sc.name, sc.count))
			}
			buf.WriteString("\n")
		}

		// Chart placeholder
		if f.config != nil && f.config.IncludeCharts && f.config.ChartFormat == "mermaid" {
			buf.WriteString("### Test Results Distribution\n\n")
			buf.WriteString("```mermaid\n")
			buf.WriteString("pie title Test Results\n")
			buf.WriteString(fmt.Sprintf("    \"Passed\" : %d\n", data.TestMetrics.PassedTests))
			buf.WriteString(fmt.Sprintf("    \"Failed\" : %d\n", data.TestMetrics.FailedTests))
			buf.WriteString(fmt.Sprintf("    \"Skipped\" : %d\n", data.TestMetrics.SkippedTests))
			buf.WriteString("```\n\n")
		}
	}

	// System Resource Metrics
	if data.SystemMetrics != nil {
		if f.config != nil && f.config.IncludeEmojis {
			buf.WriteString("## 💻 System Resource Metrics\n\n")
		} else {
			buf.WriteString("## System Resource Metrics\n\n")
		}

		buf.WriteString("### Memory Usage\n\n")
		buf.WriteString("| Metric | Value |\n")
		buf.WriteString("|--------|-------|\n")
		buf.WriteString(fmt.Sprintf("| Initial Memory | %.1f MB |\n", data.SystemMetrics.InitialMemoryMB))
		buf.WriteString(fmt.Sprintf("| Peak Memory | %.1f MB |\n", data.SystemMetrics.PeakMemoryMB))
		buf.WriteString(fmt.Sprintf("| Final Memory | %.1f MB |\n", data.SystemMetrics.FinalMemoryMB))
		buf.WriteString(fmt.Sprintf("| Memory Growth | %.1f MB |\n", data.SystemMetrics.MemoryGrowthMB))
		buf.WriteString(fmt.Sprintf("| Memory Efficiency | %.2f |\n\n", data.SystemMetrics.MemoryEfficiency))

		buf.WriteString("### System Information\n\n")
		buf.WriteString("| Property | Value |\n")
		buf.WriteString("|----------|-------|\n")
		buf.WriteString(fmt.Sprintf("| Operating System | %s %s |\n", data.SystemMetrics.GOOS, data.SystemMetrics.GOARCH))
		buf.WriteString(fmt.Sprintf("| CPU Cores | %d |\n", data.SystemMetrics.NumCPU))
		buf.WriteString(fmt.Sprintf("| Go Version | %s |\n", data.SystemMetrics.GoVersion))
		buf.WriteString(fmt.Sprintf("| GC Count | %d |\n", data.SystemMetrics.GCCount))
		buf.WriteString(fmt.Sprintf("| GC Efficiency | %.2f |\n\n", data.SystemMetrics.GCEfficiency))
	}

	// MCP Client Metrics
	if data.MCPMetrics != nil {
		if f.config != nil && f.config.IncludeEmojis {
			buf.WriteString("## 🔄 MCP Client Metrics\n\n")
		} else {
			buf.WriteString("## MCP Client Metrics\n\n")
		}

		buf.WriteString("| Metric | Value |\n")
		buf.WriteString("|--------|-------|\n")
		buf.WriteString(fmt.Sprintf("| Total Requests | %d |\n", data.MCPMetrics.TotalRequests))
		buf.WriteString(fmt.Sprintf("| Successful Requests | %d |\n", data.MCPMetrics.SuccessfulRequests))
		buf.WriteString(fmt.Sprintf("| Failed Requests | %d |\n", data.MCPMetrics.FailedRequests))
		buf.WriteString(fmt.Sprintf("| Timeout Requests | %d |\n", data.MCPMetrics.TimeoutRequests))
		buf.WriteString(fmt.Sprintf("| Error Rate | %.2f%% |\n", data.MCPMetrics.ErrorRatePercent))
		buf.WriteString(fmt.Sprintf("| Average Latency | %v |\n", data.MCPMetrics.AverageLatency))
		buf.WriteString(fmt.Sprintf("| P95 Latency | %v |\n", data.MCPMetrics.P95Latency))
		buf.WriteString(fmt.Sprintf("| P99 Latency | %v |\n", data.MCPMetrics.P99Latency))
		buf.WriteString(fmt.Sprintf("| Throughput | %.2f req/sec |\n\n", data.MCPMetrics.ThroughputPerSecond))
	}

	// Performance Analysis
	if data.PerformanceData != nil {
		if f.config != nil && f.config.IncludeEmojis {
			buf.WriteString("## 📈 Performance Analysis\n\n")
		} else {
			buf.WriteString("## Performance Analysis\n\n")
		}

		buf.WriteString(fmt.Sprintf("**Overall Performance Score:** %.1f/100\n\n", data.PerformanceData.PerformanceScore))

		if data.PerformanceData.RegressionDetection != nil {
			buf.WriteString("### Regression Analysis\n\n")
			buf.WriteString("| Metric | Value |\n")
			buf.WriteString("|--------|-------|\n")
			buf.WriteString(fmt.Sprintf("| Regression Risk | %s |\n", data.PerformanceData.RegressionDetection.RegressionRisk))
			buf.WriteString(fmt.Sprintf("| Regressions Detected | %d |\n", len(data.PerformanceData.RegressionDetection.RegressionsDetected)))
			buf.WriteString(fmt.Sprintf("| Improvements Detected | %d |\n\n", len(data.PerformanceData.RegressionDetection.ImprovementsDetected)))
		}
	}

	// Error Analysis
	if data.ErrorAnalysis != nil && data.ErrorAnalysis.TotalErrors > 0 {
		if f.config != nil && f.config.IncludeEmojis {
			buf.WriteString("## 🐛 Error Analysis\n\n")
		} else {
			buf.WriteString("## Error Analysis\n\n")
		}

		buf.WriteString(fmt.Sprintf("**Total Errors:** %d\n\n", data.ErrorAnalysis.TotalErrors))

		if len(data.ErrorAnalysis.ErrorsByType) > 0 {
			buf.WriteString("### Errors by Type\n\n")
			buf.WriteString("| Error Type | Count |\n")
			buf.WriteString("|------------|-------|\n")
			for errorType, count := range data.ErrorAnalysis.ErrorsByType {
				buf.WriteString(fmt.Sprintf("| %s | %d |\n", errorType, count))
			}
			buf.WriteString("\n")
		}

		if len(data.ErrorAnalysis.FrequentErrors) > 0 {
			buf.WriteString("### Most Frequent Errors\n\n")
			for i, pattern := range data.ErrorAnalysis.FrequentErrors {
				if i >= 5 { // Top 5
					break
				}
				buf.WriteString(fmt.Sprintf("%d. **%s** (Count: %d, Severity: %s)\n",
					i+1, pattern.Pattern, pattern.Count, pattern.Severity))
			}
			buf.WriteString("\n")
		}
	}

	// Coverage Analysis
	if data.CoverageAnalysis != nil && data.CoverageAnalysis.OverallCoverage != nil {
		if f.config != nil && f.config.IncludeEmojis {
			buf.WriteString("## 📊 Coverage Analysis\n\n")
		} else {
			buf.WriteString("## Coverage Analysis\n\n")
		}

		buf.WriteString("| Coverage Type | Percentage |\n")
		buf.WriteString("|---------------|------------|\n")
		buf.WriteString(fmt.Sprintf("| Total Coverage | %.2f%% |\n", data.CoverageAnalysis.OverallCoverage.TotalCoveragePercent))
		buf.WriteString(fmt.Sprintf("| Scenario Coverage | %.2f%% |\n", data.CoverageAnalysis.OverallCoverage.ScenarioCoveragePercent))
		buf.WriteString(fmt.Sprintf("| Language Coverage | %.2f%% |\n", data.CoverageAnalysis.OverallCoverage.LanguageCoveragePercent))
		buf.WriteString(fmt.Sprintf("| Method Coverage | %.2f%% |\n", data.CoverageAnalysis.OverallCoverage.MethodCoveragePercent))
		buf.WriteString(fmt.Sprintf("| Feature Coverage | %.2f%% |\n\n", data.CoverageAnalysis.OverallCoverage.FeatureCoveragePercent))
	}

	// Recommendations
	if len(data.Recommendations) > 0 {
		if f.config != nil && f.config.IncludeEmojis {
			buf.WriteString("## 💡 Recommendations\n\n")
		} else {
			buf.WriteString("## Recommendations\n\n")
		}

		for i, rec := range data.Recommendations {
			if i >= 10 { // Top 10
				break
			}

			priorityEmoji := "🔵"
			switch rec.Priority {
			case "critical":
				priorityEmoji = "🔴"
			case "high":
				priorityEmoji = "🟠"
			case "medium":
				priorityEmoji = "🟡"
			case "low":
				priorityEmoji = "🟢"
			}

			buf.WriteString(fmt.Sprintf("### %d. %s %s %s\n\n", i+1, priorityEmoji, rec.Title, strings.ToUpper(rec.Priority)))
			buf.WriteString(fmt.Sprintf("%s\n\n", rec.Description))

			if rec.Implementation != "" {
				buf.WriteString(fmt.Sprintf("**Implementation:** %s\n\n", rec.Implementation))
			}

			if rec.EstimatedEffort != "" {
				buf.WriteString(fmt.Sprintf("**Estimated Effort:** %s\n\n", rec.EstimatedEffort))
			}
		}
	}

	// Raw Data Section (if enabled)
	if f.config != nil && f.config.IncludeRawData && data.RawData != nil {
		buf.WriteString("## Raw Data\n\n")
		buf.WriteString("```json\n")
		for key, value := range data.RawData {
			buf.WriteString(fmt.Sprintf("\"%s\": %v,\n", key, value))
		}
		buf.WriteString("```\n\n")
	}

	// Footer
	buf.WriteString("---\n\n")
	buf.WriteString(fmt.Sprintf("*Report generated by E2E Test Reporting System v%s at %s*\n",
		data.ReportVersion, data.GeneratedAt.Format("2006-01-02 15:04:05")))

	return buf.Bytes(), nil
}

func (f *MarkdownFormatter) Extension() string {
	return "md"
}

func (f *MarkdownFormatter) ContentType() string {
	return "text/markdown"
}

// TSVFormatter formats reports as Tab-Separated Values
type TSVFormatter struct {
	config *TSVConfig
}

// TSVConfig configuration for TSV formatting
type TSVConfig struct {
	IncludeHeaders    bool
	IncludeMetadata   bool
	FlattenStructures bool
	MaxRows           int
	DateFormat        string
}

func (f *TSVFormatter) Format(data *ComprehensiveReport) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	writer.Comma = '\t' // Use tab as separator

	// Write metadata section
	if f.config != nil && f.config.IncludeMetadata {
		if f.config.IncludeHeaders {
			writer.Write([]string{"Section", "Key", "Value"})
		}

		writer.Write([]string{"Metadata", "GeneratedAt", data.GeneratedAt.Format(f.getDateFormat())})
		writer.Write([]string{"Metadata", "ReportID", data.ReportID})
		writer.Write([]string{"Metadata", "TestRunID", data.TestRunID})
		writer.Write([]string{"Metadata", "ReportVersion", data.ReportVersion})
		writer.Write([]string{}) // Empty row as separator
	}

	// Write executive summary
	if data.ExecutiveSummary != nil {
		if f.config.IncludeHeaders {
			writer.Write([]string{"Section", "Metric", "Value"})
		}

		writer.Write([]string{"ExecutiveSummary", "OverallStatus", string(data.ExecutiveSummary.OverallStatus)})
		writer.Write([]string{"ExecutiveSummary", "TotalTests", strconv.FormatInt(data.ExecutiveSummary.TotalTests, 10)})
		writer.Write([]string{"ExecutiveSummary", "PassRate", fmt.Sprintf("%.2f", data.ExecutiveSummary.PassRate)})
		writer.Write([]string{"ExecutiveSummary", "Duration", data.ExecutiveSummary.Duration.String()})
		writer.Write([]string{"ExecutiveSummary", "PerformanceScore", fmt.Sprintf("%.2f", data.ExecutiveSummary.PerformanceScore)})
		writer.Write([]string{"ExecutiveSummary", "QualityScore", fmt.Sprintf("%.2f", data.ExecutiveSummary.QualityScore)})
		writer.Write([]string{"ExecutiveSummary", "CoverageScore", fmt.Sprintf("%.2f", data.ExecutiveSummary.CoverageScore)})
		writer.Write([]string{}) // Empty row as separator
	}

	// Write test metrics
	if data.TestMetrics != nil {
		if f.config.IncludeHeaders {
			writer.Write([]string{"Section", "Metric", "Value"})
		}

		writer.Write([]string{"TestMetrics", "TotalTests", strconv.FormatInt(data.TestMetrics.TotalTests, 10)})
		writer.Write([]string{"TestMetrics", "PassedTests", strconv.FormatInt(data.TestMetrics.PassedTests, 10)})
		writer.Write([]string{"TestMetrics", "FailedTests", strconv.FormatInt(data.TestMetrics.FailedTests, 10)})
		writer.Write([]string{"TestMetrics", "SkippedTests", strconv.FormatInt(data.TestMetrics.SkippedTests, 10)})
		writer.Write([]string{"TestMetrics", "SuccessRate", fmt.Sprintf("%.2f", data.TestMetrics.SuccessRate)})
		writer.Write([]string{"TestMetrics", "TotalDuration", data.TestMetrics.TotalDuration.String()})
		writer.Write([]string{"TestMetrics", "AverageDuration", data.TestMetrics.AverageDuration.String()})
		writer.Write([]string{"TestMetrics", "FastestTest", data.TestMetrics.FastestTest.String()})
		writer.Write([]string{"TestMetrics", "SlowestTest", data.TestMetrics.SlowestTest.String()})
		writer.Write([]string{"TestMetrics", "TestsPerSecond", fmt.Sprintf("%.2f", data.TestMetrics.TestsPerSecond)})
		writer.Write([]string{"TestMetrics", "TimeoutCount", strconv.FormatInt(data.TestMetrics.TimeoutCount, 10)})
		writer.Write([]string{"TestMetrics", "RetryCount", strconv.FormatInt(data.TestMetrics.RetryCount, 10)})
		writer.Write([]string{}) // Empty row as separator
	}

	// Write system metrics
	if data.SystemMetrics != nil {
		if f.config.IncludeHeaders {
			writer.Write([]string{"Section", "Metric", "Value"})
		}

		writer.Write([]string{"SystemMetrics", "InitialMemoryMB", fmt.Sprintf("%.2f", data.SystemMetrics.InitialMemoryMB)})
		writer.Write([]string{"SystemMetrics", "PeakMemoryMB", fmt.Sprintf("%.2f", data.SystemMetrics.PeakMemoryMB)})
		writer.Write([]string{"SystemMetrics", "FinalMemoryMB", fmt.Sprintf("%.2f", data.SystemMetrics.FinalMemoryMB)})
		writer.Write([]string{"SystemMetrics", "MemoryGrowthMB", fmt.Sprintf("%.2f", data.SystemMetrics.MemoryGrowthMB)})
		writer.Write([]string{"SystemMetrics", "InitialGoroutines", strconv.Itoa(data.SystemMetrics.InitialGoroutines)})
		writer.Write([]string{"SystemMetrics", "PeakGoroutines", strconv.Itoa(data.SystemMetrics.PeakGoroutines)})
		writer.Write([]string{"SystemMetrics", "FinalGoroutines", strconv.Itoa(data.SystemMetrics.FinalGoroutines)})
		writer.Write([]string{"SystemMetrics", "GoroutineGrowth", strconv.Itoa(data.SystemMetrics.GoroutineGrowth)})
		writer.Write([]string{"SystemMetrics", "GCCount", strconv.FormatUint(uint64(data.SystemMetrics.GCCount), 10)})
		writer.Write([]string{"SystemMetrics", "MemoryEfficiency", fmt.Sprintf("%.2f", data.SystemMetrics.MemoryEfficiency)})
		writer.Write([]string{"SystemMetrics", "GCEfficiency", fmt.Sprintf("%.2f", data.SystemMetrics.GCEfficiency)})
		writer.Write([]string{"SystemMetrics", "GOOS", data.SystemMetrics.GOOS})
		writer.Write([]string{"SystemMetrics", "GOARCH", data.SystemMetrics.GOARCH})
		writer.Write([]string{"SystemMetrics", "NumCPU", strconv.Itoa(data.SystemMetrics.NumCPU)})
		writer.Write([]string{"SystemMetrics", "GoVersion", data.SystemMetrics.GoVersion})
		writer.Write([]string{}) // Empty row as separator
	}

	// Write MCP metrics
	if data.MCPMetrics != nil {
		if f.config.IncludeHeaders {
			writer.Write([]string{"Section", "Metric", "Value"})
		}

		writer.Write([]string{"MCPMetrics", "TotalRequests", strconv.FormatInt(data.MCPMetrics.TotalRequests, 10)})
		writer.Write([]string{"MCPMetrics", "SuccessfulRequests", strconv.FormatInt(data.MCPMetrics.SuccessfulRequests, 10)})
		writer.Write([]string{"MCPMetrics", "FailedRequests", strconv.FormatInt(data.MCPMetrics.FailedRequests, 10)})
		writer.Write([]string{"MCPMetrics", "TimeoutRequests", strconv.FormatInt(data.MCPMetrics.TimeoutRequests, 10)})
		writer.Write([]string{"MCPMetrics", "ErrorRatePercent", fmt.Sprintf("%.2f", data.MCPMetrics.ErrorRatePercent)})
		writer.Write([]string{"MCPMetrics", "AverageLatency", data.MCPMetrics.AverageLatency.String()})
		writer.Write([]string{"MCPMetrics", "MinLatency", data.MCPMetrics.MinLatency.String()})
		writer.Write([]string{"MCPMetrics", "MaxLatency", data.MCPMetrics.MaxLatency.String()})
		writer.Write([]string{"MCPMetrics", "P95Latency", data.MCPMetrics.P95Latency.String()})
		writer.Write([]string{"MCPMetrics", "P99Latency", data.MCPMetrics.P99Latency.String()})
		writer.Write([]string{"MCPMetrics", "ThroughputPerSecond", fmt.Sprintf("%.2f", data.MCPMetrics.ThroughputPerSecond)})
		writer.Write([]string{}) // Empty row as separator
	}

	// Write individual test results
	if len(data.TestResults) > 0 {
		// Header for test results
		if f.config.IncludeHeaders {
			writer.Write([]string{
				"TestID", "TestName", "Scenario", "Language", "StartTime", "EndTime",
				"Duration", "Status", "Success", "ErrorCount", "WarningCount",
				"RequestCount", "SuccessfulRequests", "FailedRequests", "AverageLatency",
				"ThroughputPerSecond", "ErrorRate", "MemoryUsedMB", "CPUTimeMS",
			})
		}

		maxRows := len(data.TestResults)
		if f.config != nil && f.config.MaxRows > 0 && f.config.MaxRows < maxRows {
			maxRows = f.config.MaxRows
		}

		for i, result := range data.TestResults[:maxRows] {
			record := []string{
				result.TestID,
				result.TestName,
				result.Scenario,
				result.Language,
				result.StartTime.Format(f.getDateFormat()),
				result.EndTime.Format(f.getDateFormat()),
				result.Duration.String(),
				string(result.Status),
				strconv.FormatBool(result.Success),
				strconv.Itoa(len(result.Errors)),
				strconv.Itoa(len(result.Warnings)),
			}

			// Add metrics if available
			if result.Metrics != nil {
				record = append(record,
					strconv.FormatInt(result.Metrics.RequestCount, 10),
					strconv.FormatInt(result.Metrics.SuccessfulRequests, 10),
					strconv.FormatInt(result.Metrics.FailedRequests, 10),
					result.Metrics.AverageLatency.String(),
					fmt.Sprintf("%.2f", result.Metrics.ThroughputPerSecond),
					fmt.Sprintf("%.2f", result.Metrics.ErrorRate),
				)
			} else {
				record = append(record, "", "", "", "", "", "")
			}

			// Add resource usage if available
			if result.ResourceUsage != nil {
				record = append(record,
					fmt.Sprintf("%.2f", result.ResourceUsage.MemoryUsedMB),
					strconv.FormatInt(result.ResourceUsage.CPUTimeMS, 10),
				)
			} else {
				record = append(record, "", "")
			}

			writer.Write(record)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("TSV writer error: %w", err)
	}

	return buf.Bytes(), nil
}

func (f *TSVFormatter) Extension() string {
	return "tsv"
}

func (f *TSVFormatter) ContentType() string {
	return "text/tab-separated-values"
}

func (f *TSVFormatter) getDateFormat() string {
	if f.config != nil && f.config.DateFormat != "" {
		return f.config.DateFormat
	}
	return "2006-01-02 15:04:05"
}

// XMLFormatter formats reports as XML
type XMLFormatter struct {
	config *XMLConfig
}

// XMLConfig configuration for XML formatting
type XMLConfig struct {
	IncludeSchema   bool
	PrettyPrint     bool
	IncludeMetadata bool
	NamespaceURI    string
	SchemaLocation  string
}

func (f *XMLFormatter) Format(data *ComprehensiveReport) ([]byte, error) {
	var buf bytes.Buffer

	// XML declaration
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	buf.WriteString("\n")

	// Root element with namespace
	if f.config != nil && f.config.NamespaceURI != "" {
		buf.WriteString(fmt.Sprintf(`<E2ETestReport xmlns="%s"`, f.config.NamespaceURI))
		if f.config.SchemaLocation != "" {
			buf.WriteString(fmt.Sprintf(` xsi:schemaLocation="%s %s" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"`,
				f.config.NamespaceURI, f.config.SchemaLocation))
		}
		buf.WriteString(">\n")
	} else {
		buf.WriteString("<E2ETestReport>\n")
	}

	// Report metadata
	buf.WriteString("  <ReportMetadata>\n")
	buf.WriteString(fmt.Sprintf("    <GeneratedAt>%s</GeneratedAt>\n", data.GeneratedAt.Format(time.RFC3339)))
	buf.WriteString(fmt.Sprintf("    <ReportID>%s</ReportID>\n", f.xmlEscape(data.ReportID)))
	buf.WriteString(fmt.Sprintf("    <TestRunID>%s</TestRunID>\n", f.xmlEscape(data.TestRunID)))
	buf.WriteString(fmt.Sprintf("    <ReportVersion>%s</ReportVersion>\n", f.xmlEscape(data.ReportVersion)))
	buf.WriteString("  </ReportMetadata>\n")

	// Executive Summary
	if data.ExecutiveSummary != nil {
		buf.WriteString("  <ExecutiveSummary>\n")
		buf.WriteString(fmt.Sprintf("    <OverallStatus>%s</OverallStatus>\n", data.ExecutiveSummary.OverallStatus))
		buf.WriteString(fmt.Sprintf("    <TotalTests>%d</TotalTests>\n", data.ExecutiveSummary.TotalTests))
		buf.WriteString(fmt.Sprintf("    <PassRate>%.2f</PassRate>\n", data.ExecutiveSummary.PassRate))
		buf.WriteString(fmt.Sprintf("    <Duration>%s</Duration>\n", data.ExecutiveSummary.Duration))
		buf.WriteString(fmt.Sprintf("    <PerformanceScore>%.2f</PerformanceScore>\n", data.ExecutiveSummary.PerformanceScore))
		buf.WriteString(fmt.Sprintf("    <QualityScore>%.2f</QualityScore>\n", data.ExecutiveSummary.QualityScore))
		buf.WriteString(fmt.Sprintf("    <CoverageScore>%.2f</CoverageScore>\n", data.ExecutiveSummary.CoverageScore))

		// Key findings
		if len(data.ExecutiveSummary.KeyFindings) > 0 {
			buf.WriteString("    <KeyFindings>\n")
			for _, finding := range data.ExecutiveSummary.KeyFindings {
				buf.WriteString(fmt.Sprintf("      <Finding>%s</Finding>\n", f.xmlEscape(finding)))
			}
			buf.WriteString("    </KeyFindings>\n")
		}

		// Critical issues
		if len(data.ExecutiveSummary.CriticalIssues) > 0 {
			buf.WriteString("    <CriticalIssues>\n")
			for _, issue := range data.ExecutiveSummary.CriticalIssues {
				buf.WriteString(fmt.Sprintf("      <Issue>%s</Issue>\n", f.xmlEscape(issue)))
			}
			buf.WriteString("    </CriticalIssues>\n")
		}

		buf.WriteString("  </ExecutiveSummary>\n")
	}

	// Test Metrics
	if data.TestMetrics != nil {
		buf.WriteString("  <TestMetrics>\n")
		buf.WriteString(fmt.Sprintf("    <TotalTests>%d</TotalTests>\n", data.TestMetrics.TotalTests))
		buf.WriteString(fmt.Sprintf("    <PassedTests>%d</PassedTests>\n", data.TestMetrics.PassedTests))
		buf.WriteString(fmt.Sprintf("    <FailedTests>%d</FailedTests>\n", data.TestMetrics.FailedTests))
		buf.WriteString(fmt.Sprintf("    <SkippedTests>%d</SkippedTests>\n", data.TestMetrics.SkippedTests))
		buf.WriteString(fmt.Sprintf("    <SuccessRate>%.2f</SuccessRate>\n", data.TestMetrics.SuccessRate))
		buf.WriteString(fmt.Sprintf("    <TotalDuration>%s</TotalDuration>\n", data.TestMetrics.TotalDuration))
		buf.WriteString(fmt.Sprintf("    <AverageDuration>%s</AverageDuration>\n", data.TestMetrics.AverageDuration))
		buf.WriteString(fmt.Sprintf("    <TestsPerSecond>%.2f</TestsPerSecond>\n", data.TestMetrics.TestsPerSecond))
		buf.WriteString("  </TestMetrics>\n")
	}

	// System Metrics
	if data.SystemMetrics != nil {
		buf.WriteString("  <SystemMetrics>\n")
		buf.WriteString(fmt.Sprintf("    <PeakMemoryMB>%.2f</PeakMemoryMB>\n", data.SystemMetrics.PeakMemoryMB))
		buf.WriteString(fmt.Sprintf("    <MemoryGrowthMB>%.2f</MemoryGrowthMB>\n", data.SystemMetrics.MemoryGrowthMB))
		buf.WriteString(fmt.Sprintf("    <PeakGoroutines>%d</PeakGoroutines>\n", data.SystemMetrics.PeakGoroutines))
		buf.WriteString(fmt.Sprintf("    <GCCount>%d</GCCount>\n", data.SystemMetrics.GCCount))
		buf.WriteString(fmt.Sprintf("    <GOOS>%s</GOOS>\n", data.SystemMetrics.GOOS))
		buf.WriteString(fmt.Sprintf("    <GOARCH>%s</GOARCH>\n", data.SystemMetrics.GOARCH))
		buf.WriteString(fmt.Sprintf("    <NumCPU>%d</NumCPU>\n", data.SystemMetrics.NumCPU))
		buf.WriteString(fmt.Sprintf("    <GoVersion>%s</GoVersion>\n", f.xmlEscape(data.SystemMetrics.GoVersion)))
		buf.WriteString("  </SystemMetrics>\n")
	}

	// Test Results (limited to avoid huge XML files)
	if len(data.TestResults) > 0 {
		buf.WriteString("  <TestResults>\n")
		maxResults := 100 // Limit for XML
		for i, result := range data.TestResults {
			if i >= maxResults {
				break
			}

			buf.WriteString("    <TestResult>\n")
			buf.WriteString(fmt.Sprintf("      <TestID>%s</TestID>\n", f.xmlEscape(result.TestID)))
			buf.WriteString(fmt.Sprintf("      <TestName>%s</TestName>\n", f.xmlEscape(result.TestName)))
			buf.WriteString(fmt.Sprintf("      <Scenario>%s</Scenario>\n", f.xmlEscape(result.Scenario)))
			buf.WriteString(fmt.Sprintf("      <Language>%s</Language>\n", f.xmlEscape(result.Language)))
			buf.WriteString(fmt.Sprintf("      <StartTime>%s</StartTime>\n", result.StartTime.Format(time.RFC3339)))
			buf.WriteString(fmt.Sprintf("      <Duration>%s</Duration>\n", result.Duration))
			buf.WriteString(fmt.Sprintf("      <Status>%s</Status>\n", result.Status))
			buf.WriteString(fmt.Sprintf("      <Success>%t</Success>\n", result.Success))
			buf.WriteString(fmt.Sprintf("      <ErrorCount>%d</ErrorCount>\n", len(result.Errors)))
			buf.WriteString("    </TestResult>\n")
		}
		buf.WriteString("  </TestResults>\n")
	}

	// Recommendations
	if len(data.Recommendations) > 0 {
		buf.WriteString("  <Recommendations>\n")
		for i, rec := range data.Recommendations {
			if i >= 10 { // Limit recommendations
				break
			}

			buf.WriteString("    <Recommendation>\n")
			buf.WriteString(fmt.Sprintf("      <ID>%s</ID>\n", f.xmlEscape(rec.ID)))
			buf.WriteString(fmt.Sprintf("      <Title>%s</Title>\n", f.xmlEscape(rec.Title)))
			buf.WriteString(fmt.Sprintf("      <Description>%s</Description>\n", f.xmlEscape(rec.Description)))
			buf.WriteString(fmt.Sprintf("      <Priority>%s</Priority>\n", f.xmlEscape(string(rec.Priority))))
			buf.WriteString(fmt.Sprintf("      <Category>%s</Category>\n", f.xmlEscape(string(rec.Category))))
			if rec.Implementation != "" {
				buf.WriteString(fmt.Sprintf("      <Implementation>%s</Implementation>\n", f.xmlEscape(rec.Implementation)))
			}
			buf.WriteString("    </Recommendation>\n")
		}
		buf.WriteString("  </Recommendations>\n")
	}

	buf.WriteString("</E2ETestReport>\n")

	return buf.Bytes(), nil
}

func (f *XMLFormatter) Extension() string {
	return "xml"
}

func (f *XMLFormatter) ContentType() string {
	return "application/xml"
}

func (f *XMLFormatter) xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

// NewPDFFormatter creates a new PDF formatter with configuration
func NewPDFFormatter(config *PDFConfig) *PDFFormatter {
	if config == nil {
		config = &PDFConfig{
			PageSize:      "A4",
			Orientation:   "Portrait",
			MarginTop:     20,
			MarginBottom:  20,
			MarginLeft:    20,
			MarginRight:   20,
			FontSize:      11,
			IncludeCharts: true,
		}
	}
	return &PDFFormatter{config: config}
}

// NewMarkdownFormatter creates a new Markdown formatter with configuration
func NewMarkdownFormatter(config *MarkdownConfig) *MarkdownFormatter {
	if config == nil {
		config = &MarkdownConfig{
			IncludeTOC:     true,
			IncludeCharts:  true,
			ChartFormat:    "mermaid",
			MaxTableRows:   100,
			GitHubFlavored: true,
			IncludeEmojis:  true,
		}
	}
	return &MarkdownFormatter{config: config}
}

// NewTSVFormatter creates a new TSV formatter with configuration
func NewTSVFormatter(config *TSVConfig) *TSVFormatter {
	if config == nil {
		config = &TSVConfig{
			IncludeHeaders:    true,
			IncludeMetadata:   true,
			FlattenStructures: true,
			MaxRows:           1000,
			DateFormat:        "2006-01-02 15:04:05",
		}
	}
	return &TSVFormatter{config: config}
}

// NewXMLFormatter creates a new XML formatter with configuration
func NewXMLFormatter(config *XMLConfig) *XMLFormatter {
	if config == nil {
		config = &XMLConfig{
			IncludeSchema:   true,
			PrettyPrint:     true,
			IncludeMetadata: true,
		}
	}
	return &XMLFormatter{config: config}
}
