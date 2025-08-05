package cli

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/project"
	"lsp-gateway/src/server"
	"lsp-gateway/src/server/cache"
)

// IndexCache rebuilds the cache index by processing workspace files
func IndexCache(configPath string) error {
	cfg := LoadConfigWithFallback(configPath)

	// Create LSP manager to access cache
	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LSP manager: %w", err)
	}

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

	common.CLILogger.Info("🔄 Cache Index")
	common.CLILogger.Info("%s", strings.Repeat("=", 50))
	common.CLILogger.Info("📋 Starting cache index rebuild...")

	// Get working directory
	wd, err := os.Getwd()
	if err != nil {
		common.CLILogger.Error("❌ Failed to get working directory: %v", err)
		return err
	}
	common.CLILogger.Info("🔍 Scanning workspace: %s", wd)

	// Detect languages in the workspace
	detectedLanguages, err := project.DetectLanguages(wd)
	if err != nil {
		common.CLILogger.Warn("⚠️  Failed to detect languages: %v", err)
		detectedLanguages = []string{}
	}

	// Get workspace files for indexing based on detected languages
	workspaceFiles, err := scanWorkspaceFiles(wd, detectedLanguages)
	if err != nil {
		common.CLILogger.Error("❌ Failed to scan workspace files: %v", err)
		return err
	}

	common.CLILogger.Info("📁 Found %d files to index", len(workspaceFiles))
	for _, lang := range detectedLanguages {
		common.CLILogger.Info("  • Language: %s", lang)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Update the index with workspace files
	if indexer, ok := cache.(interface {
		UpdateIndex(context.Context, []string) error
	}); ok {
		if err := indexer.UpdateIndex(ctx, workspaceFiles); err != nil {
			common.CLILogger.Error("❌ Failed to update index: %v", err)
			return err
		}
	} else {
		common.CLILogger.Info("⚠️  Cache does not support indexing operations")
		return nil
	}

	common.CLILogger.Info("✅ Cache index rebuilt successfully")
	
	// Show updated stats
	if updatedMetrics, err := cache.HealthCheck(); err == nil && updatedMetrics != nil {
		common.CLILogger.Info("📊 Updated cache stats: %d entries", updatedMetrics.EntryCount)
	}

	return nil
}

// scanWorkspaceFiles scans the workspace for files to index based on detected languages
func scanWorkspaceFiles(workingDir string, languages []string) ([]string, error) {
	var files []string
	extensions := getExtensionsForLanguages(languages)

	if len(extensions) == 0 {
		// If no languages detected, scan for all supported language files
		extensions = []string{".go", ".py", ".js", ".ts", ".jsx", ".tsx", ".java"}
	}

	// Walk through directory tree (up to 3 levels deep for performance)
	err := filepath.WalkDir(workingDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors and continue
		}

		// Skip hidden directories and common non-source directories
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || 
			   name == "build" || name == "dist" || name == "target" || name == "__pycache__" {
				return fs.SkipDir
			}

			// Limit depth to 3 levels
			relPath, _ := filepath.Rel(workingDir, path)
			depth := strings.Count(relPath, string(filepath.Separator))
			if depth > 3 {
				return fs.SkipDir
			}
			return nil
		}

		// Check if file has a relevant extension
		ext := filepath.Ext(path)
		for _, validExt := range extensions {
			if ext == validExt {
				files = append(files, path)
				break
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return files, nil
}

// getExtensionsForLanguages returns file extensions for the given languages
func getExtensionsForLanguages(languages []string) []string {
	extMap := map[string][]string{
		"go":         {".go"},
		"python":     {".py"},
		"javascript": {".js", ".jsx", ".mjs"},
		"typescript": {".ts", ".tsx"},
		"java":       {".java"},
	}

	var extensions []string
	seen := make(map[string]bool)

	for _, lang := range languages {
		if exts, ok := extMap[lang]; ok {
			for _, ext := range exts {
				if !seen[ext] {
					extensions = append(extensions, ext)
					seen[ext] = true
				}
			}
		}
	}

	return extensions
}

// ClearCache clears all cache entries
func ClearCache(configPath string) error {
	cfg := LoadConfigWithFallback(configPath)

	// Create LSP manager to access cache
	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LSP manager: %w", err)
	}

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

	common.CLILogger.Info("🧹 Cache Clear")
	common.CLILogger.Info("%s", strings.Repeat("=", 50))

	// Clear the cache
	if err := cache.Clear(); err != nil {
		common.CLILogger.Error("❌ Failed to clear cache: %v", err)
		return err
	}

	common.CLILogger.Info("✅ Cache cleared successfully")

	return nil
}

