package setup

import (
	"time"
)

// EnhancedConfigurationTemplate represents a comprehensive YAML template configuration
// supporting advanced multi-server, language pools, and performance configurations
type EnhancedConfigurationTemplate struct {
	// Basic template metadata
	Name        string `yaml:"name,omitempty" json:"name,omitempty"`
	Version     string `yaml:"version,omitempty" json:"version,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Gateway configuration
	Port                     int    `yaml:"port,omitempty" json:"port,omitempty"`
	Timeout                  string `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	MaxConcurrentRequests    int    `yaml:"max_concurrent_requests,omitempty" json:"max_concurrent_requests,omitempty"`
	ProjectAware             bool   `yaml:"project_aware,omitempty" json:"project_aware,omitempty"`
	EnableConcurrentServers  bool   `yaml:"enable_concurrent_servers,omitempty" json:"enable_concurrent_servers,omitempty"`
	MaxConcurrentServersPerLanguage int `yaml:"max_concurrent_servers_per_language,omitempty" json:"max_concurrent_servers_per_language,omitempty"`
	EnableMetrics            bool   `yaml:"enable_metrics,omitempty" json:"enable_metrics,omitempty"`
	MetricsPort              int    `yaml:"metrics_port,omitempty" json:"metrics_port,omitempty"`
	LogLevel                 string `yaml:"log_level,omitempty" json:"log_level,omitempty"`

	// Multi-server configuration
	MultiServerConfig *MultiServerConfig `yaml:"multi_server_config,omitempty" json:"multi_server_config,omitempty"`

	// Project context
	ProjectContext *ProjectContext `yaml:"project_context,omitempty" json:"project_context,omitempty"`

	// Language pools
	LanguagePools []LanguagePool `yaml:"language_pools,omitempty" json:"language_pools,omitempty"`

	// Server configurations
	Servers []ServerInstance `yaml:"servers,omitempty" json:"servers,omitempty"`

	// Pool management configuration
	PoolManagement *PoolManagement `yaml:"pool_management,omitempty" json:"pool_management,omitempty"`

	// Routing configuration
	EnableSmartRouting bool                 `yaml:"enable_smart_routing,omitempty" json:"enable_smart_routing,omitempty"`
	EnableEnhancements bool                 `yaml:"enable_enhancements,omitempty" json:"enable_enhancements,omitempty"`
	SmartRouterConfig  *SmartRouterConfig   `yaml:"smart_router_config,omitempty" json:"smart_router_config,omitempty"`
	Routing            *RoutingConfig       `yaml:"routing,omitempty" json:"routing,omitempty"`

	// Performance configuration including SCIP
	PerformanceConfig *PerformanceConfig `yaml:"performance_config,omitempty" json:"performance_config,omitempty"`

	// Optimizations
	Optimizations map[string]*LanguageOptimizations `yaml:"optimizations,omitempty" json:"optimizations,omitempty"`

	// Workflow integration
	WorkflowIntegration *WorkflowIntegration `yaml:"workflow_integration,omitempty" json:"workflow_integration,omitempty"`

	// Logging configuration
	Logging *LoggingConfig `yaml:"logging,omitempty" json:"logging,omitempty"`

	// Monitoring and observability
	Monitoring *MonitoringConfig `yaml:"monitoring,omitempty" json:"monitoring,omitempty"`

	// Deployment configuration
	Deployment *DeploymentConfig `yaml:"deployment,omitempty" json:"deployment,omitempty"`

	// Performance targets
	PerformanceTargets *PerformanceTargets `yaml:"performance_targets,omitempty" json:"performance_targets,omitempty"`

	// Testing configuration
	Testing *TestingConfig `yaml:"testing,omitempty" json:"testing,omitempty"`

	// Migration notes
	MigrationNotes string `yaml:"migration_notes,omitempty" json:"migration_notes,omitempty"`

	// Template metadata
	CreatedAt time.Time `yaml:"created_at,omitempty" json:"created_at,omitempty"`
}

// LanguagePool represents a pool of language servers for a specific language
type LanguagePool struct {
	Language      string                       `yaml:"language" json:"language"`
	DefaultServer string                       `yaml:"default_server" json:"default_server"`
	Servers       map[string]*PoolServerConfig `yaml:"servers" json:"servers"`
	LoadBalancing *LoadBalancingConfig         `yaml:"load_balancing,omitempty" json:"load_balancing,omitempty"`
	ResourceLimits *ResourceLimits             `yaml:"resource_limits,omitempty" json:"resource_limits,omitempty"`
}

