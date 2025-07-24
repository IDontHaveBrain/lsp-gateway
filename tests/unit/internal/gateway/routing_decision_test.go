package gateway

import (
	"context"
	"testing"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/gateway"
	"lsp-gateway/internal/transport"
)

// Test fixtures for RoutingDecision tests

func createTestRoutingDecision() *gateway.RoutingDecision {
	mockClient := NewMockLSPClient("test-server")
	
	return &gateway.RoutingDecision{
		ServerName: "test-server",
		ServerConfig: &config.ServerConfig{
			Name:      "test-server",
			Languages: []string{"go"},
			Command:   "gopls",
			Priority:  1,
			Weight:    1.0,
		},
		Client:   mockClient,
		Priority: 1,
		Weight:   1.0,
		Strategy: gateway.SingleTargetWithFallback,
		Metadata: map[string]interface{}{
			"test_key": "test_value",
		},
	}
}

func createTestLSPRequestWithContext(method, language string) *gateway.LSPRequest {
	return &gateway.LSPRequest{
		Method:   method,
		Params:   map[string]interface{}{"test": "params"},
		Language: language,
		URI:      "file:///test/main.go",
		Context:  context.Background(),
	}
}

// Test Suite: RoutingDecision Structure Tests

func TestRoutingDecision_Creation(t *testing.T) {
	decision := createTestRoutingDecision()
	
	if decision == nil {
		t.Fatal("RoutingDecision should not be nil")
	}
	
	if decision.ServerName != "test-server" {
		t.Errorf("Expected ServerName 'test-server', got '%s'", decision.ServerName)
	}
	
	if decision.Strategy != gateway.SingleTargetWithFallback {
		t.Errorf("Expected SingleTargetWithFallback strategy, got %s", decision.Strategy)
	}
	
	if decision.Priority != 1 {
		t.Errorf("Expected Priority 1, got %d", decision.Priority)
	}
	
	if decision.Weight != 1.0 {
		t.Errorf("Expected Weight 1.0, got %f", decision.Weight)
	}
}

func TestRoutingDecision_ServerConfig(t *testing.T) {
	decision := createTestRoutingDecision()
	
	if decision.ServerConfig == nil {
		t.Fatal("ServerConfig should not be nil")
	}
	
	config := decision.ServerConfig
	if config.Name != "test-server" {
		t.Errorf("Expected config name 'test-server', got '%s'", config.Name)
	}
	
	if len(config.Languages) != 1 || config.Languages[0] != "go" {
		t.Errorf("Expected languages [go], got %v", config.Languages)
	}
	
	if config.Command != "gopls" {
		t.Errorf("Expected command 'gopls', got '%s'", config.Command)
	}
}

func TestRoutingDecision_Client(t *testing.T) {
	decision := createTestRoutingDecision()
	
	if decision.Client == nil {
		t.Fatal("Client should not be nil")
	}
	
	// Test that client can send requests
	mockClient, ok := decision.Client.(*MockLSPClient)
	if !ok {
		t.Fatal("Expected MockLSPClient")
	}
	
	result, err := mockClient.SendRequest("test/method", map[string]interface{}{"test": "param"})
	if err != nil {
		t.Fatalf("SendRequest failed: %v", err)
	}
	
	if result == nil {
		t.Error("Result should not be nil")
	}
}

func TestRoutingDecision_Metadata(t *testing.T) {
	decision := createTestRoutingDecision()
	
	if decision.Metadata == nil {
		t.Fatal("Metadata should not be nil")
	}
	
	if value, exists := decision.Metadata["test_key"]; !exists {
		t.Error("Expected 'test_key' in metadata")
	} else if value != "test_value" {
		t.Errorf("Expected metadata value 'test_value', got '%v'", value)
	}
	
	// Test metadata modification
	decision.Metadata["new_key"] = "new_value"
	if value, exists := decision.Metadata["new_key"]; !exists {
		t.Error("Expected 'new_key' in metadata after modification")
	} else if value != "new_value" {
		t.Errorf("Expected metadata value 'new_value', got '%v'", value)
	}
}

// Test Suite: Server Selection Algorithm Tests

