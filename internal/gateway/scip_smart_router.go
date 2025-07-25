package gateway

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/indexing"
	"lsp-gateway/internal/transport"
	"lsp-gateway/mcp"
)

// SCIPRoutingStrategyType defines SCIP-aware routing strategies
type SCIPRoutingStrategyType string

const (
	SCIPCacheFirst    SCIPRoutingStrategyType = "scip_cache_first"    // Try SCIP cache first, fallback to LSP
	SCIPLSPFirst      SCIPRoutingStrategyType = "scip_lsp_first"      // Try LSP first, cache with SCIP
	SCIPHybrid        SCIPRoutingStrategyType = "scip_hybrid"         // Intelligent decision based on confidence
	SCIPCacheOnly     SCIPRoutingStrategyType = "scip_cache_only"     // SCIP cache only, no LSP fallback
	SCIPDisabled      SCIPRoutingStrategyType = "scip_disabled"       // Disable SCIP routing, use traditional
)

// ConfidenceLevel represents confidence thresholds for routing decisions
type ConfidenceLevel float64

const (
	ConfidenceMinimum = ConfidenceLevel(0.6)  // Minimum viable confidence
	ConfidenceMedium  = ConfidenceLevel(0.8)  // Medium confidence threshold
	ConfidenceHigh    = ConfidenceLevel(0.95) // High confidence threshold
)

// SCIPSmartRouter extends SmartRouter with SCIP-aware intelligent routing capabilities
type SCIPSmartRouter interface {
	SmartRouter

	// SCIP-aware routing methods
	RouteWithSCIPAwareness(request *LSPRequest) (*SCIPRoutingDecision, error)
	DecideRoutingSource(method string, params interface{}) (*RoutingSourceDecision, error)
	
	// Confidence and scoring
	CalculateConfidence(method string, params interface{}, queryResult *indexing.SCIPQueryResult) ConfidenceLevel
	GetMethodRoutingHint(method string) SCIPRoutingStrategyType
	
	// Performance optimization
	PrewarmSCIPCache(ctx context.Context, files []string) error
	OptimizeRoutingStrategy(method string, performanceData *PerformanceAnalysis) error
	
	// SCIP integration management
	SetSCIPStore(store indexing.SCIPStore) error
	GetSCIPStats() *SCIPRoutingStats
	
	// Strategy management
	SetSCIPRoutingStrategy(method string, strategy SCIPRoutingStrategyType)
	GetSCIPRoutingStrategy(method string) SCIPRoutingStrategyType
}

// SCIPRoutingDecision represents a comprehensive routing decision with SCIP context
type SCIPRoutingDecision struct {
	*RoutingDecision
	
	// SCIP-specific decision context
	SCIPQueried        bool                    `json:"scip_queried"`
	SCIPResult         *indexing.SCIPQueryResult `json:"scip_result,omitempty"`
	SCIPConfidence     ConfidenceLevel         `json:"scip_confidence"`
	UseSCIP           bool                    `json:"use_scip"`
	FallbackToLSP     bool                    `json:"fallback_to_lsp"`
	
	// Decision reasoning
	DecisionReason    string                  `json:"decision_reason"`
	Strategy          SCIPRoutingStrategyType `json:"strategy"`
	Confidence        ConfidenceLevel         `json:"confidence"`
	
	// Performance prediction
	ExpectedLatency   time.Duration           `json:"expected_latency"`
	CacheHitProbability float64              `json:"cache_hit_probability"`
	
	// Metadata
	DecisionTime      time.Time               `json:"decision_time"`
	DecisionLatency   time.Duration           `json:"decision_latency"`
	QueryAnalysis     *QueryAnalysis          `json:"query_analysis,omitempty"`
}

// RoutingSourceDecision represents the decision of which source to use for a query
type RoutingSourceDecision struct {
	Source            RoutingSource           `json:"source"`
	Confidence        ConfidenceLevel         `json:"confidence"`
	Reason            string                  `json:"reason"`
	Strategy          SCIPRoutingStrategyType `json:"strategy"`
	FallbackSource    *RoutingSource          `json:"fallback_source,omitempty"`
	EstimatedLatency  time.Duration           `json:"estimated_latency"`
	CacheKey          string                  `json:"cache_key,omitempty"`
}

// RoutingSource defines where to route the request
type RoutingSource int

const (
	SourceSCIPCache RoutingSource = iota + 1 // Route to SCIP cache
	SourceLSPServer                          // Route to LSP server
	SourceHybrid                             // Use both sources and merge
)

func (rs RoutingSource) String() string {
	switch rs {
	case SourceSCIPCache:
		return "SCIP_Cache"
	case SourceLSPServer:
		return "LSP_Server"
	case SourceHybrid:
		return "Hybrid"
	default:
		return "Unknown"
	}
}

