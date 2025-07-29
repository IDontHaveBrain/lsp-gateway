package workspace

import (
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// MetricsCollector provides comprehensive performance metrics collection for workspaces
type MetricsCollector struct {
	startTime        time.Time
	workspaceMetrics map[string]*WorkspaceMetrics
	systemMetrics    *SystemMetrics
	mu               sync.RWMutex
	isCollecting     atomic.Bool
	stopCh           chan struct{}
	wg               sync.WaitGroup
	
	// Atomic counters for high-frequency operations
	totalStartups    atomic.Int64
	totalRequests    atomic.Int64
	totalLatency     atomic.Int64 // nanoseconds
	totalCacheHits   atomic.Int64
	totalCacheMisses atomic.Int64
	totalMemoryUsage atomic.Int64
	totalErrors      atomic.Int64
	
	// Configuration
	collectionInterval time.Duration
	historySize        int
	
	// Time series data
	metricsHistory []MetricsSnapshot
	historyMu      sync.RWMutex
}

// SystemMetrics contains system-wide performance metrics
type SystemMetrics struct {
	CPUUsage       float64   `json:"cpu_usage_percent"`
	MemoryUsage    int64     `json:"memory_usage_bytes"`
	GoroutineCount int       `json:"goroutine_count"`
	OpenFiles      int       `json:"open_files"`
	NetworkConns   int       `json:"network_connections"`
	DiskIOBytes    int64     `json:"disk_io_bytes"`
	NetworkIOBytes int64     `json:"network_io_bytes"`
	LoadAverage    float64   `json:"load_average"`
	Timestamp      time.Time `json:"timestamp"`
}

// MetricsSnapshot represents a point-in-time metrics snapshot
type MetricsSnapshot struct {
	Timestamp        time.Time            `json:"timestamp"`
	SystemMetrics    *SystemMetrics       `json:"system_metrics"`
	WorkspaceCount   int                  `json:"workspace_count"`
	TotalRequests    int64                `json:"total_requests"`
	RequestsPerSec   float64              `json:"requests_per_second"`
	AverageLatency   time.Duration        `json:"average_latency"`
	CacheHitRatio    float64              `json:"cache_hit_ratio"`
	ErrorRate        float64              `json:"error_rate"`
	MemoryPerWS      int64                `json:"memory_per_workspace"`
}

// PerformanceReport contains comprehensive performance analysis
type PerformanceReport struct {
	GeneratedAt      time.Time                    `json:"generated_at"`
	CollectionPeriod time.Duration                `json:"collection_period"`
	Summary          *PerformanceSummary          `json:"summary"`
	WorkspaceMetrics map[string]*WorkspaceMetrics `json:"workspace_metrics"`
	SystemMetrics    *SystemMetrics               `json:"system_metrics"`
	TimeSeriesData   []MetricsSnapshot            `json:"time_series_data"`
	Recommendations  []PerformanceRecommendation  `json:"recommendations"`
	HealthScore      float64                      `json:"health_score"`
	Grade            string                       `json:"grade"`
}

// PerformanceSummary provides high-level performance summary
type PerformanceSummary struct {
	TotalWorkspaces     int           `json:"total_workspaces"`
	TotalRequests       int64         `json:"total_requests"`
	AverageLatency      time.Duration `json:"average_latency"`
	P95Latency          time.Duration `json:"p95_latency"`
	P99Latency          time.Duration `json:"p99_latency"`
	RequestsPerSecond   float64       `json:"requests_per_second"`
	CacheHitRatio       float64       `json:"cache_hit_ratio"`
	ErrorRate           float64       `json:"error_rate"`
	AverageMemoryPerWS  int64         `json:"average_memory_per_workspace"`
	PeakMemoryUsage     int64         `json:"peak_memory_usage"`
	UptimeSeconds       float64       `json:"uptime_seconds"`
}

// PerformanceRecommendation provides actionable performance recommendations
type PerformanceRecommendation struct {
	Type        string    `json:"type"`
	Priority    string    `json:"priority"`
	Component   string    `json:"component"`
	Issue       string    `json:"issue"`
	Suggestion  string    `json:"suggestion"`
	Impact      string    `json:"impact"`
	Timestamp   time.Time `json:"timestamp"`
}

// NewMetricsCollector creates a new metrics collector with default configuration
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		startTime:          time.Now(),
		workspaceMetrics:   make(map[string]*WorkspaceMetrics),
		systemMetrics:      &SystemMetrics{},
		stopCh:             make(chan struct{}),
		collectionInterval: 5 * time.Second,
		historySize:        720, // 1 hour of data at 5-second intervals
		metricsHistory:     make([]MetricsSnapshot, 0, 720),
	}
}

