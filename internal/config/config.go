package config

import (
	"encoding/json"
	"fmt"
	"lsp-gateway/internal/transport"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v2"
)

const (
	DefaultConfigFile = "config.yaml"
)

const (
	DefaultTransport = "stdio"
)

// Performance configuration constants
const (
	// Memory limits
	DefaultMemoryLimit = 1024 // 1GB in MB

	// Cache defaults
	DefaultCacheTTL = 30 * time.Minute

	// Indexing strategies
	IndexingStrategyEager       = "eager"
	IndexingStrategyLazy        = "lazy"
	IndexingStrategyIncremental = "incremental"
	IndexingStrategyFull        = "full"

	// Performance profiles
	PerformanceProfileDevelopment = "development"
	PerformanceProfileProduction  = "production"
	PerformanceProfileAnalysis    = "analysis"

	// Server types
	ServerTypeSingle    = "single"
	ServerTypeMulti     = "multi"
	ServerTypeWorkspace = "workspace"

	// Cache eviction strategies
	EvictionStrategyLRU    = "lru"
	EvictionStrategyLFU    = "lfu"
	EvictionStrategyRandom = "random"
	EvictionStrategyTTL    = "ttl"
)

const (
	ProjectTypeSingle          = "single-language"
	ProjectTypeMulti           = "multi-language"
	ProjectTypeMonorepo        = "monorepo"
	ProjectTypeWorkspace       = "workspace"
	ProjectTypeFrontendBackend = "frontend-backend"
	ProjectTypeMicroservices   = "microservices"
	ProjectTypePolyglot        = "polyglot"
	ProjectTypeEmpty           = "empty"
	ProjectTypeUnknown         = "unknown"
)

type LanguageInfo struct {
	Language     string   `yaml:"language" json:"language"`
	FilePatterns []string `yaml:"file_patterns" json:"file_patterns"`
	FileCount    int      `yaml:"file_count" json:"file_count"`
	RootMarkers  []string `yaml:"root_markers,omitempty" json:"root_markers,omitempty"`
}

type ProjectContext struct {
	ProjectType   string                 `yaml:"project_type" json:"project_type"`
	RootDirectory string                 `yaml:"root_directory" json:"root_directory"`
	WorkspaceRoot string                 `yaml:"workspace_root,omitempty" json:"workspace_root,omitempty"`
	Languages     []LanguageInfo         `yaml:"languages" json:"languages"`
	RequiredLSPs  []string               `yaml:"required_lsps" json:"required_lsps"`
	DetectedAt    time.Time              `yaml:"detected_at" json:"detected_at"`
	Metadata      map[string]interface{} `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

type ProjectServerOverride struct {
	Name      string                 `yaml:"name" json:"name"`
	Enabled   *bool                  `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Args      []string               `yaml:"args,omitempty" json:"args,omitempty"`
	Settings  map[string]interface{} `yaml:"settings,omitempty" json:"settings,omitempty"`
	Transport string                 `yaml:"transport,omitempty" json:"transport,omitempty"`
}

type ProjectConfig struct {
	ProjectID       string                  `yaml:"project_id" json:"project_id"`
	Name            string                  `yaml:"name,omitempty" json:"name,omitempty"`
	RootDirectory   string                  `yaml:"root_directory" json:"root_directory"`
	ServerOverrides []ProjectServerOverride `yaml:"server_overrides,omitempty" json:"server_overrides,omitempty"`
	EnabledServers  []string                `yaml:"enabled_servers,omitempty" json:"enabled_servers,omitempty"`
	Optimizations   map[string]interface{}  `yaml:"optimizations,omitempty" json:"optimizations,omitempty"`
	GeneratedAt     time.Time               `yaml:"generated_at" json:"generated_at"`
	Version         string                  `yaml:"version,omitempty" json:"version,omitempty"`
}

type ServerConfig struct {
	Name string `yaml:"name" json:"name"`

	Languages []string `yaml:"languages" json:"languages"`

	Command string `yaml:"command" json:"command"`

	Args []string `yaml:"args" json:"args"`

	Transport string `yaml:"transport" json:"transport"`

	RootMarkers []string `yaml:"root_markers,omitempty" json:"root_markers,omitempty"`

	Settings map[string]interface{} `yaml:"settings,omitempty" json:"settings,omitempty"`

	// Multi-server support fields
	Priority              int     `yaml:"priority,omitempty" json:"priority,omitempty"`
	Weight                float64 `yaml:"weight,omitempty" json:"weight,omitempty"`
	HealthCheckEndpoint   string  `yaml:"health_check_endpoint,omitempty" json:"health_check_endpoint,omitempty"`
	MaxConcurrentRequests int     `yaml:"max_concurrent_requests,omitempty" json:"max_concurrent_requests,omitempty"`

	// Enhanced multi-language fields
	WorkspaceRoots   map[string]string                 `yaml:"workspace_roots,omitempty" json:"workspace_roots,omitempty"`
	LanguageSettings map[string]map[string]interface{} `yaml:"language_settings,omitempty" json:"language_settings,omitempty"`
	ServerType       string                            `yaml:"server_type,omitempty" json:"server_type,omitempty"` // "single", "multi", "workspace"
	Dependencies     []string                          `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	Constraints      *ServerConstraints                `yaml:"constraints,omitempty" json:"constraints,omitempty"`
	Frameworks       []string                          `yaml:"frameworks,omitempty" json:"frameworks,omitempty"`
	Version          string                            `yaml:"version,omitempty" json:"version,omitempty"`
	BypassConfig     *ServerBypassConfig               `yaml:"bypass_config,omitempty" json:"bypass_config,omitempty"`
}

// SmartRouterConfig contains configuration for the SmartRouter
type SmartRouterConfig struct {
	DefaultStrategy             string            `yaml:"default_strategy,omitempty" json:"default_strategy,omitempty"`
	MethodStrategies            map[string]string `yaml:"method_strategies,omitempty" json:"method_strategies,omitempty"`
	EnablePerformanceMonitoring bool              `yaml:"enable_performance_monitoring,omitempty" json:"enable_performance_monitoring,omitempty"`
	EnableCircuitBreaker        bool              `yaml:"enable_circuit_breaker,omitempty" json:"enable_circuit_breaker,omitempty"`
	CircuitBreakerThreshold     int               `yaml:"circuit_breaker_threshold,omitempty" json:"circuit_breaker_threshold,omitempty"`
	CircuitBreakerTimeout       string            `yaml:"circuit_breaker_timeout,omitempty" json:"circuit_breaker_timeout,omitempty"`
}

// PerformanceCacheConfig contains configuration for performance caching
type PerformanceCacheConfig struct {
	Enabled bool   `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	MaxSize int    `yaml:"max_size,omitempty" json:"max_size,omitempty"`
	TTL     string `yaml:"ttl,omitempty" json:"ttl,omitempty"`
}

// RequestClassifierConfig contains configuration for request classification
type RequestClassifierConfig struct {
	Enabled bool                   `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Rules   map[string]interface{} `yaml:"rules,omitempty" json:"rules,omitempty"`
}

// ResponseAggregatorConfig contains configuration for response aggregation
type ResponseAggregatorConfig struct {
	Enabled    bool   `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	MaxServers int    `yaml:"max_servers,omitempty" json:"max_servers,omitempty"`
	Timeout    string `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// HealthMonitorConfig contains configuration for health monitoring
type HealthMonitorConfig struct {
	Enabled          bool   `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	CheckInterval    string `yaml:"check_interval,omitempty" json:"check_interval,omitempty"`
	FailureThreshold int    `yaml:"failure_threshold,omitempty" json:"failure_threshold,omitempty"`
}

type GatewayConfig struct {
	Servers []ServerConfig `yaml:"servers" json:"servers"`

	Port int `yaml:"port" json:"port"`

	Timeout               string          `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	MaxConcurrentRequests int             `yaml:"max_concurrent_requests,omitempty" json:"max_concurrent_requests,omitempty"`
	ProjectContext        *ProjectContext `yaml:"project_context,omitempty" json:"project_context,omitempty"`
	ProjectConfig         *ProjectConfig  `yaml:"project_config,omitempty" json:"project_config,omitempty"`
	ProjectAware          bool            `yaml:"project_aware,omitempty" json:"project_aware,omitempty"`

	// Multi-server configuration fields
	LanguagePools                   []LanguageServerPool `yaml:"language_pools,omitempty" json:"language_pools,omitempty"`
	GlobalMultiServerConfig         *MultiServerConfig   `yaml:"multi_server_config,omitempty" json:"multi_server_config,omitempty"`
	EnableConcurrentServers         bool                 `yaml:"enable_concurrent_servers" json:"enable_concurrent_servers"`
	MaxConcurrentServersPerLanguage int                  `yaml:"max_concurrent_servers_per_language" json:"max_concurrent_servers_per_language"`

	// SmartRouter configuration fields
	EnableSmartRouting bool               `yaml:"enable_smart_routing,omitempty" json:"enable_smart_routing,omitempty"`
	EnableEnhancements bool               `yaml:"enable_enhancements,omitempty" json:"enable_enhancements,omitempty"`
	SmartRouterConfig  *SmartRouterConfig `yaml:"smart_router_config,omitempty" json:"smart_router_config,omitempty"`

	// SCIP Smart Router configuration
	EnableSCIPRouting bool                   `yaml:"enable_scip_routing,omitempty" json:"enable_scip_routing,omitempty"`
	SCIPConfig        *SCIPSmartRouterConfig `yaml:"scip_config,omitempty" json:"scip_config,omitempty"`
	// Performance configuration
	PerformanceConfig *PerformanceConfiguration `yaml:"performance_config,omitempty" json:"performance_config,omitempty"`
	// Bypass configuration
	BypassConfig *BypassConfiguration `yaml:"bypass_config,omitempty" json:"bypass_config,omitempty"`
}

// PerformanceConfiguration contains configuration for performance optimization
type PerformanceConfiguration struct {
	Enabled         bool                   `yaml:"enabled" json:"enabled"`
	Profile         string                 `yaml:"profile,omitempty" json:"profile,omitempty"`
	AutoTuning      bool                   `yaml:"auto_tuning,omitempty" json:"auto_tuning,omitempty"`
	Version         string                 `yaml:"version,omitempty" json:"version,omitempty"`
	Caching         *CachingConfiguration  `yaml:"caching,omitempty" json:"caching,omitempty"`
	ResourceManager *ResourceManagerConfig `yaml:"resource_manager,omitempty" json:"resource_manager,omitempty"`
	Timeouts        *TimeoutConfiguration  `yaml:"timeouts,omitempty" json:"timeouts,omitempty"`
	LargeProject    *LargeProjectConfig    `yaml:"large_project,omitempty" json:"large_project,omitempty"`
	SCIP            *SCIPConfiguration     `yaml:"scip,omitempty" json:"scip,omitempty"`
	Storage         *StorageConfiguration  `yaml:"storage,omitempty" json:"storage,omitempty"`
}

// CachingConfiguration contains caching configuration
type CachingConfiguration struct {
	Enabled          bool          `yaml:"enabled" json:"enabled"`
	GlobalTTL        time.Duration `yaml:"global_ttl,omitempty" json:"global_ttl,omitempty"`
	MaxMemoryUsage   int64         `yaml:"max_memory_usage_mb,omitempty" json:"max_memory_usage_mb,omitempty"`
	EvictionStrategy string        `yaml:"eviction_strategy,omitempty" json:"eviction_strategy,omitempty"`
	ResponseCache    *CacheConfig  `yaml:"response_cache,omitempty" json:"response_cache,omitempty"`
	SemanticCache    *CacheConfig  `yaml:"semantic_cache,omitempty" json:"semantic_cache,omitempty"`
	ProjectCache     *CacheConfig  `yaml:"project_cache,omitempty" json:"project_cache,omitempty"`
	SymbolCache      *CacheConfig  `yaml:"symbol_cache,omitempty" json:"symbol_cache,omitempty"`
	CompletionCache  *CacheConfig  `yaml:"completion_cache,omitempty" json:"completion_cache,omitempty"`
	DiagnosticCache  *CacheConfig  `yaml:"diagnostic_cache,omitempty" json:"diagnostic_cache,omitempty"`
	FileSystemCache  *CacheConfig  `yaml:"filesystem_cache,omitempty" json:"filesystem_cache,omitempty"`
}

// CacheConfig contains individual cache configuration
type CacheConfig struct {
	Enabled bool          `yaml:"enabled" json:"enabled"`
	TTL     time.Duration `yaml:"ttl,omitempty" json:"ttl,omitempty"`
	MaxSize int64         `yaml:"max_size,omitempty" json:"max_size,omitempty"`
}

// SCIPConfiguration contains SCIP indexing configuration
type SCIPConfiguration struct {
	Enabled          bool                           `yaml:"enabled" json:"enabled"`
	IndexPath        string                         `yaml:"index_path,omitempty" json:"index_path,omitempty"`
	AutoRefresh      bool                           `yaml:"auto_refresh,omitempty" json:"auto_refresh,omitempty"`
	RefreshInterval  time.Duration                  `yaml:"refresh_interval,omitempty" json:"refresh_interval,omitempty"`
	FallbackToLSP    bool                           `yaml:"fallback_to_lsp,omitempty" json:"fallback_to_lsp,omitempty"`
	CacheConfig      CacheConfig                    `yaml:"cache,omitempty" json:"cache,omitempty"`
	LanguageSettings map[string]*SCIPLanguageConfig `yaml:"language_settings,omitempty" json:"language_settings,omitempty"`
}

// SCIPLanguageConfig contains language-specific SCIP configuration
type SCIPLanguageConfig struct {
	Enabled           bool                   `yaml:"enabled" json:"enabled"`
	IndexCommand      []string               `yaml:"index_command,omitempty" json:"index_command,omitempty"`
	IndexTimeout      time.Duration          `yaml:"index_timeout,omitempty" json:"index_timeout,omitempty"`
	IndexArguments    []string               `yaml:"index_arguments,omitempty" json:"index_arguments,omitempty"`
	WorkspaceSettings map[string]interface{} `yaml:"workspace_settings,omitempty" json:"workspace_settings,omitempty"`
}

// SCIPSmartRouterConfig contains configuration for SCIP Smart Router integration
type SCIPSmartRouterConfig struct {
	StorePath          string                      `yaml:"store_path,omitempty" json:"store_path,omitempty"`
	ProviderConfig     *SCIPProviderConfig         `yaml:"provider_config,omitempty" json:"provider_config,omitempty"`
	RouterConfig       *SCIPRoutingConfig          `yaml:"router_config,omitempty" json:"router_config,omitempty"`
	OptimizationConfig *AdaptiveOptimizationConfig `yaml:"optimization_config,omitempty" json:"optimization_config,omitempty"`
}

// SCIPProviderConfig configures the SCIP routing provider
type SCIPProviderConfig struct {
	HealthCheckInterval  time.Duration `yaml:"health_check_interval,omitempty" json:"health_check_interval,omitempty"`
	FallbackEnabled      bool          `yaml:"fallback_enabled,omitempty" json:"fallback_enabled,omitempty"`
	FallbackTimeout      time.Duration `yaml:"fallback_timeout,omitempty" json:"fallback_timeout,omitempty"`
	MinAcceptableQuality string        `yaml:"min_acceptable_quality,omitempty" json:"min_acceptable_quality,omitempty"`
	EnableMetricsLogging bool          `yaml:"enable_metrics_logging,omitempty" json:"enable_metrics_logging,omitempty"`
	EnableHealthLogging  bool          `yaml:"enable_health_logging,omitempty" json:"enable_health_logging,omitempty"`
}

// SCIPRoutingConfig configures SCIP-aware routing behavior
type SCIPRoutingConfig struct {
	DefaultStrategy            string             `yaml:"default_strategy,omitempty" json:"default_strategy,omitempty"`
	ConfidenceThreshold        float64            `yaml:"confidence_threshold,omitempty" json:"confidence_threshold,omitempty"`
	MaxSCIPLatency             time.Duration      `yaml:"max_scip_latency,omitempty" json:"max_scip_latency,omitempty"`
	EnableFallback             bool               `yaml:"enable_fallback,omitempty" json:"enable_fallback,omitempty"`
	EnableAdaptiveLearning     bool               `yaml:"enable_adaptive_learning,omitempty" json:"enable_adaptive_learning,omitempty"`
	CachePrewarmEnabled        bool               `yaml:"cache_prewarm_enabled,omitempty" json:"cache_prewarm_enabled,omitempty"`
	MethodStrategies           map[string]string  `yaml:"method_strategies,omitempty" json:"method_strategies,omitempty"`
	MethodConfidenceThresholds map[string]float64 `yaml:"method_confidence_thresholds,omitempty" json:"method_confidence_thresholds,omitempty"`
	OptimizationInterval       time.Duration      `yaml:"optimization_interval,omitempty" json:"optimization_interval,omitempty"`
	PerformanceWindowSize      int                `yaml:"performance_window_size,omitempty" json:"performance_window_size,omitempty"`
}

// AdaptiveOptimizationConfig configures adaptive optimization behavior
type AdaptiveOptimizationConfig struct {
	OptimizationInterval     time.Duration `yaml:"optimization_interval,omitempty" json:"optimization_interval,omitempty"`
	MinDataPoints            int           `yaml:"min_data_points,omitempty" json:"min_data_points,omitempty"`
	ConfidenceThreshold      float64       `yaml:"confidence_threshold,omitempty" json:"confidence_threshold,omitempty"`
	PerformanceThreshold     float64       `yaml:"performance_threshold,omitempty" json:"performance_threshold,omitempty"`
	LearningRate             float64       `yaml:"learning_rate,omitempty" json:"learning_rate,omitempty"`
	AdaptationRate           float64       `yaml:"adaptation_rate,omitempty" json:"adaptation_rate,omitempty"`
	ExplorationRate          float64       `yaml:"exploration_rate,omitempty" json:"exploration_rate,omitempty"`
	MaxThresholdAdjustment   float64       `yaml:"max_threshold_adjustment,omitempty" json:"max_threshold_adjustment,omitempty"`
	ThresholdDecayRate       float64       `yaml:"threshold_decay_rate,omitempty" json:"threshold_decay_rate,omitempty"`
	StrategyEvaluationWindow int           `yaml:"strategy_evaluation_window,omitempty" json:"strategy_evaluation_window,omitempty"`
	MinStrategyConfidence    float64       `yaml:"min_strategy_confidence,omitempty" json:"min_strategy_confidence,omitempty"`
	CacheWarmingEnabled      bool          `yaml:"cache_warming_enabled,omitempty" json:"cache_warming_enabled,omitempty"`
	WarmingTriggerThreshold  float64       `yaml:"warming_trigger_threshold,omitempty" json:"warming_trigger_threshold,omitempty"`
	MaxWarmingOperations     int           `yaml:"max_warming_operations,omitempty" json:"max_warming_operations,omitempty"`
}

// ResourceManagerConfig contains resource management configuration
type ResourceManagerConfig struct {
	MemoryLimits *MemoryLimitsConfig `yaml:"memory_limits,omitempty" json:"memory_limits,omitempty"`
	CPULimits    *CPULimitsConfig    `yaml:"cpu_limits,omitempty" json:"cpu_limits,omitempty"`
}

// MemoryLimitsConfig contains memory limit configuration
type MemoryLimitsConfig struct {
	MaxHeapSize    int64 `yaml:"max_heap_size_mb,omitempty" json:"max_heap_size_mb,omitempty"`
	SoftLimit      int64 `yaml:"soft_limit_mb,omitempty" json:"soft_limit_mb,omitempty"`
	PerServerLimit int64 `yaml:"per_server_limit_mb,omitempty" json:"per_server_limit_mb,omitempty"`
}

// CPULimitsConfig contains CPU limit configuration
type CPULimitsConfig struct {
	MaxUsagePercent float64 `yaml:"max_usage_percent,omitempty" json:"max_usage_percent,omitempty"`
	MaxCores        int     `yaml:"max_cores,omitempty" json:"max_cores,omitempty"`
}

// TimeoutConfiguration contains timeout configuration
type TimeoutConfiguration struct {
	GlobalTimeout     time.Duration            `yaml:"global_timeout,omitempty" json:"global_timeout,omitempty"`
	DefaultTimeout    time.Duration            `yaml:"default_timeout,omitempty" json:"default_timeout,omitempty"`
	ConnectionTimeout time.Duration            `yaml:"connection_timeout,omitempty" json:"connection_timeout,omitempty"`
	MethodTimeouts    map[string]time.Duration `yaml:"method_timeouts,omitempty" json:"method_timeouts,omitempty"`
	LanguageTimeouts  map[string]time.Duration `yaml:"language_timeouts,omitempty" json:"language_timeouts,omitempty"`
}

// LargeProjectConfig contains configuration for large projects
type LargeProjectConfig struct {
	AutoDetectSize        bool                      `yaml:"auto_detect_size,omitempty" json:"auto_detect_size,omitempty"`
	FileCountThreshold    int                       `yaml:"file_count_threshold,omitempty" json:"file_count_threshold,omitempty"`
	MaxWorkspaceSize      int64                     `yaml:"max_workspace_size_mb,omitempty" json:"max_workspace_size_mb,omitempty"`
	IndexingStrategy      string                    `yaml:"indexing_strategy,omitempty" json:"indexing_strategy,omitempty"`
	LazyLoading           bool                      `yaml:"lazy_loading,omitempty" json:"lazy_loading,omitempty"`
	WorkspacePartitioning bool                      `yaml:"workspace_partitioning,omitempty" json:"workspace_partitioning,omitempty"`
	ServerPoolScaling     *ServerPoolScalingConfig  `yaml:"server_pool_scaling,omitempty" json:"server_pool_scaling,omitempty"`
	BackgroundIndexing    *BackgroundIndexingConfig `yaml:"background_indexing,omitempty" json:"background_indexing,omitempty"`
}

// ServerPoolScalingConfig contains server pool scaling configuration
type ServerPoolScalingConfig struct {
	MinServers int `yaml:"min_servers,omitempty" json:"min_servers,omitempty"`
	MaxServers int `yaml:"max_servers,omitempty" json:"max_servers,omitempty"`
}

// BackgroundIndexingConfig contains background indexing configuration
type BackgroundIndexingConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// StorageConfiguration contains configuration for two-tier storage architecture
type StorageConfiguration struct {
	Enabled     bool                      `yaml:"enabled" json:"enabled"`
	Version     string                    `yaml:"version,omitempty" json:"version,omitempty"`
	Profile     string                    `yaml:"profile,omitempty" json:"profile,omitempty"`
	Tiers       *StorageTiersConfig       `yaml:"tiers,omitempty" json:"tiers,omitempty"`
	Strategy    *StorageStrategyConfig    `yaml:"strategy,omitempty" json:"strategy,omitempty"`
	Monitoring  *StorageMonitoringConfig  `yaml:"monitoring,omitempty" json:"monitoring,omitempty"`
	Maintenance *StorageMaintenanceConfig `yaml:"maintenance,omitempty" json:"maintenance,omitempty"`
	Security    *StorageSecurityConfig    `yaml:"security,omitempty" json:"security,omitempty"`
}

// StorageTiersConfig contains configuration for all storage tiers
type StorageTiersConfig struct {
	L1Memory *TierConfiguration `yaml:"l1_memory,omitempty" json:"l1_memory,omitempty"`
	L2Disk   *TierConfiguration `yaml:"l2_disk,omitempty" json:"l2_disk,omitempty"`
}

// TierConfiguration contains configuration for a specific storage tier
type TierConfiguration struct {
	Enabled        bool                      `yaml:"enabled" json:"enabled"`
	Capacity       string                    `yaml:"capacity,omitempty" json:"capacity,omitempty"`
	MaxEntries     int64                     `yaml:"max_entries,omitempty" json:"max_entries,omitempty"`
	EvictionPolicy string                    `yaml:"eviction_policy,omitempty" json:"eviction_policy,omitempty"`
	Backend        *BackendConfiguration     `yaml:"backend,omitempty" json:"backend,omitempty"`
	Compression    *CompressionConfiguration `yaml:"compression,omitempty" json:"compression,omitempty"`
	Encryption     *EncryptionConfiguration  `yaml:"encryption,omitempty" json:"encryption,omitempty"`
	Performance    *TierPerformanceConfig    `yaml:"performance,omitempty" json:"performance,omitempty"`
	Reliability    *TierReliabilityConfig    `yaml:"reliability,omitempty" json:"reliability,omitempty"`
}

// BackendConfiguration contains storage backend configuration
type BackendConfiguration struct {
	Type             string                 `yaml:"type" json:"type"`
	Path             string                 `yaml:"path,omitempty" json:"path,omitempty"`
	Endpoint         string                 `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Region           string                 `yaml:"region,omitempty" json:"region,omitempty"`
	Bucket           string                 `yaml:"bucket,omitempty" json:"bucket,omitempty"`
	ConnectionString string                 `yaml:"connection_string,omitempty" json:"connection_string,omitempty"`
	Authentication   *AuthenticationConfig  `yaml:"authentication,omitempty" json:"authentication,omitempty"`
	Options          map[string]interface{} `yaml:"options,omitempty" json:"options,omitempty"`
}

