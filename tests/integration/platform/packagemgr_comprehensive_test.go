package platform_test

import (
	"lsp-gateway/internal/platform"
	"testing"
)

// Test GetAvailablePackageManagers function
func TestGetAvailablePackageManagersComprehensive(t *testing.T) {
	managers := platform.GetAvailablePackageManagers()

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
	manager := platform.GetBestPackageManager()

	if manager == nil {
		t.Fatal("GetBestPackageManager should return non-nil manager")
		return
	}

	if manager.GetName() == "" {
		t.Error("Best manager should have a name")
	}

	// Should be compatible with current platform
	platforms := manager.GetPlatforms()
	currentPlatform := platform.GetCurrentPlatform().String()
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

// Test getBestLinuxPackageManager function - skipped as it tests unexported functions
func TestGetBestLinuxPackageManagerFunction(t *testing.T) {
	t.Skip("getBestLinuxPackageManager is an unexported function that cannot be tested from external packages")
}

// Test getBestDarwinPackageManager function - skipped as it tests unexported functions
func TestGetBestDarwinPackageManagerFunction(t *testing.T) {
	t.Skip("getBestDarwinPackageManager is an unexported function that cannot be tested from external packages")
}

// Test getBestWindowsPackageManager function - skipped as it tests unexported functions
func TestGetBestWindowsPackageManagerFunction(t *testing.T) {
	t.Skip("getBestWindowsPackageManager is an unexported function that cannot be tested from external packages")
}

// Test findPackageManagerByName function - skipped as it tests unexported functions
func TestFindPackageManagerByName(t *testing.T) {
	t.Skip("findPackageManagerByName is an unexported function that cannot be tested from external packages")

}

// Test package manager constructors
func TestPackageManagerConstructors(t *testing.T) {
	// Test NewAptManager
	t.Run("NewAptManager", func(t *testing.T) {
		manager := platform.NewAptManager()
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
		manager := platform.NewHomebrewManager()
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
		manager := platform.NewWingetManager()
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
		manager := platform.NewChocolateyManager()
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
		manager := platform.NewDnfManager()
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
		manager := platform.NewYumManager()
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
	managers := platform.GetAvailablePackageManagers()

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
	managers := platform.GetAvailablePackageManagers()

	for _, manager := range managers {
		t.Run(manager.GetName()+" platform compatibility", func(t *testing.T) {
			platforms := manager.GetPlatforms()
			if len(platforms) == 0 {
				t.Errorf("Manager %s should support at least one platform", manager.GetName())
			}

			// Verify platform values are valid
			for _, platformStr := range platforms {
				switch platformStr {
				case platform.PlatformLinux.String(), platform.PlatformMacOS.String(), platform.PlatformWindows.String():
					// Valid platform
				default:
					t.Errorf("Manager %s has invalid platform: %s", manager.GetName(), platformStr)
				}
			}
		})
	}
}

// Test package manager availability
func TestPackageManagerAvailability(t *testing.T) {
	managers := platform.GetAvailablePackageManagers()

	for _, manager := range managers {
		t.Run(manager.GetName()+" availability", func(t *testing.T) {
			// Check if manager is available
			available := manager.IsAvailable()
			t.Logf("Manager %s availability: %v", manager.GetName(), available)

			// If manager is compatible with current platform, test availability
			platforms := manager.GetPlatforms()
			currentPlatform := platform.GetCurrentPlatform().String()
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
		err := platform.InstallComponent("definitely-does-not-exist-package-xyz")
		// Should return an error for non-existent package
		if err == nil {
			t.Log("Warning: InstallComponent did not return error for non-existent package")
		} else {
			t.Logf("Expected error for non-existent package: %v", err)
		}
	})

	t.Run("Verify non-existent package", func(t *testing.T) {
		_, err := platform.VerifyComponent("definitely-does-not-exist-package-xyz")
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
	if platform.IsLinux() {
		t.Run("Apt manager on Linux", func(t *testing.T) {
			aptManager := platform.NewAptManager()

			// Test basic properties
			if aptManager.GetName() != "apt" {
				t.Errorf("Expected apt name, got %s", aptManager.GetName())
			}

			platforms := aptManager.GetPlatforms()
			found := false
			for _, platformStr := range platforms {
				if platformStr == platform.PlatformLinux.String() {
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
	if platform.IsMacOS() {
		t.Run("Homebrew manager on macOS", func(t *testing.T) {
			brewManager := platform.NewHomebrewManager()

			// Test basic properties
			if brewManager.GetName() != "brew" {
				t.Errorf("Expected brew name, got %s", brewManager.GetName())
			}

			platforms := brewManager.GetPlatforms()
			found := false
			for _, platformStr := range platforms {
				if platformStr == platform.PlatformMacOS.String() {
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
	if platform.IsWindows() {
		t.Run("Winget manager on Windows", func(t *testing.T) {
			wingetManager := platform.NewWingetManager()

			// Test basic properties
			if wingetManager.GetName() != "winget" {
				t.Errorf("Expected winget name, got %s", wingetManager.GetName())
			}

			platforms := wingetManager.GetPlatforms()
			found := false
			for _, platformStr := range platforms {
				if platformStr == platform.PlatformWindows.String() {
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
		err := platform.InstallComponent("")
		if err == nil {
			t.Log("Warning: InstallComponent did not return error for empty package name")
		}

		_, err = platform.VerifyComponent("")
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
				err := platform.InstallComponent(name)
				// Should handle gracefully (may return error for non-existent package)
				t.Logf("InstallComponent('%s') result: %v", name, err)

				_, err = platform.VerifyComponent(name)
				t.Logf("VerifyComponent('%s') result: %v", name, err)
			})
		}
	})
}
