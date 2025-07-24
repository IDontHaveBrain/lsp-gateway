package gateway

import (
	"encoding/json"
	"testing"
	"time"

	"lsp-gateway/mcp"
)

func createTestLogger() *mcp.StructuredLogger {
	config := &mcp.LoggerConfig{
		Level:     mcp.LogLevelDebug,
		Component: "test",
	}
	return mcp.NewStructuredLogger(config)
}

func TestAggregatorRegistry(t *testing.T) {
	logger := createTestLogger()
	registry := NewAggregatorRegistry(logger)

	// Test registry creation
	if registry == nil {
		t.Fatal("Registry should not be nil")
	}

	// Test default aggregators are registered
	testCases := []struct {
		method   string
		expected bool
	}{
		{LSP_METHOD_DEFINITION, true},
		{LSP_METHOD_REFERENCES, true},
		{LSP_METHOD_WORKSPACE_SYMBOL, true},
		{LSP_METHOD_HOVER, true},
		{"textDocument/diagnostic", true},
		{"textDocument/completion", true},
		{"unknown/method", false},
	}

	for _, tc := range testCases {
		_, exists := registry.GetAggregator(tc.method)
		if exists != tc.expected {
			t.Errorf("Method %s: expected %v, got %v", tc.method, tc.expected, exists)
		}
	}
}

func TestDefinitionAggregator(t *testing.T) {
	logger := createTestLogger()
	aggregator := &DefinitionAggregator{logger: logger}

	// Test single location response
	loc1 := Location{
		URI: "file:///test.go",
		Range: Range{
			Start: Position{Line: 10, Character: 5},
			End:   Position{Line: 10, Character: 15},
		},
	}

	responses := []interface{}{loc1}
	sources := []string{"go-lsp"}

	result, err := aggregator.Aggregate(responses, sources)
	if err != nil {
		t.Fatalf("Aggregation failed: %v", err)
	}

	resultLoc, ok := result.(Location)
	if !ok {
		t.Fatalf("Expected Location, got %T", result)
	}

	if resultLoc.URI != loc1.URI {
		t.Errorf("Expected URI %s, got %s", loc1.URI, resultLoc.URI)
	}

	// Test multiple locations response
	loc2 := Location{
		URI: "file:///test.ts",
		Range: Range{
			Start: Position{Line: 20, Character: 10},
			End:   Position{Line: 20, Character: 20},
		},
	}

	responses = []interface{}{[]Location{loc1}, []Location{loc2}}
	sources = []string{"go-lsp", "ts-lsp"}

	result, err = aggregator.Aggregate(responses, sources)
	if err != nil {
		t.Fatalf("Aggregation failed: %v", err)
	}

	resultLocs, ok := result.([]Location)
	if !ok {
		t.Fatalf("Expected []Location, got %T", result)
	}

	if len(resultLocs) != 2 {
		t.Errorf("Expected 2 locations, got %d", len(resultLocs))
	}

	// Test deduplication
	responses = []interface{}{loc1, loc1} // Same location twice
	sources = []string{"go-lsp1", "go-lsp2"}

	result, err = aggregator.Aggregate(responses, sources)
	if err != nil {
		t.Fatalf("Aggregation failed: %v", err)
	}

	resultLoc, ok = result.(Location)
	if !ok {
		t.Fatalf("Expected single Location after deduplication, got %T", result)
	}
}

