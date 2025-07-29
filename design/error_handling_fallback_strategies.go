// Error Handling and Fallback Strategies Design for Enhanced Sub-Project Routing System
// This design builds upon existing error handling patterns in the codebase and provides
// comprehensive error handling, recovery, and fallback mechanisms for sub-project routing.

package design

import (
	"context"
	"encoding/json"
	"fmt"
	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/transport"
	"sync"
	"sync/atomic"
	"time"
)

// =====================================================================================
// ERROR TAXONOMY AND CLASSIFICATION SYSTEM
// =====================================================================================

// SubProjectErrorType categorizes different types of sub-project routing errors
type SubProjectErrorType string

const (
	// Project Resolution Errors
	ErrorTypeProjectResolution SubProjectErrorType = "project_resolution"
	ErrorTypeProjectAmbiguous  SubProjectErrorType = "project_ambiguous"
	ErrorTypeProjectInvalid    SubProjectErrorType = "project_invalid"
	ErrorTypeFileSystemAccess  SubProjectErrorType = "filesystem_access"

	// LSP Client Errors
	ErrorTypeClientStartup     SubProjectErrorType = "client_startup"
	ErrorTypeClientConnection  SubProjectErrorType = "client_connection"
	ErrorTypeClientTimeout     SubProjectErrorType = "client_timeout"
	ErrorTypeClientResourceExhaustion SubProjectErrorType = "client_resource_exhaustion"

	// Request Processing Errors
	ErrorTypeRequestMalformed  SubProjectErrorType = "request_malformed"
	ErrorTypeRequestUnsupported SubProjectErrorType = "request_unsupported"
	ErrorTypeRequestCancelled  SubProjectErrorType = "request_cancelled"
	ErrorTypeConcurrencyConflict SubProjectErrorType = "concurrency_conflict"

	// System-Level Errors
	ErrorTypeCacheCorruption   SubProjectErrorType = "cache_corruption"
	ErrorTypeConfigurationReload SubProjectErrorType = "configuration_reload"
	ErrorTypeResourceCleanup   SubProjectErrorType = "resource_cleanup"
	ErrorTypeNetworkUnavailable SubProjectErrorType = "network_unavailable"
)

// ErrorSeverity defines the severity level of errors
type ErrorSeverity int

const (
	SeverityInfo ErrorSeverity = iota
	SeverityWarning
	SeverityError
	SeverityCritical
)

// ErrorRecoveryType defines how an error should be handled
type ErrorRecoveryType int

const (
	RecoveryTypeRetry ErrorRecoveryType = iota  // Retry with backoff
	RecoveryTypeFallback                        // Use fallback strategy
	RecoveryTypeDegrade                         // Graceful degradation
	RecoveryTypeFail                            // Critical failure, stop operation
)

// =====================================================================================
// COMPREHENSIVE ERROR STRUCTURES
// =====================================================================================

// SubProjectRoutingError represents a comprehensive error in the sub-project routing system
type SubProjectRoutingError struct {
	Type            SubProjectErrorType          `json:"type"`
	Severity        ErrorSeverity                `json:"severity"`
	RecoveryType    ErrorRecoveryType            `json:"recovery_type"`
	Message         string                       `json:"message"`
	Context         *ErrorContext                `json:"context"`
	Metadata        map[string]interface{}       `json:"metadata"`
	Suggestions     []string                     `json:"suggestions"`
	Cause           error                        `json:"-"`
	Timestamp       time.Time                    `json:"timestamp"`
	RequestID       string                       `json:"request_id,omitempty"`
	SessionID       string                       `json:"session_id,omitempty"`
}

// ErrorContext provides comprehensive context for error analysis and recovery
type ErrorContext struct {
	Operation      string                 `json:"operation"`
	SubProject     *SubProjectInfo        `json:"sub_project,omitempty"`
	FileURI        string                 `json:"file_uri,omitempty"`
	LSPMethod      string                 `json:"lsp_method,omitempty"`
	ClientType     string                 `json:"client_type,omitempty"`
	Language       string                 `json:"language,omitempty"`
	ProjectRoot    string                 `json:"project_root,omitempty"`
	WorkspaceRoot  string                 `json:"workspace_root,omitempty"`
	AttemptNumber  int                    `json:"attempt_number"`
	ElapsedTime    time.Duration          `json:"elapsed_time"`
	SystemInfo     map[string]interface{} `json:"system_info,omitempty"`
}

// SubProjectInfo represents minimal information about a sub-project for error context
type SubProjectInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Root         string `json:"root"`
	Language     string `json:"language"`
	ConfigValid  bool   `json:"config_valid"`
}

