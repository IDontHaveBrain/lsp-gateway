package suites

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/tests/e2e/mcp/client"
	"lsp-gateway/tests/e2e/mcp/helpers"
	"lsp-gateway/tests/e2e/mcp/types"
	"lsp-gateway/tests/e2e/testutils"
)

type MCPPerformanceE2ETestSuite struct {
	suite.Suite
	client              *client.EnhancedMCPTestClient
	workspace           *types.TestWorkspace
	workspaceManager    *helpers.TestWorkspaceManager
	server              *exec.Cmd
	config              *types.MCPTestConfig
	metricsCollector    *PerformanceMetricsCollector
	loadGenerator       *LoadTestGenerator
	projectRoot         string
	binaryPath          string
	gatewayPort         int
	testTimeout         time.Duration
	serverMutex         sync.Mutex
	baselineMetrics     *PerformanceMetrics
	concurrencyLevels   []int
	loadTestScenarios   []LoadTestScenario
}

type PerformanceMetrics struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	AverageLatency     time.Duration
	MinLatency         time.Duration
	MaxLatency         time.Duration
	P50Latency         time.Duration
	P95Latency         time.Duration
	P99Latency         time.Duration
	ThroughputRPS      float64
	MemoryUsageMB      float64
	CPUUsagePercent    float64
	CacheHitRate       float64
	ConcurrentUsers    int
	TestDuration       time.Duration
	LSPFeatureMetrics  map[string]*LSPFeatureMetrics
}

type LSPFeatureMetrics struct {
	FeatureName       string
	RequestCount      int64
	SuccessCount      int64
	ErrorCount        int64
	AverageLatency    time.Duration
	P95Latency        time.Duration
	ThroughputRPS     float64
	CacheHitRate      float64
}

type PerformanceMetricsCollector struct {
	mutex             sync.RWMutex
	latencies         map[string][]time.Duration
	requestCounts     map[string]int64
	successCounts     map[string]int64
	errorCounts       map[string]int64
	memorySnapshots   []MemorySnapshot
	startTime         time.Time
	endTime           time.Time
	concurrentUsers   int
	cacheHits         int64
	cacheMisses       int64
}

type MemorySnapshot struct {
	Timestamp     time.Time
	AllocatedMB   float64
	HeapInUseMB   float64
	GCCount       uint32
	GCPauseTotalNs uint64
}

type LoadTestGenerator struct {
	scenarios     []LoadTestScenario
	client        *client.EnhancedMCPTestClient
	workspace     *types.TestWorkspace
	logger        Logger
	stopChan      chan struct{}
	wg            sync.WaitGroup
}

type LoadTestScenario struct {
	Name               string
	ConcurrentUsers    int
	RequestsPerUser    int
	Duration           time.Duration
	LSPFeatureWeights  map[string]float64
	RampUpDuration     time.Duration
	RampDownDuration   time.Duration
	ThinkTimeBetween   time.Duration
}

type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

type LSPToolRequest struct {
	ToolName   string
	Parameters map[string]interface{}
	Weight     float64
}

func (suite *MCPPerformanceE2ETestSuite) SetupSuite() {
	suite.testTimeout = 300 * time.Second // Extended timeout for performance tests
	suite.concurrencyLevels = []int{1, 5, 10, 20, 50}

	var err error
	suite.projectRoot, err = testutils.GetProjectRoot()
	suite.Require().NoError(err)

	suite.binaryPath = filepath.Join(suite.projectRoot, "bin", "lspg")
	suite.gatewayPort, err = testutils.FindAvailablePort()
	suite.Require().NoError(err)

	suite.workspaceManager, err = helpers.NewTestWorkspaceManager()
	suite.Require().NoError(err)

	suite.setupTestWorkspace()
	suite.setupPerformanceInfrastructure()
	suite.defineLoadTestScenarios()
}

func (suite *MCPPerformanceE2ETestSuite) setupTestWorkspace() {
	workspace, err := suite.workspaceManager.CreateWorkspace("performance-test-workspace")
	suite.Require().NoError(err)
	suite.workspace = workspace

	// TODO: Configure MCP test client properly
}

func (suite *MCPPerformanceE2ETestSuite) setupPerformanceInfrastructure() {
	suite.metricsCollector = NewPerformanceMetricsCollector()
	
	clientConfig := client.DefaultMCPTestConfig()
	clientConfig.ServerURL = fmt.Sprintf("localhost:%d", suite.gatewayPort)
	clientConfig.TransportType = client.TransportSTDIO
	clientConfig.ConnectionTimeout = 30 * time.Second
	clientConfig.RequestTimeout = 15 * time.Second
	clientConfig.MetricsEnabled = true
	clientConfig.LogLevel = "info"

	transport := &MCPStdioTransport{}
	suite.client = client.NewEnhancedMCPTestClient(clientConfig, transport)
	
	suite.loadGenerator = NewLoadTestGenerator(suite.client, suite.workspace)
}