// AuthenticationConfig contains authentication configuration for storage backends
type AuthenticationConfig struct {
	Type      string `yaml:"type,omitempty" json:"type,omitempty"`
	Username  string `yaml:"username,omitempty" json:"username,omitempty"`
	Password  string `yaml:"password,omitempty" json:"password,omitempty"`
	APIKey    string `yaml:"api_key,omitempty" json:"api_key,omitempty"`
	SecretKey string `yaml:"secret_key,omitempty" json:"secret_key,omitempty"`
	Token     string `yaml:"token,omitempty" json:"token,omitempty"`
	CertPath  string `yaml:"cert_path,omitempty" json:"cert_path,omitempty"`
	KeyPath   string `yaml:"key_path,omitempty" json:"key_path,omitempty"`
}

// CompressionConfiguration contains compression settings
type CompressionConfiguration struct {
	Enabled   bool    `yaml:"enabled" json:"enabled"`
	Algorithm string  `yaml:"algorithm,omitempty" json:"algorithm,omitempty"`
	Level     int     `yaml:"level,omitempty" json:"level,omitempty"`
	MinSize   int64   `yaml:"min_size,omitempty" json:"min_size,omitempty"`
	Threshold float64 `yaml:"threshold,omitempty" json:"threshold,omitempty"`
}

// EncryptionConfiguration contains encryption settings
type EncryptionConfiguration struct {
	Enabled     bool          `yaml:"enabled" json:"enabled"`
	Algorithm   string        `yaml:"algorithm,omitempty" json:"algorithm,omitempty"`
	KeySize     int           `yaml:"key_size,omitempty" json:"key_size,omitempty"`
	KeyRotation time.Duration `yaml:"key_rotation,omitempty" json:"key_rotation,omitempty"`
	KeyPath     string        `yaml:"key_path,omitempty" json:"key_path,omitempty"`
}

// TierPerformanceConfig contains performance settings for a storage tier
type TierPerformanceConfig struct {
	MaxConcurrency     int  `yaml:"max_concurrency,omitempty" json:"max_concurrency,omitempty"`
	TimeoutMs          int  `yaml:"timeout_ms,omitempty" json:"timeout_ms,omitempty"`
	BatchSize          int  `yaml:"batch_size,omitempty" json:"batch_size,omitempty"`
	ReadBufferSize     int  `yaml:"read_buffer_size,omitempty" json:"read_buffer_size,omitempty"`
	WriteBufferSize    int  `yaml:"write_buffer_size,omitempty" json:"write_buffer_size,omitempty"`
	ConnectionPoolSize int  `yaml:"connection_pool_size,omitempty" json:"connection_pool_size,omitempty"`
	KeepAlive          bool `yaml:"keep_alive,omitempty" json:"keep_alive,omitempty"`
	PrefetchEnabled    bool `yaml:"prefetch_enabled,omitempty" json:"prefetch_enabled,omitempty"`
	PrefetchSize       int  `yaml:"prefetch_size,omitempty" json:"prefetch_size,omitempty"`
}

// TierReliabilityConfig contains reliability settings for a storage tier
type TierReliabilityConfig struct {
	RetryCount          int                          `yaml:"retry_count,omitempty" json:"retry_count,omitempty"`
	RetryDelayMs        int                          `yaml:"retry_delay_ms,omitempty" json:"retry_delay_ms,omitempty"`
	CircuitBreaker      *CircuitBreakerConfiguration `yaml:"circuit_breaker,omitempty" json:"circuit_breaker,omitempty"`
	HealthCheckInterval time.Duration                `yaml:"health_check_interval,omitempty" json:"health_check_interval,omitempty"`
	FailureThreshold    int                          `yaml:"failure_threshold,omitempty" json:"failure_threshold,omitempty"`
	RecoveryTimeout     time.Duration                `yaml:"recovery_timeout,omitempty" json:"recovery_timeout,omitempty"`
	ReplicationEnabled  bool                         `yaml:"replication_enabled,omitempty" json:"replication_enabled,omitempty"`
	ReplicationFactor   int                          `yaml:"replication_factor,omitempty" json:"replication_factor,omitempty"`
}

// CircuitBreakerConfiguration contains circuit breaker settings
type CircuitBreakerConfiguration struct {
	Enabled          bool          `yaml:"enabled" json:"enabled"`
	FailureThreshold int           `yaml:"failure_threshold,omitempty" json:"failure_threshold,omitempty"`
	RecoveryTimeout  time.Duration `yaml:"recovery_timeout,omitempty" json:"recovery_timeout,omitempty"`
	HalfOpenRequests int           `yaml:"half_open_requests,omitempty" json:"half_open_requests,omitempty"`
	MinRequestCount  int           `yaml:"min_request_count,omitempty" json:"min_request_count,omitempty"`
}

// StorageStrategyConfig contains configuration for storage strategies
type StorageStrategyConfig struct {
	PromotionStrategy *PromotionStrategyConfig `yaml:"promotion_strategy,omitempty" json:"promotion_strategy,omitempty"`
	EvictionPolicy    *EvictionPolicyConfig    `yaml:"eviction_policy,omitempty" json:"eviction_policy,omitempty"`
	AccessTracking    *AccessTrackingConfig    `yaml:"access_tracking,omitempty" json:"access_tracking,omitempty"`
	AutoOptimization  *AutoOptimizationConfig  `yaml:"auto_optimization,omitempty" json:"auto_optimization,omitempty"`
}

// PromotionStrategyConfig contains promotion strategy configuration
type PromotionStrategyConfig struct {
	Type               string                 `yaml:"type,omitempty" json:"type,omitempty"`
	Enabled            bool                   `yaml:"enabled" json:"enabled"`
	MinAccessCount     int64                  `yaml:"min_access_count,omitempty" json:"min_access_count,omitempty"`
	MinAccessFrequency float64                `yaml:"min_access_frequency,omitempty" json:"min_access_frequency,omitempty"`
	AccessTimeWindow   time.Duration          `yaml:"access_time_window,omitempty" json:"access_time_window,omitempty"`
	PromotionCooldown  time.Duration          `yaml:"promotion_cooldown,omitempty" json:"promotion_cooldown,omitempty"`
	RecencyWeight      float64                `yaml:"recency_weight,omitempty" json:"recency_weight,omitempty"`
	FrequencyWeight    float64                `yaml:"frequency_weight,omitempty" json:"frequency_weight,omitempty"`
	SizeWeight         float64                `yaml:"size_weight,omitempty" json:"size_weight,omitempty"`
	TierThresholds     map[string]float64     `yaml:"tier_thresholds,omitempty" json:"tier_thresholds,omitempty"`
	CustomParameters   map[string]interface{} `yaml:"custom_parameters,omitempty" json:"custom_parameters,omitempty"`
}

// EvictionPolicyConfig contains eviction policy configuration
type EvictionPolicyConfig struct {
	Type                string                 `yaml:"type,omitempty" json:"type,omitempty"`
	Enabled             bool                   `yaml:"enabled" json:"enabled"`
	EvictionThreshold   float64                `yaml:"eviction_threshold,omitempty" json:"eviction_threshold,omitempty"`
	TargetUtilization   float64                `yaml:"target_utilization,omitempty" json:"target_utilization,omitempty"`
	EvictionBatchSize   int                    `yaml:"eviction_batch_size,omitempty" json:"eviction_batch_size,omitempty"`
	AccessAgeThreshold  time.Duration          `yaml:"access_age_threshold,omitempty" json:"access_age_threshold,omitempty"`
	InactivityThreshold time.Duration          `yaml:"inactivity_threshold,omitempty" json:"inactivity_threshold,omitempty"`
	DefaultTTL          time.Duration          `yaml:"default_ttl,omitempty" json:"default_ttl,omitempty"`
	MaxTTL              time.Duration          `yaml:"max_ttl,omitempty" json:"max_ttl,omitempty"`
	SizeWeight          float64                `yaml:"size_weight,omitempty" json:"size_weight,omitempty"`
	CustomParameters    map[string]interface{} `yaml:"custom_parameters,omitempty" json:"custom_parameters,omitempty"`
}

// AccessTrackingConfig contains access pattern tracking configuration
type AccessTrackingConfig struct {
	Enabled              bool          `yaml:"enabled" json:"enabled"`
	TrackingGranularity  time.Duration `yaml:"tracking_granularity,omitempty" json:"tracking_granularity,omitempty"`
	HistoryRetention     time.Duration `yaml:"history_retention,omitempty" json:"history_retention,omitempty"`
	MaxTrackedKeys       int           `yaml:"max_tracked_keys,omitempty" json:"max_tracked_keys,omitempty"`
	AnalysisInterval     time.Duration `yaml:"analysis_interval,omitempty" json:"analysis_interval,omitempty"`
	MinSampleSize        int           `yaml:"min_sample_size,omitempty" json:"min_sample_size,omitempty"`
	ConfidenceThreshold  float64       `yaml:"confidence_threshold,omitempty" json:"confidence_threshold,omitempty"`
	SeasonalityDetection bool          `yaml:"seasonality_detection,omitempty" json:"seasonality_detection,omitempty"`
	TrendDetection       bool          `yaml:"trend_detection,omitempty" json:"trend_detection,omitempty"`
	LocalityTracking     bool          `yaml:"locality_tracking,omitempty" json:"locality_tracking,omitempty"`
	SemanticAnalysis     bool          `yaml:"semantic_analysis,omitempty" json:"semantic_analysis,omitempty"`
}

// AutoOptimizationConfig contains automatic optimization configuration
type AutoOptimizationConfig struct {
	Enabled              bool          `yaml:"enabled" json:"enabled"`
	OptimizationInterval time.Duration `yaml:"optimization_interval,omitempty" json:"optimization_interval,omitempty"`
	PerformanceThreshold float64       `yaml:"performance_threshold,omitempty" json:"performance_threshold,omitempty"`
	CapacityThreshold    float64       `yaml:"capacity_threshold,omitempty" json:"capacity_threshold,omitempty"`
	AutoRebalancing      bool          `yaml:"auto_rebalancing,omitempty" json:"auto_rebalancing,omitempty"`
	AutoCompaction       bool          `yaml:"auto_compaction,omitempty" json:"auto_compaction,omitempty"`
	AutoTuning           bool          `yaml:"auto_tuning,omitempty" json:"auto_tuning,omitempty"`
	LearningEnabled      bool          `yaml:"learning_enabled,omitempty" json:"learning_enabled,omitempty"`
}

// StorageMonitoringConfig contains monitoring configuration
type StorageMonitoringConfig struct {
	Enabled         bool               `yaml:"enabled" json:"enabled"`
	MetricsInterval time.Duration      `yaml:"metrics_interval,omitempty" json:"metrics_interval,omitempty"`
	HealthInterval  time.Duration      `yaml:"health_interval,omitempty" json:"health_interval,omitempty"`
	TraceRequests   bool               `yaml:"trace_requests,omitempty" json:"trace_requests,omitempty"`
	LogLevel        string             `yaml:"log_level,omitempty" json:"log_level,omitempty"`
	AlertThresholds map[string]float64 `yaml:"alert_thresholds,omitempty" json:"alert_thresholds,omitempty"`
	ExportFormat    string             `yaml:"export_format,omitempty" json:"export_format,omitempty"`
	ExportEndpoint  string             `yaml:"export_endpoint,omitempty" json:"export_endpoint,omitempty"`
	RetentionPeriod time.Duration      `yaml:"retention_period,omitempty" json:"retention_period,omitempty"`
	DetailedMetrics bool               `yaml:"detailed_metrics,omitempty" json:"detailed_metrics,omitempty"`
}

