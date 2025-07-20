package platform

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestLinuxDistributionFunctions - Test Linux-specific functions thoroughly
func TestLinuxDistributionFunctions(t *testing.T) {
	if !IsLinux() {
		t.Skip("Skipping Linux-specific tests on non-Linux platform")
	}

	t.Run("readOSRelease", func(t *testing.T) {
		data, err := readOSRelease()
		if err != nil {
			t.Logf("readOSRelease failed (expected on some systems): %v", err)
		} else {
			if len(data) > 0 {
				t.Logf("OS Release data found: %d entries", len(data))
			}
		}
	})

	t.Run("readLSBRelease", func(t *testing.T) {
		data, err := readLSBRelease()
		if err != nil {
			t.Logf("readLSBRelease failed (expected on some systems): %v", err)
		} else {
			if len(data) > 0 {
				t.Logf("LSB Release data found: %d entries", len(data))
			}
		}
	})

	t.Run("readReleaseFile", func(t *testing.T) {
		// Test with a file that doesn't exist
		_, err := readReleaseFile("/nonexistent/file")
		if err == nil {
			t.Error("Expected error for non-existent file")
		}

		// Test with os-release if it exists
		if _, err := os.Stat("/etc/os-release"); err == nil {
			data, err := readReleaseFile("/etc/os-release")
			if err != nil {
				t.Errorf("Failed to read /etc/os-release: %v", err)
			}
			if len(data) == 0 {
				t.Error("Empty data from /etc/os-release")
			}
		}
	})

	t.Run("parseOSRelease", func(t *testing.T) {
		testData := map[string]string{
			"ID":         "ubuntu",
			"NAME":       "Ubuntu",
			"VERSION_ID": "20.04",
			"ID_LIKE":    "debian",
		}

		info := &LinuxInfo{}
		parseOSRelease(testData, info)

		if info.ID != "ubuntu" {
			t.Errorf("Expected ID 'ubuntu', got '%s'", info.ID)
		}
		if info.Name != "Ubuntu" {
			t.Errorf("Expected Name 'Ubuntu', got '%s'", info.Name)
		}
		if info.Version != "20.04" {
			t.Errorf("Expected Version '20.04', got '%s'", info.Version)
		}
		if len(info.IDLike) != 1 || info.IDLike[0] != "debian" {
			t.Errorf("Expected IDLike ['debian'], got %v", info.IDLike)
		}
	})

	t.Run("parseLSBRelease", func(t *testing.T) {
		testData := map[string]string{
			"DISTRIB_ID":          "Ubuntu",
			"DISTRIB_RELEASE":     "20.04",
			"DISTRIB_DESCRIPTION": "Ubuntu 20.04 LTS",
		}

		info := &LinuxInfo{}
		parseLSBRelease(testData, info)

		if info.ID != "ubuntu" {
			t.Errorf("Expected ID 'ubuntu', got '%s'", info.ID)
		}
		if info.Version != "20.04" {
			t.Errorf("Expected Version '20.04', got '%s'", info.Version)
		}
		if info.Name != "Ubuntu 20.04 LTS" {
			t.Errorf("Expected Name 'Ubuntu 20.04 LTS', got '%s'", info.Name)
		}
	})

	t.Run("mapIDToDistribution", func(t *testing.T) {
		testCases := map[string]LinuxDistribution{
			"ubuntu":    DistributionUbuntu,
			"UBUNTU":    DistributionUbuntu,
			"debian":    DistributionDebian,
			"fedora":    DistributionFedora,
			"centos":    DistributionCentOS,
			"rhel":      DistributionRHEL,
			"red":       DistributionRHEL,
			"redhat":    DistributionRHEL,
			"arch":      DistributionArch,
			"opensuse":  DistributionOpenSUSE,
			"suse":      DistributionOpenSUSE,
			"alpine":    DistributionAlpine,
			"unknown":   DistributionUnknown,
			"invalid":   DistributionUnknown,
		}

		for id, expected := range testCases {
			result := mapIDToDistribution(id)
			if result != expected {
				t.Errorf("mapIDToDistribution(%s): expected %s, got %s", id, expected, result)
			}
		}
	})

	t.Run("detectFromDistributionFiles", func(t *testing.T) {
		info := &LinuxInfo{}
		err := detectFromDistributionFiles(info)
		// This might succeed or fail depending on the system
		t.Logf("detectFromDistributionFiles result: %v", err)
	})

	t.Run("detectFromUname", func(t *testing.T) {
		info := &LinuxInfo{}
		err := detectFromUname(info)
		if err != nil {
			t.Logf("detectFromUname failed: %v", err)
		} else {
			t.Logf("Detected distribution via uname: %s", info.Distribution)
		}
	})
}

