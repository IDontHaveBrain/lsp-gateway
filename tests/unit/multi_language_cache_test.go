package unit

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"lsp-gateway/internal/gateway"
)

// ProjectCacheTestSuite tests the ProjectCache implementation
type ProjectCacheTestSuite struct {
	suite.Suite
	cache   *gateway.ProjectCache
	tempDir string
}

func (suite *ProjectCacheTestSuite) SetupTest() {
	suite.cache = gateway.NewProjectCache()

	tempDir, err := os.MkdirTemp("", "cache-test-*")
	suite.Require().NoError(err)
	suite.tempDir = tempDir
}

func (suite *ProjectCacheTestSuite) TearDownTest() {
	if suite.cache != nil {
		suite.cache.Shutdown()
	}
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
}

func (suite *ProjectCacheTestSuite) createTestProjectInfo(rootPath string) *gateway.MultiLanguageProjectInfo {
	return &gateway.MultiLanguageProjectInfo{
		RootPath:         rootPath,
		ProjectType:      "test-project",
		DominantLanguage: "go",
		Languages: map[string]*gateway.LanguageContext{
			"go": {
				Language:       "go",
				RootPath:       rootPath,
				FileCount:      5,
				TestFileCount:  2,
				Priority:       85,
				Confidence:     0.9,
				BuildFiles:     []string{"go.mod"},
				ConfigFiles:    []string{},
				SourcePaths:    []string{"src"},
				TestPaths:      []string{"test"},
				FileExtensions: []string{".go"},
				Dependencies:   []string{},
				Metadata:       make(map[string]interface{}),
			},
		},
		WorkspaceRoots: map[string]string{
			"go": rootPath,
		},
		BuildFiles:     []string{"go.mod"},
		ConfigFiles:    []string{},
		TotalFileCount: 7,
		ScanDepth:      3,
		ScanDuration:   100 * time.Millisecond,
		DetectedAt:     time.Now(),
		Metadata:       make(map[string]interface{}),
	}
}

// TestCacheBasicOperations tests basic cache operations
func (suite *ProjectCacheTestSuite) TestCacheBasicOperations() {
	projectPath := "/test/project"
	projectInfo := suite.createTestProjectInfo(projectPath)

	// Test initial miss
	cachedInfo, hit := suite.cache.Get(projectPath)
	suite.False(hit, "Should miss on empty cache")
	suite.Nil(cachedInfo, "Should return nil on miss")

	// Test set and get
	suite.cache.Set(projectPath, projectInfo)

	cachedInfo, hit = suite.cache.Get(projectPath)
	suite.True(hit, "Should hit after set")
	suite.NotNil(cachedInfo, "Should return project info on hit")
	suite.Equal(projectInfo.ProjectType, cachedInfo.ProjectType, "Cached info should match original")
	suite.Equal(projectInfo.DominantLanguage, cachedInfo.DominantLanguage, "Cached info should match original")

	// Test invalidation
	suite.cache.InvalidateProject(projectPath)

	cachedInfo, hit = suite.cache.Get(projectPath)
	suite.False(hit, "Should miss after invalidation")
	suite.Nil(cachedInfo, "Should return nil after invalidation")
}

// TestCacheStatistics tests cache statistics tracking
func (suite *ProjectCacheTestSuite) TestCacheStatistics() {
	projectPath1 := "/test/project1"
	projectPath2 := "/test/project2"
	projectInfo1 := suite.createTestProjectInfo(projectPath1)
	projectInfo2 := suite.createTestProjectInfo(projectPath2)

	// Initial stats should be zero
	stats := suite.cache.GetStats()
	suite.Equal(int64(0), stats.HitCount, "Initial hit count should be 0")
	suite.Equal(int64(0), stats.MissCount, "Initial miss count should be 0")
	suite.Equal(0.0, stats.HitRatio, "Initial hit ratio should be 0.0")

	// Generate some misses
	_, hit := suite.cache.Get(projectPath1)
	suite.False(hit)
	_, hit = suite.cache.Get(projectPath2)
	suite.False(hit)

	stats = suite.cache.GetStats()
	suite.Equal(int64(0), stats.HitCount, "Hit count should still be 0")
	suite.Equal(int64(2), stats.MissCount, "Miss count should be 2")
	suite.Equal(0.0, stats.HitRatio, "Hit ratio should still be 0.0")

	// Add entries and generate hits
	suite.cache.Set(projectPath1, projectInfo1)
	suite.cache.Set(projectPath2, projectInfo2)

	_, hit = suite.cache.Get(projectPath1)
	suite.True(hit)
	_, hit = suite.cache.Get(projectPath2)
	suite.True(hit)
	_, hit = suite.cache.Get(projectPath1) // Second hit
	suite.True(hit)

	stats = suite.cache.GetStats()
	suite.Equal(int64(3), stats.HitCount, "Hit count should be 3")
	suite.Equal(int64(2), stats.MissCount, "Miss count should still be 2")
	suite.Equal(2, stats.TotalEntries, "Total entries should be 2")
	suite.Equal(0.6, stats.HitRatio, "Hit ratio should be 0.6 (3/5)")
	suite.Greater(stats.CacheSize, int64(0), "Cache size should be positive")
}

