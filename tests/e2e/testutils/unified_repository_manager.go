package testutils

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// UnifiedRepositoryManager provides comprehensive repository management with advanced features
// This consolidates the functionality from GenericRepoManager, PythonRepoManager, and their adapters
type UnifiedRepositoryManager struct {
	config           UnifiedRepoConfig
	workspaceDir     string
	commitHash       string
	cleanupFunc      func() error
	mu               sync.RWMutex
	errorConfig      ErrorRecoveryConfig
	healthChecker    *GenericHealthChecker
	lastError        error
}

// UnifiedRepoConfig combines the best configuration options from all previous implementations
type UnifiedRepoConfig struct {
	LanguageConfig   LanguageConfig
	TargetDir        string
	CloneTimeout     time.Duration
	EnableLogging    bool
	ForceClean       bool
	PreserveGitDir   bool
	EnableRetry      bool
	EnableHealthCheck bool
	CommitTracking   bool
}

// DefaultUnifiedRepoConfig returns a sensible default configuration
func DefaultUnifiedRepoConfig() UnifiedRepoConfig {
	return UnifiedRepoConfig{
		CloneTimeout:      300 * time.Second,
		EnableLogging:     true,
		ForceClean:        false,
		PreserveGitDir:    false,
		EnableRetry:       true,
		EnableHealthCheck: true,
		CommitTracking:    true,
	}
}

// NewUnifiedRepositoryManager creates a new unified repository manager
func NewUnifiedRepositoryManager(config UnifiedRepoConfig) *UnifiedRepositoryManager {
	if config.CloneTimeout == 0 {
		config.CloneTimeout = 300 * time.Second
	}

	if config.TargetDir == "" {
		uniqueID := generateUniqueID()
		config.TargetDir = filepath.Join("/tmp", fmt.Sprintf("lspg-%s-e2e-tests", config.LanguageConfig.Language), uniqueID)
	}

	urm := &UnifiedRepositoryManager{
		config:       config,
		workspaceDir: config.TargetDir,
		errorConfig:  DefaultErrorRecoveryConfig(),
	}

	if config.EnableHealthCheck {
		urm.healthChecker = NewGenericHealthChecker(config.TargetDir, config.LanguageConfig.RepoURL, config.LanguageConfig)
	}

	urm.cleanupFunc = urm.Cleanup
	return urm
}

// SetupRepository clones and prepares the repository with comprehensive error handling
func (urm *UnifiedRepositoryManager) SetupRepository() (string, error) {
	urm.mu.Lock()
	defer urm.mu.Unlock()

	urm.log("Setting up %s repository: %s", urm.config.LanguageConfig.Language, urm.config.LanguageConfig.RepoURL)

	if err := urm.prepareWorkspace(); err != nil {
		urm.lastError = fmt.Errorf("workspace preparation failed: %w", err)
		return "", urm.lastError
	}

	if urm.config.EnableRetry {
		return urm.setupWithRetry()
	}
	return urm.setupBasic()
}

// setupWithRetry performs repository setup with advanced retry logic
func (urm *UnifiedRepositoryManager) setupWithRetry() (string, error) {
	return urm.executeWithRetry("setup_repository", func() (string, error) {
		if urm.config.CommitTracking {
			commitHash, err := urm.getLatestCommitHashWithRetry()
			if err != nil {
				urm.log("Warning: failed to get latest commit hash: %v", err)
			} else {
				urm.commitHash = commitHash
				urm.log("Target commit hash: %s", urm.commitHash)
			}
		}

		if err := urm.cloneRepository(); err != nil {
			return "", fmt.Errorf("repository clone failed: %w", err)
		}

		if urm.config.CommitTracking && urm.commitHash != "" {
			repoPath := urm.getRepositoryPath()
			if err := urm.checkoutCommitWithRetry(repoPath, urm.commitHash); err != nil {
				urm.log("Warning: failed to checkout specific commit: %v", err)
			}
		}

		if !urm.config.PreserveGitDir {
			urm.removeGitDirectory()
		}

		if urm.config.EnableHealthCheck {
			if err := urm.healthChecker.CheckRepositoryHealth(); err != nil {
				return "", fmt.Errorf("repository health check failed: %w", err)
			}
		}

		repoPath := urm.getRepositoryPath()
		urm.log("Repository setup completed: %s", repoPath)
		return repoPath, nil
	})
}

