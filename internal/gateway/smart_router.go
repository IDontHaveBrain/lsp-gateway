package gateway

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/transport"
	"lsp-gateway/mcp"
)

// RoutingStrategyType defines how requests should be routed to servers (enum type)
type RoutingStrategyType string

const (
	SingleTargetWithFallback RoutingStrategyType = "single_target_with_fallback"
	BroadcastAggregate       RoutingStrategyType = "broadcast_aggregate"
	MultiTargetParallel      RoutingStrategyType = "multi_target_parallel"
	PrimaryWithEnhancement   RoutingStrategyType = "primary_with_enhancement"
	LoadBalanced            RoutingStrategyType = "load_balanced"
	RoundRobin              RoutingStrategyType = "round_robin"
)

// Implement RoutingStrategy interface for RoutingStrategyType
func (rst RoutingStrategyType) Name() string {
	return string(rst)
}

func (rst RoutingStrategyType) Description() string {
	switch rst {
	case SingleTargetWithFallback:
		return "Route to single target with fallback options"
	case BroadcastAggregate:
		return "Broadcast to all servers and aggregate results"
	case MultiTargetParallel:
		return "Route to multiple targets in parallel"
	case PrimaryWithEnhancement:
		return "Route to primary server with enhancement from secondary"
	case LoadBalanced:
		return "Route using load balancing algorithm"
	case RoundRobin:
		return "Route using round-robin algorithm"
	default:
		return "Unknown routing strategy"
	}
}

func (rst RoutingStrategyType) Route(request *LSPRequest, availableServers []*ServerInstance) (*RoutingDecision, error) {
	// This is a placeholder implementation - actual routing logic would be in SmartRouter
	return nil, fmt.Errorf("routing not implemented for strategy type %s", rst)
}

// RoutingMetrics tracks performance metrics for routing decisions
type RoutingMetrics struct {
	TotalRequests       int64                       `json:"total_requests"`
	SuccessfulRequests  int64                       `json:"successful_requests"`
	FailedRequests      int64                       `json:"failed_requests"`
	AverageResponseTime time.Duration               `json:"average_response_time"`
	ServerMetrics       map[string]*ServerMetrics   `json:"server_metrics"`
	StrategyMetrics     map[string]*StrategyMetrics `json:"strategy_metrics"`
	LastUpdated         time.Time                   `json:"last_updated"`
	mu                  sync.RWMutex                `json:"-"`
}

// StrategyMetrics tracks performance metrics for routing strategies
type StrategyMetrics struct {
	RequestCount        int64         `json:"request_count"`
	SuccessCount        int64         `json:"success_count"`
	AverageResponseTime time.Duration `json:"average_response_time"`
	SuccessRate         float64       `json:"success_rate"`
}

// SmartRouter interface defines advanced routing capabilities with multi-server support
type SmartRouter interface {
	// Core routing methods
	RouteRequest(request *LSPRequest) (*RoutingDecision, error)
	RouteMultiRequest(request *LSPRequest) ([]*RoutingDecision, error)
	AggregateBroadcast(request *LSPRequest) (*AggregatedResponse, error)

	// Strategy management
	SetRoutingStrategy(method string, strategy RoutingStrategyType)
	GetRoutingStrategy(method string) RoutingStrategyType
	// Performance monitoring
	GetRoutingMetrics() *RoutingMetrics
	UpdateServerPerformance(serverName string, responseTime time.Duration, success bool)

	// Backward compatibility
	GetServerForFile(fileURI string) (string, error)
	GetServerForLanguage(language string) (string, error)
}

// SmartRouterImpl implements the SmartRouter interface with advanced routing capabilities
type SmartRouterImpl struct {
	*ProjectAwareRouter // Embedded for backward compatibility

	config           *config.GatewayConfig
	workspaceManager *WorkspaceManager
	logger           *mcp.StructuredLogger

	// Strategy management
	methodStrategies map[string]RoutingStrategyType
	strategyMu       sync.RWMutex

	// Performance tracking
	metrics *RoutingMetrics

	// Circuit breaker state
	circuitBreakers map[string]*CircuitBreaker
	cbMu            sync.RWMutex

	// Load balancing state
	roundRobinCounters map[string]int
	rrMu               sync.RWMutex
}

