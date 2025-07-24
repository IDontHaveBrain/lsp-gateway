package project

import (
	"context"
	"lsp-gateway/internal/project/types"
	"os"
	"path/filepath"
	"strings"
)

// BasicLanguageDetector provides focused, efficient language detection
type BasicLanguageDetector struct {
	maxDepth int
}

// NewBasicLanguageDetector creates a new basic language detector
func NewBasicLanguageDetector() *BasicLanguageDetector {
	return &BasicLanguageDetector{
		maxDepth: 3,
	}
}

// DetectLanguage performs efficient language detection using file extensions and markers
func (d *BasicLanguageDetector) DetectLanguage(ctx context.Context, rootPath string) (*types.LanguageDetectionResult, error) {
	result := &types.LanguageDetectionResult{
		Language:        PROJECT_TYPE_UNKNOWN,
		Confidence:      0.0,
		MarkerFiles:     make([]string, 0),
		ConfigFiles:     make([]string, 0),
		SourceDirs:      make([]string, 0),
		RequiredServers: make([]string, 0),
		Dependencies:    make(map[string]string),
		Metadata:        make(map[string]interface{}),
	}

	// Check for marker files first
	markerResults := d.detectMarkerFiles(rootPath)
	if len(markerResults) > 0 {
		// Use highest confidence marker result
		bestResult := markerResults[0]
		for _, mr := range markerResults[1:] {
			if mr.Confidence > bestResult.Confidence {
				bestResult = mr
			}
		}
		result.Language = bestResult.Language
		result.Confidence = bestResult.Confidence
		result.MarkerFiles = bestResult.MarkerFiles
		result.RequiredServers = bestResult.RequiredServers
	}

	// Count files by extension to validate or determine language
	fileCounts := d.countFilesByExtension(ctx, rootPath)
	
	// If no markers found, use file extension analysis
	if result.Confidence == 0.0 {
		result = d.detectByFileExtensions(fileCounts)
	} else {
		// Validate marker-based detection with file counts
		d.validateWithFileCounts(result, fileCounts)
	}

	// Find source directories
	result.SourceDirs = d.findSourceDirectories(rootPath)
	
	return result, nil
}

// detectMarkerFiles checks for language-specific marker files
func (d *BasicLanguageDetector) detectMarkerFiles(rootPath string) []*types.LanguageDetectionResult {
	var results []*types.LanguageDetectionResult

	markers := map[string][]string{
		PROJECT_TYPE_GO:         {MARKER_GO_MOD},
		PROJECT_TYPE_PYTHON:     {MARKER_SETUP_PY, MARKER_PYPROJECT, MARKER_REQUIREMENTS},
		PROJECT_TYPE_NODEJS:     {MARKER_PACKAGE_JSON},
		PROJECT_TYPE_TYPESCRIPT: {MARKER_TSCONFIG},
		PROJECT_TYPE_JAVA:       {MARKER_POM_XML, MARKER_BUILD_GRADLE},
	}

	servers := map[string][]string{
		PROJECT_TYPE_GO:         {SERVER_GOPLS},
		PROJECT_TYPE_PYTHON:     {SERVER_PYLSP},
		PROJECT_TYPE_NODEJS:     {SERVER_TYPESCRIPT_LANG_SERVER},
		PROJECT_TYPE_TYPESCRIPT: {SERVER_TYPESCRIPT_LANG_SERVER},
		PROJECT_TYPE_JAVA:       {SERVER_JDTLS},
	}

	for language, markerFiles := range markers {
		foundMarkers := make([]string, 0)
		for _, marker := range markerFiles {
			markerPath := filepath.Join(rootPath, marker)
			if _, err := os.Stat(markerPath); err == nil {
				foundMarkers = append(foundMarkers, marker)
			}
		}

		if len(foundMarkers) > 0 {
			confidence := 0.9
			if language == PROJECT_TYPE_TYPESCRIPT && len(foundMarkers) == 1 && foundMarkers[0] == MARKER_TSCONFIG {
				// TypeScript detection might be less confident if only tsconfig.json
				confidence = 0.7
			}

			results = append(results, &types.LanguageDetectionResult{
				Language:        language,
				Confidence:      confidence,
				MarkerFiles:     foundMarkers,
				RequiredServers: servers[language],
				ConfigFiles:     foundMarkers,
				Dependencies:    make(map[string]string),
				Metadata:        make(map[string]interface{}),
				SourceDirs:      make([]string, 0),
			})
		}
	}

	return results
}

