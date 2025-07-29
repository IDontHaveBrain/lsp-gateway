package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"lsp-gateway/internal/transport"
	"lsp-gateway/mcp"
)

// SingleTargetStrategy routes requests to the best matching sub-project client
type SingleTargetStrategy struct {
	name    string
	timeout time.Duration
	logger  *mcp.StructuredLogger
}

// NewSingleTargetStrategy creates a new single target routing strategy
func NewSingleTargetStrategy(logger *mcp.StructuredLogger) RoutingStrategy {
	return &SingleTargetStrategy{
		name:    "single_target",
		timeout: 10 * time.Second,
		logger:  logger,
	}
}

func (s *SingleTargetStrategy) Name() string {
	return s.name
}

func (s *SingleTargetStrategy) Priority() int {
	return 100 // Highest priority - primary routing strategy
}

func (s *SingleTargetStrategy) GetTimeout() time.Duration {
	return s.timeout
}

func (s *SingleTargetStrategy) CanHandle(method string) bool {
	// Can handle all supported LSP methods
	supportedMethods := map[string]bool{
		"textDocument/definition":    true,
		"textDocument/references":    true,
		"textDocument/hover":         true,
		"textDocument/documentSymbol": true,
		"textDocument/completion":    true,
		"workspace/symbol":           true,
	}
	return supportedMethods[method]
}

func (s *SingleTargetStrategy) SupportsSubProject(project *SubProject) bool {
	// Can support any sub-project with active LSP clients
	if project == nil {
		return false
	}
	return len(project.LSPClients) > 0 || len(project.Languages) > 0
}

func (s *SingleTargetStrategy) Route(ctx context.Context, decision *RoutingDecision) (*JSONRPCResponse, error) {
	if decision.PrimaryClient == nil {
		return nil, fmt.Errorf("no primary client available for routing")
	}
	
	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	
	if s.logger != nil {
		s.logger.WithFields(map[string]interface{}{
			"strategy":     s.name,
			"method":       decision.Method,
			"project_id":   decision.TargetProject.ID,
			"file_uri":     decision.FileURI,
		}).Debug("Executing single target routing strategy")
	}
	
	// Send request to primary client
	result, err := decision.PrimaryClient.SendRequest(timeoutCtx, decision.Method, decision.Context)
	if err != nil {
		return nil, fmt.Errorf("primary client request failed: %w", err)
	}
	
	// Create successful response
	response := &JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      decision.RequestID,
		Result:  json.RawMessage(result),
	}
	
	if s.logger != nil {
		s.logger.WithFields(map[string]interface{}{
			"strategy":   s.name,
			"method":     decision.Method,
			"project_id": decision.TargetProject.ID,
		}).Debug("Single target routing completed successfully")
	}
	
	return response, nil
}

// HierarchicalFallbackStrategy tries parent/child project clients when primary fails
type HierarchicalFallbackStrategy struct {
	name    string
	timeout time.Duration
	logger  *mcp.StructuredLogger
}

// NewHierarchicalFallbackStrategy creates a new hierarchical fallback strategy
func NewHierarchicalFallbackStrategy(logger *mcp.StructuredLogger) RoutingStrategy {
	return &HierarchicalFallbackStrategy{
		name:    "hierarchical_fallback",
		timeout: 15 * time.Second, // Longer timeout for fallback attempts
		logger:  logger,
	}
}

func (h *HierarchicalFallbackStrategy) Name() string {
	return h.name
}

func (h *HierarchicalFallbackStrategy) Priority() int {
	return 80 // High priority fallback
}

func (h *HierarchicalFallbackStrategy) GetTimeout() time.Duration {
	return h.timeout
}

func (h *HierarchicalFallbackStrategy) CanHandle(method string) bool {
	// Can handle most LSP methods except workspace-level ones
	unsupportedMethods := map[string]bool{
		"workspace/symbol": true, // Better handled by workspace-level strategy
	}
	return !unsupportedMethods[method]
}

func (h *HierarchicalFallbackStrategy) SupportsSubProject(project *SubProject) bool {
	if project == nil {
		return false
	}
	// Supports projects with parent/child relationships
	return project.Parent != nil || len(project.Children) > 0
}

