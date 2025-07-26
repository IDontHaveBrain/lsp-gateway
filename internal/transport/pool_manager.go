package transport

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// PoolManager defines the interface for managing connection pools with advanced features
type PoolManager interface {
	GetConnection(ctx context.Context) (interface{}, error)
	ReturnConnection(conn interface{}) error
	GetStats() *PoolStats
	UpdateConfig(config *PoolConfig) error
	Resize(newSize int) error
	Shutdown(ctx context.Context) error
	HealthCheck() error
}

// PoolConfig contains comprehensive configuration for connection pooling
type PoolConfig struct {
	// Core pool settings
	MinSize    int `yaml:"min_size" json:"min_size"`
	MaxSize    int `yaml:"max_size" json:"max_size"`
	WarmupSize int `yaml:"warmup_size" json:"warmup_size"`

	// Dynamic sizing
	EnableDynamicSizing bool    `yaml:"enable_dynamic_sizing" json:"enable_dynamic_sizing"`
	TargetUtilization   float64 `yaml:"target_utilization" json:"target_utilization"`
	ScaleUpThreshold    float64 `yaml:"scale_up_threshold" json:"scale_up_threshold"`
	ScaleDownThreshold  float64 `yaml:"scale_down_threshold" json:"scale_down_threshold"`

	// Connection management
	MaxLifetime         time.Duration `yaml:"max_lifetime" json:"max_lifetime"`
	IdleTimeout         time.Duration `yaml:"idle_timeout" json:"idle_timeout"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval" json:"health_check_interval"`

	// Circuit breaker
	MaxRetries     int           `yaml:"max_retries" json:"max_retries"`
	BaseDelay      time.Duration `yaml:"base_delay" json:"base_delay"`
	CircuitTimeout time.Duration `yaml:"circuit_timeout" json:"circuit_timeout"`

	// Resource limits
	MemoryLimitMB   int64   `yaml:"memory_limit_mb" json:"memory_limit_mb"`
	CPULimitPercent float64 `yaml:"cpu_limit_percent" json:"cpu_limit_percent"`

	// Transport-specific settings
	TransportType string                 `yaml:"transport_type" json:"transport_type"`
	CustomConfig  map[string]interface{} `yaml:"custom_config,omitempty" json:"custom_config,omitempty"`
}

// PoolStats provides comprehensive statistics about pool performance and health
type PoolStats struct {
	// Connection counts
	TotalConnections  int `json:"total_connections"`
	ActiveConnections int `json:"active_connections"`
	IdleConnections   int `json:"idle_connections"`

	// Performance metrics
	AverageResponseTime time.Duration `json:"average_response_time"`
	RequestsPerSecond   float64       `json:"requests_per_second"`
	ErrorRate           float64       `json:"error_rate"`

	// Resource usage
	MemoryUsageMB   int64   `json:"memory_usage_mb"`
	CPUUsagePercent float64 `json:"cpu_usage_percent"`

	// Health status
	IsHealthy            bool      `json:"is_healthy"`
	LastHealthCheck      time.Time `json:"last_health_check"`
	UnhealthyConnections int       `json:"unhealthy_connections"`

	// Circuit breaker status
	CircuitState string `json:"circuit_state"`
	FailureCount int64  `json:"failure_count"`
	SuccessCount int64  `json:"success_count"`

	// Pool-specific metrics
	CreationRate      float64       `json:"creation_rate"`
	DestructionRate   float64       `json:"destruction_rate"`
	ReuseRate         float64       `json:"reuse_rate"`
	AvgConnectionAge  time.Duration `json:"avg_connection_age"`
	PeakConnections   int           `json:"peak_connections"`
	MinConnections    int           `json:"min_connections"`
	ConfiguredMaxSize int           `json:"configured_max_size"`
	ConfiguredMinSize int           `json:"configured_min_size"`
}

// PoolFactory defines the interface for creating and managing different types of connection pools
type PoolFactory interface {
	CreatePool(transport string, config *PoolConfig) (PoolManager, error)
	GetDefaultConfig(transport string) *PoolConfig
	ValidateConfig(config *PoolConfig) error
	GetSupportedTransports() []string
}

// EnhancedPoolFactory implements the PoolFactory interface with support for multiple transport types
type EnhancedPoolFactory struct {
	defaultConfigs map[string]*PoolConfig
	poolCreators   map[string]func(*PoolConfig) (PoolManager, error)
	mu             sync.RWMutex
}

