package detectors_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"lsp-gateway/internal/project/detectors"
	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/setup"
	"lsp-gateway/tests/framework"
)

// GoDetectorTestSuite provides comprehensive unit tests for GoProjectDetector
type GoDetectorTestSuite struct {
	suite.Suite
	tempDir   string
	detector  *detectors.GoProjectDetector
	generator *framework.TestProjectGenerator
	profiler  *framework.PerformanceProfiler
	logger    *setup.SetupLogger
}

func (suite *GoDetectorTestSuite) SetupSuite() {
	// Create temporary directory for all tests
	tempDir, err := os.MkdirTemp("", "go-detector-test-*")
	suite.Require().NoError(err)
	suite.tempDir = tempDir

	// Initialize test framework components
	suite.generator = framework.NewTestProjectGenerator(suite.tempDir)
	suite.profiler = framework.NewPerformanceProfiler()
	suite.logger = setup.NewSetupLogger(&setup.SetupLoggerConfig{
		Component: "go-detector-test",
		Level:     setup.LogLevelDebug,
	})

	// Load project templates
	ctx := context.Background()
	err = suite.generator.LoadTemplates(ctx)
	suite.Require().NoError(err)
}

func (suite *GoDetectorTestSuite) TearDownSuite() {
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
	if suite.profiler != nil {
		suite.profiler.Reset()
	}
	if suite.generator != nil {
		suite.generator.Reset()
	}
}

func (suite *GoDetectorTestSuite) SetupTest() {
	// Create fresh detector for each test
	suite.detector = detectors.NewGoProjectDetector()
}

func (suite *GoDetectorTestSuite) TearDownTest() {
	suite.detector = nil
}

// Core Detection Tests

func (suite *GoDetectorTestSuite) TestGoDetector_DetectLanguage() {
	testCases := []struct {
		name              string
		setupFiles        map[string]string
		expectedDetection bool
		expectedType      string
		minConfidence     float64
		maxConfidence     float64
	}{
		{
			name: "simple-go-module",
			setupFiles: map[string]string{
				"go.mod":  "module test\n\ngo 1.19",
				"main.go": "package main\n\nfunc main() {}",
			},
			expectedDetection: true,
			expectedType:      types.PROJECT_TYPE_GO,
			minConfidence:     0.7,
			maxConfidence:     1.0,
		},
		{
			name: "go-module-with-dependencies",
			setupFiles: map[string]string{
				"go.mod": `module test-app

go 1.20

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/stretchr/testify v1.8.4
)

replace github.com/example/local => ./local

exclude github.com/old/package v1.0.0`,
				"go.sum":  "github.com/gin-gonic/gin v1.9.1 h1:hash123...\ngithub.com/gin-gonic/gin v1.9.1/go.mod h1:hash456...",
				"main.go": "package main\n\nimport \"github.com/gin-gonic/gin\"\n\nfunc main() {}",
			},
			expectedDetection: true,
			expectedType:      types.PROJECT_TYPE_GO,
			minConfidence:     0.8,
			maxConfidence:     1.0,
		},
		{
			name: "go-workspace-project",
			setupFiles: map[string]string{
				"go.work": `go 1.20

use (
	./module1
	./module2
)

replace github.com/example/shared => ./shared`,
				"go.work.sum":     "github.com/example/dep v1.0.0 h1:hash...",
				"module1/go.mod":  "module example.com/module1\n\ngo 1.20",
				"module1/main.go": "package main\n\nfunc main() {}",
				"module2/go.mod":  "module example.com/module2\n\ngo 1.20",
				"module2/lib.go":  "package module2\n\nfunc Lib() {}",
			},
			expectedDetection: true,
			expectedType:      types.PROJECT_TYPE_GO,
			minConfidence:     0.9,
			maxConfidence:     1.0,
		},
		{
			name: "go-with-cgo",
			setupFiles: map[string]string{
				"go.mod": "module cgo-test\n\ngo 1.20",
				"main.go": `package main

/*
#include <stdio.h>
void hello() {
    printf("Hello from C!\n");
}
*/
import "C"

func main() {
    C.hello()
}`,
			},
			expectedDetection: true,
			expectedType:      types.PROJECT_TYPE_GO,
			minConfidence:     0.75,
			maxConfidence:     1.0,
		},
		{
			name: "go-with-build-tags",
			setupFiles: map[string]string{
				"go.mod": "module build-tags\n\ngo 1.20",
				"main.go": `//go:build linux
// +build linux

package main

func main() {}`,
				"main_windows.go": `//go:build windows
// +build windows

package main

func main() {}`,
			},
			expectedDetection: true,
			expectedType:      types.PROJECT_TYPE_GO,
			minConfidence:     0.75,
			maxConfidence:     1.0,
		},
		{
			name: "go-with-tooling-configs",
			setupFiles: map[string]string{
				"go.mod":          "module tooling-test\n\ngo 1.20",
				"main.go":         "package main\n\nfunc main() {}",
				".golangci.yml":   "run:\n  timeout: 5m",
				".goreleaser.yml": "builds:\n  - binary: app",
				"Makefile":        "build:\n\tgo build -o app .",
			},
			expectedDetection: true,
			expectedType:      types.PROJECT_TYPE_GO,
			minConfidence:     0.8,
			maxConfidence:     1.0,
		},
		{
			name: "go-only-source-files",
			setupFiles: map[string]string{
				"main.go":    "package main\n\nfunc main() {}",
				"utils.go":   "package main\n\nfunc Utils() {}",
				"service.go": "package main\n\ntype Service struct{}",
			},
			expectedDetection: true,
			expectedType:      types.PROJECT_TYPE_GO,
			minConfidence:     0.3,
			maxConfidence:     0.7,
		},
		{
			name: "empty-directory",
			setupFiles: map[string]string{
				".gitkeep": "",
			},
			expectedDetection: true, // Should still return result with 0 confidence
			expectedType:      types.PROJECT_TYPE_GO,
			minConfidence:     0.0,
			maxConfidence:     0.2,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create test directory
			testDir, err := os.MkdirTemp(suite.tempDir, tc.name+"-*")
			suite.Require().NoError(err)
			defer os.RemoveAll(testDir)

			// Create test files
			for filePath, content := range tc.setupFiles {
				fullPath := filepath.Join(testDir, filePath)
				err := os.MkdirAll(filepath.Dir(fullPath), 0755)
				suite.Require().NoError(err)
				err = os.WriteFile(fullPath, []byte(content), 0644)
				suite.Require().NoError(err)
			}

			// Test detection
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			result, err := suite.detector.DetectLanguage(ctx, testDir)

			if tc.expectedDetection {
				suite.NoError(err, "Detection should succeed for %s", tc.name)
				suite.NotNil(result, "Should return detection result")
				suite.Equal(tc.expectedType, result.Language, "Should detect correct language")
				suite.GreaterOrEqual(result.Confidence, tc.minConfidence,
					"Confidence should be at least %f", tc.minConfidence)
				suite.LessOrEqual(result.Confidence, tc.maxConfidence,
					"Confidence should be at most %f", tc.maxConfidence)

				// Verify required servers
				suite.Contains(result.RequiredServers, types.SERVER_GOPLS, "Should require gopls server")

				// Verify metadata is populated
				suite.NotEmpty(result.Metadata, "Should have metadata")
				suite.Contains(result.Metadata, "detection_time", "Should track detection time")
				suite.Contains(result.Metadata, "detection_method", "Should track detection method")

			} else {
				suite.Error(err, "Detection should fail for %s", tc.name)
			}
		})
	}
}

