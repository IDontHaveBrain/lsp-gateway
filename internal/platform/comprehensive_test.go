package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Test parseOSRelease function with various valid inputs
func TestParseOSRelease(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]string
		expected LinuxInfo
	}{
		{
			name: "Ubuntu",
			data: map[string]string{
				"ID":         "ubuntu",
				"NAME":       "Ubuntu",
				"VERSION_ID": "20.04",
				"ID_LIKE":    "debian",
			},
			expected: LinuxInfo{
				Distribution: DistributionUbuntu,
				ID:           "ubuntu",
				Name:         "Ubuntu",
				Version:      "20.04",
				IDLike:       []string{"debian"},
			},
		},
		{
			name: "Fedora",
			data: map[string]string{
				"ID":         "fedora",
				"NAME":       "Fedora Linux",
				"VERSION_ID": "35",
			},
			expected: LinuxInfo{
				Distribution: DistributionFedora,
				ID:           "fedora",
				Name:         "Fedora Linux",
				Version:      "35",
			},
		},
		{
			name: "CentOS",
			data: map[string]string{
				"ID":         "centos",
				"NAME":       "CentOS Linux",
				"VERSION_ID": "8",
				"ID_LIKE":    "rhel fedora",
			},
			expected: LinuxInfo{
				Distribution: DistributionCentOS,
				ID:           "centos",
				Name:         "CentOS Linux",
				Version:      "8",
				IDLike:       []string{"rhel", "fedora"},
			},
		},
		{
			name: "Unknown Distribution",
			data: map[string]string{
				"ID":         "unknownos",
				"NAME":       "Unknown OS",
				"VERSION_ID": "1.0",
			},
			expected: LinuxInfo{
				Distribution: DistributionUnknown,
				ID:           "unknownos",
				Name:         "Unknown OS",
				Version:      "1.0",
			},
		},
		{
			name: "Empty data",
			data: map[string]string{},
			expected: LinuxInfo{
				Distribution: DistributionUnknown,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &LinuxInfo{Distribution: DistributionUnknown}
			parseOSRelease(tt.data, info)

			if info.Distribution != tt.expected.Distribution {
				t.Errorf("Distribution: expected %s, got %s", tt.expected.Distribution, info.Distribution)
			}
			if info.ID != tt.expected.ID {
				t.Errorf("ID: expected %s, got %s", tt.expected.ID, info.ID)
			}
			if info.Name != tt.expected.Name {
				t.Errorf("Name: expected %s, got %s", tt.expected.Name, info.Name)
			}
			if info.Version != tt.expected.Version {
				t.Errorf("Version: expected %s, got %s", tt.expected.Version, info.Version)
			}
			if len(info.IDLike) != len(tt.expected.IDLike) {
				t.Errorf("IDLike length: expected %d, got %d", len(tt.expected.IDLike), len(info.IDLike))
			} else {
				for i, like := range tt.expected.IDLike {
					if info.IDLike[i] != like {
						t.Errorf("IDLike[%d]: expected %s, got %s", i, like, info.IDLike[i])
					}
				}
			}
		})
	}
}

