package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-gateway/tests/integration/config/helpers"

	"github.com/stretchr/testify/suite"
)

// CLIIntegrationTestSuite provides comprehensive tests for CLI command integration
type CLIIntegrationTestSuite struct {
	suite.Suite
	testHelper *helpers.ConfigTestHelper
	cliHelper  *helpers.CLITestHelper
	tempDir    string
	cleanup    []func()
}

// SetupTest initializes the test environment
func (suite *CLIIntegrationTestSuite) SetupTest() {
	tempDir, err := os.MkdirTemp("", "cli-integration-test-*")
	suite.Require().NoError(err)
	suite.tempDir = tempDir

	suite.testHelper = helpers.NewConfigTestHelper(tempDir)
	suite.cliHelper = helpers.NewCLITestHelper(tempDir)
	suite.cleanup = []func(){}

	// Register cleanup for temp directory
	suite.cleanup = append(suite.cleanup, func() {
		os.RemoveAll(tempDir)
	})
}

// TearDownTest cleans up test resources
func (suite *CLIIntegrationTestSuite) TearDownTest() {
	for _, cleanupFunc := range suite.cleanup {
		cleanupFunc()
	}
}

// TestConfigGenerateCommands tests all config generate command variations
func (suite *CLIIntegrationTestSuite) TestConfigGenerateCommands() {
	suite.Run("ConfigGenerateBasic", func() {
		// Create simple Go project
		projectStructure := map[string]string{
			"go.mod":       "module test-project\n\ngo 1.21",
			"main.go":      "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}",
			"pkg/utils.go": "package pkg\n\nfunc Utils() string {\n\treturn \"utils\"\n}",
		}

		projectPath := suite.testHelper.CreateTestProject("basic-go", projectStructure)

		// Run config generate command
		result, err := suite.cliHelper.RunCommand("config", "generate", "--project-path", projectPath)
		suite.Require().NoError(err)
		suite.NotNil(result)

		// Validate command executed successfully
		suite.Equal(0, result.ExitCode, "Command should succeed")
		suite.Contains(result.Stdout, "Configuration generated successfully", "Should show success message")

		// Validate config file was created
		configPath := filepath.Join(projectPath, "lsp-gateway.yaml")
		suite.FileExists(configPath, "Should generate config file")

		// Validate generated config content
		config, err := suite.testHelper.LoadEnhancedConfig(configPath)
		suite.Require().NoError(err)
		suite.NotNil(config)

		suite.Equal("single-language", config.ProjectType, "Should detect single-language project")
		suite.Len(config.BaseConfig.Servers, 1, "Should have one Go server")
		suite.Equal("gopls", config.BaseConfig.Servers[0].Command, "Should configure gopls")
	})

	suite.Run("ConfigGenerateWithAutoDetect", func() {
		// Create multi-language project
		multiLangStructure := map[string]string{
			// Go service
			"services/auth/go.mod":  "module auth\n\ngo 1.21",
			"services/auth/main.go": "package main\n\nfunc main() {}",

			// Python service
			"services/ml/pyproject.toml": `[project]
name = "ml-service"
version = "1.0.0"
dependencies = ["fastapi", "scikit-learn"]`,
			"services/ml/main.py": "from fastapi import FastAPI\n\napp = FastAPI()",

			// TypeScript frontend
			"frontend/package.json":  `{"name": "frontend", "dependencies": {"react": "^18.0.0", "typescript": "^5.0.0"}}`,
			"frontend/tsconfig.json": `{"compilerOptions": {"target": "ES2020"}}`,
			"frontend/src/App.tsx":   "import React from 'react';\n\nconst App = () => <div>App</div>;\n\nexport default App;",
		}

		projectPath := suite.testHelper.CreateTestProject("multi-lang", multiLangStructure)

		// Run config generate with auto-detect
		result, err := suite.cliHelper.RunCommand("config", "generate", "--auto-detect", "--project-path", projectPath)
		suite.Require().NoError(err)
		suite.Equal(0, result.ExitCode, "Command should succeed")

		// Validate auto-detection messages
		suite.Contains(result.Stdout, "Detecting project languages", "Should show detection progress")
		suite.Contains(result.Stdout, "Detected languages: go, python, typescript", "Should show detected languages")

		// Validate generated config
		configPath := filepath.Join(projectPath, "lsp-gateway.yaml")
		config, err := suite.testHelper.LoadEnhancedConfig(configPath)
		suite.Require().NoError(err)

		suite.Equal("monorepo", config.ProjectType, "Should detect monorepo project")
		suite.GreaterOrEqual(len(config.BaseConfig.Servers), 3, "Should have servers for all languages")

		// Validate each language has a server
		languages := make(map[string]bool)
		for _, server := range config.BaseConfig.Servers {
			for _, lang := range server.Languages {
				languages[lang] = true
			}
		}

		suite.True(languages["go"], "Should have Go server")
		suite.True(languages["python"], "Should have Python server")
		suite.True(languages["typescript"], "Should have TypeScript server")
	})

	suite.Run("ConfigGenerateWithTemplate", func() {
		// Create microservices project structure
		microservicesStructure := map[string]string{
			"docker-compose.yml": `version: '3.8'
services:
  auth:
    build: ./auth-service
  user:
    build: ./user-service  
  api:
    build: ./api-gateway`,

			"auth-service/go.mod":     "module auth-service\n\ngo 1.21",
			"auth-service/main.go":    "package main\n\nfunc main() {}",
			"auth-service/Dockerfile": "FROM golang:1.21\nWORKDIR /app\nCOPY . .\nRUN go build .\nCMD [\"./auth-service\"]",

			"user-service/pom.xml": `<?xml version="1.0"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>user-service</artifactId>
    <version>1.0.0</version>
</project>`,
			"user-service/src/main/java/UserService.java": "public class UserService {\n    public static void main(String[] args) {}\n}",
			"user-service/Dockerfile":                     "FROM openjdk:17\nWORKDIR /app\nCOPY target/*.jar app.jar\nCMD [\"java\", \"-jar\", \"app.jar\"]",

			"api-gateway/package.json": `{"name": "api-gateway", "dependencies": {"express": "^4.18.0", "typescript": "^5.0.0"}}`,
			"api-gateway/server.ts":    "import express from 'express';\n\nconst app = express();\napp.listen(3000);",
			"api-gateway/Dockerfile":   "FROM node:18\nWORKDIR /app\nCOPY package*.json ./\nRUN npm install\nCOPY . .\nCMD [\"npm\", \"start\"]",
		}

		projectPath := suite.testHelper.CreateTestProject("microservices", microservicesStructure)

		// Run config generate with microservices template
		result, err := suite.cliHelper.RunCommand("config", "generate", "--template", "microservices", "--project-path", projectPath)
		suite.Require().NoError(err)
		suite.Equal(0, result.ExitCode, "Command should succeed")

		// Validate template application messages
		suite.Contains(result.Stdout, "Using template: microservices", "Should show template selection")
		suite.Contains(result.Stdout, "Applying microservices optimizations", "Should show optimizations")

		// Validate generated config
		configPath := filepath.Join(projectPath, "lsp-gateway.yaml")
		config, err := suite.testHelper.LoadEnhancedConfig(configPath)
		suite.Require().NoError(err)

		suite.Equal("microservices", config.ProjectType, "Should use microservices project type")
		suite.NotNil(config.MultiServer, "Should have multi-server configuration")
		suite.True(config.MultiServer.EnableLoadBalancing, "Should enable load balancing for microservices")
		suite.NotNil(config.Performance, "Should have performance optimizations")
	})

	suite.Run("ConfigGenerateWithOptimizationMode", func() {
		// Create project for optimization testing
		projectStructure := map[string]string{
			"go.mod":  "module optimization-test\n\ngo 1.21",
			"main.go": "package main\n\nfunc main() {}",
		}

		projectPath := suite.testHelper.CreateTestProject("optimization-test", projectStructure)

		// Test different optimization modes
		optimizationModes := []string{"development", "production", "large-project", "memory-optimized"}

		for _, mode := range optimizationModes {
			// Run config generate with specific optimization mode
			result, err := suite.cliHelper.RunCommand("config", "generate",
				"--optimization", mode,
				"--project-path", projectPath,
				"--output", filepath.Join(projectPath, "lsp-gateway-"+mode+".yaml"))
			suite.Require().NoError(err)
			suite.Equal(0, result.ExitCode, "Command should succeed for mode: "+mode)

			// Validate optimization mode messages
			suite.Contains(result.Stdout, "Optimization mode: "+mode, "Should show optimization mode")

			// Validate generated config
			configPath := filepath.Join(projectPath, "lsp-gateway-"+mode+".yaml")
			config, err := suite.testHelper.LoadEnhancedConfig(configPath)
			suite.Require().NoError(err)

			suite.Equal(mode, config.Performance.OptimizationMode, "Should use correct optimization mode")

			// Validate mode-specific optimizations
			switch mode {
			case "development":
				suite.Equal(60*time.Second, config.Performance.RequestTimeout, "Development should have longer timeout")
			case "production":
				suite.GreaterOrEqual(config.Performance.MaxConcurrentRequests, 200, "Production should have high concurrency")
				suite.True(config.MultiServer.EnableLoadBalancing, "Production should enable load balancing")
			case "large-project":
				suite.NotNil(config.Optimization.LargeProjectOptimizations, "Should have large project optimizations")
				suite.True(config.Optimization.LargeProjectOptimizations.BackgroundIndexing, "Should enable background indexing")
			case "memory-optimized":
				suite.NotEmpty(config.Optimization.LargeProjectOptimizations.MemoryLimits, "Should have memory limits")
			}
		}
	})

	suite.Run("ConfigGenerateWithCustomFlags", func() {
		// Create project for custom flag testing
		projectStructure := map[string]string{
			"go.mod":  "module custom-flags\n\ngo 1.21",
			"main.go": "package main\n\nfunc main() {}",
		}

		projectPath := suite.testHelper.CreateTestProject("custom-flags", projectStructure)

		// Run config generate with custom flags
		customConfigPath := filepath.Join(projectPath, "custom-config.yaml")
		result, err := suite.cliHelper.RunCommand("config", "generate",
			"--project-path", projectPath,
			"--output", customConfigPath,
			"--port", "9090",
			"--timeout", "45s",
			"--max-concurrent", "150",
			"--enable-caching",
			"--cache-size", "2000")
		suite.Require().NoError(err)
		suite.Equal(0, result.ExitCode, "Command should succeed")

		// Validate custom flags were applied
		config, err := suite.testHelper.LoadEnhancedConfig(customConfigPath)
		suite.Require().NoError(err)

		suite.Equal(9090, config.BaseConfig.Port, "Should use custom port")
		suite.Equal(45*time.Second, config.BaseConfig.Timeout, "Should use custom timeout")
		suite.Equal(150, config.Performance.MaxConcurrentRequests, "Should use custom max concurrent")
		suite.True(config.Performance.EnableCaching, "Should enable caching")
		suite.Equal(2000, config.Performance.CacheSize, "Should use custom cache size")
	})
}

