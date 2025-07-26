package project_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"lsp-gateway/internal/project"
	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/setup"
	"lsp-gateway/tests/framework"
)

// DefaultProjectDetectorTestSuite provides comprehensive unit tests for DefaultProjectDetector
type DefaultProjectDetectorTestSuite struct {
	suite.Suite
	tempDir             string
	detector            *project.DefaultProjectDetector
	projectGenerator    *framework.TestProjectGenerator
	performanceProfiler *framework.PerformanceProfiler
	logger              *setup.SetupLogger
}

func (suite *DefaultProjectDetectorTestSuite) SetupSuite() {
	// Create temporary directory for all tests
	tempDir, err := os.MkdirTemp("", "default-project-detector-test-*")
	suite.Require().NoError(err)
	suite.tempDir = tempDir

	// Initialize test framework components
	suite.projectGenerator = framework.NewTestProjectGenerator(suite.tempDir)
	suite.performanceProfiler = framework.NewPerformanceProfiler()
	suite.logger = setup.NewSetupLogger(nil)

	// Load project templates
	ctx := context.Background()
	err = suite.projectGenerator.LoadTemplates(ctx)
	suite.Require().NoError(err)
}

func (suite *DefaultProjectDetectorTestSuite) TearDownSuite() {
	if suite.tempDir != "" {
		_ = os.RemoveAll(suite.tempDir)
	}
	if suite.performanceProfiler != nil {
		suite.performanceProfiler.Reset()
	}
	if suite.projectGenerator != nil {
		suite.projectGenerator.Reset()
	}
}

func (suite *DefaultProjectDetectorTestSuite) SetupTest() {
	// Create fresh detector for each test
	suite.detector = project.NewProjectDetector()
	suite.detector.SetLogger(suite.logger)
}

func (suite *DefaultProjectDetectorTestSuite) TearDownTest() {
	// Clean up detector resources
	suite.detector = nil
}

// Constructor and Initialization Tests

func (suite *DefaultProjectDetectorTestSuite) TestNewDefaultProjectDetector() {
	detector := project.NewProjectDetector()

	suite.NotNil(detector, "Constructor should return non-nil detector")

	// Verify default configuration values
	supportedLanguages := detector.GetSupportedLanguages()
	expectedLanguages := []string{
		types.PROJECT_TYPE_GO,
		types.PROJECT_TYPE_PYTHON,
		types.PROJECT_TYPE_JAVA,
		types.PROJECT_TYPE_TYPESCRIPT,
		types.PROJECT_TYPE_NODEJS,
	}

	for _, lang := range expectedLanguages {
		suite.Contains(supportedLanguages, lang,
			fmt.Sprintf("Should support %s language detection", lang))
	}
}

func (suite *DefaultProjectDetectorTestSuite) TestDefaultProjectDetector_RegisterDetector() {
	// Test registering custom detectors
	customDetectors := make(map[string]project.LanguageDetector)
	// Note: We'd need to create mock detectors here, but for the sake of this test
	// we'll test the configuration mechanism

	suite.detector.SetCustomDetectors(customDetectors)

	// Verify the detector still functions correctly
	languages := suite.detector.GetSupportedLanguages()
	suite.NotEmpty(languages, "Should still support built-in languages")
}

func (suite *DefaultProjectDetectorTestSuite) TestDefaultProjectDetector_Configuration() {
	// Test timeout configuration
	newTimeout := 60 * time.Second

	suite.detector.SetTimeout(newTimeout)
	// Configuration is internal, but we can verify it doesn't break functionality

	// Test max depth configuration
	suite.detector.SetMaxDepth(5)
	// Again, internal configuration but should not break functionality

	// Test logger configuration
	customLogger := setup.NewSetupLogger(nil)
	suite.detector.SetLogger(customLogger)

	// Verify detector still functions
	languages := suite.detector.GetSupportedLanguages()
	suite.NotEmpty(languages, "Configuration changes should not break functionality")
}

// Core Detection Methods Tests