// NewMetricsCollectorWithConfig creates a metrics collector with custom configuration
func NewMetricsCollectorWithConfig(interval time.Duration, historySize int) *MetricsCollector {
	mc := NewMetricsCollector()
	mc.collectionInterval = interval
	mc.historySize = historySize
	mc.metricsHistory = make([]MetricsSnapshot, 0, historySize)
	return mc
}

// StartCollection begins metrics collection in background
func (mc *MetricsCollector) StartCollection() {
	if !mc.isCollecting.CompareAndSwap(false, true) {
		return // Already collecting
	}

	mc.startTime = time.Now()
	
	mc.wg.Add(1)
	go mc.collectionWorker()
}

// StopCollection stops metrics collection
func (mc *MetricsCollector) StopCollection() {
	if !mc.isCollecting.CompareAndSwap(true, false) {
		return // Not collecting
	}

	close(mc.stopCh)
	mc.wg.Wait()
}

// RecordWorkspaceStartup records a workspace startup event
func (mc *MetricsCollector) RecordWorkspaceStartup(workspaceID string, duration time.Duration) {
	mc.totalStartups.Add(1)
	
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	if metrics, exists := mc.workspaceMetrics[workspaceID]; exists {
		metrics.StartupTime = duration
		metrics.LastUpdated = time.Now()
	} else {
		mc.workspaceMetrics[workspaceID] = &WorkspaceMetrics{
			WorkspaceID: workspaceID,
			StartupTime: duration,
			LastUpdated: time.Now(),
			HealthScore: 1.0,
		}
	}
}

// RecordRequestLatency records a request latency measurement
func (mc *MetricsCollector) RecordRequestLatency(workspaceID string, latency time.Duration) {
	mc.totalRequests.Add(1)
	mc.totalLatency.Add(latency.Nanoseconds())
	
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	if metrics, exists := mc.workspaceMetrics[workspaceID]; exists {
		metrics.RequestLatencies = append(metrics.RequestLatencies, latency)
		
		// Keep only recent latencies (last 1000 measurements)
		if len(metrics.RequestLatencies) > 1000 {
			copy(metrics.RequestLatencies, metrics.RequestLatencies[len(metrics.RequestLatencies)-1000:])
			metrics.RequestLatencies = metrics.RequestLatencies[:1000]
		}
		
		metrics.LastUpdated = time.Now()
		mc.updateWorkspaceThroughput(metrics)
	} else {
		mc.workspaceMetrics[workspaceID] = &WorkspaceMetrics{
			WorkspaceID:      workspaceID,
			RequestLatencies: []time.Duration{latency},
			LastUpdated:      time.Now(),
			HealthScore:      1.0,
		}
	}
}

// RecordMemoryUsage records memory usage for a workspace
func (mc *MetricsCollector) RecordMemoryUsage(workspaceID string, usage int64) {
	mc.totalMemoryUsage.Store(usage)
	
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	if metrics, exists := mc.workspaceMetrics[workspaceID]; exists {
		metrics.MemoryUsage = usage
		metrics.LastUpdated = time.Now()
	} else {
		mc.workspaceMetrics[workspaceID] = &WorkspaceMetrics{
			WorkspaceID: workspaceID,
			MemoryUsage: usage,
			LastUpdated: time.Now(),
			HealthScore: 1.0,
		}
	}
}

// RecordCacheStats records cache hit/miss statistics
func (mc *MetricsCollector) RecordCacheStats(workspaceID string, hits, misses int) {
	mc.totalCacheHits.Add(int64(hits))
	mc.totalCacheMisses.Add(int64(misses))
	
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	if metrics, exists := mc.workspaceMetrics[workspaceID]; exists {
		totalHits := float64(hits)
		totalRequests := float64(hits + misses)
		if totalRequests > 0 {
			metrics.CacheHitRatio = totalHits / totalRequests
		}
		metrics.LastUpdated = time.Now()
	} else {
		hitRatio := 0.0
		if hits+misses > 0 {
			hitRatio = float64(hits) / float64(hits+misses)
		}
		mc.workspaceMetrics[workspaceID] = &WorkspaceMetrics{
			WorkspaceID:   workspaceID,
			CacheHitRatio: hitRatio,
			LastUpdated:   time.Now(),
			HealthScore:   1.0,
		}
	}
}

