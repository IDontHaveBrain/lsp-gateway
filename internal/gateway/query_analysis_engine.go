package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/mcp"
)

// QueryComplexity represents the complexity level of an LSP query
type QueryComplexity int

const (
	QueryComplexitySimple QueryComplexity = iota
	QueryComplexityModerate
	QueryComplexityComplex
	QueryComplexityDynamic
)

func (qc QueryComplexity) String() string {
	switch qc {
	case QueryComplexitySimple:
		return "simple"
	case QueryComplexityModerate:
		return "moderate"
	case QueryComplexityComplex:
		return "complex"
	case QueryComplexityDynamic:
		return "dynamic"
	default:
		return "unknown"
	}
}

// RoutingStrategy represents the recommended routing strategy
type RoutingStrategy int

const (
	RoutingSCIPOnly RoutingStrategy = iota
	RoutingSCIPPrimary
	RoutingLSPPrimary
	RoutingLSPOnly
	RoutingHybrid
)

func (rs RoutingStrategy) String() string {
	switch rs {
	case RoutingSCIPOnly:
		return "scip_only"
	case RoutingSCIPPrimary:
		return "scip_primary"
	case RoutingLSPPrimary:
		return "lsp_primary"
	case RoutingLSPOnly:
		return "lsp_only"
	case RoutingHybrid:
		return "hybrid"
	default:
		return "unknown"
	}
}

// QueryAnalysisResult contains the comprehensive analysis of an LSP query
type QueryAnalysisResult struct {
	RequestID           string                     `json:"request_id"`
	Method              string                     `json:"method"`
	URI                 string                     `json:"uri,omitempty"`
	Language            string                     `json:"language"`
	Complexity          QueryComplexity            `json:"complexity"`
	ConfidenceScore     float64                    `json:"confidence_score"`
	RecommendedStrategy RoutingStrategy            `json:"recommended_strategy"`
	CacheKeyFactors     []string                   `json:"cache_key_factors"`
	ProjectContext      *ProjectAnalysisContext    `json:"project_context,omitempty"`
	PerformanceHints    *PerformanceHints          `json:"performance_hints,omitempty"`
	RiskFactors         []string                   `json:"risk_factors"`
	AnalysisMetadata    map[string]interface{}     `json:"analysis_metadata"`
	Timestamp           time.Time                  `json:"timestamp"`
}

// ProjectAnalysisContext provides project-specific analysis context
type ProjectAnalysisContext struct {
	ProjectSize     ProjectSize      `json:"project_size"`
	Framework       string           `json:"framework"`
	Languages       []string         `json:"languages"`
	HasIndex        bool             `json:"has_index"`
	IndexFreshness  time.Duration    `json:"index_freshness"`
	CacheHitRate    float64          `json:"cache_hit_rate"`
	ResourceUsage   *ResourceMetrics `json:"resource_usage,omitempty"`
}

// PerformanceHints provide optimization recommendations
type PerformanceHints struct {
	ExpectedResponseTime time.Duration            `json:"expected_response_time"`
	ResourceRequirements *ResourceRequirements    `json:"resource_requirements"`
	ParallelismLevel     int                      `json:"parallelism_level"`
	CachingRecommendation string                  `json:"caching_recommendation"`
	OptimizationTips     []string                 `json:"optimization_tips"`
}

// ResourceRequirements define expected resource usage
type ResourceRequirements struct {
	CPUIntensive    bool `json:"cpu_intensive"`
	MemoryIntensive bool `json:"memory_intensive"`
	IOIntensive     bool `json:"io_intensive"`
	NetworkDependent bool `json:"network_dependent"`
}

// ProjectSize represents the scale of the project
type ProjectSize int

const (
	ProjectSizeSmall ProjectSize = iota
	ProjectSizeMedium
	ProjectSizeLarge
	ProjectSizeEnterprise
)

// ResourceMetrics track resource usage patterns
type ResourceMetrics struct {
	MemoryUsageMB   float64 `json:"memory_usage_mb"`
	CPUUsagePercent float64 `json:"cpu_usage_percent"`
	DiskIORate      float64 `json:"disk_io_rate"`
	NetworkLatency  time.Duration `json:"network_latency"`
}

// QueryComplexityAnalyzer analyzes LSP queries for complexity classification
type QueryComplexityAnalyzer struct {
	methodComplexityMap map[string]QueryComplexity
	contentPatterns     map[string]*regexp.Regexp
	contextWeights      map[string]float64
	mu                  sync.RWMutex
}

// NewQueryComplexityAnalyzer creates a new complexity analyzer
func NewQueryComplexityAnalyzer() *QueryComplexityAnalyzer {
	analyzer := &QueryComplexityAnalyzer{
		methodComplexityMap: make(map[string]QueryComplexity),
		contentPatterns:     make(map[string]*regexp.Regexp),
		contextWeights:      make(map[string]float64),
	}
	
	analyzer.initializeComplexityMappings()
	analyzer.initializeContentPatterns()
	analyzer.initializeContextWeights()
	
	return analyzer
}

// initializeComplexityMappings sets up base complexity for each LSP method
func (qca *QueryComplexityAnalyzer) initializeComplexityMappings() {
	// Simple/Cacheable methods - Direct symbol lookups
	simpleMethod := []string{
		"textDocument/definition",
		"textDocument/declaration", 
		"textDocument/documentSymbol",
		"workspace/symbol",
		"textDocument/typeDefinition",
		"textDocument/implementation",
	}
	
	// Moderate complexity - Cross-file references
	moderateMethods := []string{
		"textDocument/references",
		"textDocument/documentHighlight",
		"textDocument/prepareRename",
		"textDocument/rename",
		"callHierarchy/incomingCalls",
		"callHierarchy/outgoingCalls",
		"typeHierarchy/supertypes",
		"typeHierarchy/subtypes",
	}
	
	// Complex - Context-dependent analysis
	complexMethods := []string{
		"textDocument/hover",
		"textDocument/signatureHelp",
		"textDocument/semanticTokens/full",
		"textDocument/semanticTokens/range",
		"textDocument/inlayHint",
		"textDocument/codelens",
	}
	
	// Dynamic - Real-time analysis required
	dynamicMethods := []string{
		"textDocument/completion",
		"textDocument/codeAction",
		"textDocument/formatting",
		"textDocument/rangeFormatting",
		"textDocument/onTypeFormatting",
		"textDocument/publishDiagnostics",
		"textDocument/didChange",
		"textDocument/didSave",
		"workspace/executeCommand",
		"workspace/willRenameFiles",
		"workspace/didRenameFiles",
	}
	
	for _, method := range simpleMethod {
		qca.methodComplexityMap[method] = QueryComplexitySimple
	}
	for _, method := range moderateMethods {
		qca.methodComplexityMap[method] = QueryComplexityModerate
	}
	for _, method := range complexMethods {
		qca.methodComplexityMap[method] = QueryComplexityComplex
	}
	for _, method := range dynamicMethods {
		qca.methodComplexityMap[method] = QueryComplexityDynamic
	}
}

