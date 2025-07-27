package detectors

import (
	"context"
	"fmt"
	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/setup"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// TypeScriptProjectDetector implements LanguageDetector interface for TypeScript/JavaScript project detection
type TypeScriptProjectDetector struct {
	logger          *setup.SetupLogger
	executor        platform.CommandExecutor
	versionChecker  *setup.VersionChecker
	nodeDetector    *setup.NodeJSDetector
	parser          *PackageJsonParser
	tsConfigParser  *TsConfigParser
	lockfileParser  *LockfileParser
	workspaceParser *WorkspaceParser
	timeout         time.Duration
}

// TypeScriptDetectionMetadata contains TypeScript/JavaScript-specific detection information
type TypeScriptDetectionMetadata struct {
	PackageInfo       *PackageJsonInfo                           `json:"package_info,omitempty"`
	TSConfigInfo      *TSConfigInfo                              `json:"tsconfig_info,omitempty"`
	NodeJSInfo        *setup.NodeJSInfo                          `json:"nodejs_info,omitempty"`
	Framework         *FrameworkInfo                             `json:"framework,omitempty"`
	PackageManager    *PackageManagerInfo                        `json:"package_manager,omitempty"`
	BuildTools        []string                                   `json:"build_tools,omitempty"`
	TestFrameworks    []string                                   `json:"test_frameworks,omitempty"`
	LintingTools      []string                                   `json:"linting_tools,omitempty"`
	TypeScriptVersion string                                     `json:"typescript_version,omitempty"`
	ModuleType        string                                     `json:"module_type,omitempty"` // "commonjs", "module", "mixed"
	MonorepoInfo      *MonorepoInfo                              `json:"monorepo_info,omitempty"`
	Dependencies      map[string]*types.TypeScriptDependencyInfo `json:"dependencies,omitempty"`
	PeerDependencies  map[string]*types.TypeScriptDependencyInfo `json:"peer_dependencies,omitempty"`
	OptionalDeps      map[string]*types.TypeScriptDependencyInfo `json:"optional_dependencies,omitempty"`
	ProjectType       string                                     `json:"project_type,omitempty"` // "library", "application", "tool", "monorepo"
}

// FrameworkInfo contains detected framework information
type FrameworkInfo struct {
	Name            string   `json:"name"`
	Version         string   `json:"version,omitempty"`
	ConfigFiles     []string `json:"config_files,omitempty"`
	Dependencies    []string `json:"dependencies,omitempty"`
	DevDependencies []string `json:"dev_dependencies,omitempty"`
	BuildCommand    string   `json:"build_command,omitempty"`
	DevCommand      string   `json:"dev_command,omitempty"`
	TestCommand     string   `json:"test_command,omitempty"`
}

// PackageManagerInfo contains package manager detection results
type PackageManagerInfo struct {
	Name        string   `json:"name"`
	Version     string   `json:"version,omitempty"`
	LockFile    string   `json:"lock_file,omitempty"`
	Command     string   `json:"command"`
	Available   bool     `json:"available"`
	Issues      []string `json:"issues,omitempty"`
	Recommended bool     `json:"recommended"`
	InstallCmd  string   `json:"install_cmd,omitempty"`
	RunCmd      string   `json:"run_cmd,omitempty"`
	AddCmd      string   `json:"add_cmd,omitempty"`
}

// MonorepoInfo contains monorepo detection results
type MonorepoInfo struct {
	Type          string   `json:"type"` // "lerna", "nx", "rush", "yarn-workspaces", "pnpm-workspaces"
	ConfigFile    string   `json:"config_file,omitempty"`
	WorkspaceRoot string   `json:"workspace_root,omitempty"`
	Packages      []string `json:"packages,omitempty"`
	Scripts       []string `json:"scripts,omitempty"`
}

// types.TypeScriptDependencyInfo contains detailed dependency information

// NewTypeScriptProjectDetector creates a new TypeScript/JavaScript project detector
func NewTypeScriptProjectDetector() *TypeScriptProjectDetector {
	logger := setup.NewSetupLogger(&setup.SetupLoggerConfig{
		Component: "typescript-project-detector",
		Level:     setup.LogLevelInfo,
	})

	return &TypeScriptProjectDetector{
		logger:          logger,
		executor:        platform.NewCommandExecutor(),
		versionChecker:  setup.NewVersionChecker(),
		nodeDetector:    setup.NewNodeJSDetector(logger),
		parser:          NewPackageJsonParser(logger),
		tsConfigParser:  NewTsConfigParser(logger),
		lockfileParser:  NewLockfileParser(logger),
		workspaceParser: NewWorkspaceParser(logger),
		timeout:         45 * time.Second,
	}
}

// DetectLanguage implements LanguageDetector.DetectLanguage
func (d *TypeScriptProjectDetector) DetectLanguage(ctx context.Context, path string) (*types.LanguageDetectionResult, error) {
	startTime := time.Now()

	d.logger.WithField("path", path).Info("Starting TypeScript/JavaScript project detection")

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	// Initialize result
	result := &types.LanguageDetectionResult{
		Language:        types.PROJECT_TYPE_TYPESCRIPT,
		Confidence:      0.0,
		MarkerFiles:     []string{},
		ConfigFiles:     []string{},
		SourceDirs:      []string{},
		TestDirs:        []string{},
		Dependencies:    make(map[string]string),
		DevDependencies: make(map[string]string),
		BuildFiles:      []string{},
		RequiredServers: []string{types.SERVER_TYPESCRIPT_LANG_SERVER},
		Metadata:        make(map[string]interface{}),
	}

	// Check for early exit due to context cancellation
	select {
	case <-timeoutCtx.Done():
		return nil, types.NewDetectionError(types.PROJECT_TYPE_TYPESCRIPT, "detection", path,
			"TypeScript detection timed out", timeoutCtx.Err())
	default:
	}

	metadata := &TypeScriptDetectionMetadata{
		Dependencies:     make(map[string]*types.TypeScriptDependencyInfo),
		PeerDependencies: make(map[string]*types.TypeScriptDependencyInfo),
		OptionalDeps:     make(map[string]*types.TypeScriptDependencyInfo),
	}

	// Step 1: Detect Node.js runtime and validate installation
	confidence, err := d.detectNodeJSRuntime(timeoutCtx, path, result, metadata)
	if err != nil {
		d.logger.WithError(err).Debug("Node.js runtime detection failed")
		// Don't return error - might still detect basic TypeScript/JavaScript project structure
	}

	// Step 2: Analyze package.json and project structure
	packageConfidence, err := d.analyzePackageJson(timeoutCtx, path, result, metadata)
	if err != nil {
		d.logger.WithError(err).Debug("Package.json analysis failed")
	}
	confidence = max(confidence, packageConfidence)

	// Step 3: Analyze TypeScript configuration
	tsConfidence, err := d.analyzeTSConfig(timeoutCtx, path, result, metadata)
	if err != nil {
		d.logger.WithError(err).Debug("TypeScript config analysis failed")
	}
	confidence = max(confidence, tsConfidence)

	// Step 4: Detect frameworks and build tools
	frameworkConfidence, err := d.detectFrameworks(timeoutCtx, path, result, metadata)
	if err != nil {
		d.logger.WithError(err).Debug("Framework detection failed")
	}
	confidence = max(confidence, frameworkConfidence)

	// Step 5: Analyze source code structure
	sourceConfidence, err := d.analyzeSourceStructure(timeoutCtx, path, result, metadata)
	if err != nil {
		d.logger.WithError(err).Debug("Source structure analysis failed")
	}
	confidence = max(confidence, sourceConfidence)

	// Step 6: Detect package manager and lockfiles
	pmConfidence, err := d.detectPackageManager(timeoutCtx, path, result, metadata)
	if err != nil {
		d.logger.WithError(err).Debug("Package manager detection failed")
	}
	confidence = max(confidence, pmConfidence)

	// Step 7: Check for monorepo structure
	monorepoConfidence, err := d.detectMonorepo(timeoutCtx, path, result, metadata)
	if err != nil {
		d.logger.WithError(err).Debug("Monorepo detection failed")
	}
	confidence = max(confidence, monorepoConfidence)

	// Step 8: Finalize detection results
	result.Confidence = confidence
	
	// Add bonus confidence for having multiple TypeScript indicators
	tsIndicators := 0
	if metadata.TSConfigInfo != nil {
		tsIndicators++
	}
	if d.hasTypeScriptFiles(path) {
		tsIndicators++
	}
	if d.hasTypeScriptDependencies(metadata.PackageInfo) {
		tsIndicators++
	}
	
	// Add bonus for multiple indicators (synergy bonus)
	if tsIndicators >= 2 {
		confidence += 0.05 * float64(tsIndicators-1)
		if confidence > 1.0 {
			confidence = 1.0
		}
		result.Confidence = confidence
	}
	
	result.Metadata["typescript_metadata"] = metadata

	// Determine final project type based on analysis
	d.determineProjectType(result, metadata)

	// Adjust language based on TypeScript presence
	if metadata.TSConfigInfo != nil || d.hasTypeScriptFiles(path) {
		result.Language = types.PROJECT_TYPE_TYPESCRIPT
	} else {
		result.Language = types.PROJECT_TYPE_NODEJS
	}

	d.logger.WithFields(map[string]interface{}{
		"confidence":     result.Confidence,
		"language":       result.Language,
		"detection_time": time.Since(startTime),
		"marker_files":   len(result.MarkerFiles),
		"dependencies":   len(result.Dependencies),
	}).Info("TypeScript/JavaScript project detection completed")

	return result, nil
}

// GetLanguageInfo implements LanguageDetector.GetLanguageInfo
func (d *TypeScriptProjectDetector) GetLanguageInfo(language string) (*types.LanguageInfo, error) {
	if language != types.PROJECT_TYPE_TYPESCRIPT && language != types.PROJECT_TYPE_NODEJS {
		return nil, types.NewProjectError(types.ProjectErrorTypeConfiguration, "typescript_detector", "",
			fmt.Sprintf("unsupported project type: %s", language), nil)
	}

	info := &types.LanguageInfo{
		Name:           language,
		DisplayName:    "TypeScript/JavaScript",
		FileExtensions: []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"},
		MinVersion:     "14.0.0",
		MaxVersion:     "",
		BuildTools:     []string{"webpack", "vite", "rollup", "parcel", "esbuild", "swc", "tsc"},
		PackageManager: "npm",
		TestFrameworks: []string{"jest", "vitest", "mocha", "jasmine", "cypress", "playwright", "testing-library"},
		LintTools:      []string{"eslint", "tslint", "jshint", "prettier", "biome"},
		FormatTools:    []string{"prettier", "biome"},
		LSPServers:     []string{types.SERVER_TYPESCRIPT_LANG_SERVER},
		Capabilities:   []string{"static_analysis", "code_completion", "refactoring", "type_checking"},
		Metadata: map[string]interface{}{
			"marker_files": d.GetMarkerFiles(),
			"supported_tools": map[string][]string{
				"build_tools":      {"webpack", "vite", "rollup", "parcel", "esbuild", "swc", "tsc"},
				"test_frameworks":  {"jest", "vitest", "mocha", "jasmine", "cypress", "playwright", "testing-library"},
				"linters":          {"eslint", "tslint", "jshint", "prettier", "biome"},
				"package_managers": {"npm", "yarn", "pnpm", "bun"},
				"frameworks":       {"react", "vue", "angular", "svelte", "next", "nuxt", "express", "fastify"},
			},
			"configuration_files": []string{
				"package.json", "tsconfig.json", "jsconfig.json", ".eslintrc.*",
				".prettierrc.*", "webpack.config.*", "vite.config.*", "rollup.config.*",
				"jest.config.*", "vitest.config.*", "cypress.config.*", "playwright.config.*",
			},
			"default_directories": map[string]string{
				"source": "src",
				"tests":  "test,tests,__tests__,spec",
				"build":  "dist,build,out",
				"config": "config",
			},
			"version_requirements": map[string]string{
				"node":       ">=14.0.0",
				"typescript": ">=4.0.0",
				"npm":        ">=6.0.0",
			},
			"priority": types.PRIORITY_TYPESCRIPT,
		},
	}

	return info, nil
}

// GetMarkerFiles implements LanguageDetector.GetMarkerFiles
func (d *TypeScriptProjectDetector) GetMarkerFiles() []string {
	return []string{
		types.MARKER_PACKAGE_JSON,
		types.MARKER_TSCONFIG,
		"jsconfig.json",
		types.MARKER_YARN_LOCK,
		types.MARKER_PNPM_LOCK,
		"package-lock.json",
		"bun.lockb",
		".npmrc",
		".yarnrc",
		".yarnrc.yml",
		"pnpm-workspace.yaml",
		"lerna.json",
		"nx.json",
		"rush.json",
	}
}

// GetRequiredServers implements LanguageDetector.GetRequiredServers
func (d *TypeScriptProjectDetector) GetRequiredServers() []string {
	return []string{types.SERVER_TYPESCRIPT_LANG_SERVER}
}

// GetPriority implements LanguageDetector.GetPriority
func (d *TypeScriptProjectDetector) GetPriority() int {
	return types.PRIORITY_TYPESCRIPT
}

// ValidateStructure implements LanguageDetector.ValidateStructure
func (d *TypeScriptProjectDetector) ValidateStructure(ctx context.Context, rootPath string) error {
	d.logger.WithField("path", rootPath).Debug("Validating TypeScript/JavaScript project structure")

	var validationErrors []string

	// Check for package.json
	packageJsonPath := filepath.Join(rootPath, types.MARKER_PACKAGE_JSON)
	if _, err := os.Stat(packageJsonPath); os.IsNotExist(err) {
		validationErrors = append(validationErrors, "package.json not found")
	} else {
		// Validate package.json structure
		if err := d.parser.ValidatePackageJson(packageJsonPath); err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("Invalid package.json: %v", err))
		}
	}

	// Check for TypeScript configuration if TypeScript files are present
	if d.hasTypeScriptFiles(rootPath) {
		tsConfigPath := filepath.Join(rootPath, types.MARKER_TSCONFIG)
		if _, err := os.Stat(tsConfigPath); os.IsNotExist(err) {
			validationErrors = append(validationErrors, "tsconfig.json not found but TypeScript files detected")
		} else {
			// Validate tsconfig.json structure
			if err := d.tsConfigParser.ValidateTSConfig(tsConfigPath); err != nil {
				validationErrors = append(validationErrors, fmt.Sprintf("Invalid tsconfig.json: %v", err))
			}
		}
	}

	// Validate Node.js runtime availability
	nodeInfo, err := d.nodeDetector.DetectNodeJS()
	if err != nil {
		validationErrors = append(validationErrors, fmt.Sprintf("Node.js runtime validation failed: %v", err))
	} else if !nodeInfo.Compatible {
		validationErrors = append(validationErrors, "Node.js version is not compatible")
	}

	// Check for source directories
	sourceDirs := []string{"src", "lib", "app"}
	hasSourceDir := false
	for _, dir := range sourceDirs {
		if _, err := os.Stat(filepath.Join(rootPath, dir)); err == nil {
			hasSourceDir = true
			break
		}
	}
	if !hasSourceDir {
		// Check for JavaScript/TypeScript files in root
		if !d.hasJavaScriptFiles(rootPath) && !d.hasTypeScriptFiles(rootPath) {
			validationErrors = append(validationErrors, "No source files or directories found")
		}
	}

	if len(validationErrors) > 0 {
		return types.NewValidationError(types.PROJECT_TYPE_TYPESCRIPT, strings.Join(validationErrors, "; "), nil)
	}

	d.logger.Debug("TypeScript/JavaScript project structure validation passed")
	return nil
}

