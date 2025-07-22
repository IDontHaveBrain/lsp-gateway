package scenarios

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"lsp-gateway/internal/testing/lsp/cases"
	"lsp-gateway/internal/testing/lsp/config"
)

// ScenarioManager manages language-specific test scenarios
type ScenarioManager struct {
	scenariosDir    string
	loadedScenarios map[string]*LanguageScenario
}

// LanguageScenario represents a complete language scenario configuration
type LanguageScenario struct {
	Name        string              `yaml:"name"`
	Description string              `yaml:"description"`
	Language    string              `yaml:"language"`
	Scenarios   []*ScenarioTestCase `yaml:"scenarios"`

	// Performance tests with specific timeout requirements
	PerformanceTests []*ScenarioTestCase `yaml:"performance_tests"`

	// Test repository configurations specific to this language
	TestRepositories map[string]*ScenarioRepository `yaml:"test_repositories"`
}

// ScenarioTestCase represents a single test scenario
type ScenarioTestCase struct {
	ID          string                 `yaml:"id"`
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Method      string                 `yaml:"method"`
	Repository  string                 `yaml:"repository"`
	File        string                 `yaml:"file"`
	Position    *config.Position       `yaml:"position"`
	Query       string                 `yaml:"query,omitempty"` // For workspace/symbol searches
	Params      map[string]interface{} `yaml:"params,omitempty"`
	Expected    *ScenarioExpected      `yaml:"expected"`
	Tags        []string               `yaml:"tags"`
	Timeout     time.Duration          `yaml:"timeout,omitempty"`
	Skip        bool                   `yaml:"skip,omitempty"`
	SkipReason  string                 `yaml:"skip_reason,omitempty"`
}

// ScenarioExpected represents expected results for scenario test cases
type ScenarioExpected struct {
	Success   bool     `yaml:"success"`
	ErrorCode *int     `yaml:"error_code,omitempty"`
	ErrorMsg  *string  `yaml:"error_msg,omitempty"`
	Contains  []string `yaml:"contains,omitempty"`
	Excludes  []string `yaml:"excludes,omitempty"`

	// Method-specific expectations with additional pattern matching
	Definition *ScenarioDefinitionExpected `yaml:"definition,omitempty"`
	References *ScenarioReferencesExpected `yaml:"references,omitempty"`
	Hover      *ScenarioHoverExpected      `yaml:"hover,omitempty"`
	Symbols    *ScenarioSymbolsExpected    `yaml:"symbols,omitempty"`
}

// ScenarioDefinitionExpected extends the base definition expected with patterns
type ScenarioDefinitionExpected struct {
	HasLocation bool          `yaml:"has_location"`
	FileURI     *string       `yaml:"file_uri,omitempty"`
	FilePattern *string       `yaml:"file_pattern,omitempty"` // Pattern matching for files
	Range       *config.Range `yaml:"range,omitempty"`
}

// ScenarioReferencesExpected extends the base references expected
type ScenarioReferencesExpected struct {
	MinCount           int  `yaml:"min_count"`
	MaxCount           int  `yaml:"max_count,omitempty"`
	IncludeDeclaration bool `yaml:"include_declaration"`
}

// ScenarioHoverExpected extends the base hover expected
type ScenarioHoverExpected struct {
	HasContent bool     `yaml:"has_content"`
	Contains   []string `yaml:"contains,omitempty"`
	Format     string   `yaml:"format,omitempty"`
}

// ScenarioSymbolsExpected extends the base symbols expected
type ScenarioSymbolsExpected struct {
	MinCount int      `yaml:"min_count"`
	MaxCount int      `yaml:"max_count,omitempty"`
	Types    []string `yaml:"types,omitempty"`
}

// ScenarioRepository represents repository configuration for scenarios
type ScenarioRepository struct {
	Path  string                   `yaml:"path"`
	Setup *ScenarioRepositorySetup `yaml:"setup,omitempty"`
}

