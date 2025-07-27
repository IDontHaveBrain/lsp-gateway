package gateway

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"lsp-gateway/mcp"
)

// PerformanceCache provides intelligent caching and performance monitoring
type PerformanceCache interface {
	// Response caching
	CacheResponse(key string, response interface{}, ttl time.Duration) error
	GetCachedResponse(key string) (interface{}, bool)
	InvalidateCache(pattern string) error

	// Performance monitoring
	RecordRequestMetrics(serverName, method string, responseTime time.Duration, success bool)
	RecordRoutingDecision(method, serverName string, strategy interface{})
	GetServerMetrics(serverName string) *ServerMetrics
	GetPerformanceMethodMetrics(method string) *CacheMethodMetrics

	// Health monitoring
	UpdateServerHealth(serverName string, healthScore float64)
	GetServerHealth(serverName string) float64
	IsServerHealthy(serverName string) bool

	// Cache optimization
	OptimizeCache() error
	GetCacheStats() *PerformanceCacheStats

	// Lifecycle
	Start(ctx context.Context) error
	Stop() error
}

// PerformanceCacheEntry represents a cached response with metadata
type PerformanceCacheEntry struct {
	Response        interface{}   `json:"response"`
	Timestamp       time.Time     `json:"timestamp"`
	TTL             time.Duration `json:"ttl"`
	AccessCount     int64         `json:"access_count"`
	LastAccess      time.Time     `json:"last_access"`
	Size            int64         `json:"size"`
	CompressionType string        `json:"compression_type"`
	CompressedData  []byte        `json:"compressed_data,omitempty"`
	Priority        int           `json:"priority"`
}

// ServerMetrics tracks comprehensive server performance data
type ServerMetrics struct {
	ServerName          string          `json:"server_name"`
	TotalRequests       int64           `json:"total_requests"`
	SuccessfulRequests  int64           `json:"successful_requests"`
	FailedRequests      int64           `json:"failed_requests"`
	AverageResponseTime time.Duration   `json:"average_response_time"`
	MinResponseTime     time.Duration   `json:"min_response_time"`
	MaxResponseTime     time.Duration   `json:"max_response_time"`
	P50ResponseTime     time.Duration   `json:"p50_response_time"`
	P95ResponseTime     time.Duration   `json:"p95_response_time"`
	P99ResponseTime     time.Duration   `json:"p99_response_time"`
	LastRequestTime     time.Time       `json:"last_request_time"`
	HealthScore         float64         `json:"health_score"`
	CircuitBreakerState string          `json:"circuit_breaker_state"`
	LoadScore           float64         `json:"load_score"`
	TrendScore          float64         `json:"trend_score"`
	ActiveConnections   int32           `json:"active_connections"`
	SuccessRate         float64         `json:"success_rate"`
	ResponseTimes       []time.Duration `json:"-"` // Circular buffer for percentiles
	ResponseTimeIndex   int             `json:"-"`
}

// GetErrorRate calculates the error rate as a float between 0 and 1
func (sm *ServerMetrics) GetErrorRate() float64 {
	if sm.TotalRequests == 0 {
		return 0.0
	}
	return float64(sm.FailedRequests) / float64(sm.TotalRequests)
}

// NewServerMetrics creates a new ServerMetrics instance
func NewServerMetrics() *ServerMetrics {
	return &ServerMetrics{
		MinResponseTime:     time.Duration(math.MaxInt64),
		ResponseTimes:       make([]time.Duration, 1000), // Default buffer size
		ResponseTimeIndex:   0,
		HealthScore:         1.0,
		CircuitBreakerState: "CLOSED",
	}
}