func TestReferencesAggregator(t *testing.T) {
	logger := createTestLogger()
	aggregator := &ReferencesAggregator{logger: logger}

	ref1 := Location{
		URI: "file:///test1.go",
		Range: Range{
			Start: Position{Line: 5, Character: 0},
			End:   Position{Line: 5, Character: 10},
		},
	}

	ref2 := Location{
		URI: "file:///test2.go",
		Range: Range{
			Start: Position{Line: 15, Character: 5},
			End:   Position{Line: 15, Character: 15},
		},
	}

	responses := []interface{}{[]Location{ref1}, []Location{ref2}}
	sources := []string{"go-lsp", "go-lsp2"}

	result, err := aggregator.Aggregate(responses, sources)
	if err != nil {
		t.Fatalf("Aggregation failed: %v", err)
	}

	resultRefs, ok := result.([]Location)
	if !ok {
		t.Fatalf("Expected []Location, got %T", result)
	}

	if len(resultRefs) != 2 {
		t.Errorf("Expected 2 references, got %d", len(resultRefs))
	}

	// Verify sorting: test1.go should come before test2.go
	if resultRefs[0].URI != "file:///test1.go" {
		t.Errorf("Expected first reference to be test1.go, got %s", resultRefs[0].URI)
	}

	// Test deduplication
	responses = []interface{}{[]Location{ref1, ref2}, []Location{ref1}} // ref1 appears twice
	sources = []string{"server1", "server2"}

	result, err = aggregator.Aggregate(responses, sources)
	if err != nil {
		t.Fatalf("Aggregation failed: %v", err)
	}

	resultRefs, ok = result.([]Location)
	if !ok {
		t.Fatalf("Expected []Location, got %T", result)
	}

	if len(resultRefs) != 2 {
		t.Errorf("Expected 2 unique references after deduplication, got %d", len(resultRefs))
	}
}

func TestSymbolAggregator(t *testing.T) {
	logger := createTestLogger()
	aggregator := &SymbolAggregator{logger: logger}

	symbol1 := SymbolInformation{
		Name: "TestFunction",
		Kind: SymbolKindFunction,
		Location: Location{
			URI: "file:///test.go",
			Range: Range{
				Start: Position{Line: 10, Character: 0},
				End:   Position{Line: 15, Character: 0},
			},
		},
		ContainerName: "TestClass",
	}

	symbol2 := SymbolInformation{
		Name: "TestVariable",
		Kind: SymbolKindVariable,
		Location: Location{
			URI: "file:///test.go",
			Range: Range{
				Start: Position{Line: 5, Character: 0},
				End:   Position{Line: 5, Character: 10},
			},
		},
		ContainerName: "TestClass",
	}

	responses := []interface{}{[]SymbolInformation{symbol1}, []SymbolInformation{symbol2}}
	sources := []string{"go-lsp", "go-lsp2"}

	result, err := aggregator.Aggregate(responses, sources)
	if err != nil {
		t.Fatalf("Aggregation failed: %v", err)
	}

	resultSymbols, ok := result.([]SymbolInformation)
	if !ok {
		t.Fatalf("Expected []SymbolInformation, got %T", result)
	}

	if len(resultSymbols) != 2 {
		t.Errorf("Expected 2 symbols, got %d", len(resultSymbols))
	}

	// Verify sorting: symbols should be sorted by name
	if resultSymbols[0].Name != "TestFunction" {
		t.Errorf("Expected first symbol to be TestFunction, got %s", resultSymbols[0].Name)
	}
}

func TestHoverAggregator(t *testing.T) {
	logger := createTestLogger()
	aggregator := &HoverAggregator{logger: logger}

	hover1 := Hover{
		Contents: MarkupContent{
			Kind:  "markdown",
			Value: "Function documentation from server 1",
		},
		Range: &Range{
			Start: Position{Line: 10, Character: 5},
			End:   Position{Line: 10, Character: 15},
		},
	}

	hover2 := Hover{
		Contents: MarkupContent{
			Kind:  "markdown",
			Value: "Additional information from server 2",
		},
	}

	responses := []interface{}{hover1, hover2}
	sources := []string{"go-lsp", "enhanced-lsp"}

	result, err := aggregator.Aggregate(responses, sources)
	if err != nil {
		t.Fatalf("Aggregation failed: %v", err)
	}

	resultHover, ok := result.(Hover)
	if !ok {
		t.Fatalf("Expected Hover, got %T", result)
	}

	// Check that content was merged
	if resultHover.Contents == nil {
		t.Fatal("Result hover should have contents")
	}

	markupContent, ok := resultHover.Contents.(MarkupContent)
	if !ok {
		t.Fatalf("Expected MarkupContent, got %T", resultHover.Contents)
	}

	expectedContent := "Function documentation from server 1\n\n---\n\nAdditional information from server 2"
	if markupContent.Value != expectedContent {
		t.Errorf("Expected merged content, got: %s", markupContent.Value)
	}

	// Check that range was preserved
	if resultHover.Range == nil {
		t.Error("Range should be preserved from first hover")
	}
}