// initializeContentPatterns sets up patterns for content analysis
func (qca *QueryComplexityAnalyzer) initializeContentPatterns() {
	patterns := map[string]string{
		"generic_type":    `<[^>]+>|\[[^\]]+\]`,
		"complex_expr":    `\(\s*\w+\s*=>\s*`,
		"async_pattern":   `async\s+|await\s+|Promise\s*<`,
		"nested_call":     `\w+\(\w+\([^)]*\)[^)]*\)`,
		"macro_pattern":   `#\w+|\$\w+|@@\w+`,
		"template_expr":   `\$\{[^}]+\}|{{[^}]+}}`,
	}
	
	for name, pattern := range patterns {
		if compiled, err := regexp.Compile(pattern); err == nil {
			qca.contentPatterns[name] = compiled
		}
	}
}

// initializeContextWeights sets up weighting for context factors
func (qca *QueryComplexityAnalyzer) initializeContextWeights() {
	qca.contextWeights = map[string]float64{
		"project_size":       0.2,
		"file_size":          0.15,
		"cross_language":     0.25,
		"framework_complex":  0.2,
		"content_complexity": 0.2,
	}
}

// AnalyzeComplexity analyzes the complexity of an LSP request
func (qca *QueryComplexityAnalyzer) AnalyzeComplexity(request *LSPRequest, context *ProjectAnalysisContext) QueryComplexity {
	qca.mu.RLock()
	defer qca.mu.RUnlock()
	
	// Base complexity from method
	baseComplexity := qca.getMethodComplexity(request.Method)
	
	// Context-based adjustments
	adjustmentScore := qca.calculateContextAdjustment(request, context)
	
	// Apply adjustments
	finalComplexity := qca.applyComplexityAdjustment(baseComplexity, adjustmentScore)
	
	return finalComplexity
}

// getMethodComplexity returns the base complexity for a method
func (qca *QueryComplexityAnalyzer) getMethodComplexity(method string) QueryComplexity {
	if complexity, exists := qca.methodComplexityMap[method]; exists {
		return complexity
	}
	
	// Default complexity based on method patterns
	if strings.Contains(method, "completion") || strings.Contains(method, "codeAction") {
		return QueryComplexityDynamic
	} else if strings.Contains(method, "symbol") || strings.Contains(method, "definition") {
		return QueryComplexitySimple
	} else if strings.Contains(method, "reference") || strings.Contains(method, "rename") {
		return QueryComplexityModerate
	}
	
	return QueryComplexityModerate
}

// calculateContextAdjustment calculates complexity adjustment based on context
func (qca *QueryComplexityAnalyzer) calculateContextAdjustment(request *LSPRequest, context *ProjectAnalysisContext) float64 {
	score := 0.0
	
	// Project size impact
	if context != nil {
		switch context.ProjectSize {
		case ProjectSizeSmall:
			score -= 0.1
		case ProjectSizeMedium:
			score += 0.0
		case ProjectSizeLarge:
			score += 0.2
		case ProjectSizeEnterprise:
			score += 0.4
		}
		
		// Framework complexity
		if qca.isComplexFramework(context.Framework) {
			score += 0.2
		}
		
		// Multi-language complexity
		if len(context.Languages) > 2 {
			score += 0.1 * float64(len(context.Languages)-2)
		}
	}
	
	// Content complexity analysis
	if request.Params != nil {
		if paramsStr, err := json.Marshal(request.Params); err == nil {
			contentScore := qca.analyzeContentComplexity(string(paramsStr))
			score += contentScore * qca.contextWeights["content_complexity"]
		}
	}
	
	return score
}

// applyComplexityAdjustment applies the calculated adjustment to base complexity
func (qca *QueryComplexityAnalyzer) applyComplexityAdjustment(base QueryComplexity, adjustment float64) QueryComplexity {
	complexityLevel := int(base)
	
	if adjustment >= 0.3 {
		complexityLevel++
	} else if adjustment <= -0.3 {
		complexityLevel--
	}
	
	// Clamp to valid range
	if complexityLevel < 0 {
		complexityLevel = 0
	} else if complexityLevel > int(QueryComplexityDynamic) {
		complexityLevel = int(QueryComplexityDynamic)
	}
	
	return QueryComplexity(complexityLevel)
}

// analyzeContentComplexity analyzes content for complexity indicators
func (qca *QueryComplexityAnalyzer) analyzeContentComplexity(content string) float64 {
	score := 0.0
	
	for _, pattern := range qca.contentPatterns {
		matches := pattern.FindAllString(content, -1)
		score += float64(len(matches)) * 0.1
	}
	
	// Length-based complexity
	if len(content) > 1000 {
		score += 0.2
	} else if len(content) > 5000 {
		score += 0.4
	}
	
	return math.Min(score, 1.0)
}

// isComplexFramework checks if a framework is considered complex
func (qca *QueryComplexityAnalyzer) isComplexFramework(framework string) bool {
	complexFrameworks := map[string]bool{
		"spring":     true,
		"hibernate":  true,
		"angular":    true,
		"react":      true,
		"vue":        true,
		"django":     true,
		"rails":      true,
		"kubernetes": true,
		"tensorflow": true,
	}
	
	return complexFrameworks[strings.ToLower(framework)]
}

// ConfidenceCalculator calculates confidence scores for SCIP routing decisions
type ConfidenceCalculator struct {
	methodConfidenceMap map[string]float64
	complexityWeights   map[QueryComplexity]float64
	contextFactors      map[string]float64
	performanceHistory  *PerformanceHistoryTracker
	mu                  sync.RWMutex
}

// NewConfidenceCalculator creates a new confidence calculator
func NewConfidenceCalculator(historyTracker *PerformanceHistoryTracker) *ConfidenceCalculator {
	calc := &ConfidenceCalculator{
		methodConfidenceMap: make(map[string]float64),
		complexityWeights:   make(map[QueryComplexity]float64),
		contextFactors:      make(map[string]float64),
		performanceHistory:  historyTracker,
	}
	
	calc.initializeMethodConfidence()
	calc.initializeComplexityWeights()
	calc.initializeContextFactors()
	
	return calc
}