// RecordRequest records a request with response time and success status
func (sm *ServerMetrics) RecordRequest(responseTime time.Duration, success bool) {
	sm.TotalRequests++
	sm.LastRequestTime = time.Now()

	if success {
		sm.SuccessfulRequests++
	} else {
		sm.FailedRequests++
	}

	// Update min/max response times
	if responseTime < sm.MinResponseTime {
		sm.MinResponseTime = responseTime
	}
	if responseTime > sm.MaxResponseTime {
		sm.MaxResponseTime = responseTime
	}

	// Update circular buffer for percentile calculation
	if len(sm.ResponseTimes) > 0 {
		sm.ResponseTimes[sm.ResponseTimeIndex] = responseTime
		sm.ResponseTimeIndex = (sm.ResponseTimeIndex + 1) % len(sm.ResponseTimes)
	}

	// Calculate exponential moving average
	alpha := 0.1
	if sm.AverageResponseTime == 0 {
		sm.AverageResponseTime = responseTime
	} else {
		newAvg := time.Duration(float64(sm.AverageResponseTime)*(1-alpha) + float64(responseTime)*alpha)
		sm.AverageResponseTime = newAvg
	}

	// Calculate percentiles
	sm.calculatePercentiles()
}

// GetAverageResponseTime returns the average response time
func (sm *ServerMetrics) GetAverageResponseTime() time.Duration {
	return sm.AverageResponseTime
}

// calculatePercentiles calculates P50, P95, and P99 response times
func (sm *ServerMetrics) calculatePercentiles() {
	times := make([]time.Duration, 0, len(sm.ResponseTimes))
	for _, t := range sm.ResponseTimes {
		if t > 0 {
			times = append(times, t)
		}
	}

	if len(times) == 0 {
		return
	}

	sort.Slice(times, func(i, j int) bool {
		return times[i] < times[j]
	})

	p50Index := len(times) * 50 / 100
	p95Index := len(times) * 95 / 100
	p99Index := len(times) * 99 / 100

	if p50Index < len(times) {
		sm.P50ResponseTime = times[p50Index]
	}
	if p95Index < len(times) {
		sm.P95ResponseTime = times[p95Index]
	}
	if p99Index < len(times) {
		sm.P99ResponseTime = times[p99Index]
	}
}

// Copy creates a deep copy of the ServerMetrics struct
func (sm *ServerMetrics) Copy() *ServerMetrics {
	if sm == nil {
		return nil
	}

	// Create a new ResponseTimes slice if it exists
	var responseTimesCopy []time.Duration
	if sm.ResponseTimes != nil {
		responseTimesCopy = make([]time.Duration, len(sm.ResponseTimes))
		copy(responseTimesCopy, sm.ResponseTimes)
	}

	return &ServerMetrics{
		ServerName:          sm.ServerName,
		TotalRequests:       sm.TotalRequests,
		SuccessfulRequests:  sm.SuccessfulRequests,
		FailedRequests:      sm.FailedRequests,
		AverageResponseTime: sm.AverageResponseTime,
		MinResponseTime:     sm.MinResponseTime,
		MaxResponseTime:     sm.MaxResponseTime,
		P50ResponseTime:     sm.P50ResponseTime,
		P95ResponseTime:     sm.P95ResponseTime,
		P99ResponseTime:     sm.P99ResponseTime,
		LastRequestTime:     sm.LastRequestTime,
		HealthScore:         sm.HealthScore,
		CircuitBreakerState: sm.CircuitBreakerState,
		LoadScore:           sm.LoadScore,
		TrendScore:          sm.TrendScore,
		ResponseTimes:       responseTimesCopy,
		ResponseTimeIndex:   sm.ResponseTimeIndex,
	}
}

// CacheMethodMetrics aggregates performance data by LSP method for caching
type CacheMethodMetrics struct {
	Method              string             `json:"method"`
	TotalRequests       int64              `json:"total_requests"`
	AverageResponseTime time.Duration      `json:"average_response_time"`
	CacheHitRate        float64            `json:"cache_hit_rate"`
	PopularServers      []string           `json:"popular_servers"`
	OptimalStrategy     string             `json:"optimal_strategy"`
	ServerPerformance   map[string]float64 `json:"server_performance"`
	OptimalTTL          time.Duration      `json:"optimal_ttl"`
	LastOptimization    time.Time          `json:"last_optimization"`
}

// PerformanceCacheStats provides comprehensive cache performance metrics
type PerformanceCacheStats struct {
	TotalEntries      int64     `json:"total_entries"`
	CacheHitRate      float64   `json:"cache_hit_rate"`
	MemoryUsage       int64     `json:"memory_usage"`
	EvictionCount     int64     `json:"eviction_count"`
	OptimizationRuns  int64     `json:"optimization_runs"`
	CompressionRatio  float64   `json:"compression_ratio"`
	L1Entries         int64     `json:"l1_entries"`
	L2Entries         int64     `json:"l2_entries"`
	HotDataPercentage float64   `json:"hot_data_percentage"`
	LastOptimization  time.Time `json:"last_optimization"`
}