func (suite *DefaultProjectDetectorTestSuite) TestDefaultProjectDetector_DetectProject() {
	testCases := []struct {
		name              string
		projectType       framework.ProjectType
		languages         []string
		complexity        framework.ProjectComplexity
		expectedType      string
		expectedLanguages []string
		shouldSucceed     bool
	}{
		{
			name:              "simple-go-project",
			projectType:       framework.ProjectTypeSingleLanguage,
			languages:         []string{"go"},
			complexity:        framework.ComplexitySimple,
			expectedType:      types.PROJECT_TYPE_GO,
			expectedLanguages: []string{"go"},
			shouldSucceed:     true,
		},
		{
			name:              "complex-python-project",
			projectType:       framework.ProjectTypeSingleLanguage,
			languages:         []string{"python"},
			complexity:        framework.ComplexityComplex,
			expectedType:      types.PROJECT_TYPE_PYTHON,
			expectedLanguages: []string{"python"},
			shouldSucceed:     true,
		},
		{
			name:              "multi-language-project",
			projectType:       framework.ProjectTypeMultiLanguage,
			languages:         []string{"go", "python", "typescript"},
			complexity:        framework.ComplexityMedium,
			expectedType:      types.PROJECT_TYPE_MIXED,
			expectedLanguages: []string{"go", "python", "typescript"},
			shouldSucceed:     true,
		},
		{
			name:              "typescript-project",
			projectType:       framework.ProjectTypeSingleLanguage,
			languages:         []string{"typescript"},
			complexity:        framework.ComplexityMedium,
			expectedType:      types.PROJECT_TYPE_TYPESCRIPT,
			expectedLanguages: []string{"typescript"},
			shouldSucceed:     true,
		},
		{
			name:              "java-project",
			projectType:       framework.ProjectTypeSingleLanguage,
			languages:         []string{"java"},
			complexity:        framework.ComplexityMedium,
			expectedType:      types.PROJECT_TYPE_JAVA,
			expectedLanguages: []string{"java"},
			shouldSucceed:     true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Generate test project
			config := &framework.ProjectGenerationConfig{
				Type:        tc.projectType,
				Languages:   tc.languages,
				Complexity:  tc.complexity,
				Size:        framework.SizeMedium,
				BuildSystem: true,
				TestFiles:   true,
			}

			testProject, err := suite.projectGenerator.GenerateProject(config)
			suite.Require().NoError(err)
			suite.Require().NotNil(testProject)

			// Test detection
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			projectCtx, err := suite.detector.DetectProject(ctx, testProject.RootPath)

			if tc.shouldSucceed {
				suite.NoError(err, "Detection should succeed for %s", tc.name)
				suite.NotNil(projectCtx, "Should return project context")

				suite.Equal(tc.expectedType, projectCtx.ProjectType,
					"Should detect correct project type")

				for _, expectedLang := range tc.expectedLanguages {
					suite.Contains(projectCtx.Languages, expectedLang,
						"Should detect language %s", expectedLang)
				}

				// Verify required fields are populated
				suite.NotEmpty(projectCtx.RootPath, "Should have root path")
				suite.NotEmpty(projectCtx.RequiredServers, "Should have LSP servers")
				suite.Greater(projectCtx.Confidence, 0.0, "Should have confidence > 0")
				suite.NotEmpty(projectCtx.MarkerFiles, "Should have marker files")
			} else {
				suite.Error(err, "Detection should fail for %s", tc.name)
			}
		})
	}
}

func (suite *DefaultProjectDetectorTestSuite) TestDefaultProjectDetector_DetectProjectType() {
	testCases := []struct {
		name         string
		setupFiles   map[string]string
		expectedType string
	}{
		{
			name: "go-module-detection",
			setupFiles: map[string]string{
				"go.mod":  "module test\n\ngo 1.19",
				"main.go": "package main\n\nfunc main() {}",
			},
			expectedType: types.PROJECT_TYPE_GO,
		},
		{
			name: "python-project-detection",
			setupFiles: map[string]string{
				"setup.py":         "from setuptools import setup\n\nsetup(name='test')",
				"requirements.txt": "flask==2.0.1",
				"main.py":          "print('hello')",
			},
			expectedType: types.PROJECT_TYPE_PYTHON,
		},
		{
			name: "typescript-project-detection",
			setupFiles: map[string]string{
				"package.json":  `{"name": "test", "dependencies": {"typescript": "^4.0.0"}}`,
				"tsconfig.json": `{"compilerOptions": {"target": "es2020"}}`,
				"index.ts":      "console.log('hello');",
			},
			expectedType: types.PROJECT_TYPE_TYPESCRIPT,
		},
		{
			name: "java-maven-detection",
			setupFiles: map[string]string{
				"pom.xml": `<?xml version="1.0"?>
<project>
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>test</artifactId>
    <version>1.0.0</version>
</project>`,
				"src/main/java/Main.java": "public class Main { public static void main(String[] args) {} }",
			},
			expectedType: types.PROJECT_TYPE_JAVA,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create temporary directory for this test
			testDir, err := os.MkdirTemp(suite.tempDir, fmt.Sprintf("type-test-%s-*", tc.name))
			suite.Require().NoError(err)
			defer func() { _ = os.RemoveAll(testDir) }()

			// Create test files
			for filePath, content := range tc.setupFiles {
				fullPath := filepath.Join(testDir, filePath)
				err := os.MkdirAll(filepath.Dir(fullPath), 0755)
				suite.Require().NoError(err)
				err = os.WriteFile(fullPath, []byte(content), 0644)
				suite.Require().NoError(err)
			}

			// Test project type detection
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			detectedType, err := suite.detector.DetectProjectType(ctx, testDir)
			suite.NoError(err, "Project type detection should succeed")
			suite.Equal(tc.expectedType, detectedType,
				"Should detect correct project type for %s", tc.name)
		})
	}
}

