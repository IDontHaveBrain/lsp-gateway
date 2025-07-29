package workspace

import (
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"lsp-gateway/internal/project"
	"lsp-gateway/internal/storage"
)

// BenchmarkWorkspaceStartup benchmarks workspace gateway startup performance
func BenchmarkWorkspaceStartup(b *testing.B) {
	testSuite := NewWorkspaceTestSuite()
	defer testSuite.Cleanup()

	languages := []string{"go", "python", "javascript"}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		workspaceName := fmt.Sprintf("bench-startup-%d", i)
		
		start := time.Now()
		err := testSuite.SetupWorkspace(workspaceName, languages)
		if err != nil {
			b.Fatalf("Failed to setup workspace: %v", err)
		}
		
		err = testSuite.StartWorkspace(workspaceName)
		if err != nil {
			b.Fatalf("Failed to start workspace: %v", err)
		}
		
		duration := time.Since(start)
		b.ReportMetric(float64(duration.Nanoseconds()), "startup_ns")
		
		testSuite.StopWorkspace(workspaceName)
	}
}

// BenchmarkPortAllocation benchmarks port allocation performance
func BenchmarkPortAllocation(b *testing.B) {
	portManager, err := NewWorkspacePortManager()
	if err != nil {
		b.Fatalf("Failed to create port manager: %v", err)
	}

	workspaceRoots := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		workspaceRoots[i] = fmt.Sprintf("/tmp/bench-workspace-%d", i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		start := time.Now()
		port, err := portManager.AllocatePort(workspaceRoots[i])
		duration := time.Since(start)
		
		if err != nil {
			b.Fatalf("Failed to allocate port: %v", err)
		}
		
		b.ReportMetric(float64(duration.Nanoseconds()), "allocation_ns")
		b.ReportMetric(float64(port), "port_allocated")
		
		portManager.ReleasePort(workspaceRoots[i])
	}
}

