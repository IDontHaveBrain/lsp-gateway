package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lsp-gateway/internal/indexing"
)

// SCIPEnhancedToolHandler extends the standard ToolHandler with SCIP-powered intelligence
type SCIPEnhancedToolHandler struct {
	*ToolHandler       // Embed standard tool handler
	scipStore          indexing.SCIPStore
	symbolResolver     *indexing.SymbolResolver
	workspaceContext   *WorkspaceContext
	config             *SCIPEnhancedToolConfig
	performanceMonitor *SCIPToolPerformanceMonitor
	degradationManager *GracefulDegradationManager
	requestCache       *SCIPToolCache
	mutex              sync.RWMutex

	// Feature flags for enabling/disabling SCIP enhancements
	enableSCIPEnhancements int32 // atomic bool
	scipAvailable          int32 // atomic bool
}

// SCIPEnhancedToolConfig configures SCIP-enhanced MCP tools
type SCIPEnhancedToolConfig struct {
	// Performance settings
	MaxQueryTime           time.Duration `json:"max_query_time"`
	SymbolSearchTimeout    time.Duration `json:"symbol_search_timeout"`
	CrossLanguageTimeout   time.Duration `json:"cross_language_timeout"`
	ContextAnalysisTimeout time.Duration `json:"context_analysis_timeout"`

	// SCIP enhancement settings
	EnableIntelligentSearch      bool `json:"enable_intelligent_search"`
	EnableCrossLanguageRefs      bool `json:"enable_cross_language_refs"`
	EnableSemanticAnalysis       bool `json:"enable_semantic_analysis"`
	EnableContextAwareness       bool `json:"enable_context_awareness"`
	EnableRefactoringSuggestions bool `json:"enable_refactoring_suggestions"`

	// Cache settings
	EnableResultCaching bool          `json:"enable_result_caching"`
	CacheSize           int           `json:"cache_size"`
	CacheTTL            time.Duration `json:"cache_ttl"`

	// Quality settings
	MinConfidenceThreshold float64 `json:"min_confidence_threshold"`
	MaxResultsPerQuery     int     `json:"max_results_per_query"`
	EnableFuzzyMatching    bool    `json:"enable_fuzzy_matching"`

	// Fallback settings
	EnableGracefulDegradation bool `json:"enable_graceful_degradation"`
	FallbackToStandardTools   bool `json:"fallback_to_standard_tools"`
}

// SCIPToolPerformanceMonitor tracks performance metrics for SCIP-enhanced tools
type SCIPToolPerformanceMonitor struct {
	// Query performance
	totalQueries    int64
	scipQueries     int64
	fallbackQueries int64

	// Timing metrics
	avgQueryTime    time.Duration
	p95QueryTime    time.Duration
	scipAvgTime     time.Duration
	fallbackAvgTime time.Duration

	// Quality metrics
	avgConfidence   float64
	scipSuccessRate float64
	cacheHitRate    float64

	// Error tracking
	scipErrors     int64
	fallbackErrors int64
	timeoutErrors  int64

	mutex     sync.RWMutex
	startTime time.Time
}

// GracefulDegradationManager handles fallback scenarios when SCIP is unavailable
type GracefulDegradationManager struct {
	scipAvailable       bool
	consecutiveFailures int64
	lastFailureTime     time.Time
	degradationLevel    DegradationLevel
	fallbackStrategies  map[string]FallbackStrategy
	mutex               sync.RWMutex
}

// DegradationLevel represents different levels of service degradation
type DegradationLevel int

const (
	DegradationNone DegradationLevel = iota
	DegradationPartial
	DegradationMajor
	DegradationComplete
)

// FallbackStrategy defines how to handle requests when SCIP is degraded
type FallbackStrategy struct {
	UseStandardTools  bool
	UseCache          bool
	SimplifiedResults bool
	ReducedFeatures   []string
	TimeoutReduction  time.Duration
}

// SCIPToolCache provides intelligent caching for SCIP tool results
type SCIPToolCache struct {
	entries   map[string]*SCIPCacheEntry
	lru       *CacheLRUList
	maxSize   int
	ttl       time.Duration
	hitCount  int64
	missCount int64
	mutex     sync.RWMutex
}

// SCIPCacheEntry represents a cached SCIP tool result
type SCIPCacheEntry struct {
	Key             string
	Result          interface{}
	Confidence      float64
	CachedAt        time.Time
	LastAccess      time.Time
	AccessCount     int64
	ExpiresAt       time.Time
	AssociatedFiles []string
	next, prev      *SCIPCacheEntry
}

// CacheLRUList implements LRU eviction for SCIP cache
type CacheLRUList struct {
	head, tail *SCIPCacheEntry
	size       int
}

