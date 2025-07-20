package platform

import (
	"runtime"
	"testing"
)

func TestCrossPlatformSupport(t *testing.T) {
	t.Run("AllPlatformsDetection", func(t *testing.T) {
		targetPlatforms := []string{"linux", "windows", "darwin"}

		currentPlatform := GetCurrentPlatform()
		t.Logf("Current platform: %s", currentPlatform)

		found := false
		for _, platform := range targetPlatforms {
			if string(currentPlatform) == platform {
				found = true
				break
			}
		}

		if !found && currentPlatform != PlatformUnknown {
			t.Errorf("Current platform %s not in target platforms", currentPlatform)
		}
	})

	t.Run("AllArchitecturesDetection", func(t *testing.T) {
		targetArchs := []string{"amd64", "arm64"}

		currentArch := GetCurrentArchitecture()
		t.Logf("Current architecture: %s", currentArch)

		found := false
		for _, arch := range targetArchs {
			if string(currentArch) == arch {
				found = true
				break
			}
		}

		if !found && currentArch != ArchUnknown {
			t.Errorf("Current architecture %s not in target architectures", currentArch)
		}
	})

	t.Run("PlatformSpecificPaths", func(t *testing.T) {
		home, err := GetHomeDirectory()
		if err != nil {
			t.Errorf("Failed to get home directory: %v", err)
		}
		t.Logf("Home directory: %s", home)

		temp := GetTempDirectory()
		t.Logf("Temp directory: %s", temp)

		ext := GetExecutableExtension()
		expectedExt := ""
		if IsWindows() {
			expectedExt = ".exe"
		}
		if ext != expectedExt {
			t.Errorf("Expected executable extension %s, got %s", expectedExt, ext)
		}
	})

	t.Run("PlatformSpecificBooleans", func(t *testing.T) {
		platforms := []bool{IsWindows(), IsLinux(), IsMacOS()}
		trueCount := 0
		for _, p := range platforms {
			if p {
				trueCount++
			}
		}

		if trueCount != 1 {
			t.Errorf("Expected exactly one platform to be true, got %d", trueCount)
		}

		expectedUnix := IsLinux() || IsMacOS()
		if IsUnix() != expectedUnix {
			t.Errorf("Unix detection mismatch: IsUnix()=%v, expected=%v", IsUnix(), expectedUnix)
		}
	})
}

func TestCrossPlatformBuildTargets(t *testing.T) {
	buildTargets := []struct {
		os   string
		arch string
	}{
		{"linux", "amd64"},
		{"windows", "amd64"},
		{"darwin", "amd64"},
		{"darwin", "arm64"},
	}

	for _, target := range buildTargets {
		t.Run(target.os+"-"+target.arch, func(t *testing.T) {
			var platform Platform
			switch target.os {
			case string(PlatformLinux):
				platform = PlatformLinux
			case "windows":
				platform = PlatformWindows
			case string(PlatformMacOS):
				platform = PlatformMacOS
			}

			if platform == "" {
				t.Errorf("Unknown platform: %s", target.os)
			}

			var arch Architecture
			switch target.arch {
			case string(ArchAMD64):
				arch = ArchAMD64
			case string(ArchARM64):
				arch = ArchARM64
			}

			if arch == "" {
				t.Errorf("Unknown architecture: %s", target.arch)
			}

			t.Logf("Target: %s-%s validated", platform, arch)
		})
	}
}

func TestPackageManagerDetection(t *testing.T) {
	t.Run("GetAvailablePackageManagers", func(t *testing.T) {
		managers := GetAvailablePackageManagers()
		t.Logf("Available package managers: %v", managers)

		if len(managers) == 0 {
			t.Log("No package managers detected - this might be expected in test environments")
		}
	})

	t.Run("GetBestPackageManager", func(t *testing.T) {
		manager := GetBestPackageManager()
		if manager != nil {
			t.Logf("Best package manager: %s", manager.GetName())
		} else {
			t.Log("No package manager available - this might be expected in test environments")
		}
	})
}

