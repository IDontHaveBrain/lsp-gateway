package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/project"
)

// LoadConfigWithFallback loads configuration with automatic fallback (for servers)
func LoadConfigWithFallback(configPath string) *config.Config {
	return loadConfigInternal(configPath, false)
}

// LoadConfigForCLI loads configuration for CLI commands with background indexing disabled
// This prevents duplicate indexing when CLI commands create temporary LSP managers
func LoadConfigForCLI(configPath string) *config.Config {
	return loadConfigInternal(configPath, true)
}

// loadConfigInternal is the shared implementation for config loading
func loadConfigInternal(configPath string, disableBackgroundIndex bool) *config.Config {
	var cfg *config.Config

	if configPath != "" {
		loadedConfig, err := config.LoadConfig(configPath)
		if err != nil {
			common.CLILogger.Warn("Failed to load config from %s, using auto-config: %v", configPath, err)
			// Use auto-config which sets project-specific cache path
			wd, _ := os.Getwd()
			cfg = config.GenerateAutoConfig(wd, project.GetAvailableLanguages)
			if cfg == nil || len(cfg.Servers) == 0 {
				cfg = config.GetDefaultConfig()
			}
		} else {
			cfg = loadedConfig
		}
	} else {
		// Try to load from default config file if it exists
		defaultConfigPath := config.GetDefaultConfigPath()
		if _, err := os.Stat(defaultConfigPath); err == nil {
			loadedConfig, err := config.LoadConfig(defaultConfigPath)
			if err != nil {
				common.CLILogger.Warn("Failed to load default config from %s, using auto-config: %v", defaultConfigPath, err)
				// Use auto-config which sets project-specific cache path
				wd, _ := os.Getwd()
				cfg = config.GenerateAutoConfig(wd, project.GetAvailableLanguages)
				if cfg == nil || len(cfg.Servers) == 0 {
					cfg = config.GetDefaultConfig()
				}
			} else {
				cfg = loadedConfig
			}
		} else {
			// Use auto-config which sets project-specific cache path
			wd, _ := os.Getwd()
			cfg = config.GenerateAutoConfig(wd, project.GetAvailableLanguages)
			if cfg == nil || len(cfg.Servers) == 0 {
				cfg = config.GetDefaultConfig()
			}
		}
	}

	// Auto-detect installed servers if using default configurations
	autoDetectInstalledServers(cfg)

	// Disable background indexing for CLI commands to prevent duplicate work
	if disableBackgroundIndex && cfg.Cache != nil {
		cfg.Cache.BackgroundIndex = false
	}

	return cfg
}

// autoDetectInstalledServers automatically detects and updates server paths for installed servers
func autoDetectInstalledServers(cfg *config.Config) {
	if cfg == nil || cfg.Servers == nil {
		return
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return // Skip auto-detection if can't get home directory
	}

	// Check Java server specifically
	if javaServer, exists := cfg.Servers["java"]; exists {
		// Only auto-detect if using the default "jdtls" command
		if javaServer.Command == "jdtls" {
			// Check if jdtls is available in PATH
			if _, err := exec.LookPath("jdtls"); err != nil {
				// jdtls not found in PATH, check for installed version
				installedJdtlsPath := getInstalledJdtlsPath(homeDir)
				if installedJdtlsPath != "" {
					common.CLILogger.Info("Auto-detected installed jdtls at: %s", installedJdtlsPath)
					javaServer.Command = installedJdtlsPath
				}
			}
		}
	}
}

// getInstalledJdtlsPath returns the path to installed jdtls if it exists
func getInstalledJdtlsPath(homeDir string) string {
	// Standard installation path: ~/.lsp-gateway/tools/java/bin/jdtls
	var jdtlsPath string

	if runtime.GOOS == "windows" {
		jdtlsPath = filepath.Join(homeDir, ".lsp-gateway", "tools", "java", "bin", "jdtls.bat")
	} else {
		jdtlsPath = filepath.Join(homeDir, ".lsp-gateway", "tools", "java", "bin", "jdtls")
	}

	// Check if the file exists and is executable
	if fileInfo, err := os.Stat(jdtlsPath); err == nil {
		// Check if it's executable (on Unix systems)
		if runtime.GOOS != "windows" {
			if fileInfo.Mode()&0111 == 0 {
				return "" // Not executable
			}
		}
		return jdtlsPath
	}

	return "" // Not found
}
