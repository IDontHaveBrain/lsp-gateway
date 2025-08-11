package project

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/gitignore"
	"lsp-gateway/src/internal/registry"
	"lsp-gateway/src/internal/security"
)

// DetectedLanguage represents a detected language with its confidence score
type DetectedLanguage struct {
	Language   string
	Confidence int
	Indicators []string
}

// DetectLanguages detects programming languages in the given directory
// Returns languages sorted by confidence (highest first)
func DetectLanguages(workingDir string) ([]string, error) {
	workingDir, err := common.ValidateAndGetWorkingDir(workingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	// Convert to absolute path for consistency
	absPath, err := filepath.Abs(workingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}
	workingDir = absPath

	// Check if directory exists
	if _, err := os.Stat(workingDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory does not exist: %s", workingDir)
	}

	detected := make(map[string]*DetectedLanguage)

	// Walk through directory tree using gitignore-aware walker
	walker := gitignore.NewWalker(workingDir)
	err = walker.WalkDir(workingDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		// Skip directories (already handled by gitignore walker)
		if d.IsDir() {
			// Limit depth to 3 levels for performance
			relPath, _ := filepath.Rel(workingDir, path)
			if strings.Count(relPath, string(os.PathSeparator)) >= 3 {
				return fs.SkipDir
			}

			return nil
		}

		// Analyze files
		analyzeFile(path, workingDir, detected)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	// Convert to sorted slice
	languages := make([]string, 0, len(detected))
	for languageName := range detected {
		languages = append(languages, languageName)
	}

	// Sort by confidence (simple priority order for now)
	sortLanguagesByPriority(languages)

	return languages, nil
}

// analyzeFile analyzes a single file and updates detection results
func analyzeFile(filePath, workingDir string, detected map[string]*DetectedLanguage) {
	fileName := filepath.Base(filePath)
	ext := strings.ToLower(filepath.Ext(fileName))

	// File extension detection using registry
	if langInfo, found := registry.GetLanguageByExtension(ext); found {
		// Special handling for JavaScript files to exclude infrastructure files
		if langInfo.Name == "javascript" {
			if !isInfrastructureJSFile(filePath, workingDir) {
				addDetection(detected, langInfo.Name, 10, fmt.Sprintf("*%s file: %s", ext, fileName))
			}
		} else {
			addDetection(detected, langInfo.Name, 10, fmt.Sprintf("*%s file: %s", ext, fileName))
		}
	}

	// Project structure detection
	switch fileName {
	case "go.mod":
		addDetection(detected, "go", 25, "go.mod file")
	case "go.sum":
		addDetection(detected, "go", 15, "go.sum file")
	case "package.json":
		// Check if it's TypeScript or JavaScript project
		if hasTypeScriptIndicators(filePath) {
			addDetection(detected, "typescript", 25, "package.json with TypeScript")
		} else if hasJavaScriptIndicators(filePath) {
			addDetection(detected, "javascript", 25, "package.json with JavaScript")
		}
	case "tsconfig.json":
		addDetection(detected, "typescript", 30, "tsconfig.json file")
	case "setup.py":
		addDetection(detected, "python", 25, "setup.py file")
	case "requirements.txt":
		addDetection(detected, "python", 20, "requirements.txt file")
	case "pyproject.toml":
		addDetection(detected, "python", 20, "pyproject.toml file")
	case "pom.xml":
		addDetection(detected, "java", 30, "pom.xml file")
	case "build.gradle", "build.gradle.kts":
		addDetection(detected, "java", 25, "Gradle build file")
	case "Cargo.toml":
		addDetection(detected, "rust", 30, "Cargo.toml file")
	case "Cargo.lock":
		addDetection(detected, "rust", 15, "Cargo.lock file")
	}
}

// hasTypeScriptIndicators checks if package.json indicates actual TypeScript project usage
func hasTypeScriptIndicators(packageJsonPath string) bool {
	// First check if tsconfig.json exists in the same directory
	dir := filepath.Dir(packageJsonPath)
	if _, err := os.Stat(filepath.Join(dir, "tsconfig.json")); err == nil {
		return true
	}

	content, err := os.ReadFile(packageJsonPath)
	if err != nil {
		return false
	}

	// Parse package.json to check for actual TypeScript project setup
	var packageData map[string]interface{}
	if err := json.Unmarshal(content, &packageData); err != nil {
		return false
	}

	// Check for TypeScript in main dependencies (not devDependencies)
	if deps, ok := packageData["dependencies"].(map[string]interface{}); ok {
		if _, hasTS := deps["typescript"]; hasTS {
			return true
		}
	}

	// Check for TypeScript-specific scripts (build/dev scripts with tsc)
	if scripts, ok := packageData["scripts"].(map[string]interface{}); ok {
		for _, script := range scripts {
			if scriptStr, ok := script.(string); ok {
				if strings.Contains(scriptStr, "tsc ") || strings.Contains(scriptStr, "tsc\"") {
					return true
				}
			}
		}
	}

	// Check for @types in dependencies (indicates TypeScript usage)
	if deps, ok := packageData["dependencies"].(map[string]interface{}); ok {
		for depName := range deps {
			if strings.HasPrefix(depName, "@types/") {
				return true
			}
		}
	}

	return false
}

// hasJavaScriptIndicators checks if package.json indicates actual JavaScript project usage
func hasJavaScriptIndicators(packageJsonPath string) bool {
	content, err := os.ReadFile(packageJsonPath)
	if err != nil {
		return false
	}

	// Parse package.json to check for actual JavaScript project setup
	var packageData map[string]interface{}
	if err := json.Unmarshal(content, &packageData); err != nil {
		return false
	}

	// Check for main entry point, but exclude wrapper/installer patterns
	if main, ok := packageData["main"].(string); ok && main != "" {
		// If main points to a .js file, check if it's not just a wrapper
		if strings.HasSuffix(main, ".js") {
			// Exclude common wrapper patterns
			fileName := filepath.Base(main)
			wrapperPatterns := []string{
				"index.js",     // Generic npm package entry point
				"installer.js", // Binary installer
				"wrapper.js",   // Binary wrapper
			}

			isWrapper := false
			for _, pattern := range wrapperPatterns {
				if fileName == pattern {
					isWrapper = true
					break
				}
			}

			// If not a wrapper pattern, consider it a real JavaScript project
			if !isWrapper {
				return true
			}
		}
	}

	// Check for JavaScript-specific dependencies (not TypeScript)
	if deps, ok := packageData["dependencies"].(map[string]interface{}); ok {
		// Look for common JavaScript-only frameworks/libraries
		jsLibraries := []string{"express", "react", "vue", "angular", "lodash", "axios", "webpack"}
		for _, lib := range jsLibraries {
			if _, hasLib := deps[lib]; hasLib {
				return true
			}
		}
	}

	// Check for JavaScript-specific scripts (not build scripts)
	if scripts, ok := packageData["scripts"].(map[string]interface{}); ok {
		for scriptName, script := range scripts {
			if scriptStr, ok := script.(string); ok {
				// Look for actual JavaScript development scripts (not build scripts)
				if scriptName == "start" && strings.Contains(scriptStr, "node ") {
					return true
				}
				if scriptName == "dev" && strings.Contains(scriptStr, "node ") {
					return true
				}
			}
		}
	}

	// Check if there are actual JavaScript source files in src/ directory
	dir := filepath.Dir(packageJsonPath)
	srcDir := filepath.Join(dir, "src")
	if _, err := os.Stat(srcDir); err == nil {
		// Check if src/ contains .js files that aren't build files
		hasAppJS := false
		filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if strings.HasSuffix(path, ".js") && !isInfrastructureJSFile(path, dir) {
				hasAppJS = true
				return filepath.SkipAll
			}
			return nil
		})
		if hasAppJS {
			return true
		}
	}

	return false
}

