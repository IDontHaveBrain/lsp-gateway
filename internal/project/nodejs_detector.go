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

// Framework mappings for dependency name to framework name lookup
var frameworkMap = map[string]string{
	"express":        "Express.js",
	"koa":            "Koa.js",
	"fastify":        "Fastify",
	"hapi":           "Hapi.js",
	"nestjs":         "NestJS",
	"react":          "React",
	"vue":            "Vue.js",
	"angular":        "Angular",
	"@angular/core":  "Angular",
	"svelte":         "Svelte",
	"next":           "Next.js",
	"nuxt":           "Nuxt.js",
	"gatsby":         "Gatsby",
	"electron":       "Electron",
	"react-native":   "React Native",
	"jest":           "Jest",
	"mocha":          "Mocha",
	"chai":           "Chai",
	"cypress":        "Cypress",
	"playwright":     "Playwright",
	"webpack":        "Webpack",
	"vite":           "Vite",
	"rollup":         "Rollup",
	"typescript":     "TypeScript",
	"eslint":         "ESLint",
	"prettier":       "Prettier",
}

// Framework to project type mappings
var frameworkProjectTypes = map[string][]string{
	"React":        {"frontend"},
	"Vue.js":       {"frontend"},
	"Angular":      {"frontend"},
	"Svelte":       {"frontend"},
	"Express.js":   {"backend"},
	"Koa.js":       {"backend"},
	"Fastify":      {"backend"},
	"Hapi.js":      {"backend"},
	"NestJS":       {"backend"},
	"Next.js":      {"fullstack"},
	"Nuxt.js":      {"fullstack"},
	"Gatsby":       {"fullstack"},
	"Electron":     {"desktop"},
	"React Native": {"mobile"},
}

// NodeJSLanguageDetector implements comprehensive Node.js project detection
type NodeJSLanguageDetector struct {
	logger   *setup.SetupLogger
	executor platform.CommandExecutor
}

// NewNodeJSLanguageDetector creates a new Node.js language detector
func NewNodeJSLanguageDetector() LanguageDetector {
	return &NodeJSLanguageDetector{
		logger:   setup.NewSetupLogger(&setup.SetupLoggerConfig{Component: "nodejs-detector"}),
		executor: platform.NewCommandExecutor(),
	}
}

