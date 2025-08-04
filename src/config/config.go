package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"gopkg.in/yaml.v3"
	"lsp-gateway/src/internal/common"
)

// Config contains LSP server configuration and optional SCIP cache settings
type Config struct {
	Servers map[string]*ServerConfig `yaml:"servers"`
	Cache   *SCIPConfig              `yaml:"cache"`
}

// ServerConfig contains configuration for a single LSP server
type ServerConfig struct {
	Command               string      `yaml:"command"`
	Args                  []string    `yaml:"args"`
	WorkingDir            string      `yaml:"working_dir,omitempty"`
	InitializationOptions interface{} `yaml:"initialization_options,omitempty"`
}

// SCIPConfig contains SCIP cache configuration settings
type SCIPConfig struct {
	Enabled             bool          `yaml:"enabled"`               // Enabled by default
	StoragePath         string        `yaml:"storage_path"`          // Cache storage directory
	MaxMemoryMB         int           `yaml:"max_memory_mb"`         // Memory limit in MB
	TTL                 time.Duration `yaml:"ttl"`                   // Cache TTL
	Languages           []string      `yaml:"languages"`             // Supported languages for caching
	BackgroundIndex     bool          `yaml:"background_index"`      // Enable background indexing
	HealthCheckInterval time.Duration `yaml:"health_check_interval"` // Health monitoring interval
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

	// Validate SCIP cache configuration if present
	if config.Cache != nil {
		if err := validateSCIPConfig(config.Cache); err != nil {
			return fmt.Errorf("invalid cache configuration: %w", err)
		}
	}

	return nil
}

// validateSCIPConfig validates SCIP cache configuration
func validateSCIPConfig(cache *SCIPConfig) error {
	// Memory validation
	if cache.MaxMemoryMB <= 0 {
		return fmt.Errorf("max_memory_mb must be greater than 0, got %d", cache.MaxMemoryMB)
	}

	// Reasonable memory limit check (warn if > 50% of system memory)
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	systemMemoryMB := int(memStats.Sys / 1024 / 1024)
	if cache.MaxMemoryMB > systemMemoryMB/2 {
		common.CLILogger.Warn("SCIP cache memory limit (%dMB) is more than 50%% of system memory (%dMB)",
			cache.MaxMemoryMB, systemMemoryMB)
	}

	// Storage path validation
	if cache.StoragePath == "" {
		return fmt.Errorf("storage_path cannot be empty")
	}

	// TTL validation
	if cache.TTL <= 0 {
		return fmt.Errorf("ttl must be greater than 0, got %v", cache.TTL)
	}
	if cache.TTL < time.Minute {
		return fmt.Errorf("ttl must be at least 1 minute, got %v", cache.TTL)
	}

	// Health check interval validation
	if cache.HealthCheckInterval <= 0 {
		return fmt.Errorf("health_check_interval must be greater than 0, got %v", cache.HealthCheckInterval)
	}
	if cache.HealthCheckInterval < time.Minute {
		return fmt.Errorf("health_check_interval must be at least 1 minute, got %v", cache.HealthCheckInterval)
	}

	// Language validation - ensure only supported languages
	supportedLanguages := map[string]bool{
		"go":         true,
		"python":     true,
		"javascript": true,
		"typescript": true,
		"java":       true,
	}

	for _, lang := range cache.Languages {
		if !supportedLanguages[lang] {
			return fmt.Errorf("unsupported language for caching: %s", lang)
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
// Cache is enabled by default with standard production settings
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
		// Cache is enabled by default with standard settings
		Cache: GetDefaultSCIPConfig(),
	}
}

// GetDefaultSCIPConfig returns a default SCIP cache configuration (enabled by default)
func GetDefaultSCIPConfig() *SCIPConfig {
	home, _ := os.UserHomeDir()
	return &SCIPConfig{
		Enabled:             true, // Enabled by default for improved performance
		StoragePath:         filepath.Join(home, ".lsp-gateway", "scip-cache"),
		MaxMemoryMB:         256,
		TTL:                 30 * time.Minute,
		Languages:           []string{"go", "python", "typescript", "java"},
		BackgroundIndex:     true,
		HealthCheckInterval: 5 * time.Minute,
	}
}

// GetDefaultConfigWithCache returns a default configuration with SCIP cache settings
// Cache is enabled by default with standard production settings
func GetDefaultConfigWithCache() *Config {
	// GetDefaultConfig() now includes cache by default, so just return it
	return GetDefaultConfig()
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

// SCIP Cache Management Functions

// HasCache returns true if the configuration has SCIP cache settings
func (c *Config) HasCache() bool {
	return c.Cache != nil
}

// IsCacheEnabled returns true if SCIP cache is enabled
func (c *Config) IsCacheEnabled() bool {
	return c.Cache != nil && c.Cache.Enabled
}

// EnableCache enables SCIP cache with default settings if not present
func (c *Config) EnableCache() {
	if c.Cache == nil {
		c.Cache = GetDefaultSCIPConfig()
	}
	c.Cache.Enabled = true
}

// DisableCache disables SCIP cache but preserves settings
func (c *Config) DisableCache() {
	if c.Cache != nil {
		c.Cache.Enabled = false
	}
}

// SetCacheStoragePath sets the SCIP cache storage path
func (c *Config) SetCacheStoragePath(path string) {
	if c.Cache == nil {
		c.Cache = GetDefaultSCIPConfig()
	}
	c.Cache.StoragePath = path
}

// SetCacheMemoryLimit sets the SCIP cache memory limit in MB
func (c *Config) SetCacheMemoryLimit(memoryMB int) error {
	if c.Cache == nil {
		c.Cache = GetDefaultSCIPConfig()
	}

	if memoryMB <= 0 {
		return fmt.Errorf("memory limit must be greater than 0, got %d", memoryMB)
	}

	c.Cache.MaxMemoryMB = memoryMB
	return nil
}

// SetCacheTTL sets the SCIP cache TTL
func (c *Config) SetCacheTTL(ttl time.Duration) error {
	if c.Cache == nil {
		c.Cache = GetDefaultSCIPConfig()
	}

	if ttl < time.Minute {
		return fmt.Errorf("TTL must be at least 1 minute, got %v", ttl)
	}

	c.Cache.TTL = ttl
	return nil
}

// SetCacheLanguages sets the languages for SCIP cache
func (c *Config) SetCacheLanguages(languages []string) error {
	if c.Cache == nil {
		c.Cache = GetDefaultSCIPConfig()
	}

	// Validate languages
	supportedLanguages := map[string]bool{
		"go":         true,
		"python":     true,
		"javascript": true,
		"typescript": true,
		"java":       true,
	}

	for _, lang := range languages {
		if !supportedLanguages[lang] {
			return fmt.Errorf("unsupported language for caching: %s", lang)
		}
	}

	c.Cache.Languages = languages
	return nil
}
