package scip

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"lsp-gateway/internal/indexing"
	"lsp-gateway/internal/transport"
	"github.com/sourcegraph/scip/bindings/go/scip"
)

// SCIPTestSuite provides comprehensive testing infrastructure for SCIP integration tests
type SCIPTestSuite struct {
	// Core components
	scipClient      *indexing.SCIPClient
	scipStore       indexing.SCIPStore
	scipMapper      *indexing.LSPSCIPMapper
	symbolResolver  *indexing.SymbolResolver
	performanceCache *indexing.PerformanceCache
	scipIndexer     transport.SCIPIndexer
	
	// Test infrastructure
	testDataDir     string
	tempDir         string
	scipIndices     map[string]string // language -> index file path
	testProjects    map[string]*TestProject
	
	// Performance tracking
	metrics         *TestMetrics
	benchmarkResults map[string]*BenchmarkResult
	
	// Configuration
	config          *SCIPTestConfig
	mutex           sync.RWMutex
	
	// Cleanup handlers
	cleanupHandlers []func() error
}

// SCIPTestConfig configures the SCIP test suite
type SCIPTestConfig struct {
	// Test data configuration
	TestDataDirectory    string        `json:"test_data_directory"`
	GenerateTestIndices  bool          `json:"generate_test_indices"`
	UseRealIndices      bool          `json:"use_real_indices"`
	IndexCacheDirectory string        `json:"index_cache_directory"`
	
	// Performance testing
	EnablePerformanceTests   bool          `json:"enable_performance_tests"`
	PerformanceTargetP99     time.Duration `json:"performance_target_p99"`
	PerformanceCacheHitRate  float64       `json:"performance_cache_hit_rate"`
	MaxMemoryUsageMB        int64         `json:"max_memory_usage_mb"`
	
	// Multi-language testing
	EnabledLanguages        []string      `json:"enabled_languages"`
	CrossLanguageReferences bool          `json:"cross_language_references"`
	
	// Error testing
	TestErrorScenarios      bool          `json:"test_error_scenarios"`
	TestFallbackBehavior    bool          `json:"test_fallback_behavior"`
	
	// Caching configuration
	TestCacheIntegration    bool          `json:"test_cache_integration"`
	CacheTestDuration       time.Duration `json:"cache_test_duration"`
	
	// Concurrency testing
	MaxConcurrentRequests   int           `json:"max_concurrent_requests"`
	ConcurrencyTestDuration time.Duration `json:"concurrency_test_duration"`
}

// TestProject represents a synthetic project for testing
type TestProject struct {
	Name         string            `json:"name"`
	Language     string            `json:"language"`
	Files        map[string]string `json:"files"`        // filename -> content
	Dependencies []string          `json:"dependencies"` // other projects this depends on
	SCIPIndex    string            `json:"scip_index"`   // path to SCIP index file
	Symbols      []*TestSymbol     `json:"symbols"`      // expected symbols
	References   []*TestReference  `json:"references"`   // expected references
}

// TestSymbol represents an expected symbol in test data
type TestSymbol struct {
	Name         string                      `json:"name"`
	Kind         scip.SymbolInformation_Kind `json:"kind"`
	URI          string                      `json:"uri"`
	Range        *scip.Range                 `json:"range"`
	Definition   *scip.Range                 `json:"definition,omitempty"`
	References   []*scip.Range               `json:"references,omitempty"`
	Documentation string                     `json:"documentation,omitempty"`
}

// TestReference represents an expected reference in test data
type TestReference struct {
	Symbol       string      `json:"symbol"`
	URI          string      `json:"uri"`
	Range        *scip.Range `json:"range"`
	Role         int32       `json:"role"`
	CrossLanguage bool       `json:"cross_language,omitempty"`
}