// Test parseLSBRelease function with various valid inputs
func TestParseLSBRelease(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]string
		expected LinuxInfo
	}{
		{
			name: "Ubuntu LSB",
			data: map[string]string{
				"DISTRIB_ID":          "Ubuntu",
				"DISTRIB_RELEASE":     "20.04",
				"DISTRIB_DESCRIPTION": "Ubuntu 20.04.3 LTS",
			},
			expected: LinuxInfo{
				Distribution: DistributionUbuntu,
				ID:           "ubuntu",
				Name:         "Ubuntu 20.04.3 LTS",
				Version:      "20.04",
			},
		},
		{
			name: "Debian LSB",
			data: map[string]string{
				"DISTRIB_ID":          "Debian",
				"DISTRIB_RELEASE":     "11",
				"DISTRIB_DESCRIPTION": "Debian GNU/Linux 11 (bullseye)",
			},
			expected: LinuxInfo{
				Distribution: DistributionDebian,
				ID:           "debian",
				Name:         "Debian GNU/Linux 11 (bullseye)",
				Version:      "11",
			},
		},
		{
			name: "Mixed case LSB",
			data: map[string]string{
				"DISTRIB_ID":          "RHEL",
				"DISTRIB_RELEASE":     "8.5",
				"DISTRIB_DESCRIPTION": "Red Hat Enterprise Linux",
			},
			expected: LinuxInfo{
				Distribution: DistributionRHEL,
				ID:           "rhel",
				Name:         "Red Hat Enterprise Linux",
				Version:      "8.5",
			},
		},
		{
			name: "Empty LSB data",
			data: map[string]string{},
			expected: LinuxInfo{
				Distribution: DistributionUnknown,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &LinuxInfo{Distribution: DistributionUnknown}
			parseLSBRelease(tt.data, info)

			if info.Distribution != tt.expected.Distribution {
				t.Errorf("Distribution: expected %s, got %s", tt.expected.Distribution, info.Distribution)
			}
			if info.ID != tt.expected.ID {
				t.Errorf("ID: expected %s, got %s", tt.expected.ID, info.ID)
			}
			if info.Name != tt.expected.Name {
				t.Errorf("Name: expected %s, got %s", tt.expected.Name, info.Name)
			}
			if info.Version != tt.expected.Version {
				t.Errorf("Version: expected %s, got %s", tt.expected.Version, info.Version)
			}
		})
	}
}

// Test mapIDToDistribution function comprehensively
func TestMapIDToDistribution(t *testing.T) {
	tests := []struct {
		id       string
		expected LinuxDistribution
	}{
		{"ubuntu", DistributionUbuntu},
		{"UBUNTU", DistributionUbuntu},
		{"Ubuntu", DistributionUbuntu},
		{"debian", DistributionDebian},
		{"DEBIAN", DistributionDebian},
		{"fedora", DistributionFedora},
		{"FEDORA", DistributionFedora},
		{"centos", DistributionCentOS},
		{"CENTOS", DistributionCentOS},
		{"rhel", DistributionRHEL},
		{"RHEL", DistributionRHEL},
		{"red", DistributionRHEL},
		{"RED", DistributionRHEL},
		{"redhat", DistributionRHEL},
		{"REDHAT", DistributionRHEL},
		{"arch", DistributionArch},
		{"ARCH", DistributionArch},
		{"opensuse", DistributionOpenSUSE},
		{"OPENSUSE", DistributionOpenSUSE},
		{"suse", DistributionOpenSUSE},
		{"SUSE", DistributionOpenSUSE},
		{"alpine", DistributionAlpine},
		{"ALPINE", DistributionAlpine},
		{"unknown", DistributionUnknown},
		{"xyz", DistributionUnknown},
		{"", DistributionUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			result := mapIDToDistribution(tt.id)
			if result != tt.expected {
				t.Errorf("mapIDToDistribution(%s): expected %s, got %s", tt.id, tt.expected, result)
			}
		})
	}
}

// Test GetPreferredPackageManagers for all distributions
func TestGetPreferredPackageManagers(t *testing.T) {
	tests := []struct {
		distribution LinuxDistribution
		expected     []string
	}{
		{DistributionUbuntu, []string{"apt"}},
		{DistributionDebian, []string{"apt"}},
		{DistributionFedora, []string{"dnf"}},
		{DistributionCentOS, []string{"dnf", "yum"}},
		{DistributionRHEL, []string{"dnf", "yum"}},
		{DistributionArch, []string{"pacman"}},
		{DistributionOpenSUSE, []string{"zypper"}},
		{DistributionAlpine, []string{"apk"}},
		{DistributionUnknown, []string{"apt", "dnf", "yum"}},
		{LinuxDistribution("nonexistent"), []string{"apt", "dnf", "yum"}},
	}

	for _, tt := range tests {
		t.Run(string(tt.distribution), func(t *testing.T) {
			result := GetPreferredPackageManagers(tt.distribution)
			if len(result) != len(tt.expected) {
				t.Errorf("Length mismatch: expected %d, got %d", len(tt.expected), len(result))
				return
			}
			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("Index %d: expected %s, got %s", i, expected, result[i])
				}
			}
		})
	}
}

