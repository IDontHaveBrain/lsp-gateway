package testutils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

type ErrorType string

const (
	ErrorTypeGit        ErrorType = "git_operation"
	ErrorTypeNetwork    ErrorType = "network"
	ErrorTypeFileSystem ErrorType = "filesystem"
	ErrorTypeTimeout    ErrorType = "timeout"
	ErrorTypeValidation ErrorType = "validation"
	ErrorTypeRepository ErrorType = "repository"
	ErrorTypeLSP        ErrorType = "lsp_operation"
	ErrorTypeAuth       ErrorType = "authentication"
	ErrorTypeResource   ErrorType = "resource_exhaustion"
	ErrorTypeConcurrency ErrorType = "concurrency"
)

type PythonRepoError struct {
	Type        ErrorType              `json:"type"`
	Operation   string                `json:"operation"`
	Message     string                `json:"message"`
	Cause       error                 `json:"cause,omitempty"`
	Context     map[string]interface{} `json:"context"`
	Timestamp   time.Time             `json:"timestamp"`
	Recoverable bool                  `json:"recoverable"`
	RetryCount  int                   `json:"retry_count"`
	StackTrace  string                `json:"stack_trace,omitempty"`
}

func (e *PythonRepoError) Error() string {
	return fmt.Sprintf("[%s] %s: %s (retry: %d)", e.Type, e.Operation, e.Message, e.RetryCount)
}

func (e *PythonRepoError) Unwrap() error {
	return e.Cause
}

func (e *PythonRepoError) WithContext(key string, value interface{}) *PythonRepoError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

func (e *PythonRepoError) IncrementRetry() *PythonRepoError {
	e.RetryCount++
	return e
}

func NewPythonRepoError(errorType ErrorType, operation string, err error) *PythonRepoError {
	message := "unknown error"
	if err != nil {
		message = err.Error()
	}

	return &PythonRepoError{
		Type:        errorType,
		Operation:   operation,
		Message:     message,
		Cause:       err,
		Context:     make(map[string]interface{}),
		Timestamp:   time.Now(),
		Recoverable: IsRecoverableError(err),
		RetryCount:  0,
	}
}

type ErrorRecoveryConfig struct {
	MaxRetries          int
	RetryDelay          time.Duration
	BackoffMultiplier   float64
	MaxRetryDelay       time.Duration
	RecoverableErrors   []ErrorType
	FallbackStrategies  map[ErrorType]RecoveryStrategy
	TimeoutMultiplier   float64
	EnableEmergencyMode bool
}

func DefaultErrorRecoveryConfig() ErrorRecoveryConfig {
	return ErrorRecoveryConfig{
		MaxRetries:        3,
		RetryDelay:        2 * time.Second,
		BackoffMultiplier: 2.0,
		MaxRetryDelay:     30 * time.Second,
		RecoverableErrors: []ErrorType{
			ErrorTypeNetwork,
			ErrorTypeTimeout,
			ErrorTypeResource,
			ErrorTypeConcurrency,
		},
		FallbackStrategies: map[ErrorType]RecoveryStrategy{
			ErrorTypeGit:         &GitRecoveryStrategy{},
			ErrorTypeNetwork:     &NetworkRecoveryStrategy{},
			ErrorTypeFileSystem:  &FileSystemRecoveryStrategy{},
			ErrorTypeRepository:  &RepositoryRecoveryStrategy{},
			ErrorTypeConcurrency: &ConcurrencyRecoveryStrategy{},
		},
		TimeoutMultiplier:   1.5,
		EnableEmergencyMode: true,
	}
}

type RecoveryStrategy interface {
	Recover(err *PythonRepoError) error
	CanRecover(err *PythonRepoError) bool
	Name() string
}

type GitRecoveryStrategy struct {
	mu sync.Mutex
}

func (g *GitRecoveryStrategy) Name() string {
	return "GitRecovery"
}

func (g *GitRecoveryStrategy) CanRecover(err *PythonRepoError) bool {
	if err.Type != ErrorTypeGit {
		return false
	}

	recoverablePatterns := []string{
		"connection timed out",
		"connection refused",
		"temporary failure",
		"network is unreachable",
		"ssl certificate problem",
		"the remote end hung up",
		"shallow update not allowed",
		"fetch failed",
	}

	errorMsg := strings.ToLower(err.Message)
	for _, pattern := range recoverablePatterns {
		if strings.Contains(errorMsg, pattern) {
			return true
		}
	}

	return err.RetryCount < 3
}