// RecordError records an error occurrence
func (mc *MetricsCollector) RecordError(workspaceID string, errorType string) {
	mc.totalErrors.Add(1)
	
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	if metrics, exists := mc.workspaceMetrics[workspaceID]; exists {
		// Update error rate based on recent requests
		if len(metrics.RequestLatencies) > 0 {
			metrics.ErrorRate = float64(mc.totalErrors.Load()) / float64(mc.totalRequests.Load())
		}
		metrics.LastUpdated = time.Now()
		mc.updateWorkspaceHealthScore(metrics)
	}
}

// GetWorkspaceMetrics returns metrics for a specific workspace
func (mc *MetricsCollector) GetWorkspaceMetrics(workspaceID string) *WorkspaceMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	if metrics, exists := mc.workspaceMetrics[workspaceID]; exists {
		// Return a copy to avoid race conditions
		metricsCopy := *metrics
		metricsCopy.RequestLatencies = make([]time.Duration, len(metrics.RequestLatencies))
		copy(metricsCopy.RequestLatencies, metrics.RequestLatencies)
		return &metricsCopy
	}
	
	return nil
}

// GetSystemMetrics returns current system metrics
func (mc *MetricsCollector) GetSystemMetrics() *SystemMetrics {
	mc.collectSystemMetrics()
	
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	// Return a copy
	systemMetricsCopy := *mc.systemMetrics
	return &systemMetricsCopy
}

// GetAllWorkspaceMetrics returns metrics for all workspaces
func (mc *MetricsCollector) GetAllWorkspaceMetrics() map[string]*WorkspaceMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	result := make(map[string]*WorkspaceMetrics)
	for id, metrics := range mc.workspaceMetrics {
		metricsCopy := *metrics
		metricsCopy.RequestLatencies = make([]time.Duration, len(metrics.RequestLatencies))
		copy(metricsCopy.RequestLatencies, metrics.RequestLatencies)
		result[id] = &metricsCopy
	}
	
	return result
}

// GeneratePerformanceReport creates a comprehensive performance report
func (mc *MetricsCollector) GeneratePerformanceReport() *PerformanceReport {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	now := time.Now()
	uptime := now.Sub(mc.startTime)
	
	// Calculate summary metrics
	summary := mc.calculatePerformanceSummary(uptime)
	
	// Get current system metrics
	systemMetrics := mc.GetSystemMetrics()
	
	// Copy time series data
	mc.historyMu.RLock()
	timeSeriesData := make([]MetricsSnapshot, len(mc.metricsHistory))
	copy(timeSeriesData, mc.metricsHistory)
	mc.historyMu.RUnlock()
	
	// Generate recommendations
	recommendations := mc.generateRecommendations(summary, systemMetrics, timeSeriesData)
	
	// Calculate health score and grade
	healthScore := mc.calculateHealthScore(summary, systemMetrics)
	grade := mc.calculateGrade(healthScore, summary)
	
	return &PerformanceReport{
		GeneratedAt:      now,
		CollectionPeriod: uptime,
		Summary:          summary,
		WorkspaceMetrics: mc.GetAllWorkspaceMetrics(),
		SystemMetrics:    systemMetrics,
		TimeSeriesData:   timeSeriesData,
		Recommendations:  recommendations,
		HealthScore:      healthScore,
		Grade:            grade,
	}
}

// ExportMetricsJSON exports metrics as JSON for external systems
func (mc *MetricsCollector) ExportMetricsJSON() ([]byte, error) {
	report := mc.GeneratePerformanceReport()
	return json.MarshalIndent(report, "", "  ")
}

// GetHistoricalData returns historical metrics data
func (mc *MetricsCollector) GetHistoricalData(since time.Time) []MetricsSnapshot {
	mc.historyMu.RLock()
	defer mc.historyMu.RUnlock()
	
	var result []MetricsSnapshot
	for _, snapshot := range mc.metricsHistory {
		if snapshot.Timestamp.After(since) {
			result = append(result, snapshot)
		}
	}
	
	return result
}

