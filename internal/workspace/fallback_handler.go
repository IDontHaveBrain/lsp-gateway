package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"lsp-gateway/mcp"
)

// FallbackHandler manages multi-level fallback strategies for failed requests
type FallbackHandler struct {
	// Core components
	clientManager SubProjectClientManager
	logger        *mcp.StructuredLogger
	
	// Fallback levels
	fallbackLevels []FallbackLevel
	
	// Error aggregation
	errorHistory   map[string]*ErrorAggregation
	historyLock    sync.RWMutex
	
	// Configuration
	maxRetries     int
	retryBackoff   time.Duration
	errorCacheTTL  time.Duration
	
	// Metrics
	fallbackMetrics *FallbackMetrics
	metricsLock     sync.RWMutex
	
	// Lifecycle
	ctx            context.Context
	cancel         context.CancelFunc
	isShutdown     bool
}

// FallbackLevel represents a level in the fallback hierarchy
type FallbackLevel struct {
	Name        string                                    `json:"name"`
	Description string                                    `json:"description"`
	Handler     func(context.Context, *JSONRPCRequest, error) (*JSONRPCResponse, error) `json:"-"`
	Timeout     time.Duration                             `json:"timeout"`
	Priority    int                                       `json:"priority"`
	Enabled     bool                                      `json:"enabled"`
}

// ErrorAggregation tracks errors across fallback attempts
type ErrorAggregation struct {
	RequestID     interface{}       `json:"request_id"`
	Method        string            `json:"method"`
	FileURI       string            `json:"file_uri"`
	Errors        []FallbackError   `json:"errors"`
	StartTime     time.Time         `json:"start_time"`
	LastAttempt   time.Time         `json:"last_attempt"`
	AttemptCount  int               `json:"attempt_count"`
	FinalError    error             `json:"-"`
}

// FallbackError represents an error from a specific fallback level
type FallbackError struct {
	Level       string    `json:"level"`
	Error       string    `json:"error"`
	Timestamp   time.Time `json:"timestamp"`
	Duration    time.Duration `json:"duration"`
	Recoverable bool      `json:"recoverable"`
}

// FallbackMetrics tracks fallback performance and usage
type FallbackMetrics struct {
	TotalFallbacks    int64                  `json:"total_fallbacks"`
	SuccessfulFallbacks int64                `json:"successful_fallbacks"`
	FailedFallbacks   int64                  `json:"failed_fallbacks"`
	LevelUsage        map[string]int64       `json:"level_usage"`
	LevelSuccess      map[string]int64       `json:"level_success"`
	AverageLatency    time.Duration          `json:"average_latency"`
	ErrorCategories   map[string]int64       `json:"error_categories"`
	LastUpdated       time.Time              `json:"last_updated"`
}

// NewFallbackHandler creates a new multi-level fallback handler
func NewFallbackHandler(logger *mcp.StructuredLogger) *FallbackHandler {
	ctx, cancel := context.WithCancel(context.Background())
	
	handler := &FallbackHandler{
		logger:          logger,
		errorHistory:    make(map[string]*ErrorAggregation),
		maxRetries:      3,
		retryBackoff:    500 * time.Millisecond,
		errorCacheTTL:   5 * time.Minute,
		fallbackMetrics: &FallbackMetrics{
			LevelUsage:      make(map[string]int64),
			LevelSuccess:    make(map[string]int64),
			ErrorCategories: make(map[string]int64),
			LastUpdated:     time.Now(),
		},
		ctx:    ctx,
		cancel: cancel,
	}
	
	// Initialize default fallback levels
	handler.initializeFallbackLevels()
	
	if logger != nil {
		logger.Info("FallbackHandler initialized with multi-level fallback support")
	}
	
	return handler
}