func TestDiagnosticAggregator(t *testing.T) {
	logger := createTestLogger()
	aggregator := &DiagnosticAggregator{logger: logger}

	diag1 := Diagnostic{
		Range: Range{
			Start: Position{Line: 5, Character: 0},
			End:   Position{Line: 5, Character: 10},
		},
		Severity: 1, // Error
		Message:  "Syntax error",
		Source:   "eslint",
	}

	diag2 := Diagnostic{
		Range: Range{
			Start: Position{Line: 10, Character: 5},
			End:   Position{Line: 10, Character: 15},
		},
		Severity: 2, // Warning
		Message:  "Unused variable",
		Source:   "tslint",
	}

	responses := []interface{}{[]Diagnostic{diag1}, []Diagnostic{diag2}}
	sources := []string{"eslint-server", "tslint-server"}

	result, err := aggregator.Aggregate(responses, sources)
	if err != nil {
		t.Fatalf("Aggregation failed: %v", err)
	}

	resultDiags, ok := result.([]Diagnostic)
	if !ok {
		t.Fatalf("Expected []Diagnostic, got %T", result)
	}

	if len(resultDiags) != 2 {
		t.Errorf("Expected 2 diagnostics, got %d", len(resultDiags))
	}

	// Verify sorting: errors (severity 1) should come before warnings (severity 2)
	if resultDiags[0].Severity != 1 {
		t.Errorf("Expected first diagnostic to be error (severity 1), got %d", resultDiags[0].Severity)
	}

	if resultDiags[1].Severity != 2 {
		t.Errorf("Expected second diagnostic to be warning (severity 2), got %d", resultDiags[1].Severity)
	}
}

func TestCompletionAggregator(t *testing.T) {
	logger := createTestLogger()
	aggregator := &CompletionAggregator{logger: logger}

	item1 := CompletionItem{
		Label:    "function1",
		Kind:     12, // Function
		Detail:   "func() string",
		SortText: "a",
	}

	item2 := CompletionItem{
		Label:     "variable1",
		Kind:      13, // Variable
		Detail:    "string",
		SortText:  "b",
		Preselect: true,
	}

	completion1 := CompletionList{
		IsIncomplete: false,
		Items:        []CompletionItem{item1},
	}

	completion2 := CompletionList{
		IsIncomplete: true,
		Items:        []CompletionItem{item2},
	}

	responses := []interface{}{completion1, completion2}
	sources := []string{"go-lsp", "enhanced-lsp"}

	result, err := aggregator.Aggregate(responses, sources)
	if err != nil {
		t.Fatalf("Aggregation failed: %v", err)
	}

	resultCompletion, ok := result.(CompletionList)
	if !ok {
		t.Fatalf("Expected CompletionList, got %T", result)
	}

	if len(resultCompletion.Items) != 2 {
		t.Errorf("Expected 2 completion items, got %d", len(resultCompletion.Items))
	}

	// Verify isIncomplete is set if any source is incomplete
	if !resultCompletion.IsIncomplete {
		t.Error("Expected isIncomplete to be true when any source is incomplete")
	}

	// Verify sorting: preselected items should come first
	if !resultCompletion.Items[0].Preselect {
		t.Error("Expected first item to be preselected")
	}
}

