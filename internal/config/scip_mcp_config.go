package config

import (
	"fmt"
	"time"
)

// SCIPMCPConfiguration extends the existing configuration system with SCIP-enhanced MCP support
type SCIPMCPConfiguration struct {
	// Core feature toggles
	Enabled                     bool `yaml:"enabled" json:"enabled"`
	EnableSCIPEnhancements      bool `yaml:"enable_scip_enhancements" json:"enable_scip_enhancements"`
	EnableFallbackMode          bool `yaml:"enable_fallback_mode" json:"enable_fallback_mode"`
	EnablePerformanceMonitoring bool `yaml:"enable_performance_monitoring" json:"enable_performance_monitoring"`
	EnableHealthChecking        bool `yaml:"enable_health_checking" json:"enable_health_checking"`

	// Performance configuration
	PerformanceConfig *SCIPMCPPerformanceConfig `yaml:"performance_config,omitempty" json:"performance_config,omitempty"`

	// Tool-specific configurations
	ToolConfigs map[string]*SCIPMCPToolConfig `yaml:"tool_configs,omitempty" json:"tool_configs,omitempty"`

	// Monitoring and alerting
	MonitoringConfig *SCIPMCPMonitoringConfig `yaml:"monitoring_config,omitempty" json:"monitoring_config,omitempty"`

	// Fallback behavior
	FallbackConfig *SCIPMCPFallbackConfig `yaml:"fallback_config,omitempty" json:"fallback_config,omitempty"`

	// Auto-configuration
	AutoConfig *SCIPMCPAutoConfigSettings `yaml:"auto_config,omitempty" json:"auto_config,omitempty"`

	// Integration settings
	IntegrationConfig *SCIPMCPIntegrationConfig `yaml:"integration_config,omitempty" json:"integration_config,omitempty"`

	// Version and metadata
	Version   string    `yaml:"version,omitempty" json:"version,omitempty"`
	CreatedAt time.Time `yaml:"created_at,omitempty" json:"created_at,omitempty"`
	UpdatedAt time.Time `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
}

// SCIPMCPPerformanceConfig defines performance targets and limits for SCIP-enhanced MCP tools
type SCIPMCPPerformanceConfig struct {
	// Target response times (from requirements)
	TargetSymbolSearchTime     time.Duration `yaml:"target_symbol_search_time" json:"target_symbol_search_time"`         // <50ms
	TargetCrossLanguageTime    time.Duration `yaml:"target_cross_language_time" json:"target_cross_language_time"`       // <100ms
	TargetContextAnalysisTime  time.Duration `yaml:"target_context_analysis_time" json:"target_context_analysis_time"`   // <200ms
	TargetSemanticAnalysisTime time.Duration `yaml:"target_semantic_analysis_time" json:"target_semantic_analysis_time"` // <500ms

	// Resource limits
	MaxConcurrentQueries int   `yaml:"max_concurrent_queries" json:"max_concurrent_queries"`
	MaxMemoryUsage       int64 `yaml:"max_memory_usage_bytes" json:"max_memory_usage_bytes"`
	MaxCacheSize         int   `yaml:"max_cache_size" json:"max_cache_size"`

	// Performance thresholds
	PerformanceDegradationThreshold float64       `yaml:"performance_degradation_threshold" json:"performance_degradation_threshold"`
	FailureRateThreshold            float64       `yaml:"failure_rate_threshold" json:"failure_rate_threshold"`
	LatencyP99Threshold             time.Duration `yaml:"latency_p99_threshold" json:"latency_p99_threshold"`

	// Cache performance
	CacheHitRateTarget    float64 `yaml:"cache_hit_rate_target" json:"cache_hit_rate_target"`
	CacheEvictionStrategy string  `yaml:"cache_eviction_strategy" json:"cache_eviction_strategy"`

	// Auto-tuning
	EnableAutoTuning bool          `yaml:"enable_auto_tuning" json:"enable_auto_tuning"`
	TuningInterval   time.Duration `yaml:"tuning_interval" json:"tuning_interval"`
	TuningAggression string        `yaml:"tuning_aggression,omitempty" json:"tuning_aggression,omitempty"` // "conservative", "moderate", "aggressive"
}

// SCIPMCPToolConfig provides per-tool configuration for SCIP-enhanced MCP tools
type SCIPMCPToolConfig struct {
	// Basic settings
	Enabled  bool          `yaml:"enabled" json:"enabled"`
	Timeout  time.Duration `yaml:"timeout" json:"timeout"`
	Priority int           `yaml:"priority,omitempty" json:"priority,omitempty"`

	// Caching settings
	CacheEnabled     bool          `yaml:"cache_enabled" json:"cache_enabled"`
	CacheTTL         time.Duration `yaml:"cache_ttl" json:"cache_ttl"`
	CacheKeyStrategy string        `yaml:"cache_key_strategy,omitempty" json:"cache_key_strategy,omitempty"`

	// Result limits
	MaxResults    int   `yaml:"max_results" json:"max_results"`
	MaxResultSize int64 `yaml:"max_result_size_bytes,omitempty" json:"max_result_size_bytes,omitempty"`

	// Quality settings
	ConfidenceThreshold    float64 `yaml:"confidence_threshold" json:"confidence_threshold"`
	EnableFuzzyMatching    bool    `yaml:"enable_fuzzy_matching,omitempty" json:"enable_fuzzy_matching,omitempty"`
	FuzzyMatchingThreshold float64 `yaml:"fuzzy_matching_threshold,omitempty" json:"fuzzy_matching_threshold,omitempty"`

	// Fallback settings
	FallbackEnabled  bool          `yaml:"fallback_enabled" json:"fallback_enabled"`
	FallbackStrategy string        `yaml:"fallback_strategy,omitempty" json:"fallback_strategy,omitempty"`
	FallbackTimeout  time.Duration `yaml:"fallback_timeout,omitempty" json:"fallback_timeout,omitempty"`

	// Enhancement features
	EnableSemanticEnhancement   bool `yaml:"enable_semantic_enhancement,omitempty" json:"enable_semantic_enhancement,omitempty"`
	EnableCrossLanguageFeatures bool `yaml:"enable_cross_language_features,omitempty" json:"enable_cross_language_features,omitempty"`
	EnableContextAwareness      bool `yaml:"enable_context_awareness,omitempty" json:"enable_context_awareness,omitempty"`

	// Custom settings for specific tools
	CustomSettings map[string]interface{} `yaml:"custom_settings,omitempty" json:"custom_settings,omitempty"`
}

// SCIPMCPMonitoringConfig defines monitoring and alerting behavior
type SCIPMCPMonitoringConfig struct {
	// Core monitoring features
	EnableMetrics   bool `yaml:"enable_metrics" json:"enable_metrics"`
	EnableTracing   bool `yaml:"enable_tracing" json:"enable_tracing"`
	EnableAlerting  bool `yaml:"enable_alerting" json:"enable_alerting"`
	EnableProfiling bool `yaml:"enable_profiling,omitempty" json:"enable_profiling,omitempty"`

	// Intervals and timing
	MetricsInterval      time.Duration `yaml:"metrics_interval" json:"metrics_interval"`
	HealthCheckInterval  time.Duration `yaml:"health_check_interval" json:"health_check_interval"`
	TracingFlushInterval time.Duration `yaml:"tracing_flush_interval,omitempty" json:"tracing_flush_interval,omitempty"`

	// Retention and storage
	MetricsRetention  time.Duration `yaml:"metrics_retention" json:"metrics_retention"`
	TracingRetention  time.Duration `yaml:"tracing_retention,omitempty" json:"tracing_retention,omitempty"`
	MaxMetricsHistory int           `yaml:"max_metrics_history,omitempty" json:"max_metrics_history,omitempty"`

	// Alert thresholds
	AlertThresholds *SCIPMCPAlertThresholds `yaml:"alert_thresholds,omitempty" json:"alert_thresholds,omitempty"`

	// Exports and integrations
	MetricsExports []MetricsExportConfig `yaml:"metrics_exports,omitempty" json:"metrics_exports,omitempty"`
	TracingExports []TracingExportConfig `yaml:"tracing_exports,omitempty" json:"tracing_exports,omitempty"`
}

// SCIPMCPAlertThresholds defines when to trigger alerts
type SCIPMCPAlertThresholds struct {
	// Performance alerts
	LatencyP50 time.Duration `yaml:"latency_p50,omitempty" json:"latency_p50,omitempty"`
	LatencyP95 time.Duration `yaml:"latency_p95,omitempty" json:"latency_p95,omitempty"`
	LatencyP99 time.Duration `yaml:"latency_p99" json:"latency_p99"`

	// Error rate alerts
	ErrorRate         float64 `yaml:"error_rate" json:"error_rate"`
	SCIPFailureRate   float64 `yaml:"scip_failure_rate,omitempty" json:"scip_failure_rate,omitempty"`
	FallbackUsageRate float64 `yaml:"fallback_usage_rate,omitempty" json:"fallback_usage_rate,omitempty"`

	// Resource alerts
	MemoryUsage float64 `yaml:"memory_usage" json:"memory_usage"`
	CPUUsage    float64 `yaml:"cpu_usage,omitempty" json:"cpu_usage,omitempty"`

	// Quality alerts
	CacheHitRate    float64 `yaml:"cache_hit_rate" json:"cache_hit_rate"`
	ConfidenceScore float64 `yaml:"confidence_score,omitempty" json:"confidence_score,omitempty"`

	// Availability alerts
	SCIPAvailability           float64 `yaml:"scip_availability" json:"scip_availability"`
	SymbolResolverAvailability float64 `yaml:"symbol_resolver_availability,omitempty" json:"symbol_resolver_availability,omitempty"`

	// Custom thresholds
	CustomThresholds map[string]float64 `yaml:"custom_thresholds,omitempty" json:"custom_thresholds,omitempty"`
}

// SCIPMCPFallbackConfig defines fallback behavior when SCIP is unavailable
type SCIPMCPFallbackConfig struct {
	// Core fallback settings
	EnableGracefulDegradation bool `yaml:"enable_graceful_degradation" json:"enable_graceful_degradation"`
	FallbackToStandardTools   bool `yaml:"fallback_to_standard_tools" json:"fallback_to_standard_tools"`
	EnablePartialFallback     bool `yaml:"enable_partial_fallback,omitempty" json:"enable_partial_fallback,omitempty"`

	// Failure detection
	MaxConsecutiveFailures int           `yaml:"max_consecutive_failures" json:"max_consecutive_failures"`
	FailureDetectionWindow time.Duration `yaml:"failure_detection_window,omitempty" json:"failure_detection_window,omitempty"`

	// Recovery behavior
	RecoveryCheckInterval   time.Duration `yaml:"recovery_check_interval" json:"recovery_check_interval"`
	RecoveryRetryCount      int           `yaml:"recovery_retry_count,omitempty" json:"recovery_retry_count,omitempty"`
	RecoveryBackoffStrategy string        `yaml:"recovery_backoff_strategy,omitempty" json:"recovery_backoff_strategy,omitempty"`

	// Timeout adjustments
	FallbackTimeoutMultiplier float64       `yaml:"fallback_timeout_multiplier" json:"fallback_timeout_multiplier"`
	MinFallbackTimeout        time.Duration `yaml:"min_fallback_timeout,omitempty" json:"min_fallback_timeout,omitempty"`
	MaxFallbackTimeout        time.Duration `yaml:"max_fallback_timeout,omitempty" json:"max_fallback_timeout,omitempty"`

	// Feature degradation
	DisableFeaturesOnFailure []string `yaml:"disable_features_on_failure,omitempty" json:"disable_features_on_failure,omitempty"`
	ReduceQualityOnFailure   bool     `yaml:"reduce_quality_on_failure,omitempty" json:"reduce_quality_on_failure,omitempty"`

	// Circuit breaker settings
	CircuitBreakerSettings *CircuitBreakerConfig `yaml:"circuit_breaker,omitempty" json:"circuit_breaker,omitempty"`
}

// SCIPMCPAutoConfigSettings enables automatic configuration optimization
type SCIPMCPAutoConfigSettings struct {
	// Core auto-config features
	EnableAutoConfiguration bool `yaml:"enable_auto_configuration" json:"enable_auto_configuration"`
	EnableLearning          bool `yaml:"enable_learning,omitempty" json:"enable_learning,omitempty"`
	EnableAdaptation        bool `yaml:"enable_adaptation,omitempty" json:"enable_adaptation,omitempty"`

	// Optimization goals
	OptimizationGoal    string             `yaml:"optimization_goal" json:"optimization_goal"` // "performance", "accuracy", "balanced", "resource_efficiency"
	OptimizationWeights map[string]float64 `yaml:"optimization_weights,omitempty" json:"optimization_weights,omitempty"`

	// Learning and adaptation
	LearningPeriod             time.Duration `yaml:"learning_period" json:"learning_period"`
	AdaptationInterval         time.Duration `yaml:"adaptation_interval" json:"adaptation_interval"`
	MinDataPointsForAdaptation int           `yaml:"min_data_points,omitempty" json:"min_data_points,omitempty"`

	// Feedback and improvement
	EnableFeedbackLoop            bool    `yaml:"enable_feedback_loop" json:"enable_feedback_loop"`
	FeedbackWindowSize            int     `yaml:"feedback_window_size,omitempty" json:"feedback_window_size,omitempty"`
	ConfidenceThresholdForChanges float64 `yaml:"confidence_threshold_for_changes,omitempty" json:"confidence_threshold_for_changes,omitempty"`

	// Safety and constraints
	MaxConfigChangesPerInterval int      `yaml:"max_config_changes_per_interval,omitempty" json:"max_config_changes_per_interval,omitempty"`
	SafetyMargin                float64  `yaml:"safety_margin,omitempty" json:"safety_margin,omitempty"`
	AllowedConfigFields         []string `yaml:"allowed_config_fields,omitempty" json:"allowed_config_fields,omitempty"`
}

// SCIPMCPIntegrationConfig defines integration with LSP Gateway components
type SCIPMCPIntegrationConfig struct {
	// LSP Gateway integration
	EnableLSPGatewayIntegration bool   `yaml:"enable_lsp_gateway_integration" json:"enable_lsp_gateway_integration"`
	LSPGatewayEndpoint          string `yaml:"lsp_gateway_endpoint,omitempty" json:"lsp_gateway_endpoint,omitempty"`

	// MCP Server integration
	MCPServerConfig *MCPServerIntegrationConfig `yaml:"mcp_server_config,omitempty" json:"mcp_server_config,omitempty"`

	// SCIP Store integration
	SCIPStoreConfig *SCIPStoreIntegrationConfig `yaml:"scip_store_config,omitempty" json:"scip_store_config,omitempty"`

	// Symbol Resolver integration
	SymbolResolverConfig *SymbolResolverIntegrationConfig `yaml:"symbol_resolver_config,omitempty" json:"symbol_resolver_config,omitempty"`

	// Workspace Context integration
	WorkspaceContextConfig *WorkspaceContextIntegrationConfig `yaml:"workspace_context_config,omitempty" json:"workspace_context_config,omitempty"`

	// External integrations
	ExternalIntegrations []ExternalIntegrationConfig `yaml:"external_integrations,omitempty" json:"external_integrations,omitempty"`
}

// Supporting configuration types

// MetricsExportConfig defines metrics export configuration
type MetricsExportConfig struct {
	Type     string            `yaml:"type" json:"type"` // "prometheus", "statsd", "influx", "custom"
	Endpoint string            `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Interval time.Duration     `yaml:"interval,omitempty" json:"interval,omitempty"`
	Format   string            `yaml:"format,omitempty" json:"format,omitempty"`
	Headers  map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Enabled  bool              `yaml:"enabled" json:"enabled"`
}

