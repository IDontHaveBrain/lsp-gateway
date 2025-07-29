package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"lsp-gateway/internal/workspace"
	"lsp-gateway/tests/integration/workspace/helpers"
)

type WorkspaceValidationSuite struct {
	suite.Suite
	tempDir   string
	detector  workspace.WorkspaceDetector
	helper    *helpers.WorkspaceTestHelper
}

func (suite *WorkspaceValidationSuite) SetupSuite() {
	tempDir, err := os.MkdirTemp("", "workspace-validation-test-*")
	require.NoError(suite.T(), err)
	
	suite.tempDir = tempDir
	suite.detector = workspace.NewWorkspaceDetector()
	suite.helper = helpers.NewWorkspaceTestHelper(tempDir)
}

func (suite *WorkspaceValidationSuite) TearDownSuite() {
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
}

func (suite *WorkspaceValidationSuite) TestValidateValidWorkspace() {
	// Create valid workspace
	validPath := suite.helper.CreateSimpleGoProject("valid-workspace", map[string]string{
		"go.mod": `module valid-workspace

go 1.24
`,
		"main.go": `package main

func main() {}
`,
	})

	err := suite.detector.ValidateWorkspace(validPath)
	assert.NoError(suite.T(), err)

	// Should also work with absolute path
	absPath, err := filepath.Abs(validPath)
	require.NoError(suite.T(), err)
	
	err = suite.detector.ValidateWorkspace(absPath)
	assert.NoError(suite.T(), err)
}

func (suite *WorkspaceValidationSuite) TestValidateEmptyPath() {
	err := suite.detector.ValidateWorkspace("")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "workspace root cannot be empty")
}

func (suite *WorkspaceValidationSuite) TestValidateNonexistentPath() {
	nonexistentPath := filepath.Join(suite.tempDir, "nonexistent", "path")
	
	err := suite.detector.ValidateWorkspace(nonexistentPath)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "workspace directory does not exist")
}

func (suite *WorkspaceValidationSuite) TestValidateFileAsWorkspace() {
	// Create a file instead of directory
	filePath := filepath.Join(suite.tempDir, "not-a-directory.txt")
	err := os.WriteFile(filePath, []byte("content"), 0644)
	require.NoError(suite.T(), err)

	err = suite.detector.ValidateWorkspace(filePath)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "workspace path is not a directory")
}

func (suite *WorkspaceValidationSuite) TestValidateUnreadableDirectory() {
	// Create directory with restricted permissions
	restrictedPath := filepath.Join(suite.tempDir, "restricted")
	err := os.MkdirAll(restrictedPath, 0755)
	require.NoError(suite.T(), err)

	// Change permissions to make it unreadable (if not Windows)
	if os.Getenv("GOOS") != "windows" {
		err = os.Chmod(restrictedPath, 0000)
		require.NoError(suite.T(), err)
		
		// Restore permissions after test
		defer func() {
			os.Chmod(restrictedPath, 0755)
		}()

		err = suite.detector.ValidateWorkspace(restrictedPath)
		assert.Error(suite.T(), err)
		assert.Contains(suite.T(), err.Error(), "cannot read workspace directory")
	}
}

func (suite *WorkspaceValidationSuite) TestValidateEmptyDirectory() {
	// Create empty directory
	emptyPath := filepath.Join(suite.tempDir, "empty")
	err := os.MkdirAll(emptyPath, 0755)
	require.NoError(suite.T(), err)

	// Empty directory should still be valid
	err = suite.detector.ValidateWorkspace(emptyPath)
	assert.NoError(suite.T(), err)
}

func (suite *WorkspaceValidationSuite) TestValidateDirectoryWithSpecialCharacters() {
	// Create directory with special characters
	specialPath := filepath.Join(suite.tempDir, "special dir with spaces & symbols!")
	err := os.MkdirAll(specialPath, 0755)
	require.NoError(suite.T(), err)

	// Create some content
	err = os.WriteFile(filepath.Join(specialPath, "file.txt"), []byte("content"), 0644)
	require.NoError(suite.T(), err)

	err = suite.detector.ValidateWorkspace(specialPath)
	assert.NoError(suite.T(), err)
}

