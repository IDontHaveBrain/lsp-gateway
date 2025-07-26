package gateway

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"lsp-gateway/internal/config"
)

// LoadBalancerMetrics tracks load balancing performance
type LoadBalancerMetrics struct {
	TotalRequests       int64            `json:"totalRequests"`
	SuccessfulRequests  int64            `json:"successfulRequests"`
	FailedRequests      int64            `json:"failedRequests"`
	AverageResponseTime time.Duration    `json:"averageResponseTime"`
	ServerSelections    map[string]int64 `json:"serverSelections"`
	LastUpdated         time.Time        `json:"lastUpdated"`
	mu                  sync.RWMutex
}

// NewLoadBalancerMetrics creates new load balancer metrics
func NewLoadBalancerMetrics() *LoadBalancerMetrics {
	return &LoadBalancerMetrics{
		ServerSelections: make(map[string]int64),
		LastUpdated:      time.Now(),
	}
}

// RecordRequest records a load balancing request
func (lbm *LoadBalancerMetrics) RecordRequest(serverName string, responseTime time.Duration, success bool) {
	lbm.mu.Lock()
	defer lbm.mu.Unlock()

	atomic.AddInt64(&lbm.TotalRequests, 1)
	lbm.ServerSelections[serverName]++

	if success {
		atomic.AddInt64(&lbm.SuccessfulRequests, 1)
	} else {
		atomic.AddInt64(&lbm.FailedRequests, 1)
	}

	// Update average response time using exponential moving average
	if lbm.AverageResponseTime == 0 {
		lbm.AverageResponseTime = responseTime
	} else {
		alpha := 0.1 // Smoothing factor
		lbm.AverageResponseTime = time.Duration(float64(lbm.AverageResponseTime)*(1-alpha) + float64(responseTime)*alpha)
	}

	lbm.LastUpdated = time.Now()
}

// GetSuccessRate returns the current success rate
func (lbm *LoadBalancerMetrics) GetSuccessRate() float64 {
	total := atomic.LoadInt64(&lbm.TotalRequests)
	if total == 0 {
		return 1.0
	}
	successful := atomic.LoadInt64(&lbm.SuccessfulRequests)
	return float64(successful) / float64(total)
}

// GetRequestRate returns requests per minute
func (lbm *LoadBalancerMetrics) GetRequestRate() float64 {
	lbm.mu.RLock()
	defer lbm.mu.RUnlock()

	elapsed := time.Since(lbm.LastUpdated).Minutes()
	if elapsed == 0 {
		return 0
	}
	return float64(atomic.LoadInt64(&lbm.TotalRequests)) / elapsed
}

// Copy creates a copy of the metrics for safe reading
func (lbm *LoadBalancerMetrics) Copy() *LoadBalancerMetrics {
	lbm.mu.RLock()
	defer lbm.mu.RUnlock()

	serverSelections := make(map[string]int64)
	for k, v := range lbm.ServerSelections {
		serverSelections[k] = v
	}

	return &LoadBalancerMetrics{
		TotalRequests:       atomic.LoadInt64(&lbm.TotalRequests),
		SuccessfulRequests:  atomic.LoadInt64(&lbm.SuccessfulRequests),
		FailedRequests:      atomic.LoadInt64(&lbm.FailedRequests),
		AverageResponseTime: lbm.AverageResponseTime,
		ServerSelections:    serverSelections,
		LastUpdated:         lbm.LastUpdated,
	}
}

// PoolMetrics tracks metrics for a language server pool
type PoolMetrics struct {
	Language            string                    `json:"language"`
	ActiveServers       int                       `json:"activeServers"`
	TotalServers        int                       `json:"totalServers"`
	HealthyServers      int                       `json:"healthyServers"`
	LoadBalancerMetrics *LoadBalancerMetrics      `json:"loadBalancerMetrics"`
	ServerMetrics       map[string]*ServerMetrics `json:"serverMetrics"`
	SelectionStrategy   string                    `json:"selectionStrategy"`
	LastRebalanced      time.Time                 `json:"lastRebalanced"`
	mu                  sync.RWMutex
}

