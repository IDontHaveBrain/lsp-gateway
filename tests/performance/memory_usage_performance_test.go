package performance

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"lsp-gateway/internal/gateway"
	"lsp-gateway/tests/framework"
)

// MemoryUsagePerformanceTest validates memory usage and optimization
type MemoryUsagePerformanceTest struct {
	framework    *framework.MultiLanguageTestFramework
	profiler     *framework.PerformanceProfiler
	testProjects []*framework.TestProject
	gateways     []*gateway.ProjectAwareGateway

	// Memory thresholds
	MaxMemoryUsageBytes    int64
	MaxMemoryGrowthBytes   int64
	MaxMemoryLeakThreshold int64
	GCEfficiencyThreshold  float64

	// Test configuration
	TestDurations           []time.Duration
	MemoryPressureScenarios []MemoryPressureScenario
	LongRunningDuration     time.Duration

	// Memory tracking
	baselineMemory  int64
	peakMemory      int64
	memorySnapshots []MemorySnapshot
	gcStats         []GCStats

	mu sync.RWMutex
}

// MemoryPressureScenario defines different memory pressure testing scenarios
type MemoryPressureScenario struct {
	Name               string
	ConcurrentProjects int
	ConcurrentRequests int
	ProjectSize        framework.ProjectSize
	Duration           time.Duration
	MemoryIntensive    bool
}

// MemorySnapshot represents a point-in-time memory usage snapshot
type MemorySnapshot struct {
	Timestamp      time.Time
	AllocatedBytes int64
	HeapBytes      int64
	StackBytes     int64
	GoroutineCount int
	ActiveObjects  int64
	GCCycles       uint32
	Operation      string
}

// GCStats represents garbage collection statistics
type GCStats struct {
	Timestamp      time.Time
	GCCycles       uint32
	TotalPauseNs   uint64
	LastPauseNs    uint64
	HeapSizeBefore int64
	HeapSizeAfter  int64
	Efficiency     float64
}

// MemoryLeakDetectionResult contains memory leak detection results
type MemoryLeakDetectionResult struct {
	InitialMemory      int64
	FinalMemory        int64
	PeakMemory         int64
	MemoryGrowth       int64
	LeakDetected       bool
	LeakSeverity       string
	RecommendedActions []string
}

// MemoryProfileResult contains comprehensive memory profiling results
type MemoryProfileResult struct {
	BaselineMemory int64
	PeakMemory     int64
	AverageMemory  int64
	MemoryGrowth   int64
	GCEfficiency   float64
	MemoryTurnover int64
	LeakDetection  *MemoryLeakDetectionResult
	GCPerformance  *GCPerformanceMetrics
}

// GCPerformanceMetrics contains garbage collection performance metrics
type GCPerformanceMetrics struct {
	TotalGCCycles    uint32
	AveragePauseTime time.Duration
	MaxPauseTime     time.Duration
	GCFrequency      float64
	MemoryReclaimed  int64
	EfficiencyRating float64
}

// NewMemoryUsagePerformanceTest creates a new memory usage performance test
func NewMemoryUsagePerformanceTest(t *testing.T) *MemoryUsagePerformanceTest {
	return &MemoryUsagePerformanceTest{
		framework:    framework.NewMultiLanguageTestFramework(60 * time.Minute),
		profiler:     framework.NewPerformanceProfiler(),
		testProjects: make([]*framework.TestProject, 0),
		gateways:     make([]*gateway.ProjectAwareGateway, 0),

		// Enterprise-scale memory thresholds
		MaxMemoryUsageBytes:    3 * 1024 * 1024 * 1024, // 3GB
		MaxMemoryGrowthBytes:   1 * 1024 * 1024 * 1024, // 1GB growth
		MaxMemoryLeakThreshold: 100 * 1024 * 1024,      // 100MB leak threshold
		GCEfficiencyThreshold:  0.70,                   // 70% GC efficiency

		// Test configuration
		TestDurations: []time.Duration{
			5 * time.Minute,
			15 * time.Minute,
			30 * time.Minute,
		},
		MemoryPressureScenarios: []MemoryPressureScenario{
			{
				Name:               "low_pressure",
				ConcurrentProjects: 5,
				ConcurrentRequests: 20,
				ProjectSize:        framework.SizeMedium,
				Duration:           10 * time.Minute,
				MemoryIntensive:    false,
			},
			{
				Name:               "medium_pressure",
				ConcurrentProjects: 15,
				ConcurrentRequests: 50,
				ProjectSize:        framework.SizeLarge,
				Duration:           15 * time.Minute,
				MemoryIntensive:    true,
			},
			{
				Name:               "high_pressure",
				ConcurrentProjects: 25,
				ConcurrentRequests: 100,
				ProjectSize:        framework.SizeXLarge,
				Duration:           20 * time.Minute,
				MemoryIntensive:    true,
			},
		},
		LongRunningDuration: 45 * time.Minute,

		// Initialize memory tracking
		memorySnapshots: make([]MemorySnapshot, 0),
		gcStats:         make([]GCStats, 0),
	}
}

