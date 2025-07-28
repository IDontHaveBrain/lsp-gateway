package e2e_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/tests/e2e/helpers"
	"lsp-gateway/tests/e2e/testutils"
)

type LSPPerformanceTestSuite struct {
	suite.Suite
	httpClient   *testutils.HttpClient
	assertHelper *e2e_test.AssertionHelper
	gatewayCmd   *exec.Cmd
	gatewayPort  int
	configPath   string
	tempDir      string
	projectRoot  string
	testTimeout  time.Duration
}

type PerformanceMetrics struct {
	TotalRequests   int
	SuccessfulReqs  int
	FailedRequests  int
	AverageLatency  time.Duration
	MinLatency      time.Duration
	MaxLatency      time.Duration
	Throughput      float64 // requests per second
	ConcurrentUsers int
	TestDuration    time.Duration
	MemoryUsage     int64   // bytes
	CPUUsage        float64 // percentage
}

type BenchmarkResult struct {
	Method       string
	Metrics      PerformanceMetrics
	Percentiles  map[int]time.Duration // 50th, 90th, 95th, 99th percentiles
	ErrorRate    float64
	CacheHitRate float64
}

func (suite *LSPPerformanceTestSuite) SetupSuite() {
	suite.testTimeout = 120 * time.Second // Longer timeout for performance tests

	var err error
	suite.projectRoot, err = testutils.GetProjectRoot()
	suite.Require().NoError(err)

	suite.tempDir, err = os.MkdirTemp("", "lsp-performance-test-*")
	suite.Require().NoError(err)

	suite.gatewayPort, err = testutils.FindAvailablePort()
	suite.Require().NoError(err)

	suite.assertHelper = e2e_test.NewAssertionHelper(suite.T())

	suite.createTestConfig()
	suite.setupPerformanceTestFiles()
}

func (suite *LSPPerformanceTestSuite) SetupTest() {
	config := testutils.HttpClientConfig{
		BaseURL:            fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
		Timeout:            30 * time.Second,
		MaxRetries:         2,
		RetryDelay:         100 * time.Millisecond,
		EnableLogging:      false, // Disable logging for performance tests
		EnableRecording:    false, // Disable recording for performance tests
		WorkspaceID:        "perf-test-workspace",
		ProjectPath:        suite.tempDir,
		ConnectionPoolSize: 50, // Larger pool for performance testing
		KeepAlive:          60 * time.Second,
	}
	suite.httpClient = testutils.NewHttpClient(config)
}

func (suite *LSPPerformanceTestSuite) TearDownTest() {
	if suite.httpClient != nil {
		suite.httpClient.Close()
	}
}

func (suite *LSPPerformanceTestSuite) TearDownSuite() {
	if suite.gatewayCmd != nil && suite.gatewayCmd.Process != nil {
		suite.gatewayCmd.Process.Signal(syscall.SIGTERM)
		suite.gatewayCmd.Wait()
	}
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
}

func (suite *LSPPerformanceTestSuite) createTestConfig() {
	configContent := fmt.Sprintf(`
servers:
- name: go-lsp
  languages:
  - go
  command: gopls
  args: []
  transport: stdio
  root_markers:
  - go.mod
  priority: 1
  weight: 1.0
port: %d
timeout: 30s
max_concurrent_requests: 200
cache:
  enabled: true
  max_size: 1000
  ttl: 300s
`, suite.gatewayPort)

	var err error
	suite.configPath, _, err = testutils.CreateTempConfig(configContent)
	suite.Require().NoError(err)
}

func (suite *LSPPerformanceTestSuite) setupPerformanceTestFiles() {
	// Create multiple Go files of varying complexity for performance testing
	files := map[string]string{
		"simple.go": `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}`,
		"complex.go":    suite.generateComplexGoFile(100),
		"large.go":      suite.generateComplexGoFile(500),
		"structs.go":    suite.generateStructsFile(50),
		"interfaces.go": suite.generateInterfacesFile(20),
	}

	for filename, content := range files {
		filePath := filepath.Join(suite.tempDir, filename)
		err := os.WriteFile(filePath, []byte(content), 0644)
		suite.Require().NoError(err)
	}

	// Create go.mod file
	goModContent := "module perftest\n\ngo 1.21\n"
	goModFile := filepath.Join(suite.tempDir, "go.mod")
	err := os.WriteFile(goModFile, []byte(goModContent), 0644)
	suite.Require().NoError(err)
}

