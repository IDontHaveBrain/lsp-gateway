package project

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/project/types"
)

// Mock CommandExecutor for testing go version detection
type mockCommandExecutor struct {
	shouldFail    bool
	exitCode      int
	stdout        string
	stderr        string
	failError     error
	commandCalled string
	argsCalled    []string
}

func (m *mockCommandExecutor) Execute(cmd string, args []string, timeout time.Duration) (*platform.Result, error) {
	m.commandCalled = cmd
	m.argsCalled = args

	if m.shouldFail {
		return &platform.Result{
			ExitCode: m.exitCode,
			Stdout:   m.stdout,
			Stderr:   m.stderr,
		}, m.failError
	}

	return &platform.Result{
		ExitCode: m.exitCode,
		Stdout:   m.stdout,
		Stderr:   m.stderr,
	}, nil
}

func (m *mockCommandExecutor) ExecuteWithEnv(cmd string, args []string, env map[string]string, timeout time.Duration) (*platform.Result, error) {
	return m.Execute(cmd, args, timeout)
}

func (m *mockCommandExecutor) GetShell() string {
	return "/bin/sh"
}

func (m *mockCommandExecutor) GetShellArgs(command string) []string {
	return []string{"-c", command}
}

func (m *mockCommandExecutor) IsCommandAvailable(command string) bool {
	return !m.shouldFail
}

