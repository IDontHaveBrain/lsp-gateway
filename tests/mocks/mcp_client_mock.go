package mocks

import (
	"context"
	"encoding/json"
	"sync"
)

type MockMcpClient struct {
	mu       sync.RWMutex
	requests []MCPRequest
}

type MCPRequest struct {
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

func NewMockMcpClient() *MockMcpClient {
	return &MockMcpClient{
		requests: make([]MCPRequest, 0),
	}
}

func (m *MockMcpClient) SendLSPRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	m.mu.Lock()
	m.requests = append(m.requests, MCPRequest{Method: method, Params: params})
	m.mu.Unlock()

	switch method {
	case "textDocument/definition":
		return json.RawMessage(`[{"uri":"file://test.go","range":{"start":{"line":10,"character":0}}}]`), nil
	case "textDocument/hover":
		return json.RawMessage(`{"contents":"test hover content"}`), nil
	default:
		return json.RawMessage(`{}`), nil
	}
}

func (m *MockMcpClient) GetRequests() []MCPRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]MCPRequest{}, m.requests...)
}

func (m *MockMcpClient) ClearRequests() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = m.requests[:0]
}