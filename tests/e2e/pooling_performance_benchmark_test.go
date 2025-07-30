package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/internal/testing/pooling"
	"lsp-gateway/internal/transport"
	"lsp-gateway/tests/e2e/testutils"
)

// PoolingPerformanceBenchmarkSuite provides comprehensive performance benchmarks
// comparing pooled servers vs fresh server creation across different languages
type PoolingPerformanceBenchmarkSuite struct {
	suite.Suite
	testTimeout    time.Duration
	poolManager    *pooling.ServerPoolManager
	benchmarkData  map[string]*LanguageBenchmarkData
	mu             sync.Mutex
}

// LanguageBenchmarkData stores performance metrics for a specific language
type LanguageBenchmarkData struct {
	Language                string        `json:"language"`
	FreshServerSetupTime    time.Duration `json:"fresh_server_setup_time"`
	PooledServerSetupTime   time.Duration `json:"pooled_server_setup_time"`
	ImprovementPercentage   float64       `json:"improvement_percentage"`
	FreshServerRequestTime  time.Duration `json:"fresh_server_request_time"`
	PooledServerRequestTime time.Duration `json:"pooled_server_request_time"`
	RequestImprovementPct   float64       `json:"request_improvement_percentage"`
	FreshServerTotalTime    time.Duration `json:"fresh_server_total_time"`
	PooledServerTotalTime   time.Duration `json:"pooled_server_total_time"`
	TotalImprovementPct     float64       `json:"total_improvement_percentage"`
	SuccessRate             float64       `json:"success_rate"`
	TestCases               int           `json:"test_cases"`
}

// BenchmarkResult captures comprehensive benchmark results
type BenchmarkResult struct {
	TestName           string             `json:"test_name"`
	OverallImprovement float64            `json:"overall_improvement_percentage"`
	LanguageResults    map[string]*LanguageBenchmarkData `json:"language_results"`
	ExecutionTime      time.Duration      `json:"execution_time"`
	PoolingWorked      bool               `json:"pooling_worked"`
	ErrorCount         int                `json:"error_count"`
	TestTimestamp      time.Time          `json:"test_timestamp"`
}

// SetupSuite initializes the benchmark test suite
func (suite *PoolingPerformanceBenchmarkSuite) SetupSuite() {
	suite.testTimeout = 10 * time.Minute // Extended timeout for benchmarks
	suite.benchmarkData = make(map[string]*LanguageBenchmarkData)
	
	// Initialize pooling for benchmarks
	err := suite.initializePoolingForBenchmarks()
	if err != nil {
		suite.T().Logf("Warning: Pooling initialization failed, will run comparison with fallback only: %v", err)
	}
}

// TearDownSuite cleans up after benchmarks
func (suite *PoolingPerformanceBenchmarkSuite) TearDownSuite() {
	if suite.poolManager != nil {
		_ = pooling.StopPooling()
	}
}

// BenchmarkJavaServerStartup_WithoutPooling measures Java server startup without pooling
func (suite *PoolingPerformanceBenchmarkSuite) BenchmarkJavaServerStartup_WithoutPooling(b *testing.B) {
	suite.benchmarkLanguageSetup(b, "java", false)
}

// BenchmarkJavaServerStartup_WithPooling measures Java server startup with pooling
func (suite *PoolingPerformanceBenchmarkSuite) BenchmarkJavaServerStartup_WithPooling(b *testing.B) {
	suite.benchmarkLanguageSetup(b, "java", true)
}

// BenchmarkTypeScriptServerStartup_WithoutPooling measures TypeScript server startup without pooling
func (suite *PoolingPerformanceBenchmarkSuite) BenchmarkTypeScriptServerStartup_WithoutPooling(b *testing.B) {
	suite.benchmarkLanguageSetup(b, "typescript", false)
}

// BenchmarkTypeScriptServerStartup_WithPooling measures TypeScript server startup with pooling
func (suite *PoolingPerformanceBenchmarkSuite) BenchmarkTypeScriptServerStartup_WithPooling(b *testing.B) {
	suite.benchmarkLanguageSetup(b, "typescript", true)
}

// BenchmarkGoServerStartup_WithoutPooling measures Go server startup without pooling
func (suite *PoolingPerformanceBenchmarkSuite) BenchmarkGoServerStartup_WithoutPooling(b *testing.B) {
	suite.benchmarkLanguageSetup(b, "go", false)
}

// BenchmarkGoServerStartup_WithPooling measures Go server startup with pooling
func (suite *PoolingPerformanceBenchmarkSuite) BenchmarkGoServerStartup_WithPooling(b *testing.B) {
	suite.benchmarkLanguageSetup(b, "go", true)
}