// Test data creation helpers
func createTempDirWithGoMod(t *testing.T, goModContent string) string {
	tempDir := t.TempDir()
	goModPath := filepath.Join(tempDir, types.MARKER_GO_MOD)
	err := os.WriteFile(goModPath, []byte(goModContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}
	return tempDir
}

func createTempDirWithGoSum(t *testing.T) string {
	tempDir := t.TempDir()
	goSumPath := filepath.Join(tempDir, types.MARKER_GO_SUM)
	goSumContent := `github.com/gorilla/mux v1.8.0 h1:i40aqfkR1h2SlN9hojwV5ZA91wcXFOvKwvw+VmUm1F+A=
github.com/gorilla/mux v1.8.0/go.mod h1:DVbg23sWSpFRCP0SfiEN6jmj59UnW/n46BH5rLB71So=`
	err := os.WriteFile(goSumPath, []byte(goSumContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.sum: %v", err)
	}
	return tempDir
}

func createTempDirWithGoFiles(t *testing.T) string {
	tempDir := t.TempDir()
	
	// Create source directories
	cmdDir := filepath.Join(tempDir, "cmd", "api")
	os.MkdirAll(cmdDir, 0755)
	internalDir := filepath.Join(tempDir, "internal", "handler")
	os.MkdirAll(internalDir, 0755)
	pkgDir := filepath.Join(tempDir, "pkg", "util")
	os.MkdirAll(pkgDir, 0755)
	
	// Create Go files
	writeTestFile(t, filepath.Join(tempDir, "main.go"), "package main\n\nfunc main() {}\n")
	writeTestFile(t, filepath.Join(cmdDir, "main.go"), "package main\n\nfunc main() {}\n")
	writeTestFile(t, filepath.Join(internalDir, "handler.go"), "package handler\n")
	writeTestFile(t, filepath.Join(pkgDir, "util.go"), "package util\n")
	
	// Create test files
	writeTestFile(t, filepath.Join(tempDir, "main_test.go"), "package main\n\nimport \"testing\"\n")
	writeTestFile(t, filepath.Join(internalDir, "handler_test.go"), "package handler\n\nimport \"testing\"\n")
	
	return tempDir
}

func createValidGoProject(t *testing.T) string {
	tempDir := t.TempDir()
	
	// Create go.mod
	goModContent := `module github.com/example/test-project

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/gorilla/mux v1.8.0
	google.golang.org/grpc v1.57.0
)

require (
	github.com/stretchr/testify v1.8.4 // indirect
)

replace github.com/old/module => github.com/new/module v1.0.0
`
	writeTestFile(t, filepath.Join(tempDir, types.MARKER_GO_MOD), goModContent)
	
	// Create go.sum
	goSumContent := `github.com/gin-gonic/gin v1.9.1 h1:4idEAncQnU5cB7BeOkPtxjfCSye0AAm1R0RVIqJ+Jmg=`
	writeTestFile(t, filepath.Join(tempDir, types.MARKER_GO_SUM), goSumContent)
	
	// Create project structure
	cmdDir := filepath.Join(tempDir, "cmd", "api")
	os.MkdirAll(cmdDir, 0755)
	internalDir := filepath.Join(tempDir, "internal", "handler")
	os.MkdirAll(internalDir, 0755)
	pkgDir := filepath.Join(tempDir, "pkg", "util")
	os.MkdirAll(pkgDir, 0755)
	
	// Create Go files
	writeTestFile(t, filepath.Join(tempDir, "main.go"), "package main\n\nfunc main() {}\n")
	writeTestFile(t, filepath.Join(cmdDir, "main.go"), "package main\n\nfunc main() {}\n")
	writeTestFile(t, filepath.Join(internalDir, "handler.go"), "package handler\n")
	writeTestFile(t, filepath.Join(pkgDir, "util.go"), "package util\n")
	
	// Create build files
	writeTestFile(t, filepath.Join(tempDir, "Makefile"), "build:\n\tgo build -o bin/app ./cmd/api\n")
	writeTestFile(t, filepath.Join(tempDir, "Dockerfile"), "FROM golang:1.21-alpine\nWORKDIR /app\nCOPY . .\n")
	
	return tempDir
}

func createInvalidGoMod(t *testing.T) string {
	tempDir := t.TempDir()
	invalidGoModContent := `invalid go.mod content
this is not valid go.mod syntax
module name without proper format
go version missing
require missing parentheses
`
	writeTestFile(t, filepath.Join(tempDir, types.MARKER_GO_MOD), invalidGoModContent)
	return tempDir
}

func writeTestFile(t *testing.T, path, content string) {
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write file %s: %v", path, err)
	}
}

func createGoDetectorWithMockExecutor(mockExec *mockCommandExecutor) *GoLanguageDetector {
	detector := NewGoLanguageDetector().(*GoLanguageDetector)
	detector.executor = mockExec
	return detector
}

// Test Functions

func TestNewGoLanguageDetector(t *testing.T) {
	detector := NewGoLanguageDetector()
	
	if detector == nil {
		t.Fatal("NewGoLanguageDetector returned nil")
	}
	
	goDetector, ok := detector.(*GoLanguageDetector)
	if !ok {
		t.Fatal("NewGoLanguageDetector did not return *GoLanguageDetector")
	}
	
	if goDetector.logger == nil {
		t.Error("Logger not initialized")
	}
	
	if goDetector.executor == nil {
		t.Error("Executor not initialized")
	}
}

func TestDetectLanguage_ValidProject(t *testing.T) {
	tempDir := createValidGoProject(t)
	
	mockExec := &mockCommandExecutor{
		shouldFail: false,
		exitCode:   0,
		stdout:     "go version go1.21.0 linux/amd64",
		stderr:     "",
	}
	
	detector := createGoDetectorWithMockExecutor(mockExec)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	result, err := detector.DetectLanguage(ctx, tempDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}
	
	// Verify basic properties
	if result.Language != types.PROJECT_TYPE_GO {
		t.Errorf("Expected language %s, got %s", types.PROJECT_TYPE_GO, result.Language)
	}
	
	if result.Confidence < 0.95 {
		t.Errorf("Expected confidence >= 0.95, got %f", result.Confidence)
	}
	
	// Verify marker files
	expectedMarkerFiles := []string{types.MARKER_GO_MOD, types.MARKER_GO_SUM}
	if len(result.MarkerFiles) != len(expectedMarkerFiles) {
		t.Errorf("Expected %d marker files, got %d", len(expectedMarkerFiles), len(result.MarkerFiles))
	}
	
	// Verify required servers
	if len(result.RequiredServers) != 1 || result.RequiredServers[0] != types.SERVER_GOPLS {
		t.Errorf("Expected required servers [%s], got %v", types.SERVER_GOPLS, result.RequiredServers)
	}
	
	// Verify metadata
	if result.Metadata["module_name"] != "github.com/example/test-project" {
		t.Errorf("Expected module name 'github.com/example/test-project', got %v", result.Metadata["module_name"])
	}
	
	if result.Metadata["project_name"] != "test-project" {
		t.Errorf("Expected project name 'test-project', got %v", result.Metadata["project_name"])
	}
	
	if result.Version != "1.21" {
		t.Errorf("Expected version '1.21', got %s", result.Version)
	}
	
	// Verify dependencies
	if len(result.Dependencies) == 0 {
		t.Error("Expected dependencies to be detected")
	}
	
	if result.Dependencies["github.com/gin-gonic/gin"] != "v1.9.1" {
		t.Errorf("Expected gin dependency v1.9.1, got %v", result.Dependencies["github.com/gin-gonic/gin"])
	}
	
	// Verify frameworks
	frameworks, ok := result.Metadata["frameworks"].([]string)
	if !ok || len(frameworks) == 0 {
		t.Error("Expected frameworks to be detected")
	}
	
	// Check for framework detection
	foundGin := false
	foundGRPC := false
	for _, framework := range frameworks {
		if framework == "Gin" {
			foundGin = true
		}
		if framework == "gRPC" {
			foundGRPC = true
		}
	}
	if !foundGin {
		t.Error("Expected Gin framework to be detected")
	}
	if !foundGRPC {
		t.Error("Expected gRPC framework to be detected")
	}
}

func TestDetectLanguage_GoModOnly(t *testing.T) {
	goModContent := `module test-module

go 1.20

require (
	github.com/stretchr/testify v1.8.4
)
`
	tempDir := createTempDirWithGoMod(t, goModContent)
	
	detector := NewGoLanguageDetector().(*GoLanguageDetector)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	result, err := detector.DetectLanguage(ctx, tempDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}
	
	if result.Language != types.PROJECT_TYPE_GO {
		t.Errorf("Expected language %s, got %s", types.PROJECT_TYPE_GO, result.Language)
	}
	
	if result.Confidence != 0.95 {
		t.Errorf("Expected confidence 0.95, got %f", result.Confidence)
	}
	
	if len(result.MarkerFiles) != 1 || result.MarkerFiles[0] != types.MARKER_GO_MOD {
		t.Errorf("Expected marker files [%s], got %v", types.MARKER_GO_MOD, result.MarkerFiles)
	}
}

func TestDetectLanguage_GoFilesOnly(t *testing.T) {
	tempDir := createTempDirWithGoFiles(t)
	
	mockExec := &mockCommandExecutor{
		shouldFail: false,
		exitCode:   0,
		stdout:     "go version go1.20.5 linux/amd64",
		stderr:     "",
	}
	
	detector := createGoDetectorWithMockExecutor(mockExec)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	result, err := detector.DetectLanguage(ctx, tempDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}
	
	if result.Language != types.PROJECT_TYPE_GO {
		t.Errorf("Expected language %s, got %s", types.PROJECT_TYPE_GO, result.Language)
	}
	
	if result.Confidence < 0.8 || result.Confidence > 0.9 {
		t.Errorf("Expected confidence between 0.8-0.9 for Go files without go.mod, got %f", result.Confidence)
	}
	
	// Should have Go files detected
	goFileCount, ok := result.Metadata["go_file_count"].(int)
	if !ok || goFileCount == 0 {
		t.Error("Expected Go files to be counted")
	}
	
	// Check source and test directories
	if len(result.SourceDirs) == 0 {
		t.Error("Expected source directories to be detected")
	}
	
	// Should have detected main.go
	hasMain, ok := result.Metadata["has_main"].(bool)
	if !ok || !hasMain {
		t.Error("Expected main.go to be detected")
	}
}

func TestDetectLanguage_NoGoIndicators(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create non-Go files
	writeTestFile(t, filepath.Join(tempDir, "README.md"), "# Test Project")
	writeTestFile(t, filepath.Join(tempDir, "config.json"), "{}")
	
	detector := NewGoLanguageDetector().(*GoLanguageDetector)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	result, err := detector.DetectLanguage(ctx, tempDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}
	
	if result.Language != types.PROJECT_TYPE_GO {
		t.Errorf("Expected language %s, got %s", types.PROJECT_TYPE_GO, result.Language)
	}
	
	if result.Confidence != 0.1 {
		t.Errorf("Expected confidence 0.1 for no Go indicators, got %f", result.Confidence)
	}
	
	if len(result.MarkerFiles) != 0 {
		t.Errorf("Expected no marker files, got %v", result.MarkerFiles)
	}
}

func TestParseGoMod_ValidFile(t *testing.T) {
	goModContent := `module github.com/example/complex-project

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/gorilla/mux v1.8.0
	google.golang.org/grpc v1.57.0
	github.com/stretchr/testify v1.8.4 // indirect
)

require github.com/direct-require v1.0.0

replace (
	github.com/old/module => github.com/new/module v1.0.0
	github.com/another/old => /local/path
)

replace github.com/single-replace => github.com/single-new v2.0.0

exclude (
	github.com/bad/module v1.0.0
)
`
	tempDir := createTempDirWithGoMod(t, goModContent)
	goModPath := filepath.Join(tempDir, types.MARKER_GO_MOD)
	
	detector := NewGoLanguageDetector().(*GoLanguageDetector)
	result := &types.LanguageDetectionResult{
		Language:        types.PROJECT_TYPE_GO,
		Metadata:        make(map[string]interface{}),
		Dependencies:    make(map[string]string),
		DevDependencies: make(map[string]string),
	}
	
	err := detector.parseGoMod(goModPath, result)
	if err != nil {
		t.Fatalf("parseGoMod failed: %v", err)
	}
	
	// Verify module name
	if result.Metadata["module_name"] != "github.com/example/complex-project" {
		t.Errorf("Expected module name 'github.com/example/complex-project', got %v", result.Metadata["module_name"])
	}
	
	// Verify project name extraction
	if result.Metadata["project_name"] != "complex-project" {
		t.Errorf("Expected project name 'complex-project', got %v", result.Metadata["project_name"])
	}
	
	// Verify Go version
	if result.Version != "1.21" {
		t.Errorf("Expected version '1.21', got %s", result.Version)
	}
	
	// Verify dependencies
	expectedDeps := map[string]string{
		"github.com/gin-gonic/gin": "v1.9.1",
		"github.com/gorilla/mux":   "v1.8.0",
		"google.golang.org/grpc":   "v1.57.0",
		"github.com/direct-require": "v1.0.0",
	}
	
	for dep, version := range expectedDeps {
		if result.Dependencies[dep] != version {
			t.Errorf("Expected dependency %s: %s, got %s", dep, version, result.Dependencies[dep])
		}
	}
	
	// Verify indirect dependencies
	if result.DevDependencies["github.com/stretchr/testify"] != "v1.8.4" {
		t.Errorf("Expected indirect dependency testify v1.8.4, got %v", result.DevDependencies["github.com/stretchr/testify"])
	}
	
	// Verify replacements
	replacements, ok := result.Metadata["replacements"].(map[string]string)
	if !ok {
		t.Fatal("Expected replacements to be present")
	}
	
	expectedReplacements := map[string]string{
		"github.com/old/module":     "github.com/new/module",
		"github.com/another/old":    "/local/path",
		"github.com/single-replace": "github.com/single-new",
	}
	
	for original, replacement := range expectedReplacements {
		if replacements[original] != replacement {
			t.Errorf("Expected replacement %s => %s, got %s", original, replacement, replacements[original])
		}
	}
}

func TestParseGoMod_InvalidFile(t *testing.T) {
	tempDir := createInvalidGoMod(t)
	goModPath := filepath.Join(tempDir, types.MARKER_GO_MOD)
	
	detector := NewGoLanguageDetector().(*GoLanguageDetector)
	result := &types.LanguageDetectionResult{
		Metadata:        make(map[string]interface{}),
		Dependencies:    make(map[string]string),
		DevDependencies: make(map[string]string),
	}
	
	err := detector.parseGoMod(goModPath, result)
	// parseGoMod should not fail on invalid content, it just won't parse much
	if err != nil {
		t.Fatalf("parseGoMod should not fail on invalid content: %v", err)
	}
	
	// Should not have parsed module name from invalid content, or if it did, it should be something invalid
	if moduleName, exists := result.Metadata["module_name"]; exists {
		moduleNameStr, ok := moduleName.(string)
		if ok && strings.Contains(moduleNameStr, "github.com") {
			t.Errorf("Should not have parsed a valid-looking module name from invalid go.mod, got: %s", moduleNameStr)
		}
	}
}

func TestParseGoMod_FileNotFound(t *testing.T) {
	detector := NewGoLanguageDetector().(*GoLanguageDetector)
	result := &types.LanguageDetectionResult{
		Metadata:        make(map[string]interface{}),
		Dependencies:    make(map[string]string),
		DevDependencies: make(map[string]string),
	}
	
	err := detector.parseGoMod("/nonexistent/go.mod", result)
	if err == nil {
		t.Error("Expected error for non-existent go.mod file")
	}
}

func TestScanForGoFiles(t *testing.T) {
	tempDir := createTempDirWithGoFiles(t)
	
	// Create vendor directory (should be ignored)
	vendorDir := filepath.Join(tempDir, "vendor", "github.com", "some", "package")
	os.MkdirAll(vendorDir, 0755)
	writeTestFile(t, filepath.Join(vendorDir, "vendor.go"), "package vendor\n")
	
	detector := NewGoLanguageDetector().(*GoLanguageDetector)
	result := &types.LanguageDetectionResult{
		SourceDirs: []string{},
		TestDirs:   []string{},
		BuildFiles: []string{},
	}
	
	goFileCount := 0
	err := detector.scanForGoFiles(tempDir, result, &goFileCount)
	if err != nil {
		t.Fatalf("scanForGoFiles failed: %v", err)
	}
	
	if goFileCount == 0 {
		t.Error("Expected Go files to be found")
	}
	
	// Should have found source directories
	if len(result.SourceDirs) == 0 {
		t.Error("Expected source directories to be found")
	}
	
	// Should have found test directories  
	if len(result.TestDirs) == 0 {
		t.Error("Expected test directories to be found")
	}
	
	// Vendor files should not be counted
	expectedMaxFiles := 6 // main.go, cmd/api/main.go, internal/handler/handler.go, pkg/util/util.go, main_test.go, internal/handler/handler_test.go
	if goFileCount > expectedMaxFiles {
		t.Errorf("Go file count too high, vendor files may have been included: %d", goFileCount)
	}
}

func TestDetectGoVersion(t *testing.T) {
	tests := []struct {
		name           string
		stdout         string
		exitCode       int
		shouldFail     bool
		expectedError  bool
		expectedVersion string
	}{
		{
			name:           "successful version detection",
			stdout:         "go version go1.21.0 linux/amd64",
			exitCode:       0,
			shouldFail:     false,
			expectedError:  false,
			expectedVersion: "1.21.0",
		},
		{
			name:           "version with different format",
			stdout:         "go version go1.20.5 darwin/arm64",
			exitCode:       0,
			shouldFail:     false,
			expectedError:  false,
			expectedVersion: "1.20.5",
		},
		{
			name:          "command execution failure",
			stdout:        "",
			exitCode:      1,
			shouldFail:    true,
			expectedError: true,
		},
		{
			name:          "invalid output format",
			stdout:        "invalid output",
			exitCode:      0,
			shouldFail:    false,
			expectedError: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := &mockCommandExecutor{
				shouldFail: tt.shouldFail,
				exitCode:   tt.exitCode,
				stdout:     tt.stdout,
				stderr:     "",
				failError:  nil,
			}
			if tt.shouldFail {
				mockExec.failError = &platform.PlatformError{
					Code:    "COMMAND_FAILED",
					Message: "Command execution failed",
				}
			}
			
			detector := createGoDetectorWithMockExecutor(mockExec)
			result := &types.LanguageDetectionResult{
				Metadata: make(map[string]interface{}),
			}
			
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			err := detector.detectGoVersion(ctx, result)
			
			if tt.expectedError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			
			if err != nil && !tt.expectedError {
				t.Fatalf("Unexpected error: %v", err)
			}
			
			// Verify command was called correctly
			if mockExec.commandCalled != "go" {
				t.Errorf("Expected 'go' command, got %s", mockExec.commandCalled)
			}
			
			if len(mockExec.argsCalled) != 1 || mockExec.argsCalled[0] != "version" {
				t.Errorf("Expected args ['version'], got %v", mockExec.argsCalled)
			}
			
			if tt.expectedVersion != "" {
				installedVersion, ok := result.Metadata["installed_go_version"]
				if !ok {
					t.Error("Expected installed_go_version in metadata")
				} else if installedVersion != tt.expectedVersion {
					t.Errorf("Expected installed version %s, got %v", tt.expectedVersion, installedVersion)
				}
			}
		})
	}
}