// TestPackageManagerHelperFunctions - Test helper functions in package managers
func TestPackageManagerHelperFunctions(t *testing.T) {
	t.Run("execCommand", func(t *testing.T) {
		// Test successful command
		result, err := execCommand("echo test", 5*time.Second)
		if err != nil {
			t.Errorf("execCommand failed: %v", err)
		}
		if !strings.Contains(result, "test") {
			t.Errorf("Expected 'test' in output, got '%s'", result)
		}

		// Test command timeout
		var longCmd string
		if IsWindows() {
			longCmd = "ping 127.0.0.1 -n 10"
		} else {
			longCmd = "sleep 2"
		}

		_, err = execCommand(longCmd, 100*time.Millisecond)
		if err == nil {
			t.Error("Expected timeout error")
		}
	})

	t.Run("extractVersion", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{"Version 1.2.3", "1.2.3"},
			{"version 2.0.0", "2.0.0"},
			{"VERSION 3.1.4", "3.1.4"},
			{"Go version go1.19.1 linux/amd64", "go1.19.1"},
			{"Python 3.9.0", "3.9.0"},
			{"v2.1.0", "v2.1.0"},
			{"No version info", "No version info"},
			{"", "unknown"},
			{"multiple lines\nversion 1.0.0\nmore text", "1.0.0"},
		}

		for _, tc := range testCases {
			result := extractVersion(tc.input)
			if result != tc.expected {
				t.Errorf("extractVersion(%q): expected %q, got %q", tc.input, tc.expected, result)
			}
		}
	})
}

// TestPackageManagerInstances - Test specific package manager instances
func TestPackageManagerInstances(t *testing.T) {
	t.Run("HomebrewManager", func(t *testing.T) {
		mgr := NewHomebrewManager()

		if mgr.GetName() != "homebrew" {
			t.Errorf("Expected name 'homebrew', got '%s'", mgr.GetName())
		}

		if mgr.RequiresAdmin() {
			t.Error("Homebrew should not require admin")
		}

		platforms := mgr.GetPlatforms()
		if len(platforms) != 1 || platforms[0] != "darwin" {
			t.Errorf("Expected platforms ['darwin'], got %v", platforms)
		}

		// Test package name mapping
		packageName := mgr.getPackageName("go")
		if packageName != "go" {
			t.Errorf("Expected 'go', got '%s'", packageName)
		}

		packageName = mgr.getPackageName("python")
		if packageName != "python@3.11" {
			t.Errorf("Expected 'python@3.11', got '%s'", packageName)
		}

		packageName = mgr.getPackageName("unknown")
		if packageName != "unknown" {
			t.Errorf("Expected 'unknown', got '%s'", packageName)
		}

		// Test verify command mapping
		verifyCmd := mgr.getVerifyCommand("go")
		if verifyCmd != "go version" {
			t.Errorf("Expected 'go version', got '%s'", verifyCmd)
		}

		verifyCmd = mgr.getVerifyCommand("unknown")
		if verifyCmd != "" {
			t.Errorf("Expected empty string for unknown command, got '%s'", verifyCmd)
		}

		// Test availability (should be false on non-Darwin)
		if runtime.GOOS != "darwin" {
			if mgr.IsAvailable() {
				t.Error("Homebrew should not be available on non-Darwin systems")
			}
		}
	})

	t.Run("AptManager", func(t *testing.T) {
		mgr := NewAptManager()

		if mgr.GetName() != "apt" {
			t.Errorf("Expected name 'apt', got '%s'", mgr.GetName())
		}

		if !mgr.RequiresAdmin() {
			t.Error("APT should require admin")
		}

		platforms := mgr.GetPlatforms()
		if len(platforms) != 1 || platforms[0] != "linux" {
			t.Errorf("Expected platforms ['linux'], got %v", platforms)
		}

		// Test package name mapping
		packageName := mgr.getPackageName("python")
		if packageName != "python3 python3-pip python3-venv" {
			t.Errorf("Expected 'python3 python3-pip python3-venv', got '%s'", packageName)
		}

		// Test verify command mapping
		verifyCmd := mgr.getVerifyCommand("python")
		if verifyCmd != "python3 --version" {
			t.Errorf("Expected 'python3 --version', got '%s'", verifyCmd)
		}
	})

	t.Run("WingetManager", func(t *testing.T) {
		mgr := NewWingetManager()

		if mgr.GetName() != "winget" {
			t.Errorf("Expected name 'winget', got '%s'", mgr.GetName())
		}

		if mgr.RequiresAdmin() {
			t.Error("Winget should not require admin")
		}

		platforms := mgr.GetPlatforms()
		if len(platforms) != 1 || platforms[0] != "windows" {
			t.Errorf("Expected platforms ['windows'], got %v", platforms)
		}

		// Test package name mapping
		packageName := mgr.getPackageName("go")
		if packageName != "GoLang.Go" {
			t.Errorf("Expected 'GoLang.Go', got '%s'", packageName)
		}

		// Test verify command mapping
		verifyCmd := mgr.getVerifyCommand("python")
		if verifyCmd != "python --version" {
			t.Errorf("Expected 'python --version', got '%s'", verifyCmd)
		}
	})

	t.Run("ChocolateyManager", func(t *testing.T) {
		mgr := NewChocolateyManager()

		if mgr.GetName() != "chocolatey" {
			t.Errorf("Expected name 'chocolatey', got '%s'", mgr.GetName())
		}

		if !mgr.RequiresAdmin() {
			t.Error("Chocolatey should require admin")
		}

		platforms := mgr.GetPlatforms()
		if len(platforms) != 1 || platforms[0] != "windows" {
			t.Errorf("Expected platforms ['windows'], got %v", platforms)
		}

		// Test package name mapping
		packageName := mgr.getPackageName("golang")
		if packageName != "golang" {
			t.Errorf("Expected 'golang', got '%s'", packageName)
		}
	})

	t.Run("YumManager", func(t *testing.T) {
		mgr := NewYumManager()

		if mgr.GetName() != "yum" {
			t.Errorf("Expected name 'yum', got '%s'", mgr.GetName())
		}

		if !mgr.RequiresAdmin() {
			t.Error("Yum should require admin")
		}

		platforms := mgr.GetPlatforms()
		if len(platforms) != 1 || platforms[0] != "linux" {
			t.Errorf("Expected platforms ['linux'], got %v", platforms)
		}
	})

	t.Run("DnfManager", func(t *testing.T) {
		mgr := NewDnfManager()

		if mgr.GetName() != "dnf" {
			t.Errorf("Expected name 'dnf', got '%s'", mgr.GetName())
		}

		if !mgr.RequiresAdmin() {
			t.Error("DNF should require admin")
		}

		platforms := mgr.GetPlatforms()
		if len(platforms) != 1 || platforms[0] != "linux" {
			t.Errorf("Expected platforms ['linux'], got %v", platforms)
		}
	})
}

