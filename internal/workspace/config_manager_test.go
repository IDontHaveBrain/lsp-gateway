package workspace

import (
	"context"
	"fmt"
	"lsp-gateway/internal/config"
	"lsp-gateway/internal/project"
	"lsp-gateway/internal/project/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWorkspaceConfigManager_GenerateAndLoad(t *testing.T) {
	testDir := t.TempDir()
	workspaceRoot := filepath.Join(testDir, "test-workspace")
	
	if err := os.MkdirAll(workspaceRoot, 0755); err != nil {
		t.Fatalf("Failed to create test workspace: %v", err)
	}

	goModContent := `module test-workspace
go 1.21
`
	goModPath := filepath.Join(workspaceRoot, "go.mod")
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	wcm := NewWorkspaceConfigManagerWithOptions(&WorkspaceConfigManagerOptions{
		BaseConfigDir: testDir,
	})

	detector := project.NewProjectDetector()
	projectContext, err := detector.DetectProject(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("Failed to detect project: %v", err)
	}

	if err := wcm.GenerateWorkspaceConfig(workspaceRoot, projectContext); err != nil {
		t.Fatalf("Failed to generate workspace config: %v", err)
	}

	config, err := wcm.LoadWorkspaceConfig(workspaceRoot)
	if err != nil {
		t.Fatalf("Failed to load workspace config: %v", err)
	}

	if config.Workspace.RootPath != workspaceRoot {
		t.Errorf("Expected root path %s, got %s", workspaceRoot, config.Workspace.RootPath)
	}

	if config.Workspace.ProjectType != project.PROJECT_TYPE_GO {
		t.Errorf("Expected project type %s, got %s", project.PROJECT_TYPE_GO, config.Workspace.ProjectType)
	}

	if len(config.Servers) == 0 {
		t.Error("Expected at least one server configuration")
	}

	if err := wcm.ValidateWorkspaceConfig(config); err != nil {
		t.Errorf("Workspace config validation failed: %v", err)
	}
}

func TestWorkspaceConfigManager_DirectoryManagement(t *testing.T) {
	testDir := t.TempDir()
	workspaceRoot := filepath.Join(testDir, "test-workspace")
	
	wcm := NewWorkspaceConfigManagerWithOptions(&WorkspaceConfigManagerOptions{
		BaseConfigDir: testDir,
	})

	if err := wcm.CreateWorkspaceDirectories(workspaceRoot); err != nil {
		t.Fatalf("Failed to create workspace directories: %v", err)
	}

	workspaceDir := wcm.GetWorkspaceDirectory(workspaceRoot)
	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		t.Errorf("Workspace directory was not created: %s", workspaceDir)
	}

	cacheDir := filepath.Join(workspaceDir, WorkspaceCacheDirName)
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		t.Errorf("Cache directory was not created: %s", cacheDir)
	}

	logsDir := filepath.Join(workspaceDir, WorkspaceLogDirName)
	if _, err := os.Stat(logsDir); os.IsNotExist(err) {
		t.Errorf("Logs directory was not created: %s", logsDir)
	}

	indexDir := filepath.Join(workspaceDir, WorkspaceIndexDirName)
	if _, err := os.Stat(indexDir); os.IsNotExist(err) {
		t.Errorf("Index directory was not created: %s", indexDir)
	}

	if err := wcm.CleanupWorkspace(workspaceRoot); err != nil {
		t.Errorf("Failed to cleanup workspace: %v", err)
	}

	if _, err := os.Stat(workspaceDir); !os.IsNotExist(err) {
		t.Errorf("Workspace directory was not cleaned up: %s", workspaceDir)
	}
}

func TestWorkspaceConfigManager_Validation(t *testing.T) {
	wcm := NewWorkspaceConfigManager()

	if err := wcm.ValidateWorkspaceConfig(nil); err == nil {
		t.Error("Expected validation error for nil config")
	}

	invalidConfig := &WorkspaceConfig{
		Workspace: WorkspaceInfo{
			WorkspaceID: "",
			RootPath:    "",
		},
		Servers: make(map[string]*config.ServerConfig),
	}

	if err := wcm.ValidateWorkspaceConfig(invalidConfig); err == nil {
		t.Error("Expected validation error for invalid config")
	}

	validConfig := &WorkspaceConfig{
		Workspace: WorkspaceInfo{
			WorkspaceID: "test-workspace",
			RootPath:    "/tmp",
			CreatedAt:   time.Now(),
		},
		Servers: map[string]*config.ServerConfig{
			"test-server": {
				Name:    "test-server",
				Command: "test-command",
			},
		},
	}

	if err := wcm.ValidateWorkspaceConfig(validConfig); err != nil {
		t.Errorf("Expected valid config to pass validation: %v", err)
	}
}

