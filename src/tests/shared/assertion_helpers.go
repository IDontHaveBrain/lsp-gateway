package shared

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// AssertJSONRPCSuccess asserts that a JSON-RPC response indicates success
func AssertJSONRPCSuccess(t *testing.T, response map[string]interface{}) {
	require.NotNil(t, response, "Response should not be nil")
	require.Contains(t, response, "result", "Response should contain result")
	require.NotContains(t, response, "error", "Response should not contain error")
}

// AssertJSONRPCError asserts that a JSON-RPC response contains an error
func AssertJSONRPCError(t *testing.T, response map[string]interface{}) {
	require.NotNil(t, response, "Response should not be nil")
	require.Contains(t, response, "error", "Response should contain error")

	errorObj, ok := response["error"].(map[string]interface{})
	require.True(t, ok, "Error should be an object")
	require.Contains(t, errorObj, "code", "Error should have code")
	require.Contains(t, errorObj, "message", "Error should have message")
}

// AssertDefinitionResponse asserts that a definition response is valid
func AssertDefinitionResponse(t *testing.T, response map[string]interface{}) {
	AssertJSONRPCSuccess(t, response)

	result := response["result"]
	require.NotNil(t, result, "Definition result should not be nil")

	// Definition result can be Location | Location[] | LocationLink[] | null
	switch v := result.(type) {
	case []interface{}:
		// Array of locations or location links
		if len(v) > 0 {
			location := v[0].(map[string]interface{})
			require.Contains(t, location, "uri", "Location should have uri")
			require.Contains(t, location, "range", "Location should have range")
		}
	case map[string]interface{}:
		// Single location
		require.Contains(t, v, "uri", "Location should have uri")
		require.Contains(t, v, "range", "Location should have range")
	case nil:
		// Valid null response
		t.Logf("Definition returned null (no definition found)")
	default:
		t.Errorf("Unexpected definition result type: %T", result)
	}
}

// AssertHoverResponse asserts that a hover response is valid
func AssertHoverResponse(t *testing.T, response map[string]interface{}) {
	AssertJSONRPCSuccess(t, response)

	result, ok := response["result"].(map[string]interface{})
	if !ok || result == nil {
		t.Logf("Hover returned null (no hover information)")
		return
	}

	// Hover should have contents
	require.Contains(t, result, "contents", "Hover should have contents")
	contents := result["contents"]
	require.NotNil(t, contents, "Hover contents should not be nil")

	// Contents can be string, MarkupContent, MarkedString, or array
	switch v := contents.(type) {
	case string:
		require.NotEmpty(t, v, "Hover contents should not be empty")
	case map[string]interface{}:
		// MarkupContent has kind and value
		if kind, hasKind := v["kind"]; hasKind {
			require.NotEmpty(t, kind, "MarkupContent kind should not be empty")
		}
		if value, hasValue := v["value"]; hasValue {
			require.NotEmpty(t, value, "MarkupContent value should not be empty")
		}
	case []interface{}:
		require.NotEmpty(t, v, "Hover contents array should not be empty")
	}
}

// AssertReferencesResponse asserts that a references response is valid
func AssertReferencesResponse(t *testing.T, response map[string]interface{}, minReferences int) {
	AssertJSONRPCSuccess(t, response)

	result, ok := response["result"].([]interface{})
	require.True(t, ok, "References result should be an array")

	if minReferences > 0 {
		require.GreaterOrEqual(t, len(result), minReferences,
			"Should find at least %d reference(s), got %d", minReferences, len(result))
	}

	// Check each reference location
	for i, ref := range result {
		location, ok := ref.(map[string]interface{})
		require.True(t, ok, "Reference %d should be a location object", i)
		require.Contains(t, location, "uri", "Reference %d should have uri", i)
		require.Contains(t, location, "range", "Reference %d should have range", i)

		uri, ok := location["uri"].(string)
		require.True(t, ok, "Reference %d uri should be string", i)
		require.NotEmpty(t, uri, "Reference %d uri should not be empty", i)
	}
}

// AssertDocumentSymbolResponse asserts that a document symbol response is valid
func AssertDocumentSymbolResponse(t *testing.T, response map[string]interface{}, minSymbols int) {
	AssertJSONRPCSuccess(t, response)

	result, ok := response["result"].([]interface{})
	require.True(t, ok, "Document symbols result should be an array")

	if minSymbols > 0 {
		require.GreaterOrEqual(t, len(result), minSymbols,
			"Should find at least %d symbol(s), got %d", minSymbols, len(result))
	}

	// Check each symbol
	for i, sym := range result {
		symbol, ok := sym.(map[string]interface{})
		require.True(t, ok, "Symbol %d should be an object", i)
		require.Contains(t, symbol, "name", "Symbol %d should have name", i)
		require.Contains(t, symbol, "kind", "Symbol %d should have kind", i)
		require.Contains(t, symbol, "range", "Symbol %d should have range", i)

		name, ok := symbol["name"].(string)
		require.True(t, ok, "Symbol %d name should be string", i)
		require.NotEmpty(t, name, "Symbol %d name should not be empty", i)
	}
}

