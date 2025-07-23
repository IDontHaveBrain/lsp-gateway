package cli_test

import (
	"lsp-gateway/internal/cli"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestLogFileCreationPermissions tests log file creation with various permission constraints
func TestLogFileCreationPermissions(t *testing.T) {
	t.Parallel()

	skipIfRoot(t)

	t.Run("log_directory_no_write_permission", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Directory write permission testing complex on Windows")
		}

		tmpDir := t.TempDir()
		logDir := filepath.Join(tmpDir, "nolog")

		// Create directory without write permissions
		err := os.MkdirAll(logDir, 0555) // read+execute only
		if err != nil {
			t.Fatalf("Failed to create log directory: %v", err)
		}

		// Ensure cleanup can access directory
		t.Cleanup(func() {
			_ = os.Chmod(logDir, 0755)
		})

		logFile := filepath.Join(logDir, "gateway.log")

		// Attempt to create log file should fail
		_, err = os.Create(logFile)
		if err == nil {
			t.Error("Expected error creating log file in non-writable directory")
		}
		assertPermissionError(t, err, "permission")
	})

	t.Run("log_directory_no_execute_permission", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Directory execute permission not applicable on Windows")
		}

		tmpDir := t.TempDir()
		logDir := filepath.Join(tmpDir, "noexec")

		// Create directory and log file first
		err := os.MkdirAll(logDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create log directory: %v", err)
		}

		logFile := filepath.Join(logDir, "gateway.log")
		err = os.WriteFile(logFile, []byte("initial log entry\n"), 0644)
		if err != nil {
			t.Fatalf("Failed to create initial log file: %v", err)
		}

		// Remove execute permission from directory
		err = os.Chmod(logDir, 0600) // read+write only, no execute
		if err != nil {
			t.Fatalf("Failed to change directory permissions: %v", err)
		}

		// Ensure cleanup can access directory
		t.Cleanup(func() {
			_ = os.Chmod(logDir, 0755)
		})

		// Attempt to access log file should fail
		_, err = os.OpenFile(logFile, os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			t.Error("Expected error accessing log file in non-executable directory")
		}
		assertPermissionError(t, err, "permission")
	})

	t.Run("existing_log_file_no_write_permission", func(t *testing.T) {
		tmpDir := t.TempDir()
		logFile := filepath.Join(tmpDir, "readonly.log")

		// Create log file with initial content
		err := os.WriteFile(logFile, []byte("initial log entry\n"), 0644)
		if err != nil {
			t.Fatalf("Failed to create initial log file: %v", err)
		}

		// Make file read-only
		err = os.Chmod(logFile, 0444)
		if err != nil {
			t.Fatalf("Failed to make log file read-only: %v", err)
		}

		// Attempt to write to read-only log file should fail
		file, err := os.OpenFile(logFile, os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			file.Close()
			t.Error("Expected error opening read-only log file for writing")
		}
		assertPermissionError(t, err, "permission")
	})
}

// TestLogFileWritePermissions tests writing to log files with permission constraints
func TestLogFileWritePermissions(t *testing.T) {
	t.Parallel()

	skipIfRoot(t)

	t.Run("log_write_permission_denied_during_operation", func(t *testing.T) {
		tmpDir := t.TempDir()
		logFile := filepath.Join(tmpDir, "operation.log")

		// Create log file with write permissions
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			t.Fatalf("Failed to create log file: %v", err)
		}

		// Write initial entry
		_, err = file.WriteString("Initial log entry\n")
		if err != nil {
			t.Fatalf("Failed to write initial log entry: %v", err)
		}
		file.Close()

		// Change file to read-only while simulating active logging
		err = os.Chmod(logFile, 0444)
		if err != nil {
			t.Fatalf("Failed to make file read-only: %v", err)
		}

		// Attempt to reopen for writing should fail
		_, err = os.OpenFile(logFile, os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			t.Error("Expected error reopening read-only log file for writing")
		}
		assertPermissionError(t, err, "permission")
	})

	t.Run("log_file_permission_recovery", func(t *testing.T) {
		tmpDir := t.TempDir()
		logFile := filepath.Join(tmpDir, "recovery.log")

		// Create log file
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			t.Fatalf("Failed to create log file: %v", err)
		}
		file.Close()

		// Make file read-only
		err = os.Chmod(logFile, 0444)
		if err != nil {
			t.Fatalf("Failed to make file read-only: %v", err)
		}

		// Verify write fails
		_, err = os.OpenFile(logFile, os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			t.Error("Expected error writing to read-only file")
		}

		// Restore write permissions
		err = os.Chmod(logFile, 0644)
		if err != nil {
			t.Fatalf("Failed to restore write permissions: %v", err)
		}

		// Verify write succeeds after recovery
		file, err = os.OpenFile(logFile, os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			t.Errorf("Expected successful write after permission recovery: %v", err)
		}
		if file != nil {
			_, err = file.WriteString("Recovery successful\n")
			if err != nil {
				t.Errorf("Failed to write after permission recovery: %v", err)
			}
			file.Close()
		}
	})
}

