package installer

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

const (
	testArchX64   = "x64"
	testArchAMD64 = "amd64"
	testArchARM64 = "arm64"
	testOsLinux   = "linux"
	testOsDarwin  = "darwin"
	testOsWindows = "windows"
	testJdkTag    = "jdk-21.0.4+7"
)

// mockPlatformInfo for testing different platform scenarios
type mockPlatformInfo struct {
	platform string
	arch     string
}

func (m *mockPlatformInfo) GetPlatform() string {
	return m.platform
}

func (m *mockPlatformInfo) GetArch() string {
	return m.arch
}

func (m *mockPlatformInfo) GetPlatformString() string {
	arch := m.arch
	switch arch {
	case testArchAMD64:
		arch = testArchX64
	case testArchARM64:
		arch = testArchARM64
	default:
		arch = testArchX64
	}

	platform := m.platform
	switch platform {
	case testOsDarwin:
		return fmt.Sprintf("darwin-%s", arch)
	case testOsLinux:
		return fmt.Sprintf("linux-%s", arch)
	case testOsWindows:
		return fmt.Sprintf("win32-%s", arch)
	default:
		return fmt.Sprintf("%s-%s", platform, arch)
	}
}

func (m *mockPlatformInfo) IsSupported() bool {
	supportedPlatforms := map[string][]string{
		testOsLinux:   {testArchAMD64, testArchARM64},
		testOsDarwin:  {testArchAMD64, testArchARM64},
		testOsWindows: {testArchAMD64},
	}

	if supportedArchs, exists := supportedPlatforms[m.platform]; exists {
		for _, supportedArch := range supportedArchs {
			if m.arch == supportedArch {
				return true
			}
		}
	}
	return false
}

func (m *mockPlatformInfo) GetJavaDownloadURL(version string) (string, string, error) {
	if !m.IsSupported() {
		return "", "", fmt.Errorf("platform %s-%s not supported for Java installation", m.platform, m.arch)
	}

	baseURL := "https://github.com/adoptium/temurin21-binaries/releases/download/jdk-21.0.4%2B7"

	var filename string
	var extractDir string

	switch m.platform {
	case testOsLinux:
		switch m.arch {
		case testArchAMD64:
			filename = "OpenJDK21U-jdk_x64_linux_hotspot_21.0.4_7.tar.gz"
			extractDir = testJdkTag
		case testArchARM64:
			filename = "OpenJDK21U-jdk_aarch64_linux_hotspot_21.0.4_7.tar.gz"
			extractDir = testJdkTag
		default:
			return "", "", fmt.Errorf("unsupported architecture for Linux: %s", m.arch)
		}
	case testOsDarwin:
		switch m.arch {
		case testArchAMD64:
			filename = "OpenJDK21U-jdk_x64_mac_hotspot_21.0.4_7.tar.gz"
			extractDir = testJdkTag + "/Contents/Home"
		case testArchARM64:
			filename = "OpenJDK21U-jdk_aarch64_mac_hotspot_21.0.4_7.tar.gz"
			extractDir = testJdkTag + "/Contents/Home"
		default:
			return "", "", fmt.Errorf("unsupported architecture for macOS: %s", m.arch)
		}
	case testOsWindows:
		if m.arch == testArchAMD64 {
			filename = "OpenJDK21U-jdk_x64_windows_hotspot_21.0.4_7.zip"
			extractDir = testJdkTag
		} else {
			return "", "", fmt.Errorf("unsupported architecture for Windows: %s", m.arch)
		}
	default:
		return "", "", fmt.Errorf("unsupported platform: %s", m.platform)
	}

	downloadURL := fmt.Sprintf("%s/%s", baseURL, filename)
	return downloadURL, extractDir, nil
}

func (m *mockPlatformInfo) GetNodeInstallCommand() []string {
	switch m.platform {
	case testOsLinux:
		return []string{"apt-get", "update", "&&", "apt-get", "install", "-y", "nodejs", "npm"}
	case testOsDarwin:
		return []string{"brew", "install", "node"}
	case testOsWindows:
		return []string{"echo", "Please install Node.js from https://nodejs.org/"}
	default:
		return []string{"echo", "Platform not supported for automatic Node.js installation"}
	}
}

