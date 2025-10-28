package testutils

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"

	"lsp-gateway/src/tests/shared/testconfig"
)

// SharedServerManager manages a single LSP gateway server instance across multiple tests
type SharedServerManager struct {
	mu sync.RWMutex

	// Server process management
	gatewayCmd    *exec.Cmd
	gatewayPort   int
	configPath    string
	projectRoot   string
	repoDir       string
	language      string
	serverStarted bool

	// HTTP client for server communication
	httpClient *HttpClient

	// Cache isolation manager for shared server
	cacheIsolationMgr *CacheIsolationManager

	// Test tracking
	testCount   int
	activeTests map[string]bool

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
	// Process exit notification
	procExitedCh chan struct{}
}

// NewSharedServerManager creates a new shared server manager
func NewSharedServerManager(repoDir string, cacheIsolationMgr *CacheIsolationManager, language string) *SharedServerManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &SharedServerManager{
		repoDir:           repoDir,
		language:          language,
		cacheIsolationMgr: cacheIsolationMgr,
		activeTests:       make(map[string]bool),
		ctx:               ctx,
		cancel:            cancel,
	}
}

// StartSharedServer starts the LSP gateway server if not already running
func (mgr *SharedServerManager) StartSharedServer(t *testing.T) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if mgr.serverStarted {
		return nil
	}

	// Find available port
	port, err := FindAvailablePort()
	if err != nil {
		return fmt.Errorf("failed to find available port: %w", err)
	}
	mgr.gatewayPort = port

	// Use a shared cache configuration that can be isolated per test
	cacheConfig := DefaultCacheIsolationConfig()
	cacheConfig.IsolationLevel = BasicIsolation  // Use basic isolation for shared server
	cacheConfig.MaxCacheSize = 256 * 1024 * 1024 // 256MB for shared server
	cacheConfig.BackgroundIndexing = false       // Disable background indexing to reduce LSP contention

	// Get project root and binary path
	pwd, _ := os.Getwd()
	mgr.projectRoot = filepath.Dir(filepath.Dir(pwd))

	binaryName := "lsp-gateway"
	if runtime.GOOS == osWindows {
		binaryName = "lsp-gateway.exe"
	}
	binaryPath := filepath.Join(mgr.projectRoot, "bin", binaryName)

	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return fmt.Errorf("lsp-gateway binary not found at %s. Run 'make local' first", binaryPath)
	}

	// If no Python LSP available, attempt to install basedpyright quickly
	if _, _, ok := detectAvailablePythonLSP(); !ok {
		ctx, cancel := context.WithTimeout(mgr.ctx, 45*time.Second)
		_ = ctx // context passed via CommandContext below
		installCmd := exec.CommandContext(mgr.ctx, binaryPath, "install", "python", "--server", "basedpyright")
		installCmd.Dir = mgr.repoDir
		installCmd.Env = append(os.Environ(),
			"GO111MODULE=on",
			fmt.Sprintf("GOPATH=%s", os.Getenv("GOPATH")),
		)
		if testing.Verbose() || os.Getenv("DEBUG") != "" {
			installCmd.Stdout = os.Stderr
			installCmd.Stderr = os.Stderr
		}
		_ = installCmd.Run()
		cancel()
	}

	// Determine an available Python LSP server (fallback order)
	pythonCmd := "jedi-language-server"
	pythonArgs := []string{}
	if cmd, args, ok := detectAvailablePythonLSP(); ok {
		pythonCmd, pythonArgs = cmd, args
	}

	// Compute per-repo JDTLS workspace to avoid cross-project interference
	javaWorkspace := filepath.Join(os.Getenv("HOME"), ".lsp-gateway", "jdtls-workspaces", fmt.Sprintf("%s-%x", filepath.Base(mgr.repoDir), md5.Sum([]byte(mgr.repoDir))))
	if err := os.MkdirAll(javaWorkspace, 0o755); err != nil {
		return fmt.Errorf("failed to create java workspace: %w", err)
	}

	// Generate shared server config (single language)
	var servers map[string]interface{}
	switch mgr.language {
	case "go":
		servers = map[string]interface{}{"go": map[string]interface{}{"command": "gopls", "args": []string{"serve"}}}
	case "python":
		servers = map[string]interface{}{"python": map[string]interface{}{"command": pythonCmd, "args": pythonArgs}}
	case "javascript":
		servers = map[string]interface{}{"javascript": map[string]interface{}{"command": "typescript-language-server", "args": []string{"--stdio"}}}
	case "typescript":
		servers = map[string]interface{}{"typescript": map[string]interface{}{"command": "typescript-language-server", "args": []string{"--stdio"}}}
	case "java":
		servers = map[string]interface{}{"java": map[string]interface{}{"command": "~/.lsp-gateway/tools/java/bin/jdtls", "args": []string{javaWorkspace}}}
	case "rust":
		servers = map[string]interface{}{"rust": map[string]interface{}{"command": "rust-analyzer", "args": []string{}}}
	case "csharp":
		servers = map[string]interface{}{"csharp": map[string]interface{}{"command": "omnisharp", "args": []string{"-lsp"}}}
	case "kotlin":
		servers = map[string]interface{}{"kotlin": map[string]interface{}{"command": testconfig.NewKotlinServerConfig().Command, "args": []string{}}}
	default:
		servers = map[string]interface{}{"go": map[string]interface{}{"command": "gopls", "args": []string{"serve"}}}
	}

	configPath, err := mgr.cacheIsolationMgr.GenerateIsolatedConfig(servers, cacheConfig)
	if err != nil {
		return fmt.Errorf("failed to generate shared server config: %w", err)
	}
	mgr.configPath = configPath

	// Start the shared server
	cmd := exec.CommandContext(mgr.ctx, binaryPath, "server", "--config", configPath, "--port", fmt.Sprintf("%d", port))
	cmd.Dir = mgr.repoDir
	cmd.Env = append(os.Environ(),
		"GO111MODULE=on",
		fmt.Sprintf("GOPATH=%s", os.Getenv("GOPATH")),
	)

	// Set process group for proper cleanup of child LSP processes
	if runtime.GOOS != osWindows {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
	}

	// Capture output for debugging
	// We'll redirect to os.Stderr so it doesn't interfere with test output
	// but we can still see errors if the server fails
	if testing.Verbose() || os.Getenv("DEBUG") != "" {
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start shared server: %w", err)
	}

	mgr.gatewayCmd = cmd
	mgr.serverStarted = true
	// Watch for early process exit to bail fast
	mgr.procExitedCh = make(chan struct{}, 1)
	go func(c *exec.Cmd, ch chan struct{}) {
		_ = c.Wait()
		select {
		case ch <- struct{}{}:
		default:
		}
	}(mgr.gatewayCmd, mgr.procExitedCh)

	// Create HTTP client for the shared server with timeout long enough for Java LSP server
	// Determine timeout based on environment - should be longer than server's internal timeout
	var httpTimeout time.Duration
	if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" {
		if runtime.GOOS == osWindows {
			httpTimeout = 210 * time.Second
		} else {
			httpTimeout = 120 * time.Second
		}
	} else {
		httpTimeout = 120 * time.Second
	}

	mgr.httpClient = NewHttpClient(HttpClientConfig{
		BaseURL: fmt.Sprintf("http://localhost:%d", port),
		Timeout: httpTimeout,
	})

	// Wait for server to be ready
	if err := mgr.waitForServerReady(t); err != nil {
		if stopErr := mgr.stopSharedServer(t); stopErr != nil {
			t.Logf("shared server cleanup error: %v", stopErr)
		}
		return fmt.Errorf("shared server failed to become ready: %w", err)
	}

	// Double-check that the process is still running
	if mgr.gatewayCmd == nil || mgr.gatewayCmd.Process == nil {
		return fmt.Errorf("shared server process terminated unexpectedly")
	}

	// Try to check if process exited (non-blocking)
	processExited := make(chan bool, 1)
	go func() {
		select {
		case <-time.After(500 * time.Millisecond):
			// Check if process is still running
			if mgr.gatewayCmd.ProcessState != nil {
				processExited <- true
			}
		case <-mgr.ctx.Done():
			return
		}
	}()

	select {
	case <-processExited:
		return fmt.Errorf("shared server process exited immediately after startup")
	case <-time.After(1 * time.Second):
		// Process is still running, continue
		t.Logf("✅ Shared server process is still running after 1 second")
	}

	t.Logf("✅ Shared LSP gateway server is ready and serving")
	return nil
}