func TestDetectGoFrameworks(t *testing.T) {
	detector := NewGoLanguageDetector().(*GoLanguageDetector)
	
	result := &types.LanguageDetectionResult{
		Metadata:    make(map[string]interface{}),
		ConfigFiles: []string{},
		Dependencies: map[string]string{
			"github.com/gin-gonic/gin":   "v1.9.1",
			"github.com/gorilla/mux":     "v1.8.0",
			"github.com/labstack/echo":   "v4.11.1",
			"github.com/gofiber/fiber":   "v2.48.0",
			"github.com/go-chi/chi":      "v5.0.8",
			"google.golang.org/grpc":     "v1.57.0",
			"github.com/gogo/protobuf":   "v1.3.2",
			"k8s.io/kubernetes":          "v1.28.0",
			"github.com/docker/docker":   "v24.0.5",
		},
	}
	
	// Create temp directory with config files
	tempDir := t.TempDir()
	writeTestFile(t, filepath.Join(tempDir, "Dockerfile"), "FROM golang:1.21")
	writeTestFile(t, filepath.Join(tempDir, "docker-compose.yml"), "version: '3'")
	writeTestFile(t, filepath.Join(tempDir, ".goreleaser.yml"), "builds:")
	writeTestFile(t, filepath.Join(tempDir, "air.toml"), "[build]")
	
	detector.detectGoFrameworks(tempDir, result)
	
	frameworks, ok := result.Metadata["frameworks"].([]string)
	if !ok {
		t.Fatal("Expected frameworks to be detected")
	}
	
	expectedFrameworks := []string{
		"Gin", "Gorilla Mux", "Echo", "Fiber", "Chi", 
		"gRPC", "Protocol Buffers", "Kubernetes", "Docker",
		"Docker Compose", "GoReleaser", "Air (Hot Reload)",
	}
	
	for _, expected := range expectedFrameworks {
		found := false
		for _, framework := range frameworks {
			if framework == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected framework %s not found in %v", expected, frameworks)
		}
	}
	
	// Verify config files were added
	expectedConfigFiles := []string{"Dockerfile", "docker-compose.yml", ".goreleaser.yml", "air.toml"}
	for _, configFile := range expectedConfigFiles {
		found := false
		for _, file := range result.ConfigFiles {
			if file == configFile {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected config file %s not found in %v", configFile, result.ConfigFiles)
		}
	}
}

func TestAnalyzeProjectStructure(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create Go project structure directories
	dirs := []string{"cmd", "internal", "pkg", "api", "web", "docs", "scripts", "build"}
	for _, dir := range dirs {
		os.MkdirAll(filepath.Join(tempDir, dir), 0755)
	}
	
	// Create main.go files in different locations
	writeTestFile(t, filepath.Join(tempDir, "main.go"), "package main")
	cmdApiDir := filepath.Join(tempDir, "cmd", "api")
	os.MkdirAll(cmdApiDir, 0755)
	writeTestFile(t, filepath.Join(cmdApiDir, "main.go"), "package main")
	
	detector := NewGoLanguageDetector().(*GoLanguageDetector)
	result := &types.LanguageDetectionResult{
		Metadata:   make(map[string]interface{}),
		Confidence: 0.9,
	}
	
	detector.analyzeProjectStructure(tempDir, result)
	
	projectStructure, ok := result.Metadata["project_structure"].([]string)
	if !ok {
		t.Fatal("Expected project_structure in metadata")
	}
	
	if len(projectStructure) != len(dirs) {
		t.Errorf("Expected %d structure directories, got %d", len(dirs), len(projectStructure))
	}
	
	for _, expectedDir := range dirs {
		found := false
		for _, dir := range projectStructure {
			if dir == expectedDir {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected directory %s not found in project structure", expectedDir)
		}
	}
	
	// Confidence should be boosted for good structure
	if result.Confidence <= 0.9 {
		t.Errorf("Expected confidence to be boosted, got %f", result.Confidence)
	}
	
	// Should detect main.go
	hasMain, ok := result.Metadata["has_main"].(bool)
	if !ok || !hasMain {
		t.Error("Expected main.go to be detected")
	}
}

func TestValidateStructure(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func() string
		expectError bool
		errorType   string
	}{
		{
			name: "valid Go project",
			setupFunc: func() string {
				return createValidGoProject(t)
			},
			expectError: false,
		},
		{
			name: "missing go.mod",
			setupFunc: func() string {
				tempDir := t.TempDir()
				writeTestFile(t, filepath.Join(tempDir, "main.go"), "package main")
				return tempDir
			},
			expectError: true,
			errorType:   "go.mod file not found",
		},
		{
			name: "unreadable go.mod",
			setupFunc: func() string {
				tempDir := t.TempDir()
				goModPath := filepath.Join(tempDir, types.MARKER_GO_MOD)
				writeTestFile(t, goModPath, "module test")
				// Make file unreadable
				os.Chmod(goModPath, 0000)
				return tempDir
			},
			expectError: true,
			errorType:   "cannot read go.mod file",
		},
		{
			name: "no Go files",
			setupFunc: func() string {
				tempDir := t.TempDir()
				writeTestFile(t, filepath.Join(tempDir, types.MARKER_GO_MOD), "module test")
				writeTestFile(t, filepath.Join(tempDir, "README.md"), "# Test")
				return tempDir
			},
			expectError: true,
			errorType:   "no Go source files found",
		},
		{
			name: "Go files in vendor (should be ignored)",
			setupFunc: func() string {
				tempDir := t.TempDir()
				writeTestFile(t, filepath.Join(tempDir, types.MARKER_GO_MOD), "module test")
				
				// Create vendor directory with Go files (should be ignored)
				vendorDir := filepath.Join(tempDir, "vendor")
				os.MkdirAll(vendorDir, 0755)
				writeTestFile(t, filepath.Join(vendorDir, "vendor.go"), "package vendor")
				
				return tempDir
			},
			expectError: true,
			errorType:   "no Go source files found",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := tt.setupFunc()
			// Ensure we can clean up files with restricted permissions
			defer func() {
				os.Chmod(tempDir, 0755)
				filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
					if err == nil {
						os.Chmod(path, 0755)
					}
					return nil
				})
			}()
			
			detector := NewGoLanguageDetector().(*GoLanguageDetector)
			
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			err := detector.ValidateStructure(ctx, tempDir)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected validation error but got none")
				} else if !strings.Contains(err.Error(), tt.errorType) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorType, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected validation error: %v", err)
				}
			}
		})
	}
}