// Test readReleaseFile function with mock files
func TestReadReleaseFile(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "platform_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name        string
		content     string
		expected    map[string]string
		expectError bool
	}{
		{
			name: "Valid os-release",
			content: `NAME="Ubuntu"
VERSION="20.04.3 LTS (Focal Fossa)"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 20.04.3 LTS"
VERSION_ID="20.04"
HOME_URL="https://www.ubuntu.com/"
SUPPORT_URL="https://help.ubuntu.com/"
BUG_REPORT_URL="https://bugs.launchpad.net/ubuntu/"
PRIVACY_POLICY_URL="https://www.ubuntu.com/legal/terms-and-policies/privacy-policy"
VERSION_CODENAME=focal
UBUNTU_CODENAME=focal
`,
			expected: map[string]string{
				"NAME":                 "Ubuntu",
				"VERSION":              "20.04.3 LTS (Focal Fossa)",
				"ID":                   "ubuntu",
				"ID_LIKE":              "debian",
				"PRETTY_NAME":          "Ubuntu 20.04.3 LTS",
				"VERSION_ID":           "20.04",
				"HOME_URL":             "https://www.ubuntu.com/",
				"SUPPORT_URL":          "https://help.ubuntu.com/",
				"BUG_REPORT_URL":       "https://bugs.launchpad.net/ubuntu/",
				"PRIVACY_POLICY_URL":   "https://www.ubuntu.com/legal/terms-and-policies/privacy-policy",
				"VERSION_CODENAME":     "focal",
				"UBUNTU_CODENAME":      "focal",
			},
			expectError: false,
		},
		{
			name: "File with comments and empty lines",
			content: `# This is a comment
ID=test
# Another comment

NAME="Test OS"
# Final comment`,
			expected: map[string]string{
				"ID":   "test",
				"NAME": "Test OS",
			},
			expectError: false,
		},
		{
			name: "File with unquoted values",
			content: `ID=ubuntu
VERSION_ID=20.04
NAME=Ubuntu`,
			expected: map[string]string{
				"ID":         "ubuntu",
				"VERSION_ID": "20.04",
				"NAME":       "Ubuntu",
			},
			expectError: false,
		},
		{
			name: "File with invalid lines",
			content: `ID=ubuntu
INVALID_LINE_WITHOUT_EQUALS
NAME="Ubuntu"
ANOTHER_INVALID=`,
			expected: map[string]string{
				"ID":             "ubuntu",
				"NAME":           "Ubuntu",
				"ANOTHER_INVALID": "",
			},
			expectError: false,
		},
		{
			name:        "Empty file",
			content:     "",
			expected:    map[string]string{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			testFile := filepath.Join(tempDir, "test_"+strings.ReplaceAll(tt.name, " ", "_"))
			err := os.WriteFile(testFile, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// Test the function
			result, err := readReleaseFile(testFile)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Check results
			if len(result) != len(tt.expected) {
				t.Errorf("Length mismatch: expected %d, got %d", len(tt.expected), len(result))
			}

			for key, expectedValue := range tt.expected {
				if result[key] != expectedValue {
					t.Errorf("Key %s: expected '%s', got '%s'", key, expectedValue, result[key])
				}
			}
		})
	}

	// Test error case - non-existent file
	t.Run("Non-existent file", func(t *testing.T) {
		_, err := readReleaseFile("/path/that/does/not/exist")
		if err == nil {
			t.Error("Expected error for non-existent file")
		}
	})
}

// Test DetectLinuxDistribution error scenarios
func TestDetectLinuxDistributionErrors(t *testing.T) {
	// This test needs to run on Linux to be meaningful
	if !IsLinux() {
		t.Skip("Skipping Linux-specific test on non-Linux platform")
	}

	// Test the function with current system (should not error)
	info, err := DetectLinuxDistribution()
	if err != nil {
		t.Errorf("DetectLinuxDistribution failed on valid Linux system: %v", err)
	}
	if info == nil {
		t.Error("DetectLinuxDistribution returned nil info")
	}
	if info.Distribution == DistributionUnknown {
		t.Logf("Warning: Could not detect specific distribution, got %s", info.Distribution)
	}
}

