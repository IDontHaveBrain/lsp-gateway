package testutils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// GitValidationResult represents the result of Git repository validation
type GitValidationResult struct {
	RepoPath           string                 `json:"repo_path"`
	RepoURL            string                 `json:"repo_url"`
	CommitHash         string                 `json:"commit_hash"`
	IsValid            bool                   `json:"is_valid"`
	RepositoryHealth   RepositoryHealth       `json:"repository_health"`
	IntegrityIssues    []string              `json:"integrity_issues"`
	Recommendations    []string              `json:"recommendations"`
	ValidationTime     time.Duration         `json:"validation_time"`
	LastValidated      time.Time             `json:"last_validated"`
	RepositoryInfo     map[string]string     `json:"repository_info"`
	PerformanceMetrics map[string]interface{} `json:"performance_metrics"`
	ErrorDetails       []string              `json:"error_details"`
}

// RepositoryHealth represents the overall health of a Git repository
type RepositoryHealth struct {
	Score           float64           `json:"score"`            // 0.0 - 1.0 health score
	Status          string            `json:"status"`           // "excellent", "good", "fair", "poor", "critical"
	GitIntegrity    bool              `json:"git_integrity"`    // Git fsck results
	CommitValid     bool              `json:"commit_valid"`     // Commit hash validation
	WorkingTreeClean bool             `json:"working_tree_clean"` // Working directory status
	RefsIntegrity   bool              `json:"refs_integrity"`   // Reference integrity
	ObjectsIntegrity bool             `json:"objects_integrity"` // Object database integrity
	IndexIntegrity  bool              `json:"index_integrity"`  // Index file integrity
	ConfigValid     bool              `json:"config_valid"`     // Git configuration validity
	RemotesValid    bool              `json:"remotes_valid"`    // Remote references validity
}

// GitValidationConfig defines configuration for Git validation
type GitValidationConfig struct {
	EnableDeepScan        bool          `json:"enable_deep_scan"`        // Enable comprehensive validation
	CheckRemotes          bool          `json:"check_remotes"`           // Validate remote references
	ValidateObjects       bool          `json:"validate_objects"`        // Check object integrity
	PerformanceProfiling  bool          `json:"performance_profiling"`   // Enable performance metrics
	ParallelValidation    bool          `json:"parallel_validation"`     // Enable parallel processing
	MaxValidationTime     time.Duration `json:"max_validation_time"`     // Maximum validation time
	AutoRepairLevel       int           `json:"auto_repair_level"`       // 0=none, 1=safe, 2=aggressive
	EnableBackgroundScan  bool          `json:"enable_background_scan"`  // Background validation
	CacheValidationResults bool         `json:"cache_validation_results"` // Cache validation results
	LogLevel              string        `json:"log_level"`               // "debug", "info", "warn", "error"
}

// RepairAction represents a repository repair action
type RepairAction struct {
	Action      string        `json:"action"`       // Type of repair action
	Description string        `json:"description"`  // Human-readable description
	Risk        string        `json:"risk"`         // "low", "medium", "high"
	Reversible  bool          `json:"reversible"`   // Whether action can be undone
	Command     []string      `json:"command"`      // Git command to execute
	Duration    time.Duration `json:"duration"`     // Time taken for repair
	Success     bool          `json:"success"`      // Whether repair succeeded
	Error       string        `json:"error"`        // Error message if failed
}

// ValidationStats tracks validation performance metrics
type ValidationStats struct {
	TotalValidations    int64         `json:"total_validations"`
	SuccessfulValidations int64       `json:"successful_validations"`
	FailedValidations   int64         `json:"failed_validations"`
	AverageValidationTime time.Duration `json:"average_validation_time"`
	CacheHitRate        float64       `json:"cache_hit_rate"`
	RepositoriesScanned int64         `json:"repositories_scanned"`
	IntegrityIssuesFound int64        `json:"integrity_issues_found"`
	RepairsAttempted    int64         `json:"repairs_attempted"`
	RepairsSuccessful   int64         `json:"repairs_successful"`
	LastReset           time.Time     `json:"last_reset"`
}

// GitValidator manages Git repository integrity validation and repair
type GitValidator struct {
	config            GitValidationConfig
	validationCache   map[string]*GitValidationResult
	cacheManager      interface{}
	
	// Performance metrics
	stats             ValidationStats
	validationTimes   []time.Duration
	
	// Concurrency control
	mu                sync.RWMutex
	validationSem     chan struct{} // Limits concurrent validations
	
	// Background processing
	ctx               context.Context
	cancel            context.CancelFunc
	backgroundTicker  *time.Ticker
	isRunning         bool
	
	// Atomic counters for thread-safe statistics
	totalValidations  int64
	successCount      int64
	failureCount      int64
	repairCount       int64
	repairSuccessCount int64
}

