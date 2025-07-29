package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/mcp"
)

// MockLSPClient implements transport.LSPClient for testing
type MockLSPClient struct {
	isActive       bool
	startErr       error
	stopErr        error
	sendRequestErr error
	sendNotifErr   error
	responses      map[string]json.RawMessage
	mutex          sync.RWMutex
	startCalled    int
	stopCalled     int
	requestCount   int
}

func NewMockLSPClient() *MockLSPClient {
	return &MockLSPClient{
		isActive:  true,
		responses: make(map[string]json.RawMessage),
	}
}

func (m *MockLSPClient) Start(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.startCalled++
	m.isActive = true
	return m.startErr
}

func (m *MockLSPClient) Stop() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.stopCalled++
	m.isActive = false
	return m.stopErr
}

func (m *MockLSPClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.requestCount++
	
	if m.sendRequestErr != nil {
		return nil, m.sendRequestErr
	}
	
	if response, exists := m.responses[method]; exists {
		return response, nil
	}
	
	// Default response
	return json.RawMessage(`{"result": "success"}`), nil
}

func (m *MockLSPClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.sendNotifErr
}

func (m *MockLSPClient) IsActive() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.isActive
}

func (m *MockLSPClient) SetResponse(method string, response json.RawMessage) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.responses[method] = response
}

func (m *MockLSPClient) GetStats() (int, int, int) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.startCalled, m.stopCalled, m.requestCount
}

// Test helpers
func createTestWorkspaceConfig() *WorkspaceConfig {
	return &WorkspaceConfig{
		Workspace: WorkspaceInfo{
			WorkspaceID: "test-workspace",
			RootPath:    "/test/workspace",
		},
		SubProjects: []*SubProjectInfo{
			{
				ID:           "test-project-1",
				Name:         "Test Project 1",
				RelativePath: "project1",
				AbsolutePath: "/test/workspace/project1",
				ProjectType:  "go",
				Languages:    []string{"go"},
			},
			{
				ID:           "test-project-2",
				Name:         "Test Project 2", 
				RelativePath: "project2",
				AbsolutePath: "/test/workspace/project2",
				ProjectType:  "python",
				Languages:    []string{"python"},
			},
		},
		Servers: map[string]*config.ServerConfig{
			"gopls": {
				Command:   "gopls",
				Args:      []string{"serve"},
				Languages: []string{"go"},
				Transport: "stdio",
			},
			"pylsp": {
				Command:   "pylsp",
				Args:      []string{},
				Languages: []string{"python"},
				Transport: "stdio",
			},
		},
	}
}

func createTestLogger() *mcp.StructuredLogger {
	config := &mcp.LoggerConfig{
		Level:     mcp.LogLevelDebug,
		Component: "test",
	}
	return mcp.NewStructuredLogger(config)
}

// TestClientRegistry tests the client registry functionality
func TestClientRegistry(t *testing.T) {
	config := DefaultClientManagerConfig()
	logger := createTestLogger()
	registry := NewClientRegistry(config, logger)
	defer registry.Shutdown(context.Background())

	// Test client registration
	t.Run("RegisterClient", func(t *testing.T) {
		mockClient := NewMockLSPClient()
		clientInfo := &ClientInfo{
			Client:        mockClient,
			SubProjectID:  "test-project",
			Language:      "go",
			WorkspaceRoot: "/test/workspace",
			Transport:     "stdio",
			StartedAt:     time.Now(),
			LastUsed:      time.Now(),
		}

		err := registry.RegisterClient(clientInfo)
		if err != nil {
			t.Fatalf("Failed to register client: %v", err)
		}

		// Verify client is registered
		client, err := registry.GetClient("test-project", "go")
		if err != nil {
			t.Fatalf("Failed to get registered client: %v", err)
		}
		if client != mockClient {
			t.Error("Retrieved client does not match registered client")
		}
	})

	t.Run("DuplicateRegistration", func(t *testing.T) {
		mockClient := NewMockLSPClient()
		clientInfo := &ClientInfo{
			Client:        mockClient,
			SubProjectID:  "test-project",
			Language:      "go",
			WorkspaceRoot: "/test/workspace",
			Transport:     "stdio",
			StartedAt:     time.Now(),
			LastUsed:      time.Now(),
		}

		err := registry.RegisterClient(clientInfo)
		if err == nil {
			t.Error("Expected error for duplicate registration")
		}
	})

	t.Run("GetAllClients", func(t *testing.T) {
		allClients := registry.GetAllClients()
		if len(allClients) != 1 {
			t.Errorf("Expected 1 sub-project, got %d", len(allClients))
		}
		if len(allClients["test-project"]) != 1 {
			t.Errorf("Expected 1 client for test-project, got %d", len(allClients["test-project"]))
		}
	})

	t.Run("UnregisterClient", func(t *testing.T) {
		err := registry.UnregisterClient("test-project", "go")
		if err != nil {
			t.Fatalf("Failed to unregister client: %v", err)
		}

		// Verify client is unregistered
		_, err = registry.GetClient("test-project", "go")
		if err == nil {
			t.Error("Expected error for unregistered client")
		}
	})

	t.Run("RegistryHealth", func(t *testing.T) {
		health := registry.GetHealth()
		if health == nil {
			t.Error("Registry health should not be nil")
		}
		if health.TotalClients != 0 {
			t.Errorf("Expected 0 total clients, got %d", health.TotalClients)
		}
	})
}

