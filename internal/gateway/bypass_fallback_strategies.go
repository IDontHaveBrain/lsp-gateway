package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"lsp-gateway/internal/indexing"
	"lsp-gateway/mcp"
)

// FallbackStrategy defines the interface for handling bypassed server requests
type FallbackStrategy interface {
	// Handle processes a request when the target server is bypassed
	Handle(ctx context.Context, request *LSPRequest, bypassInfo *BypassStateEntry) (*FallbackResponse, error)
	
	// GetStrategyName returns the name of this fallback strategy
	GetStrategyName() string
	
	// IsApplicable checks if this strategy can handle the given request/bypass scenario
	IsApplicable(request *LSPRequest, bypassInfo *BypassStateEntry) bool
}

// FallbackResponse represents the response from a fallback strategy
type FallbackResponse struct {
	Result          interface{} `json:"result"`
	Success         bool        `json:"success"`
	Source          string      `json:"source"`          // e.g., "scip_cache", "alternative_server", "degraded"
	Confidence      float64     `json:"confidence"`      // 0.0 to 1.0
	Warning         string      `json:"warning,omitempty"`
	StrategyUsed    string      `json:"strategy_used"`
	ProcessingTime  time.Duration `json:"processing_time"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// FallbackStrategyManager manages different fallback strategies
type FallbackStrategyManager struct {
	strategies      []FallbackStrategy
	scipStore       indexing.SCIPStore
	logger          *mcp.StructuredLogger
	smartRouter     *SmartRouterImpl
	bypassManager   *BypassStateManager
	fallbackMetrics *FallbackMetrics
}

// FallbackMetrics tracks fallback strategy performance
type FallbackMetrics struct {
	TotalFallbacks       int64                      `json:"total_fallbacks"`
	SuccessfulFallbacks  int64                      `json:"successful_fallbacks"`
	FailedFallbacks      int64                      `json:"failed_fallbacks"`
	StrategyUsageCount   map[string]int64           `json:"strategy_usage_count"`
	AverageResponseTime  map[string]time.Duration   `json:"average_response_time"`
	SuccessRateByStrategy map[string]float64        `json:"success_rate_by_strategy"`
	LastUpdated          time.Time                  `json:"last_updated"`
}

// NewFallbackStrategyManager creates a new fallback strategy manager
func NewFallbackStrategyManager(scipStore indexing.SCIPStore, logger *mcp.StructuredLogger, smartRouter *SmartRouterImpl, bypassManager *BypassStateManager) *FallbackStrategyManager {
	fsm := &FallbackStrategyManager{
		strategies:    make([]FallbackStrategy, 0),
		scipStore:     scipStore,
		logger:        logger,
		smartRouter:   smartRouter,
		bypassManager: bypassManager,
		fallbackMetrics: &FallbackMetrics{
			StrategyUsageCount:    make(map[string]int64),
			AverageResponseTime:   make(map[string]time.Duration),
			SuccessRateByStrategy: make(map[string]float64),
			LastUpdated:           time.Now(),
		},
	}

	// Register default fallback strategies
	fsm.registerDefaultStrategies()

	return fsm
}

// registerDefaultStrategies registers the built-in fallback strategies
func (fsm *FallbackStrategyManager) registerDefaultStrategies() {
	// Register strategies in order of preference
	fsm.AddStrategy(&SCIPCacheFallbackStrategy{
		scipStore: fsm.scipStore,
		logger:    fsm.logger,
	})
	
	fsm.AddStrategy(&AlternativeServerFallbackStrategy{
		smartRouter: fsm.smartRouter,
		logger:      fsm.logger,
	})
	
	fsm.AddStrategy(&DegradedServiceFallbackStrategy{
		logger: fsm.logger,
	})
	
	fsm.AddStrategy(&DisableOnlyFallbackStrategy{
		logger: fsm.logger,
	})
}

// AddStrategy adds a fallback strategy to the manager
func (fsm *FallbackStrategyManager) AddStrategy(strategy FallbackStrategy) {
	fsm.strategies = append(fsm.strategies, strategy)
	
	// Initialize metrics for this strategy
	name := strategy.GetStrategyName()
	fsm.fallbackMetrics.StrategyUsageCount[name] = 0
	fsm.fallbackMetrics.AverageResponseTime[name] = 0
	fsm.fallbackMetrics.SuccessRateByStrategy[name] = 0.0
}

// HandleBypassedRequest processes a request when the target server is bypassed
func (fsm *FallbackStrategyManager) HandleBypassedRequest(ctx context.Context, request *LSPRequest, bypassInfo *BypassStateEntry) (*FallbackResponse, error) {
	start := time.Now()
	
	if fsm.logger != nil {
		fsm.logger.WithFields(map[string]interface{}{
			"method":        request.Method,
			"uri":          request.URI,
			"bypassed_server": bypassInfo.ServerName,
			"bypass_reason": bypassInfo.BypassReason,
		}).Info("Handling bypassed server request")
	}

	// Try strategies in order until one succeeds
	for _, strategy := range fsm.strategies {
		if !strategy.IsApplicable(request, bypassInfo) {
			continue
		}

		strategyStart := time.Now()
		response, err := strategy.Handle(ctx, request, bypassInfo)
		strategyDuration := time.Since(strategyStart)

		// Update metrics
		fsm.updateStrategyMetrics(strategy.GetStrategyName(), strategyDuration, err == nil)

		if err == nil && response.Success {
			response.ProcessingTime = time.Since(start)
			fsm.fallbackMetrics.TotalFallbacks++
			fsm.fallbackMetrics.SuccessfulFallbacks++
			fsm.fallbackMetrics.LastUpdated = time.Now()

			if fsm.logger != nil {
				fsm.logger.WithFields(map[string]interface{}{
					"strategy":        strategy.GetStrategyName(),
					"confidence":      response.Confidence,
					"processing_time": response.ProcessingTime,
				}).Info("Fallback strategy succeeded")
			}

			return response, nil
		}

		if fsm.logger != nil {
			fsm.logger.WithFields(map[string]interface{}{
				"strategy": strategy.GetStrategyName(),
				"error":    err,
			}).Debug("Fallback strategy failed, trying next")
		}
	}

	// All strategies failed
	fsm.fallbackMetrics.TotalFallbacks++
	fsm.fallbackMetrics.FailedFallbacks++
	fsm.fallbackMetrics.LastUpdated = time.Now()

	return &FallbackResponse{
		Success:      false,
		Source:       "none",
		StrategyUsed: "none",
		Warning:      "All fallback strategies failed",
		ProcessingTime: time.Since(start),
	}, fmt.Errorf("all fallback strategies failed for bypassed server %s", bypassInfo.ServerName)
}

// updateStrategyMetrics updates performance metrics for a strategy
func (fsm *FallbackStrategyManager) updateStrategyMetrics(strategyName string, duration time.Duration, success bool) {
	fsm.fallbackMetrics.StrategyUsageCount[strategyName]++
	
	// Update average response time
	currentAvg := fsm.fallbackMetrics.AverageResponseTime[strategyName]
	count := fsm.fallbackMetrics.StrategyUsageCount[strategyName]
	newAvg := time.Duration(int64(currentAvg)*int64(count-1)+int64(duration)) / time.Duration(count)
	fsm.fallbackMetrics.AverageResponseTime[strategyName] = newAvg

	// Update success rate
	successCount := int64(fsm.fallbackMetrics.SuccessRateByStrategy[strategyName] * float64(count-1))
	if success {
		successCount++
	}
	fsm.fallbackMetrics.SuccessRateByStrategy[strategyName] = float64(successCount) / float64(count)
}

// GetFallbackMetrics returns current fallback metrics
func (fsm *FallbackStrategyManager) GetFallbackMetrics() *FallbackMetrics {
	metrics := *fsm.fallbackMetrics // Copy struct
	
	// Copy maps to avoid race conditions
	metrics.StrategyUsageCount = make(map[string]int64)
	metrics.AverageResponseTime = make(map[string]time.Duration)
	metrics.SuccessRateByStrategy = make(map[string]float64)
	
	for k, v := range fsm.fallbackMetrics.StrategyUsageCount {
		metrics.StrategyUsageCount[k] = v
	}
	for k, v := range fsm.fallbackMetrics.AverageResponseTime {
		metrics.AverageResponseTime[k] = v
	}
	for k, v := range fsm.fallbackMetrics.SuccessRateByStrategy {
		metrics.SuccessRateByStrategy[k] = v
	}
	
	return &metrics
}

// SCIP Cache Fallback Strategy
type SCIPCacheFallbackStrategy struct {
	scipStore indexing.SCIPStore
	logger    *mcp.StructuredLogger
}

func (s *SCIPCacheFallbackStrategy) GetStrategyName() string {
	return "scip_cache"
}

func (s *SCIPCacheFallbackStrategy) IsApplicable(request *LSPRequest, bypassInfo *BypassStateEntry) bool {
	if s.scipStore == nil {
		return false
	}

	// SCIP cache is good for symbol-based queries
	symbolMethods := map[string]bool{
		"textDocument/definition":    true,
		"textDocument/references":    true,
		"textDocument/documentSymbol": true,
		"workspace/symbol":          true,
		"textDocument/hover":        true,
	}

	return symbolMethods[request.Method]
}

func (s *SCIPCacheFallbackStrategy) Handle(ctx context.Context, request *LSPRequest, bypassInfo *BypassStateEntry) (*FallbackResponse, error) {
	// Try to get result from SCIP cache
	// This would integrate with the existing SCIP store implementation
	
	// For now, return a placeholder implementation
	return &FallbackResponse{
		Result:      json.RawMessage(`{"message": "SCIP cache result not implemented yet"}`),
		Success:     false, // Set to false until actually implemented
		Source:      "scip_cache",
		Confidence:  0.7,
		StrategyUsed: s.GetStrategyName(),
		Warning:     "SCIP cache fallback not fully implemented",
	}, nil
}

// Alternative Server Fallback Strategy
type AlternativeServerFallbackStrategy struct {
	smartRouter *SmartRouterImpl
	logger      *mcp.StructuredLogger
}

func (s *AlternativeServerFallbackStrategy) GetStrategyName() string {
	return "alternative_server"
}

func (s *AlternativeServerFallbackStrategy) IsApplicable(request *LSPRequest, bypassInfo *BypassStateEntry) bool {
	if s.smartRouter == nil || request.Context == nil {
		return false
	}

	// Check if there are other non-bypassed servers for this language
	nonBypassed := s.smartRouter.GetNonBypassedServersForLanguage(request.Context.Language)
	return len(nonBypassed) > 0
}

func (s *AlternativeServerFallbackStrategy) Handle(ctx context.Context, request *LSPRequest, bypassInfo *BypassStateEntry) (*FallbackResponse, error) {
	// Try to route to an alternative server
	decision, err := s.smartRouter.RouteRequest(request)
	if err != nil {
		return &FallbackResponse{
			Success:      false,
			Source:       "alternative_server",
			StrategyUsed: s.GetStrategyName(),
			Warning:      fmt.Sprintf("Failed to route to alternative server: %v", err),
		}, err
	}

	return &FallbackResponse{
		Result:       decision, // This would be the actual LSP response in real implementation
		Success:      true,
		Source:       "alternative_server",
		Confidence:   0.9,
		StrategyUsed: s.GetStrategyName(),
		Metadata: map[string]interface{}{
			"routing_decision": decision,
		},
	}, nil
}

// Degraded Service Fallback Strategy
type DegradedServiceFallbackStrategy struct {
	logger *mcp.StructuredLogger
}

func (s *DegradedServiceFallbackStrategy) GetStrategyName() string {
	return "degraded_service"
}

func (s *DegradedServiceFallbackStrategy) IsApplicable(request *LSPRequest, bypassInfo *BypassStateEntry) bool {
	// This strategy can always provide some basic response
	return true
}

func (s *DegradedServiceFallbackStrategy) Handle(ctx context.Context, request *LSPRequest, bypassInfo *BypassStateEntry) (*FallbackResponse, error) {
	// Provide minimal/empty response to keep client functional
	var result interface{}
	
	switch request.Method {
	case "textDocument/definition":
		result = []interface{}{}
	case "textDocument/references":
		result = []interface{}{}
	case "textDocument/documentSymbol":
		result = []interface{}{}
	case "workspace/symbol":
		result = []interface{}{}
	case "textDocument/hover":
		result = nil
	default:
		result = nil
	}

	return &FallbackResponse{
		Result:       result,
		Success:      true,
		Source:       "degraded_service",
		Confidence:   0.1,
		StrategyUsed: s.GetStrategyName(),
		Warning:      fmt.Sprintf("Server %s is bypassed, providing minimal response", bypassInfo.ServerName),
	}, nil
}

// Disable Only Fallback Strategy
type DisableOnlyFallbackStrategy struct {
	logger *mcp.StructuredLogger
}

func (s *DisableOnlyFallbackStrategy) GetStrategyName() string {
	return "disable_only"
}

func (s *DisableOnlyFallbackStrategy) IsApplicable(request *LSPRequest, bypassInfo *BypassStateEntry) bool {
	// This is the last resort strategy
	return true
}

func (s *DisableOnlyFallbackStrategy) Handle(ctx context.Context, request *LSPRequest, bypassInfo *BypassStateEntry) (*FallbackResponse, error) {
	return &FallbackResponse{
		Success:      false,
		Source:       "disabled",
		StrategyUsed: s.GetStrategyName(),
		Warning:      fmt.Sprintf("Server %s is bypassed and no fallback available", bypassInfo.ServerName),
	}, fmt.Errorf("server %s is bypassed and fallback strategy is disable_only", bypassInfo.ServerName)
}