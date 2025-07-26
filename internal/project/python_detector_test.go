package project

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-gateway/internal/project/types"
)

func TestNewPythonLanguageDetector(t *testing.T) {
	detector := NewPythonLanguageDetector()
	
	if detector == nil {
		t.Fatal("Expected detector to be non-nil")
	}
	
	pythonDetector, ok := detector.(*PythonLanguageDetector)
	if !ok {
		t.Fatal("Expected detector to be of type *PythonLanguageDetector")
	}
	
	if pythonDetector.logger == nil {
		t.Error("Expected logger to be initialized")
	}
}

func TestDetectLanguage_ModernProject(t *testing.T) {
	testDir := createModernPythonProject(t)
	defer os.RemoveAll(testDir)
	
	detector := NewPythonLanguageDetector()
	ctx := context.Background()
	
	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}
	
	// Verify basic properties
	if result.Language != types.PROJECT_TYPE_PYTHON {
		t.Errorf("Expected language %s, got %s", types.PROJECT_TYPE_PYTHON, result.Language)
	}
	
	if result.Confidence != 0.95 {
		t.Errorf("Expected confidence 0.95 for pyproject.toml, got %f", result.Confidence)
	}
	
	// Verify marker files
	expectedMarkers := []string{types.MARKER_PYPROJECT}
	if !slicesEqual(result.MarkerFiles, expectedMarkers) {
		t.Errorf("Expected marker files %v, got %v", expectedMarkers, result.MarkerFiles)
	}
	
	// Verify required servers
	expectedServers := []string{types.SERVER_PYLSP}
	if !slicesEqual(result.RequiredServers, expectedServers) {
		t.Errorf("Expected servers %v, got %v", expectedServers, result.RequiredServers)
	}
	
	// Verify source directories found
	if len(result.SourceDirs) == 0 {
		t.Error("Expected source directories to be detected")
	}
	
	// Verify test directories found
	if len(result.TestDirs) == 0 {
		t.Error("Expected test directories to be detected")
	}
	
	// Verify metadata
	if pyFileCount, ok := result.Metadata["python_file_count"]; !ok || pyFileCount.(int) < 1 {
		t.Error("Expected python_file_count metadata with positive value")
	}
}

func TestDetectLanguage_LegacyProject(t *testing.T) {
	testDir := createLegacyPythonProject(t)
	defer os.RemoveAll(testDir)
	
	detector := NewPythonLanguageDetector()
	ctx := context.Background()
	
	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}
	
	if result.Language != types.PROJECT_TYPE_PYTHON {
		t.Errorf("Expected language %s, got %s", types.PROJECT_TYPE_PYTHON, result.Language)
	}
	
	if result.Confidence != 0.9 {
		t.Errorf("Expected confidence 0.9 for setup.py, got %f", result.Confidence)
	}
	
	expectedMarkers := []string{types.MARKER_SETUP_PY}
	if !slicesEqual(result.MarkerFiles, expectedMarkers) {
		t.Errorf("Expected marker files %v, got %v", expectedMarkers, result.MarkerFiles)
	}
}

func TestDetectLanguage_SimpleProject(t *testing.T) {
	testDir := createSimplePythonProject(t)
	defer os.RemoveAll(testDir)
	
	detector := NewPythonLanguageDetector()
	ctx := context.Background()
	
	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}
	
	if result.Language != types.PROJECT_TYPE_PYTHON {
		t.Errorf("Expected language %s, got %s", types.PROJECT_TYPE_PYTHON, result.Language)
	}
	
	if result.Confidence != 0.85 {
		t.Errorf("Expected confidence 0.85 for requirements.txt, got %f", result.Confidence)
	}
	
	expectedMarkers := []string{types.MARKER_REQUIREMENTS}
	if !slicesEqual(result.MarkerFiles, expectedMarkers) {
		t.Errorf("Expected marker files %v, got %v", expectedMarkers, result.MarkerFiles)
	}
}

