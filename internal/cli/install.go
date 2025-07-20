package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"

	"lsp-gateway/internal/installer"

	"github.com/spf13/cobra"
)

type (
	InstallResult        = installer.InstallResult
	InstallOptions       = installer.InstallOptions
	ServerInstallOptions = installer.ServerInstallOptions
	InstallerError       = installer.InstallerError
	InstallerErrorType   = installer.InstallerErrorType
)

const (
	InstallerErrorTypeNotFound = installer.InstallerErrorTypeNotFound
	allRuntimesKeyword         = "all"
)

var (
	installForce   bool
	installVersion string
	installJSON    bool
	installTimeout time.Duration
)

var installCmd = &cobra.Command{
	Use:   CmdInstall,
	Short: "Install runtime dependencies and language servers",
	Long: `Install command provides capabilities to install programming language runtimes
and their associated language servers for LSP Gateway functionality.

Available install targets:
- runtime <name>: Install specific runtime (go, python, nodejs, java)
- runtime all:    Install all missing runtimes
- server <name>:  Install specific language server (gopls, pylsp, typescript-language-server, jdtls)
- servers:        Install all missing language servers

Examples:
  lsp-gateway install runtime go           # Install Go runtime
  lsp-gateway install runtime python       # Install Python runtime
  lsp-gateway install runtime all          # Install all missing runtimes
  lsp-gateway install runtime go --force   # Force reinstall Go runtime
  lsp-gateway install runtime go --version 1.21.0  # Install specific version
  lsp-gateway install server gopls         # Install Go language server
  lsp-gateway install server pylsp         # Install Python language server
  lsp-gateway install servers              # Install all missing language servers
  lsp-gateway install servers --force      # Force reinstall all language servers`,
}

var installRuntimeCmd = &cobra.Command{
	Use:   "runtime <name|all>",
	Short: "Install programming language runtimes",
	Long: `Install programming language runtimes required for LSP functionality.

Supported runtimes:
- go:     Go programming language (1.19+)
- python: Python programming language (3.8+)
- nodejs: Node.js JavaScript runtime (18.0+)
- java:   Java Development Kit (17+)
- all:    Install all missing runtimes

The installer will:
1. Detect the current platform and available package managers
2. Download and install the runtime using the appropriate method
3. Verify the installation was successful
4. Configure the runtime for optimal LSP functionality

Installation methods vary by platform:
- Linux:   apt, dnf, yum, pacman (depending on distribution)
- macOS:   Homebrew (brew)
- Windows: winget, Chocolatey`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"go", "python", "nodejs", "java", allRuntimesKeyword},
	RunE:      runInstallRuntime,
}

var installServerCmd = &cobra.Command{
	Use:   "server <name>",
	Short: "Install a specific language server",
	Long: `Install a specific language server for LSP functionality.

Supported language servers:
- gopls:                    Go language server
- pylsp:                    Python language server  
- typescript-language-server: TypeScript/JavaScript language server
- jdtls:                    Java language server (Eclipse JDT)

The installer will:
1. Verify the required runtime is installed
2. Install the language server using the appropriate method
3. Verify the installation was successful
4. Configure the server for optimal LSP functionality

Installation methods:
- gopls:     go install golang.org/x/tools/gopls@latest
- pylsp:     pip install python-lsp-server
- typescript-language-server: npm install -g typescript-language-server typescript
- jdtls:     Manual download and configuration (requires Java 17+)`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"gopls", "pylsp", "typescript-language-server", "jdtls"},
	RunE:      runInstallServer,
}

var installServersCmd = &cobra.Command{
	Use:   "servers",
	Short: "Install all missing language servers",
	Long: `Install all missing language servers for LSP functionality.

This command will:
1. Check which runtimes are available
2. Install language servers for available runtimes
3. Skip servers whose runtimes are not installed
4. Verify each installation was successful
5. Provide a summary of installation results

Language servers will be installed based on available runtimes:
- Go runtime available     → Install gopls
- Python runtime available → Install pylsp  
- Node.js runtime available → Install typescript-language-server
- Java runtime available   → Install jdtls

Use --force to reinstall servers that are already installed.`,
	RunE: runInstallServers,
}

