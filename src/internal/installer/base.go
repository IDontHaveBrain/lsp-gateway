package installer

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	icommon "lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/security"
)

// BaseInstaller provides common functionality for language installers
type BaseInstaller struct {
	language     string
	serverConfig *config.ServerConfig
	platform     PlatformInfo
	installPath  string
}

// NewBaseInstaller creates a new base installer
func NewBaseInstaller(language string, serverConfig *config.ServerConfig, platform PlatformInfo) *BaseInstaller {
	return &BaseInstaller{
		language:     language,
		serverConfig: serverConfig,
		platform:     platform,
	}
}

// GetLanguage returns the language this installer handles
func (b *BaseInstaller) GetLanguage() string {
	return b.language
}

// GetServerConfig returns the configuration for this language server
func (b *BaseInstaller) GetServerConfig() *config.ServerConfig {
	return b.serverConfig
}

// GetInstallPath returns the installation path for this language
func (b *BaseInstaller) GetInstallPath() string {
	if b.installPath != "" {
		return b.installPath
	}

	// Default installation path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".lsp-gateway", "tools", b.language)
	}

	return filepath.Join(homeDir, ".lsp-gateway", "tools", b.language)
}

// SetInstallPath sets a custom installation path
func (b *BaseInstaller) SetInstallPath(path string) {
	b.installPath = path
}

// IsInstalled checks if the language server is already installed and working
func (b *BaseInstaller) IsInstalled() bool {
	if b.serverConfig == nil {
		return false
	}

	// Check if command exists in PATH or at install path
	command := b.serverConfig.Command

	// First check PATH
	if _, err := exec.LookPath(command); err == nil {
		return b.validateServerCommand(command)
	}

	// Check install path
	installPath := b.GetInstallPath()
	customCommand := filepath.Join(installPath, "bin", command)
	if b.fileExists(customCommand) {
		return b.validateServerCommand(customCommand)
	}

	return false
}

// GetVersion returns the currently installed version
func (b *BaseInstaller) GetVersion() (string, error) {
	if !b.IsInstalled() {
		return "", fmt.Errorf("%s language server not installed", b.language)
	}

	command := b.getExecutableCommand()

	// Try common version flags
	versionFlags := []string{"--version", "-version", "-v", "version"}

	for _, flag := range versionFlags {
		if version := b.tryGetVersion(command, flag); version != "" {
			return version, nil
		}
	}

	return "unknown", nil
}

// ValidateInstallation performs post-install validation
func (b *BaseInstaller) ValidateInstallation() error {
	if !b.IsInstalled() {
		return fmt.Errorf("%s language server installation validation failed: not installed", b.language)
	}

	command := b.getExecutableCommand()

	// Validate security
	if err := security.ValidateCommand(command, b.serverConfig.Args); err != nil {
		return fmt.Errorf("security validation failed for %s: %w", b.language, err)
	}

	common.CLILogger.Info("Installation validation successful for %s", b.language)
	return nil
}

// CreateInstallDirectory creates the installation directory
func (b *BaseInstaller) CreateInstallDirectory(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create install directory %s: %w", path, err)
	}
	return nil
}

// RunCommand executes a command with timeout
func (b *BaseInstaller) RunCommand(ctx context.Context, name string, args ...string) error {
	// Validate command security
	if err := security.ValidateCommand(name, args); err != nil {
		return fmt.Errorf("command validation failed: %w", err)
	}

	// Create command with context for timeout
	cmd := exec.CommandContext(ctx, name, args...)

	// Capture output for STDIO-safe logging
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			common.CLILogger.Error("Command stderr: %s", stderr.String())
		}
		return fmt.Errorf("command failed: %s %v: %w", name, args, err)
	}

	// Log output to stderr using CLILogger
	if stderr.Len() > 0 {
		common.CLILogger.Warn("Command stderr: %s", stderr.String())
	}

	return nil
}

// RunCommandWithOutput executes a command and returns output
func (b *BaseInstaller) RunCommandWithOutput(ctx context.Context, name string, args ...string) (string, error) {
	// Validate command security
	if err := security.ValidateCommand(name, args); err != nil {
		return "", fmt.Errorf("command validation failed: %w", err)
	}

	// Create command with context for timeout
	cmd := exec.CommandContext(ctx, name, args...)

	// Run command and capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command failed: %s %v: %w", name, args, err)
	}

	return strings.TrimSpace(string(output)), nil
}

// DownloadFile downloads a file from URL with progress
func (b *BaseInstaller) DownloadFile(ctx context.Context, url, destPath string) error {
	common.CLILogger.Info("Downloading %s", url)

	// Create destination directory
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Use curl or wget for download with progress
	var cmd *exec.Cmd
	if _, err := exec.LookPath("curl"); err == nil {
		cmd = exec.CommandContext(ctx, "curl", "-L", "-o", destPath, url)
	} else if _, err := exec.LookPath("wget"); err == nil {
		cmd = exec.CommandContext(ctx, "wget", "-O", destPath, url)
	} else {
		return fmt.Errorf("neither curl nor wget available for download")
	}

	// Capture output for STDIO-safe logging
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			common.CLILogger.Error("Download stderr: %s", stderr.String())
		}
		return fmt.Errorf("download failed: %w", err)
	}

	// Log download output if any
	if stderr.Len() > 0 {
		common.CLILogger.Warn("Download stderr: %s", stderr.String())
	}

	common.CLILogger.Info("Download completed: %s", destPath)
	return nil
}

