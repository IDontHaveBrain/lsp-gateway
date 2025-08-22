package installer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"lsp-gateway/src/internal/common"
	icommon "lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/security"
)

// KotlinInstaller handles Kotlin language server installation
type KotlinInstaller struct {
	*BaseInstaller
}

// NewKotlinInstaller creates a new Kotlin installer
func NewKotlinInstaller(platform PlatformInfo) *KotlinInstaller {
	base := CreateSimpleInstaller("kotlin", "kotlin-lsp", []string{"--stdio"}, platform)
	return &KotlinInstaller{BaseInstaller: base}
}

// Install installs Kotlin LSP using brew on macOS or GitHub releases on other platforms
func (k *KotlinInstaller) Install(ctx context.Context, options InstallOptions) error {
	common.CLILogger.Info("Installing Kotlin Language Server...")

	// Check if already installed and working
	if !options.Force {
		if k.testKotlinLSP(ctx) {
			common.CLILogger.Info("kotlin-lsp already available and working")
			return nil
		}
	}

	// Try brew installation on macOS first
	if runtime.GOOS == "darwin" {
		if err := k.tryBrewInstall(ctx); err == nil {
			if k.testKotlinLSP(ctx) {
				common.CLILogger.Info("kotlin-lsp installed successfully via brew")
				return nil
			}
			common.CLILogger.Warn("brew installation completed but kotlin-lsp not working, trying GitHub release")
		} else {
			common.CLILogger.Warn("brew installation failed: %v, trying GitHub release", err)
		}
	}

	// Fallback to GitHub binary download
	return k.installFromGitHub(ctx, options)
}

// tryBrewInstall attempts to install via homebrew on macOS
func (k *KotlinInstaller) tryBrewInstall(ctx context.Context) error {
	// Check if brew is available
	if _, err := exec.LookPath("brew"); err != nil {
		return fmt.Errorf("homebrew not found")
	}

	common.CLILogger.Info("Installing kotlin-lsp via homebrew...")

	// Install from JetBrains tap
	if err := k.RunCommand(ctx, "brew", "install", "JetBrains/utils/kotlin-lsp"); err != nil {
		return fmt.Errorf("brew install failed: %w", err)
	}

	return nil
}

// installFromGitHub downloads and installs from GitHub releases
func (k *KotlinInstaller) installFromGitHub(ctx context.Context, options InstallOptions) error {
	common.CLILogger.Info("Installing kotlin-lsp from GitHub releases...")

	installPath := k.GetInstallPath()
	if options.InstallPath != "" {
		installPath = options.InstallPath
		k.SetInstallPath(installPath)
	}

	// Create install directory
	if err := k.CreateInstallDirectory(installPath); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	// Get download URL for current platform
	downloadURL, err := k.getGitHubReleaseURL(ctx)
	if err != nil {
		return fmt.Errorf("failed to get GitHub release URL: %w", err)
	}

	// Download the release
	archivePath := filepath.Join(installPath, "kotlin-lsp-download.zip")
	if err := k.DownloadFile(ctx, downloadURL, archivePath); err != nil {
		return fmt.Errorf("failed to download kotlin-lsp: %w", err)
	}

	// Extract the archive
	extractPath := filepath.Join(installPath, "extracted")
	if err := k.ExtractArchive(ctx, archivePath, extractPath); err != nil {
		return fmt.Errorf("failed to extract kotlin-lsp: %w", err)
	}

	// Find and setup the binary
	if err := k.setupBinary(extractPath, installPath); err != nil {
		return fmt.Errorf("failed to setup kotlin-lsp binary: %w", err)
	}

	// Clean up temporary files
	os.Remove(archivePath)
	os.RemoveAll(extractPath)

	common.CLILogger.Info("Kotlin Language Server installation completed")
	return nil
}

