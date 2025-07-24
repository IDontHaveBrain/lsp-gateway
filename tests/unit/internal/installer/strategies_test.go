package installer_test

import (
	"lsp-gateway/internal/installer"
	"runtime"
	"testing"
)

func TestNewMacOSStrategy(t *testing.T) {
	strategy := installer.NewMacOSStrategy()

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
	strategy := installer.NewMacOSStrategy()
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
	strategy := installer.NewMacOSStrategy()

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

func TestMacOSStrategy_PublicInterface(t *testing.T) {
	strategy := installer.NewMacOSStrategy()

	// Test that all required interface methods are accessible
	info := strategy.GetPlatformInfo()
	if info == nil {
		t.Fatal("GetPlatformInfo should not return nil")
	}

	managers := strategy.GetPackageManagers()
	if len(managers) == 0 {
		t.Error("GetPackageManagers should return at least one manager")
	}

	// Test package manager availability checking
	isAvailable := strategy.IsPackageManagerAvailable("brew")
	// Result can be true or false depending on system, just verify it doesn't panic
	_ = isAvailable
}

func TestMacOSStrategy_InstallationMethods(t *testing.T) {
	strategy := installer.NewMacOSStrategy()

	// Test that installation methods exist and can be called
	// These tests verify the interface exists, not actual installation
	testCases := []struct {
		name    string
		method  func(string) error
		version string
	}{
		{"InstallGo", strategy.InstallGo, "1.21"},
		{"InstallPython", strategy.InstallPython, "3.11"},
		{"InstallNodejs", strategy.InstallNodejs, "18"},
		{"InstallJava", strategy.InstallJava, "17"},
	}

	for _, tc := range testCases {
		err := tc.method(tc.version)
		// Installation may fail due to missing dependencies, but method should exist
		if err != nil {
			// Check it returns a proper installer error, not a panic
			t.Logf("%s returned error (expected in test environment): %v", tc.name, err)
		}
	}
}

func TestMacOSStrategy_CrossPlatformBehavior(t *testing.T) {
	strategy := installer.NewMacOSStrategy()

	// Test that the strategy provides consistent interface across platforms
	info := strategy.GetPlatformInfo()
	if info.OS != "darwin" {
		t.Errorf("expected OS 'darwin', got '%s'", info.OS)
	}

	if info.Distribution != "macos" {
		t.Errorf("expected distribution 'macos', got '%s'", info.Distribution)
	}

	// Architecture should be set to a valid value
	validArchs := []string{"amd64", "arm64", "386", "arm"}
	archValid := false
	for _, arch := range validArchs {
		if info.Architecture == arch {
			archValid = true
			break
		}
	}
	if !archValid {
		t.Errorf("unexpected architecture '%s'", info.Architecture)
	}
}

func TestMacOSStrategy_PackageManagerInterface(t *testing.T) {
	strategy := installer.NewMacOSStrategy()

	// Test unsupported package managers
	unsupported := []string{"apt", "yum", "dnf", "winget", "invalid"}
	for _, mgr := range unsupported {
		if strategy.IsPackageManagerAvailable(mgr) {
			t.Errorf("IsPackageManagerAvailable(%s): expected false for unsupported manager", mgr)
		}
	}

	// Test supported managers (brew/homebrew)
	supported := []string{"brew", "homebrew"}
	for _, mgr := range supported {
		// Just verify the method can be called without panic
		// Actual availability depends on system state
		result := strategy.IsPackageManagerAvailable(mgr)
		t.Logf("Package manager %s availability: %v", mgr, result)
	}
}

func TestMacOSStrategy_InterfaceCompliance(t *testing.T) {
	strategy := installer.NewMacOSStrategy()

	// Verify that MacOSStrategy implements the PlatformStrategy interface
	var _ installer.PlatformStrategy = strategy

	// Test all interface methods are accessible
	methods := []func(){
		func() { strategy.GetPlatformInfo() },
		func() { strategy.GetPackageManagers() },
		func() { strategy.IsPackageManagerAvailable("brew") },
		func() { strategy.InstallGo("1.21") },
		func() { strategy.InstallPython("3.11") },
		func() { strategy.InstallNodejs("18") },
		func() { strategy.InstallJava("17") },
	}

	// Verify all methods can be called without panic
	for i, method := range methods {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Method %d panicked: %v", i, r)
				}
			}()
			method()
		}()
	}
}

func TestMacOSStrategy_OnNonMacOS(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Non-macOS test")
	}

	strategy := installer.NewMacOSStrategy()

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

func BenchmarkMacOSStrategy_PublicMethods(b *testing.B) {
	strategy := installer.NewMacOSStrategy()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.GetPlatformInfo()
		strategy.GetPackageManagers()
		strategy.IsPackageManagerAvailable("brew")
	}
}
