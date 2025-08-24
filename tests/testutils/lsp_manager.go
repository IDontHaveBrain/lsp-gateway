package testutils

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server"
	"lsp-gateway/src/server/cache"
	"lsp-gateway/src/tests/shared/testconfig"
	"lsp-gateway/src/utils"

	"github.com/stretchr/testify/require"
)

// LSPManagerTestSetup provides comprehensive LSP manager testing utilities
type LSPManagerTestSetup struct {
	Manager     *server.LSPManager
	Config      *config.Config
	Cache       cache.SCIPCache
	Context     context.Context
	Cancel      context.CancelFunc
	Started     bool
	TempDir     string
	CleanupFunc func()
}

// LSPManagerBuilder provides a builder pattern for creating LSP manager configurations
type LSPManagerBuilder struct {
	config      *config.Config
	enableCache bool
	cacheConfig *config.CacheConfig
	languages   []string
	tempDir     string
	timeout     time.Duration
}

// NewLSPManagerBuilder creates a new builder for LSP manager configuration
func NewLSPManagerBuilder() *LSPManagerBuilder {
	return &LSPManagerBuilder{
		config:  testconfig.NewBasicGoConfig(),
		timeout: 30 * time.Second,
	}
}

// WithConfig sets the configuration for the LSP manager
func (b *LSPManagerBuilder) WithConfig(cfg *config.Config) *LSPManagerBuilder {
	b.config = cfg
	return b
}

// WithCache enables cache with default configuration
func (b *LSPManagerBuilder) WithCache() *LSPManagerBuilder {
	b.enableCache = true
	return b
}

// WithCacheConfig enables cache with custom configuration
func (b *LSPManagerBuilder) WithCacheConfig(cacheConfig *config.CacheConfig) *LSPManagerBuilder {
	b.enableCache = true
	b.cacheConfig = cacheConfig
	return b
}

// WithLanguages sets the languages for multi-language support
func (b *LSPManagerBuilder) WithLanguages(languages ...string) *LSPManagerBuilder {
	b.languages = languages
	return b
}

// WithTempDir sets a custom temporary directory
func (b *LSPManagerBuilder) WithTempDir(tempDir string) *LSPManagerBuilder {
	b.tempDir = tempDir
	return b
}

// WithTimeout sets the context timeout
func (b *LSPManagerBuilder) WithTimeout(timeout time.Duration) *LSPManagerBuilder {
	b.timeout = timeout
	return b
}

// Build creates the LSP manager test setup
func (b *LSPManagerBuilder) Build(t *testing.T) *LSPManagerTestSetup {
	return b.createSetup(t, false)
}

// BuildAndStart creates and starts the LSP manager test setup
func (b *LSPManagerBuilder) BuildAndStart(t *testing.T) *LSPManagerTestSetup {
	return b.createSetup(t, true)
}

// createSetup creates the actual LSP manager setup
func (b *LSPManagerBuilder) createSetup(t *testing.T, autoStart bool) *LSPManagerTestSetup {
	cfg := b.config
	if cfg == nil {
		cfg = testconfig.NewBasicGoConfig()
	}

	// Handle multi-language configuration
	if len(b.languages) > 0 {
		cfg = testconfig.NewMultiLangConfig(b.languages)
	}

	// Setup temporary directory
	tempDir := b.tempDir
	if tempDir == "" {
		tempDir = t.TempDir()
	}

	// Setup cache configuration if enabled
	if b.enableCache {
		cacheConfig := b.cacheConfig
		if cacheConfig == nil {
			cacheConfig = createDefaultCacheConfig(tempDir)
		}
		cfg.Cache = cacheConfig
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), b.timeout)

	// Create LSP manager
	manager, err := server.NewLSPManager(cfg)
	require.NoError(t, err, "Failed to create LSP manager")
	require.NotNil(t, manager, "LSP manager should not be nil")

	setup := &LSPManagerTestSetup{
		Manager: manager,
		Config:  cfg,
		Context: ctx,
		Cancel:  cancel,
		TempDir: tempDir,
		Started: false,
	}

	// Setup cache if enabled
	if cfg.Cache != nil && cfg.Cache.Enabled {
		cacheManager, err := cache.NewSCIPCacheManager(cfg.Cache)
		if err == nil {
			setup.Cache = cacheManager
			manager.SetCache(cacheManager)
		} else {
			t.Logf("Warning: Failed to create cache manager: %v", err)
		}
	}

	// Setup cleanup
	cleanup := func() {
		setup.Stop()
		if setup.TempDir != "" && setup.TempDir != b.tempDir {
			os.RemoveAll(setup.TempDir)
		}
	}
	setup.CleanupFunc = cleanup
	t.Cleanup(cleanup)

	// Auto-start if requested
	if autoStart {
		setup.Start(t)
	}

	return setup
}

