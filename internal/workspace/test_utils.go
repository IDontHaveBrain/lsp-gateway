package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"lsp-gateway/internal/project"
	"lsp-gateway/tests/e2e/testutils"
)

// WorkspaceTestSuite provides comprehensive testing infrastructure for workspace integration tests
type WorkspaceTestSuite struct {
	TempDirs        []string
	Gateways        map[string]WorkspaceGateway
	PortManager     WorkspacePortManager
	ConfigManager   WorkspaceConfigManager
	MockLSPServers  map[string]*MockLSPServer
	Ctx             context.Context
	Cancel          context.CancelFunc
	HTTPClients     map[string]*testutils.HttpClient
	BaseTestDir     string
	Logger          *testutils.SimpleLogger
	mu              sync.RWMutex
	cleanupHandlers []func() error
	isSetup         bool
}

// LSPRequest represents a recorded LSP request for validation and testing
type LSPRequest struct {
	Method    string      `json:"method"`
	Params    interface{} `json:"params"`
	Timestamp time.Time   `json:"timestamp"`
	ID        string      `json:"id,omitempty"`
}

// WorkspaceTestConfig configures workspace test behavior
type WorkspaceTestConfig struct {
	BaseDir           string
	EnableLogging     bool
	LogLevel          string
	Timeout           time.Duration
	MockServerDelay   time.Duration
	MockFailureRate   float64
	AutoCleanup       bool
	PortRangeStart    int
	PortRangeEnd      int
	MaxConcurrentTest int
}

// DefaultWorkspaceTestConfig returns optimized default configuration for workspace testing
func DefaultWorkspaceTestConfig() WorkspaceTestConfig {
	return WorkspaceTestConfig{
		BaseDir:           "/tmp/lspg-workspace-tests",
		EnableLogging:     true,
		LogLevel:          "debug",
		Timeout:           30 * time.Second,
		MockServerDelay:   50 * time.Millisecond,
		MockFailureRate:   0.0,
		AutoCleanup:       true,
		PortRangeStart:    9000,
		PortRangeEnd:      9100,
		MaxConcurrentTest: 5,
	}
}

// NewWorkspaceTestSuite creates a new comprehensive workspace test suite
func NewWorkspaceTestSuite() *WorkspaceTestSuite {
	return NewWorkspaceTestSuiteWithConfig(DefaultWorkspaceTestConfig())
}

// NewWorkspaceTestSuiteWithConfig creates a new workspace test suite with custom configuration
func NewWorkspaceTestSuiteWithConfig(config WorkspaceTestConfig) *WorkspaceTestSuite {
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	
	// Create base test directory
	baseTestDir := config.BaseDir
	if baseTestDir == "" {
		baseTestDir = filepath.Join(os.TempDir(), fmt.Sprintf("lspg-workspace-test-%d", time.Now().UnixNano()))
	}
	
	// Ensure base directory exists
	os.MkdirAll(baseTestDir, 0755)
	
	// Initialize logger
	logger := testutils.NewSimpleLogger()
	
	// Initialize port manager
	portManager, err := NewWorkspacePortManager()
	if err != nil {
		logger.Warn("Failed to create port manager, tests may have port conflicts", map[string]interface{}{"error": err})
	}
	
	// Initialize config manager
	configManager := NewWorkspaceConfigManagerWithOptions(&WorkspaceConfigManagerOptions{
		BaseConfigDir: baseTestDir,
	})
	
	suite := &WorkspaceTestSuite{
		TempDirs:        make([]string, 0),
		Gateways:        make(map[string]WorkspaceGateway),
		PortManager:     portManager,
		ConfigManager:   configManager,
		MockLSPServers:  make(map[string]*MockLSPServer),
		Ctx:             ctx,
		Cancel:          cancel,
		HTTPClients:     make(map[string]*testutils.HttpClient),
		BaseTestDir:     baseTestDir,
		Logger:          logger,
		cleanupHandlers: make([]func() error, 0),
		isSetup:         false,
	}
	
	// Register auto-cleanup if enabled
	if config.AutoCleanup {
		suite.registerCleanupHandler(func() error {
			return os.RemoveAll(baseTestDir)
		})
	}
	
	return suite
}

