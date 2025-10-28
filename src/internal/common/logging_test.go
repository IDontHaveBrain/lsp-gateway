package common

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewSafeLoggerLevels(t *testing.T) {
	t.Setenv("LSP_GATEWAY_DEBUG", "")
	l := NewSafeLogger("TEST")
	if l.level != LogInfo {
		t.Fatalf("expected info level")
	}
	t.Setenv("LSP_GATEWAY_DEBUG", "true")
	l2 := NewSafeLogger("TEST")
	if l2.level != LogDebug {
		t.Fatalf("expected debug level")
	}
}

func TestLoggerWritesToStderr(t *testing.T) {
	r, w, _ := os.Pipe()
	oldErr := os.Stderr
	oldOut := os.Stdout
	os.Stderr = w
	os.Stdout = w
	defer func() { os.Stderr = oldErr; os.Stdout = oldOut }()

	l := NewSafeLogger("TEST")
	l.Info("hello")
	require.NoError(t, w.Close())
	buf := make([]byte, 1024)
	n, err := r.Read(buf)
	require.NoError(t, err)
	s := string(buf[:n])
	if !strings.Contains(s, "TEST:") {
		t.Fatalf("missing prefix: %q", s)
	}
}

func TestSanitizeErrorForLogging(t *testing.T) {
	if SanitizeErrorForLogging(nil) != "" {
		t.Fatalf("nil should be empty")
	}
	long := strings.Repeat("x", 250)
	if got := SanitizeErrorForLogging(long); !strings.HasSuffix(got, "...") {
		t.Fatalf("expected truncation")
	}
	ts := "TypeScript Server Error: x\n  at a\n  at b"
	if got := SanitizeErrorForLogging(ts); !strings.Contains(got, "TypeScript Server Error") {
		t.Fatalf("expected ts prefix")
	}
}