// Helper methods for detection phases

func (d *TypeScriptProjectDetector) detectNodeJSRuntime(ctx context.Context, path string, result *types.LanguageDetectionResult, metadata *TypeScriptDetectionMetadata) (float64, error) {
	d.logger.Debug("Detecting Node.js runtime")

	nodeInfo, err := d.nodeDetector.DetectNodeJS()
	if err != nil {
		return 0.0, err
	}

	metadata.NodeJSInfo = nodeInfo

	if nodeInfo.Installed && nodeInfo.Compatible {
		result.Version = nodeInfo.NodeVersion
		d.logger.WithField("node_version", nodeInfo.NodeVersion).Debug("Compatible Node.js runtime detected")
		return 0.2, nil
	}

	return 0.0, nil
}

func (d *TypeScriptProjectDetector) analyzePackageJson(ctx context.Context, path string, result *types.LanguageDetectionResult, metadata *TypeScriptDetectionMetadata) (float64, error) {
	packageJsonPath := filepath.Join(path, types.MARKER_PACKAGE_JSON)

	if _, err := os.Stat(packageJsonPath); os.IsNotExist(err) {
		return 0.0, nil
	}

	d.logger.WithField("file", packageJsonPath).Debug("Analyzing package.json")

	packageInfo, err := d.parser.ParsePackageJson(packageJsonPath)
	if err != nil {
		return 0.0, fmt.Errorf("failed to parse package.json: %w", err)
	}

	metadata.PackageInfo = packageInfo
	result.MarkerFiles = append(result.MarkerFiles, types.MARKER_PACKAGE_JSON)
	result.ConfigFiles = append(result.ConfigFiles, types.MARKER_PACKAGE_JSON)

	// Extract dependencies
	for name, version := range packageInfo.Dependencies {
		result.Dependencies[name] = version
		metadata.Dependencies[name] = &types.TypeScriptDependencyInfo{
			Name:    name,
			Version: version,
			Type:    "dependency",
		}
	}

	for name, version := range packageInfo.DevDependencies {
		result.DevDependencies[name] = version
		metadata.Dependencies[name] = &types.TypeScriptDependencyInfo{
			Name:    name,
			Version: version,
			Type:    "devDependency",
		}
	}

	// Analyze dependencies for framework/tool detection
	d.analyzeDependencies(metadata)

	confidence := 0.6 // Strong confidence for package.json presence

	// Boost confidence for TypeScript-specific dependencies
	if d.hasTypeScriptDependencies(packageInfo) {
		confidence += 0.2
		result.Language = types.PROJECT_TYPE_TYPESCRIPT
	}

	return confidence, nil
}

