package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"lsp-gateway/internal/project"
	"lsp-gateway/internal/setup"
	"lsp-gateway/internal/transport"
)

// Container is the dependency injection container for the shared core
type Container struct {
	// Component instances
	sharedCore    SharedCore
	configMgr     ConfigManager
	registry      LSPServerRegistry
	cache         SCIPCache
	projectMgr    ProjectManager
	
	// Component factories
	factories     map[string]ComponentFactory
	
	// Configuration
	config        *ContainerConfig
	
	// State management
	mu            sync.RWMutex
	initialized   bool
	logger        *setup.SetupLogger
}

// ComponentFactory creates instances of core components
type ComponentFactory interface {
	Create(container *Container) (interface{}, error)
	Dependencies() []string
	Name() string
}

// ContainerConfig holds configuration for the DI container
type ContainerConfig struct {
	// Component configurations
	ConfigManagerType  string                 `json:"config_manager_type"`  // "file", "memory", "hybrid"
	RegistryType       string                 `json:"registry_type"`        // "simple", "pooled", "distributed"
	CacheType          string                 `json:"cache_type"`           // "memory", "disk", "hybrid"
	ProjectManagerType string                 `json:"project_manager_type"` // "default", "enhanced"
	
	// Resource limits
	MaxMemoryUsage     int64                  `json:"max_memory_usage"`
	MaxWorkspaces      int                    `json:"max_workspaces"`
	MaxServers         int                    `json:"max_servers"`
	
	// Initialization settings
	InitTimeout        time.Duration          `json:"init_timeout"`
	ShutdownTimeout    time.Duration          `json:"shutdown_timeout"`
	
	// Component-specific configs
	ComponentConfigs   map[string]interface{} `json:"component_configs"`
}

// NewContainer creates a new dependency injection container
func NewContainer(config *ContainerConfig) *Container {
	if config == nil {
		config = defaultContainerConfig()
	}
	
	return &Container{
		factories: make(map[string]ComponentFactory),
		config:    config,
		logger:    setup.NewSetupLogger(nil),
	}
}

// Register registers a component factory
func (c *Container) Register(name string, factory ComponentFactory) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.initialized {
		return fmt.Errorf("container already initialized, cannot register new components")
	}
	
	if _, exists := c.factories[name]; exists {
		return fmt.Errorf("component %s already registered", name)
	}
	
	c.factories[name] = factory
	c.logger.WithField("component", name).Debug("Component factory registered")
	return nil
}

// Initialize initializes all components in dependency order
func (c *Container) Initialize(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.initialized {
		return fmt.Errorf("container already initialized")
	}
	
	// Create initialization context with timeout
	initCtx, cancel := context.WithTimeout(ctx, c.config.InitTimeout)
	defer cancel()
	
	c.logger.Info("Initializing shared core container")
	
	// Register default factories if not already registered
	if err := c.registerDefaultFactories(); err != nil {
		return fmt.Errorf("failed to register default factories: %w", err)
	}
	
	// Resolve dependency order
	initOrder, err := c.resolveDependencyOrder()
	if err != nil {
		return fmt.Errorf("failed to resolve dependency order: %w", err)
	}
	
	// Initialize components in order
	components := make(map[string]interface{})
	for _, name := range initOrder {
		select {
		case <-initCtx.Done():
			return fmt.Errorf("initialization timeout: %w", initCtx.Err())
		default:
		}
		
		factory := c.factories[name]
		component, err := factory.Create(c)
		if err != nil {
			return fmt.Errorf("failed to create component %s: %w", name, err)
		}
		
		components[name] = component
		c.logger.WithField("component", name).Debug("Component initialized")
	}
	
	// Assign components to container fields
	if err := c.assignComponents(components); err != nil {
		return fmt.Errorf("failed to assign components: %w", err)
	}
	
	// Initialize the shared core
	if err := c.sharedCore.Initialize(initCtx); err != nil {
		return fmt.Errorf("failed to initialize shared core: %w", err)
	}
	
	c.initialized = true
	c.logger.Info("Shared core container initialized successfully")
	return nil
}

