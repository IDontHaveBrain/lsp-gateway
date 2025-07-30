package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSubProjectResolver_Integration tests the complete resolver system integration
func TestSubProjectResolver_Integration(t *testing.T) {
	t.Parallel()
	// Create temporary workspace
	tempDir := createTempWorkspace(t)
	defer os.RemoveAll(tempDir)

	// Create resolver with test options
	options := &ResolverOptions{
		CacheCapacity:   100,
		CacheTTL:        5 * time.Minute,
		MaxDepth:        5,
		RefreshInterval: 0, // Disable background refresh for tests
	}

	resolver := NewSubProjectResolver(tempDir, options)
	defer resolver.Close()

	// Refresh projects to discover test structure
	ctx := context.Background()
	err := resolver.RefreshProjects(ctx)
	if err != nil {
		t.Fatalf("Failed to refresh projects: %v", err)
	}

	// Test file URI resolution
	testCases := []struct {
		fileURI         string
		expectedProject string
		description     string
	}{
		{
			fileURI:         filepath.Join(tempDir, "backend/main.go"),
			expectedProject: "backend",
			description:     "Go backend project file",
		},
		{
			fileURI:         filepath.Join(tempDir, "frontend/src/index.js"),
			expectedProject: "frontend",
			description:     "Frontend source file",
		},
		{
			fileURI:         filepath.Join(tempDir, "shared/utils.py"),
			expectedProject: "shared",
			description:     "Shared Python utility",
		},
		{
			fileURI:         filepath.Join(tempDir, "nested/service/app.java"),
			expectedProject: "service",
			description:     "Nested Java service",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// Test both cached and non-cached resolution
			project, err := resolver.ResolveSubProject(tc.fileURI)
			if err != nil {
				t.Errorf("Resolution failed: %v", err)
				return
			}

			if project == nil {
				t.Error("Expected project but got nil")
				return
			}

			if !strings.Contains(project.Name, tc.expectedProject) &&
				!strings.Contains(project.Root, tc.expectedProject) {
				t.Errorf("Expected project %s, got %s (root: %s)",
					tc.expectedProject, project.Name, project.Root)
			}

			// Test cached resolution
			cachedProject, err := resolver.ResolveSubProjectCached(tc.fileURI)
			if err != nil {
				t.Errorf("Cached resolution failed: %v", err)
				return
			}

			if cachedProject != project {
				t.Error("Cached resolution returned different project")
			}
		})
	}

	// Test metrics
	metrics := resolver.GetMetrics()
	if metrics.TotalResolutions == 0 {
		t.Error("Expected resolution metrics to be updated")
	}

	// Test cache stats
	cacheStats := resolver.GetCacheStats()
	if cacheStats.Size == 0 {
		t.Error("Expected cache to contain entries")
	}
}