// NewPoolMetrics creates new pool metrics
func NewPoolMetrics() *PoolMetrics {
	return &PoolMetrics{
		LoadBalancerMetrics: NewLoadBalancerMetrics(),
		ServerMetrics:       make(map[string]*ServerMetrics),
		LastRebalanced:      time.Now(),
	}
}

// UpdatePoolStatus updates the pool status metrics
func (pm *PoolMetrics) UpdatePoolStatus(language string, total, active, healthy int, strategy string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.Language = language
	pm.TotalServers = total
	pm.ActiveServers = active
	pm.HealthyServers = healthy
	pm.SelectionStrategy = strategy
}

// RecordServerSelection records a server selection event
func (pm *PoolMetrics) RecordServerSelection(serverName string, responseTime time.Duration, success bool) {
	pm.LoadBalancerMetrics.RecordRequest(serverName, responseTime, success)
}

// AddServerMetrics adds metrics for a specific server
func (pm *PoolMetrics) AddServerMetrics(serverName string, metrics *ServerMetrics) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.ServerMetrics[serverName] = metrics
}

// RemoveServerMetrics removes metrics for a specific server
func (pm *PoolMetrics) RemoveServerMetrics(serverName string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.ServerMetrics, serverName)
}

// GetHealthScore returns the overall health score of the pool
func (pm *PoolMetrics) GetHealthScore() float64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.TotalServers == 0 {
		return 0.0
	}

	return float64(pm.HealthyServers) / float64(pm.TotalServers)
}

// Copy creates a copy of the pool metrics
func (pm *PoolMetrics) Copy() *PoolMetrics {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	serverMetrics := make(map[string]*ServerMetrics)
	for k, v := range pm.ServerMetrics {
		serverMetrics[k] = v.Copy()
	}

	return &PoolMetrics{
		Language:            pm.Language,
		ActiveServers:       pm.ActiveServers,
		TotalServers:        pm.TotalServers,
		HealthyServers:      pm.HealthyServers,
		LoadBalancerMetrics: pm.LoadBalancerMetrics.Copy(),
		ServerMetrics:       serverMetrics,
		SelectionStrategy:   pm.SelectionStrategy,
		LastRebalanced:      pm.LastRebalanced,
	}
}

// ManagerMetrics tracks metrics for the entire multi-server manager
type ManagerMetrics struct {
	TotalPools          int                     `json:"totalPools"`
	ActivePools         int                     `json:"activePools"`
	TotalServers        int                     `json:"totalServers"`
	HealthyServers      int                     `json:"healthyServers"`
	TotalRequests       int64                   `json:"totalRequests"`
	SuccessfulRequests  int64                   `json:"successfulRequests"`
	FailedRequests      int64                   `json:"failedRequests"`
	AverageResponseTime time.Duration           `json:"averageResponseTime"`
	PoolMetrics         map[string]*PoolMetrics `json:"poolMetrics"`
	LastUpdated         time.Time               `json:"lastUpdated"`
	mu                  sync.RWMutex
}

// NewManagerMetrics creates new manager metrics
func NewManagerMetrics() *ManagerMetrics {
	return &ManagerMetrics{
		PoolMetrics: make(map[string]*PoolMetrics),
		LastUpdated: time.Now(),
	}
}

// UpdatePoolMetrics updates metrics for a specific pool
func (mm *ManagerMetrics) UpdatePoolMetrics(language string, poolMetrics *PoolMetrics) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	mm.PoolMetrics[language] = poolMetrics
	mm.recalculateAggregateMetrics()
	mm.LastUpdated = time.Now()
}

