package scip

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-gateway/internal/indexing"
	"lsp-gateway/internal/transport"
)

// TestSCIPProductionReadiness validates SCIP components for production deployment
func TestSCIPProductionReadiness(t *testing.T) {
	t.Run("ErrorHandling", func(t *testing.T) {
		testErrorHandlingScenarios(t)
	})
	
	t.Run("FallbackMechanisms", func(t *testing.T) {
		testFallbackMechanisms(t)
	})
	
	t.Run("ConfigurationValidation", func(t *testing.T) {
		testConfigurationValidation(t)
	})
	
	t.Run("ResourceManagement", func(t *testing.T) {
		testResourceManagement(t)
	})
	
	t.Run("ConcurrentAccess", func(t *testing.T) {
		testConcurrentAccess(t)
	})
	
	t.Run("OperationalFeatures", func(t *testing.T) {
		testOperationalFeatures(t)
	})
}

// testErrorHandlingScenarios tests comprehensive error scenarios
func testErrorHandlingScenarios(t *testing.T) {
	config := &indexing.SCIPConfig{
		CacheConfig: indexing.CacheConfig{
			Enabled: true,
			MaxSize: 100,
			TTL:     5 * time.Minute,
		},
		Logging: indexing.LoggingConfig{
			LogQueries:         true,
			LogCacheOperations: true,
			LogIndexOperations: true,
		},
		Performance: indexing.PerformanceConfig{
			QueryTimeout:         5 * time.Second,
			MaxConcurrentQueries: 10,
			IndexLoadTimeout:     30 * time.Second,
		},
	}
	
	t.Run("CorruptedIndexHandling", func(t *testing.T) {
		client, err := indexing.NewSCIPClient(config)
		if err != nil {
			t.Fatalf("Failed to create SCIP client: %v", err)
		}
		defer client.Close()
		
		// Test loading corrupted index
		err = client.LoadIndex("/test/corrupted.scip")
		if err == nil {
			t.Error("Expected error for corrupted index, got nil")
		}
		
		// Verify client remains operational
		stats := client.GetStats()
		if stats.ErrorCount == 0 {
			t.Error("Error count should be incremented")
		}
	})
	
	t.Run("InvalidRequestHandling", func(t *testing.T) {
		store := indexing.NewMockSCIPStore(config)
		mapper := indexing.NewLSPSCIPMapper(store, config)
		
		// Test with nil parameters
		_, err := mapper.MapDefinition(nil)
		if err == nil {
			t.Error("Expected error for nil parameters")
		}
		
		// Test with invalid URI
		params := &indexing.LSPParams{
			Method: "textDocument/definition",
			URI:    "invalid://uri",
			Position: &indexing.LSPPosition{
				Line:      -1,
				Character: -1,
			},
		}
		_, err = mapper.MapDefinition(params)
		if err == nil {
			t.Error("Expected error for invalid URI")
		}
	})
	
	t.Run("TimeoutHandling", func(t *testing.T) {
		// Create config with very short timeout
		shortTimeoutConfig := &indexing.SCIPConfig{
			Performance: indexing.PerformanceConfig{
				QueryTimeout: 1 * time.Millisecond,
			},
		}
		
		client, err := indexing.NewSCIPClient(shortTimeoutConfig)
		if err != nil {
			t.Fatalf("Failed to create SCIP client: %v", err)
		}
		defer client.Close()
		
		// Simulate slow operation
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		
		result, err := client.QueryWithContext(ctx, "test-query")
		if err == nil {
			t.Error("Expected timeout error")
		}
		if result != nil {
			t.Error("Result should be nil on timeout")
		}
	})
	
	t.Run("MemoryPressureHandling", func(t *testing.T) {
		// Test behavior under memory pressure
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		initialMemory := m.Alloc
		
		// Create multiple clients with large cache
		clients := make([]*indexing.SCIPClient, 10)
		for i := 0; i < 10; i++ {
			largeConfig := &indexing.SCIPConfig{
				CacheConfig: indexing.CacheConfig{
					Enabled: true,
					MaxSize: 10000,
					TTL:     1 * time.Hour,
				},
			}
			client, err := indexing.NewSCIPClient(largeConfig)
			if err != nil {
				t.Fatalf("Failed to create client %d: %v", i, err)
			}
			clients[i] = client
		}
		
		// Clean up and verify memory is released
		for _, client := range clients {
			client.Close()
		}
		
		runtime.GC()
		runtime.ReadMemStats(&m)
		finalMemory := m.Alloc
		
		// Memory should not grow excessively
		memoryGrowth := finalMemory - initialMemory
		if memoryGrowth > 100*1024*1024 { // 100MB threshold
			t.Errorf("Excessive memory growth: %d bytes", memoryGrowth)
		}
	})
}

