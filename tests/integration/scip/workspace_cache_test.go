package scip

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"lsp-gateway/internal/indexing"
	"lsp-gateway/internal/workspace"
)

// TestWorkspaceSCIPCacheIsolation tests workspace-isolated SCIP cache functionality
func TestWorkspaceSCIPCacheIsolation(t *testing.T) {
	config := DefaultSCIPTestConfig()
	config.EnableWorkspaceTests = true
	config.WorkspaceIsolationTests = true
	config.TestWorkspaces = []string{"workspace1", "workspace2", "workspace3"}

	suite, err := NewSCIPTestSuite(config)
	if err != nil {
		t.Fatalf("Failed to create SCIP test suite: %v", err)
	}
	defer suite.Cleanup()

	t.Run("WorkspaceCacheIsolation", func(t *testing.T) {
		testWorkspaceCacheIsolation(t, suite)
	})

	t.Run("WorkspaceQueryRouting", func(t *testing.T) {
		testWorkspaceQueryRouting(t, suite)
	})

	t.Run("WorkspaceCachePerformance", func(t *testing.T) {
		testWorkspaceCachePerformance(t, suite)
	})

	t.Run("WorkspaceMemoryManagement", func(t *testing.T) {
		testWorkspaceMemoryManagement(t, suite)
	})

	t.Run("WorkspaceIndexLoading", func(t *testing.T) {
		testWorkspaceIndexLoading(t, suite)
	})

	t.Run("CrossProjectSymbolResolution", func(t *testing.T) {
		testCrossProjectSymbolResolution(t, suite)
	})
}

// testWorkspaceCacheIsolation verifies that workspace caches are properly isolated
func testWorkspaceCacheIsolation(t *testing.T, suite *SCIPTestSuite) {
	workspaces := []string{
		filepath.Join(suite.tempDir, "workspace1"),
		filepath.Join(suite.tempDir, "workspace2"),
		filepath.Join(suite.tempDir, "workspace3"),
	}

	// Create workspace caches
	caches := make(map[string]workspace.WorkspaceSCIPCache)
	for _, workspaceRoot := range workspaces {
		configManager := &TestWorkspaceConfigManager{
			workspaceRoot: workspaceRoot,
		}
		
		cache := workspace.NewWorkspaceSCIPCache(configManager)
		
		cacheConfig := &workspace.CacheConfig{
			MaxMemorySize:        50 * 1024 * 1024, // 50MB per workspace
			MaxMemoryEntries:     10000,
			MemoryTTL:            15 * time.Minute,
			MaxDiskSize:          200 * 1024 * 1024, // 200MB
			MaxDiskEntries:       100000,
			DiskTTL:              4 * time.Hour,
			CompressionEnabled:   true,
			CompressionThreshold: 1024,
			CompressionType:      "lz4",
			CleanupInterval:      2 * time.Minute,
			MetricsInterval:      15 * time.Second,
			SyncInterval:         30 * time.Second,			
			IsolationLevel:       workspace.IsolationStrong,
			CrossWorkspaceAccess: false,
		}
		
		if err := cache.Initialize(workspaceRoot, cacheConfig); err != nil {
			t.Fatalf("Failed to initialize workspace cache for %s: %v", workspaceRoot, err)
		}
		
		caches[workspaceRoot] = cache
		defer cache.Close()
	}

	// Test cache isolation by storing different data in each workspace
	testData := map[string]string{
		workspaces[0]: "workspace1-specific-data",
		workspaces[1]: "workspace2-specific-data",
		workspaces[2]: "workspace3-specific-data",
	}

	// Store data in each workspace cache
	for workspaceRoot, data := range testData {
		cache := caches[workspaceRoot]
		
		entry := &storage.CacheEntry{
			Method:      "textDocument/definition",
			Response:    []byte(fmt.Sprintf(`{"data": "%s"}`, data)),
			ProjectPath: workspaceRoot,
			FilePaths:   []string{filepath.Join(workspaceRoot, "test.go")},
		}
		
		key := fmt.Sprintf("test-key-%s", workspaceRoot)
		if err := cache.Set(key, entry, 10*time.Minute); err != nil {
			t.Errorf("Failed to set cache entry for workspace %s: %v", workspaceRoot, err)
		}
	}

	// Verify isolation: each workspace should only see its own data
	for workspaceRoot, expectedData := range testData {
		cache := caches[workspaceRoot]
		key := fmt.Sprintf("test-key-%s", workspaceRoot)
		
		entry, found := cache.Get(key)
		if !found {
			t.Errorf("Cache entry not found for workspace %s", workspaceRoot)
			continue
		}
		
		expectedResponse := fmt.Sprintf(`{"data": "%s"}`, expectedData)
		if string(entry.Response) != expectedResponse {
			t.Errorf("Cache isolation failed for %s: expected %s, got %s", 
				workspaceRoot, expectedResponse, string(entry.Response))
		}
		
		// Verify other workspaces cannot access this data
		for otherWorkspace, otherCache := range caches {
			if otherWorkspace == workspaceRoot {
				continue
			}
			
			if otherEntry, found := otherCache.Get(key); found {
				t.Errorf("Cache isolation violation: workspace %s can access data from %s: %s",
					otherWorkspace, workspaceRoot, string(otherEntry.Response))
			}
		}
	}

	// Verify workspace stats isolation
	for workspaceRoot, cache := range caches {
		stats := cache.GetStats()
		
		if stats.WorkspaceRoot != workspaceRoot {
			t.Errorf("Workspace stats mismatch: expected %s, got %s", 
				workspaceRoot, stats.WorkspaceRoot)
		}
		
		if stats.TotalEntries != 1 {
			t.Errorf("Expected 1 cache entry for workspace %s, got %d", 
				workspaceRoot, stats.TotalEntries)
		}
		
		if stats.HitRate == 0 {
			t.Errorf("Expected non-zero hit rate for workspace %s", workspaceRoot)
		}
		
		t.Logf("Workspace %s stats: entries=%d, hit_rate=%.2f%%, memory=%dMB",
			workspaceRoot, stats.TotalEntries, stats.HitRate*100, 
			stats.TotalSize/(1024*1024))
	}
}