// ScenarioRepositorySetup represents setup configuration for scenario repositories
type ScenarioRepositorySetup struct {
	Commands []string          `yaml:"commands"`
	Timeout  time.Duration     `yaml:"timeout"`
	Env      map[string]string `yaml:"env,omitempty"`
}

// NewScenarioManager creates a new scenario manager
func NewScenarioManager(scenariosDir string) *ScenarioManager {
	return &ScenarioManager{
		scenariosDir:    scenariosDir,
		loadedScenarios: make(map[string]*LanguageScenario),
	}
}

// LoadAllScenarios loads all language scenarios from the scenarios directory
func (sm *ScenarioManager) LoadAllScenarios() error {
	languages := []string{"go", "python", "typescript", "java"}

	for _, lang := range languages {
		if err := sm.LoadLanguageScenarios(lang); err != nil {
			return fmt.Errorf("failed to load %s scenarios: %w", lang, err)
		}
	}

	return nil
}

// LoadLanguageScenarios loads scenarios for a specific language
func (sm *ScenarioManager) LoadLanguageScenarios(language string) error {
	scenarioFile := filepath.Join(sm.scenariosDir, fmt.Sprintf("%s_scenarios.yaml", language))

	data, err := os.ReadFile(scenarioFile)
	if err != nil {
		return fmt.Errorf("failed to read scenario file %s: %w", scenarioFile, err)
	}

	var scenario LanguageScenario
	if err := yaml.Unmarshal(data, &scenario); err != nil {
		return fmt.Errorf("failed to parse scenario file %s: %w", scenarioFile, err)
	}

	// Validate the scenario
	if err := sm.validateLanguageScenario(&scenario); err != nil {
		return fmt.Errorf("scenario validation failed for %s: %w", language, err)
	}

	sm.loadedScenarios[language] = &scenario
	return nil
}

// validateLanguageScenario validates a language scenario configuration
func (sm *ScenarioManager) validateLanguageScenario(scenario *LanguageScenario) error {
	if scenario.Name == "" {
		return fmt.Errorf("scenario name is required")
	}

	if scenario.Language == "" {
		return fmt.Errorf("scenario language is required")
	}

	if len(scenario.Scenarios) == 0 {
		return fmt.Errorf("at least one test scenario is required")
	}

	// Validate each test case
	for i, testCase := range scenario.Scenarios {
		if err := sm.validateScenarioTestCase(testCase); err != nil {
			return fmt.Errorf("scenario %d validation failed: %w", i, err)
		}
	}

	// Validate performance tests
	for i, testCase := range scenario.PerformanceTests {
		if err := sm.validateScenarioTestCase(testCase); err != nil {
			return fmt.Errorf("performance test %d validation failed: %w", i, err)
		}
	}

	return nil
}

// validateScenarioTestCase validates a single scenario test case
func (sm *ScenarioManager) validateScenarioTestCase(testCase *ScenarioTestCase) error {
	if testCase.ID == "" {
		return fmt.Errorf("test case ID is required")
	}

	if testCase.Name == "" {
		return fmt.Errorf("test case name is required")
	}

	if testCase.Method == "" {
		return fmt.Errorf("test case method is required")
	}

	// Validate method is supported
	if !cases.IsMethodSupported(testCase.Method) {
		return fmt.Errorf("unsupported LSP method: %s", testCase.Method)
	}

	if testCase.Repository == "" {
		return fmt.Errorf("test case repository is required")
	}

	// For workspace/symbol, query is required instead of file/position
	if testCase.Method == cases.LSPMethodWorkspaceSymbol {
		if testCase.Query == "" {
			return fmt.Errorf("workspace symbol test case requires query")
		}
	} else {
		if testCase.File == "" {
			return fmt.Errorf("test case file is required for method %s", testCase.Method)
		}
		if testCase.Position == nil {
			return fmt.Errorf("test case position is required for method %s", testCase.Method)
		}
	}

	return nil
}

