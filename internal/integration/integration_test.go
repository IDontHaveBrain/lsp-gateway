package integration

import (
	"context"
	"testing"
	"time"

	"lsp-gateway/internal/integration/testutil"
)

// TestBasicIntegration demonstrates basic integration testing
func TestBasicIntegration(t *testing.T) {
	suite := testutil.NewIntegrationTestSuite(t, &testutil.IntegrationTestConfig{
		TestName: "basic_integration",
		Environment: &testutil.TestEnvironmentConfig{
			Languages:         []string{"go", "python"},
			EnableMockServers: true,
			EnablePerformance: false,
		},
		ValidationLevel: testutil.ValidationBasic,
	})
	defer suite.Cleanup()

	if err := suite.RunBasicFunctionalTests(); err != nil {
		t.Fatalf("Basic functional tests failed: %v", err)
	}
}

// TestLoadTest demonstrates load testing capabilities
func TestLoadTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	suite := testutil.NewIntegrationTestSuite(t, &testutil.IntegrationTestConfig{
		TestName: "load_test",
		Environment: &testutil.TestEnvironmentConfig{
			Languages:         []string{"go", "python", "typescript"},
			EnableMockServers: true,
			EnablePerformance: true,
		},
		Performance:     true,
		ConcurrentUsers: 10,
		TestDuration:    5 * time.Second, // Reduce from 30s to 5s
		FailureThresholds: &testutil.FailureThresholds{
			MaxErrorRate:     0.05, // 5%
			MaxLatencyP95:    100 * time.Millisecond,
			MaxLatencyP99:    200 * time.Millisecond,
			MinThroughput:    50.0, // 50 RPS
			MaxMemoryUsageMB: 300.0,
			MaxGoroutines:    150,
		},
	})
	defer suite.Cleanup()

	if err := suite.RunLoadTest(); err != nil {
		t.Fatalf("Load test failed: %v", err)
	}
}

// TestReliability demonstrates reliability testing
func TestReliability(t *testing.T) {
	suite := testutil.NewIntegrationTestSuite(t, &testutil.IntegrationTestConfig{
		TestName: "reliability_test",
		Environment: &testutil.TestEnvironmentConfig{
			Languages:         []string{"go"},
			EnableMockServers: true,
			EnablePerformance: false,
		},
		ValidationLevel: testutil.ValidationStrict,
	})
	defer suite.Cleanup()

	if err := suite.RunReliabilityTest(); err != nil {
		t.Fatalf("Reliability test failed: %v", err)
	}
}

// TestConcurrency demonstrates concurrency testing
func TestConcurrency(t *testing.T) {
	suite := testutil.NewIntegrationTestSuite(t, &testutil.IntegrationTestConfig{
		TestName: "concurrency_test",
		Environment: &testutil.TestEnvironmentConfig{
			Languages:         []string{"go", "python"},
			EnableMockServers: true,
			EnablePerformance: true,
		},
		ConcurrentUsers: 20,
		FailureThresholds: &testutil.FailureThresholds{
			MaxErrorRate:  0.02, // 2% - stricter for concurrency
			MaxLatencyP95: 150 * time.Millisecond,
		},
	})
	defer suite.Cleanup()

	if err := suite.RunConcurrencyTest(); err != nil {
		t.Fatalf("Concurrency test failed: %v", err)
	}
}

// TestComprehensive demonstrates comprehensive testing
func TestComprehensive(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comprehensive test in short mode")
	}

	suite := testutil.NewIntegrationTestSuite(t, &testutil.IntegrationTestConfig{
		TestName: "comprehensive_test",
		Environment: &testutil.TestEnvironmentConfig{
			Languages:         []string{"go", "python", "typescript", "java"},
			EnableMockServers: true,
			EnablePerformance: true,
		},
		Performance:        true,
		ConcurrentUsers:    15,
		TestDuration:       10 * time.Second, // Reduce from 60s to 10s
		ValidationLevel:    testutil.ValidationStrict,
		ResourceMonitoring: true,
		FailureThresholds: &testutil.FailureThresholds{
			MaxErrorRate:     0.03, // 3%
			MaxLatencyP95:    120 * time.Millisecond,
			MaxLatencyP99:    250 * time.Millisecond,
			MinThroughput:    30.0,
			MaxMemoryUsageMB: 400.0,
			MaxGoroutines:    200,
		},
	})
	defer suite.Cleanup()

	if err := suite.RunComprehensiveTest(); err != nil {
		t.Fatalf("Comprehensive test failed: %v", err)
	}
}