// BenchmarkConfigurationLoading benchmarks workspace configuration loading
func BenchmarkConfigurationLoading(b *testing.B) {
	testSuite := NewWorkspaceTestSuite()
	defer testSuite.Cleanup()

	// Pre-create workspace configurations
	configManager := testSuite.ConfigManager
	workspaceRoots := make([]string, b.N)
	
	for i := 0; i < b.N; i++ {
		workspaceRoot := fmt.Sprintf("/tmp/bench-config-%d", i)
		workspaceRoots[i] = workspaceRoot
		
		// Create test project context
		projectContext := &project.ProjectContext{
			ProjectType:  "go",
			Languages:    []string{"go"},
			RootPath:     workspaceRoot,
		}
		
		err := configManager.GenerateWorkspaceConfig(workspaceRoot, projectContext)
		if err != nil {
			b.Fatalf("Failed to generate config: %v", err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		start := time.Now()
		_, err := configManager.LoadWorkspaceConfig(workspaceRoots[i])
		duration := time.Since(start)
		
		if err != nil {
			b.Fatalf("Failed to load config: %v", err)
		}
		
		b.ReportMetric(float64(duration.Nanoseconds()), "config_load_ns")
	}
}

// BenchmarkJSONRPCHandling benchmarks JSON-RPC request processing
func BenchmarkJSONRPCHandling(b *testing.B) {
	testSuite := NewWorkspaceTestSuite()
	defer testSuite.Cleanup()

	workspaceName := "bench-jsonrpc"
	languages := []string{"go"}
	
	err := testSuite.SetupWorkspace(workspaceName, languages)
	if err != nil {
		b.Fatalf("Failed to setup workspace: %v", err)
	}
	
	err = testSuite.StartWorkspace(workspaceName)
	if err != nil {
		b.Fatalf("Failed to start workspace: %v", err)
	}
	
	err = testSuite.WaitForWorkspaceReady(workspaceName, 10*time.Second)
	if err != nil {
		b.Fatalf("Workspace not ready: %v", err)
	}

	// Test parameters for workspace/symbol request
	params := map[string]interface{}{
		"query": "test",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		start := time.Now()
		_, err := testSuite.SendJSONRPCRequest(workspaceName, "workspace/symbol", params)
		duration := time.Since(start)
		
		if err != nil {
			b.Fatalf("JSON-RPC request failed: %v", err)
		}
		
		b.ReportMetric(float64(duration.Nanoseconds()), "jsonrpc_ns")
	}
}

// BenchmarkLSPClientRouting benchmarks LSP client routing performance
func BenchmarkLSPClientRouting(b *testing.B) {
	testSuite := NewWorkspaceTestSuite()
	defer testSuite.Cleanup()

	workspaceName := "bench-routing"
	languages := []string{"go", "python", "javascript"}
	
	err := testSuite.SetupWorkspace(workspaceName, languages)
	if err != nil {
		b.Fatalf("Failed to setup workspace: %v", err)
	}
	
	err = testSuite.StartWorkspace(workspaceName)
	if err != nil {
		b.Fatalf("Failed to start workspace: %v", err)
	}

	gateway := testSuite.Gateways[workspaceName]
	languageList := []string{"go", "python", "javascript"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		language := languageList[i%len(languageList)]
		
		start := time.Now()
		client, exists := gateway.GetClient(language)
		duration := time.Since(start)
		
		if !exists {
			b.Fatalf("Client not found for language: %s", language)
		}
		
		if client == nil {
			b.Fatalf("Client is nil for language: %s", language)
		}
		
		b.ReportMetric(float64(duration.Nanoseconds()), "routing_ns")
	}
}

// BenchmarkConcurrentWorkspaces benchmarks concurrent workspace operations
func BenchmarkConcurrentWorkspaces(b *testing.B) {
	testSuite := NewWorkspaceTestSuite()
	defer testSuite.Cleanup()

	concurrency := 5
	workspaceNames := make([]string, concurrency)
	
	// Setup workspaces
	for i := 0; i < concurrency; i++ {
		workspaceName := fmt.Sprintf("bench-concurrent-%d", i)
		workspaceNames[i] = workspaceName
		
		err := testSuite.SetupWorkspace(workspaceName, []string{"go"})
		if err != nil {
			b.Fatalf("Failed to setup workspace %s: %v", workspaceName, err)
		}
		
		err = testSuite.StartWorkspace(workspaceName)
		if err != nil {
			b.Fatalf("Failed to start workspace %s: %v", workspaceName, err)
		}
	}

	// Wait for all workspaces to be ready
	for _, workspaceName := range workspaceNames {
		err := testSuite.WaitForWorkspaceReady(workspaceName, 10*time.Second)
		if err != nil {
			b.Fatalf("Workspace %s not ready: %v", workspaceName, err)
		}
	}

	params := map[string]interface{}{
		"query": "test",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		start := time.Now()
		
		for _, workspaceName := range workspaceNames {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				_, err := testSuite.SendJSONRPCRequest(name, "workspace/symbol", params)
				if err != nil {
					b.Errorf("Request failed for workspace %s: %v", name, err)
				}
			}(workspaceName)
		}
		
		wg.Wait()
		duration := time.Since(start)
		b.ReportMetric(float64(duration.Nanoseconds()), "concurrent_ns")
	}
}

// TestWorkspaceStartupPerformance validates startup performance requirements
func TestWorkspaceStartupPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	testSuite := NewWorkspaceTestSuite()
	defer testSuite.Cleanup()

	languages := []string{"go", "python"}
	workspaceName := "perf-startup"
	
	// Measure startup time
	start := time.Now()
	err := testSuite.SetupWorkspace(workspaceName, languages)
	if err != nil {
		t.Fatalf("Failed to setup workspace: %v", err)
	}
	
	err = testSuite.StartWorkspace(workspaceName)
	if err != nil {
		t.Fatalf("Failed to start workspace: %v", err)
	}
	
	err = testSuite.WaitForWorkspaceReady(workspaceName, 10*time.Second)
	if err != nil {
		t.Fatalf("Workspace not ready: %v", err)
	}
	
	startupDuration := time.Since(start)
	
	// Validate startup time requirement: < 5 seconds
	maxStartupTime := 5 * time.Second
	if startupDuration > maxStartupTime {
		t.Errorf("Workspace startup took %v, expected < %v", startupDuration, maxStartupTime)
	}
	
	t.Logf("Workspace startup completed in %v", startupDuration)
}