func (suite *DefaultProjectDetectorTestSuite) TestDefaultProjectDetector_GetWorkspaceRoot() {
	testCases := []struct {
		name           string
		setupStructure map[string]string
		testPath       string
		expectedRoot   string
		shouldFind     bool
	}{
		{
			name: "git-workspace-root",
			setupStructure: map[string]string{
				".git/config":           "git config",
				"go.mod":                "module test",
				"subdir/nested/file.go": "package nested",
			},
			testPath:     "subdir/nested",
			expectedRoot: "", // Will be set to the actual root
			shouldFind:   true,
		},
		{
			name: "go-module-root",
			setupStructure: map[string]string{
				"go.mod":             "module test",
				"cmd/app/main.go":    "package main",
				"pkg/utils/utils.go": "package utils",
			},
			testPath:     "cmd/app",
			expectedRoot: "", // Will be set to the actual root
			shouldFind:   true,
		},
		{
			name: "no-workspace-markers",
			setupStructure: map[string]string{
				"random.txt": "random content",
				"file.go":    "package main",
			},
			testPath:     "",
			expectedRoot: "",
			shouldFind:   false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create temporary directory for this test
			testRootDir, err := os.MkdirTemp(suite.tempDir, fmt.Sprintf("workspace-test-%s-*", tc.name))
			suite.Require().NoError(err)
			defer func() { _ = os.RemoveAll(testRootDir) }()

			// Create test structure
			for filePath, content := range tc.setupStructure {
				fullPath := filepath.Join(testRootDir, filePath)
				err := os.MkdirAll(filepath.Dir(fullPath), 0755)
				suite.Require().NoError(err)
				err = os.WriteFile(fullPath, []byte(content), 0644)
				suite.Require().NoError(err)
			}

			// Determine test path
			testPath := testRootDir
			if tc.testPath != "" {
				testPath = filepath.Join(testRootDir, tc.testPath)
			}

			// Test workspace root detection
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			workspaceRoot, err := suite.detector.GetWorkspaceRoot(ctx, testPath)

			if tc.shouldFind {
				suite.NoError(err, "Should find workspace root")
				suite.NotEmpty(workspaceRoot, "Should return non-empty workspace root")
				suite.True(strings.HasPrefix(testPath, workspaceRoot) ||
					strings.HasPrefix(workspaceRoot, testRootDir),
					"Workspace root should be parent of test path")
			} else {
				// May return error or empty root depending on implementation
				if err == nil {
					suite.Empty(workspaceRoot, "Should return empty root when no markers found")
				}
			}
		})
	}
}

func (suite *DefaultProjectDetectorTestSuite) TestDefaultProjectDetector_ValidateProject() {
	// Create a valid test project
	config := &framework.ProjectGenerationConfig{
		Type:        framework.ProjectTypeSingleLanguage,
		Languages:   []string{"go"},
		Complexity:  framework.ComplexitySimple,
		Size:        framework.SizeSmall,
		BuildSystem: true,
		TestFiles:   true,
	}

	testProject, err := suite.projectGenerator.GenerateProject(config)
	suite.Require().NoError(err)

	// First detect the project
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	projectCtx, err := suite.detector.DetectProject(ctx, testProject.RootPath)
	suite.Require().NoError(err)
	suite.Require().NotNil(projectCtx)

	// Test validation of valid project
	err = suite.detector.ValidateProject(ctx, projectCtx)
	suite.NoError(err, "Valid project should pass validation")

	// Test validation with nil context
	err = suite.detector.ValidateProject(ctx, nil)
	suite.Error(err, "Should error with nil project context")

	// Test validation with invalid project context
	invalidCtx := &project.ProjectContext{
		ProjectType: types.PROJECT_TYPE_GO,
		RootPath:    "/non/existent/path",
		Languages:   []string{"go"},
	}

	err = suite.detector.ValidateProject(ctx, invalidCtx)
	suite.Error(err, "Should error with invalid project path")
}

