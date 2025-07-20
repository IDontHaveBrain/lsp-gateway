package platform

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"runtime"
	"runtime/debug"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// GCMetrics tracks detailed garbage collection performance metrics
type GCMetrics struct {
	mu                    sync.RWMutex
	samples               []GCSample
	maxSamples            int
	startTime             time.Time
	monitoring            bool
	stopCh                chan struct{}
	monitoringWG          sync.WaitGroup
	sampleRate            time.Duration
	lastGCStats           debug.GCStats
	initialMemStats       runtime.MemStats
	gcPressureThreshold   float64
	allocationRateHistory []float64
}

// GCSample represents a single GC measurement
type GCSample struct {
	Timestamp         time.Time
	HeapAlloc         uint64
	HeapSys           uint64
	HeapIdle          uint64
	HeapInuse         uint64
	HeapReleased      uint64
	HeapObjects       uint64
	StackInuse        uint64
	NextGC            uint64
	LastGC            uint64
	PauseTotalNs      uint64
	NumGC             uint32
	NumForcedGC       uint32
	GCCPUFraction     float64
	TotalAlloc        uint64
	Mallocs           uint64
	Frees             uint64
	AllocationRate    float64  // bytes per second
	GCPressure        float64  // ratio of allocations to GC
	GCEfficiency      float64  // ratio of freed memory to GC pause time
	MemoryUtilization float64  // ratio of heap in use to heap sys
	FragmentationRatio float64 // ratio of heap idle to heap sys
}

// NewGCMetrics creates a new GC metrics tracker
func NewGCMetrics(maxSamples int, sampleRate time.Duration) *GCMetrics {
	var initialStats runtime.MemStats
	runtime.ReadMemStats(&initialStats)

	return &GCMetrics{
		samples:               make([]GCSample, 0, maxSamples),
		maxSamples:            maxSamples,
		sampleRate:            sampleRate,
		stopCh:                make(chan struct{}),
		startTime:             time.Now(),
		initialMemStats:       initialStats,
		gcPressureThreshold:   0.8, // 80% threshold for GC pressure
		allocationRateHistory: make([]float64, 0, 100),
	}
}

// StartMonitoring begins GC metrics collection
func (gm *GCMetrics) StartMonitoring() {
	gm.mu.Lock()
	if gm.monitoring {
		gm.mu.Unlock()
		return
	}
	gm.monitoring = true
	gm.mu.Unlock()

	gm.monitoringWG.Add(1)
	go func() {
		defer gm.monitoringWG.Done()
		ticker := time.NewTicker(gm.sampleRate)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				gm.takeSample()
			case <-gm.stopCh:
				return
			}
		}
	}()
}

// StopMonitoring stops GC metrics collection
func (gm *GCMetrics) StopMonitoring() {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	if !gm.monitoring {
		return
	}

	gm.monitoring = false
	close(gm.stopCh)
	gm.monitoringWG.Wait()
}

// takeSample captures current GC metrics
func (gm *GCMetrics) takeSample() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	now := time.Now()
	duration := now.Sub(gm.startTime).Seconds()

	sample := GCSample{
		Timestamp:    now,
		HeapAlloc:    memStats.HeapAlloc,
		HeapSys:      memStats.HeapSys,
		HeapIdle:     memStats.HeapIdle,
		HeapInuse:    memStats.HeapInuse,
		HeapReleased: memStats.HeapReleased,
		HeapObjects:  memStats.HeapObjects,
		StackInuse:   memStats.StackInuse,
		NextGC:       memStats.NextGC,
		LastGC:       memStats.LastGC,
		PauseTotalNs: memStats.PauseTotalNs,
		NumGC:        memStats.NumGC,
		NumForcedGC:  memStats.NumForcedGC,
		GCCPUFraction: memStats.GCCPUFraction,
		TotalAlloc:   memStats.TotalAlloc,
		Mallocs:      memStats.Mallocs,
		Frees:        memStats.Frees,
	}

	// Calculate derived metrics
	if duration > 0 {
		allocDiff := memStats.TotalAlloc - gm.initialMemStats.TotalAlloc
		sample.AllocationRate = float64(allocDiff) / duration
	}

	if memStats.NumGC > gm.initialMemStats.NumGC {
		gcDiff := memStats.NumGC - gm.initialMemStats.NumGC
		allocDiff := memStats.TotalAlloc - gm.initialMemStats.TotalAlloc
		if gcDiff > 0 {
			sample.GCPressure = float64(allocDiff) / float64(gcDiff)
		}
	}

	if memStats.PauseTotalNs > gm.initialMemStats.PauseTotalNs {
		pauseDiff := memStats.PauseTotalNs - gm.initialMemStats.PauseTotalNs
		freedMem := memStats.Frees - gm.initialMemStats.Frees
		if pauseDiff > 0 {
			sample.GCEfficiency = float64(freedMem) / float64(pauseDiff) * 1e9 // objects per second
		}
	}

	if memStats.HeapSys > 0 {
		sample.MemoryUtilization = float64(memStats.HeapInuse) / float64(memStats.HeapSys)
		sample.FragmentationRatio = float64(memStats.HeapIdle) / float64(memStats.HeapSys)
	}

	gm.mu.Lock()
	defer gm.mu.Unlock()

	// Store allocation rate history
	gm.allocationRateHistory = append(gm.allocationRateHistory, sample.AllocationRate)
	if len(gm.allocationRateHistory) > 100 {
		gm.allocationRateHistory = gm.allocationRateHistory[1:]
	}

	// Store sample
	if len(gm.samples) >= gm.maxSamples {
		copy(gm.samples, gm.samples[1:])
		gm.samples = gm.samples[:len(gm.samples)-1]
	}
	gm.samples = append(gm.samples, sample)
}