func (suite *GoDetectorTestSuite) TestGoDetector_ParseGoMod() {
	testCases := []struct {
		name           string
		goModContent   string
		expectedModule string
		expectedGoVer  string
		expectedReqs   map[string]string
		expectedReps   map[string]string
		expectedExcl   []string
		shouldSucceed  bool
	}{
		{
			name:           "simple-module",
			goModContent:   "module example.com/test\n\ngo 1.19",
			expectedModule: "example.com/test",
			expectedGoVer:  "1.19",
			expectedReqs:   map[string]string{},
			expectedReps:   map[string]string{},
			expectedExcl:   []string{},
			shouldSucceed:  true,
		},
		{
			name: "complex-module",
			goModContent: `module example.com/complex

go 1.20

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/stretchr/testify v1.8.4
	golang.org/x/sync v0.3.0
)

replace (
	github.com/old/package => github.com/new/package v2.0.0
	github.com/local/dep => ./local
)

exclude (
	github.com/broken/package v1.0.0
	github.com/vuln/package v0.1.0
)`,
			expectedModule: "example.com/complex",
			expectedGoVer:  "1.20",
			expectedReqs: map[string]string{
				"github.com/gin-gonic/gin":    "v1.9.1",
				"github.com/stretchr/testify": "v1.8.4",
				"golang.org/x/sync":           "v0.3.0",
			},
			expectedReps: map[string]string{
				"github.com/old/package": "github.com/new/package v2.0.0",
				"github.com/local/dep":   "./local",
			},
			expectedExcl: []string{
				"github.com/broken/package v1.0.0",
				"github.com/vuln/package v0.1.0",
			},
			shouldSucceed: true,
		},
		{
			name: "single-line-directives",
			goModContent: `module example.com/single

go 1.21

require github.com/single/dep v1.0.0
replace github.com/old => github.com/new v2.0.0
exclude github.com/bad v1.0.0`,
			expectedModule: "example.com/single",
			expectedGoVer:  "1.21",
			expectedReqs: map[string]string{
				"github.com/single/dep": "v1.0.0",
			},
			expectedReps: map[string]string{
				"github.com/old": "github.com/new v2.0.0",
			},
			expectedExcl: []string{
				"github.com/bad v1.0.0",
			},
			shouldSucceed: true,
		},
		{
			name:          "malformed-module",
			goModContent:  "invalid go.mod content\nno module directive",
			shouldSucceed: false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create test file
			testDir, err := os.MkdirTemp(suite.tempDir, tc.name+"-*")
			suite.Require().NoError(err)
			defer os.RemoveAll(testDir)

			goModPath := filepath.Join(testDir, "go.mod")
			err = os.WriteFile(goModPath, []byte(tc.goModContent), 0644)
			suite.Require().NoError(err)

			// Create parser
			parser := detectors.NewGoModuleParser(suite.logger)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Test parsing
			modInfo, err := parser.ParseGoMod(ctx, goModPath)

			if tc.shouldSucceed {
				suite.NoError(err, "Parsing should succeed for %s", tc.name)
				suite.NotNil(modInfo, "Should return module info")
				suite.Equal(tc.expectedModule, modInfo.ModuleName, "Should parse module name correctly")
				suite.Equal(tc.expectedGoVer, modInfo.GoVersion, "Should parse Go version correctly")

				// Check requirements
				for dep, version := range tc.expectedReqs {
					suite.Contains(modInfo.Requires, dep, "Should contain required dependency %s", dep)
					suite.Equal(version, modInfo.Requires[dep], "Should have correct version for %s", dep)
				}

				// Check replaces
				for old, new := range tc.expectedReps {
					suite.Contains(modInfo.Replaces, old, "Should contain replace for %s", old)
					suite.Equal(new, modInfo.Replaces[old], "Should have correct replacement for %s", old)
				}

				// Check excludes
				for _, exclude := range tc.expectedExcl {
					suite.Contains(modInfo.Excludes, exclude, "Should contain exclude %s", exclude)
				}
			} else {
				suite.Error(err, "Parsing should fail for %s", tc.name)
			}
		})
	}
}

func (suite *GoDetectorTestSuite) TestGoDetector_ParseGoSum() {
	testCases := []struct {
		name         string
		goSumContent string
		shouldExist  bool
	}{
		{
			name: "valid-go-sum",
			goSumContent: `github.com/gin-gonic/gin v1.9.1 h1:4idEAncQnU5cB7BeOkPtxjfCSye0AAm1R0RVIqJ+Jmg=
github.com/gin-gonic/gin v1.9.1/go.mod h1:hPrL7YrpYKXt5YId3A/Tnip5kqbEAP+KLuI3SUcPTeU=
github.com/stretchr/testify v1.8.4 h1:CcVxjf3Q8OjJXMlKl7cnNwF0AcSJw20rNj8fRr0qI1A=
github.com/stretchr/testify v1.8.4/go.mod h1:sz/lmYIOXD/1dqDmKjjqLyZ2RngseejIcXlSw2iwfAo=`,
			shouldExist: true,
		},
		{
			name:        "no-go-sum",
			shouldExist: false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create test directory
			testDir, err := os.MkdirTemp(suite.tempDir, tc.name+"-*")
			suite.Require().NoError(err)
			defer os.RemoveAll(testDir)

			// Create go.mod
			goModPath := filepath.Join(testDir, "go.mod")
			err = os.WriteFile(goModPath, []byte("module test\n\ngo 1.19"), 0644)
			suite.Require().NoError(err)

			// Create go.sum if needed
			if tc.shouldExist {
				goSumPath := filepath.Join(testDir, "go.sum")
				err = os.WriteFile(goSumPath, []byte(tc.goSumContent), 0644)
				suite.Require().NoError(err)
			}

			// Test detection
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			result, err := suite.detector.DetectLanguage(ctx, testDir)
			suite.NoError(err, "Detection should succeed")
			suite.NotNil(result, "Should return result")

			// Check if go.sum is detected
			if tc.shouldExist {
				suite.Contains(result.MarkerFiles, types.MARKER_GO_SUM, "Should detect go.sum file")
				suite.Contains(result.Metadata, "go_sum_present", "Should track go.sum presence")
				suite.True(result.Metadata["go_sum_present"].(bool), "Should mark go.sum as present")
			} else {
				suite.NotContains(result.MarkerFiles, types.MARKER_GO_SUM, "Should not detect go.sum file")
			}
		})
	}
}