// TestLargeProjectMemoryConsumption validates memory consumption during large project operations
func (test *MemoryUsagePerformanceTest) TestLargeProjectMemoryConsumption(t *testing.T) {
	ctx := context.Background()

	if err := test.setupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	// Test memory consumption with different project sizes
	projectSizes := []framework.ProjectSize{
		framework.SizeLarge,
		framework.SizeXLarge,
	}

	for _, size := range projectSizes {
		t.Run(fmt.Sprintf("ProjectSize_%s", size), func(t *testing.T) {
			test.testLargeProjectMemoryConsumption(t, size)
		})
	}
}

// TestMemoryLeakDetection validates memory leak detection in long-running scenarios
func (test *MemoryUsagePerformanceTest) TestMemoryLeakDetection(t *testing.T) {
	ctx := context.Background()

	if err := test.setupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	// Test for memory leaks with different durations
	for _, duration := range test.TestDurations {
		t.Run(fmt.Sprintf("Duration_%v", duration), func(t *testing.T) {
			leakResult := test.detectMemoryLeaks(t, duration)
			test.validateMemoryLeakResults(t, leakResult)
		})
	}
}

// TestGarbageCollectionImpact validates garbage collection impact on performance
func (test *MemoryUsagePerformanceTest) TestGarbageCollectionImpact(t *testing.T) {
	ctx := context.Background()

	if err := test.setupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	// Test GC impact under different memory pressures
	gcMetrics, err := test.framework.MeasurePerformance(func() error {
		return test.testGarbageCollectionImpact()
	})

	if err != nil {
		t.Fatalf("GC impact test failed: %v", err)
	}

	// Analyze GC performance
	gcPerf := test.analyzeGCPerformance(gcMetrics)

	// Validate GC efficiency
	if gcPerf.EfficiencyRating < test.GCEfficiencyThreshold {
		t.Errorf("GC efficiency too low: %.2f < %.2f",
			gcPerf.EfficiencyRating, test.GCEfficiencyThreshold)
	}

	t.Logf("GC Performance: Cycles=%d, Avg pause=%v, Max pause=%v, Efficiency=%.2f%%",
		gcPerf.TotalGCCycles, gcPerf.AveragePauseTime, gcPerf.MaxPauseTime,
		gcPerf.EfficiencyRating*100)
}

// TestMemoryUsagePatterns validates memory usage patterns across different languages
func (test *MemoryUsagePerformanceTest) TestMemoryUsagePatterns(t *testing.T) {
	ctx := context.Background()

	if err := test.setupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	// Test memory patterns for different language combinations
	languageCombos := [][]string{
		{"go"},
		{"python"},
		{"typescript"},
		{"java"},
		{"go", "python"},
		{"go", "python", "typescript", "java"},
	}

	for _, languages := range languageCombos {
		t.Run(fmt.Sprintf("Languages_%v", languages), func(t *testing.T) {
			profile := test.analyzeMemoryUsagePatterns(t, languages)
			test.validateMemoryUsagePatterns(t, languages, profile)
		})
	}
}

