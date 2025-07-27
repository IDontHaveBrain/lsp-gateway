package installer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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
	JDTLSDownloadURL    = "https://download.eclipse.org/jdtls/milestones/" + JDTLSVersion + "/jdt-language-server-" + JDTLSVersion + "-" + JDTLSTimestamp + ".tar.gz"
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

// GetJDTLSExecutablePath returns the path to the jdtls executable script
func GetJDTLSExecutablePath() string {
	installPath := getJDTLSInstallPath()
	if runtime.GOOS == "windows" {
		return filepath.Join(installPath, "bin", "jdtls.bat")
	}
	return filepath.Join(installPath, "bin", "jdtls")
}

// getJDTLSVerificationCommands returns the platform-specific verification commands for JDTLS
func getJDTLSVerificationCommands() []string {
	executablePath := GetJDTLSExecutablePath()
	// JDTLS doesn't support --version, use --help for basic functionality test
	return []string{executablePath, "--help"}
}

// getGoplsInstallPaths returns potential installation paths for gopls
func getGoplsInstallPaths() []string {
	paths := []string{}
	
	// Try GOBIN first
	if gobin := os.Getenv("GOBIN"); gobin != "" {
		paths = append(paths, filepath.Join(gobin, "gopls"))
	}
	
	// Try GOPATH/bin
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		paths = append(paths, filepath.Join(gopath, "bin", "gopls"))
	}
	
	// Try default GOPATH
	home := os.Getenv("HOME")
	if home != "" {
		defaultGopath := filepath.Join(home, "go")
		paths = append(paths, filepath.Join(defaultGopath, "bin", "gopls"))
	}
	
	// Windows specific paths
	if runtime.GOOS == "windows" {
		if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
			paths = append(paths, filepath.Join(userProfile, "go", "bin", "gopls.exe"))
		}
	}
	
	return paths
}

// getPylspInstallPaths returns potential installation paths for pylsp
func getPylspInstallPaths() []string {
	paths := []string{}
	
	// User-specific pip install paths
	home := os.Getenv("HOME")
	if home != "" {
		// Linux/macOS user site-packages
		paths = append(paths, filepath.Join(home, ".local", "bin", "pylsp"))
	}
	
	// Windows specific paths
	if runtime.GOOS == "windows" {
		if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
			paths = append(paths, filepath.Join(userProfile, "AppData", "Local", "Programs", "Python", "Scripts", "pylsp.exe"))
			paths = append(paths, filepath.Join(userProfile, "AppData", "Roaming", "Python", "Scripts", "pylsp.exe"))
		}
		if appData := os.Getenv("APPDATA"); appData != "" {
			paths = append(paths, filepath.Join(appData, "Python", "Scripts", "pylsp.exe"))
		}
	}
	
	// Common system paths
	commonPaths := []string{
		"/usr/local/bin/pylsp",
		"/usr/bin/pylsp",
		"/opt/homebrew/bin/pylsp", // macOS Homebrew
	}
	paths = append(paths, commonPaths...)
	
	return paths
}

// getTypeScriptLanguageServerInstallPaths returns potential installation paths for typescript-language-server
func getTypeScriptLanguageServerInstallPaths() []string {
	paths := []string{}
	
	// Try to get npm global prefix
	if result, err := exec.Command("npm", "config", "get", "prefix").Output(); err == nil {
		prefix := strings.TrimSpace(string(result))
		if prefix != "" {
			if runtime.GOOS == "windows" {
				paths = append(paths, filepath.Join(prefix, "typescript-language-server.cmd"))
			} else {
				paths = append(paths, filepath.Join(prefix, "bin", "typescript-language-server"))
			}
		}
	}
	
	// Default npm global paths
	home := os.Getenv("HOME")
	if home != "" && runtime.GOOS != "windows" {
		paths = append(paths, filepath.Join(home, ".npm-global", "bin", "typescript-language-server"))
	}
	
	// Windows specific paths
	if runtime.GOOS == "windows" {
		if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
			paths = append(paths, filepath.Join(userProfile, "AppData", "Roaming", "npm", "typescript-language-server.cmd"))
		}
		if appData := os.Getenv("APPDATA"); appData != "" {
			paths = append(paths, filepath.Join(appData, "npm", "typescript-language-server.cmd"))
		}
	}
	
	// Common system paths
	commonPaths := []string{
		"/usr/local/bin/typescript-language-server",
		"/usr/bin/typescript-language-server",
		"/opt/homebrew/bin/typescript-language-server", // macOS Homebrew
	}
	paths = append(paths, commonPaths...)
	
	return paths
}

