package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-gateway/internal/gateway"
	"lsp-gateway/internal/indexing"
	"lsp-gateway/tests/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SmartRouterIntegrationTest validates SCIP Smart Router with <5ms routing decisions
type SmartRouterIntegrationTest struct {
	framework    *framework.MultiLanguageTestFramework
	smartRouter  gateway.SCIPSmartRouter
	scipStore    indexing.SCIPStore
	testProject  *framework.TestProject
	
	// Performance validation
	routingDecisionLatencies []time.Duration
	confidenceScores         []gateway.ConfidenceLevel
	cacheHitRates           []float64
	
	// Test configuration
	targetRoutingLatency    time.Duration // <5ms target
	minConfidenceThreshold  gateway.ConfidenceLevel
	expectedCacheHitRate    float64
	
	mu sync.RWMutex
}

// SmartRouterTestScenario represents different routing scenarios to test
type SmartRouterTestScenario struct {
	Name                string
	RoutingStrategy     gateway.SCIPRoutingStrategyType
	ExpectedLatency     time.Duration
	ExpectedConfidence  gateway.ConfidenceLevel
	ShouldUseSCIP      bool
	ExpectedCacheHit   bool
}

// RoutingDecisionMetrics tracks routing decision performance
type RoutingDecisionMetrics struct {
	AverageLatency      time.Duration
	P95Latency          time.Duration
	P99Latency          time.Duration
	AverageConfidence   gateway.ConfidenceLevel
	SCIPUsageRate       float64
	CacheHitRate        float64
	FallbackRate        float64
	ErrorRate           float64
}

// NewSmartRouterIntegrationTest creates a new Smart Router integration test
func NewSmartRouterIntegrationTest(t *testing.T) *SmartRouterIntegrationTest {
	return &SmartRouterIntegrationTest{
		framework:               framework.NewMultiLanguageTestFramework(30 * time.Minute),
		targetRoutingLatency:    5 * time.Millisecond, // Phase 2 target: <5ms
		minConfidenceThreshold:  gateway.ConfidenceMedium,
		expectedCacheHitRate:    0.85, // Target 85% cache hit rate
		routingDecisionLatencies: make([]time.Duration, 0),
		confidenceScores:        make([]gateway.ConfidenceLevel, 0),
		cacheHitRates:          make([]float64, 0),
	}
}

// TestSmartRouterRoutingDecisionLatency validates <5ms routing decision target
func (test *SmartRouterIntegrationTest) TestSmartRouterRoutingDecisionLatency(t *testing.T) {
	ctx := context.Background()
	
	err := test.setupSmartRouterEnvironment(ctx)
	require.NoError(t, err, "Failed to setup Smart Router environment")
	defer test.cleanup()
	
	t.Log("Testing Smart Router routing decision latency (<5ms target)...")
	
	// Test scenarios with different routing strategies
	scenarios := []SmartRouterTestScenario{
		{
			Name:               "SCIP_Cache_First_High_Confidence",
			RoutingStrategy:    gateway.SCIPCacheFirst,
			ExpectedLatency:    2 * time.Millisecond,
			ExpectedConfidence: gateway.ConfidenceHigh,
			ShouldUseSCIP:     true,
			ExpectedCacheHit:  true,
		},
		{
			Name:               "SCIP_Hybrid_Medium_Confidence", 
			RoutingStrategy:    gateway.SCIPHybrid,
			ExpectedLatency:    4 * time.Millisecond,
			ExpectedConfidence: gateway.ConfidenceMedium,
			ShouldUseSCIP:     true,
			ExpectedCacheHit:  false,
		},
		{
			Name:               "SCIP_LSP_First_Low_Confidence",
			RoutingStrategy:    gateway.SCIPLSPFirst,
			ExpectedLatency:    3 * time.Millisecond,
			ExpectedConfidence: gateway.ConfidenceMinimum,
			ShouldUseSCIP:     false,
			ExpectedCacheHit:  false,
		},
	}
	
	// Execute routing decision performance tests
	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			metrics := test.executeRoutingDecisionTest(t, scenario, 100)
			test.validateRoutingDecisionMetrics(t, scenario, metrics)
		})
	}
	
	// Validate overall performance targets
	test.validateOverallRoutingPerformance(t)
}

