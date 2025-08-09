package installer

import (
	"context"
	"fmt"

	"lsp-gateway/src/internal/common"
)

// PythonInstaller handles Python language server (python-lsp-server/pylsp) installation
type PythonInstaller struct {
	*BaseInstaller
}

// NewPythonInstaller creates a new Python installer
func NewPythonInstaller(platform PlatformInfo) *PythonInstaller {
	base := CreateSimpleInstaller("python", "pylsp", []string{}, platform)

	return &PythonInstaller{
		BaseInstaller: base,
	}
}

// Install installs python-lsp-server using pip
func (p *PythonInstaller) Install(ctx context.Context, options InstallOptions) error {
	if err := p.InstallWithPackageManager(ctx, "pip", "python-lsp-server[all]", options.Version); err != nil {
		return fmt.Errorf("failed to install python-lsp-server: %w", err)
	}

	common.CLILogger.Info("python-lsp-server installation completed")
	return nil
}

// Uninstall removes python-lsp-server
func (p *PythonInstaller) Uninstall() error {
	return p.UninstallWithPackageManager("pip", "python-lsp-server")
}

// ValidateInstallation performs comprehensive validation
func (p *PythonInstaller) ValidateInstallation() error {
	return p.ValidateWithPackageManager("pylsp", "pip")
}