// HandleFailure processes a failed request through the fallback hierarchy
func (f *FallbackHandler) HandleFailure(ctx context.Context, request *JSONRPCRequest, originalError error) (*JSONRPCResponse, error) {
	if f.isShutdown {
		return nil, fmt.Errorf("fallback handler is shutdown")
	}
	
	startTime := time.Now()
	defer func() {
		f.updateLatencyMetrics(time.Since(startTime))
	}()
	
	f.updateMetrics(func(m *FallbackMetrics) { m.TotalFallbacks++ })
	
	// Create error aggregation for this request
	requestKey := f.getRequestKey(request)
	errorAgg := f.getOrCreateErrorAggregation(requestKey, request, originalError)
	
	if f.logger != nil {
		f.logger.WithFields(map[string]interface{}{
			"method":      request.Method,
			"request_id":  request.ID,
			"attempt":     errorAgg.AttemptCount + 1,
			"original_error": originalError.Error(),
		}).Info("Starting fallback handling")
	}
	
	// Try each fallback level in order
	for _, level := range f.fallbackLevels {
		if !level.Enabled {
			continue
		}
		
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("fallback handling cancelled")
		default:
		}
		
		levelStartTime := time.Now()
		f.updateMetrics(func(m *FallbackMetrics) { m.LevelUsage[level.Name]++ })
		
		if f.logger != nil {
			f.logger.WithFields(map[string]interface{}{
				"level":       level.Name,
				"method":      request.Method,
				"timeout":     level.Timeout.String(),
			}).Debug("Attempting fallback level")
		}
		
		// Create timeout context for this level
		levelCtx, cancel := context.WithTimeout(ctx, level.Timeout)
		
		// Try the fallback handler
		response, err := level.Handler(levelCtx, request, originalError)
		cancel()
		
		duration := time.Since(levelStartTime)
		
		// Record attempt in error aggregation
		fallbackErr := FallbackError{
			Level:       level.Name,
			Timestamp:   levelStartTime,
			Duration:    duration,
			Recoverable: f.isRecoverableError(err),
		}
		
		if err != nil {
			fallbackErr.Error = err.Error()
			errorAgg.Errors = append(errorAgg.Errors, fallbackErr)
			errorAgg.FinalError = err
			
			if f.logger != nil {
				f.logger.WithError(err).WithFields(map[string]interface{}{
					"level":    level.Name,
					"duration": duration.String(),
				}).Debug("Fallback level failed")
			}
			continue
		}
		
		// Success!
		fallbackErr.Error = "success"
		errorAgg.Errors = append(errorAgg.Errors, fallbackErr)
		
		f.updateMetrics(func(m *FallbackMetrics) { 
			m.SuccessfulFallbacks++
			m.LevelSuccess[level.Name]++
		})
		
		if f.logger != nil {
			f.logger.WithFields(map[string]interface{}{
				"level":       level.Name,
				"method":      request.Method,
				"duration":    duration.String(),
				"total_time":  time.Since(startTime).String(),
			}).Info("Fallback succeeded")
		}
		
		// Clean up error aggregation on success
		f.cleanupErrorAggregation(requestKey)
		
		return response, nil
	}
	
	// All fallback levels failed
	errorAgg.AttemptCount++
	errorAgg.LastAttempt = time.Now()
	
	f.updateMetrics(func(m *FallbackMetrics) { m.FailedFallbacks++ })
	
	if f.logger != nil {
		f.logger.WithFields(map[string]interface{}{
			"method":        request.Method,
			"total_attempts": len(errorAgg.Errors),
			"total_time":    time.Since(startTime).String(),
		}).Error("All fallback levels exhausted")
	}
	
	return nil, fmt.Errorf("all fallback levels failed, final error: %w", errorAgg.FinalError)
}

// SetClientManager sets the client manager for fallback operations
func (f *FallbackHandler) SetClientManager(manager SubProjectClientManager) {
	f.clientManager = manager
}

// GetMetrics returns current fallback metrics
func (f *FallbackHandler) GetMetrics() *FallbackMetrics {
	f.metricsLock.RLock()
	defer f.metricsLock.RUnlock()
	
	// Create a copy to avoid race conditions
	metricsCopy := *f.fallbackMetrics
	metricsCopy.LastUpdated = time.Now()
	
	return &metricsCopy
}

// GetErrorHistory returns error aggregation history
func (f *FallbackHandler) GetErrorHistory() map[string]*ErrorAggregation {
	f.historyLock.RLock()
	defer f.historyLock.RUnlock()
	
	history := make(map[string]*ErrorAggregation)
	for k, v := range f.errorHistory {
		history[k] = v
	}
	
	return history
}