func (suite *DefaultProjectDetectorTestSuite) TestDefaultProjectDetector_ScanWorkspace() {
	// Create a workspace with multiple projects
	workspaceDir, err := os.MkdirTemp(suite.tempDir, "workspace-scan-test-*")
	suite.Require().NoError(err)
	defer func() { _ = os.RemoveAll(workspaceDir) }()

	// Create multiple projects in the workspace
	projects := []struct {
		name      string
		language  string
		structure map[string]string
	}{
		{
			name:     "go-service",
			language: "go",
			structure: map[string]string{
				"go.mod":  "module go-service\n\ngo 1.19",
				"main.go": "package main\n\nfunc main() {}",
			},
		},
		{
			name:     "python-api",
			language: "python",
			structure: map[string]string{
				"setup.py":         "from setuptools import setup\n\nsetup(name='python-api')",
				"requirements.txt": "flask==2.0.1",
				"app.py":           "print('python api')",
			},
		},
		{
			name:     "ts-frontend",
			language: "typescript",
			structure: map[string]string{
				"package.json":  `{"name": "ts-frontend", "dependencies": {"typescript": "^4.0.0"}}`,
				"tsconfig.json": `{"compilerOptions": {"target": "es2020"}}`,
				"src/index.ts":  "console.log('frontend');",
			},
		},
	}

	for _, proj := range projects {
		projectDir := filepath.Join(workspaceDir, proj.name)
		err := os.MkdirAll(projectDir, 0755)
		suite.Require().NoError(err)

		for filePath, content := range proj.structure {
			fullPath := filepath.Join(projectDir, filePath)
			err := os.MkdirAll(filepath.Dir(fullPath), 0755)
			suite.Require().NoError(err)
			err = os.WriteFile(fullPath, []byte(content), 0644)
			suite.Require().NoError(err)
		}
	}

	// Test workspace scanning
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	detectedProjects, err := suite.detector.ScanWorkspace(ctx, workspaceDir)
	suite.NoError(err, "Workspace scanning should succeed")
	suite.NotEmpty(detectedProjects, "Should detect projects in workspace")

	// Verify all projects were detected
	suite.GreaterOrEqual(len(detectedProjects), len(projects),
		"Should detect at least as many projects as created")

	// Check that we found projects with different languages
	detectedLanguages := make(map[string]bool)
	for _, projCtx := range detectedProjects {
		for _, lang := range projCtx.Languages {
			detectedLanguages[lang] = true
		}
	}

	suite.True(detectedLanguages["go"], "Should detect Go project")
	suite.True(detectedLanguages["python"], "Should detect Python project")
	suite.True(detectedLanguages["typescript"], "Should detect TypeScript project")
}

func (suite *DefaultProjectDetectorTestSuite) TestDefaultProjectDetector_DetectMultipleProjects() {
	// Create multiple separate project directories
	projectPaths := make([]string, 0)

	for i := 0; i < 3; i++ {
		config := &framework.ProjectGenerationConfig{
			Type:        framework.ProjectTypeSingleLanguage,
			Languages:   []string{"go"},
			Complexity:  framework.ComplexitySimple,
			Size:        framework.SizeSmall,
			BuildSystem: true,
		}

		testProject, err := suite.projectGenerator.GenerateProject(config)
		suite.Require().NoError(err)
		projectPaths = append(projectPaths, testProject.RootPath)
	}

	// Test multiple project detection (we need to implement this method or test batch detection)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test detecting each project individually
	detectedProjects := make([]*project.ProjectContext, 0)
	for _, path := range projectPaths {
		projCtx, err := suite.detector.DetectProject(ctx, path)
		suite.NoError(err, "Should detect project at %s", path)
		suite.NotNil(projCtx, "Should return project context")
		detectedProjects = append(detectedProjects, projCtx)
	}

	suite.Equal(len(projectPaths), len(detectedProjects),
		"Should detect all projects")

	// Verify all detected projects are Go projects
	for _, projCtx := range detectedProjects {
		suite.Equal(types.PROJECT_TYPE_GO, projCtx.ProjectType, "Should be Go project")
		suite.Contains(projCtx.Languages, "go", "Should contain Go language")
	}
}

