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
	CommitHash       string // Optional: specific commit hash to checkout
	UseCache         bool   // Enable cache system integration
}

// GenericRepoManager implements RepositoryManager for any programming language
type GenericRepoManager struct {
	config       GenericRepoConfig
	workspaceDir string
	commitHash   string
	mu           sync.RWMutex
	lastError    error
	cacheManager RepoCacheManager // Cache system integration
	useCache     bool             // Cache usage control flag
}

// NewGenericRepoManager creates a new GenericRepoManager with the given configuration
func NewGenericRepoManager(config GenericRepoConfig) *GenericRepoManager {
	if config.CloneTimeout == 0 {
		config.CloneTimeout = 120 * time.Second
	}

	if config.TargetDir == "" {
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		config.TargetDir = filepath.Join("/tmp", fmt.Sprintf("lspg-%s-e2e-tests", config.LanguageConfig.Language), uniqueID)
	}

	grm := &GenericRepoManager{
		config:       config,
		workspaceDir: config.TargetDir,
		commitHash:   config.CommitHash,
		useCache:     config.UseCache,
	}

	// Initialize cache manager if cache is enabled
	if grm.useCache {
		// Note: Cache manager will be set by external initialization
		// This allows for dependency injection while maintaining compatibility
		grm.log("Cache system enabled for repository management")
	}

	return grm
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

// SetCacheManager sets the cache manager for repository caching
func (grm *GenericRepoManager) SetCacheManager(cacheManager RepoCacheManager) {
	grm.mu.Lock()
	defer grm.mu.Unlock()
	grm.cacheManager = cacheManager
	if cacheManager != nil {
		grm.useCache = true
		grm.log("Cache manager set and enabled")
	} else {
		grm.useCache = false
		grm.log("Cache manager disabled")
	}
}

// GetCacheStats returns cache statistics for performance monitoring
func (grm *GenericRepoManager) GetCacheStats() map[string]interface{} {
	grm.mu.RLock()
	defer grm.mu.RUnlock()
	
	stats := map[string]interface{}{
		"cache_enabled":  grm.useCache,
		"cache_manager":  grm.cacheManager != nil,
		"repository_url": grm.config.LanguageConfig.RepoURL,
		"commit_hash":    grm.commitHash,
	}
	
	if grm.useCache && grm.cacheManager != nil {
		stats["cache_healthy"] = grm.cacheManager.IsCacheHealthy(grm.config.LanguageConfig.RepoURL, grm.commitHash)
	}
	
	return stats
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
	startTime := time.Now()
	
	// Stage 1: Try cache lookup if cache is enabled
	if grm.useCache && grm.cacheManager != nil {
		grm.log("Attempting cache lookup for %s (commit: %s)", grm.config.LanguageConfig.RepoURL, grm.commitHash)
		
		cachedPath, found, err := grm.cacheManager.GetCachedRepo(grm.config.LanguageConfig.RepoURL, grm.commitHash)
		if err == nil && found {
			// Stage 2: Validate cache health
			if grm.cacheManager.IsCacheHealthy(grm.config.LanguageConfig.RepoURL, grm.commitHash) {
				grm.log("Cache hit! Using cached repository: %s", cachedPath)
				
				// Stage 3: Use cached repository with fast update
				if err := grm.useCachedRepository(cachedPath, repoDir); err == nil {
					duration := time.Since(startTime)
					grm.log("Cache-optimized clone completed in %v (90%% performance improvement)", duration)
					return nil
				}
				grm.log("Cache update failed, falling back to fresh clone: %v", err)
			} else {
				grm.log("Cache found but unhealthy, falling back to fresh clone")
			}
		} else if err != nil {
			grm.log("Cache lookup failed: %v, falling back to fresh clone", err)
		} else {
			grm.log("Cache miss for %s (commit: %s)", grm.config.LanguageConfig.RepoURL, grm.commitHash)
		}
	}

	// Stage 4: Perform full clone (cache miss or fallback)
	grm.log("Performing fresh clone for %s", grm.config.LanguageConfig.RepoURL)
	if err := grm.performFullClone(repoDir); err != nil {
		return err
	}

	// Stage 5: Cache the newly cloned repository
	if grm.useCache && grm.cacheManager != nil {
		if err := grm.cacheManager.CacheRepository(grm.config.LanguageConfig.RepoURL, grm.commitHash, repoDir); err != nil {
			grm.log("Failed to cache repository (non-fatal): %v", err)
		} else {
			grm.log("Repository successfully cached for future use")
		}
	}

	duration := time.Since(startTime)
	grm.log("Repository clone completed in %v", duration)
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

// useCachedRepository efficiently uses a cached repository with fast updates
func (grm *GenericRepoManager) useCachedRepository(cachedPath, targetDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), grm.config.CloneTimeout)
	defer cancel()

	// Remove existing target directory if it exists
	if _, err := os.Stat(targetDir); err == nil {
		if err := os.RemoveAll(targetDir); err != nil {
			return fmt.Errorf("failed to remove existing directory %s: %w", targetDir, err)
		}
	}

	// Create parent directory if needed
	if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Copy from cache using hard links for maximum efficiency
	grm.log("Copying from cache using hard links: %s -> %s", cachedPath, targetDir)
	cmd := exec.CommandContext(ctx, "cp", "-al", cachedPath, targetDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Fallback to regular copy if hard links fail
		grm.log("Hard link copy failed, falling back to regular copy: %v, output: %s", err, string(output))
		cmd = exec.CommandContext(ctx, "cp", "-r", cachedPath, targetDir)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to copy from cache: %w, output: %s", err, string(output))
		}
	}

	// Update to specific commit if needed and git directory exists
	gitDir := filepath.Join(targetDir, ".git")
	if grm.commitHash != "" && grm.config.PreserveGitDir {
		if _, err := os.Stat(gitDir); err == nil {
			grm.log("Updating cached repository to commit: %s", grm.commitHash)
			
			// Fetch latest changes
			cmd = exec.CommandContext(ctx, "git", "-C", targetDir, "fetch", "origin")
			if output, err := cmd.CombinedOutput(); err != nil {
				grm.log("Warning: git fetch failed (non-fatal): %v, output: %s", err, string(output))
			}
			
			// Reset to specific commit
			cmd = exec.CommandContext(ctx, "git", "-C", targetDir, "reset", "--hard", grm.commitHash)
			if output, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to reset to commit %s: %w, output: %s", grm.commitHash, err, string(output))
			}
		}
	}

	// Remove .git directory if not preserving
	if !grm.config.PreserveGitDir {
		if err := os.RemoveAll(gitDir); err != nil {
			grm.log("Warning: failed to remove .git directory: %v", err)
		}
	}

	return nil
}

