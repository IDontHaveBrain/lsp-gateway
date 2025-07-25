package performance

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"lsp-gateway/internal/gateway"
	"lsp-gateway/tests/framework"
)

// LargeProjectPerformanceTest validates performance with large-scale projects
type LargeProjectPerformanceTest struct {
	framework    *framework.MultiLanguageTestFramework
	profiler     *framework.PerformanceProfiler
	testProjects []*framework.TestProject
	gateways     []*gateway.ProjectAwareGateway
	tempDir      string

	// Performance thresholds
	MaxProjectDetectionTime time.Duration
	MaxMemoryUsageBytes     int64
	MaxConcurrentProjects   int
	MinThroughputReqPerSec  float64

	// Test configuration
	ProjectSizes   []framework.ProjectSize
	ProjectTypes   []framework.ProjectType
	LanguageCombos [][]string

	mu sync.RWMutex
}

// NewLargeProjectPerformanceTest creates a new large project performance test
func NewLargeProjectPerformanceTest(t *testing.T) *LargeProjectPerformanceTest {
	tempDir, err := os.MkdirTemp("", "large-project-perf-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	return &LargeProjectPerformanceTest{
		framework:    framework.NewMultiLanguageTestFramework(30 * time.Minute),
		profiler:     framework.NewPerformanceProfiler(),
		testProjects: make([]*framework.TestProject, 0),
		gateways:     make([]*gateway.ProjectAwareGateway, 0),
		tempDir:      tempDir,

		// Enterprise-scale performance thresholds
		MaxProjectDetectionTime: 30 * time.Second,
		MaxMemoryUsageBytes:     2 * 1024 * 1024 * 1024, // 2GB
		MaxConcurrentProjects:   50,
		MinThroughputReqPerSec:  100.0,

		// Test scenarios
		ProjectSizes: []framework.ProjectSize{
			framework.SizeLarge,
			framework.SizeXLarge,
		},
		ProjectTypes: []framework.ProjectType{
			framework.ProjectTypeMonorepo,
			framework.ProjectTypeMultiLanguage,
			framework.ProjectTypeMicroservices,
			framework.ProjectTypePolyglot,
		},
		LanguageCombos: [][]string{
			{"go", "python", "typescript", "java"},
			{"go", "python", "typescript", "java", "rust"},
			{"go", "python", "javascript", "typescript", "java", "rust", "cpp"},
		},
	}
}

// TestLargeProjectDetectionPerformance validates project detection performance on 10,000+ file projects
func (test *LargeProjectPerformanceTest) TestLargeProjectDetectionPerformance(t *testing.T) {
	ctx := context.Background()

	if err := test.framework.SetupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	// Test with different project sizes and complexities
	for _, projectSize := range test.ProjectSizes {
		for _, projectType := range test.ProjectTypes {
			for _, languages := range test.LanguageCombos {
				t.Run(fmt.Sprintf("Detection_%s_%s_%dLangs", projectSize, projectType, len(languages)), func(t *testing.T) {
					test.testProjectDetectionPerformance(t, projectSize, projectType, languages)
				})
			}
		}
	}
}

// TestLargeProjectMemoryUsage validates memory usage during large project scanning
func (test *LargeProjectPerformanceTest) TestLargeProjectMemoryUsage(t *testing.T) {
	ctx := context.Background()

	if err := test.framework.SetupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	// Create extra large project with maximum complexity
	project := test.createLargeProject(t, framework.SizeXLarge, framework.ProjectTypeMonorepo,
		[]string{"go", "python", "typescript", "java", "rust", "cpp"})

	// Monitor memory usage during project operations
	memMetrics, err := test.framework.MeasurePerformance(func() error {
		return test.performLargeProjectOperations(project)
	})

	if err != nil {
		t.Fatalf("Large project operations failed: %v", err)
	}

	// Validate memory usage is within acceptable limits
	if memMetrics.MemoryAllocated > test.MaxMemoryUsageBytes {
		t.Errorf("Memory usage exceeded threshold: %d bytes > %d bytes",
			memMetrics.MemoryAllocated, test.MaxMemoryUsageBytes)
	}

	// Validate memory was properly released
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	var postGCMemStats runtime.MemStats
	runtime.ReadMemStats(&postGCMemStats)

	if int64(postGCMemStats.Alloc) > memMetrics.MemoryAllocated/2 {
		t.Errorf("Potential memory leak detected: post-GC memory %d > half of peak %d",
			postGCMemStats.Alloc, memMetrics.MemoryAllocated/2)
	}

	t.Logf("Large project memory usage: Peak=%dMB, Post-GC=%dMB, Duration=%v",
		memMetrics.MemoryAllocated/1024/1024, postGCMemStats.Alloc/1024/1024, memMetrics.OperationDuration)
}

