//go:build ignore

// Design for Multi-Project Context Management System
// This file contains interface definitions and architectural components

package design

import (
	"context"
	"time"
)

// ProjectID represents a unique identifier for a project context
type ProjectID string

// ProjectState represents the lifecycle state of a project context
type ProjectState int

const (
	ProjectStateUninitialized ProjectState = iota
	ProjectStateInitializing
	ProjectStateActive
	ProjectStateIdle
	ProjectStateSuspended
	ProjectStateFailed
	ProjectStateShuttingDown
)

func (ps ProjectState) String() string {
	switch ps {
	case ProjectStateUninitialized:
		return "uninitialized"
	case ProjectStateInitializing:
		return "initializing"
	case ProjectStateActive:
		return "active"
	case ProjectStateIdle:
		return "idle"
	case ProjectStateSuspended:
		return "suspended"
	case ProjectStateFailed:
		return "failed"
	case ProjectStateShuttingDown:
		return "shutting_down"
	default:
		return "unknown"
	}
}

// MultiProjectWorkspaceContext manages multiple project contexts within a single MCP server
type MultiProjectWorkspaceContext interface {
	// Project Registration and Discovery
	RegisterProject(ctx context.Context, projectRoot string, options *ProjectRegistrationOptions) (ProjectID, error)
	UnregisterProject(ctx context.Context, projectID ProjectID) error
	DiscoverProjects(ctx context.Context, workspaceRoot string, options *ProjectDiscoveryOptions) ([]ProjectRegistrationCandidate, error)
	
	// Project Lifecycle Management
	InitializeProject(ctx context.Context, projectID ProjectID) error
	ActivateProject(ctx context.Context, projectID ProjectID) error
	SuspendProject(ctx context.Context, projectID ProjectID) error
	ShutdownProject(ctx context.Context, projectID ProjectID) error
	
	// Context Switching
	SwitchToProject(ctx context.Context, projectID ProjectID) error
	GetActiveProject() ProjectID
	GetProjectContext(projectID ProjectID) (ProjectWorkspaceContext, error)
	
	// Project Querying
	ListProjects() []ProjectInfo
	FindProjectByPath(path string) (ProjectID, bool)
	FindProjectByURI(uri string) (ProjectID, bool)
	GetProjectState(projectID ProjectID) (ProjectState, error)
	
	// Resource Management
	GetMemoryUsage() *MultiProjectMemoryStats
	CleanupIdleProjects(ctx context.Context, idleThreshold time.Duration) error
	GetResourceLimits() *MultiProjectResourceLimits
	
	// Health and Monitoring
	GetHealthStatus() *MultiProjectHealthStatus
	GetMetrics() *MultiProjectMetrics
}

// ProjectWorkspaceContext represents a single project's workspace context
type ProjectWorkspaceContext interface {
	// Project Information
	GetProjectID() ProjectID
	GetProjectRoot() string
	GetProjectType() string
	GetLanguages() []string
	GetProjectMetadata() map[string]interface{}
	
	// State Management
	GetState() ProjectState
	IsActive() bool
	IsReady() bool
	GetLastActivity() time.Time
	
	// LSP Integration
	GetLSPClients() map[string]LSPClientInfo
	GetSCIPStore() SCIPStoreInterface
	GetCacheManager() CacheManagerInterface
	
	// File Context
	IsFileInProject(uri string) bool
	GetLanguageForFile(uri string) string
	GetRelativePath(uri string) string
	
	// Tool Context Enhancement
	EnhanceToolArgs(toolName string, args map[string]interface{}) map[string]interface{}
	
	// Resource Management
	GetMemoryUsage() *ProjectMemoryStats
	Cleanup(ctx context.Context) error
	Suspend(ctx context.Context) error
	Resume(ctx context.Context) error
}

// ProjectContextManager orchestrates multiple project contexts
type ProjectContextManager interface {
	// Core Management
	Initialize(ctx context.Context, config *MultiProjectConfig) error
	Shutdown(ctx context.Context) error
	
	// Project Management
	CreateProject(ctx context.Context, spec *ProjectSpec) (ProjectWorkspaceContext, error)
	RemoveProject(ctx context.Context, projectID ProjectID) error
	GetProject(projectID ProjectID) (ProjectWorkspaceContext, error)
	
	// Context Switching
	SwitchContext(ctx context.Context, projectID ProjectID) error
	GetCurrentContext() ProjectWorkspaceContext
	
	// Resource Management
	OptimizeMemory(ctx context.Context) error
	GetGlobalStats() *GlobalProjectStats
	
	// Event System
	RegisterEventHandler(handler ProjectEventHandler)
	UnregisterEventHandler(handler ProjectEventHandler)
}

// Supporting Types and Configurations