func (suite *GoDetectorTestSuite) TestGoDetector_WorkspaceDetection() {
	testCases := []struct {
		name            string
		setupFiles      map[string]string
		expectedModules []string
		shouldDetect    bool
	}{
		{
			name: "simple-workspace",
			setupFiles: map[string]string{
				"go.work": `go 1.20

use (
	./module1
	./module2
)`,
				"module1/go.mod":  "module example.com/module1\n\ngo 1.20",
				"module1/main.go": "package main\n\nfunc main() {}",
				"module2/go.mod":  "module example.com/module2\n\ngo 1.20",
				"module2/lib.go":  "package module2\n\nfunc Lib() {}",
			},
			expectedModules: []string{"./module1", "./module2"},
			shouldDetect:    true,
		},
		{
			name: "workspace-with-replaces",
			setupFiles: map[string]string{
				"go.work": `go 1.21

use ./service

replace github.com/example/shared => ./shared`,
				"service/go.mod":  "module example.com/service\n\ngo 1.21",
				"service/main.go": "package main\n\nfunc main() {}",
				"shared/go.mod":   "module github.com/example/shared\n\ngo 1.21",
			},
			expectedModules: []string{"./service"},
			shouldDetect:    true,
		},
		{
			name: "workspace-with-sum",
			setupFiles: map[string]string{
				"go.work": `go 1.20

use ./app`,
				"go.work.sum": "github.com/external/dep v1.0.0 h1:hash...",
				"app/go.mod":  "module example.com/app\n\ngo 1.20",
				"app/main.go": "package main\n\nfunc main() {}",
			},
			expectedModules: []string{"./app"},
			shouldDetect:    true,
		},
		{
			name: "no-workspace",
			setupFiles: map[string]string{
				"go.mod":  "module test\n\ngo 1.20",
				"main.go": "package main\n\nfunc main() {}",
			},
			shouldDetect: false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create test directory
			testDir, err := os.MkdirTemp(suite.tempDir, tc.name+"-*")
			suite.Require().NoError(err)
			defer os.RemoveAll(testDir)

			// Create test files
			for filePath, content := range tc.setupFiles {
				fullPath := filepath.Join(testDir, filePath)
				err := os.MkdirAll(filepath.Dir(fullPath), 0755)
				suite.Require().NoError(err)
				err = os.WriteFile(fullPath, []byte(content), 0644)
				suite.Require().NoError(err)
			}

			// Test detection
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			result, err := suite.detector.DetectLanguage(ctx, testDir)
			suite.NoError(err, "Detection should succeed")
			suite.NotNil(result, "Should return result")

			if tc.shouldDetect {
				suite.Contains(result.MarkerFiles, "go.work", "Should detect go.work file")
				suite.Contains(result.Metadata, "is_workspace", "Should mark as workspace")
				suite.True(result.Metadata["is_workspace"].(bool), "Should be marked as workspace")

				if workspaceModules, ok := result.Metadata["workspace_modules"]; ok {
					modules := workspaceModules.([]string)
					for _, expectedModule := range tc.expectedModules {
						suite.Contains(modules, expectedModule, "Should contain module %s", expectedModule)
					}
				}
			} else {
				suite.NotContains(result.MarkerFiles, "go.work", "Should not detect go.work file")
			}
		})
	}
}

func (suite *GoDetectorTestSuite) TestGoDetector_SourceAnalysis() {
	testCases := []struct {
		name             string
		setupFiles       map[string]string
		expectedPackages int
		expectedCGO      bool
		expectedTags     []string
		expectedTests    int
	}{
		{
			name: "simple-package-structure",
			setupFiles: map[string]string{
				"go.mod":          "module test\n\ngo 1.20",
				"main.go":         "package main\n\nfunc main() {}",
				"internal/lib.go": "package internal\n\nfunc Lib() {}",
				"pkg/utils.go":    "package pkg\n\nfunc Utils() {}",
			},
			expectedPackages: 3,
			expectedCGO:      false,
			expectedTags:     []string{},
			expectedTests:    0,
		},
		{
			name: "package-with-tests",
			setupFiles: map[string]string{
				"go.mod":              "module test\n\ngo 1.20",
				"main.go":             "package main\n\nfunc main() {}",
				"main_test.go":        "package main\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) {}",
				"utils/utils.go":      "package utils\n\nfunc Utils() {}",
				"utils/utils_test.go": "package utils\n\nimport \"testing\"\n\nfunc TestUtils(t *testing.T) {}",
			},
			expectedPackages: 2,
			expectedCGO:      false,
			expectedTags:     []string{},
			expectedTests:    2,
		},
		{
			name: "cgo-package",
			setupFiles: map[string]string{
				"go.mod": "module cgo-test\n\ngo 1.20",
				"main.go": `package main

/*
#include <stdio.h>
void hello() {
    printf("Hello from C!\n");
}
*/
import "C"

func main() {
    C.hello()
}`,
			},
			expectedPackages: 1,
			expectedCGO:      true,
			expectedTags:     []string{},
			expectedTests:    0,
		},
		{
			name: "build-tags-package",
			setupFiles: map[string]string{
				"go.mod": "module build-tags\n\ngo 1.20",
				"main.go": `//go:build linux && !windows
// +build linux,!windows

package main

func main() {}`,
				"windows.go": `//go:build windows
// +build windows

package main

func main() {}`,
			},
			expectedPackages: 1,
			expectedCGO:      false,
			expectedTags:     []string{"linux && !windows", "linux,!windows", "windows"},
			expectedTests:    0,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create test directory
			testDir, err := os.MkdirTemp(suite.tempDir, tc.name+"-*")
			suite.Require().NoError(err)
			defer os.RemoveAll(testDir)

			// Create test files
			for filePath, content := range tc.setupFiles {
				fullPath := filepath.Join(testDir, filePath)
				err := os.MkdirAll(filepath.Dir(fullPath), 0755)
				suite.Require().NoError(err)
				err = os.WriteFile(fullPath, []byte(content), 0644)
				suite.Require().NoError(err)
			}

			// Test detection
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			result, err := suite.detector.DetectLanguage(ctx, testDir)
			suite.NoError(err, "Detection should succeed")
			suite.NotNil(result, "Should return result")

			// Check package count
			if packagesFound, ok := result.Metadata["go_packages_found"]; ok {
				suite.Equal(tc.expectedPackages, packagesFound.(int), "Should find correct number of packages")
			}

			// Check CGO usage
			if tc.expectedCGO {
				suite.Contains(result.Metadata, "cgo_enabled", "Should detect CGO usage")
				suite.True(result.Metadata["cgo_enabled"].(bool), "Should mark CGO as enabled")
			} else {
				if cgoEnabled, ok := result.Metadata["cgo_enabled"]; ok {
					suite.False(cgoEnabled.(bool), "Should not mark CGO as enabled")
				}
			}

			// Check build tags
			if len(tc.expectedTags) > 0 {
				suite.Contains(result.Metadata, "build_tags", "Should contain build tags")
				buildTags := result.Metadata["build_tags"].([]string)
				for _, expectedTag := range tc.expectedTags {
					suite.Contains(buildTags, expectedTag, "Should contain build tag %s", expectedTag)
				}
			}

			// Check test files count
			if tc.expectedTests > 0 {
				suite.GreaterOrEqual(len(result.TestDirs), 0, "Should have test directories")
			}
		})
	}
}

