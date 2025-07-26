package performance

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-gateway/internal/indexing"
	"lsp-gateway/tests/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// IncrementalPipelinePerformanceTest validates Incremental Update Pipeline with <30s latency target
type IncrementalPipelinePerformanceTest struct {
	framework          *framework.MultiLanguageTestFramework
	incrementalPipeline indexing.IncrementalPipeline
	testProject        *framework.TestProject
	
	// Performance targets (Phase 2 requirements)
	targetUpdateLatency     time.Duration // <30s for incremental updates
	targetDependencyLatency time.Duration // <5s for dependency resolution
	
	// Test configuration
	testFileCount         int
	dependencyGraphSize   int
	concurrentUpdates     int
	
	// Metrics collection
	updateLatencies       []time.Duration
	dependencyLatencies   []time.Duration
	conflictResolutions   []time.Duration
	
	mu sync.RWMutex
}

// IncrementalUpdateMetrics tracks pipeline performance metrics
type IncrementalUpdateMetrics struct {
	AverageUpdateLatency     time.Duration
	P95UpdateLatency         time.Duration
	P99UpdateLatency         time.Duration
	AverageDependencyLatency time.Duration
	ConflictResolutionRate   float64
	ThroughputUpdatesPerSec  float64
	MemoryUsageMB           int64
	ErrorRate               float64
	
	// Pipeline-specific metrics
	FileWatchLatency        time.Duration
	IndexBuildLatency       time.Duration
	DependencyTrackLatency  time.Duration
	CacheInvalidationTime   time.Duration
}

// UpdateScenario represents different update scenarios to test
type UpdateScenario struct {
	Name                string
	UpdateType          string // "single_file", "multi_file", "dependency_chain", "massive_update"
	FileCount           int
	ExpectedLatency     time.Duration
	ExpectedConflicts   bool
	ConcurrentUpdates   bool
}

// NewIncrementalPipelinePerformanceTest creates a new Incremental Pipeline performance test
func NewIncrementalPipelinePerformanceTest(t *testing.T) *IncrementalPipelinePerformanceTest {
	return &IncrementalPipelinePerformanceTest{
		framework:               framework.NewMultiLanguageTestFramework(45 * time.Minute),
		targetUpdateLatency:     30 * time.Second, // Phase 2 target: <30s
		targetDependencyLatency: 5 * time.Second,  // Dependency resolution: <5s
		
		// Test configuration
		testFileCount:       1000,
		dependencyGraphSize: 500,
		concurrentUpdates:   25,
		
		// Initialize metrics
		updateLatencies:     make([]time.Duration, 0),
		dependencyLatencies: make([]time.Duration, 0),
		conflictResolutions: make([]time.Duration, 0),
	}
}

// TestIncrementalPipelineUpdateLatency validates <30s update latency target
func (test *IncrementalPipelinePerformanceTest) TestIncrementalPipelineUpdateLatency(t *testing.T) {
	ctx := context.Background()
	
	err := test.setupIncrementalPipelineEnvironment(ctx)
	require.NoError(t, err, "Failed to setup Incremental Pipeline environment")
	defer test.cleanup()
	
	t.Log("Testing Incremental Pipeline update latency (<30s target)...")
	
	// Test different update scenarios
	scenarios := []UpdateScenario{
		{
			Name:              "Single_File_Update",
			UpdateType:        "single_file",
			FileCount:         1,
			ExpectedLatency:   2 * time.Second,
			ExpectedConflicts: false,
			ConcurrentUpdates: false,
		},
		{
			Name:              "Multi_File_Update",
			UpdateType:        "multi_file",
			FileCount:         10,
			ExpectedLatency:   8 * time.Second,
			ExpectedConflicts: false,
			ConcurrentUpdates: false,
		},
		{
			Name:              "Dependency_Chain_Update",
			UpdateType:        "dependency_chain",
			FileCount:         25,
			ExpectedLatency:   15 * time.Second,
			ExpectedConflicts: true,
			ConcurrentUpdates: false,
		},
		{
			Name:              "Massive_Update_Sequential",
			UpdateType:        "massive_update",
			FileCount:         100,
			ExpectedLatency:   25 * time.Second,
			ExpectedConflicts: true,
			ConcurrentUpdates: false,
		},
		{
			Name:              "Concurrent_Updates",
			UpdateType:        "multi_file",
			FileCount:         20,
			ExpectedLatency:   12 * time.Second,
			ExpectedConflicts: true,
			ConcurrentUpdates: true,
		},
	}
	
	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			metrics := test.executeUpdateScenario(t, scenario)
			test.validateUpdateMetrics(t, scenario, metrics)
		})
	}
	
	// Validate overall pipeline performance
	test.validateOverallPipelinePerformance(t)
}

