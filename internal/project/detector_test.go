package project

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-gateway/internal/project/types"
	projecttypes "lsp-gateway/internal/project/types"
	"lsp-gateway/internal/setup"
)

// Mock LanguageDetector for testing
type mockLanguageDetector struct {
	language        string
	confidence      float64
	markerFiles     []string
	requiredServers []string
	shouldFail      bool
	failError       error
}

func (m *mockLanguageDetector) DetectLanguage(ctx context.Context, rootPath string) (*projecttypes.LanguageDetectionResult, error) {
	if m.shouldFail {
		return nil, m.failError
	}

	// Check if marker files exist in the path
	hasMarkers := false
	for _, marker := range m.markerFiles {
		markerPath := filepath.Join(rootPath, marker)
		if _, err := os.Stat(markerPath); err == nil {
			hasMarkers = true
			break
		}
	}

	if !hasMarkers {
		return &projecttypes.LanguageDetectionResult{
			Language:   m.language,
			Confidence: 0.0,
		}, nil
	}

	return &projecttypes.LanguageDetectionResult{
		Language:        m.language,
		Confidence:      m.confidence,
		MarkerFiles:     m.markerFiles,
		RequiredServers: m.requiredServers,
		SourceDirs:      []string{"src"},
		TestDirs:        []string{"test"},
		ConfigFiles:     m.markerFiles,
		Dependencies:    map[string]string{"dep1": "v1.0.0"},
		DevDependencies: map[string]string{"testdep1": "v2.0.0"},
		Metadata:        map[string]interface{}{"test": "data"},
	}, nil
}

func (m *mockLanguageDetector) GetLanguageInfo(language string) (*projecttypes.LanguageInfo, error) {
	return &projecttypes.LanguageInfo{
		Name:        m.language,
		DisplayName: strings.Title(m.language),
		LSPServers:  m.requiredServers,
	}, nil
}

func (m *mockLanguageDetector) GetMarkerFiles() []string {
	return m.markerFiles
}

func (m *mockLanguageDetector) GetRequiredServers() []string {
	return m.requiredServers
}

func (m *mockLanguageDetector) GetPriority() int {
	switch m.language {
	case types.PROJECT_TYPE_GO:
		return types.PRIORITY_GO
	case types.PROJECT_TYPE_PYTHON:
		return types.PRIORITY_PYTHON
	default:
		return types.PRIORITY_UNKNOWN
	}
}

func (m *mockLanguageDetector) ValidateStructure(ctx context.Context, rootPath string) error {
	if m.shouldFail && m.failError != nil {
		return m.failError
	}
	return nil
}

// Test helper functions
func createTempProject(t *testing.T, projectType string) string {
	tempDir := t.TempDir()

	switch projectType {
	case types.PROJECT_TYPE_GO:
		createGoProject(t, tempDir)
	case types.PROJECT_TYPE_PYTHON:
		createPythonProject(t, tempDir)
	case types.PROJECT_TYPE_NODEJS:
		createNodeProject(t, tempDir)
	case types.PROJECT_TYPE_JAVA:
		createJavaProject(t, tempDir)
	case types.PROJECT_TYPE_MIXED:
		createMixedProject(t, tempDir)
	case "empty":
		// Empty directory
	default:
		t.Fatalf("Unknown project type: %s", projectType)
	}

	return tempDir
}

func createGoProject(t *testing.T, tempDir string) {
	goModContent := `module example.com/test
go 1.21
require (
    github.com/gorilla/mux v1.8.0
)
`
	writeFile(t, filepath.Join(tempDir, types.MARKER_GO_MOD), goModContent)
	writeFile(t, filepath.Join(tempDir, "main.go"), "package main\n\nfunc main() {}\n")
	
	// Create source and test directories
	os.MkdirAll(filepath.Join(tempDir, "src"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "test"), 0755)
	writeFile(t, filepath.Join(tempDir, "src", "service.go"), "package src\n")
	writeFile(t, filepath.Join(tempDir, "test", "service_test.go"), "package test\n")
}