// =====================================================================================
// FALLBACK STRATEGY INTERFACES AND IMPLEMENTATIONS
// =====================================================================================

// FallbackStrategy defines the interface for fallback strategies
type FallbackStrategy interface {
	// CanHandle determines if this strategy can handle the given error
	CanHandle(err *SubProjectRoutingError) bool
	
	// Execute performs the fallback operation
	Execute(ctx context.Context, originalRequest *LSPRequest) (*LSPResponse, error)
	
	// GetPriority returns the priority of this strategy (higher = more preferred)
	GetPriority() int
	
	// GetName returns a human-readable name for this strategy
	GetName() string
	
	// GetDescription returns a description of what this strategy does
	GetDescription() string
	
	// GetEstimatedRecoveryTime provides an estimate of how long recovery might take
	GetEstimatedRecoveryTime() time.Duration
}

// LSPRequest represents a generic LSP request for fallback processing
type LSPRequest struct {
	Method    string                 `json:"method"`
	Params    interface{}            `json:"params"`
	ID        interface{}            `json:"id"`
	Context   *ErrorContext          `json:"context"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// LSPResponse represents a generic LSP response
type LSPResponse struct {
	Result    interface{}            `json:"result,omitempty"`
	Error     *transport.RPCError    `json:"error,omitempty"`
	ID        interface{}            `json:"id"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Source    string                 `json:"source,omitempty"` // Indicates fallback source
}

// =====================================================================================
// CONCRETE FALLBACK STRATEGY IMPLEMENTATIONS
// =====================================================================================

// WorkspaceRootFallbackStrategy falls back to workspace-level client for unresolvable files
type WorkspaceRootFallbackStrategy struct {
	workspaceClient transport.LSPClient
	logger          Logger
	priority        int
}

func (w *WorkspaceRootFallbackStrategy) CanHandle(err *SubProjectRoutingError) bool {
	return err.Type == ErrorTypeProjectResolution || 
		   err.Type == ErrorTypeProjectAmbiguous ||
		   err.Type == ErrorTypeProjectInvalid
}

func (w *WorkspaceRootFallbackStrategy) Execute(ctx context.Context, req *LSPRequest) (*LSPResponse, error) {
	w.logger.WithContext(req.Context).Info("Using workspace root fallback strategy")
	
	// Route request to workspace-level client
	result, err := w.workspaceClient.SendRequest(ctx, req.Method, req.Params)
	if err != nil {
		return &LSPResponse{
			Error: &transport.RPCError{
				Code:    -32603,
				Message: fmt.Sprintf("Workspace fallback failed: %v", err),
			},
			ID: req.ID,
		}, err
	}
	
	return &LSPResponse{
		Result:   json.RawMessage(result),
		ID:       req.ID,
		Source:   "workspace_fallback",
		Metadata: map[string]interface{}{
			"fallback_strategy": w.GetName(),
			"recovery_time":     time.Since(time.Now()),
		},
	}, nil
}

func (w *WorkspaceRootFallbackStrategy) GetPriority() int { return 100 }
func (w *WorkspaceRootFallbackStrategy) GetName() string { return "workspace_root_fallback" }
func (w *WorkspaceRootFallbackStrategy) GetDescription() string {
	return "Falls back to workspace-level LSP client when sub-project resolution fails"
}
func (w *WorkspaceRootFallbackStrategy) GetEstimatedRecoveryTime() time.Duration {
	return 100 * time.Millisecond
}

// ParentDirectoryTraversalStrategy attempts to find parent projects
type ParentDirectoryTraversalStrategy struct {
	projectDetector ProjectDetector
	logger          Logger
	priority        int
}

func (p *ParentDirectoryTraversalStrategy) CanHandle(err *SubProjectRoutingError) bool {
	return err.Type == ErrorTypeProjectResolution && err.Context.FileURI != ""
}

func (p *ParentDirectoryTraversalStrategy) Execute(ctx context.Context, req *LSPRequest) (*LSPResponse, error) {
	p.logger.WithContext(req.Context).Info("Using parent directory traversal strategy")
	
	// Implementation would traverse parent directories to find valid projects
	// This is a placeholder for the actual implementation
	
	return &LSPResponse{
		Error: &transport.RPCError{
			Code:    -32601,
			Message: "Method not implemented",
		},
		ID: req.ID,
	}, fmt.Errorf("parent directory traversal not implemented")
}

