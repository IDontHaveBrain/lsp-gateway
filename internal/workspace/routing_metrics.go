package workspace

import (
	"sync"
	"sync/atomic"
	"time"

	"lsp-gateway/mcp"
)

// RoutingMetricsCollector provides comprehensive metrics collection for request routing
type RoutingMetricsCollector struct {
	// Core metrics counters
	totalRequests         int64
	successfulRequests    int64
	failedRequests        int64
	routingErrors         int64
	fallbackUsages        int64
	
	// Performance metrics
	totalLatency          int64
	routingLatency        int64
	executionLatency      int64
	
	// Strategy metrics
	strategyUsage         map[string]int64
	strategySuccess       map[string]int64
	strategyFailure       map[string]int64
	
	// Method metrics
	methodCounts          map[string]int64
	methodLatency         map[string]int64
	methodErrors          map[string]int64
	
	// Project metrics
	projectCounts         map[string]int64
	projectLatency        map[string]int64
	projectErrors         map[string]int64
	
	// Client metrics
	clientUsage           map[string]int64
	clientErrors          map[string]int64
	clientLatency         map[string]int64
	
	// Error categorization
	errorCategories       map[string]int64
	errorsByType          map[RoutingErrorType]int64
	
	// Performance targets tracking
	performanceMetrics    *PerformanceMetrics
	
	// Time-based metrics
	hourlyMetrics         map[int]*HourlyMetrics
	dailyMetrics          map[string]*DailyMetrics
	
	// Configuration
	retentionDays         int
	maxEntriesPerCategory int
	
	// Synchronization
	mu                    sync.RWMutex
	metricsLock           sync.RWMutex
	logger                *mcp.StructuredLogger
	
	// Lifecycle
	startTime             time.Time
	lastUpdated           time.Time
}

// PerformanceMetrics tracks performance against targets
type PerformanceMetrics struct {
	// Success rate tracking
	TargetSuccessRate     float64 `json:"target_success_rate"`
	ActualSuccessRate     float64 `json:"actual_success_rate"`
	SuccessRateViolations int64   `json:"success_rate_violations"`
	
	// Latency tracking
	TargetP95Latency      time.Duration `json:"target_p95_latency"`
	ActualP95Latency      time.Duration `json:"actual_p95_latency"`
	LatencyViolations     int64         `json:"latency_violations"`
	
	// Routing speed tracking
	TargetRoutingLatency  time.Duration `json:"target_routing_latency"`
	ActualRoutingLatency  time.Duration `json:"actual_routing_latency"`
	RoutingViolations     int64         `json:"routing_violations"`
	
	// Fallback rate tracking
	TargetFallbackRate    float64 `json:"target_fallback_rate"`
	ActualFallbackRate    float64 `json:"actual_fallback_rate"`
	FallbackViolations    int64   `json:"fallback_violations"`
	
	// Availability tracking
	TargetAvailability    float64 `json:"target_availability"`
	ActualAvailability    float64 `json:"actual_availability"`
	AvailabilityViolations int64  `json:"availability_violations"`
	
	// Last updated
	LastUpdated           time.Time `json:"last_updated"`
}

// HourlyMetrics tracks metrics within an hour
type HourlyMetrics struct {
	Hour              int       `json:"hour"`
	RequestCount      int64     `json:"request_count"`
	SuccessCount      int64     `json:"success_count"`
	ErrorCount        int64     `json:"error_count"`
	AverageLatency    time.Duration `json:"average_latency"`
	FallbackCount     int64     `json:"fallback_count"`
	Timestamp         time.Time `json:"timestamp"`
}

// DailyMetrics tracks metrics within a day
type DailyMetrics struct {
	Date              string    `json:"date"`
	RequestCount      int64     `json:"request_count"`
	SuccessCount      int64     `json:"success_count"`
	ErrorCount        int64     `json:"error_count"`
	AverageLatency    time.Duration `json:"average_latency"`
	FallbackCount     int64     `json:"fallback_count"`
	PeakHour          int       `json:"peak_hour"`
	PeakRequests      int64     `json:"peak_requests"`
	UniqueClients     int       `json:"unique_clients"`
	Timestamp         time.Time `json:"timestamp"`
}