func (suite *WorkspaceValidationSuite) TestValidateSymbolicLinks() {
	// Create actual directory
	actualPath := filepath.Join(suite.tempDir, "actual")
	err := os.MkdirAll(actualPath, 0755)
	require.NoError(suite.T(), err)

	// Create some content
	err = os.WriteFile(filepath.Join(actualPath, "file.txt"), []byte("content"), 0644)
	require.NoError(suite.T(), err)

	// Create symbolic link (if not Windows)
	if os.Getenv("GOOS") != "windows" {
		linkPath := filepath.Join(suite.tempDir, "symlink")
		err = os.Symlink(actualPath, linkPath)
		require.NoError(suite.T(), err)

		// Validation should work with symbolic links
		err = suite.detector.ValidateWorkspace(linkPath)
		assert.NoError(suite.T(), err)
	}
}

func (suite *WorkspaceValidationSuite) TestValidateWorkspaceIntegration() {
	// Test validation as part of workspace detection
	validPath := suite.helper.CreateSimpleGoProject("integration-test", map[string]string{
		"go.mod": `module integration-test

go 1.24
`,
		"main.go": `package main

func main() {}
`,
	})

	// Detection should succeed for valid workspace
	result, err := suite.detector.DetectWorkspaceAt(validPath)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)

	// Detection should fail for invalid workspace
	nonexistentPath := filepath.Join(suite.tempDir, "nonexistent")
	result, err = suite.detector.DetectWorkspaceAt(nonexistentPath)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "workspace validation failed")
}

func (suite *WorkspaceValidationSuite) TestValidateCorruptedMarkerFiles() {
	// Create workspace with corrupted marker files
	corruptedPath := suite.helper.CreateCorruptedWorkspace("corrupted-workspace", map[string]string{
		"go.mod": "invalid go.mod content that cannot be parsed",
		"main.go": `package main

func main() {}
`,
		"package.json": `{
  "name": "corrupted"
  // missing comma, invalid JSON
  "version": "1.0.0"
}`,
		"pyproject.toml": `[project]
name = [invalid toml syntax
`,
	})

	// Validation should still pass (we don't validate file contents, just directory structure)
	err := suite.detector.ValidateWorkspace(corruptedPath)
	assert.NoError(suite.T(), err)

	// But detection should work and handle corrupted files gracefully
	result, err := suite.detector.DetectWorkspaceAt(corruptedPath)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	
	// Should still detect marker files even if content is corrupted
	assert.Greater(suite.T(), len(result.SubProjects), 0)
}

func (suite *WorkspaceValidationSuite) TestValidateNestedSymlinks() {
	// Create nested structure with symlinks (if not Windows)
	if os.Getenv("GOOS") != "windows" {
		// Create actual directory structure
		actualPath := filepath.Join(suite.tempDir, "actual")
		nestedPath := filepath.Join(actualPath, "nested")
		err := os.MkdirAll(nestedPath, 0755)
		require.NoError(suite.T(), err)

		// Create content
		err = os.WriteFile(filepath.Join(nestedPath, "file.txt"), []byte("content"), 0644)
		require.NoError(suite.T(), err)

		// Create symbolic link to nested directory
		linkPath := filepath.Join(suite.tempDir, "link-to-nested")
		err = os.Symlink(nestedPath, linkPath)
		require.NoError(suite.T(), err)

		// Validation should work
		err = suite.detector.ValidateWorkspace(linkPath)
		assert.NoError(suite.T(), err)
	}
}

func (suite *WorkspaceValidationSuite) TestValidateCircularSymlinks() {
	// Create circular symbolic links (if not Windows)
	if os.Getenv("GOOS") != "windows" {
		linkA := filepath.Join(suite.tempDir, "link-a")
		linkB := filepath.Join(suite.tempDir, "link-b")

		// Create circular links: A -> B -> A
		err := os.Symlink(linkB, linkA)
		require.NoError(suite.T(), err)
		
		err = os.Symlink(linkA, linkB)
		require.NoError(suite.T(), err)

		// Validation should handle circular links gracefully
		err = suite.detector.ValidateWorkspace(linkA)
		assert.Error(suite.T(), err) // Should detect the circular reference or fail to read
	}
}