// detectAvailablePythonLSP tries to find an available Python LSP server executable
// in preferred order and returns its command and args.
func detectAvailablePythonLSP() (string, []string, bool) {
	type cand struct {
		cmd  string
		args []string
	}
	candidates := []cand{
		{"basedpyright-langserver", []string{"--stdio"}},
		{"pyright-langserver", []string{"--stdio"}},
		{"pylsp", []string{}},
		{"jedi-language-server", []string{"--stdio"}},
	}

	for _, c := range candidates {
		if commandAvailableThroughPythonTools(c.cmd) {
			return c.cmd, c.args, true
		}
	}

	// Fallback to uvx-managed language servers if direct binaries are unavailable.
	if _, err := exec.LookPath("uvx"); err == nil {
		uvxCandidates := []cand{
			{"uvx", []string{"--from", "basedpyright", "basedpyright-langserver", "--", "--stdio"}},
			{"uvx", []string{"--from", "pyright", "pyright-langserver", "--", "--stdio"}},
		}
		for _, c := range uvxCandidates {
			// Ensure uvx itself is runnable before returning it.
			if isCommandStartable("uvx") {
				return c.cmd, c.args, true
			}
		}
	}

	return "", nil, false
}

// commandAvailableThroughPythonTools verifies whether a python language server command
// can be launched either from PATH or from the ~/.lsp-gateway/tools/python directory.
func commandAvailableThroughPythonTools(name string) bool {
	for _, candidate := range pythonCommandPaths(name) {
		if isCommandStartable(candidate) {
			return true
		}
	}
	return false
}