func (h *HierarchicalFallbackStrategy) Route(ctx context.Context, decision *RoutingDecision) (*JSONRPCResponse, error) {
	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()
	
	if h.logger != nil {
		h.logger.WithFields(map[string]interface{}{
			"strategy":     h.name,
			"method":       decision.Method,
			"project_id":   decision.TargetProject.ID,
			"fallback_count": len(decision.FallbackClients),
		}).Debug("Executing hierarchical fallback routing strategy")
	}
	
	// Try primary client first
	if decision.PrimaryClient != nil {
		result, err := decision.PrimaryClient.SendRequest(timeoutCtx, decision.Method, decision.Context)
		if err == nil {
			response := &JSONRPCResponse{
				JSONRPC: JSONRPCVersion,
				ID:      decision.RequestID,
				Result:  json.RawMessage(result),
			}
			return response, nil
		}
		
		if h.logger != nil {
			h.logger.WithError(err).WithField("client", "primary").Debug("Primary client failed, trying fallbacks")
		}
	}
	
	// Try fallback clients in order
	var lastError error
	for i, client := range decision.FallbackClients {
		if client == nil {
			continue
		}
		
		// Check if timeout has been exceeded
		select {
		case <-timeoutCtx.Done():
			return nil, fmt.Errorf("hierarchical fallback timeout exceeded")
		default:
		}
		
		result, err := client.SendRequest(timeoutCtx, decision.Method, decision.Context)
		if err == nil {
			response := &JSONRPCResponse{
				JSONRPC: JSONRPCVersion,
				ID:      decision.RequestID,
				Result:  json.RawMessage(result),
			}
			
			if h.logger != nil {
				h.logger.WithFields(map[string]interface{}{
					"strategy":        h.name,
					"fallback_index":  i,
					"method":          decision.Method,
				}).Info("Hierarchical fallback succeeded")
			}
			
			return response, nil
		}
		
		lastError = err
		if h.logger != nil {
			h.logger.WithError(err).WithField("fallback_index", i).Debug("Fallback client failed")
		}
	}
	
	return nil, fmt.Errorf("all hierarchical fallback clients failed, last error: %w", lastError)
}

// WorkspaceLevelFallbackStrategy falls back to workspace-level clients
type WorkspaceLevelFallbackStrategy struct {
	name    string
	timeout time.Duration
	logger  *mcp.StructuredLogger
}

// NewWorkspaceLevelFallbackStrategy creates a new workspace-level fallback strategy
func NewWorkspaceLevelFallbackStrategy(logger *mcp.StructuredLogger) RoutingStrategy {
	return &WorkspaceLevelFallbackStrategy{
		name:    "workspace_level_fallback",
		timeout: 20 * time.Second, // Even longer timeout for workspace fallback
		logger:  logger,
	}
}

func (w *WorkspaceLevelFallbackStrategy) Name() string {
	return w.name
}

func (w *WorkspaceLevelFallbackStrategy) Priority() int {
	return 60 // Medium priority - used when hierarchical fails
}

func (w *WorkspaceLevelFallbackStrategy) GetTimeout() time.Duration {
	return w.timeout
}

func (w *WorkspaceLevelFallbackStrategy) CanHandle(method string) bool {
	// Can handle all methods as a last resort
	return true
}

func (w *WorkspaceLevelFallbackStrategy) SupportsSubProject(project *SubProject) bool {
	// Can support any project as a fallback
	return project != nil
}

