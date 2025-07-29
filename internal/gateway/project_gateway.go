package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/transport"
	"lsp-gateway/mcp"
)

const (
	LoggerComponentProjectGateway = "project-gateway"
)

// ProjectAwareGateway extends the existing Gateway with project/workspace awareness
type ProjectAwareGateway struct {
	*Gateway         // Embedded traditional gateway for backward compatibility
	WorkspaceManager *WorkspaceManager
	ProjectRouter    *ProjectAwareRouter
	Logger           *mcp.StructuredLogger
	mu               sync.RWMutex
}

// NewProjectAwareGateway creates a new ProjectAwareGateway with workspace management capabilities
func NewProjectAwareGateway(config *config.GatewayConfig) (*ProjectAwareGateway, error) {
	// Create traditional gateway first
	gateway, err := NewGateway(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create base gateway: %w", err)
	}

	// Create logger with project-aware component
	logConfig := &mcp.LoggerConfig{
		Level:              mcp.LogLevelInfo,
		Component:          LoggerComponentProjectGateway,
		EnableJSON:         false,
		EnableStackTrace:   false,
		EnableCaller:       true,
		EnableMetrics:      false,
		Output:             nil, // Uses default (stderr)
		IncludeTimestamp:   true,
		TimestampFormat:    TimestampFormatISO8601,
		MaxStackTraceDepth: 10,
		EnableAsyncLogging: false,
		AsyncBufferSize:    1000,
	}
	logger := mcp.NewStructuredLogger(logConfig)

	// Create workspace manager
	workspaceManager := NewWorkspaceManager(config, gateway.Router, logger)

	// Create project-aware router
	projectRouter := NewProjectAwareRouter(gateway.Router, workspaceManager, logger)

	projectGateway := &ProjectAwareGateway{
		Gateway:          gateway,
		WorkspaceManager: workspaceManager,
		ProjectRouter:    projectRouter,
		Logger:           logger,
	}

	logger.Infof("ProjectAwareGateway initialized successfully with %d servers, max %d workspaces", len(config.Servers), MaxConcurrentWorkspaces)

	return projectGateway, nil
}

// Start starts the project-aware gateway and all its components
func (pg *ProjectAwareGateway) Start(ctx context.Context) error {
	pg.mu.Lock()
	defer pg.mu.Unlock()

	if pg.Logger != nil {
		pg.Logger.Info("Starting ProjectAwareGateway with workspace management")
	}

	// Start the underlying gateway
	if err := pg.Gateway.Start(ctx); err != nil {
		return fmt.Errorf("failed to start underlying gateway: %w", err)
	}

	// Workspace manager starts automatically via its cleanup routine

	if pg.Logger != nil {
		pg.Logger.Info("ProjectAwareGateway started successfully")
	}

	return nil
}

// Stop stops the project-aware gateway and cleans up all workspaces
func (pg *ProjectAwareGateway) Stop() error {
	pg.mu.Lock()
	defer pg.mu.Unlock()

	if pg.Logger != nil {
		pg.Logger.Info("Stopping ProjectAwareGateway and cleaning up workspaces")
	}

	var errors []error

	// Shutdown workspace manager first
	if err := pg.WorkspaceManager.Shutdown(); err != nil {
		pg.Logger.WithError(err).Error("Failed to shutdown WorkspaceManager")
		errors = append(errors, fmt.Errorf("workspace manager shutdown failed: %w", err))
	}

	// Stop the underlying gateway
	if err := pg.Gateway.Stop(); err != nil {
		pg.Logger.WithError(err).Error("Failed to stop underlying Gateway")
		errors = append(errors, fmt.Errorf("underlying gateway stop failed: %w", err))
	}

	if pg.Logger != nil {
		pg.Logger.Info("ProjectAwareGateway stopped successfully")
	}

	// Return first error if any occurred
	if len(errors) > 0 {
		return errors[0]
	}
	return nil
}

// HandleJSONRPC handles JSON-RPC requests with workspace awareness
func (pg *ProjectAwareGateway) HandleJSONRPC(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	requestLogger := pg.initializeProjectRequestLogger(r)

	if !pg.validateHTTPMethod(w, r, requestLogger) {
		return
	}

	w.Header().Set("Content-Type", HTTPContentTypeJSON)

	req, ok := pg.parseAndValidateJSONRPC(w, r, requestLogger)
	if !ok {
		return
	}

	if requestLogger != nil {
		requestLogger = requestLogger.WithField("lsp_method", req.Method)
	}

	// Handle workspace-aware routing
	workspaceContext, serverName, ok := pg.handleProjectAwareRouting(w, req, requestLogger)
	if !ok {
		return
	}

	if requestLogger != nil {
		requestLogger = requestLogger.WithFields(map[string]interface{}{
			LoggerFieldServerName:  serverName,
			LoggerFieldWorkspaceID: workspaceContext.GetID(),
			LoggerFieldProjectType: workspaceContext.GetProjectType(),
		})
		requestLogger.Debug("Routed request to workspace-aware LSP server")
	}

	// Process the request with workspace context
	pg.processWorkspaceAwareLSPRequest(w, r, req, workspaceContext, serverName, requestLogger, startTime)
}