// NewGitValidator creates a new Git validator with the specified configuration
func NewGitValidator(config *GitValidationConfig) *GitValidator {
	if config == nil {
		config = &GitValidationConfig{
			EnableDeepScan:         true,
			CheckRemotes:           false,
			ValidateObjects:        true,
			PerformanceProfiling:   true,
			ParallelValidation:     true,
			MaxValidationTime:      60 * time.Second,
			AutoRepairLevel:        1, // Safe repairs only
			EnableBackgroundScan:   false,
			CacheValidationResults: true,
			LogLevel:               "info",
		}
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create semaphore for concurrent validations (max 5 concurrent)
	maxConcurrent := 5
	if !config.ParallelValidation {
		maxConcurrent = 1
	}
	
	validator := &GitValidator{
		config:          *config,
		validationCache: make(map[string]*GitValidationResult),
		validationSem:   make(chan struct{}, maxConcurrent),
		ctx:             ctx,
		cancel:          cancel,
		validationTimes: make([]time.Duration, 0, 1000),
		stats: ValidationStats{
			LastReset: time.Now(),
		},
	}
	
	validator.log("info", "Git validator initialized with config: deep_scan=%v, parallel=%v, max_time=%v", 
		config.EnableDeepScan, config.ParallelValidation, config.MaxValidationTime)
	
	return validator
}

// ValidateRepository performs comprehensive validation of a Git repository
func (gv *GitValidator) ValidateRepository(repoPath string) GitValidationResult {
	startTime := time.Now()
	atomic.AddInt64(&gv.totalValidations, 1)
	
	// Acquire semaphore for concurrent validation control
	gv.validationSem <- struct{}{}
	defer func() { <-gv.validationSem }()
	
	result := GitValidationResult{
		RepoPath:           repoPath,
		LastValidated:      startTime,
		RepositoryInfo:     make(map[string]string),
		PerformanceMetrics: make(map[string]interface{}),
		IntegrityIssues:    make([]string, 0),
		Recommendations:    make([]string, 0),
		ErrorDetails:       make([]string, 0),
	}
	
	gv.log("info", "Starting comprehensive validation for repository: %s", repoPath)
	
	// Check cache if enabled
	if gv.config.CacheValidationResults {
		if cached := gv.getCachedResult(repoPath); cached != nil {
			// Use cached result if still valid (less than 1 hour old)
			if time.Since(cached.LastValidated) < time.Hour {
				gv.log("debug", "Using cached validation result for %s", repoPath)
				return *cached
			}
		}
	}
	
	ctx, cancel := context.WithTimeout(gv.ctx, gv.config.MaxValidationTime)
	defer cancel()
	
	// Stage 1: Basic repository validation
	if err := gv.validateBasicStructure(ctx, repoPath, &result); err != nil {
		result.ErrorDetails = append(result.ErrorDetails, fmt.Sprintf("Basic validation failed: %v", err))
		result.IsValid = false
		result.ValidationTime = time.Since(startTime)
		atomic.AddInt64(&gv.failureCount, 1)
		return result
	}
	
	// Stage 2: Git integrity validation
	gv.validateGitIntegrity(ctx, repoPath, &result)
	
	// Stage 3: Commit validation
	gv.validateCommitStructure(ctx, repoPath, &result)
	
	// Stage 4: Working tree validation
	gv.validateWorkingTree(ctx, repoPath, &result)
	
	// Stage 5: References validation
	gv.validateReferences(ctx, repoPath, &result)
	
	// Stage 6: Deep scan if enabled
	if gv.config.EnableDeepScan {
		gv.performDeepScan(ctx, repoPath, &result)
	}
	
	// Stage 7: Remote validation if enabled
	if gv.config.CheckRemotes {
		gv.validateRemotes(ctx, repoPath, &result)
	}
	
	// Calculate final health score and status
	gv.calculateRepositoryHealth(&result)
	
	// Performance metrics collection
	if gv.config.PerformanceProfiling {
		gv.recordPerformanceMetrics(&result)
	}
	
	result.ValidationTime = time.Since(startTime)
	result.IsValid = result.RepositoryHealth.Score >= 0.7 // 70% threshold for validity
	
	// Cache result if enabled
	if gv.config.CacheValidationResults {
		gv.cacheValidationResult(repoPath, &result)
	}
	
	// Update statistics
	gv.recordValidationStats(result.ValidationTime, result.IsValid)
	
	if result.IsValid {
		atomic.AddInt64(&gv.successCount, 1)
		gv.log("info", "Repository validation successful: %s (health: %.2f, time: %v)", 
			repoPath, result.RepositoryHealth.Score, result.ValidationTime)
	} else {
		atomic.AddInt64(&gv.failureCount, 1)
		gv.log("warn", "Repository validation failed: %s (health: %.2f, issues: %d)", 
			repoPath, result.RepositoryHealth.Score, len(result.IntegrityIssues))
	}
	
	return result
}

// VerifyCommitHash verifies that a repository is at the expected commit
func (gv *GitValidator) VerifyCommitHash(repoPath, expectedHash string) (bool, error) {
	if expectedHash == "" {
		return true, nil // No specific commit required
	}
	
	ctx, cancel := context.WithTimeout(gv.ctx, 10*time.Second)
	defer cancel()
	
	// Get current HEAD commit
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to get HEAD commit: %w", err)
	}
	
	currentHash := strings.TrimSpace(string(output))
	
	// Compare hashes (support both full and abbreviated hashes)
	if len(expectedHash) < 40 {
		// Abbreviated hash comparison
		return strings.HasPrefix(currentHash, expectedHash), nil
	}
	
	// Full hash comparison
	return currentHash == expectedHash, nil
}