// CircuitBreaker implements circuit breaker pattern for server resilience

// NewSmartRouter creates a new SmartRouter with default configurations
func NewSmartRouter(projectRouter *ProjectAwareRouter, config *config.GatewayConfig, workspaceManager *WorkspaceManager, logger *mcp.StructuredLogger) *SmartRouterImpl {
	sr := &SmartRouterImpl{
		ProjectAwareRouter: projectRouter,
		config:             config,
		workspaceManager:   workspaceManager,
		logger:             logger,
		methodStrategies:   make(map[string]RoutingStrategyType),
		metrics: &RoutingMetrics{
			ServerMetrics:   make(map[string]*ServerMetrics),
			StrategyMetrics: make(map[string]*StrategyMetrics),
			LastUpdated:     time.Now(),
		},
		circuitBreakers:    make(map[string]*CircuitBreaker),
		roundRobinCounters: make(map[string]int),
	}

	// Set default routing strategies for common LSP methods
	sr.setDefaultStrategies()

	return sr
}

// setDefaultStrategies configures default routing strategies for common LSP methods
func (sr *SmartRouterImpl) setDefaultStrategies() {
	defaults := map[string]RoutingStrategyType{
		LSP_METHOD_DEFINITION:       SingleTargetWithFallback,
		LSP_METHOD_REFERENCES:       MultiTargetParallel,
		LSP_METHOD_DOCUMENT_SYMBOL:  SingleTargetWithFallback,
		LSP_METHOD_WORKSPACE_SYMBOL: BroadcastAggregate,
		LSP_METHOD_HOVER:            PrimaryWithEnhancement,
		"textDocument/completion":   LoadBalanced,
		"textDocument/diagnostic":   MultiTargetParallel,
		"textDocument/codeAction":   PrimaryWithEnhancement,
		"textDocument/formatting":   SingleTargetWithFallback,
	}

	for method, strategy := range defaults {
		sr.methodStrategies[method] = strategy
	}
}

// RouteRequest routes a single request using the appropriate strategy
func (sr *SmartRouterImpl) RouteRequest(request *LSPRequest) (*RoutingDecision, error) {
	if sr.logger != nil {
		sr.logger.Debugf("SmartRouter: Routing request for method %s, URI %s", request.Method, request.URI)
	}

	// Get routing strategy for this method
	strategy := sr.GetRoutingStrategy(request.Method)

	// Extract language from URI if not provided
	if (request.Context == nil || request.Context.Language == "") && request.URI != "" {
		if lang, err := sr.extractLanguageFromURI(request.URI); err == nil {
			if request.Context == nil {
				request.Context = &RequestContext{}
			}
			request.Context.Language = lang
		}
	}

	// Route based on strategy
	switch strategy.Name() {
	case "single_target_with_fallback":
		return sr.routeSingleTargetWithFallback(request)
	case "load_balanced":
		return sr.routeLoadBalanced(request)
	case "primary_with_enhancement":
		return sr.routePrimaryWithEnhancement(request)
	default:
		// Fallback to traditional routing
		return sr.routeTraditional(request)
	}
}

// RouteMultiRequest routes a request to multiple servers in parallel
func (sr *SmartRouterImpl) RouteMultiRequest(request *LSPRequest) ([]*RoutingDecision, error) {
	if sr.logger != nil {
		sr.logger.Debugf("SmartRouter: Multi-routing request for method %s", request.Method)
	}

	strategy := sr.GetRoutingStrategy(request.Method)

	switch strategy.Name() {
	case "multi_target_parallel", "broadcast_aggregate":
		return sr.routeMultiTarget(request)
	default:
		// Single target strategies return single decision
		if decision, err := sr.RouteRequest(request); err == nil {
			return []*RoutingDecision{decision}, nil
		} else {
			return nil, err
		}
	}
}