// SetupWorkspace creates and configures a test workspace with specified languages
func (wts *WorkspaceTestSuite) SetupWorkspace(name string, languages []string) error {
	wts.mu.Lock()
	defer wts.mu.Unlock()
	
	if wts.isSetup {
		return fmt.Errorf("workspace test suite already setup, call Cleanup first")
	}
	
	wts.Logger.Info("Setting up workspace for testing", map[string]interface{}{
		"workspace_name": name,
		"languages":      languages,
	})
	
	// Create temporary workspace directory
	workspaceDir := filepath.Join(wts.BaseTestDir, "workspaces", name)
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}
	wts.TempDirs = append(wts.TempDirs, workspaceDir)
	
	// Create test project files for each language
	if err := wts.CreateTestProject(workspaceDir, languages); err != nil {
		return fmt.Errorf("failed to create test project: %w", err)
	}
	
	// Setup mock LSP servers for each language
	mockServers, err := StartMockLSPServers(languages)
	if err != nil {
		return fmt.Errorf("failed to start mock LSP servers: %w", err)
	}
	wts.MockLSPServers = mockServers
	
	// Generate workspace configuration
	projectContext, err := wts.createProjectContext(workspaceDir, languages)
	if err != nil {
		return fmt.Errorf("failed to create project context: %w", err)
	}
	
	if err := wts.ConfigManager.GenerateWorkspaceConfig(workspaceDir, projectContext); err != nil {
		return fmt.Errorf("failed to generate workspace config: %w", err)
	}
	
	// Create workspace gateway
	gateway := NewWorkspaceGateway()
	wts.Gateways[name] = gateway
	
	// Initialize HTTP client for the workspace
	clientConfig := testutils.DefaultHttpClientConfig()
	clientConfig.WorkspaceID = name
	clientConfig.ProjectPath = workspaceDir
	httpClient := testutils.NewHttpClient(clientConfig)
	wts.HTTPClients[name] = httpClient
	
	wts.isSetup = true
	
	wts.Logger.Info("Workspace setup completed successfully", map[string]interface{}{"workspace_name": name})
	return nil
}

// StartWorkspace initializes and starts the workspace gateway and services
func (wts *WorkspaceTestSuite) StartWorkspace(name string) error {
	wts.mu.Lock()
	defer wts.mu.Unlock()
	
	gateway, exists := wts.Gateways[name]
	if !exists {
		return fmt.Errorf("workspace %s not found, call SetupWorkspace first", name)
	}
	
	wts.Logger.Info("Starting workspace gateway", map[string]interface{}{"workspace_name": name})
	
	// Load workspace configuration
	workspaceDir := filepath.Join(wts.BaseTestDir, "workspaces", name)
	workspaceConfig, err := wts.ConfigManager.LoadWorkspaceConfig(workspaceDir)
	if err != nil {
		return fmt.Errorf("failed to load workspace config: %w", err)
	}
	
	// Allocate port for the workspace
	port, err := wts.PortManager.AllocatePort(workspaceDir)
	if err != nil {
		return fmt.Errorf("failed to allocate port: %w", err)
	}
	
	// Create gateway configuration
	gatewayConfig := &WorkspaceGatewayConfig{
		WorkspaceRoot:    workspaceDir,
		EnableLogging:    true,
		Timeout:          10 * time.Second,
		ExtensionMapping: CreateDefaultExtensionMapping(),
	}
	
	// Initialize the gateway
	if err := gateway.Initialize(wts.Ctx, workspaceConfig, gatewayConfig); err != nil {
		return fmt.Errorf("failed to initialize gateway: %w", err)
	}
	
	// Start the gateway
	if err := gateway.Start(wts.Ctx); err != nil {
		return fmt.Errorf("failed to start gateway: %w", err)
	}
	
	// Update HTTP client with correct port
	if httpClient, exists := wts.HTTPClients[name]; exists {
		httpClient.Close() // Close old client
		
		clientConfig := testutils.DefaultHttpClientConfig()
		clientConfig.BaseURL = fmt.Sprintf("http://localhost:%d", port)
		clientConfig.WorkspaceID = name
		clientConfig.ProjectPath = workspaceDir
		wts.HTTPClients[name] = testutils.NewHttpClient(clientConfig)
	}
	
	wts.Logger.Info("Workspace gateway started successfully", map[string]interface{}{
		"workspace_name": name,
		"port":           port,
	})
	
	return nil
}

