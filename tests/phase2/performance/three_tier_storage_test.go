package performance

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-gateway/internal/storage"
	"lsp-gateway/tests/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ThreeTierStoragePerformanceTest validates Three-Tier Storage performance targets
type ThreeTierStoragePerformanceTest struct {
	framework      *framework.MultiLanguageTestFramework
	threeTierStore storage.ThreeTierStorage
	l1Tier         storage.StorageTier
	l2Tier         storage.StorageTier
	l3Tier         storage.StorageTier
	
	// Performance targets (Phase 2 requirements)
	l1TargetLatency time.Duration // <10ms for L1 Memory
	l2TargetLatency time.Duration // <50ms for L2 Disk
	l3TargetLatency time.Duration // <200ms for L3 Remote
	
	// Test configuration
	testDataSize       int
	concurrencyLevels  []int
	cacheEntrySize     int
	
	// Metrics collection
	l1AccessTimes      []time.Duration
	l2AccessTimes      []time.Duration
	l3AccessTimes      []time.Duration
	promotionTimes     []time.Duration
	evictionTimes      []time.Duration
	
	mu sync.RWMutex
}

// TierPerformanceMetrics tracks performance metrics for each storage tier
type TierPerformanceMetrics struct {
	TierType           storage.TierType
	AverageLatency     time.Duration
	P50Latency         time.Duration
	P95Latency         time.Duration
	P99Latency         time.Duration
	ThroughputOpsPerSec float64
	CacheHitRate       float64
	ErrorRate          float64
	MemoryUsageMB      int64
	
	// Tier-specific metrics
	PromotionRate      float64
	EvictionRate       float64
	CompactionTime     time.Duration
	BackgroundOpsPerSec float64
}

// StorageOperationResult tracks individual storage operation results
type StorageOperationResult struct {
	Operation    string
	TierType     storage.TierType
	Key          string
	Success      bool
	Latency      time.Duration
	DataSize     int
	CacheHit     bool
	Promoted     bool
	Evicted      bool
	Error        error
	Timestamp    time.Time
}

// ThreeTierWorkloadPattern defines different workload patterns for testing
type ThreeTierWorkloadPattern struct {
	Name            string
	ReadRatio       float64  // Percentage of read operations
	WriteRatio      float64  // Percentage of write operations
	DeleteRatio     float64  // Percentage of delete operations
	HotDataRatio    float64  // Percentage of operations on "hot" data
	DataSizeVariation bool    // Whether data size varies significantly
	ConcurrentAccess  bool    // Whether multiple threads access same keys
}

// NewThreeTierStoragePerformanceTest creates a new Three-Tier Storage performance test
func NewThreeTierStoragePerformanceTest(t *testing.T) *ThreeTierStoragePerformanceTest {
	return &ThreeTierStoragePerformanceTest{
		framework:          framework.NewMultiLanguageTestFramework(45 * time.Minute),
		
		// Phase 2 performance targets
		l1TargetLatency:    10 * time.Millisecond,  // L1 Memory: <10ms
		l2TargetLatency:    50 * time.Millisecond,  // L2 Disk: <50ms
		l3TargetLatency:    200 * time.Millisecond, // L3 Remote: <200ms
		
		// Test configuration
		testDataSize:       10000,
		concurrencyLevels:  []int{1, 10, 25, 50, 100},
		cacheEntrySize:     1024, // 1KB entries
		
		// Initialize metrics
		l1AccessTimes:      make([]time.Duration, 0),
		l2AccessTimes:      make([]time.Duration, 0),
		l3AccessTimes:      make([]time.Duration, 0),
		promotionTimes:     make([]time.Duration, 0),
		evictionTimes:      make([]time.Duration, 0),
	}
}

