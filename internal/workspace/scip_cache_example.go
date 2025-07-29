package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"lsp-gateway/internal/indexing"
	"lsp-gateway/internal/storage"
)

// ExampleWorkspaceSCIPCacheUsage demonstrates comprehensive usage of the workspace SCIP cache system
func ExampleWorkspaceSCIPCacheUsage() {
	// Step 1: Initialize workspace configuration manager
	configManager := NewWorkspaceConfigManager()
	
	// Step 2: Create workspace SCIP cache
	workspaceRoot := "/path/to/my/project"
	cache := NewWorkspaceSCIPCache(configManager)
	
	// Step 3: Configure cache with workspace-specific settings
	config := &CacheConfig{
		// Memory tier (L1) - Fast access for frequently used data
		MaxMemorySize:    2 * 1024 * 1024 * 1024, // 2GB
		MaxMemoryEntries: 100000,                  // 100k entries
		MemoryTTL:        30 * time.Minute,        // 30 minute TTL
		
		// Disk tier (L2) - Larger capacity for workspace persistence
		MaxDiskSize:   10 * 1024 * 1024 * 1024, // 10GB
		MaxDiskEntries: 1000000,                 // 1M entries
		DiskTTL:       24 * time.Hour,            // 24 hour TTL
		
		// Performance optimization
		CompressionEnabled:   true,
		CompressionThreshold: 1024,                     // Compress entries > 1KB
		CompressionType:      storage.CompressionLZ4,   // Fast compression
		
		// Maintenance intervals
		CleanupInterval: 5 * time.Minute,   // Cleanup expired entries
		MetricsInterval: 30 * time.Second,  // Update performance metrics
		SyncInterval:    60 * time.Second,  // Sync to disk
		
		// Workspace isolation
		IsolationLevel:       IsolationStrong, // Strong isolation between workspaces
		CrossWorkspaceAccess: false,           // No cross-workspace access
	}
	
	// Step 4: Initialize the cache
	if err := cache.Initialize(workspaceRoot, config); err != nil {
		log.Fatalf("Failed to initialize workspace SCIP cache: %v", err)
	}
	defer cache.Close()
	
	// Step 5: Demonstrate cache operations
	demonstrateCacheOperations(cache)
	
	// Step 6: Show performance monitoring
	demonstratePerformanceMonitoring(cache)
	
	// Step 7: Demonstrate workspace isolation
	demonstrateWorkspaceIsolation(configManager)
	
	// Step 8: Show integration with existing SCIP system
	demonstrateSCIPIntegration(workspaceRoot, configManager)
}

func demonstrateCacheOperations(cache WorkspaceSCIPCache) {
	fmt.Println("=== Cache Operations Demo ===")
	
	// Create a sample cache entry
	entry := &storage.CacheEntry{
		Method:   "textDocument/definition",
		Params:   `{"textDocument": {"uri": "file:///project/main.go"}, "position": {"line": 10, "character": 5}}`,
		Response: json.RawMessage(`{"uri": "file:///project/main.go", "range": {"start": {"line": 5, "character": 4}, "end": {"line": 5, "character": 12}}}`),
		FilePaths: []string{"/project/main.go"},
	}
	
	// Set cache entry
	cacheKey := "definition:main.go:10:5"
	if err := cache.Set(cacheKey, entry, 15*time.Minute); err != nil {
		log.Printf("Failed to set cache entry: %v", err)
		return
	}
	fmt.Printf("✓ Cached entry for key: %s\n", cacheKey)
	
	// Get cache entry
	if retrievedEntry, found := cache.Get(cacheKey); found {
		fmt.Printf("✓ Retrieved entry: method=%s, response_size=%d bytes\n", 
			retrievedEntry.Method, len(retrievedEntry.Response))
	} else {
		fmt.Println("✗ Cache entry not found")
	}
	
	// Invalidate by file
	if count, err := cache.InvalidateFile("/project/main.go"); err != nil {
		log.Printf("Failed to invalidate file: %v", err)
	} else {
		fmt.Printf("✓ Invalidated %d entries for file /project/main.go\n", count)
	}
}