// TestSmartRouterConfidenceScoring validates confidence calculation accuracy
func (test *SmartRouterIntegrationTest) TestSmartRouterConfidenceScoring(t *testing.T) {
	ctx := context.Background()
	
	err := test.setupSmartRouterEnvironment(ctx)
	require.NoError(t, err, "Failed to setup Smart Router environment")
	defer test.cleanup()
	
	t.Log("Testing Smart Router confidence scoring accuracy...")
	
	// Test confidence calculation for different query types
	confidenceTests := []struct {
		method              string
		params              interface{}
		expectedConfidence  gateway.ConfidenceLevel
		description         string
	}{
		{
			method: "textDocument/definition",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": "file:///test.go"},
				"position": map[string]interface{}{"line": 10, "character": 5},
			},
			expectedConfidence: gateway.ConfidenceHigh,
			description: "High confidence for definition queries with SCIP index",
		},
		{
			method: "textDocument/references",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": "file:///test.go"},
				"position": map[string]interface{}{"line": 15, "character": 10},
			},
			expectedConfidence: gateway.ConfidenceMedium,
			description: "Medium confidence for reference queries",
		},
		{
			method: "workspace/symbol", 
			params: map[string]interface{}{
				"query": "TestFunction",
			},
			expectedConfidence: gateway.ConfidenceMedium,
			description: "Medium confidence for workspace symbol queries",
		},
	}
	
	for _, confidenceTest := range confidenceTests {
		t.Run(confidenceTest.description, func(t *testing.T) {
			// Create mock SCIP query result
			queryResult := &indexing.SCIPQueryResult{
				Found:    true,
				CacheHit: true,
				Data:     json.RawMessage(`{"result": "mock_data"}`),
			}
			
			// Calculate confidence
			startTime := time.Now()
			confidence := test.smartRouter.CalculateConfidence(
				confidenceTest.method, 
				confidenceTest.params, 
				queryResult,
			)
			latency := time.Since(startTime)
			
			// Validate confidence score
			assert.GreaterOrEqual(t, float64(confidence), float64(confidenceTest.expectedConfidence), 
				"Confidence score should meet minimum threshold")
			
			// Validate confidence calculation latency
			assert.Less(t, latency, 1*time.Millisecond, 
				"Confidence calculation should be <1ms")
			
			test.recordConfidenceScore(confidence)
			
			t.Logf("Confidence test (%s): Score=%.2f, Latency=%v", 
				confidenceTest.description, float64(confidence), latency)
		})
	}
}

// TestSmartRouterAdaptiveStrategies validates adaptive routing strategy optimization
func (test *SmartRouterIntegrationTest) TestSmartRouterAdaptiveStrategies(t *testing.T) {
	ctx := context.Background()
	
	err := test.setupSmartRouterEnvironment(ctx)
	require.NoError(t, err, "Failed to setup Smart Router environment")
	defer test.cleanup()
	
	t.Log("Testing Smart Router adaptive strategy optimization...")
	
	// Simulate performance data that should trigger strategy optimization
	performanceData := &gateway.PerformanceAnalysis{
		Method:               "textDocument/definition",
		AverageLatency:       15 * time.Millisecond,
		P95Latency:          25 * time.Millisecond,
		CacheHitRate:        0.7,
		SCIPUsageRate:       0.8,
		ErrorRate:           0.02,
		Recommendation:      "increase_scip_confidence_threshold",
	}
	
	// Test strategy optimization
	originalStrategy := test.smartRouter.GetSCIPRoutingStrategy("textDocument/definition")
	
	err = test.smartRouter.OptimizeRoutingStrategy("textDocument/definition", performanceData)
	require.NoError(t, err, "Strategy optimization should succeed")
	
	optimizedStrategy := test.smartRouter.GetSCIPRoutingStrategy("textDocument/definition")
	
	// Validate strategy was optimized
	if performanceData.CacheHitRate < 0.8 && performanceData.AverageLatency > 10*time.Millisecond {
		assert.NotEqual(t, originalStrategy, optimizedStrategy, 
			"Strategy should be optimized based on performance data")
	}
	
	// Test that optimization improves performance
	metrics := test.executeRoutingDecisionTest(t, SmartRouterTestScenario{
		Name:            "Optimized_Strategy_Test",
		RoutingStrategy: optimizedStrategy,
		ExpectedLatency: 4 * time.Millisecond, // Should be improved
	}, 50)
	
	assert.Less(t, metrics.AverageLatency, performanceData.AverageLatency,
		"Optimized strategy should improve performance")
	
	t.Logf("Strategy optimization: Original=%s, Optimized=%s, Improvement=%v", 
		originalStrategy, optimizedStrategy, performanceData.AverageLatency - metrics.AverageLatency)
}

