package gateway

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"lsp-gateway/internal/config"
)

// Priority represents request priority levels
type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
	PriorityUrgent
)

// SelectionRequestContext provides context for server selection decisions
type SelectionRequestContext struct {
	RequestID        string        `json:"requestId"`
	URI              string        `json:"uri"`
	Method           string        `json:"method"`
	Params           interface{}   `json:"params"`
	Priority         Priority      `json:"priority"`
	RequiredFeatures []string      `json:"requiredFeatures,omitempty"`
	PreferredServer  string        `json:"preferredServer,omitempty"`
	Timeout          time.Duration `json:"timeout"`
	WorkspaceRoot    string        `json:"workspaceRoot,omitempty"`
}

// ServerSelector interface defines methods for selecting servers from pools
type ServerSelector interface {
	SelectServer(pool *LanguageServerPool, requestType string, context *SelectionRequestContext) (*ServerInstance, error)
	SelectMultipleServers(pool *LanguageServerPool, requestType string, maxServers int) ([]*ServerInstance, error)
	UpdateServerMetrics(serverName string, responseTime time.Duration, success bool)
	RebalancePool(pool *LanguageServerPool) error
	GetName() string
}

// RoundRobinSelector implements round-robin server selection
type RoundRobinSelector struct {
	name         string
	currentIndex int64
	mu           sync.RWMutex
}

// NewRoundRobinSelector creates a new round-robin selector
func NewRoundRobinSelector() *RoundRobinSelector {
	return &RoundRobinSelector{
		name: "round_robin",
	}
}

func (rr *RoundRobinSelector) GetName() string {
	return rr.name
}

func (rr *RoundRobinSelector) SelectServer(pool *LanguageServerPool, requestType string, context *SelectionRequestContext) (*ServerInstance, error) {
	servers := pool.GetHealthyServers()
	if len(servers) == 0 {
		return nil, fmt.Errorf("no healthy servers available for language %s", pool.language)
	}

	// Sort servers by name for consistent ordering
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].config.Name < servers[j].config.Name
	})

	index := atomic.AddInt64(&rr.currentIndex, 1) % int64(len(servers))
	return servers[index], nil
}

func (rr *RoundRobinSelector) SelectMultipleServers(pool *LanguageServerPool, requestType string, maxServers int) ([]*ServerInstance, error) {
	servers := pool.GetHealthyServers()
	if len(servers) == 0 {
		return nil, fmt.Errorf("no healthy servers available for language %s", pool.language)
	}

	if maxServers >= len(servers) {
		return servers, nil
	}

	// Sort servers by name for consistent ordering
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].config.Name < servers[j].config.Name
	})

	selected := make([]*ServerInstance, maxServers)
	for i := 0; i < maxServers; i++ {
		index := (atomic.LoadInt64(&rr.currentIndex) + int64(i)) % int64(len(servers))
		selected[i] = servers[index]
	}

	atomic.AddInt64(&rr.currentIndex, int64(maxServers))
	return selected, nil
}

func (rr *RoundRobinSelector) UpdateServerMetrics(serverName string, responseTime time.Duration, success bool) {
	// Round-robin doesn't use metrics for selection
}

func (rr *RoundRobinSelector) RebalancePool(pool *LanguageServerPool) error {
	// Round-robin doesn't require rebalancing
	return nil
}

// LeastConnectionsSelector implements least connections server selection
type LeastConnectionsSelector struct {
	name             string
	connectionCounts map[string]*int64
	mu               sync.RWMutex
}

// NewLeastConnectionsSelector creates a new least connections selector
func NewLeastConnectionsSelector() *LeastConnectionsSelector {
	return &LeastConnectionsSelector{
		name:             "least_connections",
		connectionCounts: make(map[string]*int64),
	}
}

func (lc *LeastConnectionsSelector) GetName() string {
	return lc.name
}