// testWorkspaceQueryRouting tests SCIP query routing to appropriate workspace caches
func testWorkspaceQueryRouting(t *testing.T, suite *SCIPTestSuite) {
	workspaces := []struct {
		root     string
		language string
		files    []string
	}{
		{
			root:     filepath.Join(suite.tempDir, "go-workspace"),
			language: "go",
			files:    []string{"main.go", "user.go", "service.go"},
		},
		{
			root:     filepath.Join(suite.tempDir, "ts-workspace"),  
			language: "typescript",
			files:    []string{"index.ts", "user.ts", "service.ts"},
		},
		{
			root:     filepath.Join(suite.tempDir, "py-workspace"),
			language: "python",
			files:    []string{"main.py", "user.py", "service.py"},
		},
	}

	// Create workspace SCIP stores for query routing
	stores := make(map[string]*workspace.WorkspaceSCIPStore)
	
	for _, ws := range workspaces {
		configManager := &TestWorkspaceConfigManager{
			workspaceRoot: ws.root,
		}
		
		scipFactory := workspace.NewWorkspaceSCIPFactory(configManager)
		
		scipConfig := &indexing.SCIPConfig{
			CacheConfig: indexing.CacheConfig{
				Enabled: true,
				MaxSize: 1000,
				TTL:     20 * time.Minute,
			},
			Performance: indexing.PerformanceConfig{
				QueryTimeout:         suite.config.WorkspaceQueryTimeout,
				MaxConcurrentQueries: 10,
				IndexLoadTimeout:     2 * time.Minute,
			},
		}
		
		store, err := scipFactory.CreateStoreWithConfig(ws.root, scipConfig)
		if err != nil {
			t.Fatalf("Failed to create workspace SCIP store for %s: %v", ws.root, err)
		}
		
		workspaceStore := store.(*workspace.WorkspaceSCIPStore)
		stores[ws.root] = workspaceStore
		defer store.Close()
	}

	// Test query routing for each workspace
	for _, ws := range workspaces {
		store := stores[ws.root]
		
		for _, file := range ws.files {
			// Test definition query routing
			t.Run(fmt.Sprintf("Definition_%s_%s", ws.language, file), func(t *testing.T) {
				uri := fmt.Sprintf("file://%s", filepath.Join(ws.root, file))
				
				params := map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": uri,
					},
					"position": map[string]interface{}{
						"line":      10,
						"character": 15,
					},
				}
				
				startTime := time.Now()
				result := store.Query("textDocument/definition", params)
				queryTime := time.Since(startTime)
				
				// Verify query was routed to correct workspace
				if result.Metadata != nil {
					if workspaceRoot, ok := result.Metadata["workspace_root"]; ok {
						if workspaceRoot != ws.root {
							t.Errorf("Query routing error: expected workspace %s, got %s",
								ws.root, workspaceRoot)
						}
					}
				}
				
				// Verify performance within workspace limits
				if queryTime > suite.config.WorkspaceQueryTimeout {
					t.Errorf("Workspace query timeout exceeded: %v > %v for %s/%s",
						queryTime, suite.config.WorkspaceQueryTimeout, ws.language, file)
				}
				
				// Record workspace metrics
				suite.recordWorkspaceQueryMetrics(ws.root, queryTime, result.CacheHit, result.Found)
				
				t.Logf("Query routed to workspace %s (%s/%s): %v, cache_hit=%v",
					ws.root, ws.language, file, queryTime, result.CacheHit)
			})
			
			// Test references query routing
			t.Run(fmt.Sprintf("References_%s_%s", ws.language, file), func(t *testing.T) {
				uri := fmt.Sprintf("file://%s", filepath.Join(ws.root, file))
				
				params := map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": uri,
					},
					"position": map[string]interface{}{
						"line":      5,
						"character": 8,
					},
					"context": map[string]interface{}{
						"includeDeclaration": false,
					},
				}
				
				startTime := time.Now()
				result := store.Query("textDocument/references", params)
				queryTime := time.Since(startTime)
				
				// Verify workspace routing
				if result.Metadata != nil {
					if workspaceRoot, ok := result.Metadata["workspace_root"]; ok {
						if workspaceRoot != ws.root {
							t.Errorf("References query routing error: expected workspace %s, got %s",
								ws.root, workspaceRoot)
						}
					}
				}
				
				if queryTime > suite.config.WorkspaceQueryTimeout {
					t.Errorf("Workspace references query timeout: %v > %v for %s/%s",
						queryTime, suite.config.WorkspaceQueryTimeout, ws.language, file)
				}
				
				suite.recordWorkspaceQueryMetrics(ws.root, queryTime, result.CacheHit, result.Found)
			})
		}
	}
}

