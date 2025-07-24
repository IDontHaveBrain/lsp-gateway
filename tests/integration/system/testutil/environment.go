package testutil

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	config "lsp-gateway/internal/config"
	"lsp-gateway/internal/gateway"
	testutil "lsp-gateway/tests/utils/helpers"
)

// TestEnvironment provides comprehensive integration testing infrastructure
type TestEnvironment struct {
	t           *testing.T
	tempDir     string
	gateway     *gateway.Gateway
	httpServer  *http.Server
	mockServers map[string]*MockLSPServer
	httpPort    int
	tcpPorts    map[string]int
	cleanup     []func()
	mu          sync.RWMutex
}

// TestEnvironmentConfig configures test environment setup
type TestEnvironmentConfig struct {
	Languages         []string
	HTTPPort          int
	EnableMockServers bool
	EnablePerformance bool
	CustomConfig      *config.GatewayConfig
	ResourceLimits    *ResourceLimits
	NetworkConditions *NetworkConditions
}

// ResourceLimits defines resource constraints for testing
type ResourceLimits struct {
	MaxMemoryMB     int
	MaxCPUPercent   int
	MaxConnections  int
	RequestTimeout  time.Duration
	ShutdownTimeout time.Duration
}

// NetworkConditions simulates network conditions
type NetworkConditions struct {
	Latency          time.Duration
	PacketLoss       float64
	Bandwidth        int // KB/s
	ConnectionErrors float64
}

// DefaultTestEnvironmentConfig returns sensible defaults for testing
func DefaultTestEnvironmentConfig() *TestEnvironmentConfig {
	return &TestEnvironmentConfig{
		Languages:         []string{"go", "python", "typescript", "java"},
		EnableMockServers: true,
		EnablePerformance: false,
		ResourceLimits: &ResourceLimits{
			MaxMemoryMB:     256,
			MaxCPUPercent:   80,
			MaxConnections:  100,
			RequestTimeout:  30 * time.Second,
			ShutdownTimeout: 10 * time.Second,
		},
		NetworkConditions: &NetworkConditions{
			Latency:          0,
			PacketLoss:       0,
			Bandwidth:        0, // Unlimited
			ConnectionErrors: 0,
		},
	}
}

// SetupTestEnvironment creates a complete test environment with mock servers and gateway
func SetupTestEnvironment(t *testing.T, config *TestEnvironmentConfig) *TestEnvironment {
	if config == nil {
		config = DefaultTestEnvironmentConfig()
	}

	env := &TestEnvironment{
		t:           t,
		tempDir:     testutil.TempDir(t),
		mockServers: make(map[string]*MockLSPServer),
		tcpPorts:    make(map[string]int),
		cleanup:     make([]func(), 0),
	}

	// Allocate HTTP port for gateway
	if config.HTTPPort == 0 {
		env.httpPort = testutil.AllocateTestPort(t)
	} else {
		env.httpPort = config.HTTPPort
	}

	// Setup mock servers if enabled
	if config.EnableMockServers {
		env.setupMockServers(config.Languages)
	}

	// Create gateway configuration
	gatewayConfig := env.createGatewayConfig(config)

	// Create and start gateway
	gw, err := gateway.NewGateway(gatewayConfig)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}
	env.gateway = gw

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second) // Increase to 15s for better reliability
	defer cancel()

	if err := gw.Start(ctx); err != nil {
		t.Fatalf("Failed to start gateway: %v", err)
	}

	// Start HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/jsonrpc", gw.HandleJSONRPC)
	env.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", env.httpPort),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		if err := env.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("HTTP server error: %v", err)
		}
	}()

	// Add cleanup for gateway and HTTP server shutdown
	env.addCleanup(func() {
		if env.httpServer != nil {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer shutdownCancel()
			if err := env.httpServer.Shutdown(shutdownCtx); err != nil {
				t.Logf("Error shutting down HTTP server: %v", err)
			}
		}
		if err := gw.Stop(); err != nil {
			t.Logf("Error stopping gateway: %v", err)
		}
	})

	return env
}

// setupMockServers creates mock LSP servers for specified languages
func (env *TestEnvironment) setupMockServers(languages []string) {
	for _, lang := range languages {
		mockServer := NewMockLSPServer(env.t, &MockLSPServerConfig{
			Language:        lang,
			Transport:       "stdio",
			ResponseLatency: 1 * time.Millisecond, // Reduce from 10ms to 1ms
			ErrorRate:       0.0,
		})

		env.mockServers[lang] = mockServer
		env.addCleanup(func() {
			mockServer.Stop()
		})
	}
}

// createGatewayConfig generates gateway configuration with mock servers
func (env *TestEnvironment) createGatewayConfig(testConfig *TestEnvironmentConfig) *config.GatewayConfig {
	if testConfig.CustomConfig != nil {
		return testConfig.CustomConfig
	}

	// Create config with mock servers
	var serverConfigs []config.ServerConfig

	if testConfig.EnableMockServers {
		// Add mock server configurations
		for lang, mockServer := range env.mockServers {
			serverConfig := config.ServerConfig{
				Name:      fmt.Sprintf("mock-%s-lsp", lang),
				Languages: []string{lang},
				Command:   mockServer.BinaryPath(),
				Args:      []string{},
				Transport: "stdio",
			}
			serverConfigs = append(serverConfigs, serverConfig)
		}
	}

	return &config.GatewayConfig{
		Port:    env.httpPort,
		Servers: serverConfigs,
	}
}

// addCleanup adds a cleanup function to be called during teardown
func (env *TestEnvironment) addCleanup(fn func()) {
	env.mu.Lock()
	defer env.mu.Unlock()
	env.cleanup = append(env.cleanup, fn)
}

// BaseURL returns the base URL for HTTP requests
func (env *TestEnvironment) BaseURL() string {
	return fmt.Sprintf("http://localhost:%d", env.httpPort)
}

// WaitForReady waits for the gateway to be ready to accept requests
func (env *TestEnvironment) WaitForReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 1 * time.Second}
	
	for time.Now().Before(deadline) {
		// First check if TCP connection is available
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", env.httpPort), 500*time.Millisecond)
		if err != nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		_ = conn.Close()
		
		// Then check health endpoint
		resp, err := client.Get(fmt.Sprintf("http://localhost:%d/health", env.httpPort))
		if err != nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		resp.Body.Close()
		
		// Accept both OK (fully ready) and Service Unavailable (starting) status
		// This allows tests to proceed even if LSP clients are still starting
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusServiceUnavailable {
			return nil
		}
		
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("gateway not ready after %v", timeout)
}

// Cleanup performs cleanup of all resources with timeout
func (env *TestEnvironment) Cleanup() {
	env.mu.Lock()
	cleanupFuncs := make([]func(), len(env.cleanup))
	copy(cleanupFuncs, env.cleanup)
	env.mu.Unlock()

	// Execute cleanup functions in reverse order with timeout
	for i := len(cleanupFuncs) - 1; i >= 0; i-- {
		func() {
			defer func() {
				if r := recover(); r != nil {
					env.t.Logf("Cleanup function panicked: %v", r)
				}
			}()

			// Add timeout for each cleanup function to prevent hanging
			done := make(chan struct{})
			go func() {
				cleanupFuncs[i]()
				close(done)
			}()

			select {
			case <-done:
				// Cleanup completed successfully
			case <-time.After(5 * time.Second):
				env.t.Logf("Cleanup function %d timed out after 5 seconds", i)
			}
		}()
	}
}
