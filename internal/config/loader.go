package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

// LoadConfig loads configuration from a YAML file with fallback to "config.yaml"
func LoadConfig(configPath string) (*GatewayConfig, error) {
	// Use default config file if no path provided
	if configPath == "" {
		configPath = DefaultConfigFile
	}

	// Check if the config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file not found: %s", configPath)
	}

	// Read the configuration file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file %s: %w", configPath, err)
	}

	// Parse the YAML configuration
	var config GatewayConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse configuration file %s: %w", configPath, err)
	}

	// Set default port if not specified
	if config.Port == 0 {
		config.Port = 8080
	}

	// Set default transport for servers if not specified
	for i := range config.Servers {
		if config.Servers[i].Transport == "" {
			config.Servers[i].Transport = DefaultTransport
		}
	}

	return &config, nil
}

// ValidateConfig performs comprehensive validation of the configuration
func ValidateConfig(config *GatewayConfig) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	// Use the existing validation method from the config struct
	if err := config.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	return nil
}

// LoadAndValidateConfig loads and validates configuration in one step
func LoadAndValidateConfig(configPath string) (*GatewayConfig, error) {
	config, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}

	if err := ValidateConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}