// GetLanguageScenarios returns scenarios for a specific language
func (sm *ScenarioManager) GetLanguageScenarios(language string) (*LanguageScenario, error) {
	scenario, exists := sm.loadedScenarios[language]
	if !exists {
		return nil, fmt.Errorf("no scenarios loaded for language: %s", language)
	}
	return scenario, nil
}

// GetAllLanguages returns all loaded languages
func (sm *ScenarioManager) GetAllLanguages() []string {
	var languages []string
	for lang := range sm.loadedScenarios {
		languages = append(languages, lang)
	}
	return languages
}

// ConvertToTestSuites converts scenarios to test suites that can be executed by the framework
func (sm *ScenarioManager) ConvertToTestSuites(ctx context.Context, languages []string, filter *cases.TestCaseFilter) ([]*cases.TestSuite, error) {
	var testSuites []*cases.TestSuite

	// If no languages specified, use all loaded languages
	if len(languages) == 0 {
		languages = sm.GetAllLanguages()
	}

	for _, language := range languages {
		scenario, err := sm.GetLanguageScenarios(language)
		if err != nil {
			return nil, fmt.Errorf("failed to get scenarios for %s: %w", language, err)
		}

		// Create repository configurations from scenario
		repos, err := sm.createRepositoryConfigs(scenario)
		if err != nil {
			return nil, fmt.Errorf("failed to create repository configs for %s: %w", language, err)
		}

		// Create test suites for each repository
		for repoName, repo := range repos {
			suite := cases.NewTestSuite(
				fmt.Sprintf("%s_%s", language, repoName),
				repo,
				getServerConfigForLanguage(language),
			)

			// Convert scenario test cases to framework test cases
			testCases, err := sm.convertScenarioTestCases(scenario.Scenarios, repo, filter)
			if err != nil {
				return nil, fmt.Errorf("failed to convert test cases for %s/%s: %w", language, repoName, err)
			}

			for _, testCase := range testCases {
				suite.AddTestCase(testCase)
			}

			// Add performance tests if requested
			if filter == nil || containsTag(filter.Tags, "performance") {
				perfTestCases, err := sm.convertScenarioTestCases(scenario.PerformanceTests, repo, filter)
				if err != nil {
					return nil, fmt.Errorf("failed to convert performance tests for %s/%s: %w", language, repoName, err)
				}

				for _, testCase := range perfTestCases {
					suite.AddTestCase(testCase)
				}
			}

			testSuites = append(testSuites, suite)
		}
	}

	return testSuites, nil
}

// createRepositoryConfigs creates repository configurations from scenario
func (sm *ScenarioManager) createRepositoryConfigs(scenario *LanguageScenario) (map[string]*config.RepositoryConfig, error) {
	repos := make(map[string]*config.RepositoryConfig)

	for repoName, scenarioRepo := range scenario.TestRepositories {
		repo := &config.RepositoryConfig{
			Name:        repoName,
			Path:        scenarioRepo.Path,
			Language:    scenario.Language,
			Description: fmt.Sprintf("%s repository for %s testing", repoName, scenario.Language),
		}

		// Convert scenario setup to framework setup
		if scenarioRepo.Setup != nil {
			repo.Setup = &config.RepositorySetup{
				Commands: scenarioRepo.Setup.Commands,
				Timeout:  scenarioRepo.Setup.Timeout,
				Env:      scenarioRepo.Setup.Env,
			}
		}

		repos[repoName] = repo
	}

	return repos, nil
}