// BenchmarkPythonServerStartup_WithoutPooling measures Python server startup without pooling
func (suite *PoolingPerformanceBenchmarkSuite) BenchmarkPythonServerStartup_WithoutPooling(b *testing.B) {
	suite.benchmarkLanguageSetup(b, "python", false)
}

// BenchmarkPythonServerStartup_WithPooling measures Python server startup with pooling
func (suite *PoolingPerformanceBenchmarkSuite) BenchmarkPythonServerStartup_WithPooling(b *testing.B) {
	suite.benchmarkLanguageSetup(b, "python", true)
}

// TestMultiLanguageWorkspaceComparison compares multi-language workspace setup times
func (suite *PoolingPerformanceBenchmarkSuite) TestMultiLanguageWorkspaceComparison() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	languages := []string{"java", "typescript", "go", "python"}
	
	// Test without pooling
	freshResult := suite.benchmarkMultiLanguageWorkspace(ctx, languages, false)
	
	// Test with pooling
	pooledResult := suite.benchmarkMultiLanguageWorkspace(ctx, languages, true)
	
	// Calculate improvements
	overallImprovement := suite.calculateOverallImprovement(freshResult, pooledResult)
	
	// Log detailed results
	suite.T().Logf("\n=== Multi-Language Workspace Performance Comparison ===")
	suite.T().Logf("Fresh Server Total Time: %v", freshResult.ExecutionTime)
	suite.T().Logf("Pooled Server Total Time: %v", pooledResult.ExecutionTime)
	suite.T().Logf("Overall Improvement: %.2f%%", overallImprovement)
	
	// Validate improvement targets
	suite.validatePerformanceTargets(overallImprovement, freshResult, pooledResult)
	
	// Save benchmark results
	suite.saveBenchmarkResults(overallImprovement, freshResult, pooledResult)
}

// TestLanguageSpecificPerformanceTargets validates specific language performance targets
func (suite *PoolingPerformanceBenchmarkSuite) TestLanguageSpecificPerformanceTargets() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	testCases := []struct {
		language           string
		targetImprovement  float64  // Minimum expected improvement percentage
		maxPooledTime      time.Duration // Maximum acceptable pooled server time
		description        string
	}{
		{
			language:          "java",
			targetImprovement: 90.0, // Java JDTLS: 5min 35s → 10-20s (94-96% improvement)
			maxPooledTime:     20 * time.Second,
			description:       "Java JDTLS should improve from 5min 35s to <20s",
		},
		{
			language:          "typescript",
			targetImprovement: 85.0, // TypeScript: 30-45s → 3-5s (89-94% improvement)
			maxPooledTime:     5 * time.Second,
			description:       "TypeScript should improve from 30-45s to <5s",
		},
		{
			language:          "go",
			targetImprovement: 80.0, // Go: 15-20s → 2-3s (85-90% improvement)
			maxPooledTime:     3 * time.Second,
			description:       "Go should improve from 15-20s to <3s",
		},
		{
			language:          "python",
			targetImprovement: 75.0, // Python: 10-15s → 2-3s (80-85% improvement)
			maxPooledTime:     3 * time.Second,
			description:       "Python should improve from 10-15s to <3s",
		},
	}
	
	for _, testCase := range testCases {
		suite.Run(testCase.language, func() {
			// Measure fresh server performance
			freshTime := suite.measureLanguageServerSetup(ctx, testCase.language, false)
			
			// Measure pooled server performance
			pooledTime := suite.measureLanguageServerSetup(ctx, testCase.language, true)
			
			// Calculate improvement
			improvement := ((freshTime.Seconds() - pooledTime.Seconds()) / freshTime.Seconds()) * 100
			
			suite.T().Logf("%s Performance Results:", testCase.language)
			suite.T().Logf("  Fresh Server Time: %v", freshTime)
			suite.T().Logf("  Pooled Server Time: %v", pooledTime)
			suite.T().Logf("  Improvement: %.2f%%", improvement)
			suite.T().Logf("  Target: %.2f%% improvement, <%v pooled time", testCase.targetImprovement, testCase.maxPooledTime)
			
			// Validate performance targets
			suite.GreaterOrEqual(improvement, testCase.targetImprovement, 
				"Language %s should achieve at least %.2f%% improvement", testCase.language, testCase.targetImprovement)
			suite.LessOrEqual(pooledTime, testCase.maxPooledTime,
				"Language %s pooled server should start in <%v", testCase.language, testCase.maxPooledTime)
		})
	}
}

// benchmarkLanguageSetup benchmarks language server setup
func (suite *PoolingPerformanceBenchmarkSuite) benchmarkLanguageSetup(b *testing.B, language string, usePooling bool) {
	ctx := context.Background()
	
	for i := 0; i < b.N; i++ {
		suite.measureLanguageServerSetup(ctx, language, usePooling)
	}
}

