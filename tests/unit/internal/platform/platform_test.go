package platform_test

import (
	"lsp-gateway/internal/platform"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestGetCurrentPlatform(t *testing.T) {
	currentPlatform := platform.GetCurrentPlatform()

	switch runtime.GOOS {
	case string(platform.PlatformWindows):
		if currentPlatform != platform.PlatformWindows {
			t.Errorf("Expected platform.PlatformWindows, got %s", currentPlatform)
		}
	case string(platform.PlatformLinux):
		if currentPlatform != platform.PlatformLinux {
			t.Errorf("Expected platform.PlatformLinux, got %s", currentPlatform)
		}
	case string(platform.PlatformMacOS):
		if currentPlatform != platform.PlatformMacOS {
			t.Errorf("Expected platform.PlatformMacOS, got %s", currentPlatform)
		}
	default:
		if currentPlatform != platform.PlatformUnknown {
			t.Errorf("Expected platform.PlatformUnknown for unsupported OS, got %s", currentPlatform)
		}
	}
}

func TestGetCurrentArchitecture(t *testing.T) {
	arch := platform.GetCurrentArchitecture()

	switch runtime.GOARCH {
	case string(platform.ArchAMD64):
		if arch != platform.ArchAMD64 {
			t.Errorf("Expected platform.ArchAMD64, got %s", arch)
		}
	case string(platform.ArchARM64):
		if arch != platform.ArchARM64 {
			t.Errorf("Expected platform.ArchARM64, got %s", arch)
		}
	case string(platform.Arch386):
		if arch != platform.Arch386 {
			t.Errorf("Expected platform.Arch386, got %s", arch)
		}
	case string(platform.ArchARM):
		if arch != platform.ArchARM {
			t.Errorf("Expected platform.ArchARM, got %s", arch)
		}
	default:
		if arch != platform.ArchUnknown {
			t.Errorf("Expected platform.ArchUnknown for unsupported architecture, got %s", arch)
		}
	}
}

func TestPlatformBooleanFunctions(t *testing.T) {
	currentPlatform := platform.GetCurrentPlatform()

	switch currentPlatform {
	case platform.PlatformWindows:
		if !platform.IsWindows() {
			t.Error("platform.IsWindows() should return true on Windows")
		}
		if platform.IsLinux() {
			t.Error("platform.IsLinux() should return false on Windows")
		}
		if platform.IsMacOS() {
			t.Error("platform.IsMacOS() should return false on Windows")
		}
		if platform.IsUnix() {
			t.Error("platform.IsUnix() should return false on Windows")
		}

	case platform.PlatformLinux:
		if platform.IsWindows() {
			t.Error("platform.IsWindows() should return false on Linux")
		}
		if !platform.IsLinux() {
			t.Error("platform.IsLinux() should return true on Linux")
		}
		if platform.IsMacOS() {
			t.Error("platform.IsMacOS() should return false on Linux")
		}
		if !platform.IsUnix() {
			t.Error("platform.IsUnix() should return true on Linux")
		}

	case platform.PlatformMacOS:
		if platform.IsWindows() {
			t.Error("platform.IsWindows() should return false on macOS")
		}
		if platform.IsLinux() {
			t.Error("platform.IsLinux() should return false on macOS")
		}
		if !platform.IsMacOS() {
			t.Error("platform.IsMacOS() should return true on macOS")
		}
		if !platform.IsUnix() {
			t.Error("platform.IsUnix() should return true on macOS")
		}
	}
}

func TestGetHomeDirectory(t *testing.T) {
	home, err := platform.GetHomeDirectory()
	if err != nil {
		t.Fatalf("GetHomeDirectory failed: %v", err)
	}

	if home == "" {
		t.Error("GetHomeDirectory returned empty string")
	}

	if _, err := os.Stat(home); os.IsNotExist(err) {
		t.Errorf("Home directory %s does not exist", home)
	}
}

func TestGetTempDirectory(t *testing.T) {
	temp := platform.GetTempDirectory()

	if temp == "" {
		t.Error("GetTempDirectory returned empty string")
	}

	if _, err := os.Stat(temp); os.IsNotExist(err) {
		t.Errorf("Temp directory %s does not exist", temp)
	}
}

func TestGetExecutableExtension(t *testing.T) {
	ext := platform.GetExecutableExtension()

	if platform.IsWindows() {
		if ext != ".exe" {
			t.Errorf("Expected .exe extension on Windows, got %s", ext)
		}
	} else {
		if ext != "" {
			t.Errorf("Expected empty extension on Unix-like systems, got %s", ext)
		}
	}
}

func TestGetPathSeparator(t *testing.T) {
	sep := platform.GetPathSeparator()

	if platform.IsWindows() {
		if sep != "\\" {
			t.Errorf("Expected \\\\ path separator on Windows, got %s", sep)
		}
	} else {
		if sep != "/" {
			t.Errorf("Expected / path separator on Unix-like systems, got %s", sep)
		}
	}
}

func TestGetPathListSeparator(t *testing.T) {
	sep := platform.GetPathListSeparator()

	if platform.IsWindows() {
		if sep != ";" {
			t.Errorf("Expected ; path list separator on Windows, got %s", sep)
		}
	} else {
		if sep != ":" {
			t.Errorf("Expected : path list separator on Unix-like systems, got %s", sep)
		}
	}
}

func TestGetPlatformString(t *testing.T) {
	platformStr := platform.GetPlatformString()

	if platformStr == "" {
		t.Error("GetPlatformString returned empty string")
	}

	expectedPlatform := platform.GetCurrentPlatform().String()
	expectedArch := platform.GetCurrentArchitecture().String()

	if !strings.Contains(platformStr, expectedPlatform) {
		t.Errorf("Platform string should contain %s, got %s", expectedPlatform, platformStr)
	}

	if !strings.Contains(platformStr, expectedArch) {
		t.Errorf("Platform string should contain %s, got %s", expectedArch, platformStr)
	}
}

func TestSupportsShell(t *testing.T) {
	testCases := []struct {
		shell    string
		expected bool
	}{
		{"cmd", platform.IsWindows()},
		{"cmd.exe", platform.IsWindows()},
		{"powershell", platform.IsWindows()},
		{"powershell.exe", platform.IsWindows()},
		{"pwsh", platform.IsWindows()},
		{"pwsh.exe", platform.IsWindows()},

		{"sh", platform.IsUnix()},
		{"bash", platform.IsUnix()},
		{"zsh", platform.IsUnix()},
		{"dash", platform.IsUnix()},
		{"ksh", platform.IsUnix()},
		{"fish", platform.IsUnix()},

		{"CMD", platform.IsWindows()},
		{"BASH", platform.IsUnix()},

		{"nonexistent_shell", false},
	}

	for _, tc := range testCases {
		result := platform.SupportsShell(tc.shell)
		if result != tc.expected {
			t.Errorf("platform.SupportsShell(%s): expected %v, got %v", tc.shell, tc.expected, result)
		}
	}
}

func TestPlatformAndArchitectureStrings(t *testing.T) {
	currentPlatform := platform.GetCurrentPlatform()
	arch := platform.GetCurrentArchitecture()

	platformStr := currentPlatform.String()
	if platformStr == "" {
		t.Error("Platform.String() returned empty string")
	}

	archStr := arch.String()
	if archStr == "" {
		t.Error("Architecture.String() returned empty string")
	}
}