// RoutingMetricsSummary provides a comprehensive summary of routing metrics
type RoutingMetricsSummary struct {
	// Overall statistics
	TotalRequests         int64         `json:"total_requests"`
	SuccessfulRequests    int64         `json:"successful_requests"` 
	FailedRequests        int64         `json:"failed_requests"`
	SuccessRate           float64       `json:"success_rate"`
	
	// Performance metrics
	AverageLatency        time.Duration `json:"average_latency"`
	P50Latency            time.Duration `json:"p50_latency"`
	P95Latency            time.Duration `json:"p95_latency"`
	P99Latency            time.Duration `json:"p99_latency"`
	
	// Routing specific metrics  
	AverageRoutingLatency time.Duration `json:"average_routing_latency"`
	RoutingSuccessRate    float64       `json:"routing_success_rate"`
	FallbackUsageRate     float64       `json:"fallback_usage_rate"`
	
	// Resource utilization
	ActiveStrategies      int           `json:"active_strategies"`
	ActiveProjects        int           `json:"active_projects"`
	ActiveClients         int           `json:"active_clients"`
	
	// Error statistics
	ErrorRate             float64       `json:"error_rate"`
	MostCommonError       string        `json:"most_common_error"`
	ErrorCount            int64         `json:"error_count"`
	
	// Strategy performance
	MostUsedStrategy      string        `json:"most_used_strategy"`
	BestPerformingStrategy string       `json:"best_performing_strategy"`
	WorstPerformingStrategy string      `json:"worst_performing_strategy"`
	
	// Time-based statistics
	RequestsPerHour       float64       `json:"requests_per_hour"`
	PeakHour              int           `json:"peak_hour"`
	PeakRequests          int64         `json:"peak_requests"`
	
	// Compliance metrics
	PerformanceCompliance *PerformanceMetrics `json:"performance_compliance"`
	
	// Collection metadata
	CollectionPeriod      time.Duration `json:"collection_period"`
	LastUpdated          time.Time     `json:"last_updated"`
	DataQuality          float64       `json:"data_quality"`
}

// NewRoutingMetricsCollector creates a new metrics collector
func NewRoutingMetricsCollector(logger *mcp.StructuredLogger) *RoutingMetricsCollector {
	collector := &RoutingMetricsCollector{
		strategyUsage:         make(map[string]int64),
		strategySuccess:       make(map[string]int64),
		strategyFailure:       make(map[string]int64),
		methodCounts:          make(map[string]int64),
		methodLatency:         make(map[string]int64),
		methodErrors:          make(map[string]int64),
		projectCounts:         make(map[string]int64),
		projectLatency:        make(map[string]int64),
		projectErrors:         make(map[string]int64),
		clientUsage:           make(map[string]int64),
		clientErrors:          make(map[string]int64),
		clientLatency:         make(map[string]int64),
		errorCategories:       make(map[string]int64),
		errorsByType:          make(map[RoutingErrorType]int64),
		hourlyMetrics:         make(map[int]*HourlyMetrics),
		dailyMetrics:          make(map[string]*DailyMetrics),
		retentionDays:         30,
		maxEntriesPerCategory: 1000,
		logger:                logger,
		startTime:             time.Now(),
		lastUpdated:           time.Now(),
		performanceMetrics: &PerformanceMetrics{
			TargetSuccessRate:    99.0,
			TargetP95Latency:     500 * time.Millisecond,
			TargetRoutingLatency: 5 * time.Millisecond,
			TargetFallbackRate:   5.0,
			TargetAvailability:   99.9,
			LastUpdated:          time.Now(),
		},
	}
	
	// Initialize hourly metrics for 24 hours
	for hour := 0; hour < 24; hour++ {
		collector.hourlyMetrics[hour] = &HourlyMetrics{
			Hour:      hour,
			Timestamp: time.Now(),
		}
	}
	
	if logger != nil {
		logger.Info("RoutingMetricsCollector initialized")
	}
	
	return collector
}

