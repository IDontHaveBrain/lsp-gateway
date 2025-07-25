package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/tests/integration/config/helpers"

	"github.com/stretchr/testify/suite"
)

// BackwardCompatibilityTestSuite provides comprehensive tests for backward compatibility
type BackwardCompatibilityTestSuite struct {
	suite.Suite
	testHelper *helpers.ConfigTestHelper
	tempDir    string
	cleanup    []func()
}

// SetupTest initializes the test environment
func (suite *BackwardCompatibilityTestSuite) SetupTest() {
	tempDir, err := os.MkdirTemp("", "compatibility-test-*")
	suite.Require().NoError(err)
	suite.tempDir = tempDir

	suite.testHelper = helpers.NewConfigTestHelper(tempDir)
	suite.cleanup = []func(){}

	// Register cleanup for temp directory
	suite.cleanup = append(suite.cleanup, func() {
		os.RemoveAll(tempDir)
	})
}

// TearDownTest cleans up test resources
func (suite *BackwardCompatibilityTestSuite) TearDownTest() {
	for _, cleanupFunc := range suite.cleanup {
		cleanupFunc()
	}
}

// TestLegacyGatewayConfigLoading tests loading of legacy GatewayConfig format
func (suite *BackwardCompatibilityTestSuite) TestLegacyGatewayConfigLoading() {
	suite.Run("LoadSimpleLegacyConfig", func() {
		// Create legacy configuration format
		legacyConfig := `port: 8080
timeout: 30s
max_concurrent_requests: 100

servers:
  - name: "go-lsp"
    languages: ["go"]
    command: "gopls"
    transport: "stdio"
    settings:
      "gopls":
        "analyses":
          "unusedparams": true
          "shadow": true
        "staticcheck": true

  - name: "python-lsp"  
    languages: ["python"]
    command: "python"
    args: ["-m", "pylsp"]
    transport: "stdio"
    settings:
      "pylsp":
        "plugins":
          "pycodestyle":
            "enabled": true
          "flake8":
            "enabled": true

  - name: "typescript-lsp"
    languages: ["typescript", "javascript"]
    command: "typescript-language-server"
    args: ["--stdio"]
    transport: "stdio"
    settings:
      "typescript":
        "preferences":
          "includeCompletionsForModuleExports": true`

		legacyConfigPath := filepath.Join(suite.tempDir, "legacy-config.yaml")
		err := os.WriteFile(legacyConfigPath, []byte(legacyConfig), 0644)
		suite.Require().NoError(err)

		// Load legacy configuration
		gatewayConfig, err := suite.testHelper.LoadLegacyGatewayConfig(legacyConfigPath)
		suite.Require().NoError(err)
		suite.NotNil(gatewayConfig)

		// Validate legacy config loaded correctly
		suite.Equal(8080, gatewayConfig.Port)
		suite.Equal(30*time.Second, gatewayConfig.Timeout)
		suite.Equal(100, gatewayConfig.MaxConcurrentRequests)
		suite.Len(gatewayConfig.Servers, 3)

		// Validate server configurations
		serversByName := make(map[string]config.ServerConfig)
		for _, server := range gatewayConfig.Servers {
			serversByName[server.Name] = server
		}

		// Check Go LSP server
		goServer := serversByName["go-lsp"]
		suite.Equal("gopls", goServer.Command)
		suite.Equal([]string{"go"}, goServer.Languages)
		suite.Equal("stdio", goServer.Transport)
		suite.NotNil(goServer.Settings)

		// Check Python LSP server
		pythonServer := serversByName["python-lsp"]
		suite.Equal("python", pythonServer.Command)
		suite.Equal([]string{"-m", "pylsp"}, pythonServer.Args)
		suite.Equal([]string{"python"}, pythonServer.Languages)

		// Check TypeScript LSP server
		tsServer := serversByName["typescript-lsp"]
		suite.Equal("typescript-language-server", tsServer.Command)
		suite.Equal([]string{"--stdio"}, tsServer.Args)
		suite.Contains(tsServer.Languages, "typescript")
		suite.Contains(tsServer.Languages, "javascript")
	})

	suite.Run("LoadLegacyConfigWithMigration", func() {
		// Create legacy config that needs migration
		legacyConfig := `port: 9090
servers:
  - name: "old-go-server"
    languages: ["go"]
    command: "go-langserver"  # Old deprecated server
    transport: "stdio"
    
  - name: "old-python-server"
    languages: ["python"]  
    command: "pyls"  # Old deprecated server
    transport: "stdio"`

		legacyConfigPath := filepath.Join(suite.tempDir, "legacy-migration.yaml")
		err := os.WriteFile(legacyConfigPath, []byte(legacyConfig), 0644)
		suite.Require().NoError(err)

		// Load and migrate legacy configuration
		migratedConfig, err := suite.testHelper.LoadAndMigrateLegacyConfig(legacyConfigPath)
		suite.Require().NoError(err)
		suite.NotNil(migratedConfig)

		// Validate migration occurred
		suite.Equal(9090, migratedConfig.BaseConfig.Port)
		suite.Len(migratedConfig.BaseConfig.Servers, 2)

		// Validate server migration
		serversByName := make(map[string]config.ServerConfig)
		for _, server := range migratedConfig.BaseConfig.Servers {
			serversByName[server.Name] = server
		}

		// Check that deprecated servers were migrated
		goServer := serversByName["old-go-server"]
		suite.Equal("gopls", goServer.Command, "Should migrate go-langserver to gopls")

		pythonServer := serversByName["old-python-server"]
		suite.Equal("python", pythonServer.Command, "Should migrate pyls command")
		suite.Equal([]string{"-m", "pylsp"}, pythonServer.Args, "Should add pylsp args")

		// Validate enhanced features were added during migration
		suite.NotNil(migratedConfig.Performance, "Should add performance config during migration")
		suite.NotNil(migratedConfig.MultiServer, "Should add multi-server config during migration")
	})

	suite.Run("LoadLegacyConfigWithFeatureDegradation", func() {
		// Create legacy config with unsupported features that should degrade gracefully
		legacyConfig := `port: 8080
unsupported_field: "this should be ignored"
deprecated_timeout: 60s

servers:
  - name: "test-server"
    languages: ["go"]
    command: "gopls"
    transport: "stdio"
    deprecated_setting: "should be ignored"
    old_health_check: "tcp://localhost:8081"`

		legacyConfigPath := filepath.Join(suite.tempDir, "legacy-degradation.yaml")
		err := os.WriteFile(legacyConfigPath, []byte(legacyConfig), 0644)
		suite.Require().NoError(err)

		// Load config with feature degradation
		config, err := suite.testHelper.LoadConfigWithDegradation(legacyConfigPath)
		suite.Require().NoError(err)
		suite.NotNil(config)

		// Validate that known fields are loaded
		suite.Equal(8080, config.Port)
		suite.Len(config.Servers, 1)

		// Validate that unknown/deprecated fields are ignored gracefully
		server := config.Servers[0]
		suite.Equal("test-server", server.Name)
		suite.Equal("gopls", server.Command)
		suite.Equal([]string{"go"}, server.Languages)
	})
}

