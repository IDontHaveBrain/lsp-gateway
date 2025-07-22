package lsp

import (
	"fmt"
	"os"

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
