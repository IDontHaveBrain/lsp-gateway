package installer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"lsp-gateway/internal/platform"
)

// Java 21 Runtime Configuration
const (
	Java21Version        = "21"
	Java21InstallDir     = "java21"
	Java21ExecutableName = "java"
)

// Platform-specific Java 21 download URLs and checksums
// Using Eclipse Temurin API: https://api.adoptium.net/v3/binary/latest/21/ga/{os}/{arch}/jdk/hotspot/normal/eclipse
var java21Downloads = map[string]map[string]JavaDownloadInfo{
	"linux": {
		"x64": {
			URL:      "https://api.adoptium.net/v3/binary/latest/21/ga/linux/x64/jdk/hotspot/normal/eclipse",
			Checksum: "", // Will be fetched dynamically from .sha256.txt
			Archive:  "tar.gz",
		},
		"aarch64": {
			URL:      "https://api.adoptium.net/v3/binary/latest/21/ga/linux/aarch64/jdk/hotspot/normal/eclipse",
			Checksum: "",
			Archive:  "tar.gz",
		},
	},
	"windows": {
		"x64": {
			URL:      "https://api.adoptium.net/v3/binary/latest/21/ga/windows/x64/jdk/hotspot/normal/eclipse",
			Checksum: "",
			Archive:  "zip",
		},
	},
	"darwin": {
		"x64": {
			URL:      "https://api.adoptium.net/v3/binary/latest/21/ga/mac/x64/jdk/hotspot/normal/eclipse",
			Checksum: "",
			Archive:  "tar.gz",
		},
		"aarch64": {
			URL:      "https://api.adoptium.net/v3/binary/latest/21/ga/mac/aarch64/jdk/hotspot/normal/eclipse",
			Checksum: "",
			Archive:  "tar.gz",
		},
	},
}

// JavaDownloadInfo contains download information for Java 21 runtime
type JavaDownloadInfo struct {
	URL      string
	Checksum string
	Archive  string
}

// AdoptiumAssetResponse represents the response from Adoptium API assets endpoint
type AdoptiumAssetResponse struct {
	Binary struct {
		Package struct {
			Name     string `json:"name"`
			Link     string `json:"link"`
			Checksum string `json:"checksum"`
		} `json:"package"`
		Architecture string `json:"architecture"`
		OS           string `json:"os"`
	} `json:"binary"`
}

// Java21RuntimeManager handles downloading and installing Java 21 runtime
type Java21RuntimeManager struct {
	executor   platform.CommandExecutor
	downloader *FileDownloader
}

// NewJava21RuntimeManager creates a new Java 21 runtime manager
func NewJava21RuntimeManager() *Java21RuntimeManager {
	return &Java21RuntimeManager{
		executor:   platform.NewCommandExecutor(),
		downloader: NewFileDownloader(),
	}
}

// Note: GetJava21ExecutablePath and getJava21InstallPath are defined in server_strategies.go

// IsJava21Installed checks if Java 21 runtime is already installed
func (j *Java21RuntimeManager) IsJava21Installed() bool {
	executablePath := GetJava21ExecutablePath()
	if _, err := os.Stat(executablePath); err != nil {
		return false
	}
	
	// Verify it's actually Java 21 by checking version
	result, err := j.executor.Execute(executablePath, []string{"-version"}, 5*time.Second)
	if err != nil || result.ExitCode != 0 {
		return false
	}
	
	// Check if output contains Java 21
	output := strings.ToLower(result.Stderr)
	return strings.Contains(output, "openjdk 21") || strings.Contains(output, "java 21") || 
		   strings.Contains(output, "21.0.")
}

