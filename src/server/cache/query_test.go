package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"lsp-gateway/src/internal/models/lsp"
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

// Test helper functions

func createTestSymbol(name, uri string, line, char int, kind lsp.SymbolKind) *lsp.SymbolInformation {
	return &lsp.SymbolInformation{
		Name: name,
		Kind: kind,
		Location: lsp.Location{
			URI: uri,
			Range: lsp.Range{
				Start: lsp.Position{Line: line, Character: char},
				End:   lsp.Position{Line: line, Character: char + len(name)},
			},
		},
	}
}

func createTestDocumentSymbol(name string, line, char int, kind lsp.SymbolKind) *lsp.DocumentSymbol {
	return &lsp.DocumentSymbol{
		Name: name,
		Kind: kind,
		Range: lsp.Range{
			Start: lsp.Position{Line: line, Character: char},
			End:   lsp.Position{Line: line, Character: char + len(name)},
		},
		SelectionRange: lsp.Range{
			Start: lsp.Position{Line: line, Character: char},
			End:   lsp.Position{Line: line, Character: char + len(name)},
		},
	}
}

// Performance tests

func TestSCIPQueryManager_DefinitionPerformance(t *testing.T) {
	fallback := NewMockLSPFallback()
	queryManager := NewSCIPQueryManager(fallback)

	// Setup test data
	uri := "file:///test.go"
	testSymbol := createTestSymbol("TestFunction", uri, 10, 5, lsp.Function)

	err := queryManager.UpdateIndex(uri, []*lsp.SymbolInformation{testSymbol}, nil)
	if err != nil {
		t.Fatalf("Failed to update index: %v", err)
	}

	// Test parameters
	params := &lsp.DefinitionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: uri},
			Position:     lsp.Position{Line: 10, Character: 5},
		},
	}

	// Performance test - should be under 1ms
	iterations := 1000
	start := time.Now()

	for i := 0; i < iterations; i++ {
		_, err := queryManager.GetDefinition(context.Background(), params)
		if err != nil {
			t.Fatalf("GetDefinition failed: %v", err)
		}
	}

	duration := time.Since(start)
	avgDuration := duration / time.Duration(iterations)

	t.Logf("Average definition lookup time: %v", avgDuration)

	// Verify performance requirement: < 1ms per query
	if avgDuration > time.Millisecond {
		t.Errorf("Definition lookup too slow: %v > 1ms", avgDuration)
	}

	// Verify no fallback calls were made (all hits from cache)
	if fallback.GetCallCount("textDocument/definition") > 0 {
		t.Errorf("Expected no fallback calls, got %d", fallback.GetCallCount("textDocument/definition"))
	}
}

func TestSCIPQueryManager_ReferencesPerformance(t *testing.T) {
	fallback := NewMockLSPFallback()
	queryManager := NewSCIPQueryManager(fallback)

	// Setup test data with references
	uri := "file:///test.go"
	testSymbol := createTestSymbol("TestFunction", uri, 10, 5, lsp.Function)

	// Create reference locations
	refs := []*lsp.Location{
		{URI: uri, Range: lsp.Range{Start: lsp.Position{Line: 20, Character: 10}}},
		{URI: uri, Range: lsp.Range{Start: lsp.Position{Line: 30, Character: 15}}},
		{URI: uri, Range: lsp.Range{Start: lsp.Position{Line: 40, Character: 8}}},
	}

	queryManager.index.mu.Lock()
	queryManager.index.SymbolIndex["TestFunction"] = testSymbol
	queryManager.index.PositionIndex[PositionKey{URI: uri, Line: 10, Character: 5}] = "TestFunction"
	queryManager.index.ReferenceGraph["TestFunction"] = refs
	queryManager.index.mu.Unlock()

	params := &lsp.ReferenceParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: uri},
			Position:     lsp.Position{Line: 10, Character: 5},
		},
		Context: lsp.ReferenceContext{IncludeDeclaration: true},
	}

	// Performance test - should be under 5ms for references
	iterations := 500
	start := time.Now()

	for i := 0; i < iterations; i++ {
		results, err := queryManager.GetReferences(context.Background(), params)
		if err != nil {
			t.Fatalf("GetReferences failed: %v", err)
		}
		if len(results) != 4 { // 3 references + 1 declaration
			t.Errorf("Expected 4 results, got %d", len(results))
		}
	}

	duration := time.Since(start)
	avgDuration := duration / time.Duration(iterations)

	t.Logf("Average references lookup time: %v", avgDuration)

	// Verify performance requirement: < 5ms per query
	if avgDuration > 5*time.Millisecond {
		t.Errorf("References lookup too slow: %v > 5ms", avgDuration)
	}
}