// AggregateBroadcast sends request to multiple servers and aggregates responses
func (sr *SmartRouterImpl) AggregateBroadcast(request *LSPRequest) (*AggregatedResponse, error) {
	startTime := time.Now()

	if sr.logger != nil {
		sr.logger.Debugf("SmartRouter: Broadcasting request for method %s", request.Method)
	}

	decisions, err := sr.RouteMultiRequest(request)
	if err != nil {
		return nil, fmt.Errorf("failed to route multi-request: %w", err)
	}

	if len(decisions) == 0 {
		return nil, fmt.Errorf("no servers available for broadcast")
	}

	// Execute requests in parallel
	responses := make([]ServerResponse, len(decisions))
	var wg sync.WaitGroup

	for i, decision := range decisions {
		wg.Add(1)
		go func(idx int, dec *RoutingDecision) {
			defer wg.Done()

			reqStart := time.Now()
			// Safety check for TargetServers
			if len(dec.TargetServers) == 0 {
				responses[idx] = ServerResponse{
					ServerName:   "unknown",
					Response:     nil,
					Error:        fmt.Errorf("no target servers available"),
					Duration:     time.Duration(0),
					Success:      false,
				}
				return
			}
			result, err := sr.executeRequest(dec.TargetServers[0].Client, request)
			responseTime := time.Since(reqStart)

			responses[idx] = ServerResponse{
				ServerName:   dec.TargetServers[0].Config.Name,
				Response:     result,
				Error:        err,
				Duration:     responseTime,
				Success:      err == nil,
			}

			// Update server performance metrics
			sr.UpdateServerPerformance(dec.TargetServers[0].Config.Name, responseTime, err == nil)
		}(i, decision)
	}

	wg.Wait()

	// Aggregate results
	var primaryResult interface{}
	var secondaryResults []ServerResponse
	successCount := 0

	for i, response := range responses {
		if response.Success {
			successCount++
			if i == 0 || primaryResult == nil {
				primaryResult = response.Response
			} else {
				secondaryResults = append(secondaryResults, response)
			}
		}
	}

	processingTime := time.Since(startTime)

	// Convert ServerResponse slice to interface slice for SecondaryResponses
	var secondaryInterfaces []interface{}
	for _, sr := range secondaryResults {
		secondaryInterfaces = append(secondaryInterfaces, sr.Response)
	}

	// Build response sources
	responseSources := make([]string, len(decisions))
	for i, response := range responses {
		if response.Success {
			responseSources[i] = response.ServerName
		}
	}

	aggregated := &AggregatedResponse{
		PrimaryResponse:    primaryResult,
		SecondaryResponses: secondaryInterfaces,
		AggregatedResult:   primaryResult,
		ResponseSources:    responseSources,
		ProcessingTime:     processingTime,
		AggregationMethod:  string(sr.GetRoutingStrategy(request.Method)),
		SuccessCount:       successCount,
		ErrorCount:         len(decisions) - successCount,
		Warnings:          []string{},
	}

	// Update strategy metrics
	sr.updateStrategyMetrics(sr.GetRoutingStrategy(request.Method).Name(), processingTime, successCount > 0)

	if successCount == 0 {
		return aggregated, fmt.Errorf("all servers failed to process request")
	}

	return aggregated, nil
}

// SetRoutingStrategy sets the routing strategy for a specific LSP method
func (sr *SmartRouterImpl) SetRoutingStrategy(method string, strategy RoutingStrategyType) {
	sr.strategyMu.Lock()
	defer sr.strategyMu.Unlock()

	sr.methodStrategies[method] = strategy

	if sr.logger != nil {
		sr.logger.Debugf("SmartRouter: Set routing strategy for %s to %s", method, strategy)
	}
}

