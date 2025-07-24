package detectors

import (
	"context"
	"fmt"
	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/setup"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GoProjectDetector implements LanguageDetector interface for Go project detection
type GoProjectDetector struct {
	logger         *setup.SetupLogger
	executor       platform.CommandExecutor
	versionChecker *setup.VersionChecker
	goDetector     *setup.GoDetector
	parser         *GoModuleParser
	workspaceParser *GoWorkspaceParser
	timeout        time.Duration
}

// GoDetectionMetadata contains Go-specific detection information
type GoDetectionMetadata struct {
	ModuleInfo      *GoModInfo                 `json:"module_info,omitempty"`
	WorkspaceInfo   *GoWorkspaceInfo           `json:"workspace_info,omitempty"`
	ImportAnalysis  *ImportAnalysis            `json:"import_analysis,omitempty"`
	BuildTags       []string                   `json:"build_tags,omitempty"`
	CGOEnabled      bool                       `json:"cgo_enabled"`
	GoVersion       string                     `json:"go_version,omitempty"`
	Dependencies    map[string]*types.GoDependencyInfo `json:"dependencies,omitempty"`
	TestPackages    []string                   `json:"test_packages,omitempty"`
	GeneratedFiles  []string                   `json:"generated_files,omitempty"`
}

// NewGoProjectDetector creates a new Go project detector
func NewGoProjectDetector() *GoProjectDetector {
	logger := setup.NewSetupLogger(&setup.SetupLoggerConfig{
		Component: "go-project-detector",
		Level:     setup.LogLevelInfo,
	})

	return &GoProjectDetector{
		logger:          logger,
		executor:        platform.NewCommandExecutor(),
		versionChecker:  setup.NewVersionChecker(),
		goDetector:      setup.NewGoDetector(),
		parser:          NewGoModuleParser(logger),
		workspaceParser: NewGoWorkspaceParser(logger),
		timeout:         45 * time.Second,
	}
}

// DetectLanguage implements LanguageDetector.DetectLanguage
func (d *GoProjectDetector) DetectLanguage(ctx context.Context, path string) (*types.LanguageDetectionResult, error) {
	startTime := time.Now()
	
	d.logger.WithField("path", path).Info("Starting Go project detection")

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	// Initialize result
	result := &types.LanguageDetectionResult{
		Language:        types.PROJECT_TYPE_GO,
		Confidence:      0.0,
		MarkerFiles:     []string{},
		ConfigFiles:     []string{},
		SourceDirs:      []string{},
		TestDirs:        []string{},
		Dependencies:    make(map[string]string),
		DevDependencies: make(map[string]string),
		BuildFiles:      []string{},
		RequiredServers: []string{types.SERVER_GOPLS},
		Metadata:        make(map[string]interface{}),
	}

	// Check for early exit due to context cancellation
	select {
	case <-timeoutCtx.Done():
		return nil, types.NewDetectionError(types.PROJECT_TYPE_GO, "detection", path, 
			"Go detection timed out", timeoutCtx.Err())
	default:
	}

	// Step 1: Detect Go runtime and validate installation
	confidence, err := d.detectGoRuntime(timeoutCtx, path, result)
	if err != nil {
		d.logger.WithError(err).Debug("Go runtime detection failed")
		// Don't return error - might still detect basic Go project structure
	}

	// Step 2: Analyze Go module structure
	moduleConfidence, err := d.analyzeGoModule(timeoutCtx, path, result)
	if err != nil {
		d.logger.WithError(err).Debug("Go module analysis failed")
	} else {
		confidence += moduleConfidence
	}

	// Step 3: Analyze workspace structure (go.work)
	workspaceConfidence, err := d.analyzeGoWorkspace(timeoutCtx, path, result)
	if err != nil {
		d.logger.WithError(err).Debug("Go workspace analysis failed") 
	} else {
		confidence += workspaceConfidence
	}

	// Step 4: Scan for Go source files and packages
	sourceConfidence, err := d.analyzeGoSources(timeoutCtx, path, result)
	if err != nil {
		d.logger.WithError(err).Debug("Go source analysis failed")
	} else {
		confidence += sourceConfidence
	}

	// Step 5: Analyze build configuration and tooling
	buildConfidence, err := d.analyzeBuildConfiguration(timeoutCtx, path, result)
	if err != nil {
		d.logger.WithError(err).Debug("Go build configuration analysis failed")
	} else {
		confidence += buildConfidence
	}

	// Normalize confidence to 0-1 range
	result.Confidence = normalizeConfidence(confidence)

	// Set detection metadata
	detectionTime := time.Since(startTime)
	result.Metadata["detection_time"] = detectionTime.String()
	result.Metadata["detection_method"] = "comprehensive_go_analysis"

	d.logger.WithFields(map[string]interface{}{
		"path":             path,
		"confidence":       result.Confidence,
		"detection_time":   detectionTime,
		"marker_files":     len(result.MarkerFiles),
		"dependencies":     len(result.Dependencies),
		"source_dirs":      len(result.SourceDirs),
	}).Info("Go project detection completed")

	return result, nil
}

// detectGoRuntime detects and validates Go runtime installation
func (d *GoProjectDetector) detectGoRuntime(ctx context.Context, path string, result *types.LanguageDetectionResult) (float64, error) {
	confidence := 0.0

	// Use existing GoDetector for runtime detection
	runtimeInfo, err := d.goDetector.DetectGoWithContext(ctx)
	if err != nil {
		return confidence, types.NewDetectionError(types.PROJECT_TYPE_GO, "runtime", path,
			fmt.Sprintf("Go runtime detection failed: %v", err), err)
	}

	if !runtimeInfo.Installed {
		result.Metadata["go_runtime_available"] = false
		result.Metadata["go_issues"] = runtimeInfo.Issues
		return confidence, types.NewDetectionError(types.PROJECT_TYPE_GO, "runtime", path,
			"Go runtime not installed or not accessible", nil)
	}

	// Runtime is available
	confidence += 0.1
	result.Metadata["go_runtime_available"] = true
	result.Version = runtimeInfo.Version
	result.Metadata["go_version"] = runtimeInfo.Version
	result.Metadata["go_path"] = runtimeInfo.Path
	result.Metadata["go_compatible"] = runtimeInfo.Compatible

	// Check version compatibility using internal/setup/version.go patterns
	if d.versionChecker.IsCompatible("go", runtimeInfo.Version) {
		confidence += 0.1
		result.Metadata["version_compatible"] = true
	} else {
		minVersion, _ := d.versionChecker.GetMinVersion("go")
		result.Metadata["version_compatible"] = false
		result.Metadata["required_version"] = minVersion
		d.logger.WithFields(map[string]interface{}{
			"installed_version": runtimeInfo.Version,
			"required_version":  minVersion,
		}).Warn("Go version below minimum requirement")
	}

	return confidence, nil
}

// analyzeGoModule analyzes go.mod files and module structure
func (d *GoProjectDetector) analyzeGoModule(ctx context.Context, path string, result *types.LanguageDetectionResult) (float64, error) {
	confidence := 0.0
	goModPath := filepath.Join(path, types.MARKER_GO_MOD)

	// Check if go.mod exists - primary indicator of Go project
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		d.logger.WithField("path", goModPath).Debug("go.mod not found")
		return confidence, nil
	}

	// go.mod found - high confidence indicator
	confidence += 0.6
	result.MarkerFiles = append(result.MarkerFiles, types.MARKER_GO_MOD)
	result.ConfigFiles = append(result.ConfigFiles, types.MARKER_GO_MOD)

	// Parse go.mod file using internal/project/detectors/go_parser.go
	modInfo, err := d.parser.ParseGoMod(ctx, goModPath)
	if err != nil {
		d.logger.WithError(err).WithField("path", goModPath).Warn("Failed to parse go.mod")
		return confidence, types.NewDetectionError(types.PROJECT_TYPE_GO, "parsing", goModPath,
			fmt.Sprintf("Failed to parse go.mod: %v", err), err)
	}

	// Successfully parsed go.mod
	confidence += 0.1
	result.Metadata["go_mod_info"] = modInfo

	// Extract module information
	if modInfo.ModuleName != "" {
		result.Metadata["module_name"] = modInfo.ModuleName
	}

	if modInfo.GoVersion != "" {
		result.Metadata["go_directive_version"] = modInfo.GoVersion
	}

	// Process dependencies
	for dep, version := range modInfo.Requires {
		if d.isTestDependency(dep) {
			result.DevDependencies[dep] = version
		} else {
			result.Dependencies[dep] = version
		}
	}

	// Add replace directives to metadata
	if len(modInfo.Replaces) > 0 {
		result.Metadata["go_replaces"] = modInfo.Replaces
	}

	// Add exclude directives to metadata
	if len(modInfo.Excludes) > 0 {
		result.Metadata["go_excludes"] = modInfo.Excludes
	}

	// Check for go.sum file - indicates dependency management
	goSumPath := filepath.Join(path, types.MARKER_GO_SUM)
	if _, err := os.Stat(goSumPath); err == nil {
		confidence += 0.05
		result.MarkerFiles = append(result.MarkerFiles, types.MARKER_GO_SUM)
		result.Metadata["go_sum_present"] = true
	}

	return confidence, nil
}

