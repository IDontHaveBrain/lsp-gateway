package framework

import (
	"context"
	"encoding/json"
	"sync"
)

type MockLSPServerManager struct {
	mu       sync.RWMutex
	running  bool
	requests []LSPRequest
}

type LSPRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

type LSPResponse struct {
	Result interface{} `json:"result,omitempty"`
	Error  *LSPError   `json:"error,omitempty"`
}

type LSPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewMockLSPServerManager() *MockLSPServerManager {
	return &MockLSPServerManager{
		requests: make([]LSPRequest, 0),
	}
}

func (m *MockLSPServerManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = true
	return nil
}

func (m *MockLSPServerManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
	return nil
}

func (m *MockLSPServerManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

func (m *MockLSPServerManager) SendRequest(method string, params map[string]interface{}) (*LSPResponse, error) {
	m.mu.Lock()
	m.requests = append(m.requests, LSPRequest{Method: method, Params: params})
	m.mu.Unlock()

	switch method {
	case "textDocument/definition":
		return &LSPResponse{
			Result: []map[string]interface{}{
				{"uri": "file://test.go", "range": map[string]interface{}{"start": map[string]int{"line": 10, "character": 0}}},
			},
		}, nil
	case "textDocument/hover":
		return &LSPResponse{
			Result: map[string]interface{}{"contents": "test hover content"},
		}, nil
	default:
		return &LSPResponse{Result: map[string]interface{}{}}, nil
	}
}

func (m *MockLSPServerManager) GetRequests() []LSPRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]LSPRequest{}, m.requests...)
}

func (m *MockLSPServerManager) ClearRequests() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = m.requests[:0]
}