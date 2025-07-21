package platform

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestTempDirectoryPermissions tests temporary directory access permission scenarios
func TestTempDirectoryPermissions(t *testing.T) {
	t.Parallel()

	skipIfRoot(t)

	t.Run("system_temp_directory_access", func(t *testing.T) {
		tempDir := GetTempDirectory()
		if tempDir == "" {
			t.Fatal("GetTempDirectory returned empty string")
		}

		// Verify we can access the system temp directory
		info, err := os.Stat(tempDir)
		if err != nil {
			t.Fatalf("Cannot access system temp directory %s: %v", tempDir, err)
		}

		if !info.IsDir() {
			t.Errorf("System temp path %s is not a directory", tempDir)
		}

		// Test creating a file in temp directory
		testFile := filepath.Join(tempDir, "lsp-gateway-test-"+generateRandomString(8))
		defer os.Remove(testFile)

		err = os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			t.Errorf("Cannot create file in system temp directory %s: %v", tempDir, err)
		}
	})

	t.Run("restricted_temp_directory", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Directory permission testing complex on Windows")
		}

		// Create a custom temp directory with restricted permissions
		baseDir := t.TempDir()
		restrictedTemp := filepath.Join(baseDir, "restricted_temp")

		// Create directory without write permissions
		err := os.MkdirAll(restrictedTemp, 0555) // read+execute only
		if err != nil {
			t.Fatalf("Failed to create restricted temp directory: %v", err)
		}

		// Ensure cleanup can access directory
		t.Cleanup(func() {
			_ = os.Chmod(restrictedTemp, 0755)
		})

		// Attempt to create temporary file should fail
		tempFile := filepath.Join(restrictedTemp, "temp_file.tmp")
		err = os.WriteFile(tempFile, []byte("temp content"), 0644)
		if err == nil {
			t.Error("Expected error creating file in read-only temp directory")
		}
		assertPermissionError(t, err, "permission")
	})

	t.Run("temp_directory_no_execute_permission", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Directory execute permission not applicable on Windows")
		}

		baseDir := t.TempDir()
		noExecTemp := filepath.Join(baseDir, "no_exec_temp")

		// Create directory and file first
		err := os.MkdirAll(noExecTemp, 0755)
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}

		tempFile := filepath.Join(noExecTemp, "existing.tmp")
		err = os.WriteFile(tempFile, []byte("existing content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}

		// Remove execute permission from directory
		err = os.Chmod(noExecTemp, 0600) // read+write only, no execute
		if err != nil {
			t.Fatalf("Failed to change directory permissions: %v", err)
		}

		// Ensure cleanup can access directory
		t.Cleanup(func() {
			_ = os.Chmod(noExecTemp, 0755)
		})

		// Attempt to access file should fail
		_, err = os.Stat(tempFile)
		if err == nil {
			t.Error("Expected error accessing file in non-executable directory")
		}
		assertPermissionError(t, err, "permission")
	})
}

