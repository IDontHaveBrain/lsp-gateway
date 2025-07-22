package scenarios

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	textcases "golang.org/x/text/cases"
	textlanguage "golang.org/x/text/language"

	"lsp-gateway/internal/testing/lsp"
	"lsp-gateway/internal/testing/lsp/cases"
)

// ScenarioCLIOptions contains options for the scenario CLI commands
type ScenarioCLIOptions struct {
	// Scenario selection
	Languages    []string
	Repositories []string
	Methods      []string
	Tags         []string
	ScenarioIDs  []string

	// Execution options
	MaxConcurrency          int
	Timeout                 time.Duration
	FailFast                bool
	IncludePerformanceTests bool
	StrictMode              bool
	RetryAttempts           int

	// Output options
	Verbose         bool
	OutputFormat    string
	OutputDirectory string
	SaveDetails     bool

	// Configuration
	ConfigFile   string
	ScenariosDir string
	WorkspaceDir string

	// Special modes
	DryRun       bool
	ListOnly     bool
	ValidateOnly bool
}

// DefaultScenarioCLIOptions returns default CLI options
func DefaultScenarioCLIOptions() *ScenarioCLIOptions {
	return &ScenarioCLIOptions{
		Languages:               []string{},
		Repositories:            []string{},
		Methods:                 []string{},
		Tags:                    []string{},
		ScenarioIDs:             []string{},
		MaxConcurrency:          4,
		Timeout:                 10 * time.Minute,
		FailFast:                false,
		IncludePerformanceTests: false,
		StrictMode:              false,
		RetryAttempts:           2,
		Verbose:                 false,
		OutputFormat:            "console",
		OutputDirectory:         "test-results",
		SaveDetails:             false,
		ConfigFile:              "",
		ScenariosDir:            "./internal/testing/lsp/scenarios",
		WorkspaceDir:            "",
		DryRun:                  false,
		ListOnly:                false,
		ValidateOnly:            false,
	}
}

// CreateScenarioCommands creates the CLI commands for scenario testing
func CreateScenarioCommands() *cobra.Command {
	options := DefaultScenarioCLIOptions()

	scenarioCmd := &cobra.Command{
		Use:   "scenario",
		Short: "Run LSP test scenarios",
		Long: `Run comprehensive LSP test scenarios across multiple languages and repositories.
		
Scenarios provide realistic testing patterns for Go, Python, TypeScript, and Java
language servers using real-world code examples and patterns.`,
		Example: `
  # Run all scenarios for all languages
  lsp-gateway scenario run
  
  # Run scenarios for specific languages
  lsp-gateway scenario run --language go --language python
  
  # Run scenarios with specific tags
  lsp-gateway scenario run --tag definition --tag interface
  
  # Run performance scenarios only
  lsp-gateway scenario run --performance --timeout 30s
  
  # List available scenarios
  lsp-gateway scenario list --language go
  
  # Validate scenario configurations
  lsp-gateway scenario validate
  
  # Run in dry-run mode to see what would be executed
  lsp-gateway scenario run --dry-run --verbose`,
		SilenceUsage: true,
	}

	// Add persistent flags
	scenarioCmd.PersistentFlags().StringSliceVar(&options.Languages, "language", []string{}, "Languages to test (go, python, typescript, java)")
	scenarioCmd.PersistentFlags().StringSliceVar(&options.Repositories, "repository", []string{}, "Repositories to include")
	scenarioCmd.PersistentFlags().StringSliceVar(&options.Methods, "method", []string{}, "LSP methods to test")
	scenarioCmd.PersistentFlags().StringSliceVar(&options.Tags, "tag", []string{}, "Tags to filter by")
	scenarioCmd.PersistentFlags().StringSliceVar(&options.ScenarioIDs, "scenario", []string{}, "Specific scenario IDs to run")
	scenarioCmd.PersistentFlags().StringVar(&options.ScenariosDir, "scenarios-dir", options.ScenariosDir, "Directory containing scenario files")
	scenarioCmd.PersistentFlags().StringVar(&options.ConfigFile, "config", "", "Configuration file path")
	scenarioCmd.PersistentFlags().BoolVar(&options.Verbose, "verbose", false, "Enable verbose output")

	// Add subcommands
	scenarioCmd.AddCommand(createRunCommand(options))
	scenarioCmd.AddCommand(createListCommand(options))
	scenarioCmd.AddCommand(createValidateCommand(options))
	scenarioCmd.AddCommand(createInfoCommand(options))
	scenarioCmd.AddCommand(createGenerateCommand(options))

	return scenarioCmd
}

