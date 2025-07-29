package core

import (
	"context"
	"time"

	"lsp-gateway/internal/project"
	"lsp-gateway/internal/transport"
)

// SharedCore defines the main interface for the shared core module
type SharedCore interface {
	// Core lifecycle management
	Initialize(ctx context.Context) error
	Start(ctx context.Context) error
	Stop() error
	
	// Component access
	Config() ConfigManager
	Registry() LSPServerRegistry
	Cache() SCIPCache
	Projects() ProjectManager
	
	// Health and metrics
	Health() HealthStatus
	Metrics() Metrics
}

// ConfigManager handles hierarchical configuration management
type ConfigManager interface {
	// Configuration retrieval with inheritance: client -> project -> global
	GetServerConfig(serverName string, workspaceID string) (*ServerConfig, error)
	GetGlobalConfig() (*GlobalConfig, error)
	GetProjectConfig(workspaceID string) (*ProjectConfig, error)
	GetClientConfig(clientID string) (*ClientConfig, error)
	
	// Configuration updates
	UpdateGlobalConfig(config *GlobalConfig) error
	UpdateProjectConfig(workspaceID string, config *ProjectConfig) error
	UpdateClientConfig(clientID string, config *ClientConfig) error
	
	// Configuration validation and merging
	ValidateConfig(config interface{}) error
	MergeConfigs(global *GlobalConfig, project *ProjectConfig, client *ClientConfig) *ResolvedConfig
	
	// Configuration watching
	WatchGlobalConfig(ctx context.Context) (<-chan *GlobalConfig, error)
	WatchProjectConfig(ctx context.Context, workspaceID string) (<-chan *ProjectConfig, error)
}

// LSPServerRegistry manages LSP server instances and their lifecycle
type LSPServerRegistry interface {
	// Server instance management
	GetServer(serverName string, workspaceID string) (transport.LSPClient, error)
	CreateServer(serverName string, workspaceID string, config *ServerConfig) (transport.LSPClient, error)
	RemoveServer(serverName string, workspaceID string) error
	
	// Bulk operations
	GetAllServers() map[string]map[string]transport.LSPClient
	GetWorkspaceServers(workspaceID string) map[string]transport.LSPClient
	RemoveWorkspace(workspaceID string) error
	
	// Server lifecycle
	StartServer(serverName string, workspaceID string) error
	StopServer(serverName string, workspaceID string) error
	RestartServer(serverName string, workspaceID string) error
	
	// Server health and monitoring
	IsServerHealthy(serverName string, workspaceID string) bool
	GetServerMetrics(serverName string, workspaceID string) *ServerMetrics
	
	// Connection pooling and resource management
	SetMaxConnections(serverName string, maxConns int)
	GetConnectionPool(serverName string) ConnectionPool
}

// SCIPCache provides shared SCIP caching functionality
type SCIPCache interface {
	// Symbol caching
	GetSymbols(workspaceID string, fileURI string) ([]Symbol, bool)
	SetSymbols(workspaceID string, fileURI string, symbols []Symbol) error
	InvalidateSymbols(workspaceID string, fileURI string) error
	
	// Reference caching
	GetReferences(workspaceID string, symbolID string) ([]Reference, bool)
	SetReferences(workspaceID string, symbolID string, refs []Reference) error
	InvalidateReferences(workspaceID string, symbolID string) error
	
	// Definition caching
	GetDefinition(workspaceID string, symbolID string) (*Definition, bool)
	SetDefinition(workspaceID string, symbolID string, def *Definition) error
	InvalidateDefinition(workspaceID string, symbolID string) error
	
	// Cache management
	InvalidateWorkspace(workspaceID string) error
	InvalidateAll() error
	GetCacheStats() *CacheStats
	
	// Memory management
	SetMemoryLimit(limitBytes int64)
	GetMemoryUsage() int64
	Compact() error
}

// ProjectManager handles project detection and workspace management
type ProjectManager interface {
	// Project detection
	DetectProject(ctx context.Context, path string) (*project.ProjectContext, error)
	DetectMultipleProjects(ctx context.Context, paths []string) (map[string]*project.ProjectContext, error)
	ScanWorkspace(ctx context.Context, workspaceRoot string) ([]*project.ProjectContext, error)
	
	// Workspace management
	CreateWorkspace(workspaceID string, rootPath string) (*WorkspaceContext, error)
	GetWorkspace(workspaceID string) (*WorkspaceContext, bool)
	RemoveWorkspace(workspaceID string) error
	GetAllWorkspaces() map[string]*WorkspaceContext
	
	// Workspace operations
	UpdateWorkspace(workspaceID string, updates *WorkspaceUpdate) error
	ValidateWorkspace(ctx context.Context, workspaceID string) error
	RefreshWorkspace(ctx context.Context, workspaceID string) error
	
	// File system watching
	WatchWorkspace(ctx context.Context, workspaceID string) (<-chan *WorkspaceEvent, error)
	StopWatching(workspaceID string) error
}

