package gateway

import (
	"context"
	"fmt"
	"sync"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/mcp"
)

// SmartRouterWithAggregation extends SmartRouter with advanced response aggregation
type SmartRouterWithAggregation struct {
	*SmartRouterImpl
	aggregatorRegistry *AggregatorRegistry
	aggregationEnabled bool
	mu                 sync.RWMutex
}

// NewSmartRouterWithAggregation creates a new SmartRouter with aggregation capabilities
func NewSmartRouterWithAggregation(projectRouter *ProjectAwareRouter, config *config.GatewayConfig, workspaceManager *WorkspaceManager, logger *mcp.StructuredLogger) *SmartRouterWithAggregation {
	baseRouter := NewSmartRouter(projectRouter, config, workspaceManager, logger)
	
	return &SmartRouterWithAggregation{
		SmartRouterImpl:    baseRouter,
		aggregatorRegistry: NewAggregatorRegistry(logger),
		aggregationEnabled: true,
	}
}

// AggregateBroadcastEnhanced sends requests to multiple servers and uses advanced aggregation
func (sr *SmartRouterWithAggregation) AggregateBroadcastEnhanced(request *LSPRequest) (*EnhancedAggregatedResponse, error) {
	startTime := time.Now()
	
	if sr.logger != nil {
		sr.logger.Debugf("SmartRouterWithAggregation: Enhanced broadcasting request for method %s", request.Method)
	}
	
	decisions, err := sr.RouteMultiRequest(request)
	if err != nil {
		return nil, fmt.Errorf("failed to route multi-request: %w", err)
	}
	
	if len(decisions) == 0 {
		return nil, fmt.Errorf("no servers available for broadcast")
	}
	
	// Execute requests in parallel
	responses := make([]interface{}, len(decisions))
	sources := make([]string, len(decisions))
	serverResponses := make([]ServerResponse, len(decisions))
	var wg sync.WaitGroup
	
	for i, decision := range decisions {
		wg.Add(1)
		go func(idx int, dec *RoutingDecision) {
			defer wg.Done()
			
			// Safe access to TargetServers
			if len(dec.TargetServers) == 0 {
				return
			}
			
			reqStart := time.Now()
			result, err := sr.executeRequest(dec.TargetServers[0].client, request)
			responseTime := time.Since(reqStart)
			
			responses[idx] = result
			sources[idx] = dec.TargetServers[0].config.Name
			serverResponses[idx] = ServerResponse{
				ServerName:   dec.TargetServers[0].config.Name,
				Response:     result,
				Error:        err,
				Duration:     responseTime,
				Success:      err == nil,
			}
			
			// Update server performance metrics
			sr.UpdateServerPerformance(dec.TargetServers[0].config.Name, responseTime, err == nil)
		}(i, decision)
	}
	
	wg.Wait()
	
	// Use aggregator registry to merge responses
	var aggregationResult *AggregationResult
	if sr.isAggregationEnabled() {
		aggregationResult, err = sr.aggregatorRegistry.AggregateResponses(request.Method, responses, sources)
		if err != nil {
			// Fallback to basic aggregation
			sr.logger.Warnf("Enhanced aggregation failed, falling back to basic: %v", err)
			aggregationResult = sr.createFallbackAggregationResult(responses, sources, time.Since(startTime))
		}
	} else {
		aggregationResult = sr.createFallbackAggregationResult(responses, sources, time.Since(startTime))
	}
	
	// Create enhanced response
	enhancedResponse := &EnhancedAggregatedResponse{
		AggregatedResponse: AggregatedResponse{
			PrimaryResponse:    aggregationResult.MergedResponse,
			SecondaryResponses: sr.convertServerResponsesToInterfaces(sr.filterSecondaryResults(serverResponses, aggregationResult.MergedResponse)),
			AggregationMethod: sr.GetRoutingStrategy(request.Method).Name(),
			ProcessingTime:   time.Since(startTime),
			SuccessCount:      len(decisions),
		},
		AggregationResult: *aggregationResult,
		QualityMetrics:    sr.calculateEnhancedQualityMetrics(aggregationResult, serverResponses),
	}
	
	// Update strategy metrics
	sr.updateStrategyMetrics(string(sr.GetRoutingStrategy(request.Method)), time.Since(startTime), aggregationResult.SuccessfulSources > 0)
	
	if aggregationResult.SuccessfulSources == 0 {
		return enhancedResponse, fmt.Errorf("all servers failed to process request")
	}
	
	return enhancedResponse, nil
}

