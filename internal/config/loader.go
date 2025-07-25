package config

import (
	"fmt"
	"os"

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
