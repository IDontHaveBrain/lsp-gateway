package pooling

import (
	"time"
)

// PoolConfig provides global configuration for the server pool manager
type PoolConfig struct {
	// Core settings
	Enabled           bool          `yaml:"enabled" json:"enabled"`
	GlobalTimeout     time.Duration `yaml:"global_timeout" json:"global_timeout"`
	ShutdownTimeout   time.Duration `yaml:"shutdown_timeout" json:"shutdown_timeout"`
	AllocationTimeout time.Duration `yaml:"allocation_timeout" json:"allocation_timeout"`
	
	// Health monitoring
	HealthCheckInterval time.Duration `yaml:"health_check_interval" json:"health_check_interval"`
	HealthCheckTimeout  time.Duration `yaml:"health_check_timeout" json:"health_check_timeout"`
	MaxUnhealthyRatio   float64       `yaml:"max_unhealthy_ratio" json:"max_unhealthy_ratio"`
	
	// Resource limits
	GlobalMaxServers  int   `yaml:"global_max_servers" json:"global_max_servers"`
	GlobalMemoryLimitMB int64 `yaml:"global_memory_limit_mb" json:"global_memory_limit_mb"`
	
	// Allocation strategy
	DefaultAllocationStrategy AllocationStrategy `yaml:"default_allocation_strategy" json:"default_allocation_strategy"`
	
	// Fallback behavior
	EnableFallback       bool          `yaml:"enable_fallback" json:"enable_fallback"`
	FallbackTimeout      time.Duration `yaml:"fallback_timeout" json:"fallback_timeout"`
	MaxFallbackRetries   int           `yaml:"max_fallback_retries" json:"max_fallback_retries"`
	
	// Language-specific configurations
	Languages map[string]*LanguagePoolConfig `yaml:"languages" json:"languages"`
	
	// Metrics and monitoring
	EnableMetrics       bool          `yaml:"enable_metrics" json:"enable_metrics"`
	MetricsInterval     time.Duration `yaml:"metrics_interval" json:"metrics_interval"`
	EnableDetailedLogs  bool          `yaml:"enable_detailed_logs" json:"enable_detailed_logs"`
}

// LanguagePoolConfig provides language-specific pool configuration
type LanguagePoolConfig struct {
	// Pool sizing
	MinServers    int `yaml:"min_servers" json:"min_servers"`
	MaxServers    int `yaml:"max_servers" json:"max_servers"`
	WarmupServers int `yaml:"warmup_servers" json:"warmup_servers"`
	
	// Timeouts
	WarmupTimeout    time.Duration `yaml:"warmup_timeout" json:"warmup_timeout"`
	IdleTimeout      time.Duration `yaml:"idle_timeout" json:"idle_timeout"`
	MaxServerAge     time.Duration `yaml:"max_server_age" json:"max_server_age"`
	
	// Server lifecycle
	MaxUseCount          int           `yaml:"max_use_count" json:"max_use_count"`
	RestartThreshold     int           `yaml:"restart_threshold" json:"restart_threshold"`
	FailureThreshold     int           `yaml:"failure_threshold" json:"failure_threshold"`
	RecoveryDelay        time.Duration `yaml:"recovery_delay" json:"recovery_delay"`
	
	// Language-specific transport settings
	TransportType  string            `yaml:"transport_type" json:"transport_type"`
	Command        string            `yaml:"command" json:"command"`
	Args           []string          `yaml:"args" json:"args"`
	WorkingDir     string            `yaml:"working_dir" json:"working_dir"`
	Environment    map[string]string `yaml:"environment" json:"environment"`
	
	// Performance tuning
	ConcurrentRequests   int                `yaml:"concurrent_requests" json:"concurrent_requests"`
	AllocationStrategy   AllocationStrategy `yaml:"allocation_strategy" json:"allocation_strategy"`
	PreferRecentServers  bool               `yaml:"prefer_recent_servers" json:"prefer_recent_servers"`
	
	// Workspace management
	EnableWorkspaceSwitching bool          `yaml:"enable_workspace_switching" json:"enable_workspace_switching"`
	WorkspaceSwitchTimeout   time.Duration `yaml:"workspace_switch_timeout" json:"workspace_switch_timeout"`
	MaxWorkspacesPerServer   int           `yaml:"max_workspaces_per_server" json:"max_workspaces_per_server"`
	
	// Resource limits
	MaxMemoryMB      int64   `yaml:"max_memory_mb" json:"max_memory_mb"`
	MaxCPUPercent    float64 `yaml:"max_cpu_percent" json:"max_cpu_percent"`
	
	// Custom configuration for specific language servers
	CustomConfig map[string]interface{} `yaml:"custom_config,omitempty" json:"custom_config,omitempty"`
}

// DefaultPoolConfig returns a sensible default configuration for the pool manager
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		Enabled:                   true,
		GlobalTimeout:             300 * time.Second,
		ShutdownTimeout:           30 * time.Second,
		AllocationTimeout:         10 * time.Second,
		HealthCheckInterval:       30 * time.Second,
		HealthCheckTimeout:        5 * time.Second,
		MaxUnhealthyRatio:         0.5,
		GlobalMaxServers:          20,
		GlobalMemoryLimitMB:       2048,
		DefaultAllocationStrategy: AllocationStrategyLeastUsed,
		EnableFallback:            true,
		FallbackTimeout:           60 * time.Second,
		MaxFallbackRetries:        3,
		EnableMetrics:             true,
		MetricsInterval:           60 * time.Second,
		EnableDetailedLogs:        false,
		Languages:                 getDefaultLanguageConfigs(),
	}
}