// CreateTestLSPManager creates an LSP manager with proper configuration
func CreateTestLSPManager(t *testing.T, cfg *config.Config) *LSPManagerTestSetup {
	return NewLSPManagerBuilder().WithConfig(cfg).Build(t)
}

// CreateAndStartLSPManager creates and starts LSP manager with lifecycle management
func CreateAndStartLSPManager(t *testing.T, cfg *config.Config) *LSPManagerTestSetup {
	return NewLSPManagerBuilder().WithConfig(cfg).BuildAndStart(t)
}

// CreateLSPManagerWithCache creates LSP manager with cache configuration
func CreateLSPManagerWithCache(t *testing.T, tempDir string) *LSPManagerTestSetup {
	builder := NewLSPManagerBuilder().WithCache()
	if tempDir != "" {
		builder = builder.WithTempDir(tempDir)
	}
	return builder.Build(t)
}

// CreateBasicLSPManager creates a basic LSP manager without cache
func CreateBasicLSPManager(t *testing.T) *LSPManagerTestSetup {
	return NewLSPManagerBuilder().Build(t)
}

// CreateMultiLangLSPManager creates manager for multiple languages
func CreateMultiLangLSPManager(t *testing.T, languages []string) *LSPManagerTestSetup {
	return NewLSPManagerBuilder().WithLanguages(languages...).Build(t)
}

// Start starts the LSP manager and cache (if present)
func (setup *LSPManagerTestSetup) Start(t *testing.T) {
	if setup.Started {
		return
	}

	// Start cache first if present
	if setup.Cache != nil {
		err := setup.Cache.Start(setup.Context)
		require.NoError(t, err, "Failed to start cache manager")
	}

	// Start LSP manager
	err := setup.Manager.Start(setup.Context)
	require.NoError(t, err, "Failed to start LSP manager")

	setup.Started = true

	// Wait for active LSP client(s) if any
	WaitForLSPManagerReady(t, setup.Manager, 10*time.Second)
}

// Stop stops the LSP manager and cache
func (setup *LSPManagerTestSetup) Stop() {
	if setup.Started {
		if setup.Manager != nil {
			setup.Manager.Stop()
		}
		if setup.Cache != nil {
			setup.Cache.Stop()
		}
		setup.Started = false
	}
	if setup.Cancel != nil {
		setup.Cancel()
	}
}

// WaitForLSPManagerReady waits for manager initialization with timeout
func WaitForLSPManagerReady(t *testing.T, manager *server.LSPManager, timeout time.Duration) {
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Check if any clients are available and active
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("Timeout waiting for LSP manager to become ready after %v", timeout)
		case <-ticker.C:
			status := manager.GetClientStatus()
			hasActiveClient := false
			for _, clientStatus := range status {
				if clientStatus.Active {
					hasActiveClient = true
					break
				}
			}
			if hasActiveClient {
				return
			}
		}
	}
}

// ProcessLSPRequest processes an LSP request through the manager
func ProcessLSPRequest(t *testing.T, manager *server.LSPManager, method string, params interface{}) interface{} {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := manager.ProcessRequest(ctx, method, params)
	require.NoError(t, err, "Failed to process LSP request: %s", method)
	return result
}

// OpenTestDocument opens a document in the LSP manager
func OpenTestDocument(t *testing.T, manager *server.LSPManager, uri, languageId string, version int, text string) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        uri,
			"languageId": languageId,
			"version":    version,
			"text":       text,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := manager.ProcessRequest(ctx, "textDocument/didOpen", params)
	if err != nil {
		t.Logf("Warning: didOpen failed: %v", err)
	}
}

// CloseTestDocument closes a document in the LSP manager
func CloseTestDocument(t *testing.T, manager *server.LSPManager, uri string) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, _ = manager.ProcessRequest(ctx, "textDocument/didClose", params)
}

// IndexTestDocument requests document symbols and ensures they are properly indexed
func IndexTestDocument(t *testing.T, manager *server.LSPManager, uri string) interface{} {
	t.Logf("IndexTestDocument: Starting document symbol indexing for %s", uri)

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := manager.ProcessRequest(ctx, "textDocument/documentSymbol", params)
	require.NoError(t, err, "documentSymbol indexing failed for %s", uri)

	// Wait for indexing to complete
	WaitForIndexing(t, manager, 5*time.Second)

	return result
}

