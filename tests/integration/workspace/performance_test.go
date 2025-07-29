package workspace

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"lsp-gateway/internal/workspace"
	"lsp-gateway/tests/integration/workspace/helpers"
)

type WorkspacePerformanceSuite struct {
	suite.Suite
	tempDir   string
	detector  workspace.WorkspaceDetector
	helper    *helpers.WorkspaceTestHelper
}

func (suite *WorkspacePerformanceSuite) SetupSuite() {
	tempDir, err := os.MkdirTemp("", "workspace-performance-test-*")
	require.NoError(suite.T(), err)
	
	suite.tempDir = tempDir
	suite.detector = workspace.NewWorkspaceDetector()
	suite.helper = helpers.NewWorkspaceTestHelper(tempDir)
}

func (suite *WorkspacePerformanceSuite) TearDownSuite() {
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
}

func (suite *WorkspacePerformanceSuite) TestWorkspaceDetectionPerformance() {
	// Test performance with different workspace sizes
	testCases := []struct {
		name         string
		projectCount int
		maxDuration  time.Duration
	}{
		{
			name:         "small-workspace",
			projectCount: 10,
			maxDuration:  2 * time.Second,
		},
		{
			name:         "medium-workspace", 
			projectCount: 25,
			maxDuration:  3 * time.Second,
		},
		{
			name:         "large-workspace",
			projectCount: 50,
			maxDuration:  5 * time.Second,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create workspace with specified number of projects
			workspacePath := suite.helper.CreateLargeWorkspace(tc.name, tc.projectCount)

			// Measure detection time
			start := time.Now()
			result, err := suite.detector.DetectWorkspaceAt(workspacePath)
			duration := time.Since(start)

			require.NoError(suite.T(), err)
			require.NotNil(suite.T(), result)

			// Validate performance
			assert.Less(suite.T(), duration, tc.maxDuration, 
				"Workspace detection took %v, expected less than %v for %d projects", 
				duration, tc.maxDuration, tc.projectCount)

			// Validate that most projects were detected
			expectedMinProjects := tc.projectCount * 80 / 100 // At least 80% of projects
			assert.GreaterOrEqual(suite.T(), len(result.SubProjects), expectedMinProjects,
				"Expected at least %d projects, got %d", expectedMinProjects, len(result.SubProjects))

			suite.T().Logf("Detected %d projects in %v", len(result.SubProjects), duration)
		})
	}
}

func (suite *WorkspacePerformanceSuite) TestPathResolutionPerformance() {
	// Create workspace with many projects
	workspacePath := suite.helper.CreateLargeWorkspace("path-resolution-perf", 30)

	result, err := suite.detector.DetectWorkspaceAt(workspacePath)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), result)

	// Test path resolution performance
	testPaths := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		// Generate various file paths for testing
		projectIndex := i % len(result.SubProjects)
		subProject := result.SubProjects[projectIndex]
		testPaths[i] = subProject.AbsolutePath + "/test-file-" + string(rune(i%10)) + ".go"
	}

	// Measure path resolution time
	start := time.Now()
	for _, testPath := range testPaths {
		project := suite.detector.FindSubProjectForPath(result, testPath)
		_ = project // We just care about the performance, not the result
	}
	duration := time.Since(start)

	// Should resolve 1000 paths in less than 100ms (< 0.1ms per lookup)
	maxDuration := 100 * time.Millisecond
	assert.Less(suite.T(), duration, maxDuration,
		"Path resolution for 1000 paths took %v, expected less than %v", duration, maxDuration)

	avgDuration := duration / time.Duration(len(testPaths))
	suite.T().Logf("Average path resolution time: %v per lookup", avgDuration)
}

func (suite *WorkspacePerformanceSuite) TestMemoryUsageDuringDetection() {
	// Create large workspace
	workspacePath := suite.helper.CreateBenchmarkWorkspace("memory-test", 20, 10)

	// Use a context to monitor memory usage patterns
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Detection should complete without excessive memory usage
	result, err := suite.detector.DetectWorkspaceWithContext(ctx, workspacePath)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), result)

	// Validate reasonable project detection
	assert.Greater(suite.T(), len(result.SubProjects), 15,
		"Expected to detect most projects in memory test")

	// Memory usage is implicit - if the test completes without OOM, it passes
	suite.T().Logf("Successfully detected %d projects with controlled memory usage", len(result.SubProjects))
}