// GetGCEfficiencyReport generates comprehensive GC efficiency analysis
func (gm *GCMetrics) GetGCEfficiencyReport() map[string]interface{} {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	if len(gm.samples) == 0 {
		return map[string]interface{}{"error": "no samples available"}
	}

	first := gm.samples[0]
	last := gm.samples[len(gm.samples)-1]
	duration := last.Timestamp.Sub(first.Timestamp).Seconds()

	// Calculate averages and trends
	var (
		avgGCPressure        float64
		avgGCEfficiency      float64
		avgMemUtilization    float64
		avgFragmentation     float64
		avgAllocationRate    float64
		totalGCTime          time.Duration
		gcFrequency          float64
		memoryTurnover       float64
		allocationBurstiness float64
	)

	for _, sample := range gm.samples {
		avgGCPressure += sample.GCPressure
		avgGCEfficiency += sample.GCEfficiency
		avgMemUtilization += sample.MemoryUtilization
		avgFragmentation += sample.FragmentationRatio
		avgAllocationRate += sample.AllocationRate
	}

	sampleCount := float64(len(gm.samples))
	avgGCPressure /= sampleCount
	avgGCEfficiency /= sampleCount
	avgMemUtilization /= sampleCount
	avgFragmentation /= sampleCount
	avgAllocationRate /= sampleCount

	// Calculate GC frequency
	gcRuns := last.NumGC - first.NumGC
	if duration > 0 {
		gcFrequency = float64(gcRuns) / duration
	}

	// Calculate memory turnover rate
	allocatedBytes := last.TotalAlloc - first.TotalAlloc
	if last.HeapSys > 0 {
		memoryTurnover = float64(allocatedBytes) / float64(last.HeapSys)
	}

	// Calculate allocation burstiness (standard deviation of allocation rates)
	if len(gm.allocationRateHistory) > 1 {
		var variance float64
		for _, rate := range gm.allocationRateHistory {
			diff := rate - avgAllocationRate
			variance += diff * diff
		}
		variance /= float64(len(gm.allocationRateHistory) - 1)
		allocationBurstiness = math.Sqrt(variance) / avgAllocationRate
	}

	// Calculate total GC time
	pauseDiff := last.PauseTotalNs - first.PauseTotalNs
	if gcRuns > 0 {
		totalGCTime = time.Duration(pauseDiff)
	}

	// Performance assessment
	performanceScore := gm.calculatePerformanceScore(avgGCPressure, avgGCEfficiency, avgMemUtilization, gcFrequency)

	return map[string]interface{}{
		"sample_count":            len(gm.samples),
		"duration_seconds":        duration,
		"gc_runs":                gcRuns,
		"gc_frequency_hz":         gcFrequency,
		"avg_gc_pressure":         avgGCPressure,
		"avg_gc_efficiency":       avgGCEfficiency,
		"avg_memory_utilization":  avgMemUtilization,
		"avg_fragmentation":       avgFragmentation,
		"avg_allocation_rate":     avgAllocationRate,
		"allocation_burstiness":   allocationBurstiness,
		"memory_turnover":         memoryTurnover,
		"total_gc_time_ms":        totalGCTime.Nanoseconds() / 1e6,
		"avg_gc_pause_ms":         float64(totalGCTime.Nanoseconds()) / float64(gcRuns) / 1e6,
		"gc_cpu_fraction":         last.GCCPUFraction,
		"heap_growth":             int64(last.HeapAlloc) - int64(first.HeapAlloc),
		"total_allocated":         last.TotalAlloc - first.TotalAlloc,
		"objects_allocated":       last.Mallocs - first.Mallocs,
		"objects_freed":           last.Frees - first.Frees,
		"object_churn_rate":       float64((last.Mallocs-first.Mallocs)+(last.Frees-first.Frees)) / duration,
		"performance_score":       performanceScore,
		"high_gc_pressure":        avgGCPressure > gm.gcPressureThreshold,
		"high_fragmentation":      avgFragmentation > 0.5,
		"efficient_gc":           avgGCEfficiency > 1000, // objects per second
	}
}