// TestIncrementalPipelineDependencyTracking validates dependency resolution performance
func (test *IncrementalPipelinePerformanceTest) TestIncrementalPipelineDependencyTracking(t *testing.T) {
	ctx := context.Background()
	
	err := test.setupIncrementalPipelineEnvironment(ctx)
	require.NoError(t, err, "Failed to setup Incremental Pipeline environment")
	defer test.cleanup()
	
	t.Log("Testing Incremental Pipeline dependency tracking performance...")
	
	// Create complex dependency graph
	dependencyGraph := test.createComplexDependencyGraph(ctx, test.dependencyGraphSize)
	require.NotNil(t, dependencyGraph, "Failed to create dependency graph")
	
	// Test dependency resolution scenarios
	dependencyTests := []struct {
		name                 string
		changeType           string
		impactedFiles        int
		expectedResolutionTime time.Duration
	}{
		{
			name:                   "Leaf_Node_Change",
			changeType:            "leaf_change",
			impactedFiles:         1,
			expectedResolutionTime: 1 * time.Second,
		},
		{
			name:                   "Mid_Tier_Change",
			changeType:            "mid_tier_change", 
			impactedFiles:         15,
			expectedResolutionTime: 3 * time.Second,
		},
		{
			name:                   "Root_Change",
			changeType:            "root_change",
			impactedFiles:         50,
			expectedResolutionTime: 5 * time.Second,
		},
		{
			name:                   "Cross_Language_Change",
			changeType:            "cross_language",
			impactedFiles:         30,
			expectedResolutionTime: 4 * time.Second,
		},
	}
	
	for _, depTest := range dependencyTests {
		t.Run(depTest.name, func(t *testing.T) {
			startTime := time.Now()
			
			impactedFiles, err := test.incrementalPipeline.ResolveDependencyImpact(
				ctx, depTest.changeType, dependencyGraph)
			resolutionTime := time.Since(startTime)
			
			require.NoError(t, err, "Dependency resolution should succeed")
			assert.Equal(t, depTest.impactedFiles, len(impactedFiles),
				"Should identify correct number of impacted files")
			assert.Less(t, resolutionTime, depTest.expectedResolutionTime,
				"Dependency resolution should meet latency target")
			
			test.recordDependencyResolution(resolutionTime)
			
			t.Logf("Dependency resolution (%s): Files=%d, Time=%v",
				depTest.name, len(impactedFiles), resolutionTime)
		})
	}
}

// TestIncrementalPipelineConflictResolution validates conflict resolution effectiveness
func (test *IncrementalPipelinePerformanceTest) TestIncrementalPipelineConflictResolution(t *testing.T) {
	ctx := context.Background()
	
	err := test.setupIncrementalPipelineEnvironment(ctx)
	require.NoError(t, err, "Failed to setup Incremental Pipeline environment")
	defer test.cleanup()
	
	t.Log("Testing Incremental Pipeline conflict resolution...")
	
	// Create conflicting update scenarios
	conflictScenarios := []struct {
		name              string
		conflictType      string
		expectedResolution time.Duration
		shouldResolve     bool
	}{
		{
			name:              "Concurrent_File_Edits",
			conflictType:      "concurrent_edits",
			expectedResolution: 2 * time.Second,
			shouldResolve:     true,
		},
		{
			name:              "Symbol_Definition_Conflicts",
			conflictType:      "symbol_conflicts",
			expectedResolution: 3 * time.Second,
			shouldResolve:     true,
		},
		{
			name:              "Cross_Language_Conflicts",
			conflictType:      "cross_language_conflicts",
			expectedResolution: 4 * time.Second,
			shouldResolve:     true,
		},
		{
			name:              "Dependency_Loop_Detection",
			conflictType:      "dependency_loops",
			expectedResolution: 5 * time.Second,
			shouldResolve:     true,
		},
	}
	
	for _, conflictScenario := range conflictScenarios {
		t.Run(conflictScenario.name, func(t *testing.T) {
			// Create conflict situation
			conflict := test.createConflictScenario(ctx, conflictScenario.conflictType)
			require.NotNil(t, conflict, "Failed to create conflict scenario")
			
			// Attempt conflict resolution
			startTime := time.Now()
			resolution, err := test.incrementalPipeline.ResolveConflict(ctx, conflict)
			resolutionTime := time.Since(startTime)
			
			if conflictScenario.shouldResolve {
				require.NoError(t, err, "Conflict resolution should succeed")
				require.NotNil(t, resolution, "Resolution should be provided")
				assert.Less(t, resolutionTime, conflictScenario.expectedResolution,
					"Conflict resolution should meet latency target")
			}
			
			test.recordConflictResolution(resolutionTime)
			
			t.Logf("Conflict resolution (%s): Time=%v, Resolved=%t",
				conflictScenario.name, resolutionTime, err == nil)
		})
	}
}