func (suite *WorkspacePerformanceSuite) TestConcurrentDetection() {
	// Test concurrent workspace detection
	workspaces := make([]string, 5)
	for i := 0; i < 5; i++ {
		workspaces[i] = suite.helper.CreateWorkspaceTemplate("concurrent-"+string(rune('A'+i)), "go-monorepo")
	}

	// Run detections concurrently
	type result struct {
		workspace *workspace.WorkspaceContext
		err       error
		duration  time.Duration
	}

	results := make(chan result, len(workspaces))

	start := time.Now()
	for _, ws := range workspaces {
		go func(workspacePath string) {
			startTime := time.Now()
			workspace, err := suite.detector.DetectWorkspaceAt(workspacePath)
			duration := time.Since(startTime)
			results <- result{workspace: workspace, err: err, duration: duration}
		}(ws)
	}

	// Collect results
	var detectedWorkspaces []*workspace.WorkspaceContext
	var maxDuration time.Duration
	for i := 0; i < len(workspaces); i++ {
		r := <-results
		require.NoError(suite.T(), r.err)
		require.NotNil(suite.T(), r.workspace)
		detectedWorkspaces = append(detectedWorkspaces, r.workspace)
		
		if r.duration > maxDuration {
			maxDuration = r.duration
		}
	}
	totalDuration := time.Since(start)

	// Validate concurrent performance
	assert.Len(suite.T(), detectedWorkspaces, len(workspaces))
	assert.Less(suite.T(), totalDuration, 10*time.Second, "Concurrent detection took too long")
	assert.Less(suite.T(), maxDuration, 5*time.Second, "Individual detection took too long")

	suite.T().Logf("Concurrent detection of %d workspaces completed in %v (max individual: %v)", 
		len(workspaces), totalDuration, maxDuration)
}

func (suite *WorkspacePerformanceSuite) TestDeepDirectoryPerformance() {
	// Create workspace with very deep directory structure
	deepStructure := make(map[string]string)
	
	// Create root project
	deepStructure["go.mod"] = `module deep-test

go 1.24
`
	deepStructure["main.go"] = `package main

func main() {}
`

	// Create nested projects up to depth limit
	currentPath := ""
	for depth := 1; depth <= 6; depth++ {
		currentPath += "/level" + string(rune('0'+depth))
		
		deepStructure[currentPath[1:]+"/go.mod"] = `module deep-test` + currentPath + `

go 1.24
`
		deepStructure[currentPath[1:]+"/main.go"] = `package main

func main() {}
`
	}

	workspacePath := suite.helper.CreateNestedProject("deep-performance", deepStructure)

	// Measure detection time for deep structure
	start := time.Now()
	result, err := suite.detector.DetectWorkspaceAt(workspacePath)
	duration := time.Since(start)

	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), result)

	// Should handle deep structures efficiently
	assert.Less(suite.T(), duration, 3*time.Second, 
		"Deep directory detection took %v, expected less than 3s", duration)

	// Should respect depth limits (default is 5)
	assert.LessOrEqual(suite.T(), len(result.SubProjects), 6, // 5 levels + root
		"Detected more projects than expected depth limit allows")

	suite.T().Logf("Deep directory detection completed in %v with %d projects", 
		duration, len(result.SubProjects))
}

func (suite *WorkspacePerformanceSuite) TestContextCancellationPerformance() {
	// Create large workspace for cancellation testing
	workspacePath := suite.helper.CreateLargeWorkspace("cancellation-test", 100)

	// Test cancellation at different timeouts
	timeouts := []time.Duration{
		10 * time.Millisecond,
		100 * time.Millisecond,
		1 * time.Second,
	}

	for _, timeout := range timeouts {
		suite.Run("timeout-"+timeout.String(), func() {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			start := time.Now()
			result, err := suite.detector.DetectWorkspaceWithContext(ctx, workspacePath)
			duration := time.Since(start)

			if timeout < 1*time.Second {
				// Short timeouts should result in cancellation
				assert.Error(suite.T(), err)
				assert.Contains(suite.T(), err.Error(), "context")
				assert.Nil(suite.T(), result)
			} else {
				// Longer timeout should allow completion
				assert.NoError(suite.T(), err)
				assert.NotNil(suite.T(), result)
			}

			// Cancellation should happen promptly
			assert.LessOrEqual(suite.T(), duration, timeout+100*time.Millisecond,
				"Cancellation took longer than expected: %v", duration)

			suite.T().Logf("Context cancellation with timeout %v completed in %v", timeout, duration)
		})
	}
}

func (suite *WorkspacePerformanceSuite) TestWorkspaceIDGenerationPerformance() {
	// Test workspace ID generation performance
	paths := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		paths[i] = suite.tempDir + "/test-workspace-" + string(rune('0'+i%10))
	}

	// Measure ID generation time
	start := time.Now()
	ids := make([]string, len(paths))
	for i, path := range paths {
		ids[i] = suite.detector.GenerateWorkspaceID(path)
	}
	duration := time.Since(start)

	// ID generation should be very fast
	maxDuration := 10 * time.Millisecond
	assert.Less(suite.T(), duration, maxDuration,
		"ID generation for 1000 paths took %v, expected less than %v", duration, maxDuration)

	// All IDs should be unique
	idSet := make(map[string]bool)
	for _, id := range ids {
		assert.False(suite.T(), idSet[id], "Duplicate ID generated: %s", id)
		idSet[id] = true
		assert.NotEmpty(suite.T(), id, "Empty ID generated")
	}

	avgDuration := duration / time.Duration(len(paths))
	suite.T().Logf("Average ID generation time: %v per ID", avgDuration)
}