// NewEnhancedPoolFactory creates a new factory with default configurations for all transport types
func NewEnhancedPoolFactory() *EnhancedPoolFactory {
	factory := &EnhancedPoolFactory{
		defaultConfigs: make(map[string]*PoolConfig),
		poolCreators:   make(map[string]func(*PoolConfig) (PoolManager, error)),
	}

	// Set up default configurations for each transport type
	factory.initializeDefaultConfigs()
	factory.registerPoolCreators()

	return factory
}

// CreatePool creates a new pool manager for the specified transport type
func (epf *EnhancedPoolFactory) CreatePool(transport string, config *PoolConfig) (PoolManager, error) {
	epf.mu.RLock()
	creator, exists := epf.poolCreators[transport]
	epf.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unsupported transport type: %s", transport)
	}

	if config == nil {
		config = epf.GetDefaultConfig(transport)
	}

	if err := epf.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid pool configuration: %w", err)
	}

	return creator(config)
}

// GetDefaultConfig returns the default configuration for the specified transport type
func (epf *EnhancedPoolFactory) GetDefaultConfig(transport string) *PoolConfig {
	epf.mu.RLock()
	defer epf.mu.RUnlock()

	if config, exists := epf.defaultConfigs[transport]; exists {
		// Return a deep copy to prevent modifications
		return epf.copyPoolConfig(config)
	}

	// Return a generic default configuration
	return epf.getGenericDefaultConfig(transport)
}

// ValidateConfig validates a pool configuration for correctness and consistency
func (epf *EnhancedPoolFactory) ValidateConfig(config *PoolConfig) error {
	if config == nil {
		return fmt.Errorf("pool configuration cannot be nil")
	}

	// Validate size constraints
	if config.MaxSize <= 0 {
		return fmt.Errorf("max_size must be greater than 0")
	}
	if config.MinSize < 0 {
		return fmt.Errorf("min_size cannot be negative")
	}
	if config.MinSize > config.MaxSize {
		return fmt.Errorf("min_size (%d) cannot be greater than max_size (%d)", config.MinSize, config.MaxSize)
	}
	if config.WarmupSize > config.MaxSize {
		return fmt.Errorf("warmup_size (%d) cannot be greater than max_size (%d)", config.WarmupSize, config.MaxSize)
	}

	// Validate dynamic sizing thresholds
	if config.EnableDynamicSizing {
		if config.TargetUtilization <= 0 || config.TargetUtilization > 1 {
			return fmt.Errorf("target_utilization must be between 0 and 1")
		}
		if config.ScaleUpThreshold <= config.TargetUtilization {
			return fmt.Errorf("scale_up_threshold must be greater than target_utilization")
		}
		if config.ScaleDownThreshold >= config.TargetUtilization {
			return fmt.Errorf("scale_down_threshold must be less than target_utilization")
		}
	}

	// Validate timeouts
	if config.MaxLifetime < 0 {
		return fmt.Errorf("max_lifetime cannot be negative")
	}
	if config.IdleTimeout < 0 {
		return fmt.Errorf("idle_timeout cannot be negative")
	}
	if config.HealthCheckInterval < 0 {
		return fmt.Errorf("health_check_interval cannot be negative")
	}

	// Validate circuit breaker settings
	if config.MaxRetries < 0 {
		return fmt.Errorf("max_retries cannot be negative")
	}
	if config.BaseDelay < 0 {
		return fmt.Errorf("base_delay cannot be negative")
	}
	if config.CircuitTimeout < 0 {
		return fmt.Errorf("circuit_timeout cannot be negative")
	}

	// Validate resource limits
	if config.MemoryLimitMB < 0 {
		return fmt.Errorf("memory_limit_mb cannot be negative")
	}
	if config.CPULimitPercent < 0 || config.CPULimitPercent > 100 {
		return fmt.Errorf("cpu_limit_percent must be between 0 and 100")
	}

	return nil
}

// GetSupportedTransports returns a list of supported transport types
func (epf *EnhancedPoolFactory) GetSupportedTransports() []string {
	epf.mu.RLock()
	defer epf.mu.RUnlock()

	transports := make([]string, 0, len(epf.poolCreators))
	for transport := range epf.poolCreators {
		transports = append(transports, transport)
	}
	return transports
}

