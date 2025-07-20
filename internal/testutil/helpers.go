package testutil

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TempFile(t *testing.T, pattern string, content string) string {
	t.Helper()

	tmpfile, err := os.CreateTemp("", pattern)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	if _, err := tmpfile.WriteString(content); err != nil {
		if closeErr := tmpfile.Close(); closeErr != nil {
			t.Logf("Failed to close temp file during cleanup: %v", closeErr)
		}
		if removeErr := os.Remove(tmpfile.Name()); removeErr != nil {
			t.Logf("Failed to remove temp file during cleanup: %v", removeErr)
		}
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	if err := tmpfile.Close(); err != nil {
		if removeErr := os.Remove(tmpfile.Name()); removeErr != nil {
			t.Logf("Failed to remove temp file during cleanup: %v", removeErr)
		}
		t.Fatalf("Failed to close temp file: %v", err)
	}

	t.Cleanup(func() {
		if err := os.Remove(tmpfile.Name()); err != nil {
			t.Logf("Warning: failed to remove temp file %s: %v", tmpfile.Name(), err)
		}
	})

	return tmpfile.Name()
}

func TempDir(t *testing.T) string {
	t.Helper()

	tmpdir, err := os.MkdirTemp("", "lsp-gateway-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(tmpdir)
	})

	return tmpdir
}

func CreateTestFile(t *testing.T, dir, filename, content string) string {
	t.Helper()

	filepath := filepath.Join(dir, filename)
	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	return filepath
}

func AssertEqual(t *testing.T, expected, actual interface{}) {
	t.Helper()

	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func AssertNil(t *testing.T, value interface{}) {
	t.Helper()

	if value != nil {
		t.Errorf("Expected nil, got %v", value)
	}
}

func AssertNotNil(t *testing.T, value interface{}) {
	t.Helper()

	if value == nil {
		t.Error("Expected non-nil value, got nil")
	}
}

func AssertError(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func AssertNoError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func AssertContains(t *testing.T, str, substr string) {
	t.Helper()

	if !contains(str, substr) {
		t.Errorf("Expected %q to contain %q", str, substr)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsAt(s, substr)
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func AssertJSONEqual(t *testing.T, expected, actual string) {
	t.Helper()

	var expectedJSON, actualJSON interface{}

	if err := json.Unmarshal([]byte(expected), &expectedJSON); err != nil {
		t.Fatalf("Failed to unmarshal expected JSON: %v", err)
	}

	if err := json.Unmarshal([]byte(actual), &actualJSON); err != nil {
		t.Fatalf("Failed to unmarshal actual JSON: %v", err)
	}

	expectedBytes, _ := json.Marshal(expectedJSON)
	actualBytes, _ := json.Marshal(actualJSON)

	if string(expectedBytes) != string(actualBytes) {
		t.Errorf("JSON not equal.\nExpected: %s\nActual: %s", expected, actual)
	}
}

func ContextWithTimeout(t *testing.T, timeout time.Duration) context.Context {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)

	return ctx
}

func CaptureOutput(t *testing.T, f func()) string {
	t.Helper()

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	_ = w.Close()
	os.Stdout = old

	out, _ := io.ReadAll(r)
	return string(out)
}

func SkipIfShort(t *testing.T) {
	t.Helper()

	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}
}

func SkipOnWindows(t *testing.T) {
	t.Helper()

	if os.PathSeparator == '\\' {
		t.Skip("Skipping test on Windows")
	}
}

func RequireCommand(t *testing.T, command string) {
	t.Helper()

	if _, err := exec.LookPath(command); err != nil {
		t.Skipf("Skipping test: command %q not found", command)
	}
}