// TestTempFileCreationPermissions tests temporary file creation with various permission constraints
func TestTempFileCreationPermissions(t *testing.T) {
	t.Parallel()

	skipIfRoot(t)

	t.Run("create_temp_file_with_secure_permissions", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create temporary file with secure permissions (600)
		tempFile := filepath.Join(tempDir, "secure_temp.tmp")
		err := os.WriteFile(tempFile, []byte("secure temp content"), 0600)
		if err != nil {
			t.Fatalf("Failed to create secure temp file: %v", err)
		}

		// Verify permissions are secure
		info, err := os.Stat(tempFile)
		if err != nil {
			t.Fatalf("Failed to stat secure temp file: %v", err)
		}

		mode := info.Mode()
		if runtime.GOOS != "windows" {
			// On Unix, verify only owner has permissions
			if mode&0077 != 0 {
				t.Errorf("Expected secure permissions (600), got %o", mode)
			}
		}
	})

	t.Run("temp_file_permission_inheritance", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create temp file and check if it inherits umask
		tempFile := filepath.Join(tempDir, "inherit_temp.tmp")
		file, err := os.OpenFile(tempFile, os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		file.Close()

		// Check actual permissions
		info, err := os.Stat(tempFile)
		if err != nil {
			t.Fatalf("Failed to stat temp file: %v", err)
		}

		mode := info.Mode()
		t.Logf("Temp file permissions: %o", mode)

		// Permissions should be reasonable (not world-writable for security)
		if runtime.GOOS != "windows" && mode&0002 != 0 {
			t.Error("Temp file should not be world-writable for security")
		}
	})

	t.Run("temp_file_creation_race_condition", func(t *testing.T) {
		tempDir := t.TempDir()

		var wg sync.WaitGroup
		var mu sync.Mutex
		createdFiles := make([]string, 0, 10)
		errors := make([]error, 0, 10)

		// Try to create temp files concurrently
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				tempFile := filepath.Join(tempDir, fmt.Sprintf("race_temp_%d_%d.tmp", id, time.Now().UnixNano()))

				// Create with exclusive access
				file, err := os.OpenFile(tempFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
				if err != nil {
					mu.Lock()
					errors = append(errors, fmt.Errorf("goroutine %d: %w", id, err))
					mu.Unlock()
					return
				}

				_, writeErr := file.WriteString(fmt.Sprintf("Content from goroutine %d", id))
				file.Close()

				if writeErr != nil {
					mu.Lock()
					errors = append(errors, fmt.Errorf("goroutine %d write: %w", id, writeErr))
					mu.Unlock()
					return
				}

				mu.Lock()
				createdFiles = append(createdFiles, tempFile)
				mu.Unlock()
			}(i)
		}

		wg.Wait()

		t.Logf("Created %d temp files, %d errors", len(createdFiles), len(errors))

		if len(errors) > 0 {
			for i, err := range errors {
				if i < 3 { // Log first few errors
					t.Logf("Race condition error %d: %v", i, err)
				}
			}
		}

		// Clean up created files
		for _, file := range createdFiles {
			_ = os.Remove(file)
		}
	})
}

// TestTempFileCleanupPermissions tests cleanup operations with permission constraints
func TestTempFileCleanupPermissions(t *testing.T) {
	t.Parallel()

	skipIfRoot(t)

	t.Run("cleanup_readonly_temp_files", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create several temp files with different permissions
		readOnlyFile := filepath.Join(tempDir, "readonly.tmp")
		writableFile := filepath.Join(tempDir, "writable.tmp")
		noPermFile := filepath.Join(tempDir, "noperm.tmp")

		// Create read-only temp file
		err := os.WriteFile(readOnlyFile, []byte("readonly content"), 0444)
		if err != nil {
			t.Fatalf("Failed to create read-only temp file: %v", err)
		}

		// Create writable temp file
		err = os.WriteFile(writableFile, []byte("writable content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create writable temp file: %v", err)
		}

		// Create no-permission temp file
		err = os.WriteFile(noPermFile, []byte("no permission content"), 0000)
		if err != nil {
			t.Fatalf("Failed to create no-permission temp file: %v", err)
		}

		// Should be able to remove writable file
		err = os.Remove(writableFile)
		if err != nil {
			t.Errorf("Failed to remove writable temp file: %v", err)
		}

		// Should be able to remove read-only file (directory permissions matter)
		err = os.Remove(readOnlyFile)
		if err != nil {
			t.Errorf("Failed to remove read-only temp file: %v", err)
		}

		// Should be able to remove no-permission file (directory permissions matter)
		err = os.Remove(noPermFile)
		if err != nil {
			t.Errorf("Failed to remove no-permission temp file: %v", err)
		}
	})

	t.Run("cleanup_temp_directory_with_restricted_permissions", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Directory permission testing complex on Windows")
		}

		baseDir := t.TempDir()
		tempSubDir := filepath.Join(baseDir, "temp_subdir")

		// Create temp subdirectory
		err := os.MkdirAll(tempSubDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create temp subdirectory: %v", err)
		}

		// Create files in subdirectory
		tempFile := filepath.Join(tempSubDir, "file.tmp")
		err = os.WriteFile(tempFile, []byte("temp content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create temp file in subdirectory: %v", err)
		}

		// Remove write permission from subdirectory
		err = os.Chmod(tempSubDir, 0555) // read+execute only
		if err != nil {
			t.Fatalf("Failed to change subdirectory permissions: %v", err)
		}

		// Ensure cleanup can access directory
		t.Cleanup(func() {
			_ = os.Chmod(tempSubDir, 0755)
		})

		// Attempt to remove file should fail
		err = os.Remove(tempFile)
		if err == nil {
			t.Error("Expected error removing file from read-only directory")
		}
		assertPermissionError(t, err, "permission")
	})
}