// createRunCommand creates the run subcommand
func createRunCommand(options *ScenarioCLIOptions) *cobra.Command {
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run LSP test scenarios",
		Long:  "Execute LSP test scenarios with specified filters and options",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScenarios(cmd.Context(), options)
		},
	}

	// Execution options
	runCmd.Flags().IntVar(&options.MaxConcurrency, "concurrency", options.MaxConcurrency, "Maximum concurrent test execution")
	runCmd.Flags().DurationVar(&options.Timeout, "timeout", options.Timeout, "Overall test timeout")
	runCmd.Flags().BoolVar(&options.FailFast, "fail-fast", options.FailFast, "Stop on first failure")
	runCmd.Flags().BoolVar(&options.IncludePerformanceTests, "performance", options.IncludePerformanceTests, "Include performance test scenarios")
	runCmd.Flags().BoolVar(&options.StrictMode, "strict", options.StrictMode, "Enable strict validation mode")
	runCmd.Flags().IntVar(&options.RetryAttempts, "retry", options.RetryAttempts, "Number of retry attempts for failed tests")

	// Output options
	runCmd.Flags().StringVar(&options.OutputFormat, "format", options.OutputFormat, "Output format (console, json, junit)")
	runCmd.Flags().StringVar(&options.OutputDirectory, "output-dir", options.OutputDirectory, "Output directory for results")
	runCmd.Flags().BoolVar(&options.SaveDetails, "save-details", options.SaveDetails, "Save detailed test results")

	// Special modes
	runCmd.Flags().BoolVar(&options.DryRun, "dry-run", options.DryRun, "Show what would be executed without running tests")
	runCmd.Flags().StringVar(&options.WorkspaceDir, "workspace-dir", "", "Temporary workspace directory")

	return runCmd
}

// createListCommand creates the list subcommand
func createListCommand(options *ScenarioCLIOptions) *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List available scenarios",
		Long:  "List all available test scenarios with filtering options",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listScenarios(cmd.Context(), options)
		},
	}

	listCmd.Flags().StringVar(&options.OutputFormat, "format", "table", "Output format (table, json, yaml)")
	listCmd.Flags().BoolVar(&options.IncludePerformanceTests, "performance", false, "Include performance scenarios")

	return listCmd
}

// createValidateCommand creates the validate subcommand
func createValidateCommand(options *ScenarioCLIOptions) *cobra.Command {
	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate scenario configurations",
		Long:  "Validate all scenario configuration files for syntax and consistency",
		RunE: func(cmd *cobra.Command, args []string) error {
			return validateScenarios(cmd.Context(), options)
		},
	}

	return validateCmd
}

// createInfoCommand creates the info subcommand
func createInfoCommand(options *ScenarioCLIOptions) *cobra.Command {
	infoCmd := &cobra.Command{
		Use:   "info",
		Short: "Show scenario information",
		Long:  "Display detailed information about loaded scenarios and statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			return showScenarioInfo(cmd.Context(), options)
		},
	}

	infoCmd.Flags().StringVar(&options.OutputFormat, "format", "table", "Output format (table, json)")

	return infoCmd
}

// createGenerateCommand creates the generate subcommand
func createGenerateCommand(options *ScenarioCLIOptions) *cobra.Command {
	generateCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate scenario templates",
		Long:  "Generate new scenario configuration templates for development",
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateScenarioTemplates(cmd.Context(), options, args)
		},
	}

	generateCmd.Flags().StringVar(&options.OutputDirectory, "output-dir", ".", "Output directory for generated templates")

	return generateCmd
}