// PoolServerConfig represents a server configuration within a language pool
type PoolServerConfig struct {
	Name                   string                 `yaml:"name" json:"name"`
	Languages              []string               `yaml:"languages" json:"languages"`
	Command                string                 `yaml:"command" json:"command"`
	Args                   []string               `yaml:"args,omitempty" json:"args,omitempty"`
	Transport              string                 `yaml:"transport" json:"transport"`
	Priority               int                    `yaml:"priority,omitempty" json:"priority,omitempty"`
	Weight                 float64                `yaml:"weight,omitempty" json:"weight,omitempty"`
	MaxConcurrentRequests  int                    `yaml:"max_concurrent_requests,omitempty" json:"max_concurrent_requests,omitempty"`
	RootMarkers            []string               `yaml:"root_markers,omitempty" json:"root_markers,omitempty"`
	Settings               map[string]interface{} `yaml:"settings,omitempty" json:"settings,omitempty"`
	Environment            map[string]string      `yaml:"environment,omitempty" json:"environment,omitempty"`
}

// ServerInstance represents a complete server instance configuration
type ServerInstance struct {
	Name                     string                    `yaml:"name" json:"name"`
	Languages                []string                  `yaml:"languages" json:"languages"`
	Command                  string                    `yaml:"command" json:"command"`
	Args                     []string                  `yaml:"args,omitempty" json:"args,omitempty"`
	Transport                string                    `yaml:"transport" json:"transport"`
	Priority                 int                       `yaml:"priority,omitempty" json:"priority,omitempty"`
	Weight                   float64                   `yaml:"weight,omitempty" json:"weight,omitempty"`
	RootMarkers              []string                  `yaml:"root_markers,omitempty" json:"root_markers,omitempty"`
	PoolConfig               *PoolConfig               `yaml:"pool_config,omitempty" json:"pool_config,omitempty"`
	ConnectionSettings       *ConnectionSettings       `yaml:"connection_settings,omitempty" json:"connection_settings,omitempty"`
	HealthCheckSettings      *HealthCheckSettings      `yaml:"health_check_settings,omitempty" json:"health_check_settings,omitempty"`
	Settings                 map[string]interface{}    `yaml:"settings,omitempty" json:"settings,omitempty"`
	Environment              map[string]string         `yaml:"environment,omitempty" json:"environment,omitempty"`
	InitializationTimeout    string                    `yaml:"initialization_timeout,omitempty" json:"initialization_timeout,omitempty"`
}

// PoolConfig represents dynamic pool configuration settings
type PoolConfig struct {
	MinSize              int     `yaml:"min_size,omitempty" json:"min_size,omitempty"`
	MaxSize              int     `yaml:"max_size,omitempty" json:"max_size,omitempty"`
	WarmupSize           int     `yaml:"warmup_size,omitempty" json:"warmup_size,omitempty"`
	EnableDynamicSizing  bool    `yaml:"enable_dynamic_sizing,omitempty" json:"enable_dynamic_sizing,omitempty"`
	TargetUtilization    float64 `yaml:"target_utilization,omitempty" json:"target_utilization,omitempty"`
	ScaleUpThreshold     float64 `yaml:"scale_up_threshold,omitempty" json:"scale_up_threshold,omitempty"`
	ScaleDownThreshold   float64 `yaml:"scale_down_threshold,omitempty" json:"scale_down_threshold,omitempty"`
	MaxLifetime          string  `yaml:"max_lifetime,omitempty" json:"max_lifetime,omitempty"`
	IdleTimeout          string  `yaml:"idle_timeout,omitempty" json:"idle_timeout,omitempty"`
	HealthCheckInterval  string  `yaml:"health_check_interval,omitempty" json:"health_check_interval,omitempty"`
	MaxRetries           int     `yaml:"max_retries,omitempty" json:"max_retries,omitempty"`
	BaseDelay            string  `yaml:"base_delay,omitempty" json:"base_delay,omitempty"`
	CircuitTimeout       string  `yaml:"circuit_timeout,omitempty" json:"circuit_timeout,omitempty"`
	MemoryLimitMB        int     `yaml:"memory_limit_mb,omitempty" json:"memory_limit_mb,omitempty"`
	CPULimitPercent      float64 `yaml:"cpu_limit_percent,omitempty" json:"cpu_limit_percent,omitempty"`
	TransportType        string  `yaml:"transport_type,omitempty" json:"transport_type,omitempty"`
	CustomConfig         map[string]interface{} `yaml:"custom_config,omitempty" json:"custom_config,omitempty"`
}

// MultiServerConfig represents global multi-server configuration
type MultiServerConfig struct {
	SelectionStrategy        string `yaml:"selection_strategy,omitempty" json:"selection_strategy,omitempty"`
	ConcurrentLimit          int    `yaml:"concurrent_limit,omitempty" json:"concurrent_limit,omitempty"`
	ResourceSharing          bool   `yaml:"resource_sharing,omitempty" json:"resource_sharing,omitempty"`
	HealthCheckInterval      string `yaml:"health_check_interval,omitempty" json:"health_check_interval,omitempty"`
	MaxRetries               int    `yaml:"max_retries,omitempty" json:"max_retries,omitempty"`
	EnableCrossLanguageCaching bool `yaml:"enable_cross_language_caching,omitempty" json:"enable_cross_language_caching,omitempty"`
	CacheTTL                 string `yaml:"cache_ttl,omitempty" json:"cache_ttl,omitempty"`
}