type ProjectRegistrationOptions struct {
	AutoInitialize   bool                   `json:"auto_initialize"`
	EnableCaching    bool                   `json:"enable_caching"`
	CacheConfig      *ProjectCacheConfig    `json:"cache_config"`
	ResourceLimits   *ProjectResourceLimits `json:"resource_limits"`
	CustomDetectors  map[string]interface{} `json:"custom_detectors"`
	InitTimeout      time.Duration          `json:"init_timeout"`
	HealthCheckURL   string                 `json:"health_check_url,omitempty"`
}

type ProjectDiscoveryOptions struct {
	MaxDepth         int                    `json:"max_depth"`
	IgnorePatterns   []string              `json:"ignore_patterns"`
	IncludePatterns  []string              `json:"include_patterns"`
	ParallelScan     bool                  `json:"parallel_scan"`
	DetectionTimeout time.Duration         `json:"detection_timeout"`
	LanguageFilters  []string              `json:"language_filters,omitempty"`
}

type ProjectRegistrationCandidate struct {
	Path            string                 `json:"path"`
	ProjectType     string                 `json:"project_type"`
	Languages       []string               `json:"languages"`
	Confidence      float64                `json:"confidence"`
	EstimatedSize   int64                  `json:"estimated_size"`
	RequiredServers []string               `json:"required_servers"`
	Metadata        map[string]interface{} `json:"metadata"`
}

type ProjectInfo struct {
	ID              ProjectID              `json:"id"`
	Root            string                 `json:"root"`
	Name            string                 `json:"name"`
	Type            string                 `json:"type"`
	State           ProjectState           `json:"state"`
	Languages       []string               `json:"languages"`
	LastActivity    time.Time              `json:"last_activity"`
	MemoryUsage     int64                  `json:"memory_usage"`
	IsActive        bool                   `json:"is_active"`
	RequiredServers []string               `json:"required_servers"`
	Metadata        map[string]interface{} `json:"metadata"`
}