// testWorkspaceCachePerformance tests performance with multiple sub-projects
func testWorkspaceCachePerformance(t *testing.T, suite *SCIPTestSuite) {
	if !suite.config.WorkspacePerformanceTests {
		t.Skip("Workspace performance tests disabled")
	}

	workspace := filepath.Join(suite.tempDir, "perf-workspace")
	configManager := &TestWorkspaceConfigManager{
		workspaceRoot: workspace,
	}

	// Create workspace cache with performance monitoring
	cache := workspace.NewWorkspaceSCIPCache(configManager)
	
	perfConfig := &workspace.CacheConfig{
		MaxMemorySize:        100 * 1024 * 1024, // 100MB
		MaxMemoryEntries:     50000,
		MemoryTTL:            30 * time.Minute,
		MaxDiskSize:          500 * 1024 * 1024, // 500MB
		MaxDiskEntries:       500000,
		DiskTTL:              8 * time.Hour,
		CompressionEnabled:   true,
		CompressionThreshold: 512,
		CompressionType:      "lz4",
		CleanupInterval:      1 * time.Minute,
		MetricsInterval:      5 * time.Second,
		SyncInterval:         15 * time.Second,
		IsolationLevel:       workspace.IsolationStrong,
		CrossWorkspaceAccess: false,
	}
	
	if err := cache.Initialize(workspace, perfConfig); err != nil {
		t.Fatalf("Failed to initialize performance workspace cache: %v", err)
	}
	defer cache.Close()

	// Simulate multiple sub-projects with different languages
	subProjects := []struct {
		name     string
		language string
		fileCount int
	}{
		{"frontend", "typescript", 50},
		{"backend", "go", 30},
		{"api", "python", 25},
		{"data", "java", 20},
		{"scripts", "python", 15},
	}

	// Performance test: concurrent cache operations
	t.Run("ConcurrentCacheOperations", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var wg sync.WaitGroup
		results := make(chan PerformanceResult, 1000)
		semaphore := make(chan struct{}, 20) // Limit concurrency

		requestCount := 0
		
		for {
			select {
			case <-ctx.Done():
				goto done
			case semaphore <- struct{}{}:
				requestCount++
				wg.Add(1)
				
				go func(reqID int) {
					defer wg.Done()
					defer func() { <-semaphore }()

					// Pick random sub-project
					project := subProjects[reqID%len(subProjects)]
					fileName := fmt.Sprintf("%s/file_%d.%s", project.name, 
						reqID%project.fileCount, getFileExtension(project.language))
					
					result := executeWorkspaceCacheOperation(cache, workspace, project.language, fileName, reqID)
					results <- result
				}(requestCount)
			}
		}

	done:
		go func() {
			wg.Wait()
			close(results)
		}()

		// Analyze performance results
		var allResults []PerformanceResult
		for result := range results {
			allResults = append(allResults, result)
		}

		analyzeWorkspacePerformance(t, allResults, cache)
	})

	// Memory pressure test
	t.Run("MemoryPressureHandling", func(t *testing.T) {
		initialStats := cache.GetStats()
		
		// Fill cache with large entries to test eviction
		for i := 0; i < 10000; i++ {
			largeData := make([]byte, 10*1024) // 10KB entries
			for j := range largeData {
				largeData[j] = byte(i % 256)
			}
			
			entry := &storage.CacheEntry{
				Method:      "textDocument/documentSymbol",
				Response:    largeData,
				ProjectPath: workspace,
				FilePaths:   []string{fmt.Sprintf("%s/large_file_%d.go", workspace, i)},
			}
			
			key := fmt.Sprintf("large-entry-%d", i)
			if err := cache.Set(key, entry, 5*time.Minute); err != nil {
				t.Logf("Cache set failed at entry %d: %v", i, err)
				break
			}
			
			// Check memory usage periodically
			if i%1000 == 0 {
				stats := cache.GetStats()
				memoryUsageMB := stats.TotalSize / (1024 * 1024)
				
				if memoryUsageMB > suite.config.WorkspaceMemoryLimit {
					t.Logf("Memory limit reached at entry %d: %dMB", i, memoryUsageMB)
					break
				}
			}
		}
		
		finalStats := cache.GetStats()
		
		// Verify eviction policies worked
		if finalStats.TotalEvictions <= initialStats.TotalEvictions {
			t.Error("Expected cache evictions under memory pressure")
		}
		
		// Verify performance maintained
		if finalStats.AverageLatency > 20*time.Millisecond {
			t.Errorf("Average latency degraded under pressure: %v", finalStats.AverageLatency)
		}
		
		t.Logf("Memory pressure test: evictions=%d, final_memory=%dMB, avg_latency=%v",
			finalStats.TotalEvictions-initialStats.TotalEvictions,
			finalStats.TotalSize/(1024*1024), finalStats.AverageLatency)
	})

	// Cache hit rate optimization test
	t.Run("CacheHitRateOptimization", func(t *testing.T) {
		// Simulate realistic access patterns
		accessPatterns := []struct {
			pattern string
			weight  int // how often this pattern occurs
		}{
			{"textDocument/definition", 40},
			{"textDocument/references", 25},
			{"textDocument/hover", 20},
			{"textDocument/documentSymbol", 10},
			{"workspace/symbol", 5},
		}
		
		// Warm up cache with realistic data
		for _, project := range subProjects {
			for i := 0; i < project.fileCount; i++ {
				fileName := fmt.Sprintf("%s/file_%d.%s", project.name, i, getFileExtension(project.language))
				
				for _, pattern := range accessPatterns {
					for w := 0; w < pattern.weight; w++ {
						entry := &storage.CacheEntry{
							Method:      pattern.pattern,
							Response:    []byte(fmt.Sprintf(`{"result": "%s_%s_%d"}`, pattern.pattern, fileName, w)),
							ProjectPath: workspace,
							FilePaths:   []string{filepath.Join(workspace, fileName)},
						}
						
						key := fmt.Sprintf("%s:%s:%d", pattern.pattern, fileName, w)
						cache.Set(key, entry, 15*time.Minute)
					}
				}
			}
		}
		
		// Test cache hit rates
		initialStats := cache.GetStats()
		hitCount := 0
		totalRequests := 1000
		
		for i := 0; i < totalRequests; i++ {
			project := subProjects[i%len(subProjects)]
			pattern := accessPatterns[i%len(accessPatterns)]
			fileName := fmt.Sprintf("%s/file_%d.%s", project.name, 
				i%project.fileCount, getFileExtension(project.language))
			
			key := fmt.Sprintf("%s:%s:%d", pattern.pattern, fileName, i%pattern.weight)
			
			if _, found := cache.Get(key); found {
				hitCount++
			}
		}
		
		hitRate := float64(hitCount) / float64(totalRequests)
		finalStats := cache.GetStats()
		
		// Validate performance targets
		if hitRate < 0.85 { // 85% hit rate target
			t.Errorf("Cache hit rate %.2f%% below target 85%%", hitRate*100)
		}
		
		if finalStats.AverageLatency > 10*time.Millisecond {
			t.Errorf("Average latency %v exceeds target 10ms", finalStats.AverageLatency)
		}
		
		t.Logf("Cache optimization test: hit_rate=%.2f%%, avg_latency=%v, total_entries=%d",
			hitRate*100, finalStats.AverageLatency, finalStats.TotalEntries)
	})
}

