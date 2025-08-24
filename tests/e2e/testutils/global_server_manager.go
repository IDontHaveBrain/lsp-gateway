package testutils

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"lsp-gateway/src/tests/shared/testconfig"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type MultiServerManager struct {
	mu sync.RWMutex
	// Repositories
	baseDir string
	repos   map[string]string // language -> repoDir
	rm      *RepoManager
	// Cache/Config
	cacheIsolationMgr *CacheIsolationManager
	configPath        string
	// Server process
	gatewayCmd  *exec.Cmd
	gatewayPort int
	httpClient  *HttpClient
	started     bool
	// CI-persistent base dir
	persistentBase bool
}

var (
	globalMgrMu sync.RWMutex
	globalMgr   *MultiServerManager
)

// InitGlobalServer clones all language repos, generates a single config with per-language working_dir,
// and starts one lsp-gateway server for the entire e2e package.
func InitGlobalServer() error {
	globalMgrMu.Lock()
	defer globalMgrMu.Unlock()
	if globalMgr != nil && globalMgr.started {
		return nil
	}
	// Choose persistent base dir in CI, temporary otherwise
	var baseDir string
	var err error
	isCI := os.Getenv("GITHUB_ACTIONS") == "true" || os.Getenv("CI") == "true"
	if isCI {
		baseDir = filepath.Join(os.Getenv("HOME"), ".lsp-gateway", "e2e-repos-global")
		if mkErr := os.MkdirAll(baseDir, 0o755); mkErr != nil {
			return fmt.Errorf("failed to create global base dir: %w", mkErr)
		}
	} else {
		baseDir, err = os.MkdirTemp("", "lsp-gateway-e2e-global")
		if err != nil {
			return fmt.Errorf("failed to create global base dir: %w")
		}
	}
	rm := NewRepoManager(baseDir)
	// Prepare repos for all languages defined in test repositories
	reposConfig := GetTestRepositories()
	repos := make(map[string]string, len(reposConfig))
	for lang := range reposConfig {
		dir, err := rm.SetupRepository(lang)
		if err != nil {
			return fmt.Errorf("setup repo for %s: %w", lang, err)
		}
		repos[lang] = dir
	}
	// Create cache isolation mgr
	isoCfg := DefaultCacheIsolationConfig()
	isoCfg.IsolationLevel = BasicIsolation
	isoCfg.MaxCacheSize = 256 * 1024 * 1024 // 256MB
	isoCfg.BackgroundIndexing = false
	cacheMgr, err := NewCacheIsolationManager(baseDir, isoCfg)
	if err != nil {
		return fmt.Errorf("cache mgr: %w", err)
	}
	// Build servers map with per-language working dir
	pythonCmd := "jedi-language-server"
	pythonArgs := []string{}
	if cmd, args, ok := detectAvailablePythonLSP(); ok {
		pythonCmd, pythonArgs = cmd, args
	}
	servers := map[string]interface{}{}
	// go
	if dir, ok := repos["go"]; ok {
		servers["go"] = map[string]interface{}{
			"command":     "gopls",
			"args":        []string{"serve"},
			"working_dir": dir,
		}
	}
	// python
	if dir, ok := repos["python"]; ok {
		servers["python"] = map[string]interface{}{
			"command":     pythonCmd,
			"args":        pythonArgs,
			"working_dir": dir,
		}
	}
	// javascript
	if dir, ok := repos["javascript"]; ok {
		servers["javascript"] = map[string]interface{}{
			"command":     "typescript-language-server",
			"args":        []string{"--stdio"},
			"working_dir": dir,
		}
	}
	// typescript
	if dir, ok := repos["typescript"]; ok {
		servers["typescript"] = map[string]interface{}{
			"command":     "typescript-language-server",
			"args":        []string{"--stdio"},
			"working_dir": dir,
		}
	}
	// java
	if dir, ok := repos["java"]; ok {
		javaWorkspace := filepath.Join(os.Getenv("HOME"), ".lsp-gateway", "jdtls-workspaces", fmt.Sprintf("%s-%x", filepath.Base(dir), md5.Sum([]byte(dir))))
		_ = os.MkdirAll(javaWorkspace, 0o755)
		servers["java"] = map[string]interface{}{
			"command":     "~/.lsp-gateway/tools/java/bin/jdtls",
			"args":        []string{javaWorkspace},
			"working_dir": dir,
		}
	}
	// rust
	if dir, ok := repos["rust"]; ok {
		servers["rust"] = map[string]interface{}{
			"command":     "rust-analyzer",
			"args":        []string{},
			"working_dir": dir,
		}
	}
	// csharp
	if dir, ok := repos["csharp"]; ok {
		servers["csharp"] = map[string]interface{}{
			"command":     "omnisharp",
			"args":        []string{"-lsp"},
			"working_dir": dir,
		}
	}
	// kotlin
	if dir, ok := repos["kotlin"]; ok {
		servers["kotlin"] = map[string]interface{}{
			"command":     testconfig.NewKotlinServerConfig().Command,
			"args":        []string{},
			"working_dir": dir,
		}
	}
	configPath, err := cacheMgr.GenerateIsolatedConfig(servers, isoCfg)
	if err != nil {
		return fmt.Errorf("generate config: %w", err)
	}
	// Find binary
	pwd, _ := os.Getwd()
	projectRoot := filepath.Dir(filepath.Dir(pwd)) // tests/e2e -> project root
	bin := "lsp-gateway"
	if runtime.GOOS == "windows" {
		bin = "lsp-gateway.exe"
	}
	binaryPath := filepath.Join(projectRoot, "bin", bin)
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return fmt.Errorf("lsp-gateway binary not found at %s. Run 'make local' first", binaryPath)
	}
	// Find available port
	port, err := FindAvailablePort()
	if err != nil {
		return fmt.Errorf("port: %w", err)
	}
	// Start server (cwd can be baseDir; working_dir per server is set)
	cmd := exec.Command(binaryPath, "server", "--config", configPath, "--port", fmt.Sprintf("%d", port))
	cmd.Dir = baseDir
	cmd.Env = append(os.Environ(),
		"GO111MODULE=on",
		fmt.Sprintf("GOPATH=%s", os.Getenv("GOPATH")),
	)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start server: %w", err)
	}
	// Wait for readiness
	baseURL := fmt.Sprintf("http://localhost:%d", port)
	healthURL := baseURL + "/health"
	if err := waitAllOrHealthy(healthURL, 240*time.Second); err != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return fmt.Errorf("server ready: %w", err)
	}
	// HTTP client
	httpClient := NewHttpClient(HttpClientConfig{BaseURL: baseURL, Timeout: 120 * time.Second})
	globalMgr = &MultiServerManager{
		baseDir:           baseDir,
		repos:             repos,
		rm:                rm,
		cacheIsolationMgr: cacheMgr,
		configPath:        configPath,
		gatewayCmd:        cmd,
		gatewayPort:       port,
		httpClient:        httpClient,
		started:           true,
		persistentBase:    isCI,
	}
	return nil
}

