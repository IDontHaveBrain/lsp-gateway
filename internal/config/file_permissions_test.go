package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestConfigFileReadPermissionDenied tests various configuration file read permission scenarios
func TestConfigFileReadPermissionDenied(t *testing.T) {
	t.Parallel()

	skipIfRoot(t)

	t.Run("config_file_no_read_permission", func(t *testing.T) {
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "config.yaml")
		
		// Create file with valid content but no read permissions (write-only)
		configContent := "port: 8080\nservers:\n  - name: go-lsp\n    languages: [go]\n    command: gopls\n    transport: stdio"
		err := os.WriteFile(configFile, []byte(configContent), 0200)
		if err != nil {
			t.Fatalf("Failed to create test config file: %v", err)
		}

		_, err = LoadConfig(configFile)
		assertPermissionError(t, err, "read")
	})

	t.Run("config_file_no_permissions", func(t *testing.T) {
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "config.yaml")
		
		// Create file with no permissions at all
		configContent := "port: 8080\nservers: []"
		err := os.WriteFile(configFile, []byte(configContent), 0000)
		if err != nil {
			t.Fatalf("Failed to create test config file: %v", err)
		}

		_, err = LoadConfig(configFile)
		assertPermissionError(t, err, "access")
	})

	t.Run("config_file_execute_only_permission", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Execute-only permissions not meaningful on Windows")
		}
		
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "config.yaml")
		
		// Create file with execute-only permissions
		configContent := "port: 8080\nservers: []"
		err := os.WriteFile(configFile, []byte(configContent), 0100)
		if err != nil {
			t.Fatalf("Failed to create test config file: %v", err)
		}

		_, err = LoadConfig(configFile)
		assertPermissionError(t, err, "permission")
	})
}

// TestConfigDirectoryPermissionDenied tests directory-level permission issues
func TestConfigDirectoryPermissionDenied(t *testing.T) {
	t.Parallel()

	skipIfRoot(t)

	t.Run("parent_directory_no_read_permission", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Directory permission testing complex on Windows")
		}
		
		tmpDir := t.TempDir()
		restrictedDir := filepath.Join(tmpDir, "restricted")
		configFile := filepath.Join(restrictedDir, "config.yaml")
		
		// Create directory and file first
		err := os.MkdirAll(restrictedDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		
		configContent := "port: 8080\nservers: []"
		err = os.WriteFile(configFile, []byte(configContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create config file: %v", err)
		}
		
		// Remove read permission from directory (keep execute for access)
		err = os.Chmod(restrictedDir, 0100) // execute only
		if err != nil {
			t.Fatalf("Failed to change directory permissions: %v", err)
		}
		
		// Ensure cleanup can access directory
		t.Cleanup(func() {
			_ = os.Chmod(restrictedDir, 0755)
		})

		_, err = LoadConfig(configFile)
		// This might succeed on some systems where execute is sufficient
		// so we'll log the result rather than assert failure
		if err != nil {
			t.Logf("Expected behavior: access denied to restricted directory: %v", err)
		} else {
			t.Log("Config loaded despite directory read restrictions (system dependent)")
		}
	})

	t.Run("parent_directory_no_execute_permission", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Directory execute permission testing not applicable on Windows")
		}
		
		tmpDir := t.TempDir()
		restrictedDir := filepath.Join(tmpDir, "noexec")
		configFile := filepath.Join(restrictedDir, "config.yaml")
		
		// Create directory and file first
		err := os.MkdirAll(restrictedDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		
		configContent := "port: 8080\nservers: []"
		err = os.WriteFile(configFile, []byte(configContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create config file: %v", err)
		}
		
		// Remove execute permission from directory (read+write only)
		err = os.Chmod(restrictedDir, 0600)
		if err != nil {
			t.Fatalf("Failed to change directory permissions: %v", err)
		}
		
		// Ensure cleanup can access directory
		t.Cleanup(func() {
			_ = os.Chmod(restrictedDir, 0755)
		})

		_, err = LoadConfig(configFile)
		assertPermissionError(t, err, "permission")
	})
}