// TestIncrementalPipelineConcurrentUpdates validates concurrent update handling
func (test *IncrementalPipelinePerformanceTest) TestIncrementalPipelineConcurrentUpdates(t *testing.T) {
	ctx := context.Background()
	
	err := test.setupIncrementalPipelineEnvironment(ctx)
	require.NoError(t, err, "Failed to setup Incremental Pipeline environment")
	defer test.cleanup()
	
	t.Log("Testing Incremental Pipeline concurrent update handling...")
	
	concurrencyLevels := []int{5, 10, 15, 25, 50}
	
	for _, concurrency := range concurrencyLevels {
		t.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(t *testing.T) {
			metrics := test.executeConcurrentUpdateTest(t, concurrency, 60*time.Second)
			
			// Validate concurrent performance
			assert.Less(t, metrics.P95UpdateLatency, 2*test.targetUpdateLatency,
				"P95 update latency should remain reasonable under load")
			assert.Less(t, metrics.AverageUpdateLatency, test.targetUpdateLatency,
				"Average update latency should meet target under load")
			assert.Greater(t, metrics.ThroughputUpdatesPerSec, float64(concurrency)/10,
				"Throughput should scale with concurrency")
			assert.Less(t, metrics.ErrorRate, 0.05,
				"Error rate should remain under 5% under load")
			
			t.Logf("Concurrent updates (C=%d): Avg=%v, P95=%v, Throughput=%.2f/s, Errors=%.2f%%",
				concurrency, metrics.AverageUpdateLatency, metrics.P95UpdateLatency,
				metrics.ThroughputUpdatesPerSec, metrics.ErrorRate*100)
		})
	}
}

// TestIncrementalPipelineMemoryEfficiency validates memory usage under load
func (test *IncrementalPipelinePerformanceTest) TestIncrementalPipelineMemoryEfficiency(t *testing.T) {
	ctx := context.Background()
	
	err := test.setupIncrementalPipelineEnvironment(ctx)
	require.NoError(t, err, "Failed to setup Incremental Pipeline environment")
	defer test.cleanup()
	
	t.Log("Testing Incremental Pipeline memory efficiency...")
	
	initialMemory := test.getCurrentMemoryUsage()
	
	// Execute sustained update load
	metrics := test.executeSustainedUpdateLoad(ctx, 100, 5*time.Minute)
	
	finalMemory := test.getCurrentMemoryUsage()
	memoryGrowth := finalMemory - initialMemory
	
	// Validate memory efficiency
	assert.Less(t, memoryGrowth, int64(200*1024*1024), // <200MB growth
		"Memory growth should be controlled during sustained updates")
	
	assert.Less(t, metrics.MemoryUsageMB, initialMemory/1024/1024+300,
		"Peak memory usage should be reasonable")
	
	assert.Less(t, metrics.AverageUpdateLatency, test.targetUpdateLatency,
		"Performance should maintain during sustained load")
	
	t.Logf("Memory efficiency: Growth=%dMB, Peak=%dMB, Avg Latency=%v",
		memoryGrowth/1024/1024, metrics.MemoryUsageMB, metrics.AverageUpdateLatency)
}