type LSPClientInfo struct {
	Language    string                 `json:"language"`
	ServerName  string                 `json:"server_name"`
	IsActive    bool                   `json:"is_active"`
	LastUsed    time.Time              `json:"last_used"`
	RequestCount int64                 `json:"request_count"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// Memory and Resource Management

type ProjectMemoryStats struct {
	ProjectID          ProjectID `json:"project_id"`
	TotalMemory        int64     `json:"total_memory"`
	CacheMemory        int64     `json:"cache_memory"`
	LSPClientMemory    int64     `json:"lsp_client_memory"`
	MetadataMemory     int64     `json:"metadata_memory"`
	EstimatedOverhead  int64     `json:"estimated_overhead"`
	LastCalculated     time.Time `json:"last_calculated"`
}

type MultiProjectMemoryStats struct {
	TotalProjects         int                    `json:"total_projects"`
	ActiveProjects        int                    `json:"active_projects"`
	TotalMemoryUsage      int64                  `json:"total_memory_usage"`
	AveragePerProject     int64                  `json:"average_per_project"`
	ProjectStats          []ProjectMemoryStats   `json:"project_stats"`
	GlobalCacheMemory     int64                  `json:"global_cache_memory"`
	SharedResourceMemory  int64                  `json:"shared_resource_memory"`
	MemoryPressureLevel   string                 `json:"memory_pressure_level"`
}

type ProjectResourceLimits struct {
	MaxMemoryMB        int64         `json:"max_memory_mb"`
	MaxCacheEntries    int           `json:"max_cache_entries"`
	MaxConcurrentLSP   int           `json:"max_concurrent_lsp"`
	IdleTimeout        time.Duration `json:"idle_timeout"`
	SuspendTimeout     time.Duration `json:"suspend_timeout"`
	EnableAutoSuspend  bool          `json:"enable_auto_suspend"`
}

type MultiProjectResourceLimits struct {
	MaxTotalProjects     int                    `json:"max_total_projects"`
	MaxActiveProjects    int                    `json:"max_active_projects"`
	MaxTotalMemoryMB     int64                  `json:"max_total_memory_mb"`
	PerProjectLimits     ProjectResourceLimits  `json:"per_project_limits"`
	GlobalCacheMaxMB     int64                  `json:"global_cache_max_mb"`
	MemoryPressureThresholds map[string]float64 `json:"memory_pressure_thresholds"`
}

// Health and Monitoring

type MultiProjectHealthStatus struct {
	OverallHealth       string                           `json:"overall_health"`
	TotalProjects       int                              `json:"total_projects"`
	HealthyProjects     int                              `json:"healthy_projects"`
	UnhealthyProjects   int                              `json:"unhealthy_projects"`
	ProjectHealthMap    map[ProjectID]ProjectHealthInfo  `json:"project_health_map"`
	SystemMetrics       SystemHealthMetrics              `json:"system_metrics"`
	LastHealthCheck     time.Time                        `json:"last_health_check"`
	Issues              []HealthIssue                    `json:"issues,omitempty"`
}

type ProjectHealthInfo struct {
	ProjectID          ProjectID                `json:"project_id"`
	Health             string                   `json:"health"`
	State              ProjectState             `json:"state"`
	LSPServersHealth   map[string]string        `json:"lsp_servers_health"`
	CacheHealth        string                   `json:"cache_health"`
	LastActivity       time.Time                `json:"last_activity"`
	ErrorCount         int                      `json:"error_count"`
	Warnings           []string                 `json:"warnings,omitempty"`
}

type SystemHealthMetrics struct {
	CPUUsage           float64   `json:"cpu_usage"`
	MemoryUsage        float64   `json:"memory_usage"`
	GoroutineCount     int       `json:"goroutine_count"`
	GCPauseTime        time.Duration `json:"gc_pause_time"`
	OpenFileDescriptors int      `json:"open_file_descriptors"`
}

type HealthIssue struct {
	Severity    string    `json:"severity"`
	Component   string    `json:"component"`
	ProjectID   ProjectID `json:"project_id,omitempty"`
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
	Suggestions []string  `json:"suggestions,omitempty"`
}

// Metrics and Performance

type MultiProjectMetrics struct {
	ProjectCount           int                              `json:"project_count"`
	ActiveProjectCount     int                              `json:"active_project_count"`
	TotalRequests          int64                            `json:"total_requests"`
	AverageResponseTime    time.Duration                    `json:"average_response_time"`
	ContextSwitchCount     int64                            `json:"context_switch_count"`
	AverageContextSwitchTime time.Duration                  `json:"average_context_switch_time"`
	CacheHitRate           float64                          `json:"cache_hit_rate"`
	ProjectMetrics         map[ProjectID]ProjectMetrics     `json:"project_metrics"`
	ResourceUtilization    ResourceUtilizationMetrics       `json:"resource_utilization"`
	ErrorRates             map[string]float64               `json:"error_rates"`
	PerformanceTrends      PerformanceTrendData             `json:"performance_trends"`
}

type ProjectMetrics struct {
	ProjectID               ProjectID     `json:"project_id"`
	RequestCount            int64         `json:"request_count"`
	AverageResponseTime     time.Duration `json:"average_response_time"`
	ErrorCount              int64         `json:"error_count"`
	CacheHitCount           int64         `json:"cache_hit_count"`
	CacheMissCount          int64         `json:"cache_miss_count"`
	LSPRequestCount         int64         `json:"lsp_request_count"`
	ContextSwitchesToThis   int64         `json:"context_switches_to_this"`
	TimeSpentActive         time.Duration `json:"time_spent_active"`
	LastActivityTime        time.Time     `json:"last_activity_time"`
}

type ResourceUtilizationMetrics struct {
	MemoryUtilization    float64 `json:"memory_utilization"`
	CPUUtilization       float64 `json:"cpu_utilization"`
	CacheUtilization     float64 `json:"cache_utilization"`
	LSPConnectionCount   int     `json:"lsp_connection_count"`
	ActiveProjectRatio   float64 `json:"active_project_ratio"`
}

type PerformanceTrendData struct {
	ResponseTimeTrend      []float64 `json:"response_time_trend"`
	MemoryUsageTrend       []int64   `json:"memory_usage_trend"`
	ContextSwitchTrend     []int64   `json:"context_switch_trend"`
	ErrorRateTrend         []float64 `json:"error_rate_trend"`
	TrendTimestamps        []time.Time `json:"trend_timestamps"`
}

// Configuration

type MultiProjectConfig struct {
	// Core Settings
	MaxProjects             int                         `json:"max_projects"`
	MaxActiveProjects       int                         `json:"max_active_projects"`
	DefaultResourceLimits   ProjectResourceLimits       `json:"default_resource_limits"`
	GlobalResourceLimits    MultiProjectResourceLimits  `json:"global_resource_limits"`
	
	// Context Switching
	ContextSwitchTimeout    time.Duration               `json:"context_switch_timeout"`
	EnableFastSwitching     bool                        `json:"enable_fast_switching"`
	PrewarmProjects         bool                        `json:"prewarm_projects"`
	
	// Memory Management
	EnableAutoMemoryManagement bool                     `json:"enable_auto_memory_management"`
	MemoryPressureThreshold   float64                  `json:"memory_pressure_threshold"`
	GCTriggerThreshold        int64                    `json:"gc_trigger_threshold"`
	
	// Caching Strategy
	SharedCacheEnabled        bool                     `json:"shared_cache_enabled"`
	PerProjectCacheEnabled    bool                     `json:"per_project_cache_enabled"`
	CacheEvictionStrategy     string                   `json:"cache_eviction_strategy"`
	
	// Health and Monitoring
	HealthCheckInterval       time.Duration            `json:"health_check_interval"`
	MetricsCollectionInterval time.Duration            `json:"metrics_collection_interval"`
	EnableDetailedMetrics     bool                     `json:"enable_detailed_metrics"`
	
	// Project Management
	AutoDiscovery             bool                     `json:"auto_discovery"`
	ProjectRegistrationMode   string                   `json:"project_registration_mode"`
	EnableProjectWatcher      bool                     `json:"enable_project_watcher"`
	
	// Error Handling
	ErrorRecoveryEnabled      bool                     `json:"error_recovery_enabled"`
	MaxRetryAttempts          int                      `json:"max_retry_attempts"`
	BackoffStrategy           string                   `json:"backoff_strategy"`
}

// Event System

type ProjectEventType int

const (
	ProjectEventRegistered ProjectEventType = iota
	ProjectEventInitialized
	ProjectEventActivated
	ProjectEventSuspended
	ProjectEventFailed
	ProjectEventUnregistered
	ProjectEventContextSwitched
	ProjectEventResourceLimitExceeded
	ProjectEventHealthChanged
)

type ProjectEvent struct {
	Type        ProjectEventType       `json:"type"`
	ProjectID   ProjectID              `json:"project_id"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata"`
	Error       error                  `json:"error,omitempty"`
}

