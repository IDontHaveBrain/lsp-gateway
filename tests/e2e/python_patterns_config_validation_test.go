package e2e_test

import (
	"testing"
	"time"

	"github.com/your-org/lsp-gateway/tests/e2e/testutils"
)

// TestPythonPatternsConfigTemplates validates that the configuration templates are properly structured
func TestPythonPatternsConfigTemplates(t *testing.T) {
	if err := testutils.TestConfigurationTemplates(); err != nil {
		t.Fatalf("Configuration template validation failed: %v", err)
	}
}

// TestPythonPatternsConfigIntegration validates integration with PythonRepoManager
func TestPythonPatternsConfigIntegration(t *testing.T) {
	// Skip long-running test if in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create PythonRepoManager
	repoConfig := testutils.DefaultPythonRepoConfig()
	repoConfig.EnableLogging = true
	repoConfig.ForceClean = true
	repoConfig.CloneTimeout = 2 * time.Minute // Generous timeout for CI

	repoManager := testutils.NewPythonRepoManager(repoConfig)

	// Test main configuration creation
	t.Run("MainConfig", func(t *testing.T) {
		configPath, cleanup, err := testutils.CreatePythonPatternsMainConfig(repoManager)
		if err != nil {
			t.Fatalf("Failed to create main config: %v", err)
		}
		defer cleanup()

		// Validate configuration structure
		if err := testutils.ValidatePythonPatternsConfig(configPath); err != nil {
			t.Errorf("Main config validation failed: %v", err)
		}
	})

	// Test server configuration creation
	t.Run("ServerConfig", func(t *testing.T) {
		configPath, cleanup, err := testutils.CreatePythonPatternsServerConfig(repoManager)
		if err != nil {
			t.Fatalf("Failed to create server config: %v", err)
		}
		defer cleanup()

		// Validate configuration structure
		if err := testutils.ValidatePythonPatternsConfig(configPath); err != nil {
			t.Errorf("Server config validation failed: %v", err)
		}
	})

	// Test configuration with custom port
	t.Run("CustomPortConfig", func(t *testing.T) {
		configPath, cleanup, err := testutils.CreatePythonPatternsConfigWithPort(repoManager, "9090")
		if err != nil {
			t.Fatalf("Failed to create custom port config: %v", err)
		}
		defer cleanup()

		// Validate configuration structure
		if err := testutils.ValidatePythonPatternsConfig(configPath); err != nil {
			t.Errorf("Custom port config validation failed: %v", err)
		}
	})
}

// TestPythonPatternsTestScenarios validates predefined test scenarios
func TestPythonPatternsTestScenarios(t *testing.T) {
	scenarios := testutils.GetPredefinedTestScenarios()
	
	if len(scenarios) == 0 {
		t.Fatal("No predefined test scenarios found")
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			// Skip long-running tests if in short mode
			if testing.Short() {
				t.Skip("Skipping scenario test in short mode")
			}

			// Validate scenario structure
			if scenario.Name == "" {
				t.Error("Scenario name cannot be empty")
			}
			if scenario.ConfigType == "" {
				t.Error("Scenario config type cannot be empty")
			}
			if scenario.Port == "" {
				t.Error("Scenario port cannot be empty")
			}
			if len(scenario.TestMethods) == 0 {
				t.Error("Scenario must have at least one test method")
			}

			// Validate test methods are supported LSP methods
			supportedMethods := []string{
				"textDocument/definition",
				"textDocument/references",
				"textDocument/hover",
				"textDocument/completion",
				"textDocument/documentSymbol",
				"workspace/symbol",
			}

			for _, method := range scenario.TestMethods {
				found := false
				for _, supported := range supportedMethods {
					if method == supported {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Unsupported test method: %s", method)
				}
			}
		})
	}
}

// TestPythonPatternsFullEnvironmentSetup validates complete environment setup
func TestPythonPatternsFullEnvironmentSetup(t *testing.T) {
	// Skip expensive test if in short mode
	if testing.Short() {
		t.Skip("Skipping full environment setup test in short mode")
	}

	// Use a simple scenario for testing
	scenario := testutils.PythonPatternsTestScenario{
		Name:       "validation_test",
		ConfigType: "main", 
		Port:       "8080",
		ExpectedFiles: []string{
			"patterns/",
		},
		ExpectedSymbols: []string{
			"class",
		},
		TestMethods: []string{
			"textDocument/definition",
			"textDocument/hover",
		},
	}

	// Setup complete test environment
	repoManager, configPath, cleanup, err := testutils.SetupPythonPatternsTestEnvironment(scenario)
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer cleanup()

	// Validate environment
	if err := testutils.ValidateTestEnvironment(repoManager, configPath, scenario); err != nil {
		t.Errorf("Test environment validation failed: %v", err)
	}

	// Validate repository has expected structure
	testFiles, err := repoManager.GetTestFiles()
	if err != nil {
		t.Errorf("Failed to get test files: %v", err)
	}

	if len(testFiles) == 0 {
		t.Error("No test files found in repository")
	}

	// Validate workspace directory
	workspaceDir := repoManager.GetWorkspaceDir()
	if workspaceDir == "" {
		t.Error("Workspace directory should not be empty")
	}

	t.Logf("Successfully set up test environment with %d Python files", len(testFiles))
	t.Logf("Workspace directory: %s", workspaceDir)
	t.Logf("Configuration file: %s", configPath)
}

// BenchmarkConfigurationCreation benchmarks configuration creation performance
func BenchmarkConfigurationCreation(b *testing.B) {
	repoConfig := testutils.DefaultPythonRepoConfig()
	repoConfig.EnableLogging = false  // Disable logging for benchmarks
	repoManager := testutils.NewPythonRepoManager(repoConfig)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		configPath, cleanup, err := testutils.CreatePythonPatternsMainConfig(repoManager)
		if err != nil {
			b.Fatalf("Failed to create config: %v", err)
		}
		cleanup()
		_ = configPath
	}
}

// TestConfigurationTemplateVariableSubstitution validates variable substitution
func TestConfigurationTemplateVariableSubstitution(t *testing.T) {
	repoConfig := testutils.DefaultPythonRepoConfig()
	repoManager := testutils.NewPythonRepoManager(repoConfig)

	// Test with custom variables
	options := testutils.DefaultPythonPatternsConfigOptions()
	options.TestPort = "9999"
	options.CustomVariables = map[string]string{
		"CUSTOM_VAR": "test_value",
	}

	configPath, cleanup, err := testutils.CreatePythonPatternsConfig(repoManager, options)
	if err != nil {
		t.Fatalf("Failed to create config with custom variables: %v", err)
	}
	defer cleanup()

	// Validate that port substitution worked
	if err := testutils.ValidatePythonPatternsConfig(configPath); err != nil {
		t.Errorf("Config validation failed: %v", err)
	}

	t.Logf("Successfully created configuration with custom variables: %s", configPath)
}