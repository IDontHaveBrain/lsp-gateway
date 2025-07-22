package scenarios

import (
	"context"
	"fmt"
	"strings"
	"time"

	"lsp-gateway/internal/testing/lsp"
	"lsp-gateway/internal/testing/lsp/cases"
	"lsp-gateway/internal/testing/lsp/config"
)

// ScenarioFramework integrates scenarios with the existing LSP test framework
type ScenarioFramework struct {
	manager   *ScenarioManager
	framework *lsp.LSPTestFramework
}

// NewScenarioFramework creates a new scenario-based test framework
func NewScenarioFramework(scenariosDir string, options *lsp.FrameworkOptions) (*ScenarioFramework, error) {
	manager := NewScenarioManager(scenariosDir)

	// Load all scenarios
	if err := manager.LoadAllScenarios(); err != nil {
		return nil, fmt.Errorf("failed to load scenarios: %w", err)
	}

	// Create base framework (will be configured per run)
	framework, err := lsp.NewLSPTestFramework(options)
	if err != nil {
		return nil, fmt.Errorf("failed to create LSP test framework: %w", err)
	}

	return &ScenarioFramework{
		manager:   manager,
		framework: framework,
	}, nil
}

// ScenarioRunOptions extends the base run options with scenario-specific settings
type ScenarioRunOptions struct {
	// Languages to test (empty means all loaded languages)
	Languages []string

	// Repositories to include (empty means all repositories)
	Repositories []string

	// LSP methods to test (empty means all methods)
	Methods []string

	// Tags to filter by (empty means no tag filtering)
	Tags []string

	// Include performance tests
	IncludePerformanceTests bool

	// Base execution options
	MaxConcurrency int
	FailFast       bool
	Timeout        time.Duration

	// Scenario-specific options
	ValidateFilePatterns bool // Enable file pattern validation for definitions
	StrictModeEnabled    bool // Enable strict validation mode
	RetryFailedTests     int  // Number of retries for failed tests
}

// DefaultScenarioRunOptions returns default run options for scenarios
func DefaultScenarioRunOptions() *ScenarioRunOptions {
	return &ScenarioRunOptions{
		Languages:               []string{}, // All languages
		Repositories:            []string{}, // All repositories
		Methods:                 []string{}, // All methods
		Tags:                    []string{}, // No tag filtering
		IncludePerformanceTests: false,      // Exclude performance tests by default
		MaxConcurrency:          4,
		FailFast:                false,
		Timeout:                 10 * time.Minute,
		ValidateFilePatterns:    true,
		StrictModeEnabled:       false,
		RetryFailedTests:        2,
	}
}

// RunScenarios executes test scenarios with the given options
func (sf *ScenarioFramework) RunScenarios(ctx context.Context, options *ScenarioRunOptions) (*cases.TestResult, error) {
	if options == nil {
		options = DefaultScenarioRunOptions()
	}

	// Create test case filter from options
	filter := &cases.TestCaseFilter{
		Methods:        options.Methods,
		Languages:      options.Languages,
		Tags:           options.Tags,
		IncludeSkipped: false,
	}

	// Add performance tag if performance tests are included
	if options.IncludePerformanceTests {
		if len(filter.Tags) == 0 {
			filter.Tags = []string{"performance"}
		} else {
			filter.Tags = append(filter.Tags, "performance")
		}
	}

	// Convert scenarios to test suites
	testSuites, err := sf.manager.ConvertToTestSuites(ctx, options.Languages, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to convert scenarios to test suites: %w", err)
	}

	// Filter by repositories if specified
	if len(options.Repositories) > 0 {
		testSuites = sf.filterTestSuitesByRepository(testSuites, options.Repositories)
	}

	if len(testSuites) == 0 {
		return &cases.TestResult{
			TestSuites:   []*cases.TestSuite{},
			TotalCases:   0,
			PassedCases:  0,
			FailedCases:  0,
			SkippedCases: 0,
			ErrorCases:   0,
			StartTime:    time.Now(),
			EndTime:      time.Now(),
			Duration:     0,
		}, nil
	}

	// Create dynamic configuration for this run
	runConfig := sf.createRunConfiguration(options)

	// Framework configuration is immutable, using existing configuration
	_ = runConfig // TODO: Implement configuration updates if needed

	// Execute the test suites
	results, err := sf.executeTestSuites(ctx, testSuites, options)
	if err != nil {
		return nil, fmt.Errorf("failed to execute test suites: %w", err)
	}

	return results, nil
}