// GetPlatformArchitecture returns the current platform and architecture for download URLs
func (j *Java21RuntimeManager) GetPlatformArchitecture() (string, string, error) {
	platformOS := runtime.GOOS
	arch := runtime.GOARCH
	
	// Normalize platform names for Eclipse Temurin API
	switch platformOS {
	case "darwin":
		platformOS = "darwin" // Keep as is for internal mapping
	case "linux":
		platformOS = "linux"
	case "windows":
		platformOS = "windows"
	default:
		return "", "", fmt.Errorf("unsupported platform: %s", platformOS)
	}
	
	// Normalize architecture names
	switch arch {
	case "amd64":
		arch = "x64"
	case "arm64":
		arch = "aarch64"
	case "386":
		arch = "x86"
	default:
		// Keep as is for other architectures
	}
	
	return platformOS, arch, nil
}

// GetDownloadInfo gets the download information for the current platform
func (j *Java21RuntimeManager) GetDownloadInfo() (*JavaDownloadInfo, error) {
	platformOS, arch, err := j.GetPlatformArchitecture()
	if err != nil {
		return nil, err
	}
	
	platformDownloads, exists := java21Downloads[platformOS]
	if !exists {
		return nil, fmt.Errorf("no Java 21 downloads available for platform: %s", platformOS)
	}
	
	downloadInfo, exists := platformDownloads[arch]
	if !exists {
		return nil, fmt.Errorf("no Java 21 downloads available for architecture: %s on %s", arch, platformOS)
	}
	
	return &downloadInfo, nil
}

// fetchChecksum fetches the SHA256 checksum using Adoptium Assets API
func (j *Java21RuntimeManager) fetchChecksum(downloadURL string) (string, error) {
	// First try the modern Adoptium Assets API approach
	checksum, err := j.fetchChecksumFromAssetsAPI()
	if err == nil && checksum != "" {
		return checksum, nil
	}
	
	// Fallback to redirect method
	return j.fetchChecksumFromRedirect(downloadURL)
}

// fetchChecksumFromAssetsAPI gets checksum from Adoptium Assets metadata API
func (j *Java21RuntimeManager) fetchChecksumFromAssetsAPI() (string, error) {
	platformOS, arch, err := j.GetPlatformArchitecture()
	if err != nil {
		return "", fmt.Errorf("failed to get platform info: %w", err)
	}
	
	// Convert platform names for Adoptium API
	apiOS := platformOS
	if platformOS == "darwin" {
		apiOS = "mac"
	}
	
	// Build assets API URL
	assetsURL := fmt.Sprintf("https://api.adoptium.net/v3/assets/latest/21/hotspot?architecture=%s&image_type=jdk&os=%s&vendor=eclipse",
		arch, apiOS)
	
	// Make HTTP request with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	resp, err := client.Get(assetsURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch assets metadata: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("assets API returned status %d", resp.StatusCode)
	}
	
	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read assets response: %w", err)
	}
	
	// Parse JSON response
	var assetResponse AdoptiumAssetResponse
	if err := json.Unmarshal(body, &assetResponse); err != nil {
		return "", fmt.Errorf("failed to parse assets response: %w", err)
	}
	
	// Extract checksum
	checksum := assetResponse.Binary.Package.Checksum
	if checksum == "" {
		return "", fmt.Errorf("no checksum found in assets response")
	}
	
	// Validate checksum format (should be 64 character SHA256)
	if len(checksum) != 64 {
		return "", fmt.Errorf("invalid checksum length: expected 64, got %d", len(checksum))
	}
	
	return checksum, nil
}