// getDefaultLanguageConfigs returns default configurations for supported languages
func getDefaultLanguageConfigs() map[string]*LanguagePoolConfig {
	return map[string]*LanguagePoolConfig{
		"java": {
			MinServers:               2,
			MaxServers:               3,
			WarmupServers:            2,
			WarmupTimeout:            90 * time.Second,
			IdleTimeout:              10 * time.Minute,
			MaxServerAge:             30 * time.Minute,
			MaxUseCount:              100,
			RestartThreshold:         5,
			FailureThreshold:         3,
			RecoveryDelay:            30 * time.Second,
			TransportType:            "stdio",
			ConcurrentRequests:       5,
			AllocationStrategy:       AllocationStrategyLeastUsed,
			PreferRecentServers:      false,
			EnableWorkspaceSwitching: true,
			WorkspaceSwitchTimeout:   30 * time.Second,
			MaxWorkspacesPerServer:   5,
			MaxMemoryMB:              512,
			MaxCPUPercent:            80.0,
		},
		"typescript": {
			MinServers:               2,
			MaxServers:               2,
			WarmupServers:            1,
			WarmupTimeout:            45 * time.Second,
			IdleTimeout:              8 * time.Minute,
			MaxServerAge:             20 * time.Minute,
			MaxUseCount:              50,
			RestartThreshold:         3,
			FailureThreshold:         2,
			RecoveryDelay:            15 * time.Second,
			TransportType:            "stdio",
			ConcurrentRequests:       3,
			AllocationStrategy:       AllocationStrategyRoundRobin,
			PreferRecentServers:      true,
			EnableWorkspaceSwitching: true,
			WorkspaceSwitchTimeout:   20 * time.Second,
			MaxWorkspacesPerServer:   3,
			MaxMemoryMB:              256,
			MaxCPUPercent:            70.0,
		},
		"go": {
			MinServers:               1,
			MaxServers:               2,
			WarmupServers:            1,
			WarmupTimeout:            20 * time.Second,
			IdleTimeout:              5 * time.Minute,
			MaxServerAge:             15 * time.Minute,
			MaxUseCount:              30,
			RestartThreshold:         2,
			FailureThreshold:         2,
			RecoveryDelay:            10 * time.Second,
			TransportType:            "stdio",
			ConcurrentRequests:       2,
			AllocationStrategy:       AllocationStrategyMostRecent,
			PreferRecentServers:      true,
			EnableWorkspaceSwitching: true,
			WorkspaceSwitchTimeout:   15 * time.Second,
			MaxWorkspacesPerServer:   4,
			MaxMemoryMB:              128,
			MaxCPUPercent:            60.0,
		},
		"python": {
			MinServers:               1,
			MaxServers:               2,
			WarmupServers:            1,
			WarmupTimeout:            15 * time.Second,
			IdleTimeout:              4 * time.Minute,
			MaxServerAge:             12 * time.Minute,
			MaxUseCount:              25,
			RestartThreshold:         2,
			FailureThreshold:         2,
			RecoveryDelay:            8 * time.Second,
			TransportType:            "stdio",
			ConcurrentRequests:       2,
			AllocationStrategy:       AllocationStrategyLeastUsed,
			PreferRecentServers:      false,
			EnableWorkspaceSwitching: true,
			WorkspaceSwitchTimeout:   10 * time.Second,
			MaxWorkspacesPerServer:   3,
			MaxMemoryMB:              96,
			MaxCPUPercent:            50.0,
		},
	}
}

// ValidateConfig validates the pool configuration for correctness
func (pc *PoolConfig) ValidateConfig() error {
	if pc.GlobalTimeout <= 0 {
		pc.GlobalTimeout = 300 * time.Second
	}
	if pc.AllocationTimeout <= 0 {
		pc.AllocationTimeout = 10 * time.Second
	}
	if pc.HealthCheckInterval <= 0 {
		pc.HealthCheckInterval = 30 * time.Second
	}
	if pc.GlobalMaxServers <= 0 {
		pc.GlobalMaxServers = 20
	}
	if pc.MaxUnhealthyRatio < 0 || pc.MaxUnhealthyRatio > 1 {
		pc.MaxUnhealthyRatio = 0.5
	}
	
	// Validate language configurations
	for lang, langConfig := range pc.Languages {
		if err := langConfig.ValidateConfig(lang); err != nil {
			return err
		}
	}
	
	return nil
}

// ValidateConfig validates the language-specific pool configuration
func (lpc *LanguagePoolConfig) ValidateConfig(language string) error {
	if lpc.MinServers < 0 {
		lpc.MinServers = 1
	}
	if lpc.MaxServers <= 0 {
		lpc.MaxServers = 2
	}
	if lpc.MinServers > lpc.MaxServers {
		lpc.MinServers = lpc.MaxServers
	}
	if lpc.WarmupServers > lpc.MaxServers {
		lpc.WarmupServers = lpc.MaxServers
	}
	if lpc.WarmupTimeout <= 0 {
		lpc.WarmupTimeout = 30 * time.Second
	}
	if lpc.IdleTimeout <= 0 {
		lpc.IdleTimeout = 5 * time.Minute
	}
	if lpc.MaxUseCount <= 0 {
		lpc.MaxUseCount = 50
	}
	if lpc.ConcurrentRequests <= 0 {
		lpc.ConcurrentRequests = 1
	}
	
	return nil
}