func (suite *LSPPerformanceTestSuite) generateComplexGoFile(numFunctions int) string {
	content := "package main\n\nimport (\n\t\"fmt\"\n\t\"time\"\n\t\"context\"\n)\n\n"

	// Add structs
	content += "type ComplexStruct struct {\n"
	for i := 0; i < 10; i++ {
		content += fmt.Sprintf("\tField%d string\n", i)
	}
	content += "}\n\n"

	// Add interface
	content += "type ComplexInterface interface {\n"
	for i := 0; i < 5; i++ {
		content += fmt.Sprintf("\tMethod%d() error\n", i)
	}
	content += "}\n\n"

	// Add methods
	for i := 0; i < numFunctions; i++ {
		content += fmt.Sprintf(`func (c *ComplexStruct) Method%d(ctx context.Context, param%d string) error {
	fmt.Printf("Executing method %%d with param %%s\n", %d, param%d)
	time.Sleep(time.Microsecond)
	return nil
}

`, i, i, i, i)
	}

	// Add main function
	content += "func main() {\n\tc := &ComplexStruct{}\n"
	for i := 0; i < 5; i++ {
		content += fmt.Sprintf("\tc.Method%d(context.Background(), \"test%d\")\n", i, i)
	}
	content += "}\n"

	return content
}

func (suite *LSPPerformanceTestSuite) generateStructsFile(numStructs int) string {
	content := "package main\n\n"

	for i := 0; i < numStructs; i++ {
		content += fmt.Sprintf("type Struct%d struct {\n", i)
		for j := 0; j < 5; j++ {
			content += fmt.Sprintf("\tField%d_%d string\n", i, j)
		}
		content += "}\n\n"
	}

	return content
}

func (suite *LSPPerformanceTestSuite) generateInterfacesFile(numInterfaces int) string {
	content := "package main\n\n"

	for i := 0; i < numInterfaces; i++ {
		content += fmt.Sprintf("type Interface%d interface {\n", i)
		for j := 0; j < 3; j++ {
			content += fmt.Sprintf("\tMethod%d_%d() error\n", i, j)
		}
		content += "}\n\n"
	}

	return content
}

func (suite *LSPPerformanceTestSuite) startGatewayServer() {
	binaryPath := filepath.Join(suite.projectRoot, "bin", "lsp-gateway")
	suite.gatewayCmd = exec.Command(binaryPath, "server", "--config", suite.configPath)
	suite.gatewayCmd.Dir = suite.projectRoot

	err := suite.gatewayCmd.Start()
	suite.Require().NoError(err)

	time.Sleep(5 * time.Second) // Longer startup time for performance tests
}

func (suite *LSPPerformanceTestSuite) stopGatewayServer() {
	if suite.gatewayCmd != nil && suite.gatewayCmd.Process != nil {
		suite.gatewayCmd.Process.Signal(syscall.SIGTERM)
		suite.gatewayCmd.Wait()
		suite.gatewayCmd = nil
	}
}

func (suite *LSPPerformanceTestSuite) waitForServerReady(ctx context.Context) {
	suite.Eventually(func() bool {
		err := suite.httpClient.HealthCheck(ctx)
		return err == nil
	}, 30*time.Second, 500*time.Millisecond, "Server should become ready")
}

func (suite *LSPPerformanceTestSuite) BenchmarkLSPDefinition(b *testing.B) {
	suite.startGatewayServer()
	defer suite.stopGatewayServer()

	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	suite.waitForServerReady(ctx)

	testFile := fmt.Sprintf("file://%s/complex.go", suite.tempDir)
	position := testutils.Position{Line: 15, Character: 10}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := suite.httpClient.Definition(ctx, testFile, position)
		if err != nil {
			b.Logf("Definition request failed: %v", err)
		}
	}
}

func (suite *LSPPerformanceTestSuite) BenchmarkLSPReferences(b *testing.B) {
	suite.startGatewayServer()
	defer suite.stopGatewayServer()

	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	suite.waitForServerReady(ctx)

	testFile := fmt.Sprintf("file://%s/complex.go", suite.tempDir)
	position := testutils.Position{Line: 10, Character: 5}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := suite.httpClient.References(ctx, testFile, position, true)
		if err != nil {
			b.Logf("References request failed: %v", err)
		}
	}
}

