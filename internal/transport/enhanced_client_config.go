package transport

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// EnhancedClientConfig extends the basic ClientConfig with comprehensive pool management
type EnhancedClientConfig struct {
	// Existing basic fields (backward compatible with ClientConfig)
	Command   string   `yaml:"command" json:"command"`
	Args      []string `yaml:"args" json:"args"`
	Transport string   `yaml:"transport" json:"transport"`

	// New pool configuration
	PoolConfig *PoolConfig `yaml:"pool_config,omitempty" json:"pool_config,omitempty"`

	// Server-specific settings
	ServerName  string   `yaml:"server_name" json:"server_name"`
	Languages   []string `yaml:"languages" json:"languages"`
	RootMarkers []string `yaml:"root_markers" json:"root_markers"`

	// Advanced connection settings
	ConnectionSettings *ConnectionSettings `yaml:"connection_settings,omitempty" json:"connection_settings,omitempty"`

	// Environment and working directory
	Environment map[string]string `yaml:"environment,omitempty" json:"environment,omitempty"`
	WorkingDir  string            `yaml:"working_dir,omitempty" json:"working_dir,omitempty"`

	// Initialization and health check settings
	InitializationTimeout time.Duration        `yaml:"initialization_timeout,omitempty" json:"initialization_timeout,omitempty"`
	HealthCheckSettings   *HealthCheckSettings `yaml:"health_check_settings,omitempty" json:"health_check_settings,omitempty"`

	// Logging and debugging
	LogLevel     string `yaml:"log_level,omitempty" json:"log_level,omitempty"`
	EnableDebug  bool   `yaml:"enable_debug,omitempty" json:"enable_debug,omitempty"`
	LogRequests  bool   `yaml:"log_requests,omitempty" json:"log_requests,omitempty"`
	LogResponses bool   `yaml:"log_responses,omitempty" json:"log_responses,omitempty"`
}

// ConnectionSettings contains transport-specific connection parameters
type ConnectionSettings struct {
	// TCP-specific settings
	TCPAddress      string        `yaml:"tcp_address,omitempty" json:"tcp_address,omitempty"`
	TCPPort         int           `yaml:"tcp_port,omitempty" json:"tcp_port,omitempty"`
	ConnectTimeout  time.Duration `yaml:"connect_timeout,omitempty" json:"connect_timeout,omitempty"`
	ReadTimeout     time.Duration `yaml:"read_timeout,omitempty" json:"read_timeout,omitempty"`
	WriteTimeout    time.Duration `yaml:"write_timeout,omitempty" json:"write_timeout,omitempty"`
	KeepAlive       bool          `yaml:"keep_alive,omitempty" json:"keep_alive,omitempty"`
	KeepAlivePeriod time.Duration `yaml:"keep_alive_period,omitempty" json:"keep_alive_period,omitempty"`

	// STDIO-specific settings
	BufferSize       int           `yaml:"buffer_size,omitempty" json:"buffer_size,omitempty"`
	StdoutBufferSize int           `yaml:"stdout_buffer_size,omitempty" json:"stdout_buffer_size,omitempty"`
	StderrBufferSize int           `yaml:"stderr_buffer_size,omitempty" json:"stderr_buffer_size,omitempty"`
	ProcessTimeout   time.Duration `yaml:"process_timeout,omitempty" json:"process_timeout,omitempty"`

	// HTTP-specific settings
	HTTPEndpoint    string            `yaml:"http_endpoint,omitempty" json:"http_endpoint,omitempty"`
	HTTPHeaders     map[string]string `yaml:"http_headers,omitempty" json:"http_headers,omitempty"`
	HTTPTimeout     time.Duration     `yaml:"http_timeout,omitempty" json:"http_timeout,omitempty"`
	MaxIdleConns    int               `yaml:"max_idle_conns,omitempty" json:"max_idle_conns,omitempty"`
	MaxConnsPerHost int               `yaml:"max_conns_per_host,omitempty" json:"max_conns_per_host,omitempty"`

	// TLS settings (for HTTPS/secure TCP)
	EnableTLS             bool     `yaml:"enable_tls,omitempty" json:"enable_tls,omitempty"`
	TLSCertFile           string   `yaml:"tls_cert_file,omitempty" json:"tls_cert_file,omitempty"`
	TLSKeyFile            string   `yaml:"tls_key_file,omitempty" json:"tls_key_file,omitempty"`
	TLSCAFile             string   `yaml:"tls_ca_file,omitempty" json:"tls_ca_file,omitempty"`
	TLSInsecureSkipVerify bool     `yaml:"tls_insecure_skip_verify,omitempty" json:"tls_insecure_skip_verify,omitempty"`
	TLSServerName         string   `yaml:"tls_server_name,omitempty" json:"tls_server_name,omitempty"`
	TLSCipherSuites       []string `yaml:"tls_cipher_suites,omitempty" json:"tls_cipher_suites,omitempty"`
}