func TestAggregationResult(t *testing.T) {
	logger := createTestLogger()
	registry := NewAggregatorRegistry(logger)

	// Test basic aggregation result
	responses := []interface{}{
		Location{URI: "file:///test.go", Range: Range{Start: Position{Line: 1, Character: 1}, End: Position{Line: 1, Character: 10}}},
		nil, // Simulate failed response
	}
	sources := []string{"server1", "server2"}

	result, err := registry.AggregateResponses(LSP_METHOD_DEFINITION, responses, sources)
	if err != nil {
		t.Fatalf("Aggregation failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	if result.TotalSources != 2 {
		t.Errorf("Expected 2 total sources, got %d", result.TotalSources)
	}

	if result.SuccessfulSources != 1 {
		t.Errorf("Expected 1 successful source, got %d", result.SuccessfulSources)
	}

	if result.MergeStrategy != "definition" {
		t.Errorf("Expected merge strategy 'definition', got %s", result.MergeStrategy)
	}

	if result.ProcessingTime == 0 {
		t.Error("Processing time should be greater than 0")
	}

	// Verify quality metrics
	if result.Quality.Score <= 0 || result.Quality.Score > 1 {
		t.Errorf("Quality score should be between 0 and 1, got %f", result.Quality.Score)
	}

	if result.Quality.Completeness != 0.5 {
		t.Errorf("Expected completeness 0.5, got %f", result.Quality.Completeness)
	}
}

func TestMapConversions(t *testing.T) {
	logger := createTestLogger()

	// Test Location map conversion
	locationMap := map[string]interface{}{
		"uri": "file:///test.go",
		"range": map[string]interface{}{
			"start": map[string]interface{}{"line": float64(10), "character": float64(5)},
			"end":   map[string]interface{}{"line": float64(10), "character": float64(15)},
		},
	}

	defAgg := &DefinitionAggregator{logger: logger}
	loc := defAgg.convertMapToLocation(locationMap)
	if loc == nil {
		t.Fatal("Location conversion should not return nil")
	}

	if loc.URI != "file:///test.go" {
		t.Errorf("Expected URI 'file:///test.go', got %s", loc.URI)
	}

	if loc.Range.Start.Line != 10 {
		t.Errorf("Expected start line 10, got %d", loc.Range.Start.Line)
	}

	// Test SymbolInformation map conversion
	symbolMap := map[string]interface{}{
		"name": "TestSymbol",
		"kind": float64(12), // Function
		"location": map[string]interface{}{
			"uri": "file:///test.go",
			"range": map[string]interface{}{
				"start": map[string]interface{}{"line": float64(5), "character": float64(0)},
				"end":   map[string]interface{}{"line": float64(10), "character": float64(0)},
			},
		},
		"containerName": "TestContainer",
	}

	symAgg := &SymbolAggregator{logger: logger}
	sym := symAgg.convertMapToSymbolInformation(symbolMap)
	if sym == nil {
		t.Fatal("Symbol conversion should not return nil")
	}

	if sym.Name != "TestSymbol" {
		t.Errorf("Expected name 'TestSymbol', got %s", sym.Name)
	}

	if sym.Kind != SymbolKindFunction {
		t.Errorf("Expected kind Function (%d), got %d", SymbolKindFunction, sym.Kind)
	}
}

func TestErrorHandling(t *testing.T) {
	logger := createTestLogger()
	registry := NewAggregatorRegistry(logger)

	// Test aggregation with all nil responses
	responses := []interface{}{nil, nil}
	sources := []string{"server1", "server2"}

	result, err := registry.AggregateResponses(LSP_METHOD_DEFINITION, responses, sources)
	if err == nil {
		t.Error("Expected error when all responses are nil")
	}

	// Test aggregation with unsupported method (should use basic aggregation)
	responses = []interface{}{"some response"}
	sources = []string{"server1"}

	result, err = registry.AggregateResponses("unsupported/method", responses, sources)
	if err != nil {
		t.Fatalf("Basic aggregation should not fail: %v", err)
	}

	if result.MergeStrategy != "basic_first_success" {
		t.Errorf("Expected basic merge strategy, got %s", result.MergeStrategy)
	}
}

func TestConcurrentAccess(t *testing.T) {
	logger := createTestLogger()
	registry := NewAggregatorRegistry(logger)

	// Test concurrent access to registry
	done := make(chan bool)
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()

			// Test getting aggregator
			_, exists := registry.GetAggregator(LSP_METHOD_DEFINITION)
			if !exists {
				t.Error("Definition aggregator should exist")
				return
			}

			// Test aggregation
			responses := []interface{}{
				Location{URI: "file:///test.go", Range: Range{Start: Position{Line: 1, Character: 1}, End: Position{Line: 1, Character: 10}}},
			}
			sources := []string{"server1"}

			_, err := registry.AggregateResponses(LSP_METHOD_DEFINITION, responses, sources)
			if err != nil {
				t.Errorf("Concurrent aggregation failed: %v", err)
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func TestHoverContentExtraction(t *testing.T) {
	logger := createTestLogger()
	aggregator := &HoverAggregator{logger: logger}

	testCases := []struct {
		name     string
		content  interface{}
		expected string
	}{
		{
			name:     "String content",
			content:  "Simple string content",
			expected: "Simple string content",
		},
		{
			name: "MarkupContent",
			content: MarkupContent{
				Kind:  "markdown",
				Value: "Markdown content",
			},
			expected: "Markdown content",
		},
		{
			name: "Map content",
			content: map[string]interface{}{
				"value": "Map value content",
			},
			expected: "Map value content",
		},
		{
			name: "Array content",
			content: []interface{}{
				"First part",
				MarkupContent{Kind: "markdown", Value: "Second part"},
			},
			expected: "First part\n\nSecond part",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := aggregator.extractContentString(tc.content)
			if result != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestJSONSerialization(t *testing.T) {
	// Test that our structures can be properly serialized to JSON
	location := Location{
		URI: "file:///test.go",
		Range: Range{
			Start: Position{Line: 10, Character: 5},
			End:   Position{Line: 10, Character: 15},
		},
	}

	data, err := json.Marshal(location)
	if err != nil {
		t.Fatalf("JSON marshaling failed: %v", err)
	}

	var unmarshaled Location
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("JSON unmarshaling failed: %v", err)
	}

	if unmarshaled.URI != location.URI {
		t.Errorf("URI mismatch after JSON round-trip: %s != %s", unmarshaled.URI, location.URI)
	}

	if unmarshaled.Range.Start.Line != location.Range.Start.Line {
		t.Errorf("Start line mismatch after JSON round-trip: %d != %d", unmarshaled.Range.Start.Line, location.Range.Start.Line)
	}
}

func BenchmarkDefinitionAggregation(b *testing.B) {
	logger := createTestLogger()
	aggregator := &DefinitionAggregator{logger: logger}

	// Create test data
	locations := make([]Location, 100)
	for i := 0; i < 100; i++ {
		locations[i] = Location{
			URI: fmt.Sprintf("file:///test%d.go", i),
			Range: Range{
				Start: Position{Line: i, Character: 0},
				End:   Position{Line: i, Character: 10},
			},
		}
	}

	responses := []interface{}{locations}
	sources := []string{"go-lsp"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := aggregator.Aggregate(responses, sources)
		if err != nil {
			b.Fatalf("Aggregation failed: %v", err)
		}
	}
}

func BenchmarkRegistryAggregation(b *testing.B) {
	logger := createTestLogger()
	registry := NewAggregatorRegistry(logger)

	responses := []interface{}{
		Location{URI: "file:///test.go", Range: Range{Start: Position{Line: 1, Character: 1}, End: Position{Line: 1, Character: 10}}},
	}
	sources := []string{"server1"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := registry.AggregateResponses(LSP_METHOD_DEFINITION, responses, sources)
		if err != nil {
			b.Fatalf("Aggregation failed: %v", err)
		}
	}
}