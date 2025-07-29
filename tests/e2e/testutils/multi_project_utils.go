package testutils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lsp-gateway/internal/config"
)

// Using SubProjectInfo from multi_project_manager.go to avoid conflicts

// EnhancedMultiProjectConfig provides configuration for the enhanced multi-project manager
type EnhancedMultiProjectConfig struct {
	WorkspaceName    string
	TargetDir        string
	Languages        []string // Required languages: go, python, typescript, java
	CloneTimeout     time.Duration
	EnableLogging    bool
	ForceClean       bool
	PreserveGitDir   bool
	ParallelSetup    bool
}

// MultiProjectManager manages multi-project workspaces with support for Go, Python, TypeScript, and Java
type MultiProjectManager struct {
	workspaceDir string
	subProjects  map[string]*SubProjectInfo
	repoManagers map[string]RepositoryManager
	config       EnhancedMultiProjectConfig
	mu           sync.RWMutex
	lastError    error
	isSetup      bool
}

// NewMultiProjectManager creates a new multi-project manager with the given configuration
func NewMultiProjectManager(config EnhancedMultiProjectConfig) *MultiProjectManager {
	// Set defaults
	if config.CloneTimeout == 0 {
		config.CloneTimeout = 300 * time.Second
	}
	if config.WorkspaceName == "" {
		config.WorkspaceName = "multi-project-workspace"
	}
	if config.TargetDir == "" {
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		config.TargetDir = filepath.Join("/tmp", "lspg-multi-project-tests", uniqueID)
	}
	if len(config.Languages) == 0 {
		// Default to all supported languages
		config.Languages = []string{"go", "python", "typescript", "java"}
	}

	return &MultiProjectManager{
		workspaceDir: config.TargetDir,
		subProjects:  make(map[string]*SubProjectInfo),
		repoManagers: make(map[string]RepositoryManager),
		config:       config,
		isSetup:      false,
	}
}

// SetupWorkspace creates the workspace with all configured sub-projects
func (mpm *MultiProjectManager) SetupWorkspace() error {
	mpm.mu.Lock()
	defer mpm.mu.Unlock()

	if mpm.isSetup {
		return nil // Already set up
	}

	mpm.log("Setting up multi-project workspace with languages: %v", mpm.config.Languages)

	// Prepare workspace directory
	if err := mpm.prepareWorkspaceDirectory(); err != nil {
		mpm.lastError = fmt.Errorf("failed to prepare workspace directory: %w", err)
		return mpm.lastError
	}

	// Setup sub-projects
	if mpm.config.ParallelSetup {
		if err := mpm.setupSubProjectsParallel(); err != nil {
			mpm.lastError = fmt.Errorf("parallel sub-project setup failed: %w", err)
			return mpm.lastError
		}
	} else {
		if err := mpm.setupSubProjectsSequential(); err != nil {
			mpm.lastError = fmt.Errorf("sequential sub-project setup failed: %w", err)
			return mpm.lastError
		}
	}

	mpm.isSetup = true
	mpm.log("Multi-project workspace setup completed: %s", mpm.workspaceDir)
	return nil
}

// GetSubProjects returns all sub-projects in the workspace
func (mpm *MultiProjectManager) GetSubProjects() map[string]*SubProjectInfo {
	mpm.mu.RLock()
	defer mpm.mu.RUnlock()

	// Return a deep copy to prevent external modification
	result := make(map[string]*SubProjectInfo)
	for lang, subProject := range mpm.subProjects {
		result[lang] = &SubProjectInfo{
			Language:    subProject.Language,
			ProjectPath: subProject.ProjectPath,
			TestFiles:   append([]string{}, subProject.TestFiles...),
			RootMarkers: append([]string{}, subProject.RootMarkers...),
			LSPConfig:   make(map[string]string),
		}
		// Copy LSPConfig map
		for k, v := range subProject.LSPConfig {
			result[lang].LSPConfig[k] = v
		}
	}
	return result
}

// GetWorkspaceDir returns the workspace root directory
func (mpm *MultiProjectManager) GetWorkspaceDir() string {
	mpm.mu.RLock()
	defer mpm.mu.RUnlock()
	return mpm.workspaceDir
}