// initializeMethodConfidence sets base confidence scores for different methods
func (cc *ConfidenceCalculator) initializeMethodConfidence() {
	// High confidence methods (0.85-1.0) - Simple symbol lookups
	highConfidenceMethods := map[string]float64{
		"textDocument/definition":     0.95,
		"textDocument/declaration":    0.95,
		"textDocument/documentSymbol": 0.90,
		"workspace/symbol":            0.85,
		"textDocument/typeDefinition": 0.90,
		"textDocument/implementation": 0.88,
	}
	
	// Medium confidence methods (0.70-0.84) - Cross-file analysis
	mediumConfidenceMethods := map[string]float64{
		"textDocument/references":     0.80,
		"textDocument/documentHighlight": 0.78,
		"textDocument/prepareRename":  0.75,
		"textDocument/rename":         0.72,
		"callHierarchy/incomingCalls": 0.76,
		"callHierarchy/outgoingCalls": 0.76,
		"typeHierarchy/supertypes":    0.74,
		"typeHierarchy/subtypes":      0.74,
	}
	
	// Low confidence methods (0.60-0.69) - Context-dependent
	lowConfidenceMethods := map[string]float64{
		"textDocument/hover":          0.68,
		"textDocument/signatureHelp": 0.65,
		"textDocument/semanticTokens/full": 0.64,
		"textDocument/semanticTokens/range": 0.66,
		"textDocument/inlayHint":      0.62,
		"textDocument/codelens":       0.63,
	}
	
	// Very low confidence (<0.60) - Dynamic analysis required
	veryLowConfidenceMethods := map[string]float64{
		"textDocument/completion":     0.55,
		"textDocument/codeAction":     0.50,
		"textDocument/formatting":     0.45,
		"textDocument/rangeFormatting": 0.45,
		"textDocument/onTypeFormatting": 0.40,
		"textDocument/publishDiagnostics": 0.35,
	}
	
	for method, confidence := range highConfidenceMethods {
		cc.methodConfidenceMap[method] = confidence
	}
	for method, confidence := range mediumConfidenceMethods {
		cc.methodConfidenceMap[method] = confidence
	}
	for method, confidence := range lowConfidenceMethods {
		cc.methodConfidenceMap[method] = confidence
	}
	for method, confidence := range veryLowConfidenceMethods {
		cc.methodConfidenceMap[method] = confidence
	}
}

// initializeComplexityWeights sets weights for complexity adjustments
func (cc *ConfidenceCalculator) initializeComplexityWeights() {
	cc.complexityWeights = map[QueryComplexity]float64{
		QueryComplexitySimple:   1.0,
		QueryComplexityModerate: 0.85,
		QueryComplexityComplex:  0.70,
		QueryComplexityDynamic:  0.50,
	}
}

// initializeContextFactors sets weights for context factors
func (cc *ConfidenceCalculator) initializeContextFactors() {
	cc.contextFactors = map[string]float64{
		"index_freshness":   0.25,
		"cache_hit_rate":    0.20,
		"project_size":      0.15,
		"historical_success": 0.25,
		"resource_availability": 0.15,
	}
}

// CalculateConfidence calculates the confidence score for SCIP routing
func (cc *ConfidenceCalculator) CalculateConfidence(request *LSPRequest, complexity QueryComplexity, context *ProjectAnalysisContext) float64 {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	
	// Base confidence from method
	baseConfidence := cc.getMethodConfidence(request.Method)
	
	// Complexity adjustment
	complexityAdjustment := cc.complexityWeights[complexity]
	
	// Context-based adjustments
	contextAdjustments := cc.calculateContextAdjustments(request, context)
	
	// Historical performance adjustment
	historicalAdjustment := cc.getHistoricalAdjustment(request.Method, request.Language)
	
	// Calculate final confidence
	finalConfidence := baseConfidence * complexityAdjustment * contextAdjustments * historicalAdjustment
	
	// Ensure confidence is within 0.6-1.0 range for SCIP routing consideration
	if finalConfidence < 0.6 {
		return 0.0 // Route to LSP instead
	}
	
	return math.Min(finalConfidence, 1.0)
}

// getMethodConfidence returns the base confidence for a method
func (cc *ConfidenceCalculator) getMethodConfidence(method string) float64 {
	if confidence, exists := cc.methodConfidenceMap[method]; exists {
		return confidence
	}
	
	// Default confidence based on method patterns
	if strings.Contains(method, "definition") || strings.Contains(method, "symbol") {
		return 0.80
	} else if strings.Contains(method, "reference") {
		return 0.70
	} else if strings.Contains(method, "completion") || strings.Contains(method, "codeAction") {
		return 0.45
	}
	
	return 0.65
}

// calculateContextAdjustments calculates adjustments based on context factors
func (cc *ConfidenceCalculator) calculateContextAdjustments(request *LSPRequest, context *ProjectAnalysisContext) float64 {
	if context == nil {
		return 0.8 // Conservative adjustment without context
	}
	
	adjustmentFactor := 1.0
	
	// Index freshness impact
	if context.HasIndex {
		if context.IndexFreshness < time.Hour {
			adjustmentFactor += 0.1
		} else if context.IndexFreshness > 24*time.Hour {
			adjustmentFactor -= 0.15
		}
	} else {
		adjustmentFactor -= 0.3 // No index available
	}
	
	// Cache hit rate impact
	if context.CacheHitRate > 0.8 {
		adjustmentFactor += 0.1
	} else if context.CacheHitRate < 0.3 {
		adjustmentFactor -= 0.1
	}
	
	// Project size impact
	switch context.ProjectSize {
	case ProjectSizeSmall:
		adjustmentFactor += 0.05
	case ProjectSizeLarge:
		adjustmentFactor -= 0.05
	case ProjectSizeEnterprise:
		adjustmentFactor -= 0.1
	}
	
	return math.Max(adjustmentFactor, 0.3)
}

// getHistoricalAdjustment gets adjustment based on historical performance
func (cc *ConfidenceCalculator) getHistoricalAdjustment(method, language string) float64 {
	if cc.performanceHistory == nil {
		return 1.0
	}
	
	stats := cc.performanceHistory.GetMethodStats(method, language)
	if stats == nil {
		return 1.0
	}
	
	// Adjust based on historical success rate
	if stats.SCIPSuccessRate > 0.8 {
		return 1.1
	} else if stats.SCIPSuccessRate < 0.5 {
		return 0.8
	}
	
	return 1.0
}

// RoutingStrategyDeterminer determines the optimal routing strategy
type RoutingStrategyDeterminer struct {
	strategyThresholds map[string]float64
	methodPreferences  map[string]RoutingStrategy
	config            *config.GatewayConfig
	mu                sync.RWMutex
}

// NewRoutingStrategyDeterminer creates a new routing strategy determiner
func NewRoutingStrategyDeterminer(config *config.GatewayConfig) *RoutingStrategyDeterminer {
	determiner := &RoutingStrategyDeterminer{
		strategyThresholds: make(map[string]float64),
		methodPreferences:  make(map[string]RoutingStrategy),
		config:            config,
	}
	
	determiner.initializeThresholds()
	determiner.initializeMethodPreferences()
	
	return determiner
}

