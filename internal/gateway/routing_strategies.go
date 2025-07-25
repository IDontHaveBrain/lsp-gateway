package gateway

import (
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/transport"
	"lsp-gateway/mcp"
)

// RoutingStrategyInterface defines how requests should be routed to servers
type RoutingStrategyInterface interface {
	Route(request *LSPRequest, availableServers []*ServerInstance) ([]*RoutingDecision, error)
	GetName() string
	GetPriority() int
	SupportsAggregation() bool
	Configure(config *StrategyConfig) error
	GetMetrics() *StrategyMetrics
}

// StrategyConfig provides configuration for routing strategies
type StrategyConfig struct {
	Name              string                 `json:"name"`
	Priority          int                    `json:"priority"`
	Timeout           time.Duration          `json:"timeout"`
	MaxServers        int                    `json:"max_servers"`
	LoadBalance       LoadBalanceType        `json:"load_balance"`
	FallbackMode      FallbackMode           `json:"fallback_mode"`
	AggregationMode   AggregationMode        `json:"aggregation_mode"`
	HealthThreshold   float64                `json:"health_threshold"`
	RetryCount        int                    `json:"retry_count"`
	CircuitBreaker    bool                   `json:"circuit_breaker"`
	Parameters        map[string]interface{} `json:"parameters"`
}

// LoadBalanceType defines load balancing algorithms
type LoadBalanceType string

const (
	LoadBalanceRoundRobin    LoadBalanceType = "round_robin"
	LoadBalanceLeastConn     LoadBalanceType = "least_conn"
	LoadBalanceResponseTime  LoadBalanceType = "response_time"
	LoadBalanceHealthScore   LoadBalanceType = "health_score"
	LoadBalanceWeighted      LoadBalanceType = "weighted"
	LoadBalanceResourceUsage LoadBalanceType = "resource_usage"
)

// FallbackMode defines fallback behavior on failures
type FallbackMode string

const (
	FallbackSequential FallbackMode = "sequential"
	FallbackParallel   FallbackMode = "parallel"
	FallbackNone       FallbackMode = "none"
	FallbackAdaptive   FallbackMode = "adaptive"
)

// AggregationMode defines how responses are aggregated
type AggregationMode string

const (
	AggregationPrimary   AggregationMode = "primary"
	AggregationMerge     AggregationMode = "merge"
	AggregationUnion     AggregationMode = "union"
	AggregationBest      AggregationMode = "best"
	AggregationConsensus AggregationMode = "consensus"
)


// StrategyRegistry manages and provides access to routing strategies
type StrategyRegistry struct {
	strategies map[string]RoutingStrategyInterface
	config     map[string]*StrategyConfig
	metrics    map[string]*StrategyMetrics
	logger     *mcp.StructuredLogger
	mu         sync.RWMutex
}

// NewStrategyRegistry creates a new strategy registry
func NewStrategyRegistry(logger *mcp.StructuredLogger) *StrategyRegistry {
	registry := &StrategyRegistry{
		strategies: make(map[string]RoutingStrategyInterface),
		config:     make(map[string]*StrategyConfig),
		metrics:    make(map[string]*StrategyMetrics),
		logger:     logger,
	}

	// Register default strategies
	registry.registerDefaultStrategies()
	return registry
}

// registerDefaultStrategies registers all default routing strategies
func (sr *StrategyRegistry) registerDefaultStrategies() {
	strategies := []RoutingStrategyInterface{
		NewSingleTargetStrategy(sr.logger),
		NewMultiTargetStrategy(sr.logger),
		NewBroadcastAggregateStrategy(sr.logger),
		NewSequentialFallbackStrategy(sr.logger),
		NewLoadBalancedStrategy(sr.logger),
		NewCrossLanguageStrategy(sr.logger),
	}

	for _, strategy := range strategies {
		sr.RegisterStrategy(strategy)
	}
}

// RegisterStrategy registers a routing strategy
func (sr *StrategyRegistry) RegisterStrategy(strategy RoutingStrategyInterface) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	name := strategy.GetName()
	sr.strategies[name] = strategy
	
	// Initialize with default config if not exists
	if sr.config[name] == nil {
		sr.config[name] = sr.getDefaultConfig(name)
	}
	
	// Initialize metrics
	if sr.metrics[name] == nil {
		sr.metrics[name] = &StrategyMetrics{}
	}

	strategy.Configure(sr.config[name])

	if sr.logger != nil {
		sr.logger.Debugf("Registered routing strategy: %s", name)
	}
}

// GetStrategy retrieves a routing strategy by name
func (sr *StrategyRegistry) GetStrategy(name string) (RoutingStrategyInterface, bool) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	strategy, exists := sr.strategies[name]
	return strategy, exists
}

// RouteRequest routes a request using the specified strategy
func (sr *StrategyRegistry) RouteRequest(strategyName string, request *LSPRequest, availableServers []*ServerInstance) ([]*RoutingDecision, error) {
	strategy, exists := sr.GetStrategy(strategyName)
	if !exists {
		return nil, fmt.Errorf("strategy %s not found", strategyName)
	}

	startTime := time.Now()
	decisions, err := strategy.Route(request, availableServers)
	duration := time.Since(startTime)

	// Update strategy metrics
	sr.updateStrategyMetrics(strategyName, duration, err == nil)

	return decisions, err
}

