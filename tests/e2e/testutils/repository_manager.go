package testutils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// RepositoryManager provides a generic interface for managing test repositories across different languages
type RepositoryManager interface {
	SetupRepository() (string, error)
	GetTestFiles() ([]string, error)
	GetWorkspaceDir() string
	Cleanup() error
	ValidateRepository() error
	GetLastError() error
}

// LanguageConfig holds language-specific configuration for repository management
type LanguageConfig struct {
	Language        string            // e.g., "python", "go", "javascript", "java"
	RepoURL         string            // Git repository URL
	TestPaths       []string          // Paths to search for test files (relative to repo root)
	FilePatterns    []string          // File patterns to match (e.g., "*.py", "*.go")
	RootMarkers     []string          // Project root markers (e.g., "go.mod", "package.json")
	ExcludePaths    []string          // Paths to exclude (e.g., "__pycache__", "node_modules")
	RepoSubDir      string            // Subdirectory within cloned repo (if needed)
	CustomVariables map[string]string // Additional custom variables for configuration
}

// GenericRepoConfig provides configuration for the generic repository manager
type GenericRepoConfig struct {
	LanguageConfig   LanguageConfig
	TargetDir        string
	CloneTimeout     time.Duration
	EnableLogging    bool
	ForceClean       bool
	PreserveGitDir   bool
}

// GenericRepoManager implements RepositoryManager for any programming language
type GenericRepoManager struct {
	config       GenericRepoConfig
	workspaceDir string
	commitHash   string
	mu           sync.RWMutex
	lastError    error
}

// NewGenericRepoManager creates a new GenericRepoManager with the given configuration
func NewGenericRepoManager(config GenericRepoConfig) *GenericRepoManager {
	if config.CloneTimeout == 0 {
		config.CloneTimeout = 300 * time.Second
	}

	if config.TargetDir == "" {
		uniqueID := generateUniqueID()
		config.TargetDir = filepath.Join("/tmp", fmt.Sprintf("lspg-%s-e2e-tests", config.LanguageConfig.Language), uniqueID)
	}

	return &GenericRepoManager{
		config:       config,
		workspaceDir: config.TargetDir,
	}
}

// SetupRepository clones and prepares the repository for testing
func (grm *GenericRepoManager) SetupRepository() (string, error) {
	grm.mu.Lock()
	defer grm.mu.Unlock()

	grm.log("Setting up %s repository: %s", grm.config.LanguageConfig.Language, grm.config.LanguageConfig.RepoURL)

	if err := grm.prepareWorkspace(); err != nil {
		grm.lastError = fmt.Errorf("workspace preparation failed: %w", err)
		return "", grm.lastError
	}

	if err := grm.cloneRepository(); err != nil {
		grm.lastError = fmt.Errorf("repository clone failed: %w", err)
		return "", grm.lastError
	}

	if err := grm.validateRepositoryStructure(); err != nil {
		grm.lastError = fmt.Errorf("repository validation failed: %w", err)
		return "", grm.lastError
	}

	repoPath := grm.getRepositoryPath()
	grm.log("Repository setup completed: %s", repoPath)
	return repoPath, nil
}

// GetTestFiles returns a list of test files matching the language configuration
func (grm *GenericRepoManager) GetTestFiles() ([]string, error) {
	grm.mu.RLock()
	defer grm.mu.RUnlock()

	repoPath := grm.getRepositoryPath()
	var allFiles []string

	for _, testPath := range grm.config.LanguageConfig.TestPaths {
		searchPath := filepath.Join(repoPath, testPath)
		files, err := grm.findFilesInPath(searchPath)
		if err != nil {
			grm.log("Warning: failed to search path %s: %v", searchPath, err)
			continue
		}
		allFiles = append(allFiles, files...)
	}

	if len(allFiles) == 0 {
		return nil, fmt.Errorf("no test files found for %s in paths: %v", grm.config.LanguageConfig.Language, grm.config.LanguageConfig.TestPaths)
	}

	grm.log("Found %d test files for %s", len(allFiles), grm.config.LanguageConfig.Language)
	return allFiles, nil
}

// GetWorkspaceDir returns the workspace directory path
func (grm *GenericRepoManager) GetWorkspaceDir() string {
	grm.mu.RLock()
	defer grm.mu.RUnlock()
	return grm.workspaceDir
}

// Cleanup removes the workspace directory and performs cleanup operations
func (grm *GenericRepoManager) Cleanup() error {
	grm.mu.Lock()
	defer grm.mu.Unlock()

	grm.log("Starting cleanup of %s workspace: %s", grm.config.LanguageConfig.Language, grm.workspaceDir)

	if grm.workspaceDir == "" {
		return nil
	}

	if _, err := os.Stat(grm.workspaceDir); os.IsNotExist(err) {
		return nil
	}

	err := os.RemoveAll(grm.workspaceDir)
	if err != nil {
		grm.log("Cleanup failed: %v", err)
		return fmt.Errorf("failed to remove workspace directory: %w", err)
	}

	grm.log("Cleanup completed successfully")
	return nil
}