// benchmarkMultiLanguageWorkspace benchmarks multi-language workspace setup
func (suite *PoolingPerformanceBenchmarkSuite) benchmarkMultiLanguageWorkspace(ctx context.Context, languages []string, usePooling bool) *BenchmarkResult {
	startTime := time.Now()
	
	// Create multi-project manager configuration
	config := testutils.MultiProjectConfig{
		EnableLogging:            true,
		EnablePooling:            usePooling,
		FallbackOnPoolingFailure: true,
	}
	
	if usePooling {
		config.PoolConfig = suite.createTestPoolConfig()
	}
	
	// Create and setup workspace
	manager := testutils.NewMultiProjectRepositoryManager(config)
	defer manager.Cleanup()
	
	_, err := manager.SetupMultiProjectWorkspace(languages)
	
	executionTime := time.Since(startTime)
	
	result := &BenchmarkResult{
		TestName:      fmt.Sprintf("MultiLanguageWorkspace_Pooling=%v", usePooling),
		ExecutionTime: executionTime,
		PoolingWorked: usePooling && err == nil,
		ErrorCount:    0,
		TestTimestamp: time.Now(),
		LanguageResults: make(map[string]*LanguageBenchmarkData),
	}
	
	if err != nil {
		result.ErrorCount = 1
		suite.T().Logf("Workspace setup failed (pooling=%v): %v", usePooling, err)
	}
	
	// Get pooling metrics if available
	if concreteManager, ok := manager.(*testutils.ConcreteMultiProjectManager); ok && usePooling {
		poolingMetrics := concreteManager.GetPoolingMetrics()
		for lang, metrics := range poolingMetrics {
			result.LanguageResults[lang] = &LanguageBenchmarkData{
				Language:    lang,
				SuccessRate: 1.0,
				TestCases:   1,
			}
			if !metrics.UsePooling {
				result.LanguageResults[lang].SuccessRate = 0.0
			}
		}
	}
	
	return result
}

// measureLanguageServerSetup measures the time to setup a language server
func (suite *PoolingPerformanceBenchmarkSuite) measureLanguageServerSetup(ctx context.Context, language string, usePooling bool) time.Duration {
	startTime := time.Now()
	
	// Create workspace directory
	workspace, err := os.MkdirTemp("", fmt.Sprintf("benchmark-%s-*", language))
	if err != nil {
		suite.T().Logf("Failed to create workspace: %v", err)
		return time.Since(startTime)
	}
	defer os.RemoveAll(workspace)
	
	// Create client configuration
	config := suite.getClientConfig(language)
	
	var client transport.LSPClient
	if usePooling {
		// Use pooled client
		poolingConfig := &testutils.PoolingConfig{
			EnablePooling:     true,
			FallbackOnFailure: true,
		}
		client, err = testutils.NewLSPClientWithOptionalPooling(language, workspace, config, poolingConfig)
	} else {
		// Use fresh client
		client, err = transport.NewLSPClient(config)
	}
	
	if err != nil {
		suite.T().Logf("Failed to create client for %s: %v", language, err)
		return time.Since(startTime)
	}
	
	// Start the client
	err = client.Start(ctx)
	if err != nil {
		suite.T().Logf("Failed to start client for %s: %v", language, err)
		return time.Since(startTime)
	}
	
	// Stop the client
	_ = client.Stop()
	
	return time.Since(startTime)
}

// calculateOverallImprovement calculates the overall improvement percentage
func (suite *PoolingPerformanceBenchmarkSuite) calculateOverallImprovement(freshResult, pooledResult *BenchmarkResult) float64 {
	if freshResult.ExecutionTime == 0 {
		return 0
	}
	
	return ((freshResult.ExecutionTime.Seconds() - pooledResult.ExecutionTime.Seconds()) / freshResult.ExecutionTime.Seconds()) * 100
}

// validatePerformanceTargets validates that performance targets are met
func (suite *PoolingPerformanceBenchmarkSuite) validatePerformanceTargets(overallImprovement float64, freshResult, pooledResult *BenchmarkResult) {
	// Overall target: 50-70% improvement
	suite.GreaterOrEqual(overallImprovement, 50.0, 
		"Overall improvement should be at least 50%%, got %.2f%%", overallImprovement)
	
	// Validate that pooling actually worked
	suite.True(pooledResult.PoolingWorked, "Pooling should have worked for performance comparison")
	
	// Validate error rates
	suite.Equal(0, freshResult.ErrorCount, "Fresh server setup should not have errors")
	suite.Equal(0, pooledResult.ErrorCount, "Pooled server setup should not have errors")
	
	// Log achievement
	if overallImprovement >= 70.0 {
		suite.T().Logf("🎉 Excellent performance! Achieved %.2f%% improvement (target: 50-70%%)", overallImprovement)
	} else if overallImprovement >= 50.0 {
		suite.T().Logf("✅ Performance target met! Achieved %.2f%% improvement (target: 50-70%%)", overallImprovement)
	}
}