// Shutdown gracefully shuts down the fallback handler
func (f *FallbackHandler) Shutdown(ctx context.Context) error {
	f.isShutdown = true
	f.cancel()
	
	if f.logger != nil {
		f.logger.Info("FallbackHandler shutdown completed")
	}
	
	return nil
}

// Initialize fallback levels with default handlers
func (f *FallbackHandler) initializeFallbackLevels() {
	f.fallbackLevels = []FallbackLevel{
		{
			Name:        "subproject_retry",
			Description: "Retry with the same sub-project client after a brief delay",
			Handler:     f.handleSubProjectRetry,
			Timeout:     5 * time.Second,
			Priority:    100,
			Enabled:     true,
		},
		{
			Name:        "alternative_clients",
			Description: "Try alternative clients within the same sub-project",
			Handler:     f.handleAlternativeClients,
			Timeout:     10 * time.Second,
			Priority:    90,
			Enabled:     true,
		},
		{
			Name:        "parent_project",
			Description: "Fallback to parent project clients",
			Handler:     f.handleParentProject,
			Timeout:     15 * time.Second,
			Priority:    80,
			Enabled:     true,
		},
		{
			Name:        "child_projects",
			Description: "Try child project clients",
			Handler:     f.handleChildProjects,
			Timeout:     15 * time.Second,
			Priority:    70,
			Enabled:     true,
		},
		{
			Name:        "workspace_level",
			Description: "Fallback to workspace-level clients",
			Handler:     f.handleWorkspaceLevel,
			Timeout:     20 * time.Second,
			Priority:    60,
			Enabled:     true,
		},
		{
			Name:        "cached_responses",
			Description: "Return cached responses for read operations",
			Handler:     f.handleCachedResponses,
			Timeout:     1 * time.Second,
			Priority:    50,
			Enabled:     true,
		},
		{
			Name:        "read_only_mode",
			Description: "Provide basic read-only functionality",
			Handler:     f.handleReadOnlyMode,
			Timeout:     500 * time.Millisecond,
			Priority:    40,
			Enabled:     true,
		},
	}
}

// Fallback level handlers

func (f *FallbackHandler) handleSubProjectRetry(ctx context.Context, request *JSONRPCRequest, originalError error) (*JSONRPCResponse, error) {
	// Simple retry with backoff
	time.Sleep(f.retryBackoff)
	
	if f.clientManager == nil {
		return nil, fmt.Errorf("client manager not available for retry")
	}
	
	// Extract project context from error if available
	projectID, language := f.extractProjectContext(request, originalError)
	if projectID == "" {
		return nil, fmt.Errorf("cannot determine project context for retry")
	}
	
	// Try to get the same client again (may have recovered)
	client, err := f.clientManager.GetClient(projectID, language)
	if err != nil {
		return nil, fmt.Errorf("client still unavailable after retry: %w", err)
	}
	
	// Retry the original request
	result, err := client.SendRequest(ctx, request.Method, request.Params)
	if err != nil {
		return nil, fmt.Errorf("retry request failed: %w", err)
	}
	
	return &JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      request.ID,
		Result:  json.RawMessage(result),
	}, nil
}

func (f *FallbackHandler) handleAlternativeClients(ctx context.Context, request *JSONRPCRequest, originalError error) (*JSONRPCResponse, error) {
	if f.clientManager == nil {
		return nil, fmt.Errorf("client manager not available for alternative client fallback")
	}
	
	projectID, originalLanguage := f.extractProjectContext(request, originalError)
	if projectID == "" {
		return nil, fmt.Errorf("cannot determine project context for alternative clients")
	}
	
	// Get all clients for the project
	allProjectClients := f.clientManager.GetRegistry().GetAllClientsForProject(projectID)
	
	// Try each alternative language client in the same project
	for language, client := range allProjectClients {
		if language == originalLanguage {
			continue // Skip the original failing client
		}
		
		if client == nil || !client.IsActive() {
			continue
		}
		
		// Try the alternative client
		result, err := client.SendRequest(ctx, request.Method, request.Params)
		if err == nil {
			if f.logger != nil {
				f.logger.WithFields(map[string]interface{}{
					"original_language": originalLanguage,
					"fallback_language": language,
					"project_id":        projectID,
				}).Info("Alternative client fallback succeeded")
			}
			
			return &JSONRPCResponse{
				JSONRPC: JSONRPCVersion,
				ID:      request.ID,
				Result:  json.RawMessage(result),
			}, nil
		}
	}
	
	return nil, fmt.Errorf("no alternative clients available or all failed")
}

