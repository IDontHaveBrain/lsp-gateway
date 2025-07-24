package project

import (
	"context"
	"encoding/json"
	"fmt"
	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/setup"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TypeScript framework name mappings
var typeScriptFrameworkMap = map[string]string{
	"typescript":                    "TypeScript",
	"@types/node":                  "Node.js Types",
	"@types/react":                 "React Types", 
	"@types/express":               "Express Types",
	"ts-node":                      "ts-node",
	"tsx":                          "tsx",
	"tsc-watch":                    "tsc-watch",
	"nodemon":                      "Nodemon",
	"concurrently":                 "Concurrently",
	"@typescript-eslint/parser":    "TypeScript ESLint",
	"@typescript-eslint/eslint-plugin": "TypeScript ESLint Plugin",
	"prettier":                     "Prettier",
	"jest":                         "Jest",
	"@types/jest":                  "Jest Types",
	"vitest":                       "Vitest", 
	"ava":                          "AVA",
	"mocha":                        "Mocha",
	"@types/mocha":                 "Mocha Types",
}

// TypeScript project type detection rules
var typeScriptProjectTypes = map[string][]string{
	"@angular/core":  {"Angular"},
	"vue":            {"Vue.js with TypeScript"},
	"next":           {"Next.js with TypeScript"},
	"@nestjs/core":   {"NestJS"},
}

// TypeScriptLanguageDetector implements comprehensive TypeScript project detection
type TypeScriptLanguageDetector struct {
	logger       *setup.SetupLogger
	executor     platform.CommandExecutor
	nodeDetector LanguageDetector
}

// NewTypeScriptLanguageDetector creates a new TypeScript language detector
func NewTypeScriptLanguageDetector() LanguageDetector {
	return &TypeScriptLanguageDetector{
		logger:       setup.NewSetupLogger(&setup.SetupLoggerConfig{Component: "typescript-detector"}),
		executor:     platform.NewCommandExecutor(),
		nodeDetector: NewNodeJSLanguageDetector(),
	}
}

func (t *TypeScriptLanguageDetector) DetectLanguage(ctx context.Context, path string) (*types.LanguageDetectionResult, error) {
	startTime := time.Now()
	
	t.logger.WithField("path", path).Debug("Starting TypeScript project detection")
	
	result := &types.LanguageDetectionResult{
		Language:        types.PROJECT_TYPE_TYPESCRIPT,
		RequiredServers: []string{types.SERVER_TYPESCRIPT_LANG_SERVER},
		Metadata:        make(map[string]interface{}),
		MarkerFiles:     []string{},
		ConfigFiles:     []string{},
		SourceDirs:      []string{},
		TestDirs:        []string{},
		BuildFiles:      []string{},
		Dependencies:    make(map[string]string),
		DevDependencies: make(map[string]string),
	}

	confidence := 0.0

	// Check for tsconfig.json (primary TypeScript indicator)
	tsconfigPath := filepath.Join(path, types.MARKER_TSCONFIG)
	if _, err := os.Stat(tsconfigPath); err == nil {
		result.MarkerFiles = append(result.MarkerFiles, types.MARKER_TSCONFIG)
		result.ConfigFiles = append(result.ConfigFiles, types.MARKER_TSCONFIG)
		confidence = 0.95
		
		if err := t.parseTsConfig(tsconfigPath, result); err != nil {
			t.logger.WithError(err).Warn("Failed to parse tsconfig.json")
			confidence = 0.8
		}
	}

	// Use Node.js detector to get package.json information
	nodeResult, err := t.nodeDetector.DetectLanguage(ctx, path)
	if err == nil && nodeResult.Confidence > 0 {
		// Check if TypeScript is in dependencies or devDependencies
		hasTypeScript := false
		allDeps := make(map[string]string)
		for k, v := range nodeResult.Dependencies {
			allDeps[k] = v
		}
		for k, v := range nodeResult.DevDependencies {
			allDeps[k] = v
		}

		if _, exists := allDeps["typescript"]; exists {
			hasTypeScript = true
			confidence = max(confidence, 0.9)
		}

		// Merge Node.js results
		result.Dependencies = nodeResult.Dependencies
		result.DevDependencies = nodeResult.DevDependencies
		
		// Copy relevant metadata
		for key, value := range nodeResult.Metadata {
			if key != "frameworks" { // We'll detect TypeScript-specific frameworks
				result.Metadata[key] = value
			}
		}

		// Copy marker files from Node.js detection
		for _, marker := range nodeResult.MarkerFiles {
			if marker != types.MARKER_TSCONFIG { // Avoid duplicates
				result.MarkerFiles = append(result.MarkerFiles, marker)
			}
		}

		// Copy config files from Node.js detection
		for _, config := range nodeResult.ConfigFiles {
			if config != types.MARKER_TSCONFIG { // Avoid duplicates
				result.ConfigFiles = append(result.ConfigFiles, config)
			}
		}

		if !hasTypeScript && confidence == 0 {
			confidence = nodeResult.Confidence * 0.3 // Reduce confidence if no TypeScript dep
		}
	}

	// Check for TypeScript files
	tsFileCount := 0
	if err := t.scanForTypeScriptFiles(path, result, &tsFileCount); err != nil {
		t.logger.WithError(err).Debug("Error scanning for TypeScript files")
	}

	// Adjust confidence based on TypeScript files found
	if tsFileCount > 0 {
		if confidence == 0 {
			confidence = 0.8 // TypeScript files without config
		} else {
			confidence = minFloat(confidence+0.1, 1.0)
		}
		result.Metadata["typescript_file_count"] = tsFileCount
	} else if confidence == 0 {
		confidence = 0.1 // No clear TypeScript indicators
	}

	result.Confidence = confidence

	// Detect TypeScript version
	if err := t.detectTypeScriptVersion(ctx, result); err != nil {
		t.logger.WithError(err).Debug("Failed to detect TypeScript version")
	}

	// Detect TypeScript-specific frameworks and libraries
	t.detectTypeScriptFrameworks(result)

	// Analyze project structure
	t.analyzeProjectStructure(path, result)

	// Look for additional TypeScript configuration files
	t.detectAdditionalConfigs(path, result)

	// Set detection metadata
	result.Metadata["detection_time"] = time.Since(startTime)
	result.Metadata["detector_version"] = types.DETECTOR_VERSION_DEFAULT

	t.logger.WithFields(map[string]interface{}{
		"confidence":      result.Confidence,
		"marker_files":    len(result.MarkerFiles),
		"source_dirs":     len(result.SourceDirs),
		"dependencies":    len(result.Dependencies),
		"detection_time":  time.Since(startTime),
	}).Info("TypeScript project detection completed")

	return result, nil
}

func (t *TypeScriptLanguageDetector) parseTsConfig(tsconfigPath string, result *types.LanguageDetectionResult) error {
	content, err := os.ReadFile(tsconfigPath)
	if err != nil {
		return fmt.Errorf("failed to read tsconfig.json: %w", err)
	}

	var tsconfig map[string]interface{}
	if err := json.Unmarshal(content, &tsconfig); err != nil {
		return fmt.Errorf("failed to parse tsconfig.json: %w", err)
	}

	// Extract compiler options
	if compilerOptions, ok := tsconfig["compilerOptions"].(map[string]interface{}); ok {
		result.Metadata["compiler_options"] = compilerOptions
		
		// Extract target TypeScript version
		if target, ok := compilerOptions["target"].(string); ok {
			result.Metadata["typescript_target"] = target
		}

		// Extract module system
		if module, ok := compilerOptions["module"].(string); ok {
			result.Metadata["module_system"] = module
		}

		// Extract output directory
		if outDir, ok := compilerOptions["outDir"].(string); ok {
			result.Metadata["output_directory"] = outDir
		}

		// Extract root directory
		if rootDir, ok := compilerOptions["rootDir"].(string); ok {
			result.Metadata["root_directory"] = rootDir
		}

		// Check for strict mode
		if strict, ok := compilerOptions["strict"].(bool); ok {
			result.Metadata["strict_mode"] = strict
		}

		// Extract lib references
		if lib, ok := compilerOptions["lib"].([]interface{}); ok {
			var libs []string
			for _, l := range lib {
				if libStr, ok := l.(string); ok {
					libs = append(libs, libStr)
				}
			}
			result.Metadata["libraries"] = libs
		}
	}

	// Extract include patterns
	if include, ok := tsconfig["include"].([]interface{}); ok {
		var includes []string
		for _, inc := range include {
			if incStr, ok := inc.(string); ok {
				includes = append(includes, incStr)
			}
		}
		result.Metadata["include_patterns"] = includes
	}

	// Extract exclude patterns
	if exclude, ok := tsconfig["exclude"].([]interface{}); ok {
		var excludes []string
		for _, exc := range exclude {
			if excStr, ok := exc.(string); ok {
				excludes = append(excludes, excStr)
			}
		}
		result.Metadata["exclude_patterns"] = excludes
	}

	// Extract extends
	if extends, ok := tsconfig["extends"].(string); ok {
		result.Metadata["extends"] = extends
	}

	// Extract project references
	if references, ok := tsconfig["references"].([]interface{}); ok {
		var refs []string
		for _, ref := range references {
			if refMap, ok := ref.(map[string]interface{}); ok {
				if path, ok := refMap["path"].(string); ok {
					refs = append(refs, path)
				}
			}
		}
		if len(refs) > 0 {
			result.Metadata["project_references"] = refs
			result.Metadata["is_composite_project"] = true
		}
	}

	return nil
}

func (t *TypeScriptLanguageDetector) scanForTypeScriptFiles(path string, result *types.LanguageDetectionResult, tsFileCount *int) error {
	sourceDirs := make(map[string]bool)
	testDirs := make(map[string]bool)
	
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue on errors
		}

		// Skip node_modules and other common ignore directories
		if info.IsDir() {
			name := info.Name()
			skipDirs := []string{"node_modules", ".git", "dist", "build", "coverage", ".nyc_output", "out"}
			for _, skipDir := range skipDirs {
				if name == skipDir {
					return filepath.SkipDir
				}
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(filePath))
		if ext == types.EXT_TS || ext == ".tsx" || ext == ".d.ts" {
			*tsFileCount++
			dir := filepath.Dir(filePath)
			relDir, _ := filepath.Rel(path, dir)
			
			fileName := strings.ToLower(info.Name())
			if strings.Contains(fileName, "test") || strings.Contains(fileName, "spec") ||
			   strings.Contains(relDir, "test") || strings.Contains(relDir, "__tests__") ||
			   strings.Contains(relDir, "spec") {
				testDirs[relDir] = true
			} else {
				sourceDirs[relDir] = true
			}

			// Check for declaration files
			if ext == ".d.ts" {
				if result.Metadata["declaration_files"] == nil {
					result.Metadata["declaration_files"] = []string{}
				}
				result.Metadata["declaration_files"] = append(result.Metadata["declaration_files"].([]string), filePath)
			}
		}

		// Look for TypeScript-specific build files
		tsBuilds := []string{"webpack.config.ts", "rollup.config.ts", "vite.config.ts", "esbuild.config.ts"}
		for _, buildFile := range tsBuilds {
			if info.Name() == buildFile {
				result.BuildFiles = append(result.BuildFiles, buildFile)
			}
		}

		return nil
	})

	// Convert maps to slices
	for dir := range sourceDirs {
		if dir != "." {
			result.SourceDirs = append(result.SourceDirs, dir)
		}
	}
	for dir := range testDirs {
		if dir != "." {
			result.TestDirs = append(result.TestDirs, dir)
		}
	}

	return err
}