func (lc *LeastConnectionsSelector) SelectServer(pool *LanguageServerPool, requestType string, context *SelectionRequestContext) (*ServerInstance, error) {
	servers := pool.GetHealthyServers()
	if len(servers) == 0 {
		return nil, fmt.Errorf("no healthy servers available for language %s", pool.language)
	}

	lc.mu.Lock()
	defer lc.mu.Unlock()

	var bestServer *ServerInstance
	var minConnections int64 = math.MaxInt64

	for _, server := range servers {
		if _, exists := lc.connectionCounts[server.config.Name]; !exists {
			count := int64(0)
			lc.connectionCounts[server.config.Name] = &count
		}

		connections := atomic.LoadInt64(lc.connectionCounts[server.config.Name])
		if connections < minConnections {
			minConnections = connections
			bestServer = server
		}
	}

	if bestServer != nil {
		atomic.AddInt64(lc.connectionCounts[bestServer.config.Name], 1)
	}

	return bestServer, nil
}

func (lc *LeastConnectionsSelector) SelectMultipleServers(pool *LanguageServerPool, requestType string, maxServers int) ([]*ServerInstance, error) {
	servers := pool.GetHealthyServers()
	if len(servers) == 0 {
		return nil, fmt.Errorf("no healthy servers available for language %s", pool.language)
	}

	if maxServers >= len(servers) {
		return servers, nil
	}

	lc.mu.Lock()
	defer lc.mu.Unlock()

	// Initialize connection counts if needed
	for _, server := range servers {
		if _, exists := lc.connectionCounts[server.config.Name]; !exists {
			count := int64(0)
			lc.connectionCounts[server.config.Name] = &count
		}
	}

	// Sort servers by connection count
	sort.Slice(servers, func(i, j int) bool {
		countI := atomic.LoadInt64(lc.connectionCounts[servers[i].config.Name])
		countJ := atomic.LoadInt64(lc.connectionCounts[servers[j].config.Name])
		return countI < countJ
	})

	selected := servers[:maxServers]
	for _, server := range selected {
		atomic.AddInt64(lc.connectionCounts[server.config.Name], 1)
	}

	return selected, nil
}

func (lc *LeastConnectionsSelector) UpdateServerMetrics(serverName string, responseTime time.Duration, success bool) {
	lc.mu.RLock()
	if count, exists := lc.connectionCounts[serverName]; exists {
		atomic.AddInt64(count, -1) // Decrement when request completes
	}
	lc.mu.RUnlock()
}

func (lc *LeastConnectionsSelector) RebalancePool(pool *LanguageServerPool) error {
	// Clean up connection counts for removed servers
	lc.mu.Lock()
	defer lc.mu.Unlock()

	servers := pool.GetHealthyServers()
	validServers := make(map[string]bool)
	for _, server := range servers {
		validServers[server.config.Name] = true
	}

	for serverName := range lc.connectionCounts {
		if !validServers[serverName] {
			delete(lc.connectionCounts, serverName)
		}
	}

	return nil
}

// ResponseTimeMetrics tracks response time statistics for a server
type ResponseTimeMetrics struct {
	averageResponseTime time.Duration
	lastResponseTime    time.Duration
	sampleCount         int64
	totalResponseTime   int64 // in nanoseconds
	lastUpdated         time.Time
	mu                  sync.RWMutex
}

// UpdateMetrics updates the response time metrics
func (rtm *ResponseTimeMetrics) UpdateMetrics(responseTime time.Duration, success bool) {
	rtm.mu.Lock()
	defer rtm.mu.Unlock()

	if success {
		rtm.lastResponseTime = responseTime
		rtm.sampleCount++
		rtm.totalResponseTime += responseTime.Nanoseconds()
		rtm.averageResponseTime = time.Duration(rtm.totalResponseTime / rtm.sampleCount)
		rtm.lastUpdated = time.Now()
	}
}