// RecordRequest records a new request
func (c *RoutingMetricsCollector) RecordRequest(method, projectID, strategy string) {
	atomic.AddInt64(&c.totalRequests, 1)
	
	c.metricsLock.Lock()
	c.methodCounts[method]++
	c.projectCounts[projectID]++
	c.strategyUsage[strategy]++
	c.metricsLock.Unlock()
	
	c.updateHourlyMetrics(func(hm *HourlyMetrics) {
		hm.RequestCount++
	})
	
	c.updateDailyMetrics(func(dm *DailyMetrics) {
		dm.RequestCount++
	})
}

// RecordSuccess records a successful request
func (c *RoutingMetricsCollector) RecordSuccess(method, projectID, strategy string, latency time.Duration) {
	atomic.AddInt64(&c.successfulRequests, 1)
	atomic.AddInt64(&c.totalLatency, int64(latency))
	
	c.metricsLock.Lock()
	c.strategySuccess[strategy]++
	c.methodLatency[method] += int64(latency)
	c.projectLatency[projectID] += int64(latency)
	c.metricsLock.Unlock()
	
	c.updateHourlyMetrics(func(hm *HourlyMetrics) {
		hm.SuccessCount++
		if hm.SuccessCount == 1 {
			hm.AverageLatency = latency
		} else {
			// Simple moving average
			hm.AverageLatency = time.Duration(
				(int64(hm.AverageLatency)*hm.SuccessCount + int64(latency)) / (hm.SuccessCount + 1),
			)
		}
	})
	
	c.updateDailyMetrics(func(dm *DailyMetrics) {
		dm.SuccessCount++
		if dm.SuccessCount == 1 {
			dm.AverageLatency = latency
		} else {
			dm.AverageLatency = time.Duration(
				(int64(dm.AverageLatency)*dm.SuccessCount + int64(latency)) / (dm.SuccessCount + 1),
			)
		}
	})
	
	c.lastUpdated = time.Now()
}

// RecordFailure records a failed request
func (c *RoutingMetricsCollector) RecordFailure(method, projectID, strategy string, errorType RoutingErrorType, errorMessage string) {
	atomic.AddInt64(&c.failedRequests, 1)
	atomic.AddInt64(&c.routingErrors, 1)
	
	c.metricsLock.Lock()
	c.strategyFailure[strategy]++
	c.methodErrors[method]++
	c.projectErrors[projectID]++
	c.errorsByType[errorType]++
	c.errorCategories[errorMessage]++
	c.metricsLock.Unlock()
	
	c.updateHourlyMetrics(func(hm *HourlyMetrics) {
		hm.ErrorCount++
	})
	
	c.updateDailyMetrics(func(dm *DailyMetrics) {
		dm.ErrorCount++
	})
	
	c.lastUpdated = time.Now()
}

// RecordFallback records fallback usage
func (c *RoutingMetricsCollector) RecordFallback(method, projectID, fallbackLevel string) {
	atomic.AddInt64(&c.fallbackUsages, 1)
	
	c.metricsLock.Lock()
	c.errorCategories["fallback_"+fallbackLevel]++
	c.metricsLock.Unlock()
	
	c.updateHourlyMetrics(func(hm *HourlyMetrics) {
		hm.FallbackCount++
	})
	
	c.updateDailyMetrics(func(dm *DailyMetrics) {
		dm.FallbackCount++
	})
	
	c.lastUpdated = time.Now()
}

// RecordRoutingLatency records routing decision latency
func (c *RoutingMetricsCollector) RecordRoutingLatency(latency time.Duration) {
	atomic.AddInt64(&c.routingLatency, int64(latency))
	
	// Check against performance targets
	if c.performanceMetrics.TargetRoutingLatency > 0 && latency > c.performanceMetrics.TargetRoutingLatency {
		atomic.AddInt64(&c.performanceMetrics.RoutingViolations, 1)
	}
	
	c.lastUpdated = time.Now()
}

// RecordClientUsage records client usage statistics
func (c *RoutingMetricsCollector) RecordClientUsage(clientID string, latency time.Duration, success bool) {
	c.metricsLock.Lock()
	c.clientUsage[clientID]++
	c.clientLatency[clientID] += int64(latency)
	if !success {
		c.clientErrors[clientID]++
	}
	c.metricsLock.Unlock()
	
	c.lastUpdated = time.Now()
}