// GetRoutingStrategy gets the routing strategy for a specific LSP method
func (sr *SmartRouterImpl) GetRoutingStrategy(method string) RoutingStrategyType {
	sr.strategyMu.RLock()
	defer sr.strategyMu.RUnlock()

	if strategy, exists := sr.methodStrategies[method]; exists {
		return strategy
	}

	// Return default strategy
	return SingleTargetWithFallback
}

// GetRoutingMetrics returns current routing performance metrics
func (sr *SmartRouterImpl) GetRoutingMetrics() *RoutingMetrics {
	sr.metrics.mu.RLock()
	defer sr.metrics.mu.RUnlock()

	// Create a deep copy to avoid race conditions
	metrics := &RoutingMetrics{
		TotalRequests:       sr.metrics.TotalRequests,
		SuccessfulRequests:  sr.metrics.SuccessfulRequests,
		FailedRequests:      sr.metrics.FailedRequests,
		AverageResponseTime: sr.metrics.AverageResponseTime,
		ServerMetrics:       make(map[string]*ServerMetrics),
		StrategyMetrics:     make(map[string]*StrategyMetrics),
		LastUpdated:         sr.metrics.LastUpdated,
	}

	// Copy server metrics
	for name, sm := range sr.metrics.ServerMetrics {
		metrics.ServerMetrics[name] = &ServerMetrics{
			TotalRequests:       sm.TotalRequests,
			SuccessfulRequests:  sm.SuccessfulRequests,
			FailedRequests:      sm.FailedRequests,
			AverageResponseTime: sm.AverageResponseTime,
			LastRequestTime:     sm.LastRequestTime,
			HealthScore:         sm.HealthScore,
			CircuitBreakerState: sm.CircuitBreakerState,
		}
	}

	// Copy strategy metrics
	for name, sm := range sr.metrics.StrategyMetrics {
		metrics.StrategyMetrics[name] = &StrategyMetrics{
			RequestCount:        sm.RequestCount,
			SuccessCount:        sm.SuccessCount,
			AverageResponseTime: sm.AverageResponseTime,
			SuccessRate:         sm.SuccessRate,
		}
	}

	return metrics
}

// UpdateServerPerformance updates performance metrics for a specific server
func (sr *SmartRouterImpl) UpdateServerPerformance(serverName string, responseTime time.Duration, success bool) {
	sr.metrics.mu.Lock()
	defer sr.metrics.mu.Unlock()

	// Initialize server metrics if not exists
	if sr.metrics.ServerMetrics[serverName] == nil {
		sr.metrics.ServerMetrics[serverName] = &ServerMetrics{
			HealthScore: 1.0,
		}
	}

	sm := sr.metrics.ServerMetrics[serverName]
	sm.TotalRequests++
	sm.LastRequestTime = time.Now()
	if success {
		sm.SuccessfulRequests++
		sr.metrics.SuccessfulRequests++
	} else {
		sm.FailedRequests++
		sr.metrics.FailedRequests++
	}

	sr.metrics.TotalRequests++

	// Update average response time
	if sm.TotalRequests > 0 {
		sm.AverageResponseTime = time.Duration(
			(int64(sm.AverageResponseTime)*(sm.TotalRequests-1) + int64(responseTime)) / sm.TotalRequests,
		)
	}

	// Calculate health score (exponential moving average)
	alpha := 0.1
	if success {
		sm.HealthScore = sm.HealthScore*(1-alpha) + alpha*1.0
	} else {
		sm.HealthScore = sm.HealthScore*(1-alpha) + alpha*0.0
	}

	// Update circuit breaker
	sr.updateCircuitBreaker(serverName, success)

	sr.metrics.LastUpdated = time.Now()
}

// GetServerForFile provides backward compatibility with existing Router interface
func (sr *SmartRouterImpl) GetServerForFile(fileURI string) (string, error) {
	if sr.logger != nil {
		sr.logger.Debugf("SmartRouter: Getting server for file %s (backward compatibility)", fileURI)
	}

	// Use ProjectAwareRouter for workspace-aware routing
	return sr.RouteRequestWithWorkspace(fileURI)
}

