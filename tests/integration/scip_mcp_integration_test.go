package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"lsp-gateway/internal/indexing"
	"lsp-gateway/mcp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSCIPMCPIntegration provides comprehensive integration tests for SCIP-enhanced MCP tools
func TestSCIPMCPIntegration(t *testing.T) {
	suite := NewSCIPMCPTestSuite(t)
	defer suite.Cleanup()

	// Test suite for different scenarios
	t.Run("BasicIntegration", suite.TestBasicIntegration)
	t.Run("PerformanceTargets", suite.TestPerformanceTargets)
	t.Run("FallbackMechanisms", suite.TestFallbackMechanisms)
	t.Run("BackwardCompatibility", suite.TestBackwardCompatibility)
	t.Run("ConfigurationManagement", suite.TestConfigurationManagement)
	t.Run("SCIPEnhancedTools", suite.TestSCIPEnhancedTools)
	t.Run("HealthMonitoring", suite.TestHealthMonitoring)
	t.Run("ConcurrentRequests", suite.TestConcurrentRequests)
}

// SCIPMCPTestSuite provides test infrastructure for SCIP-MCP integration tests
type SCIPMCPTestSuite struct {
	t                   *testing.T
	integration         *mcp.SCIPMCPIntegration
	standardHandler     *mcp.ToolHandler
	scipStore           *MockSCIPStore
	symbolResolver      *MockSymbolResolver
	workspaceContext    *mcp.WorkspaceContext
	testProjectPath     string
	cleanup             []func()
}

// NewSCIPMCPTestSuite creates a new test suite with mock components
func NewSCIPMCPTestSuite(t *testing.T) *SCIPMCPTestSuite {
	suite := &SCIPMCPTestSuite{
		t:       t,
		cleanup: make([]func(), 0),
	}

	// Setup test project
	suite.setupTestProject()

	// Create mock components
	suite.setupMockComponents()

	// Create integration
	suite.setupIntegration()

	return suite
}

// setupTestProject creates a test project structure
func (suite *SCIPMCPTestSuite) setupTestProject() {
	// Create temporary test project
	suite.testProjectPath = "/tmp/scip_mcp_test_project"
	
	// This would create actual test files in a real implementation
	// For this test, we'll use mock data
	
	suite.cleanup = append(suite.cleanup, func() {
		// Cleanup test project
	})
}

// setupMockComponents creates mock SCIP components for testing
func (suite *SCIPMCPTestSuite) setupMockComponents() {
	// Create mock SCIP store
	suite.scipStore = NewMockSCIPStore()
	
	// Create mock symbol resolver
	suite.symbolResolver = NewMockSymbolResolver()
	
	// Create workspace context
	suite.workspaceContext = &mcp.WorkspaceContext{}
	
	// Setup mock data
	suite.setupMockData()
}

// setupMockData populates mock components with test data
func (suite *SCIPMCPTestSuite) setupMockData() {
	// Add mock symbols for testing
	suite.scipStore.AddMockSymbol(&MockSymbol{
		Name:        "TestClass",
		Kind:        5, // Class
		URI:         "file:///test/example.go",
		Language:    "go",
		Range:       &MockRange{Start: MockPosition{Line: 10, Character: 0}, End: MockPosition{Line: 50, Character: 1}},
		Confidence:  0.95,
		Documentation: "A test class for demonstration",
	})
	
	suite.scipStore.AddMockSymbol(&MockSymbol{
		Name:        "testFunction",
		Kind:        12, // Function
		URI:         "file:///test/example.go",
		Language:    "go",
		Range:       &MockRange{Start: MockPosition{Line: 15, Character: 4}, End: MockPosition{Line: 25, Character: 1}},
		Confidence:  0.9,
		Documentation: "A test function",
	})
	
	// Add cross-language references
	suite.scipStore.AddMockCrossLanguageRef(&MockCrossLanguageRef{
		SourceSymbol:   "TestClass",
		SourceLanguage: "go",
		TargetSymbol:   "TestClassImpl",
		TargetLanguage: "python",
		Relationship:   "implementation",
		Confidence:     0.85,
	})
}

