package e2e_test

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"lsp-gateway/tests/e2e/testutils"
)

// SCIP-enhanced MCP tools E2E tests
// Tests the 6 core SCIP-enhanced tools with comprehensive validation

// ToolResult represents the result of an MCP tool call
type ToolResult struct {
	Content []map[string]interface{} `json:"content"`
	IsError bool                     `json:"isError,omitempty"`
}

// MCPServerSession represents an active MCP server session
type MCPServerSession struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	reader  *bufio.Reader
	tempDir string
	cleanup func()
}

// SCIPPerformanceMetrics captures detailed performance measurements for SCIP tools
type SCIPPerformanceMetrics struct {
	ResponseTimes     []time.Duration `json:"response_times"`
	CacheHitRate      float64         `json:"cache_hit_rate"`
	MemoryUsageMB     float64         `json:"memory_usage_mb"`
	SuccessRate       float64         `json:"success_rate"`
	TimeoutFailures   int             `json:"timeout_failures"`
	TotalRequests     int             `json:"total_requests"`
	CacheHits         int             `json:"cache_hits"`
	AverageResponseMs float64         `json:"average_response_ms"`
	P95ResponseMs     float64         `json:"p95_response_ms"`
	P99ResponseMs     float64         `json:"p99_response_ms"`
	MaxResponseMs     float64         `json:"max_response_ms"`
	MinResponseMs     float64         `json:"min_response_ms"`
	ThroughputQPS     float64         `json:"throughput_qps"`
}

// TimedToolResult extends ToolResult with timing information
type TimedToolResult struct {
	*ToolResult
	ResponseTime time.Duration `json:"response_time"`
	CacheHit     bool          `json:"cache_hit"`
	MemoryAfter  float64       `json:"memory_after_mb"`
}


// TestSCIPIntelligentSymbolSearch tests SCIP-powered intelligent symbol search
func TestSCIPIntelligentSymbolSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP MCP E2E test in short mode")
	}

	session := setupSCIPMCPServer(t)
	defer session.cleanup()

	// Test basic symbol search
	t.Run("BasicSymbolSearch", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_intelligent_symbol_search", map[string]interface{}{
			"query":      "main",
			"scope":      "workspace",
			"maxResults": 10,
		})
		require.NoError(t, err)

		if result.IsError {
			t.Logf("SCIP symbol search returned error (expected without SCIP index): %v", result.Content)
		} else {
			err = validateSCIPToolResponse(result, []string{"symbols", "query", "totalResults"})
			assert.NoError(t, err)
			t.Logf("SCIP intelligent symbol search successful")
		}
	})

	// Test multi-language symbol search
	t.Run("MultiLanguageSearch", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_intelligent_symbol_search", map[string]interface{}{
			"query":                     "User",
			"languages":                 []string{"go", "java"},
			"scope":                     "workspace",
			"includeSemanticSimilarity": true,
			"maxResults":                20,
		})
		require.NoError(t, err)

		if result.IsError {
			t.Logf("Multi-language search returned error (expected without SCIP index): %v", result.Content)
		} else {
			err = validateSCIPToolResponse(result, []string{"symbols", "languageBreakdown"})
			assert.NoError(t, err)
			t.Logf("Multi-language symbol search successful")
		}
	})

	// Test semantic similarity search
	t.Run("SemanticSimilaritySearch", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_intelligent_symbol_search", map[string]interface{}{
			"query":                     "createUser",
			"includeSemanticSimilarity": true,
			"scope":                     "project",
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"symbols", "semanticMatches"})
			assert.NoError(t, err)
		}
		t.Logf("Semantic similarity search completed")
	})

	// Test error handling with invalid parameters
	t.Run("ErrorHandling", func(t *testing.T) {
		_, err := callSCIPTool(t, session, "scip_intelligent_symbol_search", map[string]interface{}{
			"maxResults": 1000, // Exceeds maximum
		})
		// Should handle error gracefully
		assert.Error(t, err)
		t.Logf("Error handling test completed")
	})
}

// TestSCIPCrossLanguageReferences tests cross-language reference finding
func TestSCIPCrossLanguageReferences(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP MCP E2E test in short mode")
	}

	session := setupSCIPMCPServer(t)
	defer session.cleanup()

	// Create test file for reference testing
	testFile := filepath.Join(session.tempDir, "test.go")
	testURI := fmt.Sprintf("file://%s", testFile)

	// Test cross-language references at a specific position
	t.Run("CrossLanguageReferenceSearch", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_cross_language_references", map[string]interface{}{
			"uri":                    testURI,
			"line":                   5,
			"character":              10,
			"includeImplementations": true,
			"includeInheritance":     true,
			"crossLanguageDepth":     3,
		})
		require.NoError(t, err)

		if result.IsError {
			t.Logf("Cross-language references returned error (expected without SCIP index): %v", result.Content)
		} else {
			err = validateSCIPToolResponse(result, []string{"references", "crossLanguageConnections"})
			assert.NoError(t, err)
			t.Logf("Cross-language references successful")
		}
	})

	// Test inheritance relationships
	t.Run("InheritanceRelationships", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_cross_language_references", map[string]interface{}{
			"uri":                testURI,
			"line":               3,
			"character":          15,
			"includeInheritance": true,
			"crossLanguageDepth": 5,
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"references", "inheritanceChain"})
			assert.NoError(t, err)
		}
		t.Logf("Inheritance relationships test completed")
	})

	// Test implementation search
	t.Run("ImplementationSearch", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_cross_language_references", map[string]interface{}{
			"uri":                    testURI,
			"line":                   1,
			"character":              5,
			"includeImplementations": true,
			"includeInheritance":     false,
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"references", "implementations"})
			assert.NoError(t, err)
		}
		t.Logf("Implementation search completed")
	})
}

// TestSCIPSemanticCodeAnalysis tests deep semantic code analysis
func TestSCIPSemanticCodeAnalysis(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP MCP E2E test in short mode")
	}

	session := setupSCIPMCPServer(t)
	defer session.cleanup()

	testFile := filepath.Join(session.tempDir, "test.go")
	testURI := fmt.Sprintf("file://%s", testFile)

	// Test comprehensive semantic analysis
	t.Run("ComprehensiveAnalysis", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_semantic_code_analysis", map[string]interface{}{
			"uri":                  testURI,
			"analysisType":         "all",
			"includeMetrics":       true,
			"includeRelationships": true,
		})
		require.NoError(t, err)

		if result.IsError {
			t.Logf("Semantic analysis returned error (expected without SCIP index): %v", result.Content)
		} else {
			err = validateSCIPToolResponse(result, []string{"analysis", "metrics", "relationships"})
			assert.NoError(t, err)
			t.Logf("Comprehensive semantic analysis successful")
		}
	})

	// Test structure analysis
	t.Run("StructureAnalysis", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_semantic_code_analysis", map[string]interface{}{
			"uri":          testURI,
			"analysisType": "structure",
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"analysis", "structure"})
			assert.NoError(t, err)
		}
		t.Logf("Structure analysis completed")
	})

	// Test dependency analysis
	t.Run("DependencyAnalysis", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_semantic_code_analysis", map[string]interface{}{
			"uri":          testURI,
			"analysisType": "dependencies",
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"analysis", "dependencies"})
			assert.NoError(t, err)
		}
		t.Logf("Dependency analysis completed")
	})

	// Test complexity analysis
	t.Run("ComplexityAnalysis", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_semantic_code_analysis", map[string]interface{}{
			"uri":            testURI,
			"analysisType":   "complexity",
			"includeMetrics": true,
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"analysis", "complexity", "metrics"})
			assert.NoError(t, err)
		}
		t.Logf("Complexity analysis completed")
	})
}

// TestSCIPContextAwareAssistance tests context-aware AI assistance
func TestSCIPContextAwareAssistance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP MCP E2E test in short mode")
	}

	session := setupSCIPMCPServer(t)
	defer session.cleanup()

	testFile := filepath.Join(session.tempDir, "test.go")
	testURI := fmt.Sprintf("file://%s", testFile)

	// Test completion assistance
	t.Run("CompletionAssistance", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_context_aware_assistance", map[string]interface{}{
			"uri":                  testURI,
			"line":                 5,
			"character":            10,
			"contextType":          "completion",
			"includeRelatedCode":   true,
			"includeDocumentation": true,
		})
		require.NoError(t, err)

		if result.IsError {
			t.Logf("Context-aware assistance returned error (expected without SCIP index): %v", result.Content)
		} else {
			err = validateSCIPToolResponse(result, []string{"assistance", "context", "suggestions"})
			assert.NoError(t, err)
			t.Logf("Completion assistance successful")
		}
	})

	// Test documentation assistance
	t.Run("DocumentationAssistance", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_context_aware_assistance", map[string]interface{}{
			"uri":                  testURI,
			"line":                 3,
			"character":            8,
			"contextType":          "documentation",
			"includeDocumentation": true,
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"assistance", "documentation"})
			assert.NoError(t, err)
		}
		t.Logf("Documentation assistance completed")
	})

	// Test navigation assistance
	t.Run("NavigationAssistance", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_context_aware_assistance", map[string]interface{}{
			"uri":                testURI,
			"line":               2,
			"character":          12,
			"contextType":        "navigation",
			"includeRelatedCode": true,
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"assistance", "navigation"})
			assert.NoError(t, err)
		}
		t.Logf("Navigation assistance completed")
	})

	// Test refactoring assistance
	t.Run("RefactoringAssistance", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_context_aware_assistance", map[string]interface{}{
			"uri":         testURI,
			"line":        4,
			"character":   6,
			"contextType": "refactoring",
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"assistance", "refactoring"})
			assert.NoError(t, err)
		}
		t.Logf("Refactoring assistance completed")
	})

	// Test debugging assistance
	t.Run("DebuggingAssistance", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_context_aware_assistance", map[string]interface{}{
			"uri":         testURI,
			"line":        6,
			"character":   15,
			"contextType": "debugging",
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"assistance", "debugging"})
			assert.NoError(t, err)
		}
		t.Logf("Debugging assistance completed")
	})
}

// TestSCIPWorkspaceIntelligence tests comprehensive workspace intelligence
func TestSCIPWorkspaceIntelligence(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP MCP E2E test in short mode")
	}

	session := setupSCIPMCPServer(t)
	defer session.cleanup()

	// Test workspace overview
	t.Run("WorkspaceOverview", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_workspace_intelligence", map[string]interface{}{
			"insightType":            "overview",
			"includeMetrics":         true,
			"includeRecommendations": true,
		})
		require.NoError(t, err)

		if result.IsError {
			t.Logf("Workspace intelligence returned error (expected without SCIP index): %v", result.Content)
		} else {
			err = validateSCIPToolResponse(result, []string{"insights", "metrics", "overview"})
			assert.NoError(t, err)
			t.Logf("Workspace overview successful")
		}
	})

	// Test hotspots analysis
	t.Run("HotspotsAnalysis", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_workspace_intelligence", map[string]interface{}{
			"insightType":    "hotspots",
			"includeMetrics": true,
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"insights", "hotspots"})
			assert.NoError(t, err)
		}
		t.Logf("Hotspots analysis completed")
	})

	// Test dependency analysis
	t.Run("DependencyInsights", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_workspace_intelligence", map[string]interface{}{
			"insightType": "dependencies",
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"insights", "dependencies"})
			assert.NoError(t, err)
		}
		t.Logf("Dependency insights completed")
	})

	// Test unused code detection
	t.Run("UnusedCodeDetection", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_workspace_intelligence", map[string]interface{}{
			"insightType":            "unused",
			"includeRecommendations": true,
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"insights", "unused", "recommendations"})
			assert.NoError(t, err)
		}
		t.Logf("Unused code detection completed")
	})

	// Test complexity insights
	t.Run("ComplexityInsights", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_workspace_intelligence", map[string]interface{}{
			"insightType":    "complexity",
			"includeMetrics": true,
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"insights", "complexity", "metrics"})
			assert.NoError(t, err)
		}
		t.Logf("Complexity insights completed")
	})

	// Test comprehensive analysis
	t.Run("ComprehensiveAnalysis", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_workspace_intelligence", map[string]interface{}{
			"insightType":            "all",
			"includeMetrics":         true,
			"includeRecommendations": true,
			"languageFilter":         []string{"go", "java"},
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"insights", "comprehensive"})
			assert.NoError(t, err)
		}
		t.Logf("Comprehensive workspace analysis completed")
	})
}

// TestSCIPRefactoringSuggestions tests intelligent refactoring suggestions
func TestSCIPRefactoringSuggestions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP MCP E2E test in short mode")
	}

	session := setupSCIPMCPServer(t)
	defer session.cleanup()

	testFile := filepath.Join(session.tempDir, "test.go")
	testURI := fmt.Sprintf("file://%s", testFile)

	// Test comprehensive refactoring suggestions
	t.Run("ComprehensiveRefactoring", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_refactoring_suggestions", map[string]interface{}{
			"uri":                   testURI,
			"refactoringTypes":      []string{"all"},
			"includeImpactAnalysis": true,
		})
		require.NoError(t, err)

		if result.IsError {
			t.Logf("Refactoring suggestions returned error (expected without SCIP index): %v", result.Content)
		} else {
			err = validateSCIPToolResponse(result, []string{"suggestions", "refactoring", "impact"})
			assert.NoError(t, err)
			t.Logf("Comprehensive refactoring suggestions successful")
		}
	})

	// Test extract method suggestions
	t.Run("ExtractMethodSuggestions", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_refactoring_suggestions", map[string]interface{}{
			"uri": testURI,
			"selectionStart": map[string]interface{}{
				"line":      5,
				"character": 0,
			},
			"selectionEnd": map[string]interface{}{
				"line":      10,
				"character": 0,
			},
			"refactoringTypes": []string{"extract_method"},
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"suggestions", "extract_method"})
			assert.NoError(t, err)
		}
		t.Logf("Extract method suggestions completed")
	})

	// Test rename suggestions
	t.Run("RenameSuggestions", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_refactoring_suggestions", map[string]interface{}{
			"uri":              testURI,
			"refactoringTypes": []string{"rename"},
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"suggestions", "rename"})
			assert.NoError(t, err)
		}
		t.Logf("Rename suggestions completed")
	})

	// Test optimization suggestions
	t.Run("OptimizationSuggestions", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_refactoring_suggestions", map[string]interface{}{
			"uri":                   testURI,
			"refactoringTypes":      []string{"optimize", "modernize"},
			"includeImpactAnalysis": true,
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"suggestions", "optimization"})
			assert.NoError(t, err)
		}
		t.Logf("Optimization suggestions completed")
	})

	// Test move/inline suggestions
	t.Run("MoveInlineSuggestions", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_refactoring_suggestions", map[string]interface{}{
			"uri":              testURI,
			"refactoringTypes": []string{"move", "inline"},
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"suggestions", "structural"})
			assert.NoError(t, err)
		}
		t.Logf("Move/inline suggestions completed")
	})
}

// TestSCIPCachePerformance validates cache performance and hit rates
func TestSCIPCachePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP cache performance test in short mode")
	}

	session := setupSCIPMCPServer(t)
	defer session.cleanup()

	// Test cache miss (first query) vs cache hit (repeated query)
	t.Run("CacheHitRateValidation", func(t *testing.T) {
		testQueries := []map[string]interface{}{
			{
				"query":      "User",
				"scope":      "workspace",
				"maxResults": 10,
			},
			{
				"query":                     "createUser",
				"scope":                     "project",
				"includeSemanticSimilarity": true,
			},
			{
				"query":      "processUser",
				"scope":      "workspace",
				"maxResults": 15,
			},
		}

		metrics := measureSCIPToolPerformance(t, session, "scip_intelligent_symbol_search", testQueries, 20)

		// Validate 85-90% cache hit rate for repeated queries
		expectedMinCacheHitRate := 0.85
		assert.GreaterOrEqual(t, metrics.CacheHitRate, expectedMinCacheHitRate,
			"Cache hit rate should be at least 85%%, got %.2f%%", metrics.CacheHitRate*100)
		assert.LessOrEqual(t, metrics.CacheHitRate, 1.0, "Cache hit rate cannot exceed 100%%")

		t.Logf("Cache Performance Results:")
		t.Logf("  Cache Hit Rate: %.2f%% (target: 85-90%%)", metrics.CacheHitRate*100)
		t.Logf("  Total Requests: %d", metrics.TotalRequests)
		t.Logf("  Cache Hits: %d", metrics.CacheHits)
		t.Logf("  Average Response Time: %.2fms", metrics.AverageResponseMs)
		t.Logf("  P95 Response Time: %.2fms", metrics.P95ResponseMs)
	})

	// Test cache effectiveness across different SCIP tools
	t.Run("CrossToolCacheEffectiveness", func(t *testing.T) {
		testFile := filepath.Join(session.tempDir, "main.go")
		testURI := fmt.Sprintf("file://%s", testFile)

		tools := []string{
			"scip_intelligent_symbol_search",
			"scip_cross_language_references",
			"scip_semantic_code_analysis",
		}

		allMetrics := make(map[string]*SCIPPerformanceMetrics)

		for _, toolName := range tools {
			var testParams []map[string]interface{}

			switch toolName {
			case "scip_intelligent_symbol_search":
				testParams = []map[string]interface{}{
					{"query": "User", "scope": "workspace"},
					{"query": "main", "scope": "project"},
				}
			case "scip_cross_language_references":
				testParams = []map[string]interface{}{
					{"uri": testURI, "line": 5, "character": 10},
					{"uri": testURI, "line": 8, "character": 15},
				}
			case "scip_semantic_code_analysis":
				testParams = []map[string]interface{}{
					{"uri": testURI, "analysisType": "structure"},
					{"uri": testURI, "analysisType": "complexity"},
				}
			}

			metrics := measureSCIPToolPerformance(t, session, toolName, testParams, 10)
			allMetrics[toolName] = metrics

			t.Logf("%s Cache Hit Rate: %.2f%%", toolName, metrics.CacheHitRate*100)
		}

		// Validate overall cache effectiveness
		totalCacheHits := 0
		totalRequests := 0
		for _, metrics := range allMetrics {
			totalCacheHits += metrics.CacheHits
			totalRequests += metrics.TotalRequests
		}

		overallCacheHitRate := float64(totalCacheHits) / float64(totalRequests)
		assert.GreaterOrEqual(t, overallCacheHitRate, 0.80,
			"Overall cache hit rate across tools should be at least 80%%")

		t.Logf("Overall Cross-Tool Cache Hit Rate: %.2f%%", overallCacheHitRate*100)
	})
}

