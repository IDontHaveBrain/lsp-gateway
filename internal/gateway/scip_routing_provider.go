package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"lsp-gateway/internal/indexing"
	"lsp-gateway/mcp"
)

// SCIPRoutingProvider provides a unified interface for SCIP-aware routing decisions
type SCIPRoutingProvider interface {
	// Core routing methods
	QuerySCIP(ctx context.Context, method string, params interface{}) *SCIPQueryResponse
	EstimateConfidence(method string, params interface{}) ConfidenceLevel
	GetRoutingRecommendation(method string, params interface{}) *RoutingRecommendation
	
	// Cache management
	InvalidateCache(filePath string) error
	PrewarmCache(ctx context.Context, files []string) error
	
	// Health and performance monitoring
	IsHealthy() bool
	GetPerformanceMetrics() *SCIPPerformanceMetrics
	GetHealthStatus() *SCIPHealthStatus
	
	// Configuration and lifecycle
	UpdateConfiguration(config *SCIPProviderConfig) error
	Close() error
}

// SCIPQueryResponse represents a unified SCIP query response
type SCIPQueryResponse struct {
	Found           bool                    `json:"found"`
	Response        json.RawMessage         `json:"response,omitempty"`
	Error           string                  `json:"error,omitempty"`
	Confidence      ConfidenceLevel         `json:"confidence"`
	CacheHit        bool                    `json:"cache_hit"`
	QueryTime       time.Duration           `json:"query_time"`
	Source          string                  `json:"source"` // "scip_cache", "scip_store", "fallback"
	Metadata        map[string]interface{}  `json:"metadata,omitempty"`
	
	// Routing hints
	FallbackRequired bool                   `json:"fallback_required"`
	Quality         QueryQuality            `json:"quality"`
}

// RoutingRecommendation provides intelligent routing guidance
type RoutingRecommendation struct {
	PreferredSource    RoutingSource           `json:"preferred_source"`
	Confidence         ConfidenceLevel         `json:"confidence"`
	EstimatedLatency   time.Duration           `json:"estimated_latency"`
	CacheHitProbability float64                `json:"cache_hit_probability"`
	RecommendedStrategy SCIPRoutingStrategyType `json:"recommended_strategy"`
	Reasoning          string                  `json:"reasoning"`
	FallbackOptions    []RoutingSource         `json:"fallback_options,omitempty"`
}

// QueryQuality represents the quality assessment of a query response
type QueryQuality string

const (
	QualityHigh     QueryQuality = "high"     // >95% confidence, recent data
	QualityMedium   QueryQuality = "medium"   // 80-95% confidence
	QualityLow      QueryQuality = "low"      // 60-80% confidence
	QualityUnknown  QueryQuality = "unknown"  // <60% confidence or no data
)

// SCIPPerformanceMetrics provides comprehensive performance tracking
type SCIPPerformanceMetrics struct {
	// Query statistics
	TotalQueries         int64         `json:"total_queries"`
	CacheHits           int64         `json:"cache_hits"`
	CacheMisses         int64         `json:"cache_misses"`
	CacheHitRate        float64       `json:"cache_hit_rate"`
	
	// Performance metrics
	AverageQueryTime    time.Duration `json:"average_query_time"`
	P95QueryTime        time.Duration `json:"p95_query_time"`
	P99QueryTime        time.Duration `json:"p99_query_time"`
	
	// Quality metrics
	HighQualityQueries  int64         `json:"high_quality_queries"`
	MediumQualityQueries int64        `json:"medium_quality_queries"`
	LowQualityQueries   int64         `json:"low_quality_queries"`
	AverageConfidence   float64       `json:"average_confidence"`
	
	// Error tracking
	ErrorCount          int64         `json:"error_count"`
	ErrorRate           float64       `json:"error_rate"`
	LastError           string        `json:"last_error,omitempty"`
	LastErrorTime       time.Time     `json:"last_error_time,omitempty"`
	
	// Resource utilization
	MemoryUsage         int64         `json:"memory_usage_bytes"`
	IndexesLoaded       int           `json:"indexes_loaded"`
	
	// Timing
	StartTime           time.Time     `json:"start_time"`
	LastUpdated         time.Time     `json:"last_updated"`
}

// SCIPHealthStatus provides comprehensive health monitoring
type SCIPHealthStatus struct {
	Overall             HealthStatus  `json:"overall"`
	
	// Component health
	Store               HealthStatus  `json:"store"`
	Cache               HealthStatus  `json:"cache"`
	Mapper              HealthStatus  `json:"mapper"`
	Client              HealthStatus  `json:"client"`
	
	// Performance health
	QueryPerformance    HealthStatus  `json:"query_performance"`
	MemoryHealth        HealthStatus  `json:"memory_health"`
	ErrorHealth         HealthStatus  `json:"error_health"`
	
	// Detailed status
	StatusDetails       map[string]interface{} `json:"status_details,omitempty"`
	LastHealthCheck     time.Time     `json:"last_health_check"`
	HealthCheckDuration time.Duration `json:"health_check_duration"`
}