// ShowCacheInfo displays brief statistics about cached data
func ShowCacheInfo(configPath string) error {
	cfg := LoadConfigWithFallback(configPath)

	// Create LSP manager to access cache
	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LSP manager: %w", err)
	}

	cacheInstance := manager.GetCache()
	if cacheInstance == nil {
		common.CLILogger.Info("❌ Cache: Not available")
		return nil
	}

	metrics, err := cacheInstance.HealthCheck()
	if err != nil {
		common.CLILogger.Error("❌ Cache: Unable to get status (%v)", err)
		return err
	}

	common.CLILogger.Info("📊 Cache Info")
	common.CLILogger.Info("%s", strings.Repeat("=", 50))

	if metrics == nil {
		common.CLILogger.Info("❌ No cache metrics available")
		return nil
	}

	// Display cache status
	if metrics.HealthStatus == "DISABLED" {
		common.CLILogger.Info("⚫ Status: Disabled")
		return nil
	}

	// Basic statistics
	common.CLILogger.Info("📈 Cache Statistics:")
	common.CLILogger.Info("  • Entries: %d", metrics.EntryCount)
	common.CLILogger.Info("  • Memory: %s", formatBytes(metrics.TotalSize))
	
	// Hit/Miss ratio
	totalRequests := metrics.HitCount + metrics.MissCount
	if totalRequests > 0 {
		hitRatio := float64(metrics.HitCount) / float64(totalRequests) * 100
		common.CLILogger.Info("  • Hit Rate: %.1f%% (%d/%d)", hitRatio, metrics.HitCount, totalRequests)
	} else {
		common.CLILogger.Info("  • Hit Rate: No requests yet")
	}

	// Evictions if any
	if metrics.EvictionCount > 0 {
		common.CLILogger.Info("  • Evictions: %d", metrics.EvictionCount)
	}

	// Get and display index statistics
	if simpleCache, ok := cacheInstance.(interface{ GetIndexStats() *cache.IndexStats }); ok {
		indexStats := simpleCache.GetIndexStats()
		if indexStats != nil && indexStats.Status != "disabled" {
			common.CLILogger.Info("")
			common.CLILogger.Info("📑 Index Statistics:")
			common.CLILogger.Info("  • Indexed Documents: %d", indexStats.DocumentCount)
			common.CLILogger.Info("  • Indexed Symbols: %d", indexStats.SymbolCount)
			
			if indexStats.IndexSize > 0 {
				common.CLILogger.Info("  • Index Size: %s", formatBytes(indexStats.IndexSize))
			}
			
			// Display per-language statistics if available
			if len(indexStats.LanguageStats) > 0 {
				common.CLILogger.Info("  • Languages:")
				for lang, count := range indexStats.LanguageStats {
					common.CLILogger.Info("    - %s: %d symbols", lang, count)
				}
			}
			
			// Display last update time if available
			if !indexStats.LastUpdate.IsZero() {
				timeSinceUpdate := time.Since(indexStats.LastUpdate)
				if timeSinceUpdate < time.Minute {
					common.CLILogger.Info("  • Last Updated: %d seconds ago", int(timeSinceUpdate.Seconds()))
				} else if timeSinceUpdate < time.Hour {
					common.CLILogger.Info("  • Last Updated: %d minutes ago", int(timeSinceUpdate.Minutes()))
				} else {
					common.CLILogger.Info("  • Last Updated: %d hours ago", int(timeSinceUpdate.Hours()))
				}
			}
		}
	}

	return nil
}