// TestPathTrie_Operations tests the path trie data structure operations
func TestPathTrie_Operations(t *testing.T) {
	t.Parallel()
	trie := NewPathTrie()

	// Create test projects
	projects := []*SubProject{
		{ID: "root", Root: "/", Name: "root"},
		{ID: "app", Root: "/app", Name: "app"},
		{ID: "backend", Root: "/app/backend", Name: "backend"},
		{ID: "frontend", Root: "/app/frontend", Name: "frontend"},
		{ID: "shared", Root: "/shared", Name: "shared"},
	}

	// Insert projects
	for _, project := range projects {
		trie.Insert(project.Root, project)
	}

	// Test basic lookup
	testCases := []struct {
		path     string
		expected string
	}{
		{"/app/backend/main.go", "backend"},
		{"/app/frontend/src/index.js", "frontend"},
		{"/app/config.yml", "app"},
		{"/shared/utils.py", "shared"},
		{"/other/file.txt", "root"},
		{"/app/backend/src/deep/file.java", "backend"},
	}

	for _, tc := range testCases {
		t.Run("lookup_"+tc.path, func(t *testing.T) {
			result := trie.FindLongestMatch(tc.path)
			if result == nil {
				t.Errorf("Expected project for path %s", tc.path)
				return
			}

			if result.Name != tc.expected {
				t.Errorf("Expected %s, got %s for path %s",
					tc.expected, result.Name, tc.path)
			}
		})
	}

	// Test Contains
	if !trie.Contains("/app/backend") {
		t.Error("Expected trie to contain /app/backend")
	}

	if trie.Contains("/nonexistent") {
		t.Error("Expected trie not to contain /nonexistent")
	}

	// Test Size
	if trie.Size() != len(projects) {
		t.Errorf("Expected size %d, got %d", len(projects), trie.Size())
	}

	// Test GetAllProjects
	allProjects := trie.GetAllProjects()
	if len(allProjects) != len(projects) {
		t.Errorf("Expected %d projects, got %d", len(projects), len(allProjects))
	}

	// Test Remove
	if !trie.Remove("/app/backend") {
		t.Error("Failed to remove /app/backend")
	}

	if trie.Contains("/app/backend") {
		t.Error("Project should have been removed")
	}

	// Test Clear
	trie.Clear()
	if trie.Size() != 0 {
		t.Error("Expected empty trie after clear")
	}
}

// TestResolverCache_LRU tests the LRU cache functionality
func TestResolverCache_LRU(t *testing.T) {
	t.Parallel()
	cache := NewResolverCache(3, 5*time.Minute) // Small capacity for testing
	defer cache.Close()

	// Create test projects
	projects := []*SubProject{
		{ID: "p1", Name: "project1"},
		{ID: "p2", Name: "project2"},
		{ID: "p3", Name: "project3"},
		{ID: "p4", Name: "project4"},
	}

	// Fill cache to capacity
	cache.Put("/path1", projects[0])
	cache.Put("/path2", projects[1])
	cache.Put("/path3", projects[2])

	// Verify all entries exist
	if cache.Get("/path1") != projects[0] {
		t.Error("Expected project1 in cache")
	}

	// Add one more to trigger eviction
	cache.Put("/path4", projects[3])

	// path1 should be evicted (LRU)
	if cache.Get("/path1") != nil {
		t.Error("Expected path1 to be evicted")
	}

	// Others should still exist
	if cache.Get("/path2") != projects[1] {
		t.Error("Expected project2 in cache")
	}

	// Test cache statistics
	stats := cache.GetStats()
	if stats.Size != 3 {
		t.Errorf("Expected cache size 3, got %d", stats.Size)
	}

	if stats.HitCount == 0 {
		t.Error("Expected cache hits to be recorded")
	}
}

// TestResolverCache_TTL tests cache TTL expiration
func TestResolverCache_TTL(t *testing.T) {
	t.Parallel()
	shortTTL := 50 * time.Millisecond
	cache := NewResolverCache(10, shortTTL)
	defer cache.Close()

	project := &SubProject{ID: "test", Name: "test"}

	// Add entry to cache
	cache.Put("/test", project)

	// Should be retrievable immediately
	if cache.Get("/test") != project {
		t.Error("Expected project in cache")
	}

	// Wait for TTL expiration
	time.Sleep(shortTTL + 10*time.Millisecond)

	// Should be expired
	if cache.Get("/test") != nil {
		t.Error("Expected project to be expired")
	}

	// Verify TTL expired count
	stats := cache.GetStats()
	if stats.TTLExpiredCount == 0 {
		t.Error("Expected TTL expiration to be recorded")
	}
}