func init() {
	installRuntimeCmd.Flags().BoolVarP(&installForce, FLAG_FORCE, "f", false, "Force reinstall even if runtime is already installed")
	installRuntimeCmd.Flags().StringVarP(&installVersion, "version", "v", "", "Specific version to install (e.g., 1.21.0 for Go)")
	installRuntimeCmd.Flags().BoolVar(&installJSON, "json", false, "Output results in JSON format")
	installRuntimeCmd.Flags().DurationVar(&installTimeout, FLAG_TIMEOUT, 10*time.Minute, "Installation timeout")

	installServerCmd.Flags().BoolVarP(&installForce, FLAG_FORCE, "f", false, "Force reinstall even if server is already installed")
	installServerCmd.Flags().BoolVar(&installJSON, "json", false, "Output results in JSON format")
	installServerCmd.Flags().DurationVar(&installTimeout, FLAG_TIMEOUT, 10*time.Minute, "Installation timeout")

	installServersCmd.Flags().BoolVarP(&installForce, FLAG_FORCE, "f", false, "Force reinstall even if servers are already installed")
	installServersCmd.Flags().BoolVar(&installJSON, "json", false, "Output results in JSON format")
	installServersCmd.Flags().DurationVar(&installTimeout, FLAG_TIMEOUT, 10*time.Minute, "Installation timeout")

	installCmd.AddCommand(installRuntimeCmd)
	installCmd.AddCommand(installServerCmd)
	installCmd.AddCommand(installServersCmd)

	rootCmd.AddCommand(installCmd)
}

func runInstallRuntime(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), installTimeout)
	defer cancel()

	runtimeName := args[0]

	runtimeInstaller := installer.NewRuntimeInstaller()
	if runtimeInstaller == nil {
		return NewInstallerCreationError("runtime")
	}
	
	// Defensive check for strategy availability
	if strategy := runtimeInstaller.GetPlatformStrategy(runtime.GOOS); strategy == nil {
		return NewInstallerCreationError(fmt.Sprintf("platform strategy for %s", runtime.GOOS))
	}

	var results []*InstallResult
	var installError error

	if runtimeName == allRuntimesKeyword {
		results, installError = installAllRuntimes(ctx, runtimeInstaller)
	} else {
		result, err := installSingleRuntime(ctx, runtimeInstaller, runtimeName)
		if result != nil {
			results = []*installer.InstallResult{result}
		}
		installError = err
	}

	if installJSON {
		return outputInstallResultsJSON(results, installError)
	} else {
		return outputInstallResultsHuman(results, installError)
	}
}

func installAllRuntimes(ctx context.Context, installer *installer.DefaultRuntimeInstaller) ([]*InstallResult, error) {
	// Add defensive nil check
	if installer == nil {
		return nil, NewInstallerCreationError("runtime installer is nil")
	}
	
	supportedRuntimes := installer.GetSupportedRuntimes()
	results := make([]*InstallResult, 0, len(supportedRuntimes))
	var firstError error

	fmt.Printf("Installing all missing runtimes...\n")

	for i, runtimeName := range supportedRuntimes {
		fmt.Printf("\n[%d/%d] Installing %s runtime...\n", i+1, len(supportedRuntimes), runtimeName)

		if !installForce {
			if verifyResult, err := installer.Verify(runtimeName); err == nil && verifyResult.Installed && verifyResult.Compatible {
				fmt.Printf(LABEL_SUCCESS_RUNTIME, runtimeName, verifyResult.Version)

				result := &InstallResult{
					Success:  true,
					Runtime:  runtimeName,
					Version:  verifyResult.Version,
					Path:     verifyResult.Path,
					Duration: 0,
					Method:   "already_installed",
					Messages: []string{"Runtime already installed and verified"},
				}
				results = append(results, result)
				continue
			}
		}

		options := InstallOptions{
			Version:  installVersion,
			Force:    installForce,
			Timeout:  installTimeout,
			Platform: runtime.GOOS,
		}

		result, err := installer.Install(runtimeName, options)
		if result != nil {
			results = append(results, result)
		}

		if err != nil {
			if firstError == nil {
				firstError = err
			}
			fmt.Printf("✗ Failed to install %s runtime: %v\n", runtimeName, err)
		} else if result.Success {
			fmt.Printf("✓ Successfully installed %s runtime (version: %s)\n", runtimeName, result.Version)
		} else {
			fmt.Printf("✗ Installation of %s runtime completed with errors\n", runtimeName)
		}
	}

	fmt.Printf("\nRuntime installation summary:\n")
	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		}
	}

	fmt.Printf("- Successfully installed: %d/%d runtimes\n", successCount, len(results))
	if successCount < len(results) {
		fmt.Printf("- Failed installations: %d\n", len(results)-successCount)
	}

	return results, firstError
}

