package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
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

	return &config, nil
}

func SaveConfig(config *GatewayConfig, configPath string) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	if configPath == "" {
		configPath = DefaultConfigFile
	}

	if err := ValidateConfig(config); err != nil {
		return fmt.Errorf("cannot save invalid configuration: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration to YAML: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write configuration file %s: %w", configPath, err)
	}

	return nil
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