// RecordRequest records a request at the manager level
func (mm *ManagerMetrics) RecordRequest(responseTime time.Duration, success bool) {
	atomic.AddInt64(&mm.TotalRequests, 1)

	if success {
		atomic.AddInt64(&mm.SuccessfulRequests, 1)
	} else {
		atomic.AddInt64(&mm.FailedRequests, 1)
	}

	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Update average response time using exponential moving average
	if mm.AverageResponseTime == 0 {
		mm.AverageResponseTime = responseTime
	} else {
		alpha := 0.1
		mm.AverageResponseTime = time.Duration(float64(mm.AverageResponseTime)*(1-alpha) + float64(responseTime)*alpha)
	}

	mm.LastUpdated = time.Now()
}

// recalculateAggregateMetrics recalculates aggregate metrics from pools (must be called with lock held)
func (mm *ManagerMetrics) recalculateAggregateMetrics() {
	mm.TotalPools = len(mm.PoolMetrics)
	mm.ActivePools = 0
	mm.TotalServers = 0
	mm.HealthyServers = 0

	for _, poolMetrics := range mm.PoolMetrics {
		if poolMetrics.ActiveServers > 0 {
			mm.ActivePools++
		}
		mm.TotalServers += poolMetrics.TotalServers
		mm.HealthyServers += poolMetrics.HealthyServers
	}
}

// GetSuccessRate returns the overall success rate
func (mm *ManagerMetrics) GetSuccessRate() float64 {
	total := atomic.LoadInt64(&mm.TotalRequests)
	if total == 0 {
		return 1.0
	}
	successful := atomic.LoadInt64(&mm.SuccessfulRequests)
	return float64(successful) / float64(total)
}

// GetOverallHealthScore returns the overall health score
func (mm *ManagerMetrics) GetOverallHealthScore() float64 {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	if mm.TotalServers == 0 {
		return 0.0
	}

	return float64(mm.HealthyServers) / float64(mm.TotalServers)
}

// Copy creates a copy of the manager metrics
func (mm *ManagerMetrics) Copy() *ManagerMetrics {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	poolMetrics := make(map[string]*PoolMetrics)
	for k, v := range mm.PoolMetrics {
		poolMetrics[k] = v.Copy()
	}

	return &ManagerMetrics{
		TotalPools:          mm.TotalPools,
		ActivePools:         mm.ActivePools,
		TotalServers:        mm.TotalServers,
		HealthyServers:      mm.HealthyServers,
		TotalRequests:       atomic.LoadInt64(&mm.TotalRequests),
		SuccessfulRequests:  atomic.LoadInt64(&mm.SuccessfulRequests),
		FailedRequests:      atomic.LoadInt64(&mm.FailedRequests),
		AverageResponseTime: mm.AverageResponseTime,
		PoolMetrics:         poolMetrics,
		LastUpdated:         mm.LastUpdated,
	}
}

// LoadBalancer provides load balancing functionality with pluggable strategies
type LoadBalancer struct {
	strategy string
	selector ServerSelector
	metrics  *LoadBalancerMetrics
	logger   *log.Logger
	mu       sync.RWMutex
}

// NewLoadBalancer creates a new load balancer with the specified strategy
func NewLoadBalancer(strategy string) *LoadBalancer {
	lbConfig := &config.LoadBalancingConfig{
		Strategy:        strategy,
		HealthThreshold: 0.8,
		WeightFactors:   make(map[string]float64),
	}

	selector, err := NewServerSelector(lbConfig)
	if err != nil {
		// Fallback to round robin if strategy creation fails
		selector = NewRoundRobinSelector()
		strategy = string(RoutingStrategyRoundRobin)
	}

	return &LoadBalancer{
		strategy: strategy,
		selector: selector,
		metrics:  NewLoadBalancerMetrics(),
	}
}