// TestConfigFileWritePermissionScenarios tests scenarios where config might need to be written/modified
func TestConfigFileWritePermissionScenarios(t *testing.T) {
	t.Parallel()

	skipIfRoot(t)

	t.Run("readonly_file_system_simulation", func(t *testing.T) {
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "readonly_config.yaml")
		
		// Create a valid config file
		configContent := "port: 8080\nservers:\n  - name: go-lsp\n    languages: [go]\n    command: gopls\n    transport: stdio"
		err := os.WriteFile(configFile, []byte(configContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create config file: %v", err)
		}
		
		// Make file read-only
		err = os.Chmod(configFile, 0444)
		if err != nil {
			t.Fatalf("Failed to make file read-only: %v", err)
		}

		// Loading should succeed for read-only file
		config, err := LoadConfig(configFile)
		if err != nil {
			t.Errorf("Loading read-only config should succeed: %v", err)
		}
		if config == nil {
			t.Error("Expected valid config from read-only file")
		}
		
		// But if we tried to write to it, it should fail (simulated)
		err = os.WriteFile(configFile, []byte("modified content"), 0644)
		if err == nil {
			t.Error("Expected error when trying to write to read-only file")
		}
	})

	t.Run("config_directory_readonly", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Directory write permission testing complex on Windows")
		}
		
		tmpDir := t.TempDir()
		configDir := filepath.Join(tmpDir, "readonly_dir")
		configFile := filepath.Join(configDir, "config.yaml")
		
		// Create directory and config file
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create config directory: %v", err)
		}
		
		configContent := "port: 8080\nservers: []"
		err = os.WriteFile(configFile, []byte(configContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create config file: %v", err)
		}
		
		// Make directory read-only (remove write permission)
		err = os.Chmod(configDir, 0555) // read+execute only
		if err != nil {
			t.Fatalf("Failed to make directory read-only: %v", err)
		}
		
		// Ensure cleanup can access directory
		t.Cleanup(func() {
			_ = os.Chmod(configDir, 0755)
		})

		// Reading existing file should work
		config, err := LoadConfig(configFile)
		if err != nil {
			t.Errorf("Reading from read-only directory should work: %v", err)
		}
		if config == nil {
			t.Error("Expected valid config from read-only directory")
		}
		
		// But creating new files should fail
		newConfigFile := filepath.Join(configDir, "new_config.yaml")
		err = os.WriteFile(newConfigFile, []byte(configContent), 0644)
		if err == nil {
			t.Error("Expected error when creating file in read-only directory")
		}
	})
}

// TestConfigPermissionRecovery tests recovery scenarios after permission issues are resolved
func TestConfigPermissionRecovery(t *testing.T) {
	t.Parallel()

	skipIfRoot(t)

	t.Run("permission_recovery_after_fix", func(t *testing.T) {
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "recovery_config.yaml")
		
		// Create file with valid content but no read permissions
		configContent := "port: 8080\nservers:\n  - name: go-lsp\n    languages: [go]\n    command: gopls\n    transport: stdio"
		err := os.WriteFile(configFile, []byte(configContent), 0200)
		if err != nil {
			t.Fatalf("Failed to create test config file: %v", err)
		}

		// First attempt should fail
		_, err = LoadConfig(configFile)
		if err == nil {
			t.Error("Expected error for unreadable config file")
		}

		// Fix permissions
		err = os.Chmod(configFile, 0644)
		if err != nil {
			t.Fatalf("Failed to fix file permissions: %v", err)
		}

		// Second attempt should succeed
		config, err := LoadConfig(configFile)
		if err != nil {
			t.Errorf("Expected recovery after fixing permissions: %v", err)
		}
		if config == nil {
			t.Error("Expected valid config after permission recovery")
		}
		if config.Port != 8080 {
			t.Errorf("Expected port 8080, got %d", config.Port)
		}
	})

	t.Run("time_of_check_vs_time_of_use", func(t *testing.T) {
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "toctou_config.yaml")
		
		// Create file with valid permissions
		configContent := "port: 8080\nservers: []"
		err := os.WriteFile(configFile, []byte(configContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create config file: %v", err)
		}

		// Simulate TOCTOU by changing permissions between stat and read
		// First, verify file exists and is readable
		_, err = os.Stat(configFile)
		if err != nil {
			t.Fatalf("File should exist: %v", err)
		}

		// Change permissions to make it unreadable
		err = os.Chmod(configFile, 0000)
		if err != nil {
			t.Fatalf("Failed to change permissions: %v", err)
		}

		// Now try to load it - should handle permission change gracefully
		_, err = LoadConfig(configFile)
		assertPermissionError(t, err, "permission")

		// Restore permissions for cleanup
		_ = os.Chmod(configFile, 0644)
	})
}