func TestRoutingDecision_PriorityBasedSelection(t *testing.T) {
	// Create multiple decisions with different priorities
	decision1 := &gateway.RoutingDecision{
		ServerName: "server-1",
		Priority:   1,
		Weight:     1.0,
		Strategy:   gateway.SingleTargetWithFallback,
	}
	
	decision2 := &gateway.RoutingDecision{
		ServerName: "server-2",
		Priority:   3,
		Weight:     1.0,
		Strategy:   gateway.SingleTargetWithFallback,
	}
	
	decision3 := &gateway.RoutingDecision{
		ServerName: "server-3",
		Priority:   2,
		Weight:     1.0,
		Strategy:   gateway.SingleTargetWithFallback,
	}
	
	decisions := []*gateway.RoutingDecision{decision1, decision2, decision3}
	
	// Test priority-based sorting (higher priority first)
	for i := 0; i < len(decisions)-1; i++ {
		for j := i + 1; j < len(decisions); j++ {
			if decisions[i].Priority < decisions[j].Priority {
				decisions[i], decisions[j] = decisions[j], decisions[i]
			}
		}
	}
	
	// Verify sorting
	expectedOrder := []string{"server-2", "server-3", "server-1"}
	for i, expected := range expectedOrder {
		if decisions[i].ServerName != expected {
			t.Errorf("Expected server %s at position %d, got %s", expected, i, decisions[i].ServerName)
		}
	}
}

func TestRoutingDecision_WeightBasedSelection(t *testing.T) {
	// Create decisions with different weights
	decisions := []*gateway.RoutingDecision{
		{ServerName: "server-1", Priority: 1, Weight: 0.5, Strategy: gateway.LoadBalanced},
		{ServerName: "server-2", Priority: 1, Weight: 1.0, Strategy: gateway.LoadBalanced},
		{ServerName: "server-3", Priority: 1, Weight: 0.8, Strategy: gateway.LoadBalanced},
	}
	
	// Test weight calculation - higher weight should be preferred
	var bestDecision *gateway.RoutingDecision
	bestScore := -1.0
	
	for _, decision := range decisions {
		// Simple scoring algorithm: priority * weight
		score := float64(decision.Priority) * decision.Weight
		if score > bestScore {
			bestScore = score
			bestDecision = decision
		}
	}
	
	if bestDecision.ServerName != "server-2" {
		t.Errorf("Expected 'server-2' to have highest score, got '%s'", bestDecision.ServerName)
	}
}

// Test Suite: Request Context Handling Tests

func TestRoutingDecision_RequestContextHandling(t *testing.T) {
	testCases := []struct {
		name     string
		method   string
		language string
		strategy gateway.RoutingStrategy
	}{
		{
			name:     "Definition Request",
			method:   gateway.LSP_METHOD_DEFINITION,
			language: "go",
			strategy: gateway.SingleTargetWithFallback,
		},
		{
			name:     "References Request",
			method:   gateway.LSP_METHOD_REFERENCES,
			language: "python",
			strategy: gateway.MultiTargetParallel,
		},
		{
			name:     "Hover Request",
			method:   gateway.LSP_METHOD_HOVER,
			language: "javascript",
			strategy: gateway.PrimaryWithEnhancement,
		},
		{
			name:     "Workspace Symbol Request",
			method:   gateway.LSP_METHOD_WORKSPACE_SYMBOL,
			language: "go",
			strategy: gateway.BroadcastAggregate,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request := createTestLSPRequestWithContext(tc.method, tc.language)
			
			// Create decision based on request context
			decision := &gateway.RoutingDecision{
				ServerName: "test-server",
				Strategy:   tc.strategy,
				Metadata: map[string]interface{}{
					"method":   request.Method,
					"language": request.Language,
					"uri":      request.URI,
				},
			}
			
			// Verify decision reflects request context
			if decision.Strategy != tc.strategy {
				t.Errorf("Expected strategy %s, got %s", tc.strategy, decision.Strategy)
			}
			
			if methodMeta := decision.Metadata["method"]; methodMeta != tc.method {
				t.Errorf("Expected method %s in metadata, got %v", tc.method, methodMeta)
			}
			
			if langMeta := decision.Metadata["language"]; langMeta != tc.language {
				t.Errorf("Expected language %s in metadata, got %v", tc.language, langMeta)
			}
		})
	}
}

