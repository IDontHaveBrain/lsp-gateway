package testutils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// IsolatedTestSetup provides a complete isolated test environment
type IsolatedTestSetup struct {
	TestID          string
	Resources       *TestResources
	ConfigPath      string
	ServerProcess   *exec.Cmd
	ServerCancel    context.CancelFunc
	resourceManager *ResourceManager
	cleanup         []func() error
	mu              sync.Mutex
}

// SetupIsolatedTest creates a complete isolated test environment
func SetupIsolatedTest(testName string) (*IsolatedTestSetup, error) {
	// Create per-test resource manager for proper isolation
	rm := NewResourceManager()

	// Generate unique test ID
	testID := fmt.Sprintf("%s_%d", testName, time.Now().UnixNano())

	// Allocate resources
	resources, err := rm.AllocateTestResources(testID)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate test resources: %w", err)
	}

	setup := &IsolatedTestSetup{
		TestID:          testID,
		Resources:       resources,
		resourceManager: rm,
		cleanup:         make([]func() error, 0),
	}

	// Add resource cleanup
	setup.addCleanupTask(func() error {
		return rm.ReleaseTestResources(testID)
	})

	// Add resource manager cleanup
	setup.addCleanupTask(func() error {
		return rm.Cleanup()
	})

	return setup, nil
}

// SetupIsolatedTestWithLanguage creates an isolated test environment with a specific language config
func SetupIsolatedTestWithLanguage(testName, language string) (*IsolatedTestSetup, error) {
	setup, err := SetupIsolatedTest(testName)
	if err != nil {
		return nil, err
	}

	// Create language-specific config
	configContent, err := setup.createLanguageConfig(language)
	if err != nil {
		setup.Cleanup()
		return nil, fmt.Errorf("failed to create language config: %w", err)
	}

	// Write config file
	configPath, err := setup.Resources.Directory.CreateTempFile("config.yaml", configContent)
	if err != nil {
		setup.Cleanup()
		return nil, fmt.Errorf("failed to write config file: %w", err)
	}

	setup.ConfigPath = configPath
	return setup, nil
}

// StartServer starts an isolated server instance
func (its *IsolatedTestSetup) StartServer(serverCommand []string, timeout time.Duration) error {
	its.mu.Lock()
	defer its.mu.Unlock()

	if its.ServerProcess != nil {
		return fmt.Errorf("server already started")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	its.ServerCancel = cancel

	// Create command with isolated environment
	cmd := exec.CommandContext(ctx, serverCommand[0], serverCommand[1:]...)
	cmd.Dir = its.Resources.Directory.Path
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("TEST_PORT=%d", its.Resources.Port),
		fmt.Sprintf("TEST_DIR=%s", its.Resources.Directory.Path),
		fmt.Sprintf("TEST_ID=%s", its.TestID),
	)

	// Set up process group for proper cleanup
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	// Set up logging if needed
	logDir, _ := its.Resources.Directory.CreateSubDir("logs")
	logFile := filepath.Join(logDir, "server.log")
	if logFileHandle, err := os.Create(logFile); err == nil {
		cmd.Stdout = logFileHandle
		cmd.Stderr = logFileHandle
		its.addCleanupTask(func() error {
			logFileHandle.Close()
			return nil
		})
	}

	// Start the server
	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start server: %w", err)
	}

	its.ServerProcess = cmd

	// Register process with resource manager
	its.resourceManager.RegisterProcess(its.TestID, cmd.Process.Pid, fmt.Sprintf("%v", serverCommand), cancel)

	// Add cleanup for server process with proper process group termination
	its.addCleanupTask(func() error {
		return its.terminateServerProcess()
	})

	return nil
}

// terminateServerProcess performs proper process group termination with verification
func (its *IsolatedTestSetup) terminateServerProcess() error {
	if its.ServerProcess == nil || its.ServerProcess.Process == nil {
		return nil
	}

	pid := its.ServerProcess.Process.Pid
	pgid := pid // Process group ID is the same as PID for group leader

	// Step 1: Signal graceful shutdown via context cancellation
	if its.ServerCancel != nil {
		its.ServerCancel()
	}

	// Step 2: Wait for graceful shutdown with timeout
	gracefulTimeout := 2 * time.Second
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), gracefulTimeout)
	defer shutdownCancel()

	shutdownComplete := make(chan error, 1)
	go func() {
		shutdownComplete <- its.ServerProcess.Wait()
	}()

	select {
	case err := <-shutdownComplete:
		// Process terminated gracefully
		if err != nil && err.Error() != "signal: terminated" && err.Error() != "signal: killed" {
			return fmt.Errorf("server process ended with error: %w", err)
		}
		return its.verifyProcessTermination(pid)

	case <-shutdownCtx.Done():
		// Graceful shutdown timeout - proceed to SIGTERM
	}

	// Step 3: Send SIGTERM to process group
	if err := its.sendSignalToProcessGroup(pgid, syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM to process group %d: %w", pgid, err)
	}

	// Wait for SIGTERM to take effect
	termTimeout := 3 * time.Second
	termCtx, termCancel := context.WithTimeout(context.Background(), termTimeout)
	defer termCancel()

	select {
	case err := <-shutdownComplete:
		if err != nil && err.Error() != "signal: terminated" && err.Error() != "signal: killed" {
			return fmt.Errorf("server process ended with error after SIGTERM: %w", err)
		}
		return its.verifyProcessTermination(pid)

	case <-termCtx.Done():
		// SIGTERM timeout - proceed to SIGKILL
	}

	// Step 4: Force kill with SIGKILL to process group
	if err := its.sendSignalToProcessGroup(pgid, syscall.SIGKILL); err != nil {
		return fmt.Errorf("failed to send SIGKILL to process group %d: %w", pgid, err)
	}

	// Final wait for SIGKILL (should be immediate)
	killTimeout := 1 * time.Second
	killCtx, killCancel := context.WithTimeout(context.Background(), killTimeout)
	defer killCancel()

	select {
	case <-shutdownComplete:
		return its.verifyProcessTermination(pid)

	case <-killCtx.Done():
		return fmt.Errorf("process group %d failed to terminate even after SIGKILL", pgid)
	}
}

