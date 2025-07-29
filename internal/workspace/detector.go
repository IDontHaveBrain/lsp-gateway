package workspace

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lsp-gateway/internal/project/types"
)

// WorkspaceDetector provides workspace detection with sub-project scanning
type WorkspaceDetector interface {
	DetectWorkspace() (*WorkspaceContext, error)
	DetectWorkspaceAt(path string) (*WorkspaceContext, error)
	DetectWorkspaceWithContext(ctx context.Context, path string) (*WorkspaceContext, error)
	GenerateWorkspaceID(workspaceRoot string) string
	ValidateWorkspace(workspaceRoot string) error
	FindSubProjectForPath(workspace *WorkspaceContext, filePath string) *DetectedSubProject
}

// DetectedSubProject represents a detected sub-project within a workspace
type DetectedSubProject struct {
	ID           string   `json:"id"`             // Unique identifier
	Name         string   `json:"name"`           // Project name
	RelativePath string   `json:"relative_path"`  // Relative to workspace root
	AbsolutePath string   `json:"absolute_path"`  // Full path
	ProjectType  string   `json:"project_type"`   // Primary language/type
	Languages    []string `json:"languages"`      // All languages in project
	MarkerFiles  []string `json:"marker_files"`   // Detected marker files
	WorkspaceFolder string `json:"workspace_folder"` // For LSP workspace folder config
}

// WorkspaceContext contains workspace information with sub-project support
type WorkspaceContext struct {
	ID           string                    `json:"id"`
	Root         string                    `json:"root"`
	SubProjects  []*DetectedSubProject             `json:"sub_projects"`       // All detected sub-projects
	ProjectPaths map[string]*DetectedSubProject    `json:"project_paths"`      // Path -> DetectedSubProject mapping
	Languages    []string                  `json:"languages"`          // All languages in workspace
	Hash         string                    `json:"hash"`
	CreatedAt    time.Time                 `json:"created_at"`
	
	// Legacy fields for backward compatibility
	ProjectType  string    `json:"project_type"`    // Primary project type (for compatibility)
}

// DefaultWorkspaceDetector implements workspace detection with sub-project scanning
type DefaultWorkspaceDetector struct {
	languageDetectors map[string]*LanguageDetector
	maxDepth         int
	maxFiles         int
	ignorePatterns   []string
}

// LanguageDetector provides simplified language detection for root-level files
type LanguageDetector struct {
	Language     string
	MarkerFiles  []string
	Extensions   []string
	Priority     int
	ServerName   string
}

// NewWorkspaceDetector creates a new workspace detector with sub-project scanning
func NewWorkspaceDetector() WorkspaceDetector {
	detector := &DefaultWorkspaceDetector{
		languageDetectors: make(map[string]*LanguageDetector),
		maxDepth:         5, // Default to 5 levels as per requirements
		maxFiles:         types.MAX_FILES_TO_SCAN,
		ignorePatterns: []string{
			types.DIR_NODE_MODULES,
			types.DIR_DOT_GIT,
			types.DIR_VENDOR,
			types.DIR_TARGET,
			types.DIR_BUILD,
			types.DIR_DIST,
			types.DIR_VENV,
			types.DIR_ENV,
		},
	}
	
	detector.initializeLanguageDetectors()
	return detector
}