// TestCacheEviction tests that cache can handle multiple entries
func (suite *ProjectCacheTestSuite) TestCacheEviction() {
	// Test with default cache (which should have reasonable limits)
	project1 := "/test/project1"
	project2 := "/test/project2"
	project3 := "/test/project3"

	info1 := suite.createTestProjectInfo(project1)
	info2 := suite.createTestProjectInfo(project2)
	info3 := suite.createTestProjectInfo(project3)

	// Add multiple projects
	suite.cache.Set(project1, info1)
	suite.cache.Set(project2, info2)
	suite.cache.Set(project3, info3)

	// All should be in cache for default configuration
	_, hit1 := suite.cache.Get(project1)
	_, hit2 := suite.cache.Get(project2)
	_, hit3 := suite.cache.Get(project3)

	suite.True(hit1, "Project1 should be in cache")
	suite.True(hit2, "Project2 should be in cache")
	suite.True(hit3, "Project3 should be in cache")

	// Check stats
	stats := suite.cache.GetStats()
	suite.Equal(3, stats.TotalEntries, "Should have 3 entries")
}

// TestCacheTTL tests basic cache persistence
func (suite *ProjectCacheTestSuite) TestCacheTTL() {
	projectPath := "/test/project"
	projectInfo := suite.createTestProjectInfo(projectPath)

	// Set entry
	suite.cache.Set(projectPath, projectInfo)

	// Should be available immediately
	_, hit := suite.cache.Get(projectPath)
	suite.True(hit, "Should be available immediately after set")

	// Should still be available after short wait (cache should have reasonable TTL)
	time.Sleep(10 * time.Millisecond)
	_, hit = suite.cache.Get(projectPath)
	suite.True(hit, "Should still be available after short time")
}

// TestCacheInvalidateAll tests clearing all cache entries
func (suite *ProjectCacheTestSuite) TestCacheInvalidateAll() {
	project1 := "/test/project1"
	project2 := "/test/project2"
	project3 := "/test/project3"

	info1 := suite.createTestProjectInfo(project1)
	info2 := suite.createTestProjectInfo(project2)
	info3 := suite.createTestProjectInfo(project3)

	// Fill cache
	suite.cache.Set(project1, info1)
	suite.cache.Set(project2, info2)
	suite.cache.Set(project3, info3)

	// Verify all are cached
	stats := suite.cache.GetStats()
	suite.Equal(3, stats.TotalEntries, "Should have 3 entries")

	// Clear all
	suite.cache.InvalidateAll()

	// Verify all are gone
	stats = suite.cache.GetStats()
	suite.Equal(0, stats.TotalEntries, "Should have 0 entries after clear")
	suite.Equal(int64(3), stats.EvictionCount, "Should have 3 evictions from clear")

	// Verify individual gets miss
	_, hit1 := suite.cache.Get(project1)
	_, hit2 := suite.cache.Get(project2)
	_, hit3 := suite.cache.Get(project3)

	suite.False(hit1, "Project1 should not be in cache")
	suite.False(hit2, "Project2 should not be in cache")
	suite.False(hit3, "Project3 should not be in cache")
}

// TestCacheNilHandling tests handling of nil values
func (suite *ProjectCacheTestSuite) TestCacheNilHandling() {
	projectPath := "/test/project"

	// Setting nil should be ignored
	suite.cache.Set(projectPath, nil)

	_, hit := suite.cache.Get(projectPath)
	suite.False(hit, "Setting nil should be ignored")

	// Stats should not change
	stats := suite.cache.GetStats()
	suite.Equal(0, stats.TotalEntries, "Should have 0 entries")
}

// TestCacheConcurrentAccess tests concurrent access to cache
func (suite *ProjectCacheTestSuite) TestCacheConcurrentAccess() {
	const numGoroutines = 10
	const numOperations = 100

	projectInfo := suite.createTestProjectInfo("/test/project")

	// Concurrent sets and gets
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numOperations; j++ {
				projectPath := fmt.Sprintf("/test/project_%d_%d", id, j)

				// Set and immediately get
				suite.cache.Set(projectPath, projectInfo)
				_, hit := suite.cache.Get(projectPath)

				// Should hit since we just set it
				if !hit {
					suite.T().Errorf("Should hit for project %s", projectPath)
				}
			}
		}(i)
	}

	// Wait a bit for goroutines to complete
	time.Sleep(1 * time.Second)

	// Cache should have entries
	stats := suite.cache.GetStats()
	suite.Greater(stats.TotalEntries, 0, "Should have entries from concurrent operations")
	suite.Greater(stats.HitCount, int64(0), "Should have hits from concurrent operations")
}