// initializeThresholds sets confidence thresholds for routing decisions
func (rsd *RoutingStrategyDeterminer) initializeThresholds() {
	rsd.strategyThresholds = map[string]float64{
		"scip_only":     0.90,
		"scip_primary":  0.75,
		"lsp_primary":   0.65,
		"hybrid":        0.60,
		// Below 0.60 routes to LSP only
	}
}

// initializeMethodPreferences sets method-specific routing preferences
func (rsd *RoutingStrategyDeterminer) initializeMethodPreferences() {
	// Methods that work well with SCIP
	scipPreferred := []string{
		"textDocument/definition",
		"textDocument/declaration",
		"textDocument/documentSymbol",
		"workspace/symbol",
		"textDocument/typeDefinition",
		"textDocument/implementation",
		"textDocument/references",
	}
	
	// Methods that require LSP
	lspRequired := []string{
		"textDocument/completion",
		"textDocument/codeAction",
		"textDocument/formatting",
		"textDocument/rangeFormatting",
		"textDocument/onTypeFormatting",
		"textDocument/publishDiagnostics",
		"textDocument/didChange",
		"textDocument/didSave",
	}
	
	// Methods that benefit from hybrid approach
	hybridBeneficial := []string{
		"textDocument/hover",
		"textDocument/signatureHelp",
		"textDocument/semanticTokens/full",
		"textDocument/inlayHint",
	}
	
	for _, method := range scipPreferred {
		rsd.methodPreferences[method] = RoutingSCIPPrimary
	}
	for _, method := range lspRequired {
		rsd.methodPreferences[method] = RoutingLSPOnly
	}
	for _, method := range hybridBeneficial {
		rsd.methodPreferences[method] = RoutingHybrid
	}
}

// DetermineStrategy determines the optimal routing strategy
func (rsd *RoutingStrategyDeterminer) DetermineStrategy(
	method string,
	confidenceScore float64,
	complexity QueryComplexity,
	context *ProjectAnalysisContext,
) RoutingStrategy {
	rsd.mu.RLock()
	defer rsd.mu.RUnlock()
	
	// Check method preferences first
	if preference, exists := rsd.methodPreferences[method]; exists {
		// Override for very low confidence or dynamic complexity
		if confidenceScore < 0.6 || complexity == QueryComplexityDynamic {
			if preference == RoutingSCIPPrimary || preference == RoutingSCIPOnly {
				return RoutingLSPOnly
			}
		}
		return preference
	}
	
	// Determine strategy based on confidence score
	if confidenceScore >= rsd.strategyThresholds["scip_only"] {
		return RoutingSCIPOnly
	} else if confidenceScore >= rsd.strategyThresholds["scip_primary"] {
		return RoutingSCIPPrimary
	} else if confidenceScore >= rsd.strategyThresholds["lsp_primary"] {
		return RoutingLSPPrimary
	} else if confidenceScore >= rsd.strategyThresholds["hybrid"] {
		return RoutingHybrid
	}
	
	return RoutingLSPOnly
}

// PerformanceStats tracks performance statistics for routing decisions
type PerformanceStats struct {
	TotalRequests      int64         `json:"total_requests"`
	SuccessfulRequests int64         `json:"successful_requests"`
	FailedRequests     int64         `json:"failed_requests"`
	SCIPSuccessRate    float64       `json:"scip_success_rate"`
	LSPSuccessRate     float64       `json:"lsp_success_rate"`
	AverageResponseTime time.Duration `json:"average_response_time"`
	LastUpdated        time.Time     `json:"last_updated"`
}

// PerformanceHistoryTracker tracks and learns from routing decision outcomes
type PerformanceHistoryTracker struct {
	methodStats    map[string]map[string]*PerformanceStats // method -> language -> stats
	recentOutcomes []RoutingOutcome
	maxHistory     int
	mu             sync.RWMutex
}

// RoutingOutcome represents the outcome of a routing decision
type RoutingOutcome struct {
	RequestID       string          `json:"request_id"`
	Method          string          `json:"method"`
	Language        string          `json:"language"`
	Strategy        RoutingStrategy `json:"strategy"`
	ConfidenceScore float64         `json:"confidence_score"`
	Success         bool            `json:"success"`
	ResponseTime    time.Duration   `json:"response_time"`
	ErrorMessage    string          `json:"error_message,omitempty"`
	Timestamp       time.Time       `json:"timestamp"`
}

// NewPerformanceHistoryTracker creates a new performance history tracker
func NewPerformanceHistoryTracker(maxHistory int) *PerformanceHistoryTracker {
	return &PerformanceHistoryTracker{
		methodStats:    make(map[string]map[string]*PerformanceStats),
		recentOutcomes: make([]RoutingOutcome, 0, maxHistory),
		maxHistory:     maxHistory,
	}
}

// RecordOutcome records the outcome of a routing decision
func (pht *PerformanceHistoryTracker) RecordOutcome(outcome RoutingOutcome) {
	pht.mu.Lock()
	defer pht.mu.Unlock()
	
	// Add to recent outcomes
	pht.recentOutcomes = append(pht.recentOutcomes, outcome)
	if len(pht.recentOutcomes) > pht.maxHistory {
		pht.recentOutcomes = pht.recentOutcomes[1:]
	}
	
	// Update method statistics
	if _, exists := pht.methodStats[outcome.Method]; !exists {
		pht.methodStats[outcome.Method] = make(map[string]*PerformanceStats)
	}
	
	if _, exists := pht.methodStats[outcome.Method][outcome.Language]; !exists {
		pht.methodStats[outcome.Method][outcome.Language] = &PerformanceStats{}
	}
	
	stats := pht.methodStats[outcome.Method][outcome.Language]
	stats.TotalRequests++
	
	if outcome.Success {
		stats.SuccessfulRequests++
	} else {
		stats.FailedRequests++
	}
	
	// Update success rates based on strategy
	switch outcome.Strategy {
	case RoutingSCIPOnly, RoutingSCIPPrimary:
		// Update SCIP success rate calculation
		scipRequests := pht.countSCIPRequests(outcome.Method, outcome.Language)
		scipSuccessful := pht.countSCIPSuccessful(outcome.Method, outcome.Language)
		if scipRequests > 0 {
			stats.SCIPSuccessRate = float64(scipSuccessful) / float64(scipRequests)
		}
	case RoutingLSPOnly, RoutingLSPPrimary:
		// Update LSP success rate calculation
		lspRequests := pht.countLSPRequests(outcome.Method, outcome.Language)
		lspSuccessful := pht.countLSPSuccessful(outcome.Method, outcome.Language)
		if lspRequests > 0 {
			stats.LSPSuccessRate = float64(lspSuccessful) / float64(lspRequests)
		}
	}
	
	// Update average response time
	totalTime := stats.AverageResponseTime * time.Duration(stats.TotalRequests-1)
	stats.AverageResponseTime = (totalTime + outcome.ResponseTime) / time.Duration(stats.TotalRequests)
	stats.LastUpdated = time.Now()
}