// GetAverageResponseTime returns the current average response time
func (rtm *ResponseTimeMetrics) GetAverageResponseTime() time.Duration {
	rtm.mu.RLock()
	defer rtm.mu.RUnlock()
	return rtm.averageResponseTime
}

// ResponseTimeSelector implements response time-based server selection
type ResponseTimeSelector struct {
	name                string
	responseTimeMetrics map[string]*ResponseTimeMetrics
	mu                  sync.RWMutex
}

// NewResponseTimeSelector creates a new response time selector
func NewResponseTimeSelector() *ResponseTimeSelector {
	return &ResponseTimeSelector{
		name:                "response_time",
		responseTimeMetrics: make(map[string]*ResponseTimeMetrics),
	}
}

func (rt *ResponseTimeSelector) GetName() string {
	return rt.name
}

func (rt *ResponseTimeSelector) SelectServer(pool *LanguageServerPool, requestType string, context *SelectionRequestContext) (*ServerInstance, error) {
	servers := pool.GetHealthyServers()
	if len(servers) == 0 {
		return nil, fmt.Errorf("no healthy servers available for language %s", pool.language)
	}

	rt.mu.Lock()
	defer rt.mu.Unlock()

	var bestServer *ServerInstance
	var bestResponseTime time.Duration = time.Duration(math.MaxInt64)

	for _, server := range servers {
		if _, exists := rt.responseTimeMetrics[server.config.Name]; !exists {
			rt.responseTimeMetrics[server.config.Name] = &ResponseTimeMetrics{}
		}

		metrics := rt.responseTimeMetrics[server.config.Name]
		avgTime := metrics.GetAverageResponseTime()

		// If no metrics yet, use this server
		if avgTime == 0 {
			return server, nil
		}

		if avgTime < bestResponseTime {
			bestResponseTime = avgTime
			bestServer = server
		}
	}

	return bestServer, nil
}

func (rt *ResponseTimeSelector) SelectMultipleServers(pool *LanguageServerPool, requestType string, maxServers int) ([]*ServerInstance, error) {
	servers := pool.GetHealthyServers()
	if len(servers) == 0 {
		return nil, fmt.Errorf("no healthy servers available for language %s", pool.language)
	}

	if maxServers >= len(servers) {
		return servers, nil
	}

	rt.mu.Lock()
	defer rt.mu.Unlock()

	// Initialize metrics if needed
	for _, server := range servers {
		if _, exists := rt.responseTimeMetrics[server.config.Name]; !exists {
			rt.responseTimeMetrics[server.config.Name] = &ResponseTimeMetrics{}
		}
	}

	// Sort servers by average response time
	sort.Slice(servers, func(i, j int) bool {
		timeI := rt.responseTimeMetrics[servers[i].config.Name].GetAverageResponseTime()
		timeJ := rt.responseTimeMetrics[servers[j].config.Name].GetAverageResponseTime()

		// Servers with no metrics (time == 0) get priority
		if timeI == 0 && timeJ != 0 {
			return true
		}
		if timeI != 0 && timeJ == 0 {
			return false
		}
		return timeI < timeJ
	})

	return servers[:maxServers], nil
}

func (rt *ResponseTimeSelector) UpdateServerMetrics(serverName string, responseTime time.Duration, success bool) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if _, exists := rt.responseTimeMetrics[serverName]; !exists {
		rt.responseTimeMetrics[serverName] = &ResponseTimeMetrics{}
	}

	rt.responseTimeMetrics[serverName].UpdateMetrics(responseTime, success)
}

func (rt *ResponseTimeSelector) RebalancePool(pool *LanguageServerPool) error {
	// Clean up metrics for removed servers
	rt.mu.Lock()
	defer rt.mu.Unlock()

	servers := pool.GetHealthyServers()
	validServers := make(map[string]bool)
	for _, server := range servers {
		validServers[server.config.Name] = true
	}

	for serverName := range rt.responseTimeMetrics {
		if !validServers[serverName] {
			delete(rt.responseTimeMetrics, serverName)
		}
	}

	return nil
}

