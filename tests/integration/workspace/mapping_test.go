package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/workspace"
	"lsp-gateway/tests/integration/workspace/helpers"
)

type WorkspaceMappingSuite struct {
	suite.Suite
	tempDir   string
	detector  workspace.WorkspaceDetector
	helper    *helpers.WorkspaceTestHelper
}

func (suite *WorkspaceMappingSuite) SetupSuite() {
	tempDir, err := os.MkdirTemp("", "workspace-mapping-test-*")
	require.NoError(suite.T(), err)
	
	suite.tempDir = tempDir
	suite.detector = workspace.NewWorkspaceDetector()
	suite.helper = helpers.NewWorkspaceTestHelper(tempDir)
}

func (suite *WorkspaceMappingSuite) TearDownSuite() {
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
}

func (suite *WorkspaceMappingSuite) TestFindSubProjectForPathExactMatch() {
	// Create multi-project workspace
	workspacePath := suite.helper.CreateGoWorkspace("exact-match-test", map[string]string{
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

func main() {}
`,
		"service-b/go.mod": `module example.com/service-b

go 1.24
`,
		"service-b/main.go": `package main

func main() {}
`,
		"libs/shared/go.mod": `module example.com/shared

go 1.24
`,
		"libs/shared/utils.go": `package shared

func CommonFunc() string {
	return "shared utility"
}
`,
	})

	result, err := suite.detector.DetectWorkspaceAt(workspacePath)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), result)

	// Test exact path matches
	serviceAPath := filepath.Join(workspacePath, "service-a")
	serviceBPath := filepath.Join(workspacePath, "service-b")
	sharedPath := filepath.Join(workspacePath, "libs", "shared")

	serviceAProject := suite.detector.FindSubProjectForPath(result, serviceAPath)
	assert.NotNil(suite.T(), serviceAProject)
	assert.Equal(suite.T(), "service-a", serviceAProject.Name)
	assert.Equal(suite.T(), serviceAPath, serviceAProject.AbsolutePath)

	serviceBProject := suite.detector.FindSubProjectForPath(result, serviceBPath)
	assert.NotNil(suite.T(), serviceBProject)
	assert.Equal(suite.T(), "service-b", serviceBProject.Name)
	assert.Equal(suite.T(), serviceBPath, serviceBProject.AbsolutePath)

	sharedProject := suite.detector.FindSubProjectForPath(result, sharedPath)
	assert.NotNil(suite.T(), sharedProject)
	assert.Equal(suite.T(), "shared", sharedProject.Name)
	assert.Equal(suite.T(), sharedPath, sharedProject.AbsolutePath)
}

func (suite *WorkspaceMappingSuite) TestFindSubProjectForPathNestedFiles() {
	// Create workspace with nested structure
	workspacePath := suite.helper.CreateMonorepo("nested-files-test", map[string]string{
		"apps/web/package.json": `{
  "name": "@company/web",
  "dependencies": {
    "react": "^18.0.0"
  }
}`,
		"apps/web/src/components/Button.tsx": `export const Button = () => <button>Click</button>`,
		"apps/web/src/pages/index.tsx": `export default function Home() {
  return <div>Home</div>
}`,
		"apps/api/package.json": `{
  "name": "@company/api",
  "dependencies": {
    "express": "^4.18.0"
  }
}`,
		"apps/api/src/routes/users.js": `const express = require('express');
const router = express.Router();
module.exports = router;`,
		"packages/ui/package.json": `{
  "name": "@company/ui",
  "dependencies": {
    "react": "^18.0.0"
  }
}`,
		"packages/ui/src/Button.tsx": `export const Button = () => <button>UI Button</button>`,
	})

	result, err := suite.detector.DetectWorkspaceAt(workspacePath)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), result)

	// Test nested file path resolution
	testCases := []struct {
		filePath        string
		expectedProject string
	}{
		{
			filePath:        filepath.Join(workspacePath, "apps", "web", "src", "components", "Button.tsx"),
			expectedProject: "web",
		},
		{
			filePath:        filepath.Join(workspacePath, "apps", "web", "src", "pages", "index.tsx"),
			expectedProject: "web",
		},
		{
			filePath:        filepath.Join(workspacePath, "apps", "api", "src", "routes", "users.js"),
			expectedProject: "api",
		},
		{
			filePath:        filepath.Join(workspacePath, "packages", "ui", "src", "Button.tsx"),
			expectedProject: "ui",
		},
	}

	for _, tc := range testCases {
		project := suite.detector.FindSubProjectForPath(result, tc.filePath)
		assert.NotNil(suite.T(), project, "No project found for file path: %s", tc.filePath)
		assert.Equal(suite.T(), tc.expectedProject, project.Name, "Wrong project for file path: %s", tc.filePath)
	}
}

func (suite *WorkspaceMappingSuite) TestFindSubProjectForPathDeepestMatch() {
	// Create workspace with overlapping project boundaries
	workspacePath := suite.helper.CreateNestedProject("deepest-match-test", map[string]string{
		"go.mod": `module deepest-match-test

go 1.24
`,
		"main.go": `package main

func main() {}
`,
		"services/user-service/go.mod": `module deepest-match-test/user-service

go 1.24
`,
		"services/user-service/main.go": `package main

func main() {}
`,
		"services/user-service/internal/auth/go.mod": `module deepest-match-test/user-service/auth

go 1.24
`,
		"services/user-service/internal/auth/auth.go": `package auth

func Authenticate() {}
`,
	})

	result, err := suite.detector.DetectWorkspaceAt(workspacePath)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), result)

	// Test that deepest (most specific) project is returned
	testCases := []struct {
		filePath        string
		expectedProject string
	}{
		{
			filePath:        filepath.Join(workspacePath, "main.go"),
			expectedProject: "deepest-match-test", // Root project
		},
		{
			filePath:        filepath.Join(workspacePath, "services", "user-service", "main.go"),
			expectedProject: "user-service", // Middle level
		},
		{
			filePath:        filepath.Join(workspacePath, "services", "user-service", "internal", "auth", "auth.go"),
			expectedProject: "auth", // Deepest level
		},
		{
			filePath:        filepath.Join(workspacePath, "services", "user-service", "internal", "auth"),
			expectedProject: "auth", // Directory itself
		},
	}

	for _, tc := range testCases {
		project := suite.detector.FindSubProjectForPath(result, tc.filePath)
		assert.NotNil(suite.T(), project, "No project found for file path: %s", tc.filePath)
		assert.Equal(suite.T(), tc.expectedProject, project.Name, "Wrong project for file path: %s", tc.filePath)
	}
}

func (suite *WorkspaceMappingSuite) TestFindSubProjectForPathNonexistentFile() {
	// Create simple workspace
	workspacePath := suite.helper.CreateSimpleGoProject("nonexistent-test", map[string]string{
		"go.mod": `module nonexistent-test

go 1.24
`,
		"main.go": `package main

func main() {}
`,
	})

	result, err := suite.detector.DetectWorkspaceAt(workspacePath)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), result)

	// Test with nonexistent files that would belong to existing projects
	nonexistentInRoot := filepath.Join(workspacePath, "nonexistent.go")
	nonexistentInSubdir := filepath.Join(workspacePath, "subdir", "file.go")

	rootProject := suite.detector.FindSubProjectForPath(result, nonexistentInRoot)
	assert.NotNil(suite.T(), rootProject)
	assert.Equal(suite.T(), "nonexistent-test", rootProject.Name)

	subdirProject := suite.detector.FindSubProjectForPath(result, nonexistentInSubdir)
	assert.NotNil(suite.T(), subdirProject)
	assert.Equal(suite.T(), "nonexistent-test", subdirProject.Name)
}

func (suite *WorkspaceMappingSuite) TestFindSubProjectForPathOutsideWorkspace() {
	// Create simple workspace
	workspacePath := suite.helper.CreateSimpleGoProject("outside-test", map[string]string{
		"go.mod": `module outside-test

go 1.24
`,
		"main.go": `package main

func main() {}
`,
	})

	result, err := suite.detector.DetectWorkspaceAt(workspacePath)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), result)

	// Test with file paths outside the workspace
	outsidePath := filepath.Join(suite.tempDir, "outside-file.go")
	parentPath := filepath.Join(filepath.Dir(workspacePath), "sibling-file.go")

	outsideProject := suite.detector.FindSubProjectForPath(result, outsidePath)
	assert.Nil(suite.T(), outsideProject)

	parentProject := suite.detector.FindSubProjectForPath(result, parentPath)
	assert.Nil(suite.T(), parentProject)
}

func (suite *WorkspaceMappingSuite) TestFindSubProjectForPathNilWorkspace() {
	// Test with nil workspace
	project := suite.detector.FindSubProjectForPath(nil, "/some/path")
	assert.Nil(suite.T(), project)
}

func (suite *WorkspaceMappingSuite) TestFindSubProjectForPathEmptyWorkspace() {
	// Create empty workspace context
	emptyWorkspace := &workspace.WorkspaceContext{
		Root:         suite.tempDir,
		SubProjects:  []*workspace.DetectedSubProject{},
		ProjectPaths: map[string]*workspace.DetectedSubProject{},
		Languages:    []string{types.PROJECT_TYPE_UNKNOWN},
		ProjectType:  types.PROJECT_TYPE_UNKNOWN,
	}

	project := suite.detector.FindSubProjectForPath(emptyWorkspace, "/some/path")
	assert.Nil(suite.T(), project)
}

func (suite *WorkspaceMappingSuite) TestProjectPathMappingEfficiency() {
	// Create workspace with many sub-projects to test mapping efficiency
	workspacePath := suite.helper.CreateLargeWorkspace("efficiency-test", 20)

	result, err := suite.detector.DetectWorkspaceAt(workspacePath)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), result)

	// Verify that ProjectPaths mapping is populated
	assert.NotEmpty(suite.T(), result.ProjectPaths)
	assert.Equal(suite.T(), len(result.SubProjects), len(result.ProjectPaths)/2) // Each project has absolute and relative path mappings

	// Test lookup performance (should be O(1) for direct mapping)
	for _, subProject := range result.SubProjects {
		// Test absolute path lookup
		foundProject := result.ProjectPaths[subProject.AbsolutePath]
		assert.NotNil(suite.T(), foundProject)
		assert.Equal(suite.T(), subProject.ID, foundProject.ID)

		// Test relative path lookup
		foundProject = result.ProjectPaths[subProject.RelativePath]
		assert.NotNil(suite.T(), foundProject)
		assert.Equal(suite.T(), subProject.ID, foundProject.ID)
	}
}

func (suite *WorkspaceMappingSuite) TestSubProjectSorting() {
	// Create workspace with projects at different depths
	workspacePath := suite.helper.CreateNestedProject("sorting-test", map[string]string{
		"go.mod": `module sorting-test

go 1.24
`,
		"main.go": `package main

func main() {}
`,
		"level1/go.mod": `module sorting-test/level1

go 1.24
`,
		"level1/main.go": `package main

func main() {}
`,
		"level1/level2/go.mod": `module sorting-test/level1/level2

go 1.24
`,
		"level1/level2/main.go": `package main

func main() {}
`,
		"services/user/go.mod": `module sorting-test/user

go 1.24
`,
		"services/user/main.go": `package main

func main() {}
`,
	})

	result, err := suite.detector.DetectWorkspaceAt(workspacePath)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), result)

	// Verify that deeper projects take precedence in path resolution
	deepFilePath := filepath.Join(workspacePath, "level1", "level2", "deep-file.go")
	project := suite.detector.FindSubProjectForPath(result, deepFilePath)
	assert.NotNil(suite.T(), project)
	assert.Equal(suite.T(), "level2", project.Name) // Should match deepest project

	// Verify that ProjectPaths mapping respects depth ordering
	for absolutePath, mappedProject := range result.ProjectPaths {
		if filepath.IsAbs(absolutePath) {
			// Find all projects that could contain this path
			var candidates []*workspace.DetectedSubProject
			for _, sp := range result.SubProjects {
				if absolutePath == sp.AbsolutePath {
					candidates = append(candidates, sp)
				}
			}

			// Should map to the project with exact path match
			if len(candidates) > 0 {
				assert.Equal(suite.T(), mappedProject.AbsolutePath, absolutePath)
			}
		}
	}
}

func (suite *WorkspaceMappingSuite) TestFilePathResolutionEdgeCases() {
	// Create workspace with edge case directory names
	workspacePath := suite.helper.CreateWorkspaceWithSpecialNames("edge-cases", map[string]string{
		"go.mod": `module edge-cases

go 1.24
`,
		"main.go": `package main

func main() {}
`,
		"dir-with-spaces/go.mod": `module edge-cases/spaces

go 1.24
`,
		"dir-with-spaces/main.go": `package main

func main() {}
`,
		"dir.with.dots/go.mod": `module edge-cases/dots

go 1.24
`,
		"dir.with.dots/main.go": `package main

func main() {}
`,
		"dir_with_underscores/go.mod": `module edge-cases/underscores

go 1.24
`,
		"dir_with_underscores/main.go": `package main

func main() {}
`,
	})

	result, err := suite.detector.DetectWorkspaceAt(workspacePath)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), result)

	// Test path resolution with special characters
	testCases := []struct {
		fileName        string
		expectedProject string
	}{
		{
			fileName:        filepath.Join(workspacePath, "dir-with-spaces", "file.go"),
			expectedProject: "dir-with-spaces",
		},
		{
			fileName:        filepath.Join(workspacePath, "dir.with.dots", "file.go"),
			expectedProject: "dir.with.dots",
		},
		{
			fileName:        filepath.Join(workspacePath, "dir_with_underscores", "file.go"),
			expectedProject: "dir_with_underscores",
		},
	}

	for _, tc := range testCases {
		project := suite.detector.FindSubProjectForPath(result, tc.fileName)
		assert.NotNil(suite.T(), project, "No project found for file: %s", tc.fileName)
		assert.Equal(suite.T(), tc.expectedProject, project.Name, "Wrong project for file: %s", tc.fileName)
	}
}

func (suite *WorkspaceMappingSuite) TestCrossLanguageFileMapping() {
	// Create multi-language workspace
	workspacePath := suite.helper.CreateMultiLanguageWorkspace("cross-language", map[string]string{
		"backend/go.mod": `module cross-language/backend

go 1.24
`,
		"backend/main.go": `package main

func main() {}
`,
		"frontend/package.json": `{
  "name": "frontend",
  "dependencies": {
    "react": "^18.0.0"
  }
}`,
		"frontend/tsconfig.json": `{
  "compilerOptions": {
    "target": "es2017"
  }
}`,
		"frontend/src/App.tsx": `export default function App() {
  return <div>App</div>
}`,
		"ml-service/pyproject.toml": `[project]
name = "ml-service"
version = "0.1.0"
`,
		"ml-service/src/model.py": `def train_model():
    pass
`,
		"mobile/android/build.gradle": `android {
    compileSdkVersion 33
}`,
		"mobile/android/src/MainActivity.java": `public class MainActivity {
    
}`,
	})

	result, err := suite.detector.DetectWorkspaceAt(workspacePath)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), result)

	// Test file mapping across different languages
	testCases := []struct {
		filePath        string
		expectedProject string
		expectedLang    string
	}{
		{
			filePath:        filepath.Join(workspacePath, "backend", "api.go"),
			expectedProject: "backend",
			expectedLang:    types.PROJECT_TYPE_GO,
		},
		{
			filePath:        filepath.Join(workspacePath, "frontend", "src", "components", "Button.tsx"),
			expectedProject: "frontend",
			expectedLang:    types.PROJECT_TYPE_TYPESCRIPT,
		},
		{
			filePath:        filepath.Join(workspacePath, "ml-service", "train.py"),
			expectedProject: "ml-service",
			expectedLang:    types.PROJECT_TYPE_PYTHON,
		},
		{
			filePath:        filepath.Join(workspacePath, "mobile", "android", "src", "Service.java"),
			expectedProject: "android",
			expectedLang:    types.PROJECT_TYPE_JAVA,
		},
	}

	for _, tc := range testCases {
		project := suite.detector.FindSubProjectForPath(result, tc.filePath)
		assert.NotNil(suite.T(), project, "No project found for file: %s", tc.filePath)
		assert.Equal(suite.T(), tc.expectedProject, project.Name, "Wrong project for file: %s", tc.filePath)
		assert.Contains(suite.T(), project.Languages, tc.expectedLang, "Expected language %s not found in project %s", tc.expectedLang, project.Name)
	}
}

func TestWorkspaceMappingSuite(t *testing.T) {
	suite.Run(t, new(WorkspaceMappingSuite))
}