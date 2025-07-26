package project_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"lsp-gateway/internal/project"
)

func TestNewProjectDetector(t *testing.T) {
	detector := project.NewProjectDetector()
	if detector == nil {
		t.Fatal("NewProjectDetector should return non-nil detector")
	}
}

func TestDetectGoProject(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-go-project-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create go.mod file
	goModPath := filepath.Join(tempDir, "go.mod")
	err = os.WriteFile(goModPath, []byte("module test\n\ngo 1.21\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	detector := project.NewProjectDetector()
	ctx := context.Background()
	result, err := detector.DetectProject(ctx, tempDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Languages) == 0 {
		t.Error("Expected Go language to be detected")
	}

	found := false
	for _, lang := range result.Languages {
		if lang == "go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected Go language to be detected in results")
	}
}

func TestDetectPythonProject(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-python-project-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create requirements.txt file
	reqPath := filepath.Join(tempDir, "requirements.txt")
	err = os.WriteFile(reqPath, []byte("django==4.2.0\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	detector := project.NewProjectDetector()
	ctx := context.Background()
	result, err := detector.DetectProject(ctx, tempDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Languages) == 0 {
		t.Error("Expected Python language to be detected")
	}

	found := false
	for _, lang := range result.Languages {
		if lang == "python" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected Python language to be detected in results")
	}
}

func TestDetectTypescriptProject(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-typescript-project-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create package.json with TypeScript dependency
	packagePath := filepath.Join(tempDir, "package.json")
	packageContent := `{
		"name": "test-project",
		"dependencies": {
			"typescript": "^5.0.0"
		}
	}`
	err = os.WriteFile(packagePath, []byte(packageContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	detector := project.NewProjectDetector()
	ctx := context.Background()
	result, err := detector.DetectProject(ctx, tempDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Languages) == 0 {
		t.Error("Expected TypeScript language to be detected")
	}

	found := false
	for _, lang := range result.Languages {
		if lang == "typescript" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected TypeScript language to be detected in results")
	}
}

func TestDetectEmptyDirectory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-empty-project-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	detector := project.NewProjectDetector()
	ctx := context.Background()
	result, err := detector.DetectProject(ctx, tempDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Languages) != 0 {
		t.Error("Expected no languages to be detected in empty directory")
	}
}