// Test GetHomeDirectory error scenarios
func TestGetHomeDirectoryErrors(t *testing.T) {
	// Save original environment
	originalHome := os.Getenv("HOME")
	originalUserProfile := os.Getenv("USERPROFILE")
	originalHomeDrive := os.Getenv("HOMEDRIVE")
	originalHomePath := os.Getenv("HOMEPATH")

	defer func() {
		// Restore environment
		os.Setenv("HOME", originalHome)
		os.Setenv("USERPROFILE", originalUserProfile)
		os.Setenv("HOMEDRIVE", originalHomeDrive)
		os.Setenv("HOMEPATH", originalHomePath)
	}()

	if !IsWindows() {
		// Test Unix-like systems
		t.Run("Unix without HOME", func(t *testing.T) {
			os.Unsetenv("HOME")
			_, err := GetHomeDirectory()
			if err == nil {
				t.Error("Expected error when HOME is not set on Unix")
			}
		})
	} else {
		// Test Windows systems
		t.Run("Windows without USERPROFILE or HOMEDRIVE/HOMEPATH", func(t *testing.T) {
			os.Unsetenv("USERPROFILE")
			os.Unsetenv("HOMEDRIVE")
			os.Unsetenv("HOMEPATH")
			_, err := GetHomeDirectory()
			if err == nil {
				t.Error("Expected error when no Windows home variables are set")
			}
		})

		t.Run("Windows with only HOMEDRIVE", func(t *testing.T) {
			os.Unsetenv("USERPROFILE")
			os.Setenv("HOMEDRIVE", "C:")
			os.Unsetenv("HOMEPATH")
			_, err := GetHomeDirectory()
			if err == nil {
				t.Error("Expected error when only HOMEDRIVE is set")
			}
		})

		t.Run("Windows with only HOMEPATH", func(t *testing.T) {
			os.Unsetenv("USERPROFILE")
			os.Unsetenv("HOMEDRIVE")
			os.Setenv("HOMEPATH", "\\Users\\test")
			_, err := GetHomeDirectory()
			if err == nil {
				t.Error("Expected error when only HOMEPATH is set")
			}
		})
	}
}