func installSingleRuntime(ctx context.Context, installer *installer.DefaultRuntimeInstaller, runtimeName string) (*InstallResult, error) {
	// Add defensive nil check
	if installer == nil {
		return nil, NewInstallerCreationError("runtime installer is nil")
	}
	
	supportedRuntimes := installer.GetSupportedRuntimes()
	isSupported := false
	for _, supported := range supportedRuntimes {
		if supported == runtimeName {
			isSupported = true
			break
		}
	}

	if !isSupported {
		return nil, &InstallerError{
			Type:    InstallerErrorTypeNotFound,
			Message: fmt.Sprintf("unsupported runtime '%s'. Supported runtimes: %s", runtimeName, strings.Join(supportedRuntimes, ", ")),
		}
	}

	fmt.Printf("Installing %s runtime...\n", runtimeName)

	if !installForce {
		if verifyResult, err := installer.Verify(runtimeName); err == nil && verifyResult.Installed && verifyResult.Compatible {
			fmt.Printf(LABEL_SUCCESS_RUNTIME, runtimeName, verifyResult.Version)
			fmt.Printf("  Use --force flag to reinstall\n")

			return &InstallResult{
				Success:  true,
				Runtime:  runtimeName,
				Version:  verifyResult.Version,
				Path:     verifyResult.Path,
				Duration: 0,
				Method:   "already_installed",
				Messages: []string{"Runtime already installed and verified"},
			}, nil
		}
	}

	runtimeInfo, err := installer.GetRuntimeInfo(runtimeName)
	if err != nil {
		return nil, NewInfoRetrievalError("runtime", err)
	}

	fmt.Printf(FORMAT_INSTALLING, runtimeInfo.DisplayName, runtimeInfo.RecommendedVersion)
	if installVersion != "" {
		fmt.Printf("Requested version: %s\n", installVersion)
	}

	options := InstallOptions{
		Version:  installVersion,
		Force:    installForce,
		Timeout:  installTimeout,
		Platform: runtime.GOOS,
	}

	fmt.Printf("Installing %s runtime...\n", runtimeName)

	startTime := time.Now()
	result, err := installer.Install(runtimeName, options)
	duration := time.Since(startTime)

	if err != nil {
		fmt.Printf("✗ Installation failed after %v: %v\n", duration, err)
		return result, err
	}

	if result.Success {
		fmt.Printf("✓ Successfully installed %s runtime in %v\n", runtimeName, duration)
		fmt.Printf("  Version: %s\n", result.Version)
		fmt.Printf("  Path: %s\n", result.Path)
		fmt.Printf("  Method: %s\n", result.Method)

		if len(result.Messages) > 0 {
			fmt.Printf("  Messages:\n")
			for _, msg := range result.Messages {
				fmt.Printf("    - %s\n", msg)
			}
		}

		if len(result.Warnings) > 0 {
			fmt.Printf("  Warnings:\n")
			for _, warning := range result.Warnings {
				fmt.Printf("    ⚠ %s\n", warning)
			}
		}
	} else {
		fmt.Printf("✗ Installation completed with errors:\n")
		for _, errMsg := range result.Errors {
			fmt.Printf("    - %s\n", errMsg)
		}
	}

	return result, nil
}