// TracingExportConfig defines tracing export configuration
type TracingExportConfig struct {
	Type       string            `yaml:"type" json:"type"` // "jaeger", "zipkin", "otlp", "custom"
	Endpoint   string            `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	SampleRate float64           `yaml:"sample_rate,omitempty" json:"sample_rate,omitempty"`
	Headers    map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Enabled    bool              `yaml:"enabled" json:"enabled"`
}

// CircuitBreakerConfig defines circuit breaker configuration
type CircuitBreakerConfig struct {
	Enabled             bool          `yaml:"enabled" json:"enabled"`
	FailureThreshold    int           `yaml:"failure_threshold,omitempty" json:"failure_threshold,omitempty"`
	RecoveryTimeout     time.Duration `yaml:"recovery_timeout,omitempty" json:"recovery_timeout,omitempty"`
	HalfOpenMaxRequests int           `yaml:"half_open_max_requests,omitempty" json:"half_open_max_requests,omitempty"`
	SuccessThreshold    int           `yaml:"success_threshold,omitempty" json:"success_threshold,omitempty"`
}

// MCPServerIntegrationConfig defines MCP server integration settings
type MCPServerIntegrationConfig struct {
	Enabled              bool          `yaml:"enabled" json:"enabled"`
	ServerPort           int           `yaml:"server_port,omitempty" json:"server_port,omitempty"`
	TransportType        string        `yaml:"transport_type,omitempty" json:"transport_type,omitempty"` // "stdio", "tcp"
	MaxConnections       int           `yaml:"max_connections,omitempty" json:"max_connections,omitempty"`
	ConnectionTimeout    time.Duration `yaml:"connection_timeout,omitempty" json:"connection_timeout,omitempty"`
	EnableToolValidation bool          `yaml:"enable_tool_validation,omitempty" json:"enable_tool_validation,omitempty"`
}

// SCIPStoreIntegrationConfig defines SCIP store integration settings
type SCIPStoreIntegrationConfig struct {
	Enabled                 bool          `yaml:"enabled" json:"enabled"`
	StoreType               string        `yaml:"store_type,omitempty" json:"store_type,omitempty"` // "memory", "disk", "hybrid"
	ConnectionPoolSize      int           `yaml:"connection_pool_size,omitempty" json:"connection_pool_size,omitempty"`
	QueryTimeout            time.Duration `yaml:"query_timeout,omitempty" json:"query_timeout,omitempty"`
	EnableQueryOptimization bool          `yaml:"enable_query_optimization,omitempty" json:"enable_query_optimization,omitempty"`
	CacheStrategy           string        `yaml:"cache_strategy,omitempty" json:"cache_strategy,omitempty"`
}

// SymbolResolverIntegrationConfig defines symbol resolver integration settings
type SymbolResolverIntegrationConfig struct {
	Enabled                    bool          `yaml:"enabled" json:"enabled"`
	ResolverType               string        `yaml:"resolver_type,omitempty" json:"resolver_type,omitempty"` // "standard", "enhanced", "hybrid"
	MaxResolutionTime          time.Duration `yaml:"max_resolution_time,omitempty" json:"max_resolution_time,omitempty"`
	EnablePositionalResolution bool          `yaml:"enable_positional_resolution,omitempty" json:"enable_positional_resolution,omitempty"`
	EnableSemanticResolution   bool          `yaml:"enable_semantic_resolution,omitempty" json:"enable_semantic_resolution,omitempty"`
	CacheResolutions           bool          `yaml:"cache_resolutions,omitempty" json:"cache_resolutions,omitempty"`
}

// WorkspaceContextIntegrationConfig defines workspace context integration settings
type WorkspaceContextIntegrationConfig struct {
	Enabled                bool          `yaml:"enabled" json:"enabled"`
	AutoDetectProjects     bool          `yaml:"auto_detect_projects,omitempty" json:"auto_detect_projects,omitempty"`
	EnableProjectAwareness bool          `yaml:"enable_project_awareness,omitempty" json:"enable_project_awareness,omitempty"`
	RefreshInterval        time.Duration `yaml:"refresh_interval,omitempty" json:"refresh_interval,omitempty"`
	MaxProjectContextSize  int64         `yaml:"max_project_context_size,omitempty" json:"max_project_context_size,omitempty"`
}

// ExternalIntegrationConfig defines external system integrations
type ExternalIntegrationConfig struct {
	Name           string                 `yaml:"name" json:"name"`
	Type           string                 `yaml:"type" json:"type"`
	Enabled        bool                   `yaml:"enabled" json:"enabled"`
	Endpoint       string                 `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Authentication map[string]string      `yaml:"authentication,omitempty" json:"authentication,omitempty"`
	Settings       map[string]interface{} `yaml:"settings,omitempty" json:"settings,omitempty"`
}

