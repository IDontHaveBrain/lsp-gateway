package filepattern

import (
    "os"
    "path/filepath"
    "strings"

    "lsp-gateway/src/utils"
)

// Match reports whether a given file path or file URI matches the provided glob pattern.
// Supported patterns:
// - "**/*" or "*": match all
// - standard glob wildcards (*, ?, [class])
// - directory prefix with trailing slash (e.g. "src/")
// The matcher normalizes absolute paths to a relative path against the current
// working directory when the pattern is relative, and also tests the basename.
func Match(pathOrURI, pattern string) bool {
    if pattern == "" || pattern == "**/*" || pattern == "*" {
        return true
    }

    // Normalize potential file URI to filesystem path
    filePath := utils.URIToFilePath(pathOrURI)

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
        if ok, _ := filepath.Match(pattern, cand); ok {
            return true
        }
        if ok, _ := filepath.Match(pattern, filepath.Base(cand)); ok {
            return true
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
                    if ok, _ := filepath.Match(suffix, filepath.Base(cand)); !ok {
                        matches = false
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
    if !filepath.IsAbs(path) {
        return path
    }
    if wd, err := os.Getwd(); err == nil {
        if rel, err := filepath.Rel(wd, path); err == nil {
            return rel
        }
    }
    return path
}

