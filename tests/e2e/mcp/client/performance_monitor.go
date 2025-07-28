package client

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type PerformanceMonitor struct {
	logger          *TestLogger
	enabled         bool
	metrics         *PerformanceMetrics
	latencyTracker  *LatencyTracker
	throughputTracker *ThroughputTracker
	errorTracker    *ErrorTracker
	
	mu              sync.RWMutex
	shutdown        chan struct{}
	shutdownOnce    sync.Once
	monitoringTicker *time.Ticker
}

type PerformanceMetrics struct {
	ConnectionMetrics  *ConnectionPerformanceMetrics
	RequestMetrics     *RequestPerformanceMetrics
	LatencyMetrics     *LatencyMetrics
	ThroughputMetrics  *ThroughputMetrics
	ErrorMetrics       *ErrorPerformanceMetrics
	ResourceMetrics    *ResourceMetrics
	mu                 sync.RWMutex
}

type ConnectionPerformanceMetrics struct {
	TotalAttempts       int64
	SuccessfulAttempts  int64
	FailedAttempts      int64
	AverageConnectTime  time.Duration
	FastestConnectTime  time.Duration
	SlowestConnectTime  time.Duration
	ConnectionUptime    time.Duration
	ReconnectionCount   int64
	LastConnectionTime  time.Time
	mu                  sync.RWMutex
}

type RequestPerformanceMetrics struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	TimeoutRequests    int64
	CanceledRequests   int64
	RequestsPerSecond  float64
	AverageLatency     time.Duration
	SuccessRate        float64
	MethodMetrics      map[string]*MethodPerformanceMetrics
	mu                 sync.RWMutex
}

type MethodPerformanceMetrics struct {
	RequestCount      int64
	SuccessCount      int64
	FailureCount      int64
	AverageLatency    time.Duration
	MinLatency        time.Duration
	MaxLatency        time.Duration
	SuccessRate       float64
	LastRequestTime   time.Time
	mu                sync.RWMutex
}

type LatencyMetrics struct {
	P50Latency        time.Duration
	P90Latency        time.Duration
	P95Latency        time.Duration
	P99Latency        time.Duration
	P999Latency       time.Duration
	MinLatency        time.Duration
	MaxLatency        time.Duration
	AverageLatency    time.Duration
	SampleCount       int64
	mu                sync.RWMutex
}

type ThroughputMetrics struct {
	RequestsPerSecond    float64
	ResponsesPerSecond   float64
	BytesPerSecond       float64
	PeakRequestsPerSec   float64
	PeakResponsesPerSec  float64
	CurrentWindowRPS     float64
	LastMeasurementTime  time.Time
	mu                   sync.RWMutex
}

type ErrorPerformanceMetrics struct {
	TotalErrors       int64
	ErrorRate         float64
	ErrorsByCategory  map[string]int64
	RecentErrors      []ErrorEvent
	MaxRecentErrors   int
	mu                sync.RWMutex
}

type ResourceMetrics struct {
	MemoryUsage       int64
	CPUUsage          float64
	GoroutineCount    int64
	OpenConnections   int64
	BufferUtilization float64
	mu                sync.RWMutex
}

type LatencyTracker struct {
	samples       []time.Duration
	maxSamples    int
	currentIndex  int
	full          bool
	mu            sync.RWMutex
}

type ThroughputTracker struct {
	requestTimes  []time.Time
	responseTimes []time.Time
	windowSize    time.Duration
	mu            sync.RWMutex
}

type ErrorTracker struct {
	errorEvents   []ErrorEvent
	maxEvents     int
	errorCounts   map[string]int64
	mu            sync.RWMutex
}

type ErrorEvent struct {
	Timestamp   time.Time
	Error       string
	Category    string
	Method      string
	Latency     time.Duration
	Retryable   bool
}

const (
	DefaultMaxLatencySamples = 10000
	DefaultMaxErrorEvents    = 1000
	DefaultThroughputWindow  = 60 * time.Second
	MonitoringInterval       = 5 * time.Second
	PercentileAccuracy       = 0.01
)