// GetSharedCore returns the initialized shared core instance
func (c *Container) GetSharedCore() (SharedCore, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if !c.initialized {
		return nil, fmt.Errorf("container not initialized")
	}
	
	return c.sharedCore, nil
}

// GetComponent returns a specific component by name
func (c *Container) GetComponent(name string) (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if !c.initialized {
		return nil, fmt.Errorf("container not initialized")
	}
	
	switch name {
	case "config":
		return c.configMgr, nil
	case "registry":
		return c.registry, nil
	case "cache":
		return c.cache, nil
	case "projects":
		return c.projectMgr, nil
	case "core":
		return c.sharedCore, nil
	default:
		return nil, fmt.Errorf("unknown component: %s", name)
	}
}

// Shutdown gracefully shuts down all components
func (c *Container) Shutdown() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if !c.initialized {
		return nil
	}
	
	c.logger.Info("Shutting down shared core container")
	
	// Shutdown in reverse dependency order
	var errors []error
	
	if c.sharedCore != nil {
		if err := c.sharedCore.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop shared core: %w", err))
		}
	}
	
	// Additional cleanup for individual components if needed
	
	c.initialized = false
	
	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors: %v", errors)
	}
	
	c.logger.Info("Shared core container shut down successfully")
	return nil
}

// Component factory implementations

// ConfigManagerFactory creates ConfigManager instances
type ConfigManagerFactory struct {
	managerType string
}

func NewConfigManagerFactory(managerType string) *ConfigManagerFactory {
	return &ConfigManagerFactory{managerType: managerType}
}

func (f *ConfigManagerFactory) Create(container *Container) (interface{}, error) {
	switch f.managerType {
	case "file":
		return NewFileConfigManager(container.config.ComponentConfigs["config"])
	case "memory":
		return NewMemoryConfigManager(container.config.ComponentConfigs["config"])
	case "hybrid":
		return NewHybridConfigManager(container.config.ComponentConfigs["config"])
	default:
		return NewHybridConfigManager(container.config.ComponentConfigs["config"])
	}
}

func (f *ConfigManagerFactory) Dependencies() []string {
	return []string{} // No dependencies
}

func (f *ConfigManagerFactory) Name() string {
	return "config"
}

// LSPServerRegistryFactory creates LSPServerRegistry instances
type LSPServerRegistryFactory struct {
	registryType string
}

func NewLSPServerRegistryFactory(registryType string) *LSPServerRegistryFactory {
	return &LSPServerRegistryFactory{registryType: registryType}
}

func (f *LSPServerRegistryFactory) Create(container *Container) (interface{}, error) {
	configMgr := container.configMgr
	
	switch f.registryType {
	case "simple":
		return NewSimpleServerRegistry(configMgr, container.config.ComponentConfigs["registry"])
	case "pooled":
		return NewPooledServerRegistry(configMgr, container.config.ComponentConfigs["registry"])
	case "distributed":
		return NewDistributedServerRegistry(configMgr, container.config.ComponentConfigs["registry"])
	default:
		return NewPooledServerRegistry(configMgr, container.config.ComponentConfigs["registry"])
	}
}

func (f *LSPServerRegistryFactory) Dependencies() []string {
	return []string{"config"}
}

func (f *LSPServerRegistryFactory) Name() string {
	return "registry"
}

// SCIPCacheFactory creates SCIPCache instances
type SCIPCacheFactory struct {
	cacheType string
}

func NewSCIPCacheFactory(cacheType string) *SCIPCacheFactory {
	return &SCIPCacheFactory{cacheType: cacheType}
}

func (f *SCIPCacheFactory) Create(container *Container) (interface{}, error) {
	configMgr := container.configMgr
	
	switch f.cacheType {
	case "memory":
		return NewMemorySCIPCache(configMgr, container.config.ComponentConfigs["cache"])
	case "disk":
		return NewDiskSCIPCache(configMgr, container.config.ComponentConfigs["cache"])
	case "hybrid":
		return NewHybridSCIPCache(configMgr, container.config.ComponentConfigs["cache"])
	default:
		return NewHybridSCIPCache(configMgr, container.config.ComponentConfigs["cache"])
	}
}

func (f *SCIPCacheFactory) Dependencies() []string {
	return []string{"config"}
}