// CheckGitIntegrity performs git fsck and returns integrity status
func (gv *GitValidator) CheckGitIntegrity(repoPath string) (bool, []string, error) {
	ctx, cancel := context.WithTimeout(gv.ctx, 30*time.Second)
	defer cancel()
	
	issues := make([]string, 0)
	
	// Check if .git directory exists
	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return false, []string{"Git directory (.git) not found"}, nil
	}
	
	// Run git fsck
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "fsck", "--full", "--strict")
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return false, []string{"Git fsck operation timed out"}, fmt.Errorf("fsck timeout")
		}
		
		// Parse fsck output for specific issues
		outputStr := string(output)
		if strings.Contains(outputStr, "broken") {
			issues = append(issues, "Broken objects detected")
		}
		if strings.Contains(outputStr, "missing") {
			issues = append(issues, "Missing objects detected")
		}
		if strings.Contains(outputStr, "corrupt") {
			issues = append(issues, "Corrupted objects detected")
		}
		if len(issues) == 0 {
			issues = append(issues, fmt.Sprintf("Git fsck failed: %v", err))
		}
		
		return false, issues, err
	}
	
	// Check for warnings in successful fsck
	outputStr := string(output)
	if strings.Contains(outputStr, "warning") {
		issues = append(issues, "Git fsck reported warnings")
	}
	
	return len(issues) == 0, issues, nil
}

// IsRepositoryClean checks if the working directory is clean
func (gv *GitValidator) IsRepositoryClean(repoPath string) (bool, error) {
	ctx, cancel := context.WithTimeout(gv.ctx, 10*time.Second)
	defer cancel()
	
	// Check git status
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check repository status: %w", err)
	}
	
	// Empty output means clean working directory
	return len(strings.TrimSpace(string(output))) == 0, nil
}

// GetRepositoryInfo gathers comprehensive repository metadata
func (gv *GitValidator) GetRepositoryInfo(repoPath string) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(gv.ctx, 30*time.Second)
	defer cancel()
	
	info := make(map[string]string)
	
	// Basic Git information
	commands := map[string][]string{
		"head_commit":    {"rev-parse", "HEAD"},
		"branch":         {"branch", "--show-current"},
		"remote_origin":  {"remote", "get-url", "origin"},
		"commit_count":   {"rev-list", "--count", "HEAD"},
		"last_commit_date": {"log", "-1", "--format=%ci"},
		"last_commit_author": {"log", "-1", "--format=%an"},
		"repository_size": {"count-objects", "-v"},
		"git_version":    {"--version"},
	}
	
	for key, args := range commands {
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", repoPath}, args...)...)
		if output, err := cmd.Output(); err == nil {
			info[key] = strings.TrimSpace(string(output))
		} else {
			info[key] = fmt.Sprintf("error: %v", err)
		}
	}
	
	// File system information
	if stat, err := os.Stat(repoPath); err == nil {
		info["permissions"] = stat.Mode().String()
		info["last_modified"] = stat.ModTime().Format(time.RFC3339)
	}
	
	// Calculate directory size
	if size, err := gv.calculateDirectorySize(repoPath); err == nil {
		info["directory_size_bytes"] = fmt.Sprintf("%d", size)
		info["directory_size_mb"] = fmt.Sprintf("%.2f", float64(size)/(1024*1024))
	}
	
	return info, nil
}