func TestWorkspaceConfigManager_GetWorkspaceDirectory(t *testing.T) {
	testDir := t.TempDir()
	wcm := NewWorkspaceConfigManagerWithOptions(&WorkspaceConfigManagerOptions{
		BaseConfigDir: testDir,
	})

	workspaceRoot1 := "/path/to/workspace1"
	workspaceRoot2 := "/path/to/workspace2"

	dir1 := wcm.GetWorkspaceDirectory(workspaceRoot1)
	dir2 := wcm.GetWorkspaceDirectory(workspaceRoot2)

	if dir1 == dir2 {
		t.Error("Different workspace roots should generate different workspace directories")
	}

	if !filepath.IsAbs(dir1) {
		t.Errorf("Workspace directory should be absolute path: %s", dir1)
	}

	if !filepath.IsAbs(dir2) {
		t.Errorf("Workspace directory should be absolute path: %s", dir2)
	}

	expectedPrefix := filepath.Join(testDir, "workspaces")
	if !filepath.HasPrefix(dir1, expectedPrefix) {
		t.Errorf("Workspace directory should be under %s, got %s", expectedPrefix, dir1)
	}
}

// TestWorkspaceConfigManager_MultiProjectGeneration tests multi-project workspace generation
func TestWorkspaceConfigManager_MultiProjectGeneration(t *testing.T) {
	testDir := t.TempDir()
	workspaceRoot := filepath.Join(testDir, "multi-workspace")
	
	// Create multi-project test workspace structure
	workspaceStructure := map[string]string{
		// Go service
		"go-service/go.mod": `module go-service
go 1.21
`,
		"go-service/main.go": `package main

import "fmt"

func main() {
	fmt.Println("Hello from Go service")
}
`,
		// Python API
		"python-api/setup.py": `from setuptools import setup

setup(name="python-api", version="1.0.0")
`,
		"python-api/main.py": `def main():
	print("Hello from Python API")

if __name__ == "__main__":
	main()
`,
		// TypeScript frontend
		"frontend/package.json": `{
	"name": "frontend",
	"version": "1.0.0",
	"dependencies": {
		"typescript": "^4.0.0"
	}
}
`,
		"frontend/tsconfig.json": `{
	"compilerOptions": {
		"target": "ES2020",
		"module": "commonjs"
	}
}
`,
		"frontend/index.ts": `console.log("Hello from TypeScript frontend");
`,
		// Mixed resources folder
		"shared/README.md": "# Shared resources",
	}
	
	if err := createTestWorkspaceStructure(workspaceRoot, workspaceStructure); err != nil {
		t.Fatalf("Failed to create test workspace structure: %v", err)
	}

	wcm := NewWorkspaceConfigManagerWithOptions(&WorkspaceConfigManagerOptions{
		BaseConfigDir: testDir,
	})

	// Detect the workspace (should detect as mixed since it has multiple projects)
	detector := project.NewProjectDetector()
	projectContext, err := detector.DetectProject(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("Failed to detect project: %v", err)
	}

	// Generate workspace configuration
	if err := wcm.GenerateWorkspaceConfig(workspaceRoot, projectContext); err != nil {
		t.Fatalf("Failed to generate workspace config: %v", err)
	}

	// Load and validate the generated configuration
	config, err := wcm.LoadWorkspaceConfig(workspaceRoot)
	if err != nil {
		t.Fatalf("Failed to load workspace config: %v", err)
	}

	// Test workspace-level assertions
	if config.Workspace.RootPath != workspaceRoot {
		t.Errorf("Expected workspace root %s, got %s", workspaceRoot, config.Workspace.RootPath)
	}

	// Should detect as mixed project type due to multiple sub-projects
	expectedProjectType := types.PROJECT_TYPE_MIXED
	if config.Workspace.ProjectType != expectedProjectType {
		t.Errorf("Expected project type %s, got %s", expectedProjectType, config.Workspace.ProjectType)
	}

	// Test sub-project detection
	expectedSubProjects := 3 // go-service, python-api, frontend
	if len(config.SubProjects) < expectedSubProjects {
		t.Errorf("Expected at least %d sub-projects, got %d", expectedSubProjects, len(config.SubProjects))
	}

	// Test language aggregation
	expectedLanguages := []string{types.PROJECT_TYPE_GO, types.PROJECT_TYPE_PYTHON, types.PROJECT_TYPE_TYPESCRIPT}
	for _, expectedLang := range expectedLanguages {
		found := false
		for _, configLang := range config.Workspace.Languages {
			if configLang == expectedLang {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected language '%s' not found in workspace languages: %v", expectedLang, config.Workspace.Languages)
		}
	}

	// Test server configurations for multiple project types
	expectedServers := []string{"gopls", "pylsp", "typescript-language-server"}
	for _, expectedServer := range expectedServers {
		if _, exists := config.Servers[expectedServer]; !exists {
			t.Errorf("Expected server '%s' not found in server configurations", expectedServer)
		}
	}

	// Test resource quota assignment
	totalMaxMemory := 0
	for _, subProject := range config.SubProjects {
		if subProject.ResourceQuota != nil {
			totalMaxMemory += subProject.ResourceQuota.MaxMemoryMB
		}
	}
	if totalMaxMemory == 0 {
		t.Error("Expected resource quotas to be assigned to sub-projects")
	}

	// Test server workspace root mapping
	for serverName, serverConfig := range config.Servers {
		if len(serverConfig.WorkspaceRoots) == 0 {
			t.Errorf("Server '%s' should have workspace roots configured", serverName)
		}
	}

	// Validate the entire configuration
	if err := wcm.ValidateWorkspaceConfig(config); err != nil {
		t.Errorf("Multi-project workspace config validation failed: %v", err)
	}
}

// TestWorkspaceConfigManager_SubProjectConfigs tests sub-project configuration methods
func TestWorkspaceConfigManager_SubProjectConfigs(t *testing.T) {
	testDir := t.TempDir()
	workspaceRoot := filepath.Join(testDir, "sub-project-test")
	
	// Create test project contexts for sub-projects
	subProjectContexts := []*project.ProjectContext{
		{
			ProjectType:      types.PROJECT_TYPE_GO,
			RootPath:         filepath.Join(workspaceRoot, "service-a"),
			WorkspaceRoot:    workspaceRoot,
			Languages:        []string{types.PROJECT_TYPE_GO},
			PrimaryLanguage:  types.PROJECT_TYPE_GO,
			ModuleName:       "service-a",
			DisplayName:      "Service A",
			RequiredServers:  []string{"gopls"},
			Dependencies:     map[string]string{"gin": "1.9.0"},
			MarkerFiles:      []string{"go.mod"},
		},
		{
			ProjectType:      types.PROJECT_TYPE_PYTHON,
			RootPath:         filepath.Join(workspaceRoot, "api-service"),
			WorkspaceRoot:    workspaceRoot,
			Languages:        []string{types.PROJECT_TYPE_PYTHON},
			PrimaryLanguage:  types.PROJECT_TYPE_PYTHON,
			ModuleName:       "api-service",
			DisplayName:      "API Service",
			RequiredServers:  []string{"pylsp"},
			Dependencies:     map[string]string{"fastapi": "0.68.0"},
			MarkerFiles:      []string{"setup.py"},
		},
	}

	// Create actual directories
	for _, ctx := range subProjectContexts {
		if err := os.MkdirAll(ctx.RootPath, 0755); err != nil {
			t.Fatalf("Failed to create sub-project directory %s: %v", ctx.RootPath, err)
		}
	}

	wcm := NewWorkspaceConfigManagerWithOptions(&WorkspaceConfigManagerOptions{
		BaseConfigDir: testDir,
	})

	// Test GenerateSubProjectConfigs
	subProjectConfigs, err := wcm.GenerateSubProjectConfigs(workspaceRoot, subProjectContexts)
	if err != nil {
		t.Fatalf("Failed to generate sub-project configs: %v", err)
	}

	if len(subProjectConfigs) != len(subProjectContexts) {
		t.Errorf("Expected %d sub-project configs, got %d", len(subProjectContexts), len(subProjectConfigs))
	}

	// Test configuration properties
	for subProjectID, subProject := range subProjectConfigs {
		// Test basic properties
		if subProject.ID == "" {
			t.Errorf("Sub-project ID should not be empty")
		}
		if subProject.AbsolutePath == "" {
			t.Errorf("Sub-project absolute path should not be empty")
		}
		if len(subProject.Languages) == 0 {
			t.Errorf("Sub-project should have at least one language")
		}
		if subProject.ResourceQuota == nil {
			t.Errorf("Sub-project should have resource quota assigned")
		}

		// Test ValidateSubProjectConfig
		if err := wcm.ValidateSubProjectConfig(subProject); err != nil {
			t.Errorf("Sub-project config validation failed for %s: %v", subProjectID, err)
		}
	}

	// Create a workspace configuration with sub-projects for GetSubProjectConfig test
	detector := project.NewProjectDetector()
	projectContext, err := detector.DetectProject(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("Failed to detect project: %v", err)
	}

	if err := wcm.GenerateWorkspaceConfig(workspaceRoot, projectContext); err != nil {
		t.Fatalf("Failed to generate workspace config: %v", err)
	}

	// Test GetSubProjectConfig
	config, err := wcm.LoadWorkspaceConfig(workspaceRoot)
	if err != nil {
		t.Fatalf("Failed to load workspace config: %v", err)
	}

	if len(config.SubProjects) > 0 {
		firstSubProject := config.SubProjects[0]
		retrievedSubProject, err := wcm.GetSubProjectConfig(workspaceRoot, firstSubProject.ID)
		if err != nil {
			t.Errorf("Failed to get sub-project config: %v", err)
		}

		if retrievedSubProject.ID != firstSubProject.ID {
			t.Errorf("Retrieved sub-project ID mismatch: expected %s, got %s", firstSubProject.ID, retrievedSubProject.ID)
		}
	}

	// Test GetSubProjectConfig with invalid ID
	_, err = wcm.GetSubProjectConfig(workspaceRoot, "invalid-id")
	if err == nil {
		t.Error("Expected error when getting sub-project with invalid ID")
	}
}

// TestWorkspaceConfigManager_MultiProjectValidation tests validation with multi-project scenarios
func TestWorkspaceConfigManager_MultiProjectValidation(t *testing.T) {
	testDir := t.TempDir()
	wcm := NewWorkspaceConfigManagerWithOptions(&WorkspaceConfigManagerOptions{
		BaseConfigDir: testDir,
	})

	t.Run("ValidMultiProjectConfiguration", func(t *testing.T) {
		workspaceRoot := filepath.Join(testDir, "valid-workspace")
		if err := os.MkdirAll(workspaceRoot, 0755); err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}

		validConfig := &WorkspaceConfig{
			Workspace: WorkspaceInfo{
				WorkspaceID: "test-workspace",
				RootPath:    workspaceRoot,
				ProjectType: types.PROJECT_TYPE_MIXED,
				Languages:   []string{types.PROJECT_TYPE_GO, types.PROJECT_TYPE_PYTHON},
				CreatedAt:   time.Now(),
			},
			SubProjects: []*SubProjectInfo{
				{
					ID:           "go-project-1",
					Name:         "Go Service",
					AbsolutePath: filepath.Join(workspaceRoot, "go-service"),
					ProjectType:  types.PROJECT_TYPE_GO,
					Languages:    []string{types.PROJECT_TYPE_GO},
					RequiredServers: []string{"gopls"},
					ResourceQuota: &ResourceQuota{
						MaxMemoryMB:    512,
						MaxConcurrency: 8,
						MaxCacheSize:   100,
					},
				},
				{
					ID:           "python-project-1",
					Name:         "Python API",
					AbsolutePath: filepath.Join(workspaceRoot, "python-api"),
					ProjectType:  types.PROJECT_TYPE_PYTHON,
					Languages:    []string{types.PROJECT_TYPE_PYTHON},
					RequiredServers: []string{"pylsp"},
					ResourceQuota: &ResourceQuota{
						MaxMemoryMB:    256,
						MaxConcurrency: 4,
						MaxCacheSize:   75,
					},
				},
			},
			Servers: map[string]*config.ServerConfig{
				"gopls": {
					Name:      "gopls",
					Command:   "gopls",
					Languages: []string{types.PROJECT_TYPE_GO},
				},
				"pylsp": {
					Name:      "pylsp",
					Command:   "pylsp",
					Languages: []string{types.PROJECT_TYPE_PYTHON},
				},
			},
		}

		// Create sub-project directories
		for _, subProject := range validConfig.SubProjects {
			if err := os.MkdirAll(subProject.AbsolutePath, 0755); err != nil {
				t.Fatalf("Failed to create sub-project directory: %v", err)
			}
		}

		if err := wcm.ValidateWorkspaceConfig(validConfig); err != nil {
			t.Errorf("Valid multi-project config should pass validation: %v", err)
		}
	})

	t.Run("ValidationErrorAggregation", func(t *testing.T) {
		invalidConfig := &WorkspaceConfig{
			Workspace: WorkspaceInfo{
				WorkspaceID: "", // Invalid: empty workspace ID
				RootPath:    "",  // Invalid: empty root path
			},
			SubProjects: []*SubProjectInfo{
				{
					ID:           "duplicate-id",
					Name:         "Project 1",
					AbsolutePath: "/nonexistent/path1", // Invalid: path doesn't exist
					ProjectType:  "",                   // Invalid: empty project type
					Languages:    []string{},           // Invalid: no languages
				},
				{
					ID:           "duplicate-id", // Invalid: duplicate ID
					Name:         "Project 2",
					AbsolutePath: "/nonexistent/path2", // Invalid: path doesn't exist
					ProjectType:  types.PROJECT_TYPE_GO,
					Languages:    []string{types.PROJECT_TYPE_GO},
					RequiredServers: []string{"nonexistent-server"}, // Invalid: server doesn't exist
				},
			},
			Servers: map[string]*config.ServerConfig{}, // Invalid: no servers configured
		}

		err := wcm.ValidateWorkspaceConfig(invalidConfig)
		if err == nil {
			t.Error("Expected validation errors for invalid config")
		}

		errorMessage := err.Error()
		expectedErrors := []string{
			"workspace ID cannot be empty",
			"workspace root path cannot be empty",
			"workspace configuration must have at least one server",
			"Duplicate sub-project ID",
			"required server 'nonexistent-server' not found",
		}

		for _, expectedError := range expectedErrors {
			if !strings.Contains(errorMessage, expectedError) {
				t.Errorf("Expected error message to contain '%s', got: %s", expectedError, errorMessage)
			}
		}
	})

	t.Run("ResourceQuotaLimitWarnings", func(t *testing.T) {
		workspaceRoot := filepath.Join(testDir, "quota-test-workspace")
		if err := os.MkdirAll(workspaceRoot, 0755); err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}

		// Create sub-projects with excessive resource quotas
		subProjectDir := filepath.Join(workspaceRoot, "high-resource-project")
		if err := os.MkdirAll(subProjectDir, 0755); err != nil {
			t.Fatalf("Failed to create sub-project directory: %v", err)
		}

		configWithHighQuotas := &WorkspaceConfig{
			Workspace: WorkspaceInfo{
				WorkspaceID: "quota-test",
				RootPath:    workspaceRoot,
				ProjectType: types.PROJECT_TYPE_GO,
				Languages:   []string{types.PROJECT_TYPE_GO},
				CreatedAt:   time.Now(),
			},
			SubProjects: []*SubProjectInfo{
				{
					ID:           "high-resource-project",
					Name:         "High Resource Project",
					AbsolutePath: subProjectDir,
					ProjectType:  types.PROJECT_TYPE_GO,
					Languages:    []string{types.PROJECT_TYPE_GO},
					RequiredServers: []string{"gopls"},
					ResourceQuota: &ResourceQuota{
						MaxMemoryMB:    40000, // Exceeds reasonable limit
						MaxConcurrency: 150,   // Exceeds reasonable limit
						MaxCacheSize:   15000, // Exceeds reasonable limit
					},
				},
			},
			Servers: map[string]*config.ServerConfig{
				"gopls": {
					Name:      "gopls",
					Command:   "gopls",
					Languages: []string{types.PROJECT_TYPE_GO},
				},
			},
		}

		// Should pass validation but generate warnings
		if err := wcm.ValidateWorkspaceConfig(configWithHighQuotas); err != nil {
			t.Errorf("Config with high quotas should pass validation (with warnings): %v", err)
		}
	})
}