// PerformanceCacheConfig holds configuration for performance cache
type PerformanceCacheConfig struct {
	MaxMemoryMB          int64         `json:"max_memory_mb"`
	DefaultTTL           time.Duration `json:"default_ttl"`
	MaxCacheEntries      int           `json:"max_cache_entries"`
	CompressionEnabled   bool          `json:"compression_enabled"`
	OptimizationInterval time.Duration `json:"optimization_interval"`
	HealthThreshold      float64       `json:"health_threshold"`
	ResponseTimeBuffer   int           `json:"response_time_buffer"`
	EnablePrediction     bool          `json:"enable_prediction"`
}

// IntelligentPerformanceCache implements PerformanceCache with advanced features
type IntelligentPerformanceCache struct {
	config             *PerformanceCacheConfig
	mutex              sync.RWMutex
	cache           map[string]*PerformanceCacheEntry // L1 cache
	compressedCache map[string]*PerformanceCacheEntry // L2 cache
	serverMetrics      map[string]*ServerMetrics
	methodMetrics      map[string]*CacheMethodMetrics
	cacheStats         *PerformanceCacheStats
	running            bool
	ctx                context.Context
	cancel             context.CancelFunc
	optimizationTicker *time.Ticker

	// Cache operation counters
	hitCount     int64
	missCount    int64
	evictions    int64
	compressions int64
}

// NewIntelligentPerformanceCache creates a new performance cache instance
func NewIntelligentPerformanceCache(config *PerformanceCacheConfig) *IntelligentPerformanceCache {
	if config == nil {
		config = &PerformanceCacheConfig{
			MaxMemoryMB:          256,
			DefaultTTL:           5 * time.Minute,
			MaxCacheEntries:      10000,
			CompressionEnabled:   true,
			OptimizationInterval: 30 * time.Second,
			HealthThreshold:      0.7,
			ResponseTimeBuffer:   1000,
			EnablePrediction:     true,
		}
	}

	return &IntelligentPerformanceCache{
		config:          config,
		cache:           make(map[string]*PerformanceCacheEntry),
		compressedCache: make(map[string]*PerformanceCacheEntry),
		serverMetrics:   make(map[string]*ServerMetrics),
		methodMetrics:   make(map[string]*CacheMethodMetrics),
		cacheStats:      &PerformanceCacheStats{},
	}
}

// NewPerformanceCache creates a new performance cache with default configuration
func NewPerformanceCache(logger *mcp.StructuredLogger) PerformanceCache {
	config := &PerformanceCacheConfig{
		MaxMemoryMB:          256,
		DefaultTTL:           5 * time.Minute,
		MaxCacheEntries:      10000,
		CompressionEnabled:   true,
		OptimizationInterval: 30 * time.Second,
		HealthThreshold:      0.7,
		ResponseTimeBuffer:   1000,
		EnablePrediction:     true,
	}
	return NewIntelligentPerformanceCache(config)
}

// Start begins the performance cache operations
func (pc *IntelligentPerformanceCache) Start(ctx context.Context) error {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	if pc.running {
		return fmt.Errorf("performance cache is already running")
	}

	pc.ctx, pc.cancel = context.WithCancel(ctx)
	pc.running = true

	// Start optimization ticker
	pc.optimizationTicker = time.NewTicker(pc.config.OptimizationInterval)

	go pc.optimizationLoop()
	go pc.metricsUpdateLoop()

	return nil
}

// Stop halts the performance cache operations
func (pc *IntelligentPerformanceCache) Stop() error {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	if !pc.running {
		return nil
	}

	pc.running = false
	if pc.cancel != nil {
		pc.cancel()
	}
	if pc.optimizationTicker != nil {
		pc.optimizationTicker.Stop()
	}

	return nil
}