func outputInstallResultsJSON(results []*InstallResult, installError error) error {
	output := struct {
		Success bool             `json:"success"`
		Results []*InstallResult `json:"results"`
		Error   string           `json:"error,omitempty"`
	}{
		Success: installError == nil,
		Results: results,
	}

	if installError != nil {
		output.Error = installError.Error()
	}

	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return NewJSONMarshalError("runtime status", err)
	}

	fmt.Println(string(jsonData))
	return installError
}

func outputInstallResultsHuman(results []*InstallResult, installError error) error {
	if len(results) == 0 {
		if installError != nil {
			return installError
		}
		fmt.Println("No installation results to display")
		return nil
	}

	return installError
}

func runInstallServer(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), installTimeout)
	defer cancel()

	serverName := args[0]

	runtimeInstaller := installer.NewRuntimeInstaller()
	if runtimeInstaller == nil {
		return NewInstallerCreationError("runtime")
	}
	
	// Defensive check for strategy availability
	if strategy := runtimeInstaller.GetPlatformStrategy(runtime.GOOS); strategy == nil {
		return NewInstallerCreationError(fmt.Sprintf("platform strategy for %s", runtime.GOOS))
	}

	serverInstaller := installer.NewServerInstaller(runtimeInstaller)
	if serverInstaller == nil {
		return NewInstallerCreationError("server")
	}

	result, err := installSingleServer(ctx, serverInstaller, serverName)

	var results []*InstallResult
	if result != nil {
		results = []*InstallResult{result}
	}

	if installJSON {
		return outputInstallResultsJSON(results, err)
	} else {
		return outputInstallResultsHuman(results, err)
	}
}

func runInstallServers(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), installTimeout)
	defer cancel()

	runtimeInstaller := installer.NewRuntimeInstaller()
	if runtimeInstaller == nil {
		return NewInstallerCreationError("runtime")
	}
	
	// Defensive check for strategy availability
	if strategy := runtimeInstaller.GetPlatformStrategy(runtime.GOOS); strategy == nil {
		return NewInstallerCreationError(fmt.Sprintf("platform strategy for %s", runtime.GOOS))
	}

	serverInstaller := installer.NewServerInstaller(runtimeInstaller)
	if serverInstaller == nil {
		return NewInstallerCreationError("server")
	}

	results, installError := installAllServers(ctx, serverInstaller)

	if installJSON {
		return outputInstallResultsJSON(results, installError)
	} else {
		return outputInstallResultsHuman(results, installError)
	}
}