// DefaultSCIPMCPConfiguration returns a production-ready default configuration
func DefaultSCIPMCPConfiguration() *SCIPMCPConfiguration {
	return &SCIPMCPConfiguration{
		Enabled:                     true,
		EnableSCIPEnhancements:      true,
		EnableFallbackMode:          true,
		EnablePerformanceMonitoring: true,
		EnableHealthChecking:        true,
		Version:                     "1.0.0",
		CreatedAt:                   time.Now(),

		PerformanceConfig: &SCIPMCPPerformanceConfig{
			// Performance targets from requirements
			TargetSymbolSearchTime:     50 * time.Millisecond,
			TargetCrossLanguageTime:    100 * time.Millisecond,
			TargetContextAnalysisTime:  200 * time.Millisecond,
			TargetSemanticAnalysisTime: 500 * time.Millisecond,

			// Resource limits
			MaxConcurrentQueries: 100,
			MaxMemoryUsage:       1 << 30, // 1GB
			MaxCacheSize:         10000,

			// Performance thresholds
			PerformanceDegradationThreshold: 0.2,  // 20% degradation
			FailureRateThreshold:            0.05, // 5% failure rate
			LatencyP99Threshold:             1 * time.Second,

			// Cache settings
			CacheHitRateTarget:    0.85,
			CacheEvictionStrategy: "lru",

			// Auto-tuning
			EnableAutoTuning: true,
			TuningInterval:   5 * time.Minute,
			TuningAggression: "moderate",
		},

		ToolConfigs: map[string]*SCIPMCPToolConfig{
			"scip_intelligent_symbol_search": {
				Enabled:                     true,
				Timeout:                     50 * time.Millisecond,
				Priority:                    100,
				CacheEnabled:                true,
				CacheTTL:                    15 * time.Minute,
				MaxResults:                  50,
				ConfidenceThreshold:         0.7,
				FallbackEnabled:             true,
				FallbackStrategy:            "standard_tool",
				EnableSemanticEnhancement:   true,
				EnableCrossLanguageFeatures: true,
			},
			"scip_cross_language_references": {
				Enabled:                     true,
				Timeout:                     100 * time.Millisecond,
				Priority:                    90,
				CacheEnabled:                true,
				CacheTTL:                    10 * time.Minute,
				MaxResults:                  100,
				ConfidenceThreshold:         0.8,
				FallbackEnabled:             true,
				FallbackStrategy:            "standard_tool",
				EnableCrossLanguageFeatures: true,
			},
			"scip_semantic_code_analysis": {
				Enabled:                   true,
				Timeout:                   500 * time.Millisecond,
				Priority:                  80,
				CacheEnabled:              true,
				CacheTTL:                  30 * time.Minute,
				MaxResults:                1,
				ConfidenceThreshold:       0.75,
				FallbackEnabled:           true,
				FallbackStrategy:          "standard_tool",
				EnableSemanticEnhancement: true,
			},
			"scip_context_aware_assistance": {
				Enabled:                true,
				Timeout:                200 * time.Millisecond,
				Priority:               85,
				CacheEnabled:           true,
				CacheTTL:               5 * time.Minute,
				MaxResults:             20,
				ConfidenceThreshold:    0.7,
				FallbackEnabled:        true,
				EnableContextAwareness: true,
			},
			"scip_workspace_intelligence": {
				Enabled:             true,
				Timeout:             1 * time.Second,
				Priority:            70,
				CacheEnabled:        true,
				CacheTTL:            1 * time.Hour,
				MaxResults:          1,
				ConfidenceThreshold: 0.8,
				FallbackEnabled:     false, // Workspace intelligence doesn't have a standard fallback
			},
			"scip_refactoring_suggestions": {
				Enabled:                   true,
				Timeout:                   500 * time.Millisecond,
				Priority:                  75,
				CacheEnabled:              true,
				CacheTTL:                  10 * time.Minute,
				MaxResults:                10,
				ConfidenceThreshold:       0.8,
				FallbackEnabled:           false, // Refactoring suggestions are SCIP-specific
				EnableSemanticEnhancement: true,
			},
		},

		MonitoringConfig: &SCIPMCPMonitoringConfig{
			EnableMetrics:   true,
			EnableTracing:   false, // Disabled by default for performance
			EnableAlerting:  true,
			EnableProfiling: false,

			MetricsInterval:     30 * time.Second,
			HealthCheckInterval: 1 * time.Minute,
			MetricsRetention:    24 * time.Hour,
			MaxMetricsHistory:   1000,

			AlertThresholds: &SCIPMCPAlertThresholds{
				LatencyP99:       1 * time.Second,
				ErrorRate:        0.05,
				MemoryUsage:      0.8,
				CacheHitRate:     0.7,
				SCIPAvailability: 0.95,
			},
		},

		FallbackConfig: &SCIPMCPFallbackConfig{
			EnableGracefulDegradation: true,
			FallbackToStandardTools:   true,
			EnablePartialFallback:     true,

			MaxConsecutiveFailures:  5,
			RecoveryCheckInterval:   30 * time.Second,
			RecoveryRetryCount:      3,
			RecoveryBackoffStrategy: "exponential",

			FallbackTimeoutMultiplier: 2.0,
			MinFallbackTimeout:        100 * time.Millisecond,
			MaxFallbackTimeout:        5 * time.Second,

			DisableFeaturesOnFailure: []string{
				"semantic_analysis",
				"refactoring_suggestions",
			},
			ReduceQualityOnFailure: true,

			CircuitBreakerSettings: &CircuitBreakerConfig{
				Enabled:             true,
				FailureThreshold:    10,
				RecoveryTimeout:     30 * time.Second,
				HalfOpenMaxRequests: 5,
				SuccessThreshold:    3,
			},
		},

		AutoConfig: &SCIPMCPAutoConfigSettings{
			EnableAutoConfiguration: true,
			EnableLearning:          true,
			EnableAdaptation:        true,

			OptimizationGoal:           "balanced",
			LearningPeriod:             1 * time.Hour,
			AdaptationInterval:         15 * time.Minute,
			MinDataPointsForAdaptation: 100,

			EnableFeedbackLoop:            true,
			FeedbackWindowSize:            1000,
			ConfidenceThresholdForChanges: 0.8,

			MaxConfigChangesPerInterval: 3,
			SafetyMargin:                0.1,
			AllowedConfigFields: []string{
				"timeout",
				"cache_ttl",
				"max_results",
				"confidence_threshold",
			},
		},

		IntegrationConfig: &SCIPMCPIntegrationConfig{
			EnableLSPGatewayIntegration: true,

			MCPServerConfig: &MCPServerIntegrationConfig{
				Enabled:              true,
				TransportType:        "stdio",
				MaxConnections:       100,
				ConnectionTimeout:    30 * time.Second,
				EnableToolValidation: true,
			},

			SCIPStoreConfig: &SCIPStoreIntegrationConfig{
				Enabled:                 true,
				StoreType:               "hybrid",
				ConnectionPoolSize:      10,
				QueryTimeout:            1 * time.Second,
				EnableQueryOptimization: true,
				CacheStrategy:           "intelligent",
			},

			SymbolResolverConfig: &SymbolResolverIntegrationConfig{
				Enabled:                    true,
				ResolverType:               "enhanced",
				MaxResolutionTime:          50 * time.Millisecond,
				EnablePositionalResolution: true,
				EnableSemanticResolution:   true,
				CacheResolutions:           true,
			},

			WorkspaceContextConfig: &WorkspaceContextIntegrationConfig{
				Enabled:                true,
				AutoDetectProjects:     true,
				EnableProjectAwareness: true,
				RefreshInterval:        5 * time.Minute,
				MaxProjectContextSize:  10 * 1024 * 1024, // 10MB
			},
		},
	}
}