func NewPerformanceMonitor(enabled bool, logger *TestLogger) *PerformanceMonitor {
	pm := &PerformanceMonitor{
		logger:   logger,
		enabled:  enabled,
		shutdown: make(chan struct{}),
		metrics: &PerformanceMetrics{
			ConnectionMetrics: &ConnectionPerformanceMetrics{
				FastestConnectTime: time.Hour, // Initialize to high value
			},
			RequestMetrics: &RequestPerformanceMetrics{
				MethodMetrics: make(map[string]*MethodPerformanceMetrics),
			},
			LatencyMetrics:    &LatencyMetrics{},
			ThroughputMetrics: &ThroughputMetrics{},
			ErrorMetrics: &ErrorPerformanceMetrics{
				ErrorsByCategory: make(map[string]int64),
				RecentErrors:     make([]ErrorEvent, 0),
				MaxRecentErrors:  DefaultMaxErrorEvents,
			},
			ResourceMetrics: &ResourceMetrics{},
		},
	}
	
	pm.latencyTracker = NewLatencyTracker(DefaultMaxLatencySamples)
	pm.throughputTracker = NewThroughputTracker(DefaultThroughputWindow)
	pm.errorTracker = NewErrorTracker(DefaultMaxErrorEvents)
	
	if enabled {
		pm.startMonitoring()
	}
	
	return pm
}

func (pm *PerformanceMonitor) RecordConnectionAttempt(success bool, duration time.Duration) {
	if !pm.enabled {
		return
	}
	
	pm.metrics.ConnectionMetrics.mu.Lock()
	defer pm.metrics.ConnectionMetrics.mu.Unlock()
	
	atomic.AddInt64(&pm.metrics.ConnectionMetrics.TotalAttempts, 1)
	
	if success {
		atomic.AddInt64(&pm.metrics.ConnectionMetrics.SuccessfulAttempts, 1)
		pm.metrics.ConnectionMetrics.LastConnectionTime = time.Now()
		
		// Update connection time statistics
		if pm.metrics.ConnectionMetrics.AverageConnectTime == 0 {
			pm.metrics.ConnectionMetrics.AverageConnectTime = duration
		} else {
			pm.metrics.ConnectionMetrics.AverageConnectTime = 
				(pm.metrics.ConnectionMetrics.AverageConnectTime + duration) / 2
		}
		
		if duration < pm.metrics.ConnectionMetrics.FastestConnectTime {
			pm.metrics.ConnectionMetrics.FastestConnectTime = duration
		}
		
		if duration > pm.metrics.ConnectionMetrics.SlowestConnectTime {
			pm.metrics.ConnectionMetrics.SlowestConnectTime = duration
		}
	} else {
		atomic.AddInt64(&pm.metrics.ConnectionMetrics.FailedAttempts, 1)
	}
	
	pm.logger.Debug("Connection attempt recorded", "success", success, "duration", duration)
}

func (pm *PerformanceMonitor) RecordRequestStart(method string) {
	if !pm.enabled {
		return
	}
	
	pm.throughputTracker.RecordRequestStart()
	pm.updateMethodMetrics(method, "start")
}

func (pm *PerformanceMonitor) RecordRequestComplete(method string, success bool, latency time.Duration) {
	if !pm.enabled {
		return
	}
	
	pm.metrics.RequestMetrics.mu.Lock()
	atomic.AddInt64(&pm.metrics.RequestMetrics.TotalRequests, 1)
	if success {
		atomic.AddInt64(&pm.metrics.RequestMetrics.SuccessfulRequests, 1)
	} else {
		atomic.AddInt64(&pm.metrics.RequestMetrics.FailedRequests, 1)
	}
	pm.metrics.RequestMetrics.mu.Unlock()
	
	pm.latencyTracker.RecordLatency(latency)
	pm.throughputTracker.RecordRequestComplete()
	pm.updateMethodMetrics(method, "complete")
	pm.updateMethodLatency(method, latency, success)
	
	pm.updateLatencyMetrics()
	pm.updateSuccessRate()
	
	pm.logger.Debug("Request completed", "method", method, "success", success, "latency", latency)
}