// EnhancedAggregatedResponse extends AggregatedResponse with aggregation details
type EnhancedAggregatedResponse struct {
	AggregatedResponse          // Embedded for backward compatibility
	AggregationResult           `json:"aggregation_result"`
	QualityMetrics    EnhancedQualityMetrics `json:"quality_metrics"`
}

// EnhancedQualityMetrics provides detailed quality information
type EnhancedQualityMetrics struct {
	AggregationQuality               // Embedded base quality
	ServerReliability  map[string]float64 `json:"server_reliability"`
	MethodSupport      map[string]bool    `json:"method_support"`
	ResponseSizes      map[string]int     `json:"response_sizes"`
	ConflictResolution []ConflictResolutionDetail `json:"conflict_resolution,omitempty"`
}

// ConflictResolutionDetail provides details about conflict resolution
type ConflictResolutionDetail struct {
	ConflictType   string    `json:"conflict_type"`
	ServersInvolved []string `json:"servers_involved"`
	ResolutionStrategy string `json:"resolution_strategy"`
	ConfidenceScore float64  `json:"confidence_score"`
	Timestamp      time.Time `json:"timestamp"`
}

// RouteWithIntelligentAggregation routes requests with intelligent aggregation strategy selection
func (sr *SmartRouterWithAggregation) RouteWithIntelligentAggregation(request *LSPRequest) (*EnhancedAggregatedResponse, error) {
	// Determine if aggregation would be beneficial for this request
	strategy := sr.GetRoutingStrategy(request.Method)
	
	switch strategy {
	case BroadcastAggregate, MultiTargetParallel:
		return sr.AggregateBroadcastEnhanced(request)
	
	case PrimaryWithEnhancement:
		return sr.routeWithEnhancement(request)
	
	default:
		// Single target routing with optional aggregation for certain methods
		if sr.shouldUseAggregationForSingleTarget(request.Method) {
			return sr.routeSingleWithAggregationFallback(request)
		}
		
		// Standard single-target routing
		decision, err := sr.RouteRequest(request)
		if err != nil {
			return nil, err
		}
		
		// Safe access to TargetServers
		if len(decision.TargetServers) == 0 {
			return nil, fmt.Errorf("no target servers available")
		}
		
		result, err := sr.executeRequest(decision.TargetServers[0].client, request)
		if err != nil {
			return nil, err
		}
		
		return sr.createSingleTargetEnhancedResponse(result, decision), nil
	}
}

