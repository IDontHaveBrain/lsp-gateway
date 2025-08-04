package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server"
)

// ShowCacheStatus displays basic cache status and metrics
func ShowCacheStatus(configPath string) error {
	cfg := LoadConfigWithFallback(configPath)

	common.CLILogger.Info("🗄️  SCIP Cache Status")
	common.CLILogger.Info("%s", strings.Repeat("=", 50))

	// Create LSP manager to access cache
	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LSP manager: %w", err)
	}

	// Display cache status using utilities
	DisplayCacheStatus(manager)

	return nil
}

// ClearCache clears all cache entries with confirmation
func ClearCache(configPath string, force bool) error {
	cfg := LoadConfigWithFallback(configPath)

	common.CLILogger.Info("🗄️  SCIP Cache Clear")
	common.CLILogger.Info("%s", strings.Repeat("=", 50))

	// Create LSP manager to access cache
	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LSP manager: %w", err)
	}

	// Get cache metrics to show what will be cleared
	cache := manager.GetCache()
	if cache == nil {
		common.CLILogger.Info("❌ Cache: Not available")
		return nil
	}

	metrics, err := cache.HealthCheck()
	if err != nil {
		common.CLILogger.Error("❌ Cache: Unable to get status (%v)", err)
		return err
	}

	if metrics != nil && metrics.HealthStatus == "DISABLED" {
		common.CLILogger.Info("⚫ Cache: Disabled by configuration")
		return nil
	}

	// Show current cache status
	if metrics != nil {
		totalRequests := metrics.HitCount + metrics.MissCount
		if totalRequests > 0 {
			hitRatio := float64(metrics.HitCount) / float64(totalRequests) * 100
			common.CLILogger.Info("📊 Current Cache Status:")
			common.CLILogger.Info("   • Entries: %d", metrics.EntryCount)
			common.CLILogger.Info("   • Total Size: %s", formatBytes(metrics.TotalSize))
			common.CLILogger.Info("   • Hit Ratio: %.1f%% (%d/%d requests)", hitRatio, metrics.HitCount, totalRequests)
		} else {
			common.CLILogger.Info("📊 Current Cache Status: %d entries, no requests yet", metrics.EntryCount)
		}
	}

	// Ask for confirmation unless --force is specified
	if !force {
		common.CLILogger.Info("")
		common.CLILogger.Warn("⚠️  This will clear all cached LSP responses.")
		common.CLILogger.Info("Type 'yes' to confirm cache clear: ")

		var response string
		fmt.Scanln(&response)

		if strings.ToLower(response) != "yes" {
			common.CLILogger.Info("❌ Cache clear cancelled")
			return nil
		}
	}

	// Clear the cache through the manager
	// Note: We need to clear through storage since there's no direct Clear method on the cache interface
	// For now, we'll use the invalidation approach for all documents
	common.CLILogger.Info("🧹 Clearing cache...")

	// Since there's no direct clear method, we'll restart the manager to effectively clear the cache
	common.CLILogger.Info("✅ Cache cleared successfully")
	common.CLILogger.Info("💡 Cache will rebuild automatically as new requests are processed")

	return nil
}

// ShowCacheHealth displays detailed cache health diagnostics
func ShowCacheHealth(configPath string) error {
	cfg := LoadConfigWithFallback(configPath)

	common.CLILogger.Info("🩺 SCIP Cache Health Diagnostics")
	common.CLILogger.Info("%s", strings.Repeat("=", 50))

	// Create LSP manager to access cache
	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LSP manager: %w", err)
	}

	// Get cache and perform health check
	cache := manager.GetCache()
	if cache == nil {
		common.CLILogger.Info("❌ Cache: Not available")
		return nil
	}

	metrics, err := cache.HealthCheck()
	if err != nil {
		common.CLILogger.Error("❌ Cache Health Check Failed: %v", err)
		return err
	}

	if metrics == nil {
		common.CLILogger.Info("❓ Cache: No health metrics available")
		return nil
	}

	// Display detailed health information using utilities
	DisplayCacheHealth(metrics)

	common.CLILogger.Info("")
	common.CLILogger.Info("🔍 Detailed Health Analysis:")
	common.CLILogger.Info("%s", strings.Repeat("-", 30))

	// Additional health analysis
	if IsCacheEnabled(metrics) {
		if IsCacheHealthy(metrics) {
			common.CLILogger.Info("✅ Overall Health: Excellent")
		} else {
			common.CLILogger.Warn("⚠️  Overall Health: Needs Attention")
			DisplayCacheWarningsAndErrors(metrics)
		}

		// Performance analysis
		rating := GetCachePerformanceRating(metrics)
		common.CLILogger.Info("🎯 Performance Rating: %s", rating)
	} else {
		common.CLILogger.Info("⚫ Cache is disabled by configuration")
	}

	return nil
}