func TestDetectLanguage_PipfileProject(t *testing.T) {
	testDir := createPipfilePythonProject(t)
	defer os.RemoveAll(testDir)
	
	detector := NewPythonLanguageDetector()
	ctx := context.Background()
	
	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}
	
	if result.Language != types.PROJECT_TYPE_PYTHON {
		t.Errorf("Expected language %s, got %s", types.PROJECT_TYPE_PYTHON, result.Language)
	}
	
	if result.Confidence != 0.8 {
		t.Errorf("Expected confidence 0.8 for Pipfile, got %f", result.Confidence)
	}
	
	expectedMarkers := []string{types.MARKER_PIPFILE}
	if !slicesEqual(result.MarkerFiles, expectedMarkers) {
		t.Errorf("Expected marker files %v, got %v", expectedMarkers, result.MarkerFiles)
	}
}

func TestDetectLanguage_PythonFilesOnly(t *testing.T) {
	testDir := createPythonFilesOnlyProject(t)
	defer os.RemoveAll(testDir)
	
	detector := NewPythonLanguageDetector()
	ctx := context.Background()
	
	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}
	
	if result.Language != types.PROJECT_TYPE_PYTHON {
		t.Errorf("Expected language %s, got %s", types.PROJECT_TYPE_PYTHON, result.Language)
	}
	
	if result.Confidence != 0.8 {
		t.Errorf("Expected confidence 0.8 for Python files without project files, got %f", result.Confidence)
	}
	
	if len(result.MarkerFiles) != 0 {
		t.Errorf("Expected no marker files, got %v", result.MarkerFiles)
	}
	
	if pyFileCount, ok := result.Metadata["python_file_count"]; !ok || pyFileCount.(int) < 1 {
		t.Error("Expected python_file_count metadata with positive value")
	}
}

func TestDetectLanguage_NoPythonFiles(t *testing.T) {
	testDir := createNoPythonFilesProject(t)
	defer os.RemoveAll(testDir)
	
	detector := NewPythonLanguageDetector()
	ctx := context.Background()
	
	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}
	
	if result.Language != types.PROJECT_TYPE_PYTHON {
		t.Errorf("Expected language %s, got %s", types.PROJECT_TYPE_PYTHON, result.Language)
	}
	
	if result.Confidence != 0.1 {
		t.Errorf("Expected confidence 0.1 for no Python indicators, got %f", result.Confidence)
	}
	
	if len(result.MarkerFiles) != 0 {
		t.Errorf("Expected no marker files, got %v", result.MarkerFiles)
	}
}

func TestDetectLanguage_DirectorySkipping(t *testing.T) {
	testDir := createPythonProjectWithIgnoreDirs(t)
	defer os.RemoveAll(testDir)
	
	detector := NewPythonLanguageDetector()
	ctx := context.Background()
	
	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}
	
	// Should only count Python files in non-ignored directories
	if pyFileCount, ok := result.Metadata["python_file_count"]; !ok {
		t.Error("Expected python_file_count metadata")
	} else if pyFileCount.(int) != 2 { // Only main.py and utils.py should be counted
		t.Errorf("Expected 2 Python files to be counted (ignoring cache dirs), got %d", pyFileCount.(int))
	}
	
	// Should not include ignored directories in source dirs
	for _, dir := range result.SourceDirs {
		if dir == "__pycache__" || dir == ".pytest_cache" || dir == "venv" || dir == ".venv" || dir == "build" || dir == "dist" {
			t.Errorf("Ignored directory %s should not be in source dirs", dir)
		}
	}
}

func TestDetectLanguage_SourceVsTestDirs(t *testing.T) {
	testDir := createPythonProjectWithTestStructure(t)
	defer os.RemoveAll(testDir)
	
	detector := NewPythonLanguageDetector()
	ctx := context.Background()
	
	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}
	
	// Verify source directories
	if len(result.SourceDirs) == 0 {
		t.Error("Expected source directories to be detected")
	}
	
	// Verify test directories
	if len(result.TestDirs) == 0 {
		t.Error("Expected test directories to be detected")
	}
	
	// Check that test files are classified correctly
	hasTestDir := false
	for _, dir := range result.TestDirs {
		if strings.Contains(dir, "test") {
			hasTestDir = true
			break
		}
	}
	if !hasTestDir {
		t.Error("Expected test directories to contain 'test' in path")
	}
}