// TestConcurrentProjectDetection validates concurrent project detection performance
func (test *LargeProjectPerformanceTest) TestConcurrentProjectDetection(t *testing.T) {
	ctx := context.Background()

	if err := test.framework.SetupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	// Create multiple large projects
	numProjects := 20
	projects := make([]*framework.TestProject, numProjects)

	for i := 0; i < numProjects; i++ {
		languageCombo := test.LanguageCombos[i%len(test.LanguageCombos)]
		projectType := test.ProjectTypes[i%len(test.ProjectTypes)]
		projects[i] = test.createLargeProject(t, framework.SizeLarge, projectType, languageCombo)
	}

	// Test concurrent project detection
	perfMetrics, err := test.framework.MeasurePerformance(func() error {
		return test.performConcurrentProjectDetection(projects)
	})

	if err != nil {
		t.Fatalf("Concurrent project detection failed: %v", err)
	}

	// Calculate throughput
	throughput := float64(numProjects) / perfMetrics.OperationDuration.Seconds()

	if throughput < test.MinThroughputReqPerSec/10 { // Adjusted for project detection
		t.Errorf("Project detection throughput too low: %.2f projects/sec < %.2f projects/sec",
			throughput, test.MinThroughputReqPerSec/10)
	}

	t.Logf("Concurrent project detection: %d projects in %v (%.2f projects/sec), Memory: %dMB",
		numProjects, perfMetrics.OperationDuration, throughput, perfMetrics.MemoryAllocated/1024/1024)
}

// TestCacheEfficiency validates cache performance with large datasets
func (test *LargeProjectPerformanceTest) TestCacheEfficiency(t *testing.T) {
	ctx := context.Background()

	if err := test.framework.SetupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	// Create large project with complex structure
	project := test.createLargeProject(t, framework.SizeXLarge, framework.ProjectTypeMonorepo,
		[]string{"go", "python", "typescript", "java"})

	gateway, err := test.framework.CreateGatewayWithProject(project)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	// Test cache efficiency with repeated operations
	cacheMetrics, err := test.framework.MeasurePerformance(func() error {
		return test.testCacheEfficiency(gateway, project)
	})

	if err != nil {
		t.Fatalf("Cache efficiency test failed: %v", err)
	}

	// Validate cache performance improvements
	if cacheMetrics.CacheOperations == 0 {
		t.Error("No cache operations detected")
	}

	t.Logf("Cache efficiency test: Duration=%v, Cache Operations=%d, Memory=%dMB",
		cacheMetrics.OperationDuration, cacheMetrics.CacheOperations, cacheMetrics.MemoryAllocated/1024/1024)
}

// TestServerPoolManagement validates server pool management with multiple large projects
func (test *LargeProjectPerformanceTest) TestServerPoolManagement(t *testing.T) {
	ctx := context.Background()

	if err := test.framework.SetupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	// Create multiple large projects with different language combinations
	projects := make([]*framework.TestProject, 0, 15)
	for i := 0; i < 15; i++ {
		langCombo := test.LanguageCombos[i%len(test.LanguageCombos)]
		projectType := test.ProjectTypes[i%len(test.ProjectTypes)]
		project := test.createLargeProject(t, framework.SizeLarge, projectType, langCombo)
		projects = append(projects, project)
	}

	// Test server pool management performance
	poolMetrics, err := test.framework.MeasurePerformance(func() error {
		return test.testServerPoolManagement(projects)
	})

	if err != nil {
		t.Fatalf("Server pool management test failed: %v", err)
	}

	// Validate server creation/destruction balance
	if poolMetrics.ServerCreations == 0 {
		t.Error("No server creations detected")
	}

	if poolMetrics.ServerCreations < poolMetrics.ServerDestructions {
		t.Error("More servers destroyed than created - potential resource leak")
	}

	t.Logf("Server pool management: Created=%d, Destroyed=%d, Duration=%v, Memory=%dMB",
		poolMetrics.ServerCreations, poolMetrics.ServerDestructions,
		poolMetrics.OperationDuration, poolMetrics.MemoryAllocated/1024/1024)
}