// DevSCIPMCPConfiguration returns a development-optimized configuration
func DevSCIPMCPConfiguration() *SCIPMCPConfiguration {
	config := DefaultSCIPMCPConfiguration()

	// Adjust for development
	config.EnablePerformanceMonitoring = true
	config.MonitoringConfig.EnableTracing = true
	config.MonitoringConfig.EnableProfiling = true
	config.MonitoringConfig.MetricsInterval = 10 * time.Second

	// More lenient timeouts for debugging
	config.PerformanceConfig.TargetSymbolSearchTime = 100 * time.Millisecond
	config.PerformanceConfig.TargetCrossLanguageTime = 200 * time.Millisecond
	config.PerformanceConfig.TargetContextAnalysisTime = 500 * time.Millisecond
	config.PerformanceConfig.TargetSemanticAnalysisTime = 1 * time.Second

	// Adjust tool timeouts
	for _, toolConfig := range config.ToolConfigs {
		toolConfig.Timeout *= 2
		toolConfig.CacheTTL = 5 * time.Minute // Shorter cache for faster iteration
	}

	// Disable auto-tuning in development
	config.AutoConfig.EnableAutoConfiguration = false

	return config
}

// ProductionSCIPMCPConfiguration returns a production-optimized configuration
func ProductionSCIPMCPConfiguration() *SCIPMCPConfiguration {
	config := DefaultSCIPMCPConfiguration()

	// Production optimizations
	config.MonitoringConfig.EnableTracing = false
	config.MonitoringConfig.EnableProfiling = false

	// Stricter timeouts
	config.PerformanceConfig.TargetSymbolSearchTime = 40 * time.Millisecond
	config.PerformanceConfig.TargetCrossLanguageTime = 80 * time.Millisecond
	config.PerformanceConfig.TargetContextAnalysisTime = 150 * time.Millisecond
	config.PerformanceConfig.TargetSemanticAnalysisTime = 400 * time.Millisecond

	// Higher confidence thresholds
	for _, toolConfig := range config.ToolConfigs {
		toolConfig.ConfidenceThreshold += 0.1
	}

	// More aggressive auto-tuning
	// config.AutoConfig.TuningAggression = "aggressive" // Field doesn't exist
	config.AutoConfig.AdaptationInterval = 5 * time.Minute

	// Enhanced monitoring
	config.MonitoringConfig.MetricsRetention = 7 * 24 * time.Hour // 7 days
	config.MonitoringConfig.MaxMetricsHistory = 10000

	return config
}

