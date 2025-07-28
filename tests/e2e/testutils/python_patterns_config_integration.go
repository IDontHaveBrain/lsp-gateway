package testutils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PythonPatternsConfigOptions provides configuration options for Python patterns testing
type PythonPatternsConfigOptions struct {
	// TestPort for the LSP gateway server
	TestPort string
	// TestWorkspace is the workspace directory managed by PythonRepoManager
	TestWorkspace string
	// ConfigType specifies which configuration to use ("main" or "server")
	ConfigType string
	// CustomVariables for additional template substitution
	CustomVariables map[string]string
}

// DefaultPythonPatternsConfigOptions returns default configuration options
func DefaultPythonPatternsConfigOptions() PythonPatternsConfigOptions {
	return PythonPatternsConfigOptions{
		TestPort:        "8080",
		TestWorkspace:   "/tmp/lspg-python-e2e-tests",
		ConfigType:      "main",
		CustomVariables: make(map[string]string),
	}
}

// CreatePythonPatternsConfig creates a temporary configuration file for Python patterns testing
// Integrates with PythonRepoManager workspace management
func CreatePythonPatternsConfig(repoManager *PythonRepoManager, options PythonPatternsConfigOptions) (string, func(), error) {
	if repoManager == nil {
		return "", nil, fmt.Errorf("PythonRepoManager cannot be nil")
	}

	// Determine which template to use
	templateName := "python_patterns_config"
	if options.ConfigType == "server" {
		templateName = "python_patterns_test_server"
	}

	// Set up variables for template substitution
	variables := map[string]string{
		"TEST_PORT":      options.TestPort,
		"TEST_WORKSPACE": repoManager.GetWorkspaceDir(),
		"workspaceFolder": repoManager.GetWorkspaceDir(),
	}

	// Add custom variables
	for key, value := range options.CustomVariables {
		variables[key] = value
	}

	// Create configuration using template processing
	configOptions := TempConfigOptions{
		Template:    templateName,
		Variables:   variables,
		FilePrefix:  "python_patterns_config_",
		FileSuffix:  ".yaml",
	}

	configPath, cleanup, err := CreateTempConfigWithOptions(configOptions)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create Python patterns config: %w", err)
	}

	return configPath, cleanup, nil
}

// CreatePythonPatternsMainConfig creates the main test configuration
func CreatePythonPatternsMainConfig(repoManager *PythonRepoManager) (string, func(), error) {
	options := DefaultPythonPatternsConfigOptions()
	options.ConfigType = "main"
	return CreatePythonPatternsConfig(repoManager, options)
}

// CreatePythonPatternsServerConfig creates the test server configuration
func CreatePythonPatternsServerConfig(repoManager *PythonRepoManager) (string, func(), error) {
	options := DefaultPythonPatternsConfigOptions()
	options.ConfigType = "server"
	options.TestPort = "8081" // Different port for server config
	return CreatePythonPatternsConfig(repoManager, options)
}

// CreatePythonPatternsConfigWithPort creates configuration with specific port
func CreatePythonPatternsConfigWithPort(repoManager *PythonRepoManager, port string) (string, func(), error) {
	options := DefaultPythonPatternsConfigOptions()
	options.TestPort = port
	return CreatePythonPatternsConfig(repoManager, options)
}

// ValidatePythonPatternsConfig validates that a configuration file contains expected elements
func ValidatePythonPatternsConfig(configPath string) error {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	configString := string(content)

	// Required sections for Python patterns configuration
	requiredSections := []string{
		"port:",
		"language_pools:",
		"servers:",
		"test_settings:",
		"logging:",
	}

	// Python-specific requirements
	pythonRequirements := []string{
		"language: \"python\"",
		"command: \"pylsp\"",
		"transport: \"stdio\"",
		"patterns/",
		"textDocument/definition",
		"textDocument/references",
		"textDocument/hover",
		"textDocument/completion",
		"textDocument/documentSymbol",
		"workspace/symbol",
	}

	// Check required sections
	for _, section := range requiredSections {
		if !strings.Contains(configString, section) {
			return fmt.Errorf("missing required section: %s", section)
		}
	}

	// Check Python-specific requirements
	for _, requirement := range pythonRequirements {
		if !strings.Contains(configString, requirement) {
			return fmt.Errorf("missing Python requirement: %s", requirement)
		}
	}

	return nil
}

// PythonPatternsTestScenario represents a test scenario configuration
type PythonPatternsTestScenario struct {
	Name            string
	ConfigType      string
	Port            string
	ExpectedFiles   []string
	ExpectedSymbols []string
	TestMethods     []string
}

