package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/transport"
	"lsp-gateway/mcp"
)

// EnhancedProjectAwareGateway extends the Gateway with SmartRouter integration and project awareness
// This serves as a bridge between the existing Gateway implementation and the new SmartRouter system
type EnhancedProjectAwareGateway struct {
	*Gateway // Embedded for backward compatibility

	// Enhanced routing components
	smartRouter              *SmartRouterImpl
	workspaceManager         *WorkspaceManager
	projectRouter            *ProjectAwareRouter
	globalMultiServerManager *MultiServerManager // Reference to global multi-server manager

	// Performance monitoring
	performanceCache   PerformanceCache
	requestClassifier  RequestClassifier
	responseAggregator ResponseAggregator
	healthMonitor      *HealthMonitor

	// Configuration
	enableSmartRouting bool
	enableEnhancements bool

	mu sync.RWMutex
}

// NewEnhancedProjectAwareGateway creates a new EnhancedProjectAwareGateway with SmartRouter integration
func NewEnhancedProjectAwareGateway(config *config.GatewayConfig) (*EnhancedProjectAwareGateway, error) {
	// Create base Gateway first
	baseGateway, err := NewGateway(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create base gateway: %w", err)
	}

	// Extract enhanced components from base gateway
	smartRouter := baseGateway.GetSmartRouter()
	workspaceManager := baseGateway.GetWorkspaceManager()
	projectRouter := baseGateway.GetProjectRouter()

	gateway := &EnhancedProjectAwareGateway{
		Gateway:          baseGateway,
		smartRouter:      smartRouter,
		workspaceManager: workspaceManager,
		projectRouter:    projectRouter,

		// Get enhanced components
		performanceCache:   baseGateway.performanceCache,
		requestClassifier:  baseGateway.requestClassifier,
		responseAggregator: baseGateway.responseAggregator,
		healthMonitor:      baseGateway.health_monitor,

		// Configuration
		enableSmartRouting: baseGateway.enableSmartRouting,
		enableEnhancements: baseGateway.enableEnhancements,
	}

	return gateway, nil
}

// HandleJSONRPCWithProjectAwareness provides enhanced JSON-RPC handling with project awareness
func (pag *EnhancedProjectAwareGateway) HandleJSONRPCWithProjectAwareness(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	requestLogger := pag.initializeEnhancedRequestLogger(r)

	if !pag.validateHTTPMethod(w, r, requestLogger) {
		return
	}

	w.Header().Set("Content-Type", HTTPContentTypeJSON)

	// Parse and validate JSON-RPC request
	req, ok := pag.parseAndValidateJSONRPC(w, r, requestLogger)
	if !ok {
		return
	}

	if requestLogger != nil {
		requestLogger = requestLogger.WithField("lsp_method", req.Method)
	}

	// Use enhanced routing if SmartRouter is enabled
	if pag.enableSmartRouting && pag.smartRouter != nil {
		pag.processEnhancedJSONRPCRequest(w, r, req, requestLogger, startTime)
	} else {
		// Fallback to traditional processing
		pag.Gateway.HandleJSONRPC(w, r)
	}
}

// processEnhancedJSONRPCRequest processes requests using SmartRouter capabilities
func (pag *EnhancedProjectAwareGateway) processEnhancedJSONRPCRequest(w http.ResponseWriter, r *http.Request, req JSONRPCRequest, logger *mcp.StructuredLogger, startTime time.Time) {
	// Extract URI and context information
	uri, err := pag.extractURI(req)
	if err != nil {
		if logger != nil {
			logger.WithError(err).Error("Failed to extract URI from request")
		}
		pag.writeError(w, req.ID, InvalidParams, "Invalid parameters", err)
		return
	}

	// Get or create workspace context
	workspace, err := pag.workspaceManager.GetOrCreateWorkspace(uri)
	if err != nil {
		if logger != nil {
			logger.WithError(err).Warn("Failed to get workspace context, using traditional routing")
		}
		// Fallback to traditional routing
		pag.Gateway.HandleJSONRPC(w, r)
		return
	}

	// Enrich logger with workspace context
	if logger != nil {
		logger = logger.WithFields(map[string]interface{}{
			"workspace_id": workspace.GetID(),
			"project_type": workspace.GetProjectType(),
			"project_path": workspace.GetRootPath(),
			"languages":    workspace.GetLanguages(),
		})
	}

	// Classify request for optimal routing
	if pag.requestClassifier != nil {
		jsonrpcReq := &JSONRPCRequest{
			Method:  req.Method,
			Params:  req.Params,
			ID:      req.ID,
			JSONRPC: req.JSONRPC,
		}
		requestClass, err := pag.requestClassifier.ClassifyRequest(jsonrpcReq, uri)
		if err == nil && logger != nil {
			logger.WithField("request_class", requestClass).Debug("Request classified")
		}
	}

	// Route using SmartRouter
	serverName, ok := pag.handleEnhancedRequestRouting(w, req, workspace, logger)
	if !ok {
		return
	}

	if logger != nil {
		logger = logger.WithField("server_name", serverName)
		logger.Debug("Routed request using SmartRouter")
	}

	// Process request with enhanced capabilities
	pag.processLSPRequest(w, r, req, serverName, logger, startTime)
}

