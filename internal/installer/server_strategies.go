package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/types"
)

// JDTLS Configuration
const (
	JDTLSVersion        = "1.48.0"
	JDTLSTimestamp      = "202506271502"
	JDTLSChecksum       = "b0a7fa1240e2caf1296d59ea709c525d4a631fbefda49e7182e07e00c1de62c9"
	JDTLSDownloadURL    = "http://download.eclipse.org/jdtls/milestones/" + JDTLSVersion + "/jdt-language-server-" + JDTLSVersion + "-" + JDTLSTimestamp + ".tar.gz"
	JDTLSInstallDir     = "jdtls"
	JDTLSExecutableName = "jdtls"
)

// getJDTLSInstallPath returns the installation path for jdtls based on the platform
func getJDTLSInstallPath() string {
	switch runtime.GOOS {
	case string(platform.PlatformWindows):
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "lsp-gateway", JDTLSInstallDir)
		}
		return filepath.Join(os.Getenv("USERPROFILE"), ".lsp-gateway", JDTLSInstallDir)
	case "darwin":
		home := os.Getenv("HOME")
		return filepath.Join(home, "Library", "Application Support", "lsp-gateway", JDTLSInstallDir)
	default: // linux and others
		if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
			return filepath.Join(xdgDataHome, "lsp-gateway", JDTLSInstallDir)
		}
		home := os.Getenv("HOME")
		return filepath.Join(home, ".local", "share", "lsp-gateway", JDTLSInstallDir)
	}
}

// getJDTLSExecutablePath returns the path to the jdtls executable script
func getJDTLSExecutablePath() string {
	installPath := getJDTLSInstallPath()
	if runtime.GOOS == "windows" {
		return filepath.Join(installPath, "bin", "jdtls.bat")
	}
	return filepath.Join(installPath, "bin", "jdtls")
}

// getJDTLSVerificationCommands returns the platform-specific verification commands for JDTLS
func getJDTLSVerificationCommands() []string {
	executablePath := getJDTLSExecutablePath()
	return []string{executablePath, "--version"}
}

// Server Strategy Wrappers - these implement the ServerPlatformStrategy interface

// UniversalServerStrategy provides cross-platform server installation
type UniversalServerStrategy struct {
	executor   platform.CommandExecutor
	downloader *FileDownloader
}

// NewUniversalServerStrategy creates a new universal server strategy
func NewUniversalServerStrategy() *UniversalServerStrategy {
	return &UniversalServerStrategy{
		executor:   platform.NewCommandExecutor(),
		downloader: NewFileDownloader(),
	}
}

// InstallServer installs the specified server
func (u *UniversalServerStrategy) InstallServer(server string, options types.ServerInstallOptions) (*types.InstallResult, error) {
	start := time.Now()

	switch server {
	case ServerJDTLS:
		return u.installJDTLS(options, start)
	case ServerGopls:
		return u.installGopls(options, start)
	case ServerPylsp:
		return u.installPylsp(options, start)
	case ServerTypeScriptLanguageServer:
		return u.installTypescriptLanguageServer(options, start)
	default:
		return &types.InstallResult{
			Success:  false,
			Runtime:  server,
			Duration: time.Since(start),
			Errors:   []string{fmt.Sprintf("Unsupported server: %s", server)},
		}, fmt.Errorf("unsupported server: %s", server)
	}
}