// setupIntegration creates the SCIP-MCP integration
func (suite *SCIPMCPTestSuite) setupIntegration() {
	// Create standard handler with mock client
	mockClient := NewMockLSPGatewayClient()
	suite.standardHandler = mcp.NewToolHandler(mockClient)
	
	// Create integration configuration
	config := &mcp.SCIPMCPConfig{
		EnableSCIPEnhancements:      true,
		EnableFallbackMode:          true,
		EnablePerformanceMonitoring: true,
		EnableHealthChecking:        true,
		
		PerformanceConfig: &mcp.SCIPPerformanceConfig{
			TargetSymbolSearchTime:    50 * time.Millisecond,
			TargetCrossLanguageTime:   100 * time.Millisecond,
			TargetContextAnalysisTime: 200 * time.Millisecond,
			TargetSemanticAnalysisTime: 500 * time.Millisecond,
			MaxConcurrentQueries:      10,
			EnableAutoTuning:          false, // Disable for testing
		},
		
		ToolConfigs: map[string]*mcp.ToolSpecificConfig{
			"scip_intelligent_symbol_search": {
				Enabled:             true,
				Timeout:             50 * time.Millisecond,
				CacheEnabled:        true,
				CacheTTL:           1 * time.Minute,
				MaxResults:         20,
				ConfidenceThreshold: 0.7,
				FallbackEnabled:    true,
			},
		},
	}
	
	// Create integration
	integration, err := mcp.NewSCIPMCPIntegration(
		suite.standardHandler,
		suite.scipStore,
		suite.symbolResolver,
		suite.workspaceContext,
		config,
	)
	require.NoError(suite.t, err)
	suite.integration = integration
	
	suite.cleanup = append(suite.cleanup, func() {
		integration.Shutdown()
	})
}

// Test Methods

// TestBasicIntegration tests basic integration functionality
func (suite *SCIPMCPTestSuite) TestBasicIntegration(t *testing.T) {
	// Test that integration is properly initialized
	assert.NotNil(t, suite.integration)
	
	// Test that both standard and enhanced tools are available
	tools := suite.integration.ListTools()
	assert.NotEmpty(t, tools)
	
	// Verify SCIP-enhanced tools are present
	scipToolCount := 0
	for _, tool := range tools {
		if tool.Name == "scip_intelligent_symbol_search" ||
		   tool.Name == "scip_cross_language_references" ||
		   tool.Name == "scip_semantic_code_analysis" ||
		   tool.Name == "scip_context_aware_assistance" ||
		   tool.Name == "scip_workspace_intelligence" ||
		   tool.Name == "scip_refactoring_suggestions" {
			scipToolCount++
		}
	}
	assert.Equal(t, 6, scipToolCount, "All SCIP-enhanced tools should be available")
	
	// Test basic tool call
	ctx := context.Background()
	result, err := suite.integration.CallTool(ctx, mcp.ToolCall{
		Name:      "scip_intelligent_symbol_search",
		Arguments: map[string]interface{}{"query": "TestClass"},
	})
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
}

// TestPerformanceTargets verifies that performance targets are met
func (suite *SCIPMCPTestSuite) TestPerformanceTargets(t *testing.T) {
	ctx := context.Background()
	
	testCases := []struct {
		toolName        string
		args            map[string]interface{}
		targetTime      time.Duration
		description     string
	}{
		{
			toolName: "scip_intelligent_symbol_search",
			args:     map[string]interface{}{"query": "TestClass"},
			targetTime: 50 * time.Millisecond,
			description: "Symbol search should complete within 50ms",
		},
		{
			toolName: "scip_cross_language_references",
			args: map[string]interface{}{
				"uri":       "file:///test/example.go",
				"line":      15,
				"character": 4,
			},
			targetTime: 100 * time.Millisecond,
			description: "Cross-language references should complete within 100ms",
		},
		{
			toolName: "scip_context_aware_assistance",
			args: map[string]interface{}{
				"uri":         "file:///test/example.go",
				"line":        15,
				"character":   4,
				"contextType": "completion",
			},
			targetTime: 200 * time.Millisecond,
			description: "Context-aware assistance should complete within 200ms",
		},
		{
			toolName: "scip_semantic_code_analysis",
			args: map[string]interface{}{
				"uri":          "file:///test/example.go",
				"analysisType": "structure",
			},
			targetTime: 500 * time.Millisecond,
			description: "Semantic analysis should complete within 500ms",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			startTime := time.Now()
			
			result, err := suite.integration.CallTool(ctx, mcp.ToolCall{
				Name:      tc.toolName,
				Arguments: tc.args,
			})
			
			duration := time.Since(startTime)
			
			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.False(t, result.IsError)
			assert.True(t, duration <= tc.targetTime, 
				"Tool %s took %v, expected <= %v", tc.toolName, duration, tc.targetTime)
			
			// Verify response includes performance metadata
			if result.Meta != nil && result.Meta.RequestInfo != nil {
				assert.Contains(t, result.Meta.RequestInfo, "scip_used")
				assert.True(t, result.Meta.RequestInfo["scip_used"].(bool))
			}
		})
	}
}