// SmartRouterConfig represents intelligent routing configuration
type SmartRouterConfig struct {
	DefaultStrategy               string            `yaml:"default_strategy,omitempty" json:"default_strategy,omitempty"`
	MethodStrategies              map[string]string `yaml:"method_strategies,omitempty" json:"method_strategies,omitempty"`
	EnablePerformanceMonitoring   bool              `yaml:"enable_performance_monitoring,omitempty" json:"enable_performance_monitoring,omitempty"`
	EnableCircuitBreaker          bool              `yaml:"enable_circuit_breaker,omitempty" json:"enable_circuit_breaker,omitempty"`
	CircuitBreakerThreshold       int               `yaml:"circuit_breaker_threshold,omitempty" json:"circuit_breaker_threshold,omitempty"`
	CircuitBreakerTimeout         string            `yaml:"circuit_breaker_timeout,omitempty" json:"circuit_breaker_timeout,omitempty"`
}

// LoadBalancingConfig represents load balancing configuration
type LoadBalancingConfig struct {
	Strategy        string             `yaml:"strategy,omitempty" json:"strategy,omitempty"`
	HealthThreshold float64            `yaml:"health_threshold,omitempty" json:"health_threshold,omitempty"`
	WeightFactors   map[string]float64 `yaml:"weight_factors,omitempty" json:"weight_factors,omitempty"`
}

// ResourceLimits represents resource limits for pools or servers
type ResourceLimits struct {
	MaxMemoryMB           int `yaml:"max_memory_mb,omitempty" json:"max_memory_mb,omitempty"`
	MaxConcurrentRequests int `yaml:"max_concurrent_requests,omitempty" json:"max_concurrent_requests,omitempty"`
	MaxProcesses          int `yaml:"max_processes,omitempty" json:"max_processes,omitempty"`
	RequestTimeoutSeconds int `yaml:"request_timeout_seconds,omitempty" json:"request_timeout_seconds,omitempty"`
}

// ConnectionSettings represents connection-specific settings
type ConnectionSettings struct {
	BufferSize     int    `yaml:"buffer_size,omitempty" json:"buffer_size,omitempty"`
	ProcessTimeout string `yaml:"process_timeout,omitempty" json:"process_timeout,omitempty"`
}

// HealthCheckSettings represents health check configuration
type HealthCheckSettings struct {
	Enabled              bool   `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Interval             string `yaml:"interval,omitempty" json:"interval,omitempty"`
	Timeout              string `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	FailureThreshold     int    `yaml:"failure_threshold,omitempty" json:"failure_threshold,omitempty"`
	SuccessThreshold     int    `yaml:"success_threshold,omitempty" json:"success_threshold,omitempty"`
	Method               string `yaml:"method,omitempty" json:"method,omitempty"`
	EnableAutoRestart    bool   `yaml:"enable_auto_restart,omitempty" json:"enable_auto_restart,omitempty"`
	RestartDelay         string `yaml:"restart_delay,omitempty" json:"restart_delay,omitempty"`
	MaxConsecutiveFails  int    `yaml:"max_consecutive_fails,omitempty" json:"max_consecutive_fails,omitempty"`
}

// ProjectContext represents project context information
type ProjectContext struct {
	ProjectType                   string                     `yaml:"project_type,omitempty" json:"project_type,omitempty"`
	ArchitectureStyle             string                     `yaml:"architecture_style,omitempty" json:"architecture_style,omitempty"`
	ServiceCount                  int                        `yaml:"service_count,omitempty" json:"service_count,omitempty"`
	DeploymentPattern             string                     `yaml:"deployment_pattern,omitempty" json:"deployment_pattern,omitempty"`
	Languages                     []LanguageInfo             `yaml:"languages,omitempty" json:"languages,omitempty"`
	EnableCrossLanguageNavigation bool                       `yaml:"enable_cross_language_navigation,omitempty" json:"enable_cross_language_navigation,omitempty"`
	EnablePolyglotRefactoring     bool                       `yaml:"enable_polyglot_refactoring,omitempty" json:"enable_polyglot_refactoring,omitempty"`
	EnableDependencyAnalysis      bool                       `yaml:"enable_dependency_analysis,omitempty" json:"enable_dependency_analysis,omitempty"`
	EnableServiceMeshIntegration  bool                       `yaml:"enable_service_mesh_integration,omitempty" json:"enable_service_mesh_integration,omitempty"`
	LanguageRoles                 map[string][]string        `yaml:"language_roles,omitempty" json:"language_roles,omitempty"`
}