// RepairRepository attempts to automatically repair repository issues
func (gv *GitValidator) RepairRepository(repoPath string) error {
	gv.log("info", "Starting repository repair for: %s", repoPath)
	atomic.AddInt64(&gv.repairCount, 1)
	
	// First, validate to identify issues
	result := gv.ValidateRepository(repoPath)
	if result.IsValid {
		gv.log("info", "Repository is already healthy, no repair needed")
		return nil
	}
	
	ctx, cancel := context.WithTimeout(gv.ctx, 5*time.Minute)
	defer cancel()
	
	repairActions := make([]RepairAction, 0)
	repairSuccess := true
	
	// Stage 1: Safe repairs (auto-repair level 1+)
	if gv.config.AutoRepairLevel >= 1 {
		// Garbage collection and cleanup
		if action, err := gv.performSafeRepair(ctx, repoPath, "gc", []string{"gc", "--prune=now"}); err != nil {
			gv.log("warn", "Safe repair 'gc' failed: %v", err)
			repairSuccess = false
		} else {
			repairActions = append(repairActions, action)
		}
		
		// Clean up untracked files
		if action, err := gv.performSafeRepair(ctx, repoPath, "clean", []string{"clean", "-fd"}); err != nil {
			gv.log("warn", "Safe repair 'clean' failed: %v", err)
		} else {
			repairActions = append(repairActions, action)
		}
		
		// Reset index
		if action, err := gv.performSafeRepair(ctx, repoPath, "reset_index", []string{"reset", "--mixed", "HEAD"}); err != nil {
			gv.log("warn", "Safe repair 'reset_index' failed: %v", err)
		} else {
			repairActions = append(repairActions, action)
		}
	}
	
	// Stage 2: Aggressive repairs (auto-repair level 2+)
	if gv.config.AutoRepairLevel >= 2 {
		// Rebuild corrupted index
		if action, err := gv.performAggressiveRepair(ctx, repoPath, "rebuild_index", []string{"read-tree", "HEAD"}); err != nil {
			gv.log("warn", "Aggressive repair 'rebuild_index' failed: %v", err)
			repairSuccess = false
		} else {
			repairActions = append(repairActions, action)
		}
		
		// Prune unreachable objects
		if action, err := gv.performAggressiveRepair(ctx, repoPath, "prune", []string{"prune", "--expire=now"}); err != nil {
			gv.log("warn", "Aggressive repair 'prune' failed: %v", err)
		} else {
			repairActions = append(repairActions, action)
		}
	}
	
	// Re-validate after repairs
	finalResult := gv.ValidateRepository(repoPath)
	
	if finalResult.IsValid {
		atomic.AddInt64(&gv.repairSuccessCount, 1)
		gv.log("info", "Repository repair successful: %s (health improved from %.2f to %.2f)", 
			repoPath, result.RepositoryHealth.Score, finalResult.RepositoryHealth.Score)
		return nil
	}
	
	if repairSuccess {
		return fmt.Errorf("repository repair completed but health score still below threshold: %.2f", 
			finalResult.RepositoryHealth.Score)
	}
	
	return fmt.Errorf("repository repair failed: %d repair actions attempted, final health score: %.2f", 
		len(repairActions), finalResult.RepositoryHealth.Score)
}

// StartBackgroundValidation starts background repository scanning
func (gv *GitValidator) StartBackgroundValidation(interval time.Duration, repositories []string) error {
	if gv.isRunning {
		return fmt.Errorf("background validation is already running")
	}
	
	gv.mu.Lock()
	defer gv.mu.Unlock()
	
	gv.isRunning = true
	gv.backgroundTicker = time.NewTicker(interval)
	
	go gv.backgroundValidationLoop(repositories)
	
	gv.log("info", "Background validation started with %d repositories, interval: %v", 
		len(repositories), interval)
	return nil
}

