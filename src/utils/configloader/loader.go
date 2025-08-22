package configloader

import (
    "os"
    "os/exec"
    "runtime"
    "path/filepath"

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

	// Check C# server specifically
	if csharpServer, exists := cfg.Servers["csharp"]; exists {
		// Only auto-detect if using the default "omnisharp" command
		if csharpServer.Command == "omnisharp" {
			// Check if omnisharp is available in PATH
			if _, err := exec.LookPath("omnisharp"); err != nil {
				// Try OmniSharp with capital O
				if _, err := exec.LookPath("OmniSharp"); err == nil {
					common.CLILogger.Info("Auto-detected OmniSharp (capital O) in PATH")
					csharpServer.Command = "OmniSharp"
				} else {
					// Check for installed version
					installedOmniSharpPath := getInstalledOmniSharpPath(homeDir)
					if installedOmniSharpPath != "" {
						common.CLILogger.Info("Auto-detected installed OmniSharp at: %s", installedOmniSharpPath)
						csharpServer.Command = installedOmniSharpPath
					}
				}
			}
		}
	}

	// Check Kotlin server specifically
	if kotlinServer, exists := cfg.Servers["kotlin"]; exists {
		// Only auto-detect if using the default "kotlin-lsp" command
		if kotlinServer.Command == "kotlin-lsp" {
			// Check if kotlin-lsp is available in PATH
			if _, err := exec.LookPath("kotlin-lsp"); err != nil {
				// Check for installed version
				installedKotlinLSPPath := getInstalledKotlinLSPPath(homeDir)
				if installedKotlinLSPPath != "" {
					common.CLILogger.Info("Auto-detected installed kotlin-lsp at: %s", installedKotlinLSPPath)
					kotlinServer.Command = installedKotlinLSPPath
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

// getInstalledOmniSharpPath returns the path to installed OmniSharp if it exists
func getInstalledOmniSharpPath(homeDir string) string {
	// Check both "omnisharp" and "OmniSharp" binary names
	candidates := []string{"omnisharp", "OmniSharp"}

	for _, name := range candidates {
		var omnisharpPath string
		if runtime.GOOS == "windows" {
			omnisharpPath = common.GetLSPToolPath("csharp", name+".exe")
		} else {
			omnisharpPath = common.GetLSPToolPath("csharp", name)
		}

		// Check if the file exists and is executable
		if fileInfo, err := os.Stat(omnisharpPath); err == nil {
			// Check if it's executable (on Unix systems)
			if runtime.GOOS != "windows" {
				if fileInfo.Mode()&0111 == 0 {
					continue // Not executable, try next candidate
				}
			}
			return omnisharpPath
		}
	}

	return "" // Not found
}

// getInstalledKotlinLSPPath returns the path to installed kotlin-lsp if it exists
func getInstalledKotlinLSPPath(homeDir string) string {
    // Preferred installation path: ~/.lsp-gateway/tools/kotlin/bin/kotlin-lsp(.cmd/.bat)
    // Also support legacy layout without bin and .sh script
    candidates := []string{}
    if runtime.GOOS == "windows" {
        // Prefer native .exe if available (more reliable for stdio on Windows)
        candidates = append(candidates,
            common.GetLSPToolPath("kotlin", "kotlin-lsp.exe"),
        )
        candidates = append(candidates,
            common.GetLSPToolPath("kotlin", "kotlin-lsp.cmd"),
            common.GetLSPToolPath("kotlin", "kotlin-lsp.bat"),
        )
        // Legacy (no bin)
        root := common.GetLSPToolRoot("kotlin")
        candidates = append(candidates,
            filepath.Join(root, "kotlin-lsp.exe"),
            filepath.Join(root, "kotlin-lsp.cmd"),
            filepath.Join(root, "kotlin-lsp.bat"),
        )
    } else {
        candidates = append(candidates,
            common.GetLSPToolPath("kotlin", "kotlin-lsp"),
        )
        // Legacy (no bin) + direct .sh
        root := common.GetLSPToolRoot("kotlin")
        candidates = append(candidates,
            filepath.Join(root, "kotlin-lsp"),
            filepath.Join(root, "kotlin-lsp.sh"),
        )
    }

    for _, p := range candidates {
        if fileInfo, err := os.Stat(p); err == nil {
            if runtime.GOOS != "windows" {
                if fileInfo.Mode()&0111 == 0 {
                    continue
                }
            }
            return p
        }
    }

    return ""
}

//