// installJDTLS installs Eclipse JDT Language Server
func (u *UniversalServerStrategy) installJDTLS(options types.ServerInstallOptions, start time.Time) (*types.InstallResult, error) {
	installPath := getJDTLSInstallPath()
	executablePath := getJDTLSExecutablePath()

	// Check if already installed and not forcing reinstall
	if !options.Force {
		if _, err := os.Stat(executablePath); err == nil {
			return &types.InstallResult{
				Success:  true,
				Runtime:  ServerJDTLS,
				Version:  JDTLSVersion,
				Path:     executablePath,
				Method:   "already_installed",
				Duration: time.Since(start),
				Messages: []string{"JDTLS already installed"},
			}, nil
		}
	}

	// Create temporary directory for download
	tempDir, err := os.MkdirTemp("", "jdtls-install-*")
	if err != nil {
		return &types.InstallResult{
			Success:  false,
			Runtime:  ServerJDTLS,
			Duration: time.Since(start),
			Errors:   []string{fmt.Sprintf("Failed to create temp directory: %v", err)},
		}, err
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Download jdtls archive
	archivePath := filepath.Join(tempDir, "jdtls.tar.gz")

	downloadOptions := DownloadOptions{
		URL:              JDTLSDownloadURL,
		OutputPath:       archivePath,
		ExpectedChecksum: JDTLSChecksum,
		Timeout:          10 * time.Minute,
		MaxRetries:       3,
		ProgressCallback: func(downloaded, total int64) {
			// Could add progress reporting here if needed
		},
	}

	downloadResult := u.downloader.Download(downloadOptions)
	if !downloadResult.Success {
		return &types.InstallResult{
			Success:  false,
			Runtime:  ServerJDTLS,
			Duration: time.Since(start),
			Errors:   []string{fmt.Sprintf("Download failed: %v", downloadResult.Error)},
		}, downloadResult.Error
	}

	// Remove existing installation if it exists
	if err := os.RemoveAll(installPath); err != nil && !os.IsNotExist(err) {
		return &types.InstallResult{
			Success:  false,
			Runtime:  ServerJDTLS,
			Duration: time.Since(start),
			Errors:   []string{fmt.Sprintf("Failed to remove existing installation: %v", err)},
		}, err
	}

	// Extract archive
	if err := ExtractTarGz(archivePath, installPath); err != nil {
		return &types.InstallResult{
			Success:  false,
			Runtime:  ServerJDTLS,
			Duration: time.Since(start),
			Errors:   []string{fmt.Sprintf("Failed to extract archive: %v", err)},
		}, err
	}

	// Create executable script
	if err := u.createJDTLSScript(installPath); err != nil {
		return &types.InstallResult{
			Success:  false,
			Runtime:  ServerJDTLS,
			Duration: time.Since(start),
			Errors:   []string{fmt.Sprintf("Failed to create executable script: %v", err)},
		}, err
	}

	// Verify installation
	if _, err := os.Stat(executablePath); err != nil {
		return &types.InstallResult{
			Success:  false,
			Runtime:  ServerJDTLS,
			Duration: time.Since(start),
			Errors:   []string{fmt.Sprintf("Installation verification failed: %v", err)},
		}, err
	}

	return &types.InstallResult{
		Success:  true,
		Runtime:  ServerJDTLS,
		Version:  JDTLSVersion,
		Path:     executablePath,
		Method:   "automated_download",
		Duration: time.Since(start),
		Messages: []string{
			fmt.Sprintf("Successfully downloaded and installed JDTLS %s", JDTLSVersion),
			fmt.Sprintf("Installation path: %s", installPath),
			fmt.Sprintf("Executable: %s", executablePath),
		},
		Details: map[string]interface{}{
			"download_url":  JDTLSDownloadURL,
			"checksum":      JDTLSChecksum,
			"install_path":  installPath,
			"executable":    executablePath,
			"download_size": downloadResult.FileSize,
			"verified":      downloadResult.Verified,
		},
	}, nil
}

// createJDTLSScript creates the executable script for jdtls
func (u *UniversalServerStrategy) createJDTLSScript(installPath string) error {
	binDir := filepath.Join(installPath, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Find the main jar file
	jarPath, err := u.findJDTLSJar(installPath)
	if err != nil {
		return fmt.Errorf("failed to find JDTLS jar: %w", err)
	}

	if runtime.GOOS == "windows" {
		return u.createWindowsJDTLSScript(binDir, jarPath)
	}
	return u.createUnixJDTLSScript(binDir, jarPath)
}

// findJDTLSJar finds the main JDTLS jar file
func (u *UniversalServerStrategy) findJDTLSJar(installPath string) (string, error) {
	pluginsDir := filepath.Join(installPath, "plugins")

	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return "", fmt.Errorf("failed to read plugins directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.HasPrefix(name, "org.eclipse.jdt.ls.core_") && strings.HasSuffix(name, ".jar") {
			return filepath.Join(pluginsDir, name), nil
		}
	}

	return "", fmt.Errorf("JDTLS core jar not found in plugins directory")
}

// createWindowsJDTLSScript creates a Windows batch script for jdtls
func (u *UniversalServerStrategy) createWindowsJDTLSScript(binDir, jarPath string) error {
	scriptPath := filepath.Join(binDir, "jdtls.bat")

	// Convert paths to Windows format
	jarPath = strings.ReplaceAll(jarPath, "/", "\\")
	configDir := strings.ReplaceAll(filepath.Join(filepath.Dir(jarPath), "..", "config_win"), "/", "\\")

	scriptContent := fmt.Sprintf(`@echo off
setlocal enabledelayedexpansion

REM Eclipse JDT Language Server Launcher Script
REM Generated by lsp-gateway installer

if "%%JAVA_HOME%%" == "" (
    set JAVA_CMD=java
) else (
    set JAVA_CMD="%%JAVA_HOME%%\bin\java"
)

REM Set workspace directory
if "%%1" == "" (
    set WORKSPACE=%%cd%%
) else (
    set WORKSPACE=%%1
)

REM Launch JDTLS
%%JAVA_CMD%% ^
    -Declipse.application=org.eclipse.jdt.ls.core.id1 ^
    -Dosgi.bundles.defaultStartLevel=4 ^
    -Declipse.product=org.eclipse.jdt.ls.core.product ^
    -Dlog.level=ALL ^
    -Xmx1G ^
    --add-modules=ALL-SYSTEM ^
    --add-opens java.base/java.util=ALL-UNNAMED ^
    --add-opens java.base/java.lang=ALL-UNNAMED ^
    -jar "%s" ^
    -configuration "%s" ^
    -data "%%WORKSPACE%%" ^
    %%*
`, jarPath, configDir)

	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		return fmt.Errorf("failed to write Windows script: %w", err)
	}

	return nil
}