// TestResolverCache_Invalidation tests cache invalidation functionality
func TestResolverCache_Invalidation(t *testing.T) {
	t.Parallel()
	cache := NewResolverCache(10, 5*time.Minute)
	defer cache.Close()

	project := &SubProject{ID: "test", Name: "test"}

	// Add entries under project path
	cache.Put("/project/file1.go", project)
	cache.Put("/project/src/file2.go", project)
	cache.Put("/project/tests/file3.go", project)
	cache.Put("/other/file.go", project)

	// Invalidate by project path
	invalidated := cache.InvalidateByProject("/project")

	// Should invalidate 3 entries
	if invalidated != 3 {
		t.Errorf("Expected 3 invalidated entries, got %d", invalidated)
	}

	// Project files should be invalidated
	if cache.Get("/project/file1.go") != nil {
		t.Error("Expected /project/file1.go to be invalidated")
	}

	// Other path should remain
	if cache.Get("/other/file.go") != project {
		t.Error("Expected /other/file.go to remain in cache")
	}
}

// TestSubProject_Hierarchy tests project hierarchy building
func TestSubProject_Hierarchy(t *testing.T) {
	t.Parallel()
	tempDir := createTempWorkspace(t)
	defer os.RemoveAll(tempDir)

	resolver := NewSubProjectResolver(tempDir, &ResolverOptions{
		CacheCapacity: 100,
		CacheTTL:      5 * time.Minute,
		MaxDepth:      5,
	}).(*SubProjectResolverImpl)
	defer resolver.Close()

	// Discover projects
	ctx := context.Background()
	err := resolver.RefreshProjects(ctx)
	if err != nil {
		t.Fatalf("Failed to refresh projects: %v", err)
	}

	projects := resolver.GetAllSubProjects()

	// Find nested service project
	var serviceProject *SubProject
	for _, p := range projects {
		if strings.Contains(p.Root, "service") {
			serviceProject = p
			break
		}
	}

	if serviceProject == nil {
		t.Fatal("Expected to find service project")
	}

	// Should have parent (nested directory)
	if serviceProject.Parent == nil {
		t.Error("Expected service project to have parent")
	}

	// Verify hierarchy consistency
	if serviceProject.Parent != nil {
		found := false
		for _, child := range serviceProject.Parent.Children {
			if child == serviceProject {
				found = true
				break
			}
		}
		if !found {
			t.Error("Parent-child relationship is inconsistent")
		}
	}
}

// TestSubProjectResolver_Performance tests performance characteristics
func TestSubProjectResolver_Performance(t *testing.T) {
	t.Parallel()
	tempDir := createTempWorkspace(t)
	defer os.RemoveAll(tempDir)

	resolver := NewSubProjectResolver(tempDir, &ResolverOptions{
		CacheCapacity: 1000,
		CacheTTL:      5 * time.Minute,
		MaxDepth:      5,
	})
	defer resolver.Close()

	// Refresh projects
	ctx := context.Background()
	err := resolver.RefreshProjects(ctx)
	if err != nil {
		t.Fatalf("Failed to refresh projects: %v", err)
	}

	// Test resolution performance
	testFile := filepath.Join(tempDir, "backend/main.go")
	iterations := 1000

	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, err := resolver.ResolveSubProjectCached(testFile)
		if err != nil {
			t.Fatalf("Resolution failed: %v", err)
		}
	}
	elapsed := time.Since(start)

	avgLatency := elapsed / time.Duration(iterations)
	if avgLatency > time.Millisecond {
		t.Errorf("Average resolution latency %v exceeds 1ms target", avgLatency)
	}

	// Verify high cache hit rate
	cacheStats := resolver.GetCacheStats()
	if cacheStats.HitRate < 0.95 {
		t.Errorf("Cache hit rate %.2f below 95%% target", cacheStats.HitRate)
	}

	// Test metrics
	metrics := resolver.GetMetrics()
	if metrics.TotalResolutions < int64(iterations) {
		t.Error("Resolution count not properly tracked")
	}

	if metrics.AverageLatency > time.Millisecond {
		t.Errorf("Average latency %v exceeds target", metrics.AverageLatency)
	}
}