// GetAllStrategies returns all registered strategies
func (sr *StrategyRegistry) GetAllStrategies() map[string]RoutingStrategyInterface {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	strategies := make(map[string]RoutingStrategyInterface)
	for name, strategy := range sr.strategies {
		strategies[name] = strategy
	}
	return strategies
}

// UpdateStrategyConfig updates configuration for a strategy
func (sr *StrategyRegistry) UpdateStrategyConfig(name string, config *StrategyConfig) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	strategy, exists := sr.strategies[name]
	if !exists {
		return fmt.Errorf("strategy %s not found", name)
	}

	sr.config[name] = config
	return strategy.Configure(config)
}

// getDefaultConfig returns default configuration for a strategy
func (sr *StrategyRegistry) getDefaultConfig(name string) *StrategyConfig {
	baseConfig := &StrategyConfig{
		Name:            name,
		Priority:        5,
		Timeout:         30 * time.Second,
		MaxServers:      5,
		LoadBalance:     LoadBalanceHealthScore,
		FallbackMode:    FallbackSequential,
		AggregationMode: AggregationPrimary,
		HealthThreshold: 0.7,
		RetryCount:      2,
		CircuitBreaker:  true,
		Parameters:      make(map[string]interface{}),
	}

	// Customize based on strategy type
	switch name {
	case "single_target":
		baseConfig.MaxServers = 1
		baseConfig.FallbackMode = FallbackSequential
	case "multi_target":
		baseConfig.MaxServers = 3
		baseConfig.AggregationMode = AggregationMerge
	case "broadcast_aggregate":
		baseConfig.MaxServers = 10
		baseConfig.AggregationMode = AggregationUnion
	case "sequential_fallback":
		baseConfig.MaxServers = 5
		baseConfig.FallbackMode = FallbackSequential
	case "load_balanced":
		baseConfig.LoadBalance = LoadBalanceRoundRobin
		baseConfig.MaxServers = 3
	case "cross_language":
		baseConfig.MaxServers = 7
		baseConfig.AggregationMode = AggregationBest
	}

	return baseConfig
}

// updateStrategyMetrics updates performance metrics for a strategy
func (sr *StrategyRegistry) updateStrategyMetrics(name string, duration time.Duration, success bool) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if sr.metrics[name] == nil {
		sr.metrics[name] = &StrategyMetrics{}
	}

	metrics := sr.metrics[name]
	metrics.RequestCount++

	if success {
		metrics.SuccessCount++
	}

	// Update average response time
	if metrics.RequestCount > 0 {
		metrics.AverageResponseTime = time.Duration(
			(int64(metrics.AverageResponseTime)*(metrics.RequestCount-1) + int64(duration)) / metrics.RequestCount,
		)
	}

	// Calculate success rate
	metrics.SuccessRate = float64(metrics.SuccessCount) / float64(metrics.RequestCount)
}

// SingleTargetStrategy routes to the best single server
type SingleTargetStrategy struct {
	name    string
	config  *StrategyConfig
	logger  *mcp.StructuredLogger
	metrics *StrategyMetrics
	mu      sync.RWMutex
}

// NewSingleTargetStrategy creates a new single target strategy
func NewSingleTargetStrategy(logger *mcp.StructuredLogger) *SingleTargetStrategy {
	return &SingleTargetStrategy{
		name:    "single_target",
		logger:  logger,
		metrics: &StrategyMetrics{},
	}
}

func (s *SingleTargetStrategy) GetName() string {
	return s.name
}

func (s *SingleTargetStrategy) GetPriority() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.config != nil {
		return s.config.Priority
	}
	return 5
}

func (s *SingleTargetStrategy) SupportsAggregation() bool {
	return false
}

func (s *SingleTargetStrategy) Configure(config *StrategyConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = config
	return nil
}

func (s *SingleTargetStrategy) GetMetrics() *StrategyMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.metrics
}

func (s *SingleTargetStrategy) Route(request *LSPRequest, availableServers []*ServerInstance) ([]*RoutingDecision, error) {
	if len(availableServers) == 0 {
		return nil, fmt.Errorf("no servers available for single target routing")
	}

	// Filter healthy servers
	healthyServers := s.filterHealthyServers(availableServers)
	if len(healthyServers) == 0 {
		return nil, fmt.Errorf("no healthy servers available")
	}

	// Select best server based on health score and performance
	bestServer := s.selectBestServer(healthyServers)

	decision := &RoutingDecision{
		TargetServers:   []*ServerInstance{bestServer},
		RoutingStrategy: s.name,
		Priority:        bestServer.config.Priority,
		CreatedAt:      time.Now(),
	}

	return []*RoutingDecision{decision}, nil
}