// TestLogRotationPermissions tests log rotation with permission constraints
func TestLogRotationPermissions(t *testing.T) {
	t.Parallel()

	skipIfRoot(t)

	t.Run("log_rotation_directory_permission_denied", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Directory permission testing complex on Windows")
		}

		tmpDir := t.TempDir()
		logDir := filepath.Join(tmpDir, "logrotate")

		// Create directory and initial log file
		err := os.MkdirAll(logDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create log directory: %v", err)
		}

		logFile := filepath.Join(logDir, "gateway.log")
		err = os.WriteFile(logFile, []byte("log content\n"), 0644)
		if err != nil {
			t.Fatalf("Failed to create log file: %v", err)
		}

		// Remove write permission from directory (preventing new file creation)
		err = os.Chmod(logDir, 0555) // read+execute only
		if err != nil {
			t.Fatalf("Failed to change directory permissions: %v", err)
		}

		// Ensure cleanup can access directory
		t.Cleanup(func() {
			_ = os.Chmod(logDir, 0755)
		})

		// Simulate log rotation by trying to create backup file
		backupFile := filepath.Join(logDir, "gateway.log.1")
		err = os.Rename(logFile, backupFile)
		if err == nil {
			t.Error("Expected error during log rotation in read-only directory")
		}
		assertPermissionError(t, err, "permission")
	})

	t.Run("log_rotation_backup_file_permission_denied", func(t *testing.T) {
		tmpDir := t.TempDir()
		logFile := filepath.Join(tmpDir, "gateway.log")
		backupFile := filepath.Join(tmpDir, "gateway.log.bak")

		// Create original log file
		err := os.WriteFile(logFile, []byte("original log content\n"), 0644)
		if err != nil {
			t.Fatalf("Failed to create log file: %v", err)
		}

		// Create backup file with no write permissions
		err = os.WriteFile(backupFile, []byte("old backup\n"), 0444)
		if err != nil {
			t.Fatalf("Failed to create backup file: %v", err)
		}

		// Simulate rotation that would overwrite read-only backup
		err = os.Rename(logFile, backupFile)
		// Note: os.Rename behavior with read-only targets is platform-dependent
		// On many Unix systems, rename can overwrite read-only files if directory allows
		if err != nil {
			t.Logf("Rename failed as expected due to backup file permissions: %v", err)
		} else {
			t.Log("Rename succeeded despite read-only backup (platform allows overwrite)")
		}
	})
}

// TestConcurrentLogWritePermissions tests concurrent logging with permission constraints
func TestConcurrentLogWritePermissions(t *testing.T) {
	t.Parallel()

	skipIfRoot(t)

	t.Run("concurrent_write_permission_change", func(t *testing.T) {
		tmpDir := t.TempDir()
		logFile := filepath.Join(tmpDir, "concurrent.log")

		// Create log file
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			t.Fatalf("Failed to create log file: %v", err)
		}
		file.Close()

		var wg sync.WaitGroup
		var mu sync.Mutex
		results := make([]error, 0, 20)

		// Start concurrent writers
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for j := 0; j < 5; j++ {
					time.Sleep(time.Duration(j) * time.Millisecond)

					file, err := os.OpenFile(logFile, os.O_WRONLY|os.O_APPEND, 0644)
					if err != nil {
						mu.Lock()
						results = append(results, fmt.Errorf("goroutine %d: %w", id, err))
						mu.Unlock()
						continue
					}

					_, writeErr := file.WriteString(fmt.Sprintf("Entry from goroutine %d iteration %d\n", id, j))
					file.Close()

					if writeErr != nil {
						mu.Lock()
						results = append(results, fmt.Errorf("goroutine %d write: %w", id, writeErr))
						mu.Unlock()
					}
				}
			}(i)
		}

		// Concurrently change permissions
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				time.Sleep(2 * time.Millisecond)

				if i%2 == 0 {
					_ = os.Chmod(logFile, 0444) // read-only
				} else {
					_ = os.Chmod(logFile, 0644) // read-write
				}
			}
		}()

		wg.Wait()

		// Analyze results
		errorCount := len(results)
		t.Logf("Concurrent logging with permission changes: %d errors out of 50 operations", errorCount)

		// Should have some errors due to permission changes
		if errorCount == 0 {
			t.Log("No errors during concurrent permission changes - this might indicate insufficient permission enforcement")
		}

		for i, err := range results {
			if i < 5 { // Log first few errors
				t.Logf("Concurrent error %d: %v", i, err)
			}
		}
	})
}