// TestFallbackMechanisms tests graceful degradation and fallback
func (suite *SCIPMCPTestSuite) TestFallbackMechanisms(t *testing.T) {
	ctx := context.Background()
	
	// First, test normal operation
	result, err := suite.integration.CallTool(ctx, mcp.ToolCall{
		Name:      "scip_intelligent_symbol_search",
		Arguments: map[string]interface{}{"query": "TestClass"},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	
	// Simulate SCIP failure
	suite.scipStore.SetFailureMode(true)
	
	// Test that fallback works
	result, err = suite.integration.CallTool(ctx, mcp.ToolCall{
		Name:      "scip_intelligent_symbol_search",
		Arguments: map[string]interface{}{"query": "TestClass"},
	})
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	
	// Should have fallback metadata
	if result.Meta != nil && result.Meta.RequestInfo != nil {
		fallbackUsed, exists := result.Meta.RequestInfo["fallback_used"]
		assert.True(t, exists)
		assert.True(t, fallbackUsed.(bool))
	}
	
	// Restore SCIP functionality
	suite.scipStore.SetFailureMode(false)
}

// TestBackwardCompatibility ensures existing tools still work
func (suite *SCIPMCPTestSuite) TestBackwardCompatibility(t *testing.T) {
	ctx := context.Background()
	
	// Test standard LSP tools
	standardTools := []string{
		"goto_definition",
		"find_references",
		"get_hover_info",
		"get_document_symbols",
		"search_workspace_symbols",
	}
	
	for _, toolName := range standardTools {
		t.Run(fmt.Sprintf("Standard_%s", toolName), func(t *testing.T) {
			args := map[string]interface{}{
				"uri":       "file:///test/example.go",
				"line":      15,
				"character": 4,
			}
			
			if toolName == "search_workspace_symbols" {
				args = map[string]interface{}{"query": "TestClass"}
			} else if toolName == "get_document_symbols" {
				args = map[string]interface{}{"uri": "file:///test/example.go"}
			}
			
			result, err := suite.integration.CallTool(ctx, mcp.ToolCall{
				Name:      toolName,
				Arguments: args,
			})
			
			assert.NoError(t, err)
			assert.NotNil(t, result)
		})
	}
}

// TestConfigurationManagement tests configuration updates and validation
func (suite *SCIPMCPTestSuite) TestConfigurationManagement(t *testing.T) {
	// Test initial configuration
	originalMetrics := suite.integration.GetPerformanceMetrics()
	assert.NotNil(t, originalMetrics)
	
	// Test configuration update
	newConfig := &mcp.SCIPMCPConfig{
		EnableSCIPEnhancements:      false, // Disable SCIP
		EnableFallbackMode:          true,
		EnablePerformanceMonitoring: true,
		EnableHealthChecking:        true,
	}
	
	err := suite.integration.UpdateConfiguration(newConfig)
	assert.NoError(t, err)
	
	// Test that SCIP is disabled
	ctx := context.Background()
	result, err := suite.integration.CallTool(ctx, mcp.ToolCall{
		Name:      "scip_intelligent_symbol_search",
		Arguments: map[string]interface{}{"query": "TestClass"},
	})
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	
	// Should use fallback
	if result.Meta != nil && result.Meta.RequestInfo != nil {
		handlerType, exists := result.Meta.RequestInfo["handler_type"]
		assert.True(t, exists)
		assert.Equal(t, "standard", handlerType)
	}
}

// TestSCIPEnhancedTools tests each SCIP-enhanced tool individually
func (suite *SCIPMCPTestSuite) TestSCIPEnhancedTools(t *testing.T) {
	ctx := context.Background()
	
	// Test intelligent symbol search
	t.Run("IntelligentSymbolSearch", func(t *testing.T) {
		result, err := suite.integration.CallTool(ctx, mcp.ToolCall{
			Name: "scip_intelligent_symbol_search",
			Arguments: map[string]interface{}{
				"query":                     "TestClass",
				"scope":                     "workspace",
				"maxResults":                10,
				"includeSemanticSimilarity": true,
			},
		})
		
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.IsError)
		
		// Verify result structure
		assert.NotEmpty(t, result.Content)
		if len(result.Content) > 0 && result.Content[0].Data != nil {
			data := result.Content[0].Data.(map[string]interface{})
			assert.Contains(t, data, "query")
			assert.Contains(t, data, "results")
			assert.Contains(t, data, "confidence")
		}
	})
	
	// Test cross-language references
	t.Run("CrossLanguageReferences", func(t *testing.T) {
		result, err := suite.integration.CallTool(ctx, mcp.ToolCall{
			Name: "scip_cross_language_references",
			Arguments: map[string]interface{}{
				"uri":                   "file:///test/example.go",
				"line":                  10,
				"character":             0,
				"includeImplementations": true,
				"includeInheritance":     true,
			},
		})
		
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.IsError)
	})
	
	// Test workspace intelligence
	t.Run("WorkspaceIntelligence", func(t *testing.T) {
		result, err := suite.integration.CallTool(ctx, mcp.ToolCall{
			Name: "scip_workspace_intelligence",
			Arguments: map[string]interface{}{
				"insightType":           "overview",
				"includeMetrics":        true,
				"includeRecommendations": true,
			},
		})
		
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.IsError)
	})
	
	// Test refactoring suggestions
	t.Run("RefactoringSuggestions", func(t *testing.T) {
		result, err := suite.integration.CallTool(ctx, mcp.ToolCall{
			Name: "scip_refactoring_suggestions",
			Arguments: map[string]interface{}{
				"uri":                   "file:///test/example.go",
				"refactoringTypes":      []string{"extract_method", "optimize"},
				"includeImpactAnalysis": true,
			},
		})
		
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.IsError)
	})
}