// initializeLanguageDetectors sets up simplified language detection patterns
func (d *DefaultWorkspaceDetector) initializeLanguageDetectors() {
	// Go language detection
	d.languageDetectors[types.PROJECT_TYPE_GO] = &LanguageDetector{
		Language: types.PROJECT_TYPE_GO,
		MarkerFiles: []string{
			types.MARKER_GO_MOD,
			types.MARKER_GO_SUM,
		},
		Extensions: []string{".go"},
		Priority:   types.PRIORITY_GO,
		ServerName: types.SERVER_GOPLS,
	}

	// Python language detection
	d.languageDetectors[types.PROJECT_TYPE_PYTHON] = &LanguageDetector{
		Language: types.PROJECT_TYPE_PYTHON,
		MarkerFiles: []string{
			types.MARKER_SETUP_PY,
			types.MARKER_PYPROJECT,
			types.MARKER_REQUIREMENTS,
			types.MARKER_PIPFILE,
		},
		Extensions: []string{".py", ".pyx", ".pyi"},
		Priority:   types.PRIORITY_PYTHON,
		ServerName: types.SERVER_PYLSP,
	}

	// TypeScript language detection
	d.languageDetectors[types.PROJECT_TYPE_TYPESCRIPT] = &LanguageDetector{
		Language: types.PROJECT_TYPE_TYPESCRIPT,
		MarkerFiles: []string{
			types.MARKER_TSCONFIG,
		},
		Extensions: []string{".ts", ".tsx"},
		Priority:   types.PRIORITY_TYPESCRIPT,
		ServerName: types.SERVER_TYPESCRIPT_LANG_SERVER,
	}

	// Node.js language detection
	d.languageDetectors[types.PROJECT_TYPE_NODEJS] = &LanguageDetector{
		Language: types.PROJECT_TYPE_NODEJS,
		MarkerFiles: []string{
			types.MARKER_PACKAGE_JSON,
			types.MARKER_YARN_LOCK,
			types.MARKER_PNPM_LOCK,
		},
		Extensions: []string{".js", ".jsx", ".mjs", ".cjs"},
		Priority:   types.PRIORITY_NODEJS,
		ServerName: types.SERVER_TYPESCRIPT_LANG_SERVER,
	}

	// Java language detection
	d.languageDetectors[types.PROJECT_TYPE_JAVA] = &LanguageDetector{
		Language: types.PROJECT_TYPE_JAVA,
		MarkerFiles: []string{
			types.MARKER_POM_XML,
			types.MARKER_BUILD_GRADLE,
		},
		Extensions: []string{".java", ".jar"},
		Priority:   types.PRIORITY_JAVA,
		ServerName: types.SERVER_JDTLS,
	}
}

// DetectWorkspace detects workspace at current working directory
func (d *DefaultWorkspaceDetector) DetectWorkspace() (*WorkspaceContext, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}
	
	return d.DetectWorkspaceAt(workingDir)
}

// DetectWorkspaceAt detects workspace at specified path with sub-project scanning
func (d *DefaultWorkspaceDetector) DetectWorkspaceAt(path string) (*WorkspaceContext, error) {
	return d.DetectWorkspaceWithContext(context.Background(), path)
}