// NewLoadBalancerWithConfig creates a load balancer with specific configuration
func NewLoadBalancerWithConfig(loadBalancingConfig *config.LoadBalancingConfig, logger *log.Logger) *LoadBalancer {
	if loadBalancingConfig == nil {
		loadBalancingConfig = &config.LoadBalancingConfig{
			Strategy:        "round_robin",
			HealthThreshold: 0.8,
			WeightFactors:   make(map[string]float64),
		}
	}

	selector, err := NewServerSelector(loadBalancingConfig)
	if err != nil {
		if logger != nil {
			logger.Printf("Failed to create selector for strategy %s: %v, falling back to round robin", loadBalancingConfig.Strategy, err)
		}
		selector = NewRoundRobinSelector()
		loadBalancingConfig.Strategy = "round_robin"
	}

	return &LoadBalancer{
		strategy: loadBalancingConfig.Strategy,
		selector: selector,
		metrics:  NewLoadBalancerMetrics(),
		logger:   logger,
	}
}

// SelectServer selects the best server for a request
func (lb *LoadBalancer) SelectServer(pool *LanguageServerPool, requestType string, context *ServerSelectionContext) (*ServerInstance, error) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	startTime := time.Now()
	server, err := lb.selector.SelectServer(pool, requestType, context)
	selectionTime := time.Since(startTime)

	if err != nil {
		lb.metrics.RecordRequest("", selectionTime, false)
		return nil, fmt.Errorf("server selection failed: %w", err)
	}

	lb.metrics.RecordRequest(server.config.Name, selectionTime, true)

	if lb.logger != nil {
		lb.logger.Printf("LoadBalancer selected server %s for %s request (strategy: %s, time: %v)",
			server.config.Name, requestType, lb.strategy, selectionTime)
	}

	return server, nil
}

// SelectMultipleServers selects multiple servers for concurrent processing
func (lb *LoadBalancer) SelectMultipleServers(pool *LanguageServerPool, requestType string, maxServers int) ([]*ServerInstance, error) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	startTime := time.Now()
	servers, err := lb.selector.SelectMultipleServers(pool, requestType, maxServers)
	selectionTime := time.Since(startTime)

	if err != nil {
		lb.metrics.RecordRequest("", selectionTime, false)
		return nil, fmt.Errorf("multiple server selection failed: %w", err)
	}

	// Record selection for each server
	for _, server := range servers {
		lb.metrics.RecordRequest(server.config.Name, selectionTime, true)
	}

	if lb.logger != nil {
		serverNames := make([]string, len(servers))
		for i, server := range servers {
			serverNames[i] = server.config.Name
		}
		lb.logger.Printf("LoadBalancer selected %d servers %v for %s request (strategy: %s, time: %v)",
			len(servers), serverNames, requestType, lb.strategy, selectionTime)
	}

	return servers, nil
}

// UpdateServerMetrics updates metrics for a specific server
func (lb *LoadBalancer) UpdateServerMetrics(serverName string, responseTime time.Duration, success bool) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	lb.selector.UpdateServerMetrics(serverName, responseTime, success)
	lb.metrics.RecordRequest(serverName, responseTime, success)
}

// RebalancePool triggers rebalancing of the server pool
func (lb *LoadBalancer) RebalancePool(pool *LanguageServerPool) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if err := lb.selector.RebalancePool(pool); err != nil {
		if lb.logger != nil {
			lb.logger.Printf("Failed to rebalance pool for language %s: %v", pool.language, err)
		}
		return err
	}

	if lb.logger != nil {
		lb.logger.Printf("Successfully rebalanced pool for language %s using strategy %s", pool.language, lb.strategy)
	}

	return nil
}

// GetStrategy returns the current load balancing strategy
func (lb *LoadBalancer) GetStrategy() string {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	return lb.strategy
}

