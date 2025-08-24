package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
)

// ConfigUpdater handles updating configuration files with installed language servers
type ConfigUpdater struct {
	manager *LSPInstallManager
}

// NewConfigUpdater creates a new config updater
func NewConfigUpdater(manager *LSPInstallManager) *ConfigUpdater {
	return &ConfigUpdater{
		manager: manager,
	}
}

// UpdateConfigWithInstalledServers updates a configuration with paths to installed servers
func (u *ConfigUpdater) UpdateConfigWithInstalledServers(cfg *config.Config) (*config.Config, error) {
	if cfg == nil {
		cfg = config.GetDefaultConfig()
	}

	// Create a copy of the config
	updatedConfig := &config.Config{
		Servers: make(map[string]*config.ServerConfig),
	}

	// Copy existing config
	for language, serverConfig := range cfg.Servers {
		updatedConfig.Servers[language] = &config.ServerConfig{
			Command:               serverConfig.Command,
			Args:                  append([]string{}, serverConfig.Args...), // Copy slice
			WorkingDir:            serverConfig.WorkingDir,
			InitializationOptions: serverConfig.InitializationOptions,
		}
	}

	// Update with installed servers
	status := u.manager.GetStatus()
	updatedCount := 0

	for language, installStatus := range status {
		if !installStatus.Installed {
			continue
		}

		installer, err := u.manager.GetInstaller(language)
		if err != nil {
			common.CLILogger.Warn("Could not get installer for %s: %v", language, err)
			continue
		}

		// Get the server config from the installer (which may have custom paths)
		installerConfig := installer.GetServerConfig()
		if installerConfig == nil {
			continue
		}

		// Resolve to installed binary path when available (handles kotlin/java/csharp layouts)
		if root := common.GetLSPToolRoot(language); root != "" {
			if resolved := common.FirstExistingExecutable(root, []string{installerConfig.Command}); resolved != "" {
				installerConfig = &config.ServerConfig{
					Command:               resolved,
					Args:                  append([]string{}, installerConfig.Args...),
					WorkingDir:            installerConfig.WorkingDir,
					InitializationOptions: installerConfig.InitializationOptions,
				}
			}
		}

		// Python: prefer basedpyright (via uvx if available), then pyright, then pylsp/jedi
		if language == "python" {
			best := chooseBestPythonConfig()
			if best != nil {
				installerConfig = best
			}
		}

		// Update the config if the installed version has different settings
		if existingConfig, exists := updatedConfig.Servers[language]; exists {
			// Update command and args if they differ
			changed := false
			if installerConfig.Command != existingConfig.Command {
				common.CLILogger.Info("Updating %s command path: %s -> %s",
					language, existingConfig.Command, installerConfig.Command)
				existingConfig.Command = installerConfig.Command
				changed = true
			}
			if !reflect.DeepEqual(installerConfig.Args, existingConfig.Args) {
				common.CLILogger.Info("Updating %s args: %v -> %v",
					language, existingConfig.Args, installerConfig.Args)
				existingConfig.Args = append([]string{}, installerConfig.Args...)
				changed = true
			}
			if changed {
				updatedCount++
			}
		} else {
			// Add new language config
			updatedConfig.Servers[language] = &config.ServerConfig{
				Command:               installerConfig.Command,
				Args:                  append([]string{}, installerConfig.Args...),
				WorkingDir:            installerConfig.WorkingDir,
				InitializationOptions: installerConfig.InitializationOptions,
			}
			common.CLILogger.Info("Added %s configuration", language)
			updatedCount++
		}
	}

	if updatedCount > 0 {
		common.CLILogger.Info("Updated configuration for %d language servers", updatedCount)
	}

	return updatedConfig, nil
}

// SaveUpdatedConfig saves an updated configuration to file
func (u *ConfigUpdater) SaveUpdatedConfig(cfg *config.Config, configPath string) error {
	if configPath == "" {
		configPath = config.GetDefaultConfigPath()
	}

	// Create backup of existing config if it exists
	if _, err := os.Stat(configPath); err == nil {
		backupPath := configPath + ".backup"
		if err := copyFile(configPath, backupPath); err != nil {
			common.CLILogger.Warn("Failed to create config backup: %v", err)
		} else {
			common.CLILogger.Info("Created config backup at %s", backupPath)
		}
	}

	// Save updated config
	if err := config.SaveConfig(cfg, configPath); err != nil {
		return fmt.Errorf("failed to save updated config: %w", err)
	}

	common.CLILogger.Info("Updated configuration saved to %s", configPath)
	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	// Create destination directory if needed
	if err := os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return err
	}

	// Read source file
	data, err := common.SafeReadFileUnder(filepath.Dir(src), src)
	if err != nil {
		return err
	}

	// Write to destination
	dstResolved, err := common.ResolveUnder(filepath.Dir(dst), dst)
	if err != nil {
		return err
	}
	return os.WriteFile(dstResolved, data, 0600)
}

// chooseBestPythonConfig selects the preferred Python LSP configuration based on availability
// Priority: uvx basedpyright -> basedpyright-langserver -> pyright-langserver -> pylsp -> jedi-language-server
func chooseBestPythonConfig() *config.ServerConfig {
	// uvx/uv available -> prefer uvx basedpyright
	if _, err := execLookPathSafe("uvx"); err == nil {
		return &config.ServerConfig{Command: "uvx", Args: []string{"basedpyright-langserver", "--stdio"}}
	}
	// If only uv is present, fall back later to direct servers
	// direct basedpyright
	if _, err := execLookPathSafe("basedpyright-langserver"); err == nil {
		return &config.ServerConfig{Command: "basedpyright-langserver", Args: []string{"--stdio"}}
	}
	// pyright
	if _, err := execLookPathSafe("pyright-langserver"); err == nil {
		return &config.ServerConfig{Command: "pyright-langserver", Args: []string{"--stdio"}}
	}
	// pylsp
	if _, err := execLookPathSafe("pylsp"); err == nil {
		return &config.ServerConfig{Command: "pylsp", Args: []string{}}
	}
	// jedi
	if _, err := execLookPathSafe("jedi-language-server"); err == nil {
		return &config.ServerConfig{Command: "jedi-language-server", Args: []string{}}
	}
	return nil
}

// execLookPathSafe wraps exec.LookPath without adding a new import here
func execLookPathSafe(name string) (string, error) {
	return exec.LookPath(name)
}
