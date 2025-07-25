package config

import (
	"fmt"
	"lsp-gateway/internal/transport"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
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
	DefaultMemoryLimit = 1024 * 1024 * 1024 // 1GB in bytes
	
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
	ProjectTypeSingle        = "single-language"
	ProjectTypeMulti         = "multi-language"
	ProjectTypeMonorepo      = "monorepo"
	ProjectTypeWorkspace     = "workspace"
	ProjectTypeFrontendBackend = "frontend-backend"
	ProjectTypeMicroservices = "microservices"
	ProjectTypePolyglot      = "polyglot"
	ProjectTypeEmpty         = "empty"
	ProjectTypeUnknown       = "unknown"
)

type LanguageInfo struct {
	Language     string   `yaml:"language" json:"language"`
	FilePatterns []string `yaml:"file_patterns" json:"file_patterns"`
	FileCount    int      `yaml:"file_count" json:"file_count"`
	RootMarkers  []string `yaml:"root_markers,omitempty" json:"root_markers,omitempty"`
}

type ProjectContext struct {
	ProjectType   string         `yaml:"project_type" json:"project_type"`
	RootDirectory string         `yaml:"root_directory" json:"root_directory"`
	WorkspaceRoot string         `yaml:"workspace_root,omitempty" json:"workspace_root,omitempty"`
	Languages     []LanguageInfo `yaml:"languages" json:"languages"`
	RequiredLSPs  []string       `yaml:"required_lsps" json:"required_lsps"`
	DetectedAt    time.Time      `yaml:"detected_at" json:"detected_at"`
	Metadata      map[string]interface{} `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

type ProjectServerOverride struct {
	Name      string            `yaml:"name" json:"name"`
	Enabled   *bool             `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Args      []string          `yaml:"args,omitempty" json:"args,omitempty"`
	Settings  map[string]interface{} `yaml:"settings,omitempty" json:"settings,omitempty"`
	Transport string            `yaml:"transport,omitempty" json:"transport,omitempty"`
}