func (s *SingleTargetStrategy) filterHealthyServers(servers []*ServerInstance) []*ServerInstance {
	var healthy []*ServerInstance
	healthThreshold := 0.7
	if s.config != nil {
		healthThreshold = s.config.HealthThreshold
	}

	for _, server := range servers {
		if server.IsHealthy() && server.circuitBreaker.CanExecute() && s.getHealthScore(server) >= healthThreshold {
			healthy = append(healthy, server)
		}
	}
	return healthy
}

func (s *SingleTargetStrategy) selectBestServer(servers []*ServerInstance) *ServerInstance {
	if len(servers) == 0 {
		return nil
	}

	// Sort by composite score: health score + load score + weight
	sort.Slice(servers, func(i, j int) bool {
		scoreI := s.calculateCompositeScore(servers[i])
		scoreJ := s.calculateCompositeScore(servers[j])
		return scoreI > scoreJ
	})

	return servers[0]
}

func (s *SingleTargetStrategy) calculateCompositeScore(server *ServerInstance) float64 {
	// Composite score considering health, load, and weight
	healthWeight := 0.4
	loadWeight := 0.3
	configWeight := 0.3

	healthScore := s.getHealthScore(server)
	loadScore := 1.0 - s.getLoadScore(server) // Invert load score (lower load = higher score)
	weight := s.getServerWeight(server)
	return (healthScore * healthWeight) + (loadScore * loadWeight) + (weight * configWeight)
}

// MultiTargetStrategy routes to multiple servers in parallel
type MultiTargetStrategy struct {
	name    string
	config  *StrategyConfig
	logger  *mcp.StructuredLogger
	metrics *StrategyMetrics
	mu      sync.RWMutex
}

// NewMultiTargetStrategy creates a new multi target strategy
func NewMultiTargetStrategy(logger *mcp.StructuredLogger) *MultiTargetStrategy {
	return &MultiTargetStrategy{
		name:    "multi_target",
		logger:  logger,
		metrics: &StrategyMetrics{},
	}
}

func (m *MultiTargetStrategy) GetName() string {
	return m.name
}

func (m *MultiTargetStrategy) GetPriority() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.config != nil {
		return m.config.Priority
	}
	return 6
}

func (m *MultiTargetStrategy) SupportsAggregation() bool {
	return true
}

func (m *MultiTargetStrategy) Configure(config *StrategyConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
	return nil
}

func (m *MultiTargetStrategy) GetMetrics() *StrategyMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.metrics
}

func (m *MultiTargetStrategy) Route(request *LSPRequest, availableServers []*ServerInstance) ([]*RoutingDecision, error) {
	if len(availableServers) == 0 {
		return nil, fmt.Errorf("no servers available for multi target routing")
	}

	// Filter healthy servers
	healthyServers := m.filterHealthyServers(availableServers)
	if len(healthyServers) == 0 {
		return nil, fmt.Errorf("no healthy servers available")
	}

	// Determine optimal server count
	maxServers := 3
	if m.config != nil && m.config.MaxServers > 0 {
		maxServers = m.config.MaxServers
	}

	targetCount := min(len(healthyServers), maxServers)
	selectedServers := m.selectOptimalServers(healthyServers, targetCount)

	var decisions []*RoutingDecision
	for _, server := range selectedServers {
		decision := &RoutingDecision{
			TargetServers:   []*ServerInstance{server},
			RoutingStrategy: m.name,
			Priority:        server.config.Priority,
			CreatedAt:       time.Now(),
		}
		decisions = append(decisions, decision)
	}

	return decisions, nil
}

func (m *MultiTargetStrategy) filterHealthyServers(servers []*ServerInstance) []*ServerInstance {
	var healthy []*ServerInstance
	healthThreshold := 0.6 // Slightly lower threshold for multi-target
	if m.config != nil {
		healthThreshold = m.config.HealthThreshold
	}

	for _, server := range servers {
		if server.IsHealthy() && server.circuitBreaker.CanExecute() && m.getHealthScore(server) >= healthThreshold {
			healthy = append(healthy, server)
		}
	}
	return healthy
}

func (m *MultiTargetStrategy) selectOptimalServers(servers []*ServerInstance, count int) []*ServerInstance {
	if len(servers) <= count {
		return servers
	}

	// Sort by composite score but maintain diversity
	sort.Slice(servers, func(i, j int) bool {
		scoreI := m.calculateDiversityScore(servers[i])
		scoreJ := m.calculateDiversityScore(servers[j])
		return scoreI > scoreJ
	})

	// Select top servers with load balancing consideration
	selected := make([]*ServerInstance, 0, count)
	for i := 0; i < count && i < len(servers); i++ {
		selected = append(selected, servers[i])
	}

	return selected
}

func (m *MultiTargetStrategy) calculateDiversityScore(server *ServerInstance) float64 {
	// Score that balances performance and diversity
	healthScore := m.getHealthScore(server)
	loadScore := m.getLoadScore(server)
	weight := m.getServerWeight(server)
	baseScore := (healthScore * 0.5) + ((1.0 - loadScore) * 0.3) + (weight * 0.2)
	
	// Add randomization for diversity
	diversityFactor := 0.9 + (rand.Float64() * 0.2) // 0.9 to 1.1
	return baseScore * diversityFactor
}

