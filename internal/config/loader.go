package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/goccy/go-yaml"
)

func LoadConfig(configPath string) (*GatewayConfig, error) {
	if configPath == "" {
		configPath = DefaultConfigFile
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file not found: %s", configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file %s: %w", configPath, err)
	}

	// Handle empty files
	if len(data) == 0 {
		return nil, fmt.Errorf("configuration file %s is empty", configPath)
	}

	var config GatewayConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse configuration file %s: %w", configPath, err)
	}

	if config.Port == 0 {
		config.Port = 8080
	}

	for i := range config.Servers {
		if config.Servers[i].Transport == "" {
			config.Servers[i].Transport = DefaultTransport
		}
	}

	// Set defaults for multi-server configuration
	config.EnsureMultiServerDefaults()

	// Validate multi-server configuration if present
	if err := config.ValidateMultiServerConfig(); err != nil {
		return nil, fmt.Errorf("multi-server configuration validation failed: %w", err)
	}

	// Validate configuration consistency
	if err := config.ValidateConsistency(); err != nil {
		return nil, fmt.Errorf("configuration consistency validation failed: %w", err)
	}

	// Apply SCIP environment variable overrides
	if err := config.ApplySCIPEnvironmentVariables(); err != nil {
		return nil, fmt.Errorf("failed to apply SCIP environment variables: %w", err)
	}

	return &config, nil
}

func ValidateConfig(config *GatewayConfig) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	if err := config.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	return nil
}

// ApplySCIPEnvironmentVariables applies environment variable overrides for SCIP configuration
func (gc *GatewayConfig) ApplySCIPEnvironmentVariables() error {
	if gc.PerformanceConfig == nil {
		return nil
	}
	
	if gc.PerformanceConfig.SCIP == nil {
		return nil
	}

	return gc.PerformanceConfig.SCIP.ApplyEnvironmentDefaults()
}

// ApplyEnvironmentDefaults applies environment variable overrides for SCIP configuration
func (sc *SCIPConfiguration) ApplyEnvironmentDefaults() error {
	if sc == nil {
		return nil
	}

	// SCIP Enabled
	if enabled, err := getBoolEnv(ENV_LSP_GATEWAY_SCIP_ENABLED); err != nil {
		return fmt.Errorf("invalid %s: %w", ENV_LSP_GATEWAY_SCIP_ENABLED, err)
	} else if enabled != nil {
		sc.Enabled = *enabled
	}

	// SCIP Index Path
	if indexPath := getStringEnv(ENV_LSP_GATEWAY_SCIP_INDEX_PATH); indexPath != "" {
		sc.IndexPath = indexPath
	}

	// SCIP Auto Refresh
	if autoRefresh, err := getBoolEnv(ENV_LSP_GATEWAY_SCIP_AUTO_REFRESH); err != nil {
		return fmt.Errorf("invalid %s: %w", ENV_LSP_GATEWAY_SCIP_AUTO_REFRESH, err)
	} else if autoRefresh != nil {
		sc.AutoRefresh = *autoRefresh
	}

	// SCIP Refresh Interval
	if refreshInterval, err := getDurationEnv(ENV_LSP_GATEWAY_SCIP_REFRESH_INTERVAL); err != nil {
		return fmt.Errorf("invalid %s: %w", ENV_LSP_GATEWAY_SCIP_REFRESH_INTERVAL, err)
	} else if refreshInterval != nil {
		sc.RefreshInterval = *refreshInterval
	}

	// SCIP Fallback to LSP
	if fallbackToLSP, err := getBoolEnv(ENV_LSP_GATEWAY_SCIP_FALLBACK_TO_LSP); err != nil {
		return fmt.Errorf("invalid %s: %w", ENV_LSP_GATEWAY_SCIP_FALLBACK_TO_LSP, err)
	} else if fallbackToLSP != nil {
		sc.FallbackToLSP = *fallbackToLSP
	}

	// SCIP Cache TTL
	if cacheTTL, err := getDurationEnv(ENV_LSP_GATEWAY_SCIP_CACHE_TTL); err != nil {
		return fmt.Errorf("invalid %s: %w", ENV_LSP_GATEWAY_SCIP_CACHE_TTL, err)
	} else if cacheTTL != nil {
		sc.CacheConfig.TTL = *cacheTTL
	}

	// SCIP Cache Max Size
	if cacheMaxSize, err := getInt64Env(ENV_LSP_GATEWAY_SCIP_CACHE_MAX_SIZE); err != nil {
		return fmt.Errorf("invalid %s: %w", ENV_LSP_GATEWAY_SCIP_CACHE_MAX_SIZE, err)
	} else if cacheMaxSize != nil {
		sc.CacheConfig.MaxSize = *cacheMaxSize
	}

	return nil
}

// Helper functions for environment variable parsing

// getStringEnv gets a string environment variable
func getStringEnv(key string) string {
	return os.Getenv(key)
}

// getBoolEnv gets a boolean environment variable, returns nil if not set
func getBoolEnv(key string) (*bool, error) {
	value := os.Getenv(key)
	if value == "" {
		return nil, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil, err
	}

	return &parsed, nil
}

// getDurationEnv gets a duration environment variable, returns nil if not set
func getDurationEnv(key string) (*time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return nil, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return nil, err
	}

	return &parsed, nil
}

// getInt64Env gets an int64 environment variable, returns nil if not set
func getInt64Env(key string) (*int64, error) {
	value := os.Getenv(key)
	if value == "" {
		return nil, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil, err
	}

	return &parsed, nil
}