func (f *FallbackHandler) handleParentProject(ctx context.Context, request *JSONRPCRequest, originalError error) (*JSONRPCResponse, error) {
	if f.clientManager == nil {
		return nil, fmt.Errorf("client manager not available for parent project fallback")
	}
	
	projectID, language := f.extractProjectContext(request, originalError)
	if projectID == "" {
		return nil, fmt.Errorf("cannot determine project context for parent project fallback")
	}
	
	// This is a simplified approach - in a full implementation, we'd need project hierarchy
	// For now, try workspace-level clients as parent fallback
	allClients := f.clientManager.GetRegistry().GetAllClients()
	
	// Look for workspace-root or parent-like projects
	for otherProjectID, clients := range allClients {
		if otherProjectID == projectID {
			continue // Skip the current project
		}
		
		// Prefer workspace-root or projects with shorter paths (likely parents)
		if strings.Contains(otherProjectID, "workspace") || strings.Contains(otherProjectID, "root") {
			if client, exists := clients[language]; exists && client != nil && client.IsActive() {
				result, err := client.SendRequest(ctx, request.Method, request.Params)
				if err == nil {
					if f.logger != nil {
						f.logger.WithFields(map[string]interface{}{
							"original_project": projectID,
							"parent_project":   otherProjectID,
							"language":         language,
						}).Info("Parent project fallback succeeded")
					}
					
					return &JSONRPCResponse{
						JSONRPC: JSONRPCVersion,
						ID:      request.ID,
						Result:  json.RawMessage(result),
					}, nil
				}
			}
		}
	}
	
	return nil, fmt.Errorf("no parent project clients available")
}

func (f *FallbackHandler) handleChildProjects(ctx context.Context, request *JSONRPCRequest, originalError error) (*JSONRPCResponse, error) {
	if f.clientManager == nil {
		return nil, fmt.Errorf("client manager not available for child projects fallback")
	}
	
	projectID, language := f.extractProjectContext(request, originalError)
	if projectID == "" {
		return nil, fmt.Errorf("cannot determine project context for child projects fallback")
	}
	
	// This is a simplified approach - try all other projects as potential children
	allClients := f.clientManager.GetRegistry().GetAllClients()
	
	for otherProjectID, clients := range allClients {
		if otherProjectID == projectID {
			continue // Skip the current project
		}
		
		// Try projects that might be children (contain current project path or have longer paths)
		if len(otherProjectID) > len(projectID) || strings.Contains(otherProjectID, projectID) {
			if client, exists := clients[language]; exists && client != nil && client.IsActive() {
				result, err := client.SendRequest(ctx, request.Method, request.Params)
				if err == nil {
					if f.logger != nil {
						f.logger.WithFields(map[string]interface{}{
							"original_project": projectID,
							"child_project":    otherProjectID,
							"language":         language,
						}).Info("Child project fallback succeeded")
					}
					
					return &JSONRPCResponse{
						JSONRPC: JSONRPCVersion,
						ID:      request.ID,
						Result:  json.RawMessage(result),
					}, nil
				}
			}
		}
	}
	
	return nil, fmt.Errorf("no child project clients available")
}