// TestHealthMonitoring tests health monitoring functionality
func (suite *SCIPMCPTestSuite) TestHealthMonitoring(t *testing.T) {
	// Test health status
	health := suite.integration.GetHealthStatus()
	assert.NotNil(t, health)
	assert.NotEqual(t, mcp.HealthStateUnknown, health.Overall)
	
	// Test performance metrics
	metrics := suite.integration.GetPerformanceMetrics()
	assert.NotNil(t, metrics)
	assert.Contains(t, metrics, "scip_enabled")
	assert.Contains(t, metrics, "total_requests")
}

// TestConcurrentRequests tests concurrent request handling
func (suite *SCIPMCPTestSuite) TestConcurrentRequests(t *testing.T) {
	ctx := context.Background()
	numRequests := 10
	results := make(chan error, numRequests)
	
	// Launch concurrent requests
	for i := 0; i < numRequests; i++ {
		go func(index int) {
			_, err := suite.integration.CallTool(ctx, mcp.ToolCall{
				Name: "scip_intelligent_symbol_search",
				Arguments: map[string]interface{}{
					"query": fmt.Sprintf("TestClass%d", index),
				},
			})
			results <- err
		}(i)
	}
	
	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		err := <-results
		assert.NoError(t, err)
	}
	
	// Verify metrics show multiple requests
	metrics := suite.integration.GetPerformanceMetrics()
	totalRequests, ok := metrics["total_requests"].(int64)
	assert.True(t, ok)
	assert.True(t, totalRequests >= int64(numRequests))
}

// Cleanup cleans up test resources
func (suite *SCIPMCPTestSuite) Cleanup() {
	for i := len(suite.cleanup) - 1; i >= 0; i-- {
		suite.cleanup[i]()
	}
}