// fetchChecksumFromRedirect fallback method using HTTP redirect and .sha256.txt append
func (j *Java21RuntimeManager) fetchChecksumFromRedirect(downloadURL string) (string, error) {
	// Follow redirect to get actual GitHub release URL
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Don't follow redirects, we want to capture the redirect URL
			return http.ErrUseLastResponse
		},
	}
	
	resp, err := client.Head(downloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to get redirect URL: %w", err)
	}
	defer resp.Body.Close()
	
	// Get redirect URL
	redirectURL := resp.Header.Get("Location")
	if redirectURL == "" {
		return "", fmt.Errorf("no redirect URL found")
	}
	
	// Construct checksum URL by appending .sha256.txt to redirect URL
	checksumURL := redirectURL + ".sha256.txt"
	
	// Create temporary file for checksum
	tempFile, err := os.CreateTemp("", "java21-checksum-*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file for checksum: %w", err)
	}
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}()
	
	// Download checksum file
	checksumOptions := DownloadOptions{
		URL:        checksumURL,
		OutputPath: tempFile.Name(),
		Timeout:    30 * time.Second,
		MaxRetries: 3,
	}
	
	result := j.downloader.Download(checksumOptions)
	if !result.Success {
		return "", fmt.Errorf("failed to download checksum from %s: %w", checksumURL, result.Error)
	}
	
	// Read checksum content
	content, err := os.ReadFile(tempFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read checksum file: %w", err)
	}
	
	// Extract checksum (first 64 characters of SHA256)
	checksumContent := strings.TrimSpace(string(content))
	if len(checksumContent) >= 64 {
		return checksumContent[:64], nil
	}
	
	return "", fmt.Errorf("invalid checksum format: %s", checksumContent)
}

// DownloadAndInstall downloads and installs Java 21 runtime
func (j *Java21RuntimeManager) DownloadAndInstall(force bool) error {
	if !force && j.IsJava21Installed() {
		return nil // Already installed
	}
	
	downloadInfo, err := j.GetDownloadInfo()
	if err != nil {
		return fmt.Errorf("failed to get download info: %w", err)
	}
	
	// Fetch checksum dynamically
	checksum, err := j.fetchChecksum(downloadInfo.URL)
	if err != nil {
		return fmt.Errorf("failed to fetch checksum: %w", err)
	}
	
	// Create temporary directory for download
	tempDir, err := os.MkdirTemp("", "java21-install-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()
	
	// Download Java 21 archive
	var archiveExtension string
	switch downloadInfo.Archive {
	case "tar.gz":
		archiveExtension = ".tar.gz"
	case "zip":
		archiveExtension = ".zip"
	default:
		archiveExtension = ".tar.gz"
	}
	
	archivePath := filepath.Join(tempDir, "java21"+archiveExtension)
	
	downloadOptions := DownloadOptions{
		URL:              downloadInfo.URL,
		OutputPath:       archivePath,
		ExpectedChecksum: checksum,
		Timeout:          10 * time.Minute,
		MaxRetries:       3,
		ProgressCallback: func(downloaded, total int64) {
			// Could add progress reporting here if needed
		},
	}
	
	downloadResult := j.downloader.Download(downloadOptions)
	if !downloadResult.Success {
		return fmt.Errorf("download failed: %w", downloadResult.Error)
	}
	
	// Install using atomic installation
	return j.installJava21(archivePath, downloadInfo.Archive)
}

// installJava21 performs atomic installation of Java 21 runtime
func (j *Java21RuntimeManager) installJava21(archivePath, archiveType string) error {
	installPath := GetJava21InstallPath()
	
	// Create temporary installation directory for atomic installation
	tempInstallDir := installPath + ".tmp"
	
	// Remove any existing temporary installation
	if err := os.RemoveAll(tempInstallDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove existing temp installation: %w", err)
	}
	
	// Extract archive to temporary location
	if err := j.extractArchive(archivePath, tempInstallDir, archiveType); err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}
	
	// Find the actual Java home directory within the extracted archive
	javaHome, err := j.findJavaHome(tempInstallDir)
	if err != nil {
		_ = os.RemoveAll(tempInstallDir)
		return fmt.Errorf("failed to find Java home directory: %w", err)
	}
	
	// Validate installation using the found Java home
	if err := j.validateJavaHomeInstallation(javaHome); err != nil {
		_ = os.RemoveAll(tempInstallDir)
		return fmt.Errorf("installation validation failed: %w", err)
	}
	
	// Remove existing installation if it exists
	if err := os.RemoveAll(installPath); err != nil && !os.IsNotExist(err) {
		_ = os.RemoveAll(tempInstallDir)
		return fmt.Errorf("failed to remove existing installation: %w", err)
	}
	
	// Create the install directory
	if err := os.MkdirAll(installPath, 0755); err != nil {
		_ = os.RemoveAll(tempInstallDir)
		return fmt.Errorf("failed to create install directory: %w", err)
	}
	
	// Atomic move: copy contents of Java home to install path, then remove temp
	if err := j.moveJavaHomeContents(javaHome, installPath); err != nil {
		_ = os.RemoveAll(tempInstallDir)
		_ = os.RemoveAll(installPath)
		return fmt.Errorf("failed to move Java home contents to final location: %w", err)
	}
	
	// Clean up temporary directory after successful move
	_ = os.RemoveAll(tempInstallDir)
	
	return nil
}