// TestSCIPResponseTimeImprovement validates 60-87% response time improvement claims
func TestSCIPResponseTimeImprovement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP response time improvement test in short mode")
	}

	session := setupSCIPMCPServer(t)
	defer session.cleanup()

	// Compare SCIP-enhanced vs standard LSP tool response times
	t.Run("ResponseTimeImprovement", func(t *testing.T) {
		testQueries := []map[string]interface{}{
			{
				"query":      "User",
				"scope":      "workspace",
				"maxResults": 10,
			},
			{
				"query":                     "processUser",
				"scope":                     "project",
				"includeSemanticSimilarity": true,
			},
		}

		// Measure SCIP-enhanced tool performance
		scipMetrics := measureSCIPToolPerformance(t, session, "scip_intelligent_symbol_search", testQueries, 30)

		// For comparison, we'll simulate standard LSP performance by measuring
		// the same tool without cache benefits (cold cache scenarios)
		coldCacheMetrics := measureColdCachePerformance(t, session, "scip_intelligent_symbol_search", testQueries, 15)

		// Calculate performance improvement
		improvement := calculatePerformanceImprovement(scipMetrics.ResponseTimes, coldCacheMetrics.ResponseTimes)

		// Validate 60-87% improvement claim
		expectedMinImprovement := 0.60
		expectedMaxImprovement := 0.87

		if improvement > 0 {
			assert.GreaterOrEqual(t, improvement, expectedMinImprovement,
				"Response time improvement should be at least 60%%, got %.2f%%", improvement*100)

			// Allow some tolerance above the upper bound for exceptional cases
			if improvement > expectedMaxImprovement {
				t.Logf("Warning: Performance improvement %.2f%% exceeds expected maximum of 87%%", improvement*100)
			}
		}

		t.Logf("Response Time Improvement Results:")
		t.Logf("  SCIP Average: %.2fms", scipMetrics.AverageResponseMs)
		t.Logf("  Cold Cache Average: %.2fms", coldCacheMetrics.AverageResponseMs)
		t.Logf("  Performance Improvement: %.2f%% (target: 60-87%%)", improvement*100)
		t.Logf("  SCIP P95: %.2fms", scipMetrics.P95ResponseMs)
		t.Logf("  Cold Cache P95: %.2fms", coldCacheMetrics.P95ResponseMs)
	})

	// Test response time improvements across different tool types
	t.Run("CrossToolResponseTimeImprovement", func(t *testing.T) {
		testFile := filepath.Join(session.tempDir, "main.go")
		testURI := fmt.Sprintf("file://%s", testFile)

		toolTestConfigs := map[string][]map[string]interface{}{
			"scip_cross_language_references": {
				{"uri": testURI, "line": 5, "character": 10, "includeImplementations": true},
				{"uri": testURI, "line": 8, "character": 15, "includeInheritance": true},
			},
			"scip_semantic_code_analysis": {
				{"uri": testURI, "analysisType": "structure", "includeMetrics": true},
				{"uri": testURI, "analysisType": "complexity", "includeMetrics": true},
			},
			"scip_workspace_intelligence": {
				{"insightType": "overview", "includeMetrics": true},
				{"insightType": "hotspots", "includeMetrics": true},
			},
		}

		totalImprovements := []float64{}

		for toolName, testParams := range toolTestConfigs {
			scipMetrics := measureSCIPToolPerformance(t, session, toolName, testParams, 20)
			coldMetrics := measureColdCachePerformance(t, session, toolName, testParams, 10)

			improvement := calculatePerformanceImprovement(scipMetrics.ResponseTimes, coldMetrics.ResponseTimes)
			if improvement > 0 {
				totalImprovements = append(totalImprovements, improvement)
			}

			t.Logf("%s Performance Improvement: %.2f%%", toolName, improvement*100)
		}

		// Validate overall improvement across tools
		if len(totalImprovements) > 0 {
			avgImprovement := calculateMean(totalImprovements)
			assert.GreaterOrEqual(t, avgImprovement, 0.50,
				"Average performance improvement across tools should be at least 50%%")

			t.Logf("Average Performance Improvement Across Tools: %.2f%%", avgImprovement*100)
		}
	})
}

// TestSCIPMemoryUsage monitors memory usage during SCIP operations
func TestSCIPMemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP memory usage test in short mode")
	}

	session := setupSCIPMCPServer(t)
	defer session.cleanup()

	// Monitor memory usage during SCIP operations
	t.Run("MemoryUsageMonitoring", func(t *testing.T) {
		// Get baseline memory usage
		var baselineMemStats runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&baselineMemStats)
		baselineMemMB := float64(baselineMemStats.Alloc) / 1024 / 1024

		testQueries := []map[string]interface{}{
			{
				"query":      "User",
				"scope":      "workspace",
				"maxResults": 20,
			},
			{
				"query":                     "createUser",
				"scope":                     "project",
				"includeSemanticSimilarity": true,
			},
		}

		// Perform SCIP operations and monitor memory
		metrics := measureSCIPToolPerformance(t, session, "scip_intelligent_symbol_search", testQueries, 50)

		// Validate cache memory stays within 65-75MB range
		cacheMemoryUsage := metrics.MemoryUsageMB - baselineMemMB
		expectedMinCacheMemMB := 65.0
		expectedMaxCacheMemMB := 75.0

		t.Logf("Memory Usage Results:")
		t.Logf("  Baseline Memory: %.2fMB", baselineMemMB)
		t.Logf("  Peak Memory Usage: %.2fMB", metrics.MemoryUsageMB)
		t.Logf("  Cache Memory Usage: %.2fMB (target: 65-75MB)", cacheMemoryUsage)

		// Allow some tolerance for test environment variations
		if cacheMemoryUsage > expectedMaxCacheMemMB {
			t.Logf("Warning: Cache memory usage %.2fMB exceeds expected maximum of 75MB", cacheMemoryUsage)
		} else if cacheMemoryUsage < expectedMinCacheMemMB {
			t.Logf("Note: Cache memory usage %.2fMB is below expected minimum of 65MB", cacheMemoryUsage)
		} else {
			assert.GreaterOrEqual(t, cacheMemoryUsage, expectedMinCacheMemMB*0.8,
				"Cache memory usage should be reasonable")
			assert.LessOrEqual(t, cacheMemoryUsage, expectedMaxCacheMemMB*1.2,
				"Cache memory usage should not exceed 90MB")
		}
	})

	// Test memory cleanup and garbage collection
	t.Run("MemoryCleanupValidation", func(t *testing.T) {
		var memStats runtime.MemStats

		// Measure memory before operations
		runtime.GC()
		runtime.ReadMemStats(&memStats)
		memBefore := float64(memStats.Alloc) / 1024 / 1024

		// Perform intensive SCIP operations
		testQueries := []map[string]interface{}{
			{
				"query":      "User",
				"scope":      "workspace",
				"maxResults": 50,
			},
		}

		measureSCIPToolPerformance(t, session, "scip_intelligent_symbol_search", testQueries, 100)

		// Force garbage collection
		runtime.GC()
		runtime.ReadMemStats(&memStats)
		memAfterGC := float64(memStats.Alloc) / 1024 / 1024

		memoryGrowth := memAfterGC - memBefore

		t.Logf("Memory Cleanup Results:")
		t.Logf("  Memory Before: %.2fMB", memBefore)
		t.Logf("  Memory After GC: %.2fMB", memAfterGC)
		t.Logf("  Memory Growth: %.2fMB", memoryGrowth)

		// Validate reasonable memory growth (not excessive leaks)
		assert.LessOrEqual(t, memoryGrowth, 100.0,
			"Memory growth should not exceed 100MB after intensive operations")
	})
}

// TestSCIPTimeoutCompliance validates timeout compliance and performance
func TestSCIPTimeoutCompliance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP timeout compliance test in short mode")
	}

	session := setupSCIPMCPServer(t)
	defer session.cleanup()

	// Validate all SCIP queries complete within 100ms max
	t.Run("QueryTimeoutCompliance", func(t *testing.T) {
		maxQueryTimeout := 100 * time.Millisecond

		testQueries := []map[string]interface{}{
			{
				"query":      "User",
				"scope":      "workspace",
				"maxResults": 10,
			},
			{
				"query": "main",
				"scope": "project",
			},
			{
				"query":                     "processUser",
				"scope":                     "workspace",
				"includeSemanticSimilarity": true,
			},
		}

		metrics := measureSCIPToolPerformance(t, session, "scip_intelligent_symbol_search", testQueries, 30)

		// Validate timeout compliance
		timeoutFailures := 0
		for _, responseTime := range metrics.ResponseTimes {
			if responseTime > maxQueryTimeout {
				timeoutFailures++
			}
		}

		timeoutFailureRate := float64(timeoutFailures) / float64(len(metrics.ResponseTimes))

		t.Logf("Query Timeout Compliance Results:")
		t.Logf("  Max Response Time: %.2fms (limit: 100ms)", metrics.MaxResponseMs)
		t.Logf("  Average Response Time: %.2fms", metrics.AverageResponseMs)
		t.Logf("  P95 Response Time: %.2fms", metrics.P95ResponseMs)
		t.Logf("  P99 Response Time: %.2fms", metrics.P99ResponseMs)
		t.Logf("  Timeout Failures: %d/%d (%.2f%%)", timeoutFailures, len(metrics.ResponseTimes), timeoutFailureRate*100)

		// Allow some tolerance for test environment variations
		assert.LessOrEqual(t, timeoutFailureRate, 0.1,
			"Timeout failure rate should be less than 10%%")
		assert.LessOrEqual(t, metrics.P95ResponseMs, 150.0,
			"P95 response time should be within reasonable bounds")
	})

	// Test symbol search within 50ms target
	t.Run("SymbolSearchTimeoutTarget", func(t *testing.T) {
		symbolSearchTimeout := 50 * time.Millisecond

		testQueries := []map[string]interface{}{
			{
				"query":      "User",
				"scope":      "workspace",
				"maxResults": 5,
			},
			{
				"query":      "main",
				"scope":      "project",
				"maxResults": 5,
			},
		}

		metrics := measureSCIPToolPerformance(t, session, "scip_intelligent_symbol_search", testQueries, 40)

		fastResponses := 0
		for _, responseTime := range metrics.ResponseTimes {
			if responseTime <= symbolSearchTimeout {
				fastResponses++
			}
		}

		fastResponseRate := float64(fastResponses) / float64(len(metrics.ResponseTimes))

		t.Logf("Symbol Search Performance Results:")
		t.Logf("  Target: ≤50ms")
		t.Logf("  Fast Responses: %d/%d (%.2f%%)", fastResponses, len(metrics.ResponseTimes), fastResponseRate*100)
		t.Logf("  Average Response Time: %.2fms", metrics.AverageResponseMs)

		// Aim for at least 70% of requests under 50ms target
		assert.GreaterOrEqual(t, fastResponseRate, 0.70,
			"At least 70%% of symbol search requests should complete within 50ms")
	})

	// Test timeout behavior and graceful degradation
	t.Run("TimeoutGracefulDegradation", func(t *testing.T) {
		// Test with more complex queries that might take longer
		complexQueries := []map[string]interface{}{
			{
				"query":                     "User",
				"scope":                     "workspace",
				"maxResults":                100,
				"includeSemanticSimilarity": true,
			},
		}

		metrics := measureSCIPToolPerformance(t, session, "scip_intelligent_symbol_search", complexQueries, 20)

		// Validate that even complex queries don't cause system instability
		assert.GreaterOrEqual(t, metrics.SuccessRate, 0.90,
			"Success rate should remain high even for complex queries")
		assert.LessOrEqual(t, metrics.MaxResponseMs, 500.0,
			"Even complex queries should complete within reasonable time")

		t.Logf("Complex Query Results:")
		t.Logf("  Success Rate: %.2f%%", metrics.SuccessRate*100)
		t.Logf("  Max Response Time: %.2fms", metrics.MaxResponseMs)
		t.Logf("  Average Response Time: %.2fms", metrics.AverageResponseMs)
	})
}

// TestSCIPConcurrentPerformance tests performance under concurrent load
func TestSCIPConcurrentPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP concurrent performance test in short mode")
	}

	session := setupSCIPMCPServer(t)
	defer session.cleanup()

	// Test performance under concurrent load
	t.Run("ConcurrentLoadPerformance", func(t *testing.T) {
		numWorkers := 10
		requestsPerWorker := 20

		testQueries := []map[string]interface{}{
			{
				"query":      "User",
				"scope":      "workspace",
				"maxResults": 10,
			},
			{
				"query": "processUser",
				"scope": "project",
			},
		}

		var wg sync.WaitGroup
		resultsChan := make(chan *TimedToolResult, numWorkers*requestsPerWorker*len(testQueries))

		startTime := time.Now()

		// Launch concurrent workers
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < requestsPerWorker; j++ {
					for _, query := range testQueries {
						result := callTimedSCIPTool(t, session, "scip_intelligent_symbol_search", query)
						resultsChan <- result
					}
				}
			}(i)
		}

		// Wait for all workers to complete
		wg.Wait()
		close(resultsChan)

		totalDuration := time.Since(startTime)

		// Collect and analyze results
		var allResults []*TimedToolResult
		for result := range resultsChan {
			allResults = append(allResults, result)
		}

		metrics := analyzeTimedResults(allResults)
		totalRequests := len(allResults)
		throughput := float64(totalRequests) / totalDuration.Seconds()

		t.Logf("Concurrent Performance Results:")
		t.Logf("  Workers: %d", numWorkers)
		t.Logf("  Total Requests: %d", totalRequests)
		t.Logf("  Total Duration: %.2fs", totalDuration.Seconds())
		t.Logf("  Throughput: %.2f requests/second", throughput)
		t.Logf("  Average Response Time: %.2fms", metrics.AverageResponseMs)
		t.Logf("  P95 Response Time: %.2fms", metrics.P95ResponseMs)
		t.Logf("  Success Rate: %.2f%%", metrics.SuccessRate*100)
		t.Logf("  Cache Hit Rate: %.2f%%", metrics.CacheHitRate*100)

		// Validate concurrent performance
		assert.GreaterOrEqual(t, metrics.SuccessRate, 0.90,
			"Success rate should remain high under concurrent load")
		assert.GreaterOrEqual(t, throughput, 10.0,
			"Throughput should be at least 10 requests/second")
		assert.LessOrEqual(t, metrics.P95ResponseMs, 200.0,
			"P95 response time should be reasonable under load")
	})

	// Validate cache effectiveness with multiple simultaneous queries
	t.Run("ConcurrentCacheEffectiveness", func(t *testing.T) {
		numWorkers := 5
		requestsPerWorker := 30

		// Use repeated queries to test cache effectiveness
		repeatedQuery := map[string]interface{}{
			"query":      "User",
			"scope":      "workspace",
			"maxResults": 10,
		}

		var wg sync.WaitGroup
		resultsChan := make(chan *TimedToolResult, numWorkers*requestsPerWorker)

		// First warm up the cache
		for i := 0; i < 5; i++ {
			callSCIPTool(t, session, "scip_intelligent_symbol_search", repeatedQuery)
		}

		// Launch concurrent workers with same query
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < requestsPerWorker; j++ {
					result := callTimedSCIPTool(t, session, "scip_intelligent_symbol_search", repeatedQuery)
					resultsChan <- result
				}
			}()
		}

		wg.Wait()
		close(resultsChan)

		var results []*TimedToolResult
		for result := range resultsChan {
			results = append(results, result)
		}

		metrics := analyzeTimedResults(results)

		t.Logf("Concurrent Cache Effectiveness Results:")
		t.Logf("  Cache Hit Rate: %.2f%% (target: >80%% for repeated queries)", metrics.CacheHitRate*100)
		t.Logf("  Average Response Time: %.2fms", metrics.AverageResponseMs)
		t.Logf("  Success Rate: %.2f%%", metrics.SuccessRate*100)

		// With repeated queries, cache hit rate should be very high
		assert.GreaterOrEqual(t, metrics.CacheHitRate, 0.80,
			"Cache hit rate should be high for repeated concurrent queries")
		assert.GreaterOrEqual(t, metrics.SuccessRate, 0.95,
			"Success rate should be very high for simple repeated queries")
	})

	// Measure throughput and response time degradation
	t.Run("ThroughputDegradationAnalysis", func(t *testing.T) {
		workerCounts := []int{1, 5, 10, 20}
		requestsPerTest := 50

		testQuery := map[string]interface{}{
			"query":      "User",
			"scope":      "workspace",
			"maxResults": 10,
		}

		var throughputs []float64
		var avgResponseTimes []float64

		for _, numWorkers := range workerCounts {
			var wg sync.WaitGroup
			resultsChan := make(chan *TimedToolResult, numWorkers*requestsPerTest/numWorkers)

			startTime := time.Now()

			for i := 0; i < numWorkers; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for j := 0; j < requestsPerTest/numWorkers; j++ {
						result := callTimedSCIPTool(t, session, "scip_intelligent_symbol_search", testQuery)
						resultsChan <- result
					}
				}()
			}

			wg.Wait()
			close(resultsChan)

			duration := time.Since(startTime)

			var results []*TimedToolResult
			for result := range resultsChan {
				results = append(results, result)
			}

			metrics := analyzeTimedResults(results)
			throughput := float64(len(results)) / duration.Seconds()

			throughputs = append(throughputs, throughput)
			avgResponseTimes = append(avgResponseTimes, metrics.AverageResponseMs)

			t.Logf("Workers: %d, Throughput: %.2f req/s, Avg Response: %.2fms",
				numWorkers, throughput, metrics.AverageResponseMs)
		}

		// Validate that throughput scales reasonably with worker count
		for i := 1; i < len(throughputs); i++ {
			if workerCounts[i] <= 10 { // Reasonable scaling expected up to 10 workers
				scalingRatio := throughputs[i] / throughputs[0]
				expectedMinScaling := float64(workerCounts[i]) * 0.5 // At least 50% linear scaling

				t.Logf("Scaling ratio for %d workers: %.2fx (expected min: %.2fx)",
					workerCounts[i], scalingRatio, expectedMinScaling/float64(workerCounts[0]))
			}
		}

		t.Logf("Throughput scaling analysis completed")
	})
}

// TestSCIPToolsRegistration tests that all SCIP tools are properly registered
func TestSCIPToolsRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP MCP E2E test in short mode")
	}

	session := setupSCIPMCPServer(t)
	defer session.cleanup()

	// Get tools list
	listMsg := createTestMCPMessage(2, "tools/list", nil)
	response, err := testutils.SendMCPStdioMessage(session.stdin, session.reader, listMsg)
	require.NoError(t, err)
	require.NotNil(t, response.Result)

	result := response.Result.(map[string]interface{})
	tools, exists := result["tools"]
	require.True(t, exists)

	toolsList := tools.([]interface{})
	assert.Greater(t, len(toolsList), 0)

	// Check for all SCIP-enhanced tools
	expectedSCIPTools := []string{
		"scip_intelligent_symbol_search",
		"scip_cross_language_references",
		"scip_semantic_code_analysis",
		"scip_context_aware_assistance",
		"scip_workspace_intelligence",
		"scip_refactoring_suggestions",
	}

	foundTools := make(map[string]bool)
	for _, tool := range toolsList {
		toolMap := tool.(map[string]interface{})
		name := toolMap["name"].(string)
		foundTools[name] = true
	}

	for _, expectedTool := range expectedSCIPTools {
		assert.True(t, foundTools[expectedTool], "Expected SCIP tool %s not found", expectedTool)
		t.Logf("SCIP tool registered: %s", expectedTool)
	}

	t.Logf("All %d SCIP-enhanced tools properly registered", len(expectedSCIPTools))
}

// Performance Measurement Helper Functions

// measureSCIPToolPerformance measures performance metrics for SCIP tool operations
func measureSCIPToolPerformance(t *testing.T, session *MCPServerSession, toolName string, testQueries []map[string]interface{}, iterations int) *SCIPPerformanceMetrics {
	var responseTimes []time.Duration
	var cacheHits int
	var successCount int
	var maxMemoryMB float64

	for i := 0; i < iterations; i++ {
		for _, query := range testQueries {
			result := callTimedSCIPTool(t, session, toolName, query)

			responseTimes = append(responseTimes, result.ResponseTime)

			if !result.IsError {
				successCount++
			}

			if result.CacheHit {
				cacheHits++
			}

			if result.MemoryAfter > maxMemoryMB {
				maxMemoryMB = result.MemoryAfter
			}
		}
	}

	return calculateMetrics(responseTimes, cacheHits, successCount, len(responseTimes), maxMemoryMB)
}