// handleEnhancedRequestRouting performs enhanced routing using workspace context
func (pag *EnhancedProjectAwareGateway) handleEnhancedRequestRouting(w http.ResponseWriter, req JSONRPCRequest, workspace *WorkspaceContextImpl, logger *mcp.StructuredLogger) (string, bool) {
	// Extract language from request context
	uri, err := pag.extractURI(req)
	if err != nil {
		if logger != nil {
			logger.WithError(err).Error("Failed to extract URI for routing")
		}
		pag.writeError(w, req.ID, InvalidParams, "Invalid parameters", err)
		return "", false
	}

	language := ""
	if uri != "" {
		language = pag.extractLanguageFromURI(uri)
	}

	// Create LSPRequest for SmartRouter
	lspRequest := &LSPRequest{
		Method:  req.Method,
		Params:  req.Params,
		ID:      req.ID,
		URI:     uri,
		JSONRPC: req.JSONRPC,
		Context: &RequestContext{
			FileURI:     uri,
			Language:    language,
			RequestType: req.Method,
			WorkspaceID: workspace.GetID(),
		},
		Timestamp: time.Now(),
		RequestID: fmt.Sprintf("%v", req.ID),
	}

	// Use SmartRouter to get routing decision
	decision, err := pag.smartRouter.RouteRequest(lspRequest)
	if err != nil {
		// Fallback to project-aware routing
		if logger != nil {
			logger.WithError(err).Warn("SmartRouter failed, using project-aware fallback")
		}

		if pag.projectRouter != nil {
			serverName, err := pag.projectRouter.RouteRequestWithWorkspace(uri)
			if err != nil {
				if logger != nil {
					logger.WithError(err).Error("Project-aware routing failed")
				}
				pag.writeError(w, req.ID, MethodNotFound, "Method not found", err)
				return "", false
			}
			return serverName, true
		}

		// Final fallback to traditional routing
		serverName, err := pag.Gateway.routeRequest(req)
		if err != nil {
			if logger != nil {
				logger.WithError(err).Error("Traditional routing failed")
			}
			pag.writeError(w, req.ID, MethodNotFound, "Method not found", err)
			return "", false
		}
		return serverName, true
	}

	// Update performance cache if available
	if pag.performanceCache != nil && len(decision.TargetServers) > 0 {
		pag.performanceCache.RecordRoutingDecision(req.Method, decision.TargetServers[0].Name, decision.RoutingStrategy)
	}

	// Return the name of the first target server
	if len(decision.TargetServers) > 0 {
		return decision.TargetServers[0].Name, true
	}
	
	return "", false
}

// initializeEnhancedRequestLogger creates an enhanced logger with additional context
func (pag *EnhancedProjectAwareGateway) initializeEnhancedRequestLogger(r *http.Request) *mcp.StructuredLogger {
	baseLogger := pag.Gateway.initializeRequestLogger(r)

	if baseLogger != nil {
		// Add SmartRouter-specific fields
		return baseLogger.WithFields(map[string]interface{}{
			"smart_routing_enabled": pag.enableSmartRouting,
			"enhancements_enabled":  pag.enableEnhancements,
			"gateway_type":          "project_aware",
		})
	}

	return baseLogger
}