func TestSCIPQueryManager_WorkspaceSymbolPerformance(t *testing.T) {
	fallback := NewMockLSPFallback()
	queryManager := NewSCIPQueryManager(fallback)

	// Setup large symbol set
	symbols := make([]*lsp.SymbolInformation, 0, 1000)
	for i := 0; i < 1000; i++ {
		symbol := createTestSymbol(
			fmt.Sprintf("Symbol%d", i),
			"file:///test.go",
			i, 0,
			lsp.Function,
		)
		symbols = append(symbols, symbol)
	}

	err := queryManager.UpdateIndex("file:///test.go", symbols, nil)
	if err != nil {
		t.Fatalf("Failed to update index: %v", err)
	}

	params := &lsp.WorkspaceSymbolParams{Query: "Symbol"}

	// Performance test - should be under 10ms for workspace search
	iterations := 100
	start := time.Now()

	for i := 0; i < iterations; i++ {
		results, err := queryManager.GetWorkspaceSymbols(context.Background(), params)
		if err != nil {
			t.Fatalf("GetWorkspaceSymbols failed: %v", err)
		}
		if len(results) == 0 {
			t.Error("Expected some results from workspace symbol search")
		}
	}

	duration := time.Since(start)
	avgDuration := duration / time.Duration(iterations)

	t.Logf("Average workspace symbol search time: %v", avgDuration)

	// Verify performance requirement: < 10ms per query
	if avgDuration > 10*time.Millisecond {
		t.Errorf("Workspace symbol search too slow: %v > 10ms", avgDuration)
	}
}

// Functional tests

func TestSCIPQueryManager_GetDefinition(t *testing.T) {
	fallback := NewMockLSPFallback()
	queryManager := NewSCIPQueryManager(fallback)

	// Test cache miss scenario
	params := &lsp.DefinitionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: "file:///nonexistent.go"},
			Position:     lsp.Position{Line: 5, Character: 10},
		},
	}

	// Setup fallback response
	expectedLocation := &lsp.Location{
		URI: "file:///nonexistent.go",
		Range: lsp.Range{
			Start: lsp.Position{Line: 1, Character: 0},
			End:   lsp.Position{Line: 1, Character: 10},
		},
	}
	fallback.SetResponse("textDocument/definition", []*lsp.Location{expectedLocation})

	locations, err := queryManager.GetDefinition(context.Background(), params)
	if err != nil {
		t.Fatalf("GetDefinition failed: %v", err)
	}

	if len(locations) != 1 {
		t.Errorf("Expected 1 location, got %d", len(locations))
	}

	if locations[0].URI != expectedLocation.URI {
		t.Errorf("Expected URI %s, got %s", expectedLocation.URI, locations[0].URI)
	}

	// Verify fallback was called
	if fallback.GetCallCount("textDocument/definition") != 1 {
		t.Errorf("Expected 1 fallback call, got %d", fallback.GetCallCount("textDocument/definition"))
	}
}

func TestSCIPQueryManager_GetDocumentSymbols(t *testing.T) {
	fallback := NewMockLSPFallback()
	queryManager := NewSCIPQueryManager(fallback)

	// Setup test document symbols
	uri := "file:///test.go"
	symbols := []*lsp.DocumentSymbol{
		createTestDocumentSymbol("main", 0, 0, lsp.Function),
		createTestDocumentSymbol("helper", 10, 0, lsp.Function),
	}

	err := queryManager.UpdateIndex(uri, nil, symbols)
	if err != nil {
		t.Fatalf("Failed to update index: %v", err)
	}

	params := &lsp.DocumentSymbolParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
	}

	result, err := queryManager.GetDocumentSymbols(context.Background(), params)
	if err != nil {
		t.Fatalf("GetDocumentSymbols failed: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 symbols, got %d", len(result))
	}

	if result[0].Name != "main" {
		t.Errorf("Expected first symbol to be 'main', got '%s'", result[0].Name)
	}

	// Verify no fallback was needed
	if fallback.GetCallCount("textDocument/documentSymbol") > 0 {
		t.Errorf("Expected no fallback calls, got %d", fallback.GetCallCount("textDocument/documentSymbol"))
	}
}