// WeightFactors defines weights for different performance aspects
type WeightFactors struct {
	ResponseTimeWeight float64 `json:"responseTimeWeight"`
	AvailabilityWeight float64 `json:"availabilityWeight"`
	ResourceWeight     float64 `json:"resourceWeight"`
}

// DefaultWeightFactors returns default weight factors
func DefaultWeightFactors() *WeightFactors {
	return &WeightFactors{
		ResponseTimeWeight: 0.4,
		AvailabilityWeight: 0.4,
		ResourceWeight:     0.2,
	}
}

// PerformanceScore represents the computed performance score for a server
type PerformanceScore struct {
	responseTimeScore float64
	availabilityScore float64
	resourceScore     float64
	combinedScore     float64
	lastCalculated    time.Time
	mu                sync.RWMutex
}

// UpdateScore updates the performance score
func (ps *PerformanceScore) UpdateScore(responseTime float64, availability float64, resource float64, weights *WeightFactors) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.responseTimeScore = responseTime
	ps.availabilityScore = availability
	ps.resourceScore = resource
	ps.combinedScore = (responseTime * weights.ResponseTimeWeight) +
		(availability * weights.AvailabilityWeight) +
		(resource * weights.ResourceWeight)
	ps.lastCalculated = time.Now()
}

// GetCombinedScore returns the current combined score
func (ps *PerformanceScore) GetCombinedScore() float64 {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.combinedScore
}

// PerformanceBasedSelector implements performance-based server selection
type PerformanceBasedSelector struct {
	name          string
	serverScores  map[string]*PerformanceScore
	weightFactors *WeightFactors
	mu            sync.RWMutex
}

// NewPerformanceBasedSelector creates a new performance-based selector
func NewPerformanceBasedSelector(weights *WeightFactors) *PerformanceBasedSelector {
	if weights == nil {
		weights = DefaultWeightFactors()
	}

	return &PerformanceBasedSelector{
		name:          "performance",
		serverScores:  make(map[string]*PerformanceScore),
		weightFactors: weights,
	}
}

func (pb *PerformanceBasedSelector) GetName() string {
	return pb.name
}

func (pb *PerformanceBasedSelector) SelectServer(pool *LanguageServerPool, requestType string, context *SelectionRequestContext) (*ServerInstance, error) {
	servers := pool.GetHealthyServers()
	if len(servers) == 0 {
		return nil, fmt.Errorf("no healthy servers available for language %s", pool.language)
	}

	pb.mu.Lock()
	defer pb.mu.Unlock()

	var bestServer *ServerInstance
	var bestScore float64 = 0

	for _, server := range servers {
		score := pb.calculatePerformanceScore(server)
		if score > bestScore {
			bestScore = score
			bestServer = server
		}
	}

	return bestServer, nil
}

func (pb *PerformanceBasedSelector) SelectMultipleServers(pool *LanguageServerPool, requestType string, maxServers int) ([]*ServerInstance, error) {
	servers := pool.GetHealthyServers()
	if len(servers) == 0 {
		return nil, fmt.Errorf("no healthy servers available for language %s", pool.language)
	}

	if maxServers >= len(servers) {
		return servers, nil
	}

	pb.mu.Lock()
	defer pb.mu.Unlock()

	// Sort servers by performance score (descending)
	sort.Slice(servers, func(i, j int) bool {
		scoreI := pb.calculatePerformanceScore(servers[i])
		scoreJ := pb.calculatePerformanceScore(servers[j])
		return scoreI > scoreJ
	})

	return servers[:maxServers], nil
}