// sendSignalToProcessGroup sends a signal to the entire process group
func (its *IsolatedTestSetup) sendSignalToProcessGroup(pgid int, sig syscall.Signal) error {
	// Kill the entire process group (negative PID targets process group)
	return syscall.Kill(-pgid, sig)
}

// verifyProcessTermination verifies that the process has actually terminated
func (its *IsolatedTestSetup) verifyProcessTermination(pid int) error {
	// Check if process still exists by sending signal 0
	if err := syscall.Kill(pid, 0); err != nil {
		if err == syscall.ESRCH {
			// Process doesn't exist - successfully terminated
			return nil
		}
		// Some other error occurred
		return fmt.Errorf("error checking process %d status: %w", pid, err)
	}

	// Process still exists - this is an error
	return fmt.Errorf("process %d still exists after termination attempt", pid)
}

// verifyPortReleased verifies that a port has been properly released
func (its *IsolatedTestSetup) verifyPortReleased(port int) error {
	// Give a brief moment for port to be released after process termination
	time.Sleep(100 * time.Millisecond)

	// Try to bind to the port to verify it's available
	listener, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		return fmt.Errorf("failed to create socket for port verification: %w", err)
	}
	defer syscall.Close(listener)

	// Set SO_REUSEADDR to avoid "address already in use" errors
	if err := syscall.SetsockoptInt(listener, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		return fmt.Errorf("failed to set SO_REUSEADDR: %w", err)
	}

	// Parse address for binding
	sockaddr := &syscall.SockaddrInet4{Port: port}
	copy(sockaddr.Addr[:], []byte{127, 0, 0, 1}) // localhost

	// Try to bind to the port
	if err := syscall.Bind(listener, sockaddr); err != nil {
		if err == syscall.EADDRINUSE {
			return fmt.Errorf("port %d still in use after process termination", port)
		}
		return fmt.Errorf("failed to verify port %d release: %w", port, err)
	}

	return nil
}

// WaitForServerReady waits for the server to be ready for connections
func (its *IsolatedTestSetup) WaitForServerReady(timeout time.Duration) error {
	baseURL := fmt.Sprintf("http://localhost:%d", its.Resources.Port)

	// Use fast startup detection if timeout is short (< 5 seconds) for better performance
	if timeout < 5*time.Second {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		healthChecker := NewFastHealthChecker(baseURL)
		defer healthChecker.Close()

		return healthChecker.HybridHealthCheck(ctx, its.Resources.Port)
	}

	// For longer timeouts, use the optimized server startup detection
	config := FastStartupPollingConfig()
	config.StartupTimeout = timeout

	return WaitForServerStartupWithConfig(baseURL, its.Resources.Port, config)
}

// GetHTTPClient returns an HTTP client configured for this test's server
func (its *IsolatedTestSetup) GetHTTPClient() *HttpClient {
	config := DefaultHttpClientConfig()
	config.BaseURL = fmt.Sprintf("http://localhost:%d", its.Resources.Port)
	return NewHttpClient(config)
}

// CreateTempConfig creates a temporary configuration file in the isolated directory
func (its *IsolatedTestSetup) CreateTempConfig(content string) (string, error) {
	return its.Resources.Directory.CreateTempFile("temp_config.yaml", content)
}

// CreateTempConfigWithPort creates a config with the allocated port substituted
func (its *IsolatedTestSetup) CreateTempConfigWithPort(template string) (string, error) {
	content := substituteVariables(template, map[string]string{
		"PORT":     strconv.Itoa(its.Resources.Port),
		"TEST_ID":  its.TestID,
		"TEST_DIR": its.Resources.Directory.Path,
	})
	return its.CreateTempConfig(content)
}