// TestSetupCommands tests all setup command variations
func (suite *CLIIntegrationTestSuite) TestSetupCommands() {
	suite.Run("SetupAll", func() {
		// Create comprehensive project
		projectStructure := map[string]string{
			"go.mod":         "module setup-test\n\ngo 1.21",
			"main.go":        "package main\n\nfunc main() {}",
			"pyproject.toml": `[project]\nname = "setup-test"\nversion = "1.0.0"`,
			"main.py":        "print('Hello Python')",
			"package.json":   `{"name": "setup-test", "dependencies": {"typescript": "^5.0.0"}}`,
			"tsconfig.json":  `{"compilerOptions": {"target": "ES2020"}}`,
			"src/index.ts":   "console.log('Hello TypeScript');",
		}

		projectPath := suite.testHelper.CreateTestProject("setup-all", projectStructure)

		// Run setup all command
		result, err := suite.cliHelper.RunCommand("setup", "all", "--project-path", projectPath)
		suite.Require().NoError(err)
		suite.Equal(0, result.ExitCode, "Setup all should succeed")

		// Validate setup steps were executed
		suite.Contains(result.Stdout, "Installing language runtimes", "Should install runtimes")
		suite.Contains(result.Stdout, "Installing LSP servers", "Should install LSP servers")
		suite.Contains(result.Stdout, "Generating configuration", "Should generate config")
		suite.Contains(result.Stdout, "Setup completed successfully", "Should show completion")

		// Validate config was generated
		configPath := filepath.Join(projectPath, "lsp-gateway.yaml")
		suite.FileExists(configPath, "Config should be generated during setup")

		// Validate multi-language setup
		config, err := suite.testHelper.LoadEnhancedConfig(configPath)
		suite.Require().NoError(err)
		suite.GreaterOrEqual(len(config.BaseConfig.Servers), 3, "Should setup servers for all languages")
	})

	suite.Run("SetupWithRuntimeDetection", func() {
		// Create project structure that requires runtime detection
		projectStructure := map[string]string{
			"go.mod":                  "module runtime-detection\n\ngo 1.21",
			"main.go":                 "package main\n\nfunc main() {}",
			"Cargo.toml":              `[package]\nname = "runtime-detection"\nversion = "0.1.0"\nedition = "2021"`,
			"src/main.rs":             "fn main() { println!(\"Hello Rust\"); }",
			"pom.xml":                 `<?xml version="1.0"?><project><modelVersion>4.0.0</modelVersion><groupId>test</groupId><artifactId>test</artifactId><version>1.0.0</version></project>`,
			"src/main/java/Main.java": "public class Main { public static void main(String[] args) {} }",
		}

		projectPath := suite.testHelper.CreateTestProject("runtime-detection", projectStructure)

		// Run setup with runtime detection
		result, err := suite.cliHelper.RunCommand("setup", "all", "--detect-runtimes", "--project-path", projectPath)
		suite.Require().NoError(err)
		suite.Equal(0, result.ExitCode, "Setup with runtime detection should succeed")

		// Validate runtime detection messages
		suite.Contains(result.Stdout, "Detecting installed runtimes", "Should detect runtimes")
		suite.Contains(result.Stdout, "Found Go runtime", "Should detect Go")

		// Note: Rust and Java detection depends on actual runtime installation
		// In a real test environment, we'd mock these or ensure they're installed

		// Validate comprehensive configuration
		configPath := filepath.Join(projectPath, "lsp-gateway.yaml")
		config, err := suite.testHelper.LoadEnhancedConfig(configPath)
		suite.Require().NoError(err)

		// Should have servers for detected languages
		languageServers := make(map[string]bool)
		for _, server := range config.BaseConfig.Servers {
			for _, lang := range server.Languages {
				languageServers[lang] = true
			}
		}

		suite.True(languageServers["go"], "Should have Go server")
		// Additional language servers depend on runtime availability
	})

	suite.Run("SetupWithPerformanceTuning", func() {
		// Create large project structure for performance tuning
		largeProjectStructure := make(map[string]string)
		largeProjectStructure["go.mod"] = "module large-project\n\ngo 1.21"

		// Generate many files to trigger performance optimizations
		for i := 0; i < 50; i++ {
			largeProjectStructure[fmt.Sprintf("pkg%d/main.go", i)] = fmt.Sprintf("package pkg%d\n\nfunc Function%d() {}", i, i)
			largeProjectStructure[fmt.Sprintf("pkg%d/utils.go", i)] = fmt.Sprintf("package pkg%d\n\ntype Struct%d struct{}", i, i)
		}

		projectPath := suite.testHelper.CreateTestProject("large-setup", largeProjectStructure)

		// Run setup with performance tuning
		result, err := suite.cliHelper.RunCommand("setup", "all",
			"--performance-tuning",
			"--optimization", "large-project",
			"--project-path", projectPath)
		suite.Require().NoError(err)
		suite.Equal(0, result.ExitCode, "Setup with performance tuning should succeed")

		// Validate performance tuning messages
		suite.Contains(result.Stdout, "Analyzing project size", "Should analyze project")
		suite.Contains(result.Stdout, "Applying large project optimizations", "Should apply optimizations")
		suite.Contains(result.Stdout, "Configuring memory limits", "Should configure memory limits")

		// Validate performance configuration
		configPath := filepath.Join(projectPath, "lsp-gateway.yaml")
		config, err := suite.testHelper.LoadEnhancedConfig(configPath)
		suite.Require().NoError(err)

		suite.Equal("large-project", config.Performance.OptimizationMode, "Should use large-project mode")
		suite.NotNil(config.Optimization.LargeProjectOptimizations, "Should have large project optimizations")
		suite.True(config.Optimization.LargeProjectOptimizations.BackgroundIndexing, "Should enable background indexing")
		suite.NotEmpty(config.Optimization.LargeProjectOptimizations.MemoryLimits, "Should have memory limits")
	})
}