// TestSubProjectResolver_ErrorHandling tests error handling scenarios
func TestSubProjectResolver_ErrorHandling(t *testing.T) {
	t.Parallel()
	resolver := NewSubProjectResolver("/nonexistent", nil)
	defer resolver.Close()

	// Test invalid file URI
	_, err := resolver.ResolveSubProject("invalid://uri")
	if err == nil {
		t.Error("Expected error for invalid URI")
	}

	// Test malformed file path
	_, err = resolver.ResolveSubProject("file://")
	if err != nil {
		// Should handle gracefully, possibly with fallback
		t.Logf("Handled malformed URI: %v", err)
	}

	// Test metrics track errors
	metrics := resolver.GetMetrics()
	if metrics.URIParsingErrors == 0 {
		t.Error("Expected URI parsing errors to be tracked")
	}
}

// TestSubProjectResolver_ConcurrentAccess tests thread safety
func TestSubProjectResolver_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	tempDir := createTempWorkspace(t)
	defer os.RemoveAll(tempDir)

	resolver := NewSubProjectResolver(tempDir, nil)
	defer resolver.Close()

	// Refresh projects
	ctx := context.Background()
	err := resolver.RefreshProjects(ctx)
	if err != nil {
		t.Fatalf("Failed to refresh projects: %v", err)
	}

	// Test concurrent resolutions
	testFile := filepath.Join(tempDir, "backend/main.go")
	numGoroutines := 100
	results := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(i int) {
			for j := 0; j < 10; j++ {
				_, err := resolver.ResolveSubProjectCached(testFile)
				if err != nil {
					results <- err
					return
				}
			}
			results <- nil
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		if err := <-results; err != nil {
			t.Errorf("Concurrent resolution failed: %v", err)
		}
	}

	// Test concurrent refresh
	go func() {
		resolver.RefreshProjects(ctx)
	}()

	// Continue resolving during refresh
	for i := 0; i < 10; i++ {
		_, err := resolver.ResolveSubProject(testFile)
		if err != nil {
			t.Errorf("Resolution during refresh failed: %v", err)
		}
	}
}

// createTempWorkspace creates a temporary workspace with test project structure
func createTempWorkspace(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "lsp-test-workspace-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create project structure
	structure := map[string]string{
		"backend/main.go":              "package main\n\nfunc main() {}\n",
		"backend/go.mod":               "module backend\n\ngo 1.21\n",
		"frontend/package.json":        `{"name": "frontend", "version": "1.0.0"}`,
		"frontend/src/index.js":        "console.log('hello');\n",
		"shared/utils.py":              "def helper(): pass\n",
		"shared/setup.py":              "from setuptools import setup\nsetup(name='shared')\n",
		"nested/service/app.java":      "public class App { }\n",
		"nested/service/pom.xml":       "<project></project>\n",
		"docs/README.md":               "# Documentation\n",
		"config/settings.yml":          "debug: true\n",
	}

	for filePath, content := range structure {
		fullPath := filepath.Join(tempDir, filePath)
		dir := filepath.Dir(fullPath)

		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}

		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", fullPath, err)
		}
	}

	return tempDir
}

// BenchmarkSubProjectResolver_Resolution benchmarks resolution performance
func BenchmarkSubProjectResolver_Resolution(b *testing.B) {
	tempDir := createTempWorkspace(&testing.T{})
	defer os.RemoveAll(tempDir)

	resolver := NewSubProjectResolver(tempDir, nil)
	defer resolver.Close()

	// Setup
	ctx := context.Background()
	resolver.RefreshProjects(ctx)

	testFile := filepath.Join(tempDir, "backend/main.go")

	b.ResetTimer()

	// Benchmark cached resolution
	b.Run("CachedResolution", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := resolver.ResolveSubProjectCached(testFile)
			if err != nil {
				b.Fatalf("Resolution failed: %v", err)
			}
		}
	})

	// Benchmark uncached resolution
	b.Run("UnCachedResolution", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Clear cache to force trie lookup
			resolver.InvalidateCache(tempDir)
			_, err := resolver.ResolveSubProject(testFile)
			if err != nil {
				b.Fatalf("Resolution failed: %v", err)
			}
		}
	})
}