func (g *GitRecoveryStrategy) Recover(err *PythonRepoError) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	operation := err.Operation
	errorMsg := strings.ToLower(err.Message)

	if strings.Contains(errorMsg, "shallow") {
		return g.recoverShallowClone(err)
	}

	if strings.Contains(errorMsg, "ssl certificate") {
		return g.recoverSSLError(err)
	}

	if strings.Contains(errorMsg, "authentication") || strings.Contains(errorMsg, "permission denied") {
		return g.recoverAuthError(err)
	}

	if strings.Contains(operation, "ls-remote") {
		return g.recoverRemoteListError(err)
	}

	if strings.Contains(operation, "clone") {
		return g.recoverCloneError(err)
	}

	if strings.Contains(operation, "fetch") || strings.Contains(operation, "checkout") {
		return g.recoverFetchCheckoutError(err)
	}

	return fmt.Errorf("no recovery strategy available for git error: %w", err)
}

func (g *GitRecoveryStrategy) recoverShallowClone(err *PythonRepoError) error {
	if workspaceDir, ok := err.Context["workspace_dir"].(string); ok {
		repoDir := filepath.Join(workspaceDir, "python-patterns")
		
		if err := os.RemoveAll(repoDir); err != nil {
			return fmt.Errorf("failed to remove corrupted repository: %w", err)
		}

		log.Printf("[GitRecovery] Attempting full clone after shallow clone failure")
		
		if repoURL, ok := err.Context["repo_url"].(string); ok {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			cmd := exec.CommandContext(ctx, "git", "clone", repoURL, repoDir)
			if output, cloneErr := cmd.CombinedOutput(); cloneErr != nil {
				return fmt.Errorf("full clone recovery failed: %w, output: %s", cloneErr, string(output))
			}
		}
	}

	return nil
}

func (g *GitRecoveryStrategy) recoverSSLError(err *PythonRepoError) error {
	log.Printf("[GitRecovery] Attempting SSL error recovery")
	
	if repoURL, ok := err.Context["repo_url"].(string); ok {
		if strings.HasPrefix(repoURL, "https://") {
			log.Printf("[GitRecovery] SSL error detected, this might require manual certificate configuration")
			return fmt.Errorf("SSL certificate error requires manual intervention")
		}
	}

	return fmt.Errorf("SSL recovery not applicable")
}

func (g *GitRecoveryStrategy) recoverAuthError(err *PythonRepoError) error {
	log.Printf("[GitRecovery] Authentication error detected")
	return fmt.Errorf("authentication error requires credential configuration")
}

func (g *GitRecoveryStrategy) recoverRemoteListError(err *PythonRepoError) error {
	log.Printf("[GitRecovery] Attempting remote listing recovery")
	
	if repoURL, ok := err.Context["repo_url"].(string); ok {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "git", "ls-remote", "--heads", repoURL)
		if output, cmdErr := cmd.CombinedOutput(); cmdErr != nil {
			return fmt.Errorf("remote list recovery failed: %w, output: %s", cmdErr, string(output))
		}
	}

	return nil
}

func (g *GitRecoveryStrategy) recoverCloneError(err *PythonRepoError) error {
	log.Printf("[GitRecovery] Attempting clone error recovery")
	
	if workspaceDir, ok := err.Context["workspace_dir"].(string); ok {
		repoDir := filepath.Join(workspaceDir, "python-patterns")
		
		if _, statErr := os.Stat(repoDir); statErr == nil {
			if removeErr := os.RemoveAll(repoDir); removeErr != nil {
				log.Printf("[GitRecovery] Warning: failed to remove partial clone: %v", removeErr)
			}
		}

		if repoURL, ok := err.Context["repo_url"].(string); ok {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			cmd := exec.CommandContext(ctx, "git", "clone", "--depth=10", repoURL, repoDir)
			if output, cloneErr := cmd.CombinedOutput(); cloneErr != nil {
				return fmt.Errorf("clone recovery with depth=10 failed: %w, output: %s", cloneErr, string(output))
			}
		}
	}

	return nil
}