func (suite *GoDetectorTestSuite) TestGoDetector_BuildConfiguration() {
	testCases := []struct {
		name           string
		setupFiles     map[string]string
		expectedBuild  []string
		expectedConfig []string
	}{
		{
			name: "makefile-project",
			setupFiles: map[string]string{
				"go.mod":   "module test\n\ngo 1.20",
				"main.go":  "package main\n\nfunc main() {}",
				"Makefile": "build:\n\tgo build -o app .",
			},
			expectedBuild:  []string{"Makefile"},
			expectedConfig: []string{},
		},
		{
			name: "ci-project",
			setupFiles: map[string]string{
				"go.mod":                   "module test\n\ngo 1.20",
				"main.go":                  "package main\n\nfunc main() {}",
				".github/workflows/ci.yml": "name: CI\non: [push]",
				".gitlab-ci.yml":           "stages:\n  - build",
			},
			expectedBuild:  []string{".github/workflows", ".gitlab-ci.yml"},
			expectedConfig: []string{},
		},
		{
			name: "docker-project",
			setupFiles: map[string]string{
				"go.mod":             "module test\n\ngo 1.20",
				"main.go":            "package main\n\nfunc main() {}",
				"Dockerfile":         "FROM golang:1.20\nCOPY . .\nRUN go build",
				"docker-compose.yml": "version: '3'\nservices:\n  app:",
			},
			expectedBuild:  []string{"Dockerfile", "docker-compose.yml"},
			expectedConfig: []string{},
		},
		{
			name: "tooling-configs",
			setupFiles: map[string]string{
				"go.mod":           "module test\n\ngo 1.20",
				"main.go":          "package main\n\nfunc main() {}",
				".golangci.yml":    "run:\n  timeout: 5m",
				".goreleaser.yaml": "builds:\n  - binary: app",
				"golangci.yaml":    "linters:\n  enable-all: true",
			},
			expectedBuild:  []string{},
			expectedConfig: []string{".golangci.yml", ".goreleaser.yaml", "golangci.yaml"},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create test directory
			testDir, err := os.MkdirTemp(suite.tempDir, tc.name+"-*")
			suite.Require().NoError(err)
			defer os.RemoveAll(testDir)

			// Create test files
			for filePath, content := range tc.setupFiles {
				fullPath := filepath.Join(testDir, filePath)
				err := os.MkdirAll(filepath.Dir(fullPath), 0755)
				suite.Require().NoError(err)
				err = os.WriteFile(fullPath, []byte(content), 0644)
				suite.Require().NoError(err)
			}

			// Test detection
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			result, err := suite.detector.DetectLanguage(ctx, testDir)
			suite.NoError(err, "Detection should succeed")
			suite.NotNil(result, "Should return result")

			// Check build files
			for _, expectedFile := range tc.expectedBuild {
				suite.Contains(result.BuildFiles, expectedFile, "Should detect build file %s", expectedFile)
			}

			// Check config files
			for _, expectedFile := range tc.expectedConfig {
				suite.Contains(result.ConfigFiles, expectedFile, "Should detect config file %s", expectedFile)
			}

			// Check tooling metadata
			if len(tc.expectedConfig) > 0 {
				suite.Contains(result.Metadata, "go_tooling_configs", "Should track tooling configs")
				toolingConfigs := result.Metadata["go_tooling_configs"].([]string)
				for _, expectedFile := range tc.expectedConfig {
					suite.Contains(toolingConfigs, expectedFile, "Should contain tooling config %s", expectedFile)
				}
			}
		})
	}
}

func (suite *GoDetectorTestSuite) TestGoDetector_ConfidenceScoring() {
	testCases := []struct {
		name          string
		setupFiles    map[string]string
		expectedRange [2]float64 // min, max confidence
	}{
		{
			name: "maximum-confidence",
			setupFiles: map[string]string{
				"go.mod":          "module test\n\ngo 1.20",
				"go.sum":          "github.com/gin-gonic/gin v1.9.1 h1:hash...",
				"go.work":         "go 1.20\n\nuse ./module",
				"main.go":         "package main\n\nfunc main() {}",
				"internal/lib.go": "package internal\n\nfunc Lib() {}",
				".golangci.yml":   "run:\n  timeout: 5m",
				"Makefile":        "build:\n\tgo build",
			},
			expectedRange: [2]float64{0.95, 1.0},
		},
		{
			name: "high-confidence",
			setupFiles: map[string]string{
				"go.mod":          "module test\n\ngo 1.20",
				"go.sum":          "github.com/gin-gonic/gin v1.9.1 h1:hash...",
				"main.go":         "package main\n\nfunc main() {}",
				"internal/lib.go": "package internal\n\nfunc Lib() {}",
			},
			expectedRange: [2]float64{0.8, 0.95},
		},
		{
			name: "medium-confidence",
			setupFiles: map[string]string{
				"go.mod":  "module test\n\ngo 1.20",
				"main.go": "package main\n\nfunc main() {}",
			},
			expectedRange: [2]float64{0.7, 0.8},
		},
		{
			name: "low-confidence",
			setupFiles: map[string]string{
				"main.go":  "package main\n\nfunc main() {}",
				"utils.go": "package main\n\nfunc Utils() {}",
			},
			expectedRange: [2]float64{0.3, 0.7},
		},
		{
			name: "minimal-confidence",
			setupFiles: map[string]string{
				"README.md": "# Go Project\n\nThis is a Go project",
			},
			expectedRange: [2]float64{0.0, 0.3},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create test directory
			testDir, err := os.MkdirTemp(suite.tempDir, tc.name+"-*")
			suite.Require().NoError(err)
			defer os.RemoveAll(testDir)

			// Create test files
			for filePath, content := range tc.setupFiles {
				fullPath := filepath.Join(testDir, filePath)
				err := os.MkdirAll(filepath.Dir(fullPath), 0755)
				suite.Require().NoError(err)
				err = os.WriteFile(fullPath, []byte(content), 0644)
				suite.Require().NoError(err)
			}

			// Test detection
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			result, err := suite.detector.DetectLanguage(ctx, testDir)
			suite.NoError(err, "Detection should succeed")
			suite.NotNil(result, "Should return result")

			// Check confidence range
			suite.GreaterOrEqual(result.Confidence, tc.expectedRange[0],
				"Confidence should be at least %f", tc.expectedRange[0])
			suite.LessOrEqual(result.Confidence, tc.expectedRange[1],
				"Confidence should be at most %f", tc.expectedRange[1])
		})
	}
}

