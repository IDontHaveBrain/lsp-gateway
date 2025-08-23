package config

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/registry"
)

// Config contains LSP server configuration, unified cache settings, and MCP configuration
type Config struct {
	Servers map[string]*ServerConfig `yaml:"servers"`
	Cache   *CacheConfig             `yaml:"cache"`
	MCP     *MCPConfig               `yaml:"mcp,omitempty"`
}

// MCPConfig contains MCP server specific configuration
type MCPConfig struct {
	// Currently no MCP-specific configuration fields
	// Enhanced mode is always used
}

// ServerConfig contains configuration for a single LSP server
type ServerConfig struct {
	Command               string      `yaml:"command"`
	Args                  []string    `yaml:"args"`
	WorkingDir            string      `yaml:"working_dir,omitempty"`
	InitializationOptions interface{} `yaml:"initialization_options,omitempty"`
}

// CacheConfig contains unified cache configuration settings with simple units
type CacheConfig struct {
	Enabled            bool     `yaml:"enabled"`              // Cache enabled by default
	StoragePath        string   `yaml:"storage_path"`         // Cache storage directory
	MaxMemoryMB        int      `yaml:"max_memory_mb"`        // Memory limit in MB (simple units)
	TTLHours           int      `yaml:"ttl_hours"`            // Cache TTL in hours (simple units)
	Languages          []string `yaml:"languages"`            // Supported languages for caching
	BackgroundIndex    bool     `yaml:"background_index"`     // Enable background indexing
	HealthCheckMinutes int      `yaml:"health_check_minutes"` // Health check interval in minutes (simple units)
	EvictionPolicy     string   `yaml:"eviction_policy"`      // Cache eviction policy (lru, simple)
	DiskCache          bool     `yaml:"disk_cache"`           // Enable disk persistence
}