// createUnixJDTLSScript creates a Unix shell script for jdtls
func (u *UniversalServerStrategy) createUnixJDTLSScript(binDir, jarPath string) error {
	scriptPath := filepath.Join(binDir, "jdtls")

	var configDir string
	switch runtime.GOOS {
	case "darwin":
		configDir = filepath.Join(filepath.Dir(jarPath), "..", "config_mac")
	default:
		configDir = filepath.Join(filepath.Dir(jarPath), "..", "config_linux")
	}

	scriptContent := fmt.Sprintf(`#!/bin/bash

# Eclipse JDT Language Server Launcher Script
# Generated by lsp-gateway installer

if [ -n "$JAVA_HOME" ]; then
    JAVA_CMD="$JAVA_HOME/bin/java"
else
    JAVA_CMD="java"
fi

# Set workspace directory
if [ -z "$1" ]; then
    WORKSPACE="$(pwd)"
else
    WORKSPACE="$1"
    shift
fi

# Launch JDTLS
exec "$JAVA_CMD" \
    -Declipse.application=org.eclipse.jdt.ls.core.id1 \
    -Dosgi.bundles.defaultStartLevel=4 \
    -Declipse.product=org.eclipse.jdt.ls.core.product \
    -Dlog.level=ALL \
    -Xmx1G \
    --add-modules=ALL-SYSTEM \
    --add-opens java.base/java.util=ALL-UNNAMED \
    --add-opens java.base/java.lang=ALL-UNNAMED \
    -jar "%s" \
    -configuration "%s" \
    -data "$WORKSPACE" \
    "$@"
`, jarPath, configDir)

	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		return fmt.Errorf("failed to write Unix script: %w", err)
	}

	return nil
}