func (pb *PerformanceBasedSelector) calculatePerformanceScore(server *ServerInstance) float64 {
	if _, exists := pb.serverScores[server.config.Name]; !exists {
		pb.serverScores[server.config.Name] = &PerformanceScore{}
	}

	metrics := server.GetMetrics()
	if metrics == nil {
		return 0.0
	}

	// Calculate response time score (lower is better, so invert)
	// Use average response time calculated from total response time and request count
	var responseTimeScore float64 = 1.0
	if metrics.RequestCount > 0 {
		averageResponseTime := metrics.TotalResponseTime / time.Duration(metrics.RequestCount)
		responseTimeMs := float64(averageResponseTime.Milliseconds())
		responseTimeScore = 1.0 / (1.0 + responseTimeMs/1000.0)
	}

	// Availability score calculated from success rate
	var availabilityScore float64 = 1.0
	if metrics.RequestCount > 0 {
		availabilityScore = float64(metrics.SuccessCount) / float64(metrics.RequestCount)
	}

	// Resource score based on inverse of error rate (health score)
	resourceScore := 1.0 - metrics.ErrorRate

	score := pb.serverScores[server.config.Name]
	score.UpdateScore(responseTimeScore, availabilityScore, resourceScore, pb.weightFactors)

	return score.GetCombinedScore()
}

func (pb *PerformanceBasedSelector) UpdateServerMetrics(serverName string, responseTime time.Duration, success bool) {
	// Performance metrics are updated through server metrics, not directly here
}

func (pb *PerformanceBasedSelector) RebalancePool(pool *LanguageServerPool) error {
	// Clean up scores for removed servers
	pb.mu.Lock()
	defer pb.mu.Unlock()

	servers := pool.GetHealthyServers()
	validServers := make(map[string]bool)
	for _, server := range servers {
		validServers[server.config.Name] = true
	}

	for serverName := range pb.serverScores {
		if !validServers[serverName] {
			delete(pb.serverScores, serverName)
		}
	}

	return nil
}

// ServerCapabilities represents the capabilities of a server
type ServerCapabilities struct {
	SupportedMethods  []string           `json:"supportedMethods"`
	SupportedFeatures []string           `json:"supportedFeatures"`
	CapabilityScore   map[string]float64 `json:"capabilityScore"`
	LastUpdated       time.Time          `json:"lastUpdated"`
	mu                sync.RWMutex
}

// HasMethod checks if the server supports a specific method
func (sc *ServerCapabilities) HasMethod(method string) bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	for _, m := range sc.SupportedMethods {
		if m == method {
			return true
		}
	}
	return false
}

// HasFeature checks if the server supports a specific feature
func (sc *ServerCapabilities) HasFeature(feature string) bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	for _, f := range sc.SupportedFeatures {
		if f == feature {
			return true
		}
	}
	return false
}

// FeatureBasedSelector implements feature-based server selection
type FeatureBasedSelector struct {
	name               string
	serverCapabilities map[string]*ServerCapabilities
	mu                 sync.RWMutex
}

// NewFeatureBasedSelector creates a new feature-based selector
func NewFeatureBasedSelector() *FeatureBasedSelector {
	return &FeatureBasedSelector{
		name:               "feature",
		serverCapabilities: make(map[string]*ServerCapabilities),
	}
}

func (fb *FeatureBasedSelector) GetName() string {
	return fb.name
}

func (fb *FeatureBasedSelector) SelectServer(pool *LanguageServerPool, requestType string, context *SelectionRequestContext) (*ServerInstance, error) {
	servers := pool.GetHealthyServers()
	if len(servers) == 0 {
		return nil, fmt.Errorf("no healthy servers available for language %s", pool.language)
	}

	fb.mu.Lock()
	defer fb.mu.Unlock()

	// If no specific features required, use first healthy server
	if len(context.RequiredFeatures) == 0 {
		return servers[0], nil
	}

	var bestServer *ServerInstance
	var bestMatch float64 = 0

	for _, server := range servers {
		if fb.checkServerCapabilities(server, context.RequiredFeatures) {
			match := fb.calculateFeatureMatch(server, requestType)
			if match > bestMatch {
				bestMatch = match
				bestServer = server
			}
		}
	}

	if bestServer == nil {
		// Fallback to first server if no perfect match
		return servers[0], nil
	}

	return bestServer, nil
}