func installSingleServer(ctx context.Context, installer *installer.DefaultServerInstaller, serverName string) (*InstallResult, error) {
	// Add defensive nil check
	if installer == nil {
		return nil, NewInstallerCreationError("server installer is nil")
	}
	
	supportedServers := installer.GetSupportedServers()
	isSupported := false
	for _, supported := range supportedServers {
		if supported == serverName {
			isSupported = true
			break
		}
	}

	if !isSupported {
		return nil, &InstallerError{
			Type:    InstallerErrorTypeNotFound,
			Message: fmt.Sprintf("unsupported server '%s'. Supported servers: %s", serverName, strings.Join(supportedServers, ", ")),
		}
	}

	fmt.Printf("Installing %s language server...\n", serverName)

	if !installForce {
		if verifyResult, err := installer.Verify(serverName); err == nil && verifyResult.Installed && verifyResult.Compatible {
			fmt.Printf(LABEL_SUCCESS_SERVER, serverName, verifyResult.Version)
			fmt.Printf("  Use --force flag to reinstall\n")

			return &InstallResult{
				Success:  true,
				Runtime:  serverName,
				Version:  verifyResult.Version,
				Path:     verifyResult.Path,
				Duration: 0,
				Method:   "already_installed",
				Messages: []string{"Server already installed and verified"},
			}, nil
		}
	}

	serverInfo, err := installer.GetServerInfo(serverName)
	if err != nil {
		return nil, NewInfoRetrievalError("server", err)
	}

	fmt.Printf(FORMAT_INSTALLING, serverInfo.DisplayName, serverInfo.Name)
	fmt.Printf("Required runtime: %s\n", serverInfo.Runtime)

	startTime := time.Now()
	var result *InstallResult

	options := ServerInstallOptions{
		Version: "",
		Force:   installForce,
		Timeout: installTimeout,
	}

	result, err = installer.Install(serverName, options)

	duration := time.Since(startTime)

	if err != nil {
		fmt.Printf("✗ Installation failed after %v: %v\n", duration, err)
		return result, err
	}

	if result.Success {
		fmt.Printf("✓ Successfully installed %s server in %v\n", serverName, duration)
		fmt.Printf("  Version: %s\n", result.Version)
		fmt.Printf("  Path: %s\n", result.Path)
		fmt.Printf("  Method: %s\n", result.Method)

		if len(result.Messages) > 0 {
			fmt.Printf("  Messages:\n")
			for _, msg := range result.Messages {
				fmt.Printf("    - %s\n", msg)
			}
		}

		if len(result.Warnings) > 0 {
			fmt.Printf("  Warnings:\n")
			for _, warning := range result.Warnings {
				fmt.Printf("    ⚠ %s\n", warning)
			}
		}
	} else {
		fmt.Printf("✗ Installation completed with errors:\n")
		for _, errMsg := range result.Errors {
			fmt.Printf("    - %s\n", errMsg)
		}
	}

	return result, nil
}

func installAllServers(ctx context.Context, installer *installer.DefaultServerInstaller) ([]*InstallResult, error) {
	// Add defensive nil check
	if installer == nil {
		return nil, NewInstallerCreationError("server installer is nil")
	}
	
	supportedServers := installer.GetSupportedServers()
	results := make([]*InstallResult, 0, len(supportedServers))
	var firstError error

	fmt.Printf("Installing all missing language servers...\n")

	for i, serverName := range supportedServers {
		fmt.Printf("\n[%d/%d] Installing %s server...\n", i+1, len(supportedServers), serverName)

		if !installForce {
			if verifyResult, err := installer.Verify(serverName); err == nil && verifyResult.Installed && verifyResult.Compatible {
				fmt.Printf(LABEL_SUCCESS_SERVER, serverName, verifyResult.Version)

				result := &InstallResult{
					Success:  true,
					Runtime:  serverName,
					Version:  verifyResult.Version,
					Path:     verifyResult.Path,
					Duration: 0,
					Method:   "already_installed",
					Messages: []string{"Server already installed and verified"},
				}
				results = append(results, result)
				continue
			}
		}

		var result *InstallResult
		var err error

		options := ServerInstallOptions{
			Version: "",
			Force:   installForce,
			Timeout: installTimeout,
		}

		result, err = installer.Install(serverName, options)

		if result != nil {
			results = append(results, result)
		}

		if err != nil {
			if firstError == nil {
				firstError = err
			}
			fmt.Printf("✗ Failed to install %s server: %v\n", serverName, err)
		} else if result.Success {
			fmt.Printf("✓ Successfully installed %s server (version: %s)\n", serverName, result.Version)
		} else {
			fmt.Printf("✗ Installation of %s server completed with errors\n", serverName)
		}
	}

	fmt.Printf("\nLanguage server installation summary:\n")
	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		}
	}

	fmt.Printf("- Successfully installed: %d/%d servers\n", successCount, len(results))
	if successCount < len(results) {
		fmt.Printf("- Failed installations: %d\n", len(results)-successCount)
	}

	return results, firstError
}