// expandServerPaths expands ~ paths in server configurations
func expandServerPaths(config *Config) error {
	for serverName, serverConfig := range config.Servers {
		if serverConfig == nil {
			continue
		}

		expandedCommand, err := common.ExpandPath(serverConfig.Command)
		if err != nil {
			return fmt.Errorf("failed to expand path for server %s command '%s': %w",
				serverName, serverConfig.Command, err)
		}
		serverConfig.Command = expandedCommand

		if serverConfig.WorkingDir != "" {
			expandedWorkingDir, err := common.ExpandPath(serverConfig.WorkingDir)
			if err != nil {
				return fmt.Errorf("failed to expand working directory for server %s: %w",
					serverName, err)
			}
			serverConfig.WorkingDir = expandedWorkingDir
		}
	}

	// Also expand cache storage path if present
	if config.Cache != nil && config.Cache.StoragePath != "" {
		expandedStoragePath, err := common.ExpandPath(config.Cache.StoragePath)
		if err != nil {
			return fmt.Errorf("failed to expand cache storage path '%s': %w",
				config.Cache.StoragePath, err)
		}
		config.Cache.StoragePath = expandedStoragePath
	}

	return nil
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := common.SafeReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Expand ~ paths in server configurations
	if err := expandServerPaths(&config); err != nil {
		return nil, fmt.Errorf("failed to expand paths in config: %w", err)
	}

	// Validate config
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Ensure cache configuration exists with defaults if missing
	if config.Cache == nil {
		config.Cache = GetDefaultCacheConfig()
		common.CLILogger.Debug("Added default cache configuration to loaded config")
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

	// Validate cache configuration if present
	if config.Cache != nil {
		if err := validateCacheConfig(config.Cache); err != nil {
			return fmt.Errorf("invalid cache configuration: %w", err)
		}
	}

	// MCP config validation (currently no fields to validate)

	return nil
}

// GetAllSupportedLanguages returns all supported languages
func GetAllSupportedLanguages() []string {
	return registry.GetLanguageNames()
}

// GetSupportedLanguagesMap returns supported languages as a map for validation
func GetSupportedLanguagesMap() map[string]bool {
	languages := registry.GetLanguageNames()
	langMap := make(map[string]bool, len(languages))
	for _, lang := range languages {
		langMap[lang] = true
	}
	return langMap
}

// expandLanguageWildcard expands "*" wildcard to all supported languages
func expandLanguageWildcard(languages []string) []string {
	if len(languages) == 1 && languages[0] == "*" {
		return GetAllSupportedLanguages()
	}
	return languages
}

// validateCacheConfig validates unified cache configuration with simple units
func validateCacheConfig(cache *CacheConfig) error {
	// Expand wildcard before validation
	cache.Languages = expandLanguageWildcard(cache.Languages)

	// Memory validation
	if cache.MaxMemoryMB <= 0 {
		return fmt.Errorf("max_memory_mb must be greater than 0, got %d", cache.MaxMemoryMB)
	}

	// Reasonable memory limit check (warn if > 50% of system memory)
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	systemMemoryMB := int(memStats.Sys / 1024 / 1024)
	if cache.MaxMemoryMB > systemMemoryMB/2 {
		common.CLILogger.Warn("Cache memory limit (%dMB) is more than 50%% of system memory (%dMB)",
			cache.MaxMemoryMB, systemMemoryMB)
	}

	// Storage path validation
	if cache.StoragePath == "" {
		return fmt.Errorf("storage_path cannot be empty")
	}

	// TTL validation (in hours)
	if cache.TTLHours < 1 {
		return fmt.Errorf("ttl_hours must be at least 1 hour, got %d", cache.TTLHours)
	}

	// Health check interval validation (in minutes)
	if cache.HealthCheckMinutes < 1 {
		return fmt.Errorf("health_check_minutes must be at least 1 minute, got %d", cache.HealthCheckMinutes)
	}

	// Eviction policy validation
	validPolicies := map[string]bool{
		"lru":    true,
		"simple": true,
	}
	if cache.EvictionPolicy != "" && !validPolicies[cache.EvictionPolicy] {
		return fmt.Errorf("invalid eviction_policy: %s (must be 'lru' or 'simple')", cache.EvictionPolicy)
	}

	// Language validation - ensure only supported languages (after wildcard expansion)
	supportedLanguages := GetSupportedLanguagesMap()

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
	// Platform-specific Kotlin LSP configuration
	kotlinCommand := "kotlin-lsp"
	kotlinArgs := []string{} // JetBrains kotlin-lsp defaults to socket mode on port 9999
	
	if runtime.GOOS == "windows" {
		kotlinCommand = "kotlin-language-server" // fwcd version
		kotlinArgs = []string{} // fwcd defaults to stdio mode
	}

	return &Config{
		Servers: map[string]*ServerConfig{
			"go": {
				Command: "gopls",
				Args:    []string{"serve"},
			},
			"python": {
				Command: "basedpyright-langserver",
				Args:    []string{"--stdio"},
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
			"rust": {
				Command: "rust-analyzer",
				Args:    []string{},
			},
			"csharp": {
				Command: "omnisharp",
				Args:    []string{"-lsp"},
			},
			"kotlin": {
				Command: kotlinCommand,
				Args:    kotlinArgs,
			},
		},
		// Cache is enabled by default with standard settings
		Cache: GetDefaultCacheConfig(),
		MCP:   &MCPConfig{},
	}
}

// GetDefaultCacheConfig returns a default cache configuration with simple units (enabled by default)
func GetDefaultCacheConfig() *CacheConfig {
	home, _ := os.UserHomeDir()
	return &CacheConfig{
		Enabled:            true, // Enabled by default for improved performance
		StoragePath:        filepath.Join(home, ".lsp-gateway", "scip-cache"),
		MaxMemoryMB:        512,
		TTLHours:           24,            // 24 hours for daily development workflow
		Languages:          []string{"*"}, // Wildcard for all supported languages
		BackgroundIndex:    true,
		HealthCheckMinutes: 5, // 5 minutes
		EvictionPolicy:     "lru",
		DiskCache:          true, // Enable disk persistence by default for index data
	}
}

// GetProjectSpecificCachePath returns a project-specific cache path for multiple instance support
func GetProjectSpecificCachePath(workingDir string) string {
	// Get absolute path to ensure consistency
	absPath, err := filepath.Abs(workingDir)
	if err != nil {
		// Fallback to the working directory if unable to get absolute path
		absPath = workingDir
	}

	// Create a safe directory name from the absolute path
	// Replace path separators and other problematic characters with underscores
	projectName := filepath.Base(absPath)
	if projectName == "." || projectName == "/" {
		projectName = "root"
	}

	// Add a hash of the full path to handle duplicate project names in different locations
	hasher := md5.New()
	hasher.Write([]byte(absPath))
	pathHash := fmt.Sprintf("%x", hasher.Sum(nil))
	if len(pathHash) > 8 {
		pathHash = pathHash[:8] // Use first 8 characters of hash
	}

	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if unable to get home
		return filepath.Join(workingDir, ".lsp-gateway-cache")
	}

	return filepath.Join(home, ".lsp-gateway", "scip-cache", fmt.Sprintf("%s-%s", projectName, pathHash))
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
		Cache:   GetDefaultCacheConfig(),
		MCP:     &MCPConfig{},
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
			common.CLILogger.Debug("Added %s server configuration", language)
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

	workingDir, err := common.ValidateAndGetWorkingDir(workingDir)
	if err != nil {
		common.CLILogger.Error("Failed to get working directory: %v", err)
		return GetDefaultConfig()
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

	common.CLILogger.Debug("Detected languages: %v", languages)
	config := GenerateConfigForLanguages(languages)

	// Set project-specific cache path for multiple instance support
	projectCachePath := GetProjectSpecificCachePath(workingDir)
	config.SetCacheStoragePath(projectCachePath)
	common.CLILogger.Debug("Using project-specific cache path: %s", projectCachePath)

	return config
}

// GenerateAutoConfig is a convenience function that creates a configuration
// with only the languages that have available LSP servers in the working directory
// Usage: config := config.GenerateAutoConfig(".", project.GetAvailableLanguages)
func GenerateAutoConfig(workingDir string, getAvailableLanguages func(string) ([]string, error)) *Config {
	return DetectAndGenerateConfig(workingDir, getAvailableLanguages)
}

// Cache Management Functions

// ensureCache ensures cache config exists with defaults if nil
func (c *Config) ensureCache() {
	if c.Cache == nil {
		c.Cache = GetDefaultCacheConfig()
	}
}

// HasCache returns true if the configuration has cache settings
func (c *Config) HasCache() bool {
	return c.Cache != nil
}

// IsCacheEnabled returns true if cache is enabled
func (c *Config) IsCacheEnabled() bool {
	return c.Cache != nil && c.Cache.Enabled
}

// EnableCache enables cache with default settings if not present
func (c *Config) EnableCache() {
	c.ensureCache()
	c.Cache.Enabled = true
}

// DisableCache disables cache but preserves settings
func (c *Config) DisableCache() {
	if c.Cache != nil {
		c.Cache.Enabled = false
	}
}

// SetCacheStoragePath sets the cache storage path
func (c *Config) SetCacheStoragePath(path string) {
	c.ensureCache()
	c.Cache.StoragePath = path
}

// SetCacheMemoryLimit sets the cache memory limit in MB
func (c *Config) SetCacheMemoryLimit(memoryMB int) error {
	c.ensureCache()

	if memoryMB <= 0 {
		return fmt.Errorf("memory limit must be greater than 0, got %d", memoryMB)
	}

	c.Cache.MaxMemoryMB = memoryMB
	return nil
}

// SetCacheTTLHours sets the cache TTL in hours (simple units)
func (c *Config) SetCacheTTLHours(hours int) error {
	c.ensureCache()

	if hours < 1 {
		return fmt.Errorf("TTL must be at least 1 hour, got %d", hours)
	}

	c.Cache.TTLHours = hours
	return nil
}

// SetCacheLanguages sets the languages for cache
func (c *Config) SetCacheLanguages(languages []string) error {
	c.ensureCache()

	// Validate languages
	supportedLanguages := GetSupportedLanguagesMap()

	for _, lang := range languages {
		if !supportedLanguages[lang] {
			return fmt.Errorf("unsupported language for caching: %s", lang)
		}
	}

	c.Cache.Languages = languages
	return nil
}