func pythonCommandPaths(name string) []string {
	paths := []string{}
	if path, err := exec.LookPath(name); err == nil && path != "" {
		paths = append(paths, path)
	}
	if home, err := os.UserHomeDir(); err == nil {
		base := filepath.Join(home, ".lsp-gateway", "tools", "python")
		variants := []string{name}
		if runtime.GOOS == osWindows {
			variants = append(variants, name+".cmd", name+".exe")
		}
		for _, variant := range variants {
			p := filepath.Join(base, variant)
			if fi, err2 := os.Stat(p); err2 == nil && !fi.IsDir() {
				paths = append(paths, p)
			}
		}
	}
	return paths
}

func isCommandStartable(path string) bool {
	if path == "" {
		return false
	}
	cmd := exec.Command(path, "--help")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return false
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	case <-done:
	}
	return true
}

// PythonLSPAvailable reports whether a Python language server binary is runnable without uvx.
func PythonLSPAvailable() bool {
	for _, name := range []string{
		"basedpyright-langserver",
		"pyright-langserver",
		"pylsp",
		"jedi-language-server",
	} {
		if commandAvailableThroughPythonTools(name) {
			return true
		}
	}
	return false
}

// RustAnalyzerAvailable reports whether rust-analyzer is installed via rustup.
func RustAnalyzerAvailable() bool {
	if _, err := exec.LookPath("rustup"); err != nil {
		return false
	}
	cmd := exec.Command("rustup", "component", "list", "--installed")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return bytes.Contains(output, []byte("rust-analyzer"))
}

// waitForServerReady waits for the shared server to be ready
func (mgr *SharedServerManager) waitForServerReady(t *testing.T) error {
	healthURL := fmt.Sprintf("http://localhost:%d/health", mgr.gatewayPort)

	t.Logf("⏳ Waiting for shared server to be ready at %s...", healthURL)

	// Allow more time for heavier language servers; extend further on Windows
	maxRetries := 180
	if runtime.GOOS == osWindows {
		maxRetries = 330
	}
	for i := 0; i < maxRetries; i++ {
		// Fail immediately if process already exited
		select {
		case <-mgr.procExitedCh:
			return fmt.Errorf("shared server process exited during startup")
		default:
		}
		select {
		case <-mgr.ctx.Done():
			return fmt.Errorf("context cancelled while waiting for server")
		default:
		}
		// Fast-fail if process already exited
		if mgr.gatewayCmd != nil && mgr.gatewayCmd.ProcessState != nil {
			return fmt.Errorf("shared server process exited during startup")
		}

		// Parse actual health response to check LSP client status
		resp, err := http.Get(healthURL)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		var health map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
			_ = resp.Body.Close()
			time.Sleep(1 * time.Second)
			continue
		}
		_ = resp.Body.Close()
		// Check if required LSP clients are active
		lspClients, ok := health["lsp_clients"].(map[string]interface{})
		if !ok {
			t.Logf("Waiting for LSP clients to initialize...")
			time.Sleep(1 * time.Second)
			continue
		}

		// Check if at least one client is active; if all failed, bail fast
		hasActiveClient := false
		allFailed := true
		for lang, langClient := range lspClients {
			if clientMap, ok := langClient.(map[string]interface{}); ok {
				active, _ := clientMap["Active"].(bool)
				var errStr string
				if ev, ok := clientMap["Error"]; ok && ev != nil {
					switch v := ev.(type) {
					case string:
						errStr = v
					default:
						b, _ := json.Marshal(v)
						errStr = string(b)
					}
				}
				if active {
					hasActiveClient = true
					t.Logf("✅ %s LSP client is active", lang)
				} else if errStr != "" {
					t.Logf("❌ %s LSP client failed: %s", lang, errStr)
				} else {
					allFailed = false
					t.Logf("⏳ %s LSP client is still initializing...", lang)
				}
			}
		}

		if hasActiveClient {
			return nil
		}
		if allFailed && len(lspClients) > 0 {
			return fmt.Errorf("all LSP clients failed to start")
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("shared server failed to become ready after %d seconds", maxRetries)
}

// GetHTTPClient returns the HTTP client for the shared server
func (mgr *SharedServerManager) GetHTTPClient() *HttpClient {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	return mgr.httpClient
}

// GetServerPort returns the port the shared server is running on
func (mgr *SharedServerManager) GetServerPort() int {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	return mgr.gatewayPort
}

// RegisterTest registers a test as using the shared server
func (mgr *SharedServerManager) RegisterTest(testName string, t *testing.T) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	mgr.activeTests[testName] = true
	mgr.testCount++
}