// ValidateSCIPMCPConfiguration validates the configuration and returns any errors
func ValidateSCIPMCPConfiguration(config *SCIPMCPConfiguration) []error {
	var errors []error

	if config == nil {
		errors = append(errors, fmt.Errorf("configuration cannot be nil"))
		return errors
	}

	// Validate performance config
	if config.PerformanceConfig != nil {
		if config.PerformanceConfig.TargetSymbolSearchTime <= 0 {
			errors = append(errors, fmt.Errorf("target_symbol_search_time must be positive"))
		}
		if config.PerformanceConfig.TargetSymbolSearchTime > 1*time.Second {
			errors = append(errors, fmt.Errorf("target_symbol_search_time too high: %v", config.PerformanceConfig.TargetSymbolSearchTime))
		}
		if config.PerformanceConfig.MaxConcurrentQueries <= 0 {
			errors = append(errors, fmt.Errorf("max_concurrent_queries must be positive"))
		}
		if config.PerformanceConfig.FailureRateThreshold < 0 || config.PerformanceConfig.FailureRateThreshold > 1 {
			errors = append(errors, fmt.Errorf("failure_rate_threshold must be between 0 and 1"))
		}
	}

	// Validate tool configs
	for toolName, toolConfig := range config.ToolConfigs {
		if toolConfig.Timeout <= 0 {
			errors = append(errors, fmt.Errorf("tool %s: timeout must be positive", toolName))
		}
		if toolConfig.ConfidenceThreshold < 0 || toolConfig.ConfidenceThreshold > 1 {
			errors = append(errors, fmt.Errorf("tool %s: confidence_threshold must be between 0 and 1", toolName))
		}
		if toolConfig.MaxResults <= 0 {
			errors = append(errors, fmt.Errorf("tool %s: max_results must be positive", toolName))
		}
	}

	// Validate monitoring config
	if config.MonitoringConfig != nil {
		if config.MonitoringConfig.MetricsInterval <= 0 {
			errors = append(errors, fmt.Errorf("metrics_interval must be positive"))
		}
		if config.MonitoringConfig.HealthCheckInterval <= 0 {
			errors = append(errors, fmt.Errorf("health_check_interval must be positive"))
		}
	}

	// Validate fallback config
	if config.FallbackConfig != nil {
		if config.FallbackConfig.MaxConsecutiveFailures <= 0 {
			errors = append(errors, fmt.Errorf("max_consecutive_failures must be positive"))
		}
		if config.FallbackConfig.FallbackTimeoutMultiplier <= 0 {
			errors = append(errors, fmt.Errorf("fallback_timeout_multiplier must be positive"))
		}
	}

	return errors
}