// BroadcastAggregateStrategy broadcasts to all relevant servers
type BroadcastAggregateStrategy struct {
	name    string
	config  *StrategyConfig
	logger  *mcp.StructuredLogger
	metrics *StrategyMetrics
	mu      sync.RWMutex
}

// NewBroadcastAggregateStrategy creates a new broadcast aggregate strategy
func NewBroadcastAggregateStrategy(logger *mcp.StructuredLogger) *BroadcastAggregateStrategy {
	return &BroadcastAggregateStrategy{
		name:    "broadcast_aggregate",
		logger:  logger,
		metrics: &StrategyMetrics{},
	}
}

func (b *BroadcastAggregateStrategy) GetName() string {
	return b.name
}

func (b *BroadcastAggregateStrategy) GetPriority() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.config != nil {
		return b.config.Priority
	}
	return 8
}

func (b *BroadcastAggregateStrategy) SupportsAggregation() bool {
	return true
}

func (b *BroadcastAggregateStrategy) Configure(config *StrategyConfig) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.config = config
	return nil
}

func (b *BroadcastAggregateStrategy) GetMetrics() *StrategyMetrics {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.metrics
}

func (b *BroadcastAggregateStrategy) Route(request *LSPRequest, availableServers []*ServerInstance) ([]*RoutingDecision, error) {
	if len(availableServers) == 0 {
		return nil, fmt.Errorf("no servers available for broadcast routing")
	}

	// Filter servers for broadcasting (less strict health requirements)
	broadcastServers := b.filterBroadcastServers(availableServers)
	if len(broadcastServers) == 0 {
		return nil, fmt.Errorf("no suitable servers for broadcasting")
	}

	// Apply max server limit
	maxServers := 10
	if b.config != nil && b.config.MaxServers > 0 {
		maxServers = b.config.MaxServers
	}

	if len(broadcastServers) > maxServers {
		broadcastServers = b.selectBroadcastServers(broadcastServers, maxServers)
	}

	var decisions []*RoutingDecision
	for _, server := range broadcastServers {
		decision := &RoutingDecision{
			TargetServers:   []*ServerInstance{server},
			RoutingStrategy: b.name,
			Priority:        server.config.Priority,
			CreatedAt:       time.Now(),
		}
		decisions = append(decisions, decision)
	}

	return decisions, nil
}

func (b *BroadcastAggregateStrategy) filterBroadcastServers(servers []*ServerInstance) []*ServerInstance {
	var suitable []*ServerInstance
	healthThreshold := 0.5 // Lower threshold for broadcast
	if b.config != nil {
		healthThreshold = b.config.HealthThreshold * 0.8 // 80% of configured threshold
	}

	for _, server := range servers {
		if server.IsHealthy() && b.getHealthScore(server) >= healthThreshold {
			// Include servers even if circuit is open but health is good
			suitable = append(suitable, server)
		}
	}
	return suitable
}

func (b *BroadcastAggregateStrategy) selectBroadcastServers(servers []*ServerInstance, maxCount int) []*ServerInstance {
	// Sort by broadcast suitability
	sort.Slice(servers, func(i, j int) bool {
		scoreI := b.calculateBroadcastScore(servers[i])
		scoreJ := b.calculateBroadcastScore(servers[j])
		return scoreI > scoreJ
	})

	if len(servers) > maxCount {
		return servers[:maxCount]
	}
	return servers
}

func (b *BroadcastAggregateStrategy) calculateBroadcastScore(server *ServerInstance) float64 {
	// Score emphasizing stability and feature coverage
	healthScore := b.getHealthScore(server)
	loadScore := b.getLoadScore(server)
	weight := b.getServerWeight(server)
	stabilityScore := healthScore * 0.6
	performanceScore := (1.0 - loadScore) * 0.2
	featureScore := weight * 0.2
	
	return stabilityScore + performanceScore + featureScore
}

func (b *BroadcastAggregateStrategy) getAggregationMode() string {
	if b.config != nil {
		return string(b.config.AggregationMode)
	}
	return string(AggregationUnion)
}

func (b *BroadcastAggregateStrategy) isCrossLanguageCapable(server *ServerInstance) bool {
	return len(server.config.Languages) > 1
}

// SequentialFallbackStrategy tries servers in priority order
type SequentialFallbackStrategy struct {
	name    string
	config  *StrategyConfig
	logger  *mcp.StructuredLogger
	metrics *StrategyMetrics
	mu      sync.RWMutex
}

// NewSequentialFallbackStrategy creates a new sequential fallback strategy
func NewSequentialFallbackStrategy(logger *mcp.StructuredLogger) *SequentialFallbackStrategy {
	return &SequentialFallbackStrategy{
		name:    "sequential_fallback",
		logger:  logger,
		metrics: &StrategyMetrics{},
	}
}

func (s *SequentialFallbackStrategy) GetName() string {
	return s.name
}

func (s *SequentialFallbackStrategy) GetPriority() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.config != nil {
		return s.config.Priority
	}
	return 7
}

func (s *SequentialFallbackStrategy) SupportsAggregation() bool {
	return false
}