func (p *ParentDirectoryTraversalStrategy) GetPriority() int { return 90 }
func (p *ParentDirectoryTraversalStrategy) GetName() string { return "parent_directory_traversal" }
func (p *ParentDirectoryTraversalStrategy) GetDescription() string {
	return "Traverses parent directories to find containing projects"
}
func (p *ParentDirectoryTraversalStrategy) GetEstimatedRecoveryTime() time.Duration {
	return 200 * time.Millisecond
}

// CachedResponseFallbackStrategy uses cached responses for failed requests
type CachedResponseFallbackStrategy struct {
	cache       ResponseCache
	logger      Logger
	priority    int
}

func (c *CachedResponseFallbackStrategy) CanHandle(err *SubProjectRoutingError) bool {
	return err.Type == ErrorTypeClientTimeout || 
		   err.Type == ErrorTypeClientConnection ||
		   err.Type == ErrorTypeNetworkUnavailable
}

func (c *CachedResponseFallbackStrategy) Execute(ctx context.Context, req *LSPRequest) (*LSPResponse, error) {
	c.logger.WithContext(req.Context).Info("Using cached response fallback strategy")
	
	// Look up cached response
	cacheKey := c.generateCacheKey(req)
	cachedResponse, found := c.cache.Get(cacheKey)
	
	if !found {
		return &LSPResponse{
			Error: &transport.RPCError{
				Code:    -32603,
				Message: "No cached response available",
			},
			ID: req.ID,
		}, fmt.Errorf("no cached response for key: %s", cacheKey)
	}
	
	return &LSPResponse{
		Result:   cachedResponse,
		ID:       req.ID,
		Source:   "cache_fallback",
		Metadata: map[string]interface{}{
			"fallback_strategy": c.GetName(),
			"cache_age":         time.Since(time.Now()), // Placeholder
		},
	}, nil
}

func (c *CachedResponseFallbackStrategy) generateCacheKey(req *LSPRequest) string {
	// Generate a cache key based on method and relevant parameters
	return fmt.Sprintf("%s:%s", req.Method, req.Context.FileURI)
}

func (c *CachedResponseFallbackStrategy) GetPriority() int { return 80 }
func (c *CachedResponseFallbackStrategy) GetName() string { return "cached_response_fallback" }
func (c *CachedResponseFallbackStrategy) GetDescription() string {
	return "Uses cached responses when LSP clients are unavailable"
}
func (c *CachedResponseFallbackStrategy) GetEstimatedRecoveryTime() time.Duration {
	return 10 * time.Millisecond
}

// SCIPDataFallbackStrategy uses SCIP indexing data for offline responses
type SCIPDataFallbackStrategy struct {
	scipStore   SCIPStore
	logger      Logger
	priority    int
}

func (s *SCIPDataFallbackStrategy) CanHandle(err *SubProjectRoutingError) bool {
	return (err.Type == ErrorTypeClientConnection || 
		    err.Type == ErrorTypeClientTimeout) &&
		   (err.Context.LSPMethod == "textDocument/definition" ||
		    err.Context.LSPMethod == "textDocument/references" ||
		    err.Context.LSPMethod == "textDocument/hover")
}

func (s *SCIPDataFallbackStrategy) Execute(ctx context.Context, req *LSPRequest) (*LSPResponse, error) {
	s.logger.WithContext(req.Context).Info("Using SCIP data fallback strategy")
	
	// Use SCIP data to provide responses when LSP clients are unavailable
	// This would integrate with the existing SCIP infrastructure
	
	return &LSPResponse{
		Error: &transport.RPCError{
			Code:    -32601,
			Message: "SCIP fallback not implemented",
		},
		ID: req.ID,
	}, fmt.Errorf("SCIP fallback not implemented")
}

func (s *SCIPDataFallbackStrategy) GetPriority() int { return 70 }
func (s *SCIPDataFallbackStrategy) GetName() string { return "scip_data_fallback" }
func (s *SCIPDataFallbackStrategy) GetDescription() string {
	return "Uses SCIP indexing data to provide responses when LSP clients are unavailable"
}
func (s *SCIPDataFallbackStrategy) GetEstimatedRecoveryTime() time.Duration {
	return 50 * time.Millisecond
}

// =====================================================================================
// ERROR HANDLER AND RECOVERY SYSTEM
// =====================================================================================

// SubProjectErrorHandler is the main error handling and recovery system
type SubProjectErrorHandler struct {
	strategies       []FallbackStrategy
	circuitBreakers  map[string]*CircuitBreaker
	retryConfigs     map[SubProjectErrorType]RetryConfig
	metrics          *ErrorMetrics
	logger           Logger
	config           *ErrorHandlerConfig
	mu               sync.RWMutex
}