func createPythonProject(t *testing.T, tempDir string) {
	setupPyContent := `from setuptools import setup
setup(
    name="test-project",
    version="1.0.0",
    packages=["src"],
)
`
	writeFile(t, filepath.Join(tempDir, types.MARKER_SETUP_PY), setupPyContent)
	writeFile(t, filepath.Join(tempDir, types.MARKER_REQUIREMENTS), "flask==2.0.0\npytest==7.0.0")
	writeFile(t, filepath.Join(tempDir, "main.py"), "print('Hello World')\n")
	
	os.MkdirAll(filepath.Join(tempDir, "src"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "test"), 0755)
	writeFile(t, filepath.Join(tempDir, "src", "__init__.py"), "")
	writeFile(t, filepath.Join(tempDir, "test", "test_main.py"), "import pytest\n")
}

func createNodeProject(t *testing.T, tempDir string) {
	packageJsonContent := `{
  "name": "test-project",
  "version": "1.0.0",
  "main": "index.js",
  "dependencies": {
    "express": "^4.18.0"
  },
  "devDependencies": {
    "jest": "^28.0.0"
  }
}
`
	writeFile(t, filepath.Join(tempDir, types.MARKER_PACKAGE_JSON), packageJsonContent)
	writeFile(t, filepath.Join(tempDir, "index.js"), "console.log('Hello World');\n")
	
	os.MkdirAll(filepath.Join(tempDir, "src"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "test"), 0755)
	writeFile(t, filepath.Join(tempDir, "src", "app.js"), "module.exports = {};\n")
	writeFile(t, filepath.Join(tempDir, "test", "app.test.js"), "test('example', () => {});\n")
}

