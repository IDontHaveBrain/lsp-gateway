package testutils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// MultiProjectRepositoryManager provides an interface for managing multi-project workspaces
type MultiProjectRepositoryManager interface {
	SetupMultiProjectWorkspace(languages []string) (string, error)
	GetSubProjects() []*SubProjectInfo
	GetSubProjectPath(projectType string) (string, error)
	ValidateMultiProject() error
	Cleanup() error
	GetWorkspaceDir() string
}

// SubProjectInfo holds information about a sub-project within the multi-project workspace
type SubProjectInfo struct {
	Language     string
	ProjectPath  string
	RootMarkers  []string
	TestFiles    []string
	LSPConfig    map[string]string
}

// MultiProjectConfig provides configuration for the multi-project manager
type MultiProjectConfig struct {
	TargetDir        string
	CloneTimeout     time.Duration
	EnableLogging    bool
	ForceClean       bool
	PreserveGitDir   bool
}

// ConcreteMultiProjectManager implements MultiProjectRepositoryManager
type ConcreteMultiProjectManager struct {
	config       MultiProjectConfig
	workspaceDir string
	subProjects  []*SubProjectInfo
	repoManagers map[string]*GenericRepoManager
	mu           sync.RWMutex
	lastError    error
}

// NewMultiProjectRepositoryManager creates a new multi-project repository manager
func NewMultiProjectRepositoryManager(config MultiProjectConfig) MultiProjectRepositoryManager {
	if config.CloneTimeout == 0 {
		config.CloneTimeout = 300 * time.Second
	}

	if config.TargetDir == "" {
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		config.TargetDir = filepath.Join("/tmp", "lspg-multi-project-e2e-tests", uniqueID)
	}

	return &ConcreteMultiProjectManager{
		config:       config,
		workspaceDir: config.TargetDir,
		subProjects:  []*SubProjectInfo{},
		repoManagers: make(map[string]*GenericRepoManager),
	}
}

// SetupMultiProjectWorkspace creates a workspace with multiple sub-projects of different languages
func (mpm *ConcreteMultiProjectManager) SetupMultiProjectWorkspace(languages []string) (string, error) {
	mpm.mu.Lock()
	defer mpm.mu.Unlock()

	mpm.log("Setting up multi-project workspace with languages: %v", languages)

	if err := mpm.prepareWorkspace(); err != nil {
		mpm.lastError = fmt.Errorf("workspace preparation failed: %w", err)
		return "", mpm.lastError
	}

	// Setup each language project
	for _, language := range languages {
		if err := mpm.setupSubProject(language); err != nil {
			mpm.lastError = fmt.Errorf("failed to setup %s project: %w", language, err)
			return "", mpm.lastError
		}
	}

	mpm.log("Multi-project workspace setup completed: %s", mpm.workspaceDir)
	return mpm.workspaceDir, nil
}

// GetSubProjects returns all sub-projects in the workspace
func (mpm *ConcreteMultiProjectManager) GetSubProjects() []*SubProjectInfo {
	mpm.mu.RLock()
	defer mpm.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]*SubProjectInfo, len(mpm.subProjects))
	copy(result, mpm.subProjects)
	return result
}

// GetSubProjectPath returns the path for a specific project type
func (mpm *ConcreteMultiProjectManager) GetSubProjectPath(projectType string) (string, error) {
	mpm.mu.RLock()
	defer mpm.mu.RUnlock()

	for _, subProject := range mpm.subProjects {
		if subProject.Language == projectType {
			return subProject.ProjectPath, nil
		}
	}

	return "", fmt.Errorf("project type %s not found in workspace", projectType)
}

// ValidateMultiProject validates all sub-projects in the workspace
func (mpm *ConcreteMultiProjectManager) ValidateMultiProject() error {
	mpm.mu.RLock()
	defer mpm.mu.RUnlock()

	if len(mpm.subProjects) == 0 {
		return fmt.Errorf("no sub-projects found in workspace")
	}

	// Validate each sub-project
	for _, subProject := range mpm.subProjects {
		repoManager, exists := mpm.repoManagers[subProject.Language]
		if !exists {
			return fmt.Errorf("repository manager not found for language: %s", subProject.Language)
		}

		if err := repoManager.ValidateRepository(); err != nil {
			return fmt.Errorf("validation failed for %s project: %w", subProject.Language, err)
		}
	}

	return nil
}