// ErrorHandlerConfig configures the error handler behavior
type ErrorHandlerConfig struct {
	EnableFallbacks         bool          `yaml:"enable_fallbacks" json:"enable_fallbacks"`
	EnableCircuitBreakers   bool          `yaml:"enable_circuit_breakers" json:"enable_circuit_breakers"`
	EnableRetry             bool          `yaml:"enable_retry" json:"enable_retry"`
	MaxFallbackStrategies   int           `yaml:"max_fallback_strategies" json:"max_fallback_strategies"`
	CircuitBreakerThreshold int           `yaml:"circuit_breaker_threshold" json:"circuit_breaker_threshold"`
	CircuitBreakerTimeout   time.Duration `yaml:"circuit_breaker_timeout" json:"circuit_breaker_timeout"`
	EnableMetrics           bool          `yaml:"enable_metrics" json:"enable_metrics"`
	EnableAlerts            bool          `yaml:"enable_alerts" json:"enable_alerts"`
}

// RetryConfig defines retry behavior for different error types
type RetryConfig struct {
	MaxAttempts    int           `yaml:"max_attempts" json:"max_attempts"`
	InitialDelay   time.Duration `yaml:"initial_delay" json:"initial_delay"`
	MaxDelay       time.Duration `yaml:"max_delay" json:"max_delay"`
	BackoffFactor  float64       `yaml:"backoff_factor" json:"backoff_factor"`
	Jitter         bool          `yaml:"jitter" json:"jitter"`
	TimeoutPerAttempt time.Duration `yaml:"timeout_per_attempt" json:"timeout_per_attempt"`
}