// StorageMaintenanceConfig contains maintenance configuration
type StorageMaintenanceConfig struct {
	Enabled             bool                     `yaml:"enabled" json:"enabled"`
	Schedule            string                   `yaml:"schedule,omitempty" json:"schedule,omitempty"`
	MaintenanceWindow   *MaintenanceWindowConfig `yaml:"maintenance_window,omitempty" json:"maintenance_window,omitempty"`
	CompactionEnabled   bool                     `yaml:"compaction_enabled,omitempty" json:"compaction_enabled,omitempty"`
	CompactionThreshold float64                  `yaml:"compaction_threshold,omitempty" json:"compaction_threshold,omitempty"`
	VacuumEnabled       bool                     `yaml:"vacuum_enabled,omitempty" json:"vacuum_enabled,omitempty"`
	VacuumInterval      time.Duration            `yaml:"vacuum_interval,omitempty" json:"vacuum_interval,omitempty"`
	CleanupEnabled      bool                     `yaml:"cleanup_enabled,omitempty" json:"cleanup_enabled,omitempty"`
	CleanupAge          time.Duration            `yaml:"cleanup_age,omitempty" json:"cleanup_age,omitempty"`
	BackupEnabled       bool                     `yaml:"backup_enabled,omitempty" json:"backup_enabled,omitempty"`
	BackupInterval      time.Duration            `yaml:"backup_interval,omitempty" json:"backup_interval,omitempty"`
	BackupRetention     time.Duration            `yaml:"backup_retention,omitempty" json:"backup_retention,omitempty"`
}

// MaintenanceWindowConfig contains maintenance window configuration
type MaintenanceWindowConfig struct {
	StartTime string                 `yaml:"start_time,omitempty" json:"start_time,omitempty"`
	EndTime   string                 `yaml:"end_time,omitempty" json:"end_time,omitempty"`
	Timezone  string                 `yaml:"timezone,omitempty" json:"timezone,omitempty"`
	Days      []string               `yaml:"days,omitempty" json:"days,omitempty"`
	Blackouts []BlackoutPeriodConfig `yaml:"blackouts,omitempty" json:"blackouts,omitempty"`
}

// BlackoutPeriodConfig contains blackout period configuration
type BlackoutPeriodConfig struct {
	Start       time.Time `yaml:"start" json:"start"`
	End         time.Time `yaml:"end" json:"end"`
	Description string    `yaml:"description,omitempty" json:"description,omitempty"`
	Recurring   bool      `yaml:"recurring,omitempty" json:"recurring,omitempty"`
}

// StorageSecurityConfig contains security configuration
type StorageSecurityConfig struct {
	EncryptionAtRest    bool                      `yaml:"encryption_at_rest,omitempty" json:"encryption_at_rest,omitempty"`
	EncryptionInTransit bool                      `yaml:"encryption_in_transit,omitempty" json:"encryption_in_transit,omitempty"`
	AccessControl       *AccessControlConfig      `yaml:"access_control,omitempty" json:"access_control,omitempty"`
	AuditLogging        *AuditLoggingConfig       `yaml:"audit_logging,omitempty" json:"audit_logging,omitempty"`
	DataClassification  *DataClassificationConfig `yaml:"data_classification,omitempty" json:"data_classification,omitempty"`
}

// AccessControlConfig contains access control configuration
type AccessControlConfig struct {
	Enabled       bool                `yaml:"enabled" json:"enabled"`
	DefaultPolicy string              `yaml:"default_policy,omitempty" json:"default_policy,omitempty"`
	Roles         map[string]string   `yaml:"roles,omitempty" json:"roles,omitempty"`
	Permissions   map[string][]string `yaml:"permissions,omitempty" json:"permissions,omitempty"`
	IPWhitelist   []string            `yaml:"ip_whitelist,omitempty" json:"ip_whitelist,omitempty"`
	RateLimiting  *RateLimitingConfig `yaml:"rate_limiting,omitempty" json:"rate_limiting,omitempty"`
}

// RateLimitingConfig contains rate limiting configuration
type RateLimitingConfig struct {
	Enabled           bool          `yaml:"enabled" json:"enabled"`
	RequestsPerMinute int           `yaml:"requests_per_minute,omitempty" json:"requests_per_minute,omitempty"`
	BurstSize         int           `yaml:"burst_size,omitempty" json:"burst_size,omitempty"`
	WindowSize        time.Duration `yaml:"window_size,omitempty" json:"window_size,omitempty"`
}

// AuditLoggingConfig contains audit logging configuration
type AuditLoggingConfig struct {
	Enabled       bool   `yaml:"enabled" json:"enabled"`
	LogLevel      string `yaml:"log_level,omitempty" json:"log_level,omitempty"`
	LogLocation   string `yaml:"log_location,omitempty" json:"log_location,omitempty"`
	RetentionDays int    `yaml:"retention_days,omitempty" json:"retention_days,omitempty"`
	LogFormat     string `yaml:"log_format,omitempty" json:"log_format,omitempty"`
	IncludeData   bool   `yaml:"include_data,omitempty" json:"include_data,omitempty"`
}

// DataClassificationConfig contains data classification configuration
type DataClassificationConfig struct {
	Enabled             bool              `yaml:"enabled" json:"enabled"`
	DefaultLevel        string            `yaml:"default_level,omitempty" json:"default_level,omitempty"`
	ClassificationRules map[string]string `yaml:"classification_rules,omitempty" json:"classification_rules,omitempty"`
	HandlingPolicies    map[string]string `yaml:"handling_policies,omitempty" json:"handling_policies,omitempty"`
}

func DefaultConfig() *GatewayConfig {
	config := &GatewayConfig{
		Port:                            8080,
		Timeout:                         "30s",
		MaxConcurrentRequests:           100,
		ProjectAware:                    false,
		EnableConcurrentServers:         false,
		MaxConcurrentServersPerLanguage: DEFAULT_MAX_CONCURRENT_SERVERS_PER_LANG,

		// SmartRouter defaults (disabled by default for backward compatibility)
		EnableSmartRouting: false,
		EnableEnhancements: false,
		SmartRouterConfig: &SmartRouterConfig{
			DefaultStrategy: "single_target_with_fallback",
			MethodStrategies: map[string]string{
				"textDocument/definition":     "single_target_with_fallback",
				"textDocument/references":     "multi_target_parallel",
				"textDocument/documentSymbol": "single_target_with_fallback",
				"workspace/symbol":            "broadcast_aggregate",
				"textDocument/hover":          "primary_with_enhancement",
			},
			EnablePerformanceMonitoring: true,
			EnableCircuitBreaker:        true,
			CircuitBreakerThreshold:     5,
			CircuitBreakerTimeout:       "30s",
		},

		// Performance configuration defaults
		PerformanceConfig: DefaultPerformanceConfiguration(),
		
		// Bypass configuration defaults
		BypassConfig: DefaultBypassConfiguration(),

		Servers: []ServerConfig{
			{
				Name:        "go-lsp",
				Languages:   []string{"go"},
				Command:     "gopls",
				Args:        []string{},
				Transport:   DefaultTransport,
				RootMarkers: []string{"go.mod", "go.sum"},
				Priority:    1,
				Weight:      1.0,
			},
		},
		GlobalMultiServerConfig: DefaultMultiServerConfig(),
		LanguagePools:           []LanguageServerPool{},
	}

	// Ensure defaults are set
	config.EnsureMultiServerDefaults()

	return config
}

// DefaultPerformanceConfiguration returns default performance configuration
func DefaultPerformanceConfiguration() *PerformanceConfiguration {
	return &PerformanceConfiguration{
		Enabled:    false,
		Profile:    PerformanceProfileDevelopment,
		AutoTuning: false,
		Version:    "1.0",
		Caching: &CachingConfiguration{
			Enabled:          false,
			GlobalTTL:        DefaultCacheTTL,
			MaxMemoryUsage:   DefaultMemoryLimit,
			EvictionStrategy: "LRU",
			ResponseCache:    &CacheConfig{Enabled: false, TTL: 5 * time.Minute, MaxSize: 1000},
			SemanticCache:    &CacheConfig{Enabled: false, TTL: 15 * time.Minute, MaxSize: 500},
			ProjectCache:     &CacheConfig{Enabled: false, TTL: 30 * time.Minute, MaxSize: 100},
			SymbolCache:      &CacheConfig{Enabled: false, TTL: 10 * time.Minute, MaxSize: 2000},
			CompletionCache:  &CacheConfig{Enabled: false, TTL: 2 * time.Minute, MaxSize: 5000},
			DiagnosticCache:  &CacheConfig{Enabled: false, TTL: 1 * time.Minute, MaxSize: 1000},
			FileSystemCache:  &CacheConfig{Enabled: false, TTL: 5 * time.Minute, MaxSize: 1000},
		},
		ResourceManager: &ResourceManagerConfig{
			MemoryLimits: &MemoryLimitsConfig{
				MaxHeapSize:    DefaultMemoryLimit,
				SoftLimit:      DefaultMemoryLimit * 8 / 10, // 80% of max
				PerServerLimit: DefaultMemoryLimit / 4,      // 25% per server
			},
			CPULimits: &CPULimitsConfig{
				MaxUsagePercent: 80.0,
				MaxCores:        4,
			},
		},
		Timeouts: &TimeoutConfiguration{
			GlobalTimeout:     30 * time.Second,
			DefaultTimeout:    15 * time.Second,
			ConnectionTimeout: 5 * time.Second,
			MethodTimeouts:    make(map[string]time.Duration),
			LanguageTimeouts:  make(map[string]time.Duration),
		},
		LargeProject: &LargeProjectConfig{
			AutoDetectSize:        true,
			FileCountThreshold:    10000,
			MaxWorkspaceSize:      10240, // 10GB
			IndexingStrategy:      IndexingStrategyEager,
			LazyLoading:           false,
			WorkspacePartitioning: false,
			ServerPoolScaling: &ServerPoolScalingConfig{
				MinServers: 1,
				MaxServers: 3,
			},
			BackgroundIndexing: &BackgroundIndexingConfig{
				Enabled: false,
			},
		},
		SCIP: &SCIPConfiguration{
			Enabled:         false,
			IndexPath:       ".scip",
			AutoRefresh:     false,
			RefreshInterval: 30 * time.Minute,
			FallbackToLSP:   true,
			CacheConfig: CacheConfig{
				Enabled: false,
				TTL:     DefaultCacheTTL,
				MaxSize: 1000,
			},
			LanguageSettings: map[string]*SCIPLanguageConfig{
				"go": {
					Enabled:      false,
					IndexCommand: []string{"scip-go"},
					IndexTimeout: 10 * time.Minute,
					IndexArguments: []string{
						"--module-path", ".",
						"--output", ".scip/index.scip",
					},
					WorkspaceSettings: make(map[string]interface{}),
				},
				"typescript": {
					Enabled:      false,
					IndexCommand: []string{"scip-typescript"},
					IndexTimeout: 15 * time.Minute,
					IndexArguments: []string{
						"--project-root", ".",
						"--output", ".scip/index.scip",
					},
					WorkspaceSettings: make(map[string]interface{}),
				},
				"python": {
					Enabled:      false,
					IndexCommand: []string{"scip-python"},
					IndexTimeout: 10 * time.Minute,
					IndexArguments: []string{
						"--project-root", ".",
						"--output", ".scip/index.scip",
					},
					WorkspaceSettings: make(map[string]interface{}),
				},
			},
		},
		Storage: DefaultStorageConfiguration(),
	}
}

// DefaultStorageConfiguration returns default storage configuration
func DefaultStorageConfiguration() *StorageConfiguration {
	return &StorageConfiguration{
		Enabled: false,
		Version: "1.0",
		Profile: "development",
		Tiers: &StorageTiersConfig{
			L1Memory: &TierConfiguration{
				Enabled:        true,
				Capacity:       "4GB",
				MaxEntries:     100000,
				EvictionPolicy: "lru",
				Backend: &BackendConfiguration{
					Type: "memory",
				},
				Performance: &TierPerformanceConfig{
					MaxConcurrency:     100,
					TimeoutMs:          1000,
					BatchSize:          100,
					ReadBufferSize:     8192,
					WriteBufferSize:    8192,
					ConnectionPoolSize: 10,
					KeepAlive:          true,
					PrefetchEnabled:    true,
					PrefetchSize:       10,
				},
				Reliability: &TierReliabilityConfig{
					RetryCount:          3,
					RetryDelayMs:        100,
					HealthCheckInterval: 30 * time.Second,
					FailureThreshold:    5,
					RecoveryTimeout:     60 * time.Second,
					CircuitBreaker: &CircuitBreakerConfiguration{
						Enabled:          true,
						FailureThreshold: 10,
						RecoveryTimeout:  30 * time.Second,
						HalfOpenRequests: 5,
						MinRequestCount:  20,
					},
				},
			},
			L2Disk: &TierConfiguration{
				Enabled:        false,
				Capacity:       "200GB",
				MaxEntries:     1000000,
				EvictionPolicy: "lru",
				Backend: &BackendConfiguration{
					Type: "local_disk",
					Path: "/opt/lspg/cache",
				},
				Compression: &CompressionConfiguration{
					Enabled:   true,
					Algorithm: "snappy",
					Level:     3,
					MinSize:   1024,
					Threshold: 0.8,
				},
				Performance: &TierPerformanceConfig{
					MaxConcurrency:     50,
					TimeoutMs:          5000,
					BatchSize:          50,
					ReadBufferSize:     32768,
					WriteBufferSize:    32768,
					ConnectionPoolSize: 5,
					KeepAlive:          true,
					PrefetchEnabled:    false,
					PrefetchSize:       5,
				},
				Reliability: &TierReliabilityConfig{
					RetryCount:          3,
					RetryDelayMs:        200,
					HealthCheckInterval: 60 * time.Second,
					FailureThreshold:    3,
					RecoveryTimeout:     120 * time.Second,
					CircuitBreaker: &CircuitBreakerConfiguration{
						Enabled:          true,
						FailureThreshold: 5,
						RecoveryTimeout:  60 * time.Second,
						HalfOpenRequests: 3,
						MinRequestCount:  10,
					},
				},
			},
		},
		Strategy: &StorageStrategyConfig{
			PromotionStrategy: &PromotionStrategyConfig{
				Type:               "adaptive",
				Enabled:            true,
				MinAccessCount:     3,
				MinAccessFrequency: 0.1,
				AccessTimeWindow:   24 * time.Hour,
				PromotionCooldown:  5 * time.Minute,
				RecencyWeight:      0.4,
				FrequencyWeight:    0.4,
				SizeWeight:         0.2,
				TierThresholds: map[string]float64{
					"l1_to_l2": 0.7,
					"l2_to_l1": 0.3,
				},
			},
			EvictionPolicy: &EvictionPolicyConfig{
				Type:                "lru",
				Enabled:             true,
				EvictionThreshold:   0.85,
				TargetUtilization:   0.75,
				EvictionBatchSize:   100,
				AccessAgeThreshold:  7 * 24 * time.Hour,
				InactivityThreshold: 3 * 24 * time.Hour,
				DefaultTTL:          24 * time.Hour,
				MaxTTL:              7 * 24 * time.Hour,
				SizeWeight:          0.3,
			},
			AccessTracking: &AccessTrackingConfig{
				Enabled:              true,
				TrackingGranularity:  time.Minute,
				HistoryRetention:     7 * 24 * time.Hour,
				MaxTrackedKeys:       100000,
				AnalysisInterval:     time.Hour,
				MinSampleSize:        10,
				ConfidenceThreshold:  0.7,
				SeasonalityDetection: true,
				TrendDetection:       true,
				LocalityTracking:     true,
				SemanticAnalysis:     false,
			},
			AutoOptimization: &AutoOptimizationConfig{
				Enabled:              false,
				OptimizationInterval: 6 * time.Hour,
				PerformanceThreshold: 0.8,
				CapacityThreshold:    0.8,
				AutoRebalancing:      false,
				AutoCompaction:       true,
				AutoTuning:           false,
				LearningEnabled:      false,
			},
		},
		Monitoring: &StorageMonitoringConfig{
			Enabled:         true,
			MetricsInterval: 30 * time.Second,
			HealthInterval:  60 * time.Second,
			TraceRequests:   false,
			LogLevel:        "info",
			AlertThresholds: map[string]float64{
				"hit_rate_low":    0.5,
				"latency_high":    1000.0, // milliseconds
				"error_rate_high": 0.05,   // 5%
				"capacity_high":   0.9,    // 90%
			},
			ExportFormat:    "prometheus",
			RetentionPeriod: 7 * 24 * time.Hour,
			DetailedMetrics: false,
		},
		Maintenance: &StorageMaintenanceConfig{
			Enabled:             true,
			Schedule:            "0 2 * * *", // Daily at 2 AM
			CompactionEnabled:   true,
			CompactionThreshold: 0.3,
			VacuumEnabled:       true,
			VacuumInterval:      24 * time.Hour,
			CleanupEnabled:      true,
			CleanupAge:          7 * 24 * time.Hour,
			BackupEnabled:       false,
			BackupInterval:      24 * time.Hour,
			BackupRetention:     30 * 24 * time.Hour,
			MaintenanceWindow: &MaintenanceWindowConfig{
				StartTime: "02:00",
				EndTime:   "04:00",
				Timezone:  "UTC",
				Days:      []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"},
			},
		},
		Security: &StorageSecurityConfig{
			EncryptionAtRest:    false,
			EncryptionInTransit: false,
			AccessControl: &AccessControlConfig{
				Enabled:       false,
				DefaultPolicy: "deny",
				RateLimiting: &RateLimitingConfig{
					Enabled:           false,
					RequestsPerMinute: 1000,
					BurstSize:         100,
					WindowSize:        time.Minute,
				},
			},
			AuditLogging: &AuditLoggingConfig{
				Enabled:       false,
				LogLevel:      "info",
				LogLocation:   "/var/log/lsp-gateway/audit.log",
				RetentionDays: 30,
				LogFormat:     "json",
				IncludeData:   false,
			},
			DataClassification: &DataClassificationConfig{
				Enabled:      false,
				DefaultLevel: "internal",
				ClassificationRules: map[string]string{
					"*.secret": "confidential",
					"*.key":    "confidential",
					"*.cert":   "restricted",
				},
			},
		},
	}
}


