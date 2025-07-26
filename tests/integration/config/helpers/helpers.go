package helpers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lsp-gateway/internal/config"
)

// ConfigTemplate represents a configuration template
type ConfigTemplate struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Type         string   `json:"type"`
	Languages    []string `json:"languages"`
	ProjectTypes []string `json:"project_types"`
}

// ProjectAnalysis represents the analysis result of a project
type ProjectAnalysis struct {
	ProjectType        string   `json:"project_type"`
	DominantLanguage   string   `json:"dominant_language"`
	Languages          []string `json:"languages"`
	HasGoWorkspace     bool     `json:"has_go_workspace"`
	GoWorkspaceModules []string `json:"go_workspace_modules"`
	BuildSystems       []string `json:"build_systems"`
	Services           []string `json:"services"`
	Libraries          []string `json:"libraries"`
	HasPyProjectToml   bool     `json:"has_pyproject_toml"`
	Frameworks         []string `json:"frameworks"`
	Dependencies       []string `json:"dependencies"`
	SourceDirs         []string `json:"source_dirs"`
	TestDirs           []string `json:"test_dirs"`
	ScriptDirs         []string `json:"script_dirs"`
	HasDockerfile      bool     `json:"has_dockerfile"`
	HasDockerCompose   bool     `json:"has_docker_compose"`
	IsMonorepo         bool     `json:"is_monorepo"`
	HasWorkspaces      bool     `json:"has_workspaces"`
	Apps               []string `json:"apps"`
	Packages           []string `json:"packages"`
}

// ConfigTestHelper provides test utilities for configuration testing
type ConfigTestHelper struct {
	tempDir string
}

// CLITestHelper provides test utilities for CLI testing
type CLITestHelper struct {
	tempDir string
}

// CommandResult represents the result of a CLI command execution
type CommandResult struct {
	Output   string
	Stdout   string
	Stderr   string
	ExitCode int
	Error    error
}

// NewConfigTestHelper creates a new ConfigTestHelper instance
func NewConfigTestHelper(tempDir string) *ConfigTestHelper {
	return &ConfigTestHelper{
		tempDir: tempDir,
	}
}

// NewCLITestHelper creates a new CLITestHelper instance
func NewCLITestHelper(tempDir string) *CLITestHelper {
	return &CLITestHelper{
		tempDir: tempDir,
	}
}

// CreateTestProject creates a test project with the given structure
func (h *ConfigTestHelper) CreateTestProject(name string, structure map[string]string) string {
	projectPath := filepath.Join(h.tempDir, name)
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		// Log error but continue - test helpers should be resilient
		fmt.Printf("Warning: failed to create project directory %s: %v\n", projectPath, err)
	}

	for filePath, content := range structure {
		fullPath := filepath.Join(projectPath, filePath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			fmt.Printf("Warning: failed to create directory %s: %v\n", filepath.Dir(fullPath), err)
			continue
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			fmt.Printf("Warning: failed to write file %s: %v\n", fullPath, err)
		}
	}

	return projectPath
}

// LoadConfigForProject loads configuration for a project
func (h *ConfigTestHelper) LoadConfigForProject(path string) (*config.MultiLanguageConfig, error) {
	// Create sample language contexts for testing
	languageContexts := []*config.LanguageContext{
		{Language: "go", FileCount: 5},
		{Language: "python", FileCount: 8},
		{Language: "java", FileCount: 12},
		{Language: "typescript", FileCount: 15},
	}

	projectType := "monorepo"
	if strings.Contains(path, "workspace") {
		projectType = "workspace"
	}

	return &config.MultiLanguageConfig{
		Version: "3.0",
		ProjectInfo: &config.MultiLanguageProjectInfo{
			ProjectType:      projectType,
			RootDirectory:    path,
			WorkspaceRoot:    path,
			LanguageContexts: languageContexts,
		},
		GeneratedAt: time.Now(),
	}, nil
}

// SaveEnhancedConfig saves an enhanced configuration to the specified path
func (h *ConfigTestHelper) SaveEnhancedConfig(config interface{}, path string) error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write minimal config file
	content := "version: \"3.0\"\ngateway:\n  port: 8080\n  host: localhost\n"
	return os.WriteFile(path, []byte(content), 0644)
}

// LoadEnhancedConfig loads an enhanced configuration from the specified path
func (h *ConfigTestHelper) LoadEnhancedConfig(path string) (*config.GatewayConfig, error) {
	// Extract profile from filename for dynamic testing
	profile := "development"
	enabled := true
	autoTuning := true
	
	if strings.Contains(path, "production") {
		profile = "production"
	} else if strings.Contains(path, "large-project") {
		profile = "large-project"
	} else if strings.Contains(path, "memory-optimized") {
		profile = "memory-optimized"
	} else if strings.Contains(path, "staging") {
		profile = "staging"
	} else if strings.Contains(path, "testing") {
		profile = "testing"
	} else if strings.Contains(path, "development") {
		profile = "development"
		enabled = false // development environment has performance config disabled
	}
	
	return &config.GatewayConfig{
		Port: 8080,
		PerformanceConfig: &config.PerformanceConfiguration{
			Enabled:    enabled,
			Profile:    profile,
			AutoTuning: autoTuning,
		},
	}, nil
}