// SetStrategy changes the load balancing strategy
func (lb *LoadBalancer) SetStrategy(strategy string, cfg *config.LoadBalancingConfig) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if cfg == nil {
		cfg = &config.LoadBalancingConfig{
			Strategy:        strategy,
			HealthThreshold: 0.8,
			WeightFactors:   make(map[string]float64),
		}
	} else {
		cfg.Strategy = strategy
	}

	selector, err := NewServerSelector(cfg)
	if err != nil {
		return fmt.Errorf("failed to create selector for strategy %s: %w", strategy, err)
	}

	lb.strategy = strategy
	lb.selector = selector

	if lb.logger != nil {
		lb.logger.Printf("LoadBalancer strategy changed to %s", strategy)
	}

	return nil
}

// GetMetrics returns the current load balancer metrics
func (lb *LoadBalancer) GetMetrics() *LoadBalancerMetrics {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	return lb.metrics.Copy()
}

// GetSelector returns the current server selector (for testing/debugging)
func (lb *LoadBalancer) GetSelector() ServerSelector {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	return lb.selector
}

// Reset resets the load balancer metrics and selector state
func (lb *LoadBalancer) Reset() {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.metrics = NewLoadBalancerMetrics()

	// Recreate selector to reset its internal state
	lbConfig := &config.LoadBalancingConfig{
		Strategy:        lb.strategy,
		HealthThreshold: 0.8,
		WeightFactors:   make(map[string]float64),
	}

	if selector, err := NewServerSelector(lbConfig); err == nil {
		lb.selector = selector
	}

	if lb.logger != nil {
		lb.logger.Printf("LoadBalancer reset (strategy: %s)", lb.strategy)
	}
}

// ValidateStrategy checks if a strategy is supported
func ValidateStrategy(strategy string) error {
	supportedStrategies := []string{
		"round_robin",
		"least_connections",
		"response_time",
		"performance",
		"resource_usage",
		"feature",
	}

	for _, supported := range supportedStrategies {
		if strategy == supported {
			return nil
		}
	}

	return fmt.Errorf("unsupported load balancing strategy: %s, supported strategies: %v", strategy, supportedStrategies)
}

// GetSupportedStrategies returns a list of supported load balancing strategies
func GetSupportedStrategies() []string {
	return []string{
		"round_robin",
		"least_connections",
		"response_time",
		"performance",
		"resource_usage",
		"feature",
	}
}

// LoadBalancingStats provides statistics about load balancing performance
type LoadBalancingStats struct {
	Strategy            string                              `json:"strategy"`
	TotalRequests       int64                               `json:"totalRequests"`
	SuccessRate         float64                             `json:"successRate"`
	AverageResponseTime time.Duration                       `json:"averageResponseTime"`
	ServerDistribution  map[string]LoadBalancingServerStats `json:"serverDistribution"`
	LastUpdated         time.Time                           `json:"lastUpdated"`
}

// LoadBalancingServerStats provides per-server statistics
type LoadBalancingServerStats struct {
	RequestCount    int64         `json:"requestCount"`
	SuccessRate     float64       `json:"successRate"`
	AvgResponseTime time.Duration `json:"avgResponseTime"`
	LastUsed        time.Time     `json:"lastUsed"`
}

// GetLoadBalancingStats returns comprehensive load balancing statistics
func (lb *LoadBalancer) GetLoadBalancingStats() *LoadBalancingStats {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	metrics := lb.metrics.Copy()
	serverDistribution := make(map[string]LoadBalancingServerStats)

	for serverName, requestCount := range metrics.ServerSelections {
		serverDistribution[serverName] = LoadBalancingServerStats{
			RequestCount:    requestCount,
			SuccessRate:     metrics.GetSuccessRate(), // Individual server success rates would need more detailed tracking
			AvgResponseTime: metrics.AverageResponseTime,
			LastUsed:        metrics.LastUpdated, // This would need per-server tracking for accuracy
		}
	}

	return &LoadBalancingStats{
		Strategy:            lb.strategy,
		TotalRequests:       metrics.TotalRequests,
		SuccessRate:         metrics.GetSuccessRate(),
		AverageResponseTime: metrics.AverageResponseTime,
		ServerDistribution:  serverDistribution,
		LastUpdated:         metrics.LastUpdated,
	}
}
