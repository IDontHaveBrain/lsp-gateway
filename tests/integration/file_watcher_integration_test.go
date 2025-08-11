package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server"
)

// Verifies that the file watcher triggers incremental indexing when files change
func TestFileWatcherIncrementalIndexing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	if _, err := exec.LookPath("gopls"); err != nil {
		t.Skip("Go LSP server (gopls) not installed, skipping test")
	}

	testDir := t.TempDir()
	goMod := `module watcher-test

go 1.21
`
	require.NoError(t, os.WriteFile(filepath.Join(testDir, "go.mod"), []byte(goMod), 0644))

	filePath := filepath.Join(testDir, "main.go")
	initial := `package main

func A() int { return 1 }
`
	require.NoError(t, os.WriteFile(filePath, []byte(initial), 0644))

	origWD, _ := os.Getwd()
	require.NoError(t, os.Chdir(testDir))
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	cfg := &config.Config{
		Cache: &config.CacheConfig{
			Enabled:         true,
			StoragePath:     t.TempDir(),
			MaxMemoryMB:     64,
			TTLHours:        1,
			BackgroundIndex: true,
			Languages:       []string{"go"},
		},
		Servers: map[string]*config.ServerConfig{
			"go": {Command: "gopls", Args: []string{"serve"}},
		},
	}

	mgr, err := server.NewLSPManager(cfg)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	require.NoError(t, mgr.Start(ctx))
	defer mgr.Stop()

	// Small delay to ensure watcher routine is running
	time.Sleep(300 * time.Millisecond)

	// Capture baseline stats
	before := mgr.GetCache().GetIndexStats()

	// Modify the file to trigger fsnotify events and incremental indexing
	updated := `package main

func A() int { return 1 }
func B() int { return 2 } // new symbol
`
	require.NoError(t, os.WriteFile(filePath, []byte(updated), 0644))

	// Wait for debounce (~500ms) and indexing to complete; poll up to ~15s
	deadline := time.Now().Add(15 * time.Second)
	for {
		after := mgr.GetCache().GetIndexStats()
		// DocumentCount should be >= 1 once the file is indexed; LastUpdate change also indicates indexing
		if after != nil && (after.DocumentCount > before.DocumentCount || after.LastUpdate.After(before.LastUpdate)) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("incremental indexing did not complete: before=%+v after=%+v", before, after)
		}
		time.Sleep(250 * time.Millisecond)
	}
}
