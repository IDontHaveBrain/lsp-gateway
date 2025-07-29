package workspace

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

	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/workspace"
	"lsp-gateway/tests/integration/workspace/helpers"
)

type WorkspaceEdgeCasesSuite struct {
	suite.Suite
	tempDir   string
	detector  workspace.WorkspaceDetector
	helper    *helpers.WorkspaceTestHelper
}

func (suite *WorkspaceEdgeCasesSuite) SetupSuite() {
	tempDir, err := os.MkdirTemp("", "workspace-edge-cases-test-*")
	require.NoError(suite.T(), err)
	
	suite.tempDir = tempDir
	suite.detector = workspace.NewWorkspaceDetector()
	suite.helper = helpers.NewWorkspaceTestHelper(tempDir)
}

func (suite *WorkspaceEdgeCasesSuite) TearDownSuite() {
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
}

func (suite *WorkspaceEdgeCasesSuite) TestCircularSymlinks() {
	if os.Getenv("GOOS") == "windows" {
		suite.T().Skip("Skipping symlink test on Windows")
	}

	// Create workspace with circular symlinks
	workspacePath := filepath.Join(suite.tempDir, "circular-symlinks")
	err := os.MkdirAll(workspacePath, 0755)
	require.NoError(suite.T(), err)

	// Create valid content first
	err = os.WriteFile(filepath.Join(workspacePath, "go.mod"), []byte(`module circular-test

go 1.24
`), 0644)
	require.NoError(suite.T(), err)

	// Create circular symlinks
	linkA := filepath.Join(workspacePath, "link-a")
	linkB := filepath.Join(workspacePath, "link-b")

	err = os.Symlink(linkB, linkA)
	require.NoError(suite.T(), err)
	
	err = os.Symlink(linkA, linkB)
	require.NoError(suite.T(), err)

	// Detection should handle circular links gracefully
	result, err := suite.detector.DetectWorkspaceAt(workspacePath)
	
	// Should either succeed (ignoring circular links) or fail gracefully
	if err != nil {
		assert.Contains(suite.T(), err.Error(), "circular")
	} else {
		assert.NotNil(suite.T(), result)
		assert.Greater(suite.T(), len(result.SubProjects), 0)
	}
}

func (suite *WorkspaceEdgeCasesSuite) TestVeryLongPaths() {
	// Create workspace with very long paths
	longDirName := strings.Repeat("very-long-directory-name-", 20)
	workspacePath := filepath.Join(suite.tempDir, "long-paths")
	longPath := filepath.Join(workspacePath, longDirName, "subdir", longDirName)
	
	err := os.MkdirAll(longPath, 0755)
	require.NoError(suite.T(), err)

	// Create project files
	err = os.WriteFile(filepath.Join(workspacePath, "go.mod"), []byte(`module long-paths-test

go 1.24
`), 0644)
	require.NoError(suite.T(), err)

	err = os.WriteFile(filepath.Join(longPath, "go.mod"), []byte(`module long-paths-test/nested

go 1.24
`), 0644)
	require.NoError(suite.T(), err)

	// Detection should handle long paths
	result, err := suite.detector.DetectWorkspaceAt(workspacePath)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Greater(suite.T(), len(result.SubProjects), 0)
}