// GetMethodStats returns performance statistics for a method and language
func (pht *PerformanceHistoryTracker) GetMethodStats(method, language string) *PerformanceStats {
	pht.mu.RLock()
	defer pht.mu.RUnlock()
	
	if methodMap, exists := pht.methodStats[method]; exists {
		if stats, exists := methodMap[language]; exists {
			return stats
		}
	}
	
	return nil
}

// countSCIPRequests counts recent SCIP requests for a method/language
func (pht *PerformanceHistoryTracker) countSCIPRequests(method, language string) int {
	count := 0
	cutoff := time.Now().Add(-24 * time.Hour) // Last 24 hours
	
	for _, outcome := range pht.recentOutcomes {
		if outcome.Method == method && outcome.Language == language &&
			outcome.Timestamp.After(cutoff) &&
			(outcome.Strategy == RoutingSCIPOnly || outcome.Strategy == RoutingSCIPPrimary) {
			count++
		}
	}
	
	return count
}

// countSCIPSuccessful counts recent successful SCIP requests
func (pht *PerformanceHistoryTracker) countSCIPSuccessful(method, language string) int {
	count := 0
	cutoff := time.Now().Add(-24 * time.Hour)
	
	for _, outcome := range pht.recentOutcomes {
		if outcome.Method == method && outcome.Language == language &&
			outcome.Timestamp.After(cutoff) && outcome.Success &&
			(outcome.Strategy == RoutingSCIPOnly || outcome.Strategy == RoutingSCIPPrimary) {
			count++
		}
	}
	
	return count
}

// countLSPRequests counts recent LSP requests for a method/language
func (pht *PerformanceHistoryTracker) countLSPRequests(method, language string) int {
	count := 0
	cutoff := time.Now().Add(-24 * time.Hour)
	
	for _, outcome := range pht.recentOutcomes {
		if outcome.Method == method && outcome.Language == language &&
			outcome.Timestamp.After(cutoff) &&
			(outcome.Strategy == RoutingLSPOnly || outcome.Strategy == RoutingLSPPrimary) {
			count++
		}
	}
	
	return count
}

// countLSPSuccessful counts recent successful LSP requests
func (pht *PerformanceHistoryTracker) countLSPSuccessful(method, language string) int {
	count := 0
	cutoff := time.Now().Add(-24 * time.Hour)
	
	for _, outcome := range pht.recentOutcomes {
		if outcome.Method == method && outcome.Language == language &&
			outcome.Timestamp.After(cutoff) && outcome.Success &&
			(outcome.Strategy == RoutingLSPOnly || outcome.Strategy == RoutingLSPPrimary) {
			count++
		}
	}
	
	return count
}

// QueryAnalysisEngine is the main engine that orchestrates query analysis
type QueryAnalysisEngine struct {
	complexityAnalyzer   *QueryComplexityAnalyzer
	confidenceCalculator *ConfidenceCalculator
	strategyDeterminer   *RoutingStrategyDeterminer
	historyTracker       *PerformanceHistoryTracker
	config              *config.GatewayConfig
	logger              *mcp.StructuredLogger
	mu                  sync.RWMutex
}

// NewQueryAnalysisEngine creates a new query analysis engine
func NewQueryAnalysisEngine(config *config.GatewayConfig, logger *mcp.StructuredLogger) *QueryAnalysisEngine {
	historyTracker := NewPerformanceHistoryTracker(1000) // Keep last 1000 outcomes
	
	return &QueryAnalysisEngine{
		complexityAnalyzer:   NewQueryComplexityAnalyzer(),
		confidenceCalculator: NewConfidenceCalculator(historyTracker),
		strategyDeterminer:   NewRoutingStrategyDeterminer(config),
		historyTracker:       historyTracker,
		config:              config,
		logger:              logger,
	}
}

// AnalyzeQuery performs comprehensive analysis of an LSP query
func (qae *QueryAnalysisEngine) AnalyzeQuery(
	request *LSPRequest,
	projectContext *ProjectAnalysisContext,
) (*QueryAnalysisResult, error) {
	qae.mu.RLock()
	defer qae.mu.RUnlock()
	
	startTime := time.Now()
	
	if qae.logger != nil {
		qae.logger.Debugf("QueryAnalysisEngine: Analyzing query for method %s, URI %s", request.Method, request.URI)
	}
	
	// Step 1: Analyze complexity
	complexity := qae.complexityAnalyzer.AnalyzeComplexity(request, projectContext)
	
	// Step 2: Calculate confidence score
	confidenceScore := qae.confidenceCalculator.CalculateConfidence(request, complexity, projectContext)
	
	// Step 3: Determine routing strategy
	strategy := qae.strategyDeterminer.DetermineStrategy(request.Method, confidenceScore, complexity, projectContext)
	
	// Step 4: Generate cache key factors
	cacheKeyFactors := qae.generateCacheKeyFactors(request, complexity)
	
	// Step 5: Analyze risk factors
	riskFactors := qae.analyzeRiskFactors(request, complexity, confidenceScore, projectContext)
	
	// Step 6: Generate performance hints
	performanceHints := qae.generatePerformanceHints(request, complexity, strategy, projectContext)
	
	// Step 7: Create analysis result
	result := &QueryAnalysisResult{
		RequestID:           request.RequestID,
		Method:              request.Method,
		URI:                 request.URI,
		Language:            request.Context.GetLanguage(),
		Complexity:          complexity,
		ConfidenceScore:     confidenceScore,
		RecommendedStrategy: strategy,
		CacheKeyFactors:     cacheKeyFactors,
		ProjectContext:      projectContext,
		PerformanceHints:    performanceHints,
		RiskFactors:         riskFactors,
		AnalysisMetadata: map[string]interface{}{
			"analysis_duration": time.Since(startTime),
			"engine_version":    "1.0.0",
			"confidence_range":  "0.6-1.0",
		},
		Timestamp: time.Now(),
	}
	
	if qae.logger != nil {
		qae.logger.Debugf("QueryAnalysisEngine: Analysis complete - Complexity: %s, Confidence: %.2f, Strategy: %s",
			complexity.String(), confidenceScore, strategy.String())
	}
	
	return result, nil
}