// saveBenchmarkResults saves benchmark results for analysis
func (suite *PoolingPerformanceBenchmarkSuite) saveBenchmarkResults(overallImprovement float64, freshResult, pooledResult *BenchmarkResult) {
	results := map[string]interface{}{
		"overall_improvement_percentage": overallImprovement,
		"fresh_server_result":           freshResult,
		"pooled_server_result":          pooledResult,
		"test_timestamp":               time.Now(),
		"target_achieved":              overallImprovement >= 50.0,
	}
	
	// Save to temporary file for analysis
	if data, err := json.MarshalIndent(results, "", "  "); err == nil {
		tmpFile := fmt.Sprintf("/tmp/pooling_benchmark_results_%d.json", time.Now().Unix())
		if err := os.WriteFile(tmpFile, data, 0644); err == nil {
			suite.T().Logf("Benchmark results saved to: %s", tmpFile)
		}
	}
}

// Helper methods

func (suite *PoolingPerformanceBenchmarkSuite) initializePoolingForBenchmarks() error {
	// Create pool configuration optimized for benchmarks
	poolConfig := suite.createTestPoolConfig()
	
	// Initialize global pool manager
	err := pooling.InitializeGlobalPoolManager(poolConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize pool manager: %w", err)
	}
	
	// Start pooling
	ctx := context.Background()
	err = pooling.StartPooling(ctx)
	if err != nil {
		return fmt.Errorf("failed to start pooling: %w", err)
	}
	
	suite.poolManager = pooling.GetGlobalPoolManager()
	return nil
}

func (suite *PoolingPerformanceBenchmarkSuite) createTestPoolConfig() *pooling.PoolConfig {
	return &pooling.PoolConfig{
		Enabled:           true,
		GlobalTimeout:     300 * time.Second,
		AllocationTimeout: 30 * time.Second,
		EnableFallback:    true,
		EnableMetrics:     true,
		MetricsInterval:   10 * time.Second,
		HealthCheckInterval: 30 * time.Second,
		Languages: map[string]*pooling.LanguagePoolConfig{
			"java": {
				MinServers:              2,
				MaxServers:              3,
				WarmupTimeout:           90 * time.Second,
				IdleTimeout:             600 * time.Second,
				MaxUseCount:             10,
				FailureThreshold:        3,
				RestartThreshold:        5,
				EnableWorkspaceSwitching: true,
				TransportType:           "stdio",
				ConcurrentRequests:      2,
			},
			"typescript": {
				MinServers:              2,
				MaxServers:              2,
				WarmupTimeout:           45 * time.Second,
				IdleTimeout:             300 * time.Second,
				MaxUseCount:             15,
				FailureThreshold:        3,
				RestartThreshold:        5,
				EnableWorkspaceSwitching: true,
				TransportType:           "stdio",
				ConcurrentRequests:      3,
			},
			"go": {
				MinServers:         1,
				MaxServers:         2,
				WarmupTimeout:      20 * time.Second,
				IdleTimeout:        300 * time.Second,
				MaxUseCount:        20,
				FailureThreshold:   2,
				RestartThreshold:   3,
				TransportType:      "stdio",
				ConcurrentRequests: 4,
			},
			"python": {
				MinServers:         1,
				MaxServers:         2,
				WarmupTimeout:      15 * time.Second,
				IdleTimeout:        300 * time.Second,
				MaxUseCount:        25,
				FailureThreshold:   2,
				RestartThreshold:   3,
				TransportType:      "stdio",
				ConcurrentRequests: 3,
			},
		},
	}
}

func (suite *PoolingPerformanceBenchmarkSuite) getClientConfig(language string) transport.ClientConfig {
	configs := map[string]transport.ClientConfig{
		"java": {
			Command:   "jdtls",
			Args:      []string{},
			Transport: transport.TransportStdio,
		},
		"typescript": {
			Command:   "typescript-language-server",
			Args:      []string{"--stdio"},
			Transport: transport.TransportStdio,
		},
		"go": {
			Command:   "gopls",
			Args:      []string{},
			Transport: transport.TransportStdio,
		},
		"python": {
			Command:   "pylsp",
			Args:      []string{},
			Transport: transport.TransportStdio,
		},
	}
	
	return configs[language]
}

// Test suite runner
func TestPoolingPerformanceBenchmarkSuite(t *testing.T) {
	suite.Run(t, new(PoolingPerformanceBenchmarkSuite))
}