func (d *TypeScriptProjectDetector) analyzeTSConfig(ctx context.Context, path string, result *types.LanguageDetectionResult, metadata *TypeScriptDetectionMetadata) (float64, error) {
	tsConfigPath := filepath.Join(path, types.MARKER_TSCONFIG)

	if _, err := os.Stat(tsConfigPath); os.IsNotExist(err) {
		// Check for jsconfig.json as alternative
		jsConfigPath := filepath.Join(path, "jsconfig.json")
		if _, err := os.Stat(jsConfigPath); os.IsNotExist(err) {
			return 0.0, nil
		}
		tsConfigPath = jsConfigPath
	}

	d.logger.WithField("file", tsConfigPath).Debug("Analyzing TypeScript configuration")

	tsConfig, err := d.tsConfigParser.ParseTSConfig(tsConfigPath)
	if err != nil {
		return 0.0, fmt.Errorf("failed to parse TypeScript config: %w", err)
	}

	metadata.TSConfigInfo = tsConfig
	result.MarkerFiles = append(result.MarkerFiles, filepath.Base(tsConfigPath))
	result.ConfigFiles = append(result.ConfigFiles, filepath.Base(tsConfigPath))

	// Extract TypeScript version from compiler options or dependencies
	if compilerOpts := tsConfig.CompilerOptions; compilerOpts != nil {
		if target, ok := compilerOpts["target"].(string); ok {
			result.Metadata["typescript_target"] = target
		}
		if moduleType, ok := compilerOpts["module"].(string); ok {
			metadata.ModuleType = moduleType
		}
	}

	confidence := 0.4
	if strings.Contains(filepath.Base(tsConfigPath), "tsconfig") {
		confidence = 0.7 // Higher confidence for actual tsconfig.json
		result.Language = types.PROJECT_TYPE_TYPESCRIPT
	}

	return confidence, nil
}