// TestConfigurationUpgradePaths tests different upgrade paths from legacy to enhanced configs
func (suite *BackwardCompatibilityTestSuite) TestConfigurationUpgradePaths() {
	suite.Run("UpgradeV1ToV2", func() {
		// Create V1 configuration
		v1Config := `# LSP Gateway Configuration v1.0
version: "1.0"
port: 8080

servers:
  - name: "go-server"
    languages: ["go"]
    command: "gopls"
    transport: "stdio"`

		v1ConfigPath := filepath.Join(suite.tempDir, "config-v1.yaml")
		err := os.WriteFile(v1ConfigPath, []byte(v1Config), 0644)
		suite.Require().NoError(err)

		// Upgrade to V2
		v2Config, err := suite.testHelper.UpgradeConfigV1ToV2(v1ConfigPath)
		suite.Require().NoError(err)
		suite.NotNil(v2Config)

		// Validate V2 features added
		suite.Equal("2.0", v2Config.Version)
		suite.NotNil(v2Config.Performance, "Should add performance config in V2")
		suite.NotNil(v2Config.MultiServer, "Should add multi-server support in V2")

		// Validate backward compatibility preserved
		suite.Equal(8080, v2Config.BaseConfig.Port)
		suite.Len(v2Config.BaseConfig.Servers, 1)
		suite.Equal("go-server", v2Config.BaseConfig.Servers[0].Name)
	})

	suite.Run("UpgradeV2ToV3", func() {
		// Create V2 configuration
		v2Config := &config.EnhancedConfig{
			Version: "2.0",
			BaseConfig: config.GatewayConfig{
				Port: 8080,
				Servers: []config.ServerConfig{
					{
						Name:      "go-server",
						Languages: []string{"go"},
						Command:   "gopls",
						Transport: "stdio",
					},
				},
			},
			Performance: &config.PerformanceConfig{
				MaxConcurrentRequests: 100,
				RequestTimeout:        30 * time.Second,
			},
		}

		v2ConfigPath := filepath.Join(suite.tempDir, "config-v2.yaml")
		err := suite.testHelper.SaveEnhancedConfig(v2Config, v2ConfigPath)
		suite.Require().NoError(err)

		// Upgrade to V3
		v3Config, err := suite.testHelper.UpgradeConfigV2ToV3(v2ConfigPath)
		suite.Require().NoError(err)
		suite.NotNil(v3Config)

		// Validate V3 features added
		suite.Equal("3.0", v3Config.Version)
		suite.NotNil(v3Config.MultiLanguage, "Should add multi-language support in V3")
		suite.NotNil(v3Config.Optimization, "Should add optimization config in V3")
		suite.NotNil(v3Config.Templates, "Should add template support in V3")

		// Validate V2 features preserved
		suite.NotNil(v3Config.Performance)
		suite.Equal(100, v3Config.Performance.MaxConcurrentRequests)
		suite.Equal(30*time.Second, v3Config.Performance.RequestTimeout)

		// Validate V1 features preserved
		suite.Equal(8080, v3Config.BaseConfig.Port)
		suite.Len(v3Config.BaseConfig.Servers, 1)
	})

	suite.Run("DirectUpgradeV1ToV3", func() {
		// Create V1 configuration
		v1Config := `version: "1.0" 
port: 9000
timeout: 45s

servers:
  - name: "python-server"
    languages: ["python"]
    command: "python"
    args: ["-m", "pylsp"]
    transport: "stdio"
    
  - name: "js-server"
    languages: ["javascript", "typescript"]
    command: "typescript-language-server"
    args: ["--stdio"]  
    transport: "stdio"`

		v1ConfigPath := filepath.Join(suite.tempDir, "config-v1-direct.yaml")
		err := os.WriteFile(v1ConfigPath, []byte(v1Config), 0644)
		suite.Require().NoError(err)

		// Direct upgrade to V3
		v3Config, err := suite.testHelper.UpgradeConfigDirectV1ToV3(v1ConfigPath)
		suite.Require().NoError(err)
		suite.NotNil(v3Config)

		// Validate all modern features added
		suite.Equal("3.0", v3Config.Version)
		suite.NotNil(v3Config.Performance)
		suite.NotNil(v3Config.MultiServer)
		suite.NotNil(v3Config.MultiLanguage)
		suite.NotNil(v3Config.Optimization)

		// Validate original config preserved
		suite.Equal(9000, v3Config.BaseConfig.Port)
		suite.Equal(45*time.Second, v3Config.BaseConfig.Timeout)
		suite.Len(v3Config.BaseConfig.Servers, 2)

		// Validate server configurations preserved and enhanced
		serversByName := make(map[string]config.ServerConfig)
		for _, server := range v3Config.BaseConfig.Servers {
			serversByName[server.Name] = server
		}

		pythonServer := serversByName["python-server"]
		suite.Equal("python", pythonServer.Command)
		suite.Equal([]string{"-m", "pylsp"}, pythonServer.Args)
		suite.Equal([]string{"python"}, pythonServer.Languages)

		jsServer := serversByName["js-server"]
		suite.Equal("typescript-language-server", jsServer.Command)
		suite.Contains(jsServer.Languages, "javascript")
		suite.Contains(jsServer.Languages, "typescript")
	})
}