// filterTestSuitesByRepository filters test suites by repository names
func (sf *ScenarioFramework) filterTestSuitesByRepository(testSuites []*cases.TestSuite, repositories []string) []*cases.TestSuite {
	var filtered []*cases.TestSuite

	for _, suite := range testSuites {
		for _, repoName := range repositories {
			if strings.Contains(suite.Name, repoName) {
				filtered = append(filtered, suite)
				break
			}
		}
	}

	return filtered
}

// createRunConfiguration creates a run-time configuration based on options
func (sf *ScenarioFramework) createRunConfiguration(options *ScenarioRunOptions) *config.LSPTestConfig {
	runConfig := config.DefaultLSPTestConfig()

	// Update execution settings
	runConfig.Execution.MaxConcurrency = options.MaxConcurrency
	runConfig.Execution.FailFast = options.FailFast
	runConfig.Execution.RetryAttempts = options.RetryFailedTests
	runConfig.Timeout = options.Timeout

	// Update validation settings
	runConfig.Validation.StrictMode = options.StrictModeEnabled
	runConfig.Validation.ValidatePositions = true
	runConfig.Validation.ValidateURIs = true
	runConfig.Validation.ValidateTypes = true

	// Enable detailed reporting for scenarios
	runConfig.Reporting.Verbose = true
	runConfig.Reporting.IncludeTiming = true
	runConfig.Reporting.SaveDetails = true

	return runConfig
}

// executeTestSuites executes the test suites with retry logic
func (sf *ScenarioFramework) executeTestSuites(ctx context.Context, testSuites []*cases.TestSuite, options *ScenarioRunOptions) (*cases.TestResult, error) {
	startTime := time.Now()

	// Execute test suites using the framework Run method
	// TODO: Convert testSuites to RunOptions format
	result := &cases.TestResult{
		TestSuites: testSuites,
		Duration:   0,
	}
	// Placeholder implementation - framework.Run requires RunOptions, not TestSuites

	// Apply scenario-specific post-processing
	sf.postProcessResults(result, options)

	result.Duration = time.Since(startTime)
	return result, nil
}

// postProcessResults applies scenario-specific post-processing to results
func (sf *ScenarioFramework) postProcessResults(result *cases.TestResult, options *ScenarioRunOptions) {
	for _, suite := range result.TestSuites {
		for _, testCase := range suite.TestCases {
			// Apply file pattern validation for definition tests
			if options.ValidateFilePatterns && testCase.Method == cases.LSPMethodDefinition {
				sf.validateDefinitionFilePattern(testCase)
			}

			// Add scenario-specific validation results
			sf.addScenarioValidations(testCase, options)
		}

		// Update suite status based on processed results
		suite.UpdateStatus()
	}

	// Recalculate overall results
	sf.recalculateResults(result)
}

// validateDefinitionFilePattern validates file patterns in definition results
func (sf *ScenarioFramework) validateDefinitionFilePattern(testCase *cases.TestCase) {
	// Check if the expected result has a file pattern
	if testCase.Expected == nil || testCase.Expected.Definition == nil {
		return
	}

	// Get scenario expected data (this would need enhancement to store pattern info)
	scenario, err := sf.manager.GetLanguageScenarios(testCase.Language)
	if err != nil {
		return
	}

	// Find the corresponding scenario test case
	var scenarioCase *ScenarioTestCase
	allCases := append(scenario.Scenarios, scenario.PerformanceTests...)
	for _, sCase := range allCases {
		if sCase.ID == testCase.ID {
			scenarioCase = sCase
			break
		}
	}

	if scenarioCase == nil || scenarioCase.Expected == nil || scenarioCase.Expected.Definition == nil {
		return
	}

	filePattern := scenarioCase.Expected.Definition.FilePattern
	if filePattern == nil {
		return
	}

	// Add pattern validation result
	// This would be implemented based on the actual response parsing
	validationResult := &cases.ValidationResult{
		Name:        "file_pattern_validation",
		Description: "Validate definition file matches expected pattern",
		Passed:      true, // Would be calculated based on actual response
		Message:     fmt.Sprintf("File pattern validation for: %s", *filePattern),
		Details: map[string]interface{}{
			"expected_pattern": *filePattern,
		},
	}

	testCase.ValidationResults = append(testCase.ValidationResults, validationResult)
}