// Private methods

func (mc *MetricsCollector) collectionWorker() {
	defer mc.wg.Done()
	
	ticker := time.NewTicker(mc.collectionInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-mc.stopCh:
			return
		case <-ticker.C:
			mc.collectAndStoreSnapshot()
		}
	}
}

func (mc *MetricsCollector) collectAndStoreSnapshot() {
	// Collect current system metrics
	mc.collectSystemMetrics()
	
	// Create snapshot
	snapshot := mc.createMetricsSnapshot()
	
	// Store in history
	mc.historyMu.Lock()
	mc.metricsHistory = append(mc.metricsHistory, snapshot)
	
	// Trim history if too large
	if len(mc.metricsHistory) > mc.historySize {
		copy(mc.metricsHistory, mc.metricsHistory[1:])
		mc.metricsHistory = mc.metricsHistory[:mc.historySize]
	}
	mc.historyMu.Unlock()
}

func (mc *MetricsCollector) collectSystemMetrics() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.systemMetrics.MemoryUsage = int64(memStats.Alloc)
	mc.systemMetrics.GoroutineCount = runtime.NumGoroutine()
	mc.systemMetrics.Timestamp = time.Now()
	
	// CPU usage would require platform-specific code or external libraries
	// For now, we'll use a placeholder
	mc.systemMetrics.CPUUsage = 0.0
	
	// Network and file metrics would also require platform-specific code
	mc.systemMetrics.OpenFiles = 0
	mc.systemMetrics.NetworkConns = 0
	mc.systemMetrics.DiskIOBytes = 0
	mc.systemMetrics.NetworkIOBytes = 0
	mc.systemMetrics.LoadAverage = 0.0
}

func (mc *MetricsCollector) createMetricsSnapshot() MetricsSnapshot {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	totalRequests := mc.totalRequests.Load()
	totalCacheHits := mc.totalCacheHits.Load()
	totalCacheMisses := mc.totalCacheMisses.Load()
	totalLatency := mc.totalLatency.Load()
	totalErrors := mc.totalErrors.Load()
	
	// Calculate metrics
	requestsPerSec := 0.0
	if mc.collectionInterval > 0 {
		requestsPerSec = float64(totalRequests) / time.Since(mc.startTime).Seconds()
	}
	
	avgLatency := time.Duration(0)
	if totalRequests > 0 {
		avgLatency = time.Duration(totalLatency / totalRequests)
	}
	
	cacheHitRatio := 0.0
	totalCacheRequests := totalCacheHits + totalCacheMisses
	if totalCacheRequests > 0 {
		cacheHitRatio = float64(totalCacheHits) / float64(totalCacheRequests)
	}
	
	errorRate := 0.0
	if totalRequests > 0 {
		errorRate = float64(totalErrors) / float64(totalRequests)
	}
	
	memoryPerWS := int64(0)
	if len(mc.workspaceMetrics) > 0 {
		memoryPerWS = mc.totalMemoryUsage.Load() / int64(len(mc.workspaceMetrics))
	}
	
	return MetricsSnapshot{
		Timestamp:        time.Now(),
		SystemMetrics:    mc.systemMetrics,
		WorkspaceCount:   len(mc.workspaceMetrics),
		TotalRequests:    totalRequests,
		RequestsPerSec:   requestsPerSec,
		AverageLatency:   avgLatency,
		CacheHitRatio:    cacheHitRatio,
		ErrorRate:        errorRate,
		MemoryPerWS:      memoryPerWS,
	}
}