// MergeSCIPMCPConfiguration merges two configurations with the second taking precedence
func MergeSCIPMCPConfiguration(base, override *SCIPMCPConfiguration) *SCIPMCPConfiguration {
	if base == nil {
		return override
	}
	if override == nil {
		return base
	}

	// Create a deep copy of base
	merged := *base
	merged.UpdatedAt = time.Now()

	// Override top-level fields
	if override.Enabled != base.Enabled {
		merged.Enabled = override.Enabled
	}
	if override.EnableSCIPEnhancements != base.EnableSCIPEnhancements {
		merged.EnableSCIPEnhancements = override.EnableSCIPEnhancements
	}
	if override.EnableFallbackMode != base.EnableFallbackMode {
		merged.EnableFallbackMode = override.EnableFallbackMode
	}

	// Merge performance config if provided
	if override.PerformanceConfig != nil {
		if merged.PerformanceConfig == nil {
			merged.PerformanceConfig = &SCIPMCPPerformanceConfig{}
		}
		if override.PerformanceConfig.TargetSymbolSearchTime != 0 {
			merged.PerformanceConfig.TargetSymbolSearchTime = override.PerformanceConfig.TargetSymbolSearchTime
		}
		if override.PerformanceConfig.MaxConcurrentQueries != 0 {
			merged.PerformanceConfig.MaxConcurrentQueries = override.PerformanceConfig.MaxConcurrentQueries
		}
		// Continue for other fields...
	}

	// Merge tool configs
	if override.ToolConfigs != nil {
		if merged.ToolConfigs == nil {
			merged.ToolConfigs = make(map[string]*SCIPMCPToolConfig)
		}
		for toolName, toolConfig := range override.ToolConfigs {
			merged.ToolConfigs[toolName] = toolConfig
		}
	}

	return &merged
}

