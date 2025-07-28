package mocks

import (
	"context"
	"encoding/json"
	"sync"
)

type MockMcpClient struct {
	mu                   sync.RWMutex
	healthy              bool
	queuedResponses      []json.RawMessage
	responseIndex        int
	callCounts           map[string]int
	SendLSPRequestCalls  []SendLSPRequestCall
}

type SendLSPRequestCall struct {
	Ctx    context.Context
	Method string
	Params map[string]interface{}
}

func NewMockMcpClient() *MockMcpClient {
	return &MockMcpClient{
		healthy:             true,
		queuedResponses:     make([]json.RawMessage, 0),
		responseIndex:       0,
		callCounts:          make(map[string]int),
		SendLSPRequestCalls: make([]SendLSPRequestCall, 0),
	}
}

func (m *MockMcpClient) SetHealthy(healthy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthy = healthy
}

func (m *MockMcpClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queuedResponses = make([]json.RawMessage, 0)
	m.responseIndex = 0
	m.callCounts = make(map[string]int)
	m.SendLSPRequestCalls = make([]SendLSPRequestCall, 0)
}

func (m *MockMcpClient) SendLSPRequest(ctx context.Context, method string, params map[string]interface{}) (json.RawMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the call
	m.SendLSPRequestCalls = append(m.SendLSPRequestCalls, SendLSPRequestCall{
		Ctx:    ctx,
		Method: method,
		Params: params,
	})

	// Increment call count for the method
	m.callCounts[method]++

	// Return queued response if available
	if m.responseIndex < len(m.queuedResponses) {
		response := m.queuedResponses[m.responseIndex]
		m.responseIndex++
		return response, nil
	}

	// Return default empty response if no queued responses
	defaultResponse := json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":{}}`)
	return defaultResponse, nil
}

func (m *MockMcpClient) GetCallCount(method string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callCounts[method]
}

func (m *MockMcpClient) QueueResponse(response json.RawMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queuedResponses = append(m.queuedResponses, response)
}

func (m *MockMcpClient) IsHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.healthy
}