func (suite *WorkspaceEdgeCasesSuite) TestUnicodeFileNames() {
	// Create workspace with Unicode file and directory names
	unicodeStructure := map[string]string{
		"go.mod": `module unicode-test

go 1.24
`,
		"main.go": `package main

func main() {}
`,
		"测试目录/go.mod": `module unicode-test/chinese

go 1.24
`,
		"测试目录/主要.go": `package main

func main() {}
`,
		"русский/go.mod": `module unicode-test/russian

go 1.24
`, 
		"русский/файл.go": `package main

func main() {}
`,
		"日本語/go.mod": `module unicode-test/japanese

go 1.24
`,
		"日本語/ファイル.go": `package main

func main() {}
`,
		"🚀emoji🌟/package.json": `{
  "name": "emoji-project",
  "version": "1.0.0"
}`,
		"🚀emoji🌟/index.js": `console.log("Unicode works!");`,
	}

	workspacePath := suite.helper.CreateWorkspaceWithSpecialNames("unicode-workspace", unicodeStructure)

	result, err := suite.detector.DetectWorkspaceAt(workspacePath)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)

	// Should detect multiple sub-projects with Unicode names
	assert.Greater(suite.T(), len(result.SubProjects), 3)

	// Verify Unicode project names are handled correctly
	projectNames := make([]string, len(result.SubProjects))
	for i, sp := range result.SubProjects {
		projectNames[i] = sp.Name
	}

	expectedNames := []string{"unicode-test", "测试目录", "русский", "日本語", "🚀emoji🌟"}
	for _, expectedName := range expectedNames {
		found := false
		for _, name := range projectNames {
			if strings.Contains(name, expectedName) || name == expectedName {
				found = true
				break
			}
		}
		assert.True(suite.T(), found, "Expected Unicode project name not found: %s", expectedName)
	}
}

func (suite *WorkspaceEdgeCasesSuite) TestEmptyAndCorruptedMarkerFiles() {
	// Create workspace with various corrupted marker files
	corruptedStructure := map[string]string{
		// Valid root project
		"go.mod": `module corrupted-test

go 1.24
`,
		"main.go": `package main

func main() {}
`,
		// Empty marker files
		"empty-go/go.mod":     "",
		"empty-node/package.json": "",
		"empty-python/pyproject.toml": "",
		
		// Corrupted JSON
		"bad-json/package.json": `{
  "name": "bad-json"
  "version": missing comma
}`,
		
		// Corrupted TOML
		"bad-toml/pyproject.toml": `[project
name = missing bracket
`,
		
		// Binary data as marker file
		"binary-data/go.mod": string([]byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE}),
		
		// Very large marker file
		"large-marker/go.mod": `module large-marker

go 1.24

` + strings.Repeat("// This is a very long comment\n", 1000),
	}

	workspacePath := suite.helper.CreateCorruptedWorkspace("corrupted-markers", corruptedStructure)

	// Detection should handle corrupted files gracefully
	result, err := suite.detector.DetectWorkspaceAt(workspacePath)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)

	// Should still detect projects (we detect by file existence, not content validity)
	assert.Greater(suite.T(), len(result.SubProjects), 5)

	// Root project should be detected correctly
	rootFound := false
	for _, sp := range result.SubProjects {
		if sp.RelativePath == "." {
			rootFound = true
			assert.Equal(suite.T(), types.PROJECT_TYPE_GO, sp.ProjectType)
			break
		}
	}
	assert.True(suite.T(), rootFound, "Root project not detected")
}

func (suite *WorkspaceEdgeCasesSuite) TestPermissionDeniedDirectories() {
	if os.Getenv("GOOS") == "windows" {
		suite.T().Skip("Skipping permission test on Windows")
	}

	// Create workspace with restricted permission directories
	workspacePath := filepath.Join(suite.tempDir, "permission-test")
	err := os.MkdirAll(workspacePath, 0755)
	require.NoError(suite.T(), err)

	// Create valid root project
	err = os.WriteFile(filepath.Join(workspacePath, "go.mod"), []byte(`module permission-test

go 1.24
`), 0644)
	require.NoError(suite.T(), err)

	// Create accessible project
	accessiblePath := filepath.Join(workspacePath, "accessible")
	err = os.MkdirAll(accessiblePath, 0755)
	require.NoError(suite.T(), err)
	
	err = os.WriteFile(filepath.Join(accessiblePath, "go.mod"), []byte(`module permission-test/accessible

go 1.24
`), 0644)
	require.NoError(suite.T(), err)

	// Create restricted directory
	restrictedPath := filepath.Join(workspacePath, "restricted")
	err = os.MkdirAll(restrictedPath, 0755)
	require.NoError(suite.T(), err)
	
	err = os.WriteFile(filepath.Join(restrictedPath, "go.mod"), []byte(`module permission-test/restricted

go 1.24
`), 0644)
	require.NoError(suite.T(), err)

	// Make directory unreadable
	err = os.Chmod(restrictedPath, 0000)
	require.NoError(suite.T(), err)
	
	// Restore permissions after test
	defer func() {
		os.Chmod(restrictedPath, 0755)
	}()

	// Detection should continue despite permission errors
	result, err := suite.detector.DetectWorkspaceAt(workspacePath)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)

	// Should detect accessible projects
	assert.Greater(suite.T(), len(result.SubProjects), 1)

	// Verify accessible project is detected
	accessibleFound := false
	for _, sp := range result.SubProjects {
		if sp.Name == "accessible" {
			accessibleFound = true
			break
		}
	}
	assert.True(suite.T(), accessibleFound, "Accessible project not detected")
}