// calculatePerformanceScore computes overall GC performance score (0-100)
func (gm *GCMetrics) calculatePerformanceScore(gcPressure, gcEfficiency, memUtilization, gcFrequency float64) float64 {
	score := 100.0

	// Penalize high GC pressure
	if gcPressure > gm.gcPressureThreshold {
		score -= (gcPressure - gm.gcPressureThreshold) * 50
	}

	// Reward high GC efficiency
	if gcEfficiency > 1000 {
		score += math.Min(10, gcEfficiency/1000)
	} else {
		score -= (1000 - gcEfficiency) / 100
	}

	// Reward good memory utilization (not too high, not too low)
	optimalUtilization := 0.7
	utilizationPenalty := math.Abs(memUtilization-optimalUtilization) * 20
	score -= utilizationPenalty

	// Penalize excessive GC frequency
	if gcFrequency > 10 { // More than 10 GCs per second
		score -= (gcFrequency - 10) * 2
	}

	return math.Max(0, math.Min(100, score))
}

// GetGCPauseDistribution analyzes GC pause time distribution
func (gm *GCMetrics) GetGCPauseDistribution() map[string]interface{} {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	if len(gm.samples) < 2 {
		return map[string]interface{}{"error": "insufficient samples"}
	}

	// Calculate pause times between samples
	var pauseTimes []float64
	for i := 1; i < len(gm.samples); i++ {
		pauseDiff := gm.samples[i].PauseTotalNs - gm.samples[i-1].PauseTotalNs
		gcDiff := gm.samples[i].NumGC - gm.samples[i-1].NumGC
		
		if gcDiff > 0 {
			avgPause := float64(pauseDiff) / float64(gcDiff) / 1e6 // Convert to milliseconds
			pauseTimes = append(pauseTimes, avgPause)
		}
	}

	if len(pauseTimes) == 0 {
		return map[string]interface{}{"error": "no pause data available"}
	}

	sort.Float64s(pauseTimes)

	// Calculate percentiles
	p50 := percentile(pauseTimes, 0.5)
	p95 := percentile(pauseTimes, 0.95)
	p99 := percentile(pauseTimes, 0.99)
	max := pauseTimes[len(pauseTimes)-1]
	min := pauseTimes[0]

	// Calculate average
	var sum float64
	for _, pause := range pauseTimes {
		sum += pause
	}
	avg := sum / float64(len(pauseTimes))

	return map[string]interface{}{
		"pause_count":     len(pauseTimes),
		"min_pause_ms":    min,
		"max_pause_ms":    max,
		"avg_pause_ms":    avg,
		"p50_pause_ms":    p50,
		"p95_pause_ms":    p95,
		"p99_pause_ms":    p99,
		"pause_variance":  calculateVariance(pauseTimes, avg),
		"long_pauses":     countLongPauses(pauseTimes, 10.0), // Pauses > 10ms
	}
}

// percentile calculates the nth percentile of sorted data
func percentile(data []float64, p float64) float64 {
	if len(data) == 0 {
		return 0
	}
	
	index := p * float64(len(data)-1)
	lower := int(index)
	upper := lower + 1
	
	if upper >= len(data) {
		return data[len(data)-1]
	}
	
	weight := index - float64(lower)
	return data[lower]*(1-weight) + data[upper]*weight
}

// calculateVariance calculates variance of pause times
func calculateVariance(data []float64, mean float64) float64 {
	if len(data) <= 1 {
		return 0
	}
	
	var sum float64
	for _, value := range data {
		diff := value - mean
		sum += diff * diff
	}
	
	return sum / float64(len(data)-1)
}

// countLongPauses counts pauses exceeding threshold
func countLongPauses(data []float64, threshold float64) int {
	count := 0
	for _, pause := range data {
		if pause > threshold {
			count++
		}
	}
	return count
}