// isInfrastructureJSFile checks if a .js file is a build/infrastructure file
func isInfrastructureJSFile(filePath, workingDir string) bool {
	relPath, err := filepath.Rel(workingDir, filePath)
	if err != nil {
		// If we can't get relative path, be conservative and allow detection
		return false
	}

	// Normalize path separators
	relPath = filepath.ToSlash(relPath)

	// Check for common infrastructure/build directories
	infrastructurePaths := []string{
		"scripts/",
		"build/",
		"lib/",
		"bin/",
		"tools/",
		"config/",
		"webpack",
		"rollup",
		"babel",
		"jest",
		"test/",
		"tests/",
		"spec/",
		"__tests__/",
		"node_modules/",
		"dist/",
		"out/",
	}

	for _, infraPath := range infrastructurePaths {
		if strings.HasPrefix(relPath, infraPath) {
			return true
		}
	}

	// Check for common infrastructure filenames
	fileName := filepath.Base(filePath)
	infrastructureFiles := []string{
		"webpack.config.js",
		"rollup.config.js",
		"babel.config.js",
		"jest.config.js",
		"gulpfile.js",
		"gruntfile.js",
		"build.js",
		"installer.js",
		"postinstall.js",
		"preinstall.js",
	}

	for _, infraFile := range infrastructureFiles {
		if fileName == infraFile {
			return true
		}
	}

	return false
}