// installGopls installs the Go language server using go install
func (u *UniversalServerStrategy) installGopls(options types.ServerInstallOptions, start time.Time) (*types.InstallResult, error) {
	cmd := []string{"go", "install", "golang.org/x/tools/gopls@latest"}

	result, err := u.executor.Execute(cmd[0], cmd[1:], 5*time.Minute)
	if err != nil {
		return &types.InstallResult{
			Success:  false,
			Runtime:  ServerGopls,
			Duration: time.Since(start),
			Errors:   []string{fmt.Sprintf("Installation failed: %v", err)},
		}, err
	}

	if result.ExitCode != 0 {
		return &types.InstallResult{
			Success:  false,
			Runtime:  ServerGopls,
			Duration: time.Since(start),
			Errors:   []string{fmt.Sprintf("Installation failed with exit code %d: %s", result.ExitCode, result.Stderr)},
		}, fmt.Errorf("gopls installation failed")
	}

	return &types.InstallResult{
		Success:  true,
		Runtime:  ServerGopls,
		Method:   "go_install",
		Duration: time.Since(start),
		Messages: []string{"Successfully installed gopls"},
	}, nil
}

// installPylsp installs the Python language server using pip
func (u *UniversalServerStrategy) installPylsp(options types.ServerInstallOptions, start time.Time) (*types.InstallResult, error) {
	cmd := []string{"pip", "install", "python-lsp-server"}

	result, err := u.executor.Execute(cmd[0], cmd[1:], 5*time.Minute)
	if err != nil {
		return &types.InstallResult{
			Success:  false,
			Runtime:  ServerPylsp,
			Duration: time.Since(start),
			Errors:   []string{fmt.Sprintf("Installation failed: %v", err)},
		}, err
	}

	if result.ExitCode != 0 {
		return &types.InstallResult{
			Success:  false,
			Runtime:  ServerPylsp,
			Duration: time.Since(start),
			Errors:   []string{fmt.Sprintf("Installation failed with exit code %d: %s", result.ExitCode, result.Stderr)},
		}, fmt.Errorf("pylsp installation failed")
	}

	return &types.InstallResult{
		Success:  true,
		Runtime:  ServerPylsp,
		Method:   "pip_install",
		Duration: time.Since(start),
		Messages: []string{"Successfully installed python-lsp-server"},
	}, nil
}

// installTypescriptLanguageServer installs the TypeScript language server using npm
func (u *UniversalServerStrategy) installTypescriptLanguageServer(options types.ServerInstallOptions, start time.Time) (*types.InstallResult, error) {
	cmd := []string{"npm", "install", "-g", "typescript-language-server", "typescript"}

	result, err := u.executor.Execute(cmd[0], cmd[1:], 5*time.Minute)
	if err != nil {
		return &types.InstallResult{
			Success:  false,
			Runtime:  ServerTypeScriptLanguageServer,
			Duration: time.Since(start),
			Errors:   []string{fmt.Sprintf("Installation failed: %v", err)},
		}, err
	}

	if result.ExitCode != 0 {
		return &types.InstallResult{
			Success:  false,
			Runtime:  ServerTypeScriptLanguageServer,
			Duration: time.Since(start),
			Errors:   []string{fmt.Sprintf("Installation failed with exit code %d: %s", result.ExitCode, result.Stderr)},
		}, fmt.Errorf("typescript-language-server installation failed")
	}

	return &types.InstallResult{
		Success:  true,
		Runtime:  ServerTypeScriptLanguageServer,
		Method:   "npm_global",
		Duration: time.Since(start),
		Messages: []string{"Successfully installed typescript-language-server"},
	}, nil
}

