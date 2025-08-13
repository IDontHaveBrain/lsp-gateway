package installer

import (
	"fmt"
	"os/exec"
	"time"

	"lsp-gateway/src/internal/common"
	icommon "lsp-gateway/src/internal/common"
)

// TypeScriptInstaller handles TypeScript/JavaScript language server installation
type TypeScriptInstaller struct {
	*GenericPackageInstaller
}

// NewTypeScriptInstaller creates a new TypeScript installer
func NewTypeScriptInstaller(platform PlatformInfo) *TypeScriptInstaller {
	generic, err := NewGenericInstaller("typescript", platform)
	if err != nil {
		// Fallback to manual creation if generic fails
		base := CreateSimpleInstaller("typescript", "typescript-language-server", []string{"--stdio"}, platform)
		return &TypeScriptInstaller{
			GenericPackageInstaller: &GenericPackageInstaller{
				BaseInstaller: base,
				config: PackageConfig{
					Manager:  "npm",
					Packages: []string{"typescript-language-server", "typescript"},
				},
			},
		}
	}

	return &TypeScriptInstaller{
		GenericPackageInstaller: generic,
	}
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
	ctx, cancel := icommon.CreateContext(3 * time.Second)
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