// TestPackageManagerSelectionFunctions - Test package manager selection logic
func TestPackageManagerSelectionFunctions(t *testing.T) {
	t.Run("getBestDarwinPackageManager", func(t *testing.T) {
		// Create mock managers
		homebrew := NewHomebrewManager()
		apt := NewAptManager()
		
		available := []PackageManager{apt, homebrew}
		
		best := getBestDarwinPackageManager(available)
		if best != homebrew {
			t.Error("Expected homebrew to be best Darwin package manager")
		}

		// Test with no homebrew
		available = []PackageManager{apt}
		best = getBestDarwinPackageManager(available)
		if best != apt {
			t.Error("Expected first available manager when homebrew not available")
		}
	})

	t.Run("getBestLinuxPackageManager", func(t *testing.T) {
		apt := NewAptManager()
		dnf := NewDnfManager()
		yum := NewYumManager()

		// Test preference order: apt, dnf, yum
		available := []PackageManager{yum, dnf, apt}
		best := getBestLinuxPackageManager(available)
		if best != apt {
			t.Error("Expected apt to be preferred")
		}

		available = []PackageManager{yum, dnf}
		best = getBestLinuxPackageManager(available)
		if best != dnf {
			t.Error("Expected dnf to be preferred over yum")
		}

		available = []PackageManager{yum}
		best = getBestLinuxPackageManager(available)
		if best != yum {
			t.Error("Expected yum when only yum available")
		}
	})

	t.Run("getBestWindowsPackageManager", func(t *testing.T) {
		winget := NewWingetManager()
		choco := NewChocolateyManager()

		// Test preference order: winget, chocolatey
		available := []PackageManager{choco, winget}
		best := getBestWindowsPackageManager(available)
		if best != winget {
			t.Error("Expected winget to be preferred")
		}

		available = []PackageManager{choco}
		best = getBestWindowsPackageManager(available)
		if best != choco {
			t.Error("Expected chocolatey when only chocolatey available")
		}
	})

	t.Run("findPackageManagerByName", func(t *testing.T) {
		apt := NewAptManager()
		dnf := NewDnfManager()
		available := []PackageManager{apt, dnf}

		found := findPackageManagerByName(available, "apt")
		if found != apt {
			t.Error("Expected to find apt manager")
		}

		found = findPackageManagerByName(available, "nonexistent")
		if found != nil {
			t.Error("Expected nil for non-existent manager")
		}
	})
}