func (pm *PerformanceMonitor) RecordError(method, errorMsg, category string, latency time.Duration, retryable bool) {
	if !pm.enabled {
		return
	}
	
	errorEvent := ErrorEvent{
		Timestamp: time.Now(),
		Error:     errorMsg,
		Category:  category,
		Method:    method,
		Latency:   latency,
		Retryable: retryable,
	}
	
	pm.errorTracker.RecordError(errorEvent)
	pm.updateErrorMetrics(category)
	
	pm.logger.Debug("Error recorded", "method", method, "category", category, "retryable", retryable)
}

func (pm *PerformanceMonitor) GetMetrics() *PerformanceMetrics {
	if !pm.enabled {
		return &PerformanceMetrics{}
	}
	
	pm.metrics.mu.RLock()
	defer pm.metrics.mu.RUnlock()
	
	// Create deep copy of metrics
	methodMetrics := make(map[string]*MethodPerformanceMetrics)
	for method, metrics := range pm.metrics.RequestMetrics.MethodMetrics {
		methodMetrics[method] = &MethodPerformanceMetrics{
			RequestCount:    atomic.LoadInt64(&metrics.RequestCount),
			SuccessCount:    atomic.LoadInt64(&metrics.SuccessCount),
			FailureCount:    atomic.LoadInt64(&metrics.FailureCount),
			AverageLatency:  metrics.AverageLatency,
			MinLatency:      metrics.MinLatency,
			MaxLatency:      metrics.MaxLatency,
			SuccessRate:     metrics.SuccessRate,
			LastRequestTime: metrics.LastRequestTime,
		}
	}
	
	errorsByCategory := make(map[string]int64)
	for category, count := range pm.metrics.ErrorMetrics.ErrorsByCategory {
		errorsByCategory[category] = count
	}
	
	recentErrors := make([]ErrorEvent, len(pm.metrics.ErrorMetrics.RecentErrors))
	copy(recentErrors, pm.metrics.ErrorMetrics.RecentErrors)
	
	return &PerformanceMetrics{
		ConnectionMetrics: &ConnectionPerformanceMetrics{
			TotalAttempts:      atomic.LoadInt64(&pm.metrics.ConnectionMetrics.TotalAttempts),
			SuccessfulAttempts: atomic.LoadInt64(&pm.metrics.ConnectionMetrics.SuccessfulAttempts),
			FailedAttempts:     atomic.LoadInt64(&pm.metrics.ConnectionMetrics.FailedAttempts),
			AverageConnectTime: pm.metrics.ConnectionMetrics.AverageConnectTime,
			FastestConnectTime: pm.metrics.ConnectionMetrics.FastestConnectTime,
			SlowestConnectTime: pm.metrics.ConnectionMetrics.SlowestConnectTime,
			ConnectionUptime:   pm.calculateUptime(),
			ReconnectionCount:  atomic.LoadInt64(&pm.metrics.ConnectionMetrics.ReconnectionCount),
			LastConnectionTime: pm.metrics.ConnectionMetrics.LastConnectionTime,
		},
		RequestMetrics: &RequestPerformanceMetrics{
			TotalRequests:      atomic.LoadInt64(&pm.metrics.RequestMetrics.TotalRequests),
			SuccessfulRequests: atomic.LoadInt64(&pm.metrics.RequestMetrics.SuccessfulRequests),
			FailedRequests:     atomic.LoadInt64(&pm.metrics.RequestMetrics.FailedRequests),
			TimeoutRequests:    atomic.LoadInt64(&pm.metrics.RequestMetrics.TimeoutRequests),
			CanceledRequests:   atomic.LoadInt64(&pm.metrics.RequestMetrics.CanceledRequests),
			RequestsPerSecond:  pm.calculateRequestsPerSecond(),
			AverageLatency:     pm.metrics.RequestMetrics.AverageLatency,
			SuccessRate:        pm.metrics.RequestMetrics.SuccessRate,
			MethodMetrics:      methodMetrics,
		},
		LatencyMetrics:    pm.calculateLatencyPercentiles(),
		ThroughputMetrics: pm.calculateThroughputMetrics(),
		ErrorMetrics: &ErrorPerformanceMetrics{
			TotalErrors:      atomic.LoadInt64(&pm.metrics.ErrorMetrics.TotalErrors),
			ErrorRate:        pm.calculateErrorRate(),
			ErrorsByCategory: errorsByCategory,
			RecentErrors:     recentErrors,
			MaxRecentErrors:  pm.metrics.ErrorMetrics.MaxRecentErrors,
		},
		ResourceMetrics: pm.gatherResourceMetrics(),
	}
}