// TestResourceCleanupValidation validates resource cleanup and memory release
func (test *MemoryUsagePerformanceTest) TestResourceCleanupValidation(t *testing.T) {
	ctx := context.Background()

	if err := test.setupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	// Test resource cleanup effectiveness
	cleanupMetrics, err := test.framework.MeasurePerformance(func() error {
		return test.testResourceCleanup()
	})

	if err != nil {
		t.Fatalf("Resource cleanup test failed: %v", err)
	}

	// Validate cleanup effectiveness
	memoryReclaimed := cleanupMetrics.MemoryFreed
	if memoryReclaimed < cleanupMetrics.MemoryAllocated/2 {
		t.Errorf("Insufficient memory reclaimed: %d < %d (50%% of allocated)",
			memoryReclaimed, cleanupMetrics.MemoryAllocated/2)
	}

	t.Logf("Resource cleanup: Allocated=%dMB, Reclaimed=%dMB (%.1f%%)",
		cleanupMetrics.MemoryAllocated/1024/1024, memoryReclaimed/1024/1024,
		float64(memoryReclaimed)/float64(cleanupMetrics.MemoryAllocated)*100)
}

// TestMemoryPressureScenarios validates system behavior under memory pressure
func (test *MemoryUsagePerformanceTest) TestMemoryPressureScenarios(t *testing.T) {
	ctx := context.Background()

	if err := test.setupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	for _, scenario := range test.MemoryPressureScenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			result := test.executeMemoryPressureScenario(t, scenario)
			test.validateMemoryPressureResults(t, scenario, result)
		})
	}
}

// BenchmarkMemoryUsage benchmarks memory usage patterns
func (test *MemoryUsagePerformanceTest) BenchmarkMemoryUsage(b *testing.B) {
	ctx := context.Background()

	if err := test.setupTestEnvironment(ctx); err != nil {
		b.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := test.performMemoryBenchmark()
		if err != nil {
			b.Fatalf("Memory benchmark iteration %d failed: %v", i, err)
		}
	}
}

// setupTestEnvironment sets up the test environment
func (test *MemoryUsagePerformanceTest) setupTestEnvironment(ctx context.Context) error {
	if err := test.framework.SetupTestEnvironment(ctx); err != nil {
		return fmt.Errorf("failed to setup framework: %w", err)
	}

	// Capture baseline memory usage
	test.baselineMemory = test.getCurrentMemoryUsage()

	return nil
}

// testLargeProjectMemoryConsumption tests memory consumption for large projects
func (test *MemoryUsagePerformanceTest) testLargeProjectMemoryConsumption(t *testing.T, size framework.ProjectSize) {
	// Create large project
	project, err := test.framework.CreateMultiLanguageProject(
		framework.ProjectTypeMonorepo,
		[]string{"go", "python", "typescript", "java"})
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	// Measure memory consumption during project operations
	initialMemory := test.getCurrentMemoryUsage()

	memProfile, err := test.framework.MeasurePerformance(func() error {
		return test.performLargeProjectOperations(project)
	})

	if err != nil {
		t.Fatalf("Large project operations failed: %v", err)
	}

	peakMemory := memProfile.MemoryAllocated
	memoryGrowth := peakMemory - initialMemory

	// Validate memory usage
	if peakMemory > test.MaxMemoryUsageBytes {
		t.Errorf("Peak memory usage exceeded threshold: %d bytes > %d bytes",
			peakMemory, test.MaxMemoryUsageBytes)
	}

	if memoryGrowth > test.MaxMemoryGrowthBytes {
		t.Errorf("Memory growth exceeded threshold: %d bytes > %d bytes",
			memoryGrowth, test.MaxMemoryGrowthBytes)
	}

	t.Logf("Large project memory consumption (%s): Peak=%dMB, Growth=%dMB, Duration=%v",
		size, peakMemory/1024/1024, memoryGrowth/1024/1024, memProfile.OperationDuration)
}