// GetPredefinedTestScenarios returns predefined test scenarios for Python patterns
func GetPredefinedTestScenarios() []PythonPatternsTestScenario {
	return []PythonPatternsTestScenario{
		{
			Name:       "basic_patterns_test",
			ConfigType: "main",
			Port:       "8080",
			ExpectedFiles: []string{
				"patterns/behavioral/observer.py",
				"patterns/creational/factory.py",
				"patterns/structural/adapter.py",
			},
			ExpectedSymbols: []string{
				"Observer",
				"Factory", 
				"Adapter",
			},
			TestMethods: []string{
				"textDocument/definition",
				"textDocument/hover",
				"textDocument/completion",
			},
		},
		{
			Name:       "comprehensive_lsp_test",
			ConfigType: "server",
			Port:       "8081",
			ExpectedFiles: []string{
				"patterns/behavioral/",
				"patterns/creational/",
				"patterns/structural/",
			},
			ExpectedSymbols: []string{
				"class",
				"function",
				"method",
			},
			TestMethods: []string{
				"textDocument/definition",
				"textDocument/references",
				"textDocument/hover",
				"textDocument/completion",
				"textDocument/documentSymbol",
				"workspace/symbol",
			},
		},
		{
			Name:       "performance_benchmark_test",
			ConfigType: "main",
			Port:       "8082",
			ExpectedFiles: []string{
				"patterns/",
			},
			ExpectedSymbols: []string{
				"Pattern",
				"Implementation",
			},
			TestMethods: []string{
				"textDocument/definition",
				"textDocument/references",
			},
		},
	}
}

// CreateConfigForTestScenario creates a configuration for a specific test scenario
func CreateConfigForTestScenario(repoManager *PythonRepoManager, scenario PythonPatternsTestScenario) (string, func(), error) {
	options := DefaultPythonPatternsConfigOptions()
	options.ConfigType = scenario.ConfigType
	options.TestPort = scenario.Port
	
	// Add scenario-specific variables
	options.CustomVariables["SCENARIO_NAME"] = scenario.Name
	options.CustomVariables["TEST_METHODS"] = strings.Join(scenario.TestMethods, ",")
	
	return CreatePythonPatternsConfig(repoManager, options)
}

// SetupPythonPatternsTestEnvironment sets up a complete test environment
func SetupPythonPatternsTestEnvironment(scenario PythonPatternsTestScenario) (*PythonRepoManager, string, func(), error) {
	// Create PythonRepoManager
	repoConfig := DefaultPythonRepoConfig()
	repoConfig.EnableLogging = true
	repoConfig.ForceClean = true
	
	repoManager := NewPythonRepoManager(repoConfig)
	
	// Clone repository
	if err := repoManager.CloneRepository(); err != nil {
		return nil, "", nil, fmt.Errorf("failed to clone repository: %w", err)
	}
	
	// Create configuration
	configPath, configCleanup, err := CreateConfigForTestScenario(repoManager, scenario)
	if err != nil {
		repoManager.Cleanup()
		return nil, "", nil, fmt.Errorf("failed to create config: %w", err)
	}
	
	// Validate configuration
	if err := ValidatePythonPatternsConfig(configPath); err != nil {
		configCleanup()
		repoManager.Cleanup()
		return nil, "", nil, fmt.Errorf("config validation failed: %w", err)
	}
	
	// Combined cleanup function
	cleanup := func() {
		configCleanup()
		if cleanupErr := repoManager.Cleanup(); cleanupErr != nil {
			fmt.Printf("Warning: failed to cleanup repository: %v\n", cleanupErr)
		}
	}
	
	return repoManager, configPath, cleanup, nil
}

// ValidateTestEnvironment validates that the test environment is properly set up
func ValidateTestEnvironment(repoManager *PythonRepoManager, configPath string, scenario PythonPatternsTestScenario) error {
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
		expectedPath := filepath.Join(workspaceDir, "python-patterns", expectedFile)
		
		// For directories, check they exist
		if strings.HasSuffix(expectedFile, "/") {
			if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
				return fmt.Errorf("expected directory not found: %s", expectedFile)
			}
			continue
		}
		
		// For files, check they exist and are readable
		if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
			return fmt.Errorf("expected file not found: %s", expectedFile)
		}
	}
	
	// Validate configuration
	if err := ValidatePythonPatternsConfig(configPath); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}
	
	return nil
}

// GetConfigTemplatePaths returns the paths to the configuration templates
func GetConfigTemplatePaths() (string, string, error) {
	projectRoot, err := GetProjectRoot()
	if err != nil {
		return "", "", fmt.Errorf("failed to get project root: %w", err)
	}
	
	mainConfigPath := filepath.Join(projectRoot, "tests", "e2e", "fixtures", "python_patterns_config.yaml")
	serverConfigPath := filepath.Join(projectRoot, "tests", "e2e", "fixtures", "python_patterns_test_server.yaml")
	
	// Verify templates exist
	if _, err := os.Stat(mainConfigPath); os.IsNotExist(err) {
		return "", "", fmt.Errorf("main config template not found: %s", mainConfigPath)
	}
	
	if _, err := os.Stat(serverConfigPath); os.IsNotExist(err) {
		return "", "", fmt.Errorf("server config template not found: %s", serverConfigPath)
	}
	
	return mainConfigPath, serverConfigPath, nil
}

// TestConfigurationTemplates tests that both configuration templates are valid
func TestConfigurationTemplates() error {
	mainPath, serverPath, err := GetConfigTemplatePaths()
	if err != nil {
		return err
	}
	
	// Test main configuration template
	if err := ValidatePythonPatternsConfig(mainPath); err != nil {
		return fmt.Errorf("main config template validation failed: %w", err)
	}
	
	// Test server configuration template
	if err := ValidatePythonPatternsConfig(serverPath); err != nil {
		return fmt.Errorf("server config template validation failed: %w", err)
	}
	
	return nil
}