func (pm *PerformanceMonitor) startMonitoring() {
	pm.monitoringTicker = time.NewTicker(MonitoringInterval)
	go pm.monitoringLoop()
	pm.logger.Info("Performance monitoring started")
}

func (pm *PerformanceMonitor) monitoringLoop() {
	for {
		select {
		case <-pm.monitoringTicker.C:
			pm.updatePerformanceMetrics()
		case <-pm.shutdown:
			return
		}
	}
}

func (pm *PerformanceMonitor) updatePerformanceMetrics() {
	pm.updateLatencyMetrics()
	pm.updateThroughputMetrics()
	pm.updateSuccessRate()
	pm.cleanupOldData()
}

func (pm *PerformanceMonitor) updateMethodMetrics(method, eventType string) {
	pm.metrics.RequestMetrics.mu.Lock()
	defer pm.metrics.RequestMetrics.mu.Unlock()
	
	metrics, exists := pm.metrics.RequestMetrics.MethodMetrics[method]
	if !exists {
		metrics = &MethodPerformanceMetrics{
			MinLatency: time.Hour, // Initialize to high value
		}
		pm.metrics.RequestMetrics.MethodMetrics[method] = metrics
	}
	
	switch eventType {
	case "start":
		atomic.AddInt64(&metrics.RequestCount, 1)
		metrics.LastRequestTime = time.Now()
	case "complete":
		// Handled in RecordRequestComplete
	}
}

func (pm *PerformanceMonitor) updateMethodLatency(method string, latency time.Duration, success bool) {
	pm.metrics.RequestMetrics.mu.Lock()
	defer pm.metrics.RequestMetrics.mu.Unlock()
	
	metrics, exists := pm.metrics.RequestMetrics.MethodMetrics[method]
	if !exists {
		return
	}
	
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	
	if success {
		atomic.AddInt64(&metrics.SuccessCount, 1)
	} else {
		atomic.AddInt64(&metrics.FailureCount, 1)
	}
	
	// Update latency statistics
	if metrics.AverageLatency == 0 {
		metrics.AverageLatency = latency
	} else {
		metrics.AverageLatency = (metrics.AverageLatency + latency) / 2
	}
	
	if latency < metrics.MinLatency {
		metrics.MinLatency = latency
	}
	
	if latency > metrics.MaxLatency {
		metrics.MaxLatency = latency
	}
	
	// Update success rate
	totalRequests := atomic.LoadInt64(&metrics.RequestCount)
	successfulRequests := atomic.LoadInt64(&metrics.SuccessCount)
	if totalRequests > 0 {
		metrics.SuccessRate = float64(successfulRequests) / float64(totalRequests) * 100
	}
}

func (pm *PerformanceMonitor) updateLatencyMetrics() {
	percentiles := pm.latencyTracker.CalculatePercentiles()
	
	pm.metrics.LatencyMetrics.mu.Lock()
	defer pm.metrics.LatencyMetrics.mu.Unlock()
	
	pm.metrics.LatencyMetrics.P50Latency = percentiles[50]
	pm.metrics.LatencyMetrics.P90Latency = percentiles[90]
	pm.metrics.LatencyMetrics.P95Latency = percentiles[95]
	pm.metrics.LatencyMetrics.P99Latency = percentiles[99]
	pm.metrics.LatencyMetrics.P999Latency = percentiles[99.9]
	pm.metrics.LatencyMetrics.MinLatency = pm.latencyTracker.GetMin()
	pm.metrics.LatencyMetrics.MaxLatency = pm.latencyTracker.GetMax()
	pm.metrics.LatencyMetrics.AverageLatency = pm.latencyTracker.GetAverage()
	atomic.StoreInt64(&pm.metrics.LatencyMetrics.SampleCount, int64(pm.latencyTracker.GetSampleCount()))
}