// GetSummary returns a comprehensive metrics summary
func (c *RoutingMetricsCollector) GetSummary() *RoutingMetricsSummary {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	totalReqs := atomic.LoadInt64(&c.totalRequests)
	successReqs := atomic.LoadInt64(&c.successfulRequests)
	failedReqs := atomic.LoadInt64(&c.failedRequests)
	totalLatency := atomic.LoadInt64(&c.totalLatency)
	fallbacks := atomic.LoadInt64(&c.fallbackUsages)
	
	var successRate, errorRate, fallbackRate, avgLatency float64
	
	if totalReqs > 0 {
		successRate = float64(successReqs) / float64(totalReqs) * 100
		errorRate = float64(failedReqs) / float64(totalReqs) * 100
		fallbackRate = float64(fallbacks) / float64(totalReqs) * 100
	}
	
	if successReqs > 0 {
		avgLatency = float64(totalLatency) / float64(successReqs)
	}
	
	// Find most used strategy
	var mostUsedStrategy string
	var maxUsage int64
	c.metricsLock.RLock()
	for strategy, usage := range c.strategyUsage {
		if usage > maxUsage {
			maxUsage = usage
			mostUsedStrategy = strategy
		}
	}
	
	// Find most common error
	var mostCommonError string
	var maxErrorCount int64
	for errorMsg, count := range c.errorCategories {
		if count > maxErrorCount {
			maxErrorCount = count
			mostCommonError = errorMsg
		}
	}
	c.metricsLock.RUnlock()
	
	// Calculate requests per hour
	var requestsPerHour float64
	if uptime := time.Since(c.startTime).Hours(); uptime > 0 {
		requestsPerHour = float64(totalReqs) / uptime
	}
	
	// Update performance metrics
	c.updatePerformanceMetrics(successRate, time.Duration(avgLatency), fallbackRate)
	
	return &RoutingMetricsSummary{
		TotalRequests:         totalReqs,
		SuccessfulRequests:    successReqs,
		FailedRequests:        failedReqs,
		SuccessRate:           successRate,
		AverageLatency:        time.Duration(avgLatency),
		ErrorRate:             errorRate,
		FallbackUsageRate:     fallbackRate,
		MostUsedStrategy:      mostUsedStrategy,
		MostCommonError:       mostCommonError,
		ErrorCount:            failedReqs,
		RequestsPerHour:       requestsPerHour,
		PerformanceCompliance: c.performanceMetrics,
		CollectionPeriod:      time.Since(c.startTime),
		LastUpdated:           c.lastUpdated,
		DataQuality:           c.calculateDataQuality(),
	}
}

// GetPerformanceMetrics returns current performance compliance metrics
func (c *RoutingMetricsCollector) GetPerformanceMetrics() *PerformanceMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	// Create a copy to avoid race conditions
	metrics := *c.performanceMetrics
	return &metrics
}

// GetHourlyMetrics returns metrics for a specific hour
func (c *RoutingMetricsCollector) GetHourlyMetrics(hour int) *HourlyMetrics {
	c.metricsLock.RLock()
	defer c.metricsLock.RUnlock()
	
	if metrics, exists := c.hourlyMetrics[hour]; exists {
		// Return a copy
		metricsCopy := *metrics
		return &metricsCopy
	}
	
	return nil
}

// GetDailyMetrics returns metrics for a specific date
func (c *RoutingMetricsCollector) GetDailyMetrics(date string) *DailyMetrics {
	c.metricsLock.RLock()
	defer c.metricsLock.RUnlock()
	
	if metrics, exists := c.dailyMetrics[date]; exists {
		// Return a copy
		metricsCopy := *metrics
		return &metricsCopy
	}
	
	return nil
}

