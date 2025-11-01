package config

import (
	"os"
	"path/filepath"

	"lsp-gateway/src/internal/constants"
)

func getTestDefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	cacheDir := filepath.Join(home, constants.DefaultLSPToolsDir, constants.DefaultCacheDirName)

	return &Config{
		Servers: getDefaultServerConfigs(),
		Cache: &CacheConfig{
			Enabled:            true,
			StoragePath:        cacheDir,
			MaxMemoryMB:        constants.TestCacheMemoryMB,
			TTLHours:           constants.TestCacheTTLHours,
			Languages:          []string{"*"},
			BackgroundIndex:    constants.TestBackgroundIndexing,
			HealthCheckMinutes: constants.TestHealthCheckMinutes,
		},
		MCP: &MCPConfig{},
	}
}

func NewTestConfig(opts ...TestConfigOption) *Config {
	cfg := getTestDefaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

type TestConfigOption func(*Config)

func WithCacheMemory(mb int) TestConfigOption {
	return func(c *Config) {
		if c.Cache != nil {
			c.Cache.MaxMemoryMB = mb
		}
	}
}

func WithCacheTTL(hours int) TestConfigOption {
	return func(c *Config) {
		if c.Cache != nil {
			c.Cache.TTLHours = hours
		}
	}
}

func WithCachePath(path string) TestConfigOption {
	return func(c *Config) {
		if c.Cache != nil {
			c.Cache.StoragePath = path
		}
	}
}

func WithLanguages(languages ...string) TestConfigOption {
	return func(c *Config) {
		if c.Cache != nil {
			c.Cache.Languages = languages
		}
		filteredServers := make(map[string]*ServerConfig)
		for _, lang := range languages {
			if srv, exists := c.Servers[lang]; exists {
				filteredServers[lang] = srv
			}
		}
		if len(filteredServers) > 0 {
			c.Servers = filteredServers
		}
	}
}

func WithBackgroundIndexing(enabled bool) TestConfigOption {
	return func(c *Config) {
		if c.Cache != nil {
			c.Cache.BackgroundIndex = enabled
		}
	}
}

func WithDiskCache(enabled bool) TestConfigOption {
	return func(c *Config) {
		if c.Cache != nil {
			c.Cache.DiskCache = enabled
		}
	}
}

func WithEvictionPolicy(policy string) TestConfigOption {
	return func(c *Config) {
		if c.Cache != nil {
			c.Cache.EvictionPolicy = policy
		}
	}
}

func WithHealthCheckInterval(minutes int) TestConfigOption {
	return func(c *Config) {
		if c.Cache != nil {
			c.Cache.HealthCheckMinutes = minutes
		}
	}
}

func WithCacheEnabled(enabled bool) TestConfigOption {
	return func(c *Config) {
		if c.Cache != nil {
			c.Cache.Enabled = enabled
		}
	}
}

func WithServerConfig(language string, command string, args []string) TestConfigOption {
	return func(c *Config) {
		if c.Servers == nil {
			c.Servers = make(map[string]*ServerConfig)
		}
		c.Servers[language] = &ServerConfig{
			Command: command,
			Args:    args,
		}
	}
}

func DefaultTestConfig(tempDir string) *Config {
	return NewTestConfig(
		WithCachePath(filepath.Join(tempDir, "test-cache")),
	)
}

func MemoryOnlyTestConfig(tempDir string) *Config {
	return NewTestConfig(
		WithCachePath(filepath.Join(tempDir, "test-cache")),
		WithDiskCache(false),
	)
}

func MultiLangTestConfig(tempDir string, languages ...string) *Config {
	return NewTestConfig(
		WithCachePath(filepath.Join(tempDir, "test-cache")),
		WithLanguages(languages...),
	)
}

func LargeTestConfig(tempDir string) *Config {
	return NewTestConfig(
		WithCachePath(filepath.Join(tempDir, "test-cache")),
		WithCacheMemory(128),
		WithCacheTTL(24),
	)
}

func BasicGoTestConfig(tempDir string) *Config {
	return MultiLangTestConfig(tempDir, "go")
}