// WaitForIndexing waits for indexing completion with timeout
func WaitForIndexing(t *testing.T, manager *server.LSPManager, timeout time.Duration) {
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	// Poll for index stats to show ready or some activity
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if stats := manager.GetIndexStats(); stats != nil {
				switch s := stats.(type) {
				case *cache.IndexStats:
					if s.Status == "ready" || s.SymbolCount > 0 {
						return
					}
				case map[string]interface{}:
					if st, ok := s["status"].(string); ok && st == "ready" {
						return
					}
					// Optional numeric check if provided in map form
					if v, ok := s["symbol_count"]; ok {
						switch val := v.(type) {
						case int:
							if val > 0 {
								return
							}
						case int64:
							if val > 0 {
								return
							}
						case float64:
							if val > 0 {
								return
							}
						}
					}
				}
			}
		}
	}
}

// CreateTestFile creates a test file with the given content and returns its URI
func (setup *LSPManagerTestSetup) CreateTestFile(t *testing.T, filename, content string) string {
	filePath := filepath.Join(setup.TempDir, filename)

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	err := os.MkdirAll(dir, 0755)
	require.NoError(t, err, "Failed to create directory: %s", dir)

	// Write file content
	err = os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err, "Failed to write test file: %s", filePath)

	return utils.FilePathToURI(filePath)
}

// ValidateResponse validates that a response is not nil and properly formatted
func (setup *LSPManagerTestSetup) ValidateResponse(t *testing.T, method string, response interface{}) {
	require.NotNil(t, response, "Response should not be nil for method: %s", method)

	// Log response type for debugging
	t.Logf("Response for %s: type=%T", method, response)

	// Validate JSON marshaling
	if jsonData, err := json.Marshal(response); err == nil {
		t.Logf("Response for %s is valid JSON (length: %d)", method, len(jsonData))
	} else {
		t.Errorf("Response for %s failed JSON marshaling: %v", method, err)
	}
}

// GetCacheStats returns cache statistics if cache is available
func (setup *LSPManagerTestSetup) GetCacheStats(t *testing.T) map[string]interface{} {
	if setup.Cache == nil {
		t.Log("Cache not available for statistics")
		return nil
	}

	// Try to get stats from cache manager
	if cacheManager, ok := setup.Cache.(*cache.SCIPCacheManager); ok {
		if stats := cacheManager.GetIndexStats(); stats != nil {
			return map[string]interface{}{
				"documentCount":  stats.DocumentCount,
				"symbolCount":    stats.SymbolCount,
				"referenceCount": stats.ReferenceCount,
			}
		}
	}

	return nil
}

// WaitForCachePopulation waits for cache to be populated with data
func (setup *LSPManagerTestSetup) WaitForCachePopulation(t *testing.T, timeout time.Duration) bool {
	if setup.Cache == nil {
		return false
	}

	if timeout == 0 {
		timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Logf("Timeout waiting for cache population after %v", timeout)
			return false
		case <-ticker.C:
			if stats := setup.GetCacheStats(t); stats != nil {
				if symbolCount, ok := stats["symbolCount"].(int); ok && symbolCount > 0 {
					t.Logf("Cache populated with %d symbols", symbolCount)
					return true
				}
			}
		}
	}
}

// CreateGoTestFile creates a standard Go test file with common patterns
func (setup *LSPManagerTestSetup) CreateGoTestFile(t *testing.T, packageName string) string {
	content := fmt.Sprintf(`package %s

import (
	"fmt"
	"strings"
)

type TestStruct struct {
	Name  string
	Value int
}

func main() {
	ts := TestStruct{
		Name:  "test",
		Value: 42,
	}
	fmt.Println(ts.Name)
	result := processData(ts)
	fmt.Println(result)
}

func processData(data TestStruct) string {
	return strings.ToUpper(data.Name)
}

func helperFunction() {
	fmt.Println("Helper")
}
`, packageName)

	return setup.CreateTestFile(t, "test.go", content)
}

// CreatePythonTestFile creates a standard Python test file
func (setup *LSPManagerTestSetup) CreatePythonTestFile(t *testing.T) string {
	content := `def main():
    print("Hello, World!")
    
def helper_function():
    return "helper"

class TestClass:
    def __init__(self, name):
        self.name = name
    
    def get_name(self):
        return self.name

if __name__ == "__main__":
    main()
`
	return setup.CreateTestFile(t, "test.py", content)
}

