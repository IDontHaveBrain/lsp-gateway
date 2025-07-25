package project

import (
	"context"
	"fmt"
	"lsp-gateway/internal/setup"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// VCSType represents the type of version control system
type VCSType string

const (
	VCSTypeGit       VCSType = "git"
	VCSTypeSVN       VCSType = "svn"
	VCSTypeMercurial VCSType = "hg"
	VCSTypeBazaar    VCSType = "bzr"
	VCSTypeNone      VCSType = "none"
)

// VCSInfo contains information about detected version control system
type VCSInfo struct {
	Type     VCSType `json:"type"`
	RootPath string  `json:"root_path"`
	Remote   string  `json:"remote,omitempty"`
	Branch   string  `json:"branch,omitempty"`
	Revision string  `json:"revision,omitempty"`
}

// WorkspaceInfo contains comprehensive workspace information
type WorkspaceInfo struct {
	RootPath      string                 `json:"root_path"`
	ProjectType   string                 `json:"project_type"`
	VCS           *VCSInfo               `json:"vcs,omitempty"`
	RootMarkers   []string               `json:"root_markers"`
	Boundaries    []string               `json:"boundaries,omitempty"`
	IsMonorepo    bool                   `json:"is_monorepo"`
	SubProjects   []string               `json:"sub_projects,omitempty"`
	DetectionTime time.Duration          `json:"detection_time"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	DetectedAt    time.Time              `json:"detected_at"`
}

// WorkspaceDetectorOptions configures workspace detection behavior
type WorkspaceDetectorOptions struct {
	MaxDepth           int
	TimeoutDuration    time.Duration
	SearchParents      bool
	EnableVCSDetection bool
	EnableCaching      bool
	CacheDuration      time.Duration
	ExcludePatterns    []string
	RequireVCS         bool
	MonorepoDetection  bool
	MaxSearchUp        int
	Logger             *setup.SetupLogger
}

// DefaultWorkspaceDetectorOptions returns sensible defaults
func DefaultWorkspaceDetectorOptions() *WorkspaceDetectorOptions {
	return &WorkspaceDetectorOptions{
		MaxDepth:           MAX_DIRECTORY_DEPTH,
		TimeoutDuration:    30 * time.Second,
		SearchParents:      true,
		EnableVCSDetection: true,
		EnableCaching:      true,
		CacheDuration:      10 * time.Minute,
		ExcludePatterns:    []string{".git", "node_modules", "vendor", ".venv", "__pycache__", "target", "build"},
		RequireVCS:         false,
		MonorepoDetection:  true,
		MaxSearchUp:        20, // Maximum levels to search up for workspace root
		Logger:             nil,
	}
}

// WorkspaceDetector finds project workspace roots and detects VCS information
type WorkspaceDetector struct {
	options     *WorkspaceDetectorOptions
	logger      *setup.SetupLogger
	mu          sync.RWMutex
	cache       map[string]*workspaceDetectionResult
	rootMarkers []string
	vcsMarkers  map[VCSType][]string
}

// workspaceDetectionResult caches detection results
type workspaceDetectionResult struct {
	workspace *WorkspaceInfo
	error     error
	timestamp time.Time
}

// NewWorkspaceDetector creates a new workspace detector
func NewWorkspaceDetector(options *WorkspaceDetectorOptions) *WorkspaceDetector {
	if options == nil {
		options = DefaultWorkspaceDetectorOptions()
	}

	logger := options.Logger
	if logger == nil {
		logger = setup.NewSetupLogger(&setup.SetupLoggerConfig{
			Component: "workspace-detector",
			Level:     setup.LogLevelInfo,
		})
	}

	detector := &WorkspaceDetector{
		options: options,
		logger:  logger.WithField("component", "workspace_detector"),
		cache:   make(map[string]*workspaceDetectionResult),
		rootMarkers: []string{
			// Version Control
			DIR_DOT_GIT, ".svn", ".hg", ".bzr",
			// Go
			MARKER_GO_MOD, MARKER_GO_SUM,
			// JavaScript/TypeScript
			MARKER_PACKAGE_JSON, MARKER_YARN_LOCK, MARKER_PNPM_LOCK,
			MARKER_TSCONFIG,
			// Python
			MARKER_SETUP_PY, MARKER_PYPROJECT, MARKER_REQUIREMENTS,
			MARKER_PIPFILE, "poetry.lock", "conda.yaml", "environment.yml",
			// Java
			MARKER_POM_XML, MARKER_BUILD_GRADLE, "gradle.properties",
			"build.gradle.kts", "settings.gradle",
			// Rust
			MARKER_CARGO_TOML, "Cargo.lock",
			// Other
			"Makefile", "CMakeLists.txt", "meson.build",
			"composer.json", "mix.exs", "dune-project",
		},
		vcsMarkers: map[VCSType][]string{
			VCSTypeGit:       {DIR_DOT_GIT},
			VCSTypeSVN:       {".svn"},
			VCSTypeMercurial: {".hg"},
			VCSTypeBazaar:    {".bzr"},
		},
	}

	return detector
}

// FindWorkspaceRoot finds the workspace root for a given path
func (d *WorkspaceDetector) FindWorkspaceRoot(ctx context.Context, startPath string) (*WorkspaceInfo, error) {
	if startPath == "" {
		return nil, NewProjectError(
			ProjectErrorTypeWorkspace,
			PROJECT_TYPE_UNKNOWN,
			startPath,
			"start path cannot be empty",
			nil,
		).WithSuggestions([]string{
			"Provide a valid starting path",
			"Use current directory: .",
			"Use absolute path for clarity",
		})
	}

	// Check cache first
	if d.options.EnableCaching {
		if result := d.getCachedResult(startPath); result != nil {
			if result.workspace != nil {
				return result.workspace, nil
			}
			return nil, result.error
		}
	}

	startTime := time.Now()
	d.logger.WithField("start_path", startPath).Info("Starting workspace root detection")

	// Normalize start path
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return nil, NewProjectError(
			ProjectErrorTypeWorkspace,
			PROJECT_TYPE_UNKNOWN,
			startPath,
			fmt.Sprintf("cannot resolve absolute path: %v", err),
			err,
		)
	}

	// Ensure it's a directory
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, NewProjectError(
			ProjectErrorTypeWorkspace,
			PROJECT_TYPE_UNKNOWN,
			absPath,
			fmt.Sprintf("cannot access start path: %v", err),
			err,
		)
	}

	if !info.IsDir() {
		absPath = filepath.Dir(absPath)
	}

	// Detect workspace with timeout
	ctx, cancel := context.WithTimeout(ctx, d.options.TimeoutDuration)
	defer cancel()

	resultChan := make(chan *workspaceDetectionResult, 1)
	go func() {
		workspace, err := d.detectWorkspace(absPath)
		resultChan <- &workspaceDetectionResult{
			workspace: workspace,
			error:     err,
			timestamp: time.Now(),
		}
	}()

	select {
	case result := <-resultChan:
		if d.options.EnableCaching {
			d.cacheResult(startPath, result)
		}

		duration := time.Since(startTime)

		if result.workspace != nil {
			result.workspace.DetectionTime = duration
			result.workspace.DetectedAt = startTime

			d.logger.WithFields(map[string]interface{}{
				"start_path":     startPath,
				"workspace_root": result.workspace.RootPath,
				"project_type":   result.workspace.ProjectType,
				"duration":       duration.String(),
			}).Info("Workspace detection completed successfully")

			return result.workspace, nil
		}

		d.logger.WithError(result.error).WithFields(map[string]interface{}{
			"start_path": startPath,
			"duration":   duration.String(),
		}).Warn("Workspace detection failed")

		return nil, result.error

	case <-ctx.Done():
		err := NewDetectionTimeoutError(startPath, d.options.TimeoutDuration)
		d.logger.WithError(err).WithField("start_path", startPath).Error("Workspace detection timeout")
		return nil, err
	}
}

// detectWorkspace performs the actual workspace detection
func (d *WorkspaceDetector) detectWorkspace(startPath string) (*WorkspaceInfo, error) {
	workspace := &WorkspaceInfo{
		Metadata: make(map[string]interface{}),
	}

	// Search for workspace root
	rootPath, rootMarkers, err := d.findRootPath(startPath)
	if err != nil {
		return nil, err
	}

	workspace.RootPath = rootPath
	workspace.RootMarkers = rootMarkers

	// Detect project type
	projectType := d.detectProjectType(rootPath, rootMarkers)
	workspace.ProjectType = projectType

	// Detect VCS if enabled
	if d.options.EnableVCSDetection {
		vcsInfo := d.DetectVCS(rootPath)
		workspace.VCS = vcsInfo
	}

	// Detect monorepo structure if enabled
	if d.options.MonorepoDetection {
		isMonorepo, subProjects := d.detectMonorepo(rootPath)
		workspace.IsMonorepo = isMonorepo
		workspace.SubProjects = subProjects
	}

	// Detect workspace boundaries
	boundaries := d.detectWorkspaceBoundaries(rootPath)
	workspace.Boundaries = boundaries

	vcsType := "none"
	if workspace.VCS != nil {
		vcsType = string(workspace.VCS.Type)
	}

	d.logger.WithFields(map[string]interface{}{
		"root_path":    rootPath,
		"project_type": projectType,
		"root_markers": rootMarkers,
		"is_monorepo":  workspace.IsMonorepo,
		"sub_projects": len(workspace.SubProjects),
		"vcs_type":     vcsType,
	}).Debug("Workspace detection details")

	return workspace, nil
}

// findRootPath searches for the workspace root starting from the given path
func (d *WorkspaceDetector) findRootPath(startPath string) (string, []string, error) {
	currentPath := startPath
	levelsUp := 0

	for levelsUp <= d.options.MaxSearchUp {
		// Check for root markers in current directory
		foundMarkers := d.checkRootMarkers(currentPath)
		if len(foundMarkers) > 0 {
			d.logger.WithFields(map[string]interface{}{
				"path":      currentPath,
				"markers":   foundMarkers,
				"levels_up": levelsUp,
			}).Debug("Found workspace root markers")

			return currentPath, foundMarkers, nil
		}

		// Stop at filesystem root
		parent := filepath.Dir(currentPath)
		if parent == currentPath {
			break
		}

		// Stop if we don't want to search parents
		if !d.options.SearchParents && levelsUp > 0 {
			break
		}

		currentPath = parent
		levelsUp++
	}

	// If no markers found and VCS is not required, use start path
	if !d.options.RequireVCS {
		d.logger.WithField("path", startPath).Debug("No workspace root markers found, using start path")
		return startPath, []string{}, nil
	}

	return "", nil, NewWorkspaceRootNotFoundError(startPath)
}

// checkRootMarkers checks for workspace root markers in a directory
func (d *WorkspaceDetector) checkRootMarkers(path string) []string {
	var foundMarkers []string

	for _, marker := range d.rootMarkers {
		markerPath := filepath.Join(path, marker)
		if _, err := os.Stat(markerPath); err == nil {
			foundMarkers = append(foundMarkers, marker)
		}
	}

	return foundMarkers
}

// detectProjectType determines the project type based on root markers
func (d *WorkspaceDetector) detectProjectType(rootPath string, markers []string) string {
	// Priority-based detection
	typeScores := make(map[string]int)

	for _, marker := range markers {
		switch marker {
		case MARKER_GO_MOD, MARKER_GO_SUM:
			typeScores[PROJECT_TYPE_GO] += PRIORITY_GO
		case MARKER_PACKAGE_JSON:
			// Check if it's TypeScript or pure Node.js
			packagePath := filepath.Join(rootPath, MARKER_PACKAGE_JSON)
			if d.hasTypeScriptDependencies(packagePath) {
				typeScores[PROJECT_TYPE_TYPESCRIPT] += PRIORITY_TYPESCRIPT
			} else {
				typeScores[PROJECT_TYPE_NODEJS] += PRIORITY_NODEJS
			}
		case MARKER_TSCONFIG:
			typeScores[PROJECT_TYPE_TYPESCRIPT] += PRIORITY_TYPESCRIPT
		case MARKER_PYPROJECT, MARKER_SETUP_PY, MARKER_REQUIREMENTS, MARKER_PIPFILE:
			typeScores[PROJECT_TYPE_PYTHON] += PRIORITY_PYTHON
		case MARKER_POM_XML, MARKER_BUILD_GRADLE:
			typeScores[PROJECT_TYPE_JAVA] += PRIORITY_JAVA
		case MARKER_CARGO_TOML:
			typeScores["rust"] += PRIORITY_JAVA // Using same priority as Java
		}
	}

	// Find highest scoring type
	highestScore := 0
	detectedType := PROJECT_TYPE_UNKNOWN
	multipleTypes := false

	for projectType, score := range typeScores {
		if score > highestScore {
			highestScore = score
			detectedType = projectType
			multipleTypes = false
		} else if score == highestScore && score > 0 {
			multipleTypes = true
		}
	}

	if multipleTypes {
		return PROJECT_TYPE_MIXED
	}

	return detectedType
}

// hasTypeScriptDependencies checks if a package.json contains TypeScript dependencies
func (d *WorkspaceDetector) hasTypeScriptDependencies(packageJsonPath string) bool {
	// This would typically parse the package.json and check for TypeScript dependencies
	// For now, we'll check for common TypeScript files in the directory
	dir := filepath.Dir(packageJsonPath)

	// Check for tsconfig.json
	if _, err := os.Stat(filepath.Join(dir, MARKER_TSCONFIG)); err == nil {
		return true
	}

	// Check for .ts files in common directories
	checkDirs := []string{"src", "lib", "types", "."}
	for _, checkDir := range checkDirs {
		dirPath := filepath.Join(dir, checkDir)
		if entries, err := os.ReadDir(dirPath); err == nil {
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".ts") || strings.HasSuffix(entry.Name(), ".tsx") {
					return true
				}
			}
		}
	}

	return false
}

// DetectVCS detects version control system information
func (d *WorkspaceDetector) DetectVCS(rootPath string) *VCSInfo {
	for vcsType, markers := range d.vcsMarkers {
		for _, marker := range markers {
			markerPath := filepath.Join(rootPath, marker)
			if info, err := os.Stat(markerPath); err == nil && info.IsDir() {
				vcsInfo := &VCSInfo{
					Type:     vcsType,
					RootPath: rootPath,
				}

				// Get additional VCS information
				d.enrichVCSInfo(vcsInfo, markerPath)
				return vcsInfo
			}
		}
	}

	return &VCSInfo{
		Type:     VCSTypeNone,
		RootPath: rootPath,
	}
}

// enrichVCSInfo adds additional information to VCS info
func (d *WorkspaceDetector) enrichVCSInfo(vcsInfo *VCSInfo, vcsPath string) {
	switch vcsInfo.Type {
	case VCSTypeGit:
		d.enrichGitInfo(vcsInfo, vcsPath)
	case VCSTypeSVN:
		d.enrichSVNInfo(vcsInfo, vcsPath)
	case VCSTypeMercurial:
		d.enrichHgInfo(vcsInfo, vcsPath)
	}
}

// enrichGitInfo enriches Git VCS information
func (d *WorkspaceDetector) enrichGitInfo(vcsInfo *VCSInfo, gitPath string) {
	// Try to read basic Git information
	// This is a simplified implementation - in production you might want to use git commands or libraries

	// Read HEAD to get current branch/revision
	headPath := filepath.Join(gitPath, "HEAD")
	if headContent, err := os.ReadFile(headPath); err == nil {
		headStr := strings.TrimSpace(string(headContent))
		if strings.HasPrefix(headStr, "ref: refs/heads/") {
			vcsInfo.Branch = strings.TrimPrefix(headStr, "ref: refs/heads/")
		} else {
			vcsInfo.Revision = headStr
		}
	}

	// Try to read remote information from config
	configPath := filepath.Join(gitPath, "config")
	if configContent, err := os.ReadFile(configPath); err == nil {
		// Simple parsing for remote origin URL
		lines := strings.Split(string(configContent), "\n")
		inRemoteOrigin := false
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == `[remote "origin"]` {
				inRemoteOrigin = true
				continue
			}
			if inRemoteOrigin && strings.HasPrefix(line, "url = ") {
				vcsInfo.Remote = strings.TrimPrefix(line, "url = ")
				break
			}
			if strings.HasPrefix(line, "[") && line != `[remote "origin"]` {
				inRemoteOrigin = false
			}
		}
	}
}

// enrichSVNInfo enriches SVN VCS information
func (d *WorkspaceDetector) enrichSVNInfo(vcsInfo *VCSInfo, svnPath string) {
	// Read SVN entries file for basic information
	entriesPath := filepath.Join(svnPath, "entries")
	if entriesContent, err := os.ReadFile(entriesPath); err == nil {
		lines := strings.Split(string(entriesContent), "\n")
		if len(lines) > 4 {
			vcsInfo.Revision = strings.TrimSpace(lines[3])
		}
		if len(lines) > 5 {
			vcsInfo.Remote = strings.TrimSpace(lines[4])
		}
	}
}

// enrichHgInfo enriches Mercurial VCS information
func (d *WorkspaceDetector) enrichHgInfo(vcsInfo *VCSInfo, hgPath string) {
	// Read .hg/branch for current branch
	branchPath := filepath.Join(hgPath, "branch")
	if branchContent, err := os.ReadFile(branchPath); err == nil {
		vcsInfo.Branch = strings.TrimSpace(string(branchContent))
	}

	// Read .hg/dirstate for current revision
	dirstatePath := filepath.Join(hgPath, "dirstate")
	if dirstate, err := os.ReadFile(dirstatePath); err == nil && len(dirstate) >= 20 {
		// First 20 bytes contain the parent changeset hash
		vcsInfo.Revision = fmt.Sprintf("%x", dirstate[:20])
	}
}

// detectMonorepo checks if the workspace is a monorepo
func (d *WorkspaceDetector) detectMonorepo(rootPath string) (bool, []string) {
	subProjects := []string{}

	// Look for multiple project roots in subdirectories
	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return false, subProjects
	}

	projectDirs := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Skip common non-project directories
		skipDirs := map[string]bool{
			"node_modules": true, "vendor": true, ".git": true,
			".venv": true, "venv": true, "env": true,
			"__pycache__": true, "target": true, "build": true,
			"dist": true, ".idea": true, ".vscode": true,
		}

		if skipDirs[entry.Name()] {
			continue
		}

		subPath := filepath.Join(rootPath, entry.Name())
		if markers := d.checkRootMarkers(subPath); len(markers) > 0 {
			projectDirs++
			subProjects = append(subProjects, entry.Name())
		}
	}

	// Consider it a monorepo if there are multiple project directories
	// or specific monorepo indicators
	isMonorepo := projectDirs > 1 || d.hasMonorepoIndicators(rootPath)

	return isMonorepo, subProjects
}

// hasMonorepoIndicators checks for common monorepo configuration files
func (d *WorkspaceDetector) hasMonorepoIndicators(rootPath string) bool {
	indicators := []string{
		"lerna.json", "nx.json", "workspace.json",
		"pnpm-workspace.yaml", "rush.json",
		"bazel.BUILD", "WORKSPACE", "BUILD.bazel",
	}

	for _, indicator := range indicators {
		if _, err := os.Stat(filepath.Join(rootPath, indicator)); err == nil {
			return true
		}
	}

	return false
}

// detectWorkspaceBoundaries identifies workspace boundaries
func (d *WorkspaceDetector) detectWorkspaceBoundaries(rootPath string) []string {
	boundaries := []string{}

	// Common boundary indicators
	boundaryFiles := []string{
		".gitignore", ".dockerignore", ".gitmodules",
		"workspace.json", "lerna.json", "nx.json",
	}

	for _, boundaryFile := range boundaryFiles {
		if _, err := os.Stat(filepath.Join(rootPath, boundaryFile)); err == nil {
			boundaries = append(boundaries, boundaryFile)
		}
	}

	return boundaries
}

// GetRootMarkers returns the list of root markers used for detection
func (d *WorkspaceDetector) GetRootMarkers() []string {
	return append([]string{}, d.rootMarkers...) // Return a copy
}

// SetRootMarkers sets custom root markers for detection
func (d *WorkspaceDetector) SetRootMarkers(markers []string) {
	d.rootMarkers = append([]string{}, markers...) // Store a copy
}

// AddRootMarker adds a new root marker to the detection list
func (d *WorkspaceDetector) AddRootMarker(marker string) {
	d.rootMarkers = append(d.rootMarkers, marker)
}

// getCachedResult retrieves a cached detection result if still valid
func (d *WorkspaceDetector) getCachedResult(path string) *workspaceDetectionResult {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result, exists := d.cache[path]
	if !exists {
		return nil
	}

	// Check if cache entry is still valid
	if time.Since(result.timestamp) > d.options.CacheDuration {
		// Remove stale entry
		delete(d.cache, path)
		return nil
	}

	return result
}

// cacheResult stores a detection result in the cache
func (d *WorkspaceDetector) cacheResult(path string, result *workspaceDetectionResult) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Limit cache size
	if len(d.cache) >= 1000 {
		// Remove oldest entries
		oldest := time.Now()
		oldestKey := ""
		for k, v := range d.cache {
			if v.timestamp.Before(oldest) {
				oldest = v.timestamp
				oldestKey = k
			}
		}
		if oldestKey != "" {
			delete(d.cache, oldestKey)
		}
	}

	d.cache[path] = result
}

// ClearCache clears the detection cache
func (d *WorkspaceDetector) ClearCache() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cache = make(map[string]*workspaceDetectionResult)
}

// GetCacheStats returns cache statistics
func (d *WorkspaceDetector) GetCacheStats() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()

	validEntries := 0
	expiredEntries := 0
	now := time.Now()

	for _, result := range d.cache {
		if now.Sub(result.timestamp) <= d.options.CacheDuration {
			validEntries++
		} else {
			expiredEntries++
		}
	}

	return map[string]interface{}{
		"total_entries":   len(d.cache),
		"valid_entries":   validEntries,
		"expired_entries": expiredEntries,
		"cache_duration":  d.options.CacheDuration.String(),
	}
}