// analyzeGoWorkspace analyzes go.work files for multi-module workspaces
func (d *GoProjectDetector) analyzeGoWorkspace(ctx context.Context, path string, result *types.LanguageDetectionResult) (float64, error) {
	confidence := 0.0
	goWorkPath := filepath.Join(path, "go.work")

	// Check if go.work exists
	if _, err := os.Stat(goWorkPath); os.IsNotExist(err) {
		return confidence, nil // Not an error - just no workspace
	}

	// go.work found - indicates workspace mode
	confidence += 0.2
	result.MarkerFiles = append(result.MarkerFiles, "go.work")
	result.ConfigFiles = append(result.ConfigFiles, "go.work")

	// Parse go.work file
	workspaceInfo, err := d.workspaceParser.ParseGoWork(ctx, goWorkPath)
	if err != nil {
		d.logger.WithError(err).WithField("path", goWorkPath).Warn("Failed to parse go.work")
		return confidence, types.NewDetectionError(types.PROJECT_TYPE_GO, "parsing", goWorkPath,
			fmt.Sprintf("Failed to parse go.work: %v", err), err)
	}

	// Successfully parsed go.work
	confidence += 0.05
	result.Metadata["go_workspace_info"] = workspaceInfo
	result.Metadata["is_workspace"] = true
	result.Metadata["workspace_modules"] = workspaceInfo.Modules

	// Check for go.work.sum
	goWorkSumPath := filepath.Join(path, "go.work.sum")
	if _, err := os.Stat(goWorkSumPath); err == nil {
		result.MarkerFiles = append(result.MarkerFiles, "go.work.sum")
		result.Metadata["go_work_sum_present"] = true
	}

	return confidence, nil
}