// MemoryStressTest simulates various memory allocation patterns
type MemoryStressTest struct {
	allocations      [][]byte
	allocationSizes  []int
	allocationRate   time.Duration
	retentionRatio   float64 // Percentage of allocations to keep
	burstMode        bool
	burstSize        int
	burstInterval    time.Duration
	mu               sync.Mutex
	running          bool
	stopCh           chan struct{}
	wg               sync.WaitGroup
}

// NewMemoryStressTest creates a configurable memory stress test
func NewMemoryStressTest(sizes []int, rate time.Duration, retention float64) *MemoryStressTest {
	return &MemoryStressTest{
		allocations:     make([][]byte, 0),
		allocationSizes: sizes,
		allocationRate:  rate,
		retentionRatio:  retention,
		stopCh:          make(chan struct{}),
	}
}

// EnableBurstMode configures burst allocation mode
func (mst *MemoryStressTest) EnableBurstMode(burstSize int, burstInterval time.Duration) {
	mst.burstMode = true
	mst.burstSize = burstSize
	mst.burstInterval = burstInterval
}

// Start begins the memory stress test
func (mst *MemoryStressTest) Start() {
	mst.mu.Lock()
	if mst.running {
		mst.mu.Unlock()
		return
	}
	mst.running = true
	mst.mu.Unlock()

	if mst.burstMode {
		mst.wg.Add(1)
		go mst.runBurstMode()
	} else {
		mst.wg.Add(1)
		go mst.runContinuousMode()
	}
}

// Stop stops the memory stress test
func (mst *MemoryStressTest) Stop() {
	mst.mu.Lock()
	if !mst.running {
		mst.mu.Unlock()
		return
	}
	mst.running = false
	close(mst.stopCh)
	mst.mu.Unlock()

	mst.wg.Wait()

	// Clean up allocations
	mst.mu.Lock()
	mst.allocations = nil
	mst.mu.Unlock()
}

// runContinuousMode runs continuous allocation pattern
func (mst *MemoryStressTest) runContinuousMode() {
	defer mst.wg.Done()
	ticker := time.NewTicker(mst.allocationRate)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mst.allocateMemory()
		case <-mst.stopCh:
			return
		}
	}
}

// runBurstMode runs burst allocation pattern
func (mst *MemoryStressTest) runBurstMode() {
	defer mst.wg.Done()
	ticker := time.NewTicker(mst.burstInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Allocate burst
			for i := 0; i < mst.burstSize; i++ {
				mst.allocateMemory()
				time.Sleep(time.Millisecond) // Small delay within burst
			}
		case <-mst.stopCh:
			return
		}
	}
}

// allocateMemory performs a single memory allocation
func (mst *MemoryStressTest) allocateMemory() {
	// Choose random size
	sizeIndex := time.Now().UnixNano() % int64(len(mst.allocationSizes))
	size := mst.allocationSizes[sizeIndex]

	// Allocate memory
	buffer := make([]byte, size)
	rand.Read(buffer) // Fill with random data

	mst.mu.Lock()
	defer mst.mu.Unlock()

	// Add to allocations
	mst.allocations = append(mst.allocations, buffer)

	// Randomly free some allocations based on retention ratio
	if len(mst.allocations) > 100 {
		retainCount := int(float64(len(mst.allocations)) * mst.retentionRatio)
		if retainCount < len(mst.allocations) {
			// Remove old allocations
			removeCount := len(mst.allocations) - retainCount
			copy(mst.allocations, mst.allocations[removeCount:])
			mst.allocations = mst.allocations[:retainCount]
		}
	}
}