// Test GetTempDirectory with various environment variables
func TestGetTempDirectoryVariations(t *testing.T) {
	// Save original environment
	originalTmpdir := os.Getenv("TMPDIR")
	originalTmp := os.Getenv("TMP")
	originalTemp := os.Getenv("TEMP")

	defer func() {
		// Restore environment
		os.Setenv("TMPDIR", originalTmpdir)
		os.Setenv("TMP", originalTmp)
		os.Setenv("TEMP", originalTemp)
	}()

	tests := []struct {
		name     string
		tmpdir   string
		tmp      string
		temp     string
		expected string
	}{
		{
			name:     "TMPDIR set",
			tmpdir:   "/custom/tmp",
			tmp:      "",
			temp:     "",
			expected: "/custom/tmp",
		},
		{
			name:     "TMP set (TMPDIR empty)",
			tmpdir:   "",
			tmp:      "/another/tmp",
			temp:     "",
			expected: "/another/tmp",
		},
		{
			name:     "TEMP set (TMPDIR and TMP empty)",
			tmpdir:   "",
			tmp:      "",
			temp:     "/third/tmp",
			expected: "/third/tmp",
		},
		{
			name:   "All empty - fallback",
			tmpdir: "",
			tmp:    "",
			temp:   "",
			expected: func() string {
				if IsWindows() {
					return "C:\\Windows\\Temp"
				}
				return "/tmp"
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment
			if tt.tmpdir == "" {
				os.Unsetenv("TMPDIR")
			} else {
				os.Setenv("TMPDIR", tt.tmpdir)
			}
			if tt.tmp == "" {
				os.Unsetenv("TMP")
			} else {
				os.Setenv("TMP", tt.tmp)
			}
			if tt.temp == "" {
				os.Unsetenv("TEMP")
			} else {
				os.Setenv("TEMP", tt.temp)
			}

			result := GetTempDirectory()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// Test SupportsShell with more edge cases
func TestSupportsShellEdgeCases(t *testing.T) {
	tests := []struct {
		shell    string
		expected bool
	}{
		// Test path variations
		{"/usr/bin/bash", IsUnix()},
		{"/bin/sh", IsUnix()},
		{"C:\\Windows\\System32\\cmd.exe", IsWindows()},
		{"C:/Windows/System32/cmd.exe", IsWindows()},
		
		// Test case variations
		{"BASH", IsUnix()},
		{"SH", IsUnix()},
		{"CMD", IsWindows()},
		{"POWERSHELL", IsWindows()},
		
		// Test empty and invalid inputs
		{"", false},
		{" ", false},
		{"invalid-shell-name", false},
		{"bash.exe", false}, // bash.exe doesn't exist typically
		
		// Test shells with spaces (edge case)
		{"Program Files\\PowerShell\\pwsh.exe", IsWindows()},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("shell_%s", tt.shell), func(t *testing.T) {
			result := SupportsShell(tt.shell)
			if result != tt.expected {
				t.Errorf("SupportsShell(%q): expected %v, got %v", tt.shell, tt.expected, result)
			}
		})
	}
}

// Test String methods for custom types
func TestLinuxDistributionString(t *testing.T) {
	tests := []LinuxDistribution{
		DistributionUbuntu,
		DistributionDebian,
		DistributionFedora,
		DistributionCentOS,
		DistributionRHEL,
		DistributionArch,
		DistributionOpenSUSE,
		DistributionAlpine,
		DistributionUnknown,
	}

	for _, dist := range tests {
		t.Run(string(dist), func(t *testing.T) {
			result := dist.String()
			expected := string(dist)
			if result != expected {
				t.Errorf("String(): expected %s, got %s", expected, result)
			}
		})
	}
}

// Test detectFromUname function (if accessible)
func TestDetectFromUnameIntegration(t *testing.T) {
	if !IsLinux() {
		t.Skip("Skipping Linux-specific test on non-Linux platform")
	}

	// Create a mock LinuxInfo to test with
	info := &LinuxInfo{Distribution: DistributionUnknown}
	
	// Test detectFromUname function
	err := detectFromUname(info)
	if err != nil {
		t.Logf("detectFromUname failed (may be expected): %v", err)
	} else {
		t.Logf("detectFromUname succeeded, detected: %s", info.Distribution)
		if info.Distribution == DistributionUnknown {
			t.Log("Warning: detectFromUname did not set a specific distribution")
		}
	}
}

// Test detectFromDistributionFiles function
func TestDetectFromDistributionFilesIntegration(t *testing.T) {
	if !IsLinux() {
		t.Skip("Skipping Linux-specific test on non-Linux platform")
	}

	info := &LinuxInfo{Distribution: DistributionUnknown}
	
	err := detectFromDistributionFiles(info)
	if err != nil {
		t.Logf("detectFromDistributionFiles failed (may be expected): %v", err)
	} else {
		t.Logf("detectFromDistributionFiles succeeded, detected: %s", info.Distribution)
		if info.Distribution == DistributionUnknown {
			t.Log("Warning: detectFromDistributionFiles did not set a specific distribution")
		}
	}
}

// Test command executor creation and basic functionality
func TestCommandExecutorCreation(t *testing.T) {
	executor := NewCommandExecutor()
	if executor == nil {
		t.Fatal("NewCommandExecutor returned nil")
	}

	// Test a simple command
	result, err := executor.Execute("echo", []string{"test"}, 5*time.Second)
	if err != nil {
		t.Errorf("Failed to execute simple echo command: %v", err)
	} else {
		if !strings.Contains(result.Stdout, "test") {
			t.Errorf("Expected 'test' in output, got: %s", result.Stdout)
		}
	}
}

// Test LinuxInfo struct initialization and fields
func TestLinuxInfoStruct(t *testing.T) {
	info := &LinuxInfo{
		Distribution: DistributionUbuntu,
		Version:      "20.04",
		Name:         "Ubuntu",
		ID:           "ubuntu",
		IDLike:       []string{"debian"},
	}

	if info.Distribution != DistributionUbuntu {
		t.Errorf("Expected Ubuntu distribution, got %s", info.Distribution)
	}
	if info.Version != "20.04" {
		t.Errorf("Expected version 20.04, got %s", info.Version)
	}
	if info.Name != "Ubuntu" {
		t.Errorf("Expected name Ubuntu, got %s", info.Name)
	}
	if info.ID != "ubuntu" {
		t.Errorf("Expected ID ubuntu, got %s", info.ID)
	}
	if len(info.IDLike) != 1 || info.IDLike[0] != "debian" {
		t.Errorf("Expected IDLike [debian], got %v", info.IDLike)
	}
}

// Test readOSRelease and readLSBRelease wrapper functions
func TestReadReleaseWrappers(t *testing.T) {
	if !IsLinux() {
		t.Skip("Skipping Linux-specific test on non-Linux platform")
	}

	// Test readOSRelease
	osRelease, err := readOSRelease()
	if err != nil {
		t.Logf("readOSRelease failed (may be expected): %v", err)
	} else {
		if len(osRelease) == 0 {
			t.Log("Warning: readOSRelease returned empty map")
		} else {
			t.Logf("readOSRelease succeeded, found %d entries", len(osRelease))
		}
	}

	// Test readLSBRelease
	lsbRelease, err := readLSBRelease()
	if err != nil {
		t.Logf("readLSBRelease failed (may be expected): %v", err)
	} else {
		if len(lsbRelease) == 0 {
			t.Log("Warning: readLSBRelease returned empty map")
		} else {
			t.Logf("readLSBRelease succeeded, found %d entries", len(lsbRelease))
		}
	}
}

// Test all Platform constants and their string representations
func TestPlatformConstants(t *testing.T) {
	platforms := []Platform{
		PlatformWindows,
		PlatformLinux,
		PlatformMacOS,
		PlatformUnknown,
	}

	expectedStrings := []string{
		"windows",
		"linux",
		"darwin",
		"unknown",
	}

	for i, platform := range platforms {
		if platform.String() != expectedStrings[i] {
			t.Errorf("Platform %v: expected string %s, got %s", platform, expectedStrings[i], platform.String())
		}
	}
}

// Test all Architecture constants and their string representations
func TestArchitectureConstants(t *testing.T) {
	architectures := []Architecture{
		ArchAMD64,
		ArchARM64,
		Arch386,
		ArchARM,
		ArchUnknown,
	}

	expectedStrings := []string{
		"amd64",
		"arm64",
		"386",
		"arm",
		"unknown",
	}

	for i, arch := range architectures {
		if arch.String() != expectedStrings[i] {
			t.Errorf("Architecture %v: expected string %s, got %s", arch, expectedStrings[i], arch.String())
		}
	}
}

// Test all Linux Distribution constants
func TestLinuxDistributionConstants(t *testing.T) {
	distributions := []LinuxDistribution{
		DistributionUbuntu,
		DistributionDebian,
		DistributionFedora,
		DistributionCentOS,
		DistributionRHEL,
		DistributionArch,
		DistributionOpenSUSE,
		DistributionAlpine,
		DistributionUnknown,
	}

	expectedStrings := []string{
		"ubuntu",
		"debian",
		"fedora",
		"centos",
		"rhel",
		"arch",
		"opensuse",
		"alpine",
		"unknown",
	}

	for i, dist := range distributions {
		if dist.String() != expectedStrings[i] {
			t.Errorf("Distribution %v: expected string %s, got %s", dist, expectedStrings[i], dist.String())
		}
	}
}

// Test cross-platform path handling edge cases
func TestCrossPlatformPathHandling(t *testing.T) {
	tests := []struct {
		name        string
		platform    Platform
		expectedSep string
		expectedExt string
		expectedList string
	}{
		{"Windows", PlatformWindows, "\\", ".exe", ";"},
		{"Linux", PlatformLinux, "/", "", ":"},
		{"macOS", PlatformMacOS, "/", "", ":"},
		{"Unknown", PlatformUnknown, "/", "", ":"}, // Default to Unix-like
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test current platform behavior
			currentPlatform := GetCurrentPlatform()
			if currentPlatform == tt.platform {
				sep := GetPathSeparator()
				if sep != tt.expectedSep {
					t.Errorf("GetPathSeparator(): expected %s, got %s", tt.expectedSep, sep)
				}

				ext := GetExecutableExtension()
				if ext != tt.expectedExt {
					t.Errorf("GetExecutableExtension(): expected %s, got %s", tt.expectedExt, ext)
				}

				listSep := GetPathListSeparator()
				if listSep != tt.expectedList {
					t.Errorf("GetPathListSeparator(): expected %s, got %s", tt.expectedList, listSep)
				}
			}
		})
	}
}

// Test file parsing with malformed content
func TestReadReleaseFileMalformed(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "platform_malformed_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name    string
		content string
	}{
		{
			name: "Only comments",
			content: `# Comment 1
# Comment 2
# Comment 3`,
		},
		{
			name: "Only empty lines",
			content: "\n\n\n\n",
		},
		{
			name: "Mixed invalid lines",
			content: `VALID=value
INVALID LINE
=STARTS_WITH_EQUALS
ENDS_WITH_EQUALS=
NO_VALUE
=
`,
		},
		{
			name: "Unicode content",
			content: `NAME="测试系统"
VERSION="1.0 α"
ID=test_unicode`,
		},
		{
			name: "Very long lines",
			content: fmt.Sprintf("NAME=%s\nID=test", strings.Repeat("very_long_name_", 100)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tempDir, "test_"+strings.ReplaceAll(tt.name, " ", "_"))
			err := os.WriteFile(testFile, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			result, err := readReleaseFile(testFile)
			if err != nil {
				t.Errorf("readReleaseFile failed: %v", err)
				return
			}

			// Should not crash and should return a map (even if empty)
			if result == nil {
				t.Error("readReleaseFile returned nil map")
			}
			t.Logf("Parsed %d entries from malformed content", len(result))
		})
	}
}

