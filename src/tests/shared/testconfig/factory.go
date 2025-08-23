package testconfig

import (
	"runtime"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/registry"
)

// NewGoServerConfig returns a standard Go server configuration for testing
func NewGoServerConfig() *config.ServerConfig {
	return &config.ServerConfig{
		Command: "gopls",
		Args:    []string{"serve"},
	}
}

// NewPythonServerConfig returns a standard Python server configuration for testing
func NewPythonServerConfig() *config.ServerConfig {
	return &config.ServerConfig{
		Command: "jedi-language-server",
		Args:    []string{},
	}
}

// NewTypeScriptServerConfig returns a standard TypeScript server configuration for testing
func NewTypeScriptServerConfig() *config.ServerConfig {
	return &config.ServerConfig{
		Command: "typescript-language-server",
		Args:    []string{"--stdio"},
	}
}

// NewJavaScriptServerConfig returns a standard JavaScript server configuration for testing
func NewJavaScriptServerConfig() *config.ServerConfig {
	return &config.ServerConfig{
		Command: "typescript-language-server",
		Args:    []string{"--stdio"},
	}
}

// NewJavaServerConfig returns a standard Java server configuration for testing
func NewJavaServerConfig() *config.ServerConfig {
	return &config.ServerConfig{
		Command: "jdtls",
		Args:    []string{},
	}
}

// NewRustServerConfig returns a standard Rust server configuration for testing
func NewRustServerConfig() *config.ServerConfig {
	return &config.ServerConfig{
		Command: "rust-analyzer",
		Args:    []string{},
	}
}

// NewKotlinServerConfig returns a standard Kotlin server configuration for testing
func NewKotlinServerConfig() *config.ServerConfig {
	// Use platform-specific command, trying to resolve installed path first
	command := "kotlin-lsp"
	candidates := []string{"kotlin-lsp"}

	if runtime.GOOS == "windows" {
		command = "kotlin-language-server"
		candidates = []string{"kotlin-language-server"}
	}

	// Try to resolve to installed binary path
	if root := common.GetLSPToolRoot("kotlin"); root != "" {
		if resolved := common.FirstExistingExecutable(root, candidates); resolved != "" {
			command = resolved
		}
	}

	return &config.ServerConfig{
		Command: command,
		Args:    []string{},
	}
}

// NewMultiLangConfig creates a configuration with the specified languages
func NewMultiLangConfig(languages []string) *config.Config {
	servers := make(map[string]*config.ServerConfig)

	for _, lang := range languages {
		switch lang {
		case "go":
			servers["go"] = NewGoServerConfig()
		case "python":
			servers["python"] = NewPythonServerConfig()
		case "typescript":
			servers["typescript"] = NewTypeScriptServerConfig()
		case "javascript":
			servers["javascript"] = NewJavaScriptServerConfig()
		case "java":
			servers["java"] = NewJavaServerConfig()
		case "rust":
			servers["rust"] = NewRustServerConfig()
		case "kotlin":
			servers["kotlin"] = NewKotlinServerConfig()
		}
	}

	return &config.Config{
		Servers: servers,
	}
}

// NewTestConfig creates a configuration with all supported languages
func NewTestConfig() *config.Config {
	return NewMultiLangConfig(registry.GetLanguageNames())
}

// NewTestConfigWithCache creates a test configuration with cache enabled
func NewTestConfigWithCache() *config.Config {
	cfg := NewTestConfig()
	cfg.Cache = &config.CacheConfig{
		Enabled:     true,
		MaxMemoryMB: 256,
		TTLHours:    1,
		StoragePath: "/tmp/test-cache",
		Languages:   registry.GetLanguageNames(),
		DiskCache:   false,
	}
	return cfg
}

// NewBasicGoConfig creates a basic configuration with only Go language
func NewBasicGoConfig() *config.Config {
	return &config.Config{
		Servers: map[string]*config.ServerConfig{
			"go": NewGoServerConfig(),
		},
	}
}

// NewBasicGoConfigWithCache creates a basic Go configuration with cache enabled
func NewBasicGoConfigWithCache() *config.Config {
	cfg := NewBasicGoConfig()
	cfg.Cache = &config.CacheConfig{
		Enabled:     true,
		MaxMemoryMB: 64,
		TTLHours:    1,
		StoragePath: "/tmp/test-cache",
		Languages:   []string{"go"},
		DiskCache:   false,
	}
	return cfg
}

// NewCacheConfig creates a standard test cache configuration
func NewCacheConfig(storagePath string) *config.CacheConfig {
	return &config.CacheConfig{
		Enabled:     true,
		MaxMemoryMB: 128,
		TTLHours:    1,
		StoragePath: storagePath,
		Languages:   registry.GetLanguageNames(),
		DiskCache:   false,
	}
}

// NewCustomCacheConfig creates a cache configuration with custom parameters
func NewCustomCacheConfig(enabled bool, memoryMB int, ttlHours int, storagePath string) *config.CacheConfig {
	return &config.CacheConfig{
		Enabled:     enabled,
		MaxMemoryMB: memoryMB,
		TTLHours:    ttlHours,
		StoragePath: storagePath,
		Languages:   registry.GetLanguageNames(),
		DiskCache:   false,
	}
}

// NewConfigWithCustomCache creates a configuration with a custom cache config
func NewConfigWithCustomCache(cacheConfig *config.CacheConfig) *config.Config {
	cfg := NewTestConfig()
	cfg.Cache = cacheConfig
	return cfg
}