// GetToolConfig returns the configuration for a specific tool
func (config *SCIPMCPConfiguration) GetToolConfig(toolName string) *SCIPMCPToolConfig {
	if config.ToolConfigs == nil {
		return nil
	}
	return config.ToolConfigs[toolName]
}

// IsToolEnabled checks if a specific tool is enabled
func (config *SCIPMCPConfiguration) IsToolEnabled(toolName string) bool {
	if !config.Enabled || !config.EnableSCIPEnhancements {
		return false
	}

	toolConfig := config.GetToolConfig(toolName)
	if toolConfig == nil {
		return false
	}

	return toolConfig.Enabled
}

// GetEffectiveTimeout returns the effective timeout for a tool considering fallback
func (config *SCIPMCPConfiguration) GetEffectiveTimeout(toolName string) time.Duration {
	toolConfig := config.GetToolConfig(toolName)
	if toolConfig == nil {
		return 1 * time.Second // Default timeout
	}

	timeout := toolConfig.Timeout

	// Apply fallback timeout multiplier if needed
	if config.FallbackConfig != nil && config.FallbackConfig.FallbackTimeoutMultiplier > 1 {
		timeout = time.Duration(float64(timeout) * config.FallbackConfig.FallbackTimeoutMultiplier)

		// Respect min/max limits
		if config.FallbackConfig.MinFallbackTimeout > 0 && timeout < config.FallbackConfig.MinFallbackTimeout {
			timeout = config.FallbackConfig.MinFallbackTimeout
		}
		if config.FallbackConfig.MaxFallbackTimeout > 0 && timeout > config.FallbackConfig.MaxFallbackTimeout {
			timeout = config.FallbackConfig.MaxFallbackTimeout
		}
	}

	return timeout
}