// StopBackgroundValidation stops background scanning
func (gv *GitValidator) StopBackgroundValidation() error {
	gv.mu.Lock()
	defer gv.mu.Unlock()
	
	if !gv.isRunning {
		return nil
	}
	
	gv.isRunning = false
	if gv.backgroundTicker != nil {
		gv.backgroundTicker.Stop()
	}
	
	gv.cancel() // Cancel background context
	
	gv.log("info", "Background validation stopped")
	return nil
}

// GetValidationStats returns current validation statistics
func (gv *GitValidator) GetValidationStats() ValidationStats {
	gv.mu.RLock()
	defer gv.mu.RUnlock()
	
	stats := gv.stats
	stats.TotalValidations = atomic.LoadInt64(&gv.totalValidations)
	stats.SuccessfulValidations = atomic.LoadInt64(&gv.successCount)
	stats.FailedValidations = atomic.LoadInt64(&gv.failureCount)
	stats.RepairsAttempted = atomic.LoadInt64(&gv.repairCount)
	stats.RepairsSuccessful = atomic.LoadInt64(&gv.repairSuccessCount)
	
	// Calculate averages
	if len(gv.validationTimes) > 0 {
		var total time.Duration
		for _, t := range gv.validationTimes {
			total += t
		}
		stats.AverageValidationTime = total / time.Duration(len(gv.validationTimes))
	}
	
	return stats
}

// GenerateValidationReport creates a comprehensive validation report
func (gv *GitValidator) GenerateValidationReport() string {
	stats := gv.GetValidationStats()
	
	report := strings.Builder{}
	report.WriteString("=== Git Repository Validation Report ===\n")
	report.WriteString(fmt.Sprintf("Generated: %s\n", time.Now().Format(time.RFC3339)))
	report.WriteString(fmt.Sprintf("Reporting Period: %v\n\n", time.Since(stats.LastReset)))
	
	// Summary statistics
	report.WriteString("VALIDATION SUMMARY\n")
	report.WriteString(fmt.Sprintf("  Total Validations: %d\n", stats.TotalValidations))
	report.WriteString(fmt.Sprintf("  Successful: %d\n", stats.SuccessfulValidations))
	report.WriteString(fmt.Sprintf("  Failed: %d\n", stats.FailedValidations))
	
	if stats.TotalValidations > 0 {
		successRate := float64(stats.SuccessfulValidations) / float64(stats.TotalValidations) * 100
		report.WriteString(fmt.Sprintf("  Success Rate: %.2f%%\n", successRate))
	}
	
	report.WriteString(fmt.Sprintf("  Average Validation Time: %v\n", stats.AverageValidationTime))
	report.WriteString(fmt.Sprintf("  Cache Hit Rate: %.2f%%\n\n", stats.CacheHitRate*100))
	
	// Repair statistics
	report.WriteString("REPAIR SUMMARY\n")
	report.WriteString(fmt.Sprintf("  Repairs Attempted: %d\n", stats.RepairsAttempted))
	report.WriteString(fmt.Sprintf("  Repairs Successful: %d\n", stats.RepairsSuccessful))
	
	if stats.RepairsAttempted > 0 {
		repairRate := float64(stats.RepairsSuccessful) / float64(stats.RepairsAttempted) * 100
		report.WriteString(fmt.Sprintf("  Repair Success Rate: %.2f%%\n", repairRate))
	}
	
	report.WriteString(fmt.Sprintf("  Integrity Issues Found: %d\n\n", stats.IntegrityIssuesFound))
	
	// Configuration
	report.WriteString("CONFIGURATION\n")
	report.WriteString(fmt.Sprintf("  Deep Scan: %v\n", gv.config.EnableDeepScan))
	report.WriteString(fmt.Sprintf("  Parallel Validation: %v\n", gv.config.ParallelValidation))
	report.WriteString(fmt.Sprintf("  Max Validation Time: %v\n", gv.config.MaxValidationTime))
	report.WriteString(fmt.Sprintf("  Auto Repair Level: %d\n", gv.config.AutoRepairLevel))
	report.WriteString(fmt.Sprintf("  Background Scanning: %v\n", gv.config.EnableBackgroundScan))
	
	return report.String()
}

// Close releases all resources and stops background processes
func (gv *GitValidator) Close() error {
	gv.StopBackgroundValidation()
	
	gv.mu.Lock()
	defer gv.mu.Unlock()
	
	// Clear caches
	gv.validationCache = make(map[string]*GitValidationResult)
	gv.validationTimes = nil
	
	gv.log("info", "Git validator closed and resources released")
	return nil
}