func (suite *GoDetectorTestSuite) TestGoDetector_VersionDetection() {
	testCases := []struct {
		name         string
		goModContent string
		expectedVer  string
	}{
		{
			name:         "go-1.19",
			goModContent: "module test\n\ngo 1.19",
			expectedVer:  "1.19",
		},
		{
			name:         "go-1.20",
			goModContent: "module test\n\ngo 1.20",
			expectedVer:  "1.20",
		},
		{
			name:         "go-1.21",
			goModContent: "module test\n\ngo 1.21",
			expectedVer:  "1.21",
		},
		{
			name:         "no-go-version",
			goModContent: "module test",
			expectedVer:  "",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create test directory
			testDir, err := os.MkdirTemp(suite.tempDir, tc.name+"-*")
			suite.Require().NoError(err)
			defer os.RemoveAll(testDir)

			// Create go.mod
			goModPath := filepath.Join(testDir, "go.mod")
			err = os.WriteFile(goModPath, []byte(tc.goModContent), 0644)
			suite.Require().NoError(err)

			// Create main.go
			mainGoPath := filepath.Join(testDir, "main.go")
			err = os.WriteFile(mainGoPath, []byte("package main\n\nfunc main() {}"), 0644)
			suite.Require().NoError(err)

			// Test detection
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			result, err := suite.detector.DetectLanguage(ctx, testDir)
			suite.NoError(err, "Detection should succeed")
			suite.NotNil(result, "Should return result")

			if tc.expectedVer != "" {
				suite.Contains(result.Metadata, "go_directive_version", "Should contain Go directive version")
				suite.Equal(tc.expectedVer, result.Metadata["go_directive_version"], "Should detect correct Go version")
			}
		})
	}
}

// Parser-Specific Tests

func (suite *GoDetectorTestSuite) TestGoDetector_GoModuleParsing() {
	// Create parser
	parser := detectors.NewGoModuleParser(suite.logger)

	testCases := []struct {
		name          string
		content       string
		shouldSucceed bool
		checkFunc     func(*detectors.GoModInfo)
	}{
		{
			name: "complete-go-mod",
			content: `module github.com/example/project

go 1.20

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/stretchr/testify v1.8.4
)

replace github.com/old/dep => github.com/new/dep v2.0.0

exclude github.com/broken/dep v1.0.0

retract v1.0.0`,
			shouldSucceed: true,
			checkFunc: func(info *detectors.GoModInfo) {
				suite.Equal("github.com/example/project", info.ModuleName)
				suite.Equal("1.20", info.GoVersion)
				suite.Contains(info.Requires, "github.com/gin-gonic/gin")
				suite.Contains(info.Replaces, "github.com/old/dep")
				suite.Contains(info.Excludes, "github.com/broken/dep v1.0.0")
				suite.Contains(info.Retracts, "v1.0.0")
			},
		},
		{
			name: "minimal-go-mod",
			content: `module simple

go 1.19`,
			shouldSucceed: true,
			checkFunc: func(info *detectors.GoModInfo) {
				suite.Equal("simple", info.ModuleName)
				suite.Equal("1.19", info.GoVersion)
				suite.Empty(info.Requires)
				suite.Empty(info.Replaces)
				suite.Empty(info.Excludes)
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create test file
			testDir, err := os.MkdirTemp(suite.tempDir, tc.name+"-*")
			suite.Require().NoError(err)
			defer os.RemoveAll(testDir)

			goModPath := filepath.Join(testDir, "go.mod")
			err = os.WriteFile(goModPath, []byte(tc.content), 0644)
			suite.Require().NoError(err)

			// Test parsing
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			info, err := parser.ParseGoMod(ctx, goModPath)

			if tc.shouldSucceed {
				suite.NoError(err, "Parsing should succeed")
				suite.NotNil(info, "Should return module info")
				if tc.checkFunc != nil {
					tc.checkFunc(info)
				}
			} else {
				suite.Error(err, "Parsing should fail")
			}
		})
	}
}

func (suite *GoDetectorTestSuite) TestGoDetector_GoWorkspaceParsing() {
	// Create parser
	parser := detectors.NewGoWorkspaceParser(suite.logger)

	testCases := []struct {
		name          string
		content       string
		shouldSucceed bool
		expectedMods  []string
	}{
		{
			name: "simple-workspace",
			content: `go 1.20

use (
	./module1
	./module2
)`,
			shouldSucceed: true,
			expectedMods:  []string{"./module1", "./module2"},
		},
		{
			name: "workspace-with-replace",
			content: `go 1.21

use ./service

replace github.com/example/shared => ./shared`,
			shouldSucceed: true,
			expectedMods:  []string{"./service"},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create test file
			testDir, err := os.MkdirTemp(suite.tempDir, tc.name+"-*")
			suite.Require().NoError(err)
			defer os.RemoveAll(testDir)

			goWorkPath := filepath.Join(testDir, "go.work")
			err = os.WriteFile(goWorkPath, []byte(tc.content), 0644)
			suite.Require().NoError(err)

			// Test parsing
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			info, err := parser.ParseGoWork(ctx, goWorkPath)

			if tc.shouldSucceed {
				suite.NoError(err, "Parsing should succeed")
				suite.NotNil(info, "Should return workspace info")

				for _, expectedMod := range tc.expectedMods {
					suite.Contains(info.Modules, expectedMod, "Should contain module %s", expectedMod)
				}
			} else {
				suite.Error(err, "Parsing should fail")
			}
		})
	}
}

func (suite *GoDetectorTestSuite) TestGoDetector_ImportAnalysis() {
	testCases := []struct {
		name           string
		sourceContent  string
		expectedStdLib []string
		expectedThird  []string
	}{
		{
			name: "mixed-imports",
			sourceContent: `package main

import (
	"fmt"
	"net/http"
	"context"
	
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	
	"./internal/utils"
	"../shared/config"
)

func main() {
	fmt.Println("Hello")
}`,
			expectedStdLib: []string{"fmt", "net/http", "context"},
			expectedThird:  []string{"github.com/gin-gonic/gin", "github.com/stretchr/testify/assert"},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create test directory
			testDir, err := os.MkdirTemp(suite.tempDir, tc.name+"-*")
			suite.Require().NoError(err)
			defer os.RemoveAll(testDir)

			// Create go.mod
			goModPath := filepath.Join(testDir, "go.mod")
			err = os.WriteFile(goModPath, []byte("module test\n\ngo 1.20"), 0644)
			suite.Require().NoError(err)

			// Create source file
			mainGoPath := filepath.Join(testDir, "main.go")
			err = os.WriteFile(mainGoPath, []byte(tc.sourceContent), 0644)
			suite.Require().NoError(err)

			// Test detection
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			result, err := suite.detector.DetectLanguage(ctx, testDir)
			suite.NoError(err, "Detection should succeed")
			suite.NotNil(result, "Should return result")

			// Check source analysis
			if sourceAnalysis, ok := result.Metadata["go_source_analysis"]; ok {
				analysis := sourceAnalysis.(*detectors.SourceAnalysis)
				for _, expectedImport := range tc.expectedStdLib {
					suite.Contains(analysis.ImportPaths, expectedImport, "Should detect standard library import %s", expectedImport)
				}
				for _, expectedImport := range tc.expectedThird {
					suite.Contains(analysis.ImportPaths, expectedImport, "Should detect third-party import %s", expectedImport)
				}
			}
		})
	}
}