// NewSCIPEnhancedToolHandler creates a new SCIP-enhanced tool handler
func NewSCIPEnhancedToolHandler(
	baseHandler *ToolHandler,
	scipStore indexing.SCIPStore,
	symbolResolver *indexing.SymbolResolver,
	workspaceContext *WorkspaceContext,
	config *SCIPEnhancedToolConfig,
) *SCIPEnhancedToolHandler {
	if config == nil {
		config = DefaultSCIPEnhancedToolConfig()
	}

	handler := &SCIPEnhancedToolHandler{
		ToolHandler:        baseHandler,
		scipStore:          scipStore,
		symbolResolver:     symbolResolver,
		workspaceContext:   workspaceContext,
		config:             config,
		performanceMonitor: NewSCIPToolPerformanceMonitor(),
		degradationManager: NewGracefulDegradationManager(),
		requestCache:       NewSCIPToolCache(config.CacheSize, config.CacheTTL),
	}

	// Initialize SCIP availability
	handler.checkSCIPAvailability()

	// Register SCIP-enhanced tools
	handler.registerSCIPEnhancedTools()

	return handler
}

// DefaultSCIPEnhancedToolConfig returns optimized default configuration
func DefaultSCIPEnhancedToolConfig() *SCIPEnhancedToolConfig {
	return &SCIPEnhancedToolConfig{
		// Performance targets from requirements
		MaxQueryTime:           100 * time.Millisecond,
		SymbolSearchTimeout:    50 * time.Millisecond,
		CrossLanguageTimeout:   100 * time.Millisecond,
		ContextAnalysisTimeout: 200 * time.Millisecond,

		// Feature enablement
		EnableIntelligentSearch:      true,
		EnableCrossLanguageRefs:      true,
		EnableSemanticAnalysis:       true,
		EnableContextAwareness:       true,
		EnableRefactoringSuggestions: true,

		// Caching
		EnableResultCaching: true,
		CacheSize:           10000,
		CacheTTL:            15 * time.Minute,

		// Quality
		MinConfidenceThreshold: 0.7,
		MaxResultsPerQuery:     50,
		EnableFuzzyMatching:    true,

		// Fallback
		EnableGracefulDegradation: true,
		FallbackToStandardTools:   true,
	}
}

