package transport

import (
	"encoding/json"
	"testing"
	"time"
)

// MockSCIPIndexer implements SCIPIndexer interface for testing
type MockSCIPIndexer struct {
	indexedRequests []IndexedRequest
	enabled         bool
	closed          bool
}

type IndexedRequest struct {
	Method    string
	Params    interface{}
	Response  json.RawMessage
	RequestID string
}

func NewMockSCIPIndexer(enabled bool) *MockSCIPIndexer {
	return &MockSCIPIndexer{
		indexedRequests: make([]IndexedRequest, 0),
		enabled:         enabled,
		closed:          false,
	}
}

func (m *MockSCIPIndexer) IndexResponse(method string, params interface{}, response json.RawMessage, requestID string) {
	if !m.enabled || m.closed {
		return
	}
	
	m.indexedRequests = append(m.indexedRequests, IndexedRequest{
		Method:    method,
		Params:    params,
		Response:  response,
		RequestID: requestID,
	})
}

func (m *MockSCIPIndexer) IsEnabled() bool {
	return m.enabled && !m.closed
}

func (m *MockSCIPIndexer) Close() error {
	m.closed = true
	return nil
}

func (m *MockSCIPIndexer) GetIndexedRequests() []IndexedRequest {
	return m.indexedRequests
}

func (m *MockSCIPIndexer) Reset() {
	m.indexedRequests = m.indexedRequests[:0]
}

func TestIsCacheableMethod(t *testing.T) {
	tests := []struct {
		method   string
		expected bool
	}{
		{"textDocument/definition", true},
		{"textDocument/references", true},
		{"textDocument/documentSymbol", true},
		{"workspace/symbol", true},
		{"textDocument/hover", true},
		{"textDocument/completion", false},
		{"textDocument/didOpen", false},
		{"unknown/method", false},
	}

	for _, test := range tests {
		t.Run(test.method, func(t *testing.T) {
			result := IsCacheableMethod(test.method)
			if result != test.expected {
				t.Errorf("IsCacheableMethod(%s) = %v, expected %v", test.method, result, test.expected)
			}
		})
	}
}

func TestGetCacheableMethod(t *testing.T) {
	method, exists := GetCacheableMethod("textDocument/definition")
	if !exists {
		t.Error("Expected textDocument/definition to be cacheable")
	}

	if method.Method != "textDocument/definition" {
		t.Errorf("Expected method name 'textDocument/definition', got '%s'", method.Method)
	}

	if method.PerformanceGain != 0.87 {
		t.Errorf("Expected performance gain 0.87, got %f", method.PerformanceGain)
	}

	_, exists = GetCacheableMethod("textDocument/completion")
	if exists {
		t.Error("Expected textDocument/completion to not be cacheable")
	}
}

func TestGetAllCacheableMethods(t *testing.T) {
	methods := GetAllCacheableMethods()
	
	expectedMethods := []string{
		"textDocument/definition",
		"textDocument/references",
		"textDocument/documentSymbol",
		"workspace/symbol",
		"textDocument/hover",
	}

	if len(methods) != len(expectedMethods) {
		t.Errorf("Expected %d cacheable methods, got %d", len(expectedMethods), len(methods))
	}

	for _, expected := range expectedMethods {
		if _, exists := methods[expected]; !exists {
			t.Errorf("Expected method '%s' to be in cacheable methods", expected)
		}
	}
}

func TestNoOpSCIPIndexer(t *testing.T) {
	indexer := &NoOpSCIPIndexer{}

	// Test that all methods are safe no-ops
	indexer.IndexResponse("testDocument/definition", nil, json.RawMessage(`{"result": "test"}`), "req_1")
	
	if indexer.IsEnabled() {
		t.Error("NoOpSCIPIndexer should not be enabled")
	}

	if err := indexer.Close(); err != nil {
		t.Errorf("NoOpSCIPIndexer.Close() should not return error, got: %v", err)
	}
}