func TestDetectLanguage_MarkerFilePriority(t *testing.T) {
	testDir := createPythonProjectWithMultipleMarkers(t)
	defer os.RemoveAll(testDir)
	
	detector := NewPythonLanguageDetector()
	ctx := context.Background()
	
	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}
	
	// Should pick pyproject.toml (highest priority) and ignore others
	if result.Confidence != 0.95 {
		t.Errorf("Expected confidence 0.95 for pyproject.toml priority, got %f", result.Confidence)
	}
	
	// Should only have one marker file (the highest priority one)
	if len(result.MarkerFiles) != 1 || result.MarkerFiles[0] != types.MARKER_PYPROJECT {
		t.Errorf("Expected only pyproject.toml marker file, got %v", result.MarkerFiles)
	}
}

func TestDetectLanguage_DifferentPythonExtensions(t *testing.T) {
	testDir := createPythonProjectWithDifferentExtensions(t)
	defer os.RemoveAll(testDir)
	
	detector := NewPythonLanguageDetector()
	ctx := context.Background()
	
	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}
	
	// Should detect all Python file types: .py, .pyw, .pyx
	if pyFileCount, ok := result.Metadata["python_file_count"]; !ok {
		t.Error("Expected python_file_count metadata")
	} else if pyFileCount.(int) != 3 { // script.py, window.pyw, extension.pyx
		t.Errorf("Expected 3 Python files (.py, .pyw, .pyx), got %d", pyFileCount.(int))
	}
}

func TestValidateStructure_ValidProject(t *testing.T) {
	testDir := createModernPythonProject(t)
	defer os.RemoveAll(testDir)
	
	detector := NewPythonLanguageDetector()
	ctx := context.Background()
	
	err := detector.ValidateStructure(ctx, testDir)
	if err != nil {
		t.Errorf("ValidateStructure should pass for valid project: %v", err)
	}
}

func TestValidateStructure_NoPythonFiles(t *testing.T) {
	testDir := createEmptyProject(t)
	defer os.RemoveAll(testDir)
	
	detector := NewPythonLanguageDetector()
	ctx := context.Background()
	
	err := detector.ValidateStructure(ctx, testDir)
	if err == nil {
		t.Error("ValidateStructure should fail for project with no Python files")
	}
	
	projectErr, ok := err.(*types.ProjectError)
	if !ok {
		t.Errorf("Expected ProjectError, got %T", err)
	} else {
		if projectErr.Type != types.ProjectErrorTypeValidation {
			t.Errorf("Expected validation error type, got %s", projectErr.Type)
		}
		if projectErr.ProjectType != types.PROJECT_TYPE_PYTHON {
			t.Errorf("Expected Python project type, got %s", projectErr.ProjectType)
		}
	}
}

func TestPythonDetector_GetMarkerFiles(t *testing.T) {
	detector := NewPythonLanguageDetector()
	
	markers := detector.GetMarkerFiles()
	
	expected := []string{
		types.MARKER_PYPROJECT,
		types.MARKER_SETUP_PY,
		types.MARKER_REQUIREMENTS,
		types.MARKER_PIPFILE,
	}
	
	if !slicesEqual(markers, expected) {
		t.Errorf("Expected marker files %v, got %v", expected, markers)
	}
}

func TestPythonDetector_GetRequiredServers(t *testing.T) {
	detector := NewPythonLanguageDetector()
	
	servers := detector.GetRequiredServers()
	
	expected := []string{types.SERVER_PYLSP}
	
	if !slicesEqual(servers, expected) {
		t.Errorf("Expected servers %v, got %v", expected, servers)
	}
}