// registerSCIPEnhancedTools registers all SCIP-enhanced MCP tools
func (h *SCIPEnhancedToolHandler) registerSCIPEnhancedTools() {
	// Enhanced symbol search tools
	scipSymbolSearchTitle := "SCIP Intelligent Symbol Search"
	scipSymbolSearchOutputSchema := &map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type": map[string]interface{}{"type": "string"},
						"text": map[string]interface{}{"type": "string"},
						"data": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"symbols": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"name": map[string]interface{}{"type": "string"},
											"kind": map[string]interface{}{"type": "string"},
											"language": map[string]interface{}{"type": "string"},
											"location": map[string]interface{}{"type": "object"},
											"confidence": map[string]interface{}{"type": "number"},
											"semanticSimilarity": map[string]interface{}{"type": "number"},
										},
									},
								},
								"totalResults": map[string]interface{}{"type": "integer"},
								"searchMetrics": map[string]interface{}{"type": "object"},
							},
						},
					},
				},
			},
			"isError": map[string]interface{}{"type": "boolean"},
			"meta": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"timestamp": map[string]interface{}{"type": "string"},
					"duration": map[string]interface{}{"type": "string"},
					"scipAvailable": map[string]interface{}{"type": "boolean"},
					"cacheHit": map[string]interface{}{"type": "boolean"},
				},
			},
		},
	}
	h.Tools["scip_intelligent_symbol_search"] = Tool{
		Name:         "scip_intelligent_symbol_search",
		Title:        &scipSymbolSearchTitle,
		Description:  "Intelligent symbol search with SCIP-powered semantic understanding across languages",
		OutputSchema: scipSymbolSearchOutputSchema,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Symbol search query (supports fuzzy matching and semantic search)",
				},
				"languages": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Optional language filter (e.g., ['go', 'python', 'typescript'])",
				},
				"scope": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"workspace", "project", "file", "local"},
					"default":     "workspace",
					"description": "Search scope",
				},
				"includeSemanticSimilarity": map[string]interface{}{
					"type":        "boolean",
					"default":     true,
					"description": "Include semantically similar symbols",
				},
				"maxResults": map[string]interface{}{
					"type":        "integer",
					"default":     20,
					"maximum":     100,
					"description": "Maximum number of results to return",
				},
			},
			"required": []string{"query"},
		},
	}

	// Cross-language reference tools
	scipCrossLangTitle := "SCIP Cross-Language References"
	scipCrossLangOutputSchema := &map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type": map[string]interface{}{"type": "string"},
						"text": map[string]interface{}{"type": "string"},
						"data": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"crossLanguageReferences": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"uri": map[string]interface{}{"type": "string"},
											"range": map[string]interface{}{"type": "object"},
											"language": map[string]interface{}{"type": "string"},
											"referenceType": map[string]interface{}{"type": "string"},
											"depth": map[string]interface{}{"type": "integer"},
										},
									},
								},
								"implementations": map[string]interface{}{"type": "array"},
								"inheritanceChain": map[string]interface{}{"type": "array"},
								"analysisDepth": map[string]interface{}{"type": "integer"},
							},
						},
					},
				},
			},
			"isError": map[string]interface{}{"type": "boolean"},
			"meta": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"timestamp": map[string]interface{}{"type": "string"},
					"duration": map[string]interface{}{"type": "string"},
					"scipAvailable": map[string]interface{}{"type": "boolean"},
					"cacheHit": map[string]interface{}{"type": "boolean"},
				},
			},
		},
	}
	h.Tools["scip_cross_language_references"] = Tool{
		Name:         "scip_cross_language_references",
		Title:        &scipCrossLangTitle,
		Description:  "Find cross-language references and dependencies using SCIP symbol relationships",
		OutputSchema: scipCrossLangOutputSchema,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"uri": map[string]interface{}{
					"type":        "string",
					"description": "File URI",
				},
				"line": map[string]interface{}{
					"type":        "integer",
					"description": "Line number (0-based)",
				},
				"character": map[string]interface{}{
					"type":        "integer",
					"description": "Character position (0-based)",
				},
				"includeImplementations": map[string]interface{}{
					"type":        "boolean",
					"default":     true,
					"description": "Include implementations and overrides",
				},
				"includeInheritance": map[string]interface{}{
					"type":        "boolean",
					"default":     true,
					"description": "Include inheritance relationships",
				},
				"crossLanguageDepth": map[string]interface{}{
					"type":        "integer",
					"default":     3,
					"maximum":     10,
					"description": "Maximum depth for cross-language traversal",
				},
			},
			"required": []string{"uri", "line", "character"},
		},
	}

	// Semantic code analysis tools
	scipSemanticTitle := "SCIP Semantic Code Analysis"
	scipSemanticOutputSchema := &map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type": map[string]interface{}{"type": "string"},
						"text": map[string]interface{}{"type": "string"},
						"data": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"structure": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"symbols": map[string]interface{}{"type": "array"},
										"hierarchy": map[string]interface{}{"type": "object"},
										"complexity": map[string]interface{}{"type": "object"},
									},
								},
								"dependencies": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{"type": "object"},
								},
								"metrics": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"linesOfCode": map[string]interface{}{"type": "integer"},
										"cyclomaticComplexity": map[string]interface{}{"type": "number"},
										"maintainabilityIndex": map[string]interface{}{"type": "number"},
									},
								},
								"relationships": map[string]interface{}{"type": "array"},
							},
						},
					},
				},
			},
			"isError": map[string]interface{}{"type": "boolean"},
			"meta": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"timestamp": map[string]interface{}{"type": "string"},
					"duration": map[string]interface{}{"type": "string"},
					"scipAvailable": map[string]interface{}{"type": "boolean"},
					"cacheHit": map[string]interface{}{"type": "boolean"},
				},
			},
		},
	}
	h.Tools["scip_semantic_code_analysis"] = Tool{
		Name:         "scip_semantic_code_analysis",
		Title:        &scipSemanticTitle,
		Description:  "Deep semantic code analysis using SCIP symbol understanding",
		OutputSchema: scipSemanticOutputSchema,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"uri": map[string]interface{}{
					"type":        "string",
					"description": "File URI for analysis",
				},
				"analysisType": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"structure", "dependencies", "complexity", "relationships", "all"},
					"default":     "all",
					"description": "Type of semantic analysis to perform",
				},
				"includeMetrics": map[string]interface{}{
					"type":        "boolean",
					"default":     true,
					"description": "Include code quality metrics",
				},
				"includeRelationships": map[string]interface{}{
					"type":        "boolean",
					"default":     true,
					"description": "Include symbol relationships",
				},
			},
			"required": []string{"uri"},
		},
	}

	// Context-aware assistance tools
	scipContextTitle := "SCIP Context-Aware Assistance"
	scipContextOutputSchema := &map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type": map[string]interface{}{"type": "string"},
						"text": map[string]interface{}{"type": "string"},
						"data": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"contextType": map[string]interface{}{"type": "string"},
								"suggestions": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"text": map[string]interface{}{"type": "string"},
											"kind": map[string]interface{}{"type": "string"},
											"confidence": map[string]interface{}{"type": "number"},
											"detail": map[string]interface{}{"type": "string"},
										},
									},
								},
								"relatedCode": map[string]interface{}{"type": "array"},
								"documentation": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"summary": map[string]interface{}{"type": "string"},
										"details": map[string]interface{}{"type": "string"},
										"examples": map[string]interface{}{"type": "array"},
									},
								},
								"contextAnalysis": map[string]interface{}{"type": "object"},
							},
						},
					},
				},
			},
			"isError": map[string]interface{}{"type": "boolean"},
			"meta": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"timestamp": map[string]interface{}{"type": "string"},
					"duration": map[string]interface{}{"type": "string"},
					"scipAvailable": map[string]interface{}{"type": "boolean"},
					"cacheHit": map[string]interface{}{"type": "boolean"},
				},
			},
		},
	}
	h.Tools["scip_context_aware_assistance"] = Tool{
		Name:         "scip_context_aware_assistance",
		Title:        &scipContextTitle,
		Description:  "Context-aware AI assistance with SCIP-powered workspace understanding",
		OutputSchema: scipContextOutputSchema,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"uri": map[string]interface{}{
					"type":        "string",
					"description": "Current file URI",
				},
				"line": map[string]interface{}{
					"type":        "integer",
					"description": "Current line number (0-based)",
				},
				"character": map[string]interface{}{
					"type":        "integer",
					"description": "Current character position (0-based)",
				},
				"contextType": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"completion", "documentation", "navigation", "refactoring", "debugging"},
					"default":     "completion",
					"description": "Type of assistance context",
				},
				"includeRelatedCode": map[string]interface{}{
					"type":        "boolean",
					"default":     true,
					"description": "Include related code snippets",
				},
				"includeDocumentation": map[string]interface{}{
					"type":        "boolean",
					"default":     true,
					"description": "Include symbol documentation",
				},
			},
			"required": []string{"uri", "line", "character"},
		},
	}

	// Workspace intelligence tools
	scipWorkspaceTitle := "SCIP Workspace Intelligence"
	scipWorkspaceOutputSchema := &map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type": map[string]interface{}{"type": "string"},
						"text": map[string]interface{}{"type": "string"},
						"data": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"overview": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"totalFiles": map[string]interface{}{"type": "integer"},
										"totalSymbols": map[string]interface{}{"type": "integer"},
										"languages": map[string]interface{}{"type": "array"},
										"projectHealth": map[string]interface{}{"type": "number"},
									},
								},
								"hotspots": map[string]interface{}{"type": "array"},
								"dependencies": map[string]interface{}{"type": "object"},
								"unusedCode": map[string]interface{}{"type": "array"},
								"complexityMetrics": map[string]interface{}{"type": "object"},
								"recommendations": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"category": map[string]interface{}{"type": "string"},
											"description": map[string]interface{}{"type": "string"},
											"priority": map[string]interface{}{"type": "string"},
											"impact": map[string]interface{}{"type": "string"},
										},
									},
								},
							},
						},
					},
				},
			},
			"isError": map[string]interface{}{"type": "boolean"},
			"meta": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"timestamp": map[string]interface{}{"type": "string"},
					"duration": map[string]interface{}{"type": "string"},
					"scipAvailable": map[string]interface{}{"type": "boolean"},
					"cacheHit": map[string]interface{}{"type": "boolean"},
				},
			},
		},
	}
	h.Tools["scip_workspace_intelligence"] = Tool{
		Name:         "scip_workspace_intelligence",
		Title:        &scipWorkspaceTitle,
		Description:  "Comprehensive workspace intelligence and insights using SCIP indexing",
		OutputSchema: scipWorkspaceOutputSchema,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"insightType": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"overview", "hotspots", "dependencies", "unused", "complexity", "all"},
					"default":     "overview",
					"description": "Type of workspace insight to generate",
				},
				"includeMetrics": map[string]interface{}{
					"type":        "boolean",
					"default":     true,
					"description": "Include quantitative metrics",
				},
				"includeRecommendations": map[string]interface{}{
					"type":        "boolean",
					"default":     true,
					"description": "Include improvement recommendations",
				},
				"languageFilter": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Optional language filter",
				},
			},
			"required": []string{},
		},
	}

	// Refactoring suggestions tools
	scipRefactoringTitle := "SCIP Refactoring Suggestions"
	scipRefactoringOutputSchema := &map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type": map[string]interface{}{"type": "string"},
						"text": map[string]interface{}{"type": "string"},
						"data": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"suggestions": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"type": map[string]interface{}{"type": "string"},
											"title": map[string]interface{}{"type": "string"},
											"description": map[string]interface{}{"type": "string"},
											"confidence": map[string]interface{}{"type": "number"},
											"range": map[string]interface{}{"type": "object"},
											"newCode": map[string]interface{}{"type": "string"},
											"impact": map[string]interface{}{"type": "object"},
										},
									},
								},
								"impactAnalysis": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"affectedFiles": map[string]interface{}{"type": "array"},
										"breakingChanges": map[string]interface{}{"type": "boolean"},
										"riskLevel": map[string]interface{}{"type": "string"},
										"estimatedEffort": map[string]interface{}{"type": "string"},
									},
								},
								"qualityMetrics": map[string]interface{}{"type": "object"},
							},
						},
					},
				},
			},
			"isError": map[string]interface{}{"type": "boolean"},
			"meta": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"timestamp": map[string]interface{}{"type": "string"},
					"duration": map[string]interface{}{"type": "string"},
					"scipAvailable": map[string]interface{}{"type": "boolean"},
					"cacheHit": map[string]interface{}{"type": "boolean"},
				},
			},
		},
	}
	h.Tools["scip_refactoring_suggestions"] = Tool{
		Name:         "scip_refactoring_suggestions",
		Title:        &scipRefactoringTitle,
		Description:  "Intelligent refactoring suggestions based on SCIP symbol analysis",
		OutputSchema: scipRefactoringOutputSchema,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"uri": map[string]interface{}{
					"type":        "string",
					"description": "File URI for refactoring analysis",
				},
				"selectionStart": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"line":      map[string]interface{}{"type": "integer"},
						"character": map[string]interface{}{"type": "integer"},
					},
					"description": "Start of selection (optional for whole file)",
				},
				"selectionEnd": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"line":      map[string]interface{}{"type": "integer"},
						"character": map[string]interface{}{"type": "integer"},
					},
					"description": "End of selection (optional for whole file)",
				},
				"refactoringTypes": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
						"enum": []string{"extract_method", "rename", "move", "inline", "optimize", "modernize", "all"},
					},
					"default":     []string{"all"},
					"description": "Types of refactoring to suggest",
				},
				"includeImpactAnalysis": map[string]interface{}{
					"type":        "boolean",
					"default":     true,
					"description": "Include refactoring impact analysis",
				},
			},
			"required": []string{"uri"},
		},
	}
}