// HealthStatus represents component health status
type HealthStatus string

const (
	HealthHealthy   HealthStatus = "healthy"
	HealthDegraded  HealthStatus = "degraded"
	HealthUnhealthy HealthStatus = "unhealthy"
	HealthUnknown   HealthStatus = "unknown"
)

// SCIPProviderConfig configures the SCIP routing provider
type SCIPProviderConfig struct {
	// Performance thresholds
	PerformanceTargets  *PerformanceTargets  `json:"performance_targets"`
	
	// Health monitoring
	HealthCheckInterval time.Duration        `json:"health_check_interval"`
	RoutingHealthThresholds    *RoutingHealthThresholds    `json:"health_thresholds"`
	
	// Cache configuration
	CacheSettings       *CacheSettings       `json:"cache_settings"`
	
	// Fallback behavior
	FallbackEnabled     bool                 `json:"fallback_enabled"`
	FallbackTimeout     time.Duration        `json:"fallback_timeout"`
	
	// Quality settings
	MinAcceptableQuality QueryQuality        `json:"min_acceptable_quality"`
	
	// Logging
	EnableMetricsLogging bool                `json:"enable_metrics_logging"`
	EnableHealthLogging  bool                `json:"enable_health_logging"`
}

// PerformanceTargets defines performance expectations
type PerformanceTargets struct {
	MaxQueryTime        time.Duration `json:"max_query_time"`
	TargetP95           time.Duration `json:"target_p95"`
	TargetP99           time.Duration `json:"target_p99"`
	MinCacheHitRate     float64       `json:"min_cache_hit_rate"`
	MaxErrorRate        float64       `json:"max_error_rate"`
}

// RoutingHealthThresholds defines health monitoring thresholds
type RoutingHealthThresholds struct {
	ErrorRateThreshold    float64       `json:"error_rate_threshold"`
	LatencyThreshold      time.Duration `json:"latency_threshold"`
	MemoryThreshold       int64         `json:"memory_threshold"`
	CacheHitRateThreshold float64       `json:"cache_hit_rate_threshold"`
}

// CacheSettings defines cache behavior
type CacheSettings struct {
	EnablePrewarming    bool          `json:"enable_prewarming"`
	PrewarmingInterval  time.Duration `json:"prewarming_interval"`
	InvalidationDelay   time.Duration `json:"invalidation_delay"`
	MaxCacheAge         time.Duration `json:"max_cache_age"`
}

// SCIPRoutingProviderImpl implements the SCIPRoutingProvider interface
type SCIPRoutingProviderImpl struct {
	// Core SCIP components
	scipStore      indexing.SCIPStore
	scipClient     *indexing.SCIPClient
	scipMapper     *indexing.LSPSCIPMapper
	
	// Performance tracking
	performanceTracker *SCIPPerformanceTracker
	healthChecker      *SCIPHealthChecker
	
	// Configuration
	config         *SCIPProviderConfig
	configMutex    sync.RWMutex
	
	// State management
	started        bool
	ctx            context.Context
	cancelFunc     context.CancelFunc
	
	// Metrics
	metrics        *SCIPPerformanceMetrics
	metricsMutex   sync.RWMutex
	
	// Logging
	logger         *mcp.StructuredLogger
}

// SCIPPerformanceTracker tracks detailed performance metrics
type SCIPPerformanceTracker struct {
	queryTimes     []time.Duration
	queryTimeMutex sync.RWMutex
	
	// Atomic counters
	totalQueries       int64
	cacheHits         int64
	cacheMisses       int64
	errorCount        int64
	highQualityCount  int64
	mediumQualityCount int64
	lowQualityCount   int64
	
	// Timing
	startTime         time.Time
	
	// Configuration
	maxSamples        int
	
	// Health tracking
	lastMetricsUpdate time.Time
}

// SCIPHealthChecker monitors component health
type SCIPHealthChecker struct {
	provider           *SCIPRoutingProviderImpl
	lastHealthStatus   *SCIPHealthStatus
	healthMutex        sync.RWMutex
	
	// Configuration
	checkInterval      time.Duration
	
	// Context
	ctx                context.Context
	cancelFunc         context.CancelFunc
}