func (g *GitRecoveryStrategy) recoverFetchCheckoutError(err *PythonRepoError) error {
	log.Printf("[GitRecovery] Attempting fetch/checkout recovery")
	
	if workspaceDir, ok := err.Context["workspace_dir"].(string); ok {
		repoDir := filepath.Join(workspaceDir, "python-patterns")
		
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		resetCmd := exec.CommandContext(ctx, "git", "reset", "--hard", "HEAD")
		resetCmd.Dir = repoDir
		if output, resetErr := resetCmd.CombinedOutput(); resetErr != nil {
			log.Printf("[GitRecovery] Git reset failed: %v, output: %s", resetErr, string(output))
		}

		cleanCmd := exec.CommandContext(ctx, "git", "clean", "-fd")
		cleanCmd.Dir = repoDir
		if output, cleanErr := cleanCmd.CombinedOutput(); cleanErr != nil {
			log.Printf("[GitRecovery] Git clean failed: %v, output: %s", cleanErr, string(output))
		}
	}

	return nil
}

type NetworkRecoveryStrategy struct{}

func (n *NetworkRecoveryStrategy) Name() string {
	return "NetworkRecovery"
}

func (n *NetworkRecoveryStrategy) CanRecover(err *PythonRepoError) bool {
	if err.Type != ErrorTypeNetwork {
		return false
	}

	errorMsg := strings.ToLower(err.Message)
	recoverablePatterns := []string{
		"connection timed out",
		"connection refused",
		"network is unreachable",
		"temporary failure in name resolution",
		"no route to host",
		"connection reset by peer",
	}

	for _, pattern := range recoverablePatterns {
		if strings.Contains(errorMsg, pattern) {
			return true
		}
	}

	return false
}

func (n *NetworkRecoveryStrategy) Recover(err *PythonRepoError) error {
	log.Printf("[NetworkRecovery] Attempting network recovery")

	if repoURL, ok := err.Context["repo_url"].(string); ok {
		if err := n.testConnectivity(repoURL); err != nil {
			return fmt.Errorf("network connectivity test failed: %w", err)
		}
	}

	time.Sleep(5 * time.Second)
	return nil
}

func (n *NetworkRecoveryStrategy) testConnectivity(repoURL string) error {
	var host string
	
	if strings.HasPrefix(repoURL, "https://github.com") {
		host = "github.com:443"
	} else if strings.HasPrefix(repoURL, "http://") {
		host = strings.TrimPrefix(repoURL, "http://")
		if idx := strings.Index(host, "/"); idx != -1 {
			host = host[:idx]
		}
		host += ":80"
	} else if strings.HasPrefix(repoURL, "https://") {
		host = strings.TrimPrefix(repoURL, "https://")
		if idx := strings.Index(host, "/"); idx != -1 {
			host = host[:idx]
		}
		host += ":443"
	}

	if host != "" {
		conn, err := net.DialTimeout("tcp", host, 10*time.Second)
		if err != nil {
			return fmt.Errorf("connectivity test failed for %s: %w", host, err)
		}
		conn.Close()
	}

	return nil
}

type FileSystemRecoveryStrategy struct{}

func (f *FileSystemRecoveryStrategy) Name() string {
	return "FileSystemRecovery"
}

func (f *FileSystemRecoveryStrategy) CanRecover(err *PythonRepoError) bool {
	if err.Type != ErrorTypeFileSystem {
		return false
	}

	errorMsg := strings.ToLower(err.Message)
	recoverablePatterns := []string{
		"permission denied",
		"file exists",
		"directory not empty",
		"no space left on device",
		"resource temporarily unavailable",
	}

	for _, pattern := range recoverablePatterns {
		if strings.Contains(errorMsg, pattern) {
			return true
		}
	}

	return false
}

func (f *FileSystemRecoveryStrategy) Recover(err *PythonRepoError) error {
	log.Printf("[FileSystemRecovery] Attempting filesystem recovery")

	errorMsg := strings.ToLower(err.Message)

	if strings.Contains(errorMsg, "permission denied") {
		return f.recoverPermissionError(err)
	}

	if strings.Contains(errorMsg, "directory not empty") || strings.Contains(errorMsg, "file exists") {
		return f.recoverExistingPathError(err)
	}

	if strings.Contains(errorMsg, "no space left") {
		return f.recoverDiskSpaceError(err)
	}

	return fmt.Errorf("no recovery strategy for filesystem error: %w", err)
}