// setupBasic performs basic repository setup without retry logic
func (urm *UnifiedRepositoryManager) setupBasic() (string, error) {
	if err := urm.cloneRepository(); err != nil {
		urm.lastError = fmt.Errorf("repository clone failed: %w", err)
		return "", urm.lastError
	}

	if !urm.config.PreserveGitDir {
		urm.removeGitDirectory()
	}

	if urm.config.EnableHealthCheck {
		if err := urm.healthChecker.CheckRepositoryHealth(); err != nil {
			urm.lastError = fmt.Errorf("repository health check failed: %w", err)
			return "", urm.lastError
		}
	}

	repoPath := urm.getRepositoryPath()
	urm.log("Repository setup completed: %s", repoPath)
	return repoPath, nil
}

// GetTestFiles returns a list of test files matching the language configuration
func (urm *UnifiedRepositoryManager) GetTestFiles() ([]string, error) {
	urm.mu.RLock()
	defer urm.mu.RUnlock()

	repoPath := urm.getRepositoryPath()
	var allFiles []string

	for _, testPath := range urm.config.LanguageConfig.TestPaths {
		searchPath := filepath.Join(repoPath, testPath)
		files, err := urm.findFilesInPath(searchPath)
		if err != nil {
			urm.log("Warning: failed to search path %s: %v", searchPath, err)
			continue
		}
		allFiles = append(allFiles, files...)
	}

	if len(allFiles) == 0 {
		return nil, fmt.Errorf("no test files found for %s in paths: %v", 
			urm.config.LanguageConfig.Language, urm.config.LanguageConfig.TestPaths)
	}

	urm.log("Found %d test files for %s", len(allFiles), urm.config.LanguageConfig.Language)
	return allFiles, nil
}

// GetWorkspaceDir returns the workspace directory path
func (urm *UnifiedRepositoryManager) GetWorkspaceDir() string {
	urm.mu.RLock()
	defer urm.mu.RUnlock()
	return urm.workspaceDir
}

// Cleanup removes the workspace directory with comprehensive error handling
func (urm *UnifiedRepositoryManager) Cleanup() error {
	urm.mu.Lock()
	defer urm.mu.Unlock()

	urm.log("Starting cleanup of %s workspace: %s", urm.config.LanguageConfig.Language, urm.workspaceDir)

	if urm.workspaceDir == "" {
		return nil
	}

	if _, err := os.Stat(urm.workspaceDir); os.IsNotExist(err) {
		return nil
	}

	if urm.config.EnableRetry {
		return urm.cleanupWithRetry()
	}

	err := os.RemoveAll(urm.workspaceDir)
	if err != nil {
		urm.log("Cleanup failed: %v", err)
		return fmt.Errorf("failed to remove workspace directory: %w", err)
	}

	urm.log("Cleanup completed successfully")
	return nil
}

// cleanupWithRetry performs cleanup with retry logic and emergency fallback
func (urm *UnifiedRepositoryManager) cleanupWithRetry() error {
	err := urm.retryOperation("cleanup", func() error {
		return os.RemoveAll(urm.workspaceDir)
	})

	if err != nil {
		urm.log("Normal cleanup failed, attempting emergency cleanup: %v", err)
		if emergencyErr := urm.emergencyCleanup(); emergencyErr != nil {
			return fmt.Errorf("both normal and emergency cleanup failed - normal: %w, emergency: %v", err, emergencyErr)
		}
		urm.log("Emergency cleanup completed successfully")
		return nil
	}

	urm.log("Cleanup completed successfully")
	return nil
}

// ValidateRepository validates the repository structure and content
func (urm *UnifiedRepositoryManager) ValidateRepository() error {
	urm.mu.RLock()
	defer urm.mu.RUnlock()

	if !urm.isInitialized() {
		return fmt.Errorf("repository manager not initialized")
	}

	if urm.config.EnableHealthCheck && urm.healthChecker != nil {
		return urm.healthChecker.CheckRepositoryHealth()
	}

	return urm.basicValidation()
}

// GetLastError returns the last error encountered
func (urm *UnifiedRepositoryManager) GetLastError() error {
	urm.mu.RLock()
	defer urm.mu.RUnlock()
	return urm.lastError
}