func TestPythonDetector_GetPriority(t *testing.T) {
	detector := NewPythonLanguageDetector()
	
	priority := detector.GetPriority()
	
	if priority != types.PRIORITY_PYTHON {
		t.Errorf("Expected priority %d, got %d", types.PRIORITY_PYTHON, priority)
	}
}

func TestPythonDetector_GetLanguageInfo(t *testing.T) {
	detector := NewPythonLanguageDetector()
	
	info, err := detector.GetLanguageInfo(types.PROJECT_TYPE_PYTHON)
	if err != nil {
		t.Fatalf("GetLanguageInfo failed: %v", err)
	}
	
	if info.Name != types.PROJECT_TYPE_PYTHON {
		t.Errorf("Expected name %s, got %s", types.PROJECT_TYPE_PYTHON, info.Name)
	}
	
	if info.DisplayName != "Python" {
		t.Errorf("Expected display name 'Python', got %s", info.DisplayName)
	}
	
	expectedExtensions := []string{".py", ".pyw", ".pyx"}
	if !slicesEqual(info.FileExtensions, expectedExtensions) {
		t.Errorf("Expected extensions %v, got %v", expectedExtensions, info.FileExtensions)
	}
	
	expectedServers := []string{types.SERVER_PYLSP}
	if !slicesEqual(info.LSPServers, expectedServers) {
		t.Errorf("Expected servers %v, got %v", expectedServers, info.LSPServers)
	}
}

// Helper functions for creating test data

func createModernPythonProject(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "python-modern-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	
	// Create pyproject.toml
	pyprojectContent := `[tool.poetry]
name = "mypackage"
version = "0.1.0"
description = ""

[tool.poetry.dependencies]
python = "^3.8"
requests = "^2.25.0"

[tool.poetry.dev-dependencies]
pytest = "^6.0.0"
`
	writeFile(t, filepath.Join(tempDir, "pyproject.toml"), pyprojectContent)
	
	// Create source structure
	srcDir := filepath.Join(tempDir, "src", "mypackage")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create src dir: %v", err)
	}
	
	writeFile(t, filepath.Join(srcDir, "__init__.py"), "")
	writeFile(t, filepath.Join(srcDir, "main.py"), "def main():\n    pass\n")
	
	// Create tests
	testDir := filepath.Join(tempDir, "tests")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test dir: %v", err)
	}
	
	writeFile(t, filepath.Join(testDir, "test_main.py"), "def test_main():\n    pass\n")
	
	return tempDir
}

func createLegacyPythonProject(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "python-legacy-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	
	// Create setup.py
	setupContent := `from setuptools import setup, find_packages

setup(
    name="mypackage",
    version="0.1.0",
    packages=find_packages(),
    install_requires=[
        "requests>=2.25.0",
    ],
)`
	writeFile(t, filepath.Join(tempDir, "setup.py"), setupContent)
	
	// Create package structure
	pkgDir := filepath.Join(tempDir, "mypackage")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("Failed to create package dir: %v", err)
	}
	
	writeFile(t, filepath.Join(pkgDir, "__init__.py"), "")
	writeFile(t, filepath.Join(pkgDir, "core.py"), "def function():\n    pass\n")
	
	// Create tests
	testDir := filepath.Join(tempDir, "tests")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test dir: %v", err)
	}
	
	writeFile(t, filepath.Join(testDir, "test_core.py"), "def test_function():\n    pass\n")
	
	return tempDir
}

func createSimplePythonProject(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "python-simple-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	
	// Create requirements.txt
	requirementsContent := `requests>=2.25.0
numpy>=1.20.0
pytest>=6.0.0
`
	writeFile(t, filepath.Join(tempDir, "requirements.txt"), requirementsContent)
	
	// Create simple Python files
	writeFile(t, filepath.Join(tempDir, "main.py"), "import requests\n\ndef main():\n    pass\n")
	writeFile(t, filepath.Join(tempDir, "utils.py"), "def helper():\n    pass\n")
	
	return tempDir
}