// TestPortAllocationPerformance validates port allocation performance
func TestPortAllocationPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	portManager, err := NewWorkspacePortManager()
	if err != nil {
		t.Fatalf("Failed to create port manager: %v", err)
	}

	const numWorkspaces = 20
	workspaceRoots := make([]string, numWorkspaces)
	allocatedPorts := make([]int, numWorkspaces)
	
	// Measure allocation time
	start := time.Now()
	for i := 0; i < numWorkspaces; i++ {
		workspaceRoot := fmt.Sprintf("/tmp/perf-port-%d", i)
		workspaceRoots[i] = workspaceRoot
		
		port, err := portManager.AllocatePort(workspaceRoot)
		if err != nil {
			t.Fatalf("Failed to allocate port for workspace %d: %v", i, err)
		}
		allocatedPorts[i] = port
	}
	totalAllocationTime := time.Since(start)
	
	// Validate allocation time requirement: < 100ms per allocation
	avgAllocationTime := totalAllocationTime / numWorkspaces
	maxAllocationTime := 100 * time.Millisecond
	if avgAllocationTime > maxAllocationTime {
		t.Errorf("Average port allocation took %v, expected < %v", avgAllocationTime, maxAllocationTime)
	}
	
	// Cleanup
	for _, workspaceRoot := range workspaceRoots {
		portManager.ReleasePort(workspaceRoot)
	}
	
	t.Logf("Port allocation: %d workspaces in %v (avg: %v per allocation)", 
		numWorkspaces, totalAllocationTime, avgAllocationTime)
}

// TestMemoryUsageUnderLoad validates memory usage requirements
func TestMemoryUsageUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	testSuite := NewWorkspaceTestSuite()
	defer testSuite.Cleanup()

	// Capture initial memory
	runtime.GC()
	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)

	const numWorkspaces = 5
	workspaceNames := make([]string, numWorkspaces)
	
	// Create multiple workspaces
	for i := 0; i < numWorkspaces; i++ {
		workspaceName := fmt.Sprintf("perf-memory-%d", i)
		workspaceNames[i] = workspaceName
		
		err := testSuite.SetupWorkspace(workspaceName, []string{"go"})
		if err != nil {
			t.Fatalf("Failed to setup workspace %s: %v", workspaceName, err)
		}
		
		err = testSuite.StartWorkspace(workspaceName)
		if err != nil {
			t.Fatalf("Failed to start workspace %s: %v", workspaceName, err)
		}
	}

	// Force GC and measure memory
	runtime.GC()
	var loadMem runtime.MemStats
	runtime.ReadMemStats(&loadMem)
	
	memoryIncrease := loadMem.Alloc - initialMem.Alloc
	memoryPerWorkspace := memoryIncrease / numWorkspaces
	
	// Validate memory usage requirement: < 100MB per workspace
	maxMemoryPerWorkspace := uint64(100 * 1024 * 1024) // 100MB
	if memoryPerWorkspace > maxMemoryPerWorkspace {
		t.Errorf("Memory per workspace: %d bytes, expected < %d bytes", 
			memoryPerWorkspace, maxMemoryPerWorkspace)
	}
	
	t.Logf("Memory usage: %d workspaces using %d bytes total (%d bytes per workspace)", 
		numWorkspaces, memoryIncrease, memoryPerWorkspace)
}