func (s *SequentialFallbackStrategy) Configure(config *StrategyConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = config
	return nil
}

func (s *SequentialFallbackStrategy) GetMetrics() *StrategyMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.metrics
}

func (s *SequentialFallbackStrategy) Route(request *LSPRequest, availableServers []*ServerInstance) ([]*RoutingDecision, error) {
	if len(availableServers) == 0 {
		return nil, fmt.Errorf("no servers available for sequential fallback routing")
	}

	// Sort servers by fallback priority
	orderedServers := s.orderServersByPriority(availableServers)
	
	// Find the first healthy server that can handle the request
	for _, server := range orderedServers {
		if s.isServerSuitable(server) {
			decision := &RoutingDecision{
				TargetServers:   []*ServerInstance{server},
				RoutingStrategy: s.name,
				Priority:        server.config.Priority,
				CreatedAt:       time.Now(),
			}
			return []*RoutingDecision{decision}, nil
		}
	}

	return nil, fmt.Errorf("no suitable servers found in fallback chain")
}

func (s *SequentialFallbackStrategy) orderServersByPriority(servers []*ServerInstance) []*ServerInstance {
	// Create a copy to avoid modifying original slice
	ordered := make([]*ServerInstance, len(servers))
	copy(ordered, servers)

	// Sort by priority (higher first), then by health score, then by load
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].config.Priority != ordered[j].config.Priority {
			return ordered[i].config.Priority > ordered[j].config.Priority
		}
		healthScoreI := s.getHealthScore(ordered[i])
		healthScoreJ := s.getHealthScore(ordered[j])
		if healthScoreI != healthScoreJ {
			return healthScoreI > healthScoreJ
		}
		loadScoreI := s.getLoadScore(ordered[i])
		loadScoreJ := s.getLoadScore(ordered[j])
		return loadScoreI < loadScoreJ // Lower load is better
	})

	return ordered
}

func (s *SequentialFallbackStrategy) isServerSuitable(server *ServerInstance) bool {
	healthThreshold := 0.6
	if s.config != nil {
		healthThreshold = s.config.HealthThreshold
	}

	return server.IsHealthy() && 
		   server.circuitBreaker.CanExecute() && 
		   s.getHealthScore(server) >= healthThreshold
}

// LoadBalancedStrategy distributes load across servers
type LoadBalancedStrategy struct {
	name            string
	config          *StrategyConfig
	logger          *mcp.StructuredLogger
	metrics         *StrategyMetrics
	roundRobinIndex int64
	mu              sync.RWMutex
}

// NewLoadBalancedStrategy creates a new load balanced strategy
func NewLoadBalancedStrategy(logger *mcp.StructuredLogger) *LoadBalancedStrategy {
	return &LoadBalancedStrategy{
		name:    "load_balanced",
		logger:  logger,
		metrics: &StrategyMetrics{},
	}
}

func (l *LoadBalancedStrategy) GetName() string {
	return l.name
}

func (l *LoadBalancedStrategy) GetPriority() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.config != nil {
		return l.config.Priority
	}
	return 6
}

func (l *LoadBalancedStrategy) SupportsAggregation() bool {
	return false
}

func (l *LoadBalancedStrategy) Configure(config *StrategyConfig) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.config = config
	return nil
}

func (l *LoadBalancedStrategy) GetMetrics() *StrategyMetrics {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.metrics
}

func (l *LoadBalancedStrategy) Route(request *LSPRequest, availableServers []*ServerInstance) ([]*RoutingDecision, error) {
	if len(availableServers) == 0 {
		return nil, fmt.Errorf("no servers available for load balanced routing")
	}

	// Filter healthy servers
	healthyServers := l.filterHealthyServers(availableServers)
	if len(healthyServers) == 0 {
		return nil, fmt.Errorf("no healthy servers available for load balancing")
	}

	// Select server based on load balancing algorithm
	selectedServer := l.selectServerByLoadBalancing(healthyServers)
	if selectedServer == nil {
		return nil, fmt.Errorf("failed to select server using load balancing")
	}

	decision := &RoutingDecision{
		TargetServers:   []*ServerInstance{selectedServer},
		RoutingStrategy: l.name,
		Priority:        selectedServer.config.Priority,
		CreatedAt:       time.Now(),
	}

	return []*RoutingDecision{decision}, nil
}

func (l *LoadBalancedStrategy) filterHealthyServers(servers []*ServerInstance) []*ServerInstance {
	var healthy []*ServerInstance
	healthThreshold := 0.6
	if l.config != nil {
		healthThreshold = l.config.HealthThreshold
	}

	for _, server := range servers {
		if server.IsHealthy() && server.circuitBreaker.CanExecute() && l.getHealthScore(server) >= healthThreshold {
			healthy = append(healthy, server)
		}
	}
	return healthy
}