// LanguageInfo represents language-specific information
type LanguageInfo struct {
	Language     string   `yaml:"language" json:"language"`
	Frameworks   []string `yaml:"frameworks,omitempty" json:"frameworks,omitempty"`
	FilePatterns []string `yaml:"file_patterns,omitempty" json:"file_patterns,omitempty"`
	RootMarkers  []string `yaml:"root_markers,omitempty" json:"root_markers,omitempty"`
}

// PoolManagement represents global pool management configuration
type PoolManagement struct {
	EnableGlobalMonitoring bool                      `yaml:"enable_global_monitoring,omitempty" json:"enable_global_monitoring,omitempty"`
	MonitoringInterval     string                    `yaml:"monitoring_interval,omitempty" json:"monitoring_interval,omitempty"`
	MaxTotalConnections    int                       `yaml:"max_total_connections,omitempty" json:"max_total_connections,omitempty"`
	MaxTotalMemoryMB       int                       `yaml:"max_total_memory_mb,omitempty" json:"max_total_memory_mb,omitempty"`
	MaxTotalCPUPercent     float64                   `yaml:"max_total_cpu_percent,omitempty" json:"max_total_cpu_percent,omitempty"`
	EnableOrphanCleanup    bool                      `yaml:"enable_orphan_cleanup,omitempty" json:"enable_orphan_cleanup,omitempty"`
	CleanupInterval        string                    `yaml:"cleanup_interval,omitempty" json:"cleanup_interval,omitempty"`
	EnableDetailedMetrics  bool                      `yaml:"enable_detailed_metrics,omitempty" json:"enable_detailed_metrics,omitempty"`
	MetricsRetention       string                    `yaml:"metrics_retention,omitempty" json:"metrics_retention,omitempty"`
	MetricsGranularity     string                    `yaml:"metrics_granularity,omitempty" json:"metrics_granularity,omitempty"`
	GlobalCircuitBreaker   *GlobalCircuitBreaker     `yaml:"global_circuit_breaker,omitempty" json:"global_circuit_breaker,omitempty"`
	EmergencyMode          *EmergencyMode            `yaml:"emergency_mode,omitempty" json:"emergency_mode,omitempty"`
}

// GlobalCircuitBreaker represents global circuit breaker configuration
type GlobalCircuitBreaker struct {
	Enabled                  bool    `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	FailureThreshold         float64 `yaml:"failure_threshold,omitempty" json:"failure_threshold,omitempty"`
	RecoveryTimeout          string  `yaml:"recovery_timeout,omitempty" json:"recovery_timeout,omitempty"`
	EnableLanguageIsolation  bool    `yaml:"enable_language_isolation,omitempty" json:"enable_language_isolation,omitempty"`
}

// EmergencyMode represents emergency mode configuration
type EmergencyMode struct {
	Enabled               bool     `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	TriggerErrorRate      float64  `yaml:"trigger_error_rate,omitempty" json:"trigger_error_rate,omitempty"`
	TriggerMemoryPercent  float64  `yaml:"trigger_memory_percent,omitempty" json:"trigger_memory_percent,omitempty"`
	TriggerCPUPercent     float64  `yaml:"trigger_cpu_percent,omitempty" json:"trigger_cpu_percent,omitempty"`
	Actions               []string `yaml:"actions,omitempty" json:"actions,omitempty"`
}

// RoutingConfig represents advanced routing configuration
type RoutingConfig struct {
	Strategy                    string                     `yaml:"strategy,omitempty" json:"strategy,omitempty"`
	EnableCaching               bool                       `yaml:"enable_caching,omitempty" json:"enable_caching,omitempty"`
	CacheTTL                    string                     `yaml:"cache_ttl,omitempty" json:"cache_ttl,omitempty"`
	EnableRequestClassification bool                       `yaml:"enable_request_classification,omitempty" json:"enable_request_classification,omitempty"`
	CrossLanguageRules          []CrossLanguageRule        `yaml:"cross_language_rules,omitempty" json:"cross_language_rules,omitempty"`
	RequestDistribution         *RequestDistribution       `yaml:"request_distribution,omitempty" json:"request_distribution,omitempty"`
}

// CrossLanguageRule represents cross-language routing rules
type CrossLanguageRule struct {
	FromLanguage string `yaml:"from_language" json:"from_language"`
	ToLanguage   string `yaml:"to_language" json:"to_language"`
	Condition    string `yaml:"condition" json:"condition"`
	Priority     string `yaml:"priority" json:"priority"`
}