// testFallbackMechanisms tests graceful fallback to LSP
func testFallbackMechanisms(t *testing.T) {
	t.Run("SCIPUnavailableFallback", func(t *testing.T) {
		// Create a store that simulates SCIP unavailability
		store := &failingMockStore{
			failureRate: 1.0, // Always fail
		}
		
		mapper := indexing.NewLSPSCIPMapper(store, nil)
		
		params := &indexing.LSPParams{
			Method: "textDocument/definition",
			TextDocument: &indexing.TextDocumentIdentifier{
				URI: "file:///test.go",
			},
			Position: &indexing.LSPPosition{
				Line:      10,
				Character: 5,
			},
		}
		
		// Should return empty result, not error (graceful degradation)
		result, err := mapper.MapDefinition(params)
		if err != nil {
			t.Errorf("Expected graceful fallback, got error: %v", err)
		}
		if result == nil {
			t.Error("Expected empty result, got nil")
		}
		
		// Verify result is empty array
		var locations []indexing.LSPLocation
		if err := json.Unmarshal(result, &locations); err != nil {
			t.Errorf("Failed to unmarshal result: %v", err)
		}
		if len(locations) != 0 {
			t.Errorf("Expected empty locations, got %d", len(locations))
		}
	})
	
	t.Run("PartialSCIPFailure", func(t *testing.T) {
		// Test with intermittent failures
		store := &failingMockStore{
			failureRate: 0.5, // 50% failure rate
		}
		
		mapper := indexing.NewLSPSCIPMapper(store, nil)
		
		successCount := 0
		totalRequests := 100
		
		for i := 0; i < totalRequests; i++ {
			params := &indexing.LSPParams{
				Method: "textDocument/references",
				TextDocument: &indexing.TextDocumentIdentifier{
					URI: fmt.Sprintf("file:///test%d.go", i),
				},
				Position: &indexing.LSPPosition{
					Line:      i,
					Character: 0,
				},
			}
			
			_, err := mapper.MapReferences(params)
			if err == nil {
				successCount++
			}
		}
		
		// Should handle partial failures gracefully
		if successCount == 0 {
			t.Error("All requests failed, expected some successes")
		}
		if successCount == totalRequests {
			t.Error("All requests succeeded, expected some failures")
		}
		
		t.Logf("Success rate: %.2f%% (%d/%d)", float64(successCount)/float64(totalRequests)*100, successCount, totalRequests)
	})
	
	t.Run("FallbackLatency", func(t *testing.T) {
		// Test that fallback doesn't add excessive latency
		store := &slowMockStore{
			delay: 100 * time.Millisecond,
		}
		
		mapper := indexing.NewLSPSCIPMapper(store, nil)
		
		start := time.Now()
		params := &indexing.LSPParams{
			Method: "textDocument/hover",
			TextDocument: &indexing.TextDocumentIdentifier{
				URI: "file:///test.go",
			},
			Position: &indexing.LSPPosition{
				Line:      5,
				Character: 10,
			},
		}
		
		_, err := mapper.MapHover(params)
		elapsed := time.Since(start)
		
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		
		// Should timeout and fallback quickly
		if elapsed > 200*time.Millisecond {
			t.Errorf("Fallback took too long: %v", elapsed)
		}
	})
}

