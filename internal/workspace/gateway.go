package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/transport"
	"lsp-gateway/mcp"
)

// WorkspaceGateway defines the interface for simplified single workspace operation
type WorkspaceGateway interface {
	// Core lifecycle methods
	Initialize(ctx context.Context, workspaceConfig *WorkspaceConfig, gatewayConfig *WorkspaceGatewayConfig) error
	HandleJSONRPC(w http.ResponseWriter, r *http.Request)
	Start(ctx context.Context) error
	Stop() error
	Health() GatewayHealth

	// Legacy client management (maintained for backward compatibility)
	GetClient(language string) (transport.LSPClient, bool)

	// Enhanced sub-project routing methods
	GetSubProjectClient(subProjectID, language string) (transport.LSPClient, error)
	GetSubProjects() []*SubProject
	RefreshSubProjects(ctx context.Context) error
	GetRoutingMetrics() *RoutingMetrics

	// NEW: Simple sub-project methods
	EnableSubProjectRouting(enabled bool)
}

// WorkspaceGatewayConfig represents simplified configuration for workspace gateway
type WorkspaceGatewayConfig struct {
	WorkspaceRoot    string            `yaml:"workspace_root" json:"workspace_root"`
	ExtensionMapping map[string]string `yaml:"extension_mapping" json:"extension_mapping"`
	Timeout          time.Duration     `yaml:"timeout" json:"timeout"`
	EnableLogging    bool              `yaml:"enable_logging" json:"enable_logging"`
}

// GatewayHealth represents the health status of the workspace gateway
type GatewayHealth struct {
	IsHealthy      bool                    `json:"is_healthy"`
	WorkspaceRoot  string                  `json:"workspace_root"`
	ActiveClients  int                     `json:"active_clients"`
	ClientStatuses map[string]ClientStatus `json:"client_statuses"`
	Errors         []string                `json:"errors,omitempty"`
	LastCheck      time.Time               `json:"last_check"`

	// Enhanced sub-project health information
	SubProjects         int                                `json:"sub_projects"`
	SubProjectClients   map[string]map[string]ClientStatus `json:"sub_project_clients,omitempty"`
	RoutingMetrics      *RoutingMetrics                    `json:"routing_metrics,omitempty"`
	ClientManagerHealth *ClientManagerHealth               `json:"client_manager_health,omitempty"`
}

// ClientStatus represents the status of an individual LSP client
type ClientStatus struct {
	Language string    `json:"language"`
	IsActive bool      `json:"is_active"`
	LastUsed time.Time `json:"last_used"`
	Error    string    `json:"error,omitempty"`
}

// JSONRPCRequest represents a JSON-RPC request
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC response
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// workspaceGateway implements the WorkspaceGateway interface
type workspaceGateway struct {
	// Existing fields (keep all for backward compatibility)
	workspaceRoot    string
	workspaceConfig  *WorkspaceConfig
	gatewayConfig    *WorkspaceGatewayConfig
	clients          map[string]transport.LSPClient // language -> client mapping (legacy)
	extensionMapping map[string]string              // extension -> language mapping (legacy)

	// Enhanced routing components (existing complex system)
	requestRouter      SubProjectRequestRouter
	subProjectResolver SubProjectResolver
	clientManager      SubProjectClientManager
	fallbackHandler    *FallbackHandler
	routingMetrics     *RoutingMetrics

	// NEW: Simple sub-project routing components
	simpleRequestRouter *SimpleRequestRouter
	enableSubProjects   bool // Feature flag for gradual rollout

	mu        sync.RWMutex
	logger    *mcp.StructuredLogger
	isStarted bool
	ctx       context.Context
	cancel    context.CancelFunc
}

// Constants for JSON-RPC
const (
	JSONRPCVersion = "2.0"

	// Error codes
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603

	HTTPContentTypeJSON = "application/json"
	HTTPMethodPOST      = "POST"
	URIPrefixFile       = "file://"
)

// NewWorkspaceGateway creates a new workspace gateway instance
func NewWorkspaceGateway() WorkspaceGateway {
	wg := &workspaceGateway{
		clients:          make(map[string]transport.LSPClient),
		extensionMapping: make(map[string]string),
	}

	// Initialize enhanced routing components (will be configured during Initialize)
	wg.requestRouter = NewSubProjectRequestRouter(nil) // logger will be set later

	return wg
}