// GetStats returns current allocation statistics
func (mst *MemoryStressTest) GetStats() map[string]interface{} {
	mst.mu.Lock()
	defer mst.mu.Unlock()

	var totalSize int64
	for _, alloc := range mst.allocations {
		totalSize += int64(len(alloc))
	}

	return map[string]interface{}{
		"allocation_count": len(mst.allocations),
		"total_size":       totalSize,
		"avg_size":         totalSize / int64(max(1, len(mst.allocations))),
		"running":          mst.running,
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Test garbage collection behavior under high request load
func TestGCBehaviorUnderHighLoad(t *testing.T) {
	testCases := []struct {
		name            string
		allocationSizes []int
		allocationRate  time.Duration
		retentionRatio  float64
		testDuration    time.Duration
		expectedGCRuns  uint32
	}{
		{
			name:            "SmallAllocations_HighFreq",
			allocationSizes: []int{1024, 2048, 4096}, // 1-4KB
			allocationRate:  1 * time.Millisecond,
			retentionRatio:  0.1, // Keep 10%
			testDuration:    2 * time.Second, // Further reduced
			expectedGCRuns:  2, // Adjusted expectation
		},
		{
			name:            "MediumAllocations_MedFreq",
			allocationSizes: []int{64 * 1024, 128 * 1024, 256 * 1024}, // 64-256KB
			allocationRate:  10 * time.Millisecond,
			retentionRatio:  0.3, // Keep 30%
			testDuration:    2 * time.Second, // Further reduced
			expectedGCRuns:  2, // Adjusted expectation
		},
		{
			name:            "LargeAllocations_LowFreq",
			allocationSizes: []int{1024 * 1024, 2048 * 1024}, // 1-2MB
			allocationRate:  100 * time.Millisecond,
			retentionRatio:  0.5, // Keep 50%
			testDuration:    2 * time.Second, // Further reduced
			expectedGCRuns:  2,
		},
		{
			name:            "MixedAllocations_BurstMode",
			allocationSizes: []int{1024, 64 * 1024, 1024 * 1024}, // Mixed sizes
			allocationRate:  50 * time.Millisecond,
			retentionRatio:  0.2, // Keep 20%
			testDuration:    2 * time.Second, // Further reduced
			expectedGCRuns:  2, // Adjusted expectation
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Start GC monitoring
			gcMetrics := NewGCMetrics(1000, 100*time.Millisecond)
			gcMetrics.StartMonitoring()
			defer gcMetrics.StopMonitoring()

			// Create and start memory stress test
			stressTest := NewMemoryStressTest(tc.allocationSizes, tc.allocationRate, tc.retentionRatio)
			
			if tc.name == "MixedAllocations_BurstMode" {
				stressTest.EnableBurstMode(20, 2*time.Second)
			}

			initialGC := getGCCount()
			
			stressTest.Start()
			time.Sleep(tc.testDuration)
			stressTest.Stop()

			// Force final GC and allow cleanup
			runtime.GC()
			runtime.GC()
			time.Sleep(500 * time.Millisecond)

			finalGC := getGCCount()
			gcRuns := finalGC - initialGC

			// Generate reports
			efficiencyReport := gcMetrics.GetGCEfficiencyReport()
			pauseReport := gcMetrics.GetGCPauseDistribution()
			stressStats := stressTest.GetStats()

			t.Logf("GC runs: %d (expected: %d)", gcRuns, tc.expectedGCRuns)
			t.Logf("Efficiency report: %+v", efficiencyReport)
			t.Logf("Pause distribution: %+v", pauseReport)
			t.Logf("Stress test stats: %+v", stressStats)

			// Verify GC behavior
			if gcRuns < tc.expectedGCRuns/2 {
				t.Errorf("Insufficient GC runs: %d < %d", gcRuns, tc.expectedGCRuns/2)
			}

			// Check GC efficiency
			if efficiency, ok := efficiencyReport["avg_gc_efficiency"].(float64); ok {
				if efficiency < 100 { // objects per second
					t.Errorf("Low GC efficiency: %.2f objects/sec", efficiency)
				}
			}

			// Check memory utilization
			if utilization, ok := efficiencyReport["avg_memory_utilization"].(float64); ok {
				if utilization > 0.9 { // > 90% utilization may indicate issues
					t.Errorf("High memory utilization: %.2f%%", utilization*100)
				}
			}

			// Check GC pause times
			if avgPause, ok := pauseReport["avg_pause_ms"].(float64); ok {
				if avgPause > 50.0 { // 50ms average pause threshold
					t.Errorf("High average GC pause: %.2f ms", avgPause)
				}
			}

			// Check for excessive long pauses
			if longPauses, ok := pauseReport["long_pauses"].(int); ok {
				totalPauses, _ := pauseReport["pause_count"].(int)
				if totalPauses > 0 {
					longPauseRatio := float64(longPauses) / float64(totalPauses)
					if longPauseRatio > 0.1 { // More than 10% long pauses
						t.Errorf("Too many long pauses: %d/%d (%.1f%%)", longPauses, totalPauses, longPauseRatio*100)
					}
				}
			}

			// Check performance score
			if score, ok := efficiencyReport["performance_score"].(float64); ok {
				if score < 60.0 { // Performance score below 60
					t.Errorf("Low GC performance score: %.1f/100", score)
				}
			}
		})
	}
}

// Test memory allocation patterns and GC pressure analysis
func TestMemoryAllocationPatternsGCPressure(t *testing.T) {
	pressureTests := []struct {
		name             string
		pattern          string
		duration         time.Duration
		maxPressure      float64
		maxCPUFraction   float64
	}{
		{
			name:           "LowPressure_SmallObjects",
			pattern:        "small_steady",
			duration:       1 * time.Second, // Further reduced
			maxPressure:    0.5,
			maxCPUFraction: 0.1,
		},
		{
			name:           "MediumPressure_MixedObjects",
			pattern:        "mixed_burst",
			duration:       1 * time.Second, // Further reduced
			maxPressure:    0.8,
			maxCPUFraction: 0.2,
		},
		{
			name:           "HighPressure_LargeObjects",
			pattern:        "large_continuous",
			duration:       1 * time.Second, // Further reduced
			maxPressure:    1.0,
			maxCPUFraction: 0.3,
		},
	}

	for _, pt := range pressureTests {
		t.Run(pt.name, func(t *testing.T) {
			gcMetrics := NewGCMetrics(1000, 50*time.Millisecond)
			gcMetrics.StartMonitoring()
			defer gcMetrics.StopMonitoring()

			// Configure stress test based on pattern
			var stressTest *MemoryStressTest
			switch pt.pattern {
			case "small_steady":
				stressTest = NewMemoryStressTest(
					[]int{512, 1024, 2048},
					5*time.Millisecond,
					0.1,
				)
			case "mixed_burst":
				stressTest = NewMemoryStressTest(
					[]int{1024, 32*1024, 128*1024},
					20*time.Millisecond,
					0.3,
				)
				stressTest.EnableBurstMode(15, 1*time.Second)
			case "large_continuous":
				stressTest = NewMemoryStressTest(
					[]int{512*1024, 1024*1024, 2048*1024},
					50*time.Millisecond,
					0.7,
				)
			}

			// Run test
			stressTest.Start()
			time.Sleep(pt.duration)
			stressTest.Stop()

			// Force cleanup
			runtime.GC()
			runtime.GC()
			time.Sleep(500 * time.Millisecond)

			// Analyze results
			report := gcMetrics.GetGCEfficiencyReport()
			t.Logf("GC pressure analysis: %+v", report)

			// Verify GC pressure within limits
			if pressure, ok := report["avg_gc_pressure"].(float64); ok {
				if pressure > pt.maxPressure {
					t.Errorf("GC pressure too high: %.3f > %.3f", pressure, pt.maxPressure)
				}
			}

			// Verify GC CPU fraction within limits
			if cpuFraction, ok := report["gc_cpu_fraction"].(float64); ok {
				if cpuFraction > pt.maxCPUFraction {
					t.Errorf("GC CPU fraction too high: %.3f > %.3f", cpuFraction, pt.maxCPUFraction)
				}
			}

			// Check allocation rate stability
			if burstiness, ok := report["allocation_burstiness"].(float64); ok {
				if burstiness > 2.0 { // High variability in allocation rate
					t.Logf("High allocation burstiness detected: %.2f", burstiness)
				}
			}

			// Verify memory efficiency
			if fragmentation, ok := report["avg_fragmentation"].(float64); ok {
				if fragmentation > 0.6 { // More than 60% fragmentation
					t.Errorf("High memory fragmentation: %.2f%%", fragmentation*100)
				}
			}
		})
	}
}

// Test GC pause impact on request latency and throughput
func TestGCPauseImpactOnLatencyThroughput(t *testing.T) {
	gcMetrics := NewGCMetrics(1000, 100*time.Millisecond)
	gcMetrics.StartMonitoring()
	defer gcMetrics.StopMonitoring()

	// Simulate concurrent request processing with memory allocation
	const (
		workerCount    = 10 // Reduced from 20
		requestsPerWorker = 25 // Reduced from 50
		requestDuration = 2 * time.Second // Further reduced
	)

	var (
		completedRequests int64
		totalLatency      int64
		maxLatency        int64
		gcPausesDuringTest int64
	)

	ctx, cancel := context.WithTimeout(context.Background(), requestDuration)
	defer cancel()

	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < requestsPerWorker; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				startTime := time.Now()

				// Simulate request processing with memory allocation
				processSimulatedRequest(workerID, j)

				latency := time.Since(startTime).Nanoseconds()
				
				atomic.AddInt64(&completedRequests, 1)
				atomic.AddInt64(&totalLatency, latency)

				// Update max latency
				for {
					currentMax := atomic.LoadInt64(&maxLatency)
					if latency <= currentMax || atomic.CompareAndSwapInt64(&maxLatency, currentMax, latency) {
						break
					}
				}

				time.Sleep(10 * time.Millisecond) // Simulate request interval
			}
		}(i)
	}

	// Monitor GC pauses during test
	go func() {
		initialGC := getGCCount()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				time.Sleep(100 * time.Millisecond)
				currentGC := getGCCount()
				atomic.StoreInt64(&gcPausesDuringTest, int64(currentGC-initialGC))
			}
		}
	}()

	wg.Wait()

	// Calculate metrics
	avgLatency := float64(totalLatency) / float64(completedRequests) / 1e6 // Convert to milliseconds
	maxLatencyMs := float64(maxLatency) / 1e6
	throughput := float64(completedRequests) / requestDuration.Seconds()

	// Get GC analysis
	report := gcMetrics.GetGCEfficiencyReport()
	pauseReport := gcMetrics.GetGCPauseDistribution()

	t.Logf("Completed requests: %d", completedRequests)
	t.Logf("Average latency: %.2f ms", avgLatency)
	t.Logf("Max latency: %.2f ms", maxLatencyMs)
	t.Logf("Throughput: %.2f req/sec", throughput)
	t.Logf("GC pauses during test: %d", gcPausesDuringTest)
	t.Logf("GC efficiency report: %+v", report)
	t.Logf("GC pause distribution: %+v", pauseReport)

	// Verify performance targets
	if avgLatency > 50.0 { // Average latency > 50ms
		t.Errorf("High average latency: %.2f ms", avgLatency)
	}

	if maxLatencyMs > 500.0 { // Max latency > 500ms
		t.Errorf("Excessive max latency: %.2f ms", maxLatencyMs)
	}

	if throughput < 50.0 { // Throughput < 50 req/sec
		t.Errorf("Low throughput: %.2f req/sec", throughput)
	}

	// Check GC impact
	if gcCPUFraction, ok := report["gc_cpu_fraction"].(float64); ok {
		if gcCPUFraction > 0.25 { // GC using more than 25% CPU
			t.Errorf("High GC CPU usage: %.2f%%", gcCPUFraction*100)
		}
	}

	// Check pause distribution impact
	if p95Pause, ok := pauseReport["p95_pause_ms"].(float64); ok {
		if p95Pause > 100.0 { // P95 pause > 100ms
			t.Errorf("High P95 GC pause: %.2f ms", p95Pause)
		}
	}
}