func (t *TypeScriptLanguageDetector) detectTypeScriptVersion(ctx context.Context, result *types.LanguageDetectionResult) error {
	// Try to get TypeScript version from installed tsc
	cmd := []string{"tsc", "--version"}
	execResult, err := t.executor.Execute(cmd[0], cmd[1:], 10*time.Second)
	if err != nil {
		// Try npx tsc --version
		npxCmd := []string{"npx", "tsc", "--version"}
		npxResult, err := t.executor.Execute(npxCmd[0], npxCmd[1:], 10*time.Second)
		if err != nil {
			return err
		}
		execResult = npxResult
	}

	if execResult.ExitCode == 0 {
		output := strings.TrimSpace(execResult.Stdout)
		// Parse "Version 4.9.4"
		if strings.HasPrefix(output, "Version ") {
			version := strings.TrimPrefix(output, "Version ")
			if result.Version == "" { // Only set if not already set
				result.Version = version
			}
			result.Metadata["installed_typescript_version"] = version
		}
	}

	// Also check package.json for TypeScript version
	if tsVersion, exists := result.Dependencies["typescript"]; exists {
		result.Metadata["package_typescript_version"] = tsVersion
	}
	if tsDevVersion, exists := result.DevDependencies["typescript"]; exists {
		result.Metadata["package_typescript_dev_version"] = tsDevVersion
	}

	return nil
}