// HealthCheckSettings defines how health checks should be performed
type HealthCheckSettings struct {
	Enabled          bool          `yaml:"enabled" json:"enabled"`
	Interval         time.Duration `yaml:"interval" json:"interval"`
	Timeout          time.Duration `yaml:"timeout" json:"timeout"`
	FailureThreshold int           `yaml:"failure_threshold" json:"failure_threshold"`
	SuccessThreshold int           `yaml:"success_threshold" json:"success_threshold"`

	// Health check method - "ping", "initialize", "custom"
	Method       string                 `yaml:"method" json:"method"`
	CustomParams map[string]interface{} `yaml:"custom_params,omitempty" json:"custom_params,omitempty"`

	// Recovery settings
	EnableAutoRestart   bool          `yaml:"enable_auto_restart" json:"enable_auto_restart"`
	RestartDelay        time.Duration `yaml:"restart_delay" json:"restart_delay"`
	MaxConsecutiveFails int           `yaml:"max_consecutive_fails" json:"max_consecutive_fails"`
}

// Validate validates the enhanced client configuration
func (ecc *EnhancedClientConfig) Validate() error {
	// Validate basic required fields
	if ecc.Command == "" {
		return fmt.Errorf("command is required")
	}

	if ecc.Transport == "" {
		return fmt.Errorf("transport type is required")
	}

	// Validate transport type
	supportedTransports := []string{TransportStdio, TransportTCP, TransportHTTP}
	validTransport := false
	for _, t := range supportedTransports {
		if ecc.Transport == t {
			validTransport = true
			break
		}
	}
	if !validTransport {
		return fmt.Errorf("unsupported transport type: %s", ecc.Transport)
	}

	// Validate pool configuration if provided
	if ecc.PoolConfig != nil {
		factory := NewEnhancedPoolFactory()
		if err := factory.ValidateConfig(ecc.PoolConfig); err != nil {
			return fmt.Errorf("invalid pool configuration: %w", err)
		}
	}

	// Validate connection settings if provided
	if ecc.ConnectionSettings != nil {
		if err := ecc.validateConnectionSettings(); err != nil {
			return fmt.Errorf("invalid connection settings: %w", err)
		}
	}

	// Validate health check settings if provided
	if ecc.HealthCheckSettings != nil {
		if err := ecc.validateHealthCheckSettings(); err != nil {
			return fmt.Errorf("invalid health check settings: %w", err)
		}
	}

	// Validate timeouts
	if ecc.InitializationTimeout < 0 {
		return fmt.Errorf("initialization_timeout cannot be negative")
	}

	// Validate server name if provided
	if ecc.ServerName == "" && len(ecc.Languages) > 0 {
		return fmt.Errorf("server_name is required when languages are specified")
	}

	return nil
}

// SetDefaults applies default values to unset configuration fields
func (ecc *EnhancedClientConfig) SetDefaults() error {
	// Set default pool configuration if not provided
	if ecc.PoolConfig == nil {
		factory := NewEnhancedPoolFactory()
		ecc.PoolConfig = factory.GetDefaultConfig(ecc.Transport)
	}

	// Set default connection settings if not provided
	if ecc.ConnectionSettings == nil {
		ecc.ConnectionSettings = ecc.getDefaultConnectionSettings()
	}

	// Set default health check settings if not provided
	if ecc.HealthCheckSettings == nil {
		ecc.HealthCheckSettings = ecc.getDefaultHealthCheckSettings()
	}

	// Set default initialization timeout
	if ecc.InitializationTimeout == 0 {
		ecc.InitializationTimeout = 30 * time.Second
	}

	// Set default log level
	if ecc.LogLevel == "" {
		ecc.LogLevel = "info"
	}

	// Set transport-specific defaults
	switch ecc.Transport {
	case TransportTCP:
		if ecc.ConnectionSettings.TCPPort == 0 {
			ecc.ConnectionSettings.TCPPort = 7070 // Default LSP port
		}
		if ecc.ConnectionSettings.ConnectTimeout == 0 {
			ecc.ConnectionSettings.ConnectTimeout = 10 * time.Second
		}
	case TransportStdio:
		if ecc.ConnectionSettings.BufferSize == 0 {
			ecc.ConnectionSettings.BufferSize = 8192
		}
		if ecc.ConnectionSettings.ProcessTimeout == 0 {
			ecc.ConnectionSettings.ProcessTimeout = 30 * time.Second
		}
	case TransportHTTP:
		if ecc.ConnectionSettings.HTTPTimeout == 0 {
			ecc.ConnectionSettings.HTTPTimeout = 30 * time.Second
		}
		if ecc.ConnectionSettings.MaxIdleConns == 0 {
			ecc.ConnectionSettings.MaxIdleConns = 100
		}
	}

	return nil
}