// testWorkspaceMemoryManagement tests workspace SCIP cache memory usage and limits
func testWorkspaceMemoryManagement(t *testing.T, suite *SCIPTestSuite) {
	workspace := filepath.Join(suite.tempDir, "memory-workspace")
	configManager := &TestWorkspaceConfigManager{
		workspaceRoot: workspace,
	}

	// Test with constrained memory limits
	memoryConfig := &workspace.CacheConfig{
		MaxMemorySize:        20 * 1024 * 1024, // 20MB limit
		MaxMemoryEntries:     5000,
		MemoryTTL:            10 * time.Minute,
		MaxDiskSize:          100 * 1024 * 1024, // 100MB
		MaxDiskEntries:       50000,
		DiskTTL:              2 * time.Hour,
		CompressionEnabled:   true,
		CompressionThreshold: 256,
		CompressionType:      "lz4",
		CleanupInterval:      30 * time.Second,
		MetricsInterval:      5 * time.Second,
		SyncInterval:         10 * time.Second,
		IsolationLevel:       workspace.IsolationStrong,
		CrossWorkspaceAccess: false,
	}

	cache := workspace.NewWorkspaceSCIPCache(configManager)
	if err := cache.Initialize(workspace, memoryConfig); err != nil {
		t.Fatalf("Failed to initialize memory-constrained cache: %v", err)
	}
	defer cache.Close()

	// Test memory enforcement
	t.Run("MemoryLimitEnforcement", func(t *testing.T) {
		entrySize := 4 * 1024 // 4KB entries
		maxEntries := (20 * 1024 * 1024) / entrySize // Should fit ~5000 entries
		
		for i := 0; i < maxEntries*2; i++ { // Try to exceed limit
			data := make([]byte, entrySize)
			for j := range data {
				data[j] = byte(i % 256)
			}
			
			entry := &storage.CacheEntry{
				Method:      "textDocument/definition",
				Response:    data,
				ProjectPath: workspace,
				FilePaths:   []string{fmt.Sprintf("%s/test_%d.go", workspace, i)},
			}
			
			key := fmt.Sprintf("memory-test-%d", i)
			if err := cache.Set(key, entry, 5*time.Minute); err != nil {
				t.Logf("Memory limit reached at entry %d: %v", i, err)
				break
			}
			
			// Check if we've exceeded memory limits
			if i%500 == 0 {
				stats := cache.GetStats()
				memoryUsage := stats.L1Stats.SizeBytes
				
				if memoryUsage > memoryConfig.MaxMemorySize {
					t.Errorf("Memory limit exceeded: %d > %d at entry %d",
						memoryUsage, memoryConfig.MaxMemorySize, i)
				}
				
				t.Logf("Memory usage at entry %d: %dMB/%dMB, entries=%d",
					i, memoryUsage/(1024*1024), memoryConfig.MaxMemorySize/(1024*1024),
					stats.L1Stats.EntryCount)
			}
		}
		
		finalStats := cache.GetStats()
		
		// Verify memory management worked
		if finalStats.L1Stats.SizeBytes > memoryConfig.MaxMemorySize {
			t.Errorf("Final memory usage %d exceeds limit %d",
				finalStats.L1Stats.SizeBytes, memoryConfig.MaxMemorySize)
		}
		
		if finalStats.TotalEvictions == 0 {
			t.Error("Expected memory evictions but none occurred")
		}
		
		t.Logf("Memory management: final_memory=%dMB, evictions=%d, entries=%d",
			finalStats.L1Stats.SizeBytes/(1024*1024), finalStats.TotalEvictions,
			finalStats.L1Stats.EntryCount)
	})

	// Test tier promotion/demotion
	t.Run("TierMemoryManagement", func(t *testing.T) {
		// Fill L1 cache to capacity
		for i := 0; i < 1000; i++ {
			entry := &storage.CacheEntry{
				Method:      "textDocument/hover",
				Response:    []byte(fmt.Sprintf(`{"hover": "data_%d"}`, i)),
				ProjectPath: workspace,
				FilePaths:   []string{fmt.Sprintf("%s/hover_%d.go", workspace, i)},
			}
			
			key := fmt.Sprintf("tier-test-%d", i)
			cache.Set(key, entry, 5*time.Minute)
		}
		
		midStats := cache.GetStats()
		
		// Access some entries multiple times to trigger promotion
		hotKeys := []string{"tier-test-0", "tier-test-1", "tier-test-2"}
		for _, key := range hotKeys {
			for j := 0; j < 10; j++ { // Access 10 times
				cache.Get(key)
			}
		}
		
		// Add more entries to trigger demotion
		for i := 1000; i < 1500; i++ {
			entry := &storage.CacheEntry{
				Method:      "textDocument/documentSymbol",
				Response:    []byte(fmt.Sprintf(`{"symbols": "data_%d"}`, i)),  
				ProjectPath: workspace,
				FilePaths:   []string{fmt.Sprintf("%s/symbols_%d.go", workspace, i)},
			}
			
			key := fmt.Sprintf("tier-test-%d", i)
			cache.Set(key, entry, 5*time.Minute)
		}
		
		finalStats := cache.GetStats()
		
		// Verify tier management
		if finalStats.TotalPromotions <= midStats.TotalPromotions {
			t.Error("Expected cache promotions during tier management")
		}
		
		// Hot keys should still be accessible (either in L1 or L2)
		for _, key := range hotKeys {
			if _, found := cache.Get(key); !found {
				t.Errorf("Hot key %s was evicted unexpectedly", key)
			}
		}
		
		t.Logf("Tier management: L1_entries=%d, L2_entries=%d, promotions=%d",
			finalStats.L1Stats.EntryCount, finalStats.L2Stats.EntryCount,
			finalStats.TotalPromotions)
	})
}