// extractArchive extracts the Java 21 archive to the specified directory
func (j *Java21RuntimeManager) extractArchive(archivePath, destDir, archiveType string) error {
	switch archiveType {
	case "tar.gz":
		return ExtractTarGz(archivePath, destDir)
	case "zip":
		return j.extractZip(archivePath, destDir)
	default:
		return fmt.Errorf("unsupported archive type: %s", archiveType)
	}
}

// extractZip extracts a ZIP archive (for Windows)
func (j *Java21RuntimeManager) extractZip(archivePath, destDir string) error {
	// For now, use platform-specific extraction
	if runtime.GOOS == "windows" {
		// Use PowerShell to extract ZIP on Windows
		cmd := exec.Command("powershell", "-Command", 
			fmt.Sprintf("Expand-Archive -Path '%s' -DestinationPath '%s' -Force", archivePath, destDir))
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to extract ZIP archive: %w", err)
		}
		return nil
	}
	
	// For non-Windows platforms, use unzip command if available
	cmd := exec.Command("unzip", "-q", archivePath, "-d", destDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to extract ZIP archive: %w", err)
	}
	
	return nil
}

// validateInstallation validates that the Java 21 installation is complete and functional
func (j *Java21RuntimeManager) validateInstallation(installDir string) error {
	// Find the actual Java installation directory (may be nested)
	javaHome, err := j.findJavaHome(installDir)
	if err != nil {
		return fmt.Errorf("failed to find Java home directory: %w", err)
	}
	
	// Check required directories and files exist
	requiredPaths := []string{
		filepath.Join(javaHome, "bin"),
		filepath.Join(javaHome, "lib"),
	}
	
	// Platform-specific Java executable
	var javaExecutable string
	if runtime.GOOS == "windows" {
		javaExecutable = filepath.Join(javaHome, "bin", "java.exe")
	} else {
		javaExecutable = filepath.Join(javaHome, "bin", "java")
	}
	requiredPaths = append(requiredPaths, javaExecutable)
	
	for _, path := range requiredPaths {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("required path missing: %s", path)
		}
	}
	
	// Test Java executable functionality
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, javaExecutable, "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Java executable test failed: %w, output: %s", err, string(output))
	}
	
	// Verify it's Java 21
	outputStr := strings.ToLower(string(output))
	if !strings.Contains(outputStr, "21") {
		return fmt.Errorf("installed Java is not version 21: %s", string(output))
	}
	
	return nil
}

// findJavaHome finds the actual Java home directory within the extracted archive
func (j *Java21RuntimeManager) findJavaHome(extractDir string) (string, error) {
	// Java archives often contain a top-level directory like "jdk-21.0.1+12"
	// We need to find the directory that contains "bin" and "lib" subdirectories
	
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return "", fmt.Errorf("failed to read extract directory: %w", err)
	}
	
	// Check if extractDir itself is the Java home
	if j.isJavaHome(extractDir) {
		return extractDir, nil
	}
	
	// Look for subdirectories that might be Java home
	for _, entry := range entries {
		if entry.IsDir() {
			candidatePath := filepath.Join(extractDir, entry.Name())
			if j.isJavaHome(candidatePath) {
				return candidatePath, nil
			}
		}
	}
	
	return "", fmt.Errorf("could not find Java home directory in extracted archive")
}

