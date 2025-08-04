package installer

import (
	"fmt"
	"os"
	"path/filepath"

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

		// Update the config if the installed version has different settings
		if existingConfig, exists := updatedConfig.Servers[language]; exists {
			// Check if we need to update the command path
			if installerConfig.Command != existingConfig.Command {
				common.CLILogger.Info("Updating %s command path: %s -> %s",
					language, existingConfig.Command, installerConfig.Command)
				existingConfig.Command = installerConfig.Command
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

// GenerateConfigForInstalledServers creates a new config with only installed servers
func (u *ConfigUpdater) GenerateConfigForInstalledServers() (*config.Config, error) {
	cfg := &config.Config{
		Servers: make(map[string]*config.ServerConfig),
	}

	status := u.manager.GetStatus()

	for language, installStatus := range status {
		if !installStatus.Installed {
			continue
		}

		installer, err := u.manager.GetInstaller(language)
		if err != nil {
			continue
		}

		serverConfig := installer.GetServerConfig()
		if serverConfig != nil {
			cfg.Servers[language] = &config.ServerConfig{
				Command:               serverConfig.Command,
				Args:                  append([]string{}, serverConfig.Args...),
				WorkingDir:            serverConfig.WorkingDir,
				InitializationOptions: serverConfig.InitializationOptions,
			}
		}
	}

	return cfg, nil
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

// GetRecommendedConfig returns a configuration that includes both default and installed servers
func (u *ConfigUpdater) GetRecommendedConfig() (*config.Config, error) {
	// Start with default config
	defaultConfig := config.GetDefaultConfig()

	// Update with installed servers
	return u.UpdateConfigWithInstalledServers(defaultConfig)
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	// Create destination directory if needed
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	// Read source file
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	// Write to destination
	return os.WriteFile(dst, data, 0644)
}