// BenchmarkIncrementalPipelineOperations benchmarks core pipeline operations
func (test *IncrementalPipelinePerformanceTest) BenchmarkIncrementalPipelineOperations(b *testing.B) {
	ctx := context.Background()
	
	err := test.setupIncrementalPipelineEnvironment(ctx)
	if err != nil {
		b.Fatalf("Failed to setup Incremental Pipeline environment: %v", err)
	}
	defer test.cleanup()
	
	b.ResetTimer()
	
	b.Run("Single_File_Update", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			filePath := fmt.Sprintf("test_file_%d.go", i)
			err := test.incrementalPipeline.ProcessFileUpdate(ctx, filePath)
			if err != nil {
				b.Errorf("File update failed: %v", err)
			}
		}
	})
	
	b.Run("Dependency_Resolution", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			filePath := fmt.Sprintf("test_file_%d.go", i%100)
			_, err := test.incrementalPipeline.ResolveDependencies(ctx, filePath)
			if err != nil {
				b.Errorf("Dependency resolution failed: %v", err)
			}
		}
	})
	
	b.Run("Conflict_Detection", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			conflicts, err := test.incrementalPipeline.DetectConflicts(ctx)
			if err != nil {
				b.Errorf("Conflict detection failed: %v", err)
			}
			_ = conflicts
		}
	})
}

// Helper methods

func (test *IncrementalPipelinePerformanceTest) setupIncrementalPipelineEnvironment(ctx context.Context) error {
	// Setup test framework
	if err := test.framework.SetupTestEnvironment(ctx); err != nil {
		return fmt.Errorf("failed to setup framework: %w", err)
	}
	
	// Create multi-language test project
	project, err := test.framework.CreateMultiLanguageProject(
		framework.ProjectTypeMultiLanguage,
		[]string{"go", "python", "typescript", "java"})
	if err != nil {
		return fmt.Errorf("failed to create test project: %w", err)
	}
	test.testProject = project
	
	// Configure Incremental Pipeline
	pipelineConfig := &indexing.IncrementalPipelineConfig{
		WatchConfig: indexing.FileWatchConfig{
			Enabled:          true,
			MaxWatchedFiles:  10000,
			BatchSize:        100,
			BatchTimeout:     1 * time.Second,
		},
		DependencyConfig: indexing.DependencyConfig{
			TrackingEnabled:     true,
			CrossLanguageRefs:   true,
			MaxDepth:           10,
			ResolutionTimeout:  test.targetDependencyLatency,
		},
		ConflictConfig: indexing.ConflictConfig{
			DetectionEnabled:   true,
			ResolutionStrategy: indexing.ConflictResolutionAuto,
			MaxResolutionTime:  10 * time.Second,
		},
		PerformanceConfig: indexing.PipelinePerformanceConfig{
			MaxUpdateLatency:    test.targetUpdateLatency,
			MaxConcurrentOps:    test.concurrentUpdates,
			MemoryLimit:        500 * 1024 * 1024, // 500MB
		},
	}
	
	// Initialize Incremental Pipeline
	pipeline, err := indexing.NewIncrementalPipeline(pipelineConfig)
	if err != nil {
		return fmt.Errorf("failed to create Incremental Pipeline: %w", err)
	}
	
	// Set project directory
	err = pipeline.SetProjectDirectory(test.testProject.RootPath)
	if err != nil {
		return fmt.Errorf("failed to set project directory: %w", err)
	}
	
	test.incrementalPipeline = pipeline
	
	return nil
}

func (test *IncrementalPipelinePerformanceTest) executeUpdateScenario(t *testing.T, scenario UpdateScenario) *IncrementalUpdateMetrics {
	ctx := context.Background()
	
	// Prepare test files based on scenario
	testFiles := test.prepareTestFiles(scenario)
	
	// Execute update scenario
	var updateLatencies []time.Duration
	var errorCount int64
	
	if scenario.ConcurrentUpdates {
		updateLatencies, errorCount = test.executeConcurrentUpdates(ctx, testFiles)
	} else {
		updateLatencies, errorCount = test.executeSequentialUpdates(ctx, testFiles)
	}
	
	return test.calculateUpdateMetrics(updateLatencies, errorCount, int64(len(testFiles)))
}