// testConfigurationValidation tests configuration schema and validation
func testConfigurationValidation(t *testing.T) {
	t.Run("InvalidConfigurationHandling", func(t *testing.T) {
		testCases := []struct {
			name   string
			config *indexing.SCIPConfig
			valid  bool
		}{
			{
				name:   "NilConfig",
				config: nil,
				valid:  false,
			},
			{
				name: "NegativeCacheSize",
				config: &indexing.SCIPConfig{
					CacheConfig: indexing.CacheConfig{
						MaxSize: -1,
					},
				},
				valid: false,
			},
			{
				name: "InvalidTimeout",
				config: &indexing.SCIPConfig{
					Performance: indexing.PerformanceConfig{
						QueryTimeout: -1 * time.Second,
					},
				},
				valid: false,
			},
			{
				name: "ValidMinimalConfig",
				config: &indexing.SCIPConfig{
					CacheConfig: indexing.CacheConfig{
						Enabled: false,
					},
				},
				valid: true,
			},
			{
				name: "ValidFullConfig",
				config: &indexing.SCIPConfig{
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
						QueryTimeout:         10 * time.Second,
						MaxConcurrentQueries: 50,
						IndexLoadTimeout:     5 * time.Minute,
					},
				},
				valid: true,
			},
		}
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := indexing.NewSCIPClient(tc.config)
				if tc.valid && err != nil {
					t.Errorf("Expected valid config, got error: %v", err)
				}
				if !tc.valid && err == nil {
					t.Error("Expected invalid config error, got nil")
				}
			})
		}
	})
	
	t.Run("EnvironmentVariableSupport", func(t *testing.T) {
		// Test environment variable override
		envVars := map[string]string{
			"SCIP_CACHE_ENABLED":   "true",
			"SCIP_CACHE_MAX_SIZE":  "5000",
			"SCIP_CACHE_TTL":       "1h",
			"SCIP_QUERY_TIMEOUT":   "30s",
			"SCIP_LOG_QUERIES":     "false",
		}
		
		// Set environment variables
		for k, v := range envVars {
			t.Setenv(k, v)
		}
		
		// Load config with environment overrides
		config := indexing.LoadSCIPConfigWithEnv()
		
		// Validate overrides were applied
		if !config.CacheConfig.Enabled {
			t.Error("Cache should be enabled from env")
		}
		if config.CacheConfig.MaxSize != 5000 {
			t.Errorf("Expected cache size 5000, got %d", config.CacheConfig.MaxSize)
		}
		if config.CacheConfig.TTL != 1*time.Hour {
			t.Errorf("Expected TTL 1h, got %v", config.CacheConfig.TTL)
		}
		if config.Performance.QueryTimeout != 30*time.Second {
			t.Errorf("Expected query timeout 30s, got %v", config.Performance.QueryTimeout)
		}
		if config.Logging.LogQueries {
			t.Error("Query logging should be disabled from env")
		}
	})
}

