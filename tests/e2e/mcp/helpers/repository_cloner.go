package helpers

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RepositoryCloner handles Git repository cloning operations for MCP E2E testing
type RepositoryCloner struct {
	repositoryURL string
	targetCommit  string
	logger        *log.Logger
}

// CloneResult contains the results of a repository cloning operation
type CloneResult struct {
	WorkspacePath      string
	CommitHash         string
	RepositoryURL      string  
	CloneTime          time.Duration
	ValidationErrors   []string
}

// NewRepositoryCloner creates a new repository cloner instance
func NewRepositoryCloner(url, commit string) *RepositoryCloner {
	return &RepositoryCloner{
		repositoryURL: url,
		targetCommit:  commit,
		logger:        log.New(os.Stdout, "[RepositoryCloner] ", log.LstdFlags),
	}
}

// CloneToWorkspace clones the repository to the specified workspace directory
func (rc *RepositoryCloner) CloneToWorkspace(workspaceDir string) (*CloneResult, error) {
	startTime := time.Now()
	
	result := &CloneResult{
		RepositoryURL:    rc.repositoryURL,
		ValidationErrors: make([]string, 0),
	}

	// Create temporary directory for cloning
	tempCloneDir, err := os.MkdirTemp("", "repo-clone-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tempCloneDir); removeErr != nil {
			rc.logger.Printf("Warning: failed to cleanup temp directory %s: %v", tempCloneDir, removeErr)
		}
	}()

	// Extract repository name for destination
	repoName := rc.extractRepositoryName()
	finalWorkspacePath := filepath.Join(workspaceDir, repoName)

	// Ensure workspace directory exists
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace directory: %w", err)
	}

	// Step 1: Clone repository with retry mechanism
	if err := rc.cloneWithRetry(tempCloneDir, 3); err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Step 2: Checkout specific commit
	if err := rc.checkoutCommit(tempCloneDir); err != nil {
		return nil, fmt.Errorf("failed to checkout commit %s: %w", rc.targetCommit, err)
	}

	// Step 3: Verify commit hash
	actualCommit, err := rc.getCurrentCommitHash(tempCloneDir)
	if err != nil {
		return nil, fmt.Errorf("failed to verify commit hash: %w", err)
	}
	
	if !strings.HasPrefix(actualCommit, rc.targetCommit) {
		return nil, fmt.Errorf("commit mismatch: expected %s, got %s", rc.targetCommit, actualCommit)
	}

	// Step 4: Move to final location
	if err := rc.moveToFinalLocation(tempCloneDir, finalWorkspacePath); err != nil {
		return nil, fmt.Errorf("failed to move repository to workspace: %w", err)
	}

	// Step 5: Validate repository structure
	if err := rc.ValidateRepository(finalWorkspacePath); err != nil {
		result.ValidationErrors = append(result.ValidationErrors, err.Error())
	}

	// Step 6: Verify Go module structure
	if err := rc.VerifyGoModule(finalWorkspacePath); err != nil {
		result.ValidationErrors = append(result.ValidationErrors, err.Error())
	}

	result.WorkspacePath = finalWorkspacePath
	result.CommitHash = actualCommit
	result.CloneTime = time.Since(startTime)

	rc.logger.Printf("Successfully cloned repository %s to %s in %v", rc.repositoryURL, finalWorkspacePath, result.CloneTime)
	
	return result, nil
}

// ValidateRepository validates the basic repository structure
func (rc *RepositoryCloner) ValidateRepository(workspacePath string) error {
	// Check if .git directory exists
	gitDir := filepath.Join(workspacePath, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return fmt.Errorf("invalid repository: .git directory not found")
	}

	// Check for basic Go files (for fatih/color repository)
	expectedFiles := []string{"color.go", "go.mod"}
	for _, file := range expectedFiles {
		filePath := filepath.Join(workspacePath, file)
		if _, err := os.Stat(filePath); err != nil {
			return fmt.Errorf("expected file not found: %s", file)
		}
	}

	return nil
}