// RequestDistribution represents request distribution configuration
type RequestDistribution struct {
	EnableWorkloadClassification bool `yaml:"enable_workload_classification,omitempty" json:"enable_workload_classification,omitempty"`
	EnableResourceAwareness      bool `yaml:"enable_resource_awareness,omitempty" json:"enable_resource_awareness,omitempty"`
	EnableLatencyOptimization    bool `yaml:"enable_latency_optimization,omitempty" json:"enable_latency_optimization,omitempty"`
}

// PerformanceConfig represents comprehensive performance configuration including SCIP
type PerformanceConfig struct {
	SCIP *SCIPConfig `yaml:"scip,omitempty" json:"scip,omitempty"`
}

// SCIPConfig represents SCIP (SCIP Code Intelligence Protocol) configuration
type SCIPConfig struct {
	Enabled              bool                           `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	IndexPath            string                         `yaml:"index_path,omitempty" json:"index_path,omitempty"`
	AutoRefresh          bool                           `yaml:"auto_refresh,omitempty" json:"auto_refresh,omitempty"`
	RefreshInterval      string                         `yaml:"refresh_interval,omitempty" json:"refresh_interval,omitempty"`
	FallbackToLSP        bool                           `yaml:"fallback_to_lsp,omitempty" json:"fallback_to_lsp,omitempty"`
	FallbackTimeout      string                         `yaml:"fallback_timeout,omitempty" json:"fallback_timeout,omitempty"`
	Cache                *SCIPCache                     `yaml:"cache,omitempty" json:"cache,omitempty"`
	CrossLanguage        *SCIPCrossLanguage             `yaml:"cross_language,omitempty" json:"cross_language,omitempty"`
	LanguageSettings     map[string]*SCIPLanguageConfig `yaml:"language_settings,omitempty" json:"language_settings,omitempty"`
	Monitoring           *SCIPMonitoring                `yaml:"monitoring,omitempty" json:"monitoring,omitempty"`
	SmartRouting         *SCIPSmartRouting              `yaml:"smart_routing,omitempty" json:"smart_routing,omitempty"`
	WorkflowIntegration  *SCIPWorkflowIntegration       `yaml:"workflow_integration,omitempty" json:"workflow_integration,omitempty"`
}

// SCIPCache represents SCIP caching configuration
type SCIPCache struct {
	Enabled                     bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	TTL                         string `yaml:"ttl,omitempty" json:"ttl,omitempty"`
	MaxSize                     int  `yaml:"max_size,omitempty" json:"max_size,omitempty"`
	EnableModulePartitioning    bool `yaml:"enable_module_partitioning,omitempty" json:"enable_module_partitioning,omitempty"`
	CacheByGoVersion            bool `yaml:"cache_by_go_version,omitempty" json:"cache_by_go_version,omitempty"`
	EnableCrossLanguageCache    bool `yaml:"enable_cross_language_cache,omitempty" json:"enable_cross_language_cache,omitempty"`
	CachePartitionByLanguage    bool `yaml:"cache_partition_by_language,omitempty" json:"cache_partition_by_language,omitempty"`
}

// SCIPCrossLanguage represents cross-language intelligence features
type SCIPCrossLanguage struct {
	EnableNavigation      bool `yaml:"enable_navigation,omitempty" json:"enable_navigation,omitempty"`
	EnableDependencyGraph bool `yaml:"enable_dependency_graph,omitempty" json:"enable_dependency_graph,omitempty"`
	EnableAPIDiscovery    bool `yaml:"enable_api_discovery,omitempty" json:"enable_api_discovery,omitempty"`
	EnableInterfaceMatching bool `yaml:"enable_interface_matching,omitempty" json:"enable_interface_matching,omitempty"`
	EnableTypeBridging    bool `yaml:"enable_type_bridging,omitempty" json:"enable_type_bridging,omitempty"`
}

// SCIPLanguageConfig represents language-specific SCIP configuration
type SCIPLanguageConfig struct {
	Enabled                         bool                      `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	IndexingStrategies              map[string]*IndexingStrategy `yaml:"indexing_strategies,omitempty" json:"indexing_strategies,omitempty"`
	IndexCommand                    []string                  `yaml:"index_command,omitempty" json:"index_command,omitempty"`
	IndexTimeout                    string                    `yaml:"index_timeout,omitempty" json:"index_timeout,omitempty"`
	IndexConcurrency                int                       `yaml:"index_concurrency,omitempty" json:"index_concurrency,omitempty"`
	VirtualEnvSupport               bool                      `yaml:"virtual_env_support,omitempty" json:"virtual_env_support,omitempty"`
	EnableJupyterAnalysis           bool                      `yaml:"enable_jupyter_analysis,omitempty" json:"enable_jupyter_analysis,omitempty"`
	EnableNodeModulesAnalysis       bool                      `yaml:"enable_node_modules_analysis,omitempty" json:"enable_node_modules_analysis,omitempty"`
	EnableFrontendFrameworkAnalysis bool                      `yaml:"enable_frontend_framework_analysis,omitempty" json:"enable_frontend_framework_analysis,omitempty"`
	EnableSpringFrameworkAnalysis   bool                      `yaml:"enable_spring_framework_analysis,omitempty" json:"enable_spring_framework_analysis,omitempty"`
	EnableJPAAnalysis               string                    `yaml:"enable_jpa_analysis,omitempty" json:"enable_jpa_analysis,omitempty"`
	JVMArgs                         []string                  `yaml:"jvm_args,omitempty" json:"jvm_args,omitempty"`
	EnableCargoWorkspaceAnalysis    bool                      `yaml:"enable_cargo_workspace_analysis,omitempty" json:"enable_cargo_workspace_analysis,omitempty"`
	EnableAsyncAnalysis             bool                      `yaml:"enable_async_analysis,omitempty" json:"enable_async_analysis,omitempty"`
	EnableCMakeAnalysis             bool                      `yaml:"enable_cmake_analysis,omitempty" json:"enable_cmake_analysis,omitempty"`
	EnableTemplateAnalysis          bool                      `yaml:"enable_template_analysis,omitempty" json:"enable_template_analysis,omitempty"`
	AdvancedFeatures                map[string]interface{}    `yaml:"advanced_features,omitempty" json:"advanced_features,omitempty"`
	BuildIntegration                map[string]interface{}    `yaml:"build_integration,omitempty" json:"build_integration,omitempty"`
	Environments                    map[string]interface{}    `yaml:"environments,omitempty" json:"environments,omitempty"`
	DataScience                     map[string]interface{}    `yaml:"data_science,omitempty" json:"data_science,omitempty"`
	Fullstack                       map[string]interface{}    `yaml:"fullstack,omitempty" json:"fullstack,omitempty"`
	SpringBoot                      map[string]interface{}    `yaml:"spring_boot,omitempty" json:"spring_boot,omitempty"`
	Systems                         map[string]interface{}    `yaml:"systems,omitempty" json:"systems,omitempty"`
	HPC                             map[string]interface{}    `yaml:"hpc,omitempty" json:"hpc,omitempty"`
	Microservices                   map[string]interface{}    `yaml:"microservices,omitempty" json:"microservices,omitempty"`
}