// Initialize initializes the workspace gateway with configuration
func (wg *workspaceGateway) Initialize(ctx context.Context, workspaceConfig *WorkspaceConfig, gatewayConfig *WorkspaceGatewayConfig) error {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	if wg.isStarted {
		return fmt.Errorf("gateway already initialized and started")
	}

	wg.workspaceRoot = workspaceConfig.Workspace.RootPath
	wg.workspaceConfig = workspaceConfig
	wg.gatewayConfig = gatewayConfig
	wg.ctx, wg.cancel = context.WithCancel(ctx)

	// Initialize logger
	if gatewayConfig != nil && gatewayConfig.EnableLogging {
		logLevel := wg.parseLogLevel(workspaceConfig.Logging.Level)
		loggerConfig := &mcp.LoggerConfig{
			Level:     logLevel,
			Component: "workspace-gateway",
		}
		wg.logger = mcp.NewStructuredLogger(loggerConfig)
		if wg.logger != nil {
			wg.logger.WithField("workspace_root", wg.workspaceRoot).Info("Initializing workspace gateway")
		}
	}

	// Setup extension mapping
	if gatewayConfig != nil && gatewayConfig.ExtensionMapping != nil {
		for ext, lang := range gatewayConfig.ExtensionMapping {
			wg.extensionMapping[strings.ToLower(strings.TrimPrefix(ext, "."))] = lang
		}
	}

	// Add default extension mappings if not provided
	wg.addDefaultExtensionMappings()

	// Initialize enhanced routing components
	if err := wg.initializeEnhancedRouting(); err != nil {
		return fmt.Errorf("failed to initialize enhanced routing: %w", err)
	}

	// NEW: Initialize simple request router for feature-flagged integration
	if wg.subProjectResolver != nil && wg.clientManager != nil {
		wg.simpleRequestRouter = NewSimpleRequestRouter(wg.subProjectResolver, wg.clientManager, wg.logger)
		wg.enableSubProjects = true // Enable by default, can be controlled via config
	}

	return nil
}

// parseLogLevel converts string log level to mcp.LogLevel
func (wg *workspaceGateway) parseLogLevel(level string) mcp.LogLevel {
	switch strings.ToLower(level) {
	case "trace":
		return mcp.LogLevelTrace
	case "debug":
		return mcp.LogLevelDebug
	case "info":
		return mcp.LogLevelInfo
	case "warn", "warning":
		return mcp.LogLevelWarn
	case "error":
		return mcp.LogLevelError
	case "fatal":
		return mcp.LogLevelFatal
	default:
		return mcp.LogLevelInfo // default to info
	}
}

// addDefaultExtensionMappings adds default file extension to language mappings
func (wg *workspaceGateway) addDefaultExtensionMappings() {
	defaults := map[string]string{
		"go":   "go",
		"mod":  "go",
		"sum":  "go",
		"work": "go",
		"py":   "python",
		"pyx":  "python",
		"pyi":  "python",
		"js":   "javascript",
		"jsx":  "javascript",
		"ts":   "typescript",
		"tsx":  "typescript",
		"java": "java",
		"kt":   "kotlin",
		"rs":   "rust",
		"c":    "c",
		"cpp":  "cpp",
		"cc":   "cpp",
		"cxx":  "cpp",
		"h":    "c",
		"hpp":  "cpp",
	}

	for ext, lang := range defaults {
		if _, exists := wg.extensionMapping[ext]; !exists {
			wg.extensionMapping[ext] = lang
		}
	}
}