func (suite *LSPPerformanceTestSuite) BenchmarkLSPHover(b *testing.B) {
	suite.startGatewayServer()
	defer suite.stopGatewayServer()

	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	suite.waitForServerReady(ctx)

	testFile := fmt.Sprintf("file://%s/complex.go", suite.tempDir)
	position := testutils.Position{Line: 12, Character: 8}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := suite.httpClient.Hover(ctx, testFile, position)
		if err != nil {
			b.Logf("Hover request failed: %v", err)
		}
	}
}

func (suite *LSPPerformanceTestSuite) BenchmarkLSPDocumentSymbol(b *testing.B) {
	suite.startGatewayServer()
	defer suite.stopGatewayServer()

	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	suite.waitForServerReady(ctx)

	testFile := fmt.Sprintf("file://%s/large.go", suite.tempDir)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := suite.httpClient.DocumentSymbol(ctx, testFile)
		if err != nil {
			b.Logf("DocumentSymbol request failed: %v", err)
		}
	}
}

func (suite *LSPPerformanceTestSuite) BenchmarkLSPWorkspaceSymbol(b *testing.B) {
	suite.startGatewayServer()
	defer suite.stopGatewayServer()

	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	suite.waitForServerReady(ctx)

	queries := []string{"Complex", "Method", "Struct", "Interface", "Field"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		query := queries[i%len(queries)]
		_, err := suite.httpClient.WorkspaceSymbol(ctx, query)
		if err != nil {
			b.Logf("WorkspaceSymbol request failed: %v", err)
		}
	}
}

func (suite *LSPPerformanceTestSuite) BenchmarkLSPCompletion(b *testing.B) {
	suite.startGatewayServer()
	defer suite.stopGatewayServer()

	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	suite.waitForServerReady(ctx)

	testFile := fmt.Sprintf("file://%s/complex.go", suite.tempDir)
	position := testutils.Position{Line: 20, Character: 5}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := suite.httpClient.Completion(ctx, testFile, position)
		if err != nil {
			b.Logf("Completion request failed: %v", err)
		}
	}
}

func (suite *LSPPerformanceTestSuite) TestConcurrentPerformance() {
	suite.T().Run("ConcurrentLSPRequests", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		concurrencyLevels := []int{1, 5, 10, 20, 50}
		requestsPerLevel := 100

		for _, concurrency := range concurrencyLevels {
			suite.T().Run(fmt.Sprintf("Concurrency_%d", concurrency), func(t *testing.T) {
				result := suite.runConcurrentPerformanceTest(ctx, concurrency, requestsPerLevel)

				suite.T().Logf("Concurrency %d results:", concurrency)
				suite.T().Logf("  Total requests: %d", result.Metrics.TotalRequests)
				suite.T().Logf("  Successful: %d", result.Metrics.SuccessfulReqs)
				suite.T().Logf("  Failed: %d", result.Metrics.FailedRequests)
				suite.T().Logf("  Error rate: %.2f%%", result.ErrorRate*100)
				suite.T().Logf("  Average latency: %v", result.Metrics.AverageLatency)
				suite.T().Logf("  Throughput: %.2f req/s", result.Metrics.Throughput)
				suite.T().Logf("  50th percentile: %v", result.Percentiles[50])
				suite.T().Logf("  95th percentile: %v", result.Percentiles[95])
				suite.T().Logf("  99th percentile: %v", result.Percentiles[99])

				// Performance assertions
				suite.LessOrEqual(result.ErrorRate, 0.05, "Error rate should be <= 5%")
				suite.Greater(result.Metrics.Throughput, float64(concurrency)*0.5, "Throughput should scale with concurrency")
				suite.Less(result.Percentiles[95], 5*time.Second, "95th percentile should be reasonable")
			})
		}
	})
}