// CallTool handles both standard and SCIP-enhanced tool calls
func (h *SCIPEnhancedToolHandler) CallTool(ctx context.Context, call ToolCall) (*ToolResult, error) {
	startTime := time.Now()
	atomic.AddInt64(&h.performanceMonitor.totalQueries, 1)

	// Check if this is a SCIP-enhanced tool
	if strings.HasPrefix(call.Name, "scip_") {
		return h.callSCIPEnhancedTool(ctx, call, startTime)
	}

	// Fallback to standard tool handler
	return h.ToolHandler.CallTool(ctx, call)
}

// callSCIPEnhancedTool handles SCIP-enhanced tool calls with performance monitoring
func (h *SCIPEnhancedToolHandler) callSCIPEnhancedTool(ctx context.Context, call ToolCall, startTime time.Time) (*ToolResult, error) {
	// Check if SCIP enhancements are enabled and available
	if !h.isSCIPAvailable() {
		if h.config.FallbackToStandardTools {
			return h.handleSCIPFallback(ctx, call, "SCIP unavailable")
		}
		return h.createErrorResult("SCIP enhancements are not available", MCPErrorUnsupportedFeature)
	}

	// Check cache first if enabled
	if h.config.EnableResultCaching {
		if cached := h.requestCache.Get(h.generateCacheKey(call)); cached != nil {
			return h.createCachedResult(cached, time.Since(startTime))
		}
	}

	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, h.config.MaxQueryTime)
	defer cancel()

	// Route to specific SCIP tool handler
	var result *ToolResult
	var err error

	switch call.Name {
	case "scip_intelligent_symbol_search":
		result, err = h.handleSCIPIntelligentSymbolSearch(ctx, call.Arguments)
	case "scip_cross_language_references":
		result, err = h.handleSCIPCrossLanguageReferences(ctx, call.Arguments)
	case "scip_semantic_code_analysis":
		result, err = h.handleSCIPSemanticCodeAnalysis(ctx, call.Arguments)
	case "scip_context_aware_assistance":
		result, err = h.handleSCIPContextAwareAssistance(ctx, call.Arguments)
	case "scip_workspace_intelligence":
		result, err = h.handleSCIPWorkspaceIntelligence(ctx, call.Arguments)
	case "scip_refactoring_suggestions":
		result, err = h.handleSCIPRefactoringSuggestions(ctx, call.Arguments)
	default:
		return h.createErrorResult(fmt.Sprintf("Unknown SCIP tool: %s", call.Name), MCPErrorMethodNotFound)
	}

	// Handle errors and fallbacks
	if err != nil {
		h.degradationManager.recordFailure()
		if h.config.FallbackToStandardTools {
			return h.handleSCIPFallback(ctx, call, fmt.Sprintf("SCIP error: %v", err))
		}
		return h.createErrorResult(fmt.Sprintf("SCIP tool error: %v", err), MCPErrorInternalError)
	}

	// Record success metrics
	queryTime := time.Since(startTime)
	atomic.AddInt64(&h.performanceMonitor.scipQueries, 1)
	h.performanceMonitor.recordSuccess(queryTime)

	// Cache successful result if enabled
	if h.config.EnableResultCaching && result != nil && !result.IsError {
		h.requestCache.Set(h.generateCacheKey(call), result, h.extractConfidence(result))
	}

	// Add performance metadata
	if result.Meta == nil {
		result.Meta = &ResponseMetadata{}
	}
	result.Meta.Duration = queryTime.String()
	result.Meta.RequestInfo = map[string]interface{}{
		"scip_enhanced": true,
		"query_time":    queryTime.Milliseconds(),
		"tool_name":     call.Name,
	}

	return result, nil
}

