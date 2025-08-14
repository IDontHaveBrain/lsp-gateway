package common

import (
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestValidateAndGetWorkingDir(t *testing.T) {
    tempDir := t.TempDir()
    currentDir, _ := os.Getwd()

    t.Run("empty_uses_current", func(t *testing.T) {
        got, err := ValidateAndGetWorkingDir("")
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        if got != currentDir {
            t.Fatalf("expected %q got %q", currentDir, got)
        }
    })

    t.Run("valid_absolute", func(t *testing.T) {
        got, err := ValidateAndGetWorkingDir(tempDir)
        if err != nil || !filepath.IsAbs(got) {
            t.Fatalf("got %q err %v", got, err)
        }
    })

    t.Run("relative_dot", func(t *testing.T) {
        got, err := ValidateAndGetWorkingDir(".")
        if err != nil || !filepath.IsAbs(got) {
            t.Fatalf("got %q err %v", got, err)
        }
    })

    t.Run("nonexistent", func(t *testing.T) {
        _, err := ValidateAndGetWorkingDir("/nonexistent/path")
        if err == nil || !strings.Contains(err.Error(), "does not exist") {
            t.Fatalf("expected not exist error, got %v", err)
        }
    })

    t.Run("file_instead_of_dir", func(t *testing.T) {
        f := filepath.Join(tempDir, "f.txt")
        if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
            t.Fatalf("write: %v", err)
        }
        _, err := ValidateAndGetWorkingDir(f)
        if err == nil || !strings.Contains(err.Error(), "not a directory") {
            t.Fatalf("expected directory error, got %v", err)
        }
    })
}

func TestExpandPath(t *testing.T) {
    home, _ := os.UserHomeDir()
    got, err := ExpandPath("~")
    if err != nil || got != home {
        t.Fatalf("expand ~ failed: %q %v", got, err)
    }
    got2, err := ExpandPath("~/sub")
    if err != nil || !strings.HasPrefix(got2, home+string(os.PathSeparator)) {
        t.Fatalf("expand ~/sub failed: %q %v", got2, err)
    }
    got3, err := ExpandPath("/etc")
    if err != nil || got3 != "/etc" {
        t.Fatalf("passthrough failed: %q %v", got3, err)
    }
}

func TestGetLSPToolPath(t *testing.T) {
    p := GetLSPToolPath("go", "gopls")
    s := filepath.ToSlash(p)
    if !strings.Contains(s, "/.lsp-gateway/tools/go/bin/gopls") {
        t.Fatalf("unexpected path: %q", s)
    }
}