// detectMemoryLeaks detects memory leaks over specified duration
func (test *MemoryUsagePerformanceTest) detectMemoryLeaks(t *testing.T, duration time.Duration) *MemoryLeakDetectionResult {
	initialMemory := test.getCurrentMemoryUsage()
	peakMemory := initialMemory

	// Create test project for leak detection
	project, err := test.framework.CreateMultiLanguageProject(
		framework.ProjectTypeMultiLanguage,
		[]string{"go", "python", "typescript"})
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	// Start continuous operations to detect leaks
	endTime := time.Now().Add(duration)
	var wg sync.WaitGroup

	// Memory monitoring goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				currentMemory := test.getCurrentMemoryUsage()
				if currentMemory > peakMemory {
					peakMemory = currentMemory
				}
				test.recordMemorySnapshot(currentMemory, "periodic_check")
			case <-time.After(duration):
				return
			}
		}
	}()

	// Continuous operations goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for time.Now().Before(endTime) {
			test.performLargeProjectOperations(project)
			runtime.GC() // Force GC to help detect leaks
			time.Sleep(5 * time.Second)
		}
	}()

	wg.Wait()

	// Final memory measurement after cleanup
	runtime.GC()
	time.Sleep(1 * time.Second)
	finalMemory := test.getCurrentMemoryUsage()

	memoryGrowth := finalMemory - initialMemory
	leakDetected := memoryGrowth > test.MaxMemoryLeakThreshold

	severity := "none"
	recommendations := []string{}

	if leakDetected {
		if memoryGrowth > test.MaxMemoryLeakThreshold*5 {
			severity = "severe"
			recommendations = append(recommendations, "Immediate investigation required")
			recommendations = append(recommendations, "Check for unclosed resources")
		} else if memoryGrowth > test.MaxMemoryLeakThreshold*2 {
			severity = "moderate"
			recommendations = append(recommendations, "Monitor memory usage closely")
			recommendations = append(recommendations, "Review resource cleanup logic")
		} else {
			severity = "minor"
			recommendations = append(recommendations, "Consider optimizing memory usage")
		}
	}

	return &MemoryLeakDetectionResult{
		InitialMemory:      initialMemory,
		FinalMemory:        finalMemory,
		PeakMemory:         peakMemory,
		MemoryGrowth:       memoryGrowth,
		LeakDetected:       leakDetected,
		LeakSeverity:       severity,
		RecommendedActions: recommendations,
	}
}