func (pm *PerformanceMonitor) updateThroughputMetrics() {
	pm.metrics.ThroughputMetrics.mu.Lock()
	defer pm.metrics.ThroughputMetrics.mu.Unlock()
	
	pm.metrics.ThroughputMetrics.RequestsPerSecond = pm.throughputTracker.GetRequestsPerSecond()
	pm.metrics.ThroughputMetrics.ResponsesPerSecond = pm.throughputTracker.GetResponsesPerSecond()
	pm.metrics.ThroughputMetrics.CurrentWindowRPS = pm.throughputTracker.GetCurrentWindowRPS()
	pm.metrics.ThroughputMetrics.LastMeasurementTime = time.Now()
	
	// Update peak values
	if pm.metrics.ThroughputMetrics.RequestsPerSecond > pm.metrics.ThroughputMetrics.PeakRequestsPerSec {
		pm.metrics.ThroughputMetrics.PeakRequestsPerSec = pm.metrics.ThroughputMetrics.RequestsPerSecond
	}
	
	if pm.metrics.ThroughputMetrics.ResponsesPerSecond > pm.metrics.ThroughputMetrics.PeakResponsesPerSec {
		pm.metrics.ThroughputMetrics.PeakResponsesPerSec = pm.metrics.ThroughputMetrics.ResponsesPerSecond
	}
}

func (pm *PerformanceMonitor) updateSuccessRate() {
	totalRequests := atomic.LoadInt64(&pm.metrics.RequestMetrics.TotalRequests)
	successfulRequests := atomic.LoadInt64(&pm.metrics.RequestMetrics.SuccessfulRequests)
	
	if totalRequests > 0 {
		pm.metrics.RequestMetrics.SuccessRate = float64(successfulRequests) / float64(totalRequests) * 100
	}
}

func (pm *PerformanceMonitor) updateErrorMetrics(category string) {
	pm.metrics.ErrorMetrics.mu.Lock()
	defer pm.metrics.ErrorMetrics.mu.Unlock()
	
	atomic.AddInt64(&pm.metrics.ErrorMetrics.TotalErrors, 1)
	pm.metrics.ErrorMetrics.ErrorsByCategory[category]++
}

func (pm *PerformanceMonitor) calculateUptime() time.Duration {
	pm.metrics.ConnectionMetrics.mu.RLock()
	defer pm.metrics.ConnectionMetrics.mu.RUnlock()
	
	if pm.metrics.ConnectionMetrics.LastConnectionTime.IsZero() {
		return 0
	}
	return time.Since(pm.metrics.ConnectionMetrics.LastConnectionTime)
}

func (pm *PerformanceMonitor) calculateRequestsPerSecond() float64 {
	return pm.throughputTracker.GetRequestsPerSecond()
}

func (pm *PerformanceMonitor) calculateLatencyPercentiles() *LatencyMetrics {
	percentiles := pm.latencyTracker.CalculatePercentiles()
	
	return &LatencyMetrics{
		P50Latency:     percentiles[50],
		P90Latency:     percentiles[90],
		P95Latency:     percentiles[95],
		P99Latency:     percentiles[99],
		P999Latency:    percentiles[99.9],
		MinLatency:     pm.latencyTracker.GetMin(),
		MaxLatency:     pm.latencyTracker.GetMax(),
		AverageLatency: pm.latencyTracker.GetAverage(),
		SampleCount:    int64(pm.latencyTracker.GetSampleCount()),
	}
}

func (pm *PerformanceMonitor) calculateThroughputMetrics() *ThroughputMetrics {
	return &ThroughputMetrics{
		RequestsPerSecond:   pm.throughputTracker.GetRequestsPerSecond(),
		ResponsesPerSecond:  pm.throughputTracker.GetResponsesPerSecond(),
		CurrentWindowRPS:    pm.throughputTracker.GetCurrentWindowRPS(),
		PeakRequestsPerSec:  pm.metrics.ThroughputMetrics.PeakRequestsPerSec,
		PeakResponsesPerSec: pm.metrics.ThroughputMetrics.PeakResponsesPerSec,
		LastMeasurementTime: time.Now(),
	}
}