// initializeDefaultConfigs sets up default configurations for all transport types
func (epf *EnhancedPoolFactory) initializeDefaultConfigs() {
	// TCP transport default configuration
	epf.defaultConfigs[TransportTCP] = &PoolConfig{
		MinSize:             1,
		MaxSize:             10,
		WarmupSize:          2,
		EnableDynamicSizing: true,
		TargetUtilization:   0.7,
		ScaleUpThreshold:    0.8,
		ScaleDownThreshold:  0.5,
		MaxLifetime:         30 * time.Minute,
		IdleTimeout:         5 * time.Minute,
		HealthCheckInterval: 30 * time.Second,
		MaxRetries:          3,
		BaseDelay:           100 * time.Millisecond,
		CircuitTimeout:      10 * time.Second,
		MemoryLimitMB:       100,
		CPULimitPercent:     80.0,
		TransportType:       TransportTCP,
	}

	// STDIO transport default configuration
	epf.defaultConfigs[TransportStdio] = &PoolConfig{
		MinSize:             1,
		MaxSize:             5,
		WarmupSize:          1,
		EnableDynamicSizing: false, // Less aggressive for STDIO
		TargetUtilization:   0.8,
		ScaleUpThreshold:    0.9,
		ScaleDownThreshold:  0.6,
		MaxLifetime:         15 * time.Minute,
		IdleTimeout:         3 * time.Minute,
		HealthCheckInterval: 1 * time.Minute,
		MaxRetries:          5,
		BaseDelay:           200 * time.Millisecond,
		CircuitTimeout:      15 * time.Second,
		MemoryLimitMB:       50,
		CPULimitPercent:     60.0,
		TransportType:       TransportStdio,
	}

	// HTTP transport default configuration
	epf.defaultConfigs[TransportHTTP] = &PoolConfig{
		MinSize:             2,
		MaxSize:             20,
		WarmupSize:          3,
		EnableDynamicSizing: true,
		TargetUtilization:   0.6,
		ScaleUpThreshold:    0.75,
		ScaleDownThreshold:  0.4,
		MaxLifetime:         45 * time.Minute,
		IdleTimeout:         10 * time.Minute,
		HealthCheckInterval: 20 * time.Second,
		MaxRetries:          2,
		BaseDelay:           50 * time.Millisecond,
		CircuitTimeout:      5 * time.Second,
		MemoryLimitMB:       200,
		CPULimitPercent:     70.0,
		TransportType:       TransportHTTP,
	}
}

// registerPoolCreators registers the factory functions for each transport type
func (epf *EnhancedPoolFactory) registerPoolCreators() {
	epf.poolCreators[TransportTCP] = func(config *PoolConfig) (PoolManager, error) {
		return NewEnhancedTCPPoolManager(config)
	}

	epf.poolCreators[TransportStdio] = func(config *PoolConfig) (PoolManager, error) {
		return NewEnhancedStdioPoolManager(config)
	}

	epf.poolCreators[TransportHTTP] = func(config *PoolConfig) (PoolManager, error) {
		return NewEnhancedHTTPPoolManager(config)
	}
}

// copyPoolConfig creates a deep copy of a pool configuration
func (epf *EnhancedPoolFactory) copyPoolConfig(original *PoolConfig) *PoolConfig {
	if original == nil {
		return nil
	}

	copy := *original

	// Deep copy the custom config map
	if original.CustomConfig != nil {
		copy.CustomConfig = make(map[string]interface{})
		for k, v := range original.CustomConfig {
			copy.CustomConfig[k] = v
		}
	}

	return &copy
}

// getGenericDefaultConfig returns a generic default configuration for unknown transport types
func (epf *EnhancedPoolFactory) getGenericDefaultConfig(transport string) *PoolConfig {
	return &PoolConfig{
		MinSize:             1,
		MaxSize:             5,
		WarmupSize:          1,
		EnableDynamicSizing: false,
		TargetUtilization:   0.7,
		ScaleUpThreshold:    0.8,
		ScaleDownThreshold:  0.5,
		MaxLifetime:         20 * time.Minute,
		IdleTimeout:         5 * time.Minute,
		HealthCheckInterval: 1 * time.Minute,
		MaxRetries:          3,
		BaseDelay:           100 * time.Millisecond,
		CircuitTimeout:      10 * time.Second,
		MemoryLimitMB:       50,
		CPULimitPercent:     60.0,
		TransportType:       transport,
	}
}

// Placeholder functions for pool managers - these would be implemented in separate files
func NewEnhancedTCPPoolManager(config *PoolConfig) (PoolManager, error) {
	// This would be implemented in a separate file (e.g., tcp_pool_manager.go)
	return nil, fmt.Errorf("enhanced TCP pool manager not yet implemented")
}

func NewEnhancedStdioPoolManager(config *PoolConfig) (PoolManager, error) {
	// This would be implemented in a separate file (e.g., stdio_pool_manager.go)
	return nil, fmt.Errorf("enhanced STDIO pool manager not yet implemented")
}

func NewEnhancedHTTPPoolManager(config *PoolConfig) (PoolManager, error) {
	// This would be implemented in a separate file (e.g., http_pool_manager.go)
	return nil, fmt.Errorf("enhanced HTTP pool manager not yet implemented")
}
