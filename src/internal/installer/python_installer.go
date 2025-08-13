package installer

// PythonInstaller handles Python language server (python-lsp-server/pylsp) installation
type PythonInstaller struct {
	*GenericPackageInstaller
}

// NewPythonInstaller creates a new Python installer
func NewPythonInstaller(platform PlatformInfo) *PythonInstaller {
	generic, err := NewGenericInstaller("python", platform)
	if err != nil {
		// Fallback to manual creation if generic fails
		base := CreateSimpleInstaller("python", "pylsp", []string{}, platform)
		return &PythonInstaller{
			GenericPackageInstaller: &GenericPackageInstaller{
				BaseInstaller: base,
				config: PackageConfig{
					Manager:  "pip",
					Packages: []string{"python-lsp-server[all]"},
				},
			},
		}
	}

	return &PythonInstaller{
		GenericPackageInstaller: generic,
	}
}
