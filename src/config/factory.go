package config

import (
	"os"
	"path/filepath"
	"runtime"

	"lsp-gateway/src/internal/constants"
)

type ConfigBuilder struct {
	config *Config
}

func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		config: getDefaultConfig(),
	}
}

func NewTestConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		config: getTestDefaultConfig(),
	}
}

func getDefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	cacheDir := filepath.Join(home, constants.DefaultLSPToolsDir, constants.DefaultCacheDirName)

	return &Config{
		Servers: getDefaultServerConfigs(),
		Cache: &CacheConfig{
			Enabled:            true,
			StoragePath:        cacheDir,
			MaxMemoryMB:        constants.DefaultCacheMemoryMB,
			TTLHours:           constants.DefaultCacheTTLHours,
			Languages:          []string{"*"},
			BackgroundIndex:    constants.DefaultBackgroundIndexing,
			DiskCache:          constants.DefaultDiskCachePersistence,
			EvictionPolicy:     constants.DefaultEvictionPolicy,
			HealthCheckMinutes: constants.DefaultHealthCheckMinutes,
		},
		MCP: &MCPConfig{},
	}
}

func getTestDefaultConfig() *Config {
	cfg := getDefaultConfig()
	cfg.Cache.MaxMemoryMB = constants.TestCacheMemoryMB
	cfg.Cache.TTLHours = constants.TestCacheTTLHours
	cfg.Cache.BackgroundIndex = constants.TestBackgroundIndexing
	cfg.Cache.HealthCheckMinutes = constants.TestHealthCheckMinutes
	return cfg
}

func getDefaultServerConfigs() map[string]*ServerConfig {
	kotlinCommand := "kotlin-lsp"
	kotlinArgs := []string{}

	if runtime.GOOS == "windows" {
		kotlinCommand = "kotlin-language-server"
		kotlinArgs = []string{}
	}

	return map[string]*ServerConfig{
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
	}
}

func (b *ConfigBuilder) WithCacheMemory(mb int) *ConfigBuilder {
	b.config.Cache.MaxMemoryMB = mb
	return b
}

func (b *ConfigBuilder) WithCacheTTL(hours int) *ConfigBuilder {
	b.config.Cache.TTLHours = hours
	return b
}

func (b *ConfigBuilder) WithCachePath(path string) *ConfigBuilder {
	b.config.Cache.StoragePath = path
	return b
}

func (b *ConfigBuilder) WithLanguages(languages ...string) *ConfigBuilder {
	b.config.Cache.Languages = languages
	return b
}

func (b *ConfigBuilder) WithBackgroundIndexing(enabled bool) *ConfigBuilder {
	b.config.Cache.BackgroundIndex = enabled
	return b
}

func (b *ConfigBuilder) WithDiskCache(enabled bool) *ConfigBuilder {
	b.config.Cache.DiskCache = enabled
	return b
}

func (b *ConfigBuilder) WithEvictionPolicy(policy string) *ConfigBuilder {
	b.config.Cache.EvictionPolicy = policy
	return b
}

func (b *ConfigBuilder) WithHealthCheckInterval(minutes int) *ConfigBuilder {
	b.config.Cache.HealthCheckMinutes = minutes
	return b
}

func (b *ConfigBuilder) WithCacheEnabled(enabled bool) *ConfigBuilder {
	b.config.Cache.Enabled = enabled
	return b
}

func (b *ConfigBuilder) WithServerConfig(language string, command string, args []string) *ConfigBuilder {
	if b.config.Servers == nil {
		b.config.Servers = make(map[string]*ServerConfig)
	}
	b.config.Servers[language] = &ServerConfig{
		Command: command,
		Args:    args,
	}
	return b
}

func (b *ConfigBuilder) Build() (*Config, error) {
	if err := validateConfig(b.config); err != nil {
		return nil, err
	}
	return b.config, nil
}

func (b *ConfigBuilder) MustBuild() *Config {
	cfg, err := b.Build()
	if err != nil {
		panic(err)
	}
	return cfg
}

func DefaultTestConfig(tempDir string) *Config {
	cacheDir := filepath.Join(tempDir, "test-cache")
	return NewTestConfigBuilder().
		WithCachePath(cacheDir).
		MustBuild()
}

func MemoryOnlyTestConfig(tempDir string) *Config {
	cacheDir := filepath.Join(tempDir, "test-cache")
	return NewTestConfigBuilder().
		WithCachePath(cacheDir).
		WithDiskCache(false).
		MustBuild()
}

func MultiLangTestConfig(tempDir string, languages ...string) *Config {
	cacheDir := filepath.Join(tempDir, "test-cache")
	return NewTestConfigBuilder().
		WithCachePath(cacheDir).
		WithLanguages(languages...).
		MustBuild()
}

func LargeTestConfig(tempDir string) *Config {
	cacheDir := filepath.Join(tempDir, "test-cache")
	return NewTestConfigBuilder().
		WithCachePath(cacheDir).
		WithCacheMemory(128).
		WithCacheTTL(24).
		MustBuild()
}

func BasicGoTestConfig(tempDir string) *Config {
	return MultiLangTestConfig(tempDir, "go")
}