func (f *FileSystemRecoveryStrategy) recoverPermissionError(err *PythonRepoError) error {
	if path, ok := err.Context["path"].(string); ok {
		info, statErr := os.Stat(path)
		if statErr == nil {
			currentMode := info.Mode()
			log.Printf("[FileSystemRecovery] Current permissions for %s: %v", path, currentMode)
			
			if info.IsDir() {
				newMode := currentMode | 0755
				if chmodErr := os.Chmod(path, newMode); chmodErr != nil {
					return fmt.Errorf("failed to fix directory permissions: %w", chmodErr)
				}
			} else {
				newMode := currentMode | 0644
				if chmodErr := os.Chmod(path, newMode); chmodErr != nil {
					return fmt.Errorf("failed to fix file permissions: %w", chmodErr)
				}
			}
		}
	}

	return nil
}

func (f *FileSystemRecoveryStrategy) recoverExistingPathError(err *PythonRepoError) error {
	if path, ok := err.Context["path"].(string); ok {
		log.Printf("[FileSystemRecovery] Removing existing path: %s", path)
		if removeErr := os.RemoveAll(path); removeErr != nil {
			return fmt.Errorf("failed to remove existing path: %w", removeErr)
		}
	}

	return nil
}

func (f *FileSystemRecoveryStrategy) recoverDiskSpaceError(err *PythonRepoError) error {
	log.Printf("[FileSystemRecovery] Disk space exhaustion detected")

	if workspaceDir, ok := err.Context["workspace_dir"].(string); ok {
		usage, err := f.getDiskUsage(workspaceDir)
		if err != nil {
			log.Printf("[FileSystemRecovery] Failed to get disk usage: %v", err)
			return fmt.Errorf("disk space recovery failed: insufficient space")
		}

		log.Printf("[FileSystemRecovery] Available space: %d MB", usage.Available/(1024*1024))
		
		if usage.Available < 100*1024*1024 {
			return fmt.Errorf("insufficient disk space: %d MB available", usage.Available/(1024*1024))
		}
	}

	return nil
}

type DiskUsage struct {
	Available uint64
	Total     uint64
	Used      uint64
}

func (f *FileSystemRecoveryStrategy) getDiskUsage(path string) (*DiskUsage, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return nil, err
	}

	return &DiskUsage{
		Available: stat.Bavail * uint64(stat.Bsize),
		Total:     stat.Blocks * uint64(stat.Bsize),
		Used:      (stat.Blocks - stat.Bfree) * uint64(stat.Bsize),
	}, nil
}

type RepositoryRecoveryStrategy struct{}

func (r *RepositoryRecoveryStrategy) Name() string {
	return "RepositoryRecovery"
}

func (r *RepositoryRecoveryStrategy) CanRecover(err *PythonRepoError) bool {
	if err.Type != ErrorTypeRepository {
		return false
	}

	errorMsg := strings.ToLower(err.Message)
	recoverablePatterns := []string{
		"patterns directory not found",
		"repository validation failed",
		"expected directory not found",
		"invalid repository structure",
	}

	for _, pattern := range recoverablePatterns {
		if strings.Contains(errorMsg, pattern) {
			return true
		}
	}

	return false
}

func (r *RepositoryRecoveryStrategy) Recover(err *PythonRepoError) error {
	log.Printf("[RepositoryRecovery] Attempting repository structure recovery")

	if workspaceDir, ok := err.Context["workspace_dir"].(string); ok {
		return r.validateAndRepairRepository(workspaceDir)
	}

	return fmt.Errorf("workspace directory not available for recovery")
}