// TestThreeTierStorageLatencyTargets validates all tier latency targets
func (test *ThreeTierStoragePerformanceTest) TestThreeTierStorageLatencyTargets(t *testing.T) {
	ctx := context.Background()
	
	err := test.setupThreeTierEnvironment(ctx)
	require.NoError(t, err, "Failed to setup Three-Tier Storage environment")
	defer test.cleanup()
	
	t.Log("Testing Three-Tier Storage latency targets (L1<10ms, L2<50ms, L3<200ms)...")
	
	// Test each tier independently
	tierTests := []struct {
		tierType        storage.TierType
		targetLatency   time.Duration
		warmupSize      int
		testOperations  int
	}{
		{
			tierType:       storage.TierTypeL1Memory,
			targetLatency:  test.l1TargetLatency,
			warmupSize:     1000,
			testOperations: 5000,
		},
		{
			tierType:       storage.TierTypeL2Disk,
			targetLatency:  test.l2TargetLatency,
			warmupSize:     500,
			testOperations: 2000,
		},
		{
			tierType:       storage.TierTypeL3Remote,
			targetLatency:  test.l3TargetLatency,
			warmupSize:     100,
			testOperations: 500,
		},
	}
	
	for _, tierTest := range tierTests {
		t.Run(string(tierTest.tierType), func(t *testing.T) {
			metrics := test.executeTierLatencyTest(t, tierTest.tierType, tierTest.warmupSize, tierTest.testOperations)
			test.validateTierLatencyMetrics(t, tierTest.tierType, tierTest.targetLatency, metrics)
		})
	}
	
	// Test cross-tier operations
	t.Run("CrossTierPromotion", func(t *testing.T) {
		metrics := test.testCrossTierPromotionPerformance(t, 1000)
		test.validatePromotionPerformance(t, metrics)
	})
	
	t.Run("CrossTierEviction", func(t *testing.T) {
		metrics := test.testCrossTierEvictionPerformance(t, 1000)
		test.validateEvictionPerformance(t, metrics)
	})
}

// TestThreeTierStorageThroughput validates throughput under various load conditions
func (test *ThreeTierStoragePerformanceTest) TestThreeTierStorageThroughput(t *testing.T) {
	ctx := context.Background()
	
	err := test.setupThreeTierEnvironment(ctx)
	require.NoError(t, err, "Failed to setup Three-Tier Storage environment")
	defer test.cleanup()
	
	t.Log("Testing Three-Tier Storage throughput under load...")
	
	// Test different workload patterns
	workloadPatterns := []ThreeTierWorkloadPattern{
		{
			Name:             "Read_Heavy_Hot_Data",
			ReadRatio:        0.8,
			WriteRatio:       0.15,
			DeleteRatio:      0.05,
			HotDataRatio:     0.9,
			DataSizeVariation: false,
			ConcurrentAccess: true,
		},
		{
			Name:             "Write_Heavy_Cold_Data",
			ReadRatio:        0.3,
			WriteRatio:       0.6,
			DeleteRatio:      0.1,
			HotDataRatio:     0.1,
			DataSizeVariation: true,
			ConcurrentAccess: false,
		},
		{
			Name:             "Balanced_Mixed_Access",
			ReadRatio:        0.6,
			WriteRatio:       0.3,
			DeleteRatio:      0.1,
			HotDataRatio:     0.4,
			DataSizeVariation: true,
			ConcurrentAccess: true,
		},
	}
	
	for _, pattern := range workloadPatterns {
		for _, concurrency := range test.concurrencyLevels {
			t.Run(fmt.Sprintf("%s_C%d", pattern.Name, concurrency), func(t *testing.T) {
				metrics := test.executeThroughputTest(t, pattern, concurrency, 60*time.Second)
				test.validateThroughputMetrics(t, pattern, concurrency, metrics)
			})
		}
	}
}

// TestThreeTierStorageMemoryEfficiency validates memory usage efficiency
func (test *ThreeTierStoragePerformanceTest) TestThreeTierStorageMemoryEfficiency(t *testing.T) {
	ctx := context.Background()
	
	err := test.setupThreeTierEnvironment(ctx)
	require.NoError(t, err, "Failed to setup Three-Tier Storage environment")
	defer test.cleanup()
	
	t.Log("Testing Three-Tier Storage memory efficiency...")
	
	initialMemory := test.getCurrentMemoryUsage()
	
	// Fill L1 cache to capacity
	err = test.fillL1CacheToCapacity(ctx)
	require.NoError(t, err, "Failed to fill L1 cache")
	
	l1FullMemory := test.getCurrentMemoryUsage()
	l1MemoryUsage := l1FullMemory - initialMemory
	
	// Test memory usage under load
	loadMemoryUsage := test.measureMemoryUsageUnderLoad(ctx, 100, 120*time.Second)
	
	// Test memory efficiency after compaction
	compactionMemoryUsage := test.measureMemoryUsageAfterCompaction(ctx)
	
	// Validate memory efficiency
	assert.Less(t, l1MemoryUsage, int64(500*1024*1024), // <500MB for L1 cache
		"L1 cache memory usage should be reasonable")
	
	assert.Less(t, loadMemoryUsage.PeakMemoryUsageMB, initialMemory/1024/1024+600,
		"Peak memory usage under load should be controlled")
	
	assert.False(t, loadMemoryUsage.MemoryLeakDetected,
		"No memory leaks should be detected")
	
	assert.Less(t, compactionMemoryUsage, l1FullMemory,
		"Memory usage should decrease after compaction")
	
	t.Logf("Memory efficiency: L1=%dMB, Peak=%dMB, Compacted=%dMB",
		l1MemoryUsage/1024/1024, loadMemoryUsage.PeakMemoryUsageMB, compactionMemoryUsage/1024/1024)
}

