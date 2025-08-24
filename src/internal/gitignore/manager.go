// Package gitignore provides gitignore-aware walking and filtering utilities.
package gitignore

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Manager struct {
	cache   map[string]*GitIgnore
	cacheMu sync.RWMutex
	rootDir string
	enabled bool
}

func NewManager(rootDir string) *Manager {
	return &Manager{
		cache:   make(map[string]*GitIgnore),
		rootDir: rootDir,
		enabled: true,
	}
}

func (m *Manager) SetEnabled(enabled bool) {
	m.enabled = enabled
}

func (m *Manager) IsEnabled() bool {
	return m.enabled
}

func (m *Manager) ShouldIgnore(path string) bool {
	if !m.enabled {
		return false
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return false
	}
	isDir := info.IsDir()

	currentDir := filepath.Dir(absPath)

	var gitignores []*GitIgnore
	for currentDir != "" && strings.HasPrefix(absPath, currentDir) {
		if currentDir == "/" || currentDir == filepath.Dir(currentDir) {
			if currentDir != m.rootDir {
				break
			}
		}

		gi := m.getOrLoadGitIgnore(currentDir)
		if gi != nil {
			gitignores = append(gitignores, gi)
		}

		if currentDir == m.rootDir {
			break
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			break
		}
		currentDir = parentDir
	}

	for i := len(gitignores) - 1; i >= 0; i-- {
		if gitignores[i].Match(absPath, isDir) {
			return true
		}
	}

	return false
}

func (m *Manager) getOrLoadGitIgnore(dir string) *GitIgnore {
	gitignorePath := filepath.Join(dir, ".gitignore")

	m.cacheMu.RLock()
	if gi, exists := m.cache[gitignorePath]; exists {
		m.cacheMu.RUnlock()
		return gi
	}
	m.cacheMu.RUnlock()

	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()

	if gi, exists := m.cache[gitignorePath]; exists {
		return gi
	}

	gi, err := ParseGitIgnoreFile(gitignorePath)
	if err == nil {
		m.cache[gitignorePath] = gi
		return gi
	}

	return nil
}

func (m *Manager) ClearCache() {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	m.cache = make(map[string]*GitIgnore)
}

func (m *Manager) LoadGitIgnoreForDirectory(dir string) error {
	gitignorePath := filepath.Join(dir, ".gitignore")
	gi, err := ParseGitIgnoreFile(gitignorePath)
	if err != nil {
		return err
	}

	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	m.cache[gitignorePath] = gi

	return nil
}

var defaultPatterns = []string{
	".git",
	".svn",
	".hg",
	".bzr",
	"node_modules",
	"vendor",
	"__pycache__",
	"*.pyc",
	"*.pyo",
	".DS_Store",
	"Thumbs.db",
	".idea",
	".vscode",
	"*.swp",
	"*.swo",
	"*~",
	".#*",
}

func (m *Manager) ShouldIgnoreDefault(path string) bool {
	base := filepath.Base(path)

	// Check base name against patterns
	for _, pattern := range defaultPatterns {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}

	// Also check if path contains default ignored directories
	pathParts := strings.Split(filepath.ToSlash(path), "/")
	for _, part := range pathParts {
		if part == ".git" || part == ".svn" || part == ".hg" || part == ".bzr" ||
			part == "node_modules" || part == "vendor" || part == "__pycache__" ||
			part == ".idea" || part == ".vscode" {
			return true
		}
	}

	return false
}