// getTypeScriptFrameworkName returns the framework name for a given dependency name
func (t *TypeScriptLanguageDetector) getTypeScriptFrameworkName(dep string) (string, bool) {
	framework, exists := typeScriptFrameworkMap[dep]
	return framework, exists
}

// detectTypeScriptProjectTypes determines project types based on dependencies
func (t *TypeScriptLanguageDetector) detectTypeScriptProjectTypes(allDeps map[string]string) []string {
	projectTypes := []string{}
	
	// Direct project type detection
	for dep, types := range typeScriptProjectTypes {
		if _, exists := allDeps[dep]; exists {
			projectTypes = append(projectTypes, types...)
		}
	}
	
	// Special cases requiring TypeScript dependency
	if _, existsTS := allDeps["typescript"]; existsTS {
		if _, exists := allDeps["vue"]; exists {
			projectTypes = append(projectTypes, "Vue.js with TypeScript")
		}
		if _, exists := allDeps["next"]; exists {
			projectTypes = append(projectTypes, "Next.js with TypeScript")
		}
	}
	
	return projectTypes
}

func (t *TypeScriptLanguageDetector) detectTypeScriptFrameworks(result *types.LanguageDetectionResult) {
	frameworks := []string{}
	
	// Check dependencies for TypeScript-specific frameworks
	allDeps := make(map[string]string)
	for k, v := range result.Dependencies {
		allDeps[k] = v
	}
	for k, v := range result.DevDependencies {
		allDeps[k] = v
	}

	for dep := range allDeps {
		// Use map lookup instead of switch statement
		if framework, exists := t.getTypeScriptFrameworkName(dep); exists {
			frameworks = append(frameworks, framework)
		}

		// Framework-specific TypeScript detection for @types/ packages
		if strings.HasPrefix(dep, "@types/") {
			typesFor := strings.TrimPrefix(dep, "@types/")
			frameworks = append(frameworks, fmt.Sprintf("%s Types", typesFor))
		}
	}

	if len(frameworks) > 0 {
		result.Metadata["frameworks"] = frameworks
	}

	// Detect specific TypeScript project types using helper method
	projectTypes := t.detectTypeScriptProjectTypes(allDeps)
	if len(projectTypes) > 0 {
		result.Metadata["project_types"] = projectTypes
	}
}