func (f *SCIPCacheFactory) Name() string {
	return "cache"
}

// ProjectManagerFactory creates ProjectManager instances
type ProjectManagerFactory struct {
	managerType string
}

func NewProjectManagerFactory(managerType string) *ProjectManagerFactory {
	return &ProjectManagerFactory{managerType: managerType}
}

func (f *ProjectManagerFactory) Create(container *Container) (interface{}, error) {
	configMgr := container.configMgr
	cache := container.cache
	
	switch f.managerType {
	case "default":
		return NewDefaultProjectManager(configMgr, cache, container.config.ComponentConfigs["projects"])
	case "enhanced":
		return NewEnhancedProjectManager(configMgr, cache, container.config.ComponentConfigs["projects"])
	default:
		return NewEnhancedProjectManager(configMgr, cache, container.config.ComponentConfigs["projects"])
	}
}

func (f *ProjectManagerFactory) Dependencies() []string {
	return []string{"config", "cache"}
}

func (f *ProjectManagerFactory) Name() string {
	return "projects"
}

// SharedCoreFactory creates SharedCore instances
type SharedCoreFactory struct{}

func NewSharedCoreFactory() *SharedCoreFactory {
	return &SharedCoreFactory{}
}

func (f *SharedCoreFactory) Create(container *Container) (interface{}, error) {
	return NewDefaultSharedCore(
		container.configMgr,
		container.registry,
		container.cache,
		container.projectMgr,
		container.config.ComponentConfigs["core"],
	)
}

func (f *SharedCoreFactory) Dependencies() []string {
	return []string{"config", "registry", "cache", "projects"}
}

func (f *SharedCoreFactory) Name() string {
	return "core"
}

// Helper methods

func (c *Container) registerDefaultFactories() error {
	// Only register if not already registered
	defaultFactories := map[string]ComponentFactory{
		"config":   NewConfigManagerFactory(c.config.ConfigManagerType),
		"registry": NewLSPServerRegistryFactory(c.config.RegistryType),
		"cache":    NewSCIPCacheFactory(c.config.CacheType),
		"projects": NewProjectManagerFactory(c.config.ProjectManagerType),
		"core":     NewSharedCoreFactory(),
	}
	
	for name, factory := range defaultFactories {
		if _, exists := c.factories[name]; !exists {
			c.factories[name] = factory
		}
	}
	
	return nil
}