// verifyCommandWithPaths checks command availability using installation paths and PATH fallback
func verifyCommandWithPaths(executor platform.CommandExecutor, command string, installPaths []string) (bool, string, error) {
	// First, check installation-specific paths
	for _, path := range installPaths {
		if _, err := os.Stat(path); err == nil {
			// Verify the file is executable by running it with a version check
			if result, err := executor.Execute(path, []string{"--version"}, 3*time.Second); err == nil && result.ExitCode == 0 {
				return true, path, nil
			}
			// Some servers may not support --version, try --help
			if result, err := executor.Execute(path, []string{"--help"}, 3*time.Second); err == nil && result.ExitCode == 0 {
				return true, path, nil
			}
			// If file exists but isn't executable, note this as a potential issue
		}
	}
	
	// Fall back to PATH-based verification
	if executor.IsCommandAvailable(command) {
		// Try to get the full path for the command
		if path, err := exec.LookPath(command); err == nil {
			return true, path, nil
		}
		return true, command, nil // Command available but path unknown
	}
	
	return false, "", fmt.Errorf("command %s not found in installation paths or PATH", command)
}

// updatePathEnvironment adds installation directories to PATH for verification
func updatePathEnvironment(installPaths []string) map[string]string {
	env := make(map[string]string)
	currentPath := os.Getenv("PATH")
	
	// Extract directory paths from full executable paths
	dirs := make(map[string]bool)
	for _, fullPath := range installPaths {
		dir := filepath.Dir(fullPath)
		if _, err := os.Stat(dir); err == nil {
			dirs[dir] = true
		}
	}
	
	// Build new PATH
	pathSeparator := ":"
	if runtime.GOOS == "windows" {
		pathSeparator = ";"
	}
	
	newPathParts := []string{currentPath}
	for dir := range dirs {
		// Only add if not already in PATH
		if !strings.Contains(currentPath, dir) {
			newPathParts = append([]string{dir}, newPathParts...)
		}
	}
	
	env["PATH"] = strings.Join(newPathParts, pathSeparator)
	return env
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
			Details:  map[string]interface{}{"server": server},
		}, fmt.Errorf("unsupported server: %s", server)
	}
}