// testWorkspaceIndexLoading tests SCIP index loading and management per workspace
func testWorkspaceIndexLoading(t *testing.T, suite *SCIPTestSuite) {
	workspaces := []string{
		filepath.Join(suite.tempDir, "index-workspace-1"),
		filepath.Join(suite.tempDir, "index-workspace-2"),
		filepath.Join(suite.tempDir, "index-workspace-3"),  
	}

	for _, workspaceRoot := range workspaces {
		t.Run(fmt.Sprintf("IndexLoading_%s", filepath.Base(workspaceRoot)), func(t *testing.T) {
			configManager := &TestWorkspaceConfigManager{
				workspaceRoot: workspaceRoot,
			}
			
			scipFactory := workspace.NewWorkspaceSCIPFactory(configManager)
			
			scipConfig := &indexing.SCIPConfig{
				CacheConfig: indexing.CacheConfig{
					Enabled: true,
					MaxSize: 5000,
					TTL:     20 * time.Minute,
				},
				Performance: indexing.PerformanceConfig{
					QueryTimeout:         15 * time.Millisecond,
					MaxConcurrentQueries: 5,
					IndexLoadTimeout:     1 * time.Minute,
				},
			}
			
			store, err := scipFactory.CreateStoreWithConfig(workspaceRoot, scipConfig)
			if err != nil {
				t.Fatalf("Failed to create workspace SCIP store: %v", err)
			}
			defer store.Close()
			
			workspaceStore := store.(*workspace.WorkspaceSCIPStore)
			
			// Test workspace info loading
			workspaceInfo := workspaceStore.GetWorkspaceInfo()
			if workspaceInfo == nil {
				t.Error("Workspace info should be available")
			} else {
				if workspaceInfo.RootPath != workspaceRoot {
					t.Errorf("Workspace root mismatch: expected %s, got %s",
						workspaceRoot, workspaceInfo.RootPath)
				}
				
				t.Logf("Workspace info: id=%s, name=%s, root=%s",
					workspaceInfo.WorkspaceID, workspaceInfo.Name, workspaceInfo.RootPath)
			}
			
			// Test index loading performance
			startTime := time.Now()
			
			// Simulate loading SCIP index
			indexPath := filepath.Join(suite.testDataDir, "scip_indices", "go-test-project.scip")
			if err := store.LoadIndex(indexPath); err != nil {
				t.Logf("Index loading warning (expected in test): %v", err)
			}
			
			loadTime := time.Since(startTime)
			
			if loadTime > scipConfig.Performance.IndexLoadTimeout {
				t.Errorf("Index loading timeout: %v > %v", loadTime, scipConfig.Performance.IndexLoadTimeout)
			}
			
			// Test index queries after loading
			testQueries := []struct {
				method string
				params map[string]interface{}
			}{
				{
					method: "textDocument/definition",
					params: map[string]interface{}{
						"textDocument": map[string]interface{}{
							"uri": fmt.Sprintf("file://%s/main.go", workspaceRoot),
						},
						"position": map[string]interface{}{
							"line":      10,
							"character": 15,
						},
					},
				},
				{
					method: "workspace/symbol",
					params: map[string]interface{}{
						"query": "User",
					},
				},
			}
			
			for _, query := range testQueries {
				queryStart := time.Now()
				result := store.Query(query.method, query.params)
				queryTime := time.Since(queryStart)
				
				if queryTime > scipConfig.Performance.QueryTimeout {
					t.Errorf("Query timeout for %s: %v > %v", query.method, queryTime, scipConfig.Performance.QueryTimeout)
				}
				
				// Verify workspace context in result
				if result.Metadata != nil {
					if wsRoot, ok := result.Metadata["workspace_root"]; ok {
						if wsRoot != workspaceRoot {
							t.Errorf("Query result workspace mismatch: expected %s, got %s",
								workspaceRoot, wsRoot)
						}
					}
				}
				
				t.Logf("Query %s for workspace %s: %v, found=%v",
					query.method, filepath.Base(workspaceRoot), queryTime, result.Found)
			}
			
			// Test stats
			stats := store.GetStats()
			
			if stats.IndexesLoaded == 0 {
				t.Error("Expected at least 1 logical index loaded")
			}
			
			if stats.TotalQueries != int64(len(testQueries)) {
				t.Errorf("Query count mismatch: expected %d, got %d",
					len(testQueries), stats.TotalQueries)
			}
			
			t.Logf("Index loading completed for %s: load_time=%v, queries=%d, hit_rate=%.2f%%",
				filepath.Base(workspaceRoot), loadTime, stats.TotalQueries, stats.CacheHitRate*100)
		})
	}
}