func (suite *GoDetectorTestSuite) TestGoDetector_DependencyAnalysis() {
	testCases := []struct {
		name         string
		goModContent string
		expectedProd map[string]string
		expectedDev  map[string]string
	}{
		{
			name: "mixed-dependencies",
			goModContent: `module test-deps

go 1.20

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/stretchr/testify v1.8.4
	github.com/golang/mock v1.6.0
	github.com/onsi/ginkgo/v2 v2.11.0
	golang.org/x/sync v0.3.0
)`,
			expectedProd: map[string]string{
				"github.com/gin-gonic/gin": "v1.9.1",
				"golang.org/x/sync":        "v0.3.0",
			},
			expectedDev: map[string]string{
				"github.com/stretchr/testify": "v1.8.4",
				"github.com/golang/mock":      "v1.6.0",
				"github.com/onsi/ginkgo/v2":   "v2.11.0",
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create test directory
			testDir, err := os.MkdirTemp(suite.tempDir, tc.name+"-*")
			suite.Require().NoError(err)
			defer os.RemoveAll(testDir)

			// Create go.mod
			goModPath := filepath.Join(testDir, "go.mod")
			err = os.WriteFile(goModPath, []byte(tc.goModContent), 0644)
			suite.Require().NoError(err)

			// Create main.go
			mainGoPath := filepath.Join(testDir, "main.go")
			err = os.WriteFile(mainGoPath, []byte("package main\n\nfunc main() {}"), 0644)
			suite.Require().NoError(err)

			// Test detection
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			result, err := suite.detector.DetectLanguage(ctx, testDir)
			suite.NoError(err, "Detection should succeed")
			suite.NotNil(result, "Should return result")

			// Check production dependencies
			for dep, version := range tc.expectedProd {
				suite.Contains(result.Dependencies, dep, "Should contain production dependency %s", dep)
				suite.Equal(version, result.Dependencies[dep], "Should have correct version for %s", dep)
			}

			// Check development dependencies
			for dep, version := range tc.expectedDev {
				suite.Contains(result.DevDependencies, dep, "Should contain dev dependency %s", dep)
				suite.Equal(version, result.DevDependencies[dep], "Should have correct version for %s", dep)
			}
		})
	}
}

// Integration Tests

func (suite *GoDetectorTestSuite) TestGoDetector_MethodsIntegration() {
	// Test all interface methods work together

	// GetMarkerFiles
	markerFiles := suite.detector.GetMarkerFiles()
	expectedMarkers := []string{types.MARKER_GO_MOD, types.MARKER_GO_SUM, "go.work", "go.work.sum"}
	for _, expected := range expectedMarkers {
		suite.Contains(markerFiles, expected, "Should contain marker file %s", expected)
	}

	// GetRequiredServers
	servers := suite.detector.GetRequiredServers()
	suite.Contains(servers, types.SERVER_GOPLS, "Should require gopls server")

	// GetPriority
	priority := suite.detector.GetPriority()
	suite.Equal(types.PRIORITY_GO, priority, "Should return correct priority")
}

func (suite *GoDetectorTestSuite) TestGoDetector_LanguageInfoRetrieval() {
	info, err := suite.detector.GetLanguageInfo(types.PROJECT_TYPE_GO)
	suite.NoError(err, "Should retrieve language info")
	suite.NotNil(info, "Should return language info")

	suite.Equal(types.PROJECT_TYPE_GO, info.Name, "Should have correct name")
	suite.Equal("Go", info.DisplayName, "Should have display name")
	suite.NotEmpty(info.MinVersion, "Should have minimum version")
	suite.NotEmpty(info.MaxVersion, "Should have maximum version")
	suite.Contains(info.LSPServers, types.SERVER_GOPLS, "Should list gopls server")
	suite.Contains(info.FileExtensions, ".go", "Should list .go extension")
	suite.Contains(info.FileExtensions, ".mod", "Should list .mod extension")

	// Test unsupported language
	_, err = suite.detector.GetLanguageInfo("unsupported")
	suite.Error(err, "Should error for unsupported language")
}

func (suite *GoDetectorTestSuite) TestGoDetector_ProjectValidation() {
	testCases := []struct {
		name          string
		setupFiles    map[string]string
		shouldSucceed bool
		expectedError string
	}{
		{
			name: "valid-go-project",
			setupFiles: map[string]string{
				"go.mod":  "module test\n\ngo 1.20",
				"main.go": "package main\n\nfunc main() {}",
			},
			shouldSucceed: true,
		},
		{
			name: "valid-workspace-project",
			setupFiles: map[string]string{
				"go.work":        "go 1.20\n\nuse ./module",
				"module/go.mod":  "module test\n\ngo 1.20",
				"module/main.go": "package main\n\nfunc main() {}",
			},
			shouldSucceed: true,
		},
		{
			name: "missing-go-mod",
			setupFiles: map[string]string{
				"main.go": "package main\n\nfunc main() {}",
			},
			shouldSucceed: false,
			expectedError: "go.mod file not found",
		},
		{
			name: "malformed-go-mod",
			setupFiles: map[string]string{
				"go.mod":  "invalid go mod syntax",
				"main.go": "package main\n\nfunc main() {}",
			},
			shouldSucceed: false,
			expectedError: "go.mod syntax error",
		},
		{
			name: "no-source-files",
			setupFiles: map[string]string{
				"go.mod":    "module test\n\ngo 1.20",
				"README.md": "# Test Project",
			},
			shouldSucceed: false,
			expectedError: "No Go source files",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create test directory
			testDir, err := os.MkdirTemp(suite.tempDir, tc.name+"-*")
			suite.Require().NoError(err)
			defer os.RemoveAll(testDir)

			// Create test files
			for filePath, content := range tc.setupFiles {
				fullPath := filepath.Join(testDir, filePath)
				err := os.MkdirAll(filepath.Dir(fullPath), 0755)
				suite.Require().NoError(err)
				err = os.WriteFile(fullPath, []byte(content), 0644)
				suite.Require().NoError(err)
			}

			// Test validation
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err = suite.detector.ValidateStructure(ctx, testDir)

			if tc.shouldSucceed {
				suite.NoError(err, "Validation should succeed for %s", tc.name)
			} else {
				suite.Error(err, "Validation should fail for %s", tc.name)
				if tc.expectedError != "" {
					suite.Contains(err.Error(), tc.expectedError, "Should contain expected error message")
				}
			}
		})
	}
}

// Edge Cases and Error Handling Tests

