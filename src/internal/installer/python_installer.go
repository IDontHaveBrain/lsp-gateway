package installer

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
)

// PythonInstaller handles Python language server (python-lsp-server/pylsp) installation
type PythonInstaller struct {
	*BaseInstaller
}

// NewPythonInstaller creates a new Python installer
func NewPythonInstaller(platform PlatformInfo) *PythonInstaller {
	serverConfig := &config.ServerConfig{
		Command: "pylsp",
		Args:    []string{},
	}

	base := NewBaseInstaller("python", serverConfig, platform)

	return &PythonInstaller{
		BaseInstaller: base,
	}
}

// Install installs python-lsp-server using pip
func (p *PythonInstaller) Install(ctx context.Context, options InstallOptions) error {
	common.CLILogger.Info("Installing Python language server (python-lsp-server)...")

	// Check if pip is installed
	if !p.isPipInstalled() {
		return fmt.Errorf("pip is not installed. Please install Python and pip first")
	}

	// Determine version to install
	version := options.Version
	packageName := "python-lsp-server[all]"
	if version != "" {
		packageName = fmt.Sprintf("python-lsp-server[all]==%s", version)
	}

	common.CLILogger.Info("Installing python-lsp-server from PyPI: %s", packageName)

	// Install python-lsp-server using pip (with extra features)
	if err := p.RunCommand(ctx, "pip", "install", "--user", packageName); err != nil {
		return fmt.Errorf("failed to install python-lsp-server: %w", err)
	}

	common.CLILogger.Info("python-lsp-server installation completed")
	return nil
}

// Uninstall removes python-lsp-server
func (p *PythonInstaller) Uninstall() error {
	if !p.isPipInstalled() {
		return fmt.Errorf("pip not available for uninstalling python-lsp-server")
	}

	common.CLILogger.Info("Uninstalling python-lsp-server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := p.RunCommand(ctx, "pip", "uninstall", "-y", "python-lsp-server"); err != nil {
		return fmt.Errorf("failed to uninstall python-lsp-server: %w", err)
	}

	common.CLILogger.Info("python-lsp-server uninstalled successfully")
	return nil
}

// GetVersion returns the installed python-lsp-server version
func (p *PythonInstaller) GetVersion() (string, error) {
	if !p.IsInstalled() {
		return "", fmt.Errorf("python-lsp-server not installed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get pylsp version
	output, err := p.RunCommandWithOutput(ctx, "pylsp", "--version")
	if err != nil {
		return "", fmt.Errorf("failed to get python-lsp-server version: %w", err)
	}

	return output, nil
}

// IsInstalled checks if python-lsp-server is installed and working
func (p *PythonInstaller) IsInstalled() bool {
	// Check if pylsp exists in PATH
	if _, err := exec.LookPath("pylsp"); err == nil {
		return p.validateServerCommand("pylsp")
	}

	return false
}

// ValidateInstallation performs comprehensive validation
func (p *PythonInstaller) ValidateInstallation() error {
	// Python-specific validation
	if !p.isPipInstalled() {
		return fmt.Errorf("pip is not available but python-lsp-server may be present - this may cause issues")
	}

	// Check if pylsp is installed and working
	if !p.IsInstalled() {
		return fmt.Errorf("python-lsp-server installation validation failed: not installed")
	}

	common.CLILogger.Info("python-lsp-server validation successful")
	return nil
}

// isPipInstalled checks if pip is installed
func (p *PythonInstaller) isPipInstalled() bool {
	_, err := exec.LookPath("pip")
	return err == nil
}
