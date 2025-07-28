package testutils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LanguageTestScenario represents a test scenario for any programming language
type LanguageTestScenario struct {
	Name            string            // Test scenario name
	Language        string            // Programming language
	ConfigType      string            // Configuration type ("main" or "server")
	Port            string            // Server port
	ExpectedFiles   []string          // Expected files in the repository
	ExpectedSymbols []string          // Expected symbols to find
	TestMethods     []string          // LSP methods to test
	CustomConfig    map[string]string // Custom configuration variables
}

// LanguageConfigOptions provides configuration options for any language testing
type LanguageConfigOptions struct {
	TestPort        string            // Port for the LSP gateway server
	TestWorkspace   string            // Workspace directory managed by RepositoryManager
	ConfigType      string            // Configuration type ("main" or "server")
	Language        string            // Programming language
	CustomVariables map[string]string // Additional template substitution variables
}

// DefaultLanguageConfigOptions returns default configuration options for a given language
func DefaultLanguageConfigOptions(language string) LanguageConfigOptions {
	return LanguageConfigOptions{
		TestPort:        "8080",
		TestWorkspace:   fmt.Sprintf("/tmp/lspg-%s-e2e-tests", language),
		ConfigType:      "main",
		Language:        language,
		CustomVariables: make(map[string]string),
	}
}

// CreateLanguageConfig creates a temporary configuration file for any language testing
func CreateLanguageConfig(repoManager RepositoryManager, options LanguageConfigOptions) (string, func(), error) {
	if repoManager == nil {
		return "", nil, fmt.Errorf("RepositoryManager cannot be nil")
	}

	// Determine template name based on language and config type
	templateName := fmt.Sprintf("%s_config", options.Language)
	if options.ConfigType == "server" {
		templateName = fmt.Sprintf("%s_test_server", options.Language)
	}

	// Set up variables for template substitution
	variables := map[string]string{
		"TEST_PORT":      options.TestPort,
		"TEST_WORKSPACE": repoManager.GetWorkspaceDir(),
		"workspaceFolder": repoManager.GetWorkspaceDir(),
		"LANGUAGE":       options.Language,
	}

	// Add language-specific variables from repository manager
	if genericManager, ok := repoManager.(*GenericRepoManager); ok {
		langConfig := genericManager.config.LanguageConfig
		for key, value := range langConfig.CustomVariables {
			variables[key] = value
		}
	}

	// Add custom variables
	for key, value := range options.CustomVariables {
		variables[key] = value
	}

	// Create configuration using template processing
	configOptions := TempConfigOptions{
		Template:    templateName,
		Variables:   variables,
		FilePrefix:  fmt.Sprintf("%s_config_", options.Language),
		FileSuffix:  ".yaml",
	}

	configPath, cleanup, err := CreateTempConfigWithOptions(configOptions)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create %s config: %w", options.Language, err)
	}

	return configPath, cleanup, nil
}

// Language-specific configuration creation functions

// CreatePythonLanguageConfig creates Python configuration using the new modular system
func CreatePythonLanguageConfig(repoManager RepositoryManager) (string, func(), error) {
	options := DefaultLanguageConfigOptions("python")
	return CreateLanguageConfig(repoManager, options)
}

// CreateGoLanguageConfig creates Go configuration
func CreateGoLanguageConfig(repoManager RepositoryManager) (string, func(), error) {
	options := DefaultLanguageConfigOptions("go")
	return CreateLanguageConfig(repoManager, options)
}

// CreateJavaScriptLanguageConfig creates JavaScript/TypeScript configuration
func CreateJavaScriptLanguageConfig(repoManager RepositoryManager) (string, func(), error) {
	options := DefaultLanguageConfigOptions("javascript")
	return CreateLanguageConfig(repoManager, options)
}

// CreateJavaLanguageConfig creates Java configuration
func CreateJavaLanguageConfig(repoManager RepositoryManager) (string, func(), error) {
	options := DefaultLanguageConfigOptions("java")
	return CreateLanguageConfig(repoManager, options)
}

// CreateTypeScriptLanguageConfig creates TypeScript configuration
func CreateTypeScriptLanguageConfig(repoManager RepositoryManager) (string, func(), error) {
	options := DefaultLanguageConfigOptions("typescript")
	return CreateLanguageConfig(repoManager, options)
}

// SetupLanguageTestEnvironment sets up a complete test environment for any language
func SetupLanguageTestEnvironment(language string, scenario LanguageTestScenario) (RepositoryManager, string, func(), error) {
	var repoManager RepositoryManager
	var err error

	// Create appropriate repository manager based on language
	switch strings.ToLower(language) {
	case "python":
		repoManager = NewPythonRepositoryManager()
	case "go":
		repoManager = NewGoRepositoryManager()
	case "javascript":
		repoManager = NewJavaScriptRepositoryManager()
	case "typescript":
		repoManager = NewTypeScriptRepositoryManager()
	case "java":
		repoManager = NewJavaRepositoryManager()
	case "rust":
		repoManager = NewRustRepositoryManager()
	default:
		return nil, "", nil, fmt.Errorf("unsupported language: %s", language)
	}

	// Setup repository
	_, err = repoManager.SetupRepository()
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to setup %s repository: %w", language, err)
	}

	// Create configuration
	options := DefaultLanguageConfigOptions(language)
	options.ConfigType = scenario.ConfigType
	options.TestPort = scenario.Port
	
	// Add scenario-specific variables
	if options.CustomVariables == nil {
		options.CustomVariables = make(map[string]string)
	}
	options.CustomVariables["SCENARIO_NAME"] = scenario.Name
	options.CustomVariables["TEST_METHODS"] = strings.Join(scenario.TestMethods, ",")
	
	// Add custom config from scenario
	for key, value := range scenario.CustomConfig {
		options.CustomVariables[key] = value
	}

	configPath, configCleanup, err := CreateLanguageConfig(repoManager, options)
	if err != nil {
		repoManager.Cleanup()
		return nil, "", nil, fmt.Errorf("failed to create %s config: %w", language, err)
	}

	// Validate configuration
	if err := ValidateLanguageConfig(configPath, language); err != nil {
		configCleanup()
		repoManager.Cleanup()
		return nil, "", nil, fmt.Errorf("%s config validation failed: %w", language, err)
	}

	// Combined cleanup function
	cleanup := func() {
		configCleanup()
		if cleanupErr := repoManager.Cleanup(); cleanupErr != nil {
			fmt.Printf("Warning: failed to cleanup %s repository: %v\n", language, cleanupErr)
		}
	}

	return repoManager, configPath, cleanup, nil
}