// IndexingStrategy represents different indexing strategies for languages
type IndexingStrategy struct {
	IndexCommand                       []string `yaml:"index_command,omitempty" json:"index_command,omitempty"`
	IndexTimeout                       string   `yaml:"index_timeout,omitempty" json:"index_timeout,omitempty"`
	IndexConcurrency                   int      `yaml:"index_concurrency,omitempty" json:"index_concurrency,omitempty"`
	EnableCrossModuleAnalysis          bool     `yaml:"enable_cross_module_analysis,omitempty" json:"enable_cross_module_analysis,omitempty"`
	EnableVendorAnalysis               bool     `yaml:"enable_vendor_analysis,omitempty" json:"enable_vendor_analysis,omitempty"`
	EnableTestAnalysis                 bool     `yaml:"enable_test_analysis,omitempty" json:"enable_test_analysis,omitempty"`
	BuildFlags                         []string `yaml:"build_flags,omitempty" json:"build_flags,omitempty"`
	EnableBenchmarkAnalysis            bool     `yaml:"enable_benchmark_analysis,omitempty" json:"enable_benchmark_analysis,omitempty"`
	EnableFuzzTestAnalysis             bool     `yaml:"enable_fuzz_test_analysis,omitempty" json:"enable_fuzz_test_analysis,omitempty"`
	EnableBuildConstraintAnalysis      bool     `yaml:"enable_build_constraint_analysis,omitempty" json:"enable_build_constraint_analysis,omitempty"`
	EnableCrossCompilationAnalysis     bool     `yaml:"enable_cross_compilation_analysis,omitempty" json:"enable_cross_compilation_analysis,omitempty"`
}

// SCIPMonitoring represents SCIP monitoring configuration
type SCIPMonitoring struct {
	Enabled                              bool                          `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	EnableLanguageSpecificMetrics       bool                          `yaml:"enable_language_specific_metrics,omitempty" json:"enable_language_specific_metrics,omitempty"`
	EnableCrossLanguagePerformanceTracking bool                      `yaml:"enable_cross_language_performance_tracking,omitempty" json:"enable_cross_language_performance_tracking,omitempty"`
	LanguageMetrics                      map[string]map[string]bool    `yaml:"language_metrics,omitempty" json:"language_metrics,omitempty"`
	BuildMetrics                         map[string]bool               `yaml:"build_metrics,omitempty" json:"build_metrics,omitempty"`
	RuntimeMetrics                       map[string]bool               `yaml:"runtime_metrics,omitempty" json:"runtime_metrics,omitempty"`
	ToolchainMetrics                     map[string]bool               `yaml:"toolchain_metrics,omitempty" json:"toolchain_metrics,omitempty"`
}

// SCIPSmartRouting represents SCIP smart routing configuration
type SCIPSmartRouting struct {
	Enabled        bool                          `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	RoutingRules   []SCIPRoutingRule             `yaml:"routing_rules,omitempty" json:"routing_rules,omitempty"`
	RoutingStrategies map[string]interface{}     `yaml:"routing_strategies,omitempty" json:"routing_strategies,omitempty"`
}