func (suite *LSPPerformanceTestSuite) runConcurrentPerformanceTest(ctx context.Context, concurrency, requestsPerWorker int) BenchmarkResult {
	var wg sync.WaitGroup
	var mu sync.Mutex

	latencies := make([]time.Duration, 0, concurrency*requestsPerWorker)
	successCount := 0
	errorCount := 0

	testFile := fmt.Sprintf("file://%s/complex.go", suite.tempDir)

	startTime := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < requestsPerWorker; j++ {
				requestStart := time.Now()

				// Rotate through different LSP methods
				var err error
				switch (workerID*requestsPerWorker + j) % 6 {
				case 0:
					_, err = suite.httpClient.Definition(ctx, testFile, testutils.Position{Line: 15, Character: 10})
				case 1:
					_, err = suite.httpClient.References(ctx, testFile, testutils.Position{Line: 10, Character: 5}, true)
				case 2:
					_, err = suite.httpClient.Hover(ctx, testFile, testutils.Position{Line: 12, Character: 8})
				case 3:
					_, err = suite.httpClient.DocumentSymbol(ctx, testFile)
				case 4:
					_, err = suite.httpClient.WorkspaceSymbol(ctx, "Complex")
				case 5:
					_, err = suite.httpClient.Completion(ctx, testFile, testutils.Position{Line: 20, Character: 5})
				}

				latency := time.Since(requestStart)

				mu.Lock()
				latencies = append(latencies, latency)
				if err != nil {
					errorCount++
				} else {
					successCount++
				}
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	// Calculate metrics
	totalRequests := len(latencies)

	// Sort latencies for percentile calculations
	suite.sortDurations(latencies)

	var totalLatency time.Duration
	minLatency := latencies[0]
	maxLatency := latencies[len(latencies)-1]

	for _, latency := range latencies {
		totalLatency += latency
	}

	avgLatency := totalLatency / time.Duration(totalRequests)
	throughput := float64(totalRequests) / totalDuration.Seconds()

	// Calculate percentiles
	percentiles := map[int]time.Duration{
		50: latencies[totalRequests*50/100],
		90: latencies[totalRequests*90/100],
		95: latencies[totalRequests*95/100],
		99: latencies[totalRequests*99/100],
	}

	return BenchmarkResult{
		Method: "Mixed",
		Metrics: PerformanceMetrics{
			TotalRequests:   totalRequests,
			SuccessfulReqs:  successCount,
			FailedRequests:  errorCount,
			AverageLatency:  avgLatency,
			MinLatency:      minLatency,
			MaxLatency:      maxLatency,
			Throughput:      throughput,
			ConcurrentUsers: concurrency,
			TestDuration:    totalDuration,
		},
		Percentiles: percentiles,
		ErrorRate:   float64(errorCount) / float64(totalRequests),
	}
}

func (suite *LSPPerformanceTestSuite) sortDurations(durations []time.Duration) {
	// Simple bubble sort for durations
	n := len(durations)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if durations[j] > durations[j+1] {
				durations[j], durations[j+1] = durations[j+1], durations[j]
			}
		}
	}
}

func (suite *LSPPerformanceTestSuite) TestMemoryUsage() {
	suite.T().Run("MemoryUsageAnalysis", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		// Record memory usage before test
		var initialMemStats, finalMemStats runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&initialMemStats)

		// Run a series of requests
		testFile := fmt.Sprintf("file://%s/large.go", suite.tempDir)
		numRequests := 1000

		for i := 0; i < numRequests; i++ {
			// Rotate through different methods to stress test memory
			switch i % 6 {
			case 0:
				suite.httpClient.Definition(ctx, testFile, testutils.Position{Line: i%100 + 1, Character: 5})
			case 1:
				suite.httpClient.References(ctx, testFile, testutils.Position{Line: i%100 + 1, Character: 5}, true)
			case 2:
				suite.httpClient.Hover(ctx, testFile, testutils.Position{Line: i%100 + 1, Character: 5})
			case 3:
				suite.httpClient.DocumentSymbol(ctx, testFile)
			case 4:
				suite.httpClient.WorkspaceSymbol(ctx, fmt.Sprintf("Method%d", i%50))
			case 5:
				suite.httpClient.Completion(ctx, testFile, testutils.Position{Line: i%100 + 1, Character: 5})
			}

			// Force GC every 100 requests to get accurate memory measurements
			if i%100 == 0 {
				runtime.GC()
			}
		}

		// Record memory usage after test
		runtime.GC()
		runtime.ReadMemStats(&finalMemStats)

		memoryGrowth := finalMemStats.Alloc - initialMemStats.Alloc
		suite.T().Logf("Memory usage analysis:")
		suite.T().Logf("  Initial allocated: %d bytes", initialMemStats.Alloc)
		suite.T().Logf("  Final allocated: %d bytes", finalMemStats.Alloc)
		suite.T().Logf("  Memory growth: %d bytes", memoryGrowth)
		suite.T().Logf("  Memory growth per request: %d bytes", int64(memoryGrowth)/int64(numRequests))
		suite.T().Logf("  Total mallocs: %d", finalMemStats.Mallocs-initialMemStats.Mallocs)
		suite.T().Logf("  GC cycles: %d", finalMemStats.NumGC-initialMemStats.NumGC)

		// Assertions for memory usage
		memoryGrowthPerRequest := int64(memoryGrowth) / int64(numRequests)
		suite.Less(memoryGrowthPerRequest, int64(10*1024), "Memory growth per request should be < 10KB")
	})
}

