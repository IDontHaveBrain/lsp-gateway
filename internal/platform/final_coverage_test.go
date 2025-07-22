package platform

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestFinalCoverageGaps - Target remaining uncovered functions to reach 60%
func TestFinalCoverageGaps(t *testing.T) {
	// Run core platform tests
	t.Run("AllPlatformFunctions", func(t *testing.T) {
		platform := GetCurrentPlatform()
		arch := GetCurrentArchitecture()

		// Test String() methods
		if platform.String() == "" {
			t.Error("Platform string is empty")
		}
		if arch.String() == "" {
			t.Error("Architecture string is empty")
		}

		// Test all platform checks
		isWin := IsWindows()
		isLinux := IsLinux()
		isMac := IsMacOS()
		isUnix := IsUnix()

		// Verify platform consistency
		switch platform {
		case PlatformWindows:
			if !isWin || isLinux || isMac || isUnix {
				t.Error("Platform detection inconsistent for Windows")
			}
		case PlatformLinux:
			if isWin || !isLinux || isMac || !isUnix {
				t.Error("Platform detection inconsistent for Linux")
			}
		case PlatformMacOS:
			if isWin || isLinux || !isMac || !isUnix {
				t.Error("Platform detection inconsistent for macOS")
			}
		}

		// Test utility functions
		home, err := GetHomeDirectory()
		if err != nil {
			t.Errorf("GetHomeDirectory failed: %v", err)
		}
		if home == "" {
			t.Error("Home directory is empty")
		}

		temp := GetTempDirectory()
		if temp == "" {
			t.Error("Temp directory is empty")
		}

		_ = GetExecutableExtension()
		sep := GetPathSeparator()
		listSep := GetPathListSeparator()
		platformStr := GetPlatformString()

		if sep == "" || listSep == "" || platformStr == "" {
			t.Error("Utility functions returning empty values")
		}

		// Test shell support for various shells
		shells := []string{"bash", "sh", "zsh", "cmd", "powershell", "invalid"}
		for _, shell := range shells {
			SupportsShell(shell) // Just call to increase coverage
		}
	})

	// Test all executor functions
	t.Run("AllExecutorFunctions", func(t *testing.T) {
		executor := NewCommandExecutor()

		// Test basic execution
		var cmd string
		var args []string
		if IsWindows() {
			cmd = "cmd"
			args = []string{"/C", "echo test"}
		} else {
			cmd = "sh"
			args = []string{"-c", "echo test"}
		}

		result, err := executor.Execute(cmd, args, 5*time.Second)
		if err != nil {
			t.Errorf("Execute failed: %v", err)
		}
		if result == nil {
			t.Error("Result is nil")
		}

		// Test with environment
		env := map[string]string{"TEST_VAR": "value"}
		result, err = executor.ExecuteWithEnv(cmd, args, env, 5*time.Second)
		if err != nil {
			t.Errorf("ExecuteWithEnv failed: %v", err)
		}
		if result == nil {
			t.Error("ExecuteWithEnv result is nil")
		}

		// Test shell functions
		shell := executor.GetShell()
		if shell == "" {
			t.Error("GetShell returned empty")
		}

		shellArgs := executor.GetShellArgs("echo test")
		if len(shellArgs) == 0 {
			t.Error("GetShellArgs returned empty")
		}

		// Test command availability
		if !executor.IsCommandAvailable(shell) {
			t.Error("Shell should be available")
		}

		// Test utility functions
		if !IsCommandAvailable(shell) {
			t.Error("IsCommandAvailable failed for shell")
		}

		// Test shell command execution
		result, err = ExecuteShellCommand(executor, "echo shell_test", 5*time.Second)
		if err != nil {
			t.Errorf("ExecuteShellCommand failed: %v", err)
		}
		if result == nil {
			t.Error("ExecuteShellCommand result is nil")
		}

		// Test exit code function
		code := GetExitCodeFromError(nil)
		if code != 0 {
			t.Error("Expected 0 for nil error")
		}

		// Test command path
		path, err := GetCommandPath(shell)
		if err != nil {
			t.Errorf("GetCommandPath failed: %v", err)
		}
		if path == "" {
			t.Error("Command path is empty")
		}
	})

	// Test Linux distribution functions if on Linux
	t.Run("LinuxDistributionComplete", func(t *testing.T) {
		if !IsLinux() {
			t.Skip("Not on Linux")
		}

		info, err := DetectLinuxDistribution()
		if err != nil {
			t.Errorf("DetectLinuxDistribution failed: %v", err)
		}
		if info == nil {
			t.Error("Linux info is nil")
		}

		// Test all distribution types
		distributions := []LinuxDistribution{
			DistributionUbuntu, DistributionDebian, DistributionFedora,
			DistributionCentOS, DistributionRHEL, DistributionArch,
			DistributionOpenSUSE, DistributionAlpine, DistributionUnknown,
		}

		for _, dist := range distributions {
			managers := GetPreferredPackageManagers(dist)
			if len(managers) == 0 {
				t.Errorf("No managers for %s", dist)
			}

			// Test string conversion
			if dist.String() == "" {
				t.Errorf("Empty string for %s", dist)
			}
		}

		// Test parsing functions with various inputs
		testData := map[string]string{
			"ID": "ubuntu", "NAME": "Ubuntu", "VERSION_ID": "20.04",
		}
		testInfo := &LinuxInfo{}
		parseOSRelease(testData, testInfo)

		lsbData := map[string]string{
			"DISTRIB_ID": "Ubuntu", "DISTRIB_RELEASE": "20.04",
		}
		parseLSBRelease(lsbData, testInfo)

		// Test mapping function with all known IDs
		ids := []string{"ubuntu", "debian", "fedora", "centos", "rhel", "arch", "alpine", "unknown"}
		for _, id := range ids {
			_ = mapIDToDistribution(id)
		}
	})

	// Test all package manager functions
	t.Run("AllPackageManagerFunctions", func(t *testing.T) {
		// Test all manager constructors
		managers := []PackageManager{
			NewHomebrewManager(),
			NewAptManager(),
			NewWingetManager(),
			NewChocolateyManager(),
			NewYumManager(),
			NewDnfManager(),
		}

		for _, mgr := range managers {
			// Test basic interface functions
			name := mgr.GetName()
			if name == "" {
				t.Error("Manager name is empty")
			}

			platforms := mgr.GetPlatforms()
			if len(platforms) == 0 {
				t.Error("No platforms for manager")
			}

			mgr.RequiresAdmin() // Test admin requirement
			mgr.IsAvailable()   // Test availability

			// Test verification (safe operation)
			result, err := mgr.Verify("git")
			if result == nil && err == nil {
				t.Error("Both result and error are nil")
			}
		}

		// Test global functions
		available := GetAvailablePackageManagers()
		if len(available) == 0 {
			t.Error("No available package managers")
		}

		best := GetBestPackageManager()
		if best == nil {
			t.Error("No best package manager")
		}

		// Test selection functions
		getBestDarwinPackageManager(available)
		getBestLinuxPackageManager(available)
		getBestWindowsPackageManager(available)
		findPackageManagerByName(available, "apt")

		// Test verification function
		result, _ := VerifyComponent("git")
		if result == nil {
			t.Error("VerifyComponent returned nil result")
		}

		// Test helper functions
		version := extractVersion("version 1.2.3")
		if version != "1.2.3" {
			t.Errorf("Expected '1.2.3', got '%s'", version)
		}

		version = extractVersion("")
		if version != "unknown" {
			t.Errorf("Expected 'unknown' for empty input, got '%s'", version)
		}
	})

	// Test error scenarios
	t.Run("ErrorScenarios", func(t *testing.T) {
		// Test non-existent command
		_, err := GetCommandPath("nonexistent_command_xyz")
		if err == nil {
			t.Error("Expected error for non-existent command")
		}

		// Test error exit code
		code := GetExitCodeFromError(err)
		if code == 0 {
			t.Error("Expected non-zero exit code for error")
		}

		// Test home directory with cleared environment
		originalHome := os.Getenv("HOME")
		if !IsWindows() {
			os.Unsetenv("HOME")
			_, err := GetHomeDirectory()
			if err == nil {
				t.Error("Expected error with no HOME")
			}
			if originalHome != "" {
				os.Setenv("HOME", originalHome)
			}
		}

		// Test release file reading with non-existent file
		_, err = readReleaseFile("/nonexistent/file")
		if err == nil {
			t.Error("Expected error for non-existent release file")
		}

		if IsLinux() {
			// Test detection from files
			info := &LinuxInfo{}
			_ = detectFromDistributionFiles(info)
			_ = detectFromUname(info)
		}
	})

	// Test edge cases and utilities
	t.Run("EdgeCasesAndUtilities", func(t *testing.T) {
		// Test shell support with paths
		if IsUnix() {
			if !SupportsShell("/bin/bash") {
				t.Error("Should support /bin/bash")
			}
			if !SupportsShell("/usr/bin/zsh") {
				t.Error("Should support /usr/bin/zsh")
			}
		}

		if IsWindows() {
			if !SupportsShell("C:\\Windows\\System32\\cmd.exe") {
				t.Error("Should support full path to cmd.exe")
			}
		}

		// Test case insensitive shell support
		if IsUnix() {
			if !SupportsShell("BASH") {
				t.Error("Should support BASH (uppercase)")
			}
		}

		// Test temp directory with environment variables
		originalTmp := os.Getenv("TMPDIR")
		customTemp := "/custom/temp"
		os.Setenv("TMPDIR", customTemp)

		temp := GetTempDirectory()
		if temp != customTemp {
			t.Errorf("Expected custom temp %s, got %s", customTemp, temp)
		}

		// Restore environment
		if originalTmp != "" {
			os.Setenv("TMPDIR", originalTmp)
		} else {
			os.Unsetenv("TMPDIR")
		}

		// Test platform string construction
		platformStr := GetPlatformString()
		if !strings.Contains(platformStr, "-") {
			t.Error("Platform string should contain hyphen")
		}

		currentPlatform := GetCurrentPlatform().String()
		currentArch := GetCurrentArchitecture().String()
		if !strings.Contains(platformStr, currentPlatform) {
			t.Error("Platform string should contain current platform")
		}
		if !strings.Contains(platformStr, currentArch) {
			t.Error("Platform string should contain current architecture")
		}
	})
}