func (c *Container) resolveDependencyOrder() ([]string, error) {
	// Topological sort of dependencies
	visited := make(map[string]bool)
	visiting := make(map[string]bool)
	var order []string
	
	var visit func(string) error
	visit = func(name string) error {
		if visiting[name] {
			return fmt.Errorf("circular dependency detected: %s", name)
		}
		if visited[name] {
			return nil
		}
		
		visiting[name] = true
		
		factory, exists := c.factories[name]
		if !exists {
			return fmt.Errorf("factory not found: %s", name)
		}
		
		for _, dep := range factory.Dependencies() {
			if err := visit(dep); err != nil {
				return err
			}
		}
		
		visiting[name] = false
		visited[name] = true
		order = append(order, name)
		
		return nil
	}
	
	for name := range c.factories {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	
	return order, nil
}

func (c *Container) assignComponents(components map[string]interface{}) error {
	var ok bool
	
	if c.configMgr, ok = components["config"].(ConfigManager); !ok {
		return fmt.Errorf("invalid config manager type")
	}
	
	if c.registry, ok = components["registry"].(LSPServerRegistry); !ok {
		return fmt.Errorf("invalid registry type")
	}
	
	if c.cache, ok = components["cache"].(SCIPCache); !ok {
		return fmt.Errorf("invalid cache type")
	}
	
	if c.projectMgr, ok = components["projects"].(ProjectManager); !ok {
		return fmt.Errorf("invalid project manager type")
	}
	
	if c.sharedCore, ok = components["core"].(SharedCore); !ok {
		return fmt.Errorf("invalid shared core type")
	}
	
	return nil
}

func defaultContainerConfig() *ContainerConfig {
	return &ContainerConfig{
		ConfigManagerType:  "hybrid",
		RegistryType:       "pooled",
		CacheType:          "hybrid",
		ProjectManagerType: "enhanced",
		
		MaxMemoryUsage:     1024 * 1024 * 1024, // 1GB
		MaxWorkspaces:      100,
		MaxServers:         50,
		
		InitTimeout:        30 * time.Second,
		ShutdownTimeout:    10 * time.Second,
		
		ComponentConfigs:   make(map[string]interface{}),
	}
}

// Factory registry for external registration
var GlobalFactoryRegistry = make(map[string]ComponentFactory)

// RegisterGlobalFactory registers a factory globally
func RegisterGlobalFactory(name string, factory ComponentFactory) {
	GlobalFactoryRegistry[name] = factory
}

// GetGlobalFactory retrieves a globally registered factory
func GetGlobalFactory(name string) (ComponentFactory, bool) {
	factory, exists := GlobalFactoryRegistry[name]
	return factory, exists
}

// Missing constructor function implementations

// NewFileConfigManager creates a file-based config manager (delegates to hybrid)
func NewFileConfigManager(configData interface{}) (ConfigManager, error) {
	return NewHybridConfigManager(configData)
}

// NewMemoryConfigManager creates a memory-based config manager (delegates to hybrid)
func NewMemoryConfigManager(configData interface{}) (ConfigManager, error) {
	return NewHybridConfigManager(configData)
}

// SimpleServerRegistry is a basic LSP server registry implementation
type SimpleServerRegistry struct {
	mu      sync.RWMutex
	servers map[string]map[string]transport.LSPClient
	config  ConfigManager
}

// NewSimpleServerRegistry creates a simple server registry
func NewSimpleServerRegistry(configMgr ConfigManager, configData interface{}) (LSPServerRegistry, error) {
	return &SimpleServerRegistry{
		servers: make(map[string]map[string]transport.LSPClient),
		config:  configMgr,
	}, nil
}

// GetServer implements LSPServerRegistry interface
func (r *SimpleServerRegistry) GetServer(serverName string, workspaceID string) (transport.LSPClient, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	if workspace, exists := r.servers[workspaceID]; exists {
		if server, exists := workspace[serverName]; exists {
			return server, nil
		}
	}
	return nil, fmt.Errorf("server %s not found for workspace %s", serverName, workspaceID)
}

// CreateServer implements LSPServerRegistry interface
func (r *SimpleServerRegistry) CreateServer(serverName string, workspaceID string, config *ServerConfig) (transport.LSPClient, error) {
	// Basic implementation - would create actual transport client
	return nil, fmt.Errorf("not implemented")
}

// RemoveServer implements LSPServerRegistry interface
func (r *SimpleServerRegistry) RemoveServer(serverName string, workspaceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if workspace, exists := r.servers[workspaceID]; exists {
		delete(workspace, serverName)
		if len(workspace) == 0 {
			delete(r.servers, workspaceID)
		}
	}
	return nil
}

// GetAllServers implements LSPServerRegistry interface
func (r *SimpleServerRegistry) GetAllServers() map[string]map[string]transport.LSPClient {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	result := make(map[string]map[string]transport.LSPClient)
	for wsID, servers := range r.servers {
		result[wsID] = make(map[string]transport.LSPClient)
		for serverName, client := range servers {
			result[wsID][serverName] = client
		}
	}
	return result
}

// GetWorkspaceServers implements LSPServerRegistry interface
func (r *SimpleServerRegistry) GetWorkspaceServers(workspaceID string) map[string]transport.LSPClient {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	if workspace, exists := r.servers[workspaceID]; exists {
		result := make(map[string]transport.LSPClient)
		for name, client := range workspace {
			result[name] = client
		}
		return result
	}
	return make(map[string]transport.LSPClient)
}

// RemoveWorkspace implements LSPServerRegistry interface
func (r *SimpleServerRegistry) RemoveWorkspace(workspaceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	delete(r.servers, workspaceID)
	return nil
}

// StartServer implements LSPServerRegistry interface
func (r *SimpleServerRegistry) StartServer(serverName string, workspaceID string) error {
	return fmt.Errorf("not implemented")
}

// StopServer implements LSPServerRegistry interface
func (r *SimpleServerRegistry) StopServer(serverName string, workspaceID string) error {
	return fmt.Errorf("not implemented")
}

// RestartServer implements LSPServerRegistry interface
func (r *SimpleServerRegistry) RestartServer(serverName string, workspaceID string) error {
	return fmt.Errorf("not implemented")
}

// IsServerHealthy implements LSPServerRegistry interface
func (r *SimpleServerRegistry) IsServerHealthy(serverName string, workspaceID string) bool {
	return false
}

// GetServerMetrics implements LSPServerRegistry interface
func (r *SimpleServerRegistry) GetServerMetrics(serverName string, workspaceID string) *ServerMetrics {
	return &ServerMetrics{}
}

// SetMaxConnections implements LSPServerRegistry interface
func (r *SimpleServerRegistry) SetMaxConnections(serverName string, maxConns int) {
	// No-op for simple registry
}

// GetConnectionPool implements LSPServerRegistry interface
func (r *SimpleServerRegistry) GetConnectionPool(serverName string) ConnectionPool {
	return nil
}

// NewPooledServerRegistry creates a pooled server registry (delegates to simple for now)
func NewPooledServerRegistry(configMgr ConfigManager, configData interface{}) (LSPServerRegistry, error) {
	return NewSimpleServerRegistry(configMgr, configData)
}

// NewDistributedServerRegistry creates a distributed server registry (delegates to simple for now)
func NewDistributedServerRegistry(configMgr ConfigManager, configData interface{}) (LSPServerRegistry, error) {
	return NewSimpleServerRegistry(configMgr, configData)
}

// Basic SCIP cache implementations

// MemorySCIPCache is a memory-only SCIP cache
type MemorySCIPCache struct {
	mu    sync.RWMutex
	data  map[string]interface{}
}

// NewMemorySCIPCache creates a memory-based SCIP cache
func NewMemorySCIPCache(configMgr ConfigManager, configData interface{}) (SCIPCache, error) {
	return &MemorySCIPCache{
		data: make(map[string]interface{}),
	}, nil
}

// GetSymbols implements SCIPCache interface
func (c *MemorySCIPCache) GetSymbols(workspaceID string, fileURI string) ([]Symbol, bool) {
	return nil, false
}

// SetSymbols implements SCIPCache interface
func (c *MemorySCIPCache) SetSymbols(workspaceID string, fileURI string, symbols []Symbol) error {
	return nil
}

// InvalidateSymbols implements SCIPCache interface
func (c *MemorySCIPCache) InvalidateSymbols(workspaceID string, fileURI string) error {
	return nil
}

// GetReferences implements SCIPCache interface
func (c *MemorySCIPCache) GetReferences(workspaceID string, symbolID string) ([]Reference, bool) {
	return nil, false
}

// SetReferences implements SCIPCache interface
func (c *MemorySCIPCache) SetReferences(workspaceID string, symbolID string, refs []Reference) error {
	return nil
}

// InvalidateReferences implements SCIPCache interface
func (c *MemorySCIPCache) InvalidateReferences(workspaceID string, symbolID string) error {
	return nil
}

// GetDefinition implements SCIPCache interface
func (c *MemorySCIPCache) GetDefinition(workspaceID string, symbolID string) (*Definition, bool) {
	return nil, false
}

// SetDefinition implements SCIPCache interface
func (c *MemorySCIPCache) SetDefinition(workspaceID string, symbolID string, def *Definition) error {
	return nil
}

// InvalidateDefinition implements SCIPCache interface
func (c *MemorySCIPCache) InvalidateDefinition(workspaceID string, symbolID string) error {
	return nil
}

// InvalidateWorkspace implements SCIPCache interface
func (c *MemorySCIPCache) InvalidateWorkspace(workspaceID string) error {
	return nil
}

// InvalidateAll implements SCIPCache interface
func (c *MemorySCIPCache) InvalidateAll() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]interface{})
	return nil
}