// NewSCIPRoutingProvider creates a new unified SCIP routing provider
func NewSCIPRoutingProvider(
	scipStore indexing.SCIPStore,
	scipClient *indexing.SCIPClient,
	scipMapper *indexing.LSPSCIPMapper,
	config *SCIPProviderConfig,
	logger *mcp.StructuredLogger,
) (SCIPRoutingProvider, error) {
	if scipStore == nil {
		return nil, fmt.Errorf("SCIP store cannot be nil")
	}
	
	if config == nil {
		config = getDefaultSCIPProviderConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create performance tracker
	performanceTracker := &SCIPPerformanceTracker{
		startTime:  time.Now(),
		maxSamples: 1000,
		queryTimes: make([]time.Duration, 0, 1000),
	}
	
	// Initialize metrics
	metrics := &SCIPPerformanceMetrics{
		StartTime:   time.Now(),
		LastUpdated: time.Now(),
	}
	
	provider := &SCIPRoutingProviderImpl{
		scipStore:          scipStore,
		scipClient:         scipClient,
		scipMapper:         scipMapper,
		performanceTracker: performanceTracker,
		config:             config,
		ctx:                ctx,
		cancelFunc:         cancel,
		metrics:            metrics,
		logger:             logger,
	}
	
	// Create health checker
	provider.healthChecker = &SCIPHealthChecker{
		provider:      provider,
		checkInterval: config.HealthCheckInterval,
		ctx:           ctx,
	}
	
	// Start background processes
	if err := provider.start(); err != nil {
		return nil, fmt.Errorf("failed to start SCIP routing provider: %w", err)
	}
	
	if logger != nil {
		logger.Infof("SCIP Routing Provider initialized successfully")
	}
	
	return provider, nil
}

// QuerySCIP performs a unified SCIP query with comprehensive error handling
func (p *SCIPRoutingProviderImpl) QuerySCIP(ctx context.Context, method string, params interface{}) *SCIPQueryResponse {
	startTime := time.Now()
	
	// Track query
	atomic.AddInt64(&p.performanceTracker.totalQueries, 1)
	
	defer func() {
		queryTime := time.Since(startTime)
		p.recordQueryTime(queryTime)
		p.updateMetrics()
	}()
	
	response := &SCIPQueryResponse{
		QueryTime: 0, // Will be set in defer
		Metadata:  make(map[string]interface{}),
	}
	
	// Set query timeout
	if deadline, ok := ctx.Deadline(); ok {
		response.Metadata["timeout"] = deadline.Sub(time.Now())
	}
	
	// Check if provider is healthy
	if !p.IsHealthy() {
		response.Error = "SCIP provider is unhealthy"
		response.FallbackRequired = true
		response.Quality = QualityUnknown
		p.recordError("Provider unhealthy")
		return response
	}
	
	// Query SCIP store
	if p.scipStore != nil {
		scipResult := p.scipStore.Query(method, params)
		
		response.Found = scipResult.Found
		response.Response = scipResult.Response
		response.CacheHit = scipResult.CacheHit
		response.Source = "scip_store"
		
		if scipResult.Error != "" {
			response.Error = scipResult.Error
			p.recordError(scipResult.Error)
		}
		
		// Calculate confidence using mapper if available
		if p.scipMapper != nil && scipResult.Found {
			confidence := p.scipMapper.QuerySCIP(method, params).Confidence
			response.Confidence = ConfidenceLevel(confidence)
		} else {
			response.Confidence = ConfidenceLevel(scipResult.Confidence)
		}
		
		// Determine quality
		response.Quality = p.assessQuality(response.Confidence, scipResult.CacheHit, scipResult.Error)
		
		// Set fallback recommendation
		minAcceptableConfidence := queryQualityToConfidenceLevel(p.config.MinAcceptableQuality)
		response.FallbackRequired = !scipResult.Found || 
			response.Confidence < minAcceptableConfidence ||
			scipResult.Error != ""
			
		// Update cache hit metrics
		if scipResult.CacheHit {
			atomic.AddInt64(&p.performanceTracker.cacheHits, 1)
		} else {
			atomic.AddInt64(&p.performanceTracker.cacheMisses, 1)
		}
		
		// Record quality metrics
		p.recordQualityMetrics(response.Quality)
		
		response.Metadata["scip_result"] = map[string]interface{}{
			"method":      scipResult.Method,
			"index_path":  scipResult.IndexPath,
			"query_time":  scipResult.QueryTime,
		}
	} else {
		response.Error = "SCIP store not available"
		response.FallbackRequired = true
		response.Quality = QualityUnknown
		response.Source = "unavailable"
		p.recordError("SCIP store unavailable")
	}
	
	response.QueryTime = time.Since(startTime)
	
	if p.logger != nil && p.config.EnableMetricsLogging {
		p.logger.Debugf("SCIP query %s completed in %v (found: %t, confidence: %.2f, quality: %s)",
			method, response.QueryTime, response.Found, response.Confidence, response.Quality)
	}
	
	return response
}

// EstimateConfidence estimates confidence for a query without executing it
func (p *SCIPRoutingProviderImpl) EstimateConfidence(method string, params interface{}) ConfidenceLevel {
	if p.scipMapper != nil {
		// Use mapper's confidence estimation
		lspParams, err := p.scipMapper.ParseLSPParams(method, params)
		if err != nil {
			return ConfidenceMinimum
		}
		
		analysis := &QueryAnalysis{
			Method:       method,
			IsSymbolQuery: isSymbolMethod(method),
			RequiresRuntime: requiresRuntimeMethod(method),
			CacheEligible: isCacheEligibleMethod(method),
			Complexity:    estimateMethodComplexity(method),
		}
		
		if lspParams.URI != "" {
			analysis.FileURI = lspParams.URI
		}
		
		// Estimate based on method characteristics and historical performance
		baseConfidence := getBaseConfidenceForMethod(method)
		
		// Adjust based on analysis
		if analysis.IsSymbolQuery && !analysis.RequiresRuntime {
			baseConfidence += 0.1
		}
		if analysis.CacheEligible {
			baseConfidence += 0.05
		}
		if analysis.Complexity <= ComplexityModerate {
			baseConfidence += 0.05
		}
		
		// Historical performance adjustment
		if performance := p.getMethodPerformance(method); performance != nil {
			historyAdjustment := (performance.AverageConfidence - 0.5) * 0.1
			baseConfidence = ConfidenceLevel(float64(baseConfidence) + historyAdjustment)
		}
		
		// Clamp to valid range
		if baseConfidence < ConfidenceMinimum {
			return ConfidenceMinimum
		}
		if baseConfidence > 1.0 {
			return 1.0
		}
		
		return ConfidenceLevel(baseConfidence)
	}
	
	// Fallback estimation based on method type
	return getBaseConfidenceForMethod(method)
}

// GetRoutingRecommendation provides intelligent routing guidance
func (p *SCIPRoutingProviderImpl) GetRoutingRecommendation(method string, params interface{}) *RoutingRecommendation {
	confidence := p.EstimateConfidence(method, params)
	
	recommendation := &RoutingRecommendation{
		Confidence: confidence,
		Reasoning:  "",
	}
	
	// Determine preferred source based on confidence and method characteristics
	if confidence >= ConfidenceHigh && isSymbolMethod(method) && !requiresRuntimeMethod(method) {
		recommendation.PreferredSource = SourceSCIPCache
		recommendation.RecommendedStrategy = SCIPCacheFirst
		recommendation.Reasoning = "High confidence static analysis suitable for cache"
		recommendation.EstimatedLatency = 5 * time.Millisecond
		recommendation.CacheHitProbability = 0.8
		recommendation.FallbackOptions = []RoutingSource{SourceLSPServer}
		
	} else if confidence >= ConfidenceMedium && isCacheEligibleMethod(method) {
		recommendation.PreferredSource = SourceHybrid
		recommendation.RecommendedStrategy = SCIPHybrid
		recommendation.Reasoning = "Medium confidence - use hybrid approach for optimal results"
		recommendation.EstimatedLatency = 15 * time.Millisecond
		recommendation.CacheHitProbability = 0.6
		recommendation.FallbackOptions = []RoutingSource{SourceLSPServer}
		
	} else {
		recommendation.PreferredSource = SourceLSPServer
		recommendation.RecommendedStrategy = SCIPLSPFirst
		recommendation.Reasoning = "Low confidence or runtime-dependent - prefer LSP"
		recommendation.EstimatedLatency = 100 * time.Millisecond
		recommendation.CacheHitProbability = 0.2
	}
	
	// Adjust based on current health
	if !p.IsHealthy() {
		recommendation.PreferredSource = SourceLSPServer
		recommendation.RecommendedStrategy = SCIPDisabled
		recommendation.Reasoning = "SCIP provider unhealthy - use LSP only"
		recommendation.EstimatedLatency = 200 * time.Millisecond
		recommendation.CacheHitProbability = 0.0
		recommendation.FallbackOptions = nil
	}
	
	return recommendation
}

// InvalidateCache invalidates cache entries for a specific file
func (p *SCIPRoutingProviderImpl) InvalidateCache(filePath string) error {
	if p.scipStore != nil {
		p.scipStore.InvalidateFile(filePath)
		
		if p.logger != nil {
			p.logger.Debugf("Invalidated SCIP cache for file: %s", filePath)
		}
	}
	
	return nil
}

// PrewarmCache preloads cache with data for specified files
func (p *SCIPRoutingProviderImpl) PrewarmCache(ctx context.Context, files []string) error {
	if !p.config.CacheSettings.EnablePrewarming || p.scipStore == nil {
		return nil
	}
	
	// Implementation would preload common queries for the specified files
	// This is a placeholder for the actual prewarming logic
	
	if p.logger != nil {
		p.logger.Infof("Prewarming SCIP cache for %d files", len(files))
	}
	
	return nil
}

// IsHealthy checks if the provider is in a healthy state
func (p *SCIPRoutingProviderImpl) IsHealthy() bool {
	p.healthChecker.healthMutex.RLock()
	defer p.healthChecker.healthMutex.RUnlock()
	
	if p.healthChecker.lastHealthStatus == nil {
		return false
	}
	
	return p.healthChecker.lastHealthStatus.Overall == HealthHealthy ||
		   p.healthChecker.lastHealthStatus.Overall == HealthDegraded
}

// GetPerformanceMetrics returns current performance metrics
func (p *SCIPRoutingProviderImpl) GetPerformanceMetrics() *SCIPPerformanceMetrics {
	p.metricsMutex.RLock()
	defer p.metricsMutex.RUnlock()
	
	// Create a deep copy to avoid race conditions
	metrics := &SCIPPerformanceMetrics{
		TotalQueries:         atomic.LoadInt64(&p.performanceTracker.totalQueries),
		CacheHits:           atomic.LoadInt64(&p.performanceTracker.cacheHits),
		CacheMisses:         atomic.LoadInt64(&p.performanceTracker.cacheMisses),
		HighQualityQueries:  atomic.LoadInt64(&p.performanceTracker.highQualityCount),
		MediumQualityQueries: atomic.LoadInt64(&p.performanceTracker.mediumQualityCount),
		LowQualityQueries:   atomic.LoadInt64(&p.performanceTracker.lowQualityCount),
		ErrorCount:          atomic.LoadInt64(&p.performanceTracker.errorCount),
		StartTime:           p.performanceTracker.startTime,
		LastUpdated:         time.Now(),
	}
	
	// Calculate derived metrics
	if metrics.TotalQueries > 0 {
		metrics.CacheHitRate = float64(metrics.CacheHits) / float64(metrics.TotalQueries)
		metrics.ErrorRate = float64(metrics.ErrorCount) / float64(metrics.TotalQueries)
		
		totalQuality := metrics.HighQualityQueries + metrics.MediumQualityQueries + metrics.LowQualityQueries
		if totalQuality > 0 {
			metrics.AverageConfidence = (float64(metrics.HighQualityQueries)*0.95 + 
				float64(metrics.MediumQualityQueries)*0.8 + 
				float64(metrics.LowQualityQueries)*0.6) / float64(totalQuality)
		}
	}
	
	// Calculate timing metrics
	p.performanceTracker.queryTimeMutex.RLock()
	if len(p.performanceTracker.queryTimes) > 0 {
		metrics.AverageQueryTime = p.calculateAverageQueryTime()
		metrics.P95QueryTime = p.calculatePercentile(0.95)
		metrics.P99QueryTime = p.calculatePercentile(0.99)
	}
	p.performanceTracker.queryTimeMutex.RUnlock()
	
	// Get memory usage from store
	if p.scipStore != nil {
		storeStats := p.scipStore.GetStats()
		metrics.MemoryUsage = storeStats.MemoryUsage
		metrics.IndexesLoaded = storeStats.IndexesLoaded
	}
	
	return metrics
}

// GetHealthStatus returns current health status
func (p *SCIPRoutingProviderImpl) GetHealthStatus() *SCIPHealthStatus {
	p.healthChecker.healthMutex.RLock()
	defer p.healthChecker.healthMutex.RUnlock()
	
	if p.healthChecker.lastHealthStatus != nil {
		// Return a copy
		status := *p.healthChecker.lastHealthStatus
		return &status
	}
	
	// Return default unhealthy status
	return &SCIPHealthStatus{
		Overall:         HealthUnknown,
		Store:           HealthUnknown,
		Cache:           HealthUnknown,
		Mapper:          HealthUnknown,
		Client:          HealthUnknown,
		QueryPerformance: HealthUnknown,
		MemoryHealth:    HealthUnknown,
		ErrorHealth:     HealthUnknown,
		LastHealthCheck: time.Now(),
	}
}

// UpdateConfiguration updates the provider configuration
func (p *SCIPRoutingProviderImpl) UpdateConfiguration(config *SCIPProviderConfig) error {
	p.configMutex.Lock()
	defer p.configMutex.Unlock()
	
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}
	
	p.config = config
	
	// Update health checker interval
	if p.healthChecker != nil {
		p.healthChecker.checkInterval = config.HealthCheckInterval
	}
	
	if p.logger != nil {
		p.logger.Infof("SCIP routing provider configuration updated")
	}
	
	return nil
}

