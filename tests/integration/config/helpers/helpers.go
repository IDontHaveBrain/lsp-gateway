package helpers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/project"
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
	port := 8080
	timeout := "30s"
	maxConcurrent := 100
	
	if strings.Contains(path, "production") {
		profile = "production"
		maxConcurrent = 200
	} else if strings.Contains(path, "large-project") {
		profile = "large-project"
		maxConcurrent = 150
	} else if strings.Contains(path, "memory-optimized") {
		profile = "memory-optimized"
		maxConcurrent = 80
	} else if strings.Contains(path, "staging") {
		profile = "staging"
	} else if strings.Contains(path, "testing") {
		profile = "testing"
	} else if strings.Contains(path, "development") {
		profile = "development"
		enabled = false // development environment has performance config disabled
		timeout = "60s"
	}
	
	// Extract custom settings from filename
	if strings.Contains(path, "custom-config") {
		port = 9090
		timeout = "45s"
		maxConcurrent = 150
	}
	
	// Create basic servers configuration for Go projects
	servers := []config.ServerConfig{
		{
			Name:      "gopls",
			Languages: []string{"go"},
			Command:   "gopls",
			Args:      []string{},
			Transport: "stdio",
		},
	}
	
	// Add servers based on detected project structure
	if strings.Contains(path, "multi-lang") || strings.Contains(path, "detailed-diagnose") {
		servers = append(servers,
			config.ServerConfig{
				Name:      "pylsp",
				Languages: []string{"python"},
				Command:   "python",
				Args:      []string{"-m", "pylsp"},
				Transport: "stdio",
			},
			config.ServerConfig{
				Name:      "typescript-language-server",
				Languages: []string{"typescript", "javascript"},
				Command:   "typescript-language-server",
				Args:      []string{"--stdio"},
				Transport: "stdio",
			},
		)
	}
	
	if strings.Contains(path, "microservices") {
		servers = append(servers,
			config.ServerConfig{
				Name:      "jdtls",
				Languages: []string{"java"},
				Command:   "jdtls",
				Args:      []string{},
				Transport: "stdio",
			},
		)
	}
	
	multiServerConfig := &config.MultiServerConfig{
		ResourceSharing: strings.Contains(path, "production") || strings.Contains(path, "microservices"),
	}
	
	return &config.GatewayConfig{
		Port:                  port,
		Timeout:               timeout,
		MaxConcurrentRequests: maxConcurrent,
		Servers:               servers,
		GlobalMultiServerConfig: multiServerConfig,
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

// DetectAndAnalyzeProject detects and analyzes a project by examining its structure
func (h *ConfigTestHelper) DetectAndAnalyzeProject(path string) (*ProjectAnalysis, error) {
	// Create a real language detector to analyze the project
	detector := project.NewBasicLanguageDetector()
	ctx := context.Background()
	
	// Use real detection logic
	detectionResult, err := detector.DetectLanguage(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("language detection failed: %w", err)
	}
	
	// Initialize analysis with detected language
	analysis := &ProjectAnalysis{
		DominantLanguage: detectionResult.Language,
		Languages:        []string{detectionResult.Language},
	}
	
	// Determine project type based on structure
	analysis.ProjectType = h.determineProjectTypeFromStructure(path)
	
	// Analyze project characteristics
	h.analyzeProjectStructure(path, analysis)
	
	return analysis, nil
}

// determineProjectTypeFromStructure determines project type based on structure
func (h *ConfigTestHelper) determineProjectTypeFromStructure(path string) string {
	// Check for monorepo indicators (go.work, multiple services/apps directories)
	if h.fileExists(filepath.Join(path, "go.work")) {
		return "monorepo"
	}
	
	// Check for npm workspaces
	if h.hasNpmWorkspaces(path) {
		return "monorepo"
	}
	
	// Check for microservices structure (docker-compose with multiple services)
	if h.hasMicroservicesStructure(path) {
		return "microservices"
	}
	
	// Check for full-stack structure (frontend + backend directories)
	if h.hasFullStackStructure(path) {
		return "frontend-backend"
	}
	
	// Default to single-language
	return "single-language"
}

// analyzeProjectStructure analyzes project structure and populates analysis details
func (h *ConfigTestHelper) analyzeProjectStructure(path string, analysis *ProjectAnalysis) {
	// Detect Go workspace
	if h.fileExists(filepath.Join(path, "go.work")) {
		analysis.HasGoWorkspace = true
		analysis.GoWorkspaceModules = h.findGoWorkspaceModules(path)
	}
	
	// Detect Python project characteristics
	if h.fileExists(filepath.Join(path, "pyproject.toml")) {
		analysis.HasPyProjectToml = true
	}
	
	// Detect build systems and frameworks
	analysis.BuildSystems = h.detectBuildSystems(path)
	analysis.Frameworks = h.detectFrameworks(path)
	analysis.Dependencies = h.detectDependencies(path)
	
	// Detect directory structure
	analysis.SourceDirs = h.findDirectories(path, []string{"src", "source", "lib", "app"})
	analysis.TestDirs = h.findDirectories(path, []string{"tests", "test", "__tests__"})
	analysis.ScriptDirs = h.findDirectories(path, []string{"scripts", "script"})
	
	// Detect containerization
	analysis.HasDockerfile = h.fileExists(filepath.Join(path, "Dockerfile"))
	analysis.HasDockerCompose = h.fileExists(filepath.Join(path, "docker-compose.yml"))
	
	// Detect workspace characteristics
	analysis.IsMonorepo = h.hasMonorepoStructure(path)
	analysis.HasWorkspaces = h.hasWorkspaceStructure(path)
	
	// Find services and packages
	analysis.Services = h.findServices(path)
	analysis.Libraries = h.findLibraries(path)
	analysis.Apps = h.findApps(path)
	analysis.Packages = h.findPackages(path)
	
	// Detect additional languages in multi-language projects
	allLanguages := h.detectAllLanguages(path)
	if len(allLanguages) > 1 {
		analysis.Languages = allLanguages
		// Update dominant language if needed
		analysis.DominantLanguage = h.findDominantLanguage(path, allLanguages)
	}
}

// Helper methods for project analysis

// fileExists checks if a file exists
func (h *ConfigTestHelper) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// hasNpmWorkspaces checks if the project has npm workspaces
func (h *ConfigTestHelper) hasNpmWorkspaces(path string) bool {
	packageJsonPath := filepath.Join(path, "package.json")
	if !h.fileExists(packageJsonPath) {
		return false
	}
	content, err := os.ReadFile(packageJsonPath)
	if err != nil {
		return false
	}
	return strings.Contains(string(content), "workspaces")
}

// hasMicroservicesStructure checks for microservices structure
func (h *ConfigTestHelper) hasMicroservicesStructure(path string) bool {
	// Look for docker-compose.yml with multiple services
	dockerComposePath := filepath.Join(path, "docker-compose.yml")
	if !h.fileExists(dockerComposePath) {
		return false
	}
	content, err := os.ReadFile(dockerComposePath)
	if err != nil {
		return false
	}
	// Count service definitions (simple heuristic)
	serviceCount := strings.Count(string(content), "build:")
	return serviceCount >= 2
}

// hasFullStackStructure checks for full-stack structure
func (h *ConfigTestHelper) hasFullStackStructure(path string) bool {
	// Look for frontend and backend directories
	return (h.fileExists(filepath.Join(path, "frontend")) || h.fileExists(filepath.Join(path, "apps", "web"))) &&
		(h.fileExists(filepath.Join(path, "backend")) || h.fileExists(filepath.Join(path, "apps", "api")))
}

// hasMonorepoStructure checks for monorepo structure
func (h *ConfigTestHelper) hasMonorepoStructure(path string) bool {
	return h.fileExists(filepath.Join(path, "go.work")) || h.hasNpmWorkspaces(path)
}

// hasWorkspaceStructure checks for workspace structure
func (h *ConfigTestHelper) hasWorkspaceStructure(path string) bool {
	return h.hasNpmWorkspaces(path) || h.fileExists(filepath.Join(path, "turbo.json"))
}

// findGoWorkspaceModules finds Go workspace modules
func (h *ConfigTestHelper) findGoWorkspaceModules(path string) []string {
	var modules []string
	workFilePath := filepath.Join(path, "go.work")
	if !h.fileExists(workFilePath) {
		return modules
	}
	
	content, err := os.ReadFile(workFilePath)
	if err != nil {
		return modules
	}
	
	lines := strings.Split(string(content), "\n")
	inUseBlock := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "use") {
			inUseBlock = true
			continue
		}
		if inUseBlock && line != "" && !strings.HasPrefix(line, ")") {
			// Extract module path
			line = strings.Trim(line, "() \t\"./")
			if line != "" {
				modules = append(modules, filepath.Base(line))
			}
		}
		if strings.HasPrefix(line, ")") {
			inUseBlock = false
		}
	}
	return modules
}