func (r *RepositoryRecoveryStrategy) validateAndRepairRepository(workspaceDir string) error {
	repoPath := filepath.Join(workspaceDir, "python-patterns")
	
	expectedDirs := []string{
		"patterns",
		"patterns/creational",
		"patterns/structural", 
		"patterns/behavioral",
	}

	for _, expectedDir := range expectedDirs {
		dirPath := filepath.Join(repoPath, expectedDir)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			log.Printf("[RepositoryRecovery] Creating missing directory: %s", expectedDir)
			if mkdirErr := os.MkdirAll(dirPath, 0755); mkdirErr != nil {
				return fmt.Errorf("failed to create directory %s: %w", expectedDir, mkdirErr)
			}
		}
	}

	pythonFiles, err := r.findPythonFiles(repoPath)
	if err != nil {
		return fmt.Errorf("failed to scan for Python files: %w", err)
	}

	if len(pythonFiles) == 0 {
		return fmt.Errorf("no Python files found in repository - repository may be corrupted")
	}

	log.Printf("[RepositoryRecovery] Found %d Python files", len(pythonFiles))
	return nil
}

func (r *RepositoryRecoveryStrategy) findPythonFiles(rootPath string) ([]string, error) {
	var pythonFiles []string

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), ".py") {
			if !strings.Contains(path, "__pycache__") && !strings.HasPrefix(info.Name(), ".") {
				pythonFiles = append(pythonFiles, path)
			}
		}

		return nil
	})

	return pythonFiles, err
}

type ConcurrencyRecoveryStrategy struct{}

func (c *ConcurrencyRecoveryStrategy) Name() string {
	return "ConcurrencyRecovery"
}

func (c *ConcurrencyRecoveryStrategy) CanRecover(err *PythonRepoError) bool {
	if err.Type != ErrorTypeConcurrency {
		return false
	}

	errorMsg := strings.ToLower(err.Message)
	recoverablePatterns := []string{
		"resource temporarily unavailable",
		"text file busy",
		"device or resource busy",
		"file exists",
	}

	for _, pattern := range recoverablePatterns {
		if strings.Contains(errorMsg, pattern) {
			return true
		}
	}

	return false
}

func (c *ConcurrencyRecoveryStrategy) Recover(err *PythonRepoError) error {
	log.Printf("[ConcurrencyRecovery] Attempting concurrency recovery")

	delay := time.Duration(err.RetryCount+1) * time.Second
	if delay > 10*time.Second {
		delay = 10 * time.Second
	}

	log.Printf("[ConcurrencyRecovery] Waiting %v before retry", delay)
	time.Sleep(delay)

	return nil
}

func ClassifyError(err error) ErrorType {
	if err == nil {
		return ErrorTypeValidation
	}

	errorMsg := strings.ToLower(err.Error())

	gitPatterns := []string{
		"git", "clone", "fetch", "checkout", "ls-remote", "repository",
		"remote", "branch", "commit", "shallow",
	}

	networkPatterns := []string{
		"connection", "network", "dns", "timeout", "refused", "unreachable",
		"ssl", "tls", "certificate", "handshake", "proxy",
	}

	filesystemPatterns := []string{
		"permission denied", "no such file", "directory", "file exists",
		"no space left", "read-only", "mount", "filesystem",
	}

	timeoutPatterns := []string{
		"timeout", "deadline exceeded", "context canceled", "context deadline exceeded",
	}

	resourcePatterns := []string{
		"no space left", "out of memory", "resource temporarily unavailable",
		"too many open files", "device or resource busy",
	}

	concurrencyPatterns := []string{
		"text file busy", "resource busy", "lock", "concurrent",
	}

	for _, pattern := range gitPatterns {
		if strings.Contains(errorMsg, pattern) {
			return ErrorTypeGit
		}
	}

	for _, pattern := range networkPatterns {
		if strings.Contains(errorMsg, pattern) {
			return ErrorTypeNetwork
		}
	}

	for _, pattern := range timeoutPatterns {
		if strings.Contains(errorMsg, pattern) {
			return ErrorTypeTimeout
		}
	}

	for _, pattern := range resourcePatterns {
		if strings.Contains(errorMsg, pattern) {
			return ErrorTypeResource
		}
	}

	for _, pattern := range concurrencyPatterns {
		if strings.Contains(errorMsg, pattern) {
			return ErrorTypeConcurrency
		}
	}

	for _, pattern := range filesystemPatterns {
		if strings.Contains(errorMsg, pattern) {
			return ErrorTypeFileSystem
		}
	}

	if exitError, ok := err.(*exec.ExitError); ok {
		if exitError.ExitCode() == 128 {
			return ErrorTypeGit
		}
	}

	return ErrorTypeValidation
}