// CacheResponse stores a response with intelligent caching strategy
func (pc *IntelligentPerformanceCache) CacheResponse(key string, response interface{}, ttl time.Duration) error {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	if ttl == 0 {
		ttl = pc.config.DefaultTTL
	}

	// Serialize response to calculate size
	data, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	size := int64(len(data))
	entry := &PerformanceCacheEntry{
		Response:    response,
		Timestamp:   time.Now(),
		TTL:         ttl,
		AccessCount: 0,
		LastAccess:  time.Now(),
		Size:        size,
		Priority:    pc.calculatePriority(key, size),
	}

	// Apply compression for large responses
	if pc.config.CompressionEnabled && size > 1024 {
		compressed, err := pc.compressData(data)
		if err == nil && len(compressed) < len(data) {
			entry.CompressedData = compressed
			entry.CompressionType = "gzip"
			pc.compressions++
		}
	}

	// Determine cache level based on priority and size
	cacheLevel := pc.determineCacheLevel(entry)

	switch cacheLevel {
	case 1:
		pc.cache[key] = entry
		pc.cacheStats.L1Entries++
	case 2:
		pc.compressedCache[key] = entry
		pc.cacheStats.L2Entries++
	default:
		pc.compressedCache[key] = entry
		pc.cacheStats.L2Entries++
	}

	pc.cacheStats.TotalEntries++
	pc.updateMemoryUsage()

	// Check if we need to evict entries
	if pc.shouldEvict() {
		pc.evictEntries()
	}

	return nil
}

// GetCachedResponse retrieves a cached response
func (pc *IntelligentPerformanceCache) GetCachedResponse(key string) (interface{}, bool) {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	// Check L1 cache first
	if entry, exists := pc.cache[key]; exists {
		if pc.isEntryValid(entry) {
			entry.AccessCount++
			entry.LastAccess = time.Now()
			pc.hitCount++
			return entry.Response, true
		} else {
			delete(pc.cache, key)
			pc.cacheStats.L1Entries--
		}
	}

	// Check L2 cache
	if entry, exists := pc.compressedCache[key]; exists {
		if pc.isEntryValid(entry) {
			entry.AccessCount++
			entry.LastAccess = time.Now()

			response := entry.Response
			if entry.CompressedData != nil {
				decompressed, err := pc.decompressData(entry.CompressedData)
				if err == nil {
					if err := json.Unmarshal(decompressed, &response); err != nil {
						// If unmarshal fails, continue with original response
						response = entry.Response
					}
				}
			}

			pc.hitCount++
			return response, true
		} else {
			delete(pc.compressedCache, key)
			pc.cacheStats.L2Entries--
		}
	}


	pc.missCount++
	return nil, false
}

// InvalidateCache removes cache entries matching a pattern
func (pc *IntelligentPerformanceCache) InvalidateCache(pattern string) error {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	regex, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern: %w", err)
	}

	// Invalidate from all cache levels
	for key := range pc.cache {
		if regex.MatchString(key) {
			delete(pc.cache, key)
			pc.cacheStats.L1Entries--
		}
	}

	for key := range pc.compressedCache {
		if regex.MatchString(key) {
			delete(pc.compressedCache, key)
			pc.cacheStats.L2Entries--
		}
	}


	pc.updateCacheStats()
	return nil
}

// RecordRequestMetrics records performance metrics for a request
func (pc *IntelligentPerformanceCache) RecordRequestMetrics(serverName, method string, responseTime time.Duration, success bool) {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	// Update server metrics
	serverMetrics := pc.getOrCreateServerMetrics(serverName)
	serverMetrics.TotalRequests++
	serverMetrics.LastRequestTime = time.Now()

	if success {
		serverMetrics.SuccessfulRequests++
	} else {
		serverMetrics.FailedRequests++
	}

	// Update response time statistics
	pc.updateResponseTimeMetrics(serverMetrics, responseTime)

	// Update method metrics
	methodMetrics := pc.getOrCreatePerformanceMethodMetrics(method)
	methodMetrics.TotalRequests++
	pc.updateMethodResponseTime(methodMetrics, responseTime)
	pc.updateMethodServerPerformance(methodMetrics, serverName, responseTime)

	// Update health score
	pc.calculateHealthScore(serverMetrics)
}