// Configuration and Edge Case Tests

func (suite *DefaultProjectDetectorTestSuite) TestDefaultProjectDetector_TimeoutHandling() {
	// Create a simple project
	config := &framework.ProjectGenerationConfig{
		Type:        framework.ProjectTypeSingleLanguage,
		Languages:   []string{"go"},
		Complexity:  framework.ComplexitySimple,
		Size:        framework.SizeSmall,
		BuildSystem: true,
	}

	testProject, err := suite.projectGenerator.GenerateProject(config)
	suite.Require().NoError(err)

	// Test with very short timeout
	shortCtx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	_, err = suite.detector.DetectProject(shortCtx, testProject.RootPath)

	// Should either succeed quickly or timeout
	if err != nil {
		suite.Contains(err.Error(), "context deadline exceeded",
			"Should timeout with context deadline error")
	}

	// Test with reasonable timeout
	normalCtx, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel2()

	projCtx, err := suite.detector.DetectProject(normalCtx, testProject.RootPath)
	suite.NoError(err, "Should succeed with normal timeout")
	suite.NotNil(projCtx, "Should return project context")
}

func (suite *DefaultProjectDetectorTestSuite) TestDefaultProjectDetector_MaxDepthLimits() {
	// Create deeply nested project structure
	deepDir, err := os.MkdirTemp(suite.tempDir, "deep-test-*")
	suite.Require().NoError(err)
	defer func() { _ = os.RemoveAll(deepDir) }()

	// Create nested structure
	currentPath := deepDir
	for depth := 0; depth < 15; depth++ {
		currentPath = filepath.Join(currentPath, fmt.Sprintf("level-%d", depth))
		err := os.MkdirAll(currentPath, 0755)
		suite.Require().NoError(err)
	}

	// Add a go.mod at the deep level
	goModPath := filepath.Join(currentPath, "go.mod")
	err = os.WriteFile(goModPath, []byte("module deep-test\n\ngo 1.19"), 0644)
	suite.Require().NoError(err)

	mainGoPath := filepath.Join(currentPath, "main.go")
	err = os.WriteFile(mainGoPath, []byte("package main\n\nfunc main() {}"), 0644)
	suite.Require().NoError(err)

	// Test with limited depth
	suite.detector.SetMaxDepth(5)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Detection from root should not find the deep project due to depth limit
	projCtx, err := suite.detector.DetectProject(ctx, deepDir)

	// Depending on implementation, this might succeed (finding no projects) or fail
	// The key is that it should not hang or crash
	if err != nil {
		suite.NotContains(err.Error(), "panic", "Should not panic with depth limits")
	} else if projCtx != nil {
		// If it finds something, it should be valid
		suite.NotEmpty(projCtx.ProjectType, "Detected project should have valid type")
	}

	// Test detection from deeper starting point
	midPath := filepath.Join(deepDir, "level-0", "level-1", "level-2")
	projCtx2, err2 := suite.detector.DetectProject(ctx, midPath)

	// Similar expectations
	if err2 != nil {
		suite.NotContains(err2.Error(), "panic", "Should not panic with depth limits")
	} else if projCtx2 != nil {
		suite.NotEmpty(projCtx2.ProjectType, "Detected project should have valid type")
	}
}