// runScenarios executes the scenario tests
func runScenarios(ctx context.Context, options *ScenarioCLIOptions) error {
	fmt.Printf("🚀 Running LSP Test Scenarios\n\n")

	if options.Verbose {
		fmt.Printf("Configuration:\n")
		fmt.Printf("  Languages: %v\n", options.Languages)
		fmt.Printf("  Repositories: %v\n", options.Repositories)
		fmt.Printf("  Methods: %v\n", options.Methods)
		fmt.Printf("  Tags: %v\n", options.Tags)
		fmt.Printf("  Scenarios Dir: %s\n", options.ScenariosDir)
		fmt.Printf("  Max Concurrency: %d\n", options.MaxConcurrency)
		fmt.Printf("  Timeout: %v\n", options.Timeout)
		fmt.Printf("  Performance Tests: %v\n", options.IncludePerformanceTests)
		fmt.Printf("\n")
	}

	// Create scenario framework
	frameworkOptions := &lsp.FrameworkOptions{
		ConfigPath:   options.ConfigFile,
		Verbose:      options.Verbose,
		ColorEnabled: true,
		DryRun:       options.DryRun,
	}

	framework, err := NewScenarioFramework(options.ScenariosDir, frameworkOptions)
	if err != nil {
		return fmt.Errorf("failed to create scenario framework: %w", err)
	}

	// Create run options
	runOptions := &ScenarioRunOptions{
		Languages:               options.Languages,
		Repositories:            options.Repositories,
		Methods:                 options.Methods,
		Tags:                    options.Tags,
		IncludePerformanceTests: options.IncludePerformanceTests,
		MaxConcurrency:          options.MaxConcurrency,
		FailFast:                options.FailFast,
		Timeout:                 options.Timeout,
		ValidateFilePatterns:    true,
		StrictModeEnabled:       options.StrictMode,
		RetryFailedTests:        options.RetryAttempts,
	}

	// Dry run mode
	if options.DryRun {
		return showDryRunResults(framework, runOptions)
	}

	// Execute scenarios
	startTime := time.Now()
	result, err := framework.RunScenarios(ctx, runOptions)
	duration := time.Since(startTime)

	if err != nil {
		return fmt.Errorf("scenario execution failed: %w", err)
	}

	// Display results
	if err := displayResults(result, options, duration); err != nil {
		return fmt.Errorf("failed to display results: %w", err)
	}

	// Save results if requested
	if options.SaveDetails {
		if err := saveDetailedResults(result, framework, options); err != nil {
			fmt.Printf("⚠️  Warning: Failed to save detailed results: %v\n", err)
		}
	}

	// Exit with non-zero code if tests failed
	if !result.Success() {
		os.Exit(1)
	}

	return nil
}

// listScenarios lists available scenarios
func listScenarios(ctx context.Context, options *ScenarioCLIOptions) error {
	fmt.Printf("📋 Available LSP Test Scenarios\n\n")

	manager := NewScenarioManager(options.ScenariosDir)
	if err := manager.LoadAllScenarios(); err != nil {
		return fmt.Errorf("failed to load scenarios: %w", err)
	}

	summaries := manager.ListScenarios()

	switch options.OutputFormat {
	case "json":
		return outputJSON(summaries)
	case "yaml":
		return outputYAML(summaries)
	default:
		return outputTable(summaries, options)
	}
}

// validateScenarios validates scenario configurations
func validateScenarios(ctx context.Context, options *ScenarioCLIOptions) error {
	fmt.Printf("✅ Validating LSP Test Scenarios\n\n")

	manager := NewScenarioManager(options.ScenariosDir)
	if err := manager.ValidateScenarioFiles(); err != nil {
		fmt.Printf("❌ Validation failed: %v\n", err)
		return err
	}

	fmt.Printf("✅ All scenario configurations are valid\n")
	return nil
}

// showScenarioInfo displays scenario information
func showScenarioInfo(ctx context.Context, options *ScenarioCLIOptions) error {
	fmt.Printf("ℹ️  LSP Test Scenario Information\n\n")

	manager := NewScenarioManager(options.ScenariosDir)
	if err := manager.LoadAllScenarios(); err != nil {
		return fmt.Errorf("failed to load scenarios: %w", err)
	}

	summaries := manager.ListScenarios()

	switch options.OutputFormat {
	case "json":
		return outputJSON(summaries)
	default:
		return displayScenarioInfo(summaries)
	}
}