// UnregisterTest unregisters a test from using the shared server
func (mgr *SharedServerManager) UnregisterTest(testName string, t *testing.T) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	delete(mgr.activeTests, testName)
}

// IsServerRunning returns true if the shared server is running
func (mgr *SharedServerManager) IsServerRunning() bool {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	return mgr.serverStarted
}

// StopSharedServer stops the shared LSP gateway server
func (mgr *SharedServerManager) StopSharedServer(t *testing.T) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	return mgr.stopSharedServer(t)
}

// stopSharedServer internal method to stop the server (assumes lock is held)
func (mgr *SharedServerManager) stopSharedServer(t *testing.T) error {
	if !mgr.serverStarted || mgr.gatewayCmd == nil {
		return nil
	}

	// Cancel context to stop any ongoing operations
	mgr.cancel()

	// Close HTTP client
	if mgr.httpClient != nil {
		if err := mgr.httpClient.Close(); err != nil {
			t.Logf("⚠️  Failed to close shared server HTTP client: %v", err)
		}
		mgr.httpClient = nil
	}

	// Gracefully terminate the server and all child LSP processes
	if mgr.gatewayCmd.Process != nil {
		pid := mgr.gatewayCmd.Process.Pid

		// Kill entire process group to ensure all child LSP servers are terminated
		if runtime.GOOS != osWindows {
			// Send SIGTERM to process group (negative PID)
			if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
				t.Logf("⚠️  Failed to send SIGTERM to process group: %v", err)
				// Fallback to killing just the process
				_ = mgr.gatewayCmd.Process.Signal(syscall.SIGTERM)
			}
		} else {
			if err := mgr.gatewayCmd.Process.Signal(syscall.SIGTERM); err != nil {
				t.Logf("⚠️  Failed to send SIGTERM to shared server: %v", err)
			}
		}

		// Wait up to 10 seconds for graceful shutdown
		done := make(chan error, 1)
		go func() {
			done <- mgr.gatewayCmd.Wait()
		}()

		select {
		case <-done:
			// Process exited gracefully
		case <-time.After(10 * time.Second):
			// Force kill process group if still running
			if runtime.GOOS != osWindows {
				if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
					t.Logf("⚠️  Failed to SIGKILL process group: %v", err)
				}
			} else {
				if err := mgr.gatewayCmd.Process.Kill(); err != nil {
					t.Logf("⚠️  Failed to kill shared server process: %v", err)
				}
			}
			if err := mgr.gatewayCmd.Wait(); err != nil {
				t.Logf("⚠️  Shared server wait error after kill: %v", err)
			}
		}
	}

	mgr.gatewayCmd = nil
	mgr.serverStarted = false
	mgr.procExitedCh = nil

	return nil
}

// GetActiveTestCount returns the number of active tests using the shared server
func (mgr *SharedServerManager) GetActiveTestCount() int {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	return len(mgr.activeTests)
}

// GetServerInfo returns information about the shared server
func (mgr *SharedServerManager) GetServerInfo() map[string]interface{} {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	return map[string]interface{}{
		"running":      mgr.serverStarted,
		"port":         mgr.gatewayPort,
		"config_path":  mgr.configPath,
		"project_root": mgr.projectRoot,
		"repo_dir":     mgr.repoDir,
		"active_tests": len(mgr.activeTests),
		"total_tests":  mgr.testCount,
	}
}