// CreateTypeScriptTestFile creates a standard TypeScript test file
func (setup *LSPManagerTestSetup) CreateTypeScriptTestFile(t *testing.T) string {
	content := `interface TestInterface {
    name: string;
    value: number;
}

class TestClass implements TestInterface {
    name: string;
    value: number;

    constructor(name: string, value: number) {
        this.name = name;
        this.value = value;
    }

    getName(): string {
        return this.name;
    }
}

function processData(data: TestInterface): string {
    return data.name.toUpperCase();
}

const testInstance = new TestClass("test", 42);
console.log(processData(testInstance));
`
	return setup.CreateTestFile(t, "test.ts", content)
}

// CreateKotlinTestFile creates a standard Kotlin test file
func (setup *LSPManagerTestSetup) CreateKotlinTestFile(t *testing.T) string {
	content := `package com.example.test

import kotlinx.coroutines.*

data class TestData(
    val name: String,
    val value: Int
)

class TestClass(private val data: TestData) {
    fun getName(): String = data.name
    
    fun processData(): String {
        return data.name.uppercase()
    }
    
    suspend fun asyncOperation(): String = withContext(Dispatchers.IO) {
        delay(100)
        "Processed: ${data.name}"
    }
}

fun main() {
    val testData = TestData("test", 42)
    val testInstance = TestClass(testData)
    println(testInstance.processData())
    
    runBlocking {
        println(testInstance.asyncOperation())
    }
}

fun helperFunction(input: String): String {
    return input.lowercase()
}
`
	return setup.CreateTestFile(t, "test.kt", content)
}

// createDefaultCacheConfig creates a default cache configuration for testing
func createDefaultCacheConfig(tempDir string) *config.CacheConfig {
	cacheDir := filepath.Join(tempDir, "lsp-cache")
	return &config.CacheConfig{
		Enabled:         true,
		StoragePath:     cacheDir,
		MaxMemoryMB:     64,
		TTLHours:        1,
		Languages:       []string{"go", "python", "typescript", "javascript", "kotlin"},
		BackgroundIndex: false,
		DiskCache:       true,
		EvictionPolicy:  "lru",
	}
}

// TestLSPOperations contains common LSP operation test patterns
type TestLSPOperations struct {
	setup *LSPManagerTestSetup
}

// NewTestLSPOperations creates a new test operations helper
func NewTestLSPOperations(setup *LSPManagerTestSetup) *TestLSPOperations {
	return &TestLSPOperations{setup: setup}
}

// TestDefinition tests textDocument/definition operation
func (ops *TestLSPOperations) TestDefinition(t *testing.T, uri string, line, character int) interface{} {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": character,
		},
	}

	result := ProcessLSPRequest(t, ops.setup.Manager, "textDocument/definition", params)
	ops.setup.ValidateResponse(t, "textDocument/definition", result)
	return result
}

// TestReferences tests textDocument/references operation
func (ops *TestLSPOperations) TestReferences(t *testing.T, uri string, line, character int) interface{} {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": character,
		},
		"context": map[string]interface{}{
			"includeDeclaration": true,
		},
	}

	result := ProcessLSPRequest(t, ops.setup.Manager, "textDocument/references", params)
	ops.setup.ValidateResponse(t, "textDocument/references", result)
	return result
}

// TestHover tests textDocument/hover operation
func (ops *TestLSPOperations) TestHover(t *testing.T, uri string, line, character int) interface{} {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": character,
		},
	}

	result := ProcessLSPRequest(t, ops.setup.Manager, "textDocument/hover", params)
	ops.setup.ValidateResponse(t, "textDocument/hover", result)
	return result
}

// TestDocumentSymbol tests textDocument/documentSymbol operation
func (ops *TestLSPOperations) TestDocumentSymbol(t *testing.T, uri string) interface{} {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
	}

	result := ProcessLSPRequest(t, ops.setup.Manager, "textDocument/documentSymbol", params)
	ops.setup.ValidateResponse(t, "textDocument/documentSymbol", result)
	return result
}

// TestWorkspaceSymbol tests workspace/symbol operation
func (ops *TestLSPOperations) TestWorkspaceSymbol(t *testing.T, query string) interface{} {
	params := map[string]interface{}{
		"query": query,
	}

	result := ProcessLSPRequest(t, ops.setup.Manager, "workspace/symbol", params)
	ops.setup.ValidateResponse(t, "workspace/symbol", result)
	return result
}

// TestCompletion tests textDocument/completion operation
func (ops *TestLSPOperations) TestCompletion(t *testing.T, uri string, line, character int) interface{} {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": character,
		},
	}

	result := ProcessLSPRequest(t, ops.setup.Manager, "textDocument/completion", params)
	ops.setup.ValidateResponse(t, "textDocument/completion", result)
	return result
}
