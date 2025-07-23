package platform_test

import (
	"context"
	"fmt"
	"lsp-gateway/internal/platform"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCoverageAssessment - Quick test to assess current coverage gaps
func TestCoverageAssessment(t *testing.T) {
	t.Run("BasicPlatformFunctions", func(t *testing.T) {
		// Test platform detection functions (likely well covered)
		currentPlatform := platform.GetCurrentPlatform()
		if currentPlatform == "" {
			t.Error("GetCurrentPlatform returned empty")
		}

		arch := platform.GetCurrentArchitecture()
		if arch == "" {
			t.Error("GetCurrentArchitecture returned empty")
		}

		// Test platform string functions
		platformStr := platform.GetPlatformString()
		if platformStr == "" {
			t.Error("GetPlatformString returned empty")
		}
	})

	t.Run("DirectoryFunctions", func(t *testing.T) {
		// Test home directory with error scenarios
		home, err := platform.GetHomeDirectory()
		if err != nil {
			t.Errorf("GetHomeDirectory failed: %v", err)
		}
		if home == "" {
			t.Error("Home directory is empty")
		}

		// Test temp directory
		temp := platform.GetTempDirectory()
		if temp == "" {
			t.Error("Temp directory is empty")
		}
	})

	t.Run("UtilityFunctions", func(t *testing.T) {
		// Test path functions
		sep := platform.GetPathSeparator()
		if sep == "" {
			t.Error("Path separator is empty")
		}

		listSep := platform.GetPathListSeparator()
		if listSep == "" {
			t.Error("Path list separator is empty")
		}

		ext := platform.GetExecutableExtension()
		// Extension can be empty on Unix, so just check it's consistent
		if platform.IsWindows() && ext != ".exe" {
			t.Errorf("Expected .exe on Windows, got %s", ext)
		}
	})
}

// TestExecutorFunctionCoverage - Test functions that might lack coverage
func TestExecutorFunctionCoverage(t *testing.T) {
	t.Run("GetExitCodeFromError", func(t *testing.T) {
		// Test with nil error
		code := platform.GetExitCodeFromError(nil)
		if code != 0 {
			t.Errorf("Expected 0 for nil error, got %d", code)
		}

		// Test with generic error
		code = platform.GetExitCodeFromError(fmt.Errorf("generic error"))
		if code != -1 {
			t.Errorf("Expected -1 for generic error, got %d", code)
		}
	})

	t.Run("GetCommandPath", func(t *testing.T) {
		// Test finding a command that should exist
		if platform.IsWindows() {
			path, err := platform.GetCommandPath("cmd")
			if err != nil {
				t.Errorf("Expected to find cmd on Windows: %v", err)
			}
			if path == "" {
				t.Error("Command path is empty")
			}
		} else {
			path, err := platform.GetCommandPath("sh")
			if err != nil {
				t.Errorf("Expected to find sh on Unix: %v", err)
			}
			if path == "" {
				t.Error("Command path is empty")
			}
		}

		// Test non-existent command
		_, err := platform.GetCommandPath("nonexistent_command_xyz123")
		if err == nil {
			t.Error("Expected error for non-existent command")
		}
	})

	t.Run("ExecuteShellCommandWithContext", func(t *testing.T) {
		executor := platform.NewCommandExecutor()
		ctx := context.Background()

		result, err := platform.ExecuteShellCommandWithContext(executor, ctx, "echo context_test")
		if err != nil {
			t.Errorf("ExecuteShellCommandWithContext failed: %v", err)
		}
		if result == nil {
			t.Error("Result is nil")
			return
		}
		if !strings.Contains(result.Stdout, "context_test") {
			t.Error("Output doesn't contain expected text")
		}

		// Test with timeout context
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		// Use a command that should timeout
		var longCmd string
		if platform.IsWindows() {
			longCmd = "ping 127.0.0.1 -n 10"
		} else {
			longCmd = "sleep 2"
		}

		_, err = platform.ExecuteShellCommandWithContext(executor, ctx, longCmd)
		if err == nil {
			t.Error("Expected timeout error")
		}
	})
}

// TestLinuxDistributionCoverage - Test Linux distribution detection (might have gaps)
func TestLinuxDistributionCoverage(t *testing.T) {
	if !platform.IsLinux() {
		t.Skip("Skipping Linux-specific tests on non-Linux platform")
	}

	t.Run("DetectLinuxDistribution", func(t *testing.T) {
		info, err := platform.DetectLinuxDistribution()
		if err != nil {
			t.Errorf("DetectLinuxDistribution failed: %v", err)
		}
		if info == nil {
			t.Error("Linux info is nil")
			return
		}
		if info.Distribution == "" {
			t.Error("Distribution is empty")
		}
	})

	t.Run("GetPreferredPackageManagers", func(t *testing.T) {
		// Test all known distributions
		testDistributions := []platform.LinuxDistribution{
			platform.DistributionUbuntu,
			platform.DistributionDebian,
			platform.DistributionFedora,
			platform.DistributionCentOS,
			platform.DistributionRHEL,
			platform.DistributionArch,
			platform.DistributionOpenSUSE,
			platform.DistributionAlpine,
			platform.DistributionUnknown,
		}

		for _, dist := range testDistributions {
			managers := platform.GetPreferredPackageManagers(dist)
			if len(managers) == 0 {
				t.Errorf("No package managers returned for %s", dist)
			}
		}
	})

	t.Run("LinuxDistributionString", func(t *testing.T) {
		dist := platform.DistributionUbuntu
		if dist.String() != "ubuntu" {
			t.Errorf("Expected 'ubuntu', got %s", dist.String())
		}
	})
}

// TestPackageManagerCoverage - Test package manager functions that might lack coverage
func TestPackageManagerCoverage(t *testing.T) {
	t.Run("GetAvailablePackageManagers", func(t *testing.T) {
		managers := platform.GetAvailablePackageManagers()
		// Should have at least one manager on any platform
		if len(managers) == 0 {
			t.Error("No package managers available")
		}

		// Test that all returned managers are actually available
		for _, mgr := range managers {
			if !mgr.IsAvailable() {
				t.Errorf("Package manager %s claims to be available but isn't", mgr.GetName())
			}
		}
	})

	t.Run("GetBestPackageManager", func(t *testing.T) {
		best := platform.GetBestPackageManager()
		if best == nil {
			t.Error("GetBestPackageManager returned nil")
		}
		if !best.IsAvailable() {
			t.Error("Best package manager is not available")
		}
	})

	t.Run("InstallAndVerifyComponent", func(t *testing.T) {
		// Test verification without installation (safer for tests)
		result, err := platform.VerifyComponent("git")
		if err != nil {
			t.Logf("VerifyComponent failed (expected in test environment): %v", err)
		}
		if result == nil {
			t.Error("VerificationResult is nil")
		}
	})
}

// TestShellSupportCoverage - Test shell support edge cases
func TestShellSupportCoverage(t *testing.T) {
	t.Run("SupportsShellEdgeCases", func(t *testing.T) {
		testCases := []struct {
			shell    string
			expected bool
		}{
			// Path-based shells
			{"/bin/bash", platform.IsUnix()},
			{"/usr/bin/zsh", platform.IsUnix()},
			{"C:\\Windows\\System32\\cmd.exe", platform.IsWindows()},
			{"C:\\Program Files\\PowerShell\\7\\pwsh.exe", platform.IsWindows()},

			// Case variations
			{"BASH", platform.IsUnix()},
			{"CMD", platform.IsWindows()},
			{"PowerShell", platform.IsWindows()},

			// Invalid shells
			{"invalid_shell", false},
			{"", false},
		}

		for _, tc := range testCases {
			result := platform.SupportsShell(tc.shell)
			if result != tc.expected {
				t.Errorf("platform.SupportsShell(%s): expected %v, got %v", tc.shell, tc.expected, result)
			}
		}
	})
}

// TestErrorHandlingCoverage - Test error conditions that might lack coverage
func TestErrorHandlingCoverage(t *testing.T) {
	t.Run("HomeDirectoryErrors", func(t *testing.T) {
		// Save original environment
		originalHome := os.Getenv("HOME")
		originalUserProfile := os.Getenv("USERPROFILE")
		originalHomeDrive := os.Getenv("HOMEDRIVE")
		originalHomePath := os.Getenv("HOMEPATH")

		defer func() {
			// Restore environment
			_ = os.Setenv("HOME", originalHome)
			_ = os.Setenv("USERPROFILE", originalUserProfile)
			_ = os.Setenv("HOMEDRIVE", originalHomeDrive)
			_ = os.Setenv("HOMEPATH", originalHomePath)
		}()

		if platform.IsWindows() {
			// Clear all Windows home environment variables
			_ = os.Unsetenv("USERPROFILE")
			_ = os.Unsetenv("HOMEDRIVE")
			_ = os.Unsetenv("HOMEPATH")

			_, err := platform.GetHomeDirectory()
			if err == nil {
				t.Error("Expected error when no Windows home variables are set")
			}
		} else {
			// Clear Unix home environment variable
			_ = os.Unsetenv("HOME")

			_, err := platform.GetHomeDirectory()
			if err == nil {
				t.Error("Expected error when HOME is not set")
			}
		}
	})

	t.Run("DetectLinuxDistributionOnNonLinux", func(t *testing.T) {
		if platform.IsLinux() {
			t.Skip("Skipping non-Linux test on Linux platform")
		}

		_, err := platform.DetectLinuxDistribution()
		if err == nil {
			t.Error("Expected error when detecting Linux distribution on non-Linux platform")
		}
	})
}

// TestFileSystemFunctions - Test file system related functions
func TestFileSystemFunctions(t *testing.T) {
	t.Run("TempDirectoryVariations", func(t *testing.T) {
		// Save original environment
		originalTMPDIR := os.Getenv("TMPDIR")
		originalTMP := os.Getenv("TMP")
		originalTEMP := os.Getenv("TEMP")

		defer func() {
			// Restore environment
			if originalTMPDIR != "" {
				_ = os.Setenv("TMPDIR", originalTMPDIR)
			} else {
				_ = os.Unsetenv("TMPDIR")
			}
			if originalTMP != "" {
				_ = os.Setenv("TMP", originalTMP)
			} else {
				_ = os.Unsetenv("TMP")
			}
			if originalTEMP != "" {
				_ = os.Setenv("TEMP", originalTEMP)
			} else {
				_ = os.Unsetenv("TEMP")
			}
		}()

		// Test with custom TMPDIR
		customTemp := filepath.Join(os.TempDir(), "custom_temp")
		os.Setenv("TMPDIR", customTemp)
		temp := platform.GetTempDirectory()
		if temp != customTemp {
			t.Errorf("Expected custom temp directory %s, got %s", customTemp, temp)
		}

		// Clear all temp environment variables
		os.Unsetenv("TMPDIR")
		os.Unsetenv("TMP")
		os.Unsetenv("TEMP")

		temp = platform.GetTempDirectory()
		if platform.IsWindows() {
			if temp != "C:\\Windows\\Temp" {
				t.Errorf("Expected default Windows temp, got %s", temp)
			}
		} else {
			if temp != "/tmp" {
				t.Errorf("Expected /tmp on Unix, got %s", temp)
			}
		}
	})
}
