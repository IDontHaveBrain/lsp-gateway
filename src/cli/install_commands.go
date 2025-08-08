package cli

import (
	"context"
	"fmt"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/installer"
)

// ShowInstallStatus displays installation status for all language servers
func ShowInstallStatus() error {
	common.CLILogger.Info("Language Server Installation Status")

	manager := installer.GetDefaultInstallManager()
	status := manager.GetStatus()

	if len(status) == 0 {
		common.CLILogger.Warn("No language installers available")
		return nil
	}

	installedCount := 0
	for language, installStatus := range status {
		statusText := "Not Installed"

		if installStatus.Installed {
			statusText = "Installed"
			installedCount++
		}

		common.CLILogger.Info("%s: %s", language, statusText)

		if installStatus.Version != "" {
			common.CLILogger.Info("   Version: %s", installStatus.Version)
		}

		if installStatus.Error != nil {
			common.CLILogger.Error("   Error: %v", installStatus.Error)
		}

		common.CLILogger.Info("")
	}

	common.CLILogger.Info("Summary: %d/%d language servers installed", installedCount, len(status))

	return nil
}

// InstallAll installs all supported language servers
func InstallAll(installPath, version string, force, offline bool) error {
	common.CLILogger.Info("Installing all language servers...")

	manager := installer.GetDefaultInstallManager()

	options := installer.InstallOptions{
		InstallPath:    installPath,
		Version:        version,
		Force:          force,
		Offline:        offline,
		SkipValidation: false,
		UpdateConfig:   true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	if err := manager.InstallAll(ctx, options); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	common.CLILogger.Info("All language servers installation completed")
	return nil
}

// InstallLanguage installs a specific language server
func InstallLanguage(language, installPath, version string, force, offline bool) error {
	common.CLILogger.Info("Installing %s language server...", language)

	manager := installer.GetDefaultInstallManager()

	options := installer.InstallOptions{
		InstallPath:    installPath,
		Version:        version,
		Force:          force,
		Offline:        offline,
		SkipValidation: false,
		UpdateConfig:   true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	if err := manager.InstallLanguage(ctx, language, options); err != nil {
		return fmt.Errorf("failed to install %s: %w", language, err)
	}

	common.CLILogger.Info("%s language server installation completed", language)
	return nil
}

// UpdateConfigWithInstalled updates configuration with installed language servers
func UpdateConfigWithInstalled(configPath string) error {
	common.CLILogger.Info("Updating configuration with installed language servers...")

	manager := installer.GetDefaultInstallManager()
	updater := installer.NewConfigUpdater(manager)

	// Get current config or use default
	var currentConfig *config.Config
	if configPath != "" {
		if loadedConfig, err := config.LoadConfig(configPath); err == nil {
			currentConfig = loadedConfig
		}
	}

	if currentConfig == nil {
		currentConfig = config.GetDefaultConfig()
	}

	// Update config with installed servers
	updatedConfig, err := updater.UpdateConfigWithInstalledServers(currentConfig)
	if err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}

	// Determine config path
	saveConfigPath := configPath
	if saveConfigPath == "" {
		saveConfigPath = config.GetDefaultConfigPath()
	}

	// Save updated config
	if err := updater.SaveUpdatedConfig(updatedConfig, saveConfigPath); err != nil {
		return fmt.Errorf("failed to save updated config: %w", err)
	}

	common.CLILogger.Info("Configuration updated successfully")
	return nil
}
