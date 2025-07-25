package unit_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-gateway/internal/config"
)

func TestOptimizationStrategies(t *testing.T) {
	// Create a test multi-language configuration
	projectInfo := &config.MultiLanguageProjectInfo{
		ProjectType:   config.ProjectTypeMulti,
		RootDirectory: "/test/project",
		LanguageContexts: []*config.LanguageContext{
			{
				Language:     "go",
				FilePatterns: []string{"*.go"},
				FileCount:    20,
				RootMarkers:  []string{"go.mod"},
				RootPath:     "/test/project",
				Frameworks:   []string{},
			},
			{
				Language:     "python",
				FilePatterns: []string{"*.py"},
				FileCount:    15,
				RootMarkers:  []string{"requirements.txt"},
				RootPath:     "/test/project",
				Frameworks:   []string{},
			},
		},
		DetectedAt: time.Now(),
		Metadata:   make(map[string]interface{}),
	}

	generator := config.NewConfigGenerator()
	mlConfig, err := generator.GenerateMultiLanguageConfig(projectInfo)
	if err != nil {
		t.Fatalf("Failed to generate multi-language config: %v", err)
	}

	// Test production optimization
	t.Run("ProductionOptimization", func(t *testing.T) {
		prodConfig := *mlConfig // Copy
		prodOpt := config.NewProductionOptimization()
		
		err := prodOpt.ApplyOptimizations(&prodConfig)
		if err != nil {
			t.Fatalf("Failed to apply production optimization: %v", err)
		}

		if prodConfig.OptimizedFor != config.OptimizationProduction {
			t.Errorf("Expected OptimizedFor to be %s, got %s", config.OptimizationProduction, prodConfig.OptimizedFor)
		}

		// Check production-specific settings
		performanceSettings := prodOpt.GetPerformanceSettings()
		if performanceSettings["indexing_strategy"] != "incremental" {
			t.Errorf("Expected incremental indexing strategy for production")
		}

		memorySettings := prodOpt.GetMemorySettings()
		if memorySettings["memory_mode"] != "conservative" {
			t.Errorf("Expected conservative memory mode for production")
		}
	})

	// Test analysis optimization
	t.Run("AnalysisOptimization", func(t *testing.T) {
		analysisConfig := *mlConfig // Copy
		analysisOpt := config.NewAnalysisOptimization()
		
		err := analysisOpt.ApplyOptimizations(&analysisConfig)
		if err != nil {
			t.Fatalf("Failed to apply analysis optimization: %v", err)
		}

		if analysisConfig.OptimizedFor != config.OptimizationAnalysis {
			t.Errorf("Expected OptimizedFor to be %s, got %s", config.OptimizationAnalysis, analysisConfig.OptimizedFor)
		}

		// Check analysis-specific settings
		performanceSettings := analysisOpt.GetPerformanceSettings()
		if performanceSettings["indexing_strategy"] != "full" {
			t.Errorf("Expected full indexing strategy for analysis")
		}

		if !performanceSettings["enable_all_analyses"].(bool) {
			t.Errorf("Expected all analyses to be enabled for analysis mode")
		}
	})

	// Test development optimization
	t.Run("DevelopmentOptimization", func(t *testing.T) {
		devConfig := *mlConfig // Copy
		devOpt := config.NewDevelopmentOptimization()
		
		err := devOpt.ApplyOptimizations(&devConfig)
		if err != nil {
			t.Fatalf("Failed to apply development optimization: %v", err)
		}

		if devConfig.OptimizedFor != config.OptimizationDevelopment {
			t.Errorf("Expected OptimizedFor to be %s, got %s", config.OptimizationDevelopment, devConfig.OptimizedFor)
		}

		// Check development-specific settings
		performanceSettings := devOpt.GetPerformanceSettings()
		if !performanceSettings["enable_diagnostics"].(bool) {
			t.Errorf("Expected diagnostics to be enabled for development mode")
		}
	})
}

func TestOptimizationManager(t *testing.T) {
	manager := config.NewOptimizationManager()

	// Test getting available strategies
	strategies := manager.GetAvailableStrategies()
	expectedStrategies := []string{"production", "analysis", "development"}
	
	if len(strategies) != len(expectedStrategies) {
		t.Errorf("Expected %d strategies, got %d", len(expectedStrategies), len(strategies))
	}

	// Test getting specific strategy
	prodStrategy, err := manager.GetStrategy("production")
	if err != nil {
		t.Fatalf("Failed to get production strategy: %v", err)
	}

	if prodStrategy.GetOptimizationName() != "production" {
		t.Errorf("Expected production strategy name, got %s", prodStrategy.GetOptimizationName())
	}

	// Test unknown strategy
	_, err = manager.GetStrategy("unknown")
	if err == nil {
		t.Error("Expected error for unknown strategy")
	}
}

