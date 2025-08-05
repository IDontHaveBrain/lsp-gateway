package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/security"
)

// JavaInstaller handles Java language server (jdtls) and JDK installation
type JavaInstaller struct {
	*BaseInstaller
	jdkPath string
}

// NewJavaInstaller creates a new Java installer
func NewJavaInstaller(platform PlatformInfo) *JavaInstaller {
	serverConfig := &config.ServerConfig{
		Command: "jdtls",
		Args:    []string{},
	}

	base := NewBaseInstaller("java", serverConfig, platform)

	return &JavaInstaller{
		BaseInstaller: base,
	}
}

// Install installs JDK and Eclipse JDT Language Server
func (j *JavaInstaller) Install(ctx context.Context, options InstallOptions) error {
	common.CLILogger.Info("Installing Java development environment...")

	installPath := j.GetInstallPath()
	if options.InstallPath != "" {
		installPath = options.InstallPath
		j.SetInstallPath(installPath) // Update the installer's install path
	}

	// Create install directory
	if err := j.CreateInstallDirectory(installPath); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	// Step 1: Install JDK
	jdkPath := filepath.Join(installPath, "jdk")
	if err := j.installJDK(ctx, jdkPath); err != nil {
		return fmt.Errorf("failed to install JDK: %w", err)
	}
	j.jdkPath = jdkPath

	// Step 2: Install Eclipse JDT Language Server
	jdtlsPath := filepath.Join(installPath, "jdtls")
	if err := j.installJDTLS(ctx, jdtlsPath); err != nil {
		return fmt.Errorf("failed to install JDTLS: %w", err)
	}

	// Step 3: Create wrapper script
	if err := j.createJDTLSWrapper(installPath, jdkPath, jdtlsPath); err != nil {
		return fmt.Errorf("failed to create JDTLS wrapper: %w", err)
	}

	common.CLILogger.Info("Java development environment installation completed")
	return nil
}

// installJDK downloads and installs JDK
func (j *JavaInstaller) installJDK(ctx context.Context, jdkPath string) error {
	common.CLILogger.Info("Installing JDK...")

	// Get download URL for current platform
	downloadURL, extractDir, err := j.platform.GetJavaDownloadURL("17")
	if err != nil {
		return fmt.Errorf("failed to get JDK download URL: %w", err)
	}

	// Download JDK with proper extension
	var archivePath string
	if strings.Contains(downloadURL, ".tar.gz") {
		archivePath = filepath.Join(jdkPath, "jdk-download.tar.gz")
	} else if strings.Contains(downloadURL, ".zip") {
		archivePath = filepath.Join(jdkPath, "jdk-download.zip")
	} else {
		archivePath = filepath.Join(jdkPath, "jdk-download.tar.gz") // Default to tar.gz
	}

	if err := j.DownloadFile(ctx, downloadURL, archivePath); err != nil {
		return fmt.Errorf("failed to download JDK: %w", err)
	}

	// Extract JDK
	extractPath := filepath.Join(jdkPath, "extracted")
	if err := j.ExtractArchive(ctx, archivePath, extractPath); err != nil {
		return fmt.Errorf("failed to extract JDK: %w", err)
	}

	// Move extracted JDK to final location
	finalJDKPath := filepath.Join(jdkPath, "current")
	extractedJDKPath := filepath.Join(extractPath, extractDir)

	if err := os.Rename(extractedJDKPath, finalJDKPath); err != nil {
		return fmt.Errorf("failed to move JDK to final location: %w", err)
	}

	// Clean up temporary files
	os.RemoveAll(archivePath)
	os.RemoveAll(extractPath)

	common.CLILogger.Info("JDK installation completed at %s", finalJDKPath)
	return nil
}

// installJDTLS downloads and installs Eclipse JDT Language Server
func (j *JavaInstaller) installJDTLS(ctx context.Context, jdtlsPath string) error {
	common.CLILogger.Info("Installing Eclipse JDT Language Server...")

	// Get latest JDTLS release URL
	jdtlsURL := "https://download.eclipse.org/jdtls/snapshots/jdt-language-server-latest.tar.gz"

	// Download JDTLS
	archivePath := filepath.Join(jdtlsPath, "jdtls-download.tar.gz")
	if err := j.DownloadFile(ctx, jdtlsURL, archivePath); err != nil {
		return fmt.Errorf("failed to download JDTLS: %w", err)
	}

	// Extract JDTLS
	if err := j.ExtractArchive(ctx, archivePath, jdtlsPath); err != nil {
		return fmt.Errorf("failed to extract JDTLS: %w", err)
	}

	// Clean up archive
	os.Remove(archivePath)

	common.CLILogger.Info("Eclipse JDT Language Server installation completed at %s", jdtlsPath)
	return nil
}

