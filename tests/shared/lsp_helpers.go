package shared

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server"
	"lsp-gateway/src/server/cache"
	"lsp-gateway/src/tests/shared/testconfig"

	"github.com/stretchr/testify/require"
)

// LSPManagerSetup holds the LSP manager and related components for testing
type LSPManagerSetup struct {
	Manager *server.LSPManager
	Config  *config.Config
	Cache   *cache.SCIPCacheManager
	Context context.Context
	Cancel  context.CancelFunc
	Started bool
}

// CreateBasicConfig creates a basic LSP configuration
func CreateBasicConfig() *config.Config {
	return testconfig.NewBasicGoConfig()
}

// CreateConfigWithCache creates an LSP configuration with cache enabled
func CreateConfigWithCache(cacheConfig *config.CacheConfig) *config.Config {
	cfg := CreateBasicConfig()
	cfg.Cache = cacheConfig
	return cfg
}

// CreateMultiLangConfig creates a configuration for multiple languages
func CreateMultiLangConfig() *config.Config {
	return testconfig.NewMultiLangConfig([]string{"go", "python", "typescript"})
}

// CreateLSPManagerSetup creates and initializes an LSP manager setup
func CreateLSPManagerSetup(t *testing.T, cfg *config.Config) *LSPManagerSetup {
	if cfg == nil {
		cfg = CreateBasicConfig()
	}

	manager, err := server.NewLSPManager(cfg)
	require.NoError(t, err, "Failed to create LSP manager")
	require.NotNil(t, manager, "LSP manager should not be nil")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	setup := &LSPManagerSetup{
		Manager: manager,
		Config:  cfg,
		Context: ctx,
		Cancel:  cancel,
		Started: false,
	}

	// Create cache if enabled
	if cfg.Cache != nil && cfg.Cache.Enabled {
		cacheManager, err := cache.NewSCIPCacheManager(cfg.Cache)
		if err == nil {
			setup.Cache = cacheManager
			manager.SetCache(cacheManager)
		} else {
			t.Logf("Warning: Failed to create cache manager: %v", err)
		}
	}

	return setup
}

// StartLSPManager starts the LSP manager and cache (if present)
func (setup *LSPManagerSetup) Start(t *testing.T) {
	if setup.Cache != nil {
		err := setup.Cache.Start(setup.Context)
		require.NoError(t, err, "Failed to start cache manager")
	}

	err := setup.Manager.Start(setup.Context)
	require.NoError(t, err, "Failed to start LSP manager")
	
	setup.Started = true
	// Poll for at least one active client or exit quickly if none configured
	waitCtx, cancel := context.WithTimeout(setup.Context, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-waitCtx.Done():
			return
		case <-ticker.C:
			status := setup.Manager.GetClientStatus()
			for _, st := range status {
				if st.Active {
					return
				}
			}
			// If no servers configured, don't block
			if len(status) == 0 {
				return
			}
		}
	}
}

// Stop stops the LSP manager and cache
func (setup *LSPManagerSetup) Stop() {
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

// ProcessRequest processes an LSP request through the manager
func (setup *LSPManagerSetup) ProcessRequest(t *testing.T, method string, params interface{}) interface{} {
	result, err := setup.Manager.ProcessRequest(setup.Context, method, params)
	require.NoError(t, err, "Failed to process LSP request: %s", method)
	return result
}

// OpenDocument opens a document in the LSP manager
func (setup *LSPManagerSetup) OpenDocument(t *testing.T, uri, languageId string, version int, text string) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        uri,
			"languageId": languageId,
			"version":    version,
			"text":       text,
		},
	}
	_, err := setup.Manager.ProcessRequest(setup.Context, "textDocument/didOpen", params)
	if err != nil {
		t.Logf("Warning: didOpen failed: %v", err)
	}
}

// CloseDocument closes a document in the LSP manager
func (setup *LSPManagerSetup) CloseDocument(t *testing.T, uri string) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
	}
	_, _ = setup.Manager.ProcessRequest(setup.Context, "textDocument/didClose", params)
}