func IsRecoverableError(err error) bool {
	if err == nil {
		return false
	}

	errorType := ClassifyError(err)
	recoverableTypes := []ErrorType{
		ErrorTypeNetwork,
		ErrorTypeTimeout,
		ErrorTypeResource,
		ErrorTypeConcurrency,
		ErrorTypeFileSystem,
	}

	for _, recoverableType := range recoverableTypes {
		if errorType == recoverableType {
			return true
		}
	}

	if errorType == ErrorTypeGit {
		errorMsg := strings.ToLower(err.Error())
		gitRecoverablePatterns := []string{
			"connection timed out",
			"connection refused",
			"temporary failure",
			"network is unreachable",
			"shallow update not allowed",
			"fetch failed",
		}

		for _, pattern := range gitRecoverablePatterns {
			if strings.Contains(errorMsg, pattern) {
				return true
			}
		}
	}

	return false
}

func IsTransientError(err error) bool {
	if err == nil {
		return false
	}

	errorMsg := strings.ToLower(err.Error())
	transientPatterns := []string{
		"connection timed out",
		"connection refused",
		"temporary failure",
		"network is unreachable",
		"resource temporarily unavailable",
		"text file busy",
		"device or resource busy",
		"too many open files",
	}

	for _, pattern := range transientPatterns {
		if strings.Contains(errorMsg, pattern) {
			return true
		}
	}

	return false
}

func RetryWithBackoff(operation func() error, config ErrorRecoveryConfig) error {
	var lastErr error
	delay := config.RetryDelay

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			if delay > config.MaxRetryDelay {
				delay = config.MaxRetryDelay
			}
			
			log.Printf("[RetryWithBackoff] Attempt %d/%d, waiting %v", attempt, config.MaxRetries, delay)
			time.Sleep(delay)
			delay = time.Duration(float64(delay) * config.BackoffMultiplier)
		}

		err := operation()
		if err == nil {
			if attempt > 0 {
				log.Printf("[RetryWithBackoff] Operation succeeded after %d attempts", attempt)
			}
			return nil
		}

		lastErr = err

		if !IsTransientError(err) && !IsRecoverableError(err) {
			log.Printf("[RetryWithBackoff] Non-recoverable error: %v", err)
			break
		}

		if attempt < config.MaxRetries {
			log.Printf("[RetryWithBackoff] Attempt %d failed: %v", attempt, err)
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", config.MaxRetries+1, lastErr)
}

func ExecuteWithRetry(fn func() error, maxRetries int, delay time.Duration) error {
	config := ErrorRecoveryConfig{
		MaxRetries:        maxRetries,
		RetryDelay:        delay,
		BackoffMultiplier: 1.5,
		MaxRetryDelay:     30 * time.Second,
	}

	return RetryWithBackoff(fn, config)
}

func FormatErrorForLogging(err *PythonRepoError) string {
	contextStr := ""
	if len(err.Context) > 0 {
		if contextBytes, marshalErr := json.Marshal(err.Context); marshalErr == nil {
			contextStr = string(contextBytes)
		}
	}

	return fmt.Sprintf(
		"[%s] %s failed at %s (retry: %d, recoverable: %t): %s | Context: %s",
		err.Type,
		err.Operation,
		err.Timestamp.Format(time.RFC3339),
		err.RetryCount,
		err.Recoverable,
		err.Message,
		contextStr,
	)
}

func EmergencyCleanup(workspaceDir string) error {
	if workspaceDir == "" {
		return fmt.Errorf("workspace directory is empty")
	}

	if !strings.Contains(workspaceDir, "lspg-python-e2e-tests") {
		return fmt.Errorf("refusing to cleanup directory that doesn't appear to be a test workspace: %s", workspaceDir)
	}

	log.Printf("[EmergencyCleanup] Starting emergency cleanup of %s", workspaceDir)

	maxAttempts := 3
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
			log.Printf("[EmergencyCleanup] Attempt %d/%d", attempt+1, maxAttempts)
		}

		if err := forceRemoveDirectory(workspaceDir); err != nil {
			lastErr = err
			log.Printf("[EmergencyCleanup] Attempt %d failed: %v", attempt+1, err)
			continue
		}

		log.Printf("[EmergencyCleanup] Successfully cleaned up workspace")
		return nil
	}

	return fmt.Errorf("emergency cleanup failed after %d attempts: %w", maxAttempts, lastErr)
}

