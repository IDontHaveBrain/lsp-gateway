package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/transport"
	"lsp-gateway/mcp"
)

type MockLSPClient struct {
	mu           sync.RWMutex
	active       bool
	startErr     error
	stopErr      error
	requestErr   error
	notifyErr    error
	startCount   int
	stopCount    int
	requestCount int
	notifyCount  int
	responses    map[string]json.RawMessage
}

func NewMockLSPClient() *MockLSPClient {
	return &MockLSPClient{
		responses: make(map[string]json.RawMessage),
	}
}

func (m *MockLSPClient) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.startCount++
	if m.startErr != nil {
		return m.startErr
	}

	m.active = true
	return nil
}

func (m *MockLSPClient) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopCount++
	if m.stopErr != nil {
		return m.stopErr
	}

	m.active = false
	return nil
}

func (m *MockLSPClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.requestCount++
	if m.requestErr != nil {
		return nil, m.requestErr
	}

	if response, exists := m.responses[method]; exists {
		return response, nil
	}

	return json.RawMessage(`{"result":"mock"}`), nil
}

func (m *MockLSPClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.notifyCount++
	return m.notifyErr
}

func (m *MockLSPClient) IsActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.active
}

func (m *MockLSPClient) SetStartError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startErr = err
}

func (m *MockLSPClient) SetStopError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopErr = err
}

func (m *MockLSPClient) SetRequestError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestErr = err
}

func (m *MockLSPClient) SetNotificationError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifyErr = err
}

func (m *MockLSPClient) SetResponse(method string, response json.RawMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[method] = response
}

func (m *MockLSPClient) GetStartCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.startCount
}

func (m *MockLSPClient) GetStopCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stopCount
}

func (m *MockLSPClient) GetRequestCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.requestCount
}

func (m *MockLSPClient) GetNotificationCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.notifyCount
}

type TestableGateway struct {
	*Gateway
	clientFactory func(transport.ClientConfig) (transport.LSPClient, error)
}