// IndexDocument requests document symbols and ensures they are properly indexed into the cache
func (setup *LSPManagerSetup) IndexDocument(t *testing.T, uri string) interface{} {
	t.Logf("IndexDocument: Starting document symbol indexing for %s", uri)

	// Check cache stats before
	if setup.Cache != nil {
		beforeStats := setup.Cache.GetIndexStats()
		if beforeStats != nil {
			t.Logf("IndexDocument: Before - %d documents, %d symbols, %d references",
				beforeStats.DocumentCount, beforeStats.SymbolCount, beforeStats.ReferenceCount)
		}
	}

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
	}

	t.Logf("IndexDocument: Calling ProcessRequest with textDocument/documentSymbol")
	result, err := setup.Manager.ProcessRequest(setup.Context, "textDocument/documentSymbol", params)
	if err != nil {
		t.Errorf("documentSymbol indexing failed: %v", err)
		return nil
	}

	t.Logf("IndexDocument: ProcessRequest succeeded, result type: %T", result)

	// Ensure cache gets populated - wait longer for automatic indexing
	if setup.Cache != nil {
		t.Logf("IndexDocument: Waiting for automatic indexing to complete...")

		// Poll for a short period for symbol count > 0
		pollCtx, cancel := context.WithTimeout(setup.Context, 2*time.Second)
		defer cancel()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-pollCtx.Done():
				goto forceIndex
			case <-ticker.C:
				stats := setup.Cache.GetIndexStats()
				if stats != nil {
					t.Logf("IndexDocument: Cache has %d documents, %d symbols, %d references",
						stats.DocumentCount, stats.SymbolCount, stats.ReferenceCount)
					if stats.SymbolCount > 0 {
						t.Logf("IndexDocument: Success! Symbols indexed automatically")
						return result
					}
				}
			}
		}
forceIndex:
        // If automatic indexing didn't work, try forcing it by calling ProcessRequest again
        t.Logf("IndexDocument: Automatic indexing didn't complete in initial window; trying to force indexing")
		
		// Try calling ProcessRequest again - sometimes the first call doesn't trigger indexing
		t.Logf("IndexDocument: Calling ProcessRequest again to force indexing")
		_, err2 := setup.Manager.ProcessRequest(setup.Context, "textDocument/documentSymbol", params)
		if err2 != nil {
			t.Logf("IndexDocument: Second ProcessRequest failed: %v", err2)
		}
		
		// Poll again after forced request
		pollCtx2, cancel2 := context.WithTimeout(setup.Context, 2*time.Second)
		defer cancel2()
		forcedPoll:
		for {
			select {
			case <-pollCtx2.Done():
				break forcedPoll
			case <-ticker.C:
				finalStats := setup.Cache.GetIndexStats()
				if finalStats != nil && finalStats.SymbolCount > 0 {
					t.Logf("IndexDocument: Success! Second call resulted in %d symbols", finalStats.SymbolCount)
					return result
				}
			}
		}
		finalStats := setup.Cache.GetIndexStats()
		if finalStats != nil && finalStats.SymbolCount > 0 {
			t.Logf("IndexDocument: Success! Second call resulted in %d symbols", finalStats.SymbolCount)
			return result
		}

		// Log result details for debugging
		t.Logf("IndexDocument: Still no symbols after second attempt. Result type: %T, is nil: %v", result, result == nil)
		if result != nil {
			if resultBytes, err := json.Marshal(result); err == nil {
				t.Logf("IndexDocument: Result JSON: %s", string(resultBytes[:min(500, len(resultBytes))]))
			}
		}
	}

	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// WaitForInitialization waits for LSP server initialization
func (setup *LSPManagerSetup) WaitForInitialization() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			status := setup.Manager.GetClientStatus()
			for _, st := range status {
				if st.Active {
					return
				}
			}
			if len(status) == 0 {
				return
			}
		}
	}
}

// CreateHTTPGatewaySetup creates an HTTP gateway setup for testing
type HTTPGatewaySetup struct {
	Gateway   *server.HTTPGateway
	Config    *config.Config
	ServerURL string
	Context   context.Context
	Cancel    context.CancelFunc
	Started   bool
}