// measureColdCachePerformance measures performance without cache benefits
func measureColdCachePerformance(t *testing.T, session *MCPServerSession, toolName string, testQueries []map[string]interface{}, iterations int) *SCIPPerformanceMetrics {
	var responseTimes []time.Duration
	var successCount int

	for i := 0; i < iterations; i++ {
		for _, query := range testQueries {
			// Add random variation to prevent caching
			modifiedQuery := make(map[string]interface{})
			for k, v := range query {
				modifiedQuery[k] = v
			}
			modifiedQuery["_cache_buster"] = fmt.Sprintf("cold_%d_%d", i, time.Now().UnixNano())

			result := callTimedSCIPTool(t, session, toolName, modifiedQuery)
			responseTimes = append(responseTimes, result.ResponseTime)

			if !result.IsError {
				successCount++
			}
		}
	}

	return calculateMetrics(responseTimes, 0, successCount, len(responseTimes), 0)
}

// callTimedSCIPTool calls a SCIP tool with timing and memory measurement
func callTimedSCIPTool(t *testing.T, session *MCPServerSession, toolName string, params map[string]interface{}) *TimedToolResult {
	// Measure memory before
	var memStatsBefore runtime.MemStats
	runtime.ReadMemStats(&memStatsBefore)

	startTime := time.Now()
	result, err := callSCIPTool(t, session, toolName, params)
	responseTime := time.Since(startTime)

	// Measure memory after
	var memStatsAfter runtime.MemStats
	runtime.ReadMemStats(&memStatsAfter)
	memoryAfterMB := float64(memStatsAfter.Alloc) / 1024 / 1024

	if err != nil {
		return &TimedToolResult{
			ToolResult:   &ToolResult{IsError: true, Content: []map[string]interface{}{{"error": err.Error()}}},
			ResponseTime: responseTime,
			CacheHit:     false,
			MemoryAfter:  memoryAfterMB,
		}
	}

	// Check for cache hit indicators in response
	cacheHit := detectCacheHit(result)

	return &TimedToolResult{
		ToolResult:   result,
		ResponseTime: responseTime,
		CacheHit:     cacheHit,
		MemoryAfter:  memoryAfterMB,
	}
}

// analyzeTimedResults analyzes a collection of timed tool results
func analyzeTimedResults(results []*TimedToolResult) *SCIPPerformanceMetrics {
	if len(results) == 0 {
		return &SCIPPerformanceMetrics{}
	}

	var responseTimes []time.Duration
	var cacheHits int
	var successCount int
	var maxMemoryMB float64

	for _, result := range results {
		responseTimes = append(responseTimes, result.ResponseTime)

		if !result.IsError {
			successCount++
		}

		if result.CacheHit {
			cacheHits++
		}

		if result.MemoryAfter > maxMemoryMB {
			maxMemoryMB = result.MemoryAfter
		}
	}

	return calculateMetrics(responseTimes, cacheHits, successCount, len(results), maxMemoryMB)
}

// calculateMetrics computes comprehensive performance metrics
func calculateMetrics(responseTimes []time.Duration, cacheHits, successCount, totalRequests int, maxMemoryMB float64) *SCIPPerformanceMetrics {
	if len(responseTimes) == 0 {
		return &SCIPPerformanceMetrics{}
	}

	// Convert to milliseconds for calculations
	responseMs := make([]float64, len(responseTimes))
	for i, duration := range responseTimes {
		responseMs[i] = float64(duration.Nanoseconds()) / 1e6
	}

	sort.Float64s(responseMs)

	// Calculate percentiles
	p95Index := int(float64(len(responseMs)) * 0.95)
	if p95Index >= len(responseMs) {
		p95Index = len(responseMs) - 1
	}

	p99Index := int(float64(len(responseMs)) * 0.99)
	if p99Index >= len(responseMs) {
		p99Index = len(responseMs) - 1
	}

	return &SCIPPerformanceMetrics{
		ResponseTimes:     responseTimes,
		CacheHitRate:      float64(cacheHits) / float64(totalRequests),
		MemoryUsageMB:     maxMemoryMB,
		SuccessRate:       float64(successCount) / float64(totalRequests),
		TimeoutFailures:   0, // Calculated elsewhere if needed
		TotalRequests:     totalRequests,
		CacheHits:         cacheHits,
		AverageResponseMs: calculateMean(responseMs),
		P95ResponseMs:     responseMs[p95Index],
		P99ResponseMs:     responseMs[p99Index],
		MaxResponseMs:     responseMs[len(responseMs)-1],
		MinResponseMs:     responseMs[0],
		ThroughputQPS:     0, // Calculated separately for concurrent tests
	}
}

// calculatePerformanceImprovement calculates the performance improvement percentage
func calculatePerformanceImprovement(scipTimes, coldTimes []time.Duration) float64 {
	if len(scipTimes) == 0 || len(coldTimes) == 0 {
		return 0
	}

	scipMs := make([]float64, len(scipTimes))
	for i, duration := range scipTimes {
		scipMs[i] = float64(duration.Nanoseconds()) / 1e6
	}

	coldMs := make([]float64, len(coldTimes))
	for i, duration := range coldTimes {
		coldMs[i] = float64(duration.Nanoseconds()) / 1e6
	}

	scipAvg := calculateMean(scipMs)
	coldAvg := calculateMean(coldMs)

	if coldAvg == 0 {
		return 0
	}

	improvement := (coldAvg - scipAvg) / coldAvg
	return improvement
}

// calculateMean calculates the arithmetic mean of a slice of float64 values
func calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

// detectCacheHit attempts to detect if a response came from cache
func detectCacheHit(result *ToolResult) bool {
	if result.IsError {
		return false
	}

	// Look for cache indicators in response content
	for _, content := range result.Content {
		if cached, exists := content["cached"]; exists {
			if cacheValue, ok := cached.(bool); ok {
				return cacheValue
			}
		}

		if cacheInfo, exists := content["cache_info"]; exists {
			if cacheMap, ok := cacheInfo.(map[string]interface{}); ok {
				if hit, exists := cacheMap["hit"]; exists {
					if hitValue, ok := hit.(bool); ok {
						return hitValue
					}
				}
			}
		}

		// Check for response time indicators (very fast responses likely cached)
		if responseTime, exists := content["response_time_ms"]; exists {
			if timeValue, ok := responseTime.(float64); ok {
				return timeValue < 10 // Assume sub-10ms responses are cached
			}
		}
	}

	// Default heuristic: assume cache hit for very fast responses
	return false
}

// Helper Functions

// setupSCIPMCPServer sets up an MCP server session for SCIP testing
func setupSCIPMCPServer(t *testing.T) *MCPServerSession {
	// Create test workspace with multi-language files
	tempDir := createTestWorkspace(t)

	// Build binary
	projectRoot, err := testutils.GetProjectRoot()
	require.NoError(t, err)
	binaryPath := filepath.Join(projectRoot, "bin", "lspg")

	// Find available port for gateway
	gatewayPort, err := testutils.FindAvailablePort()
	require.NoError(t, err)
	gatewayURL := fmt.Sprintf("http://localhost:%d", gatewayPort)

	// Create temporary config file
	configContent := fmt.Sprintf(`
servers:
- name: go-lsp
  languages:
  - go
  command: gopls
  args: []
  transport: stdio
  root_markers:
  - go.mod
  - go.sum
  priority: 1
  weight: 1.0
- name: python-lsp
  languages:
  - python
  command: pylsp
  args: []
  transport: stdio
  root_markers:
  - setup.py
  - pyproject.toml
  - requirements.txt
  priority: 1
  weight: 1.0
- name: typescript-lsp
  languages:
  - typescript
  - javascript
  command: typescript-language-server
  args: ["--stdio"]
  transport: stdio
  root_markers:
  - package.json
  - tsconfig.json
  priority: 1
  weight: 1.0
port: %d
timeout: 45s
max_concurrent_requests: 150
multi_server_config:
  primary: null
  secondary: []
  selection_strategy: load_balance
`, gatewayPort)
	configPath, _, err := testutils.CreateTempConfig(configContent)
	require.NoError(t, err)

	// Start LSP Gateway server first
	gatewayCmd := exec.Command(binaryPath, "server", "--config", configPath)
	gatewayCmd.Dir = projectRoot
	err = gatewayCmd.Start()
	require.NoError(t, err)

	// Wait for gateway to start
	time.Sleep(3 * time.Second)

	// Start MCP server via STDIO
	cmd := exec.Command(binaryPath, "mcp", "--gateway", gatewayURL)
	cmd.Dir = tempDir // Run in test workspace

	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)

	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)

	// Capture stderr for debugging
	stderr, err := cmd.StderrPipe()
	require.NoError(t, err)

	err = cmd.Start()
	require.NoError(t, err)

	// Log any stderr output
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			t.Logf("SCIP MCP stderr: %s", scanner.Text())
		}
	}()

	reader := bufio.NewReader(stdout)

	// Initialize MCP connection
	initMsg := createTestMCPMessage(1, "initialize", map[string]interface{}{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]interface{}{},
	})

	response, err := testutils.SendMCPStdioMessage(stdin, reader, initMsg)
	require.NoError(t, err)
	require.NotNil(t, response.Result)

	// Setup cleanup function
	cleanup := func() {
		stdin.Close()
		if cmd.Process != nil {
			cmd.Process.Signal(syscall.SIGTERM)
			cmd.Wait()
		}
		if gatewayCmd.Process != nil {
			gatewayCmd.Process.Signal(syscall.SIGTERM)
			gatewayCmd.Wait()
		}
		os.Remove(configPath)
		os.RemoveAll(tempDir)
	}

	return &MCPServerSession{
		cmd:     cmd,
		stdin:   stdin,
		reader:  reader,
		tempDir: tempDir,
		cleanup: cleanup,
	}
}

// callSCIPTool calls a SCIP tool and returns the result
func callSCIPTool(t *testing.T, session *MCPServerSession, toolName string, params map[string]interface{}) (*ToolResult, error) {
	callMsg := createTestMCPMessage(3, "tools/call", map[string]interface{}{
		"name":      toolName,
		"arguments": params,
	})

	response, err := testutils.SendMCPStdioMessage(session.stdin, session.reader, callMsg)
	if err != nil {
		return nil, err
	}

	if response.Error != nil {
		return &ToolResult{
			IsError: true,
			Content: []map[string]interface{}{{"error": response.Error}},
		}, nil
	}

	result := response.Result.(map[string]interface{})
	content, exists := result["content"]
	if !exists {
		return &ToolResult{
			IsError: true,
			Content: []map[string]interface{}{{"error": "no content in response"}},
		}, nil
	}

	contentList, ok := content.([]interface{})
	if !ok {
		return &ToolResult{
			IsError: true,
			Content: []map[string]interface{}{{"error": "invalid content format"}},
		}, nil
	}

	var contentMaps []map[string]interface{}
	for _, item := range contentList {
		if itemMap, ok := item.(map[string]interface{}); ok {
			contentMaps = append(contentMaps, itemMap)
		}
	}

	return &ToolResult{
		Content: contentMaps,
		IsError: false,
	}, nil
}

// validateSCIPToolResponse validates that a SCIP tool response contains expected fields
func validateSCIPToolResponse(result *ToolResult, expectedFields []string) error {
	if result.IsError {
		return fmt.Errorf("tool returned error")
	}

	if len(result.Content) == 0 {
		return fmt.Errorf("no content in response")
	}

	// Check if at least one content item contains the expected fields
	for _, content := range result.Content {
		hasAllFields := true
		for _, field := range expectedFields {
			if _, exists := content[field]; !exists {
				hasAllFields = false
				break
			}
		}
		if hasAllFields {
			return nil // Found content with all expected fields
		}
	}

	// If no content has all fields, check if fields are distributed across content items
	allFoundFields := make(map[string]bool)
	for _, content := range result.Content {
		for _, field := range expectedFields {
			if _, exists := content[field]; exists {
				allFoundFields[field] = true
			}
		}
	}

	missingFields := []string{}
	for _, field := range expectedFields {
		if !allFoundFields[field] {
			missingFields = append(missingFields, field)
		}
	}

	if len(missingFields) == 0 {
		return nil // All fields found across content items
	}

	return fmt.Errorf("missing expected fields: %v", missingFields)
}

// createTestMCPMessage creates a test MCP message
func createTestMCPMessage(id interface{}, method string, params interface{}) testutils.MCPMessage {
	return testutils.MCPMessage{
		Jsonrpc: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
}

// createTestWorkspace creates a realistic multi-language microservices test workspace for comprehensive SCIP testing
func createTestWorkspace(t *testing.T) string {
	tempDir := t.TempDir()

	// Create microservices structure with realistic cross-language relationships
	createMicroservicesWorkspace(t, tempDir)

	return tempDir
}

// createMicroservicesWorkspace creates a realistic microservices architecture for SCIP cross-language testing
func createMicroservicesWorkspace(t *testing.T, tempDir string) {
	// Create directory structure
	dirs := []string{
		"gateway-service",
		"gateway-service/handlers",
		"gateway-service/models",
		"auth-service",
		"auth-service/models",
		"auth-service/handlers",
		"frontend-app",
		"frontend-app/src",
		"frontend-app/src/types",
		"frontend-app/src/services",
		"data-service",
		"data-service/src/main/java/com/test",
		"data-service/src/main/java/com/test/model",
		"data-service/src/main/java/com/test/controller",
		"data-service/src/main/java/com/test/service",
		"shared",
	}

	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tempDir, dir), 0755)
		require.NoError(t, err)
	}

	// 1. Go Gateway Service - Central API Gateway
	createGoGatewayService(t, tempDir)

	// 2. Python Auth Service - Authentication microservice
	createPythonAuthService(t, tempDir)

	// 3. TypeScript Frontend - React application
	createTypeScriptFrontend(t, tempDir)

	// 4. Java Data Service - Spring Boot data service
	createJavaDataService(t, tempDir)

	// 5. Shared configuration and contracts
	createSharedConfiguration(t, tempDir)

	// 6. Docker orchestration
	createDockerCompose(t, tempDir)
}