// Cleanup removes the workspace directory and performs cleanup operations
func (mpm *ConcreteMultiProjectManager) Cleanup() error {
	mpm.mu.Lock()
	defer mpm.mu.Unlock()

	mpm.log("Starting cleanup of multi-project workspace: %s", mpm.workspaceDir)

	// Cleanup individual repository managers
	for language, repoManager := range mpm.repoManagers {
		if err := repoManager.Cleanup(); err != nil {
			mpm.log("Warning: cleanup failed for %s project: %v", language, err)
		}
	}

	// Clear internal state
	mpm.subProjects = []*SubProjectInfo{}
	mpm.repoManagers = make(map[string]*GenericRepoManager)

	if mpm.workspaceDir == "" {
		return nil
	}

	if _, err := os.Stat(mpm.workspaceDir); os.IsNotExist(err) {
		return nil
	}

	err := os.RemoveAll(mpm.workspaceDir)
	if err != nil {
		mpm.log("Cleanup failed: %v", err)
		return fmt.Errorf("failed to remove workspace directory: %w", err)
	}

	mpm.log("Cleanup completed successfully")
	return nil
}

// GetWorkspaceDir returns the workspace directory path
func (mpm *ConcreteMultiProjectManager) GetWorkspaceDir() string {
	mpm.mu.RLock()
	defer mpm.mu.RUnlock()
	return mpm.workspaceDir
}

// Private helper methods

func (mpm *ConcreteMultiProjectManager) log(format string, args ...interface{}) {
	if mpm.config.EnableLogging {
		fmt.Printf("[MultiProjectManager] %s\n", fmt.Sprintf(format, args...))
	}
}

func (mpm *ConcreteMultiProjectManager) prepareWorkspace() error {
	if mpm.config.ForceClean {
		if err := os.RemoveAll(mpm.workspaceDir); err != nil && !os.IsNotExist(err) {
			mpm.log("Warning: failed to clean existing workspace: %v", err)
		}
	}

	return os.MkdirAll(mpm.workspaceDir, 0755)
}

func (mpm *ConcreteMultiProjectManager) setupSubProject(language string) error {
	mpm.log("Setting up %s sub-project", language)

	// Get language configuration
	langConfig, err := mpm.getLanguageConfig(language)
	if err != nil {
		return fmt.Errorf("failed to get language config for %s: %w", language, err)
	}

	// Create sub-project directory
	subProjectDir := mpm.getSubProjectName(language)
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
		RootMarkers: langConfig.RootMarkers,
		TestFiles:   testFiles,
		LSPConfig:   langConfig.CustomVariables,
	}

	// Store the sub-project and repository manager
	mpm.subProjects = append(mpm.subProjects, subProjectInfo)
	mpm.repoManagers[language] = repoManager

	mpm.log("Successfully set up %s sub-project at: %s", language, actualPath)
	return nil
}

func (mpm *ConcreteMultiProjectManager) getLanguageConfig(language string) (LanguageConfig, error) {
	switch strings.ToLower(language) {
	case "go", "golang":
		return GetGoLanguageConfig(), nil
	case "python", "py":
		return GetPythonLanguageConfig(), nil
	case "javascript", "js":
		return GetJavaScriptLanguageConfig(), nil
	case "typescript", "ts":
		return GetTypeScriptLanguageConfig(), nil
	case "java":
		return GetJavaLanguageConfig(), nil
	case "rust", "rs":
		return GetRustLanguageConfig(), nil
	default:
		return LanguageConfig{}, fmt.Errorf("unsupported language: %s", language)
	}
}

func (mpm *ConcreteMultiProjectManager) getSubProjectName(language string) string {
	switch strings.ToLower(language) {
	case "go", "golang":
		return "go-project"
	case "python", "py":
		return "python-project"
	case "javascript", "js":
		return "javascript-project"
	case "typescript", "ts":
		return "typescript-project"
	case "java":
		return "java-project"
	case "rust", "rs":
		return "rust-project"
	default:
		return fmt.Sprintf("%s-project", language)
	}
}