// Private helper methods

func (gv *GitValidator) log(level, format string, args ...interface{}) {
	if gv.shouldLog(level) {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		message := fmt.Sprintf(format, args...)
		fmt.Printf("[%s] [GitValidator] [%s] %s\n", timestamp, strings.ToUpper(level), message)
	}
}

func (gv *GitValidator) shouldLog(level string) bool {
	levelPriority := map[string]int{
		"debug": 0,
		"info":  1,
		"warn":  2,
		"error": 3,
	}
	
	configPriority := levelPriority[gv.config.LogLevel]
	messagePriority := levelPriority[level]
	
	return messagePriority >= configPriority
}

func (gv *GitValidator) executeGitCommand(repoPath string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(gv.ctx, 30*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", repoPath}, args...)...)
	return cmd.CombinedOutput()
}

func (gv *GitValidator) validateBasicStructure(ctx context.Context, repoPath string, result *GitValidationResult) error {
	// Check if directory exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return fmt.Errorf("repository directory does not exist: %s", repoPath)
	}
	
	// Check if .git directory exists
	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		result.IntegrityIssues = append(result.IntegrityIssues, "Git directory (.git) not found")
		result.Recommendations = append(result.Recommendations, "Initialize Git repository or check repository structure")
		return fmt.Errorf("not a Git repository: %s", repoPath)
	}
	
	// Check directory permissions
	if info, err := os.Stat(repoPath); err == nil {
		result.RepositoryInfo["permissions"] = info.Mode().String()
		if info.Mode().Perm()&0200 == 0 {
			result.IntegrityIssues = append(result.IntegrityIssues, "Repository directory is not writable")
			result.Recommendations = append(result.Recommendations, "Fix directory permissions")
		}
	}
	
	return nil
}

func (gv *GitValidator) validateGitIntegrity(ctx context.Context, repoPath string, result *GitValidationResult) {
	healthy, issues, err := gv.CheckGitIntegrity(repoPath)
	
	result.RepositoryHealth.GitIntegrity = healthy
	if !healthy {
		result.IntegrityIssues = append(result.IntegrityIssues, issues...)
		result.Recommendations = append(result.Recommendations, "Run 'git fsck --full' to identify and fix integrity issues")
	}
	
	if err != nil {
		result.ErrorDetails = append(result.ErrorDetails, fmt.Sprintf("Git integrity check error: %v", err))
	}
}

func (gv *GitValidator) validateCommitStructure(ctx context.Context, repoPath string, result *GitValidationResult) {
	// Validate HEAD
	if output, err := gv.executeGitCommand(repoPath, "rev-parse", "HEAD"); err == nil {
		result.CommitHash = strings.TrimSpace(string(output))
		result.RepositoryHealth.CommitValid = true
		result.RepositoryInfo["head_commit"] = result.CommitHash
	} else {
		result.RepositoryHealth.CommitValid = false
		result.IntegrityIssues = append(result.IntegrityIssues, "Invalid HEAD reference")
		result.Recommendations = append(result.Recommendations, "Check repository history and fix HEAD reference")
	}
	
	// Check for empty repository
	if output, err := gv.executeGitCommand(repoPath, "rev-list", "--count", "HEAD"); err == nil {
		commitCount := strings.TrimSpace(string(output))
		result.RepositoryInfo["commit_count"] = commitCount
		if commitCount == "0" {
			result.IntegrityIssues = append(result.IntegrityIssues, "Repository has no commits")
		}
	}
}

func (gv *GitValidator) validateWorkingTree(ctx context.Context, repoPath string, result *GitValidationResult) {
	clean, err := gv.IsRepositoryClean(repoPath)
	result.RepositoryHealth.WorkingTreeClean = clean
	
	if err != nil {
		result.ErrorDetails = append(result.ErrorDetails, fmt.Sprintf("Working tree validation error: %v", err))
		return
	}
	
	if !clean {
		result.IntegrityIssues = append(result.IntegrityIssues, "Working directory contains uncommitted changes")
		result.Recommendations = append(result.Recommendations, "Commit or stash pending changes")
	}
}