// addCleanupTask adds a cleanup task to be executed during cleanup
func (its *IsolatedTestSetup) addCleanupTask(task func() error) {
	its.Resources.Directory.AddCleanupTask(task)
	its.cleanup = append(its.cleanup, task)
}

// Cleanup cleans up all resources associated with this test
func (its *IsolatedTestSetup) Cleanup() error {
	its.mu.Lock()
	defer its.mu.Unlock()

	var errors []error

	// Step 1: Terminate server process first (most critical)
	if its.ServerProcess != nil {
		if err := its.terminateServerProcess(); err != nil {
			errors = append(errors, fmt.Errorf("server termination failed: %w", err))
		}
		its.ServerProcess = nil
	}

	// Step 2: Cancel any remaining contexts
	if its.ServerCancel != nil {
		its.ServerCancel()
		its.ServerCancel = nil
	}

	// Step 3: Execute cleanup tasks in reverse order
	for i := len(its.cleanup) - 1; i >= 0; i-- {
		if err := its.cleanup[i](); err != nil {
			errors = append(errors, fmt.Errorf("cleanup task %d failed: %w", i, err))
		}
	}

	// Step 4: Verify port is released (resource cleanup verification)
	if its.Resources != nil && its.Resources.Port != 0 {
		if err := its.verifyPortReleased(its.Resources.Port); err != nil {
			errors = append(errors, fmt.Errorf("port %d not properly released: %w", its.Resources.Port, err))
		}
	}

	// Step 5: Final resource cleanup
	if its.Resources != nil {
		if err := its.Resources.Directory.Cleanup(); err != nil {
			errors = append(errors, fmt.Errorf("directory cleanup failed: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}
	return nil
}

// createLanguageConfig generates a basic config for the specified language
func (its *IsolatedTestSetup) createLanguageConfig(language string) (string, error) {
	templates := map[string]string{
		"go": `
servers:
  - name: "go-server"
    language: "go"
    command: ["gopls"]
    transport: "stdio"
    root_path: "{{TEST_DIR}}"

gateway:
  host: "localhost"
  port: {{PORT}}

performance:
  cache:
    enabled: true
    memory_limit: "50MB"

logging:
  level: "info"
  file: "{{TEST_DIR}}/logs/gateway.log"
`,
		"python": `
servers:
  - name: "python-server"
    language: "python"
    command: ["pylsp"]
    transport: "stdio"
    root_path: "{{TEST_DIR}}"

gateway:
  host: "localhost"
  port: {{PORT}}

performance:
  cache:
    enabled: true
    memory_limit: "50MB"

logging:
  level: "info"
  file: "{{TEST_DIR}}/logs/gateway.log"
`,
		"typescript": `
servers:
  - name: "typescript-server"
    language: "typescript"
    command: ["typescript-language-server", "--stdio"]
    transport: "stdio"
    root_path: "{{TEST_DIR}}"

gateway:
  host: "localhost"
  port: {{PORT}}

performance:
  cache:
    enabled: true
    memory_limit: "50MB"

logging:
  level: "info"
  file: "{{TEST_DIR}}/logs/gateway.log"
`,
		"javascript": `
servers:
  - name: "javascript-server"
    language: "javascript"
    command: ["typescript-language-server", "--stdio"]
    transport: "stdio"
    root_path: "{{TEST_DIR}}"

gateway:
  host: "localhost"
  port: {{PORT}}

performance:
  cache:
    enabled: true
    memory_limit: "50MB"

logging:
  level: "info"
  file: "{{TEST_DIR}}/logs/gateway.log"
`,
		"java": `
servers:
  - name: "java-server"
    language: "java"
    command: ["jdtls"]
    transport: "stdio"
    root_path: "{{TEST_DIR}}"

gateway:
  host: "localhost"
  port: {{PORT}}

performance:
  cache:
    enabled: true
    memory_limit: "100MB"

logging:
  level: "info"
  file: "{{TEST_DIR}}/logs/gateway.log"
`,
	}

	template, exists := templates[language]
	if !exists {
		return "", fmt.Errorf("unsupported language: %s", language)
	}

	return substituteVariables(template, map[string]string{
		"PORT":     strconv.Itoa(its.Resources.Port),
		"TEST_DIR": its.Resources.Directory.Path,
		"TEST_ID":  its.TestID,
	}), nil
}

// QuickIsolatedTest provides a simple way to run an isolated test with automatic cleanup
func QuickIsolatedTest(testName string, testFunc func(*IsolatedTestSetup) error) error {
	setup, err := SetupIsolatedTest(testName)
	if err != nil {
		return err
	}
	defer setup.Cleanup()

	return testFunc(setup)
}

// QuickIsolatedTestWithLanguage runs an isolated test with language-specific setup
func QuickIsolatedTestWithLanguage(testName, language string, testFunc func(*IsolatedTestSetup) error) error {
	setup, err := SetupIsolatedTestWithLanguage(testName, language)
	if err != nil {
		return err
	}
	defer setup.Cleanup()

	return testFunc(setup)
}