func TestCommandExecutorCrossPlatform(t *testing.T) {
	executor := NewCommandExecutor()

	t.Run("ShellDetection", func(t *testing.T) {
		shell := executor.GetShell()
		t.Logf("Detected shell: %s", shell)

		if shell == "" {
			t.Error("No shell detected")
		}

		if !SupportsShell(shell) {
			t.Errorf("Shell %s not supported on current platform", shell)
		}
	})

	t.Run("PlatformSpecificCommands", func(t *testing.T) {
		var testCmd string
		var testArgs []string

		if IsWindows() {
			testCmd = "cmd"
			testArgs = []string{"/C", "echo", "test"}
		} else {
			testCmd = "echo"
			testArgs = []string{"test"}
		}

		if executor.IsCommandAvailable(testCmd) {
			result, err := executor.Execute(testCmd, testArgs, 10*1000*1000*1000) // 10 seconds
			if err != nil {
				t.Logf("Command execution failed (expected in test environments): %v", err)
			} else {
				t.Logf("Command output: %s", result.Stdout)
				if result.ExitCode != 0 {
					t.Errorf("Command failed with exit code %d", result.ExitCode)
				}
			}
		} else {
			t.Logf("Command %s not available (expected in test environments)", testCmd)
		}
	})
}

func TestLinuxDistributionDetection(t *testing.T) {
	if !IsLinux() {
		t.Skip("Skipping Linux-specific test on non-Linux platform")
	}

	info, err := DetectLinuxDistribution()
	if err != nil {
		t.Errorf("Failed to detect Linux distribution: %v", err)
	} else {
		t.Logf("Detected Linux distribution: %s %s (ID: %s)",
			info.Name, info.Version, info.ID)

		if info.Distribution == DistributionUnknown {
			t.Error("Could not determine Linux distribution")
		}

		managers := GetPreferredPackageManagers(info.Distribution)
		t.Logf("Preferred package managers for %s: %v", info.Distribution, managers)

		if len(managers) == 0 {
			t.Error("No preferred package managers found")
		}
	}
}

func BenchmarkPlatformDetection(b *testing.B) {
	b.Run("GetCurrentPlatform", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = GetCurrentPlatform()
		}
	})

	b.Run("GetCurrentArchitecture", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = GetCurrentArchitecture()
		}
	})

	b.Run("GetPlatformString", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = GetPlatformString()
		}
	})
}

func TestCrossPlatformCompatibility(t *testing.T) {
	t.Run("RuntimeGOOS", func(t *testing.T) {
		goos := runtime.GOOS
		platform := GetCurrentPlatform()

		switch goos {
		case "windows":
			if platform != PlatformWindows {
				t.Errorf("GOOS=%s but platform=%s", goos, platform)
			}
		case "linux":
			if platform != PlatformLinux {
				t.Errorf("GOOS=%s but platform=%s", goos, platform)
			}
		case "darwin":
			if platform != PlatformMacOS {
				t.Errorf("GOOS=%s but platform=%s", goos, platform)
			}
		default:
			if platform != PlatformUnknown {
				t.Errorf("GOOS=%s should map to PlatformUnknown, got %s", goos, platform)
			}
		}
	})

	t.Run("RuntimeGOARCH", func(t *testing.T) {
		goarch := runtime.GOARCH
		arch := GetCurrentArchitecture()

		switch goarch {
		case "amd64":
			if arch != ArchAMD64 {
				t.Errorf("GOARCH=%s but arch=%s", goarch, arch)
			}
		case "arm64":
			if arch != ArchARM64 {
				t.Errorf("GOARCH=%s but arch=%s", goarch, arch)
			}
		case "386":
			if arch != Arch386 {
				t.Errorf("GOARCH=%s but arch=%s", goarch, arch)
			}
		case "arm":
			if arch != ArchARM {
				t.Errorf("GOARCH=%s but arch=%s", goarch, arch)
			}
		default:
			if arch != ArchUnknown {
				t.Errorf("GOARCH=%s should map to ArchUnknown, got %s", goarch, arch)
			}
		}
	})
}