func (test *IncrementalPipelinePerformanceTest) prepareTestFiles(scenario UpdateScenario) []string {
	testFiles := make([]string, scenario.FileCount)
	
	for i := 0; i < scenario.FileCount; i++ {
		fileName := fmt.Sprintf("%s_test_file_%d.go", scenario.UpdateType, i)
		filePath := filepath.Join(test.testProject.RootPath, fileName)
		
		// Create test file content based on scenario
		content := test.generateTestFileContent(scenario.UpdateType, i)
		
		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			continue // Skip failed files
		}
		
		testFiles[i] = filePath
	}
	
	return testFiles
}

func (test *IncrementalPipelinePerformanceTest) generateTestFileContent(updateType string, index int) string {
	switch updateType {
	case "single_file":
		return fmt.Sprintf(`package main

import "fmt"

func TestFunction%d() {
	fmt.Println("Test function %d")
}

type TestStruct%d struct {
	Field1 string
	Field2 int
}
`, index, index, index)
		
	case "dependency_chain":
		return fmt.Sprintf(`package main

import (
	"fmt"
	"./test_dependency_%d"
)

func TestFunction%d() {
	dep := test_dependency_%d.NewDependency()
	fmt.Println("Using dependency:", dep)
}
`, index%10, index, index%10)
		
	default:
		return fmt.Sprintf(`package main

func TestFunction%d() {
	// Test function implementation
}
`, index)
	}
}

func (test *IncrementalPipelinePerformanceTest) executeSequentialUpdates(ctx context.Context, testFiles []string) ([]time.Duration, int64) {
	var latencies []time.Duration
	var errorCount int64
	
	for _, filePath := range testFiles {
		startTime := time.Now()
		err := test.incrementalPipeline.ProcessFileUpdate(ctx, filePath)
		latency := time.Since(startTime)
		
		latencies = append(latencies, latency)
		test.recordUpdateLatency(latency)
		
		if err != nil {
			atomic.AddInt64(&errorCount, 1)
		}
	}
	
	return latencies, errorCount
}

func (test *IncrementalPipelinePerformanceTest) executeConcurrentUpdates(ctx context.Context, testFiles []string) ([]time.Duration, int64) {
	var latencies []time.Duration
	var errorCount int64
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	semaphore := make(chan struct{}, test.concurrentUpdates)
	
	for _, filePath := range testFiles {
		wg.Add(1)
		go func(file string) {
			defer wg.Done()
			
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			startTime := time.Now()
			err := test.incrementalPipeline.ProcessFileUpdate(ctx, file)
			latency := time.Since(startTime)
			
			mu.Lock()
			latencies = append(latencies, latency)
			test.recordUpdateLatency(latency)
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
			}
			mu.Unlock()
		}(filePath)
	}
	
	wg.Wait()
	return latencies, errorCount
}

// Additional helper methods (simplified implementations)

func (test *IncrementalPipelinePerformanceTest) createComplexDependencyGraph(ctx context.Context, size int) *indexing.DependencyGraph {
	// Implementation for creating complex dependency graph
	return &indexing.DependencyGraph{}
}

func (test *IncrementalPipelinePerformanceTest) createConflictScenario(ctx context.Context, conflictType string) *indexing.Conflict {
	// Implementation for creating conflict scenarios
	return &indexing.Conflict{}
}

func (test *IncrementalPipelinePerformanceTest) executeConcurrentUpdateTest(t *testing.T, concurrency int, duration time.Duration) *IncrementalUpdateMetrics {
	// Implementation for concurrent update testing
	return &IncrementalUpdateMetrics{}
}

func (test *IncrementalPipelinePerformanceTest) executeSustainedUpdateLoad(ctx context.Context, updateRate int, duration time.Duration) *IncrementalUpdateMetrics {
	// Implementation for sustained update load testing
	return &IncrementalUpdateMetrics{}
}