func TestSCIPQueryManager_UpdateIndex(t *testing.T) {
	fallback := NewMockLSPFallback()
	queryManager := NewSCIPQueryManager(fallback)

	uri := "file:///test.go"
	symbols := []*lsp.SymbolInformation{
		createTestSymbol("TestFunc1", uri, 5, 0, lsp.Function),
		createTestSymbol("TestFunc2", uri, 15, 0, lsp.Function),
	}

	documentSymbols := []*lsp.DocumentSymbol{
		createTestDocumentSymbol("TestFunc1", 5, 0, lsp.Function),
		createTestDocumentSymbol("TestFunc2", 15, 0, lsp.Function),
	}

	err := queryManager.UpdateIndex(uri, symbols, documentSymbols)
	if err != nil {
		t.Fatalf("UpdateIndex failed: %v", err)
	}

	// Verify symbols were indexed
	queryManager.index.mu.RLock()
	if len(queryManager.index.SymbolIndex) != 2 {
		t.Errorf("Expected 2 symbols in index, got %d", len(queryManager.index.SymbolIndex))
	}

	if len(queryManager.index.PositionIndex) != 2 {
		t.Errorf("Expected 2 position entries, got %d", len(queryManager.index.PositionIndex))
	}

	if len(queryManager.index.DocumentSymbols) != 1 {
		t.Errorf("Expected 1 document in symbol index, got %d", len(queryManager.index.DocumentSymbols))
	}
	queryManager.index.mu.RUnlock()
}

func TestSCIPQueryManager_InvalidateDocument(t *testing.T) {
	fallback := NewMockLSPFallback()
	queryManager := NewSCIPQueryManager(fallback)

	uri := "file:///test.go"
	symbols := []*lsp.SymbolInformation{
		createTestSymbol("TestFunc", uri, 5, 0, lsp.Function),
	}

	// Add symbols to index
	err := queryManager.UpdateIndex(uri, symbols, nil)
	if err != nil {
		t.Fatalf("UpdateIndex failed: %v", err)
	}

	// Verify symbols are present
	queryManager.index.mu.RLock()
	initialSymbolCount := len(queryManager.index.SymbolIndex)
	initialPositionCount := len(queryManager.index.PositionIndex)
	queryManager.index.mu.RUnlock()

	if initialSymbolCount == 0 {
		t.Fatal("Expected symbols to be present before invalidation")
	}

	// Invalidate document
	err = queryManager.InvalidateDocument(uri)
	if err != nil {
		t.Fatalf("InvalidateDocument failed: %v", err)
	}

	// Verify symbols were removed
	queryManager.index.mu.RLock()
	finalSymbolCount := len(queryManager.index.SymbolIndex)
	finalPositionCount := len(queryManager.index.PositionIndex)
	queryManager.index.mu.RUnlock()

	if finalSymbolCount >= initialSymbolCount {
		t.Errorf("Expected symbol count to decrease after invalidation: %d -> %d",
			initialSymbolCount, finalSymbolCount)
	}

	if finalPositionCount >= initialPositionCount {
		t.Errorf("Expected position count to decrease after invalidation: %d -> %d",
			initialPositionCount, finalPositionCount)
	}
}