func (w *WorkspaceLevelFallbackStrategy) Route(ctx context.Context, decision *RoutingDecision) (*JSONRPCResponse, error) {
	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()
	
	if w.logger != nil {
		w.logger.WithFields(map[string]interface{}{
			"strategy":   w.name,
			"method":     decision.Method,
			"project_id": decision.TargetProject.ID,
		}).Debug("Executing workspace-level fallback strategy")
	}
	
	// For workspace-level fallback, we try all available clients
	clients := []transport.LSPClient{decision.PrimaryClient}
	clients = append(clients, decision.FallbackClients...)
	
	var lastError error
	for i, client := range clients {
		if client == nil {
			continue
		}
		
		// Check timeout
		select {
		case <-timeoutCtx.Done():
			return nil, fmt.Errorf("workspace-level fallback timeout exceeded")
		default:
		}
		
		result, err := client.SendRequest(timeoutCtx, decision.Method, decision.Context)
		if err == nil {
			response := &JSONRPCResponse{
				JSONRPC: JSONRPCVersion,
				ID:      decision.RequestID,
				Result:  json.RawMessage(result),
			}
			
			if w.logger != nil {
				w.logger.WithFields(map[string]interface{}{
					"strategy":    w.name,
					"client_index": i,
					"method":      decision.Method,
				}).Info("Workspace-level fallback succeeded")
			}
			
			return response, nil
		}
		
		lastError = err
		if w.logger != nil {
			w.logger.WithError(err).WithField("client_index", i).Debug("Workspace-level client failed")
		}
	}
	
	return nil, fmt.Errorf("all workspace-level fallback clients failed, last error: %w", lastError)
}

// CachedResponseStrategy returns cached responses for read operations when clients fail
type CachedResponseStrategy struct {
	name    string
	timeout time.Duration
	logger  *mcp.StructuredLogger
	cache   map[string]*CachedResponse
}

// CachedResponse represents a cached LSP response
type CachedResponse struct {
	Response  *JSONRPCResponse `json:"response"`
	Timestamp time.Time        `json:"timestamp"`
	TTL       time.Duration    `json:"ttl"`
	Method    string           `json:"method"`
	FileURI   string           `json:"file_uri"`
}

// NewCachedResponseStrategy creates a new cached response strategy
func NewCachedResponseStrategy(logger *mcp.StructuredLogger) RoutingStrategy {
	return &CachedResponseStrategy{
		name:    "cached_response",
		timeout: 1 * time.Second, // Very fast - just cache lookup
		logger:  logger,
		cache:   make(map[string]*CachedResponse),
	}
}

func (c *CachedResponseStrategy) Name() string {
	return c.name
}

func (c *CachedResponseStrategy) Priority() int {
	return 40 // Lower priority - used as last resort
}

func (c *CachedResponseStrategy) GetTimeout() time.Duration {
	return c.timeout
}

func (c *CachedResponseStrategy) CanHandle(method string) bool {
	// Only handle read-only operations that can be safely cached
	readOnlyMethods := map[string]bool{
		"textDocument/definition":    true,
		"textDocument/references":    true,
		"textDocument/hover":         true,
		"textDocument/documentSymbol": true,
		"workspace/symbol":           true,
		// Not completion - that's too dynamic
	}
	return readOnlyMethods[method]
}

func (c *CachedResponseStrategy) SupportsSubProject(project *SubProject) bool {
	// Can provide cached responses for any project
	return project != nil
}

func (c *CachedResponseStrategy) Route(ctx context.Context, decision *RoutingDecision) (*JSONRPCResponse, error) {
	if c.logger != nil {
		c.logger.WithFields(map[string]interface{}{
			"strategy":   c.name,
			"method":     decision.Method,
			"file_uri":   decision.FileURI,
		}).Debug("Checking cached response strategy")
	}
	
	// Generate cache key
	cacheKey := fmt.Sprintf("%s:%s:%s", decision.Method, decision.TargetProject.ID, decision.FileURI)
	
	// Check for cached response
	if cached, exists := c.cache[cacheKey]; exists {
		// Check if cache entry is still valid
		if time.Since(cached.Timestamp) < cached.TTL {
			if c.logger != nil {
				c.logger.WithFields(map[string]interface{}{
					"strategy":  c.name,
					"cache_key": cacheKey,
					"age":       time.Since(cached.Timestamp).String(),
				}).Info("Returning cached response")
			}
			
			// Return cached response with updated ID
			response := *cached.Response
			response.ID = decision.RequestID
			return &response, nil
		} else {
			// Remove expired cache entry
			delete(c.cache, cacheKey)
		}
	}
	
	// No valid cached response available
	return nil, fmt.Errorf("no cached response available for %s", cacheKey)
}