// TestClientHealthMonitor tests the health monitoring functionality
func TestClientHealthMonitor(t *testing.T) {
	config := DefaultClientManagerConfig()
	logger := createTestLogger()
	registry := NewClientRegistry(config, logger)
	healthMonitor := NewClientHealthMonitor(registry, config, logger)
	defer registry.Shutdown(context.Background())
	defer healthMonitor.Shutdown(context.Background())

	// Register a test client
	mockClient := NewMockLSPClient()
	clientInfo := &ClientInfo{
		Client:        mockClient,
		SubProjectID:  "test-project",
		Language:      "go",
		WorkspaceRoot: "/test/workspace",
		Transport:     "stdio",
		StartedAt:     time.Now(),
		LastUsed:      time.Now(),
	}
	registry.RegisterClient(clientInfo)

	t.Run("PerformHealthCheck", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := healthMonitor.PerformHealthCheck(ctx)
		if err != nil {
			t.Fatalf("Health check failed: %v", err)
		}

		// Check that health data was created
		healthInfo := healthMonitor.GetClientHealth("test-project", "go")
		if healthInfo == nil {
			t.Error("Health info should not be nil after health check")
		} else {
			if healthInfo.HealthCheckCount == 0 {
				t.Error("Health check count should be greater than 0")
			}
			if !healthInfo.IsHealthy {
				t.Error("Client should be healthy")
			}
		}
	})

	t.Run("FailingClient", func(t *testing.T) {
		// Make the mock client fail
		mockClient.sendRequestErr = fmt.Errorf("mock error")
		mockClient.isActive = false

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := healthMonitor.PerformHealthCheck(ctx)
		if err != nil {
			t.Fatalf("Health check failed: %v", err)
		}

		healthInfo := healthMonitor.GetClientHealth("test-project", "go")
		if healthInfo == nil {
			t.Error("Health info should not be nil")
		} else {
			if healthInfo.IsHealthy {
				t.Error("Client should be unhealthy")
			}
			if healthInfo.ConsecutiveFails == 0 {
				t.Error("Should have consecutive failures")
			}
		}
	})
}

// TestCircuitBreaker tests the circuit breaker functionality
func TestCircuitBreaker(t *testing.T) {
	breaker := NewCircuitBreaker(3, 100*time.Millisecond)

	t.Run("InitialState", func(t *testing.T) {
		if breaker.GetState() != CircuitBreakerClosed {
			t.Error("Circuit breaker should start in closed state")
		}
		if !breaker.AllowRequest() {
			t.Error("Circuit breaker should allow requests when closed")
		}
	})

	t.Run("FailureThreshold", func(t *testing.T) {
		// Record failures to open the circuit
		for i := 0; i < 3; i++ {
			breaker.RecordFailure()
		}

		if breaker.GetState() != CircuitBreakerOpen {
			t.Error("Circuit breaker should be open after failure threshold")
		}
		if breaker.AllowRequest() {
			t.Error("Circuit breaker should not allow requests when open")
		}
	})

	t.Run("Recovery", func(t *testing.T) {
		// Wait for recovery timeout
		time.Sleep(150 * time.Millisecond)

		if !breaker.AllowRequest() {
			t.Error("Circuit breaker should allow requests after recovery timeout")
		}
		if breaker.GetState() != CircuitBreakerHalfOpen {
			t.Error("Circuit breaker should be half-open after recovery timeout")
		}

		// Record success to close the circuit
		breaker.RecordSuccess()
		breaker.RecordSuccess()
		breaker.RecordSuccess()

		if breaker.GetState() != CircuitBreakerClosed {
			t.Error("Circuit breaker should be closed after successful requests")
		}
	})
}

