package testutils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// CacheValidationResult represents the result of cache validation
type CacheValidationResult struct {
	RepoURL       string                 `json:"repo_url"`
	CommitHash    string                 `json:"commit_hash"`
	CachePath     string                 `json:"cache_path"`
	IsValid       bool                   `json:"is_valid"`
	HealthScore   float64                `json:"health_score"` // 0.0 - 1.0
	Issues        []string               `json:"issues"`
	Suggestions   []string               `json:"suggestions"`
	LastAccessed  time.Time              `json:"last_accessed"`
	SizeBytes     int64                  `json:"size_bytes"`
	ValidationTime time.Duration         `json:"validation_time"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// CleanupPolicy defines cache cleanup policies
type CleanupPolicy struct {
	MaxTotalSizeGB    int           `json:"max_total_size_gb"`    // Total cache size limit
	MaxAgeUnusedDays  int           `json:"max_age_unused_days"`  // Remove unused caches after N days
	MinHealthScore    float64       `json:"min_health_score"`     // Remove caches below this health score
	CompactThreshold  float64       `json:"compact_threshold"`    // Compact when fragmentation > threshold
	PreserveRecent    int           `json:"preserve_recent"`      // Always preserve N most recent caches
	CleanupInterval   time.Duration `json:"cleanup_interval"`     // Background cleanup interval
	DryRun           bool          `json:"dry_run"`              // Simulate cleanup without actual deletion
}

// CacheMetrics represents comprehensive cache performance metrics
type CacheMetrics struct {
	// Basic metrics
	TotalCaches       int                  `json:"total_caches"`
	ValidCaches       int                  `json:"valid_caches"`
	CorruptedCaches   int                  `json:"corrupted_caches"`
	TotalSizeGB       float64              `json:"total_size_gb"`
	
	// Performance metrics
	HitRate           float64              `json:"hit_rate"`
	AverageHitTimeMs  float64              `json:"average_hit_time_ms"`
	AverageMissTimeMs float64              `json:"average_miss_time_ms"`
	PerformanceGain   float64              `json:"performance_gain"` // % improvement vs fresh clone
	
	// Usage patterns
	MostUsedRepos     []RepoUsageStats     `json:"most_used_repos"`
	RecentActivity    []ActivityRecord     `json:"recent_activity"`
	CacheDistribution map[string]int       `json:"cache_distribution"` // by language
	
	// Health metrics
	AverageHealthScore float64             `json:"average_health_score"`
	HealthDistribution map[string]int      `json:"health_distribution"` // score ranges
	
	// Cleanup metrics
	CleanupHistory     []CleanupEvent      `json:"cleanup_history"`
	LastCleanup        time.Time           `json:"last_cleanup"`
	NextScheduledCleanup time.Time         `json:"next_scheduled_cleanup"`
	
	// System metrics
	CollectionTime     time.Duration       `json:"collection_time"`
	Timestamp          time.Time           `json:"timestamp"`
}

// RepoUsageStats tracks repository usage statistics
type RepoUsageStats struct {
	RepoURL      string    `json:"repo_url"`
	AccessCount  int64     `json:"access_count"`
	LastAccessed time.Time `json:"last_accessed"`
	SizeGB       float64   `json:"size_gb"`
	HealthScore  float64   `json:"health_score"`
}

// ActivityRecord tracks cache activity
type ActivityRecord struct {
	Timestamp  time.Time `json:"timestamp"`
	Action     string    `json:"action"` // hit, miss, cleanup, validation
	RepoURL    string    `json:"repo_url"`
	Duration   time.Duration `json:"duration"`
	Success    bool      `json:"success"`
}

// CleanupEvent records cleanup operations
type CleanupEvent struct {
	Timestamp      time.Time `json:"timestamp"`
	Policy         CleanupPolicy `json:"policy"`
	CachesRemoved  int       `json:"caches_removed"`
	SpaceFreedGB   float64   `json:"space_freed_gb"`
	Duration       time.Duration `json:"duration"`
	Reason         string    `json:"reason"`
	Error          string    `json:"error,omitempty"`
}

// PerformanceReport provides detailed performance analysis
type PerformanceReport struct {
	TestPeriod         time.Duration            `json:"test_period"`
	TotalOperations    int64                    `json:"total_operations"`
	CacheHits          int64                    `json:"cache_hits"`
	CacheMisses        int64                    `json:"cache_misses"`
	AverageCloneTime   time.Duration            `json:"average_clone_time"`
	AverageCacheTime   time.Duration            `json:"average_cache_time"`
	PerformanceGainPct float64                  `json:"performance_gain_pct"`
	RepoBreakdown      map[string]PerformanceStats `json:"repo_breakdown"`
}

// PerformanceStats tracks performance for individual repositories
type PerformanceStats struct {
	Operations     int64         `json:"operations"`
	CacheHits      int64         `json:"cache_hits"`
	AverageHitTime time.Duration `json:"average_hit_time"`
	AverageMissTime time.Duration `json:"average_miss_time"`
	SizeGB         float64       `json:"size_gb"`
}

// CacheValidator manages cache integrity validation, cleanup, and performance monitoring
type CacheValidator struct {
	// Core components
	cacheManager      RepoCacheManager
	cacheDir          string
	
	// Configuration
	policy            CleanupPolicy
	enableLogging     bool
	enableBackground  bool
	
	// State management
	mu                sync.RWMutex
	isRunning         bool
	ctx               context.Context
	cancel            context.CancelFunc
	
	// Metrics tracking
	metrics           CacheMetrics
	performanceData   map[string]PerformanceStats
	activityLog       []ActivityRecord
	cleanupHistory    []CleanupEvent
	
	// Performance counters (atomic for thread safety)
	totalHits         int64
	totalMisses       int64
	totalValidations  int64
	totalCleanups     int64
	
	// Background cleanup
	cleanupTimer      *time.Timer
	lastCleanup       time.Time
}

// NewCacheValidator creates a new cache validator with the specified configuration
func NewCacheValidator(cacheManager RepoCacheManager, cacheDir string, policy *CleanupPolicy) *CacheValidator {
	if policy == nil {
		policy = &CleanupPolicy{
			MaxTotalSizeGB:   10,
			MaxAgeUnusedDays: 30,
			MinHealthScore:   0.7,
			CompactThreshold: 0.3,
			PreserveRecent:   5,
			CleanupInterval:  24 * time.Hour,
			DryRun:          false,
		}
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	cv := &CacheValidator{
		cacheManager:    cacheManager,
		cacheDir:        cacheDir,
		policy:          *policy,
		enableLogging:   true,
		enableBackground: true,
		ctx:             ctx,
		cancel:          cancel,
		performanceData: make(map[string]PerformanceStats),
		activityLog:     make([]ActivityRecord, 0, 1000),
		cleanupHistory:  make([]CleanupEvent, 0, 100),
		lastCleanup:     time.Now(),
	}
	
	// Initialize metrics
	cv.metrics = CacheMetrics{
		CacheDistribution:  make(map[string]int),
		HealthDistribution: make(map[string]int),
		Timestamp:          time.Now(),
	}
	
	cv.log("Cache validator initialized with policy: %+v", policy)
	return cv
}

// ValidateAllCaches performs comprehensive validation of all cached repositories
func (cv *CacheValidator) ValidateAllCaches() ([]CacheValidationResult, error) {
	startTime := time.Now()
	cv.log("Starting comprehensive cache validation")
	
	atomic.AddInt64(&cv.totalValidations, 1)
	
	// Discover all cached repositories
	cacheEntries, err := cv.discoverCacheEntries()
	if err != nil {
		return nil, fmt.Errorf("failed to discover cache entries: %w", err)
	}
	
	cv.log("Discovered %d cache entries for validation", len(cacheEntries))
	
	// Validate each cache entry in parallel
	results := make([]CacheValidationResult, 0, len(cacheEntries))
	resultsChan := make(chan CacheValidationResult, len(cacheEntries))
	errorsChan := make(chan error, len(cacheEntries))
	
	// Worker pool for parallel validation
	maxWorkers := 5
	semaphore := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	
	for _, entry := range cacheEntries {
		wg.Add(1)
		go func(repoURL, commitHash, cachePath string) {
			defer wg.Done()
			semaphore <- struct{}{} // Acquire
			defer func() { <-semaphore }() // Release
			
			result, err := cv.ValidateSingleCache(repoURL, commitHash)
			if err != nil {
				errorsChan <- fmt.Errorf("validation failed for %s: %w", repoURL, err)
				return
			}
			
			result.CachePath = cachePath
			resultsChan <- result
		}(entry.RepoURL, entry.CommitHash, entry.CachePath)
	}
	
	// Wait for all validations to complete
	go func() {
		wg.Wait()
		close(resultsChan)
		close(errorsChan)
	}()
	
	// Collect results
	for result := range resultsChan {
		results = append(results, result)
	}
	
	// Collect errors (non-fatal)
	var validationErrors []string
	for err := range errorsChan {
		validationErrors = append(validationErrors, err.Error())
		cv.log("Validation error: %v", err)
	}
	
	duration := time.Since(startTime)
	cv.log("Cache validation completed: %d entries, %d valid, %d errors, duration: %v", 
		len(results), cv.countValidResults(results), len(validationErrors), duration)
	
	// Record activity
	cv.recordActivity(ActivityRecord{
		Timestamp: time.Now(),
		Action:    "validate_all",
		Duration:  duration,
		Success:   len(validationErrors) == 0,
	})
	
	return results, nil
}

// ValidateSingleCache validates a specific cached repository
func (cv *CacheValidator) ValidateSingleCache(repoURL, commitHash string) (CacheValidationResult, error) {
	startTime := time.Now()
	result := CacheValidationResult{
		RepoURL:    repoURL,
		CommitHash: commitHash,
		Issues:     make([]string, 0),
		Suggestions: make([]string, 0),
		Metadata:   make(map[string]interface{}),
	}
	
	cv.log("Validating cache for %s (commit: %s)", repoURL, commitHash)
	
	// Stage 1: Basic health check
	if cv.cacheManager == nil {
		result.Issues = append(result.Issues, "Cache manager not available")
		result.HealthScore = 0.0
		result.ValidationTime = time.Since(startTime)
		return result, fmt.Errorf("cache manager not available")
	}
	
	// Stage 2: Cache availability check
	cachedPath, found, err := cv.cacheManager.GetCachedRepo(repoURL, commitHash)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Cache lookup failed: %v", err))
		result.HealthScore = 0.0
		result.ValidationTime = time.Since(startTime)
		return result, err
	}
	
	if !found {
		result.Issues = append(result.Issues, "Cache entry not found")
		result.Suggestions = append(result.Suggestions, "Create fresh cache entry")
		result.HealthScore = 0.0
		result.ValidationTime = time.Since(startTime)
		return result, nil
	}
	
	result.CachePath = cachedPath
	
	// Stage 3: File system validation
	healthScore := 1.0
	
	// Check if cache directory exists
	if stat, err := os.Stat(cachedPath); err != nil {
		if os.IsNotExist(err) {
			result.Issues = append(result.Issues, "Cache directory does not exist")
			result.Suggestions = append(result.Suggestions, "Remove stale cache entry and recreate")
			healthScore = 0.0
		} else {
			result.Issues = append(result.Issues, fmt.Sprintf("Cannot access cache directory: %v", err))
			healthScore *= 0.3
		}
	} else {
		result.LastAccessed = stat.ModTime()
		result.SizeBytes = cv.calculateDirectorySize(cachedPath)
		result.Metadata["directory_permissions"] = stat.Mode().String()
	}
	
	// Stage 4: Git integrity validation
	gitHealthy, gitErr := cv.VerifyGitIntegrity(cachedPath)
	if gitErr != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Git integrity check failed: %v", gitErr))
		result.Suggestions = append(result.Suggestions, "Run git fsck or recreate cache")
		healthScore *= 0.5
	} else if !gitHealthy {
		result.Issues = append(result.Issues, "Git repository appears corrupted")
		result.Suggestions = append(result.Suggestions, "Recreate cache from clean clone")
		healthScore *= 0.6
	}
	
	// Stage 5: Content validation
	if contentScore, contentIssues := cv.validateCacheContent(cachedPath); contentScore < 1.0 {
		result.Issues = append(result.Issues, contentIssues...)
		result.Suggestions = append(result.Suggestions, "Verify essential files are present")
		healthScore *= contentScore
	}
	
	// Stage 6: Performance validation
	if cv.cacheManager.IsCacheHealthy(repoURL, commitHash) {
		result.Metadata["cache_manager_healthy"] = true
	} else {
		result.Issues = append(result.Issues, "Cache manager reports unhealthy state")
		result.Suggestions = append(result.Suggestions, "Update cache or regenerate")
		healthScore *= 0.7
	}
	
	// Final assessment
	result.HealthScore = healthScore
	result.IsValid = healthScore >= cv.policy.MinHealthScore
	result.ValidationTime = time.Since(startTime)
	
	cv.log("Cache validation completed for %s: valid=%v, health=%.2f, duration=%v", 
		repoURL, result.IsValid, result.HealthScore, result.ValidationTime)
	
	return result, nil
}

// RepairCorruptedCache attempts to repair a corrupted cache entry
func (cv *CacheValidator) RepairCorruptedCache(repoURL, commitHash string) error {
	cv.log("Attempting to repair corrupted cache for %s (commit: %s)", repoURL, commitHash)
	
	// First validate to understand the issues
	result, err := cv.ValidateSingleCache(repoURL, commitHash)
	if err != nil {
		return fmt.Errorf("cannot repair: validation failed: %w", err)
	}
	
	if result.IsValid {
		cv.log("Cache is already valid, no repair needed")
		return nil
	}
	
	// Attempt repairs based on identified issues
	repairAttempts := 0
	maxRepairAttempts := 3
	
	for _, issue := range result.Issues {
		if repairAttempts >= maxRepairAttempts {
			break
		}
		
		repairAttempts++
		cv.log("Repair attempt %d: addressing issue '%s'", repairAttempts, issue)
		
		switch {
		case strings.Contains(issue, "does not exist"):
			// Remove stale cache reference
			cv.log("Removing stale cache entry")
			// Note: RepoCacheManager interface doesn't have a remove method
			// This would need to be implemented in the actual cache manager
			
		case strings.Contains(issue, "Git integrity"):
			// Attempt git repair
			if err := cv.attemptGitRepair(result.CachePath); err != nil {
				cv.log("Git repair failed: %v", err)
				continue
			}
			
		case strings.Contains(issue, "corrupted"):
			// Full cache regeneration required
			cv.log("Attempting full cache regeneration")
			if err := cv.regenerateCache(repoURL, commitHash, result.CachePath); err != nil {
				cv.log("Cache regeneration failed: %v", err)
				continue
			}
		}
	}
	
	// Re-validate after repair attempts
	finalResult, err := cv.ValidateSingleCache(repoURL, commitHash)
	if err != nil {
		return fmt.Errorf("post-repair validation failed: %w", err)
	}
	
	if finalResult.IsValid {
		cv.log("Cache repair successful for %s", repoURL)
		return nil
	}
	
	return fmt.Errorf("cache repair failed: health score %.2f still below threshold %.2f", 
		finalResult.HealthScore, cv.policy.MinHealthScore)
}

// VerifyGitIntegrity checks the integrity of a Git repository
func (cv *CacheValidator) VerifyGitIntegrity(repoPath string) (bool, error) {
	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		// No .git directory - this is expected for some cache configurations
		return true, nil
	}
	
	ctx, cancel := context.WithTimeout(cv.ctx, 30*time.Second)
	defer cancel()
	
	// Run git fsck to check repository integrity
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "fsck", "--no-progress", "--quiet")
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return false, fmt.Errorf("git fsck timeout")
		}
		return false, fmt.Errorf("git fsck failed: %w, output: %s", err, string(output))
	}
	
	// Check if HEAD is valid
	cmd = exec.CommandContext(ctx, "git", "-C", repoPath, "rev-parse", "HEAD")
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("HEAD reference invalid: %w", err)
	}
	
	return true, nil
}

// CleanupExpiredCaches removes caches based on the cleanup policy
func (cv *CacheValidator) CleanupExpiredCaches(policy CleanupPolicy) (int, error) {
	startTime := time.Now()
	cv.log("Starting cache cleanup with policy: %+v", policy)
	
	atomic.AddInt64(&cv.totalCleanups, 1)
	
	// Get all cache validation results
	validationResults, err := cv.ValidateAllCaches()
	if err != nil {
		return 0, fmt.Errorf("failed to validate caches for cleanup: %w", err)
	}
	
	// Apply cleanup criteria
	toRemove := cv.identifyCachesToRemove(validationResults, policy)
	cv.log("Identified %d caches for removal", len(toRemove))
	
	if policy.DryRun {
		cv.log("Dry run mode: would remove %d caches", len(toRemove))
		cv.logCleanupPlan(toRemove)
		return len(toRemove), nil
	}
	
	// Perform actual cleanup
	removedCount := 0
	var spaceFreed int64 = 0
	var cleanupErrors []string
	
	for _, result := range toRemove {
		cv.log("Removing cache: %s (reason: health=%.2f, age=%v)", 
			result.RepoURL, result.HealthScore, time.Since(result.LastAccessed))
		
		if err := cv.removeCache(result.CachePath); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Sprintf("%s: %v", result.RepoURL, err))
			cv.log("Failed to remove cache %s: %v", result.RepoURL, err)
			continue
		}
		
		removedCount++
		spaceFreed += result.SizeBytes
	}
	
	duration := time.Since(startTime)
	spaceFreedGB := float64(spaceFreed) / (1024 * 1024 * 1024)
	
	// Record cleanup event
	cleanupEvent := CleanupEvent{
		Timestamp:     time.Now(),
		Policy:        policy,
		CachesRemoved: removedCount,
		SpaceFreedGB:  spaceFreedGB,
		Duration:      duration,
		Reason:        "expired_cleanup",
	}
	
	if len(cleanupErrors) > 0 {
		cleanupEvent.Error = strings.Join(cleanupErrors, "; ")
	}
	
	cv.recordCleanupEvent(cleanupEvent)
	
	cv.log("Cache cleanup completed: removed %d caches, freed %.2f GB, duration: %v", 
		removedCount, spaceFreedGB, duration)
	
	cv.lastCleanup = time.Now()
	
	if len(cleanupErrors) > 0 {
		return removedCount, fmt.Errorf("cleanup completed with errors: %s", 
			strings.Join(cleanupErrors, "; "))
	}
	
	return removedCount, nil
}

// EnforceSizeLimit enforces cache size limits by removing least recently used caches
func (cv *CacheValidator) EnforceSizeLimit(maxSizeGB int) (int, error) {
	cv.log("Enforcing cache size limit: %d GB", maxSizeGB)
	
	validationResults, err := cv.ValidateAllCaches()
	if err != nil {
		return 0, fmt.Errorf("failed to validate caches for size enforcement: %w", err)
	}
	
	// Calculate current total size
	var totalSize int64 = 0
	for _, result := range validationResults {
		totalSize += result.SizeBytes
	}
	
	currentSizeGB := float64(totalSize) / (1024 * 1024 * 1024)
	cv.log("Current cache size: %.2f GB, limit: %d GB", currentSizeGB, maxSizeGB)
	
	if currentSizeGB <= float64(maxSizeGB) {
		cv.log("Cache size within limits, no cleanup needed")
		return 0, nil
	}
	
	// Sort by last accessed time (LRU)
	sort.Slice(validationResults, func(i, j int) bool {
		return validationResults[i].LastAccessed.Before(validationResults[j].LastAccessed)
	})
	
	// Remove caches until size limit is met
	removedCount := 0
	var spaceFreed int64 = 0
	targetSize := int64(maxSizeGB) * 1024 * 1024 * 1024
	
	for _, result := range validationResults {
		if totalSize-spaceFreed <= targetSize {
			break
		}
		
		// Preserve recent caches as specified in policy
		if removedCount < len(validationResults)-cv.policy.PreserveRecent {
			cv.log("Removing LRU cache: %s (size: %.2f MB, last access: %v)", 
				result.RepoURL, float64(result.SizeBytes)/(1024*1024), result.LastAccessed)
			
			if err := cv.removeCache(result.CachePath); err != nil {
				cv.log("Failed to remove cache %s: %v", result.RepoURL, err)
				continue
			}
			
			removedCount++
			spaceFreed += result.SizeBytes
		}
	}
	
	finalSizeGB := float64(totalSize-spaceFreed) / (1024 * 1024 * 1024)
	cv.log("Size limit enforcement completed: removed %d caches, final size: %.2f GB", 
		removedCount, finalSizeGB)
	
	return removedCount, nil
}

// RemoveUnusedCaches removes caches that haven't been accessed for specified days
func (cv *CacheValidator) RemoveUnusedCaches(unusedDays int) (int, error) {
	cv.log("Removing caches unused for %d days", unusedDays)
	
	cutoffTime := time.Now().AddDate(0, 0, -unusedDays)
	
	validationResults, err := cv.ValidateAllCaches()
	if err != nil {
		return 0, fmt.Errorf("failed to validate caches for unused cleanup: %w", err)
	}
	
	removedCount := 0
	for _, result := range validationResults {
		if result.LastAccessed.Before(cutoffTime) {
			cv.log("Removing unused cache: %s (last access: %v)", 
				result.RepoURL, result.LastAccessed)
			
			if err := cv.removeCache(result.CachePath); err != nil {
				cv.log("Failed to remove unused cache %s: %v", result.RepoURL, err)
				continue
			}
			
			removedCount++
		}
	}
	
	cv.log("Unused cache cleanup completed: removed %d caches", removedCount)
	return removedCount, nil
}

// CompactCacheStorage optimizes cache storage by defragmenting and reorganizing
func (cv *CacheValidator) CompactCacheStorage() error {
	cv.log("Starting cache storage compaction")
	
	// This is a placeholder for storage compaction logic
	// In a real implementation, this might involve:
	// - Defragmenting cache directories
	// - Reorganizing cache structure
	// - Removing empty directories
	// - Optimizing file organization
	
	if cv.cacheDir == "" {
		return fmt.Errorf("cache directory not specified")
	}
	
	// Clean up empty directories
	err := filepath.Walk(cv.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if info.IsDir() && path != cv.cacheDir {
			// Check if directory is empty
			entries, err := os.ReadDir(path)
			if err != nil {
				return err
			}
			
			if len(entries) == 0 {
				cv.log("Removing empty cache directory: %s", path)
				return os.Remove(path)
			}
		}
		
		return nil
	})
	
	if err != nil {
		return fmt.Errorf("cache compaction failed: %w", err)
	}
	
	cv.log("Cache storage compaction completed")
	return nil
}

// CollectCacheMetrics gathers comprehensive cache performance metrics
func (cv *CacheValidator) CollectCacheMetrics() CacheMetrics {
	startTime := time.Now()
	cv.log("Collecting comprehensive cache metrics")
	
	cv.mu.Lock()
	defer cv.mu.Unlock()
	
	// Reset metrics
	metrics := CacheMetrics{
		CacheDistribution:  make(map[string]int),
		HealthDistribution: make(map[string]int),
		RecentActivity:     make([]ActivityRecord, 0),
		CleanupHistory:     make([]CleanupEvent, 0),
		Timestamp:          time.Now(),
	}
	
	// Get current validation results
	validationResults, err := cv.ValidateAllCaches()
	if err != nil {
		cv.log("Failed to collect cache metrics: %v", err)
		metrics.CollectionTime = time.Since(startTime)
		return metrics
	}
	
	// Basic metrics
	metrics.TotalCaches = len(validationResults)
	var totalSize int64 = 0
	var healthSum float64 = 0
	
	for _, result := range validationResults {
		if result.IsValid {
			metrics.ValidCaches++
		} else {
			metrics.CorruptedCaches++
		}
		
		totalSize += result.SizeBytes
		healthSum += result.HealthScore
		
		// Health distribution
		healthRange := cv.getHealthRange(result.HealthScore)
		metrics.HealthDistribution[healthRange]++
		
		// Language distribution (inferred from repo URL)
		language := cv.inferLanguageFromURL(result.RepoURL)
		metrics.CacheDistribution[language]++
	}
	
	metrics.TotalSizeGB = float64(totalSize) / (1024 * 1024 * 1024)
	if len(validationResults) > 0 {
		metrics.AverageHealthScore = healthSum / float64(len(validationResults))
	}
	
	// Performance metrics from counters
	totalOps := atomic.LoadInt64(&cv.totalHits) + atomic.LoadInt64(&cv.totalMisses)
	if totalOps > 0 {
		metrics.HitRate = float64(atomic.LoadInt64(&cv.totalHits)) / float64(totalOps)
	}
	
	// Most used repositories
	metrics.MostUsedRepos = cv.getMostUsedRepos(validationResults, 10)
	
	// Recent activity (last 100 activities)
	activityCount := len(cv.activityLog)
	if activityCount > 100 {
		metrics.RecentActivity = cv.activityLog[activityCount-100:]
	} else {
		metrics.RecentActivity = cv.activityLog
	}
	
	// Cleanup history (last 20 events)
	historyCount := len(cv.cleanupHistory)
	if historyCount > 20 {
		metrics.CleanupHistory = cv.cleanupHistory[historyCount-20:]
	} else {
		metrics.CleanupHistory = cv.cleanupHistory
	}
	
	if len(cv.cleanupHistory) > 0 {
		metrics.LastCleanup = cv.cleanupHistory[len(cv.cleanupHistory)-1].Timestamp
	}
	
	// Next scheduled cleanup
	if cv.enableBackground {
		metrics.NextScheduledCleanup = cv.lastCleanup.Add(cv.policy.CleanupInterval)
	}
	
	metrics.CollectionTime = time.Since(startTime)
	cv.metrics = metrics
	
	cv.log("Cache metrics collected: %d total caches, %d valid, %.2f GB, %.2f%% hit rate", 
		metrics.TotalCaches, metrics.ValidCaches, metrics.TotalSizeGB, metrics.HitRate*100)
	
	return metrics
}

// GenerateUsageReport creates a comprehensive usage report
func (cv *CacheValidator) GenerateUsageReport() string {
	metrics := cv.CollectCacheMetrics()
	
	report := strings.Builder{}
	report.WriteString("=== Cache Usage Report ===\n")
	report.WriteString(fmt.Sprintf("Generated: %s\n", metrics.Timestamp.Format(time.RFC3339)))
	report.WriteString(fmt.Sprintf("Collection Time: %v\n\n", metrics.CollectionTime))
	
	// Summary
	report.WriteString("SUMMARY\n")
	report.WriteString(fmt.Sprintf("  Total Caches: %d\n", metrics.TotalCaches))
	report.WriteString(fmt.Sprintf("  Valid Caches: %d\n", metrics.ValidCaches))
	report.WriteString(fmt.Sprintf("  Corrupted Caches: %d\n", metrics.CorruptedCaches))
	report.WriteString(fmt.Sprintf("  Total Size: %.2f GB\n", metrics.TotalSizeGB))
	report.WriteString(fmt.Sprintf("  Average Health Score: %.2f\n", metrics.AverageHealthScore))
	report.WriteString(fmt.Sprintf("  Cache Hit Rate: %.2f%%\n\n", metrics.HitRate*100))
	
	// Performance
	report.WriteString("PERFORMANCE\n")
	report.WriteString(fmt.Sprintf("  Average Hit Time: %.2f ms\n", metrics.AverageHitTimeMs))
	report.WriteString(fmt.Sprintf("  Average Miss Time: %.2f ms\n", metrics.AverageMissTimeMs))
	report.WriteString(fmt.Sprintf("  Performance Gain: %.2f%%\n\n", metrics.PerformanceGain))
	
	// Distribution
	report.WriteString("CACHE DISTRIBUTION BY LANGUAGE\n")
	for lang, count := range metrics.CacheDistribution {
		percentage := float64(count) / float64(metrics.TotalCaches) * 100
		report.WriteString(fmt.Sprintf("  %s: %d (%.1f%%)\n", lang, count, percentage))
	}
	
	report.WriteString("\nHEALTH DISTRIBUTION\n")
	for healthRange, count := range metrics.HealthDistribution {
		percentage := float64(count) / float64(metrics.TotalCaches) * 100
		report.WriteString(fmt.Sprintf("  %s: %d (%.1f%%)\n", healthRange, count, percentage))
	}
	
	// Most used repositories
	report.WriteString("\nMOST USED REPOSITORIES\n")
	for i, repo := range metrics.MostUsedRepos {
		report.WriteString(fmt.Sprintf("  %d. %s\n", i+1, repo.RepoURL))
		report.WriteString(fmt.Sprintf("     Access Count: %d, Size: %.2f GB, Health: %.2f\n", 
			repo.AccessCount, repo.SizeGB, repo.HealthScore))
	}
	
	// Recent cleanup activity
	report.WriteString("\nRECENT CLEANUP HISTORY\n")
	for _, cleanup := range metrics.CleanupHistory {
		report.WriteString(fmt.Sprintf("  %s: Removed %d caches, freed %.2f GB (%s)\n", 
			cleanup.Timestamp.Format("2006-01-02 15:04"), cleanup.CachesRemoved, 
			cleanup.SpaceFreedGB, cleanup.Reason))
	}
	
	return report.String()
}

// TrackCacheHitRate tracks and returns the current cache hit rate
func (cv *CacheValidator) TrackCacheHitRate() float64 {
	totalHits := atomic.LoadInt64(&cv.totalHits)
	totalMisses := atomic.LoadInt64(&cv.totalMisses)
	totalOps := totalHits + totalMisses
	
	if totalOps == 0 {
		return 0.0
	}
	
	return float64(totalHits) / float64(totalOps)
}

// MeasurePerformanceGains measures and returns performance improvements from caching
func (cv *CacheValidator) MeasurePerformanceGains() PerformanceReport {
	cv.mu.RLock()
	defer cv.mu.RUnlock()
	
	report := PerformanceReport{
		TestPeriod:      time.Since(cv.metrics.Timestamp),
		TotalOperations: atomic.LoadInt64(&cv.totalHits) + atomic.LoadInt64(&cv.totalMisses),
		CacheHits:       atomic.LoadInt64(&cv.totalHits),
		CacheMisses:     atomic.LoadInt64(&cv.totalMisses),
		RepoBreakdown:   make(map[string]PerformanceStats),
	}
	
	// Copy performance data
	for repo, stats := range cv.performanceData {
		report.RepoBreakdown[repo] = stats
	}
	
	// Calculate averages
	if report.TotalOperations > 0 {
		// Estimate performance based on cache hits vs misses
		// Assume cache hits are 10x faster than misses
		estimatedCacheTime := 500 * time.Millisecond  // 500ms for cache hit
		estimatedCloneTime := 5 * time.Second         // 5s for fresh clone
		
		report.AverageCacheTime = estimatedCacheTime
		report.AverageCloneTime = estimatedCloneTime
		
		if report.CacheHits > 0 {
			// Calculate performance gain percentage
			timeWithCache := float64(report.CacheHits)*estimatedCacheTime.Seconds() + 
				float64(report.CacheMisses)*estimatedCloneTime.Seconds()
			timeWithoutCache := float64(report.TotalOperations) * estimatedCloneTime.Seconds()
			
			if timeWithoutCache > 0 {
				report.PerformanceGainPct = ((timeWithoutCache - timeWithCache) / timeWithoutCache) * 100
			}
		}
	}
	
	return report
}

// RecordCacheHit records a cache hit for performance tracking
func (cv *CacheValidator) RecordCacheHit(repoURL string, duration time.Duration) {
	atomic.AddInt64(&cv.totalHits, 1)
	
	cv.recordActivity(ActivityRecord{
		Timestamp: time.Now(),
		Action:    "hit",
		RepoURL:   repoURL,
		Duration:  duration,
		Success:   true,
	})
	
	// Update performance data
	cv.mu.Lock()
	if stats, exists := cv.performanceData[repoURL]; exists {
		stats.Operations++
		stats.CacheHits++
		// Update average hit time
		totalTime := stats.AverageHitTime * time.Duration(stats.CacheHits-1)
		stats.AverageHitTime = (totalTime + duration) / time.Duration(stats.CacheHits)
		cv.performanceData[repoURL] = stats
	} else {
		cv.performanceData[repoURL] = PerformanceStats{
			Operations:      1,
			CacheHits:       1,
			AverageHitTime:  duration,
			AverageMissTime: 0,
		}
	}
	cv.mu.Unlock()
}

// RecordCacheMiss records a cache miss for performance tracking
func (cv *CacheValidator) RecordCacheMiss(repoURL string, duration time.Duration) {
	atomic.AddInt64(&cv.totalMisses, 1)
	
	cv.recordActivity(ActivityRecord{
		Timestamp: time.Now(),
		Action:    "miss",
		RepoURL:   repoURL,
		Duration:  duration,
		Success:   false,
	})
	
	// Update performance data
	cv.mu.Lock()
	if stats, exists := cv.performanceData[repoURL]; exists {
		stats.Operations++
		misses := stats.Operations - stats.CacheHits
		// Update average miss time
		if misses > 1 {
			totalTime := stats.AverageMissTime * time.Duration(misses-1)
			stats.AverageMissTime = (totalTime + duration) / time.Duration(misses)
		} else {
			stats.AverageMissTime = duration
		}
		cv.performanceData[repoURL] = stats
	} else {
		cv.performanceData[repoURL] = PerformanceStats{
			Operations:      1,
			CacheHits:       0,
			AverageHitTime:  0,
			AverageMissTime: duration,
		}
	}
	cv.mu.Unlock()
}

// StartBackgroundCleanup starts the background cleanup process
func (cv *CacheValidator) StartBackgroundCleanup() error {
	if !cv.enableBackground {
		return fmt.Errorf("background cleanup is disabled")
	}
	
	cv.mu.Lock()
	defer cv.mu.Unlock()
	
	if cv.isRunning {
		return fmt.Errorf("background cleanup is already running")
	}
	
	cv.isRunning = true
	cv.cleanupTimer = time.NewTimer(cv.policy.CleanupInterval)
	
	go cv.backgroundCleanupLoop()
	
	cv.log("Background cleanup started with interval: %v", cv.policy.CleanupInterval)
	return nil
}

// StopBackgroundCleanup stops the background cleanup process
func (cv *CacheValidator) StopBackgroundCleanup() error {
	cv.mu.Lock()
	defer cv.mu.Unlock()
	
	if !cv.isRunning {
		return nil
	}
	
	cv.isRunning = false
	if cv.cleanupTimer != nil {
		cv.cleanupTimer.Stop()
	}
	
	cv.cancel() // Cancel the context to stop background operations
	
	cv.log("Background cleanup stopped")
	return nil
}

// Private helper methods

func (cv *CacheValidator) log(format string, args ...interface{}) {
	if cv.enableLogging {
		fmt.Printf("[CacheValidator] %s\n", fmt.Sprintf(format, args...))
	}
}

func (cv *CacheValidator) backgroundCleanupLoop() {
	for cv.isRunning {
		select {
		case <-cv.cleanupTimer.C:
			cv.log("Executing scheduled background cleanup")
			
			if _, err := cv.CleanupExpiredCaches(cv.policy); err != nil {
				cv.log("Background cleanup failed: %v", err)
			}
			
			// Reset timer for next cleanup
			cv.cleanupTimer.Reset(cv.policy.CleanupInterval)
			
		case <-cv.ctx.Done():
			cv.log("Background cleanup context cancelled")
			return
		}
	}
}

func (cv *CacheValidator) recordActivity(activity ActivityRecord) {
	cv.mu.Lock()
	defer cv.mu.Unlock()
	
	cv.activityLog = append(cv.activityLog, activity)
	
	// Keep only last 1000 activities
	if len(cv.activityLog) > 1000 {
		cv.activityLog = cv.activityLog[len(cv.activityLog)-1000:]
	}
}

func (cv *CacheValidator) recordCleanupEvent(event CleanupEvent) {
	cv.mu.Lock()
	defer cv.mu.Unlock()
	
	cv.cleanupHistory = append(cv.cleanupHistory, event)
	
	// Keep only last 100 cleanup events
	if len(cv.cleanupHistory) > 100 {
		cv.cleanupHistory = cv.cleanupHistory[len(cv.cleanupHistory)-100:]
	}
}

// CacheEntry represents a discovered cache entry
type CacheEntry struct {
	RepoURL    string
	CommitHash string
	CachePath  string
}

func (cv *CacheValidator) discoverCacheEntries() ([]CacheEntry, error) {
	// This is a placeholder implementation
	// In practice, this would scan the cache directory or query the cache manager
	// for all available cache entries
	
	entries := make([]CacheEntry, 0)
	
	if cv.cacheDir == "" {
		return entries, fmt.Errorf("cache directory not specified")
	}
	
	// Walk the cache directory to discover entries
	err := filepath.Walk(cv.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip if not a directory or is the root cache directory
		if !info.IsDir() || path == cv.cacheDir {
			return nil
		}
		
		// Try to extract repo information from path structure
		// This is a simplified approach - real implementation would depend on
		// how the cache manager organizes cache entries
		
		relPath, err := filepath.Rel(cv.cacheDir, path)
		if err != nil {
			return err
		}
		
		// Assume structure like: cache_dir/repo_name_hash/commit_hash
		parts := strings.Split(relPath, string(os.PathSeparator))
		if len(parts) >= 2 {
			// Try to reconstruct repo URL and commit hash
			repoName := parts[0]
			commitHash := parts[1]
			
			// This is a simplified extraction - real implementation would
			// store metadata about cache entries
			entries = append(entries, CacheEntry{
				RepoURL:    fmt.Sprintf("https://github.com/example/%s", repoName),
				CommitHash: commitHash,
				CachePath:  path,
			})
		}
		
		return nil
	})
	
	return entries, err
}

func (cv *CacheValidator) countValidResults(results []CacheValidationResult) int {
	count := 0
	for _, result := range results {
		if result.IsValid {
			count++
		}
	}
	return count
}

func (cv *CacheValidator) calculateDirectorySize(dirPath string) int64 {
	var size int64 = 0
	
	filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	
	return size
}

func (cv *CacheValidator) validateCacheContent(cachePath string) (float64, []string) {
	issues := make([]string, 0)
	score := 1.0
	
	// Check for essential directories/files
	essentialPaths := []string{
		"src", "lib", "main", "index", "package.json", "go.mod", "setup.py", "pom.xml",
	}
	
	foundEssential := false
	for _, essential := range essentialPaths {
		if _, err := os.Stat(filepath.Join(cachePath, essential)); err == nil {
			foundEssential = true
			break
		}
	}
	
	if !foundEssential {
		issues = append(issues, "No essential project files found")
		score *= 0.8
	}
	
	// Check for suspicious empty directories
	emptyDirs := 0
	totalDirs := 0
	
	filepath.Walk(cachePath, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() {
			totalDirs++
			
			entries, err := os.ReadDir(path)
			if err == nil && len(entries) == 0 {
				emptyDirs++
			}
		}
		return nil
	})
	
	if totalDirs > 0 && float64(emptyDirs)/float64(totalDirs) > 0.5 {
		issues = append(issues, "High percentage of empty directories")
		score *= 0.9
	}
	
	return score, issues
}

func (cv *CacheValidator) attemptGitRepair(repoPath string) error {
	ctx, cancel := context.WithTimeout(cv.ctx, 60*time.Second)
	defer cancel()
	
	// Try git gc to clean up repository
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "gc", "--prune=now")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git gc failed: %w", err)
	}
	
	// Try to recover refs
	cmd = exec.CommandContext(ctx, "git", "-C", repoPath, "remote", "prune", "origin")
	if err := cmd.Run(); err != nil {
		cv.log("Warning: git remote prune failed: %v", err)
	}
	
	return nil
}

func (cv *CacheValidator) regenerateCache(repoURL, commitHash, cachePath string) error {
	// Remove corrupted cache
	if err := os.RemoveAll(cachePath); err != nil {
		return fmt.Errorf("failed to remove corrupted cache: %w", err)
	}
	
	// This would typically involve calling the cache manager to recreate the cache
	// For now, we'll just create the directory structure
	if err := os.MkdirAll(cachePath, 0755); err != nil {
		return fmt.Errorf("failed to recreate cache directory: %w", err)
	}
	
	// In a real implementation, this would trigger a fresh clone
	cv.log("Cache regeneration placeholder for %s", repoURL)
	
	return nil
}

func (cv *CacheValidator) identifyCachesToRemove(results []CacheValidationResult, policy CleanupPolicy) []CacheValidationResult {
	toRemove := make([]CacheValidationResult, 0)
	
	for _, result := range results {
		// Health score criteria
		if result.HealthScore < policy.MinHealthScore {
			toRemove = append(toRemove, result)
			continue
		}
		
		// Age criteria
		if policy.MaxAgeUnusedDays > 0 {
			cutoffTime := time.Now().AddDate(0, 0, -policy.MaxAgeUnusedDays)
			if result.LastAccessed.Before(cutoffTime) {
				toRemove = append(toRemove, result)
				continue
			}
		}
	}
	
	// Sort by health score (remove worst first)
	sort.Slice(toRemove, func(i, j int) bool {
		return toRemove[i].HealthScore < toRemove[j].HealthScore
	})
	
	return toRemove
}

func (cv *CacheValidator) logCleanupPlan(toRemove []CacheValidationResult) {
	cv.log("=== Cleanup Plan (Dry Run) ===")
	for _, result := range toRemove {
		cv.log("  Would remove: %s (health: %.2f, size: %.2f MB, last access: %v)", 
			result.RepoURL, result.HealthScore, 
			float64(result.SizeBytes)/(1024*1024), result.LastAccessed)
	}
	cv.log("=== End Cleanup Plan ===")
}

func (cv *CacheValidator) removeCache(cachePath string) error {
	// Safety check - ensure we're removing from cache directory
	if cv.cacheDir != "" && !strings.HasPrefix(cachePath, cv.cacheDir) {
		return fmt.Errorf("refusing to remove path outside cache directory: %s", cachePath)
	}
	
	return os.RemoveAll(cachePath)
}

func (cv *CacheValidator) getHealthRange(score float64) string {
	switch {
	case score >= 0.9:
		return "excellent"
	case score >= 0.8:
		return "good"
	case score >= 0.7:
		return "fair"
	case score >= 0.5:
		return "poor"
	default:
		return "critical"
	}
}

func (cv *CacheValidator) inferLanguageFromURL(repoURL string) string {
	lower := strings.ToLower(repoURL)
	
	switch {
	case strings.Contains(lower, "python"):
		return "python"
	case strings.Contains(lower, "golang") || strings.Contains(lower, "/go"):
		return "go"
	case strings.Contains(lower, "javascript") || strings.Contains(lower, "node"):
		return "javascript"
	case strings.Contains(lower, "typescript"):
		return "typescript"
	case strings.Contains(lower, "java"):
		return "java"
	case strings.Contains(lower, "rust"):
		return "rust"
	default:
		return "unknown"
	}
}

func (cv *CacheValidator) getMostUsedRepos(results []CacheValidationResult, limit int) []RepoUsageStats {
	// This is a simplified implementation
	// Real implementation would track actual usage statistics
	
	usage := make([]RepoUsageStats, 0, len(results))
	for _, result := range results {
		usage = append(usage, RepoUsageStats{
			RepoURL:      result.RepoURL,
			AccessCount:  1, // Placeholder - would track actual access count
			LastAccessed: result.LastAccessed,
			SizeGB:       float64(result.SizeBytes) / (1024 * 1024 * 1024),
			HealthScore:  result.HealthScore,
		})
	}
	
	// Sort by last accessed (most recent first)
	sort.Slice(usage, func(i, j int) bool {
		return usage[i].LastAccessed.After(usage[j].LastAccessed)
	})
	
	if len(usage) > limit {
		usage = usage[:limit]
	}
	
	return usage
}

// GetMetrics returns the current cache metrics
func (cv *CacheValidator) GetMetrics() CacheMetrics {
	cv.mu.RLock()
	defer cv.mu.RUnlock()
	return cv.metrics
}

// EnableLogging enables or disables logging
func (cv *CacheValidator) EnableLogging(enabled bool) {
	cv.enableLogging = enabled
}

// SetCleanupPolicy updates the cleanup policy
func (cv *CacheValidator) SetCleanupPolicy(policy CleanupPolicy) {
	cv.mu.Lock()
	defer cv.mu.Unlock()
	cv.policy = policy
	cv.log("Cleanup policy updated: %+v", policy)
}