// isSCIPAvailable checks if SCIP enhancements are available
func (h *SCIPEnhancedToolHandler) isSCIPAvailable() bool {
	return atomic.LoadInt32(&h.enableSCIPEnhancements) == 1 &&
		atomic.LoadInt32(&h.scipAvailable) == 1
}

// checkSCIPAvailability verifies SCIP store and resolver availability
func (h *SCIPEnhancedToolHandler) checkSCIPAvailability() {
	available := h.scipStore != nil && h.symbolResolver != nil

	if available {
		// Test SCIP store with a simple query
		// Note: Query method has built-in 5-second timeout for concurrency control
		testResult := h.scipStore.Query("test", map[string]interface{}{})
		available = testResult.Error == "" || strings.Contains(testResult.Error, "degraded mode")
	}

	if available {
		atomic.StoreInt32(&h.scipAvailable, 1)
		atomic.StoreInt32(&h.enableSCIPEnhancements, 1)
	} else {
		atomic.StoreInt32(&h.scipAvailable, 0)
		atomic.StoreInt32(&h.enableSCIPEnhancements, 0)
	}
}

// Utility methods for result creation and caching

// createErrorResult creates a standardized error result
func (h *SCIPEnhancedToolHandler) createErrorResult(message string, code MCPErrorCode) (*ToolResult, error) {
	return &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: message,
		}},
		IsError: true,
		Error: &StructuredError{
			Code:      code,
			Message:   message,
			Retryable: code == MCPErrorTimeout || code == MCPErrorConnectionFailed,
		},
	}, nil
}