// ValidateRepository validates the repository structure and content
func (grm *GenericRepoManager) ValidateRepository() error {
	grm.mu.RLock()
	defer grm.mu.RUnlock()

	repoPath := grm.getRepositoryPath()
	
	// Check if repository directory exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return fmt.Errorf("repository directory not found: %s", repoPath)
	}

	// Validate root markers if specified
	if len(grm.config.LanguageConfig.RootMarkers) > 0 {
		found := false
		for _, marker := range grm.config.LanguageConfig.RootMarkers {
			markerPath := filepath.Join(repoPath, marker)
			if _, err := os.Stat(markerPath); err == nil {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("none of the expected root markers found: %v", grm.config.LanguageConfig.RootMarkers)
		}
	}

	// Validate test paths exist
	for _, testPath := range grm.config.LanguageConfig.TestPaths {
		fullPath := filepath.Join(repoPath, testPath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return fmt.Errorf("test path not found: %s", testPath)
		}
	}

	return nil
}

// GetLastError returns the last error encountered
func (grm *GenericRepoManager) GetLastError() error {
	grm.mu.RLock()
	defer grm.mu.RUnlock()
	return grm.lastError
}

// Private helper methods

func (grm *GenericRepoManager) log(format string, args ...interface{}) {
	if grm.config.EnableLogging {
		fmt.Printf("[%sRepoManager] %s\n", strings.Title(grm.config.LanguageConfig.Language), fmt.Sprintf(format, args...))
	}
}

func (grm *GenericRepoManager) prepareWorkspace() error {
	if grm.config.ForceClean {
		if err := os.RemoveAll(grm.workspaceDir); err != nil && !os.IsNotExist(err) {
			grm.log("Warning: failed to clean existing workspace: %v", err)
		}
	}

	return os.MkdirAll(grm.workspaceDir, 0755)
}

func (grm *GenericRepoManager) cloneRepository() error {
	repoDir := grm.getRepositoryPath()
	
	// Remove existing directory if it exists
	if _, err := os.Stat(repoDir); err == nil {
		if err := os.RemoveAll(repoDir); err != nil {
			return fmt.Errorf("failed to remove existing directory %s: %w", repoDir, err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), grm.config.CloneTimeout)
	defer cancel()

	// Use shallow clone for performance
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", grm.config.LanguageConfig.RepoURL, repoDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(repoDir) // Clean up failed clone
		return fmt.Errorf("git clone failed: %w, output: %s", err, string(output))
	}

	// Remove .git directory if not preserving
	if !grm.config.PreserveGitDir {
		gitDir := filepath.Join(repoDir, ".git")
		if err := os.RemoveAll(gitDir); err != nil {
			grm.log("Warning: failed to remove .git directory: %v", err)
		}
	}

	return nil
}

func (grm *GenericRepoManager) getRepositoryPath() string {
	if grm.config.LanguageConfig.RepoSubDir != "" {
		return filepath.Join(grm.workspaceDir, grm.config.LanguageConfig.RepoSubDir)
	}
	
	// Extract repo name from URL
	repoName := filepath.Base(grm.config.LanguageConfig.RepoURL)
	if strings.HasSuffix(repoName, ".git") {
		repoName = strings.TrimSuffix(repoName, ".git")
	}
	
	return filepath.Join(grm.workspaceDir, repoName)
}

func (grm *GenericRepoManager) validateRepositoryStructure() error {
	repoPath := grm.getRepositoryPath()
	
	// Check if any test files exist
	testFiles, err := grm.findAllTestFiles(repoPath)
	if err != nil {
		return fmt.Errorf("failed to validate repository structure: %w", err)
	}
	
	if len(testFiles) == 0 {
		return fmt.Errorf("no files matching patterns %v found in repository", grm.config.LanguageConfig.FilePatterns)
	}
	
	return nil
}

func (grm *GenericRepoManager) findFilesInPath(searchPath string) ([]string, error) {
	var files []string
	
	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if info.IsDir() {
			// Skip excluded directories
			for _, excludePath := range grm.config.LanguageConfig.ExcludePaths {
				if strings.Contains(path, excludePath) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		
		// Check if file matches any pattern
		for _, pattern := range grm.config.LanguageConfig.FilePatterns {
			matched, err := filepath.Match(pattern, info.Name())
			if err != nil {
				continue
			}
			if matched {
				// Get relative path from workspace root
				relPath, err := filepath.Rel(grm.workspaceDir, path)
				if err != nil {
					return err
				}
				files = append(files, relPath)
				break
			}
		}
		
		return nil
	})
	
	return files, err
}

func (grm *GenericRepoManager) findAllTestFiles(repoPath string) ([]string, error) {
	var allFiles []string
	
	for _, testPath := range grm.config.LanguageConfig.TestPaths {
		searchPath := filepath.Join(repoPath, testPath)
		if _, err := os.Stat(searchPath); os.IsNotExist(err) {
			continue // Skip non-existent paths
		}
		
		files, err := grm.findFilesInPath(searchPath)
		if err != nil {
			return nil, err
		}
		allFiles = append(allFiles, files...)
	}
	
	return allFiles, nil
}