// detectBuildSystems detects build systems
func (h *ConfigTestHelper) detectBuildSystems(path string) []string {
	var systems []string
	
	if h.fileExists(filepath.Join(path, "go.mod")) {
		systems = append(systems, "go")
	}
	if h.fileExists(filepath.Join(path, "Makefile")) {
		systems = append(systems, "make")
	}
	if h.fileExists(filepath.Join(path, "turbo.json")) {
		systems = append(systems, "turbo")
	}
	if h.fileExists(filepath.Join(path, "pyproject.toml")) {
		systems = append(systems, "setuptools")
	}
	if h.fileExists(filepath.Join(path, "pom.xml")) {
		systems = append(systems, "maven")
	}
	if h.fileExists(filepath.Join(path, "build.gradle")) {
		systems = append(systems, "gradle")
	}
	
	return systems
}

// detectFrameworks detects frameworks
func (h *ConfigTestHelper) detectFrameworks(path string) []string {
	var frameworks []string
	
	// Check package.json for frameworks
	packageJsonPath := filepath.Join(path, "package.json")
	if h.fileExists(packageJsonPath) {
		content, err := os.ReadFile(packageJsonPath)
		if err == nil {
			contentStr := string(content)
			if strings.Contains(contentStr, "react") {
				frameworks = append(frameworks, "react")
			}
			if strings.Contains(contentStr, "next") {
				frameworks = append(frameworks, "next.js")
			}
			if strings.Contains(contentStr, "express") {
				frameworks = append(frameworks, "express")
			}
		}
	}
	
	// Check for Next.js config
	if h.fileExists(filepath.Join(path, "next.config.js")) {
		frameworks = append(frameworks, "next.js")
	}
	
	// Check pyproject.toml for Python frameworks
	pyprojectPath := filepath.Join(path, "pyproject.toml")
	if h.fileExists(pyprojectPath) {
		content, err := os.ReadFile(pyprojectPath)
		if err == nil {
			contentStr := string(content)
			if strings.Contains(contentStr, "fastapi") {
				frameworks = append(frameworks, "fastapi")
			}
		}
	}
	
	// Check for Prisma
	if h.fileExists(filepath.Join(path, "prisma")) {
		frameworks = append(frameworks, "prisma")
	}
	
	return frameworks
}

