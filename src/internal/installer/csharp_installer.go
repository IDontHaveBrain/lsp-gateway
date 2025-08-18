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
	// Ensure executable bit on Linux/macOS
	if runtime.GOOS != "windows" {
		// Set permissions on the actual binary that was extracted
		omnisharpPath := filepath.Join(installBin, "omnisharp")
		OmniSharpPath := filepath.Join(installBin, "OmniSharp")
		
		// Check which binary exists and set permissions
		if _, err := os.Stat(omnisharpPath); err == nil {
			os.Chmod(omnisharpPath, 0755)
		}
		if _, err := os.Stat(OmniSharpPath); err == nil {
			os.Chmod(OmniSharpPath, 0755)
		}
		
		// Also check for the actual omnisharp binary in the extraction
		// Some archives contain the binary directly, others in subdirectories
		if err := filepath.WalkDir(installBin, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			name := d.Name()
			if name == "omnisharp" || name == "OmniSharp" {
				os.Chmod(path, 0755)
			}
			return nil
		}); err != nil {
			// Ignore walk errors
		}
	}
	return nil
}

// IsInstalled checks if OmniSharp is properly installed
func (c *CSharpInstaller) IsInstalled() bool {
	installBin := filepath.Join(c.GetInstallPath(), "bin")
	
	// Check for various possible binary names
	candidates := []string{"omnisharp", "OmniSharp"}
	if runtime.GOOS == "windows" {
		for i, name := range candidates {
			candidates[i] = name + ".exe"
		}
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
