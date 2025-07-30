package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-gateway/internal/project/types"
)

func TestNewWorkspaceDetector(t *testing.T) {
	t.Parallel()
	detector := NewWorkspaceDetector()
	if detector == nil {
		t.Fatal("NewWorkspaceDetector returned nil")
	}

	// Test that it implements the interface
	_, ok := detector.(WorkspaceDetector)
	if !ok {
		t.Fatal("NewWorkspaceDetector does not implement WorkspaceDetector interface")
	}
}

func TestDetectWorkspace(t *testing.T) {
	t.Parallel()
	detector := NewWorkspaceDetector()
	
	workspace, err := detector.DetectWorkspace()
	if err != nil {
		t.Fatalf("DetectWorkspace failed: %v", err)
	}

	if workspace == nil {
		t.Fatal("DetectWorkspace returned nil workspace")
	}

	// Verify basic fields are set
	if workspace.Root == "" {
		t.Error("Workspace root is empty")
	}

	if workspace.ID == "" {
		t.Error("Workspace ID is empty")
	}

	if workspace.Hash == "" {
		t.Error("Workspace hash is empty")
	}

	if workspace.CreatedAt.IsZero() {
		t.Error("Workspace CreatedAt is zero")
	}

	// Should have at least one language (even if unknown)
	if len(workspace.Languages) == 0 {
		t.Error("Workspace languages is empty")
	}
}

