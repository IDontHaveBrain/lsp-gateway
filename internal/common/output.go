package common

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	StatusSuccess     = "success"
	StatusError       = "error"
	StatusWarning     = "warning"
	StatusInfo        = "info"
	StatusPassed      = "passed"
	StatusFailed      = "failed"
	StatusFail        = "fail"
	StatusOK          = "ok"
	StatusWarn        = "warn"
	StatusInformation = "information"
	StatusSkipped     = "skipped"
	StatusSkip        = "skip"
)

const (
	IconSuccess = "✓ SUCCESS"
	IconFailed  = "✗ FAILED"
	IconWarning = "⚠ WARNING"
	IconInfo    = "ℹ INFO"
	IconSkipped = "- SKIPPED"
)

const (
	OutputValidationPassed = "✓ Validation passed"
	OutputValidationFailed = "✗ Validation failed"
)

type OutputManager struct {
	jsonMode bool
	verbose  bool
}

func NewOutputManager(jsonMode, verbose bool) *OutputManager {
	return &OutputManager{
		jsonMode: jsonMode,
		verbose:  verbose,
	}
}

type Result struct {
	Name        string                 `json:"name"`
	Status      string                 `json:"status"` // "success", "error", "warning", "info"
	Message     string                 `json:"message"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Suggestions []string               `json:"suggestions,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

type Report struct {
	Title     string                 `json:"title"`
	Timestamp time.Time              `json:"timestamp"`
	Results   []Result               `json:"results"`
	Summary   map[string]interface{} `json:"summary,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type ValidationReport struct {
	Valid    bool     `json:"valid"`
	Issues   []string `json:"issues,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

func (om *OutputManager) OutputResult(result Result) error {
	if om.jsonMode {
		return om.outputJSON(result)
	}
	return om.outputResultHuman(result)
}

func (om *OutputManager) OutputReport(report Report) error {
	if om.jsonMode {
		return om.outputJSON(report)
	}
	return om.outputReportHuman(report)
}

func (om *OutputManager) OutputValidation(validation ValidationReport) error {
	if om.jsonMode {
		return om.outputJSON(validation)
	}
	return om.outputValidationHuman(validation)
}

func (om *OutputManager) OutputTable(headers []string, rows [][]string) error {
	if om.jsonMode {
		data := make([]map[string]string, len(rows))
		for i, row := range rows {
			entry := make(map[string]string)
			for j, header := range headers {
				if j < len(row) {
					entry[header] = row[j]
				}
			}
			data[i] = entry
		}
		return om.outputJSON(data)
	}

	return om.outputTableHuman(headers, rows)
}

func (om *OutputManager) outputJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func (om *OutputManager) outputResultHuman(result Result) error {
	fmt.Printf("%s: %s\n", result.Name, om.getStatusIcon(result.Status))
	fmt.Printf("  %s\n", result.Message)

	if om.verbose && len(result.Details) > 0 {
		fmt.Printf("  Details:\n")
		for key, value := range result.Details {
			fmt.Printf(FORMAT_DETAIL_VALUE, key, value)
		}
	}

	if len(result.Suggestions) > 0 {
		fmt.Printf("  Suggestions:\n")
		for _, suggestion := range result.Suggestions {
			fmt.Printf("    - %s\n", suggestion)
		}
	}

	fmt.Println()
	return nil
}

func (om *OutputManager) outputReportHuman(report Report) error {
	om.PrintHeader(report.Title)

	if report.Metadata != nil {
		fmt.Printf("Report Details:\n")
		for key, value := range report.Metadata {
			fmt.Printf(FORMAT_LIST_INDENT, cases.Title(language.English).String(key), value)
		}
		fmt.Println()
	}

	for _, result := range report.Results {
		if err := om.outputResultHuman(result); err != nil {
			return err
		}
	}

	if report.Summary != nil {
		fmt.Printf("Summary:\n")
		for key, value := range report.Summary {
			fmt.Printf(FORMAT_LIST_INDENT, cases.Title(language.English).String(key), value)
		}
		fmt.Println()
	}

	return nil
}

func (om *OutputManager) outputValidationHuman(validation ValidationReport) error {
	om.PrintHeader("Validation Results")

	if validation.Valid {
		fmt.Println(OutputValidationPassed)
	} else {
		fmt.Println(OutputValidationFailed)
	}

	if len(validation.Issues) > 0 {
		fmt.Printf("\nIssues:\n")
		for i, issue := range validation.Issues {
			fmt.Printf("  %d. %s\n", i+1, issue)
		}
	}

	if len(validation.Warnings) > 0 {
		fmt.Printf("\nWarnings:\n")
		for i, warning := range validation.Warnings {
			fmt.Printf("  %d. %s\n", i+1, warning)
		}
	}

	fmt.Println()
	return nil
}

func (om *OutputManager) outputTableHuman(headers []string, rows [][]string) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if _, err := fmt.Fprintln(w, strings.Join(headers, "\t")); err != nil {
		return fmt.Errorf("failed to write table headers: %w", err)
	}
	if _, err := fmt.Fprintln(w, strings.Repeat("-", len(strings.Join(headers, "\t")))); err != nil {
		return fmt.Errorf("failed to write table separator: %w", err)
	}

	for _, row := range rows {
		if _, err := fmt.Fprintln(w, strings.Join(row, "\t")); err != nil {
			return fmt.Errorf("failed to write table row: %w", err)
		}
	}

	return w.Flush()
}

func (om *OutputManager) PrintHeader(title string) {
	if om.jsonMode {
		return // Don't print headers in JSON mode
	}

	fmt.Printf("\n%s\n", title)
	fmt.Printf("%s\n\n", strings.Repeat("=", len(title)))
}

func (om *OutputManager) PrintSubheader(title string) {
	if om.jsonMode {
		return // Don't print headers in JSON mode
	}

	fmt.Printf("%s\n", title)
	fmt.Printf("%s\n", strings.Repeat("-", len(title)))
}

func (om *OutputManager) getStatusIcon(status string) string {
	switch strings.ToLower(status) {
	case StatusSuccess, StatusPassed, StatusOK:
		return IconSuccess
	case StatusError, StatusFailed, StatusFail:
		return IconFailed
	case StatusWarning, StatusWarn:
		return IconWarning
	case StatusInfo, StatusInformation:
		return IconInfo
	case StatusSkipped, StatusSkip:
		return IconSkipped
	default:
		return strings.ToUpper(status)
	}
}

func (om *OutputManager) Success(name, message string, details map[string]interface{}) Result {
	return Result{
		Name:      name,
		Status:    StatusSuccess,
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
}

func (om *OutputManager) Error(name, message string, suggestions []string) Result {
	return Result{
		Name:        name,
		Status:      StatusError,
		Message:     message,
		Suggestions: suggestions,
		Timestamp:   time.Now(),
	}
}

func (om *OutputManager) Warning(name, message string, suggestions []string) Result {
	return Result{
		Name:        name,
		Status:      StatusWarning,
		Message:     message,
		Suggestions: suggestions,
		Timestamp:   time.Now(),
	}
}

func (om *OutputManager) Info(name, message string, details map[string]interface{}) Result {
	return Result{
		Name:      name,
		Status:    StatusInfo,
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
}
