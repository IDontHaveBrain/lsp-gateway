package helpers

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// WorkspaceTestHelper provides utilities for creating test workspaces
type WorkspaceTestHelper struct {
	tempDir string
}

// NewWorkspaceTestHelper creates a new WorkspaceTestHelper instance
func NewWorkspaceTestHelper(tempDir string) *WorkspaceTestHelper {
	return &WorkspaceTestHelper{
		tempDir: tempDir,
	}
}

// CreateGoWorkspace creates a Go workspace with go.work file and modules
func (h *WorkspaceTestHelper) CreateGoWorkspace(name string, structure map[string]string) string {
	return h.createProjectStructure(name, structure)
}

// CreateMonorepo creates a monorepo structure with multiple languages
func (h *WorkspaceTestHelper) CreateMonorepo(name string, structure map[string]string) string {
	return h.createProjectStructure(name, structure)
}

// CreateMicroservicesProject creates a microservices project structure
func (h *WorkspaceTestHelper) CreateMicroservicesProject(name string, structure map[string]string) string {
	return h.createProjectStructure(name, structure)
}

// CreatePythonMLProject creates a Python ML project structure
func (h *WorkspaceTestHelper) CreatePythonMLProject(name string, structure map[string]string) string {
	return h.createProjectStructure(name, structure)
}

// CreateNestedProject creates a nested project structure for depth testing
func (h *WorkspaceTestHelper) CreateNestedProject(name string, structure map[string]string) string {
	return h.createProjectStructure(name, structure)
}

// CreateSimpleGoProject creates a simple Go project
func (h *WorkspaceTestHelper) CreateSimpleGoProject(name string, structure map[string]string) string {
	return h.createProjectStructure(name, structure)
}

// CreateEmptyWorkspace creates an empty workspace directory
func (h *WorkspaceTestHelper) CreateEmptyWorkspace(name string) string {
	workspacePath := filepath.Join(h.tempDir, name)
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		fmt.Printf("Warning: failed to create empty workspace %s: %v\n", workspacePath, err)
	}
	return workspacePath
}

// CreateCorruptedWorkspace creates a workspace with corrupted marker files
func (h *WorkspaceTestHelper) CreateCorruptedWorkspace(name string, structure map[string]string) string {
	return h.createProjectStructure(name, structure)
}

// CreatePartiallyCorruptedWorkspace creates a workspace with some corrupted subdirectories
func (h *WorkspaceTestHelper) CreatePartiallyCorruptedWorkspace(name string, structure map[string]string) string {
	return h.createProjectStructure(name, structure)
}

// CreateWorkspaceWithIgnoredDirs creates a workspace with directories that should be ignored
func (h *WorkspaceTestHelper) CreateWorkspaceWithIgnoredDirs(name string, structure map[string]string) string {
	return h.createProjectStructure(name, structure)
}

// CreateWorkspaceWithSpecialNames creates a workspace with special characters in directory names
func (h *WorkspaceTestHelper) CreateWorkspaceWithSpecialNames(name string, structure map[string]string) string {
	return h.createProjectStructure(name, structure)
}

// CreateMultiLanguageWorkspace creates a workspace with multiple programming languages
func (h *WorkspaceTestHelper) CreateMultiLanguageWorkspace(name string, structure map[string]string) string {
	return h.createProjectStructure(name, structure)
}

// CreateLargeWorkspace creates a workspace with many sub-projects for performance testing
func (h *WorkspaceTestHelper) CreateLargeWorkspace(name string, count int) string {
	structure := make(map[string]string)
	
	// Create root project
	structure["go.mod"] = fmt.Sprintf(`module %s

go 1.24
`, name)
	structure["main.go"] = `package main

func main() {}
`

	// Create multiple sub-projects
	for i := 0; i < count; i++ {
		projectName := fmt.Sprintf("project-%03d", i)
		
		// Alternate between different project types for variety
		switch i % 4 {
		case 0: // Go project
			structure[fmt.Sprintf("services/%s/go.mod", projectName)] = fmt.Sprintf(`module %s/%s

go 1.24
`, name, projectName)
			structure[fmt.Sprintf("services/%s/main.go", projectName)] = `package main

func main() {}
`
		case 1: // Node.js project
			structure[fmt.Sprintf("apps/%s/package.json", projectName)] = fmt.Sprintf(`{
  "name": "%s",
  "version": "1.0.0",
  "dependencies": {
    "express": "^4.18.0"
  }
}`, projectName)
			structure[fmt.Sprintf("apps/%s/index.js", projectName)] = `const express = require('express');
const app = express();
app.listen(3000);
`
		case 2: // Python project
			structure[fmt.Sprintf("libs/%s/pyproject.toml", projectName)] = fmt.Sprintf(`[project]
name = "%s"
version = "0.1.0"
`, projectName)
			structure[fmt.Sprintf("libs/%s/main.py", projectName)] = `def main():
    pass

if __name__ == "__main__":
    main()
`
		case 3: // TypeScript project
			structure[fmt.Sprintf("packages/%s/package.json", projectName)] = fmt.Sprintf(`{
  "name": "%s",
  "version": "1.0.0",
  "dependencies": {
    "typescript": "^4.0.0"
  }
}`, projectName)
			structure[fmt.Sprintf("packages/%s/tsconfig.json", projectName)] = `{
  "compilerOptions": {
    "target": "es2017"
  }
}`
			structure[fmt.Sprintf("packages/%s/index.ts", projectName)] = `export function hello(): string {
  return "Hello, World!";
}
`
		}
	}

	return h.createProjectStructure(name, structure)
}

