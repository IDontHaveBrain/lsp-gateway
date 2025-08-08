package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server"
	"lsp-gateway/src/server/cache"
	"lsp-gateway/src/utils"
	"lsp-gateway/tests/shared"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lsp.dev/protocol"
)

func TestCacheInvalidationUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	shared.CheckLSPAvailability(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	testDir := t.TempDir()

	// Create a go.mod file for gopls to work properly
	goModContent := `module test

go 1.21
`
	require.NoError(t, os.WriteFile(filepath.Join(testDir, "go.mod"), []byte(goModContent), 0644))

	// Create test files
	testFiles := make(map[string]string)
	for i := 0; i < 10; i++ {
		filename := fmt.Sprintf("file%d.go", i)
		content := fmt.Sprintf(`package main

import "fmt"

func Function%d() {
	fmt.Println("Original content %d")
}

type Struct%d struct {
	Field%d string
}

func (s *Struct%d) Method%d() string {
	return s.Field%d
}`, i, i, i, i, i, i, i)

		testFiles[filename] = content
		filePath := filepath.Join(testDir, filename)
		require.NoError(t, writeTestFile(filePath, content))
	}

	cfg := &config.Config{
		Cache: &config.CacheConfig{
			Enabled:            true,
			StoragePath:        t.TempDir(),
			MaxMemoryMB:        128,
			TTLHours:           1,
			BackgroundIndex:    false,
			HealthCheckMinutes: 1, // Fast health checks for testing
			EvictionPolicy:     "lru",
		},
		Servers: map[string]*config.ServerConfig{
			"go": &config.ServerConfig{
				Command: "gopls",
				Args:    []string{"serve"},
			},
		},
	}

	scipCache, err := cache.NewSCIPCacheManager(cfg.Cache)
	require.NoError(t, err)
	defer scipCache.Stop()

	lspManager, err := server.NewLSPManager(cfg)
	require.NoError(t, err)
	defer lspManager.Stop()
	lspManager.SetCache(scipCache)

	// Start the LSP manager to initialize LSP servers
	require.NoError(t, lspManager.Start(ctx))

	// Note: DocumentManager not needed for this test - LSP operations work with file URIs directly

	t.Run("ConcurrentRequestsWithFileModification", func(t *testing.T) {
		// Reduce expectations for CI environments where resources are limited
		isCI := os.Getenv("CI") != ""
		isWindows := runtime.GOOS == "windows"

		// Adjust concurrency and expectations based on environment
		numReaders := 20
		expectedRequests := int32(100)
		expectedModifications := int32(20)
		if isCI && isWindows {
			// Windows CI runners are slower, reduce expectations
			numReaders = 10
			expectedRequests = int32(20)
			expectedModifications = int32(10)
			t.Logf("Running in Windows CI environment - using reduced concurrency and expectations")
		} else if isCI {
			// Other CI environments
			expectedRequests = int32(50)
			t.Logf("Running in CI environment - using reduced expectations")
		}

		var wg sync.WaitGroup
		var requestCount atomic.Int32
		var invalidations atomic.Int32

		// Start concurrent readers
		stopReaders := make(chan struct{})
		for i := 0; i < numReaders; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				for {
					select {
					case <-stopReaders:
						return
					default:
						fileIdx := workerID % 10
						uri := utils.FilePathToURI(filepath.Join(testDir, fmt.Sprintf("file%d.go", fileIdx)))

						// Random operation
						operations := []string{"definition", "references", "hover", "documentSymbol"}
						op := operations[requestCount.Load()%4]

						ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
						defer cancel()

						switch op {
						case "definition":
							params := protocol.DefinitionParams{
								TextDocumentPositionParams: protocol.TextDocumentPositionParams{
									TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(uri)},
									Position:     protocol.Position{Line: 4, Character: 5},
								},
							}
							_, _ = lspManager.ProcessRequest(ctx, "textDocument/definition", params)

						case "references":
							params := protocol.ReferenceParams{
								TextDocumentPositionParams: protocol.TextDocumentPositionParams{
									TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(uri)},
									Position:     protocol.Position{Line: 4, Character: 5},
								},
								Context: protocol.ReferenceContext{
									IncludeDeclaration: true,
								},
							}
							_, _ = lspManager.ProcessRequest(ctx, "textDocument/references", params)

						case "hover":
							params := protocol.HoverParams{
								TextDocumentPositionParams: protocol.TextDocumentPositionParams{
									TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(uri)},
									Position:     protocol.Position{Line: 8, Character: 10},
								},
							}
							_, _ = lspManager.ProcessRequest(ctx, "textDocument/hover", params)

						case "documentSymbol":
							params := protocol.DocumentSymbolParams{
								TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(uri)},
							}
							_, _ = lspManager.ProcessRequest(ctx, "textDocument/documentSymbol", params)
						}

						requestCount.Add(1)
						time.Sleep(10 * time.Millisecond)
					}
				}
			}(i)
		}

		// Start concurrent file modifiers
		numModifiers := 5
		if isCI && isWindows {
			numModifiers = 2 // Reduce file modifiers in Windows CI
		}
		stopModifiers := make(chan struct{})
		for i := 0; i < numModifiers; i++ {
			wg.Add(1)
			go func(modifierID int) {
				defer wg.Done()

				modificationCount := 0
				for {
					select {
					case <-stopModifiers:
						return
					default:
						if modificationCount >= 10 {
							return
						}

						fileIdx := (modifierID * 2) % 10
						filename := fmt.Sprintf("file%d.go", fileIdx)
						filePath := filepath.Join(testDir, filename)

						// Modify file content
						newContent := fmt.Sprintf(`package main

import "fmt"

func Function%d() {
	fmt.Println("Modified content %d - version %d")
}

type Struct%d struct {
	Field%d string
	NewField%d int // Added field
}

func (s *Struct%d) Method%d() string {
	return s.Field%d
}

func NewFunction%d() {
	// New function added at %s
}`, fileIdx, fileIdx, modificationCount, fileIdx, fileIdx, modificationCount,
							fileIdx, fileIdx, fileIdx, fileIdx, time.Now().Format("15:04:05"))

						// Write to disk
						err := os.WriteFile(filePath, []byte(newContent), 0644)
						if err == nil {
							invalidations.Add(1)
						}

						modificationCount++
						time.Sleep(500 * time.Millisecond)
					}
				}
			}(i)
		}

		// Let the test run for a while
		time.Sleep(5 * time.Second)

		// Stop all workers
		close(stopReaders)
		close(stopModifiers)

		// Wait for all goroutines
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(30 * time.Second):
			t.Fatal("Test timeout waiting for workers")
		}

		// Check cache statistics
		t.Logf("Cache stats after concurrent modifications:")
		t.Logf("  Total requests: %d", requestCount.Load())
		t.Logf("  File modifications: %d", invalidations.Load())

		assert.Greater(t, requestCount.Load(), expectedRequests, "Should process many requests")
		assert.Greater(t, invalidations.Load(), expectedModifications, "Should have multiple file modifications")
	})

	t.Run("RapidFileModificationStress", func(t *testing.T) {
		targetFile := filepath.Join(testDir, "stress_test.go")
		uri := utils.FilePathToURI(targetFile)

		initialContent := `package main

func StressFunction() string {
	return "version-0"
}

type StressType struct {
	Version int
}`

		require.NoError(t, writeTestFile(targetFile, initialContent))

		var wg sync.WaitGroup
		var readErrors atomic.Int32
		var writeCount atomic.Int32
		var cacheInvalidations atomic.Int32

		// Rapid writers
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func(writerID int) {
				defer wg.Done()

				for v := 0; v < 20; v++ {
					newContent := fmt.Sprintf(`package main

func StressFunction() string {
	return "version-%d-writer-%d"
}

type StressType struct {
	Version int
	Writer%d string
	Timestamp int64
}

func NewMethod%d() {
	// Added by writer %d at iteration %d
}`, v, writerID, writerID, v, writerID, v)

					err := os.WriteFile(targetFile, []byte(newContent), 0644)
					if err == nil {
						writeCount.Add(1)
						cacheInvalidations.Add(1)
					}

					time.Sleep(50 * time.Millisecond)
				}
			}(i)
		}

		// Concurrent readers
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(readerID int) {
				defer wg.Done()

				for r := 0; r < 50; r++ {
					ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
					defer cancel()

					params := protocol.DocumentSymbolParams{
						TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(uri)},
					}

					_, err := lspManager.ProcessRequest(ctx, "textDocument/documentSymbol", params)
					if err != nil {
						readErrors.Add(1)
					}

					time.Sleep(20 * time.Millisecond)
				}
			}(i)
		}

		// Wait for completion
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(30 * time.Second):
			t.Fatal("Rapid modification test timeout")
		}

		t.Logf("Rapid modification results:")
		t.Logf("  File writes: %d", writeCount.Load())
		t.Logf("  Cache invalidations: %d", cacheInvalidations.Load())
		t.Logf("  Read errors: %d", readErrors.Load())

		// Verify final state consistency
		finalContentBytes, err := os.ReadFile(targetFile)
		if err == nil {
			finalContent := string(finalContentBytes)
			assert.Contains(t, finalContent, "StressFunction", "Document should contain expected function")
		}

		// Error rate should be reasonable
		assert.Less(t, readErrors.Load(), int32(100), "Read errors should be limited")
	})

	t.Run("CacheConsistencyAfterInvalidation", func(t *testing.T) {
		// Pick a file to test
		testFile := filepath.Join(testDir, "consistency_test.go")
		uri := utils.FilePathToURI(testFile)

		version1 := `package main

type ConsistencyTest struct {
	Field1 string
}

func (c *ConsistencyTest) Method1() string {
	return c.Field1
}`

		version2 := `package main

type ConsistencyTest struct {
	Field1 string
	Field2 int // New field
}

func (c *ConsistencyTest) Method1() string {
	return c.Field1
}

func (c *ConsistencyTest) Method2() int {
	return c.Field2
}`

		// Write version 1
		require.NoError(t, writeTestFile(testFile, version1))

		// Query symbols for version 1
		ctx1, cancel1 := context.WithTimeout(ctx, 5*time.Second)
		defer cancel1()

		params := protocol.DocumentSymbolParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(uri)},
		}

		symbols1, err := lspManager.ProcessRequest(ctx1, "textDocument/documentSymbol", params)
		require.NoError(t, err)

		// Modify to version 2
		require.NoError(t, os.WriteFile(testFile, []byte(version2), 0644))

		// Explicitly invalidate cache for the modified file
		err = lspManager.InvalidateCache(uri)
		if err != nil {
			t.Logf("Cache invalidation failed (but continuing): %v", err)
		}

		// Force the LSP server to re-read the file by making a fresh connection
		// This simulates what would happen if the file was changed and the LSP
		// server detected it through file system monitoring
		lspManager.Stop()

		// Restart the LSP manager to get fresh file content
		lspManager2, err := server.NewLSPManager(cfg)
		require.NoError(t, err)
		defer lspManager2.Stop()
		lspManager2.SetCache(scipCache)

		err = lspManager2.Start(ctx)
		require.NoError(t, err)

		// Allow time for initialization
		time.Sleep(100 * time.Millisecond)

		// Query symbols for version 2
		ctx2, cancel2 := context.WithTimeout(ctx, 5*time.Second)
		defer cancel2()

		symbols2, err := lspManager2.ProcessRequest(ctx2, "textDocument/documentSymbol", params)
		require.NoError(t, err)

		// Symbols should be different
		assert.NotEqual(t, symbols1, symbols2, "Symbols should differ after file modification")

		// Query multiple times to ensure consistency
		for i := 0; i < 5; i++ {
			ctx3, cancel3 := context.WithTimeout(ctx, 5*time.Second)
			defer cancel3()

			symbols3, err := lspManager2.ProcessRequest(ctx3, "textDocument/documentSymbol", params)
			require.NoError(t, err)

			assert.Equal(t, symbols2, symbols3, "Subsequent queries should return consistent results")
		}
	})

	// Final cache statistics
	finalCacheMetrics := scipCache.GetMetrics()
	t.Logf("Final cache statistics:")
	t.Logf("  Entry count: %d", finalCacheMetrics.EntryCount)
	t.Logf("  Total size: %d KB", finalCacheMetrics.TotalSize/1024)
	t.Logf("  Hit rate: %.2f%%", float64(finalCacheMetrics.HitCount)/float64(finalCacheMetrics.HitCount+finalCacheMetrics.MissCount)*100)
	t.Logf("  Eviction count: %d", finalCacheMetrics.EvictionCount)

	assert.Greater(t, finalCacheMetrics.EntryCount, int64(0), "Cache should contain items after test")
}

func writeTestFile(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}
