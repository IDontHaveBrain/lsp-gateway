package installer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/security"
)

type CSharpInstaller struct {
	*BaseInstaller
}

func NewCSharpInstaller(platform PlatformInfo) *CSharpInstaller {
	base := CreateSimpleInstaller("csharp", "omnisharp", []string{"-lsp"}, platform)
	return &CSharpInstaller{BaseInstaller: base}
}

func (c *CSharpInstaller) Install(ctx context.Context, options InstallOptions) error {
	installBin := filepath.Join(c.GetInstallPath(), "bin")
	if err := c.CreateInstallDirectory(installBin); err != nil {
		return err
	}

	if c.IsInstalledByCommand("omnisharp") || c.IsInstalledByCommand("OmniSharp") {
		return c.linkFromPath(installBin)
	}

	url, err := c.resolveLatestAssetURL(ctx)
	if err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "omnisharp-download-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	filename := filepath.Base(url)
	archivePath := filepath.Join(tmpDir, filename)

	dlCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := c.DownloadFile(dlCtx, url, archivePath); err != nil {
		return err
	}

	if err := c.ExtractArchive(ctx, archivePath, installBin); err != nil {
		return err
	}

	// Find and ensure the OmniSharp binary is executable
	if err := c.ensureOmniSharpExecutable(installBin); err != nil {
		return fmt.Errorf("failed to setup OmniSharp binary: %w", err)
	}

	return nil
}

// IsInstalled checks if OmniSharp is properly installed
func (c *CSharpInstaller) IsInstalled() bool {
	installBin := filepath.Join(c.GetInstallPath(), "bin")

	// Check for various possible binary names including wrapper scripts
	var candidates []string
	if runtime.GOOS == "windows" {
		candidates = []string{"omnisharp.cmd", "omnisharp.exe", "OmniSharp.exe"}
	} else {
		candidates = []string{"omnisharp", "OmniSharp"}
	}

	// Check if any of the candidate binaries exist
	for _, candidate := range candidates {
		binaryPath := filepath.Join(installBin, candidate)
		if info, err := os.Stat(binaryPath); err == nil {
			// On Unix, also check if it's executable
			if runtime.GOOS == "windows" {
				return true
			}
			// Check if file has execute permission for owner
			if info.Mode()&0100 != 0 {
				return true
			}
		}
	}

	// Also check for OmniSharp.dll with dotnet wrapper
	dllPath := filepath.Join(installBin, "OmniSharp.dll")
	if _, err := os.Stat(dllPath); err == nil {
		// Check if wrapper script exists
		wrapperName := "omnisharp"
		if runtime.GOOS == "windows" {
			wrapperName = "omnisharp.cmd"
		}
		wrapperPath := filepath.Join(installBin, wrapperName)
		if _, err := os.Stat(wrapperPath); err == nil {
			return true
		}
	}

	// Also check if it's available in PATH
	for _, cmd := range []string{"omnisharp", "OmniSharp"} {
		if c.IsInstalledByCommand(cmd) {
			return true
		}
	}

	return false
}

func (c *CSharpInstaller) GetVersion() (string, error) {
	if !c.IsInstalled() {
		return "", fmt.Errorf("csharp language server not installed")
	}
	return "OmniSharp-Roslyn LSP (installed)", nil
}