// Start starts the workspace gateway and initializes LSP clients
func (wg *workspaceGateway) Start(ctx context.Context) error {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	if wg.isStarted {
		return fmt.Errorf("gateway already started")
	}

	if wg.workspaceConfig == nil {
		return fmt.Errorf("gateway not initialized")
	}

	if wg.logger != nil {
		wg.logger.Info("Starting workspace gateway")
	}

	// Initialize legacy LSP clients for each configured server (for backward compatibility)
	for serverName, serverConfig := range wg.workspaceConfig.Servers {
		if serverConfig == nil {
			continue
		}

		// Create LSP clients for each language supported by this server
		for _, language := range serverConfig.Languages {
			client, err := wg.createLSPClient(serverName, language, serverConfig)
			if err != nil {
				if wg.logger != nil {
					wg.logger.WithError(err).WithFields(map[string]interface{}{
						"server":   serverName,
						"language": language,
					}).Error("Failed to create legacy LSP client")
				}
				continue
			}

			// Start the client
			if err := client.Start(ctx); err != nil {
				if wg.logger != nil {
					wg.logger.WithError(err).WithFields(map[string]interface{}{
						"server":   serverName,
						"language": language,
					}).Error("Failed to start legacy LSP client")
				}
				continue
			}

			wg.clients[language] = client

			if wg.logger != nil {
				wg.logger.WithFields(map[string]interface{}{
					"server":   serverName,
					"language": language,
				}).Info("Legacy LSP client started successfully")
			}
		}
	}

	// Initialize sub-project clients using enhanced routing system (backward compatibility)
	if wg.subProjectResolver != nil && wg.clientManager != nil {
		projects := wg.subProjectResolver.GetAllSubProjects()
		if wg.logger != nil {
			wg.logger.WithField("project_count", len(projects)).Info("Starting clients for detected sub-projects")
		}

		for _, project := range projects {
			if err := wg.clientManager.StartClients(ctx, project); err != nil {
				if wg.logger != nil {
					wg.logger.WithError(err).WithFields(map[string]interface{}{
						"project_id":   project.ID,
						"project_type": project.ProjectType,
						"project_root": project.Root,
					}).Warn("Failed to start clients for sub-project")
				}
				continue
			}

			if wg.logger != nil {
				wg.logger.WithFields(map[string]interface{}{
					"project_id":   project.ID,
					"project_type": project.ProjectType,
					"languages":    project.Languages,
					"project_root": project.Root,
				}).Info("Sub-project clients started successfully")
			}
		}

		// Also initialize simple request router if both components are available
		if wg.simpleRequestRouter == nil {
			wg.simpleRequestRouter = NewSimpleRequestRouter(wg.subProjectResolver, wg.clientManager, wg.logger)
		}
	}

	wg.isStarted = true

	if wg.logger != nil {
		wg.logger.WithField("active_clients", len(wg.clients)).Info("Workspace gateway started successfully")
	}

	return nil
}

// createLSPClient creates an LSP client for the given server and language
func (wg *workspaceGateway) createLSPClient(serverName, language string, config *config.ServerConfig) (transport.LSPClient, error) {
	clientConfig := transport.ClientConfig{
		Command:   config.Command,
		Args:      config.Args,
		Transport: config.Transport,
	}

	if clientConfig.Transport == "" {
		clientConfig.Transport = transport.TransportStdio
	}

	return transport.NewLSPClient(clientConfig)
}

// Stop stops the workspace gateway and cleans up all LSP clients
func (wg *workspaceGateway) Stop() error {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	if !wg.isStarted {
		return nil
	}

	if wg.logger != nil {
		wg.logger.Info("Stopping workspace gateway")
	}

	var errors []error

	// Stop all LSP clients
	for language, client := range wg.clients {
		if err := client.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop %s client: %w", language, err))
			if wg.logger != nil {
				wg.logger.WithError(err).WithField("language", language).Error("Failed to stop LSP client")
			}
		} else {
			if wg.logger != nil {
				wg.logger.WithField("language", language).Info("LSP client stopped successfully")
			}
		}
	}

	// Shutdown enhanced routing components
	if wg.requestRouter != nil {
		if err := wg.requestRouter.Shutdown(wg.ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to shutdown request router: %w", err))
		}
	}

	// Clean up simple request router (no shutdown method needed)
	wg.simpleRequestRouter = nil
	wg.enableSubProjects = false

	if wg.clientManager != nil {
		if err := wg.clientManager.Shutdown(wg.ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to shutdown client manager: %w", err))
		}
	}

	if wg.fallbackHandler != nil {
		if err := wg.fallbackHandler.Shutdown(wg.ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to shutdown fallback handler: %w", err))
		}
	}

	if wg.subProjectResolver != nil {
		if err := wg.subProjectResolver.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close sub-project resolver: %w", err))
		}
	}

	// Clear clients map
	wg.clients = make(map[string]transport.LSPClient)
	wg.isStarted = false

	// Cancel context
	if wg.cancel != nil {
		wg.cancel()
	}

	if wg.logger != nil {
		wg.logger.Info("Workspace gateway stopped")
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors occurred while stopping clients: %v", errors)
	}

	return nil
}