// RecordRoutingDecision records routing decision metrics
func (pc *IntelligentPerformanceCache) RecordRoutingDecision(method, serverName string, strategy interface{}) {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	// Update method metrics with routing strategy information
	methodMetrics := pc.getOrCreatePerformanceMethodMetrics(method)
	methodMetrics.TotalRequests++

	// Update server metrics with routing decision
	serverMetrics := pc.getOrCreateServerMetrics(serverName)
	serverMetrics.TotalRequests++
	serverMetrics.LastRequestTime = time.Now()

	// Record strategy usage (could be extended for more detailed routing analytics)
	if strategyStr, ok := strategy.(string); ok {
		// Store strategy information in server metrics metadata if needed
		_ = strategyStr // Placeholder for future strategy analytics
	}
}

// GetServerMetrics returns performance metrics for a server
func (pc *IntelligentPerformanceCache) GetServerMetrics(serverName string) *ServerMetrics {
	pc.mutex.RLock()
	defer pc.mutex.RUnlock()

	if metrics, exists := pc.serverMetrics[serverName]; exists {
		// Return a copy to prevent external modification
		metricsCopy := *metrics
		return &metricsCopy
	}
	return nil
}

// GetPerformanceMethodMetrics returns performance metrics for a method
func (pc *IntelligentPerformanceCache) GetPerformanceMethodMetrics(method string) *CacheMethodMetrics {
	pc.mutex.RLock()
	defer pc.mutex.RUnlock()

	if metrics, exists := pc.methodMetrics[method]; exists {
		// Return a copy to prevent external modification
		metricsCopy := *metrics
		return &metricsCopy
	}
	return nil
}

// UpdateServerHealth updates the health score for a server
func (pc *IntelligentPerformanceCache) UpdateServerHealth(serverName string, healthScore float64) {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	serverMetrics := pc.getOrCreateServerMetrics(serverName)
	serverMetrics.HealthScore = healthScore
}

// GetServerHealth returns the current health score for a server
func (pc *IntelligentPerformanceCache) GetServerHealth(serverName string) float64 {
	pc.mutex.RLock()
	defer pc.mutex.RUnlock()

	if metrics, exists := pc.serverMetrics[serverName]; exists {
		return metrics.HealthScore
	}
	return 0.0
}

// IsServerHealthy checks if a server is considered healthy
func (pc *IntelligentPerformanceCache) IsServerHealthy(serverName string) bool {
	return pc.GetServerHealth(serverName) >= pc.config.HealthThreshold
}

// OptimizeCache performs comprehensive cache optimization
func (pc *IntelligentPerformanceCache) OptimizeCache() error {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	startTime := time.Now()

	// Optimize cache levels
	pc.optimizeCacheLevels()

	// Update TTL based on access patterns
	pc.optimizeTTL()

	// Compress unused entries
	if pc.config.CompressionEnabled {
		pc.compressUnusedEntries()
	}

	// Update cache statistics
	pc.updateCacheStats()

	pc.cacheStats.OptimizationRuns++
	pc.cacheStats.LastOptimization = startTime

	return nil
}

// GetCacheStats returns comprehensive cache statistics
func (pc *IntelligentPerformanceCache) GetCacheStats() *PerformanceCacheStats {
	pc.mutex.RLock()
	defer pc.mutex.RUnlock()

	// Update hit rate
	totalRequests := pc.hitCount + pc.missCount
	if totalRequests > 0 {
		pc.cacheStats.CacheHitRate = float64(pc.hitCount) / float64(totalRequests)
	}

	// Calculate hot data percentage
	hotEntries := int64(0)
	threshold := time.Now().Add(-time.Hour)

	for _, entry := range pc.cache {
		if entry.LastAccess.After(threshold) {
			hotEntries++
		}
	}

	if pc.cacheStats.TotalEntries > 0 {
		pc.cacheStats.HotDataPercentage = float64(hotEntries) / float64(pc.cacheStats.TotalEntries)
	}

	// Return a copy of the current performance cache stats
	statsCopy := *pc.cacheStats
	return &statsCopy
}

// Helper methods

