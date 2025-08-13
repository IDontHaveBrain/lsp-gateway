package installer

import (
	"context"
	"fmt"

	"lsp-gateway/src/internal/common"
)

// PackageConfig defines the configuration for a package-based installer
type PackageConfig struct {
	Manager  string   // Package manager: "go", "pip", "npm"
	Packages []string // Package names to install
}

// GenericPackageInstaller handles language servers that can be installed via standard package managers
type GenericPackageInstaller struct {
	*BaseInstaller
	config PackageConfig
}

// packageConfigs defines the package manager and package names for each supported language
var packageConfigs = map[string]PackageConfig{
	"go": {
		Manager:  "go",
		Packages: []string{"golang.org/x/tools/gopls"},
	},
	"python": {
		Manager:  "pip",
		Packages: []string{"python-lsp-server[all]"},
	},
	"typescript": {
		Manager:  "npm",
		Packages: []string{"typescript-language-server", "typescript"},
	},
}

// NewGenericInstaller creates a new generic package installer
func NewGenericInstaller(language string, platform PlatformInfo) (*GenericPackageInstaller, error) {
	config, exists := packageConfigs[language]
	if !exists {
		return nil, fmt.Errorf("no package configuration found for language: %s", language)
	}

	var command string
	var args []string

	switch language {
	case "go":
		command = "gopls"
		args = []string{"serve"}
	case "python":
		command = "pylsp"
		args = []string{}
	case "typescript":
		command = "typescript-language-server"
		args = []string{"--stdio"}
	}

	base := CreateSimpleInstaller(language, command, args, platform)

	return &GenericPackageInstaller{
		BaseInstaller: base,
		config:        config,
	}, nil
}

// Install installs the language server using the configured package manager
func (g *GenericPackageInstaller) Install(ctx context.Context, options InstallOptions) error {
	version := options.Version
	if version == "" {
		version = "latest"
	}

	// Install all configured packages
	for _, packageName := range g.config.Packages {
		if err := g.InstallWithPackageManager(ctx, g.config.Manager, packageName, version); err != nil {
			return fmt.Errorf("failed to install %s: %w", packageName, err)
		}
	}

	common.CLILogger.Info("%s installation completed", g.GetLanguage())
	return nil
}

// Uninstall removes the language server using the configured package manager
func (g *GenericPackageInstaller) Uninstall() error {
	// Uninstall all configured packages
	for _, packageName := range g.config.Packages {
		if err := g.UninstallWithPackageManager(g.config.Manager, packageName); err != nil {
			common.CLILogger.Warn("Failed to uninstall %s: %v", packageName, err)
		}
	}
	return nil
}

// ValidateInstallation performs comprehensive validation
func (g *GenericPackageInstaller) ValidateInstallation() error {
	command := g.GetServerConfig().Command
	return g.ValidateWithPackageManager(command, g.config.Manager)
}