// SCIPRoutingRule represents SCIP routing rules
type SCIPRoutingRule struct {
	SourceLanguage  string   `yaml:"source_language" json:"source_language"`
	TargetLanguages []string `yaml:"target_languages" json:"target_languages"`
	Condition       string   `yaml:"condition" json:"condition"`
	Priority        string   `yaml:"priority" json:"priority"`
}

// SCIPWorkflowIntegration represents SCIP workflow integration
type SCIPWorkflowIntegration struct {
	GitHooks    map[string][]string     `yaml:"git_hooks,omitempty" json:"git_hooks,omitempty"`
	IDEFeatures map[string]bool         `yaml:"ide_features,omitempty" json:"ide_features,omitempty"`
	CICD        map[string]bool         `yaml:"ci_cd,omitempty" json:"ci_cd,omitempty"`
}

// LanguageOptimizations represents language-specific optimizations
type LanguageOptimizations struct {
	BuildTools []BuildTool     `yaml:"build_tools,omitempty" json:"build_tools,omitempty"`
	Linting    []LintingTool   `yaml:"linting,omitempty" json:"linting,omitempty"`
	Testing    []TestingTool   `yaml:"testing,omitempty" json:"testing,omitempty"`
	Profiling  []ProfilingTool `yaml:"profiling,omitempty" json:"profiling,omitempty"`
	Frameworks map[string]interface{} `yaml:"frameworks,omitempty" json:"frameworks,omitempty"`
}

// BuildTool represents build tool configuration
type BuildTool struct {
	Name     string                 `yaml:"name" json:"name"`
	Enabled  bool                   `yaml:"enabled" json:"enabled"`
	Settings map[string]interface{} `yaml:"settings,omitempty" json:"settings,omitempty"`
}

// LintingTool represents linting tool configuration
type LintingTool struct {
	Name       string                 `yaml:"name" json:"name"`
	Enabled    bool                   `yaml:"enabled" json:"enabled"`
	ConfigFile string                 `yaml:"config_file,omitempty" json:"config_file,omitempty"`
	Settings   map[string]interface{} `yaml:"settings,omitempty" json:"settings,omitempty"`
}

// TestingTool represents testing tool configuration
type TestingTool struct {
	Name     string                 `yaml:"name" json:"name"`
	Enabled  bool                   `yaml:"enabled" json:"enabled"`
	Settings map[string]interface{} `yaml:"settings,omitempty" json:"settings,omitempty"`
}

// ProfilingTool represents profiling tool configuration
type ProfilingTool struct {
	Name     string                 `yaml:"name" json:"name"`
	Enabled  bool                   `yaml:"enabled" json:"enabled"`
	Settings map[string]interface{} `yaml:"settings,omitempty" json:"settings,omitempty"`
}

// WorkflowIntegration represents workflow integration configuration
type WorkflowIntegration struct {
	Git    *GitIntegration    `yaml:"git,omitempty" json:"git,omitempty"`
	VSCode *VSCodeIntegration `yaml:"vscode,omitempty" json:"vscode,omitempty"`
}

// GitIntegration represents Git integration configuration
type GitIntegration struct {
	Hooks map[string][]string `yaml:"hooks,omitempty" json:"hooks,omitempty"`
}