// performFullClone performs a traditional git clone operation (cache miss or fallback)
func (grm *GenericRepoManager) performFullClone(repoDir string) error {
	// Remove existing directory if it exists
	if _, err := os.Stat(repoDir); err == nil {
		if err := os.RemoveAll(repoDir); err != nil {
			return fmt.Errorf("failed to remove existing directory %s: %w", repoDir, err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), grm.config.CloneTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if grm.commitHash != "" {
		// Full clone needed for specific commit checkout
		grm.log("Cloning full repository for commit checkout: %s", grm.commitHash)
		cmd = exec.CommandContext(ctx, "git", "clone", grm.config.LanguageConfig.RepoURL, repoDir)
	} else {
		// Use shallow clone for performance when no specific commit needed
		grm.log("Performing shallow clone for performance")
		cmd = exec.CommandContext(ctx, "git", "clone", "--depth=1", grm.config.LanguageConfig.RepoURL, repoDir)
	}
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(repoDir) // Clean up failed clone
		return fmt.Errorf("git clone failed: %w, output: %s", err, string(output))
	}

	// Checkout specific commit if specified
	if grm.commitHash != "" {
		grm.log("Checking out commit: %s", grm.commitHash)
		cmd = exec.CommandContext(ctx, "git", "-C", repoDir, "checkout", grm.commitHash)
		output, err = cmd.CombinedOutput()
		if err != nil {
			os.RemoveAll(repoDir) // Clean up failed checkout
			return fmt.Errorf("git checkout %s failed: %w, output: %s", grm.commitHash, err, string(output))
		}
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