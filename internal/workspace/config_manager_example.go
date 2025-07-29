package workspace

import (
	"context"
	"fmt"
	"lsp-gateway/internal/project"
	"lsp-gateway/internal/setup"
	"os"
	"path/filepath"
)

func ExampleWorkspaceConfigManager() error {
	workspaceManager := NewWorkspaceConfigManager()
	
	workspaceRoot := "/tmp/example-workspace"
	if err := os.MkdirAll(workspaceRoot, 0755); err != nil {
		return fmt.Errorf("failed to create example workspace: %w", err)
	}
	defer os.RemoveAll(workspaceRoot)

	goModContent := `module example-workspace
go 1.21
`
	if err := os.WriteFile(filepath.Join(workspaceRoot, "go.mod"), []byte(goModContent), 0644); err != nil {
		return fmt.Errorf("failed to create go.mod: %w", err)
	}

	detector := project.NewProjectDetector()
	detector.SetLogger(setup.NewSetupLogger(&setup.SetupLoggerConfig{
		Component: "example",
		Level:     setup.LogLevelInfo,
	}))

	projectContext, err := detector.DetectProject(context.Background(), workspaceRoot)
	if err != nil {
		return fmt.Errorf("failed to detect project: %w", err)
	}

	fmt.Printf("Detected project: %s (languages: %v)\n", 
		projectContext.ProjectType, projectContext.Languages)

	if err := workspaceManager.GenerateWorkspaceConfig(workspaceRoot, projectContext); err != nil {
		return fmt.Errorf("failed to generate workspace config: %w", err)
	}

	workspaceConfig, err := workspaceManager.LoadWorkspaceConfig(workspaceRoot)
	if err != nil {
		return fmt.Errorf("failed to load workspace config: %w", err)
	}

	if err := workspaceManager.ValidateWorkspaceConfig(workspaceConfig); err != nil {
		return fmt.Errorf("workspace config validation failed: %w", err)
	}

	fmt.Printf("Workspace Configuration:\n")
	fmt.Printf("  ID: %s\n", workspaceConfig.Workspace.WorkspaceID)
	fmt.Printf("  Root: %s\n", workspaceConfig.Workspace.RootPath)
	fmt.Printf("  Type: %s\n", workspaceConfig.Workspace.ProjectType)
	fmt.Printf("  Languages: %v\n", workspaceConfig.Workspace.Languages)
	fmt.Printf("  Servers: %d configured\n", len(workspaceConfig.Servers))
	
	for serverName := range workspaceConfig.Servers {
		fmt.Printf("    - %s\n", serverName)
	}

	configPath := workspaceManager.GetWorkspaceConfigPath(workspaceRoot)
	fmt.Printf("  Config path: %s\n", configPath)

	workspaceDir := workspaceManager.GetWorkspaceDirectory(workspaceRoot)
	fmt.Printf("  Workspace directory: %s\n", workspaceDir)

	if err := workspaceManager.CleanupWorkspace(workspaceRoot); err != nil {
		fmt.Printf("Warning: failed to cleanup workspace: %v\n", err)
	}

	fmt.Println("Example completed successfully!")
	return nil
}

func ExampleHierarchicalConfig() error {
	workspaceManager := NewWorkspaceConfigManager()
	
	workspaceRoot := "/tmp/example-hierarchy-workspace"
	if err := os.MkdirAll(workspaceRoot, 0755); err != nil {
		return fmt.Errorf("failed to create example workspace: %w", err)
	}
	defer os.RemoveAll(workspaceRoot)

	packageJsonContent := `{
  "name": "example-typescript-project",
  "version": "1.0.0",
  "dependencies": {
    "typescript": "^4.0.0"
  }
}
`
	if err := os.WriteFile(filepath.Join(workspaceRoot, "package.json"), []byte(packageJsonContent), 0644); err != nil {
		return fmt.Errorf("failed to create package.json: %w", err)
	}

	tsconfigContent := `{
  "compilerOptions": {
    "target": "es2020",
    "module": "commonjs",
    "strict": true
  }
}
`
	if err := os.WriteFile(filepath.Join(workspaceRoot, "tsconfig.json"), []byte(tsconfigContent), 0644); err != nil {
		return fmt.Errorf("failed to create tsconfig.json: %w", err)
	}

	detector := project.NewProjectDetector()
	projectContext, err := detector.DetectProject(context.Background(), workspaceRoot)
	if err != nil {
		return fmt.Errorf("failed to detect project: %w", err)
	}

	if err := workspaceManager.GenerateWorkspaceConfig(workspaceRoot, projectContext); err != nil {
		return fmt.Errorf("failed to generate workspace config: %w", err)
	}

	globalConfigPath := ""
	hierarchicalConfig, err := workspaceManager.LoadWithHierarchy(workspaceRoot, globalConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load hierarchical config: %w", err)
	}

	fmt.Printf("Hierarchical Configuration:\n")
	fmt.Printf("  Project: %s (%s)\n", 
		hierarchicalConfig.Workspace.Name, hierarchicalConfig.Workspace.ProjectType)
	fmt.Printf("  Performance enabled: %v\n", hierarchicalConfig.Performance.Enabled)
	fmt.Printf("  SCIP enabled: %v\n", hierarchicalConfig.Performance.SCIP.Enabled)
	fmt.Printf("  Cache enabled: %v\n", hierarchicalConfig.Cache.Enabled)

	if err := workspaceManager.CleanupWorkspace(workspaceRoot); err != nil {
		fmt.Printf("Warning: failed to cleanup workspace: %v\n", err)
	}

	fmt.Println("Hierarchical config example completed successfully!")
	return nil
}