// createProjectStructure is the common implementation for creating project structures
func (h *WorkspaceTestHelper) createProjectStructure(name string, structure map[string]string) string {
	projectPath := filepath.Join(h.tempDir, name)
	
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		fmt.Printf("Warning: failed to create project directory %s: %v\n", projectPath, err)
		return projectPath
	}

	for filePath, content := range structure {
		fullPath := filepath.Join(projectPath, filePath)
		
		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			fmt.Printf("Warning: failed to create directory %s: %v\n", filepath.Dir(fullPath), err)
			continue
		}

		// Write file content
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			fmt.Printf("Warning: failed to write file %s: %v\n", fullPath, err)
		}
	}

	return projectPath
}

// ValidateWorkspaceStructure validates that a workspace has the expected structure
func (h *WorkspaceTestHelper) ValidateWorkspaceStructure(workspacePath string, expectedFiles []string) bool {
	for _, expectedFile := range expectedFiles {
		fullPath := filepath.Join(workspacePath, expectedFile)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			fmt.Printf("Expected file not found: %s\n", fullPath)
			return false
		}
	}
	return true
}

// CleanupWorkspace removes a workspace directory and all its contents
func (h *WorkspaceTestHelper) CleanupWorkspace(workspacePath string) error {
	return os.RemoveAll(workspacePath)
}

// GetWorkspacePath returns the full path for a workspace by name
func (h *WorkspaceTestHelper) GetWorkspacePath(name string) string {
	return filepath.Join(h.tempDir, name)
}

// CreateFileInWorkspace creates a single file in an existing workspace
func (h *WorkspaceTestHelper) CreateFileInWorkspace(workspacePath, relativePath, content string) error {
	fullPath := filepath.Join(workspacePath, relativePath)
	
	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	return os.WriteFile(fullPath, []byte(content), 0644)
}

// ListWorkspaceFiles returns a list of all files in a workspace (for debugging)
func (h *WorkspaceTestHelper) ListWorkspaceFiles(workspacePath string) ([]string, error) {
	var files []string
	
	err := filepath.Walk(workspacePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if !info.IsDir() {
			relativePath, err := filepath.Rel(workspacePath, path)
			if err != nil {
				return err
			}
			files = append(files, relativePath)
		}
		
		return nil
	})

	return files, err
}

// CreateWorkspaceTemplate creates a workspace from a predefined template
func (h *WorkspaceTestHelper) CreateWorkspaceTemplate(name, templateType string) string {
	var structure map[string]string

	switch templateType {
	case "go-monorepo":
		structure = map[string]string{
			"go.work": `go 1.24

use (
	./service-a
	./service-b
	./libs/shared
)
`,
			"service-a/go.mod": `module example.com/service-a

go 1.24
`,
			"service-a/main.go": `package main

import "fmt"

func main() {
	fmt.Println("Service A")
}
`,
			"service-b/go.mod": `module example.com/service-b

go 1.24
`,
			"service-b/main.go": `package main

import "fmt"

func main() {
	fmt.Println("Service B")
}
`,
			"libs/shared/go.mod": `module example.com/shared

go 1.24
`,
			"libs/shared/utils.go": `package shared

func CommonFunc() string {
	return "shared utility"
}
`,
		}

	case "fullstack-monorepo":
		structure = map[string]string{
			"package.json": `{
  "name": "fullstack-monorepo",
  "workspaces": ["apps/*", "packages/*"],
  "scripts": {
    "build": "turbo run build"
  }
}`,
			"turbo.json": `{
  "pipeline": {
    "build": {}
  }
}`,
			"apps/web/package.json": `{
  "name": "@company/web",
  "dependencies": {
    "react": "^18.0.0",
    "next": "^13.0.0"
  }
}`,
			"apps/web/tsconfig.json": `{
  "compilerOptions": {
    "target": "es2017"
  }
}`,
			"apps/api/package.json": `{
  "name": "@company/api",
  "dependencies": {
    "express": "^4.18.0"
  }
}`,
			"packages/ui/package.json": `{
  "name": "@company/ui",
  "dependencies": {
    "react": "^18.0.0"
  }
}`,
		}

	case "microservices":
		structure = map[string]string{
			"docker-compose.yml": `version: '3.8'
services:
  user-service:
    build: ./services/user-service
  order-service:
    build: ./services/order-service
  api-gateway:
    build: ./gateway
`,
			"services/user-service/go.mod": `module microservices/user-service

go 1.24
`,
			"services/user-service/main.go": `package main

func main() {}
`,
			"services/order-service/package.json": `{
  "name": "order-service",
  "dependencies": {
    "express": "^4.18.0"
  }
}`,
			"gateway/go.mod": `module microservices/gateway

go 1.24
`,
		}

	case "python-ml":
		structure = map[string]string{
			"pyproject.toml": `[project]
name = "ml-project"
version = "0.1.0"
dependencies = [
    "scikit-learn>=1.0.0",
    "tensorflow>=2.10.0"
]
`,
			"src/models/classifier.py": `import sklearn
from sklearn.ensemble import RandomForestClassifier

class Classifier:
    def __init__(self):
        self.model = RandomForestClassifier()
`,
			"src/training/train.py": `def train_model():
    pass
`,
		}

	default:
		// Simple Go project template
		structure = map[string]string{
			"go.mod": fmt.Sprintf(`module %s

go 1.24
`, name),
			"main.go": `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`,
		}
	}

	return h.createProjectStructure(name, structure)
}

