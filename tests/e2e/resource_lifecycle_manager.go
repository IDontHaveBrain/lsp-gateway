package e2e_test

import (
	"context"
	"fmt"
	"log"
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

// ResourceLifecycleManager provides comprehensive resource management for E2E tests
type ResourceLifecycleManager struct {
	// Core resource management
	resources        map[string]*ManagedResource
	resourcePools    map[ResourceType]*ResourcePool
	allocations      map[string]*ResourceAllocation
	
	// MockMcpClient management
	mcpClientPool    *McpClientPool
	mcpClientFactory *McpClientFactory
	
	// Environment management
	testEnvironments map[string]*TestEnvironment
	workspaceManager *TestWorkspaceManager
	tempDirManager   *TempDirectoryManager
	
	// Lifecycle tracking
	lifecycleEvents  []*ResourceLifecycleEvent
	resourceMetrics  *ResourceMetrics
	healthChecker    *ResourceHealthChecker
	
	// Synchronization and coordination
	mu               sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
	cleanupFunctions []CleanupFunction
	
	// Configuration
	config           *ResourceManagerConfig
	logger           *log.Logger
	
	// Performance monitoring
	performanceMonitor *ResourcePerformanceMonitor
	resourceUsageTracker *ResourceUsageTracker
}

// ManagedResource represents a resource under lifecycle management
type ManagedResource struct {
	ID           string
	Name         string
	Type         ResourceType
	Status       ResourceStatus
	Owner        string
	TestID       string
	
	// Resource details
	CreatedAt    time.Time
	LastUsed     time.Time
	UsageCount   int64
	Metadata     map[string]interface{}
	
	// Lifecycle management
	StartupTime  time.Duration
	CleanupTime  time.Duration
	HealthStatus ResourceHealthStatus
	LastHealthCheck time.Time
	
	// Resource-specific data
	ServerInstance  *ServerResourceData
	ProjectData     *ProjectResourceData
	ClientData      *ClientResourceData
	ConfigData      *ConfigResourceData
	WorkspaceData   *WorkspaceResourceData
	
	// Cleanup and management
	CleanupFunc     CleanupFunction
	HealthCheckFunc HealthCheckFunction
	ResetFunc       ResetFunction
	
	// Synchronization
	mu              sync.RWMutex
	inUse           bool
	references      int32
}

// ResourcePool manages pools of similar resources for efficient allocation
type ResourcePool struct {
	Type            ResourceType
	MaxSize         int
	MinSize         int
	CurrentSize     int
	Available       []*ManagedResource
	InUse          []*ManagedResource
	CreationFunc    ResourceCreationFunction
	ValidationFunc  ResourceValidationFunction
	
	// Pool statistics
	TotalAllocations   int64
	TotalDeallocations int64
	PeakUsage         int
	AverageUsage      float64
	
	// Pool management
	mu              sync.RWMutex
	creationTimeout time.Duration
	maxWaitTime     time.Duration
}

// McpClientPool manages a pool of MockMcpClient instances for parallel testing
type McpClientPool struct {
	clients         []*mocks.MockMcpClient
	available       chan *mocks.MockMcpClient
	inUse          map[string]*mocks.MockMcpClient
	factory        *McpClientFactory
	
	// Pool configuration
	maxClients      int
	minClients      int
	currentClients  int32
	
	// Pool statistics
	totalCreated    int64
	totalDestroyed  int64
	peakUsage      int32
	
	// Synchronization
	mu             sync.RWMutex
	ctx            context.Context
	cleanupTimeout time.Duration
}

// McpClientFactory creates and configures MockMcpClient instances
type McpClientFactory struct {
	defaultConfig   *McpClientConfig
	responseFactory *MockResponseFactory
	errorFactory    *MockErrorFactory
	metricsTemplate *mcp.ConnectionMetrics
	
	// Factory statistics
	totalCreated    int64
	creationTime    time.Duration
	mu             sync.RWMutex
}

// TestEnvironment represents an isolated test environment
type TestEnvironment struct {
	ID              string
	Name            string
	TempDir         string
	ProjectsDir     string
	ConfigDir       string
	LogDir          string
	
	// Environment resources
	Projects        map[string]*framework.TestProject
	Servers         map[string]*ServerResourceData
	Clients         map[string]*ClientResourceData
	Configs         map[string]*ConfigResourceData
	
	// Environment lifecycle
	CreatedAt       time.Time
	LastUsed        time.Time
	CleanupFuncs    []CleanupFunction
	
	// Environment isolation
	ProcessGroup    int
	NetworkNamespace string
	ResourceLimits  *ResourceLimits
	
	// Synchronization
	mu             sync.RWMutex
	active         bool
	refCount       int32
}

// Resource-specific data structures
type ServerResourceData struct {
	Port        int
	PID         int
	ConfigPath  string
	LogPath     string
	Gateway     interface{}
	Transport   interface{}
	Health      *HealthStatus
}

type ProjectResourceData struct {
	Project     *framework.TestProject
	Generator   interface{}
	ContentMgr  interface{}
	FileCount   int
	TotalSize   int64
}

type ClientResourceData struct {
	MockClient  *mocks.MockMcpClient
	Config      *McpClientConfig
	Metrics     *mcp.ConnectionMetrics
	State       ClientResourceState
}

type ConfigResourceData struct {
	ConfigPath  string
	Config      interface{}
	Template    string
	Validation  *ConfigValidationResult
}

type WorkspaceResourceData struct {
	WorkspacePath string
	Manager       interface{}
	Projects      []string
	FileWatchers  []interface{}
}

// Configuration and state types
type ResourceManagerConfig struct {
	// Resource pool settings
	MaxServerInstances      int
	MaxProjectInstances     int
	MaxClientInstances      int
	MaxWorkspaceInstances   int
	
	// Timeout configurations
	ResourceCreationTimeout time.Duration
	ResourceCleanupTimeout  time.Duration
	HealthCheckInterval     time.Duration
	ResourcePoolTimeout     time.Duration
	
	// MockMcpClient settings
	McpClientPoolSize       int
	McpClientTimeout        time.Duration
	MockResponseQueueSize   int
	ErrorSimulationEnabled  bool
	
	// Environment isolation
	TempDirPrefix          string
	CleanupOnFailure       bool
	PreserveOnDebug        bool
	MaxConcurrentTests     int
	
	// Performance settings
	ResourceMetricsEnabled bool
	PerformanceMonitoring  bool
	MemoryThresholdMB      int64
	CPUThresholdPercent    float64
}

type McpClientConfig struct {
	BaseURL            string
	Timeout            time.Duration
	MaxRetries         int
	CircuitBreakerConfig *CircuitBreakerConfig
	RetryPolicyConfig    *RetryPolicyConfig
	MetricsConfig        *MetricsConfig
	ResponseQueueSize    int
	ErrorQueueSize       int
}

type CircuitBreakerConfig struct {
	MaxFailures    int
	Timeout        time.Duration
	MaxRequests    int
	SuccessThreshold int
}

type RetryPolicyConfig struct {
	MaxRetries      int
	InitialBackoff  time.Duration
	MaxBackoff      time.Duration
	BackoffFactor   float64
	JitterEnabled   bool
	RetryableErrors map[mcp.ErrorCategory]bool
}

type MetricsConfig struct {
	CollectionEnabled bool
	HistogramBuckets  []float64
	MetricsInterval   time.Duration
}

// Status and state enums
type ResourceStatus string

const (
	ResourceStatusCreating   ResourceStatus = "creating"
	ResourceStatusAvailable  ResourceStatus = "available"
	ResourceStatusAllocated  ResourceStatus = "allocated"
	ResourceStatusInUse      ResourceStatus = "in_use"
	ResourceStatusReleasing  ResourceStatus = "releasing"
	ResourceStatusCleaning   ResourceStatus = "cleaning"
	ResourceStatusDestroyed  ResourceStatus = "destroyed"
	ResourceStatusError      ResourceStatus = "error"
)

type ResourceHealthStatus string

const (
	HealthStatusHealthy    ResourceHealthStatus = "healthy"
	HealthStatusDegraded   ResourceHealthStatus = "degraded"
	HealthStatusUnhealthy  ResourceHealthStatus = "unhealthy"
	HealthStatusUnknown    ResourceHealthStatus = "unknown"
)

type ClientResourceState string

const (
	ClientStateInitialized ClientResourceState = "initialized"
	ClientStateConfigured  ClientResourceState = "configured"
	ClientStateActive      ClientResourceState = "active"
	ClientStateError       ClientResourceState = "error"
	ClientStateResetting   ClientResourceState = "resetting"
)

// Function types for resource management
type CleanupFunction func() error
type HealthCheckFunction func() error
type ResetFunction func() error
type ResourceCreationFunction func() (*ManagedResource, error)
type ResourceValidationFunction func(*ManagedResource) error

// Event and monitoring types
type ResourceLifecycleEvent struct {
	Timestamp   time.Time
	EventType   string
	ResourceID  string
	TestID      string
	Details     map[string]interface{}
	Error       error
}

type ResourceMetrics struct {
	TotalResources      int64
	ActiveResources     int64
	AllocatedResources  int64
	FailedAllocations   int64
	SuccessfulCleanups  int64
	FailedCleanups      int64
	AverageLifetime     time.Duration
	PeakMemoryUsage     int64
	PeakCPUUsage        float64
	
	// Per-type metrics
	ServerMetrics    *ResourceTypeMetrics
	ProjectMetrics   *ResourceTypeMetrics
	ClientMetrics    *ResourceTypeMetrics
	ConfigMetrics    *ResourceTypeMetrics
	WorkspaceMetrics *ResourceTypeMetrics
}

type ResourceTypeMetrics struct {
	Total       int64
	Active      int64
	Failed      int64
	AvgLifetime time.Duration
	AvgSize     int64
}

// Helper types
type ResourceAllocation struct {
	ResourceID   string
	TestID       string
	AllocatedAt  time.Time
	ExpiresAt    time.Time
	Priority     int
	Metadata     map[string]interface{}
}

type ResourceLimits struct {
	MaxMemoryMB  int64
	MaxCPUPercent float64
	MaxFiles     int
	MaxProcesses int
}

type HealthStatus struct {
	Status      ResourceHealthStatus
	LastCheck   time.Time
	CheckCount  int64
	ErrorCount  int64
	LastError   error
	Metadata    map[string]interface{}
}

type ConfigValidationResult struct {
	Valid       bool
	Errors      []error
	Warnings    []string
	ValidatedAt time.Time
}

// NewResourceLifecycleManager creates a new comprehensive resource lifecycle manager
func NewResourceLifecycleManager(config *ResourceManagerConfig, logger *log.Logger) *ResourceLifecycleManager {
	if config == nil {
		config = getDefaultResourceManagerConfig()
	}
	
	if logger == nil {
		logger = log.New(os.Stdout, "[ResourceLifecycle] ", log.LstdFlags|log.Lshortfile)
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	rlm := &ResourceLifecycleManager{
		resources:           make(map[string]*ManagedResource),
		resourcePools:       make(map[ResourceType]*ResourcePool),
		allocations:         make(map[string]*ResourceAllocation),
		testEnvironments:    make(map[string]*TestEnvironment),
		lifecycleEvents:     make([]*ResourceLifecycleEvent, 0),
		cleanupFunctions:    make([]CleanupFunction, 0),
		config:              config,
		logger:              logger,
		ctx:                 ctx,
		cancel:              cancel,
	}
	
	// Initialize components
	rlm.initializeResourcePools()
	rlm.initializeMcpClientPool()
	rlm.initializeWorkspaceManager()
	rlm.initializeTempDirectoryManager()
	rlm.initializeResourceMetrics()
	rlm.initializeHealthChecker()
	rlm.initializePerformanceMonitor()
	
	logger.Printf("ResourceLifecycleManager initialized with config: %+v", config)
	
	return rlm
}

// initializeResourcePools sets up resource pools for different resource types
func (rlm *ResourceLifecycleManager) initializeResourcePools() {
	// Server resource pool
	rlm.resourcePools[ResourceTypeServer] = &ResourcePool{
		Type:            ResourceTypeServer,
		MaxSize:         rlm.config.MaxServerInstances,
		MinSize:         1,
		Available:       make([]*ManagedResource, 0),
		InUse:          make([]*ManagedResource, 0),
		CreationFunc:    rlm.createServerResource,
		ValidationFunc:  rlm.validateServerResource,
		creationTimeout: rlm.config.ResourceCreationTimeout,
		maxWaitTime:     30 * time.Second,
	}
	
	// Project resource pool
	rlm.resourcePools[ResourceTypeProject] = &ResourcePool{
		Type:            ResourceTypeProject,
		MaxSize:         rlm.config.MaxProjectInstances,
		MinSize:         0,
		Available:       make([]*ManagedResource, 0),
		InUse:          make([]*ManagedResource, 0),
		CreationFunc:    rlm.createProjectResource,
		ValidationFunc:  rlm.validateProjectResource,
		creationTimeout: rlm.config.ResourceCreationTimeout,
		maxWaitTime:     60 * time.Second,
	}
	
	// Client resource pool
	rlm.resourcePools[ResourceTypeClient] = &ResourcePool{
		Type:            ResourceTypeClient,
		MaxSize:         rlm.config.MaxClientInstances,
		MinSize:         2,
		Available:       make([]*ManagedResource, 0),
		InUse:          make([]*ManagedResource, 0),
		CreationFunc:    rlm.createClientResource,
		ValidationFunc:  rlm.validateClientResource,
		creationTimeout: 10 * time.Second,
		maxWaitTime:     15 * time.Second,
	}
	
	// Config resource pool
	rlm.resourcePools[ResourceTypeConfig] = &ResourcePool{
		Type:            ResourceTypeConfig,
		MaxSize:         20,
		MinSize:         1,
		Available:       make([]*ManagedResource, 0),
		InUse:          make([]*ManagedResource, 0),
		CreationFunc:    rlm.createConfigResource,
		ValidationFunc:  rlm.validateConfigResource,
		creationTimeout: 5 * time.Second,
		maxWaitTime:     10 * time.Second,
	}
	
	// Workspace resource pool
	rlm.resourcePools[ResourceTypeWorkspace] = &ResourcePool{
		Type:            ResourceTypeWorkspace,
		MaxSize:         rlm.config.MaxWorkspaceInstances,
		MinSize:         0,
		Available:       make([]*ManagedResource, 0),
		InUse:          make([]*ManagedResource, 0),
		CreationFunc:    rlm.createWorkspaceResource,
		ValidationFunc:  rlm.validateWorkspaceResource,
		creationTimeout: rlm.config.ResourceCreationTimeout,
		maxWaitTime:     45 * time.Second,
	}
	
	rlm.logger.Printf("Initialized %d resource pools", len(rlm.resourcePools))
}

// initializeMcpClientPool sets up the MockMcpClient pool
func (rlm *ResourceLifecycleManager) initializeMcpClientPool() {
	defaultConfig := &McpClientConfig{
		BaseURL:    "http://localhost:8080",
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		CircuitBreakerConfig: &CircuitBreakerConfig{
			MaxFailures:      5,
			Timeout:          60 * time.Second,
			MaxRequests:      3,
			SuccessThreshold: 2,
		},
		RetryPolicyConfig: &RetryPolicyConfig{
			MaxRetries:     3,
			InitialBackoff: 500 * time.Millisecond,
			MaxBackoff:     30 * time.Second,
			BackoffFactor:  2.0,
			JitterEnabled:  true,
			RetryableErrors: map[mcp.ErrorCategory]bool{
				mcp.ErrorCategoryNetwork:   true,
				mcp.ErrorCategoryTimeout:   true,
				mcp.ErrorCategoryServer:    true,
				mcp.ErrorCategoryRateLimit: true,
				mcp.ErrorCategoryClient:    false,
				mcp.ErrorCategoryProtocol:  false,
				mcp.ErrorCategoryUnknown:   true,
			},
		},
		MetricsConfig: &MetricsConfig{
			CollectionEnabled: rlm.config.ResourceMetricsEnabled,
			HistogramBuckets:  []float64{0.01, 0.05, 0.1, 0.5, 1, 2.5, 5, 10},
			MetricsInterval:   5 * time.Second,
		},
		ResponseQueueSize: rlm.config.MockResponseQueueSize,
		ErrorQueueSize:    rlm.config.MockResponseQueueSize / 4,
	}
	
	rlm.mcpClientFactory = &McpClientFactory{
		defaultConfig:   defaultConfig,
		responseFactory: NewMockResponseFactory(),
		errorFactory:    NewMockErrorFactory(),
		metricsTemplate: &mcp.ConnectionMetrics{
			TotalRequests:    0,
			SuccessfulReqs:   0,
			FailedRequests:   0,
			TimeoutCount:     0,
			ConnectionErrors: 0,
			AverageLatency:   100 * time.Millisecond,
			LastRequestTime:  time.Now(),
			LastSuccessTime:  time.Now(),
		},
	}
	
	rlm.mcpClientPool = &McpClientPool{
		clients:        make([]*mocks.MockMcpClient, 0, rlm.config.McpClientPoolSize),
		available:      make(chan *mocks.MockMcpClient, rlm.config.McpClientPoolSize),
		inUse:         make(map[string]*mocks.MockMcpClient),
		factory:       rlm.mcpClientFactory,
		maxClients:    rlm.config.McpClientPoolSize,
		minClients:    2,
		ctx:           rlm.ctx,
		cleanupTimeout: rlm.config.ResourceCleanupTimeout,
	}
	
	// Pre-populate the pool with minimum clients
	for i := 0; i < rlm.mcpClientPool.minClients; i++ {
		if client, err := rlm.createMcpClient(); err == nil {
			rlm.mcpClientPool.clients = append(rlm.mcpClientPool.clients, client)
			rlm.mcpClientPool.available <- client
			atomic.AddInt32(&rlm.mcpClientPool.currentClients, 1)
			atomic.AddInt64(&rlm.mcpClientPool.totalCreated, 1)
		} else {
			rlm.logger.Printf("Failed to create initial MCP client: %v", err)
		}
	}
	
	rlm.logger.Printf("Initialized MCP client pool with %d initial clients", rlm.mcpClientPool.minClients)
}

// AllocateResource allocates a resource for a specific test
func (rlm *ResourceLifecycleManager) AllocateResource(testID string, resourceType ResourceType, requirements map[string]interface{}) (*ManagedResource, error) {
	rlm.mu.Lock()
	defer rlm.mu.Unlock()
	
	rlm.logger.Printf("Allocating %s resource for test %s", resourceType, testID)
	
	pool, exists := rlm.resourcePools[resourceType]
	if !exists {
		return nil, fmt.Errorf("no resource pool found for type %s", resourceType)
	}
	
	// Try to get an available resource from the pool
	resource, err := rlm.getAvailableResource(pool, testID, requirements)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate %s resource: %w", resourceType, err)
	}
	
	// Mark resource as allocated
	resource.mu.Lock()
	resource.Status = ResourceStatusAllocated
	resource.Owner = testID
	resource.TestID = testID
	resource.LastUsed = time.Now()
	resource.UsageCount++
	resource.inUse = true
	atomic.AddInt32(&resource.references, 1)
	resource.mu.Unlock()
	
	// Create allocation record
	allocation := &ResourceAllocation{
		ResourceID:  resource.ID,
		TestID:      testID,
		AllocatedAt: time.Now(),
		ExpiresAt:   time.Now().Add(2 * time.Hour), // Default 2-hour expiration
		Priority:    1,
		Metadata:    requirements,
	}
	rlm.allocations[resource.ID] = allocation
	
	// Update pool statistics
	pool.mu.Lock()
	pool.TotalAllocations++
	// Move from available to in-use
	for i, res := range pool.Available {
		if res.ID == resource.ID {
			pool.Available = append(pool.Available[:i], pool.Available[i+1:]...)
			break
		}
	}
	pool.InUse = append(pool.InUse, resource)
	if len(pool.InUse) > pool.PeakUsage {
		pool.PeakUsage = len(pool.InUse)
	}
	pool.mu.Unlock()
	
	// Log allocation event
	rlm.logResourceEvent("resource_allocated", resource.ID, testID, map[string]interface{}{
		"resource_type": resourceType,
		"requirements":  requirements,
	}, nil)
	
	// Update metrics
	if rlm.resourceMetrics != nil {
		atomic.AddInt64(&rlm.resourceMetrics.AllocatedResources, 1)
	}
	
	rlm.logger.Printf("Successfully allocated %s resource %s for test %s", resourceType, resource.ID, testID)
	return resource, nil
}

// ReleaseResource releases a resource and returns it to the pool
func (rlm *ResourceLifecycleManager) ReleaseResource(resourceID, testID string) error {
	rlm.mu.Lock()
	defer rlm.mu.Unlock()
	
	rlm.logger.Printf("Releasing resource %s for test %s", resourceID, testID)
	
	resource, exists := rlm.resources[resourceID]
	if !exists {
		return fmt.Errorf("resource %s not found", resourceID)
	}
	
	// Validate ownership
	if resource.Owner != testID {
		return fmt.Errorf("resource %s is owned by %s, not %s", resourceID, resource.Owner, testID)
	}
	
	resource.mu.Lock()
	resource.Status = ResourceStatusReleasing
	resource.inUse = false
	refs := atomic.AddInt32(&resource.references, -1)
	resource.mu.Unlock()
	
	// Clean up allocation record
	delete(rlm.allocations, resourceID)
	
	// If no more references, reset the resource
	if refs == 0 {
		if err := rlm.resetResource(resource); err != nil {
			rlm.logger.Printf("Failed to reset resource %s: %v", resourceID, err)
			return rlm.destroyResource(resource)
		}
		
		// Return to pool
		pool := rlm.resourcePools[resource.Type]
		pool.mu.Lock()
		
		// Move from in-use to available
		for i, res := range pool.InUse {
			if res.ID == resourceID {
				pool.InUse = append(pool.InUse[:i], pool.InUse[i+1:]...)
				break
			}
		}
		pool.Available = append(pool.Available, resource)
		pool.TotalDeallocations++
		pool.mu.Unlock()
		
		resource.mu.Lock()
		resource.Status = ResourceStatusAvailable
		resource.Owner = ""
		resource.TestID = ""
		resource.mu.Unlock()
	}
	
	// Log release event
	rlm.logResourceEvent("resource_released", resourceID, testID, map[string]interface{}{
		"remaining_references": refs,
	}, nil)
	
	// Update metrics
	if rlm.resourceMetrics != nil {
		atomic.AddInt64(&rlm.resourceMetrics.AllocatedResources, -1)
	}
	
	rlm.logger.Printf("Successfully released resource %s for test %s", resourceID, testID)
	return nil
}

// AllocateMcpClient allocates a MockMcpClient from the pool
func (rlm *ResourceLifecycleManager) AllocateMcpClient(testID string, config *McpClientConfig) (*mocks.MockMcpClient, error) {
	rlm.logger.Printf("Allocating MCP client for test %s", testID)
	
	var client *mocks.MockMcpClient
	
	select {
	case client = <-rlm.mcpClientPool.available:
		// Got a client from the pool
	case <-time.After(10 * time.Second):
		// Timeout waiting for available client, try to create new one
		if atomic.LoadInt32(&rlm.mcpClientPool.currentClients) < int32(rlm.mcpClientPool.maxClients) {
			newClient, err := rlm.createMcpClient()
			if err != nil {
				return nil, fmt.Errorf("failed to create new MCP client: %w", err)
			}
			client = newClient
			atomic.AddInt32(&rlm.mcpClientPool.currentClients, 1)
			atomic.AddInt64(&rlm.mcpClientPool.totalCreated, 1)
		} else {
			return nil, fmt.Errorf("timeout waiting for available MCP client and pool is at capacity")
		}
	}
	
	// Configure the client
	if err := rlm.configureMcpClient(client, config); err != nil {
		// Return client to pool and return error
		select {
		case rlm.mcpClientPool.available <- client:
		default:
			// Pool full, destroy client
			atomic.AddInt32(&rlm.mcpClientPool.currentClients, -1)
		}
		return nil, fmt.Errorf("failed to configure MCP client: %w", err)
	}
	
	// Track client allocation
	rlm.mcpClientPool.mu.Lock()
	rlm.mcpClientPool.inUse[testID] = client
	currentUsage := int32(len(rlm.mcpClientPool.inUse))
	if currentUsage > atomic.LoadInt32(&rlm.mcpClientPool.peakUsage) {
		atomic.StoreInt32(&rlm.mcpClientPool.peakUsage, currentUsage)
	}
	rlm.mcpClientPool.mu.Unlock()
	
	rlm.logger.Printf("Successfully allocated MCP client for test %s", testID)
	return client, nil
}

// ReleaseMcpClient returns a MockMcpClient to the pool
func (rlm *ResourceLifecycleManager) ReleaseMcpClient(testID string) error {
	rlm.logger.Printf("Releasing MCP client for test %s", testID)
	
	rlm.mcpClientPool.mu.Lock()
	client, exists := rlm.mcpClientPool.inUse[testID]
	if !exists {
		rlm.mcpClientPool.mu.Unlock()
		return fmt.Errorf("no MCP client allocated for test %s", testID)
	}
	delete(rlm.mcpClientPool.inUse, testID)
	rlm.mcpClientPool.mu.Unlock()
	
	// Reset client state
	client.Reset()
	client.SetHealthy(true)
	
	// Return to pool
	select {
	case rlm.mcpClientPool.available <- client:
		rlm.logger.Printf("Returned MCP client to pool for test %s", testID)
	default:
		// Pool is full, destroy the client
		atomic.AddInt32(&rlm.mcpClientPool.currentClients, -1)
		atomic.AddInt64(&rlm.mcpClientPool.totalDestroyed, 1)
		rlm.logger.Printf("Pool full, destroyed MCP client for test %s", testID)
	}
	
	return nil
}

// CreateTestEnvironment creates an isolated test environment
func (rlm *ResourceLifecycleManager) CreateTestEnvironment(testID string, requirements *EnvironmentRequirements) (*TestEnvironment, error) {
	rlm.mu.Lock()
	defer rlm.mu.Unlock()
	
	rlm.logger.Printf("Creating test environment for test %s", testID)
	
	// Create temporary directory structure
	tempDir, err := rlm.tempDirManager.CreateTempDir(testID)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	
	env := &TestEnvironment{
		ID:          testID,
		Name:        fmt.Sprintf("test-env-%s", testID),
		TempDir:     tempDir,
		ProjectsDir: filepath.Join(tempDir, "projects"),
		ConfigDir:   filepath.Join(tempDir, "configs"),
		LogDir:      filepath.Join(tempDir, "logs"),
		Projects:    make(map[string]*framework.TestProject),
		Servers:     make(map[string]*ServerResourceData),
		Clients:     make(map[string]*ClientResourceData),
		Configs:     make(map[string]*ConfigResourceData),
		CreatedAt:   time.Now(),
		CleanupFuncs: make([]CleanupFunction, 0),
		active:      true,
		refCount:    1,
	}
	
	// Create directory structure
	for _, dir := range []string{env.ProjectsDir, env.ConfigDir, env.LogDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			rlm.cleanupTestEnvironment(env)
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	
	// Set resource limits if specified
	if requirements != nil && requirements.ResourceLimits != nil {
		env.ResourceLimits = requirements.ResourceLimits
	}
	
	// Add cleanup function
	env.CleanupFuncs = append(env.CleanupFuncs, func() error {
		return os.RemoveAll(env.TempDir)
	})
	
	rlm.testEnvironments[testID] = env
	
	rlm.logger.Printf("Created test environment %s at %s", testID, tempDir)
	return env, nil
}

// CleanupTestEnvironment cleans up a test environment
func (rlm *ResourceLifecycleManager) CleanupTestEnvironment(testID string) error {
	rlm.mu.Lock()
	defer rlm.mu.Unlock()
	
	env, exists := rlm.testEnvironments[testID]
	if !exists {
		return nil // Already cleaned up
	}
	
	return rlm.cleanupTestEnvironment(env)
}

// CleanupAll performs comprehensive cleanup of all resources
func (rlm *ResourceLifecycleManager) CleanupAll() error {
	rlm.logger.Printf("Starting comprehensive resource cleanup")
	
	var errors []error
	
	// Cleanup all test environments
	rlm.mu.Lock()
	environments := make([]*TestEnvironment, 0, len(rlm.testEnvironments))
	for _, env := range rlm.testEnvironments {
		environments = append(environments, env)
	}
	rlm.mu.Unlock()
	
	for _, env := range environments {
		if err := rlm.cleanupTestEnvironment(env); err != nil {
			errors = append(errors, fmt.Errorf("failed to cleanup environment %s: %w", env.ID, err))
		}
	}
	
	// Cleanup all resource pools
	for resourceType, pool := range rlm.resourcePools {
		if err := rlm.cleanupResourcePool(pool); err != nil {
			errors = append(errors, fmt.Errorf("failed to cleanup %s pool: %w", resourceType, err))
		}
	}
	
	// Cleanup MCP client pool
	if err := rlm.cleanupMcpClientPool(); err != nil {
		errors = append(errors, fmt.Errorf("failed to cleanup MCP client pool: %w", err))
	}
	
	// Execute cleanup functions
	for i := len(rlm.cleanupFunctions) - 1; i >= 0; i-- {
		if err := rlm.cleanupFunctions[i](); err != nil {
			errors = append(errors, err)
		}
	}
	
	// Cancel context
	if rlm.cancel != nil {
		rlm.cancel()
	}
	
	if len(errors) > 0 {
		rlm.logger.Printf("Resource cleanup completed with %d errors", len(errors))
		return fmt.Errorf("cleanup errors: %v", errors)
	}
	
	rlm.logger.Printf("Resource cleanup completed successfully")
	return nil
}

// GetResourceMetrics returns current resource metrics
func (rlm *ResourceLifecycleManager) GetResourceMetrics() *ResourceMetrics {
	if rlm.resourceMetrics == nil {
		return nil
	}
	
	// Create a copy to avoid race conditions
	return &ResourceMetrics{
		TotalResources:      atomic.LoadInt64(&rlm.resourceMetrics.TotalResources),
		ActiveResources:     atomic.LoadInt64(&rlm.resourceMetrics.ActiveResources),
		AllocatedResources:  atomic.LoadInt64(&rlm.resourceMetrics.AllocatedResources),
		FailedAllocations:   atomic.LoadInt64(&rlm.resourceMetrics.FailedAllocations),
		SuccessfulCleanups:  atomic.LoadInt64(&rlm.resourceMetrics.SuccessfulCleanups),
		FailedCleanups:      atomic.LoadInt64(&rlm.resourceMetrics.FailedCleanups),
		AverageLifetime:     rlm.resourceMetrics.AverageLifetime,
		PeakMemoryUsage:     atomic.LoadInt64(&rlm.resourceMetrics.PeakMemoryUsage),
		PeakCPUUsage:        rlm.resourceMetrics.PeakCPUUsage,
		ServerMetrics:       rlm.resourceMetrics.ServerMetrics,
		ProjectMetrics:      rlm.resourceMetrics.ProjectMetrics,
		ClientMetrics:       rlm.resourceMetrics.ClientMetrics,
		ConfigMetrics:       rlm.resourceMetrics.ConfigMetrics,
		WorkspaceMetrics:    rlm.resourceMetrics.WorkspaceMetrics,
	}
}

// GetMcpClientPoolStats returns MCP client pool statistics
func (rlm *ResourceLifecycleManager) GetMcpClientPoolStats() map[string]interface{} {
	rlm.mcpClientPool.mu.RLock()
	defer rlm.mcpClientPool.mu.RUnlock()
	
	return map[string]interface{}{
		"max_clients":      rlm.mcpClientPool.maxClients,
		"current_clients":  atomic.LoadInt32(&rlm.mcpClientPool.currentClients),
		"available_clients": len(rlm.mcpClientPool.available),
		"in_use_clients":   len(rlm.mcpClientPool.inUse),
		"total_created":    atomic.LoadInt64(&rlm.mcpClientPool.totalCreated),
		"total_destroyed":  atomic.LoadInt64(&rlm.mcpClientPool.totalDestroyed),
		"peak_usage":       atomic.LoadInt32(&rlm.mcpClientPool.peakUsage),
	}
}

// Helper methods for internal implementation
func getDefaultResourceManagerConfig() *ResourceManagerConfig {
	return &ResourceManagerConfig{
		MaxServerInstances:      5,
		MaxProjectInstances:     10,
		MaxClientInstances:      15,
		MaxWorkspaceInstances:   8,
		ResourceCreationTimeout: 60 * time.Second,
		ResourceCleanupTimeout:  30 * time.Second,
		HealthCheckInterval:     30 * time.Second,
		ResourcePoolTimeout:     2 * time.Minute,
		McpClientPoolSize:       10,
		McpClientTimeout:        30 * time.Second,
		MockResponseQueueSize:   100,
		ErrorSimulationEnabled:  true,
		TempDirPrefix:          "e2e-test-",
		CleanupOnFailure:       true,
		PreserveOnDebug:        false,
		MaxConcurrentTests:     4,
		ResourceMetricsEnabled: true,
		PerformanceMonitoring:  true,
		MemoryThresholdMB:      1024,
		CPUThresholdPercent:    80.0,
	}
}

// EnvironmentRequirements specifies requirements for test environments
type EnvironmentRequirements struct {
	ProjectTypes    []framework.ProjectType
	Languages       []string
	ResourceLimits  *ResourceLimits
	IsolationLevel  string
	PreserveLogs    bool
}