// installJDTLS installs Eclipse JDT Language Server
func (u *UniversalServerStrategy) installJDTLS(options types.ServerInstallOptions, start time.Time) (*types.InstallResult, error) {
	installPath := getJDTLSInstallPath()
	executablePath := GetJDTLSExecutablePath()

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
				Details:  map[string]interface{}{"server": ServerJDTLS, "executable": executablePath},
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
			Details:  map[string]interface{}{"server": ServerJDTLS},
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
			Details:  map[string]interface{}{"server": ServerJDTLS},
		}, downloadResult.Error
	}

	// Remove existing installation if it exists
	if err := os.RemoveAll(installPath); err != nil && !os.IsNotExist(err) {
		return &types.InstallResult{
			Success:  false,
			Runtime:  ServerJDTLS,
			Duration: time.Since(start),
			Errors:   []string{fmt.Sprintf("Failed to remove existing installation: %v", err)},
			Details:  map[string]interface{}{"server": ServerJDTLS},
		}, err
	}

	// Extract archive
	if err := ExtractTarGz(archivePath, installPath); err != nil {
		return &types.InstallResult{
			Success:  false,
			Runtime:  ServerJDTLS,
			Duration: time.Since(start),
			Errors:   []string{fmt.Sprintf("Failed to extract archive: %v", err)},
			Details:  map[string]interface{}{"server": ServerJDTLS, "archive_path": archivePath, "install_path": installPath},
		}, err
	}

	// Validate post-extraction structure
	if err := u.validatePostExtraction(installPath); err != nil {
		return &types.InstallResult{
			Success:  false,
			Runtime:  ServerJDTLS,
			Duration: time.Since(start),
			Errors:   []string{fmt.Sprintf("Post-extraction validation failed: %v", err)},
			Details:  map[string]interface{}{"server": ServerJDTLS, "install_path": installPath},
		}, err
	}

	// Create executable script
	platform := options.Platform
	if platform == "" {
		platform = runtime.GOOS
	}
	if err := u.createJDTLSScript(installPath, platform); err != nil {
		return &types.InstallResult{
			Success:  false,
			Runtime:  ServerJDTLS,
			Duration: time.Since(start),
			Errors:   []string{fmt.Sprintf("Failed to create executable script: %v", err)},
			Details:  map[string]interface{}{"server": ServerJDTLS, "install_path": installPath, "platform": platform},
		}, err
	}

	// Validate script content and permissions
	if err := u.validateScriptIntegrity(executablePath, installPath); err != nil {
		return &types.InstallResult{
			Success:  false,
			Runtime:  ServerJDTLS,
			Duration: time.Since(start),
			Errors:   []string{fmt.Sprintf("Script validation failed: %v", err)},
			Details:  map[string]interface{}{"server": ServerJDTLS, "executable_path": executablePath},
		}, err
	}

	// Comprehensive installation validation
	if err := u.validateJDTLSInstallation(executablePath); err != nil {
		return &types.InstallResult{
			Success:  false,
			Runtime:  ServerJDTLS,
			Duration: time.Since(start),
			Errors:   []string{fmt.Sprintf("Installation validation failed: %v", err)},
			Details:  map[string]interface{}{
				"server": ServerJDTLS, 
				"executable_path": executablePath,
				"install_path": installPath,
				"troubleshooting": "Check Java installation and PATH configuration",
			},
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
func (u *UniversalServerStrategy) createJDTLSScript(installPath string, platform string) error {
	binDir := filepath.Join(installPath, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Find the main jar file
	jarPath, err := u.findJDTLSJar(installPath)
	if err != nil {
		return fmt.Errorf("failed to find JDTLS jar: %w", err)
	}

	if platform == "windows" {
		return u.createWindowsJDTLSScript(binDir, jarPath, platform)
	}
	return u.createUnixJDTLSScript(binDir, jarPath, platform)
}

// validatePostExtraction validates that key files exist after JDTLS extraction
func (u *UniversalServerStrategy) validatePostExtraction(installPath string) error {
	// Check required directories exist
	requiredDirs := []string{
		filepath.Join(installPath, "plugins"),
		filepath.Join(installPath, "config_linux"),
	}
	
	// Add platform-specific config directories
	switch runtime.GOOS {
	case "windows":
		requiredDirs = append(requiredDirs, filepath.Join(installPath, "config_win"))
	case "darwin":
		requiredDirs = append(requiredDirs, filepath.Join(installPath, "config_mac"))
	}
	
	for _, dir := range requiredDirs {
		if stat, err := os.Stat(dir); err != nil {
			return fmt.Errorf("required directory missing: %s", dir)
		} else if !stat.IsDir() {
			return fmt.Errorf("path exists but is not a directory: %s", dir)
		}
	}
	
	// Verify the main JDTLS jar exists
	pluginsDir := filepath.Join(installPath, "plugins")
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return fmt.Errorf("cannot read plugins directory: %w", err)
	}
	
	var foundMainJar bool
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "org.eclipse.jdt.ls.core_") && strings.HasSuffix(entry.Name(), ".jar") {
			foundMainJar = true
			break
		}
	}
	
	if !foundMainJar {
		return fmt.Errorf("JDTLS main jar (org.eclipse.jdt.ls.core_*.jar) not found in plugins directory")
	}
	
	return nil
}

