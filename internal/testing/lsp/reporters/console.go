package reporters

import (
	"fmt"
	"strings"

	"lsp-gateway/internal/testing/lsp/cases"
	"lsp-gateway/internal/testing/lsp/validators"
)

// ConsoleReporter reports test results to the console
type ConsoleReporter struct {
	verbose      bool
	colorEnabled bool
}

// NewConsoleReporter creates a new console reporter
func NewConsoleReporter(verbose bool, colorEnabled bool) *ConsoleReporter {
	return &ConsoleReporter{
		verbose:      verbose,
		colorEnabled: colorEnabled,
	}
}

// ReportTestResult reports the overall test result
func (r *ConsoleReporter) ReportTestResult(result *cases.TestResult) {
	r.printHeader("LSP Test Results")
	r.println("")

	// Print summary
	r.printSummary(result)
	r.println("")

	// Print test suite details
	for _, testSuite := range result.TestSuites {
		r.reportTestSuite(testSuite)
	}

	// Print final status
	r.printFinalStatus(result)
}

// printHeader prints a formatted header
func (r *ConsoleReporter) printHeader(title string) {
	border := strings.Repeat("=", len(title)+4)
	r.println(border)
	r.println(fmt.Sprintf("  %s  ", title))
	r.println(border)
}

// printSummary prints the test summary
func (r *ConsoleReporter) printSummary(result *cases.TestResult) {
	r.println("SUMMARY")
	r.println("-------")
	r.println(fmt.Sprintf("Total Test Suites: %d", len(result.TestSuites)))
	r.println(fmt.Sprintf("Total Test Cases:  %d", result.TotalCases))
	r.println(fmt.Sprintf("Passed:           %s", r.colorize(fmt.Sprintf("%d", result.PassedCases), "green")))
	r.println(fmt.Sprintf("Failed:           %s", r.colorize(fmt.Sprintf("%d", result.FailedCases), "red")))
	r.println(fmt.Sprintf("Skipped:          %s", r.colorize(fmt.Sprintf("%d", result.SkippedCases), "yellow")))
	r.println(fmt.Sprintf("Errors:           %s", r.colorize(fmt.Sprintf("%d", result.ErrorCases), "red")))
	r.println(fmt.Sprintf("Pass Rate:        %s", r.colorize(fmt.Sprintf("%.1f%%", result.PassRate()), "green")))
	r.println(fmt.Sprintf("Duration:         %v", result.Duration))
}

// reportTestSuite reports details for a test suite
func (r *ConsoleReporter) reportTestSuite(testSuite *cases.TestSuite) {
	// Print suite header
	status := r.getStatusSymbol(testSuite.Status)
	r.println(fmt.Sprintf("%s Test Suite: %s (%s)", status, testSuite.Name, testSuite.Repository.Language))

	if r.verbose {
		r.println(fmt.Sprintf("  Description: %s", testSuite.Description))
		r.println(fmt.Sprintf("  Workspace:   %s", testSuite.WorkspaceDir))
		r.println(fmt.Sprintf("  Duration:    %v", testSuite.Duration))
		r.println(fmt.Sprintf("  Results:     %d passed, %d failed, %d skipped, %d errors",
			testSuite.PassedCases, testSuite.FailedCases, testSuite.SkippedCases, testSuite.ErrorCases))
	}

	// Print test cases
	for _, testCase := range testSuite.TestCases {
		r.reportTestCase(testCase)
	}

	r.println("")
}

// reportTestCase reports details for a test case
func (r *ConsoleReporter) reportTestCase(testCase *cases.TestCase) {
	status := r.getStatusSymbol(testCase.Status)
	r.println(fmt.Sprintf("  %s %s (%s)", status, testCase.Name, testCase.Method))

	if r.verbose {
		r.println(fmt.Sprintf("    File:     %s", testCase.Config.File))
		r.println(fmt.Sprintf("    Position: line %d, character %d", testCase.Position.Line, testCase.Position.Character))
		r.println(fmt.Sprintf("    Duration: %v", testCase.Duration))

		if testCase.Error != nil {
			r.println(fmt.Sprintf("    Error:    %s", r.colorize(testCase.Error.Error(), "red")))
		}
	}

	// Print validation results if verbose or there are failures
	if r.verbose || testCase.Status == cases.TestStatusFailed {
		r.reportValidationResults(testCase.ValidationResults)
	}
}