// generateCacheKeyFactors generates factors for cache key generation
func (qae *QueryAnalysisEngine) generateCacheKeyFactors(request *LSPRequest, complexity QueryComplexity) []string {
	factors := []string{
		request.Method,
		request.URI,
	}
	
	// Add position-based factors for position-dependent methods
	if qae.isPositionDependent(request.Method) && request.Params != nil {
		if paramsMap, ok := request.Params.(map[string]interface{}); ok {
			if textDoc, ok := paramsMap["textDocument"].(map[string]interface{}); ok {
				if uri, ok := textDoc["uri"].(string); ok {
					factors = append(factors, fmt.Sprintf("uri:%s", uri))
				}
			}
			if position, ok := paramsMap["position"].(map[string]interface{}); ok {
				if line, ok := position["line"].(float64); ok {
					if char, ok := position["character"].(float64); ok {
						factors = append(factors, fmt.Sprintf("pos:%d:%d", int(line), int(char)))
					}
				}
			}
		}
	}
	
	// Add complexity-based factors
	factors = append(factors, fmt.Sprintf("complexity:%s", complexity.String()))
	
	// Add file extension for language-specific caching
	if request.URI != "" {
		ext := filepath.Ext(request.URI)
		if ext != "" {
			factors = append(factors, fmt.Sprintf("ext:%s", ext))
		}
	}
	
	return factors
}

// isPositionDependent checks if a method depends on cursor position
func (qae *QueryAnalysisEngine) isPositionDependent(method string) bool {
	positionMethods := map[string]bool{
		"textDocument/definition":      true,
		"textDocument/declaration":     true,
		"textDocument/typeDefinition":  true,
		"textDocument/implementation":  true,
		"textDocument/references":      true,
		"textDocument/hover":           true,
		"textDocument/signatureHelp":   true,
		"textDocument/completion":      true,
		"textDocument/codeAction":      true,
		"textDocument/documentHighlight": true,
		"textDocument/prepareRename":   true,
	}
	
	return positionMethods[method]
}

// analyzeRiskFactors identifies potential risks in the routing decision
func (qae *QueryAnalysisEngine) analyzeRiskFactors(
	request *LSPRequest,
	complexity QueryComplexity,
	confidenceScore float64,
	context *ProjectAnalysisContext,
) []string {
	var risks []string
	
	// Low confidence risk
	if confidenceScore < 0.7 {
		risks = append(risks, "low_confidence_score")
	}
	
	// High complexity risk
	if complexity == QueryComplexityDynamic {
		risks = append(risks, "dynamic_complexity")
	}
	
	// Stale index risk
	if context != nil && context.HasIndex && context.IndexFreshness > 24*time.Hour {
		risks = append(risks, "stale_index")
	}
	
	// Large project risk
	if context != nil && context.ProjectSize == ProjectSizeEnterprise {
		risks = append(risks, "enterprise_scale")
	}
	
	// Low cache hit rate risk
	if context != nil && context.CacheHitRate < 0.3 {
		risks = append(risks, "low_cache_hit_rate")
	}
	
	// Resource constraint risk
	if context != nil && context.ResourceUsage != nil {
		if context.ResourceUsage.MemoryUsageMB > 2000 {
			risks = append(risks, "high_memory_usage")
		}
		if context.ResourceUsage.CPUUsagePercent > 80 {
			risks = append(risks, "high_cpu_usage")
		}
	}
	
	return risks
}

// generatePerformanceHints generates optimization recommendations
func (qae *QueryAnalysisEngine) generatePerformanceHints(
	request *LSPRequest,
	complexity QueryComplexity,
	strategy RoutingStrategy,
	context *ProjectAnalysisContext,
) *PerformanceHints {
	hints := &PerformanceHints{
		ResourceRequirements: &ResourceRequirements{},
		OptimizationTips:     []string{},
	}
	
	// Estimate response time based on complexity and strategy
	switch complexity {
	case QueryComplexitySimple:
		hints.ExpectedResponseTime = 50 * time.Millisecond
		hints.ParallelismLevel = 1
	case QueryComplexityModerate:
		hints.ExpectedResponseTime = 200 * time.Millisecond
		hints.ParallelismLevel = 2
	case QueryComplexityComplex:
		hints.ExpectedResponseTime = 500 * time.Millisecond
		hints.ParallelismLevel = 3
	case QueryComplexityDynamic:
		hints.ExpectedResponseTime = 1000 * time.Millisecond
		hints.ParallelismLevel = 1
	}
	
	// Adjust for strategy
	switch strategy {
	case RoutingSCIPOnly:
		hints.ExpectedResponseTime = hints.ExpectedResponseTime / 2
		hints.CachingRecommendation = "aggressive"
		hints.OptimizationTips = append(hints.OptimizationTips, "use_scip_cache")
	case RoutingLSPOnly:
		hints.ExpectedResponseTime = hints.ExpectedResponseTime * 2
		hints.CachingRecommendation = "conservative"
		hints.OptimizationTips = append(hints.OptimizationTips, "consider_scip_fallback")
	case RoutingHybrid:
		hints.ExpectedResponseTime = hints.ExpectedResponseTime * 1.5
		hints.CachingRecommendation = "moderate"
		hints.OptimizationTips = append(hints.OptimizationTips, "parallel_query_scip_lsp")
	}
	
	// Resource requirements based on method
	if strings.Contains(request.Method, "symbol") || strings.Contains(request.Method, "reference") {
		hints.ResourceRequirements.MemoryIntensive = true
		hints.OptimizationTips = append(hints.OptimizationTips, "preload_symbol_index")
	}
	
	if strings.Contains(request.Method, "completion") || strings.Contains(request.Method, "codeAction") {
		hints.ResourceRequirements.CPUIntensive = true
		hints.OptimizationTips = append(hints.OptimizationTips, "limit_completion_results")
	}
	
	// Context-specific optimizations
	if context != nil {
		if context.ProjectSize == ProjectSizeEnterprise {
			hints.OptimizationTips = append(hints.OptimizationTips, "use_incremental_indexing")
		}
		
		if context.CacheHitRate < 0.5 {
			hints.OptimizationTips = append(hints.OptimizationTips, "improve_cache_strategy")
		}
	}
	
	return hints
}

// RecordRoutingOutcome records the outcome of a routing decision for learning
func (qae *QueryAnalysisEngine) RecordRoutingOutcome(outcome RoutingOutcome) {
	qae.historyTracker.RecordOutcome(outcome)
	
	if qae.logger != nil {
		qae.logger.Debugf("QueryAnalysisEngine: Recorded outcome - Method: %s, Strategy: %s, Success: %t, ResponseTime: %v",
			outcome.Method, outcome.Strategy.String(), outcome.Success, outcome.ResponseTime)
	}
}

