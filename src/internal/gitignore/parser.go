package gitignore

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type Pattern struct {
	pattern  string
	negation bool
	dirOnly  bool
	absPath  bool
}

type GitIgnore struct {
	patterns []Pattern
	baseDir  string
}

func NewGitIgnore(baseDir string) *GitIgnore {
	return &GitIgnore{
		patterns: []Pattern{},
		baseDir:  baseDir,
	}
}

func ParseGitIgnoreFile(filePath string) (*GitIgnore, error) {
	baseDir := filepath.Dir(filePath)
	gi := NewGitIgnore(baseDir)

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return gi, nil
		}
		return nil, err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		pattern := parsePattern(line)
		if pattern != nil {
			gi.patterns = append(gi.patterns, *pattern)
		}
	}

	return gi, scanner.Err()
}

func parsePattern(line string) *Pattern {
	if line == "" {
		return nil
	}

	pattern := &Pattern{}

	if strings.HasPrefix(line, "!") {
		pattern.negation = true
		line = line[1:]
	}

	if strings.HasPrefix(line, "\\!") || strings.HasPrefix(line, "\\#") {
		line = line[1:]
	}

	if strings.HasPrefix(line, "/") {
		pattern.absPath = true
		line = line[1:]
	}

	if strings.HasSuffix(line, "/") {
		pattern.dirOnly = true
		line = strings.TrimSuffix(line, "/")
	}

	if line == "" {
		return nil
	}

	pattern.pattern = line

	return pattern
}

func (gi *GitIgnore) Match(path string, isDir bool) bool {
	if gi == nil || len(gi.patterns) == 0 {
		return false
	}

	relPath, err := filepath.Rel(gi.baseDir, path)
	if err != nil {
		return false
	}

	relPath = filepath.ToSlash(relPath)
	parts := strings.Split(relPath, "/")

	matched := false
	for _, pattern := range gi.patterns {
		if pattern.dirOnly && !isDir {
			continue
		}

		isMatch := matchPattern(pattern, relPath, parts, isDir)

		if isMatch {
			if pattern.negation {
				matched = false
			} else {
				matched = true
			}
		}
	}

	return matched
}

func matchPattern(pattern Pattern, relPath string, parts []string, isDir bool) bool {
	patternStr := pattern.pattern

	if strings.Contains(patternStr, "**") {
		return matchDoubleAsterisk(patternStr, relPath)
	}

	if pattern.absPath {
		return matchSimplePattern(patternStr, relPath)
	}

	for i, part := range parts {
		subPath := strings.Join(parts[i:], "/")
		if matchSimplePattern(patternStr, subPath) {
			return true
		}
		if matchSimplePattern(patternStr, part) {
			return true
		}
	}

	return false
}

func matchDoubleAsterisk(pattern, path string) bool {
	// Handle ** pattern which matches zero or more directories
	if pattern == "**" {
		return true
	}

	// Convert pattern to regex-like format
	// Split by ** to handle them separately
	parts := strings.Split(pattern, "**/")
	if len(parts) > 1 {
		// Pattern starts with **/
		if strings.HasPrefix(pattern, "**/") {
			suffix := strings.Join(parts[1:], "**/")
			// Check if path ends with the suffix pattern
			pathParts := strings.Split(path, "/")
			for i := range pathParts {
				subPath := strings.Join(pathParts[i:], "/")
				if matchSimplePattern(suffix, subPath) {
					return true
				}
			}
		}
		// Pattern contains **/ in the middle
		for i := 0; i < len(parts)-1; i++ {
			prefix := parts[i]
			suffix := strings.Join(parts[i+1:], "**/")

			if strings.HasPrefix(path, prefix) {
				remaining := strings.TrimPrefix(path, prefix)
				remaining = strings.TrimPrefix(remaining, "/")

				// Try matching suffix at any depth
				pathParts := strings.Split(remaining, "/")
				for j := range pathParts {
					subPath := strings.Join(pathParts[j:], "/")
					if matchSimplePattern(suffix, subPath) {
						return true
					}
				}
			}
		}
	}

	// Simple pattern matching for patterns without **/
	return matchSimplePattern(pattern, path)
}

func matchSimplePattern(pattern, name string) bool {
	matched, _ := filepath.Match(pattern, name)
	if matched {
		return true
	}

	if strings.Contains(pattern, "/") {
		matched, _ = filepath.Match(pattern, name)
		return matched
	}

	return false
}