func TestConfigurationIntegrator(t *testing.T) {
	integrator := config.NewConfigurationIntegrator()

	// Create test configurations
	config1 := createTestMultiLanguageConfig("go", "/test/project1")
	config2 := createTestMultiLanguageConfig("python", "/test/project2")

	// Test integration
	t.Run("IntegrateConfigurations", func(t *testing.T) {
		integrated, err := integrator.IntegrateConfigurations(config1, config2)
		if err != nil {
			t.Fatalf("Failed to integrate configurations: %v", err)
		}

		if len(integrated.ServerConfigs) != 2 {
			t.Errorf("Expected 2 server configs after integration, got %d", len(integrated.ServerConfigs))
		}

		// Check metadata
		if integrated.Metadata["integration_timestamp"] == nil {
			t.Error("Expected integration timestamp in metadata")
		}

		if integrated.Metadata["source_configs"].(int) != 2 {
			t.Errorf("Expected 2 source configs in metadata, got %v", integrated.Metadata["source_configs"])
		}
	})

	// Test validation
	t.Run("ValidateConfiguration", func(t *testing.T) {
		err := integrator.ValidateConfiguration(config1)
		if err != nil {
			t.Errorf("Validation failed for valid configuration: %v", err)
		}

		// Test invalid configuration
		invalidConfig := &config.MultiLanguageConfig{
			ProjectInfo:   nil, // Invalid: nil project info
			ServerConfigs: []*config.ServerConfig{},
		}

		err = integrator.ValidateConfiguration(invalidConfig)
		if err == nil {
			t.Error("Expected validation error for invalid configuration")
		}
	})

	// Test converting to gateway config
	t.Run("ConvertToGatewayConfig", func(t *testing.T) {
		gatewayConfig, err := integrator.ConvertToGatewayConfig(config1)
		if err != nil {
			t.Fatalf("Failed to convert to gateway config: %v", err)
		}

		if len(gatewayConfig.Servers) == 0 {
			t.Error("Expected servers in gateway config")
		}

		if gatewayConfig.ProjectAware != true {
			t.Error("Expected project aware to be true")
		}
	})
}

func TestConfigurationMigration(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test legacy gateway config migration
	t.Run("LegacyGatewayMigration", func(t *testing.T) {
		legacyConfig := map[string]interface{}{
			"port": 8080,
			"servers": []map[string]interface{}{
				{
					"name":      "go-lsp",
					"languages": []string{"go"},
					"command":   "gopls",
					"transport": "stdio",
				},
			},
		}

		handler := &config.LegacyGatewayMigrationHandler{}
		if !handler.CanHandle(legacyConfig) {
			t.Error("Expected handler to handle legacy gateway config")
		}

		migratedConfig, err := handler.Migrate(legacyConfig)
		if err != nil {
			t.Fatalf("Failed to migrate legacy config: %v", err)
		}

		if len(migratedConfig.ServerConfigs) != 1 {
			t.Errorf("Expected 1 server config after migration, got %d", len(migratedConfig.ServerConfigs))
		}

		if migratedConfig.Metadata["migrated_from"] != "legacy_gateway" {
			t.Error("Expected migration metadata")
		}
	})

	// Test simple config migration
	t.Run("SimpleConfigMigration", func(t *testing.T) {
		simpleConfig := map[string]interface{}{
			"languages": []interface{}{"go", "python"},
		}

		handler := &config.SimpleConfigMigrationHandler{}
		if !handler.CanHandle(simpleConfig) {
			t.Error("Expected handler to handle simple config")
		}

		migratedConfig, err := handler.Migrate(simpleConfig)
		if err != nil {
			t.Fatalf("Failed to migrate simple config: %v", err)
		}

		if len(migratedConfig.ServerConfigs) != 2 {
			t.Errorf("Expected 2 server configs after migration, got %d", len(migratedConfig.ServerConfigs))
		}
	})
}

