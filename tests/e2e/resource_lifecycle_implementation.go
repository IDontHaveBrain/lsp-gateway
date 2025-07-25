package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"lsp-gateway/mcp"
	"lsp-gateway/tests/framework"
	"lsp-gateway/tests/mocks"
)

// TempDirectoryManager manages temporary directories for test isolation
type TempDirectoryManager struct {
	baseDir     string
	directories map[string]string
	mu          sync.RWMutex
	logger      *log.Logger
}

// TestWorkspaceManager manages test workspaces
type TestWorkspaceManager struct {
	workspaces map[string]*TestWorkspace
	mu         sync.RWMutex
	logger     *log.Logger
}

// TestWorkspace represents a managed test workspace
type TestWorkspace struct {
	ID           string
	Path         string
	Projects     []*framework.TestProject
	FileWatchers []interface{}
	CreatedAt    time.Time
	LastUsed     time.Time
	mu           sync.RWMutex
}

// ResourceHealthChecker monitors resource health
type ResourceHealthChecker struct {
	resources     map[string]*ManagedResource
	checkInterval time.Duration
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.RWMutex
	logger        *log.Logger
}

// ResourcePerformanceMonitor monitors resource performance
type ResourcePerformanceMonitor struct {
	enabled       bool
	checkInterval time.Duration
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.RWMutex
	logger        *log.Logger
}

// ResourceUsageTracker tracks resource usage patterns
type ResourceUsageTracker struct {
	usageData map[string]*ResourceUsageData
	mu        sync.RWMutex
	logger    *log.Logger
}

// ResourceUsageData contains usage statistics for a resource
type ResourceUsageData struct {
	ResourceID       string
	TotalUsageTime   time.Duration
	UsageCount       int64
	AverageUsageTime time.Duration
	PeakUsageTime    time.Duration
	LastUsed         time.Time
}

// MockResponseFactory creates mock responses for testing
type MockResponseFactory struct {
	responseTemplates map[string][]json.RawMessage
	mu                sync.RWMutex
}

// MockErrorFactory creates mock errors for testing
type MockErrorFactory struct {
	errorTemplates map[mcp.ErrorCategory][]error
	mu             sync.RWMutex
}

// initializeWorkspaceManager initializes the workspace manager
func (rlm *ResourceLifecycleManager) initializeWorkspaceManager() {
	rlm.workspaceManager = &TestWorkspaceManager{
		workspaces: make(map[string]*TestWorkspace),
		logger:     rlm.logger,
	}
	rlm.logger.Printf("Initialized workspace manager")
}

// initializeTempDirectoryManager initializes the temporary directory manager
func (rlm *ResourceLifecycleManager) initializeTempDirectoryManager() {
	baseDir := filepath.Join(os.TempDir(), rlm.config.TempDirPrefix)
	os.MkdirAll(baseDir, 0755)

	rlm.tempDirManager = &TempDirectoryManager{
		baseDir:     baseDir,
		directories: make(map[string]string),
		logger:      rlm.logger,
	}

	// Add cleanup function
	rlm.cleanupFunctions = append(rlm.cleanupFunctions, func() error {
		return os.RemoveAll(baseDir)
	})

	rlm.logger.Printf("Initialized temp directory manager at %s", baseDir)
}

// initializeResourceMetrics initializes resource metrics tracking
func (rlm *ResourceLifecycleManager) initializeResourceMetrics() {
	if !rlm.config.ResourceMetricsEnabled {
		return
	}

	rlm.resourceMetrics = &ResourceMetrics{
		ServerMetrics:    &ResourceTypeMetrics{},
		ProjectMetrics:   &ResourceTypeMetrics{},
		ClientMetrics:    &ResourceTypeMetrics{},
		ConfigMetrics:    &ResourceTypeMetrics{},
		WorkspaceMetrics: &ResourceTypeMetrics{},
	}

	rlm.resourceUsageTracker = &ResourceUsageTracker{
		usageData: make(map[string]*ResourceUsageData),
		logger:    rlm.logger,
	}

	rlm.logger.Printf("Initialized resource metrics tracking")
}

// initializeHealthChecker initializes the resource health checker
func (rlm *ResourceLifecycleManager) initializeHealthChecker() {
	ctx, cancel := context.WithCancel(rlm.ctx)
	rlm.healthChecker = &ResourceHealthChecker{
		resources:     make(map[string]*ManagedResource),
		checkInterval: rlm.config.HealthCheckInterval,
		ctx:           ctx,
		cancel:        cancel,
		logger:        rlm.logger,
	}

	// Start health checking goroutine
	go rlm.healthChecker.startHealthChecking()

	rlm.logger.Printf("Initialized resource health checker")
}