func (d *TypeScriptProjectDetector) detectFrameworks(ctx context.Context, path string, result *types.LanguageDetectionResult, metadata *TypeScriptDetectionMetadata) (float64, error) {
	d.logger.Debug("Detecting frameworks and build tools")

	confidence := 0.0

	// Framework detection based on dependencies and config files
	if metadata.PackageInfo != nil {
		framework := d.detectFrameworkFromDependencies(metadata.PackageInfo)
		if framework != nil {
			metadata.Framework = framework
			result.Metadata["framework"] = framework.Name
			confidence += 0.2
		}
	}

	// Detect build tools from config files and dependencies
	buildTools := d.detectBuildTools(path, metadata)
	if len(buildTools) > 0 {
		metadata.BuildTools = buildTools
		result.Metadata["build_tools"] = buildTools
		confidence += 0.1
	}

	// Detect test frameworks
	testFrameworks := d.detectTestFrameworks(metadata)
	if len(testFrameworks) > 0 {
		metadata.TestFrameworks = testFrameworks
		result.Metadata["test_frameworks"] = testFrameworks
		confidence += 0.1
	}

	// Detect linting tools
	lintingTools := d.detectLintingTools(path, metadata)
	if len(lintingTools) > 0 {
		metadata.LintingTools = lintingTools
		result.Metadata["linting_tools"] = lintingTools
		confidence += 0.1
	}

	return confidence, nil
}