func forceRemoveDirectory(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}

	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if !info.IsDir() {
			if chmodErr := os.Chmod(path, 0666); chmodErr != nil {
				log.Printf("[EmergencyCleanup] Failed to change file permissions: %v", chmodErr)
			}
		} else {
			if chmodErr := os.Chmod(path, 0755); chmodErr != nil {
				log.Printf("[EmergencyCleanup] Failed to change directory permissions: %v", chmodErr)
			}
		}

		return nil
	}); err != nil {
		log.Printf("[EmergencyCleanup] Warning during permission fix: %v", err)
	}

	return os.RemoveAll(dir)
}

func RecoverFromPartialClone(repoDir string) error {
	log.Printf("[RecoverFromPartialClone] Attempting to recover partial clone at %s", repoDir)

	gitDir := filepath.Join(repoDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository: %s", repoDir)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	statusCmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	statusCmd.Dir = repoDir
	output, err := statusCmd.CombinedOutput()
	if err != nil {
		log.Printf("[RecoverFromPartialClone] Git status failed: %v", err)
		return os.RemoveAll(repoDir)
	}

	if len(output) > 0 {
		log.Printf("[RecoverFromPartialClone] Repository has uncommitted changes, performing reset")
		
		resetCmd := exec.CommandContext(ctx, "git", "reset", "--hard", "HEAD")
		resetCmd.Dir = repoDir
		if resetOutput, resetErr := resetCmd.CombinedOutput(); resetErr != nil {
			log.Printf("[RecoverFromPartialClone] Git reset failed: %v, output: %s", resetErr, string(resetOutput))
			return os.RemoveAll(repoDir)
		}

		cleanCmd := exec.CommandContext(ctx, "git", "clean", "-fd")
		cleanCmd.Dir = repoDir
		if cleanOutput, cleanErr := cleanCmd.CombinedOutput(); cleanErr != nil {
			log.Printf("[RecoverFromPartialClone] Git clean failed: %v, output: %s", cleanErr, string(cleanOutput))
		}
	}

	fsckCmd := exec.CommandContext(ctx, "git", "fsck")
	fsckCmd.Dir = repoDir
	if fsckOutput, fsckErr := fsckCmd.CombinedOutput(); fsckErr != nil {
		log.Printf("[RecoverFromPartialClone] Repository integrity check failed: %v, output: %s", fsckErr, string(fsckOutput))
		return os.RemoveAll(repoDir)
	}

	log.Printf("[RecoverFromPartialClone] Repository recovery completed successfully")
	return nil
}

func ValidateAndRepairWorkspace(workspaceDir string) error {
	log.Printf("[ValidateAndRepairWorkspace] Validating workspace: %s", workspaceDir)

	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		log.Printf("[ValidateAndRepairWorkspace] Workspace doesn't exist, creating: %s", workspaceDir)
		if mkdirErr := os.MkdirAll(workspaceDir, 0755); mkdirErr != nil {
			return fmt.Errorf("failed to create workspace directory: %w", mkdirErr)
		}
	}

	repoDir := filepath.Join(workspaceDir, "python-patterns")
	if _, err := os.Stat(repoDir); err == nil {
		if err := RecoverFromPartialClone(repoDir); err != nil {
			log.Printf("[ValidateAndRepairWorkspace] Repository recovery failed: %v", err)
			return err
		}
	}

	tempFile := filepath.Join(workspaceDir, "test-write-permissions")
	if err := os.WriteFile(tempFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("workspace is not writable: %w", err)
	}
	os.Remove(tempFile)

	log.Printf("[ValidateAndRepairWorkspace] Workspace validation completed successfully")
	return nil
}

var (
	gitOperationMutex sync.Mutex
	activeOperations  = make(map[string]bool)
)