func TestLSPPlatformInfo_GetPlatform(t *testing.T) {
	platform := NewLSPPlatformInfo()

	result := platform.GetPlatform()

	// Should return current runtime platform
	expected := runtime.GOOS
	if result != expected {
		t.Errorf("Expected platform %s, got %s", expected, result)
	}

	// Should be one of supported platforms
	supportedPlatforms := []string{testOsLinux, testOsDarwin, testOsWindows}
	found := false
	for _, supported := range supportedPlatforms {
		if result == supported {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Platform %s not in supported platforms %v", result, supportedPlatforms)
	}
}

func TestLSPPlatformInfo_GetArch(t *testing.T) {
	platform := NewLSPPlatformInfo()

	result := platform.GetArch()

	// Should return current runtime architecture
	expected := runtime.GOARCH
	if result != expected {
		t.Errorf("Expected architecture %s, got %s", expected, result)
	}

	// Should be one of common architectures
	commonArchs := []string{testArchAMD64, testArchARM64, "386", "arm"}
	found := false
	for _, arch := range commonArchs {
		if result == arch {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Architecture %s not in common architectures %v", result, commonArchs)
	}
}

func TestLSPPlatformInfo_GetPlatformString(t *testing.T) {
	tests := []struct {
		name           string
		platform       string
		arch           string
		expectedSuffix string
	}{
		{
			name:           "Linux amd64",
			platform:       testOsLinux,
			arch:           testArchAMD64,
			expectedSuffix: "linux-x64",
		},
		{
			name:           "Linux arm64",
			platform:       testOsLinux,
			arch:           testArchARM64,
			expectedSuffix: "linux-arm64",
		},
		{
			name:           "macOS amd64",
			platform:       testOsDarwin,
			arch:           testArchAMD64,
			expectedSuffix: "darwin-x64",
		},
		{
			name:           "macOS arm64",
			platform:       testOsDarwin,
			arch:           testArchARM64,
			expectedSuffix: "darwin-arm64",
		},
		{
			name:           "Windows amd64",
			platform:       testOsWindows,
			arch:           testArchAMD64,
			expectedSuffix: "win32-x64",
		},
		{
			name:           "Unknown platform",
			platform:       "freebsd",
			arch:           testArchAMD64,
			expectedSuffix: "freebsd-x64",
		},
		{
			name:           "Unknown architecture defaults to x64",
			platform:       testOsLinux,
			arch:           "riscv64",
			expectedSuffix: "linux-x64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockPlatformInfo{
				platform: tt.platform,
				arch:     tt.arch,
			}

			result := mock.GetPlatformString()

			if result != tt.expectedSuffix {
				t.Errorf("Expected platform string %s, got %s", tt.expectedSuffix, result)
			}
		})
	}
}

func TestLSPPlatformInfo_IsSupported(t *testing.T) {
	tests := []struct {
		name      string
		platform  string
		arch      string
		supported bool
	}{
		{
			name:      "Linux amd64 - supported",
			platform:  testOsLinux,
			arch:      testArchAMD64,
			supported: true,
		},
		{
			name:      "Linux arm64 - supported",
			platform:  testOsLinux,
			arch:      testArchARM64,
			supported: true,
		},
		{
			name:      "macOS amd64 - supported",
			platform:  testOsDarwin,
			arch:      testArchAMD64,
			supported: true,
		},
		{
			name:      "macOS arm64 - supported",
			platform:  testOsDarwin,
			arch:      testArchARM64,
			supported: true,
		},
		{
			name:      "Windows amd64 - supported",
			platform:  testOsWindows,
			arch:      testArchAMD64,
			supported: true,
		},
		{
			name:      "Windows arm64 - not supported",
			platform:  testOsWindows,
			arch:      testArchARM64,
			supported: false,
		},
		{
			name:      "Linux 386 - not supported",
			platform:  testOsLinux,
			arch:      "386",
			supported: false,
		},
		{
			name:      "FreeBSD - not supported",
			platform:  "freebsd",
			arch:      testArchAMD64,
			supported: false,
		},
		{
			name:      "Unknown platform - not supported",
			platform:  "plan9",
			arch:      testArchAMD64,
			supported: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockPlatformInfo{
				platform: tt.platform,
				arch:     tt.arch,
			}

			result := mock.IsSupported()

			if result != tt.supported {
				t.Errorf("Expected support status %v, got %v", tt.supported, result)
			}
		})
	}
}

