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

// PythonRepoConfig defines configuration for Python repository management
type PythonRepoConfig struct {
	RepoURL          string
	TargetDir        string
	CloneTimeout     time.Duration
	EnableLogging    bool
	ForceClean       bool
	PreserveGitDir   bool
}

// DefaultPythonRepoConfig returns a default configuration for Python repository management
func DefaultPythonRepoConfig() PythonRepoConfig {
	return PythonRepoConfig{
		RepoURL:          "https://github.com/faif/python-patterns.git",
		TargetDir:        "",
		CloneTimeout:     300 * time.Second, // 5 minutes for network operations
		EnableLogging:    true,
		ForceClean:       false,
		PreserveGitDir:   false,
	}
}

// PythonRepoManager provides comprehensive Git repository management for Python testing
type PythonRepoManager struct {
	RepoURL      string
	CommitHash   string
	WorkspaceDir string
	CleanupFunc  func() error
	config       PythonRepoConfig
	mu           sync.RWMutex
	errorConfig  ErrorRecoveryConfig
	healthChecker *HealthChecker
	lastError    *PythonRepoError
}

// NewPythonRepoManager creates a new PythonRepoManager with the given configuration
func NewPythonRepoManager(config PythonRepoConfig) *PythonRepoManager {
	// Set defaults if not provided
	if config.RepoURL == "" {
		config.RepoURL = "https://github.com/faif/python-patterns.git"
	}
	if config.CloneTimeout == 0 {
		config.CloneTimeout = 300 * time.Second
	}

	// Generate unique workspace directory if not provided
	if config.TargetDir == "" {
		uniqueID := generateUniqueID()
		config.TargetDir = filepath.Join("/tmp", "lspg-python-e2e-tests", uniqueID)
	}

	prm := &PythonRepoManager{
		RepoURL:       config.RepoURL,
		WorkspaceDir:  config.TargetDir,
		config:        config,
		errorConfig:   DefaultErrorRecoveryConfig(),
		healthChecker: NewHealthChecker(config.TargetDir, config.RepoURL),
	}

	// Set cleanup function
	prm.CleanupFunc = prm.Cleanup

	return prm
}

// generateUniqueID creates a unique identifier for workspace directories
func generateUniqueID() string {
	timestamp := time.Now().Unix()
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)
	return fmt.Sprintf("%d-%x", timestamp, randomBytes)
}

// log logs a message if logging is enabled
func (prm *PythonRepoManager) log(format string, args ...interface{}) {
	if prm.config.EnableLogging {
		log.Printf("[PythonRepoManager] "+format, args...)
	}
}

// CloneRepository clones the repository and checks out the latest commit with comprehensive error handling
func (prm *PythonRepoManager) CloneRepository() error {
	prm.mu.Lock()
	defer prm.mu.Unlock()

	prm.log("Starting repository clone process for %s", prm.RepoURL)

	// Validate and prepare workspace
	if err := prm.prepareWorkspace(); err != nil {
		prm.lastError = NewPythonRepoError(ErrorTypeFileSystem, "workspace_preparation", err)
		LogErrorWithContext(prm.lastError, CreateErrorContext("workspace_preparation", "workspace_dir", prm.WorkspaceDir))
		return prm.lastError
	}

	// Execute clone operation with recovery
	return ExecuteGitOperationWithRecovery("clone_repository", prm.WorkspaceDir, prm.RepoURL, func() error {
		// Get the latest commit hash first
		commitHash, err := prm.getLatestCommitHashWithRetry()
		if err != nil {
			return fmt.Errorf("failed to get latest commit: %w", err)
		}
		prm.CommitHash = commitHash
		prm.log("Target commit hash: %s", prm.CommitHash)

		// Clone repository
		repoDir := filepath.Join(prm.WorkspaceDir, "python-patterns")
		if err := prm.cloneRepoWithRetry(repoDir); err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}

		// Checkout specific commit
		if err := prm.checkoutCommitWithRetry(repoDir, prm.CommitHash); err != nil {
			return fmt.Errorf("failed to checkout commit %s: %w", prm.CommitHash, err)
		}

		// Remove .git directory if not preserving
		if !prm.config.PreserveGitDir {
			gitDir := filepath.Join(repoDir, ".git")
			if err := WrapWithErrorHandling("remove_git_dir", func() error {
				return os.RemoveAll(gitDir)
			}); err != nil {
				prm.log("Warning: failed to remove .git directory: %v", err)
			}
		}

		// Validate repository health
		if err := prm.healthChecker.CheckRepositoryHealth(); err != nil {
			return fmt.Errorf("repository health check failed: %w", err)
		}

		prm.log("Repository clone completed successfully")
		return nil
	})
}

