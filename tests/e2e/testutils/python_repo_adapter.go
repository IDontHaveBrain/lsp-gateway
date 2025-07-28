package testutils

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// PythonRepoManagerAdapter provides backward compatibility for existing Python tests
// It wraps the new GenericRepoManager while maintaining the old PythonRepoManager interface
type PythonRepoManagerAdapter struct {
	genericManager *GenericRepoManager
	config         PythonRepoConfig
}

// NewPythonRepoManagerAdapter creates a new adapter that wraps GenericRepoManager
// This allows existing Python tests to work without changes
func NewPythonRepoManagerAdapter(config PythonRepoConfig) *PythonRepoManagerAdapter {
	// Convert old config to new generic config
	genericConfig := GenericRepoConfig{
		LanguageConfig: GetPythonLanguageConfig(),
		TargetDir:      config.TargetDir,
		CloneTimeout:   config.CloneTimeout,
		EnableLogging:  config.EnableLogging,
		ForceClean:     config.ForceClean,
		PreserveGitDir: config.PreserveGitDir,
	}
	
	// Override with custom repo URL if provided
	if config.RepoURL != "" {
		genericConfig.LanguageConfig.RepoURL = config.RepoURL
	}

	return &PythonRepoManagerAdapter{
		genericManager: NewGenericRepoManager(genericConfig),
		config:         config,
	}
}

// SetupRepository sets up the Python repository (compatibility method)
func (prma *PythonRepoManagerAdapter) SetupRepository() (string, error) {
	return prma.genericManager.SetupRepository()
}

// CloneRepository clones the repository (compatibility method)
func (prma *PythonRepoManagerAdapter) CloneRepository() error {
	_, err := prma.genericManager.SetupRepository()
	return err
}

// GetTestFiles returns Python test files (compatibility method)
func (prma *PythonRepoManagerAdapter) GetTestFiles() ([]string, error) {
	return prma.genericManager.GetTestFiles()
}

// GetWorkspaceDir returns the workspace directory (compatibility method)
func (prma *PythonRepoManagerAdapter) GetWorkspaceDir() string {
	return prma.genericManager.GetWorkspaceDir()
}

// Cleanup performs cleanup (compatibility method)
func (prma *PythonRepoManagerAdapter) Cleanup() error {
	return prma.genericManager.Cleanup()
}

// ValidateRepository validates the repository (compatibility method)
func (prma *PythonRepoManagerAdapter) ValidateRepository() error {
	return prma.genericManager.ValidateRepository()
}

// GetLastError returns the last error (compatibility method)
func (prma *PythonRepoManagerAdapter) GetLastError() *PythonRepoError {
	err := prma.genericManager.GetLastError()
	if err == nil {
		return nil
	}
	
	// Convert generic error to PythonRepoError for compatibility
	return NewPythonRepoError(ErrorTypeGeneral, "repository_operation", err)
}

// Legacy methods that maintain exact compatibility with old PythonRepoManager

// GetLatestCommit returns the latest commit hash (compatibility method)
func (prma *PythonRepoManagerAdapter) GetLatestCommit() (string, error) {
	// For backward compatibility, we can try to get the commit hash from the repo
	repoPath := filepath.Join(prma.GetWorkspaceDir(), "python-patterns")
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get commit hash: %w", err)
	}
	
	return strings.TrimSpace(string(output)), nil
}

// CheckNetworkConnectivity validates network connectivity (compatibility method)
func (prma *PythonRepoManagerAdapter) CheckNetworkConnectivity() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, "git", "ls-remote", prma.config.RepoURL, "HEAD")
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("network connectivity check failed: %w", err)
	}
	
	return nil
}

// ForceCleanup performs force cleanup (compatibility method)
func (prma *PythonRepoManagerAdapter) ForceCleanup() error {
	return prma.genericManager.Cleanup()
}

// RecoverFromError attempts error recovery (compatibility method)
func (prma *PythonRepoManagerAdapter) RecoverFromError() error {
	// For backward compatibility, we'll implement a simple recovery mechanism
	lastErr := prma.genericManager.GetLastError()
	if lastErr == nil {
		return fmt.Errorf("no error to recover from")
	}
	
	// Try to re-setup the repository as a recovery mechanism
	_, err := prma.genericManager.SetupRepository()
	return err
}

// Factory function that maintains backward compatibility

// NewPythonRepoManagerWithAdapter creates a new Python repository manager using the adapter
// This function can be used as a drop-in replacement for the old NewPythonRepoManager
func NewPythonRepoManagerWithAdapter(config PythonRepoConfig) *PythonRepoManagerAdapter {
	return NewPythonRepoManagerAdapter(config)
}

// Migration helper functions

// MigratePythonRepoManagerToGeneric helps migrate existing code to use the new generic system
func MigratePythonRepoManagerToGeneric(oldConfig PythonRepoConfig) *GenericRepoManager {
	genericConfig := GenericRepoConfig{
		LanguageConfig: GetPythonLanguageConfig(),
		TargetDir:      oldConfig.TargetDir,
		CloneTimeout:   oldConfig.CloneTimeout,
		EnableLogging:  oldConfig.EnableLogging,
		ForceClean:     oldConfig.ForceClean,
		PreserveGitDir: oldConfig.PreserveGitDir,
	}
	
	if oldConfig.RepoURL != "" {
		genericConfig.LanguageConfig.RepoURL = oldConfig.RepoURL
	}
	
	return NewGenericRepoManager(genericConfig)
}

// CreateGenericPythonConfig creates a generic config from Python-specific parameters
func CreateGenericPythonConfig(repoURL, targetDir string, enableLogging bool) GenericRepoConfig {
	config := GenericRepoConfig{
		LanguageConfig: GetPythonLanguageConfig(),
		TargetDir:      targetDir,
		CloneTimeout:   300 * time.Second,
		EnableLogging:  enableLogging,
		ForceClean:     false,
		PreserveGitDir: false,
	}
	
	if repoURL != "" {
		config.LanguageConfig.RepoURL = repoURL
	}
	
	return config
}