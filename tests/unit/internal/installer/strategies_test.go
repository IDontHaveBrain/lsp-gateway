package installer_test

import (
	"lsp-gateway/internal/installer"
	"os"
	"runtime"
	"strings"
	"testing"

	"lsp-gateway/internal/platform"
)

func TestNewMacOSStrategy(t *testing.T) {
	strategy := NewMacOSStrategy()

	if strategy == nil {
		t.Fatal("NewMacOSStrategy returned nil")
	}

	managers := strategy.GetPackageManagers()
	if len(managers) != 1 || managers[0] != "brew" {
		t.Errorf("expected package managers [brew], got %v", managers)
	}

	info := strategy.GetPlatformInfo()
	if info.OS != "darwin" {
		t.Errorf("expected OS 'darwin', got '%s'", info.OS)
	}
	if info.Distribution != "macos" {
		t.Errorf("expected distribution 'macos', got '%s'", info.Distribution)
	}
	if info.Architecture == "" {
		t.Error("expected non-empty architecture")
	}
}

func TestMacOSStrategy_GetPlatformInfo(t *testing.T) {
	strategy := NewMacOSStrategy()
	info := strategy.GetPlatformInfo()

	if info == nil {
		t.Fatal("GetPlatformInfo returned nil")
	}

	expectedFields := map[string]string{
		"OS":           "darwin",
		"Distribution": "macos",
	}

	for field, expected := range expectedFields {
		var actual string
		switch field {
		case "OS":
			actual = info.OS
		case "Distribution":
			actual = info.Distribution
		}

		if actual != expected {
			t.Errorf("expected %s '%s', got '%s'", field, expected, actual)
		}
	}

	validArchs := []string{"amd64", "arm64", "386", "arm"}
	found := false
	for _, arch := range validArchs {
		if info.Architecture == arch {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("unexpected architecture '%s'", info.Architecture)
	}
}

func TestMacOSStrategy_IsPackageManagerAvailable(t *testing.T) {
	strategy := NewMacOSStrategy()

	testCases := []struct {
		manager  string
		expected bool
	}{
		{"brew", true},     // Should check actual availability
		{"homebrew", true}, // Should check actual availability
		{"apt", false},     // Not supported on macOS
		{"winget", false},  // Not supported on macOS
		{"invalid", false}, // Invalid manager
	}

	for _, tc := range testCases {
		result := strategy.IsPackageManagerAvailable(tc.manager)

		if (tc.manager == "apt" || tc.manager == "winget" || tc.manager == "invalid") && result {
			t.Errorf("IsPackageManagerAvailable(%s): expected false for unsupported manager, got %v", tc.manager, result)
		}
	}
}

func TestMacOSStrategy_VersionExtraction(t *testing.T) {
	strategy := NewMacOSStrategy()

	majorVersionTests := []struct {
		input    string
		expected string
	}{
		{"17", "17"},
		{"17.0.2", "17"},
		{"21.0.1", "21"},
		{"11", "11"},
		{"invalid", ""},
		{"", ""},
	}

	for _, test := range majorVersionTests {
		result := strategy.extractMajorVersion(test.input)
		if result != test.expected {
			t.Errorf("extractMajorVersion(%s): expected '%s', got '%s'", test.input, test.expected, result)
		}
	}

	majorMinorTests := []struct {
		input    string
		expected string
	}{
		{"3.11", "3.11"},
		{"3.11.5", "3.11"},
		{"3.12.0", "3.12"},
		{"2.7", "2.7"},
		{"invalid", ""},
		{"", ""},
		{"3", ""}, // No minor version
	}

	for _, test := range majorMinorTests {
		result := strategy.extractMajorMinorVersion(test.input)
		if result != test.expected {
			t.Errorf("extractMajorMinorVersion(%s): expected '%s', got '%s'", test.input, test.expected, result)
		}
	}

	goVersionTests := []struct {
		input    string
		expected string
	}{
		{"go1.21", "1.21"},
		{"1.21.0", "1.21"},
		{"go1.20.5", "1.20"},
		{"1.19", "1.19"},
		{"invalid", ""},
	}

	for _, test := range goVersionTests {
		result := strategy.normalizeGoVersion(test.input)
		if result != test.expected {
			t.Errorf("normalizeGoVersion(%s): expected '%s', got '%s'", test.input, test.expected, result)
		}
	}
}

func TestMacOSStrategy_PackageNameGeneration(t *testing.T) {
	strategy := NewMacOSStrategy()

	pythonTests := []struct {
		version  string
		expected string
	}{
		{"", "python@3.11"},
		{"latest", "python@3.11"},
		{"3.11", "python@3.11"},
		{"3.12.0", "python@3.12"},
		{"3.10.8", "python@3.10"},
	}

	for _, test := range pythonTests {
		result := strategy.getPythonPackageName(test.version)
		if !strings.HasPrefix(result, "python@") {
			t.Errorf("getPythonPackageName(%s): expected python@ formula, got '%s'", test.version, result)
		}
	}

	javaTests := []struct {
		version  string
		expected string
	}{
		{"", "openjdk@21"},
		{"latest", "openjdk@21"},
		{"17", "openjdk@17"},
		{"17.0.2", "openjdk@17"},
		{"21.0.1", "openjdk@21"},
	}

	for _, test := range javaTests {
		result := strategy.getJavaPackageName(test.version)
		if !strings.HasPrefix(result, "openjdk@") {
			t.Errorf("getJavaPackageName(%s): expected openjdk@ formula, got '%s'", test.version, result)
		}
	}
}

func TestMacOSStrategy_UpdateHomebrewPath(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-specific test")
	}

	strategy := NewMacOSStrategy()

	originalPath := os.Getenv("PATH")
	defer func() { _ = os.Setenv("PATH", originalPath) }()

	_ = os.Setenv("PATH", "/usr/bin:/bin")

	err := strategy.updateHomebrewPath()
	if err != nil {
		t.Errorf("updateHomebrewPath failed: %v", err)
	}

	newPath := os.Getenv("PATH")
	arch := platform.GetCurrentArchitecture()

	var expectedBrewPath string
	switch arch {
	case platform.ArchARM64:
		expectedBrewPath = "/opt/homebrew/bin"
	default:
		expectedBrewPath = "/usr/local/bin"
	}

	if !strings.Contains(newPath, expectedBrewPath) {
		t.Errorf("expected PATH to contain '%s', got '%s'", expectedBrewPath, newPath)
	}
}