// Close cleans up resources and shuts down the provider
func (p *SCIPRoutingProviderImpl) Close() error {
	if p.cancelFunc != nil {
		p.cancelFunc()
	}
	
	var errs []error
	
	// Close SCIP store
	if p.scipStore != nil {
		if err := p.scipStore.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close SCIP store: %w", err))
		}
	}
	
	// Close SCIP client
	if p.scipClient != nil {
		if err := p.scipClient.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close SCIP client: %w", err))
		}
	}
	
	// Close mapper
	if p.scipMapper != nil {
		if err := p.scipMapper.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close SCIP mapper: %w", err))
		}
	}
	
	if p.logger != nil {
		p.logger.Infof("SCIP routing provider closed")
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("errors during close: %v", errs)
	}
	
	return nil
}

// Helper methods

// start initializes background processes
func (p *SCIPRoutingProviderImpl) start() error {
	p.started = true
	
	// Start health checker
	go p.healthChecker.runHealthChecks()
	
	// Start periodic metrics update
	go p.runMetricsUpdater()
	
	return nil
}

// runMetricsUpdater periodically updates metrics
func (p *SCIPRoutingProviderImpl) runMetricsUpdater() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			p.updateMetrics()
		case <-p.ctx.Done():
			return
		}
	}
}

// updateMetrics updates cached metrics
func (p *SCIPRoutingProviderImpl) updateMetrics() {
	p.metricsMutex.Lock()
	defer p.metricsMutex.Unlock()
	
	p.metrics.LastUpdated = time.Now()
	p.performanceTracker.lastMetricsUpdate = time.Now()
}

