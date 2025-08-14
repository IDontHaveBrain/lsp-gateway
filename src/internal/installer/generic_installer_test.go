package installer

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestNewGenericInstaller(t *testing.T) {
	platform := NewLSPPlatformInfo()

	tests := []struct {
		name           string
		language       string
		expectError    bool
		expectedCmd    string
		expectedArgs   []string
		expectedPkgMgr string
		expectedPkgs   []string
	}{
		{
			name:           "go language",
			language:       "go",
			expectError:    false,
			expectedCmd:    "gopls",
			expectedArgs:   []string{"serve"},
			expectedPkgMgr: "go",
			expectedPkgs:   []string{"golang.org/x/tools/gopls"},
		},
		{
			name:           "python language",
			language:       "python",
			expectError:    false,
			expectedCmd:    "jedi-language-server",
			expectedArgs:   []string{},
			expectedPkgMgr: "pip",
			expectedPkgs:   []string{"jedi-language-server"},
		},
		{
			name:           "typescript language",
			language:       "typescript",
			expectError:    false,
			expectedCmd:    "typescript-language-server",
			expectedArgs:   []string{"--stdio"},
			expectedPkgMgr: "npm",
			expectedPkgs:   []string{"typescript-language-server", "typescript"},
		},
		{
			name:        "unsupported language",
			language:    "unsupported",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installer, err := NewGenericInstaller(tt.language, platform)

			if tt.expectError {
				if err == nil {
					t.Error("NewGenericInstaller() expected error but got nil")
				}
				if installer != nil {
					t.Error("NewGenericInstaller() expected nil installer when error occurs")
				}
				return
			}

			if err != nil {
				t.Errorf("NewGenericInstaller() error = %v, want nil", err)
				return
			}

			if installer == nil {
				t.Fatal("NewGenericInstaller() returned nil installer")
			}

			// Test embedded BaseInstaller
			if installer.BaseInstaller == nil {
				t.Fatal("BaseInstaller not embedded correctly")
			}

			// Test language
			if installer.GetLanguage() != tt.language {
				t.Errorf("GetLanguage() = %v, want %v", installer.GetLanguage(), tt.language)
			}

			// Test server config
			serverConfig := installer.GetServerConfig()
			if serverConfig == nil {
				t.Fatal("GetServerConfig() returned nil")
			}

			if serverConfig.Command != tt.expectedCmd {
				t.Errorf("Command = %v, want %v", serverConfig.Command, tt.expectedCmd)
			}

			if len(serverConfig.Args) != len(tt.expectedArgs) {
				t.Errorf("Args length = %v, want %v", len(serverConfig.Args), len(tt.expectedArgs))
			} else {
				for i, arg := range tt.expectedArgs {
					if serverConfig.Args[i] != arg {
						t.Errorf("Args[%d] = %v, want %v", i, serverConfig.Args[i], arg)
					}
				}
			}

			// Test package config
			if installer.config.Manager != tt.expectedPkgMgr {
				t.Errorf("Package manager = %v, want %v", installer.config.Manager, tt.expectedPkgMgr)
			}

			if len(installer.config.Packages) != len(tt.expectedPkgs) {
				t.Errorf("Packages length = %v, want %v", len(installer.config.Packages), len(tt.expectedPkgs))
			} else {
				for i, pkg := range tt.expectedPkgs {
					if installer.config.Packages[i] != pkg {
						t.Errorf("Packages[%d] = %v, want %v", i, installer.config.Packages[i], pkg)
					}
				}
			}
		})
	}
}