// routeWithEnhancement implements primary with enhancement strategy
func (sr *SmartRouterWithAggregation) routeWithEnhancement(request *LSPRequest) (*EnhancedAggregatedResponse, error) {
	startTime := time.Now()
	
	decisions, err := sr.RouteMultiRequest(request)
	if err != nil {
		return nil, err
	}
	
	if len(decisions) == 0 {
		return nil, fmt.Errorf("no servers available for enhancement routing")
	}
	
	// Execute primary server first
	primaryDecision := decisions[0]
	
	// Safe access to TargetServers
	if len(primaryDecision.TargetServers) == 0 {
		return nil, fmt.Errorf("no target servers available for primary decision")
	}
	
	primaryResult, primaryErr := sr.executeRequest(primaryDecision.TargetServers[0].client, request)
	
	// If primary succeeds and we have enhancement servers, query them
	var enhancementResults []interface{}
	var enhancementSources []string
	
	if primaryErr == nil && len(decisions) > 1 {
		var wg sync.WaitGroup
		enhancementResults = make([]interface{}, len(decisions)-1)
		enhancementSources = make([]string, len(decisions)-1)
		
		for i, decision := range decisions[1:] {
			wg.Add(1)
			go func(idx int, dec *RoutingDecision) {
				defer wg.Done()
				
				// Safe access to TargetServers
				if len(dec.TargetServers) == 0 {
					return
				}
				
				result, err := sr.executeRequest(dec.TargetServers[0].client, request)
				if err == nil {
					enhancementResults[idx] = result
					enhancementSources[idx] = dec.TargetServers[0].config.Name
				}
			}(i, decision)
		}
		
		wg.Wait()
	}
	
	// Aggregate primary with enhancements
	allResults := []interface{}{primaryResult}
	allSources := []string{primaryDecision.TargetServers[0].config.Name}
	
	for i, result := range enhancementResults {
		if result != nil {
			allResults = append(allResults, result)
			allSources = append(allSources, enhancementSources[i])
		}
	}
	
	aggregationResult, err := sr.aggregatorRegistry.AggregateResponses(request.Method, allResults, allSources)
	if err != nil {
		// Return primary result if aggregation fails
		aggregationResult = &AggregationResult{
			MergedResponse:    primaryResult,
			SourceMapping:     map[string]interface{}{primaryDecision.TargetServers[0].config.Name: primaryResult},
			MergeStrategy:     "primary_only",
			TotalSources:      1,
			SuccessfulSources: 1,
			ProcessingTime:    time.Since(startTime),
			Quality: AggregationQuality{
				Score:        0.7,
				Completeness: 1.0,
			},
		}
	}
	
	return &EnhancedAggregatedResponse{
		AggregatedResponse: AggregatedResponse{
			PrimaryResponse:   aggregationResult.MergedResponse,
			AggregationMethod: PrimaryWithEnhancement.Name(),
			ProcessingTime:  time.Since(startTime),
			SuccessCount:     len(allResults),
		},
		AggregationResult: *aggregationResult,
		QualityMetrics: EnhancedQualityMetrics{
			AggregationQuality: aggregationResult.Quality,
			ServerReliability:  sr.getServerReliabilityScores(allSources),
		},
	}, nil
}

// routeSingleWithAggregationFallback routes to single target with aggregation fallback
func (sr *SmartRouterWithAggregation) routeSingleWithAggregationFallback(request *LSPRequest) (*EnhancedAggregatedResponse, error) {
	decision, err := sr.RouteRequest(request)
	if err != nil {
		return nil, err
	}
	
	// Safe access to TargetServers
	if len(decision.TargetServers) == 0 {
		return nil, fmt.Errorf("no target servers available")
	}
	
	result, err := sr.executeRequest(decision.TargetServers[0].client, request)
	if err != nil {
		// Try aggregation with multiple servers as fallback
		return sr.AggregateBroadcastEnhanced(request)
	}
	
	return sr.createSingleTargetEnhancedResponse(result, decision), nil
}

// Helper methods

// isAggregationEnabled checks if aggregation is enabled
func (sr *SmartRouterWithAggregation) isAggregationEnabled() bool {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return sr.aggregationEnabled
}

// SetAggregationEnabled enables or disables aggregation
func (sr *SmartRouterWithAggregation) SetAggregationEnabled(enabled bool) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.aggregationEnabled = enabled
	
	if sr.logger != nil {
		sr.logger.Infof("Response aggregation %s", map[bool]string{true: "enabled", false: "disabled"}[enabled])
	}
}

// shouldUseAggregationForSingleTarget determines if single target should use aggregation fallback
func (sr *SmartRouterWithAggregation) shouldUseAggregationForSingleTarget(method string) bool {
	// Methods that benefit from aggregation fallback
	aggregationBeneficialMethods := map[string]bool{
		LSP_METHOD_DEFINITION:       true,
		LSP_METHOD_REFERENCES:       true,
		LSP_METHOD_WORKSPACE_SYMBOL: true,
		LSP_METHOD_HOVER:            false, // Usually better to have single authoritative hover
		"textDocument/completion":   false, // Completion should be context-specific
	}
	
	return aggregationBeneficialMethods[method]
}