// DetectWorkspaceWithContext detects workspace with context for cancellation
func (d *DefaultWorkspaceDetector) DetectWorkspaceWithContext(ctx context.Context, path string) (*WorkspaceContext, error) {
	if err := d.ValidateWorkspace(path); err != nil {
		return nil, fmt.Errorf("workspace validation failed: %w", err)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Detect sub-projects recursively
	subProjects, err := d.detectSubProjects(ctx, absPath, d.maxDepth)
	if err != nil {
		return nil, fmt.Errorf("sub-project detection failed: %w", err)
	}

	// Build project path mapping for efficient lookup
	projectPaths := d.buildProjectPathMapping(subProjects)

	// Determine workspace-level languages
	workspaceLanguages := d.determineWorkspaceLanguages(subProjects)

	// Determine primary project type for backward compatibility
	primaryType := d.determinePrimaryProjectType(subProjects, workspaceLanguages)
	
	workspace := &WorkspaceContext{
		Root:         absPath,
		SubProjects:  subProjects,
		ProjectPaths: projectPaths,
		Languages:    workspaceLanguages,
		ProjectType:  primaryType, // Legacy field
		CreatedAt:    time.Now(),
	}

	// Generate workspace ID and hash
	workspace.ID = d.GenerateWorkspaceID(absPath)
	workspace.Hash = d.generateWorkspaceHash(workspace)

	return workspace, nil
}

// detectSubProjects recursively scans for sub-projects within the workspace
func (d *DefaultWorkspaceDetector) detectSubProjects(ctx context.Context, rootPath string, maxDepth int) ([]*DetectedSubProject, error) {
	var subProjects []*DetectedSubProject
	var mu sync.Mutex
	filesScanned := 0

	err := d.walkDirectory(ctx, rootPath, rootPath, 0, maxDepth, &filesScanned, func(projectPath, relativePath string, markerFiles []string) error {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Create sub-project
		subProject, err := d.createSubProject(projectPath, relativePath, markerFiles)
		if err != nil {
			return fmt.Errorf("failed to create sub-project at %s: %w", projectPath, err)
		}

		mu.Lock()
		subProjects = append(subProjects, subProject)
		mu.Unlock()

		return nil
	})

	if err != nil {
		return nil, err
	}

	// If no sub-projects detected, create one for the root
	if len(subProjects) == 0 {
		rootSubProject, err := d.createSubProject(rootPath, ".", d.detectMarkerFiles(rootPath))
		if err != nil {
			return nil, fmt.Errorf("failed to create root sub-project: %w", err)
		}
		subProjects = append(subProjects, rootSubProject)
	}

	return subProjects, nil
}

// walkDirectory performs recursive directory traversal with ignore patterns
func (d *DefaultWorkspaceDetector) walkDirectory(ctx context.Context, rootPath, currentPath string, currentDepth, maxDepth int, filesScanned *int, projectCallback func(string, string, []string) error) error {
	// Check depth limit
	if currentDepth > maxDepth {
		return nil
	}

	// Check files scanned limit
	if *filesScanned >= d.maxFiles {
		return nil
	}

	entries, err := os.ReadDir(currentPath)
	if err != nil {
		// Log warning but continue with other directories
		return nil
	}

	// Check for project markers in current directory
	markerFiles := d.detectMarkerFiles(currentPath)
	if len(markerFiles) > 0 {
		relativePath, err := filepath.Rel(rootPath, currentPath)
		if err != nil {
			relativePath = currentPath
		}
		if err := projectCallback(currentPath, relativePath, markerFiles); err != nil {
			return err
		}
	}

	// Recursively scan subdirectories
	for _, entry := range entries {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		*filesScanned++
		if *filesScanned >= d.maxFiles {
			break
		}

		if !entry.IsDir() {
			continue
		}

		// Skip ignored directories
		if d.shouldIgnoreDirectory(entry.Name()) {
			continue
		}

		subDirPath := filepath.Join(currentPath, entry.Name())
		if err := d.walkDirectory(ctx, rootPath, subDirPath, currentDepth+1, maxDepth, filesScanned, projectCallback); err != nil {
			return err
		}
	}

	return nil
}

// shouldIgnoreDirectory checks if a directory should be ignored during scanning
func (d *DefaultWorkspaceDetector) shouldIgnoreDirectory(dirName string) bool {
	for _, pattern := range d.ignorePatterns {
		if dirName == pattern {
			return true
		}
	}
	return false
}

// detectMarkerFiles finds project marker files in a directory
func (d *DefaultWorkspaceDetector) detectMarkerFiles(dirPath string) []string {
	var markerFiles []string

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return markerFiles
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		for _, detector := range d.languageDetectors {
			for _, markerFile := range detector.MarkerFiles {
				if fileName == markerFile {
					markerFiles = append(markerFiles, fileName)
					break
				}
			}
		}
	}

	return markerFiles
}

// createSubProject creates a DetectedSubProject instance from a detected project path
func (d *DefaultWorkspaceDetector) createSubProject(projectPath, relativePath string, markerFiles []string) (*DetectedSubProject, error) {
	// Detect languages for this sub-project
	languages, primaryType := d.detectLanguagesAtPath(projectPath, markerFiles)

	// Generate project name from path
	projectName := filepath.Base(projectPath)
	if relativePath == "." {
		projectName = filepath.Base(projectPath)
	} else {
		projectName = strings.ReplaceAll(relativePath, string(filepath.Separator), "-")
	}

	subProject := &DetectedSubProject{
		ID:           d.generateProjectID(projectPath),
		Name:         projectName,
		RelativePath: relativePath,
		AbsolutePath: projectPath,
		ProjectType:  primaryType,
		Languages:    languages,
		MarkerFiles:  markerFiles,
		WorkspaceFolder: projectPath, // Use absolute path for LSP workspace folder
	}

	return subProject, nil
}