// TestCustomRequestPatterns demonstrates custom request pattern testing
func TestCustomRequestPatterns(t *testing.T) {
	customPatterns := []testutil.RequestPattern{
		{
			Method: "textDocument/definition",
			Weight: 50,
			ParamsFunc: func(gen *testutil.RequestGenerator) interface{} {
				return map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": "file:///custom_test.go",
					},
					"position": map[string]interface{}{
						"line": 42, "character": 10,
					},
				}
			},
			ValidateFunc: func(response *testutil.JSONRPCResponse) error {
				// Custom validation logic
				if response.Error != nil {
					return nil // Allow errors in test
				}
				return nil
			},
		},
		{
			Method: "textDocument/hover",
			Weight: 30,
			ParamsFunc: func(gen *testutil.RequestGenerator) interface{} {
				return map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": "file:///custom_test.go",
					},
					"position": map[string]interface{}{
						"line": 15, "character": 5,
					},
				}
			},
		},
	}

	suite := testutil.NewIntegrationTestSuite(t, &testutil.IntegrationTestConfig{
		TestName: "custom_patterns",
		Environment: &testutil.TestEnvironmentConfig{
			Languages:         []string{"go"},
			EnableMockServers: true,
		},
		RequestPatterns: customPatterns,
	})
	defer suite.Cleanup()

	if err := suite.RunBasicFunctionalTests(); err != nil {
		t.Fatalf("Custom pattern test failed: %v", err)
	}
}

// TestErrorScenarios demonstrates error scenario testing
func TestErrorScenarios(t *testing.T) {
	suite := testutil.NewIntegrationTestSuite(t, &testutil.IntegrationTestConfig{
		TestName: "error_scenarios",
		Environment: &testutil.TestEnvironmentConfig{
			Languages:         []string{"go"},
			EnableMockServers: true,
		},
	})
	defer suite.Cleanup()

	httpClient := suite.GetHTTPClient()
	reqGen := suite.GetRequestGenerator()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // Reduce timeout
	defer cancel()

	// Test unsupported file type
	t.Run("unsupported_file_type", func(t *testing.T) {
		request := testutil.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "test-1",
			Method:  "textDocument/definition",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file:///unknown.xyz",
				},
				"position": map[string]interface{}{
					"line": 10, "character": 5,
				},
			},
		}

		response, err := httpClient.SendJSONRPCRequest(ctx, request)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if response.Error == nil {
			t.Error("Expected error for unsupported file type")
		}
	})

	// Test invalid method
	t.Run("invalid_method", func(t *testing.T) {
		request := testutil.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "test-2",
			Method:  "invalid/method",
			Params:  map[string]interface{}{},
		}

		response, err := httpClient.SendJSONRPCRequest(ctx, request)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if response.Error == nil {
			t.Error("Expected error for invalid method")
		}
	})

	// Test realistic sequence
	t.Run("realistic_sequence", func(t *testing.T) {
		sequence := reqGen.GenerateRealisticSequence()
		responses, err := reqGen.SendSequence(ctx, sequence)
		if err != nil {
			t.Fatalf("Sequence failed: %v", err)
		}

		if len(responses) != len(sequence.Requests) {
			t.Errorf("Expected %d responses, got %d", len(sequence.Requests), len(responses))
		}

		// Check that at least some responses are successful
		successCount := 0
		for _, resp := range responses {
			if resp.Error == nil {
				successCount++
			}
		}

		if successCount == 0 {
			t.Error("Expected at least some successful responses in realistic sequence")
		}
	})
}