// TestDiagnoseCommands tests all diagnose command variations
func (suite *CLIIntegrationTestSuite) TestDiagnoseCommands() {
	suite.Run("DiagnoseBasic", func() {
		// Create project with config
		projectStructure := map[string]string{
			"go.mod":  "module diagnose-test\n\ngo 1.21",
			"main.go": "package main\n\nfunc main() {}",
		}

		projectPath := suite.testHelper.CreateTestProject("diagnose-basic", projectStructure)

		// Generate config first
		_, err := suite.cliHelper.RunCommand("config", "generate", "--project-path", projectPath)
		suite.Require().NoError(err)

		// Run diagnose command
		result, err := suite.cliHelper.RunCommand("diagnose", "--project-path", projectPath)
		suite.Require().NoError(err)
		suite.Equal(0, result.ExitCode, "Diagnose should succeed")

		// Validate diagnostic output
		suite.Contains(result.Stdout, "System Diagnostics", "Should show diagnostic header")
		suite.Contains(result.Stdout, "Configuration: Valid", "Should validate configuration")
		suite.Contains(result.Stdout, "LSP Servers:", "Should check LSP servers")
		suite.Contains(result.Stdout, "Project Structure: Detected", "Should analyze project structure")
	})

	suite.Run("DiagnoseWithValidation", func() {
		// Create project with potentially problematic config
		projectStructure := map[string]string{
			"go.mod":  "module diagnose-validation\n\ngo 1.21",
			"main.go": "package main\n\nfunc main() {}",
		}

		projectPath := suite.testHelper.CreateTestProject("diagnose-validation", projectStructure)

		// Create invalid config for testing
		invalidConfig := `port: 8080
servers:
  - name: "invalid-server"
    languages: ["unknown-language"]
    command: "non-existent-command"
    transport: "invalid-transport"`

		configPath := filepath.Join(projectPath, "lsp-gateway.yaml")
		err := os.WriteFile(configPath, []byte(invalidConfig), 0644)
		suite.Require().NoError(err)

		// Run diagnose with validation
		result, err := suite.cliHelper.RunCommand("diagnose", "--validate", "--project-path", projectPath)
		suite.Require().NoError(err)
		// Note: Exit code might be non-zero for validation errors, but command should not fail

		// Validate diagnostic output shows problems
		suite.Contains(result.Stdout, "Configuration Issues Found", "Should detect config issues")
		suite.Contains(result.Stdout, "Unknown language: unknown-language", "Should detect unknown language")
		suite.Contains(result.Stdout, "Invalid transport: invalid-transport", "Should detect invalid transport")
		suite.Contains(result.Stdout, "Command not found: non-existent-command", "Should detect missing command")
	})

	suite.Run("DiagnoseWithPerformanceAnalysis", func() {
		// Create project for performance analysis
		projectStructure := map[string]string{
			"go.mod":  "module perf-diagnose\n\ngo 1.21",
			"main.go": "package main\n\nfunc main() {}",
		}

		projectPath := suite.testHelper.CreateTestProject("perf-diagnose", projectStructure)

		// Generate config with performance settings
		_, err := suite.cliHelper.RunCommand("config", "generate",
			"--optimization", "production",
			"--project-path", projectPath)
		suite.Require().NoError(err)

		// Run diagnose with performance analysis
		result, err := suite.cliHelper.RunCommand("diagnose", "--performance", "--project-path", projectPath)
		suite.Require().NoError(err)
		suite.Equal(0, result.ExitCode, "Performance diagnose should succeed")

		// Validate performance analysis output
		suite.Contains(result.Stdout, "Performance Analysis", "Should show performance analysis")
		suite.Contains(result.Stdout, "Memory Usage:", "Should analyze memory usage")
		suite.Contains(result.Stdout, "Request Throughput:", "Should analyze throughput")
		suite.Contains(result.Stdout, "Cache Performance:", "Should analyze cache performance")
		suite.Contains(result.Stdout, "Optimization Recommendations:", "Should provide recommendations")
	})

	suite.Run("DiagnoseWithDetailedOutput", func() {
		// Create multi-language project for detailed diagnosis
		multiLangStructure := map[string]string{
			"go.mod":         "module detailed-diagnose\n\ngo 1.21",
			"main.go":        "package main\n\nfunc main() {}",
			"pyproject.toml": `[project]\nname = "detailed-diagnose"\nversion = "1.0.0"`,
			"main.py":        "print('Hello Python')",
			"package.json":   `{"name": "detailed-diagnose", "dependencies": {"typescript": "^5.0.0"}}`,
			"tsconfig.json":  `{"compilerOptions": {"target": "ES2020"}}`,
			"src/index.ts":   "console.log('Hello TypeScript');",
		}

		projectPath := suite.testHelper.CreateTestProject("detailed-diagnose", multiLangStructure)

		// Generate comprehensive config
		_, err := suite.cliHelper.RunCommand("config", "generate", "--auto-detect", "--project-path", projectPath)
		suite.Require().NoError(err)

		// Run diagnose with detailed output
		result, err := suite.cliHelper.RunCommand("diagnose", "--detailed", "--project-path", projectPath)
		suite.Require().NoError(err)
		suite.Equal(0, result.ExitCode, "Detailed diagnose should succeed")

		// Validate detailed output
		suite.Contains(result.Stdout, "Detailed System Analysis", "Should show detailed analysis header")
		suite.Contains(result.Stdout, "Language Detection Results:", "Should show language detection details")
		suite.Contains(result.Stdout, "LSP Server Status:", "Should show server status details")
		suite.Contains(result.Stdout, "Project Structure Analysis:", "Should show structure analysis")
		suite.Contains(result.Stdout, "Configuration Validation:", "Should show config validation details")
		suite.Contains(result.Stdout, "Optimization Suggestions:", "Should provide optimization suggestions")

		// Should show information for each detected language
		suite.Contains(result.Stdout, "Go: gopls", "Should show Go LSP details")
		suite.Contains(result.Stdout, "Python: pylsp", "Should show Python LSP details")
		suite.Contains(result.Stdout, "TypeScript: typescript-language-server", "Should show TypeScript LSP details")
	})
}