// QueryAnalysis provides detailed analysis of LSP query for routing decisions
type QueryAnalysis struct {
	Method           string                 `json:"method"`
	Complexity       QueryComplexity        `json:"complexity"`
	StaticAnalysisHint bool                 `json:"static_analysis_hint"`
	DynamicAnalysisHint bool                `json:"dynamic_analysis_hint"`
	FileURI          string                 `json:"file_uri,omitempty"`
	Language         string                 `json:"language,omitempty"`
	ProjectContext   string                 `json:"project_context,omitempty"`
	
	// Method-specific analysis
	IsSymbolQuery    bool                   `json:"is_symbol_query"`
	IsWorkspaceQuery bool                   `json:"is_workspace_query"`
	RequiresRuntime  bool                   `json:"requires_runtime"`
	CacheEligible    bool                   `json:"cache_eligible"`
	
	// Performance hints
	ExpectedDataSize int64                  `json:"expected_data_size"`
	ComputationCost  ComputationCost        `json:"computation_cost"`
	
	// Context metadata
	AnalysisTime     time.Time              `json:"analysis_time"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// QueryComplexity defines the complexity level of a query
type QueryComplexity int

const (
	ComplexitySimple QueryComplexity = iota + 1 // Simple symbol lookup
	ComplexityModerate                           // Cross-file analysis
	ComplexityComplex                            // Workspace-wide analysis
	ComplexityDynamic                            // Runtime-dependent analysis
)

func (qc QueryComplexity) String() string {
	switch qc {
	case ComplexitySimple:
		return "Simple"
	case ComplexityModerate:
		return "Moderate"
	case ComplexityComplex:
		return "Complex"
	case ComplexityDynamic:
		return "Dynamic"
	default:
		return "Unknown"
	}
}

// ComputationCost defines the computational cost of a query
type ComputationCost int

const (
	CostLow ComputationCost = iota + 1  // <5ms expected
	CostMedium                          // 5-50ms expected
	CostHigh                            // 50-200ms expected
	CostVeryHigh                        // >200ms expected
)

func (cc ComputationCost) String() string {
	switch cc {
	case CostLow:
		return "Low"
	case CostMedium:
		return "Medium"
	case CostHigh:
		return "High"
	case CostVeryHigh:
		return "VeryHigh"
	default:
		return "Unknown"
	}
}

// ConfidenceScorer provides intelligent confidence scoring for routing decisions
type ConfidenceScorer struct {
	config                *SCIPScoringConfig
	methodConfidenceMap   map[string]ConfidenceLevel
	performanceHistory    *PerformanceHistory
	adaptiveLearning      bool
	mutex                 sync.RWMutex
}

// SCIPScoringConfig configures the confidence scoring algorithm
type SCIPScoringConfig struct {
	BaseConfidence       ConfidenceLevel        `json:"base_confidence"`
	CacheHitBonus        float64                `json:"cache_hit_bonus"`
	RecentAccessBonus    float64                `json:"recent_access_bonus"`
	FileStalenessPenalty float64                `json:"file_staleness_penalty"`
	ComplexityPenalty    map[QueryComplexity]float64 `json:"complexity_penalty"`
	MethodBonus          map[string]float64     `json:"method_bonus"`
	AdaptiveLearning     bool                   `json:"adaptive_learning"`
	LearningRate         float64                `json:"learning_rate"`
}

// PerformanceHistory tracks historical performance for adaptive learning
type PerformanceHistory struct {
	methodPerformance map[string]*MethodPerformance
	mutex             sync.RWMutex
}

// MethodPerformance tracks performance metrics for specific LSP methods
type MethodPerformance struct {
	SCIPHitRate          float64       `json:"scip_hit_rate"`
	SCIPAverageLatency   time.Duration `json:"scip_average_latency"`
	LSPAverageLatency    time.Duration `json:"lsp_average_latency"`
	SCIPAccuracy         float64       `json:"scip_accuracy"`
	TotalQueries         int64         `json:"total_queries"`
	LastUpdated          time.Time     `json:"last_updated"`
}

// SCIPRoutingStats provides comprehensive statistics for SCIP routing
type SCIPRoutingStats struct {
	TotalQueries         int64                              `json:"total_queries"`
	SCIPQueries          int64                              `json:"scip_queries"`
	LSPQueries           int64                              `json:"lsp_queries"`
	HybridQueries        int64                              `json:"hybrid_queries"`
	
	// Hit rates and accuracy
	SCIPHitRate          float64                            `json:"scip_hit_rate"`
	CacheHitRate         float64                            `json:"cache_hit_rate"`
	AccuracyRate         float64                            `json:"accuracy_rate"`
	
	// Performance metrics
	AverageSCIPLatency   time.Duration                      `json:"average_scip_latency"`
	AverageLSPLatency    time.Duration                      `json:"average_lsp_latency"`
	AverageDecisionTime  time.Duration                      `json:"average_decision_time"`
	
	// Strategy effectiveness
	StrategyStats        map[SCIPRoutingStrategyType]*StrategyStats `json:"strategy_stats"`
	MethodStats          map[string]*MethodRoutingStats              `json:"method_stats"`
	
	// Error tracking
	ErrorRate            float64                            `json:"error_rate"`
	FallbackRate         float64                            `json:"fallback_rate"`
	
	// Timing
	StartTime            time.Time                          `json:"start_time"`
	LastUpdated          time.Time                          `json:"last_updated"`
}

// StrategyStats tracks performance for specific routing strategies
type StrategyStats struct {
	UseCount       int64         `json:"use_count"`
	SuccessRate    float64       `json:"success_rate"`
	AverageLatency time.Duration `json:"average_latency"`
	ErrorCount     int64         `json:"error_count"`
}

// MethodRoutingStats tracks routing statistics for specific LSP methods
type MethodRoutingStats struct {
	SCIPSuccessRate   float64       `json:"scip_success_rate"`
	LSPSuccessRate    float64       `json:"lsp_success_rate"`
	PreferredSource   RoutingSource `json:"preferred_source"`
	ConfidenceScore   ConfidenceLevel `json:"confidence_score"`
	TotalQueries      int64         `json:"total_queries"`
	LastOptimized     time.Time     `json:"last_optimized"`
}

// PerformanceAnalysis provides detailed performance analysis for strategy optimization
type PerformanceAnalysis struct {
	Method            string        `json:"method"`
	SCIPLatencyP95    time.Duration `json:"scip_latency_p95"`
	LSPLatencyP95     time.Duration `json:"lsp_latency_p95"`
	SCIPErrorRate     float64       `json:"scip_error_rate"`
	LSPErrorRate      float64       `json:"lsp_error_rate"`
	CacheHitRate      float64       `json:"cache_hit_rate"`
	RecommendedStrategy SCIPRoutingStrategyType `json:"recommended_strategy"`
	ConfidenceLevel   ConfidenceLevel `json:"confidence_level"`
	AnalysisTime      time.Time     `json:"analysis_time"`
}

// SCIPSmartRouterImpl implements the SCIPSmartRouter interface
type SCIPSmartRouterImpl struct {
	*SmartRouterImpl
	
	// SCIP integration
	scipStore       indexing.SCIPStore
	confidenceScorer *ConfidenceScorer
	
	// SCIP-specific configuration
	scipConfig      *SCIPRoutingConfig
	methodStrategies map[string]SCIPRoutingStrategyType
	strategiesMutex sync.RWMutex
	
	// Performance tracking
	stats           *SCIPRoutingStats
	statsMutex      sync.RWMutex
	
	// Optimization
	optimizer       *RoutingOptimizer
	adaptiveEnabled bool
	
	// Context
	ctx             context.Context
	cancelFunc      context.CancelFunc
}

// SCIPRoutingConfig configures SCIP-aware routing behavior
type SCIPRoutingConfig struct {
	DefaultStrategy       SCIPRoutingStrategyType          `json:"default_strategy"`
	ConfidenceThreshold   ConfidenceLevel                  `json:"confidence_threshold"`
	MaxSCIPLatency        time.Duration                    `json:"max_scip_latency"`
	EnableFallback        bool                             `json:"enable_fallback"`
	EnableAdaptiveLearning bool                            `json:"enable_adaptive_learning"`
	CachePrewarmEnabled   bool                             `json:"cache_prewarm_enabled"`
	
	// Method-specific overrides
	MethodStrategies      map[string]SCIPRoutingStrategyType `json:"method_strategies"`
	MethodConfidenceThresholds map[string]ConfidenceLevel   `json:"method_confidence_thresholds"`
	
	// Performance optimization
	OptimizationInterval  time.Duration                    `json:"optimization_interval"`
	PerformanceWindowSize int                              `json:"performance_window_size"`
	
	// Scoring configuration
	ScoringConfig         *SCIPScoringConfig               `json:"scoring_config"`
}

// RoutingOptimizer provides continuous optimization of routing strategies
type RoutingOptimizer struct {
	config          *SCIPRoutingConfig
	performanceData map[string]*PerformanceWindow
	mutex           sync.RWMutex
	lastOptimization time.Time
}

// PerformanceWindow tracks recent performance data for optimization
type PerformanceWindow struct {
	Samples    []*PerformanceSample `json:"samples"`
	WindowSize int                  `json:"window_size"`
	mutex      sync.RWMutex
}

// PerformanceSample represents a single performance measurement
type PerformanceSample struct {
	Method       string                  `json:"method"`
	Source       RoutingSource           `json:"source"`
	Latency      time.Duration           `json:"latency"`
	Success      bool                    `json:"success"`
	Confidence   ConfidenceLevel         `json:"confidence"`
	CacheHit     bool                    `json:"cache_hit"`
	Timestamp    time.Time               `json:"timestamp"`
}

// NewSCIPSmartRouter creates a new SCIP-aware smart router
func NewSCIPSmartRouter(
	baseRouter *SmartRouterImpl,
	scipStore indexing.SCIPStore,
	config *SCIPRoutingConfig,
	logger *mcp.StructuredLogger,
) (*SCIPSmartRouterImpl, error) {
	if baseRouter == nil {
		return nil, fmt.Errorf("base router cannot be nil")
	}
	
	if config == nil {
		config = getDefaultSCIPRoutingConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create confidence scorer
	scorer := NewConfidenceScorer(config.ScoringConfig)
	
	// Create performance tracking
	stats := &SCIPRoutingStats{
		StrategyStats: make(map[SCIPRoutingStrategyType]*StrategyStats),
		MethodStats:   make(map[string]*MethodRoutingStats),
		StartTime:     time.Now(),
		LastUpdated:   time.Now(),
	}
	
	// Create optimizer
	optimizer := &RoutingOptimizer{
		config:          config,
		performanceData: make(map[string]*PerformanceWindow),
		lastOptimization: time.Now(),
	}
	
	router := &SCIPSmartRouterImpl{
		SmartRouterImpl:  baseRouter,
		scipStore:        scipStore,
		confidenceScorer: scorer,
		scipConfig:       config,
		methodStrategies: make(map[string]SCIPRoutingStrategyType),
		stats:           stats,
		optimizer:       optimizer,
		adaptiveEnabled: config.EnableAdaptiveLearning,
		ctx:             ctx,
		cancelFunc:      cancel,
	}
	
	// Initialize method strategies
	router.initializeMethodStrategies()
	
	// Start background optimization if adaptive learning is enabled
	if config.EnableAdaptiveLearning {
		go router.backgroundOptimization()
	}
	
	if logger != nil {
		logger.Infof("SCIP Smart Router initialized with strategy: %s", config.DefaultStrategy)
	}
	
	return router, nil
}

// RouteWithSCIPAwareness performs intelligent routing with SCIP awareness
func (sr *SCIPSmartRouterImpl) RouteWithSCIPAwareness(request *LSPRequest) (*SCIPRoutingDecision, error) {
	startTime := time.Now()
	
	// Analyze the query
	analysis := sr.analyzeQuery(request)
	
	// Get routing strategy for this method
	strategy := sr.GetSCIPRoutingStrategy(request.Method)
	
	// Make routing source decision
	sourceDecision, err := sr.DecideRoutingSource(request.Method, request.Params)
	if err != nil {
		return nil, fmt.Errorf("failed to decide routing source: %w", err)
	}
	
	// Create SCIP routing decision
	decision := &SCIPRoutingDecision{
		SCIPQueried:         false,
		SCIPConfidence:     ConfidenceMinimum,
		UseSCIP:            sourceDecision.Source == SourceSCIPCache || sourceDecision.Source == SourceHybrid,
		FallbackToLSP:      sourceDecision.FallbackSource != nil,
		DecisionReason:     sourceDecision.Reason,
		Strategy:           strategy,
		Confidence:         sourceDecision.Confidence,
		ExpectedLatency:    sourceDecision.EstimatedLatency,
		DecisionTime:       startTime,
		DecisionLatency:    time.Since(startTime),
		QueryAnalysis:      analysis,
	}
	
	// Handle SCIP routing if needed
	if decision.UseSCIP && sr.scipStore != nil {
		scipResult := sr.scipStore.Query(request.Method, request.Params)
		decision.SCIPQueried = true
		decision.SCIPResult = &scipResult
		decision.SCIPConfidence = sr.CalculateConfidence(request.Method, request.Params, &scipResult)
		
		// Update confidence based on SCIP result
		if scipResult.Found && decision.SCIPConfidence >= sr.scipConfig.ConfidenceThreshold {
			decision.Confidence = decision.SCIPConfidence
		} else if sr.scipConfig.EnableFallback {
			decision.FallbackToLSP = true
			decision.UseSCIP = false
			decision.DecisionReason = fmt.Sprintf("SCIP confidence %.2f below threshold %.2f", 
				decision.SCIPConfidence, sr.scipConfig.ConfidenceThreshold)
		}
	}
	
	// Get base routing decision if we need LSP
	if !decision.UseSCIP || decision.FallbackToLSP {
		baseDecision, err := sr.SmartRouterImpl.RouteRequest(request)
		if err != nil {
			return decision, fmt.Errorf("LSP routing failed: %w", err)
		}
		decision.RoutingDecision = baseDecision
	}
	
	// Update statistics
	sr.updateRoutingStats(decision)
	
	decision.DecisionLatency = time.Since(startTime)
	
	return decision, nil
}

// DecideRoutingSource determines the optimal routing source for a query
func (sr *SCIPSmartRouterImpl) DecideRoutingSource(method string, params interface{}) (*RoutingSourceDecision, error) {
	strategy := sr.GetSCIPRoutingStrategy(method)
	analysis := sr.analyzeMethodParams(method, params)
	
	var source RoutingSource
	var confidence ConfidenceLevel
	var reason string
	var fallbackSource *RoutingSource
	var estimatedLatency time.Duration
	
	switch strategy {
	case SCIPCacheFirst:
		source = SourceSCIPCache
		fallback := SourceLSPServer
		fallbackSource = &fallback
		confidence = sr.estimateConfidence(method, analysis)
		reason = "Cache-first strategy with LSP fallback"
		estimatedLatency = sr.estimateLatency(method, source)
		
	case SCIPLSPFirst:
		source = SourceLSPServer
		confidence = ConfidenceHigh
		reason = "LSP-first strategy for authoritative results"
		estimatedLatency = sr.estimateLatency(method, source)
		
	case SCIPHybrid:
		// Intelligent decision based on query analysis
		if analysis.IsSymbolQuery && !analysis.RequiresRuntime && analysis.Complexity <= ComplexityModerate {
			source = SourceSCIPCache
			fallback := SourceLSPServer
			fallbackSource = &fallback
			confidence = sr.estimateConfidence(method, analysis)
			reason = "Hybrid: Simple symbol query suitable for cache"
		} else if analysis.RequiresRuntime || analysis.Complexity >= ComplexityComplex {
			source = SourceLSPServer
			confidence = ConfidenceHigh
			reason = "Hybrid: Complex/runtime query requires LSP"
		} else {
			source = SourceHybrid
			confidence = ConfidenceMedium
			reason = "Hybrid: Using both sources for optimal results"
		}
		estimatedLatency = sr.estimateLatency(method, source)
		
	case SCIPCacheOnly:
		source = SourceSCIPCache
		confidence = sr.estimateConfidence(method, analysis)
		reason = "Cache-only strategy"
		estimatedLatency = sr.estimateLatency(method, source)
		
	case SCIPDisabled:
		fallthrough
	default:
		source = SourceLSPServer
		confidence = ConfidenceHigh
		reason = "SCIP disabled, using traditional LSP routing"
		estimatedLatency = sr.estimateLatency(method, source)
	}
	
	return &RoutingSourceDecision{
		Source:           source,
		Confidence:       confidence,
		Reason:           reason,
		Strategy:         strategy,
		FallbackSource:   fallbackSource,
		EstimatedLatency: estimatedLatency,
		CacheKey:         sr.generateCacheKey(method, params),
	}, nil
}

// CalculateConfidence calculates confidence level for SCIP query results
func (sr *SCIPSmartRouterImpl) CalculateConfidence(method string, params interface{}, queryResult *indexing.SCIPQueryResult) ConfidenceLevel {
	return sr.confidenceScorer.CalculateConfidence(method, params, queryResult)
}

// GetMethodRoutingHint returns the preferred routing strategy for a method
func (sr *SCIPSmartRouterImpl) GetMethodRoutingHint(method string) SCIPRoutingStrategyType {
	return sr.GetSCIPRoutingStrategy(method)
}

// PrewarmSCIPCache preloads SCIP cache with data for specified files
func (sr *SCIPSmartRouterImpl) PrewarmSCIPCache(ctx context.Context, files []string) error {
	if !sr.scipConfig.CachePrewarmEnabled || sr.scipStore == nil {
		return nil
	}
	
	// Implementation would preload common queries for the specified files
	// This is a placeholder for the actual prewarming logic
	return nil
}

// OptimizeRoutingStrategy optimizes routing strategy based on performance data
func (sr *SCIPSmartRouterImpl) OptimizeRoutingStrategy(method string, performanceData *PerformanceAnalysis) error {
	if !sr.adaptiveEnabled {
		return nil
	}
	
	sr.strategiesMutex.Lock()
	defer sr.strategiesMutex.Unlock()
	
	// Update strategy based on performance analysis
	if performanceData.RecommendedStrategy != "" {
		sr.methodStrategies[method] = performanceData.RecommendedStrategy
		
		// Update method stats
		sr.statsMutex.Lock()
		if sr.stats.MethodStats[method] == nil {
			sr.stats.MethodStats[method] = &MethodRoutingStats{}
		}
		sr.stats.MethodStats[method].LastOptimized = time.Now()
		sr.statsMutex.Unlock()
		
		if sr.logger != nil {
			sr.logger.Infof("Optimized routing strategy for %s to %s (confidence: %.2f)", 
				method, performanceData.RecommendedStrategy, performanceData.ConfidenceLevel)
		}
	}
	
	return nil
}

// SetSCIPStore sets the SCIP store for routing
func (sr *SCIPSmartRouterImpl) SetSCIPStore(store indexing.SCIPStore) error {
	sr.scipStore = store
	return nil
}

// GetSCIPStats returns comprehensive SCIP routing statistics
func (sr *SCIPSmartRouterImpl) GetSCIPStats() *SCIPRoutingStats {
	sr.statsMutex.RLock()
	defer sr.statsMutex.RUnlock()
	
	// Create a deep copy to avoid race conditions
	stats := &SCIPRoutingStats{
		TotalQueries:        sr.stats.TotalQueries,
		SCIPQueries:         sr.stats.SCIPQueries,
		LSPQueries:          sr.stats.LSPQueries,
		HybridQueries:       sr.stats.HybridQueries,
		SCIPHitRate:         sr.stats.SCIPHitRate,
		CacheHitRate:        sr.stats.CacheHitRate,
		AccuracyRate:        sr.stats.AccuracyRate,
		AverageSCIPLatency:  sr.stats.AverageSCIPLatency,
		AverageLSPLatency:   sr.stats.AverageLSPLatency,
		AverageDecisionTime: sr.stats.AverageDecisionTime,
		ErrorRate:           sr.stats.ErrorRate,
		FallbackRate:        sr.stats.FallbackRate,
		StartTime:           sr.stats.StartTime,
		LastUpdated:         sr.stats.LastUpdated,
		StrategyStats:       make(map[SCIPRoutingStrategyType]*StrategyStats),
		MethodStats:         make(map[string]*MethodRoutingStats),
	}
	
	// Copy strategy stats
	for k, v := range sr.stats.StrategyStats {
		stats.StrategyStats[k] = &StrategyStats{
			UseCount:       v.UseCount,
			SuccessRate:    v.SuccessRate,
			AverageLatency: v.AverageLatency,
			ErrorCount:     v.ErrorCount,
		}
	}
	
	// Copy method stats
	for k, v := range sr.stats.MethodStats {
		stats.MethodStats[k] = &MethodRoutingStats{
			SCIPSuccessRate:  v.SCIPSuccessRate,
			LSPSuccessRate:   v.LSPSuccessRate,
			PreferredSource:  v.PreferredSource,
			ConfidenceScore:  v.ConfidenceScore,
			TotalQueries:     v.TotalQueries,
			LastOptimized:    v.LastOptimized,
		}
	}
	
	return stats
}

// SetSCIPRoutingStrategy sets the SCIP routing strategy for a method
func (sr *SCIPSmartRouterImpl) SetSCIPRoutingStrategy(method string, strategy SCIPRoutingStrategyType) {
	sr.strategiesMutex.Lock()
	defer sr.strategiesMutex.Unlock()
	
	sr.methodStrategies[method] = strategy
	
	if sr.logger != nil {
		sr.logger.Debugf("Set SCIP routing strategy for %s to %s", method, strategy)
	}
}

// GetSCIPRoutingStrategy gets the SCIP routing strategy for a method
func (sr *SCIPSmartRouterImpl) GetSCIPRoutingStrategy(method string) SCIPRoutingStrategyType {
	sr.strategiesMutex.RLock()
	defer sr.strategiesMutex.RUnlock()
	
	if strategy, exists := sr.methodStrategies[method]; exists {
		return strategy
	}
	
	// Check config overrides
	if strategy, exists := sr.scipConfig.MethodStrategies[method]; exists {
		return strategy
	}
	
	return sr.scipConfig.DefaultStrategy
}

// Helper methods

// initializeMethodStrategies sets up default SCIP routing strategies for common methods
func (sr *SCIPSmartRouterImpl) initializeMethodStrategies() {
	defaults := map[string]SCIPRoutingStrategyType{
		LSP_METHOD_DEFINITION:      SCIPCacheFirst,    // Simple symbol lookups work well with cache
		LSP_METHOD_REFERENCES:      SCIPHybrid,        // May need both cache and live analysis
		LSP_METHOD_DOCUMENT_SYMBOL: SCIPCacheFirst,    // Static structure, good for cache
		LSP_METHOD_WORKSPACE_SYMBOL: SCIPHybrid,       // Large queries benefit from cache + live
		LSP_METHOD_HOVER:           SCIPCacheFirst,    // Static info, good for cache
		"textDocument/completion":  SCIPLSPFirst,      // Dynamic, requires live analysis
		"textDocument/diagnostic":  SCIPLSPFirst,      // Dynamic, requires live analysis
		"textDocument/codeAction":  SCIPLSPFirst,      // Dynamic, requires live analysis
		"textDocument/formatting":  SCIPDisabled,      // Not suitable for SCIP
	}
	
	for method, strategy := range defaults {
		sr.methodStrategies[method] = strategy
	}
}

// analyzeQuery performs comprehensive analysis of an LSP query
func (sr *SCIPSmartRouterImpl) analyzeQuery(request *LSPRequest) *QueryAnalysis {
	analysis := &QueryAnalysis{
		Method:           request.Method,
		AnalysisTime:     time.Now(),
		Metadata:         make(map[string]interface{}),
	}
	
	// Extract file and language information
	if request.URI != "" {
		analysis.FileURI = request.URI
		analysis.Language = sr.extractLanguage(request.URI)
	}
	
	if request.Context != nil {
		analysis.ProjectContext = request.Context.ProjectType
		analysis.IsWorkspaceQuery = request.Context.IsWorkspaceRequest()
	}
	
	// Analyze method characteristics
	analysis.IsSymbolQuery = sr.isSymbolQuery(request.Method)
	analysis.RequiresRuntime = sr.requiresRuntimeInfo(request.Method)
	analysis.CacheEligible = sr.isCacheEligible(request.Method)
	
	// Determine complexity
	analysis.Complexity = sr.analyzeComplexity(request)
	analysis.ComputationCost = sr.estimateComputationCost(request)
	
	// Set analysis hints
	analysis.StaticAnalysisHint = !analysis.RequiresRuntime && analysis.IsSymbolQuery
	analysis.DynamicAnalysisHint = analysis.RequiresRuntime || analysis.Complexity >= ComplexityComplex
	
	return analysis
}

// analyzeMethodParams analyzes method parameters for routing decisions
func (sr *SCIPSmartRouterImpl) analyzeMethodParams(method string, params interface{}) *QueryAnalysis {
	analysis := &QueryAnalysis{
		Method:       method,
		AnalysisTime: time.Now(),
		Metadata:     make(map[string]interface{}),
	}
	
	// Basic analysis based on method type
	analysis.IsSymbolQuery = sr.isSymbolQuery(method)
	analysis.RequiresRuntime = sr.requiresRuntimeInfo(method)
	analysis.CacheEligible = sr.isCacheEligible(method)
	
	// Extract URI if available
	if paramMap, ok := params.(map[string]interface{}); ok {
		if textDoc, ok := paramMap["textDocument"].(map[string]interface{}); ok {
			if uri, ok := textDoc["uri"].(string); ok {
				analysis.FileURI = uri
				analysis.Language = sr.extractLanguage(uri)
			}
		}
	}
	
	// Determine complexity based on method and params
	analysis.Complexity = sr.estimateComplexityFromParams(method, params)
	analysis.ComputationCost = sr.estimateCostFromParams(method, params)
	
	return analysis
}

// isSymbolQuery checks if the method is a symbol-related query
func (sr *SCIPSmartRouterImpl) isSymbolQuery(method string) bool {
	symbolMethods := map[string]bool{
		LSP_METHOD_DEFINITION:      true,
		LSP_METHOD_REFERENCES:      true,
		LSP_METHOD_DOCUMENT_SYMBOL: true,
		LSP_METHOD_WORKSPACE_SYMBOL: true,
		LSP_METHOD_HOVER:           true,
	}
	
	return symbolMethods[method]
}

// requiresRuntimeInfo checks if the method requires runtime information
func (sr *SCIPSmartRouterImpl) requiresRuntimeInfo(method string) bool {
	runtimeMethods := map[string]bool{
		"textDocument/completion":  true,
		"textDocument/diagnostic":  true,
		"textDocument/codeAction":  true,
	}
	
	return runtimeMethods[method]
}

// isCacheEligible checks if the method results are suitable for caching
func (sr *SCIPSmartRouterImpl) isCacheEligible(method string) bool {
	cacheEligibleMethods := map[string]bool{
		LSP_METHOD_DEFINITION:      true,
		LSP_METHOD_REFERENCES:      true,
		LSP_METHOD_DOCUMENT_SYMBOL: true,
		LSP_METHOD_WORKSPACE_SYMBOL: true,
		LSP_METHOD_HOVER:           true,
	}
	
	return cacheEligibleMethods[method]
}

// analyzeComplexity determines query complexity based on request
func (sr *SCIPSmartRouterImpl) analyzeComplexity(request *LSPRequest) QueryComplexity {
	switch request.Method {
	case LSP_METHOD_HOVER, LSP_METHOD_DEFINITION:
		return ComplexitySimple
	case LSP_METHOD_REFERENCES, LSP_METHOD_DOCUMENT_SYMBOL:
		return ComplexityModerate
	case LSP_METHOD_WORKSPACE_SYMBOL:
		return ComplexityComplex
	case "textDocument/completion", "textDocument/diagnostic":
		return ComplexityDynamic
	default:
		return ComplexityModerate
	}
}

// estimateComplexityFromParams estimates complexity from method and parameters
func (sr *SCIPSmartRouterImpl) estimateComplexityFromParams(method string, params interface{}) QueryComplexity {
	// Basic estimation based on method
	baseComplexity := ComplexityModerate
	
	switch method {
	case LSP_METHOD_HOVER, LSP_METHOD_DEFINITION:
		baseComplexity = ComplexitySimple
	case LSP_METHOD_WORKSPACE_SYMBOL:
		baseComplexity = ComplexityComplex
	case "textDocument/completion", "textDocument/diagnostic":
		baseComplexity = ComplexityDynamic
	}
	
	// Could analyze params for more detailed complexity estimation
	// This is a simplified version
	
	return baseComplexity
}

// estimateComputationCost estimates the computational cost of a request
func (sr *SCIPSmartRouterImpl) estimateComputationCost(request *LSPRequest) ComputationCost {
	switch request.Method {
	case LSP_METHOD_HOVER:
		return CostLow
	case LSP_METHOD_DEFINITION, LSP_METHOD_DOCUMENT_SYMBOL:
		return CostMedium
	case LSP_METHOD_REFERENCES:
		return CostHigh
	case LSP_METHOD_WORKSPACE_SYMBOL:
		return CostVeryHigh
	default:
		return CostMedium
	}
}

// estimateCostFromParams estimates cost from method and parameters
func (sr *SCIPSmartRouterImpl) estimateCostFromParams(method string, params interface{}) ComputationCost {
	// Simple estimation based on method
	switch method {
	case LSP_METHOD_HOVER:
		return CostLow
	case LSP_METHOD_DEFINITION, LSP_METHOD_DOCUMENT_SYMBOL:
		return CostMedium
	case LSP_METHOD_REFERENCES:
		return CostHigh
	case LSP_METHOD_WORKSPACE_SYMBOL:
		return CostVeryHigh
	default:
		return CostMedium
	}
}

// estimateConfidence estimates confidence for a method and analysis
func (sr *SCIPSmartRouterImpl) estimateConfidence(method string, analysis *QueryAnalysis) ConfidenceLevel {
	if sr.confidenceScorer != nil {
		return sr.confidenceScorer.EstimateConfidence(method, analysis)
	}
	
	// Fallback estimation
	if analysis.IsSymbolQuery && !analysis.RequiresRuntime {
		return ConfidenceHigh
	} else if analysis.CacheEligible {
		return ConfidenceMedium
	}
	
	return ConfidenceMinimum
}

// estimateLatency estimates latency for a method and source
func (sr *SCIPSmartRouterImpl) estimateLatency(method string, source RoutingSource) time.Duration {
	switch source {
	case SourceSCIPCache:
		switch method {
		case LSP_METHOD_HOVER, LSP_METHOD_DEFINITION:
			return 2 * time.Millisecond
		case LSP_METHOD_DOCUMENT_SYMBOL:
			return 5 * time.Millisecond
		case LSP_METHOD_REFERENCES:
			return 8 * time.Millisecond
		case LSP_METHOD_WORKSPACE_SYMBOL:
			return 15 * time.Millisecond
		default:
			return 5 * time.Millisecond
		}
	case SourceLSPServer:
		switch method {
		case LSP_METHOD_HOVER:
			return 20 * time.Millisecond
		case LSP_METHOD_DEFINITION:
			return 50 * time.Millisecond
		case LSP_METHOD_DOCUMENT_SYMBOL:
			return 100 * time.Millisecond
		case LSP_METHOD_REFERENCES:
			return 200 * time.Millisecond
		case LSP_METHOD_WORKSPACE_SYMBOL:
			return 500 * time.Millisecond
		default:
			return 100 * time.Millisecond
		}
	case SourceHybrid:
		// Estimate as slightly higher than cache due to overhead
		cacheLatency := sr.estimateLatency(method, SourceSCIPCache)
		return cacheLatency + (cacheLatency / 4)
	default:
		return 100 * time.Millisecond
	}
}

// extractLanguage extracts language from file URI
func (sr *SCIPSmartRouterImpl) extractLanguage(uri string) string {
	if !strings.HasPrefix(uri, "file://") {
		return ""
	}
	
	path := strings.TrimPrefix(uri, "file://")
	ext := filepath.Ext(path)
	
	extToLang := map[string]string{
		".go":   "go",
		".py":   "python",
		".js":   "javascript",
		".ts":   "typescript",
		".jsx":  "javascript",
		".tsx":  "typescript",
		".java": "java",
		".rs":   "rust",
	}
	
	if lang, exists := extToLang[ext]; exists {
		return lang
	}
	
	return ""
}

// generateCacheKey generates a cache key for method and params
func (sr *SCIPSmartRouterImpl) generateCacheKey(method string, params interface{}) string {
	// Simple implementation - could be enhanced
	return fmt.Sprintf("%s:%v", method, params)
}

// updateRoutingStats updates routing statistics
func (sr *SCIPSmartRouterImpl) updateRoutingStats(decision *SCIPRoutingDecision) {
	sr.statsMutex.Lock()
	defer sr.statsMutex.Unlock()
	
	sr.stats.TotalQueries++
	sr.stats.LastUpdated = time.Now()
	
	if decision.UseSCIP {
		sr.stats.SCIPQueries++
	} else {
		sr.stats.LSPQueries++
	}
	
	// Update strategy stats
	strategy := decision.Strategy
	if sr.stats.StrategyStats[strategy] == nil {
		sr.stats.StrategyStats[strategy] = &StrategyStats{}
	}
	sr.stats.StrategyStats[strategy].UseCount++
	
	// Update method stats
	method := decision.QueryAnalysis.Method
	if sr.stats.MethodStats[method] == nil {
		sr.stats.MethodStats[method] = &MethodRoutingStats{}
	}
	sr.stats.MethodStats[method].TotalQueries++
	
	// Update decision time metrics
	sr.updateLatencyMetrics(decision)
}

// updateLatencyMetrics updates latency-related metrics
func (sr *SCIPSmartRouterImpl) updateLatencyMetrics(decision *SCIPRoutingDecision) {
	// Simple exponential moving average
	alpha := 0.1
	
	if decision.UseSCIP && decision.SCIPResult != nil {
		if sr.stats.AverageSCIPLatency == 0 {
			sr.stats.AverageSCIPLatency = decision.SCIPResult.QueryTime
		} else {
			sr.stats.AverageSCIPLatency = time.Duration(
				float64(sr.stats.AverageSCIPLatency)*(1-alpha) + 
				float64(decision.SCIPResult.QueryTime)*alpha,
			)
		}
	}
	
	if sr.stats.AverageDecisionTime == 0 {
		sr.stats.AverageDecisionTime = decision.DecisionLatency
	} else {
		sr.stats.AverageDecisionTime = time.Duration(
			float64(sr.stats.AverageDecisionTime)*(1-alpha) + 
			float64(decision.DecisionLatency)*alpha,
		)
	}
}

// backgroundOptimization runs continuous performance optimization
func (sr *SCIPSmartRouterImpl) backgroundOptimization() {
	ticker := time.NewTicker(sr.scipConfig.OptimizationInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			sr.performOptimization()
		case <-sr.ctx.Done():
			return
		}
	}
}

// performOptimization analyzes performance and optimizes strategies
func (sr *SCIPSmartRouterImpl) performOptimization() {
	if !sr.adaptiveEnabled {
		return
	}
	
	// Analyze performance for each method
	sr.statsMutex.RLock()
	methodStats := make(map[string]*MethodRoutingStats)
	for k, v := range sr.stats.MethodStats {
		methodStats[k] = v
	}
	sr.statsMutex.RUnlock()
	
	for method, stats := range methodStats {
		if stats.TotalQueries < 10 { // Need sufficient data
			continue
		}
		
		// Analyze and potentially optimize strategy
		analysis := sr.analyzeMethodPerformance(method, stats)
		if analysis.RecommendedStrategy != sr.GetSCIPRoutingStrategy(method) {
			if err := sr.OptimizeRoutingStrategy(method, analysis); err != nil && sr.logger != nil {
				sr.logger.Warnf("Failed to optimize routing strategy for %s: %v", method, err)
			}
		}
	}
}

// analyzeMethodPerformance analyzes performance data for a method
func (sr *SCIPSmartRouterImpl) analyzeMethodPerformance(method string, stats *MethodRoutingStats) *PerformanceAnalysis {
	// This is a simplified analysis - real implementation would be more sophisticated
	analysis := &PerformanceAnalysis{
		Method:       method,
		AnalysisTime: time.Now(),
	}
	
	// Recommend strategy based on success rates
	if stats.SCIPSuccessRate > 0.9 && stats.SCIPSuccessRate > stats.LSPSuccessRate {
		analysis.RecommendedStrategy = SCIPCacheFirst
		analysis.ConfidenceLevel = ConfidenceHigh
	} else if stats.LSPSuccessRate > 0.9 {
		analysis.RecommendedStrategy = SCIPLSPFirst
		analysis.ConfidenceLevel = ConfidenceHigh
	} else {
		analysis.RecommendedStrategy = SCIPHybrid
		analysis.ConfidenceLevel = ConfidenceMedium
	}
	
	return analysis
}

// getDefaultSCIPRoutingConfig returns default SCIP routing configuration
func getDefaultSCIPRoutingConfig() *SCIPRoutingConfig {
	return &SCIPRoutingConfig{
		DefaultStrategy:           SCIPHybrid,
		ConfidenceThreshold:       ConfidenceMedium,
		MaxSCIPLatency:            100 * time.Millisecond,
		EnableFallback:            true,
		EnableAdaptiveLearning:    true,
		CachePrewarmEnabled:       true,
		MethodStrategies:          make(map[string]SCIPRoutingStrategyType),
		MethodConfidenceThresholds: make(map[string]ConfidenceLevel),
		OptimizationInterval:      5 * time.Minute,
		PerformanceWindowSize:     100,
		ScoringConfig:             getDefaultScoringConfig(),
	}
}

// getDefaultScoringConfig returns default scoring configuration
func getDefaultScoringConfig() *SCIPScoringConfig {
	return &SCIPScoringConfig{
		BaseConfidence:       ConfidenceMedium,
		CacheHitBonus:        0.1,
		RecentAccessBonus:    0.05,
		FileStalenessPenalty: 0.1,
		ComplexityPenalty: map[QueryComplexity]float64{
			ComplexitySimple:   0.0,
			ComplexityModerate: 0.05,
			ComplexityComplex:  0.1,
			ComplexityDynamic:  0.2,
		},
		MethodBonus: map[string]float64{
			LSP_METHOD_DEFINITION:      0.1,
			LSP_METHOD_DOCUMENT_SYMBOL: 0.1,
			LSP_METHOD_HOVER:           0.15,
		},
		AdaptiveLearning: true,
		LearningRate:     0.1,
	}
}

// NewConfidenceScorer creates a new confidence scorer
func NewConfidenceScorer(config *SCIPScoringConfig) *ConfidenceScorer {
	if config == nil {
		config = getDefaultScoringConfig()
	}
	
	return &ConfidenceScorer{
		config:              config,
		methodConfidenceMap: make(map[string]ConfidenceLevel),
		performanceHistory:  &PerformanceHistory{
			methodPerformance: make(map[string]*MethodPerformance),
		},
		adaptiveLearning: config.AdaptiveLearning,
	}
}

// CalculateConfidence calculates confidence level for SCIP query results
func (cs *ConfidenceScorer) CalculateConfidence(method string, params interface{}, queryResult *indexing.SCIPQueryResult) ConfidenceLevel {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()
	
	baseConfidence := float64(cs.config.BaseConfidence)
	
	// Cache hit bonus
	if queryResult.CacheHit {
		baseConfidence += cs.config.CacheHitBonus
	}
	
	// Method-specific bonus
	if bonus, exists := cs.config.MethodBonus[method]; exists {
		baseConfidence += bonus
	}
	
	// Query result quality
	if queryResult.Found {
		baseConfidence += 0.1
	} else {
		baseConfidence -= 0.2
	}
	
	// Confidence from SCIP result
	if queryResult.Confidence > 0 {
		baseConfidence = (baseConfidence + queryResult.Confidence) / 2
	}
	
	// Performance history adjustment
	if cs.adaptiveLearning {
		if perf, exists := cs.performanceHistory.methodPerformance[method]; exists {
			historyAdjustment := (perf.SCIPAccuracy - 0.5) * 0.2
			baseConfidence += historyAdjustment
		}
	}
	
	// Clamp to valid range
	if baseConfidence < float64(ConfidenceMinimum) {
		return ConfidenceMinimum
	}
	if baseConfidence > 1.0 {
		return 1.0
	}
	
	return ConfidenceLevel(baseConfidence)
}

// EstimateConfidence estimates confidence without querying SCIP
func (cs *ConfidenceScorer) EstimateConfidence(method string, analysis *QueryAnalysis) ConfidenceLevel {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()
	
	baseConfidence := float64(cs.config.BaseConfidence)
	
	// Method-specific bonus
	if bonus, exists := cs.config.MethodBonus[method]; exists {
		baseConfidence += bonus
	}
	
	// Complexity penalty
	if penalty, exists := cs.config.ComplexityPenalty[analysis.Complexity]; exists {
		baseConfidence -= penalty
	}
	
	// Cache eligibility bonus
	if analysis.CacheEligible {
		baseConfidence += 0.1
	}
	
	// Static analysis hint bonus
	if analysis.StaticAnalysisHint {
		baseConfidence += 0.1
	}
	
	// Dynamic analysis penalty
	if analysis.DynamicAnalysisHint {
		baseConfidence -= 0.15
	}
	
	// Clamp to valid range
	if baseConfidence < float64(ConfidenceMinimum) {
		return ConfidenceMinimum
	}
	if baseConfidence > 1.0 {
		return 1.0
	}
	
	return ConfidenceLevel(baseConfidence)
}

// Close cleans up resources
func (sr *SCIPSmartRouterImpl) Close() error {
	if sr.cancelFunc != nil {
		sr.cancelFunc()
	}
	
	if sr.scipStore != nil {
		return sr.scipStore.Close()
	}
	
	return nil
}

// Ensure SCIPSmartRouterImpl implements SCIPSmartRouter interface at compile time
var _ SCIPSmartRouter = (*SCIPSmartRouterImpl)(nil)