func (fb *FeatureBasedSelector) SelectMultipleServers(pool *LanguageServerPool, requestType string, maxServers int) ([]*ServerInstance, error) {
	servers := pool.GetHealthyServers()
	if len(servers) == 0 {
		return nil, fmt.Errorf("no healthy servers available for language %s", pool.language)
	}

	if maxServers >= len(servers) {
		return servers, nil
	}

	fb.mu.Lock()
	defer fb.mu.Unlock()

	// Sort servers by feature match score
	sort.Slice(servers, func(i, j int) bool {
		scoreI := fb.calculateFeatureMatch(servers[i], requestType)
		scoreJ := fb.calculateFeatureMatch(servers[j], requestType)
		return scoreI > scoreJ
	})

	return servers[:maxServers], nil
}

func (fb *FeatureBasedSelector) checkServerCapabilities(server *ServerInstance, requiredFeatures []string) bool {
	if _, exists := fb.serverCapabilities[server.config.Name]; !exists {
		// Initialize default capabilities
		fb.serverCapabilities[server.config.Name] = &ServerCapabilities{
			SupportedMethods:  []string{"textDocument/definition", "textDocument/references", "textDocument/hover"},
			SupportedFeatures: []string{"basic"},
			CapabilityScore:   make(map[string]float64),
			LastUpdated:       time.Now(),
		}
	}

	capabilities := fb.serverCapabilities[server.config.Name]
	for _, feature := range requiredFeatures {
		if !capabilities.HasFeature(feature) {
			return false
		}
	}

	return true
}

func (fb *FeatureBasedSelector) calculateFeatureMatch(server *ServerInstance, requestType string) float64 {
	if _, exists := fb.serverCapabilities[server.config.Name]; !exists {
		return 0.5 // Default score for unknown capabilities
	}

	capabilities := fb.serverCapabilities[server.config.Name]
	if !capabilities.HasMethod(requestType) {
		return 0.0
	}

	// Return capability score for this request type, or default
	if score, exists := capabilities.CapabilityScore[requestType]; exists {
		return score
	}

	return 0.8 // Default good score for supported methods
}

func (fb *FeatureBasedSelector) UpdateServerMetrics(serverName string, responseTime time.Duration, success bool) {
	// Feature-based selection doesn't directly use response time metrics
}

func (fb *FeatureBasedSelector) RebalancePool(pool *LanguageServerPool) error {
	// Clean up capabilities for removed servers
	fb.mu.Lock()
	defer fb.mu.Unlock()

	servers := pool.GetHealthyServers()
	validServers := make(map[string]bool)
	for _, server := range servers {
		validServers[server.config.Name] = true
	}

	for serverName := range fb.serverCapabilities {
		if !validServers[serverName] {
			delete(fb.serverCapabilities, serverName)
		}
	}

	return nil
}

// MultiServerSelector combines multiple selection strategies with fallback
type MultiServerSelector struct {
	name               string
	primaryStrategy    ServerSelector
	fallbackStrategies []ServerSelector
	affinityRules      map[string]string // request type -> preferred server
	mu                 sync.RWMutex
}

// NewMultiServerSelector creates a new multi-server selector
func NewMultiServerSelector(primary ServerSelector, fallbacks []ServerSelector) *MultiServerSelector {
	return &MultiServerSelector{
		name:               "multi_strategy",
		primaryStrategy:    primary,
		fallbackStrategies: fallbacks,
		affinityRules:      make(map[string]string),
	}
}

func (ms *MultiServerSelector) GetName() string {
	return ms.name
}