// TestSmartRouterConcurrentDecisions validates performance under concurrent load
func (test *SmartRouterIntegrationTest) TestSmartRouterConcurrentDecisions(t *testing.T) {
	ctx := context.Background()
	
	err := test.setupSmartRouterEnvironment(ctx)
	require.NoError(t, err, "Failed to setup Smart Router environment")
	defer test.cleanup()
	
	t.Log("Testing Smart Router performance under concurrent load...")
	
	concurrencyLevels := []int{10, 25, 50, 100, 200}
	
	for _, concurrency := range concurrencyLevels {
		t.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(t *testing.T) {
			metrics := test.executeConcurrentRoutingTest(t, concurrency, 30*time.Second)
			
			// Validate performance under load
			assert.Less(t, metrics.P95Latency, 10*time.Millisecond,
				"P95 latency should remain under 10ms even under load")
			assert.Less(t, metrics.AverageLatency, test.targetRoutingLatency,
				"Average latency should meet <5ms target under load")
			assert.Greater(t, metrics.SCIPUsageRate, 0.7,
				"SCIP usage rate should remain high under load")
			assert.Less(t, metrics.ErrorRate, 0.05,
				"Error rate should remain under 5% under load")
			
			t.Logf("Concurrent routing (C=%d): Avg=%v, P95=%v, SCIP=%.2f%%, Errors=%.2f%%",
				concurrency, metrics.AverageLatency, metrics.P95Latency,
				metrics.SCIPUsageRate*100, metrics.ErrorRate*100)
		})
	}
}

// TestSmartRouterCacheIntegration validates cache-first routing effectiveness
func (test *SmartRouterIntegrationTest) TestSmartRouterCacheIntegration(t *testing.T) {
	ctx := context.Background()
	
	err := test.setupSmartRouterEnvironment(ctx)
	require.NoError(t, err, "Failed to setup Smart Router environment")
	defer test.cleanup()
	
	t.Log("Testing Smart Router cache integration effectiveness...")
	
	// Pre-warm cache with known entries
	err = test.prewarmSCIPCache(ctx)
	require.NoError(t, err, "Failed to pre-warm SCIP cache")
	
	// Test cache-first routing effectiveness
	cacheTests := []struct {
		name               string
		routingStrategy    gateway.SCIPRoutingStrategyType
		expectedCacheHit   bool
		expectedLatency    time.Duration
	}{
		{
			name:            "Cache_First_Warm_Cache",
			routingStrategy: gateway.SCIPCacheFirst,
			expectedCacheHit: true,
			expectedLatency: 2 * time.Millisecond,
		},
		{
			name:            "Hybrid_Strategy_Warm_Cache",
			routingStrategy: gateway.SCIPHybrid,
			expectedCacheHit: true,
			expectedLatency: 3 * time.Millisecond,
		},
		{
			name:            "LSP_First_Strategy",
			routingStrategy: gateway.SCIPLSPFirst,
			expectedCacheHit: false,
			expectedLatency: 4 * time.Millisecond,
		},
	}
	
	for _, cacheTest := range cacheTests {
		t.Run(cacheTest.name, func(t *testing.T) {
			test.smartRouter.SetSCIPRoutingStrategy("textDocument/definition", cacheTest.routingStrategy)
			
			metrics := test.executeRoutingDecisionTest(t, SmartRouterTestScenario{
				Name:             cacheTest.name,
				RoutingStrategy:  cacheTest.routingStrategy,
				ExpectedLatency:  cacheTest.expectedLatency,
				ExpectedCacheHit: cacheTest.expectedCacheHit,
			}, 100)
			
			// Validate cache effectiveness
			if cacheTest.expectedCacheHit {
				assert.Greater(t, metrics.CacheHitRate, 0.8,
					"Cache hit rate should be >80% for cache-first strategies")
			}
			
			assert.Less(t, metrics.AverageLatency, cacheTest.expectedLatency,
				"Latency should meet expectations for cache strategy")
			
			t.Logf("Cache integration (%s): Hit rate=%.2f%%, Latency=%v",
				cacheTest.name, metrics.CacheHitRate*100, metrics.AverageLatency)
		})
	}
}

