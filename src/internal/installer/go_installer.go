package installer

import (
	"context"
	"fmt"
	"os/exec"
	"time"

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
	common.CLILogger.Info("Installing Go language server (gopls)...")

	// Check if Go is installed
	if !g.isGoInstalled() {
		return fmt.Errorf("Go is not installed. Please install Go first from https://golang.org/dl/")
	}

	// Determine version to install
	version := options.Version
	if version == "" {
		version = "latest"
	}

	// Install gopls using go install
	installTarget := fmt.Sprintf("golang.org/x/tools/gopls@%s", version)

	common.CLILogger.Info("Installing gopls from %s", installTarget)

	if err := g.RunCommand(ctx, "go", "install", installTarget); err != nil {
		return fmt.Errorf("failed to install gopls: %w", err)
	}

	common.CLILogger.Info("gopls installation completed")
	return nil
}

// Uninstall removes gopls (Go doesn't have built-in uninstall, so we'll provide guidance)
func (g *GoInstaller) Uninstall() error {
	common.CLILogger.Info("Go doesn't provide built-in uninstall for gopls")
	common.CLILogger.Info("To manually remove gopls:")
	common.CLILogger.Info("1. Remove $GOBIN/gopls (usually ~/go/bin/gopls)")
	common.CLILogger.Info("2. Clean module cache: go clean -modcache")

	return fmt.Errorf("manual uninstall required - see instructions above")
}

// GetVersion returns the installed gopls version
func (g *GoInstaller) GetVersion() (string, error) {
	return g.GetVersionByCommand("gopls", "version")
}

// IsInstalled checks if gopls is installed and working
func (g *GoInstaller) IsInstalled() bool {
	return g.IsInstalledByCommand("gopls")
}

// ValidateInstallation performs comprehensive validation
func (g *GoInstaller) ValidateInstallation() error {
	// Basic validation using generic method
	if err := g.ValidateByCommand("gopls"); err != nil {
		return err
	}

	// Go-specific validation
	if !g.isGoInstalled() {
		return fmt.Errorf("Go is not installed but gopls is present - this may cause issues")
	}

	// Test that gopls can start in LSP mode
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Test gopls help command
	if _, err := g.RunCommandWithOutput(ctx, "gopls", "help"); err != nil {
		return fmt.Errorf("gopls validation failed - unable to run help command: %w", err)
	}

	common.CLILogger.Info("gopls validation successful")
	return nil
}

// isGoInstalled checks if Go compiler is installed
func (g *GoInstaller) isGoInstalled() bool {
	_, err := exec.LookPath("go")
	return err == nil
}