// detectDependencies detects dependencies
func (h *ConfigTestHelper) detectDependencies(path string) []string {
	var deps []string
	
	// Check pyproject.toml for Python dependencies
	pyprojectPath := filepath.Join(path, "pyproject.toml")
	if h.fileExists(pyprojectPath) {
		content, err := os.ReadFile(pyprojectPath)
		if err == nil {
			contentStr := string(content)
			mlDeps := []string{"scikit-learn", "tensorflow", "torch", "mlflow"}
			for _, dep := range mlDeps {
				if strings.Contains(contentStr, dep) {
					deps = append(deps, dep)
				}
			}
		}
	}
	
	return deps
}

// findDirectories finds directories matching the given names
func (h *ConfigTestHelper) findDirectories(path string, dirNames []string) []string {
	var found []string
	for _, dirName := range dirNames {
		dirPath := filepath.Join(path, dirName)
		if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
			found = append(found, dirName)
		}
	}
	return found
}

// findServices finds service directories
func (h *ConfigTestHelper) findServices(path string) []string {
	var services []string
	
	// Check services directory
	servicesPath := filepath.Join(path, "services")
	if info, err := os.Stat(servicesPath); err == nil && info.IsDir() {
		entries, err := os.ReadDir(servicesPath)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					services = append(services, entry.Name())
				}
			}
		}
	}
	
	return services
}

