package e2e_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"lsp-gateway/mcp"
)

// AssertionHelper provides utilities for making assertions in E2E tests
type AssertionHelper struct {
	t           *testing.T
	failOnError bool
}

// NewAssertionHelper creates a new assertion helper
func NewAssertionHelper(t *testing.T) *AssertionHelper {
	return &AssertionHelper{
		t:           t,
		failOnError: true,
	}
}

// AssertLSPResponseStructure validates the structure of an LSP response
func (a *AssertionHelper) AssertLSPResponseStructure(method string, response json.RawMessage) bool {
	switch method {
	case mcp.LSP_METHOD_WORKSPACE_SYMBOL:
		return a.assertWorkspaceSymbolResponse(response)
	case mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION:
		return a.assertDefinitionResponse(response)
	case mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES:
		return a.assertReferencesResponse(response)
	case mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER:
		return a.assertHoverResponse(response)
	case mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS:
		return a.assertDocumentSymbolsResponse(response)
	default:
		return a.logError(fmt.Sprintf("Unknown LSP method for response validation: %s", method))
	}
}

// AssertMetricsImprovement checks if metrics show improvement over baseline
func (a *AssertionHelper) AssertMetricsImprovement(current, baseline mcp.ConnectionMetrics) bool {
	success := true
	
	// Check success rate improvement
	currentSuccessRate := float64(current.SuccessfulReqs) / float64(current.TotalRequests)
	baselineSuccessRate := float64(baseline.SuccessfulReqs) / float64(baseline.TotalRequests)
	
	if currentSuccessRate < baselineSuccessRate {
		success = a.logError(fmt.Sprintf("Success rate degraded: current %.2f < baseline %.2f", 
			currentSuccessRate, baselineSuccessRate)) && success
	}
	
	// Check latency improvement (current should be better or within 10% tolerance)
	latencyTolerance := float64(baseline.AverageLatency) * 0.1
	if float64(current.AverageLatency) > float64(baseline.AverageLatency)+latencyTolerance {
		success = a.logError(fmt.Sprintf("Latency degraded: current %v > baseline %v + tolerance %v", 
			current.AverageLatency, baseline.AverageLatency, time.Duration(latencyTolerance))) && success
	}
	
	return success
}

// AssertCircuitBreakerBehavior validates circuit breaker state transitions
func (a *AssertionHelper) AssertCircuitBreakerBehavior(states []mcp.CircuitBreakerState, expectedTransitions []mcp.CircuitBreakerState) bool {
	if len(states) != len(expectedTransitions) {
		return a.logError(fmt.Sprintf("Circuit breaker state count mismatch: got %d, expected %d", 
			len(states), len(expectedTransitions)))
	}
	
	for i, state := range states {
		if state != expectedTransitions[i] {
			return a.logError(fmt.Sprintf("Circuit breaker state mismatch at position %d: got %v, expected %v", 
				i, state, expectedTransitions[i]))
		}
	}
	
	return true
}

// AssertPerformanceThresholds validates performance metrics against thresholds
func (a *AssertionHelper) AssertPerformanceThresholds(metrics mcp.ConnectionMetrics, thresholds PerformanceThresholds) bool {
	success := true
	
	// Check response time threshold
	if metrics.AverageLatency > thresholds.MaxResponseTime {
		success = a.logError(fmt.Sprintf("Response time threshold exceeded: %v > %v", 
			metrics.AverageLatency, thresholds.MaxResponseTime)) && success
	}
	
	// Check error rate threshold
	errorRate := float64(metrics.FailedRequests) / float64(metrics.TotalRequests)
	if errorRate > thresholds.MaxErrorRate {
		success = a.logError(fmt.Sprintf("Error rate threshold exceeded: %.2f > %.2f", 
			errorRate, thresholds.MaxErrorRate)) && success
	}
	
	// Check minimum throughput (if we have timing data)
	if thresholds.MinThroughput > 0 {
		// This would need actual timing data to calculate properly
		// For now, we'll just check that we have some successful requests
		if metrics.SuccessfulReqs == 0 {
			success = a.logError("No successful requests for throughput calculation") && success
		}
	}
	
	return success
}

// AssertResponseContainsFields checks that a JSON response contains required fields
func (a *AssertionHelper) AssertResponseContainsFields(response json.RawMessage, requiredFields []string) bool {
	var data map[string]interface{}
	if err := json.Unmarshal(response, &data); err != nil {
		return a.logError(fmt.Sprintf("Failed to unmarshal response: %v", err))
	}
	
	for _, field := range requiredFields {
		if _, exists := data[field]; !exists {
			return a.logError(fmt.Sprintf("Required field '%s' not found in response", field))
		}
	}
	
	return true
}

// AssertEqualsWithTolerance checks if two values are equal within a tolerance
func (a *AssertionHelper) AssertEqualsWithTolerance(actual, expected, tolerance float64) bool {
	diff := actual - expected
	if diff < 0 {
		diff = -diff
	}
	
	if diff > tolerance {
		return a.logError(fmt.Sprintf("Values not equal within tolerance: |%.2f - %.2f| = %.2f > %.2f", 
			actual, expected, diff, tolerance))
	}
	
	return true
}