// Advanced methods from PythonRepoManager

// GetLatestCommit returns the latest commit hash (if commit tracking is enabled)
func (urm *UnifiedRepositoryManager) GetLatestCommit() (string, error) {
	urm.mu.RLock()
	defer urm.mu.RUnlock()

	if !urm.config.CommitTracking {
		return "", fmt.Errorf("commit tracking is disabled")
	}

	if urm.commitHash != "" {
		return urm.commitHash, nil
	}

	var commitHash string
	err := urm.retryOperation("get_latest_commit", func() error {
		var err error
		commitHash, err = urm.getLatestCommitHashWithRetry()
		if err != nil {
			return err
		}
		urm.commitHash = commitHash
		return nil
	})

	return commitHash, err
}

// CheckNetworkConnectivity validates network connectivity to the repository
func (urm *UnifiedRepositoryManager) CheckNetworkConnectivity() error {
	if urm.config.EnableHealthCheck && urm.healthChecker != nil {
		return urm.healthChecker.CheckNetworkConnectivity()
	}
	return urm.basicNetworkCheck()
}

// ForceCleanup performs an emergency cleanup operation
func (urm *UnifiedRepositoryManager) ForceCleanup() error {
	urm.mu.Lock()
	defer urm.mu.Unlock()

	urm.log("Starting force cleanup of workspace: %s", urm.workspaceDir)
	
	if err := urm.emergencyCleanup(); err != nil {
		urm.lastError = err
		return err
	}

	urm.log("Force cleanup completed successfully")
	return nil
}

// Private helper methods

func (urm *UnifiedRepositoryManager) log(format string, args ...interface{}) {
	if urm.config.EnableLogging {
		log.Printf("[%sRepoManager] "+format, append([]interface{}{strings.Title(urm.config.LanguageConfig.Language)}, args...)...)
	}
}

func (urm *UnifiedRepositoryManager) prepareWorkspace() error {
	if urm.config.ForceClean {
		if err := os.RemoveAll(urm.workspaceDir); err != nil && !os.IsNotExist(err) {
			urm.log("Warning: failed to clean existing workspace: %v", err)
		}
	}

	return os.MkdirAll(urm.workspaceDir, 0755)
}