// GetServerForLanguage provides backward compatibility with existing Router interface
func (sr *SmartRouterImpl) GetServerForLanguage(language string) (string, error) {
	if sr.logger != nil {
		sr.logger.Debugf("SmartRouter: Getting server for language %s (backward compatibility)", language)
	}

	// Use embedded Router's functionality
	serverName, exists := sr.GetServerByLanguage(language)
	if !exists {
		return "", fmt.Errorf("no server found for language %s", language)
	}
	return serverName, nil
}

// Private helper methods

// routeSingleTargetWithFallback routes to primary server with fallback options
func (sr *SmartRouterImpl) routeSingleTargetWithFallback(request *LSPRequest) (*RoutingDecision, error) {
	if request.Context == nil || request.Context.Language == "" {
		return nil, fmt.Errorf("language not specified for single target routing")
	}

	// Get servers for language with priority ordering
	servers, err := sr.config.GetServersForLanguage(request.Context.Language, 3) // Max 3 for fallback
	if err != nil {
		return nil, fmt.Errorf("no servers available for language %s: %w", request.Context.Language, err)
	}

	// Try servers in order of priority, considering circuit breaker state
	for _, server := range servers {
		if sr.isServerHealthy(server.Name) {
			_, err := sr.getClientForServer(server, request)
			if err != nil {
				continue
			}

			return &RoutingDecision{
				TargetServers:   nil, // TODO: Implement proper ServerInstance creation
				RoutingStrategy: string(SingleTargetWithFallback),
				RequestContext: &RequestContext{
					FileURI:       request.URI,
					Language:      request.Context.Language,
					RequestType:   request.Method,
					WorkspaceRoot: "", // TODO: Extract from request.Context when available
					ProjectType:   "", // TODO: Extract from request.Context when available  
					WorkspaceID:   request.Context.WorkspaceID,
				},
				Priority:        server.Priority,
				CreatedAt:       time.Now(),
				DecisionID:      fmt.Sprintf("decision_%d", time.Now().UnixNano()),
			}, nil
		}
	}

	return nil, fmt.Errorf("no healthy servers available for language %s", request.Context.Language)
}

// routeLoadBalanced implements load balancing across multiple servers
func (sr *SmartRouterImpl) routeLoadBalanced(request *LSPRequest) (*RoutingDecision, error) {
	if request.Context == nil || request.Context.Language == "" {
		return nil, fmt.Errorf("language not specified for load balanced routing")
	}

	pool, lbConfig, err := sr.config.GetServerPoolWithConfig(request.Context.Language)
	if err != nil {
		// Fallback to single server routing
		return sr.routeSingleTargetWithFallback(request)
	}

	// Get healthy servers
	var healthyServers []*config.ServerConfig
	for _, server := range pool.Servers {
		if sr.isServerHealthy(server.Name) {
			healthyServers = append(healthyServers, server)
		}
	}

	if len(healthyServers) == 0 {
		return nil, fmt.Errorf("no healthy servers available for load balancing")
	}

	// Select server based on load balancing strategy
	var selectedServer *config.ServerConfig
	switch lbConfig.Strategy {
	case "round_robin":
		selectedServer = sr.selectRoundRobin(request.Context.Language, healthyServers)
	case "least_connections":
		selectedServer = sr.selectLeastConnections(healthyServers)
	case "response_time":
		selectedServer = sr.selectByResponseTime(healthyServers)
	default:
		selectedServer = healthyServers[0] // Default to first healthy server
	}

	_, err = sr.getClientForServer(selectedServer, request)
	if err != nil {
		return nil, fmt.Errorf("failed to get client for server %s: %w", selectedServer.Name, err)
	}

	return &RoutingDecision{
		TargetServers:   nil, // TODO: Implement proper ServerInstance creation
		RoutingStrategy: string(LoadBalanced),
		RequestContext: &RequestContext{
			FileURI:       request.URI,
			Language:      request.Context.Language,
			RequestType:   request.Method,
			WorkspaceRoot: "", // TODO: Extract from request.Context when available
			ProjectType:   "", // TODO: Extract from request.Context when available  
			WorkspaceID:   request.Context.WorkspaceID,
		},
		Priority:        selectedServer.Priority,
		CreatedAt:       time.Now(),
		DecisionID:      fmt.Sprintf("decision_%d", time.Now().UnixNano()),
	}, nil
}