// ValidateLanguageConfig validates that a configuration file contains expected elements for a language
func ValidateLanguageConfig(configPath, language string) error {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	configString := string(content)

	// Common required sections for all languages
	commonSections := []string{
		"port:",
		"servers:",
		"logging:",
	}

	// Language-specific requirements
	var languageRequirements []string
	switch strings.ToLower(language) {
	case "python":
		languageRequirements = []string{
			"language: \"python\"",
			"command: \"pylsp\"",
			"transport: \"stdio\"",
		}
	case "go":
		languageRequirements = []string{
			"language: \"go\"",
			"command: \"gopls\"",
			"transport: \"stdio\"",
		}
	case "javascript", "typescript":
		languageRequirements = []string{
			"transport: \"stdio\"",
		}
	case "java":
		languageRequirements = []string{
			"language: \"java\"",
			"transport: \"stdio\"",
		}
	case "rust":
		languageRequirements = []string{
			"language: \"rust\"",
			"command: \"rust-analyzer\"",
			"transport: \"stdio\"",
		}
	}

	// Check common sections
	for _, section := range commonSections {
		if !strings.Contains(configString, section) {
			return fmt.Errorf("missing required section: %s", section)
		}
	}

	// Check language-specific requirements
	for _, requirement := range languageRequirements {
		if !strings.Contains(configString, requirement) {
			return fmt.Errorf("missing %s requirement: %s", language, requirement)
		}
	}

	return nil
}

// ValidateLanguageTestEnvironment validates that the test environment is properly set up
func ValidateLanguageTestEnvironment(repoManager RepositoryManager, configPath string, scenario LanguageTestScenario) error {
	// Validate repository
	if err := repoManager.ValidateRepository(); err != nil {
		return fmt.Errorf("repository validation failed: %w", err)
	}

	// Validate workspace files
	testFiles, err := repoManager.GetTestFiles()
	if err != nil {
		return fmt.Errorf("failed to get test files: %w", err)
	}

	if len(testFiles) == 0 {
		return fmt.Errorf("no test files found in repository")
	}

	// Validate expected files exist
	workspaceDir := repoManager.GetWorkspaceDir()
	for _, expectedFile := range scenario.ExpectedFiles {
		// For directories, check they exist
		if strings.HasSuffix(expectedFile, "/") {
			expectedPath := filepath.Join(workspaceDir, expectedFile)
			if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
				return fmt.Errorf("expected directory not found: %s", expectedFile)
			}
			continue
		}

		// For files, check they exist and are readable
		expectedPath := filepath.Join(workspaceDir, expectedFile)
		if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
			return fmt.Errorf("expected file not found: %s", expectedFile)
		}
	}

	// Validate configuration
	if err := ValidateLanguageConfig(configPath, scenario.Language); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	return nil
}

// Predefined test scenarios for different languages

// GetPythonTestScenarios returns predefined Python test scenarios
func GetPythonTestScenarios() []LanguageTestScenario {
	return []LanguageTestScenario{
		{
			Name:     "python_basic_patterns",
			Language: "python",
			ConfigType: "main",
			Port:     "8080",
			ExpectedFiles: []string{
				"python-patterns/patterns/behavioral/observer.py",
				"python-patterns/patterns/creational/builder.py",
				"python-patterns/patterns/structural/adapter.py",
			},
			ExpectedSymbols: []string{"Observer", "Builder", "Adapter"},
			TestMethods: []string{
				"textDocument/definition",
				"textDocument/hover",
				"textDocument/completion",
			},
		},
	}
}

// GetGoTestScenarios returns predefined Go test scenarios
func GetGoTestScenarios() []LanguageTestScenario {
	return []LanguageTestScenario{
		{
			Name:     "go_basic_examples",
			Language: "go",
			ConfigType: "main", 
			Port:     "8081",
			ExpectedFiles: []string{
				"example/",
			},
			ExpectedSymbols: []string{"main", "func", "package"},
			TestMethods: []string{
				"textDocument/definition",
				"textDocument/references",
				"textDocument/hover",
			},
		},
	}
}

// GetJavaScriptTestScenarios returns predefined JavaScript test scenarios
func GetJavaScriptTestScenarios() []LanguageTestScenario {
	return []LanguageTestScenario{
		{
			Name:     "js_typescript_core",
			Language: "javascript",
			ConfigType: "main",
			Port:     "8082",
			ExpectedFiles: []string{
				"TypeScript/src/",
			},
			ExpectedSymbols: []string{"class", "function", "interface"},
			TestMethods: []string{
				"textDocument/definition",
				"textDocument/references",
				"textDocument/hover",
				"textDocument/completion",
			},
		},
	}
}