// countFilesByExtension counts files by extension with depth limit
func (d *BasicLanguageDetector) countFilesByExtension(ctx context.Context, rootPath string) map[string]int {
	counts := make(map[string]int)
	
	_ = filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil || info.IsDir() {
			return nil
		}

		// Check depth limit
		relPath, _ := filepath.Rel(rootPath, path)
		depth := strings.Count(relPath, string(filepath.Separator))
		if depth > d.maxDepth {
			return nil
		}

		// Skip common ignore directories
		if d.shouldSkipPath(path) {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != "" {
			counts[ext]++
		}

		return nil
	})

	return counts
}

// detectByFileExtensions determines language based on file extension counts
func (d *BasicLanguageDetector) detectByFileExtensions(fileCounts map[string]int) *types.LanguageDetectionResult {
	languageCounts := map[string]int{
		PROJECT_TYPE_GO:         fileCounts[".go"],
		PROJECT_TYPE_PYTHON:     fileCounts[".py"] + fileCounts[".pyx"],
		PROJECT_TYPE_NODEJS:     fileCounts[".js"] + fileCounts[".jsx"] + fileCounts[".mjs"],
		PROJECT_TYPE_TYPESCRIPT: fileCounts[".ts"] + fileCounts[".tsx"],
		PROJECT_TYPE_JAVA:       fileCounts[".java"],
	}

	servers := map[string][]string{
		PROJECT_TYPE_GO:         {SERVER_GOPLS},
		PROJECT_TYPE_PYTHON:     {SERVER_PYLSP},
		PROJECT_TYPE_NODEJS:     {SERVER_TYPESCRIPT_LANG_SERVER},
		PROJECT_TYPE_TYPESCRIPT: {SERVER_TYPESCRIPT_LANG_SERVER},
		PROJECT_TYPE_JAVA:       {SERVER_JDTLS},
	}

	// Find language with most files
	var bestLanguage string
	maxCount := 0
	for language, count := range languageCounts {
		if count > maxCount {
			maxCount = count
			bestLanguage = language
		}
	}

	confidence := 0.0
	if maxCount > 0 {
		totalFiles := 0
		for _, count := range fileCounts {
			totalFiles += count
		}
		confidence = float64(maxCount) / float64(totalFiles)
		if confidence > 0.8 {
			confidence = 0.8 // Cap at 0.8 for extension-only detection
		}
	}

	if bestLanguage == "" {
		bestLanguage = PROJECT_TYPE_UNKNOWN
	}

	return &types.LanguageDetectionResult{
		Language:        bestLanguage,
		Confidence:      confidence,
		MarkerFiles:     make([]string, 0),
		ConfigFiles:     make([]string, 0),
		SourceDirs:      make([]string, 0),
		RequiredServers: servers[bestLanguage],
		Dependencies:    make(map[string]string),
		Metadata:        map[string]interface{}{"file_counts": languageCounts},
	}
}

// validateWithFileCounts validates marker-based detection with file counts
func (d *BasicLanguageDetector) validateWithFileCounts(result *types.LanguageDetectionResult, fileCounts map[string]int) {
	expectedExts := map[string][]string{
		PROJECT_TYPE_GO:         {".go"},
		PROJECT_TYPE_PYTHON:     {".py", ".pyx"},
		PROJECT_TYPE_NODEJS:     {".js", ".jsx", ".mjs"},
		PROJECT_TYPE_TYPESCRIPT: {".ts", ".tsx"},
		PROJECT_TYPE_JAVA:       {".java"},
	}

	if exts, exists := expectedExts[result.Language]; exists {
		hasFiles := false
		for _, ext := range exts {
			if fileCounts[ext] > 0 {
				hasFiles = true
				break
			}
		}
		if !hasFiles {
			result.Confidence *= 0.5 // Reduce confidence if no matching files found
		}
	}
}

// findSourceDirectories identifies common source directories
func (d *BasicLanguageDetector) findSourceDirectories(rootPath string) []string {
	var sourceDirs []string
	commonSourceDirs := []string{"src", "lib", "source", "app", "pkg"}

	for _, dirName := range commonSourceDirs {
		dirPath := filepath.Join(rootPath, dirName)
		if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
			sourceDirs = append(sourceDirs, dirName)
		}
	}

	return sourceDirs
}

// shouldSkipPath determines if a path should be skipped during scanning
func (d *BasicLanguageDetector) shouldSkipPath(path string) bool {
	skipDirs := map[string]bool{
		"node_modules": true, ".git": true, ".svn": true, "__pycache__": true,
		".pytest_cache": true, "venv": true, ".venv": true, "env": true,
		"vendor": true, "target": true, "build": true, "dist": true,
		".idea": true, ".vscode": true, "bin": true, "obj": true,
	}

	parts := strings.Split(path, string(filepath.Separator))
	for _, part := range parts {
		if skipDirs[part] {
			return true
		}
	}
	return false
}