func TestLSPPlatformInfo_GetJavaDownloadURL(t *testing.T) {
	tests := []struct {
		name        string
		platform    string
		arch        string
		expectError bool
		urlContains string
		extractDir  string
	}{
		{
			name:        "Linux amd64",
			platform:    testOsLinux,
			arch:        testArchAMD64,
			expectError: false,
			urlContains: "OpenJDK21U-jdk_x64_linux_hotspot_21.0.4_7.tar.gz",
			extractDir:  testJdkTag,
		},
		{
			name:        "Linux arm64",
			platform:    testOsLinux,
			arch:        testArchARM64,
			expectError: false,
			urlContains: "OpenJDK21U-jdk_aarch64_linux_hotspot_21.0.4_7.tar.gz",
			extractDir:  testJdkTag,
		},
		{
			name:        "macOS amd64",
			platform:    testOsDarwin,
			arch:        testArchAMD64,
			expectError: false,
			urlContains: "OpenJDK21U-jdk_x64_mac_hotspot_21.0.4_7.tar.gz",
			extractDir:  testJdkTag + "/Contents/Home",
		},
		{
			name:        "macOS arm64",
			platform:    testOsDarwin,
			arch:        testArchARM64,
			expectError: false,
			urlContains: "OpenJDK21U-jdk_aarch64_mac_hotspot_21.0.4_7.tar.gz",
			extractDir:  testJdkTag + "/Contents/Home",
		},
		{
			name:        "Windows amd64",
			platform:    testOsWindows,
			arch:        testArchAMD64,
			expectError: false,
			urlContains: "OpenJDK21U-jdk_x64_windows_hotspot_21.0.4_7.zip",
			extractDir:  testJdkTag,
		},
		{
			name:        "Windows arm64 - unsupported architecture",
			platform:    testOsWindows,
			arch:        testArchARM64,
			expectError: true,
		},
		{
			name:        "Linux 386 - unsupported architecture",
			platform:    testOsLinux,
			arch:        "386",
			expectError: true,
		},
		{
			name:        "FreeBSD - unsupported platform",
			platform:    "freebsd",
			arch:        testArchAMD64,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockPlatformInfo{
				platform: tt.platform,
				arch:     tt.arch,
			}

			url, extractDir, err := mock.GetJavaDownloadURL("21")

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for %s-%s, but got none", tt.platform, tt.arch)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if !strings.Contains(url, tt.urlContains) {
				t.Errorf("Expected URL to contain %s, got %s", tt.urlContains, url)
			}

			if extractDir != tt.extractDir {
				t.Errorf("Expected extract dir %s, got %s", tt.extractDir, extractDir)
			}

			// Verify URL structure
			expectedPrefix := "https://github.com/adoptium/temurin21-binaries/releases/download/jdk-21.0.4%2B7"
			if !strings.HasPrefix(url, expectedPrefix) {
				t.Errorf("URL should start with %s, got %s", expectedPrefix, url)
			}
		})
	}
}

func TestLSPPlatformInfo_GetNodeInstallCommand(t *testing.T) {
	tests := []struct {
		name            string
		platform        string
		expectedCommand []string
	}{
		{
			name:            "Linux",
			platform:        "linux",
			expectedCommand: []string{"apt-get", "update", "&&", "apt-get", "install", "-y", "nodejs", "npm"},
		},
		{
			name:            "macOS",
			platform:        "darwin",
			expectedCommand: []string{"brew", "install", "node"},
		},
		{
			name:            "Windows",
			platform:        "windows",
			expectedCommand: []string{"echo", "Please install Node.js from https://nodejs.org/"},
		},
		{
			name:            "FreeBSD - unsupported",
			platform:        "freebsd",
			expectedCommand: []string{"echo", "Platform not supported for automatic Node.js installation"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockPlatformInfo{
				platform: tt.platform,
				arch:     "amd64", // arch doesn't matter for this test
			}

			result := mock.GetNodeInstallCommand()

			if len(result) != len(tt.expectedCommand) {
				t.Errorf("Expected command length %d, got %d", len(tt.expectedCommand), len(result))
				return
			}

			for i, expected := range tt.expectedCommand {
				if result[i] != expected {
					t.Errorf("Expected command[%d] = %s, got %s", i, expected, result[i])
				}
			}
		})
	}
}

