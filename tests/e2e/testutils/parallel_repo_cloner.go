package testutils

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// RepoCacheManager interface for repository caching (to be implemented by parallel task)
type RepoCacheManager interface {
	GetCachedRepo(repoURL, commitHash string) (string, bool, error)
	CacheRepository(repoURL, commitHash, repoPath string) error
	UpdateCachedRepo(repoURL, commitHash, repoPath string) error
	IsCacheHealthy(repoURL, commitHash string) bool
}

// CloneRequest represents a request to clone a repository
type CloneRequest struct {
	RepoURL     string
	CommitHash  string
	TargetDir   string
	Config      GenericRepoConfig
	RequestID   string
	Priority    int
}

// CloneResult represents the result of a clone operation
type CloneResult struct {
	RequestID   string
	RepoURL     string
	Success     bool
	RepoPath    string
	Error       error
	Duration    time.Duration
	CacheHit    bool
	RetryCount  int
	StartTime   time.Time
	EndTime     time.Time
}

// CloneProgress represents the current progress of parallel clone operations
type CloneProgress struct {
	TotalRequests    int
	CompletedCount   int
	SuccessCount     int
	FailureCount     int
	CacheHitCount    int
	ActiveWorkers    int
	QueuedRequests   int
	AverageDuration  time.Duration
	ElapsedTime      time.Duration
	EstimatedTimeLeft time.Duration
	CurrentOperations []string
}

// WorkerStatus represents the status of a worker
type WorkerStatus struct {
	WorkerID      int
	Active        bool
	CurrentRepo   string
	StartTime     time.Time
	OperationCount int
}

// ParallelRepoCloner manages parallel repository cloning operations
type ParallelRepoCloner struct {
	maxWorkers       int
	cacheManager     RepoCacheManager
	workers          []*worker
	requestQueue     chan CloneRequest
	resultQueue      chan CloneResult
	progressMutex    sync.RWMutex
	progress         CloneProgress
	workerStatuses   []*WorkerStatus
	ctx              context.Context
	cancel           context.CancelFunc
	startTime        time.Time
	enableLogging    bool
	retryMaxAttempts int
	retryBaseDelay   time.Duration
}

// worker represents a single clone worker
type worker struct {
	id           int
	cloner       *ParallelRepoCloner
	ctx          context.Context
	wg           *sync.WaitGroup
}