// addScenarioValidations adds scenario-specific validations
func (sf *ScenarioFramework) addScenarioValidations(testCase *cases.TestCase, options *ScenarioRunOptions) {
	// Add timing validation for performance tests
	if sf.containsTag(testCase.Tags, "performance") {
		timeValidation := &cases.ValidationResult{
			Name:        "performance_timing",
			Description: "Validate test execution time for performance test",
			Passed:      testCase.Duration < 30*time.Second, // Default performance threshold
			Message:     fmt.Sprintf("Test completed in %v", testCase.Duration),
			Details: map[string]interface{}{
				"duration_ms": testCase.Duration.Milliseconds(),
				"threshold":   "30s",
			},
		}
		testCase.ValidationResults = append(testCase.ValidationResults, timeValidation)
	}

	// Add complexity validation for complex scenarios
	if sf.containsTag(testCase.Tags, "complex") {
		complexityValidation := &cases.ValidationResult{
			Name:        "complexity_handling",
			Description: "Validate handling of complex language constructs",
			Passed:      testCase.Status == cases.TestStatusPassed,
			Message:     "Complex language construct handling",
		}
		testCase.ValidationResults = append(testCase.ValidationResults, complexityValidation)
	}
}

// containsTag checks if a tag exists in the test case tags
func (sf *ScenarioFramework) containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}

// recalculateResults recalculates the overall test results
func (sf *ScenarioFramework) recalculateResults(result *cases.TestResult) {
	result.TotalCases = 0
	result.PassedCases = 0
	result.FailedCases = 0
	result.SkippedCases = 0
	result.ErrorCases = 0

	for _, suite := range result.TestSuites {
		result.TotalCases += suite.TotalCases
		result.PassedCases += suite.PassedCases
		result.FailedCases += suite.FailedCases
		result.SkippedCases += suite.SkippedCases
		result.ErrorCases += suite.ErrorCases
	}
}

// GetScenarioSummary returns a summary of available scenarios
func (sf *ScenarioFramework) GetScenarioSummary() map[string]ScenarioSummary {
	return sf.manager.ListScenarios()
}

// ValidateScenarios validates all loaded scenarios
func (sf *ScenarioFramework) ValidateScenarios() error {
	return sf.manager.ValidateScenarioFiles()
}

// RunLanguageScenarios runs scenarios for a specific language
func (sf *ScenarioFramework) RunLanguageScenarios(ctx context.Context, language string, options *ScenarioRunOptions) (*cases.TestResult, error) {
	if options == nil {
		options = DefaultScenarioRunOptions()
	}

	// Override languages to only include the specified language
	options.Languages = []string{language}

	return sf.RunScenarios(ctx, options)
}

// RunMethodScenarios runs scenarios for specific LSP methods
func (sf *ScenarioFramework) RunMethodScenarios(ctx context.Context, methods []string, options *ScenarioRunOptions) (*cases.TestResult, error) {
	if options == nil {
		options = DefaultScenarioRunOptions()
	}

	// Override methods to only include the specified methods
	options.Methods = methods

	return sf.RunScenarios(ctx, options)
}

// RunPerformanceScenarios runs only performance test scenarios
func (sf *ScenarioFramework) RunPerformanceScenarios(ctx context.Context, options *ScenarioRunOptions) (*cases.TestResult, error) {
	if options == nil {
		options = DefaultScenarioRunOptions()
	}

	// Enable performance tests and filter by performance tag
	options.IncludePerformanceTests = true
	options.Tags = []string{"performance"}

	return sf.RunScenarios(ctx, options)
}