// routePrimaryWithEnhancement routes to primary server with enhancement from secondary
func (sr *SmartRouterImpl) routePrimaryWithEnhancement(request *LSPRequest) (*RoutingDecision, error) {
	// For now, route to primary server - enhancement logic can be added later
	return sr.routeSingleTargetWithFallback(request)
}

// routeMultiTarget routes to multiple servers for parallel processing
func (sr *SmartRouterImpl) routeMultiTarget(request *LSPRequest) ([]*RoutingDecision, error) {
	if request.Context == nil || request.Context.Language == "" {
		return nil, fmt.Errorf("language not specified for multi-target routing")
	}

	servers, err := sr.config.GetServersForLanguage(request.Context.Language, 5) // Max 5 for parallel
	if err != nil {
		return nil, fmt.Errorf("no servers available for language %s: %w", request.Context.Language, err)
	}

	var decisions []*RoutingDecision
	for _, server := range servers {
		if sr.isServerHealthy(server.Name) {
			_, err := sr.getClientForServer(server, request)
			if err != nil {
				continue
			}

			decisions = append(decisions, &RoutingDecision{
				TargetServers:   nil, // TODO: Implement proper ServerInstance creation
				RoutingStrategy: string(MultiTargetParallel),
				RequestContext: &RequestContext{
					FileURI:       request.URI,
					Language:      request.Context.Language,
					RequestType:   request.Method,
					WorkspaceRoot: "", // TODO: Extract from request.Context when available
					ProjectType:   "", // TODO: Extract from request.Context when available  
					WorkspaceID:   request.Context.WorkspaceID,
				},
				Priority:        server.Priority,
				CreatedAt:       time.Now(),
				DecisionID:      fmt.Sprintf("decision_%d", time.Now().UnixNano()),
			})
		}
	}

	if len(decisions) == 0 {
		return nil, fmt.Errorf("no healthy servers available for multi-target routing")
	}

	// Sort by priority (higher priority first)
	sort.Slice(decisions, func(i, j int) bool {
		return decisions[i].Priority > decisions[j].Priority
	})

	return decisions, nil
}

// routeTraditional falls back to traditional routing via ProjectAwareRouter
func (sr *SmartRouterImpl) routeTraditional(request *LSPRequest) (*RoutingDecision, error) {
	var serverName string
	var err error

	if request.URI != "" {
		serverName, err = sr.RouteRequestWithWorkspace(request.URI)
	} else if request.Context != nil && request.Context.Language != "" {
		var exists bool
		serverName, exists = sr.GetServerByLanguage(request.Context.Language)
		if !exists {
			err = fmt.Errorf("no server found for language %s", request.Context.Language)
		}
	} else {
		return nil, fmt.Errorf("insufficient information for traditional routing")
	}

	if err != nil {
		return nil, err
	}

	// Get server config
	var serverConfig *config.ServerConfig
	for _, server := range sr.config.Servers {
		if server.Name == serverName {
			serverConfig = &server
			break
		}
	}

	if serverConfig == nil {
		return nil, fmt.Errorf("server configuration not found for %s", serverName)
	}

	_, err = sr.getClientForServer(serverConfig, request)
	if err != nil {
		return nil, fmt.Errorf("failed to get client for server %s: %w", serverName, err)
	}
	return &RoutingDecision{
		TargetServers: nil, // TODO: Implement proper ServerInstance creation
		RoutingStrategy: string(SingleTargetWithFallback),
		RequestContext: &RequestContext{
			FileURI:       request.URI,
			Language:      request.Context.Language,
			RequestType:   request.Method,
			WorkspaceRoot: "", // TODO: Extract from request.Context when available
			ProjectType:   "", // TODO: Extract from request.Context when available  
			WorkspaceID:   request.Context.WorkspaceID,
		},
		Priority:        serverConfig.Priority,
		CreatedAt:       time.Now(),
		DecisionID:      fmt.Sprintf("decision_%d", time.Now().UnixNano()),
	}, nil
}

