package lsp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"lsp-gateway/internal/testing/lsp/cases"
	"lsp-gateway/internal/testing/lsp/config"
	"lsp-gateway/internal/testing/lsp/reporters"
	"lsp-gateway/internal/testing/lsp/runner"
	"lsp-gateway/internal/testing/lsp/validators"
)

// LSPTestFramework is the main entry point for the LSP testing framework
type LSPTestFramework struct {
	config    *config.LSPTestConfig
	runner    *runner.TestRunner
	validator *validators.LSPResponseValidator
	reporter  *reporters.ConsoleReporter
	logger    TestLogger
}

// FrameworkOptions configures the LSP test framework
type FrameworkOptions struct {
	ConfigPath   string
	ReposPath    string
	Verbose      bool
	DryRun       bool
	Filter       *cases.TestCaseFilter
	OutputDir    string
	ColorEnabled bool
	LogTiming    bool
}

// DefaultFrameworkOptions returns default framework options
func DefaultFrameworkOptions() *FrameworkOptions {
	return &FrameworkOptions{
		ConfigPath:   "lsp-test-config.yaml",
		ReposPath:    "test-repositories.yaml",
		Verbose:      false,
		DryRun:       false,
		Filter:       &cases.TestCaseFilter{IncludeSkipped: false},
		OutputDir:    "test-results",
		ColorEnabled: true,
		LogTiming:    true,
	}
}

// NewLSPTestFramework creates a new LSP test framework
func NewLSPTestFramework(options *FrameworkOptions) (*LSPTestFramework, error) {
	if options == nil {
		options = DefaultFrameworkOptions()
	}

	// Load configuration
	testConfig, err := config.LoadConfig(options.ConfigPath)
	if err != nil {
		// If config doesn't exist, use default config
		if os.IsNotExist(err) {
			testConfig = config.DefaultLSPTestConfig()
			// Try to load repositories separately
			if repositories, repoErr := config.LoadRepositoriesConfig(options.ReposPath); repoErr == nil {
				testConfig.Repositories = repositories
			}
		} else {
			return nil, fmt.Errorf("failed to load configuration: %w", err)
		}
	}

	// Update config with runtime options
	if options.OutputDir != "" {
		testConfig.Reporting.OutputDir = options.OutputDir
	}
	if options.Verbose {
		testConfig.Reporting.Verbose = options.Verbose
	}

	// Create logger
	var logger TestLogger
	if options.Verbose {
		logger = NewSimpleTestLogger(true)
	} else {
		logger = NewSimpleTestLogger(false)
	}

	if options.LogTiming {
		logger = NewTimedLogger(logger)
	}

	// Create adapter for the logger interface compatibility
	runnerLogger := &loggerAdapter{logger: logger}

	// Create test runner
	testRunner, err := runner.NewTestRunner(testConfig, runnerLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to create test runner: %w", err)
	}

	// Load test suites
	if err := testRunner.LoadTestSuites(); err != nil {
		return nil, fmt.Errorf("failed to load test suites: %w", err)
	}

	// Create validator
	validator := validators.NewLSPResponseValidator(testConfig.Validation)

	// Create reporter
	reporter := reporters.NewConsoleReporter(options.Verbose, options.ColorEnabled)

	framework := &LSPTestFramework{
		config:    testConfig,
		runner:    testRunner,
		validator: validator,
		reporter:  reporter,
		logger:    logger,
	}

	return framework, nil
}

// Run executes the LSP test framework
func (f *LSPTestFramework) Run(ctx context.Context, options *runner.RunOptions) (*cases.TestResult, error) {
	f.logger.Info("Starting LSP test framework")

	// Run tests
	result, err := f.runner.Run(ctx, options)
	if err != nil {
		f.logger.Error("Test run failed: %v", err)
		return result, err
	}

	// Validate test results
	if err := f.validateResults(result); err != nil {
		f.logger.Error("Result validation failed: %v", err)
		return result, err
	}

	// Report results
	f.reporter.ReportTestResult(result)

	// Save results if configured
	if f.config.Reporting.SaveDetails {
		if err := f.saveResults(result); err != nil {
			f.logger.Warn("Failed to save results: %v", err)
		}
	}

	f.logger.Info("LSP test framework completed")
	return result, nil
}