func (suite *DefaultProjectDetectorTestSuite) TestDefaultProjectDetector_IgnorePatterns() {
	// Create project with files that should be ignored
	testDir, err := os.MkdirTemp(suite.tempDir, "ignore-test-*")
	suite.Require().NoError(err)
	defer func() { _ = os.RemoveAll(testDir) }()

	// Create valid project files
	validFiles := map[string]string{
		"go.mod":       "module ignore-test\n\ngo 1.19",
		"main.go":      "package main\n\nfunc main() {}",
		"pkg/utils.go": "package pkg\n\nfunc Utils() {}",
	}

	// Create files that should be ignored
	ignoredFiles := map[string]string{
		"node_modules/package/index.js": "module.exports = {};",
		".git/config":                   "git config",
		"build/output.bin":              "binary data",
		"target/classes/Main.class":     "compiled java",
		"__pycache__/module.pyc":        "compiled python",
		".DS_Store":                     "mac metadata",
		"vendor/lib/library.go":         "package vendor",
	}

	// Create all files
	allFiles := make(map[string]string)
	for k, v := range validFiles {
		allFiles[k] = v
	}
	for k, v := range ignoredFiles {
		allFiles[k] = v
	}

	for filePath, content := range allFiles {
		fullPath := filepath.Join(testDir, filePath)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		suite.Require().NoError(err)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		suite.Require().NoError(err)
	}

	// Test detection
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	projCtx, err := suite.detector.DetectProject(ctx, testDir)
	suite.NoError(err, "Detection should succeed with ignored files present")
	suite.NotNil(projCtx, "Should return project context")

	suite.Equal(types.PROJECT_TYPE_GO, projCtx.ProjectType, "Should detect Go project")

	// Verify only valid marker files are included
	for _, markerFile := range projCtx.MarkerFiles {
		suite.NotContains(markerFile, "node_modules", "Should not include node_modules files")
		suite.NotContains(markerFile, ".git", "Should not include .git files")
		suite.NotContains(markerFile, "build/", "Should not include build files")
		suite.NotContains(markerFile, "target/", "Should not include target files")
		suite.NotContains(markerFile, "__pycache__", "Should not include __pycache__ files")
		suite.NotContains(markerFile, ".DS_Store", "Should not include .DS_Store files")
		suite.NotContains(markerFile, "vendor/", "Should not include vendor files")
	}
}

func (suite *DefaultProjectDetectorTestSuite) TestDefaultProjectDetector_ParallelDetection() {
	// Create multiple projects for concurrent detection
	projectPaths := make([]string, 0)
	numProjects := 5

	for i := 0; i < numProjects; i++ {
		config := &framework.ProjectGenerationConfig{
			Type:        framework.ProjectTypeSingleLanguage,
			Languages:   []string{"go"},
			Complexity:  framework.ComplexitySimple,
			Size:        framework.SizeSmall,
			BuildSystem: true,
		}

		testProject, err := suite.projectGenerator.GenerateProject(config)
		suite.Require().NoError(err)
		projectPaths = append(projectPaths, testProject.RootPath)
	}

	// Test concurrent detection
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	results := make([]*project.ProjectContext, numProjects)
	errors := make([]error, numProjects)

	for i, path := range projectPaths {
		wg.Add(1)
		go func(index int, projectPath string) {
			defer wg.Done()

			// Create a separate detector for each goroutine to test thread safety
			detector := project.NewProjectDetector()
			projCtx, err := detector.DetectProject(ctx, projectPath)

			results[index] = projCtx
			errors[index] = err
		}(i, path)
	}

	wg.Wait()

	// Verify all detections succeeded
	for i := 0; i < numProjects; i++ {
		suite.NoError(errors[i], "Concurrent detection %d should succeed", i)
		suite.NotNil(results[i], "Should return project context for detection %d", i)
		suite.Equal(types.PROJECT_TYPE_GO, results[i].ProjectType,
			"Should detect Go project for detection %d", i)
	}
}

func (suite *DefaultProjectDetectorTestSuite) TestDefaultProjectDetector_ErrorHandling() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test non-existent path
	_, err := suite.detector.DetectProject(ctx, "/non/existent/path")
	suite.Error(err, "Should error for non-existent path")

	// Test empty path
	_, err = suite.detector.DetectProject(ctx, "")
	suite.Error(err, "Should error for empty path")

	// Test file instead of directory
	tempFile, err := os.CreateTemp(suite.tempDir, "test-file-*")
	suite.Require().NoError(err)
	_ = tempFile.Close()
	defer func() { _ = os.Remove(tempFile.Name()) }()

	_, err = suite.detector.DetectProject(ctx, tempFile.Name())
	suite.Error(err, "Should error when path is a file, not directory")

	// Test permission denied scenario (if applicable on current OS)
	if runtime.GOOS != "windows" { // Skip on Windows due to different permission model
		restrictedDir, err := os.MkdirTemp(suite.tempDir, "restricted-*")
		suite.Require().NoError(err)
		defer func() { _ = os.RemoveAll(restrictedDir) }()

		// Remove read permissions
		err = os.Chmod(restrictedDir, 0000)
		suite.Require().NoError(err)

		_, _ = suite.detector.DetectProject(ctx, restrictedDir)
		// Should either error due to permissions or handle gracefully
		// Restore permissions for cleanup
		_ = os.Chmod(restrictedDir, 0755)
	}

	// Test project type detection errors
	_, err = suite.detector.DetectProjectType(ctx, "/non/existent/path")
	suite.Error(err, "Project type detection should error for non-existent path")

	// Test workspace root detection errors
	_, err = suite.detector.GetWorkspaceRoot(ctx, "/non/existent/path")
	suite.Error(err, "Workspace root detection should error for non-existent path")
}