func TestLSPPlatformInfo_CurrentPlatformIntegration(t *testing.T) {
	// Integration test using real platform info
	platform := NewLSPPlatformInfo()

	// Test that current platform methods work together
	currentPlatform := platform.GetPlatform()
	currentArch := platform.GetArch()

	t.Logf("Current platform: %s-%s", currentPlatform, currentArch)

	// Test platform string generation
	platformString := platform.GetPlatformString()
	if platformString == "" {
		t.Error("Platform string should not be empty")
	}
	t.Logf("Platform string: %s", platformString)

	// Test support detection
	isSupported := platform.IsSupported()
	t.Logf("Platform supported: %v", isSupported)

	// If platform is supported, Java URL should work
	if isSupported {
		url, extractDir, err := platform.GetJavaDownloadURL("21")
		if err != nil {
			t.Errorf("Java download URL failed for supported platform: %v", err)
		} else {
			t.Logf("Java URL: %s", url)
			t.Logf("Extract dir: %s", extractDir)
		}
	}

	// Node install command should always return something
	nodeCmd := platform.GetNodeInstallCommand()
	if len(nodeCmd) == 0 {
		t.Error("Node install command should not be empty")
	}
	t.Logf("Node install command: %v", nodeCmd)
}

func TestPlatformInfo_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		arch     string
		testCase string
	}{
		{
			name:     "Empty platform",
			platform: "",
			arch:     "amd64",
			testCase: "empty_platform",
		},
		{
			name:     "Empty architecture",
			platform: "linux",
			arch:     "",
			testCase: "empty_arch",
		},
		{
			name:     "Both empty",
			platform: "",
			arch:     "",
			testCase: "both_empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockPlatformInfo{
				platform: tt.platform,
				arch:     tt.arch,
			}

			// These should not panic and should return reasonable defaults
			platformString := mock.GetPlatformString()
			isSupported := mock.IsSupported()
			nodeCmd := mock.GetNodeInstallCommand()

			t.Logf("Platform string: %s", platformString)
			t.Logf("Is supported: %v", isSupported)
			t.Logf("Node command: %v", nodeCmd)

			// Empty values should result in unsupported platform
			if tt.testCase == "empty_platform" || tt.testCase == "both_empty" {
				if isSupported {
					t.Error("Empty platform should not be supported")
				}
			}
		})
	}
}

func TestPlatformInfo_JavaDownloadErrorMessages(t *testing.T) {
	tests := []struct {
		name           string
		platform       string
		arch           string
		expectedErrMsg string
	}{
		{
			name:           "Unsupported platform",
			platform:       "plan9",
			arch:           "amd64",
			expectedErrMsg: "platform plan9-amd64 not supported for Java installation",
		},
		{
			name:           "Linux unsupported arch",
			platform:       "linux",
			arch:           "mips",
			expectedErrMsg: "platform linux-mips not supported for Java installation",
		},
		{
			name:           "Windows unsupported arch",
			platform:       "windows",
			arch:           "arm64",
			expectedErrMsg: "platform windows-arm64 not supported for Java installation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockPlatformInfo{
				platform: tt.platform,
				arch:     tt.arch,
			}

			_, _, err := mock.GetJavaDownloadURL("21")

			if err == nil {
				t.Errorf("Expected error for %s-%s", tt.platform, tt.arch)
				return
			}

			if !strings.Contains(err.Error(), tt.expectedErrMsg) {
				t.Errorf("Expected error message to contain '%s', got '%s'", tt.expectedErrMsg, err.Error())
			}
		})
	}
}

func TestPlatformInfo_ArchitectureNormalization(t *testing.T) {
	tests := []struct {
		inputArch    string
		expectedArch string
	}{
		{testArchAMD64, testArchX64},
		{testArchARM64, testArchARM64},
		{"386", testArchX64},     // Falls back to x64
		{"arm", testArchX64},     // Falls back to x64
		{"mips", testArchX64},    // Falls back to x64
		{"riscv64", testArchX64}, // Falls back to x64
		{"", testArchX64},        // Falls back to x64
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("arch_%s", tt.inputArch), func(t *testing.T) {
			mock := &mockPlatformInfo{
				platform: testOsLinux,
				arch:     tt.inputArch,
			}

			platformString := mock.GetPlatformString()
			expectedString := fmt.Sprintf("linux-%s", tt.expectedArch)

			if platformString != expectedString {
				t.Errorf("Expected platform string %s, got %s", expectedString, platformString)
			}
		})
	}
}