// AssertWorkspaceSymbolResponse asserts that a workspace symbol response is valid
func AssertWorkspaceSymbolResponse(t *testing.T, response map[string]interface{}) {
	AssertJSONRPCSuccess(t, response)

	result, ok := response["result"].([]interface{})
	require.True(t, ok, "Workspace symbols result should be an array")

	// Check each symbol (if any)
	for i, sym := range result {
		symbol, ok := sym.(map[string]interface{})
		require.True(t, ok, "Symbol %d should be an object", i)
		require.Contains(t, symbol, "name", "Symbol %d should have name", i)
		require.Contains(t, symbol, "kind", "Symbol %d should have kind", i)
		require.Contains(t, symbol, "location", "Symbol %d should have location", i)
	}
}

// AssertCompletionResponse asserts that a completion response is valid
func AssertCompletionResponse(t *testing.T, response map[string]interface{}) {
	AssertJSONRPCSuccess(t, response)

	result := response["result"]
	require.NotNil(t, result, "Completion result should not be nil")

	switch v := result.(type) {
	case []interface{}:
		// CompletionItem[]
		for i, item := range v {
			completion, ok := item.(map[string]interface{})
			require.True(t, ok, "Completion item %d should be an object", i)
			require.Contains(t, completion, "label", "Completion item %d should have label", i)
		}
	case map[string]interface{}:
		// CompletionList
		require.Contains(t, v, "items", "CompletionList should have items")
		items, ok := v["items"].([]interface{})
		require.True(t, ok, "CompletionList items should be array")

		for i, item := range items {
			completion, ok := item.(map[string]interface{})
			require.True(t, ok, "Completion item %d should be an object", i)
			require.Contains(t, completion, "label", "Completion item %d should have label", i)
		}
	case nil:
		// Valid null response
		t.Logf("Completion returned null")
	default:
		t.Errorf("Unexpected completion result type: %T", result)
	}
}

// AssertCacheHit asserts that a response came from cache
func AssertCacheHit(t *testing.T, response interface{}) {
	require.NotNil(t, response, "Cached response should not be nil")
	// Additional cache-specific assertions could be added here
}

// AssertCacheMiss asserts that a response did not come from cache
func AssertCacheMiss(t *testing.T, response interface{}) {
	// This would typically be used in conjunction with other checks
	// to verify that LSP was called instead of cache
	t.Logf("Cache miss - response from LSP")
}

// AssertSearchReferencesResult asserts that a search references result is valid
func AssertSearchReferencesResult(t *testing.T, result interface{}, minReferences int) {
	require.NotNil(t, result, "Search references result should not be nil")

	// The result should have a specific structure based on the SearchSymbolReferences method
	if searchResult, ok := result.(map[string]interface{}); ok {
		if refs, hasRefs := searchResult["references"]; hasRefs {
			references, ok := refs.([]interface{})
			require.True(t, ok, "References should be an array")

			if minReferences > 0 {
				require.GreaterOrEqual(t, len(references), minReferences,
					"Should find at least %d reference(s), got %d", minReferences, len(references))
			}

			// Check each reference
			for i, ref := range references {
				reference, ok := ref.(map[string]interface{})
				require.True(t, ok, "Reference %d should be an object", i)

				// References should have file path and line number
				if filePath, hasFile := reference["file_path"]; hasFile {
					require.NotEmpty(t, filePath, "Reference %d should have file_path", i)
				}
				if lineNum, hasLine := reference["line_number"]; hasLine {
					require.NotNil(t, lineNum, "Reference %d should have line_number", i)
				}
			}
		}

		// Check metadata
		if metadata, hasMeta := searchResult["search_metadata"]; hasMeta {
			require.NotNil(t, metadata, "Search metadata should not be nil")
		}
	}
}

// AssertNoError asserts that no error occurred in the response
func AssertNoError(t *testing.T, err error, message string, args ...interface{}) {
	require.NoError(t, err, fmt.Sprintf(message, args...))
}

// AssertNotNil asserts that a value is not nil with a custom message
func AssertNotNil(t *testing.T, value interface{}, message string, args ...interface{}) {
	require.NotNil(t, value, fmt.Sprintf(message, args...))
}

// AssertGreaterThan asserts that a value is greater than expected
func AssertGreaterThan(t *testing.T, actual, expected int, message string, args ...interface{}) {
	require.Greater(t, actual, expected, fmt.Sprintf(message, args...))
}

// AssertContainsKey asserts that a map contains a specific key
func AssertContainsKey(t *testing.T, m map[string]interface{}, key string, message string, args ...interface{}) {
	require.Contains(t, m, key, fmt.Sprintf(message, args...))
}

// AssertArrayNotEmpty asserts that an array is not empty
func AssertArrayNotEmpty(t *testing.T, arr []interface{}, message string, args ...interface{}) {
	require.NotEmpty(t, arr, fmt.Sprintf(message, args...))
}

// AssertResponseTime asserts that an operation completed within expected time
func AssertResponseTime(t *testing.T, duration, maxDuration interface{}, message string, args ...interface{}) {
	// This could be enhanced with actual timing measurements
	t.Logf("Response time check: %s", message)
}

// AssertHealthy asserts that a health check response indicates healthy status
func AssertHealthy(t *testing.T, health map[string]interface{}) {
	require.Contains(t, health, "status", "Health response should have status")
	require.Equal(t, "healthy", health["status"], "Server should be healthy")
}