// VerifyServer verifies that the specified server is installed and functional
func (u *UniversalServerStrategy) VerifyServer(server string) (*types.VerificationResult, error) {
	switch server {
	case ServerJDTLS:
		return u.verifyJDTLS()
	case ServerGopls:
		return u.verifyGopls()
	case ServerPylsp:
		return u.verifyPylsp()
	case ServerTypeScriptLanguageServer:
		return u.verifyTypescriptLanguageServer()
	default:
		return &types.VerificationResult{
			Installed: false,
			Runtime:   server,
			Issues:    []types.Issue{{Severity: types.IssueSeverityHigh, Title: "Unsupported server", Description: fmt.Sprintf("Server %s is not supported", server)}},
		}, fmt.Errorf("unsupported server: %s", server)
	}
}

// verifyJDTLS verifies JDTLS installation
func (u *UniversalServerStrategy) verifyJDTLS() (*types.VerificationResult, error) {
	executablePath := getJDTLSExecutablePath()

	result := &types.VerificationResult{
		Runtime: ServerJDTLS,
		Issues:  []types.Issue{},
		Details: make(map[string]interface{}),
	}

	// Check if executable exists
	if _, err := os.Stat(executablePath); err != nil {
		result.Issues = append(result.Issues, types.Issue{
			Severity:    types.IssueSeverityHigh,
			Title:       "JDTLS executable not found",
			Description: fmt.Sprintf("JDTLS executable not found at %s", executablePath),
		})
		return result, nil
	}

	result.Installed = true
	result.Path = executablePath
	result.Version = JDTLSVersion
	result.Compatible = true
	result.Details["install_path"] = getJDTLSInstallPath()
	result.Details["executable"] = executablePath

	return result, nil
}

// verifyGopls verifies gopls installation
func (u *UniversalServerStrategy) verifyGopls() (*types.VerificationResult, error) {
	result := &types.VerificationResult{
		Runtime: ServerGopls,
		Issues:  []types.Issue{},
	}

	if !u.executor.IsCommandAvailable("gopls") {
		result.Issues = append(result.Issues, types.Issue{
			Severity:    types.IssueSeverityHigh,
			Title:       "gopls not found",
			Description: "gopls command is not available in PATH",
		})
		return result, nil
	}

	result.Installed = true
	result.Compatible = true
	return result, nil
}

// verifyPylsp verifies pylsp installation
func (u *UniversalServerStrategy) verifyPylsp() (*types.VerificationResult, error) {
	result := &types.VerificationResult{
		Runtime: ServerPylsp,
		Issues:  []types.Issue{},
	}

	if !u.executor.IsCommandAvailable("pylsp") {
		result.Issues = append(result.Issues, types.Issue{
			Severity:    types.IssueSeverityHigh,
			Title:       "pylsp not found",
			Description: "pylsp command is not available in PATH",
		})
		return result, nil
	}

	result.Installed = true
	result.Compatible = true
	return result, nil
}

// verifyTypescriptLanguageServer verifies typescript-language-server installation
func (u *UniversalServerStrategy) verifyTypescriptLanguageServer() (*types.VerificationResult, error) {
	result := &types.VerificationResult{
		Runtime: ServerTypeScriptLanguageServer,
		Issues:  []types.Issue{},
	}

	if !u.executor.IsCommandAvailable("typescript-language-server") {
		result.Issues = append(result.Issues, types.Issue{
			Severity:    types.IssueSeverityHigh,
			Title:       "typescript-language-server not found",
			Description: "typescript-language-server command is not available in PATH",
		})
		return result, nil
	}

	result.Installed = true
	result.Compatible = true
	return result, nil
}

// GetInstallCommand returns the command that would be used to install the server
func (u *UniversalServerStrategy) GetInstallCommand(server, version string) ([]string, error) {
	switch server {
	case ServerJDTLS:
		return []string{"# Automated download and installation from Eclipse releases"}, nil
	case ServerGopls:
		return []string{"go", "install", "golang.org/x/tools/gopls@latest"}, nil
	case ServerPylsp:
		return []string{"pip", "install", "python-lsp-server"}, nil
	case ServerTypeScriptLanguageServer:
		return []string{"npm", "install", "-g", "typescript-language-server", "typescript"}, nil
	default:
		return []string{}, fmt.Errorf("unsupported server: %s", server)
	}
}