// CircuitBreaker implements circuit breaker pattern for sub-project operations
type CircuitBreaker struct {
	name            string
	state           int32 // atomic: 0=closed, 1=open, 2=half-open
	failureCount    int64 // atomic
	successCount    int64 // atomic
	lastFailureTime int64 // atomic (unix nano)
	threshold       int64
	timeout         time.Duration
	mu              sync.RWMutex
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int32

const (
	CircuitClosed CircuitBreakerState = iota
	CircuitOpen
	CircuitHalfOpen
)

// ErrorMetrics tracks error patterns and recovery effectiveness
type ErrorMetrics struct {
	// Error counts by type
	errorCounts        map[SubProjectErrorType]int64
	// Recovery success rates
	recoverySuccessRates map[string]float64
	// Fallback strategy usage
	fallbackUsage      map[string]int64
	// Circuit breaker states
	circuitStates      map[string]CircuitBreakerState
	// Response times
	responseTimeMs     map[string]int64
	mu                 sync.RWMutex
}

// =====================================================================================
// MAIN ERROR HANDLER IMPLEMENTATION
// =====================================================================================

// NewSubProjectErrorHandler creates a new comprehensive error handler
func NewSubProjectErrorHandler(config *ErrorHandlerConfig, logger Logger) *SubProjectErrorHandler {
	handler := &SubProjectErrorHandler{
		strategies:      make([]FallbackStrategy, 0),
		circuitBreakers: make(map[string]*CircuitBreaker),
		retryConfigs:    make(map[SubProjectErrorType]RetryConfig),
		metrics:         NewErrorMetrics(),
		logger:          logger,
		config:          config,
	}
	
	// Initialize default retry configurations
	handler.setupDefaultRetryConfigs()
	
	// Register default fallback strategies
	handler.registerDefaultStrategies()
	
	return handler
}

// HandleError is the main entry point for error handling and recovery
func (h *SubProjectErrorHandler) HandleError(ctx context.Context, err *SubProjectRoutingError, originalRequest *LSPRequest) (*LSPResponse, error) {
	h.logger.WithFields(map[string]interface{}{
		"error_type":    err.Type,
		"severity":      err.Severity,
		"recovery_type": err.RecoveryType,
		"request_id":    err.RequestID,
	}).Error("Handling sub-project routing error")
	
	// Update metrics
	h.metrics.RecordError(err.Type)
	
	// Check circuit breaker state
	if h.config.EnableCircuitBreakers {
		if breaker := h.getCircuitBreaker(err.Context.Operation); breaker != nil {
			if breaker.IsOpen() {
				return h.createCircuitOpenResponse(originalRequest), 
					   fmt.Errorf("circuit breaker open for operation: %s", err.Context.Operation)
			}
		}
	}
	
	// Handle based on recovery type
	switch err.RecoveryType {
	case RecoveryTypeRetry:
		return h.handleRetry(ctx, err, originalRequest)
	case RecoveryTypeFallback:
		return h.handleFallback(ctx, err, originalRequest)
	case RecoveryTypeDegrade:
		return h.handleGracefulDegradation(ctx, err, originalRequest)
	case RecoveryTypeFail:
		return h.handleCriticalFailure(ctx, err, originalRequest)
	default:
		// Default to fallback strategy
		return h.handleFallback(ctx, err, originalRequest)
	}
}

// handleFallback executes appropriate fallback strategies
func (h *SubProjectErrorHandler) handleFallback(ctx context.Context, err *SubProjectRoutingError, req *LSPRequest) (*LSPResponse, error) {
	if !h.config.EnableFallbacks {
		return h.createErrorResponse(req, "Fallbacks disabled"), 
			   fmt.Errorf("fallback handling disabled")
	}
	
	// Find applicable fallback strategies
	applicableStrategies := h.findApplicableStrategies(err)
	
	if len(applicableStrategies) == 0 {
		h.logger.Warn("No applicable fallback strategies found")
		return h.createErrorResponse(req, "No fallback strategies available"), 
			   fmt.Errorf("no applicable fallback strategies")
	}
	
	// Try strategies in priority order
	for _, strategy := range applicableStrategies {
		h.logger.WithField("strategy", strategy.GetName()).Info("Attempting fallback strategy")
		
		// Create timeout context for strategy execution
		strategyCtx, cancel := context.WithTimeout(ctx, strategy.GetEstimatedRecoveryTime()*2)
		defer cancel()
		
		response, strategyErr := strategy.Execute(strategyCtx, req)
		if strategyErr == nil {
			h.metrics.RecordFallbackSuccess(strategy.GetName())
			h.logger.WithField("strategy", strategy.GetName()).Info("Fallback strategy succeeded")
			return response, nil
		}
		
		h.metrics.RecordFallbackFailure(strategy.GetName())
		h.logger.WithFields(map[string]interface{}{
			"strategy": strategy.GetName(),
			"error":    strategyErr,
		}).Warn("Fallback strategy failed")
	}
	
	return h.createErrorResponse(req, "All fallback strategies failed"), 
		   fmt.Errorf("all fallback strategies exhausted")
}

// handleRetry implements retry logic with exponential backoff
func (h *SubProjectErrorHandler) handleRetry(ctx context.Context, err *SubProjectRoutingError, req *LSPRequest) (*LSPResponse, error) {
	if !h.config.EnableRetry {
		return h.createErrorResponse(req, "Retry disabled"), 
			   fmt.Errorf("retry handling disabled")
	}
	
	retryConfig, exists := h.retryConfigs[err.Type]
	if !exists {
		// Use default retry config
		retryConfig = RetryConfig{
			MaxAttempts:       3,
			InitialDelay:      100 * time.Millisecond,
			MaxDelay:          5 * time.Second,
			BackoffFactor:     2.0,
			Jitter:            true,
			TimeoutPerAttempt: 30 * time.Second,
		}
	}
	
	var lastErr error
	delay := retryConfig.InitialDelay
	
	for attempt := 1; attempt <= retryConfig.MaxAttempts; attempt++ {
		if attempt > 1 {
			// Apply backoff delay
			if retryConfig.Jitter {
				// Add jitter to prevent thundering herd
				jitter := time.Duration(float64(delay) * 0.1 * (2*time.Now().UnixNano()%2 - 1))
				time.Sleep(delay + jitter)
			} else {
				time.Sleep(delay)
			}
			
			// Increase delay for next attempt
			delay = time.Duration(float64(delay) * retryConfig.BackoffFactor)
			if delay > retryConfig.MaxDelay {
				delay = retryConfig.MaxDelay
			}
		}
		
		// Create timeout context for this attempt
		attemptCtx, cancel := context.WithTimeout(ctx, retryConfig.TimeoutPerAttempt)
		defer cancel()
		
		// Update context with attempt number
		req.Context.AttemptNumber = attempt
		
		// Attempt the original operation again
		// This would need to be implemented to re-execute the original operation
		h.logger.WithFields(map[string]interface{}{
			"attempt": attempt,
			"max_attempts": retryConfig.MaxAttempts,
		}).Info("Retrying operation")
		
		// Placeholder - actual retry implementation would go here
		// For now, return an error to indicate retry logic is in place
		lastErr = fmt.Errorf("retry attempt %d failed", attempt)
	}
	
	return h.createErrorResponse(req, fmt.Sprintf("Retry exhausted after %d attempts", retryConfig.MaxAttempts)), 
		   fmt.Errorf("retry exhausted: %w", lastErr)
}

// =====================================================================================
// SUPPORTING METHODS AND UTILITIES
// =====================================================================================

// findApplicableStrategies finds and sorts fallback strategies by priority
func (h *SubProjectErrorHandler) findApplicableStrategies(err *SubProjectRoutingError) []FallbackStrategy {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	var applicable []FallbackStrategy
	for _, strategy := range h.strategies {
		if strategy.CanHandle(err) {
			applicable = append(applicable, strategy)
		}
	}
	
	// Sort by priority (highest first)
	for i := 0; i < len(applicable)-1; i++ {
		for j := i + 1; j < len(applicable); j++ {
			if applicable[i].GetPriority() < applicable[j].GetPriority() {
				applicable[i], applicable[j] = applicable[j], applicable[i]
			}
		}
	}
	
	// Limit to configured maximum
	if h.config.MaxFallbackStrategies > 0 && len(applicable) > h.config.MaxFallbackStrategies {
		applicable = applicable[:h.config.MaxFallbackStrategies]
	}
	
	return applicable
}

// setupDefaultRetryConfigs initializes default retry configurations
func (h *SubProjectErrorHandler) setupDefaultRetryConfigs() {
	h.retryConfigs[ErrorTypeClientTimeout] = RetryConfig{
		MaxAttempts:       2,
		InitialDelay:      1 * time.Second,
		MaxDelay:          10 * time.Second,
		BackoffFactor:     2.0,
		Jitter:            true,
		TimeoutPerAttempt: 30 * time.Second,
	}
	
	h.retryConfigs[ErrorTypeClientConnection] = RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      500 * time.Millisecond,
		MaxDelay:          5 * time.Second,
		BackoffFactor:     2.0,
		Jitter:            true,
		TimeoutPerAttempt: 20 * time.Second,
	}
	
	h.retryConfigs[ErrorTypeNetworkUnavailable] = RetryConfig{
		MaxAttempts:       2,
		InitialDelay:      2 * time.Second,
		MaxDelay:          15 * time.Second,
		BackoffFactor:     2.0,
		Jitter:            true,
		TimeoutPerAttempt: 45 * time.Second,
	}
}