func (l *LoadBalancedStrategy) selectServerByLoadBalancing(servers []*ServerInstance) *ServerInstance {
	method := LoadBalanceRoundRobin
	if l.config != nil {
		method = l.config.LoadBalance
	}

	switch method {
	case LoadBalanceRoundRobin:
		return l.selectRoundRobin(servers)
	case LoadBalanceLeastConn:
		return l.selectLeastConnections(servers)
	case LoadBalanceResponseTime:
		return l.selectByResponseTime(servers)
	case LoadBalanceHealthScore:
		return l.selectByHealthScore(servers)
	case LoadBalanceWeighted:
		return l.selectWeighted(servers)
	case LoadBalanceResourceUsage:
		return l.selectByResourceUsage(servers)
	default:
		return l.selectRoundRobin(servers)
	}
}

func (l *LoadBalancedStrategy) selectRoundRobin(servers []*ServerInstance) *ServerInstance {
	if len(servers) == 0 {
		return nil
	}
	
	index := atomic.AddInt64(&l.roundRobinIndex, 1) - 1
	return servers[index%int64(len(servers))]
}

func (l *LoadBalancedStrategy) selectLeastConnections(servers []*ServerInstance) *ServerInstance {
	if len(servers) == 0 {
		return nil
	}

	var bestServer *ServerInstance
	var minConnections int64 = ^int64(0) // Max int64

	for _, server := range servers {
		if server.metrics != nil {
			activeConns := server.connectionCount
			if int64(activeConns) < minConnections {
				minConnections = int64(activeConns)
				bestServer = server
			}
		} else {
			// No metrics means no connections
			return server
		}
	}

	if bestServer == nil {
		return servers[0]
	}
	return bestServer
}

func (l *LoadBalancedStrategy) selectByResponseTime(servers []*ServerInstance) *ServerInstance {
	if len(servers) == 0 {
		return nil
	}

	var bestServer *ServerInstance
	var bestTime time.Duration = time.Duration(^uint64(0) >> 1) // Max duration

	for _, server := range servers {
		if server.metrics != nil {
			avgTime := server.metrics.GetAverageResponseTime()
			if avgTime < bestTime {
				bestTime = avgTime
				bestServer = server
			}
		} else {
			// No metrics means potentially fastest
			return server
		}
	}

	if bestServer == nil {
		return servers[0]
	}
	return bestServer
}

func (l *LoadBalancedStrategy) selectByHealthScore(servers []*ServerInstance) *ServerInstance {
	if len(servers) == 0 {
		return nil
	}

	sort.Slice(servers, func(i, j int) bool {
		return l.getHealthScore(servers[i]) > l.getHealthScore(servers[j])
	})

	return servers[0]
}

func (l *LoadBalancedStrategy) selectWeighted(servers []*ServerInstance) *ServerInstance {
	if len(servers) == 0 {
		return nil
	}

	// Calculate total weight
	totalWeight := 0.0
	for _, server := range servers {
		totalWeight += l.getServerWeight(server)
	}

	if totalWeight == 0 {
		return l.selectRoundRobin(servers)
	}

	// Generate random value
	random := rand.Float64() * totalWeight
	currentWeight := 0.0

	for _, server := range servers {
		currentWeight += l.getServerWeight(server)
		if random <= currentWeight {
			return server
		}
	}

	return servers[len(servers)-1]
}

func (l *LoadBalancedStrategy) selectByResourceUsage(servers []*ServerInstance) *ServerInstance {
	if len(servers) == 0 {
		return nil
	}

	// Select server with lowest load score
	sort.Slice(servers, func(i, j int) bool {
		return l.getLoadScore(servers[i]) < l.getLoadScore(servers[j])
	})

	return servers[0]
}

func (l *LoadBalancedStrategy) getLoadBalanceMethod() string {
	if l.config != nil {
		return string(l.config.LoadBalance)
	}
	return string(LoadBalanceRoundRobin)
}

// CrossLanguageStrategy specialized for cross-language scenarios
type CrossLanguageStrategy struct {
	name    string
	config  *StrategyConfig
	logger  *mcp.StructuredLogger
	metrics *StrategyMetrics
	mu      sync.RWMutex
}

// NewCrossLanguageStrategy creates a new cross language strategy
func NewCrossLanguageStrategy(logger *mcp.StructuredLogger) *CrossLanguageStrategy {
	return &CrossLanguageStrategy{
		name:    "cross_language",
		logger:  logger,
		metrics: &StrategyMetrics{},
	}
}

func (c *CrossLanguageStrategy) GetName() string {
	return c.name
}

func (c *CrossLanguageStrategy) GetPriority() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.config != nil {
		return c.config.Priority
	}
	return 9
}

func (c *CrossLanguageStrategy) SupportsAggregation() bool {
	return true
}

func (c *CrossLanguageStrategy) Configure(config *StrategyConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = config
	return nil
}

func (c *CrossLanguageStrategy) GetMetrics() *StrategyMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.metrics
}