func (suite *WorkspaceEdgeCasesSuite) TestMaxFilesLimitExceeded() {
	// Create workspace that exceeds MAX_FILES_TO_SCAN limit
	workspacePath := filepath.Join(suite.tempDir, "max-files-test")
	err := os.MkdirAll(workspacePath, 0755)
	require.NoError(suite.T(), err)

	// Create root project
	err = os.WriteFile(filepath.Join(workspacePath, "go.mod"), []byte(`module max-files-test

go 1.24
`), 0644)
	require.NoError(suite.T(), err)

	// Create many files to exceed the scan limit
	for i := 0; i < types.MAX_FILES_TO_SCAN+100; i++ {
		dirName := fmt.Sprintf("dir-%04d", i)
		dirPath := filepath.Join(workspacePath, dirName)
		err = os.MkdirAll(dirPath, 0755)
		require.NoError(suite.T(), err)

		// Every 50th directory gets a project marker
		if i%50 == 0 {
			err = os.WriteFile(filepath.Join(dirPath, "go.mod"), []byte(fmt.Sprintf(`module max-files-test/%s

go 1.24
`, dirName)), 0644)
			require.NoError(suite.T(), err)
		}

		// Create some files in each directory
		for j := 0; j < 3; j++ {
			fileName := fmt.Sprintf("file-%d.txt", j)
			err = os.WriteFile(filepath.Join(dirPath, fileName), []byte("content"), 0644)
			require.NoError(suite.T(), err)
		}
	}

	// Detection should handle file limit gracefully
	result, err := suite.detector.DetectWorkspaceAt(workspacePath)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)

	// Should detect some projects (limited by MAX_FILES_TO_SCAN)
	assert.Greater(suite.T(), len(result.SubProjects), 0)
	suite.T().Logf("Detected %d projects with file limit in effect", len(result.SubProjects))
}

func (suite *WorkspaceEdgeCasesSuite) TestMaxDepthLimitExceeded() {
	// Create very deep nested structure beyond maxDepth
	deepStructure := make(map[string]string)
	
	// Create root
	deepStructure["go.mod"] = `module depth-test

go 1.24
`

	// Create nested structure beyond default maxDepth (5)
	currentPath := ""
	for depth := 1; depth <= 10; depth++ {
		currentPath += fmt.Sprintf("/level-%d", depth)
		relativePath := currentPath[1:] // Remove leading slash
		
		deepStructure[relativePath+"/go.mod"] = fmt.Sprintf(`module depth-test%s

go 1.24
`, currentPath)
		deepStructure[relativePath+"/main.go"] = `package main

func main() {}
`
	}

	workspacePath := suite.helper.CreateNestedProject("depth-limit-test", deepStructure)

	result, err := suite.detector.DetectWorkspaceAt(workspacePath)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)

	// Should respect depth limit (maxDepth = 5, so max 6 projects including root)
	assert.LessOrEqual(suite.T(), len(result.SubProjects), 6,
		"Detected more projects than depth limit allows: %d", len(result.SubProjects))

	// Verify deepest detected project is within limit
	maxDepthFound := 0
	for _, sp := range result.SubProjects {
		depth := strings.Count(sp.RelativePath, string(filepath.Separator))
		if depth > maxDepthFound {
			maxDepthFound = depth
		}
	}
	assert.LessOrEqual(suite.T(), maxDepthFound, 5,
		"Found project deeper than expected limit: depth %d", maxDepthFound)
}

