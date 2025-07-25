package gateway

import (
	"sync"
	"time"
)

// SimpleServerMetrics tracks basic performance metrics for a server instance
type SimpleServerMetrics struct {
	RequestCount        int64         `json:"request_count"`
	SuccessCount        int64         `json:"success_count"`
	FailureCount        int64         `json:"failure_count"`
	TotalResponseTime   time.Duration `json:"total_response_time"`
	LastResponseTime    time.Duration `json:"last_response_time"`
	MinResponseTime     time.Duration `json:"min_response_time"`
	MaxResponseTime     time.Duration `json:"max_response_time"`
	ActiveConnections   int32         `json:"active_connections"`
	TotalBytesProcessed int64         `json:"total_bytes_processed"`
	ErrorRate           float64       `json:"error_rate"`
	LastAccessed        time.Time     `json:"last_accessed"`
	mu                  sync.RWMutex  `json:"-"`
}

// NewSimpleServerMetrics creates a new simple server metrics instance
func NewSimpleServerMetrics() *SimpleServerMetrics {
	return &SimpleServerMetrics{
		LastAccessed:    time.Now(),
		MinResponseTime: time.Duration(^uint64(0) >> 1), // Max duration as initial min
	}
}

// RecordRequest records a request with response time and success status
func (sm *SimpleServerMetrics) RecordRequest(responseTime time.Duration, success bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.RequestCount++
	sm.LastResponseTime = responseTime
	sm.TotalResponseTime += responseTime
	sm.LastAccessed = time.Now()

	if responseTime < sm.MinResponseTime {
		sm.MinResponseTime = responseTime
	}
	if responseTime > sm.MaxResponseTime {
		sm.MaxResponseTime = responseTime
	}

	if success {
		sm.SuccessCount++
	} else {
		sm.FailureCount++
	}

	// Calculate error rate
	if sm.RequestCount > 0 {
		sm.ErrorRate = float64(sm.FailureCount) / float64(sm.RequestCount)
	}
}

// GetAverageResponseTime returns the average response time
func (sm *SimpleServerMetrics) GetAverageResponseTime() time.Duration {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.RequestCount == 0 {
		return 0
	}
	return sm.TotalResponseTime / time.Duration(sm.RequestCount)
}

// GetSuccessRate returns the success rate as a percentage
func (sm *SimpleServerMetrics) GetSuccessRate() float64 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.RequestCount == 0 {
		return 0.0
	}
	return float64(sm.SuccessCount) / float64(sm.RequestCount) * 100.0
}

// GetErrorRate returns the error rate as a percentage
func (sm *SimpleServerMetrics) GetErrorRate() float64 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.ErrorRate * 100.0
}

// Copy creates a copy of the server metrics
func (sm *SimpleServerMetrics) Copy() *SimpleServerMetrics {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return &SimpleServerMetrics{
		RequestCount:        sm.RequestCount,
		SuccessCount:        sm.SuccessCount,
		FailureCount:        sm.FailureCount,
		TotalResponseTime:   sm.TotalResponseTime,
		LastResponseTime:    sm.LastResponseTime,
		MinResponseTime:     sm.MinResponseTime,
		MaxResponseTime:     sm.MaxResponseTime,
		ActiveConnections:   sm.ActiveConnections,
		TotalBytesProcessed: sm.TotalBytesProcessed,
		ErrorRate:           sm.ErrorRate,
		LastAccessed:        sm.LastAccessed,
	}
}

// SimplePoolMetrics tracks basic performance metrics for a language server pool
type SimplePoolMetrics struct {
	Language            string                    `json:"language"`
	TotalServers        int                       `json:"total_servers"`
	ActiveServers       int                       `json:"active_servers"`
	HealthyServers      int                       `json:"healthy_servers"`
	TotalRequests       int64                     `json:"total_requests"`
	SuccessfulRequests  int64                     `json:"successful_requests"`
	FailedRequests      int64                     `json:"failed_requests"`
	AverageResponseTime time.Duration             `json:"average_response_time"`
	LoadDistribution    map[string]int64          `json:"load_distribution"`
	ServerMetrics       map[string]*SimpleServerMetrics `json:"server_metrics"`
	LastUpdated         time.Time                 `json:"last_updated"`
	mu                  sync.RWMutex              `json:"-"`
}

// NewSimplePoolMetrics creates a new simple pool metrics instance
func NewSimplePoolMetrics() *SimplePoolMetrics {
	return &SimplePoolMetrics{
		LoadDistribution: make(map[string]int64),
		ServerMetrics:    make(map[string]*SimpleServerMetrics),
		LastUpdated:      time.Now(),
	}
}

// Copy creates a copy of the pool metrics
func (pm *SimplePoolMetrics) Copy() *SimplePoolMetrics {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	loadDist := make(map[string]int64)
	for k, v := range pm.LoadDistribution {
		loadDist[k] = v
	}

	serverMets := make(map[string]*SimpleServerMetrics)
	for k, v := range pm.ServerMetrics {
		serverMets[k] = v.Copy()
	}

	return &SimplePoolMetrics{
		Language:            pm.Language,
		TotalServers:        pm.TotalServers,
		ActiveServers:       pm.ActiveServers,
		HealthyServers:      pm.HealthyServers,
		TotalRequests:       pm.TotalRequests,
		SuccessfulRequests:  pm.SuccessfulRequests,
		FailedRequests:      pm.FailedRequests,
		AverageResponseTime: pm.AverageResponseTime,
		LoadDistribution:    loadDist,
		ServerMetrics:       serverMets,
		LastUpdated:         pm.LastUpdated,
	}
}

// RecordServerSelection records a server selection event with performance metrics
func (pm *SimplePoolMetrics) RecordServerSelection(serverName string, responseTime time.Duration, success bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Update overall pool request statistics
	pm.TotalRequests++
	if success {
		pm.SuccessfulRequests++
	} else {
		pm.FailedRequests++
	}

	// Update load distribution for server selection tracking
	pm.LoadDistribution[serverName]++

	// Update average response time using exponential moving average
	if pm.AverageResponseTime == 0 {
		pm.AverageResponseTime = responseTime
	} else {
		alpha := 0.1 // Smoothing factor for exponential moving average  
		pm.AverageResponseTime = time.Duration(float64(pm.AverageResponseTime)*(1-alpha) + float64(responseTime)*alpha)
	}

	// Ensure server metrics exist
	if pm.ServerMetrics[serverName] == nil {
		pm.ServerMetrics[serverName] = NewSimpleServerMetrics()
	}

	// Record server-specific metrics
	pm.ServerMetrics[serverName].RecordRequest(responseTime, success)

	pm.LastUpdated = time.Now()
}

// UpdatePoolStatus updates the pool status metrics for SCIP integration
func (pm *SimplePoolMetrics) UpdatePoolStatus(language string, total, active, healthy int, strategy string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.Language = language
	pm.TotalServers = total
	pm.ActiveServers = active
	pm.HealthyServers = healthy
	pm.LastUpdated = time.Now()
}