func (urm *UnifiedRepositoryManager) cloneRepository() error {
	repoDir := urm.getRepositoryPath()
	
	if _, err := os.Stat(repoDir); err == nil {
		if err := os.RemoveAll(repoDir); err != nil {
			return fmt.Errorf("failed to remove existing directory %s: %w", repoDir, err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), urm.config.CloneTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", urm.config.LanguageConfig.RepoURL, repoDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(repoDir)
		return fmt.Errorf("git clone failed: %w, output: %s", err, string(output))
	}

	return nil
}

func (urm *UnifiedRepositoryManager) getRepositoryPath() string {
	if urm.config.LanguageConfig.RepoSubDir != "" {
		return filepath.Join(urm.workspaceDir, urm.config.LanguageConfig.RepoSubDir)
	}
	
	repoName := filepath.Base(urm.config.LanguageConfig.RepoURL)
	if strings.HasSuffix(repoName, ".git") {
		repoName = strings.TrimSuffix(repoName, ".git")
	}
	
	return filepath.Join(urm.workspaceDir, repoName)
}

func (urm *UnifiedRepositoryManager) removeGitDirectory() {
	repoPath := urm.getRepositoryPath()
	gitDir := filepath.Join(repoPath, ".git")
	if err := os.RemoveAll(gitDir); err != nil {
		urm.log("Warning: failed to remove .git directory: %v", err)
	}
}

func (urm *UnifiedRepositoryManager) findFilesInPath(searchPath string) ([]string, error) {
	var files []string
	
	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if info.IsDir() {
			for _, excludePath := range urm.config.LanguageConfig.ExcludePaths {
				if strings.Contains(path, excludePath) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		
		for _, pattern := range urm.config.LanguageConfig.FilePatterns {
			matched, err := filepath.Match(pattern, info.Name())
			if err != nil {
				continue
			}
			if matched {
				relPath, err := filepath.Rel(urm.workspaceDir, path)
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

func (urm *UnifiedRepositoryManager) isInitialized() bool {
	return urm.workspaceDir != "" && urm.config.LanguageConfig.RepoURL != ""
}

func (urm *UnifiedRepositoryManager) basicValidation() error {
	repoPath := urm.getRepositoryPath()
	
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return fmt.Errorf("repository directory not found: %s", repoPath)
	}

	if len(urm.config.LanguageConfig.RootMarkers) > 0 {
		found := false
		for _, marker := range urm.config.LanguageConfig.RootMarkers {
			markerPath := filepath.Join(repoPath, marker)
			if _, err := os.Stat(markerPath); err == nil {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("none of the expected root markers found: %v", urm.config.LanguageConfig.RootMarkers)
		}
	}

	for _, testPath := range urm.config.LanguageConfig.TestPaths {
		fullPath := filepath.Join(repoPath, testPath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return fmt.Errorf("test path not found: %s", testPath)
		}
	}

	return nil
}

// Advanced retry operations

func (urm *UnifiedRepositoryManager) executeWithRetry(operation string, fn func() (string, error)) (string, error) {
	var result string
	var lastErr error
	
	for attempt := 0; attempt <= urm.errorConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(float64(urm.errorConfig.RetryDelay) * (1 << (attempt - 1)))
			if delay > urm.errorConfig.MaxRetryDelay {
				delay = urm.errorConfig.MaxRetryDelay
			}
			urm.log("Retry attempt %d for %s, waiting %v", attempt, operation, delay)
			time.Sleep(delay)
		}

		result, lastErr = fn()
		if lastErr == nil {
			if attempt > 0 {
				urm.log("Operation %s succeeded after %d attempts", operation, attempt)
			}
			return result, nil
		}

		if !urm.isRetriableError(lastErr) {
			urm.log("Non-retriable error in %s: %v", operation, lastErr)
			break
		}

		urm.log("Attempt %d failed for %s: %v", attempt, operation, lastErr)
	}

	return "", fmt.Errorf("operation %s failed after %d attempts: %w", operation, urm.errorConfig.MaxRetries+1, lastErr)
}

func (urm *UnifiedRepositoryManager) retryOperation(operation string, fn func() error) error {
	_, err := urm.executeWithRetry(operation, func() (string, error) {
		return "", fn()
	})
	return err
}

func (urm *UnifiedRepositoryManager) isRetriableError(err error) bool {
	if err == nil {
		return false
	}

	errorMsg := strings.ToLower(err.Error())
	retriablePatterns := []string{
		"connection refused",
		"timeout",
		"temporary failure",
		"network",
		"resource temporarily unavailable",
		"text file busy",
		"device or resource busy",
	}

	for _, pattern := range retriablePatterns {
		if strings.Contains(errorMsg, pattern) {
			return true
		}
	}

	return false
}

func (urm *UnifiedRepositoryManager) getLatestCommitHashWithRetry() (string, error) {
	var commitHash string

	err := urm.retryOperation("get_latest_commit_hash", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), urm.config.CloneTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, "git", "ls-remote", urm.config.LanguageConfig.RepoURL, "HEAD")
		output, cmdErr := cmd.Output()
		if cmdErr != nil {
			return fmt.Errorf("git ls-remote failed: %w", cmdErr)
		}

		parts := strings.Fields(string(output))
		if len(parts) < 1 {
			return fmt.Errorf("invalid git ls-remote output: %s", string(output))
		}

		commitHash = strings.TrimSpace(parts[0])
		if len(commitHash) != 40 {
			return fmt.Errorf("invalid commit hash format: %s", commitHash)
		}

		return nil
	})

	return commitHash, err
}

func (urm *UnifiedRepositoryManager) checkoutCommitWithRetry(repoDir, commitHash string) error {
	return urm.retryOperation("checkout_commit", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		fetchCmd := exec.CommandContext(ctx, "git", "fetch", "origin", commitHash)
		fetchCmd.Dir = repoDir
		if output, fetchErr := fetchCmd.CombinedOutput(); fetchErr != nil {
			urm.log("Git fetch warning: %s", string(output))
		}

		cmd := exec.CommandContext(ctx, "git", "checkout", commitHash)
		cmd.Dir = repoDir
		output, cmdErr := cmd.CombinedOutput()
		if cmdErr != nil {
			return fmt.Errorf("git checkout failed: %w, output: %s", cmdErr, string(output))
		}

		return nil
	})
}