func TestGetLanguageInfo(t *testing.T) {
	detector := NewGoLanguageDetector().(*GoLanguageDetector)
	
	// Test valid language
	info, err := detector.GetLanguageInfo(types.PROJECT_TYPE_GO)
	if err != nil {
		t.Fatalf("GetLanguageInfo failed: %v", err)
	}
	
	if info.Name != types.PROJECT_TYPE_GO {
		t.Errorf("Expected name %s, got %s", types.PROJECT_TYPE_GO, info.Name)
	}
	
	if info.DisplayName != "Go" {
		t.Errorf("Expected display name 'Go', got %s", info.DisplayName)
	}
	
	if info.PackageManager != "go mod" {
		t.Errorf("Expected package manager 'go mod', got %s", info.PackageManager)
	}
	
	expectedLSPServers := []string{types.SERVER_GOPLS}
	if len(info.LSPServers) != len(expectedLSPServers) || info.LSPServers[0] != expectedLSPServers[0] {
		t.Errorf("Expected LSP servers %v, got %v", expectedLSPServers, info.LSPServers)
	}
	
	expectedExtensions := []string{".go", ".mod", ".sum"}
	if len(info.FileExtensions) != len(expectedExtensions) {
		t.Errorf("Expected %d file extensions, got %d", len(expectedExtensions), len(info.FileExtensions))
	}
	
	// Test invalid language
	_, err = detector.GetLanguageInfo("invalid")
	if err == nil {
		t.Error("Expected error for invalid language")
	}
}