// Mock Components for Testing

// MockSCIPStore implements a mock SCIP store for testing
type MockSCIPStore struct {
	symbols              []*MockSymbol
	crossLanguageRefs    []*MockCrossLanguageRef
	failureMode          bool
	queryLatency         time.Duration
	mutex                sync.RWMutex
}

type MockSymbol struct {
	Name          string
	Kind          int
	URI           string
	Language      string
	Range         *MockRange
	Confidence    float64
	Documentation string
}

type MockRange struct {
	Start MockPosition
	End   MockPosition
}

type MockPosition struct {
	Line      int
	Character int
}

type MockCrossLanguageRef struct {
	SourceSymbol   string
	SourceLanguage string
	TargetSymbol   string
	TargetLanguage string
	Relationship   string
	Confidence     float64
}

func NewMockSCIPStore() *MockSCIPStore {
	return &MockSCIPStore{
		symbols:           make([]*MockSymbol, 0),
		crossLanguageRefs: make([]*MockCrossLanguageRef, 0),
		queryLatency:      10 * time.Millisecond,
	}
}

func (m *MockSCIPStore) LoadIndex(path string) error {
	return nil
}

func (m *MockSCIPStore) Query(method string, params interface{}) indexing.SCIPQueryResult {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	time.Sleep(m.queryLatency)
	
	if m.failureMode {
		return indexing.SCIPQueryResult{
			Found:      false,
			Method:     method,
			Error:      "mock failure",
			CacheHit:   false,
			QueryTime:  m.queryLatency,
			Confidence: 0.0,
		}
	}
	
	// Simulate different query responses based on method
	var response []byte
	var found bool
	
	switch method {
	case "workspace/symbol":
		// Return mock symbols
		if paramMap, ok := params.(map[string]interface{}); ok {
			if query, ok := paramMap["query"].(string); ok {
				var results []map[string]interface{}
				for _, symbol := range m.symbols {
					if containsIgnoreCase(symbol.Name, query) {
						results = append(results, map[string]interface{}{
							"name": symbol.Name,
							"kind": symbol.Kind,
							"location": map[string]interface{}{
								"uri": symbol.URI,
								"range": map[string]interface{}{
									"start": map[string]interface{}{
										"line":      symbol.Range.Start.Line,
										"character": symbol.Range.Start.Character,
									},
									"end": map[string]interface{}{
										"line":      symbol.Range.End.Line,
										"character": symbol.Range.End.Character,
									},
								},
							},
							"containerName": "",
						})
					}
				}
				response, _ = json.Marshal(results)
				found = len(results) > 0
			}
		}
	default:
		// Return empty successful response for other methods
		response = []byte("[]")
		found = true
	}
	
	return indexing.SCIPQueryResult{
		Found:      found,
		Method:     method,
		Response:   response,
		CacheHit:   false,
		QueryTime:  m.queryLatency,
		Confidence: 0.9,
	}
}

func (m *MockSCIPStore) CacheResponse(method string, params interface{}, response json.RawMessage) error {
	return nil
}

func (m *MockSCIPStore) InvalidateFile(filePath string) {
	// No-op for mock
}

func (m *MockSCIPStore) GetStats() indexing.SCIPStoreStats {
	return indexing.SCIPStoreStats{
		IndexesLoaded:    1,
		TotalQueries:     100,
		CacheHitRate:     0.8,
		AverageQueryTime: m.queryLatency,
		CacheSize:        50,
		MemoryUsage:      1024 * 1024, // 1MB
	}
}

func (m *MockSCIPStore) Close() error {
	return nil
}

func (m *MockSCIPStore) AddMockSymbol(symbol *MockSymbol) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.symbols = append(m.symbols, symbol)
}

func (m *MockSCIPStore) AddMockCrossLanguageRef(ref *MockCrossLanguageRef) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.crossLanguageRefs = append(m.crossLanguageRefs, ref)
}

func (m *MockSCIPStore) SetFailureMode(enabled bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.failureMode = enabled
}

func (m *MockSCIPStore) SetQueryLatency(latency time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.queryLatency = latency
}