// TestWorkspaceConfigManager_ServerConfigurationIntegration tests server configuration with multiple workspace folders
func TestWorkspaceConfigManager_ServerConfigurationIntegration(t *testing.T) {
	testDir := t.TempDir()
	workspaceRoot := filepath.Join(testDir, "server-integration-test")
	
	// Create multi-language workspace structure
	workspaceStructure := map[string]string{
		"backend/go.mod": `module backend
go 1.21
`,
		"backend/main.go": "package main\n\nfunc main() {}\n",
		"frontend/package.json": `{
	"name": "frontend",
	"version": "1.0.0",
	"dependencies": {
		"typescript": "^4.0.0"
	}
}
`,
		"frontend/tsconfig.json": `{
	"compilerOptions": {
		"target": "ES2020"
	}
}
`,
		"scripts/setup.py": `from setuptools import setup
setup(name="scripts", version="1.0.0")
`,
	}
	
	if err := createTestWorkspaceStructure(workspaceRoot, workspaceStructure); err != nil {
		t.Fatalf("Failed to create test workspace: %v", err)
	}

	wcm := NewWorkspaceConfigManagerWithOptions(&WorkspaceConfigManagerOptions{
		BaseConfigDir: testDir,
	})

	// Generate workspace configuration
	detector := project.NewProjectDetector()
	projectContext, err := detector.DetectProject(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("Failed to detect project: %v", err)
	}

	if err := wcm.GenerateWorkspaceConfig(workspaceRoot, projectContext); err != nil {
		t.Fatalf("Failed to generate workspace config: %v", err)
	}

	config, err := wcm.LoadWorkspaceConfig(workspaceRoot)
	if err != nil {
		t.Fatalf("Failed to load workspace config: %v", err)
	}

	// Test server configuration integration
	t.Run("MultipleWorkspaceFolders", func(t *testing.T) {
		for serverName, serverConfig := range config.Servers {
			if len(serverConfig.WorkspaceRoots) == 0 {
				t.Errorf("Server '%s' should have workspace roots configured", serverName)
			}

			// Each server should have workspace roots for its supported languages
			for _, language := range serverConfig.Languages {
				if _, exists := serverConfig.WorkspaceRoots[language]; !exists {
					t.Errorf("Server '%s' should have workspace root for language '%s'", serverName, language)
				}
			}
		}
	})

	t.Run("LanguageToServerMapping", func(t *testing.T) {
		// Test that all workspace languages have corresponding servers
		workspaceLanguages := config.Workspace.Languages
		for _, language := range workspaceLanguages {
			found := false
			for _, serverConfig := range config.Servers {
				for _, serverLang := range serverConfig.Languages {
					if serverLang == language {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				t.Errorf("No server configuration found for language '%s'", language)
			}
		}
	})

	t.Run("ConsolidatedServerConfigs", func(t *testing.T) {
		// Test that servers with shared languages have consolidated configurations
		serverLanguageCount := make(map[string]map[string]int)
		for serverName, serverConfig := range config.Servers {
			serverLanguageCount[serverName] = make(map[string]int)
			for _, language := range serverConfig.Languages {
				serverLanguageCount[serverName][language]++
			}
		}

		// Verify no duplicate language mappings per server
		for serverName, languageCount := range serverLanguageCount {
			for language, count := range languageCount {
				if count > 1 {
					t.Errorf("Server '%s' has duplicate language mapping for '%s'", serverName, language)
				}
			}
		}
	})
}

// TestWorkspaceConfigManager_BackwardCompatibility tests backward compatibility with single-project workspaces
func TestWorkspaceConfigManager_BackwardCompatibility(t *testing.T) {
	testDir := t.TempDir()
	wcm := NewWorkspaceConfigManagerWithOptions(&WorkspaceConfigManagerOptions{
		BaseConfigDir: testDir,
	})

	t.Run("SingleProjectWorkspace", func(t *testing.T) {
		workspaceRoot := filepath.Join(testDir, "single-project")
		
		// Create simple Go project
		if err := os.MkdirAll(workspaceRoot, 0755); err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}

		goModContent := `module single-project
go 1.21
`
		if err := os.WriteFile(filepath.Join(workspaceRoot, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatalf("Failed to create go.mod: %v", err)
		}

		detector := project.NewProjectDetector()
		projectContext, err := detector.DetectProject(context.Background(), workspaceRoot)
		if err != nil {
			t.Fatalf("Failed to detect project: %v", err)
		}

		if err := wcm.GenerateWorkspaceConfig(workspaceRoot, projectContext); err != nil {
			t.Fatalf("Failed to generate workspace config: %v", err)
		}

		config, err := wcm.LoadWorkspaceConfig(workspaceRoot)
		if err != nil {
			t.Fatalf("Failed to load workspace config: %v", err)
		}

		// Test single-project behavior
		if config.Workspace.ProjectType != types.PROJECT_TYPE_GO {
			t.Errorf("Expected project type %s, got %s", types.PROJECT_TYPE_GO, config.Workspace.ProjectType)
		}

		// Should work without sub-projects or with minimal sub-projects
		if len(config.SubProjects) > 1 {
			t.Errorf("Single project workspace should have at most 1 sub-project, got %d", len(config.SubProjects))
		}

		// Should have server configurations
		if len(config.Servers) == 0 {
			t.Error("Single project workspace should have server configurations")
		}

		// Should validate successfully
		if err := wcm.ValidateWorkspaceConfig(config); err != nil {
			t.Errorf("Single project workspace validation failed: %v", err)
		}
	})

	t.Run("EmptySubProjectScenarios", func(t *testing.T) {
		workspaceRoot := filepath.Join(testDir, "empty-subprojects")
		if err := os.MkdirAll(workspaceRoot, 0755); err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}

		// Test GenerateSubProjectConfigs with empty sub-projects
		emptySubProjects := []*project.ProjectContext{}
		subProjectConfigs, err := wcm.GenerateSubProjectConfigs(workspaceRoot, emptySubProjects)
		if err != nil {
			t.Errorf("GenerateSubProjectConfigs with empty input should not fail: %v", err)
		}
		if len(subProjectConfigs) != 0 {
			t.Errorf("Expected empty sub-project configs, got %d", len(subProjectConfigs))
		}

		// Test GetSubProjectConfig with non-existent workspace
		_, err = wcm.GetSubProjectConfig(workspaceRoot, "any-id")
		if err == nil {
			t.Error("GetSubProjectConfig should fail for non-existent workspace")
		}
	})

	t.Run("ExistingFunctionalityIntact", func(t *testing.T) {
		workspaceRoot := filepath.Join(testDir, "functionality-test")
		if err := os.MkdirAll(workspaceRoot, 0755); err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}

		// Test existing methods work as before
		configPath := wcm.GetWorkspaceConfigPath(workspaceRoot)
		if configPath == "" {
			t.Error("GetWorkspaceConfigPath should return a valid path")
		}

		workspaceDir := wcm.GetWorkspaceDirectory(workspaceRoot)
		if workspaceDir == "" {
			t.Error("GetWorkspaceDirectory should return a valid directory")
		}

		if err := wcm.CreateWorkspaceDirectories(workspaceRoot); err != nil {
			t.Errorf("CreateWorkspaceDirectories should work: %v", err)
		}

		// Clean up
		if err := wcm.CleanupWorkspace(workspaceRoot); err != nil {
			t.Errorf("CleanupWorkspace should work: %v", err)
		}
	})
}

