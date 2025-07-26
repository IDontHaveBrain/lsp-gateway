package mocks

import (
	"context"
	"encoding/json"
	"sync"
)

type MockMcpClient struct {
	mu                  sync.RWMutex
	requests            []MCPRequest
	queuedResponses     []json.RawMessage
	callCounts          map[string]int
	isHealthy           bool
	SendLSPRequestCalls []MCPRequestCall
}

type MCPRequestCall struct {
	Method string      `json:"method"`
	Params interface{} `json:"params"`
	Ctx    context.Context
}

type MCPRequest struct {
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

func NewMockMcpClient() *MockMcpClient {
	return &MockMcpClient{
		requests:            make([]MCPRequest, 0),
		queuedResponses:     make([]json.RawMessage, 0),
		callCounts:          make(map[string]int),
		isHealthy:           true,
		SendLSPRequestCalls: make([]MCPRequestCall, 0),
	}
}

func (m *MockMcpClient) SendLSPRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the request
	m.requests = append(m.requests, MCPRequest{Method: method, Params: params})
	m.SendLSPRequestCalls = append(m.SendLSPRequestCalls, MCPRequestCall{
		Method: method,
		Params: params,
		Ctx:    ctx,
	})

	// Increment call count
	m.callCounts[method]++

	// Check if we have queued responses
	if len(m.queuedResponses) > 0 {
		response := m.queuedResponses[0]
		m.queuedResponses = m.queuedResponses[1:]  // Always consume the response
		// Try to validate if the response format matches the expected method
		if m.isResponseCompatibleWithMethod(response, method) {
			return response, nil
		}
		// If not compatible, fall through to default responses
	}

	// Fall back to default responses
	switch method {
	case "textDocument/definition":
		return json.RawMessage(`[{"uri":"file://test.go","range":{"start":{"line":10,"character":0}}}]`), nil
	case "textDocument/hover":
		return json.RawMessage(`{"contents":"test hover content"}`), nil
	case "textDocument/references":
		return json.RawMessage(`[{"uri":"file://test.go","range":{"start":{"line":5,"character":0}}}]`), nil
	case "textDocument/documentSymbol":
		return json.RawMessage(`[{"name":"TestClass","kind":5,"range":{"start":{"line":1,"character":0},"end":{"line":10,"character":1}},"selectionRange":{"start":{"line":1,"character":6},"end":{"line":1,"character":15}},"children":[{"name":"testMethod","kind":6,"range":{"start":{"line":3,"character":4},"end":{"line":5,"character":5}},"selectionRange":{"start":{"line":3,"character":4},"end":{"line":3,"character":14}}},{"name":"testProperty","kind":7,"range":{"start":{"line":2,"character":4},"end":{"line":2,"character":16}},"selectionRange":{"start":{"line":2,"character":4},"end":{"line":2,"character":16}}}]},{"name":"TestFunction","kind":12,"range":{"start":{"line":12,"character":0},"end":{"line":15,"character":1}},"selectionRange":{"start":{"line":12,"character":9},"end":{"line":12,"character":21}}},{"name":"TestInterface","kind":11,"range":{"start":{"line":17,"character":0},"end":{"line":20,"character":1}},"selectionRange":{"start":{"line":17,"character":10},"end":{"line":17,"character":23}}},{"name":"TestEnum","kind":10,"range":{"start":{"line":22,"character":0},"end":{"line":26,"character":1}},"selectionRange":{"start":{"line":22,"character":5},"end":{"line":22,"character":13}}}]`), nil
	case "workspace/symbol":
		return json.RawMessage(`[{"name":"WorkspaceSymbol","kind":5,"location":{"uri":"file://test.go"}}]`), nil
	case "textDocument/completion":
		return json.RawMessage(`{"items":[{"label":"testCompletion","kind":1}]}`), nil
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

// SetHealthy sets the health status of the mock client
func (m *MockMcpClient) SetHealthy(healthy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isHealthy = healthy
}

// IsHealthy returns the current health status
func (m *MockMcpClient) IsHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isHealthy
}

// Reset clears all state of the mock client
func (m *MockMcpClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = m.requests[:0]
	m.queuedResponses = m.queuedResponses[:0]
	m.callCounts = make(map[string]int)
	m.SendLSPRequestCalls = m.SendLSPRequestCalls[:0]
	m.isHealthy = true
}

// QueueResponse adds a response to the queue that will be returned by SendLSPRequest
func (m *MockMcpClient) QueueResponse(response json.RawMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queuedResponses = append(m.queuedResponses, response)
}

// GetCallCount returns the number of times a specific method was called
func (m *MockMcpClient) GetCallCount(method string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callCounts[method]
}

// GetLastCall returns the last call made to SendLSPRequest
func (m *MockMcpClient) GetLastCall() *MCPRequestCall {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.SendLSPRequestCalls) == 0 {
		return nil
	}
	return &m.SendLSPRequestCalls[len(m.SendLSPRequestCalls)-1]
}

// GetQueuedResponsesCount returns the number of queued responses remaining
func (m *MockMcpClient) GetQueuedResponsesCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.queuedResponses)
}

// GetTotalCallCount returns the total number of calls made
func (m *MockMcpClient) GetTotalCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.SendLSPRequestCalls)
}

// isResponseCompatibleWithMethod checks if a queued response is compatible with the method being called
func (m *MockMcpClient) isResponseCompatibleWithMethod(response json.RawMessage, method string) bool {
	// Quick heuristic: check if response format matches expected format for the method
	responseStr := string(response)
	
	switch method {
	case "textDocument/documentSymbol", "textDocument/references", "workspace/symbol":
		// These methods expect arrays
		return len(responseStr) > 0 && responseStr[0] == '['
	case "textDocument/definition", "textDocument/hover", "textDocument/completion":
		// These methods can accept objects or arrays
		return len(responseStr) > 0 && (responseStr[0] == '{' || responseStr[0] == '[')
	default:
		// For unsupported methods, reject
		return false
	}
}