func (pc *IntelligentPerformanceCache) calculatePriority(key string, size int64) int {
	// Higher priority for smaller, frequently accessed items
	priority := 1

	if strings.Contains(key, "definition") || strings.Contains(key, "hover") {
		priority += 2 // High priority for navigation
	}

	if size < 1024 {
		priority += 1 // Prefer smaller responses
	}

	return priority
}

func (pc *IntelligentPerformanceCache) determineCacheLevel(entry *PerformanceCacheEntry) int {
	if entry.Priority >= 3 && entry.Size < 10*1024 {
		return 1 // L1 for high priority, small items
	}
	return 2 // L2 for larger items
}

func (pc *IntelligentPerformanceCache) isEntryValid(entry *PerformanceCacheEntry) bool {
	return time.Since(entry.Timestamp) < entry.TTL
}

func (pc *IntelligentPerformanceCache) shouldEvict() bool {
	maxEntries := pc.config.MaxCacheEntries
	currentEntries := len(pc.cache) + len(pc.compressedCache)

	return currentEntries > maxEntries || pc.cacheStats.MemoryUsage > pc.config.MaxMemoryMB*1024*1024
}

func (pc *IntelligentPerformanceCache) evictEntries() {
	// LRU eviction with priority consideration
	type entryInfo struct {
		key        string
		entry      *PerformanceCacheEntry
		cacheLevel int
		score      float64
	}

	var entries []entryInfo

	// Collect all entries with scores
	for key, entry := range pc.cache {
		score := pc.calculateEvictionScore(entry)
		entries = append(entries, entryInfo{key, entry, 1, score})
	}

	for key, entry := range pc.compressedCache {
		score := pc.calculateEvictionScore(entry)
		entries = append(entries, entryInfo{key, entry, 2, score})
	}


	// Sort by eviction score (lower is more likely to be evicted)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].score < entries[j].score
	})

	// Evict bottom 10%
	evictCount := len(entries) / 10
	if evictCount < 1 {
		evictCount = 1
	}

	for i := 0; i < evictCount && i < len(entries); i++ {
		entry := entries[i]
		switch entry.cacheLevel {
		case 1:
			delete(pc.cache, entry.key)
			pc.cacheStats.L1Entries--
		case 2:
			delete(pc.compressedCache, entry.key)
			pc.cacheStats.L2Entries--
		}
		pc.evictions++
	}

	pc.cacheStats.EvictionCount = pc.evictions
	pc.updateMemoryUsage()
}

func (pc *IntelligentPerformanceCache) calculateEvictionScore(entry *PerformanceCacheEntry) float64 {
	// Score based on access frequency, recency, and size
	timeSinceAccess := time.Since(entry.LastAccess).Seconds()
	accessFrequency := float64(entry.AccessCount)
	sizeScore := 1.0 / (1.0 + float64(entry.Size)/1024.0) // Prefer smaller items
	priorityScore := float64(entry.Priority)

	// Lower score means more likely to be evicted
	return (accessFrequency * priorityScore * sizeScore) / (1.0 + timeSinceAccess)
}

func (pc *IntelligentPerformanceCache) getOrCreateServerMetrics(serverName string) *ServerMetrics {
	if metrics, exists := pc.serverMetrics[serverName]; exists {
		return metrics
	}

	metrics := &ServerMetrics{
		ServerName:          serverName,
		MinResponseTime:     time.Duration(math.MaxInt64),
		ResponseTimes:       make([]time.Duration, pc.config.ResponseTimeBuffer),
		ResponseTimeIndex:   0,
		HealthScore:         1.0,
		CircuitBreakerState: "CLOSED",
		ActiveConnections:   0,
		SuccessRate:         0.0,
	}
	pc.serverMetrics[serverName] = metrics
	return metrics
}

func (pc *IntelligentPerformanceCache) getOrCreatePerformanceMethodMetrics(method string) *CacheMethodMetrics {
	if metrics, exists := pc.methodMetrics[method]; exists {
		return metrics
	}

	metrics := &CacheMethodMetrics{
		Method:            method,
		ServerPerformance: make(map[string]float64),
		OptimalTTL:        pc.config.DefaultTTL,
	}
	pc.methodMetrics[method] = metrics
	return metrics
}