// createCachedResult creates a result from cached data
func (h *SCIPEnhancedToolHandler) createCachedResult(cached interface{}, queryTime time.Duration) (*ToolResult, error) {
	if result, ok := cached.(*ToolResult); ok {
		// Update metadata for cache hit
		if result.Meta == nil {
			result.Meta = &ResponseMetadata{}
		}
		result.Meta.CacheHit = true
		result.Meta.Duration = queryTime.String()
		return result, nil
	}
	return h.createErrorResult("Invalid cached result", MCPErrorInternalError)
}

// generateCacheKey creates a unique cache key for the tool call
func (h *SCIPEnhancedToolHandler) generateCacheKey(call ToolCall) string {
	argsJSON, _ := json.Marshal(call.Arguments)
	return fmt.Sprintf("%s:%s", call.Name, string(argsJSON))
}

// extractConfidence extracts confidence level from tool result
func (h *SCIPEnhancedToolHandler) extractConfidence(result *ToolResult) float64 {
	if result.Meta != nil && result.Meta.RequestInfo != nil {
		if confidence, ok := result.Meta.RequestInfo["confidence"].(float64); ok {
			return confidence
		}
	}
	return 0.8 // Default confidence
}

// handleSCIPFallback handles fallback to standard tools when SCIP fails
func (h *SCIPEnhancedToolHandler) handleSCIPFallback(ctx context.Context, call ToolCall, reason string) (*ToolResult, error) {
	atomic.AddInt64(&h.performanceMonitor.fallbackQueries, 1)

	// Map SCIP tools to standard equivalents
	standardTool := h.mapSCIPToStandardTool(call.Name)
	if standardTool == "" {
		return h.createErrorResult(fmt.Sprintf("No fallback available for %s: %s", call.Name, reason), MCPErrorUnsupportedFeature)
	}

	// Create standard tool call
	standardCall := ToolCall{
		Name:      standardTool,
		Arguments: h.adaptArgumentsForStandardTool(call.Arguments, standardTool),
	}

	// Call standard tool
	result, err := h.ToolHandler.CallTool(ctx, standardCall)
	if err != nil {
		return nil, err
	}

	// Add fallback metadata
	if result.Meta == nil {
		result.Meta = &ResponseMetadata{}
	}
	result.Meta.RequestInfo = map[string]interface{}{
		"fallback_used":   true,
		"fallback_reason": reason,
		"original_tool":   call.Name,
		"standard_tool":   standardTool,
	}

	return result, nil
}

