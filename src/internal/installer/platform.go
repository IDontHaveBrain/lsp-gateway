package installer

import (
	"fmt"
	"runtime"
)

// LSPPlatformInfo implements PlatformInfo interface
type LSPPlatformInfo struct{}

// NewLSPPlatformInfo creates a new platform info instance
func NewLSPPlatformInfo() *LSPPlatformInfo {
	return &LSPPlatformInfo{}
}

// GetPlatform returns current platform (linux, darwin, windows)
func (p *LSPPlatformInfo) GetPlatform() string {
	return runtime.GOOS
}

// GetArch returns current architecture (amd64, arm64)
func (p *LSPPlatformInfo) GetArch() string {
	return runtime.GOARCH
}

// GetPlatformString returns platform string for downloads
func (p *LSPPlatformInfo) GetPlatformString() string {
	platform := p.GetPlatform()
	arch := p.GetArch()

	// Normalize architecture names
	switch arch {
	case "amd64":
		arch = "x64"
	case "arm64":
		arch = "arm64"
	default:
		arch = "x64" // Default fallback
	}

	// Normalize platform names
	switch platform {
	case "darwin":
		return fmt.Sprintf("darwin-%s", arch)
	case "linux":
		return fmt.Sprintf("linux-%s", arch)
	case "windows":
		return fmt.Sprintf("win32-%s", arch)
	default:
		return fmt.Sprintf("%s-%s", platform, arch)
	}
}

// IsSupported checks if current platform is supported
func (p *LSPPlatformInfo) IsSupported() bool {
	platform := p.GetPlatform()
	arch := p.GetArch()

	// Supported platforms
	supportedPlatforms := map[string][]string{
		"linux":   {"amd64", "arm64"},
		"darwin":  {"amd64", "arm64"},
		"windows": {"amd64"},
	}

	if supportedArchs, exists := supportedPlatforms[platform]; exists {
		for _, supportedArch := range supportedArchs {
			if arch == supportedArch {
				return true
			}
		}
	}

	return false
}

// GetJavaDownloadURL returns the appropriate JDK download URL for the platform
func (p *LSPPlatformInfo) GetJavaDownloadURL(version string) (string, string, error) {
	if !p.IsSupported() {
		return "", "", fmt.Errorf("platform %s-%s not supported for Java installation", p.GetPlatform(), p.GetArch())
	}

	// Use Eclipse Temurin Java 21 (required for current JDTLS)
	baseURL := "https://github.com/adoptium/temurin21-binaries/releases/download/jdk-21.0.4%2B7"

	platform := p.GetPlatform()
	arch := p.GetArch()

	var filename string
	var extractDir string

	switch platform {
	case "linux":
		if arch == "amd64" {
			filename = "OpenJDK21U-jdk_x64_linux_hotspot_21.0.4_7.tar.gz"
			extractDir = "jdk-21.0.4+7"
		} else if arch == "arm64" {
			filename = "OpenJDK21U-jdk_aarch64_linux_hotspot_21.0.4_7.tar.gz"
			extractDir = "jdk-21.0.4+7"
		} else {
			return "", "", fmt.Errorf("unsupported architecture for Linux: %s", arch)
		}
	case "darwin":
		if arch == "amd64" {
			filename = "OpenJDK21U-jdk_x64_mac_hotspot_21.0.4_7.tar.gz"
			extractDir = "jdk-21.0.4+7/Contents/Home"
		} else if arch == "arm64" {
			filename = "OpenJDK21U-jdk_aarch64_mac_hotspot_21.0.4_7.tar.gz"
			extractDir = "jdk-21.0.4+7/Contents/Home"
		} else {
			return "", "", fmt.Errorf("unsupported architecture for macOS: %s", arch)
		}
	case "windows":
		if arch == "amd64" {
			filename = "OpenJDK21U-jdk_x64_windows_hotspot_21.0.4_7.zip"
			extractDir = "jdk-21.0.4+7"
		} else {
			return "", "", fmt.Errorf("unsupported architecture for Windows: %s", arch)
		}
	default:
		return "", "", fmt.Errorf("unsupported platform: %s", platform)
	}

	downloadURL := fmt.Sprintf("%s/%s", baseURL, filename)
	return downloadURL, extractDir, nil
}

// GetNodeInstallCommand returns the command to install Node.js/npm if needed
func (p *LSPPlatformInfo) GetNodeInstallCommand() []string {
	platform := p.GetPlatform()

	switch platform {
	case "linux":
		// Try different package managers
		return []string{"apt-get", "update", "&&", "apt-get", "install", "-y", "nodejs", "npm"}
	case "darwin":
		// Assume Homebrew is available
		return []string{"brew", "install", "node"}
	case "windows":
		// Recommend manual installation
		return []string{"echo", "Please install Node.js from https://nodejs.org/"}
	default:
		return []string{"echo", "Platform not supported for automatic Node.js installation"}
	}
}