func createPipfilePythonProject(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "python-pipfile-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	
	// Create Pipfile
	pipfileContent := `[[source]]
url = "https://pypi.org/simple"
verify_ssl = true
name = "pypi"

[packages]
requests = "*"
numpy = "*"

[dev-packages]
pytest = "*"

[requires]
python_version = "3.8"
`
	writeFile(t, filepath.Join(tempDir, "Pipfile"), pipfileContent)
	writeFile(t, filepath.Join(tempDir, "app.py"), "def app():\n    pass\n")
	
	return tempDir
}

func createPythonFilesOnlyProject(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "python-files-only-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	
	writeFile(t, filepath.Join(tempDir, "script.py"), "print('Hello World')\n")
	writeFile(t, filepath.Join(tempDir, "helper.py"), "def help():\n    pass\n")
	
	return tempDir
}

func createNoPythonFilesProject(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "python-no-files-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	
	writeFile(t, filepath.Join(tempDir, "README.md"), "# No Python Project\n")
	
	return tempDir
}

func createPythonProjectWithIgnoreDirs(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "python-ignore-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	
	// Create main Python files
	writeFile(t, filepath.Join(tempDir, "main.py"), "def main():\n    pass\n")
	writeFile(t, filepath.Join(tempDir, "utils.py"), "def util():\n    pass\n")
	
	// Create ignored directories with Python files
	ignoreDirs := []string{"__pycache__", ".pytest_cache", "venv", ".venv", "build", "dist"}
	for _, dirName := range ignoreDirs {
		ignoreDir := filepath.Join(tempDir, dirName)
		if err := os.MkdirAll(ignoreDir, 0755); err != nil {
			t.Fatalf("Failed to create ignore dir %s: %v", dirName, err)
		}
		
		writeFile(t, filepath.Join(ignoreDir, "ignored.py"), "# This should be ignored\n")
	}
	
	return tempDir
}

func createPythonProjectWithTestStructure(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "python-structure-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	
	// Create source files
	srcDir := filepath.Join(tempDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create src dir: %v", err)
	}
	
	writeFile(t, filepath.Join(srcDir, "main.py"), "def main():\n    pass\n")
	writeFile(t, filepath.Join(srcDir, "module.py"), "def function():\n    pass\n")
	
	// Create test files in different structures
	testDir := filepath.Join(tempDir, "tests")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create tests dir: %v", err)
	}
	
	writeFile(t, filepath.Join(testDir, "test_main.py"), "def test_main():\n    pass\n")
	
	// Create test file with "test" in filename
	writeFile(t, filepath.Join(tempDir, "test_utils.py"), "def test_utils():\n    pass\n")
	
	return tempDir
}

func createPythonProjectWithMultipleMarkers(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "python-multi-markers-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	
	// Create all marker files - should pick pyproject.toml with highest priority
	writeFile(t, filepath.Join(tempDir, "pyproject.toml"), "[tool.poetry]\nname = \"test\"\n")
	writeFile(t, filepath.Join(tempDir, "setup.py"), "from setuptools import setup\n")
	writeFile(t, filepath.Join(tempDir, "requirements.txt"), "requests\n")
	writeFile(t, filepath.Join(tempDir, "Pipfile"), "[packages]\nrequests = \"*\"\n")
	writeFile(t, filepath.Join(tempDir, "main.py"), "def main():\n    pass\n")
	
	return tempDir
}

func createPythonProjectWithDifferentExtensions(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "python-extensions-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	
	// Create files with different Python extensions
	writeFile(t, filepath.Join(tempDir, "script.py"), "def function():\n    pass\n")
	writeFile(t, filepath.Join(tempDir, "window.pyw"), "# Windows Python script\nprint('Hello')\n")
	writeFile(t, filepath.Join(tempDir, "extension.pyx"), "# Cython file\ncdef int add(int a, int b):\n    return a + b\n")
	
	return tempDir
}

func createEmptyProject(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "python-empty-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	
	writeFile(t, filepath.Join(tempDir, "README.md"), "# Empty Project\n")
	
	return tempDir
}

// Utility functions

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}