// TestMigrationCommands tests configuration migration commands
func (suite *CLIIntegrationTestSuite) TestMigrationCommands() {
	suite.Run("MigrateLegacyConfig", func() {
		// Create legacy configuration
		legacyConfig := `port: 8080
timeout: 30s

servers:
  - name: "old-go-server"
    languages: ["go"]
    command: "go-langserver"  # Deprecated command
    transport: "stdio"
    
  - name: "old-python-server"
    languages: ["python"]
    command: "pyls"  # Deprecated command
    transport: "stdio"`

		projectPath := suite.testHelper.CreateTestProject("migrate-legacy", map[string]string{
			"go.mod":  "module migrate-test\n\ngo 1.21",
			"main.go": "package main\n\nfunc main() {}",
		})

		legacyConfigPath := filepath.Join(projectPath, "legacy-config.yaml")
		err := os.WriteFile(legacyConfigPath, []byte(legacyConfig), 0644)
		suite.Require().NoError(err)

		// Run migration command
		migratedConfigPath := filepath.Join(projectPath, "lsp-gateway.yaml")
		result, err := suite.cliHelper.RunCommand("config", "migrate",
			"--input", legacyConfigPath,
			"--output", migratedConfigPath)
		suite.Require().NoError(err)
		suite.Equal(0, result.ExitCode, "Migration should succeed")

		// Validate migration messages
		suite.Contains(result.Stdout, "Migrating configuration", "Should show migration progress")
		suite.Contains(result.Stdout, "Deprecated command detected: go-langserver", "Should detect deprecated commands")
		suite.Contains(result.Stdout, "Migrating to: gopls", "Should show migration target")
		suite.Contains(result.Stdout, "Migration completed successfully", "Should show completion")

		// Validate migrated configuration
		config, err := suite.testHelper.LoadEnhancedConfig(migratedConfigPath)
		suite.Require().NoError(err)

		// Should have enhanced structure
		suite.NotNil(config.Performance, "Should add performance config during migration")
		suite.NotNil(config.MultiServer, "Should add multi-server config during migration")

		// Validate server migration
		serversByName := make(map[string]bool)
		for _, server := range config.BaseConfig.Servers {
			if server.Name == "old-go-server" {
				suite.Equal("gopls", server.Command, "Should migrate go-langserver to gopls")
				serversByName["go"] = true
			}
			if server.Name == "old-python-server" {
				suite.Equal("python", server.Command, "Should migrate pyls command")
				suite.Equal([]string{"-m", "pylsp"}, server.Args, "Should add pylsp args")
				serversByName["python"] = true
			}
		}

		suite.True(serversByName["go"], "Should have migrated Go server")
		suite.True(serversByName["python"], "Should have migrated Python server")
	})

	suite.Run("MigrateWithBackup", func() {
		// Create configuration to migrate
		originalConfig := `port: 9090
servers:
  - name: "test-server"
    languages: ["go"]
    command: "old-command"
    transport: "stdio"`

		projectPath := suite.testHelper.CreateTestProject("migrate-backup", map[string]string{
			"go.mod":  "module migrate-backup\n\ngo 1.21",
			"main.go": "package main\n\nfunc main() {}",
		})

		configPath := filepath.Join(projectPath, "config.yaml")
		err := os.WriteFile(configPath, []byte(originalConfig), 0644)
		suite.Require().NoError(err)

		// Run migration with backup
		result, err := suite.cliHelper.RunCommand("config", "migrate",
			"--input", configPath,
			"--backup")
		suite.Require().NoError(err)
		suite.Equal(0, result.ExitCode, "Migration with backup should succeed")

		// Validate backup was created
		backupPath := configPath + ".backup"
		suite.FileExists(backupPath, "Should create backup file")

		// Validate backup content matches original
		backupContent, err := os.ReadFile(backupPath)
		suite.Require().NoError(err)
		suite.Equal(originalConfig, string(backupContent), "Backup should match original")

		// Validate migration occurred
		suite.Contains(result.Stdout, "Backup created:", "Should show backup creation")
		suite.Contains(result.Stdout, "Migration completed", "Should show completion")
	})
}