// addDetection adds or updates a language detection
func addDetection(detected map[string]*DetectedLanguage, language string, confidence int, indicator string) {
	if existing, exists := detected[language]; exists {
		existing.Confidence += confidence
		existing.Indicators = append(existing.Indicators, indicator)
	} else {
		detected[language] = &DetectedLanguage{
			Language:   language,
			Confidence: confidence,
			Indicators: []string{indicator},
		}
	}
}

// sortLanguagesByPriority sorts languages by priority/confidence
func sortLanguagesByPriority(languages []string) {
	// Simple priority order based on common usage patterns
	// Only include supported languages from registry
	priority := make(map[string]int)
	supportedLanguages := registry.GetLanguageNames()

	// Assign priorities based on common usage patterns
	for i, lang := range supportedLanguages {
		switch lang {
		case "go":
			priority[lang] = 4
		case "typescript":
			priority[lang] = 3
		case "javascript":
			priority[lang] = 2
		case "python":
			priority[lang] = 1
		case "java":
			priority[lang] = 0
		case "rust":
			priority[lang] = 2
		default:
			priority[lang] = i // Use index as fallback priority
		}
	}

	// Sort using a simple bubble sort for clarity
	for i := 0; i < len(languages)-1; i++ {
		for j := 0; j < len(languages)-i-1; j++ {
			p1 := priority[languages[j]]
			p2 := priority[languages[j+1]]
			if p1 < p2 {
				languages[j], languages[j+1] = languages[j+1], languages[j]
			}
		}
	}
}

// IsLSPServerAvailable checks if the LSP server for a language is available
func IsLSPServerAvailable(language string) bool {
	// First validate that the language is supported
	if !registry.IsLanguageSupported(language) {
		return false
	}

	// Get default configuration to find server command
	defaultConfig := config.GetDefaultConfig()
	serverConfig, exists := defaultConfig.Servers[language]
	if !exists {
		return false
	}

	// Validate command is allowed by security
	if err := security.ValidateCommand(serverConfig.Command, serverConfig.Args); err != nil {
		return false
	}

	// Check if command exists in PATH
	_, err := exec.LookPath(serverConfig.Command)
	return err == nil
}

// GetAvailableLanguages returns only languages that have LSP servers available
func GetAvailableLanguages(workingDir string) ([]string, error) {
	allLanguages, err := DetectLanguages(workingDir)
	if err != nil {
		return nil, err
	}

	available := make([]string, 0, len(allLanguages))
	for _, language := range allLanguages {
		if IsLSPServerAvailable(language) {
			available = append(available, language)
		}
	}

	return available, nil
}