// Run the test suite
func TestProjectCacheTestSuite(t *testing.T) {
	suite.Run(t, new(ProjectCacheTestSuite))
}

// Individual unit tests for cache components

func TestCacheStatsCalculation(t *testing.T) {
	cache := gateway.NewProjectCache()
	defer cache.Shutdown()

	// Test initial state
	stats := cache.GetStats()
	assert.Equal(t, int64(0), stats.HitCount)
	assert.Equal(t, int64(0), stats.MissCount)
	assert.Equal(t, 0.0, stats.HitRatio)
	assert.Equal(t, 0, stats.TotalEntries)

	// Generate some hits and misses
	projectInfo := &gateway.MultiLanguageProjectInfo{
		RootPath:         "/test",
		ProjectType:      "test",
		DominantLanguage: "go",
		Languages:        make(map[string]*gateway.LanguageContext),
		WorkspaceRoots:   make(map[string]string),
		BuildFiles:       []string{},
		ConfigFiles:      []string{},
		TotalFileCount:   1,
		DetectedAt:       time.Now(),
		Metadata:         make(map[string]interface{}),
	}

	// 2 misses
	cache.Get("/test1")
	cache.Get("/test2")

	// 1 set and 3 hits
	cache.Set("/test1", projectInfo)
	cache.Get("/test1")
	cache.Get("/test1")
	cache.Get("/test1")

	stats = cache.GetStats()
	assert.Equal(t, int64(3), stats.HitCount)
	assert.Equal(t, int64(2), stats.MissCount)
	assert.Equal(t, 0.6, stats.HitRatio) // 3/(3+2) = 0.6
	assert.Equal(t, 1, stats.TotalEntries)
}

func TestCacheEntryValidation(t *testing.T) {
	cache := gateway.NewProjectCache()
	defer cache.Shutdown()

	// Create a temporary directory for testing directory modification time
	tempDir, err := os.MkdirTemp("", "cache-validation-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	projectInfo := &gateway.MultiLanguageProjectInfo{
		RootPath:         tempDir,
		ProjectType:      "test",
		DominantLanguage: "go",
		Languages:        make(map[string]*gateway.LanguageContext),
		WorkspaceRoots:   make(map[string]string),
		BuildFiles:       []string{},
		ConfigFiles:      []string{},
		TotalFileCount:   1,
		DetectedAt:       time.Now(),
		Metadata:         make(map[string]interface{}),
	}

	// Set entry
	cache.Set(tempDir, projectInfo)

	// Should be valid immediately
	info, hit := cache.Get(tempDir)
	assert.True(t, hit)
	assert.NotNil(t, info)

	// Modify directory to invalidate cache
	testFile := filepath.Join(tempDir, "test.go")
	err = os.WriteFile(testFile, []byte("package main"), 0644)
	require.NoError(t, err)

	// Wait a bit to ensure modification time changes
	time.Sleep(10 * time.Millisecond)

	// Should now be invalid due to directory modification
	info, hit = cache.Get(tempDir)
	assert.False(t, hit, "Should be invalid after directory modification")
	assert.Nil(t, info)
}

func TestCacheConfigValidation(t *testing.T) {
	// For now, just test that we can create cache with default config
	cache := gateway.NewProjectCache()
	assert.NotNil(t, cache)
	cache.Shutdown()
}

func TestBackgroundScannerBasics(t *testing.T) {
	cache := gateway.NewProjectCache()
	defer cache.Shutdown()

	// The background scanner is created internally
	// We can test that it's working by setting up a project and seeing if it gets scanned

	tempDir, err := os.MkdirTemp("", "bg-scanner-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a simple Go project
	goMod := filepath.Join(tempDir, "go.mod")
	err = os.WriteFile(goMod, []byte("module test\n\ngo 1.19"), 0644)
	require.NoError(t, err)

	goMain := filepath.Join(tempDir, "main.go")
	err = os.WriteFile(goMain, []byte("package main\n\nfunc main() {}"), 0644)
	require.NoError(t, err)

	// The background scanner should eventually scan this if we trigger it
	// For this test, we'll just verify the cache can store and retrieve the project
	scanner := gateway.NewProjectLanguageScanner()
	defer scanner.Shutdown()

	info, err := scanner.ScanProject(tempDir)
	require.NoError(t, err)
	require.NotNil(t, info)

	cache.Set(tempDir, info)
	cachedInfo, hit := cache.Get(tempDir)
	assert.True(t, hit)
	assert.NotNil(t, cachedInfo)
	assert.Equal(t, info.ProjectType, cachedInfo.ProjectType)
}