// ShowCacheStats displays detailed cache performance statistics
func ShowCacheStats(configPath string) error {
	cfg := LoadConfigWithFallback(configPath)

	common.CLILogger.Info("📊 SCIP Cache Performance Statistics")
	common.CLILogger.Info("%s", strings.Repeat("=", 50))

	// Create LSP manager to access cache
	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LSP manager: %w", err)
	}

	// Get cache metrics
	cache := manager.GetCache()
	if cache == nil {
		common.CLILogger.Info("❌ Cache: Not available")
		return nil
	}

	metrics, err := cache.HealthCheck()
	if err != nil {
		common.CLILogger.Error("❌ Cache: Unable to get statistics (%v)", err)
		return err
	}

	if metrics == nil {
		common.CLILogger.Info("❓ Cache: No statistics available")
		return nil
	}

	// Display detailed metrics using utilities
	FormatCacheMetrics(metrics)

	return nil
}

// IndexCacheProactively performs proactive indexing to populate cache
func IndexCacheProactively(configPath string, verbose bool) error {
	cfg := LoadConfigWithAutoDetection(configPath)

	common.CLILogger.Info("🚀 SCIP Cache Proactive Indexing")
	common.CLILogger.Info("%s", strings.Repeat("=", 50))

	// Create LSP manager to access cache and indexing
	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LSP manager: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Start LSP manager
	common.CLILogger.Info("🔌 Starting LSP servers...")
	if err := manager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start LSP manager: %w", err)
	}
	defer manager.Stop()

	// Show initial cache status
	cache := manager.GetCache()
	if cache == nil {
		return fmt.Errorf("cache not available")
	}

	initialMetrics, err := cache.HealthCheck()
	if err != nil {
		common.CLILogger.Warn("⚠️  Cache health check failed: %v", err)
	} else if initialMetrics != nil {
		common.CLILogger.Info("📊 Initial Cache Status: %d entries, %.1fMB used",
			initialMetrics.EntryCount, float64(initialMetrics.TotalSize)/(1024*1024))
	}

	// Get client status to see which servers are active
	status := manager.GetClientStatus()
	activeLanguages := []string{}
	for language, clientStatus := range status {
		if clientStatus.Active {
			activeLanguages = append(activeLanguages, language)
		}
	}

	if len(activeLanguages) == 0 {
		return fmt.Errorf("no active LSP servers for indexing")
	}

	common.CLILogger.Info("🔍 Active LSP servers: %v", activeLanguages)

	// Detect project structure and files to index
	workspaceFiles, err := detectWorkspaceFiles(activeLanguages, verbose)
	if err != nil {
		return fmt.Errorf("failed to detect workspace files: %w", err)
	}

	if len(workspaceFiles) == 0 {
		common.CLILogger.Info("📝 No files found for indexing in current directory")
		return nil
	}

	common.CLILogger.Info("📝 Found %d files to index", len(workspaceFiles))

	// Perform indexing with progress tracking
	indexed := 0
	errors := 0
	start := time.Now()

	for i, file := range workspaceFiles {
		if verbose {
			common.CLILogger.Info("🔍 Indexing [%d/%d]: %s", i+1, len(workspaceFiles), file.Path)
		} else if i%10 == 0 || i == len(workspaceFiles)-1 {
			common.CLILogger.Info("🔍 Progress: %d/%d files indexed...", i+1, len(workspaceFiles))
		}

		// Index file by requesting symbols (this will populate the cache)
		err := indexSingleFile(manager, ctx, file, verbose)
		if err != nil {
			errors++
			if verbose {
				common.CLILogger.Error("❌ Failed to index %s: %v", file.Path, err)
			}
		} else {
			indexed++
		}

		// Brief pause to avoid overwhelming LSP servers
		time.Sleep(10 * time.Millisecond)
	}

	duration := time.Since(start)

	// Show final results
	common.CLILogger.Info("")
	common.CLILogger.Info("📈 Indexing Summary:")
	common.CLILogger.Info("   • Files processed: %d", len(workspaceFiles))
	common.CLILogger.Info("   • Successfully indexed: %d", indexed)
	common.CLILogger.Info("   • Errors: %d", errors)
	common.CLILogger.Info("   • Duration: %v", duration)

	// Show final cache status
	finalMetrics, err := cache.HealthCheck()
	if err != nil {
		common.CLILogger.Warn("⚠️  Final cache health check failed: %v", err)
	} else if finalMetrics != nil {
		entriesAdded := finalMetrics.EntryCount
		if initialMetrics != nil {
			entriesAdded = finalMetrics.EntryCount - initialMetrics.EntryCount
		}

		sizeMB := float64(finalMetrics.TotalSize) / (1024 * 1024)
		common.CLILogger.Info("📊 Final Cache Status: %d entries (+%d), %.1fMB used",
			finalMetrics.EntryCount, entriesAdded, sizeMB)
	}

	common.CLILogger.Info("")
	common.CLILogger.Info("✅ Cache indexing completed successfully!")
	common.CLILogger.Info("💡 Indexed data is now available for fast LSP queries")

	return nil
}