// Test edge cases in shell support function
func TestSupportsShellExtended(t *testing.T) {
	tests := []struct {
		shell       string
		description string
	}{
		{"/usr/local/bin/fish", "Fish with full path"},
		{"/opt/homebrew/bin/zsh", "Zsh with homebrew path"},
		{"C:\\Program Files\\PowerShell\\7\\pwsh.exe", "PowerShell 7 with spaces"},
		{"./bash", "Relative path bash"},
		{"../bin/sh", "Relative path with parent directory"},
		{"   bash   ", "Shell name with spaces (should work after trim)"},
		{"bash\n", "Shell name with newline"},
		{"cmd\r\n", "Shell name with Windows line ending"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			result := SupportsShell(tt.shell)
			t.Logf("SupportsShell(%q) = %v", tt.shell, result)
			// We don't assert specific results here since they depend on platform
			// and actual shell availability, but we verify the function doesn't crash
		})
	}
}

// Test error conditions in Linux distribution detection
func TestDetectLinuxDistributionNonLinux(t *testing.T) {
	if IsLinux() {
		t.Skip("Skipping non-Linux test on Linux platform")
	}

	// Should return error on non-Linux systems
	info, err := DetectLinuxDistribution()
	if err == nil {
		t.Error("Expected error when calling DetectLinuxDistribution on non-Linux platform")
	}
	if info != nil {
		t.Error("Expected nil info when calling DetectLinuxDistribution on non-Linux platform")
	}
}

// Test package manager preferences edge cases
func TestGetPreferredPackageManagersEdgeCases(t *testing.T) {
	// Test with empty/nil distribution
	emptyDist := LinuxDistribution("")
	result := GetPreferredPackageManagers(emptyDist)
	expected := []string{"apt", "dnf", "yum"} // Default fallback
	if len(result) != len(expected) {
		t.Errorf("Empty distribution: expected %v, got %v", expected, result)
	}

	// Test with custom distribution name
	customDist := LinuxDistribution("custom-linux-distro")
	result = GetPreferredPackageManagers(customDist)
	if len(result) != len(expected) {
		t.Errorf("Custom distribution: expected %v, got %v", expected, result)
	}
}