func (n *NodeJSLanguageDetector) DetectLanguage(ctx context.Context, path string) (*types.LanguageDetectionResult, error) {
	startTime := time.Now()
	
	n.logger.WithField("path", path).Debug("Starting Node.js project detection")
	
	result := &types.LanguageDetectionResult{
		Language:        types.PROJECT_TYPE_NODEJS,
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

	// Check for package.json (primary indicator)
	packageJsonPath := filepath.Join(path, types.MARKER_PACKAGE_JSON)
	if _, err := os.Stat(packageJsonPath); err == nil {
		result.MarkerFiles = append(result.MarkerFiles, types.MARKER_PACKAGE_JSON)
		result.ConfigFiles = append(result.ConfigFiles, types.MARKER_PACKAGE_JSON)
		confidence = 0.95
		
		if err := n.parsePackageJson(packageJsonPath, result); err != nil {
			n.logger.WithError(err).Warn("Failed to parse package.json")
			confidence = 0.8
		}
	}

	// Check for lock files
	lockFiles := []string{types.MARKER_YARN_LOCK, types.MARKER_PNPM_LOCK, "package-lock.json"}
	for _, lockFile := range lockFiles {
		lockPath := filepath.Join(path, lockFile)
		if _, err := os.Stat(lockPath); err == nil {
			result.MarkerFiles = append(result.MarkerFiles, lockFile)
			if confidence == 0 {
				confidence = 0.7 // Lock file without package.json is unusual
			} else {
				confidence = min(confidence+0.05, 1.0)
			}
			
			// Determine package manager
			switch lockFile {
			case types.MARKER_YARN_LOCK:
				result.Metadata["package_manager"] = "yarn"
			case types.MARKER_PNPM_LOCK:
				result.Metadata["package_manager"] = "pnpm"
			case "package-lock.json":
				result.Metadata["package_manager"] = "npm"
			}
		}
	}

	// Check for Node.js files
	nodeFileCount := 0
	if err := n.scanForNodeFiles(path, result, &nodeFileCount); err != nil {
		n.logger.WithError(err).Debug("Error scanning for Node.js files")
	}

	// Adjust confidence based on JS/TS files found
	if nodeFileCount > 0 {
		if confidence == 0 {
			confidence = 0.6 // JS files without package.json
		}
		result.Metadata["node_file_count"] = nodeFileCount
	} else if confidence == 0 {
		confidence = 0.1 // No clear Node.js indicators
	}

	result.Confidence = confidence

	// Detect Node.js version
	if err := n.detectNodeVersion(ctx, result); err != nil {
		n.logger.WithError(err).Debug("Failed to detect Node.js version")
	}

	// Detect frameworks and libraries
	n.detectNodeFrameworks(result)

	// Detect project structure
	n.analyzeProjectStructure(path, result)

	// Look for additional configuration files
	n.detectAdditionalConfigs(path, result)

	// Set detection metadata
	result.Metadata["detection_time"] = time.Since(startTime)
	result.Metadata["detector_version"] = "1.0.0"

	n.logger.WithFields(map[string]interface{}{
		"confidence":      result.Confidence,
		"marker_files":    len(result.MarkerFiles),
		"source_dirs":     len(result.SourceDirs),
		"dependencies":    len(result.Dependencies),
		"detection_time":  time.Since(startTime),
	}).Info("Node.js project detection completed")

	return result, nil
}

func (n *NodeJSLanguageDetector) parsePackageJson(packageJsonPath string, result *types.LanguageDetectionResult) error {
	content, err := os.ReadFile(packageJsonPath)
	if err != nil {
		return fmt.Errorf("failed to read package.json: %w", err)
	}

	var packageJson map[string]interface{}
	if err := json.Unmarshal(content, &packageJson); err != nil {
		return fmt.Errorf("failed to parse package.json: %w", err)
	}

	// Extract basic project information
	if name, ok := packageJson["name"].(string); ok {
		result.Metadata["project_name"] = name
	}

	if version, ok := packageJson["version"].(string); ok {
		result.Metadata["project_version"] = version
	}

	if description, ok := packageJson["description"].(string); ok {
		result.Metadata["description"] = description
	}

	if main, ok := packageJson["main"].(string); ok {
		result.Metadata["main_file"] = main
	}

	// Extract scripts
	if scripts, ok := packageJson["scripts"].(map[string]interface{}); ok {
		scriptMap := make(map[string]string)
		for key, value := range scripts {
			if strValue, ok := value.(string); ok {
				scriptMap[key] = strValue
			}
		}
		result.Metadata["scripts"] = scriptMap
	}

	// Extract dependencies
	if deps, ok := packageJson["dependencies"].(map[string]interface{}); ok {
		for name, version := range deps {
			if versionStr, ok := version.(string); ok {
				result.Dependencies[name] = versionStr
			}
		}
	}

	// Extract dev dependencies
	if devDeps, ok := packageJson["devDependencies"].(map[string]interface{}); ok {
		for name, version := range devDeps {
			if versionStr, ok := version.(string); ok {
				result.DevDependencies[name] = versionStr
			}
		}
	}

	// Extract engines
	if engines, ok := packageJson["engines"].(map[string]interface{}); ok {
		if nodeVersion, ok := engines["node"].(string); ok {
			result.Version = nodeVersion
			result.Metadata["node_version_requirement"] = nodeVersion
		}
		if npmVersion, ok := engines["npm"].(string); ok {
			result.Metadata["npm_version_requirement"] = npmVersion
		}
	}

	// Extract keywords
	if keywords, ok := packageJson["keywords"].([]interface{}); ok {
		var keywordStrings []string
		for _, keyword := range keywords {
			if keywordStr, ok := keyword.(string); ok {
				keywordStrings = append(keywordStrings, keywordStr)
			}
		}
		result.Metadata["keywords"] = keywordStrings
	}

	return nil
}

func (n *NodeJSLanguageDetector) scanForNodeFiles(path string, result *types.LanguageDetectionResult, nodeFileCount *int) error {
	sourceDirs := make(map[string]bool)
	testDirs := make(map[string]bool)
	
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue on errors
		}

		// Skip node_modules and other common ignore directories
		if info.IsDir() {
			name := info.Name()
			skipDirs := []string{"node_modules", ".git", "dist", "build", "coverage", ".nyc_output"}
			for _, skipDir := range skipDirs {
				if name == skipDir {
					return filepath.SkipDir
				}
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(filePath))
		if ext == types.EXT_JS || ext == types.EXT_TS || ext == ".jsx" || ext == types.EXT_TSX || ext == ".mjs" || ext == ".cjs" {
			*nodeFileCount++
			dir := filepath.Dir(filePath)
			relDir, _ := filepath.Rel(path, dir)
			
			fileName := strings.ToLower(info.Name())
			if strings.Contains(fileName, "test") || strings.Contains(fileName, "spec") ||
			   strings.Contains(relDir, "test") || strings.Contains(relDir, "__tests__") {
				testDirs[relDir] = true
			} else {
				sourceDirs[relDir] = true
			}
		}

		// Look for build and configuration files
		buildFiles := []string{"webpack.config.js", "rollup.config.js", "vite.config.js", 
			"babel.config.js", ".babelrc", "gulpfile.js", "Gruntfile.js"}
		for _, buildFile := range buildFiles {
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

func (n *NodeJSLanguageDetector) detectNodeVersion(ctx context.Context, result *types.LanguageDetectionResult) error {
	// Try to get Node.js version from installed Node
	cmd := []string{"node", "--version"}
	execResult, err := n.executor.Execute(cmd[0], cmd[1:], 10*time.Second)
	if err != nil {
		return err
	}

	if execResult.ExitCode == 0 {
		output := strings.TrimSpace(execResult.Stdout)
		// Parse "v18.17.0"
		version := strings.TrimPrefix(output, "v")
		if result.Version == "" { // Only set if not already set from package.json
			result.Version = version
		}
		result.Metadata["installed_node_version"] = version
	}

	// Also try to get npm version
	npmCmd := []string{"npm", "--version"}
	npmResult, err := n.executor.Execute(npmCmd[0], npmCmd[1:], 10*time.Second)
	if err == nil && npmResult.ExitCode == 0 {
		npmVersion := strings.TrimSpace(npmResult.Stdout)
		result.Metadata["installed_npm_version"] = npmVersion
	}

	return nil
}

// getFrameworkName returns the framework name for a given dependency name
func (n *NodeJSLanguageDetector) getFrameworkName(dep string) (string, bool) {
	framework, exists := frameworkMap[dep]
	return framework, exists
}

// detectProjectTypes determines project types based on detected frameworks
func (n *NodeJSLanguageDetector) detectProjectTypes(frameworks []string) []string {
	projectTypesSet := make(map[string]bool)
	for _, framework := range frameworks {
		if types, exists := frameworkProjectTypes[framework]; exists {
			for _, projectType := range types {
				projectTypesSet[projectType] = true
			}
		}
	}
	
	projectTypes := make([]string, 0, len(projectTypesSet))
	for projectType := range projectTypesSet {
		projectTypes = append(projectTypes, projectType)
	}
	return projectTypes
}

func (n *NodeJSLanguageDetector) detectNodeFrameworks(result *types.LanguageDetectionResult) {
	frameworks := []string{}
	
	// Check dependencies for known frameworks
	allDeps := make(map[string]string)
	for k, v := range result.Dependencies {
		allDeps[k] = v
	}
	for k, v := range result.DevDependencies {
		allDeps[k] = v
	}

	for dep := range allDeps {
		if framework, exists := n.getFrameworkName(dep); exists {
			frameworks = append(frameworks, framework)
		}
	}

	if len(frameworks) > 0 {
		result.Metadata["frameworks"] = frameworks
	}

	// Detect project type based on frameworks
	projectTypes := n.detectProjectTypes(frameworks)
	if len(projectTypes) > 0 {
		result.Metadata["project_types"] = projectTypes
	}
}

func (n *NodeJSLanguageDetector) analyzeProjectStructure(path string, result *types.LanguageDetectionResult) {
	commonDirs := []string{"src", "lib", "dist", "build", "public", "static", "assets", "components", "pages", "views", "routes", "middleware", "models", "controllers", "services", "utils", "helpers", "tests", "__tests__", "spec"}
	foundDirs := []string{}

	for _, dir := range commonDirs {
		dirPath := filepath.Join(path, dir)
		if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
			foundDirs = append(foundDirs, dir)
		}
	}

	if len(foundDirs) > 0 {
		result.Metadata["project_structure"] = foundDirs
	}

	// Check for common Node.js entry points
	entryPoints := []string{"index.js", "app.js", "server.js", "main.js", "index.ts", "app.ts", "server.ts", "main.ts"}
	for _, entryPoint := range entryPoints {
		if _, err := os.Stat(filepath.Join(path, entryPoint)); err == nil {
			result.Metadata["entry_point"] = entryPoint
			break
		}
	}

	// Check for common directories to determine project type
	if _, err := os.Stat(filepath.Join(path, "public")); err == nil {
		if _, err := os.Stat(filepath.Join(path, "src")); err == nil {
			result.Metadata["is_spa_project"] = true
		}
	}

	if _, err := os.Stat(filepath.Join(path, "pages")); err == nil {
		result.Metadata["is_pages_based"] = true
	}
}

func (n *NodeJSLanguageDetector) detectAdditionalConfigs(path string, result *types.LanguageDetectionResult) {
	configFiles := map[string]string{
		".eslintrc.js":     "ESLint",
		".eslintrc.json":   "ESLint",
		".prettierrc":      "Prettier",
		"tsconfig.json":    "TypeScript",
		"jest.config.js":   "Jest",
		"cypress.json":     "Cypress",
		"webpack.config.js": "Webpack",
		"vite.config.js":   "Vite",
		"rollup.config.js": "Rollup",
		".babelrc":         "Babel",
		"babel.config.js":  "Babel",
		"nodemon.json":     "Nodemon",
		"pm2.config.js":    "PM2",
		"Dockerfile":       "Docker",
		"docker-compose.yml": "Docker Compose",
		".nvmrc":           "NVM",
		".node-version":    "Node Version",
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
// GetLanguageInfo returns comprehensive information about Node.js language support
func (n *NodeJSLanguageDetector) GetLanguageInfo(language string) (*types.LanguageInfo, error) {
	if language != types.PROJECT_TYPE_NODEJS {
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	return &types.LanguageInfo{
		Name:           types.PROJECT_TYPE_NODEJS,
		DisplayName:    "Node.js",
		MinVersion:     "14.0.0",
		MaxVersion:     "21.0.0",
		BuildTools:     []string{"npm", "yarn", "pnpm", "webpack", "vite", "parcel"},
		PackageManager: "npm",
		TestFrameworks: []string{"jest", "mocha", "vitest", "tap", "ava"},
		LintTools:      []string{"eslint", "jshint", "standardjs"},
		FormatTools:    []string{"prettier", "standardjs"},
		LSPServers:     []string{types.SERVER_TYPESCRIPT_LANG_SERVER},
		FileExtensions: []string{".js", ".mjs", ".cjs", ".json"},
		Capabilities:   []string{"completion", "hover", "definition", "references", "formatting", "code_action", "rename"},
		Metadata: map[string]interface{}{
			"package_manager_support": []string{"npm", "yarn", "pnpm"},
			"runtime":                 "Node.js",
			"documentation":           "nodejs.org",
		},
	}, nil
}

func (n *NodeJSLanguageDetector) GetMarkerFiles() []string {
	return []string{types.MARKER_PACKAGE_JSON, types.MARKER_YARN_LOCK, types.MARKER_PNPM_LOCK}
}

func (n *NodeJSLanguageDetector) GetRequiredServers() []string {
	return []string{types.SERVER_TYPESCRIPT_LANG_SERVER}
}

func (n *NodeJSLanguageDetector) GetPriority() int {
	return types.PRIORITY_NODEJS
}

func (n *NodeJSLanguageDetector) ValidateStructure(ctx context.Context, path string) error {
	// Check for package.json
	packageJsonPath := filepath.Join(path, types.MARKER_PACKAGE_JSON)
	if _, err := os.Stat(packageJsonPath); os.IsNotExist(err) {
		return types.NewValidationError(types.PROJECT_TYPE_NODEJS, "package.json file not found", nil).
			WithMetadata("missing_files", []string{types.MARKER_PACKAGE_JSON}).
			WithSuggestion("Initialize Node.js project: npm init").
			WithSuggestion("Ensure you're in the correct Node.js project directory")
	}

	// Check if package.json is valid JSON
	content, err := os.ReadFile(packageJsonPath)
	if err != nil {
		return types.NewValidationError(types.PROJECT_TYPE_NODEJS, "cannot read package.json file", err).
			WithMetadata("invalid_files", []string{types.MARKER_PACKAGE_JSON})
	}

	var packageJson map[string]interface{}
	if err := json.Unmarshal(content, &packageJson); err != nil {
		return types.NewValidationError(types.PROJECT_TYPE_NODEJS, "package.json is not valid JSON", err).
			WithMetadata("invalid_files", []string{types.MARKER_PACKAGE_JSON})
	}

	// Check for JavaScript/TypeScript files
	hasJSFiles := false
	err = filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		
		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(filePath))
			if ext == types.EXT_JS || ext == types.EXT_TS || ext == ".jsx" || ext == types.EXT_TSX || ext == ".mjs" || ext == ".cjs" {
				hasJSFiles = true
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
		return types.NewValidationError(types.PROJECT_TYPE_NODEJS, "error scanning for JavaScript/TypeScript files", err)
	}

	if !hasJSFiles {
		return types.NewValidationError(types.PROJECT_TYPE_NODEJS, "no JavaScript/TypeScript source files found", nil).
			WithMetadata("structure_issues", []string{"no " + types.EXT_JS + ", .ts, .jsx, " + types.EXT_TSX + " files found in project"})
	}

	return nil
}