// StopWorkspace gracefully stops the workspace gateway and cleans up resources
func (wts *WorkspaceTestSuite) StopWorkspace(name string) error {
	wts.mu.Lock()
	defer wts.mu.Unlock()
	
	wts.Logger.Info("Stopping workspace gateway", map[string]interface{}{"workspace_name": name})
	
	var errors []error
	
	// Stop gateway
	if gateway, exists := wts.Gateways[name]; exists {
		if err := gateway.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop gateway: %w", err))
		}
		delete(wts.Gateways, name)
	}
	
	// Close HTTP client
	if httpClient, exists := wts.HTTPClients[name]; exists {
		if err := httpClient.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close HTTP client: %w", err))
		}
		delete(wts.HTTPClients, name)
	}
	
	// Release port
	workspaceDir := filepath.Join(wts.BaseTestDir, "workspaces", name)
	if wts.PortManager != nil {
		if err := wts.PortManager.ReleasePort(workspaceDir); err != nil {
			errors = append(errors, fmt.Errorf("failed to release port: %w", err))
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("errors stopping workspace: %v", errors)
	}
	
	wts.Logger.Info("Workspace stopped successfully", map[string]interface{}{"workspace_name": name})
	return nil
}

// Cleanup performs comprehensive cleanup of all test resources
func (wts *WorkspaceTestSuite) Cleanup() {
	wts.mu.Lock()
	defer wts.mu.Unlock()
	
	wts.Logger.Info("Starting comprehensive workspace test cleanup", map[string]interface{}{})
	
	// Stop all mock LSP servers
	StopMockLSPServers(wts.MockLSPServers)
	wts.MockLSPServers = make(map[string]*MockLSPServer)
	
	// Stop all workspace gateways
	for name := range wts.Gateways {
		wts.StopWorkspace(name)
	}
	
	// Close all HTTP clients
	for name, client := range wts.HTTPClients {
		client.Close()
		delete(wts.HTTPClients, name)
	}
	
	// Run custom cleanup handlers
	for _, handler := range wts.cleanupHandlers {
		if err := handler(); err != nil {
			wts.Logger.Warn("Cleanup handler failed", map[string]interface{}{"error": err})
		}
	}
	
	// Remove temporary directories
	for _, dir := range wts.TempDirs {
		if err := os.RemoveAll(dir); err != nil {
			wts.Logger.Warn("Failed to remove temp directory", map[string]interface{}{"error": err, "directory": dir})
		}
	}
	wts.TempDirs = wts.TempDirs[:0]
	
	// Cancel context
	if wts.Cancel != nil {
		wts.Cancel()
	}
	
	wts.isSetup = false
	wts.Logger.Info("Workspace test cleanup completed", map[string]interface{}{})
}

// CreateTestProject creates a realistic test project with files for specified languages
func (wts *WorkspaceTestSuite) CreateTestProject(dir string, languages []string) error {
	for _, lang := range languages {
		langDir := filepath.Join(dir, lang)
		if err := os.MkdirAll(langDir, 0755); err != nil {
			return fmt.Errorf("failed to create language directory %s: %w", lang, err)
		}
		
		if err := SetupWorkspaceProjectFiles(langDir, lang); err != nil {
			return fmt.Errorf("failed to setup project files for %s: %w", lang, err)
		}
	}
	
	return nil
}

// WaitForWorkspaceReady waits for workspace to become ready with comprehensive health checking
func (wts *WorkspaceTestSuite) WaitForWorkspaceReady(name string, timeout time.Duration) error {
	wts.mu.RLock()
	httpClient, exists := wts.HTTPClients[name]
	gateway, gatewayExists := wts.Gateways[name]
	wts.mu.RUnlock()
	
	if !exists || !gatewayExists {
		return fmt.Errorf("workspace %s not found", name)
	}
	
	wts.Logger.Info("Waiting for workspace to become ready", map[string]interface{}{
		"workspace_name": name,
		"timeout":        timeout,
	})
	
	ctx, cancel := context.WithTimeout(wts.Ctx, timeout)
	defer cancel()
	
	// Use fast polling with exponential backoff for efficiency
	pollConfig := testutils.DefaultPollingConfig()
	pollConfig.Timeout = timeout
	
	condition := func() (bool, error) {
		// Check gateway health
		health := gateway.Health()
		if !health.IsHealthy {
			return false, nil
		}
		
		// Check HTTP connectivity
		if err := httpClient.FastHealthCheck(ctx); err != nil {
			return false, nil
		}
		
		// Test basic LSP functionality
		_, err := httpClient.WorkspaceSymbol(ctx, "test")
		if err != nil {
			return false, nil
		}
		
		return true, nil
	}
	
	return testutils.WaitForConditionWithContext(ctx, condition, pollConfig, "waiting for workspace ready")
}