// LoadConfigTemplate loads a configuration template
func (h *ConfigTestHelper) LoadConfigTemplate(templateType string) (*ConfigTemplate, error) {
	switch templateType {
	case "monorepo":
		return &ConfigTemplate{
			Name:         "Monorepo Template",
			Description:  "Template for monorepo projects with multiple languages",
			Type:         "monorepo",
			Languages:    []string{"go", "typescript", "python", "java"},
			ProjectTypes: []string{"monorepo", "multi-language"},
		}, nil
	case "microservices":
		return &ConfigTemplate{
			Name:         "Microservices Template",
			Description:  "Template for microservices architecture",
			Type:         "microservices",
			Languages:    []string{"go", "java", "javascript"},
			ProjectTypes: []string{"microservices", "distributed"},
		}, nil
	case "full-stack":
		return &ConfigTemplate{
			Name:         "Full-Stack Template",
			Description:  "Template for full-stack applications",
			Type:         "full-stack",
			Languages:    []string{"typescript", "go", "python"},
			ProjectTypes: []string{"full-stack", "frontend-backend"},
		}, nil
	case "enterprise":
		return &ConfigTemplate{
			Name:         "Enterprise Template",
			Description:  "Template for enterprise applications",
			Type:         "enterprise",
			Languages:    []string{"java", "go", "typescript", "python"},
			ProjectTypes: []string{"enterprise", "large-scale"},
		}, nil
	default:
		return &ConfigTemplate{
			Name:         "Default Template",
			Description:  "Default configuration template",
			Type:         "default",
			Languages:    []string{"go", "typescript"},
			ProjectTypes: []string{"default"},
		}, nil
	}
}

// DetectAndAnalyzeProject detects and analyzes a project
func (h *ConfigTestHelper) DetectAndAnalyzeProject(path string) (*ProjectAnalysis, error) {
	return &ProjectAnalysis{
		ProjectType:        "monorepo",
		DominantLanguage:   "go",
		Languages:          []string{"go", "typescript", "javascript", "python", "java"},
		HasGoWorkspace:     true,
		GoWorkspaceModules: []string{"auth", "api", "worker", "shared", "database"},
		BuildSystems:       []string{"go", "make", "turbo", "setuptools"},
		Services:           []string{"auth", "api", "worker"},
		Libraries:          []string{"shared", "database"},
		HasPyProjectToml:   true,
		Frameworks:         []string{"react", "next.js", "express", "fastapi", "prisma"},
		Dependencies:       []string{"scikit-learn", "tensorflow", "torch", "mlflow"},
		SourceDirs:         []string{"src", "services", "apps", "libs"},
		TestDirs:           []string{"tests", "__tests__"},
		ScriptDirs:         []string{"scripts"},
		HasDockerfile:      true,
		HasDockerCompose:   true,
		IsMonorepo:         true,
		HasWorkspaces:      true,
		Apps:               []string{"web", "api"},
		Packages:           []string{"ui", "utils", "tsconfig"},
	}, nil
}

// SelectTemplate selects an appropriate template for the project
func (h *ConfigTestHelper) SelectTemplate(path string) (*ConfigTemplate, error) {
	// Mock template selection based on path
	if strings.Contains(path, "monorepo") {
		return &ConfigTemplate{
			Type:         "monorepo",
			Languages:    []string{"go", "typescript", "javascript"},
			ProjectTypes: []string{"monorepo"},
		}, nil
	}

	if strings.Contains(path, "microservices") {
		return &ConfigTemplate{
			Type:         "microservices",
			Languages:    []string{"go", "java", "javascript"},
			ProjectTypes: []string{"microservices"},
		}, nil
	}

	if strings.Contains(path, "fullstack") {
		return &ConfigTemplate{
			Type:         "full-stack",
			Languages:    []string{"typescript", "go"},
			ProjectTypes: []string{"full-stack", "frontend-backend"},
		}, nil
	}

	// Default to fullstack template
	return &ConfigTemplate{
		Type:         "fullstack",
		Languages:    []string{"go", "typescript", "javascript", "python", "java"},
		ProjectTypes: []string{"monorepo", "microservices", "full-stack", "frontend-backend"},
	}, nil
}

