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
	if pattern == "" || pattern == "**/*" || pattern == "*" || pattern == "." || pattern == "./" {
		return true
	}
	filePath, pattern := normalizeForMatch(pathOrURI, pattern)
	if strings.HasSuffix(pattern, "/") {
		dir := strings.TrimSuffix(pattern, "/")
		rel := normalizeRelative(filePath)
		if strings.HasPrefix(filePath, pattern) || strings.HasPrefix(rel, pattern) {
			return true
		}
		return strings.Contains("/"+filePath+"/", "/"+dir+"/") || strings.Contains("/"+rel+"/", "/"+dir+"/")
	}
	candidates := enumerateCandidates(filePath)
	for _, cand := range candidates {
		if ok, _ := path.Match(pattern, cand); ok {
			return true
		}
		if matchPattern(cand, pattern) {
			return true
		}
		if matchWithDoubleStar(cand, pattern) {
			return true
		}
	}
	return false
}
func normalizeForMatch(pathOrURI, pattern string) (string, string) {
	fp := utils.URIToFilePathCached(pathOrURI)
	fp = strings.ReplaceAll(fp, "\\", "/")
	p := strings.ReplaceAll(pattern, "\\", "/")
	return fp, p
}
func enumerateCandidates(filePath string) []string {
	rel := normalizeRelative(filePath)
	if rel != filePath {
		return []string{filePath, rel}
	}
	return []string{filePath}
}
func matchPattern(cand, pattern string) bool {
	if strings.Contains(pattern, "/") {
		patternDir := path.Dir(pattern)
		patternFile := path.Base(pattern)
		candDir := path.Dir(cand)
		candFile := path.Base(cand)
		if patternDir != "." && !strings.Contains(patternDir, "*") {
			matched := false
			if candDir == patternDir || strings.HasSuffix(candDir, "/"+patternDir) {
				matched = true
			} else if strings.HasSuffix(candDir, patternDir) {
				idx := strings.LastIndex(candDir, patternDir)
				if idx > 0 && candDir[idx-1] == '/' || idx == 0 {
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
		if ok, _ := path.Match(pattern, path.Base(cand)); ok {
			return true
		}
		if !strings.ContainsAny(pattern, "*?[]") && !strings.Contains(pattern, "/") && !strings.Contains(pattern, ".") {
			if strings.Contains("/"+cand+"/", "/"+pattern+"/") {
				return true
			}
		}
	}
	return false
}
func matchWithDoubleStar(cand, pattern string) bool {
	if !strings.Contains(pattern, "**") {
		return false
	}
	parts := strings.Split(pattern, "**")
	if len(parts) != 2 {
		return false
	}
	prefix := strings.TrimSuffix(parts[0], "/")
	suffix := strings.TrimPrefix(parts[1], "/")
	matches := prefix == "" || strings.Contains(cand, prefix)
	if matches && suffix != "" && suffix != "*" {
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
			if ok, _ := path.Match(suffix, path.Base(cand)); !ok {
				matches = false
			}
		}
	}
	return matches
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