// GetPoolConfig returns the pool configuration, creating a default if needed
func (ecc *EnhancedClientConfig) GetPoolConfig() *PoolConfig {
	if ecc.PoolConfig == nil {
		factory := NewEnhancedPoolFactory()
		ecc.PoolConfig = factory.GetDefaultConfig(ecc.Transport)
	}
	return ecc.PoolConfig
}

// ToBasicClientConfig converts to the basic ClientConfig for backward compatibility
func (ecc *EnhancedClientConfig) ToBasicClientConfig() *ClientConfig {
	return &ClientConfig{
		Command:   ecc.Command,
		Args:      ecc.Args,
		Transport: ecc.Transport,
	}
}

// FromBasicClientConfig creates an enhanced config from a basic config
func FromBasicClientConfig(basic *ClientConfig) *EnhancedClientConfig {
	enhanced := &EnhancedClientConfig{
		Command:   basic.Command,
		Args:      basic.Args,
		Transport: basic.Transport,
	}

	// Apply defaults
	if err := enhanced.SetDefaults(); err != nil {
		// SetDefaults is critical for proper initialization, panic if it fails
		panic(fmt.Sprintf("failed to set defaults for enhanced client config: %v", err))
	}

	return enhanced
}

// Clone creates a deep copy of the configuration
func (ecc *EnhancedClientConfig) Clone() *EnhancedClientConfig {
	clone := &EnhancedClientConfig{
		Command:               ecc.Command,
		Args:                  make([]string, len(ecc.Args)),
		Transport:             ecc.Transport,
		ServerName:            ecc.ServerName,
		Languages:             make([]string, len(ecc.Languages)),
		RootMarkers:           make([]string, len(ecc.RootMarkers)),
		WorkingDir:            ecc.WorkingDir,
		InitializationTimeout: ecc.InitializationTimeout,
		LogLevel:              ecc.LogLevel,
		EnableDebug:           ecc.EnableDebug,
		LogRequests:           ecc.LogRequests,
		LogResponses:          ecc.LogResponses,
	}

	copy(clone.Args, ecc.Args)
	copy(clone.Languages, ecc.Languages)
	copy(clone.RootMarkers, ecc.RootMarkers)

	// Deep copy environment
	if ecc.Environment != nil {
		clone.Environment = make(map[string]string)
		for k, v := range ecc.Environment {
			clone.Environment[k] = v
		}
	}

	// Deep copy pool config
	if ecc.PoolConfig != nil {
		factory := NewEnhancedPoolFactory()
		clone.PoolConfig = factory.copyPoolConfig(ecc.PoolConfig)
	}

	// Deep copy connection settings
	if ecc.ConnectionSettings != nil {
		clone.ConnectionSettings = ecc.cloneConnectionSettings()
	}

	// Deep copy health check settings
	if ecc.HealthCheckSettings != nil {
		clone.HealthCheckSettings = ecc.cloneHealthCheckSettings()
	}

	return clone
}