// TestCrossplatformTempPermissions tests temporary file permissions across platforms
func TestCrossplatformTempPermissions(t *testing.T) {
	t.Parallel()

	t.Run("platform_specific_temp_behavior", func(t *testing.T) {
		tempDir := GetTempDirectory()
		testFile := filepath.Join(tempDir, "platform_test_"+generateRandomString(8))
		defer os.Remove(testFile)

		switch runtime.GOOS {
		case "windows":
			// On Windows, test basic file creation
			err := os.WriteFile(testFile, []byte("windows temp"), 0644)
			if err != nil {
				t.Errorf("Failed to create temp file on Windows: %v", err)
			}

		case "linux", "darwin":
			// On Unix systems, test permission-based creation
			err := os.WriteFile(testFile, []byte("unix temp"), 0600)
			if err != nil {
				t.Errorf("Failed to create temp file on Unix: %v", err)
			}

			// Verify permissions
			info, err := os.Stat(testFile)
			if err != nil {
				t.Fatalf("Failed to stat temp file: %v", err)
			}

			mode := info.Mode()
			if mode&0077 != 0 {
				t.Errorf("Expected secure permissions (600), got %o", mode)
			}

		default:
			t.Logf("Unknown platform %s, testing basic temp file creation", runtime.GOOS)
			err := os.WriteFile(testFile, []byte("unknown platform temp"), 0644)
			if err != nil {
				t.Errorf("Failed to create temp file on %s: %v", runtime.GOOS, err)
			}
		}
	})

	t.Run("temp_directory_detection_permissions", func(t *testing.T) {
		tempDir := GetTempDirectory()

		// Verify temp directory is accessible
		info, err := os.Stat(tempDir)
		if err != nil {
			t.Fatalf("Cannot access detected temp directory %s: %v", tempDir, err)
		}

		if !info.IsDir() {
			t.Errorf("Detected temp path %s is not a directory", tempDir)
		}

		// Test creating and writing to temp directory
		testFile := filepath.Join(tempDir, "detection_test_"+generateRandomString(8))
		defer os.Remove(testFile)

		content := []byte("temp directory detection test")
		err = os.WriteFile(testFile, content, 0644)
		if err != nil {
			t.Errorf("Cannot write to detected temp directory %s: %v", tempDir, err)
		}

		// Verify we can read back
		readContent, err := os.ReadFile(testFile)
		if err != nil {
			t.Errorf("Cannot read from temp file: %v", err)
		}

		if string(readContent) != string(content) {
			t.Error("Read content doesn't match written content")
		}
	})
}

// TestTempFileSecurityPermissions tests security-related temporary file scenarios
func TestTempFileSecurityPermissions(t *testing.T) {
	t.Parallel()

	skipIfRoot(t)

	t.Run("temp_file_umask_handling", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("umask not applicable on Windows")
		}

		tempDir := t.TempDir()

		// Create temp file with explicit permissions
		tempFile := filepath.Join(tempDir, "umask_test.tmp")
		err := os.WriteFile(tempFile, []byte("umask test"), 0666)
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}

		// Check actual permissions (should be modified by umask)
		info, err := os.Stat(tempFile)
		if err != nil {
			t.Fatalf("Failed to stat temp file: %v", err)
		}

		mode := info.Mode()
		t.Logf("Temp file permissions with umask: %o", mode)

		// Should not be world-writable regardless of umask for security
		if mode&0002 != 0 {
			t.Error("Temp file should not be world-writable for security")
		}
	})

	t.Run("temp_file_symbolic_link_attack_prevention", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Symbolic link behavior different on Windows")
		}

		tempDir := t.TempDir()

		// Create a target file that attacker might want to overwrite
		targetFile := filepath.Join(tempDir, "important_file.txt")
		err := os.WriteFile(targetFile, []byte("important content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create target file: %v", err)
		}

		// Create symbolic link with same name as intended temp file
		tempFileName := filepath.Join(tempDir, "temp_file.tmp")
		err = os.Symlink(targetFile, tempFileName)
		if err != nil {
			t.Fatalf("Failed to create symbolic link: %v", err)
		}

		// Try to create temp file with O_EXCL to prevent symlink attack
		file, err := os.OpenFile(tempFileName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			file.Close()
			t.Error("Expected error when creating file over existing symlink")
		}

		// Verify target file wasn't modified
		content, err := os.ReadFile(targetFile)
		if err != nil {
			t.Fatalf("Failed to read target file: %v", err)
		}

		if string(content) != "important content" {
			t.Error("Target file was modified - potential symlink attack succeeded")
		}
	})

	t.Run("temp_file_race_condition_prevention", func(t *testing.T) {
		tempDir := t.TempDir()

		// Generate unique temp file name
		tempFileName := filepath.Join(tempDir, "race_test_"+generateRandomString(16)+".tmp")

		var wg sync.WaitGroup
		var successCount int32
		var mu sync.Mutex

		// Try to create the same temp file from multiple goroutines
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				// Use O_EXCL to ensure exclusive creation
				file, err := os.OpenFile(tempFileName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
				if err == nil {
					mu.Lock()
					successCount++
					mu.Unlock()

					_, writeErr := file.WriteString(fmt.Sprintf("Created by goroutine %d", id))
					file.Close()

					if writeErr != nil {
						t.Errorf("Write error in goroutine %d: %v", id, writeErr)
					}
				}
			}(i)
		}

		wg.Wait()

		// Only one goroutine should have succeeded
		if successCount != 1 {
			t.Errorf("Expected exactly 1 successful creation, got %d", successCount)
		}

		// Clean up
		_ = os.Remove(tempFileName)
	})
}