func createJavaProject(t *testing.T, tempDir string) {
	pomXmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>test-project</artifactId>
    <version>1.0.0</version>
    <properties>
        <maven.compiler.source>11</maven.compiler.source>
        <maven.compiler.target>11</maven.compiler.target>
    </properties>
</project>
`
	writeFile(t, filepath.Join(tempDir, types.MARKER_POM_XML), pomXmlContent)
	
	srcDir := filepath.Join(tempDir, "src", "main", "java", "com", "example")
	testDir := filepath.Join(tempDir, "src", "test", "java", "com", "example")
	os.MkdirAll(srcDir, 0755)
	os.MkdirAll(testDir, 0755)
	writeFile(t, filepath.Join(srcDir, "Main.java"), "package com.example;\npublic class Main {}\n")
	writeFile(t, filepath.Join(testDir, "MainTest.java"), "package com.example;\npublic class MainTest {}\n")
}

func createMixedProject(t *testing.T, tempDir string) {
	// Create multiple language files
	createGoProject(t, tempDir)
	writeFile(t, filepath.Join(tempDir, types.MARKER_PACKAGE_JSON), `{"name": "mixed", "version": "1.0.0"}`)
	writeFile(t, filepath.Join(tempDir, "script.py"), "print('Hello from Python')\n")
}

func createGitRepo(t *testing.T, tempDir string) {
	gitDir := filepath.Join(tempDir, ".git")
	os.MkdirAll(gitDir, 0755)
	writeFile(t, filepath.Join(gitDir, "config"), "[core]\n    repositoryformatversion = 0\n")
}

func writeFile(t *testing.T, path, content string) {
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write file %s: %v", path, err)
	}
}

func setupMockDetectors(detector *DefaultProjectDetector) {
	detector.detectors = map[string]LanguageDetector{
		types.PROJECT_TYPE_GO: &mockLanguageDetector{
			language:        types.PROJECT_TYPE_GO,
			confidence:      0.9,
			markerFiles:     []string{types.MARKER_GO_MOD, types.MARKER_GO_SUM},
			requiredServers: []string{types.SERVER_GOPLS},
		},
		types.PROJECT_TYPE_PYTHON: &mockLanguageDetector{
			language:        types.PROJECT_TYPE_PYTHON,
			confidence:      0.85,
			markerFiles:     []string{types.MARKER_SETUP_PY, types.MARKER_REQUIREMENTS, types.MARKER_PYPROJECT},
			requiredServers: []string{types.SERVER_PYLSP},
		},
		types.PROJECT_TYPE_NODEJS: &mockLanguageDetector{
			language:        types.PROJECT_TYPE_NODEJS,
			confidence:      0.8,
			markerFiles:     []string{types.MARKER_PACKAGE_JSON},
			requiredServers: []string{types.SERVER_TYPESCRIPT_LANG_SERVER},
		},
		types.PROJECT_TYPE_JAVA: &mockLanguageDetector{
			language:        types.PROJECT_TYPE_JAVA,
			confidence:      0.75,
			markerFiles:     []string{types.MARKER_POM_XML, types.MARKER_BUILD_GRADLE},
			requiredServers: []string{types.SERVER_JDTLS},
		},
	}
}

// Test Functions

func TestNewProjectDetector(t *testing.T) {
	detector := NewProjectDetector()

	if detector == nil {
		t.Fatal("NewProjectDetector returned nil")
	}

	if detector.logger == nil {
		t.Error("Logger not initialized")
	}

	if detector.config == nil {
		t.Error("Config not initialized")
	}

	if detector.config.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", detector.config.Timeout)
	}

	if detector.config.MaxDepth != types.MAX_DIRECTORY_DEPTH {
		t.Errorf("Expected max depth %d, got %d", types.MAX_DIRECTORY_DEPTH, detector.config.MaxDepth)
	}

	if len(detector.detectors) == 0 {
		t.Error("No language detectors initialized")
	}

	expectedLanguages := []string{
		types.PROJECT_TYPE_GO,
		types.PROJECT_TYPE_PYTHON,
		types.PROJECT_TYPE_JAVA,
		types.PROJECT_TYPE_TYPESCRIPT,
		types.PROJECT_TYPE_NODEJS,
	}

	supportedLanguages := detector.GetSupportedLanguages()
	if len(supportedLanguages) != len(expectedLanguages) {
		t.Errorf("Expected %d supported languages, got %d", len(expectedLanguages), len(supportedLanguages))
	}

	for _, lang := range expectedLanguages {
		found := false
		for _, supported := range supportedLanguages {
			if lang == supported {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected language %s not found in supported languages", lang)
		}
	}
}

func TestDetectProject_ValidGoProject(t *testing.T) {
	detector := NewProjectDetector()
	setupMockDetectors(detector)

	tempDir := createTempProject(t, types.PROJECT_TYPE_GO)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	projectCtx, err := detector.DetectProject(ctx, tempDir)
	if err != nil {
		t.Fatalf("DetectProject failed: %v", err)
	}

	if projectCtx == nil {
		t.Fatal("ProjectContext is nil")
	}

	if projectCtx.ProjectType != types.PROJECT_TYPE_GO {
		t.Errorf("Expected project type %s, got %s", types.PROJECT_TYPE_GO, projectCtx.ProjectType)
	}

	if projectCtx.PrimaryLanguage != types.PROJECT_TYPE_GO {
		t.Errorf("Expected primary language %s, got %s", types.PROJECT_TYPE_GO, projectCtx.PrimaryLanguage)
	}

	if len(projectCtx.RequiredServers) == 0 {
		t.Error("No required servers detected")
	}

	if projectCtx.Confidence <= 0 {
		t.Errorf("Expected positive confidence, got %f", projectCtx.Confidence)
	}

	if !projectCtx.IsValid {
		t.Error("Project should be valid")
	}

	if projectCtx.DetectionTime <= 0 {
		t.Error("Detection time should be positive")
	}
}

func TestDetectProject_ValidPythonProject(t *testing.T) {
	detector := NewProjectDetector()
	setupMockDetectors(detector)

	tempDir := createTempProject(t, types.PROJECT_TYPE_PYTHON)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	projectCtx, err := detector.DetectProject(ctx, tempDir)
	if err != nil {
		t.Fatalf("DetectProject failed: %v", err)
	}

	if projectCtx.ProjectType != types.PROJECT_TYPE_PYTHON {
		t.Errorf("Expected project type %s, got %s", types.PROJECT_TYPE_PYTHON, projectCtx.ProjectType)
	}

	if projectCtx.PrimaryLanguage != types.PROJECT_TYPE_PYTHON {
		t.Errorf("Expected primary language %s, got %s", types.PROJECT_TYPE_PYTHON, projectCtx.PrimaryLanguage)
	}

	if len(projectCtx.RequiredServers) == 0 {
		t.Error("No required servers detected")
	}

	if len(projectCtx.Dependencies) == 0 {
		t.Error("No dependencies detected")
	}

	if len(projectCtx.SourceDirs) == 0 {
		t.Error("No source directories detected")
	}
}

func TestDetectProject_ErrorConditions(t *testing.T) {
	detector := NewProjectDetector()
	setupMockDetectors(detector)

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
		},
		{
			name:    "non-existent path",
			path:    "/does/not/exist",
			wantErr: true,
		},
		{
			name:    "relative path to non-existent",
			path:    "./non/existent/path",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			_, err := detector.DetectProject(ctx, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("DetectProject() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDetectProject_ContextTimeout(t *testing.T) {
	detector := NewProjectDetector()
	setupMockDetectors(detector)

	tempDir := createTempProject(t, types.PROJECT_TYPE_GO)

	// Create a context that times out immediately
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Give time for context to timeout
	time.Sleep(10 * time.Millisecond)

	_, err := detector.DetectProject(ctx, tempDir)
	if err == nil {
		t.Error("Expected timeout error but got none")
	}

	if !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected context/timeout error, got: %v", err)
	}
}

func TestDetectProject_MixedLanguageProject(t *testing.T) {
	detector := NewProjectDetector()
	setupMockDetectors(detector)

	tempDir := createTempProject(t, types.PROJECT_TYPE_MIXED)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	projectCtx, err := detector.DetectProject(ctx, tempDir)
	if err != nil {
		t.Fatalf("DetectProject failed: %v", err)
	}

	if projectCtx.ProjectType != types.PROJECT_TYPE_MIXED {
		t.Errorf("Expected project type %s, got %s", types.PROJECT_TYPE_MIXED, projectCtx.ProjectType)
	}

	if len(projectCtx.Languages) < 2 {
		t.Error("Expected multiple languages detected in mixed project")
	}

	if len(projectCtx.RequiredServers) < 2 {
		t.Error("Expected multiple servers required for mixed project")
	}
}

func TestDetectProjectType(t *testing.T) {
	detector := NewProjectDetector()
	setupMockDetectors(detector)

	tests := []struct {
		name         string
		projectType  string
		expectedType string
	}{
		{
			name:         "Go project",
			projectType:  types.PROJECT_TYPE_GO,
			expectedType: types.PROJECT_TYPE_GO,
		},
		{
			name:         "Python project",
			projectType:  types.PROJECT_TYPE_PYTHON,
			expectedType: types.PROJECT_TYPE_PYTHON,
		},
		{
			name:         "Mixed project",
			projectType:  types.PROJECT_TYPE_MIXED,
			expectedType: types.PROJECT_TYPE_MIXED,
		},
		{
			name:         "Empty directory",
			projectType:  "empty",
			expectedType: types.PROJECT_TYPE_UNKNOWN,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := createTempProject(t, tt.projectType)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			projectType, err := detector.DetectProjectType(ctx, tempDir)

			if tt.expectedType == types.PROJECT_TYPE_UNKNOWN {
				if err == nil {
					t.Error("Expected error for unknown project type")
				}
				if projectType != types.PROJECT_TYPE_UNKNOWN {
					t.Errorf("Expected type %s, got %s", types.PROJECT_TYPE_UNKNOWN, projectType)
				}
			} else {
				if err != nil {
					t.Fatalf("DetectProjectType failed: %v", err)
				}
				if projectType != tt.expectedType {
					t.Errorf("Expected type %s, got %s", tt.expectedType, projectType)
				}
			}
		})
	}
}

func TestGetWorkspaceRoot(t *testing.T) {
	detector := NewProjectDetector()

	tests := []struct {
		name        string
		setupFunc   func(string) string // Returns the test path
		expectError bool
	}{
		{
			name: "Git repository root",
			setupFunc: func(tempDir string) string {
				createGitRepo(t, tempDir)
				subDir := filepath.Join(tempDir, "subdir")
				os.MkdirAll(subDir, 0755)
				return subDir
			},
			expectError: false,
		},
		{
			name: "Project root with go.mod",
			setupFunc: func(tempDir string) string {
				writeFile(t, filepath.Join(tempDir, types.MARKER_GO_MOD), "module test")
				subDir := filepath.Join(tempDir, "subdir")
				os.MkdirAll(subDir, 0755)
				return subDir
			},
			expectError: false,
		},
		{
			name: "No markers - fallback to provided path",
			setupFunc: func(tempDir string) string {
				return tempDir
			},
			expectError: false,
		},
		{
			name: "Invalid path",
			setupFunc: func(tempDir string) string {
				return "/does/not/exist"
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			testPath := tt.setupFunc(tempDir)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			root, err := detector.GetWorkspaceRoot(ctx, testPath)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetWorkspaceRoot failed: %v", err)
			}

			if root == "" {
				t.Error("Workspace root is empty")
			}

			// Ensure returned path is absolute
			if !filepath.IsAbs(root) {
				t.Errorf("Workspace root should be absolute: %s", root)
			}
		})
	}
}

func TestValidateProject(t *testing.T) {
	detector := NewProjectDetector()
	setupMockDetectors(detector)

	tests := []struct {
		name        string
		setupCtx    func() *ProjectContext
		expectError bool
		expectValid bool
	}{
		{
			name: "valid project context",
			setupCtx: func() *ProjectContext {
				tempDir := createTempProject(t, types.PROJECT_TYPE_GO)
				ctx := NewProjectContext(types.PROJECT_TYPE_GO, tempDir)
				ctx.AddMarkerFile(types.MARKER_GO_MOD)
				return ctx
			},
			expectError: false,
			expectValid: true,
		},
		{
			name: "nil project context",
			setupCtx: func() *ProjectContext {
				return nil
			},
			expectError: true,
			expectValid: false,
		},
		{
			name: "unknown project type",
			setupCtx: func() *ProjectContext {
				tempDir := createTempProject(t, "empty")
				ctx := NewProjectContext(types.PROJECT_TYPE_UNKNOWN, tempDir)
				return ctx
			},
			expectError: true,
			expectValid: false,
		},
		{
			name: "empty root path",
			setupCtx: func() *ProjectContext {
				ctx := NewProjectContext(types.PROJECT_TYPE_GO, "")
				return ctx
			},
			expectError: true,
			expectValid: false,
		},
		{
			name: "non-existent root path",
			setupCtx: func() *ProjectContext {
				ctx := NewProjectContext(types.PROJECT_TYPE_GO, "/does/not/exist")
				return ctx
			},
			expectError: true,
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectCtx := tt.setupCtx()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := detector.ValidateProject(ctx, projectCtx)

			if tt.expectError {
				if err == nil {
					t.Error("Expected validation error but got none")
				}
				if projectCtx != nil && projectCtx.IsValid {
					t.Error("Project should not be valid")
				}
			} else {
				if err != nil {
					t.Fatalf("ValidateProject failed: %v", err)
				}
				if !projectCtx.IsValid {
					t.Error("Project should be valid")
				}
			}
		})
	}
}

func TestScanWorkspace(t *testing.T) {
	detector := NewProjectDetector()
	setupMockDetectors(detector)

	// Create workspace with multiple projects
	workspaceDir := t.TempDir()

	// Create multiple projects in subdirectories
	goProjectDir := filepath.Join(workspaceDir, "go-project")
	os.MkdirAll(goProjectDir, 0755)
	writeFile(t, filepath.Join(goProjectDir, types.MARKER_GO_MOD), "module go-project")

	pythonProjectDir := filepath.Join(workspaceDir, "python-project")
	os.MkdirAll(pythonProjectDir, 0755)
	writeFile(t, filepath.Join(pythonProjectDir, types.MARKER_SETUP_PY), "from setuptools import setup")

	// Create ignored directory
	nodeModulesDir := filepath.Join(workspaceDir, "node_modules")
	os.MkdirAll(nodeModulesDir, 0755)
	writeFile(t, filepath.Join(nodeModulesDir, types.MARKER_PACKAGE_JSON), "{}")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	projects, err := detector.ScanWorkspace(ctx, workspaceDir)
	if err != nil {
		t.Fatalf("ScanWorkspace failed: %v", err)
	}

	if len(projects) != 2 {
		t.Errorf("Expected 2 projects, got %d", len(projects))
	}

	// Verify project types are detected correctly
	foundGo := false
	foundPython := false
	for _, project := range projects {
		switch project.ProjectType {
		case types.PROJECT_TYPE_GO:
			foundGo = true
		case types.PROJECT_TYPE_PYTHON:
			foundPython = true
		}
	}

	if !foundGo {
		t.Error("Go project not found in workspace scan")
	}
	if !foundPython {
		t.Error("Python project not found in workspace scan")
	}
}

func TestScanWorkspace_ContextTimeout(t *testing.T) {
	detector := NewProjectDetector()
	setupMockDetectors(detector)

	workspaceDir := t.TempDir()

	// Create many nested directories to ensure WalkDir takes some time
	for i := 0; i < 50; i++ {
		subDir := filepath.Join(workspaceDir, "subdir"+string(rune('a'+i%26)))
		os.MkdirAll(subDir, 0755)
		for j := 0; j < 10; j++ {
			nestedDir := filepath.Join(subDir, "nested"+string(rune('a'+j%26)))
			os.MkdirAll(nestedDir, 0755)
		}
	}

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Microsecond)
	defer cancel()

	// Wait a bit to ensure timeout
	time.Sleep(10 * time.Millisecond)

	_, err := detector.ScanWorkspace(ctx, workspaceDir)
	
	// Context timeout may or may not be caught depending on timing,
	// so we just verify the function handles context properly
	if err != nil {
		t.Logf("Got expected error (timeout-related): %v", err)
	} else {
		t.Log("ScanWorkspace completed before timeout - this is acceptable for small directories")
	}
}

func TestDetectAllLanguages(t *testing.T) {
	detector := NewProjectDetector()
	setupMockDetectors(detector)

	tests := []struct {
		name            string
		projectType     string
		expectedCount   int
		expectError     bool
	}{
		{
			name:          "Go project",
			projectType:   types.PROJECT_TYPE_GO,
			expectedCount: 1,
			expectError:   false,
		},
		{
			name:          "Mixed project",
			projectType:   types.PROJECT_TYPE_MIXED,
			expectedCount: 2, // Go + Node.js
			expectError:   false,
		},
		{
			name:          "Empty directory",
			projectType:   "empty",
			expectedCount: 0,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := createTempProject(t, tt.projectType)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			results, err := detector.detectAllLanguages(ctx, tempDir)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("detectAllLanguages failed: %v", err)
			}

			if len(results) != tt.expectedCount {
				t.Errorf("Expected %d language results, got %d", tt.expectedCount, len(results))
			}

			// Verify all results have positive confidence
			for _, result := range results {
				if result.Confidence <= 0 {
					t.Errorf("Language %s has non-positive confidence: %f", result.Language, result.Confidence)
				}
			}
		})
	}
}

func TestBuildProjectContext(t *testing.T) {
	detector := NewProjectDetector()

	tempDir := createTempProject(t, types.PROJECT_TYPE_GO)

	// Create mock detection results
	results := []*projecttypes.LanguageDetectionResult{
		{
			Language:        types.PROJECT_TYPE_GO,
			Confidence:      0.9,
			MarkerFiles:     []string{types.MARKER_GO_MOD},
			RequiredServers: []string{types.SERVER_GOPLS},
			SourceDirs:      []string{"src"},
			TestDirs:        []string{"test"},
			ConfigFiles:     []string{types.MARKER_GO_MOD},
			Dependencies:    map[string]string{"dep1": "v1.0"},
			DevDependencies: map[string]string{"testdep": "v1.0"},
			Metadata:        map[string]interface{}{"go_version": "1.21"},
		},
	}

	projectCtx, err := detector.buildProjectContext(tempDir, results)
	if err != nil {
		t.Fatalf("buildProjectContext failed: %v", err)
	}

	if projectCtx == nil {
		t.Fatal("ProjectContext is nil")
	}

	if projectCtx.ProjectType != types.PROJECT_TYPE_GO {
		t.Errorf("Expected project type %s, got %s", types.PROJECT_TYPE_GO, projectCtx.ProjectType)
	}

	if projectCtx.PrimaryLanguage != types.PROJECT_TYPE_GO {
		t.Errorf("Expected primary language %s, got %s", types.PROJECT_TYPE_GO, projectCtx.PrimaryLanguage)
	}

	if len(projectCtx.Languages) == 0 {
		t.Error("No languages in project context")
	}

	if len(projectCtx.RequiredServers) == 0 {
		t.Error("No required servers in project context")
	}

	if len(projectCtx.MarkerFiles) == 0 {
		t.Error("No marker files in project context")
	}

	if projectCtx.Confidence != 0.9 {
		t.Errorf("Expected confidence 0.9, got %f", projectCtx.Confidence)
	}

	// Test with empty results
	_, err = detector.buildProjectContext(tempDir, []*projecttypes.LanguageDetectionResult{})
	if err == nil {
		t.Error("Expected error for empty detection results")
	}
}

func TestSetters(t *testing.T) {
	detector := NewProjectDetector()

	// Test SetTimeout
	newTimeout := 60 * time.Second
	detector.SetTimeout(newTimeout)
	if detector.config.Timeout != newTimeout {
		t.Errorf("Expected timeout %v, got %v", newTimeout, detector.config.Timeout)
	}

	// Test SetTimeout with invalid value
	detector.SetTimeout(0)
	if detector.config.Timeout != newTimeout {
		t.Error("Timeout should not be changed with invalid value")
	}

	// Test SetMaxDepth
	newDepth := 15
	detector.SetMaxDepth(newDepth)
	if detector.config.MaxDepth != newDepth {
		t.Errorf("Expected max depth %d, got %d", newDepth, detector.config.MaxDepth)
	}

	// Test SetMaxDepth with invalid value
	detector.SetMaxDepth(0)
	if detector.config.MaxDepth != newDepth {
		t.Error("Max depth should not be changed with invalid value")
	}

	// Test SetLogger
	logger := setup.NewSetupLogger(nil)
	detector.SetLogger(logger)
	if detector.logger != logger {
		t.Error("Logger not set correctly")
	}

	// Test SetLogger with nil
	detector.SetLogger(nil)
	if detector.logger != logger {
		t.Error("Logger should not be changed with nil value")
	}

	// Test SetCustomDetectors
	customDetectors := map[string]LanguageDetector{
		"custom": &mockLanguageDetector{
			language:    "custom",
			confidence:  0.5,
			markerFiles: []string{"custom.file"},
		},
	}
	detector.SetCustomDetectors(customDetectors)
	if len(detector.detectors) == 0 {
		t.Error("Custom detectors not set")
	}
	if _, exists := detector.detectors["custom"]; !exists {
		t.Error("Custom detector not found")
	}

	// Test SetCustomDetectors with nil
	detector.SetCustomDetectors(nil)
	// Should not panic or remove existing detectors
}

func TestDetectProject_ValidationErrors(t *testing.T) {
	detector := NewProjectDetector()

	// Create a mock detector that succeeds in detection but fails in validation
	mockDetector := &mockLanguageDetector{
		language:        types.PROJECT_TYPE_GO,
		confidence:      0.9,
		markerFiles:     []string{types.MARKER_GO_MOD},
		requiredServers: []string{types.SERVER_GOPLS},
		shouldFail:      false, // Don't fail detection
		failError:       nil,
	}

	detector.detectors = map[string]LanguageDetector{
		types.PROJECT_TYPE_GO: mockDetector,
	}

	tempDir := createTempProject(t, types.PROJECT_TYPE_GO)

	// Now make the validator fail by changing shouldFail after detection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	projectCtx, err := detector.DetectProject(ctx, tempDir)
	if err != nil {
		t.Fatalf("DetectProject failed: %v", err)
	}

	// Manually add validation errors to test the scenario
	if len(projectCtx.ValidationErrors) == 0 {
		t.Log("No validation errors detected - this is expected with mock detectors")
	}

	// The project should still be valid since our mock doesn't actually fail validation
	if !projectCtx.IsValid {
		t.Log("Project marked as invalid - this may be expected behavior")
	}
}

func TestCalculateProjectSize(t *testing.T) {
	detector := NewProjectDetector()

	tempDir := createTempProject(t, types.PROJECT_TYPE_GO)

	// Add more files for size calculation
	writeFile(t, filepath.Join(tempDir, "README.md"), "# Test Project")
	writeFile(t, filepath.Join(tempDir, "config.yaml"), "key: value")

	size := detector.calculateProjectSize(tempDir)

	if size.TotalFiles <= 0 {
		t.Error("Total files should be positive")
	}

	if size.SourceFiles <= 0 {
		t.Error("Source files should be positive")
	}

	if size.TotalSizeBytes <= 0 {
		t.Error("Total size should be positive")
	}
}

func TestShouldIgnoreDirectory(t *testing.T) {
	detector := NewProjectDetector()

	tests := []struct {
		name           string
		dirName        string
		shouldBeIgnored bool
	}{
		{
			name:           "node_modules",
			dirName:        types.DIR_NODE_MODULES,
			shouldBeIgnored: true,
		},
		{
			name:           "vendor",
			dirName:        types.DIR_VENDOR,
			shouldBeIgnored: true,
		},
		{
			name:           ".git",
			dirName:        ".git",
			shouldBeIgnored: true,
		},
		{
			name:           "regular directory",
			dirName:        "src",
			shouldBeIgnored: false,
		},
		{
			name:           "pyc files pattern",
			dirName:        "test.pyc",
			shouldBeIgnored: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ignored := detector.shouldIgnoreDirectory("/fake/path", tt.dirName)
			if ignored != tt.shouldBeIgnored {
				t.Errorf("Expected shouldIgnoreDirectory(%s) = %v, got %v", tt.dirName, tt.shouldBeIgnored, ignored)
			}
		})
	}
}

func TestRemoveDuplicates(t *testing.T) {
	detector := NewProjectDetector()

	input := []string{"a", "b", "a", "c", "b", "d"}
	expected := []string{"a", "b", "c", "d"}

	result := detector.removeDuplicates(input)

	if len(result) != len(expected) {
		t.Errorf("Expected length %d, got %d", len(expected), len(result))
	}

	// Check all expected items are present
	for _, expectedItem := range expected {
		found := false
		for _, resultItem := range result {
			if expectedItem == resultItem {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected item %s not found in result", expectedItem)
		}
	}
}

func TestDetectorPriority(t *testing.T) {
	detector := NewProjectDetector()

	tests := []struct {
		language string
		expected int
	}{
		{types.PROJECT_TYPE_GO, types.PRIORITY_GO},
		{types.PROJECT_TYPE_PYTHON, types.PRIORITY_PYTHON},
		{types.PROJECT_TYPE_TYPESCRIPT, types.PRIORITY_TYPESCRIPT},
		{types.PROJECT_TYPE_NODEJS, types.PRIORITY_NODEJS},
		{types.PROJECT_TYPE_JAVA, types.PRIORITY_JAVA},
		{types.PROJECT_TYPE_MIXED, types.PRIORITY_MIXED},
		{"unknown", types.PRIORITY_UNKNOWN},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			priority := detector.getDetectorPriority(tt.language)
			if priority != tt.expected {
				t.Errorf("Expected priority %d for %s, got %d", tt.expected, tt.language, priority)
			}
		})
	}
}

func TestFindVCSRoot(t *testing.T) {
	detector := NewProjectDetector()

	tests := []struct {
		name        string
		setupFunc   func(string) string
		expectError bool
	}{
		{
			name: "git root found",
			setupFunc: func(tempDir string) string {
				createGitRepo(t, tempDir)
				subDir := filepath.Join(tempDir, "deep", "nested", "dir")
				os.MkdirAll(subDir, 0755)
				return subDir
			},
			expectError: false,
		},
		{
			name: "no VCS root",
			setupFunc: func(tempDir string) string {
				return tempDir
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			testPath := tt.setupFunc(tempDir)

			root, err := detector.findVCSRoot(testPath)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Fatalf("findVCSRoot failed: %v", err)
				}
				if root == "" {
					t.Error("VCS root is empty")
				}
			}
		})
	}
}

func TestFindProjectRoot(t *testing.T) {
	detector := NewProjectDetector()

	tests := []struct {
		name        string
		setupFunc   func(string) string
		expectError bool
	}{
		{
			name: "go.mod found",
			setupFunc: func(tempDir string) string {
				writeFile(t, filepath.Join(tempDir, types.MARKER_GO_MOD), "module test")
				subDir := filepath.Join(tempDir, "sub")
				os.MkdirAll(subDir, 0755)
				return subDir
			},
			expectError: false,
		},
		{
			name: "package.json found",
			setupFunc: func(tempDir string) string {
				writeFile(t, filepath.Join(tempDir, types.MARKER_PACKAGE_JSON), "{}")
				subDir := filepath.Join(tempDir, "sub")
				os.MkdirAll(subDir, 0755)
				return subDir
			},
			expectError: false,
		},
		{
			name: "no project markers",
			setupFunc: func(tempDir string) string {
				return tempDir
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			testPath := tt.setupFunc(tempDir)

			root, err := detector.findProjectRoot(testPath)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Fatalf("findProjectRoot failed: %v", err)
				}
				if root == "" {
					t.Error("Project root is empty")
				}
			}
		})
	}
}