// GetWorkspaceContextForURI returns workspace context for a given URI
func (pag *EnhancedProjectAwareGateway) GetWorkspaceContextForURI(uri string) (*WorkspaceContextImpl, error) {
	if pag.workspaceManager == nil {
		return nil, fmt.Errorf("workspace manager not available")
	}

	return pag.workspaceManager.GetOrCreateWorkspace(uri)
}

// GetMultiLanguageWorkspaceInfo returns multi-language information for a workspace
func (pag *EnhancedProjectAwareGateway) GetMultiLanguageWorkspaceInfo(workspaceID string) (*MultiLanguageProjectInfo, error) {
	if pag.workspaceManager == nil {
		return nil, fmt.Errorf("workspace manager not available")
	}

	workspace, exists := pag.workspaceManager.GetWorkspaceByID(workspaceID)
	if !exists {
		return nil, fmt.Errorf("workspace not found: %s", workspaceID)
	}

	return workspace.GetMultiLanguageInfo(), nil
}

// SetActiveLanguageForWorkspace sets the active language for a workspace
func (pag *EnhancedProjectAwareGateway) SetActiveLanguageForWorkspace(workspaceID, language string) error {
	if pag.workspaceManager == nil {
		return fmt.Errorf("workspace manager not available")
	}

	return pag.workspaceManager.SetActiveLanguage(workspaceID, language)
}

// GetLanguageSpecificClient returns a language-specific LSP client for a workspace
func (pag *EnhancedProjectAwareGateway) GetLanguageSpecificClient(workspaceID, language string) (transport.LSPClient, error) {
	if pag.workspaceManager == nil {
		return nil, fmt.Errorf("workspace manager not available")
	}

	return pag.workspaceManager.GetLanguageSpecificClient(workspaceID, language)
}

// GetSmartRoutingMetrics returns SmartRouter performance metrics
func (pag *EnhancedProjectAwareGateway) GetSmartRoutingMetrics() *RoutingMetrics {
	if pag.smartRouter != nil {
		return pag.smartRouter.GetRoutingMetrics()
	}
	return nil
}

// IsSmartRoutingAvailable returns whether SmartRouter is available and enabled
func (pag *EnhancedProjectAwareGateway) IsSmartRoutingAvailable() bool {
	return pag.enableSmartRouting && pag.smartRouter != nil
}

// IsEnhancementsAvailable returns whether enhancements are available and enabled
func (pag *EnhancedProjectAwareGateway) IsEnhancementsAvailable() bool {
	return pag.enableEnhancements && pag.performanceCache != nil && pag.requestClassifier != nil
}

// Start starts the ProjectAwareGateway and all its components
func (pag *EnhancedProjectAwareGateway) Start(ctx context.Context) error {
	// Start the base gateway first
	if err := pag.Gateway.Start(ctx); err != nil {
		return fmt.Errorf("failed to start base gateway: %w", err)
	}

	// Start enhanced components if available
	if pag.healthMonitor != nil {
		go pag.healthMonitor.StartMonitoring(ctx)
	}

	if pag.Logger != nil {
		pag.Logger.Info("ProjectAwareGateway started successfully with SmartRouter integration")
	}

	return nil
}

// Stop stops the ProjectAwareGateway and all its components
func (pag *EnhancedProjectAwareGateway) Stop() error {
	// Stop enhanced components first
	if pag.healthMonitor != nil {
		pag.healthMonitor.StopMonitoring()
	}

	// Stop the base gateway
	return pag.Gateway.Stop()
}

// ValidateSmartRouterIntegration validates that SmartRouter integration is working correctly
func (pag *EnhancedProjectAwareGateway) ValidateSmartRouterIntegration() error {
	if !pag.enableSmartRouting {
		return fmt.Errorf("SmartRouter is not enabled")
	}

	if pag.smartRouter == nil {
		return fmt.Errorf("SmartRouter instance is nil")
	}

	if pag.workspaceManager == nil {
		return fmt.Errorf("WorkspaceManager instance is nil")
	}

	if pag.projectRouter == nil {
		return fmt.Errorf("ProjectAwareRouter instance is nil")
	}

	// Validate multi-language workspace support
	if err := pag.workspaceManager.ValidateMultiLanguageSupport(); err != nil {
		return fmt.Errorf("multi-language support validation failed: %w", err)
	}

	if pag.Logger != nil {
		pag.Logger.Info("SmartRouter integration validation passed")
	}

	return nil
}

