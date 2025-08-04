package installer

import (
	"lsp-gateway/src/internal/common"
)

// CreateInstallManager creates and configures a new install manager with all language installers
func CreateInstallManager() *LSPInstallManager {
	manager := NewLSPInstallManager()
	platform := NewLSPPlatformInfo()

	// Check platform support
	if !platform.IsSupported() {
		common.CLILogger.Warn("Current platform %s not fully supported for auto-installation", platform.GetPlatformString())
	}

	// Register all language installers
	registerAllInstallers(manager, platform)

	common.CLILogger.Info("Install manager initialized with %d language installers", len(manager.GetSupportedLanguages()))

	return manager
}

// registerAllInstallers registers all supported language installers
func registerAllInstallers(manager *LSPInstallManager, platform PlatformInfo) {
	// Go installer
	goInstaller := NewGoInstaller(platform)
	manager.RegisterInstaller("go", goInstaller)

	// Python installer
	pythonInstaller := NewPythonInstaller(platform)
	manager.RegisterInstaller("python", pythonInstaller)

	// TypeScript installer (handles both TypeScript and JavaScript)
	typescriptInstaller := NewTypeScriptInstaller(platform)
	manager.RegisterInstaller("typescript", typescriptInstaller)
	manager.RegisterInstaller("javascript", typescriptInstaller) // Same installer for JS

	// Java installer
	javaInstaller := NewJavaInstaller(platform)
	manager.RegisterInstaller("java", javaInstaller)
}

// GetDefaultInstallManager returns a pre-configured install manager
// This is the main entry point for other packages
func GetDefaultInstallManager() *LSPInstallManager {
	return CreateInstallManager()
}