// TestConfigCrossplatformPermissions tests permission handling across different platforms
func TestConfigCrossplatformPermissions(t *testing.T) {
	t.Parallel()

	t.Run("windows_file_attributes", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("Windows-specific test")
		}
		
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "windows_config.yaml")
		
		// Create config file
		configContent := "port: 8080\nservers: []"
		err := os.WriteFile(configFile, []byte(configContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create config file: %v", err)
		}

		// On Windows, test with read-only attribute
		err = os.Chmod(configFile, 0444)
		if err != nil {
			t.Fatalf("Failed to set read-only: %v", err)
		}

		// Should still be able to read
		config, err := LoadConfig(configFile)
		if err != nil {
			t.Errorf("Should be able to read read-only file on Windows: %v", err)
		}
		if config == nil {
			t.Error("Expected valid config on Windows")
		}
	})

	t.Run("unix_permission_combinations", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Unix-specific test")
		}
		
		skipIfRoot(t)
		
		tmpDir := t.TempDir()
		
		permissionTests := []struct {
			name        string
			mode        os.FileMode
			shouldRead  bool
			description string
		}{
			{"owner_read_only", 0400, true, "Owner read permission"},
			{"owner_write_only", 0200, false, "Owner write permission only"},
			{"owner_execute_only", 0100, false, "Owner execute permission only"},
			{"group_read_only", 0040, false, "Group read permission only"},
			{"world_read_only", 0004, false, "World read permission only"},
			{"owner_read_write", 0600, true, "Owner read+write permissions"},
			{"all_read", 0444, true, "All read permissions"},
			{"no_permissions", 0000, false, "No permissions"},
		}

		for _, tt := range permissionTests {
			t.Run(tt.name, func(t *testing.T) {
				configFile := filepath.Join(tmpDir, fmt.Sprintf("config_%s.yaml", tt.name))
				
				configContent := "port: 8080\nservers: []"
				err := os.WriteFile(configFile, []byte(configContent), 0644)
				if err != nil {
					t.Fatalf("Failed to create config file: %v", err)
				}

				err = os.Chmod(configFile, tt.mode)
				if err != nil {
					t.Fatalf("Failed to set permissions %o: %v", tt.mode, err)
				}

				config, err := LoadConfig(configFile)
				
				if tt.shouldRead {
					if err != nil {
						t.Errorf("Expected to read file with %s (%o): %v", tt.description, tt.mode, err)
					}
					if config == nil {
						t.Error("Expected valid config")
					}
				} else {
					if err == nil {
						t.Errorf("Expected error reading file with %s (%o)", tt.description, tt.mode)
					}
				}
			})
		}
	})
}

// TestConfigSecurityPermissions tests security-related permission scenarios
func TestConfigSecurityPermissions(t *testing.T) {
	t.Parallel()

	skipIfRoot(t)

	t.Run("world_writable_config_warning", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("World-writable concept different on Windows")
		}
		
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "world_writable_config.yaml")
		
		// Create config file with world-writable permissions
		configContent := "port: 8080\nservers: []"
		file, err := os.OpenFile(configFile, os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			t.Fatalf("Failed to create config file: %v", err)
		}
		_, err = file.WriteString(configContent)
		file.Close()
		if err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		// Explicitly set world-writable permissions (umask might have modified them)
		err = os.Chmod(configFile, 0666)
		if err != nil {
			t.Fatalf("Failed to set world-writable permissions: %v", err)
		}

		// Should still load but potentially log warning
		config, err := LoadConfig(configFile)
		if err != nil {
			t.Errorf("Should load world-writable config: %v", err)
		}
		if config == nil {
			t.Error("Expected valid config")
		}

		// Check file permissions
		info, err := os.Stat(configFile)
		if err != nil {
			t.Fatalf("Failed to stat file: %v", err)
		}
		
		mode := info.Mode()
		// Check if file has group or world write permissions (potential security issue)
		if mode&0022 == 0 {
			t.Log("File permissions were restricted by umask - this is actually good for security")
		} else {
			t.Logf("File has broad write permissions (%o) - potential security concern", mode)
		}
	})

	t.Run("secure_permissions_preference", func(t *testing.T) {
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "secure_config.yaml")
		
		// Create config file with secure permissions (600)
		configContent := "port: 8080\nservers: []"
		err := os.WriteFile(configFile, []byte(configContent), 0600)
		if err != nil {
			t.Fatalf("Failed to create config file: %v", err)
		}

		// Should load successfully
		config, err := LoadConfig(configFile)
		if err != nil {
			t.Errorf("Should load securely-permissioned config: %v", err)
		}
		if config == nil {
			t.Error("Expected valid config")
		}

		// Verify permissions are secure
		info, err := os.Stat(configFile)
		if err != nil {
			t.Fatalf("Failed to stat file: %v", err)
		}
		
		mode := info.Mode()
		if runtime.GOOS != "windows" {
			// On Unix, check that group and others have no permissions
			if mode&0077 != 0 {
				t.Error("Expected file to have secure permissions (600)")
			}
		}
	})
}