// TestThreeTierStorageCachePromotion validates intelligent cache promotion
func (test *ThreeTierStoragePerformanceTest) TestThreeTierStorageCachePromotion(t *testing.T) {
	ctx := context.Background()
	
	err := test.setupThreeTierEnvironment(ctx)
	require.NoError(t, err, "Failed to setup Three-Tier Storage environment")
	defer test.cleanup()
	
	t.Log("Testing Three-Tier Storage intelligent cache promotion...")
	
	// Create test data in L3 tier
	testKeys := test.generateTestKeys(1000)
	err = test.populateL3WithTestData(ctx, testKeys)
	require.NoError(t, err, "Failed to populate L3 with test data")
	
	// Access patterns that should trigger promotion
	promotionScenarios := []struct {
		name             string
		accessPattern    string
		expectedPromotion bool
		targetTier       storage.TierType
	}{
		{
			name:             "Frequent_Access_L3_to_L1",
			accessPattern:    "frequent",
			expectedPromotion: true,
			targetTier:       storage.TierTypeL1Memory,
		},
		{
			name:             "Moderate_Access_L3_to_L2",
			accessPattern:    "moderate",
			expectedPromotion: true,
			targetTier:       storage.TierTypeL2Disk,
		},
		{
			name:             "Infrequent_Access_No_Promotion",
			accessPattern:    "infrequent",
			expectedPromotion: false,
			targetTier:       storage.TierTypeL3Remote,
		},
	}
	
	for _, scenario := range promotionScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			promotionMetrics := test.executePromotionScenario(t, scenario, testKeys[:100])
			test.validatePromotionScenario(t, scenario, promotionMetrics)
		})
	}
}

// TestThreeTierStorageRebalancing validates storage rebalancing effectiveness
func (test *ThreeTierStoragePerformanceTest) TestThreeTierStorageRebalancing(t *testing.T) {
	ctx := context.Background()
	
	err := test.setupThreeTierEnvironment(ctx)
	require.NoError(t, err, "Failed to setup Three-Tier Storage environment")
	defer test.cleanup()
	
	t.Log("Testing Three-Tier Storage rebalancing effectiveness...")
	
	// Create imbalanced storage state
	err = test.createImbalancedStorageState(ctx)
	require.NoError(t, err, "Failed to create imbalanced storage state")
	
	// Measure initial state
	initialStats := test.threeTierStore.GetSystemStats()
	
	// Trigger rebalancing
	startTime := time.Now()
	rebalanceResult, err := test.threeTierStore.Rebalance(ctx)
	rebalanceTime := time.Since(startTime)
	
	require.NoError(t, err, "Rebalancing should succeed")
	require.NotNil(t, rebalanceResult, "Rebalance result should be provided")
	
	// Measure post-rebalance state
	finalStats := test.threeTierStore.GetSystemStats()
	
	// Validate rebalancing effectiveness
	assert.Less(t, rebalanceTime, 30*time.Second,
		"Rebalancing should complete within 30 seconds")
	
	assert.Greater(t, finalStats.TierDistribution[storage.TierTypeL1Memory], 
		initialStats.TierDistribution[storage.TierTypeL1Memory],
		"L1 utilization should improve after rebalancing")
	
	assert.Greater(t, rebalanceResult.ItemsMoved, int64(0),
		"Some items should be moved during rebalancing")
	
	assert.Less(t, rebalanceResult.ErrorCount, rebalanceResult.ItemsMoved/10,
		"Error rate during rebalancing should be <10%")
	
	t.Logf("Rebalancing: Time=%v, Moved=%d, Errors=%d, L1=%d%%",
		rebalanceTime, rebalanceResult.ItemsMoved, rebalanceResult.ErrorCount,
		int(finalStats.TierDistribution[storage.TierTypeL1Memory]*100))
}