func (pc *IntelligentPerformanceCache) updateResponseTimeMetrics(metrics *ServerMetrics, responseTime time.Duration) {
	// Update min/max
	if responseTime < metrics.MinResponseTime {
		metrics.MinResponseTime = responseTime
	}
	if responseTime > metrics.MaxResponseTime {
		metrics.MaxResponseTime = responseTime
	}

	// Update circular buffer for percentile calculation
	metrics.ResponseTimes[metrics.ResponseTimeIndex] = responseTime
	metrics.ResponseTimeIndex = (metrics.ResponseTimeIndex + 1) % len(metrics.ResponseTimes)

	// Calculate exponential moving average
	alpha := 0.1
	if metrics.AverageResponseTime == 0 {
		metrics.AverageResponseTime = responseTime
	} else {
		newAvg := time.Duration(float64(metrics.AverageResponseTime)*(1-alpha) + float64(responseTime)*alpha)
		metrics.AverageResponseTime = newAvg
	}

	// Calculate percentiles
	pc.calculatePercentiles(metrics)
}

func (pc *IntelligentPerformanceCache) calculatePercentiles(metrics *ServerMetrics) {
	times := make([]time.Duration, 0, len(metrics.ResponseTimes))
	for _, t := range metrics.ResponseTimes {
		if t > 0 {
			times = append(times, t)
		}
	}

	if len(times) == 0 {
		return
	}

	sort.Slice(times, func(i, j int) bool {
		return times[i] < times[j]
	})

	p50Index := len(times) * 50 / 100
	p95Index := len(times) * 95 / 100
	p99Index := len(times) * 99 / 100

	if p50Index < len(times) {
		metrics.P50ResponseTime = times[p50Index]
	}
	if p95Index < len(times) {
		metrics.P95ResponseTime = times[p95Index]
	}
	if p99Index < len(times) {
		metrics.P99ResponseTime = times[p99Index]
	}
}

func (pc *IntelligentPerformanceCache) calculateHealthScore(metrics *ServerMetrics) {
	if metrics.TotalRequests == 0 {
		metrics.HealthScore = 1.0
		return
	}

	// Success rate component (0-0.4)
	successRate := float64(metrics.SuccessfulRequests) / float64(metrics.TotalRequests)
	successComponent := successRate * 0.4

	// Response time component (0-0.4)
	targetResponseTime := 100 * time.Millisecond
	responseTimeRatio := float64(targetResponseTime) / float64(metrics.AverageResponseTime)
	if responseTimeRatio > 1.0 {
		responseTimeRatio = 1.0
	}
	responseTimeComponent := responseTimeRatio * 0.4

	// Load component (0-0.2)
	loadComponent := (1.0 - metrics.LoadScore) * 0.2

	metrics.HealthScore = successComponent + responseTimeComponent + loadComponent
}

func (pc *IntelligentPerformanceCache) updateMethodResponseTime(metrics *CacheMethodMetrics, responseTime time.Duration) {
	alpha := 0.1
	if metrics.AverageResponseTime == 0 {
		metrics.AverageResponseTime = responseTime
	} else {
		newAvg := time.Duration(float64(metrics.AverageResponseTime)*(1-alpha) + float64(responseTime)*alpha)
		metrics.AverageResponseTime = newAvg
	}
}

func (pc *IntelligentPerformanceCache) updateMethodServerPerformance(metrics *CacheMethodMetrics, serverName string, responseTime time.Duration) {
	currentPerf, exists := metrics.ServerPerformance[serverName]
	if !exists {
		metrics.ServerPerformance[serverName] = float64(responseTime.Nanoseconds())
	} else {
		alpha := 0.1
		newPerf := currentPerf*(1-alpha) + float64(responseTime.Nanoseconds())*alpha
		metrics.ServerPerformance[serverName] = newPerf
	}
}

func (pc *IntelligentPerformanceCache) compressData(data []byte) ([]byte, error) {
	var buf strings.Builder
	gz := gzip.NewWriter(&buf)

	_, err := gz.Write(data)
	if err != nil {
		return nil, err
	}

	err = gz.Close()
	if err != nil {
		return nil, err
	}

	return []byte(buf.String()), nil
}