// MockSymbolResolver implements a mock symbol resolver for testing
type MockSymbolResolver struct {
	resolveLatency time.Duration
}

func NewMockSymbolResolver() *MockSymbolResolver {
	return &MockSymbolResolver{
		resolveLatency: 5 * time.Millisecond,
	}
}

func (m *MockSymbolResolver) ResolveSymbolAtPosition(uri string, position indexing.Position) (*indexing.ResolvedSymbol, error) {
	time.Sleep(m.resolveLatency)
	
	return &indexing.ResolvedSymbol{
		Symbol:      "mockSymbol",
		DisplayName: "Mock Symbol",
		URI:         uri,
		Confidence:  0.9,
		Documentation: "Mock symbol for testing",
	}, nil
}

func (m *MockSymbolResolver) GetStats() *indexing.ResolverStats {
	return &indexing.ResolverStats{
		TotalResolutions:     100,
		SuccessfulResolutions: 95,
		AvgResolutionTime:    m.resolveLatency,
		CacheHitRate:         0.8,
		SymbolCount:          1000,
		DocumentCount:        50,
	}
}

func (m *MockSymbolResolver) Close() error {
	return nil
}

// MockLSPGatewayClient implements a mock LSP gateway client for testing
type MockLSPGatewayClient struct {
	responseLatency time.Duration
}

func NewMockLSPGatewayClient() *MockLSPGatewayClient {
	return &MockLSPGatewayClient{
		responseLatency: 20 * time.Millisecond,
	}
}

func (m *MockLSPGatewayClient) SendLSPRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	time.Sleep(m.responseLatency)
	
	// Return mock responses based on method
	switch method {
	case "textDocument/definition":
		response := []map[string]interface{}{
			{
				"uri": "file:///test/example.go",
				"range": map[string]interface{}{
					"start": map[string]interface{}{"line": 10, "character": 0},
					"end":   map[string]interface{}{"line": 10, "character": 10},
				},
			},
		}
		return json.Marshal(response)
	case "textDocument/references":
		response := []map[string]interface{}{
			{
				"uri": "file:///test/example.go",
				"range": map[string]interface{}{
					"start": map[string]interface{}{"line": 15, "character": 4},
					"end":   map[string]interface{}{"line": 15, "character": 14},
				},
			},
		}
		return json.Marshal(response)
	case "workspace/symbol":
		response := []map[string]interface{}{
			{
				"name": "TestClass",
				"kind": 5,
				"location": map[string]interface{}{
					"uri": "file:///test/example.go",
					"range": map[string]interface{}{
						"start": map[string]interface{}{"line": 10, "character": 0},
						"end":   map[string]interface{}{"line": 50, "character": 1},
					},
				},
			},
		}
		return json.Marshal(response)
	default:
		return json.Marshal(map[string]interface{}{"result": "mock_response"})
	}
}

// Helper functions

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// Benchmark tests for performance validation

func BenchmarkSCIPIntelligentSymbolSearch(b *testing.B) {
	suite := NewSCIPMCPTestSuite(&testing.T{})
	defer suite.Cleanup()
	
	ctx := context.Background()
	call := mcp.ToolCall{
		Name:      "scip_intelligent_symbol_search",
		Arguments: map[string]interface{}{"query": "TestClass"},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := suite.integration.CallTool(ctx, call)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSCIPCrossLanguageReferences(b *testing.B) {
	suite := NewSCIPMCPTestSuite(&testing.T{})
	defer suite.Cleanup()
	
	ctx := context.Background()
	call := mcp.ToolCall{
		Name: "scip_cross_language_references",
		Arguments: map[string]interface{}{
			"uri":       "file:///test/example.go",
			"line":      15,
			"character": 4,
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := suite.integration.CallTool(ctx, call)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkConcurrentSCIPRequests(b *testing.B) {
	suite := NewSCIPMCPTestSuite(&testing.T{})
	defer suite.Cleanup()
	
	ctx := context.Background()
	call := mcp.ToolCall{
		Name:      "scip_intelligent_symbol_search",
		Arguments: map[string]interface{}{"query": "TestClass"},
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := suite.integration.CallTool(ctx, call)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}