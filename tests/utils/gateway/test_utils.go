package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/gateway"
	"lsp-gateway/internal/transport"
)

// MockLSPClient provides a mock implementation of transport.LSPClient for testing
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

// NewMockLSPClient creates a new MockLSPClient
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

// TestableGateway wraps gateway.Gateway with test-specific functionality
type TestableGateway struct {
	*gateway.Gateway
	clientFactory func(transport.ClientConfig) (transport.LSPClient, error)
}

// NewTestableGateway creates a new TestableGateway with custom client factory
func NewTestableGateway(config *config.GatewayConfig, clientFactory func(transport.ClientConfig) (transport.LSPClient, error)) (*TestableGateway, error) {
	gw, err := gateway.NewGateway(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create gateway: %w", err)
	}

	return &TestableGateway{Gateway: gw, clientFactory: clientFactory}, nil
}

// MakeJSONRPCRequest sends an HTTP JSON-RPC request to the specified base URL
func MakeJSONRPCRequest(t *testing.T, baseURL string, request gateway.JSONRPCRequest) gateway.JSONRPCResponse {
	requestBody, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	resp, err := http.Post(baseURL+"/jsonrpc", "application/json", bytes.NewReader(requestBody))
	if err != nil {
		t.Fatalf("Failed to make HTTP request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("cleanup error closing response body: %v", err)
		}
	}()

	var response gateway.JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	return response
}

// CreateJSONRPCRequest creates a JSON-RPC request with the specified method and URI
func CreateJSONRPCRequest(method, uri string) []byte {
	var params interface{}

	if uri != "" {
		params = map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": uri,
			},
			"position": map[string]interface{}{
				"line":      10,
				"character": 5,
			},
		}
	}

	req := gateway.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}

	data, _ := json.Marshal(req)
	return data
}