// createFallbackAggregationResult creates a basic aggregation result when enhanced aggregation fails
func (sr *SmartRouterWithAggregation) createFallbackAggregationResult(responses []interface{}, sources []string, processingTime time.Duration) *AggregationResult {
	var primaryResult interface{}
	successfulSources := 0
	
	for _, response := range responses {
		if response != nil {
			successfulSources++
			if primaryResult == nil {
				primaryResult = response
			}
		}
	}
	
	sourceMapping := make(map[string]interface{})
	for i, source := range sources {
		if i < len(responses) {
			sourceMapping[source] = responses[i]
		}
	}
	
	return &AggregationResult{
		MergedResponse:    primaryResult,
		SourceMapping:     sourceMapping,
		MergeStrategy:     "fallback_first_success",
		TotalSources:      len(sources),
		SuccessfulSources: successfulSources,
		ProcessingTime:    processingTime,
		Quality: AggregationQuality{
			Score:             0.5,
			Completeness:      float64(successfulSources) / float64(len(sources)),
			Consistency:       1.0,
			SourceReliability: 0.7,
		},
	}
}

// filterSecondaryResults filters secondary results to exclude the primary result
func (sr *SmartRouterWithAggregation) filterSecondaryResults(serverResponses []ServerResponse, primaryResult interface{}) []ServerResponse {
	var secondaryResults []ServerResponse
	
	for _, response := range serverResponses {
		if response.Success && response.Response != primaryResult {
			secondaryResults = append(secondaryResults, response)
		}
	}
	
	return secondaryResults
}

// convertServerResponsesToInterfaces converts []ServerResponse to []interface{}
func (sr *SmartRouterWithAggregation) convertServerResponsesToInterfaces(serverResponses []ServerResponse) []interface{} {
	var interfaces []interface{}
	
	for _, response := range serverResponses {
		interfaces = append(interfaces, response.Response)
	}
	
	return interfaces
}

// calculateEnhancedQualityMetrics calculates detailed quality metrics
func (sr *SmartRouterWithAggregation) calculateEnhancedQualityMetrics(aggregationResult *AggregationResult, serverResponses []ServerResponse) EnhancedQualityMetrics {
	serverReliability := make(map[string]float64)
	methodSupport := make(map[string]bool)
	responseSizes := make(map[string]int)
	
	for _, response := range serverResponses {
		// Calculate server reliability based on success and response time
		reliability := 0.0
		if response.Success {
			reliability = 1.0
			// Adjust based on response time (faster = more reliable)
			if response.Duration < 100*time.Millisecond {
				reliability = 1.0
			} else if response.Duration < 500*time.Millisecond {
				reliability = 0.9
			} else if response.Duration < 1*time.Second {
				reliability = 0.8
			} else {
				reliability = 0.7
			}
		}
		
		serverReliability[response.ServerName] = reliability
		methodSupport[response.ServerName] = response.Success
		
		if response.Response != nil {
			responseSizes[response.ServerName] = sr.estimateResponseSize(response.Response)
		}
	}
	
	return EnhancedQualityMetrics{
		AggregationQuality: aggregationResult.Quality,
		ServerReliability:  serverReliability,
		MethodSupport:      methodSupport,
		ResponseSizes:      responseSizes,
	}
}

// getServerReliabilityScores gets reliability scores for servers
func (sr *SmartRouterWithAggregation) getServerReliabilityScores(sources []string) map[string]float64 {
	reliability := make(map[string]float64)
	
	for _, source := range sources {
		score := sr.getServerHealthScore(source)
		reliability[source] = score
	}
	
	return reliability
}

// createSingleTargetEnhancedResponse creates an enhanced response for single target routing
func (sr *SmartRouterWithAggregation) createSingleTargetEnhancedResponse(result interface{}, decision *RoutingDecision) *EnhancedAggregatedResponse {
	// Safe access to TargetServers
	if len(decision.TargetServers) == 0 {
		return &EnhancedAggregatedResponse{}
	}
	
	serverName := decision.TargetServers[0].config.Name
	aggregationResult := AggregationResult{
		MergedResponse:    result,
		SourceMapping:     map[string]interface{}{serverName: result},
		MergeStrategy:     "single_target",
		TotalSources:      1,
		SuccessfulSources: 1,
		Quality: AggregationQuality{
			Score:             1.0,
			Completeness:      1.0,
			Consistency:       1.0,
			SourceReliability: sr.getServerHealthScore(serverName),
		},
	}
	
	return &EnhancedAggregatedResponse{
		AggregatedResponse: AggregatedResponse{
			PrimaryResponse:  result,
			AggregationMethod: SingleTargetWithFallback.Name(),
			SuccessCount:    1,
		},
		AggregationResult: aggregationResult,
		QualityMetrics: EnhancedQualityMetrics{
			AggregationQuality: aggregationResult.Quality,
			ServerReliability:  map[string]float64{serverName: sr.getServerHealthScore(serverName)},
			MethodSupport:      map[string]bool{serverName: true},
		},
	}
}