// generateScenarioTemplates generates scenario templates
func generateScenarioTemplates(ctx context.Context, options *ScenarioCLIOptions, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("language argument required (go, python, typescript, java)")
	}

	language := args[0]
	if !isValidLanguage(language) {
		return fmt.Errorf("invalid language: %s (valid: go, python, typescript, java)", language)
	}

	fmt.Printf("🏗️  Generating %s scenario template\n\n", language)

	template := generateLanguageTemplate(language)

	outputFile := filepath.Join(options.OutputDirectory, fmt.Sprintf("%s_scenarios_template.yaml", language))
	if err := os.WriteFile(outputFile, []byte(template), 0644); err != nil {
		return fmt.Errorf("failed to write template file: %w", err)
	}

	fmt.Printf("✅ Template generated: %s\n", outputFile)
	return nil
}

// Helper functions

func showDryRunResults(framework *ScenarioFramework, options *ScenarioRunOptions) error {
	fmt.Printf("🔍 Dry Run Mode - No tests will be executed\n\n")

	// Get scenario summaries
	summaries := framework.GetScenarioSummary()

	// Filter based on options
	var totalScenarios int
	for language, summary := range summaries {
		if len(options.Languages) > 0 && !contains(options.Languages, language) {
			continue
		}

		fmt.Printf("Language: %s\n", strings.ToUpper(language))
		fmt.Printf("  Name: %s\n", summary.Name)
		fmt.Printf("  Description: %s\n", summary.Description)
		fmt.Printf("  Test Cases: %d\n", summary.TestCaseCount)
		fmt.Printf("  Performance Tests: %d\n", summary.PerformanceTests)
		fmt.Printf("  Repositories: %d\n", summary.RepositoryCount)

		if len(summary.Methods) > 0 {
			fmt.Printf("  Methods:\n")
			for method, count := range summary.Methods {
				if len(options.Methods) == 0 || contains(options.Methods, method) {
					fmt.Printf("    - %s: %d tests\n", method, count)
					totalScenarios += count
				}
			}
		}

		if len(summary.Tags) > 0 && len(options.Tags) > 0 {
			fmt.Printf("  Matching Tags:\n")
			for tag, count := range summary.Tags {
				if contains(options.Tags, tag) {
					fmt.Printf("    - %s: %d tests\n", tag, count)
				}
			}
		}

		fmt.Printf("\n")
	}

	fmt.Printf("📊 Summary:\n")
	fmt.Printf("  Total scenarios that would run: %d\n", totalScenarios)
	fmt.Printf("  Max concurrency: %d\n", options.MaxConcurrency)
	fmt.Printf("  Timeout: %v\n", options.Timeout)
	fmt.Printf("  Performance tests: %v\n", options.IncludePerformanceTests)

	return nil
}

func displayResults(result *cases.TestResult, options *ScenarioCLIOptions, duration time.Duration) error {
	fmt.Printf("\n📊 Test Results Summary\n")
	fmt.Printf("=" + strings.Repeat("=", 50) + "\n")
	fmt.Printf("Total Tests:    %d\n", result.TotalCases)
	fmt.Printf("Passed:         %d (%.1f%%)\n", result.PassedCases, result.PassRate())
	fmt.Printf("Failed:         %d\n", result.FailedCases)
	fmt.Printf("Skipped:        %d\n", result.SkippedCases)
	fmt.Printf("Errors:         %d\n", result.ErrorCases)
	fmt.Printf("Duration:       %v\n", duration)
	fmt.Printf("=" + strings.Repeat("=", 50) + "\n")

	// Show failed tests
	if result.FailedCases > 0 {
		fmt.Printf("\n❌ Failed Tests:\n")
		for _, suite := range result.TestSuites {
			for _, testCase := range suite.TestCases {
				if testCase.Status == cases.TestStatusFailed {
					fmt.Printf("  - %s: %s\n", testCase.ID, testCase.Name)
					if options.Verbose && testCase.Error != nil {
						fmt.Printf("    Error: %v\n", testCase.Error)
					}
				}
			}
		}
	}

	// Show success message or failure
	if result.Success() {
		fmt.Printf("\n✅ All tests passed!\n")
	} else {
		fmt.Printf("\n❌ Some tests failed. See details above.\n")
	}

	return nil
}