func (f *FallbackHandler) handleWorkspaceLevel(ctx context.Context, request *JSONRPCRequest, originalError error) (*JSONRPCResponse, error) {
	if f.clientManager == nil {
		return nil, fmt.Errorf("client manager not available for workspace level fallback")
	}
	
	_, language := f.extractProjectContext(request, originalError)
	if language == "" {
		// Try to infer language from request
		language = f.inferLanguageFromRequest(request)
	}
	
	// Try to find any workspace-level or root client
	allClients := f.clientManager.GetRegistry().GetAllClients()
	
	// First try workspace-root clients
	for projectID, clients := range allClients {
		if strings.Contains(strings.ToLower(projectID), "workspace") || 
		   strings.Contains(strings.ToLower(projectID), "root") {
			
			// Try the same language first
			if language != "" {
				if client, exists := clients[language]; exists && client != nil && client.IsActive() {
					result, err := client.SendRequest(ctx, request.Method, request.Params)
					if err == nil {
						if f.logger != nil {
							f.logger.WithFields(map[string]interface{}{
								"workspace_project": projectID,
								"language":          language,
							}).Info("Workspace level fallback succeeded")
						}
						
						return &JSONRPCResponse{
							JSONRPC: JSONRPCVersion,
							ID:      request.ID,
							Result:  json.RawMessage(result),
						}, nil
					}
				}
			}
			
			// Try any available client in workspace project
			for clientLang, client := range clients {
				if client != nil && client.IsActive() {
					result, err := client.SendRequest(ctx, request.Method, request.Params)
					if err == nil {
						if f.logger != nil {
							f.logger.WithFields(map[string]interface{}{
								"workspace_project": projectID,
								"fallback_language": clientLang,
							}).Info("Workspace level fallback succeeded with different language")
						}
						
						return &JSONRPCResponse{
							JSONRPC: JSONRPCVersion,
							ID:      request.ID,
							Result:  json.RawMessage(result),
						}, nil
					}
				}
			}
		}
	}
	
	return nil, fmt.Errorf("no workspace level clients available")
}

func (f *FallbackHandler) handleCachedResponses(ctx context.Context, request *JSONRPCRequest, originalError error) (*JSONRPCResponse, error) {
	// Return cached responses for read operations
	readOnlyMethods := map[string]bool{
		"textDocument/definition":    true,
		"textDocument/references":    true,
		"textDocument/hover":         true,
		"textDocument/documentSymbol": true,
		"workspace/symbol":           true,
	}
	
	if !readOnlyMethods[request.Method] {
		return nil, fmt.Errorf("method %s not cacheable", request.Method)
	}
	
	// For now, return empty response
	// In a full implementation, this would check an actual cache
	var result interface{}
	switch request.Method {
	case "textDocument/definition", "textDocument/references":
		result = []interface{}{}
	case "textDocument/documentSymbol", "workspace/symbol":
		result = []interface{}{}
	case "textDocument/hover":
		result = nil
	default:
		result = nil
	}
	
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cached response: %w", err)
	}
	
	return &JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      request.ID,
		Result:  json.RawMessage(resultBytes),
	}, nil
}

func (f *FallbackHandler) handleReadOnlyMode(ctx context.Context, request *JSONRPCRequest, originalError error) (*JSONRPCResponse, error) {
	// Provide basic read-only responses
	readOnlyMethods := map[string]bool{
		"textDocument/definition":    true,
		"textDocument/references":    true,
		"textDocument/hover":         true,
		"textDocument/documentSymbol": true,
		"workspace/symbol":           true,
	}
	
	if !readOnlyMethods[request.Method] {
		return nil, fmt.Errorf("method %s not supported in read-only mode", request.Method)
	}
	
	if f.logger != nil {
		f.logger.WithField("method", request.Method).Warn("Providing read-only fallback response")
	}
	
	// Return empty response for read operations
	var result interface{}
	switch request.Method {
	case "textDocument/definition", "textDocument/references":
		result = []interface{}{}
	case "textDocument/documentSymbol", "workspace/symbol":
		result = []interface{}{}
	case "textDocument/hover":
		result = nil
	default:
		result = nil
	}
	
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal read-only response: %w", err)
	}
	
	return &JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      request.ID,
		Result:  json.RawMessage(resultBytes),
	}, nil
}

// Helper methods

func (f *FallbackHandler) getRequestKey(request *JSONRPCRequest) string {
	return fmt.Sprintf("%s:%v", request.Method, request.ID)
}

func (f *FallbackHandler) getOrCreateErrorAggregation(key string, request *JSONRPCRequest, originalError error) *ErrorAggregation {
	f.historyLock.Lock()
	defer f.historyLock.Unlock()
	
	if agg, exists := f.errorHistory[key]; exists {
		return agg
	}
	
	// Extract file URI from request if possible
	fileURI := ""
	if params, ok := request.Params.(map[string]interface{}); ok {
		if textDoc, exists := params["textDocument"]; exists {
			if textDocMap, ok := textDoc.(map[string]interface{}); ok {
				if uri, exists := textDocMap["uri"]; exists {
					if uriStr, ok := uri.(string); ok {
						fileURI = uriStr
					}
				}
			}
		}
	}
	
	agg := &ErrorAggregation{
		RequestID:   request.ID,
		Method:      request.Method,
		FileURI:     fileURI,
		Errors:      make([]FallbackError, 0),
		StartTime:   time.Now(),
		LastAttempt: time.Now(),
		FinalError:  originalError,
	}
	
	f.errorHistory[key] = agg
	return agg
}