func (c *CrossLanguageStrategy) Route(request *LSPRequest, availableServers []*ServerInstance) ([]*RoutingDecision, error) {
	if len(availableServers) == 0 {
		return nil, fmt.Errorf("no servers available for cross-language routing")
	}

	// Detect languages involved in the request
	languages := c.detectRequestLanguages(request)
	
	// Group servers by language capability
	languageGroups := c.groupServersByLanguage(availableServers, languages)
	
	// Select optimal servers for cross-language coordination
	selectedServers := c.selectCrossLanguageServers(languageGroups, languages)
	
	if len(selectedServers) == 0 {
		return nil, fmt.Errorf("no suitable servers for cross-language request")
	}

	var decisions []*RoutingDecision
	for _, server := range selectedServers {
		decision := &RoutingDecision{
			TargetServers:   []*ServerInstance{server},
			RoutingStrategy: c.name,
			Priority:        server.config.Priority,
			CreatedAt:       time.Now(),
		}
		decisions = append(decisions, decision)
	}

	return decisions, nil
}

func (c *CrossLanguageStrategy) detectRequestLanguages(request *LSPRequest) []string {
	var languages []string
	
	// Primary language from request context
	if request.Context != nil && request.Context.Language != "" {
		languages = append(languages, request.Context.Language)
	}
	
	// Detect from URI if available
	if request.URI != "" {
		if lang := c.detectLanguageFromURI(request.URI); lang != "" && !c.contains(languages, lang) {
			languages = append(languages, lang)
		}
	}
	
	// Detect embedded languages (e.g., JavaScript in HTML, CSS in Vue)
	embeddedLangs := c.detectEmbeddedLanguages(request)
	for _, lang := range embeddedLangs {
		if !c.contains(languages, lang) {
			languages = append(languages, lang)
		}
	}
	
	return languages
}

func (c *CrossLanguageStrategy) detectLanguageFromURI(uri string) string {
	if !strings.HasPrefix(uri, "file://") {
		return ""
	}
	
	// Extract file extension and map to language
	ext := strings.ToLower(filepath.Ext(uri))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".java":
		return "java"
	case ".vue":
		return "vue"
	case ".html":
		return "html"
	case ".css":
		return "css"
	case ".rs":
		return "rust"
	case ".c":
		return "c"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	}
	return ""
}

func (c *CrossLanguageStrategy) detectEmbeddedLanguages(request *LSPRequest) []string {
	var embedded []string
	
	// Detect based on file patterns and content
	if request.URI != "" {
		ext := strings.ToLower(filepath.Ext(request.URI))
		switch ext {
		case ".vue":
			embedded = append(embedded, "javascript", "css", "html")
		case ".html":
			embedded = append(embedded, "javascript", "css")
		case ".jsx", ".tsx":
			embedded = append(embedded, "html") // JSX
		case ".md":
			embedded = append(embedded, "markdown")
		}
	}
	
	return embedded
}

func (c *CrossLanguageStrategy) groupServersByLanguage(servers []*ServerInstance, languages []string) map[string][]*ServerInstance {
	groups := make(map[string][]*ServerInstance)
	
	for _, language := range languages {
		groups[language] = []*ServerInstance{}
	}
	
	for _, server := range servers {
		if !server.IsHealthy() || !server.circuitBreaker.CanExecute() {
			continue
		}
		
		for _, language := range languages {
			if c.serverSupportsLanguage(server, language) {
				groups[language] = append(groups[language], server)
			}
		}
	}
	
	return groups
}

func (c *CrossLanguageStrategy) selectCrossLanguageServers(groups map[string][]*ServerInstance, languages []string) []*ServerInstance {
	var selected []*ServerInstance
	usedServers := make(map[string]bool)
	
	maxServers := 7
	if c.config != nil && c.config.MaxServers > 0 {
		maxServers = c.config.MaxServers
	}
	
	// Prioritize multi-language servers
	for _, servers := range groups {
		for _, server := range servers {
			if usedServers[server.config.Name] {
				continue
			}
			
			// Prefer servers that support multiple required languages
			supportedCount := c.countSupportedLanguages(server, languages)
			if supportedCount > 1 && len(selected) < maxServers {
				selected = append(selected, server)
				usedServers[server.config.Name] = true
			}
		}
	}
	
	// Add specialized servers for remaining languages
	for _, language := range languages {
		if len(selected) >= maxServers {
			break
		}
		
		servers := groups[language]
		for _, server := range servers {
			if usedServers[server.config.Name] {
				continue
			}
			
			selected = append(selected, server)
			usedServers[server.config.Name] = true
			break
		}
	}
	
	return selected
}

func (c *CrossLanguageStrategy) serverSupportsLanguage(server *ServerInstance, language string) bool {
	for _, lang := range server.config.Languages {
		if lang == language {
			return true
		}
	}
	return false
}

func (c *CrossLanguageStrategy) countSupportedLanguages(server *ServerInstance, languages []string) int {
	count := 0
	for _, language := range languages {
		if c.serverSupportsLanguage(server, language) {
			count++
		}
	}
	return count
}

func (c *CrossLanguageStrategy) getCoordinationRole(server *ServerInstance, languages []string) string {
	supportedCount := c.countSupportedLanguages(server, languages)
	if supportedCount > 1 {
		return "coordinator"
	}
	return "specialist"
}

func (c *CrossLanguageStrategy) supportsTemplateLanguages(server *ServerInstance) bool {
	templateLanguages := []string{"html", "vue", "jsx", "tsx"}
	for _, lang := range server.config.Languages {
		if c.contains(templateLanguages, lang) {
			return true
		}
	}
	return false
}

