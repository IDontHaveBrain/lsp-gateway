package gateway

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ServerMetrics tracks performance metrics for a server instance
type ServerMetrics struct {
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

// NewServerMetrics creates a new server metrics instance
func NewServerMetrics() *ServerMetrics {
	return &ServerMetrics{
		LastAccessed:    time.Now(),
		MinResponseTime: time.Duration(^uint64(0) >> 1), // Max duration as initial min
	}
}

// RecordRequest records a request with response time and success status
func (sm *ServerMetrics) RecordRequest(responseTime time.Duration, success bool) {
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
func (sm *ServerMetrics) GetAverageResponseTime() time.Duration {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.RequestCount == 0 {
		return 0
	}
	return sm.TotalResponseTime / time.Duration(sm.RequestCount)
}

// GetSuccessRate returns the success rate as a percentage
func (sm *ServerMetrics) GetSuccessRate() float64 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.RequestCount == 0 {
		return 0.0
	}
	return float64(sm.SuccessCount) / float64(sm.RequestCount) * 100.0
}

// Copy creates a copy of the server metrics
func (sm *ServerMetrics) Copy() *ServerMetrics {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return &ServerMetrics{
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

// PoolMetrics tracks performance metrics for a language server pool
type PoolMetrics struct {
	Language            string                    `json:"language"`
	TotalServers        int                       `json:"total_servers"`
	ActiveServers       int                       `json:"active_servers"`
	HealthyServers      int                       `json:"healthy_servers"`
	TotalRequests       int64                     `json:"total_requests"`
	SuccessfulRequests  int64                     `json:"successful_requests"`
	FailedRequests      int64                     `json:"failed_requests"`
	AverageResponseTime time.Duration             `json:"average_response_time"`
	LoadDistribution    map[string]int64          `json:"load_distribution"`
	ServerMetrics       map[string]*ServerMetrics `json:"server_metrics"`
	LastUpdated         time.Time                 `json:"last_updated"`
	mu                  sync.RWMutex              `json:"-"`
}

// NewPoolMetrics creates a new pool metrics instance
func NewPoolMetrics() *PoolMetrics {
	return &PoolMetrics{
		LoadDistribution: make(map[string]int64),
		ServerMetrics:    make(map[string]*ServerMetrics),
		LastUpdated:      time.Now(),
	}
}

// Copy creates a copy of the pool metrics
func (pm *PoolMetrics) Copy() *PoolMetrics {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	loadDist := make(map[string]int64)
	for k, v := range pm.LoadDistribution {
		loadDist[k] = v
	}

	serverMets := make(map[string]*ServerMetrics)
	for k, v := range pm.ServerMetrics {
		serverMets[k] = v.Copy()
	}

	return &PoolMetrics{
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

// ManagerMetrics tracks overall performance metrics for the MultiServerManager
type ManagerMetrics struct {
	TotalPools          int                     `json:"total_pools"`
	TotalServers        int                     `json:"total_servers"`
	ActiveServers       int                     `json:"active_servers"`
	HealthyServers      int                     `json:"healthy_servers"`
	TotalRequests       int64                   `json:"total_requests"`
	SuccessfulRequests  int64                   `json:"successful_requests"`
	FailedRequests      int64                   `json:"failed_requests"`
	AverageResponseTime time.Duration           `json:"average_response_time"`
	PoolMetrics         map[string]*PoolMetrics `json:"pool_metrics"`
	Uptime              time.Duration           `json:"uptime"`
	StartTime           time.Time               `json:"start_time"`
	LastUpdated         time.Time               `json:"last_updated"`
	mu                  sync.RWMutex            `json:"-"`
}

// NewManagerMetrics creates a new manager metrics instance
func NewManagerMetrics() *ManagerMetrics {
	return &ManagerMetrics{
		PoolMetrics: make(map[string]*PoolMetrics),
		StartTime:   time.Now(),
		LastUpdated: time.Now(),
	}
}

// Copy creates a copy of the manager metrics
func (mm *ManagerMetrics) Copy() *ManagerMetrics {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	poolMets := make(map[string]*PoolMetrics)
	for k, v := range mm.PoolMetrics {
		poolMets[k] = v.Copy()
	}

	return &ManagerMetrics{
		TotalPools:          mm.TotalPools,
		TotalServers:        mm.TotalServers,
		ActiveServers:       mm.ActiveServers,
		HealthyServers:      mm.HealthyServers,
		TotalRequests:       mm.TotalRequests,
		SuccessfulRequests:  mm.SuccessfulRequests,
		FailedRequests:      mm.FailedRequests,
		AverageResponseTime: mm.AverageResponseTime,
		PoolMetrics:         poolMets,
		Uptime:              time.Since(mm.StartTime),
		StartTime:           mm.StartTime,
		LastUpdated:         mm.LastUpdated,
	}
}

// LoadBalancer handles server selection within a pool
type LoadBalancer struct {
	strategy string
	mu       sync.RWMutex
}

// NewLoadBalancer creates a new load balancer with the specified strategy
func NewLoadBalancer(strategy string) *LoadBalancer {
	return &LoadBalancer{
		strategy: strategy,
	}
}

// SelectServer selects a server from the pool based on the load balancing strategy
func (lb *LoadBalancer) SelectServer(pool *LanguageServerPool, requestType string) (*ServerInstance, error) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	healthyServers := pool.GetHealthyServers()
	if len(healthyServers) == 0 {
		return nil, fmt.Errorf("no healthy servers available")
	}

	// Simple round-robin selection for now
	// In a full implementation, this would use different strategies
	return healthyServers[0], nil
}

// SelectMultipleServers selects multiple servers for concurrent requests
func (lb *LoadBalancer) SelectMultipleServers(pool *LanguageServerPool, maxServers int) ([]*ServerInstance, error) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	healthyServers := pool.GetHealthyServers()
	if len(healthyServers) == 0 {
		return nil, fmt.Errorf("no healthy servers available")
	}

	// Return up to maxServers healthy servers
	count := len(healthyServers)
	if count > maxServers {
		count = maxServers
	}

	return healthyServers[:count], nil
}

// ResourceMonitor monitors system resources
type ResourceMonitor struct {
	mu sync.RWMutex
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor() *ResourceMonitor {
	return &ResourceMonitor{}
}

// StartMonitoring starts resource monitoring (stub implementation)
func (rm *ResourceMonitor) StartMonitoring(ctx context.Context) error {
	// Stub implementation - would monitor CPU, memory, etc.
	return nil
}