// recordQueryTime records a query execution time
func (p *SCIPRoutingProviderImpl) recordQueryTime(duration time.Duration) {
	p.performanceTracker.queryTimeMutex.Lock()
	defer p.performanceTracker.queryTimeMutex.Unlock()
	
	p.performanceTracker.queryTimes = append(p.performanceTracker.queryTimes, duration)
	
	// Keep only recent samples
	if len(p.performanceTracker.queryTimes) > p.performanceTracker.maxSamples {
		p.performanceTracker.queryTimes = p.performanceTracker.queryTimes[1:]
	}
}

// recordError records an error
func (p *SCIPRoutingProviderImpl) recordError(errorMsg string) {
	atomic.AddInt64(&p.performanceTracker.errorCount, 1)
	
	p.metricsMutex.Lock()
	p.metrics.LastError = errorMsg
	p.metrics.LastErrorTime = time.Now()
	p.metricsMutex.Unlock()
}

// recordQualityMetrics records quality metrics
func (p *SCIPRoutingProviderImpl) recordQualityMetrics(quality QueryQuality) {
	switch quality {
	case QualityHigh:
		atomic.AddInt64(&p.performanceTracker.highQualityCount, 1)
	case QualityMedium:
		atomic.AddInt64(&p.performanceTracker.mediumQualityCount, 1)
	case QualityLow:
		atomic.AddInt64(&p.performanceTracker.lowQualityCount, 1)
	}
}

