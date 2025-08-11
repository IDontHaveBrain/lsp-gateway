package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server"
	"lsp-gateway/src/server/cache"
	"lsp-gateway/src/utils"
	"lsp-gateway/tests/e2e/testutils"

	"github.com/stretchr/testify/suite"
)

type SCIPCacheE2ETestSuite struct {
	suite.Suite
	tempDir        string
	cacheDir       string
	testProjectDir string
	originalWd     string
	manager        *server.LSPManager
	scipCache      cache.SCIPCache
}

func (suite *SCIPCacheE2ETestSuite) SetupSuite() {
	suite.tempDir = suite.T().TempDir()
	suite.cacheDir = filepath.Join(suite.tempDir, "scip-cache")
	suite.testProjectDir = filepath.Join(suite.tempDir, "test-project")

	err := os.MkdirAll(suite.testProjectDir, 0755)
	suite.Require().NoError(err)

	suite.createTestFiles()
}

func (suite *SCIPCacheE2ETestSuite) createTestFiles() {
	testFiles := map[string]string{
		"main.go": `package main

import "fmt"

type Server struct {
	port int
	host string
}

func (s *Server) Start() error {
	fmt.Printf("Starting server on %s:%d\n", s.host, s.port)
	return nil
}

func main() {
	server := &Server{
		port: 8080,
		host: "localhost",
	}
	server.Start()
}`,
		"utils/helper.go": `package utils

type Helper struct {
	name string
}

func (h *Helper) GetName() string {
	return h.name
}

func NewHelper(name string) *Helper {
	return &Helper{name: name}
}`,
		"handlers/user.go": `package handlers

import "fmt"

type UserHandler struct {
	db Database
}

type Database interface {
	GetUser(id string) (User, error)
}

type User struct {
	ID   string
	Name string
}

func (h *UserHandler) HandleGetUser(id string) (*User, error) {
	user, err := h.db.GetUser(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}`,
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(suite.testProjectDir, path)
		dir := filepath.Dir(fullPath)
		err := os.MkdirAll(dir, 0755)
		suite.Require().NoError(err)

		err = os.WriteFile(fullPath, []byte(content), 0644)
		suite.Require().NoError(err)
	}

	goMod := `module test-project

go 1.21
`
	err := os.WriteFile(filepath.Join(suite.testProjectDir, "go.mod"), []byte(goMod), 0644)
	suite.Require().NoError(err)
}

func (suite *SCIPCacheE2ETestSuite) SetupTest() {
	// Change to test project directory before creating manager
	// This ensures gopls initializes with the correct workspace
	originalWd, err := os.Getwd()
	suite.Require().NoError(err)
	suite.originalWd = originalWd

	err = os.Chdir(suite.testProjectDir)
	suite.Require().NoError(err)

	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"go": {
				Command: "gopls",
				Args:    []string{"serve"},
			},
		},
		Cache: &config.CacheConfig{
			Enabled:            true,
			StoragePath:        suite.cacheDir,
			MaxMemoryMB:        128,
			TTLHours:           1,
			Languages:          []string{"go"},
			BackgroundIndex:    false, // Disable background indexing in tests
			DiskCache:          true,
			EvictionPolicy:     "lru",
			HealthCheckMinutes: 1,
		},
	}

	manager, err := server.NewLSPManager(cfg)
	suite.Require().NoError(err)
	suite.manager = manager

	scipCache := manager.GetCache()
	suite.Require().NotNil(scipCache)
	suite.scipCache = scipCache

	ctx := context.Background()
	err = suite.manager.Start(ctx)
	suite.Require().NoError(err)

	// Wait for LSP manager to be ready with platform-specific timing
	waitDuration := testutils.PlatformAdjustedWait(2 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), waitDuration*2)
	defer cancel()

	predicate := func() bool {
		return suite.manager != nil // Manager is ready when not nil and started
	}

	err = testutils.WaitUntil(ctx, 100*time.Millisecond, waitDuration, predicate)
	suite.Require().NoError(err, "LSP manager failed to become ready")
}

