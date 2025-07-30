package testutils

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// MockRepoCacheManager provides a simple mock implementation for testing
type MockRepoCacheManager struct {
	cache map[string]string
}

func NewMockRepoCacheManager() *MockRepoCacheManager {
	return &MockRepoCacheManager{
		cache: make(map[string]string),
	}
}

func (m *MockRepoCacheManager) GetCachedRepo(repoURL, commitHash string) (string, bool, error) {
	key := fmt.Sprintf("%s#%s", repoURL, commitHash)
	path, exists := m.cache[key]
	return path, exists, nil
}

func (m *MockRepoCacheManager) CacheRepository(repoURL, commitHash, repoPath string) error {
	key := fmt.Sprintf("%s#%s", repoURL, commitHash)
	m.cache[key] = repoPath
	return nil
}

func (m *MockRepoCacheManager) UpdateCachedRepo(repoURL, commitHash, repoPath string) error {
	return m.CacheRepository(repoURL, commitHash, repoPath)
}

func (m *MockRepoCacheManager) IsCacheHealthy(repoURL, commitHash string) bool {
	_, exists, _ := m.GetCachedRepo(repoURL, commitHash)
	return exists
}

func TestParallelRepoCloner_Basic(t *testing.T) {
	// Create temporary directory for testing
	tempDir := filepath.Join("/tmp", fmt.Sprintf("parallel-cloner-test-%d", time.Now().UnixNano()))
	defer os.RemoveAll(tempDir)
	
	// Create mock cache manager
	cacheManager := NewMockRepoCacheManager()
	
	// Create parallel cloner with 2 workers
	cloner := NewParallelRepoCloner(2, cacheManager)
	defer cloner.CancelAll()
	
	// Test basic functionality
	if cloner.maxWorkers != 2 {
		t.Errorf("Expected 2 workers, got %d", cloner.maxWorkers)
	}
	
	if cloner.cacheManager != cacheManager {
		t.Error("Cache manager not set correctly")
	}
	
	// Test progress tracking
	progress := cloner.GetCloneProgress()
	if progress.TotalRequests != 0 {
		t.Errorf("Expected 0 total requests, got %d", progress.TotalRequests)
	}
	
	if progress.ActiveWorkers != 0 {
		t.Errorf("Expected 0 active workers, got %d", progress.ActiveWorkers)
	}
}

func TestParallelRepoCloner_CloneRequests(t *testing.T) {
	// Create temporary directory for testing
	tempDir := filepath.Join("/tmp", fmt.Sprintf("parallel-cloner-test-%d", time.Now().UnixNano()))
	defer os.RemoveAll(tempDir)
	
	// Create mock cache manager
	cacheManager := NewMockRepoCacheManager()
	
	// Create parallel cloner with 2 workers
	cloner := NewParallelRepoCloner(2, cacheManager)
	defer cloner.CancelAll()
	
	// Create test requests (using small, fast-to-clone repositories)
	requests := []CloneRequest{
		{
			RepoURL:   "https://github.com/octocat/Hello-World.git",
			RequestID: "test-1",
			TargetDir: filepath.Join(tempDir, "repo1"),
			Config: GenericRepoConfig{
				CloneTimeout:   30 * time.Second,
				EnableLogging:  false,
				PreserveGitDir: false,
			},
			Priority: 1,
		},
		{
			RepoURL:   "https://github.com/octocat/Spoon-Knife.git",
			RequestID: "test-2",
			TargetDir: filepath.Join(tempDir, "repo2"),
			Config: GenericRepoConfig{
				CloneTimeout:   30 * time.Second,
				EnableLogging:  false,
				PreserveGitDir: false,
			},
			Priority: 1,
		},
	}
	
	// Skip this test if running in CI or without network access
	if os.Getenv("CI") == "true" || os.Getenv("SKIP_NETWORK_TESTS") == "true" {
		t.Skip("Skipping network-dependent test")
	}
	
	// Execute parallel clone
	results, err := cloner.CloneRepositoriesParallel(requests)
	if err != nil {
		t.Fatalf("Parallel clone failed: %v", err)
	}
	
	// Validate results
	if len(results) != len(requests) {
		t.Errorf("Expected %d results, got %d", len(requests), len(results))
	}
	
	for i, result := range results {
		if result.RequestID != requests[i].RequestID {
			t.Errorf("Result %d: expected RequestID %s, got %s", i, requests[i].RequestID, result.RequestID)
		}
		
		if result.RepoURL != requests[i].RepoURL {
			t.Errorf("Result %d: expected RepoURL %s, got %s", i, requests[i].RepoURL, result.RepoURL)
		}
		
		// Note: We can't guarantee success in all environments, so we just check structure
		if result.Duration == 0 {
			t.Errorf("Result %d: duration should be > 0", i)
		}
		
		if result.StartTime.IsZero() {
			t.Errorf("Result %d: start time should be set", i)
		}
		
		if result.EndTime.IsZero() {
			t.Errorf("Result %d: end time should be set", i)
		}
		
		// If successful, check that directory exists
		if result.Success {
			if _, err := os.Stat(result.RepoPath); os.IsNotExist(err) {
				t.Errorf("Result %d: claimed success but directory doesn't exist: %s", i, result.RepoPath)
			}
		}
	}
	
	// Check final progress
	progress := cloner.GetCloneProgress()
	if progress.TotalRequests != len(requests) {
		t.Errorf("Expected %d total requests, got %d", len(requests), progress.TotalRequests)
	}
	
	if progress.CompletedCount != len(requests) {
		t.Errorf("Expected %d completed requests, got %d", len(requests), progress.CompletedCount)
	}
}

func TestParallelRepoCloner_EmptyRequests(t *testing.T) {
	// Create mock cache manager
	cacheManager := NewMockRepoCacheManager()
	
	// Create parallel cloner
	cloner := NewParallelRepoCloner(2, cacheManager)
	defer cloner.CancelAll()
	
	// Test with empty requests
	results, err := cloner.CloneRepositoriesParallel([]CloneRequest{})
	if err != nil {
		t.Errorf("Expected no error for empty requests, got: %v", err)
	}
	
	if len(results) != 0 {
		t.Errorf("Expected 0 results for empty requests, got %d", len(results))
	}
}

func TestParallelRepoCloner_DefaultWorkerCount(t *testing.T) {
	// Create mock cache manager
	cacheManager := NewMockRepoCacheManager()
	
	// Test with invalid worker count (should default to 3)
	cloner := NewParallelRepoCloner(0, cacheManager)
	defer cloner.CancelAll()
	
	if cloner.maxWorkers != 3 {
		t.Errorf("Expected default 3 workers for invalid input, got %d", cloner.maxWorkers)
	}
	
	// Test with negative worker count (should default to 3)
	cloner2 := NewParallelRepoCloner(-1, cacheManager)
	defer cloner2.CancelAll()
	
	if cloner2.maxWorkers != 3 {
		t.Errorf("Expected default 3 workers for negative input, got %d", cloner2.maxWorkers)
	}
}

func BenchmarkParallelRepoCloner_Progress(b *testing.B) {
	// Create mock cache manager
	cacheManager := NewMockRepoCacheManager()
	
	// Create parallel cloner
	cloner := NewParallelRepoCloner(4, cacheManager)
	defer cloner.CancelAll()
	
	// Benchmark progress tracking
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cloner.GetCloneProgress()
	}
}