// NewParallelRepoCloner creates a new parallel repository cloner
func NewParallelRepoCloner(maxWorkers int, cacheManager RepoCacheManager) *ParallelRepoCloner {
	if maxWorkers <= 0 {
		maxWorkers = 3 // Default to 3 workers for safe concurrent git operations
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	cloner := &ParallelRepoCloner{
		maxWorkers:       maxWorkers,
		cacheManager:     cacheManager,
		workers:          make([]*worker, maxWorkers),
		requestQueue:     make(chan CloneRequest, maxWorkers*2),
		resultQueue:      make(chan CloneResult, maxWorkers*2),
		workerStatuses:   make([]*WorkerStatus, maxWorkers),
		ctx:              ctx,
		cancel:           cancel,
		enableLogging:    true,
		retryMaxAttempts: 3,
		retryBaseDelay:   time.Second,
	}
	
	// Initialize worker statuses
	for i := 0; i < maxWorkers; i++ {
		cloner.workerStatuses[i] = &WorkerStatus{
			WorkerID: i,
			Active:   false,
		}
	}
	
	return cloner
}

// CloneRepositoriesParallel performs parallel cloning of multiple repositories
func (prc *ParallelRepoCloner) CloneRepositoriesParallel(requests []CloneRequest) ([]CloneResult, error) {
	if len(requests) == 0 {
		return []CloneResult{}, nil
	}
	
	prc.log("Starting parallel clone of %d repositories with %d workers", len(requests), prc.maxWorkers)
	
	// Reset progress
	prc.progressMutex.Lock()
	prc.progress = CloneProgress{
		TotalRequests:     len(requests),
		CompletedCount:    0,
		SuccessCount:      0,
		FailureCount:      0,
		CacheHitCount:     0,
		ActiveWorkers:     0,
		QueuedRequests:    len(requests),
		CurrentOperations: make([]string, 0),
	}
	prc.startTime = time.Now()
	prc.progressMutex.Unlock()
	
	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < prc.maxWorkers; i++ {
		prc.workers[i] = &worker{
			id:     i,
			cloner: prc,
			ctx:    prc.ctx,
			wg:     &wg,
		}
		wg.Add(1)
		go prc.workers[i].run()
	}
	
	// Send requests to workers
	go func() {
		defer close(prc.requestQueue)
		for _, request := range requests {
			select {
			case prc.requestQueue <- request:
				// Request queued successfully
			case <-prc.ctx.Done():
				prc.log("Context cancelled while queueing requests")
				return
			}
		}
	}()
	
	// Collect results
	results := make([]CloneResult, 0, len(requests))
	resultMap := make(map[string]CloneResult)
	var resultMapMutex sync.RWMutex
	var resultWg sync.WaitGroup
	
	resultWg.Add(1)
	go func() {
		defer resultWg.Done()
		for result := range prc.resultQueue {
			prc.progressMutex.Lock()
			prc.progress.CompletedCount++
			prc.progress.QueuedRequests = len(requests) - prc.progress.CompletedCount
			
			if result.Success {
				prc.progress.SuccessCount++
			} else {
				prc.progress.FailureCount++
			}
			
			if result.CacheHit {
				prc.progress.CacheHitCount++
			}
			
			// Update average duration
			if prc.progress.CompletedCount > 0 {
				totalDuration := time.Since(prc.startTime)
				prc.progress.AverageDuration = totalDuration / time.Duration(prc.progress.CompletedCount)
				
				// Estimate time left
				if prc.progress.CompletedCount < len(requests) {
					remaining := len(requests) - prc.progress.CompletedCount
					prc.progress.EstimatedTimeLeft = prc.progress.AverageDuration * time.Duration(remaining)
				}
			}
			
			prc.progress.ElapsedTime = time.Since(prc.startTime)
			prc.progressMutex.Unlock()
			
			resultMapMutex.Lock()
			resultMap[result.RequestID] = result
			resultMapMutex.Unlock()
			
			prc.log("Completed clone %s: success=%v, cache_hit=%v, duration=%v", 
				result.RequestID, result.Success, result.CacheHit, result.Duration)
		}
	}()
	
	// Wait for all workers to complete
	wg.Wait()
	close(prc.resultQueue)
	
	// Wait for all results to be processed
	resultWg.Wait()
	
	// Collect results in original order
	for _, request := range requests {
		resultMapMutex.RLock()
		result, exists := resultMap[request.RequestID]
		resultMapMutex.RUnlock()
		
		if exists {
			results = append(results, result)
		} else {
			// Create error result for missing requests
			results = append(results, CloneResult{
				RequestID: request.RequestID,
				RepoURL:   request.RepoURL,
				Success:   false,
				Error:     fmt.Errorf("clone operation not completed"),
				Duration:  0,
			})
		}
	}
	
	prc.log("Parallel clone completed: %d/%d successful, %d cache hits, total time: %v", 
		prc.progress.SuccessCount, len(requests), prc.progress.CacheHitCount, time.Since(prc.startTime))
	
	return results, nil
}

// CloneSingleRepository performs a single repository clone with cache optimization
func (prc *ParallelRepoCloner) CloneSingleRepository(request CloneRequest) CloneResult {
	startTime := time.Now()
	result := CloneResult{
		RequestID: request.RequestID,
		RepoURL:   request.RepoURL,
		StartTime: startTime,
	}
	
	// Try cache first if cache manager is available
	if prc.cacheManager != nil {
		if cachedPath, found, err := prc.cacheManager.GetCachedRepo(request.RepoURL, request.CommitHash); err == nil && found {
			if prc.cacheManager.IsCacheHealthy(request.RepoURL, request.CommitHash) {
				// Use cached repository
				if err := prc.updateFromCache(cachedPath, request.TargetDir, request.CommitHash); err == nil {
					result.Success = true
					result.RepoPath = request.TargetDir
					result.CacheHit = true
					result.Duration = time.Since(startTime)
					result.EndTime = time.Now()
					prc.log("Cache hit for %s: %s", request.RepoURL, request.TargetDir)
					return result
				}
				prc.log("Cache update failed for %s, falling back to fresh clone: %v", request.RepoURL, err)
			}
		}
	}
	
	// Perform fresh clone with retry logic
	var lastErr error
	for attempt := 0; attempt < prc.retryMaxAttempts; attempt++ {
		if attempt > 0 {
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * prc.retryBaseDelay
			prc.log("Retrying clone %s (attempt %d/%d) after %v delay", request.RepoURL, attempt+1, prc.retryMaxAttempts, delay)
			
			select {
			case <-time.After(delay):
			case <-prc.ctx.Done():
				result.Error = prc.ctx.Err()
				result.Duration = time.Since(startTime)
				result.EndTime = time.Now()
				return result
			}
		}
		
		err := prc.performFreshClone(request)
		if err == nil {
			// Success - cache the result if cache manager available
			if prc.cacheManager != nil {
				if cacheErr := prc.cacheManager.CacheRepository(request.RepoURL, request.CommitHash, request.TargetDir); cacheErr != nil {
					prc.log("Failed to cache repository %s: %v", request.RepoURL, cacheErr)
				}
			}
			
			result.Success = true
			result.RepoPath = request.TargetDir
			result.CacheHit = false
			result.RetryCount = attempt
			result.Duration = time.Since(startTime)
			result.EndTime = time.Now()
			return result
		}
		
		lastErr = err
		
		// Check if we should retry (network errors, timeout, etc.)
		if !prc.shouldRetry(err) {
			break
		}
	}
	
	result.Success = false
	result.Error = lastErr
	result.RetryCount = prc.retryMaxAttempts
	result.Duration = time.Since(startTime)
	result.EndTime = time.Now()
	return result
}

// GetCloneProgress returns the current progress of clone operations
func (prc *ParallelRepoCloner) GetCloneProgress() CloneProgress {
	prc.progressMutex.RLock()
	defer prc.progressMutex.RUnlock()
	
	// Create a copy to avoid concurrent access issues
	progress := prc.progress
	progress.CurrentOperations = make([]string, len(prc.progress.CurrentOperations))
	copy(progress.CurrentOperations, prc.progress.CurrentOperations)
	
	// Count active workers
	activeCount := 0
	for _, status := range prc.workerStatuses {
		if status.Active {
			activeCount++
		}
	}
	progress.ActiveWorkers = activeCount
	
	return progress
}

// CancelAll cancels all ongoing clone operations
func (prc *ParallelRepoCloner) CancelAll() error {
	prc.log("Cancelling all clone operations")
	prc.cancel()
	return nil
}

// Private helper methods

func (prc *ParallelRepoCloner) log(format string, args ...interface{}) {
	if prc.enableLogging {
		fmt.Printf("[ParallelRepoCloner] %s\n", fmt.Sprintf(format, args...))
	}
}

func (prc *ParallelRepoCloner) updateFromCache(cachedPath, targetDir, commitHash string) error {
	// Create target directory
	if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}
	
	// Remove existing target if it exists
	if _, err := os.Stat(targetDir); err == nil {
		if err := os.RemoveAll(targetDir); err != nil {
			return fmt.Errorf("failed to remove existing target: %w", err)
		}
	}
	
	// Copy from cache with hard links for efficiency
	cmd := exec.CommandContext(prc.ctx, "cp", "-al", cachedPath, targetDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to copy from cache: %w, output: %s", err, string(output))
	}
	
	// Update to specific commit if needed
	if commitHash != "" {
		cmd = exec.CommandContext(prc.ctx, "git", "-C", targetDir, "fetch", "origin")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to fetch updates: %w, output: %s", err, string(output))
		}
		
		cmd = exec.CommandContext(prc.ctx, "git", "-C", targetDir, "reset", "--hard", commitHash)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to reset to commit: %w, output: %s", err, string(output))
		}
	}
	
	return nil
}