// BenchmarkSmartRouterPerformance benchmarks Smart Router routing decisions
func (test *SmartRouterIntegrationTest) BenchmarkSmartRouterPerformance(b *testing.B) {
	ctx := context.Background()
	
	err := test.setupSmartRouterEnvironment(ctx)
	if err != nil {
		b.Fatalf("Failed to setup Smart Router environment: %v", err)
	}
	defer test.cleanup()
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		request := &gateway.LSPRequest{
			Method: "textDocument/definition",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": "file:///test.go"},
				"position": map[string]interface{}{"line": i % 100, "character": 5},
			},
		}
		
		_, err := test.smartRouter.RouteWithSCIPAwareness(request)
		if err != nil {
			b.Fatalf("Routing decision failed: %v", err)
		}
	}
}

// Helper methods

func (test *SmartRouterIntegrationTest) setupSmartRouterEnvironment(ctx context.Context) error {
	// Setup test framework
	if err := test.framework.SetupTestEnvironment(ctx); err != nil {
		return fmt.Errorf("failed to setup framework: %w", err)
	}
	
	// Create test project
	project, err := test.framework.CreateMultiLanguageProject(
		framework.ProjectTypeMultiLanguage,
		[]string{"go", "python", "typescript", "java"})
	if err != nil {
		return fmt.Errorf("failed to create test project: %w", err)
	}
	test.testProject = project
	
	// Initialize SCIP store
	scipConfig := &indexing.SCIPConfig{
		CacheConfig: indexing.CacheConfig{
			Enabled: true,
			MaxSize: 1000,
			TTL:     10 * time.Minute,
		},
		Performance: indexing.PerformanceConfig{
			QueryTimeout:         100 * time.Millisecond,
			MaxConcurrentQueries: 50,
		},
	}
	
	test.scipStore = indexing.NewSCIPIndexStore(scipConfig)
	
	// Create Smart Router
	routerConfig := &gateway.SmartRouterConfig{
		EnableSCIPIntegration: true,
		DefaultStrategy:       gateway.SCIPHybrid,
		PerformanceTargets: &gateway.PerformanceTargets{
			MaxRoutingLatency: test.targetRoutingLatency,
			MinConfidence:     test.minConfidenceThreshold,
		},
	}
	
	smartRouter, err := gateway.NewSCIPSmartRouter(routerConfig)
	if err != nil {
		return fmt.Errorf("failed to create Smart Router: %w", err)
	}
	
	// Set SCIP store
	err = smartRouter.SetSCIPStore(test.scipStore)
	if err != nil {
		return fmt.Errorf("failed to set SCIP store: %w", err)
	}
	
	test.smartRouter = smartRouter
	
	return nil
}

func (test *SmartRouterIntegrationTest) prewarmSCIPCache(ctx context.Context) error {
	// Pre-warm cache with common files
	commonFiles := []string{
		"file:///test.go",
		"file:///main.go", 
		"file:///utils.go",
		"file:///service.go",
	}
	
	return test.smartRouter.PrewarmSCIPCache(ctx, commonFiles)
}

func (test *SmartRouterIntegrationTest) executeRoutingDecisionTest(t *testing.T, scenario SmartRouterTestScenario, iterations int) *RoutingDecisionMetrics {
	test.smartRouter.SetSCIPRoutingStrategy("textDocument/definition", scenario.RoutingStrategy)
	
	var (
		latencies     []time.Duration
		confidences   []gateway.ConfidenceLevel
		scipUsed      int64
		cacheHits     int64
		fallbacks     int64
		errors        int64
	)
	
	for i := 0; i < iterations; i++ {
		request := &gateway.LSPRequest{
			Method: "textDocument/definition",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": "file:///test.go"},
				"position": map[string]interface{}{"line": i % 50, "character": 5},
			},
		}
		
		startTime := time.Now()
		decision, err := test.smartRouter.RouteWithSCIPAwareness(request)
		latency := time.Since(startTime)
		
		if err != nil {
			atomic.AddInt64(&errors, 1)
			continue
		}
		
		latencies = append(latencies, latency)
		confidences = append(confidences, decision.Confidence)
		
		if decision.UseSCIP {
			atomic.AddInt64(&scipUsed, 1)
		}
		if decision.SCIPResult != nil && decision.SCIPResult.CacheHit {
			atomic.AddInt64(&cacheHits, 1)
		}
		if decision.FallbackToLSP {
			atomic.AddInt64(&fallbacks, 1)
		}
		
		test.recordRoutingDecision(latency, decision.Confidence)
	}
	
	return test.calculateRoutingMetrics(latencies, confidences, scipUsed, cacheHits, fallbacks, errors, int64(iterations))
}

