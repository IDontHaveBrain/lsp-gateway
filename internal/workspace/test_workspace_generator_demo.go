// +build ignore

package main

import (
	"fmt"
	"log"

	workspace "lsp-gateway/internal/workspace"
	"lsp-gateway/internal/project/types"
)

// Demonstration of the comprehensive test workspace generator
func main() {
	fmt.Println("=== Test Workspace Generator Demo ===")

	// Example 1: Simple multi-project workspace
	fmt.Println("\n1. Creating simple multi-project workspace...")
	workspace1, err := workspace.CreateSimpleMultiProjectWorkspace(
		"demo-workspace", 
		[]string{types.PROJECT_TYPE_GO, types.PROJECT_TYPE_PYTHON, types.PROJECT_TYPE_TYPESCRIPT},
	)
	if err != nil {
		log.Fatalf("Failed to create workspace: %v", err)
	}

	fmt.Printf("   Created workspace: %s\n", workspace1.Name)
	fmt.Printf("   Root path: %s\n", workspace1.RootPath)
	fmt.Printf("   Projects: %d\n", len(workspace1.Projects))
	fmt.Printf("   Languages: %v\n", workspace1.Languages)

	// Example 2: Complex nested workspace
	fmt.Println("\n2. Creating complex nested workspace...")
	workspace2, err := workspace.CreateComplexNestedWorkspace("complex-demo")
	if err != nil {
		log.Fatalf("Failed to create nested workspace: %v", err)
	}

	fmt.Printf("   Created nested workspace: %s\n", workspace2.Name)
	fmt.Printf("   Root path: %s\n", workspace2.RootPath)
	fmt.Printf("   Projects: %d\n", len(workspace2.Projects))
	fmt.Printf("   Languages: %v\n", workspace2.Languages)

	// Example 3: Workspace with cross-references
	fmt.Println("\n3. Creating workspace with cross-references...")
	workspace3, err := workspace.CreateTestWorkspaceWithCrossReferences("cross-ref-demo")
	if err != nil {
		log.Fatalf("Failed to create cross-ref workspace: %v", err)
	}

	fmt.Printf("   Created cross-ref workspace: %s\n", workspace3.Name)
	fmt.Printf("   Root path: %s\n", workspace3.RootPath)
	fmt.Printf("   Projects: %d\n", len(workspace3.Projects))
	fmt.Printf("   Languages: %v\n", workspace3.Languages)

	// Example 4: Custom workspace generation
	fmt.Println("\n4. Creating custom workspace...")
	generator := workspace.NewTestWorkspaceGenerator()

	config := &workspace.MultiProjectWorkspaceConfig{
		Name: "custom-demo",
		Projects: []*workspace.ProjectConfig{
			{
				Name:         "api-gateway",
				RelativePath: "gateway",
				ProjectType:  types.PROJECT_TYPE_GO,
				Languages:    []string{types.PROJECT_TYPE_GO},
				EnableBuild:  true,
				EnableTests:  true,
			},
			{
				Name:         "data-processor",
				RelativePath: "processor",
				ProjectType:  types.PROJECT_TYPE_PYTHON,
				Languages:    []string{types.PROJECT_TYPE_PYTHON},
				EnableBuild:  true,
				EnableTests:  true,
			},
		},
		SharedDirectories: []string{"shared", "config", "docs"},
		WorkspaceFiles: map[string]string{
			"README.md": "# Custom Demo Workspace\n\nA custom multi-project workspace for demonstration.\n",
			"Makefile":  "# Custom Makefile\n\nbuild:\n\t@echo \"Building all projects\"\n",
			"docker-compose.yml": "version: '3.8'\nservices:\n  gateway:\n    build: ./gateway\n  processor:\n    build: ./processor\n",
		},
		GenerateMetadata: true,
		ValidationLevel:  workspace.ValidationComplete,
	}

	workspace4, err := generator.GenerateMultiProjectWorkspace(config)
	if err != nil {
		log.Fatalf("Failed to create custom workspace: %v", err)
	}

	fmt.Printf("   Created custom workspace: %s\n", workspace4.Name)
	fmt.Printf("   Root path: %s\n", workspace4.RootPath)
	fmt.Printf("   Projects: %d\n", len(workspace4.Projects))
	fmt.Printf("   Generated files: %d\n", len(workspace4.GeneratedFiles))

	// Example 5: Validate workspace structures
	fmt.Println("\n5. Validating workspace structures...")
	
	workspaces := []*workspace.TestWorkspace{workspace1, workspace2, workspace3, workspace4}
	for i, ws := range workspaces {
		if err := generator.ValidateWorkspaceStructure(ws); err != nil {
			fmt.Printf("   Workspace %d validation failed: %v\n", i+1, err)
		} else {
			fmt.Printf("   Workspace %d validation passed ✓\n", i+1)
		}
	}

	// Example 6: Show project details for the first workspace
	fmt.Println("\n6. Project details for first workspace:")
	for _, project := range workspace1.Projects {
		fmt.Printf("   - %s (%s) at %s\n", project.Name, project.ProjectType, project.RelativePath)
		fmt.Printf("     Languages: %v\n", project.Languages)
		fmt.Printf("     Marker files: %v\n", project.MarkerFiles)
	}

	// Cleanup all workspaces
	fmt.Println("\n7. Cleaning up workspaces...")
	cleanupWorkspaces := []*workspace.TestWorkspace{workspace1, workspace2, workspace3, workspace4}
	for i, ws := range cleanupWorkspaces {
		if err := generator.CleanupWorkspace(ws); err != nil {
			fmt.Printf("   Failed to clean workspace %d: %v\n", i+1, err)
		} else {
			fmt.Printf("   Cleaned workspace %d ✓\n", i+1)
		}
	}

	fmt.Println("\n=== Demo Complete ===")
}