// GetCacheStats implements SCIPCache interface
func (c *MemorySCIPCache) GetCacheStats() *CacheStats {
	return &CacheStats{}
}

// SetMemoryLimit implements SCIPCache interface
func (c *MemorySCIPCache) SetMemoryLimit(limitBytes int64) {
	// No-op for basic implementation
}

// GetMemoryUsage implements SCIPCache interface
func (c *MemorySCIPCache) GetMemoryUsage() int64 {
	return 0
}

// Compact implements SCIPCache interface
func (c *MemorySCIPCache) Compact() error {
	return nil
}

// NewDiskSCIPCache creates a disk-based SCIP cache (delegates to memory for now)
func NewDiskSCIPCache(configMgr ConfigManager, configData interface{}) (SCIPCache, error) {
	return NewMemorySCIPCache(configMgr, configData)
}

// NewHybridSCIPCache creates a hybrid SCIP cache (delegates to memory for now)
func NewHybridSCIPCache(configMgr ConfigManager, configData interface{}) (SCIPCache, error) {
	return NewMemorySCIPCache(configMgr, configData)
}

// Basic Project Manager implementations

// DefaultProjectManager is a basic project manager
type DefaultProjectManager struct {
	config ConfigManager
	cache  SCIPCache
}

// NewDefaultProjectManager creates a default project manager
func NewDefaultProjectManager(configMgr ConfigManager, cache SCIPCache, configData interface{}) (ProjectManager, error) {
	return &DefaultProjectManager{
		config: configMgr,
		cache:  cache,
	}, nil
}