// registerDefaultStrategies registers built-in fallback strategies
func (h *SubProjectErrorHandler) registerDefaultStrategies() {
	// These would be initialized with proper dependencies
	strategies := []FallbackStrategy{
		&WorkspaceRootFallbackStrategy{priority: 100},
		&ParentDirectoryTraversalStrategy{priority: 90},
		&CachedResponseFallbackStrategy{priority: 80},
		&SCIPDataFallbackStrategy{priority: 70},
	}
	
	for _, strategy := range strategies {
		h.RegisterFallbackStrategy(strategy)
	}
}

// RegisterFallbackStrategy adds a new fallback strategy
func (h *SubProjectErrorHandler) RegisterFallbackStrategy(strategy FallbackStrategy) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.strategies = append(h.strategies, strategy)
	h.logger.WithFields(map[string]interface{}{
		"strategy":    strategy.GetName(),
		"priority":    strategy.GetPriority(),
		"description": strategy.GetDescription(),
	}).Info("Registered fallback strategy")
}

// getCircuitBreaker gets or creates a circuit breaker for an operation
func (h *SubProjectErrorHandler) getCircuitBreaker(operation string) *CircuitBreaker {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if breaker, exists := h.circuitBreakers[operation]; exists {
		return breaker
	}
	
	breaker := &CircuitBreaker{
		name:      operation,
		threshold: int64(h.config.CircuitBreakerThreshold),
		timeout:   h.config.CircuitBreakerTimeout,
	}
	
	h.circuitBreakers[operation] = breaker
	return breaker
}

// createErrorResponse creates a standardized error response
func (h *SubProjectErrorHandler) createErrorResponse(req *LSPRequest, message string) *LSPResponse {
	return &LSPResponse{
		Error: &transport.RPCError{
			Code:    -32603,
			Message: message,
		},
		ID: req.ID,
		Metadata: map[string]interface{}{
			"error_handler": "sub_project_error_handler",
			"timestamp":     time.Now().Unix(),
		},
	}
}