// validateScriptIntegrity validates that the created script has correct content and permissions
func (u *UniversalServerStrategy) validateScriptIntegrity(executablePath, installPath string) error {
	// Check if script file exists
	stat, err := os.Stat(executablePath)
	if err != nil {
		return fmt.Errorf("script file not found: %w", err)
	}
	
	// Check script permissions (should be executable)
	mode := stat.Mode()
	if mode&0111 == 0 {
		return fmt.Errorf("script is not executable: %s (mode: %o)", executablePath, mode)
	}
	
	// Read and validate script content
	content, err := os.ReadFile(executablePath)
	if err != nil {
		return fmt.Errorf("cannot read script content: %w", err)
	}
	
	scriptContent := string(content)
	
	// Validate script contains expected elements
	expectedElements := []string{
		"org.eclipse.jdt.ls.core",  // Main class reference
		"-jar",                     // Java jar execution
		"configuration",            // Configuration directory reference
	}
	
	for _, element := range expectedElements {
		if !strings.Contains(scriptContent, element) {
			return fmt.Errorf("script missing expected element: %s", element)
		}
	}
	
	// Validate script points to existing jar file
	jarPath, err := u.findJDTLSJar(installPath)
	if err != nil {
		return fmt.Errorf("cannot find jar referenced in script: %w", err)
	}
	
	// The script should reference the jar path (accounting for different path formats)
	jarName := filepath.Base(jarPath)
	if !strings.Contains(scriptContent, jarName) {
		return fmt.Errorf("script does not reference the correct jar file: %s", jarName)
	}
	
	return nil
}

// validateJDTLSInstallation performs comprehensive validation of JDTLS installation
func (u *UniversalServerStrategy) validateJDTLSInstallation(executablePath string) error {
	// Check if the executable file exists
	if _, err := os.Stat(executablePath); err != nil {
		return fmt.Errorf("executable not found at %s: %w", executablePath, err)
	}

	// Validate Java environment first
	if err := u.validateJavaEnvironment(); err != nil {
		return fmt.Errorf("Java environment validation failed: %w", err)
	}

	// JDTLS is a Language Server Protocol server, not a CLI tool that supports --help or --version
	// Instead of trying to run it (which requires workspace setup), we validate:
	// 1. Script exists and is executable (already checked above)
	// 2. Java is available (already checked above)
	// 3. Script contains proper Java invocation and jar references
	
	// Read and validate script content to ensure it's properly formed
	content, err := os.ReadFile(executablePath)
	if err != nil {
		return fmt.Errorf("cannot read executable script: %w", err)
	}
	
	scriptContent := string(content)
	
	// Validate script contains essential JDTLS elements
	requiredElements := []string{
		"java",                              // Java invocation
		"eclipse.application=org.eclipse.jdt.ls.core.id1", // JDTLS application
		"-jar",                             // Jar execution
		"configuration",                    // Configuration directory
		"data",                            // Data/workspace directory
	}
	
	for _, element := range requiredElements {
		if !strings.Contains(scriptContent, element) {
			return fmt.Errorf("script validation failed: missing required element '%s'", element)
		}
	}
	
	// Additional check: ensure the jar file referenced in the script actually exists
	jarPath, err := u.findJDTLSJar(filepath.Dir(filepath.Dir(executablePath)))
	if err != nil {
		return fmt.Errorf("script validation failed: %w", err)
	}
	
	if _, err := os.Stat(jarPath); err != nil {
		return fmt.Errorf("script validation failed: referenced jar file does not exist at %s", jarPath)
	}

	return nil
}

// validateJavaEnvironment checks if Java is available and suitable for JDTLS
func (u *UniversalServerStrategy) validateJavaEnvironment() error {
	// Try to find Java executable
	javaCmd := "java"
	if javaHome := os.Getenv("JAVA_HOME"); javaHome != "" {
		javaCmd = filepath.Join(javaHome, "bin", "java")
	}
	
	// Test Java availability and version
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, javaCmd, "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Java not available or not working (command: %s): %w, output: %s", javaCmd, err, string(output))
	}
	
	// Basic validation that Java is responding
	outputStr := strings.ToLower(string(output))
	if !strings.Contains(outputStr, "java") && !strings.Contains(outputStr, "openjdk") {
		return fmt.Errorf("Java version output does not look valid: %s", string(output))
	}
	
	return nil
}