func (suite *WorkspacePerformanceSuite) TestWorkspaceHashGenerationPerformance() {
	// Create workspaces with different structures
	workspaces := []string{
		suite.helper.CreateWorkspaceTemplate("hash-small", "default"),
		suite.helper.CreateWorkspaceTemplate("hash-monorepo", "go-monorepo"),
		suite.helper.CreateWorkspaceTemplate("hash-fullstack", "fullstack-monorepo"),
		suite.helper.CreateLargeWorkspace("hash-large", 20),
	}

	// Detect all workspaces first
	workspaceContexts := make([]*workspace.WorkspaceContext, len(workspaces))
	for i, ws := range workspaces {
		result, err := suite.detector.DetectWorkspaceAt(ws)
		require.NoError(suite.T(), err)
		workspaceContexts[i] = result
	}

	// Measure hash generation (implicit in detection)
	start := time.Now()
	hashes := make([]string, len(workspaceContexts))
	for i, ctx := range workspaceContexts {
		// Hash is generated during detection, we just access it
		hashes[i] = ctx.Hash
	}
	duration := time.Since(start)

	// Hash access should be instantaneous
	assert.Less(suite.T(), duration, 1*time.Millisecond,
		"Hash access took %v, expected instantaneous", duration)

	// All hashes should be unique and non-empty
	hashSet := make(map[string]bool)
	for i, hash := range hashes {
		assert.NotEmpty(suite.T(), hash, "Empty hash for workspace %d", i)
		assert.False(suite.T(), hashSet[hash], "Duplicate hash: %s", hash)
		hashSet[hash] = true
	}

	suite.T().Logf("Generated %d unique hashes", len(hashes))
}

func (suite *WorkspacePerformanceSuite) TestScalabilityLimits() {
	// Test workspace detection at scale limits
	if testing.Short() {
		suite.T().Skip("Skipping scalability test in short mode")
	}

	// Create workspace at scale limits (up to 50 sub-projects as per requirements)
	workspacePath := suite.helper.CreateLargeWorkspace("scalability-test", 50)

	start := time.Now()
	result, err := suite.detector.DetectWorkspaceAt(workspacePath)
	duration := time.Since(start)

	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), result)

	// Should handle 50 projects within 5 seconds
	assert.Less(suite.T(), duration, 5*time.Second,
		"Scalability test took %v, expected less than 5s", duration)

	// Should detect most of the 50 projects
	assert.GreaterOrEqual(suite.T(), len(result.SubProjects), 40,
		"Expected at least 40 projects at scale, got %d", len(result.SubProjects))

	// Memory usage should be reasonable (under 50MB is implicit - if OOM occurs, test fails)
	suite.T().Logf("Scalability test: %d projects detected in %v", len(result.SubProjects), duration)

	// Test path resolution performance at scale
	pathResolutionStart := time.Now()
	for _, subProject := range result.SubProjects {
		testFile := subProject.AbsolutePath + "/test.go"
		foundProject := suite.detector.FindSubProjectForPath(result, testFile)
		assert.NotNil(suite.T(), foundProject)
	}
	pathResolutionDuration := time.Since(pathResolutionStart)

	// Path resolution should be fast even at scale
	assert.Less(suite.T(), pathResolutionDuration, 10*time.Millisecond,
		"Path resolution at scale took %v, expected under 10ms", pathResolutionDuration)

	suite.T().Logf("Path resolution at scale: %v for %d lookups", 
		pathResolutionDuration, len(result.SubProjects))
}

func (suite *WorkspacePerformanceSuite) TestWorkspaceValidationPerformance() {
	// Create workspaces of different sizes for validation performance testing
	workspaces := []struct {
		name  string
		count int
	}{
		{"validation-small", 5},
		{"validation-medium", 15},
		{"validation-large", 30},
	}

	for _, ws := range workspaces {
		workspacePath := suite.helper.CreateLargeWorkspace(ws.name, ws.count)

		// Measure validation time
		start := time.Now()
		err := suite.detector.ValidateWorkspace(workspacePath)
		duration := time.Since(start)

		assert.NoError(suite.T(), err)

		// Validation should be very fast (under 100ms even for large workspaces)
		assert.Less(suite.T(), duration, 100*time.Millisecond,
			"Workspace validation took %v for %d projects, expected under 100ms", 
			duration, ws.count)

		suite.T().Logf("Validation performance for %d projects: %v", ws.count, duration)
	}
}

func TestWorkspacePerformanceSuite(t *testing.T) {
	suite.Run(t, new(WorkspacePerformanceSuite))
}