// GetWorkspaceClient retrieves an LSP client for a specific workspace and language
func (pg *ProjectAwareGateway) GetWorkspaceClient(workspaceID, language string) (transport.LSPClient, error) {
	return pg.WorkspaceManager.GetLSPClient(workspaceID, language)
}

// GetWorkspaceContext retrieves or creates workspace context for a file URI
func (pg *ProjectAwareGateway) GetWorkspaceContext(fileURI string) (WorkspaceContext, error) {
	workspace, err := pg.WorkspaceManager.GetOrCreateWorkspace(fileURI)
	if err != nil {
		return nil, err
	}
	return workspace, nil
}

// GetAllWorkspaces returns information about all active workspaces
func (pg *ProjectAwareGateway) GetAllWorkspaces() []WorkspaceContext {
	workspaces := pg.WorkspaceManager.GetAllWorkspaces()
	result := make([]WorkspaceContext, len(workspaces))
	for i, workspace := range workspaces {
		result[i] = workspace
	}
	return result
}

// CleanupWorkspace manually cleans up a specific workspace
func (pg *ProjectAwareGateway) CleanupWorkspace(workspaceID string) error {
	return pg.WorkspaceManager.CleanupWorkspace(workspaceID)
}

// GetClient maintains backward compatibility by delegating to traditional Gateway
func (pg *ProjectAwareGateway) GetClient(serverName string) (transport.LSPClient, bool) {
	return pg.Gateway.GetClient(serverName)
}

// handleProjectAwareRouting performs workspace-aware request routing
func (pg *ProjectAwareGateway) handleProjectAwareRouting(w http.ResponseWriter, req JSONRPCRequest, logger *mcp.StructuredLogger) (WorkspaceContext, string, bool) {
	// Extract URI from request for workspace discovery
	uri, err := pg.extractURIForWorkspace(req)
	if err != nil {
		if logger != nil {
			logger.WithError(err).Warn("Failed to extract URI for workspace routing, falling back to traditional routing")
		}
		// Fallback to traditional routing
		serverName, ok := pg.handleTraditionalRouting(w, req, logger)
		if !ok {
			return nil, "", false
		}
		// Create a temporary workspace context for non-workspace-aware requests
		tempWorkspace := &WorkspaceContextImpl{
			ID:           "traditional",
			RootPath:     "/",
			ProjectType:  "generic",
			ProjectName:  "traditional",
			Languages:    []string{"text"},
			IsActiveFlag: true,
		}
		return tempWorkspace, serverName, true
	}

	// Get or create workspace context
	workspace, err := pg.WorkspaceManager.GetOrCreateWorkspace(uri)
	if err != nil {
		if logger != nil {
			logger.WithError(err).Warn("Failed to get workspace context, falling back to traditional routing")
		}
		// Fallback to traditional routing
		serverName, ok := pg.handleTraditionalRouting(w, req, logger)
		if !ok {
			return nil, "", false
		}
		// Create a temporary workspace context
		tempWorkspace := &WorkspaceContextImpl{
			ID:           "fallback",
			RootPath:     "/",
			ProjectType:  "generic",
			ProjectName:  "fallback",
			Languages:    []string{"text"},
			IsActiveFlag: true,
		}
		return tempWorkspace, serverName, true
	}

	// Use project-aware routing to select server
	serverName, err := pg.ProjectRouter.RouteRequestWithWorkspace(uri)
	if err != nil {
		if logger != nil {
			logger.WithError(err).Error("Failed to route request with workspace awareness")
		}
		pg.writeError(w, req.ID, MethodNotFound, "Method not found", err)
		return nil, "", false
	}

	return workspace, serverName, true
}

// handleTraditionalRouting performs traditional request routing for fallback cases
func (pg *ProjectAwareGateway) handleTraditionalRouting(w http.ResponseWriter, req JSONRPCRequest, logger *mcp.StructuredLogger) (string, bool) {
	serverName, err := pg.routeRequest(req)
	if err != nil {
		if logger != nil {
			logger.WithError(err).Error("Failed to route request using traditional routing")
		}
		pg.writeError(w, req.ID, MethodNotFound, "Method not found", err)
		return "", false
	}
	return serverName, true
}