// GetClient retrieves an LSP client by language with O(1) lookup (legacy method for backward compatibility)
func (wg *workspaceGateway) GetClient(language string) (transport.LSPClient, bool) {
	wg.mu.RLock()
	defer wg.mu.RUnlock()

	client, exists := wg.clients[language]
	return client, exists
}

// GetSubProjectClient retrieves an LSP client for a specific sub-project and language
func (wg *workspaceGateway) GetSubProjectClient(subProjectID, language string) (transport.LSPClient, error) {
	wg.mu.RLock()
	defer wg.mu.RUnlock()

	if wg.clientManager == nil {
		return nil, fmt.Errorf("client manager not initialized")
	}

	return wg.clientManager.GetClient(subProjectID, language)
}

// GetSubProjects returns all detected sub-projects in the workspace
func (wg *workspaceGateway) GetSubProjects() []*SubProject {
	wg.mu.RLock()
	defer wg.mu.RUnlock()

	if wg.subProjectResolver == nil {
		return nil
	}

	return wg.subProjectResolver.GetAllSubProjects()
}

// RefreshSubProjects refreshes the sub-project detection and updates internal state
func (wg *workspaceGateway) RefreshSubProjects(ctx context.Context) error {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	if wg.subProjectResolver == nil {
		return fmt.Errorf("sub-project resolver not initialized")
	}

	// Refresh project discovery
	if err := wg.subProjectResolver.RefreshProjects(ctx); err != nil {
		if wg.logger != nil {
			wg.logger.WithError(err).Error("Failed to refresh sub-projects")
		}
		return fmt.Errorf("failed to refresh sub-projects: %w", err)
	}

	// Start clients for newly discovered projects if gateway is already started
	if wg.isStarted && wg.clientManager != nil {
		projects := wg.subProjectResolver.GetAllSubProjects()
		for _, project := range projects {
			if err := wg.clientManager.StartClients(ctx, project); err != nil {
				if wg.logger != nil {
					wg.logger.WithError(err).WithField("project_id", project.ID).Warn("Failed to start clients for sub-project")
				}
			}
		}
	}

	if wg.logger != nil {
		wg.logger.Info("Sub-projects refreshed successfully")
	}

	return nil
}

// GetRoutingMetrics returns current routing performance metrics
func (wg *workspaceGateway) GetRoutingMetrics() *RoutingMetrics {
	wg.mu.RLock()
	defer wg.mu.RUnlock()

	if wg.requestRouter != nil {
		return wg.requestRouter.GetRoutingMetrics()
	}

	// Return local metrics if request router is not available
	return wg.routingMetrics
}

// Health returns the current health status of the workspace gateway
func (wg *workspaceGateway) Health() GatewayHealth {
	wg.mu.RLock()
	defer wg.mu.RUnlock()

	health := GatewayHealth{
		WorkspaceRoot:     wg.workspaceRoot,
		ActiveClients:     0,
		ClientStatuses:    make(map[string]ClientStatus),
		SubProjectClients: make(map[string]map[string]ClientStatus),
		LastCheck:         time.Now(),
	}

	var errors []string

	// Legacy client health (for backward compatibility)
	for language, client := range wg.clients {
		isActive := client.IsActive()
		status := ClientStatus{
			Language: language,
			IsActive: isActive,
			LastUsed: time.Now(), // TODO: Track actual last used time
		}

		if isActive {
			health.ActiveClients++
		} else {
			status.Error = "client not active"
			errors = append(errors, fmt.Sprintf("%s client not active", language))
		}

		health.ClientStatuses[language] = status
	}

	// Sub-project health information
	if wg.subProjectResolver != nil {
		projects := wg.subProjectResolver.GetAllSubProjects()
		health.SubProjects = len(projects)

		// Get sub-project client statuses
		if wg.clientManager != nil {
			allStatuses := wg.clientManager.GetAllClientStatuses()
			for projectID, projectClients := range allStatuses {
				projectStatuses := make(map[string]ClientStatus)
				for language, clientStatus := range projectClients {
					status := ClientStatus{
						Language: clientStatus.Language,
						IsActive: clientStatus.IsActive,
						LastUsed: clientStatus.LastUsed,
						Error:    clientStatus.LastError,
					}

					if status.IsActive {
						health.ActiveClients++
					} else if status.Error != "" {
						errors = append(errors, fmt.Sprintf("sub-project %s %s client: %s", projectID, language, status.Error))
					}

					projectStatuses[language] = status
				}
				health.SubProjectClients[projectID] = projectStatuses
			}

			// Get client manager health
			if managerHealth, err := wg.clientManager.HealthCheck(context.Background()); err == nil {
				health.ClientManagerHealth = managerHealth
			} else {
				errors = append(errors, fmt.Sprintf("client manager health check failed: %v", err))
			}
		}
	}

	// Include routing metrics
	health.RoutingMetrics = wg.GetRoutingMetrics()

	health.IsHealthy = len(errors) == 0 && (health.ActiveClients > 0 || health.SubProjects > 0)
	health.Errors = errors

	return health
}