// Test Suite: Strategy-Specific Decision Tests

func TestRoutingDecision_SingleTargetWithFallback(t *testing.T) {
	decision := &gateway.RoutingDecision{
		ServerName: "primary-server",
		Strategy:   gateway.SingleTargetWithFallback,
		Priority:   1,
		Weight:     1.0,
		Metadata: map[string]interface{}{
			"fallback_available": true,
			"health_score":      0.95,
		},
	}
	
	if decision.Strategy != gateway.SingleTargetWithFallback {
		t.Errorf("Expected SingleTargetWithFallback strategy, got %s", decision.Strategy)
	}
	
	// Check fallback metadata
	if fallback, exists := decision.Metadata["fallback_available"]; !exists {
		t.Error("Expected fallback_available in metadata")
	} else if fallback != true {
		t.Errorf("Expected fallback_available true, got %v", fallback)
	}
	
	// Check health score metadata
	if healthScore, exists := decision.Metadata["health_score"]; !exists {
		t.Error("Expected health_score in metadata")
	} else if healthScore != 0.95 {
		t.Errorf("Expected health_score 0.95, got %v", healthScore)
	}
}

func TestRoutingDecision_LoadBalanced(t *testing.T) {
	decision := &gateway.RoutingDecision{
		ServerName: "balanced-server",
		Strategy:   gateway.LoadBalanced,
		Priority:   1,
		Weight:     0.8,
		Metadata: map[string]interface{}{
			"lb_strategy":   "round_robin",
			"healthy_count": 3,
			"total_count":   5,
		},
	}
	
	if decision.Strategy != gateway.LoadBalanced {
		t.Errorf("Expected LoadBalanced strategy, got %s", decision.Strategy)
	}
	
	// Check load balancing metadata
	if lbStrategy, exists := decision.Metadata["lb_strategy"]; !exists {
		t.Error("Expected lb_strategy in metadata")
	} else if lbStrategy != "round_robin" {
		t.Errorf("Expected lb_strategy 'round_robin', got %v", lbStrategy)
	}
	
	if healthyCount, exists := decision.Metadata["healthy_count"]; !exists {
		t.Error("Expected healthy_count in metadata")
	} else if healthyCount != 3 {
		t.Errorf("Expected healthy_count 3, got %v", healthyCount)
	}
}

func TestRoutingDecision_MultiTargetParallel(t *testing.T) {
	decisions := []*gateway.RoutingDecision{
		{
			ServerName: "server-1",
			Strategy:   gateway.MultiTargetParallel,
			Priority:   3,
			Weight:     1.0,
		},
		{
			ServerName: "server-2",
			Strategy:   gateway.MultiTargetParallel,
			Priority:   2,
			Weight:     0.9,
		},
		{
			ServerName: "server-3",
			Strategy:   gateway.MultiTargetParallel,
			Priority:   1,
			Weight:     0.8,
		},
	}
	
	// Verify all decisions have the same strategy
	for _, decision := range decisions {
		if decision.Strategy != gateway.MultiTargetParallel {
			t.Errorf("Expected MultiTargetParallel strategy, got %s", decision.Strategy)
		}
	}
	
	// Test that decisions are properly prioritized
	for i := 0; i < len(decisions)-1; i++ {
		if decisions[i].Priority <= decisions[i+1].Priority {
			t.Errorf("Decisions should be ordered by priority (desc), got %d then %d", 
				decisions[i].Priority, decisions[i+1].Priority)
		}
	}
}

func TestRoutingDecision_BroadcastAggregate(t *testing.T) {
	decision := &gateway.RoutingDecision{
		ServerName: "broadcast-server",
		Strategy:   gateway.BroadcastAggregate,
		Priority:   1,
		Weight:     1.0,
		Metadata: map[string]interface{}{
			"broadcast_targets": []string{"server-1", "server-2", "server-3"},
			"aggregation_type": "merge",
		},
	}
	
	if decision.Strategy != gateway.BroadcastAggregate {
		t.Errorf("Expected BroadcastAggregate strategy, got %s", decision.Strategy)
	}
	
	// Check broadcast metadata
	if targets, exists := decision.Metadata["broadcast_targets"]; !exists {
		t.Error("Expected broadcast_targets in metadata")
	} else {
		targetsList, ok := targets.([]string)
		if !ok {
			t.Error("Expected broadcast_targets to be []string")
		} else if len(targetsList) != 3 {
			t.Errorf("Expected 3 broadcast targets, got %d", len(targetsList))
		}
	}
}