// Performance and Benchmarking Tests

func (suite *DefaultProjectDetectorTestSuite) TestDefaultProjectDetector_PerformanceProfile() {
	// Start performance profiling
	ctx := context.Background()
	err := suite.performanceProfiler.Start(ctx)
	suite.Require().NoError(err)
	defer func() { _ = suite.performanceProfiler.Stop() }()

	// Create test project for performance testing
	config := &framework.ProjectGenerationConfig{
		Type:         framework.ProjectTypeMultiLanguage,
		Languages:    []string{"go", "python", "typescript"},
		Complexity:   framework.ComplexityComplex,
		Size:         framework.SizeLarge,
		BuildSystem:  true,
		TestFiles:    true,
		Dependencies: true,
	}

	testProject, err := suite.projectGenerator.GenerateProject(config)
	suite.Require().NoError(err)

	// Start operation profiling
	metrics, err := suite.performanceProfiler.StartOperation("project_detection")
	suite.Require().NoError(err)
	suite.Require().NotNil(metrics)

	// Perform detection
	detectionCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	startTime := time.Now()
	projCtx, err := suite.detector.DetectProject(detectionCtx, testProject.RootPath)
	detectionDuration := time.Since(startTime)

	suite.NoError(err, "Performance test detection should succeed")
	suite.NotNil(projCtx, "Should return project context")

	// End operation profiling
	finalMetrics, err := suite.performanceProfiler.EndOperation(metrics.OperationID)
	suite.NoError(err, "Should end performance profiling")
	suite.NotNil(finalMetrics, "Should return final metrics")

	// Verify performance characteristics
	suite.Less(detectionDuration, 30*time.Second,
		"Detection should complete within reasonable time")

	suite.Greater(finalMetrics.OperationDuration, time.Duration(0),
		"Should track operation duration")

	// Log performance results for analysis
	suite.T().Logf("Detection Performance Metrics:")
	suite.T().Logf("  Duration: %v", detectionDuration)
	suite.T().Logf("  Memory Allocated: %d bytes", finalMetrics.MemoryAllocated)
	suite.T().Logf("  Goroutines: %d", finalMetrics.GoroutineCount)
	suite.T().Logf("  Project Type: %s", projCtx.ProjectType)
	suite.T().Logf("  Languages: %v", projCtx.Languages)
	suite.T().Logf("  Required Servers: %v", projCtx.RequiredServers)
}

// Benchmark tests for performance measurement

func (suite *DefaultProjectDetectorTestSuite) BenchmarkSimpleGoProjectDetection() {
	// Create simple Go project
	config := &framework.ProjectGenerationConfig{
		Type:        framework.ProjectTypeSingleLanguage,
		Languages:   []string{"go"},
		Complexity:  framework.ComplexitySimple,
		Size:        framework.SizeSmall,
		BuildSystem: true,
	}

	testProject, err := suite.projectGenerator.GenerateProject(config)
	suite.Require().NoError(err)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Run benchmark
	suite.T().Run("BenchmarkSimpleGoDetection", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			detector := project.NewProjectDetector()
			_, err := detector.DetectProject(ctx, testProject.RootPath)
			assert.NoError(t, err)
		}
	})
}

func (suite *DefaultProjectDetectorTestSuite) BenchmarkComplexMultiLanguageDetection() {
	// Create complex multi-language project
	config := &framework.ProjectGenerationConfig{
		Type:         framework.ProjectTypeMultiLanguage,
		Languages:    []string{"go", "python", "typescript", "java"},
		Complexity:   framework.ComplexityComplex,
		Size:         framework.SizeLarge,
		BuildSystem:  true,
		TestFiles:    true,
		Dependencies: true,
	}

	testProject, err := suite.projectGenerator.GenerateProject(config)
	suite.Require().NoError(err)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Run benchmark
	suite.T().Run("BenchmarkComplexMultiLanguageDetection", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			detector := project.NewProjectDetector()
			_, err := detector.DetectProject(ctx, testProject.RootPath)
			assert.NoError(t, err)
		}
	})
}

// Test Suite Runner

func TestDefaultProjectDetectorTestSuite(t *testing.T) {
	suite.Run(t, new(DefaultProjectDetectorTestSuite))
}