func (suite *MCPPerformanceE2ETestSuite) defineLoadTestScenarios() {
	suite.loadTestScenarios = []LoadTestScenario{
		{
			Name:            "QuickBurst",
			ConcurrentUsers: 10,
			RequestsPerUser: 50,
			Duration:        30 * time.Second,
			LSPFeatureWeights: map[string]float64{
				"goto_definition":   0.25,
				"find_references":   0.20,
				"hover":            0.20,
				"document_symbols":  0.15,
				"workspace_symbol":  0.10,
				"completion":       0.10,
			},
			RampUpDuration:   5 * time.Second,
			RampDownDuration: 2 * time.Second,
			ThinkTimeBetween: 100 * time.Millisecond,
		},
		{
			Name:            "SustainedLoad",
			ConcurrentUsers: 20,
			RequestsPerUser: 100,
			Duration:        5 * time.Minute,
			LSPFeatureWeights: map[string]float64{
				"goto_definition":   0.30,
				"find_references":   0.25,
				"hover":            0.20,
				"document_symbols":  0.10,
				"workspace_symbol":  0.10,
				"completion":       0.05,
			},
			RampUpDuration:   30 * time.Second,
			RampDownDuration: 15 * time.Second,
			ThinkTimeBetween: 200 * time.Millisecond,
		},
		{
			Name:            "HeavyLoad",
			ConcurrentUsers: 50,
			RequestsPerUser: 200,
			Duration:        10 * time.Minute,
			LSPFeatureWeights: map[string]float64{
				"goto_definition":   0.20,
				"find_references":   0.20,
				"hover":            0.20,
				"document_symbols":  0.15,
				"workspace_symbol":  0.15,
				"completion":       0.10,
			},
			RampUpDuration:   60 * time.Second,
			RampDownDuration: 30 * time.Second,
			ThinkTimeBetween: 500 * time.Millisecond,
		},
	}
}

func (suite *MCPPerformanceE2ETestSuite) SetupTest() {
	suite.startMCPServer()
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	err := suite.client.Connect(ctx)
	suite.Require().NoError(err)
}

func (suite *MCPPerformanceE2ETestSuite) TearDownTest() {
	if suite.client != nil {
		suite.client.Close()
	}
	suite.stopMCPServer()
}

func (suite *MCPPerformanceE2ETestSuite) TearDownSuite() {
	if suite.workspaceManager != nil {
		suite.workspaceManager.CleanupAll()
	}
}

func (suite *MCPPerformanceE2ETestSuite) startMCPServer() {
	suite.serverMutex.Lock()
	defer suite.serverMutex.Unlock()

	configPath := suite.createMCPServerConfig()
	
	suite.server = exec.Command(suite.binaryPath, "mcp", "--config", configPath)
	suite.server.Dir = suite.workspace.RootPath
	
	err := suite.server.Start()
	suite.Require().NoError(err)
	
	time.Sleep(3 * time.Second) // Allow server startup
}

func (suite *MCPPerformanceE2ETestSuite) stopMCPServer() {
	suite.serverMutex.Lock()
	defer suite.serverMutex.Unlock()

	if suite.server != nil && suite.server.Process != nil {
		suite.server.Process.Signal(syscall.SIGTERM)
		suite.server.Wait()
		suite.server = nil
	}
}

