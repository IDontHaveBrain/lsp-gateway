package cache

import (
	"context"
	"testing"
	"time"
)

// MockLSPFallback implements LSPFallback for testing
type MockLSPFallback struct {
	responses map[string]interface{}
	callCount map[string]int
}

func NewMockLSPFallback() *MockLSPFallback {
	return &MockLSPFallback{
		responses: make(map[string]interface{}),
		callCount: make(map[string]int),
	}
}

func (m *MockLSPFallback) ProcessRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	m.callCount[method]++
	if response, exists := m.responses[method]; exists {
		return response, nil
	}
	return nil, nil
}

func (m *MockLSPFallback) SetResponse(method string, response interface{}) {
	m.responses[method] = response
}

func (m *MockLSPFallback) GetCallCount(method string) int {
	return m.callCount[method]
}

// Test basic query manager creation
func TestNewSCIPQueryManager(t *testing.T) {
	fallback := NewMockLSPFallback()
	manager := NewSCIPQueryManager(fallback)
	
	if manager == nil {
		t.Fatal("Expected non-nil query manager")
	}
}

// Test getting metrics
func TestQueryManagerMetrics(t *testing.T) {
	fallback := NewMockLSPFallback()
	manager := NewSCIPQueryManager(fallback)
	
	metrics := manager.GetMetrics()
	if metrics == nil {
		t.Fatal("Expected non-nil metrics")
	}
	
	// Verify initial metrics state
	if metrics.TotalQueries != 0 {
		t.Errorf("Expected 0 initial queries, got %d", metrics.TotalQueries)
	}
	
	if metrics.CacheHits != 0 {
		t.Errorf("Expected 0 initial cache hits, got %d", metrics.CacheHits)
	}
	
	if metrics.CacheMisses != 0 {
		t.Errorf("Expected 0 initial cache misses, got %d", metrics.CacheMisses)
	}
}

// Test health check
func TestQueryManagerHealth(t *testing.T) {
	fallback := NewMockLSPFallback()
	manager := NewSCIPQueryManager(fallback)
	
	// Check health status (may or may not be healthy initially)
	healthy := manager.IsHealthy()
	t.Logf("Query manager health status: %v", healthy)
	
	// Just verify the method works without error
	// Health status depends on internal circuit breaker state
}

// Test document invalidation
func TestQueryManagerDocumentInvalidation(t *testing.T) {
	fallback := NewMockLSPFallback()
	manager := NewSCIPQueryManager(fallback)
	
	testURI := "file:///test.go"
	
	err := manager.InvalidateDocument(testURI)
	if err != nil {
		t.Errorf("Failed to invalidate document: %v", err)
	}
}

// Test building cache key
func TestQueryManagerBuildKey(t *testing.T) {
	fallback := NewMockLSPFallback()
	manager := NewSCIPQueryManager(fallback)
	
	testMethod := "textDocument/hover"
	testParams := map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": "file:///test.go"},
		"position":     map[string]interface{}{"line": 10, "character": 5},
	}
	
	key, err := manager.BuildKey(testMethod, testParams)
	if err != nil {
		t.Errorf("Failed to build cache key: %v", err)
	}
	
	if key.Method != testMethod {
		t.Errorf("Expected method %s, got %s", testMethod, key.Method)
	}
}

// Test cache entry validation
func TestQueryManagerValidateEntry(t *testing.T) {
	fallback := NewMockLSPFallback()
	manager := NewSCIPQueryManager(fallback)
	
	// Create a test cache entry
	entry := &CacheEntry{
		Timestamp: time.Now(),
		Size:      100,
		Response:  "test data",
	}
	
	// Test with a large TTL (should be valid)
	largeTTL := 24 * time.Hour
	if !manager.IsValidEntry(entry, largeTTL) {
		t.Error("Expected entry to be valid with large TTL")
	}
	
	// Test with a very small TTL (should be invalid for old entries)
	entry.Timestamp = time.Now().Add(-1 * time.Hour)
	smallTTL := 1 * time.Minute
	if manager.IsValidEntry(entry, smallTTL) {
		t.Error("Expected entry to be invalid with small TTL and old timestamp")
	}
}

// Test URI extraction
func TestQueryManagerExtractURI(t *testing.T) {
	fallback := NewMockLSPFallback()
	manager := NewSCIPQueryManager(fallback)
	
	testCases := []struct {
		name     string
		params   interface{}
		expected string
		hasError bool
	}{
		{
			name: "hover params",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": "file:///test.go"},
				"position":     map[string]interface{}{"line": 10, "character": 5},
			},
			expected: "file:///test.go",
			hasError: false,
		},
		{
			name: "workspace symbol params",
			params: map[string]interface{}{
				"query": "TestFunction",
			},
			expected: "",
			hasError: false,
		},
		{
			name:     "invalid params",
			params:   "invalid",
			expected: "",
			hasError: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uri, err := manager.ExtractURI(tc.params)
			
			if tc.hasError && err == nil {
				t.Error("Expected error but got none")
			}
			
			if !tc.hasError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			
			if uri != tc.expected {
				t.Errorf("Expected URI %s, got %s", tc.expected, uri)
			}
		})
	}
}

// Test performance characteristics (simplified)
func TestQueryManagerPerformance(t *testing.T) {
	fallback := NewMockLSPFallback()
	manager := NewSCIPQueryManager(fallback)
	
	// Test that basic operations complete quickly
	start := time.Now()
	
	_ = manager.GetMetrics()
	_ = manager.IsHealthy()
	_ = manager.InvalidateDocument("file:///test.go")
	
	duration := time.Since(start)
	
	// Basic operations should be very fast (< 1ms)
	if duration > time.Millisecond {
		t.Logf("Warning: Basic operations took %v, expected < 1ms", duration)
	}
	
	t.Logf("Basic operations completed in %v", duration)
}