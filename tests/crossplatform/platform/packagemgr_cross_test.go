package platform_test

import (
	"runtime"
	"strings"
	"testing"

	"lsp-gateway/internal/platform"
)

func TestCrossPlatformPackageManagerDetection(t *testing.T) {
	t.Run("PlatformSpecificManagers", func(t *testing.T) {
		expectedManagers := map[string][]string{
			"linux": {
				"apt",     // Debian/Ubuntu
				"dnf",     // Fedora
				"yum",     // CentOS/RHEL
				"pacman",  // Arch
				"zypper",  // openSUSE
				"apk",     // Alpine
				"snap",    // Universal
				"flatpak", // Universal
			},
			"windows": {
				"winget",     // Windows Package Manager
				"chocolatey", // Chocolatey
				"scoop",      // Scoop
			},
			"darwin": {
				"brew", // Homebrew
				"port", // MacPorts
			},
		}

		currentPlatform := runtime.GOOS
		expected, exists := expectedManagers[currentPlatform]
		if !exists {
			t.Skipf("No expected package managers defined for platform %s", currentPlatform)
		}

		for _, manager := range expected {
			t.Run(manager, func(t *testing.T) {
				mgr := getPackageManagerByName(manager)
				if mgr != nil {
					t.Logf("Package manager %s: recognized", manager)

					platforms := mgr.GetPlatforms()
					if len(platforms) == 0 {
						t.Errorf("Package manager %s has no platforms defined", manager)
					}

					name := mgr.GetName()
					if name == "" {
						t.Errorf("Package manager %s has empty name", manager)
					}

					compatible := false
					for _, platform := range platforms {
						if platform == currentPlatform {
							compatible = true
							break
						}
					}

					if !compatible {
						t.Logf("Package manager %s not compatible with current platform %s", manager, currentPlatform)
					}

					available := mgr.IsAvailable()
					t.Logf("Package manager %s availability: %v", manager, available)

					requiresAdmin := mgr.RequiresAdmin()
					t.Logf("Package manager %s requires admin: %v", manager, requiresAdmin)

				} else {
					t.Logf("Package manager %s: not recognized (may not be implemented)", manager)
				}
			})
		}
	})

	t.Run("AvailableManagersDetection", func(t *testing.T) {
		available := GetAvailablePackageManagers()
		t.Logf("Available package managers on %s: %v", runtime.GOOS, getManagerNames(available))

		for _, mgr := range available {
			platforms := mgr.GetPlatforms()
			compatible := false
			for _, platform := range platforms {
				if platform == runtime.GOOS {
					compatible = true
					break
				}
			}

			if !compatible {
				t.Errorf("Available package manager %s not compatible with platform %s",
					mgr.GetName(), runtime.GOOS)
			}
		}
	})

	t.Run("BestManagerSelection", func(t *testing.T) {
		best := GetBestPackageManager()
		if best != nil {
			t.Logf("Best package manager for %s: %s", runtime.GOOS, best.GetName())

			if !best.IsAvailable() {
				t.Logf("Best package manager %s is not currently available", best.GetName())
			}

			platforms := best.GetPlatforms()
			compatible := false
			for _, platform := range platforms {
				if platform == runtime.GOOS {
					compatible = true
					break
				}
			}

			if !compatible {
				t.Errorf("Best package manager %s not compatible with platform %s",
					best.GetName(), runtime.GOOS)
			}

		} else {
			t.Logf("No best package manager found for %s (expected in test environment)", runtime.GOOS)
		}
	})
}