// HandleJSONRPC handles incoming JSON-RPC HTTP requests with enhanced routing
func (wg *workspaceGateway) HandleJSONRPC(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Existing validation (keep)
	if r.Method != HTTPMethodPOST {
		wg.writeError(w, nil, InvalidRequest, "Method not allowed", fmt.Errorf("expected POST, got %s", r.Method))
		return
	}

	w.Header().Set("Content-Type", HTTPContentTypeJSON)

	req, ok := wg.parseAndValidateJSONRPC(w, r)
	if !ok {
		return
	}

	// NEW: Try sub-project routing first (if enabled)
	if wg.enableSubProjects && wg.simpleRequestRouter != nil {
		result, err := wg.simpleRequestRouter.RouteRequest(wg.ctx, req.Method, req.Params)
		if err == nil {
			// Success with sub-project routing
			wg.writeSuccessResponse(w, req.ID, result)
			wg.logRequestSuccess(req.Method, "sub-project", time.Since(startTime))
			return
		}

		// Log sub-project routing failure but continue with fallback
		if wg.logger != nil {
			wg.logger.WithError(err).WithField("method", req.Method).Debug("Sub-project routing failed, falling back to legacy")
		}
	}

	// FALLBACK: Use existing legacy routing (keep all existing logic)
	language, err := wg.extractLanguageFromRequest(req)
	if err != nil {
		wg.writeError(w, req.ID, InvalidParams, "Failed to determine language", err)
		return
	}

	client, exists := wg.GetClient(language)
	if !exists {
		wg.writeError(w, req.ID, MethodNotFound, fmt.Sprintf("No LSP client for language: %s", language), nil)
		return
	}

	result, err := client.SendRequest(wg.ctx, req.Method, req.Params)
	if err != nil {
		wg.writeError(w, req.ID, InternalError, "LSP request failed", err)
		return
	}

	wg.writeSuccessResponse(w, req.ID, json.RawMessage(result))
	wg.logRequestSuccess(req.Method, "legacy", time.Since(startTime))
}

// parseAndValidateJSONRPC parses and validates incoming JSON-RPC requests
func (wg *workspaceGateway) parseAndValidateJSONRPC(w http.ResponseWriter, r *http.Request) (JSONRPCRequest, bool) {
	var req JSONRPCRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if wg.logger != nil {
			wg.logger.WithError(err).Error("Failed to parse JSON-RPC request")
		}
		wg.writeError(w, nil, ParseError, "Parse error", err)
		return req, false
	}

	if req.JSONRPC != JSONRPCVersion {
		if wg.logger != nil {
			wg.logger.WithField("jsonrpc_version", req.JSONRPC).Error("Invalid JSON-RPC version")
		}
		wg.writeError(w, req.ID, InvalidRequest, "Invalid JSON-RPC version",
			fmt.Errorf("expected %s, got %s", JSONRPCVersion, req.JSONRPC))
		return req, false
	}

	if req.Method == "" {
		if wg.logger != nil {
			wg.logger.Error("Missing method field in JSON-RPC request")
		}
		wg.writeError(w, req.ID, InvalidRequest, "Missing method field", nil)
		return req, false
	}

	return req, true
}

