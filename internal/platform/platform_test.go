package platform

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestGetCurrentPlatform(t *testing.T) {
	platform := GetCurrentPlatform()

	switch runtime.GOOS {
	case string(PlatformWindows):
		if platform != PlatformWindows {
			t.Errorf("Expected PlatformWindows, got %s", platform)
		}
	case string(PlatformLinux):
		if platform != PlatformLinux {
			t.Errorf("Expected PlatformLinux, got %s", platform)
		}
	case string(PlatformMacOS):
		if platform != PlatformMacOS {
			t.Errorf("Expected PlatformMacOS, got %s", platform)
		}
	default:
		if platform != PlatformUnknown {
			t.Errorf("Expected PlatformUnknown for unsupported OS, got %s", platform)
		}
	}
}

func TestGetCurrentArchitecture(t *testing.T) {
	arch := GetCurrentArchitecture()

	switch runtime.GOARCH {
	case string(ArchAMD64):
		if arch != ArchAMD64 {
			t.Errorf("Expected ArchAMD64, got %s", arch)
		}
	case string(ArchARM64):
		if arch != ArchARM64 {
			t.Errorf("Expected ArchARM64, got %s", arch)
		}
	case string(Arch386):
		if arch != Arch386 {
			t.Errorf("Expected Arch386, got %s", arch)
		}
	case string(ArchARM):
		if arch != ArchARM {
			t.Errorf("Expected ArchARM, got %s", arch)
		}
	default:
		if arch != ArchUnknown {
			t.Errorf("Expected ArchUnknown for unsupported architecture, got %s", arch)
		}
	}
}

func TestPlatformBooleanFunctions(t *testing.T) {
	currentPlatform := GetCurrentPlatform()

	switch currentPlatform {
	case PlatformWindows:
		if !IsWindows() {
			t.Error("IsWindows() should return true on Windows")
		}
		if IsLinux() {
			t.Error("IsLinux() should return false on Windows")
		}
		if IsMacOS() {
			t.Error("IsMacOS() should return false on Windows")
		}
		if IsUnix() {
			t.Error("IsUnix() should return false on Windows")
		}

	case PlatformLinux:
		if IsWindows() {
			t.Error("IsWindows() should return false on Linux")
		}
		if !IsLinux() {
			t.Error("IsLinux() should return true on Linux")
		}
		if IsMacOS() {
			t.Error("IsMacOS() should return false on Linux")
		}
		if !IsUnix() {
			t.Error("IsUnix() should return true on Linux")
		}

	case PlatformMacOS:
		if IsWindows() {
			t.Error("IsWindows() should return false on macOS")
		}
		if IsLinux() {
			t.Error("IsLinux() should return false on macOS")
		}
		if !IsMacOS() {
			t.Error("IsMacOS() should return true on macOS")
		}
		if !IsUnix() {
			t.Error("IsUnix() should return true on macOS")
		}
	}
}

func TestGetHomeDirectory(t *testing.T) {
	home, err := GetHomeDirectory()
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
	temp := GetTempDirectory()

	if temp == "" {
		t.Error("GetTempDirectory returned empty string")
	}

	if _, err := os.Stat(temp); os.IsNotExist(err) {
		t.Errorf("Temp directory %s does not exist", temp)
	}
}

func TestGetExecutableExtension(t *testing.T) {
	ext := GetExecutableExtension()

	if IsWindows() {
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
	sep := GetPathSeparator()

	if IsWindows() {
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
	sep := GetPathListSeparator()

	if IsWindows() {
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
	platformStr := GetPlatformString()

	if platformStr == "" {
		t.Error("GetPlatformString returned empty string")
	}

	expectedPlatform := GetCurrentPlatform().String()
	expectedArch := GetCurrentArchitecture().String()

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
		{"cmd", IsWindows()},
		{"cmd.exe", IsWindows()},
		{"powershell", IsWindows()},
		{"powershell.exe", IsWindows()},
		{"pwsh", IsWindows()},
		{"pwsh.exe", IsWindows()},

		{"sh", IsUnix()},
		{"bash", IsUnix()},
		{"zsh", IsUnix()},
		{"dash", IsUnix()},
		{"ksh", IsUnix()},
		{"fish", IsUnix()},

		{"CMD", IsWindows()},
		{"BASH", IsUnix()},

		{"nonexistent_shell", false},
	}

	for _, tc := range testCases {
		result := SupportsShell(tc.shell)
		if result != tc.expected {
			t.Errorf("SupportsShell(%s): expected %v, got %v", tc.shell, tc.expected, result)
		}
	}
}

func TestPlatformAndArchitectureStrings(t *testing.T) {
	platform := GetCurrentPlatform()
	arch := GetCurrentArchitecture()

	platformStr := platform.String()
	if platformStr == "" {
		t.Error("Platform.String() returned empty string")
	}

	archStr := arch.String()
	if archStr == "" {
		t.Error("Architecture.String() returned empty string")
	}
}