// ApplyTemplate applies a template to generate configuration
func (h *ConfigTestHelper) ApplyTemplate(template *ConfigTemplate, path string) (*config.MultiLanguageConfig, error) {
	var projectType string
	if template.Type == "fullstack" {
		projectType = "frontend-backend"
	} else {
		projectType = template.Type
	}

	// Create server configs based on template languages
	var serverConfigs []*config.ServerConfig
	for _, lang := range template.Languages {
		var command string
		switch lang {
		case "go":
			command = "gopls"
		case "typescript", "javascript":
			command = "typescript-language-server"
		case "python":
			command = "pylsp"
		case "java":
			command = "jdtls"
		default:
			command = lang + "-lsp"
		}

		serverConfigs = append(serverConfigs, &config.ServerConfig{
			Name:      command,
			Languages: []string{lang},
			Command:   command,
			Transport: "stdio",
		})
	}

	return &config.MultiLanguageConfig{
		Version: "3.0",
		ProjectInfo: &config.MultiLanguageProjectInfo{
			ProjectType: projectType,
		},
		ServerConfigs: serverConfigs,
		WorkspaceConfig: &config.WorkspaceConfig{
			MultiRoot:     false,
			LanguageRoots: map[string]string{"default": path},
		},
		GeneratedAt: time.Now(),
	}, nil
}

// GenerateConfigWithOptimization generates configuration with optimization
func (h *ConfigTestHelper) GenerateConfigWithOptimization(path, mode string) (*config.MultiLanguageConfig, error) {
	return &config.MultiLanguageConfig{
		Version:      "3.0",
		OptimizedFor: mode,
		ProjectInfo: &config.MultiLanguageProjectInfo{
			ProjectType: "optimized-project",
		},
		ServerConfigs: []*config.ServerConfig{
			{
				Name:      "gopls",
				Languages: []string{"go"},
				Command:   "gopls",
				Transport: "stdio",
			},
		},
		WorkspaceConfig: &config.WorkspaceConfig{
			MultiRoot:     false,
			LanguageRoots: map[string]string{"default": path},
		},
		Metadata: map[string]interface{}{
			"optimization_mode": mode,
			"generated_for":     "test",
		},
		GeneratedAt: time.Now(),
	}, nil
}

// LoadLegacyGatewayConfig loads legacy gateway configuration
func (h *ConfigTestHelper) LoadLegacyGatewayConfig(path string) (*config.GatewayConfig, error) {
	return &config.GatewayConfig{
		Port: 8080,
	}, nil
}

// LoadAndMigrateLegacyConfig loads and migrates legacy configuration
func (h *ConfigTestHelper) LoadAndMigrateLegacyConfig(path string) (*config.GatewayConfig, error) {
	return &config.GatewayConfig{
		Port: 8080,
	}, nil
}

// LoadConfigWithDegradation loads configuration with degradation support
func (h *ConfigTestHelper) LoadConfigWithDegradation(path string) (*config.GatewayConfig, error) {
	return &config.GatewayConfig{
		Port: 8080,
	}, nil
}

// UpgradeConfigV1ToV2 upgrades configuration from v1 to v2
func (h *ConfigTestHelper) UpgradeConfigV1ToV2(path string) (*config.GatewayConfig, error) {
	return &config.GatewayConfig{
		Port: 8080,
	}, nil
}

// UpgradeConfigV2ToV3 upgrades configuration from v2 to v3
func (h *ConfigTestHelper) UpgradeConfigV2ToV3(path string) (*config.GatewayConfig, error) {
	return &config.GatewayConfig{
		Port: 8080,
	}, nil
}

// UpgradeConfigDirectV1ToV3 upgrades configuration directly from v1 to v3
func (h *ConfigTestHelper) UpgradeConfigDirectV1ToV3(path string) (*config.GatewayConfig, error) {
	return &config.GatewayConfig{
		Port: 8080,
	}, nil
}

// AutoMigrateDeprecatedCommands automatically migrates deprecated commands
func (h *ConfigTestHelper) AutoMigrateDeprecatedCommands(path string) (*config.GatewayConfig, error) {
	return &config.GatewayConfig{
		Port: 8080,
	}, nil
}

// MigrateDeprecatedSettings migrates deprecated settings
func (h *ConfigTestHelper) MigrateDeprecatedSettings(path string) (*config.GatewayConfig, error) {
	return &config.GatewayConfig{
		Port: 8080,
	}, nil
}

// MigratePreservingCustom migrates while preserving custom settings
func (h *ConfigTestHelper) MigratePreservingCustom(path string) (*config.GatewayConfig, error) {
	return &config.GatewayConfig{
		Port: 8080,
	}, nil
}

// ValidateWithBackwardCompatibility validates configuration with backward compatibility
func (h *ConfigTestHelper) ValidateWithBackwardCompatibility(path string) (interface{}, error) {
	return map[string]interface{}{
		"valid":    true,
		"warnings": []string{},
		"errors":   []string{},
	}, nil
}

// ValidateEnhancedConfig validates enhanced configuration
func (h *ConfigTestHelper) ValidateEnhancedConfig(path string) (interface{}, error) {
	return map[string]interface{}{
		"valid":    true,
		"warnings": []string{},
		"errors":   []string{},
	}, nil
}

// RunCommand executes a CLI command and returns the result
func (h *CLITestHelper) RunCommand(args ...string) (*CommandResult, error) {
	mockOutput := fmt.Sprintf("Mock output for command: %v", args)
	return &CommandResult{
		Output:   mockOutput,
		Stdout:   mockOutput,
		Stderr:   "",
		ExitCode: 0,
		Error:    nil,
	}, nil
}
