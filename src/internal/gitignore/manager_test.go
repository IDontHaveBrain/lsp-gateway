package gitignore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagerShouldIgnore(t *testing.T) {
	tempDir := t.TempDir()

	rootGitignore := filepath.Join(tempDir, ".gitignore")
	err := os.WriteFile(rootGitignore, []byte("*.log\nnode_modules/\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(tempDir, "src")
	err = os.MkdirAll(subDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	subGitignore := filepath.Join(subDir, ".gitignore")
	err = os.WriteFile(subGitignore, []byte("*.tmp\nbuild/\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	manager := NewManager(tempDir)

	tests := []struct {
		path     string
		expected bool
		desc     string
	}{
		{filepath.Join(tempDir, "test.log"), true, "root .gitignore log pattern"},
		{filepath.Join(tempDir, "node_modules"), true, "root node_modules"},
		{filepath.Join(subDir, "test.log"), true, "inherited log pattern"},
		{filepath.Join(subDir, "test.tmp"), true, "sub .gitignore tmp pattern"},
		{filepath.Join(subDir, "build"), true, "sub build directory"},
		{filepath.Join(tempDir, "test.txt"), false, "non-matching file"},
		{filepath.Join(subDir, "main.go"), false, "non-matching in subdir"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			os.MkdirAll(filepath.Dir(tt.path), 0755)

			isDir := filepath.Base(tt.path) == "node_modules" || filepath.Base(tt.path) == "build"
			if isDir {
				os.MkdirAll(tt.path, 0755)
			} else {
				os.WriteFile(tt.path, []byte("test"), 0644)
			}

			result := manager.ShouldIgnore(tt.path)
			if result != tt.expected {
				t.Errorf("ShouldIgnore(%s) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestManagerNestedGitignore(t *testing.T) {
	tempDir := t.TempDir()

	rootGitignore := filepath.Join(tempDir, ".gitignore")
	err := os.WriteFile(rootGitignore, []byte("*.global\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	level1 := filepath.Join(tempDir, "level1")
	os.MkdirAll(level1, 0755)
	level1Gitignore := filepath.Join(level1, ".gitignore")
	err = os.WriteFile(level1Gitignore, []byte("*.level1\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	level2 := filepath.Join(level1, "level2")
	os.MkdirAll(level2, 0755)
	level2Gitignore := filepath.Join(level2, ".gitignore")
	err = os.WriteFile(level2Gitignore, []byte("*.level2\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	manager := NewManager(tempDir)

	tests := []struct {
		path     string
		expected bool
		desc     string
	}{
		{filepath.Join(tempDir, "test.global"), true, "root pattern"},
		{filepath.Join(level1, "test.global"), true, "inherited root pattern"},
		{filepath.Join(level2, "test.global"), true, "double inherited root pattern"},
		{filepath.Join(level1, "test.level1"), true, "level1 pattern"},
		{filepath.Join(level2, "test.level1"), true, "inherited level1 pattern"},
		{filepath.Join(level2, "test.level2"), true, "level2 pattern"},
		{filepath.Join(tempDir, "test.level2"), false, "level2 pattern shouldn't apply to root"},
		{filepath.Join(level1, "test.level2"), false, "level2 pattern shouldn't apply to level1"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			os.WriteFile(tt.path, []byte("test"), 0644)
			result := manager.ShouldIgnore(tt.path)
			if result != tt.expected {
				t.Errorf("ShouldIgnore(%s) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestManagerDefaultPatterns(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	tests := []struct {
		path     string
		expected bool
		desc     string
	}{
		{filepath.Join(tempDir, ".git"), true, ".git directory"},
		{filepath.Join(tempDir, ".svn"), true, ".svn directory"},
		{filepath.Join(tempDir, "node_modules"), true, "node_modules"},
		{filepath.Join(tempDir, "vendor"), true, "vendor directory"},
		{filepath.Join(tempDir, "__pycache__"), true, "Python cache"},
		{filepath.Join(tempDir, "file.pyc"), true, "Python compiled file"},
		{filepath.Join(tempDir, ".DS_Store"), true, "macOS file"},
		{filepath.Join(tempDir, "Thumbs.db"), true, "Windows file"},
		{filepath.Join(tempDir, ".idea"), true, "IDE directory"},
		{filepath.Join(tempDir, ".vscode"), true, "VSCode directory"},
		{filepath.Join(tempDir, "file.swp"), true, "Vim swap file"},
		{filepath.Join(tempDir, "normal.txt"), false, "normal file"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := manager.ShouldIgnoreDefault(tt.path)
			if result != tt.expected {
				t.Errorf("ShouldIgnoreDefault(%s) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestManagerCaching(t *testing.T) {
	tempDir := t.TempDir()
	gitignorePath := filepath.Join(tempDir, ".gitignore")

	err := os.WriteFile(gitignorePath, []byte("*.cache\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	manager := NewManager(tempDir)

	testFile := filepath.Join(tempDir, "test.cache")
	os.WriteFile(testFile, []byte("test"), 0644)

	if !manager.ShouldIgnore(testFile) {
		t.Error("First call should match")
	}

	if !manager.ShouldIgnore(testFile) {
		t.Error("Second call should use cache and match")
	}

	manager.ClearCache()

	if !manager.ShouldIgnore(testFile) {
		t.Error("After cache clear should still match")
	}
}

func TestManagerEnabled(t *testing.T) {
	tempDir := t.TempDir()
	gitignorePath := filepath.Join(tempDir, ".gitignore")

	err := os.WriteFile(gitignorePath, []byte("*.ignore\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	manager := NewManager(tempDir)
	testFile := filepath.Join(tempDir, "test.ignore")
	os.WriteFile(testFile, []byte("test"), 0644)

	if !manager.IsEnabled() {
		t.Error("Manager should be enabled by default")
	}

	if !manager.ShouldIgnore(testFile) {
		t.Error("Should ignore when enabled")
	}

	manager.SetEnabled(false)

	if manager.IsEnabled() {
		t.Error("Manager should be disabled")
	}

	if manager.ShouldIgnore(testFile) {
		t.Error("Should not ignore when disabled")
	}
}