// CreateHTTPGateway creates an HTTP gateway for testing
func CreateHTTPGateway(t *testing.T, port string, cfg *config.Config) *HTTPGatewaySetup {
	if cfg == nil {
		cfg = CreateBasicConfig()
	}
	
	if port == "" {
		// Dynamic port binding
		port = ":0"
	}
	
	gateway, err := server.NewHTTPGateway(port, cfg, false)
	require.NoError(t, err, "Failed to create HTTP gateway")
	require.NotNil(t, gateway, "HTTP gateway should not be nil")
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	// ServerURL is initialized after Start when actual port is known
	serverAddr := ""
	
	return &HTTPGatewaySetup{
		Gateway:   gateway,
		Config:    cfg,
		ServerURL: serverAddr,
		Context:   ctx,
		Cancel:    cancel,
		Started:   false,
	}
}

// StartGateway starts the HTTP gateway
func (setup *HTTPGatewaySetup) Start(t *testing.T) {
	err := setup.Gateway.Start(setup.Context)
	require.NoError(t, err, "Failed to start HTTP gateway")
	
	setup.Started = true
	// Determine actual bound port and set server URL
	port := setup.Gateway.Port()
	require.NotEqual(t, 0, port, "Gateway should expose a listening port after start")
	setup.ServerURL = fmt.Sprintf("127.0.0.1:%d", port)
	// Wait for readiness using health endpoint
	ctx, cancel := context.WithTimeout(setup.Context, 10*time.Second)
	defer cancel()
	require.NoError(t, WaitForHTTPReady(ctx, setup.GetHealthURL()), "Gateway did not become ready in time")
}

// Stop stops the HTTP gateway
func (setup *HTTPGatewaySetup) Stop() {
	if setup.Started && setup.Gateway != nil {
		setup.Gateway.Stop()
		setup.Started = false
	}
	if setup.Cancel != nil {
		setup.Cancel()
	}
}

// GetJSONRPCURL returns the JSON-RPC endpoint URL
func (setup *HTTPGatewaySetup) GetJSONRPCURL() string {
	return fmt.Sprintf("http://%s/jsonrpc", setup.ServerURL)
}

// GetHealthURL returns the health endpoint URL
func (setup *HTTPGatewaySetup) GetHealthURL() string {
	return fmt.Sprintf("http://%s/health", setup.ServerURL)
}
 
// WaitForHTTPReady polls the given health URL until it returns HTTP 200 or the context is done.
func WaitForHTTPReady(ctx context.Context, healthURL string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			resp, err := client.Get(healthURL)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return nil
				}
			}
		}
	}
}

// CheckLSPAvailability checks if the required LSP servers are available
func CheckLSPAvailability(t *testing.T, languages ...string) {
	if len(languages) == 0 {
		languages = []string{"go"} // Default to Go
	}

	for _, lang := range languages {
		switch lang {
		case "go":
			_, err := exec.LookPath("gopls")
			if err != nil {
				t.Skip("Go LSP server (gopls) not installed, skipping test")
			}
		case "python":
			_, err := exec.LookPath("jedi-language-server")
			if err != nil {
				t.Skip("Python LSP server (jedi-language-server) not installed, skipping test")
			}
		case "typescript", "javascript":
			_, err := exec.LookPath("typescript-language-server")
			if err != nil {
				t.Skip("TypeScript LSP server not installed, skipping test")
			}
		case "java":
			_, err := exec.LookPath("jdtls")
			if err != nil {
				t.Skip("Java LSP server (jdtls) not installed, skipping test")
			}
		}
	}
}

// SkipIfShortMode skips the test if running in short mode
func SkipIfShortMode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
}

// WaitForServer waits for a server to become ready
func WaitForServer(duration time.Duration) {
	if duration == 0 {
		duration = 3 * time.Second
	}
	time.Sleep(duration)
}

// CreateLSPManagerWithCache creates an LSP manager with cache configuration
func CreateLSPManagerWithCache(t *testing.T, tempDir string) *LSPManagerSetup {
	cacheConfig := CreateBasicCacheConfig(tempDir)
	cfg := CreateConfigWithCache(cacheConfig)
	setup := CreateLSPManagerSetup(t, cfg)
	return setup
}

// CreateBasicLSPManager creates a basic LSP manager without cache
func CreateBasicLSPManager(t *testing.T) *LSPManagerSetup {
	cfg := CreateBasicConfig()
	setup := CreateLSPManagerSetup(t, cfg)
	return setup
}