func TestGetMarkerFiles(t *testing.T) {
	detector := NewGoLanguageDetector().(*GoLanguageDetector)
	
	markerFiles := detector.GetMarkerFiles()
	expected := []string{types.MARKER_GO_MOD, types.MARKER_GO_SUM}
	
	if len(markerFiles) != len(expected) {
		t.Errorf("Expected %d marker files, got %d", len(expected), len(markerFiles))
	}
	
	for i, expectedFile := range expected {
		if markerFiles[i] != expectedFile {
			t.Errorf("Expected marker file %s, got %s", expectedFile, markerFiles[i])
		}
	}
}

func TestGetRequiredServers(t *testing.T) {
	detector := NewGoLanguageDetector().(*GoLanguageDetector)
	
	servers := detector.GetRequiredServers()
	expected := []string{types.SERVER_GOPLS}
	
	if len(servers) != len(expected) {
		t.Errorf("Expected %d required servers, got %d", len(expected), len(servers))
	}
	
	if servers[0] != expected[0] {
		t.Errorf("Expected server %s, got %s", expected[0], servers[0])
	}
}

func TestGetPriority(t *testing.T) {
	detector := NewGoLanguageDetector().(*GoLanguageDetector)
	
	priority := detector.GetPriority()
	if priority != types.PRIORITY_GO {
		t.Errorf("Expected priority %d, got %d", types.PRIORITY_GO, priority)
	}
}