func demonstratePerformanceMonitoring(cache WorkspaceSCIPCache) {
	fmt.Println("\n=== Performance Monitoring Demo ===")
	
	// Get comprehensive statistics
	stats := cache.GetStats()
	
	fmt.Printf("Workspace: %s (hash: %s)\n", stats.WorkspaceRoot, stats.WorkspaceHash)
	fmt.Printf("Total Entries: %d, Total Size: %d bytes\n", stats.TotalEntries, stats.TotalSize)
	fmt.Printf("Cache Hit Rate: %.2f%%, Average Latency: %v\n", stats.HitRate*100, stats.AverageLatency)
	
	// L1 Memory Cache Stats
	fmt.Printf("\nL1 Memory Cache:\n")
	fmt.Printf("  Entries: %d, Size: %d bytes, Utilization: %.1f%%\n", 
		stats.L1Stats.EntryCount, stats.L1Stats.SizeBytes, stats.L1Stats.Utilization*100)
	fmt.Printf("  Hit Rate: %.2f%%, Avg Latency: %v\n", 
		stats.L1Stats.HitRate*100, stats.L1Stats.AvgLatency)
	
	// L2 Disk Cache Stats
	fmt.Printf("\nL2 Disk Cache:\n")
	fmt.Printf("  Entries: %d, Size: %d bytes, Utilization: %.1f%%\n", 
		stats.L2Stats.EntryCount, stats.L2Stats.SizeBytes, stats.L2Stats.Utilization*100)
	fmt.Printf("  Hit Rate: %.2f%%, Avg Latency: %v\n", 
		stats.L2Stats.HitRate*100, stats.L2Stats.AvgLatency)
	
	// Health monitoring
	fmt.Printf("\nHealth Status:\n")
	fmt.Printf("  Health Score: %.2f, Last Check: %v\n", stats.HealthScore, stats.LastHealthCheck)
	if len(stats.Issues) > 0 {
		fmt.Printf("  Issues: %d\n", len(stats.Issues))
		for _, issue := range stats.Issues {
			fmt.Printf("    - %s: %s\n", issue.Type, issue.Message)
		}
	} else {
		fmt.Println("  ✓ No health issues detected")
	}
}

func demonstrateWorkspaceIsolation(configManager WorkspaceConfigManager) {
	fmt.Println("\n=== Workspace Isolation Demo ===")
	
	// Create two separate workspace caches
	workspace1 := "/path/to/project1"
	workspace2 := "/path/to/project2"
	
	cache1 := NewWorkspaceSCIPCache(configManager)
	cache2 := NewWorkspaceSCIPCache(configManager)
	
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
		CleanupInterval:      30 * time.Second,
		MetricsInterval:      10 * time.Second,
		SyncInterval:         30 * time.Second,
		IsolationLevel:       IsolationStrong,
		CrossWorkspaceAccess: false,
	}
	
	// Initialize both caches
	if err := cache1.Initialize(workspace1, config); err != nil {
		log.Printf("Failed to initialize cache1: %v", err)
		return
	}
	defer cache1.Close()
	
	if err := cache2.Initialize(workspace2, config); err != nil {
		log.Printf("Failed to initialize cache2: %v", err)
		return
	}
	defer cache2.Close()
	
	// Demonstrate isolation
	testKey := "isolation_test"
	entry1 := &storage.CacheEntry{
		Method:   "textDocument/definition",
		Response: json.RawMessage(`{"workspace": "project1"}`),
	}
	entry2 := &storage.CacheEntry{
		Method:   "textDocument/definition", 
		Response: json.RawMessage(`{"workspace": "project2"}`),
	}
	
	// Set same key in both caches
	cache1.Set(testKey, entry1, 5*time.Minute)
	cache2.Set(testKey, entry2, 5*time.Minute)
	
	// Verify isolation
	if retrieved1, found := cache1.Get(testKey); found {
		if retrieved2, found := cache2.Get(testKey); found {
			fmt.Printf("✓ Workspace isolation verified:\n")
			fmt.Printf("  Cache1 response: %s\n", string(retrieved1.Response))
			fmt.Printf("  Cache2 response: %s\n", string(retrieved2.Response))
		}
	}
	
	// Show different workspace hashes
	stats1 := cache1.GetStats()
	stats2 := cache2.GetStats()
	fmt.Printf("✓ Different workspace hashes: %s vs %s\n", stats1.WorkspaceHash, stats2.WorkspaceHash)
}