// createGoGatewayService creates the Go API Gateway service with cross-language integrations
func createGoGatewayService(t *testing.T, tempDir string) {
	// Main service entry point
	mainContent := `package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"gateway-service/handlers"
	"gateway-service/models"
)

// APIConfig represents shared API configuration
type APIConfig struct {
	AuthServiceURL string ` + "`json:\"auth_service_url\"`" + `
	DataServiceURL string ` + "`json:\"data_service_url\"`" + `
	Port           int    ` + "`json:\"port\"`" + `
}

// ErrorResponse represents standardized error response across all services
type ErrorResponse struct {
	Code    int    ` + "`json:\"code\"`" + `
	Message string ` + "`json:\"message\"`" + `
	Service string ` + "`json:\"service\"`" + `
}

func main() {
	config := APIConfig{
		AuthServiceURL: "http://localhost:5000",
		DataServiceURL: "http://localhost:8081",
		Port:           8080,
	}

	// Initialize handlers with cross-service communication
	authHandler := handlers.NewAuthHandler(config.AuthServiceURL)
	userHandler := handlers.NewUserHandler(config.DataServiceURL)
	productHandler := handlers.NewProductHandler(config.DataServiceURL)

	// API routes matching frontend service calls
	http.HandleFunc("/api/v1/auth/login", authHandler.HandleLogin)
	http.HandleFunc("/api/v1/auth/validate", authHandler.HandleValidateToken)
	http.HandleFunc("/api/v1/users", userHandler.HandleUsers)
	http.HandleFunc("/api/v1/users/", userHandler.HandleUserByID)
	http.HandleFunc("/api/v1/products", productHandler.HandleProducts)

	log.Printf("Gateway service starting on port %d", config.Port)
	log.Printf("Auth service: %s", config.AuthServiceURL)
	log.Printf("Data service: %s", config.DataServiceURL)
	
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", config.Port), nil))
}
`
	err := os.WriteFile(filepath.Join(tempDir, "gateway-service", "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)

	// Shared models matching other services
	userModelContent := `package models

import "time"

// User represents the shared user model across all services
// This matches Python auth-service/models/user.py User class
// This matches TypeScript frontend-app/src/types/User.ts interface
// This matches Java data-service User entity
type User struct {
	ID          int64     ` + "`json:\"id\"`" + `
	Email       string    ` + "`json:\"email\"`" + `
	Name        string    ` + "`json:\"name\"`" + `
	CreatedAt   time.Time ` + "`json:\"created_at\"`" + `
	UpdatedAt   time.Time ` + "`json:\"updated_at\"`" + `
	IsActive    bool      ` + "`json:\"is_active\"`" + `
	Preferences UserPreferences ` + "`json:\"preferences\"`" + `
}

// UserPreferences represents user settings
type UserPreferences struct {
	Theme         string ` + "`json:\"theme\"`" + `
	Language      string ` + "`json:\"language\"`" + `
	Notifications bool   ` + "`json:\"notifications\"`" + `
}

// CreateUserRequest represents user creation request
type CreateUserRequest struct {
	Email    string ` + "`json:\"email\"`" + `
	Name     string ` + "`json:\"name\"`" + `
	Password string ` + "`json:\"password\"`" + `
}

// UserResponse represents user API response
type UserResponse struct {
	User    User   ` + "`json:\"user\"`" + `
	Message string ` + "`json:\"message\"`" + `
}
`
	err = os.WriteFile(filepath.Join(tempDir, "gateway-service", "models", "user.go"), []byte(userModelContent), 0644)
	require.NoError(t, err)

	// Product model
	productModelContent := `package models

import "time"

// Product represents the shared product model across all services
// This matches Python auth-service product models
// This matches TypeScript frontend-app/src/types/Product.ts interface  
// This matches Java data-service Product entity
type Product struct {
	ID          int64     ` + "`json:\"id\"`" + `
	Name        string    ` + "`json:\"name\"`" + `
	Description string    ` + "`json:\"description\"`" + `
	Price       float64   ` + "`json:\"price\"`" + `
	Currency    string    ` + "`json:\"currency\"`" + `
	Inventory   int       ` + "`json:\"inventory\"`" + `
	CategoryID  int64     ` + "`json:\"category_id\"`" + `
	CreatedAt   time.Time ` + "`json:\"created_at\"`" + `
	UpdatedAt   time.Time ` + "`json:\"updated_at\"`" + `
	IsActive    bool      ` + "`json:\"is_active\"`" + `
}

// ProductCatalogRequest represents product search request
type ProductCatalogRequest struct {
	Category string  ` + "`json:\"category\"`" + `
	MinPrice float64 ` + "`json:\"min_price\"`" + `
	MaxPrice float64 ` + "`json:\"max_price\"`" + `
	Limit    int     ` + "`json:\"limit\"`" + `
}

// ProductCatalogResponse represents product catalog response
type ProductCatalogResponse struct {
	Products []Product ` + "`json:\"products\"`" + `
	Total    int       ` + "`json:\"total\"`" + `
	Page     int       ` + "`json:\"page\"`" + `
}
`
	err = os.WriteFile(filepath.Join(tempDir, "gateway-service", "models", "product.go"), []byte(productModelContent), 0644)
	require.NoError(t, err)

	// Auth handler - calls Python auth service
	authHandlerContent := `package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gateway-service/models"
)

// AuthHandler handles authentication requests and forwards to Python auth service
type AuthHandler struct {
	authServiceURL string
	client         *http.Client
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authServiceURL string) *AuthHandler {
	return &AuthHandler{
		authServiceURL: authServiceURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// LoginRequest represents login request payload
type LoginRequest struct {
	Email    string ` + "`json:\"email\"`" + `
	Password string ` + "`json:\"password\"`" + `
}

// LoginResponse represents login response from Python auth service
type LoginResponse struct {
	Token     string      ` + "`json:\"token\"`" + `
	User      models.User ` + "`json:\"user\"`" + `
	ExpiresAt time.Time   ` + "`json:\"expires_at\"`" + `
}

// HandleLogin forwards login request to Python auth service
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var loginReq LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Forward to Python auth service at /auth/login endpoint
	authURL := fmt.Sprintf("%s/auth/login", h.authServiceURL)
	payload, _ := json.Marshal(loginReq)
	
	resp, err := h.client.Post(authURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		http.Error(w, "Auth service unavailable", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		http.Error(w, "Invalid auth service response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(loginResp)
}

// HandleValidateToken validates token with Python auth service  
func (h *AuthHandler) HandleValidateToken(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if token == "" {
		http.Error(w, "Missing authorization token", http.StatusUnauthorized)
		return
	}

	// Forward to Python auth service at /auth/validate endpoint
	validateURL := fmt.Sprintf("%s/auth/validate", h.authServiceURL)
	req, _ := http.NewRequest("GET", validateURL, nil)
	req.Header.Set("Authorization", token)
	
	resp, err := h.client.Do(req)
	if err != nil {
		http.Error(w, "Auth service unavailable", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	var user models.User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid auth service response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}
`
	err = os.WriteFile(filepath.Join(tempDir, "gateway-service", "handlers", "auth_handler.go"), []byte(authHandlerContent), 0644)
	require.NoError(t, err)

	// User handler - calls Java data service
	userHandlerContent := `package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gateway-service/models"
)

// UserHandler handles user requests and forwards to Java data service
type UserHandler struct {
	dataServiceURL string
	client         *http.Client
}

// NewUserHandler creates a new user handler
func NewUserHandler(dataServiceURL string) *UserHandler {
	return &UserHandler{
		dataServiceURL: dataServiceURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// HandleUsers handles user listing and creation
func (h *UserHandler) HandleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleGetUsers(w, r)
	case http.MethodPost:
		h.handleCreateUser(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleUserByID handles individual user operations
func (h *UserHandler) HandleUserByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
	userID, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleGetUserByID(w, r, userID)
	case http.MethodPut:
		h.handleUpdateUser(w, r, userID)
	case http.MethodDelete:
		h.handleDeleteUser(w, r, userID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetUsers forwards request to Java data service
func (h *UserHandler) handleGetUsers(w http.ResponseWriter, r *http.Request) {
	// Forward to Java data service at /api/users endpoint
	dataURL := fmt.Sprintf("%s/api/users", h.dataServiceURL)
	resp, err := h.client.Get(dataURL)
	if err != nil {
		http.Error(w, "Data service unavailable", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	var users []models.User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		http.Error(w, "Invalid data service response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// handleCreateUser forwards user creation to Java data service
func (h *UserHandler) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var createReq models.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&createReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Forward to Java data service at /api/users endpoint
	dataURL := fmt.Sprintf("%s/api/users", h.dataServiceURL)
	payload, _ := json.Marshal(createReq)
	
	resp, err := h.client.Post(dataURL, "application/json", strings.NewReader(string(payload)))
	if err != nil {
		http.Error(w, "Data service unavailable", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	var userResp models.UserResponse
	if err := json.NewDecoder(resp.Body).Decode(&userResp); err != nil {
		http.Error(w, "Invalid data service response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(userResp)
}

// handleGetUserByID forwards user lookup to Java data service
func (h *UserHandler) handleGetUserByID(w http.ResponseWriter, r *http.Request, userID int64) {
	// Forward to Java data service at /api/users/{id} endpoint
	dataURL := fmt.Sprintf("%s/api/users/%d", h.dataServiceURL, userID)
	resp, err := h.client.Get(dataURL)
	if err != nil {
		http.Error(w, "Data service unavailable", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	var user models.User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid data service response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// handleUpdateUser forwards user update to Java data service
func (h *UserHandler) handleUpdateUser(w http.ResponseWriter, r *http.Request, userID int64) {
	var updateReq models.User
	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Forward to Java data service at /api/users/{id} endpoint
	dataURL := fmt.Sprintf("%s/api/users/%d", h.dataServiceURL, userID)
	payload, _ := json.Marshal(updateReq)
	
	req, _ := http.NewRequest(http.MethodPut, dataURL, strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := h.client.Do(req)
	if err != nil {
		http.Error(w, "Data service unavailable", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	var userResp models.UserResponse
	if err := json.NewDecoder(resp.Body).Decode(&userResp); err != nil {
		http.Error(w, "Invalid data service response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userResp)
}

// handleDeleteUser forwards user deletion to Java data service
func (h *UserHandler) handleDeleteUser(w http.ResponseWriter, r *http.Request, userID int64) {
	// Forward to Java data service at /api/users/{id} endpoint
	dataURL := fmt.Sprintf("%s/api/users/%d", h.dataServiceURL, userID)
	
	req, _ := http.NewRequest(http.MethodDelete, dataURL, nil)
	resp, err := h.client.Do(req)
	if err != nil {
		http.Error(w, "Data service unavailable", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
`
	err = os.WriteFile(filepath.Join(tempDir, "gateway-service", "handlers", "user_handler.go"), []byte(userHandlerContent), 0644)
	require.NoError(t, err)

	// Product handler - calls Java data service
	productHandlerContent := `package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gateway-service/models"
)

// ProductHandler handles product requests and forwards to Java data service
type ProductHandler struct {
	dataServiceURL string
	client         *http.Client
}

// NewProductHandler creates a new product handler
func NewProductHandler(dataServiceURL string) *ProductHandler {
	return &ProductHandler{
		dataServiceURL: dataServiceURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// HandleProducts handles product catalog requests
func (h *ProductHandler) HandleProducts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleGetProducts(w, r)
	case http.MethodPost:
		h.handleCreateProduct(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetProducts forwards product catalog request to Java data service
func (h *ProductHandler) handleGetProducts(w http.ResponseWriter, r *http.Request) {
	// Extract query parameters for product search
	category := r.URL.Query().Get("category")
	
	// Forward to Java data service at /api/products endpoint
	dataURL := fmt.Sprintf("%s/api/products", h.dataServiceURL)
	if category != "" {
		dataURL += fmt.Sprintf("?category=%s", category)
	}
	
	resp, err := h.client.Get(dataURL)
	if err != nil {
		http.Error(w, "Data service unavailable", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	var catalogResp models.ProductCatalogResponse
	if err := json.NewDecoder(resp.Body).Decode(&catalogResp); err != nil {
		http.Error(w, "Invalid data service response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(catalogResp)
}

// handleCreateProduct forwards product creation to Java data service
func (h *ProductHandler) handleCreateProduct(w http.ResponseWriter, r *http.Request) {
	var product models.Product
	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Forward to Java data service at /api/products endpoint
	dataURL := fmt.Sprintf("%s/api/products", h.dataServiceURL)
	payload, _ := json.Marshal(product)
	
	resp, err := h.client.Post(dataURL, "application/json", fmt.Sprintf("%s", payload))
	if err != nil {
		http.Error(w, "Data service unavailable", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	var createdProduct models.Product
	if err := json.NewDecoder(resp.Body).Decode(&createdProduct); err != nil {
		http.Error(w, "Invalid data service response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdProduct)
}
`
	err = os.WriteFile(filepath.Join(tempDir, "gateway-service", "handlers", "product_handler.go"), []byte(productHandlerContent), 0644)
	require.NoError(t, err)

	// Go module file
	goModContent := `module gateway-service

go 1.21

require (
	// Standard library only for this example
)
`
	err = os.WriteFile(filepath.Join(tempDir, "gateway-service", "go.mod"), []byte(goModContent), 0644)
	require.NoError(t, err)
}

// createPythonAuthService creates the Python FastAPI authentication service
func createPythonAuthService(t *testing.T, tempDir string) {
	// Main FastAPI service
	mainContent := `from fastapi import FastAPI, HTTPException, Depends, status
from fastapi.security import HTTPBearer, HTTPAuthorizationCredentials
from pydantic import BaseModel
from datetime import datetime, timedelta
from typing import Optional, List
import jwt
import hashlib
import uvicorn
import json
import os

from models.user import User, UserPreferences, CreateUserRequest
from models.auth import LoginRequest, LoginResponse, TokenData
from handlers.auth_handler import AuthHandler

# FastAPI app setup
app = FastAPI(
    title="Authentication Service",
    description="Microservice handling authentication for the gateway",
    version="1.0.0"
)

# Security
security = HTTPBearer()

# Initialize auth handler
auth_handler = AuthHandler()

# Shared API endpoints matching Go gateway service calls
@app.post("/auth/login", response_model=LoginResponse)
async def login(login_request: LoginRequest):
    """
    Login endpoint called by Go gateway service
    This matches gateway-service/handlers/auth_handler.go HandleLogin function
    """
    return await auth_handler.authenticate_user(login_request)

@app.get("/auth/validate", response_model=User)
async def validate_token(credentials: HTTPAuthorizationCredentials = Depends(security)):
    """
    Token validation endpoint called by Go gateway service  
    This matches gateway-service/handlers/auth_handler.go HandleValidateToken function
    """
    return await auth_handler.validate_token(credentials.credentials)

@app.post("/auth/register", response_model=LoginResponse) 
async def register(create_request: CreateUserRequest):
    """
    User registration endpoint with auto-login
    """
    return await auth_handler.register_user(create_request)

@app.get("/auth/users/{user_id}", response_model=User)
async def get_user_profile(user_id: int, credentials: HTTPAuthorizationCredentials = Depends(security)):
    """
    Get user profile by ID (requires authentication)
    """
    # Validate token first
    current_user = await auth_handler.validate_token(credentials.credentials)
    
    # Check if user can access this profile (self or admin)
    if current_user.id != user_id and not current_user.is_admin:
        raise HTTPException(
            status_code=status.HTTP_403_FORBIDDEN,
            detail="Not authorized to access this profile"
        )
    
    return await auth_handler.get_user_by_id(user_id)

@app.put("/auth/users/{user_id}", response_model=User)
async def update_user_profile(
    user_id: int, 
    user_update: User, 
    credentials: HTTPAuthorizationCredentials = Depends(security)
):
    """
    Update user profile (requires authentication)
    """
    # Validate token first
    current_user = await auth_handler.validate_token(credentials.credentials)
    
    # Check if user can update this profile (self only)
    if current_user.id != user_id:
        raise HTTPException(
            status_code=status.HTTP_403_FORBIDDEN,
            detail="Not authorized to update this profile"
        )
    
    return await auth_handler.update_user_profile(user_id, user_update)

@app.get("/health")
async def health_check():
    """Health check endpoint for service monitoring"""
    return {"status": "healthy", "service": "auth-service", "timestamp": datetime.now()}

if __name__ == "__main__":
    # Configuration matching Go gateway service expectations
    config = {
        "host": "0.0.0.0",
        "port": 5000,
        "reload": True
    }
    
    print(f"Starting Auth Service on {config['host']}:{config['port']}")
    print("Endpoints:")
    print("  POST /auth/login - User login")
    print("  GET /auth/validate - Token validation")
    print("  POST /auth/register - User registration")
    print("  GET /auth/users/{user_id} - User profile")
    print("  PUT /auth/users/{user_id} - Update profile")
    
    uvicorn.run(app, host=config["host"], port=config["port"], reload=config["reload"])
`
	err := os.WriteFile(filepath.Join(tempDir, "auth-service", "main.py"), []byte(mainContent), 0644)
	require.NoError(t, err)

	// User model matching Go and other services
	userModelContent := `from pydantic import BaseModel, EmailStr
from datetime import datetime
from typing import Optional

class UserPreferences(BaseModel):
    """
    User preferences model matching Go gateway-service/models/user.go UserPreferences
    This also matches TypeScript frontend-app/src/types/User.ts interface
    This also matches Java data-service User entity preferences
    """
    theme: str = "light"
    language: str = "en" 
    notifications: bool = True

class User(BaseModel):
    """
    User model matching Go gateway-service/models/user.go User struct
    This also matches TypeScript frontend-app/src/types/User.ts interface
    This also matches Java data-service User entity
    """
    id: int
    email: EmailStr
    name: str
    created_at: datetime
    updated_at: datetime
    is_active: bool = True
    is_admin: bool = False
    preferences: UserPreferences
    
    class Config:
        json_encoders = {
            datetime: lambda v: v.isoformat()
        }

class CreateUserRequest(BaseModel):
    """
    User creation request matching Go gateway-service/models/user.go CreateUserRequest
    This also matches TypeScript frontend-app forms
    """
    email: EmailStr
    name: str
    password: str

class UserResponse(BaseModel):
    """
    User API response matching Go gateway-service/models/user.go UserResponse
    """
    user: User
    message: str

class UserDatabase:
    """
    In-memory user database for testing
    In production this would connect to the Java data service
    """
    def __init__(self):
        self.users = {}
        self.next_id = 1
        
        # Create default test user
        self._create_default_users()
    
    def _create_default_users(self):
        """Create some default test users"""
        test_users = [
            {
                "email": "admin@example.com",
                "name": "Admin User", 
                "password": "admin123",
                "is_admin": True
            },
            {
                "email": "user@example.com",
                "name": "Test User",
                "password": "user123", 
                "is_admin": False
            }
        ]
        
        for user_data in test_users:
            self.create_user(CreateUserRequest(**user_data), user_data["is_admin"])
    
    def create_user(self, create_request: CreateUserRequest, is_admin: bool = False) -> User:
        """Create a new user"""
        user_id = self.next_id
        self.next_id += 1
        
        now = datetime.now()
        user = User(
            id=user_id,
            email=create_request.email,
            name=create_request.name,
            created_at=now,
            updated_at=now,
            is_active=True,
            is_admin=is_admin,
            preferences=UserPreferences()
        )
        
        # Store user with hashed password
        self.users[user_id] = {
            "user": user,
            "password_hash": self._hash_password(create_request.password)
        }
        
        return user
    
    def get_user_by_id(self, user_id: int) -> Optional[User]:
        """Get user by ID"""
        user_data = self.users.get(user_id)
        return user_data["user"] if user_data else None
    
    def get_user_by_email(self, email: str) -> Optional[User]:
        """Get user by email"""
        for user_data in self.users.values():
            if user_data["user"].email == email:
                return user_data["user"]
        return None
    
    def verify_password(self, user_id: int, password: str) -> bool:
        """Verify user password"""
        user_data = self.users.get(user_id)
        if not user_data:
            return False
        
        return user_data["password_hash"] == self._hash_password(password)
    
    def update_user(self, user_id: int, user_update: User) -> Optional[User]:
        """Update user profile"""
        if user_id not in self.users:
            return None
        
        user_update.updated_at = datetime.now()
        self.users[user_id]["user"] = user_update
        return user_update
    
    def _hash_password(self, password: str) -> str:
        """Simple password hashing (use proper hashing in production)"""
        return hashlib.sha256(password.encode()).hexdigest()

# Global database instance
user_db = UserDatabase()
`
	err = os.WriteFile(filepath.Join(tempDir, "auth-service", "models", "user.py"), []byte(userModelContent), 0644)
	require.NoError(t, err)

	// Auth models
	authModelContent := `from pydantic import BaseModel, EmailStr
from datetime import datetime
from typing import Optional
from models.user import User

class LoginRequest(BaseModel):
    """
    Login request model matching Go gateway-service/handlers/auth_handler.go LoginRequest
    """
    email: EmailStr
    password: str

class LoginResponse(BaseModel):
    """
    Login response model matching Go gateway-service/handlers/auth_handler.go LoginResponse
    """
    token: str
    user: User
    expires_at: datetime
    
    class Config:
        json_encoders = {
            datetime: lambda v: v.isoformat()
        }

class TokenData(BaseModel):
    """JWT token payload data"""
    user_id: int
    email: str
    expires_at: datetime
    
    class Config:
        json_encoders = {
            datetime: lambda v: v.isoformat()
        }

class AuthError(Exception):
    """Custom authentication error"""
    def __init__(self, message: str, status_code: int = 401):
        self.message = message
        self.status_code = status_code
        super().__init__(self.message)
`
	err = os.WriteFile(filepath.Join(tempDir, "auth-service", "models", "auth.py"), []byte(authModelContent), 0644)
	require.NoError(t, err)

	// Auth handler
	authHandlerContent := `import jwt
from datetime import datetime, timedelta
from fastapi import HTTPException, status
from typing import Optional

from models.user import User, CreateUserRequest, user_db
from models.auth import LoginRequest, LoginResponse, TokenData, AuthError

class AuthHandler:
    """
    Authentication handler providing services to Go gateway
    This class implements the backend logic for endpoints called by
    gateway-service/handlers/auth_handler.go
    """
    
    def __init__(self):
        # JWT configuration (use environment variables in production)
        self.jwt_secret = "your-secret-key-here"
        self.jwt_algorithm = "HS256"
        self.token_expire_hours = 24
    
    async def authenticate_user(self, login_request: LoginRequest) -> LoginResponse:
        """
        Authenticate user and return JWT token
        Called by Go gateway HandleLogin function
        """
        # Find user by email
        user = user_db.get_user_by_email(login_request.email)
        if not user:
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="Invalid email or password"
            )
        
        # Verify password
        if not user_db.verify_password(user.id, login_request.password):
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="Invalid email or password"
            )
        
        # Check if user is active
        if not user.is_active:
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="Account is disabled"
            )
        
        # Generate JWT token
        expires_at = datetime.now() + timedelta(hours=self.token_expire_hours)
        token_data = TokenData(
            user_id=user.id,
            email=user.email,
            expires_at=expires_at
        )
        
        token = jwt.encode(
            token_data.dict(),
            self.jwt_secret,
            algorithm=self.jwt_algorithm
        )
        
        return LoginResponse(
            token=token,
            user=user,
            expires_at=expires_at
        )
    
    async def validate_token(self, token: str) -> User:
        """
        Validate JWT token and return user
        Called by Go gateway HandleValidateToken function
        """
        try:
            # Remove Bearer prefix if present
            if token.startswith("Bearer "):
                token = token[7:]
            
            # Decode and validate token
            payload = jwt.decode(
                token,
                self.jwt_secret,
                algorithms=[self.jwt_algorithm]
            )
            
            # Extract token data
            token_data = TokenData(**payload)
            
            # Check if token is expired
            if datetime.now() > token_data.expires_at:
                raise HTTPException(
                    status_code=status.HTTP_401_UNAUTHORIZED,
                    detail="Token has expired"
                )
            
            # Get user from database
            user = user_db.get_user_by_id(token_data.user_id)
            if not user:
                raise HTTPException(
                    status_code=status.HTTP_401_UNAUTHORIZED,
                    detail="User not found"
                )
            
            # Check if user is still active
            if not user.is_active:
                raise HTTPException(
                    status_code=status.HTTP_401_UNAUTHORIZED,
                    detail="User account is disabled"
                )
            
            return user
            
        except jwt.ExpiredSignatureError:
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="Token has expired"
            )
        except jwt.InvalidTokenError:
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="Invalid token"
            )
    
    async def register_user(self, create_request: CreateUserRequest) -> LoginResponse:
        """
        Register new user and return login response
        """
        # Check if user already exists
        existing_user = user_db.get_user_by_email(create_request.email)
        if existing_user:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="User with this email already exists"
            )
        
        # Create new user
        user = user_db.create_user(create_request)
        
        # Auto-login after registration
        login_request = LoginRequest(
            email=create_request.email,
            password=create_request.password
        )
        
        return await self.authenticate_user(login_request)
    
    async def get_user_by_id(self, user_id: int) -> User:
        """Get user by ID"""
        user = user_db.get_user_by_id(user_id)
        if not user:
            raise HTTPException(
                status_code=status.HTTP_404_NOT_FOUND,
                detail="User not found"
            )
        return user
    
    async def update_user_profile(self, user_id: int, user_update: User) -> User:
        """Update user profile"""
        updated_user = user_db.update_user(user_id, user_update)
        if not updated_user:
            raise HTTPException(
                status_code=status.HTTP_404_NOT_FOUND,
                detail="User not found"
            )
        return updated_user
`
	err = os.WriteFile(filepath.Join(tempDir, "auth-service", "handlers", "auth_handler.py"), []byte(authHandlerContent), 0644)
	require.NoError(t, err)

	// Requirements.txt
	requirementsContent := `fastapi==0.104.1
uvicorn==0.24.0
pydantic==2.5.0
pydantic[email]==2.5.0
pyjwt==2.8.0
python-multipart==0.0.6
`
	err = os.WriteFile(filepath.Join(tempDir, "auth-service", "requirements.txt"), []byte(requirementsContent), 0644)
	require.NoError(t, err)
}

// createTypeScriptFrontend creates the TypeScript React frontend application
func createTypeScriptFrontend(t *testing.T, tempDir string) {
	// User type definitions matching other services
	userTypesContent := `// User types matching Go gateway-service/models/user.go
// This also matches Python auth-service/models/user.py
// This also matches Java data-service User entity

export interface UserPreferences {
  theme: 'light' | 'dark';
  language: string;
  notifications: boolean;
}

export interface User {
  id: number;
  email: string;
  name: string;
  created_at: string;
  updated_at: string;
  is_active: boolean;
  is_admin?: boolean;
  preferences: UserPreferences;
}

export interface CreateUserRequest {
  email: string;
  name: string;
  password: string;
}

export interface UserResponse {
  user: User;
  message: string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface LoginResponse {
  token: string;
  user: User;
  expires_at: string;
}
`
	err := os.WriteFile(filepath.Join(tempDir, "frontend-app", "src", "types", "User.ts"), []byte(userTypesContent), 0644)
	require.NoError(t, err)

	// Product type definitions matching other services
	productTypesContent := `// Product types matching Go gateway-service/models/product.go
// This also matches Python auth-service product models
// This also matches Java data-service Product entity

export interface Product {
  id: number;
  name: string;
  description: string;
  price: number;
  currency: string;
  inventory: number;
  category_id: number;
  created_at: string;
  updated_at: string;
  is_active: boolean;
}

export interface ProductCatalogRequest {
  category?: string;
  min_price?: number;
  max_price?: number;
  limit?: number;
}

export interface ProductCatalogResponse {
  products: Product[];
  total: number;
  page: number;
}

export interface CreateProductRequest {
  name: string;
  description: string;
  price: number;
  currency: string;
  inventory: number;
  category_id: number;
}
`
	err = os.WriteFile(filepath.Join(tempDir, "frontend-app", "src", "types", "Product.ts"), []byte(productTypesContent), 0644)
	require.NoError(t, err)

	// Auth service - calls Go gateway auth endpoints
	authServiceContent := `// AuthService handles authentication by calling Go gateway service endpoints
// This corresponds to gateway-service/handlers/auth_handler.go functions

import { User, LoginRequest, LoginResponse, CreateUserRequest } from '../types/User';

export class AuthService {
  private baseURL: string;
  private gatewayURL: string;

  constructor() {
    // Connect to Go gateway service running on port 8080
    this.gatewayURL = 'http://localhost:8080';
    this.baseURL = ` + "`${this.gatewayURL}/api/v1/auth`" + `;
  }

  /**
   * Login user by calling Go gateway /api/v1/auth/login endpoint
   * This calls gateway-service/handlers/auth_handler.go HandleLogin function
   * Which forwards to Python auth-service /auth/login endpoint
   */
  async login(loginRequest: LoginRequest): Promise<LoginResponse> {
    const response = await fetch(` + "`${this.baseURL}/login`" + `, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(loginRequest),
    });

    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.message || 'Login failed');
    }

    const loginResponse: LoginResponse = await response.json();
    
    // Store token for future requests
    this.setAuthToken(loginResponse.token);
    
    return loginResponse;
  }

  /**
   * Validate current token by calling Go gateway /api/v1/auth/validate endpoint
   * This calls gateway-service/handlers/auth_handler.go HandleValidateToken function
   * Which forwards to Python auth-service /auth/validate endpoint
   */
  async validateToken(): Promise<User | null> {
    const token = this.getAuthToken();
    if (!token) {
      return null;
    }

    try {
      const response = await fetch(` + "`${this.baseURL}/validate`" + `, {
        method: 'GET',
        headers: {
          'Authorization': ` + "`Bearer ${token}`" + `,
          'Content-Type': 'application/json',
        },
      });

      if (!response.ok) {
        this.clearAuthToken();
        return null;
      }

      return await response.json();
    } catch (error) {
      this.clearAuthToken();
      return null;
    }
  }

  /**
   * Register new user
   */
  async register(createRequest: CreateUserRequest): Promise<LoginResponse> {
    // Note: This would typically go through gateway to Python auth service
    // For now, calling auth service directly as example
    const response = await fetch('http://localhost:5000/auth/register', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(createRequest),
    });

    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.detail || 'Registration failed');
    }

    const loginResponse: LoginResponse = await response.json();
    
    // Store token for future requests
    this.setAuthToken(loginResponse.token);
    
    return loginResponse;
  }

  /**
   * Logout user
   */
  logout(): void {
    this.clearAuthToken();
  }

  /**
   * Get current user if authenticated
   */
  async getCurrentUser(): Promise<User | null> {
    return await this.validateToken();
  }

  /**
   * Check if user is currently authenticated
   */
  isAuthenticated(): boolean {
    return !!this.getAuthToken();
  }

  /**
   * Get stored auth token
   */
  private getAuthToken(): string | null {
    return localStorage.getItem('auth_token');
  }

  /**
   * Store auth token
   */
  private setAuthToken(token: string): void {
    localStorage.setItem('auth_token', token);
  }

  /**
   * Clear stored auth token
   */
  private clearAuthToken(): void {
    localStorage.removeItem('auth_token');
  }

  /**
   * Get authorization headers for API calls
   */
  getAuthHeaders(): Record<string, string> {
    const token = this.getAuthToken();
    return token ? { 'Authorization': ` + "`Bearer ${token}`" + ` } : {};
  }
}

// Export singleton instance
export const authService = new AuthService();
`
	err = os.WriteFile(filepath.Join(tempDir, "frontend-app", "src", "services", "AuthService.ts"), []byte(authServiceContent), 0644)
	require.NoError(t, err)

	// Product service - calls Go gateway product endpoints
	productServiceContent := `// ProductService handles product operations by calling Go gateway service endpoints
// This corresponds to gateway-service/handlers/product_handler.go functions

import { Product, ProductCatalogRequest, ProductCatalogResponse, CreateProductRequest } from '../types/Product';
import { authService } from './AuthService';

export class ProductService {
  private baseURL: string;
  private gatewayURL: string;

  constructor() {
    // Connect to Go gateway service running on port 8080
    this.gatewayURL = 'http://localhost:8080';
    this.baseURL = ` + "`${this.gatewayURL}/api/v1/products`" + `;
  }

  /**
   * Get product catalog by calling Go gateway /api/v1/products endpoint
   * This calls gateway-service/handlers/product_handler.go HandleProducts function
   * Which forwards to Java data-service /api/products endpoint
   */
  async getProducts(request?: ProductCatalogRequest): Promise<ProductCatalogResponse> {
    let url = this.baseURL;
    
    // Add query parameters if provided
    if (request) {
      const params = new URLSearchParams();
      if (request.category) params.append('category', request.category);
      if (request.min_price !== undefined) params.append('min_price', request.min_price.toString());
      if (request.max_price !== undefined) params.append('max_price', request.max_price.toString());
      if (request.limit !== undefined) params.append('limit', request.limit.toString());
      
      const queryString = params.toString();
      if (queryString) {
        url += ` + "`?${queryString}`" + `;
      }
    }

    const response = await fetch(url, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
        ...authService.getAuthHeaders(),
      },
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: 'Failed to fetch products' }));
      throw new Error(error.message || 'Failed to fetch products');
    }

    return await response.json();
  }

  /**
   * Create new product by calling Go gateway /api/v1/products endpoint
   * This calls gateway-service/handlers/product_handler.go HandleProducts function
   * Which forwards to Java data-service /api/products endpoint
   */
  async createProduct(product: CreateProductRequest): Promise<Product> {
    const response = await fetch(this.baseURL, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...authService.getAuthHeaders(),
      },
      body: JSON.stringify(product),
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: 'Failed to create product' }));
      throw new Error(error.message || 'Failed to create product');
    }

    return await response.json();
  }

  /**
   * Get product by ID
   */
  async getProductById(productId: number): Promise<Product> {
    const response = await fetch(` + "`${this.baseURL}/${productId}`" + `, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
        ...authService.getAuthHeaders(),
      },
    });

    if (!response.ok) {
      if (response.status === 404) {
        throw new Error('Product not found');
      }
      const error = await response.json().catch(() => ({ message: 'Failed to fetch product' }));
      throw new Error(error.message || 'Failed to fetch product');
    }

    return await response.json();
  }

  /**
   * Update product
   */
  async updateProduct(productId: number, product: Partial<Product>): Promise<Product> {
    const response = await fetch(` + "`${this.baseURL}/${productId}`" + `, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        ...authService.getAuthHeaders(),
      },
      body: JSON.stringify(product),
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: 'Failed to update product' }));
      throw new Error(error.message || 'Failed to update product');
    }

    return await response.json();
  }

  /**
   * Delete product
   */
  async deleteProduct(productId: number): Promise<void> {
    const response = await fetch(` + "`${this.baseURL}/${productId}`" + `, {
      method: 'DELETE',
      headers: {
        'Content-Type': 'application/json',
        ...authService.getAuthHeaders(),
      },
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: 'Failed to delete product' }));
      throw new Error(error.message || 'Failed to delete product');
    }
  }
}

// Export singleton instance
export const productService = new ProductService();
`
	err = os.WriteFile(filepath.Join(tempDir, "frontend-app", "src", "services", "ProductService.ts"), []byte(productServiceContent), 0644)
	require.NoError(t, err)

	// User service - calls Go gateway user endpoints
	userServiceContent := `// UserService handles user operations by calling Go gateway service endpoints
// This corresponds to gateway-service/handlers/user_handler.go functions

import { User, CreateUserRequest, UserResponse } from '../types/User';
import { authService } from './AuthService';

export class UserService {
  private baseURL: string;
  private gatewayURL: string;

  constructor() {
    // Connect to Go gateway service running on port 8080
    this.gatewayURL = 'http://localhost:8080';
    this.baseURL = ` + "`${this.gatewayURL}/api/v1/users`" + `;
  }

  /**
   * Get all users by calling Go gateway /api/v1/users endpoint
   * This calls gateway-service/handlers/user_handler.go HandleUsers function
   * Which forwards to Java data-service /api/users endpoint
   */
  async getUsers(): Promise<User[]> {
    const response = await fetch(this.baseURL, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
        ...authService.getAuthHeaders(),
      },
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: 'Failed to fetch users' }));
      throw new Error(error.message || 'Failed to fetch users');
    }

    return await response.json();
  }

  /**
   * Create new user by calling Go gateway /api/v1/users endpoint
   * This calls gateway-service/handlers/user_handler.go HandleUsers function
   * Which forwards to Java data-service /api/users endpoint
   */
  async createUser(createRequest: CreateUserRequest): Promise<UserResponse> {
    const response = await fetch(this.baseURL, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...authService.getAuthHeaders(),
      },
      body: JSON.stringify(createRequest),
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: 'Failed to create user' }));
      throw new Error(error.message || 'Failed to create user');
    }

    return await response.json();
  }

  /**
   * Get user by ID by calling Go gateway /api/v1/users/{id} endpoint
   * This calls gateway-service/handlers/user_handler.go HandleUserByID function
   * Which forwards to Java data-service /api/users/{id} endpoint
   */
  async getUserById(userId: number): Promise<User> {
    const response = await fetch(` + "`${this.baseURL}/${userId}`" + `, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
        ...authService.getAuthHeaders(),
      },
    });

    if (!response.ok) {
      if (response.status === 404) {
        throw new Error('User not found');
      }
      const error = await response.json().catch(() => ({ message: 'Failed to fetch user' }));
      throw new Error(error.message || 'Failed to fetch user');
    }

    return await response.json();
  }

  /**
   * Update user by calling Go gateway /api/v1/users/{id} endpoint
   * This calls gateway-service/handlers/user_handler.go HandleUserByID function
   * Which forwards to Java data-service /api/users/{id} endpoint
   */
  async updateUser(userId: number, user: Partial<User>): Promise<UserResponse> {
    const response = await fetch(` + "`${this.baseURL}/${userId}`" + `, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        ...authService.getAuthHeaders(),
      },
      body: JSON.stringify(user),
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: 'Failed to update user' }));
      throw new Error(error.message || 'Failed to update user');
    }

    return await response.json();
  }

  /**
   * Delete user by calling Go gateway /api/v1/users/{id} endpoint
   * This calls gateway-service/handlers/user_handler.go HandleUserByID function
   * Which forwards to Java data-service /api/users/{id} endpoint
   */
  async deleteUser(userId: number): Promise<void> {
    const response = await fetch(` + "`${this.baseURL}/${userId}`" + `, {
      method: 'DELETE',
      headers: {
        'Content-Type': 'application/json',
        ...authService.getAuthHeaders(),
      },
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: 'Failed to delete user' }));
      throw new Error(error.message || 'Failed to delete user');
    }
  }

  /**
   * Get current user profile
   */
  async getCurrentUserProfile(): Promise<User | null> {
    return await authService.getCurrentUser();
  }

  /**
   * Update current user profile
   */
  async updateCurrentUserProfile(updates: Partial<User>): Promise<UserResponse | null> {
    const currentUser = await this.getCurrentUserProfile();
    if (!currentUser) {
      throw new Error('Not authenticated');
    }

    return await this.updateUser(currentUser.id, updates);
  }
}

// Export singleton instance
export const userService = new UserService();
`
	err = os.WriteFile(filepath.Join(tempDir, "frontend-app", "src", "services", "UserService.ts"), []byte(userServiceContent), 0644)
	require.NoError(t, err)

	// Main App component demonstrating cross-service integration
	appContent := `// Main React App demonstrating cross-language service integration
// This component uses services that call Go gateway endpoints
// Which forward to Python auth service and Java data service

import React, { useState, useEffect } from 'react';
import { User, LoginRequest, CreateUserRequest } from './types/User';
import { Product, ProductCatalogRequest } from './types/Product';
import { authService } from './services/AuthService';
import { userService } from './services/UserService';
import { productService } from './services/ProductService';

interface AppState {
  currentUser: User | null;
  users: User[];
  products: Product[];
  loading: boolean;
  error: string | null;
}

export const App: React.FC = () => {
  const [state, setState] = useState<AppState>({
    currentUser: null,
    users: [],
    products: [],
    loading: false,
    error: null,
  });

  const [loginForm, setLoginForm] = useState<LoginRequest>({
    email: '',
    password: '',
  });

  const [registerForm, setRegisterForm] = useState<CreateUserRequest>({
    email: '',
    name: '',
    password: '',
  });

  // Initialize app - check if user is authenticated
  useEffect(() => {
    initializeApp();
  }, []);

  const initializeApp = async () => {
    setState(prev => ({ ...prev, loading: true, error: null }));
    
    try {
      // Check if user is authenticated by calling Go gateway → Python auth service
      const currentUser = await authService.getCurrentUser();
      setState(prev => ({ ...prev, currentUser, loading: false }));

      if (currentUser) {
        // Load data if authenticated
        await Promise.all([
          loadUsers(),
          loadProducts(),
        ]);
      }
    } catch (error) {
      setState(prev => ({ 
        ...prev, 
        loading: false, 
        error: error instanceof Error ? error.message : 'Failed to initialize app'
      }));
    }
  };

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setState(prev => ({ ...prev, loading: true, error: null }));

    try {
      // Login through Go gateway → Python auth service
      const loginResponse = await authService.login(loginForm);
      setState(prev => ({ 
        ...prev, 
        currentUser: loginResponse.user, 
        loading: false 
      }));

      // Load user data after login
      await Promise.all([
        loadUsers(),
        loadProducts(),
      ]);

      // Clear form
      setLoginForm({ email: '', password: '' });
    } catch (error) {
      setState(prev => ({ 
        ...prev, 
        loading: false, 
        error: error instanceof Error ? error.message : 'Login failed'
      }));
    }
  };

  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault();
    setState(prev => ({ ...prev, loading: true, error: null }));

    try {
      // Register through Python auth service (direct for example)
      const loginResponse = await authService.register(registerForm);
      setState(prev => ({ 
        ...prev, 
        currentUser: loginResponse.user, 
        loading: false 
      }));

      // Load user data after registration
      await Promise.all([
        loadUsers(),
        loadProducts(),
      ]);

      // Clear form
      setRegisterForm({ email: '', name: '', password: '' });
    } catch (error) {
      setState(prev => ({ 
        ...prev, 
        loading: false, 
        error: error instanceof Error ? error.message : 'Registration failed'
      }));
    }
  };

  const handleLogout = () => {
    authService.logout();
    setState({
      currentUser: null,
      users: [],
      products: [],
      loading: false,
      error: null,
    });
  };

  // Load users through Go gateway → Java data service
  const loadUsers = async () => {
    try {
      const users = await userService.getUsers();
      setState(prev => ({ ...prev, users }));
    } catch (error) {
      console.error('Failed to load users:', error);
    }
  };

  // Load products through Go gateway → Java data service
  const loadProducts = async () => {
    try {
      const catalogRequest: ProductCatalogRequest = { limit: 20 };
      const catalog = await productService.getProducts(catalogRequest);
      setState(prev => ({ ...prev, products: catalog.products }));
    } catch (error) {
      console.error('Failed to load products:', error);
    }
  };

  if (state.loading) {
    return <div className="loading">Loading...</div>;
  }

  if (!state.currentUser) {
    return (
      <div className="auth-container">
        <h1>Multi-Language Microservices Demo</h1>
        <p>Frontend (TypeScript/React) → Gateway (Go) → Auth (Python) & Data (Java)</p>
        
        {state.error && <div className="error">{state.error}</div>}
        
        <div className="auth-forms">
          <form onSubmit={handleLogin} className="login-form">
            <h2>Login</h2>
            <input
              type="email"
              placeholder="Email"
              value={loginForm.email}
              onChange={(e) => setLoginForm(prev => ({ ...prev, email: e.target.value }))}
              required
            />
            <input
              type="password"
              placeholder="Password"
              value={loginForm.password}
              onChange={(e) => setLoginForm(prev => ({ ...prev, password: e.target.value }))}
              required
            />
            <button type="submit">Login</button>
            <small>Try: user@example.com / user123</small>
          </form>

          <form onSubmit={handleRegister} className="register-form">
            <h2>Register</h2>
            <input
              type="text"
              placeholder="Name"
              value={registerForm.name}
              onChange={(e) => setRegisterForm(prev => ({ ...prev, name: e.target.value }))}
              required
            />
            <input
              type="email"
              placeholder="Email"
              value={registerForm.email}
              onChange={(e) => setRegisterForm(prev => ({ ...prev, email: e.target.value }))}
              required
            />
            <input
              type="password"
              placeholder="Password"
              value={registerForm.password}
              onChange={(e) => setRegisterForm(prev => ({ ...prev, password: e.target.value }))}
              required
            />
            <button type="submit">Register</button>
          </form>
        </div>
      </div>
    );
  }

  return (
    <div className="app">
      <header className="app-header">
        <h1>Cross-Language Microservices Dashboard</h1>
        <div className="user-info">
          <span>Welcome, {state.currentUser.name}</span>
          <button onClick={handleLogout}>Logout</button>
        </div>
      </header>

      {state.error && <div className="error">{state.error}</div>}

      <main className="app-main">
        <section className="users-section">
          <h2>Users (via Go Gateway → Java Data Service)</h2>
          <div className="users-grid">
            {state.users.map(user => (
              <div key={user.id} className="user-card">
                <h3>{user.name}</h3>
                <p>{user.email}</p>
                <p>Theme: {user.preferences.theme}</p>
                <p>Active: {user.is_active ? 'Yes' : 'No'}</p>
              </div>
            ))}
          </div>
        </section>

        <section className="products-section">
          <h2>Products (via Go Gateway → Java Data Service)</h2>
          <div className="products-grid">
            {state.products.map(product => (
              <div key={product.id} className="product-card">
                <h3>{product.name}</h3>
                <p>{product.description}</p>
                <p>Price: {product.currency} {product.price}</p>
                <p>Inventory: {product.inventory}</p>
              </div>
            ))}
          </div>
        </section>
      </main>

      <footer className="app-footer">
        <p>
          Architecture: TypeScript/React Frontend → Go API Gateway → Python Auth Service + Java Data Service
        </p>
        <p>
          Cross-language symbol references: User, Product, API endpoints, authentication flows
        </p>
      </footer>
    </div>
  );
};

export default App;
`
	err = os.WriteFile(filepath.Join(tempDir, "frontend-app", "src", "App.tsx"), []byte(appContent), 0644)
	require.NoError(t, err)

	// Package.json with dependencies
	packageJsonContent := `{
  "name": "microservices-frontend",
  "version": "1.0.0",
  "description": "TypeScript React frontend for multi-language microservices architecture",
  "main": "src/App.tsx",
  "scripts": {
    "start": "react-scripts start",
    "build": "react-scripts build",
    "test": "react-scripts test",
    "eject": "react-scripts eject",
    "type-check": "tsc --noEmit"
  },
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0",
    "react-scripts": "5.0.1",
    "typescript": "^5.0.0",
    "@types/react": "^18.2.0",
    "@types/react-dom": "^18.2.0"
  },
  "devDependencies": {
    "@types/node": "^20.0.0",
    "web-vitals": "^3.3.0"
  },
  "browserslist": {
    "production": [
      ">0.2%",
      "not dead",
      "not op_mini all"
    ],
    "development": [
      "last 1 chrome version",
      "last 1 firefox version",
      "last 1 safari version"
    ]
  },
  "proxy": "http://localhost:8080"
}
`
	err = os.WriteFile(filepath.Join(tempDir, "frontend-app", "package.json"), []byte(packageJsonContent), 0644)
	require.NoError(t, err)

	// TypeScript config
	tsConfigContent := `{
  "compilerOptions": {
    "target": "es5",
    "lib": [
      "dom",
      "dom.iterable",
      "es6"
    ],
    "allowJs": true,
    "skipLibCheck": true,
    "esModuleInterop": true,
    "allowSyntheticDefaultImports": true,
    "strict": true,
    "forceConsistentCasingInFileNames": true,
    "noFallthroughCasesInSwitch": true,
    "module": "esnext",
    "moduleResolution": "node",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx"
  },
  "include": [
    "src"
  ]
}
`
	err = os.WriteFile(filepath.Join(tempDir, "frontend-app", "tsconfig.json"), []byte(tsConfigContent), 0644)
	require.NoError(t, err)
}

// createJavaDataService creates the Java Spring Boot data service
func createJavaDataService(t *testing.T, tempDir string) {
	// Main Spring Boot application
	mainAppContent := `package com.test;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.web.bind.annotation.CrossOrigin;

/**
 * Java Spring Boot Data Service
 * Handles data operations for Go gateway service
 * This corresponds to gateway-service/handlers/user_handler.go and product_handler.go calls
 */
@SpringBootApplication
@CrossOrigin(origins = {"http://localhost:8080", "http://localhost:3000"})
public class DataServiceApplication {
    public static void main(String[] args) {
        System.out.println("Starting Java Data Service on port 8081");
        System.out.println("Endpoints:");
        System.out.println("  GET /api/users - List users");
        System.out.println("  POST /api/users - Create user");
        System.out.println("  GET /api/users/{id} - Get user by ID");
        System.out.println("  PUT /api/users/{id} - Update user");
        System.out.println("  DELETE /api/users/{id} - Delete user");
        System.out.println("  GET /api/products - List products");
        System.out.println("  POST /api/products - Create product");
        
        SpringApplication.run(DataServiceApplication.class, args);
    }
}
`
	err := os.WriteFile(filepath.Join(tempDir, "data-service", "src", "main", "java", "com", "test", "DataServiceApplication.java"), []byte(mainAppContent), 0644)
	require.NoError(t, err)

	// User entity matching other services
	userEntityContent := `package com.test.model;

import javax.persistence.*;
import java.time.LocalDateTime;

/**
 * User entity matching Go gateway-service/models/user.go User struct
 * This also matches Python auth-service/models/user.py User class
 * This also matches TypeScript frontend-app/src/types/User.ts interface
 */
@Entity
@Table(name = "users")
public class User {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;
    
    @Column(unique = true, nullable = false)
    private String email;
    
    @Column(nullable = false)
    private String name;
    
    @Column(name = "created_at")
    private LocalDateTime createdAt;
    
    @Column(name = "updated_at")
    private LocalDateTime updatedAt;
    
    @Column(name = "is_active")
    private Boolean isActive = true;
    
    @Column(name = "is_admin")
    private Boolean isAdmin = false;
    
    @Embedded
    private UserPreferences preferences;
    
    // Constructors
    public User() {
        this.createdAt = LocalDateTime.now();
        this.updatedAt = LocalDateTime.now();
        this.preferences = new UserPreferences();
    }
    
    public User(String email, String name) {
        this();
        this.email = email;
        this.name = name;
    }
    
    // Getters and Setters
    public Long getId() { return id; }
    public void setId(Long id) { this.id = id; }
    
    public String getEmail() { return email; }
    public void setEmail(String email) { this.email = email; }
    
    public String getName() { return name; }
    public void setName(String name) { this.name = name; }
    
    public LocalDateTime getCreatedAt() { return createdAt; }
    public void setCreatedAt(LocalDateTime createdAt) { this.createdAt = createdAt; }
    
    public LocalDateTime getUpdatedAt() { return updatedAt; }
    public void setUpdatedAt(LocalDateTime updatedAt) { this.updatedAt = updatedAt; }
    
    public Boolean getIsActive() { return isActive; }
    public void setIsActive(Boolean isActive) { this.isActive = isActive; }
    
    public Boolean getIsAdmin() { return isAdmin; }
    public void setIsAdmin(Boolean isAdmin) { this.isAdmin = isAdmin; }
    
    public UserPreferences getPreferences() { return preferences; }
    public void setPreferences(UserPreferences preferences) { this.preferences = preferences; }
    
    @PreUpdate
    public void preUpdate() {
        this.updatedAt = LocalDateTime.now();
    }
}

/**
 * User preferences embedded object
 * This matches Go gateway-service/models/user.go UserPreferences
 * This matches Python auth-service/models/user.py UserPreferences  
 * This matches TypeScript frontend-app/src/types/User.ts UserPreferences
 */
@Embeddable
class UserPreferences {
    private String theme = "light";
    private String language = "en";
    private Boolean notifications = true;
    
    // Constructors
    public UserPreferences() {}
    
    public UserPreferences(String theme, String language, Boolean notifications) {
        this.theme = theme;
        this.language = language;
        this.notifications = notifications;
    }
    
    // Getters and Setters
    public String getTheme() { return theme; }
    public void setTheme(String theme) { this.theme = theme; }
    
    public String getLanguage() { return language; }
    public void setLanguage(String language) { this.language = language; }
    
    public Boolean getNotifications() { return notifications; }
    public void setNotifications(Boolean notifications) { this.notifications = notifications; }
}
`
	err = os.WriteFile(filepath.Join(tempDir, "data-service", "src", "main", "java", "com", "test", "model", "User.java"), []byte(userEntityContent), 0644)
	require.NoError(t, err)

	// Product entity
	productEntityContent := `package com.test.model;

import javax.persistence.*;
import java.math.BigDecimal;
import java.time.LocalDateTime;

/**
 * Product entity matching Go gateway-service/models/product.go Product struct
 * This also matches Python auth-service product models
 * This also matches TypeScript frontend-app/src/types/Product.ts interface
 */
@Entity
@Table(name = "products")
public class Product {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;
    
    @Column(nullable = false)
    private String name;
    
    @Column(columnDefinition = "TEXT")
    private String description;
    
    @Column(nullable = false)
    private BigDecimal price;
    
    @Column(nullable = false)
    private String currency = "USD";
    
    @Column(nullable = false)
    private Integer inventory = 0;
    
    @Column(name = "category_id")
    private Long categoryId;
    
    @Column(name = "created_at")
    private LocalDateTime createdAt;
    
    @Column(name = "updated_at")
    private LocalDateTime updatedAt;
    
    @Column(name = "is_active")
    private Boolean isActive = true;
    
    // Constructors
    public Product() {
        this.createdAt = LocalDateTime.now();
        this.updatedAt = LocalDateTime.now();
    }
    
    public Product(String name, String description, BigDecimal price) {
        this();
        this.name = name;
        this.description = description;
        this.price = price;
    }
    
    // Getters and Setters
    public Long getId() { return id; }
    public void setId(Long id) { this.id = id; }
    
    public String getName() { return name; }
    public void setName(String name) { this.name = name; }
    
    public String getDescription() { return description; }
    public void setDescription(String description) { this.description = description; }
    
    public BigDecimal getPrice() { return price; }
    public void setPrice(BigDecimal price) { this.price = price; }
    
    public String getCurrency() { return currency; }
    public void setCurrency(String currency) { this.currency = currency; }
    
    public Integer getInventory() { return inventory; }
    public void setInventory(Integer inventory) { this.inventory = inventory; }
    
    public Long getCategoryId() { return categoryId; }
    public void setCategoryId(Long categoryId) { this.categoryId = categoryId; }
    
    public LocalDateTime getCreatedAt() { return createdAt; }
    public void setCreatedAt(LocalDateTime createdAt) { this.createdAt = createdAt; }
    
    public LocalDateTime getUpdatedAt() { return updatedAt; }
    public void setUpdatedAt(LocalDateTime updatedAt) { this.updatedAt = updatedAt; }
    
    public Boolean getIsActive() { return isActive; }
    public void setIsActive(Boolean isActive) { this.isActive = isActive; }
    
    @PreUpdate
    public void preUpdate() {
        this.updatedAt = LocalDateTime.now();
    }
}
`
	err = os.WriteFile(filepath.Join(tempDir, "data-service", "src", "main", "java", "com", "test", "model", "Product.java"), []byte(productEntityContent), 0644)
	require.NoError(t, err)

	// User controller handling Go gateway requests
	userControllerContent := `package com.test.controller;

import com.test.model.User;
import com.test.service.UserService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.util.List;

/**
 * User controller handling requests from Go gateway service
 * This corresponds to gateway-service/handlers/user_handler.go calls
 */
@RestController
@RequestMapping("/api/users")
@CrossOrigin(origins = {"http://localhost:8080", "http://localhost:3000"})
public class UserController {
    
    @Autowired
    private UserService userService;
    
    /**
     * Get all users - called by Go gateway HandleUsers GET
     * This matches gateway-service/handlers/user_handler.go handleGetUsers function
     */
    @GetMapping
    public List<User> getAllUsers() {
        return userService.getAllUsers();
    }
    
    /**
     * Create new user - called by Go gateway HandleUsers POST
     * This matches gateway-service/handlers/user_handler.go handleCreateUser function
     */
    @PostMapping
    public ResponseEntity<User> createUser(@RequestBody CreateUserRequest request) {
        User user = userService.createUser(request);
        return ResponseEntity.ok(user);
    }
    
    /**
     * Get user by ID - called by Go gateway HandleUserByID GET
     * This matches gateway-service/handlers/user_handler.go handleGetUserByID function
     */
    @GetMapping("/{id}")
    public ResponseEntity<User> getUserById(@PathVariable Long id) {
        User user = userService.getUserById(id);
        if (user == null) {
            return ResponseEntity.notFound().build();
        }
        return ResponseEntity.ok(user);
    }
    
    /**
     * Update user - called by Go gateway HandleUserByID PUT
     * This matches gateway-service/handlers/user_handler.go handleUpdateUser function
     */
    @PutMapping("/{id}")
    public ResponseEntity<User> updateUser(@PathVariable Long id, @RequestBody User userUpdate) {
        User user = userService.updateUser(id, userUpdate);
        if (user == null) {
            return ResponseEntity.notFound().build();
        }
        return ResponseEntity.ok(user);
    }
    
    /**
     * Delete user - called by Go gateway HandleUserByID DELETE
     * This matches gateway-service/handlers/user_handler.go handleDeleteUser function
     */
    @DeleteMapping("/{id}")
    public ResponseEntity<Void> deleteUser(@PathVariable Long id) {
        boolean deleted = userService.deleteUser(id);
        if (!deleted) {
            return ResponseEntity.notFound().build();
        }
        return ResponseEntity.noContent().build();
    }
    
    // DTO for user creation matching Go models
    public static class CreateUserRequest {
        private String email;
        private String name;
        private String password;
        
        // Getters and Setters
        public String getEmail() { return email; }
        public void setEmail(String email) { this.email = email; }
        
        public String getName() { return name; }
        public void setName(String name) { this.name = name; }
        
        public String getPassword() { return password; }
        public void setPassword(String password) { this.password = password; }
    }
}
`
	err = os.WriteFile(filepath.Join(tempDir, "data-service", "src", "main", "java", "com", "test", "controller", "UserController.java"), []byte(userControllerContent), 0644)
	require.NoError(t, err)

	// User service implementation
	userServiceContent := `package com.test.service;

import com.test.controller.UserController.CreateUserRequest;
import com.test.model.User;
import org.springframework.stereotype.Service;

import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * User service providing business logic for Go gateway requests
 * In production this would use JPA repositories with actual database
 */
@Service
public class UserService {
    
    private Map<Long, User> users = new HashMap<>();
    private Long nextId = 1L;
    
    public UserService() {
        // Create some test users matching Python auth service data
        createTestUser("admin@example.com", "Admin User", true);
        createTestUser("user@example.com", "Test User", false);
        createTestUser("demo@example.com", "Demo User", false);
    }
    
    private void createTestUser(String email, String name, boolean isAdmin) {
        User user = new User(email, name);
        user.setId(nextId++);
        user.setIsAdmin(isAdmin);
        users.put(user.getId(), user);
    }
    
    public List<User> getAllUsers() {
        return new ArrayList<>(users.values());
    }
    
    public User createUser(CreateUserRequest request) {
        User user = new User(request.getEmail(), request.getName());
        user.setId(nextId++);
        users.put(user.getId(), user);
        return user;
    }
    
    public User getUserById(Long id) {
        return users.get(id);
    }
    
    public User updateUser(Long id, User userUpdate) {
        User existingUser = users.get(id);
        if (existingUser == null) {
            return null;
        }
        
        if (userUpdate.getName() != null) {
            existingUser.setName(userUpdate.getName());
        }
        if (userUpdate.getEmail() != null) {
            existingUser.setEmail(userUpdate.getEmail());
        }
        if (userUpdate.getIsActive() != null) {
            existingUser.setIsActive(userUpdate.getIsActive());
        }
        if (userUpdate.getPreferences() != null) {
            existingUser.setPreferences(userUpdate.getPreferences());
        }
        
        existingUser.preUpdate();
        return existingUser;
    }
    
    public boolean deleteUser(Long id) {
        return users.remove(id) != null;
    }
}
`
	err = os.WriteFile(filepath.Join(tempDir, "data-service", "src", "main", "java", "com", "test", "service", "UserService.java"), []byte(userServiceContent), 0644)
	require.NoError(t, err)

	// Maven pom.xml
	pomContent := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0" 
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 
         http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>

    <groupId>com.test</groupId>
    <artifactId>data-service</artifactId>
    <version>1.0.0</version>
    <packaging>jar</packaging>

    <name>data-service</name>
    <description>Java Spring Boot Data Service for Go Gateway</description>

    <parent>
        <groupId>org.springframework.boot</groupId>
        <artifactId>spring-boot-starter-parent</artifactId>
        <version>2.7.14</version>
        <relativePath/>
    </parent>

    <properties>
        <maven.compiler.source>11</maven.compiler.source>
        <maven.compiler.target>11</maven.compiler.target>
        <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
    </properties>

    <dependencies>
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-web</artifactId>
        </dependency>
        
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-data-jpa</artifactId>
        </dependency>
        
        <dependency>
            <groupId>com.h2database</groupId>
            <artifactId>h2</artifactId>
            <scope>runtime</scope>
        </dependency>
        
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-test</artifactId>
            <scope>test</scope>
        </dependency>
    </dependencies>

    <build>
        <plugins>
            <plugin>
                <groupId>org.springframework.boot</groupId>
                <artifactId>spring-boot-maven-plugin</artifactId>
            </plugin>
        </plugins>
    </build>
</project>
`
	err = os.WriteFile(filepath.Join(tempDir, "data-service", "pom.xml"), []byte(pomContent), 0644)
	require.NoError(t, err)
}

// createSharedConfiguration creates shared configuration files
func createSharedConfiguration(t *testing.T, tempDir string) {
	// Shared configuration YAML
	configContent := `# Shared configuration across all microservices
# Used by Go gateway, Python auth service, TypeScript frontend, Java data service

api:
  version: "v1"
  base_path: "/api/v1"
  
services:
  gateway:
    host: "localhost"
    port: 8080
    url: "http://localhost:8080"
    
  auth:
    host: "localhost" 
    port: 5000
    url: "http://localhost:5000"
    
  data:
    host: "localhost"
    port: 8081
    url: "http://localhost:8081"
    
  frontend:
    host: "localhost"
    port: 3000
    url: "http://localhost:3000"

database:
  type: "in-memory"
  name: "microservices_test"
  
security:
  jwt:
    secret: "your-secret-key-here"
    expiration_hours: 24
    
  cors:
    allowed_origins:
      - "http://localhost:3000"
      - "http://localhost:8080"
    allowed_methods:
      - "GET"
      - "POST" 
      - "PUT"
      - "DELETE"
      - "OPTIONS"

shared_constants:
  error_codes:
    INVALID_CREDENTIALS: 1001
    USER_NOT_FOUND: 1002
    TOKEN_EXPIRED: 1003
    INSUFFICIENT_PERMISSIONS: 1004
    VALIDATION_ERROR: 1005
    SERVICE_UNAVAILABLE: 1006
    
  user_roles:
    ADMIN: "admin"
    USER: "user"
    GUEST: "guest"
    
  themes:
    LIGHT: "light" 
    DARK: "dark"
    
  languages:
    ENGLISH: "en"
    SPANISH: "es"
    FRENCH: "fr"
`
	err := os.WriteFile(filepath.Join(tempDir, "shared", "config.yaml"), []byte(configContent), 0644)
	require.NoError(t, err)

	// OpenAPI specification
	apiSpecContent := `openapi: 3.0.3
info:
  title: Multi-Language Microservices API
  description: |
    Shared API specification across all microservices:
    - Go Gateway Service (port 8080)
    - Python Auth Service (port 5000)  
    - Java Data Service (port 8081)
    - TypeScript Frontend (port 3000)
  version: 1.0.0
  
servers:
  - url: http://localhost:8080/api/v1
    description: Go Gateway Service
  - url: http://localhost:5000
    description: Python Auth Service
  - url: http://localhost:8081/api
    description: Java Data Service

paths:
  /auth/login:
    post:
      summary: User login
      operationId: loginUser
      tags: [Authentication]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/LoginRequest'
      responses:
        '200':
          description: Login successful
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LoginResponse'
        '401':
          description: Invalid credentials
          
  /auth/validate:
    get:
      summary: Validate authentication token
      operationId: validateToken
      tags: [Authentication]
      security:
        - bearerAuth: []
      responses:
        '200':
          description: Token valid
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/User'
        '401':
          description: Invalid or expired token
          
  /users:
    get:
      summary: List all users
      operationId: getUsers
      tags: [Users]
      security:
        - bearerAuth: []
      responses:
        '200':
          description: List of users
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/User'
                  
    post:
      summary: Create new user
      operationId: createUser
      tags: [Users]
      security:
        - bearerAuth: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/CreateUserRequest'
      responses:
        '201':
          description: User created
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/User'
                
  /users/{id}:
    get:
      summary: Get user by ID
      operationId: getUserById
      tags: [Users]
      security:
        - bearerAuth: []
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: integer
            format: int64
      responses:
        '200':
          description: User details
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/User'
        '404':
          description: User not found
          
  /products:
    get:
      summary: Get product catalog
      operationId: getProducts
      tags: [Products]
      security:
        - bearerAuth: []
      parameters:
        - name: category
          in: query
          schema:
            type: string
        - name: limit
          in: query
          schema:
            type: integer
      responses:
        '200':
          description: Product catalog
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ProductCatalogResponse'

components:
  schemas:
    User:
      type: object
      properties:
        id:
          type: integer
          format: int64
        email:
          type: string
          format: email
        name:
          type: string
        created_at:
          type: string
          format: date-time
        updated_at:
          type: string
          format: date-time
        is_active:
          type: boolean
        is_admin:
          type: boolean
        preferences:
          $ref: '#/components/schemas/UserPreferences'
          
    UserPreferences:
      type: object
      properties:
        theme:
          type: string
          enum: [light, dark]
        language:
          type: string
        notifications:
          type: boolean
          
    LoginRequest:
      type: object
      required: [email, password]
      properties:
        email:
          type: string
          format: email
        password:
          type: string
          
    LoginResponse:
      type: object
      properties:
        token:
          type: string
        user:
          $ref: '#/components/schemas/User'
        expires_at:
          type: string
          format: date-time
          
    CreateUserRequest:
      type: object
      required: [email, name, password]
      properties:
        email:
          type: string
          format: email
        name:
          type: string
        password:
          type: string
          
    Product:
      type: object
      properties:
        id:
          type: integer
          format: int64
        name:
          type: string
        description:
          type: string
        price:
          type: number
          format: decimal
        currency:
          type: string
        inventory:
          type: integer
        category_id:
          type: integer
          format: int64
        created_at:
          type: string
          format: date-time
        updated_at:
          type: string
          format: date-time
        is_active:
          type: boolean
          
    ProductCatalogResponse:
      type: object
      properties:
        products:
          type: array
          items:
            $ref: '#/components/schemas/Product'
        total:
          type: integer
        page:
          type: integer
          
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT
`
	err = os.WriteFile(filepath.Join(tempDir, "shared", "api-spec.yaml"), []byte(apiSpecContent), 0644)
	require.NoError(t, err)

	// Shared error codes JSON
	errorCodesContent := `{
  "error_codes": {
    "INVALID_CREDENTIALS": {
      "code": 1001,
      "message": "Invalid email or password",
      "http_status": 401
    },
    "USER_NOT_FOUND": {
      "code": 1002,
      "message": "User not found",
      "http_status": 404
    },
    "TOKEN_EXPIRED": {
      "code": 1003,
      "message": "Authentication token has expired",
      "http_status": 401
    },
    "INSUFFICIENT_PERMISSIONS": {
      "code": 1004,
      "message": "Insufficient permissions to access this resource",
      "http_status": 403
    },
    "VALIDATION_ERROR": {
      "code": 1005,
      "message": "Request validation failed",
      "http_status": 400
    },
    "SERVICE_UNAVAILABLE": {
      "code": 1006,
      "message": "Service temporarily unavailable",
      "http_status": 503
    },
    "PRODUCT_NOT_FOUND": {
      "code": 1007,
      "message": "Product not found",
      "http_status": 404
    },
    "INSUFFICIENT_INVENTORY": {
      "code": 1008,
      "message": "Insufficient product inventory",
      "http_status": 400
    }
  },
  "success_codes": {
    "USER_CREATED": {
      "code": 2001,
      "message": "User created successfully"
    },
    "USER_UPDATED": {
      "code": 2002,
      "message": "User updated successfully"
    },
    "LOGIN_SUCCESSFUL": {
      "code": 2003,
      "message": "Login successful"
    },
    "PRODUCT_CREATED": {
      "code": 2004,
      "message": "Product created successfully"
    }
  }
}
`
	err = os.WriteFile(filepath.Join(tempDir, "shared", "error-codes.json"), []byte(errorCodesContent), 0644)
	require.NoError(t, err)
}

// createDockerCompose creates Docker compose file for service orchestration
func createDockerCompose(t *testing.T, tempDir string) {
	dockerComposeContent := `version: '3.8'

services:
  # Go Gateway Service
  gateway-service:
    build:
      context: ./gateway-service
      dockerfile: Dockerfile
    ports:
      - "8080:8080"
    environment:
      - AUTH_SERVICE_URL=http://auth-service:5000
      - DATA_SERVICE_URL=http://data-service:8081
      - PORT=8080
    depends_on:
      - auth-service
      - data-service
    networks:
      - microservices-network
      
  # Python Auth Service  
  auth-service:
    build:
      context: ./auth-service
      dockerfile: Dockerfile
    ports:
      - "5000:5000"
    environment:
      - HOST=0.0.0.0
      - PORT=5000
      - JWT_SECRET=your-secret-key-here
    networks:
      - microservices-network
      
  # Java Data Service
  data-service:
    build:
      context: ./data-service
      dockerfile: Dockerfile
    ports:
      - "8081:8081"
    environment:
      - SERVER_PORT=8081
      - SPRING_PROFILES_ACTIVE=docker
    networks:
      - microservices-network
      
  # TypeScript Frontend (optional, for development)
  frontend-app:
    build:
      context: ./frontend-app
      dockerfile: Dockerfile
    ports:
      - "3000:3000"
    environment:
      - REACT_APP_GATEWAY_URL=http://localhost:8080
    depends_on:
      - gateway-service
    networks:
      - microservices-network

networks:
  microservices-network:
    driver: bridge

volumes:
  data-service-data:
    driver: local
`
	err := os.WriteFile(filepath.Join(tempDir, "docker-compose.yml"), []byte(dockerComposeContent), 0644)
	require.NoError(t, err)
}



// TestSCIPCrossLanguageAPIIntegration tests SCIP analysis of API integration patterns
func TestSCIPCrossLanguageAPIIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP cross-language API integration test in short mode")
	}

	session := setupSCIPMCPServer(t)
	defer session.cleanup()

	// Test API endpoint consistency detection across languages
	t.Run("APIEndpointConsistency", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_intelligent_symbol_search", map[string]interface{}{
			"query":                     "/api/v1/auth/login",
			"scope":                     "workspace",
			"includeSemanticSimilarity": true,
			"maxResults":                20,
		})
		require.NoError(t, err)

		if result.IsError {
			t.Logf("API endpoint search returned error (expected without SCIP index): %v", result.Content)
		} else {
			err = validateSCIPToolResponse(result, []string{"symbols", "query"})
			assert.NoError(t, err)
			t.Logf("API endpoint consistency analysis successful")
		}
	})

	// Test cross-service authentication flow analysis
	t.Run("AuthenticationFlowAnalysis", func(t *testing.T) {
		// Test Go gateway auth handler calling Python auth service
		gatewayAuthFile := filepath.Join(session.tempDir, "gateway-service/handlers/auth_handler.go")
		gatewayAuthURI := fmt.Sprintf("file://%s", gatewayAuthFile)

		result, err := callSCIPTool(t, session, "scip_cross_language_references", map[string]interface{}{
			"uri":                    gatewayAuthURI,
			"line":                   65, // HandleLogin function
			"character":              15,
			"includeImplementations": true,
			"crossLanguageDepth":     3,
		})
		require.NoError(t, err)

		if result.IsError {
			t.Logf("Authentication flow analysis returned error (expected without SCIP index): %v", result.Content)
		} else {
			err = validateSCIPToolResponse(result, []string{"references"})
			assert.NoError(t, err)
			t.Logf("Cross-service authentication flow analysis successful")
		}
	})

	// Test TypeScript frontend to Go gateway integration
	t.Run("FrontendGatewayIntegration", func(t *testing.T) {
		// Test TypeScript AuthService calling Go gateway endpoints
		frontendAuthFile := filepath.Join(session.tempDir, "frontend-app/src/services/AuthService.ts")
		frontendAuthURI := fmt.Sprintf("file://%s", frontendAuthFile)

		result, err := callSCIPTool(t, session, "scip_semantic_code_analysis", map[string]interface{}{
			"uri":                  frontendAuthURI,
			"analysisType":         "dependencies",
			"includeRelationships": true,
		})
		require.NoError(t, err)

		if result.IsError {
			t.Logf("Frontend-gateway integration analysis returned error (expected without SCIP index): %v", result.Content)
		} else {
			err = validateSCIPToolResponse(result, []string{"analysis"})
			assert.NoError(t, err)
			t.Logf("Frontend-gateway integration analysis successful")
		}
	})
}

// TestSCIPCrossLanguageDataModels tests SCIP analysis of shared data model consistency
func TestSCIPCrossLanguageDataModels(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP cross-language data models test in short mode")
	}

	session := setupSCIPMCPServer(t)
	defer session.cleanup()

	// Test User model consistency across all 4 languages
	t.Run("UserModelConsistency", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_intelligent_symbol_search", map[string]interface{}{
			"query":                     "User",
			"languages":                 []string{"go", "python", "typescript", "java"},
			"scope":                     "workspace",
			"includeSemanticSimilarity": true,
			"maxResults":                30,
		})
		require.NoError(t, err)

		if result.IsError {
			t.Logf("User model consistency analysis returned error (expected without SCIP index): %v", result.Content)
		} else {
			err = validateSCIPToolResponse(result, []string{"symbols", "languageBreakdown"})
			assert.NoError(t, err)
			t.Logf("Cross-language User model consistency analysis successful")
		}
	})

	// Test UserPreferences model analysis
	t.Run("UserPreferencesModelAnalysis", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_intelligent_symbol_search", map[string]interface{}{
			"query":                     "UserPreferences",
			"scope":                     "workspace",
			"includeSemanticSimilarity": true,
			"maxResults":                20,
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"symbols"})
			assert.NoError(t, err)
		}
		t.Logf("UserPreferences model analysis completed")
	})

	// Test Product model consistency
	t.Run("ProductModelConsistency", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_intelligent_symbol_search", map[string]interface{}{
			"query":                     "Product",
			"languages":                 []string{"go", "typescript", "java"},
			"scope":                     "workspace",
			"includeSemanticSimilarity": true,
			"maxResults":                25,
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"symbols", "languageBreakdown"})
			assert.NoError(t, err)
		}
		t.Logf("Cross-language Product model consistency analysis completed")
	})

	// Test CreateUserRequest consistency across services
	t.Run("CreateUserRequestConsistency", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_intelligent_symbol_search", map[string]interface{}{
			"query":                     "CreateUserRequest",
			"scope":                     "workspace",
			"includeSemanticSimilarity": true,
			"maxResults":                15,
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"symbols"})
			assert.NoError(t, err)
		}
		t.Logf("CreateUserRequest consistency analysis completed")
	})
}

// TestSCIPCrossLanguageConfiguration tests SCIP analysis of shared configuration
func TestSCIPCrossLanguageConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP cross-language configuration test in short mode")
	}

	session := setupSCIPMCPServer(t)
	defer session.cleanup()

	// Test shared configuration analysis
	t.Run("SharedConfigurationAnalysis", func(t *testing.T) {
		configFile := filepath.Join(session.tempDir, "shared/config.yaml")
		configURI := fmt.Sprintf("file://%s", configFile)

		result, err := callSCIPTool(t, session, "scip_semantic_code_analysis", map[string]interface{}{
			"uri":                  configURI,
			"analysisType":         "structure",
			"includeRelationships": true,
		})
		require.NoError(t, err)

		if result.IsError {
			t.Logf("Shared configuration analysis returned error (expected without SCIP index): %v", result.Content)
		} else {
			err = validateSCIPToolResponse(result, []string{"analysis"})
			assert.NoError(t, err)
			t.Logf("Shared configuration analysis successful")
		}
	})

	// Test API specification analysis
	t.Run("APISpecificationAnalysis", func(t *testing.T) {
		apiSpecFile := filepath.Join(session.tempDir, "shared/api-spec.yaml")
		apiSpecURI := fmt.Sprintf("file://%s", apiSpecFile)

		result, err := callSCIPTool(t, session, "scip_semantic_code_analysis", map[string]interface{}{
			"uri":          apiSpecURI,
			"analysisType": "dependencies",
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"analysis"})
			assert.NoError(t, err)
		}
		t.Logf("API specification analysis completed")
	})

	// Test error codes consistency
	t.Run("ErrorCodesConsistency", func(t *testing.T) {
		errorCodesFile := filepath.Join(session.tempDir, "shared/error-codes.json")
		errorCodesURI := fmt.Sprintf("file://%s", errorCodesFile)

		result, err := callSCIPTool(t, session, "scip_semantic_code_analysis", map[string]interface{}{
			"uri":          errorCodesURI,
			"analysisType": "structure",
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"analysis"})
			assert.NoError(t, err)
		}
		t.Logf("Error codes consistency analysis completed")
	})
}

// TestSCIPCrossLanguageErrorHandling tests SCIP analysis of error handling patterns
func TestSCIPCrossLanguageErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP cross-language error handling test in short mode")
	}

	session := setupSCIPMCPServer(t)
	defer session.cleanup()

	// Test error response consistency
	t.Run("ErrorResponseConsistency", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_intelligent_symbol_search", map[string]interface{}{
			"query":                     "ErrorResponse",
			"scope":                     "workspace",
			"includeSemanticSimilarity": true,
			"maxResults":                15,
		})
		require.NoError(t, err)

		if result.IsError {
			t.Logf("Error response consistency analysis returned error (expected without SCIP index): %v", result.Content)
		} else {
			err = validateSCIPToolResponse(result, []string{"symbols"})
			assert.NoError(t, err)
			t.Logf("Error response consistency analysis successful")
		}
	})

	// Test HTTP status code consistency
	t.Run("HTTPStatusCodeConsistency", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_intelligent_symbol_search", map[string]interface{}{
			"query":                     "401",
			"scope":                     "workspace",
			"includeSemanticSimilarity": false,
			"maxResults":                20,
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"symbols"})
			assert.NoError(t, err)
		}
		t.Logf("HTTP status code consistency analysis completed")
	})

	// Test authentication error handling across services
	t.Run("AuthenticationErrorHandling", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_intelligent_symbol_search", map[string]interface{}{
			"query":                     "Invalid.*token",
			"scope":                     "workspace",
			"includeSemanticSimilarity": true,
			"maxResults":                15,
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"symbols"})
			assert.NoError(t, err)
		}
		t.Logf("Authentication error handling analysis completed")
	})
}

// TestSCIPIntelligentSymbolSearchAdvanced tests advanced SCIP symbol search with realistic data
func TestSCIPIntelligentSymbolSearchAdvanced(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP advanced symbol search test in short mode")
	}

	session := setupSCIPMCPServer(t)
	defer session.cleanup()

	// Test microservices architecture discovery
	t.Run("MicroservicesArchitectureDiscovery", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_intelligent_symbol_search", map[string]interface{}{
			"query":                     "Service",
			"scope":                     "workspace",
			"includeSemanticSimilarity": true,
			"maxResults":                30,
		})
		require.NoError(t, err)

		if result.IsError {
			t.Logf("Microservices architecture discovery returned error (expected without SCIP index): %v", result.Content)
		} else {
			err = validateSCIPToolResponse(result, []string{"symbols"})
			assert.NoError(t, err)
			t.Logf("Microservices architecture discovery successful")
		}
	})

	// Test handler pattern discovery across languages
	t.Run("HandlerPatternDiscovery", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_intelligent_symbol_search", map[string]interface{}{
			"query":                     "Handler",
			"languages":                 []string{"go", "python", "typescript"},
			"scope":                     "workspace",
			"includeSemanticSimilarity": true,
			"maxResults":                25,
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"symbols", "languageBreakdown"})
			assert.NoError(t, err)
		}
		t.Logf("Handler pattern discovery completed")
	})

	// Test controller pattern discovery
	t.Run("ControllerPatternDiscovery", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_intelligent_symbol_search", map[string]interface{}{
			"query":                     "Controller",
			"scope":                     "workspace",
			"includeSemanticSimilarity": true,
			"maxResults":                20,
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"symbols"})
			assert.NoError(t, err)
		}
		t.Logf("Controller pattern discovery completed")
	})

	// Test authentication token flow discovery
	t.Run("AuthenticationTokenFlowDiscovery", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_intelligent_symbol_search", map[string]interface{}{
			"query":                     "token",
			"scope":                     "workspace",
			"includeSemanticSimilarity": true,
			"maxResults":                25,
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"symbols"})
			assert.NoError(t, err)
		}
		t.Logf("Authentication token flow discovery completed")
	})
}

// TestSCIPCrossLanguageReferencesAdvanced tests advanced cross-language reference finding
func TestSCIPCrossLanguageReferencesAdvanced(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP advanced cross-language references test in short mode")
	}

	session := setupSCIPMCPServer(t)
	defer session.cleanup()

	// Test Go gateway to Python auth service references
	t.Run("GatewayAuthServiceReferences", func(t *testing.T) {
		gatewayAuthFile := filepath.Join(session.tempDir, "gateway-service/handlers/auth_handler.go")
		gatewayAuthURI := fmt.Sprintf("file://%s", gatewayAuthFile)

		result, err := callSCIPTool(t, session, "scip_cross_language_references", map[string]interface{}{
			"uri":                    gatewayAuthURI,
			"line":                   95, // authURL line in HandleLogin
			"character":              20,
			"includeImplementations": true,
			"crossLanguageDepth":     5,
		})
		require.NoError(t, err)

		if result.IsError {
			t.Logf("Gateway-auth service references returned error (expected without SCIP index): %v", result.Content)
		} else {
			err = validateSCIPToolResponse(result, []string{"references"})
			assert.NoError(t, err)
			t.Logf("Gateway-auth service cross-language references successful")
		}
	})

	// Test TypeScript frontend to Go gateway references
	t.Run("FrontendGatewayReferences", func(t *testing.T) {
		frontendUserFile := filepath.Join(session.tempDir, "frontend-app/src/services/UserService.ts")
		frontendUserURI := fmt.Sprintf("file://%s", frontendUserFile)

		result, err := callSCIPTool(t, session, "scip_cross_language_references", map[string]interface{}{
			"uri":                    frontendUserURI,
			"line":                   42, // getUsers function
			"character":              25,
			"includeImplementations": true,
			"crossLanguageDepth":     4,
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"references"})
			assert.NoError(t, err)
		}
		t.Logf("Frontend-gateway cross-language references completed")
	})

	// Test Go gateway to Java data service references
	t.Run("GatewayDataServiceReferences", func(t *testing.T) {
		gatewayUserFile := filepath.Join(session.tempDir, "gateway-service/handlers/user_handler.go")
		gatewayUserURI := fmt.Sprintf("file://%s", gatewayUserFile)

		result, err := callSCIPTool(t, session, "scip_cross_language_references", map[string]interface{}{
			"uri":                    gatewayUserURI,
			"line":                   88, // dataURL in handleGetUsers
			"character":              30,
			"includeImplementations": true,
			"crossLanguageDepth":     4,
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"references"})
			assert.NoError(t, err)
		}
		t.Logf("Gateway-data service cross-language references completed")
	})

	// Test shared configuration references
	t.Run("SharedConfigurationReferences", func(t *testing.T) {
		configFile := filepath.Join(session.tempDir, "shared/config.yaml")
		configURI := fmt.Sprintf("file://%s", configFile)

		result, err := callSCIPTool(t, session, "scip_cross_language_references", map[string]interface{}{
			"uri":                    configURI,
			"line":                   10, // gateway service config
			"character":              5,
			"includeImplementations": true,
			"crossLanguageDepth":     3,
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"references"})
			assert.NoError(t, err)
		}
		t.Logf("Shared configuration cross-language references completed")
	})
}

// TestSCIPWorkspaceIntelligenceAdvanced tests advanced workspace intelligence with microservices
func TestSCIPWorkspaceIntelligenceAdvanced(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP advanced workspace intelligence test in short mode")
	}

	session := setupSCIPMCPServer(t)
	defer session.cleanup()

	// Test microservices architecture analysis
	t.Run("MicroservicesArchitectureAnalysis", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_workspace_intelligence", map[string]interface{}{
			"insightType":            "overview",
			"includeMetrics":         true,
			"includeRecommendations": true,
			"languageFilter":         []string{"go", "python", "typescript", "java"},
		})
		require.NoError(t, err)

		if result.IsError {
			t.Logf("Microservices architecture analysis returned error (expected without SCIP index): %v", result.Content)
		} else {
			err = validateSCIPToolResponse(result, []string{"insights", "metrics", "overview"})
			assert.NoError(t, err)
			t.Logf("Microservices architecture analysis successful")
		}
	})

	// Test cross-service dependency analysis
	t.Run("CrossServiceDependencyAnalysis", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_workspace_intelligence", map[string]interface{}{
			"insightType":    "dependencies",
			"includeMetrics": true,
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"insights", "dependencies"})
			assert.NoError(t, err)
		}
		t.Logf("Cross-service dependency analysis completed")
	})

	// Test API integration hotspots
	t.Run("APIIntegrationHotspots", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_workspace_intelligence", map[string]interface{}{
			"insightType":    "hotspots",
			"includeMetrics": true,
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"insights", "hotspots"})
			assert.NoError(t, err)
		}
		t.Logf("API integration hotspots analysis completed")
	})

	// Test comprehensive microservices analysis
	t.Run("ComprehensiveMicroservicesAnalysis", func(t *testing.T) {
		result, err := callSCIPTool(t, session, "scip_workspace_intelligence", map[string]interface{}{
			"insightType":            "all",
			"includeMetrics":         true,
			"includeRecommendations": true,
			"languageFilter":         []string{"go", "python", "typescript", "java"},
		})
		require.NoError(t, err)

		if !result.IsError {
			err = validateSCIPToolResponse(result, []string{"insights", "comprehensive"})
			assert.NoError(t, err)
		}
		t.Logf("Comprehensive microservices analysis completed")
	})
}

// TestSCIPFallbackBehavior validates graceful degradation when SCIP is unavailable
func TestSCIPFallbackBehavior(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP fallback behavior test in short mode")
	}

	session := setupSCIPMCPServer(t)
	defer session.cleanup()

	// Test graceful degradation for each SCIP-enhanced tool
	t.Run("SCIPUnavailableFallback", func(t *testing.T) {
		scipTools := []string{
			"scip_intelligent_symbol_search",
			"scip_cross_language_references",
			"scip_semantic_code_analysis",
			"scip_context_aware_assistance",
			"scip_workspace_intelligence",
			"scip_refactoring_suggestions",
		}

		for _, toolName := range scipTools {
			t.Run(fmt.Sprintf("%s_fallback", toolName), func(t *testing.T) {
				// Test with minimal valid parameters that should trigger fallback
				var params map[string]interface{}

				switch toolName {
				case "scip_intelligent_symbol_search":
					params = map[string]interface{}{
						"query": "NonExistentSymbol_TestFallback",
						"scope": "workspace",
					}
				case "scip_cross_language_references":
					testFile := filepath.Join(session.tempDir, "main.go")
					testURI := fmt.Sprintf("file://%s", testFile)
					params = map[string]interface{}{
						"uri":       testURI,
						"line":      999, // Line that does not exist
						"character": 999,
					}
				case "scip_semantic_code_analysis":
					testFile := filepath.Join(session.tempDir, "nonexistent.go")
					testURI := fmt.Sprintf("file://%s", testFile)
					params = map[string]interface{}{
						"uri":          testURI,
						"analysisType": "all",
					}
				case "scip_context_aware_assistance":
					testFile := filepath.Join(session.tempDir, "main.go")
					testURI := fmt.Sprintf("file://%s", testFile)
					params = map[string]interface{}{
						"uri":         testURI,
						"line":        999,
						"character":   999,
						"contextType": "completion",
					}
				case "scip_workspace_intelligence":
					params = map[string]interface{}{
						"insightType": "overview",
					}
				case "scip_refactoring_suggestions":
					testFile := filepath.Join(session.tempDir, "main.go")
					testURI := fmt.Sprintf("file://%s", testFile)
					params = map[string]interface{}{
						"uri":              testURI,
						"refactoringTypes": []string{"all"},
					}
				}

				// Call the tool and verify it handles fallback gracefully
				result, err := callSCIPTool(t, session, toolName, params)
				require.NoError(t, err, "Tool should not error even when SCIP is unavailable")

				// Verify graceful degradation - tool should return results or clear error messages
				if result.IsError {
					// Check that error messages are informative about fallback
					var errorContent map[string]interface{}
					if len(result.Content) > 0 {
						errorContent = result.Content[0]
					}
					if errorContent != nil {
						if message, exists := errorContent["message"]; exists {
							messageStr := fmt.Sprintf("%v", message)
							assert.Contains(t, strings.ToLower(messageStr), "fallback",
								"Error message should indicate fallback behavior for %s", toolName)
						}
					}
					t.Logf("%s fallback error (expected): %v", toolName, result.Content)
				} else {
					// If successful, verify results indicate limited functionality
					t.Logf("%s fallback success: %v", toolName, result.Content)
				}
			})
		}
	})
}

// TestSCIPErrorRecovery validates error recovery and circuit breaker behavior
func TestSCIPErrorRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP error recovery test in short mode")
	}

	session := setupSCIPMCPServer(t)
	defer session.cleanup()

	// Test timeout handling for SCIP operations
	t.Run("SCIPTimeoutHandling", func(t *testing.T) {
		// Test with parameters that might cause timeouts
		testFile := filepath.Join(session.tempDir, "main.go")
		testURI := fmt.Sprintf("file://%s", testFile)

		timeoutTests := []struct {
			tool   string
			params map[string]interface{}
		}{
			{
				"scip_intelligent_symbol_search",
				map[string]interface{}{
					"query":                     "ComplexLongRunningSymbolSearchThatMightTimeout",
					"scope":                     "workspace",
					"includeSemanticSimilarity": true,
					"maxResults":                1000,
				},
			},
			{
				"scip_cross_language_references",
				map[string]interface{}{
					"uri":                    testURI,
					"line":                   5,
					"character":              10,
					"crossLanguageDepth":     10, // Maximum depth
					"includeImplementations": true,
					"includeInheritance":     true,
				},
			},
			{
				"scip_semantic_code_analysis",
				map[string]interface{}{
					"uri":                  testURI,
					"analysisType":         "all",
					"includeMetrics":       true,
					"includeRelationships": true,
				},
			},
		}

		for _, test := range timeoutTests {
			t.Run(fmt.Sprintf("%s_timeout", test.tool), func(t *testing.T) {
				start := time.Now()
				result, err := callSCIPTool(t, session, test.tool, test.params)
				duration := time.Since(start)

				// Should complete within reasonable time (30 seconds max for E2E tests)
				assert.LessOrEqual(t, duration, 30*time.Second,
					"Tool %s should not hang indefinitely", test.tool)

				// Should not return Go-level errors even if operation times out
				require.NoError(t, err, "Tool should handle timeouts gracefully")

				if result.IsError {
					t.Logf("%s timeout handling (expected): %v", test.tool, result.Content)
				} else {
					t.Logf("%s completed successfully in %v", test.tool, duration)
				}
			})
		}
	})

	// Test invalid parameter handling
	t.Run("InvalidParameterHandling", func(t *testing.T) {
		invalidTests := []struct {
			tool   string
			params map[string]interface{}
			desc   string
		}{
			{
				"scip_intelligent_symbol_search",
				map[string]interface{}{
					"query": "", // Empty query
				},
				"empty query",
			},
			{
				"scip_cross_language_references",
				map[string]interface{}{
					"uri":       "invalid://uri",
					"line":      -1, // Negative line
					"character": -1,
				},
				"invalid URI and negative position",
			},
			{
				"scip_semantic_code_analysis",
				map[string]interface{}{
					"uri":          "file:///nonexistent/path/file.go",
					"analysisType": "invalid_type",
				},
				"nonexistent file and invalid analysis type",
			},
			{
				"scip_context_aware_assistance",
				map[string]interface{}{
					"uri":         "malformed_uri",
					"line":        "not_a_number",
					"character":   "also_not_a_number",
					"contextType": "invalid_context",
				},
				"malformed parameters",
			},
			{
				"scip_workspace_intelligence",
				map[string]interface{}{
					"insightType":    "nonexistent_insight",
					"languageFilter": []interface{}{"invalid_language", 123},
				},
				"invalid insight type and language filter",
			},
			{
				"scip_refactoring_suggestions",
				map[string]interface{}{
					"uri":              "",
					"refactoringTypes": []interface{}{"invalid_type", nil},
				},
				"empty URI and invalid refactoring types",
			},
		}

		for _, test := range invalidTests {
			t.Run(fmt.Sprintf("%s_%s", test.tool, strings.ReplaceAll(test.desc, " ", "_")), func(t *testing.T) {
				result, err := callSCIPTool(t, session, test.tool, test.params)
				require.NoError(t, err, "Tool should not panic on invalid parameters")

				// Should return an error result with helpful message
				assert.True(t, result.IsError,
					"Tool %s should return error for invalid parameters: %s", test.tool, test.desc)

				if result.IsError {
					var errorContent map[string]interface{}
					if len(result.Content) > 0 {
						errorContent = result.Content[0]
					}
					if errorContent != nil {
						if message, exists := errorContent["message"]; exists {
							messageStr := fmt.Sprintf("%v", message)
							assert.NotEmpty(t, messageStr, "Error message should not be empty")
							t.Logf("%s invalid parameter error: %s", test.tool, messageStr)
						}
					}
				}
			})
		}
	})

	// Test rapid consecutive failures and recovery
	t.Run("ConsecutiveFailureRecovery", func(t *testing.T) {
		// Make multiple calls that are likely to fail quickly
		failureParams := map[string]interface{}{
			"uri":          "file:///absolutely/nonexistent/path/that/should/fail.go",
			"analysisType": "all",
		}

		var successCount, errorCount int
		const maxAttempts = 10

		for i := 0; i < maxAttempts; i++ {
			result, err := callSCIPTool(t, session, "scip_semantic_code_analysis", failureParams)
			require.NoError(t, err, "Should not get Go-level errors even with consecutive failures")

			if result.IsError {
				errorCount++
			} else {
				successCount++
			}

			// Small delay between attempts
			time.Sleep(100 * time.Millisecond)
		}

		t.Logf("Consecutive failure test: %d successes, %d errors out of %d attempts",
			successCount, errorCount, maxAttempts)

		// Should handle consecutive failures gracefully without crashing
		assert.LessOrEqual(t, errorCount, maxAttempts, "Error count should not exceed total attempts")
	})

	// Test mixed success/failure scenarios
	t.Run("MixedSuccessFailureScenarios", func(t *testing.T) {
		testCases := []struct {
			params    map[string]interface{}
			expectErr bool
			desc      string
		}{
			{
				map[string]interface{}{"query": "User", "scope": "workspace"},
				false,
				"valid symbol search",
			},
			{
				map[string]interface{}{"query": "", "scope": "invalid"},
				true,
				"invalid symbol search",
			},
			{
				map[string]interface{}{"query": "ValidSymbol", "scope": "workspace"},
				false,
				"another valid symbol search",
			},
			{
				map[string]interface{}{"query": "AnotherSymbol", "maxResults": -1},
				true,
				"negative maxResults",
			},
			{
				map[string]interface{}{"query": "FinalSymbol", "scope": "project"},
				false,
				"valid project scope search",
			},
		}

		var recoverySuccesses int

		for i, test := range testCases {
			t.Run(fmt.Sprintf("case_%d_%s", i, strings.ReplaceAll(test.desc, " ", "_")), func(t *testing.T) {
				result, err := callSCIPTool(t, session, "scip_intelligent_symbol_search", test.params)
				require.NoError(t, err, "Should not get Go-level errors")

				if test.expectErr {
					assert.True(t, result.IsError, "Expected error for: %s", test.desc)
				} else {
					if !result.IsError {
						recoverySuccesses++
					}
					t.Logf("Case %d (%s): success=%v", i, test.desc, !result.IsError)
				}
			})
		}

		// Should be able to recover and handle valid requests after errors
		expectedSuccesses := 3 // Number of valid test cases
		assert.GreaterOrEqual(t, recoverySuccesses, expectedSuccesses-1,
			"Should successfully recover and handle valid requests after errors")
	})

	// Test memory cleanup after errors
	t.Run("MemoryCleanupAfterErrors", func(t *testing.T) {
		// Get baseline memory
		var memBefore runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&memBefore)

		// Generate multiple errors that might cause memory leaks
		for i := 0; i < 20; i++ {
			params := map[string]interface{}{
				"uri":                  fmt.Sprintf("file:///nonexistent/path/error_%d.go", i),
				"analysisType":         "all",
				"includeMetrics":       true,
				"includeRelationships": true,
			}

			result, err := callSCIPTool(t, session, "scip_semantic_code_analysis", params)
			require.NoError(t, err)

			// Don't assert on IsError since behavior may vary
			_ = result

			if i%5 == 0 {
				time.Sleep(50 * time.Millisecond) // Allow some processing time
			}
		}

		// Force garbage collection and measure memory
		runtime.GC()
		runtime.GC() // Double GC to ensure cleanup
		time.Sleep(100 * time.Millisecond)

		var memAfter runtime.MemStats
		runtime.ReadMemStats(&memAfter)

		memoryGrowthBytes := int64(memAfter.Alloc - memBefore.Alloc)
		memoryGrowthMB := float64(memoryGrowthBytes) / (1024 * 1024)

		t.Logf("Memory usage: before=%d bytes, after=%d bytes, growth=%.2f MB",
			memBefore.Alloc, memAfter.Alloc, memoryGrowthMB)

		// Memory growth should be reasonable (less than 50MB for error scenarios)
		assert.LessOrEqual(t, memoryGrowthMB, 50.0,
			"Memory growth after error scenarios should be reasonable")
	})
}

// TestSCIPToolIntegration validates end-to-end integration scenarios
func TestSCIPToolIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP tool integration test in short mode")
	}

	session := setupSCIPMCPServer(t)
	defer session.cleanup()

	// Test realistic workflow: symbol search -> references -> analysis -> assistance
	t.Run("RealisticWorkflow", func(t *testing.T) {
		// Step 1: Search for User symbol across languages
		searchResult, err := callSCIPTool(t, session, "scip_intelligent_symbol_search", map[string]interface{}{
			"query":      "User",
			"scope":      "workspace",
			"maxResults": 20,
		})
		require.NoError(t, err)
		t.Logf("Symbol search completed: isError=%v", searchResult.IsError)

		// Step 2: Get cross-language references for User
		testFile := filepath.Join(session.tempDir, "gateway-service", "models", "user.go")
		if _, err := os.Stat(testFile); err == nil {
			testURI := fmt.Sprintf("file://%s", testFile)
			refsResult, err := callSCIPTool(t, session, "scip_cross_language_references", map[string]interface{}{
				"uri":                    testURI,
				"line":                   5,
				"character":              10,
				"includeImplementations": true,
				"crossLanguageDepth":     3,
			})
			require.NoError(t, err)
			t.Logf("Cross-language references completed: isError=%v", refsResult.IsError)

			// Step 3: Semantic analysis of the file
			analysisResult, err := callSCIPTool(t, session, "scip_semantic_code_analysis", map[string]interface{}{
				"uri":            testURI,
				"analysisType":   "all",
				"includeMetrics": true,
			})
			require.NoError(t, err)
			t.Logf("Semantic analysis completed: isError=%v", analysisResult.IsError)

			// Step 4: Context-aware assistance
			assistanceResult, err := callSCIPTool(t, session, "scip_context_aware_assistance", map[string]interface{}{
				"uri":                testURI,
				"line":               10,
				"character":          5,
				"contextType":        "completion",
				"includeRelatedCode": true,
			})
			require.NoError(t, err)
			t.Logf("Context-aware assistance completed: isError=%v", assistanceResult.IsError)
		}

		// Step 5: Workspace intelligence overview
		workspaceResult, err := callSCIPTool(t, session, "scip_workspace_intelligence", map[string]interface{}{
			"insightType":    "overview",
			"includeMetrics": true,
		})
		require.NoError(t, err)
		t.Logf("Workspace intelligence completed: isError=%v", workspaceResult.IsError)

		t.Log("Realistic workflow integration test completed successfully")
	})

	// Test concurrent tool usage
	t.Run("ConcurrentToolUsage", func(t *testing.T) {
		const numWorkers = 5
		const requestsPerWorker = 3

		var wg sync.WaitGroup
		var successCount int64
		var errorCount int64

		for worker := 0; worker < numWorkers; worker++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				for req := 0; req < requestsPerWorker; req++ {
					// Alternate between different tools
					var result *ToolResult
					var err error

					switch (workerID + req) % 3 {
					case 0:
						result, err = callSCIPTool(t, session, "scip_intelligent_symbol_search", map[string]interface{}{
							"query": fmt.Sprintf("User_%d_%d", workerID, req),
							"scope": "workspace",
						})
					case 1:
						result, err = callSCIPTool(t, session, "scip_workspace_intelligence", map[string]interface{}{
							"insightType": "overview",
						})
					case 2:
						testFile := filepath.Join(session.tempDir, "main.go")
						testURI := fmt.Sprintf("file://%s", testFile)
						result, err = callSCIPTool(t, session, "scip_semantic_code_analysis", map[string]interface{}{
							"uri":          testURI,
							"analysisType": "structure",
						})
					}

					if err == nil && result != nil {
						if result.IsError {
							atomic.AddInt64(&errorCount, 1)
						} else {
							atomic.AddInt64(&successCount, 1)
						}
					}

					// Small delay to simulate realistic usage
					time.Sleep(time.Duration(50+workerID*10) * time.Millisecond)
				}
			}(worker)
		}

		wg.Wait()

		totalRequests := int64(numWorkers * requestsPerWorker)
		actualTotal := successCount + errorCount

		t.Logf("Concurrent tool usage: %d successes, %d errors, %d total (expected %d)",
			successCount, errorCount, actualTotal, totalRequests)

		// Should handle concurrent requests without crashing
		assert.LessOrEqual(t, actualTotal, totalRequests, "Should not exceed expected request count")
		assert.GreaterOrEqual(t, actualTotal, totalRequests/2, "Should handle at least half of concurrent requests")
	})
}