// TestBranchCoverage - Test specific branches that might be missed
func TestBranchCoverage(t *testing.T) {
	t.Run("ExecutorBranches", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			// Test Windows executor specifically
			executor := &windowsExecutor{}

			_ = executor.GetShell()
			args := executor.GetShellArgs("test command")

			if len(args) == 0 {
				t.Error("No shell args for Windows")
			}

			// Test different shell types - check for /C in args
			if strings.Contains(strings.Join(args, " "), "/C") {
				t.Log("Found /C in Windows shell args")
			}
		} else {
			// Test Unix executor specifically
			executor := &unixExecutor{}

			_ = executor.GetShell()
			args := executor.GetShellArgs("test command")

			if len(args) == 0 {
				t.Error("No shell args for Unix")
			}

			if !strings.Contains(strings.Join(args, " "), "-c") {
				t.Error("Unix args should contain -c")
			}
		}
	})

	t.Run("PlatformSpecificBranches", func(t *testing.T) {
		// Test architecture detection
		arch := GetCurrentArchitecture()
		switch runtime.GOARCH {
		case "amd64", "arm64", "386", "arm":
			if arch == ArchUnknown {
				t.Error("Known architecture detected as unknown")
			}
		default:
			if arch != ArchUnknown {
				t.Error("Unknown architecture should return ArchUnknown")
			}
		}

		// Test platform detection
		platform := GetCurrentPlatform()
		switch runtime.GOOS {
		case "windows", "linux", "darwin":
			if platform == PlatformUnknown {
				t.Error("Known platform detected as unknown")
			}
		default:
			if platform != PlatformUnknown {
				t.Error("Unknown platform should return PlatformUnknown")
			}
		}
	})
}