// analyzeGoSources scans for Go source files and analyzes package structure
func (d *GoProjectDetector) analyzeGoSources(ctx context.Context, path string, result *types.LanguageDetectionResult) (float64, error) {
	confidence := 0.0
	sourceAnalysis := &SourceAnalysis{
		Packages:     make(map[string]*PackageInfo),
		TestFiles:    []string{},
		BuildTags:    make(map[string]bool),
		CGOUsage:     false,
		ImportPaths:  make(map[string]bool),
	}

	// Walk directory tree looking for .go files
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			return nil // Skip files we can't access
		}

		if !strings.HasSuffix(filePath, ".go") {
			return nil
		}

		// Skip vendor directories and other common excludes
		if d.shouldSkipPath(filePath, path) {
			return nil
		}

		// Analyze Go source file
		if err := d.analyzeGoSourceFile(filePath, sourceAnalysis); err != nil {
			d.logger.WithError(err).WithField("file", filePath).Debug("Failed to analyze Go source file")
			return nil // Continue processing other files
		}

		return nil
	})

	if err != nil {
		return confidence, types.NewDetectionError(types.PROJECT_TYPE_GO, "scanning", path,
			fmt.Sprintf("Failed to scan Go sources: %v", err), err)
	}

	// Calculate confidence based on findings
	if len(sourceAnalysis.Packages) > 0 {
		confidence += 0.3 // Found Go packages
		
		// Categorize directories
		for pkgPath, pkgInfo := range sourceAnalysis.Packages {
			relPath, _ := filepath.Rel(path, pkgPath)
			
			if pkgInfo.IsTest {
				result.TestDirs = append(result.TestDirs, relPath)
			} else {
				result.SourceDirs = append(result.SourceDirs, relPath)
			}
		}
	}

	// Add metadata about source analysis
	result.Metadata["go_packages_found"] = len(sourceAnalysis.Packages)
	result.Metadata["go_source_analysis"] = sourceAnalysis
	
	if sourceAnalysis.CGOUsage {
		result.Metadata["cgo_enabled"] = true
		confidence += 0.05
	}

	if len(sourceAnalysis.BuildTags) > 0 {
		tags := make([]string, 0, len(sourceAnalysis.BuildTags))
		for tag := range sourceAnalysis.BuildTags {
			tags = append(tags, tag)
		}
		result.Metadata["build_tags"] = tags
	}

	return confidence, nil
}

