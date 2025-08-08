package shared

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"lsp-gateway/src/utils"
)

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

// LSPPosition represents a position in a document
type LSPPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// LSPTextDocument represents a text document identifier
type LSPTextDocument struct {
	URI string `json:"uri"`
}

// CreateJSONRPCRequest creates a basic JSON-RPC request
func CreateJSONRPCRequest(method string, params interface{}, id interface{}) *JSONRPCRequest {
	return &JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      id,
	}
}

// CreateDefinitionRequest creates a textDocument/definition request
func CreateDefinitionRequest(fileURI string, line, character int, id interface{}) *JSONRPCRequest {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fileURI,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": character,
		},
	}
	return CreateJSONRPCRequest("textDocument/definition", params, id)
}

// CreateDefinitionRequestFromFile creates a definition request using file path
func CreateDefinitionRequestFromFile(filePath string, line, character int, id interface{}) *JSONRPCRequest {
	uri := utils.FilePathToURI(filePath)
	return CreateDefinitionRequest(uri, line, character, id)
}

// CreateHoverRequest creates a textDocument/hover request
func CreateHoverRequest(fileURI string, line, character int, id interface{}) *JSONRPCRequest {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fileURI,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": character,
		},
	}
	return CreateJSONRPCRequest("textDocument/hover", params, id)
}

// CreateHoverRequestFromFile creates a hover request using file path
func CreateHoverRequestFromFile(filePath string, line, character int, id interface{}) *JSONRPCRequest {
	uri := utils.FilePathToURI(filePath)
	return CreateHoverRequest(uri, line, character, id)
}

// CreateReferencesRequest creates a textDocument/references request
func CreateReferencesRequest(fileURI string, line, character int, includeDeclaration bool, id interface{}) *JSONRPCRequest {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fileURI,
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

// CreateReferencesRequestFromFile creates a references request using file path
func CreateReferencesRequestFromFile(filePath string, line, character int, includeDeclaration bool, id interface{}) *JSONRPCRequest {
	uri := utils.FilePathToURI(filePath)
	return CreateReferencesRequest(uri, line, character, includeDeclaration, id)
}

// CreateDocumentSymbolRequest creates a textDocument/documentSymbol request
func CreateDocumentSymbolRequest(fileURI string, id interface{}) *JSONRPCRequest {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fileURI,
		},
	}
	return CreateJSONRPCRequest("textDocument/documentSymbol", params, id)
}

// CreateDocumentSymbolRequestFromFile creates a document symbol request using file path
func CreateDocumentSymbolRequestFromFile(filePath string, id interface{}) *JSONRPCRequest {
	uri := utils.FilePathToURI(filePath)
	return CreateDocumentSymbolRequest(uri, id)
}

// CreateWorkspaceSymbolRequest creates a workspace/symbol request
func CreateWorkspaceSymbolRequest(query string, id interface{}) *JSONRPCRequest {
	params := map[string]interface{}{
		"query": query,
	}
	return CreateJSONRPCRequest("workspace/symbol", params, id)
}

// CreateCompletionRequest creates a textDocument/completion request
func CreateCompletionRequest(fileURI string, line, character int, id interface{}) *JSONRPCRequest {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fileURI,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": character,
		},
	}
	return CreateJSONRPCRequest("textDocument/completion", params, id)
}

// CreateCompletionRequestFromFile creates a completion request using file path
func CreateCompletionRequestFromFile(filePath string, line, character int, id interface{}) *JSONRPCRequest {
	uri := utils.FilePathToURI(filePath)
	return CreateCompletionRequest(uri, line, character, id)
}

// CreateDidOpenRequest creates a textDocument/didOpen notification
func CreateDidOpenRequest(fileURI, languageID string, version int, text string) *JSONRPCRequest {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        fileURI,
			"languageId": languageID,
			"version":    version,
			"text":       text,
		},
	}
	return CreateJSONRPCRequest("textDocument/didOpen", params, nil)
}

// CreateDidOpenRequestFromFile creates a didOpen notification using file content
func CreateDidOpenRequestFromFile(filePath, languageID string, version int, content string) *JSONRPCRequest {
	uri := utils.FilePathToURI(filePath)
	return CreateDidOpenRequest(uri, languageID, version, content)
}

// CreateDidCloseRequest creates a textDocument/didClose notification
func CreateDidCloseRequest(fileURI string) *JSONRPCRequest {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fileURI,
		},
	}
	return CreateJSONRPCRequest("textDocument/didClose", params, nil)
}

// CreateDidCloseRequestFromFile creates a didClose notification using file path
func CreateDidCloseRequestFromFile(filePath string) *JSONRPCRequest {
	uri := utils.FilePathToURI(filePath)
	return CreateDidCloseRequest(uri)
}

// SendJSONRPCRequest sends a JSON-RPC request to the given URL and returns the response
func SendJSONRPCRequest(t *testing.T, client *http.Client, url string, request *JSONRPCRequest) map[string]interface{} {
	requestBody, err := json.Marshal(request)
	require.NoError(t, err, "Failed to marshal JSON-RPC request")

	resp, err := client.Post(url, "application/json", bytes.NewReader(requestBody))
	require.NoError(t, err, "Failed to send JSON-RPC request")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "Expected HTTP 200 OK")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read response body")

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	require.NoError(t, err, "Failed to unmarshal JSON-RPC response")

	return response
}

// SendJSONRPCRequestRaw sends a raw JSON-RPC request map
func SendJSONRPCRequestRaw(t *testing.T, client *http.Client, url string, request map[string]interface{}) map[string]interface{} {
	requestBody, err := json.Marshal(request)
	require.NoError(t, err, "Failed to marshal JSON-RPC request")

	resp, err := client.Post(url, "application/json", bytes.NewReader(requestBody))
	require.NoError(t, err, "Failed to send JSON-RPC request")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "Expected HTTP 200 OK")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read response body")

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	require.NoError(t, err, "Failed to unmarshal JSON-RPC response")

	return response
}

// CreateHTTPClient creates an HTTP client with reasonable timeout for tests
func CreateHTTPClient(timeout time.Duration) *http.Client {
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &http.Client{
		Timeout: timeout,
	}
}

// BuildJSONRPCURL constructs the JSON-RPC endpoint URL
func BuildJSONRPCURL(serverAddr string) string {
	return fmt.Sprintf("http://%s/jsonrpc", serverAddr)
}

// BuildHealthURL constructs the health endpoint URL
func BuildHealthURL(serverAddr string) string {
	return fmt.Sprintf("http://%s/health", serverAddr)
}

// CheckHealthEndpoint performs a health check against the server
func CheckHealthEndpoint(t *testing.T, client *http.Client, serverAddr string) {
	healthURL := BuildHealthURL(serverAddr)
	resp, err := client.Get(healthURL)
	require.NoError(t, err, "Health check failed")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "Health check should return 200")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read health response")

	var health map[string]interface{}
	err = json.Unmarshal(body, &health)
	require.NoError(t, err, "Failed to parse health response")
	require.Equal(t, "healthy", health["status"], "Server should be healthy")
}

// CreateBatchRequest creates a batch JSON-RPC request (array of requests)
func CreateBatchRequest(requests ...*JSONRPCRequest) []*JSONRPCRequest {
	return requests
}

// CreateInvalidMethodRequest creates a request with an invalid method for error testing
func CreateInvalidMethodRequest(id interface{}) *JSONRPCRequest {
	return CreateJSONRPCRequest("invalid/method", map[string]interface{}{}, id)
}
