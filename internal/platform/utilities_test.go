package platform

import (
	"os"
	"strings"
	"testing"
)

// Test GetHomeDirectory Windows-specific logic with comprehensive mocking
func TestGetHomeDirectoryWindowsLogic(t *testing.T) {
	if !IsWindows() {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	// Save original environment variables
	originalUserProfile := os.Getenv("USERPROFILE")
	originalHomeDrive := os.Getenv("HOMEDRIVE")
	originalHomePath := os.Getenv("HOMEPATH")

	defer func() {
		restoreEnv(t, "USERPROFILE", originalUserProfile)
		restoreEnv(t, "HOMEDRIVE", originalHomeDrive)
		restoreEnv(t, "HOMEPATH", originalHomePath)
	}()

	t.Run("USERPROFILE priority over HOMEDRIVE+HOMEPATH", func(t *testing.T) {
		t.Setenv("USERPROFILE", "C:\\Users\\priority")
		t.Setenv("HOMEDRIVE", "D:")
		t.Setenv("HOMEPATH", "\\Users\\fallback")

		result, err := GetHomeDirectory()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		if result != "C:\\Users\\priority" {
			t.Errorf("Expected USERPROFILE to take priority, got: %s", result)
		}
	})

	t.Run("HOMEDRIVE+HOMEPATH concatenation", func(t *testing.T) {
		t.Setenv("USERPROFILE", "")
		t.Setenv("HOMEDRIVE", "C:")
		t.Setenv("HOMEPATH", "\\Users\\testuser")

		result, err := GetHomeDirectory()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		expected := "C:\\Users\\testuser"
		if result != expected {
			t.Errorf("Expected %s, got %s", expected, result)
		}
	})

	t.Run("Missing HOMEPATH with HOMEDRIVE", func(t *testing.T) {
		t.Setenv("USERPROFILE", "")
		t.Setenv("HOMEDRIVE", "C:")
		t.Setenv("HOMEPATH", "")

		_, err := GetHomeDirectory()
		if err == nil {
			t.Error("Expected error when HOMEPATH is missing")
		}
	})

	t.Run("Missing HOMEDRIVE with HOMEPATH", func(t *testing.T) {
		t.Setenv("USERPROFILE", "")
		t.Setenv("HOMEDRIVE", "")
		t.Setenv("HOMEPATH", "\\Users\\testuser")

		_, err := GetHomeDirectory()
		if err == nil {
			t.Error("Expected error when HOMEDRIVE is missing")
		}
	})
}

// Test GetHomeDirectory Unix-specific logic with HOME variable
func TestGetHomeDirectoryUnixLogic(t *testing.T) {
	if IsWindows() {
		t.Skip("Skipping Unix-specific test on Windows platform")
	}

	originalHome := os.Getenv("HOME")
	defer restoreEnv(t, "HOME", originalHome)

	t.Run("HOME variable set", func(t *testing.T) {
		t.Setenv("HOME", "/home/testuser")

		result, err := GetHomeDirectory()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		if result != "/home/testuser" {
			t.Errorf("Expected /home/testuser, got %s", result)
		}
	})

	t.Run("HOME variable empty", func(t *testing.T) {
		t.Setenv("HOME", "")

		_, err := GetHomeDirectory()
		if err == nil {
			t.Error("Expected error when HOME is empty")
		}
	})
}

// Test GetTempDirectory environment variable precedence
func TestGetTempDirectoryPrecedence(t *testing.T) {
	originalTmpDir := os.Getenv("TMPDIR")
	originalTmp := os.Getenv("TMP")
	originalTemp := os.Getenv("TEMP")

	defer func() {
		restoreEnv(t, "TMPDIR", originalTmpDir)
		restoreEnv(t, "TMP", originalTmp)
		restoreEnv(t, "TEMP", originalTemp)
	}()

	t.Run("TMPDIR has highest precedence", func(t *testing.T) {
		t.Setenv("TMPDIR", "/highest/priority")
		t.Setenv("TMP", "/should/not/use")
		t.Setenv("TEMP", "/should/not/use/either")

		result := GetTempDirectory()
		if result != "/highest/priority" {
			t.Errorf("Expected TMPDIR to have precedence, got: %s", result)
		}
	})

	t.Run("TMP used when TMPDIR empty", func(t *testing.T) {
		t.Setenv("TMPDIR", "")
		t.Setenv("TMP", "/second/priority")
		t.Setenv("TEMP", "/should/not/use")

		result := GetTempDirectory()
		if result != "/second/priority" {
			t.Errorf("Expected TMP to be used, got: %s", result)
		}
	})

	t.Run("TEMP used when TMPDIR and TMP empty", func(t *testing.T) {
		t.Setenv("TMPDIR", "")
		t.Setenv("TMP", "")
		t.Setenv("TEMP", "/third/priority")

		result := GetTempDirectory()
		if result != "/third/priority" {
			t.Errorf("Expected TEMP to be used, got: %s", result)
		}
	})

	t.Run("Platform fallback when all empty", func(t *testing.T) {
		t.Setenv("TMPDIR", "")
		t.Setenv("TMP", "")
		t.Setenv("TEMP", "")

		result := GetTempDirectory()
		expected := "/tmp"
		if IsWindows() {
			expected = "C:\\Windows\\Temp"
		}

		if result != expected {
			t.Errorf("Expected platform fallback %s, got: %s", expected, result)
		}
	})
}

// Test SupportsShell with specific Windows shell variations
func TestSupportsShellWindowsSpecific(t *testing.T) {
	if !IsWindows() {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	windowsShells := []struct {
		shell    string
		expected bool
	}{
		{"cmd", true},
		{"cmd.exe", true},
		{"CMD", true},
		{"CMD.EXE", true},
		{"powershell", true},
		{"powershell.exe", true},
		{"POWERSHELL", true},
		{"POWERSHELL.EXE", true},
		{"pwsh", true},
		{"pwsh.exe", true},
		{"PWSH", true},
		{"PWSH.EXE", true},
		{"C:\\Windows\\System32\\cmd.exe", true},
		{"C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe", true},
		{"C:\\Program Files\\PowerShell\\7\\pwsh.exe", true},
		{"bash", false},
		{"sh", false},
		{"zsh", false},
		{"invalid", false},
	}

	for _, test := range windowsShells {
		t.Run(test.shell, func(t *testing.T) {
			result := SupportsShell(test.shell)
			if result != test.expected {
				t.Errorf("SupportsShell(%q) on Windows: expected %v, got %v",
					test.shell, test.expected, result)
			}
		})
	}
}

// Test SupportsShell with specific Unix shell variations
func TestSupportsShellUnixSpecific(t *testing.T) {
	if IsWindows() {
		t.Skip("Skipping Unix-specific test on Windows platform")
	}

	unixShells := []struct {
		shell    string
		expected bool
	}{
		{"sh", true},
		{"bash", true},
		{"zsh", true},
		{"dash", true},
		{"ksh", true},
		{"fish", true},
		{"SH", true},
		{"BASH", true},
		{"ZSH", true},
		{"/bin/bash", true},
		{"/usr/bin/zsh", true},
		{"/usr/local/bin/fish", true},
		{"./bash", true},
		{"../bin/sh", true},
		{"cmd", false},
		{"cmd.exe", false},
		{"powershell", false},
		{"invalid", false},
	}

	for _, test := range unixShells {
		t.Run(test.shell, func(t *testing.T) {
			result := SupportsShell(test.shell)
			if result != test.expected {
				t.Errorf("SupportsShell(%q) on Unix: expected %v, got %v",
					test.shell, test.expected, result)
			}
		})
	}
}

// Test SupportsShell case insensitivity
func TestSupportsShellCaseInsensitive(t *testing.T) {
	caseMixtures := []string{
		"bash", "BASH", "Bash", "bAsH", "BaSh",
		"cmd", "CMD", "Cmd", "cMd", "CmD",
		"powershell", "POWERSHELL", "PowerShell", "powerSHELL",
		"zsh", "ZSH", "Zsh", "zSh", "ZsH",
	}

	for _, shell := range caseMixtures {
		t.Run(shell, func(t *testing.T) {
			result := SupportsShell(shell)

			// Verify case insensitivity works
			lowercase := strings.ToLower(shell)
			expectedResult := SupportsShell(lowercase)

			if result != expectedResult {
				t.Errorf("Case insensitivity failed for %q: expected %v, got %v",
					shell, expectedResult, result)
			}
		})
	}
}

// Test GetPreferredPackageManagers ordering for multi-manager distributions
func TestGetPreferredPackageManagersOrdering(t *testing.T) {
	multiManagerTests := []struct {
		distribution   LinuxDistribution
		expectedFirst  string
		expectedLength int
	}{
		{DistributionCentOS, "dnf", 2},
		{DistributionRHEL, "dnf", 2},
		{DistributionUnknown, "apt", 3},
	}

	for _, test := range multiManagerTests {
		t.Run(string(test.distribution), func(t *testing.T) {
			result := GetPreferredPackageManagers(test.distribution)

			if len(result) != test.expectedLength {
				t.Errorf("Expected %d package managers, got %d", test.expectedLength, len(result))
				return
			}

			if result[0] != test.expectedFirst {
				t.Errorf("Expected first package manager to be %s, got %s",
					test.expectedFirst, result[0])
			}

			// Verify no duplicates
			seen := make(map[string]bool)
			for _, manager := range result {
				if seen[manager] {
					t.Errorf("Duplicate package manager found: %s", manager)
				}
				seen[manager] = true
			}
		})
	}
}

// Test platform path utilities for better coverage
func TestPlatformPathUtilities(t *testing.T) {
	t.Run("GetExecutableExtension cross-platform", func(t *testing.T) {
		ext := GetExecutableExtension()
		if IsWindows() {
			if ext != ".exe" {
				t.Errorf("Expected .exe on Windows, got %s", ext)
			}
		} else {
			if ext != "" {
				t.Errorf("Expected empty extension on Unix, got %s", ext)
			}
		}
	})

	t.Run("GetPathSeparator cross-platform", func(t *testing.T) {
		sep := GetPathSeparator()
		if IsWindows() {
			if sep != "\\" {
				t.Errorf("Expected \\\\ on Windows, got %s", sep)
			}
		} else {
			if sep != "/" {
				t.Errorf("Expected / on Unix, got %s", sep)
			}
		}
	})

	t.Run("GetPathListSeparator cross-platform", func(t *testing.T) {
		sep := GetPathListSeparator()
		if IsWindows() {
			if sep != ";" {
				t.Errorf("Expected ; on Windows, got %s", sep)
			}
		} else {
			if sep != ":" {
				t.Errorf("Expected : on Unix, got %s", sep)
			}
		}
	})

	t.Run("GetPlatformString format", func(t *testing.T) {
		platformStr := GetPlatformString()
		if platformStr == "" {
			t.Error("GetPlatformString should not be empty")
		}

		if !strings.Contains(platformStr, "-") {
			t.Errorf("Expected platform-arch format, got %s", platformStr)
		}
	})
}

// Test SupportsShell edge cases for better coverage
func TestSupportsShellAdditionalCases(t *testing.T) {
	edgeCases := []struct {
		shell    string
		expected bool
		reason   string
	}{
		{"", false, "empty string"},
		{" ", false, "space only"},
		{"   ", false, "multiple spaces"},
		{"\t", false, "tab character"},
		{"\n", false, "newline character"},
		{"shell_with_underscore", false, "invalid shell name"},
		{"shell-with-dash", false, "invalid shell name"},
		{"shell123", false, "shell with numbers"},
		{"bash\n", false, "bash with trailing newline"},
		{"cmd\r", false, "cmd with carriage return"},
		{"./sh", IsUnix(), "relative path sh"},
		{"../bash", IsUnix(), "parent directory bash"},
		{"C:\\cmd.exe", IsWindows(), "Windows absolute path"},
		{"\\windows\\system32\\cmd.exe", IsWindows(), "Windows path without drive"},
	}

	for _, test := range edgeCases {
		t.Run(test.reason, func(t *testing.T) {
			result := SupportsShell(test.shell)
			if result != test.expected {
				t.Errorf("SupportsShell(%q): expected %v, got %v (%s)",
					test.shell, test.expected, result, test.reason)
			}
		})
	}
}

// Test GetTempDirectory edge cases to reach higher coverage
func TestGetTempDirectoryEdgeCases(t *testing.T) {
	originalTmpDir := os.Getenv("TMPDIR")
	originalTmp := os.Getenv("TMP")
	originalTemp := os.Getenv("TEMP")

	defer func() {
		restoreEnv(t, "TMPDIR", originalTmpDir)
		restoreEnv(t, "TMP", originalTmp)
		restoreEnv(t, "TEMP", originalTemp)
	}()

	t.Run("All environment variables with whitespace", func(t *testing.T) {
		t.Setenv("TMPDIR", "   ")
		t.Setenv("TMP", "\t")
		t.Setenv("TEMP", "\n")

		result := GetTempDirectory()
		// The function returns the first non-empty env var, so it should return "   "
		expected := "   "

		if result != expected {
			t.Errorf("Expected whitespace value '   ', got %q", result)
		}
	})

	t.Run("Mixed empty and valid values", func(t *testing.T) {
		t.Setenv("TMPDIR", "")
		t.Setenv("TMP", "")
		t.Setenv("TEMP", "/valid/temp")

		result := GetTempDirectory()
		if result != "/valid/temp" {
			t.Errorf("Expected /valid/temp, got %s", result)
		}
	})
}

// Test GetHomeDirectory additional error scenarios for better coverage
func TestGetHomeDirectoryAdditionalScenarios(t *testing.T) {
	originalHome := os.Getenv("HOME")
	originalUserProfile := os.Getenv("USERPROFILE")
	originalHomeDrive := os.Getenv("HOMEDRIVE")
	originalHomePath := os.Getenv("HOMEPATH")

	defer func() {
		restoreEnv(t, "HOME", originalHome)
		restoreEnv(t, "USERPROFILE", originalUserProfile)
		restoreEnv(t, "HOMEDRIVE", originalHomeDrive)
		restoreEnv(t, "HOMEPATH", originalHomePath)
	}()

	if IsWindows() {
		t.Run("Windows empty USERPROFILE with valid HOMEDRIVE+HOMEPATH", func(t *testing.T) {
			t.Setenv("USERPROFILE", "")
			t.Setenv("HOMEDRIVE", "D:")
			t.Setenv("HOMEPATH", "\\Users\\altuser")

			result, err := GetHomeDirectory()
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result != "D:\\Users\\altuser" {
				t.Errorf("Expected D:\\Users\\altuser, got %s", result)
			}
		})

		t.Run("Windows whitespace USERPROFILE", func(t *testing.T) {
			t.Setenv("USERPROFILE", "   ")
			t.Setenv("HOMEDRIVE", "C:")
			t.Setenv("HOMEPATH", "\\Users\\test")

			result, err := GetHomeDirectory()
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Function returns the whitespace USERPROFILE value literally
			if result != "   " {
				t.Errorf("Expected whitespace USERPROFILE '   ', got %q", result)
			}
		})
	} else {
		t.Run("Unix whitespace HOME variable", func(t *testing.T) {
			t.Setenv("HOME", "   ")

			result, err := GetHomeDirectory()
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Function returns the whitespace HOME value literally
			if result != "   " {
				t.Errorf("Expected whitespace HOME '   ', got %q", result)
			}
		})
	}
}

// Helper functions

func restoreEnv(t *testing.T, key, value string) {
	if value == "" {
		if err := os.Unsetenv(key); err != nil {
			t.Logf("Warning: Failed to unset %s: %v", key, err)
		}
	} else {
		if err := os.Setenv(key, value); err != nil {
			t.Logf("Warning: Failed to restore %s: %v", key, err)
		}
	}
}