// mapSCIPToStandardTool maps SCIP tools to their standard equivalents
func (h *SCIPEnhancedToolHandler) mapSCIPToStandardTool(scipTool string) string {
	mapping := map[string]string{
		"scip_intelligent_symbol_search": "search_workspace_symbols",
		"scip_cross_language_references": "find_references",
		"scip_semantic_code_analysis":    "get_document_symbols",
		"scip_context_aware_assistance":  "get_hover_info",
		"scip_workspace_intelligence":    "search_workspace_symbols",
		"scip_refactoring_suggestions":   "get_document_symbols",
	}
	return mapping[scipTool]
}

// adaptArgumentsForStandardTool adapts SCIP tool arguments for standard tools
func (h *SCIPEnhancedToolHandler) adaptArgumentsForStandardTool(args map[string]interface{}, standardTool string) map[string]interface{} {
	adapted := make(map[string]interface{})

	// Copy common arguments
	for _, key := range []string{"uri", "line", "character", "query"} {
		if value, exists := args[key]; exists {
			adapted[key] = value
		}
	}

	// Add tool-specific adaptations
	switch standardTool {
	case "find_references":
		adapted["includeDeclaration"] = true
	case "search_workspace_symbols":
		if query, exists := args["query"]; exists {
			adapted["query"] = query
		} else {
			adapted["query"] = ""
		}
	}

	return adapted
}

// Performance monitor methods

// NewSCIPToolPerformanceMonitor creates a new performance monitor
func NewSCIPToolPerformanceMonitor() *SCIPToolPerformanceMonitor {
	return &SCIPToolPerformanceMonitor{
		startTime: time.Now(),
	}
}

// recordSuccess records a successful SCIP query
func (pm *SCIPToolPerformanceMonitor) recordSuccess(duration time.Duration) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Update timing metrics using exponential moving average
	if pm.avgQueryTime == 0 {
		pm.avgQueryTime = duration
	} else {
		alpha := 0.1
		pm.avgQueryTime = time.Duration(float64(pm.avgQueryTime)*(1-alpha) + float64(duration)*alpha)
	}

	// Update P95 (simplified)
	if duration > pm.p95QueryTime {
		pm.p95QueryTime = duration
	}
}

// GetStats returns current performance statistics
func (pm *SCIPToolPerformanceMonitor) GetStats() map[string]interface{} {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	totalQueries := atomic.LoadInt64(&pm.totalQueries)
	scipQueries := atomic.LoadInt64(&pm.scipQueries)
	fallbackQueries := atomic.LoadInt64(&pm.fallbackQueries)

	var scipSuccessRate float64
	if scipQueries > 0 {
		scipSuccessRate = float64(scipQueries-atomic.LoadInt64(&pm.scipErrors)) / float64(scipQueries)
	}

	return map[string]interface{}{
		"total_queries":     totalQueries,
		"scip_queries":      scipQueries,
		"fallback_queries":  fallbackQueries,
		"avg_query_time_ms": pm.avgQueryTime.Milliseconds(),
		"p95_query_time_ms": pm.p95QueryTime.Milliseconds(),
		"scip_success_rate": scipSuccessRate,
		"uptime_hours":      time.Since(pm.startTime).Hours(),
	}
}

// Graceful degradation manager methods

// NewGracefulDegradationManager creates a new degradation manager
func NewGracefulDegradationManager() *GracefulDegradationManager {
	return &GracefulDegradationManager{
		scipAvailable:      true,
		degradationLevel:   DegradationNone,
		fallbackStrategies: make(map[string]FallbackStrategy),
	}
}

// recordFailure records a SCIP failure and adjusts degradation level
func (gm *GracefulDegradationManager) recordFailure() {
	gm.mutex.Lock()
	defer gm.mutex.Unlock()

	gm.consecutiveFailures++
	gm.lastFailureTime = time.Now()

	// Adjust degradation level based on failure count
	switch {
	case gm.consecutiveFailures >= 10:
		gm.degradationLevel = DegradationComplete
		gm.scipAvailable = false
	case gm.consecutiveFailures >= 5:
		gm.degradationLevel = DegradationMajor
	case gm.consecutiveFailures >= 2:
		gm.degradationLevel = DegradationPartial
	}
}

// SCIP tool cache implementation