type ProjectEventHandler interface {
	HandleProjectEvent(event ProjectEvent) error
	GetHandlerID() string
}

// Cache and SCIP Integration Interfaces

type SCIPStoreInterface interface {
	// Project-scoped SCIP operations
	GetProjectIndex(projectID ProjectID) (interface{}, error)
	InvalidateProjectCache(projectID ProjectID) error
	GetProjectCacheStats(projectID ProjectID) interface{}
}

type CacheManagerInterface interface {
	// Project-scoped cache operations
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration) error
	Delete(key string) error
	Clear() error
	GetStats() interface{}
}

// Project Specification for Creation

type ProjectSpec struct {
	Root            string                 `json:"root"`
	Name            string                 `json:"name,omitempty"`
	Type            string                 `json:"type,omitempty"`
	Languages       []string               `json:"languages,omitempty"`
	ResourceLimits  *ProjectResourceLimits `json:"resource_limits,omitempty"`
	InitOptions     *ProjectInitOptions    `json:"init_options,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

type ProjectInitOptions struct {
	AutoDetectLanguages   bool          `json:"auto_detect_languages"`
	EnableSCIPIndexing    bool          `json:"enable_scip_indexing"`
	PreloadCache          bool          `json:"preload_cache"`
	InitTimeout           time.Duration `json:"init_timeout"`
	HealthCheckEndpoint   string        `json:"health_check_endpoint,omitempty"`
}

// Project Cache Configuration

type ProjectCacheConfig struct {
	L1CacheEnabled        bool          `json:"l1_cache_enabled"`
	L1MaxEntries          int           `json:"l1_max_entries"`
	L1TTL                 time.Duration `json:"l1_ttl"`
	L2CacheEnabled        bool          `json:"l2_cache_enabled"`
	L2MaxSizeMB           int64         `json:"l2_max_size_mb"`
	L2TTL                 time.Duration `json:"l2_ttl"`
	CompressionEnabled    bool          `json:"compression_enabled"`
	CompressionThreshold  int64         `json:"compression_threshold"`
	EvictionStrategy      string        `json:"eviction_strategy"`
	EnablePersistence     bool          `json:"enable_persistence"`
}

// Global Statistics

type GlobalProjectStats struct {
	TotalProjects           int                              `json:"total_projects"`
	ActiveProjects          int                              `json:"active_projects"`
	IdleProjects           int                              `json:"idle_projects"`
	SuspendedProjects      int                              `json:"suspended_projects"`
	FailedProjects         int                              `json:"failed_projects"`
	TotalMemoryUsage       int64                            `json:"total_memory_usage"`
	AverageMemoryPerProject int64                           `json:"average_memory_per_project"`
	TotalRequestsServed    int64                            `json:"total_requests_served"`
	AverageResponseTime    time.Duration                    `json:"average_response_time"`
	ContextSwitchesTotal   int64                            `json:"context_switches_total"`
	CacheHitRateGlobal     float64                          `json:"cache_hit_rate_global"`
	UptimeTotal            time.Duration                    `json:"uptime_total"`
	ProjectDistribution    map[string]int                   `json:"project_distribution"`
	LanguageDistribution   map[string]int                   `json:"language_distribution"`
	ErrorRatesByProject    map[ProjectID]float64            `json:"error_rates_by_project"`
}