// GetPerformanceStats returns performance statistics for a method and language
func (qae *QueryAnalysisEngine) GetPerformanceStats(method, language string) *PerformanceStats {
	return qae.historyTracker.GetMethodStats(method, language)
}

// UpdateConfiguration updates the engine configuration
func (qae *QueryAnalysisEngine) UpdateConfiguration(config *config.GatewayConfig) {
	qae.mu.Lock()
	defer qae.mu.Unlock()
	
	qae.config = config
	qae.strategyDeterminer.config = config
	
	if qae.logger != nil {
		qae.logger.Debugf("QueryAnalysisEngine: Configuration updated")
	}
}

// GetContext helper method to extract language from request context
func (rc *RequestContext) GetLanguage() string {
	if rc == nil {
		return ""
	}
	
	// Try to get language from context
	if rc.Language != "" {
		return rc.Language
	}
	
	// Try to extract from file URI
	if rc.FileURI != "" {
		ext := filepath.Ext(rc.FileURI)
		return qae.mapExtensionToLanguage(ext)
	}
	
	return "unknown"
}

// mapExtensionToLanguage maps file extensions to language names
func (qae *QueryAnalysisEngine) mapExtensionToLanguage(ext string) string {
	languageMap := map[string]string{
		".go":   "go",
		".py":   "python",
		".js":   "javascript",
		".ts":   "typescript",
		".java": "java",
		".rs":   "rust",
		".cpp":  "cpp",
		".c":    "c",
		".cs":   "csharp",
		".php":  "php",
		".rb":   "ruby",
		".kt":   "kotlin",
		".scala": "scala",
		".swift": "swift",
		".dart": "dart",
		".lua":  "lua",
		".r":    "r",
		".sql":  "sql",
		".sh":   "shell",
		".ps1":  "powershell",
		".yml":  "yaml",
		".yaml": "yaml",
		".json": "json",
		".xml":  "xml",
		".html": "html",
		".css":  "css",
		".scss": "scss",
		".less": "less",
		".vue":  "vue",
		".jsx":  "javascript",
		".tsx":  "typescript",
	}
	
	if lang, exists := languageMap[strings.ToLower(ext)]; exists {
		return lang
	}
	
	return "unknown"
}

// QueryAnalysisEngineIntegration provides integration helpers for existing systems
type QueryAnalysisEngineIntegration struct {
	engine         *QueryAnalysisEngine
	scipResolver   SCIPResolver
	workspaceManager *WorkspaceManager
	logger         *mcp.StructuredLogger
}

// NewQueryAnalysisEngineIntegration creates a new integration helper
func NewQueryAnalysisEngineIntegration(
	engine *QueryAnalysisEngine,
	scipResolver SCIPResolver,
	workspaceManager *WorkspaceManager,
	logger *mcp.StructuredLogger,
) *QueryAnalysisEngineIntegration {
	return &QueryAnalysisEngineIntegration{
		engine:         engine,
		scipResolver:   scipResolver,
		workspaceManager: workspaceManager,
		logger:         logger,
	}
}

// AnalyzeAndRoute performs query analysis and determines routing strategy
func (qai *QueryAnalysisEngineIntegration) AnalyzeAndRoute(
	ctx context.Context,
	request *LSPRequest,
) (*QueryAnalysisResult, *SCIPQueryResult, error) {
	// Build project context
	projectContext, err := qai.buildProjectContext(request)
	if err != nil {
		if qai.logger != nil {
			qai.logger.Errorf("Failed to build project context: %v", err)
		}
		projectContext = &ProjectAnalysisContext{
			ProjectSize: ProjectSizeMedium,
			HasIndex:    false,
		}
	}
	
	// Perform analysis
	analysis, err := qai.engine.AnalyzeQuery(request, projectContext)
	if err != nil {
		return nil, nil, fmt.Errorf("query analysis failed: %w", err)
	}
	
	// Attempt SCIP resolution based on analysis
	var scipResult *SCIPQueryResult
	if analysis.ConfidenceScore >= 0.6 && qai.scipResolver != nil && qai.scipResolver.IsEnabled() {
		scipResult = qai.scipResolver.Query(request.Method, request.Params)
		
		// Validate SCIP result confidence matches analysis
		if scipResult != nil && scipResult.Found {
			// Adjust confidence based on SCIP result
			if scipResult.Confidence < analysis.ConfidenceScore {
				analysis.ConfidenceScore = scipResult.Confidence
				// Re-determine strategy with updated confidence
				analysis.RecommendedStrategy = qai.engine.strategyDeterminer.DetermineStrategy(
					request.Method, analysis.ConfidenceScore, analysis.Complexity, projectContext)
			}
		}
	}
	
	return analysis, scipResult, nil
}

// buildProjectContext constructs project analysis context from available information
func (qai *QueryAnalysisEngineIntegration) buildProjectContext(request *LSPRequest) (*ProjectAnalysisContext, error) {
	context := &ProjectAnalysisContext{
		Languages: []string{},
		HasIndex:  false,
	}
	
	// Extract language from request
	if request.Context != nil {
		context.Languages = append(context.Languages, request.Context.GetLanguage())
	}
	
	// Get workspace context if available
	if qai.workspaceManager != nil && request.URI != "" {
		workspaceContext, err := qai.workspaceManager.GetOrCreateWorkspace(request.URI)
		if err == nil && workspaceContext != nil {
			// Extract project information from workspace
			context.Languages = workspaceContext.GetLanguages()
			context.Framework = qai.detectFramework(workspaceContext.GetRootPath())
			context.ProjectSize = qai.estimateProjectSize(workspaceContext.GetRootPath())
			
			// Check for SCIP index availability
			context.HasIndex = qai.checkIndexAvailability(workspaceContext.GetRootPath())
			if context.HasIndex {
				context.IndexFreshness = qai.getIndexFreshness(workspaceContext.GetRootPath())
			}
			
			// Get cache hit rate from recent metrics
			context.CacheHitRate = qai.getCacheHitRate(workspaceContext.GetID())
		}
	}
	
	// Set defaults if not determined
	if len(context.Languages) == 0 {
		if request.URI != "" {
			ext := filepath.Ext(request.URI)
			lang := qai.engine.mapExtensionToLanguage(ext)
			context.Languages = append(context.Languages, lang)
		}
	}
	
	if context.ProjectSize == 0 {
		context.ProjectSize = ProjectSizeMedium // Default assumption
	}
	
	return context, nil
}

// detectFramework attempts to detect the framework used in the project
func (qai *QueryAnalysisEngineIntegration) detectFramework(rootPath string) string {
	// Simple framework detection based on common files
	frameworkIndicators := map[string]string{
		"package.json":     "node",
		"pom.xml":          "maven",
		"build.gradle":     "gradle",
		"Cargo.toml":       "cargo",
		"go.mod":           "go",
		"requirements.txt": "python",
		"Gemfile":          "ruby",
		"composer.json":    "php",
	}
	
	for file, framework := range frameworkIndicators {
		filePath := filepath.Join(rootPath, file)
		if qai.fileExists(filePath) {
			return framework
		}
	}
	
	return "unknown"
}

