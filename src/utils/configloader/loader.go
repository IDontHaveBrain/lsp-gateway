package configloader

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/project"
)

func LoadOrAuto(configPath string) *config.Config {
	if configPath != "" {
		if loaded, err := config.LoadConfig(configPath); err == nil {
			return loaded
		}
	}

	defaultPath := config.GetDefaultConfigPath()
	if _, err := os.Stat(defaultPath); err == nil {
		if loaded, err := config.LoadConfig(defaultPath); err == nil {
			return loaded
		}
	}

	wd, _ := os.Getwd()
	auto := config.GenerateAutoConfig(wd, project.GetAvailableLanguages)
	if auto != nil && len(auto.Servers) > 0 {
		return auto
	}

	common.CLILogger.Warn("Falling back to default config")
	return config.GetDefaultConfig()
}

// LoadForServer loads configuration for server operations with auto-detection of installed servers
func LoadForServer(configPath string) *config.Config {
	cfg := LoadOrAuto(configPath)

	// Auto-detect installed servers if using default configurations
	autoDetectInstalledServers(cfg)

	return cfg
}

// LoadForCLI loads configuration for CLI commands with background indexing disabled
// This prevents duplicate indexing when CLI commands create temporary LSP managers
func LoadForCLI(configPath string) *config.Config {
	cfg := LoadOrAuto(configPath)

	// Auto-detect installed servers if using default configurations
	autoDetectInstalledServers(cfg)

	// Disable background indexing for CLI commands to prevent duplicate work
	if cfg.Cache != nil {
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