func TestDetectWorkspaceAt(t *testing.T) {
	t.Parallel()
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "workspace_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	detector := NewWorkspaceDetector()

	// Test with empty directory (should detect as unknown)
	workspace, err := detector.DetectWorkspaceAt(tempDir)
	if err != nil {
		t.Fatalf("DetectWorkspaceAt failed: %v", err)
	}

	if workspace.ProjectType != types.PROJECT_TYPE_UNKNOWN {
		t.Errorf("Expected unknown project type, got %s", workspace.ProjectType)
	}

	// Test with Go project marker
	goModPath := filepath.Join(tempDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module test"), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	workspace, err = detector.DetectWorkspaceAt(tempDir)
	if err != nil {
		t.Fatalf("DetectWorkspaceAt failed for Go project: %v", err)
	}

	if workspace.ProjectType != types.PROJECT_TYPE_GO {
		t.Errorf("Expected Go project type, got %s", workspace.ProjectType)
	}

	if !contains(workspace.Languages, types.PROJECT_TYPE_GO) {
		t.Error("Go language not detected in languages list")
	}
}

func TestGenerateWorkspaceID(t *testing.T) {
	t.Parallel()
	detector := NewWorkspaceDetector()
	
	// Test ID generation consistency
	path := "/test/workspace"
	id1 := detector.GenerateWorkspaceID(path)
	id2 := detector.GenerateWorkspaceID(path)

	if id1 != id2 {
		t.Error("GenerateWorkspaceID should generate consistent IDs for same path")
	}

	// Test different paths generate different IDs
	id3 := detector.GenerateWorkspaceID("/different/path")
	if id1 == id3 {
		t.Error("Different paths should generate different IDs")
	}

	// Test ID format
	if len(id1) == 0 {
		t.Error("Generated ID is empty")
	}

	if !startsWith(id1, "workspace_") {
		t.Error("Generated ID should start with 'workspace_'")
	}
}

func TestValidateWorkspace(t *testing.T) {
	t.Parallel()
	detector := NewWorkspaceDetector()

	// Test empty path
	err := detector.ValidateWorkspace("")
	if err == nil {
		t.Error("ValidateWorkspace should fail for empty path")
	}

	// Test non-existent path
	err = detector.ValidateWorkspace("/non/existent/path")
	if err == nil {
		t.Error("ValidateWorkspace should fail for non-existent path")
	}

	// Test with valid directory
	tempDir, err := os.MkdirTemp("", "workspace_validate_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	err = detector.ValidateWorkspace(tempDir)
	if err != nil {
		t.Errorf("ValidateWorkspace should succeed for valid directory: %v", err)
	}

	// Test with file instead of directory
	tempFile := filepath.Join(tempDir, "testfile")
	if err := os.WriteFile(tempFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	err = detector.ValidateWorkspace(tempFile)
	if err == nil {
		t.Error("ValidateWorkspace should fail for file path")
	}
}

func TestMultiLanguageDetection(t *testing.T) {
	t.Parallel()
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "multi_lang_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create multiple language markers
	goModPath := filepath.Join(tempDir, "go.mod")
	packageJsonPath := filepath.Join(tempDir, "package.json")
	
	if err := os.WriteFile(goModPath, []byte("module test"), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}
	
	if err := os.WriteFile(packageJsonPath, []byte(`{"name": "test"}`), 0644); err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	detector := NewWorkspaceDetector()
	workspace, err := detector.DetectWorkspaceAt(tempDir)
	if err != nil {
		t.Fatalf("DetectWorkspaceAt failed: %v", err)
	}

	// Should detect as mixed project
	if workspace.ProjectType != types.PROJECT_TYPE_MIXED {
		t.Errorf("Expected mixed project type, got %s", workspace.ProjectType)
	}

	// Should have both languages
	if len(workspace.Languages) < 2 {
		t.Errorf("Expected at least 2 languages, got %d", len(workspace.Languages))
	}

	if !contains(workspace.Languages, types.PROJECT_TYPE_GO) {
		t.Error("Go language not detected")
	}

	if !contains(workspace.Languages, types.PROJECT_TYPE_NODEJS) {
		t.Error("Node.js language not detected")
	}
}


func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// ====== SUB-PROJECT DETECTION COMPREHENSIVE TESTS ======

// pathTestCase represents a test case for path-to-project mapping
type pathTestCase struct {
	filePath        string
	expectedProject string // relative path of expected project
	description     string
}

// TestDetectSubProjects tests basic sub-project detection functionality
func TestDetectSubProjects(t *testing.T) {
	t.Parallel()
	// Create test workspace structure
	workspace, cleanup := createTestWorkspace(t, map[string]interface{}{
		"go.mod":                  "module root",
		"main.go":                 "package main",
		"frontend/package.json":   `{"name": "frontend"}`,
		"backend/requirements.txt": "flask==2.0.0",
		"services/auth/go.mod":    "module auth",
	})
	defer cleanup()

	detector := NewWorkspaceDetector()
	ctx := context.Background()

	workspaceCtx, err := detector.DetectWorkspaceWithContext(ctx, workspace)
	if err != nil {
		t.Fatalf("DetectWorkspaceWithContext failed: %v", err)
	}

	// Should detect 4 sub-projects
	expectedProjects := 4
	if len(workspaceCtx.SubProjects) != expectedProjects {
		t.Errorf("Expected %d sub-projects, got %d", expectedProjects, len(workspaceCtx.SubProjects))
	}

	// Verify specific sub-projects exist
	projectMap := make(map[string]*DetectedSubProject)
	for _, sp := range workspaceCtx.SubProjects {
		projectMap[sp.RelativePath] = sp
	}

	expectedPaths := []string{".", "frontend", "backend", "services/auth"}
	for _, expectedPath := range expectedPaths {
		if _, exists := projectMap[expectedPath]; !exists {
			t.Errorf("Expected sub-project at '%s' not found", expectedPath)
		}
	}

	// Verify root project is Go
	rootProject := projectMap["."]
	if rootProject == nil {
		t.Fatal("Root project not found")
	}
	if rootProject.ProjectType != types.PROJECT_TYPE_GO {
		t.Errorf("Expected root project type %s, got %s", types.PROJECT_TYPE_GO, rootProject.ProjectType)
	}

	// Verify frontend is Node.js
	frontendProject := projectMap["frontend"]
	if frontendProject == nil {
		t.Fatal("Frontend project not found")
	}
	if frontendProject.ProjectType != types.PROJECT_TYPE_NODEJS {
		t.Errorf("Expected frontend project type %s, got %s", types.PROJECT_TYPE_NODEJS, frontendProject.ProjectType)
	}

	// Verify backend is Python
	backendProject := projectMap["backend"]
	if backendProject == nil {
		t.Fatal("Backend project not found")
	}
	if backendProject.ProjectType != types.PROJECT_TYPE_PYTHON {
		t.Errorf("Expected backend project type %s, got %s", types.PROJECT_TYPE_PYTHON, backendProject.ProjectType)
	}
}

// TestNestedProjectDetection tests handling of nested projects
func TestNestedProjectDetection(t *testing.T) {
	t.Parallel()
	workspace, cleanup := createTestWorkspace(t, map[string]interface{}{
		"go.mod":                         "module root",
		"services/go.mod":                "module services",
		"services/auth/go.mod":           "module auth",
		"services/auth/handlers/go.mod":  "module handlers",
		"services/api/package.json":      `{"name": "api"}`,
	})
	defer cleanup()

	detector := NewWorkspaceDetector()
	workspaceCtx, err := detector.DetectWorkspaceAt(workspace)
	if err != nil {
		t.Fatalf("DetectWorkspaceAt failed: %v", err)
	}

	// Should detect 5 nested projects
	expectedProjects := 5
	if len(workspaceCtx.SubProjects) != expectedProjects {
		t.Errorf("Expected %d sub-projects, got %d", expectedProjects, len(workspaceCtx.SubProjects))
	}

	// Verify deepest project precedence in path mapping
	deepestPath := filepath.Join(workspace, "services", "auth", "handlers", "main.go")
	foundProject := detector.FindSubProjectForPath(workspaceCtx, deepestPath)
	if foundProject == nil {
		t.Fatal("No project found for deepest path")
	}
	if foundProject.RelativePath != "services/auth/handlers" {
		t.Errorf("Expected deepest project 'services/auth/handlers', got '%s'", foundProject.RelativePath)
	}
}

// TestMaxDepthLimit tests depth limit enforcement
func TestMaxDepthLimit(t *testing.T) {
	t.Parallel()
	workspace, cleanup := createNestedProjects(t, 7) // Create projects beyond max depth
	defer cleanup()

	detector := NewWorkspaceDetector()
	workspaceCtx, err := detector.DetectWorkspaceAt(workspace)
	if err != nil {
		t.Fatalf("DetectWorkspaceAt failed: %v", err)
	}

	// Should not detect projects beyond depth 5
	maxDepthFound := 0
	for _, project := range workspaceCtx.SubProjects {
		depth := strings.Count(project.RelativePath, string(filepath.Separator))
		if depth > maxDepthFound {
			maxDepthFound = depth
		}
	}

	if maxDepthFound > 5 {
		t.Errorf("Found project at depth %d, exceeds max depth of 5", maxDepthFound)
	}
}

// TestIgnoreDirectories tests that common directories are ignored
func TestIgnoreDirectories(t *testing.T) {
	t.Parallel()
	workspace, cleanup := createTestWorkspace(t, map[string]interface{}{
		"go.mod":                            "module root",
		"node_modules/some-package/go.mod":  "module ignored1",
		".git/hooks/go.mod":                 "module ignored2",
		"vendor/github.com/pkg/go.mod":      "module ignored3",
		"target/classes/go.mod":             "module ignored4",
		"build/output/go.mod":               "module ignored5",
		"dist/bundle/go.mod":                "module ignored6",
		"venv/lib/python/go.mod":            "module ignored7",
		"env/bin/go.mod":                    "module ignored8",
		"src/main/go.mod":                   "module valid", // Should be detected
	})
	defer cleanup()

	detector := NewWorkspaceDetector()
	workspaceCtx, err := detector.DetectWorkspaceAt(workspace)
	if err != nil {
		t.Fatalf("DetectWorkspaceAt failed: %v", err)
	}

	// Should only detect root and src/main projects
	expectedProjects := 2
	if len(workspaceCtx.SubProjects) != expectedProjects {
		t.Errorf("Expected %d sub-projects, got %d", expectedProjects, len(workspaceCtx.SubProjects))
		for _, sp := range workspaceCtx.SubProjects {
			t.Logf("Detected project: %s", sp.RelativePath)
		}
	}

	// Verify no ignored directories were detected
	ignoredDirs := []string{"node_modules", ".git", "vendor", "target", "build", "dist", "venv", "env"}
	for _, project := range workspaceCtx.SubProjects {
		for _, ignoredDir := range ignoredDirs {
			if strings.Contains(project.RelativePath, ignoredDir) {
				t.Errorf("Ignored directory '%s' was detected in project: %s", ignoredDir, project.RelativePath)
			}
		}
	}
}

// TestProjectPathMapping tests file path to project mapping
func TestProjectPathMapping(t *testing.T) {
	t.Parallel()
	workspace, cleanup := createTestWorkspace(t, map[string]interface{}{
		"go.mod":                  "module root",
		"frontend/package.json":   `{"name": "frontend"}`,
		"backend/requirements.txt": "flask==2.0.0",
		"services/auth/go.mod":    "module auth",
	})
	defer cleanup()

	detector := NewWorkspaceDetector()
	workspaceCtx, err := detector.DetectWorkspaceAt(workspace)
	if err != nil {
		t.Fatalf("DetectWorkspaceAt failed: %v", err)
	}

	testCases := []pathTestCase{
		{
			filePath:        filepath.Join(workspace, "main.go"),
			expectedProject: ".",
			description:     "Root file should map to root project",
		},
		{
			filePath:        filepath.Join(workspace, "frontend", "src", "index.js"),
			expectedProject: "frontend",
			description:     "Frontend file should map to frontend project",
		},
		{
			filePath:        filepath.Join(workspace, "backend", "app.py"),
			expectedProject: "backend",
			description:     "Backend file should map to backend project",
		},
		{
			filePath:        filepath.Join(workspace, "services", "auth", "handlers.go"),
			expectedProject: "services/auth",
			description:     "Auth service file should map to auth project",
		},
		{
			filePath:        filepath.Join(workspace, "docs", "README.md"),
			expectedProject: ".",
			description:     "Unmapped path should fallback to root project",
		},
	}

	assertProjectMapping(t, detector, workspaceCtx, testCases)
}

// TestMultiLanguageWorkspace tests complex multi-project workspace
func TestMultiLanguageWorkspace(t *testing.T) {
	t.Parallel()
	workspace, cleanup := createTestWorkspace(t, map[string]interface{}{
		"go.mod":                     "module root",
		"README.md":                  "# Root Project",
		"frontend/package.json":      `{"name": "frontend"}`,
		"frontend/tsconfig.json":     `{"compilerOptions": {}}`,
		"frontend/src/app.tsx":       "// React app",
		"backend/requirements.txt":   "flask==2.0.0",
		"backend/setup.py":           "from setuptools import setup",
		"backend/app.py":             "# Flask app",
		"services/auth/go.mod":       "module auth",
		"services/auth/main.go":      "package main",
		"services/api/pom.xml":       `<project><groupId>com.test</groupId></project>`,
		"services/api/src/Main.java": "public class Main {}",
		"scripts/setup.py":           "# Setup scripts",
		"scripts/deploy.py":          "# Deploy scripts",
	})
	defer cleanup()

	detector := NewWorkspaceDetector()
	workspaceCtx, err := detector.DetectWorkspaceAt(workspace)
	if err != nil {
		t.Fatalf("DetectWorkspaceAt failed: %v", err)
	}

	// Should detect multiple sub-projects
	expectedMinProjects := 6
	if len(workspaceCtx.SubProjects) < expectedMinProjects {
		t.Errorf("Expected at least %d sub-projects, got %d", expectedMinProjects, len(workspaceCtx.SubProjects))
	}

	// Verify workspace-level language aggregation
	expectedLanguages := []string{
		types.PROJECT_TYPE_GO,
		types.PROJECT_TYPE_NODEJS,
		types.PROJECT_TYPE_TYPESCRIPT,
		types.PROJECT_TYPE_PYTHON,
		types.PROJECT_TYPE_JAVA,
	}

	for _, expectedLang := range expectedLanguages {
		if !contains(workspaceCtx.Languages, expectedLang) {
			t.Errorf("Expected workspace language '%s' not found in: %v", expectedLang, workspaceCtx.Languages)
		}
	}

	// Workspace should be marked as mixed
	if workspaceCtx.ProjectType != types.PROJECT_TYPE_MIXED {
		t.Errorf("Expected workspace project type %s, got %s", types.PROJECT_TYPE_MIXED, workspaceCtx.ProjectType)
	}

	// Verify specific project types
	projectMap := make(map[string]*DetectedSubProject)
	for _, sp := range workspaceCtx.SubProjects {
		projectMap[sp.RelativePath] = sp
	}

	// Frontend should detect both Node.js and TypeScript but be marked as mixed
	frontendProject := projectMap["frontend"]
	if frontendProject != nil {
		if frontendProject.ProjectType != types.PROJECT_TYPE_MIXED {
			t.Errorf("Expected frontend project type %s, got %s", types.PROJECT_TYPE_MIXED, frontendProject.ProjectType)
		}
		if len(frontendProject.Languages) < 2 {
			t.Errorf("Expected frontend to have multiple languages, got: %v", frontendProject.Languages)
		}
	}

	// Backend should detect Python (multiple markers)
	backendProject := projectMap["backend"]
	if backendProject != nil {
		if backendProject.ProjectType != types.PROJECT_TYPE_PYTHON {
			t.Errorf("Expected backend project type %s, got %s", types.PROJECT_TYPE_PYTHON, backendProject.ProjectType)
		}
		if len(backendProject.MarkerFiles) < 2 {
			t.Errorf("Expected backend to have multiple marker files, got: %v", backendProject.MarkerFiles)
		}
	}
}

// TestSubProjectIDGeneration tests unique ID generation for sub-projects
func TestSubProjectIDGeneration(t *testing.T) {
	t.Parallel()
	workspace, cleanup := createTestWorkspace(t, map[string]interface{}{
		"go.mod":                  "module root",
		"frontend/package.json":   `{"name": "frontend"}`,
		"backend/requirements.txt": "flask==2.0.0",
	})
	defer cleanup()

	detector := NewWorkspaceDetector()
	workspaceCtx, err := detector.DetectWorkspaceAt(workspace)
	if err != nil {
		t.Fatalf("DetectWorkspaceAt failed: %v", err)
	}

	// Verify all projects have unique IDs
	idMap := make(map[string]*DetectedSubProject)
	for _, project := range workspaceCtx.SubProjects {
		if project.ID == "" {
			t.Errorf("Project at '%s' has empty ID", project.RelativePath)
			continue
		}

		if !strings.HasPrefix(project.ID, "project_") {
			t.Errorf("Project ID '%s' doesn't have expected prefix 'project_'", project.ID)
		}

		if existingProject, exists := idMap[project.ID]; exists {
			t.Errorf("Duplicate project ID '%s' found for projects '%s' and '%s'", 
				project.ID, project.RelativePath, existingProject.RelativePath)
		}
		idMap[project.ID] = project
	}

	// Verify ID consistency across multiple detections
	workspaceCtx2, err := detector.DetectWorkspaceAt(workspace)
	if err != nil {
		t.Fatalf("Second DetectWorkspaceAt failed: %v", err)
	}

	for _, project1 := range workspaceCtx.SubProjects {
		for _, project2 := range workspaceCtx2.SubProjects {
			if project1.RelativePath == project2.RelativePath {
				if project1.ID != project2.ID {
					t.Errorf("Inconsistent ID generation for project '%s': '%s' vs '%s'", 
						project1.RelativePath, project1.ID, project2.ID)
				}
			}
		}
	}
}

// TestBackwardCompatibility tests that legacy API behavior is unchanged
func TestBackwardCompatibility(t *testing.T) {
	t.Parallel()
	// Test with single Go project (should behave like legacy)
	singleProject, cleanup1 := createTestWorkspace(t, map[string]interface{}{
		"go.mod": "module test",
		"main.go": "package main",
	})
	defer cleanup1()

	detector := NewWorkspaceDetector()
	workspaceCtx, err := detector.DetectWorkspaceAt(singleProject)
	if err != nil {
		t.Fatalf("DetectWorkspaceAt failed: %v", err)
	}

	// Legacy fields should be populated correctly
	if workspaceCtx.ProjectType != types.PROJECT_TYPE_GO {
		t.Errorf("Expected legacy ProjectType %s, got %s", types.PROJECT_TYPE_GO, workspaceCtx.ProjectType)
	}

	if !contains(workspaceCtx.Languages, types.PROJECT_TYPE_GO) {
		t.Errorf("Expected Go in Languages: %v", workspaceCtx.Languages)
	}

	// Should have exactly one sub-project
	if len(workspaceCtx.SubProjects) != 1 {
		t.Errorf("Expected 1 sub-project, got %d", len(workspaceCtx.SubProjects))
	}

	// Root sub-project should have correct properties
	rootProject := workspaceCtx.SubProjects[0]
	if rootProject.RelativePath != "." {
		t.Errorf("Expected root project relative path '.', got '%s'", rootProject.RelativePath)
	}

	// Test mixed project backward compatibility
	mixedProject, cleanup2 := createTestWorkspace(t, map[string]interface{}{
		"go.mod":        "module test",
		"package.json":  `{"name": "test"}`,
	})
	defer cleanup2()

	workspaceCtx2, err := detector.DetectWorkspaceAt(mixedProject)
	if err != nil {
		t.Fatalf("DetectWorkspaceAt failed for mixed project: %v", err)
	}

	if workspaceCtx2.ProjectType != types.PROJECT_TYPE_MIXED {
		t.Errorf("Expected legacy ProjectType %s for mixed project, got %s", 
			types.PROJECT_TYPE_MIXED, workspaceCtx2.ProjectType)
	}

	if len(workspaceCtx2.Languages) < 2 {
		t.Errorf("Expected multiple languages for mixed project, got: %v", workspaceCtx2.Languages)
	}
}

// TestPerformanceLimits tests file scanning limits and performance
func TestPerformanceLimits(t *testing.T) {
	t.Parallel()
	workspace, cleanup := createLargeWorkspace(t, 100) // 100 subdirectories
	defer cleanup()

	detector := NewWorkspaceDetector()
	
	// Test with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	workspaceCtx, err := detector.DetectWorkspaceWithContext(ctx, workspace)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("DetectWorkspaceWithContext failed: %v", err)
	}

	// Should complete within reasonable time
	if duration > 3*time.Second {
		t.Errorf("Detection took too long: %v", duration)
	}

	// Should respect file scan limits (not fail due to limits)
	if workspaceCtx == nil {
		t.Fatal("WorkspaceContext is nil")
	}

	t.Logf("Detected %d sub-projects in %v", len(workspaceCtx.SubProjects), duration)
}

// TestPermissionErrors tests graceful handling of permission denied directories
func TestPermissionErrors(t *testing.T) {
	t.Parallel()
	workspace, cleanup := createTestWorkspace(t, map[string]interface{}{
		"go.mod": "module root",
		"accessible/package.json": `{"name": "accessible"}`,
	})
	defer cleanup()

	// Create a directory with restricted permissions (skip on Windows)
	restrictedDir := filepath.Join(workspace, "restricted")
	if err := os.Mkdir(restrictedDir, 0000); err != nil {
		t.Skipf("Cannot create restricted directory: %v", err)
	}
	defer os.Chmod(restrictedDir, 0755) // Restore permissions for cleanup

	// Create a project marker in the restricted directory
	restrictedMarker := filepath.Join(restrictedDir, "go.mod")
	if err := os.WriteFile(restrictedMarker, []byte("module restricted"), 0644); err != nil {
		// If we can't write, that's actually what we want for this test
	}

	detector := NewWorkspaceDetector()
	workspaceCtx, err := detector.DetectWorkspaceAt(workspace)
	
	// Should not fail due to permission errors
	if err != nil {
		t.Fatalf("DetectWorkspaceAt should handle permission errors gracefully: %v", err)
	}

	// Should still detect accessible projects
	if len(workspaceCtx.SubProjects) < 2 {
		t.Errorf("Expected at least 2 accessible projects, got %d", len(workspaceCtx.SubProjects))
	}

	// Verify accessible project was detected
	found := false
	for _, project := range workspaceCtx.SubProjects {
		if project.RelativePath == "accessible" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Accessible project should be detected despite permission errors")
	}
}

// TestFindSubProjectForPath tests the FindSubProjectForPath functionality
func TestFindSubProjectForPath(t *testing.T) {
	t.Parallel()
	workspace, cleanup := createTestWorkspace(t, map[string]interface{}{
		"go.mod":                  "module root",
		"frontend/package.json":   `{"name": "frontend"}`,
		"services/auth/go.mod":    "module auth",
		"services/api/pom.xml":    `<project></project>`,
	})
	defer cleanup()

	detector := NewWorkspaceDetector()
	workspaceCtx, err := detector.DetectWorkspaceAt(workspace)
	if err != nil {
		t.Fatalf("DetectWorkspaceAt failed: %v", err)
	}

	testCases := []struct {
		filePath        string
		expectedProject string
		description     string
	}{
		{
			filePath:        filepath.Join(workspace, "main.go"),
			expectedProject: ".",
			description:     "Root file should map to root project",
		},
		{
			filePath:        filepath.Join(workspace, "frontend", "src", "components", "App.js"),
			expectedProject: "frontend",
			description:     "Deep frontend file should map to frontend project",
		},
		{
			filePath:        filepath.Join(workspace, "services", "auth", "handlers", "login.go"),
			expectedProject: "services/auth",
			description:     "Deep auth file should map to auth project",
		},
		{
			filePath:        filepath.Join(workspace, "services", "api", "controllers", "UserController.java"),
			expectedProject: "services/api",
			description:     "Deep API file should map to API project",
		},
		{
			filePath:        filepath.Join(workspace, "docs", "architecture.md"),
			expectedProject: ".",
			description:     "Unmapped file should default to root project",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			foundProject := detector.FindSubProjectForPath(workspaceCtx, tc.filePath)
			if foundProject == nil {
				t.Fatalf("No project found for path: %s", tc.filePath)
			}
			if foundProject.RelativePath != tc.expectedProject {
				t.Errorf("Expected project '%s', got '%s' for path: %s", 
					tc.expectedProject, foundProject.RelativePath, tc.filePath)
			}
		})
	}
}

// TestEmptyAndUnknownProjects tests handling of edge cases
func TestEmptyAndUnknownProjects(t *testing.T) {
	t.Parallel()
	// Test completely empty directory
	emptyDir, err := os.MkdirTemp("", "empty_workspace_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(emptyDir)

	detector := NewWorkspaceDetector()
	workspaceCtx, err := detector.DetectWorkspaceAt(emptyDir)
	if err != nil {
		t.Fatalf("DetectWorkspaceAt failed for empty directory: %v", err)
	}

	// Should create a single unknown project
	if len(workspaceCtx.SubProjects) != 1 {
		t.Errorf("Expected 1 sub-project for empty directory, got %d", len(workspaceCtx.SubProjects))
	}

	rootProject := workspaceCtx.SubProjects[0]
	if rootProject.ProjectType != types.PROJECT_TYPE_UNKNOWN {
		t.Errorf("Expected unknown project type, got %s", rootProject.ProjectType)
	}

	if workspaceCtx.ProjectType != types.PROJECT_TYPE_UNKNOWN {
		t.Errorf("Expected workspace project type unknown, got %s", workspaceCtx.ProjectType)
	}

	// Test directory with non-project files
	nonProjectDir, cleanup := createTestWorkspace(t, map[string]interface{}{
		"README.md":     "# Documentation",
		"LICENSE":       "MIT License",
		"docs/guide.md": "# User Guide",
	})
	defer cleanup()

	workspaceCtx2, err := detector.DetectWorkspaceAt(nonProjectDir)
	if err != nil {
		t.Fatalf("DetectWorkspaceAt failed for non-project directory: %v", err)
	}

	// Should still create root project as unknown
	if len(workspaceCtx2.SubProjects) != 1 {
		t.Errorf("Expected 1 sub-project for non-project directory, got %d", len(workspaceCtx2.SubProjects))
	}

	if workspaceCtx2.SubProjects[0].ProjectType != types.PROJECT_TYPE_UNKNOWN {
		t.Errorf("Expected unknown project type for non-project directory, got %s", 
			workspaceCtx2.SubProjects[0].ProjectType)
	}
}

// TestWorkspaceHashGeneration tests workspace hash generation with sub-projects
func TestWorkspaceHashGeneration(t *testing.T) {
	t.Parallel()
	workspace, cleanup := createTestWorkspace(t, map[string]interface{}{
		"go.mod":                  "module root",
		"frontend/package.json":   `{"name": "frontend"}`,
	})
	defer cleanup()

	detector := NewWorkspaceDetector()
	
	// Detect same workspace twice
	workspaceCtx1, err := detector.DetectWorkspaceAt(workspace)
	if err != nil {
		t.Fatalf("First DetectWorkspaceAt failed: %v", err)
	}

	workspaceCtx2, err := detector.DetectWorkspaceAt(workspace)
	if err != nil {
		t.Fatalf("Second DetectWorkspaceAt failed: %v", err)
	}

	// Hashes should be identical for same workspace
	if workspaceCtx1.Hash != workspaceCtx2.Hash {
		t.Errorf("Workspace hashes should be identical: '%s' vs '%s'", 
			workspaceCtx1.Hash, workspaceCtx2.Hash)
	}

	// Hash should not be empty
	if workspaceCtx1.Hash == "" {
		t.Error("Workspace hash should not be empty")
	}

	// Modify workspace and verify hash changes
	newFile := filepath.Join(workspace, "backend", "requirements.txt")
	os.MkdirAll(filepath.Dir(newFile), 0755)
	os.WriteFile(newFile, []byte("flask==2.0.0"), 0644)

	workspaceCtx3, err := detector.DetectWorkspaceAt(workspace)
	if err != nil {
		t.Fatalf("Third DetectWorkspaceAt failed: %v", err)
	}

	// Hash should change after adding new project
	if workspaceCtx1.Hash == workspaceCtx3.Hash {
		t.Error("Workspace hash should change after adding new sub-project")
	}
}

// ====== HELPER FUNCTIONS FOR TESTS ======

// createTestWorkspace creates a temporary workspace with the specified structure
func createTestWorkspace(t *testing.T, structure map[string]interface{}) (string, func()) {
	tempDir, err := os.MkdirTemp("", "workspace_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	for path, content := range structure {
		fullPath := filepath.Join(tempDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory for %s: %v", path, err)
		}

		var contentStr string
		switch v := content.(type) {
		case string:
			contentStr = v
		default:
			contentStr = fmt.Sprintf("%v", v)
		}

		if err := os.WriteFile(fullPath, []byte(contentStr), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", path, err)
		}
	}

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

// createNestedProjects creates a workspace with projects nested to specified depth
func createNestedProjects(t *testing.T, maxDepth int) (string, func()) {
	tempDir, err := os.MkdirTemp("", "nested_projects_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create root project
	rootGoMod := filepath.Join(tempDir, "go.mod")
	if err := os.WriteFile(rootGoMod, []byte("module root"), 0644); err != nil {
		t.Fatalf("Failed to create root go.mod: %v", err)
	}

	// Create nested projects
	currentDir := tempDir
	for i := 1; i <= maxDepth; i++ {
		subDir := filepath.Join(currentDir, fmt.Sprintf("level%d", i))
		if err := os.Mkdir(subDir, 0755); err != nil {
			t.Fatalf("Failed to create directory at level %d: %v", i, err)
		}

		goModPath := filepath.Join(subDir, "go.mod")
		if err := os.WriteFile(goModPath, []byte(fmt.Sprintf("module level%d", i)), 0644); err != nil {
			t.Fatalf("Failed to create go.mod at level %d: %v", i, err)
		}

		currentDir = subDir
	}

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

// createLargeWorkspace creates a workspace with many subdirectories for performance testing
func createLargeWorkspace(t *testing.T, numDirs int) (string, func()) {
	tempDir, err := os.MkdirTemp("", "large_workspace_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create root project
	rootGoMod := filepath.Join(tempDir, "go.mod")
	if err := os.WriteFile(rootGoMod, []byte("module root"), 0644); err != nil {
		t.Fatalf("Failed to create root go.mod: %v", err)
	}

	// Create many subdirectories with some containing projects
	for i := 0; i < numDirs; i++ {
		subDir := filepath.Join(tempDir, fmt.Sprintf("subdir%d", i))
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("Failed to create subdir %d: %v", i, err)
		}

		// Every 10th directory gets a project marker
		if i%10 == 0 {
			markerPath := filepath.Join(subDir, "package.json")
			content := fmt.Sprintf(`{"name": "project%d"}`, i)
			if err := os.WriteFile(markerPath, []byte(content), 0644); err != nil {
				t.Fatalf("Failed to create project marker in subdir %d: %v", i, err)
			}
		}

		// Add some regular files
		for j := 0; j < 5; j++ {
			filePath := filepath.Join(subDir, fmt.Sprintf("file%d.txt", j))
			if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
				t.Fatalf("Failed to create file in subdir %d: %v", i, err)
			}
		}
	}

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

// verifySubProject validates a DetectedSubProject instance
func verifySubProject(t *testing.T, sp *DetectedSubProject, expectedType string, expectedPath string) {
	if sp == nil {
		t.Fatal("DetectedSubProject is nil")
	}

	if sp.ProjectType != expectedType {
		t.Errorf("Expected project type %s, got %s", expectedType, sp.ProjectType)
	}

	if sp.RelativePath != expectedPath {
		t.Errorf("Expected relative path %s, got %s", expectedPath, sp.RelativePath)
	}

	if sp.ID == "" {
		t.Error("Project ID is empty")
	}

	if sp.Name == "" {
		t.Error("Project name is empty")
	}

	if sp.AbsolutePath == "" {
		t.Error("Absolute path is empty")
	}

	if len(sp.Languages) == 0 {
		t.Error("Project languages is empty")
	}
}

// assertProjectMapping validates path-to-project mapping
func assertProjectMapping(t *testing.T, detector WorkspaceDetector, workspace *WorkspaceContext, testCases []pathTestCase) {
	for _, tc := range testCases {
		t.Run(tc.description, func(subT *testing.T) {
			foundProject := detector.FindSubProjectForPath(workspace, tc.filePath)
			if foundProject == nil {
				subT.Fatalf("No project found for path: %s", tc.filePath)
			}
			if foundProject.RelativePath != tc.expectedProject {
				subT.Errorf("Expected project '%s', got '%s' for path: %s", 
					tc.expectedProject, foundProject.RelativePath, tc.filePath)
			}
		})
	}
}