// VSCodeIntegration represents VS Code integration configuration
type VSCodeIntegration struct {
	Settings map[string]interface{} `yaml:"settings,omitempty" json:"settings,omitempty"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	GoServers            bool              `yaml:"go_servers,omitempty" json:"go_servers,omitempty"`
	BuildIntegration     bool              `yaml:"build_integration,omitempty" json:"build_integration,omitempty"`
	PerformanceMetrics   bool              `yaml:"performance_metrics,omitempty" json:"performance_metrics,omitempty"`
	SCIPIndexing         bool              `yaml:"scip_indexing,omitempty" json:"scip_indexing,omitempty"`
	PoolEvents           bool              `yaml:"pool_events,omitempty" json:"pool_events,omitempty"`
	ConnectionLifecycle  bool              `yaml:"connection_lifecycle,omitempty" json:"connection_lifecycle,omitempty"`
	HealthChecks         bool              `yaml:"health_checks,omitempty" json:"health_checks,omitempty"`
	CircuitBreakerEvents bool              `yaml:"circuit_breaker_events,omitempty" json:"circuit_breaker_events,omitempty"`
	CrossLanguageEvents  bool              `yaml:"cross_language_events,omitempty" json:"cross_language_events,omitempty"`
	Levels               map[string]string `yaml:"levels,omitempty" json:"levels,omitempty"`
}

// MonitoringConfig represents monitoring configuration
type MonitoringConfig struct {
	EnableDistributedTracing    bool                       `yaml:"enable_distributed_tracing,omitempty" json:"enable_distributed_tracing,omitempty"`
	EnableServiceMeshMetrics    bool                       `yaml:"enable_service_mesh_metrics,omitempty" json:"enable_service_mesh_metrics,omitempty"`
	EnableCrossLanguageProfiling bool                      `yaml:"enable_cross_language_profiling,omitempty" json:"enable_cross_language_profiling,omitempty"`
	LanguageMetrics             map[string][]string        `yaml:"language_metrics,omitempty" json:"language_metrics,omitempty"`
	EnableSecurityMonitoring    bool                       `yaml:"enable_security_monitoring,omitempty" json:"enable_security_monitoring,omitempty"`
	EnableComplianceMonitoring  bool                       `yaml:"enable_compliance_monitoring,omitempty" json:"enable_compliance_monitoring,omitempty"`
	EnableCostMonitoring        bool                       `yaml:"enable_cost_monitoring,omitempty" json:"enable_cost_monitoring,omitempty"`
}

// DeploymentConfig represents deployment configuration
type DeploymentConfig struct {
	ContainerOrchestration string                     `yaml:"container_orchestration,omitempty" json:"container_orchestration,omitempty"`
	ServiceMesh            string                     `yaml:"service_mesh,omitempty" json:"service_mesh,omitempty"`
	MonitoringStack        string                     `yaml:"monitoring_stack,omitempty" json:"monitoring_stack,omitempty"`
	LoggingStack           string                     `yaml:"logging_stack,omitempty" json:"logging_stack,omitempty"`
	DeploymentStrategies   map[string]bool            `yaml:"deployment_strategies,omitempty" json:"deployment_strategies,omitempty"`
}

// PerformanceTargets represents performance targets configuration
type PerformanceTargets struct {
	InitializationTimeMax            string           `yaml:"initialization_time_max,omitempty" json:"initialization_time_max,omitempty"`
	ResponseTimeP95                  string           `yaml:"response_time_p95,omitempty" json:"response_time_p95,omitempty"`
	MemoryUsageMax                   string           `yaml:"memory_usage_max,omitempty" json:"memory_usage_max,omitempty"`
	CPUUsageMax                      string           `yaml:"cpu_usage_max,omitempty" json:"cpu_usage_max,omitempty"`
	ConcurrentRequestsMax            int              `yaml:"concurrent_requests_max,omitempty" json:"concurrent_requests_max,omitempty"`
	CrossLanguageNavigationTime      string           `yaml:"cross_language_navigation_time,omitempty" json:"cross_language_navigation_time,omitempty"`
	SCIPPerformance                  *SCIPPerformanceTargets `yaml:"scip_performance,omitempty" json:"scip_performance,omitempty"`
}

// SCIPPerformanceTargets represents SCIP performance targets
type SCIPPerformanceTargets struct {
	IndexQueryTimeP95           string  `yaml:"index_query_time_p95,omitempty" json:"index_query_time_p95,omitempty"`
	CrossLanguageQueryTimeP95   string  `yaml:"cross_language_query_time_p95,omitempty" json:"cross_language_query_time_p95,omitempty"`
	IndexBuildTimeMax           string  `yaml:"index_build_time_max,omitempty" json:"index_build_time_max,omitempty"`
	CacheHitRateMin             float64 `yaml:"cache_hit_rate_min,omitempty" json:"cache_hit_rate_min,omitempty"`
	LanguageCoverageMin         float64 `yaml:"language_coverage_min,omitempty" json:"language_coverage_min,omitempty"`
}

// TestingConfig represents testing configuration
type TestingConfig struct {
	Scenarios              []string `yaml:"scenarios,omitempty" json:"scenarios,omitempty"`
	TimeoutSeconds         int      `yaml:"timeout_seconds,omitempty" json:"timeout_seconds,omitempty"`
	ParallelExecution      bool     `yaml:"parallel_execution,omitempty" json:"parallel_execution,omitempty"`
	EnableContractTesting  bool     `yaml:"enable_contract_testing,omitempty" json:"enable_contract_testing,omitempty"`
	EnablePolyglotIntegrationTests bool `yaml:"enable_polyglot_integration_tests,omitempty" json:"enable_polyglot_integration_tests,omitempty"`
	EnablePerformanceRegressionTests bool `yaml:"enable_performance_regression_tests,omitempty" json:"enable_performance_regression_tests,omitempty"`
}