// TestStandardLoggerPermissions tests integration with Go's standard logger
func TestStandardLoggerPermissions(t *testing.T) {
	t.Parallel()

	skipIfRoot(t)

	t.Run("standard_logger_permission_denied", func(t *testing.T) {
		tmpDir := t.TempDir()
		logFile := filepath.Join(tmpDir, "stdlog.log")

		// Create log file
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			t.Fatalf("Failed to create log file: %v", err)
		}
		file.Close()

		// Make file read-only
		err = os.Chmod(logFile, 0444)
		if err != nil {
			t.Fatalf("Failed to make file read-only: %v", err)
		}

		// Try to open for logging
		logWriter, err := os.OpenFile(logFile, os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			logWriter.Close()
			t.Error("Expected error opening read-only file for logging")
		}
		assertPermissionError(t, err, "permission")
	})

	t.Run("logger_with_permission_fallback", func(t *testing.T) {
		tmpDir := t.TempDir()
		primaryLog := filepath.Join(tmpDir, "primary.log")
		fallbackLog := filepath.Join(tmpDir, "fallback.log")

		// Create primary log file
		err := os.WriteFile(primaryLog, []byte("primary log\n"), 0644)
		if err != nil {
			t.Fatalf("Failed to create primary log: %v", err)
		}

		// Create fallback log file
		err = os.WriteFile(fallbackLog, []byte("fallback log\n"), 0644)
		if err != nil {
			t.Fatalf("Failed to create fallback log: %v", err)
		}

		// Make primary log read-only
		err = os.Chmod(primaryLog, 0444)
		if err != nil {
			t.Fatalf("Failed to make primary log read-only: %v", err)
		}

		// Simulate logger fallback logic
		var logger *log.Logger

		// Try primary first
		primaryFile, err := os.OpenFile(primaryLog, os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			// Fall back to secondary
			fallbackFile, fallbackErr := os.OpenFile(fallbackLog, os.O_WRONLY|os.O_APPEND, 0644)
			if fallbackErr != nil {
				t.Fatalf("Both primary and fallback logging failed: primary=%v, fallback=%v", err, fallbackErr)
			}
			logger = log.New(fallbackFile, "", log.LstdFlags)
			fallbackFile.Close()
		} else {
			primaryFile.Close()
			t.Error("Expected primary log to be read-only")
		}

		if logger == nil {
			t.Error("Expected fallback logger to be created")
		}
	})
}