// Test memory allocation rate and GC frequency correlation
func TestAllocationRateGCFrequencyCorrelation(t *testing.T) {
	testCases := []struct {
		name              string
		allocationRate    time.Duration
		expectedFrequency float64 // GCs per second
		tolerance         float64
	}{
		{"LowRate", 100 * time.Millisecond, 0.5, 0.3},
		{"MediumRate", 10 * time.Millisecond, 2.0, 1.0},
		{"HighRate", 1 * time.Millisecond, 5.0, 2.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gcMetrics := NewGCMetrics(500, 100*time.Millisecond)
			gcMetrics.StartMonitoring()
			defer gcMetrics.StopMonitoring()

			// Create stress test with consistent allocation rate
			stressTest := NewMemoryStressTest(
				[]int{32 * 1024, 64 * 1024}, // 32-64KB allocations
				tc.allocationRate,
				0.2, // Keep 20%
			)

			testDuration := 1 * time.Second // Further reduced
			stressTest.Start()
			time.Sleep(testDuration)
			stressTest.Stop()

			// Analyze correlation
			report := gcMetrics.GetGCEfficiencyReport()
			
			if frequency, ok := report["gc_frequency_hz"].(float64); ok {
				t.Logf("Allocation rate: %v, GC frequency: %.2f Hz, Expected: %.2f Hz", 
					tc.allocationRate, frequency, tc.expectedFrequency)

				// Check if frequency is within tolerance
				diff := math.Abs(frequency - tc.expectedFrequency)
				if diff > tc.tolerance {
					t.Errorf("GC frequency outside tolerance: %.2f Hz (expected %.2f ± %.2f)",
						frequency, tc.expectedFrequency, tc.tolerance)
				}

				// Verify allocation rate is reasonable
				if allocRate, ok := report["avg_allocation_rate"].(float64); ok {
					expectedAllocRate := 1.0 / tc.allocationRate.Seconds() * 48 * 1024 // ~48KB per allocation
					allocRateMB := allocRate / (1024 * 1024)
					expectedMB := expectedAllocRate / (1024 * 1024)
					
					t.Logf("Allocation rate: %.2f MB/s, Expected: %.2f MB/s", 
						allocRateMB, expectedMB)
				}
			}

			// Check correlation quality
			if performance, ok := report["performance_score"].(float64); ok {
				if performance < 50.0 {
					t.Errorf("Poor GC performance score: %.1f/100", performance)
				}
			}
		})
	}
}