// AssertNotEmpty checks that a value is not empty
func (a *AssertionHelper) AssertNotEmpty(value interface{}, name string) bool {
	if value == nil {
		return a.logError(fmt.Sprintf("%s is nil", name))
	}
	
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String:
		if v.Len() == 0 {
			return a.logError(fmt.Sprintf("%s is empty string", name))
		}
	case reflect.Slice, reflect.Array, reflect.Map:
		if v.Len() == 0 {
			return a.logError(fmt.Sprintf("%s is empty", name))
		}
	case reflect.Chan:
		if v.IsNil() {
			return a.logError(fmt.Sprintf("%s channel is nil", name))
		}
	}
	
	return true
}

// Performance threshold definitions
type PerformanceThresholds struct {
	MaxResponseTime time.Duration
	MaxErrorRate    float64
	MinThroughput   float64
}

// Private helper methods

func (a *AssertionHelper) assertWorkspaceSymbolResponse(response json.RawMessage) bool {
	var symbols []map[string]interface{}
	if err := json.Unmarshal(response, &symbols); err != nil {
		return a.logError(fmt.Sprintf("Failed to unmarshal workspace symbol response: %v", err))
	}
	
	if len(symbols) == 0 {
		return a.logError("Workspace symbol response is empty")
	}
	
	// Check first symbol has required fields
	symbol := symbols[0]
	requiredFields := []string{"name", "kind", "location"}
	for _, field := range requiredFields {
		if _, exists := symbol[field]; !exists {
			return a.logError(fmt.Sprintf("Workspace symbol missing required field: %s", field))
		}
	}
	
	// Check location structure
	location, ok := symbol["location"].(map[string]interface{})
	if !ok {
		return a.logError("Workspace symbol location is not an object")
	}
	
	locationFields := []string{"uri", "range"}
	for _, field := range locationFields {
		if _, exists := location[field]; !exists {
			return a.logError(fmt.Sprintf("Workspace symbol location missing field: %s", field))
		}
	}
	
	return true
}

func (a *AssertionHelper) assertDefinitionResponse(response json.RawMessage) bool {
	var definition map[string]interface{}
	if err := json.Unmarshal(response, &definition); err != nil {
		return a.logError(fmt.Sprintf("Failed to unmarshal definition response: %v", err))
	}
	
	requiredFields := []string{"uri", "range"}
	for _, field := range requiredFields {
		if _, exists := definition[field]; !exists {
			return a.logError(fmt.Sprintf("Definition response missing required field: %s", field))
		}
	}
	
	return a.assertRangeStructure(definition["range"])
}

func (a *AssertionHelper) assertReferencesResponse(response json.RawMessage) bool {
	var references []map[string]interface{}
	if err := json.Unmarshal(response, &references); err != nil {
		return a.logError(fmt.Sprintf("Failed to unmarshal references response: %v", err))
	}
	
	if len(references) == 0 {
		return a.logError("References response is empty")
	}
	
	// Check first reference has required fields
	ref := references[0]
	requiredFields := []string{"uri", "range"}
	for _, field := range requiredFields {
		if _, exists := ref[field]; !exists {
			return a.logError(fmt.Sprintf("Reference missing required field: %s", field))
		}
	}
	
	return a.assertRangeStructure(ref["range"])
}

func (a *AssertionHelper) assertHoverResponse(response json.RawMessage) bool {
	var hover map[string]interface{}
	if err := json.Unmarshal(response, &hover); err != nil {
		return a.logError(fmt.Sprintf("Failed to unmarshal hover response: %v", err))
	}
	
	if _, exists := hover["contents"]; !exists {
		return a.logError("Hover response missing contents field")
	}
	
	return true
}

func (a *AssertionHelper) assertDocumentSymbolsResponse(response json.RawMessage) bool {
	var symbols []map[string]interface{}
	if err := json.Unmarshal(response, &symbols); err != nil {
		return a.logError(fmt.Sprintf("Failed to unmarshal document symbols response: %v", err))
	}
	
	if len(symbols) == 0 {
		return a.logError("Document symbols response is empty")
	}
	
	// Check first symbol has required fields
	symbol := symbols[0]
	requiredFields := []string{"name", "kind", "range"}
	for _, field := range requiredFields {
		if _, exists := symbol[field]; !exists {
			return a.logError(fmt.Sprintf("Document symbol missing required field: %s", field))
		}
	}
	
	return a.assertRangeStructure(symbol["range"])
}

func (a *AssertionHelper) assertRangeStructure(rangeData interface{}) bool {
	rangeObj, ok := rangeData.(map[string]interface{})
	if !ok {
		return a.logError("Range is not an object")
	}
	
	requiredFields := []string{"start", "end"}
	for _, field := range requiredFields {
		if _, exists := rangeObj[field]; !exists {
			return a.logError(fmt.Sprintf("Range missing required field: %s", field))
		}
	}
	
	// Check start and end positions
	for _, posField := range []string{"start", "end"} {
		pos, ok := rangeObj[posField].(map[string]interface{})
		if !ok {
			return a.logError(fmt.Sprintf("Range %s is not an object", posField))
		}
		
		posFields := []string{"line", "character"}
		for _, field := range posFields {
			if _, exists := pos[field]; !exists {
				return a.logError(fmt.Sprintf("Range %s missing field: %s", posField, field))
			}
		}
	}
	
	return true
}

func (a *AssertionHelper) logError(message string) bool {
	if a.failOnError && a.t != nil {
		a.t.Error(message)
	} else {
		fmt.Printf("Assertion failed: %s\n", message)
	}
	return false
}

// SetFailOnError controls whether assertion failures cause test failures
func (a *AssertionHelper) SetFailOnError(fail bool) {
	a.failOnError = fail
}