// createJDTLSWrapper creates a wrapper script to run JDTLS with custom JDK
func (j *JavaInstaller) createJDTLSWrapper(installPath, jdkPath, jdtlsPath string) error {
	common.CLILogger.Info("Creating JDTLS wrapper script...")

	binDir := filepath.Join(installPath, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	platform := j.platform.GetPlatform()
	var wrapperPath string
	var wrapperContent string

	if platform == "windows" {
		wrapperPath = filepath.Join(binDir, "jdtls.bat")
		wrapperContent = j.createWindowsWrapper(jdkPath, jdtlsPath)
	} else {
		wrapperPath = filepath.Join(binDir, "jdtls")
		wrapperContent = j.createUnixWrapper(jdkPath, jdtlsPath)
	}

	// Write wrapper script
	if err := os.WriteFile(wrapperPath, []byte(wrapperContent), 0755); err != nil {
		return fmt.Errorf("failed to write wrapper script: %w", err)
	}

	common.CLILogger.Info("JDTLS wrapper created at %s", wrapperPath)

	// Ensure file is synced to disk on Windows
	if j.platform.GetPlatform() == "windows" {
		// Small delay to ensure file system sync on Windows
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

// createUnixWrapper creates a Unix shell wrapper script
func (j *JavaInstaller) createUnixWrapper(jdkPath, jdtlsPath string) string {
	javaExe := filepath.Join(jdkPath, "current", "bin", "java")
	jdtlsJar := filepath.Join(jdtlsPath, "plugins", "org.eclipse.equinox.launcher_*.jar")
	configDir := filepath.Join(jdtlsPath, "config_linux")

	// Adjust config directory for macOS
	if j.platform.GetPlatform() == "darwin" {
		configDir = filepath.Join(jdtlsPath, "config_mac")
	}

	return fmt.Sprintf(`#!/bin/bash
# JDTLS wrapper script with custom JDK
# Generated by LSP Gateway installer

JAVA_HOME="%s/current"
JAVA_EXE="%s"
JDTLS_JAR="%s"
CONFIG_DIR="%s"

# Use custom workspace if not specified
if [ -z "$1" ]; then
    WORKSPACE="$HOME/.lsp-gateway/workspace"
else
    WORKSPACE="$1"
fi

# Ensure Java executable exists
if [ ! -f "$JAVA_EXE" ]; then
    echo "Error: Java executable not found at $JAVA_EXE"
    exit 1
fi

# Find the actual launcher jar
LAUNCHER_JAR=$(ls ${JDTLS_JAR} 2>/dev/null | head -n1)
if [ ! -f "$LAUNCHER_JAR" ]; then
    echo "Error: JDTLS launcher jar not found at $JDTLS_JAR"
    exit 1
fi

# Run JDTLS
exec "$JAVA_EXE" \
    -Declipse.application=org.eclipse.jdt.ls.core.id1 \
    -Dosgi.bundles.defaultStartLevel=4 \
    -Declipse.product=org.eclipse.jdt.ls.core.product \
    -Dlog.level=ALL \
    -noverify \
    -Xmx1G \
    -jar "$LAUNCHER_JAR" \
    -configuration "$CONFIG_DIR" \
    -data "$WORKSPACE" \
    --add-modules=ALL-SYSTEM \
    --add-opens java.base/java.util=ALL-UNNAMED \
    --add-opens java.base/java.lang=ALL-UNNAMED
`, jdkPath, javaExe, jdtlsJar, configDir)
}

// createWindowsWrapper creates a Windows batch wrapper script
func (j *JavaInstaller) createWindowsWrapper(jdkPath, jdtlsPath string) string {
	javaExe := filepath.Join(jdkPath, "current", "bin", "java.exe")
	jdtlsPluginsDir := filepath.Join(jdtlsPath, "plugins")
	configDir := filepath.Join(jdtlsPath, "config_win")

	return fmt.Sprintf(`@echo off
REM JDTLS wrapper script with custom JDK
REM Generated by LSP Gateway installer

set JAVA_HOME=%s\current
set JAVA_EXE=%s
set PLUGINS_DIR=%s
set CONFIG_DIR=%s

REM Use custom workspace if not specified
if "%%1"=="" (
    set WORKSPACE=%%USERPROFILE%%\.lsp-gateway\workspace
) else (
    set WORKSPACE=%%1
)

REM Check Java executable
if not exist "%%JAVA_EXE%%" (
    echo Error: Java executable not found at %%JAVA_EXE%%
    exit /b 1
)

REM Find launcher jar
set LAUNCHER_JAR=
for %%%%f in ("%%PLUGINS_DIR%%\org.eclipse.equinox.launcher_*.jar") do (
    set LAUNCHER_JAR=%%%%f
)

if "%%LAUNCHER_JAR%%"=="" (
    echo Error: JDTLS launcher jar not found in %%PLUGINS_DIR%%
    exit /b 1
)

REM Run JDTLS
"%%JAVA_EXE%%" ^
    -Declipse.application=org.eclipse.jdt.ls.core.id1 ^
    -Dosgi.bundles.defaultStartLevel=4 ^
    -Declipse.product=org.eclipse.jdt.ls.core.product ^
    -Dlog.level=ALL ^
    -noverify ^
    -Xmx1G ^
    -jar "%%LAUNCHER_JAR%%" ^
    -configuration "%%CONFIG_DIR%%" ^
    -data "%%WORKSPACE%%" ^
    --add-modules=ALL-SYSTEM ^
    --add-opens java.base/java.util=ALL-UNNAMED ^
    --add-opens java.base/java.lang=ALL-UNNAMED
`, jdkPath, javaExe, jdtlsPluginsDir, configDir)
}

// Uninstall removes JDK and JDTLS
func (j *JavaInstaller) Uninstall() error {
	installPath := j.GetInstallPath()

	common.CLILogger.Info("Uninstalling Java development environment...")

	if err := os.RemoveAll(installPath); err != nil {
		return fmt.Errorf("failed to remove Java installation: %w", err)
	}

	common.CLILogger.Info("Java development environment uninstalled successfully")
	return nil
}

// GetVersion returns the installed Java version
func (j *JavaInstaller) GetVersion() (string, error) {
	if !j.IsInstalled() {
		return "", fmt.Errorf("Java development environment not installed")
	}

	// Get version from wrapper script
	installPath := j.GetInstallPath()

	// Also check Java version
	jdkPath := filepath.Join(installPath, "jdk", "current")
	javaExe := filepath.Join(jdkPath, "bin", "java")
	if j.platform.GetPlatform() == "windows" {
		javaExe += ".exe"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if javaVersion, err := j.RunCommandWithOutput(ctx, javaExe, "-version"); err == nil {
		return fmt.Sprintf("Java: %s, JDTLS: installed", strings.Split(javaVersion, "\n")[0]), nil
	}

	return "JDTLS installed (version unknown)", nil
}

// IsInstalled checks if Java development environment is installed
func (j *JavaInstaller) IsInstalled() bool {
	installPath := j.GetInstallPath()

	// Check for wrapper script
	binDir := filepath.Join(installPath, "bin")

	var wrapperPath string
	if j.platform.GetPlatform() == "windows" {
		wrapperPath = filepath.Join(binDir, "jdtls.bat")
	} else {
		wrapperPath = filepath.Join(binDir, "jdtls")
	}

	if !j.fileExists(wrapperPath) {
		common.CLILogger.Debug("Java wrapper script not found at: %s", wrapperPath)
		return false
	}

	// Check for JDK
	jdkPath := filepath.Join(installPath, "jdk", "current")
	javaExe := filepath.Join(jdkPath, "bin", "java")
	if j.platform.GetPlatform() == "windows" {
		javaExe += ".exe"
	}

	if !j.fileExists(javaExe) {
		common.CLILogger.Debug("Java executable not found at: %s", javaExe)
		return false
	}

	// Check for JDTLS
	jdtlsPath := filepath.Join(installPath, "jdtls")
	if !j.fileExists(jdtlsPath) {
		common.CLILogger.Debug("JDTLS directory not found at: %s", jdtlsPath)
		return false
	}

	return true
}

// GetServerConfig returns the updated server configuration with custom paths
func (j *JavaInstaller) GetServerConfig() *config.ServerConfig {
	if !j.IsInstalled() {
		return j.serverConfig
	}

	installPath := j.GetInstallPath()
	binDir := filepath.Join(installPath, "bin")

	var command string
	if j.platform.GetPlatform() == "windows" {
		command = filepath.Join(binDir, "jdtls.bat")
	} else {
		command = filepath.Join(binDir, "jdtls")
	}

	// Return updated config with custom command path
	return &config.ServerConfig{
		Command:    command,
		Args:       []string{},
		WorkingDir: "",
	}
}

// ValidateInstallation performs comprehensive validation
func (j *JavaInstaller) ValidateInstallation() error {
	// Check if installed using our custom IsInstalled logic
	if !j.IsInstalled() {
		return fmt.Errorf("java language server installation validation failed: not installed")
	}

	installPath := j.GetInstallPath()

	// Validate JDK installation
	jdkPath := filepath.Join(installPath, "jdk", "current")
	javaExe := filepath.Join(jdkPath, "bin", "java")
	if j.platform.GetPlatform() == "windows" {
		javaExe += ".exe"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := j.RunCommandWithOutput(ctx, javaExe, "-version"); err != nil {
		return fmt.Errorf("JDK validation failed: %w", err)
	}

	// Validate JDTLS installation
	jdtlsPath := filepath.Join(installPath, "jdtls")
	if !j.fileExists(jdtlsPath) {
		return fmt.Errorf("JDTLS installation not found at %s", jdtlsPath)
	}

	// Validate wrapper script security
	binDir := filepath.Join(installPath, "bin")
	var wrapperPath string
	if j.platform.GetPlatform() == "windows" {
		wrapperPath = filepath.Join(binDir, "jdtls.bat")
	} else {
		wrapperPath = filepath.Join(binDir, "jdtls")
	}

	if err := security.ValidateCommand(wrapperPath, []string{}); err != nil {
		return fmt.Errorf("security validation failed for java wrapper: %w", err)
	}

	common.CLILogger.Info("Java development environment validation successful")
	return nil
}
