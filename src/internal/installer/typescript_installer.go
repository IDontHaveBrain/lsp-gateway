package installer

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
)

// TypeScriptInstaller handles TypeScript/JavaScript language server installation
type TypeScriptInstaller struct {
	*BaseInstaller
}

// NewTypeScriptInstaller creates a new TypeScript installer
func NewTypeScriptInstaller(platform PlatformInfo) *TypeScriptInstaller {
	serverConfig := &config.ServerConfig{
		Command: "typescript-language-server",
		Args:    []string{"--stdio"},
	}

	base := NewBaseInstaller("typescript", serverConfig, platform)

	return &TypeScriptInstaller{
		BaseInstaller: base,
	}
}

// Install installs typescript-language-server using npm
func (t *TypeScriptInstaller) Install(ctx context.Context, options InstallOptions) error {
	common.CLILogger.Info("Installing TypeScript language server...")

	// Check if npm is installed
	if !t.isNpmInstalled() {
		return fmt.Errorf("npm is not installed. Please install Node.js and npm first")
	}

	// Determine versions to install
	tsVersion := options.Version
	tsServerPackage := "typescript-language-server"
	typescriptPackage := "typescript"

	if tsVersion != "" {
		tsServerPackage = fmt.Sprintf("typescript-language-server@%s", tsVersion)
		typescriptPackage = fmt.Sprintf("typescript@%s", tsVersion)
	}

	common.CLILogger.Info("Installing TypeScript language server and TypeScript compiler...")

	// Install both typescript-language-server and typescript
	packages := []string{"install", "-g", tsServerPackage, typescriptPackage}

	if err := t.RunCommand(ctx, "npm", packages...); err != nil {
		return fmt.Errorf("failed to install TypeScript language server: %w", err)
	}

	common.CLILogger.Info("TypeScript language server installation completed")
	return nil
}

// Uninstall removes typescript-language-server and typescript
func (t *TypeScriptInstaller) Uninstall() error {
	if !t.isNpmInstalled() {
		return fmt.Errorf("npm not available for uninstalling TypeScript language server")
	}

	common.CLILogger.Info("Uninstalling TypeScript language server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Uninstall both packages
	if err := t.RunCommand(ctx, "npm", "uninstall", "-g", "typescript-language-server", "typescript"); err != nil {
		return fmt.Errorf("failed to uninstall TypeScript language server: %w", err)
	}

	common.CLILogger.Info("TypeScript language server uninstalled successfully")
	return nil
}

// GetVersion returns the installed typescript-language-server version
func (t *TypeScriptInstaller) GetVersion() (string, error) {
	return t.GetVersionByCommand("typescript-language-server", "--version")
}

// IsInstalled checks if typescript-language-server is installed and working
func (t *TypeScriptInstaller) IsInstalled() bool {
	return t.IsInstalledByCommand("typescript-language-server")
}

// ValidateInstallation performs comprehensive validation
func (t *TypeScriptInstaller) ValidateInstallation() error {
	// Basic validation using generic method
	if err := t.ValidateByCommand("typescript-language-server"); err != nil {
		return err
	}

	// TypeScript-specific validation
	if !t.isNpmInstalled() {
		return fmt.Errorf("npm is not available but typescript-language-server is present - this may cause issues")
	}

	// Check that TypeScript compiler is also available
	if !t.isTypeScriptInstalled() {
		common.CLILogger.Warn("TypeScript compiler (tsc) not found - language server may have limited functionality")
	}

	// Test that typescript-language-server can start
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Test typescript-language-server help or version
	if _, err := t.RunCommandWithOutput(ctx, "typescript-language-server", "--help"); err != nil {
		return fmt.Errorf("typescript-language-server validation failed - unable to run help command: %w", err)
	}

	common.CLILogger.Info("TypeScript language server validation successful")
	return nil
}

// isNpmInstalled checks if npm is installed
func (t *TypeScriptInstaller) isNpmInstalled() bool {
	_, err := exec.LookPath("npm")
	return err == nil
}

// isTypeScriptInstalled checks if TypeScript compiler is installed
func (t *TypeScriptInstaller) isTypeScriptInstalled() bool {
	_, err := exec.LookPath("tsc")
	return err == nil
}