func (d *TypeScriptProjectDetector) analyzeSourceStructure(ctx context.Context, path string, result *types.LanguageDetectionResult, metadata *TypeScriptDetectionMetadata) (float64, error) {
	d.logger.Debug("Analyzing source code structure")

	confidence := 0.0

	// Find source directories
	sourceDirs := d.findSourceDirectories(path)
	result.SourceDirs = sourceDirs
	if len(sourceDirs) > 0 {
		confidence += 0.1
	}

	// Find test directories
	testDirs := d.findTestDirectories(path)
	result.TestDirs = testDirs

	// Check for TypeScript/JavaScript files
	hasTS := d.hasTypeScriptFiles(path)
	hasJS := d.hasJavaScriptFiles(path)

	if hasTS {
		confidence += 0.3
		result.Language = types.PROJECT_TYPE_TYPESCRIPT
		if tsVersion := d.detectTypeScriptVersion(metadata); tsVersion != "" {
			metadata.TypeScriptVersion = tsVersion
			result.Version = tsVersion
		}
	} else if hasJS {
		confidence += 0.2
	}

	// Determine module type
	moduleType := d.determineModuleType(path, metadata)
	if moduleType != "" {
		metadata.ModuleType = moduleType
		result.Metadata["module_type"] = moduleType
	}

	// Count files for project size estimation
	jsFiles := d.countJavaScriptFiles(path)
	tsFiles := d.countTypeScriptFiles(path)
	result.Metadata["javascript_files"] = jsFiles
	result.Metadata["typescript_files"] = tsFiles

	return confidence, nil
}

func (d *TypeScriptProjectDetector) detectPackageManager(ctx context.Context, path string, result *types.LanguageDetectionResult, metadata *TypeScriptDetectionMetadata) (float64, error) {
	d.logger.Debug("Detecting package manager")

	confidence := 0.0
	var packageManager *PackageManagerInfo

	// Check for lockfiles and determine package manager
	if _, err := os.Stat(filepath.Join(path, types.MARKER_YARN_LOCK)); err == nil {
		packageManager = d.detectYarn(path)
		result.MarkerFiles = append(result.MarkerFiles, types.MARKER_YARN_LOCK)
		confidence += 0.1
	} else if _, err := os.Stat(filepath.Join(path, types.MARKER_PNPM_LOCK)); err == nil {
		packageManager = d.detectPnpm(path)
		result.MarkerFiles = append(result.MarkerFiles, types.MARKER_PNPM_LOCK)
		confidence += 0.1
	} else if _, err := os.Stat(filepath.Join(path, "package-lock.json")); err == nil {
		packageManager = d.detectNpm(path)
		result.MarkerFiles = append(result.MarkerFiles, "package-lock.json")
		confidence += 0.1
	} else if _, err := os.Stat(filepath.Join(path, "bun.lockb")); err == nil {
		packageManager = d.detectBun(path)
		result.MarkerFiles = append(result.MarkerFiles, "bun.lockb")
		confidence += 0.1
	} else {
		// Default to npm if no lockfile found
		packageManager = d.detectNpm(path)
	}

	if packageManager != nil {
		metadata.PackageManager = packageManager
		result.Metadata["package_manager"] = packageManager.Name
	}

	return confidence, nil
}

func (d *TypeScriptProjectDetector) detectMonorepo(ctx context.Context, path string, result *types.LanguageDetectionResult, metadata *TypeScriptDetectionMetadata) (float64, error) {
	d.logger.Debug("Detecting monorepo structure")

	confidence := 0.0
	var monorepoInfo *MonorepoInfo

	// Check for various monorepo configuration files
	monorepoConfigs := map[string]string{
		"lerna.json":          "lerna",
		"nx.json":             "nx",
		"rush.json":           "rush",
		"pnpm-workspace.yaml": "pnpm-workspaces",
		"pnpm-workspace.yml":  "pnpm-workspaces",
	}

	for configFile, repoType := range monorepoConfigs {
		configPath := filepath.Join(path, configFile)
		if _, err := os.Stat(configPath); err == nil {
			monorepoInfo = &MonorepoInfo{
				Type:          repoType,
				ConfigFile:    configFile,
				WorkspaceRoot: path,
			}
			result.MarkerFiles = append(result.MarkerFiles, configFile)
			result.ConfigFiles = append(result.ConfigFiles, configFile)
			confidence += 0.2
			break
		}
	}

	// Check for yarn/npm workspaces
	if monorepoInfo == nil && metadata.PackageInfo != nil {
		if workspaces := metadata.PackageInfo.Workspaces; len(workspaces) > 0 {
			pmType := "npm-workspaces"
			if metadata.PackageManager != nil && metadata.PackageManager.Name == "yarn" {
				pmType = "yarn-workspaces"
			}
			monorepoInfo = &MonorepoInfo{
				Type:          pmType,
				ConfigFile:    types.MARKER_PACKAGE_JSON,
				WorkspaceRoot: path,
				Packages:      workspaces,
			}
			confidence += 0.2
		}
	}

	if monorepoInfo != nil {
		metadata.MonorepoInfo = monorepoInfo
		result.Metadata["monorepo_type"] = monorepoInfo.Type
		result.Metadata["is_monorepo"] = true

		// Parse workspace packages
		packages, err := d.workspaceParser.ParseWorkspaces(path, monorepoInfo)
		if err == nil && len(packages) > 0 {
			monorepoInfo.Packages = packages
			result.Metadata["workspace_packages"] = len(packages)
		}
	}

	return confidence, nil
}

// Helper utility methods

func (d *TypeScriptProjectDetector) hasTypeScriptFiles(path string) bool {
	tsPattern := regexp.MustCompile(`\.tsx?$`)
	return d.hasFilesMatchingPattern(path, tsPattern)
}

func (d *TypeScriptProjectDetector) hasJavaScriptFiles(path string) bool {
	jsPattern := regexp.MustCompile(`\.(js|jsx|mjs|cjs)$`)
	return d.hasFilesMatchingPattern(path, jsPattern)
}