func (pc *IntelligentPerformanceCache) decompressData(data []byte) ([]byte, error) {
	reader := strings.NewReader(string(data))
	gz, err := gzip.NewReader(reader)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()

	var result strings.Builder
	_, err = io.Copy(&result, gz)
	if err != nil {
		return nil, err
	}

	return []byte(result.String()), nil
}

func (pc *IntelligentPerformanceCache) optimizationLoop() {
	for {
		select {
		case <-pc.ctx.Done():
			return
		case <-pc.optimizationTicker.C:
			if err := pc.OptimizeCache(); err != nil {
				// Optimization error - continue background processing
				_ = err
			}
		}
	}
}

func (pc *IntelligentPerformanceCache) metricsUpdateLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-pc.ctx.Done():
			return
		case <-ticker.C:
			pc.updateAllMetrics()
		}
	}
}

func (pc *IntelligentPerformanceCache) updateAllMetrics() {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	// Update cache statistics
	pc.updateCacheStats()

	// Update method cache hit rates
	for _, methodMetrics := range pc.methodMetrics {
		pc.updateMethodCacheHitRate(methodMetrics)
	}
}

func (pc *IntelligentPerformanceCache) updateCacheStats() {
	pc.cacheStats.TotalEntries = int64(len(pc.cache) + len(pc.compressedCache))
	pc.updateMemoryUsage()

	totalRequests := pc.hitCount + pc.missCount
	if totalRequests > 0 {
		pc.cacheStats.CacheHitRate = float64(pc.hitCount) / float64(totalRequests)
	}

	// Calculate compression ratio
	if pc.compressions > 0 {
		pc.cacheStats.CompressionRatio = 0.7 // Approximate compression ratio
	}
}

func (pc *IntelligentPerformanceCache) updateMemoryUsage() {
	var totalSize int64

	for _, entry := range pc.cache {
		if entry.CompressedData != nil {
			totalSize += int64(len(entry.CompressedData))
		} else {
			totalSize += entry.Size
		}
	}

	for _, entry := range pc.compressedCache {
		if entry.CompressedData != nil {
			totalSize += int64(len(entry.CompressedData))
		} else {
			totalSize += entry.Size
		}
	}


	pc.cacheStats.MemoryUsage = totalSize
}

func (pc *IntelligentPerformanceCache) updateMethodCacheHitRate(metrics *CacheMethodMetrics) {
	// This would be calculated based on actual cache hits for this method
	// For now, use overall cache hit rate as approximation
	metrics.CacheHitRate = pc.cacheStats.CacheHitRate
}

func (pc *IntelligentPerformanceCache) optimizeCacheLevels() {
	// Move frequently accessed items to higher cache levels
	for key, entry := range pc.compressedCache {
		if entry.AccessCount > 10 && entry.Size < 10*1024 {
			// Move to L1
			pc.cache[key] = entry
			delete(pc.compressedCache, key)
			pc.cacheStats.L1Entries++
			pc.cacheStats.L2Entries--
		}
	}

}

func (pc *IntelligentPerformanceCache) optimizeTTL() {
	// Analyze access patterns and adjust TTL for methods
	for _, methodMetrics := range pc.methodMetrics {
		if methodMetrics.TotalRequests > 100 {
			// Increase TTL for frequently accessed methods
			if methodMetrics.AverageResponseTime < 50*time.Millisecond {
				methodMetrics.OptimalTTL = pc.config.DefaultTTL * 2
			} else {
				methodMetrics.OptimalTTL = pc.config.DefaultTTL / 2
			}
		}
	}
}

func (pc *IntelligentPerformanceCache) compressUnusedEntries() {
	threshold := time.Now().Add(-time.Hour)

	for _, entry := range pc.cache {
		if entry.LastAccess.Before(threshold) && entry.CompressedData == nil && entry.Size > 1024 {
			data, err := json.Marshal(entry.Response)
			if err == nil {
				compressed, err := pc.compressData(data)
				if err == nil && len(compressed) < len(data) {
					entry.CompressedData = compressed
					entry.CompressionType = "gzip"
					entry.Response = nil // Clear uncompressed data
					pc.compressions++
				}
			}
		}
	}
}