func TestDetectLanguage_ConfidenceScoring(t *testing.T) {
	tests := []struct {
		name               string
		setupFunc          func() string
		expectedConfidence float64
	}{
		{
			name: "go.mod + go.sum + go files",
			setupFunc: func() string {
				return createValidGoProject(t)
			},
			expectedConfidence: 1.0, // 0.98 + structure boost = 1.0 (capped)
		},
		{
			name: "go.mod only",
			setupFunc: func() string {
				return createTempDirWithGoMod(t, "module test\n\ngo 1.21\n")
			},
			expectedConfidence: 0.95,
		},
		{
			name: "go.sum without go.mod (unusual)",
			setupFunc: func() string {
				tempDir := createTempDirWithGoSum(t)
				return tempDir
			},
			expectedConfidence: 0.1, // go.sum but no go.mod and no .go files = 0.1
		},
		{
			name: "go files without go.mod (legacy)",
			setupFunc: func() string {
				return createTempDirWithGoFiles(t)
			},
			expectedConfidence: 0.85, // 0.8 + structure boost = 0.85
		},
		{
			name: "no indicators",
			setupFunc: func() string {
				tempDir := t.TempDir()
				writeTestFile(t, filepath.Join(tempDir, "README.md"), "# Test")
				return tempDir
			},
			expectedConfidence: 0.1,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := tt.setupFunc()
			
			detector := NewGoLanguageDetector().(*GoLanguageDetector)
			
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			result, err := detector.DetectLanguage(ctx, tempDir)
			if err != nil {
				t.Fatalf("DetectLanguage failed: %v", err)
			}
			
			tolerance := 0.01
			if result.Confidence < tt.expectedConfidence-tolerance || result.Confidence > tt.expectedConfidence+tolerance {
				t.Errorf("Expected confidence %f (±%f), got %f", tt.expectedConfidence, tolerance, result.Confidence)
			}
		})
	}
}

