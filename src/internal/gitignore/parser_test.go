package gitignore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseGitIgnoreFile(t *testing.T) {
	tempDir := t.TempDir()
	gitignorePath := filepath.Join(tempDir, ".gitignore")

	content := `# Comments should be ignored
*.log
/build
node_modules/
!important.log
src/**/*.tmp
*.swp
.DS_Store
`

	err := os.WriteFile(gitignorePath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	gi, err := ParseGitIgnoreFile(gitignorePath)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		path     string
		isDir    bool
		expected bool
		desc     string
	}{
		{filepath.Join(tempDir, "test.log"), false, true, "*.log pattern"},
		{filepath.Join(tempDir, "important.log"), false, false, "negation pattern"},
		{filepath.Join(tempDir, "build"), true, true, "/build pattern"},
		{filepath.Join(tempDir, "node_modules"), true, true, "directory pattern"},
		{filepath.Join(tempDir, "src/test.tmp"), false, true, "double asterisk pattern"},
		{filepath.Join(tempDir, "src/sub/test.tmp"), false, true, "nested double asterisk"},
		{filepath.Join(tempDir, "file.swp"), false, true, "swap file pattern"},
		{filepath.Join(tempDir, ".DS_Store"), false, true, "DS_Store pattern"},
		{filepath.Join(tempDir, "normal.txt"), false, false, "non-matching file"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := gi.Match(tt.path, tt.isDir)
			if result != tt.expected {
				t.Errorf("Match(%s, %v) = %v, want %v", tt.path, tt.isDir, result, tt.expected)
			}
		})
	}
}

func TestGitIgnoreEmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	gitignorePath := filepath.Join(tempDir, ".gitignore")

	err := os.WriteFile(gitignorePath, []byte(""), 0644)
	if err != nil {
		t.Fatal(err)
	}

	gi, err := ParseGitIgnoreFile(gitignorePath)
	if err != nil {
		t.Fatal(err)
	}

	testFile := filepath.Join(tempDir, "test.txt")
	if gi.Match(testFile, false) {
		t.Error("Empty gitignore should not match any files")
	}
}

func TestGitIgnoreNonExistentFile(t *testing.T) {
	tempDir := t.TempDir()
	gitignorePath := filepath.Join(tempDir, ".gitignore")

	gi, err := ParseGitIgnoreFile(gitignorePath)
	if err != nil {
		t.Fatal("Should not error on non-existent file")
	}

	testFile := filepath.Join(tempDir, "test.txt")
	if gi.Match(testFile, false) {
		t.Error("Non-existent gitignore should not match any files")
	}
}

func TestComplexPatterns(t *testing.T) {
	tempDir := t.TempDir()
	gitignorePath := filepath.Join(tempDir, ".gitignore")

	content := `# Complex patterns
**/node_modules
**/.git
**/build/
target/
*.class
*.jar
!lib/*.jar
src/**/temp/
*.py[cod]
__pycache__/
`

	err := os.WriteFile(gitignorePath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	gi, err := ParseGitIgnoreFile(gitignorePath)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		path     string
		isDir    bool
		expected bool
		desc     string
	}{
		{filepath.Join(tempDir, "node_modules"), true, true, "root node_modules"},
		{filepath.Join(tempDir, "src/node_modules"), true, true, "nested node_modules"},
		{filepath.Join(tempDir, ".git"), true, true, "git directory"},
		{filepath.Join(tempDir, "src/.git"), true, true, "nested git directory"},
		{filepath.Join(tempDir, "build"), true, true, "build directory"},
		{filepath.Join(tempDir, "target"), true, true, "target directory"},
		{filepath.Join(tempDir, "Test.class"), false, true, "class file"},
		{filepath.Join(tempDir, "app.jar"), false, true, "jar file"},
		{filepath.Join(tempDir, "lib/needed.jar"), false, false, "negated jar in lib"},
		{filepath.Join(tempDir, "src/test/temp"), true, true, "temp directory pattern"},
		{filepath.Join(tempDir, "test.pyc"), false, true, "Python compiled file"},
		{filepath.Join(tempDir, "__pycache__"), true, true, "Python cache directory"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := gi.Match(tt.path, tt.isDir)
			if result != tt.expected {
				t.Errorf("Match(%s, %v) = %v, want %v", tt.path, tt.isDir, result, tt.expected)
			}
		})
	}
}