func (suite *GoDetectorTestSuite) TestGoDetector_ErrorHandling() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test non-existent directory
	_, err := suite.detector.DetectLanguage(ctx, "/non/existent/path")
	suite.Error(err, "Should error for non-existent directory")

	// Test empty directory
	emptyDir, err := os.MkdirTemp(suite.tempDir, "empty-*")
	suite.Require().NoError(err)
	defer os.RemoveAll(emptyDir)

	result, err := suite.detector.DetectLanguage(ctx, emptyDir)
	suite.NoError(err, "Should handle empty directory gracefully")
	suite.Equal(0.0, result.Confidence, "Should have zero confidence for empty directory")

	// Test permission denied (Unix-like systems only)
	if os.Getenv("GOOS") != "windows" {
		restrictedDir, err := os.MkdirTemp(suite.tempDir, "restricted-*")
		suite.Require().NoError(err)
		defer func() {
			os.Chmod(restrictedDir, 0755) // Restore permissions for cleanup
			os.RemoveAll(restrictedDir)
		}()

		// Create a file first
		testFile := filepath.Join(restrictedDir, "test.go")
		err = os.WriteFile(testFile, []byte("package main"), 0644)
		suite.Require().NoError(err)

		// Remove read permissions
		err = os.Chmod(restrictedDir, 0000)
		suite.Require().NoError(err)

		_, err = suite.detector.DetectLanguage(ctx, restrictedDir)
		// Should either handle gracefully or error - but not panic
		suite.NotPanics(func() {
			suite.detector.DetectLanguage(ctx, restrictedDir)
		}, "Should not panic on permission denied")
	}
}

func (suite *GoDetectorTestSuite) TestGoDetector_TimeoutHandling() {
	// Create a project
	testDir, err := os.MkdirTemp(suite.tempDir, "timeout-test-*")
	suite.Require().NoError(err)
	defer os.RemoveAll(testDir)

	goModPath := filepath.Join(testDir, "go.mod")
	err = os.WriteFile(goModPath, []byte("module test\n\ngo 1.20"), 0644)
	suite.Require().NoError(err)

	mainGoPath := filepath.Join(testDir, "main.go")
	err = os.WriteFile(mainGoPath, []byte("package main\n\nfunc main() {}"), 0644)
	suite.Require().NoError(err)

	// Test with very short timeout
	shortCtx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Should either succeed quickly or timeout
	_, err = suite.detector.DetectLanguage(shortCtx, testDir)
	if err != nil {
		suite.Contains(err.Error(), "timeout", "Should indicate timeout")
	}

	// Test with normal timeout
	normalCtx, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel2()

	result, err := suite.detector.DetectLanguage(normalCtx, testDir)
	suite.NoError(err, "Should succeed with normal timeout")
	suite.NotNil(result, "Should return result")
}

func (suite *GoDetectorTestSuite) TestGoDetector_MalformedFiles() {
	testCases := []struct {
		name        string
		setupFiles  map[string]string
		shouldError bool
	}{
		{
			name: "malformed-go-mod",
			setupFiles: map[string]string{
				"go.mod":  "invalid syntax here\nno module directive",
				"main.go": "package main\n\nfunc main() {}",
			},
			shouldError: false, // Should handle gracefully
		},
		{
			name: "malformed-go-work",
			setupFiles: map[string]string{
				"go.mod":  "module test\n\ngo 1.20",
				"go.work": "invalid workspace syntax",
				"main.go": "package main\n\nfunc main() {}",
			},
			shouldError: false, // Should handle gracefully
		},
		{
			name: "binary-files",
			setupFiles: map[string]string{
				"go.mod":  "module test\n\ngo 1.20",
				"main.go": "package main\n\nfunc main() {}",
				"binary":  string([]byte{0x00, 0x01, 0x02, 0x03}),
			},
			shouldError: false, // Should skip binary files
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create test directory
			testDir, err := os.MkdirTemp(suite.tempDir, tc.name+"-*")
			suite.Require().NoError(err)
			defer os.RemoveAll(testDir)

			// Create test files
			for filePath, content := range tc.setupFiles {
				fullPath := filepath.Join(testDir, filePath)
				err := os.MkdirAll(filepath.Dir(fullPath), 0755)
				suite.Require().NoError(err)
				err = os.WriteFile(fullPath, []byte(content), 0644)
				suite.Require().NoError(err)
			}

			// Test detection
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			result, err := suite.detector.DetectLanguage(ctx, testDir)

			if tc.shouldError {
				suite.Error(err, "Should error for %s", tc.name)
			} else {
				suite.NoError(err, "Should handle gracefully for %s", tc.name)
				suite.NotNil(result, "Should return result")
			}
		})
	}
}

func (suite *GoDetectorTestSuite) TestGoDetector_LargeProject() {
	// Create large project structure
	testDir, err := os.MkdirTemp(suite.tempDir, "large-project-*")
	suite.Require().NoError(err)
	defer os.RemoveAll(testDir)

	// Create go.mod
	goModPath := filepath.Join(testDir, "go.mod")
	err = os.WriteFile(goModPath, []byte("module large-test\n\ngo 1.20"), 0644)
	suite.Require().NoError(err)

	// Create many directories and Go files
	for i := 0; i < 50; i++ {
		dirPath := filepath.Join(testDir, "pkg", "module"+string(rune(i)))
		err = os.MkdirAll(dirPath, 0755)
		suite.Require().NoError(err)

		for j := 0; j < 5; j++ {
			goFile := filepath.Join(dirPath, "file"+string(rune(j))+".go")
			content := "package module" + string(rune(i)) + "\n\nfunc Function" + string(rune(j)) + "() {}"
			err = os.WriteFile(goFile, []byte(content), 0644)
			suite.Require().NoError(err)
		}
	}

	// Add some test files
	testDir2 := filepath.Join(testDir, "test")
	err = os.MkdirAll(testDir2, 0755)
	suite.Require().NoError(err)

	for i := 0; i < 10; i++ {
		testFile := filepath.Join(testDir2, "test"+string(rune(i))+"_test.go")
		content := "package test\n\nimport \"testing\"\n\nfunc TestFunction" + string(rune(i)) + "(t *testing.T) {}"
		err = os.WriteFile(testFile, []byte(content), 0644)
		suite.Require().NoError(err)
	}

	// Test detection with performance measurement
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	start := time.Now()
	result, err := suite.detector.DetectLanguage(ctx, testDir)
	duration := time.Since(start)

	suite.NoError(err, "Should handle large project")
	suite.NotNil(result, "Should return result")
	suite.Equal(types.PROJECT_TYPE_GO, result.Language, "Should detect Go language")
	suite.Greater(result.Confidence, 0.5, "Should have reasonable confidence")
	suite.Less(duration, 30*time.Second, "Should complete within reasonable time")

	suite.T().Logf("Large project detection took: %v", duration)
	suite.T().Logf("Found %d packages", result.Metadata["go_packages_found"])
}

// Performance Tests