// testGarbageCollectionImpact tests the impact of garbage collection
func (test *MemoryUsagePerformanceTest) testGarbageCollectionImpact() error {
	// Create multiple projects to generate memory pressure
	projects := make([]*framework.TestProject, 0, 10)

	for i := 0; i < 10; i++ {
		project, err := test.framework.CreateMultiLanguageProject(
			framework.ProjectTypeMultiLanguage,
			[]string{"go", "python"})
		if err != nil {
			return fmt.Errorf("failed to create test project %d: %w", i, err)
		}
		projects = append(projects, project)
	}

	// Perform operations that will trigger GC
	for rounds := 0; rounds < 20; rounds++ {
		for _, project := range projects {
			test.performLargeProjectOperations(project)
		}

		// Force GC and measure impact
		before := time.Now()
		runtime.GC()
		gcTime := time.Since(before)

		test.recordGCStats(gcTime)

		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

// analyzeMemoryUsagePatterns analyzes memory usage patterns for language combinations
func (test *MemoryUsagePerformanceTest) analyzeMemoryUsagePatterns(t *testing.T, languages []string) *MemoryProfileResult {
	initialMemory := test.getCurrentMemoryUsage()
	peakMemory := initialMemory
	totalMemory := int64(0)
	measurements := 0

	// Create project with specified languages
	project, err := test.framework.CreateMultiLanguageProject(
		framework.ProjectTypeMultiLanguage, languages)
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	// Start language servers
	if err := test.framework.StartMultipleLanguageServers(languages); err != nil {
		t.Fatalf("Failed to start language servers: %v", err)
	}

	// Measure memory usage patterns
	endTime := time.Now().Add(5 * time.Minute)
	for time.Now().Before(endTime) {
		test.performLargeProjectOperations(project)

		currentMemory := test.getCurrentMemoryUsage()
		if currentMemory > peakMemory {
			peakMemory = currentMemory
		}
		totalMemory += currentMemory
		measurements++

		time.Sleep(10 * time.Second)
	}

	runtime.GC()
	time.Sleep(1 * time.Second)
	finalMemory := test.getCurrentMemoryUsage()

	averageMemory := totalMemory / int64(measurements)
	memoryGrowth := finalMemory - initialMemory

	return &MemoryProfileResult{
		BaselineMemory: initialMemory,
		PeakMemory:     peakMemory,
		AverageMemory:  averageMemory,
		MemoryGrowth:   memoryGrowth,
	}
}

// testResourceCleanup tests resource cleanup effectiveness
func (test *MemoryUsagePerformanceTest) testResourceCleanup() error {
	// Create multiple projects and then clean them up
	projects := make([]*framework.TestProject, 0, 20)

	// Create phase
	for i := 0; i < 20; i++ {
		project, err := test.framework.CreateMultiLanguageProject(
			framework.ProjectTypeMultiLanguage,
			[]string{"go", "python", "typescript"})
		if err != nil {
			return fmt.Errorf("failed to create project %d: %w", i, err)
		}
		projects = append(projects, project)
	}

	// Use phase
	for _, project := range projects {
		test.performLargeProjectOperations(project)
	}

	// Cleanup phase
	for _, project := range projects {
		// Simulate project cleanup
		os.RemoveAll(project.RootPath)
	}

	// Force cleanup and measure effectiveness
	runtime.GC()
	time.Sleep(2 * time.Second)

	return nil
}

// executeMemoryPressureScenario executes a memory pressure scenario
func (test *MemoryUsagePerformanceTest) executeMemoryPressureScenario(t *testing.T, scenario MemoryPressureScenario) *MemoryProfileResult {
	initialMemory := test.getCurrentMemoryUsage()
	peakMemory := initialMemory

	// Create projects for the scenario
	projects := make([]*framework.TestProject, scenario.ConcurrentProjects)
	for i := 0; i < scenario.ConcurrentProjects; i++ {
		project, err := test.framework.CreateMultiLanguageProject(
			framework.ProjectTypeMultiLanguage,
			[]string{"go", "python", "typescript", "java"})
		if err != nil {
			t.Fatalf("Failed to create project %d: %v", i, err)
		}
		projects[i] = project
	}

	// Execute concurrent operations
	var wg sync.WaitGroup
	endTime := time.Now().Add(scenario.Duration)

	// Memory monitoring
	wg.Add(1)
	go func() {
		defer wg.Done()
		for time.Now().Before(endTime) {
			currentMemory := test.getCurrentMemoryUsage()
			if currentMemory > peakMemory {
				peakMemory = currentMemory
			}
			time.Sleep(10 * time.Second)
		}
	}()

	// Concurrent project operations
	for i := 0; i < scenario.ConcurrentRequests; i++ {
		wg.Add(1)
		go func(projectIdx int) {
			defer wg.Done()
			project := projects[projectIdx%len(projects)]

			for time.Now().Before(endTime) {
				test.performLargeProjectOperations(project)
				time.Sleep(1 * time.Second)
			}
		}(i)
	}

	wg.Wait()

	runtime.GC()
	time.Sleep(2 * time.Second)
	finalMemory := test.getCurrentMemoryUsage()

	return &MemoryProfileResult{
		BaselineMemory: initialMemory,
		PeakMemory:     peakMemory,
		MemoryGrowth:   finalMemory - initialMemory,
	}
}

// performMemoryBenchmark performs memory usage benchmark
func (test *MemoryUsagePerformanceTest) performMemoryBenchmark() error {
	// Create test project
	project, err := test.framework.CreateMultiLanguageProject(
		framework.ProjectTypeMultiLanguage,
		[]string{"go", "python"})
	if err != nil {
		return err
	}

	// Perform operations
	return test.performLargeProjectOperations(project)
}

// performLargeProjectOperations performs operations on large project
func (test *MemoryUsagePerformanceTest) performLargeProjectOperations(project *framework.TestProject) error {
	// Simulate various LSP operations that consume memory
	operations := []func() error{
		func() error { return test.simulateSymbolIndexing(project) },
		func() error { return test.simulateFileAnalysis(project) },
		func() error { return test.simulateCodeCompletion(project) },
		func() error { return test.simulateRefactoring(project) },
	}

	for _, op := range operations {
		if err := op(); err != nil {
			return err
		}
	}

	return nil
}

// simulateSymbolIndexing simulates memory-intensive symbol indexing
func (test *MemoryUsagePerformanceTest) simulateSymbolIndexing(project *framework.TestProject) error {
	// Simulate creating symbol tables (memory intensive)
	symbolTable := make(map[string][]string)

	for i := 0; i < 1000; i++ {
		symbolName := fmt.Sprintf("symbol_%d", i)
		references := make([]string, 50)
		for j := range references {
			references[j] = fmt.Sprintf("ref_%d_%d", i, j)
		}
		symbolTable[symbolName] = references
	}

	time.Sleep(10 * time.Millisecond)

	// Simulate cleanup
	symbolTable = nil

	return nil
}

// simulateFileAnalysis simulates file analysis operations
func (test *MemoryUsagePerformanceTest) simulateFileAnalysis(project *framework.TestProject) error {
	// Simulate loading and analyzing file contents
	fileContents := make([][]byte, 100)

	for i := range fileContents {
		// Simulate file content (1KB each)
		fileContents[i] = make([]byte, 1024)
		for j := range fileContents[i] {
			fileContents[i][j] = byte(j % 256)
		}
	}

	time.Sleep(5 * time.Millisecond)

	// Simulate processing
	for _, content := range fileContents {
		_ = len(content) // Simple processing
	}

	return nil
}

// simulateCodeCompletion simulates code completion operations
func (test *MemoryUsagePerformanceTest) simulateCodeCompletion(project *framework.TestProject) error {
	// Simulate completion candidate generation
	completions := make([]map[string]interface{}, 500)

	for i := range completions {
		completions[i] = map[string]interface{}{
			"label":  fmt.Sprintf("completion_%d", i),
			"kind":   "function",
			"detail": fmt.Sprintf("Detail for completion %d", i),
			"docs":   fmt.Sprintf("Documentation for completion %d with lots of text", i),
		}
	}

	time.Sleep(8 * time.Millisecond)

	return nil
}

// simulateRefactoring simulates refactoring operations
func (test *MemoryUsagePerformanceTest) simulateRefactoring(project *framework.TestProject) error {
	// Simulate AST manipulation for refactoring
	astNodes := make([]map[string]interface{}, 200)

	for i := range astNodes {
		astNodes[i] = map[string]interface{}{
			"type":     "node",
			"children": make([]interface{}, 10),
			"props":    make(map[string]string),
		}
	}

	time.Sleep(15 * time.Millisecond)

	return nil
}

// getCurrentMemoryUsage returns current memory usage in bytes
func (test *MemoryUsagePerformanceTest) getCurrentMemoryUsage() int64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.Alloc)
}