// Helper methods for server selection and health management

// isServerHealthy checks if a server is healthy and not circuit broken
func (sr *SmartRouterImpl) isServerHealthy(serverName string) bool {
	sr.cbMu.RLock()
	defer sr.cbMu.RUnlock()

	cb, exists := sr.circuitBreakers[serverName]
	if !exists {
		return true // No circuit breaker means healthy
	}

	cb.mu.RLock()
	defer cb.mu.RUnlock()
	if cb.state == CircuitBreakerStateOpen {
		// Check if timeout has passed for half-open state
		if time.Since(cb.lastFailureTime) > cb.config.TimeoutDuration {
			cb.state = CircuitBreakerStateHalfOpen
			return true
		}
		return false
	}

	return true
}

// updateCircuitBreaker updates circuit breaker state based on request result
func (sr *SmartRouterImpl) updateCircuitBreaker(serverName string, success bool) {
	sr.cbMu.Lock()
	defer sr.cbMu.Unlock()

	cb, exists := sr.circuitBreakers[serverName]
	if !exists {
		config := &CircuitBreakerConfig{
			ErrorThreshold:      5,
			TimeoutDuration:     30 * time.Second,
			MaxHalfOpenRequests: 3,
			SuccessThreshold:    2,
			MinRequestsToTrip:   5,
		}
		cb = NewCircuitBreakerWithConfig(config)
		sr.circuitBreakers[serverName] = cb
	}

	if success {
		cb.RecordSuccess()
	} else {
		cb.RecordFailure()
	}

	// Update server metrics
	if sm := sr.metrics.ServerMetrics[serverName]; sm != nil {
		sm.CircuitBreakerState = cb.GetState().String()
	}
}

// getServerHealthScore returns the health score for a server
func (sr *SmartRouterImpl) getServerHealthScore(serverName string) float64 {
	sr.metrics.mu.RLock()
	defer sr.metrics.mu.RUnlock()

	if sm := sr.metrics.ServerMetrics[serverName]; sm != nil {
		return sm.HealthScore
	}
	return 1.0 // Default healthy score
}

// selectRoundRobin selects server using round-robin algorithm
func (sr *SmartRouterImpl) selectRoundRobin(language string, servers []*config.ServerConfig) *config.ServerConfig {
	sr.rrMu.Lock()
	defer sr.rrMu.Unlock()

	count := sr.roundRobinCounters[language]
	selected := servers[count%len(servers)]
	sr.roundRobinCounters[language] = count + 1

	return selected
}

// selectLeastConnections selects server with least active connections
func (sr *SmartRouterImpl) selectLeastConnections(servers []*config.ServerConfig) *config.ServerConfig {
	sr.metrics.mu.RLock()
	defer sr.metrics.mu.RUnlock()

	var selected *config.ServerConfig
	minConnections := int64(^uint64(0) >> 1) // Max int64

	for _, server := range servers {
		if sm := sr.metrics.ServerMetrics[server.Name]; sm != nil {
			activeConnections := sm.TotalRequests - sm.SuccessfulRequests - sm.FailedRequests
			if activeConnections < minConnections {
				minConnections = activeConnections
				selected = server
			}
		} else {
			// No metrics means no active connections
			return server
		}
	}

	if selected == nil {
		return servers[0] // Fallback to first server
	}

	return selected
}