func (test *SmartRouterIntegrationTest) executeConcurrentRoutingTest(t *testing.T, concurrency int, duration time.Duration) *RoutingDecisionMetrics {
	var (
		totalLatencies = make([]time.Duration, 0)
		totalConfidences = make([]gateway.ConfidenceLevel, 0)
		scipUsed      int64
		cacheHits     int64
		fallbacks     int64
		errors        int64
		requests      int64
		wg            sync.WaitGroup
		mu            sync.Mutex
	)
	
	endTime := time.Now().Add(duration)
	
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			workerLatencies := make([]time.Duration, 0)
			workerConfidences := make([]gateway.ConfidenceLevel, 0)
			
			for time.Now().Before(endTime) {
				request := &gateway.LSPRequest{
					Method: "textDocument/definition",
					Params: map[string]interface{}{
						"textDocument": map[string]interface{}{"uri": "file:///test.go"},
						"position": map[string]interface{}{"line": workerID % 100, "character": 5},
					},
				}
				
				startTime := time.Now()
				decision, err := test.smartRouter.RouteWithSCIPAwareness(request)
				latency := time.Since(startTime)
				
				atomic.AddInt64(&requests, 1)
				
				if err != nil {
					atomic.AddInt64(&errors, 1)
					continue
				}
				
				workerLatencies = append(workerLatencies, latency)
				workerConfidences = append(workerConfidences, decision.Confidence)
				
				if decision.UseSCIP {
					atomic.AddInt64(&scipUsed, 1)
				}
				if decision.SCIPResult != nil && decision.SCIPResult.CacheHit {
					atomic.AddInt64(&cacheHits, 1)
				}
				if decision.FallbackToLSP {
					atomic.AddInt64(&fallbacks, 1)
				}
				
				time.Sleep(10 * time.Millisecond)
			}
			
			// Aggregate worker results
			mu.Lock()
			totalLatencies = append(totalLatencies, workerLatencies...)
			totalConfidences = append(totalConfidences, workerConfidences...)
			mu.Unlock()
		}(i)
	}
	
	wg.Wait()
	
	return test.calculateRoutingMetrics(totalLatencies, totalConfidences, scipUsed, cacheHits, fallbacks, errors, requests)
}

func (test *SmartRouterIntegrationTest) calculateRoutingMetrics(latencies []time.Duration, confidences []gateway.ConfidenceLevel, scipUsed, cacheHits, fallbacks, errors, totalRequests int64) *RoutingDecisionMetrics {
	if len(latencies) == 0 {
		return &RoutingDecisionMetrics{}
	}
	
	// Calculate average latency
	var totalLatency time.Duration
	for _, latency := range latencies {
		totalLatency += latency
	}
	avgLatency := totalLatency / time.Duration(len(latencies))
	
	// Calculate percentiles
	sortedLatencies := make([]time.Duration, len(latencies))
	copy(sortedLatencies, latencies)
	for i := 0; i < len(sortedLatencies); i++ {
		for j := i + 1; j < len(sortedLatencies); j++ {
			if sortedLatencies[i] > sortedLatencies[j] {
				sortedLatencies[i], sortedLatencies[j] = sortedLatencies[j], sortedLatencies[i]
			}
		}
	}
	
	p95Index := int(float64(len(sortedLatencies)) * 0.95)
	p99Index := int(float64(len(sortedLatencies)) * 0.99)
	if p95Index >= len(sortedLatencies) {
		p95Index = len(sortedLatencies) - 1
	}
	if p99Index >= len(sortedLatencies) {
		p99Index = len(sortedLatencies) - 1
	}
	
	// Calculate average confidence
	var totalConfidence float64
	for _, confidence := range confidences {
		totalConfidence += float64(confidence)
	}
	avgConfidence := gateway.ConfidenceLevel(totalConfidence / float64(len(confidences)))
	
	return &RoutingDecisionMetrics{
		AverageLatency:    avgLatency,
		P95Latency:       sortedLatencies[p95Index],
		P99Latency:       sortedLatencies[p99Index],
		AverageConfidence: avgConfidence,
		SCIPUsageRate:     float64(scipUsed) / float64(totalRequests),
		CacheHitRate:      float64(cacheHits) / float64(totalRequests),
		FallbackRate:      float64(fallbacks) / float64(totalRequests),
		ErrorRate:         float64(errors) / float64(totalRequests),
	}
}