// TestSubProjectClientManager tests the main client manager functionality
func TestSubProjectClientManager(t *testing.T) {
	workspaceConfig := createTestWorkspaceConfig()
	config := DefaultClientManagerConfig()
	config.EnableLazyInitialization = true
	config.HealthCheckInterval = 100 * time.Millisecond
	logger := createTestLogger()

	// Mock the transport.NewLSPClient function for testing
	// originalNewLSPClient := transport.NewLSPClient  // TODO: implement proper mocking
	defer func() {
		// Restore the original function - this is a conceptual approach
		// In practice, you'd need dependency injection or a factory pattern
	}()

	// Create the manager
	manager := NewSubProjectClientManager(workspaceConfig, config, logger)
	defer manager.Shutdown(context.Background())

	t.Run("GetNonExistentClient", func(t *testing.T) {
		_, err := manager.GetClient("nonexistent", "go")
		if err == nil {
			t.Error("Expected error for non-existent client")
		}
	})

	t.Run("GetAllClientStatuses", func(t *testing.T) {
		statuses := manager.GetAllClientStatuses()
		if statuses == nil {
			t.Error("Client statuses should not be nil")
		}
	})

	t.Run("HealthCheck", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		health, err := manager.HealthCheck(ctx)
		if err != nil {
			t.Fatalf("Health check failed: %v", err)
		}
		if health == nil {
			t.Error("Health result should not be nil")
		}
	})

	t.Run("LoadBalancingInfo", func(t *testing.T) {
		info := manager.GetLoadBalancingInfo()
		if info == nil {
			t.Error("Load balancing info should not be nil")
		}
	})

	t.Run("ResourceLimits", func(t *testing.T) {
		// Test resource limit checking
		config.MaxClients = 0 // Set limit to 0 to trigger limit check
		
		subProject := &SubProject{
			ID:        "test-limit-project",
			Name:      "Test Limit Project",
			Root:      "/test/limit",
			Languages: []string{"go"},
		}

		_, err := manager.CreateClient(context.Background(), subProject, "go")
		if err == nil {
			t.Error("Expected error due to resource limits")
		}
	})
}

// TestConcurrentAccess tests thread safety of the client manager
func TestConcurrentAccess(t *testing.T) {
	workspaceConfig := createTestWorkspaceConfig()
	config := DefaultClientManagerConfig()
	logger := createTestLogger()
	
	manager := NewSubProjectClientManager(workspaceConfig, config, logger)
	defer manager.Shutdown(context.Background())

	// Test concurrent access
	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numOperations)

	// Launch goroutines that perform various operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < numOperations; j++ {
				// Mix of different operations
				switch j % 4 {
				case 0:
					_, err := manager.GetClient("test-project-1", "go")
					if err != nil {
						// Expected for non-existent clients
					}
				case 1:
					statuses := manager.GetAllClientStatuses()
					if statuses == nil {
						errors <- fmt.Errorf("nil client statuses")
					}
				case 2:
					ctx, cancel := context.WithTimeout(context.Background(), time.Second)
					_, err := manager.HealthCheck(ctx)
					cancel()
					if err != nil {
						errors <- fmt.Errorf("health check error: %w", err)
					}
				case 3:
					info := manager.GetLoadBalancingInfo()
					if info == nil {
						errors <- fmt.Errorf("nil load balancing info")
					}
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Errorf("Concurrent access error: %v", err)
	}
}

// TestRegistryCleanup tests the cleanup functionality
func TestRegistryCleanup(t *testing.T) {
	config := DefaultClientManagerConfig()
	logger := createTestLogger()
	registry := NewClientRegistry(config, logger)
	defer registry.Shutdown(context.Background())

	// Register several clients with different ages
	now := time.Now()
	clients := []*ClientInfo{
		{
			Client:        NewMockLSPClient(),
			SubProjectID:  "project1",
			Language:      "go",
			StartedAt:     now.Add(-2 * time.Hour), // Old client
			LastUsed:      now.Add(-1 * time.Hour), // Idle client
		},
		{
			Client:        NewMockLSPClient(),
			SubProjectID:  "project2", 
			Language:      "python",
			StartedAt:     now.Add(-10 * time.Minute), // Recent client
			LastUsed:      now.Add(-1 * time.Minute),  // Recent use
		},
	}

	for _, clientInfo := range clients {
		registry.RegisterClient(clientInfo)
	}

	// Test cleanup
	t.Run("CleanupOldClients", func(t *testing.T) {
		removedCount := registry.Cleanup(1*time.Hour, 30*time.Minute)
		if removedCount == 0 {
			t.Error("Expected at least one client to be removed during cleanup")
		}
	})

	t.Run("GetIdleClients", func(t *testing.T) {
		cutoff := now.Add(-30 * time.Minute)
		idleClients := registry.GetIdleClients(cutoff)
		// The number depends on what was cleaned up, just verify it returns a slice
		if idleClients == nil {
			t.Error("GetIdleClients should not return nil")
		}
	})
}