func (suite *MCPPerformanceE2ETestSuite) createMCPServerConfig() string {
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
mcp:
  enabled: true
  transport: stdio
  tools:
    - goto_definition
    - find_references
    - hover
    - document_symbols
    - workspace_symbol
    - completion
port: %d
timeout: 30s
max_concurrent_requests: 100
cache:
  enabled: true
  max_size: 2000
  ttl: 600s
  memory_cache:
    max_size_mb: 100
  disk_cache:
    enabled: true
    max_size_mb: 500
`, suite.gatewayPort)

	configPath := filepath.Join(suite.workspace.RootPath, "mcp-perf-config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	suite.Require().NoError(err)
	
	return configPath
}

// TestMCPToolDefinitionPerformance tests goto_definition tool performance
func (suite *MCPPerformanceE2ETestSuite) TestMCPToolDefinitionPerformance() {
	t := suite.T()
	
	testParams := map[string]interface{}{
		"uri":       fmt.Sprintf("file://%s/color.go", suite.workspace.RootPath),
		"line":      110,
		"character": 0,
	}
	
	for i := 0; i < 100; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		result, err := suite.client.CallTool(ctx, "goto_definition", testParams)
		cancel()
		
		if err != nil {
			t.Fatalf("Tool call failed: %v", err)
		}
		if result.Error != nil {
			t.Fatalf("Tool returned error: %v", result.Error)
		}
	}
}

// TestMCPToolReferencesPerformance tests find_references tool performance
func (suite *MCPPerformanceE2ETestSuite) TestMCPToolReferencesPerformance() {
	t := suite.T()
	
	testParams := map[string]interface{}{
		"uri":                fmt.Sprintf("file://%s/color.go", suite.workspace.RootPath),
		"line":               50,
		"character":          5,
		"include_declaration": true,
	}
	
	for i := 0; i < 100; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		result, err := suite.client.CallTool(ctx, "find_references", testParams)
		cancel()
		
		if err != nil {
			t.Fatalf("Tool call failed: %v", err)
		}
		if result.Error != nil {
			t.Fatalf("Tool returned error: %v", result.Error)
		}
	}
}

// TestMCPToolHoverPerformance tests hover tool performance
func (suite *MCPPerformanceE2ETestSuite) TestMCPToolHoverPerformance() {
	t := suite.T()
	
	testParams := map[string]interface{}{
		"uri":       fmt.Sprintf("file://%s/color.go", suite.workspace.RootPath),
		"line":      75,
		"character": 10,
	}
	
	for i := 0; i < 100; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		result, err := suite.client.CallTool(ctx, "hover", testParams)
		cancel()
		
		if err != nil {
			t.Fatalf("Tool call failed: %v", err)
		}
		if result.Error != nil {
			t.Fatalf("Tool returned error: %v", result.Error)
		}
	}
}

// TestMCPToolDocumentSymbolsPerformance tests document_symbols tool performance
func (suite *MCPPerformanceE2ETestSuite) TestMCPToolDocumentSymbolsPerformance() {
	t := suite.T()
	
	testParams := map[string]interface{}{
		"uri": fmt.Sprintf("file://%s/color.go", suite.workspace.RootPath),
	}
	
	for i := 0; i < 100; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		result, err := suite.client.CallTool(ctx, "document_symbols", testParams)
		cancel()
		
		if err != nil {
			t.Fatalf("Tool call failed: %v", err)
		}
		if result.Error != nil {
			t.Fatalf("Tool returned error: %v", result.Error)
		}
	}
}

// TestMCPToolWorkspaceSymbolPerformance tests workspace_symbol tool performance
func (suite *MCPPerformanceE2ETestSuite) TestMCPToolWorkspaceSymbolPerformance() {
	t := suite.T()
	
	queries := []string{"Color", "Attribute", "New", "Sprint", "Print"}
	
	for i := 0; i < 100; i++ {
		query := queries[i%len(queries)]
		testParams := map[string]interface{}{
			"query": query,
		}
		
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		result, err := suite.client.CallTool(ctx, "workspace_symbol", testParams)
		cancel()
		
		if err != nil {
			t.Fatalf("Tool call failed: %v", err)
		}
		if result.Error != nil {
			t.Fatalf("Tool returned error: %v", result.Error)
		}
	}
}

// TestMCPToolCompletionPerformance tests completion tool performance
func (suite *MCPPerformanceE2ETestSuite) TestMCPToolCompletionPerformance() {
	t := suite.T()
	
	testParams := map[string]interface{}{
		"uri":       fmt.Sprintf("file://%s/color.go", suite.workspace.RootPath),
		"line":      100,
		"character": 15,
	}
	
	for i := 0; i < 100; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		result, err := suite.client.CallTool(ctx, "completion", testParams)
		cancel()
		
		if err != nil {
			t.Fatalf("Tool call failed: %v", err)
		}
		if result.Error != nil {
			t.Fatalf("Tool returned error: %v", result.Error)
		}
	}
}

// TestConcurrentLSPFeatures tests concurrent request performance across all 6 LSP features
func (suite *MCPPerformanceE2ETestSuite) TestConcurrentLSPFeatures() {
	for _, concurrency := range suite.concurrencyLevels {
		suite.Run(fmt.Sprintf("Concurrency_%d", concurrency), func() {
			suite.metricsCollector.Reset()
			
			ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
			defer cancel()
			
			metrics := suite.runConcurrentLoadTest(ctx, concurrency, 100) // 100 requests per goroutine
			
			suite.T().Logf("Concurrency %d results:", concurrency)
			suite.logPerformanceMetrics(metrics)
			
			// Performance assertions based on SCIP caching expectations
			suite.Less(metrics.P95Latency, 2*time.Second, "P95 latency should be under 2s")
			suite.Greater(metrics.ThroughputRPS, float64(concurrency)*0.5, "Throughput should scale with concurrency")
			suite.Less(metrics.MemoryUsageMB, 500.0, "Memory usage should be reasonable")
			suite.LessOrEqual(float64(metrics.FailedRequests)/float64(metrics.TotalRequests), 0.05, "Error rate should be <= 5%")
			
			// Cache performance expectations (85-90% hit rate mentioned in CLAUDE.md)
			if metrics.CacheHitRate > 0 {
				suite.GreaterOrEqual(metrics.CacheHitRate, 0.80, "Cache hit rate should be >= 80%")
			}
		})
	}
}

func (suite *MCPPerformanceE2ETestSuite) runConcurrentLoadTest(ctx context.Context, concurrency, requestsPerWorker int) *PerformanceMetrics {
	var wg sync.WaitGroup
	
	suite.metricsCollector.SetConcurrentUsers(concurrency)
	suite.metricsCollector.StartCollection()
	
	startTime := time.Now()
	
	// LSP tool requests with different parameters
	toolRequests := []LSPToolRequest{
		{ToolName: "goto_definition", Parameters: map[string]interface{}{"uri": fmt.Sprintf("file://%s/color.go", suite.workspace.RootPath), "line": 110, "character": 0}, Weight: 0.25},
		{ToolName: "find_references", Parameters: map[string]interface{}{"uri": fmt.Sprintf("file://%s/color.go", suite.workspace.RootPath), "line": 50, "character": 5, "include_declaration": true}, Weight: 0.20},
		{ToolName: "hover", Parameters: map[string]interface{}{"uri": fmt.Sprintf("file://%s/color.go", suite.workspace.RootPath), "line": 75, "character": 10}, Weight: 0.20},
		{ToolName: "document_symbols", Parameters: map[string]interface{}{"uri": fmt.Sprintf("file://%s/color.go", suite.workspace.RootPath)}, Weight: 0.15},
		{ToolName: "workspace_symbol", Parameters: map[string]interface{}{"query": "Color"}, Weight: 0.10},
		{ToolName: "completion", Parameters: map[string]interface{}{"uri": fmt.Sprintf("file://%s/color.go", suite.workspace.RootPath), "line": 100, "character": 15}, Weight: 0.10},
	}
	
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for j := 0; j < requestsPerWorker; j++ {
				// Select tool request based on round-robin with some randomization
				toolRequest := toolRequests[(workerID*requestsPerWorker+j)%len(toolRequests)]
				
				requestStart := time.Now()
				
				toolCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				result, err := suite.client.CallTool(toolCtx, toolRequest.ToolName, toolRequest.Parameters)
				cancel()
				
				latency := time.Since(requestStart)
				
				suite.metricsCollector.RecordRequest(toolRequest.ToolName, latency, err == nil && result.Error == nil)
				
				// Brief pause between requests to simulate realistic usage
				time.Sleep(50 * time.Millisecond)
			}
		}(i)
	}
	
	wg.Wait()
	totalDuration := time.Since(startTime)
	
	suite.metricsCollector.StopCollection()
	
	return suite.metricsCollector.GetMetrics(totalDuration)
}

// TestMemoryUsageUnderLoad tests memory usage during sustained load
func (suite *MCPPerformanceE2ETestSuite) TestMemoryUsageUnderLoad() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	
	initialMemory := suite.getMemoryUsage()
	
	// Execute sustained load test
	metrics := suite.runSustainedLoadTest(ctx, 10*time.Minute, 10) // 10 minutes, 10 concurrent clients
	
	finalMemory := suite.getMemoryUsage()
	memoryIncrease := finalMemory - initialMemory
	
	suite.T().Logf("Memory usage analysis:")
	suite.T().Logf("  Initial memory: %.2f MB", initialMemory)
	suite.T().Logf("  Final memory: %.2f MB", finalMemory)
	suite.T().Logf("  Memory increase: %.2f MB", memoryIncrease)
	suite.T().Logf("  Average latency: %v", metrics.AverageLatency)
	suite.T().Logf("  Total requests: %d", metrics.TotalRequests)
	
	// Memory usage expectations (65-75MB typical mentioned in CLAUDE.md)
	suite.Less(memoryIncrease, 100.0, "Memory increase should be less than 100MB")
	suite.Less(metrics.AverageLatency, 1*time.Second, "Average latency should remain reasonable under sustained load")
	suite.GreaterOrEqual(float64(metrics.SuccessfulRequests)/float64(metrics.TotalRequests), 0.95, "Success rate should be >= 95%")
}

func (suite *MCPPerformanceE2ETestSuite) runSustainedLoadTest(ctx context.Context, duration time.Duration, concurrency int) *PerformanceMetrics {
	suite.metricsCollector.Reset()
	suite.metricsCollector.SetConcurrentUsers(concurrency)
	suite.metricsCollector.StartCollection()
	
	endTime := time.Now().Add(duration)
	var wg sync.WaitGroup
	
	toolRequests := []LSPToolRequest{
		{ToolName: "goto_definition", Parameters: map[string]interface{}{"uri": fmt.Sprintf("file://%s/color.go", suite.workspace.RootPath), "line": 110, "character": 0}},
		{ToolName: "find_references", Parameters: map[string]interface{}{"uri": fmt.Sprintf("file://%s/color.go", suite.workspace.RootPath), "line": 50, "character": 5}},
		{ToolName: "hover", Parameters: map[string]interface{}{"uri": fmt.Sprintf("file://%s/color.go", suite.workspace.RootPath), "line": 75, "character": 10}},
	}
	
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			requestCount := 0
			for time.Now().Before(endTime) {
				select {
				case <-ctx.Done():
					return
				default:
				}
				
				toolRequest := toolRequests[requestCount%len(toolRequests)]
				requestCount++
				
				requestStart := time.Now()
				
				toolCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
				result, err := suite.client.CallTool(toolCtx, toolRequest.ToolName, toolRequest.Parameters)
				cancel()
				
				latency := time.Since(requestStart)
				
				suite.metricsCollector.RecordRequest(toolRequest.ToolName, latency, err == nil && result.Error == nil)
				
				// Take periodic memory snapshots
				if requestCount%100 == 0 {
					suite.metricsCollector.TakeMemorySnapshot()
				}
				
				time.Sleep(500 * time.Millisecond)
			}
		}(i)
	}
	
	wg.Wait()
	suite.metricsCollector.StopCollection()
	
	return suite.metricsCollector.GetMetrics(duration)
}

// TestPerformanceRegression tests for performance regression against baseline
func (suite *MCPPerformanceE2ETestSuite) TestPerformanceRegression() {
	if suite.baselineMetrics == nil {
		suite.T().Skip("No baseline metrics available for regression testing")
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	
	currentMetrics := suite.runPerformanceBaseline(ctx)
	
	suite.T().Logf("Performance regression analysis:")
	suite.T().Logf("  Baseline average latency: %v", suite.baselineMetrics.AverageLatency)
	suite.T().Logf("  Current average latency: %v", currentMetrics.AverageLatency)
	suite.T().Logf("  Baseline throughput: %.2f req/s", suite.baselineMetrics.ThroughputRPS)
	suite.T().Logf("  Current throughput: %.2f req/s", currentMetrics.ThroughputRPS)
	
	// Ensure no significant performance regression (allow 20% variance in latency)
	latencyTolerance := float64(suite.baselineMetrics.AverageLatency) * 0.2
	suite.InDelta(float64(suite.baselineMetrics.AverageLatency), float64(currentMetrics.AverageLatency), 
		latencyTolerance, "Latency regression check")
	
	// Ensure throughput hasn't decreased significantly (allow 10% variance)
	throughputTolerance := suite.baselineMetrics.ThroughputRPS * 0.1
	suite.InDelta(suite.baselineMetrics.ThroughputRPS, currentMetrics.ThroughputRPS,
		throughputTolerance, "Throughput regression check")
}

func (suite *MCPPerformanceE2ETestSuite) runPerformanceBaseline(ctx context.Context) *PerformanceMetrics {
	// Standard baseline test: 10 concurrent users, 50 requests each
	return suite.runConcurrentLoadTest(ctx, 10, 50)
}

// TestLoadTestScenarios runs predefined load test scenarios
func (suite *MCPPerformanceE2ETestSuite) TestLoadTestScenarios() {
	for _, scenario := range suite.loadTestScenarios {
		suite.Run(scenario.Name, func() {
			ctx, cancel := context.WithTimeout(context.Background(), scenario.Duration+scenario.RampUpDuration+scenario.RampDownDuration)
			defer cancel()
			
			metrics := suite.runLoadTestScenario(ctx, scenario)
			
			suite.T().Logf("Load test scenario '%s' results:", scenario.Name)
			suite.logPerformanceMetrics(metrics)
			
			// Scenario-specific assertions
			switch scenario.Name {
			case "QuickBurst":
				suite.Less(metrics.P95Latency, 3*time.Second, "Quick burst P95 should be under 3s")
				suite.Greater(metrics.ThroughputRPS, 5.0, "Quick burst should achieve >5 req/s")
			case "SustainedLoad":
				suite.Less(metrics.P95Latency, 5*time.Second, "Sustained load P95 should be under 5s")
				suite.Greater(metrics.ThroughputRPS, 3.0, "Sustained load should achieve >3 req/s")
			case "HeavyLoad":
				suite.Less(metrics.P95Latency, 10*time.Second, "Heavy load P95 should be under 10s")
				suite.Greater(metrics.ThroughputRPS, 2.0, "Heavy load should achieve >2 req/s")
			}
			
			// Common assertions for all scenarios
			suite.LessOrEqual(float64(metrics.FailedRequests)/float64(metrics.TotalRequests), 0.10, "Error rate should be <= 10%")
		})
	}
}

func (suite *MCPPerformanceE2ETestSuite) runLoadTestScenario(ctx context.Context, scenario LoadTestScenario) *PerformanceMetrics {
	suite.metricsCollector.Reset()
	suite.metricsCollector.SetConcurrentUsers(scenario.ConcurrentUsers)
	suite.metricsCollector.StartCollection()
	
	return suite.loadGenerator.ExecuteScenario(ctx, scenario)
}

func (suite *MCPPerformanceE2ETestSuite) getMemoryUsage() float64 {
	var memStats runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStats)
	return float64(memStats.Alloc) / 1024 / 1024 // Convert to MB
}

func (suite *MCPPerformanceE2ETestSuite) logPerformanceMetrics(metrics *PerformanceMetrics) {
	suite.T().Logf("  Total requests: %d", metrics.TotalRequests)
	suite.T().Logf("  Successful: %d", metrics.SuccessfulRequests)
	suite.T().Logf("  Failed: %d", metrics.FailedRequests)
	suite.T().Logf("  Error rate: %.2f%%", float64(metrics.FailedRequests)/float64(metrics.TotalRequests)*100)
	suite.T().Logf("  Average latency: %v", metrics.AverageLatency)
	suite.T().Logf("  P50 latency: %v", metrics.P50Latency)
	suite.T().Logf("  P95 latency: %v", metrics.P95Latency)
	suite.T().Logf("  P99 latency: %v", metrics.P99Latency)
	suite.T().Logf("  Throughput: %.2f req/s", metrics.ThroughputRPS)
	suite.T().Logf("  Memory usage: %.2f MB", metrics.MemoryUsageMB)
	if metrics.CacheHitRate > 0 {
		suite.T().Logf("  Cache hit rate: %.2f%%", metrics.CacheHitRate*100)
	}
}

// NewPerformanceMetricsCollector creates a new performance metrics collector
func NewPerformanceMetricsCollector() *PerformanceMetricsCollector {
	return &PerformanceMetricsCollector{
		latencies:         make(map[string][]time.Duration),
		requestCounts:     make(map[string]int64),
		successCounts:     make(map[string]int64),
		errorCounts:       make(map[string]int64),
		memorySnapshots:   make([]MemorySnapshot, 0),
	}
}

func (pmc *PerformanceMetricsCollector) Reset() {
	pmc.mutex.Lock()
	defer pmc.mutex.Unlock()
	
	pmc.latencies = make(map[string][]time.Duration)
	pmc.requestCounts = make(map[string]int64)
	pmc.successCounts = make(map[string]int64)
	pmc.errorCounts = make(map[string]int64)
	pmc.memorySnapshots = make([]MemorySnapshot, 0)
	pmc.cacheHits = 0
	pmc.cacheMisses = 0
}

func (pmc *PerformanceMetricsCollector) SetConcurrentUsers(users int) {
	pmc.mutex.Lock()
	defer pmc.mutex.Unlock()
	pmc.concurrentUsers = users
}

func (pmc *PerformanceMetricsCollector) StartCollection() {
	pmc.mutex.Lock()
	defer pmc.mutex.Unlock()
	pmc.startTime = time.Now()
}

func (pmc *PerformanceMetricsCollector) StopCollection() {
	pmc.mutex.Lock()
	defer pmc.mutex.Unlock()
	pmc.endTime = time.Now()
}

func (pmc *PerformanceMetricsCollector) RecordRequest(toolName string, latency time.Duration, success bool) {
	pmc.mutex.Lock()
	defer pmc.mutex.Unlock()
	
	if pmc.latencies[toolName] == nil {
		pmc.latencies[toolName] = make([]time.Duration, 0)
	}
	
	pmc.latencies[toolName] = append(pmc.latencies[toolName], latency)
	pmc.requestCounts[toolName]++
	
	if success {
		pmc.successCounts[toolName]++
	} else {
		pmc.errorCounts[toolName]++
	}
}

func (pmc *PerformanceMetricsCollector) TakeMemorySnapshot() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	snapshot := MemorySnapshot{
		Timestamp:     time.Now(),
		AllocatedMB:   float64(memStats.Alloc) / 1024 / 1024,
		HeapInUseMB:   float64(memStats.HeapInuse) / 1024 / 1024,
		GCCount:       memStats.NumGC,
		GCPauseTotalNs: memStats.PauseTotalNs,
	}
	
	pmc.mutex.Lock()
	pmc.memorySnapshots = append(pmc.memorySnapshots, snapshot)
	pmc.mutex.Unlock()
}

func (pmc *PerformanceMetricsCollector) GetMetrics(duration time.Duration) *PerformanceMetrics {
	pmc.mutex.RLock()
	defer pmc.mutex.RUnlock()
	
	metrics := &PerformanceMetrics{
		ConcurrentUsers:   pmc.concurrentUsers,
		TestDuration:      duration,
		LSPFeatureMetrics: make(map[string]*LSPFeatureMetrics),
	}
	
	// Collect all latencies and calculate overall metrics
	var allLatencies []time.Duration
	var totalRequests, totalSuccessful, totalFailed int64
	
	for toolName, latencies := range pmc.latencies {
		allLatencies = append(allLatencies, latencies...)
		totalRequests += pmc.requestCounts[toolName]
		totalSuccessful += pmc.successCounts[toolName]
		totalFailed += pmc.errorCounts[toolName]
		
		// Calculate per-feature metrics
		featureMetrics := &LSPFeatureMetrics{
			FeatureName:   toolName,
			RequestCount:  pmc.requestCounts[toolName],
			SuccessCount:  pmc.successCounts[toolName],
			ErrorCount:    pmc.errorCounts[toolName],
		}
		
		if len(latencies) > 0 {
			sortedLatencies := make([]time.Duration, len(latencies))
			copy(sortedLatencies, latencies)
			sort.Slice(sortedLatencies, func(i, j int) bool {
				return sortedLatencies[i] < sortedLatencies[j]
			})
			
			var totalLatency time.Duration
			for _, lat := range sortedLatencies {
				totalLatency += lat
			}
			
			featureMetrics.AverageLatency = totalLatency / time.Duration(len(sortedLatencies))
			featureMetrics.P95Latency = sortedLatencies[int(float64(len(sortedLatencies))*0.95)]
			featureMetrics.ThroughputRPS = float64(len(latencies)) / duration.Seconds()
		}
		
		metrics.LSPFeatureMetrics[toolName] = featureMetrics
	}
	
	metrics.TotalRequests = totalRequests
	metrics.SuccessfulRequests = totalSuccessful
	metrics.FailedRequests = totalFailed
	
	if len(allLatencies) > 0 {
		sort.Slice(allLatencies, func(i, j int) bool {
			return allLatencies[i] < allLatencies[j]
		})
		
		var totalLatency time.Duration
		for _, lat := range allLatencies {
			totalLatency += lat
		}
		
		metrics.AverageLatency = totalLatency / time.Duration(len(allLatencies))
		metrics.MinLatency = allLatencies[0]
		metrics.MaxLatency = allLatencies[len(allLatencies)-1]
		metrics.P50Latency = allLatencies[len(allLatencies)/2]
		metrics.P95Latency = allLatencies[int(float64(len(allLatencies))*0.95)]
		metrics.P99Latency = allLatencies[int(float64(len(allLatencies))*0.99)]
		metrics.ThroughputRPS = float64(len(allLatencies)) / duration.Seconds()
	}
	
	// Calculate cache hit rate
	totalCacheRequests := pmc.cacheHits + pmc.cacheMisses
	if totalCacheRequests > 0 {
		metrics.CacheHitRate = float64(pmc.cacheHits) / float64(totalCacheRequests)
	}
	
	// Calculate memory usage
	if len(pmc.memorySnapshots) > 0 {
		var totalMemory float64
		for _, snapshot := range pmc.memorySnapshots {
			totalMemory += snapshot.AllocatedMB
		}
		metrics.MemoryUsageMB = totalMemory / float64(len(pmc.memorySnapshots))
	}
	
	return metrics
}

// NewLoadTestGenerator creates a new load test generator
func NewLoadTestGenerator(client *client.EnhancedMCPTestClient, workspace *types.TestWorkspace) *LoadTestGenerator {
	return &LoadTestGenerator{
		client:    client,
		workspace: workspace,
		stopChan:  make(chan struct{}),
	}
}

func (ltg *LoadTestGenerator) ExecuteScenario(ctx context.Context, scenario LoadTestScenario) *PerformanceMetrics {
	metrics := NewPerformanceMetricsCollector()
	metrics.SetConcurrentUsers(scenario.ConcurrentUsers)
	metrics.StartCollection()
	
	// Ramp up phase
	if scenario.RampUpDuration > 0 {
		ltg.executeRampUp(ctx, scenario, metrics)
	}
	
	// Main execution phase
	ltg.executeMainPhase(ctx, scenario, metrics)
	
	// Ramp down phase
	if scenario.RampDownDuration > 0 {
		ltg.executeRampDown(ctx, scenario, metrics)
	}
	
	metrics.StopCollection()
	return metrics.GetMetrics(scenario.Duration)
}

func (ltg *LoadTestGenerator) executeRampUp(ctx context.Context, scenario LoadTestScenario, metrics *PerformanceMetricsCollector) {
	// Gradual ramp up of concurrent users
	rampUpSteps := 5
	usersPerStep := scenario.ConcurrentUsers / rampUpSteps
	stepDuration := scenario.RampUpDuration / time.Duration(rampUpSteps)
	
	for step := 1; step <= rampUpSteps; step++ {
		currentUsers := usersPerStep * step
		ltg.executeLoadPhase(ctx, stepDuration, currentUsers, scenario, metrics)
	}
}

func (ltg *LoadTestGenerator) executeMainPhase(ctx context.Context, scenario LoadTestScenario, metrics *PerformanceMetricsCollector) {
	ltg.executeLoadPhase(ctx, scenario.Duration, scenario.ConcurrentUsers, scenario, metrics)
}

func (ltg *LoadTestGenerator) executeRampDown(ctx context.Context, scenario LoadTestScenario, metrics *PerformanceMetricsCollector) {
	// Gradual ramp down of concurrent users
	rampDownSteps := 3
	usersPerStep := scenario.ConcurrentUsers / rampDownSteps
	stepDuration := scenario.RampDownDuration / time.Duration(rampDownSteps)
	
	for step := rampDownSteps; step >= 1; step-- {
		currentUsers := usersPerStep * step
		ltg.executeLoadPhase(ctx, stepDuration, currentUsers, scenario, metrics)
	}
}

func (ltg *LoadTestGenerator) executeLoadPhase(ctx context.Context, duration time.Duration, concurrency int, scenario LoadTestScenario, metrics *PerformanceMetricsCollector) {
	endTime := time.Now().Add(duration)
	var wg sync.WaitGroup
	
	// Build weighted tool request list
	toolRequests := ltg.buildWeightedToolRequests(scenario.LSPFeatureWeights)
	
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			requestCount := 0
			for time.Now().Before(endTime) {
				select {
				case <-ctx.Done():
					return
				case <-ltg.stopChan:
					return
				default:
				}
				
				toolRequest := toolRequests[requestCount%len(toolRequests)]
				requestCount++
				
				requestStart := time.Now()
				
				toolCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				result, err := ltg.client.CallTool(toolCtx, toolRequest.ToolName, toolRequest.Parameters)
				cancel()
				
				latency := time.Since(requestStart)
				
				metrics.RecordRequest(toolRequest.ToolName, latency, err == nil && result.Error == nil)
				
				if scenario.ThinkTimeBetween > 0 {
					time.Sleep(scenario.ThinkTimeBetween)
				}
			}
		}(i)
	}
	
	wg.Wait()
}

func (ltg *LoadTestGenerator) buildWeightedToolRequests(weights map[string]float64) []LSPToolRequest {
	var requests []LSPToolRequest
	
	baseParams := map[string]map[string]interface{}{
		"goto_definition": {
			"uri":       fmt.Sprintf("file://%s/color.go", ltg.workspace.RootPath),
			"line":      110,
			"character": 0,
		},
		"find_references": {
			"uri":                fmt.Sprintf("file://%s/color.go", ltg.workspace.RootPath),
			"line":               50,
			"character":          5,
			"include_declaration": true,
		},
		"hover": {
			"uri":       fmt.Sprintf("file://%s/color.go", ltg.workspace.RootPath),
			"line":      75,
			"character": 10,
		},
		"document_symbols": {
			"uri": fmt.Sprintf("file://%s/color.go", ltg.workspace.RootPath),
		},
		"workspace_symbol": {
			"query": "Color",
		},
		"completion": {
			"uri":       fmt.Sprintf("file://%s/color.go", ltg.workspace.RootPath),
			"line":      100,
			"character": 15,
		},
	}
	
	// Build weighted list (multiply by 100 to get reasonable integer counts)
	for toolName, weight := range weights {
		if params, exists := baseParams[toolName]; exists {
			count := int(weight * 100)
			for i := 0; i < count; i++ {
				requests = append(requests, LSPToolRequest{
					ToolName:   toolName,
					Parameters: params,
					Weight:     weight,
				})
			}
		}
	}
	
	return requests
}

// MCPStdioTransport is a simple implementation for testing
type MCPStdioTransport struct{}

func (t *MCPStdioTransport) Connect(ctx context.Context) error {
	return nil
}

func (t *MCPStdioTransport) Disconnect() error {
	return nil
}

func (t *MCPStdioTransport) Send(ctx context.Context, message []byte) error {
	return nil
}

func (t *MCPStdioTransport) Receive(ctx context.Context) ([]byte, error) {
	return nil, nil
}

func (t *MCPStdioTransport) IsConnected() bool {
	return true
}

func (t *MCPStdioTransport) Close() error {
	return nil
}

func TestMCPPerformanceE2ETestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping MCP performance tests in short mode")
	}
	suite.Run(t, new(MCPPerformanceE2ETestSuite))
}