func (s *ServerConfig) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("server name cannot be empty")
	}

	// Check for extremely long server names
	if len(s.Name) > 500 {
		return fmt.Errorf("server name too long: %d characters, maximum 500 allowed", len(s.Name))
	}

	if len(s.Languages) == 0 {
		return fmt.Errorf("server must support at least one language")
	}

	// Validate individual language strings
	for i, lang := range s.Languages {
		if strings.TrimSpace(lang) == "" {
			return fmt.Errorf("language at index %d cannot be empty or whitespace-only", i)
		}
	}

	if s.Command == "" {
		return fmt.Errorf("server command cannot be empty")
	}

	// Check for invalid characters in command
	if !utf8.ValidString(s.Command) {
		return fmt.Errorf("command contains invalid UTF-8 characters")
	}
	if strings.ContainsAny(s.Command, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x0b\x0c\x0e\x0f") {
		return fmt.Errorf("command contains invalid control characters")
	}

	if s.Transport == "" {
		return fmt.Errorf("server transport cannot be empty")
	}

	validTransports := map[string]bool{
		transport.TransportStdio: true,
		transport.TransportTCP:   true,
		transport.TransportHTTP:  true,
	}

	if !validTransports[s.Transport] {
		return fmt.Errorf("invalid transport type: %s, must be one of: stdio, tcp, http", s.Transport)
	}

	// Validate multi-server specific fields
	if err := s.ValidateMultiServerFields(); err != nil {
		return fmt.Errorf("multi-server field validation failed: %w", err)
	}

	return nil
}

func (pc *ProjectContext) Validate() error {
	if pc.ProjectType == "" {
		return fmt.Errorf("project type cannot be empty")
	}

	validTypes := map[string]bool{
		ProjectTypeSingle:          true,
		ProjectTypeMulti:           true,
		ProjectTypeMonorepo:        true,
		ProjectTypeWorkspace:       true,
		ProjectTypeFrontendBackend: true,
		ProjectTypeMicroservices:   true,
		ProjectTypePolyglot:        true,
		ProjectTypeEmpty:           true,
		ProjectTypeUnknown:         true,
	}

	if !validTypes[pc.ProjectType] {
		return fmt.Errorf("invalid project type: %s, must be one of: single-language, multi-language, monorepo, workspace, frontend-backend, microservices, polyglot, empty, unknown", pc.ProjectType)
	}

	if pc.RootDirectory == "" {
		return fmt.Errorf("root directory cannot be empty")
	}

	if !filepath.IsAbs(pc.RootDirectory) {
		return fmt.Errorf("root directory must be an absolute path: %s", pc.RootDirectory)
	}

	if pc.WorkspaceRoot != "" && !filepath.IsAbs(pc.WorkspaceRoot) {
		return fmt.Errorf("workspace root must be an absolute path: %s", pc.WorkspaceRoot)
	}

	if len(pc.Languages) == 0 {
		return fmt.Errorf("at least one language must be detected")
	}

	for i, lang := range pc.Languages {
		if err := lang.Validate(); err != nil {
			return fmt.Errorf("language info at index %d: %w", i, err)
		}
	}

	if pc.DetectedAt.IsZero() {
		return fmt.Errorf("detected_at timestamp cannot be zero")
	}

	return nil
}

func (li *LanguageInfo) Validate() error {
	if li.Language == "" {
		return fmt.Errorf("language cannot be empty")
	}

	if strings.TrimSpace(li.Language) == "" {
		return fmt.Errorf("language cannot be whitespace-only")
	}

	if len(li.FilePatterns) == 0 {
		return fmt.Errorf("at least one file pattern must be specified")
	}

	for i, pattern := range li.FilePatterns {
		if strings.TrimSpace(pattern) == "" {
			return fmt.Errorf("file pattern at index %d cannot be empty or whitespace-only", i)
		}
	}

	if li.FileCount < 0 {
		return fmt.Errorf("file count cannot be negative: %d", li.FileCount)
	}

	return nil
}

func (pc *ProjectConfig) Validate() error {
	if pc.ProjectID == "" {
		return fmt.Errorf("project ID cannot be empty")
	}

	if pc.RootDirectory == "" {
		return fmt.Errorf("root directory cannot be empty")
	}

	if !filepath.IsAbs(pc.RootDirectory) {
		return fmt.Errorf("root directory must be an absolute path: %s", pc.RootDirectory)
	}

	for i, override := range pc.ServerOverrides {
		if err := override.Validate(); err != nil {
			return fmt.Errorf("server override at index %d: %w", i, err)
		}
	}

	for i, serverName := range pc.EnabledServers {
		if strings.TrimSpace(serverName) == "" {
			return fmt.Errorf("enabled server name at index %d cannot be empty or whitespace-only", i)
		}
	}

	if pc.GeneratedAt.IsZero() {
		return fmt.Errorf("generated_at timestamp cannot be zero")
	}

	return nil
}

func (pso *ProjectServerOverride) Validate() error {
	if pso.Name == "" {
		return fmt.Errorf("server override name cannot be empty")
	}

	if pso.Transport != "" {
		validTransports := map[string]bool{
			transport.TransportStdio: true,
			transport.TransportTCP:   true,
			transport.TransportHTTP:  true,
		}

		if !validTransports[pso.Transport] {
			return fmt.Errorf("invalid transport type: %s, must be one of: stdio, tcp, http", pso.Transport)
		}
	}

	return nil
}

func (c *GatewayConfig) GetServerByLanguage(language string) (*ServerConfig, error) {
	// First check if we have a language pool configured for this language
	if pool, err := c.GetServerPoolByLanguage(language); err == nil && pool != nil {
		// Return the default server from the pool if specified
		if pool.DefaultServer != "" {
			if defaultServer := pool.Servers[pool.DefaultServer]; defaultServer != nil {
				return defaultServer, nil
			}
		}
		// Return first available server from pool for backward compatibility
		for _, server := range pool.Servers {
			return server, nil
		}
	}

	// Fallback to original logic for non-pool configurations
	for _, server := range c.Servers {
		for _, lang := range server.Languages {
			if lang == language {
				return &server, nil
			}
		}
	}
	return nil, fmt.Errorf("no server found for language: %s", language)
}

func (c *GatewayConfig) GetServerByName(name string) (*ServerConfig, error) {
	for _, server := range c.Servers {
		if server.Name == name {
			return &server, nil
		}
	}
	return nil, fmt.Errorf("no server found with name: %s", name)
}

func (c *GatewayConfig) GetProjectAwareServers() []ServerConfig {
	if !c.ProjectAware || c.ProjectConfig == nil {
		return c.Servers
	}

	if len(c.ProjectConfig.EnabledServers) == 0 {
		return c.Servers
	}

	enabledMap := make(map[string]bool)
	for _, name := range c.ProjectConfig.EnabledServers {
		enabledMap[name] = true
	}

	var filteredServers []ServerConfig
	for _, server := range c.Servers {
		if enabledMap[server.Name] {
			filteredServers = append(filteredServers, server)
		}
	}

	return filteredServers
}

func (c *GatewayConfig) ApplyProjectOverrides() error {
	if !c.ProjectAware || c.ProjectConfig == nil {
		return nil
	}

	overridesMap := make(map[string]ProjectServerOverride)
	for _, override := range c.ProjectConfig.ServerOverrides {
		overridesMap[override.Name] = override
	}

	for i, server := range c.Servers {
		if override, exists := overridesMap[server.Name]; exists {
			if override.Enabled != nil && !*override.Enabled {
				continue
			}

			if len(override.Args) > 0 {
				c.Servers[i].Args = override.Args
			}

			if override.Transport != "" {
				c.Servers[i].Transport = override.Transport
			}

			if override.Settings != nil {
				c.Servers[i].Settings = override.Settings
			}
		}
	}

	return nil
}

func (c *GatewayConfig) GetRequiredLSPServers() []string {
	if c.ProjectContext == nil {
		return nil
	}
	return c.ProjectContext.RequiredLSPs
}

func (c *GatewayConfig) GetDetectedLanguages() []string {
	if c.ProjectContext == nil {
		return nil
	}

	var languages []string
	for _, lang := range c.ProjectContext.Languages {
		languages = append(languages, lang.Language)
	}
	return languages
}

func (c *GatewayConfig) IsProjectType(projectType string) bool {
	if c.ProjectContext == nil {
		return false
	}
	return c.ProjectContext.ProjectType == projectType
}

func (c *GatewayConfig) GetProjectRoot() string {
	if c.ProjectContext == nil {
		return ""
	}
	return c.ProjectContext.RootDirectory
}

func (c *GatewayConfig) GetWorkspaceRoot() string {
	if c.ProjectContext == nil {
		return ""
	}
	return c.ProjectContext.WorkspaceRoot
}

func (c *GatewayConfig) HasLanguage(language string) bool {
	if c.ProjectContext == nil {
		return false
	}

	for _, lang := range c.ProjectContext.Languages {
		if lang.Language == language {
			return true
		}
	}
	return false
}

func (c *GatewayConfig) GetLanguageFileCount(language string) int {
	if c.ProjectContext == nil {
		return 0
	}

	for _, lang := range c.ProjectContext.Languages {
		if lang.Language == language {
			return lang.FileCount
		}
	}
	return 0
}

func NewProjectContext(projectType, rootDir string) *ProjectContext {
	absRoot, _ := filepath.Abs(rootDir)
	return &ProjectContext{
		ProjectType:   projectType,
		RootDirectory: absRoot,
		Languages:     []LanguageInfo{},
		RequiredLSPs:  []string{},
		DetectedAt:    time.Now(),
		Metadata:      make(map[string]interface{}),
	}
}

func NewProjectConfig(projectID, rootDir string) *ProjectConfig {
	absRoot, _ := filepath.Abs(rootDir)
	return &ProjectConfig{
		ProjectID:       projectID,
		RootDirectory:   absRoot,
		ServerOverrides: []ProjectServerOverride{},
		EnabledServers:  []string{},
		Optimizations:   make(map[string]interface{}),
		GeneratedAt:     time.Now(),
	}
}

// Enhanced multi-language configuration structures

type ServerConstraints struct {
	MinFileCount    int      `yaml:"min_file_count,omitempty" json:"min_file_count,omitempty"`
	MaxFileCount    int      `yaml:"max_file_count,omitempty" json:"max_file_count,omitempty"`
	RequiredMarkers []string `yaml:"required_markers,omitempty" json:"required_markers,omitempty"`
	ExcludedMarkers []string `yaml:"excluded_markers,omitempty" json:"excluded_markers,omitempty"`
	ProjectTypes    []string `yaml:"project_types,omitempty" json:"project_types,omitempty"`
	MinVersion      string   `yaml:"min_version,omitempty" json:"min_version,omitempty"`
}