// validateResults validates all test case results
func (f *LSPTestFramework) validateResults(result *cases.TestResult) error {
	f.logger.Info("Validating test results...")

	for _, testSuite := range result.TestSuites {
		for _, testCase := range testSuite.TestCases {
			// Skip validation for skipped or error cases
			if testCase.Status == cases.TestStatusSkipped || testCase.Status == cases.TestStatusError {
				continue
			}

			if testCase.Status == cases.TestStatusPassed {
				// Validate the response
				if err := f.validator.ValidateTestCase(testCase); err != nil {
					f.logger.Error("Validation failed for test case %s: %v", testCase.ID, err)
					// Don't return error, just log it - validation updates the test case status
				}
			}
		}
	}

	return nil
}

// saveResults saves detailed test results to files
func (f *LSPTestFramework) saveResults(result *cases.TestResult) error {
	outputDir := f.config.Reporting.OutputDir
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Save summary
	summaryPath := filepath.Join(outputDir, "summary.txt")
	if err := f.saveSummary(result, summaryPath); err != nil {
		return fmt.Errorf("failed to save summary: %w", err)
	}

	// Save detailed results for failed tests
	for _, testSuite := range result.TestSuites {
		for _, testCase := range testSuite.TestCases {
			if testCase.Status == cases.TestStatusFailed || testCase.Status == cases.TestStatusError {
				if err := f.saveTestCaseDetails(testCase, outputDir); err != nil {
					f.logger.Warn("Failed to save details for test case %s: %v", testCase.ID, err)
				}
			}
		}
	}

	return nil
}

// saveSummary saves a text summary of results
func (f *LSPTestFramework) saveSummary(result *cases.TestResult, path string) error {
	summary := fmt.Sprintf(`LSP Test Results Summary
========================

Test Execution: %v to %v (Duration: %v)

Overall Results:
- Total Test Suites: %d
- Total Test Cases:  %d
- Passed:           %d
- Failed:           %d
- Skipped:          %d
- Errors:           %d
- Pass Rate:        %.1f%%

Test Suites:
`,
		result.StartTime.Format("2006-01-02 15:04:05"),
		result.EndTime.Format("2006-01-02 15:04:05"),
		result.Duration,
		len(result.TestSuites),
		result.TotalCases,
		result.PassedCases,
		result.FailedCases,
		result.SkippedCases,
		result.ErrorCases,
		result.PassRate(),
	)

	for _, testSuite := range result.TestSuites {
		summary += fmt.Sprintf("- %s (%s): %d/%d passed (%v)\n",
			testSuite.Name,
			testSuite.Repository.Language,
			testSuite.PassedCases,
			testSuite.TotalCases,
			testSuite.Duration,
		)
	}

	return os.WriteFile(path, []byte(summary), 0644)
}

// saveTestCaseDetails saves detailed information for a test case
func (f *LSPTestFramework) saveTestCaseDetails(testCase *cases.TestCase, outputDir string) error {
	fileName := fmt.Sprintf("failed_%s.txt", testCase.ID)
	fileName = filepath.Join(outputDir, fileName)

	details := fmt.Sprintf(`Test Case Details: %s
=====================================

Name:        %s
Method:      %s
Language:    %s
File:        %s
Position:    line %d, character %d
Status:      %s
Duration:    %v

`,
		testCase.ID,
		testCase.Name,
		testCase.Method,
		testCase.Language,
		testCase.Config.File,
		testCase.Position.Line,
		testCase.Position.Character,
		testCase.Status.String(),
		testCase.Duration,
	)

	if testCase.Error != nil {
		details += fmt.Sprintf("Error: %s\n\n", testCase.Error.Error())
	}

	if testCase.Response != nil {
		details += fmt.Sprintf("Response:\n%s\n\n", string(testCase.Response))
	}

	if len(testCase.ValidationResults) > 0 {
		details += "Validation Results:\n"
		for _, result := range testCase.ValidationResults {
			status := "PASS"
			if !result.Passed {
				status = "FAIL"
			}
			details += fmt.Sprintf("- [%s] %s: %s\n", status, result.Description, result.Message)
		}
	}

	return os.WriteFile(fileName, []byte(details), 0644)
}