// queryQualityToConfidenceLevel converts QueryQuality to ConfidenceLevel
func queryQualityToConfidenceLevel(quality QueryQuality) ConfidenceLevel {
	switch quality {
	case QualityHigh:
		return ConfidenceHigh
	case QualityMedium:
		return ConfidenceMedium
	case QualityLow:
		return ConfidenceMinimum
	default:
		return ConfidenceMinimum
	}
}

// assessQuality determines the quality of a query response
func (p *SCIPRoutingProviderImpl) assessQuality(confidence ConfidenceLevel, cacheHit bool, errorMsg string) QueryQuality {
	if errorMsg != "" {
		return QualityUnknown
	}
	
	if confidence >= ConfidenceHigh {
		return QualityHigh
	} else if confidence >= ConfidenceMedium {
		return QualityMedium
	} else if confidence >= ConfidenceMinimum {
		return QualityLow
	}
	
	return QualityUnknown
}

// calculateAverageQueryTime calculates average query time
func (p *SCIPRoutingProviderImpl) calculateAverageQueryTime() time.Duration {
	if len(p.performanceTracker.queryTimes) == 0 {
		return 0
	}
	
	var total time.Duration
	for _, t := range p.performanceTracker.queryTimes {
		total += t
	}
	
	return total / time.Duration(len(p.performanceTracker.queryTimes))
}

// calculatePercentile calculates the nth percentile of query times
func (p *SCIPRoutingProviderImpl) calculatePercentile(percentile float64) time.Duration {
	if len(p.performanceTracker.queryTimes) == 0 {
		return 0
	}
	
	// Create a sorted copy
	times := make([]time.Duration, len(p.performanceTracker.queryTimes))
	copy(times, p.performanceTracker.queryTimes)
	
	// Simple sort - for production should use sort.Slice
	for i := 0; i < len(times); i++ {
		for j := i + 1; j < len(times); j++ {
			if times[i] > times[j] {
				times[i], times[j] = times[j], times[i]
			}
		}
	}
	
	index := int(float64(len(times)-1) * percentile)
	return times[index]
}

// getMethodPerformance returns performance data for a method
func (p *SCIPRoutingProviderImpl) getMethodPerformance(method string) *MethodPerformanceData {
	// This would be implemented with actual performance tracking
	// For now, return nil
	return nil
}