// SendJSONRPCRequest sends a JSON-RPC request to the specified workspace
func (wts *WorkspaceTestSuite) SendJSONRPCRequest(workspace string, method string, params interface{}) (interface{}, error) {
	wts.mu.RLock()
	httpClient, exists := wts.HTTPClients[workspace]
	wts.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("workspace %s not found", workspace)
	}
	
	ctx, cancel := context.WithTimeout(wts.Ctx, 10*time.Second)
	defer cancel()
	
	// Route to appropriate LSP method
	switch method {
	case "textDocument/definition":
		if defParams, ok := params.(map[string]interface{}); ok {
			uri := defParams["textDocument"].(map[string]interface{})["uri"].(string)
			pos := defParams["position"].(map[string]interface{})
			position := testutils.Position{
				Line:      int(pos["line"].(float64)),
				Character: int(pos["character"].(float64)),
			}
			return httpClient.Definition(ctx, uri, position)
		}
		
	case "textDocument/references":
		if refParams, ok := params.(map[string]interface{}); ok {
			uri := refParams["textDocument"].(map[string]interface{})["uri"].(string)
			pos := refParams["position"].(map[string]interface{})
			position := testutils.Position{
				Line:      int(pos["line"].(float64)),
				Character: int(pos["character"].(float64)),
			}
			includeDecl := refParams["context"].(map[string]interface{})["includeDeclaration"].(bool)
			return httpClient.References(ctx, uri, position, includeDecl)
		}
		
	case "textDocument/hover":
		if hoverParams, ok := params.(map[string]interface{}); ok {
			uri := hoverParams["textDocument"].(map[string]interface{})["uri"].(string)
			pos := hoverParams["position"].(map[string]interface{})
			position := testutils.Position{
				Line:      int(pos["line"].(float64)),
				Character: int(pos["character"].(float64)),
			}
			return httpClient.Hover(ctx, uri, position)
		}
		
	case "textDocument/documentSymbol":
		if docParams, ok := params.(map[string]interface{}); ok {
			uri := docParams["textDocument"].(map[string]interface{})["uri"].(string)
			return httpClient.DocumentSymbol(ctx, uri)
		}
		
	case "workspace/symbol":
		if wsParams, ok := params.(map[string]interface{}); ok {
			query := wsParams["query"].(string)
			return httpClient.WorkspaceSymbol(ctx, query)
		}
		
	case "textDocument/completion":
		if compParams, ok := params.(map[string]interface{}); ok {
			uri := compParams["textDocument"].(map[string]interface{})["uri"].(string)
			pos := compParams["position"].(map[string]interface{})
			position := testutils.Position{
				Line:      int(pos["line"].(float64)),
				Character: int(pos["character"].(float64)),
			}
			return httpClient.Completion(ctx, uri, position)
		}
	}
	
	return nil, fmt.Errorf("unsupported LSP method: %s", method)
}

// ValidateWorkspaceIsolation ensures workspaces are properly isolated from each other
func (wts *WorkspaceTestSuite) ValidateWorkspaceIsolation() error {
	wts.mu.RLock()
	defer wts.mu.RUnlock()
	
	if len(wts.Gateways) < 2 {
		return nil // Nothing to validate with less than 2 workspaces
	}
	
	wts.Logger.Info("Validating workspace isolation", map[string]interface{}{})
	
	// Check port isolation
	assignedPorts := make(map[int]string)
	for name := range wts.Gateways {
		workspaceDir := filepath.Join(wts.BaseTestDir, "workspaces", name)
		if port, exists := wts.PortManager.GetAssignedPort(workspaceDir); exists {
			if existingWorkspace, duplicate := assignedPorts[port]; duplicate {
				return fmt.Errorf("port %d assigned to both %s and %s", port, name, existingWorkspace)
			}
			assignedPorts[port] = name
		}
	}
	
	// Check directory isolation
	for name1 := range wts.Gateways {
		for name2 := range wts.Gateways {
			if name1 != name2 {
				dir1 := filepath.Join(wts.BaseTestDir, "workspaces", name1)
				dir2 := filepath.Join(wts.BaseTestDir, "workspaces", name2)
				
				// Ensure directories are different
				if dir1 == dir2 {
					return fmt.Errorf("workspaces %s and %s share the same directory", name1, name2)
				}
				
				// Ensure no directory nesting
				if isSubPath(dir1, dir2) || isSubPath(dir2, dir1) {
					return fmt.Errorf("workspace directories %s and %s are nested", name1, name2)
				}
			}
		}
	}
	
	wts.Logger.Info("Workspace isolation validation passed", map[string]interface{}{})
	return nil
}