func demonstrateSCIPIntegration(workspaceRoot string, configManager WorkspaceConfigManager) {
	fmt.Println("\n=== SCIP Integration Demo ===")
	
	// Create workspace SCIP store (implements indexing.SCIPStore interface)
	scipStore := NewWorkspaceSCIPStore(workspaceRoot, configManager)
	
	// Initialize with SCIP configuration
	scipConfig := &indexing.SCIPConfig{
		CacheConfig: indexing.CacheConfig{
			Enabled: true,
			MaxSize: 10000,
			TTL:     30 * time.Minute,
		},
		Logging: indexing.LoggingConfig{
			LogQueries:         true,
			LogCacheOperations: true,
			LogIndexOperations: true,
		},
		Performance: indexing.PerformanceConfig{
			QueryTimeout:         10 * time.Second,
			MaxConcurrentQueries: 50,
			IndexLoadTimeout:     5 * time.Minute,
		},
	}
	
	if err := scipStore.Initialize(scipConfig); err != nil {
		log.Printf("Failed to initialize SCIP store: %v", err)
		return
	}
	defer scipStore.Close()
	
	// Demonstrate SCIP interface usage
	fmt.Println("✓ Workspace SCIP store initialized")
	
	// Load index (workspace-isolated)
	if err := scipStore.LoadIndex(workspaceRoot); err != nil {
		log.Printf("Failed to load index: %v", err)
	} else {
		fmt.Printf("✓ Loaded workspace index for %s\n", workspaceRoot)
	}
	
	// Perform SCIP query
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///project/main.go",
		},
		"position": map[string]interface{}{
			"line":      10,
			"character": 5,
		},
	}
	
	result := scipStore.Query("textDocument/definition", params)
	fmt.Printf("✓ SCIP query result: found=%t, cache_hit=%t, query_time=%v\n", 
		result.Found, result.CacheHit, result.QueryTime)
	
	// Cache a response
	response := json.RawMessage(`{"uri": "file:///project/types.go", "range": {"start": {"line": 15, "character": 0}}}`)
	if err := scipStore.CacheResponse("textDocument/definition", params, response); err != nil {
		log.Printf("Failed to cache response: %v", err)
	} else {
		fmt.Println("✓ Cached SCIP response")
	}
	
	// Get SCIP statistics
	scipStats := scipStore.GetStats()
	fmt.Printf("✓ SCIP Stats: queries=%d, hit_rate=%.2f%%, cache_size=%d\n", 
		scipStats.TotalQueries, scipStats.CacheHitRate*100, scipStats.CacheSize)
}

// ExampleCacheOptimization demonstrates cache optimization features
func ExampleCacheOptimization() {
	fmt.Println("\n=== Cache Optimization Demo ===")
	
	configManager := NewWorkspaceConfigManager()
	cache := NewWorkspaceSCIPCache(configManager)
	
	config := &CacheConfig{
		MaxMemorySize:        50 * 1024 * 1024, // Smaller cache for optimization demo
		MaxMemoryEntries:     500,
		MemoryTTL:            5 * time.Minute,
		MaxDiskSize:          200 * 1024 * 1024,
		MaxDiskEntries:       2000,
		DiskTTL:              30 * time.Minute,
		CompressionEnabled:   true,
		CompressionThreshold: 512,
		CompressionType:      storage.CompressionLZ4,
		CleanupInterval:      30 * time.Second,
		MetricsInterval:      10 * time.Second,
		SyncInterval:         30 * time.Second,
		IsolationLevel:       IsolationStrong,
	}
	
	workspaceRoot := "/path/to/optimization/demo"
	if err := cache.Initialize(workspaceRoot, config); err != nil {
		log.Printf("Failed to initialize cache: %v", err)
		return
	}
	defer cache.Close()
	
	// Add some test data to fill cache
	for i := 0; i < 100; i++ {
		entry := &storage.CacheEntry{
			Method:   "textDocument/definition",
			Response: json.RawMessage(fmt.Sprintf(`{"test_data": "entry_%d"}`, i)),
		}
		cache.Set(fmt.Sprintf("test_key_%d", i), entry, 10*time.Minute)
	}
	
	// Show initial stats
	initialStats := cache.GetStats()
	fmt.Printf("Initial state: %d entries, %d bytes\n", 
		initialStats.TotalEntries, initialStats.TotalSize)
	
	// Perform optimization
	ctx := context.Background()
	result, err := cache.Optimize(ctx)
	if err != nil {
		log.Printf("Optimization failed: %v", err)
		return
	}
	
	// Show optimization results
	fmt.Printf("✓ Optimization completed in %v\n", result.Duration)
	fmt.Printf("  Entries processed: %d\n", result.EntriesProcessed)
	fmt.Printf("  Entries evicted: %d\n", result.EntriesEvicted)
	fmt.Printf("  Entries promoted: %d\n", result.EntriesPromoted)
	fmt.Printf("  Space reclaimed: %d bytes\n", result.SpaceReclaimed)
	fmt.Printf("  Performance gain: %.1f%%\n", result.PerformanceGain)
	
	// Show final stats
	finalStats := cache.GetStats()
	fmt.Printf("Final state: %d entries, %d bytes\n", 
		finalStats.TotalEntries, finalStats.TotalSize)
}