// CacheResponse stores a response in the cache for future use
func (c *CachedResponseStrategy) CacheResponse(method, projectID, fileURI string, response *JSONRPCResponse, ttl time.Duration) {
	cacheKey := fmt.Sprintf("%s:%s:%s", method, projectID, fileURI)
	
	cached := &CachedResponse{
		Response:  response,
		Timestamp: time.Now(),
		TTL:       ttl,
		Method:    method,
		FileURI:   fileURI,
	}
	
	c.cache[cacheKey] = cached
	
	if c.logger != nil {
		c.logger.WithFields(map[string]interface{}{
			"cache_key": cacheKey,
			"ttl":       ttl.String(),
		}).Debug("Response cached for future use")
	}
}

// ClearCache removes all cached responses
func (c *CachedResponseStrategy) ClearCache() {
	c.cache = make(map[string]*CachedResponse)
	
	if c.logger != nil {
		c.logger.Info("Response cache cleared")
	}
}

// GetCacheStats returns cache statistics
func (c *CachedResponseStrategy) GetCacheStats() map[string]interface{} {
	now := time.Now()
	valid := 0
	expired := 0
	
	for _, cached := range c.cache {
		if now.Sub(cached.Timestamp) < cached.TTL {
			valid++
		} else {
			expired++
		}
	}
	
	return map[string]interface{}{
		"total_entries":   len(c.cache),
		"valid_entries":   valid,
		"expired_entries": expired,
		"cache_hit_rate":  "N/A", // Would need request tracking to calculate
	}
}

// ReadOnlyModeStrategy provides basic responses when all else fails
type ReadOnlyModeStrategy struct {
	name    string
	timeout time.Duration
	logger  *mcp.StructuredLogger
}

// NewReadOnlyModeStrategy creates a new read-only mode strategy
func NewReadOnlyModeStrategy(logger *mcp.StructuredLogger) RoutingStrategy {
	return &ReadOnlyModeStrategy{
		name:    "read_only_mode",
		timeout: 100 * time.Millisecond, // Very fast - just return empty response
		logger:  logger,
	}
}

func (r *ReadOnlyModeStrategy) Name() string {
	return r.name
}

func (r *ReadOnlyModeStrategy) Priority() int {
	return 10 // Lowest priority - absolute last resort
}

func (r *ReadOnlyModeStrategy) GetTimeout() time.Duration {
	return r.timeout
}

func (r *ReadOnlyModeStrategy) CanHandle(method string) bool {
	// Can provide basic responses for read operations
	readOnlyMethods := map[string]bool{
		"textDocument/definition":    true,
		"textDocument/references":    true,
		"textDocument/hover":         true,
		"textDocument/documentSymbol": true,
		"workspace/symbol":           true,
	}
	return readOnlyMethods[method]
}

func (r *ReadOnlyModeStrategy) SupportsSubProject(project *SubProject) bool {
	// Can provide basic responses for any project
	return project != nil
}

func (r *ReadOnlyModeStrategy) Route(ctx context.Context, decision *RoutingDecision) (*JSONRPCResponse, error) {
	if r.logger != nil {
		r.logger.WithFields(map[string]interface{}{
			"strategy": r.name,
			"method":   decision.Method,
		}).Warn("Using read-only mode strategy - LSP functionality may be limited")
	}
	
	// Return empty response based on method type
	var result interface{}
	
	switch decision.Method {
	case "textDocument/definition":
		result = []interface{}{} // Empty locations array
	case "textDocument/references":
		result = []interface{}{} // Empty locations array
	case "textDocument/hover":
		result = nil // No hover information
	case "textDocument/documentSymbol":
		result = []interface{}{} // Empty symbols array
	case "workspace/symbol":
		result = []interface{}{} // Empty symbols array
	default:
		result = nil
	}
	
	// Marshal result to JSON
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal read-only response: %w", err)
	}
	
	response := &JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      decision.RequestID,
		Result:  json.RawMessage(resultBytes),
	}
	
	if r.logger != nil {
		r.logger.WithField("method", decision.Method).Info("Read-only mode response provided")
	}
	
	return response, nil
}