// TestAutomaticMigration tests automatic migration capabilities
func (suite *BackwardCompatibilityTestSuite) TestAutomaticMigration() {
	suite.Run("DetectAndMigrateDeprecatedCommands", func() {
		// Create config with deprecated LSP server commands
		deprecatedConfig := `port: 8080
servers:
  - name: "go-server"
    languages: ["go"]
    command: "go-langserver"  # Deprecated, should migrate to gopls
    transport: "stdio"
    
  - name: "python-server"
    languages: ["python"]
    command: "pyls"  # Deprecated, should migrate to pylsp
    transport: "stdio"
    
  - name: "rust-server"
    languages: ["rust"]
    command: "rls"  # Deprecated, should migrate to rust-analyzer
    transport: "stdio"
    
  - name: "cpp-server"
    languages: ["cpp", "c"]
    command: "cquery"  # Deprecated, should migrate to clangd
    transport: "stdio"`

		configPath := filepath.Join(suite.tempDir, "deprecated-commands.yaml")
		err := os.WriteFile(configPath, []byte(deprecatedConfig), 0644)
		suite.Require().NoError(err)

		// Perform automatic migration
		migratedConfig, err := suite.testHelper.AutoMigrateDeprecatedCommands(configPath)
		suite.Require().NoError(err)
		suite.NotNil(migratedConfig)

		// Validate migrations
		serversByName := make(map[string]config.ServerConfig)
		for _, server := range migratedConfig.BaseConfig.Servers {
			serversByName[server.Name] = server
		}

		// Check Go server migration
		goServer := serversByName["go-server"]
		suite.Equal("gopls", goServer.Command, "Should migrate go-langserver to gopls")

		// Check Python server migration
		pythonServer := serversByName["python-server"]
		suite.Equal("python", pythonServer.Command, "Should migrate pyls command")
		suite.Equal([]string{"-m", "pylsp"}, pythonServer.Args, "Should add correct pylsp args")

		// Check Rust server migration
		rustServer := serversByName["rust-server"]
		suite.Equal("rust-analyzer", rustServer.Command, "Should migrate rls to rust-analyzer")

		// Check C++ server migration
		cppServer := serversByName["cpp-server"]
		suite.Equal("clangd", cppServer.Command, "Should migrate cquery to clangd")
	})

	suite.Run("MigrateDeprecatedSettings", func() {
		// Create config with deprecated settings format
		oldSettingsConfig := `port: 8080  
servers:
  - name: "go-server"
    languages: ["go"]
    command: "gopls"
    transport: "stdio"
    settings:
      # Old format settings
      "go.useLanguageServer": true
      "go.languageServerFlags": ["-logfile", "/tmp/gopls.log"]
      "go.languageServerExperimentalFeatures":
        "diagnosticsDelay": "500ms"
        "watchFileChanges": true
        
  - name: "python-server"
    languages: ["python"]
    command: "python"
    args: ["-m", "pylsp"]
    transport: "stdio"
    settings:
      # Old format settings
      "python.linting.enabled": true
      "python.linting.pylintEnabled": true
      "python.formatting.provider": "black"`

		configPath := filepath.Join(suite.tempDir, "deprecated-settings.yaml")
		err := os.WriteFile(configPath, []byte(oldSettingsConfig), 0644)
		suite.Require().NoError(err)

		// Migrate deprecated settings
		migratedConfig, err := suite.testHelper.MigrateDeprecatedSettings(configPath)
		suite.Require().NoError(err)
		suite.NotNil(migratedConfig)

		// Validate settings migration
		serversByName := make(map[string]config.ServerConfig)
		for _, server := range migratedConfig.BaseConfig.Servers {
			serversByName[server.Name] = server
		}

		// Check Go server settings migration
		goServer := serversByName["go-server"]
		suite.NotNil(goServer.Settings)
		suite.Contains(goServer.Settings, "gopls", "Should have modern gopls settings")

		// Check Python server settings migration
		pythonServer := serversByName["python-server"]
		suite.NotNil(pythonServer.Settings)
		suite.Contains(pythonServer.Settings, "pylsp", "Should have modern pylsp settings")
	})

	suite.Run("PreserveCustomConfigurations", func() {
		// Create config with custom configurations that should be preserved
		customConfig := `port: 8080
custom_field: "preserve this"
timeout: 60s

servers:
  - name: "custom-go-server"
    languages: ["go"]
    command: "gopls"
    transport: "stdio"
    custom_server_field: "preserve this too"
    settings:
      "gopls":
        "analyses":
          "unusedparams": true
        "customAnalysis": "preserve this analysis"
        
  - name: "custom-python-server"
    languages: ["python"]
    command: "python"
    args: ["-m", "pylsp"]
    transport: "stdio"
    custom_timeout: 45
    settings:
      "pylsp":
        "plugins":
          "pycodestyle":
            "enabled": true
          "customPlugin":
            "enabled": true
            "config": "preserve this"`

		configPath := filepath.Join(suite.tempDir, "custom-config.yaml")
		err := os.WriteFile(configPath, []byte(customConfig), 0644)
		suite.Require().NoError(err)

		// Migrate while preserving custom configurations
		migratedConfig, err := suite.testHelper.MigratePreservingCustom(configPath)
		suite.Require().NoError(err)
		suite.NotNil(migratedConfig)

		// Validate custom fields preserved
		suite.Equal(8080, migratedConfig.BaseConfig.Port)
		suite.Equal(60*time.Second, migratedConfig.BaseConfig.Timeout)

		// Validate servers preserved with custom fields
		suite.Len(migratedConfig.BaseConfig.Servers, 2)

		serversByName := make(map[string]config.ServerConfig)
		for _, server := range migratedConfig.BaseConfig.Servers {
			serversByName[server.Name] = server
		}

		// Check custom Go server
		goServer := serversByName["custom-go-server"]
		suite.Equal("gopls", goServer.Command)
		suite.NotNil(goServer.Settings)

		// Check custom Python server
		pythonServer := serversByName["custom-python-server"]
		suite.Equal("python", pythonServer.Command)
		suite.Equal([]string{"-m", "pylsp"}, pythonServer.Args)
		suite.NotNil(pythonServer.Settings)
	})
}