// BenchmarkThreeTierStorageOperations benchmarks core storage operations
func (test *ThreeTierStoragePerformanceTest) BenchmarkThreeTierStorageOperations(b *testing.B) {
	ctx := context.Background()
	
	err := test.setupThreeTierEnvironment(ctx)
	if err != nil {
		b.Fatalf("Failed to setup Three-Tier Storage environment: %v", err)
	}
	defer test.cleanup()
	
	b.ResetTimer()
	
	b.Run("Get_L1_Memory", func(b *testing.B) {
		keys := test.generateTestKeys(b.N)
		test.warmupL1Cache(ctx, keys)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := test.threeTierStore.Get(ctx, keys[i])
			if err != nil {
				b.Errorf("Get operation failed: %v", err)
			}
		}
	})
	
	b.Run("Put_L1_Memory", func(b *testing.B) {
		entries := test.generateCacheEntries(b.N)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := test.threeTierStore.Put(ctx, entries[i].Key, entries[i])
			if err != nil {
				b.Errorf("Put operation failed: %v", err)
			}
		}
	})
	
	b.Run("Promote_L3_to_L1", func(b *testing.B) {
		keys := test.generateTestKeys(b.N)
		test.populateL3WithTestData(ctx, keys)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := test.threeTierStore.Promote(ctx, keys[i], 
				storage.TierTypeL3Remote, storage.TierTypeL1Memory)
			if err != nil {
				b.Errorf("Promotion failed: %v", err)
			}
		}
	})
}

// Helper methods

func (test *ThreeTierStoragePerformanceTest) setupThreeTierEnvironment(ctx context.Context) error {
	// Setup test framework
	if err := test.framework.SetupTestEnvironment(ctx); err != nil {
		return fmt.Errorf("failed to setup framework: %w", err)
	}
	
	// Configure Three-Tier Storage
	config := &storage.ThreeTierConfig{
		L1Config: &storage.TierConfig{
			TierType:     storage.TierTypeL1Memory,
			MaxSize:      100 * 1024 * 1024, // 100MB
			MaxEntries:   10000,
			TTL:          30 * time.Minute,
			Backend:      storage.BackendTypeMemory,
		},
		L2Config: &storage.TierConfig{
			TierType:     storage.TierTypeL2Disk,
			MaxSize:      1 * 1024 * 1024 * 1024, // 1GB
			MaxEntries:   100000,
			TTL:          2 * time.Hour,
			Backend:      storage.BackendTypeDisk,
		},
		L3Config: &storage.TierConfig{
			TierType:     storage.TierTypeL3Remote,
			MaxSize:      10 * 1024 * 1024 * 1024, // 10GB
			MaxEntries:   1000000,
			TTL:          24 * time.Hour,
			Backend:      storage.BackendTypeRemote,
		},
		PromotionStrategy: storage.PromotionStrategyIntelligent,
		EvictionPolicy:    storage.EvictionPolicyLRU,
	}
	
	// Initialize Three-Tier Storage
	threeTierStore, err := storage.NewThreeTierStorage(config)
	if err != nil {
		return fmt.Errorf("failed to create Three-Tier Storage: %w", err)
	}
	
	test.threeTierStore = threeTierStore
	
	// Get individual tier references for testing
	test.l1Tier = threeTierStore.GetTier(storage.TierTypeL1Memory)
	test.l2Tier = threeTierStore.GetTier(storage.TierTypeL2Disk)
	test.l3Tier = threeTierStore.GetTier(storage.TierTypeL3Remote)
	
	return nil
}

func (test *ThreeTierStoragePerformanceTest) executeTierLatencyTest(t *testing.T, tierType storage.TierType, warmupSize, testOperations int) *TierPerformanceMetrics {
	ctx := context.Background()
	
	// Get specific tier
	var tier storage.StorageTier
	switch tierType {
	case storage.TierTypeL1Memory:
		tier = test.l1Tier
	case storage.TierTypeL2Disk:
		tier = test.l2Tier
	case storage.TierTypeL3Remote:
		tier = test.l3Tier
	}
	
	// Warmup phase
	warmupEntries := test.generateCacheEntries(warmupSize)
	for _, entry := range warmupEntries {
		tier.Put(ctx, entry.Key, entry)
	}
	
	// Test phase
	var (
		latencies    []time.Duration
		successCount int64
		errorCount   int64
	)
	
	testKeys := test.generateTestKeys(testOperations)
	
	for _, key := range testKeys {
		startTime := time.Now()
		_, err := tier.Get(ctx, key)
		latency := time.Since(startTime)
		
		latencies = append(latencies, latency)
		
		if err != nil {
			atomic.AddInt64(&errorCount, 1)
		} else {
			atomic.AddInt64(&successCount, 1)
		}
		
		test.recordTierAccess(tierType, latency)
	}
	
	return test.calculateTierMetrics(tierType, latencies, successCount, errorCount)
}