// TestConfigConcurrentPermissionAccess tests concurrent access with permission constraints
func TestConfigConcurrentPermissionAccess(t *testing.T) {
	t.Parallel()

	skipIfRoot(t)

	t.Run("concurrent_read_with_permission_changes", func(t *testing.T) {
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "concurrent_perm_config.yaml")
		
		configContent := "port: 8080\nservers: []"
		err := os.WriteFile(configFile, []byte(configContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create config file: %v", err)
		}

		const numGoroutines = 10
		results := make(chan error, numGoroutines)
		
		// Start multiple goroutines trying to read the config
		for i := 0; i < numGoroutines; i++ {
			go func(iteration int) {
				// Add small delay to create race conditions
				time.Sleep(time.Duration(iteration) * time.Millisecond)
				
				_, err := LoadConfig(configFile)
				results <- err
			}(i)
		}

		// Concurrently change permissions
		go func() {
			for i := 0; i < 5; i++ {
				time.Sleep(5 * time.Millisecond)
				_ = os.Chmod(configFile, 0600)
				time.Sleep(5 * time.Millisecond)
				_ = os.Chmod(configFile, 0644)
			}
		}()

		// Collect results
		successCount := 0
		errorCount := 0
		for i := 0; i < numGoroutines; i++ {
			err := <-results
			if err != nil {
				errorCount++
				t.Logf("Concurrent read error: %v", err)
			} else {
				successCount++
			}
		}

		// Should have at least some successes
		if successCount == 0 {
			t.Error("Expected at least some successful concurrent reads")
		}
		
		t.Logf("Concurrent permission test: %d successes, %d errors", successCount, errorCount)
	})
}

// Helper Functions

func skipIfRoot(t *testing.T) {
	t.Helper()
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}
}

func assertPermissionError(t *testing.T, err error, expectedSubstring string) {
	t.Helper()
	if err == nil {
		t.Error("Expected permission error but got none")
		return
	}
	
	errMsg := strings.ToLower(err.Error())
	expectedPatterns := []string{
		"permission denied",
		"access is denied",
		"access denied",
		"not permitted",
		"operation not permitted",
		expectedSubstring,
	}
	
	found := false
	for _, pattern := range expectedPatterns {
		if strings.Contains(errMsg, pattern) {
			found = true
			break
		}
	}
	
	if !found {
		t.Errorf("Expected permission error containing one of %v, got: %v", expectedPatterns, err)
	}
}

func createTestConfigWithPermissions(t *testing.T, dir string, filename string, content string, perm os.FileMode) string {
	t.Helper()
	
	configFile := filepath.Join(dir, filename)
	err := os.WriteFile(configFile, []byte(content), perm)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}
	
	return configFile
}

func verifyConfigLoadBehavior(t *testing.T, configFile string, expectSuccess bool, description string) {
	t.Helper()
	
	config, err := LoadConfig(configFile)
	
	if expectSuccess {
		if err != nil {
			t.Errorf("%s: expected success but got error: %v", description, err)
		}
		if config == nil {
			t.Errorf("%s: expected valid config but got nil", description)
		}
	} else {
		if err == nil {
			t.Errorf("%s: expected error but loading succeeded", description)
		}
	}
}