// NewSCIPToolCache creates a new SCIP tool cache
func NewSCIPToolCache(maxSize int, ttl time.Duration) *SCIPToolCache {
	return &SCIPToolCache{
		entries: make(map[string]*SCIPCacheEntry),
		lru:     &CacheLRUList{},
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Get retrieves a cached result
func (c *SCIPToolCache) Get(key string) interface{} {
	c.mutex.RLock()
	entry, exists := c.entries[key]
	c.mutex.RUnlock()

	if !exists {
		atomic.AddInt64(&c.missCount, 1)
		return nil
	}

	// Check expiration
	if time.Now().After(entry.ExpiresAt) {
		c.mutex.Lock()
		c.evict(key)
		c.mutex.Unlock()
		atomic.AddInt64(&c.missCount, 1)
		return nil
	}

	// Update access and move to front
	c.mutex.Lock()
	entry.LastAccess = time.Now()
	entry.AccessCount++
	c.lru.moveToFront(entry)
	c.mutex.Unlock()

	atomic.AddInt64(&c.hitCount, 1)
	return entry.Result
}

// Set stores a result in the cache
func (c *SCIPToolCache) Set(key string, result interface{}, confidence float64) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Remove existing entry
	if existing, exists := c.entries[key]; exists {
		c.lru.remove(existing)
	}

	// Create new entry
	entry := &SCIPCacheEntry{
		Key:         key,
		Result:      result,
		Confidence:  confidence,
		CachedAt:    time.Now(),
		LastAccess:  time.Now(),
		ExpiresAt:   time.Now().Add(c.ttl),
		AccessCount: 1,
	}

	// Add to cache
	c.entries[key] = entry
	c.lru.addToFront(entry)

	// Evict if necessary
	for len(c.entries) > c.maxSize {
		oldest := c.lru.removeLast()
		if oldest != nil {
			delete(c.entries, oldest.Key)
		}
	}
}

// evict removes an entry from the cache
func (c *SCIPToolCache) evict(key string) {
	if entry, exists := c.entries[key]; exists {
		c.lru.remove(entry)
		delete(c.entries, key)
	}
}

// LRU list implementation

// addToFront adds entry to front of LRU list
func (lru *CacheLRUList) addToFront(entry *SCIPCacheEntry) {
	if lru.head == nil {
		lru.head = entry
		lru.tail = entry
	} else {
		entry.next = lru.head
		lru.head.prev = entry
		lru.head = entry
	}
	lru.size++
}

// remove removes entry from LRU list
func (lru *CacheLRUList) remove(entry *SCIPCacheEntry) {
	if entry.prev != nil {
		entry.prev.next = entry.next
	} else {
		lru.head = entry.next
	}

	if entry.next != nil {
		entry.next.prev = entry.prev
	} else {
		lru.tail = entry.prev
	}

	entry.prev = nil
	entry.next = nil
	lru.size--
}

// moveToFront moves entry to front of LRU list
func (lru *CacheLRUList) moveToFront(entry *SCIPCacheEntry) {
	lru.remove(entry)
	lru.addToFront(entry)
}

// removeLast removes and returns last entry
func (lru *CacheLRUList) removeLast() *SCIPCacheEntry {
	if lru.tail == nil {
		return nil
	}
	last := lru.tail
	lru.remove(last)
	return last
}

// Public interface methods

// EnableSCIPEnhancements enables SCIP-powered features
func (h *SCIPEnhancedToolHandler) EnableSCIPEnhancements() {
	atomic.StoreInt32(&h.enableSCIPEnhancements, 1)
}

// DisableSCIPEnhancements disables SCIP-powered features
func (h *SCIPEnhancedToolHandler) DisableSCIPEnhancements() {
	atomic.StoreInt32(&h.enableSCIPEnhancements, 0)
}

// GetPerformanceStats returns comprehensive performance statistics
func (h *SCIPEnhancedToolHandler) GetPerformanceStats() map[string]interface{} {
	stats := h.performanceMonitor.GetStats()

	// Add cache statistics
	hitCount := atomic.LoadInt64(&h.requestCache.hitCount)
	missCount := atomic.LoadInt64(&h.requestCache.missCount)
	total := hitCount + missCount
	var hitRate float64
	if total > 0 {
		hitRate = float64(hitCount) / float64(total)
	}

	stats["cache_hit_rate"] = hitRate
	stats["cache_size"] = len(h.requestCache.entries)
	stats["scip_available"] = h.isSCIPAvailable()
	stats["degradation_level"] = h.degradationManager.degradationLevel

	return stats
}

// UpdateConfiguration updates the SCIP tool configuration
func (h *SCIPEnhancedToolHandler) UpdateConfiguration(config *SCIPEnhancedToolConfig) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.config = config
}

// GetConfiguration returns the current configuration
func (h *SCIPEnhancedToolHandler) GetConfiguration() *SCIPEnhancedToolConfig {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	configCopy := *h.config
	return &configCopy
}
