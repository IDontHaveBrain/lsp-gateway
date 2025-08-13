package installer

// GoInstaller handles Go language server (gopls) installation
type GoInstaller struct {
	*GenericPackageInstaller
}

// NewGoInstaller creates a new Go installer
func NewGoInstaller(platform PlatformInfo) *GoInstaller {
	generic, err := NewGenericInstaller("go", platform)
	if err != nil {
		// Fallback to manual creation if generic fails
		base := CreateSimpleInstaller("go", "gopls", []string{"serve"}, platform)
		return &GoInstaller{
			GenericPackageInstaller: &GenericPackageInstaller{
				BaseInstaller: base,
				config: PackageConfig{
					Manager:  "go",
					Packages: []string{"golang.org/x/tools/gopls"},
				},
			},
		}
	}

	return &GoInstaller{
		GenericPackageInstaller: generic,
	}
}