// TestClientManagerShutdown tests graceful shutdown
func TestClientManagerShutdown(t *testing.T) {
	workspaceConfig := createTestWorkspaceConfig()
	config := DefaultClientManagerConfig()
	logger := createTestLogger()

	manager := NewSubProjectClientManager(workspaceConfig, config, logger)

	// Register some mock clients directly in registry for testing
	registry := manager.GetRegistry()
	mockClient := NewMockLSPClient()
	clientInfo := &ClientInfo{
		Client:        mockClient,
		SubProjectID:  "test-project",
		Language:      "go",
		WorkspaceRoot: "/test/workspace",
		Transport:     "stdio",
		StartedAt:     time.Now(),
		LastUsed:      time.Now(),
	}
	registry.RegisterClient(clientInfo)

	t.Run("GracefulShutdown", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := manager.Shutdown(ctx)
		if err != nil {
			t.Fatalf("Shutdown failed: %v", err)
		}

		// Verify that operations fail after shutdown
		_, err = manager.GetClient("test-project", "go")
		if err == nil {
			t.Error("Expected error after shutdown")
		}
	})

	t.Run("MultipleShutdowns", func(t *testing.T) {
		// Multiple shutdowns should not cause problems
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := manager.Shutdown(ctx)
		if err != nil {
			t.Errorf("Second shutdown should not fail: %v", err)
		}
	})
}

// BenchmarkClientOperations benchmarks common client operations
func BenchmarkClientOperations(b *testing.B) {
	workspaceConfig := createTestWorkspaceConfig()
	config := DefaultClientManagerConfig()
	logger := createTestLogger()
	
	manager := NewSubProjectClientManager(workspaceConfig, config, logger)
	defer manager.Shutdown(context.Background())

	registry := manager.GetRegistry()
	
	// Pre-populate with some clients
	for i := 0; i < 10; i++ {
		mockClient := NewMockLSPClient()
		clientInfo := &ClientInfo{
			Client:        mockClient,
			SubProjectID:  fmt.Sprintf("project-%d", i),
			Language:      "go",
			WorkspaceRoot: fmt.Sprintf("/test/workspace/project-%d", i),
			Transport:     "stdio",
			StartedAt:     time.Now(),
			LastUsed:      time.Now(),
		}
		registry.RegisterClient(clientInfo)
	}

	b.Run("GetClient", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = manager.GetClient("project-0", "go")
		}
	})

	b.Run("GetAllClientStatuses", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = manager.GetAllClientStatuses()
		}
	})

	b.Run("HealthCheck", func(b *testing.B) {
		ctx := context.Background()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = manager.HealthCheck(ctx)
		}
	})
}

// TestErrorRecovery tests error recovery scenarios
func TestErrorRecovery(t *testing.T) {
	config := DefaultClientManagerConfig()
	config.EnableAutoRestart = true
	logger := createTestLogger()
	registry := NewClientRegistry(config, logger)
	healthMonitor := NewClientHealthMonitor(registry, config, logger)
	defer registry.Shutdown(context.Background())
	defer healthMonitor.Shutdown(context.Background())

	// Register a client that will fail
	mockClient := NewMockLSPClient()
	mockClient.sendRequestErr = fmt.Errorf("simulated failure")
	clientInfo := &ClientInfo{
		Client:        mockClient,
		SubProjectID:  "failing-project",
		Language:      "go",
		WorkspaceRoot: "/test/workspace",
		Transport:     "stdio",
		StartedAt:     time.Now(),
		LastUsed:      time.Now(),
	}
	registry.RegisterClient(clientInfo)

	t.Run("FailureDetection", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Perform multiple health checks to trigger failure detection
		for i := 0; i < 5; i++ {
			healthMonitor.PerformHealthCheck(ctx)
			time.Sleep(100 * time.Millisecond)
		}

		healthInfo := healthMonitor.GetClientHealth("failing-project", "go")
		if healthInfo == nil {
			t.Error("Health info should not be nil")
		} else {
			if healthInfo.IsHealthy {
				t.Error("Client should be detected as unhealthy")
			}
			if healthInfo.ConsecutiveFails == 0 {
				t.Error("Should have consecutive failures recorded")
			}
		}
	})

	t.Run("HealthRecovery", func(t *testing.T) {
		// Fix the client
		mockClient.sendRequestErr = nil
		mockClient.isActive = true

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Perform health check to detect recovery
		healthMonitor.PerformHealthCheck(ctx)

		healthInfo := healthMonitor.GetClientHealth("failing-project", "go")
		if healthInfo == nil {
			t.Error("Health info should not be nil")
		} else {
			if !healthInfo.IsHealthy {
				t.Error("Client should be detected as healthy after recovery")
			}
		}
	})
}