// createCircuitOpenResponse creates a response when circuit breaker is open
func (h *SubProjectErrorHandler) createCircuitOpenResponse(req *LSPRequest) *LSPResponse {
	return &LSPResponse{
		Error: &transport.RPCError{
			Code:    -32603,
			Message: "Service temporarily unavailable - circuit breaker is open",
		},
		ID: req.ID,
		Metadata: map[string]interface{}{
			"circuit_breaker": "open",
			"retry_after":     h.config.CircuitBreakerTimeout.Seconds(),
		},
	}
}

// handleGracefulDegradation implements graceful degradation strategies
func (h *SubProjectErrorHandler) handleGracefulDegradation(ctx context.Context, err *SubProjectRoutingError, req *LSPRequest) (*LSPResponse, error) {
	h.logger.Info("Applying graceful degradation")
	
	// Implement read-only mode or limited functionality
	return h.createErrorResponse(req, "Service degraded - limited functionality available"), 
		   fmt.Errorf("graceful degradation applied")
}

// handleCriticalFailure handles critical failures that require system attention
func (h *SubProjectErrorHandler) handleCriticalFailure(ctx context.Context, err *SubProjectRoutingError, req *LSPRequest) (*LSPResponse, error) {
	h.logger.WithFields(map[string]interface{}{
		"error_type": err.Type,
		"severity":   err.Severity,
		"context":    err.Context,
	}).Error("Critical failure in sub-project routing")
	
	// This would trigger alerts and possibly system shutdown procedures
	return h.createErrorResponse(req, "Critical system error"), 
		   fmt.Errorf("critical failure: %s", err.Message)
}

// =====================================================================================
// CIRCUIT BREAKER IMPLEMENTATION
// =====================================================================================

// IsOpen checks if the circuit breaker is open
func (cb *CircuitBreaker) IsOpen() bool {
	state := CircuitBreakerState(atomic.LoadInt32(&cb.state))
	
	switch state {
	case CircuitClosed:
		return false
	case CircuitOpen:
		// Check if we should transition to half-open
		lastFailure := atomic.LoadInt64(&cb.lastFailureTime)
		if time.Since(time.Unix(0, lastFailure)) > cb.timeout {
			// Try to transition to half-open
			atomic.CompareAndSwapInt32(&cb.state, int32(CircuitOpen), int32(CircuitHalfOpen))
		}
		return atomic.LoadInt32(&cb.state) == int32(CircuitOpen)
	case CircuitHalfOpen:
		return false
	default:
		return false
	}
}

// RecordSuccess records a successful operation
func (cb *CircuitBreaker) RecordSuccess() {
	atomic.AddInt64(&cb.successCount, 1)
	
	state := CircuitBreakerState(atomic.LoadInt32(&cb.state))
	if state == CircuitHalfOpen {
		// Transition back to closed
		atomic.StoreInt32(&cb.state, int32(CircuitClosed))
		atomic.StoreInt64(&cb.failureCount, 0)
	}
}

// RecordFailure records a failed operation
func (cb *CircuitBreaker) RecordFailure() {
	failures := atomic.AddInt64(&cb.failureCount, 1)
	atomic.StoreInt64(&cb.lastFailureTime, time.Now().UnixNano())
	
	if failures >= cb.threshold {
		atomic.StoreInt32(&cb.state, int32(CircuitOpen))
	}
}

// =====================================================================================
// ERROR METRICS IMPLEMENTATION
// =====================================================================================

// NewErrorMetrics creates a new error metrics tracker
func NewErrorMetrics() *ErrorMetrics {
	return &ErrorMetrics{
		errorCounts:          make(map[SubProjectErrorType]int64),
		recoverySuccessRates: make(map[string]float64),
		fallbackUsage:        make(map[string]int64),
		circuitStates:        make(map[string]CircuitBreakerState),
		responseTimeMs:       make(map[string]int64),
	}
}

// RecordError records an error occurrence
func (em *ErrorMetrics) RecordError(errorType SubProjectErrorType) {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.errorCounts[errorType]++
}

// RecordFallbackSuccess records a successful fallback
func (em *ErrorMetrics) RecordFallbackSuccess(strategyName string) {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.fallbackUsage[strategyName]++
}

// RecordFallbackFailure records a failed fallback
func (em *ErrorMetrics) RecordFallbackFailure(strategyName string) {
	em.mu.Lock()
	defer em.mu.Unlock()
	// Implementation would track failure rates
}