// prepareWorkspace prepares and validates the workspace directory
func (prm *PythonRepoManager) prepareWorkspace() error {
	// Validate workspace directory
	if err := ValidateAndRepairWorkspace(prm.WorkspaceDir); err != nil {
		return fmt.Errorf("workspace validation failed: %w", err)
	}

	// Clean workspace if it exists and ForceClean is enabled
	if prm.config.ForceClean {
		if err := WrapWithErrorHandling("clean_workspace", func() error {
			return prm.cleanWorkspace()
		}); err != nil {
			prm.log("Warning: failed to clean existing workspace: %v", err)
		}
	}

	// Create workspace directory with proper permissions
	return WrapWithErrorHandling("create_workspace", func() error {
		return os.MkdirAll(prm.WorkspaceDir, 0755)
	})
}

// getLatestCommitHashWithRetry gets the latest commit hash from the remote repository with retry logic
func (prm *PythonRepoManager) getLatestCommitHashWithRetry() (string, error) {
	var commitHash string

	err := RetryWithBackoff(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), prm.config.CloneTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, "git", "ls-remote", prm.RepoURL, "HEAD")
		output, cmdErr := cmd.Output()
		if cmdErr != nil {
			if exitErr, ok := cmdErr.(*exec.ExitError); ok {
				return fmt.Errorf("git ls-remote failed with exit code %d: %s", 
					exitErr.ExitCode(), string(exitErr.Stderr))
			}
			return fmt.Errorf("git ls-remote execution failed: %w", cmdErr)
		}

		// Parse output: "commit_hash\tHEAD"
		parts := strings.Fields(string(output))
		if len(parts) < 1 {
			return fmt.Errorf("invalid git ls-remote output: %s", string(output))
		}

		commitHash = strings.TrimSpace(parts[0])
		if len(commitHash) != 40 {
			return fmt.Errorf("invalid commit hash format: %s", commitHash)
		}

		return nil
	}, prm.errorConfig)

	return commitHash, err
}

// getLatestCommitHash gets the latest commit hash from the remote repository (backward compatibility)
func (prm *PythonRepoManager) getLatestCommitHash() (string, error) {
	return prm.getLatestCommitHashWithRetry()
}

// GetLatestCommit returns the latest commit hash (public interface) with error handling
func (prm *PythonRepoManager) GetLatestCommit() (string, error) {
	prm.mu.RLock()
	defer prm.mu.RUnlock()

	if prm.CommitHash != "" {
		return prm.CommitHash, nil
	}

	var commitHash string
	err := WrapWithErrorHandling("get_latest_commit", func() error {
		var err error
		commitHash, err = prm.getLatestCommitHashWithRetry()
		if err != nil {
			return err
		}
		prm.CommitHash = commitHash
		return nil
	})

	return commitHash, err
}