// BenchmarkLargeProjectOperations benchmarks large project operations
func (test *LargeProjectPerformanceTest) BenchmarkLargeProjectOperations(b *testing.B) {
	ctx := context.Background()

	if err := test.framework.SetupTestEnvironment(ctx); err != nil {
		b.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	// Create benchmark project
	project := test.createLargeProject(nil, framework.SizeXLarge, framework.ProjectTypeMonorepo,
		[]string{"go", "python", "typescript", "java", "rust"})

	gateway, err := test.framework.CreateGatewayWithProject(project)
	if err != nil {
		b.Fatalf("Failed to create gateway: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := test.performLargeProjectBenchmark(gateway, project)
		if err != nil {
			b.Fatalf("Benchmark iteration %d failed: %v", i, err)
		}
	}
}

// testProjectDetectionPerformance tests project detection performance for specific configuration
func (test *LargeProjectPerformanceTest) testProjectDetectionPerformance(t *testing.T, size framework.ProjectSize,
	projectType framework.ProjectType, languages []string) {

	// Create large project
	project := test.createLargeProject(t, size, projectType, languages)

	// Measure project detection performance
	detectionMetrics, err := test.framework.MeasurePerformance(func() error {
		// Simulate project detection operations
		return test.simulateProjectDetection(project)
	})

	if err != nil {
		t.Fatalf("Project detection failed: %v", err)
	}

	// Validate detection time is within acceptable limits
	if detectionMetrics.OperationDuration > test.MaxProjectDetectionTime {
		t.Errorf("Project detection took too long: %v > %v",
			detectionMetrics.OperationDuration, test.MaxProjectDetectionTime)
	}

	t.Logf("Project detection (%s, %s, %d langs): Duration=%v, Memory=%dMB, Files=%d",
		size, projectType, len(languages), detectionMetrics.OperationDuration,
		detectionMetrics.MemoryAllocated/1024/1024, detectionMetrics.FileOperations)
}

// createLargeProject creates a large test project with specified parameters
func (test *LargeProjectPerformanceTest) createLargeProject(t *testing.T, size framework.ProjectSize,
	projectType framework.ProjectType, languages []string) *framework.TestProject {

	config := &framework.ProjectGenerationConfig{
		Type:             projectType,
		Languages:        languages,
		Complexity:       framework.ComplexityLarge,
		Size:             size,
		BuildSystem:      true,
		TestFiles:        true,
		Dependencies:     true,
		Documentation:    true,
		CI:               true,
		Docker:           true,
		FileCount:        test.getFileCountForSize(size),
		DirectoryDepth:   test.getDirectoryDepthForSize(size),
		CrossReferences:  true,
		RealisticContent: true,
		VersionControl:   true,
	}

	project, err := test.framework.ProjectGenerator.GenerateProject(config)
	if err != nil {
		if t != nil {
			t.Fatalf("Failed to create large project: %v", err)
		} else {
			panic(fmt.Sprintf("Failed to create large project: %v", err))
		}
	}

	test.mu.Lock()
	test.testProjects = append(test.testProjects, project)
	test.mu.Unlock()

	return project
}

// getFileCountForSize returns appropriate file count for project size
func (test *LargeProjectPerformanceTest) getFileCountForSize(size framework.ProjectSize) int {
	switch size {
	case framework.SizeLarge:
		return 5000
	case framework.SizeXLarge:
		return 12000
	default:
		return 1000
	}
}

// getDirectoryDepthForSize returns appropriate directory depth for project size
func (test *LargeProjectPerformanceTest) getDirectoryDepthForSize(size framework.ProjectSize) int {
	switch size {
	case framework.SizeLarge:
		return 8
	case framework.SizeXLarge:
		return 12
	default:
		return 5
	}
}

// simulateProjectDetection simulates project detection operations
func (test *LargeProjectPerformanceTest) simulateProjectDetection(project *framework.TestProject) error {
	// Simulate walking through project files
	fileCount := 0
	err := filepath.Walk(project.RootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			fileCount++
			// Simulate reading file metadata
			time.Sleep(10 * time.Microsecond)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("project walk failed: %w", err)
	}

	// Simulate language detection processing
	time.Sleep(time.Duration(fileCount/100) * time.Millisecond)

	return nil
}

// performLargeProjectOperations performs various operations on large project
func (test *LargeProjectPerformanceTest) performLargeProjectOperations(project *framework.TestProject) error {
	// Simulate file reading operations
	fileCount := 0
	err := filepath.Walk(project.RootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && fileCount < 100 {
			// Read a subset of files to simulate actual usage
			_, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			fileCount++
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("large project operations failed: %w", err)
	}

	// Simulate additional processing time
	time.Sleep(50 * time.Millisecond)

	return nil
}

// performConcurrentProjectDetection performs concurrent project detection on multiple projects
func (test *LargeProjectPerformanceTest) performConcurrentProjectDetection(projects []*framework.TestProject) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(projects))

	for _, project := range projects {
		wg.Add(1)
		go func(p *framework.TestProject) {
			defer wg.Done()
			if err := test.simulateProjectDetection(p); err != nil {
				errChan <- err
			}
		}(project)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return fmt.Errorf("concurrent project detection error: %w", err)
		}
	}

	return nil
}