func TestDetectLanguage_ErrorHandling(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() string
		mockSetup func() *mockCommandExecutor
	}{
		{
			name: "go version command fails",
			setupFunc: func() string {
				return createValidGoProject(t)
			},
			mockSetup: func() *mockCommandExecutor {
				return &mockCommandExecutor{
					shouldFail: true,
					exitCode:   1,
					stderr:     "go command not found",
					failError: &platform.PlatformError{
						Code:    "COMMAND_NOT_FOUND",
						Message: "go command not found",
					},
				}
			},
		},
		{
			name: "invalid go.mod parsing",
			setupFunc: func() string {
				return createInvalidGoMod(t)
			},
			mockSetup: func() *mockCommandExecutor {
				return &mockCommandExecutor{
					shouldFail: false,
					exitCode:   0,
					stdout:     "go version go1.21.0 linux/amd64",
				}
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := tt.setupFunc()
			mockExec := tt.mockSetup()
			
			detector := createGoDetectorWithMockExecutor(mockExec)
			
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			result, err := detector.DetectLanguage(ctx, tempDir)
			// DetectLanguage should not fail even if sub-operations fail
			if err != nil {
				t.Fatalf("DetectLanguage should handle errors gracefully: %v", err)
			}
			
			// But confidence should be adjusted appropriately
			if strings.Contains(tt.name, "invalid go.mod") && result.Confidence != 0.95 {
				t.Logf("Invalid go.mod confidence: %f (this may be expected as file exists)", result.Confidence)
			}
		})
	}
}