// Individual unit test functions for specific edge cases

func TestDefaultProjectDetector_ContextCancellation(t *testing.T) {
	// Create test project
	tempDir, err := os.MkdirTemp("", "context-cancel-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create simple Go project
	goMod := filepath.Join(tempDir, "go.mod")
	err = os.WriteFile(goMod, []byte("module test\n\ngo 1.19"), 0644)
	require.NoError(t, err)

	mainGo := filepath.Join(tempDir, "main.go")
	err = os.WriteFile(mainGo, []byte("package main\n\nfunc main() {}"), 0644)
	require.NoError(t, err)

	// Test context cancellation
	detector := project.NewProjectDetector()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = detector.DetectProject(ctx, tempDir)
	assert.Error(t, err, "Should error when context is cancelled")
	assert.Contains(t, err.Error(), "context canceled", "Should indicate context cancellation")
}

func TestDefaultProjectDetector_SymlinkHandling(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping symlink test on Windows")
	}

	// Create test directories
	tempDir, err := os.MkdirTemp("", "symlink-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	realProjectDir := filepath.Join(tempDir, "real-project")
	err = os.MkdirAll(realProjectDir, 0755)
	require.NoError(t, err)

	// Create real project
	goMod := filepath.Join(realProjectDir, "go.mod")
	err = os.WriteFile(goMod, []byte("module symlink-test\n\ngo 1.19"), 0644)
	require.NoError(t, err)

	mainGo := filepath.Join(realProjectDir, "main.go")
	err = os.WriteFile(mainGo, []byte("package main\n\nfunc main() {}"), 0644)
	require.NoError(t, err)

	// Create symlink to project
	symlinkPath := filepath.Join(tempDir, "symlink-project")
	err = os.Symlink(realProjectDir, symlinkPath)
	require.NoError(t, err)

	// Test detection through symlink
	detector := project.NewProjectDetector()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	projCtx, err := detector.DetectProject(ctx, symlinkPath)
	assert.NoError(t, err, "Should handle symlinks correctly")
	assert.NotNil(t, projCtx, "Should detect project through symlink")
	assert.Equal(t, types.PROJECT_TYPE_GO, projCtx.ProjectType, "Should detect Go project")
}

func TestDefaultProjectDetector_EmptyDirectory(t *testing.T) {
	// Create empty directory
	tempDir, err := os.MkdirTemp("", "empty-dir-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Test detection on empty directory
	detector := project.NewProjectDetector()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	projCtx, err := detector.DetectProject(ctx, tempDir)

	// Should either return unknown project type or error
	if err == nil {
		assert.Equal(t, types.PROJECT_TYPE_UNKNOWN, projCtx.ProjectType,
			"Empty directory should be detected as unknown project type")
	} else {
		assert.Contains(t, err.Error(), "no project",
			"Should indicate no project found in empty directory")
	}
}

func TestDefaultProjectDetector_LargeDirectoryStructure(t *testing.T) {
	// Create directory with many files
	tempDir, err := os.MkdirTemp("", "large-dir-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create base project files
	goMod := filepath.Join(tempDir, "go.mod")
	err = os.WriteFile(goMod, []byte("module large-test\n\ngo 1.19"), 0644)
	require.NoError(t, err)

	// Create many directories and files
	for i := 0; i < 100; i++ {
		dirPath := filepath.Join(tempDir, fmt.Sprintf("dir%d", i))
		err = os.MkdirAll(dirPath, 0755)
		require.NoError(t, err)

		for j := 0; j < 10; j++ {
			filePath := filepath.Join(dirPath, fmt.Sprintf("file%d.go", j))
			content := fmt.Sprintf("package dir%d\n\nfunc Function%d() {}", i, j)
			err = os.WriteFile(filePath, []byte(content), 0644)
			require.NoError(t, err)
		}
	}

	// Test detection with large directory structure
	detector := project.NewProjectDetector()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	startTime := time.Now()
	projCtx, err := detector.DetectProject(ctx, tempDir)
	duration := time.Since(startTime)

	assert.NoError(t, err, "Should handle large directory structure")
	assert.NotNil(t, projCtx, "Should detect project in large directory")
	assert.Equal(t, types.PROJECT_TYPE_GO, projCtx.ProjectType, "Should detect Go project")
	assert.Less(t, duration, 30*time.Second, "Should complete within reasonable time")

	t.Logf("Large directory detection took: %v", duration)
}
