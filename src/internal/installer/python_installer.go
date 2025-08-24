package installer

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	serverBasedPyrightLS = "basedpyright-langserver"
	serverJediLS         = "jedi-language-server"
	serverPyrightLS      = "pyright-langserver"
	cmdBasedpyright      = "basedpyright"
	cmdPyright           = "pyright"
	pkgBasedPyright      = "basedpyright"
	pkgJedi              = "jedi-language-server"
	pkgPyright           = "pyright"
)

type PythonInstaller struct {
	*GenericPackageInstaller
}

func NewPythonInstaller(platform PlatformInfo) *PythonInstaller {
	generic, err := NewGenericInstaller("python", platform)
	if err != nil {
		// Default to basedpyright if no configuration found
		base := CreateSimpleInstaller("python", serverBasedPyrightLS, []string{"--stdio"}, platform)
		return &PythonInstaller{
			GenericPackageInstaller: &GenericPackageInstaller{
				BaseInstaller: base,
				config: PackageConfig{
					Manager:  cmdPip,
					Packages: []string{pkgBasedPyright},
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
	case "", "basedpyright", serverBasedPyrightLS:
		// Default to basedpyright
		p.serverConfig.Command = serverBasedPyrightLS
		p.serverConfig.Args = []string{"--stdio"}
		p.config.Manager = cmdPip
		p.config.Packages = []string{pkgBasedPyright}

	case "jedi", serverJediLS:
		// Use jedi-language-server
		p.serverConfig.Command = serverJediLS
		p.serverConfig.Args = []string{}
		p.config.Manager = cmdPip
		p.config.Packages = []string{pkgJedi}

	case "pyright", serverPyrightLS:
		// Install pyright via npm (npm always installs globally with -g flag)
		p.serverConfig.Command = serverPyrightLS
		p.serverConfig.Args = []string{"--stdio"}
		p.config.Manager = cmdNpm
		p.config.Packages = []string{pkgPyright}

	default:
		return fmt.Errorf("unsupported python server variant: %s (supported: basedpyright, jedi, pyright)", options.Server)
	}

	// If uv/uvx is available, prefer using it to run Python-based servers without system pip installs
	// Detect uvx/uv
	uvxAvailable := p.isCommandInstalled("uvx", "--version") || p.isCommandInstalled("uv", "--version")

	if uvxAvailable {
		switch p.serverConfig.Command {
		case serverBasedPyrightLS:
			// Switch to uvx-based execution
			p.serverConfig.Command = pmUVX
			p.serverConfig.Args = append([]string{serverBasedPyrightLS}, []string{"--stdio"}...)
			p.config.Manager = pmUVX
			p.config.Packages = []string{pkgBasedPyright}
		case serverJediLS:
			p.serverConfig.Command = pmUVX
			p.serverConfig.Args = []string{serverJediLS}
			p.config.Manager = pmUVX
			p.config.Packages = []string{pkgJedi}
		}
	}

	return p.GenericPackageInstaller.Install(ctx, options)
}

// IsInstalled checks if any Python language server is installed
func (p *PythonInstaller) IsInstalled() bool {
	// Check for basedpyright-langserver (default)
	if p.IsInstalledByCommand(serverBasedPyrightLS) {
		return true
	}

	// Check for jedi-language-server
	if p.IsInstalledByCommand(serverJediLS) {
		return true
	}

	// Check for pyright-langserver
	if p.IsInstalledByCommand(serverPyrightLS) {
		return true
	}

	// If uv/uvx exists, we can run Python LSPs on-demand
	if p.IsInstalledByCommand("uvx") || p.IsInstalledByCommand("uv") {
		return true
	}

	return false
}

// GetVersion returns the version of the installed Python language server
func (p *PythonInstaller) GetVersion() (string, error) {
	// Check for basedpyright-langserver first (default)
	if p.IsInstalledByCommand(serverBasedPyrightLS) {
		// Try to get version from basedpyright command
		// Note: basedpyright-langserver doesn't have --version, but basedpyright does
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, cmdBasedpyright, "--version")
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

	// If uv/uvx is present, try to get basedpyright version via uvx
	if p.IsInstalledByCommand("uvx") || p.IsInstalledByCommand("uv") {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		var cmd *exec.Cmd
		if _, err := exec.LookPath("uvx"); err == nil {
			cmd = exec.CommandContext(ctx, "uvx", cmdBasedpyright, "--version")
		} else {
			cmd = exec.CommandContext(ctx, "uv", "tool", "run", cmdBasedpyright, "--version")
		}
		output, err := cmd.Output()
		if err == nil {
			version := strings.TrimSpace(string(output))
			if strings.HasPrefix(version, "basedpyright ") {
				return strings.TrimPrefix(version, "basedpyright "), nil
			}
			if version != "" {
				return version, nil
			}
		}
		return "basedpyright (uvx)", nil
	}

	// Check for pyright-langserver
	if p.IsInstalledByCommand(serverPyrightLS) {
		// Try to get version from pyright command
		// Note: pyright-langserver doesn't have --version, but pyright does
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, cmdPyright, "--version")
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
	if p.IsInstalledByCommand(serverJediLS) {
		version, err := p.GetVersionByCommand(serverJediLS, "--version")
		if err == nil {
			// Extract version number from output like "jedi-language-server 0.45.1"
			lines := strings.Split(version, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.Contains(line, serverJediLS) {
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