func (d *TypeScriptProjectDetector) hasFilesMatchingPattern(root string, pattern *regexp.Regexp) bool {
	found := false
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || found {
			return err
		}
		if !info.IsDir() && pattern.MatchString(info.Name()) {
			found = true
			return filepath.SkipDir
		}
		// Skip node_modules and other common directories
		if info.IsDir() && (info.Name() == types.DIR_NODE_MODULES || info.Name() == types.DIR_DOT_GIT || info.Name() == "dist" || info.Name() == "build") {
			return filepath.SkipDir
		}
		return nil
	})
	return found
}

func (d *TypeScriptProjectDetector) countTypeScriptFiles(path string) int {
	return d.countFilesWithExtensions(path, []string{".ts", ".tsx"})
}

func (d *TypeScriptProjectDetector) countJavaScriptFiles(path string) int {
	return d.countFilesWithExtensions(path, []string{".js", ".jsx", ".mjs", ".cjs"})
}

func (d *TypeScriptProjectDetector) countFilesWithExtensions(root string, extensions []string) int {
	count := 0
	extMap := make(map[string]bool)
	for _, ext := range extensions {
		extMap[ext] = true
	}

	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			ext := filepath.Ext(info.Name())
			if extMap[ext] {
				count++
			}
		}
		// Skip node_modules and other directories
		if info.IsDir() && (info.Name() == types.DIR_NODE_MODULES || info.Name() == types.DIR_DOT_GIT || info.Name() == "dist" || info.Name() == "build") {
			return filepath.SkipDir
		}
		return nil
	})
	return count
}

func (d *TypeScriptProjectDetector) findSourceDirectories(path string) []string {
	var sourceDirs []string
	commonSourceDirs := []string{"src", "lib", "app", "source", "client", "server"}

	for _, dir := range commonSourceDirs {
		dirPath := filepath.Join(path, dir)
		if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
			sourceDirs = append(sourceDirs, dir)
		}
	}

	return sourceDirs
}

func (d *TypeScriptProjectDetector) findTestDirectories(path string) []string {
	var testDirs []string
	commonTestDirs := []string{"test", "tests", "__tests__", "spec", "specs", "e2e", "integration"}

	for _, dir := range commonTestDirs {
		dirPath := filepath.Join(path, dir)
		if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
			testDirs = append(testDirs, dir)
		}
	}

	return testDirs
}

func (d *TypeScriptProjectDetector) hasTypeScriptDependencies(packageInfo *PackageJsonInfo) bool {
	tsDepNames := []string{"typescript", "@types/node", "ts-node", "tsx", "tsc"}

	allDeps := make(map[string]string)
	for k, v := range packageInfo.Dependencies {
		allDeps[k] = v
	}
	for k, v := range packageInfo.DevDependencies {
		allDeps[k] = v
	}

	for _, depName := range tsDepNames {
		if _, exists := allDeps[depName]; exists {
			return true
		}
	}

	return false
}

func (d *TypeScriptProjectDetector) analyzeDependencies(metadata *TypeScriptDetectionMetadata) {
	if metadata.PackageInfo == nil {
		return
	}

	// Classify dependencies
	for name, info := range metadata.Dependencies {
		d.classifyDependency(name, info)
	}
}

func (d *TypeScriptProjectDetector) classifyDependency(name string, info *types.TypeScriptDependencyInfo) {
	// Framework detection
	frameworks := map[string]bool{
		"react": true, "vue": true, "angular": true, "@angular/core": true,
		"svelte": true, "next": true, "nuxt": true, "gatsby": true,
		"express": true, "fastify": true, "koa": true, "nestjs": true,
	}
	if frameworks[name] || strings.Contains(name, "react") || strings.Contains(name, "vue") || strings.Contains(name, "@angular") {
		info.IsFramework = true
	}

	// Build tool detection
	buildTools := map[string]bool{
		"webpack": true, "vite": true, "rollup": true, "parcel": true,
		"esbuild": true, "swc": true, "babel": true, "@babel/core": true,
		"tsc": true, "@swc/core": true,
	}
	if buildTools[name] || strings.Contains(name, "webpack") || strings.Contains(name, "babel") || strings.Contains(name, "@swc") {
		info.IsBuildTool = true
	}

	// Test framework detection
	testFrameworks := map[string]bool{
		"jest": true, "vitest": true, "mocha": true, "jasmine": true,
		"cypress": true, "playwright": true, "@testing-library/react": true,
		"@testing-library/vue": true, "karma": true, "puppeteer": true,
	}
	if testFrameworks[name] || strings.Contains(name, "test") || strings.Contains(name, "@testing-library") {
		info.IsTestFramework = true
	}

	// Linter detection
	linters := map[string]bool{
		"eslint": true, "tslint": true, "prettier": true, "jshint": true,
		"stylelint": true, "biome": true, "@typescript-eslint/parser": true,
	}
	if linters[name] || strings.Contains(name, "eslint") || strings.Contains(name, "lint") {
		info.IsLinter = true
	}
}