// TestValidationCommands tests configuration validation commands
func (suite *CLIIntegrationTestSuite) TestValidationCommands() {
	suite.Run("ValidateValidConfig", func() {
		// Create valid configuration
		projectStructure := map[string]string{
			"go.mod":  "module validate-test\n\ngo 1.21",
			"main.go": "package main\n\nfunc main() {}",
		}

		projectPath := suite.testHelper.CreateTestProject("validate-valid", projectStructure)

		// Generate valid config
		_, err := suite.cliHelper.RunCommand("config", "generate", "--project-path", projectPath)
		suite.Require().NoError(err)

		// Run validate command
		configPath := filepath.Join(projectPath, "lsp-gateway.yaml")
		result, err := suite.cliHelper.RunCommand("config", "validate", "--config", configPath)
		suite.Require().NoError(err)
		suite.Equal(0, result.ExitCode, "Validation should succeed for valid config")

		// Validate success messages
		suite.Contains(result.Stdout, "Configuration is valid", "Should show validation success")
		suite.Contains(result.Stdout, "All servers configured correctly", "Should validate servers")
		suite.Contains(result.Stdout, "No issues found", "Should show no issues")
	})

	suite.Run("ValidateInvalidConfig", func() {
		// Create invalid configuration
		invalidConfig := `port: "invalid-port"  # Should be number
timeout: "invalid-timeout"  # Should be duration

servers:
  - name: ""  # Empty name
    languages: []  # Empty languages
    command: ""  # Empty command
    transport: "invalid-transport"  # Invalid transport
    
  - name: "duplicate-server"
    languages: ["go"]
    command: "gopls"
    transport: "stdio"
    
  - name: "duplicate-server"  # Duplicate name
    languages: ["python"]
    command: "pylsp"
    transport: "stdio"`

		projectPath := suite.testHelper.CreateTestProject("validate-invalid", map[string]string{
			"go.mod":  "module validate-invalid\n\ngo 1.21",
			"main.go": "package main\n\nfunc main() {}",
		})

		configPath := filepath.Join(projectPath, "invalid-config.yaml")
		err := os.WriteFile(configPath, []byte(invalidConfig), 0644)
		suite.Require().NoError(err)

		// Run validate command on invalid config
		result, err := suite.cliHelper.RunCommand("config", "validate", "--config", configPath)
		// Command should run but report validation errors
		suite.NotEqual(0, result.ExitCode, "Validation should fail for invalid config")

		// Validate error messages
		suite.Contains(result.Stdout, "Configuration validation failed", "Should show validation failure")
		suite.Contains(result.Stdout, "Invalid port value", "Should detect invalid port")
		suite.Contains(result.Stdout, "Invalid timeout format", "Should detect invalid timeout")
		suite.Contains(result.Stdout, "Server name cannot be empty", "Should detect empty server name")
		suite.Contains(result.Stdout, "Duplicate server name", "Should detect duplicate names")
		suite.Contains(result.Stdout, "Invalid transport", "Should detect invalid transport")
	})

	suite.Run("ValidateWithRecommendations", func() {
		// Create config that works but could be improved
		suboptimalConfig := `port: 8080
timeout: 10s  # Very short timeout

servers:
  - name: "go-server"
    languages: ["go"]
    command: "gopls"
    transport: "stdio"
    # Missing optimization settings`

		projectPath := suite.testHelper.CreateTestProject("validate-recommendations", map[string]string{
			"go.mod":  "module validate-recommendations\n\ngo 1.21",
			"main.go": "package main\n\nfunc main() {}",
		})

		configPath := filepath.Join(projectPath, "suboptimal-config.yaml")
		err := os.WriteFile(configPath, []byte(suboptimalConfig), 0644)
		suite.Require().NoError(err)

		// Run validate with recommendations
		result, err := suite.cliHelper.RunCommand("config", "validate", "--config", configPath, "--recommendations")
		suite.Require().NoError(err)
		suite.Equal(0, result.ExitCode, "Validation should succeed but provide recommendations")

		// Validate recommendation messages
		suite.Contains(result.Stdout, "Configuration is valid", "Should show validation success")
		suite.Contains(result.Stdout, "Optimization Recommendations:", "Should provide recommendations")
		suite.Contains(result.Stdout, "Consider increasing timeout", "Should recommend timeout increase")
		suite.Contains(result.Stdout, "Add performance configuration", "Should recommend performance config")
		suite.Contains(result.Stdout, "Enable multi-server features", "Should recommend multi-server features")
	})
}

// Helper function to check if string slice contains item
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if strings.Contains(s, item) {
			return true
		}
	}
	return false
}

// Run the test suite
func TestCLIIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(CLIIntegrationTestSuite))
}