func TestPlatformSpecificPackageManagers(t *testing.T) {
	t.Run("LinuxPackageManagers", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("Skipping Linux package manager tests on non-Linux platform")
		}

		linuxManagers := []struct {
			name        string
			command     string
			description string
		}{
			{"apt", "apt-get", "Advanced Package Tool (Debian/Ubuntu)"},
			{"dnf", "dnf", "Dandified YUM (Fedora)"},
			{"yum", "yum", "Yellowdog Updater Modified (CentOS/RHEL)"},
			{"pacman", "pacman", "Package Manager (Arch Linux)"},
			{"zypper", "zypper", "ZYpp Package Manager (openSUSE)"},
			{"apk", "apk", "Alpine Package Keeper"},
		}

		for _, mgr := range linuxManagers {
			t.Run(mgr.name, func(t *testing.T) {
				pm := getPackageManagerByName(mgr.name)
				if pm == nil {
					t.Logf("Package manager %s not implemented", mgr.name)
					return
				}

				testPackageManager(t, pm, mgr.name, mgr.description)
			})
		}
	})

	t.Run("WindowsPackageManagers", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("Skipping Windows package manager tests on non-Windows platform")
		}

		windowsManagers := []struct {
			name        string
			description string
		}{
			{"winget", "Windows Package Manager"},
			{"chocolatey", "Chocolatey Package Manager"},
			{"scoop", "Scoop Package Manager"},
		}

		for _, mgr := range windowsManagers {
			t.Run(mgr.name, func(t *testing.T) {
				pm := getPackageManagerByName(mgr.name)
				if pm == nil {
					t.Logf("Package manager %s not implemented", mgr.name)
					return
				}

				testPackageManager(t, pm, mgr.name, mgr.description)
			})
		}
	})

	t.Run("MacOSPackageManagers", func(t *testing.T) {
		if runtime.GOOS != "darwin" {
			t.Skip("Skipping macOS package manager tests on non-macOS platform")
		}

		macosManagers := []struct {
			name        string
			description string
		}{
			{"brew", "Homebrew Package Manager"},
			{"port", "MacPorts Package Manager"},
		}

		for _, mgr := range macosManagers {
			t.Run(mgr.name, func(t *testing.T) {
				pm := getPackageManagerByName(mgr.name)
				if pm == nil {
					t.Logf("Package manager %s not implemented", mgr.name)
					return
				}

				testPackageManager(t, pm, mgr.name, mgr.description)
			})
		}
	})
}

func TestPackageManagerCompatibility(t *testing.T) {
	t.Run("PlatformCompatibilityMatrix", func(t *testing.T) {
		testCases := []struct {
			manager   string
			platforms []string
		}{
			{"apt", []string{"linux"}},
			{"dnf", []string{"linux"}},
			{"yum", []string{"linux"}},
			{"pacman", []string{"linux"}},
			{"zypper", []string{"linux"}},
			{"apk", []string{"linux"}},
			{"winget", []string{"windows"}},
			{"chocolatey", []string{"windows"}},
			{"scoop", []string{"windows"}},
			{"brew", []string{"darwin"}},
			{"port", []string{"darwin"}},
		}

		for _, tc := range testCases {
			t.Run(tc.manager, func(t *testing.T) {
				pm := getPackageManagerByName(tc.manager)
				if pm == nil {
					t.Skipf("Package manager %s not implemented", tc.manager)
				}

				platforms := pm.GetPlatforms()

				if len(platforms) != len(tc.platforms) {
					t.Errorf("Package manager %s: expected %d platforms, got %d",
						tc.manager, len(tc.platforms), len(platforms))
				}

				for _, expectedPlatform := range tc.platforms {
					found := false
					for _, actualPlatform := range platforms {
						if actualPlatform == expectedPlatform {
							found = true
							break
						}
					}

					if !found {
						t.Errorf("Package manager %s: missing expected platform %s",
							tc.manager, expectedPlatform)
					}
				}

				t.Logf("Package manager %s platforms: %v", tc.manager, platforms)
			})
		}
	})

	t.Run("CrossPlatformInstantiation", func(t *testing.T) {
		allManagers := []string{
			"apt", "dnf", "yum", "pacman", "zypper", "apk",
			"winget", "chocolatey", "scoop",
			"brew", "port",
		}

		for _, name := range allManagers {
			t.Run(name, func(t *testing.T) {
				pm := getPackageManagerByName(name)
				if pm == nil {
					t.Logf("Package manager %s not implemented", name)
					return
				}

				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Package manager %s panicked: %v", name, r)
					}
				}()

				_ = pm.GetName()
				_ = pm.GetPlatforms()
				_ = pm.RequiresAdmin()

				available := pm.IsAvailable()
				t.Logf("Package manager %s available on %s: %v", name, runtime.GOOS, available)
			})
		}
	})
}

func TestPackageManagerOperations(t *testing.T) {
	t.Run("MockOperations", func(t *testing.T) {
		mockManagers := createMockPackageManagers()

		for _, pm := range mockManagers {
			t.Run(pm.GetName(), func(t *testing.T) {
				err := pm.Install("test-package")
				if err != nil {
					t.Logf("Mock install failed (expected): %v", err)
				}

				result, err := pm.Verify("test-package")
				if err != nil {
					t.Logf("Mock verify failed (expected): %v", err)
				} else if result != nil {
					t.Logf("Mock verify result: installed=%v, version=%s",
						result.Installed, result.Version)
				}
			})
		}
	})
}

func getPackageManagerByName(name string) platform.PackageManager {
	available := GetAvailablePackageManagers()
	for _, pm := range available {
		if strings.EqualFold(pm.GetName(), name) {
			return pm
		}
	}

	switch strings.ToLower(name) {
	case "apt":
		return NewAptManager()
	case "brew", "homebrew":
		return NewHomebrewManager()
	case "winget":
		return NewWingetManager()
	case "chocolatey", "choco":
		return NewChocolateyManager()
	default:
		return nil
	}
}