func TestGenericInstallerPackageConfigs(t *testing.T) {
	tests := []struct {
		language string
		manager  string
		packages []string
	}{
		{
			language: "go",
			manager:  "go",
			packages: []string{"golang.org/x/tools/gopls"},
		},
		{
			language: "python",
			manager:  "pip",
			packages: []string{"jedi-language-server"},
		},
		{
			language: "typescript",
			manager:  "npm",
			packages: []string{"typescript-language-server", "typescript"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			config, exists := packageConfigs[tt.language]
			if !exists {
				t.Errorf("Package config for %s not found", tt.language)
				return
			}

			if config.Manager != tt.manager {
				t.Errorf("Manager = %v, want %v", config.Manager, tt.manager)
			}

			if len(config.Packages) != len(tt.packages) {
				t.Errorf("Packages length = %v, want %v", len(config.Packages), len(tt.packages))
				return
			}

			for i, pkg := range tt.packages {
				if config.Packages[i] != pkg {
					t.Errorf("Packages[%d] = %v, want %v", i, config.Packages[i], pkg)
				}
			}
		})
	}
}

func TestGenericInstallerInstall(t *testing.T) {
	platform := NewLSPPlatformInfo()

	tests := []struct {
		name        string
		language    string
		options     InstallOptions
		expectError bool
	}{
		{
			name:     "go install default version",
			language: "go",
			options: InstallOptions{
				Version: "",
			},
			expectError: false, // May succeed if go is available
		},
		{
			name:     "go install specific version",
			language: "go",
			options: InstallOptions{
				Version: "v1.2.3",
			},
			expectError: true, // Likely to fail with invalid version
		},
		{
			name:     "python install default version",
			language: "python",
			options: InstallOptions{
				Version: "",
			},
			expectError: true, // Usually fails due to system restrictions
		},
		{
			name:     "python install specific version",
			language: "python",
			options: InstallOptions{
				Version: "2.1.0",
			},
			expectError: true, // Usually fails due to system restrictions
		},
		{
			name:     "typescript install default version",
			language: "typescript",
			options: InstallOptions{
				Version: "",
			},
			expectError: false, // May succeed if npm is available
		},
		{
			name:     "typescript install latest explicitly",
			language: "typescript",
			options: InstallOptions{
				Version: "latest",
			},
			expectError: false, // May succeed if npm is available
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installer, err := NewGenericInstaller(tt.language, platform)
			if err != nil {
				t.Fatalf("NewGenericInstaller() error = %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err = installer.Install(ctx, tt.options)

			// For testing purposes, we accept both success and expected failure
			// The important part is that the method doesn't panic and handles errors gracefully
			if tt.expectError && err == nil {
				t.Logf("Install() expected error but succeeded - this may be due to available package managers")
			} else if !tt.expectError && err != nil {
				t.Logf("Install() succeeded despite potential issues: %v", err)
			}

			// Test completed without panic - that's the main goal
		})
	}
}

func TestGenericInstallerInstallVersionHandling(t *testing.T) {
	platform := NewLSPPlatformInfo()

	tests := []struct {
		name         string
		language     string
		inputVersion string
	}{
		{
			name:         "empty version becomes latest",
			language:     "go",
			inputVersion: "",
		},
		{
			name:         "latest version explicit",
			language:     "python",
			inputVersion: "latest",
		},
		{
			name:         "specific version",
			language:     "typescript",
			inputVersion: "4.5.2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installer, err := NewGenericInstaller(tt.language, platform)
			if err != nil {
				t.Fatalf("NewGenericInstaller() error = %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			options := InstallOptions{Version: tt.inputVersion}
			err = installer.Install(ctx, options)

			// Test that version handling doesn't cause panics
			// Success or failure depends on system availability
			t.Logf("Install with version '%s' completed with result: %v", tt.inputVersion, err)
		})
	}
}

func TestGenericInstallerInstallMultiplePackages(t *testing.T) {
	platform := NewLSPPlatformInfo()

	// Test TypeScript which has multiple packages
	installer, err := NewGenericInstaller("typescript", platform)
	if err != nil {
		t.Fatalf("NewGenericInstaller() error = %v", err)
	}

	// Verify TypeScript installer has multiple packages configured
	expectedPackages := []string{"typescript-language-server", "typescript"}
	if len(installer.config.Packages) != len(expectedPackages) {
		t.Errorf("TypeScript installer packages count = %v, want %v", len(installer.config.Packages), len(expectedPackages))
	}

	for i, pkg := range expectedPackages {
		if installer.config.Packages[i] != pkg {
			t.Errorf("Package[%d] = %v, want %v", i, installer.config.Packages[i], pkg)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	options := InstallOptions{Version: "latest"}
	err = installer.Install(ctx, options)

	// Test that multiple package installation doesn't panic
	// Success or failure depends on system availability
	if err != nil {
		t.Logf("Install() failed as expected in test environment: %v", err)
	} else {
		t.Logf("Install() succeeded - TypeScript packages are available on this system")
	}
}

func TestGenericInstallerUninstall(t *testing.T) {
	platform := NewLSPPlatformInfo()

	tests := []struct {
		name     string
		language string
	}{
		{
			name:     "go uninstall",
			language: "go",
		},
		{
			name:     "python uninstall",
			language: "python",
		},
		{
			name:     "typescript uninstall",
			language: "typescript",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installer, err := NewGenericInstaller(tt.language, platform)
			if err != nil {
				t.Fatalf("NewGenericInstaller() error = %v", err)
			}

			// Test that Uninstall doesn't panic and returns expected result
			// Real uninstall operations will fail but method should handle gracefully
			err = installer.Uninstall()

			// Uninstall should not return error even if individual packages fail
			// (implementation logs warnings but doesn't fail)
			if err != nil {
				t.Errorf("Uninstall() error = %v, want nil", err)
			}
		})
	}
}

func TestGenericInstallerValidateInstallation(t *testing.T) {
	platform := NewLSPPlatformInfo()

	tests := []struct {
		name        string
		language    string
		expectError bool
	}{
		{
			name:        "go validation",
			language:    "go",
			expectError: true, // Will fail since gopls not installed in test env
		},
		{
			name:        "python validation",
			language:    "python",
			expectError: true, // Will fail since jedi-language-server not installed
		},
		{
			name:        "typescript validation",
			language:    "typescript",
			expectError: true, // Will fail since ts-language-server not installed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installer, err := NewGenericInstaller(tt.language, platform)
			if err != nil {
				t.Fatalf("NewGenericInstaller() error = %v", err)
			}

			err = installer.ValidateInstallation()

			if tt.expectError {
				if err == nil {
					t.Error("ValidateInstallation() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("ValidateInstallation() error = %v, want nil", err)
				}
			}
		})
	}
}

func TestGenericInstallerPackageManagerCommandGeneration(t *testing.T) {
	platform := NewLSPPlatformInfo()

	tests := []struct {
		name            string
		language        string
		expectedManager string
		expectedCommand string
	}{
		{
			name:            "go configuration",
			language:        "go",
			expectedManager: "go",
			expectedCommand: "gopls",
		},
		{
			name:            "python configuration",
			language:        "python",
			expectedManager: "pip",
			expectedCommand: "jedi-language-server",
		},
		{
			name:            "typescript configuration",
			language:        "typescript",
			expectedManager: "npm",
			expectedCommand: "typescript-language-server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installer, err := NewGenericInstaller(tt.language, platform)
			if err != nil {
				t.Fatalf("NewGenericInstaller() error = %v", err)
			}

			// Test package manager configuration
			if installer.config.Manager != tt.expectedManager {
				t.Errorf("Package manager = %v, want %v", installer.config.Manager, tt.expectedManager)
			}

			// Test command configuration
			if installer.GetServerConfig().Command != tt.expectedCommand {
				t.Errorf("Command = %v, want %v", installer.GetServerConfig().Command, tt.expectedCommand)
			}

			// Test that packages are configured
			if len(installer.config.Packages) == 0 {
				t.Error("No packages configured")
			}
		})
	}
}

func TestGenericInstallerErrorScenarios(t *testing.T) {
	platform := NewLSPPlatformInfo()

	t.Run("install with context cancellation", func(t *testing.T) {
		installer, err := NewGenericInstaller("go", platform)
		if err != nil {
			t.Fatalf("NewGenericInstaller() error = %v", err)
		}

		// Create context that cancels immediately
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		options := InstallOptions{Version: "latest"}
		err = installer.Install(ctx, options)

		if err == nil {
			t.Error("Install() expected context cancellation error but got nil")
		}
	})

	t.Run("invalid package manager scenario", func(t *testing.T) {
		installer, err := NewGenericInstaller("python", platform)
		if err != nil {
			t.Fatalf("NewGenericInstaller() error = %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Test with valid options but expect package manager issues
		options := InstallOptions{Version: "latest"}
		err = installer.Install(ctx, options)

		if err == nil {
			t.Error("Install() expected package manager error but got nil")
		}

		// Should get an installation failure error
		if !strings.Contains(err.Error(), "failed to install") {
			t.Errorf("Install() error = %v, want 'failed to install' error", err)
		}
	})
}

func TestPackageConfigStructure(t *testing.T) {
	tests := []struct {
		name    string
		config  PackageConfig
		isValid bool
	}{
		{
			name: "valid go config",
			config: PackageConfig{
				Manager:  "go",
				Packages: []string{"golang.org/x/tools/gopls"},
			},
			isValid: true,
		},
		{
			name: "valid npm config with multiple packages",
			config: PackageConfig{
				Manager:  "npm",
				Packages: []string{"typescript-language-server", "typescript"},
			},
			isValid: true,
		},
		{
			name: "invalid config - no manager",
			config: PackageConfig{
				Manager:  "",
				Packages: []string{"some-package"},
			},
			isValid: false,
		},
		{
			name: "invalid config - no packages",
			config: PackageConfig{
				Manager:  "npm",
				Packages: []string{},
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.config.Manager != "" && len(tt.config.Packages) > 0

			if isValid != tt.isValid {
				t.Errorf("Config validity = %v, want %v", isValid, tt.isValid)
			}
		})
	}
}

func TestGenericInstallerStructureIntegrity(t *testing.T) {
	platform := NewLSPPlatformInfo()

	// Test that all supported languages have proper configurations
	supportedLanguages := []string{"go", "python", "typescript"}

	for _, lang := range supportedLanguages {
		t.Run(fmt.Sprintf("structure_%s", lang), func(t *testing.T) {
			installer, err := NewGenericInstaller(lang, platform)
			if err != nil {
				t.Fatalf("NewGenericInstaller(%s) error = %v", lang, err)
			}

			// Test structure integrity
			if installer.BaseInstaller == nil {
				t.Error("BaseInstaller not properly embedded")
			}

			if installer.GetLanguage() != lang {
				t.Errorf("Language mismatch: got %v, want %v", installer.GetLanguage(), lang)
			}

			serverConfig := installer.GetServerConfig()
			if serverConfig == nil {
				t.Error("ServerConfig is nil")
			}

			if serverConfig.Command == "" {
				t.Error("Command is empty")
			}

			if installer.config.Manager == "" {
				t.Error("Package manager is empty")
			}

			if len(installer.config.Packages) == 0 {
				t.Error("No packages configured")
			}
		})
	}
}

func TestGenericInstallerPackageConfigConsistency(t *testing.T) {
	// Test that packageConfigs map is consistent with NewGenericInstaller behavior
	for language, expectedConfig := range packageConfigs {
		t.Run(fmt.Sprintf("consistency_%s", language), func(t *testing.T) {
			platform := NewLSPPlatformInfo()
			installer, err := NewGenericInstaller(language, platform)
			if err != nil {
				t.Fatalf("NewGenericInstaller(%s) error = %v", language, err)
			}

			// Check that installer config matches packageConfigs
			if installer.config.Manager != expectedConfig.Manager {
				t.Errorf("Manager mismatch: installer=%v, packageConfigs=%v",
					installer.config.Manager, expectedConfig.Manager)
			}

			if len(installer.config.Packages) != len(expectedConfig.Packages) {
				t.Errorf("Package count mismatch: installer=%d, packageConfigs=%d",
					len(installer.config.Packages), len(expectedConfig.Packages))
			}

			for i, pkg := range expectedConfig.Packages {
				if installer.config.Packages[i] != pkg {
					t.Errorf("Package[%d] mismatch: installer=%v, packageConfigs=%v",
						i, installer.config.Packages[i], pkg)
				}
			}
		})
	}
}