// TestWorkspaceConfigManager_ConfigurationRoundtrip tests the full configuration lifecycle
func TestWorkspaceConfigManager_ConfigurationRoundtrip(t *testing.T) {
	testDir := t.TempDir()
	workspaceRoot := filepath.Join(testDir, "roundtrip-test")
	
	// Create comprehensive workspace structure
	workspaceStructure := map[string]string{
		"services/auth/go.mod": `module auth
go 1.21
`,
		"services/auth/main.go": "package main\n\nfunc main() {}\n",
		"services/user/go.mod": `module user
go 1.21
`,
		"services/user/main.go": "package main\n\nfunc main() {}\n",
		"web/package.json": `{
	"name": "web",
	"version": "1.0.0"
}
`,
		"web/index.js": "console.log('web app');\n",
		"analytics/setup.py": `from setuptools import setup
setup(name="analytics", version="1.0.0")
`,
		"analytics/main.py": "print('analytics service')\n",
	}
	
	if err := createTestWorkspaceStructure(workspaceRoot, workspaceStructure); err != nil {
		t.Fatalf("Failed to create test workspace: %v", err)
	}

	wcm := NewWorkspaceConfigManagerWithOptions(&WorkspaceConfigManagerOptions{
		BaseConfigDir: testDir,
	})

	// Step 1: Generate configuration
	detector := project.NewProjectDetector()
	projectContext, err := detector.DetectProject(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("Failed to detect project: %v", err)
	}

	startTime := time.Now()
	if err := wcm.GenerateWorkspaceConfig(workspaceRoot, projectContext); err != nil {
		t.Fatalf("Failed to generate workspace config: %v", err)
	}
	generationTime := time.Since(startTime)

	// Step 2: Load configuration
	startTime = time.Now()
	config, err := wcm.LoadWorkspaceConfig(workspaceRoot)
	if err != nil {
		t.Fatalf("Failed to load workspace config: %v", err)
	}
	loadTime := time.Since(startTime)

	// Step 3: Validate configuration
	startTime = time.Now()
	if err := wcm.ValidateWorkspaceConfig(config); err != nil {
		t.Fatalf("Configuration validation failed: %v", err)
	}
	validationTime := time.Since(startTime)

	// Performance assertions for large multi-project workspaces
	maxGenerationTime := 5 * time.Second
	maxLoadTime := 1 * time.Second
	maxValidationTime := 1 * time.Second

	if generationTime > maxGenerationTime {
		t.Errorf("Configuration generation took too long: %v (max: %v)", generationTime, maxGenerationTime)
	}
	if loadTime > maxLoadTime {
		t.Errorf("Configuration loading took too long: %v (max: %v)", loadTime, maxLoadTime)
	}
	if validationTime > maxValidationTime {
		t.Errorf("Configuration validation took too long: %v (max: %v)", validationTime, maxValidationTime)
	}

	// Test comprehensive configuration properties
	t.Run("SubProjectCount", func(t *testing.T) {
		expectedMinProjects := 4 // auth, user, web, analytics
		if len(config.SubProjects) < expectedMinProjects {
			t.Errorf("Expected at least %d sub-projects, got %d", expectedMinProjects, len(config.SubProjects))
		}
	})

	t.Run("ServerConfigurations", func(t *testing.T) {
		expectedServers := []string{"gopls", "typescript-language-server", "pylsp"}
		for _, expectedServer := range expectedServers {
			if _, exists := config.Servers[expectedServer]; !exists {
				t.Errorf("Expected server '%s' not found", expectedServer)
			}
		}
	})

	t.Run("WorkspaceRootMapping", func(t *testing.T) {
		for _, subProject := range config.SubProjects {
			found := false
			for _, serverConfig := range config.Servers {
				for _, workspaceRoot := range serverConfig.WorkspaceRoots {
					if strings.HasPrefix(subProject.AbsolutePath, workspaceRoot) {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				t.Errorf("Sub-project '%s' not properly mapped to any server workspace root", subProject.ID)
			}
		}
	})
}

// TestWorkspaceConfigManager_ErrorHandling tests comprehensive error scenarios
func TestWorkspaceConfigManager_ErrorHandling(t *testing.T) {
	testDir := t.TempDir()
	wcm := NewWorkspaceConfigManagerWithOptions(&WorkspaceConfigManagerOptions{
		BaseConfigDir: testDir,
	})

	t.Run("InvalidSubProjectValidation", func(t *testing.T) {
		invalidSubProjects := []*SubProjectInfo{
			nil, // Nil sub-project
			{
				ID: "", // Empty ID
			},
			{
				ID:           "valid-id",
				AbsolutePath: "", // Empty path
			},
			{
				ID:           "another-valid-id",
				AbsolutePath: "/nonexistent/path", // Non-existent path
				ProjectType:  "",                  // Empty project type
			},
			{
				ID:           "quota-test",
				AbsolutePath: testDir,
				ProjectType:  types.PROJECT_TYPE_GO,
				Languages:    []string{types.PROJECT_TYPE_GO},
				ResourceQuota: &ResourceQuota{
					MaxMemoryMB:    -100, // Negative quota
					MaxConcurrency: -5,   // Negative concurrency
					MaxCacheSize:   -50,  // Negative cache size
				},
			},
		}

		for i, subProject := range invalidSubProjects {
			err := wcm.ValidateSubProjectConfig(subProject)
			if err == nil {
				t.Errorf("Expected validation error for invalid sub-project %d", i)
			}
		}
	})

	t.Run("LoadingNonExistentConfig", func(t *testing.T) {
		nonExistentPath := filepath.Join(testDir, "nonexistent-workspace")
		_, err := wcm.LoadWorkspaceConfig(nonExistentPath)
		if err == nil {
			t.Error("Expected error when loading non-existent workspace config")
		}
		expectedError := "workspace configuration not found"
		if !strings.Contains(err.Error(), expectedError) {
			t.Errorf("Expected error message to contain '%s', got: %s", expectedError, err.Error())
		}
	})

	t.Run("GenerateConfigWithNilContext", func(t *testing.T) {
		workspaceRoot := filepath.Join(testDir, "nil-context-test")
		err := wcm.GenerateWorkspaceConfig(workspaceRoot, nil)
		if err == nil {
			t.Error("Expected error when generating config with nil project context")
		}
		expectedError := "project context cannot be nil"
		if !strings.Contains(err.Error(), expectedError) {
			t.Errorf("Expected error message to contain '%s', got: %s", expectedError, err.Error())
		}
	})
}

// Helper function to create test workspace structure
func createTestWorkspaceStructure(workspaceRoot string, structure map[string]string) error {
	for relativePath, content := range structure {
		fullPath := filepath.Join(workspaceRoot, relativePath)
		dir := filepath.Dir(fullPath)
		
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", fullPath, err)
		}
	}
	return nil
}

// Benchmark tests for performance validation
func BenchmarkWorkspaceConfigManager_MultiProjectGeneration(b *testing.B) {
	testDir := b.TempDir()
	
	// Create large workspace structure for performance testing
	largeWorkspaceStructure := make(map[string]string)
	for i := 0; i < 20; i++ {
		serviceDir := fmt.Sprintf("service-%d", i)
		largeWorkspaceStructure[fmt.Sprintf("%s/go.mod", serviceDir)] = fmt.Sprintf("module %s\ngo 1.21\n", serviceDir)
		largeWorkspaceStructure[fmt.Sprintf("%s/main.go", serviceDir)] = "package main\n\nfunc main() {}\n"
	}
	
	workspaceRoot := filepath.Join(testDir, "benchmark-workspace")
	if err := createTestWorkspaceStructure(workspaceRoot, largeWorkspaceStructure); err != nil {
		b.Fatalf("Failed to create benchmark workspace: %v", err)
	}

	wcm := NewWorkspaceConfigManagerWithOptions(&WorkspaceConfigManagerOptions{
		BaseConfigDir: testDir,
	})

	detector := project.NewProjectDetector()
	projectContext, err := detector.DetectProject(context.Background(), workspaceRoot)
	if err != nil {
		b.Fatalf("Failed to detect project: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := wcm.GenerateWorkspaceConfig(workspaceRoot, projectContext); err != nil {
			b.Fatalf("Failed to generate workspace config: %v", err)
		}
	}
}