// convertScenarioTestCases converts scenario test cases to framework test cases
func (sm *ScenarioManager) convertScenarioTestCases(scenarioTestCases []*ScenarioTestCase, repo *config.RepositoryConfig, filter *cases.TestCaseFilter) ([]*cases.TestCase, error) {
	var testCases []*cases.TestCase

	for _, scenarioCase := range scenarioTestCases {
		// Skip if the test case doesn't match the repository
		if scenarioCase.Repository != repo.Name {
			continue
		}

		// Convert to framework test case config
		testCaseConfig := &config.TestCaseConfig{
			ID:          scenarioCase.ID,
			Name:        scenarioCase.Name,
			Description: scenarioCase.Description,
			Method:      scenarioCase.Method,
			File:        scenarioCase.File,
			Position:    scenarioCase.Position,
			Params:      scenarioCase.Params,
			Tags:        scenarioCase.Tags,
			Timeout:     scenarioCase.Timeout,
			Skip:        scenarioCase.Skip,
			SkipReason:  scenarioCase.SkipReason,
			Expected:    sm.convertScenarioExpected(scenarioCase.Expected),
		}

		// Add workspace symbol query to params
		if scenarioCase.Method == cases.LSPMethodWorkspaceSymbol && scenarioCase.Query != "" {
			if testCaseConfig.Params == nil {
				testCaseConfig.Params = make(map[string]interface{})
			}
			testCaseConfig.Params["query"] = scenarioCase.Query
		}

		// Set default timeout if not specified
		if testCaseConfig.Timeout == 0 {
			testCaseConfig.Timeout = 30 * time.Second
		}

		// Create framework test case
		testCase := cases.NewTestCase(scenarioCase.ID, repo, testCaseConfig)

		// Apply filter if specified
		if filter != nil && !filter.Matches(testCase) {
			continue
		}

		testCases = append(testCases, testCase)
	}

	return testCases, nil
}

// convertScenarioExpected converts scenario expected results to framework expected results
func (sm *ScenarioManager) convertScenarioExpected(scenarioExpected *ScenarioExpected) *config.ExpectedResult {
	if scenarioExpected == nil {
		return nil
	}

	expected := &config.ExpectedResult{
		Success:  scenarioExpected.Success,
		Contains: scenarioExpected.Contains,
		Excludes: scenarioExpected.Excludes,
	}

	if scenarioExpected.ErrorCode != nil {
		expected.ErrorCode = scenarioExpected.ErrorCode
	}

	if scenarioExpected.ErrorMsg != nil {
		expected.ErrorMsg = scenarioExpected.ErrorMsg
	}

	// Convert method-specific expectations
	if scenarioExpected.Definition != nil {
		expected.Definition = &config.DefinitionExpected{
			HasLocation: scenarioExpected.Definition.HasLocation,
			FileURI:     scenarioExpected.Definition.FileURI,
			Range:       scenarioExpected.Definition.Range,
		}
	}

	if scenarioExpected.References != nil {
		expected.References = &config.ReferencesExpected{
			MinCount:    scenarioExpected.References.MinCount,
			MaxCount:    scenarioExpected.References.MaxCount,
			IncludeDecl: scenarioExpected.References.IncludeDeclaration,
		}
	}

	if scenarioExpected.Hover != nil {
		expected.Hover = &config.HoverExpected{
			HasContent: scenarioExpected.Hover.HasContent,
			Contains:   scenarioExpected.Hover.Contains,
			Format:     scenarioExpected.Hover.Format,
		}
	}

	if scenarioExpected.Symbols != nil {
		expected.Symbols = &config.SymbolsExpected{
			MinCount: scenarioExpected.Symbols.MinCount,
			MaxCount: scenarioExpected.Symbols.MaxCount,
			Types:    scenarioExpected.Symbols.Types,
		}
	}

	return expected
}

// getServerConfigForLanguage returns default server configuration for a language
func getServerConfigForLanguage(language string) *config.ServerConfig {
	serverConfigs := config.DefaultServerConfigs()

	switch language {
	case "go":
		return serverConfigs["gopls"]
	case "python":
		return serverConfigs["pylsp"]
	case "typescript":
		return serverConfigs["tsserver"]
	case "java":
		return serverConfigs["jdtls"]
	default:
		// Return a generic server config
		return &config.ServerConfig{
			Name:            fmt.Sprintf("%s-lsp", language),
			Language:        language,
			Command:         fmt.Sprintf("%s-language-server", language),
			Transport:       "stdio",
			StartTimeout:    15 * time.Second,
			ShutdownTimeout: 5 * time.Second,
			PreWarmup:       true,
			InitializeDelay: 1 * time.Second,
		}
	}
}