func (suite *WorkspaceValidationSuite) TestValidateVeryDeepDirectoryStructure() {
	// Create very deep directory structure
	deepPath := suite.tempDir
	for i := 0; i < 50; i++ {
		deepPath = filepath.Join(deepPath, "level"+string(rune('0'+i%10)))
	}
	
	err := os.MkdirAll(deepPath, 0755)
	require.NoError(suite.T(), err)

	// Create content at the deep level
	err = os.WriteFile(filepath.Join(deepPath, "deep-file.txt"), []byte("deep content"), 0644)
	require.NoError(suite.T(), err)

	// Validation should work even for very deep paths
	err = suite.detector.ValidateWorkspace(deepPath)
	assert.NoError(suite.T(), err)
}

func (suite *WorkspaceValidationSuite) TestValidateWorkspaceWithManyFiles() {
	// Create workspace with many files
	manyFilesPath := filepath.Join(suite.tempDir, "many-files")
	err := os.MkdirAll(manyFilesPath, 0755)
	require.NoError(suite.T(), err)

	// Create 1000 files
	for i := 0; i < 1000; i++ {
		filename := filepath.Join(manyFilesPath, "file"+string(rune('0'+i%10))+".txt")
		err = os.WriteFile(filename, []byte("content"), 0644)
		require.NoError(suite.T(), err)
	}

	// Validation should work with many files
	err = suite.detector.ValidateWorkspace(manyFilesPath)
	assert.NoError(suite.T(), err)
}

func (suite *WorkspaceValidationSuite) TestValidateWorkspaceWithLongFilenames() {
	// Create workspace with very long filenames
	longNamesPath := filepath.Join(suite.tempDir, "long-names")
	err := os.MkdirAll(longNamesPath, 0755)
	require.NoError(suite.T(), err)

	// Create file with very long name (but within filesystem limits)
	longName := "this-is-a-very-long-filename-that-tests-filesystem-limits-" +
		"and-should-still-work-correctly-with-our-validation-logic-" +
		"even-though-it-is-quite-long-and-unusual.txt"
	
	longFilePath := filepath.Join(longNamesPath, longName)
	err = os.WriteFile(longFilePath, []byte("content"), 0644)
	require.NoError(suite.T(), err)

	// Validation should work with long filenames
	err = suite.detector.ValidateWorkspace(longNamesPath)
	assert.NoError(suite.T(), err)
}

func (suite *WorkspaceValidationSuite) TestValidateWorkspaceWithUnicodeNames() {
	// Create workspace with unicode characters in names
	unicodePath := filepath.Join(suite.tempDir, "unicode-测试-🚀")
	err := os.MkdirAll(unicodePath, 0755)
	require.NoError(suite.T(), err)

	// Create files with unicode names
	unicodeFiles := []string{
		"测试文件.go",
		"файл.py",
		"ファイル.js",
		"🚀rocket.ts",
		"café-münchen.java",
	}

	for _, filename := range unicodeFiles {
		filePath := filepath.Join(unicodePath, filename)
		err = os.WriteFile(filePath, []byte("content"), 0644)
		require.NoError(suite.T(), err)
	}

	// Validation should work with unicode names
	err = suite.detector.ValidateWorkspace(unicodePath)
	assert.NoError(suite.T(), err)
}

func (suite *WorkspaceValidationSuite) TestValidateWorkspaceRecovery() {
	// Test workspace validation recovery scenarios
	recoveryPath := suite.helper.CreatePartiallyCorruptedWorkspace("recovery-test", map[string]string{
		"go.mod": `module recovery-test

go 1.24
`,
		"main.go": `package main

func main() {}
`,
		"corrupted-dir/.keep": "",  // This will create a directory
	})

	// Make one subdirectory unreadable (if not Windows)
	corruptedDir := filepath.Join(recoveryPath, "corrupted-dir")
	if os.Getenv("GOOS") != "windows" {
		err := os.Chmod(corruptedDir, 0000)
		require.NoError(suite.T(), err)
		
		defer func() {
			os.Chmod(corruptedDir, 0755)
		}()
	}

	// Validation should still work on the main directory
	err := suite.detector.ValidateWorkspace(recoveryPath)
	assert.NoError(suite.T(), err)

	// Detection should work and handle corrupted subdirectories gracefully
	result, err := suite.detector.DetectWorkspaceAt(recoveryPath)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Greater(suite.T(), len(result.SubProjects), 0)
}

func TestWorkspaceValidationSuite(t *testing.T) {
	suite.Run(t, new(WorkspaceValidationSuite))
}