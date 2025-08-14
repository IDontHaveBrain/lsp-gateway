package installer

import (
	"context"
	"fmt"
)

type PythonInstaller struct {
	*GenericPackageInstaller
}

func NewPythonInstaller(platform PlatformInfo) *PythonInstaller {
	generic, err := NewGenericInstaller("python", platform)
	if err != nil {
		base := CreateSimpleInstaller("python", "jedi-language-server", []string{}, platform)
		return &PythonInstaller{
			GenericPackageInstaller: &GenericPackageInstaller{
				BaseInstaller: base,
				config: PackageConfig{
					Manager:  "pip",
					Packages: []string{"jedi-language-server"},
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
	// Only support jedi-language-server
	if options.Server != "" && options.Server != "jedi" && options.Server != "jedi-language-server" {
		return fmt.Errorf("unsupported python server variant: %s (supported: jedi)", options.Server)
	}

	p.BaseInstaller.serverConfig.Command = "jedi-language-server"
	p.BaseInstaller.serverConfig.Args = []string{}
	p.config.Manager = "pip"
	p.config.Packages = []string{"jedi-language-server"}

	return p.GenericPackageInstaller.Install(ctx, options)
}