// TestValidationWithBackwardCompatibility tests validation with backward compatibility considerations
func (suite *BackwardCompatibilityTestSuite) TestValidationWithBackwardCompatibility() {
	suite.Run("ValidateLegacyConfigStructure", func() {
		// Create legacy config with mixed old and new format
		mixedConfig := `port: 8080
version: "1.0"  # Legacy version
timeout: 30s

servers:
  - name: "modern-go-server"
    languages: ["go"]  
    command: "gopls"
    transport: "stdio"
    # Modern settings
    settings:
      "gopls":
        "analyses":
          "unusedparams": true
          
  - name: "legacy-python-server"
    languages: ["python"]
    command: "pyls"  # Legacy command
    transport: "stdio" 
    # Legacy settings format
    settings:
      "python.linting.enabled": true`

		configPath := filepath.Join(suite.tempDir, "mixed-config.yaml")
		err := os.WriteFile(configPath, []byte(mixedConfig), 0644)
		suite.Require().NoError(err)

		// Validate with backward compatibility
		validation, err := suite.testHelper.ValidateWithBackwardCompatibility(configPath)
		suite.Require().NoError(err)
		suite.NotNil(validation)

		// Should have warnings but not errors for backward compatibility
		suite.False(validation.HasErrors(), "Should not have errors for backward compatible config")
		suite.True(validation.HasWarnings(), "Should have warnings for deprecated features")

		// Check specific warnings
		warnings := validation.GetWarnings()
		suite.True(containsWarning(warnings, "deprecated command"), "Should warn about deprecated pyls command")
		suite.True(containsWarning(warnings, "legacy settings"), "Should warn about legacy settings format")
	})

	suite.Run("ValidateUpgradedConfig", func() {
		// Create and upgrade a legacy config
		legacyConfig := `port: 8080
servers:
  - name: "test-server"
    languages: ["go"]
    command: "go-langserver"  # Will be upgraded
    transport: "stdio"`

		legacyPath := filepath.Join(suite.tempDir, "upgrade-validate.yaml")
		err := os.WriteFile(legacyPath, []byte(legacyConfig), 0644)
		suite.Require().NoError(err)

		// Upgrade config
		upgradedConfig, err := suite.testHelper.UpgradeConfigV1ToV2(legacyPath)
		suite.Require().NoError(err)

		// Save upgraded config
		upgradedPath := filepath.Join(suite.tempDir, "upgraded-config.yaml")
		err = suite.testHelper.SaveEnhancedConfig(upgradedConfig, upgradedPath)
		suite.Require().NoError(err)

		// Validate upgraded config
		validation, err := suite.testHelper.ValidateEnhancedConfig(upgradedPath)
		suite.Require().NoError(err)
		suite.NotNil(validation)

		// Upgraded config should be fully valid
		suite.False(validation.HasErrors(), "Upgraded config should not have errors")
		suite.False(validation.HasWarnings(), "Upgraded config should not have warnings")
		suite.True(validation.IsValid(), "Upgraded config should be fully valid")
	})
}

// Helper function to check if warnings contain specific text
func containsWarning(warnings []string, text string) bool {
	for _, warning := range warnings {
		if contains(warning, text) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsAt(s, substr, 1)))
}

func containsAt(s, substr string, start int) bool {
	if start >= len(s) {
		return false
	}
	if len(s)-start < len(substr) {
		return false
	}
	if s[start:start+len(substr)] == substr {
		return true
	}
	return containsAt(s, substr, start+1)
}

// Run the test suite
func TestBackwardCompatibilityTestSuite(t *testing.T) {
	suite.Run(t, new(BackwardCompatibilityTestSuite))
}