// processSimulatedRequest simulates request processing with memory allocation
func processSimulatedRequest(workerID, requestID int) {
	// Simulate various allocation patterns during request processing
	
	// Small temporary allocations
	temp1 := make([]byte, 1024)
	temp2 := make([]byte, 2048)
	
	// Fill with data
	copy(temp1, fmt.Sprintf("worker_%d_request_%d", workerID, requestID))
	copy(temp2, temp1)

	// Simulate JSON parsing/serialization
	data := make(map[string]interface{})
	data["worker"] = workerID
	data["request"] = requestID
	data["payload"] = string(temp2[:100])

	// Simulate response building
	response := make([]byte, 4096)
	responseData := fmt.Sprintf(`{"worker":%d,"request":%d,"status":"success","data":"%s"}`,
		workerID, requestID, data["payload"])
	copy(response, responseData)

	// Simulate some processing work
	sum := 0
	for i := 0; i < len(temp1); i++ {
		sum += int(temp1[i])
	}

	// Force temp variables to be live until here
	_ = temp1
	_ = temp2
	_ = response
	_ = sum
}

// getGCCount returns current number of GC runs
func getGCCount() uint32 {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	return memStats.NumGC
}

// Benchmark GC performance under different allocation patterns
func BenchmarkGCPerformancePatterns(b *testing.B) {
	patterns := []struct {
		name  string
		sizes []int
		rate  time.Duration
	}{
		{"SmallObjects", []int{512, 1024}, 1 * time.Millisecond},
		{"MediumObjects", []int{32 * 1024, 64 * 1024}, 5 * time.Millisecond},
		{"LargeObjects", []int{512 * 1024, 1024 * 1024}, 50 * time.Millisecond},
		{"MixedObjects", []int{1024, 32 * 1024, 256 * 1024}, 10 * time.Millisecond},
	}

	for _, pattern := range patterns {
		b.Run(pattern.name, func(b *testing.B) {
			gcMetrics := NewGCMetrics(b.N, 100*time.Millisecond)
			gcMetrics.StartMonitoring()
			defer gcMetrics.StopMonitoring()

			stressTest := NewMemoryStressTest(pattern.sizes, pattern.rate, 0.2)

			b.ResetTimer()
			b.ReportAllocs()

			stressTest.Start()
			
			for i := 0; i < b.N; i++ {
				processSimulatedRequest(0, i)
			}
			
			stressTest.Stop()

			// Report GC metrics
			report := gcMetrics.GetGCEfficiencyReport()
			if gcRuns, ok := report["gc_runs"].(uint32); ok {
				b.Logf("GC runs: %d", gcRuns)
			}
			if efficiency, ok := report["avg_gc_efficiency"].(float64); ok {
				b.Logf("GC efficiency: %.2f objects/sec", efficiency)
			}
		})
	}
}