// containsTag checks if a tag is in the list of tags
func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}

// ListScenarios returns a summary of all loaded scenarios
func (sm *ScenarioManager) ListScenarios() map[string]ScenarioSummary {
	summaries := make(map[string]ScenarioSummary)

	for language, scenario := range sm.loadedScenarios {
		summary := ScenarioSummary{
			Language:         language,
			Name:             scenario.Name,
			Description:      scenario.Description,
			TestCaseCount:    len(scenario.Scenarios),
			PerformanceTests: len(scenario.PerformanceTests),
			RepositoryCount:  len(scenario.TestRepositories),
			Methods:          make(map[string]int),
			Tags:             make(map[string]int),
		}

		// Count methods and tags
		allTestCases := append(scenario.Scenarios, scenario.PerformanceTests...)
		for _, testCase := range allTestCases {
			summary.Methods[testCase.Method]++
			for _, tag := range testCase.Tags {
				summary.Tags[tag]++
			}
		}

		summaries[language] = summary
	}

	return summaries
}

// ScenarioSummary provides a summary of scenarios for a language
type ScenarioSummary struct {
	Language         string         `json:"language"`
	Name             string         `json:"name"`
	Description      string         `json:"description"`
	TestCaseCount    int            `json:"test_case_count"`
	PerformanceTests int            `json:"performance_tests"`
	RepositoryCount  int            `json:"repository_count"`
	Methods          map[string]int `json:"methods"` // Method name -> count
	Tags             map[string]int `json:"tags"`    // Tag name -> count
}

// GetScenariosByTags returns test cases that match specific tags
func (sm *ScenarioManager) GetScenariosByTags(language string, tags []string) ([]*ScenarioTestCase, error) {
	scenario, err := sm.GetLanguageScenarios(language)
	if err != nil {
		return nil, err
	}

	var matchingCases []*ScenarioTestCase

	allTestCases := append(scenario.Scenarios, scenario.PerformanceTests...)
	for _, testCase := range allTestCases {
		if sm.hasAnyTag(testCase.Tags, tags) {
			matchingCases = append(matchingCases, testCase)
		}
	}

	return matchingCases, nil
}

// hasAnyTag checks if any of the required tags match the test case tags
func (sm *ScenarioManager) hasAnyTag(testCaseTags, requiredTags []string) bool {
	for _, required := range requiredTags {
		for _, testTag := range testCaseTags {
			if strings.EqualFold(testTag, required) {
				return true
			}
		}
	}
	return false
}

// ValidateScenarioFiles validates all scenario files without loading them
func (sm *ScenarioManager) ValidateScenarioFiles() error {
	languages := []string{"go", "python", "typescript", "java"}

	for _, lang := range languages {
		scenarioFile := filepath.Join(sm.scenariosDir, fmt.Sprintf("%s_scenarios.yaml", lang))

		if _, err := os.Stat(scenarioFile); os.IsNotExist(err) {
			return fmt.Errorf("scenario file missing for language %s: %s", lang, scenarioFile)
		}

		data, err := os.ReadFile(scenarioFile)
		if err != nil {
			return fmt.Errorf("failed to read scenario file %s: %w", scenarioFile, err)
		}

		var scenario LanguageScenario
		if err := yaml.Unmarshal(data, &scenario); err != nil {
			return fmt.Errorf("failed to parse scenario file %s: %w", scenarioFile, err)
		}

		if err := sm.validateLanguageScenario(&scenario); err != nil {
			return fmt.Errorf("scenario validation failed for %s: %w", lang, err)
		}
	}

	return nil
}
