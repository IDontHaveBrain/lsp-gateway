package installer

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"lsp-gateway/src/internal/common"
)

// TypeScriptInstaller handles TypeScript/JavaScript language server installation
type TypeScriptInstaller struct {
	*BaseInstaller
}

// NewTypeScriptInstaller creates a new TypeScript installer
func NewTypeScriptInstaller(platform PlatformInfo) *TypeScriptInstaller {
	base := CreateSimpleInstaller("typescript", "typescript-language-server", []string{"--stdio"}, platform)

	return &TypeScriptInstaller{
		BaseInstaller: base,
	}
}

// Install installs typescript-language-server using npm
func (t *TypeScriptInstaller) Install(ctx context.Context, options InstallOptions) error {
	// Install typescript-language-server
	if err := t.InstallWithPackageManager(ctx, "npm", "typescript-language-server", options.Version); err != nil {
		return fmt.Errorf("failed to install TypeScript language server: %w", err)
	}

	// Also install typescript compiler
	if err := t.InstallWithPackageManager(ctx, "npm", "typescript", options.Version); err != nil {
		common.CLILogger.Warn("Failed to install TypeScript compiler: %v", err)
	}

	common.CLILogger.Info("TypeScript language server installation completed")
	return nil
}

// Uninstall removes typescript-language-server and typescript
func (t *TypeScriptInstaller) Uninstall() error {
	if err := t.UninstallWithPackageManager("npm", "typescript-language-server"); err != nil {
		return err
	}
	// Also uninstall typescript compiler
	if err := t.UninstallWithPackageManager("npm", "typescript"); err != nil {
		common.CLILogger.Warn("Failed to uninstall TypeScript compiler: %v", err)
	}
	return nil
}

// ValidateInstallation performs comprehensive validation
func (t *TypeScriptInstaller) ValidateInstallation() error {
	// Use consolidated validation
	if err := t.ValidateWithPackageManager("typescript-language-server", "npm"); err != nil {
		return err
	}

	// Additional TypeScript-specific checks
	if !t.isTypeScriptInstalled() {
		common.CLILogger.Warn("TypeScript compiler (tsc) not found - language server may have limited functionality")
	}

	// Test that typescript-language-server can start
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if _, err := t.RunCommandWithOutput(ctx, "typescript-language-server", "--help"); err != nil {
		return fmt.Errorf("typescript-language-server validation failed - unable to run help command: %w", err)
	}

	return nil
}

// isTypeScriptInstalled checks if TypeScript compiler is installed
func (t *TypeScriptInstaller) isTypeScriptInstalled() bool {
	_, err := exec.LookPath("tsc")
	return err == nil
}
