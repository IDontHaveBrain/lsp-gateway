package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-gateway/internal/storage"
)

func TestWorkspaceSCIPCache_Initialize(t *testing.T) {
	t.Parallel()
	// Create temporary workspace directory
	tempDir, err := os.MkdirTemp("", "workspace-scip-cache-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create config manager
	configManager := NewWorkspaceConfigManager()
	
	// Create workspace SCIP cache
	cache := NewWorkspaceSCIPCache(configManager)
	
	// Test initialization
	config := &CacheConfig{
		MaxMemorySize:        100 * 1024 * 1024, // 100MB
		MaxMemoryEntries:     1000,
		MemoryTTL:            10 * time.Minute,
		MaxDiskSize:          500 * 1024 * 1024, // 500MB
		MaxDiskEntries:       5000,
		DiskTTL:              1 * time.Hour,
		CompressionEnabled:   true,
		CompressionThreshold: 1024,
		CompressionType:      storage.CompressionLZ4,
		CleanupInterval:      1 * time.Minute,
		MetricsInterval:      10 * time.Second,
		SyncInterval:        30 * time.Second,
		IsolationLevel:      IsolationStrong,
		CrossWorkspaceAccess: false,
	}
	
	err = cache.Initialize(tempDir, config)
	if err != nil {
		t.Fatalf("Failed to initialize workspace SCIP cache: %v", err)
	}
	defer cache.Close()
	
	// Verify initialization
	stats := cache.GetStats()
	if stats.WorkspaceRoot != tempDir {
		t.Errorf("Expected workspace root %s, got %s", tempDir, stats.WorkspaceRoot)
	}
	
	if stats.WorkspaceHash == "" {
		t.Error("Expected non-empty workspace hash")
	}
}

func TestWorkspaceSCIPCache_SetAndGet(t *testing.T) {
	t.Parallel()
	// Setup cache
	tempDir, cache := setupTestCache(t)
	defer os.RemoveAll(tempDir)
	defer cache.Close()
	
	// Create test entry
	testKey := "test_method:params"
	testResponse := json.RawMessage(`{"result": "test_data"}`)
	
	entry := &storage.CacheEntry{
		Method:   "textDocument/definition",
		Params:   `{"textDocument": {"uri": "file://test.go"}}`,
		Response: testResponse,
		FilePaths: []string{"test.go"},
	}
	
	// Test Set
	err := cache.Set(testKey, entry, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to set cache entry: %v", err)
	}
	
	// Test Get
	retrievedEntry, found := cache.Get(testKey)
	if !found {
		t.Fatal("Cache entry not found")
	}
	
	if retrievedEntry.Method != entry.Method {
		t.Errorf("Expected method %s, got %s", entry.Method, retrievedEntry.Method)
	}
	
	if string(retrievedEntry.Response) != string(testResponse) {
		t.Errorf("Expected response %s, got %s", testResponse, retrievedEntry.Response)
	}
}

func TestWorkspaceSCIPCache_InvalidateFile(t *testing.T) {
	t.Parallel()
	// Setup cache
	tempDir, cache := setupTestCache(t)
	defer os.RemoveAll(tempDir)
	defer cache.Close()
	
	// Create test entries
	entry1 := &storage.CacheEntry{
		Method:    "textDocument/definition",
		Params:    `{"textDocument": {"uri": "file://test1.go"}}`,
		Response:  json.RawMessage(`{"result": "data1"}`),
		FilePaths: []string{"test1.go"},
	}
	
	entry2 := &storage.CacheEntry{
		Method:    "textDocument/definition",
		Params:    `{"textDocument": {"uri": "file://test2.go"}}`,
		Response:  json.RawMessage(`{"result": "data2"}`),
		FilePaths: []string{"test2.go"},
	}
	
	// Set entries
	cache.Set("key1", entry1, 5*time.Minute)
	cache.Set("key2", entry2, 5*time.Minute)
	
	// Verify both entries exist
	_, found1 := cache.Get("key1")
	_, found2 := cache.Get("key2")
	if !found1 || !found2 {
		t.Fatal("Cache entries should exist before invalidation")
	}
	
	// Invalidate file
	count, err := cache.InvalidateFile("test1.go")
	if err != nil {
		t.Fatalf("Failed to invalidate file: %v", err)
	}
	
	if count == 0 {
		t.Error("Expected at least one entry to be invalidated")
	}
	
	// Verify invalidation
	_, found1After := cache.Get("key1")
	_, found2After := cache.Get("key2")
	
	if found1After {
		t.Error("Entry for test1.go should be invalidated")
	}
	if !found2After {
		t.Error("Entry for test2.go should still exist")
	}
}

func TestWorkspaceSCIPCache_WorkspaceIsolation(t *testing.T) {
	t.Parallel()
	// Create two separate workspace directories
	tempDir1, err := os.MkdirTemp("", "workspace1")
	if err != nil {
		t.Fatalf("Failed to create temp directory 1: %v", err)
	}
	defer os.RemoveAll(tempDir1)
	
	tempDir2, err := os.MkdirTemp("", "workspace2")
	if err != nil {
		t.Fatalf("Failed to create temp directory 2: %v", err)
	}
	defer os.RemoveAll(tempDir2)
	
	// Create two separate caches
	configManager := NewWorkspaceConfigManager()
	cache1 := NewWorkspaceSCIPCache(configManager)
	cache2 := NewWorkspaceSCIPCache(configManager)
	
	config := getTestConfig()
	
	// Initialize both caches
	err = cache1.Initialize(tempDir1, config)
	if err != nil {
		t.Fatalf("Failed to initialize cache1: %v", err)
	}
	defer cache1.Close()
	
	err = cache2.Initialize(tempDir2, config)
	if err != nil {
		t.Fatalf("Failed to initialize cache2: %v", err)
	}
	defer cache2.Close()
	
	// Verify different workspace hashes
	stats1 := cache1.GetStats()
	stats2 := cache2.GetStats()
	
	if stats1.WorkspaceHash == stats2.WorkspaceHash {
		t.Error("Different workspaces should have different hashes")
	}
	
	// Test data isolation
	testKey := "test_key"
	entry1 := &storage.CacheEntry{
		Method:   "textDocument/definition",
		Response: json.RawMessage(`{"result": "workspace1_data"}`),
	}
	entry2 := &storage.CacheEntry{
		Method:   "textDocument/definition",
		Response: json.RawMessage(`{"result": "workspace2_data"}`),
	}
	
	// Set different data in each cache
	cache1.Set(testKey, entry1, 5*time.Minute)
	cache2.Set(testKey, entry2, 5*time.Minute)
	
	// Verify isolation
	retrieved1, found1 := cache1.Get(testKey)
	retrieved2, found2 := cache2.Get(testKey)
	
	if !found1 || !found2 {
		t.Fatal("Both cache entries should exist")
	}
	
	if string(retrieved1.Response) == string(retrieved2.Response) {
		t.Error("Cache entries should be isolated between workspaces")
	}
}

func TestWorkspaceSCIPCache_Performance(t *testing.T) {
	t.Parallel()
	// Setup cache
	tempDir, cache := setupTestCache(t)
	defer os.RemoveAll(tempDir)
	defer cache.Close()
	
	// Test L1 memory cache performance (<10ms target)
	entry := &storage.CacheEntry{
		Method:   "textDocument/definition",
		Response: json.RawMessage(`{"result": "performance_test"}`),
	}
	
	// Warm up cache
	cache.Set("perf_test", entry, 5*time.Minute)
	
	// Measure get performance
	start := time.Now()
	for i := 0; i < 1000; i++ {
		cache.Get("perf_test")
	}
	elapsed := time.Since(start)
	
	avgLatency := elapsed / 1000
	if avgLatency > 10*time.Millisecond {
		t.Errorf("Average cache latency %v exceeds 10ms target", avgLatency)
	}
	
	// Test cache hit rate
	stats := cache.GetStats()
	if stats.HitRate < 0.9 { // Expect >90% hit rate for repeated access
		t.Errorf("Cache hit rate %f is below 90%%", stats.HitRate)
	}
}

func TestWorkspaceSCIPCache_MemoryManagement(t *testing.T) {
	t.Parallel()
	// Setup cache with small memory limit
	tempDir, err := os.MkdirTemp("", "workspace-memory-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	configManager := NewWorkspaceConfigManager()
	cache := NewWorkspaceSCIPCache(configManager)
	
	config := &CacheConfig{
		MaxMemorySize:        1024 * 1024, // 1MB - small for testing eviction
		MaxMemoryEntries:     10,           // Small entry limit
		MemoryTTL:            10 * time.Minute,
		MaxDiskSize:          10 * 1024 * 1024, // 10MB
		MaxDiskEntries:       100,
		DiskTTL:              1 * time.Hour,
		CompressionEnabled:   true,
		CompressionThreshold: 512,
		CompressionType:      storage.CompressionLZ4,
		CleanupInterval:      10 * time.Second,
		MetricsInterval:      5 * time.Second,
		SyncInterval:        15 * time.Second,
		IsolationLevel:      IsolationStrong,
	}
	
	err = cache.Initialize(tempDir, config)
	if err != nil {
		t.Fatalf("Failed to initialize cache: %v", err)
	}
	defer cache.Close()
	
	// Add entries beyond memory limit
	largeData := make([]byte, 64*1024) // 64KB per entry
	for i := 0; i < 20; i++ {
		entry := &storage.CacheEntry{
			Method:   "textDocument/definition",
			Response: json.RawMessage(largeData),
		}
		cache.Set(fmt.Sprintf("key_%d", i), entry, 5*time.Minute)
	}
	
	// Verify that eviction occurred
	stats := cache.GetStats()
	if stats.L1Stats.EntryCount > 10 {
		t.Errorf("L1 cache should not exceed 10 entries, got %d", stats.L1Stats.EntryCount)
	}
	
	// Verify data was promoted to L2
	if stats.L2Stats.EntryCount == 0 {
		t.Error("Some entries should have been moved to L2 cache")
	}
}

func TestWorkspaceSCIPCache_Optimization(t *testing.T) {
	t.Parallel()
	// Setup cache
	tempDir, cache := setupTestCache(t)
	defer os.RemoveAll(tempDir)
	defer cache.Close()
	
	// Add some test data
	for i := 0; i < 10; i++ {
		entry := &storage.CacheEntry{
			Method:   "textDocument/definition",
			Response: json.RawMessage(`{"result": "optimization_test"}`),
		}
		cache.Set(fmt.Sprintf("opt_key_%d", i), entry, 5*time.Minute)
	}
	
	// Run optimization
	ctx := context.Background()
	result, err := cache.Optimize(ctx)
	if err != nil {
		t.Fatalf("Cache optimization failed: %v", err)
	}
	
	if result.Duration <= 0 {
		t.Error("Optimization should have taken some time")
	}
	
	if result.StartTime.IsZero() {
		t.Error("Optimization result should have start time")
	}
}

func TestWorkspaceSCIPCache_DirectoryStructure(t *testing.T) {
	t.Parallel()
	// Setup cache
	tempDir, cache := setupTestCache(t)
	defer os.RemoveAll(tempDir)
	defer cache.Close()
	
	// Verify workspace directory structure was created
	configManager := NewWorkspaceConfigManager()
	workspaceDir := configManager.GetWorkspaceDirectory(tempDir)
	
	expectedDirs := []string{
		filepath.Join(workspaceDir, "cache", "scip"),
		filepath.Join(workspaceDir, "cache", "scip", "memory"),
		filepath.Join(workspaceDir, "cache", "scip", "disk"),
		filepath.Join(workspaceDir, "cache", "scip", "index"),
		filepath.Join(workspaceDir, "cache", "scip", "responses"),
	}
	
	for _, dir := range expectedDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Expected directory %s was not created", dir)
		}
	}
	
	// Verify metadata file was created
	metadataPath := filepath.Join(workspaceDir, "cache", "scip", "metadata.json")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Error("Metadata file was not created")
	}
}