func (suite *WorkspaceEdgeCasesSuite) TestConcurrentModification() {
	// Create workspace
	workspacePath := suite.helper.CreateSimpleGoProject("concurrent-mod", map[string]string{
		"go.mod": `module concurrent-mod

go 1.24
`,
		"main.go": `package main

func main() {}
`,
	})

	// Start detection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Modify workspace during detection
	go func() {
		time.Sleep(10 * time.Millisecond)
		
		// Add a new project
		newProjectPath := filepath.Join(workspacePath, "new-service")
		os.MkdirAll(newProjectPath, 0755)
		os.WriteFile(filepath.Join(newProjectPath, "go.mod"), []byte(`module concurrent-mod/new-service

go 1.24
`), 0644)
		
		// Remove a file
		os.Remove(filepath.Join(workspacePath, "main.go"))
		
		// Create and remove directory
		tempDir := filepath.Join(workspacePath, "temp")
		os.MkdirAll(tempDir, 0755)
		time.Sleep(5 * time.Millisecond)
		os.RemoveAll(tempDir)
	}()

	// Detection should handle concurrent modifications gracefully
	result, err := suite.detector.DetectWorkspaceWithContext(ctx, workspacePath)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Greater(suite.T(), len(result.SubProjects), 0)
}

func (suite *WorkspaceEdgeCasesSuite) TestSpecialFileNames() {
	// Create workspace with special file names that might cause issues
	specialStructure := map[string]string{
		"go.mod": `module special-files

go 1.24
`,
		// Files with spaces
		"dir with spaces/go.mod": `module special-files/spaces

go 1.24
`,
		"dir with spaces/file with spaces.go": `package main

func main() {}
`,
		// Files with special characters
		"dir-with@special#chars$/package.json": `{
  "name": "special-chars",
  "version": "1.0.0"
}`,
		// Files starting with dots
		".hidden-dir/go.mod": `module special-files/hidden

go 1.24
`,
		// Files with very long names
		"dir-with-extremely-long-name-that-might-cause-issues-in-some-filesystems/go.mod": `module special-files/long-name

go 1.24
`,
		// Files with numbers and mixed case
		"Dir123_MixedCase/go.mod": `module special-files/mixed

go 1.24
`,
	}

	workspacePath := suite.helper.CreateWorkspaceWithSpecialNames("special-files", specialStructure)

	result, err := suite.detector.DetectWorkspaceAt(workspacePath)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)

	// Should handle special file names gracefully
	assert.Greater(suite.T(), len(result.SubProjects), 4)

	// Verify projects with special names are detected
	projectNames := make([]string, len(result.SubProjects))
	for i, sp := range result.SubProjects {
		projectNames[i] = sp.Name
	}

	expectedNames := []string{"special-files", "dir with spaces", "special-chars", "hidden", "long-name", "Dir123_MixedCase"}
	for _, expectedName := range expectedNames {
		found := false
		for _, name := range projectNames {
			if strings.Contains(name, expectedName) || name == expectedName {
				found = true
				break
			}
		}
		assert.True(suite.T(), found, "Expected special-named project not found: %s", expectedName)
	}
}

