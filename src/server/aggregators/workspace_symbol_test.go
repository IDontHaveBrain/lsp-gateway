package aggregators

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLSPClient implements the minimal LSPClient interface for testing
type mockLSPClient struct {
	supportsWorkspace bool
	response          json.RawMessage
	err               error
}

func (m *mockLSPClient) Start(ctx context.Context) error {
	return nil
}

func (m *mockLSPClient) Stop() error {
	return nil
}

func (m *mockLSPClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	return m.response, m.err
}

func (m *mockLSPClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	return nil
}

func (m *mockLSPClient) IsActive() bool {
	return true
}

func (m *mockLSPClient) Supports(method string) bool {
	return m.supportsWorkspace
}

func TestWorkspaceSymbolAggregator_HandlesNullResponse(t *testing.T) {
	aggregator := NewWorkspaceSymbolAggregator()

	// Test with null response
	clients := map[string]interface{}{
		"go": &mockLSPClient{
			supportsWorkspace: true,
			response:          json.RawMessage("null"),
			err:               nil,
		},
	}

	params := map[string]interface{}{
		"query": "test",
	}

	result, err := aggregator.ProcessWorkspaceSymbol(context.Background(), clients, params)
	require.NoError(t, err, "Should handle null response without error")
	
	// Result should be an empty array
	symbols, ok := result.([]interface{})
	assert.True(t, ok, "Result should be an array")
	assert.Empty(t, symbols, "Result should be empty for null response")
}

func TestWorkspaceSymbolAggregator_HandlesEmptyResponse(t *testing.T) {
	aggregator := NewWorkspaceSymbolAggregator()

	// Test with empty response
	clients := map[string]interface{}{
		"go": &mockLSPClient{
			supportsWorkspace: true,
			response:          json.RawMessage(""),
			err:               nil,
		},
	}

	params := map[string]interface{}{
		"query": "test",
	}

	result, err := aggregator.ProcessWorkspaceSymbol(context.Background(), clients, params)
	require.NoError(t, err, "Should handle empty response without error")
	
	// Result should be an empty array
	symbols, ok := result.([]interface{})
	assert.True(t, ok, "Result should be an array")
	assert.Empty(t, symbols, "Result should be empty for empty response")
}

func TestWorkspaceSymbolAggregator_HandlesValidArrayResponse(t *testing.T) {
	aggregator := NewWorkspaceSymbolAggregator()

	// Test with valid array response
	validResponse := `[{"name": "TestSymbol", "kind": 12}]`
	clients := map[string]interface{}{
		"go": &mockLSPClient{
			supportsWorkspace: true,
			response:          json.RawMessage(validResponse),
			err:               nil,
		},
	}

	params := map[string]interface{}{
		"query": "test",
	}

	result, err := aggregator.ProcessWorkspaceSymbol(context.Background(), clients, params)
	require.NoError(t, err, "Should handle valid array response without error")
	
	// Result should be an array with one symbol
	symbols, ok := result.([]interface{})
	assert.True(t, ok, "Result should be an array")
	assert.Len(t, symbols, 1, "Result should contain one symbol")
}

func TestWorkspaceSymbolAggregator_HandlesEmptyArrayResponse(t *testing.T) {
	aggregator := NewWorkspaceSymbolAggregator()

	// Test with empty array response
	clients := map[string]interface{}{
		"go": &mockLSPClient{
			supportsWorkspace: true,
			response:          json.RawMessage("[]"),
			err:               nil,
		},
	}

	params := map[string]interface{}{
		"query": "test",
	}

	result, err := aggregator.ProcessWorkspaceSymbol(context.Background(), clients, params)
	require.NoError(t, err, "Should handle empty array response without error")
	
	// Result should be an empty array
	symbols, ok := result.([]interface{})
	assert.True(t, ok, "Result should be an array")
	assert.Empty(t, symbols, "Result should be empty for empty array response")
}