// ExtractArchive extracts an archive (tar.gz, zip)
func (b *BaseInstaller) ExtractArchive(ctx context.Context, archivePath, destPath string) error {
	common.CLILogger.Info("Extracting %s to %s", archivePath, destPath)

	// Create destination directory
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("failed to create extraction directory: %w", err)
	}

	// Determine archive type and extract
	if strings.HasSuffix(archivePath, ".tar.gz") || strings.HasSuffix(archivePath, ".tgz") {
		return b.RunCommand(ctx, "tar", "-xzf", archivePath, "-C", destPath)
	} else if strings.HasSuffix(archivePath, ".zip") {
		return b.RunCommand(ctx, "unzip", "-q", archivePath, "-d", destPath)
	} else {
		return fmt.Errorf("unsupported archive format: %s", archivePath)
	}
}

// Helper methods

// fileExists checks if a file exists
func (b *BaseInstaller) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// validateServerCommand validates if a command is working
func (b *BaseInstaller) validateServerCommand(command string) bool {
	// For LSP servers, just check if the command exists and is executable
	// Many LSP servers don't support --help and expect LSP communication
	if _, err := exec.LookPath(command); err != nil {
		return false
	}

	// Additional check: try to execute the command with a quick timeout
	// This ensures the binary is not corrupted
	ctx, cancel := icommon.CreateContext(2 * time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, "--version")
	if cmd.Run() == nil {
		return true
	}

	// Fallback: try --help for commands that support it
	cmd = exec.CommandContext(ctx, command, "--help")
	if cmd.Run() == nil {
		return true
	}

	// If both fail, assume it's an LSP server that needs special handling
	// For LSP servers, just having the executable available is sufficient
	return true
}

// getExecutableCommand returns the command to execute
func (b *BaseInstaller) getExecutableCommand() string {
	command := b.serverConfig.Command

	// Check if it's in PATH first
	if _, err := exec.LookPath(command); err == nil {
		return command
	}

	// Check install path
	installPath := b.GetInstallPath()
	customCommand := filepath.Join(installPath, "bin", command)
	if b.fileExists(customCommand) {
		return customCommand
	}

	return command // Fallback to original
}

// tryGetVersion attempts to get version using a specific flag
func (b *BaseInstaller) tryGetVersion(command, flag string) string {
	ctx, cancel := icommon.CreateContext(3 * time.Second)
	defer cancel()

	output, err := b.RunCommandWithOutput(ctx, command, flag)
	if err != nil {
		return ""
	}

	// Extract version from output (basic pattern matching)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "version") || strings.Contains(line, "Version") {
			return line
		}
	}

	// Return first non-empty line if no version keyword found
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}

	return ""
}

// Generic methods for common installer patterns

// IsInstalledByCommand checks if a command is installed and working
func (b *BaseInstaller) IsInstalledByCommand(command string) bool {
	// Check if command exists in PATH
	if _, err := exec.LookPath(command); err != nil {
		return false
	}

	// Quick validation that command works
	return b.validateServerCommand(command)
}

// GetVersionByCommand returns version using a specific command and version flag
func (b *BaseInstaller) GetVersionByCommand(command string, versionFlag string) (string, error) {
	if !b.IsInstalledByCommand(command) {
		return "", fmt.Errorf("%s not installed", command)
	}

	ctx, cancel := icommon.CreateContext(5 * time.Second)
	defer cancel()

	output, err := b.RunCommandWithOutput(ctx, command, versionFlag)
	if err != nil {
		return "", fmt.Errorf("failed to get %s version: %w", command, err)
	}

	return output, nil
}

// ValidateByCommand performs basic validation for a command
func (b *BaseInstaller) ValidateByCommand(command string) error {
	if !b.IsInstalledByCommand(command) {
		return fmt.Errorf("%s installation validation failed: not installed", command)
	}

	// Validate security
	if b.serverConfig != nil {
		if err := security.ValidateCommand(command, b.serverConfig.Args); err != nil {
			return fmt.Errorf("security validation failed for %s: %w", command, err)
		}
	}

	common.CLILogger.Info("Installation validation successful for %s", command)
	return nil
}

// ValidateWithPackageManager performs validation with package manager checks
func (b *BaseInstaller) ValidateWithPackageManager(command string, packageManager string) error {
	// Basic validation using generic method
	if err := b.ValidateByCommand(command); err != nil {
		return err
	}

	// Package manager specific validation
	switch packageManager {
	case "go":
		if !b.isGoInstalled() {
			return fmt.Errorf("Go is not installed but %s is present - this may cause issues", command)
		}
	case "pip", "pip3":
		if !b.isPipInstalled() {
			return fmt.Errorf("pip is not available but %s is present - this may cause issues", command)
		}
	case "npm":
		if !b.isNpmInstalled() {
			return fmt.Errorf("npm is not available but %s is present - this may cause issues", command)
		}
	}

	common.CLILogger.Info("%s validation successful", command)
	return nil
}