// SetGlobalMultiServerManager sets the global multi-server manager reference
func (pag *EnhancedProjectAwareGateway) SetGlobalMultiServerManager(globalManager *MultiServerManager) {
	pag.mu.Lock()
	defer pag.mu.Unlock()
	pag.globalMultiServerManager = globalManager
}

// processWorkspaceAwareLSPRequest processes LSP requests with workspace awareness
func (pag *EnhancedProjectAwareGateway) processWorkspaceAwareLSPRequest(req *WorkspaceAwareJSONRPCRequest, w http.ResponseWriter) error {
	workspace, exists := pag.workspaceManager.GetWorkspaceByID(req.WorkspaceID)
	if !exists {
		return fmt.Errorf("workspace not found: %s", req.WorkspaceID)
	}

	// Use workspace server manager if available
	if workspace.ServerManager != nil {
		return pag.processWorkspaceMultiServerRequest(req, workspace, w)
	}

	// Fallback to existing single-server logic
	return pag.processWorkspaceSingleServerRequest(req, workspace, w)
}

// processWorkspaceMultiServerRequest processes requests using workspace server manager
func (pag *EnhancedProjectAwareGateway) processWorkspaceMultiServerRequest(req *WorkspaceAwareJSONRPCRequest, workspace *WorkspaceContextImpl, w http.ResponseWriter) error {
	language := pag.extractLanguageFromURI(fmt.Sprintf("%v", req.Params))

	// Create workspace request router
	router := NewWorkspaceRequestRouter(workspace.ServerManager)

	// Get appropriate servers from workspace manager
	servers, err := router.RouteRequest(language, req.Method, false)
	if err != nil {
		return fmt.Errorf("failed to route request in workspace %s: %w", workspace.ID, err)
	}

	// Execute request based on number of servers
	if len(servers) == 1 {
		return pag.executeSingleServerRequest(servers[0], req, w)
	}

	return pag.executeConcurrentServerRequest(servers, req, w)
}

// processWorkspaceSingleServerRequest processes requests using single server fallback
func (pag *EnhancedProjectAwareGateway) processWorkspaceSingleServerRequest(req *WorkspaceAwareJSONRPCRequest, workspace *WorkspaceContextImpl, w http.ResponseWriter) error {
	language := pag.extractLanguageFromURI(fmt.Sprintf("%v", req.Params))

	// Get language-specific client using existing workspace manager methods
	client, err := pag.workspaceManager.GetLanguageSpecificClient(workspace.ID, language)
	if err != nil {
		return fmt.Errorf("failed to get LSP client for language %s: %w", language, err)
	}

	// Execute single request
	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := client.SendRequest(ctx, req.Method, req.Params)

	// Record request if workspace server manager is available
	if workspace.ServerManager != nil {
		record := RequestRecord{
			RequestID:  fmt.Sprintf("%v", req.ID),
			Language:   language,
			Method:     req.Method,
			StartTime:  startTime,
			EndTime:    time.Now(),
			Duration:   time.Since(startTime),
			Success:    err == nil,
			ServerName: "single-server-fallback",
		}
		if err != nil {
			record.Error = err.Error()
		}
		workspace.ServerManager.RecordRequest(record)
	}

	if err != nil {
		return fmt.Errorf("LSP request failed: %w", err)
	}

	// Send response
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}

	return json.NewEncoder(w).Encode(response)
}

// executeSingleServerRequest executes a request on a single server
func (pag *EnhancedProjectAwareGateway) executeSingleServerRequest(server *ServerInstance, req *WorkspaceAwareJSONRPCRequest, w http.ResponseWriter) error {
	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := server.SendRequest(ctx, req.Method, req.Params)
	if err != nil {
		return fmt.Errorf("server request failed: %w", err)
	}

	// Send response
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}

	duration := time.Since(startTime)
	if pag.Logger != nil {
		pag.Logger.Infof("Single server request completed in %v (server: %s)", duration, server.config.Name)
	}

	return json.NewEncoder(w).Encode(response)
}