// cloneRepoWithRetry performs the actual git clone operation with retry logic
func (prm *PythonRepoManager) cloneRepoWithRetry(targetDir string) error {
	// Remove existing directory if it exists to avoid conflicts
	if _, err := os.Stat(targetDir); err == nil {
		if removeErr := os.RemoveAll(targetDir); removeErr != nil {
			return fmt.Errorf("failed to remove existing directory %s: %w", targetDir, removeErr)
		}
	}

	return RetryWithBackoff(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), prm.config.CloneTimeout)
		defer cancel()

		// Use shallow clone for performance
		cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", prm.RepoURL, targetDir)
		output, cmdErr := cmd.CombinedOutput()
		if cmdErr != nil {
			// Clean up failed clone attempt
			os.RemoveAll(targetDir)
			
			if exitErr, ok := cmdErr.(*exec.ExitError); ok {
				return fmt.Errorf("git clone failed with exit code %d: %s", 
					exitErr.ExitCode(), string(output))
			}
			return fmt.Errorf("git clone execution failed: %w", cmdErr)
		}

		prm.log("Git clone completed: %s", targetDir)
		return nil
	}, prm.errorConfig)
}

// cloneRepo performs the actual git clone operation (backward compatibility)
func (prm *PythonRepoManager) cloneRepo(targetDir string) error {
	return prm.cloneRepoWithRetry(targetDir)
}

// checkoutCommitWithRetry checks out a specific commit with retry logic
func (prm *PythonRepoManager) checkoutCommitWithRetry(repoDir, commitHash string) error {
	return RetryWithBackoff(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// First, fetch the specific commit if shallow clone doesn't have it
		fetchCmd := exec.CommandContext(ctx, "git", "fetch", "origin", commitHash)
		fetchCmd.Dir = repoDir
		if output, fetchErr := fetchCmd.CombinedOutput(); fetchErr != nil {
			prm.log("Git fetch warning (may be normal for shallow clone): %s", string(output))
			
			// Try fetching all refs as fallback
			fetchAllCmd := exec.CommandContext(ctx, "git", "fetch", "origin")
			fetchAllCmd.Dir = repoDir
			if fetchAllOutput, fetchAllErr := fetchAllCmd.CombinedOutput(); fetchAllErr != nil {
				prm.log("Git fetch all refs warning: %s", string(fetchAllOutput))
			}
		}

		// Checkout the commit
		cmd := exec.CommandContext(ctx, "git", "checkout", commitHash)
		cmd.Dir = repoDir
		output, cmdErr := cmd.CombinedOutput()
		if cmdErr != nil {
			if exitErr, ok := cmdErr.(*exec.ExitError); ok {
				return fmt.Errorf("git checkout failed with exit code %d: %s", 
					exitErr.ExitCode(), string(output))
			}
			return fmt.Errorf("git checkout execution failed: %w", cmdErr)
		}

		prm.log("Git checkout completed: %s", commitHash)
		return nil
	}, prm.errorConfig)
}

// checkoutCommit checks out a specific commit (backward compatibility)
func (prm *PythonRepoManager) checkoutCommit(repoDir, commitHash string) error {
	return prm.checkoutCommitWithRetry(repoDir, commitHash)
}

// GetTestFiles returns a list of Python files in the patterns directory
func (prm *PythonRepoManager) GetTestFiles() ([]string, error) {
	prm.mu.RLock()
	defer prm.mu.RUnlock()

	repoDir := filepath.Join(prm.WorkspaceDir, "python-patterns")
	patternsDir := filepath.Join(repoDir, "patterns")

	// Check if patterns directory exists
	if _, err := os.Stat(patternsDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("patterns directory not found: %s", patternsDir)
	}

	var pythonFiles []string

	// Walk through patterns directory and subdirectories
	err := filepath.Walk(patternsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-Python files
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".py") {
			return nil
		}

		// Skip __pycache__ and other internal files
		if strings.Contains(path, "__pycache__") || strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Get relative path from workspace root
		relPath, err := filepath.Rel(prm.WorkspaceDir, path)
		if err != nil {
			return err
		}

		pythonFiles = append(pythonFiles, relPath)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk patterns directory: %w", err)
	}

	prm.log("Found %d Python files in patterns directory", len(pythonFiles))
	return pythonFiles, nil
}

// GetWorkspaceDir returns the workspace directory path
func (prm *PythonRepoManager) GetWorkspaceDir() string {
	prm.mu.RLock()
	defer prm.mu.RUnlock()
	return prm.WorkspaceDir
}