// MethodPerformanceData tracks performance for specific methods
type MethodPerformanceData struct {
	AverageConfidence float64
	AverageLatency   time.Duration
	SuccessRate      float64
}

// Helper functions for method classification

func isSymbolMethod(method string) bool {
	symbolMethods := map[string]bool{
		"textDocument/definition":      true,
		"textDocument/references":      true,
		"textDocument/documentSymbol": true,
		"workspace/symbol":            true,
		"textDocument/hover":          true,
	}
	return symbolMethods[method]
}

func requiresRuntimeMethod(method string) bool {
	runtimeMethods := map[string]bool{
		"textDocument/completion": true,
		"textDocument/diagnostic": true,
		"textDocument/codeAction": true,
	}
	return runtimeMethods[method]
}

func isCacheEligibleMethod(method string) bool {
	cacheEligible := map[string]bool{
		"textDocument/definition":      true,
		"textDocument/references":      true,
		"textDocument/documentSymbol": true,
		"workspace/symbol":            true,
		"textDocument/hover":          true,
	}
	return cacheEligible[method]
}

func estimateMethodComplexity(method string) QueryComplexity {
	complexities := map[string]QueryComplexity{
		"textDocument/hover":          ComplexitySimple,
		"textDocument/definition":     ComplexitySimple,
		"textDocument/references":     ComplexityModerate,
		"textDocument/documentSymbol": ComplexityModerate,
		"workspace/symbol":           ComplexityComplex,
		"textDocument/completion":    ComplexityDynamic,
		"textDocument/diagnostic":    ComplexityDynamic,
	}
	
	if complexity, exists := complexities[method]; exists {
		return complexity
	}
	return ComplexityModerate
}

func getBaseConfidenceForMethod(method string) ConfidenceLevel {
	confidences := map[string]ConfidenceLevel{
		"textDocument/hover":          ConfidenceHigh,
		"textDocument/definition":     ConfidenceHigh,
		"textDocument/documentSymbol": ConfidenceMedium,
		"textDocument/references":     ConfidenceMedium,
		"workspace/symbol":           ConfidenceMedium,
	}
	
	if confidence, exists := confidences[method]; exists {
		return confidence
	}
	return ConfidenceMinimum
}

// getDefaultSCIPProviderConfig returns default configuration
func getDefaultSCIPProviderConfig() *SCIPProviderConfig {
	return &SCIPProviderConfig{
		PerformanceTargets: &PerformanceTargets{
			MaxQueryTime:    50 * time.Millisecond,
			TargetP95:       25 * time.Millisecond,
			TargetP99:       50 * time.Millisecond,
			MinCacheHitRate: 0.6,
			MaxErrorRate:    0.1,
		},
		HealthCheckInterval: 30 * time.Second,
		RoutingHealthThresholds: &RoutingHealthThresholds{
			ErrorRateThreshold:    0.1,
			LatencyThreshold:      100 * time.Millisecond,
			MemoryThreshold:       1024 * 1024 * 1024, // 1GB
			CacheHitRateThreshold: 0.4,
		},
		CacheSettings: &CacheSettings{
			EnablePrewarming:   true,
			PrewarmingInterval: 5 * time.Minute,
			InvalidationDelay:  100 * time.Millisecond,
			MaxCacheAge:        30 * time.Minute,
		},
		FallbackEnabled:      true,
		FallbackTimeout:      5 * time.Second,
		MinAcceptableQuality: QualityLow,
		EnableMetricsLogging: true,
		EnableHealthLogging:  true,
	}
}

// runHealthChecks performs periodic health monitoring
func (hc *SCIPHealthChecker) runHealthChecks() {
	ticker := time.NewTicker(hc.checkInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			hc.performHealthCheck()
		case <-hc.ctx.Done():
			return
		}
	}
}

// performHealthCheck executes a comprehensive health check
func (hc *SCIPHealthChecker) performHealthCheck() {
	startTime := time.Now()
	
	status := &SCIPHealthStatus{
		StatusDetails:       make(map[string]interface{}),
		LastHealthCheck:     startTime,
		HealthCheckDuration: 0, // Will be set at the end
	}
	
	// Check SCIP store health
	status.Store = hc.checkStoreHealth()
	
	// Check cache health
	status.Cache = hc.checkCacheHealth()
	
	// Check mapper health
	status.Mapper = hc.checkMapperHealth()
	
	// Check client health
	status.Client = hc.checkClientHealth()
	
	// Check performance health
	status.QueryPerformance = hc.checkPerformanceHealth()
	
	// Check memory health
	status.MemoryHealth = hc.checkMemoryHealth()
	
	// Check error health
	status.ErrorHealth = hc.checkErrorHealth()
	
	// Determine overall health
	status.Overall = hc.calculateOverallHealth(status)
	
	status.HealthCheckDuration = time.Since(startTime)
	
	// Update cached status
	hc.healthMutex.Lock()
	hc.lastHealthStatus = status
	hc.healthMutex.Unlock()
	
	// Log health status if configured
	if hc.provider.config.EnableHealthLogging && hc.provider.logger != nil {
		hc.provider.logger.Debugf("SCIP health check completed: overall=%s, duration=%v", 
			status.Overall, status.HealthCheckDuration)
	}
}