type ProjectConfig struct {
	ProjectID       string                   `yaml:"project_id" json:"project_id"`
	Name            string                   `yaml:"name,omitempty" json:"name,omitempty"`
	RootDirectory   string                   `yaml:"root_directory" json:"root_directory"`
	ServerOverrides []ProjectServerOverride  `yaml:"server_overrides,omitempty" json:"server_overrides,omitempty"`
	EnabledServers  []string                 `yaml:"enabled_servers,omitempty" json:"enabled_servers,omitempty"`
	Optimizations   map[string]interface{}   `yaml:"optimizations,omitempty" json:"optimizations,omitempty"`
	GeneratedAt     time.Time                `yaml:"generated_at" json:"generated_at"`
	Version         string                   `yaml:"version,omitempty" json:"version,omitempty"`
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
	Priority int `yaml:"priority,omitempty" json:"priority,omitempty"`
	Weight float64 `yaml:"weight,omitempty" json:"weight,omitempty"`
	HealthCheckEndpoint string `yaml:"health_check_endpoint,omitempty" json:"health_check_endpoint,omitempty"`
	MaxConcurrentRequests int `yaml:"max_concurrent_requests,omitempty" json:"max_concurrent_requests,omitempty"`

	// Enhanced multi-language fields
	WorkspaceRoots  map[string]string         `yaml:"workspace_roots,omitempty" json:"workspace_roots,omitempty"`
	LanguageSettings map[string]map[string]interface{} `yaml:"language_settings,omitempty" json:"language_settings,omitempty"`
	ServerType      string                    `yaml:"server_type,omitempty" json:"server_type,omitempty"` // "single", "multi", "workspace"
	Dependencies    []string                  `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	Constraints     *ServerConstraints        `yaml:"constraints,omitempty" json:"constraints,omitempty"`
	Frameworks      []string                  `yaml:"frameworks,omitempty" json:"frameworks,omitempty"`
	Version         string                    `yaml:"version,omitempty" json:"version,omitempty"`
}

// SmartRouterConfig contains configuration for the SmartRouter
type SmartRouterConfig struct {
	DefaultStrategy string `yaml:"default_strategy,omitempty" json:"default_strategy,omitempty"`
	MethodStrategies map[string]string `yaml:"method_strategies,omitempty" json:"method_strategies,omitempty"`
	EnablePerformanceMonitoring bool `yaml:"enable_performance_monitoring,omitempty" json:"enable_performance_monitoring,omitempty"`
	EnableCircuitBreaker bool `yaml:"enable_circuit_breaker,omitempty" json:"enable_circuit_breaker,omitempty"`
	CircuitBreakerThreshold int `yaml:"circuit_breaker_threshold,omitempty" json:"circuit_breaker_threshold,omitempty"`
	CircuitBreakerTimeout string `yaml:"circuit_breaker_timeout,omitempty" json:"circuit_breaker_timeout,omitempty"`
}

// PerformanceCacheConfig contains configuration for performance caching
type PerformanceCacheConfig struct {
	Enabled bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	MaxSize int `yaml:"max_size,omitempty" json:"max_size,omitempty"`
	TTL string `yaml:"ttl,omitempty" json:"ttl,omitempty"`
}

// RequestClassifierConfig contains configuration for request classification
type RequestClassifierConfig struct {
	Enabled bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Rules map[string]interface{} `yaml:"rules,omitempty" json:"rules,omitempty"`
}

// ResponseAggregatorConfig contains configuration for response aggregation
type ResponseAggregatorConfig struct {
	Enabled bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	MaxServers int `yaml:"max_servers,omitempty" json:"max_servers,omitempty"`
	Timeout string `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// HealthMonitorConfig contains configuration for health monitoring
type HealthMonitorConfig struct {
	Enabled bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	CheckInterval string `yaml:"check_interval,omitempty" json:"check_interval,omitempty"`
	FailureThreshold int `yaml:"failure_threshold,omitempty" json:"failure_threshold,omitempty"`
}

type GatewayConfig struct {
	Servers []ServerConfig `yaml:"servers" json:"servers"`

	Port int `yaml:"port" json:"port"`

	Timeout               string         `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	MaxConcurrentRequests int            `yaml:"max_concurrent_requests,omitempty" json:"max_concurrent_requests,omitempty"`
	ProjectContext        *ProjectContext `yaml:"project_context,omitempty" json:"project_context,omitempty"`
	ProjectConfig         *ProjectConfig  `yaml:"project_config,omitempty" json:"project_config,omitempty"`
	ProjectAware          bool           `yaml:"project_aware,omitempty" json:"project_aware,omitempty"`

	// Multi-server configuration fields
	LanguagePools []LanguageServerPool `yaml:"language_pools,omitempty" json:"language_pools,omitempty"`
	GlobalMultiServerConfig *MultiServerConfig `yaml:"multi_server_config,omitempty" json:"multi_server_config,omitempty"`
	EnableConcurrentServers bool `yaml:"enable_concurrent_servers" json:"enable_concurrent_servers"`
	MaxConcurrentServersPerLanguage int `yaml:"max_concurrent_servers_per_language" json:"max_concurrent_servers_per_language"`
	
	// SmartRouter configuration fields
	EnableSmartRouting bool `yaml:"enable_smart_routing,omitempty" json:"enable_smart_routing,omitempty"`
	EnableEnhancements bool `yaml:"enable_enhancements,omitempty" json:"enable_enhancements,omitempty"`
	SmartRouterConfig *SmartRouterConfig `yaml:"smart_router_config,omitempty" json:"smart_router_config,omitempty"`
	
	// Performance configuration
	PerformanceConfig *PerformanceConfiguration `yaml:"performance_config,omitempty" json:"performance_config,omitempty"`
}

// PerformanceConfiguration contains configuration for performance optimization
type PerformanceConfiguration struct {
	Enabled      bool                   `yaml:"enabled" json:"enabled"`
	Profile      string                 `yaml:"profile,omitempty" json:"profile,omitempty"`
	AutoTuning   bool                   `yaml:"auto_tuning,omitempty" json:"auto_tuning,omitempty"`
	Version      string                 `yaml:"version,omitempty" json:"version,omitempty"`
	Caching      *CachingConfiguration  `yaml:"caching,omitempty" json:"caching,omitempty"`
	ResourceManager *ResourceManagerConfig `yaml:"resource_manager,omitempty" json:"resource_manager,omitempty"`
	Timeouts     *TimeoutConfiguration  `yaml:"timeouts,omitempty" json:"timeouts,omitempty"`
	LargeProject *LargeProjectConfig    `yaml:"large_project,omitempty" json:"large_project,omitempty"`
}

// CachingConfiguration contains caching configuration
type CachingConfiguration struct {
	Enabled          bool                    `yaml:"enabled" json:"enabled"`
	GlobalTTL        time.Duration          `yaml:"global_ttl,omitempty" json:"global_ttl,omitempty"`
	MaxMemoryUsage   int64                  `yaml:"max_memory_usage_mb,omitempty" json:"max_memory_usage_mb,omitempty"`
	EvictionStrategy string                 `yaml:"eviction_strategy,omitempty" json:"eviction_strategy,omitempty"`
	ResponseCache    *CacheConfig           `yaml:"response_cache,omitempty" json:"response_cache,omitempty"`
	SemanticCache    *CacheConfig           `yaml:"semantic_cache,omitempty" json:"semantic_cache,omitempty"`
	ProjectCache     *CacheConfig           `yaml:"project_cache,omitempty" json:"project_cache,omitempty"`
	SymbolCache      *CacheConfig           `yaml:"symbol_cache,omitempty" json:"symbol_cache,omitempty"`
	CompletionCache  *CacheConfig           `yaml:"completion_cache,omitempty" json:"completion_cache,omitempty"`
	DiagnosticCache  *CacheConfig           `yaml:"diagnostic_cache,omitempty" json:"diagnostic_cache,omitempty"`
	FileSystemCache  *CacheConfig           `yaml:"filesystem_cache,omitempty" json:"filesystem_cache,omitempty"`
}

// CacheConfig contains individual cache configuration
type CacheConfig struct {
	Enabled   bool          `yaml:"enabled" json:"enabled"`
	TTL       time.Duration `yaml:"ttl,omitempty" json:"ttl,omitempty"`
	MaxSize   int64         `yaml:"max_size,omitempty" json:"max_size,omitempty"`
}

// ResourceManagerConfig contains resource management configuration
type ResourceManagerConfig struct {
	MemoryLimits *MemoryLimitsConfig `yaml:"memory_limits,omitempty" json:"memory_limits,omitempty"`
	CPULimits    *CPULimitsConfig    `yaml:"cpu_limits,omitempty" json:"cpu_limits,omitempty"`
}

// MemoryLimitsConfig contains memory limit configuration
type MemoryLimitsConfig struct {
	MaxHeapSize     int64 `yaml:"max_heap_size_mb,omitempty" json:"max_heap_size_mb,omitempty"`
	SoftLimit       int64 `yaml:"soft_limit_mb,omitempty" json:"soft_limit_mb,omitempty"`
	PerServerLimit  int64 `yaml:"per_server_limit_mb,omitempty" json:"per_server_limit_mb,omitempty"`
}

// CPULimitsConfig contains CPU limit configuration
type CPULimitsConfig struct {
	MaxUsagePercent float64 `yaml:"max_usage_percent,omitempty" json:"max_usage_percent,omitempty"`
	MaxCores        int     `yaml:"max_cores,omitempty" json:"max_cores,omitempty"`
}

// TimeoutConfiguration contains timeout configuration
type TimeoutConfiguration struct {
	GlobalTimeout      time.Duration            `yaml:"global_timeout,omitempty" json:"global_timeout,omitempty"`
	DefaultTimeout     time.Duration            `yaml:"default_timeout,omitempty" json:"default_timeout,omitempty"`
	ConnectionTimeout  time.Duration            `yaml:"connection_timeout,omitempty" json:"connection_timeout,omitempty"`
	MethodTimeouts     map[string]time.Duration `yaml:"method_timeouts,omitempty" json:"method_timeouts,omitempty"`
	LanguageTimeouts   map[string]time.Duration `yaml:"language_timeouts,omitempty" json:"language_timeouts,omitempty"`
}

// LargeProjectConfig contains configuration for large projects
type LargeProjectConfig struct {
	AutoDetectSize          bool                    `yaml:"auto_detect_size,omitempty" json:"auto_detect_size,omitempty"`
	FileCountThreshold      int                     `yaml:"file_count_threshold,omitempty" json:"file_count_threshold,omitempty"`
	MaxWorkspaceSize        int64                   `yaml:"max_workspace_size_mb,omitempty" json:"max_workspace_size_mb,omitempty"`
	IndexingStrategy        string                  `yaml:"indexing_strategy,omitempty" json:"indexing_strategy,omitempty"`
	LazyLoading            bool                    `yaml:"lazy_loading,omitempty" json:"lazy_loading,omitempty"`
	WorkspacePartitioning  bool                    `yaml:"workspace_partitioning,omitempty" json:"workspace_partitioning,omitempty"`
	ServerPoolScaling      *ServerPoolScalingConfig `yaml:"server_pool_scaling,omitempty" json:"server_pool_scaling,omitempty"`
	BackgroundIndexing     *BackgroundIndexingConfig `yaml:"background_indexing,omitempty" json:"background_indexing,omitempty"`
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

func DefaultConfig() *GatewayConfig {
	config := &GatewayConfig{
		Port:                  8080,
		Timeout:               "30s",
		MaxConcurrentRequests: 100,
		ProjectAware:          false,
		EnableConcurrentServers: false,
		MaxConcurrentServersPerLanguage: DEFAULT_MAX_CONCURRENT_SERVERS_PER_LANG,
		
		// SmartRouter defaults (disabled by default for backward compatibility)
		EnableSmartRouting: false,
		EnableEnhancements: false,
		SmartRouterConfig: &SmartRouterConfig{
			DefaultStrategy: "single_target_with_fallback",
			MethodStrategies: map[string]string{
				"textDocument/definition":      "single_target_with_fallback",
				"textDocument/references":      "multi_target_parallel",
				"textDocument/documentSymbol":  "single_target_with_fallback",
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
		LanguagePools: []LanguageServerPool{},
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
			AutoDetectSize:         true,
			FileCountThreshold:     10000,
			MaxWorkspaceSize:       10240, // 10GB
			IndexingStrategy:       IndexingStrategyEager,
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
	}
}

func (c *GatewayConfig) Validate() error {
	if c.Port < 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d, must be between 0 and 65535", c.Port)
	}

	if len(c.Servers) == 0 {
		return fmt.Errorf("at least one server must be configured")
	}

	names := make(map[string]bool)
	for _, server := range c.Servers {
		if err := server.Validate(); err != nil {
			return fmt.Errorf("server %s: %w", server.Name, err)
		}

		if names[server.Name] {
			return fmt.Errorf("duplicate server name: %s", server.Name)
		}
		names[server.Name] = true
	}

	if c.ProjectContext != nil {
		if err := c.ProjectContext.Validate(); err != nil {
			return fmt.Errorf("project context validation failed: %w", err)
		}
	}

	if c.ProjectConfig != nil {
		if err := c.ProjectConfig.Validate(); err != nil {
			return fmt.Errorf("project config validation failed: %w", err)
		}
	}

	// Validate multi-server configuration
	if err := c.ValidateMultiServerConfig(); err != nil {
		return fmt.Errorf("multi-server configuration validation failed: %w", err)
	}

	// Validate configuration consistency
	if err := c.ValidateConsistency(); err != nil {
		return fmt.Errorf("configuration consistency validation failed: %w", err)
	}

	// Validate performance configuration
	if c.PerformanceConfig != nil {
		if err := c.PerformanceConfig.Validate(); err != nil {
			return fmt.Errorf("performance configuration validation failed: %w", err)
		}
	}

	return nil
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
	MultiRoot       bool                      `yaml:"multi_root" json:"multi_root"`
	LanguageRoots   map[string]string         `yaml:"language_roots" json:"language_roots"`
	SharedSettings  map[string]interface{}    `yaml:"shared_settings,omitempty" json:"shared_settings,omitempty"`
	CrossLanguageReferences bool             `yaml:"cross_language_references" json:"cross_language_references"`
	GlobalIgnores   []string                  `yaml:"global_ignores,omitempty" json:"global_ignores,omitempty"`
	IndexingStrategy string                   `yaml:"indexing_strategy,omitempty" json:"indexing_strategy,omitempty"`
}

type MultiLanguageProjectInfo struct {
	ProjectType      string                    `yaml:"project_type" json:"project_type"`
	RootDirectory    string                    `yaml:"root_directory" json:"root_directory"`
	WorkspaceRoot    string                    `yaml:"workspace_root,omitempty" json:"workspace_root,omitempty"`
	LanguageContexts []*LanguageContext        `yaml:"language_contexts" json:"language_contexts"`
	Frameworks       []*Framework              `yaml:"frameworks,omitempty" json:"frameworks,omitempty"`
	MonorepoLayout   *MonorepoLayout           `yaml:"monorepo_layout,omitempty" json:"monorepo_layout,omitempty"`
	DetectedAt       time.Time                 `yaml:"detected_at" json:"detected_at"`
	Metadata         map[string]interface{}    `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

type LanguageContext struct {
	Language         string                    `yaml:"language" json:"language"`
	Version          string                    `yaml:"version,omitempty" json:"version,omitempty"`
	FilePatterns     []string                  `yaml:"file_patterns" json:"file_patterns"`
	FileCount        int                       `yaml:"file_count" json:"file_count"`
	RootMarkers      []string                  `yaml:"root_markers" json:"root_markers"`
	RootPath         string                    `yaml:"root_path" json:"root_path"`
	Submodules       []string                  `yaml:"submodules,omitempty" json:"submodules,omitempty"`
	BuildSystem      string                    `yaml:"build_system,omitempty" json:"build_system,omitempty"`
	PackageManager   string                    `yaml:"package_manager,omitempty" json:"package_manager,omitempty"`
	Frameworks       []string                  `yaml:"frameworks,omitempty" json:"frameworks,omitempty"`
	TestFrameworks   []string                  `yaml:"test_frameworks,omitempty" json:"test_frameworks,omitempty"`
	LintingTools     []string                  `yaml:"linting_tools,omitempty" json:"linting_tools,omitempty"`
	Complexity       *LanguageComplexity       `yaml:"complexity,omitempty" json:"complexity,omitempty"`
}

type Framework struct {
	Name        string            `yaml:"name" json:"name"`
	Version     string            `yaml:"version,omitempty" json:"version,omitempty"`
	Language    string            `yaml:"language" json:"language"`
	ConfigFiles []string          `yaml:"config_files,omitempty" json:"config_files,omitempty"`
	Features    []string          `yaml:"features,omitempty" json:"features,omitempty"`
	Settings    map[string]interface{} `yaml:"settings,omitempty" json:"settings,omitempty"`
}

type MonorepoLayout struct {
	Strategy        string            `yaml:"strategy" json:"strategy"` // "language-separated", "mixed", "microservices"
	Workspaces      []string          `yaml:"workspaces,omitempty" json:"workspaces,omitempty"`
	SharedLibraries []string          `yaml:"shared_libraries,omitempty" json:"shared_libraries,omitempty"`
	BuildRoots      map[string]string `yaml:"build_roots,omitempty" json:"build_roots,omitempty"`
}

type LanguageComplexity struct {
	LinesOfCode     int     `yaml:"lines_of_code" json:"lines_of_code"`
	CyclomaticComplexity int `yaml:"cyclomatic_complexity,omitempty" json:"cyclomatic_complexity,omitempty"`
	DependencyCount int     `yaml:"dependency_count,omitempty" json:"dependency_count,omitempty"`
	NestingDepth    int     `yaml:"nesting_depth,omitempty" json:"nesting_depth,omitempty"`
}

// Multi-server configuration structures

type MultiServerConfig struct {
	Primary *ServerConfig `yaml:"primary" json:"primary"`
	Secondary []*ServerConfig `yaml:"secondary" json:"secondary"`
	SelectionStrategy string `yaml:"selection_strategy" json:"selection_strategy"` // "performance", "feature", "load_balance", "random"
	ConcurrentLimit int `yaml:"concurrent_limit" json:"concurrent_limit"`
	ResourceSharing bool `yaml:"resource_sharing" json:"resource_sharing"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval" json:"health_check_interval"`
	MaxRetries int `yaml:"max_retries" json:"max_retries"`
}

type LanguageServerPool struct {
	Language string `yaml:"language" json:"language"`
	Servers map[string]*ServerConfig `yaml:"servers" json:"servers"`
	DefaultServer string `yaml:"default_server" json:"default_server"`
	LoadBalancingConfig *LoadBalancingConfig `yaml:"load_balancing" json:"load_balancing"`
	ResourceLimits *ResourceLimits `yaml:"resource_limits" json:"resource_limits"`
}

type LoadBalancingConfig struct {
	Strategy string `yaml:"strategy" json:"strategy"` // "round_robin", "least_connections", "response_time", "resource_usage"
	HealthThreshold float64 `yaml:"health_threshold" json:"health_threshold"`
	WeightFactors map[string]float64 `yaml:"weight_factors" json:"weight_factors"`
}

type ResourceLimits struct {
	MaxMemoryMB int64 `yaml:"max_memory_mb" json:"max_memory_mb"`
	MaxConcurrentRequests int `yaml:"max_concurrent_requests" json:"max_concurrent_requests"`
	MaxProcesses int `yaml:"max_processes" json:"max_processes"`
	RequestTimeoutSeconds int `yaml:"request_timeout_seconds" json:"request_timeout_seconds"`
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
		Port:                  8080,
		Timeout:               "30s",
		MaxConcurrentRequests: 100,
		ProjectAware:          true,
		EnableConcurrentServers: len(mlc.ServerConfigs) > 1,
		MaxConcurrentServersPerLanguage: DEFAULT_MAX_CONCURRENT_SERVERS_PER_LANG,
		Servers: make([]ServerConfig, len(mlc.ServerConfigs)),
		LanguagePools: []LanguageServerPool{},
		GlobalMultiServerConfig: DefaultMultiServerConfig(),
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
	case "production":
		config.MaxConcurrentRequests = 200
		config.Timeout = "15s"
	case "analysis":
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
				gc.ProjectContext.Languages[i] = LanguageInfo{
					Language:     langCtx.Language,
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
	case "production":
		gc.MaxConcurrentRequests = 200
		gc.Timeout = "15s"
	case "analysis":
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
			"enabled": c.PerformanceConfig.Caching.Enabled,
			"global_ttl": c.PerformanceConfig.Caching.GlobalTTL.String(),
			"max_memory_usage_mb": c.PerformanceConfig.Caching.MaxMemoryUsage,
			"eviction_strategy": c.PerformanceConfig.Caching.EvictionStrategy,
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
			"global_timeout": c.PerformanceConfig.Timeouts.GlobalTimeout.String(),
			"default_timeout": c.PerformanceConfig.Timeouts.DefaultTimeout.String(),
			"connection_timeout": c.PerformanceConfig.Timeouts.ConnectionTimeout.String(),
		}
		metrics["timeouts"] = timeoutMetrics
	}
	
	// Large project metrics
	if c.PerformanceConfig.LargeProject != nil {
		largeProjectMetrics := map[string]interface{}{
			"auto_detect_size": c.PerformanceConfig.LargeProject.AutoDetectSize,
			"max_workspace_size_mb": c.PerformanceConfig.LargeProject.MaxWorkspaceSize,
			"indexing_strategy": c.PerformanceConfig.LargeProject.IndexingStrategy,
			"lazy_loading": c.PerformanceConfig.LargeProject.LazyLoading,
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
	// Check SmartRouterConfig compatibility with other settings
	if c.EnableSmartRouting && c.SmartRouterConfig != nil {
		// Validate method strategies
		for method, strategy := range c.SmartRouterConfig.MethodStrategies {
			if method == "" {
				return fmt.Errorf("empty method name in smart router method strategies")
			}
			if strategy == "" {
				return fmt.Errorf("empty strategy for method %s in smart router configuration", method)
			}
		}

		// Check circuit breaker configuration consistency
		if c.SmartRouterConfig.EnableCircuitBreaker {
			if c.SmartRouterConfig.CircuitBreakerThreshold <= 0 {
				return fmt.Errorf("circuit breaker threshold must be positive when circuit breaker is enabled: %d", c.SmartRouterConfig.CircuitBreakerThreshold)
			}

			if c.SmartRouterConfig.CircuitBreakerTimeout != "" {
				if _, err := time.ParseDuration(c.SmartRouterConfig.CircuitBreakerTimeout); err != nil {
					return fmt.Errorf("invalid circuit breaker timeout format: %s, error: %w", c.SmartRouterConfig.CircuitBreakerTimeout, err)
				}
			}
		}
	}

	// Validate ProjectContext consistency with servers
	if c.ProjectContext != nil {
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

		// Check if all required languages have available servers
		for reqLang := range requiredLanguages {
			if !availableLanguages[reqLang] {
				return fmt.Errorf("project context requires language %s but no server supports it", reqLang)
			}
		}

		// Validate required LSP servers exist
		serverNames := make(map[string]bool)
		for _, server := range c.Servers {
			serverNames[server.Name] = true  
		}

		for _, requiredLSP := range c.ProjectContext.RequiredLSPs {
			if !serverNames[requiredLSP] {
				return fmt.Errorf("project context requires LSP server %s but it is not configured", requiredLSP)
			}
		}
	}

	// Check performance configuration compatibility
	if c.PerformanceConfig != nil && c.PerformanceConfig.Enabled {
		// Validate timeout consistency with global settings
		if c.PerformanceConfig.Timeouts != nil {
			globalTimeout := c.PerformanceConfig.Timeouts.GlobalTimeout
			if globalTimeout > 0 {
				if configTimeout, err := time.ParseDuration(c.Timeout); err == nil {
					if globalTimeout < configTimeout {
						return fmt.Errorf("performance global timeout (%v) is less than config timeout (%v)", globalTimeout, configTimeout)
					}
				}
			}
		}

		// Validate memory limits with server limits
		if c.PerformanceConfig.ResourceManager != nil && c.PerformanceConfig.ResourceManager.MemoryLimits != nil {
			memLimits := c.PerformanceConfig.ResourceManager.MemoryLimits
			
			// Check consistency with language pools resource limits
			for _, pool := range c.LanguagePools {
				if pool.ResourceLimits != nil {
					if int64(pool.ResourceLimits.MaxMemoryMB) > memLimits.MaxHeapSize {
						return fmt.Errorf("language pool %s memory limit (%d MB) exceeds global max heap size (%d MB)", pool.Language, pool.ResourceLimits.MaxMemoryMB, memLimits.MaxHeapSize)
					}
				}
			}
		}
	}

	// Ensure timeout settings are reasonable
	if c.Timeout != "" {
		if duration, err := time.ParseDuration(c.Timeout); err != nil {
			return fmt.Errorf("invalid timeout format: %s, error: %w", c.Timeout, err)
		} else {
			if duration <= 0 {
				return fmt.Errorf("timeout must be positive: %v", duration)
			}
			if duration > 1*time.Hour {
				return fmt.Errorf("timeout is too large: %v, maximum allowed: 1h", duration)
			}
		}
	}

	// Validate concurrent requests consistency 
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

// AutoGenerateConfigFromPath generates a MultiLanguageConfig from a project path
func AutoGenerateConfigFromPath(projectPath string) (*MultiLanguageConfig, error) {
	generator := NewConfigGenerator()
	
	// Create basic project info for the path
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

