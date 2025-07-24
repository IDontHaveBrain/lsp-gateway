package project

import (
	"context"
	"fmt"
	"io/fs"
	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/project/detectors"
	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/setup"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)


// DetectionConfig holds configuration for project detection
type DetectionConfig struct {
	Timeout          time.Duration            `json:"timeout"`
	MaxDepth         int                      `json:"max_depth"`
	MaxFilesToScan   int                      `json:"max_files_to_scan"`
	IgnorePatterns   []string                 `json:"ignore_patterns"`
	CustomDetectors  map[string]LanguageDetector `json:"-"`
	EnableParallel   bool                     `json:"enable_parallel"`
	FollowSymlinks   bool                     `json:"follow_symlinks"`
}

// DefaultProjectDetector implements ProjectDetector interface
type DefaultProjectDetector struct {
	logger         *setup.SetupLogger
	config         *DetectionConfig
	detectors      map[string]LanguageDetector
	executor       platform.CommandExecutor
	sessionID      string
}

// DetectionReport contains comprehensive detection results
type DetectionReport struct {
	Projects      []*ProjectContext      `json:"projects"`
	WorkspaceRoot string                 `json:"workspace_root"`
	TotalProjects int                    `json:"total_projects"`
	DetectionTime time.Duration          `json:"detection_time"`
	Issues        []string               `json:"issues,omitempty"`
	Warnings      []string               `json:"warnings,omitempty"`
	Platform      platform.Platform      `json:"platform"`
	Architecture  platform.Architecture  `json:"architecture"`
	SessionID     string                 `json:"session_id"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// NewProjectDetector creates a new DefaultProjectDetector with default configuration
func NewProjectDetector() *DefaultProjectDetector {
	detector := &DefaultProjectDetector{
		logger: setup.NewSetupLogger(nil),
		config: &DetectionConfig{
			Timeout:        30 * time.Second,
			MaxDepth:       types.MAX_DIRECTORY_DEPTH,
			MaxFilesToScan: types.MAX_FILES_TO_SCAN,
			IgnorePatterns: []string{
				types.DIR_NODE_MODULES,
				types.DIR_VENDOR,
				types.DIR_BUILD,
				types.DIR_DIST,
				types.DIR_TARGET,
				".git",
				".svn",
				".hg",
				"__pycache__",
				"*.pyc",
				"*.class",
				"*.jar",
				".DS_Store",
				"Thumbs.db",
			},
			CustomDetectors: make(map[string]LanguageDetector),
			EnableParallel:  true,
			FollowSymlinks:  false,
		},
		detectors: make(map[string]LanguageDetector),
		executor:  platform.NewCommandExecutor(),
		sessionID: fmt.Sprintf("detect_%d", time.Now().Unix()),
	}
	
	// Initialize language-specific detectors
	detector.initializeDetectors()
	
	return detector
}

func (d *DefaultProjectDetector) initializeDetectors() {
	// Initialize comprehensive language detectors from detectors package
	d.detectors[types.PROJECT_TYPE_GO] = detectors.NewGoProjectDetector()
	d.detectors[types.PROJECT_TYPE_PYTHON] = detectors.NewPythonProjectDetector()
	d.detectors[types.PROJECT_TYPE_JAVA] = detectors.NewJavaProjectDetector()
	
	// TypeScript detector handles both TypeScript and Node.js projects
	tsDetector := detectors.NewTypeScriptProjectDetector()
	d.detectors[types.PROJECT_TYPE_TYPESCRIPT] = tsDetector
	d.detectors[types.PROJECT_TYPE_NODEJS] = tsDetector
}

func (d *DefaultProjectDetector) SetLogger(logger *setup.SetupLogger) {
	if logger != nil {
		d.logger = logger
	}
}

func (d *DefaultProjectDetector) SetTimeout(timeout time.Duration) {
	if timeout > 0 {
		d.config.Timeout = timeout
	}
}

func (d *DefaultProjectDetector) SetMaxDepth(depth int) {
	if depth > 0 {
		d.config.MaxDepth = depth
	}
}

func (d *DefaultProjectDetector) SetCustomDetectors(detectors map[string]LanguageDetector) {
	if detectors != nil {
		d.config.CustomDetectors = detectors
		// Merge with existing detectors
		for lang, detector := range detectors {
			d.detectors[lang] = detector
		}
	}
}

// DetectProject performs comprehensive project detection at the given path
func (d *DefaultProjectDetector) DetectProject(ctx context.Context, path string) (*ProjectContext, error) {
	startTime := time.Now()
	
	d.logger.WithOperation("project-detection").
		WithField(types.LOG_FIELD_PROJECT_ROOT, path).
		Info(types.PROJECT_DETECTION_STARTED)
	
	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	// Validate path
	absPath, err := d.validateAndNormalizePath(path)
	if err != nil {
		return nil, types.NewProjectError(types.ProjectErrorTypeDetection, types.PROJECT_TYPE_UNKNOWN, path, 
			fmt.Sprintf("Invalid path: %v", err), err)
	}

	// Get workspace root
	workspaceRoot, err := d.GetWorkspaceRoot(timeoutCtx, absPath)
	if err != nil {
		d.logger.WithError(err).Warn("Could not determine workspace root, using provided path")
		workspaceRoot = absPath
	}

	// Detect project types
	detectionResults, err := d.detectAllLanguages(timeoutCtx, workspaceRoot)
	if err != nil {
		return nil, types.NewProjectError(types.ProjectErrorTypeDetection, types.PROJECT_TYPE_UNKNOWN, workspaceRoot,
			fmt.Sprintf("Language detection failed: %v", err), err)
	}

	// Build project context
	projectContext, err := d.buildProjectContext(workspaceRoot, detectionResults)
	if err != nil {
		return nil, types.NewProjectError(types.ProjectErrorTypeDetection, types.PROJECT_TYPE_UNKNOWN, workspaceRoot,
			fmt.Sprintf("Failed to build project context: %v", err), err)
	}

	// Set detection metadata
	projectContext.DetectedAt = startTime
	projectContext.DetectionTime = time.Since(startTime)
	projectContext.DetectionMethod = "comprehensive_scan"
	projectContext.SetMetadata("session_id", d.sessionID)
	projectContext.SetMetadata("detector_version", "1.0.0")

	// Validate project structure
	if err := d.ValidateProject(timeoutCtx, projectContext); err != nil {
		d.logger.WithError(err).Warn("Project validation failed")
		projectContext.AddValidationError(err.Error())
	}

	d.logger.WithFields(map[string]interface{}{
		types.LOG_FIELD_PROJECT_TYPE:     projectContext.ProjectType,
		types.LOG_FIELD_PROJECT_ROOT:     projectContext.RootPath,
		types.LOG_FIELD_PROJECT_LANGUAGE: projectContext.PrimaryLanguage,
		types.LOG_FIELD_DETECTION_TIME:   projectContext.DetectionTime,
		"confidence":               projectContext.Confidence,
		"languages_detected":       len(projectContext.Languages),
		"servers_required":         len(projectContext.RequiredServers),
	}).Info(types.PROJECT_DETECTION_COMPLETED)

	return projectContext, nil
}

// DetectProjectType determines the primary project type for a given path
func (d *DefaultProjectDetector) DetectProjectType(ctx context.Context, path string) (string, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	absPath, err := d.validateAndNormalizePath(path)
	if err != nil {
		return types.PROJECT_TYPE_UNKNOWN, err
	}

	detectionResults, err := d.detectAllLanguages(timeoutCtx, absPath)
	if err != nil {
		return types.PROJECT_TYPE_UNKNOWN, err
	}

	if len(detectionResults) == 0 {
		return types.PROJECT_TYPE_UNKNOWN, types.NewProjectNotFoundError(absPath)
	}

	// Sort by priority and confidence
	sort.Slice(detectionResults, func(i, j int) bool {
		iPriority := d.getDetectorPriority(detectionResults[i].Language)
		jPriority := d.getDetectorPriority(detectionResults[j].Language)
		
		if iPriority != jPriority {
			return iPriority > jPriority
		}
		return detectionResults[i].Confidence > detectionResults[j].Confidence
	})

	primaryType := detectionResults[0].Language
	if len(detectionResults) > 1 {
		primaryType = types.PROJECT_TYPE_MIXED
	}

	return primaryType, nil
}

// GetWorkspaceRoot determines the workspace root for a given path
func (d *DefaultProjectDetector) GetWorkspaceRoot(ctx context.Context, path string) (string, error) {
	absPath, err := d.validateAndNormalizePath(path)
	if err != nil {
		return "", err
	}

	// Look for version control markers first
	vcsRoot, err := d.findVCSRoot(absPath)
	if err == nil {
		d.logger.WithField("vcs_root", vcsRoot).Debug("Found VCS root")
		return vcsRoot, nil
	}

	// Look for project markers
	projectRoot, err := d.findProjectRoot(absPath)
	if err == nil {
		d.logger.WithField("project_root", projectRoot).Debug("Found project root")
		return projectRoot, nil
	}

	// Fallback to the provided path
	d.logger.WithField("fallback_root", absPath).Debug(types.USING_DIRECTORY_AS_ROOT)
	return absPath, nil
}

// ValidateProject validates the structure and configuration of a detected project
func (d *DefaultProjectDetector) ValidateProject(ctx context.Context, projectCtx *ProjectContext) error {
	if projectCtx == nil {
		return types.NewValidationError(types.PROJECT_TYPE_UNKNOWN, "Project context is nil", nil)
	}

	d.logger.WithField(types.LOG_FIELD_PROJECT_TYPE, projectCtx.ProjectType).Debug("Starting project validation")

	var validationErrors []string
	var validationWarnings []string

	// Validate basic structure
	if projectCtx.ProjectType == types.PROJECT_TYPE_UNKNOWN {
		validationErrors = append(validationErrors, "Project type could not be determined")
	}

	if projectCtx.RootPath == "" {
		validationErrors = append(validationErrors, "Project root path is empty")
	} else if _, err := os.Stat(projectCtx.RootPath); os.IsNotExist(err) {
		validationErrors = append(validationErrors, fmt.Sprintf("Project root path does not exist: %s", projectCtx.RootPath))
	}

	// Validate marker files exist
	for _, markerFile := range projectCtx.MarkerFiles {
		fullPath := filepath.Join(projectCtx.RootPath, markerFile)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			validationWarnings = append(validationWarnings, fmt.Sprintf("Marker file not found: %s", markerFile))
		}
	}

	// Language-specific validation
	if detector, exists := d.detectors[projectCtx.ProjectType]; exists {
		if err := detector.ValidateStructure(ctx, projectCtx.RootPath); err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("Language-specific validation failed: %v", err))
		}
	}

	// Set validation results
	if len(validationErrors) > 0 {
		projectCtx.ValidationErrors = validationErrors
		projectCtx.IsValid = false
		
		validationError := types.NewValidationError(projectCtx.ProjectType, "Project validation failed", nil)
		for _, err := range validationErrors {
			validationError.WithSuggestion(err)
		}
		return validationError
	}

	if len(validationWarnings) > 0 {
		projectCtx.ValidationWarnings = validationWarnings
	}

	projectCtx.IsValid = true
	d.logger.WithField(types.LOG_FIELD_PROJECT_TYPE, projectCtx.ProjectType).Debug(types.PROJECT_VALIDATION_PASSED)
	return nil
}

// DetectMultipleProjects detects projects in multiple paths
func (d *DefaultProjectDetector) DetectMultipleProjects(ctx context.Context, paths []string) (map[string]*ProjectContext, error) {
	results := make(map[string]*ProjectContext)
	
	for _, path := range paths {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}
		
		projectCtx, err := d.DetectProject(ctx, path)
		if err != nil {
			d.logger.WithError(err).WithField("path", path).Warn("Failed to detect project")
			continue
		}
		
		results[path] = projectCtx
	}
	
	return results, nil
}

// ScanWorkspace scans a workspace directory for all projects
func (d *DefaultProjectDetector) ScanWorkspace(ctx context.Context, workspaceRoot string) ([]*ProjectContext, error) {
	startTime := time.Now()
	
	d.logger.WithField(types.LOG_FIELD_WORKSPACE_ROOT, workspaceRoot).Info("Starting workspace scan")
	
	absPath, err := d.validateAndNormalizePath(workspaceRoot)
	if err != nil {
		return nil, err
	}

	var projects []*ProjectContext
	var scanErrors []string

	err = filepath.WalkDir(absPath, func(path string, entry fs.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			scanErrors = append(scanErrors, fmt.Sprintf("Error accessing %s: %v", path, err))
			return nil
		}

		if !entry.IsDir() {
			return nil
		}

		// Check if we should ignore this directory
		if d.shouldIgnoreDirectory(path, entry.Name()) {
			return filepath.SkipDir
		}

		// Check depth limit
		relPath, _ := filepath.Rel(absPath, path)
		depth := strings.Count(relPath, string(filepath.Separator))
		if depth > d.config.MaxDepth {
			return filepath.SkipDir
		}

		// Try to detect project at this path
		projectCtx, err := d.DetectProject(ctx, path)
		if err != nil {
			// Not necessarily an error - just no project here
			return nil
		}

		projects = append(projects, projectCtx)
		
		// Skip subdirectories of detected projects to avoid duplicates
		return filepath.SkipDir
	})

	if err != nil && err != ctx.Err() {
		return projects, fmt.Errorf("workspace scan failed: %v", err)
	}

	duration := time.Since(startTime)
	d.logger.WithFields(map[string]interface{}{
		"projects_found": len(projects),
		"scan_time":     duration,
		"scan_errors":   len(scanErrors),
	}).Info("Workspace scan completed")

	return projects, nil
}

// Helper methods

func (d *DefaultProjectDetector) validateAndNormalizePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %v", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", fmt.Errorf("path does not exist: %s", absPath)
	}

	return absPath, nil
}

func (d *DefaultProjectDetector) detectAllLanguages(ctx context.Context, path string) ([]*types.LanguageDetectionResult, error) {
	var results []*types.LanguageDetectionResult

	for language, detector := range d.detectors {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		result, err := detector.DetectLanguage(ctx, path)
		if err != nil {
			d.logger.WithError(err).WithField("language", language).Debug("Language detection failed")
			continue
		}

		if result != nil && result.Confidence > 0 {
			results = append(results, result)
		}
	}

	return results, nil
}

func (d *DefaultProjectDetector) buildProjectContext(rootPath string, results []*types.LanguageDetectionResult) (*ProjectContext, error) {
	if len(results) == 0 {
		return nil, types.NewProjectNotFoundError(rootPath)
	}

	// Sort results by priority and confidence
	sort.Slice(results, func(i, j int) bool {
		iPriority := d.getDetectorPriority(results[i].Language)
		jPriority := d.getDetectorPriority(results[j].Language)
		
		if iPriority != jPriority {
			return iPriority > jPriority
		}
		return results[i].Confidence > results[j].Confidence
	})

	// Determine primary project type
	primaryType := results[0].Language
	if len(results) > 1 {
		primaryType = types.PROJECT_TYPE_MIXED
	}

	// Create project context
	ctx := NewProjectContext(primaryType, rootPath)
	ctx.PrimaryLanguage = results[0].Language

	// Aggregate information from all detected languages
	allServers := make(map[string]bool)
	allMarkerFiles := make(map[string]bool)
	totalConfidence := 0.0

	for _, result := range results {
		ctx.AddLanguage(result.Language)
		
		if result.Version != "" {
			ctx.SetLanguageVersion(result.Language, result.Version)
		}

		// Add required servers
		for _, server := range result.RequiredServers {
			allServers[server] = true
		}

		// Add marker files
		for _, marker := range result.MarkerFiles {
			allMarkerFiles[marker] = true
		}

		// Merge dependencies
		for dep, version := range result.Dependencies {
			ctx.Dependencies[dep] = version
		}

		for dep, version := range result.DevDependencies {
			ctx.DevDependencies[dep] = version
		}

		// Add source and test directories
		ctx.SourceDirs = append(ctx.SourceDirs, result.SourceDirs...)
		ctx.TestDirs = append(ctx.TestDirs, result.TestDirs...)
		ctx.ConfigFiles = append(ctx.ConfigFiles, result.ConfigFiles...)
		ctx.BuildFiles = append(ctx.BuildFiles, result.BuildFiles...)

		// Merge metadata
		for key, value := range result.Metadata {
			ctx.SetMetadata(fmt.Sprintf("%s_%s", result.Language, key), value)
		}

		totalConfidence += result.Confidence
	}

	// Set aggregated data
	for server := range allServers {
		ctx.AddRequiredServer(server)
	}

	for marker := range allMarkerFiles {
		ctx.AddMarkerFile(marker)
	}

	// Calculate overall confidence
	ctx.SetConfidence(totalConfidence / float64(len(results)))

	// Remove duplicates from slices
	ctx.SourceDirs = d.removeDuplicates(ctx.SourceDirs)
	ctx.TestDirs = d.removeDuplicates(ctx.TestDirs)
	ctx.ConfigFiles = d.removeDuplicates(ctx.ConfigFiles)
	ctx.BuildFiles = d.removeDuplicates(ctx.BuildFiles)

	// Calculate project size
	ctx.ProjectSize = d.calculateProjectSize(rootPath)

	return ctx, nil
}

func (d *DefaultProjectDetector) getDetectorPriority(language string) int {
	switch language {
	case types.PROJECT_TYPE_GO:
		return types.PRIORITY_GO
	case types.PROJECT_TYPE_PYTHON:
		return types.PRIORITY_PYTHON
	case types.PROJECT_TYPE_TYPESCRIPT:
		return types.PRIORITY_TYPESCRIPT
	case types.PROJECT_TYPE_NODEJS:
		return types.PRIORITY_NODEJS
	case types.PROJECT_TYPE_JAVA:
		return types.PRIORITY_JAVA
	case types.PROJECT_TYPE_MIXED:
		return types.PRIORITY_MIXED
	default:
		return types.PRIORITY_UNKNOWN
	}
}

func (d *DefaultProjectDetector) findVCSRoot(path string) (string, error) {
	current := path
	for {
		gitDir := filepath.Join(current, types.DIR_DOT_GIT)
		if _, err := os.Stat(gitDir); err == nil {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	
	return "", fmt.Errorf("no VCS root found")
}

func (d *DefaultProjectDetector) findProjectRoot(path string) (string, error) {
	current := path
	for {
		// Check for common project markers
		markers := []string{
			types.MARKER_GO_MOD, types.MARKER_PACKAGE_JSON, types.MARKER_SETUP_PY,
			types.MARKER_PYPROJECT, types.MARKER_POM_XML, types.MARKER_BUILD_GRADLE,
			types.MARKER_TSCONFIG, types.MARKER_CARGO_TOML,
		}

		for _, marker := range markers {
			markerPath := filepath.Join(current, marker)
			if _, err := os.Stat(markerPath); err == nil {
				return current, nil
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return "", fmt.Errorf("no project root found")
}

func (d *DefaultProjectDetector) shouldIgnoreDirectory(path, name string) bool {
	for _, pattern := range d.config.IgnorePatterns {
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
	}
	return false
}

func (d *DefaultProjectDetector) removeDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	var result []string
	
	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}
	
	return result
}

func (d *DefaultProjectDetector) calculateProjectSize(rootPath string) types.ProjectSize {
	size := types.ProjectSize{}
	
	err := filepath.WalkDir(rootPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if entry.IsDir() {
			if d.shouldIgnoreDirectory(path, entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		size.TotalFiles++
		
		if info, err := entry.Info(); err == nil {
			size.TotalSizeBytes += info.Size()
		}

		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".go", ".py", ".js", ".ts", ".java", ".rs", ".c", ".cpp", ".h":
			size.SourceFiles++
		case ".test.go", ".spec.js", ".test.js", ".spec.ts", ".test.ts":
			size.TestFiles++
		case ".json", ".yaml", ".yml", ".toml", ".ini", ".cfg", ".conf":
			size.ConfigFiles++
		}

		return nil
	})

	if err != nil {
		d.logger.WithError(err).Debug("Error calculating project size")
	}

	return size
}

// GetSupportedLanguages returns the list of programming languages that this
// detector can identify and analyze.
func (d *DefaultProjectDetector) GetSupportedLanguages() []string {
	return []string{
		types.PROJECT_TYPE_GO,
		types.PROJECT_TYPE_PYTHON,
		types.PROJECT_TYPE_JAVA,
		types.PROJECT_TYPE_TYPESCRIPT,
		types.PROJECT_TYPE_NODEJS,
	}
}

// Language detector factory functions
// These create instances of the comprehensive language detectors implemented in separate files

// Note: The actual implementations are in separate files:
// - go_detector.go for Go projects  
// - python_detector.go for Python projects
// - nodejs_detector.go for Node.js projects
// - java_detector.go for Java projects
// - typescript_detector.go for TypeScript projects