func TestConfigurationValidators(t *testing.T) {
	// Test framework compatibility validator
	t.Run("FrameworkCompatibilityValidator", func(t *testing.T) {
		validator := &config.FrameworkCompatibilityValidator{}

		// Create config with framework but no matching server
		invalidConfig := &config.MultiLanguageConfig{
			ProjectInfo: &config.MultiLanguageProjectInfo{
				Frameworks: []*config.Framework{
					{
						Name:     "react",
						Language: "typescript",
					},
				},
			},
			ServerConfigs: []*config.ServerConfig{
				{
					Name:      "go-lsp",
					Languages: []string{"go"}, // No typescript server
				},
			},
		}

		err := validator.Validate(invalidConfig)
		if err == nil {
			t.Error("Expected validation error for framework without matching server")
		}

		// Create valid config
		validConfig := &config.MultiLanguageConfig{
			ProjectInfo: &config.MultiLanguageProjectInfo{
				Frameworks: []*config.Framework{
					{
						Name:     "react",
						Language: "typescript",
					},
				},
			},
			ServerConfigs: []*config.ServerConfig{
				{
					Name:      "typescript-lsp",
					Languages: []string{"typescript"},
				},
			},
		}

		err = validator.Validate(validConfig)
		if err != nil {
			t.Errorf("Expected valid config to pass validation: %v", err)
		}
	})

	// Test resource constraint validator
	t.Run("ResourceConstraintValidator", func(t *testing.T) {
		validator := &config.ResourceConstraintValidator{}

		// Create config with excessive concurrent requests
		invalidConfig := &config.MultiLanguageConfig{
			ServerConfigs: []*config.ServerConfig{
				{
					Name:                  "server1",
					MaxConcurrentRequests: 600,
				},
				{
					Name:                  "server2",
					MaxConcurrentRequests: 500,
				},
			},
		}

		err := validator.Validate(invalidConfig)
		if err == nil {
			t.Error("Expected validation error for excessive concurrent requests")
		}
	})

	// Test language consistency validator
	t.Run("LanguageConsistencyValidator", func(t *testing.T) {
		validator := &config.LanguageConsistencyValidator{}

		// Create config with language context but no server
		invalidConfig := &config.MultiLanguageConfig{
			ProjectInfo: &config.MultiLanguageProjectInfo{
				LanguageContexts: []*config.LanguageContext{
					{
						Language: "rust",
					},
				},
			},
			ServerConfigs: []*config.ServerConfig{
				{
					Name:      "go-lsp",
					Languages: []string{"go"}, // No rust server
				},
			},
		}

		err := validator.Validate(invalidConfig)
		if err == nil {
			t.Error("Expected validation error for language without server")
		}
	})
}

func TestFileIOIntegration(t *testing.T) {
	// Create a test configuration
	testConfig := createTestMultiLanguageConfig("go", "/test/project")

	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "config_io_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test YAML I/O
	t.Run("YAMLFileIO", func(t *testing.T) {
		yamlPath := filepath.Join(tempDir, "test_config.yaml")

		// Write YAML
		err := testConfig.WriteYAML(yamlPath)
		if err != nil {
			t.Fatalf("Failed to write YAML config: %v", err)
		}

		// Read YAML
		loadedConfig, err := config.LoadMultiLanguageConfig(yamlPath)
		if err != nil {
			t.Fatalf("Failed to load YAML config: %v", err)
		}

		// Verify loaded config
		if len(loadedConfig.ServerConfigs) != len(testConfig.ServerConfigs) {
			t.Errorf("Expected %d server configs, got %d", len(testConfig.ServerConfigs), len(loadedConfig.ServerConfigs))
		}
	})

	// Test JSON I/O
	t.Run("JSONFileIO", func(t *testing.T) {
		jsonPath := filepath.Join(tempDir, "test_config.json")

		// Write JSON
		err := testConfig.WriteJSON(jsonPath)
		if err != nil {
			t.Fatalf("Failed to write JSON config: %v", err)
		}

		// Read JSON
		loadedConfig, err := config.LoadMultiLanguageConfig(jsonPath)
		if err != nil {
			t.Fatalf("Failed to load JSON config: %v", err)
		}

		// Verify loaded config
		if len(loadedConfig.ServerConfigs) != len(testConfig.ServerConfigs) {
			t.Errorf("Expected %d server configs, got %d", len(testConfig.ServerConfigs), len(loadedConfig.ServerConfigs))
		}
	})
}

// Helper function to create test multi-language config
func createTestMultiLanguageConfig(language, rootPath string) *config.MultiLanguageConfig {
	projectInfo := &config.MultiLanguageProjectInfo{
		ProjectType:   config.ProjectTypeSingle,
		RootDirectory: rootPath,
		LanguageContexts: []*config.LanguageContext{
			{
				Language:     language,
				FilePatterns: []string{fmt.Sprintf("*.%s", language)},
				FileCount:    10,
				RootMarkers:  []string{},
				RootPath:     rootPath,
				Frameworks:   []string{},
			},
		},
		DetectedAt:    time.Now(),
		Metadata:      make(map[string]interface{}),
	}

	generator := config.NewConfigGenerator()
	mlConfig, _ := generator.GenerateMultiLanguageConfig(projectInfo)
	return mlConfig
}