func (mc *MetricsCollector) calculatePerformanceSummary(uptime time.Duration) *PerformanceSummary {
	totalRequests := mc.totalRequests.Load()
	totalLatency := mc.totalLatency.Load()
	totalCacheHits := mc.totalCacheHits.Load()
	totalCacheMisses := mc.totalCacheMisses.Load()
	totalErrors := mc.totalErrors.Load()
	
	avgLatency := time.Duration(0)
	if totalRequests > 0 {
		avgLatency = time.Duration(totalLatency / totalRequests)
	}
	
	requestsPerSec := 0.0
	if uptime > 0 {
		requestsPerSec = float64(totalRequests) / uptime.Seconds()
	}
	
	cacheHitRatio := 0.0
	totalCacheRequests := totalCacheHits + totalCacheMisses
	if totalCacheRequests > 0 {
		cacheHitRatio = float64(totalCacheHits) / float64(totalCacheRequests)
	}
	
	errorRate := 0.0
	if totalRequests > 0 {
		errorRate = float64(totalErrors) / float64(totalRequests)
	}
	
	// Calculate percentiles from workspace metrics
	var allLatencies []time.Duration
	var totalMemory int64
	
	for _, metrics := range mc.workspaceMetrics {
		allLatencies = append(allLatencies, metrics.RequestLatencies...)
		totalMemory += metrics.MemoryUsage
	}
	
	p95Latency := time.Duration(0)
	p99Latency := time.Duration(0)
	if len(allLatencies) > 0 {
		sortDurations(allLatencies)
		p95Latency = calculatePercentile(allLatencies, 0.95)
		p99Latency = calculatePercentile(allLatencies, 0.99)
	}
	
	avgMemoryPerWS := int64(0)
	if len(mc.workspaceMetrics) > 0 {
		avgMemoryPerWS = totalMemory / int64(len(mc.workspaceMetrics))
	}
	
	return &PerformanceSummary{
		TotalWorkspaces:     len(mc.workspaceMetrics),
		TotalRequests:       totalRequests,
		AverageLatency:      avgLatency,
		P95Latency:          p95Latency,
		P99Latency:          p99Latency,
		RequestsPerSecond:   requestsPerSec,
		CacheHitRatio:       cacheHitRatio,
		ErrorRate:           errorRate,
		AverageMemoryPerWS:  avgMemoryPerWS,
		PeakMemoryUsage:     mc.systemMetrics.MemoryUsage,
		UptimeSeconds:       uptime.Seconds(),
	}
}

// sortDurations sorts a slice of time.Duration values in ascending order
func sortDurations(durations []time.Duration) {
	for i := 0; i < len(durations); i++ {
		for j := i + 1; j < len(durations); j++ {
			if durations[i] > durations[j] {
				durations[i], durations[j] = durations[j], durations[i]
			}
		}
	}
}

// calculatePercentile calculates the specified percentile from a sorted slice of durations
func calculatePercentile(sortedDurations []time.Duration, percentile float64) time.Duration {
	if len(sortedDurations) == 0 {
		return 0
	}
	
	index := int(float64(len(sortedDurations)-1) * percentile)
	if index < 0 {
		index = 0
	}
	if index >= len(sortedDurations) {
		index = len(sortedDurations) - 1
	}
	
	return sortedDurations[index]
}

func (mc *MetricsCollector) generateRecommendations(summary *PerformanceSummary, 
	systemMetrics *SystemMetrics, timeSeriesData []MetricsSnapshot) []PerformanceRecommendation {
	
	var recommendations []PerformanceRecommendation
	now := time.Now()
	
	// High latency recommendation
	if summary.P95Latency > 50*time.Millisecond {
		recommendations = append(recommendations, PerformanceRecommendation{
			Type:       "performance",
			Priority:   "high",
			Component:  "request_processing",
			Issue:      fmt.Sprintf("High P95 latency: %v", summary.P95Latency),
			Suggestion: "Consider optimizing request processing, adding caching, or scaling resources",
			Impact:     "User experience degradation",
			Timestamp:  now,
		})
	}
	
	// Low cache hit ratio recommendation
	if summary.CacheHitRatio < 0.8 {
		recommendations = append(recommendations, PerformanceRecommendation{
			Type:       "caching",
			Priority:   "medium",
			Component:  "cache_system",
			Issue:      fmt.Sprintf("Low cache hit ratio: %.2f%%", summary.CacheHitRatio*100),
			Suggestion: "Review cache configuration, increase cache size, or optimize cache keys",
			Impact:     "Increased latency and resource usage",
			Timestamp:  now,
		})
	}
	
	// High error rate recommendation
	if summary.ErrorRate > 0.05 {
		recommendations = append(recommendations, PerformanceRecommendation{
			Type:       "reliability",
			Priority:   "critical",
			Component:  "request_handling",
			Issue:      fmt.Sprintf("High error rate: %.2f%%", summary.ErrorRate*100),
			Suggestion: "Investigate error causes, improve error handling, or check resource availability",
			Impact:     "Service reliability issues",
			Timestamp:  now,
		})
	}
	
	// High memory usage recommendation
	maxMemoryPerWorkspace := int64(100 * 1024 * 1024) // 100MB
	if summary.AverageMemoryPerWS > maxMemoryPerWorkspace {
		recommendations = append(recommendations, PerformanceRecommendation{
			Type:       "resource",
			Priority:   "medium",
			Component:  "memory_management",
			Issue:      fmt.Sprintf("High memory usage per workspace: %d MB", summary.AverageMemoryPerWS/(1024*1024)),
			Suggestion: "Optimize memory usage, reduce cache sizes, or investigate memory leaks",
			Impact:     "Resource exhaustion risk",
			Timestamp:  now,
		})
	}
	
	// Low throughput recommendation
	if summary.RequestsPerSecond < 100 && summary.TotalRequests > 1000 {
		recommendations = append(recommendations, PerformanceRecommendation{
			Type:       "throughput",
			Priority:   "medium",
			Component:  "request_processing",
			Issue:      fmt.Sprintf("Low throughput: %.2f RPS", summary.RequestsPerSecond),
			Suggestion: "Consider connection pooling, request batching, or horizontal scaling",
			Impact:     "Reduced system capacity",
			Timestamp:  now,
		})
	}
	
	return recommendations
}