func (prc *ParallelRepoCloner) performFreshClone(request CloneRequest) error {
	// Create target directory
	if err := os.MkdirAll(filepath.Dir(request.TargetDir), 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}
	
	// Remove existing target if it exists
	if _, err := os.Stat(request.TargetDir); err == nil {
		if err := os.RemoveAll(request.TargetDir); err != nil {
			return fmt.Errorf("failed to remove existing target: %w", err)
		}
	}
	
	// Clone repository
	var cmd *exec.Cmd
	if request.CommitHash != "" {
		// Full clone needed for specific commit checkout
		cmd = exec.CommandContext(prc.ctx, "git", "clone", request.RepoURL, request.TargetDir)
	} else {
		// Use shallow clone for performance when no specific commit needed
		cmd = exec.CommandContext(prc.ctx, "git", "clone", "--depth=1", request.RepoURL, request.TargetDir)
	}
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(request.TargetDir) // Clean up failed clone
		return fmt.Errorf("git clone failed: %w, output: %s", err, string(output))
	}
	
	// Checkout specific commit if specified
	if request.CommitHash != "" {
		cmd = exec.CommandContext(prc.ctx, "git", "-C", request.TargetDir, "checkout", request.CommitHash)
		output, err = cmd.CombinedOutput()
		if err != nil {
			os.RemoveAll(request.TargetDir) // Clean up failed checkout
			return fmt.Errorf("git checkout %s failed: %w, output: %s", request.CommitHash, err, string(output))
		}
	}
	
	// Remove .git directory if not preserving
	if !request.Config.PreserveGitDir {
		gitDir := filepath.Join(request.TargetDir, ".git")
		if err := os.RemoveAll(gitDir); err != nil {
			prc.log("Warning: failed to remove .git directory: %v", err)
		}
	}
	
	return nil
}