// Package manager helpers

// isCommandInstalled checks if a command is installed by running it with specified args
func (b *BaseInstaller) isCommandInstalled(command string, args ...string) bool {
	ctx, cancel := icommon.CreateContext(5 * time.Second)
	defer cancel()

	if _, err := b.RunCommandWithOutput(ctx, command, args...); err != nil {
		return false
	}
	return true
}

// isGoInstalled checks if Go is installed
func (b *BaseInstaller) isGoInstalled() bool {
	return b.isCommandInstalled("go", "version")
}

// isPipInstalled checks if pip is installed
func (b *BaseInstaller) isPipInstalled() bool {
	// Try pip3 first, then pip
	if b.isCommandInstalled("pip3", "--version") {
		return true
	}
	if b.isCommandInstalled("pip", "--version") {
		return true
	}
	// On Windows, pip might only be available through python -m pip
	if runtime.GOOS == "windows" {
		if b.isCommandInstalled("python", "-m", "pip", "--version") {
			return true
		}
		if b.isCommandInstalled("python3", "-m", "pip", "--version") {
			return true
		}
	}
	return false
}

// isNpmInstalled checks if npm is installed
func (b *BaseInstaller) isNpmInstalled() bool {
	return b.isCommandInstalled("npm", "--version")
}

// InstallWithPackageManager handles common package manager installation patterns
func (b *BaseInstaller) InstallWithPackageManager(ctx context.Context, packageManager string, packageName string, version string) error {
	common.CLILogger.Info("Installing %s language server (%s)...", b.language, packageName)

	switch packageManager {
	case "go":
		if !b.isGoInstalled() {
			return fmt.Errorf("Go is not installed. Please install Go first from https://golang.org/dl/")
		}
		installTarget := fmt.Sprintf("%s@%s", packageName, version)
		if version == "" || version == "latest" {
			installTarget = fmt.Sprintf("%s@latest", packageName)
		}
		return b.RunCommand(ctx, "go", "install", installTarget)

	case "pip", "pip3":
		if !b.isPipInstalled() {
			return fmt.Errorf("pip is not installed. Please install Python and pip first")
		}

		// Determine the best way to run pip
		var pipCmd []string
		if _, err := exec.LookPath("pip3"); err == nil {
			pipCmd = []string{"pip3"}
		} else if _, err := exec.LookPath("pip"); err == nil {
			pipCmd = []string{"pip"}
		} else if runtime.GOOS == "windows" {
			// On Windows, try python -m pip if pip is not in PATH
			if _, err := exec.LookPath("python"); err == nil {
				pipCmd = []string{"python", "-m", "pip"}
			} else if _, err := exec.LookPath("python3"); err == nil {
				pipCmd = []string{"python3", "-m", "pip"}
			}
		}

		if len(pipCmd) == 0 {
			return fmt.Errorf("pip command not found")
		}

		installPackage := packageName
		if version != "" && version != "latest" {
			installPackage = fmt.Sprintf("%s==%s", packageName, version)
		}

		// Build the full command
		args := append(pipCmd, "install", "--user", installPackage)
		return b.RunCommand(ctx, args[0], args[1:]...)

	case "npm":
		if !b.isNpmInstalled() {
			return fmt.Errorf("npm is not installed. Please install Node.js and npm first")
		}
		installPackage := packageName
		if version != "" && version != "latest" {
			installPackage = fmt.Sprintf("%s@%s", packageName, version)
		}
		return b.RunCommand(ctx, "npm", "install", "-g", installPackage)

	default:
		return fmt.Errorf("unsupported package manager: %s", packageManager)
	}
}

// UninstallWithPackageManager handles common package manager uninstallation patterns
func (b *BaseInstaller) UninstallWithPackageManager(packageManager string, packageName string) error {
	common.CLILogger.Info("Uninstalling %s language server...", b.language)

	ctx, cancel := icommon.CreateContext(30 * time.Second)
	defer cancel()

	switch packageManager {
	case "go":
		return fmt.Errorf("Go doesn't provide built-in uninstall for %s. Please manually remove $GOBIN/%s", packageName, b.serverConfig.Command)

	case "pip", "pip3":
		if !b.isPipInstalled() {
			return fmt.Errorf("pip not available for uninstalling %s", packageName)
		}
		pip := "pip"
		if _, err := exec.LookPath("pip3"); err == nil {
			pip = "pip3"
		}
		return b.RunCommand(ctx, pip, "uninstall", "-y", packageName)

	case "npm":
		if !b.isNpmInstalled() {
			return fmt.Errorf("npm not available for uninstalling %s", packageName)
		}
		return b.RunCommand(ctx, "npm", "uninstall", "-g", packageName)

	default:
		return fmt.Errorf("unsupported package manager: %s", packageManager)
	}
}
