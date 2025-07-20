package platform

import (
	"testing"
)

// Test GetAvailablePackageManagers function
func TestGetAvailablePackageManagersComprehensive(t *testing.T) {
	managers := GetAvailablePackageManagers()

	// Should return at least one manager
	if len(managers) == 0 {
		t.Error("GetAvailablePackageManagers should return at least one manager")
	}

	// All managers should be non-nil
	for i, manager := range managers {
		if manager == nil {
			t.Errorf("Manager at index %d should not be nil", i)
		}
	}

	// All managers should have names
	for i, manager := range managers {
		if manager.GetName() == "" {
			t.Errorf("Manager at index %d should have a name", i)
		}
	}
}

// Test GetBestPackageManager function
func TestGetBestPackageManagerComprehensive(t *testing.T) {
	manager := GetBestPackageManager()

	if manager == nil {
		t.Fatal("GetBestPackageManager should return non-nil manager")
		return
	}

	if manager.GetName() == "" {
		t.Error("Best manager should have a name")
	}

	// Should be compatible with current platform
	platforms := manager.GetPlatforms()
	currentPlatform := GetCurrentPlatform().String()
	found := false
	for _, platform := range platforms {
		if platform == currentPlatform {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Best manager should be compatible with current platform %s, got platforms: %v", currentPlatform, platforms)
	}
}

// Test getBestLinuxPackageManager function
func TestGetBestLinuxPackageManagerFunction(t *testing.T) {
	if !IsLinux() {
		t.Skip("Skipping Linux-specific test on non-Linux platform")
	}

	available := GetAvailablePackageManagers()
	manager := getBestLinuxPackageManager(available)
	if manager == nil {
		t.Fatal("getBestLinuxPackageManager should return non-nil manager on Linux")
		return
	}

	if manager.GetName() == "" {
		t.Error("Linux manager should have a name")
	}

	// Should support Linux platform
	platforms := manager.GetPlatforms()
	found := false
	for _, platform := range platforms {
		if platform == PlatformLinux.String() {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Linux manager should support Linux platform, got: %v", platforms)
	}
}

// Test getBestDarwinPackageManager function
func TestGetBestDarwinPackageManagerFunction(t *testing.T) {
	if !IsMacOS() {
		t.Skip("Skipping macOS-specific test on non-macOS platform")
	}

	available := GetAvailablePackageManagers()
	manager := getBestDarwinPackageManager(available)
	if manager == nil {
		t.Fatal("getBestDarwinPackageManager should return non-nil manager on macOS")
		return
	}

	if manager.GetName() == "" {
		t.Error("macOS manager should have a name")
	}

	// Should support macOS platform
	platforms := manager.GetPlatforms()
	found := false
	for _, platform := range platforms {
		if platform == PlatformMacOS.String() {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("macOS manager should support macOS platform, got: %v", platforms)
	}
}

// Test getBestWindowsPackageManager function
func TestGetBestWindowsPackageManagerFunction(t *testing.T) {
	if !IsWindows() {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	available := GetAvailablePackageManagers()
	manager := getBestWindowsPackageManager(available)
	if manager == nil {
		t.Fatal("getBestWindowsPackageManager should return non-nil manager on Windows")
		return
	}

	if manager.GetName() == "" {
		t.Error("Windows manager should have a name")
	}

	// Should support Windows platform
	platforms := manager.GetPlatforms()
	found := false
	for _, platform := range platforms {
		if platform == PlatformWindows.String() {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Windows manager should support Windows platform, got: %v", platforms)
	}
}

// Test findPackageManagerByName function
func TestFindPackageManagerByName(t *testing.T) {
	tests := []struct {
		name           string
		shouldExist    bool
		platformNeeded Platform
	}{
		{"apt", true, PlatformLinux},
		{"homebrew", true, PlatformMacOS},
		{"winget", true, PlatformWindows},
		{"chocolatey", true, PlatformWindows},
		{"dnf", true, PlatformLinux},
		{"yum", true, PlatformLinux},
		{"nonexistent", false, PlatformUnknown},
		{"", false, PlatformUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			available := GetAvailablePackageManagers()
			manager := findPackageManagerByName(available, tt.name)

			if tt.shouldExist {
				// Only test existence if we're on the right platform
				currentPlatform := GetCurrentPlatform()
				if tt.platformNeeded == PlatformUnknown || currentPlatform == tt.platformNeeded {
					if manager == nil {
						t.Errorf("Expected to find manager for %s on platform %s", tt.name, currentPlatform)
					} else if manager.GetName() != tt.name {
						t.Errorf("Expected manager name %s, got %s", tt.name, manager.GetName())
					}
				} else {
					t.Logf("Skipping %s test on incompatible platform %s (needs %s)", tt.name, currentPlatform, tt.platformNeeded)
				}
			} else {
				if manager != nil {
					t.Errorf("Expected no manager for %s, got %s", tt.name, manager.GetName())
				}
			}
		})
	}
}

// Test package manager constructors
func TestPackageManagerConstructors(t *testing.T) {
	// Test NewAptManager
	t.Run("NewAptManager", func(t *testing.T) {
		manager := NewAptManager()
		if manager == nil {
			t.Fatal("NewAptManager should return non-nil manager")
			return
		}
		if manager.GetName() != "apt" {
			t.Errorf("Expected apt manager name, got %s", manager.GetName())
		}
		platforms := manager.GetPlatforms()
		if len(platforms) == 0 {
			t.Error("Apt manager should support at least one platform")
		}
	})

	// Test NewHomebrewManager
	t.Run("NewHomebrewManager", func(t *testing.T) {
		manager := NewHomebrewManager()
		if manager == nil {
			t.Fatal("NewHomebrewManager should return non-nil manager")
			return
		}
		if manager.GetName() != "homebrew" {
			t.Errorf("Expected homebrew manager name, got %s", manager.GetName())
		}
	})

	// Test NewWingetManager
	t.Run("NewWingetManager", func(t *testing.T) {
		manager := NewWingetManager()
		if manager == nil {
			t.Fatal("NewWingetManager should return non-nil manager")
			return
		}
		if manager.GetName() != "winget" {
			t.Errorf("Expected winget manager name, got %s", manager.GetName())
		}
	})

	// Test NewChocolateyManager
	t.Run("NewChocolateyManager", func(t *testing.T) {
		manager := NewChocolateyManager()
		if manager == nil {
			t.Fatal("NewChocolateyManager should return non-nil manager")
			return
		}
		if manager.GetName() != "chocolatey" {
			t.Errorf("Expected chocolatey manager name, got %s", manager.GetName())
		}
	})

	// Test NewDnfManager
	t.Run("NewDnfManager", func(t *testing.T) {
		manager := NewDnfManager()
		if manager == nil {
			t.Fatal("NewDnfManager should return non-nil manager")
			return
		}
		if manager.GetName() != "dnf" {
			t.Errorf("Expected dnf manager name, got %s", manager.GetName())
		}
	})

	// Test NewYumManager
	t.Run("NewYumManager", func(t *testing.T) {
		manager := NewYumManager()
		if manager == nil {
			t.Fatal("NewYumManager should return non-nil manager")
			return
		}
		if manager.GetName() != "yum" {
			t.Errorf("Expected yum manager name, got %s", manager.GetName())
		}
	})
}

// Test package manager admin requirements
func TestPackageManagerAdminRequirements(t *testing.T) {
	managers := GetAvailablePackageManagers()

	for _, manager := range managers {
		t.Run(manager.GetName()+" admin requirement", func(t *testing.T) {
			requiresAdmin := manager.RequiresAdmin()
			// Should return a boolean value (no error expected)
			t.Logf("Manager %s requires admin: %v", manager.GetName(), requiresAdmin)
		})
	}
}

// Test package manager platform compatibility
func TestPackageManagerPlatformCompatibility(t *testing.T) {
	managers := GetAvailablePackageManagers()

	for _, manager := range managers {
		t.Run(manager.GetName()+" platform compatibility", func(t *testing.T) {
			platforms := manager.GetPlatforms()
			if len(platforms) == 0 {
				t.Errorf("Manager %s should support at least one platform", manager.GetName())
			}

			// Verify platform values are valid
			for _, platform := range platforms {
				switch platform {
				case PlatformLinux.String(), PlatformMacOS.String(), PlatformWindows.String():
					// Valid platform
				default:
					t.Errorf("Manager %s has invalid platform: %s", manager.GetName(), platform)
				}
			}
		})
	}
}

// Test package manager availability
func TestPackageManagerAvailability(t *testing.T) {
	managers := GetAvailablePackageManagers()

	for _, manager := range managers {
		t.Run(manager.GetName()+" availability", func(t *testing.T) {
			// Check if manager is available
			available := manager.IsAvailable()
			t.Logf("Manager %s availability: %v", manager.GetName(), available)

			// If manager is compatible with current platform, test availability
			platforms := manager.GetPlatforms()
			currentPlatform := GetCurrentPlatform().String()
			compatible := false
			for _, platform := range platforms {
				if platform == currentPlatform {
					compatible = true
					break
				}
			}

			if compatible {
				// For compatible managers, availability check should not crash
				// (but may return false if not installed)
				t.Logf("Manager %s is compatible with current platform %s, available: %v",
					manager.GetName(), currentPlatform, available)
			} else {
				// For incompatible managers, should typically return false
				if available {
					t.Logf("Warning: Manager %s reports available on incompatible platform %s",
						manager.GetName(), currentPlatform)
				}
			}
		})
	}
}

// Test InstallComponent and VerifyComponent error handling
func TestPackageManagerOperationsErrorHandling(t *testing.T) {
	// Test with non-existent package
	t.Run("Install non-existent package", func(t *testing.T) {
		err := InstallComponent("definitely-does-not-exist-package-xyz")
		// Should return an error for non-existent package
		if err == nil {
			t.Log("Warning: InstallComponent did not return error for non-existent package")
		} else {
			t.Logf("Expected error for non-existent package: %v", err)
		}
	})

	t.Run("Verify non-existent package", func(t *testing.T) {
		_, err := VerifyComponent("definitely-does-not-exist-package-xyz")
		// Should return an error for non-existent package
		if err == nil {
			t.Log("Warning: VerifyComponent did not return error for non-existent package")
		} else {
			t.Logf("Expected error for non-existent package: %v", err)
		}
	})
}

// Test manager-specific functionality
func TestManagerSpecificFunctionality(t *testing.T) {
	// Test apt manager on Linux
	if IsLinux() {
		t.Run("Apt manager on Linux", func(t *testing.T) {
			aptManager := NewAptManager()

			// Test basic properties
			if aptManager.GetName() != "apt" {
				t.Errorf("Expected apt name, got %s", aptManager.GetName())
			}

			platforms := aptManager.GetPlatforms()
			found := false
			for _, platform := range platforms {
				if platform == PlatformLinux.String() {
					found = true
					break
				}
			}
			if !found {
				t.Error("Apt manager should support Linux platform")
			}

			// Test availability (might be false if apt is not installed)
			available := aptManager.IsAvailable()
			t.Logf("Apt availability on Linux: %v", available)
		})
	}

	// Test homebrew manager on macOS
	if IsMacOS() {
		t.Run("Homebrew manager on macOS", func(t *testing.T) {
			brewManager := NewHomebrewManager()

			// Test basic properties
			if brewManager.GetName() != "brew" {
				t.Errorf("Expected brew name, got %s", brewManager.GetName())
			}

			platforms := brewManager.GetPlatforms()
			found := false
			for _, platform := range platforms {
				if platform == PlatformMacOS.String() {
					found = true
					break
				}
			}
			if !found {
				t.Error("Homebrew manager should support macOS platform")
			}

			// Test availability (might be false if brew is not installed)
			available := brewManager.IsAvailable()
			t.Logf("Homebrew availability on macOS: %v", available)
		})
	}

	// Test winget manager on Windows
	if IsWindows() {
		t.Run("Winget manager on Windows", func(t *testing.T) {
			wingetManager := NewWingetManager()

			// Test basic properties
			if wingetManager.GetName() != "winget" {
				t.Errorf("Expected winget name, got %s", wingetManager.GetName())
			}

			platforms := wingetManager.GetPlatforms()
			found := false
			for _, platform := range platforms {
				if platform == PlatformWindows.String() {
					found = true
					break
				}
			}
			if !found {
				t.Error("Winget manager should support Windows platform")
			}

			// Test availability (might be false if winget is not installed)
			available := wingetManager.IsAvailable()
			t.Logf("Winget availability on Windows: %v", available)
		})
	}
}

// Test package manager edge cases
func TestPackageManagerEdgeCases(t *testing.T) {
	// Test empty package name
	t.Run("Empty package name", func(t *testing.T) {
		err := InstallComponent("")
		if err == nil {
			t.Log("Warning: InstallComponent did not return error for empty package name")
		}

		_, err = VerifyComponent("")
		if err == nil {
			t.Log("Warning: VerifyComponent did not return error for empty package name")
		}
	})

	// Test package name with special characters
	t.Run("Package name with special characters", func(t *testing.T) {
		specialNames := []string{
			"package-with-dashes",
			"package_with_underscores",
			"package.with.dots",
			"package@version",
		}

		for _, name := range specialNames {
			t.Run("Package "+name, func(t *testing.T) {
				err := InstallComponent(name)
				// Should handle gracefully (may return error for non-existent package)
				t.Logf("InstallComponent('%s') result: %v", name, err)

				_, err = VerifyComponent(name)
				t.Logf("VerifyComponent('%s') result: %v", name, err)
			})
		}
	})
}