// estimateResponseSize estimates the size of a response for metrics
func (sr *SmartRouterWithAggregation) estimateResponseSize(response interface{}) int {
	// Simple estimation based on response type
	switch resp := response.(type) {
	case []Location:
		return len(resp) * 100 // Rough estimate
	case []SymbolInformation:
		return len(resp) * 200
	case []Diagnostic:
		return len(resp) * 150
	case CompletionList:
		return len(resp.Items) * 100
	case string:
		return len(resp)
	default:
		return 100 // Default estimate
	}
}

// Integration Example Usage

// ExampleUsage demonstrates how to use the enhanced router with aggregation
func ExampleUsage() {
	// This is an example of how to integrate the ResponseAggregator with existing systems
	
	// 1. Create the enhanced router (this would typically be done in your main initialization)
	var projectRouter *ProjectAwareRouter // Assume this is initialized
	var config *config.GatewayConfig       // Assume this is loaded
	var workspaceManager *WorkspaceManager // Assume this is created
	var logger *mcp.StructuredLogger       // Assume this is configured
	
	enhancedRouter := NewSmartRouterWithAggregation(projectRouter, config, workspaceManager, logger)
	
	// 2. Enable or disable aggregation based on configuration
	enhancedRouter.SetAggregationEnabled(true)
	
	// 3. Use the enhanced router for requests
	request := &LSPRequest{
		Method: LSP_METHOD_DEFINITION,
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///path/to/file.go",
			},
			"position": map[string]interface{}{
				"line":      10,
				"character": 5,
			},
		},
		URI:     "file:///path/to/file.go",
		Context: &RequestContext{},
	}
	
	// 4. Route with intelligent aggregation
	response, err := enhancedRouter.RouteWithIntelligentAggregation(request)
	if err != nil {
		logger.Errorf("Request routing failed: %v", err)
		return
	}
	
	// 5. Use the enhanced response
	logger.Infof("Request completed with %d sources, quality score: %.2f", 
		response.AggregationResult.TotalSources, 
		response.QualityMetrics.Score)
	
	// 6. Access aggregation details if needed
	if len(response.ConflictInfo) > 0 {
		logger.Warnf("Found %d conflicts during aggregation", len(response.ConflictInfo))
	}
}

// CustomAggregatorExample shows how to create and register custom aggregators
func CustomAggregatorExample() {
	logger := &mcp.StructuredLogger{}
	registry := NewAggregatorRegistry(logger)
	
	// Create a custom aggregator for a specific method
	customAggregator := &CustomMethodAggregator{logger: logger}
	registry.RegisterAggregator(customAggregator)
	
	// Now the registry can handle the custom method
	responses := []interface{}{"custom response 1", "custom response 2"}
	sources := []string{"server1", "server2"}
	
	result, err := registry.AggregateResponses("custom/method", responses, sources)
	if err != nil {
		logger.Errorf("Custom aggregation failed: %v", err)
		return
	}
	
	logger.Infof("Custom aggregation completed: %s", result.MergeStrategy)
}

// CustomMethodAggregator example implementation
type CustomMethodAggregator struct {
	logger *mcp.StructuredLogger
}

func (c *CustomMethodAggregator) GetAggregationType() string {
	return "custom_method"
}

func (c *CustomMethodAggregator) SupportedMethods() []string {
	return []string{"custom/method"}
}

func (c *CustomMethodAggregator) Aggregate(responses []interface{}, sources []string) (interface{}, error) {
	// Custom aggregation logic
	var combinedResponse []string
	
	for _, response := range responses {
		if str, ok := response.(string); ok {
			combinedResponse = append(combinedResponse, str)
		}
	}
	
	return map[string]interface{}{
		"combined": combinedResponse,
		"sources":  sources,
	}, nil
}