// ExampleAdvancedConfiguration shows advanced configuration options
func ExampleAdvancedConfiguration() {
	fmt.Println("\n=== Advanced Configuration Demo ===")
	
	// Production-ready configuration for large workspaces
	productionConfig := &CacheConfig{
		// Large memory tier for high-frequency access
		MaxMemorySize:    8 * 1024 * 1024 * 1024, // 8GB
		MaxMemoryEntries: 500000,                  // 500k entries
		MemoryTTL:        45 * time.Minute,        // Longer TTL for production
		
		// Large disk tier for workspace persistence
		MaxDiskSize:   50 * 1024 * 1024 * 1024, // 50GB
		MaxDiskEntries: 5000000,                 // 5M entries
		DiskTTL:       7 * 24 * time.Hour,       // 1 week TTL
		
		// Aggressive compression for large data
		CompressionEnabled:   true,
		CompressionThreshold: 512,                       // Compress smaller entries
		CompressionType:      storage.CompressionZstd,   // Better compression ratio
		
		// Frequent maintenance for optimal performance
		CleanupInterval: 2 * time.Minute,   // More frequent cleanup
		MetricsInterval: 15 * time.Second,  // More frequent metrics
		SyncInterval:    30 * time.Second,  // Frequent sync for reliability
		
		// Strong isolation for security
		IsolationLevel:       IsolationStrong,
		CrossWorkspaceAccess: false,
	}
	
	// Development configuration for fast iteration
	developmentConfig := &CacheConfig{
		// Smaller memory tier for development
		MaxMemorySize:    1 * 1024 * 1024 * 1024, // 1GB
		MaxMemoryEntries: 50000,                   // 50k entries
		MemoryTTL:        15 * time.Minute,        // Shorter TTL for faster refresh
		
		// Moderate disk tier
		MaxDiskSize:   5 * 1024 * 1024 * 1024, // 5GB
		MaxDiskEntries: 500000,                // 500k entries
		DiskTTL:       4 * time.Hour,           // Shorter TTL for development
		
		// Light compression for faster access
		CompressionEnabled:   true,
		CompressionThreshold: 2048,                     // Only compress larger entries
		CompressionType:      storage.CompressionLZ4,   // Fastest compression
		
		// Less frequent maintenance
		CleanupInterval: 10 * time.Minute,  // Less frequent cleanup
		MetricsInterval: 30 * time.Second,  // Standard metrics interval
		SyncInterval:    2 * time.Minute,   // Less frequent sync
		
		// Moderate isolation for development flexibility
		IsolationLevel:       IsolationWeak,
		CrossWorkspaceAccess: false,
	}
	
	fmt.Println("✓ Production config: 8GB memory, 50GB disk, aggressive compression")
	fmt.Println("✓ Development config: 1GB memory, 5GB disk, fast compression")
	
	// Example of runtime configuration updates
	fmt.Println("\n=== Runtime Configuration Updates ===")
	fmt.Println("Note: Cache configuration is set at initialization time.")
	fmt.Println("For runtime updates, close and reinitialize the cache with new config.")
	
	_ = productionConfig
	_ = developmentConfig
}