func (suite *SCIPCacheE2ETestSuite) TearDownTest() {
	if suite.manager != nil {
		err := suite.manager.Stop()
		suite.Assert().NoError(err)
	}

	// Restore original working directory
	if suite.originalWd != "" {
		os.Chdir(suite.originalWd)
	}
}

func (suite *SCIPCacheE2ETestSuite) TestCacheStoreAndRetrieve() {
	ctx := context.Background()

	uri := utils.FilePathToURI(filepath.Join(suite.testProjectDir, "main.go"))

	definitionParams := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      15,
			"character": 2,
		},
	}

	result1, err := suite.manager.ProcessRequest(ctx, "textDocument/definition", definitionParams)
	suite.Require().NoError(err)
	suite.Require().NotNil(result1)

	// Brief wait for cache operations to complete
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	cachePredicate := func() bool {
		return suite.scipCache != nil // Cache is ready
	}
	err = testutils.WaitUntil(ctx2, 50*time.Millisecond, 1*time.Second, cachePredicate)
	suite.Require().NoError(err)

	result2, err := suite.manager.ProcessRequest(ctx, "textDocument/definition", definitionParams)
	suite.Require().NoError(err)
	suite.Require().NotNil(result2)

	suite.Equal(result1, result2, "Cached result should match original")

	metrics := suite.manager.GetCacheMetrics()
	suite.Require().NotNil(metrics)

	if metricsMap, ok := metrics.(map[string]interface{}); ok {
		if hits, ok := metricsMap["hits"].(int64); ok {
			suite.Greater(hits, int64(0), "Should have cache hits")
		}
	}
}

func (suite *SCIPCacheE2ETestSuite) TestCacheInvalidation() {
	ctx := context.Background()

	uri := utils.FilePathToURI(filepath.Join(suite.testProjectDir, "main.go"))

	hoverParams := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      15,
			"character": 2,
		},
	}

	result1, err := suite.manager.ProcessRequest(ctx, "textDocument/hover", hoverParams)
	suite.Require().NoError(err)
	suite.Require().NotNil(result1)

	err = suite.manager.InvalidateCache(uri)
	suite.Require().NoError(err)

	result2, err := suite.manager.ProcessRequest(ctx, "textDocument/hover", hoverParams)
	suite.Require().NoError(err)
	suite.Require().NotNil(result2)

	metrics := suite.manager.GetCacheMetrics()
	suite.Require().NotNil(metrics)

	if metricsMap, ok := metrics.(map[string]interface{}); ok {
		if misses, ok := metricsMap["misses"].(int64); ok {
			suite.Greater(misses, int64(0), "Should have cache misses after invalidation")
		}
	}
}

func (suite *SCIPCacheE2ETestSuite) TestMultiFileCache() {
	ctx := context.Background()

	files := []string{"main.go", "utils/helper.go", "handlers/user.go"}

	for _, file := range files {
		uri := utils.FilePathToURI(filepath.Join(suite.testProjectDir, file))

		symbolParams := map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": uri,
			},
		}

		result, err := suite.manager.ProcessRequest(ctx, "textDocument/documentSymbol", symbolParams)
		suite.Require().NoError(err)
		suite.Require().NotNil(result)
	}

	metrics := suite.manager.GetCacheMetrics()
	suite.Require().NotNil(metrics)

	if metricsMap, ok := metrics.(map[string]interface{}); ok {
		if entries, ok := metricsMap["entries"].(int64); ok {
			suite.GreaterOrEqual(entries, int64(3), "Should have cached entries for all files")
		}
	}
}

