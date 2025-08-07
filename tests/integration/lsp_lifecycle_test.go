package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server"
	"lsp-gateway/src/server/cache"
)

func TestLSPManagerLifecycleErrorHandling(t *testing.T) {
	_, err := exec.LookPath("gopls")
	if err != nil {
		t.Skip("Go LSP server (gopls) not installed, skipping test")
	}

	testDir := t.TempDir()
	testFile := filepath.Join(testDir, "test.go")
	err = os.WriteFile(testFile, []byte(`package main

import "fmt"

func main() {
	fmt.Println("test")
}

func testFunction(param string) string {
	return param + " processed"
}
`), 0644)
	require.NoError(t, err)

	t.Run("StartupWithInvalidServer", func(t *testing.T) {
		cfg := &config.Config{
			Servers: map[string]*config.ServerConfig{
				"invalid": {
					Command: "nonexistent-lsp-server",
					Args:    []string{"serve"},
				},
			},
		}
		
		lspManager, err := server.NewLSPManager(cfg)
		require.NoError(t, err)
		
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		
		err = lspManager.Start(ctx)
		// Manager starts successfully even if individual servers fail
		require.NoError(t, err)
		defer lspManager.Stop()
		
		// Try to process a request for the invalid language
		testURI := uri.File(testFile)
		params := &protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: testURI,
				},
				Position: protocol.Position{
					Line:      9,
					Character: 5,
				},
			},
		}
		
		_, err = lspManager.ProcessRequest(ctx, "textDocument/definition", params)
		// Should get an error since no valid LSP server is running
		require.Error(t, err)
	})

	t.Run("NormalOperationWithGopls", func(t *testing.T) {
		cfg := &config.Config{
			Servers: map[string]*config.ServerConfig{
				"go": {
					Command: "gopls",
					Args:    []string{"serve"},
				},
			},
		}
		
		lspManager, err := server.NewLSPManager(cfg)
		require.NoError(t, err)
		
		ctx := context.Background()
		err = lspManager.Start(ctx)
		require.NoError(t, err)
		defer lspManager.Stop()
		
		// Test normal operation
		testURI := uri.File(testFile)
		params := &protocol.HoverParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: testURI,
				},
				Position: protocol.Position{
					Line:      5,
					Character: 5,
				},
			},
		}
		
		result, err := lspManager.ProcessRequest(ctx, "textDocument/hover", params)
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("ContextCancellationDuringRequest", func(t *testing.T) {
		cfg := &config.Config{
			Servers: map[string]*config.ServerConfig{
				"go": {
					Command: "gopls",
					Args:    []string{"serve"},
				},
			},
		}
		
		lspManager, err := server.NewLSPManager(cfg)
		require.NoError(t, err)
		
		ctx := context.Background()
		err = lspManager.Start(ctx)
		require.NoError(t, err)
		defer lspManager.Stop()
		
		requestCtx, cancel := context.WithCancel(context.Background())
		
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()
		
		testURI := uri.File(testFile)
		params := &protocol.HoverParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: testURI,
				},
				Position: protocol.Position{
					Line:      5,
					Character: 5,
				},
			},
		}
		
		// This might succeed if it completes before cancellation
		_, err = lspManager.ProcessRequest(requestCtx, "textDocument/hover", params)
		if err != nil {
			require.Contains(t, err.Error(), "context canceled")
		}
	})

	t.Run("CacheStartFailure", func(t *testing.T) {
		invalidCacheDir := "/invalid/path/that/does/not/exist/cache"
		cacheConfig := &config.CacheConfig{
			Enabled:      true,
			StoragePath:  invalidCacheDir,
			MaxMemoryMB:  64,
			TTLHours:     1,
			Languages:    []string{"go"},
			DiskCache:    true,
		}
		
		_, err := cache.NewSCIPCacheManager(cacheConfig)
		require.Error(t, err)
	})
}

func TestLSPManagerWithCache(t *testing.T) {
	_, err := exec.LookPath("gopls")
	if err != nil {
		t.Skip("Go LSP server (gopls) not installed, skipping test")
	}

	testDir := t.TempDir()
	cacheDir := filepath.Join(testDir, "cache")
	testFile := filepath.Join(testDir, "cache_test.go")
	err = os.WriteFile(testFile, []byte(`package main

func cacheTest() string {
	return "cached"
}
`), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		Cache: &config.CacheConfig{
			Enabled:      true,
			StoragePath:  cacheDir,
			MaxMemoryMB:  64,
			TTLHours:     1,
			Languages:    []string{"go"},
		},
		Servers: map[string]*config.ServerConfig{
			"go": {
				Command: "gopls",
				Args:    []string{"serve"},
			},
		},
	}
	
	lspManager, err := server.NewLSPManager(cfg)
	require.NoError(t, err)
	
	ctx := context.Background()
	err = lspManager.Start(ctx)
	require.NoError(t, err)
	defer lspManager.Stop()

	t.Run("CacheIntegration", func(t *testing.T) {
		testURI := uri.File(testFile)
		params := &protocol.HoverParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: testURI,
				},
				Position: protocol.Position{
					Line:      2,
					Character: 5,
				},
			},
		}
		
		// First request - should go to LSP server
		result1, err := lspManager.ProcessRequest(ctx, "textDocument/hover", params)
		require.NoError(t, err)
		require.NotNil(t, result1)
		
		// Second request - might be cached
		result2, err := lspManager.ProcessRequest(ctx, "textDocument/hover", params)
		require.NoError(t, err)
		require.NotNil(t, result2)
	})
}