// validateConnectionSettings validates transport-specific connection settings
func (ecc *EnhancedClientConfig) validateConnectionSettings() error {
	cs := ecc.ConnectionSettings

	switch ecc.Transport {
	case TransportTCP:
		if cs.TCPPort < 0 || cs.TCPPort > 65535 {
			return fmt.Errorf("tcp_port must be between 0 and 65535")
		}
		if cs.ConnectTimeout < 0 {
			return fmt.Errorf("connect_timeout cannot be negative")
		}
	case TransportStdio:
		if cs.BufferSize < 0 {
			return fmt.Errorf("buffer_size cannot be negative")
		}
		if cs.ProcessTimeout < 0 {
			return fmt.Errorf("process_timeout cannot be negative")
		}
	case TransportHTTP:
		if cs.HTTPTimeout < 0 {
			return fmt.Errorf("http_timeout cannot be negative")
		}
		if cs.MaxIdleConns < 0 {
			return fmt.Errorf("max_idle_conns cannot be negative")
		}
	}

	return nil
}

// validateHealthCheckSettings validates health check configuration
func (ecc *EnhancedClientConfig) validateHealthCheckSettings() error {
	hc := ecc.HealthCheckSettings

	if hc.Enabled {
		if hc.Interval <= 0 {
			return fmt.Errorf("health check interval must be positive")
		}
		if hc.Timeout <= 0 {
			return fmt.Errorf("health check timeout must be positive")
		}
		if hc.FailureThreshold <= 0 {
			return fmt.Errorf("failure threshold must be positive")
		}
		if hc.SuccessThreshold <= 0 {
			return fmt.Errorf("success threshold must be positive")
		}

		validMethods := []string{"ping", "initialize", "custom"}
		validMethod := false
		for _, method := range validMethods {
			if hc.Method == method {
				validMethod = true
				break
			}
		}
		if !validMethod {
			return fmt.Errorf("invalid health check method: %s", hc.Method)
		}
	}

	return nil
}

// getDefaultConnectionSettings returns default connection settings for the transport type
func (ecc *EnhancedClientConfig) getDefaultConnectionSettings() *ConnectionSettings {
	settings := &ConnectionSettings{}

	switch ecc.Transport {
	case TransportTCP:
		settings.TCPAddress = "localhost"
		settings.TCPPort = 7070
		settings.ConnectTimeout = 10 * time.Second
		settings.ReadTimeout = 30 * time.Second
		settings.WriteTimeout = 30 * time.Second
		settings.KeepAlive = true
		settings.KeepAlivePeriod = 30 * time.Second
	case TransportStdio:
		settings.BufferSize = 8192
		settings.StdoutBufferSize = 4096
		settings.StderrBufferSize = 4096
		settings.ProcessTimeout = 30 * time.Second
	case TransportHTTP:
		settings.HTTPTimeout = 30 * time.Second
		settings.MaxIdleConns = 100
		settings.MaxConnsPerHost = 10
	}

	return settings
}

// getDefaultHealthCheckSettings returns default health check settings
func (ecc *EnhancedClientConfig) getDefaultHealthCheckSettings() *HealthCheckSettings {
	return &HealthCheckSettings{
		Enabled:             true,
		Interval:            30 * time.Second,
		Timeout:             10 * time.Second,
		FailureThreshold:    3,
		SuccessThreshold:    1,
		Method:              "ping",
		EnableAutoRestart:   true,
		RestartDelay:        5 * time.Second,
		MaxConsecutiveFails: 5,
	}
}

// cloneConnectionSettings creates a deep copy of connection settings
func (ecc *EnhancedClientConfig) cloneConnectionSettings() *ConnectionSettings {
	if ecc.ConnectionSettings == nil {
		return nil
	}

	cs := ecc.ConnectionSettings
	clone := &ConnectionSettings{
		TCPAddress:            cs.TCPAddress,
		TCPPort:               cs.TCPPort,
		ConnectTimeout:        cs.ConnectTimeout,
		ReadTimeout:           cs.ReadTimeout,
		WriteTimeout:          cs.WriteTimeout,
		KeepAlive:             cs.KeepAlive,
		KeepAlivePeriod:       cs.KeepAlivePeriod,
		BufferSize:            cs.BufferSize,
		StdoutBufferSize:      cs.StdoutBufferSize,
		StderrBufferSize:      cs.StderrBufferSize,
		ProcessTimeout:        cs.ProcessTimeout,
		HTTPEndpoint:          cs.HTTPEndpoint,
		HTTPTimeout:           cs.HTTPTimeout,
		MaxIdleConns:          cs.MaxIdleConns,
		MaxConnsPerHost:       cs.MaxConnsPerHost,
		EnableTLS:             cs.EnableTLS,
		TLSCertFile:           cs.TLSCertFile,
		TLSKeyFile:            cs.TLSKeyFile,
		TLSCAFile:             cs.TLSCAFile,
		TLSInsecureSkipVerify: cs.TLSInsecureSkipVerify,
		TLSServerName:         cs.TLSServerName,
		TLSCipherSuites:       make([]string, len(cs.TLSCipherSuites)),
	}

	copy(clone.TLSCipherSuites, cs.TLSCipherSuites)

	if cs.HTTPHeaders != nil {
		clone.HTTPHeaders = make(map[string]string)
		for k, v := range cs.HTTPHeaders {
			clone.HTTPHeaders[k] = v
		}
	}

	return clone
}