// cleanWorkspace removes the existing workspace directory
func (prm *PythonRepoManager) cleanWorkspace() error {
	if prm.WorkspaceDir == "" {
		return nil
	}

	if _, err := os.Stat(prm.WorkspaceDir); os.IsNotExist(err) {
		return nil
	}

	return os.RemoveAll(prm.WorkspaceDir)
}

// Cleanup removes the workspace directory and performs cleanup operations with error recovery
func (prm *PythonRepoManager) Cleanup() error {
	prm.mu.Lock()
	defer prm.mu.Unlock()

	prm.log("Starting cleanup of workspace: %s", prm.WorkspaceDir)

	// Try normal cleanup first
	if err := WrapWithErrorHandling("normal_cleanup", func() error {
		return prm.cleanWorkspace()
	}); err != nil {
		prm.log("Normal cleanup failed, attempting emergency cleanup: %v", err)
		
		// Attempt emergency cleanup
		if emergencyErr := EmergencyCleanup(prm.WorkspaceDir); emergencyErr != nil {
			return fmt.Errorf("both normal and emergency cleanup failed - normal: %w, emergency: %v", err, emergencyErr)
		}
		
		prm.log("Emergency cleanup completed successfully")
		return nil
	}

	prm.log("Cleanup completed successfully")
	return nil
}

// GetLastError returns the last error encountered during operations
func (prm *PythonRepoManager) GetLastError() *PythonRepoError {
	prm.mu.RLock()
	defer prm.mu.RUnlock()
	return prm.lastError
}

// ValidateRepository validates the repository structure and health
func (prm *PythonRepoManager) ValidateRepository() error {
	prm.mu.RLock()
	defer prm.mu.RUnlock()

	if !prm.isInitialized() {
		return fmt.Errorf("repository manager not initialized")
	}

	return WrapWithErrorHandling("validate_repository", func() error {
		return prm.healthChecker.CheckRepositoryHealth()
	})
}

// isInitialized checks if the repository manager has been properly initialized
func (prm *PythonRepoManager) isInitialized() bool {
	return prm.WorkspaceDir != "" && prm.RepoURL != ""
}

// CheckNetworkConnectivity validates network connectivity to the repository
func (prm *PythonRepoManager) CheckNetworkConnectivity() error {
	return WrapWithErrorHandling("check_network", func() error {
		return prm.healthChecker.CheckNetworkConnectivity()
	})
}

// ForceCleanup performs an emergency cleanup operation
func (prm *PythonRepoManager) ForceCleanup() error {
	prm.mu.Lock()
	defer prm.mu.Unlock()

	prm.log("Starting force cleanup of workspace: %s", prm.WorkspaceDir)
	
	if err := EmergencyCleanup(prm.WorkspaceDir); err != nil {
		prm.lastError = NewPythonRepoError(ErrorTypeFileSystem, "force_cleanup", err)
		LogErrorWithContext(prm.lastError, CreateErrorContext("force_cleanup", "workspace_dir", prm.WorkspaceDir))
		return prm.lastError
	}

	prm.log("Force cleanup completed successfully")
	return nil
}

// RecoverFromError attempts to recover from the last encountered error
func (prm *PythonRepoManager) RecoverFromError() error {
	prm.mu.Lock()
	defer prm.mu.Unlock()

	if prm.lastError == nil {
		return fmt.Errorf("no error to recover from")
	}

	config := DefaultErrorRecoveryConfig()
	strategy, exists := config.FallbackStrategies[prm.lastError.Type]
	if !exists {
		return fmt.Errorf("no recovery strategy available for error type: %s", prm.lastError.Type)
	}

	if !strategy.CanRecover(prm.lastError) {
		return fmt.Errorf("error is not recoverable: %s", prm.lastError.Message)
	}

	prm.log("Attempting recovery from error: %s", prm.lastError.Message)
	
	if err := strategy.Recover(prm.lastError); err != nil {
		return fmt.Errorf("recovery failed: %w", err)
	}

	prm.log("Recovery completed successfully")
	prm.lastError = nil
	return nil
}