func (ms *MultiServerSelector) SelectServer(pool *LanguageServerPool, requestType string, context *SelectionRequestContext) (*ServerInstance, error) {
	// Try primary strategy first
	if server, err := ms.primaryStrategy.SelectServer(pool, requestType, context); err == nil {
		return server, nil
	}

	// Try fallback strategies
	for _, strategy := range ms.fallbackStrategies {
		if server, err := strategy.SelectServer(pool, requestType, context); err == nil {
			return server, nil
		}
	}

	return nil, fmt.Errorf("no available servers found using any strategy for language %s", pool.language)
}

func (ms *MultiServerSelector) SelectMultipleServers(pool *LanguageServerPool, requestType string, maxServers int) ([]*ServerInstance, error) {
	return ms.selectWithDiversification(pool, requestType, maxServers)
}

func (ms *MultiServerSelector) selectWithDiversification(pool *LanguageServerPool, requestType string, maxServers int) ([]*ServerInstance, error) {
	var selected []*ServerInstance
	usedStrategies := make(map[string]bool)

	// Try to select servers using different strategies for diversity
	strategies := append([]ServerSelector{ms.primaryStrategy}, ms.fallbackStrategies...)

	for _, strategy := range strategies {
		if len(selected) >= maxServers {
			break
		}

		if usedStrategies[strategy.GetName()] {
			continue
		}

		if servers, err := strategy.SelectMultipleServers(pool, requestType, maxServers-len(selected)); err == nil {
			for _, server := range servers {
				if !ms.avoidDuplicateSelection(selected, server) {
					selected = append(selected, server)
					if len(selected) >= maxServers {
						break
					}
				}
			}
			usedStrategies[strategy.GetName()] = true
		}
	}

	if len(selected) == 0 {
		return nil, fmt.Errorf("no servers selected using any strategy for language %s", pool.language)
	}

	return selected, nil
}

func (ms *MultiServerSelector) avoidDuplicateSelection(selected []*ServerInstance, candidate *ServerInstance) bool {
	for _, server := range selected {
		if server.config.Name == candidate.config.Name {
			return true
		}
	}
	return false
}

func (ms *MultiServerSelector) UpdateServerMetrics(serverName string, responseTime time.Duration, success bool) {
	ms.primaryStrategy.UpdateServerMetrics(serverName, responseTime, success)
	for _, strategy := range ms.fallbackStrategies {
		strategy.UpdateServerMetrics(serverName, responseTime, success)
	}
}

func (ms *MultiServerSelector) RebalancePool(pool *LanguageServerPool) error {
	if err := ms.primaryStrategy.RebalancePool(pool); err != nil {
		return err
	}

	for _, strategy := range ms.fallbackStrategies {
		if err := strategy.RebalancePool(pool); err != nil {
			return err
		}
	}

	return nil
}

// HealthAwareSelector wraps another selector with health awareness
type HealthAwareSelector struct {
	name              string
	baseSelector      ServerSelector
	healthThreshold   float64
	maxUnhealthyRatio float64
	mu                sync.RWMutex
}

// NewHealthAwareSelector creates a new health-aware selector
func NewHealthAwareSelector(base ServerSelector, healthThreshold, maxUnhealthyRatio float64) *HealthAwareSelector {
	return &HealthAwareSelector{
		name:              "health_aware_" + base.GetName(),
		baseSelector:      base,
		healthThreshold:   healthThreshold,
		maxUnhealthyRatio: maxUnhealthyRatio,
	}
}

func (ha *HealthAwareSelector) GetName() string {
	return ha.name
}

func (ha *HealthAwareSelector) SelectServer(pool *LanguageServerPool, requestType string, context *SelectionRequestContext) (*ServerInstance, error) {
	healthyServers := ha.getHealthyServers(pool)
	if len(healthyServers) == 0 {
		return nil, fmt.Errorf("no healthy servers available for language %s", pool.language)
	}

	// Create a temporary pool with only healthy servers
	healthyPool := &LanguageServerPool{
		language:      pool.language,
		servers:       make(map[string]*ServerInstance),
		activeServers: healthyServers,
	}

	for _, server := range healthyServers {
		healthyPool.servers[server.config.Name] = server
	}

	return ha.baseSelector.SelectServer(healthyPool, requestType, context)
}

