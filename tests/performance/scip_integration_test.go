package performance

import (
	"testing"
	"time"
	
	"lsp-gateway/internal/indexing"
)

// TestSCIPIntegrationCompilation verifies SCIP performance tests compile and basic functionality
func TestSCIPIntegrationCompilation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP integration compilation test in short mode")
	}

	// Test that we can create a SCIP performance test instance
	scipTest := NewSCIPPerformanceTest(t)
	
	if scipTest == nil {
		t.Fatal("Failed to create SCIP performance test instance")
	}

	// Verify configuration is set correctly
	if scipTest.MinCacheHitRate != 0.10 {
		t.Errorf("Expected MinCacheHitRate 0.10, got %f", scipTest.MinCacheHitRate)
	}

	if scipTest.MaxSCIPOverhead != 50*time.Millisecond {
		t.Errorf("Expected MaxSCIPOverhead 50ms, got %v", scipTest.MaxSCIPOverhead)
	}

	if scipTest.MaxMemoryImpact != 100*1024*1024 {
		t.Errorf("Expected MaxMemoryImpact 100MB, got %d", scipTest.MaxMemoryImpact)
	}

	// Verify supported methods are configured
	if len(scipTest.SCIPSupportedMethods) != 5 {
		t.Errorf("Expected 5 SCIP supported methods, got %d", len(scipTest.SCIPSupportedMethods))
	}

	if len(scipTest.LSPOnlyMethods) != 4 {
		t.Errorf("Expected 4 LSP-only methods, got %d", len(scipTest.LSPOnlyMethods))
	}

	t.Log("SCIP performance test integration compiled and initialized successfully")
}

// TestSCIPPerformanceTestSuiteIntegration verifies integration with main performance suite
func TestSCIPPerformanceTestSuiteIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP performance suite integration test in short mode")
	}

	// Test that performance test suite can be created and includes SCIP
	suite := NewPerformanceTestSuite(t)
	
	if suite == nil {
		t.Fatal("Failed to create performance test suite")
	}

	// Initialize results structure
	suite.currentResults = &PerformanceTestResults{
		Timestamp:  time.Now(),
		SystemInfo: suite.collectSystemInfo(),
	}

	// Test that SCIP results can be set
	scipResults := &SCIPPerformanceResults{
		CacheHitRate:               0.12,
		MemoryImpactMB:            85,
		PerformanceRegression:     false,
		ThroughputMaintained:      true,
		RequirementsMet:           true,
		BaselineThroughputReqPerSec: 120.0,
		SCIPThroughputReqPerSec:    118.0,
		BaselineP95ResponseTime:    45,
		SCIPP95ResponseTime:        48,
	}

	suite.currentResults.SCIPPerformanceResults = scipResults

	// Test score calculation includes SCIP
	suite.calculateOverallPerformanceScore()

	if suite.currentResults.OverallPerformanceScore == 0 {
		t.Error("Overall performance score should not be 0")
	}

	t.Logf("Performance suite integration successful, score: %.1f", suite.currentResults.OverallPerformanceScore)
}

// TestSCIPMockStore verifies mock SCIP store functionality
func TestSCIPMockStore(t *testing.T) {
	// Setup minimal SCIP environment for testing
	scipConfig := &indexing.SCIPConfig{
		CacheConfig: indexing.CacheConfig{
			Enabled: true,
			MaxSize: 100,
			TTL:     1 * time.Minute,
		},
		Logging: indexing.LoggingConfig{
			LogQueries:         false,
			LogCacheOperations: false,
			LogIndexOperations: false,
		},
	}

	store := indexing.NewSCIPIndexStore(scipConfig)
	
	// Test index loading
	err := store.LoadIndex("/mock/test/index")
	if err != nil {
		t.Fatalf("Failed to load mock index: %v", err)
	}

	// Test querying
	result := store.Query("textDocument/definition", map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": "file:///test.go"},
		"position":     map[string]interface{}{"line": 5, "character": 10},
	})

	if result.Method != "textDocument/definition" {
		t.Errorf("Expected method 'textDocument/definition', got '%s'", result.Method)
	}

	// Test stats
	stats := store.GetStats()
	if stats.TotalQueries != 1 {
		t.Errorf("Expected 1 total query, got %d", stats.TotalQueries)
	}

	// Cleanup
	store.Close()

	t.Log("SCIP mock store functionality verified")
}