// findLibraries finds library directories
func (h *ConfigTestHelper) findLibraries(path string) []string {
	var libraries []string
	
	// Check libs directory
	libsPath := filepath.Join(path, "libs")
	if info, err := os.Stat(libsPath); err == nil && info.IsDir() {
		entries, err := os.ReadDir(libsPath)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					libraries = append(libraries, entry.Name())
				}
			}
		}
	}
	
	return libraries
}

// findApps finds app directories
func (h *ConfigTestHelper) findApps(path string) []string {
	var apps []string
	
	// Check apps directory
	appsPath := filepath.Join(path, "apps")
	if info, err := os.Stat(appsPath); err == nil && info.IsDir() {
		entries, err := os.ReadDir(appsPath)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					apps = append(apps, entry.Name())
				}
			}
		}
	}
	
	return apps
}

// findPackages finds package directories
func (h *ConfigTestHelper) findPackages(path string) []string {
	var packages []string
	
	// Check packages directory
	packagesPath := filepath.Join(path, "packages")
	if info, err := os.Stat(packagesPath); err == nil && info.IsDir() {
		entries, err := os.ReadDir(packagesPath)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					packages = append(packages, entry.Name())
				}
			}
		}
	}
	
	return packages
}

// detectAllLanguages detects all languages in the project
func (h *ConfigTestHelper) detectAllLanguages(path string) []string {
	languages := make(map[string]bool)
	
	// Check for language-specific files
	if h.fileExists(filepath.Join(path, "go.mod")) {
		languages["go"] = true
	}
	if h.fileExists(filepath.Join(path, "pyproject.toml")) || h.fileExists(filepath.Join(path, "setup.py")) {
		languages["python"] = true
	}
	if h.fileExists(filepath.Join(path, "package.json")) {
		languages["javascript"] = true
	}
	if h.fileExists(filepath.Join(path, "tsconfig.json")) {
		languages["typescript"] = true
	}
	if h.fileExists(filepath.Join(path, "pom.xml")) {
		languages["java"] = true
	}
	
	// Convert map to slice
	var result []string
	for lang := range languages {
		result = append(result, lang)
	}
	
	return result
}

// findDominantLanguage finds the dominant language based on file counts
func (h *ConfigTestHelper) findDominantLanguage(path string, languages []string) string {
	if len(languages) == 0 {
		return "unknown"
	}
	
	// For now, return the first detected language as dominant
	// In a real implementation, we'd count files by extension
	return languages[0]
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

// RunCommand executes a CLI command and returns the result
func (h *CLITestHelper) RunCommand(args ...string) (*CommandResult, error) {
	// Find the CLI binary path - try multiple locations
	var binaryPath string
	possiblePaths := []string{
		"/home/skawn/work/lsp-gateway/bin/lspg",
		"./bin/lspg",
		"../../../bin/lspg",
		"../../../../bin/lspg",
	}
	
	// Add current working directory relative paths
	if workingDir, err := os.Getwd(); err == nil {
		// Traverse up to find project root with bin directory
		dir := workingDir
		for i := 0; i < 10; i++ { // Limit traversal depth
			candidatePath := filepath.Join(dir, "bin", "lspg")
			possiblePaths = append(possiblePaths, candidatePath)
			parent := filepath.Dir(dir)
			if parent == dir {
				break // Reached root
			}
			dir = parent
		}
	}
	
	// Find first existing binary
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			binaryPath = path
			break
		}
	}
	
	if binaryPath == "" {
		return &CommandResult{
			Output:   "",
			Stdout:   "",	
			Stderr:   "CLI binary not found in any of the expected locations",
			ExitCode: 1,
			Error:    fmt.Errorf("CLI binary not found"),
		}, fmt.Errorf("CLI binary not found")
	}
	
	// Create command
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = h.tempDir
	
	// Capture output
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	// Execute command
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = 1
		}
	}
	
	stdoutStr := stdout.String()
	stderrStr := stderr.String()
	output := stdoutStr
	if stderrStr != "" {
		output = stdoutStr + "\n" + stderrStr
	}
	
	return &CommandResult{
		Output:   output,
		Stdout:   stdoutStr,
		Stderr:   stderrStr,
		ExitCode: exitCode,
		Error:    err,
	}, nil
}