func saveDetailedResults(result *cases.TestResult, framework *ScenarioFramework, options *ScenarioCLIOptions) error {
	// Create output directory
	if err := os.MkdirAll(options.OutputDirectory, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate detailed report
	report := framework.CreateScenarioReport(result)

	// Save JSON report
	jsonFile := filepath.Join(options.OutputDirectory, "scenario-report.json")
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON report: %w", err)
	}

	if err := os.WriteFile(jsonFile, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write JSON report: %w", err)
	}

	fmt.Printf("📄 Detailed results saved to: %s\n", jsonFile)
	return nil
}

func outputJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func outputYAML(data interface{}) error {
	// Would use yaml package to marshal data
	fmt.Printf("YAML output not implemented yet\n")
	return nil
}

func outputTable(summaries map[string]ScenarioSummary, options *ScenarioCLIOptions) error {
	fmt.Printf("%-15s %-30s %-10s %-15s %-15s\n", "LANGUAGE", "NAME", "TESTS", "PERF_TESTS", "REPOSITORIES")
	fmt.Printf("%s\n", strings.Repeat("-", 85))

	for language, summary := range summaries {
		if len(options.Languages) > 0 && !contains(options.Languages, language) {
			continue
		}

		name := summary.Name
		if len(name) > 30 {
			name = name[:27] + "..."
		}

		fmt.Printf("%-15s %-30s %-10d %-15d %-15d\n",
			strings.ToUpper(language),
			name,
			summary.TestCaseCount,
			summary.PerformanceTests,
			summary.RepositoryCount)

		if options.Verbose {
			// Show methods breakdown
			fmt.Printf("  Methods: ")
			var methods []string
			for method, count := range summary.Methods {
				methods = append(methods, fmt.Sprintf("%s(%d)", method, count))
			}
			fmt.Printf("%s\n", strings.Join(methods, ", "))
		}
	}

	return nil
}

func displayScenarioInfo(summaries map[string]ScenarioSummary) error {
	for language, summary := range summaries {
		fmt.Printf("Language: %s\n", strings.ToUpper(language))
		fmt.Printf("  Name: %s\n", summary.Name)
		fmt.Printf("  Description: %s\n", summary.Description)
		fmt.Printf("  Test Cases: %d\n", summary.TestCaseCount)
		fmt.Printf("  Performance Tests: %d\n", summary.PerformanceTests)
		fmt.Printf("  Repositories: %d\n", summary.RepositoryCount)

		fmt.Printf("  Methods:\n")
		for method, count := range summary.Methods {
			fmt.Printf("    - %s: %d\n", method, count)
		}

		fmt.Printf("  Tags:\n")
		for tag, count := range summary.Tags {
			fmt.Printf("    - %s: %d\n", tag, count)
		}

		fmt.Printf("\n")
	}

	return nil
}

func generateLanguageTemplate(language string) string {
	return fmt.Sprintf(`# %s Language Test Scenarios Template
# Generated template for creating new %s test scenarios

name: "%s LSP Scenarios"
description: "Test scenarios for %s language server"
language: "%s"

scenarios:
  - id: "%s_example_definition"
    name: "%s Definition Example"
    description: "Example definition test case"
    method: "textDocument/definition"
    repository: "%s-repo"
    file: "example.%s"
    position:
      line: 10
      character: 15
    expected:
      success: true
      definition:
        has_location: true
    tags: ["%s", "definition", "example"]

test_repositories:
  %s-repo:
    path: "./test-repositories/%s/example"
    setup:
      commands:
        - "# Add setup commands here"
      timeout: "5m"
`,
		textcases.Title(textlanguage.Und).String(language), language,
		textcases.Title(textlanguage.Und).String(language), language, language,
		language, textcases.Title(textlanguage.Und).String(language),
		language, getFileExtension(language),
		language,
		language, language)
}

func isValidLanguage(language string) bool {
	validLanguages := []string{"go", "python", "typescript", "java"}
	return contains(validLanguages, language)
}

func getFileExtension(language string) string {
	switch language {
	case "go":
		return "go"
	case "python":
		return "py"
	case "typescript":
		return "ts"
	case "java":
		return "java"
	default:
		return "txt"
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