// GenerateConfig generates a multi-language configuration for the workspace
func (mpm *MultiProjectManager) GenerateConfig(port int) (string, error) {
	mpm.mu.RLock()
	defer mpm.mu.RUnlock()

	if !mpm.isSetup {
		return "", fmt.Errorf("workspace not set up, call SetupWorkspace() first")
	}

	// Create project info for configuration generator
	projectInfo, err := mpm.createMultiLanguageProjectInfo()
	if err != nil {
		return "", fmt.Errorf("failed to create project info: %w", err)
	}

	// Generate configuration using the existing config generator
	generator := config.NewConfigGenerator()
	multiLangConfig, err := generator.GenerateMultiLanguageConfig(projectInfo)
	if err != nil {
		return "", fmt.Errorf("failed to generate multi-language config: %w", err)
	}

	// Convert to YAML string with port configuration
	configYAML, err := mpm.convertConfigToYAML(multiLangConfig, port)
	if err != nil {
		return "", fmt.Errorf("failed to convert config to YAML: %w", err)
	}

	return configYAML, nil
}

// ValidateWorkspace validates that all sub-projects are properly set up
func (mpm *MultiProjectManager) ValidateWorkspace() error {
	mpm.mu.RLock()
	defer mpm.mu.RUnlock()

	if !mpm.isSetup {
		return fmt.Errorf("workspace not set up")
	}

	if len(mpm.subProjects) == 0 {
		return fmt.Errorf("no sub-projects found in workspace")
	}

	// Validate each sub-project
	for language, subProject := range mpm.subProjects {
		repoManager, exists := mpm.repoManagers[language]
		if !exists || repoManager == nil {
			return fmt.Errorf("repository manager not found for language: %s", language)
		}

		if err := repoManager.ValidateRepository(); err != nil {
			return fmt.Errorf("validation failed for %s project: %w", language, err)
		}

		// Validate project directory exists
		if _, err := os.Stat(subProject.ProjectPath); os.IsNotExist(err) {
			return fmt.Errorf("project directory not found for %s: %s", language, subProject.ProjectPath)
		}

		// Validate root markers exist
		if err := mpm.validateRootMarkers(subProject); err != nil {
			return fmt.Errorf("root marker validation failed for %s: %w", language, err)
		}

		mpm.log("Validation successful for %s project at: %s", language, subProject.ProjectPath)
	}

	return nil
}

// Cleanup removes the workspace directory and performs cleanup operations
func (mpm *MultiProjectManager) Cleanup() error {
	mpm.mu.Lock()
	defer mpm.mu.Unlock()

	mpm.log("Starting cleanup of multi-project workspace: %s", mpm.workspaceDir)

	// Cleanup individual repository managers
	for language, repoManager := range mpm.repoManagers {
		if repoManager != nil {
			if err := repoManager.Cleanup(); err != nil {
				mpm.log("Warning: cleanup failed for %s project: %v", language, err)
			}
		}
	}

	// Clear internal state
	mpm.subProjects = make(map[string]*SubProjectInfo)
	mpm.repoManagers = make(map[string]RepositoryManager)
	mpm.isSetup = false

	// Remove workspace directory
	if mpm.workspaceDir != "" {
		if _, err := os.Stat(mpm.workspaceDir); err == nil {
			if err := os.RemoveAll(mpm.workspaceDir); err != nil {
				mpm.log("Cleanup failed: %v", err)
				return fmt.Errorf("failed to remove workspace directory: %w", err)
			}
		}
	}

	mpm.log("Cleanup completed successfully")
	return nil
}

// Private helper methods

func (mpm *MultiProjectManager) log(format string, args ...interface{}) {
	if mpm.config.EnableLogging {
		fmt.Printf("[MultiProjectManager] %s\n", fmt.Sprintf(format, args...))
	}
}

func (mpm *MultiProjectManager) prepareWorkspaceDirectory() error {
	if mpm.config.ForceClean {
		if err := os.RemoveAll(mpm.workspaceDir); err != nil && !os.IsNotExist(err) {
			mpm.log("Warning: failed to clean existing workspace: %v", err)
		}
	}

	return os.MkdirAll(mpm.workspaceDir, 0755)
}

func (mpm *MultiProjectManager) setupSubProjectsSequential() error {
	for _, language := range mpm.config.Languages {
		if err := mpm.setupSubProject(language); err != nil {
			return fmt.Errorf("failed to setup %s project: %w", language, err)
		}
	}
	return nil
}