// findJDTLSJar finds the main JDTLS launcher jar file
func (u *UniversalServerStrategy) findJDTLSJar(installPath string) (string, error) {
	pluginsDir := filepath.Join(installPath, "plugins")

	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return "", fmt.Errorf("failed to read plugins directory: %w", err)
	}

	// First, try to find the main Eclipse launcher jar (preferred method)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Look for the main equinox launcher jar (not platform-specific ones)
		if strings.HasPrefix(name, "org.eclipse.equinox.launcher_") && strings.HasSuffix(name, ".jar") {
			return filepath.Join(pluginsDir, name), nil
		}
	}

	// Second, try to find the JDTLS core jar
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.HasPrefix(name, "org.eclipse.jdt.ls.core_") && strings.HasSuffix(name, ".jar") {
			return filepath.Join(pluginsDir, name), nil
		}
	}

	return "", fmt.Errorf("JDTLS launcher jar (org.eclipse.equinox.launcher_*.jar) or core jar not found in plugins directory")
}

// createWindowsJDTLSScript creates a Windows batch script for jdtls
func (u *UniversalServerStrategy) createWindowsJDTLSScript(binDir, jarPath string, platform string) error {
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

REM Determine launch method based on jar type
for %%F in ("%s") do set JAR_NAME=%%~nxF

echo %%JAR_NAME%% | findstr /i "equinox.launcher" >nul
if %%errorlevel%% == 0 (
    REM Use equinox launcher with Eclipse application
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
) else (
    REM Use core jar with Eclipse launcher mechanism
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
)
`, jarPath, jarPath, configDir, jarPath, configDir)

	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		return fmt.Errorf("failed to write Windows script: %w", err)
	}

	return nil
}

// createUnixJDTLSScript creates a Unix shell script for jdtls
func (u *UniversalServerStrategy) createUnixJDTLSScript(binDir, jarPath string, platform string) error {
	scriptPath := filepath.Join(binDir, "jdtls")

	var configDir string
	switch platform {
	case "windows":
		configDir = filepath.Join(filepath.Dir(jarPath), "..", "config_win")
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

# Determine launch method based on jar type
JAR_NAME=$(basename "%s")

if [[ "$JAR_NAME" == *"equinox.launcher"* ]]; then
    # Use equinox launcher with Eclipse application
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
else
    # Use core jar with Eclipse launcher mechanism
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
fi
`, jarPath, jarPath, configDir, jarPath, configDir)

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
			Details:  map[string]interface{}{"server": ServerGopls},
		}, err
	}

	if result.ExitCode != 0 {
		return &types.InstallResult{
			Success:  false,
			Runtime:  ServerGopls,
			Duration: time.Since(start),
			Errors:   []string{fmt.Sprintf("Installation failed with exit code %d: %s", result.ExitCode, result.Stderr)},
			Details:  map[string]interface{}{"server": ServerGopls},
		}, fmt.Errorf("gopls installation failed")
	}

	return &types.InstallResult{
		Success:  true,
		Runtime:  ServerGopls,
		Method:   "go_install",
		Duration: time.Since(start),
		Messages: []string{"Successfully installed gopls"},
		Details:  map[string]interface{}{"server": ServerGopls},
	}, nil
}