func TestSCIPQueryManager_CircuitBreaker(t *testing.T) {
	fallback := NewMockLSPFallback()
	queryManager := NewSCIPQueryManager(fallback)

	// Force circuit breaker to open by recording failures
	for i := 0; i < 10; i++ {
		queryManager.circuitBreaker.recordFailure()
	}

	// Verify circuit breaker is open
	if queryManager.circuitBreaker.allowRequest() {
		t.Error("Expected circuit breaker to be open and reject requests")
	}

	params := &lsp.DefinitionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: "file:///test.go"},
			Position:     lsp.Position{Line: 1, Character: 1},
		},
	}

	// Setup fallback response
	fallback.SetResponse("textDocument/definition", []*lsp.Location{})

	// This should still work because circuit breaker allows fallback
	_, err := queryManager.GetDefinition(context.Background(), params)
	if err != nil {
		t.Fatalf("Expected request to succeed through fallback: %v", err)
	}

	// Verify fallback was called
	if fallback.GetCallCount("textDocument/definition") != 1 {
		t.Errorf("Expected 1 fallback call, got %d", fallback.GetCallCount("textDocument/definition"))
	}
}

func TestSCIPQueryManager_GetMetrics(t *testing.T) {
	fallback := NewMockLSPFallback()
	queryManager := NewSCIPQueryManager(fallback)

	// Initially metrics should be zero
	metrics := queryManager.GetMetrics()
	if metrics.TotalQueries != 0 {
		t.Errorf("Expected 0 total queries, got %d", metrics.TotalQueries)
	}

	// Perform some operations
	uri := "file:///test.go"
	symbols := []*lsp.SymbolInformation{
		createTestSymbol("TestFunc", uri, 5, 0, lsp.Function),
	}

	err := queryManager.UpdateIndex(uri, symbols, nil)
	if err != nil {
		t.Fatalf("UpdateIndex failed: %v", err)
	}

	params := &lsp.DefinitionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: uri},
			Position:     lsp.Position{Line: 5, Character: 0},
		},
	}

	// Perform successful query (cache hit)
	_, err = queryManager.GetDefinition(context.Background(), params)
	if err != nil {
		t.Fatalf("GetDefinition failed: %v", err)
	}

	// Check updated metrics
	metrics = queryManager.GetMetrics()
	if metrics.TotalQueries != 1 {
		t.Errorf("Expected 1 total query, got %d", metrics.TotalQueries)
	}

	if metrics.CacheHits != 1 {
		t.Errorf("Expected 1 cache hit, got %d", metrics.CacheHits)
	}

	if metrics.CacheMisses != 0 {
		t.Errorf("Expected 0 cache misses, got %d", metrics.CacheMisses)
	}
}

// Benchmark tests

func BenchmarkSCIPQueryManager_GetDefinition(b *testing.B) {
	fallback := NewMockLSPFallback()
	queryManager := NewSCIPQueryManager(fallback)

	// Setup test data
	uri := "file:///benchmark.go"
	symbol := createTestSymbol("BenchmarkFunc", uri, 10, 5, lsp.Function)

	err := queryManager.UpdateIndex(uri, []*lsp.SymbolInformation{symbol}, nil)
	if err != nil {
		b.Fatalf("Failed to update index: %v", err)
	}

	params := &lsp.DefinitionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: uri},
			Position:     lsp.Position{Line: 10, Character: 5},
		},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := queryManager.GetDefinition(context.Background(), params)
		if err != nil {
			b.Fatalf("GetDefinition failed: %v", err)
		}
	}
}

func BenchmarkSCIPQueryManager_WorkspaceSymbols(b *testing.B) {
	fallback := NewMockLSPFallback()
	queryManager := NewSCIPQueryManager(fallback)

	// Setup large symbol set
	symbols := make([]*lsp.SymbolInformation, 0, 10000)
	for i := 0; i < 10000; i++ {
		symbol := createTestSymbol(
			fmt.Sprintf("BenchSymbol%d", i),
			"file:///benchmark.go",
			i, 0,
			lsp.Function,
		)
		symbols = append(symbols, symbol)
	}

	err := queryManager.UpdateIndex("file:///benchmark.go", symbols, nil)
	if err != nil {
		b.Fatalf("Failed to update index: %v", err)
	}

	params := &lsp.WorkspaceSymbolParams{Query: "BenchSymbol"}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := queryManager.GetWorkspaceSymbols(context.Background(), params)
		if err != nil {
			b.Fatalf("GetWorkspaceSymbols failed: %v", err)
		}
	}
}
