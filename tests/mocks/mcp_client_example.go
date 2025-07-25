package mocks

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"lsp-gateway/mcp"
)

// Example demonstrates how to use the MockMcpClient in tests
func ExampleMockMcpClient() {
	// Create a new mock client
	mockClient := NewMockMcpClient()
	
	// Example 1: Basic usage with default responses
	ctx := context.Background()
	result, err := mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_WORKSPACE_SYMBOL, map[string]interface{}{"query": "test"})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	var response map[string]interface{}
	json.Unmarshal(result, &response)
	fmt.Printf("Default response: %v\n", response)
	
	// Example 2: Queue custom responses
	customResponse := json.RawMessage(`{"custom": "response", "count": 42}`)
	mockClient.QueueResponse(customResponse)
	
	result2, _ := mockClient.SendLSPRequest(ctx, "custom/method", nil)
	var customResp map[string]interface{}
	json.Unmarshal(result2, &customResp)
	fmt.Printf("Custom response: %v\n", customResp)
	
	// Example 3: Queue errors
	mockClient.QueueError(fmt.Errorf("network timeout"))
	_, err3 := mockClient.SendLSPRequest(ctx, "failing/method", nil)
	fmt.Printf("Expected error: %v\n", err3)
	
	// Example 4: Custom function behavior
	mockClient.SendLSPRequestFunc = func(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
		if method == "special/method" {
			return json.RawMessage(`{"special": true}`), nil
		}
		return json.RawMessage(`{"generic": true}`), nil
	}
	
	specialResult, _ := mockClient.SendLSPRequest(ctx, "special/method", nil)
	genericResult, _ := mockClient.SendLSPRequest(ctx, "other/method", nil)
	
	var specialResp, genericResp map[string]interface{}
	json.Unmarshal(specialResult, &specialResp)
	json.Unmarshal(genericResult, &genericResp)
	
	fmt.Printf("Special response: %v\n", specialResp)
	fmt.Printf("Generic response: %v\n", genericResp)
	
	// Example 5: Verify call tracking
	fmt.Printf("Total calls made: %d\n", len(mockClient.SendLSPRequestCalls))
	fmt.Printf("Calls to workspace/symbol: %d\n", mockClient.GetCallCount(mcp.LSP_METHOD_WORKSPACE_SYMBOL))
	
	lastCall := mockClient.GetLastCall()
	if lastCall != nil {
		fmt.Printf("Last method called: %s\n", lastCall.Method)
	}
	
	// Example 6: Metrics manipulation
	mockClient.UpdateMetrics(10, 8, 2, 0, 0, time.Millisecond * 150)
	metrics := mockClient.GetMetrics()
	fmt.Printf("Success rate: %.1f%%\n", float64(metrics.SuccessfulReqs)/float64(metrics.TotalRequests)*100)
	
	// Example 7: Health status
	fmt.Printf("Is healthy: %v\n", mockClient.IsHealthy())
	mockClient.SetHealthy(false)
	fmt.Printf("After setting unhealthy: %v\n", mockClient.IsHealthy())
	
	// Example 8: Circuit breaker state
	fmt.Printf("Circuit breaker state: %v\n", mockClient.GetCircuitBreakerState())
	mockClient.SetCircuitBreakerState(mcp.CircuitOpen)
	fmt.Printf("After opening circuit: %v\n", mockClient.GetCircuitBreakerState())
	
	// Example 9: Error categorization
	networkErr := fmt.Errorf("network error")
	category := mockClient.CategorizeError(networkErr)
	fmt.Printf("Error category for 'network error': %v\n", category)
	
	// Example 10: Backoff calculation
	backoff := mockClient.CalculateBackoff(3)
	fmt.Printf("Backoff for attempt 3: %v\n", backoff)
	
	// Example 11: Reset for clean state
	mockClient.Reset()
	fmt.Printf("Calls after reset: %d\n", len(mockClient.SendLSPRequestCalls))
}

// TestScenarios demonstrates common testing scenarios
func TestScenarios() {
	// Scenario 1: Testing retry logic
	mockClient := NewMockMcpClient()
	
	// Queue errors first, then success
	mockClient.QueueError(fmt.Errorf("temporary failure"))
	mockClient.QueueError(fmt.Errorf("another failure"))
	mockClient.QueueResponse(json.RawMessage(`{"success": true}`))
	
	// Scenario 2: Testing circuit breaker behavior
	mockClient.SetCircuitBreakerState(mcp.CircuitOpen)
	
	// Test would verify that requests are rejected when circuit is open
	
	// Scenario 3: Testing timeout scenarios
	mockClient.SetTimeout(time.Millisecond * 100)
	
	// Test would verify timeout behavior
	
	// Scenario 4: Testing different LSP methods
	methods := []string{
		mcp.LSP_METHOD_WORKSPACE_SYMBOL,
		mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION,
		mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES,
		mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER,
		mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS,
	}
	
	ctx := context.Background()
	for _, method := range methods {
		result, err := mockClient.SendLSPRequest(ctx, method, nil)
		if err != nil {
			fmt.Printf("Method %s failed: %v\n", method, err)
		} else {
			fmt.Printf("Method %s succeeded with %d bytes\n", method, len(result))
		}
	}
}