func (test *SmartRouterIntegrationTest) validateRoutingDecisionMetrics(t *testing.T, scenario SmartRouterTestScenario, metrics *RoutingDecisionMetrics) {
	// Validate latency target
	assert.Less(t, metrics.AverageLatency, test.targetRoutingLatency,
		"Average routing decision latency should be <5ms")
	assert.Less(t, metrics.P95Latency, 2*test.targetRoutingLatency,
		"P95 routing decision latency should be <10ms")
	
	// Validate confidence scoring
	assert.GreaterOrEqual(t, float64(metrics.AverageConfidence), float64(test.minConfidenceThreshold),
		"Average confidence should meet minimum threshold")
	
	// Validate SCIP usage based on strategy
	if scenario.ShouldUseSCIP {
		assert.Greater(t, metrics.SCIPUsageRate, 0.7,
			"SCIP usage rate should be >70% for SCIP-first strategies")
	}
	
	// Validate cache effectiveness
	if scenario.ExpectedCacheHit {
		assert.Greater(t, metrics.CacheHitRate, 0.8,
			"Cache hit rate should be >80% for cache-optimized scenarios")
	}
	
	// Validate error rate
	assert.Less(t, metrics.ErrorRate, 0.05,
		"Error rate should be <5%")
}

func (test *SmartRouterIntegrationTest) validateOverallRoutingPerformance(t *testing.T) {
	test.mu.RLock()
	defer test.mu.RUnlock()
	
	if len(test.routingDecisionLatencies) == 0 {
		return
	}
	
	// Calculate overall metrics
	var totalLatency time.Duration
	for _, latency := range test.routingDecisionLatencies {
		totalLatency += latency
	}
	avgLatency := totalLatency / time.Duration(len(test.routingDecisionLatencies))
	
	var totalConfidence float64
	for _, confidence := range test.confidenceScores {
		totalConfidence += float64(confidence)
	}
	avgConfidence := gateway.ConfidenceLevel(totalConfidence / float64(len(test.confidenceScores)))
	
	// Validate overall performance
	assert.Less(t, avgLatency, test.targetRoutingLatency,
		"Overall average routing latency should meet <5ms target")
	assert.GreaterOrEqual(t, float64(avgConfidence), float64(test.minConfidenceThreshold),
		"Overall average confidence should meet threshold")
	
	t.Logf("Overall Smart Router performance: Latency=%v, Confidence=%.2f",
		avgLatency, float64(avgConfidence))
}

func (test *SmartRouterIntegrationTest) recordRoutingDecision(latency time.Duration, confidence gateway.ConfidenceLevel) {
	test.mu.Lock()
	defer test.mu.Unlock()
	
	test.routingDecisionLatencies = append(test.routingDecisionLatencies, latency)
	test.confidenceScores = append(test.confidenceScores, confidence)
}

func (test *SmartRouterIntegrationTest) recordConfidenceScore(confidence gateway.ConfidenceLevel) {
	test.mu.Lock()
	defer test.mu.Unlock()
	
	test.confidenceScores = append(test.confidenceScores, confidence)
}

func (test *SmartRouterIntegrationTest) cleanup() {
	if test.scipStore != nil {
		test.scipStore.Close()
	}
	if test.framework != nil {
		test.framework.CleanupAll()
	}
}

// Integration test suite

func TestSmartRouterIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Smart Router integration tests in short mode")
	}
	
	test := NewSmartRouterIntegrationTest(t)
	
	t.Run("RoutingDecisionLatency", test.TestSmartRouterRoutingDecisionLatency)
	t.Run("ConfidenceScoring", test.TestSmartRouterConfidenceScoring)
	t.Run("AdaptiveStrategies", test.TestSmartRouterAdaptiveStrategies)
	t.Run("ConcurrentDecisions", test.TestSmartRouterConcurrentDecisions)
	t.Run("CacheIntegration", test.TestSmartRouterCacheIntegration)
}

func BenchmarkSmartRouterIntegration(b *testing.B) {
	test := NewSmartRouterIntegrationTest(nil)
	test.BenchmarkSmartRouterPerformance(b)
}