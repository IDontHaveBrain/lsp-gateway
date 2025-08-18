package configloader

import (
	"os"
	"os/exec"
	"runtime"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/javautil"
	"lsp-gateway/src/internal/project"
)

func LoadOrAuto(configPath string) *config.Config {
	if configPath != "" {
		if loaded, err := config.LoadConfig(configPath); err == nil {
			common.CLILogger.Debug("Loaded config from path: %s (cache=%v)", configPath, loaded.Cache != nil)
			return loaded
		}
	}

	defaultPath := config.GetDefaultConfigPath()
	if common.FileExists(defaultPath) {
		if loaded, err := config.LoadConfig(defaultPath); err == nil {
			common.CLILogger.Debug("Loaded config from default path: %s (cache=%v)", defaultPath, loaded.Cache != nil)
			return loaded
		}
	}

	wd, _ := os.Getwd()
	auto := config.GenerateAutoConfig(wd, project.GetAvailableLanguages)
	if auto != nil && len(auto.Servers) > 0 {
		common.CLILogger.Debug("Using auto-generated config (cache=%v, servers=%d)", auto.Cache != nil, len(auto.Servers))
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
				} else {
					if javautil.ConfigureJavaFromLocalDownloads(homeDir, javaServer) {
						common.CLILogger.Info("Auto-configured Java using local JDK + JDTLS under ~/.lsp-gateway")
					}
				}

				// Rust auto-detection: if rust-analyzer not in PATH but rustup nightly has it,
				// configure to use 'rustup run nightly rust-analyzer'
				if rustServer, exists := cfg.Servers["rust"]; exists {
					if rustServer.Command == "rust-analyzer" {
						if _, err := exec.LookPath("rust-analyzer"); err != nil {
							if _, err := exec.LookPath("rustup"); err == nil {
								// Check if nightly rust-analyzer exists
								cmd := exec.Command("rustup", "which", "--toolchain", "nightly", "rust-analyzer")
								if out, err := cmd.CombinedOutput(); err == nil && len(out) > 0 {
									common.CLILogger.Info("Auto-configured Rust to use 'rustup run nightly rust-analyzer'")
									rustServer.Command = "rustup"
									rustServer.Args = []string{"run", "nightly", "rust-analyzer"}
								}
							}
						}
					}
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
		jdtlsPath = common.GetLSPToolPath("java", "jdtls.bat")
	} else {
		jdtlsPath = common.GetLSPToolPath("java", "jdtls")
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

//
