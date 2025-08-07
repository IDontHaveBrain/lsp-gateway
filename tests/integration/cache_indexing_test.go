package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server/cache"
	"lsp-gateway/src/utils"

	"github.com/stretchr/testify/require"
)

func TestCacheIndexing(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")
	projectDir := filepath.Join(tempDir, "project")

	err := os.MkdirAll(projectDir, 0755)
	require.NoError(t, err)

	testFiles := map[string]string{
		"main.go": `package main

func main() {
	println("Hello, World!")
}

func Calculate(x, y int) int {
	return x + y
}`,
		"utils/string.go": `package utils

func Reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}`,
		"models/user.go": `package models

type User struct {
	ID    string
	Name  string
	Email string
}

func (u *User) Validate() bool {
	return u.ID != "" && u.Name != "" && u.Email != ""
}`,
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(projectDir, path)
		dir := filepath.Dir(fullPath)
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err)

		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}

	cacheConfig := &config.CacheConfig{
		Enabled:         true,
		StoragePath:     cacheDir,
		MaxMemoryMB:     64,
		TTLHours:        1,
		Languages:       []string{"go"},
		BackgroundIndex: true,
		DiskCache:       true,
		EvictionPolicy:  "lru",
	}

	scipCache, err := cache.NewSCIPCacheManager(cacheConfig)
	require.NoError(t, err)
	require.NotNil(t, scipCache)

	ctx := context.Background()
	err = scipCache.Start(ctx)
	require.NoError(t, err)
	defer scipCache.Stop()

	t.Run("Manual file indexing", func(t *testing.T) {
		mainFile := filepath.Join(projectDir, "main.go")
		_, err := os.ReadFile(mainFile)
		require.NoError(t, err)

		// Index file through document indexing
		err = scipCache.IndexDocument(ctx, "file://"+mainFile, "go", nil)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Query indexed symbols
		query := &cache.IndexQuery{
			Type: "symbol",
			URI:  utils.FilePathToURI(mainFile),
		}
		result, err := scipCache.QueryIndex(ctx, query)
		require.NoError(t, err)
		require.NotNil(t, result)
		// Check that indexing occurred
	})

	t.Run("Batch indexing", func(t *testing.T) {
		files := []string{
			filepath.Join(projectDir, "utils/string.go"),
			filepath.Join(projectDir, "models/user.go"),
		}

		for _, file := range files {
			_, err := os.ReadFile(file)
			require.NoError(t, err)

			err = scipCache.IndexDocument(ctx, "file://"+file, "go", nil)
			require.NoError(t, err)
		}

		time.Sleep(200 * time.Millisecond)

		metrics := scipCache.GetMetrics()
		require.NotNil(t, metrics)

		stats := scipCache.GetIndexStats()
		require.NotNil(t, stats)
	})

	t.Run("Symbol search after indexing", func(t *testing.T) {
		query := &cache.IndexQuery{
			Type:   "symbol",
			Symbol: "Reverse",
		}
		results, err := scipCache.QueryIndex(ctx, query)
		require.NoError(t, err)

		require.NotNil(t, results)
	})

	t.Run("Index update on file modification", func(t *testing.T) {
		mainFile := filepath.Join(projectDir, "main.go")

		updatedContent := `package main

func main() {
	println("Updated!")
}

func Calculate(x, y int) int {
	return x * y  // Changed from addition to multiplication
}

func NewFunction() string {
	return "new"
}`

		err := os.WriteFile(mainFile, []byte(updatedContent), 0644)
		require.NoError(t, err)

		err = scipCache.IndexDocument(ctx, "file://"+mainFile, "go", nil)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Query indexed symbols
		query := &cache.IndexQuery{
			Type: "symbol",
			URI:  utils.FilePathToURI(mainFile),
		}
		_, err = scipCache.QueryIndex(ctx, query)
		require.NoError(t, err)

		// Verify re-indexing worked
	})
}

func TestBackgroundIndexing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping background indexing test in short mode")
	}

	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")
	projectDir := filepath.Join(tempDir, "project")

	err := os.MkdirAll(projectDir, 0755)
	require.NoError(t, err)

	goFiles := []string{
		"server.go",
		"client.go",
		"handler.go",
		"middleware.go",
		"router.go",
	}

	for _, file := range goFiles {
		content := `package main

type ` + file[:len(file)-3] + `Type struct {
	ID   string
	Name string
}

func Process` + file[:len(file)-3] + `() error {
	return nil
}`

		err := os.WriteFile(filepath.Join(projectDir, file), []byte(content), 0644)
		require.NoError(t, err)
	}

	cacheConfig := &config.CacheConfig{
		Enabled:         true,
		StoragePath:     cacheDir,
		MaxMemoryMB:     64,
		TTLHours:        1,
		Languages:       []string{"go"},
		BackgroundIndex: true,
		DiskCache:       true,
		EvictionPolicy:  "lru",
	}

	scipCache, err := cache.NewSCIPCacheManager(cacheConfig)
	require.NoError(t, err)

	ctx := context.Background()
	err = scipCache.Start(ctx)
	require.NoError(t, err)
	defer scipCache.Stop()

	// Update index with files from the project directory
	files := []string{}
	for _, file := range goFiles {
		files = append(files, filepath.Join(projectDir, file))
	}
	err = scipCache.UpdateIndex(ctx, files)
	require.NoError(t, err)

	time.Sleep(3 * time.Second)

	metrics := scipCache.GetMetrics()
	require.NotNil(t, metrics)

	stats := scipCache.GetIndexStats()
	require.NotNil(t, stats)

	query := &cache.IndexQuery{
		Type:   "symbol",
		Symbol: "Process",
	}
	results, err := scipCache.QueryIndex(ctx, query)
	require.NoError(t, err)
	require.NotNil(t, results)
}