func ExecuteGitOperationWithRecovery(operation string, workspaceDir, repoURL string, fn func() error) error {
	gitOperationMutex.Lock()
	operationKey := fmt.Sprintf("%s:%s", operation, workspaceDir)
	if activeOperations[operationKey] {
		gitOperationMutex.Unlock()
		return fmt.Errorf("git operation already in progress: %s", operation)
	}
	activeOperations[operationKey] = true
	gitOperationMutex.Unlock()

	defer func() {
		gitOperationMutex.Lock()
		delete(activeOperations, operationKey)
		gitOperationMutex.Unlock()
	}()

	config := DefaultErrorRecoveryConfig()
	
	return RetryWithBackoff(func() error {
		err := fn()
		if err != nil {
			repoErr := NewPythonRepoError(ClassifyError(err), operation, err)
			repoErr.WithContext("workspace_dir", workspaceDir)
			repoErr.WithContext("repo_url", repoURL)

			if strategy, exists := config.FallbackStrategies[repoErr.Type]; exists && strategy.CanRecover(repoErr) {
				log.Printf("[ExecuteGitOperationWithRecovery] Attempting recovery for %s", operation)
				if recoveryErr := strategy.Recover(repoErr); recoveryErr != nil {
					log.Printf("[ExecuteGitOperationWithRecovery] Recovery failed: %v", recoveryErr)
					return repoErr
				}
				
				repoErr.IncrementRetry()
				return repoErr
			}

			return repoErr
		}
		return nil
	}, config)
}

func WrapWithErrorHandling(operation string, fn func() error) error {
	return ExecuteWithRetry(func() error {
		err := fn()
		if err != nil {
			errorType := ClassifyError(err)
			repoErr := NewPythonRepoError(errorType, operation, err)
			
			if !repoErr.Recoverable {
				return err
			}

			return repoErr
		}
		return nil
	}, 3, 2*time.Second)
}

type HealthChecker struct {
	workspaceDir string
	repoURL      string
}

func NewHealthChecker(workspaceDir, repoURL string) *HealthChecker {
	return &HealthChecker{
		workspaceDir: workspaceDir,
		repoURL:      repoURL,
	}
}

func (h *HealthChecker) CheckRepositoryHealth() error {
	repoDir := filepath.Join(h.workspaceDir, "python-patterns")
	
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		return fmt.Errorf("repository directory not found: %s", repoDir)
	}

	gitDir := filepath.Join(repoDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		log.Printf("[HealthChecker] .git directory not found, checking for extracted repository")
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		statusCmd := exec.CommandContext(ctx, "git", "status")
		statusCmd.Dir = repoDir
		if _, err := statusCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git repository health check failed: %w", err)
		}
	}

	patternsDir := filepath.Join(repoDir, "patterns")
	if _, err := os.Stat(patternsDir); os.IsNotExist(err) {
		return fmt.Errorf("patterns directory not found: %s", patternsDir)
	}

	pythonFiles := 0
	err := filepath.Walk(patternsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".py") {
			pythonFiles++
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to scan patterns directory: %w", err)
	}

	if pythonFiles == 0 {
		return fmt.Errorf("no Python files found in patterns directory")
	}

	log.Printf("[HealthChecker] Repository health check passed: %d Python files found", pythonFiles)
	return nil
}

func (h *HealthChecker) CheckNetworkConnectivity() error {
	if h.repoURL == "" {
		return fmt.Errorf("repository URL not configured")
	}

	strategy := &NetworkRecoveryStrategy{}
	testErr := &PythonRepoError{
		Type:    ErrorTypeNetwork,
		Context: map[string]interface{}{"repo_url": h.repoURL},
	}

	return strategy.testConnectivity(h.repoURL)
}

func CreateErrorContext(operation string, params ...interface{}) map[string]interface{} {
	context := map[string]interface{}{
		"operation": operation,
		"timestamp": time.Now(),
	}

	for i := 0; i < len(params); i += 2 {
		if i+1 < len(params) {
			if key, ok := params[i].(string); ok {
				context[key] = params[i+1]
			}
		}
	}

	return context
}

func LogErrorWithContext(err error, context map[string]interface{}) {
	if repoErr, ok := err.(*PythonRepoError); ok {
		log.Printf("[ERROR] %s", FormatErrorForLogging(repoErr))
	} else {
		log.Printf("[ERROR] %v | Context: %+v", err, context)
	}
}

func ChainErrors(primary error, secondary error) error {
	if primary == nil {
		return secondary
	}
	if secondary == nil {
		return primary
	}
	return fmt.Errorf("primary: %w, secondary: %v", primary, secondary)
}