// ValidateInstallation performs post-install validation for C#
func (c *CSharpInstaller) ValidateInstallation() error {
	if !c.IsInstalled() {
		return fmt.Errorf("csharp language server installation validation failed: not installed")
	}

	// Find the actual executable path and update server config
	installBin := filepath.Join(c.GetInstallPath(), "bin")
	var executablePath string

	// Check for wrapper script first (for .dll installations)
	if runtime.GOOS == "windows" {
		wrapperPath := filepath.Join(installBin, "omnisharp.cmd")
		if _, err := os.Stat(wrapperPath); err == nil {
			executablePath = wrapperPath
		}
	} else {
		wrapperPath := filepath.Join(installBin, "omnisharp")
		if _, err := os.Stat(wrapperPath); err == nil {
			executablePath = wrapperPath
		}
	}

	// If no wrapper, look for native binary
	if executablePath == "" {
		var candidates []string
		if runtime.GOOS == "windows" {
			candidates = []string{"OmniSharp.exe", "omnisharp.exe"}
		} else {
			candidates = []string{"OmniSharp", "omnisharp"}
		}

		for _, candidate := range candidates {
			path := filepath.Join(installBin, candidate)
			if _, err := os.Stat(path); err == nil {
				executablePath = path
				break
			}
		}
	}

	if executablePath == "" {
		return fmt.Errorf("csharp language server executable not found")
	}

	// Update server config to use the actual path
	if c.serverConfig != nil {
		c.serverConfig.Command = executablePath
	}

	// Validate security
	args := []string{"-lsp"}
	if c.serverConfig != nil {
		args = c.serverConfig.Args
	}
	if err := security.ValidateCommand(executablePath, args); err != nil {
		return fmt.Errorf("security validation failed for csharp: %w", err)
	}

	common.CLILogger.Info("Installation validation successful for omnisharp")
	common.CLILogger.Info("C# validation successful")
	return nil
}

func (c *CSharpInstaller) Uninstall() error {
	installBin := filepath.Join(c.GetInstallPath(), "bin")
	bin := filepath.Join(installBin, "omnisharp")
	if runtime.GOOS == "windows" {
		bin = bin + ".exe"
	}
	_ = os.Remove(bin)
	return nil
}

func (c *CSharpInstaller) linkFromPath(installBin string) error {
	var found string
	var err error
	found, err = exec.LookPath("omnisharp")
	if err != nil {
		found, err = exec.LookPath("OmniSharp")
		if err != nil {
			return fmt.Errorf("unable to locate OmniSharp in PATH despite detection")
		}
	}
	target := filepath.Join(installBin, "omnisharp")
	if runtime.GOOS == "windows" {
		target = target + ".exe"
	}
	if _, err := os.Stat(target); err == nil {
		return nil
	}
	if runtime.GOOS != "windows" {
		if err := os.Symlink(found, target); err == nil {
			return nil
		}
	}
	return copyFile(found, target)
}

func (c *CSharpInstaller) resolveLatestAssetURL(ctx context.Context) (string, error) {
	platform := runtime.GOOS
	arch := runtime.GOARCH
	plat := ""
	ext := ""
	switch platform {
	case "linux":
		if isMusl() {
			plat = "linux-musl"
		} else {
			plat = "linux"
		}
		ext = ".tar.gz"
	case "darwin":
		plat = "osx"
		ext = ".zip"
	case "windows":
		plat = "win"
		ext = ".zip"
	default:
		return "", fmt.Errorf("unsupported platform: %s", platform)
	}

	archMap := map[string]string{
		"amd64": "x64",
		"arm64": "arm64",
		"386":   "x86",
	}
	archTag, ok := archMap[arch]
	if !ok {
		return "", fmt.Errorf("unsupported architecture: %s", arch)
	}

	apiURL := "https://api.github.com/repos/OmniSharp/omnisharp-roslyn/releases/latest"
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
	var data struct {
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	var candidates []string
	base := fmt.Sprintf("omnisharp-%s-%s", plat, archTag)
	candidates = append(candidates, base+ext)
	candidates = append(candidates, base+"-net6.0"+ext)
	if platform == "darwin" {
		candidates = append(candidates, "omnisharp-osx"+ext)
	}
	for _, asset := range data.Assets {
		for _, cand := range candidates {
			if strings.EqualFold(asset.Name, cand) {
				return asset.URL, nil
			}
		}
	}
	for _, asset := range data.Assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, plat) && strings.Contains(name, archTag) && strings.HasSuffix(name, strings.ToLower(ext)) && !strings.Contains(name, ".http-") {
			return asset.URL, nil
		}
	}
	return "", errors.New("no suitable OmniSharp asset found")
}

func (c *CSharpInstaller) findOmniSharpBinary(root string) (string, error) {
	var candidates []string
	if runtime.GOOS == "windows" {
		candidates = []string{"omnisharp.exe", "OmniSharp.exe"}
	} else {
		candidates = []string{"omnisharp", "OmniSharp"}
	}
	var match string
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name := filepath.Base(path)
		for _, cnd := range candidates {
			if strings.EqualFold(name, cnd) {
				match = path
				return errors.New("found")
			}
		}
		return nil
	})
	if match == "" {
		return "", errors.New("OmniSharp binary not found in archive")
	}
	return match, nil
}

