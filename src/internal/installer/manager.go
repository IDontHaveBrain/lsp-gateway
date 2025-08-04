package installer

import (
	"context"
	"fmt"
	"sync"

	"lsp-gateway/src/internal/common"
)

// LSPInstallManager implements InstallManager interface
type LSPInstallManager struct {
	installers map[string]LanguageInstaller
	mutex      sync.RWMutex
}

// NewLSPInstallManager creates a new install manager
func NewLSPInstallManager() *LSPInstallManager {
	return &LSPInstallManager{
		installers: make(map[string]LanguageInstaller),
	}
}

// RegisterInstaller registers a language installer
func (m *LSPInstallManager) RegisterInstaller(language string, installer LanguageInstaller) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.installers[language] = installer
	common.CLILogger.Info("Registered installer for %s", language)
}

// GetInstaller returns installer for a specific language
func (m *LSPInstallManager) GetInstaller(language string) (LanguageInstaller, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	installer, exists := m.installers[language]
	if !exists {
		return nil, fmt.Errorf("no installer found for language: %s", language)
	}

	return installer, nil
}

// InstallLanguage installs a specific language server
func (m *LSPInstallManager) InstallLanguage(ctx context.Context, language string, options InstallOptions) error {
	installer, err := m.GetInstaller(language)
	if err != nil {
		return fmt.Errorf("failed to get installer for %s: %w", language, err)
	}

	common.CLILogger.Info("Installing %s language server...", language)

	// Check if already installed and not forcing reinstall
	if !options.Force && installer.IsInstalled() {
		version, _ := installer.GetVersion()
		common.CLILogger.Info("%s language server already installed (version: %s)", language, version)
		return nil
	}

	// Perform installation
	if err := installer.Install(ctx, options); err != nil {
		return fmt.Errorf("failed to install %s: %w", language, err)
	}

	// Validate installation if not skipped
	if !options.SkipValidation {
		if err := installer.ValidateInstallation(); err != nil {
			return fmt.Errorf("installation validation failed for %s: %w", language, err)
		}
	}

	common.CLILogger.Info("Successfully installed %s language server", language)
	return nil
}

// InstallAll installs all supported language servers
func (m *LSPInstallManager) InstallAll(ctx context.Context, options InstallOptions) error {
	languages := m.GetSupportedLanguages()
	if len(languages) == 0 {
		return fmt.Errorf("no language installers registered")
	}

	common.CLILogger.Info("Installing all supported language servers...")

	var errors []error
	successCount := 0

	for _, language := range languages {
		if err := m.InstallLanguage(ctx, language, options); err != nil {
			common.CLILogger.Error("Failed to install %s: %v", language, err)
			errors = append(errors, fmt.Errorf("%s: %w", language, err))
		} else {
			successCount++
		}
	}

	common.CLILogger.Info("Installation complete: %d/%d successful", successCount, len(languages))

	if len(errors) > 0 {
		return fmt.Errorf("some installations failed: %d errors", len(errors))
	}

	return nil
}

// GetStatus returns installation status for all languages
func (m *LSPInstallManager) GetStatus() map[string]InstallStatus {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	status := make(map[string]InstallStatus)

	for language, installer := range m.installers {
		installStatus := InstallStatus{
			Language:  language,
			Available: true,
		}

		// Check if installed
		installStatus.Installed = installer.IsInstalled()

		// Get version if installed
		if installStatus.Installed {
			if version, err := installer.GetVersion(); err == nil {
				installStatus.Version = version
			} else {
				installStatus.Error = err
			}
		}

		status[language] = installStatus
	}

	return status
}

// GetSupportedLanguages returns list of supported languages
func (m *LSPInstallManager) GetSupportedLanguages() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	languages := make([]string, 0, len(m.installers))
	for language := range m.installers {
		languages = append(languages, language)
	}

	return languages
}