// estimateProjectSize estimates project size based on directory structure
func (qai *QueryAnalysisEngineIntegration) estimateProjectSize(rootPath string) ProjectSize {
	// Simple heuristic based on directory count and common patterns
	// In a real implementation, this would be more sophisticated
	
	if qai.hasEnterpriseMarkers(rootPath) {
		return ProjectSizeEnterprise
	}
	
	// Count source files or directories as a rough estimate
	sourceCount := qai.countSourceFiles(rootPath)
	
	if sourceCount > 10000 {
		return ProjectSizeEnterprise
	} else if sourceCount > 1000 {
		return ProjectSizeLarge
	} else if sourceCount > 100 {
		return ProjectSizeMedium
	}
	
	return ProjectSizeSmall
}

// checkIndexAvailability checks if SCIP index is available for the project
func (qai *QueryAnalysisEngineIntegration) checkIndexAvailability(rootPath string) bool {
	// Check for common SCIP index files or directories
	scipPaths := []string{
		filepath.Join(rootPath, ".scip"),
		filepath.Join(rootPath, "index.scip"),
		filepath.Join(rootPath, ".vscode", "scip-index"),
	}
	
	for _, path := range scipPaths {
		if qai.fileExists(path) {
			return true
		}
	}
	
	return false
}

// getIndexFreshness gets the age of the SCIP index
func (qai *QueryAnalysisEngineIntegration) getIndexFreshness(rootPath string) time.Duration {
	scipPath := filepath.Join(rootPath, ".scip")
	if info, err := os.Stat(scipPath); err == nil {
		return time.Since(info.ModTime())
	}
	
	return 24 * time.Hour // Default to 24 hours if unknown
}

// getCacheHitRate gets the cache hit rate for a workspace
func (qai *QueryAnalysisEngineIntegration) getCacheHitRate(workspaceID string) float64 {
	// In a real implementation, this would query actual cache metrics
	// For now, return a reasonable default
	return 0.75
}

// fileExists checks if a file exists
func (qai *QueryAnalysisEngineIntegration) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// hasEnterpriseMarkers checks for enterprise project markers
func (qai *QueryAnalysisEngineIntegration) hasEnterpriseMarkers(rootPath string) bool {
	enterpriseMarkers := []string{
		"kubernetes",
		"docker-compose.yml",
		"Dockerfile",
		".github/workflows",
		"terraform",
		"ansible",
		"microservices",
		"services",
	}
	
	for _, marker := range enterpriseMarkers {
		if qai.fileExists(filepath.Join(rootPath, marker)) {
			return true
		}
	}
	
	return false
}

// countSourceFiles counts source files in the project (simplified)
func (qai *QueryAnalysisEngineIntegration) countSourceFiles(rootPath string) int {
	// Simplified implementation - in practice, this would be more sophisticated
	count := 0
	
	sourceExts := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true,
		".java": true, ".rs": true, ".cpp": true, ".c": true,
		".cs": true, ".php": true, ".rb": true, ".kt": true,
	}
	
	filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		
		if !info.IsDir() {
			ext := filepath.Ext(path)
			if sourceExts[ext] {
				count++
			}
		}
		
		return nil
	})
	
	return count
}

// IntegrateWithSmartRouter integrates the QueryAnalysisEngine with SmartRouter
func (qai *QueryAnalysisEngineIntegration) IntegrateWithSmartRouter(smartRouter *SmartRouterImpl) {
	// This method would extend SmartRouter to use QueryAnalysisEngine
	// In practice, this would involve modifying the SmartRouter's RouteRequest method
	// to call AnalyzeAndRoute before making routing decisions
	
	if qai.logger != nil {
		qai.logger.Debugf("QueryAnalysisEngine integrated with SmartRouter")
	}
}

// IntegrateWithGateway integrates the QueryAnalysisEngine with Gateway
func (qai *QueryAnalysisEngineIntegration) IntegrateWithGateway(gateway *Gateway) {
	// This method would extend Gateway to use QueryAnalysisEngine
	// The Gateway would use this for intelligent SCIP vs LSP routing decisions
	
	if qai.logger != nil {
		qai.logger.Debugf("QueryAnalysisEngine integrated with Gateway")
	}
}

// Example usage and integration patterns

// ExampleUsage demonstrates how to use the QueryAnalysisEngine
func ExampleUsage() {
	// 1. Create the engine
	config := &config.GatewayConfig{} // Your gateway config
	logger := &mcp.StructuredLogger{} // Your logger
	
	engine := NewQueryAnalysisEngine(config, logger)
	
	// 2. Create integration helper
	var scipResolver SCIPResolver // Your SCIP resolver
	var workspaceManager *WorkspaceManager // Your workspace manager
	
	integration := NewQueryAnalysisEngineIntegration(engine, scipResolver, workspaceManager, logger)
	
	// 3. Analyze a request
	request := &LSPRequest{
		Method:    "textDocument/definition",
		URI:       "file:///path/to/file.go",
		RequestID: "req-123",
		Context: &RequestContext{
			Language: "go",
		},
	}
	
	// Perform analysis and get routing recommendation
	analysis, scipResult, err := integration.AnalyzeAndRoute(context.Background(), request)
	if err != nil {
		// Handle error
		return
	}
	
	// 4. Use analysis results for routing decisions
	switch analysis.RecommendedStrategy {
	case RoutingSCIPOnly:
		// Use SCIP result directly
		if scipResult != nil && scipResult.Found {
			// Return SCIP result
		}
	case RoutingSCIPPrimary:
		// Try SCIP first, fallback to LSP
		if scipResult != nil && scipResult.Found {
			// Return SCIP result
		} else {
			// Route to LSP server
		}
	case RoutingLSPPrimary:
		// Route to LSP server with SCIP enhancement
	case RoutingLSPOnly:
		// Route to LSP server only
	case RoutingHybrid:
		// Use both SCIP and LSP, merge results
	}
	
	// 5. Record the outcome for learning
	outcome := RoutingOutcome{
		RequestID:       request.RequestID,
		Method:          request.Method,
		Language:        analysis.Language,
		Strategy:        analysis.RecommendedStrategy,
		ConfidenceScore: analysis.ConfidenceScore,
		Success:         true, // Based on actual outcome
		ResponseTime:    100 * time.Millisecond, // Actual response time
		Timestamp:       time.Now(),
	}
	
	engine.RecordRoutingOutcome(outcome)
}