// testResourceManagement tests memory and file handle management
func testResourceManagement(t *testing.T) {
	t.Run("MemoryLeakDetection", func(t *testing.T) {
		config := &indexing.SCIPConfig{
			CacheConfig: indexing.CacheConfig{
				Enabled: true,
				MaxSize: 100,
				TTL:     1 * time.Minute,
			},
		}
		
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		initialAlloc := memStats.Alloc
		
		// Create and destroy many instances
		for i := 0; i < 100; i++ {
			client, err := indexing.NewSCIPClient(config)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}
			
			// Simulate some operations
			store := indexing.NewMockSCIPStore(config)
			mapper := indexing.NewLSPSCIPMapper(store, config)
			
			for j := 0; j < 10; j++ {
				params := &indexing.LSPParams{
					Method: "textDocument/definition",
					TextDocument: &indexing.TextDocumentIdentifier{
						URI: fmt.Sprintf("file:///test%d.go", j),
					},
					Position: &indexing.LSPPosition{
						Line:      j,
						Character: 0,
					},
				}
				_, _ = mapper.MapDefinition(params)
			}
			
			// Clean up
			client.Close()
			mapper.Close()
			store.Close()
		}
		
		// Force GC and check memory
		runtime.GC()
		runtime.ReadMemStats(&memStats)
		finalAlloc := memStats.Alloc
		
		memoryGrowth := int64(finalAlloc) - int64(initialAlloc)
		maxAcceptableGrowth := int64(10 * 1024 * 1024) // 10MB
		
		if memoryGrowth > maxAcceptableGrowth {
			t.Errorf("Potential memory leak detected: %d bytes growth", memoryGrowth)
		}
	})
	
	t.Run("FileHandleManagement", func(t *testing.T) {
		// Test that file handles are properly closed
		config := &indexing.SCIPConfig{
			Performance: indexing.PerformanceConfig{
				IndexLoadTimeout: 5 * time.Second,
			},
		}
		
		client, err := indexing.NewSCIPClient(config)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		
		// Load multiple indices
		indices := []string{
			"/test/index1.scip",
			"/test/index2.scip",
			"/test/index3.scip",
		}
		
		for _, idx := range indices {
			_ = client.LoadIndex(idx) // Ignore errors for this test
		}
		
		// Close client and verify cleanup
		err = client.Close()
		if err != nil {
			t.Errorf("Failed to close client: %v", err)
		}
		
		// Verify stats show proper cleanup
		stats := client.GetStats()
		if stats.IndexLoads != int64(len(indices)) {
			t.Errorf("Expected %d index loads, got %d", len(indices), stats.IndexLoads)
		}
	})
	
	t.Run("CacheEvictionUnderPressure", func(t *testing.T) {
		config := &indexing.SCIPConfig{
			CacheConfig: indexing.CacheConfig{
				Enabled: true,
				MaxSize: 10, // Very small cache
				TTL:     1 * time.Hour,
			},
		}
		
		cache, err := indexing.NewPerformanceCache(&indexing.PerformanceCacheConfig{
			MaxSize:              10,
			TTL:                  1 * time.Hour,
			EvictionPolicy:       "lru",
			EnableDistributed:    false,
			EnablePreloading:     false,
			PreloadPaths:         nil,
			MonitoringInterval:   1 * time.Minute,
			CompressionEnabled:   false,
			CompressionThreshold: 1024,
		})
		if err != nil {
			t.Fatalf("Failed to create cache: %v", err)
		}
		defer cache.Cleanup()
		
		// Add more items than cache can hold
		for i := 0; i < 20; i++ {
			key := fmt.Sprintf("key-%d", i)
			value := fmt.Sprintf("value-%d", i)
			err := cache.Set(key, []byte(value), 1*time.Hour)
			if err != nil {
				t.Errorf("Failed to set cache key %s: %v", key, err)
			}
		}
		
		// Check cache size
		stats := cache.GetStats()
		if stats.Size > 10 {
			t.Errorf("Cache size exceeds limit: %d > 10", stats.Size)
		}
		
		// Verify eviction happened
		if stats.Evictions == 0 {
			t.Error("Expected evictions, got none")
		}
	})
}