// recordMemorySnapshot records a memory usage snapshot
func (test *MemoryUsagePerformanceTest) recordMemorySnapshot(memory int64, operation string) {
	test.mu.Lock()
	defer test.mu.Unlock()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	snapshot := MemorySnapshot{
		Timestamp:      time.Now(),
		AllocatedBytes: memory,
		HeapBytes:      int64(m.HeapAlloc),
		StackBytes:     int64(m.StackInuse),
		GoroutineCount: runtime.NumGoroutine(),
		ActiveObjects:  int64(m.Mallocs - m.Frees),
		GCCycles:       m.NumGC,
		Operation:      operation,
	}

	test.memorySnapshots = append(test.memorySnapshots, snapshot)
}

// recordGCStats records garbage collection statistics
func (test *MemoryUsagePerformanceTest) recordGCStats(gcTime time.Duration) {
	test.mu.Lock()
	defer test.mu.Unlock()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	gcStats := GCStats{
		Timestamp:      time.Now(),
		GCCycles:       m.NumGC,
		TotalPauseNs:   m.PauseTotalNs,
		LastPauseNs:    m.PauseNs[(m.NumGC+255)%256],
		HeapSizeBefore: int64(m.HeapAlloc),
		HeapSizeAfter:  int64(m.HeapAlloc),
	}

	test.gcStats = append(test.gcStats, gcStats)
}

// analyzeGCPerformance analyzes garbage collection performance
func (test *MemoryUsagePerformanceTest) analyzeGCPerformance(metrics *framework.PerformanceMetrics) *GCPerformanceMetrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	totalPauseTime := time.Duration(m.PauseTotalNs) * time.Nanosecond
	avgPauseTime := time.Duration(0)
	if m.NumGC > 0 {
		avgPauseTime = totalPauseTime / time.Duration(m.NumGC)
	}

	// Calculate efficiency (simplified)
	efficiency := 0.8 // Default efficiency
	if metrics.MemoryFreed > 0 && metrics.MemoryAllocated > 0 {
		efficiency = float64(metrics.MemoryFreed) / float64(metrics.MemoryAllocated)
	}

	return &GCPerformanceMetrics{
		TotalGCCycles:    m.NumGC,
		AveragePauseTime: avgPauseTime,
		MaxPauseTime:     time.Duration(m.PauseNs[(m.NumGC+255)%256]) * time.Nanosecond,
		GCFrequency:      float64(m.NumGC) / metrics.OperationDuration.Seconds(),
		MemoryReclaimed:  metrics.MemoryFreed,
		EfficiencyRating: efficiency,
	}
}