func (mc *MetricsCollector) calculateHealthScore(summary *PerformanceSummary, systemMetrics *SystemMetrics) float64 {
	score := 100.0
	
	// Deduct for high latency
	if summary.P95Latency > 50*time.Millisecond {
		score -= 20
	}
	
	// Deduct for low cache hit ratio
	if summary.CacheHitRatio < 0.8 {
		score -= 15
	}
	
	// Deduct for high error rate
	if summary.ErrorRate > 0.05 {
		score -= 30
	}
	
	// Deduct for high memory usage
	if summary.AverageMemoryPerWS > 100*1024*1024 {
		score -= 10
	}
	
	// Deduct for low throughput
	if summary.RequestsPerSecond < 100 && summary.TotalRequests > 1000 {
		score -= 10
	}
	
	// Normalize to 0-1 range
	if score < 0 {
		score = 0
	}
	
	return score / 100.0
}

func (mc *MetricsCollector) calculateGrade(healthScore float64, summary *PerformanceSummary) string {
	switch {
	case healthScore >= 0.9:
		return "A"
	case healthScore >= 0.8:
		return "B"
	case healthScore >= 0.7:
		return "C"
	case healthScore >= 0.6:
		return "D"
	default:
		return "F"
	}
}

func (mc *MetricsCollector) updateWorkspaceThroughput(metrics *WorkspaceMetrics) {
	if len(metrics.RequestLatencies) > 1 {
		// Calculate throughput based on recent requests
		recentDuration := time.Since(metrics.LastUpdated.Add(-time.Minute))
		if recentDuration > 0 {
			recentRequests := 0
			for _, latency := range metrics.RequestLatencies {
				if latency > 0 { // Simple proxy for recent requests
					recentRequests++
				}
			}
			metrics.ThroughputRPS = float64(recentRequests) / recentDuration.Seconds()
		}
	}
}

func (mc *MetricsCollector) updateWorkspaceHealthScore(metrics *WorkspaceMetrics) {
	score := 1.0
	
	// Penalize high error rates
	if metrics.ErrorRate > 0.05 {
		score -= 0.3
	}
	
	// Penalize low cache hit ratios
	if metrics.CacheHitRatio < 0.8 {
		score -= 0.2
	}
	
	// Penalize high memory usage
	if metrics.MemoryUsage > 100*1024*1024 { // 100MB
		score -= 0.2
	}
	
	// Penalize high latencies
	if len(metrics.RequestLatencies) > 0 {
		var avgLatency time.Duration
		var totalLatency int64
		for _, latency := range metrics.RequestLatencies {
			totalLatency += latency.Nanoseconds()
		}
		avgLatency = time.Duration(totalLatency / int64(len(metrics.RequestLatencies)))
		
		if avgLatency > 50*time.Millisecond {
			score -= 0.3
		}
	}
	
	if score < 0 {
		score = 0
	}
	
	metrics.HealthScore = score
}