// reportValidationResults reports validation results
func (r *ConsoleReporter) reportValidationResults(results []*cases.ValidationResult) {
	if len(results) == 0 {
		return
	}

	summary := validators.GetValidationSummary(results)
	if r.verbose {
		r.println(fmt.Sprintf("    Validations: %d total, %d passed, %d failed (%.1f%% pass rate)",
			summary["total"], summary["passed"], summary["failed"], summary["pass_rate"]))
	}

	for _, result := range results {
		if !result.Passed || r.verbose {
			symbol := r.getValidationSymbol(result.Passed)
			color := "red"
			if result.Passed {
				color = "green"
			}
			r.println(fmt.Sprintf("      %s %s: %s",
				symbol,
				result.Description,
				r.colorize(result.Message, color)))
		}
	}
}

// printFinalStatus prints the final status
func (r *ConsoleReporter) printFinalStatus(result *cases.TestResult) {
	r.println(strings.Repeat("-", 50))

	if result.Success() {
		r.println(r.colorize("✓ ALL TESTS PASSED", "green"))
	} else {
		r.println(r.colorize("✗ SOME TESTS FAILED", "red"))
	}

	r.println(fmt.Sprintf("Completed in %v", result.Duration))
}

// getStatusSymbol returns a symbol for the test status
func (r *ConsoleReporter) getStatusSymbol(status cases.TestStatus) string {
	switch status {
	case cases.TestStatusPassed:
		return r.colorize("✓", "green")
	case cases.TestStatusFailed:
		return r.colorize("✗", "red")
	case cases.TestStatusSkipped:
		return r.colorize("⊘", "yellow")
	case cases.TestStatusError:
		return r.colorize("✗", "red")
	case cases.TestStatusRunning:
		return r.colorize("⋯", "blue")
	case cases.TestStatusPending:
		return r.colorize("◯", "gray")
	default:
		return "?"
	}
}

// getValidationSymbol returns a symbol for validation result
func (r *ConsoleReporter) getValidationSymbol(passed bool) string {
	if passed {
		return r.colorize("✓", "green")
	}
	return r.colorize("✗", "red")
}

// colorize applies color to text if colors are enabled
func (r *ConsoleReporter) colorize(text, color string) string {
	if !r.colorEnabled {
		return text
	}

	switch color {
	case "red":
		return fmt.Sprintf("\033[31m%s\033[0m", text)
	case "green":
		return fmt.Sprintf("\033[32m%s\033[0m", text)
	case "yellow":
		return fmt.Sprintf("\033[33m%s\033[0m", text)
	case "blue":
		return fmt.Sprintf("\033[34m%s\033[0m", text)
	case "gray":
		return fmt.Sprintf("\033[37m%s\033[0m", text)
	default:
		return text
	}
}

// println prints a line to the console
func (r *ConsoleReporter) println(text string) {
	fmt.Println(text)
}

// ReportProgress reports test progress (for live updates)
func (r *ConsoleReporter) ReportProgress(completed, total int, currentTest *cases.TestCase) {
	if !r.verbose {
		return
	}

	percentage := float64(completed) / float64(total) * 100
	progress := fmt.Sprintf("[%d/%d] %.1f%%", completed, total, percentage)

	if currentTest != nil {
		fmt.Printf("\r%s - Running: %s (%s)", progress, currentTest.Name, currentTest.Method)
	} else {
		fmt.Printf("\r%s", progress)
	}
}

// ReportTestCaseResult reports individual test case completion
func (r *ConsoleReporter) ReportTestCaseResult(testCase *cases.TestCase) {
	if !r.verbose {
		return
	}

	status := r.getStatusSymbol(testCase.Status)
	fmt.Printf("\n%s %s (%s) - %v\n", status, testCase.Name, testCase.Method, testCase.Duration)

	if testCase.Status == cases.TestStatusFailed && testCase.Error != nil {
		fmt.Printf("  Error: %s\n", r.colorize(testCase.Error.Error(), "red"))
	}
}

// ClearProgress clears the progress line
func (r *ConsoleReporter) ClearProgress() {
	if r.verbose {
		fmt.Print("\r" + strings.Repeat(" ", 100) + "\r")
	}
}