// detectLanguagesAtPath detects languages at a specific path, considering marker files
func (d *DefaultWorkspaceDetector) detectLanguagesAtPath(dirPath string, markerFiles []string) ([]string, string) {
	languageScores := make(map[string]int)
	detectedLanguages := make(map[string]bool)

	// Score based on marker files first (higher priority)
	for _, markerFile := range markerFiles {
		for langType, detector := range d.languageDetectors {
			for _, detectorMarker := range detector.MarkerFiles {
				if markerFile == detectorMarker {
					languageScores[langType] += detector.Priority
					detectedLanguages[langType] = true
				}
			}
		}
	}

	// Check file extensions for additional context
	entries, err := os.ReadDir(dirPath)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			fileName := entry.Name()
			for langType, detector := range d.languageDetectors {
				for _, ext := range detector.Extensions {
					if strings.HasSuffix(strings.ToLower(fileName), ext) {
						// Give lower weight to extension matches than marker files
						languageScores[langType] += detector.Priority / 4
						detectedLanguages[langType] = true
					}
				}
			}
		}
	}

	// Convert detected languages to slice
	var languages []string
	for lang := range detectedLanguages {
		languages = append(languages, lang)
	}

	// Determine primary project type
	primaryType := d.determinePrimaryType(languageScores)
	
	// Handle multiple languages or no languages
	if len(languages) > 1 {
		primaryType = types.PROJECT_TYPE_MIXED
	} else if len(languages) == 0 {
		primaryType = types.PROJECT_TYPE_UNKNOWN
		languages = []string{types.PROJECT_TYPE_UNKNOWN}
	}

	return languages, primaryType
}

// buildProjectPathMapping creates efficient path-to-project mapping
func (d *DefaultWorkspaceDetector) buildProjectPathMapping(subProjects []*DetectedSubProject) map[string]*DetectedSubProject {
	projectPaths := make(map[string]*DetectedSubProject)

	// Sort projects by path depth (deepest first) for proper precedence
	for i := 0; i < len(subProjects); i++ {
		for j := i + 1; j < len(subProjects); j++ {
			// Sort by path depth (number of separators), deepest first
			depthI := strings.Count(subProjects[i].AbsolutePath, string(filepath.Separator))
			depthJ := strings.Count(subProjects[j].AbsolutePath, string(filepath.Separator))
			if depthI < depthJ {
				subProjects[i], subProjects[j] = subProjects[j], subProjects[i]
			}
		}
	}

	// Build mapping with deepest projects taking precedence
	for _, project := range subProjects {
		projectPaths[project.AbsolutePath] = project
		// Also map the relative path for convenience
		projectPaths[project.RelativePath] = project
	}

	return projectPaths
}

// determineWorkspaceLanguages aggregates all languages from sub-projects
func (d *DefaultWorkspaceDetector) determineWorkspaceLanguages(subProjects []*DetectedSubProject) []string {
	languageSet := make(map[string]bool)

	for _, project := range subProjects {
		for _, lang := range project.Languages {
			if lang != types.PROJECT_TYPE_UNKNOWN {
				languageSet[lang] = true
			}
		}
	}

	var languages []string
	for lang := range languageSet {
		languages = append(languages, lang)
	}

	// If no languages detected, return unknown
	if len(languages) == 0 {
		languages = []string{types.PROJECT_TYPE_UNKNOWN}
	}

	return languages
}