func (pm *PerformanceMonitor) calculateErrorRate() float64 {
	totalRequests := atomic.LoadInt64(&pm.metrics.RequestMetrics.TotalRequests)
	totalErrors := atomic.LoadInt64(&pm.metrics.ErrorMetrics.TotalErrors)
	
	if totalRequests > 0 {
		return float64(totalErrors) / float64(totalRequests) * 100
	}
	return 0.0
}

func (pm *PerformanceMonitor) gatherResourceMetrics() *ResourceMetrics {
	// Note: In a real implementation, you would gather actual system metrics
	// For this test implementation, we'll return placeholder values
	return &ResourceMetrics{
		MemoryUsage:       0, // Would use runtime.MemStats
		CPUUsage:          0, // Would use system CPU metrics
		GoroutineCount:    0, // Would use runtime.NumGoroutine()
		OpenConnections:   0, // Would track actual connection count
		BufferUtilization: 0, // Would calculate based on buffer usage
	}
}

func (pm *PerformanceMonitor) cleanupOldData() {
	// Clean up old error events
	pm.errorTracker.CleanupOldEvents(time.Hour)
	
	// Clean up old throughput data
	pm.throughputTracker.CleanupOldData()
}

func (pm *PerformanceMonitor) Close() error {
	pm.shutdownOnce.Do(func() {
		close(pm.shutdown)
	})
	
	if pm.monitoringTicker != nil {
		pm.monitoringTicker.Stop()
	}
	
	pm.logger.Info("Performance monitor closed")
	return nil
}

func NewLatencyTracker(maxSamples int) *LatencyTracker {
	return &LatencyTracker{
		samples:    make([]time.Duration, maxSamples),
		maxSamples: maxSamples,
	}
}

func (lt *LatencyTracker) RecordLatency(latency time.Duration) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	
	lt.samples[lt.currentIndex] = latency
	lt.currentIndex = (lt.currentIndex + 1) % lt.maxSamples
	
	if !lt.full && lt.currentIndex == 0 {
		lt.full = true
	}
}

func (lt *LatencyTracker) CalculatePercentiles() map[float64]time.Duration {
	lt.mu.RLock()
	defer lt.mu.RUnlock()
	
	sampleCount := lt.getSampleCount()
	if sampleCount == 0 {
		return map[float64]time.Duration{}
	}
	
	samples := make([]time.Duration, sampleCount)
	if lt.full {
		copy(samples, lt.samples)
	} else {
		copy(samples, lt.samples[:lt.currentIndex])
	}
	
	sort.Slice(samples, func(i, j int) bool {
		return samples[i] < samples[j]
	})
	
	percentiles := map[float64]time.Duration{
		50:   samples[int(float64(sampleCount)*0.50)],
		90:   samples[int(float64(sampleCount)*0.90)],
		95:   samples[int(float64(sampleCount)*0.95)],
		99:   samples[int(float64(sampleCount)*0.99)],
		99.9: samples[int(float64(sampleCount)*0.999)],
	}
	
	return percentiles
}

func (lt *LatencyTracker) GetAverage() time.Duration {
	lt.mu.RLock()
	defer lt.mu.RUnlock()
	
	sampleCount := lt.getSampleCount()
	if sampleCount == 0 {
		return 0
	}
	
	var total time.Duration
	if lt.full {
		for _, sample := range lt.samples {
			total += sample
		}
	} else {
		for i := 0; i < lt.currentIndex; i++ {
			total += lt.samples[i]
		}
	}
	
	return total / time.Duration(sampleCount)
}

func (lt *LatencyTracker) GetMin() time.Duration {
	lt.mu.RLock()
	defer lt.mu.RUnlock()
	
	sampleCount := lt.getSampleCount()
	if sampleCount == 0 {
		return 0
	}
	
	min := lt.samples[0]
	if lt.full {
		for _, sample := range lt.samples {
			if sample < min {
				min = sample
			}
		}
	} else {
		for i := 0; i < lt.currentIndex; i++ {
			if lt.samples[i] < min {
				min = lt.samples[i]
			}
		}
	}
	
	return min
}