// testCrossProjectSymbolResolution tests cross-project symbol resolution within workspace
func testCrossProjectSymbolResolution(t *testing.T, suite *SCIPTestSuite) {
	workspace := filepath.Join(suite.tempDir, "multi-project-workspace")
	configManager := &TestWorkspaceConfigManager{
		workspaceRoot: workspace,
	}

	scipFactory := workspace.NewWorkspaceSCIPFactory(configManager)
	
	scipConfig := &indexing.SCIPConfig{
		CacheConfig: indexing.CacheConfig{
			Enabled: true,
			MaxSize: 10000,
			TTL:     30 * time.Minute,
		},
		Performance: indexing.PerformanceConfig{
			QueryTimeout:         20 * time.Millisecond,
			MaxConcurrentQueries: 10,
			IndexLoadTimeout:     2 * time.Minute,
		},
	}
	
	store, err := scipFactory.CreateStoreWithConfig(workspace, scipConfig)
	if err != nil {
		t.Fatalf("Failed to create multi-project workspace store: %v", err)
	}
	defer store.Close()

	// Simulate multiple projects within the workspace
	projects := []struct {
		name      string
		language  string
		symbols   []string
	}{
		{
			name:     "frontend",
			language: "typescript",
			symbols:  []string{"UserComponent", "UserService", "UserRepository"},
		},
		{
			name:     "backend",
			language: "go",
			symbols:  []string{"UserHandler", "UserService", "UserModel"},
		},
		{
			name:     "shared",
			language: "typescript",
			symbols:  []string{"UserInterface", "UserValidation", "UserUtils"},
		},
		{
			name:     "data",
			language: "python",
			symbols:  []string{"UserSchema", "UserMigration", "UserSeeder"},
		},
	}

	// Pre-populate cache with cross-project symbol data
	for _, project := range projects {
		for _, symbol := range project.symbols {
			// Definition entry
			defEntry := &storage.CacheEntry{
				Method:      "textDocument/definition",
				Response:    []byte(fmt.Sprintf(`[{"uri": "file://%s/%s/main.%s", "range": {"start": {"line": 10, "character": 0}, "end": {"line": 10, "character": %d}}}]`, 
					workspace, project.name, getFileExtension(project.language), len(symbol))),
				ProjectPath: workspace,
				FilePaths:   []string{fmt.Sprintf("%s/%s/main.%s", workspace, project.name, getFileExtension(project.language))},
			}
			
			// References entry
			refEntry := &storage.CacheEntry{
				Method:      "textDocument/references",
				Response:    []byte(fmt.Sprintf(`[{"uri": "file://%s/%s/main.%s", "range": {"start": {"line": 15, "character": 5}, "end": {"line": 15, "character": %d}}}]`,
					workspace, project.name, getFileExtension(project.language), 5+len(symbol))),
				ProjectPath: workspace,
				FilePaths:   []string{fmt.Sprintf("%s/%s/main.%s", workspace, project.name, getFileExtension(project.language))},
			}
			
			// Workspace symbol entry
			wsEntry := &storage.CacheEntry{
				Method:      "workspace/symbol",
				Response:    []byte(fmt.Sprintf(`[{"name": "%s", "kind": 5, "location": {"uri": "file://%s/%s/main.%s", "range": {"start": {"line": 10, "character": 0}, "end": {"line": 10, "character": %d}}}}]`,
					symbol, workspace, project.name, getFileExtension(project.language), len(symbol))),
				ProjectPath: workspace,
				FilePaths:   []string{fmt.Sprintf("%s/%s/main.%s", workspace, project.name, getFileExtension(project.language))},
			}

			cache := store.(*workspace.WorkspaceSCIPStore)
			cache.CacheResponse("textDocument/definition", 
				map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": fmt.Sprintf("file://%s/%s/main.%s", workspace, project.name, getFileExtension(project.language)),
					},
					"position": map[string]interface{}{"line": 10, "character": 0},
				}, defEntry.Response)
			
			cache.CacheResponse("textDocument/references",
				map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": fmt.Sprintf("file://%s/%s/main.%s", workspace, project.name, getFileExtension(project.language)),
					},
					"position": map[string]interface{}{"line": 10, "character": 0},
					"context": map[string]interface{}{"includeDeclaration": false},
				}, refEntry.Response)
			
			cache.CacheResponse("workspace/symbol",
				map[string]interface{}{"query": symbol}, wsEntry.Response)
		}
	}

	// Test cross-project symbol resolution
	t.Run("CrossProjectDefinitionLookup", func(t *testing.T) {
		for _, project := range projects {
			for _, symbol := range project.symbols {
				// Look up definition across projects
				params := map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": fmt.Sprintf("file://%s/%s/main.%s", workspace, project.name, getFileExtension(project.language)),
					},
					"position": map[string]interface{}{
						"line":      10,
						"character": 0,
					},
				}
				
				result := store.Query("textDocument/definition", params)
				
				if !result.Found {
					t.Errorf("Cross-project definition not found for %s in %s", symbol, project.name)
					continue
				}
				
				if !result.CacheHit {
					t.Logf("Cache miss for cross-project definition: %s/%s", project.name, symbol)
				}
				
				// Verify workspace context
				if result.Metadata != nil {
					if wsRoot, ok := result.Metadata["workspace_root"]; ok {
						if wsRoot != workspace {
							t.Errorf("Cross-project query workspace mismatch: expected %s, got %s",
								workspace, wsRoot)
						}
					}
				}
				
				t.Logf("Cross-project definition found: %s/%s, cache_hit=%v",
					project.name, symbol, result.CacheHit)
			}
		}
	})

	t.Run("CrossProjectWorkspaceSymbolSearch", func(t *testing.T) {
		// Search for symbols that should exist across multiple projects
		commonSymbols := []string{"User", "Service", "Repository"}
		
		for _, symbolQuery := range commonSymbols {
			result := store.Query("workspace/symbol", map[string]interface{}{
				"query": symbolQuery,
			})
			
			if !result.Found {
				t.Errorf("Cross-project workspace symbol search failed for: %s", symbolQuery)
				continue
			}
			
			// Count projects that should contain this symbol
			expectedProjects := 0
			for _, project := range projects {
				for _, symbol := range project.symbols {
					if symbol == symbolQuery+"Component" || symbol == symbolQuery+"Service" || 
					   symbol == symbolQuery+"Repository" || symbol == symbolQuery+"Handler" ||
					   symbol == symbolQuery+"Model" || symbol == symbolQuery+"Interface" ||
					   symbol == symbolQuery+"Schema" {
						expectedProjects++
						break
					}
				}
			}
			
			t.Logf("Cross-project workspace symbol search: query=%s, found=%v, cache_hit=%v, expected_projects=%d",
				symbolQuery, result.Found, result.CacheHit, expectedProjects)
		}
	})

	t.Run("CrossProjectPerformanceAnalysis", func(t *testing.T) {
		// Test performance of cross-project queries
		testDuration := 10 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), testDuration)
		defer cancel()

		queryCount := 0
		cacheHits := 0
		totalLatency := time.Duration(0)
		
		for {
			select {
			case <-ctx.Done():
				goto done
			default:
				project := projects[queryCount%len(projects)]
				symbol := project.symbols[queryCount%len(project.symbols)]
				
				startTime := time.Now()
				
				params := map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": fmt.Sprintf("file://%s/%s/main.%s", workspace, project.name, getFileExtension(project.language)),
					},
					"position": map[string]interface{}{
						"line":      10,
						"character": 0,
					},
				}
				
				result := store.Query("textDocument/definition", params)
				queryTime := time.Since(startTime)
				
				queryCount++
				totalLatency += queryTime
				
				if result.CacheHit {
					cacheHits++
				}
				
				if queryTime > 20*time.Millisecond {
					t.Logf("Slow cross-project query: %s/%s took %v", project.name, symbol, queryTime)
				}
			}
		}

	done:
		if queryCount == 0 {
			t.Error("No cross-project queries executed")
			return
		}
		
		avgLatency := totalLatency / time.Duration(queryCount)
		hitRate := float64(cacheHits) / float64(queryCount)
		
		// Performance validation
		if avgLatency > 15*time.Millisecond {
			t.Errorf("Cross-project average latency %v exceeds target 15ms", avgLatency)
		}
		
		if hitRate < 0.80 { // 80% hit rate target for cross-project
			t.Errorf("Cross-project cache hit rate %.2f%% below target 80%%", hitRate*100)
		}
		
		t.Logf("Cross-project performance: queries=%d, avg_latency=%v, hit_rate=%.2f%%",
			queryCount, avgLatency, hitRate*100)
	})
}