// TestMetrics tracks test execution metrics
type TestMetrics struct {
	// Test execution
	TotalTests       int           `json:"total_tests"`
	PassedTests      int           `json:"passed_tests"`
	FailedTests      int           `json:"failed_tests"`
	SkippedTests     int           `json:"skipped_tests"`
	TotalDuration    time.Duration `json:"total_duration"`
	
	// Performance metrics
	AverageQueryTime time.Duration `json:"average_query_time"`
	P95QueryTime     time.Duration `json:"p95_query_time"`
	P99QueryTime     time.Duration `json:"p99_query_time"`
	CacheHitRate     float64       `json:"cache_hit_rate"`
	MemoryUsage      int64         `json:"memory_usage_bytes"`
	
	// SCIP-specific metrics
	IndexLoadTime    time.Duration `json:"index_load_time"`
	QueryCount       int64         `json:"query_count"`
	SuccessfulQueries int64        `json:"successful_queries"`
	FailedQueries    int64         `json:"failed_queries"`
	CachePromotions  int64         `json:"cache_promotions"`
	
	// Multi-language metrics
	CrossLanguageQueries int64 `json:"cross_language_queries"`
	LanguageCoverage    map[string]int `json:"language_coverage"`
	
	mutex               sync.RWMutex  `json:"-"`
}

// BenchmarkResult stores benchmark execution results
type BenchmarkResult struct {
	TestName         string        `json:"test_name"`
	Duration         time.Duration `json:"duration"`
	Operations       int64         `json:"operations"`
	OperationsPerSec float64       `json:"operations_per_sec"`
	MemoryAllocated  int64         `json:"memory_allocated"`
	MemoryAllocations int64        `json:"memory_allocations"`
	CacheHitRate     float64       `json:"cache_hit_rate"`
	P99Latency       time.Duration `json:"p99_latency"`
	Success          bool          `json:"success"`
	Error            string        `json:"error,omitempty"`
}

// LSPTestRequest represents an LSP request for testing
type LSPTestRequest struct {
	Method    string                 `json:"method"`
	Params    map[string]interface{} `json:"params"`
	URI       string                 `json:"uri"`
	Position  *indexing.LSPPosition  `json:"position,omitempty"`
	Query     string                 `json:"query,omitempty"`
	Expected  interface{}            `json:"expected"`
	ShouldFail bool                  `json:"should_fail,omitempty"`
}

// LSPTestResponse represents an LSP response validation
type LSPTestResponse struct {
	Method       string          `json:"method"`
	Response     json.RawMessage `json:"response"`
	Found        bool            `json:"found"`
	CacheHit     bool            `json:"cache_hit"`
	QueryTime    time.Duration   `json:"query_time"`
	Confidence   float64         `json:"confidence"`
	ValidatedFields []string     `json:"validated_fields"`
}