func (hc *SCIPHealthChecker) checkStoreHealth() HealthStatus {
	if hc.provider.scipStore == nil {
		return HealthUnhealthy
	}
	
	stats := hc.provider.scipStore.GetStats()
	
	// Check if store has loaded indices
	if stats.IndexesLoaded == 0 {
		return HealthDegraded
	}
	
	// Check error rate
	if stats.TotalQueries > 0 {
		// Note: Error rate calculation would need to be added to SCIPStoreStats
		// For now, assume healthy if we have queries
		return HealthHealthy
	}
	
	return HealthHealthy
}

func (hc *SCIPHealthChecker) checkCacheHealth() HealthStatus {
	if hc.provider.scipStore == nil {
		return HealthUnhealthy
	}
	
	stats := hc.provider.scipStore.GetStats()
	
	// Check cache hit rate
	if stats.TotalQueries > 10 && stats.CacheHitRate < hc.provider.config.RoutingHealthThresholds.CacheHitRateThreshold {
		return HealthDegraded
	}
	
	return HealthHealthy
}

func (hc *SCIPHealthChecker) checkMapperHealth() HealthStatus {
	if hc.provider.scipMapper == nil {
		return HealthDegraded // Mapper is optional
	}
	
	// Check mapper stats
	stats := hc.provider.scipMapper.GetStats()
	
	if stats.TotalRequests > 0 && stats.FailedRequests > 0 {
		errorRate := float64(stats.FailedRequests) / float64(stats.TotalRequests)
		if errorRate > hc.provider.config.RoutingHealthThresholds.ErrorRateThreshold {
			return HealthDegraded
		}
	}
	
	return HealthHealthy
}

func (hc *SCIPHealthChecker) checkClientHealth() HealthStatus {
	if hc.provider.scipClient == nil {
		return HealthDegraded // Client is optional
	}
	
	// Check if client is healthy
	if !hc.provider.scipClient.IsHealthy() {
		return HealthUnhealthy
	}
	
	return HealthHealthy
}

func (hc *SCIPHealthChecker) checkPerformanceHealth() HealthStatus {
	metrics := hc.provider.GetPerformanceMetrics()
	
	// Check P95 latency
	if metrics.P95QueryTime > hc.provider.config.PerformanceTargets.TargetP95 {
		return HealthDegraded
	}
	
	// Check average latency
	if metrics.AverageQueryTime > hc.provider.config.RoutingHealthThresholds.LatencyThreshold {
		return HealthDegraded
	}
	
	return HealthHealthy
}

func (hc *SCIPHealthChecker) checkMemoryHealth() HealthStatus {
	metrics := hc.provider.GetPerformanceMetrics()
	
	// Check memory usage
	if metrics.MemoryUsage > hc.provider.config.RoutingHealthThresholds.MemoryThreshold {
		return HealthDegraded
	}
	
	return HealthHealthy
}

func (hc *SCIPHealthChecker) checkErrorHealth() HealthStatus {
	metrics := hc.provider.GetPerformanceMetrics()
	
	// Check error rate
	if metrics.ErrorRate > hc.provider.config.RoutingHealthThresholds.ErrorRateThreshold {
		return HealthDegraded
	}
	
	return HealthHealthy
}

func (hc *SCIPHealthChecker) calculateOverallHealth(status *SCIPHealthStatus) HealthStatus {
	// Count health statuses
	unhealthyCount := 0
	degradedCount := 0
	healthyCount := 0
	
	statuses := []HealthStatus{
		status.Store,
		status.Cache,
		status.Mapper,
		status.Client,
		status.QueryPerformance,
		status.MemoryHealth,
		status.ErrorHealth,
	}
	
	for _, s := range statuses {
		switch s {
		case HealthUnhealthy:
			unhealthyCount++
		case HealthDegraded:
			degradedCount++
		case HealthHealthy:
			healthyCount++
		}
	}
	
	// Determine overall health
	if unhealthyCount > 0 {
		return HealthUnhealthy
	} else if degradedCount > 2 {
		return HealthDegraded
	} else if degradedCount > 0 {
		return HealthDegraded
	}
	
	return HealthHealthy
}

// Ensure SCIPRoutingProviderImpl implements SCIPRoutingProvider interface at compile time
var _ SCIPRoutingProvider = (*SCIPRoutingProviderImpl)(nil)