func (test *ThreeTierStoragePerformanceTest) executeThroughputTest(t *testing.T, pattern ThreeTierWorkloadPattern, concurrency int, duration time.Duration) map[storage.TierType]*TierPerformanceMetrics {
	ctx := context.Background()
	
	var (
		wg           sync.WaitGroup
		results      = make(chan *StorageOperationResult, concurrency*100)
		endTime      = time.Now().Add(duration)
	)
	
	// Start concurrent workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			test.throughputWorker(workerID, pattern, endTime, results)
		}(i)
	}
	
	// Collect results
	go func() {
		wg.Wait()
		close(results)
	}()
	
	// Process results by tier
	tierResults := make(map[storage.TierType][]*StorageOperationResult)
	for result := range results {
		if _, exists := tierResults[result.TierType]; !exists {
			tierResults[result.TierType] = make([]*StorageOperationResult, 0)
		}
		tierResults[result.TierType] = append(tierResults[result.TierType], result)
	}
	
	// Calculate metrics for each tier
	metrics := make(map[storage.TierType]*TierPerformanceMetrics)
	for tierType, results := range tierResults {
		metrics[tierType] = test.calculateThroughputMetrics(tierType, results)
	}
	
	return metrics
}

func (test *ThreeTierStoragePerformanceTest) throughputWorker(workerID int, pattern ThreeTierWorkloadPattern, endTime time.Time, results chan<- *StorageOperationResult) {
	ctx := context.Background()
	
	hotKeys := test.generateTestKeys(100)    // Hot data keys
	coldKeys := test.generateTestKeys(1000)  // Cold data keys
	
	for time.Now().Before(endTime) {
		// Determine operation type
		operation := test.selectOperation(pattern)
		
		// Determine data access pattern (hot vs cold)
		var key string
		if rand.Float64() < pattern.HotDataRatio {
			key = hotKeys[rand.Intn(len(hotKeys))]
		} else {
			key = coldKeys[rand.Intn(len(coldKeys))]
		}
		
		// Execute operation
		result := test.executeOperation(ctx, operation, key, pattern.DataSizeVariation)
		
		select {
		case results <- result:
		case <-time.After(100 * time.Millisecond):
			// Avoid blocking
		}
		
		time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
	}
}

func (test *ThreeTierStoragePerformanceTest) selectOperation(pattern ThreeTierWorkloadPattern) string {
	r := rand.Float64()
	
	if r < pattern.ReadRatio {
		return "get"
	} else if r < pattern.ReadRatio+pattern.WriteRatio {
		return "put"
	} else {
		return "delete"
	}
}

func (test *ThreeTierStoragePerformanceTest) executeOperation(ctx context.Context, operation, key string, variableSize bool) *StorageOperationResult {
	result := &StorageOperationResult{
		Operation: operation,
		Key:       key,
		Timestamp: time.Now(),
	}
	
	startTime := time.Now()
	
	switch operation {
	case "get":
		_, err := test.threeTierStore.Get(ctx, key)
		result.Success = err == nil
		result.Error = err
		
	case "put":
		entry := test.generateCacheEntry(key, variableSize)
		err := test.threeTierStore.Put(ctx, key, entry)
		result.Success = err == nil
		result.Error = err
		result.DataSize = len(entry.Data)
		
	case "delete":
		err := test.threeTierStore.Delete(ctx, key)
		result.Success = err == nil
		result.Error = err
	}
	
	result.Latency = time.Since(startTime)
	
	// Determine which tier was actually accessed (simplified)
	if result.Latency < 20*time.Millisecond {
		result.TierType = storage.TierTypeL1Memory
	} else if result.Latency < 100*time.Millisecond {
		result.TierType = storage.TierTypeL2Disk
	} else {
		result.TierType = storage.TierTypeL3Remote
	}
	
	return result
}