func (f *FallbackHandler) cleanupErrorAggregation(key string) {
	f.historyLock.Lock()
	defer f.historyLock.Unlock()
	
	delete(f.errorHistory, key)
}

func (f *FallbackHandler) isRecoverableError(err error) bool {
	if err == nil {
		return false
	}
	
	errorStr := err.Error()
	recoverablePatterns := []string{
		"timeout",
		"connection",
		"temporary",
		"not ready",
		"circuit breaker",
	}
	
	for _, pattern := range recoverablePatterns {
		if strings.Contains(strings.ToLower(errorStr), pattern) {
			return true
		}
	}
	
	return false
}

func (f *FallbackHandler) updateMetrics(updateFunc func(*FallbackMetrics)) {
	f.metricsLock.Lock()
	updateFunc(f.fallbackMetrics)
	f.metricsLock.Unlock()
}

func (f *FallbackHandler) updateLatencyMetrics(latency time.Duration) {
	f.metricsLock.Lock()
	defer f.metricsLock.Unlock()
	
	// Simple moving average
	if f.fallbackMetrics.TotalFallbacks == 1 {
		f.fallbackMetrics.AverageLatency = latency
	} else {
		weight := 0.1
		f.fallbackMetrics.AverageLatency = time.Duration(
			float64(f.fallbackMetrics.AverageLatency)*(1-weight) + float64(latency)*weight,
		)
	}
}

// extractProjectContext extracts project ID and language from request or error context
func (f *FallbackHandler) extractProjectContext(request *JSONRPCRequest, originalError error) (string, string) {
	// First try to extract from routing error
	if routingErr, ok := originalError.(*RoutingError); ok {
		return routingErr.ProjectID, f.inferLanguageFromURI(routingErr.FileURI)
	}
	
	// Extract from request parameters
	if params, ok := request.Params.(map[string]interface{}); ok {
		if textDoc, exists := params["textDocument"]; exists {
			if textDocMap, ok := textDoc.(map[string]interface{}); ok {
				if uri, exists := textDocMap["uri"]; exists {
					if uriStr, ok := uri.(string); ok {
						language := f.inferLanguageFromURI(uriStr)
						// Project ID would need to be resolved from URI
						// For now, return empty project ID but valid language
						return "", language
					}
				}
			}
		}
	}
	
	return "", ""
}

// inferLanguageFromRequest infers programming language from request parameters
func (f *FallbackHandler) inferLanguageFromRequest(request *JSONRPCRequest) string {
	if params, ok := request.Params.(map[string]interface{}); ok {
		if textDoc, exists := params["textDocument"]; exists {
			if textDocMap, ok := textDoc.(map[string]interface{}); ok {
				if uri, exists := textDocMap["uri"]; exists {
					if uriStr, ok := uri.(string); ok {
						return f.inferLanguageFromURI(uriStr)
					}
				}
			}
		}
	}
	return ""
}

// inferLanguageFromURI infers programming language from file URI
func (f *FallbackHandler) inferLanguageFromURI(uri string) string {
	if uri == "" {
		return ""
	}
	
	// Simple extension-based language detection
	extensionMap := map[string]string{
		"go":   "go",
		"py":   "python",
		"js":   "javascript",
		"ts":   "typescript",
		"java": "java",
		"rs":   "rust",
		"c":    "c",
		"cpp":  "cpp",
		"cs":   "csharp",
	}
	
	// Extract extension from URI
	if strings.Contains(uri, ".") {
		parts := strings.Split(uri, ".")
		if len(parts) > 1 {
			ext := strings.ToLower(parts[len(parts)-1])
			if language, exists := extensionMap[ext]; exists {
				return language
			}
		}
	}
	
	return ""
}