func (c *CrossLanguageStrategy) contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Helper functions for server metrics and conversion

// getHealthScore calculates a health score from server health status
func (s *SingleTargetStrategy) getHealthScore(server *ServerInstance) float64 {
	if server.healthStatus == nil {
		return 0.5 // Default moderate health
	}
	// Convert error rate to health score (1.0 - error_rate)
	return 1.0 - server.healthStatus.ErrorRate
}

func (s *SingleTargetStrategy) getLoadScore(server *ServerInstance) float64 {
	if server.metrics == nil {
		return 0.5 // Default moderate load
	}
	// Simple load score based on connection count (normalized)
	// This is a simplified calculation - in practice you'd want more sophisticated metrics
	maxConnections := 100.0
	if server.connectionCount > int32(maxConnections) {
		return 1.0
	}
	return float64(server.connectionCount) / maxConnections
}

func (s *SingleTargetStrategy) getServerWeight(server *ServerInstance) float64 {
	if server.config == nil {
		return 1.0 // Default weight
	}
	// Use priority as weight (higher priority = higher weight)
	return float64(server.config.Priority) / 10.0
}


// MultiTargetStrategy helper methods
func (m *MultiTargetStrategy) getHealthScore(server *ServerInstance) float64 {
	if server.healthStatus == nil {
		return 0.5
	}
	return 1.0 - server.healthStatus.ErrorRate
}

func (m *MultiTargetStrategy) getLoadScore(server *ServerInstance) float64 {
	if server.metrics == nil {
		return 0.5
	}
	maxConnections := 100.0
	if server.connectionCount > int32(maxConnections) {
		return 1.0
	}
	return float64(server.connectionCount) / maxConnections
}

func (m *MultiTargetStrategy) getServerWeight(server *ServerInstance) float64 {
	if server.config == nil {
		return 1.0
	}
	return float64(server.config.Priority) / 10.0
}


// BroadcastAggregateStrategy helper methods
func (b *BroadcastAggregateStrategy) getHealthScore(server *ServerInstance) float64 {
	if server.healthStatus == nil {
		return 0.5
	}
	return 1.0 - server.healthStatus.ErrorRate
}

func (b *BroadcastAggregateStrategy) getLoadScore(server *ServerInstance) float64 {
	if server.metrics == nil {
		return 0.5
	}
	maxConnections := 100.0
	if server.connectionCount > int32(maxConnections) {
		return 1.0
	}
	return float64(server.connectionCount) / maxConnections
}

func (b *BroadcastAggregateStrategy) getServerWeight(server *ServerInstance) float64 {
	if server.config == nil {
		return 1.0
	}
	return float64(server.config.Priority) / 10.0
}


// SequentialFallbackStrategy helper methods
func (s *SequentialFallbackStrategy) getHealthScore(server *ServerInstance) float64 {
	if server.healthStatus == nil {
		return 0.5
	}
	return 1.0 - server.healthStatus.ErrorRate
}

func (s *SequentialFallbackStrategy) getLoadScore(server *ServerInstance) float64 {
	if server.metrics == nil {
		return 0.5
	}
	maxConnections := 100.0
	if server.connectionCount > int32(maxConnections) {
		return 1.0
	}
	return float64(server.connectionCount) / maxConnections
}

func (s *SequentialFallbackStrategy) getServerWeight(server *ServerInstance) float64 {
	if server.config == nil {
		return 1.0
	}
	return float64(server.config.Priority) / 10.0
}


// LoadBalancedStrategy helper methods
func (l *LoadBalancedStrategy) getHealthScore(server *ServerInstance) float64 {
	if server.healthStatus == nil {
		return 0.5
	}
	return 1.0 - server.healthStatus.ErrorRate
}

func (l *LoadBalancedStrategy) getLoadScore(server *ServerInstance) float64 {
	if server.metrics == nil {
		return 0.5
	}
	maxConnections := 100.0
	if server.connectionCount > int32(maxConnections) {
		return 1.0
	}
	return float64(server.connectionCount) / maxConnections
}

func (l *LoadBalancedStrategy) getServerWeight(server *ServerInstance) float64 {
	if server.config == nil {
		return 1.0
	}
	return float64(server.config.Priority) / 10.0
}


// CrossLanguageStrategy helper methods
func (c *CrossLanguageStrategy) getHealthScore(server *ServerInstance) float64 {
	if server.healthStatus == nil {
		return 0.5
	}
	return 1.0 - server.healthStatus.ErrorRate
}

func (c *CrossLanguageStrategy) getLoadScore(server *ServerInstance) float64 {
	if server.metrics == nil {
		return 0.5
	}
	maxConnections := 100.0
	if server.connectionCount > int32(maxConnections) {
		return 1.0
	}
	return float64(server.connectionCount) / maxConnections
}

func (c *CrossLanguageStrategy) getServerWeight(server *ServerInstance) float64 {
	if server.config == nil {
		return 1.0
	}
	return float64(server.config.Priority) / 10.0
}


// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}