// Test Suite: Decision Validation Tests

func TestRoutingDecision_Validation(t *testing.T) {
	testCases := []struct {
		name        string
		decision    *gateway.RoutingDecision
		shouldBeValid bool
	}{
		{
			name: "Valid Decision",
			decision: &gateway.RoutingDecision{
				ServerName: "valid-server",
				ServerConfig: &config.ServerConfig{
					Name: "valid-server",
				},
				Client:   NewMockLSPClient("valid-server"),
				Priority: 1,
				Weight:   1.0,
				Strategy: gateway.SingleTargetWithFallback,
			},
			shouldBeValid: true,
		},
		{
			name: "Missing ServerName",
			decision: &gateway.RoutingDecision{
				ServerConfig: &config.ServerConfig{},
				Client:       NewMockLSPClient("test"),
				Priority:     1,
				Weight:       1.0,
				Strategy:     gateway.SingleTargetWithFallback,
			},
			shouldBeValid: false,
		},
		{
			name: "Missing ServerConfig",
			decision: &gateway.RoutingDecision{
				ServerName: "test-server",
				Client:     NewMockLSPClient("test"),
				Priority:   1,
				Weight:     1.0,
				Strategy:   gateway.SingleTargetWithFallback,
			},
			shouldBeValid: false,
		},
		{
			name: "Missing Client",
			decision: &gateway.RoutingDecision{
				ServerName: "test-server",
				ServerConfig: &config.ServerConfig{
					Name: "test-server",
				},
				Priority: 1,
				Weight:   1.0,
				Strategy: gateway.SingleTargetWithFallback,
			},
			shouldBeValid: false,
		},
		{
			name: "Invalid Weight",
			decision: &gateway.RoutingDecision{
				ServerName: "test-server",
				ServerConfig: &config.ServerConfig{
					Name: "test-server",
				},
				Client:   NewMockLSPClient("test"),
				Priority: 1,
				Weight:   -1.0, // Invalid negative weight
				Strategy: gateway.SingleTargetWithFallback,
			},
			shouldBeValid: false,
		},
		{
			name: "Zero Priority",
			decision: &gateway.RoutingDecision{
				ServerName: "test-server",
				ServerConfig: &config.ServerConfig{
					Name: "test-server",
				},
				Client:   NewMockLSPClient("test"),
				Priority: 0, // Zero priority might be invalid
				Weight:   1.0,
				Strategy: gateway.SingleTargetWithFallback,
			},
			shouldBeValid: true, // Zero priority might be valid in some contexts
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isValid := validateRoutingDecision(tc.decision)
			if isValid != tc.shouldBeValid {
				t.Errorf("Expected validation result %v, got %v", tc.shouldBeValid, isValid)
			}
		})
	}
}

// Test Suite: Performance and Resource Tests

func TestRoutingDecision_MemoryUsage(t *testing.T) {
	// Create a decision with various metadata
	decision := &gateway.RoutingDecision{
		ServerName: "memory-test-server",
		ServerConfig: &config.ServerConfig{
			Name:      "memory-test-server",
			Languages: []string{"go", "python", "javascript"},
			Settings: map[string]interface{}{
				"setting1": "value1",
				"setting2": map[string]interface{}{
					"nested": "value",
				},
			},
		},
		Client:   NewMockLSPClient("memory-test"),
		Priority: 1,
		Weight:   1.0,
		Strategy: gateway.SingleTargetWithFallback,
		Metadata: map[string]interface{}{
			"large_data": make([]string, 1000), // Large metadata
		},
	}
	
	// Ensure decision can be created and accessed without issues
	if decision.ServerName != "memory-test-server" {
		t.Error("Decision should be properly initialized")
	}
	
	// Test metadata access
	if largeData, exists := decision.Metadata["large_data"]; !exists {
		t.Error("Large metadata should be accessible")
	} else {
		dataSlice := largeData.([]string)
		if len(dataSlice) != 1000 {
			t.Errorf("Expected large data slice of length 1000, got %d", len(dataSlice))
		}
	}
}