// GetWorkspaceMetrics returns comprehensive metrics for a workspace
func (wts *WorkspaceTestSuite) GetWorkspaceMetrics(name string) (map[string]interface{}, error) {
	wts.mu.RLock()
	defer wts.mu.RUnlock()
	
	httpClient, clientExists := wts.HTTPClients[name]
	gateway, gatewayExists := wts.Gateways[name]
	
	if !clientExists || !gatewayExists {
		return nil, fmt.Errorf("workspace %s not found", name)
	}
	
	metrics := make(map[string]interface{})
	
	// HTTP client metrics
	clientMetrics := httpClient.GetMetrics()
	metrics["http_client"] = map[string]interface{}{
		"total_requests":    clientMetrics.TotalRequests,
		"successful_reqs":   clientMetrics.SuccessfulReqs,
		"failed_requests":   clientMetrics.FailedRequests,
		"average_latency":   clientMetrics.AverageLatency,
		"min_latency":       clientMetrics.MinLatency,
		"max_latency":       clientMetrics.MaxLatency,
		"connection_errors": clientMetrics.ConnectionErrors,
		"timeout_errors":    clientMetrics.TimeoutErrors,
	}
	
	// Gateway health
	health := gateway.Health()
	metrics["gateway_health"] = map[string]interface{}{
		"is_healthy":      health.IsHealthy,
		"active_clients":  health.ActiveClients,
		"client_statuses": health.ClientStatuses,
		"errors":          health.Errors,
		"last_check":      health.LastCheck,
	}
	
	// Mock server metrics (if available)
	mockMetrics := make(map[string]interface{})
	for lang, mockServer := range wts.MockLSPServers {
		history := mockServer.GetRequestHistory()
		mockMetrics[lang] = map[string]interface{}{
			"request_count": len(history),
			"is_running":    mockServer.IsRunning,
			"port":          mockServer.Port,
			"failure_rate":  mockServer.FailureRate,
		}
	}
	metrics["mock_servers"] = mockMetrics
	
	return metrics, nil
}

// registerCleanupHandler adds a cleanup handler to be executed during Cleanup()
func (wts *WorkspaceTestSuite) registerCleanupHandler(handler func() error) {
	wts.cleanupHandlers = append(wts.cleanupHandlers, handler)
}

// createProjectContext creates a project context for workspace configuration generation
func (wts *WorkspaceTestSuite) createProjectContext(workspaceDir string, languages []string) (*project.ProjectContext, error) {
	// Create a minimal project context for testing
	return &project.ProjectContext{
		ProjectType:    "multi-language",
		Languages:      languages,
		RootPath:       workspaceDir,
		ConfigFiles:    make([]string, 0),
		Dependencies:   make(map[string]string),
		BuildSystem:    "make",
		PackageManager: "npm",
		IsMonorepo:     len(languages) > 1,
	}, nil
}

// CreateDefaultExtensionMapping returns default file extension mappings for testing
func CreateDefaultExtensionMapping() map[string]string {
	return map[string]string{
		"go":   "go",
		"py":   "python",
		"js":   "javascript",
		"ts":   "typescript",
		"java": "java",
		"rs":   "rust",
		"c":    "c",
		"cpp":  "cpp",
		"h":    "c",
		"hpp":  "cpp",
	}
}

// isSubPath checks if child is a subpath of parent
func isSubPath(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != ".." && !filepath.IsAbs(rel) && rel != "."
}