func (t *TypeScriptLanguageDetector) analyzeProjectStructure(path string, result *types.LanguageDetectionResult) {
	// TypeScript-specific directories
	tsDirs := []string{"src", "lib", "types", "typings", "dist", "build", "out", "@types"}
	foundDirs := []string{}

	for _, dir := range tsDirs {
		dirPath := filepath.Join(path, dir)
		if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
			foundDirs = append(foundDirs, dir)
		}
	}

	if len(foundDirs) > 0 {
		result.Metadata["typescript_structure"] = foundDirs
	}

	// Check for monorepo structure (TypeScript projects often use monorepos)
	monorepoIndicators := []string{"packages", "apps", "libs", "workspace.json", "nx.json", "lerna.json"}
	isMonorepo := false
	
	for _, indicator := range monorepoIndicators {
		if _, err := os.Stat(filepath.Join(path, indicator)); err == nil {
			isMonorepo = true
			break
		}
	}
	
	if isMonorepo {
		result.Metadata["is_monorepo"] = true
	}

	// Check for multiple tsconfig files (common in complex TypeScript projects)
	tsconfigFiles := []string{"tsconfig.json", "tsconfig.build.json", "tsconfig.dev.json", "tsconfig.prod.json", "tsconfig.test.json"}
	foundTsConfigs := []string{}
	
	for _, config := range tsconfigFiles {
		if _, err := os.Stat(filepath.Join(path, config)); err == nil {
			foundTsConfigs = append(foundTsConfigs, config)
		}
	}
	
	if len(foundTsConfigs) > 1 {
		result.Metadata["multiple_tsconfigs"] = foundTsConfigs
	}
}

func (t *TypeScriptLanguageDetector) detectAdditionalConfigs(path string, result *types.LanguageDetectionResult) {
	configFiles := map[string]string{
		"tsconfig.build.json":    "TypeScript Build Config",
		"tsconfig.dev.json":      "TypeScript Dev Config",
		"tsconfig.prod.json":     "TypeScript Production Config",
		"tsconfig.test.json":     "TypeScript Test Config",
		"tslint.json":           "TSLint (deprecated)",
		".eslintrc.js":          "ESLint Configuration",
		".eslintrc.json":        "ESLint Configuration",
		"prettier.config.js":    "Prettier Configuration",
		".prettierrc":           "Prettier Configuration",
		"jest.config.ts":        "Jest Configuration (TypeScript)",
		"vitest.config.ts":      "Vitest Configuration",
		"webpack.config.ts":     "Webpack Configuration (TypeScript)",
		"rollup.config.ts":      "Rollup Configuration (TypeScript)",
		"vite.config.ts":        "Vite Configuration (TypeScript)",
		"esbuild.config.ts":     "ESBuild Configuration (TypeScript)",
	}

	for file, description := range configFiles {
		if _, err := os.Stat(filepath.Join(path, file)); err == nil {
			result.ConfigFiles = append(result.ConfigFiles, file)
			if result.Metadata["config_tools"] == nil {
				result.Metadata["config_tools"] = []string{}
			}
			result.Metadata["config_tools"] = append(result.Metadata["config_tools"].([]string), description)
		}
	}
}