// Helper functions

type TestWorkspaceConfigManager struct {
	workspaceRoot string
}

func (m *TestWorkspaceConfigManager) GetWorkspaceDirectory(workspaceRoot string) string {
	return filepath.Join(workspaceRoot, ".lsp-gateway")
}

func (m *TestWorkspaceConfigManager) LoadWorkspaceConfig(workspaceRoot string) (*WorkspaceConfig, error) {
	return &WorkspaceConfig{
		Workspace: workspace.WorkspaceInfo{
			WorkspaceID: fmt.Sprintf("test-ws-%s", filepath.Base(workspaceRoot)),
			Name:        filepath.Base(workspaceRoot),
			RootPath:    workspaceRoot,
			Hash:        fmt.Sprintf("hash-%s", filepath.Base(workspaceRoot)),
			CreatedAt:   time.Now(),
			LastUpdated: time.Now(),
			Version:     "1.0.0",
		},
	}, nil
}

func executeWorkspaceCacheOperation(cache WorkspaceSCIPCache, workspace, language, fileName string, reqID int) PerformanceResult {
	startTime := time.Now()
	
	// Simulate different cache operations
	operations := []string{"get", "set", "get", "set", "get"} // More gets than sets
	operation := operations[reqID%len(operations)]
	
	key := fmt.Sprintf("%s:%s:%s", language, fileName, operation)
	
	var success bool
	var errorMsg string
	
	switch operation {
	case "get":
		_, found := cache.Get(key)
		success = true // Getting is always successful, even if not found
		
	case "set":
		entry := &storage.CacheEntry{
			Method:      "textDocument/definition",
			Response:    []byte(fmt.Sprintf(`{"result": "%s_%s_%d"}`, language, fileName, reqID)),
			ProjectPath: workspace,
			FilePaths:   []string{filepath.Join(workspace, fileName)},
		}
		
		err := cache.Set(key, entry, 10*time.Minute)
		success = (err == nil)
		if err != nil {
			errorMsg = err.Error()
		}
	}
	
	return PerformanceResult{
		Language: language,
		Method:   operation,
		Duration: time.Since(startTime),
		Success:  success,
		Error:    errorMsg,
	}
}