// getGitHubReleaseURL gets the download URL for the latest release
func (k *KotlinInstaller) getGitHubReleaseURL(ctx context.Context) (string, error) {
	apiURL := "https://api.github.com/repos/Kotlin/kotlin-lsp/releases/latest"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "lsp-gateway-installer")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to fetch releases: %s", resp.Status)
	}

	var release struct {
		Body string `json:"body"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	// Extract the standalone zip URL from the release body
	// Looking for [Download](URL) pattern in markdown
	lines := strings.Split(release.Body, "\n")
	for _, line := range lines {
		// Look for lines containing "Standalone" and check the next download link
		if strings.Contains(line, "Standalone") || strings.Contains(line, "kotlin-lsp") {
			// Extract URL from markdown link format [Download](URL)
			if strings.Contains(line, "[Download](") {
				start := strings.Index(line, "[Download](") + len("[Download](")
				end := strings.Index(line[start:], ")")
				if end > 0 {
					url := line[start : start+end]
					if strings.HasSuffix(url, ".zip") && !strings.Contains(url, ".vsix") {
						return url, nil
					}
				}
			}
		}
		// Also check for direct download links in the line
		if strings.Contains(line, "download-cdn.jetbrains.com/kotlin-lsp") {
			// Try to extract URL from markdown link
			startIdx := strings.Index(line, "(https://")
			if startIdx != -1 {
				startIdx += 1 // Skip the opening parenthesis
				endIdx := strings.Index(line[startIdx:], ")")
				if endIdx > 0 {
					url := line[startIdx : startIdx+endIdx]
					if strings.HasSuffix(url, ".zip") && !strings.Contains(url, ".vsix") {
						return url, nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("no suitable kotlin-lsp download URL found in release")
}

// setupBinary finds and sets up the kotlin-lsp binary and required files
func (k *KotlinInstaller) setupBinary(extractPath, installPath string) error {
	// The official Kotlin LSP has kotlin-lsp.sh in the root directory
	// Copy all files from extract path to install path
	if err := copyDirContents(extractPath, installPath); err != nil {
		return fmt.Errorf("failed to copy kotlin-lsp files: %w", err)
	}

	// Ensure bin directory exists for consistent layout
	binDir := filepath.Join(installPath, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Set up the appropriate command based on platform
	if runtime.GOOS == "windows" {
		// Prefer .cmd, then .bat, then fallback
		cmdPath := filepath.Join(installPath, "kotlin-lsp.cmd")
		if _, err := os.Stat(cmdPath); err == nil {
			// Copy shim into bin for stable path
			dest := filepath.Join(binDir, "kotlin-lsp.cmd")
			if err := copyFile(cmdPath, dest); err == nil {
				k.serverConfig.Command = dest
			} else {
				k.serverConfig.Command = cmdPath
			}
		} else {
			batPath := filepath.Join(installPath, "kotlin-lsp.bat")
			if _, err := os.Stat(batPath); err == nil {
				dest := filepath.Join(binDir, "kotlin-lsp.bat")
				if err := copyFile(batPath, dest); err == nil {
					k.serverConfig.Command = dest
				} else {
					k.serverConfig.Command = batPath
				}
			} else {
				// Fallback to .sh script even on Windows (might work with Git Bash/WSL)
				shPath := filepath.Join(installPath, "kotlin-lsp.sh")
				if _, err := os.Stat(shPath); err == nil {
					dest := filepath.Join(binDir, "kotlin-lsp.sh")
					if err := copyFile(shPath, dest); err == nil {
						k.serverConfig.Command = dest
					} else {
						k.serverConfig.Command = shPath
					}
				} else {
					return fmt.Errorf("kotlin-lsp executable not found")
				}
			}
		}
	} else {
		// Unix-like systems use the .sh script
		shPath := filepath.Join(installPath, "kotlin-lsp.sh")
		if _, err := os.Stat(shPath); err != nil {
			return fmt.Errorf("kotlin-lsp.sh not found in extracted content")
		}

		// Make executable
		if err := os.Chmod(shPath, 0755); err != nil {
			return fmt.Errorf("failed to make kotlin-lsp.sh executable: %w", err)
		}

		// Create a symlink in bin without .sh extension
		linkPath := filepath.Join(binDir, "kotlin-lsp")
		os.Remove(linkPath) // Remove if exists
		if err := os.Symlink(shPath, linkPath); err != nil {
			// If symlink fails, copy the script as bin/kotlin-lsp
			common.CLILogger.Warn("Failed to create symlink, copying script to bin: %v", err)
			if err := copyFile(shPath, linkPath); err != nil {
				// Fallback: use the .sh path directly
				k.serverConfig.Command = shPath
			} else {
				if err := os.Chmod(linkPath, 0755); err == nil {
					k.serverConfig.Command = linkPath
				} else {
					k.serverConfig.Command = shPath
				}
			}
		} else {
			k.serverConfig.Command = linkPath
		}
	}

	common.CLILogger.Info("kotlin-lsp setup completed at %s", installPath)
	return nil
}

// testKotlinLSP tests if kotlin-lsp is working
func (k *KotlinInstaller) testKotlinLSP(ctx context.Context) bool {
	testCtx, cancel := icommon.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 1) Try configured command directly (absolute or relative path)
	if cmd := strings.TrimSpace(k.serverConfig.Command); cmd != "" {
		if _, err := k.RunCommandWithOutput(testCtx, cmd, "--version"); err == nil {
			return true
		}
		// If version flag fails, consider presence of executable sufficient
		if _, err := exec.LookPath(cmd); err == nil {
			return true
		}
	}

	// 2) Try standard PATH commands
	if _, err := k.RunCommandWithOutput(testCtx, "kotlin-lsp", "--version"); err == nil {
		k.serverConfig.Command = "kotlin-lsp"
		return true
	}
	if _, err := exec.LookPath("kotlin-lsp"); err == nil {
		k.serverConfig.Command = "kotlin-lsp"
		return true
	}

	// 3) Try known variants
	if _, err := k.RunCommandWithOutput(testCtx, "kotlin-lsp.sh", "--version"); err == nil {
		k.serverConfig.Command = "kotlin-lsp.sh"
		return true
	}
	if _, err := exec.LookPath("kotlin-lsp.sh"); err == nil {
		k.serverConfig.Command = "kotlin-lsp.sh"
		return true
	}

	if runtime.GOOS == "windows" {
		if _, err := k.RunCommandWithOutput(testCtx, "kotlin-lsp.cmd", "--version"); err == nil {
			k.serverConfig.Command = "kotlin-lsp.cmd"
			return true
		}
		if _, err := exec.LookPath("kotlin-lsp.cmd"); err == nil {
			k.serverConfig.Command = "kotlin-lsp.cmd"
			return true
		}
		if _, err := k.RunCommandWithOutput(testCtx, "kotlin-lsp.bat", "--version"); err == nil {
			k.serverConfig.Command = "kotlin-lsp.bat"
			return true
		}
		if _, err := exec.LookPath("kotlin-lsp.bat"); err == nil {
			k.serverConfig.Command = "kotlin-lsp.bat"
			return true
		}
		if _, err := k.RunCommandWithOutput(testCtx, "kotlin-lsp.exe", "--version"); err == nil {
			k.serverConfig.Command = "kotlin-lsp.exe"
			return true
		}
		if _, err := exec.LookPath("kotlin-lsp.exe"); err == nil {
			k.serverConfig.Command = "kotlin-lsp.exe"
			return true
		}
	}

	installPath := k.GetInstallPath()
	if cmd := common.FirstExistingExecutable(installPath, []string{"kotlin-lsp"}); cmd != "" {
		if _, err := k.RunCommandWithOutput(testCtx, cmd, "--version"); err == nil {
			k.serverConfig.Command = cmd
			return true
		}
		k.serverConfig.Command = cmd
		return true
	}

	return false
}

// IsInstalled checks if Kotlin LSP is properly installed and working
func (k *KotlinInstaller) IsInstalled() bool {
	// Prefer BaseInstaller logic which checks PATH and installPath and tolerates LSP semantics
	if k.BaseInstaller.IsInstalled() {
		return true
	}

	ctx, cancel := icommon.CreateContext(5 * time.Second)
	defer cancel()
	return k.testKotlinLSP(ctx)
}

// GetVersion returns the version of the installed Kotlin LSP
func (k *KotlinInstaller) GetVersion() (string, error) {
	if !k.IsInstalled() {
		return "", fmt.Errorf("kotlin language server not installed")
	}

	ctx, cancel := icommon.CreateContext(5 * time.Second)
	defer cancel()

	// Try to get version info
	if output, err := k.RunCommandWithOutput(ctx, k.serverConfig.Command, "--version"); err == nil {
		version := strings.TrimSpace(output)
		if version != "" {
			return version, nil
		}
	}

	return "kotlin-lsp (installed)", nil
}

// ValidateInstallation performs comprehensive validation
func (k *KotlinInstaller) ValidateInstallation() error {
	if !k.IsInstalled() {
		return fmt.Errorf("kotlin language server installation validation failed: not installed")
	}

	// Validate security
	if err := security.ValidateCommand(k.serverConfig.Command, k.serverConfig.Args); err != nil {
		return fmt.Errorf("security validation failed for kotlin: %w", err)
	}

	common.CLILogger.Info("Kotlin Language Server validation successful")
	return nil
}

// Uninstall removes the Kotlin LSP installation
func (k *KotlinInstaller) Uninstall() error {
	installPath := k.GetInstallPath()

	common.CLILogger.Info("Uninstalling Kotlin Language Server...")

	if err := os.RemoveAll(installPath); err != nil {
		return fmt.Errorf("failed to remove kotlin installation: %w", err)
	}

	common.CLILogger.Info("Kotlin Language Server uninstalled successfully")
	return nil
}