type MultiLanguageConfig struct {
	ProjectInfo     *MultiLanguageProjectInfo `yaml:"project_info" json:"project_info"`
	ServerConfigs   []*ServerConfig           `yaml:"servers" json:"servers"`
	WorkspaceConfig *WorkspaceConfig          `yaml:"workspace" json:"workspace"`
	OptimizedFor    string                    `yaml:"optimized_for" json:"optimized_for"` // "development", "production", "analysis"
	GeneratedAt     time.Time                 `yaml:"generated_at" json:"generated_at"`
	Version         string                    `yaml:"version" json:"version"`
	Metadata        map[string]interface{}    `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

type WorkspaceConfig struct {
	MultiRoot               bool                   `yaml:"multi_root" json:"multi_root"`
	LanguageRoots           map[string]string      `yaml:"language_roots" json:"language_roots"`
	SharedSettings          map[string]interface{} `yaml:"shared_settings,omitempty" json:"shared_settings,omitempty"`
	CrossLanguageReferences bool                   `yaml:"cross_language_references" json:"cross_language_references"`
	GlobalIgnores           []string               `yaml:"global_ignores,omitempty" json:"global_ignores,omitempty"`
	IndexingStrategy        string                 `yaml:"indexing_strategy,omitempty" json:"indexing_strategy,omitempty"`
}

type MultiLanguageProjectInfo struct {
	ProjectType      string                 `yaml:"project_type" json:"project_type"`
	RootDirectory    string                 `yaml:"root_directory" json:"root_directory"`
	WorkspaceRoot    string                 `yaml:"workspace_root,omitempty" json:"workspace_root,omitempty"`
	LanguageContexts []*LanguageContext     `yaml:"language_contexts" json:"language_contexts"`
	Frameworks       []*Framework           `yaml:"frameworks,omitempty" json:"frameworks,omitempty"`
	MonorepoLayout   *MonorepoLayout        `yaml:"monorepo_layout,omitempty" json:"monorepo_layout,omitempty"`
	DetectedAt       time.Time              `yaml:"detected_at" json:"detected_at"`
	Metadata         map[string]interface{} `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

type LanguageContext struct {
	Language       string              `yaml:"language" json:"language"`
	Version        string              `yaml:"version,omitempty" json:"version,omitempty"`
	FilePatterns   []string            `yaml:"file_patterns" json:"file_patterns"`
	FileCount      int                 `yaml:"file_count" json:"file_count"`
	RootMarkers    []string            `yaml:"root_markers" json:"root_markers"`
	RootPath       string              `yaml:"root_path" json:"root_path"`
	Submodules     []string            `yaml:"submodules,omitempty" json:"submodules,omitempty"`
	BuildSystem    string              `yaml:"build_system,omitempty" json:"build_system,omitempty"`
	PackageManager string              `yaml:"package_manager,omitempty" json:"package_manager,omitempty"`
	Frameworks     []string            `yaml:"frameworks,omitempty" json:"frameworks,omitempty"`
	TestFrameworks []string            `yaml:"test_frameworks,omitempty" json:"test_frameworks,omitempty"`
	LintingTools   []string            `yaml:"linting_tools,omitempty" json:"linting_tools,omitempty"`
	Complexity     *LanguageComplexity `yaml:"complexity,omitempty" json:"complexity,omitempty"`
}

type Framework struct {
	Name        string                 `yaml:"name" json:"name"`
	Version     string                 `yaml:"version,omitempty" json:"version,omitempty"`
	Language    string                 `yaml:"language" json:"language"`
	ConfigFiles []string               `yaml:"config_files,omitempty" json:"config_files,omitempty"`
	Features    []string               `yaml:"features,omitempty" json:"features,omitempty"`
	Settings    map[string]interface{} `yaml:"settings,omitempty" json:"settings,omitempty"`
}

type MonorepoLayout struct {
	Strategy        string            `yaml:"strategy" json:"strategy"` // "language-separated", "mixed", "microservices"
	Workspaces      []string          `yaml:"workspaces,omitempty" json:"workspaces,omitempty"`
	SharedLibraries []string          `yaml:"shared_libraries,omitempty" json:"shared_libraries,omitempty"`
	BuildRoots      map[string]string `yaml:"build_roots,omitempty" json:"build_roots,omitempty"`
}

type LanguageComplexity struct {
	LinesOfCode          int `yaml:"lines_of_code" json:"lines_of_code"`
	CyclomaticComplexity int `yaml:"cyclomatic_complexity,omitempty" json:"cyclomatic_complexity,omitempty"`
	DependencyCount      int `yaml:"dependency_count,omitempty" json:"dependency_count,omitempty"`
	NestingDepth         int `yaml:"nesting_depth,omitempty" json:"nesting_depth,omitempty"`
}

// Multi-server configuration structures

type MultiServerConfig struct {
	Primary             *ServerConfig   `yaml:"primary" json:"primary"`
	Secondary           []*ServerConfig `yaml:"secondary" json:"secondary"`
	SelectionStrategy   string          `yaml:"selection_strategy" json:"selection_strategy"` // "performance", "feature", "load_balance", "random"
	ConcurrentLimit     int             `yaml:"concurrent_limit" json:"concurrent_limit"`
	ResourceSharing     bool            `yaml:"resource_sharing" json:"resource_sharing"`
	HealthCheckInterval time.Duration   `yaml:"health_check_interval" json:"health_check_interval"`
	MaxRetries          int             `yaml:"max_retries" json:"max_retries"`
}

type LanguageServerPool struct {
	Language            string                   `yaml:"language" json:"language"`
	Servers             map[string]*ServerConfig `yaml:"servers" json:"servers"`
	DefaultServer       string                   `yaml:"default_server" json:"default_server"`
	LoadBalancingConfig *LoadBalancingConfig     `yaml:"load_balancing" json:"load_balancing"`
	ResourceLimits      *ResourceLimits          `yaml:"resource_limits" json:"resource_limits"`
}

type LoadBalancingConfig struct {
	Strategy        string             `yaml:"strategy" json:"strategy"` // "round_robin", "least_connections", "response_time", "resource_usage"
	HealthThreshold float64            `yaml:"health_threshold" json:"health_threshold"`
	WeightFactors   map[string]float64 `yaml:"weight_factors" json:"weight_factors"`
	FallbackEnabled bool               `yaml:"fallback_enabled" json:"fallback_enabled"`
	MaxRetries      int                `yaml:"max_retries" json:"max_retries"`
	RetryDelay      time.Duration      `yaml:"retry_delay" json:"retry_delay"`
}

type ResourceLimits struct {
	MaxMemoryMB           int64 `yaml:"max_memory_mb" json:"max_memory_mb"`
	MaxConcurrentRequests int   `yaml:"max_concurrent_requests" json:"max_concurrent_requests"`
	MaxProcesses          int   `yaml:"max_processes" json:"max_processes"`
	RequestTimeoutSeconds int   `yaml:"request_timeout_seconds" json:"request_timeout_seconds"`
}

// ServerBypassConfig contains bypass configuration for a specific server
type ServerBypassConfig struct {
	Enabled               bool                    `yaml:"enabled" json:"enabled"`
	BypassConditions      []string                `yaml:"bypass_conditions,omitempty" json:"bypass_conditions,omitempty"`
	BypassStrategy        string                  `yaml:"bypass_strategy,omitempty" json:"bypass_strategy,omitempty"`
	FallbackServer        string                  `yaml:"fallback_server,omitempty" json:"fallback_server,omitempty"`
	RecoveryAttempts      int                     `yaml:"recovery_attempts,omitempty" json:"recovery_attempts,omitempty"`
	HealthThreshold       float64                 `yaml:"health_threshold,omitempty" json:"health_threshold,omitempty"`
	CooldownPeriod        string                  `yaml:"cooldown_period,omitempty" json:"cooldown_period,omitempty"`
	Timeouts              *BypassTimeouts         `yaml:"timeouts,omitempty" json:"timeouts,omitempty"`
	FailureThresholds     *BypassFailureThresholds `yaml:"failure_thresholds,omitempty" json:"failure_thresholds,omitempty"`
	MethodConfigs         map[string]*BypassMethodConfig `yaml:"method_configs,omitempty" json:"method_configs,omitempty"`
	CircuitBreakerConfig  *BypassCircuitBreakerConfig    `yaml:"circuit_breaker,omitempty" json:"circuit_breaker,omitempty"`
	NotificationConfig    *BypassNotificationConfig      `yaml:"notifications,omitempty" json:"notifications,omitempty"`
}

// BypassTimeouts contains timeout configurations for bypass functionality
type BypassTimeouts struct {
	Startup       string `yaml:"startup,omitempty" json:"startup,omitempty"`
	Request       string `yaml:"request,omitempty" json:"request,omitempty"`
	WorkspaceInit string `yaml:"workspace_init,omitempty" json:"workspace_init,omitempty"`
	Shutdown      string `yaml:"shutdown,omitempty" json:"shutdown,omitempty"`
}

// BypassFailureThresholds contains failure threshold configurations
type BypassFailureThresholds struct {
	ConsecutiveFailures int     `yaml:"consecutive_failures,omitempty" json:"consecutive_failures,omitempty"`
	ErrorRatePercent    int     `yaml:"error_rate_percent,omitempty" json:"error_rate_percent,omitempty"`
	ResponseTimeMs      int     `yaml:"response_time_ms,omitempty" json:"response_time_ms,omitempty"`
	MemoryUsageMB       int     `yaml:"memory_usage_mb,omitempty" json:"memory_usage_mb,omitempty"`
	CPUUsagePercent     float64 `yaml:"cpu_usage_percent,omitempty" json:"cpu_usage_percent,omitempty"`
}

// BypassMethodConfig contains method-specific bypass configuration
type BypassMethodConfig struct {
	Timeout          string                 `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	BypassStrategy   string                 `yaml:"bypass_strategy,omitempty" json:"bypass_strategy,omitempty"`
	CacheTTL         string                 `yaml:"cache_ttl,omitempty" json:"cache_ttl,omitempty"`
	FallbackEnabled  bool                   `yaml:"fallback_enabled,omitempty" json:"fallback_enabled,omitempty"`
	FallbackResponse map[string]interface{} `yaml:"fallback_response,omitempty" json:"fallback_response,omitempty"`
	ServeStale       bool                   `yaml:"serve_stale,omitempty" json:"serve_stale,omitempty"`
	MaxResults       int                    `yaml:"max_results,omitempty" json:"max_results,omitempty"`
}

// BypassCircuitBreakerConfig contains circuit breaker configuration for bypass
type BypassCircuitBreakerConfig struct {
	Enabled            bool   `yaml:"enabled" json:"enabled"`
	FailureThreshold   int    `yaml:"failure_threshold,omitempty" json:"failure_threshold,omitempty"`
	SuccessThreshold   int    `yaml:"success_threshold,omitempty" json:"success_threshold,omitempty"`
	Timeout            string `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	FailureWindowSize  int    `yaml:"failure_window_size,omitempty" json:"failure_window_size,omitempty"`
}

// BypassNotificationConfig contains notification configuration for bypass events
type BypassNotificationConfig struct {
	Enabled  bool     `yaml:"enabled" json:"enabled"`
	Levels   []string `yaml:"levels,omitempty" json:"levels,omitempty"`
	Channels struct {
		Console bool `yaml:"console,omitempty" json:"console,omitempty"`
		LogFile bool `yaml:"log_file,omitempty" json:"log_file,omitempty"`
		Webhook bool `yaml:"webhook,omitempty" json:"webhook,omitempty"`
	} `yaml:"channels,omitempty" json:"channels,omitempty"`
}

// LanguageBypassConfig contains language-specific bypass configuration
type LanguageBypassConfig struct {
	Language              string                      `yaml:"language" json:"language"`
	Strategy              string                      `yaml:"strategy,omitempty" json:"strategy,omitempty"`
	Conditions            []string                    `yaml:"conditions,omitempty" json:"conditions,omitempty"`
	Timeout               string                      `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	PerformanceThresholds *LanguagePerformanceThresholds `yaml:"performance_thresholds,omitempty" json:"performance_thresholds,omitempty"`
	Recovery              *LanguageRecoveryConfig     `yaml:"recovery,omitempty" json:"recovery,omitempty"`
}

// LanguagePerformanceThresholds contains performance thresholds for language-specific bypass
type LanguagePerformanceThresholds struct {
	MaxResponseTime string  `yaml:"max_response_time,omitempty" json:"max_response_time,omitempty"`
	MaxMemoryUsage  string  `yaml:"max_memory_usage,omitempty" json:"max_memory_usage,omitempty"`
	MaxCPUUsage     float64 `yaml:"max_cpu_usage,omitempty" json:"max_cpu_usage,omitempty"`
}

// LanguageRecoveryConfig contains recovery configuration for language-specific bypass
type LanguageRecoveryConfig struct {
	Enabled     bool   `yaml:"enabled" json:"enabled"`
	MaxAttempts int    `yaml:"max_attempts,omitempty" json:"max_attempts,omitempty"`
	Cooldown    string `yaml:"cooldown,omitempty" json:"cooldown,omitempty"`
	HealthCheckMethod string `yaml:"health_check_method,omitempty" json:"health_check_method,omitempty"`
}

// ProjectPatternConfig contains project pattern-specific configuration
type ProjectPatternConfig struct {
	Patterns      []string `yaml:"patterns,omitempty" json:"patterns,omitempty"`
	Optimizations []string `yaml:"optimizations,omitempty" json:"optimizations,omitempty"`
	ServerInstances int    `yaml:"server_instances,omitempty" json:"server_instances,omitempty"`
	MemoryLimit     string `yaml:"memory_limit,omitempty" json:"memory_limit,omitempty"`
	ConcurrentRequests int `yaml:"concurrent_requests,omitempty" json:"concurrent_requests,omitempty"`
}

// GlobalBypassConfig contains global bypass configuration settings
type GlobalBypassConfig struct {
	EnableGlobalBypass          bool   `yaml:"enable_global_bypass" json:"enable_global_bypass"`
	AutoBypassOnConsecutive     bool   `yaml:"auto_bypass_on_consecutive" json:"auto_bypass_on_consecutive"`
	AutoBypassOnCircuitBreaker  bool   `yaml:"auto_bypass_on_circuit_breaker" json:"auto_bypass_on_circuit_breaker"`
	AutoBypassOnHealthDegraded  bool   `yaml:"auto_bypass_on_health_degraded" json:"auto_bypass_on_health_degraded"`
	ConsecutiveFailureThreshold int    `yaml:"consecutive_failure_threshold" json:"consecutive_failure_threshold"`
	RecoveryCheckInterval       string `yaml:"recovery_check_interval" json:"recovery_check_interval"`
	MaxRecoveryAttempts         int    `yaml:"max_recovery_attempts" json:"max_recovery_attempts"`
	UserConfirmationRequired    bool   `yaml:"user_confirmation_required" json:"user_confirmation_required"`
}

// BypassMonitoringConfig contains monitoring configuration for bypass events
type BypassMonitoringConfig struct {
	Enabled         bool                    `yaml:"enabled" json:"enabled"`
	RetentionPeriod string                  `yaml:"retention_period,omitempty" json:"retention_period,omitempty"`
	Export          *BypassExportConfig     `yaml:"export,omitempty" json:"export,omitempty"`
}

// BypassExportConfig contains export configuration for bypass metrics
type BypassExportConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	Format   string `yaml:"format,omitempty" json:"format,omitempty"`
	Endpoint string `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
}

// BypassAdvancedConfig contains advanced bypass features configuration
type BypassAdvancedConfig struct {
	PredictiveBypass      bool `yaml:"predictive_bypass,omitempty" json:"predictive_bypass,omitempty"`
	MLOptimization        bool `yaml:"ml_optimization,omitempty" json:"ml_optimization,omitempty"`
	RecommendationSystem  bool `yaml:"recommendation_system,omitempty" json:"recommendation_system,omitempty"`
	AuditTrail            bool `yaml:"audit_trail,omitempty" json:"audit_trail,omitempty"`
}

// BypassConfiguration contains comprehensive bypass configuration for the LSP Gateway
type BypassConfiguration struct {
	// Enable bypass functionality globally
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Version string `yaml:"version,omitempty" json:"version,omitempty"`
	
	// Global bypass settings
	GlobalBypass *GlobalBypassConfig `yaml:"global_bypass,omitempty" json:"global_bypass,omitempty"`
	
	// Server-specific bypass configurations
	ServerBypass []*ServerBypassConfig `yaml:"server_bypass,omitempty" json:"server_bypass,omitempty"`
	
	// Language-specific bypass configurations
	LanguageBypass map[string]*LanguageBypassConfig `yaml:"language_bypass,omitempty" json:"language_bypass,omitempty"`
	
	// Circuit breaker configuration
	CircuitBreaker *BypassCircuitBreakerConfig `yaml:"circuit_breaker,omitempty" json:"circuit_breaker,omitempty"`
	
	// Notification settings
	Notifications *BypassNotificationConfig `yaml:"notifications,omitempty" json:"notifications,omitempty"`
	
	// Monitoring and metrics
	Monitoring *BypassMonitoringConfig `yaml:"monitoring,omitempty" json:"monitoring,omitempty"`
	
	// Advanced features
	Advanced *BypassAdvancedConfig `yaml:"advanced,omitempty" json:"advanced,omitempty"`
}

// Validate validates the bypass configuration
func (bc *BypassConfiguration) Validate() error {
	if bc == nil {
		return nil // Optional configuration
	}
	
	// Validate global bypass configuration
	if bc.GlobalBypass != nil {
		if err := bc.validateGlobalBypass(); err != nil {
			return fmt.Errorf("global bypass configuration invalid: %w", err)
		}
	}
	
	// Validate server-specific configurations
	for i, serverConfig := range bc.ServerBypass {
		if err := bc.validateServerBypass(serverConfig); err != nil {
			return fmt.Errorf("server bypass configuration [%d] invalid: %w", i, err)
		}
	}
	
	// Validate language-specific configurations
	for language, langConfig := range bc.LanguageBypass {
		if err := bc.validateLanguageBypass(langConfig); err != nil {
			return fmt.Errorf("language bypass configuration for %s invalid: %w", language, err)
		}
	}
	
	return nil
}

// validateGlobalBypass validates global bypass settings
func (bc *BypassConfiguration) validateGlobalBypass() error {
	gb := bc.GlobalBypass
	if gb.ConsecutiveFailureThreshold < 1 {
		return fmt.Errorf("consecutive failure threshold must be at least 1")
	}
	if gb.MaxRecoveryAttempts < 0 {
		return fmt.Errorf("max recovery attempts cannot be negative")
	}
	return nil
}

// validateServerBypass validates server-specific bypass configuration
func (bc *BypassConfiguration) validateServerBypass(sc *ServerBypassConfig) error {
	if sc == nil {
		return fmt.Errorf("server bypass configuration cannot be nil")
	}
	
	// Validate bypass strategies
	validStrategies := map[string]bool{
		"fail_gracefully":   true,
		"fallback_server":   true,
		"cache_response":    true,
		"circuit_breaker":   true,
		"retry_with_backoff": true,
	}
	
	if sc.BypassStrategy != "" && !validStrategies[sc.BypassStrategy] {
		return fmt.Errorf("invalid bypass strategy: %s", sc.BypassStrategy)
	}
	
	// Validate health threshold
	if sc.HealthThreshold < 0.0 || sc.HealthThreshold > 1.0 {
		return fmt.Errorf("health threshold must be between 0.0 and 1.0")
	}
	
	return nil
}

// validateLanguageBypass validates language-specific bypass configuration
func (bc *BypassConfiguration) validateLanguageBypass(lc *LanguageBypassConfig) error {
	if lc == nil {
		return fmt.Errorf("language bypass configuration cannot be nil")
	}
	
	// Validate performance thresholds
	if lc.PerformanceThresholds != nil {
		if lc.PerformanceThresholds.MaxCPUUsage < 0 || lc.PerformanceThresholds.MaxCPUUsage > 100 {
			return fmt.Errorf("max CPU usage must be between 0 and 100")
		}
	}
	
	return nil
}

// DefaultBypassConfiguration returns a default bypass configuration
func DefaultBypassConfiguration() *BypassConfiguration {
	return &BypassConfiguration{
		Enabled: true,
		Version: "1.0",
		GlobalBypass: &GlobalBypassConfig{
			EnableGlobalBypass:          true,
			AutoBypassOnConsecutive:     true,
			AutoBypassOnCircuitBreaker:  true,
			AutoBypassOnHealthDegraded:  false,
			ConsecutiveFailureThreshold: 3,
			RecoveryCheckInterval:       "5m",
			MaxRecoveryAttempts:         3,
			UserConfirmationRequired:    false,
		},
		CircuitBreaker: &BypassCircuitBreakerConfig{
			Enabled:           true,
			FailureThreshold:  5,
			SuccessThreshold:  3,
			Timeout:           "30s",
			FailureWindowSize: 10,
		},
		Notifications: &BypassNotificationConfig{
			Enabled: true,
			Levels:  []string{"warning", "error", "critical"},
		},
		Monitoring: &BypassMonitoringConfig{
			Enabled:         true,
			RetentionPeriod: "24h",
		},
		Advanced: &BypassAdvancedConfig{
			PredictiveBypass:     false,
			MLOptimization:       false,
			RecommendationSystem: true,
			AuditTrail:          true,
		},
	}
}

// Validate validates the gateway configuration including bypass settings
func (gc *GatewayConfig) Validate() error {
	if gc == nil {
		return fmt.Errorf("gateway configuration cannot be nil")
	}
	
	// Validate basic configuration
	if gc.Port <= 0 || gc.Port > 65535 {
		return fmt.Errorf("invalid port: %d (must be between 1 and 65535)", gc.Port)
	}
	
	if len(gc.Servers) == 0 {
		return fmt.Errorf("at least one server must be configured")
	}
	
	// Validate server configurations
	for i, server := range gc.Servers {
		if server.Name == "" {
			return fmt.Errorf("server[%d] name cannot be empty", i)
		}
		if server.Command == "" {
			return fmt.Errorf("server[%d] command cannot be empty", i)
		}
		if len(server.Languages) == 0 {
			return fmt.Errorf("server[%d] must support at least one language", i)
		}
	}
	
	// Validate bypass configuration if present
	if gc.BypassConfig != nil {
		if err := gc.BypassConfig.Validate(); err != nil {
			return fmt.Errorf("bypass configuration validation failed: %w", err)
		}
	}
	
	return nil
}

// Multi-server configuration methods

// GetServerPoolByLanguage returns the server pool for a specific language
func (c *GatewayConfig) GetServerPoolByLanguage(language string) (*LanguageServerPool, error) {
	for i, pool := range c.LanguagePools {
		if pool.Language == language {
			return &c.LanguagePools[i], nil
		}
	}
	return nil, fmt.Errorf("no server pool found for language: %s", language)
}

// GetServersForLanguage returns multiple servers for a language based on strategy
func (c *GatewayConfig) GetServersForLanguage(language string, maxServers int) ([]*ServerConfig, error) {
	if pool, err := c.GetServerPoolByLanguage(language); err == nil && pool != nil {
		var servers []*ServerConfig
		count := 0

		// Add default server first if configured
		if pool.DefaultServer != "" {
			if defaultServer := pool.Servers[pool.DefaultServer]; defaultServer != nil {
				servers = append(servers, defaultServer)
				count++
			}
		}

		// Add additional servers up to maxServers limit
		for name, server := range pool.Servers {
			if count >= maxServers {
				break
			}
			if name != pool.DefaultServer { // Skip default server if already added
				servers = append(servers, server)
				count++
			}
		}

		if len(servers) > 0 {
			return servers, nil
		}
	}

	// Fallback to single server from original servers list
	if server, err := c.GetServerByLanguage(language); err == nil {
		return []*ServerConfig{server}, nil
	}

	return nil, fmt.Errorf("no servers found for language: %s", language)
}

// GetServerPoolWithConfig returns server pool with load balancing configuration
func (c *GatewayConfig) GetServerPoolWithConfig(language string) (*LanguageServerPool, *LoadBalancingConfig, error) {
	pool, err := c.GetServerPoolByLanguage(language)
	if err != nil {
		return nil, nil, err
	}

	return pool, pool.LoadBalancingConfig, nil
}

// IsMultiServerEnabled checks if multi-server mode is enabled for language
func (c *GatewayConfig) IsMultiServerEnabled(language string) bool {
	if !c.EnableConcurrentServers {
		return false
	}

	pool, err := c.GetServerPoolByLanguage(language)
	if err != nil {
		return false
	}

	return len(pool.Servers) > 1
}

// GetResourceLimits returns resource limits for language server pool
func (c *GatewayConfig) GetResourceLimits(language string) *ResourceLimits {
	if pool, err := c.GetServerPoolByLanguage(language); err == nil && pool != nil {
		return pool.ResourceLimits
	}
	return nil
}

// Multi-language configuration integration methods

// ToGatewayConfig converts MultiLanguageConfig to GatewayConfig for backward compatibility
func (mlc *MultiLanguageConfig) ToGatewayConfig() (*GatewayConfig, error) {
	if mlc == nil {
		return nil, fmt.Errorf("multi-language config cannot be nil")
	}

	config := &GatewayConfig{
		Port:                            8080,
		Timeout:                         "30s",
		MaxConcurrentRequests:           100,
		ProjectAware:                    true,
		EnableConcurrentServers:         len(mlc.ServerConfigs) > 1,
		MaxConcurrentServersPerLanguage: DEFAULT_MAX_CONCURRENT_SERVERS_PER_LANG,
		Servers:                         make([]ServerConfig, len(mlc.ServerConfigs)),
		LanguagePools:                   []LanguageServerPool{},
		GlobalMultiServerConfig:         DefaultMultiServerConfig(),
	}

	// Convert server configurations
	for i, serverConfig := range mlc.ServerConfigs {
		if serverConfig != nil {
			config.Servers[i] = *serverConfig
		}
	}

	// Convert project info to project context
	if mlc.ProjectInfo != nil {
		projectContext := &ProjectContext{
			ProjectType:   mlc.ProjectInfo.ProjectType,
			RootDirectory: mlc.ProjectInfo.RootDirectory,
			WorkspaceRoot: mlc.ProjectInfo.WorkspaceRoot,
			Languages:     make([]LanguageInfo, len(mlc.ProjectInfo.LanguageContexts)),
			RequiredLSPs:  []string{},
			DetectedAt:    mlc.ProjectInfo.DetectedAt,
			Metadata:      mlc.ProjectInfo.Metadata,
		}

		// Convert language contexts to language info
		for i, langCtx := range mlc.ProjectInfo.LanguageContexts {
			if langCtx != nil {
				projectContext.Languages[i] = LanguageInfo{
					Language:     langCtx.Language,
					FilePatterns: langCtx.FilePatterns,
					FileCount:    langCtx.FileCount,
					RootMarkers:  langCtx.RootMarkers,
				}
			}
		}

		// Extract required LSP servers from server configs
		for _, serverConfig := range mlc.ServerConfigs {
			if serverConfig != nil {
				projectContext.RequiredLSPs = append(projectContext.RequiredLSPs, serverConfig.Name)
			}
		}

		config.ProjectContext = projectContext
	}

	// Set optimization-specific defaults
	switch mlc.OptimizedFor {
	case PerformanceProfileProduction:
		config.MaxConcurrentRequests = 200
		config.Timeout = "15s"
	case PerformanceProfileAnalysis:
		config.MaxConcurrentRequests = 50
		config.Timeout = "60s"
	default: // Development
		config.MaxConcurrentRequests = 100
		config.Timeout = "30s"
	}

	// Create language pools if multiple servers for same language
	languageServerMap := make(map[string][]*ServerConfig)
	for _, serverConfig := range mlc.ServerConfigs {
		if serverConfig != nil {
			for _, language := range serverConfig.Languages {
				languageServerMap[language] = append(languageServerMap[language], serverConfig)
			}
		}
	}

	for language, servers := range languageServerMap {
		if len(servers) > 1 {
			pool := CreateLanguageServerPool(language)
			for _, server := range servers {
				if err := pool.AddServerToPool(server); err != nil {
					return nil, fmt.Errorf("failed to add server %s to pool for language %s: %w", server.Name, language, err)
				}
			}
			config.LanguagePools = append(config.LanguagePools, *pool)
		}
	}

	config.EnsureMultiServerDefaults()

	return config, nil
}

// EnhanceWithMultiLanguage updates existing GatewayConfig with multi-language support
func (gc *GatewayConfig) EnhanceWithMultiLanguage(mlConfig *MultiLanguageConfig) error {
	if mlConfig == nil {
		return fmt.Errorf("multi-language config cannot be nil")
	}

	// Update project context if available
	if mlConfig.ProjectInfo != nil {
		if gc.ProjectContext == nil {
			gc.ProjectContext = &ProjectContext{}
		}

		gc.ProjectContext.ProjectType = mlConfig.ProjectInfo.ProjectType
		gc.ProjectContext.RootDirectory = mlConfig.ProjectInfo.RootDirectory
		gc.ProjectContext.WorkspaceRoot = mlConfig.ProjectInfo.WorkspaceRoot
		gc.ProjectContext.DetectedAt = mlConfig.ProjectInfo.DetectedAt
		gc.ProjectContext.Metadata = mlConfig.ProjectInfo.Metadata

		// Convert language contexts
		gc.ProjectContext.Languages = make([]LanguageInfo, len(mlConfig.ProjectInfo.LanguageContexts))
		for i, langCtx := range mlConfig.ProjectInfo.LanguageContexts {
			if langCtx != nil {
				// Map detected language to LSP server language for validation
				mappedLanguage := mapDetectedLanguageToLSPLanguage(langCtx.Language)
				
				gc.ProjectContext.Languages[i] = LanguageInfo{
					Language:     mappedLanguage,
					FilePatterns: langCtx.FilePatterns,
					FileCount:    langCtx.FileCount,
					RootMarkers:  langCtx.RootMarkers,
				}
			}
		}
	}

	// Update server configurations
	if len(mlConfig.ServerConfigs) > 0 {
		gc.Servers = make([]ServerConfig, len(mlConfig.ServerConfigs))
		for i, serverConfig := range mlConfig.ServerConfigs {
			if serverConfig != nil {
				gc.Servers[i] = *serverConfig
			}
		}
	}

	// Enable project awareness and concurrent servers if beneficial
	gc.ProjectAware = true
	if len(mlConfig.ServerConfigs) > 1 {
		gc.EnableConcurrentServers = true
	}

	// Apply optimization-specific settings
	switch mlConfig.OptimizedFor {
	case PerformanceProfileProduction:
		gc.MaxConcurrentRequests = 200
		gc.Timeout = "15s"
	case PerformanceProfileAnalysis:
		gc.MaxConcurrentRequests = 50
		gc.Timeout = "60s"
	default:
		gc.MaxConcurrentRequests = 100
		gc.Timeout = "30s"
	}

	return nil
}

// AutoGenerateConfig automatically generates multi-language configuration from project path
func AutoGenerateConfig(projectPath string) (*MultiLanguageConfig, error) {
	// This would typically integrate with project detection logic
	// For now, return a basic implementation
	generator := NewConfigGenerator()

	// Create mock project info for demonstration
	// In real implementation, this would use project detection
	projectInfo := &MultiLanguageProjectInfo{
		ProjectType:   ProjectTypeMulti,
		RootDirectory: projectPath,
		LanguageContexts: []*LanguageContext{
			{
				Language:     "go",
				FilePatterns: []string{"*.go"},
				FileCount:    10,
				RootMarkers:  []string{"go.mod"},
				RootPath:     projectPath,
			},
		},
		DetectedAt: time.Now(),
		Metadata:   make(map[string]interface{}),
	}

	return generator.GenerateMultiLanguageConfig(projectInfo)
}

// Configuration file I/O methods - implementations in multi_language.go

// NOTE: Full implementations of WriteYAML, WriteJSON, and LoadMultiLanguageConfig
// are available in multi_language.go. These methods provide complete file I/O
// functionality with proper error handling and format detection.

// Configuration integration methods

// LoadConfigurationWithMigration loads configuration with automatic migration support
func LoadConfigurationWithMigration(configPath string) (*MultiLanguageConfig, error) {
	integrator := NewConfigurationIntegrator()
	return integrator.MigrateConfiguration(configPath)
}

// GenerateEnhancedConfigFromPath generates an enhanced configuration from a project path
func GenerateEnhancedConfigFromPath(projectPath, optimizationMode string) (*MultiLanguageConfig, error) {
	integrator := NewConfigurationIntegrator()
	return integrator.GenerateEnhancedConfiguration(projectPath, optimizationMode)
}

// IntegrateMultipleConfigs integrates multiple configuration sources
func IntegrateMultipleConfigs(configs ...*MultiLanguageConfig) (*MultiLanguageConfig, error) {
	integrator := NewConfigurationIntegrator()
	return integrator.IntegrateConfigurations(configs...)
}

// Configuration optimization methods

// OptimizeForPerformance optimizes the configuration for performance
func (mlc *MultiLanguageConfig) OptimizeForPerformance() error {
	productionOpt := NewProductionOptimization()
	return productionOpt.ApplyOptimizations(mlc)
}

// OptimizeForAccuracy optimizes the configuration for accuracy
func (mlc *MultiLanguageConfig) OptimizeForAccuracy() error {
	analysisOpt := NewAnalysisOptimization()
	return analysisOpt.ApplyOptimizations(mlc)
}

// OptimizeForDevelopment optimizes the configuration for development
func (mlc *MultiLanguageConfig) OptimizeForDevelopment() error {
	devOpt := NewDevelopmentOptimization()
	return devOpt.ApplyOptimizations(mlc)
}

// ApplyOptimizationStrategy applies a specific optimization strategy
func (mlc *MultiLanguageConfig) ApplyOptimizationStrategy(strategyName string) error {
	manager := NewOptimizationManager()
	return manager.ApplyOptimization(mlc, strategyName)
}

// ResolveServerConflicts resolves conflicts between multiple servers for the same language
func (mlc *MultiLanguageConfig) ResolveServerConflicts() error {
	languageServerMap := make(map[string][]*ServerConfig)

	// Group servers by language
	for _, serverConfig := range mlc.ServerConfigs {
		if serverConfig != nil {
			for _, language := range serverConfig.Languages {
				languageServerMap[language] = append(languageServerMap[language], serverConfig)
			}
		}
	}

	// Resolve conflicts by priority and weight
	for _, servers := range languageServerMap {
		if len(servers) > 1 {
			// Sort by priority (descending), then by weight (descending)
			sort.Slice(servers, func(i, j int) bool {
				if servers[i].Priority != servers[j].Priority {
					return servers[i].Priority > servers[j].Priority
				}
				return servers[i].Weight > servers[j].Weight
			})

			// Keep the highest priority server, mark others as secondary
			for i := 1; i < len(servers); i++ {
				servers[i].Priority = servers[0].Priority - i
				servers[i].Weight = servers[0].Weight * 0.8 // Reduce weight for secondary servers
			}
		}
	}

	return nil
}

// Helper methods for configuration management

// GetServerConfigByLanguage returns the server configuration for a specific language
func (mlc *MultiLanguageConfig) GetServerConfigByLanguage(language string) (*ServerConfig, error) {
	for _, serverConfig := range mlc.ServerConfigs {
		if serverConfig != nil {
			for _, lang := range serverConfig.Languages {
				if lang == language {
					return serverConfig, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("no server configuration found for language: %s", language)
}

// GetLanguageContextByLanguage returns the language context for a specific language
func (mlc *MultiLanguageConfig) GetLanguageContextByLanguage(language string) (*LanguageContext, error) {
	if mlc.ProjectInfo == nil {
		return nil, fmt.Errorf("project info is nil")
	}

	for _, langCtx := range mlc.ProjectInfo.LanguageContexts {
		if langCtx != nil && langCtx.Language == language {
			return langCtx, nil
		}
	}
	return nil, fmt.Errorf("no language context found for language: %s", language)
}

// GetSupportedLanguages returns all languages supported by this configuration
func (mlc *MultiLanguageConfig) GetSupportedLanguages() []string {
	languages := make(map[string]bool)

	for _, serverConfig := range mlc.ServerConfigs {
		if serverConfig != nil {
			for _, lang := range serverConfig.Languages {
				languages[lang] = true
			}
		}
	}

	var result []string
	for lang := range languages {
		result = append(result, lang)
	}

	sort.Strings(result)
	return result
}

// GetFrameworksByLanguage returns frameworks for a specific language
func (mlc *MultiLanguageConfig) GetFrameworksByLanguage(language string) []*Framework {
	if mlc.ProjectInfo == nil {
		return nil
	}

	var frameworks []*Framework
	for _, framework := range mlc.ProjectInfo.Frameworks {
		if framework != nil && framework.Language == language {
			frameworks = append(frameworks, framework)
		}
	}

	return frameworks
}

// IsMonorepo returns true if this is a monorepo configuration
func (mlc *MultiLanguageConfig) IsMonorepo() bool {
	if mlc.ProjectInfo == nil {
		return false
	}
	return mlc.ProjectInfo.ProjectType == ProjectTypeMonorepo
}

// HasFramework returns true if the specified framework is configured
func (mlc *MultiLanguageConfig) HasFramework(frameworkName string) bool {
	if mlc.ProjectInfo == nil {
		return false
	}

	for _, framework := range mlc.ProjectInfo.Frameworks {
		if framework != nil && framework.Name == frameworkName {
			return true
		}
	}

	return false
}

// GetComplexityMetrics returns aggregated complexity metrics
func (mlc *MultiLanguageConfig) GetComplexityMetrics() map[string]*LanguageComplexity {
	if mlc.ProjectInfo == nil {
		return nil
	}

	metrics := make(map[string]*LanguageComplexity)
	for _, langCtx := range mlc.ProjectInfo.LanguageContexts {
		if langCtx != nil && langCtx.Complexity != nil {
			metrics[langCtx.Language] = langCtx.Complexity
		}
	}

	return metrics
}

// Performance configuration integration methods

// GetPerformanceConfig returns the performance configuration, creating default if nil
func (c *GatewayConfig) GetPerformanceConfig() *PerformanceConfiguration {
	if c.PerformanceConfig == nil {
		c.PerformanceConfig = DefaultPerformanceConfiguration()
	}
	return c.PerformanceConfig
}

// EnablePerformanceOptimizations enables performance optimizations
func (c *GatewayConfig) EnablePerformanceOptimizations(profile string) error {
	perfConfig := c.GetPerformanceConfig()
	perfConfig.Enabled = true

	// Optimize for the specified profile
	if err := perfConfig.OptimizeForProfile(profile); err != nil {
		return fmt.Errorf("failed to optimize for profile %s: %w", profile, err)
	}

	// Apply environment defaults
	if err := perfConfig.ApplyEnvironmentDefaults(); err != nil {
		return fmt.Errorf("failed to apply environment defaults: %w", err)
	}

	return nil
}

// ApplyPerformanceOverrides applies performance configuration overrides
func (c *GatewayConfig) ApplyPerformanceOverrides(overrides map[string]interface{}) error {
	if c.PerformanceConfig == nil {
		c.PerformanceConfig = DefaultPerformanceConfiguration()
	}

	// Apply timeout overrides
	if timeoutOverrides, ok := overrides["timeouts"].(map[string]interface{}); ok {
		if globalTimeout, exists := timeoutOverrides["global_timeout"].(string); exists {
			if duration, err := time.ParseDuration(globalTimeout); err == nil {
				c.PerformanceConfig.Timeouts.GlobalTimeout = duration
			}
		}
	}

	// Apply cache overrides
	if cacheOverrides, ok := overrides["caching"].(map[string]interface{}); ok {
		if enabled, exists := cacheOverrides["enabled"].(bool); exists {
			c.PerformanceConfig.Caching.Enabled = enabled
		}
		if maxMemory, exists := cacheOverrides["max_memory_usage_mb"].(int64); exists {
			c.PerformanceConfig.Caching.MaxMemoryUsage = maxMemory
		}
	}

	// Apply memory overrides
	if memoryOverrides, ok := overrides["memory"].(map[string]interface{}); ok {
		if maxHeap, exists := memoryOverrides["max_heap_size_mb"].(int64); exists {
			c.PerformanceConfig.ResourceManager.MemoryLimits.MaxHeapSize = maxHeap
		}
	}

	return nil
}

// GetEffectiveTimeout returns the effective timeout for a given method and language
func (c *GatewayConfig) GetEffectiveTimeout(method, language string) time.Duration {
	if c.PerformanceConfig == nil || c.PerformanceConfig.Timeouts == nil {
		// Fallback to legacy timeout
		if duration, err := time.ParseDuration(c.Timeout); err == nil {
			return duration
		}
		return 30 * time.Second
	}

	timeouts := c.PerformanceConfig.Timeouts

	// Check method-specific timeout first
	if methodTimeout, exists := timeouts.MethodTimeouts[method]; exists {
		return methodTimeout
	}

	// Check language-specific timeout
	if langTimeout, exists := timeouts.LanguageTimeouts[language]; exists {
		return langTimeout
	}

	// Return default timeout
	if timeouts.DefaultTimeout > 0 {
		return timeouts.DefaultTimeout
	}

	return timeouts.GlobalTimeout
}

// GetEffectiveMemoryLimit returns the effective memory limit for a server
func (c *GatewayConfig) GetEffectiveMemoryLimit(serverName string) int64 {
	if c.PerformanceConfig == nil || c.PerformanceConfig.ResourceManager == nil ||
		c.PerformanceConfig.ResourceManager.MemoryLimits == nil {
		return DefaultMemoryLimit
	}

	memLimits := c.PerformanceConfig.ResourceManager.MemoryLimits

	// Return per-server limit if configured
	if memLimits.PerServerLimit > 0 {
		return memLimits.PerServerLimit
	}

	// Return soft limit as default
	if memLimits.SoftLimit > 0 {
		return memLimits.SoftLimit
	}

	return memLimits.MaxHeapSize
}

// IsCacheEnabled returns whether caching is enabled for a specific cache type
func (c *GatewayConfig) IsCacheEnabled(cacheType string) bool {
	if c.PerformanceConfig == nil || c.PerformanceConfig.Caching == nil {
		return false
	}

	cache := c.PerformanceConfig.Caching
	if !cache.Enabled {
		return false
	}

	switch cacheType {
	case "response":
		return cache.ResponseCache != nil && cache.ResponseCache.Enabled
	case "semantic":
		return cache.SemanticCache != nil && cache.SemanticCache.Enabled
	case "project":
		return cache.ProjectCache != nil && cache.ProjectCache.Enabled
	case "symbol":
		return cache.SymbolCache != nil && cache.SymbolCache.Enabled
	case "completion":
		return cache.CompletionCache != nil && cache.CompletionCache.Enabled
	case "diagnostic":
		return cache.DiagnosticCache != nil && cache.DiagnosticCache.Enabled
	case "filesystem":
		return cache.FileSystemCache != nil && cache.FileSystemCache.Enabled
	default:
		return cache.Enabled
	}
}

// GetCacheTTL returns the TTL for a specific cache type
func (c *GatewayConfig) GetCacheTTL(cacheType string) time.Duration {
	if c.PerformanceConfig == nil || c.PerformanceConfig.Caching == nil {
		return DefaultCacheTTL
	}

	cache := c.PerformanceConfig.Caching

	switch cacheType {
	case "response":
		if cache.ResponseCache != nil {
			return cache.ResponseCache.TTL
		}
	case "semantic":
		if cache.SemanticCache != nil {
			return cache.SemanticCache.TTL
		}
	case "project":
		if cache.ProjectCache != nil {
			return cache.ProjectCache.TTL
		}
	case "symbol":
		if cache.SymbolCache != nil {
			return cache.SymbolCache.TTL
		}
	case "completion":
		if cache.CompletionCache != nil {
			return cache.CompletionCache.TTL
		}
	case "diagnostic":
		if cache.DiagnosticCache != nil {
			return cache.DiagnosticCache.TTL
		}
	case "filesystem":
		if cache.FileSystemCache != nil {
			return cache.FileSystemCache.TTL
		}
	}

	return cache.GlobalTTL
}

// IsLargeProject returns whether the current project should be treated as large
func (c *GatewayConfig) IsLargeProject() bool {
	if c.PerformanceConfig == nil || c.PerformanceConfig.LargeProject == nil {
		return false
	}

	largeProject := c.PerformanceConfig.LargeProject

	// Check if auto-detection is enabled
	if largeProject.AutoDetectSize {
		// Check project context for file count
		if c.ProjectContext != nil {
			totalFiles := 0
			for _, lang := range c.ProjectContext.Languages {
				totalFiles += lang.FileCount
			}
			return totalFiles >= largeProject.FileCountThreshold
		}
	}

	return false
}

// GetIndexingStrategy returns the appropriate indexing strategy for the project
func (c *GatewayConfig) GetIndexingStrategy() string {
	if c.PerformanceConfig == nil || c.PerformanceConfig.LargeProject == nil {
		return IndexingStrategyEager
	}

	largeProject := c.PerformanceConfig.LargeProject

	// Return configured strategy if set
	if largeProject.IndexingStrategy != "" {
		return largeProject.IndexingStrategy
	}

	// Auto-select based on project size
	if c.IsLargeProject() {
		return IndexingStrategyIncremental
	}

	return IndexingStrategyEager
}

// GetOptimalServerCount returns the optimal number of servers for a language
func (c *GatewayConfig) GetOptimalServerCount(language string) int {
	if c.PerformanceConfig == nil || c.PerformanceConfig.LargeProject == nil ||
		c.PerformanceConfig.LargeProject.ServerPoolScaling == nil {
		return 1
	}

	scaling := c.PerformanceConfig.LargeProject.ServerPoolScaling

	// For large projects, return more servers
	if c.IsLargeProject() {
		return scaling.MaxServers
	}

	return scaling.MinServers
}

// ShouldEnableBackgroundIndexing returns whether background indexing should be enabled
func (c *GatewayConfig) ShouldEnableBackgroundIndexing() bool {
	if c.PerformanceConfig == nil || c.PerformanceConfig.LargeProject == nil ||
		c.PerformanceConfig.LargeProject.BackgroundIndexing == nil {
		return false
	}

	bgIndexing := c.PerformanceConfig.LargeProject.BackgroundIndexing

	// Enable for large projects by default
	return bgIndexing.Enabled && c.IsLargeProject()
}

// GetPerformanceProfile returns the current performance profile
func (c *GatewayConfig) GetPerformanceProfile() string {
	if c.PerformanceConfig == nil {
		return PerformanceProfileDevelopment
	}
	return c.PerformanceConfig.Profile
}

// UpdatePerformanceProfile updates the performance profile and applies optimizations
func (c *GatewayConfig) UpdatePerformanceProfile(profile string) error {
	perfConfig := c.GetPerformanceConfig()
	return perfConfig.OptimizeForProfile(profile)
}

// GetPerformanceMetrics returns current performance configuration metrics
func (c *GatewayConfig) GetPerformanceMetrics() map[string]interface{} {
	metrics := make(map[string]interface{})

	if c.PerformanceConfig == nil {
		return metrics
	}

	// Basic metrics
	metrics["enabled"] = c.PerformanceConfig.Enabled
	metrics["profile"] = c.PerformanceConfig.Profile
	metrics["auto_tuning"] = c.PerformanceConfig.AutoTuning

	// Cache metrics
	if c.PerformanceConfig.Caching != nil {
		cacheMetrics := map[string]interface{}{
			"enabled":             c.PerformanceConfig.Caching.Enabled,
			"global_ttl":          c.PerformanceConfig.Caching.GlobalTTL.String(),
			"max_memory_usage_mb": c.PerformanceConfig.Caching.MaxMemoryUsage,
			"eviction_strategy":   c.PerformanceConfig.Caching.EvictionStrategy,
		}
		metrics["caching"] = cacheMetrics
	}

	// Resource metrics
	if c.PerformanceConfig.ResourceManager != nil {
		resourceMetrics := map[string]interface{}{}

		if c.PerformanceConfig.ResourceManager.MemoryLimits != nil {
			resourceMetrics["memory_max_heap_mb"] = c.PerformanceConfig.ResourceManager.MemoryLimits.MaxHeapSize
			resourceMetrics["memory_soft_limit_mb"] = c.PerformanceConfig.ResourceManager.MemoryLimits.SoftLimit
		}

		if c.PerformanceConfig.ResourceManager.CPULimits != nil {
			resourceMetrics["cpu_max_usage_percent"] = c.PerformanceConfig.ResourceManager.CPULimits.MaxUsagePercent
			resourceMetrics["cpu_max_cores"] = c.PerformanceConfig.ResourceManager.CPULimits.MaxCores
		}

		metrics["resources"] = resourceMetrics
	}

	// Timeout metrics
	if c.PerformanceConfig.Timeouts != nil {
		timeoutMetrics := map[string]interface{}{
			"global_timeout":     c.PerformanceConfig.Timeouts.GlobalTimeout.String(),
			"default_timeout":    c.PerformanceConfig.Timeouts.DefaultTimeout.String(),
			"connection_timeout": c.PerformanceConfig.Timeouts.ConnectionTimeout.String(),
		}
		metrics["timeouts"] = timeoutMetrics
	}

	// Large project metrics
	if c.PerformanceConfig.LargeProject != nil {
		largeProjectMetrics := map[string]interface{}{
			"auto_detect_size":       c.PerformanceConfig.LargeProject.AutoDetectSize,
			"max_workspace_size_mb":  c.PerformanceConfig.LargeProject.MaxWorkspaceSize,
			"indexing_strategy":      c.PerformanceConfig.LargeProject.IndexingStrategy,
			"lazy_loading":           c.PerformanceConfig.LargeProject.LazyLoading,
			"workspace_partitioning": c.PerformanceConfig.LargeProject.WorkspacePartitioning,
		}
		metrics["large_project"] = largeProjectMetrics
	}

	return metrics
}

// IsPerformanceOptimizationEnabled returns whether performance optimization is enabled
func (c *GatewayConfig) IsPerformanceOptimizationEnabled() bool {
	return c.PerformanceConfig != nil && c.PerformanceConfig.Enabled
}

// GetPerformanceConfigSummary returns a summary of the performance configuration
func (c *GatewayConfig) GetPerformanceConfigSummary() string {
	if c.PerformanceConfig == nil {
		return "Performance configuration: disabled"
	}

	status := "disabled"
	if c.PerformanceConfig.Enabled {
		status = "enabled"
	}

	return fmt.Sprintf("Performance configuration: %s (profile: %s, auto-tuning: %t, version: %s)",
		status, c.PerformanceConfig.Profile, c.PerformanceConfig.AutoTuning, c.PerformanceConfig.Version)
}

// Validate validates the performance configuration
func (pc *PerformanceConfiguration) Validate() error {
	if pc == nil {
		return nil // nil performance config is valid (disabled)
	}

	// Validate profile
	validProfiles := map[string]bool{
		PerformanceProfileDevelopment: true,
		PerformanceProfileProduction:  true,
		PerformanceProfileAnalysis:    true,
	}
	if pc.Profile != "" && !validProfiles[pc.Profile] {
		return fmt.Errorf("invalid performance profile: %s, must be one of: development, production, analysis", pc.Profile)
	}

	// Validate caching configuration
	if pc.Caching != nil {
		if pc.Caching.MaxMemoryUsage < 0 {
			return fmt.Errorf("max memory usage cannot be negative: %d", pc.Caching.MaxMemoryUsage)
		}
		if pc.Caching.MaxMemoryUsage > MAX_MEMORY_MB_LIMIT {
			return fmt.Errorf("max memory usage exceeds limit: %d MB, maximum allowed: %d MB", pc.Caching.MaxMemoryUsage, MAX_MEMORY_MB_LIMIT)
		}
		if pc.Caching.GlobalTTL < 0 {
			return fmt.Errorf("global TTL cannot be negative: %v", pc.Caching.GlobalTTL)
		}
	}

	// Validate resource manager configuration
	if pc.ResourceManager != nil {
		if pc.ResourceManager.MemoryLimits != nil {
			if pc.ResourceManager.MemoryLimits.MaxHeapSize < 0 {
				return fmt.Errorf("max heap size cannot be negative: %d", pc.ResourceManager.MemoryLimits.MaxHeapSize)
			}
			if pc.ResourceManager.MemoryLimits.MaxHeapSize > MAX_MEMORY_MB_LIMIT {
				return fmt.Errorf("max heap size exceeds limit: %d MB, maximum allowed: %d MB", pc.ResourceManager.MemoryLimits.MaxHeapSize, MAX_MEMORY_MB_LIMIT)
			}
		}
		if pc.ResourceManager.CPULimits != nil {
			if pc.ResourceManager.CPULimits.MaxUsagePercent < 0 || pc.ResourceManager.CPULimits.MaxUsagePercent > 100 {
				return fmt.Errorf("max usage percent must be between 0 and 100: %.2f", pc.ResourceManager.CPULimits.MaxUsagePercent)
			}
		}
	}

	// Validate timeouts configuration
	if pc.Timeouts != nil {
		if pc.Timeouts.GlobalTimeout < 0 {
			return fmt.Errorf("global timeout cannot be negative: %v", pc.Timeouts.GlobalTimeout)
		}
		if pc.Timeouts.DefaultTimeout < 0 {
			return fmt.Errorf("default timeout cannot be negative: %v", pc.Timeouts.DefaultTimeout)
		}
		if pc.Timeouts.ConnectionTimeout < 0 {
			return fmt.Errorf("connection timeout cannot be negative: %v", pc.Timeouts.ConnectionTimeout)
		}
	}

	// Validate large project configuration
	if pc.LargeProject != nil {
		if pc.LargeProject.FileCountThreshold < 0 {
			return fmt.Errorf("file count threshold cannot be negative: %d", pc.LargeProject.FileCountThreshold)
		}
		if pc.LargeProject.MaxWorkspaceSize < 0 {
			return fmt.Errorf("max workspace size cannot be negative: %d", pc.LargeProject.MaxWorkspaceSize)
		}
		validStrategies := map[string]bool{
			IndexingStrategyEager:       true,
			IndexingStrategyLazy:        true,
			IndexingStrategyIncremental: true,
		}
		if pc.LargeProject.IndexingStrategy != "" && !validStrategies[pc.LargeProject.IndexingStrategy] {
			return fmt.Errorf("invalid indexing strategy: %s, must be one of: eager, lazy, incremental", pc.LargeProject.IndexingStrategy)
		}
	}

	return nil
}

// OptimizeForProfile optimizes the performance configuration for a specific profile
func (pc *PerformanceConfiguration) OptimizeForProfile(profile string) error {
	validProfiles := map[string]bool{
		PerformanceProfileDevelopment: true,
		PerformanceProfileProduction:  true,
		PerformanceProfileAnalysis:    true,
	}
	if !validProfiles[profile] {
		return fmt.Errorf("invalid performance profile: %s", profile)
	}

	pc.Profile = profile

	switch profile {
	case PerformanceProfileProduction:
		pc.Enabled = true
		if pc.Caching != nil {
			pc.Caching.Enabled = true
		}
		if pc.LargeProject != nil {
			pc.LargeProject.LazyLoading = true
			pc.LargeProject.IndexingStrategy = IndexingStrategyIncremental
		}
	case PerformanceProfileAnalysis:
		pc.Enabled = true
		if pc.Timeouts != nil {
			pc.Timeouts.GlobalTimeout = 60 * time.Second
			pc.Timeouts.DefaultTimeout = 45 * time.Second
		}
	case PerformanceProfileDevelopment:
		pc.Enabled = false
		if pc.LargeProject != nil {
			pc.LargeProject.IndexingStrategy = IndexingStrategyEager
		}
	}

	return nil
}

// ApplyEnvironmentDefaults applies environment-specific defaults
func (pc *PerformanceConfiguration) ApplyEnvironmentDefaults() error {
	// This would typically check environment variables or system resources
	// For now, just ensure reasonable defaults are set
	if pc.ResourceManager != nil && pc.ResourceManager.MemoryLimits != nil {
		memLimits := pc.ResourceManager.MemoryLimits
		if memLimits.MaxHeapSize == 0 {
			memLimits.MaxHeapSize = DefaultMemoryLimit
		}
		if memLimits.SoftLimit == 0 {
			memLimits.SoftLimit = memLimits.MaxHeapSize * 8 / 10
		}
		if memLimits.PerServerLimit == 0 {
			memLimits.PerServerLimit = memLimits.MaxHeapSize / 4
		}
	}

	return nil
}

// ValidateMultiServerConfig validates the multi-server configuration
func (c *GatewayConfig) ValidateMultiServerConfig() error {
	// Validate GlobalMultiServerConfig
	if c.GlobalMultiServerConfig != nil {
		if c.GlobalMultiServerConfig.SelectionStrategy != "" {
			validStrategies := map[string]bool{
				SELECTION_STRATEGY_PERFORMANCE:  true,
				SELECTION_STRATEGY_FEATURE:      true,
				SELECTION_STRATEGY_LOAD_BALANCE: true,
				SELECTION_STRATEGY_RANDOM:       true,
			}
			if !validStrategies[c.GlobalMultiServerConfig.SelectionStrategy] {
				return fmt.Errorf("invalid selection strategy: %s, must be one of: performance, feature, load_balance, random", c.GlobalMultiServerConfig.SelectionStrategy)
			}
		}

		if c.GlobalMultiServerConfig.ConcurrentLimit < 0 {
			return fmt.Errorf("concurrent limit cannot be negative: %d", c.GlobalMultiServerConfig.ConcurrentLimit)
		}

		if c.GlobalMultiServerConfig.ConcurrentLimit > MAX_CONCURRENT_LIMIT {
			return fmt.Errorf("concurrent limit exceeds maximum: %d, maximum allowed: %d", c.GlobalMultiServerConfig.ConcurrentLimit, MAX_CONCURRENT_LIMIT)
		}

		if c.GlobalMultiServerConfig.MaxRetries < 0 {
			return fmt.Errorf("max retries cannot be negative: %d", c.GlobalMultiServerConfig.MaxRetries)
		}

		if c.GlobalMultiServerConfig.MaxRetries > MAX_RETRIES_LIMIT {
			return fmt.Errorf("max retries exceeds limit: %d, maximum allowed: %d", c.GlobalMultiServerConfig.MaxRetries, MAX_RETRIES_LIMIT)
		}
	}

	// Validate LanguagePools configuration
	for i, pool := range c.LanguagePools {
		if pool.Language == "" {
			return fmt.Errorf("language cannot be empty in language server pool at index %d", i)
		}

		if len(pool.Servers) == 0 {
			return fmt.Errorf("language server pool for %s at index %d must have at least one server", pool.Language, i)
		}

		// Validate each server in the pool
		for name, server := range pool.Servers {
			if name == "" {
				return fmt.Errorf("server name cannot be empty in pool for language %s at index %d", pool.Language, i)
			}
			if server == nil {
				return fmt.Errorf("server %s is nil in pool for language %s at index %d", name, pool.Language, i)
			}

			// Ensure server supports the pool's language
			supports := false
			for _, lang := range server.Languages {
				if lang == pool.Language {
					supports = true
					break
				}
			}
			if !supports {
				return fmt.Errorf("server %s in pool does not support language %s at index %d", name, pool.Language, i)
			}
		}

		// Validate default server exists if specified
		if pool.DefaultServer != "" {
			if _, exists := pool.Servers[pool.DefaultServer]; !exists {
				return fmt.Errorf("default server %s not found in pool for language %s at index %d", pool.DefaultServer, pool.Language, i)
			}
		}

		// Validate resource limits if provided
		if pool.ResourceLimits != nil {
			if pool.ResourceLimits.MaxMemoryMB < 0 {
				return fmt.Errorf("max memory cannot be negative for language %s at index %d: %d", pool.Language, i, pool.ResourceLimits.MaxMemoryMB)
			}
			if pool.ResourceLimits.MaxMemoryMB > MAX_MEMORY_MB_LIMIT {
				return fmt.Errorf("max memory exceeds limit for language %s at index %d: %d MB, maximum allowed: %d MB", pool.Language, i, pool.ResourceLimits.MaxMemoryMB, MAX_MEMORY_MB_LIMIT)
			}
		}
	}

	// Validate EnableConcurrentServers settings
	if c.EnableConcurrentServers {
		if c.MaxConcurrentServersPerLanguage <= 0 {
			return fmt.Errorf("max concurrent servers per language must be positive when concurrent servers are enabled: %d", c.MaxConcurrentServersPerLanguage)
		}

		if c.MaxConcurrentServersPerLanguage > MAX_CONCURRENT_SERVERS_LIMIT {
			return fmt.Errorf("max concurrent servers per language exceeds limit: %d, maximum allowed: %d", c.MaxConcurrentServersPerLanguage, MAX_CONCURRENT_SERVERS_LIMIT)
		}
	}

	return nil
}

// ValidateConsistency validates configuration consistency
func (c *GatewayConfig) ValidateConsistency() error {
	if err := c.validateSmartRouterConfig(); err != nil {
		return err
	}

	if err := c.validateProjectContextConsistency(); err != nil {
		return err
	}

	if err := c.validatePerformanceConfigConsistency(); err != nil {
		return err
	}

	if err := c.validateTimeoutSettings(); err != nil {
		return err
	}

	if err := c.validateConcurrentRequestsSettings(); err != nil {
		return err
	}

	return nil
}

// validateSmartRouterConfig validates SmartRouterConfig compatibility with other settings
func (c *GatewayConfig) validateSmartRouterConfig() error {
	if !c.EnableSmartRouting || c.SmartRouterConfig == nil {
		return nil
	}

	if err := c.validateMethodStrategies(); err != nil {
		return err
	}

	return c.validateCircuitBreakerConfig()
}

// validateMethodStrategies validates smart router method strategies
func (c *GatewayConfig) validateMethodStrategies() error {
	for method, strategy := range c.SmartRouterConfig.MethodStrategies {
		if method == "" {
			return fmt.Errorf("empty method name in smart router method strategies")
		}
		if strategy == "" {
			return fmt.Errorf("empty strategy for method %s in smart router configuration", method)
		}
	}
	return nil
}

// validateCircuitBreakerConfig validates circuit breaker configuration consistency
func (c *GatewayConfig) validateCircuitBreakerConfig() error {
	if !c.SmartRouterConfig.EnableCircuitBreaker {
		return nil
	}

	if c.SmartRouterConfig.CircuitBreakerThreshold <= 0 {
		return fmt.Errorf("circuit breaker threshold must be positive when circuit breaker is enabled: %d", c.SmartRouterConfig.CircuitBreakerThreshold)
	}

	if c.SmartRouterConfig.CircuitBreakerTimeout != "" {
		if _, err := time.ParseDuration(c.SmartRouterConfig.CircuitBreakerTimeout); err != nil {
			return fmt.Errorf("invalid circuit breaker timeout format: %s, error: %w", c.SmartRouterConfig.CircuitBreakerTimeout, err)
		}
	}

	return nil
}

// validateProjectContextConsistency validates ProjectContext consistency with servers
func (c *GatewayConfig) validateProjectContextConsistency() error {
	if c.ProjectContext == nil {
		return nil
	}

	if err := c.validateRequiredLanguagesAvailable(); err != nil {
		return err
	}

	return c.validateRequiredLSPServersExist()
}

// validateRequiredLanguagesAvailable checks if all required languages have available servers
func (c *GatewayConfig) validateRequiredLanguagesAvailable() error {
	requiredLanguages := make(map[string]bool)
	for _, lang := range c.ProjectContext.Languages {
		requiredLanguages[lang.Language] = true
	}

	availableLanguages := make(map[string]bool)
	for _, server := range c.Servers {
		for _, lang := range server.Languages {
			availableLanguages[lang] = true
		}
	}

	for reqLang := range requiredLanguages {
		if !availableLanguages[reqLang] {
			return fmt.Errorf("project context requires language %s but no server supports it", reqLang)
		}
	}

	return nil
}

// validateRequiredLSPServersExist validates that required LSP servers exist
func (c *GatewayConfig) validateRequiredLSPServersExist() error {
	serverNames := make(map[string]bool)
	for _, server := range c.Servers {
		serverNames[server.Name] = true
	}

	for _, requiredLSP := range c.ProjectContext.RequiredLSPs {
		if !serverNames[requiredLSP] {
			return fmt.Errorf("project context requires LSP server %s but it is not configured", requiredLSP)
		}
	}

	return nil
}

// validatePerformanceConfigConsistency validates performance configuration compatibility
func (c *GatewayConfig) validatePerformanceConfigConsistency() error {
	if c.PerformanceConfig == nil || !c.PerformanceConfig.Enabled {
		return nil
	}

	if err := c.validatePerformanceTimeouts(); err != nil {
		return err
	}

	return c.validateMemoryLimitsConsistency()
}

// validatePerformanceTimeouts validates timeout consistency with global settings
func (c *GatewayConfig) validatePerformanceTimeouts() error {
	if c.PerformanceConfig.Timeouts == nil {
		return nil
	}

	globalTimeout := c.PerformanceConfig.Timeouts.GlobalTimeout
	if globalTimeout <= 0 {
		return nil
	}

	configTimeout, err := time.ParseDuration(c.Timeout)
	if err != nil {
		return nil
	}

	if globalTimeout < configTimeout {
		return fmt.Errorf("performance global timeout (%v) is less than config timeout (%v)", globalTimeout, configTimeout)
	}

	return nil
}

// validateMemoryLimitsConsistency validates memory limits with server limits
func (c *GatewayConfig) validateMemoryLimitsConsistency() error {
	if c.PerformanceConfig.ResourceManager == nil || c.PerformanceConfig.ResourceManager.MemoryLimits == nil {
		return nil
	}

	memLimits := c.PerformanceConfig.ResourceManager.MemoryLimits

	for _, pool := range c.LanguagePools {
		if pool.ResourceLimits == nil {
			continue
		}

		if int64(pool.ResourceLimits.MaxMemoryMB) > memLimits.MaxHeapSize {
			return fmt.Errorf("language pool %s memory limit (%d MB) exceeds global max heap size (%d MB)", pool.Language, pool.ResourceLimits.MaxMemoryMB, memLimits.MaxHeapSize)
		}
	}

	return nil
}

// validateTimeoutSettings validates timeout format and reasonableness
func (c *GatewayConfig) validateTimeoutSettings() error {
	if c.Timeout == "" {
		return nil
	}

	duration, err := time.ParseDuration(c.Timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout format: %s, error: %w", c.Timeout, err)
	}

	if duration <= 0 {
		return fmt.Errorf("timeout must be positive: %v", duration)
	}

	if duration > 1*time.Hour {
		return fmt.Errorf("timeout is too large: %v, maximum allowed: 1h", duration)
	}

	return nil
}

// validateConcurrentRequestsSettings validates concurrent requests consistency
func (c *GatewayConfig) validateConcurrentRequestsSettings() error {
	if c.MaxConcurrentRequests <= 0 {
		return fmt.Errorf("max concurrent requests must be positive: %d", c.MaxConcurrentRequests)
	}

	if c.MaxConcurrentRequests > MAX_CONCURRENT_REQUESTS_LIMIT {
		return fmt.Errorf("max concurrent requests exceeds limit: %d, maximum allowed: %d", c.MaxConcurrentRequests, MAX_CONCURRENT_REQUESTS_LIMIT)
	}

	return nil
}

// ValidateMultiServerFields validates multi-server specific fields for ServerConfig
func (s *ServerConfig) ValidateMultiServerFields() error {
	// Check Priority values are reasonable
	if s.Priority < 0 {
		return fmt.Errorf("priority cannot be negative: %d", s.Priority)
	}

	if s.Priority > MAX_PRIORITY {
		return fmt.Errorf("priority exceeds maximum allowed: %d, maximum: %d", s.Priority, MAX_PRIORITY)
	}

	// Check Weight values are reasonable
	if s.Weight < 0 {
		return fmt.Errorf("weight cannot be negative: %.2f", s.Weight)
	}

	if s.Weight > MAX_WEIGHT {
		return fmt.Errorf("weight exceeds maximum allowed: %.2f, maximum: %.2f", s.Weight, MAX_WEIGHT)
	}

	// Validate HealthCheckEndpoint if provided
	if s.HealthCheckEndpoint != "" {
		// Basic URL format validation
		if !strings.HasPrefix(s.HealthCheckEndpoint, "http://") && !strings.HasPrefix(s.HealthCheckEndpoint, "https://") {
			return fmt.Errorf("health check endpoint must be a valid HTTP/HTTPS URL: %s", s.HealthCheckEndpoint)
		}
	}

	// Check MaxConcurrentRequests is reasonable
	if s.MaxConcurrentRequests < 0 {
		return fmt.Errorf("max concurrent requests cannot be negative: %d", s.MaxConcurrentRequests)
	}

	if s.MaxConcurrentRequests > MAX_SERVER_CONCURRENT_REQUESTS {
		return fmt.Errorf("max concurrent requests exceeds limit: %d, maximum allowed: %d", s.MaxConcurrentRequests, MAX_SERVER_CONCURRENT_REQUESTS)
	}

	// Validate ServerType field
	if s.ServerType != "" {
		validTypes := map[string]bool{
			ServerTypeSingle:    true,
			ServerTypeMulti:     true,
			ServerTypeWorkspace: true,
		}
		if !validTypes[s.ServerType] {
			return fmt.Errorf("invalid server type: %s, must be one of: single, multi, workspace", s.ServerType)
		}
	}

	// Check Constraints if provided
	if s.Constraints != nil {
		if s.Constraints.MinFileCount < 0 {
			return fmt.Errorf("min file count cannot be negative: %d", s.Constraints.MinFileCount)
		}
		if s.Constraints.MaxFileCount < 0 {
			return fmt.Errorf("max file count cannot be negative: %d", s.Constraints.MaxFileCount)
		}
		if s.Constraints.MinFileCount > 0 && s.Constraints.MaxFileCount > 0 && s.Constraints.MinFileCount > s.Constraints.MaxFileCount {
			return fmt.Errorf("min file count (%d) cannot exceed max file count (%d)", s.Constraints.MinFileCount, s.Constraints.MaxFileCount)
		}
	}

	// Validate workspace roots if provided
	for language, rootPath := range s.WorkspaceRoots {
		if language == "" {
			return fmt.Errorf("empty language key in workspace roots")
		}
		if rootPath == "" {
			return fmt.Errorf("empty root path for language %s in workspace roots", language)
		}
		if !filepath.IsAbs(rootPath) {
			return fmt.Errorf("workspace root for language %s must be absolute path: %s", language, rootPath)
		}
	}

	// Validate language settings structure
	for language, settings := range s.LanguageSettings {
		if language == "" {
			return fmt.Errorf("empty language key in language settings")
		}
		if settings == nil {
			return fmt.Errorf("language settings for %s cannot be nil", language)
		}
	}

	// Validate dependencies
	for i, dep := range s.Dependencies {
		if strings.TrimSpace(dep) == "" {
			return fmt.Errorf("dependency at index %d cannot be empty or whitespace-only", i)
		}
	}

	// Validate frameworks
	for i, framework := range s.Frameworks {
		if strings.TrimSpace(framework) == "" {
			return fmt.Errorf("framework at index %d cannot be empty or whitespace-only", i)
		}
	}

	return nil
}

// Validate validates the MultiLanguageConfig
func (mlc *MultiLanguageConfig) Validate() error {
	if mlc == nil {
		return fmt.Errorf("multi-language config cannot be nil")
	}

	// Validate server configurations
	for i, serverConfig := range mlc.ServerConfigs {
		if serverConfig == nil {
			return fmt.Errorf("server config at index %d cannot be nil", i)
		}
		if err := serverConfig.Validate(); err != nil {
			return fmt.Errorf("server config at index %d validation failed: %w", i, err)
		}
	}

	// Validate project info
	if mlc.ProjectInfo != nil {
		if mlc.ProjectInfo.ProjectType == "" {
			return fmt.Errorf("project type cannot be empty")
		}
		if mlc.ProjectInfo.RootDirectory == "" {
			return fmt.Errorf("root directory cannot be empty")
		}

		// Validate language contexts
		for i, langCtx := range mlc.ProjectInfo.LanguageContexts {
			if langCtx == nil {
				return fmt.Errorf("language context at index %d cannot be nil", i)
			}
			if langCtx.Language == "" {
				return fmt.Errorf("language at index %d cannot be empty", i)
			}
		}
	}

	// Validate workspace config
	if mlc.WorkspaceConfig != nil {
		for lang, root := range mlc.WorkspaceConfig.LanguageRoots {
			if lang == "" {
				return fmt.Errorf("empty language key in workspace language roots")
			}
			if root == "" {
				return fmt.Errorf("empty root path for language %s in workspace", lang)
			}
		}
	}

	return nil
}

// WriteYAML writes the MultiLanguageConfig to a YAML file
func (mlc *MultiLanguageConfig) WriteYAML(filepath string) error {
	data, err := yaml.Marshal(mlc)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write YAML file: %w", err)
	}

	return nil
}

// WriteJSON writes the MultiLanguageConfig to a JSON file
func (mlc *MultiLanguageConfig) WriteJSON(filepath string) error {
	data, err := json.MarshalIndent(mlc, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config to JSON: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write JSON file: %w", err)
	}

	return nil
}

// Validate validates the MultiServerConfig structure
func (msc *MultiServerConfig) Validate() error {
	if msc == nil {
		return fmt.Errorf("MultiServerConfig cannot be nil")
	}

	// Validate primary server
	if msc.Primary == nil {
		return fmt.Errorf("primary server configuration is required")
	}
	if err := msc.Primary.Validate(); err != nil {
		return fmt.Errorf("primary server configuration is invalid: %w", err)
	}

	// Validate selection strategy
	validStrategies := map[string]bool{
		"performance":  true,
		"feature":      true,
		"load_balance": true,
		"random":       true,
	}
	if !validStrategies[msc.SelectionStrategy] {
		return fmt.Errorf("invalid selection strategy: %s", msc.SelectionStrategy)
	}

	// Validate concurrent limit
	if msc.ConcurrentLimit < 0 {
		return fmt.Errorf("concurrent limit cannot be negative")
	}

	// Validate max retries
	if msc.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative")
	}

	// Validate secondary servers
	for i, server := range msc.Secondary {
		if server == nil {
			return fmt.Errorf("secondary server at index %d cannot be nil", i)
		}
		if err := server.Validate(); err != nil {
			return fmt.Errorf("secondary server at index %d is invalid: %w", i, err)
		}
	}

	return nil
}

// Validate validates the ResourceLimits structure
func (rl *ResourceLimits) Validate() error {
	if rl == nil {
		return fmt.Errorf("ResourceLimits cannot be nil")
	}

	if rl.MaxMemoryMB < 0 {
		return fmt.Errorf("max memory cannot be negative")
	}

	if rl.MaxConcurrentRequests < 0 {
		return fmt.Errorf("max concurrent requests cannot be negative")
	}

	if rl.MaxProcesses < 0 {
		return fmt.Errorf("max processes cannot be negative")
	}

	if rl.RequestTimeoutSeconds < 0 {
		return fmt.Errorf("request timeout cannot be negative")
	}

	return nil
}

// Validate validates the LoadBalancingConfig structure
func (lbc *LoadBalancingConfig) Validate() error {
	if lbc == nil {
		return fmt.Errorf("LoadBalancingConfig cannot be nil")
	}

	// Validate strategy
	validStrategies := map[string]bool{
		"round_robin":       true,
		"least_connections": true,
		"response_time":     true,
		"resource_usage":    true,
	}
	if !validStrategies[lbc.Strategy] {
		return fmt.Errorf("invalid load balancing strategy: %s", lbc.Strategy)
	}

	// Validate health threshold
	if lbc.HealthThreshold < 0.0 || lbc.HealthThreshold > 1.0 {
		return fmt.Errorf("health threshold must be between 0.0 and 1.0, got: %f", lbc.HealthThreshold)
	}

	// Validate weight factors
	for name, weight := range lbc.WeightFactors {
		if weight < 0.0 {
			return fmt.Errorf("weight factor for %s cannot be negative: %f", name, weight)
		}
	}

	return nil
}

// LoadMultiLanguageConfig loads a MultiLanguageConfig from a file
func LoadMultiLanguageConfig(filepath string) (*MultiLanguageConfig, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config MultiLanguageConfig

	// Try YAML first, then JSON
	if err := yaml.Unmarshal(data, &config); err != nil {
		// If YAML fails, try JSON
		if jsonErr := json.Unmarshal(data, &config); jsonErr != nil {
			return nil, fmt.Errorf("failed to parse config as YAML or JSON: YAML error: %v, JSON error: %v", err, jsonErr)
		}
	}

	// Validate the loaded config
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("loaded config is invalid: %w", err)
	}

	return &config, nil
}

// AutoGenerateConfigFromPath generates a MultiLanguageConfig from a project path
func AutoGenerateConfigFromPath(projectPath string) (*MultiLanguageConfig, error) {
	generator := NewConfigGenerator()

	// Simple language detection based on file markers
	detectedLanguages := detectLanguagesFromPath(projectPath)
	
	if len(detectedLanguages) == 0 {
		// If no languages detected, fall back to default multi-language config
		return generator.GenerateDefaultMultiLanguageConfig()
	}

	// Create language contexts from detected languages
	var languageContexts []*LanguageContext
	for _, lang := range detectedLanguages {
		ctx := &LanguageContext{
			Language:     lang,
			RootPath:     projectPath,
			FilePatterns: getFilePatterns(lang),
			RootMarkers:  getRootMarkers(lang),
			FileCount:    10, // Default estimate
		}
		languageContexts = append(languageContexts, ctx)
	}

	// Create project info with detected languages
	projectInfo := &MultiLanguageProjectInfo{
		ProjectType:      ProjectTypeMulti,
		RootDirectory:    projectPath,
		LanguageContexts: languageContexts,
		DetectedAt:       time.Now(),
		Metadata:         make(map[string]interface{}),
	}

	return generator.GenerateMultiLanguageConfig(projectInfo)
}

// mapDetectedLanguageToLSPLanguage maps project detection language names to LSP server language names
func mapDetectedLanguageToLSPLanguage(detectedLanguage string) string {
	languageMap := map[string]string{
		"nodejs":     "javascript", // Node.js projects map to javascript
		"javascript": "javascript", // JavaScript projects (no change)
		"typescript": "typescript", // TypeScript projects (no change)
		"js":         "javascript", // Common JS abbreviation
		"ts":         "typescript", // Common TS abbreviation
	}
	
	if mapped, exists := languageMap[detectedLanguage]; exists {
		return mapped
	}
	
	// Return original language if no mapping exists
	return detectedLanguage
}

// detectLanguagesFromPath detects languages in a project path by checking for marker files
func detectLanguagesFromPath(projectPath string) []string {
	var detectedLanguages []string
	languageMarkers := map[string][]string{
		"java":       {"pom.xml", "build.gradle", "build.gradle.kts", "*.java"},
		"go":         {"go.mod", "go.sum", "*.go"},
		"python":     {"pyproject.toml", "requirements.txt", "setup.py", "*.py"},
		"javascript": {"package.json", "*.js", "*.jsx"},
		"typescript": {"tsconfig.json", "*.ts", "*.tsx"},
		"rust":       {"Cargo.toml", "*.rs"},
		"cpp":        {"CMakeLists.txt", "Makefile", "*.cpp", "*.cc", "*.h"},
		"csharp":     {"*.csproj", "*.sln", "*.cs"},
	}

	for language, markers := range languageMarkers {
		for _, marker := range markers {
			// Check if marker file exists
			if strings.Contains(marker, "*") {
				// Pattern matching - check if any file matches
				pattern := filepath.Join(projectPath, marker)
				matches, _ := filepath.Glob(pattern)
				if len(matches) > 0 {
					detectedLanguages = append(detectedLanguages, language)
					break
				}
			} else {
				// Exact file match
				markerPath := filepath.Join(projectPath, marker)
				if _, err := os.Stat(markerPath); err == nil {
					detectedLanguages = append(detectedLanguages, language)
					break
				}
			}
		}
	}

	return detectedLanguages
}

// getFilePatterns returns file patterns for a language
func getFilePatterns(language string) []string {
	patterns := map[string][]string{
		"go":         {"*.go"},
		"python":     {"*.py"},
		"javascript": {"*.js", "*.jsx", "*.mjs"},
		"typescript": {"*.ts", "*.tsx"},
		"java":       {"*.java"},
		"rust":       {"*.rs"},
		"cpp":        {"*.cpp", "*.cc", "*.cxx", "*.hpp", "*.h"},
		"csharp":     {"*.cs"},
	}
	
	if p, ok := patterns[language]; ok {
		return p
	}
	return []string{"*"}
}

// getRootMarkers returns root markers for a language
func getRootMarkers(language string) []string {
	markers := map[string][]string{
		"go":         {"go.mod", "go.sum"},
		"python":     {"pyproject.toml", "requirements.txt", "setup.py"},
		"javascript": {"package.json", "node_modules"},
		"typescript": {"tsconfig.json", "package.json"},
		"java":       {"pom.xml", "build.gradle", "build.gradle.kts"},
		"rust":       {"Cargo.toml"},
		"cpp":        {"CMakeLists.txt", "Makefile", ".clangd"},
		"csharp":     {"*.csproj", "*.sln"},
	}
	
	if m, ok := markers[language]; ok {
		return m
	}
	return []string{}
}
