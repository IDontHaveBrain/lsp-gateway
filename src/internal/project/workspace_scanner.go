package project

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// ScanWorkspaceFiles scans the workspace for source files based on detected languages
func ScanWorkspaceFiles(workingDir string, languages []string) ([]string, error) {
	var files []string
	extensions := GetExtensionsForLanguages(languages)

	if len(extensions) == 0 {
		// If no languages detected, scan for all supported language files
		extensions = []string{".go", ".py", ".js", ".ts", ".jsx", ".tsx", ".java"}
	}

	// Walk through directory tree (up to 3 levels deep for performance)
	err := filepath.WalkDir(workingDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors and continue
		}

		// Skip hidden directories and common non-source directories
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" ||
				name == "build" || name == "dist" || name == "target" || name == "__pycache__" ||
				name == "bin" || name == "obj" || name == "out" {
				return fs.SkipDir
			}

			// Limit depth to 3 levels
			relPath, _ := filepath.Rel(workingDir, path)
			depth := strings.Count(relPath, string(filepath.Separator))
			if depth > 3 {
				return fs.SkipDir
			}
			return nil
		}

		// Check if file has a relevant extension
		ext := filepath.Ext(path)
		for _, validExt := range extensions {
			if ext == validExt {
				files = append(files, path)
				break
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return files, nil
}

// GetExtensionsForLanguages returns file extensions for the given languages
func GetExtensionsForLanguages(languages []string) []string {
	extMap := map[string][]string{
		"go":         {".go"},
		"python":     {".py"},
		"javascript": {".js", ".jsx", ".mjs"},
		"typescript": {".ts", ".tsx"},
		"java":       {".java"},
	}

	var extensions []string
	seen := make(map[string]bool)

	for _, lang := range languages {
		if exts, ok := extMap[lang]; ok {
			for _, ext := range exts {
				if !seen[ext] {
					extensions = append(extensions, ext)
					seen[ext] = true
				}
			}
		}
	}

	return extensions
}