// VerifyGoModule verifies the Go module structure and runs go mod tidy
func (rc *RepositoryCloner) VerifyGoModule(workspacePath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check go.mod exists
	goModPath := filepath.Join(workspacePath, "go.mod")
	if _, err := os.Stat(goModPath); err != nil {
		return fmt.Errorf("go.mod not found: %w", err)
	}

	// Run go mod tidy to ensure dependencies are ready
	rc.logger.Printf("Running go mod tidy in %s", workspacePath)
	cmd := exec.CommandContext(ctx, "go", "mod", "tidy")
	cmd.Dir = workspacePath
	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go mod tidy failed: %w\nOutput: %s", err, string(output))
	}

	// Verify go build works
	rc.logger.Printf("Verifying go build in %s", workspacePath)
	buildCmd := exec.CommandContext(ctx, "go", "build", "./...")
	buildCmd.Dir = workspacePath
	buildCmd.Env = append(os.Environ(), "GO111MODULE=on")
	
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build failed: %w\nOutput: %s", err, string(buildOutput))
	}

	return nil
}

// Private helper methods

func (rc *RepositoryCloner) extractRepositoryName() string {
	// Extract repository name from URL (e.g., "https://github.com/fatih/color.git" -> "color")
	parts := strings.Split(rc.repositoryURL, "/")
	if len(parts) == 0 {
		return "repository"
	}
	
	name := parts[len(parts)-1]
	if strings.HasSuffix(name, ".git") {
		name = strings.TrimSuffix(name, ".git")
	}
	
	return name
}

func (rc *RepositoryCloner) cloneWithRetry(targetDir string, maxRetries int) error {
	var lastErr error
	
	for attempt := 1; attempt <= maxRetries; attempt++ {
		rc.logger.Printf("Cloning attempt %d/%d: %s", attempt, maxRetries, rc.repositoryURL)
		
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		
		// Use shallow clone for performance, then fetch the specific commit
		cmd := exec.CommandContext(ctx, "git", "clone", "--no-checkout", rc.repositoryURL, targetDir)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		
		output, err := cmd.CombinedOutput()
		cancel()
		
		if err == nil {
			rc.logger.Printf("Clone successful on attempt %d", attempt)
			return nil
		}
		
		lastErr = fmt.Errorf("git clone failed (attempt %d): %w\nOutput: %s", attempt, err, string(output))
		rc.logger.Printf("Clone attempt %d failed: %v", attempt, lastErr)
		
		if attempt < maxRetries {
			// Clean up failed attempt
			os.RemoveAll(targetDir)
			time.Sleep(time.Duration(attempt) * time.Second) // Exponential backoff
		}
	}
	
	return lastErr
}

func (rc *RepositoryCloner) checkoutCommit(repoDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	// First, fetch the specific commit (in case it's not in the default branch)
	rc.logger.Printf("Fetching commit %s", rc.targetCommit)
	fetchCmd := exec.CommandContext(ctx, "git", "fetch", "origin", rc.targetCommit)
	fetchCmd.Dir = repoDir
	fetchCmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	
	// Ignore fetch errors - the commit might already be available
	fetchOutput, fetchErr := fetchCmd.CombinedOutput()
	if fetchErr != nil {
		rc.logger.Printf("Fetch warning (continuing): %v\nOutput: %s", fetchErr, string(fetchOutput))
	}

	// Checkout the specific commit
	rc.logger.Printf("Checking out commit %s", rc.targetCommit)
	checkoutCmd := exec.CommandContext(ctx, "git", "checkout", rc.targetCommit)
	checkoutCmd.Dir = repoDir
	
	output, err := checkoutCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git checkout failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func (rc *RepositoryCloner) getCurrentCommitHash(repoDir string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current commit: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

func (rc *RepositoryCloner) moveToFinalLocation(source, destination string) error {
	// Remove destination if it exists
	if _, err := os.Stat(destination); err == nil {
		if err := os.RemoveAll(destination); err != nil {
			return fmt.Errorf("failed to remove existing destination: %w", err)
		}
	}

	// Move the repository
	if err := os.Rename(source, destination); err != nil {
		return fmt.Errorf("failed to move repository: %w", err)
	}

	return nil
}

// GetRepositoryInfo returns information about the configured repository
func (rc *RepositoryCloner) GetRepositoryInfo() (string, string) {
	return rc.repositoryURL, rc.targetCommit
}

// SetLogger allows customizing the logger used by the cloner
func (rc *RepositoryCloner) SetLogger(logger *log.Logger) {
	rc.logger = logger
}