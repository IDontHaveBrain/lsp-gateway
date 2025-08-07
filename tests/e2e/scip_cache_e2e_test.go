package e2e_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server"
	"lsp-gateway/src/server/cache"

	"github.com/stretchr/testify/suite"
)

type SCIPCacheE2ETestSuite struct {
	suite.Suite
	tempDir        string
	cacheDir       string
	testProjectDir string
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
			BackgroundIndex:    true,
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

	time.Sleep(2 * time.Second)
}

func (suite *SCIPCacheE2ETestSuite) TearDownTest() {
	if suite.manager != nil {
		err := suite.manager.Stop()
		suite.Assert().NoError(err)
	}
}

func (suite *SCIPCacheE2ETestSuite) TestCacheStoreAndRetrieve() {
	ctx := context.Background()

	uri := common.FilePathToURI(filepath.Join(suite.testProjectDir, "main.go"))

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

	time.Sleep(100 * time.Millisecond)

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

	uri := common.FilePathToURI(filepath.Join(suite.testProjectDir, "main.go"))

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
		uri := common.FilePathToURI(filepath.Join(suite.testProjectDir, file))

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
	uri := common.FilePathToURI(filepath.Join(suite.testProjectDir, "main.go"))

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

	timeout := time.After(10 * time.Second)
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

	workspaceParams := map[string]interface{}{
		"query": "Server",
	}

	result1, err := suite.manager.ProcessRequest(ctx, "workspace/symbol", workspaceParams)
	suite.Require().NoError(err)
	suite.Require().NotNil(result1)

	time.Sleep(100 * time.Millisecond)

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