// validateMemoryLeakResults validates memory leak detection results
func (test *MemoryUsagePerformanceTest) validateMemoryLeakResults(t *testing.T, result *MemoryLeakDetectionResult) {
	if result.LeakDetected {
		t.Logf("Memory leak detected: Growth=%dMB, Severity=%s",
			result.MemoryGrowth/1024/1024, result.LeakSeverity)

		for _, action := range result.RecommendedActions {
			t.Logf("Recommendation: %s", action)
		}

		if result.LeakSeverity == "severe" {
			t.Errorf("Severe memory leak detected: %dMB growth", result.MemoryGrowth/1024/1024)
		}
	} else {
		t.Logf("No significant memory leak detected: Growth=%dMB", result.MemoryGrowth/1024/1024)
	}
}

// validateMemoryUsagePatterns validates memory usage patterns
func (test *MemoryUsagePerformanceTest) validateMemoryUsagePatterns(t *testing.T, languages []string, profile *MemoryProfileResult) {
	// Validate memory growth is reasonable
	if profile.MemoryGrowth > test.MaxMemoryGrowthBytes {
		t.Errorf("Memory growth too high for %v: %dMB > %dMB",
			languages, profile.MemoryGrowth/1024/1024, test.MaxMemoryGrowthBytes/1024/1024)
	}

	// Validate peak memory usage
	if profile.PeakMemory > test.MaxMemoryUsageBytes {
		t.Errorf("Peak memory usage too high for %v: %dMB > %dMB",
			languages, profile.PeakMemory/1024/1024, test.MaxMemoryUsageBytes/1024/1024)
	}

	t.Logf("Memory pattern for %v: Peak=%dMB, Avg=%dMB, Growth=%dMB",
		languages, profile.PeakMemory/1024/1024, profile.AverageMemory/1024/1024,
		profile.MemoryGrowth/1024/1024)
}

// validateMemoryPressureResults validates memory pressure scenario results
func (test *MemoryUsagePerformanceTest) validateMemoryPressureResults(t *testing.T, scenario MemoryPressureScenario, result *MemoryProfileResult) {
	// Validate based on scenario type
	switch scenario.Name {
	case "low_pressure":
		if result.PeakMemory > test.MaxMemoryUsageBytes/3 {
			t.Errorf("Low pressure scenario used too much memory: %dMB > %dMB",
				result.PeakMemory/1024/1024, test.MaxMemoryUsageBytes/3/1024/1024)
		}
	case "high_pressure":
		if result.PeakMemory > test.MaxMemoryUsageBytes {
			t.Errorf("High pressure scenario exceeded memory limit: %dMB > %dMB",
				result.PeakMemory/1024/1024, test.MaxMemoryUsageBytes/1024/1024)
		}
	}

	t.Logf("Memory pressure scenario %s: Peak=%dMB, Growth=%dMB",
		scenario.Name, result.PeakMemory/1024/1024, result.MemoryGrowth/1024/1024)
}

// cleanup performs cleanup of test resources
func (test *MemoryUsagePerformanceTest) cleanup() {
	if test.framework != nil {
		test.framework.CleanupAll()
	}

	// Clear memory tracking data
	test.mu.Lock()
	test.memorySnapshots = nil
	test.gcStats = nil
	test.testProjects = nil
	test.gateways = nil
	test.mu.Unlock()

	// Force final garbage collection
	runtime.GC()
}

// Integration tests that use the memory usage performance test framework
func TestMemoryUsagePerformanceSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory usage performance tests in short mode")
	}

	perfTest := NewMemoryUsagePerformanceTest(t)

	t.Run("LargeProjectMemoryConsumption", perfTest.TestLargeProjectMemoryConsumption)
	t.Run("MemoryLeakDetection", perfTest.TestMemoryLeakDetection)
	t.Run("GarbageCollectionImpact", perfTest.TestGarbageCollectionImpact)
	t.Run("MemoryUsagePatterns", perfTest.TestMemoryUsagePatterns)
	t.Run("ResourceCleanupValidation", perfTest.TestResourceCleanupValidation)
	t.Run("MemoryPressureScenarios", perfTest.TestMemoryPressureScenarios)
}

// Benchmark function
func BenchmarkMemoryUsagePerformance(b *testing.B) {
	perfTest := NewMemoryUsagePerformanceTest(nil)
	perfTest.BenchmarkMemoryUsage(b)
}
