package mcp

import (
	"context"
	"fmt"
	"lsp-gateway/internal/config"
	"lsp-gateway/internal/project"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// WorkspaceContext manages project-aware context for MCP tools
type WorkspaceContext struct {
	// Project information
	ProjectContext *project.ProjectContext `json:"project_context,omitempty"`
	ConfigContext  *config.ProjectContext  `json:"config_context,omitempty"`
	ProjectConfig  *config.ProjectConfig   `json:"project_config,omitempty"`

	// MCP-specific settings
	ProjectPath     string   `json:"project_path"`
	WorkspaceRoots  []string `json:"workspace_roots"`
	ActiveLanguages []string `json:"active_languages"`
	EnabledServers  []string `json:"enabled_servers"`

	// Caching and performance
	DetectionCache map[string]*project.ProjectContext `json:"-"`
	LastDetection  time.Time                          `json:"last_detection"`
	CacheTimeout   time.Duration                      `json:"cache_timeout"`

	// Thread safety
	mutex    sync.RWMutex            `json:"-"`
	detector project.ProjectDetector `json:"-"`

	// Status and metadata
	IsInitialized bool          `json:"is_initialized"`
	DetectionTime time.Duration `json:"detection_time"`
	Errors        []string      `json:"errors,omitempty"`
	Warnings      []string      `json:"warnings,omitempty"`
}

// WorkspaceContextConfig holds configuration for workspace context
type WorkspaceContextConfig struct {
	AutoDetectProject   bool          `json:"auto_detect_project"`
	ProjectPath         string        `json:"project_path,omitempty"`
	WorkspaceRoots      []string      `json:"workspace_roots,omitempty"`
	CacheTimeout        time.Duration `json:"cache_timeout"`
	MaxDetectionTime    time.Duration `json:"max_detection_time"`
	EnableProjectConfig bool          `json:"enable_project_config"`
}

// DefaultWorkspaceContextConfig returns default configuration
func DefaultWorkspaceContextConfig() *WorkspaceContextConfig {
	return &WorkspaceContextConfig{
		AutoDetectProject:   true,
		CacheTimeout:        10 * time.Minute,
		MaxDetectionTime:    30 * time.Second,
		EnableProjectConfig: true,
	}
}

// NewWorkspaceContext creates a new workspace context
func NewWorkspaceContext(config *WorkspaceContextConfig) *WorkspaceContext {
	if config == nil {
		config = DefaultWorkspaceContextConfig()
	}

	return &WorkspaceContext{
		ProjectPath:     config.ProjectPath,
		WorkspaceRoots:  config.WorkspaceRoots,
		ActiveLanguages: make([]string, 0),
		EnabledServers:  make([]string, 0),
		DetectionCache:  make(map[string]*project.ProjectContext),
		CacheTimeout:    config.CacheTimeout,
		detector:        project.NewProjectDetector(),
		IsInitialized:   false,
		Errors:          make([]string, 0),
		Warnings:        make([]string, 0),
	}
}

// Initialize performs initial workspace detection and setup
func (w *WorkspaceContext) Initialize(ctx context.Context) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	startTime := time.Now()

	// Clear previous state
	w.Errors = make([]string, 0)
	w.Warnings = make([]string, 0)

	// Auto-detect project if no explicit path provided
	if w.ProjectPath == "" {
		if cwd, err := os.Getwd(); err == nil {
			w.ProjectPath = cwd
		} else {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	// Perform project detection
	projectCtx, err := w.detector.DetectProject(ctx, w.ProjectPath)
	if err != nil {
		w.addError(fmt.Sprintf("Project detection failed: %v", err))
		// Continue with limited functionality
	} else {
		w.ProjectContext = projectCtx
		w.updateFromProjectContext()
	}

	// Convert to config format for compatibility
	if w.ProjectContext != nil {
		w.ConfigContext = w.convertToConfigContext(w.ProjectContext)
		w.ProjectConfig = w.generateProjectConfig(w.ProjectContext)
	}

	// Cache the detection result
	if w.ProjectContext != nil {
		w.DetectionCache[w.ProjectPath] = w.ProjectContext
	}

	w.LastDetection = startTime
	w.DetectionTime = time.Since(startTime)
	w.IsInitialized = true

	return nil
}

// GetProjectContext returns the current project context with thread safety
func (w *WorkspaceContext) GetProjectContext() *project.ProjectContext {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	return w.ProjectContext
}

// GetConfigContext returns config-compatible project context
func (w *WorkspaceContext) GetConfigContext() *config.ProjectContext {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	return w.ConfigContext
}

// GetProjectConfig returns project-specific configuration
func (w *WorkspaceContext) GetProjectConfig() *config.ProjectConfig {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	return w.ProjectConfig
}

// IsProjectAware returns whether the workspace has project awareness
func (w *WorkspaceContext) IsProjectAware() bool {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	return w.IsInitialized && w.ProjectContext != nil
}

// GetActiveLanguages returns languages detected in the current project
func (w *WorkspaceContext) GetActiveLanguages() []string {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	return append([]string(nil), w.ActiveLanguages...)
}

// GetEnabledServers returns LSP servers enabled for the current project
func (w *WorkspaceContext) GetEnabledServers() []string {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	return append([]string(nil), w.EnabledServers...)
}

// GetWorkspaceRoots returns all workspace root directories
func (w *WorkspaceContext) GetWorkspaceRoots() []string {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	return append([]string(nil), w.WorkspaceRoots...)
}

// IsFileInProject checks if a file URI belongs to the current project
func (w *WorkspaceContext) IsFileInProject(uri string) bool {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	if w.ProjectContext == nil {
		return false
	}

	// Convert URI to file path
	filePath := w.uriToPath(uri)
	if filePath == "" {
		return false
	}

	// Check if file is within project root
	absProjectRoot, _ := filepath.Abs(w.ProjectContext.RootPath)
	absFilePath, _ := filepath.Abs(filePath)

	return strings.HasPrefix(absFilePath, absProjectRoot)
}

// GetProjectRoot returns the project root directory
func (w *WorkspaceContext) GetProjectRoot() string {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	if w.ProjectContext != nil {
		return w.ProjectContext.RootPath
	}
	return w.ProjectPath
}

// GetLanguageForFile determines the language for a given file URI
func (w *WorkspaceContext) GetLanguageForFile(uri string) string {
	filePath := w.uriToPath(uri)
	if filePath == "" {
		return ""
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return LANG_PYTHON
	case ".js", ".jsx":
		return LANG_JAVASCRIPT
	case ".ts", ".tsx":
		return LANG_TYPESCRIPT
	case ".java":
		return LANG_JAVA
	case ".rs":
		return "rust"
	case ".c", ".h":
		return "c"
	case ".cpp", ".hpp", ".cc", ".cxx":
		return "cpp"
	default:
		return ""
	}
}

// RefreshProject re-detects the project if cache has expired
func (w *WorkspaceContext) RefreshProject(ctx context.Context) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	// Check if refresh is needed
	if time.Since(w.LastDetection) < w.CacheTimeout {
		return nil // Cache still valid
	}

	// Re-detect project
	projectCtx, err := w.detector.DetectProject(ctx, w.ProjectPath)
	if err != nil {
		w.addError(fmt.Sprintf("Project refresh failed: %v", err))
		return err
	}

	w.ProjectContext = projectCtx
	w.updateFromProjectContext()

	// Update config contexts
	w.ConfigContext = w.convertToConfigContext(w.ProjectContext)
	w.ProjectConfig = w.generateProjectConfig(w.ProjectContext)

	// Update cache
	w.DetectionCache[w.ProjectPath] = w.ProjectContext
	w.LastDetection = time.Now()

	return nil
}

// EnhanceToolArgs adds project context to tool arguments
func (w *WorkspaceContext) EnhanceToolArgs(toolName string, args map[string]interface{}) map[string]interface{} {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	if !w.IsInitialized || w.ProjectContext == nil {
		return args
	}

	enhanced := make(map[string]interface{})
	for k, v := range args {
		enhanced[k] = v
	}

	// Add project context information
	enhanced["project_root"] = w.ProjectContext.RootPath
	enhanced["project_type"] = w.ProjectContext.ProjectType
	enhanced["primary_language"] = w.ProjectContext.PrimaryLanguage
	enhanced["active_languages"] = w.ActiveLanguages
	enhanced["workspace_roots"] = w.WorkspaceRoots

	// Add file context if URI is provided
	if uri, ok := args["uri"].(string); ok {
		enhanced["is_in_project"] = w.IsFileInProject(uri)
		enhanced["file_language"] = w.GetLanguageForFile(uri)
		enhanced["relative_path"] = w.getRelativePath(uri)
	}

	return enhanced
}

// GetProjectMetadata returns project-specific metadata for MCP responses
func (w *WorkspaceContext) GetProjectMetadata() map[string]interface{} {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	metadata := make(map[string]interface{})

	if w.ProjectContext != nil {
		metadata["project_type"] = w.ProjectContext.ProjectType
		metadata["primary_language"] = w.ProjectContext.PrimaryLanguage
		metadata["languages"] = w.ProjectContext.Languages
		metadata["required_servers"] = w.ProjectContext.RequiredServers
		metadata["project_root"] = w.ProjectContext.RootPath
		metadata["workspace_root"] = w.ProjectContext.WorkspaceRoot
		metadata["is_monorepo"] = w.ProjectContext.IsMonorepo
		metadata["confidence"] = w.ProjectContext.Confidence
		metadata["detection_time"] = w.ProjectContext.DetectionTime.String()
	}

	metadata["is_project_aware"] = w.IsInitialized
	metadata["cache_valid"] = time.Since(w.LastDetection) < w.CacheTimeout
	metadata["workspace_roots"] = w.WorkspaceRoots

	return metadata
}

// Private helper methods

func (w *WorkspaceContext) updateFromProjectContext() {
	if w.ProjectContext == nil {
		return
	}

	w.ActiveLanguages = append([]string(nil), w.ProjectContext.Languages...)
	w.EnabledServers = append([]string(nil), w.ProjectContext.RequiredServers...)

	// Add workspace root if different from project root
	if w.ProjectContext.WorkspaceRoot != w.ProjectContext.RootPath {
		w.WorkspaceRoots = []string{w.ProjectContext.WorkspaceRoot, w.ProjectContext.RootPath}
	} else {
		w.WorkspaceRoots = []string{w.ProjectContext.RootPath}
	}
}

func (w *WorkspaceContext) convertToConfigContext(projectCtx *project.ProjectContext) *config.ProjectContext {
	if projectCtx == nil {
		return nil
	}

	var languages []config.LanguageInfo
	for _, lang := range projectCtx.Languages {
		langInfo := config.LanguageInfo{
			Language:     lang,
			FilePatterns: w.getFilePatterns(lang),
			FileCount:    w.getLanguageFileCount(lang, projectCtx),
		}
		languages = append(languages, langInfo)
	}

	configCtx := &config.ProjectContext{
		ProjectType:   w.mapProjectType(projectCtx.ProjectType),
		RootDirectory: projectCtx.RootPath,
		WorkspaceRoot: projectCtx.WorkspaceRoot,
		Languages:     languages,
		RequiredLSPs:  projectCtx.RequiredServers,
		DetectedAt:    projectCtx.DetectedAt,
		Metadata:      projectCtx.Metadata,
	}

	return configCtx
}

func (w *WorkspaceContext) generateProjectConfig(projectCtx *project.ProjectContext) *config.ProjectConfig {
	if projectCtx == nil {
		return nil
	}

	projectConfig := &config.ProjectConfig{
		ProjectID:      fmt.Sprintf("project_%s_%d", projectCtx.ProjectType, time.Now().Unix()),
		Name:           projectCtx.DisplayName,
		RootDirectory:  projectCtx.RootPath,
		EnabledServers: projectCtx.RequiredServers,
		GeneratedAt:    time.Now(),
		Version:        "1.0.0",
	}

	// Add server overrides based on project type
	var overrides []config.ProjectServerOverride
	for _, server := range projectCtx.RequiredServers {
		override := config.ProjectServerOverride{
			Name:    server,
			Enabled: &[]bool{true}[0], // Pointer to true
		}
		overrides = append(overrides, override)
	}

	projectConfig.ServerOverrides = overrides

	return projectConfig
}

func (w *WorkspaceContext) mapProjectType(projectType string) string {
	switch projectType {
	case "mixed":
		return config.ProjectTypeMulti
	case "monorepo":
		return config.ProjectTypeMonorepo
	default:
		return config.ProjectTypeSingle
	}
}

func (w *WorkspaceContext) getFilePatterns(language string) []string {
	switch language {
	case "go":
		return []string{"*.go"}
	case LANG_PYTHON:
		return []string{"*.py"}
	case LANG_JAVASCRIPT:
		return []string{"*.js", "*.jsx"}
	case LANG_TYPESCRIPT:
		return []string{"*.ts", "*.tsx"}
	case LANG_JAVA:
		return []string{"*.java"}
	default:
		return []string{}
	}
}

func (w *WorkspaceContext) getLanguageFileCount(language string, projectCtx *project.ProjectContext) int {
	// This would need to be implemented based on actual file counting
	// For now, return a placeholder based on project size
	switch language {
	case projectCtx.PrimaryLanguage:
		return int(float64(projectCtx.ProjectSize.SourceFiles) * 0.7)
	default:
		return int(float64(projectCtx.ProjectSize.SourceFiles) * 0.3 / float64(len(projectCtx.Languages)-1))
	}
}

func (w *WorkspaceContext) uriToPath(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		return strings.TrimPrefix(uri, "file://")
	}
	return uri
}

func (w *WorkspaceContext) getRelativePath(uri string) string {
	filePath := w.uriToPath(uri)
	if filePath == "" || w.ProjectContext == nil {
		return ""
	}

	relPath, err := filepath.Rel(w.ProjectContext.RootPath, filePath)
	if err != nil {
		return filepath.Base(filePath)
	}
	return relPath
}

func (w *WorkspaceContext) addError(error string) {
	w.Errors = append(w.Errors, error)
}

// GetErrors returns all errors encountered during workspace context operations
func (w *WorkspaceContext) GetErrors() []string {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	return append([]string(nil), w.Errors...)
}

// GetWarnings returns all warnings from workspace context operations
func (w *WorkspaceContext) GetWarnings() []string {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	return append([]string(nil), w.Warnings...)
}

// HasErrors returns whether any errors occurred
func (w *WorkspaceContext) HasErrors() bool {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	return len(w.Errors) > 0
}

// ClearErrors clears all stored errors and warnings
func (w *WorkspaceContext) ClearErrors() {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	w.Errors = make([]string, 0)
	w.Warnings = make([]string, 0)
}
