package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"lsp-gateway/src/internal/common"
)

// Config contains LSP server configuration
type Config struct {
	Servers map[string]*ServerConfig `yaml:"servers"`
}

// ServerConfig contains configuration for a single LSP server
type ServerConfig struct {
	Command               string      `yaml:"command"`
	Args                  []string    `yaml:"args"`
	WorkingDir            string      `yaml:"working_dir,omitempty"`
	InitializationOptions interface{} `yaml:"initialization_options,omitempty"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate config
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

// SaveConfig saves configuration to a YAML file
func SaveConfig(config *Config, path string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GenerateDefaultConfig generates a default configuration file
func GenerateDefaultConfig(path string) error {
	config := GetDefaultConfig()
	return SaveConfig(config, path)
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	if config.Servers == nil {
		return fmt.Errorf("servers configuration is required")
	}

	for language, serverConfig := range config.Servers {
		if serverConfig.Command == "" {
			return fmt.Errorf("command is required for language %s", language)
		}
	}

	return nil
}

// GetDefaultConfigPath returns the default configuration file path
func GetDefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".lsp-gateway", "config.yaml")
}

// GetDefaultConfig returns a default configuration for common LSP servers
func GetDefaultConfig() *Config {
	return &Config{
		Servers: map[string]*ServerConfig{
			"go": {
				Command: "gopls",
				Args:    []string{"serve"},
			},
			"python": {
				Command: "pylsp",
				Args:    []string{},
			},
			"javascript": {
				Command: "typescript-language-server",
				Args:    []string{"--stdio"},
			},
			"typescript": {
				Command: "typescript-language-server",
				Args:    []string{"--stdio"},
			},
			"java": {
				Command: "jdtls",
				Args:    []string{},
			},
		},
	}
}

// GenerateConfigForLanguages generates a Config for the specified languages only
// Uses the same server commands as GetDefaultConfig() but only includes requested languages
func GenerateConfigForLanguages(languages []string) *Config {
	if len(languages) == 0 {
		common.CLILogger.Warn("No languages provided, returning empty config")
		return &Config{Servers: make(map[string]*ServerConfig)}
	}

	// Get the full default configuration for reference
	defaultConfig := GetDefaultConfig()

	// Create new config with only requested languages
	config := &Config{
		Servers: make(map[string]*ServerConfig),
	}

	for _, language := range languages {
		if serverConfig, exists := defaultConfig.Servers[language]; exists {
			// Create a copy of the server config
			config.Servers[language] = &ServerConfig{
				Command:               serverConfig.Command,
				Args:                  append([]string{}, serverConfig.Args...), // Copy slice
				WorkingDir:            serverConfig.WorkingDir,
				InitializationOptions: serverConfig.InitializationOptions,
			}
			common.CLILogger.Info("Added %s server configuration", language)
		} else {
			common.CLILogger.Warn("No default configuration found for language: %s", language)
		}
	}

	if len(config.Servers) == 0 {
		common.CLILogger.Warn("No valid server configurations generated")
	}

	return config
}

// DetectAndGenerateConfig generates a Config using a provided language detector function
// This avoids circular dependency by accepting the detector as a parameter
func DetectAndGenerateConfig(workingDir string, detector func(string) ([]string, error)) *Config {
	if detector == nil {
		common.CLILogger.Warn("No language detector provided, returning default config")
		return GetDefaultConfig()
	}

	if workingDir == "" {
		var err error
		workingDir, err = os.Getwd()
		if err != nil {
			common.CLILogger.Error("Failed to get working directory: %v", err)
			return GetDefaultConfig()
		}
	}

	// Detect available languages using the provided detector
	languages, err := detector(workingDir)
	if err != nil {
		common.CLILogger.Error("Failed to detect languages: %v", err)
		return GetDefaultConfig()
	}

	if len(languages) == 0 {
		common.CLILogger.Warn("No languages detected, returning default config")
		return GetDefaultConfig()
	}

	common.CLILogger.Info("Detected languages: %v", languages)
	return GenerateConfigForLanguages(languages)
}

// GenerateAutoConfig is a convenience function that creates a configuration
// with only the languages that have available LSP servers in the working directory
// Usage: config := config.GenerateAutoConfig(".", project.GetAvailableLanguages)
func GenerateAutoConfig(workingDir string, getAvailableLanguages func(string) ([]string, error)) *Config {
	return DetectAndGenerateConfig(workingDir, getAvailableLanguages)
}
