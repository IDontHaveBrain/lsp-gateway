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
	case "":
		p.serverConfig.Command = serverBasedPyrightLS
		p.serverConfig.Args = []string{"--stdio"}
		p.config.Manager = cmdPip
		p.config.Packages = []string{pkgBasedPyright}
	case "basedpyright", serverBasedPyrightLS:
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

	return p.GenericPackageInstaller.Install(ctx, options)
}

// IsInstalled checks if any Python language server is installed
func (p *PythonInstaller) IsInstalled() bool {
	if p.IsInstalledByCommand(serverBasedPyrightLS) {
		return true
	}

	if p.IsInstalledByCommand(serverPyrightLS) {
		return true
	}

	if p.IsInstalledByCommand(serverJediLS) {
		return true
	}

	if p.IsInstalledByCommand("uvx") || p.IsInstalledByCommand("uv") {
		return true
	}

	return false
}

// GetVersion returns the version of the installed Python language server
func (p *PythonInstaller) GetVersion() (string, error) {
	if p.IsInstalledByCommand(serverBasedPyrightLS) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, cmdBasedpyright, "--version")
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

		return "basedpyright (installed)", nil
	}

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

	if p.IsInstalledByCommand(serverPyrightLS) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, cmdPyright, "--version")
		output, err := cmd.Output()
		if err == nil {
			version := strings.TrimSpace(string(output))
			if strings.HasPrefix(version, "pyright ") {
				return strings.TrimPrefix(version, "pyright "), nil
			}
			if version != "" {
				return version, nil
			}
		}

		return "pyright (installed)", nil
	}

	if p.IsInstalledByCommand(serverJediLS) {
		version, err := p.GetVersionByCommand(serverJediLS, "--version")
		if err == nil {
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