func TestSafeIndexerWrapper(t *testing.T) {
	mockIndexer := NewMockSCIPIndexer(true)
	wrapper := NewSafeIndexerWrapper(mockIndexer, 2) // Max 2 goroutines

	// Test that cacheable methods are indexed
	wrapper.SafeIndexResponse("textDocument/definition", nil, json.RawMessage(`{"result": "test"}`), "req_1")
	
	// Give goroutine time to complete
	time.Sleep(100 * time.Millisecond)

	requests := mockIndexer.GetIndexedRequests()
	if len(requests) != 1 {
		t.Errorf("Expected 1 indexed request, got %d", len(requests))
	}

	if requests[0].Method != "textDocument/definition" {
		t.Errorf("Expected method 'textDocument/definition', got '%s'", requests[0].Method)
	}

	// Test non-cacheable methods are not indexed
	mockIndexer.Reset()
	wrapper.SafeIndexResponse("textDocument/completion", nil, json.RawMessage(`{"result": "test"}`), "req_2")
	
	time.Sleep(100 * time.Millisecond)
	
	requests = mockIndexer.GetIndexedRequests()
	if len(requests) != 0 {
		t.Errorf("Expected 0 indexed requests for non-cacheable method, got %d", len(requests))
	}
}

func TestSafeIndexerWrapperDisabled(t *testing.T) {
	mockIndexer := NewMockSCIPIndexer(false) // Disabled indexer
	wrapper := NewSafeIndexerWrapper(mockIndexer, 2)

	wrapper.SafeIndexResponse("textDocument/definition", nil, json.RawMessage(`{"result": "test"}`), "req_1")
	
	time.Sleep(100 * time.Millisecond)

	requests := mockIndexer.GetIndexedRequests()
	if len(requests) != 0 {
		t.Errorf("Expected 0 indexed requests for disabled indexer, got %d", len(requests))
	}
}

func TestSafeIndexerWrapperNilIndexer(t *testing.T) {
	wrapper := NewSafeIndexerWrapper(nil, 2)

	// Should not panic or cause issues
	wrapper.SafeIndexResponse("textDocument/definition", nil, json.RawMessage(`{"result": "test"}`), "req_1")
	
	if wrapper.IsEnabled() {
		t.Error("Wrapper with nil indexer should not be enabled")
	}

	if err := wrapper.Close(); err != nil {
		t.Errorf("Wrapper.Close() with nil indexer should not return error, got: %v", err)
	}
}

func TestSafeIndexerWrapperCapacityLimit(t *testing.T) {
	mockIndexer := NewMockSCIPIndexer(true)
	wrapper := NewSafeIndexerWrapper(mockIndexer, 1) // Max 1 goroutine

	// Send multiple requests quickly to test capacity limiting
	for i := 0; i < 5; i++ {
		wrapper.SafeIndexResponse("textDocument/definition", nil, json.RawMessage(`{"result": "test"}`), "req_"+string(rune(i+'1')))
	}
	
	// Give time for processing
	time.Sleep(200 * time.Millisecond)

	requests := mockIndexer.GetIndexedRequests()
	// Due to capacity limiting, we might not get all 5 requests indexed
	// But we should get at least 1
	if len(requests) < 1 {
		t.Errorf("Expected at least 1 indexed request, got %d", len(requests))
	}
	
	// And no more than 5 (the number we sent)
	if len(requests) > 5 {
		t.Errorf("Expected at most 5 indexed requests, got %d", len(requests))
	}
}

func BenchmarkIsCacheableMethod(b *testing.B) {
	methods := []string{
		"textDocument/definition",
		"textDocument/references",
		"textDocument/completion",
		"unknown/method",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		method := methods[i%len(methods)]
		IsCacheableMethod(method)
	}
}

func BenchmarkSafeIndexerWrapper(b *testing.B) {
	mockIndexer := NewMockSCIPIndexer(true)
	wrapper := NewSafeIndexerWrapper(mockIndexer, 10)
	
	response := json.RawMessage(`{"result": {"uri": "file:///test.go", "range": {"start": {"line": 10, "character": 5}}}}`)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wrapper.SafeIndexResponse("textDocument/definition", nil, response, "req_1")
	}
}