// processWorkspaceAwareLSPRequest processes an LSP request with workspace context
func (pg *ProjectAwareGateway) processWorkspaceAwareLSPRequest(w http.ResponseWriter, r *http.Request, req JSONRPCRequest, workspace WorkspaceContext, serverName string, logger *mcp.StructuredLogger, startTime time.Time) {
	// For workspace-aware requests, try to get workspace-specific client first
	var client transport.LSPClient
	var exists bool

	if workspace.GetID() != "traditional" && workspace.GetID() != "fallback" {
		// Determine language from the request URI
		uri, err := pg.extractURIForWorkspace(req)
		if err == nil {
			language := pg.extractLanguageFromURI(uri)
			if language != "" {
				// Try to get workspace-specific client
				workspaceClient, err := pg.WorkspaceManager.GetLSPClient(workspace.GetID(), language)
				if err == nil && workspaceClient != nil && workspaceClient.IsActive() {
					client = workspaceClient
					exists = true
					if logger != nil {
						logger.Debugf("Using workspace-specific LSP client for language: %s", language)
					}
				}
			}
		}
	}

	// Fallback to traditional client if workspace-specific client not available
	if !exists {
		client, exists = pg.Gateway.GetClient(serverName)
		if logger != nil && exists {
			logger.Debug("Using traditional LSP client")
		}
	}

	// Validate client availability
	if !exists {
		if logger != nil {
			logger.Error("LSP server not found")
		}
		pg.writeError(w, req.ID, InternalError, ERROR_INTERNAL,
			fmt.Errorf(ERROR_SERVER_NOT_FOUND, serverName))
		return
	}

	if !client.IsActive() {
		if logger != nil {
			logger.Error("LSP server is not active")
		}
		pg.writeError(w, req.ID, InternalError, "Internal error",
			fmt.Errorf("server %s is not active", serverName))
		return
	}

	// Process the request
	if req.ID == nil {
		pg.handleWorkspaceNotification(w, r, req, client, logger, startTime)
		return
	}

	pg.handleWorkspaceRequest(w, r, req, client, logger, startTime)
}

// extractURIForWorkspace extracts URI from request parameters for workspace operations
func (pg *ProjectAwareGateway) extractURIForWorkspace(req JSONRPCRequest) (string, error) {
	// Use existing Gateway method for URI extraction
	return pg.extractURIFromParams(req)
}

// extractLanguageFromURI extracts language from URI using the router
func (pg *ProjectAwareGateway) extractLanguageFromURI(uri string) string {
	// Use the project router to determine language
	filePath := uri
	if strings.HasPrefix(uri, URIPrefixFile) {
		filePath = strings.TrimPrefix(uri, URIPrefixFile)
	}

	ext := strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(filePath, "."), "."))
	if ext == "" {
		return ""
	}

	language, exists := pg.ProjectRouter.GetLanguageByExtension(ext)
	if !exists {
		return ""
	}

	return language
}

// handleWorkspaceNotification handles notifications with workspace context
func (pg *ProjectAwareGateway) handleWorkspaceNotification(w http.ResponseWriter, r *http.Request, req JSONRPCRequest, client transport.LSPClient, logger *mcp.StructuredLogger, startTime time.Time) {
	if logger != nil {
		logger.Debug("Processing workspace-aware LSP notification")
	}

	err := client.SendNotification(r.Context(), req.Method, req.Params)
	if err != nil {
		if logger != nil {
			logger.WithError(err).Error("Failed to send workspace-aware LSP notification")
		}
		pg.writeError(w, req.ID, InternalError, "Internal error", err)
		return
	}

	duration := time.Since(startTime)
	if logger != nil {
		logger.WithField("duration", duration.String()).Info("Workspace-aware LSP notification processed successfully")
	}
	w.WriteHeader(http.StatusOK)
}