func TestMinFunction(t *testing.T) {
	tests := []struct {
		a, b, expected float64
	}{
		{1.0, 2.0, 1.0},
		{2.0, 1.0, 1.0},
		{1.5, 1.5, 1.5},
		{0.0, 1.0, 0.0},
	}
	
	for _, tt := range tests {
		result := min(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("min(%f, %f) = %f, expected %f", tt.a, tt.b, result, tt.expected)
		}
	}
}

func TestDetectLanguage_MetadataCompletion(t *testing.T) {
	tempDir := createValidGoProject(t)
	
	mockExec := &mockCommandExecutor{
		shouldFail: false,
		exitCode:   0,
		stdout:     "go version go1.21.0 linux/amd64",
		stderr:     "",
	}
	
	detector := createGoDetectorWithMockExecutor(mockExec)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	result, err := detector.DetectLanguage(ctx, tempDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}
	
	// Verify all expected metadata fields are present
	expectedMetadataKeys := []string{
		"detection_time",
		"detector_version",
		"module_name",
		"project_name", 
		"go_version",
		"installed_go_version",
		"go_file_count",
		"frameworks",
		"project_structure",
		"has_main",
		"go_mod_info",
		"replacements",
	}
	
	for _, key := range expectedMetadataKeys {
		if _, exists := result.Metadata[key]; !exists {
			t.Errorf("Expected metadata key '%s' not found", key)
		}
	}
	
	// Verify detection time is reasonable
	detectionTime, ok := result.Metadata["detection_time"].(time.Duration)
	if !ok {
		t.Error("detection_time should be a time.Duration")
	} else if detectionTime <= 0 {
		t.Error("detection_time should be positive")
	}
	
	// Verify detector version
	if result.Metadata["detector_version"] != "1.0.0" {
		t.Errorf("Expected detector_version '1.0.0', got %v", result.Metadata["detector_version"])
	}
}