// TestExecutorEdgeCases - Test executor edge cases and error conditions
func TestExecutorEdgeCases(t *testing.T) {
	t.Run("WindowsExecutorShellSelection", func(t *testing.T) {
		if !IsWindows() {
			t.Skip("Skipping Windows-specific test")
		}

		executor := &windowsExecutor{}
		shell := executor.GetShell()
		
		// Should return cmd, powershell, or pwsh
		validShells := []string{"cmd", "powershell", "pwsh"}
		isValid := false
		for _, validShell := range validShells {
			if shell == validShell {
				isValid = true
				break
			}
		}
		if !isValid {
			t.Errorf("Unexpected Windows shell: %s", shell)
		}

		// Test shell args for different shells
		args := executor.GetShellArgs("echo test")
		if len(args) == 0 {
			t.Error("No shell args returned")
		}
	})

	t.Run("UnixExecutorShellSelection", func(t *testing.T) {
		if IsWindows() {
			t.Skip("Skipping Unix-specific test")
		}

		executor := &unixExecutor{}
		
		// Save original SHELL environment variable
		originalShell := os.Getenv("SHELL")
		defer func() {
			if originalShell != "" {
				os.Setenv("SHELL", originalShell)
			} else {
				os.Unsetenv("SHELL")
			}
		}()

		// Test with custom SHELL
		os.Setenv("SHELL", "/bin/zsh")
		shell := executor.GetShell()
		if shell != "/bin/zsh" {
			t.Errorf("Expected /bin/zsh, got %s", shell)
		}

		// Test with no SHELL environment variable
		os.Unsetenv("SHELL")
		shell = executor.GetShell()
		if shell != "bash" && shell != "sh" {
			t.Errorf("Expected bash or sh, got %s", shell)
		}

		// Test shell args for different shells
		args := executor.GetShellArgs("echo test")
		if len(args) == 0 {
			t.Error("No shell args returned")
		}
		
		// Should contain -c
		found := false
		for _, arg := range args {
			if arg == "-c" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected -c in shell args")
		}
	})
}

// TestHomeDirectoryEdgeCases - Test home directory detection edge cases
func TestHomeDirectoryEdgeCases(t *testing.T) {
	// Save original environment variables
	originalHome := os.Getenv("HOME")
	originalUserProfile := os.Getenv("USERPROFILE")
	originalHomeDrive := os.Getenv("HOMEDRIVE")
	originalHomePath := os.Getenv("HOMEPATH")

	defer func() {
		// Restore environment
		if originalHome != "" {
			os.Setenv("HOME", originalHome)
		} else {
			os.Unsetenv("HOME")
		}
		if originalUserProfile != "" {
			os.Setenv("USERPROFILE", originalUserProfile)
		} else {
			os.Unsetenv("USERPROFILE")
		}
		if originalHomeDrive != "" {
			os.Setenv("HOMEDRIVE", originalHomeDrive)
		} else {
			os.Unsetenv("HOMEDRIVE")
		}
		if originalHomePath != "" {
			os.Setenv("HOMEPATH", originalHomePath)
		} else {
			os.Unsetenv("HOMEPATH")
		}
	}()

	t.Run("WindowsHomeDetection", func(t *testing.T) {
		if !IsWindows() {
			t.Skip("Skipping Windows-specific test")
		}

		// Test USERPROFILE
		os.Setenv("USERPROFILE", "C:\\Users\\testuser")
		os.Unsetenv("HOMEDRIVE")
		os.Unsetenv("HOMEPATH")

		home, err := GetHomeDirectory()
		if err != nil {
			t.Errorf("GetHomeDirectory failed with USERPROFILE: %v", err)
		}
		if home != "C:\\Users\\testuser" {
			t.Errorf("Expected C:\\Users\\testuser, got %s", home)
		}

		// Test HOMEDRIVE + HOMEPATH
		os.Unsetenv("USERPROFILE")
		os.Setenv("HOMEDRIVE", "C:")
		os.Setenv("HOMEPATH", "\\Users\\testuser")

		home, err = GetHomeDirectory()
		if err != nil {
			t.Errorf("GetHomeDirectory failed with HOMEDRIVE+HOMEPATH: %v", err)
		}
		if home != "C:\\Users\\testuser" {
			t.Errorf("Expected C:\\Users\\testuser, got %s", home)
		}

		// Test partial HOMEDRIVE/HOMEPATH
		os.Setenv("HOMEDRIVE", "C:")
		os.Unsetenv("HOMEPATH")

		_, err = GetHomeDirectory()
		if err == nil {
			t.Error("Expected error with incomplete HOMEDRIVE/HOMEPATH")
		}
	})

	t.Run("UnixHomeDetection", func(t *testing.T) {
		if IsWindows() {
			t.Skip("Skipping Unix-specific test")
		}

		// Test with HOME environment variable
		os.Setenv("HOME", "/home/testuser")

		home, err := GetHomeDirectory()
		if err != nil {
			t.Errorf("GetHomeDirectory failed with HOME: %v", err)
		}
		if home != "/home/testuser" {
			t.Errorf("Expected /home/testuser, got %s", home)
		}
	})
}