// determinePrimaryProjectType determines the primary project type for backward compatibility
func (d *DefaultWorkspaceDetector) determinePrimaryProjectType(subProjects []*DetectedSubProject, workspaceLanguages []string) string {
	if len(workspaceLanguages) == 0 {
		return types.PROJECT_TYPE_UNKNOWN
	}

	if len(workspaceLanguages) > 1 {
		return types.PROJECT_TYPE_MIXED
	}

	// If only one language, return it
	if len(workspaceLanguages) == 1 {
		return workspaceLanguages[0]
	}

	// Fallback: use the primary type of the root project if available
	for _, project := range subProjects {
		if project.RelativePath == "." {
			return project.ProjectType
		}
	}

	return types.PROJECT_TYPE_UNKNOWN
}

// generateProjectID creates a unique identifier for a sub-project
func (d *DefaultWorkspaceDetector) generateProjectID(projectPath string) string {
	hash := sha256.Sum256([]byte(projectPath))
	return fmt.Sprintf("project_%x", hash[:8])
}

// FindSubProjectForPath finds the most specific sub-project for a given file path
func (d *DefaultWorkspaceDetector) FindSubProjectForPath(workspace *WorkspaceContext, filePath string) *DetectedSubProject {
	if workspace == nil || workspace.ProjectPaths == nil {
		return nil
	}

	// Try absolute path first
	if project, exists := workspace.ProjectPaths[filePath]; exists {
		return project
	}

	// Find the most specific (deepest) project that contains this file
	var bestMatch *DetectedSubProject
	bestMatchDepth := -1

	for _, project := range workspace.SubProjects {
		if strings.HasPrefix(filePath, project.AbsolutePath) {
			depth := strings.Count(project.AbsolutePath, string(filepath.Separator))
			if depth > bestMatchDepth {
				bestMatch = project
				bestMatchDepth = depth
			}
		}
	}

	return bestMatch
}

// determinePrimaryType finds the language with the highest score
func (d *DefaultWorkspaceDetector) determinePrimaryType(languageScores map[string]int) string {
	if len(languageScores) == 0 {
		return types.PROJECT_TYPE_UNKNOWN
	}

	maxScore := 0
	primaryType := types.PROJECT_TYPE_UNKNOWN

	for langType, score := range languageScores {
		if score > maxScore {
			maxScore = score
			primaryType = langType
		}
	}

	return primaryType
}

// GenerateWorkspaceID creates a consistent workspace identifier based on path
func (d *DefaultWorkspaceDetector) GenerateWorkspaceID(workspaceRoot string) string {
	// Use hash of absolute path for consistent ID generation
	hash := sha256.Sum256([]byte(workspaceRoot))
	return fmt.Sprintf("workspace_%x", hash[:8])
}

// generateWorkspaceHash creates a hash representing workspace state including sub-projects
func (d *DefaultWorkspaceDetector) generateWorkspaceHash(workspace *WorkspaceContext) string {
	// Include sub-project information in hash for more accurate state representation
	var subProjectInfo []string
	for _, project := range workspace.SubProjects {
		subProjectInfo = append(subProjectInfo, fmt.Sprintf("%s:%s:%s", 
			project.RelativePath, 
			project.ProjectType, 
			strings.Join(project.Languages, ","),
		))
	}

	content := fmt.Sprintf("%s:%s:%s:%s", 
		workspace.Root,
		workspace.ProjectType,
		strings.Join(workspace.Languages, ","),
		strings.Join(subProjectInfo, ";"),
	)
	
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash[:16])
}

// ValidateWorkspace ensures workspace directory exists and is accessible
func (d *DefaultWorkspaceDetector) ValidateWorkspace(workspaceRoot string) error {
	if workspaceRoot == "" {
		return fmt.Errorf("workspace root cannot be empty")
	}

	info, err := os.Stat(workspaceRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("workspace directory does not exist: %s", workspaceRoot)
		}
		return fmt.Errorf("cannot access workspace directory: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("workspace path is not a directory: %s", workspaceRoot)
	}

	// Check if directory is readable
	entries, err := os.ReadDir(workspaceRoot)
	if err != nil {
		return fmt.Errorf("cannot read workspace directory: %w", err)
	}

	// Basic sanity check - ensure it's not completely empty or inaccessible
	_ = entries // We just want to ensure we can read it

	return nil
}