// copyDirContents copies all files from srcDir into dstDir recursively
func copyDirContents(srcDir, dstDir string) error {
	return filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dstDir, rel)
		if d.IsDir() {
			if rel == "." {
				return nil
			}
			return os.MkdirAll(target, 0755)
		}
		// Ensure parent exists
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}

// ensureOmniSharpExecutable finds the OmniSharp binary and ensures it's executable
func (c *CSharpInstaller) ensureOmniSharpExecutable(installBin string) error {
	var foundBinary string
	var candidates []string

	if runtime.GOOS == "windows" {
		candidates = []string{"OmniSharp.exe", "omnisharp.exe", "OmniSharp.dll"}
	} else {
		candidates = []string{"OmniSharp", "omnisharp", "OmniSharp.dll"}
	}

	// First check in the root of installBin
	for _, candidate := range candidates {
		path := filepath.Join(installBin, candidate)
		if _, err := os.Stat(path); err == nil {
			foundBinary = path
			break
		}
	}

	// If not found, search recursively
	if foundBinary == "" {
		err := filepath.WalkDir(installBin, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			name := d.Name()
			for _, candidate := range candidates {
				if strings.EqualFold(name, candidate) {
					foundBinary = path
					return filepath.SkipAll
				}
			}
			return nil
		})
		if err != nil && err != filepath.SkipAll {
			return fmt.Errorf("error searching for OmniSharp binary: %w", err)
		}
	}

	if foundBinary == "" {
		return fmt.Errorf("OmniSharp binary not found after extraction")
	}

	common.CLILogger.Info("Found OmniSharp binary at: %s", foundBinary)

	// For .dll files, we need to create a wrapper script
	if strings.HasSuffix(foundBinary, ".dll") {
		return c.createDotNetWrapper(foundBinary, installBin)
	}

	// For native binaries, ensure they're executable
	if runtime.GOOS != "windows" {
		if err := os.Chmod(foundBinary, 0755); err != nil {
			return fmt.Errorf("failed to make binary executable: %w", err)
		}

		// Create a symlink with lowercase name if the binary has uppercase name
		baseName := filepath.Base(foundBinary)
		if baseName == "OmniSharp" {
			symlinkPath := filepath.Join(installBin, "omnisharp")
			// Remove existing symlink if it exists
			os.Remove(symlinkPath)
			// Create relative symlink
			if err := os.Symlink(baseName, symlinkPath); err != nil {
				common.CLILogger.Warn("Failed to create omnisharp symlink: %v", err)
			}
		}
	}

	return nil
}

// createDotNetWrapper creates a wrapper script for .dll based OmniSharp
func (c *CSharpInstaller) createDotNetWrapper(dllPath string, installBin string) error {
	wrapperPath := filepath.Join(installBin, "omnisharp")

	if runtime.GOOS == "windows" {
		wrapperPath += ".cmd"
		content := fmt.Sprintf(`@echo off
dotnet "%s" %%*
`, dllPath)
		return os.WriteFile(wrapperPath, []byte(content), 0755)
	} else {
		content := fmt.Sprintf(`#!/bin/sh
exec dotnet "%s" "$@"
`, dllPath)
		if err := os.WriteFile(wrapperPath, []byte(content), 0755); err != nil {
			return err
		}
		// Also create OmniSharp symlink
		symlinkPath := filepath.Join(installBin, "OmniSharp")
		os.Remove(symlinkPath)
		return os.Symlink("omnisharp", symlinkPath)
	}
}

func isMusl() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	if _, err := os.Stat("/etc/alpine-release"); err == nil {
		return true
	}
	out, err := exec.Command("sh", "-c", "ldd --version 2>&1").CombinedOutput()
	if err == nil && strings.Contains(strings.ToLower(string(out)), "musl") {
		return true
	}
	if _, err := os.Stat("/lib/ld-musl-x86_64.so.1"); err == nil {
		return true
	}
	return false
}