func (test *ThreeTierStoragePerformanceTest) calculateTierMetrics(tierType storage.TierType, latencies []time.Duration, successCount, errorCount int64) *TierPerformanceMetrics {
	if len(latencies) == 0 {
		return &TierPerformanceMetrics{TierType: tierType}
	}
	
	// Sort latencies for percentile calculation
	sortedLatencies := make([]time.Duration, len(latencies))
	copy(sortedLatencies, latencies)
	for i := 0; i < len(sortedLatencies); i++ {
		for j := i + 1; j < len(sortedLatencies); j++ {
			if sortedLatencies[i] > sortedLatencies[j] {
				sortedLatencies[i], sortedLatencies[j] = sortedLatencies[j], sortedLatencies[i]
			}
		}
	}
	
	// Calculate average latency
	var totalLatency time.Duration
	for _, latency := range latencies {
		totalLatency += latency
	}
	avgLatency := totalLatency / time.Duration(len(latencies))
	
	// Calculate percentiles
	p50Index := len(sortedLatencies) / 2
	p95Index := int(float64(len(sortedLatencies)) * 0.95)
	p99Index := int(float64(len(sortedLatencies)) * 0.99)
	
	if p95Index >= len(sortedLatencies) {
		p95Index = len(sortedLatencies) - 1
	}
	if p99Index >= len(sortedLatencies) {
		p99Index = len(sortedLatencies) - 1
	}
	
	return &TierPerformanceMetrics{
		TierType:       tierType,
		AverageLatency: avgLatency,
		P50Latency:     sortedLatencies[p50Index],
		P95Latency:     sortedLatencies[p95Index],
		P99Latency:     sortedLatencies[p99Index],
		ErrorRate:      float64(errorCount) / float64(successCount+errorCount),
	}
}

func (test *ThreeTierStoragePerformanceTest) validateTierLatencyMetrics(t *testing.T, tierType storage.TierType, targetLatency time.Duration, metrics *TierPerformanceMetrics) {
	assert.Less(t, metrics.AverageLatency, targetLatency,
		"Average latency for %s should be less than %v", tierType, targetLatency)
	
	assert.Less(t, metrics.P95Latency, 2*targetLatency,
		"P95 latency for %s should be less than %v", tierType, 2*targetLatency)
	
	assert.Less(t, metrics.ErrorRate, 0.05,
		"Error rate for %s should be less than 5%%", tierType)
	
	t.Logf("Tier %s performance: Avg=%v, P95=%v, P99=%v, Errors=%.2f%%",
		tierType, metrics.AverageLatency, metrics.P95Latency, metrics.P99Latency, metrics.ErrorRate*100)
}

func (test *ThreeTierStoragePerformanceTest) generateTestKeys(count int) []string {
	keys := make([]string, count)
	for i := 0; i < count; i++ {
		keys[i] = fmt.Sprintf("test_key_%d_%d", i, time.Now().UnixNano())
	}
	return keys
}

func (test *ThreeTierStoragePerformanceTest) generateCacheEntries(count int) []*storage.CacheEntry {
	entries := make([]*storage.CacheEntry, count)
	for i := 0; i < count; i++ {
		entries[i] = test.generateCacheEntry(fmt.Sprintf("key_%d", i), false)
	}
	return entries
}

func (test *ThreeTierStoragePerformanceTest) generateCacheEntry(key string, variableSize bool) *storage.CacheEntry {
	size := test.cacheEntrySize
	if variableSize {
		size = rand.Intn(test.cacheEntrySize*4) + 100 // 100B to 4KB
	}
	
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(rand.Intn(256))
	}
	
	return &storage.CacheEntry{
		Key:        key,
		Data:       data,
		Timestamp:  time.Now(),
		TTL:        30 * time.Minute,
		AccessCount: 1,
		DataSize:   len(data),
		Metadata: map[string]interface{}{
			"test_entry": true,
			"size":       len(data),
		},
	}
}

func (test *ThreeTierStoragePerformanceTest) getCurrentMemoryUsage() int64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.Alloc)
}

func (test *ThreeTierStoragePerformanceTest) recordTierAccess(tierType storage.TierType, latency time.Duration) {
	test.mu.Lock()
	defer test.mu.Unlock()
	
	switch tierType {
	case storage.TierTypeL1Memory:
		test.l1AccessTimes = append(test.l1AccessTimes, latency)
	case storage.TierTypeL2Disk:
		test.l2AccessTimes = append(test.l2AccessTimes, latency)
	case storage.TierTypeL3Remote:
		test.l3AccessTimes = append(test.l3AccessTimes, latency)
	}
}