// waitAllOrHealthy waits until the health endpoint is reachable and at least one LSP client is active.
func waitAllOrHealthy(healthURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(healthURL)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		var health map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&health)
		resp.Body.Close()
		lspClients, ok := health["lsp_clients"].(map[string]interface{})
		if !ok {
			time.Sleep(1 * time.Second)
			continue
		}
		for lang, v := range lspClients {
			if m, ok := v.(map[string]interface{}); ok {
				if active, _ := m["Active"].(bool); active {
					_ = lang
					return nil
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("timeout waiting for server health")
}
func IsGlobalServerRunning() bool {
	globalMgrMu.RLock()
	defer globalMgrMu.RUnlock()
	return globalMgr != nil && globalMgr.started
}
func GetGlobalHTTPClient() *HttpClient {
	globalMgrMu.RLock()
	defer globalMgrMu.RUnlock()
	if globalMgr == nil {
		return nil
	}
	return globalMgr.httpClient
}
func GetGlobalServerPort() int {
	globalMgrMu.RLock()
	defer globalMgrMu.RUnlock()
	if globalMgr == nil {
		return 0
	}
	return globalMgr.gatewayPort
}

// GetGlobalRepoDir returns the repo dir for a language, if available.
func GetGlobalRepoDir(language string) (string, bool) {
	globalMgrMu.RLock()
	defer globalMgrMu.RUnlock()
	if globalMgr == nil {
		return "", false
	}
	dir, ok := globalMgr.repos[language]
	return dir, ok
}
func GetGlobalReposBaseDir() string {
	globalMgrMu.RLock()
	defer globalMgrMu.RUnlock()
	if globalMgr == nil {
		return ""
	}
	return globalMgr.baseDir
}

// ShutdownGlobalServer stops the global server and cleans up.
func ShutdownGlobalServer() {
	globalMgrMu.Lock()
	defer globalMgrMu.Unlock()
	if globalMgr == nil {
		return
	}
	if globalMgr.httpClient != nil {
		globalMgr.httpClient.Close()
	}
	if globalMgr.gatewayCmd != nil && globalMgr.gatewayCmd.Process != nil {
		_ = globalMgr.gatewayCmd.Process.Kill()
		_, _ = globalMgr.gatewayCmd.Process.Wait()
	}
	if globalMgr.cacheIsolationMgr != nil {
		_ = globalMgr.cacheIsolationMgr.Cleanup()
	}
	// Keep repos in CI for cache reuse
	if globalMgr.baseDir != "" && !globalMgr.persistentBase {
		_ = os.RemoveAll(globalMgr.baseDir)
	}
	globalMgr = nil
}