// GetConfiguration returns the test configuration
func (f *LSPTestFramework) GetConfiguration() *config.LSPTestConfig {
	return f.config
}

// GetSupportedMethods returns the list of supported LSP methods
func (f *LSPTestFramework) GetSupportedMethods() []string {
	return cases.SupportedLSPMethods()
}

// CreateSampleConfig creates a sample configuration file
func CreateSampleConfig(configPath string) error {
	testConfig := config.DefaultLSPTestConfig()

	// Add sample server configurations
	testConfig.Servers = config.DefaultServerConfigs()

	// Add sample repository
	sampleRepo := &config.RepositoryConfig{
		Name:        "sample-go-project",
		Path:        "./test-data",
		Language:    "go",
		Description: "Sample Go project for testing",
		TestCases: []*config.TestCaseConfig{
			{
				ID:          "go_definition_1",
				Name:        "Go function definition",
				Description: "Test textDocument/definition on a Go function",
				Method:      cases.LSPMethodDefinition,
				File:        "sample.go",
				Position:    &config.Position{Line: 14, Character: 9}, // TestFunction
				Expected: &config.ExpectedResult{
					Success: true,
					Definition: &config.DefinitionExpected{
						HasLocation: true,
					},
				},
			},
			{
				ID:          "go_hover_1",
				Name:        "Go function hover",
				Description: "Test textDocument/hover on a Go function",
				Method:      cases.LSPMethodHover,
				File:        "sample.go",
				Position:    &config.Position{Line: 14, Character: 9}, // TestFunction
				Expected: &config.ExpectedResult{
					Success: true,
					Hover: &config.HoverExpected{
						HasContent: true,
						Contains:   []string{"func", "TestFunction"},
					},
				},
			},
		},
	}

	testConfig.Repositories = []*config.RepositoryConfig{sampleRepo}

	return testConfig.SaveConfig(configPath)
}

// Quick runner function for simple use cases
func RunLSPTests(ctx context.Context, options *FrameworkOptions) (*cases.TestResult, error) {
	framework, err := NewLSPTestFramework(options)
	if err != nil {
		return nil, err
	}

	runOptions := runner.DefaultRunOptions()
	if options.DryRun {
		runOptions.DryRun = true
	}
	if options.Filter != nil {
		runOptions.Filter = options.Filter
	}

	return framework.Run(ctx, runOptions)
}

// loggerAdapter adapts between lsp.TestLogger and runner.TestLogger interfaces
type loggerAdapter struct {
	logger TestLogger
}

func (a *loggerAdapter) Debug(format string, args ...interface{}) {
	a.logger.Debug(format, args...)
}

func (a *loggerAdapter) Info(format string, args ...interface{}) {
	a.logger.Info(format, args...)
}

func (a *loggerAdapter) Warn(format string, args ...interface{}) {
	a.logger.Warn(format, args...)
}

func (a *loggerAdapter) Error(format string, args ...interface{}) {
	a.logger.Error(format, args...)
}

func (a *loggerAdapter) WithFields(fields map[string]interface{}) runner.TestLogger {
	adapted := a.logger.WithFields(fields)
	return &loggerAdapter{logger: adapted}
}

func (a *loggerAdapter) WithTestCase(testCase *cases.TestCase) runner.TestLogger {
	adapted := a.logger.WithTestCase(testCase)
	return &loggerAdapter{logger: adapted}
}

func (a *loggerAdapter) WithTestSuite(testSuite *cases.TestSuite) runner.TestLogger {
	adapted := a.logger.WithTestSuite(testSuite)
	return &loggerAdapter{logger: adapted}
}
