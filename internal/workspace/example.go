package workspace

import (
	"fmt"
	"log"
)

// ExampleUsage demonstrates how to use the simplified workspace detector
func ExampleUsage() {
	// Create a new workspace detector
	detector := NewWorkspaceDetector()

	// Detect workspace at current working directory
	workspace, err := detector.DetectWorkspace()
	if err != nil {
		log.Fatalf("Failed to detect workspace: %v", err)
	}

	// Display workspace information
	fmt.Printf("Workspace Information:\n")
	fmt.Printf("  ID: %s\n", workspace.ID)
	fmt.Printf("  Root: %s\n", workspace.Root)
	fmt.Printf("  Project Type: %s\n", workspace.ProjectType)
	fmt.Printf("  Languages: %v\n", workspace.Languages)
	fmt.Printf("  Hash: %s\n", workspace.Hash)
	fmt.Printf("  Created At: %s\n", workspace.CreatedAt.Format("2006-01-02 15:04:05"))

	// Validate the workspace
	if err := detector.ValidateWorkspace(workspace.Root); err != nil {
		log.Printf("Workspace validation failed: %v", err)
	} else {
		fmt.Printf("  Validation: ✓ Passed\n")
	}

	// Generate workspace ID for a specific path
	customID := detector.GenerateWorkspaceID("/path/to/custom/workspace")
	fmt.Printf("  Custom Workspace ID for '/path/to/custom/workspace': %s\n", customID)

	// Detect workspace at a specific path
	if customWorkspace, err := detector.DetectWorkspaceAt("/tmp"); err == nil {
		fmt.Printf("\nCustom Workspace at /tmp:\n")
		fmt.Printf("  ID: %s\n", customWorkspace.ID)
		fmt.Printf("  Project Type: %s\n", customWorkspace.ProjectType)
		fmt.Printf("  Languages: %v\n", customWorkspace.Languages)
	}
}

// Example with different workspace types
func ExampleWorkspaceTypes() {
	// detector := NewWorkspaceDetector() // Not used in this example

	examples := []struct {
		name         string
		files        []string
		expectedType string
	}{
		{
			name:         "Go Project",
			files:        []string{"go.mod", "main.go"},
			expectedType: "go",
		},
		{
			name:         "Python Project", 
			files:        []string{"setup.py", "main.py"},
			expectedType: "python",
		},
		{
			name:         "TypeScript Project",
			files:        []string{"tsconfig.json", "index.ts"},
			expectedType: "typescript",
		},
		{
			name:         "Node.js Project",
			files:        []string{"package.json", "index.js"},
			expectedType: "nodejs",
		},
		{
			name:         "Java Project",
			files:        []string{"pom.xml", "Main.java"},
			expectedType: "java",
		},
		{
			name:         "Mixed Project",
			files:        []string{"go.mod", "package.json", "main.go", "index.js"},
			expectedType: "mixed",
		},
		{
			name:         "Unknown Project",
			files:        []string{"README.md", "data.txt"},
			expectedType: "unknown",
		},
	}

	fmt.Printf("Workspace Detection Examples:\n")
	fmt.Printf("=============================\n\n")

	for _, example := range examples {
		fmt.Printf("%s:\n", example.name)
		fmt.Printf("  Files: %v\n", example.files)
		fmt.Printf("  Expected Type: %s\n", example.expectedType)
		fmt.Printf("  Detection Logic: Based on marker files and extensions at root level\n\n")
	}
}

// Key differences from complex project detector
func ExampleSimplifiedDifferences() {
	fmt.Printf("Simplified Workspace Detector vs Complex Project Detector:\n")
	fmt.Printf("=========================================================\n\n")

	fmt.Printf("Simplified Workspace Detector:\n")
	fmt.Printf("  ✓ Uses os.Getwd() as workspace root (no recursive searching)\n")
	fmt.Printf("  ✓ Detects languages at workspace root level only\n")
	fmt.Printf("  ✓ No VCS detection or monorepo handling\n")
	fmt.Printf("  ✓ No caching or complex configuration\n")
	fmt.Printf("  ✓ Simple workspace hash for identification\n")
	fmt.Printf("  ✓ Focused on single workspace boundary\n")
	fmt.Printf("  ✓ MAX_DEPTH: 1 (root level only)\n\n")

	fmt.Printf("Complex Project Detector:\n")
	fmt.Printf("  • Recursive directory scanning (MAX_DEPTH: 10)\n")
	fmt.Printf("  • VCS detection (.git, .svn, etc.)\n")
	fmt.Printf("  • Monorepo detection and sub-project aggregation\n")
	fmt.Printf("  • Confidence scoring and language prioritization\n")
	fmt.Printf("  • Complex project aggregation and mixed project types\n")
	fmt.Printf("  • Caching and performance optimizations\n")
	fmt.Printf("  • Multiple workspace boundaries\n\n")

	fmt.Printf("Use Cases for Simplified Detector:\n")
	fmt.Printf("  • Quick workspace identification\n")
	fmt.Printf("  • Single-directory projects\n")
	fmt.Printf("  • Fast startup and minimal overhead\n")
	fmt.Printf("  • Simple project context creation\n")
}