package gitignore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWalkerWithGitignore(t *testing.T) {
	tempDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tempDir, ".gitignore"), []byte("*.ignore\nignored/\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	os.WriteFile(filepath.Join(tempDir, "included.txt"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tempDir, "excluded.ignore"), []byte("test"), 0644)

	ignoredDir := filepath.Join(tempDir, "ignored")
	os.MkdirAll(ignoredDir, 0755)
	os.WriteFile(filepath.Join(ignoredDir, "file.txt"), []byte("test"), 0644)

	includedDir := filepath.Join(tempDir, "included")
	os.MkdirAll(includedDir, 0755)
	os.WriteFile(filepath.Join(includedDir, "file.txt"), []byte("test"), 0644)

	walker := NewWalker(tempDir)

	var visited []string
	err = walker.WalkDir(tempDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(tempDir, path)
		if relPath != "." {
			visited = append(visited, relPath)
		}
		return nil
	})

	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]bool{
		".gitignore":        true,
		"included.txt":      true,
		"included":          true,
		"included/file.txt": true,
	}

	notExpected := map[string]bool{
		"excluded.ignore":  true,
		"ignored":          true,
		"ignored/file.txt": true,
	}

	for _, v := range visited {
		if notExpected[v] {
			t.Errorf("Should not have visited: %s", v)
		}
	}

	visitedMap := make(map[string]bool)
	for _, v := range visited {
		visitedMap[v] = true
	}

	for exp := range expected {
		if !visitedMap[exp] {
			t.Errorf("Should have visited: %s", exp)
		}
	}
}

func TestWalkerFilterPaths(t *testing.T) {
	tempDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tempDir, ".gitignore"), []byte("*.log\n*.tmp\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	paths := []string{
		filepath.Join(tempDir, "main.go"),
		filepath.Join(tempDir, "test.log"),
		filepath.Join(tempDir, "data.tmp"),
		filepath.Join(tempDir, "README.md"),
		filepath.Join(tempDir, ".git/config"),
		filepath.Join(tempDir, "node_modules/package.json"),
	}

	for _, p := range paths {
		os.MkdirAll(filepath.Dir(p), 0755)
		os.WriteFile(p, []byte("test"), 0644)
	}

	filtered := FilterPaths(paths, tempDir)

	expected := []string{
		filepath.Join(tempDir, "main.go"),
		filepath.Join(tempDir, "README.md"),
	}

	if len(filtered) != len(expected) {
		t.Errorf("Expected %d paths, got %d", len(expected), len(filtered))
		t.Logf("Filtered paths: %v", filtered)
	}

	filteredMap := make(map[string]bool)
	for _, f := range filtered {
		filteredMap[f] = true
	}

	for _, exp := range expected {
		if !filteredMap[exp] {
			t.Errorf("Expected path not found: %s", exp)
		}
	}
}

func TestWalkerShouldSkipDirectory(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{".git", true},
		{".svn", true},
		{"node_modules", true},
		{"vendor", true},
		{"__pycache__", true},
		{".hidden", true},
		{"normal", false},
		{"src", false},
		{".", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldSkipDirectory(tt.name)
			if result != tt.expected {
				t.Errorf("ShouldSkipDirectory(%s) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestWalkerIsGitIgnored(t *testing.T) {
	tempDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tempDir, ".gitignore"), []byte("*.secret\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	secretFile := filepath.Join(tempDir, "config.secret")
	normalFile := filepath.Join(tempDir, "config.yaml")

	os.WriteFile(secretFile, []byte("secret"), 0644)
	os.WriteFile(normalFile, []byte("normal"), 0644)

	if !IsGitIgnored(secretFile) {
		t.Error("Secret file should be ignored")
	}

	if IsGitIgnored(normalFile) {
		t.Error("Normal file should not be ignored")
	}
}

func TestWalkerMaxDepth(t *testing.T) {
	tempDir := t.TempDir()

	deep := filepath.Join(tempDir, "a/b/c/d/e")
	os.MkdirAll(deep, 0755)
	os.WriteFile(filepath.Join(deep, "deep.txt"), []byte("test"), 0644)

	shallow := filepath.Join(tempDir, "a/b")
	os.WriteFile(filepath.Join(shallow, "shallow.txt"), []byte("test"), 0644)

	walker := NewWalker(tempDir)

	var visited []string
	err := walker.Walk(tempDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			relPath, _ := filepath.Rel(tempDir, path)
			visited = append(visited, relPath)
		}
		return nil
	})

	if err != nil {
		t.Fatal(err)
	}

	foundShallow := false
	foundDeep := false

	for _, v := range visited {
		if v == "a/b/shallow.txt" {
			foundShallow = true
		}
		if v == "a/b/c/d/e/deep.txt" {
			foundDeep = true
		}
	}

	if !foundShallow {
		t.Error("Should have found shallow file within depth limit")
	}

	if foundDeep {
		t.Error("Should not have found deep file beyond depth limit")
	}
}