// ConnectionPool manages connection pooling for LSP servers
type ConnectionPool interface {
	Get() (transport.LSPClient, error)
	Put(client transport.LSPClient) error
	Close() error
	Size() int
	Available() int
}

// Configuration types with hierarchical inheritance

// GlobalConfig contains system-wide configuration
type GlobalConfig struct {
	// Server defaults
	DefaultTimeout    time.Duration            `json:"default_timeout"`
	MaxConnections    int                      `json:"max_connections"`
	ServerDefaults    map[string]*ServerConfig `json:"server_defaults"`
	
	// Cache settings
	CacheConfig       *CacheConfig             `json:"cache_config"`
	
	// Resource limits
	MemoryLimit       int64                    `json:"memory_limit"`
	MaxWorkspaces     int                      `json:"max_workspaces"`
	
	// Logging and monitoring
	LogLevel          string                   `json:"log_level"`
	MetricsEnabled    bool                     `json:"metrics_enabled"`
	
	// Feature flags
	Features          map[string]bool          `json:"features"`
	
	Version           string                   `json:"version"`
	UpdatedAt         time.Time                `json:"updated_at"`
}

// ProjectConfig contains project-specific configuration
type ProjectConfig struct {
	WorkspaceID       string                   `json:"workspace_id"`
	ProjectType       string                   `json:"project_type"`
	RootPath          string                   `json:"root_path"`
	
	// Server overrides
	ServerOverrides   map[string]*ServerConfig `json:"server_overrides"`
	
	// Project-specific settings
	SourceDirs        []string                 `json:"source_dirs"`
	ExcludeDirs       []string                 `json:"exclude_dirs"`
	BuildCommand      string                   `json:"build_command"`
	TestCommand       string                   `json:"test_command"`
	
	// Language-specific settings
	LanguageSettings  map[string]interface{}   `json:"language_settings"`
	
	Version           string                   `json:"version"`
	UpdatedAt         time.Time                `json:"updated_at"`
}

// ClientConfig contains client-specific configuration
type ClientConfig struct {
	ClientID          string                   `json:"client_id"`
	ClientType        string                   `json:"client_type"` // "http", "mcp"
	
	// Client preferences
	PreferredServers  []string                 `json:"preferred_servers"`
	DisabledServers   []string                 `json:"disabled_servers"`
	
	// Performance settings
	MaxRequestTime    time.Duration            `json:"max_request_time"`
	MaxConcurrency    int                      `json:"max_concurrency"`
	
	// Feature preferences
	EnabledFeatures   []string                 `json:"enabled_features"`
	DisabledFeatures  []string                 `json:"disabled_features"`
	
	Version           string                   `json:"version"`
	UpdatedAt         time.Time                `json:"updated_at"`
}

// ServerConfig contains LSP server configuration
type ServerConfig struct {
	Name              string                   `json:"name"`
	Command           []string                 `json:"command"`
	Args              []string                 `json:"args"`
	Env               map[string]string        `json:"env"`
	
	// Connection settings
	Transport         string                   `json:"transport"` // "stdio", "tcp"
	Port              int                      `json:"port,omitempty"`
	Host              string                   `json:"host,omitempty"`
	
	// Timeouts and limits
	StartupTimeout    time.Duration            `json:"startup_timeout"`
	RequestTimeout    time.Duration            `json:"request_timeout"`
	ShutdownTimeout   time.Duration            `json:"shutdown_timeout"`
	MaxRetries        int                      `json:"max_retries"`
	
	// Health checking
	HealthCheckInterval time.Duration          `json:"health_check_interval"`
	HealthCheckTimeout  time.Duration          `json:"health_check_timeout"`
	
	// Language support
	LanguageIDs       []string                 `json:"language_ids"`
	FileExtensions    []string                 `json:"file_extensions"`
	
	// Capabilities
	InitializationOptions interface{}          `json:"initialization_options,omitempty"`
	
	Version           string                   `json:"version"`
	UpdatedAt         time.Time                `json:"updated_at"`
}

// ResolvedConfig is the final merged configuration
type ResolvedConfig struct {
	ServerConfigs     map[string]*ServerConfig `json:"server_configs"`
	CacheConfig       *CacheConfig             `json:"cache_config"`
	ClientSettings    *ClientConfig            `json:"client_settings"`
	ProjectSettings   *ProjectConfig           `json:"project_settings"`
	GlobalSettings    *GlobalConfig            `json:"global_settings"`
	
	ResolvedAt        time.Time                `json:"resolved_at"`
}

// CacheConfig contains SCIP cache configuration
type CacheConfig struct {
	// Memory cache settings
	MemoryCacheSize   int64                    `json:"memory_cache_size"`
	MemoryTTL         time.Duration            `json:"memory_ttl"`
	
	// Disk cache settings
	DiskCacheEnabled  bool                     `json:"disk_cache_enabled"`
	DiskCacheSize     int64                    `json:"disk_cache_size"`
	DiskCachePath     string                   `json:"disk_cache_path"`
	DiskTTL           time.Duration            `json:"disk_ttl"`
	
	// Cache behavior
	WriteThrough      bool                     `json:"write_through"`
	CompactionInterval time.Duration           `json:"compaction_interval"`
	MaxEntries        int                      `json:"max_entries"`
}

