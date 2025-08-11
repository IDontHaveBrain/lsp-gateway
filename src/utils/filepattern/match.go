package filepattern

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"lsp-gateway/src/utils"
)

// Match reports whether a given file path or file URI matches the provided glob pattern.
// Supported patterns:
// - "**/*" or "*": match all
// - standard glob wildcards (*, ?, [class])
// - directory prefix with trailing slash (e.g. "src/")
// - directory paths with patterns (e.g. "src/server/*.go", "src/**/*.go")
// The matcher normalizes absolute paths to a relative path against the current
// working directory when the pattern is relative, and also tests the basename.
func Match(pathOrURI, pattern string) bool {
	if pattern == "" || pattern == "**/*" || pattern == "*" {
		return true
	}

	// Normalize potential file URI to filesystem path
	filePath := utils.URIToFilePath(pathOrURI)

	// Normalize Windows backslashes to forward slashes for consistent matching
	// Note: filepath.ToSlash doesn't work on Linux for backslash paths, so we replace manually
	filePath = strings.ReplaceAll(filePath, "\\", "/")
	pattern = strings.ReplaceAll(pattern, "\\", "/")

	// If the pattern ends with a slash, treat as directory prefix
	if strings.HasSuffix(pattern, "/") {
		return strings.HasPrefix(filePath, pattern) || strings.HasPrefix(normalizeRelative(filePath), pattern)
	}

	// If pattern is relative and path is absolute, also try relative to cwd
	candidates := []string{filePath}
	rel := normalizeRelative(filePath)
	if rel != filePath {
		candidates = append(candidates, rel)
	}

	for _, cand := range candidates {
		// Direct full path match (using path package for consistent slash handling)
		if ok, _ := path.Match(pattern, cand); ok {
			return true
		}

		// Check if pattern contains directory separators
		if strings.Contains(pattern, "/") {
			// Handle patterns like "src/server/*.go" or "path/to/*.txt"
			// Split pattern into directory and file pattern parts using path package
			patternDir := path.Dir(pattern)
			patternFile := path.Base(pattern)

			// Check if the candidate's directory matches the pattern directory
			// and the filename matches the file pattern
			candDir := path.Dir(cand)
			candFile := path.Base(cand)

			// For patterns like "src/server/*.go", we need exact directory match
			if patternDir != "." && !strings.Contains(patternDir, "*") {
				// Check both absolute and relative paths
				// candDir could be absolute: /home/user/project/src/server
				// patternDir is relative: src/server
				// We need to check if candDir ends with patternDir as a complete path segment
				matched := false
				if candDir == patternDir {
					matched = true
				} else if strings.HasSuffix(candDir, "/"+patternDir) {
					matched = true
				} else if strings.HasSuffix(candDir, patternDir) {
					// Ensure it's a complete path segment, not partial match
					idx := strings.LastIndex(candDir, patternDir)
					if idx > 0 && candDir[idx-1] == '/' {
						matched = true
					} else if idx == 0 {
						matched = true
					}
				}

				if matched {
					if ok, _ := path.Match(patternFile, candFile); ok {
						return true
					}
				}
			}
		} else {
			// Pattern without directory separator - match against basename only
			if ok, _ := path.Match(pattern, path.Base(cand)); ok {
				return true
			}
		}

		// Handle ** split pattern: prefix**/suffix simplified check
		if strings.Contains(pattern, "**") {
			parts := strings.Split(pattern, "**")
			if len(parts) == 2 {
				prefix := strings.TrimSuffix(parts[0], "/")
				suffix := strings.TrimPrefix(parts[1], "/")
				matches := true
				if prefix != "" && !strings.Contains(cand, prefix) {
					matches = false
				}
				if matches && suffix != "" && suffix != "*" {
					// Handle suffix with directory paths like "server/*.go"
					if strings.Contains(suffix, "/") {
						suffixDir := path.Dir(suffix)
						suffixFile := path.Base(suffix)
						candSuffix := cand
						if prefix != "" {
							idx := strings.LastIndex(cand, prefix)
							if idx >= 0 {
								candSuffix = cand[idx+len(prefix):]
								candSuffix = strings.TrimPrefix(candSuffix, "/")
							}
						}
						if suffixDir != "." {
							if !strings.Contains(candSuffix, suffixDir) {
								matches = false
							}
						}
						if matches {
							if ok, _ := path.Match(suffixFile, path.Base(cand)); !ok {
								matches = false
							}
						}
					} else {
						// Simple file pattern suffix
						if ok, _ := path.Match(suffix, path.Base(cand)); !ok {
							matches = false
						}
					}
				}
				if matches {
					return true
				}
			}
		}
	}
	return false
}

func normalizeRelative(path string) string {
	// Already slash-normalized from caller
	if !filepath.IsAbs(filepath.FromSlash(path)) {
		return path
	}

	// Try to find the project root by looking for go.mod
	wd, err := os.Getwd()
	if err != nil {
		return path
	}

	// Start from current directory and walk up to find project root
	projectRoot := wd
	for {
		if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
			// Found go.mod, this is the project root
			break
		}
		parent := filepath.Dir(projectRoot)
		if parent == projectRoot {
			// Reached filesystem root, use original working directory
			projectRoot = wd
			break
		}
		projectRoot = parent
	}

	// Convert back to native path for Rel calculation
	nativePath := filepath.FromSlash(path)
	if rel, err := filepath.Rel(projectRoot, nativePath); err == nil {
		// Convert result back to forward slashes
		return filepath.ToSlash(rel)
	}

	return path
}