// testConcurrentAccess tests thread safety and concurrent request handling
func testConcurrentAccess(t *testing.T) {
	t.Run("ConcurrentQueryHandling", func(t *testing.T) {
		config := &indexing.SCIPConfig{
			CacheConfig: indexing.CacheConfig{
				Enabled: true,
				MaxSize: 1000,
				TTL:     5 * time.Minute,
			},
			Performance: indexing.PerformanceConfig{
				MaxConcurrentQueries: 50,
				QueryTimeout:         5 * time.Second,
			},
		}
		
		store := indexing.NewMockSCIPStore(config)
		mapper := indexing.NewLSPSCIPMapper(store, config)
		defer mapper.Close()
		
		concurrency := 100
		requestsPerWorker := 50
		var wg sync.WaitGroup
		var successCount int64
		var errorCount int64
		
		// Launch concurrent workers
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				
				for j := 0; j < requestsPerWorker; j++ {
					params := &indexing.LSPParams{
						Method: "textDocument/definition",
						TextDocument: &indexing.TextDocumentIdentifier{
							URI: fmt.Sprintf("file:///worker%d/test%d.go", workerID, j),
						},
						Position: &indexing.LSPPosition{
							Line:      rand.Intn(100),
							Character: rand.Intn(80),
						},
					}
					
					_, err := mapper.MapDefinition(params)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
					} else {
						atomic.AddInt64(&successCount, 1)
					}
				}
			}(i)
		}
		
		// Wait for completion
		wg.Wait()
		
		totalRequests := int64(concurrency * requestsPerWorker)
		successRate := float64(successCount) / float64(totalRequests) * 100
		
		t.Logf("Concurrent test results: Success=%d, Errors=%d, Success Rate=%.2f%%",
			successCount, errorCount, successRate)
		
		// Should handle concurrent load with high success rate
		if successRate < 95.0 {
			t.Errorf("Success rate too low: %.2f%% < 95%%", successRate)
		}
	})
	
	t.Run("RaceConditionDetection", func(t *testing.T) {
		if !testing.Short() {
			// Run with race detector: go test -race
			config := &indexing.SCIPConfig{
				CacheConfig: indexing.CacheConfig{
					Enabled: true,
					MaxSize: 100,
					TTL:     1 * time.Minute,
				},
			}
			
			client, err := indexing.NewSCIPClient(config)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}
			defer client.Close()
			
			// Concurrent operations that might race
			var wg sync.WaitGroup
			for i := 0; i < 10; i++ {
				wg.Add(3)
				
				// Concurrent loads
				go func() {
					defer wg.Done()
					_ = client.LoadIndex("/test/index.scip")
				}()
				
				// Concurrent queries
				go func() {
					defer wg.Done()
					_, _ = client.Query("test-symbol")
				}()
				
				// Concurrent stats access
				go func() {
					defer wg.Done()
					_ = client.GetStats()
				}()
			}
			
			wg.Wait()
		}
	})
	
	t.Run("DeadlockPrevention", func(t *testing.T) {
		config := &indexing.SCIPConfig{
			Performance: indexing.PerformanceConfig{
				MaxConcurrentQueries: 5,
				QueryTimeout:         1 * time.Second,
			},
		}
		
		store := indexing.NewMockSCIPStore(config)
		mapper := indexing.NewLSPSCIPMapper(store, config)
		defer mapper.Close()
		
		// Create potential deadlock scenario
		done := make(chan bool)
		timeout := time.After(5 * time.Second)
		
		go func() {
			var wg sync.WaitGroup
			
			// Multiple goroutines trying to acquire resources
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					
					// Nested operations that could deadlock
					for j := 0; j < 5; j++ {
						params := &indexing.LSPParams{
							Method: "textDocument/references",
							TextDocument: &indexing.TextDocumentIdentifier{
								URI: fmt.Sprintf("file:///deadlock%d.go", id),
							},
							Position: &indexing.LSPPosition{
								Line:      j,
								Character: 0,
							},
						}
						_, _ = mapper.MapReferences(params)
					}
				}(i)
			}
			
			wg.Wait()
			done <- true
		}()
		
		// Check for deadlock
		select {
		case <-done:
			// Success - no deadlock
		case <-timeout:
			t.Fatal("Potential deadlock detected - operations did not complete within timeout")
		}
	})
}

// testOperationalFeatures tests monitoring, logging, and observability
func testOperationalFeatures(t *testing.T) {
	t.Run("MetricsCollection", func(t *testing.T) {
		config := &indexing.SCIPConfig{
			Logging: indexing.LoggingConfig{
				LogQueries:         true,
				LogCacheOperations: true,
				LogIndexOperations: true,
			},
		}
		
		client, err := indexing.NewSCIPClient(config)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		defer client.Close()
		
		// Perform various operations
		_ = client.LoadIndex("/test/index.scip")
		_, _ = client.Query("test-symbol")
		
		// Check metrics
		stats := client.GetStats()
		
		if stats.IndexLoads != 1 {
			t.Errorf("Expected 1 index load, got %d", stats.IndexLoads)
		}
		if stats.QueryCount != 1 {
			t.Errorf("Expected 1 query, got %d", stats.QueryCount)
		}
		if stats.ErrorCount != 2 { // Both operations should fail with test paths
			t.Errorf("Expected 2 errors, got %d", stats.ErrorCount)
		}
	})
	
	t.Run("LoggingValidation", func(t *testing.T) {
		// Test that logging doesn't expose sensitive information
		config := &indexing.SCIPConfig{
			Logging: indexing.LoggingConfig{
				LogQueries:         true,
				LogCacheOperations: true,
				LogIndexOperations: true,
			},
		}
		
		store := indexing.NewMockSCIPStore(config)
		mapper := indexing.NewLSPSCIPMapper(store, config)
		defer mapper.Close()
		
		// Query with potentially sensitive data
		params := &indexing.LSPParams{
			Method: "textDocument/definition",
			TextDocument: &indexing.TextDocumentIdentifier{
				URI: "file:///secret/api-key-12345.go",
			},
			Position: &indexing.LSPPosition{
				Line:      10,
				Character: 5,
			},
			Context: map[string]interface{}{
				"auth_token": "secret-token-value",
			},
		}
		
		_, _ = mapper.MapDefinition(params)
		
		// Verify logging (would need to capture logs in real implementation)
		// Logs should not contain "api-key-12345" or "secret-token-value"
	})
	
	t.Run("HealthCheckEndpoint", func(t *testing.T) {
		config := &indexing.SCIPConfig{
			Performance: indexing.PerformanceConfig{
				QueryTimeout: 5 * time.Second,
			},
		}
		
		client, err := indexing.NewSCIPClient(config)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		defer client.Close()
		
		// Simulate health check
		health := client.HealthCheck()
		
		if health.Status != "healthy" && health.Status != "degraded" {
			t.Errorf("Unexpected health status: %s", health.Status)
		}
		
		// Check health details
		if health.LastActivity.IsZero() {
			t.Error("Last activity time should be set")
		}
		if health.IndexCount < 0 {
			t.Error("Index count should be non-negative")
		}
	})
}