func NewTestableGateway(config *config.GatewayConfig, clientFactory func(transport.ClientConfig) (transport.LSPClient, error)) (*TestableGateway, error) {
	logConfig := &mcp.LoggerConfig{
		Level:              mcp.LogLevelInfo,
		Component:          "gateway",
		EnableJSON:         false,
		EnableStackTrace:   false,
		EnableCaller:       true,
		EnableMetrics:      false,
		Output:             nil, // Uses default (stderr)
		IncludeTimestamp:   true,
		TimestampFormat:    "2006-01-02T15:04:05Z07:00",
		MaxStackTraceDepth: 10,
		EnableAsyncLogging: false,
		AsyncBufferSize:    1000,
	}
	logger := mcp.NewStructuredLogger(logConfig)

	gateway := &Gateway{
		config:  config,
		clients: make(map[string]transport.LSPClient),
		router:  NewRouter(),
		logger:  logger,
	}

	for _, serverConfig := range config.Servers {
		client, err := clientFactory(transport.ClientConfig{
			Command:   serverConfig.Command,
			Args:      serverConfig.Args,
			Transport: serverConfig.Transport,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create client for %s: %w", serverConfig.Name, err)
		}

		gateway.clients[serverConfig.Name] = client
		gateway.router.RegisterServer(serverConfig.Name, serverConfig.Languages)
	}

	return &TestableGateway{Gateway: gateway, clientFactory: clientFactory}, nil
}

func createTestConfig() *config.GatewayConfig {
	return &config.GatewayConfig{
		Port: 8080,
		Servers: []config.ServerConfig{
			{
				Name:      "go-lsp",
				Languages: []string{"go"},
				Command:   "gopls",
				Args:      []string{},
				Transport: "stdio",
			},
			{
				Name:      "python-lsp",
				Languages: []string{"python"},
				Command:   "python",
				Args:      []string{"-m", "pylsp"},
				Transport: "stdio",
			},
			{
				Name:      "typescript-lsp",
				Languages: []string{"typescript", "javascript"},
				Command:   "typescript-language-server",
				Args:      []string{"--stdio"},
				Transport: "stdio",
			},
		},
	}
}

func createSingleServerConfig() *config.GatewayConfig {
	return &config.GatewayConfig{
		Port: 8080,
		Servers: []config.ServerConfig{
			{
				Name:      "go-lsp",
				Languages: []string{"go"},
				Command:   "gopls",
				Args:      []string{},
				Transport: "stdio",
			},
		},
	}
}

func TestNewGateway(t *testing.T) {
	t.Parallel()
	mockClients := make(map[string]*MockLSPClient)
	mockClientFactory := func(config transport.ClientConfig) (transport.LSPClient, error) {
		mock := NewMockLSPClient()
		mockClients[config.Command] = mock
		return mock, nil
	}

	t.Run("ValidConfig", func(t *testing.T) {
		testValidGatewayConfig(t, mockClientFactory)
	})

	t.Run("EmptyConfig", func(t *testing.T) {
		testEmptyGatewayConfig(t, mockClientFactory)
	})

	t.Run("ClientCreationError", func(t *testing.T) {
		testClientCreationError(t)
	})
}

func testValidGatewayConfig(t *testing.T, mockClientFactory func(transport.ClientConfig) (transport.LSPClient, error)) {
	config := createTestConfig()
	testableGateway, err := NewTestableGateway(config, mockClientFactory)

	if err != nil {
		t.Fatalf("NewTestableGateway() failed: %v", err)
	}

	gateway := testableGateway.Gateway
	validateGatewayBasics(t, gateway, config)
	validateGatewayClients(t, gateway)
	validateRouterConfiguration(t, gateway)
}

func testEmptyGatewayConfig(t *testing.T, mockClientFactory func(transport.ClientConfig) (transport.LSPClient, error)) {
	config := &config.GatewayConfig{
		Port:    8080,
		Servers: []config.ServerConfig{},
	}

	testableGateway, err := NewTestableGateway(config, mockClientFactory)

	if err != nil {
		t.Fatalf("NewTestableGateway() failed: %v", err)
	}

	gateway := testableGateway.Gateway

	if gateway == nil {
		t.Fatal("NewTestableGateway() returned nil gateway")
	}

	if len(gateway.clients) != 0 {
		t.Fatalf("Expected 0 clients, got %d", len(gateway.clients))
	}
}

func testClientCreationError(t *testing.T) {
	errorClientFactory := func(config transport.ClientConfig) (transport.LSPClient, error) {
		return nil, errors.New("mock client creation error")
	}

	config := createSingleServerConfig()
	testableGateway, err := NewTestableGateway(config, errorClientFactory)

	if err == nil {
		t.Fatal("NewTestableGateway() should have failed")
	}

	if testableGateway != nil {
		t.Fatal("NewTestableGateway() should return nil on error")
	}

	expectedError := "failed to create client for go-lsp"
	if err.Error()[:len(expectedError)] != expectedError {
		t.Fatalf("Expected error to start with %s, got %s", expectedError, err.Error())
	}
}

func validateGatewayBasics(t *testing.T, gateway *Gateway, config *config.GatewayConfig) {
	if gateway == nil {
		t.Fatal("NewGateway() returned nil gateway")
	}

	if gateway.config != config {
		t.Fatal("Gateway config not set correctly")
	}

	if gateway.clients == nil {
		t.Fatal("Gateway clients map is nil")
	}

	if gateway.router == nil {
		t.Fatal("Gateway router is nil")
	}
}

func validateGatewayClients(t *testing.T, gateway *Gateway) {
	expectedServers := []string{"go-lsp", "python-lsp", "typescript-lsp"}
	for _, serverName := range expectedServers {
		client, exists := gateway.GetClient(serverName)
		if !exists {
			t.Fatalf("Client %s not found", serverName)
		}
		if client == nil {
			t.Fatalf("Client %s is nil", serverName)
		}
	}
}

func validateRouterConfiguration(t *testing.T, gateway *Gateway) {
	server, err := gateway.router.RouteRequest("file:///test.go")
	if err != nil {
		t.Fatalf("Router not configured correctly: %v", err)
	}
	if server != "go-lsp" {
		t.Fatalf("Expected go-lsp, got %s", server)
	}
}

func TestGatewayStart(t *testing.T) {
	t.Parallel()
	mockClients := make(map[string]*MockLSPClient)
	mockClientFactory := func(config transport.ClientConfig) (transport.LSPClient, error) {
		mock := NewMockLSPClient()
		mockClients[config.Command] = mock
		return mock, nil
	}

	t.Run("StartAllClients", func(t *testing.T) {
		config := createTestConfig()
		testableGateway, err := NewTestableGateway(config, mockClientFactory)
		if err != nil {
			t.Fatalf("NewTestableGateway() failed: %v", err)
		}

		gateway := testableGateway.Gateway

		ctx := context.Background()
		err = gateway.Start(ctx)

		if err != nil {
			t.Fatalf("Gateway.Start() failed: %v", err)
		}

		for command, mock := range mockClients {
			if mock.GetStartCount() != 1 {
				t.Fatalf("Client %s Start() called %d times, expected 1", command, mock.GetStartCount())
			}
			if !mock.IsActive() {
				t.Fatalf("Client %s should be active", command)
			}
		}
	})

	t.Run("StartWithClientError", func(t *testing.T) {
		mockClientsLocal := make(map[string]*MockLSPClient)
		errorClientFactory := func(config transport.ClientConfig) (transport.LSPClient, error) {
			mock := NewMockLSPClient()
			mockClientsLocal[config.Command] = mock
			return mock, nil
		}

		config := createTestConfig()
		testableGateway, err := NewTestableGateway(config, errorClientFactory)
		if err != nil {
			t.Fatalf("NewTestableGateway() failed: %v", err)
		}

		gateway := testableGateway.Gateway

		var firstMock *MockLSPClient
		for _, mock := range mockClientsLocal {
			firstMock = mock
			break
		}
		firstMock.SetStartError(errors.New("mock start error"))

		ctx := context.Background()
		err = gateway.Start(ctx)

		if err == nil {
			t.Fatal("Gateway.Start() should have failed")
		}

		expectedError := "failed to start client"
		if err.Error()[:len(expectedError)] != expectedError {
			t.Fatalf("Expected error to start with %s, got %s", expectedError, err.Error())
		}
	})

	t.Run("StartEmptyGateway", func(t *testing.T) {
		config := &config.GatewayConfig{
			Port:    8080,
			Servers: []config.ServerConfig{},
		}
		testableGateway, err := NewTestableGateway(config, mockClientFactory)
		if err != nil {
			t.Fatalf("NewTestableGateway() failed: %v", err)
		}

		gateway := testableGateway.Gateway

		ctx := context.Background()
		err = gateway.Start(ctx)

		if err != nil {
			t.Fatalf("Gateway.Start() should not fail for empty gateway: %v", err)
		}
	})
}

func TestGatewayStop(t *testing.T) {
	t.Parallel()
	mockClients := make(map[string]*MockLSPClient)
	mockClientFactory := func(config transport.ClientConfig) (transport.LSPClient, error) {
		mock := NewMockLSPClient()
		mockClients[config.Command] = mock
		return mock, nil
	}

	t.Run("StopAllClients", func(t *testing.T) {
		config := createTestConfig()
		testableGateway, err := NewTestableGateway(config, mockClientFactory)
		if err != nil {
			t.Fatalf("NewTestableGateway() failed: %v", err)
		}

		gateway := testableGateway.Gateway

		ctx := context.Background()
		if err := gateway.Start(ctx); err != nil {
			t.Fatalf("Gateway.Start() failed: %v", err)
		}

		err = gateway.Stop()

		if err != nil {
			t.Fatalf("Gateway.Stop() failed: %v", err)
		}

		for command, mock := range mockClients {
			if mock.GetStopCount() != 1 {
				t.Fatalf("Client %s Stop() called %d times, expected 1", command, mock.GetStopCount())
			}
			if mock.IsActive() {
				t.Fatalf("Client %s should not be active", command)
			}
		}
	})

	t.Run("StopWithClientError", func(t *testing.T) {
		mockClientsLocal := make(map[string]*MockLSPClient)
		errorClientFactory := func(config transport.ClientConfig) (transport.LSPClient, error) {
			mock := NewMockLSPClient()
			mockClientsLocal[config.Command] = mock
			return mock, nil
		}

		config := createTestConfig()
		testableGateway, err := NewTestableGateway(config, errorClientFactory)
		if err != nil {
			t.Fatalf("NewTestableGateway() failed: %v", err)
		}

		gateway := testableGateway.Gateway

		var firstMock *MockLSPClient
		for _, mock := range mockClientsLocal {
			firstMock = mock
			break
		}
		firstMock.SetStopError(errors.New("mock stop error"))

		err = gateway.Stop()

		if err == nil {
			t.Fatal("Gateway.Stop() should have failed")
		}

		expectedError := "failed to stop client"
		if err.Error()[:len(expectedError)] != expectedError {
			t.Fatalf("Expected error to start with %s, got %s", expectedError, err.Error())
		}
	})

	t.Run("StopEmptyGateway", func(t *testing.T) {
		config := &config.GatewayConfig{
			Port:    8080,
			Servers: []config.ServerConfig{},
		}
		testableGateway, err := NewTestableGateway(config, mockClientFactory)
		if err != nil {
			t.Fatalf("NewTestableGateway() failed: %v", err)
		}

		gateway := testableGateway.Gateway

		err = gateway.Stop()

		if err != nil {
			t.Fatalf("Gateway.Stop() should not fail for empty gateway: %v", err)
		}
	})
}

func TestGatewayGetClient(t *testing.T) {
	t.Parallel()
	mockClients := make(map[string]*MockLSPClient)
	mockClientFactory := func(config transport.ClientConfig) (transport.LSPClient, error) {
		mock := NewMockLSPClient()
		mockClients[config.Command] = mock
		return mock, nil
	}

	t.Run("GetExistingClient", func(t *testing.T) {
		config := createTestConfig()
		testableGateway, err := NewTestableGateway(config, mockClientFactory)
		if err != nil {
			t.Fatalf("NewTestableGateway() failed: %v", err)
		}

		gateway := testableGateway.Gateway

		client, exists := gateway.GetClient("go-lsp")

		if !exists {
			t.Fatal("GetClient() should find go-lsp client")
		}

		if client == nil {
			t.Fatal("GetClient() returned nil client")
		}

		if _, ok := client.(*MockLSPClient); !ok {
			t.Fatal("GetClient() returned wrong client type")
		}
	})

	t.Run("GetNonexistentClient", func(t *testing.T) {
		config := createTestConfig()
		testableGateway, err := NewTestableGateway(config, mockClientFactory)
		if err != nil {
			t.Fatalf("NewTestableGateway() failed: %v", err)
		}

		gateway := testableGateway.Gateway

		client, exists := gateway.GetClient("nonexistent")

		if exists {
			t.Fatal("GetClient() should not find nonexistent client")
		}

		if client != nil {
			t.Fatal("GetClient() should return nil for nonexistent client")
		}
	})

	t.Run("GetAllConfiguredClients", func(t *testing.T) {
		config := createTestConfig()
		testableGateway, err := NewTestableGateway(config, mockClientFactory)
		if err != nil {
			t.Fatalf("NewTestableGateway() failed: %v", err)
		}

		gateway := testableGateway.Gateway

		expectedClients := []string{"go-lsp", "python-lsp", "typescript-lsp"}

		for _, serverName := range expectedClients {
			client, exists := gateway.GetClient(serverName)

			if !exists {
				t.Fatalf("GetClient() should find %s client", serverName)
			}

			if client == nil {
				t.Fatalf("GetClient() returned nil for %s client", serverName)
			}
		}
	})
}

func TestGatewayLifecycleIntegration(t *testing.T) {
	t.Parallel()
	mockClients := make(map[string]*MockLSPClient)
	mockClientFactory := func(config transport.ClientConfig) (transport.LSPClient, error) {
		mock := NewMockLSPClient()
		mockClients[config.Command] = mock
		return mock, nil
	}

	t.Run("FullLifecycle", func(t *testing.T) {
		config := createTestConfig()

		testableGateway, err := NewTestableGateway(config, mockClientFactory)
		if err != nil {
			t.Fatalf("NewTestableGateway() failed: %v", err)
		}

		gateway := testableGateway.Gateway

		client, exists := gateway.GetClient("go-lsp")
		if !exists {
			t.Fatal("go-lsp client not found after creation")
		}
		if client.IsActive() {
			t.Fatal("Client should not be active before Start()")
		}

		ctx := context.Background()
		if err := gateway.Start(ctx); err != nil {
			t.Fatalf("Gateway.Start() failed: %v", err)
		}

		if !client.IsActive() {
			t.Fatal("Client should be active after Start()")
		}

		if err := gateway.Stop(); err != nil {
			t.Fatalf("Gateway.Stop() failed: %v", err)
		}

		if client.IsActive() {
			t.Fatal("Client should not be active after Stop()")
		}

		mock := mockClients["gopls"]
		if mock.GetStartCount() != 1 {
			t.Fatalf("Expected 1 Start() call, got %d", mock.GetStartCount())
		}
		if mock.GetStopCount() != 1 {
			t.Fatalf("Expected 1 Stop() call, got %d", mock.GetStopCount())
		}
	})
}

func TestGatewayConcurrentAccess(t *testing.T) {
	mockClients := make(map[string]*MockLSPClient)
	mockClientFactory := func(config transport.ClientConfig) (transport.LSPClient, error) {
		mock := NewMockLSPClient()
		mockClients[config.Command] = mock
		return mock, nil
	}

	config := createTestConfig()
	testableGateway, err := NewTestableGateway(config, mockClientFactory)
	if err != nil {
		t.Fatalf("NewTestableGateway() failed: %v", err)
	}

	gateway := testableGateway.Gateway

	done := make(chan bool)
	const numGoroutines = 10
	const numIterations = 100

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()

			for j := 0; j < numIterations; j++ {
				client, exists := gateway.GetClient("go-lsp")
				if !exists || client == nil {
					t.Errorf("GetClient failed in concurrent test")
					return
				}

				_, exists = gateway.GetClient("nonexistent")
				if exists {
					t.Errorf("GetClient should not find nonexistent client")
					return
				}
			}
		}()
	}

	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent test timed out")
		}
	}

	t.Log("Concurrent access test passed")
}

func TestGatewayStartStopConcurrency(t *testing.T) {
	mockClients := make(map[string]*MockLSPClient)
	mockClientFactory := func(config transport.ClientConfig) (transport.LSPClient, error) {
		mock := NewMockLSPClient()
		mockClients[config.Command] = mock
		return mock, nil
	}

	config := createSingleServerConfig()
	testableGateway, err := NewTestableGateway(config, mockClientFactory)
	if err != nil {
		t.Fatalf("NewTestableGateway() failed: %v", err)
	}

	gateway := testableGateway.Gateway

	const numOperations = 5
	done := make(chan error, numOperations*2)

	for i := 0; i < numOperations; i++ {
		go func() {
			ctx := context.Background()
			done <- gateway.Start(ctx)
		}()
	}

	for i := 0; i < numOperations; i++ {
		go func() {
			done <- gateway.Stop()
		}()
	}

	for i := 0; i < numOperations*2; i++ {
		select {
		case err := <-done:
			_ = err
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent start/stop test timed out")
		}
	}

	t.Log("Concurrent start/stop test passed")
}