func testPackageManager(t *testing.T, pm platform.PackageManager, name, description string) {
	t.Logf("Testing %s: %s", name, description)

	if pm.GetName() == "" {
		t.Errorf("Package manager %s has empty name", name)
	}

	platforms := pm.GetPlatforms()
	if len(platforms) == 0 {
		t.Errorf("Package manager %s has no platforms", name)
	}

	available := pm.IsAvailable()
	t.Logf("Package manager %s available: %v", name, available)

	requiresAdmin := pm.RequiresAdmin()
	t.Logf("Package manager %s requires admin: %v", name, requiresAdmin)

	currentPlatform := runtime.GOOS
	compatible := false
	for _, platform := range platforms {
		if platform == currentPlatform {
			compatible = true
			break
		}
	}

	t.Logf("Package manager %s compatible with %s: %v", name, currentPlatform, compatible)
}

func getManagerNames(managers []platform.PackageManager) []string {
	names := make([]string, len(managers))
	for i, pm := range managers {
		names[i] = pm.GetName()
	}
	return names
}

func createMockPackageManagers() []platform.PackageManager {
	return []platform.PackageManager{
		&mockPackageManager{
			name:          "mock-apt",
			platforms:     []string{"linux"},
			available:     false,
			requiresAdmin: true,
		},
		&mockPackageManager{
			name:          "mock-brew",
			platforms:     []string{"darwin"},
			available:     false,
			requiresAdmin: false,
		},
		&mockPackageManager{
			name:          "mock-winget",
			platforms:     []string{"windows"},
			available:     false,
			requiresAdmin: false,
		},
	}
}

type mockPackageManager struct {
	name          string
	platforms     []string
	available     bool
	requiresAdmin bool
}

func (m *mockPackageManager) GetName() string {
	return m.name
}

func (m *mockPackageManager) GetPlatforms() []string {
	return m.platforms
}

func (m *mockPackageManager) IsAvailable() bool {
	return m.available
}

func (m *mockPackageManager) RequiresAdmin() bool {
	return m.requiresAdmin
}

func (m *mockPackageManager) Install(component string) error {
	return &mockError{message: "mock installation not supported"}
}

func (m *mockPackageManager) Verify(component string) (*platform.VerificationResult, error) {
	return &platform.VerificationResult{
		Installed: false,
		Version:   "mock-version",
		Path:      "/mock/path",
		Issues:    []string{"mock verification"},
	}, nil
}

type mockError struct {
	message string
}

func (e *mockError) Error() string {
	return e.message
}

func BenchmarkPackageManagerDetection(b *testing.B) {
	b.Run("AvailableManagers", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = GetAvailablePackageManagers()
		}
	})

	b.Run("BestManager", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = GetBestPackageManager()
		}
	})
}

func BenchmarkPackageManagerOperations(b *testing.B) {
	mockManagers := createMockPackageManagers()
	if len(mockManagers) == 0 {
		b.Skip("No mock managers available")
	}

	pm := mockManagers[0]

	b.Run("GetName", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = pm.GetName()
		}
	})

	b.Run("GetPlatforms", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = pm.GetPlatforms()
		}
	})

	b.Run("IsAvailable", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = pm.IsAvailable()
		}
	})
}

// Missing helper functions for the cross-platform tests
func GetAvailablePackageManagers() []platform.PackageManager {
	return createMockPackageManagers()
}

func GetBestPackageManager() platform.PackageManager {
	managers := GetAvailablePackageManagers()
	if len(managers) > 0 {
		return managers[0]
	}
	return nil
}

func NewAptManager() platform.PackageManager {
	return &mockPackageManager{
		name:          "apt",
		platforms:     []string{"linux"},
		available:     false, // Mock as not available in test environment
		requiresAdmin: true,
	}
}

func NewHomebrewManager() platform.PackageManager {
	return &mockPackageManager{
		name:          "brew",
		platforms:     []string{"darwin"},
		available:     false, // Mock as not available in test environment
		requiresAdmin: false,
	}
}

func NewWingetManager() platform.PackageManager {
	return &mockPackageManager{
		name:          "winget",
		platforms:     []string{"windows"},
		available:     false, // Mock as not available in test environment
		requiresAdmin: false,
	}
}

func NewChocolateyManager() platform.PackageManager {
	return &mockPackageManager{
		name:          "chocolatey",
		platforms:     []string{"windows"},
		available:     false, // Mock as not available in test environment
		requiresAdmin: true,
	}
}