// Mock implementations for testing

type failingMockStore struct {
	failureRate float64
}

func (s *failingMockStore) LoadIndex(path string) error {
	if rand.Float64() < s.failureRate {
		return fmt.Errorf("simulated SCIP failure")
	}
	return nil
}

func (s *failingMockStore) GetDocument(uri string) (*indexing.DocumentData, error) {
	if rand.Float64() < s.failureRate {
		return nil, fmt.Errorf("simulated SCIP failure")
	}
	return &indexing.DocumentData{URI: uri}, nil
}

func (s *failingMockStore) FindSymbolAtPosition(uri string, line, character int) (*indexing.SymbolEntry, error) {
	if rand.Float64() < s.failureRate {
		return nil, fmt.Errorf("simulated SCIP failure")
	}
	return &indexing.SymbolEntry{Symbol: "test.symbol"}, nil
}

func (s *failingMockStore) FindReferences(symbol string) ([]*indexing.OccurrenceEntry, error) {
	if rand.Float64() < s.failureRate {
		return nil, fmt.Errorf("simulated SCIP failure")
	}
	return []*indexing.OccurrenceEntry{}, nil
}

func (s *failingMockStore) SearchSymbols(query string) ([]*indexing.SymbolEntry, error) {
	if rand.Float64() < s.failureRate {
		return nil, fmt.Errorf("simulated SCIP failure")
	}
	return []*indexing.SymbolEntry{}, nil
}

func (s *failingMockStore) GetStats() *indexing.StoreStats {
	return &indexing.StoreStats{}
}

func (s *failingMockStore) Close() error {
	return nil
}

type slowMockStore struct {
	delay time.Duration
}

func (s *slowMockStore) LoadIndex(path string) error {
	time.Sleep(s.delay)
	return nil
}

func (s *slowMockStore) GetDocument(uri string) (*indexing.DocumentData, error) {
	time.Sleep(s.delay)
	return &indexing.DocumentData{URI: uri}, nil
}

func (s *slowMockStore) FindSymbolAtPosition(uri string, line, character int) (*indexing.SymbolEntry, error) {
	time.Sleep(s.delay)
	return &indexing.SymbolEntry{Symbol: "test.symbol"}, nil
}

func (s *slowMockStore) FindReferences(symbol string) ([]*indexing.OccurrenceEntry, error) {
	time.Sleep(s.delay)
	return []*indexing.OccurrenceEntry{}, nil
}

func (s *slowMockStore) SearchSymbols(query string) ([]*indexing.SymbolEntry, error) {
	time.Sleep(s.delay)
	return []*indexing.SymbolEntry{}, nil
}

func (s *slowMockStore) GetStats() *indexing.StoreStats {
	return &indexing.StoreStats{}
}

func (s *slowMockStore) Close() error {
	return nil
}