func (lt *LatencyTracker) GetMax() time.Duration {
	lt.mu.RLock()
	defer lt.mu.RUnlock()
	
	sampleCount := lt.getSampleCount()
	if sampleCount == 0 {
		return 0
	}
	
	max := lt.samples[0]
	if lt.full {
		for _, sample := range lt.samples {
			if sample > max {
				max = sample
			}
		}
	} else {
		for i := 0; i < lt.currentIndex; i++ {
			if lt.samples[i] > max {
				max = lt.samples[i]
			}
		}
	}
	
	return max
}

func (lt *LatencyTracker) GetSampleCount() int {
	lt.mu.RLock()
	defer lt.mu.RUnlock()
	return lt.getSampleCount()
}

func (lt *LatencyTracker) getSampleCount() int {
	if lt.full {
		return lt.maxSamples
	}
	return lt.currentIndex
}

func NewThroughputTracker(windowSize time.Duration) *ThroughputTracker {
	return &ThroughputTracker{
		requestTimes:  make([]time.Time, 0),
		responseTimes: make([]time.Time, 0),
		windowSize:    windowSize,
	}
}

func (tt *ThroughputTracker) RecordRequestStart() {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	tt.requestTimes = append(tt.requestTimes, time.Now())
}

func (tt *ThroughputTracker) RecordRequestComplete() {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	tt.responseTimes = append(tt.responseTimes, time.Now())
}

func (tt *ThroughputTracker) GetRequestsPerSecond() float64 {
	tt.mu.RLock()
	defer tt.mu.RUnlock()
	
	now := time.Now()
	cutoff := now.Add(-tt.windowSize)
	
	count := 0
	for _, requestTime := range tt.requestTimes {
		if requestTime.After(cutoff) {
			count++
		}
	}
	
	return float64(count) / tt.windowSize.Seconds()
}

func (tt *ThroughputTracker) GetResponsesPerSecond() float64 {
	tt.mu.RLock()
	defer tt.mu.RUnlock()
	
	now := time.Now()
	cutoff := now.Add(-tt.windowSize)
	
	count := 0
	for _, responseTime := range tt.responseTimes {
		if responseTime.After(cutoff) {
			count++
		}
	}
	
	return float64(count) / tt.windowSize.Seconds()
}

func (tt *ThroughputTracker) GetCurrentWindowRPS() float64 {
	return tt.GetRequestsPerSecond()
}

func (tt *ThroughputTracker) CleanupOldData() {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	
	now := time.Now()
	cutoff := now.Add(-tt.windowSize * 2) // Keep twice the window size for analysis
	
	// Clean up old request times
	validRequests := make([]time.Time, 0, len(tt.requestTimes))
	for _, requestTime := range tt.requestTimes {
		if requestTime.After(cutoff) {
			validRequests = append(validRequests, requestTime)
		}
	}
	tt.requestTimes = validRequests
	
	// Clean up old response times
	validResponses := make([]time.Time, 0, len(tt.responseTimes))
	for _, responseTime := range tt.responseTimes {
		if responseTime.After(cutoff) {
			validResponses = append(validResponses, responseTime)
		}
	}
	tt.responseTimes = validResponses
}

func NewErrorTracker(maxEvents int) *ErrorTracker {
	return &ErrorTracker{
		errorEvents: make([]ErrorEvent, 0),
		maxEvents:   maxEvents,
		errorCounts: make(map[string]int64),
	}
}

func (et *ErrorTracker) RecordError(event ErrorEvent) {
	et.mu.Lock()
	defer et.mu.Unlock()
	
	et.errorEvents = append(et.errorEvents, event)
	et.errorCounts[event.Category]++
	
	// Keep only the most recent events
	if len(et.errorEvents) > et.maxEvents {
		et.errorEvents = et.errorEvents[len(et.errorEvents)-et.maxEvents:]
	}
}

func (et *ErrorTracker) CleanupOldEvents(maxAge time.Duration) {
	et.mu.Lock()
	defer et.mu.Unlock()
	
	cutoff := time.Now().Add(-maxAge)
	validEvents := make([]ErrorEvent, 0, len(et.errorEvents))
	
	for _, event := range et.errorEvents {
		if event.Timestamp.After(cutoff) {
			validEvents = append(validEvents, event)
		}
	}
	
	et.errorEvents = validEvents
}