// analyzeBuildConfiguration analyzes build tools and configuration
func (d *GoProjectDetector) analyzeBuildConfiguration(ctx context.Context, path string, result *types.LanguageDetectionResult) (float64, error) {
	confidence := 0.0

	// Check for common Go build files
	buildFiles := []string{
		"Makefile",
		"build.sh",
		"build.go",
		".github/workflows",
		".gitlab-ci.yml",
		"Dockerfile",
		"docker-compose.yml",
	}

	for _, buildFile := range buildFiles {
		buildFilePath := filepath.Join(path, buildFile)
		if _, err := os.Stat(buildFilePath); err == nil {
			result.BuildFiles = append(result.BuildFiles, buildFile)
			confidence += 0.02
		}
	}

	// Check for Go-specific tooling configuration
	toolConfigs := []string{
		".golangci.yml",
		".golangci.yaml", 
		"golangci.yml",
		"golangci.yaml",
		".goreleaser.yml",
		".goreleaser.yaml",
	}

	foundTools := []string{}
	for _, toolConfig := range toolConfigs {
		toolPath := filepath.Join(path, toolConfig)
		if _, err := os.Stat(toolPath); err == nil {
			foundTools = append(foundTools, toolConfig)
			result.ConfigFiles = append(result.ConfigFiles, toolConfig)
			confidence += 0.02
		}
	}

	if len(foundTools) > 0 {
		result.Metadata["go_tooling_configs"] = foundTools
	}

	return confidence, nil
}

// GetMarkerFiles implements LanguageDetector.GetMarkerFiles
func (d *GoProjectDetector) GetMarkerFiles() []string {
	return []string{
		types.MARKER_GO_MOD,
		types.MARKER_GO_SUM,
		"go.work",
		"go.work.sum",
	}
}

// GetRequiredServers implements LanguageDetector.GetRequiredServers
func (d *GoProjectDetector) GetRequiredServers() []string {
	return []string{types.SERVER_GOPLS}
}

// GetPriority implements LanguageDetector.GetPriority  
func (d *GoProjectDetector) GetPriority() int {
	return types.PRIORITY_GO
}

// ValidateStructure implements LanguageDetector.ValidateStructure
func (d *GoProjectDetector) ValidateStructure(ctx context.Context, path string) error {
	d.logger.WithField("path", path).Debug("Validating Go project structure")

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	var validationErrors []string

	// Check for go.mod file - required for modern Go projects
	goModPath := filepath.Join(path, types.MARKER_GO_MOD)
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		validationErrors = append(validationErrors, "go.mod file not found - not a Go module")
	} else {
		// Validate go.mod syntax
		if _, err := d.parser.ParseGoMod(timeoutCtx, goModPath); err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("go.mod syntax error: %v", err))
		}
	}

	// Check for Go source files
	hasGoFiles, err := d.hasGoSourceFiles(path)
	if err != nil {
		validationErrors = append(validationErrors, fmt.Sprintf("Error scanning for Go files: %v", err))
	} else if !hasGoFiles {
		validationErrors = append(validationErrors, "No Go source files (.go) found")
	}

	// Validate workspace if go.work exists
	goWorkPath := filepath.Join(path, "go.work")
	if _, err := os.Stat(goWorkPath); err == nil {
		if _, err := d.workspaceParser.ParseGoWork(timeoutCtx, goWorkPath); err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("go.work syntax error: %v", err))
		}
	}

	// Check Go version compatibility if runtime is available
	if runtimeInfo, err := d.goDetector.DetectGoWithContext(timeoutCtx); err == nil && runtimeInfo.Installed {
		if !d.versionChecker.IsCompatible("go", runtimeInfo.Version) {
			minVersion, _ := d.versionChecker.GetMinVersion("go")
			validationErrors = append(validationErrors, 
				fmt.Sprintf("Go version %s is below minimum requirement %s", runtimeInfo.Version, minVersion))
		}
	}

	if len(validationErrors) > 0 {
		return types.NewValidationError(types.PROJECT_TYPE_GO, "Go project validation failed", nil)
	}

	d.logger.WithField("path", path).Debug("Go project structure validation passed")
	return nil
}