// Data types for SCIP operations

type Symbol struct {
	ID                string                   `json:"id"`
	Name              string                   `json:"name"`
	Kind              string                   `json:"kind"`
	Location          *Location                `json:"location"`
	Documentation     string                   `json:"documentation,omitempty"`
	Detail            string                   `json:"detail,omitempty"`
	Deprecated        bool                     `json:"deprecated,omitempty"`
	
	CachedAt          time.Time                `json:"cached_at"`
}

type Reference struct {
	Location          *Location                `json:"location"`
	Context           *ReferenceContext        `json:"context,omitempty"`
	
	CachedAt          time.Time                `json:"cached_at"`
}

type Definition struct {
	Location          *Location                `json:"location"`
	TargetRange       *Range                   `json:"target_range,omitempty"`
	
	CachedAt          time.Time                `json:"cached_at"`
}

type Location struct {
	URI               string                   `json:"uri"`
	Range             *Range                   `json:"range"`
}

type Range struct {
	Start             *Position                `json:"start"`
	End               *Position                `json:"end"`
}

type Position struct {
	Line              int                      `json:"line"`
	Character         int                      `json:"character"`
}

type ReferenceContext struct {
	IncludeDeclaration bool                    `json:"include_declaration"`
}

// Workspace management types

type WorkspaceContext struct {
	ID                string                   `json:"id"`
	RootPath          string                   `json:"root_path"`
	ProjectContext    *project.ProjectContext  `json:"project_context"`
	
	// Active servers
	Servers           map[string]transport.LSPClient `json:"-"`
	
	// Workspace state
	IsInitialized     bool                     `json:"is_initialized"`
	CreatedAt         time.Time                `json:"created_at"`
	LastAccessedAt    time.Time                `json:"last_accessed_at"`
	
	// File watching
	IsWatching        bool                     `json:"is_watching"`
	WatchedPaths      []string                 `json:"watched_paths"`
}

type WorkspaceUpdate struct {
	RootPath          *string                  `json:"root_path,omitempty"`
	ProjectContext    *project.ProjectContext  `json:"project_context,omitempty"`
	WatchedPaths      []string                 `json:"watched_paths,omitempty"`
}

type WorkspaceEvent struct {
	Type              string                   `json:"type"` // "created", "modified", "deleted"
	Path              string                   `json:"path"`
	WorkspaceID       string                   `json:"workspace_id"`
	Timestamp         time.Time                `json:"timestamp"`
}

// Monitoring and health types

type HealthStatus struct {
	Overall           string                   `json:"overall"` // "healthy", "degraded", "unhealthy"
	Components        map[string]ComponentHealth `json:"components"`
	Timestamp         time.Time                `json:"timestamp"`
}

type ComponentHealth struct {
	Status            string                   `json:"status"`
	Message           string                   `json:"message,omitempty"`
	LastCheck         time.Time                `json:"last_check"`
}

type Metrics struct {
	// Server metrics
	ServerCount       int                      `json:"server_count"`
	ActiveServers     int                      `json:"active_servers"`
	FailedServers     int                      `json:"failed_servers"`
	
	// Cache metrics
	CacheHitRate      float64                  `json:"cache_hit_rate"`
	CacheSize         int64                    `json:"cache_size"`
	CacheEvictions    int64                    `json:"cache_evictions"`
	
	// Workspace metrics
	WorkspaceCount    int                      `json:"workspace_count"`
	ActiveWorkspaces  int                      `json:"active_workspaces"`
	
	// Performance metrics
	AvgResponseTime   time.Duration            `json:"avg_response_time"`
	RequestCount      int64                    `json:"request_count"`
	ErrorCount        int64                    `json:"error_count"`
	
	Timestamp         time.Time                `json:"timestamp"`
}

type ServerMetrics struct {
	RequestCount      int64                    `json:"request_count"`
	ErrorCount        int64                    `json:"error_count"`
	AvgResponseTime   time.Duration            `json:"avg_response_time"`
	IsHealthy         bool                     `json:"is_healthy"`
	LastHealthCheck   time.Time                `json:"last_health_check"`
	Uptime            time.Duration            `json:"uptime"`
}

type CacheStats struct {
	// Memory cache stats
	MemoryHits        int64                    `json:"memory_hits"`
	MemoryMisses      int64                    `json:"memory_misses"`
	MemorySize        int64                    `json:"memory_size"`
	MemoryEntries     int                      `json:"memory_entries"`
	
	// Disk cache stats
	DiskHits          int64                    `json:"disk_hits"`
	DiskMisses        int64                    `json:"disk_misses"`
	DiskSize          int64                    `json:"disk_size"`
	DiskEntries       int                      `json:"disk_entries"`
	
	// Overall stats
	HitRate           float64                  `json:"hit_rate"`
	Evictions         int64                    `json:"evictions"`
	LastCompaction    time.Time                `json:"last_compaction"`
}