// DetectProject implements ProjectManager interface
func (pm *DefaultProjectManager) DetectProject(ctx context.Context, path string) (*project.ProjectContext, error) {
	// Basic implementation - would use actual project detection
	return nil, fmt.Errorf("not implemented")
}

// DetectMultipleProjects implements ProjectManager interface
func (pm *DefaultProjectManager) DetectMultipleProjects(ctx context.Context, paths []string) (map[string]*project.ProjectContext, error) {
	return make(map[string]*project.ProjectContext), nil
}

// ScanWorkspace implements ProjectManager interface
func (pm *DefaultProjectManager) ScanWorkspace(ctx context.Context, workspaceRoot string) ([]*project.ProjectContext, error) {
	return []*project.ProjectContext{}, nil
}

// CreateWorkspace implements ProjectManager interface
func (pm *DefaultProjectManager) CreateWorkspace(workspaceID string, rootPath string) (*WorkspaceContext, error) {
	return &WorkspaceContext{
		ID:       workspaceID,
		RootPath: rootPath,
		Servers:  make(map[string]transport.LSPClient),
	}, nil
}

// GetWorkspace implements ProjectManager interface
func (pm *DefaultProjectManager) GetWorkspace(workspaceID string) (*WorkspaceContext, bool) {
	return nil, false
}

// RemoveWorkspace implements ProjectManager interface
func (pm *DefaultProjectManager) RemoveWorkspace(workspaceID string) error {
	return nil
}

// GetAllWorkspaces implements ProjectManager interface
func (pm *DefaultProjectManager) GetAllWorkspaces() map[string]*WorkspaceContext {
	return make(map[string]*WorkspaceContext)
}

// UpdateWorkspace implements ProjectManager interface
func (pm *DefaultProjectManager) UpdateWorkspace(workspaceID string, updates *WorkspaceUpdate) error {
	return nil
}

// ValidateWorkspace implements ProjectManager interface
func (pm *DefaultProjectManager) ValidateWorkspace(ctx context.Context, workspaceID string) error {
	return nil
}

// RefreshWorkspace implements ProjectManager interface
func (pm *DefaultProjectManager) RefreshWorkspace(ctx context.Context, workspaceID string) error {
	return nil
}

// WatchWorkspace implements ProjectManager interface
func (pm *DefaultProjectManager) WatchWorkspace(ctx context.Context, workspaceID string) (<-chan *WorkspaceEvent, error) {
	ch := make(chan *WorkspaceEvent)
	close(ch)
	return ch, nil
}

// StopWatching implements ProjectManager interface
func (pm *DefaultProjectManager) StopWatching(workspaceID string) error {
	return nil
}

// NewEnhancedProjectManager creates an enhanced project manager (delegates to default for now)
func NewEnhancedProjectManager(configMgr ConfigManager, cache SCIPCache, configData interface{}) (ProjectManager, error) {
	return NewDefaultProjectManager(configMgr, cache, configData)
}