// GetLanguageInfo returns comprehensive information about Go language support
func (d *GoProjectDetector) GetLanguageInfo(language string) (*types.LanguageInfo, error) {
	if language != types.PROJECT_TYPE_GO {
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	return &types.LanguageInfo{
		Name:           types.PROJECT_TYPE_GO,
		DisplayName:    "Go",
		MinVersion:     "1.19",
		MaxVersion:     "1.22",
		BuildTools:     []string{"go build", "go install", "make", "goreleaser", "task"},
		PackageManager: "go mod",
		TestFrameworks: []string{"testing", "testify", "ginkgo", "gomega"},
		LintTools:      []string{"golangci-lint", "gofmt", "goimports", "go vet", "staticcheck"},
		FormatTools:    []string{"gofmt", "goimports", "gofumpt"},
		LSPServers:     []string{types.SERVER_GOPLS},
		FileExtensions: []string{".go", ".mod", ".sum"},
		Capabilities:   []string{"completion", "hover", "definition", "references", "formatting", "code_action", "rename"},
		Metadata: map[string]interface{}{
			"module_system":     "go modules",
			"package_hosting":   "pkg.go.dev",
			"documentation":     "godoc",
			"workspace_support": true,
		},
	}, nil
}

// Helper methods

func (d *GoProjectDetector) shouldSkipPath(filePath, rootPath string) bool {
	relativePath, _ := filepath.Rel(rootPath, filePath)
	pathComponents := strings.Split(relativePath, string(filepath.Separator))
	
	skipDirs := map[string]bool{
		"vendor":       true,
		"node_modules": true,
		".git":         true,
		"__pycache__":  true,
		"build":        true,
		"dist":         true,
		"target":       true,
	}

	for _, component := range pathComponents {
		if skipDirs[component] {
			return true
		}
	}

	return false
}

func (d *GoProjectDetector) hasGoSourceFiles(path string) (bool, error) {
	found := false
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		
		if strings.HasSuffix(filePath, ".go") && !d.shouldSkipPath(filePath, path) {
			found = true
			return filepath.SkipDir // Found at least one, can stop
		}
		
		return nil
	})
	
	return found, err
}

func (d *GoProjectDetector) isTestDependency(dep string) bool {
	testPrefixes := []string{
		"github.com/stretchr/testify",
		"github.com/golang/mock",
		"github.com/onsi/ginkgo",
		"github.com/onsi/gomega",
		"gotest.tools",
	}
	
	for _, prefix := range testPrefixes {
		if strings.HasPrefix(dep, prefix) {
			return true
		}
	}
	
	return false
}

func (d *GoProjectDetector) analyzeGoSourceFile(filePath string, analysis *SourceAnalysis) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	dir := filepath.Dir(filePath)
	
	if analysis.Packages[dir] == nil {
		analysis.Packages[dir] = &PackageInfo{
			Path:      dir,
			Files:     []string{},
			IsTest:    false,
			HasCGO:    false,
			BuildTags: []string{},
		}
	}

	pkgInfo := analysis.Packages[dir]
	pkgInfo.Files = append(pkgInfo.Files, filePath)

	// Check if this is a test file
	if strings.HasSuffix(filePath, "_test.go") {
		pkgInfo.IsTest = true
		analysis.TestFiles = append(analysis.TestFiles, filePath)
	}

	// Analyze file content
	for i, line := range lines {
		line = strings.TrimSpace(line)
		
		// Check for build tags (must be in first few lines and start with // +build or //go:build)
		if i < 10 && (strings.HasPrefix(line, "// +build") || strings.HasPrefix(line, "//go:build")) {
			buildTag := strings.TrimPrefix(strings.TrimPrefix(line, "// +build"), "//go:build")
			buildTag = strings.TrimSpace(buildTag)
			if buildTag != "" {
				analysis.BuildTags[buildTag] = true
				pkgInfo.BuildTags = append(pkgInfo.BuildTags, buildTag)
			}
		}
		
		// Check for CGO usage
		if strings.Contains(line, "import \"C\"") || strings.Contains(line, "/*") {
			analysis.CGOUsage = true
			pkgInfo.HasCGO = true
		}
		
		// Extract import paths
		if strings.HasPrefix(line, "import") && strings.Contains(line, "\"") {
			// Simple import extraction - could be enhanced for better parsing
			start := strings.Index(line, "\"")
			end := strings.LastIndex(line, "\"")
			if start != -1 && end != -1 && start != end {
				importPath := line[start+1 : end]
				analysis.ImportPaths[importPath] = true
			}
		}
	}

	return nil
}

func normalizeConfidence(confidence float64) float64 {
	if confidence > 1.0 {
		return 1.0
	}
	if confidence < 0.0 {
		return 0.0
	}
	return confidence
}

// Data structures for analysis

type SourceAnalysis struct {
	Packages    map[string]*PackageInfo `json:"packages"`
	TestFiles   []string                `json:"test_files"`
	BuildTags   map[string]bool         `json:"build_tags"`
	CGOUsage    bool                    `json:"cgo_usage"`
	ImportPaths map[string]bool         `json:"import_paths"`
}

type PackageInfo struct {
	Path      string   `json:"path"`
	Files     []string `json:"files"`
	IsTest    bool     `json:"is_test"`
	HasCGO    bool     `json:"has_cgo"`
	BuildTags []string `json:"build_tags"`
}