// ResetMetrics resets all metrics counters
func (c *RoutingMetricsCollector) ResetMetrics() {
	atomic.StoreInt64(&c.totalRequests, 0)
	atomic.StoreInt64(&c.successfulRequests, 0)
	atomic.StoreInt64(&c.failedRequests, 0)
	atomic.StoreInt64(&c.routingErrors, 0)
	atomic.StoreInt64(&c.fallbackUsages, 0)
	atomic.StoreInt64(&c.totalLatency, 0)
	atomic.StoreInt64(&c.routingLatency, 0)
	atomic.StoreInt64(&c.executionLatency, 0)
	
	c.metricsLock.Lock()
	c.strategyUsage = make(map[string]int64)
	c.strategySuccess = make(map[string]int64)
	c.strategyFailure = make(map[string]int64)
	c.methodCounts = make(map[string]int64)
	c.methodLatency = make(map[string]int64)
	c.methodErrors = make(map[string]int64)
	c.projectCounts = make(map[string]int64)
	c.projectLatency = make(map[string]int64)
	c.projectErrors = make(map[string]int64)
	c.clientUsage = make(map[string]int64)
	c.clientErrors = make(map[string]int64)
	c.clientLatency = make(map[string]int64)
	c.errorCategories = make(map[string]int64)
	c.errorsByType = make(map[RoutingErrorType]int64)
	c.metricsLock.Unlock()
	
	c.startTime = time.Now()
	c.lastUpdated = time.Now()
	
	if c.logger != nil {
		c.logger.Info("Routing metrics reset")
	}
}

// Helper methods

func (c *RoutingMetricsCollector) updateHourlyMetrics(updateFunc func(*HourlyMetrics)) {
	hour := time.Now().Hour()
	c.metricsLock.Lock()
	if metrics, exists := c.hourlyMetrics[hour]; exists {
		updateFunc(metrics)
		metrics.Timestamp = time.Now()
	}
	c.metricsLock.Unlock()
}

func (c *RoutingMetricsCollector) updateDailyMetrics(updateFunc func(*DailyMetrics)) {
	date := time.Now().Format("2006-01-02")
	c.metricsLock.Lock()
	if metrics, exists := c.dailyMetrics[date]; exists {
		updateFunc(metrics)
	} else {
		metrics := &DailyMetrics{
			Date:      date,
			Timestamp: time.Now(),
		}
		updateFunc(metrics)
		c.dailyMetrics[date] = metrics
	}
	c.metricsLock.Unlock()
}

func (c *RoutingMetricsCollector) updatePerformanceMetrics(successRate float64, avgLatency time.Duration, fallbackRate float64) {
	c.performanceMetrics.ActualSuccessRate = successRate
	c.performanceMetrics.ActualRoutingLatency = avgLatency
	c.performanceMetrics.ActualFallbackRate = fallbackRate
	
	// Calculate availability (simplified)
	c.performanceMetrics.ActualAvailability = successRate
	
	// Check violations
	if c.performanceMetrics.TargetSuccessRate > 0 && successRate < c.performanceMetrics.TargetSuccessRate {
		atomic.AddInt64(&c.performanceMetrics.SuccessRateViolations, 1)
	}
	
	if c.performanceMetrics.TargetFallbackRate > 0 && fallbackRate > c.performanceMetrics.TargetFallbackRate {
		atomic.AddInt64(&c.performanceMetrics.FallbackViolations, 1)
	}
	
	if c.performanceMetrics.TargetAvailability > 0 && successRate < c.performanceMetrics.TargetAvailability {
		atomic.AddInt64(&c.performanceMetrics.AvailabilityViolations, 1)
	}
	
	c.performanceMetrics.LastUpdated = time.Now()
}

func (c *RoutingMetricsCollector) calculateDataQuality() float64 {
	// Simple data quality calculation based on completeness
	totalReqs := atomic.LoadInt64(&c.totalRequests)
	if totalReqs == 0 {
		return 100.0
	}
	
	// Check data consistency
	successReqs := atomic.LoadInt64(&c.successfulRequests)
	failedReqs := atomic.LoadInt64(&c.failedRequests)
	recordedReqs := successReqs + failedReqs
	
	if recordedReqs == 0 {
		return 0.0
	}
	
	// Calculate completeness ratio
	completeness := float64(recordedReqs) / float64(totalReqs) * 100
	
	// Ensure it doesn't exceed 100%
	if completeness > 100.0 {
		completeness = 100.0
	}
	
	return completeness
}