// cloneHealthCheckSettings creates a deep copy of health check settings
func (ecc *EnhancedClientConfig) cloneHealthCheckSettings() *HealthCheckSettings {
	if ecc.HealthCheckSettings == nil {
		return nil
	}

	hc := ecc.HealthCheckSettings
	clone := &HealthCheckSettings{
		Enabled:             hc.Enabled,
		Interval:            hc.Interval,
		Timeout:             hc.Timeout,
		FailureThreshold:    hc.FailureThreshold,
		SuccessThreshold:    hc.SuccessThreshold,
		Method:              hc.Method,
		EnableAutoRestart:   hc.EnableAutoRestart,
		RestartDelay:        hc.RestartDelay,
		MaxConsecutiveFails: hc.MaxConsecutiveFails,
	}

	if hc.CustomParams != nil {
		clone.CustomParams = make(map[string]interface{})
		for k, v := range hc.CustomParams {
			clone.CustomParams[k] = v
		}
	}

	return clone
}

// Configuration helper functions

// LoadPoolConfigFromYAML loads pool configuration from YAML data
func LoadPoolConfigFromYAML(yamlData []byte) (*PoolConfig, error) {
	var config PoolConfig
	if err := yaml.Unmarshal(yamlData, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pool config: %w", err)
	}

	factory := NewEnhancedPoolFactory()
	if err := factory.ValidateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid pool configuration: %w", err)
	}

	return &config, nil
}

// MergePoolConfigs merges two pool configurations, with override taking precedence
func MergePoolConfigs(base, override *PoolConfig) *PoolConfig {
	if base == nil {
		return override
	}
	if override == nil {
		return base
	}

	merged := *base

	// Override non-zero values
	if override.MinSize != 0 {
		merged.MinSize = override.MinSize
	}
	if override.MaxSize != 0 {
		merged.MaxSize = override.MaxSize
	}
	if override.WarmupSize != 0 {
		merged.WarmupSize = override.WarmupSize
	}
	if override.TargetUtilization != 0 {
		merged.TargetUtilization = override.TargetUtilization
	}
	if override.ScaleUpThreshold != 0 {
		merged.ScaleUpThreshold = override.ScaleUpThreshold
	}
	if override.ScaleDownThreshold != 0 {
		merged.ScaleDownThreshold = override.ScaleDownThreshold
	}
	if override.MaxLifetime != 0 {
		merged.MaxLifetime = override.MaxLifetime
	}
	if override.IdleTimeout != 0 {
		merged.IdleTimeout = override.IdleTimeout
	}
	if override.HealthCheckInterval != 0 {
		merged.HealthCheckInterval = override.HealthCheckInterval
	}
	if override.MaxRetries != 0 {
		merged.MaxRetries = override.MaxRetries
	}
	if override.BaseDelay != 0 {
		merged.BaseDelay = override.BaseDelay
	}
	if override.CircuitTimeout != 0 {
		merged.CircuitTimeout = override.CircuitTimeout
	}
	if override.MemoryLimitMB != 0 {
		merged.MemoryLimitMB = override.MemoryLimitMB
	}
	if override.CPULimitPercent != 0 {
		merged.CPULimitPercent = override.CPULimitPercent
	}

	// Boolean fields
	merged.EnableDynamicSizing = override.EnableDynamicSizing

	// String fields
	if override.TransportType != "" {
		merged.TransportType = override.TransportType
	}

	// Merge custom config
	if override.CustomConfig != nil {
		if merged.CustomConfig == nil {
			merged.CustomConfig = make(map[string]interface{})
		}
		for k, v := range override.CustomConfig {
			merged.CustomConfig[k] = v
		}
	}

	return &merged
}
