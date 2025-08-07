package installer

import (
	"context"
	"fmt"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
)

// GoInstaller handles Go language server (gopls) installation
type GoInstaller struct {
	*BaseInstaller
}

// NewGoInstaller creates a new Go installer
func NewGoInstaller(platform PlatformInfo) *GoInstaller {
	serverConfig := &config.ServerConfig{
		Command: "gopls",
		Args:    []string{"serve"},
	}

	base := NewBaseInstaller("go", serverConfig, platform)

	return &GoInstaller{
		BaseInstaller: base,
	}
}

// Install installs gopls using go install
func (g *GoInstaller) Install(ctx context.Context, options InstallOptions) error {
	version := options.Version
	if version == "" {
		version = "latest"
	}

	if err := g.InstallWithPackageManager(ctx, "go", "golang.org/x/tools/gopls", version); err != nil {
		return fmt.Errorf("failed to install gopls: %w", err)
	}

	common.CLILogger.Info("gopls installation completed")
	return nil
}

// Uninstall removes gopls
func (g *GoInstaller) Uninstall() error {
	return g.UninstallWithPackageManager("go", "golang.org/x/tools/gopls")
}

// ValidateInstallation performs comprehensive validation
func (g *GoInstaller) ValidateInstallation() error {
	return g.ValidateWithPackageManager("gopls", "go")
}