// GetMetricsSnapshot returns a snapshot of current metrics
func (em *ErrorMetrics) GetMetricsSnapshot() map[string]interface{} {
	em.mu.RLock()
	defer em.mu.RUnlock()
	
	return map[string]interface{}{
		"error_counts":           em.errorCounts,
		"recovery_success_rates": em.recoverySuccessRates,
		"fallback_usage":         em.fallbackUsage,
		"circuit_states":         em.circuitStates,
		"response_times":         em.responseTimeMs,
		"timestamp":              time.Now().Unix(),
	}
}

// =====================================================================================
// INTERFACES AND PLACEHOLDER TYPES
// =====================================================================================

// Logger interface for error handler logging
type Logger interface {
	WithFields(fields map[string]interface{}) Logger
	WithContext(ctx *ErrorContext) Logger
	Info(msg string)
	Warn(msg string)
	Error(msg string)
}

// ProjectDetector interface for project detection
type ProjectDetector interface {
	DetectProject(path string) (*SubProjectInfo, error)
}

// ResponseCache interface for caching responses
type ResponseCache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration)
}

// SCIPStore interface for SCIP data access
type SCIPStore interface {
	GetDefinition(uri string, position Position) ([]Location, error)
	GetReferences(uri string, position Position) ([]Location, error)
	GetHover(uri string, position Position) (*Hover, error)
}

// Placeholder types for LSP data structures
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Hover struct {
	Contents interface{} `json:"contents"`
	Range    *Range      `json:"range,omitempty"`
}

// =====================================================================================
// INTEGRATION POINTS AND USAGE EXAMPLES
// =====================================================================================

/*
INTEGRATION WITH EXISTING SYSTEMS:

1. Gateway Integration (internal/gateway/handlers.go):
   - Wrap existing request handlers with error handling
   - Use SubProjectErrorHandler for all sub-project routing errors
   - Integrate with existing RPC error responses

2. Transport Layer Integration (internal/transport/):
   - Extend existing circuit breaker patterns
   - Integrate with transport-level error handling
   - Use consistent error classification

3. Project Detection Integration (internal/project/):
   - Use existing project error types as foundation
   - Extend with sub-project specific error handling
   - Integrate with workspace detection

4. SCIP Integration (internal/indexing/):
   - Use SCIP data for fallback responses
   - Integrate with existing SCIP caching
   - Provide offline capabilities

USAGE EXAMPLE:

```go
// Initialize error handler
config := &ErrorHandlerConfig{
    EnableFallbacks:         true,
    EnableCircuitBreakers:   true,
    EnableRetry:             true,
    MaxFallbackStrategies:   3,
    CircuitBreakerThreshold: 5,
    CircuitBreakerTimeout:   30 * time.Second,
}

errorHandler := NewSubProjectErrorHandler(config, logger)

// Handle an error during sub-project routing
err := &SubProjectRoutingError{
    Type:         ErrorTypeProjectResolution,
    Severity:     SeverityError,
    RecoveryType: RecoveryTypeFallback,
    Message:      "Unable to resolve file to sub-project",
    Context: &ErrorContext{
        Operation:     "route_request",
        FileURI:       "file:///path/to/file.go",
        LSPMethod:     "textDocument/definition",
        WorkspaceRoot: "/path/to/workspace",
    },
}

originalRequest := &LSPRequest{
    Method: "textDocument/definition",
    Params: definitionParams,
    ID:     1,
    Context: err.Context,
}

response, recoveryErr := errorHandler.HandleError(ctx, err, originalRequest)
if recoveryErr != nil {
    // Handle unrecoverable error
    return nil, recoveryErr
}

// Use recovered response
return response, nil
```

TESTING STRATEGY:

1. Unit Tests:
   - Test individual fallback strategies
   - Test circuit breaker behavior
   - Test retry logic and backoff

2. Integration Tests:
   - Test error handler with real transport clients
   - Test fallback strategies with actual project structures
   - Test metrics collection and reporting

3. Chaos Engineering:
   - Inject failures at various points
   - Test system resilience under load
   - Validate fallback strategy effectiveness

4. Performance Tests:
   - Measure overhead of error handling
   - Test fallback response times
   - Validate circuit breaker performance

MONITORING AND ALERTING:

1. Metrics Collection:
   - Error rates by type and project
   - Fallback strategy success rates
   - Circuit breaker state changes
   - Recovery time distributions

2. Alerting Rules:
   - High error rates (>5% over 5 minutes)
   - Fallback strategy failures (>50% over 10 minutes)
   - Circuit breaker open states (any circuit open >1 minute)
   - Critical error occurrences (any critical error)

3. Dashboards:
   - Real-time error rate monitoring
   - Fallback strategy effectiveness
   - System health overview
   - Recovery time trends
*/