// NewSCIPTestSuite creates a new comprehensive SCIP test suite
func NewSCIPTestSuite(config *SCIPTestConfig) (*SCIPTestSuite, error) {
	if config == nil {
		config = DefaultSCIPTestConfig()
	}
	
	// Create temporary directory for test data
	tempDir, err := os.MkdirTemp("", "scip-integration-tests-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	
	suite := &SCIPTestSuite{
		testDataDir:      config.TestDataDirectory,
		tempDir:          tempDir,
		scipIndices:      make(map[string]string),
		testProjects:     make(map[string]*TestProject),
		metrics:          &TestMetrics{LanguageCoverage: make(map[string]int)},
		benchmarkResults: make(map[string]*BenchmarkResult),
		config:           config,
		cleanupHandlers:  make([]func() error, 0),
	}
	
	// Initialize test data directory
	if err := suite.initializeTestData(); err != nil {
		suite.Cleanup()
		return nil, fmt.Errorf("failed to initialize test data: %w", err)
	}
	
	// Generate or load test projects
	if err := suite.setupTestProjects(); err != nil {
		suite.Cleanup()
		return nil, fmt.Errorf("failed to setup test projects: %w", err)
	}
	
	// Initialize SCIP components
	if err := suite.initializeSCIPComponents(); err != nil {
		suite.Cleanup()
		return nil, fmt.Errorf("failed to initialize SCIP components: %w", err)
	}
	
	return suite, nil
}

// DefaultSCIPTestConfig returns a default test configuration
func DefaultSCIPTestConfig() *SCIPTestConfig {
	return &SCIPTestConfig{
		TestDataDirectory:       "testdata/scip",
		GenerateTestIndices:     true,
		UseRealIndices:         false,
		IndexCacheDirectory:    "/tmp/scip-test-cache",
		EnablePerformanceTests: true,
		PerformanceTargetP99:   10 * time.Millisecond,
		PerformanceCacheHitRate: 0.85,
		MaxMemoryUsageMB:       100,
		EnabledLanguages:       []string{"go", "typescript", "python", "java"},
		CrossLanguageReferences: true,
		TestErrorScenarios:     true,
		TestFallbackBehavior:   true,
		TestCacheIntegration:   true,
		CacheTestDuration:      5 * time.Minute,
		MaxConcurrentRequests:  50,
		ConcurrencyTestDuration: 2 * time.Minute,
	}
}

// initializeTestData sets up the test data directory structure
func (suite *SCIPTestSuite) initializeTestData() error {
	// Create test data directory if it doesn't exist
	if err := os.MkdirAll(suite.testDataDir, 0755); err != nil {
		return fmt.Errorf("failed to create test data directory: %w", err)
	}
	
	// Create subdirectories for different test categories
	subdirs := []string{
		"scip_indices",
		"test_projects",
		"synthetic_data",
		"performance_data",
		"error_scenarios",
		"cache_test_data",
	}
	
	for _, subdir := range subdirs {
		dirPath := filepath.Join(suite.testDataDir, subdir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed to create subdirectory %s: %w", subdir, err)
		}
	}
	
	return nil
}

// setupTestProjects generates or loads test projects for each language
func (suite *SCIPTestSuite) setupTestProjects() error {
	for _, language := range suite.config.EnabledLanguages {
		project, err := suite.generateTestProject(language)
		if err != nil {
			return fmt.Errorf("failed to generate test project for %s: %w", language, err)
		}
		
		suite.testProjects[language] = project
		
		// Generate SCIP index for the project
		if suite.config.GenerateTestIndices {
			indexPath, err := suite.generateSCIPIndex(project)
			if err != nil {
				return fmt.Errorf("failed to generate SCIP index for %s: %w", language, err)
			}
			suite.scipIndices[language] = indexPath
		}
	}
	
	// Generate cross-language test projects if enabled
	if suite.config.CrossLanguageReferences {
		if err := suite.generateCrossLanguageProject(); err != nil {
			return fmt.Errorf("failed to generate cross-language project: %w", err)
		}
	}
	
	return nil
}

// initializeSCIPComponents initializes all SCIP components for testing
func (suite *SCIPTestSuite) initializeSCIPComponents() error {
	// Create SCIP configuration
	scipConfig := &indexing.SCIPConfig{
		CacheConfig: indexing.CacheConfig{
			Enabled: true,
			MaxSize: 1000,
			TTL:     30 * time.Minute,
		},
		Logging: indexing.LoggingConfig{
			LogQueries:         true,
			LogCacheOperations: true,
			LogIndexOperations: true,
		},
		Performance: indexing.PerformanceConfig{
			QueryTimeout:         suite.config.PerformanceTargetP99,
			MaxConcurrentQueries: suite.config.MaxConcurrentRequests,
			IndexLoadTimeout:     5 * time.Minute,
		},
	}
	
	// Initialize SCIP client
	client, err := indexing.NewSCIPClient(scipConfig)
	if err != nil {
		return fmt.Errorf("failed to create SCIP client: %w", err)
	}
	suite.scipClient = client
	suite.addCleanupHandler(func() error { return client.Close() })
	
	// Initialize SCIP store
	store := indexing.NewRealSCIPStore(scipConfig)
	suite.scipStore = store
	suite.addCleanupHandler(func() error { return store.Close() })
	
	// Initialize LSP-SCIP mapper
	mapper := indexing.NewLSPSCIPMapper(store, scipConfig)
	suite.scipMapper = mapper
	suite.addCleanupHandler(func() error { return mapper.Close() })
	
	// Initialize symbol resolver
	resolver, err := indexing.NewSymbolResolver(client, indexing.DefaultResolverConfig())
	if err != nil {
		return fmt.Errorf("failed to create symbol resolver: %w", err)
	}
	suite.symbolResolver = resolver
	suite.addCleanupHandler(func() error { return resolver.Close() })
	
	// Initialize performance cache if cache integration testing is enabled
	if suite.config.TestCacheIntegration {
		cacheConfig := indexing.DefaultPerformanceCacheConfig()
		cache, err := indexing.NewPerformanceCache(cacheConfig)
		if err != nil {
			return fmt.Errorf("failed to create performance cache: %w", err)
		}
		suite.performanceCache = cache
		suite.addCleanupHandler(func() error { 
			cache.Cleanup()
			return nil
		})
	}
	
	// Initialize SCIP indexer for transport integration
	indexer := transport.NewIntegrationSCIPIndexer()
	suite.scipIndexer = indexer
	suite.addCleanupHandler(func() error { return indexer.Close() })
	
	// Load SCIP indices into components
	if err := suite.loadSCIPIndices(); err != nil {
		return fmt.Errorf("failed to load SCIP indices: %w", err)
	}
	
	return nil
}

// loadSCIPIndices loads generated SCIP indices into the components
func (suite *SCIPTestSuite) loadSCIPIndices() error {
	for language, indexPath := range suite.scipIndices {
		// Load into SCIP client
		if err := suite.scipClient.LoadIndex(indexPath); err != nil {
			return fmt.Errorf("failed to load SCIP index for %s: %w", language, err)
		}
		
		// Load into SCIP store
		if err := suite.scipStore.LoadIndex(indexPath); err != nil {
			return fmt.Errorf("failed to load SCIP index into store for %s: %w", language, err)
		}
	}
	
	return nil
}

// addCleanupHandler adds a cleanup handler to be called when the suite is cleaned up
func (suite *SCIPTestSuite) addCleanupHandler(handler func() error) {
	suite.cleanupHandlers = append(suite.cleanupHandlers, handler)
}

// Cleanup cleans up all resources used by the test suite
func (suite *SCIPTestSuite) Cleanup() error {
	var errors []error
	
	// Call all cleanup handlers in reverse order
	for i := len(suite.cleanupHandlers) - 1; i >= 0; i-- {
		if err := suite.cleanupHandlers[i](); err != nil {
			errors = append(errors, err)
		}
	}
	
	// Clean up temporary directory
	if suite.tempDir != "" {
		if err := os.RemoveAll(suite.tempDir); err != nil {
			errors = append(errors, fmt.Errorf("failed to remove temp directory: %w", err))
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}
	
	return nil
}

// GetMetrics returns the current test metrics
func (suite *SCIPTestSuite) GetMetrics() *TestMetrics {
	suite.metrics.mutex.RLock()
	defer suite.metrics.mutex.RUnlock()
	
	// Create a copy to avoid concurrent modification
	metrics := *suite.metrics
	metrics.LanguageCoverage = make(map[string]int)
	for k, v := range suite.metrics.LanguageCoverage {
		metrics.LanguageCoverage[k] = v
	}
	
	return &metrics
}

// RecordTestResult records the result of a test execution
func (suite *SCIPTestSuite) RecordTestResult(testName string, passed bool, duration time.Duration) {
	suite.metrics.mutex.Lock()
	defer suite.metrics.mutex.Unlock()
	
	suite.metrics.TotalTests++
	suite.metrics.TotalDuration += duration
	
	if passed {
		suite.metrics.PassedTests++
	} else {
		suite.metrics.FailedTests++
	}
}

// RecordQueryMetrics records query execution metrics
func (suite *SCIPTestSuite) RecordQueryMetrics(queryTime time.Duration, cacheHit bool, success bool) {
	suite.metrics.mutex.Lock()
	defer suite.metrics.mutex.Unlock()
	
	suite.metrics.QueryCount++
	
	if success {
		suite.metrics.SuccessfulQueries++
	} else {
		suite.metrics.FailedQueries++
	}
	
	// Update average query time (simple moving average)
	if suite.metrics.AverageQueryTime == 0 {
		suite.metrics.AverageQueryTime = queryTime
	} else {
		suite.metrics.AverageQueryTime = (suite.metrics.AverageQueryTime + queryTime) / 2
	}
	
	// Update P99 (simplified - would use proper percentile calculation in production)
	if queryTime > suite.metrics.P99QueryTime {
		suite.metrics.P99QueryTime = queryTime
	}
	
	// Update P95 (simplified)
	if queryTime > suite.metrics.P95QueryTime {
		suite.metrics.P95QueryTime = queryTime
	}
	
	// Update cache hit rate
	if cacheHit {
		// Simplified cache hit rate calculation
		hits := float64(suite.metrics.QueryCount) * suite.metrics.CacheHitRate + 1
		suite.metrics.CacheHitRate = hits / float64(suite.metrics.QueryCount)
	} else {
		hits := float64(suite.metrics.QueryCount) * suite.metrics.CacheHitRate
		suite.metrics.CacheHitRate = hits / float64(suite.metrics.QueryCount)
	}
}

// ValidatePerformanceTargets validates that performance targets are met
func (suite *SCIPTestSuite) ValidatePerformanceTargets(t *testing.T) {
	metrics := suite.GetMetrics()
	
	// Validate P99 query time
	if metrics.P99QueryTime > suite.config.PerformanceTargetP99 {
		t.Errorf("P99 query time %v exceeds target %v", 
			metrics.P99QueryTime, suite.config.PerformanceTargetP99)
	}
	
	// Validate cache hit rate
	if metrics.CacheHitRate < suite.config.PerformanceCacheHitRate {
		t.Errorf("Cache hit rate %.2f%% below target %.2f%%", 
			metrics.CacheHitRate*100, suite.config.PerformanceCacheHitRate*100)
	}
	
	// Validate memory usage
	maxMemoryBytes := suite.config.MaxMemoryUsageMB * 1024 * 1024
	if metrics.MemoryUsage > maxMemoryBytes {
		t.Errorf("Memory usage %d bytes exceeds target %d bytes",
			metrics.MemoryUsage, maxMemoryBytes)
	}
	
	// Validate success rate
	if metrics.QueryCount > 0 {
		successRate := float64(metrics.SuccessfulQueries) / float64(metrics.QueryCount)
		if successRate < 0.95 { // 95% success rate target
			t.Errorf("Query success rate %.2f%% below target 95%%", successRate*100)
		}
	}
}

// GenerateTestReport generates a comprehensive test report
func (suite *SCIPTestSuite) GenerateTestReport() map[string]interface{} {
	metrics := suite.GetMetrics()
	
	report := map[string]interface{}{
		"test_execution": map[string]interface{}{
			"total_tests":    metrics.TotalTests,
			"passed_tests":   metrics.PassedTests,
			"failed_tests":   metrics.FailedTests,
			"skipped_tests":  metrics.SkippedTests,
			"total_duration": metrics.TotalDuration.String(),
		},
		"performance": map[string]interface{}{
			"average_query_time": metrics.AverageQueryTime.String(),
			"p95_query_time":     metrics.P95QueryTime.String(),
			"p99_query_time":     metrics.P99QueryTime.String(),
			"cache_hit_rate":     fmt.Sprintf("%.2f%%", metrics.CacheHitRate*100),
			"memory_usage_mb":    metrics.MemoryUsage / (1024 * 1024),
		},
		"scip_metrics": map[string]interface{}{
			"query_count":        metrics.QueryCount,
			"successful_queries": metrics.SuccessfulQueries,
			"failed_queries":     metrics.FailedQueries,
			"cache_promotions":   metrics.CachePromotions,
			"index_load_time":    metrics.IndexLoadTime.String(),
		},
		"multi_language": map[string]interface{}{
			"cross_language_queries": metrics.CrossLanguageQueries,
			"language_coverage":      metrics.LanguageCoverage,
		},
		"benchmark_results": suite.benchmarkResults,
		"configuration":     suite.config,
	}
	
	return report
}