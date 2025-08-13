package testutils

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"lsp-gateway/src/utils"
)

var requestIDCounter int64

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      interface{} `json:"id"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

// JSONRPCRequestBuilder provides a fluent interface for building JSON-RPC requests
type JSONRPCRequestBuilder struct {
	method string
	params interface{}
	id     interface{}
}

// NewJSONRPCRequestBuilder creates a new builder instance
func NewJSONRPCRequestBuilder() *JSONRPCRequestBuilder {
	return &JSONRPCRequestBuilder{}
}

// WithMethod sets the JSON-RPC method
func (b *JSONRPCRequestBuilder) WithMethod(method string) *JSONRPCRequestBuilder {
	b.method = method
	return b
}

// WithParams sets the request parameters
func (b *JSONRPCRequestBuilder) WithParams(params interface{}) *JSONRPCRequestBuilder {
	b.params = params
	return b
}

// WithID sets the request ID
func (b *JSONRPCRequestBuilder) WithID(id interface{}) *JSONRPCRequestBuilder {
	b.id = id
	return b
}

// Build creates the final JSON-RPC request
func (b *JSONRPCRequestBuilder) Build() *JSONRPCRequest {
	id := b.id
	if id == nil {
		id = atomic.AddInt64(&requestIDCounter, 1)
	}

	return &JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  b.method,
		Params:  b.params,
		ID:      id,
	}
}

// generateID generates a unique request ID
func generateID() int64 {
	return atomic.AddInt64(&requestIDCounter, 1)
}

// Basic Request Builders

// CreateJSONRPCRequest creates a basic JSON-RPC request with automatic ID generation
func CreateJSONRPCRequest(method string, params interface{}, id interface{}) *JSONRPCRequest {
	if id == nil {
		id = generateID()
	}
	return &JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      id,
	}
}

// LSP Method Builders

// CreateHoverRequest creates a textDocument/hover request
func CreateHoverRequest(uri string, line, character int) *JSONRPCRequest {
	return CreateHoverRequestWithID(uri, line, character, nil)
}

// CreateHoverRequestWithID creates a textDocument/hover request with custom ID
func CreateHoverRequestWithID(uri string, line, character int, id interface{}) *JSONRPCRequest {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": character,
		},
	}
	return CreateJSONRPCRequest("textDocument/hover", params, id)
}

// CreateDefinitionRequest creates a textDocument/definition request
func CreateDefinitionRequest(uri string, line, character int) *JSONRPCRequest {
	return CreateDefinitionRequestWithID(uri, line, character, nil)
}

// CreateDefinitionRequestWithID creates a textDocument/definition request with custom ID
func CreateDefinitionRequestWithID(uri string, line, character int, id interface{}) *JSONRPCRequest {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": character,
		},
	}
	return CreateJSONRPCRequest("textDocument/definition", params, id)
}

// CreateReferencesRequest creates a textDocument/references request
func CreateReferencesRequest(uri string, line, character int) *JSONRPCRequest {
	return CreateReferencesRequestWithID(uri, line, character, true, nil)
}

// CreateReferencesRequestWithID creates a textDocument/references request with custom options
func CreateReferencesRequestWithID(uri string, line, character int, includeDeclaration bool, id interface{}) *JSONRPCRequest {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": character,
		},
		"context": map[string]interface{}{
			"includeDeclaration": includeDeclaration,
		},
	}
	return CreateJSONRPCRequest("textDocument/references", params, id)
}

// CreateSymbolsRequest creates a textDocument/documentSymbol request
func CreateSymbolsRequest(uri string) *JSONRPCRequest {
	return CreateSymbolsRequestWithID(uri, nil)
}

// CreateSymbolsRequestWithID creates a textDocument/documentSymbol request with custom ID
func CreateSymbolsRequestWithID(uri string, id interface{}) *JSONRPCRequest {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
	}
	return CreateJSONRPCRequest("textDocument/documentSymbol", params, id)
}

// CreateWorkspaceSymbolsRequest creates a workspace/symbol request
func CreateWorkspaceSymbolsRequest(query string) *JSONRPCRequest {
	return CreateWorkspaceSymbolsRequestWithID(query, nil)
}

// CreateWorkspaceSymbolsRequestWithID creates a workspace/symbol request with custom ID
func CreateWorkspaceSymbolsRequestWithID(query string, id interface{}) *JSONRPCRequest {
	params := map[string]interface{}{
		"query": query,
	}
	return CreateJSONRPCRequest("workspace/symbol", params, id)
}

// CreateCompletionRequest creates a textDocument/completion request
func CreateCompletionRequest(uri string, line, character int) *JSONRPCRequest {
	return CreateCompletionRequestWithID(uri, line, character, nil)
}

// CreateCompletionRequestWithID creates a textDocument/completion request with custom ID
func CreateCompletionRequestWithID(uri string, line, character int, id interface{}) *JSONRPCRequest {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": character,
		},
	}
	return CreateJSONRPCRequest("textDocument/completion", params, id)
}

// Advanced Request Builders

// CreateBatchRequest creates a batch JSON-RPC request
func CreateBatchRequest(requests ...*JSONRPCRequest) []*JSONRPCRequest {
	return requests
}

// CreateDidOpenRequest creates a textDocument/didOpen notification
func CreateDidOpenRequest(uri, languageId string, version int, text string) *JSONRPCRequest {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        uri,
			"languageId": languageId,
			"version":    version,
			"text":       text,
		},
	}
	return CreateJSONRPCRequest("textDocument/didOpen", params, nil)
}

// CreateDidCloseRequest creates a textDocument/didClose notification
func CreateDidCloseRequest(uri string) *JSONRPCRequest {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
	}
	return CreateJSONRPCRequest("textDocument/didClose", params, nil)
}

// CreateDidChangeRequest creates a textDocument/didChange notification
func CreateDidChangeRequest(uri string, version int, changes []map[string]interface{}) *JSONRPCRequest {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":     uri,
			"version": version,
		},
		"contentChanges": changes,
	}
	return CreateJSONRPCRequest("textDocument/didChange", params, nil)
}

// CreateInitializeRequest creates an LSP initialize request
func CreateInitializeRequest(rootURI string, capabilities map[string]interface{}) *JSONRPCRequest {
	return CreateInitializeRequestWithID(rootURI, capabilities, nil)
}

// CreateInitializeRequestWithID creates an LSP initialize request with custom ID
func CreateInitializeRequestWithID(rootURI string, capabilities map[string]interface{}, id interface{}) *JSONRPCRequest {
	if capabilities == nil {
		capabilities = map[string]interface{}{
			"textDocument": map[string]interface{}{
				"definition": map[string]interface{}{
					"dynamicRegistration": false,
				},
				"hover": map[string]interface{}{
					"dynamicRegistration": false,
				},
				"references": map[string]interface{}{
					"dynamicRegistration": false,
				},
				"documentSymbol": map[string]interface{}{
					"dynamicRegistration": false,
				},
				"completion": map[string]interface{}{
					"dynamicRegistration": false,
				},
			},
			"workspace": map[string]interface{}{
				"symbol": map[string]interface{}{
					"dynamicRegistration": false,
				},
			},
		}
	}

	params := map[string]interface{}{
		"processId":    nil,
		"rootUri":      rootURI,
		"capabilities": capabilities,
	}
	return CreateJSONRPCRequest("initialize", params, id)
}

// File Path Helpers

// CreateHoverRequestFromFile creates a hover request using file path
func CreateHoverRequestFromFile(filePath string, line, character int) *JSONRPCRequest {
	uri := utils.FilePathToURI(filePath)
	return CreateHoverRequest(uri, line, character)
}

// CreateDefinitionRequestFromFile creates a definition request using file path
func CreateDefinitionRequestFromFile(filePath string, line, character int) *JSONRPCRequest {
	uri := utils.FilePathToURI(filePath)
	return CreateDefinitionRequest(uri, line, character)
}

// CreateReferencesRequestFromFile creates a references request using file path
func CreateReferencesRequestFromFile(filePath string, line, character int) *JSONRPCRequest {
	uri := utils.FilePathToURI(filePath)
	return CreateReferencesRequest(uri, line, character)
}

// CreateSymbolsRequestFromFile creates a document symbols request using file path
func CreateSymbolsRequestFromFile(filePath string) *JSONRPCRequest {
	uri := utils.FilePathToURI(filePath)
	return CreateSymbolsRequest(uri)
}

// CreateCompletionRequestFromFile creates a completion request using file path
func CreateCompletionRequestFromFile(filePath string, line, character int) *JSONRPCRequest {
	uri := utils.FilePathToURI(filePath)
	return CreateCompletionRequest(uri, line, character)
}

// CreateDidOpenRequestFromFile creates a didOpen notification using file path
func CreateDidOpenRequestFromFile(filePath, languageId string, version int, text string) *JSONRPCRequest {
	uri := utils.FilePathToURI(filePath)
	return CreateDidOpenRequest(uri, languageId, version, text)
}

// CreateDidCloseRequestFromFile creates a didClose notification using file path
func CreateDidCloseRequestFromFile(filePath string) *JSONRPCRequest {
	uri := utils.FilePathToURI(filePath)
	return CreateDidCloseRequest(uri)
}

// CreateInitializeRequestFromFile creates an initialize request using root path
func CreateInitializeRequestFromFile(rootPath string, capabilities map[string]interface{}) *JSONRPCRequest {
	rootURI := utils.FilePathToURI(rootPath)
	return CreateInitializeRequest(rootURI, capabilities)
}

// Response Validation Helpers

// ValidateJSONRPCResponse validates the basic structure of a JSON-RPC response
func ValidateJSONRPCResponse(t *testing.T, response map[string]interface{}) {
	require.NotNil(t, response, "Response should not be nil")
	require.Contains(t, response, "jsonrpc", "Response should contain jsonrpc field")
	require.Equal(t, "2.0", response["jsonrpc"], "Response should be JSON-RPC 2.0")
	require.Contains(t, response, "id", "Response should contain id field")
}

// ExtractResult extracts the result from a JSON-RPC response
func ExtractResult(t *testing.T, response map[string]interface{}) interface{} {
	ValidateJSONRPCResponse(t, response)
	require.Contains(t, response, "result", "Response should contain result field")
	require.NotContains(t, response, "error", "Response should not contain error field")
	return response["result"]
}

// ExtractError extracts the error from a JSON-RPC response
func ExtractError(t *testing.T, response map[string]interface{}) map[string]interface{} {
	ValidateJSONRPCResponse(t, response)
	require.Contains(t, response, "error", "Response should contain error field")
	require.NotContains(t, response, "result", "Response should not contain result field")

	errorObj, ok := response["error"].(map[string]interface{})
	require.True(t, ok, "Error should be an object")
	require.Contains(t, errorObj, "code", "Error should have code")
	require.Contains(t, errorObj, "message", "Error should have message")

	return errorObj
}

// AssertSuccess asserts that a JSON-RPC response indicates success
func AssertSuccess(t *testing.T, response map[string]interface{}) interface{} {
	return ExtractResult(t, response)
}

// AssertError asserts that a JSON-RPC response contains an error
func AssertError(t *testing.T, response map[string]interface{}) map[string]interface{} {
	return ExtractError(t, response)
}

// Advanced Response Helpers

// ParseJSONRPCResponse parses a raw JSON-RPC response from bytes
func ParseJSONRPCResponse(t *testing.T, data []byte) map[string]interface{} {
	var response map[string]interface{}
	err := json.Unmarshal(data, &response)
	require.NoError(t, err, "Failed to parse JSON-RPC response")
	return response
}

// ValidateDefinitionResult validates a textDocument/definition response result
func ValidateDefinitionResult(t *testing.T, result interface{}) {
	require.NotNil(t, result, "Definition result should not be nil")

	switch v := result.(type) {
	case []interface{}:
		if len(v) > 0 {
			location := v[0].(map[string]interface{})
			require.Contains(t, location, "uri", "Location should have uri")
			require.Contains(t, location, "range", "Location should have range")
		}
	case map[string]interface{}:
		require.Contains(t, v, "uri", "Location should have uri")
		require.Contains(t, v, "range", "Location should have range")
	case nil:
		// Valid null response
	default:
		require.Fail(t, fmt.Sprintf("Unexpected definition result type: %T", result))
	}
}

// ValidateHoverResult validates a textDocument/hover response result
func ValidateHoverResult(t *testing.T, result interface{}) {
	if result == nil {
		return // Null hover is valid
	}

	hoverObj, ok := result.(map[string]interface{})
	require.True(t, ok, "Hover result should be an object or null")
	require.Contains(t, hoverObj, "contents", "Hover should have contents")
}

// ValidateSymbolsResult validates a document symbols response result
func ValidateSymbolsResult(t *testing.T, result interface{}) {
	require.NotNil(t, result, "Symbols result should not be nil")

	symbols, ok := result.([]interface{})
	require.True(t, ok, "Symbols result should be an array")

	for _, symbol := range symbols {
		symbolObj, ok := symbol.(map[string]interface{})
		require.True(t, ok, "Symbol should be an object")
		require.Contains(t, symbolObj, "name", "Symbol should have name")
		require.Contains(t, symbolObj, "kind", "Symbol should have kind")
	}
}

// ValidateReferencesResult validates a textDocument/references response result
func ValidateReferencesResult(t *testing.T, result interface{}) {
	if result == nil {
		return // Null references is valid
	}

	references, ok := result.([]interface{})
	require.True(t, ok, "References result should be an array or null")

	for _, ref := range references {
		refObj, ok := ref.(map[string]interface{})
		require.True(t, ok, "Reference should be an object")
		require.Contains(t, refObj, "uri", "Reference should have uri")
		require.Contains(t, refObj, "range", "Reference should have range")
	}
}

// ValidateCompletionResult validates a textDocument/completion response result
func ValidateCompletionResult(t *testing.T, result interface{}) {
	if result == nil {
		return // Null completion is valid
	}

	// Can be CompletionList or CompletionItem[]
	switch v := result.(type) {
	case map[string]interface{}:
		// CompletionList
		require.Contains(t, v, "isIncomplete", "CompletionList should have isIncomplete")
		require.Contains(t, v, "items", "CompletionList should have items")
	case []interface{}:
		// CompletionItem[]
		for _, item := range v {
			itemObj, ok := item.(map[string]interface{})
			require.True(t, ok, "Completion item should be an object")
			require.Contains(t, itemObj, "label", "Completion item should have label")
		}
	default:
		require.Fail(t, fmt.Sprintf("Unexpected completion result type: %T", result))
	}
}

// Enhanced Request Builders

// CreateEnhancedCompletionRequest creates a completion request with trigger context
func CreateEnhancedCompletionRequest(uri string, line, character int, triggerKind int, triggerCharacter string) *JSONRPCRequest {
	return CreateEnhancedCompletionRequestWithID(uri, line, character, triggerKind, triggerCharacter, nil)
}

// CreateEnhancedCompletionRequestWithID creates a completion request with trigger context and custom ID
func CreateEnhancedCompletionRequestWithID(uri string, line, character int, triggerKind int, triggerCharacter string, id interface{}) *JSONRPCRequest {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": character,
		},
		"context": map[string]interface{}{
			"triggerKind":      triggerKind,
			"triggerCharacter": triggerCharacter,
		},
	}
	return CreateJSONRPCRequest("textDocument/completion", params, id)
}

// CreateInvalidMethodRequest creates a request with invalid method for error testing
func CreateInvalidMethodRequest() *JSONRPCRequest {
	return CreateJSONRPCRequest("invalid/method", map[string]interface{}{}, nil)
}

// CreateMalformedRequest creates a malformed request for error testing
func CreateMalformedRequest() map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "textDocument/definition",
		// Missing required params
		"id": generateID(),
	}
}