func (d *TypeScriptProjectDetector) detectFrameworkFromDependencies(packageInfo *PackageJsonInfo) *FrameworkInfo {
	allDeps := make(map[string]string)
	for k, v := range packageInfo.Dependencies {
		allDeps[k] = v
	}
	for k, v := range packageInfo.DevDependencies {
		allDeps[k] = v
	}

	// Framework priority order
	frameworks := []struct {
		name     string
		keys     []string
		config   []string
		buildCmd string
		devCmd   string
		testCmd  string
	}{
		{"Next.js", []string{"next"}, []string{"next.config.js", "next.config.ts"}, "next build", "next dev", "next test"},
		{"Nuxt.js", []string{"nuxt", "nuxt3"}, []string{"nuxt.config.js", "nuxt.config.ts"}, "nuxt build", "nuxt dev", "nuxt test"},
		{"React", []string{"react"}, []string{}, "react-scripts build", "react-scripts start", "react-scripts test"},
		{"Vue", []string{"vue", "@vue/core"}, []string{"vue.config.js"}, "vue-cli-service build", "vue-cli-service serve", "vue-cli-service test"},
		{"Angular", []string{"@angular/core"}, []string{"angular.json", ".angular-cli.json"}, "ng build", "ng serve", "ng test"},
		{"Svelte", []string{"svelte"}, []string{"svelte.config.js"}, "svelte-kit build", "svelte-kit dev", "svelte-kit test"},
		{"Express", []string{"express"}, []string{}, "node server.js", "nodemon server.js", "npm test"},
		{"Fastify", []string{"fastify"}, []string{}, "node server.js", "nodemon server.js", "npm test"},
	}

	for _, fw := range frameworks {
		for _, key := range fw.keys {
			if version, exists := allDeps[key]; exists {
				return &FrameworkInfo{
					Name:         fw.name,
					Version:      version,
					ConfigFiles:  fw.config,
					BuildCommand: fw.buildCmd,
					DevCommand:   fw.devCmd,
					TestCommand:  fw.testCmd,
				}
			}
		}
	}

	return nil
}

func (d *TypeScriptProjectDetector) detectBuildTools(path string, metadata *TypeScriptDetectionMetadata) []string {
	var buildTools []string

	// Check for build tool config files
	buildConfigs := map[string]string{
		"webpack.config.js": "webpack",
		"webpack.config.ts": "webpack",
		"vite.config.js":    "vite",
		"vite.config.ts":    "vite",
		"rollup.config.js":  "rollup",
		"rollup.config.ts":  "rollup",
		"parcel.config.js":  "parcel",
		"parcel.config.ts":  "parcel",
		"esbuild.config.js": "esbuild",
		"babel.config.js":   "babel",
		".babelrc":          "babel",
	}

	for configFile, tool := range buildConfigs {
		if _, err := os.Stat(filepath.Join(path, configFile)); err == nil {
			buildTools = append(buildTools, tool)
		}
	}

	// Check dependencies for build tools
	if metadata.PackageInfo != nil {
		buildToolDeps := []string{"webpack", "vite", "rollup", "parcel", "esbuild", "@swc/core", "babel", "@babel/core"}
		allDeps := make(map[string]string)
		for k, v := range metadata.PackageInfo.Dependencies {
			allDeps[k] = v
		}
		for k, v := range metadata.PackageInfo.DevDependencies {
			allDeps[k] = v
		}

		for _, tool := range buildToolDeps {
			if _, exists := allDeps[tool]; exists {
				if !containsString(buildTools, tool) {
					buildTools = append(buildTools, tool)
				}
			}
		}
	}

	return buildTools
}

func (d *TypeScriptProjectDetector) detectTestFrameworks(metadata *TypeScriptDetectionMetadata) []string {
	var testFrameworks []string

	if metadata.PackageInfo == nil {
		return testFrameworks
	}

	testDeps := []string{"jest", "vitest", "mocha", "jasmine", "cypress", "playwright", "@testing-library/react", "@testing-library/vue", "karma"}
	allDeps := make(map[string]string)
	for k, v := range metadata.PackageInfo.Dependencies {
		allDeps[k] = v
	}
	for k, v := range metadata.PackageInfo.DevDependencies {
		allDeps[k] = v
	}

	for _, framework := range testDeps {
		if _, exists := allDeps[framework]; exists {
			testFrameworks = append(testFrameworks, framework)
		}
	}

	return testFrameworks
}

func (d *TypeScriptProjectDetector) detectLintingTools(path string, metadata *TypeScriptDetectionMetadata) []string {
	var lintingTools []string

	// Check for linting config files
	lintConfigs := map[string]string{
		".eslintrc.js":        "eslint",
		".eslintrc.json":      "eslint",
		".eslintrc.yml":       "eslint",
		".eslintrc.yaml":      "eslint",
		".eslintrc":           "eslint",
		"eslint.config.js":    "eslint",
		".prettierrc":         "prettier",
		".prettierrc.js":      "prettier",
		".prettierrc.json":    "prettier",
		"prettier.config.js":  "prettier",
		"tslint.json":         "tslint",
		"stylelint.config.js": "stylelint",
	}

	for configFile, tool := range lintConfigs {
		if _, err := os.Stat(filepath.Join(path, configFile)); err == nil {
			if !containsString(lintingTools, tool) {
				lintingTools = append(lintingTools, tool)
			}
		}
	}

	// Check dependencies for linting tools
	if metadata.PackageInfo != nil {
		lintDeps := []string{"eslint", "prettier", "tslint", "stylelint", "biome", "@typescript-eslint/parser"}
		allDeps := make(map[string]string)
		for k, v := range metadata.PackageInfo.Dependencies {
			allDeps[k] = v
		}
		for k, v := range metadata.PackageInfo.DevDependencies {
			allDeps[k] = v
		}

		for _, tool := range lintDeps {
			if _, exists := allDeps[tool]; exists {
				baseTool := strings.Split(tool, "/")[0]
				if baseTool == "@typescript-eslint" {
					baseTool = "eslint"
				}
				if !containsString(lintingTools, baseTool) {
					lintingTools = append(lintingTools, baseTool)
				}
			}
		}
	}

	return lintingTools
}

