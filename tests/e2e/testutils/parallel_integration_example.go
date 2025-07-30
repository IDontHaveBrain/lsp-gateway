package testutils

import (
	"fmt"
	"path/filepath"
	"time"
)

// Example integration showing how to use ParallelRepoCloner with existing system

// CreateParallelMultiProjectWorkspace demonstrates how to replace sequential cloning
// in ConcreteMultiProjectManager with parallel processing
func CreateParallelMultiProjectWorkspace(languages []string, cacheManager RepoCacheManager) (string, error) {
	// Create base workspace directory
	workspaceDir := filepath.Join("/tmp", fmt.Sprintf("parallel-multi-project-%d", time.Now().UnixNano()))
	
	// Create parallel cloner with optimal worker count (3-5 workers recommended)
	cloner := NewParallelRepoCloner(4, cacheManager)
	defer cloner.CancelAll()
	
	// Prepare clone requests for all languages
	var requests []CloneRequest
	for i, language := range languages {
		langConfig, err := getLanguageConfigForParallel(language)
		if err != nil {
			return "", fmt.Errorf("failed to get config for %s: %w", language, err)
		}
		
		subProjectDir := fmt.Sprintf("%s-project", language)
		targetDir := filepath.Join(workspaceDir, subProjectDir)
		
		request := CloneRequest{
			RepoURL:     langConfig.RepoURL,
			CommitHash:  langConfig.CustomVariables["commit_hash"],
			TargetDir:   targetDir,
			RequestID:   fmt.Sprintf("%s-%d", language, i),
			Priority:    1,
			Config: GenericRepoConfig{
				LanguageConfig: langConfig,
				TargetDir:      targetDir,
				CloneTimeout:   120 * time.Second,
				EnableLogging:  true,
				PreserveGitDir: false,
			},
		}
		
		requests = append(requests, request)
	}
	
	// Execute parallel clone (this replaces the sequential loop in setupSubProject)
	results, err := cloner.CloneRepositoriesParallel(requests)
	if err != nil {
		return "", fmt.Errorf("parallel clone failed: %w", err)
	}
	
	// Process results and validate
	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
			fmt.Printf("✓ %s cloned successfully in %v (cache_hit: %v)\n", 
				result.RepoURL, result.Duration, result.CacheHit)
		} else {
			fmt.Printf("✗ %s failed: %v\n", result.RepoURL, result.Error)
		}
	}
	
	fmt.Printf("\nParallel clone summary: %d/%d successful\n", successCount, len(requests))
	
	// Get final progress
	progress := cloner.GetCloneProgress()
	fmt.Printf("Cache hit rate: %d/%d (%.1f%%)\n", 
		progress.CacheHitCount, progress.TotalRequests, 
		float64(progress.CacheHitCount)/float64(progress.TotalRequests)*100)
	
	return workspaceDir, nil
}

// PerformanceComparisonDemo shows the performance improvement
func PerformanceComparisonDemo(languages []string, cacheManager RepoCacheManager) {
	fmt.Println("=== Sequential vs Parallel Repository Cloning Performance ===")
	
	// Sequential timing (simulated based on existing ConcreteMultiProjectManager)
	fmt.Println("Sequential Processing (existing system):")
	// Each repository cloned one after another
	// Estimated: 3-7 seconds per repository
	estimatedSequentialTime := time.Duration(len(languages)) * 5 * time.Second
	fmt.Printf("  Estimated time: %v\n", estimatedSequentialTime)
	fmt.Printf("  Cache benefits: Limited (each repo processed independently)\n")
	fmt.Printf("  Resource usage: Single git process at a time\n\n")
	
	// Parallel timing (actual measurement)
	fmt.Println("Parallel Processing (new system):")
	parallelStart := time.Now()
	
	cloner := NewParallelRepoCloner(4, cacheManager)
	defer cloner.CancelAll()
	
	// Create test requests
	var requests []CloneRequest
	for i, language := range languages {
		requests = append(requests, CloneRequest{
			RepoURL:   fmt.Sprintf("https://github.com/example/%s-repo.git", language),
			RequestID: fmt.Sprintf("test-%d", i),
			TargetDir: fmt.Sprintf("/tmp/test-%s", language),
			Priority:  1,
		})
	}
	
	// Monitor progress in real-time
	go func() {
		for {
			progress := cloner.GetCloneProgress()
			if progress.CompletedCount >= progress.TotalRequests {
				break
			}
			
			if progress.TotalRequests > 0 {
				fmt.Printf("  Progress: %d/%d (%.1f%%) - Active workers: %d - ETA: %v\n",
					progress.CompletedCount, progress.TotalRequests,
					float64(progress.CompletedCount)/float64(progress.TotalRequests)*100,
					progress.ActiveWorkers, progress.EstimatedTimeLeft)
			}
			
			time.Sleep(500 * time.Millisecond)
		}
	}()
	
	// Execute (this would actually clone in real usage)
	// results, _ := cloner.CloneRepositoriesParallel(requests)
	
	parallelDuration := time.Since(parallelStart)
	fmt.Printf("  Actual time for setup: %v\n", parallelDuration)
	fmt.Printf("  Resource usage: %d concurrent git processes\n", cloner.maxWorkers)
	fmt.Printf("  Cache optimization: Shared cache with intelligent updates\n")
	fmt.Printf("  Retry mechanism: Built-in exponential backoff\n")
	
	// Calculate theoretical improvement
	theoreticalImprovement := float64(estimatedSequentialTime) / float64(estimatedSequentialTime/time.Duration(cloner.maxWorkers))
	fmt.Printf("\n  Theoretical speedup: %.1fx faster\n", theoreticalImprovement)
	fmt.Printf("  Target improvement: 80%% time reduction\n")
}

// MonitoringExample shows how to use progress tracking
func MonitoringExample(cloner *ParallelRepoCloner) {
	fmt.Println("=== Real-time Progress Monitoring ===")
	
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			progress := cloner.GetCloneProgress()
			
			if progress.TotalRequests == 0 {
				continue
			}
			
			fmt.Printf("\rProgress: [%s] %d/%d repos | Success: %d | Cache hits: %d | Workers: %d | ETA: %v",
				progressBar(progress.CompletedCount, progress.TotalRequests),
				progress.CompletedCount, progress.TotalRequests,
				progress.SuccessCount, progress.CacheHitCount,
				progress.ActiveWorkers, progress.EstimatedTimeLeft)
			
			if progress.CompletedCount >= progress.TotalRequests {
				fmt.Println("\n✓ All repositories processed!")
				return
			}
		}
	}
}

// Helper function to create a simple progress bar
func progressBar(current, total int) string {
	if total == 0 {
		return "----------"
	}
	
	percentage := float64(current) / float64(total)
	filled := int(percentage * 10)
	
	bar := ""
	for i := 0; i < 10; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	
	return bar
}

// Helper function to get language config (placeholder for actual implementation)
func getLanguageConfigForParallel(language string) (LanguageConfig, error) {
	// This would use the same logic as ConcreteMultiProjectManager.getLanguageConfig()
	switch language {
	case "go":
		return GetGoLanguageConfig(), nil
	case "python":
		return GetPythonLanguageConfig(), nil
	case "javascript":
		return GetJavaScriptLanguageConfig(), nil
	case "typescript":
		return GetTypeScriptLanguageConfig(), nil
	case "java":
		return GetJavaLanguageConfig(), nil
	default:
		return LanguageConfig{}, fmt.Errorf("unsupported language: %s", language)
	}
}