// extractLanguageFromRequest extracts the programming language from the request parameters
func (wg *workspaceGateway) extractLanguageFromRequest(req JSONRPCRequest) (string, error) {
	// Convert params to map for easy access
	paramsMap, ok := req.Params.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid request parameters format")
	}

	// Try to extract URI from different possible parameter structures
	var uri string

	// Check textDocument parameter (most common case)
	if textDoc, exists := paramsMap["textDocument"]; exists {
		if textDocMap, ok := textDoc.(map[string]interface{}); ok {
			if uriVal, exists := textDocMap["uri"]; exists {
				if uriStr, ok := uriVal.(string); ok {
					uri = uriStr
				}
			}
		}
	}

	// Check direct uri parameter
	if uri == "" {
		if uriVal, exists := paramsMap["uri"]; exists {
			if uriStr, ok := uriVal.(string); ok {
				uri = uriStr
			}
		}
	}

	// Check position-based parameters (for go to definition, references, etc.)
	if uri == "" {
		if positionMap, exists := paramsMap["position"]; exists && positionMap != nil {
			// Look for textDocument in the same level
			if textDoc, exists := paramsMap["textDocument"]; exists {
				if textDocMap, ok := textDoc.(map[string]interface{}); ok {
					if uriVal, exists := textDocMap["uri"]; exists {
						if uriStr, ok := uriVal.(string); ok {
							uri = uriStr
						}
					}
				}
			}
		}
	}

	if uri == "" {
		return "", fmt.Errorf("no file URI found in request parameters")
	}

	return wg.extractLanguageFromURI(uri)
}

// extractLanguageFromURI extracts the programming language from a file URI
func (wg *workspaceGateway) extractLanguageFromURI(uri string) (string, error) {
	if uri == "" {
		return "", fmt.Errorf("empty URI provided")
	}

	// Remove URI prefix
	filePath := uri
	if strings.HasPrefix(uri, URIPrefixFile) {
		filePath = strings.TrimPrefix(uri, URIPrefixFile)
	}

	// Get file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		return "", fmt.Errorf("cannot determine file type from URI: %s", uri)
	}

	ext = strings.TrimPrefix(ext, ".")

	// Lookup language by extension (O(1) lookup)
	wg.mu.RLock()
	language, exists := wg.extensionMapping[ext]
	wg.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("unsupported file extension: %s", ext)
	}

	return language, nil
}

// writeError writes a JSON-RPC error response
func (wg *workspaceGateway) writeError(w http.ResponseWriter, id interface{}, code int, message string, err error) {
	var data interface{}
	if err != nil {
		data = err.Error()
	}

	response := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	w.WriteHeader(http.StatusOK)

	if encodeErr := json.NewEncoder(w).Encode(response); encodeErr != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		if wg.logger != nil {
			wg.logger.WithError(encodeErr).Error("Failed to encode error response")
		}
	}
}

// Enhanced routing methods

// initializeEnhancedRouting initializes the enhanced routing components
func (wg *workspaceGateway) initializeEnhancedRouting() error {
	// Initialize routing metrics
	wg.routingMetrics = &RoutingMetrics{
		StrategyUsage: make(map[string]int64),
		LastUpdated:   time.Now(),
	}

	// Initialize sub-project resolver
	if wg.workspaceRoot != "" {
		resolverOptions := &ResolverOptions{
			CacheCapacity:   1000,
			CacheTTL:        30 * time.Minute,
			MaxDepth:        10,
			RefreshInterval: 5 * time.Minute,
		}
		wg.subProjectResolver = NewSubProjectResolver(wg.workspaceRoot, resolverOptions)

		// Perform initial project discovery
		if err := wg.subProjectResolver.RefreshProjects(wg.ctx); err != nil {
			if wg.logger != nil {
				wg.logger.WithError(err).Warn("Failed to perform initial project discovery")
			}
		}
	}

	// Initialize client manager
	if wg.workspaceConfig != nil {
		clientConfig := DefaultClientManagerConfig()
		wg.clientManager = NewSubProjectClientManager(wg.workspaceConfig, clientConfig, wg.logger)
	}

	// Initialize fallback handler
	wg.fallbackHandler = NewFallbackHandler(wg.logger)
	if wg.fallbackHandler != nil && wg.clientManager != nil {
		wg.fallbackHandler.SetClientManager(wg.clientManager)
	}

	// Configure request router
	if wg.requestRouter != nil {
		// Re-create with proper logger
		wg.requestRouter = NewSubProjectRequestRouter(wg.logger)

		// Set dependencies
		if wg.subProjectResolver != nil {
			wg.requestRouter.SetResolver(wg.subProjectResolver)
		}
		if wg.clientManager != nil {
			wg.requestRouter.SetClientManager(wg.clientManager)
		}
	}

	if wg.logger != nil {
		wg.logger.Info("Enhanced routing system initialized successfully")
	}

	return nil
}

