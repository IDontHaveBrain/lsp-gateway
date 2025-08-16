package installer

import (
    "context"
    "fmt"
    "path/filepath"

    icommon "lsp-gateway/src/internal/common"
)

// CSharpInstaller handles C# language server (csharp-ls) installation via dotnet tool
type CSharpInstaller struct {
    *BaseInstaller
}

// NewCSharpInstaller creates a new C# installer
func NewCSharpInstaller(platform PlatformInfo) *CSharpInstaller {
    base := CreateSimpleInstaller("csharp", "csharp-ls", []string{}, platform)
    return &CSharpInstaller{BaseInstaller: base}
}

// Install installs csharp-ls using dotnet tool into the standard lsp-gateway tools path
func (c *CSharpInstaller) Install(ctx context.Context, options InstallOptions) error {
    if !c.isCommandInstalled("dotnet", "--version") {
        return fmt.Errorf("dotnet is not installed. Install .NET SDK from https://dotnet.microsoft.com/download")
    }

    // Ensure install directory exists and install to bin subdir to match resolver
    installBin := filepath.Join(c.GetInstallPath(), "bin")
    if err := c.CreateInstallDirectory(installBin); err != nil {
        return err
    }

    pkg := "csharp-ls"

    // Build base args for dotnet tool
    args := []string{"tool", "install", "--tool-path", installBin, pkg}
    if options.Version != "" && options.Version != "latest" {
        args = append(args, "--version", options.Version)
    }

    // Try install first
    installErr := c.RunCommand(ctx, "dotnet", args...)
    if installErr != nil {
        // If already installed, try update
        updateArgs := []string{"tool", "update", "--tool-path", installBin, pkg}
        if options.Version != "" && options.Version != "latest" {
            updateArgs = append(updateArgs, "--version", options.Version)
        }
        if err := c.RunCommand(ctx, "dotnet", updateArgs...); err != nil {
            return fmt.Errorf("failed to install/update csharp-ls: %w", installErr)
        }
    }

    return nil
}

// Uninstall removes csharp-ls from the tool-path
func (c *CSharpInstaller) Uninstall() error {
    if !c.isCommandInstalled("dotnet", "--version") {
        return nil
    }
    installBin := filepath.Join(c.GetInstallPath(), "bin")
    ctx, cancel := icommon.CreateContext(30 * 1e9) // 30s
    defer cancel()
    _ = c.RunCommand(ctx, "dotnet", "tool", "uninstall", "--tool-path", installBin, "csharp-ls")
    return nil
}

// ValidateInstallation uses default BaseInstaller validation
// IsInstalled also uses BaseInstaller behavior