func (d *TypeScriptProjectDetector) detectTypeScriptVersion(metadata *TypeScriptDetectionMetadata) string {
	if metadata.PackageInfo == nil {
		return ""
	}

	// Check devDependencies first, then dependencies
	if version, exists := metadata.PackageInfo.DevDependencies["typescript"]; exists {
		return version
	}
	if version, exists := metadata.PackageInfo.Dependencies["typescript"]; exists {
		return version
	}

	// Try to detect from installed TypeScript
	result, err := d.executor.Execute("tsc", []string{"--version"}, 5*time.Second)
	if err == nil && result.ExitCode == 0 {
		// Parse "Version 4.9.5" format
		if strings.HasPrefix(result.Stdout, "Version ") {
			return strings.TrimSpace(strings.TrimPrefix(result.Stdout, "Version "))
		}
	}

	return ""
}

func (d *TypeScriptProjectDetector) determineModuleType(path string, metadata *TypeScriptDetectionMetadata) string {
	// Check package.json type field
	if metadata.PackageInfo != nil && metadata.PackageInfo.Type != "" {
		return metadata.PackageInfo.Type
	}

	// Check tsconfig.json module setting
	if metadata.TSConfigInfo != nil && metadata.TSConfigInfo.CompilerOptions != nil {
		if moduleType, ok := metadata.TSConfigInfo.CompilerOptions["module"].(string); ok {
			return strings.ToLower(moduleType)
		}
	}

	// Check for .mjs/.cjs files
	if d.hasFilesWithExtension(path, ".mjs") {
		return "module"
	}
	if d.hasFilesWithExtension(path, ".cjs") {
		return "commonjs"
	}

	return "commonjs" // Default
}

func (d *TypeScriptProjectDetector) hasFilesWithExtension(root string, extension string) bool {
	found := false
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || found {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), extension) {
			found = true
			return filepath.SkipDir
		}
		if info.IsDir() && (info.Name() == "node_modules" || info.Name() == types.DIR_DOT_GIT) {
			return filepath.SkipDir
		}
		return nil
	})
	return found
}

func (d *TypeScriptProjectDetector) detectYarn(path string) *PackageManagerInfo {
	info := &PackageManagerInfo{
		Name:       "yarn",
		Command:    "yarn",
		LockFile:   types.MARKER_YARN_LOCK,
		InstallCmd: "yarn install",
		RunCmd:     "yarn",
		AddCmd:     "yarn add",
	}

	if d.executor.IsCommandAvailable("yarn") {
		result, err := d.executor.Execute("yarn", []string{"--version"}, 5*time.Second)
		if err == nil && result.ExitCode == 0 {
			info.Available = true
			info.Version = strings.TrimSpace(result.Stdout)
			info.Recommended = true
		}
	}

	return info
}

func (d *TypeScriptProjectDetector) detectPnpm(path string) *PackageManagerInfo {
	info := &PackageManagerInfo{
		Name:       "pnpm",
		Command:    "pnpm",
		LockFile:   types.MARKER_PNPM_LOCK,
		InstallCmd: "pnpm install",
		RunCmd:     "pnpm",
		AddCmd:     "pnpm add",
	}

	if d.executor.IsCommandAvailable("pnpm") {
		result, err := d.executor.Execute("pnpm", []string{"--version"}, 5*time.Second)
		if err == nil && result.ExitCode == 0 {
			info.Available = true
			info.Version = strings.TrimSpace(result.Stdout)
			info.Recommended = true
		}
	}

	return info
}

func (d *TypeScriptProjectDetector) detectNpm(path string) *PackageManagerInfo {
	info := &PackageManagerInfo{
		Name:       "npm",
		Command:    "npm",
		LockFile:   "package-lock.json",
		InstallCmd: "npm install",
		RunCmd:     "npm run",
		AddCmd:     "npm install",
	}

	if d.executor.IsCommandAvailable("npm") {
		result, err := d.executor.Execute("npm", []string{"--version"}, 5*time.Second)
		if err == nil && result.ExitCode == 0 {
			info.Available = true
			info.Version = strings.TrimSpace(result.Stdout)
			info.Recommended = false // Lower priority than yarn/pnpm
		}
	}

	return info
}

func (d *TypeScriptProjectDetector) detectBun(path string) *PackageManagerInfo {
	info := &PackageManagerInfo{
		Name:       "bun",
		Command:    "bun",
		LockFile:   "bun.lockb",
		InstallCmd: "bun install",
		RunCmd:     "bun run",
		AddCmd:     "bun add",
	}

	if d.executor.IsCommandAvailable("bun") {
		result, err := d.executor.Execute("bun", []string{"--version"}, 5*time.Second)
		if err == nil && result.ExitCode == 0 {
			info.Available = true
			info.Version = strings.TrimSpace(result.Stdout)
			info.Recommended = true
		}
	}

	return info
}

func (d *TypeScriptProjectDetector) determineProjectType(result *types.LanguageDetectionResult, metadata *TypeScriptDetectionMetadata) {
	projectType := "application"

	if metadata.PackageInfo != nil {
		// Library indicators
		if metadata.PackageInfo.Main != "" || metadata.PackageInfo.Module != "" ||
			len(metadata.PackageInfo.Exports) > 0 || metadata.PackageInfo.Types != "" {
			projectType = "library"
		}

		// CLI tool indicators
		if len(metadata.PackageInfo.Bin) > 0 {
			projectType = "tool"
		}
	}

	// Monorepo indicators
	if metadata.MonorepoInfo != nil {
		projectType = "monorepo"
	}

	metadata.ProjectType = projectType
	result.Metadata["project_type"] = projectType
}

// Utility functions

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