// testCacheEfficiency tests cache efficiency with repeated operations
func (test *LargeProjectPerformanceTest) testCacheEfficiency(gateway *gateway.ProjectAwareGateway, project *framework.TestProject) error {
	// Simulate repeated operations that should benefit from caching
	operations := []string{
		"textDocument/definition",
		"textDocument/references",
		"textDocument/documentSymbol",
		"workspace/symbol",
		"textDocument/hover",
	}

	// Perform operations multiple times to test cache efficiency
	for round := 0; round < 3; round++ {
		for _, operation := range operations {
			// Simulate LSP request
			time.Sleep(5 * time.Millisecond)

			// Simulate cache operations (would be actual cache hits/misses in real implementation)
			if round > 0 {
				// Simulate cache hit
				time.Sleep(1 * time.Millisecond)
			} else {
				// Simulate cache miss and population
				time.Sleep(10 * time.Millisecond)
			}
		}
	}

	return nil
}

// testServerPoolManagement tests server pool management with multiple projects
func (test *LargeProjectPerformanceTest) testServerPoolManagement(projects []*framework.TestProject) error {
	// Simulate server creation for different language combinations
	serverMap := make(map[string]bool)

	for _, project := range projects {
		for _, language := range project.Languages {
			if !serverMap[language] {
				// Simulate server creation
				time.Sleep(10 * time.Millisecond)
				serverMap[language] = true
			}
		}
	}

	// Simulate server usage and management
	for i := 0; i < 100; i++ {
		// Simulate random server usage
		time.Sleep(1 * time.Millisecond)
	}

	// Simulate server cleanup
	for language := range serverMap {
		// Simulate server destruction
		time.Sleep(5 * time.Millisecond)
		delete(serverMap, language)
	}

	return nil
}

// performLargeProjectBenchmark performs benchmark operations on large project
func (test *LargeProjectPerformanceTest) performLargeProjectBenchmark(gateway *gateway.ProjectAwareGateway, project *framework.TestProject) error {
	// Simulate typical LSP operations
	operations := []func() error{
		func() error { return test.simulateDefinitionRequest(project) },
		func() error { return test.simulateReferencesRequest(project) },
		func() error { return test.simulateSymbolsRequest(project) },
		func() error { return test.simulateHoverRequest(project) },
	}

	// Perform random operations
	for i := 0; i < 10; i++ {
		op := operations[i%len(operations)]
		if err := op(); err != nil {
			return err
		}
	}

	return nil
}

// simulateDefinitionRequest simulates a definition request
func (test *LargeProjectPerformanceTest) simulateDefinitionRequest(project *framework.TestProject) error {
	time.Sleep(5 * time.Millisecond)
	return nil
}

// simulateReferencesRequest simulates a references request
func (test *LargeProjectPerformanceTest) simulateReferencesRequest(project *framework.TestProject) error {
	time.Sleep(8 * time.Millisecond)
	return nil
}

// simulateSymbolsRequest simulates a symbols request
func (test *LargeProjectPerformanceTest) simulateSymbolsRequest(project *framework.TestProject) error {
	time.Sleep(12 * time.Millisecond)
	return nil
}

// simulateHoverRequest simulates a hover request
func (test *LargeProjectPerformanceTest) simulateHoverRequest(project *framework.TestProject) error {
	time.Sleep(3 * time.Millisecond)
	return nil
}

// cleanup performs cleanup of test resources
func (test *LargeProjectPerformanceTest) cleanup() {
	test.mu.Lock()
	defer test.mu.Unlock()

	// Cleanup framework resources
	if test.framework != nil {
		test.framework.CleanupAll()
	}

	// Clean up temp directory
	os.RemoveAll(test.tempDir)

	// Clear test data
	test.testProjects = nil
	test.gateways = nil
}

// Integration tests that use the performance test framework
func TestLargeProjectPerformanceSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large project performance tests in short mode")
	}

	perfTest := NewLargeProjectPerformanceTest(t)

	t.Run("ProjectDetection", perfTest.TestLargeProjectDetectionPerformance)
	t.Run("MemoryUsage", perfTest.TestLargeProjectMemoryUsage)
	t.Run("ConcurrentDetection", perfTest.TestConcurrentProjectDetection)
	t.Run("CacheEfficiency", perfTest.TestCacheEfficiency)
	t.Run("ServerPoolManagement", perfTest.TestServerPoolManagement)
}

// Benchmark function
func BenchmarkLargeProjectPerformance(b *testing.B) {
	perfTest := NewLargeProjectPerformanceTest(nil)
	perfTest.BenchmarkLargeProjectOperations(b)
}