// isJavaHome checks if a directory looks like a Java home (contains bin and lib directories)
func (j *Java21RuntimeManager) isJavaHome(dir string) bool {
	requiredDirs := []string{"bin", "lib"}
	for _, reqDir := range requiredDirs {
		if stat, err := os.Stat(filepath.Join(dir, reqDir)); err != nil || !stat.IsDir() {
			return false
		}
	}
	return true
}

// VerifyInstallation verifies that Java 21 runtime is properly installed and functional
func (j *Java21RuntimeManager) VerifyInstallation() error {
	if !j.IsJava21Installed() {
		return fmt.Errorf("Java 21 runtime is not installed")
	}
	
	executablePath := GetJava21ExecutablePath()
	
	// Test functionality
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, executablePath, "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Java 21 verification failed: %w, output: %s", err, string(output))
	}
	
	// Verify it's Java 21
	outputStr := strings.ToLower(string(output))
	if !strings.Contains(outputStr, "21") {
		return fmt.Errorf("installed Java is not version 21: %s", string(output))
	}
	
	return nil
}

// GetJava21Version returns the installed Java 21 version string
func (j *Java21RuntimeManager) GetJava21Version() (string, error) {
	if !j.IsJava21Installed() {
		return "", fmt.Errorf("Java 21 runtime is not installed")
	}
	
	executablePath := GetJava21ExecutablePath()
	
	result, err := j.executor.Execute(executablePath, []string{"-version"}, 5*time.Second)
	if err != nil {
		return "", fmt.Errorf("failed to get Java version: %w", err)
	}
	
	if result.ExitCode != 0 {
		return "", fmt.Errorf("Java version command failed: %s", result.Stderr)
	}
	
	// Parse version from stderr (Java outputs version info to stderr)
	lines := strings.Split(result.Stderr, "\n")
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "openjdk") || 
		   strings.Contains(strings.ToLower(line), "java") {
			return strings.TrimSpace(line), nil
		}
	}
	
	return strings.TrimSpace(result.Stderr), nil
}

// validateJavaHomeInstallation validates that a specific Java home directory is complete and functional
func (j *Java21RuntimeManager) validateJavaHomeInstallation(javaHome string) error {
	// Check required directories and files exist
	requiredPaths := []string{
		filepath.Join(javaHome, "bin"),
		filepath.Join(javaHome, "lib"),
	}
	
	// Platform-specific Java executable
	var javaExecutable string
	if runtime.GOOS == "windows" {
		javaExecutable = filepath.Join(javaHome, "bin", "java.exe")
	} else {
		javaExecutable = filepath.Join(javaHome, "bin", "java")
	}
	requiredPaths = append(requiredPaths, javaExecutable)
	
	for _, path := range requiredPaths {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("required path missing: %s", path)
		}
	}
	
	// Test Java executable functionality
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, javaExecutable, "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Java executable test failed: %w, output: %s", err, string(output))
	}
	
	// Verify it's Java 21
	outputStr := strings.ToLower(string(output))
	if !strings.Contains(outputStr, "21") {
		return fmt.Errorf("installed Java is not version 21: %s", string(output))
	}
	
	return nil
}

// moveJavaHomeContents moves all contents from Java home source to destination directory
func (j *Java21RuntimeManager) moveJavaHomeContents(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}
	
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		
		if err := os.Rename(srcPath, dstPath); err != nil {
			return fmt.Errorf("failed to move %s to %s: %w", srcPath, dstPath, err)
		}
	}
	
	return nil
}

// Uninstall removes the Java 21 runtime installation
func (j *Java21RuntimeManager) Uninstall() error {
	installPath := GetJava21InstallPath()
	
	if _, err := os.Stat(installPath); os.IsNotExist(err) {
		return nil // Already uninstalled
	}
	
	return os.RemoveAll(installPath)
}