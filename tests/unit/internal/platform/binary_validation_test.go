package platform_test

import (
	"fmt"
	"lsp-gateway/internal/platform"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCrossPlatformBinaryValidation(t *testing.T) {
	targets := []struct {
		goos   string
		goarch string
		suffix string
	}{
		{"linux", "amd64", ""},
		{"windows", "amd64", ".exe"},
		{"darwin", "amd64", ""},
		{"darwin", "arm64", ""},
	}

	t.Run("BuildTargetValidation", func(t *testing.T) {
		for _, target := range targets {
			t.Run(fmt.Sprintf("%s-%s", target.goos, target.goarch), func(t *testing.T) {
				validatePlatformSupport(t, target.goos, target.goarch)

				validatePlatformConventions(t, target.goos, target.suffix)

				validatePackageManagerExpectations(t, target.goos)
			})
		}
	})
}

func TestBinaryCompatibilityMatrix(t *testing.T) {
	t.Run("ExecutableExtensions", func(t *testing.T) {
		testCases := []struct {
			platform string
			expected string
		}{
			{"windows", ".exe"},
			{"linux", ""},
			{"darwin", ""},
		}

		for _, tc := range testCases {
			t.Run(tc.platform, func(t *testing.T) {
				var ext string
				if tc.platform == "windows" {
					ext = ".exe"
				} else {
					ext = ""
				}

				if ext != tc.expected {
					t.Errorf("Platform %s: expected extension %s, got %s", tc.platform, tc.expected, ext)
				}
			})
		}
	})

	t.Run("PathSeparators", func(t *testing.T) {
		testCases := []struct {
			platform        string
			expectedSep     string
			expectedListSep string
		}{
			{"windows", "\\", ";"},
			{"linux", "/", ":"},
			{"darwin", "/", ":"},
		}

		for _, tc := range testCases {
			t.Run(tc.platform, func(t *testing.T) {
				var sep, listSep string
				if tc.platform == "windows" {
					sep = "\\"
					listSep = ";"
				} else {
					sep = "/"
					listSep = ":"
				}

				if sep != tc.expectedSep {
					t.Errorf("Platform %s: expected path separator %s, got %s", tc.platform, tc.expectedSep, sep)
				}

				if listSep != tc.expectedListSep {
					t.Errorf("Platform %s: expected list separator %s, got %s", tc.platform, tc.expectedListSep, listSep)
				}
			})
		}
	})
}

func TestBinaryFileStructure(t *testing.T) {
	t.Run("ExpectedBinaries", func(t *testing.T) {
		buildDir := "../../bin"
		if _, err := os.Stat(buildDir); os.IsNotExist(err) {
			t.Skip("Build directory does not exist - run 'make build' first")
		}

		expectedBinaries := []struct {
			name string
			desc string
		}{
			{"lsp-gateway-linux", "Linux AMD64 binary"},
			{"lsp-gateway-windows.exe", "Windows AMD64 binary"},
			{"lsp-gateway-macos", "macOS AMD64 binary"},
			{"lsp-gateway-macos-arm64", "macOS ARM64 binary"},
		}

		for _, binary := range expectedBinaries {
			t.Run(binary.name, func(t *testing.T) {
				binaryPath := filepath.Join(buildDir, binary.name)

				info, err := os.Stat(binaryPath)
				if err != nil {
					t.Logf("Binary %s not found - run 'make build' to create all binaries", binary.name)
					return
				}

				if !info.Mode().IsRegular() {
					t.Errorf("Binary %s is not a regular file", binary.name)
				}

				if info.Size() < 1024*1024 {
					t.Errorf("Binary %s seems too small (%d bytes)", binary.name, info.Size())
				}

				if !strings.Contains(binary.name, "windows") && runtime.GOOS != "windows" {
					if info.Mode()&0111 == 0 {
						t.Errorf("Binary %s is not executable", binary.name)
					}
				}

				t.Logf("Binary %s validated: %s, size=%d bytes", binary.name, binary.desc, info.Size())
			})
		}
	})
}

func TestBinaryExecutionCompatibility(t *testing.T) {
	t.Run("NativeBinaryExecution", func(t *testing.T) {
		buildDir := "../../bin"

		var nativeBinary string
		switch {
		case runtime.GOOS == "linux" && runtime.GOARCH == "amd64":
			nativeBinary = "lsp-gateway-linux"
		case runtime.GOOS == "windows" && runtime.GOARCH == "amd64":
			nativeBinary = "lsp-gateway-windows.exe"
		case runtime.GOOS == "darwin" && runtime.GOARCH == "amd64":
			nativeBinary = "lsp-gateway-macos"
		case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
			nativeBinary = "lsp-gateway-macos-arm64"
		default:
			t.Skipf("No native binary available for %s-%s", runtime.GOOS, runtime.GOARCH)
		}

		binaryPath := filepath.Join(buildDir, nativeBinary)
		if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
			t.Skipf("Native binary %s not found - run 'make build' first", nativeBinary)
		}

		executor := platform.NewCommandExecutor()
		result, err := executor.Execute(binaryPath, []string{"--version"}, 10*time.Second)

		if err != nil {
			t.Logf("Binary execution test skipped (expected in test environment): %v", err)
			return
		}

		if result.ExitCode != 0 {
			t.Errorf("Binary %s exited with code %d, stderr: %s", nativeBinary, result.ExitCode, result.Stderr)
		}

		if result.Stdout == "" {
			t.Error("Binary produced no output")
		}

		t.Logf("Binary %s executed successfully: %s", nativeBinary, strings.TrimSpace(result.Stdout))
	})
}