// handleWorkspaceRequest handles requests with workspace context
func (pg *ProjectAwareGateway) handleWorkspaceRequest(w http.ResponseWriter, r *http.Request, req JSONRPCRequest, client transport.LSPClient, logger *mcp.StructuredLogger, startTime time.Time) {
	if logger != nil {
		logger.Debug("Sending workspace-aware request to LSP server")
	}

	// Create timeout context to prevent infinite wait
	timeoutCtx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	result, err := client.SendRequest(timeoutCtx, req.Method, req.Params)
	if err != nil {
		if logger != nil {
			logger.WithError(err).Error("Workspace-aware LSP server request failed")
		}

		// Check if error is due to context deadline exceeded
		if err == context.DeadlineExceeded || strings.Contains(err.Error(), "context deadline exceeded") {
			pg.writeError(w, req.ID, InternalError, "Request timeout: context deadline exceeded", err)
		} else {
			pg.writeError(w, req.ID, InternalError, "Internal error", err)
		}
		return
	}

	response := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      req.ID,
		Result:  result,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		if logger != nil {
			logger.WithError(err).Error("Failed to encode workspace-aware JSON response")
		}
		pg.writeError(w, req.ID, InternalError, "Internal error",
			fmt.Errorf("failed to encode response: %w", err))
		return
	}

	duration := time.Since(startTime)
	responseData, err := json.Marshal(response)
	responseSize := 0
	if err != nil {
		if logger != nil {
			logger.WithError(err).Warn("Failed to marshal response for logging metrics")
		}
		if response.Result != nil {
			responseSize = len(fmt.Sprintf("%v", response.Result))
		}
	} else {
		responseSize = len(responseData)
	}

	if logger != nil {
		logger.WithFields(map[string]interface{}{
			"duration":      duration.String(),
			"response_size": responseSize,
		}).Info("Workspace-aware request processed successfully")
	}
}

// initializeProjectRequestLogger creates a logger with project-aware context
func (pg *ProjectAwareGateway) initializeProjectRequestLogger(r *http.Request) *mcp.StructuredLogger {
	requestID := "req_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	var requestLogger *mcp.StructuredLogger
	if pg.Logger != nil {
		requestLogger = pg.Logger.WithRequestID(requestID)
		requestLogger.WithFields(map[string]interface{}{
			"method":      r.Method,
			"remote_addr": r.RemoteAddr,
			"user_agent":  r.UserAgent(),
		}).Info("Received HTTP request for project-aware processing")
	}
	return requestLogger
}

// validateHTTPMethod validates the HTTP method for project-aware requests
func (pg *ProjectAwareGateway) validateHTTPMethod(w http.ResponseWriter, r *http.Request, logger *mcp.StructuredLogger) bool {
	if r.Method != http.MethodPost {
		if logger != nil {
			logger.Warn("Invalid HTTP method for project-aware request, rejecting")
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

// parseAndValidateJSONRPC parses and validates JSON-RPC request for project-aware processing
func (pg *ProjectAwareGateway) parseAndValidateJSONRPC(w http.ResponseWriter, r *http.Request, logger *mcp.StructuredLogger) (JSONRPCRequest, bool) {
	var req JSONRPCRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if logger != nil {
			logger.WithError(err).Error("Failed to parse JSON-RPC request for project-aware processing")
		}
		pg.writeError(w, nil, ParseError, "Parse error", err)
		return req, false
	}

	if req.JSONRPC != JSONRPCVersion {
		if logger != nil {
			logger.WithField("jsonrpc_version", req.JSONRPC).Error("Invalid JSON-RPC version for project-aware request")
		}
		pg.writeError(w, req.ID, InvalidRequest, ERROR_INVALID_REQUEST,
			fmt.Errorf(FORMAT_INVALID_JSON_RPC, req.JSONRPC))
		return req, false
	}

	if req.Method == "" {
		if logger != nil {
			logger.Error("Missing method field in project-aware JSON-RPC request")
		}
		pg.writeError(w, req.ID, InvalidRequest, ERROR_INVALID_REQUEST,
			fmt.Errorf("missing method field"))
		return req, false
	}

	return req, true
}

// writeError writes error responses for project-aware requests
func (pg *ProjectAwareGateway) writeError(w http.ResponseWriter, id interface{}, code int, message string, err error) {
	// Delegate to the underlying Gateway's writeError method
	pg.Gateway.writeError(w, id, code, message, err)
}

// GetWorkspaceInfo returns information about a specific workspace
func (pg *ProjectAwareGateway) GetWorkspaceInfo(workspaceID string) (map[string]interface{}, error) {
	workspace, exists := pg.WorkspaceManager.GetWorkspaceByID(workspaceID)
	if !exists {
		return nil, fmt.Errorf("workspace not found: %s", workspaceID)
	}

	return workspace.GetInfo(), nil
}

// IsProjectAware indicates that this gateway supports project-aware operations
func (pg *ProjectAwareGateway) IsProjectAware() bool {
	return true
}

// GetProjectRouter returns the project-aware router for advanced usage
func (pg *ProjectAwareGateway) GetProjectRouter() *ProjectAwareRouter {
	return pg.ProjectRouter
}