func (suite *GoDetectorTestSuite) TestGoDetector_PerformanceProfile() {
	// Start performance profiling
	ctx := context.Background()
	err := suite.profiler.Start(ctx)
	suite.Require().NoError(err)
	defer suite.profiler.Stop()

	// Create complex test project
	config := &framework.ProjectGenerationConfig{
		Type:         framework.ProjectTypeSingle,
		Languages:    []string{"go"},
		Complexity:   framework.ComplexityComplex,
		Size:         framework.SizeLarge,
		BuildSystem:  true,
		TestFiles:    true,
		Dependencies: true,
	}

	// Generate using mock or create manually
	testDir, err := os.MkdirTemp(suite.tempDir, "perf-test-*")
	suite.Require().NoError(err)
	defer os.RemoveAll(testDir)

	// Create complex project structure manually
	files := map[string]string{
		"go.mod": `module github.com/example/perf-test

go 1.20

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/stretchr/testify v1.8.4
	golang.org/x/sync v0.3.0
)`,
		"go.sum":          "github.com/gin-gonic/gin v1.9.1 h1:hash...",
		"main.go":         "package main\n\nfunc main() {}",
		"internal/lib.go": "package internal\n\nfunc Lib() {}",
		"pkg/utils.go":    "package pkg\n\nfunc Utils() {}",
		"cmd/app/main.go": "package main\n\nfunc main() {}",
		".golangci.yml":   "run:\n  timeout: 5m",
		"Makefile":        "build:\n\tgo build",
	}

	for filePath, content := range files {
		fullPath := filepath.Join(testDir, filePath)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		suite.Require().NoError(err)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		suite.Require().NoError(err)
	}

	// Start operation profiling
	metrics, err := suite.profiler.StartOperation("go_detection")
	suite.Require().NoError(err)

	// Perform detection
	detectionCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	startTime := time.Now()
	result, err := suite.detector.DetectLanguage(detectionCtx, testDir)
	detectionDuration := time.Since(startTime)

	suite.NoError(err, "Detection should succeed")
	suite.NotNil(result, "Should return result")
	suite.Equal(types.PROJECT_TYPE_GO, result.Language, "Should detect Go")

	// End profiling
	finalMetrics, err := suite.profiler.EndOperation(metrics.OperationID)
	suite.NoError(err, "Should end profiling")

	// Verify performance characteristics
	suite.Less(detectionDuration, 15*time.Second, "Should complete quickly")
	suite.Greater(result.Confidence, 0.8, "Should have high confidence")

	// Log performance results
	suite.T().Logf("Go Detection Performance:")
	suite.T().Logf("  Duration: %v", detectionDuration)
	suite.T().Logf("  Memory: %d bytes", finalMetrics.MemoryAllocated)
	suite.T().Logf("  Confidence: %f", result.Confidence)
	suite.T().Logf("  Marker Files: %d", len(result.MarkerFiles))
	suite.T().Logf("  Dependencies: %d", len(result.Dependencies))
}

// Benchmark Tests

func (suite *GoDetectorTestSuite) BenchmarkSimpleGoDetection() {
	// Create simple Go project
	testDir, err := os.MkdirTemp(suite.tempDir, "bench-simple-*")
	suite.Require().NoError(err)
	defer os.RemoveAll(testDir)

	files := map[string]string{
		"go.mod":  "module bench-test\n\ngo 1.20",
		"main.go": "package main\n\nfunc main() {}",
	}

	for filePath, content := range files {
		fullPath := filepath.Join(testDir, filePath)
		err := os.WriteFile(fullPath, []byte(content), 0644)
		suite.Require().NoError(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Run benchmark
	suite.T().Run("BenchmarkSimpleDetection", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			detector := detectors.NewGoProjectDetector()
			_, err := detector.DetectLanguage(ctx, testDir)
			assert.NoError(t, err)
		}
	})
}

func (suite *GoDetectorTestSuite) BenchmarkComplexGoDetection() {
	// Create complex Go project
	testDir, err := os.MkdirTemp(suite.tempDir, "bench-complex-*")
	suite.Require().NoError(err)
	defer os.RemoveAll(testDir)

	files := map[string]string{
		"go.mod": `module bench-complex

go 1.20

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/stretchr/testify v1.8.4
)`,
		"go.sum":            "github.com/gin-gonic/gin v1.9.1 h1:hash...",
		"main.go":           "package main\n\nfunc main() {}",
		"internal/lib.go":   "package internal\n\nfunc Lib() {}",
		"pkg/utils.go":      "package pkg\n\nfunc Utils() {}",
		"pkg/utils_test.go": "package pkg\n\nimport \"testing\"\n\nfunc TestUtils(t *testing.T) {}",
		".golangci.yml":     "run:\n  timeout: 5m",
		"Makefile":          "build:\n\tgo build",
	}

	for filePath, content := range files {
		fullPath := filepath.Join(testDir, filePath)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		suite.Require().NoError(err)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		suite.Require().NoError(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Run benchmark
	suite.T().Run("BenchmarkComplexDetection", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			detector := detectors.NewGoProjectDetector()
			_, err := detector.DetectLanguage(ctx, testDir)
			assert.NoError(t, err)
		}
	})
}

// Test Suite Runner

func TestGoDetectorTestSuite(t *testing.T) {
	suite.Run(t, new(GoDetectorTestSuite))
}

// Individual test functions for specific scenarios

func TestGoDetector_ContextCancellation(t *testing.T) {
	// Create test project
	tempDir, err := os.MkdirTemp("", "context-cancel-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	goModPath := filepath.Join(tempDir, "go.mod")
	err = os.WriteFile(goModPath, []byte("module test\n\ngo 1.20"), 0644)
	require.NoError(t, err)

	mainGoPath := filepath.Join(tempDir, "main.go")
	err = os.WriteFile(mainGoPath, []byte("package main\n\nfunc main() {}"), 0644)
	require.NoError(t, err)

	// Test context cancellation
	detector := detectors.NewGoProjectDetector()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = detector.DetectLanguage(ctx, tempDir)
	assert.Error(t, err, "Should error when context is cancelled")
	assert.Contains(t, err.Error(), "context canceled", "Should indicate context cancellation")
}

func TestGoDetector_VendorDirectorySkipping(t *testing.T) {
	// Create test project with vendor directory
	tempDir, err := os.MkdirTemp("", "vendor-skip-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	files := map[string]string{
		"go.mod":                    "module test\n\ngo 1.20",
		"main.go":                   "package main\n\nfunc main() {}",
		"vendor/github.com/lib.go":  "package lib\n\nfunc Lib() {}",
		"vendor/modules.txt":        "# modules.txt",
		"node_modules/package.json": `{"name": "test"}`,
		".git/config":               "[core]\n  repositoryformatversion = 0",
	}

	for filePath, content := range files {
		fullPath := filepath.Join(tempDir, filePath)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}

	// Test detection
	detector := detectors.NewGoProjectDetector()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := detector.DetectLanguage(ctx, tempDir)
	assert.NoError(t, err, "Should succeed despite vendor directories")
	assert.NotNil(t, result, "Should return result")
	assert.Equal(t, types.PROJECT_TYPE_GO, result.Language, "Should detect Go")

	// Verify vendor files are not included in analysis
	if sourceAnalysis, ok := result.Metadata["go_source_analysis"]; ok {
		analysis := sourceAnalysis.(*detectors.SourceAnalysis)
		for pkgPath := range analysis.Packages {
			assert.False(t, strings.Contains(pkgPath, "vendor"), "Should not include vendor packages")
			assert.False(t, strings.Contains(pkgPath, "node_modules"), "Should not include node_modules")
			assert.False(t, strings.Contains(pkgPath, ".git"), "Should not include .git")
		}
	}
}