// installPylsp installs the Python language server using pip
func (u *UniversalServerStrategy) installPylsp(options types.ServerInstallOptions, start time.Time) (*types.InstallResult, error) {
	// Try multiple installation strategies to handle different Python environments
	strategies := []struct {
		name        string
		cmd         []string
		description string
	}{
		{
			name:        "pip_with_break_system_packages",
			cmd:         []string{"pip", "install", "python-lsp-server", "--break-system-packages"},
			description: "Install using pip with --break-system-packages flag (for externally-managed environments)",
		},
		{
			name:        "pip_user_install",
			cmd:         []string{"pip", "install", "--user", "python-lsp-server"},
			description: "Install using pip with --user flag",
		},
		{
			name:        "pip3_with_break_system_packages",
			cmd:         []string{"pip3", "install", "python-lsp-server", "--break-system-packages"},
			description: "Install using pip3 with --break-system-packages flag",
		},
		{
			name:        "pip3_user_install",
			cmd:         []string{"pip3", "install", "--user", "python-lsp-server"},
			description: "Install using pip3 with --user flag",
		},
		{
			name:        "python_module_pip",
			cmd:         []string{"python3", "-m", "pip", "install", "python-lsp-server", "--break-system-packages"},
			description: "Install using python3 -m pip with --break-system-packages flag",
		},
		{
			name:        "python_module_pip_user",
			cmd:         []string{"python3", "-m", "pip", "install", "--user", "python-lsp-server"},
			description: "Install using python3 -m pip with --user flag",
		},
	}

	var lastError error
	var allErrors []string

	for _, strategy := range strategies {
		result, err := u.executor.Execute(strategy.cmd[0], strategy.cmd[1:], 5*time.Minute)
		
		if err != nil {
			lastError = err
			errorMsg := fmt.Sprintf("%s failed: %v", strategy.description, err)
			allErrors = append(allErrors, errorMsg)
			continue
		}

		if result.ExitCode != 0 {
			lastError = fmt.Errorf("pylsp installation failed with strategy: %s", strategy.name)
			
			// Check for externally-managed-environment error
			errorMsg := result.Stderr
			if strings.Contains(strings.ToLower(errorMsg), "externally-managed-environment") {
				errorMsg = fmt.Sprintf("%s failed: Python environment is externally managed (PEP 668). %s", 
					strategy.description, result.Stderr)
			} else {
				errorMsg = fmt.Sprintf("%s failed with exit code %d: %s", 
					strategy.description, result.ExitCode, result.Stderr)
			}
			allErrors = append(allErrors, errorMsg)
			continue
		}

		// Success with this strategy
		return &types.InstallResult{
			Success:  true,
			Runtime:  ServerPylsp,
			Method:   strategy.name,
			Duration: time.Since(start),
			Messages: []string{
				fmt.Sprintf("Successfully installed python-lsp-server using: %s", strategy.description),
				result.Stdout,
			},
			Details: map[string]interface{}{
				"server":           ServerPylsp,
				"strategy":         strategy.name,
				"command":          strings.Join(strategy.cmd, " "),
				"installation_output": result.Stdout,
			},
		}, nil
	}

	// All strategies failed
	return &types.InstallResult{
		Success:  false,
		Runtime:  ServerPylsp,
		Duration: time.Since(start),
		Errors:   allErrors,
		Details: map[string]interface{}{
			"server":             ServerPylsp,
			"attempted_strategies": len(strategies),
			"last_error":         lastError.Error(),
		},
	}, fmt.Errorf("all installation strategies failed: %v", lastError)
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
			Details:  map[string]interface{}{"server": ServerTypeScriptLanguageServer},
		}, err
	}

	if result.ExitCode != 0 {
		return &types.InstallResult{
			Success:  false,
			Runtime:  ServerTypeScriptLanguageServer,
			Duration: time.Since(start),
			Errors:   []string{fmt.Sprintf("Installation failed with exit code %d: %s", result.ExitCode, result.Stderr)},
			Details:  map[string]interface{}{"server": ServerTypeScriptLanguageServer},
		}, fmt.Errorf("typescript-language-server installation failed")
	}

	return &types.InstallResult{
		Success:  true,
		Runtime:  ServerTypeScriptLanguageServer,
		Method:   "npm_global",
		Duration: time.Since(start),
		Messages: []string{"Successfully installed typescript-language-server"},
		Details:  map[string]interface{}{"server": ServerTypeScriptLanguageServer},
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
	executablePath := GetJDTLSExecutablePath()

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

	// Try to verify the executable is functional
	if versionResult, err := u.executor.Execute(executablePath, []string{"--version"}, 5*time.Second); err == nil && versionResult.ExitCode == 0 {
		result.Details["version_output"] = strings.TrimSpace(versionResult.Stdout)
	} else {
		// JDTLS may not support --version, try a basic functional test
		result.Details["version_check_failed"] = true
	}

	result.Installed = true
	result.Path = executablePath
	result.Version = JDTLSVersion
	result.Compatible = true
	result.Details["install_path"] = getJDTLSInstallPath()
	result.Details["executable_path"] = executablePath

	return result, nil
}