// selectByResponseTime selects server with best average response time
func (sr *SmartRouterImpl) selectByResponseTime(servers []*config.ServerConfig) *config.ServerConfig {
	sr.metrics.mu.RLock()
	defer sr.metrics.mu.RUnlock()

	var selected *config.ServerConfig
	bestResponseTime := time.Duration(^uint64(0) >> 1) // Max duration

	for _, server := range servers {
		if sm := sr.metrics.ServerMetrics[server.Name]; sm != nil {
			if sm.AverageResponseTime < bestResponseTime {
				bestResponseTime = sm.AverageResponseTime
				selected = server
			}
		} else {
			// No metrics means potentially fastest
			return server
		}
	}

	if selected == nil {
		return servers[0] // Fallback to first server
	}

	return selected
}

// getClientForServer gets or creates an LSP client for the specified server
func (sr *SmartRouterImpl) getClientForServer(server *config.ServerConfig, request *LSPRequest) (transport.LSPClient, error) {
	// If we have workspace context, use workspace manager
	if request.Context != nil && request.Context.WorkspaceID != "" && sr.workspaceManager != nil {
		return sr.workspaceManager.GetLSPClient(request.Context.WorkspaceID, server.Name)
	}

	// Use project-aware router's client selection
	if sr.ProjectAwareRouter != nil {
		// Try to get client via server language mapping
		if request.Context != nil && request.Context.Language != "" {
			if serverName, exists := sr.GetServerByLanguage(request.Context.Language); exists {
				if serverName == server.Name {
					// Create client for the matching server
					clientConfig := transport.ClientConfig{
						Command:   server.Command,
						Args:      server.Args,
						Transport: server.Transport,
					}
					return transport.NewLSPClient(clientConfig)
				}
			}
		}
		// Fallback to creating client directly
		clientConfig := transport.ClientConfig{
			Command:   server.Command,
			Args:      server.Args,
			Transport: server.Transport,
		}
		return transport.NewLSPClient(clientConfig)
	}

	return nil, fmt.Errorf("no client available for server %s", server.Name)
}

// executeRequest executes an LSP request on the given client
func (sr *SmartRouterImpl) executeRequest(client transport.LSPClient, request *LSPRequest) (interface{}, error) {
	if client == nil {
		return nil, fmt.Errorf("nil client provided for request execution")
	}
	return client.SendRequest(context.Background(), request.Method, request.Params)
}

// extractLanguageFromURI extracts language from file URI
func (sr *SmartRouterImpl) extractLanguageFromURI(fileURI string) (string, error) {
	if !strings.HasPrefix(fileURI, "file://") {
		return "", fmt.Errorf("invalid file URI: %s", fileURI)
	}

	path := strings.TrimPrefix(fileURI, "file://")
	ext := filepath.Ext(path)

	// Map common extensions to languages
	extToLang := map[string]string{
		".go":   "go",
		".py":   "python",
		".js":   "javascript",
		".ts":   "typescript",
		".jsx":  "javascript",
		".tsx":  "typescript",
		".java": "java",
		".c":    "c",
		".cpp":  "cpp",
		".h":    "c",
		".hpp":  "cpp",
		".rs":   "rust",
		".rb":   "ruby",
		".php":  "php",
	}

	if lang, exists := extToLang[ext]; exists {
		return lang, nil
	}

	return "", fmt.Errorf("unable to determine language from URI: %s", fileURI)
}

// updateStrategyMetrics updates performance metrics for routing strategies
func (sr *SmartRouterImpl) updateStrategyMetrics(strategy string, responseTime time.Duration, success bool) {
	sr.metrics.mu.Lock()
	defer sr.metrics.mu.Unlock()

	if sr.metrics.StrategyMetrics[strategy] == nil {
		sr.metrics.StrategyMetrics[strategy] = &StrategyMetrics{}
	}

	sm := sr.metrics.StrategyMetrics[strategy]
	sm.RequestCount++

	if success {
		sm.SuccessCount++
	}

	// Update average response time
	if sm.RequestCount > 0 {
		sm.AverageResponseTime = time.Duration(
			(int64(sm.AverageResponseTime)*(sm.RequestCount-1) + int64(responseTime)) / sm.RequestCount,
		)
	}

	// Calculate success rate
	sm.SuccessRate = float64(sm.SuccessCount) / float64(sm.RequestCount)
}