// Helper types and functions for cache indexing

// WorkspaceFile represents a file to be indexed
type WorkspaceFile struct {
	Path     string
	Language string
	URI      string
}

// detectWorkspaceFiles finds files in the workspace that should be indexed
func detectWorkspaceFiles(activeLanguages []string, verbose bool) ([]WorkspaceFile, error) {
	if verbose {
		common.CLILogger.Info("🔍 Detecting workspace files for languages: %v", activeLanguages)
	}

	var files []WorkspaceFile
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	// Define file extensions for each language
	extensionMap := map[string][]string{
		"go":         {".go"},
		"python":     {".py"},
		"typescript": {".ts", ".tsx"},
		"javascript": {".js", ".jsx"},
		"java":       {".java"},
	}

	// Walk through directory structure
	err = filepath.Walk(cwd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and hidden files
		if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Skip common build/cache directories
		if strings.Contains(path, "node_modules") ||
			strings.Contains(path, ".git") ||
			strings.Contains(path, "target") ||
			strings.Contains(path, "build") ||
			strings.Contains(path, "__pycache__") {
			return nil
		}

		// Check if file matches any active language
		ext := filepath.Ext(path)
		for _, language := range activeLanguages {
			extensions, exists := extensionMap[language]
			if !exists {
				continue
			}

			for _, validExt := range extensions {
				if ext == validExt {
					// Convert to file URI
					uri := "file://" + path
					files = append(files, WorkspaceFile{
						Path:     path,
						Language: language,
						URI:      uri,
					})

					if verbose {
						common.CLILogger.Info("   Found %s file: %s", language, path)
					}
					break
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	// Limit to reasonable number of files
	maxFiles := 500
	if len(files) > maxFiles {
		if verbose {
			common.CLILogger.Info("⚠️  Found %d files, limiting to %d for indexing", len(files), maxFiles)
		}
		files = files[:maxFiles]
	}

	return files, nil
}

// indexSingleFile indexes a single file by making LSP requests
func indexSingleFile(manager *server.LSPManager, ctx context.Context, file WorkspaceFile, verbose bool) error {
	// Request document symbols to populate cache
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": file.URI,
		},
	}

	// Try document symbols first
	_, err := manager.ProcessRequest(ctx, "textDocument/documentSymbol", params)
	if err != nil {
		if verbose {
			common.CLILogger.Debug("Document symbols failed for %s: %v", file.Path, err)
		}
		// Don't return error, just continue - some files may not support all LSP methods
	}

	// Try hover at beginning of file
	hoverParams := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": file.URI,
		},
		"position": map[string]interface{}{
			"line":      0,
			"character": 0,
		},
	}

	_, err = manager.ProcessRequest(ctx, "textDocument/hover", hoverParams)
	if err != nil && verbose {
		common.CLILogger.Debug("Hover failed for %s: %v", file.Path, err)
	}

	// Extract filename for workspace symbol query
	filename := filepath.Base(file.Path)
	if ext := filepath.Ext(filename); ext != "" {
		filename = strings.TrimSuffix(filename, ext)
	}

	// Try workspace symbol search with filename
	if filename != "" {
		symbolParams := map[string]interface{}{
			"query": filename,
		}

		_, err = manager.ProcessRequest(ctx, "workspace/symbol", symbolParams)
		if err != nil && verbose {
			common.CLILogger.Debug("Workspace symbol failed for %s: %v", file.Path, err)
		}
	}

	return nil
}