func (ha *HealthAwareSelector) SelectMultipleServers(pool *LanguageServerPool, requestType string, maxServers int) ([]*ServerInstance, error) {
	healthyServers := ha.getHealthyServers(pool)
	if len(healthyServers) == 0 {
		return nil, fmt.Errorf("no healthy servers available for language %s", pool.language)
	}

	// Create a temporary pool with only healthy servers
	healthyPool := &LanguageServerPool{
		language:      pool.language,
		servers:       make(map[string]*ServerInstance),
		activeServers: healthyServers,
	}

	for _, server := range healthyServers {
		healthyPool.servers[server.config.Name] = server
	}

	return ha.baseSelector.SelectMultipleServers(healthyPool, requestType, maxServers)
}

func (ha *HealthAwareSelector) isServerHealthy(server *ServerInstance) bool {
	metrics := server.GetMetrics()
	if metrics == nil {
		return false
	}

	// Calculate health score from error rate (1.0 - error_rate)
	healthScore := 1.0 - metrics.ErrorRate
	
	// For now, just check health score - circuit breaker is handled at transport level
	return healthScore >= ha.healthThreshold
}

func (ha *HealthAwareSelector) getHealthyServers(pool *LanguageServerPool) []*ServerInstance {
	var healthy []*ServerInstance
	for _, server := range pool.GetHealthyServers() {
		if ha.isServerHealthy(server) {
			healthy = append(healthy, server)
		}
	}
	return healthy
}

func (ha *HealthAwareSelector) UpdateServerMetrics(serverName string, responseTime time.Duration, success bool) {
	ha.baseSelector.UpdateServerMetrics(serverName, responseTime, success)
}

func (ha *HealthAwareSelector) RebalancePool(pool *LanguageServerPool) error {
	return ha.baseSelector.RebalancePool(pool)
}

// NewServerSelector creates a server selector based on configuration
func NewServerSelector(config *config.LoadBalancingConfig) (ServerSelector, error) {
	if config == nil {
		return NewRoundRobinSelector(), nil
	}

	var baseSelector ServerSelector
	var err error

	switch config.Strategy {
	case "round_robin":
		baseSelector = NewRoundRobinSelector()
	case "least_connections":
		baseSelector = NewLeastConnectionsSelector()
	case "response_time":
		baseSelector = NewResponseTimeSelector()
	case "performance":
		weights := DefaultWeightFactors()
		if config.WeightFactors != nil {
			if responseWeight, ok := config.WeightFactors["response_time"]; ok {
				weights.ResponseTimeWeight = responseWeight
			}
			if availabilityWeight, ok := config.WeightFactors["availability"]; ok {
				weights.AvailabilityWeight = availabilityWeight
			}
			if resourceWeight, ok := config.WeightFactors["resource"]; ok {
				weights.ResourceWeight = resourceWeight
			}
		}
		baseSelector = NewPerformanceBasedSelector(weights)
	case "feature":
		baseSelector = NewFeatureBasedSelector()
	default:
		return nil, fmt.Errorf("unknown selection strategy: %s", config.Strategy)
	}

	// Wrap with health awareness if threshold is specified
	if config.HealthThreshold > 0 {
		baseSelector = NewHealthAwareSelector(baseSelector, config.HealthThreshold, 0.3)
	}

	return baseSelector, err
}

// CreateSelectorForLanguage creates a selector specifically for a language
func CreateSelectorForLanguage(language string, config *config.LoadBalancingConfig) (ServerSelector, error) {
	selector, err := NewServerSelector(config)
	if err != nil {
		return nil, err
	}

	// Add language-specific optimizations here if needed
	return selector, nil
}