func TestMacOSStrategy_InstallationMethods(t *testing.T) {
	strategy := NewMacOSStrategy()

	installMethods := []struct {
		name    string
		method  func(string) error
		version string
	}{
		{"InstallGo", strategy.InstallGo, "1.21"},
		{"InstallPython", strategy.InstallPython, "3.11"},
		{"InstallNodejs", strategy.InstallNodejs, "18"},
		{"InstallJava", strategy.InstallJava, "17"},
	}

	for _, test := range installMethods {
		err := test.method(test.version)

		if err != nil {
			var installerErr *installer.InstallerError
			if !isInstallerError(err, &installerErr) {
				t.Errorf("%s should return installer.InstallerError when Homebrew unavailable, got %T", test.name, err)
			}
		}
	}
}

func TestMacOSStrategy_EnvironmentSetup(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-specific test")
	}

	strategy := NewMacOSStrategy()

	err := strategy.setupPythonEnvironment()
	if err != nil {
		t.Errorf("setupPythonEnvironment should not fail, got: %v", err)
	}

	err = strategy.setupJavaEnvironment("17")
	if err != nil {
		t.Errorf("setupJavaEnvironment should not fail, got: %v", err)
	}
}

func TestMacOSStrategy_OnNonMacOS(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Non-macOS test")
	}

	strategy := NewMacOSStrategy()

	if strategy.IsPackageManagerAvailable("brew") {
		t.Error("Homebrew should not be available on non-macOS platforms")
	}

	info := strategy.GetPlatformInfo()
	if info.OS != "darwin" {
		t.Errorf("expected OS 'darwin' even on non-macOS, got '%s'", info.OS)
	}
}

func isInstallerError(err error, target **installer.InstallerError) bool {
	if installerErr, ok := err.(*installer.InstallerError); ok {
		*target = installerErr
		return true
	}
	return false
}

func BenchmarkMacOSStrategy_VersionExtraction(b *testing.B) {
	strategy := NewMacOSStrategy()
	versions := []string{"1.21.0", "3.11.5", "17.0.2", "go1.20.5"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		version := versions[i%len(versions)]
		strategy.extractMajorVersion(version)
		strategy.extractMajorMinorVersion(version)
		strategy.normalizeGoVersion(version)
	}
}

func BenchmarkMacOSStrategy_PackageNameGeneration(b *testing.B) {
	strategy := NewMacOSStrategy()
	versions := []string{"3.11", "17", "1.21", "18"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		version := versions[i%len(versions)]
		strategy.getPythonPackageName(version)
		strategy.getJavaPackageName(version)
	}
}