// Helper functions

func setupTestCache(t *testing.T) (string, WorkspaceSCIPCache) {
	tempDir, err := os.MkdirTemp("", "workspace-scip-cache-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	
	configManager := NewWorkspaceConfigManager()
	cache := NewWorkspaceSCIPCache(configManager)
	
	config := getTestConfig()
	
	err = cache.Initialize(tempDir, config)
	if err != nil {
		t.Fatalf("Failed to initialize workspace SCIP cache: %v", err)
	}
	
	return tempDir, cache
}

func getTestConfig() *CacheConfig {
	return &CacheConfig{
		MaxMemorySize:        50 * 1024 * 1024, // 50MB
		MaxMemoryEntries:     1000,
		MemoryTTL:            10 * time.Minute,
		MaxDiskSize:          200 * 1024 * 1024, // 200MB
		MaxDiskEntries:       5000,
		DiskTTL:              1 * time.Hour,
		CompressionEnabled:   true,
		CompressionThreshold: 1024,
		CompressionType:      storage.CompressionLZ4,
		CleanupInterval:      30 * time.Second,
		MetricsInterval:      10 * time.Second,
		SyncInterval:        30 * time.Second,
		IsolationLevel:      IsolationStrong,
		CrossWorkspaceAccess: false,
	}
}