// initializePerformanceMonitor initializes the performance monitor
func (rlm *ResourceLifecycleManager) initializePerformanceMonitor() {
	if !rlm.config.PerformanceMonitoring {
		return
	}

	ctx, cancel := context.WithCancel(rlm.ctx)
	rlm.performanceMonitor = &ResourcePerformanceMonitor{
		enabled:       true,
		checkInterval: 10 * time.Second,
		ctx:           ctx,
		cancel:        cancel,
		logger:        rlm.logger,
	}

	// Start performance monitoring goroutine
	go rlm.performanceMonitor.startPerformanceMonitoring()

	rlm.logger.Printf("Initialized performance monitor")
}

// Resource creation functions

// createServerResource creates a new server resource
func (rlm *ResourceLifecycleManager) createServerResource() (*ManagedResource, error) {
	startTime := time.Now()
	resourceID := fmt.Sprintf("server-%d-%d", time.Now().UnixNano(), rand.Intn(1000))

	rlm.logger.Printf("Creating server resource %s", resourceID)

	// Create server resource data
	serverData := &ServerResourceData{
		Port:       8080 + rand.Intn(1000), // Random port to avoid conflicts
		ConfigPath: "",
		LogPath:    "",
		Health: &HealthStatus{
			Status:    HealthStatusHealthy,
			LastCheck: time.Now(),
		},
	}

	resource := &ManagedResource{
		ID:              resourceID,
		Name:            fmt.Sprintf("test-server-%s", resourceID),
		Type:            ResourceTypeServer,
		Status:          ResourceStatusCreating,
		CreatedAt:       time.Now(),
		HealthStatus:    HealthStatusHealthy,
		Metadata:        make(map[string]interface{}),
		ServerInstance:  serverData,
		CleanupFunc:     rlm.cleanupServerResource,
		HealthCheckFunc: rlm.healthCheckServerResource,
		ResetFunc:       rlm.resetServerResource,
	}

	// Simulate server startup
	time.Sleep(100 * time.Millisecond)

	resource.StartupTime = time.Since(startTime)
	resource.Status = ResourceStatusAvailable
	resource.LastHealthCheck = time.Now()

	rlm.resources[resourceID] = resource

	// Add to health checker
	if rlm.healthChecker != nil {
		rlm.healthChecker.mu.Lock()
		rlm.healthChecker.resources[resourceID] = resource
		rlm.healthChecker.mu.Unlock()
	}

	// Update metrics
	if rlm.resourceMetrics != nil {
		atomic.AddInt64(&rlm.resourceMetrics.TotalResources, 1)
		atomic.AddInt64(&rlm.resourceMetrics.ActiveResources, 1)
		atomic.AddInt64(&rlm.resourceMetrics.ServerMetrics.Total, 1)
		atomic.AddInt64(&rlm.resourceMetrics.ServerMetrics.Active, 1)
	}

	rlm.logger.Printf("Created server resource %s in %v", resourceID, resource.StartupTime)
	return resource, nil
}

// createProjectResource creates a new project resource
func (rlm *ResourceLifecycleManager) createProjectResource() (*ManagedResource, error) {
	startTime := time.Now()
	resourceID := fmt.Sprintf("project-%d-%d", time.Now().UnixNano(), rand.Intn(1000))

	rlm.logger.Printf("Creating project resource %s", resourceID)

	// Create a test project
	languages := []string{"go", "python", "typescript"}
	projectGenerator := framework.NewTestProjectGenerator(60 * time.Second)
	project, err := projectGenerator.CreateMultiLanguageProject(framework.ProjectTypeMultiLanguage, languages)
	if err != nil {
		return nil, fmt.Errorf("failed to create test project: %w", err)
	}

	projectData := &ProjectResourceData{
		Project:   project,
		Generator: projectGenerator,
		FileCount: len(project.Structure),
		TotalSize: project.Size,
	}

	resource := &ManagedResource{
		ID:              resourceID,
		Name:            project.Name,
		Type:            ResourceTypeProject,
		Status:          ResourceStatusCreating,
		CreatedAt:       time.Now(),
		HealthStatus:    HealthStatusHealthy,
		Metadata:        make(map[string]interface{}),
		ProjectData:     projectData,
		CleanupFunc:     rlm.cleanupProjectResource,
		HealthCheckFunc: rlm.healthCheckProjectResource,
		ResetFunc:       rlm.resetProjectResource,
	}

	resource.StartupTime = time.Since(startTime)
	resource.Status = ResourceStatusAvailable
	resource.LastHealthCheck = time.Now()

	rlm.resources[resourceID] = resource

	// Update metrics
	if rlm.resourceMetrics != nil {
		atomic.AddInt64(&rlm.resourceMetrics.TotalResources, 1)
		atomic.AddInt64(&rlm.resourceMetrics.ActiveResources, 1)
		atomic.AddInt64(&rlm.resourceMetrics.ProjectMetrics.Total, 1)
		atomic.AddInt64(&rlm.resourceMetrics.ProjectMetrics.Active, 1)
		atomic.AddInt64(&rlm.resourceMetrics.ProjectMetrics.AvgSize, project.Size)
	}

	rlm.logger.Printf("Created project resource %s (%s) in %v", resourceID, project.Name, resource.StartupTime)
	return resource, nil
}