// verifyGopls verifies gopls installation
func (u *UniversalServerStrategy) verifyGopls() (*types.VerificationResult, error) {
	result := &types.VerificationResult{
		Runtime: ServerGopls,
		Issues:  []types.Issue{},
		Details: make(map[string]interface{}),
	}

	// Get potential installation paths
	installPaths := getGoplsInstallPaths()
	result.Details["searched_paths"] = installPaths

	// Check installation-aware verification
	found, executablePath, err := verifyCommandWithPaths(u.executor, "gopls", installPaths)
	if !found {
		result.Issues = append(result.Issues, types.Issue{
			Severity:    types.IssueSeverityHigh,
			Title:       "gopls not found",
			Description: fmt.Sprintf("gopls command is not available in installation paths or PATH. Error: %v", err),
		})
		return result, nil
	}

	result.Installed = true
	result.Compatible = true
	result.Path = executablePath
	result.Details["executable_path"] = executablePath

	// Try to get version information
	if versionResult, err := u.executor.Execute(executablePath, []string{"version"}, 5*time.Second); err == nil && versionResult.ExitCode == 0 {
		result.Version = strings.TrimSpace(versionResult.Stdout)
		result.Details["version_output"] = result.Version
	}

	return result, nil
}

// verifyPylsp verifies pylsp installation
func (u *UniversalServerStrategy) verifyPylsp() (*types.VerificationResult, error) {
	result := &types.VerificationResult{
		Runtime: ServerPylsp,
		Issues:  []types.Issue{},
		Details: make(map[string]interface{}),
	}

	// Get potential installation paths
	installPaths := getPylspInstallPaths()
	result.Details["searched_paths"] = installPaths

	// Check installation-aware verification
	found, executablePath, err := verifyCommandWithPaths(u.executor, "pylsp", installPaths)
	if !found {
		result.Issues = append(result.Issues, types.Issue{
			Severity:    types.IssueSeverityHigh,
			Title:       "pylsp not found",
			Description: fmt.Sprintf("pylsp command is not available in installation paths or PATH. Error: %v", err),
		})
		return result, nil
	}

	result.Installed = true
	result.Compatible = true
	result.Path = executablePath
	result.Details["executable_path"] = executablePath

	// Try to get version information
	if versionResult, err := u.executor.Execute(executablePath, []string{"--version"}, 5*time.Second); err == nil && versionResult.ExitCode == 0 {
		result.Version = strings.TrimSpace(versionResult.Stdout)
		result.Details["version_output"] = result.Version
	}

	return result, nil
}

// verifyTypescriptLanguageServer verifies typescript-language-server installation
func (u *UniversalServerStrategy) verifyTypescriptLanguageServer() (*types.VerificationResult, error) {
	result := &types.VerificationResult{
		Runtime: ServerTypeScriptLanguageServer,
		Issues:  []types.Issue{},
		Details: make(map[string]interface{}),
	}

	// Get potential installation paths
	installPaths := getTypeScriptLanguageServerInstallPaths()
	result.Details["searched_paths"] = installPaths

	// Check installation-aware verification
	found, executablePath, err := verifyCommandWithPaths(u.executor, "typescript-language-server", installPaths)
	if !found {
		result.Issues = append(result.Issues, types.Issue{
			Severity:    types.IssueSeverityHigh,
			Title:       "typescript-language-server not found",
			Description: fmt.Sprintf("typescript-language-server command is not available in installation paths or PATH. Error: %v", err),
		})
		return result, nil
	}

	result.Installed = true
	result.Compatible = true
	result.Path = executablePath
	result.Details["executable_path"] = executablePath

	// Try to get version information
	if versionResult, err := u.executor.Execute(executablePath, []string{"--version"}, 5*time.Second); err == nil && versionResult.ExitCode == 0 {
		result.Version = strings.TrimSpace(versionResult.Stdout)
		result.Details["version_output"] = result.Version
	}

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
		return []string{"pip", "install", "python-lsp-server", "--break-system-packages"}, nil
	case ServerTypeScriptLanguageServer:
		return []string{"npm", "install", "-g", "typescript-language-server", "typescript"}, nil
	default:
		return []string{}, fmt.Errorf("unsupported server: %s", server)
	}
}
