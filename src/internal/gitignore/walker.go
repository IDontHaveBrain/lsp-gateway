package gitignore

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type WalkFunc func(path string, d os.DirEntry, err error) error

type Walker struct {
	manager *Manager
}

func NewWalker(rootDir string) *Walker {
	return &Walker{
		manager: NewManager(rootDir),
	}
}

func (w *Walker) Walk(root string, fn WalkFunc) error {
	return w.walk(root, fn, 0, 3)
}

func (w *Walker) walk(root string, fn WalkFunc, depth, maxDepth int) error {
	if depth > maxDepth {
		return nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return fn(root, nil, err)
	}

	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())

		if w.manager.ShouldIgnore(path) {
			if entry.IsDir() {
				continue
			}
			continue
		}

		if w.manager.ShouldIgnoreDefault(path) {
			if entry.IsDir() {
				continue
			}
			continue
		}

		err := fn(path, entry, nil)
		if err != nil {
			if errors.Is(err, filepath.SkipDir) {
				continue
			}
			return err
		}

		if entry.IsDir() {
			if err := w.walk(path, fn, depth+1, maxDepth); err != nil && !errors.Is(err, filepath.SkipDir) {
				return err
			}
		}
	}

	return nil
}

func (w *Walker) WalkDir(root string, fn fs.WalkDirFunc) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fn(path, d, err)
		}

		if d != nil && w.manager.ShouldIgnore(path) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d != nil && w.manager.ShouldIgnoreDefault(path) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		return fn(path, d, err)
	})
}

func (w *Walker) SetEnabled(enabled bool) {
	w.manager.SetEnabled(enabled)
}

func (w *Walker) IsEnabled() bool {
	return w.manager.IsEnabled()
}

func IsGitIgnored(path string) bool {
	dir := filepath.Dir(path)
	manager := NewManager(dir)
	return manager.ShouldIgnore(path)
}

func FilterPaths(paths []string, rootDir string) []string {
	manager := NewManager(rootDir)
	var filtered []string

	for _, path := range paths {
		if !manager.ShouldIgnore(path) && !manager.ShouldIgnoreDefault(path) {
			filtered = append(filtered, path)
		}
	}

	return filtered
}

func ShouldSkipDirectory(name string) bool {
	skipDirs := []string{
		".git", ".svn", ".hg", ".bzr",
		"node_modules", "vendor",
		"__pycache__", "target",
		"build", "dist", "out",
		".idea", ".vscode",
	}

	for _, skip := range skipDirs {
		if name == skip {
			return true
		}
	}

	return strings.HasPrefix(name, ".") && name != "."
}