// Interface implementation methods
// GetLanguageInfo returns comprehensive information about TypeScript language support
func (t *TypeScriptLanguageDetector) GetLanguageInfo(language string) (*types.LanguageInfo, error) {
	if language != types.PROJECT_TYPE_TYPESCRIPT {
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	return &types.LanguageInfo{
		Name:           types.PROJECT_TYPE_TYPESCRIPT,
		DisplayName:    "TypeScript",
		MinVersion:     "4.0.0",
		MaxVersion:     "5.3.0",
		BuildTools:     []string{"tsc", "webpack", "vite", "rollup", "parcel", "esbuild"},
		PackageManager: "npm",
		TestFrameworks: []string{"jest", "vitest", "mocha", "jasmine", "cypress"},
		LintTools:      []string{"eslint", "tslint", "typescript-eslint"},
		FormatTools:    []string{"prettier", "typescript-formatter"},
		LSPServers:     []string{types.SERVER_TYPESCRIPT_LANG_SERVER},
		FileExtensions: []string{".ts", ".tsx", ".d.ts"},
		Capabilities:   []string{"completion", "hover", "definition", "references", "formatting", "code_action", "rename", "refactor"},
		Metadata: map[string]interface{}{
			"transpiles_to":    "JavaScript",
			"type_checking":    true,
			"jsx_support":      true,
			"documentation":    "typescriptlang.org",
			"ecosystem":        "JavaScript/Node.js",
		},
	}, nil
}

func (t *TypeScriptLanguageDetector) GetMarkerFiles() []string {
	return []string{types.MARKER_TSCONFIG}
}

func (t *TypeScriptLanguageDetector) GetRequiredServers() []string {
	return []string{types.SERVER_TYPESCRIPT_LANG_SERVER}
}

func (t *TypeScriptLanguageDetector) GetPriority() int {
	return types.PRIORITY_TYPESCRIPT
}

func (t *TypeScriptLanguageDetector) ValidateStructure(ctx context.Context, path string) error {
	// Check for tsconfig.json
	tsconfigPath := filepath.Join(path, types.MARKER_TSCONFIG)
	if _, err := os.Stat(tsconfigPath); os.IsNotExist(err) {
		return types.NewValidationError(types.PROJECT_TYPE_TYPESCRIPT, "tsconfig.json file not found", nil).
			WithMetadata("missing_files", []string{types.MARKER_TSCONFIG}).
			WithSuggestion("Initialize TypeScript project: tsc --init").
			WithSuggestion("Create tsconfig.json manually").
			WithSuggestion("Ensure you're in the correct TypeScript project directory")
	}

	// Check if tsconfig.json is valid JSON
	content, err := os.ReadFile(tsconfigPath)
	if err != nil {
		return types.NewValidationError(types.PROJECT_TYPE_TYPESCRIPT, "cannot read tsconfig.json file", err).
			WithMetadata("invalid_files", []string{types.MARKER_TSCONFIG})
	}

	var tsconfig map[string]interface{}
	if err := json.Unmarshal(content, &tsconfig); err != nil {
		return types.NewValidationError(types.PROJECT_TYPE_TYPESCRIPT, "tsconfig.json is not valid JSON", err).
			WithMetadata("invalid_files", []string{types.MARKER_TSCONFIG})
	}

	// Check for TypeScript files
	hasTSFiles := false
	err = filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		
		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(filePath))
			if ext == ".ts" || ext == ".tsx" {
				hasTSFiles = true
				return filepath.SkipAll // Found at least one, can stop
			}
		}
		
		// Skip node_modules directory
		if info.IsDir() && info.Name() == "node_modules" {
			return filepath.SkipDir
		}
		
		return nil
	})

	if err != nil {
		return types.NewValidationError(types.PROJECT_TYPE_TYPESCRIPT, "error scanning for TypeScript files", err)
	}

	if !hasTSFiles {
		return types.NewValidationError(types.PROJECT_TYPE_TYPESCRIPT, "no TypeScript source files found", nil).
			WithMetadata("structure_issues", []string{"no .ts or .tsx files found in project"})
	}

	return nil
}

// Helper functions
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}