// executeConcurrentServerRequest executes a request on multiple servers concurrently
func (pag *EnhancedProjectAwareGateway) executeConcurrentServerRequest(servers []*ServerInstance, req *WorkspaceAwareJSONRPCRequest, w http.ResponseWriter) error {
	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Execute requests concurrently
	type serverResult struct {
		result json.RawMessage
		err    error
		server string
	}

	resultChan := make(chan serverResult, len(servers))

	for _, server := range servers {
		go func(srv *ServerInstance) {
			result, err := srv.SendRequest(ctx, req.Method, req.Params)
			resultChan <- serverResult{
				result: result,
				err:    err,
				server: srv.config.Name,
			}
		}(server)
	}

	// Collect results
	var results []json.RawMessage
	var errors []string
	successCount := 0

	for i := 0; i < len(servers); i++ {
		select {
		case res := <-resultChan:
			if res.err != nil {
				errors = append(errors, fmt.Sprintf("server %s: %v", res.server, res.err))
			} else {
				results = append(results, res.result)
				successCount++
			}
		case <-ctx.Done():
			return fmt.Errorf("concurrent request timeout")
		}
	}

	// Return error if no servers succeeded
	if successCount == 0 {
		return fmt.Errorf("all servers failed: %v", errors)
	}

	// Aggregate results (simple approach: return first successful result)
	var finalResult json.RawMessage
	if len(results) > 0 {
		finalResult = results[0]
	}

	// If method supports result aggregation, combine results
	if req.Method == "workspace/symbol" || req.Method == "textDocument/references" {
		if aggregatedResult, err := pag.aggregateResults(req.Method, results); err == nil {
			finalResult = aggregatedResult
		}
	}

	// Send response
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  finalResult,
	}

	duration := time.Since(startTime)
	if pag.Logger != nil {
		pag.Logger.Infof("Concurrent server request completed in %v (%d/%d servers succeeded)",
			duration, successCount, len(servers))
	}

	return json.NewEncoder(w).Encode(response)
}

// aggregateResults aggregates results from multiple servers
func (pag *EnhancedProjectAwareGateway) aggregateResults(method string, results []json.RawMessage) (json.RawMessage, error) {
	switch method {
	case "workspace/symbol":
		return pag.aggregateSymbolResults(results)
	case "textDocument/references":
		return pag.aggregateReferenceResults(results)
	default:
		// Return first result for methods that don't support aggregation
		if len(results) > 0 {
			return results[0], nil
		}
		return nil, fmt.Errorf("no results to aggregate")
	}
}

// aggregateSymbolResults aggregates workspace symbol results
func (pag *EnhancedProjectAwareGateway) aggregateSymbolResults(results []json.RawMessage) (json.RawMessage, error) {
	var allSymbols []interface{}

	for _, result := range results {
		var symbols []interface{}
		if err := json.Unmarshal(result, &symbols); err == nil {
			allSymbols = append(allSymbols, symbols...)
		}
	}

	return json.Marshal(allSymbols)
}

// aggregateReferenceResults aggregates reference results
func (pag *EnhancedProjectAwareGateway) aggregateReferenceResults(results []json.RawMessage) (json.RawMessage, error) {
	var allReferences []interface{}

	for _, result := range results {
		var references []interface{}
		if err := json.Unmarshal(result, &references); err == nil {
			allReferences = append(allReferences, references...)
		}
	}

	return json.Marshal(allReferences)
}

// extractLanguageFromURI extracts language from URI or parameters
func (pag *EnhancedProjectAwareGateway) extractLanguageFromURI(uriOrParams interface{}) string {
	// Try to extract URI from string parameter
	if uri, ok := uriOrParams.(string); ok && uri != "" {
		// Use base Gateway method which handles proper language detection
		if lang, err := pag.Gateway.extractLanguageFromURI(uri); err == nil {
			return lang
		}
	}
	
	// Fallback: try to extract from structured parameters
	if params, ok := uriOrParams.(map[string]interface{}); ok {
		if textDoc, ok := params["textDocument"].(map[string]interface{}); ok {
			if uri, ok := textDoc["uri"].(string); ok {
				if lang, err := pag.Gateway.extractLanguageFromURI(uri); err == nil {
					return lang
				}
			}
		}
	}
	
	return "go" // Default language for demonstration
}