// TestTempPermissionRecovery tests recovery scenarios after permission issues
func TestTempPermissionRecovery(t *testing.T) {
	t.Parallel()

	skipIfRoot(t)

	t.Run("temp_directory_permission_recovery", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Directory permission testing complex on Windows")
		}

		baseDir := t.TempDir()
		tempSubDir := filepath.Join(baseDir, "recovery_temp")

		// Create temp directory
		err := os.MkdirAll(tempSubDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}

		// Make directory read-only
		err = os.Chmod(tempSubDir, 0555)
		if err != nil {
			t.Fatalf("Failed to make directory read-only: %v", err)
		}

		// Verify we can't create files
		tempFile := filepath.Join(tempSubDir, "test.tmp")
		err = os.WriteFile(tempFile, []byte("test"), 0644)
		if err == nil {
			t.Error("Expected error creating file in read-only directory")
		}

		// Recover permissions
		err = os.Chmod(tempSubDir, 0755)
		if err != nil {
			t.Fatalf("Failed to recover directory permissions: %v", err)
		}

		// Should now be able to create files
		err = os.WriteFile(tempFile, []byte("recovery test"), 0644)
		if err != nil {
			t.Errorf("Failed to create file after permission recovery: %v", err)
		}

		// Clean up
		_ = os.Remove(tempFile)
	})

	t.Run("temp_file_permission_change_during_operation", func(t *testing.T) {
		tempDir := t.TempDir()
		tempFile := filepath.Join(tempDir, "permission_change.tmp")

		// Create temp file
		file, err := os.OpenFile(tempFile, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}

		// Write initial content
		_, err = file.WriteString("initial content\n")
		if err != nil {
			t.Fatalf("Failed to write initial content: %v", err)
		}

		// Change file permissions while file is open
		err = os.Chmod(tempFile, 0444) // read-only
		if err != nil {
			t.Fatalf("Failed to change file permissions: %v", err)
		}

		// Try to write more content (might succeed because file is already open)
		_, err = file.WriteString("additional content\n")
		if err != nil {
			t.Logf("Write failed after permission change (expected on some systems): %v", err)
		} else {
			t.Log("Write succeeded despite permission change (file already open)")
		}

		file.Close()

		// Try to reopen for writing (should fail)
		_, err = os.OpenFile(tempFile, os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			t.Error("Expected error reopening read-only file for writing")
		}
	})
}

// Helper Functions for Temp Permission Tests

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
		"read-only file system",
		"file exists",
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

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	bytes := make([]byte, length)

	_, err := rand.Read(bytes)
	if err != nil {
		// Fallback to timestamp-based generation
		return fmt.Sprintf("%d", time.Now().UnixNano())[:length]
	}

	for i, b := range bytes {
		bytes[i] = charset[b%byte(len(charset))]
	}

	return string(bytes)
}