// handleWithEnhancedRouting processes requests using the enhanced routing system
func (wg *workspaceGateway) handleWithEnhancedRouting(w http.ResponseWriter, req JSONRPCRequest, startTime time.Time) {
	// 1. Route the request through enhanced routing system
	decision, err := wg.requestRouter.RouteRequest(wg.ctx, &req)
	if err != nil {
		// Try fallback handling
		if response, fallbackErr := wg.requestRouter.HandleRoutingFailure(wg.ctx, &req, err); fallbackErr == nil {
			wg.writeResponse(w, response)
			wg.logRequestCompletion(req.Method, "enhanced_fallback", time.Since(startTime))
			return
		}

		// All routing failed
		wg.writeError(w, req.ID, InternalError, "Request routing failed", err)
		return
	}

	// 2. Execute request using selected strategy
	response, err := decision.Strategy.Route(wg.ctx, decision)
	if err != nil {
		// Try fallback handling
		if fallbackResponse, fallbackErr := wg.requestRouter.HandleRoutingFailure(wg.ctx, &req, err); fallbackErr == nil {
			wg.writeResponse(w, fallbackResponse)
			wg.logRequestCompletion(req.Method, "enhanced_execution_fallback", time.Since(startTime))
			return
		}

		// Execution failed
		wg.writeError(w, req.ID, InternalError, "Request execution failed", err)
		return
	}

	// 3. Return successful response
	wg.writeResponse(w, response)
	wg.logRequestCompletion(req.Method, decision.Strategy.Name(), time.Since(startTime))
}

// handleWithLegacyRouting processes requests using the legacy routing system
func (wg *workspaceGateway) handleWithLegacyRouting(w http.ResponseWriter, req JSONRPCRequest, startTime time.Time) {
	// Extract language from request parameters
	language, err := wg.extractLanguageFromRequest(req)
	if err != nil {
		wg.writeError(w, req.ID, InvalidParams, "Failed to determine language", err)
		return
	}

	// Get LSP client for the language (O(1) lookup)
	client, exists := wg.GetClient(language)
	if !exists {
		wg.writeError(w, req.ID, MethodNotFound, fmt.Sprintf("No LSP client for language: %s", language), nil)
		return
	}

	// Send request to LSP server
	result, err := client.SendRequest(wg.ctx, req.Method, req.Params)
	if err != nil {
		wg.writeError(w, req.ID, InternalError, "LSP request failed", err)
		return
	}

	// Send successful response
	response := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      req.ID,
		Result:  json.RawMessage(result),
	}

	wg.writeResponse(w, &response)
	wg.logRequestCompletion(req.Method, language, time.Since(startTime))
}

// writeResponse writes a JSON-RPC response to the HTTP response writer
func (wg *workspaceGateway) writeResponse(w http.ResponseWriter, response *JSONRPCResponse) {
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		if wg.logger != nil {
			wg.logger.WithError(err).Error("Failed to encode response")
		}
	}
}

// logRequestCompletion logs the completion of a request with timing information
func (wg *workspaceGateway) logRequestCompletion(method, strategy string, duration time.Duration) {
	if wg.logger != nil {
		wg.logger.WithFields(map[string]interface{}{
			"method":   method,
			"strategy": strategy,
			"duration": duration.String(),
		}).Debug("JSON-RPC request completed successfully")
	}
}

// NEW: Helper methods for simplified integration

// writeSuccessResponse writes a successful JSON-RPC response
func (wg *workspaceGateway) writeSuccessResponse(w http.ResponseWriter, id interface{}, result json.RawMessage) {
	response := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  result,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		if wg.logger != nil {
			wg.logger.WithError(err).Error("Failed to encode success response")
		}
	}
}

// logRequestSuccess logs successful request completion with timing information
func (wg *workspaceGateway) logRequestSuccess(method, routingType string, duration time.Duration) {
	if wg.logger != nil {
		wg.logger.WithFields(map[string]interface{}{
			"method":       method,
			"routing_type": routingType,
			"duration":     duration.String(),
		}).Debug("JSON-RPC request completed successfully")
	}
}

// EnableSubProjectRouting enables or disables sub-project routing
func (wg *workspaceGateway) EnableSubProjectRouting(enabled bool) {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	wg.enableSubProjects = enabled

	if wg.logger != nil {
		wg.logger.WithField("enabled", enabled).Info("Sub-project routing setting changed")
	}
}