// TestCachePerformanceIsolation validates SCIP cache performance and isolation
func TestCachePerformanceIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	configManager := NewWorkspaceConfigManager()
	
	// Create two isolated workspace caches
	cache1 := NewWorkspaceSCIPCache(configManager)
	cache2 := NewWorkspaceSCIPCache(configManager)
	
	defer cache1.Close()
	defer cache2.Close()
	
	workspace1 := "/tmp/cache-perf-1"
	workspace2 := "/tmp/cache-perf-2"
	
	// Initialize caches
	err := cache1.Initialize(workspace1, nil)
	if err != nil {
		t.Fatalf("Failed to initialize cache1: %v", err)
	}
	
	err = cache2.Initialize(workspace2, nil)
	if err != nil {
		t.Fatalf("Failed to initialize cache2: %v", err)
	}

	// Test cache isolation by setting different values for same key
	key := "test-key"
	now := time.Now()
	
	// Create proper storage.CacheEntry instances
	response1, _ := json.Marshal("workspace1-data")
	entry1 := &storage.CacheEntry{
		Method:     "test/method",
		Params:     "{}",
		Response:   response1,
		CreatedAt:  now,
		AccessedAt: now,
		TTL:        time.Hour,
		Key:        key,
	}
	
	response2, _ := json.Marshal("workspace2-data")
	entry2 := &storage.CacheEntry{
		Method:     "test/method",
		Params:     "{}",
		Response:   response2,
		CreatedAt:  now,
		AccessedAt: now,
		TTL:        time.Hour,
		Key:        key,
	}
	
	err = cache1.Set(key, entry1, time.Hour)
	if err != nil {
		t.Fatalf("Failed to set cache1 entry: %v", err)
	}
	
	err = cache2.Set(key, entry2, time.Hour)
	if err != nil {
		t.Fatalf("Failed to set cache2 entry: %v", err)
	}
	
	// Verify isolation
	retrieved1, exists1 := cache1.Get(key)
	if !exists1 {
		t.Fatalf("Entry not found in cache1")
	}
	
	retrieved2, exists2 := cache2.Get(key)
	if !exists2 {
		t.Fatalf("Entry not found in cache2")
	}
	
	// Compare the response data to verify isolation
	var data1, data2 string
	json.Unmarshal(retrieved1.Response, &data1)
	json.Unmarshal(retrieved2.Response, &data2)
	
	if data1 == data2 {
		t.Errorf("Cache isolation failed: both caches returned same value")
	}
	
	// Test performance - cache hit ratio should be > 80%
	const numOperations = 100
	for i := 0; i < numOperations; i++ {
		testKey := fmt.Sprintf("perf-key-%d", i%10) // 10 unique keys, repeated
		testResponse, _ := json.Marshal(fmt.Sprintf("data-%d", i))
		testEntry := &storage.CacheEntry{
			Method:     "test/method",
			Params:     "{}",
			Response:   testResponse,
			CreatedAt:  time.Now(),
			AccessedAt: time.Now(),
			TTL:        time.Hour,
			Key:        testKey,
		}
		
		cache1.Set(testKey, testEntry, time.Hour)
		cache1.Get(testKey) // This should be a hit
	}
	
	stats := cache1.GetStats()
	hitRate := stats.HitRate
	minHitRate := 0.8 // 80%
	
	if hitRate < minHitRate {
		t.Errorf("Cache hit rate: %.2f, expected >= %.2f", hitRate, minHitRate)
	}
	
	t.Logf("Cache performance: hit rate %.2f%%, average latency %v", 
		hitRate*100, stats.AverageLatency)
}

// TestWorkspaceScaling validates workspace scaling capabilities
func TestWorkspaceScaling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	testSuite := NewWorkspaceTestSuite()
	defer testSuite.Cleanup()

	const maxWorkspaces = 10
	workspaceNames := make([]string, maxWorkspaces)
	
	start := time.Now()
	
	// Create workspaces incrementally and measure performance
	for i := 1; i <= maxWorkspaces; i++ {
		workspaceName := fmt.Sprintf("scale-test-%d", i)
		workspaceNames[i-1] = workspaceName
		
		setupStart := time.Now()
		err := testSuite.SetupWorkspace(workspaceName, []string{"go"})
		if err != nil {
			t.Fatalf("Failed to setup workspace %d: %v", i, err)
		}
		
		err = testSuite.StartWorkspace(workspaceName)
		if err != nil {
			t.Fatalf("Failed to start workspace %d: %v", i, err)
		}
		setupDuration := time.Since(setupStart)
		
		// Performance should degrade linearly, not exponentially
		expectedMaxTime := time.Duration(i) * time.Second
		if setupDuration > expectedMaxTime {
			t.Errorf("Workspace %d setup took %v, expected < %v", i, setupDuration, expectedMaxTime)
		}
		
		// Test isolation
		err = testSuite.ValidateWorkspaceIsolation()
		if err != nil {
			t.Fatalf("Workspace isolation failed at %d workspaces: %v", i, err)
		}
	}
	
	totalTime := time.Since(start)
	avgTimePerWorkspace := totalTime / maxWorkspaces
	
	t.Logf("Workspace scaling: %d workspaces in %v (avg: %v per workspace)", 
		maxWorkspaces, totalTime, avgTimePerWorkspace)
}