func (mpm *MultiProjectManager) setupSubProjectsParallel() error {
	type setupResult struct {
		language string
		err      error
	}

	results := make(chan setupResult, len(mpm.config.Languages))
	
	// Start parallel setup
	for _, language := range mpm.config.Languages {
		go func(lang string) {
			err := mpm.setupSubProject(lang)
			results <- setupResult{language: lang, err: err}
		}(language)
	}

	// Collect results
	var errors []string
	for i := 0; i < len(mpm.config.Languages); i++ {
		result := <-results
		if result.err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", result.language, result.err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("parallel setup failures: %s", strings.Join(errors, "; "))
	}

	return nil
}

func (mpm *MultiProjectManager) setupSubProject(language string) error {
	mpm.log("Setting up %s sub-project", language)

	// Get language configuration
	langConfig, err := mpm.getLanguageConfig(language)
	if err != nil {
		return fmt.Errorf("failed to get language config for %s: %w", language, err)
	}

	// Create sub-project directory
	subProjectDir := mpm.getSubProjectDirName(language)
	subProjectPath := filepath.Join(mpm.workspaceDir, subProjectDir)

	// Create repository manager configuration
	repoConfig := GenericRepoConfig{
		LanguageConfig: langConfig,
		TargetDir:      subProjectPath,
		CloneTimeout:   mpm.config.CloneTimeout,
		EnableLogging:  mpm.config.EnableLogging,
		ForceClean:     mpm.config.ForceClean,
		PreserveGitDir: mpm.config.PreserveGitDir,
	}

	// Extract commit hash from language config if available
	if commitHash, exists := langConfig.CustomVariables["commit_hash"]; exists {
		repoConfig.CommitHash = commitHash
	}

	// Create and setup repository manager
	repoManager := NewGenericRepoManager(repoConfig)
	actualPath, err := repoManager.SetupRepository()
	if err != nil {
		return fmt.Errorf("failed to setup repository for %s: %w", language, err)
	}

	// Get test files
	testFiles, err := repoManager.GetTestFiles()
	if err != nil {
		mpm.log("Warning: failed to get test files for %s: %v", language, err)
		testFiles = []string{} // Continue with empty test files
	}

	// Create SubProjectInfo
	subProjectInfo := &SubProjectInfo{
		Language:    language,
		ProjectPath: actualPath,
		TestFiles:   testFiles,
		RootMarkers: langConfig.RootMarkers,
		LSPConfig:   make(map[string]string),
	}

	// Store the sub-project and repository manager (thread-safe)
	mpm.subProjects[language] = subProjectInfo
	mpm.repoManagers[language] = repoManager

	mpm.log("Successfully set up %s sub-project at: %s", language, actualPath)
	return nil
}

func (mpm *MultiProjectManager) getLanguageConfig(language string) (LanguageConfig, error) {
	switch strings.ToLower(language) {
	case "go", "golang":
		return GetGoLanguageConfig(), nil
	case "python", "py":
		return GetPythonLanguageConfig(), nil
	case "typescript", "ts":
		return GetTypeScriptLanguageConfig(), nil
	case "javascript", "js":
		return GetJavaScriptLanguageConfig(), nil
	case "java":
		return GetJavaLanguageConfig(), nil
	default:
		return LanguageConfig{}, fmt.Errorf("unsupported language: %s", language)
	}
}

func (mpm *MultiProjectManager) getSubProjectDirName(language string) string {
	switch strings.ToLower(language) {
	case "go", "golang":
		return "go-project"
	case "python", "py":
		return "python-project"
	case "typescript", "ts":
		return "typescript-project"
	case "javascript", "js":
		return "javascript-project"
	case "java":
		return "java-project"
	default:
		return fmt.Sprintf("%s-project", language)
	}
}