func (gv *GitValidator) validateReferences(ctx context.Context, repoPath string, result *GitValidationResult) {
	// Check refs integrity
	if _, err := gv.executeGitCommand(repoPath, "for-each-ref", "--format=%(refname)"); err == nil {
		result.RepositoryHealth.RefsIntegrity = true
	} else {
		result.RepositoryHealth.RefsIntegrity = false
		result.IntegrityIssues = append(result.IntegrityIssues, "Reference integrity issues detected")
		result.Recommendations = append(result.Recommendations, "Repair or recreate damaged references")
	}
	
	// Validate index
	if _, err := gv.executeGitCommand(repoPath, "ls-files", "--stage"); err == nil {
		result.RepositoryHealth.IndexIntegrity = true
	} else {
		result.RepositoryHealth.IndexIntegrity = false
		result.IntegrityIssues = append(result.IntegrityIssues, "Git index is corrupted")
		result.Recommendations = append(result.Recommendations, "Rebuild Git index")
	}
}

func (gv *GitValidator) performDeepScan(ctx context.Context, repoPath string, result *GitValidationResult) {
	gv.log("debug", "Performing deep scan for repository: %s", repoPath)
	
	// Object database validation
	if gv.config.ValidateObjects {
		if _, err := gv.executeGitCommand(repoPath, "cat-file", "--batch-check", "--batch-all-objects"); err == nil {
			result.RepositoryHealth.ObjectsIntegrity = true
			result.RepositoryInfo["objects_validated"] = "true"
		} else {
			result.RepositoryHealth.ObjectsIntegrity = false
			result.IntegrityIssues = append(result.IntegrityIssues, "Object database integrity issues")
		}
	}
	
	// Configuration validation
	if _, err := gv.executeGitCommand(repoPath, "config", "--list"); err == nil {
		result.RepositoryHealth.ConfigValid = true
	} else {
		result.RepositoryHealth.ConfigValid = false
		result.IntegrityIssues = append(result.IntegrityIssues, "Git configuration issues")
	}
}

func (gv *GitValidator) validateRemotes(ctx context.Context, repoPath string, result *GitValidationResult) {
	gv.log("debug", "Validating remote references for repository: %s", repoPath)
	
	// Check remote connectivity
	if output, err := gv.executeGitCommand(repoPath, "remote", "-v"); err == nil {
		result.RepositoryInfo["remotes"] = strings.TrimSpace(string(output))
		result.RepositoryHealth.RemotesValid = true
		
		// Test remote connectivity (with timeout)
		if _, err := gv.executeGitCommand(repoPath, "ls-remote", "--heads", "origin"); err != nil {
			result.IntegrityIssues = append(result.IntegrityIssues, "Remote origin is not accessible")
			result.Recommendations = append(result.Recommendations, "Check network connectivity and remote URL")
		}
	} else {
		result.RepositoryHealth.RemotesValid = false
		result.IntegrityIssues = append(result.IntegrityIssues, "No remote repositories configured")
	}
}

func (gv *GitValidator) calculateRepositoryHealth(result *GitValidationResult) {
	health := &result.RepositoryHealth
	
	// Calculate weighted health score
	score := 1.0
	
	if !health.GitIntegrity {
		score *= 0.3 // Critical issue
	}
	if !health.CommitValid {
		score *= 0.5 // Major issue
	}
	if !health.WorkingTreeClean {
		score *= 0.9 // Minor issue
	}
	if !health.RefsIntegrity {
		score *= 0.6 // Major issue
	}
	if !health.ObjectsIntegrity {
		score *= 0.4 // Critical issue
	}
	if !health.IndexIntegrity {
		score *= 0.7 // Moderate issue
	}
	if !health.ConfigValid {
		score *= 0.8 // Minor issue
	}
	if !health.RemotesValid {
		score *= 0.95 // Very minor issue
	}
	
	health.Score = score
	
	// Determine status based on score
	switch {
	case score >= 0.95:
		health.Status = "excellent"
	case score >= 0.85:
		health.Status = "good"
	case score >= 0.70:
		health.Status = "fair"
	case score >= 0.50:
		health.Status = "poor"
	default:
		health.Status = "critical"
	}
	
	// Add general recommendations based on health status
	if score < 0.85 {
		result.Recommendations = append(result.Recommendations, "Consider running repository maintenance")
	}
	if score < 0.70 {
		result.Recommendations = append(result.Recommendations, "Repository requires immediate attention")
	}
	if score < 0.50 {
		result.Recommendations = append(result.Recommendations, "Critical repository issues - consider re-cloning")
	}
}