// CountSubDirectories counts the number of subdirectories in a workspace
func (h *WorkspaceTestHelper) CountSubDirectories(workspacePath string) int {
	count := 0
	
	filepath.Walk(workspacePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue walking even if there are errors
		}
		
		if info.IsDir() && path != workspacePath {
			count++
		}
		
		return nil
	})

	return count
}

// GetTempDir returns the temporary directory used by this helper
func (h *WorkspaceTestHelper) GetTempDir() string {
	return h.tempDir
}

// CreateBenchmarkWorkspace creates a workspace specifically for benchmarking
func (h *WorkspaceTestHelper) CreateBenchmarkWorkspace(name string, projectCount, filesPerProject int) string {
	structure := make(map[string]string)
	
	for i := 0; i < projectCount; i++ {
		projectName := fmt.Sprintf("project-%03d", i)
		
		// Create Go module
		structure[fmt.Sprintf("%s/go.mod", projectName)] = fmt.Sprintf(`module benchmark/%s

go 1.24
`, projectName)
		
		// Create multiple Go files
		for j := 0; j < filesPerProject; j++ {
			fileName := fmt.Sprintf("%s/file_%03d.go", projectName, j)
			structure[fileName] = fmt.Sprintf(`package main

import "fmt"

func Function%d() {
	fmt.Println("Function %d in project %s")
}
`, j, j, projectName)
		}
	}

	return h.createProjectStructure(name, structure)
}

// ValidateProjectCount validates that a workspace has the expected number of projects
func (h *WorkspaceTestHelper) ValidateProjectCount(workspacePath string, expectedCount int) bool {
	count := 0
	
	// Count directories with marker files (go.mod, package.json, etc.)
	filepath.Walk(workspacePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		
		if !info.IsDir() {
			return nil
		}
		
		// Check for project marker files
		markerFiles := []string{"go.mod", "package.json", "pyproject.toml", "pom.xml", "tsconfig.json"}
		for _, marker := range markerFiles {
			markerPath := filepath.Join(path, marker)
			if _, err := os.Stat(markerPath); err == nil {
				count++
				break // Found one marker, don't count multiple markers in same directory
			}
		}
		
		return nil
	})

	return count == expectedCount
}

// CreateSymlinkWorkspace creates a workspace with symbolic links (Unix only)
func (h *WorkspaceTestHelper) CreateSymlinkWorkspace(name string) string {
	if os.Getenv("GOOS") == "windows" {
		// On Windows, create regular directories instead
		return h.CreateWorkspaceTemplate(name, "default")
	}

	// Create actual directories
	actualPath := filepath.Join(h.tempDir, name+"-actual")
	structure := map[string]string{
		"go.mod": fmt.Sprintf(`module %s

go 1.24
`, name),
		"main.go": `package main

func main() {}
`,
		"service/go.mod": fmt.Sprintf(`module %s/service

go 1.24
`, name),
		"service/main.go": `package main

func main() {}
`,
	}
	
	actualWorkspace := h.createProjectStructure(name+"-actual", structure)
	
	// Create workspace with symlinks
	workspacePath := filepath.Join(h.tempDir, name)
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		fmt.Printf("Warning: failed to create symlink workspace %s: %v\n", workspacePath, err)
		return workspacePath
	}

	// Create symlinks to actual files/directories
	os.Symlink(filepath.Join(actualWorkspace, "go.mod"), filepath.Join(workspacePath, "go.mod"))
	os.Symlink(filepath.Join(actualWorkspace, "main.go"), filepath.Join(workspacePath, "main.go"))
	os.Symlink(filepath.Join(actualWorkspace, "service"), filepath.Join(workspacePath, "service"))

	return workspacePath
}