func (mpm *MultiProjectManager) validateRootMarkers(subProject *SubProjectInfo) error {
	if len(subProject.RootMarkers) == 0 {
		return nil // No markers to validate
	}

	found := false
	for _, marker := range subProject.RootMarkers {
		markerPath := filepath.Join(subProject.ProjectPath, marker)
		if _, err := os.Stat(markerPath); err == nil {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("none of the expected root markers found: %v", subProject.RootMarkers)
	}

	return nil
}

func (mpm *MultiProjectManager) createMultiLanguageProjectInfo() (*config.MultiLanguageProjectInfo, error) {
	languageContexts := make([]*config.LanguageContext, 0, len(mpm.subProjects))

	for language, subProject := range mpm.subProjects {
		langCtx := &config.LanguageContext{
			Language:     language,
			FilePatterns: mpm.getFilePatterns(language),
			RootMarkers:  subProject.RootMarkers,
			RootPath:     subProject.ProjectPath,
			FileCount:    len(subProject.TestFiles),
			Frameworks:   []string{}, // Could be enhanced to detect frameworks
			Version:      "latest",
		}
		languageContexts = append(languageContexts, langCtx)
	}

	projectInfo := &config.MultiLanguageProjectInfo{
		ProjectType:       config.ProjectTypeMulti,
		RootDirectory:     mpm.workspaceDir,
		WorkspaceRoot:     mpm.workspaceDir,
		LanguageContexts:  languageContexts,
		DetectedAt:        time.Now(),
		Metadata:          map[string]interface{}{
			"workspace_name": mpm.config.WorkspaceName,
			"languages":      mpm.config.Languages,
		},
	}

	return projectInfo, nil
}

func (mpm *MultiProjectManager) getFilePatterns(language string) []string {
	switch strings.ToLower(language) {
	case "go", "golang":
		return []string{"*.go"}
	case "python", "py":
		return []string{"*.py"}
	case "typescript", "ts":
		return []string{"*.ts", "*.tsx"}
	case "javascript", "js":
		return []string{"*.js", "*.jsx"}
	case "java":
		return []string{"*.java"}
	default:
		return []string{"*"}
	}
}

func (mpm *MultiProjectManager) convertConfigToYAML(multiLangConfig *config.MultiLanguageConfig, port int) (string, error) {
	var yamlBuilder strings.Builder
	
	// Write server configuration header
	yamlBuilder.WriteString("# Multi-Project LSP Gateway Configuration\n")
	yamlBuilder.WriteString("# Generated at: " + time.Now().Format(time.RFC3339) + "\n\n")
	
	// Server configuration
	yamlBuilder.WriteString("server:\n")
	yamlBuilder.WriteString(fmt.Sprintf("  port: %d\n", port))
	yamlBuilder.WriteString("  host: \"localhost\"\n\n")
	
	// LSP servers configuration
	yamlBuilder.WriteString("servers:\n")
	
	for _, serverConfig := range multiLangConfig.ServerConfigs {
		yamlBuilder.WriteString(fmt.Sprintf("  - name: \"%s\"\n", serverConfig.Name))
		yamlBuilder.WriteString(fmt.Sprintf("    languages: [%s]\n", mpm.formatLanguageList(serverConfig.Languages)))
		yamlBuilder.WriteString(fmt.Sprintf("    command: \"%s\"\n", serverConfig.Command))
		
		if len(serverConfig.Args) > 0 {
			yamlBuilder.WriteString("    args:\n")
			for _, arg := range serverConfig.Args {
				yamlBuilder.WriteString(fmt.Sprintf("      - \"%s\"\n", arg))
			}
		}
		
		yamlBuilder.WriteString(fmt.Sprintf("    transport: \"%s\"\n", serverConfig.Transport))
		
		if len(serverConfig.RootMarkers) > 0 {
			yamlBuilder.WriteString("    root_markers:\n")
			for _, marker := range serverConfig.RootMarkers {
				yamlBuilder.WriteString(fmt.Sprintf("      - \"%s\"\n", marker))
			}
		}
		
		yamlBuilder.WriteString("\n")
	}
	
	// Performance configuration
	yamlBuilder.WriteString("performance:\n")
	yamlBuilder.WriteString("  cache:\n")
	yamlBuilder.WriteString("    enabled: true\n")
	yamlBuilder.WriteString("    memory_limit: \"1GB\"\n")
	yamlBuilder.WriteString("    disk_limit: \"5GB\"\n\n")
	
	// Logging configuration
	yamlBuilder.WriteString("logging:\n")
	yamlBuilder.WriteString("  level: \"info\"\n")
	yamlBuilder.WriteString("  format: \"json\"\n\n")
	
	// Workspace configuration
	yamlBuilder.WriteString("workspace:\n")
	yamlBuilder.WriteString("  multi_root: true\n")
	yamlBuilder.WriteString("  cross_language_references: true\n")
	yamlBuilder.WriteString("  language_roots:\n")
	
	for language, subProject := range mpm.subProjects {
		yamlBuilder.WriteString(fmt.Sprintf("    %s: \"%s\"\n", language, subProject.ProjectPath))
	}
	
	return yamlBuilder.String(), nil
}

func (mpm *MultiProjectManager) formatLanguageList(languages []string) string {
	quotedLanguages := make([]string, len(languages))
	for i, lang := range languages {
		quotedLanguages[i] = "\"" + lang + "\""
	}
	return strings.Join(quotedLanguages, ", ")
}

// Utility functions for creating language-specific managers

// NewGoRepositoryManagerForMultiProject creates a Go repository manager for multi-project use
func NewGoRepositoryManagerForMultiProject(targetDir string, config EnhancedMultiProjectConfig) *GenericRepoManager {
	repoConfig := GenericRepoConfig{
		LanguageConfig: GetGoLanguageConfig(),
		TargetDir:      targetDir,
		CloneTimeout:   config.CloneTimeout,
		EnableLogging:  config.EnableLogging,
		ForceClean:     config.ForceClean,
		PreserveGitDir: config.PreserveGitDir,
	}
	
	// Extract commit hash from language config
	langConfig := GetGoLanguageConfig()
	if commitHash, exists := langConfig.CustomVariables["commit_hash"]; exists {
		repoConfig.CommitHash = commitHash
	}
	
	return NewGenericRepoManager(repoConfig)
}

// NewPythonRepositoryManagerForMultiProject creates a Python repository manager for multi-project use
func NewPythonRepositoryManagerForMultiProject(targetDir string, config EnhancedMultiProjectConfig) *GenericRepoManager {
	repoConfig := GenericRepoConfig{
		LanguageConfig: GetPythonLanguageConfig(),
		TargetDir:      targetDir,
		CloneTimeout:   config.CloneTimeout,
		EnableLogging:  config.EnableLogging,
		ForceClean:     config.ForceClean,
		PreserveGitDir: config.PreserveGitDir,
	}
	
	return NewGenericRepoManager(repoConfig)
}

// NewTypeScriptRepositoryManagerForMultiProject creates a TypeScript repository manager for multi-project use
func NewTypeScriptRepositoryManagerForMultiProject(targetDir string, config EnhancedMultiProjectConfig) *GenericRepoManager {
	repoConfig := GenericRepoConfig{
		LanguageConfig: GetTypeScriptLanguageConfig(),
		TargetDir:      targetDir,
		CloneTimeout:   config.CloneTimeout,
		EnableLogging:  config.EnableLogging,
		ForceClean:     config.ForceClean,
		PreserveGitDir: config.PreserveGitDir,
	}
	
	// Extract commit hash from language config
	langConfig := GetTypeScriptLanguageConfig()
	if commitHash, exists := langConfig.CustomVariables["commit_hash"]; exists {
		repoConfig.CommitHash = commitHash
	}
	
	return NewGenericRepoManager(repoConfig)
}

// NewJavaRepositoryManagerForMultiProject creates a Java repository manager for multi-project use
func NewJavaRepositoryManagerForMultiProject(targetDir string, config EnhancedMultiProjectConfig) *GenericRepoManager {
	repoConfig := GenericRepoConfig{
		LanguageConfig: GetJavaLanguageConfig(),
		TargetDir:      targetDir,
		CloneTimeout:   config.CloneTimeout,
		EnableLogging:  config.EnableLogging,
		ForceClean:     config.ForceClean,
		PreserveGitDir: config.PreserveGitDir,
	}
	
	// Extract commit hash from language config
	langConfig := GetJavaLanguageConfig()
	if commitHash, exists := langConfig.CustomVariables["commit_hash"]; exists {
		repoConfig.CommitHash = commitHash
	}
	
	return NewGenericRepoManager(repoConfig)
}

// CreateDefaultMultiProjectManager creates a multi-project manager with default settings for all supported languages
func CreateDefaultMultiProjectManager() *MultiProjectManager {
	config := EnhancedMultiProjectConfig{
		WorkspaceName:  "default-multi-project",
		Languages:      []string{"go", "python", "typescript", "java"},
		CloneTimeout:   300 * time.Second,
		EnableLogging:  true,
		ForceClean:     true,
		PreserveGitDir: true,
		ParallelSetup:  true,
	}
	
	return NewMultiProjectManager(config)
}

// CreateQuickMultiProjectManager creates a multi-project manager optimized for quick testing
func CreateQuickMultiProjectManager(languages []string) *MultiProjectManager {
	config := EnhancedMultiProjectConfig{
		WorkspaceName:  "quick-test-workspace",
		Languages:      languages,
		CloneTimeout:   120 * time.Second,
		EnableLogging:  false,
		ForceClean:     true,
		PreserveGitDir: false,
		ParallelSetup:  true,
	}
	
	return NewMultiProjectManager(config)
}