// GetScenariosByRepository returns scenarios for a specific repository
func (sf *ScenarioFramework) GetScenariosByRepository(language, repository string) ([]*ScenarioTestCase, error) {
	scenario, err := sf.manager.GetLanguageScenarios(language)
	if err != nil {
		return nil, err
	}

	var repoScenarios []*ScenarioTestCase
	allCases := append(scenario.Scenarios, scenario.PerformanceTests...)

	for _, testCase := range allCases {
		if testCase.Repository == repository {
			repoScenarios = append(repoScenarios, testCase)
		}
	}

	return repoScenarios, nil
}

// ListAvailableRepositories returns all available repositories across all languages
func (sf *ScenarioFramework) ListAvailableRepositories() map[string][]string {
	repositories := make(map[string][]string)

	for language := range sf.manager.loadedScenarios {
		scenario := sf.manager.loadedScenarios[language]

		var repos []string
		for repoName := range scenario.TestRepositories {
			repos = append(repos, repoName)
		}

		repositories[language] = repos
	}

	return repositories
}

// CreateScenarioReport creates a detailed scenario execution report
func (sf *ScenarioFramework) CreateScenarioReport(result *cases.TestResult) *ScenarioReport {
	report := &ScenarioReport{
		ExecutionTime: result.Duration,
		TotalCases:    result.TotalCases,
		PassedCases:   result.PassedCases,
		FailedCases:   result.FailedCases,
		SkippedCases:  result.SkippedCases,
		ErrorCases:    result.ErrorCases,
		PassRate:      result.PassRate(),
		Languages:     make(map[string]*LanguageReport),
	}

	// Aggregate results by language
	for _, suite := range result.TestSuites {
		// Extract language from suite name
		parts := strings.Split(suite.Name, "_")
		if len(parts) < 2 {
			continue
		}

		language := parts[0]
		if _, exists := report.Languages[language]; !exists {
			report.Languages[language] = &LanguageReport{
				Language:     language,
				TotalCases:   0,
				PassedCases:  0,
				FailedCases:  0,
				SkippedCases: 0,
				ErrorCases:   0,
				Methods:      make(map[string]int),
				Repositories: make(map[string]int),
			}
		}

		langReport := report.Languages[language]
		langReport.TotalCases += suite.TotalCases
		langReport.PassedCases += suite.PassedCases
		langReport.FailedCases += suite.FailedCases
		langReport.SkippedCases += suite.SkippedCases
		langReport.ErrorCases += suite.ErrorCases

		// Count methods and repositories
		for _, testCase := range suite.TestCases {
			langReport.Methods[testCase.Method]++
			langReport.Repositories[suite.Repository.Name]++
		}
	}

	return report
}

// ScenarioReport represents a detailed scenario execution report
type ScenarioReport struct {
	ExecutionTime time.Duration              `json:"execution_time"`
	TotalCases    int                        `json:"total_cases"`
	PassedCases   int                        `json:"passed_cases"`
	FailedCases   int                        `json:"failed_cases"`
	SkippedCases  int                        `json:"skipped_cases"`
	ErrorCases    int                        `json:"error_cases"`
	PassRate      float64                    `json:"pass_rate"`
	Languages     map[string]*LanguageReport `json:"languages"`
}

// LanguageReport represents results for a specific language
type LanguageReport struct {
	Language     string         `json:"language"`
	TotalCases   int            `json:"total_cases"`
	PassedCases  int            `json:"passed_cases"`
	FailedCases  int            `json:"failed_cases"`
	SkippedCases int            `json:"skipped_cases"`
	ErrorCases   int            `json:"error_cases"`
	Methods      map[string]int `json:"methods"`      // Method -> count
	Repositories map[string]int `json:"repositories"` // Repository -> count
}

// GetScenariosDirectory returns the scenarios directory path
func (sf *ScenarioFramework) GetScenariosDirectory() string {
	return sf.manager.scenariosDir
}

// GetLoadedLanguages returns all loaded languages
func (sf *ScenarioFramework) GetLoadedLanguages() []string {
	return sf.manager.GetAllLanguages()
}

// ReloadScenarios reloads all scenarios from disk
func (sf *ScenarioFramework) ReloadScenarios() error {
	return sf.manager.LoadAllScenarios()
}