func analyzeWorkspacePerformance(t *testing.T, results []PerformanceResult, cache WorkspaceSCIPCache) {
	if len(results) == 0 {
		t.Error("No workspace performance results to analyze")
		return
	}

	// Group by operation type
	opStats := make(map[string][]time.Duration)
	langStats := make(map[string][]time.Duration)
	successCount := 0
	
	for _, result := range results {
		if result.Success {
			successCount++
			opStats[result.Method] = append(opStats[result.Method], result.Duration)
			langStats[result.Language] = append(langStats[result.Language], result.Duration)
		}
	}
	
	successRate := float64(successCount) / float64(len(results)) * 100
	cacheStats := cache.GetStats()
	
	t.Logf("Workspace cache performance analysis:")
	t.Logf("  Total operations: %d", len(results))
	t.Logf("  Success rate: %.2f%%", successRate)
	t.Logf("  Cache hit rate: %.2f%%", cacheStats.HitRate*100)
	t.Logf("  Memory usage: %dMB", cacheStats.TotalSize/(1024*1024))
	
	// Operation performance analysis
	t.Logf("  Operation performance:")
	for op, durations := range opStats {
		if len(durations) > 0 {
			avg := calculateAverage(durations)
			p95 := calculatePercentile(durations, 95)
			t.Logf("    %s: avg=%v, p95=%v, samples=%d", op, avg, p95, len(durations))
		}
	}
	
	// Language performance analysis  
	t.Logf("  Language performance:")
	for lang, durations := range langStats {
		if len(durations) > 0 {
			avg := calculateAverage(durations)
			p95 := calculatePercentile(durations, 95)
			t.Logf("    %s: avg=%v, p95=%v, samples=%d", lang, avg, p95, len(durations))
		}
	}
	
	// Validate performance targets
	if successRate < 95.0 {
		t.Errorf("Workspace cache success rate %.2f%% below target 95%%", successRate)
	}
	
	if cacheStats.HitRate < 0.60 { // 60% hit rate target for mixed operations
		t.Errorf("Workspace cache hit rate %.2f%% below target 60%%", cacheStats.HitRate*100)
	}
}

func getFileExtension(language string) string {
	switch language {
	case "go":
		return "go"
	case "typescript":
		return "ts"
	case "python":
		return "py"
	case "java":
		return "java"
	default:
		return "txt"
	}
}

func (suite *SCIPTestSuite) recordWorkspaceQueryMetrics(workspaceRoot string, queryTime time.Duration, cacheHit, success bool) {
	suite.metrics.mutex.Lock()
	defer suite.metrics.mutex.Unlock()
	
	suite.metrics.WorkspaceQueries++
	
	if cacheHit {
		suite.metrics.WorkspaceCacheHits++
	}
	
	// Update or create workspace-specific metrics
	if suite.metrics.WorkspaceStats == nil {
		suite.metrics.WorkspaceStats = make(map[string]*WorkspaceMetrics)
	}
	
	wsMetrics, exists := suite.metrics.WorkspaceStats[workspaceRoot]
	if !exists {
		wsMetrics = &WorkspaceMetrics{
			WorkspaceRoot:  workspaceRoot,
			IsolationLevel: "strong",
		}
		suite.metrics.WorkspaceStats[workspaceRoot] = wsMetrics
	}
	
	wsMetrics.QueryCount++
	wsMetrics.LastQuery = time.Now()
	
	// Update average latency (simple moving average)
	if wsMetrics.AverageLatency == 0 {
		wsMetrics.AverageLatency = queryTime
	} else {
		wsMetrics.AverageLatency = (wsMetrics.AverageLatency + queryTime) / 2
	}
	
	// Update cache hit rate
	if cacheHit {
		hits := float64(wsMetrics.QueryCount-1) * wsMetrics.CacheHitRate + 1
		wsMetrics.CacheHitRate = hits / float64(wsMetrics.QueryCount)
	} else {
		hits := float64(wsMetrics.QueryCount-1) * wsMetrics.CacheHitRate
		wsMetrics.CacheHitRate = hits / float64(wsMetrics.QueryCount)
	}
}