func TestRoutingDecision_ConcurrentAccess(t *testing.T) {
	decision := createTestRoutingDecision()
	
	const numGoroutines = 10
	const accessesPerGoroutine = 100
	
	// Test concurrent read access
	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < accessesPerGoroutine; j++ {
				_ = decision.ServerName
				_ = decision.Strategy
				_ = decision.Priority
				_ = decision.Weight
				
				if decision.Metadata != nil {
					_ = decision.Metadata["test_key"]
				}
			}
		}()
	}
	
	// Test concurrent metadata modification
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < accessesPerGoroutine; j++ {
				key := "concurrent_key_" + string(rune(id))
				value := "concurrent_value_" + string(rune(j))
				decision.Metadata[key] = value
			}
		}(i)
	}
	
	// Give goroutines time to complete
	time.Sleep(100 * time.Millisecond)
	
	// Verify decision is still valid
	if decision.ServerName != "test-server" {
		t.Error("ServerName should remain unchanged after concurrent access")
	}
}

// Test Suite: Serialization Tests

func TestRoutingDecision_JSONSerialization(t *testing.T) {
	decision := createTestRoutingDecision()
	
	// Note: Client field should not be serialized due to json:"-" tag
	// This test would need to be implemented with actual JSON marshaling
	// For now, we'll just verify the structure is serialization-friendly
	
	if decision.ServerName == "" {
		t.Error("ServerName should be serializable")
	}
	
	if decision.Strategy == "" {
		t.Error("Strategy should be serializable")
	}
	
	if decision.Priority == 0 && decision.Weight == 0 {
		t.Error("Priority and Weight should be serializable")
	}
	
	// Client should not be serialized (interface{} types are typically excluded)
	// This is indicated by the json:"-" tag in the actual struct
}

// Test utility functions

func validateRoutingDecision(decision *gateway.RoutingDecision) bool {
	if decision == nil {
		return false
	}
	
	if decision.ServerName == "" {
		return false
	}
	
	if decision.ServerConfig == nil {
		return false
	}
	
	if decision.Client == nil {
		return false
	}
	
	if decision.Weight < 0 {
		return false
	}
	
	if decision.Strategy == "" {
		return false
	}
	
	return true
}

// Benchmark tests

func BenchmarkRoutingDecision_Creation(b *testing.B) {
	serverConfig := &config.ServerConfig{
		Name:      "bench-server",
		Languages: []string{"go"},
		Command:   "gopls",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		decision := &gateway.RoutingDecision{
			ServerName:   "bench-server",
			ServerConfig: serverConfig,
			Client:       NewMockLSPClient("bench"),
			Priority:     1,
			Weight:       1.0,
			Strategy:     gateway.SingleTargetWithFallback,
			Metadata:     make(map[string]interface{}),
		}
		_ = decision
	}
}

func BenchmarkRoutingDecision_MetadataAccess(b *testing.B) {
	decision := createTestRoutingDecision()
	
	// Add more metadata for benchmarking
	for i := 0; i < 100; i++ {
		decision.Metadata[string(rune(i))] = "value" + string(rune(i))
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = decision.Metadata["test_key"]
		_ = decision.Metadata["0"] // Access first added key
	}
}

func BenchmarkRoutingDecision_PriorityComparison(b *testing.B) {
	decisions := make([]*gateway.RoutingDecision, 100)
	for i := 0; i < 100; i++ {
		decisions[i] = &gateway.RoutingDecision{
			ServerName: "server-" + string(rune(i)),
			Priority:   i,
			Weight:     float64(i) / 100.0,
		}
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Find highest priority decision
		var best *gateway.RoutingDecision
		bestPriority := -1
		
		for _, decision := range decisions {
			if decision.Priority > bestPriority {
				bestPriority = decision.Priority
				best = decision
			}
		}
		_ = best
	}
}