func (prc *ParallelRepoCloner) shouldRetry(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := strings.ToLower(err.Error())
	
	// Retry on network-related errors
	retryableErrors := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"network is unreachable",
		"temporary failure",
		"ssl",
		"tls",
	}
	
	for _, retryable := range retryableErrors {
		if strings.Contains(errStr, retryable) {
			return true
		}
	}
	
	return false
}

func (prc *ParallelRepoCloner) updateWorkerStatus(workerID int, active bool, currentRepo string) {
	prc.progressMutex.Lock()
	defer prc.progressMutex.Unlock()
	
	if workerID >= 0 && workerID < len(prc.workerStatuses) {
		status := prc.workerStatuses[workerID]
		status.Active = active
		status.CurrentRepo = currentRepo
		
		if active {
			status.StartTime = time.Now()
			status.OperationCount++
		}
		
		// Update current operations list
		prc.progress.CurrentOperations = make([]string, 0, prc.maxWorkers)
		for _, s := range prc.workerStatuses {
			if s.Active && s.CurrentRepo != "" {
				prc.progress.CurrentOperations = append(prc.progress.CurrentOperations, s.CurrentRepo)
			}
		}
	}
}

// worker.run executes the worker loop
func (w *worker) run() {
	defer w.wg.Done()
	
	w.cloner.log("Worker %d started", w.id)
	defer w.cloner.log("Worker %d stopped", w.id)
	
	for {
		select {
		case request, ok := <-w.cloner.requestQueue:
			if !ok {
				// Channel closed, worker should exit
				w.cloner.updateWorkerStatus(w.id, false, "")
				return
			}
			
			w.cloner.updateWorkerStatus(w.id, true, request.RepoURL)
			
			// Process the clone request
			result := w.cloner.CloneSingleRepository(request)
			
			w.cloner.updateWorkerStatus(w.id, false, "")
			
			// Send result
			select {
			case w.cloner.resultQueue <- result:
				// Result sent successfully
			case <-w.ctx.Done():
				// Context cancelled
				return
			}
			
		case <-w.ctx.Done():
			// Context cancelled
			w.cloner.updateWorkerStatus(w.id, false, "")
			return
		}
	}
}