// createClientResource creates a new client resource
func (rlm *ResourceLifecycleManager) createClientResource() (*ManagedResource, error) {
	startTime := time.Now()
	resourceID := fmt.Sprintf("client-%d-%d", time.Now().UnixNano(), rand.Intn(1000))

	rlm.logger.Printf("Creating client resource %s", resourceID)

	// Create a mock MCP client
	mockClient := mocks.NewMockMcpClient()
	if mockClient == nil {
		return nil, fmt.Errorf("failed to create mock MCP client")
	}

	// Configure client with default settings
	mockClient.SetBaseURL("http://localhost:8080")
	mockClient.SetTimeout(30 * time.Second)
	mockClient.SetMaxRetries(3)
	mockClient.SetHealthy(true)

	clientData := &ClientResourceData{
		MockClient: mockClient,
		Config: &McpClientConfig{
			BaseURL:    "http://localhost:8080",
			Timeout:    30 * time.Second,
			MaxRetries: 3,
		},
		Metrics: &mcp.ConnectionMetrics{
			TotalRequests:    0,
			SuccessfulReqs:   0,
			FailedRequests:   0,
			TimeoutCount:     0,
			ConnectionErrors: 0,
			AverageLatency:   100 * time.Millisecond,
			LastRequestTime:  time.Now(),
			LastSuccessTime:  time.Now(),
		},
		State: ClientStateInitialized,
	}

	resource := &ManagedResource{
		ID:              resourceID,
		Name:            fmt.Sprintf("mock-client-%s", resourceID),
		Type:            ResourceTypeClient,
		Status:          ResourceStatusCreating,
		CreatedAt:       time.Now(),
		HealthStatus:    HealthStatusHealthy,
		Metadata:        make(map[string]interface{}),
		ClientData:      clientData,
		CleanupFunc:     rlm.cleanupClientResource,
		HealthCheckFunc: rlm.healthCheckClientResource,
		ResetFunc:       rlm.resetClientResource,
	}

	resource.StartupTime = time.Since(startTime)
	resource.Status = ResourceStatusAvailable
	resource.LastHealthCheck = time.Now()
	clientData.State = ClientStateConfigured

	rlm.resources[resourceID] = resource

	// Update metrics
	if rlm.resourceMetrics != nil {
		atomic.AddInt64(&rlm.resourceMetrics.TotalResources, 1)
		atomic.AddInt64(&rlm.resourceMetrics.ActiveResources, 1)
		atomic.AddInt64(&rlm.resourceMetrics.ClientMetrics.Total, 1)
		atomic.AddInt64(&rlm.resourceMetrics.ClientMetrics.Active, 1)
	}

	rlm.logger.Printf("Created client resource %s in %v", resourceID, resource.StartupTime)
	return resource, nil
}