// TestLogPermissionSecurityScenarios tests security-related logging permission scenarios
func TestLogPermissionSecurityScenarios(t *testing.T) {
	t.Parallel()

	skipIfRoot(t)

	t.Run("secure_log_file_permissions", func(t *testing.T) {
		tmpDir := t.TempDir()
		secureLog := filepath.Join(tmpDir, "secure.log")

		// Create log file with secure permissions (600 - owner only)
		file, err := os.OpenFile(secureLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			t.Fatalf("Failed to create secure log file: %v", err)
		}

		_, err = file.WriteString("Secure log entry\n")
		if err != nil {
			t.Errorf("Failed to write to secure log: %v", err)
		}
		file.Close()

		// Verify permissions are secure
		info, err := os.Stat(secureLog)
		if err != nil {
			t.Fatalf("Failed to stat secure log: %v", err)
		}

		mode := info.Mode()
		if runtime.GOOS != "windows" {
			// On Unix, verify only owner has permissions
			if mode&0077 != 0 {
				t.Errorf("Expected secure permissions (600), got %o", mode)
			}
		}
	})

	t.Run("world_writable_log_directory", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("World-writable concept different on Windows")
		}

		tmpDir := t.TempDir()
		publicLogDir := filepath.Join(tmpDir, "public_logs")

		// Create directory and set world-writable permissions explicitly
		err := os.MkdirAll(publicLogDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create public log directory: %v", err)
		}

		// Explicitly set world-writable permissions
		err = os.Chmod(publicLogDir, 0777)
		if err != nil {
			t.Fatalf("Failed to set world-writable permissions: %v", err)
		}

		logFile := filepath.Join(publicLogDir, "public.log")

		// Should be able to create log file
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			t.Errorf("Failed to create log in world-writable directory: %v", err)
		}
		if file != nil {
			file.Close()
		}

		// Verify directory permissions
		dirInfo, err := os.Stat(publicLogDir)
		if err != nil {
			t.Fatalf("Failed to stat public directory: %v", err)
		}

		dirMode := dirInfo.Mode()
		if dirMode&0002 == 0 {
			t.Log("Directory permissions were restricted by umask - good for security")
		} else {
			t.Logf("Directory has world-write permissions (%o) - potential security risk", dirMode)
		}

		t.Log("WARNING: World-writable log directory detected - potential security risk")
	})
}

// TestLogPermissionCleanupScenarios tests cleanup operations with permission constraints
func TestLogPermissionCleanupScenarios(t *testing.T) {
	t.Parallel()

	skipIfRoot(t)

	t.Run("log_cleanup_permission_denied", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Directory permission testing complex on Windows")
		}

		tmpDir := t.TempDir()
		logDir := filepath.Join(tmpDir, "cleanup_test")

		// Create directory and log files
		err := os.MkdirAll(logDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create log directory: %v", err)
		}

		// Create several log files
		for i := 0; i < 3; i++ {
			logFile := filepath.Join(logDir, fmt.Sprintf("old_%d.log", i))
			err = os.WriteFile(logFile, []byte(fmt.Sprintf("Old log %d\n", i)), 0644)
			if err != nil {
				t.Fatalf("Failed to create old log file %d: %v", i, err)
			}
		}

		// Remove write permission from directory
		err = os.Chmod(logDir, 0555) // read+execute only
		if err != nil {
			t.Fatalf("Failed to change directory permissions: %v", err)
		}

		// Ensure cleanup can access directory
		t.Cleanup(func() {
			_ = os.Chmod(logDir, 0755)
		})

		// Attempt to remove old log files should fail
		oldLogFile := filepath.Join(logDir, "old_0.log")
		err = os.Remove(oldLogFile)
		if err == nil {
			t.Error("Expected error removing file from read-only directory")
		}
		assertPermissionError(t, err, "permission")
	})

	t.Run("log_cleanup_partial_permissions", func(t *testing.T) {
		tmpDir := t.TempDir()
		logDir := filepath.Join(tmpDir, "partial_cleanup")

		// Create directory and log files
		err := os.MkdirAll(logDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create log directory: %v", err)
		}

		writableLog := filepath.Join(logDir, "writable.log")
		readonlyLog := filepath.Join(logDir, "readonly.log")

		// Create writable log
		err = os.WriteFile(writableLog, []byte("Writable log\n"), 0644)
		if err != nil {
			t.Fatalf("Failed to create writable log: %v", err)
		}

		// Create read-only log
		err = os.WriteFile(readonlyLog, []byte("Read-only log\n"), 0444)
		if err != nil {
			t.Fatalf("Failed to create read-only log: %v", err)
		}

		// Should be able to remove writable log
		err = os.Remove(writableLog)
		if err != nil {
			t.Errorf("Failed to remove writable log: %v", err)
		}

		// Removing read-only file might succeed or fail depending on directory permissions
		err = os.Remove(readonlyLog)
		if err != nil {
			t.Logf("Cannot remove read-only log file (expected): %v", err)
		} else {
			t.Log("Read-only log file was successfully removed (directory permissions allow)")
		}
	})
}

// Helper Functions for Log Permission Tests

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