func (suite *LSPPerformanceTestSuite) TestThroughputScaling() {
	suite.T().Run("ThroughputScaling", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		// Test different file sizes and complexities
		testCases := []struct {
			name     string
			filename string
			requests int
		}{
			{"SimpleFile", "simple.go", 200},
			{"ComplexFile", "complex.go", 100},
			{"LargeFile", "large.go", 50},
		}

		for _, testCase := range testCases {
			suite.T().Run(testCase.name, func(t *testing.T) {
				testFile := fmt.Sprintf("file://%s/%s", suite.tempDir, testCase.filename)

				suite.httpClient.ClearMetrics()
				startTime := time.Now()

				// Execute requests sequentially to measure throughput
				successCount := 0
				for i := 0; i < testCase.requests; i++ {
					_, err := suite.httpClient.Definition(ctx, testFile, testutils.Position{Line: 10, Character: 5})
					if err == nil {
						successCount++
					}
				}

				duration := time.Since(startTime)
				throughput := float64(successCount) / duration.Seconds()

				suite.T().Logf("%s throughput results:", testCase.name)
				suite.T().Logf("  Requests: %d", testCase.requests)
				suite.T().Logf("  Successful: %d", successCount)
				suite.T().Logf("  Duration: %v", duration)
				suite.T().Logf("  Throughput: %.2f req/s", throughput)
				suite.T().Logf("  Average latency: %v", duration/time.Duration(successCount))

				// Basic throughput expectations
				suite.Greater(throughput, 1.0, "Should achieve at least 1 req/s throughput")
				suite.GreaterOrEqual(float64(successCount)/float64(testCase.requests), 0.95, "Should have 95%+ success rate")
			})
		}
	})
}

func TestLSPPerformanceTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping LSP performance tests in short mode")
	}
	suite.Run(t, new(LSPPerformanceTestSuite))
}

// Additional benchmark functions for Go's built-in benchmark system

func BenchmarkLSPDefinitionSingle(b *testing.B) {
	suite := &LSPPerformanceTestSuite{}
	suite.Suite.SetT(&testing.T{})

	// Quick setup
	suite.projectRoot, _ = testutils.GetProjectRoot()
	suite.tempDir, _ = os.MkdirTemp("", "bench-*")
	defer os.RemoveAll(suite.tempDir)

	suite.gatewayPort, _ = testutils.FindAvailablePort()
	suite.createTestConfig()
	suite.setupPerformanceTestFiles()

	config := testutils.DefaultHttpClientConfig()
	config.BaseURL = fmt.Sprintf("http://localhost:%d", suite.gatewayPort)
	config.EnableLogging = false
	config.EnableRecording = false
	suite.httpClient = testutils.NewHttpClient(config)
	defer suite.httpClient.Close()

	suite.BenchmarkLSPDefinition(b)
}

func BenchmarkLSPReferencesSingle(b *testing.B) {
	suite := &LSPPerformanceTestSuite{}
	suite.Suite.SetT(&testing.T{})

	// Quick setup
	suite.projectRoot, _ = testutils.GetProjectRoot()
	suite.tempDir, _ = os.MkdirTemp("", "bench-*")
	defer os.RemoveAll(suite.tempDir)

	suite.gatewayPort, _ = testutils.FindAvailablePort()
	suite.createTestConfig()
	suite.setupPerformanceTestFiles()

	config := testutils.DefaultHttpClientConfig()
	config.BaseURL = fmt.Sprintf("http://localhost:%d", suite.gatewayPort)
	config.EnableLogging = false
	config.EnableRecording = false
	suite.httpClient = testutils.NewHttpClient(config)
	defer suite.httpClient.Close()

	suite.BenchmarkLSPReferences(b)
}