func (suite *WorkspaceEdgeCasesSuite) TestPathTraversalSecurity() {
	// Create workspace with path traversal attempts
	traversalStructure := map[string]string{
		"go.mod": `module traversal-test

go 1.24
`,
		"main.go": `package main

func main() {}
`,
		// Attempt directory traversal (should be handled safely)
		"../../../etc/passwd": "fake passwd file",
		"..\\..\\windows\\system32\\file": "fake windows file",
		"subdir/../../../outside": "file outside workspace",
	}

	// This should create files safely within the workspace
	workspacePath := suite.helper.CreateWorkspaceWithSpecialNames("traversal-test", traversalStructure)

	result, err := suite.detector.DetectWorkspaceAt(workspacePath)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)

	// Should detect legitimate projects only
	assert.Greater(suite.T(), len(result.SubProjects), 0)

	// Verify no files were created outside workspace
	outsideFiles := []string{
		filepath.Join(suite.tempDir, "etc", "passwd"),
		filepath.Join(suite.tempDir, "windows", "system32", "file"),
		filepath.Join(suite.tempDir, "outside"),
	}

	for _, outsideFile := range outsideFiles {
		_, err := os.Stat(outsideFile)
		assert.True(suite.T(), os.IsNotExist(err), "File should not exist outside workspace: %s", outsideFile)
	}
}

func (suite *WorkspaceEdgeCasesSuite) TestDetectionWithBrokenSymlinks() {
	if os.Getenv("GOOS") == "windows" {
		suite.T().Skip("Skipping symlink test on Windows")
	}

	// Create workspace with broken symlinks
	workspacePath := filepath.Join(suite.tempDir, "broken-symlinks")
	err := os.MkdirAll(workspacePath, 0755)
	require.NoError(suite.T(), err)

	// Create valid project
	err = os.WriteFile(filepath.Join(workspacePath, "go.mod"), []byte(`module broken-symlinks

go 1.24
`), 0644)
	require.NoError(suite.T(), err)

	// Create broken symlinks
	brokenLink1 := filepath.Join(workspacePath, "broken-link-1")
	brokenLink2 := filepath.Join(workspacePath, "broken-link-2")

	err = os.Symlink("/nonexistent/path", brokenLink1)
	require.NoError(suite.T(), err)
	
	err = os.Symlink("./also-nonexistent", brokenLink2)
	require.NoError(suite.T(), err)

	// Create symlink to existing file, then remove the target
	targetFile := filepath.Join(workspacePath, "target-file")
	err = os.WriteFile(targetFile, []byte("content"), 0644)
	require.NoError(suite.T(), err)

	linkToTarget := filepath.Join(workspacePath, "link-to-target")
	err = os.Symlink(targetFile, linkToTarget)
	require.NoError(suite.T(), err)

	// Remove target to break the link
	err = os.Remove(targetFile)
	require.NoError(suite.T(), err)

	// Detection should handle broken symlinks gracefully
	result, err := suite.detector.DetectWorkspaceAt(workspacePath)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Greater(suite.T(), len(result.SubProjects), 0)
}

func (suite *WorkspaceEdgeCasesSuite) TestZeroSizeAndBinaryFiles() {
	// Create workspace with zero-size and binary marker files
	binaryStructure := map[string]string{
		"go.mod": `module binary-test

go 1.24
`,
		"main.go": `package main

func main() {}
`,
		// Zero-size marker files
		"zero-size/go.mod":     "",
		"zero-size/package.json": "",
		
		// Binary marker files (invalid but should be handled)
		"binary/go.mod": string([]byte{
			0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
			0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
			0xFF, 0xFE, 0xFD, 0xFC, 0xFB, 0xFA, 0xF9, 0xF8,
		}),
	}

	workspacePath := suite.helper.CreateCorruptedWorkspace("binary-files", binaryStructure)

	// Detection should handle binary and zero-size files
	result, err := suite.detector.DetectWorkspaceAt(workspacePath)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)

	// Should detect projects based on file existence, not content
	assert.Greater(suite.T(), len(result.SubProjects), 2)

	// Root project should be valid
	rootFound := false
	for _, sp := range result.SubProjects {
		if sp.RelativePath == "." {
			rootFound = true
			assert.Equal(suite.T(), types.PROJECT_TYPE_GO, sp.ProjectType)
			break
		}
	}
	assert.True(suite.T(), rootFound, "Root project not detected")
}

func TestWorkspaceEdgeCasesSuite(t *testing.T) {
	suite.Run(t, new(WorkspaceEdgeCasesSuite))
}