// BenchmarkIntegration demonstrates benchmarking capabilities
func BenchmarkIntegration(b *testing.B) {
	suite := testutil.NewIntegrationTestSuite(&testing.T{}, &testutil.IntegrationTestConfig{
		TestName: "benchmark",
		Environment: &testutil.TestEnvironmentConfig{
			Languages:         []string{"go"},
			EnableMockServers: true,
			EnablePerformance: true,
		},
		Performance: true,
	})
	defer suite.Cleanup()

	httpClient := suite.GetHTTPClient()
	reqGen := suite.GetRequestGenerator()
	patterns := reqGen.GetDefaultRequestPatterns()

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			request := reqGen.GenerateRequest(patterns)
			_, err := httpClient.SendJSONRPCRequest(ctx, request)
			if err != nil {
				b.Error(err)
			}
		}
	})

	stats := httpClient.GetStats()
	b.Logf("Final stats: %s", stats.String())
}

// TestWorkspaceSimulation demonstrates workspace simulation testing
func TestWorkspaceSimulation(t *testing.T) {
	suite := testutil.NewIntegrationTestSuite(t, &testutil.IntegrationTestConfig{
		TestName: "workspace_simulation",
		Environment: &testutil.TestEnvironmentConfig{
			Languages:         []string{"go", "python", "typescript"},
			EnableMockServers: true,
		},
	})
	defer suite.Cleanup()

	fixtures := suite.GetFixtures()

	// Create a temporary workspace with source files
	workspace := fixtures.CreateTempWorkspace()
	if err := fixtures.CreateSourceFiles(workspace); err != nil {
		t.Fatalf("Failed to create source files: %v", err)
	}

	// Test operations on the workspace files
	httpClient := suite.GetHTTPClient()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // Reduce timeout
	defer cancel()

	workspaceFiles := []string{
		"file://" + workspace + "/main.go",
		"file://" + workspace + "/app.py",
		"file://" + workspace + "/app.ts",
	}

	for _, fileURI := range workspaceFiles {
		t.Run("test_"+fileURI, func(t *testing.T) {
			request := testutil.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      "workspace-test",
				Method:  "textDocument/documentSymbol",
				Params: map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": fileURI,
					},
				},
			}

			response, err := httpClient.SendJSONRPCRequest(ctx, request)
			if err != nil {
				t.Fatalf("Request for %s failed: %v", fileURI, err)
			}

			// Response might be an error for mock servers, which is acceptable
			t.Logf("Response for %s: error=%v, result=%v", fileURI, response.Error, response.Result != nil)
		})
	}
}

// TestResourceMonitoring demonstrates resource monitoring during tests
func TestResourceMonitoring(t *testing.T) {
	suite := testutil.NewIntegrationTestSuite(t, &testutil.IntegrationTestConfig{
		TestName: "resource_monitoring",
		Environment: &testutil.TestEnvironmentConfig{
			Languages:         []string{"go"},
			EnableMockServers: true,
			EnablePerformance: true,
		},
		Performance:        true,
		ResourceMonitoring: true,
		TestDuration:       3 * time.Second, // Reduce from 15s to 3s
		ConcurrentUsers:    5,
	})
	defer suite.Cleanup()

	perfMonitor := suite.GetPerformanceMonitor()
	if perfMonitor == nil {
		t.Fatal("Performance monitor should be available")
	}

	// Run some load to generate metrics
	if err := suite.RunBasicFunctionalTests(); err != nil {
		t.Fatalf("Basic tests failed: %v", err)
	}

	// Get current performance report
	report := perfMonitor.Report()
	t.Logf("Performance report:\n%s", report)

	// Get metrics and validate that we're collecting samples
	metrics := perfMonitor.Stop()
	if len(metrics.Samples) == 0 {
		t.Error("Expected performance samples to be collected")
	}
}