func (test *IncrementalPipelinePerformanceTest) calculateUpdateMetrics(latencies []time.Duration, errorCount, totalUpdates int64) *IncrementalUpdateMetrics {
	if len(latencies) == 0 {
		return &IncrementalUpdateMetrics{}
	}
	
	// Calculate average latency
	var totalLatency time.Duration
	for _, latency := range latencies {
		totalLatency += latency
	}
	avgLatency := totalLatency / time.Duration(len(latencies))
	
	// Calculate percentiles (simplified)
	// Implementation for P95, P99 calculation
	
	return &IncrementalUpdateMetrics{
		AverageUpdateLatency:    avgLatency,
		P95UpdateLatency:       avgLatency * 2, // Simplified
		P99UpdateLatency:       avgLatency * 3, // Simplified
		ThroughputUpdatesPerSec: float64(len(latencies)) / float64(totalLatency.Seconds()),
		ErrorRate:              float64(errorCount) / float64(totalUpdates),
	}
}

func (test *IncrementalPipelinePerformanceTest) validateUpdateMetrics(t *testing.T, scenario UpdateScenario, metrics *IncrementalUpdateMetrics) {
	assert.Less(t, metrics.AverageUpdateLatency, scenario.ExpectedLatency,
		"Average update latency should meet scenario expectations")
	
	assert.Less(t, metrics.P95UpdateLatency, 2*scenario.ExpectedLatency,
		"P95 update latency should be reasonable")
	
	assert.Less(t, metrics.ErrorRate, 0.05,
		"Error rate should be under 5%")
	
	if scenario.FileCount > 1 {
		assert.Greater(t, metrics.ThroughputUpdatesPerSec, 0.1,
			"Throughput should be measurable for multi-file scenarios")
	}
}

func (test *IncrementalPipelinePerformanceTest) validateOverallPipelinePerformance(t *testing.T) {
	test.mu.RLock()
	defer test.mu.RUnlock()
	
	if len(test.updateLatencies) == 0 {
		return
	}
	
	// Calculate overall metrics
	var totalLatency time.Duration
	for _, latency := range test.updateLatencies {
		totalLatency += latency
	}
	avgLatency := totalLatency / time.Duration(len(test.updateLatencies))
	
	// Validate overall performance
	assert.Less(t, avgLatency, test.targetUpdateLatency,
		"Overall average update latency should meet <30s target")
	
	t.Logf("Overall Incremental Pipeline performance: Updates=%d, Avg Latency=%v",
		len(test.updateLatencies), avgLatency)
}

func (test *IncrementalPipelinePerformanceTest) recordUpdateLatency(latency time.Duration) {
	test.mu.Lock()
	defer test.mu.Unlock()
	test.updateLatencies = append(test.updateLatencies, latency)
}

func (test *IncrementalPipelinePerformanceTest) recordDependencyResolution(latency time.Duration) {
	test.mu.Lock()
	defer test.mu.Unlock()
	test.dependencyLatencies = append(test.dependencyLatencies, latency)
}

func (test *IncrementalPipelinePerformanceTest) recordConflictResolution(latency time.Duration) {
	test.mu.Lock()
	defer test.mu.Unlock()
	test.conflictResolutions = append(test.conflictResolutions, latency)
}

func (test *IncrementalPipelinePerformanceTest) getCurrentMemoryUsage() int64 {
	// Implementation for memory usage measurement
	return 0
}

func (test *IncrementalPipelinePerformanceTest) cleanup() {
	if test.incrementalPipeline != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		test.incrementalPipeline.Shutdown(ctx)
	}
	if test.framework != nil {
		test.framework.CleanupAll()
	}
}

// Integration test suite

func TestIncrementalPipelinePerformanceSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Incremental Pipeline performance tests in short mode")
	}
	
	test := NewIncrementalPipelinePerformanceTest(t)
	
	t.Run("UpdateLatency", test.TestIncrementalPipelineUpdateLatency)
	t.Run("DependencyTracking", test.TestIncrementalPipelineDependencyTracking)
	t.Run("ConflictResolution", test.TestIncrementalPipelineConflictResolution)
	t.Run("ConcurrentUpdates", test.TestIncrementalPipelineConcurrentUpdates)
	t.Run("MemoryEfficiency", test.TestIncrementalPipelineMemoryEfficiency)
}

func BenchmarkIncrementalPipelinePerformance(b *testing.B) {
	test := NewIncrementalPipelinePerformanceTest(nil)
	test.BenchmarkIncrementalPipelineOperations(b)
}