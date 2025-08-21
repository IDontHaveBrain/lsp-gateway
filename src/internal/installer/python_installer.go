package installer

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type PythonInstaller struct {
	*GenericPackageInstaller
}

func NewPythonInstaller(platform PlatformInfo) *PythonInstaller {
	generic, err := NewGenericInstaller("python", platform)
	if err != nil {
		// Default to basedpyright if no configuration found
		base := CreateSimpleInstaller("python", "basedpyright-langserver", []string{"--stdio"}, platform)
		return &PythonInstaller{
			GenericPackageInstaller: &GenericPackageInstaller{
				BaseInstaller: base,
				config: PackageConfig{
					Manager:  "pip",
					Packages: []string{"basedpyright"},
				},
			},
		}
	}

	return &PythonInstaller{
		GenericPackageInstaller: generic,
	}
}

// Install allows selecting python LSP variant via options.Server
func (p *PythonInstaller) Install(ctx context.Context, options InstallOptions) error {
	// Support basedpyright, pyright, and jedi-language-server
	switch options.Server {
	case "", "basedpyright", "basedpyright-langserver":
		// Default to basedpyright
		p.BaseInstaller.serverConfig.Command = "basedpyright-langserver"
		p.BaseInstaller.serverConfig.Args = []string{"--stdio"}
		p.config.Manager = "pip"
		p.config.Packages = []string{"basedpyright"}
		
	case "jedi", "jedi-language-server":
		// Use jedi-language-server
		p.BaseInstaller.serverConfig.Command = "jedi-language-server"
		p.BaseInstaller.serverConfig.Args = []string{}
		p.config.Manager = "pip"
		p.config.Packages = []string{"jedi-language-server"}

	case "pyright", "pyright-langserver":
		// Install pyright via npm (npm always installs globally with -g flag)
		p.BaseInstaller.serverConfig.Command = "pyright-langserver"
		p.BaseInstaller.serverConfig.Args = []string{"--stdio"}
		p.config.Manager = "npm"
		p.config.Packages = []string{"pyright"}

	default:
		return fmt.Errorf("unsupported python server variant: %s (supported: basedpyright, jedi, pyright)", options.Server)
	}

	return p.GenericPackageInstaller.Install(ctx, options)
}

// IsInstalled checks if any Python language server is installed
func (p *PythonInstaller) IsInstalled() bool {
	// Check for basedpyright-langserver (default)
	if p.BaseInstaller.IsInstalledByCommand("basedpyright-langserver") {
		return true
	}
	
	// Check for jedi-language-server
	if p.BaseInstaller.IsInstalledByCommand("jedi-language-server") {
		return true
	}
	
	// Check for pyright-langserver
	if p.BaseInstaller.IsInstalledByCommand("pyright-langserver") {
		return true
	}
	
	return false
}

// GetVersion returns the version of the installed Python language server
func (p *PythonInstaller) GetVersion() (string, error) {
	// Check for basedpyright-langserver first (default)
	if p.BaseInstaller.IsInstalledByCommand("basedpyright-langserver") {
		// Try to get version from basedpyright command
		// Note: basedpyright-langserver doesn't have --version, but basedpyright does
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		
		cmd := exec.CommandContext(ctx, "basedpyright", "--version")
		output, err := cmd.Output()
		if err == nil {
			// Extract version from output like "basedpyright 1.x.x"
			version := strings.TrimSpace(string(output))
			if strings.HasPrefix(version, "basedpyright ") {
				return strings.TrimPrefix(version, "basedpyright "), nil
			}
			if version != "" {
				return version, nil
			}
		}
		
		// If basedpyright command isn't available but basedpyright-langserver is, report as installed
		return "basedpyright (installed)", nil
	}
	
	// Check for pyright-langserver
	if p.BaseInstaller.IsInstalledByCommand("pyright-langserver") {
		// Try to get version from pyright command
		// Note: pyright-langserver doesn't have --version, but pyright does
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		
		cmd := exec.CommandContext(ctx, "pyright", "--version")
		output, err := cmd.Output()
		if err == nil {
			// Extract version from output like "pyright 1.1.403"
			version := strings.TrimSpace(string(output))
			if strings.HasPrefix(version, "pyright ") {
				return strings.TrimPrefix(version, "pyright "), nil
			}
			if version != "" {
				return version, nil
			}
		}
		
		// If pyright command isn't available but pyright-langserver is, report as installed
		return "pyright (installed)", nil
	}
	
	// Check for jedi-language-server
	if p.BaseInstaller.IsInstalledByCommand("jedi-language-server") {
		version, err := p.BaseInstaller.GetVersionByCommand("jedi-language-server", "--version")
		if err == nil {
			// Extract version number from output like "jedi-language-server 0.45.1"
			lines := strings.Split(version, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.Contains(line, "jedi-language-server") {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						return parts[len(parts)-1], nil
					}
				}
			}
			return version, nil
		}
	}
	
	return "", fmt.Errorf("python language server not installed")
}
