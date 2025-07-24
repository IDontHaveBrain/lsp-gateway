package gateway_test

import (
	"context"
	"encoding/json"
	"fmt"
	"lsp-gateway/internal/config"
	"lsp-gateway/internal/gateway"
	"lsp-gateway/internal/transport"
	"lsp-gateway/mcp"
	"sync"
	"testing"
)

// MockLSPClient for testing gateway functionality
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

// TestableGateway wraps Gateway for testing
type TestableGateway struct {
	*gateway.Gateway
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

	gateway := &gateway.Gateway{
		Config:  config,
		Clients: make(map[string]transport.LSPClient),
		Router:  gateway.NewRouter(),
		Logger:  logger,
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

		gateway.Clients[serverConfig.Name] = client
		gateway.Router.RegisterServer(serverConfig.Name, serverConfig.Languages)
	}

	return &TestableGateway{Gateway: gateway, clientFactory: clientFactory}, nil
}

// createTestConfig creates a standard test configuration
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

// createSingleServerConfig creates a test configuration with a single server
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

// createMockClientFactory creates a factory function that returns MockLSPClient instances
func createMockClientFactory(t *testing.T) (func(transport.ClientConfig) (transport.LSPClient, error), map[string]*MockLSPClient) {
	mockClients := make(map[string]*MockLSPClient)
	factory := func(config transport.ClientConfig) (transport.LSPClient, error) {
		mock := NewMockLSPClient()
		mockClients[config.Command] = mock
		return mock, nil
	}
	return factory, mockClients
}

// TestMockLSPClientAccessibility verifies MockLSPClient is accessible across test files
func TestMockLSPClientAccessibility(t *testing.T) {
	t.Parallel()

	// Test that we can create a MockLSPClient
	mock := NewMockLSPClient()
	if mock == nil {
		t.Fatal("NewMockLSPClient() returned nil")
	}

	// Test basic interface compliance
	if !mock.IsActive() {
		// Client should not be active initially
	}

	ctx := context.Background()

	// Test Start method
	if err := mock.Start(ctx); err != nil {
		t.Fatalf("MockLSPClient.Start() failed: %v", err)
	}

	if !mock.IsActive() {
		t.Fatal("MockLSPClient should be active after Start()")
	}

	// Test request functionality
	response, err := mock.SendRequest(ctx, "test/method", map[string]interface{}{"test": "param"})
	if err != nil {
		t.Fatalf("MockLSPClient.SendRequest() failed: %v", err)
	}

	if response == nil {
		t.Fatal("MockLSPClient.SendRequest() returned nil response")
	}

	// Test notification functionality
	if err := mock.SendNotification(ctx, "test/notification", map[string]interface{}{"test": "param"}); err != nil {
		t.Fatalf("MockLSPClient.SendNotification() failed: %v", err)
	}

	// Test Stop method
	if err := mock.Stop(); err != nil {
		t.Fatalf("MockLSPClient.Stop() failed: %v", err)
	}

	if mock.IsActive() {
		t.Fatal("MockLSPClient should not be active after Stop()")
	}

	t.Log("MockLSPClient accessibility and basic functionality verified")
}