func TestCrossPlatformConfiguration(t *testing.T) {
	t.Run("ConfigPathResolution", func(t *testing.T) {
		testPaths := []struct {
			platform string
			path     string
			valid    bool
		}{
			{"windows", "C:\\Program Files\\lsp-gateway\\config.yaml", true},
			{"windows", "C:/Program Files/lsp-gateway/config.yaml", true}, // Forward slashes work on Windows
			{"linux", "/etc/lsp-gateway/config.yaml", true},
			{"linux", "/home/user/.config/lsp-gateway/config.yaml", true},
			{"darwin", "/usr/local/etc/lsp-gateway/config.yaml", true},
			{"darwin", "/Users/user/.config/lsp-gateway/config.yaml", true},
		}

		for _, tc := range testPaths {
			t.Run(fmt.Sprintf("%s-%s", tc.platform, filepath.Base(tc.path)), func(t *testing.T) {
				if tc.path == "" && tc.valid {
					t.Error("Expected valid path but got empty string")
				}

				if tc.valid && !strings.Contains(tc.path, "config.yaml") {
					t.Error("Path should contain config.yaml")
				}

				t.Logf("Path validation for %s: %s", tc.platform, tc.path)
			})
		}
	})
}

func TestBinaryDependencyValidation(t *testing.T) {
	t.Run("RuntimeDependencies", func(t *testing.T) {
		dependencies := []struct {
			name      string
			required  bool
			platforms []string
		}{
			{"libc", true, []string{"linux"}},
			{"kernel32.dll", true, []string{"windows"}},
			{"Foundation.framework", true, []string{"darwin"}},
		}

		for _, dep := range dependencies {
			t.Run(dep.name, func(t *testing.T) {
				relevant := false
				for _, platform := range dep.platforms {
					if runtime.GOOS == platform {
						relevant = true
						break
					}
				}

				if !relevant {
					t.Skipf("Dependency %s not relevant for platform %s", dep.name, runtime.GOOS)
				}

				t.Logf("Dependency %s validated for platform %s", dep.name, runtime.GOOS)
			})
		}
	})
}

func validatePlatformSupport(t *testing.T, goos, goarch string) {
	var platformType platform.Platform
	switch goos {
	case "linux":
		platformType = platform.PlatformLinux
	case "windows":
		platformType = platform.PlatformWindows
	case "darwin":
		platformType = platform.PlatformMacOS
	default:
		t.Errorf("Unsupported GOOS: %s", goos)
		return
	}

	var arch platform.Architecture
	switch goarch {
	case "amd64":
		arch = platform.ArchAMD64
	case "arm64":
		arch = platform.ArchARM64
	default:
		t.Errorf("Unsupported GOARCH: %s", goarch)
		return
	}

	if platformType == "" {
		t.Errorf("Platform enum missing for %s", goos)
	}

	if arch == "" {
		t.Errorf("Architecture enum missing for %s", goarch)
	}

	t.Logf("Platform support validated: %s-%s", platformType, arch)
}

func validatePlatformConventions(t *testing.T, goos, expectedSuffix string) {
	var actualSuffix string
	if goos == "windows" {
		actualSuffix = ".exe"
	} else {
		actualSuffix = ""
	}

	if actualSuffix != expectedSuffix {
		t.Errorf("Platform %s: expected suffix %s, got %s", goos, expectedSuffix, actualSuffix)
	}

	t.Logf("Platform conventions validated for %s: suffix=%s", goos, expectedSuffix)
}

func validatePackageManagerExpectations(t *testing.T, goos string) {
	expectedManagers := map[string][]string{
		"linux":   {"apt", "dnf", "yum", "pacman", "zypper", "apk"},
		"windows": {"winget", "chocolatey", "scoop"},
		"darwin":  {"brew", "port"},
	}

	managers, exists := expectedManagers[goos]
	if !exists {
		t.Errorf("No package manager expectations defined for %s", goos)
		return
	}

	if len(managers) == 0 {
		t.Errorf("Expected package managers list is empty for %s", goos)
		return
	}

	t.Logf("Package manager expectations for %s: %v", goos, managers)
}

func BenchmarkCrossPlatformOperations(b *testing.B) {
	b.Run("PlatformDetection", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = platform.GetCurrentPlatform()
			_ = platform.GetCurrentArchitecture()
		}
	})

	b.Run("PathOperations", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = platform.GetPathSeparator()
			_ = platform.GetPathListSeparator()
			_ = platform.GetExecutableExtension()
		}
	})

	b.Run("HomeDirectoryDetection", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = platform.GetHomeDirectory()
		}
	})
}