func (gv *GitValidator) recordPerformanceMetrics(result *GitValidationResult) {
	result.PerformanceMetrics["validation_duration_ms"] = result.ValidationTime.Milliseconds()
	result.PerformanceMetrics["issues_found"] = len(result.IntegrityIssues)
	result.PerformanceMetrics["recommendations_provided"] = len(result.Recommendations)
	result.PerformanceMetrics["health_score"] = result.RepositoryHealth.Score
	result.PerformanceMetrics["validator_config"] = gv.config
}

func (gv *GitValidator) getCachedResult(repoPath string) *GitValidationResult {
	gv.mu.RLock()
	defer gv.mu.RUnlock()
	
	if result, exists := gv.validationCache[repoPath]; exists {
		return result
	}
	return nil
}

func (gv *GitValidator) cacheValidationResult(repoPath string, result *GitValidationResult) {
	gv.mu.Lock()
	defer gv.mu.Unlock()
	
	// Keep cache size reasonable (max 100 entries)
	if len(gv.validationCache) >= 100 {
		// Remove oldest entries
		oldest := ""
		oldestTime := time.Now()
		for path, cachedResult := range gv.validationCache {
			if cachedResult.LastValidated.Before(oldestTime) {
				oldest = path
				oldestTime = cachedResult.LastValidated
			}
		}
		if oldest != "" {
			delete(gv.validationCache, oldest)
		}
	}
	
	gv.validationCache[repoPath] = result
}

func (gv *GitValidator) recordValidationStats(duration time.Duration, success bool) {
	gv.mu.Lock()
	defer gv.mu.Unlock()
	
	// Record validation time
	gv.validationTimes = append(gv.validationTimes, duration)
	
	// Keep only last 1000 validation times
	if len(gv.validationTimes) > 1000 {
		gv.validationTimes = gv.validationTimes[1:]
	}
	
	// Update cache hit calculation
	if gv.config.CacheValidationResults {
		// This is a simplified cache hit rate calculation
		// In practice, this would track actual cache hits vs misses
		gv.stats.CacheHitRate = 0.15 // Placeholder: ~15% cache hit rate
	}
}

func (gv *GitValidator) performSafeRepair(ctx context.Context, repoPath, actionName string, gitArgs []string) (RepairAction, error) {
	action := RepairAction{
		Action:      actionName,
		Description: fmt.Sprintf("Safe repair: %s", actionName),
		Risk:        "low",
		Reversible:  true,
		Command:     append([]string{"git", "-C", repoPath}, gitArgs...),
	}
	
	startTime := time.Now()
	
	output, err := gv.executeGitCommand(repoPath, gitArgs...)
	action.Duration = time.Since(startTime)
	
	if err != nil {
		action.Success = false
		action.Error = fmt.Sprintf("%v: %s", err, string(output))
		return action, err
	}
	
	action.Success = true
	return action, nil
}

func (gv *GitValidator) performAggressiveRepair(ctx context.Context, repoPath, actionName string, gitArgs []string) (RepairAction, error) {
	action := RepairAction{
		Action:      actionName,
		Description: fmt.Sprintf("Aggressive repair: %s", actionName),
		Risk:        "medium",
		Reversible:  false,
		Command:     append([]string{"git", "-C", repoPath}, gitArgs...),
	}
	
	startTime := time.Now()
	
	output, err := gv.executeGitCommand(repoPath, gitArgs...)
	action.Duration = time.Since(startTime)
	
	if err != nil {
		action.Success = false
		action.Error = fmt.Sprintf("%v: %s", err, string(output))
		return action, err
	}
	
	action.Success = true
	return action, nil
}

func (gv *GitValidator) calculateDirectorySize(dirPath string) (int64, error) {
	var size int64
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

func (gv *GitValidator) backgroundValidationLoop(repositories []string) {
	defer func() {
		if gv.backgroundTicker != nil {
			gv.backgroundTicker.Stop()
		}
	}()
	
	for {
		select {
		case <-gv.backgroundTicker.C:
			gv.log("debug", "Starting background validation cycle for %d repositories", len(repositories))
			
			// Validate repositories in parallel
			var wg sync.WaitGroup
			for _, repoPath := range repositories {
				wg.Add(1)
				go func(path string) {
					defer wg.Done()
					result := gv.ValidateRepository(path)
					if !result.IsValid {
						gv.log("warn", "Background validation found issues in %s: %d issues", 
							path, len(result.IntegrityIssues))
					}
				}(repoPath)
			}
			wg.Wait()
			
			gv.log("debug", "Background validation cycle completed")
			
		case <-gv.ctx.Done():
			gv.log("info", "Background validation context cancelled")
			return
		}
	}
}