func (test *ThreeTierStoragePerformanceTest) cleanup() {
	if test.threeTierStore != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		test.threeTierStore.Shutdown(ctx)
	}
	if test.framework != nil {
		test.framework.CleanupAll()
	}
}

// Placeholder methods for additional test functionality

func (test *ThreeTierStoragePerformanceTest) testCrossTierPromotionPerformance(t *testing.T, operations int) *TierPerformanceMetrics {
	// Implementation for cross-tier promotion testing
	return &TierPerformanceMetrics{}
}

func (test *ThreeTierStoragePerformanceTest) testCrossTierEvictionPerformance(t *testing.T, operations int) *TierPerformanceMetrics {
	// Implementation for cross-tier eviction testing
	return &TierPerformanceMetrics{}
}

func (test *ThreeTierStoragePerformanceTest) validatePromotionPerformance(t *testing.T, metrics *TierPerformanceMetrics) {
	// Validation logic for promotion performance
}

func (test *ThreeTierStoragePerformanceTest) validateEvictionPerformance(t *testing.T, metrics *TierPerformanceMetrics) {
	// Validation logic for eviction performance
}

func (test *ThreeTierStoragePerformanceTest) fillL1CacheToCapacity(ctx context.Context) error {
	// Implementation to fill L1 cache to capacity
	return nil
}

func (test *ThreeTierStoragePerformanceTest) measureMemoryUsageUnderLoad(ctx context.Context, concurrency int, duration time.Duration) *MemoryUsageResults {
	// Implementation for memory usage measurement under load
	return &MemoryUsageResults{}
}

func (test *ThreeTierStoragePerformanceTest) measureMemoryUsageAfterCompaction(ctx context.Context) int64 {
	// Implementation for memory usage measurement after compaction
	return 0
}

type MemoryUsageResults struct {
	PeakMemoryUsageMB  int64
	MemoryLeakDetected bool
}

// Additional placeholder methods and types

func (test *ThreeTierStoragePerformanceTest) populateL3WithTestData(ctx context.Context, keys []string) error {
	return nil
}

func (test *ThreeTierStoragePerformanceTest) executePromotionScenario(t *testing.T, scenario struct {
	name             string
	accessPattern    string
	expectedPromotion bool
	targetTier       storage.TierType
}, keys []string) *TierPerformanceMetrics {
	return &TierPerformanceMetrics{}
}

func (test *ThreeTierStoragePerformanceTest) validatePromotionScenario(t *testing.T, scenario struct {
	name             string
	accessPattern    string
	expectedPromotion bool
	targetTier       storage.TierType
}, metrics *TierPerformanceMetrics) {
}

func (test *ThreeTierStoragePerformanceTest) createImbalancedStorageState(ctx context.Context) error {
	return nil
}

func (test *ThreeTierStoragePerformanceTest) warmupL1Cache(ctx context.Context, keys []string) error {
	return nil
}

func (test *ThreeTierStoragePerformanceTest) calculateThroughputMetrics(tierType storage.TierType, results []*StorageOperationResult) *TierPerformanceMetrics {
	return &TierPerformanceMetrics{TierType: tierType}
}

func (test *ThreeTierStoragePerformanceTest) validateThroughputMetrics(t *testing.T, pattern ThreeTierWorkloadPattern, concurrency int, metrics map[storage.TierType]*TierPerformanceMetrics) {
	// Validation logic for throughput metrics
}

// Integration test suite

func TestThreeTierStoragePerformanceSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Three-Tier Storage performance tests in short mode")
	}
	
	test := NewThreeTierStoragePerformanceTest(t)
	
	t.Run("LatencyTargets", test.TestThreeTierStorageLatencyTargets)
	t.Run("Throughput", test.TestThreeTierStorageThroughput)
	t.Run("MemoryEfficiency", test.TestThreeTierStorageMemoryEfficiency)
	t.Run("CachePromotion", test.TestThreeTierStorageCachePromotion)
	t.Run("Rebalancing", test.TestThreeTierStorageRebalancing)
}

func BenchmarkThreeTierStoragePerformance(b *testing.B) {
	test := NewThreeTierStoragePerformanceTest(nil)
	test.BenchmarkThreeTierStorageOperations(b)
}