func (urm *UnifiedRepositoryManager) basicNetworkCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, "git", "ls-remote", urm.config.LanguageConfig.RepoURL, "HEAD")
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("network connectivity check failed: %w", err)
	}
	
	return nil
}

func (urm *UnifiedRepositoryManager) emergencyCleanup() error {
	if urm.workspaceDir == "" {
		return fmt.Errorf("workspace directory is empty")
	}

	languageTest := fmt.Sprintf("lspg-%s-e2e-tests", urm.config.LanguageConfig.Language)
	if !strings.Contains(urm.workspaceDir, languageTest) {
		return fmt.Errorf("refusing to cleanup directory that doesn't appear to be a test workspace: %s", urm.workspaceDir)
	}

	urm.log("Starting emergency cleanup of %s", urm.workspaceDir)

	maxAttempts := 3
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
			urm.log("Emergency cleanup attempt %d/%d", attempt+1, maxAttempts)
		}

		if err := urm.forceRemoveDirectory(urm.workspaceDir); err != nil {
			lastErr = err
			urm.log("Emergency cleanup attempt %d failed: %v", attempt+1, err)
			continue
		}

		urm.log("Emergency cleanup completed successfully")
		return nil
	}

	return fmt.Errorf("emergency cleanup failed after %d attempts: %w", maxAttempts, lastErr)
}

func (urm *UnifiedRepositoryManager) forceRemoveDirectory(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if !info.IsDir() {
			os.Chmod(path, 0666)
		} else {
			os.Chmod(path, 0755)
		}

		return nil
	})

	return os.RemoveAll(dir)
}

// Factory functions for language-specific managers

// NewPythonUnifiedRepositoryManager creates a unified repository manager configured for Python
func NewPythonUnifiedRepositoryManager(customConfig ...UnifiedRepoConfig) *UnifiedRepositoryManager {
	var config UnifiedRepoConfig
	if len(customConfig) > 0 {
		config = customConfig[0]
	} else {
		config = DefaultUnifiedRepoConfig()
	}
	
	config.LanguageConfig = GetPythonLanguageConfig()
	return NewUnifiedRepositoryManager(config)
}

// NewGoUnifiedRepositoryManager creates a unified repository manager configured for Go
func NewGoUnifiedRepositoryManager(customConfig ...UnifiedRepoConfig) *UnifiedRepositoryManager {
	var config UnifiedRepoConfig
	if len(customConfig) > 0 {
		config = customConfig[0]
	} else {
		config = DefaultUnifiedRepoConfig()
	}
	
	config.LanguageConfig = GetGoLanguageConfig()
	return NewUnifiedRepositoryManager(config)
}

// NewJavaScriptUnifiedRepositoryManager creates a unified repository manager configured for JavaScript/TypeScript
func NewJavaScriptUnifiedRepositoryManager(customConfig ...UnifiedRepoConfig) *UnifiedRepositoryManager {
	var config UnifiedRepoConfig
	if len(customConfig) > 0 {
		config = customConfig[0]
	} else {
		config = DefaultUnifiedRepoConfig()
	}
	
	config.LanguageConfig = GetJavaScriptLanguageConfig()
	return NewUnifiedRepositoryManager(config)
}

// NewJavaUnifiedRepositoryManager creates a unified repository manager configured for Java
func NewJavaUnifiedRepositoryManager(customConfig ...UnifiedRepoConfig) *UnifiedRepositoryManager {
	var config UnifiedRepoConfig
	if len(customConfig) > 0 {
		config = customConfig[0]
	} else {
		config = DefaultUnifiedRepoConfig()
	}
	
	config.LanguageConfig = GetJavaLanguageConfig()
	return NewUnifiedRepositoryManager(config)
}

// NewRustUnifiedRepositoryManager creates a unified repository manager configured for Rust
func NewRustUnifiedRepositoryManager(customConfig ...UnifiedRepoConfig) *UnifiedRepositoryManager {
	var config UnifiedRepoConfig
	if len(customConfig) > 0 {
		config = customConfig[0]
	} else {
		config = DefaultUnifiedRepoConfig()
	}
	
	config.LanguageConfig = GetRustLanguageConfig()
	return NewUnifiedRepositoryManager(config)
}