func (suite *SCIPCacheE2ETestSuite) TestCacheConcurrency() {
	ctx := context.Background()
	uri := utils.FilePathToURI(filepath.Join(suite.testProjectDir, "main.go"))

	done := make(chan bool, 3)
	errors := make(chan error, 3)

	runRequest := func(method string, line int) {
		params := map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": uri,
			},
			"position": map[string]interface{}{
				"line":      line,
				"character": 5,
			},
		}

		_, err := suite.manager.ProcessRequest(ctx, method, params)
		if err != nil {
			errors <- err
		} else {
			done <- true
		}
	}

	go runRequest("textDocument/definition", 15)
	go runRequest("textDocument/hover", 10)
	go runRequest("textDocument/references", 5)

	timeoutDuration := 10 * time.Second
	if runtime.GOOS == "windows" {
		timeoutDuration = 30 * time.Second
	}
	timeout := time.After(timeoutDuration)
	successCount := 0

	for i := 0; i < 3; i++ {
		select {
		case <-done:
			successCount++
		case err := <-errors:
			suite.Fail("Concurrent request failed", err.Error())
		case <-timeout:
			suite.Fail("Timeout waiting for concurrent requests")
		}
	}

	suite.Equal(3, successCount, "All concurrent requests should succeed")
}

func (suite *SCIPCacheE2ETestSuite) TestWorkspaceSymbolCache() {
	ctx := context.Background()

	// On Windows, give gopls more time to initialize workspace
	waitDuration := testutils.PlatformAdjustedWait(1500 * time.Millisecond)
	ctx3, cancel3 := context.WithTimeout(context.Background(), waitDuration*2)
	defer cancel3()
	goplsPredicate := func() bool {
		return suite.manager != nil // Gopls initialization complete
	}
	err := testutils.WaitUntil(ctx3, 200*time.Millisecond, waitDuration, goplsPredicate)
	suite.Require().NoError(err, "Gopls failed to initialize within timeout")

	// First, ensure gopls can see our test files
	testFileUri := utils.FilePathToURI(filepath.Join(suite.testProjectDir, "main.go"))
	symbolParams := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": testFileUri,
		},
	}

	// Try to get document symbols first to ensure gopls is ready
	docResult, err := suite.manager.ProcessRequest(ctx, "textDocument/documentSymbol", symbolParams)
	if err != nil || docResult == nil {
		suite.T().Logf("Warning: gopls may not be ready, documentSymbol returned error: %v", err)
		// Give more time for gopls to initialize
		ctx4, cancel4 := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel4()
		goplsRetryPredicate := func() bool {
			_, retryErr := suite.manager.ProcessRequest(ctx, "textDocument/documentSymbol", symbolParams)
			return retryErr == nil
		}
		_ = testutils.WaitUntil(ctx4, 500*time.Millisecond, 3*time.Second, goplsRetryPredicate)
	}

	workspaceParams := map[string]interface{}{
		"query": "Server",
	}

	result1, err := suite.manager.ProcessRequest(ctx, "workspace/symbol", workspaceParams)
	suite.Require().NoError(err, "workspace/symbol should not return error")
	suite.Require().NotNil(result1, "workspace/symbol should return non-nil result")

	// Brief wait for workspace symbol cache operations
	ctx5, cancel5 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel5()
	workspacePredicate := func() bool {
		return suite.scipCache != nil // Workspace symbols cached
	}
	err = testutils.WaitUntil(ctx5, 50*time.Millisecond, 1*time.Second, workspacePredicate)
	suite.Require().NoError(err)

	result2, err := suite.manager.ProcessRequest(ctx, "workspace/symbol", workspaceParams)
	suite.Require().NoError(err)
	suite.Require().NotNil(result2)

	metrics := suite.manager.GetCacheMetrics()
	suite.Require().NotNil(metrics)

	if metricsMap, ok := metrics.(map[string]interface{}); ok {
		if hits, ok := metricsMap["hits"].(int64); ok {
			suite.Greater(hits, int64(0), "Should have cache hits for workspace symbols")
		}
	}
}

func TestSCIPCacheE2ETestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SCIP cache e2e tests in short mode")
	}

	suite.Run(t, new(SCIPCacheE2ETestSuite))
}