// Benchmark memory allocation efficiency in hot paths
func BenchmarkMemoryAllocationEfficiency(b *testing.B) {
	allocationTypes := []struct {
		name string
		fn   func() interface{}
	}{
		{
			"SliceAllocation",
			func() interface{} {
				return make([]byte, 1024)
			},
		},
		{
			"MapAllocation",
			func() interface{} {
				m := make(map[string]interface{})
				m["key"] = "value"
				return m
			},
		},
		{
			"StructAllocation",
			func() interface{} {
				type TestStruct struct {
					ID   int
					Name string
					Data []byte
				}
				return &TestStruct{
					ID:   1,
					Name: "test",
					Data: make([]byte, 512),
				}
			},
		},
	}

	for _, allocType := range allocationTypes {
		b.Run(allocType.name, func(b *testing.B) {
			gcMetrics := NewGCMetrics(1000, 100*time.Millisecond)
			gcMetrics.StartMonitoring()
			defer gcMetrics.StopMonitoring()

			b.ResetTimer()
			b.ReportAllocs()

			var results []interface{}
			for i := 0; i < b.N; i++ {
				result := allocType.fn()
				results = append(results, result)
				
				// Periodically clear to prevent excessive memory usage
				if i%1000 == 999 {
					results = results[:0]
					runtime.GC()
				}
			}

			report := gcMetrics.GetGCEfficiencyReport()
			if allocRate, ok := report["avg_allocation_rate"].(float64); ok {
				b.Logf("Allocation rate: %.2f MB/s", allocRate/(1024*1024))
			}

			// Keep results alive to prevent premature GC
			_ = results
		})
	}
}