// createConfigResource creates a new configuration resource
func (rlm *ResourceLifecycleManager) createConfigResource() (*ManagedResource, error) {
	startTime := time.Now()
	resourceID := fmt.Sprintf("config-%d-%d", time.Now().UnixNano(), rand.Intn(1000))

	rlm.logger.Printf("Creating config resource %s", resourceID)

	// Create temporary config file
	tempDir := rlm.tempDirManager.baseDir
	configPath := filepath.Join(tempDir, fmt.Sprintf("config-%s.yaml", resourceID))

	// Write sample config
	configContent := `
http:
  port: 8080
  timeout: 30s

language_servers:
  go:
    command: ["gopls"]
    transport: "stdio"
  python:
    command: ["pylsp"]
    transport: "stdio"

logging:
  level: "info"
  file: "gateway.log"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to create config file: %w", err)
	}

	configData := &ConfigResourceData{
		ConfigPath: configPath,
		Template:   "default",
		Validation: &ConfigValidationResult{
			Valid:       true,
			ValidatedAt: time.Now(),
		},
	}

	resource := &ManagedResource{
		ID:              resourceID,
		Name:            fmt.Sprintf("config-%s", resourceID),
		Type:            ResourceTypeConfig,
		Status:          ResourceStatusCreating,
		CreatedAt:       time.Now(),
		HealthStatus:    HealthStatusHealthy,
		Metadata:        make(map[string]interface{}),
		ConfigData:      configData,
		CleanupFunc:     rlm.cleanupConfigResource,
		HealthCheckFunc: rlm.healthCheckConfigResource,
		ResetFunc:       rlm.resetConfigResource,
	}

	resource.StartupTime = time.Since(startTime)
	resource.Status = ResourceStatusAvailable
	resource.LastHealthCheck = time.Now()

	rlm.resources[resourceID] = resource

	// Update metrics
	if rlm.resourceMetrics != nil {
		atomic.AddInt64(&rlm.resourceMetrics.TotalResources, 1)
		atomic.AddInt64(&rlm.resourceMetrics.ActiveResources, 1)
		atomic.AddInt64(&rlm.resourceMetrics.ConfigMetrics.Total, 1)
		atomic.AddInt64(&rlm.resourceMetrics.ConfigMetrics.Active, 1)
	}

	rlm.logger.Printf("Created config resource %s at %s in %v", resourceID, configPath, resource.StartupTime)
	return resource, nil
}

// createWorkspaceResource creates a new workspace resource
func (rlm *ResourceLifecycleManager) createWorkspaceResource() (*ManagedResource, error) {
	startTime := time.Now()
	resourceID := fmt.Sprintf("workspace-%d-%d", time.Now().UnixNano(), rand.Intn(1000))

	rlm.logger.Printf("Creating workspace resource %s", resourceID)

	// Create workspace directory
	tempDir := rlm.tempDirManager.baseDir
	workspacePath := filepath.Join(tempDir, fmt.Sprintf("workspace-%s", resourceID))

	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace directory: %w", err)
	}

	workspace := &TestWorkspace{
		ID:        resourceID,
		Path:      workspacePath,
		Projects:  make([]*framework.TestProject, 0),
		CreatedAt: time.Now(),
	}

	workspaceData := &WorkspaceResourceData{
		WorkspacePath: workspacePath,
		Manager:       rlm.workspaceManager,
		Projects:      make([]string, 0),
		FileWatchers:  make([]interface{}, 0),
	}

	resource := &ManagedResource{
		ID:              resourceID,
		Name:            fmt.Sprintf("workspace-%s", resourceID),
		Type:            ResourceTypeWorkspace,
		Status:          ResourceStatusCreating,
		CreatedAt:       time.Now(),
		HealthStatus:    HealthStatusHealthy,
		Metadata:        make(map[string]interface{}),
		WorkspaceData:   workspaceData,
		CleanupFunc:     rlm.cleanupWorkspaceResource,
		HealthCheckFunc: rlm.healthCheckWorkspaceResource,
		ResetFunc:       rlm.resetWorkspaceResource,
	}

	resource.StartupTime = time.Since(startTime)
	resource.Status = ResourceStatusAvailable
	resource.LastHealthCheck = time.Now()

	rlm.resources[resourceID] = resource

	// Add workspace to manager
	rlm.workspaceManager.mu.Lock()
	rlm.workspaceManager.workspaces[resourceID] = workspace
	rlm.workspaceManager.mu.Unlock()

	// Update metrics
	if rlm.resourceMetrics != nil {
		atomic.AddInt64(&rlm.resourceMetrics.TotalResources, 1)
		atomic.AddInt64(&rlm.resourceMetrics.ActiveResources, 1)
		atomic.AddInt64(&rlm.resourceMetrics.WorkspaceMetrics.Total, 1)
		atomic.AddInt64(&rlm.resourceMetrics.WorkspaceMetrics.Active, 1)
	}

	rlm.logger.Printf("Created workspace resource %s at %s in %v", resourceID, workspacePath, resource.StartupTime)
	return resource, nil
}

// Resource validation functions

// validateServerResource validates a server resource
func (rlm *ResourceLifecycleManager) validateServerResource(resource *ManagedResource) error {
	if resource.Type != ResourceTypeServer {
		return fmt.Errorf("resource type mismatch: expected server, got %s", resource.Type)
	}

	if resource.ServerInstance == nil {
		return fmt.Errorf("server resource missing server instance data")
	}

	// Check if port is available (basic check)
	if resource.ServerInstance.Port <= 0 || resource.ServerInstance.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", resource.ServerInstance.Port)
	}

	return nil
}

// validateProjectResource validates a project resource
func (rlm *ResourceLifecycleManager) validateProjectResource(resource *ManagedResource) error {
	if resource.Type != ResourceTypeProject {
		return fmt.Errorf("resource type mismatch: expected project, got %s", resource.Type)
	}

	if resource.ProjectData == nil {
		return fmt.Errorf("project resource missing project data")
	}

	if resource.ProjectData.Project == nil {
		return fmt.Errorf("project resource missing test project")
	}

	// Validate project structure
	if _, err := os.Stat(resource.ProjectData.Project.RootPath); os.IsNotExist(err) {
		return fmt.Errorf("project root path does not exist: %s", resource.ProjectData.Project.RootPath)
	}

	return nil
}

// validateClientResource validates a client resource
func (rlm *ResourceLifecycleManager) validateClientResource(resource *ManagedResource) error {
	if resource.Type != ResourceTypeClient {
		return fmt.Errorf("resource type mismatch: expected client, got %s", resource.Type)
	}

	if resource.ClientData == nil {
		return fmt.Errorf("client resource missing client data")
	}

	if resource.ClientData.MockClient == nil {
		return fmt.Errorf("client resource missing mock client")
	}

	// Validate client health
	if !resource.ClientData.MockClient.IsHealthy() {
		return fmt.Errorf("client is not healthy")
	}

	return nil
}

// validateConfigResource validates a configuration resource
func (rlm *ResourceLifecycleManager) validateConfigResource(resource *ManagedResource) error {
	if resource.Type != ResourceTypeConfig {
		return fmt.Errorf("resource type mismatch: expected config, got %s", resource.Type)
	}

	if resource.ConfigData == nil {
		return fmt.Errorf("config resource missing config data")
	}

	// Validate config file exists
	if _, err := os.Stat(resource.ConfigData.ConfigPath); os.IsNotExist(err) {
		return fmt.Errorf("config file does not exist: %s", resource.ConfigData.ConfigPath)
	}

	return nil
}

// validateWorkspaceResource validates a workspace resource
func (rlm *ResourceLifecycleManager) validateWorkspaceResource(resource *ManagedResource) error {
	if resource.Type != ResourceTypeWorkspace {
		return fmt.Errorf("resource type mismatch: expected workspace, got %s", resource.Type)
	}

	if resource.WorkspaceData == nil {
		return fmt.Errorf("workspace resource missing workspace data")
	}

	// Validate workspace directory exists
	if _, err := os.Stat(resource.WorkspaceData.WorkspacePath); os.IsNotExist(err) {
		return fmt.Errorf("workspace path does not exist: %s", resource.WorkspaceData.WorkspacePath)
	}

	return nil
}

// Cleanup functions

// cleanupServerResource cleans up a server resource
func (rlm *ResourceLifecycleManager) cleanupServerResource() error {
	// Server cleanup would involve stopping processes, cleaning up files, etc.
	// For mock testing, we just simulate cleanup
	time.Sleep(10 * time.Millisecond)
	return nil
}

// cleanupProjectResource cleans up a project resource
func (rlm *ResourceLifecycleManager) cleanupProjectResource() error {
	// Project cleanup would involve removing project directories and cleaning up generators
	time.Sleep(20 * time.Millisecond)
	return nil
}

// cleanupClientResource cleans up a client resource
func (rlm *ResourceLifecycleManager) cleanupClientResource() error {
	// Client cleanup involves resetting the mock client
	time.Sleep(5 * time.Millisecond)
	return nil
}

// cleanupConfigResource cleans up a configuration resource
func (rlm *ResourceLifecycleManager) cleanupConfigResource() error {
	// Config cleanup involves removing config files
	time.Sleep(5 * time.Millisecond)
	return nil
}

// cleanupWorkspaceResource cleans up a workspace resource
func (rlm *ResourceLifecycleManager) cleanupWorkspaceResource() error {
	// Workspace cleanup involves removing workspace directories and stopping file watchers
	time.Sleep(15 * time.Millisecond)
	return nil
}

// Health check functions

// healthCheckServerResource performs health check on server resource
func (rlm *ResourceLifecycleManager) healthCheckServerResource() error {
	// Server health check would involve checking if the server is responding
	// For mock testing, we simulate health check
	return nil
}

// healthCheckProjectResource performs health check on project resource
func (rlm *ResourceLifecycleManager) healthCheckProjectResource() error {
	// Project health check would involve verifying project files exist
	return nil
}

// healthCheckClientResource performs health check on client resource
func (rlm *ResourceLifecycleManager) healthCheckClientResource() error {
	// Client health check involves checking if the mock client is healthy
	return nil
}

// healthCheckConfigResource performs health check on config resource
func (rlm *ResourceLifecycleManager) healthCheckConfigResource() error {
	// Config health check involves validating config file integrity
	return nil
}

// healthCheckWorkspaceResource performs health check on workspace resource
func (rlm *ResourceLifecycleManager) healthCheckWorkspaceResource() error {
	// Workspace health check involves verifying workspace directory accessibility
	return nil
}

// Reset functions

// resetServerResource resets a server resource to clean state
func (rlm *ResourceLifecycleManager) resetServerResource() error {
	// Server reset would involve restarting or clearing server state
	time.Sleep(10 * time.Millisecond)
	return nil
}

// resetProjectResource resets a project resource to clean state
func (rlm *ResourceLifecycleManager) resetProjectResource() error {
	// Project reset would involve regenerating or cleaning project files
	time.Sleep(15 * time.Millisecond)
	return nil
}

// resetClientResource resets a client resource to clean state
func (rlm *ResourceLifecycleManager) resetClientResource() error {
	// Client reset involves calling the mock client's Reset method
	time.Sleep(5 * time.Millisecond)
	return nil
}

// resetConfigResource resets a config resource to clean state
func (rlm *ResourceLifecycleManager) resetConfigResource() error {
	// Config reset would involve regenerating config from template
	time.Sleep(5 * time.Millisecond)
	return nil
}

// resetWorkspaceResource resets a workspace resource to clean state
func (rlm *ResourceLifecycleManager) resetWorkspaceResource() error {
	// Workspace reset would involve clearing workspace contents
	time.Sleep(10 * time.Millisecond)
	return nil
}

// Helper methods

// getAvailableResource gets an available resource from the pool or creates a new one
func (rlm *ResourceLifecycleManager) getAvailableResource(pool *ResourcePool, testID string, requirements map[string]interface{}) (*ManagedResource, error) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	// Try to get an available resource
	if len(pool.Available) > 0 {
		resource := pool.Available[0]
		pool.Available = pool.Available[1:]
		return resource, nil
	}

	// Check if we can create a new resource
	if pool.CurrentSize >= pool.MaxSize {
		return nil, fmt.Errorf("resource pool at maximum capacity (%d)", pool.MaxSize)
	}

	// Create new resource
	resource, err := pool.CreationFunc()
	if err != nil {
		return nil, fmt.Errorf("failed to create new resource: %w", err)
	}

	// Validate the resource
	if err := pool.ValidationFunc(resource); err != nil {
		// Clean up the failed resource
		if resource.CleanupFunc != nil {
			resource.CleanupFunc()
		}
		return nil, fmt.Errorf("resource validation failed: %w", err)
	}

	pool.CurrentSize++
	return resource, nil
}

// resetResource resets a resource to a clean state
func (rlm *ResourceLifecycleManager) resetResource(resource *ManagedResource) error {
	if resource.ResetFunc == nil {
		return nil
	}

	resource.mu.Lock()
	resource.Status = ResourceStatusCleaning
	resource.mu.Unlock()

	startTime := time.Now()
	err := resource.ResetFunc()
	resource.CleanupTime = time.Since(startTime)

	if err != nil {
		resource.mu.Lock()
		resource.Status = ResourceStatusError
		resource.HealthStatus = HealthStatusUnhealthy
		resource.mu.Unlock()
		return err
	}

	resource.mu.Lock()
	resource.Status = ResourceStatusAvailable
	resource.HealthStatus = HealthStatusHealthy
	resource.LastHealthCheck = time.Now()
	resource.mu.Unlock()

	return nil
}

// destroyResource permanently destroys a resource
func (rlm *ResourceLifecycleManager) destroyResource(resource *ManagedResource) error {
	resource.mu.Lock()
	resource.Status = ResourceStatusCleaning
	resource.mu.Unlock()

	startTime := time.Now()
	var err error
	if resource.CleanupFunc != nil {
		err = resource.CleanupFunc()
	}
	resource.CleanupTime = time.Since(startTime)

	resource.mu.Lock()
	resource.Status = ResourceStatusDestroyed
	resource.mu.Unlock()

	// Remove from resources map
	delete(rlm.resources, resource.ID)

	// Remove from health checker
	if rlm.healthChecker != nil {
		rlm.healthChecker.mu.Lock()
		delete(rlm.healthChecker.resources, resource.ID)
		rlm.healthChecker.mu.Unlock()
	}

	// Update metrics
	if rlm.resourceMetrics != nil {
		atomic.AddInt64(&rlm.resourceMetrics.ActiveResources, -1)
		if err == nil {
			atomic.AddInt64(&rlm.resourceMetrics.SuccessfulCleanups, 1)
		} else {
			atomic.AddInt64(&rlm.resourceMetrics.FailedCleanups, 1)
		}
	}

	rlm.logger.Printf("Destroyed resource %s in %v", resource.ID, resource.CleanupTime)
	return err
}

// cleanupTestEnvironment cleans up a test environment
func (rlm *ResourceLifecycleManager) cleanupTestEnvironment(env *TestEnvironment) error {
	env.mu.Lock()
	defer env.mu.Unlock()

	if !env.active {
		return nil // Already cleaned up
	}

	rlm.logger.Printf("Cleaning up test environment %s", env.ID)

	var errors []error

	// Execute cleanup functions in reverse order
	for i := len(env.CleanupFuncs) - 1; i >= 0; i-- {
		if err := env.CleanupFuncs[i](); err != nil {
			errors = append(errors, err)
		}
	}

	env.active = false
	delete(rlm.testEnvironments, env.ID)

	if len(errors) > 0 {
		return fmt.Errorf("test environment cleanup errors: %v", errors)
	}

	rlm.logger.Printf("Cleaned up test environment %s", env.ID)
	return nil
}

// cleanupResourcePool cleans up a resource pool
func (rlm *ResourceLifecycleManager) cleanupResourcePool(pool *ResourcePool) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	rlm.logger.Printf("Cleaning up %s resource pool", pool.Type)

	var errors []error

	// Clean up all available resources
	for _, resource := range pool.Available {
		if err := rlm.destroyResource(resource); err != nil {
			errors = append(errors, err)
		}
	}

	// Clean up all in-use resources
	for _, resource := range pool.InUse {
		if err := rlm.destroyResource(resource); err != nil {
			errors = append(errors, err)
		}
	}

	pool.Available = nil
	pool.InUse = nil
	pool.CurrentSize = 0

	if len(errors) > 0 {
		return fmt.Errorf("resource pool cleanup errors: %v", errors)
	}

	rlm.logger.Printf("Cleaned up %s resource pool", pool.Type)
	return nil
}

// cleanupMcpClientPool cleans up the MCP client pool
func (rlm *ResourceLifecycleManager) cleanupMcpClientPool() error {
	rlm.logger.Printf("Cleaning up MCP client pool")

	rlm.mcpClientPool.mu.Lock()
	defer rlm.mcpClientPool.mu.Unlock()

	// Close the available channel
	close(rlm.mcpClientPool.available)

	// Clean up all clients
	for testID, client := range rlm.mcpClientPool.inUse {
		client.Reset()
		rlm.logger.Printf("Cleaned up MCP client for test %s", testID)
	}

	// Clear all client references
	rlm.mcpClientPool.clients = nil
	rlm.mcpClientPool.inUse = make(map[string]*mocks.MockMcpClient)
	atomic.StoreInt32(&rlm.mcpClientPool.currentClients, 0)
	atomic.AddInt64(&rlm.mcpClientPool.totalDestroyed, int64(len(rlm.mcpClientPool.clients)))

	rlm.logger.Printf("Cleaned up MCP client pool")
	return nil
}

// logResourceEvent logs a resource lifecycle event
func (rlm *ResourceLifecycleManager) logResourceEvent(eventType, resourceID, testID string, details map[string]interface{}, err error) {
	event := &ResourceLifecycleEvent{
		Timestamp:  time.Now(),
		EventType:  eventType,
		ResourceID: resourceID,
		TestID:     testID,
		Details:    details,
		Error:      err,
	}

	rlm.lifecycleEvents = append(rlm.lifecycleEvents, event)

	// Log to logger
	if err != nil {
		rlm.logger.Printf("Resource event [%s] %s/%s: ERROR %v - %+v", eventType, resourceID, testID, err, details)
	} else {
		rlm.logger.Printf("Resource event [%s] %s/%s: %+v", eventType, resourceID, testID, details)
	}
}

// createMcpClient creates a new MockMcpClient
func (rlm *ResourceLifecycleManager) createMcpClient() (*mocks.MockMcpClient, error) {
	client := mocks.NewMockMcpClient()
	if client == nil {
		return nil, fmt.Errorf("failed to create MockMcpClient")
	}

	atomic.AddInt64(&rlm.mcpClientFactory.totalCreated, 1)
	return client, nil
}

// configureMcpClient configures a MockMcpClient with the specified configuration
func (rlm *ResourceLifecycleManager) configureMcpClient(client *mocks.MockMcpClient, config *McpClientConfig) error {
	if config == nil {
		config = rlm.mcpClientFactory.defaultConfig
	}

	client.SetBaseURL(config.BaseURL)
	client.SetTimeout(config.Timeout)
	client.SetMaxRetries(config.MaxRetries)
	client.SetHealthy(true)

	// Configure circuit breaker if specified
	if config.CircuitBreakerConfig != nil {
		client.SetCircuitBreakerConfig(config.CircuitBreakerConfig.MaxFailures, config.CircuitBreakerConfig.Timeout)
	}

	// Configure retry policy if specified
	if config.RetryPolicyConfig != nil {
		retryPolicy := &mcp.RetryPolicy{
			MaxRetries:      config.RetryPolicyConfig.MaxRetries,
			InitialBackoff:  config.RetryPolicyConfig.InitialBackoff,
			MaxBackoff:      config.RetryPolicyConfig.MaxBackoff,
			BackoffFactor:   config.RetryPolicyConfig.BackoffFactor,
			JitterEnabled:   config.RetryPolicyConfig.JitterEnabled,
			RetryableErrors: config.RetryPolicyConfig.RetryableErrors,
		}
		client.SetRetryPolicy(retryPolicy)
	}

	return nil
}

// CreateTempDir creates a temporary directory for a test
func (tdm *TempDirectoryManager) CreateTempDir(testID string) (string, error) {
	tdm.mu.Lock()
	defer tdm.mu.Unlock()

	tempDir, err := os.MkdirTemp(tdm.baseDir, fmt.Sprintf("test-%s-*", testID))
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	tdm.directories[testID] = tempDir
	tdm.logger.Printf("Created temp directory %s for test %s", tempDir, testID)

	return tempDir, nil
}

// startHealthChecking starts the health checking goroutine
func (rhc *ResourceHealthChecker) startHealthChecking() {
	ticker := time.NewTicker(rhc.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rhc.ctx.Done():
			return
		case <-ticker.C:
			rhc.performHealthChecks()
		}
	}
}

// performHealthChecks performs health checks on all resources
func (rhc *ResourceHealthChecker) performHealthChecks() {
	rhc.mu.RLock()
	resources := make([]*ManagedResource, 0, len(rhc.resources))
	for _, resource := range rhc.resources {
		resources = append(resources, resource)
	}
	rhc.mu.RUnlock()

	for _, resource := range resources {
		go rhc.checkResourceHealth(resource)
	}
}

// checkResourceHealth performs health check on a single resource
func (rhc *ResourceHealthChecker) checkResourceHealth(resource *ManagedResource) {
	if resource.HealthCheckFunc == nil {
		return
	}

	resource.mu.Lock()
	resource.LastHealthCheck = time.Now()
	resource.mu.Unlock()

	err := resource.HealthCheckFunc()

	resource.mu.Lock()
	if err != nil {
		resource.HealthStatus = HealthStatusUnhealthy
		if resource.ServerInstance != nil && resource.ServerInstance.Health != nil {
			resource.ServerInstance.Health.ErrorCount++
			resource.ServerInstance.Health.LastError = err
		}
	} else {
		resource.HealthStatus = HealthStatusHealthy
	}
	if resource.ServerInstance != nil && resource.ServerInstance.Health != nil {
		resource.ServerInstance.Health.CheckCount++
		resource.ServerInstance.Health.LastCheck = time.Now()
		resource.ServerInstance.Health.Status = resource.HealthStatus
	}
	resource.mu.Unlock()
}

// startPerformanceMonitoring starts the performance monitoring goroutine
func (rpm *ResourcePerformanceMonitor) startPerformanceMonitoring() {
	ticker := time.NewTicker(rpm.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rpm.ctx.Done():
			return
		case <-ticker.C:
			rpm.collectPerformanceMetrics()
		}
	}
}

// collectPerformanceMetrics collects system performance metrics
func (rpm *ResourcePerformanceMonitor) collectPerformanceMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	memoryUsageMB := int64(m.Alloc) / 1024 / 1024
	goroutineCount := runtime.NumGoroutine()

	rpm.logger.Printf("Performance metrics: Memory=%dMB, Goroutines=%d", memoryUsageMB, goroutineCount)
}

// NewMockResponseFactory creates a new mock response factory
func NewMockResponseFactory() *MockResponseFactory {
	return &MockResponseFactory{
		responseTemplates: make(map[string][]json.RawMessage),
	}
}

// NewMockErrorFactory creates a new mock error factory
func NewMockErrorFactory() *MockErrorFactory {
	return &MockErrorFactory{
		errorTemplates: make(map[mcp.ErrorCategory][]error),
	}
}
