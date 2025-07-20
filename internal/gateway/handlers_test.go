package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/transport"
)

func createTestGatewayForHandlers(t *testing.T) (*Gateway, map[string]*MockLSPClient) {
	cfg := &config.GatewayConfig{
		Port: 8080,
		Servers: []config.ServerConfig{
			{
				Name:      "gopls",
				Languages: []string{"go"},
				Command:   "gopls",
				Args:      []string{},
				Transport: "stdio",
			},
			{
				Name:      "pyright",
				Languages: []string{"python"},
				Command:   "pyright",
				Args:      []string{},
				Transport: "stdio",
			},
			{
				Name:      "typescript-lsp",
				Languages: []string{"typescript", "javascript"},
				Command:   "typescript-language-server",
				Args:      []string{"--stdio"},
				Transport: "stdio",
			},
			{
				Name:      "jdtls",
				Languages: []string{"java"},
				Command:   "jdtls",
				Args:      []string{},
				Transport: "stdio",
			},
		},
	}

	gateway := &Gateway{
		config:  cfg,
		clients: make(map[string]transport.LSPClient),
		router:  NewRouter(),
	}

	mockClients := make(map[string]*MockLSPClient)

	mockClients["gopls"] = NewMockLSPClient()
	mockClients["pyright"] = NewMockLSPClient()
	mockClients["typescript-lsp"] = NewMockLSPClient()
	mockClients["jdtls"] = NewMockLSPClient()

	gateway.clients["gopls"] = mockClients["gopls"]
	gateway.clients["pyright"] = mockClients["pyright"]
	gateway.clients["typescript-lsp"] = mockClients["typescript-lsp"]
	gateway.clients["jdtls"] = mockClients["jdtls"]

	gateway.router.RegisterServer("gopls", []string{"go"})
	gateway.router.RegisterServer("pyright", []string{"python"})
	gateway.router.RegisterServer("typescript-lsp", []string{"typescript", "javascript"})
	gateway.router.RegisterServer("jdtls", []string{"java"})

	for _, mockClient := range mockClients {
		if err := mockClient.Start(context.TODO()); err != nil {
			t.Logf("error starting mock client: %v", err)
		}
	}

	return gateway, mockClients
}

func TestHandleJSONRPC_HTTPMethodValidation(t *testing.T) {
	t.Parallel()
	gateway, _ := createTestGatewayForHandlers(t)

	tests := []struct {
		method         string
		expectedStatus int
		expectedError  string
	}{
		{
			method:         "GET",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedError:  "Method not allowed",
		},
		{
			method:         "PUT",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedError:  "Method not allowed",
		},
		{
			method:         "DELETE",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedError:  "Method not allowed",
		},
		{
			method:         "PATCH",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedError:  "Method not allowed",
		},
		{
			method:         "OPTIONS",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedError:  "Method not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/jsonrpc", nil)
			w := httptest.NewRecorder()

			gateway.HandleJSONRPC(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			body := w.Body.String()
			if !bytes.Contains([]byte(body), []byte(tt.expectedError)) {
				t.Errorf("Expected error message '%s' in response body, got: %s", tt.expectedError, body)
			}
		})
	}
}

func TestHandleJSONRPC_JSONParsingErrors(t *testing.T) {
	t.Parallel()
	gateway, _ := createTestGatewayForHandlers(t)

	tests := []struct {
		name          string
		body          string
		expectedCode  int
		expectedError string
	}{
		{
			name:          "invalid JSON",
			body:          `{"invalid": json}`,
			expectedCode:  ParseError,
			expectedError: "Parse error",
		},
		{
			name:          "empty body",
			body:          ``,
			expectedCode:  ParseError,
			expectedError: "Parse error",
		},
		{
			name:          "malformed JSON",
			body:          `{"jsonrpc": JSONRPCVersion, "method": "test"`,
			expectedCode:  ParseError,
			expectedError: "Parse error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			gateway.HandleJSONRPC(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
			}

			var response JSONRPCResponse
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if response.Error == nil {
				t.Fatal("Expected error in response")
			}

			if response.Error.Code != tt.expectedCode {
				t.Errorf("Expected error code %d, got %d", tt.expectedCode, response.Error.Code)
			}

			if response.Error.Message != tt.expectedError {
				t.Errorf("Expected error message '%s', got '%s'", tt.expectedError, response.Error.Message)
			}
		})
	}
}

func TestHandleJSONRPC_JSONRPCValidation(t *testing.T) {
	t.Parallel()
	gateway, _ := createTestGatewayForHandlers(t)

	tests := []struct {
		name          string
		request       JSONRPCRequest
		expectedCode  int
		expectedError string
	}{
		{
			name: "invalid JSON-RPC version",
			request: JSONRPCRequest{
				JSONRPC: "1.0",
				ID:      1,
				Method:  "textDocument/definition",
			},
			expectedCode:  InvalidRequest,
			expectedError: "Invalid request",
		},
		{
			name: "missing JSON-RPC version",
			request: JSONRPCRequest{
				ID:     1,
				Method: "textDocument/definition",
			},
			expectedCode:  InvalidRequest,
			expectedError: "Invalid request",
		},
		{
			name: "empty method",
			request: JSONRPCRequest{
				JSONRPC: JSONRPCVersion,
				ID:      1,
				Method:  "",
			},
			expectedCode:  InvalidRequest,
			expectedError: "Invalid request",
		},
		{
			name: "missing method",
			request: JSONRPCRequest{
				JSONRPC: JSONRPCVersion,
				ID:      1,
			},
			expectedCode:  InvalidRequest,
			expectedError: "Invalid request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			gateway.HandleJSONRPC(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
			}

			var response JSONRPCResponse
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if response.Error == nil {
				t.Fatal("Expected error in response")
			}

			if response.Error.Code != tt.expectedCode {
				t.Errorf("Expected error code %d, got %d", tt.expectedCode, response.Error.Code)
			}

			if response.Error.Message != tt.expectedError {
				t.Errorf("Expected error message '%s', got '%s'", tt.expectedError, response.Error.Message)
			}

			if fmt.Sprintf("%v", response.ID) != fmt.Sprintf("%v", tt.request.ID) {
				t.Errorf("Expected response ID %v, got %v", tt.request.ID, response.ID)
			}
		})
	}
}

func TestHandleJSONRPC_RequestRouting(t *testing.T) {
	t.Parallel()
	gateway, _ := createTestGatewayForHandlers(t)

	tests := []struct {
		name          string
		method        string
		params        interface{}
		expectedCode  int
		expectedError string
	}{
		{
			name:   "unsupported file extension",
			method: "textDocument/definition",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file:///test.xyz",
				},
				"position": map[string]interface{}{
					"line":      0,
					"character": 5,
				},
			},
			expectedCode:  MethodNotFound,
			expectedError: "Method not found",
		},
		{
			name:   "missing textDocument parameter",
			method: "textDocument/definition",
			params: map[string]interface{}{
				"position": map[string]interface{}{
					"line":      0,
					"character": 5,
				},
			},
			expectedCode:  MethodNotFound,
			expectedError: "Method not found",
		},
		{
			name:   "missing URI in textDocument",
			method: "textDocument/definition",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{},
				"position": map[string]interface{}{
					"line":      0,
					"character": 5,
				},
			},
			expectedCode:  MethodNotFound,
			expectedError: "Method not found",
		},
		{
			name:   "no file extension",
			method: "textDocument/definition",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file:///test",
				},
				"position": map[string]interface{}{
					"line":      0,
					"character": 5,
				},
			},
			expectedCode:  MethodNotFound,
			expectedError: "Method not found",
		},
		{
			name:   "hover with unsupported file extension",
			method: LSPMethodHover,
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file:///test.xyz",
				},
				"position": map[string]interface{}{
					"line":      0,
					"character": 5,
				},
			},
			expectedCode:  MethodNotFound,
			expectedError: "Method not found",
		},
		{
			name:   "hover with missing textDocument parameter",
			method: LSPMethodHover,
			params: map[string]interface{}{
				"position": map[string]interface{}{
					"line":      0,
					"character": 5,
				},
			},
			expectedCode:  MethodNotFound,
			expectedError: "Method not found",
		},
		{
			name:   "hover with missing URI in textDocument",
			method: LSPMethodHover,
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{},
				"position": map[string]interface{}{
					"line":      0,
					"character": 5,
				},
			},
			expectedCode:  MethodNotFound,
			expectedError: "Method not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := JSONRPCRequest{
				JSONRPC: JSONRPCVersion,
				ID:      1,
				Method:  tt.method,
				Params:  tt.params,
			}

			body, _ := json.Marshal(request)
			req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			gateway.HandleJSONRPC(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
			}

			var response JSONRPCResponse
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if response.Error == nil {
				t.Fatal("Expected error in response")
			}

			if response.Error.Code != tt.expectedCode {
				t.Errorf("Expected error code %d, got %d", tt.expectedCode, response.Error.Code)
			}

			if response.Error.Message != tt.expectedError {
				t.Errorf("Expected error message '%s', got '%s'", tt.expectedError, response.Error.Message)
			}
		})
	}
}

func TestHandleJSONRPC_LSPClientHandling(t *testing.T) {
	t.Parallel()
	gateway, mockClients := createTestGatewayForHandlers(t)

	t.Run("inactive client", func(t *testing.T) {
		mockClient := mockClients["gopls"]
		if err := mockClient.Stop(); err != nil {
			t.Logf("error stopping mock client: %v", err)
		}

		request := JSONRPCRequest{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "textDocument/definition",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file:///test.go",
				},
				"position": map[string]interface{}{
					"line":      0,
					"character": 5,
				},
			},
		}

		body, _ := json.Marshal(request)
		req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		gateway.HandleJSONRPC(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response JSONRPCResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Error == nil {
			t.Fatal("Expected error in response")
		}

		if response.Error.Code != InternalError {
			t.Errorf("Expected error code %d, got %d", InternalError, response.Error.Code)
		}

		if err := mockClient.Start(context.TODO()); err != nil {
			t.Logf("error restarting mock client: %v", err)
		}
	})

	t.Run("missing server", func(t *testing.T) {
		originalClient := gateway.clients["gopls"]
		delete(gateway.clients, "gopls")

		request := JSONRPCRequest{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "textDocument/definition",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file:///test.go",
				},
				"position": map[string]interface{}{
					"line":      0,
					"character": 5,
				},
			},
		}

		body, _ := json.Marshal(request)
		req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		gateway.HandleJSONRPC(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response JSONRPCResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Error == nil {
			t.Fatal("Expected error in response")
		}

		if response.Error.Code != InternalError {
			t.Errorf("Expected error code %d, got %d", InternalError, response.Error.Code)
		}

		gateway.clients["gopls"] = originalClient
	})
}

func TestHandleJSONRPC_NotificationHandling(t *testing.T) {
	t.Parallel()
	gateway, mockClients := createTestGatewayForHandlers(t)

	tests := []struct {
		name               string
		notificationError  error
		expectedStatusCode int
	}{
		{
			name:               "successful notification",
			notificationError:  nil,
			expectedStatusCode: http.StatusOK,
		},
		{
			name:               "notification error",
			notificationError:  fmt.Errorf("notification failed"),
			expectedStatusCode: http.StatusOK, // Should still be 200 with JSON-RPC error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mockClients["gopls"]
			mockClient.SetNotificationError(tt.notificationError)

			request := JSONRPCRequest{
				JSONRPC: JSONRPCVersion,
				Method:  "textDocument/didOpen",
				Params: map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri":        "file:///test.go",
						"languageId": "go",
						"version":    1,
						"text":       "package main",
					},
				},
			}

			body, _ := json.Marshal(request)
			req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			gateway.HandleJSONRPC(w, req)

			if w.Code != tt.expectedStatusCode {
				t.Errorf("Expected status %d, got %d", tt.expectedStatusCode, w.Code)
			}

			if tt.notificationError == nil {
				if w.Body.Len() != 0 {
					t.Errorf("Expected empty response body for successful notification, got: %s", w.Body.String())
				}
			} else {
				var response JSONRPCResponse
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if response.Error == nil {
					t.Fatal("Expected error in response for failed notification")
				}

				if response.Error.Code != InternalError {
					t.Errorf("Expected error code %d, got %d", InternalError, response.Error.Code)
				}
			}

			mockClient.SetNotificationError(nil)
		})
	}
}

func getMockClientName(method string, params interface{}) string {
	if method == "textDocument/definition" || method == LSPMethodHover {
		paramsMap := params.(map[string]interface{})
		textDoc := paramsMap["textDocument"].(map[string]interface{})
		uri := textDoc["uri"].(string)
		if bytes.Contains([]byte(uri), []byte(".go")) {
			return "gopls"
		} else if bytes.Contains([]byte(uri), []byte(".py")) {
			return "pyright"
		} else if bytes.Contains([]byte(uri), []byte(".ts")) || bytes.Contains([]byte(uri), []byte(".js")) {
			return "typescript-lsp"
		} else if bytes.Contains([]byte(uri), []byte(".java")) {
			return "jdtls"
		}
	}
	return ""
}

func executeJSONRPCTest(t *testing.T, gateway *Gateway, mockClients map[string]*MockLSPClient, method string, params interface{}, mockResponse json.RawMessage, expectedResult interface{}) {
	mockClientName := getMockClientName(method, params)
	mockClient := mockClients[mockClientName]
	mockClient.SetResponse(method, mockResponse)

	request := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  method,
		Params:  params,
	}

	body, _ := json.Marshal(request)
	req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	gateway.HandleJSONRPC(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Error != nil {
		t.Fatalf("Unexpected error in response: %v", response.Error)
	}

	if response.JSONRPC != JSONRPCVersion {
		t.Errorf("Expected JSON-RPC version 2.0, got %s", response.JSONRPC)
	}

	if fmt.Sprintf("%v", response.ID) != fmt.Sprintf("%v", request.ID) {
		t.Errorf("Expected response ID %v, got %v", request.ID, response.ID)
	}

	expectedJSON, _ := json.Marshal(expectedResult)
	resultJSON, _ := json.Marshal(response.Result)

	if string(expectedJSON) != string(resultJSON) {
		t.Errorf("Expected result %s, got %s", string(expectedJSON), string(resultJSON))
	}

	mockClient.SetResponse(method, nil)
}

func TestHandleJSONRPC_DefinitionRequests(t *testing.T) {
	t.Parallel()
	gateway, mockClients := createTestGatewayForHandlers(t)

	t.Run("textDocument/definition for Go file", func(t *testing.T) {
		params := map[string]interface{}{
			"textDocument": map[string]interface{}{"uri": "file:///test.go"},
			"position":     map[string]interface{}{"line": 0, "character": 5},
		}
		mockResponse := json.RawMessage(`{"uri": "file:///test.go", "range": {"start": {"line": 0, "character": 0}, "end": {"line": 0, "character": 5}}}`)
		expectedResult := map[string]interface{}{
			"uri": "file:///test.go",
			"range": map[string]interface{}{
				"start": map[string]interface{}{"line": float64(0), "character": float64(0)},
				"end":   map[string]interface{}{"line": float64(0), "character": float64(5)},
			},
		}
		executeJSONRPCTest(t, gateway, mockClients, "textDocument/definition", params, mockResponse, expectedResult)
	})

	t.Run("textDocument/definition for Python file", func(t *testing.T) {
		params := map[string]interface{}{
			"textDocument": map[string]interface{}{"uri": "file:///test.py"},
			"position":     map[string]interface{}{"line": 5, "character": 10},
		}
		mockResponse := json.RawMessage(`[{"uri": "file:///module.py", "range": {"start": {"line": 10, "character": 0}, "end": {"line": 10, "character": 15}}}]`)
		expectedResult := []interface{}{
			map[string]interface{}{
				"uri": "file:///module.py",
				"range": map[string]interface{}{
					"start": map[string]interface{}{"line": float64(10), "character": float64(0)},
					"end":   map[string]interface{}{"line": float64(10), "character": float64(15)},
				},
			},
		}
		executeJSONRPCTest(t, gateway, mockClients, "textDocument/definition", params, mockResponse, expectedResult)
	})
}

func TestHandleJSONRPC_HoverRequests(t *testing.T) {
	t.Parallel()
	gateway, mockClients := createTestGatewayForHandlers(t)

	t.Run("textDocument/hover for Go file", func(t *testing.T) {
		params := map[string]interface{}{
			"textDocument": map[string]interface{}{"uri": "file:///test.go"},
			"position":     map[string]interface{}{"line": 3, "character": 8},
		}
		mockResponse := json.RawMessage(`{"contents": {"kind": "markdown", "value": "func main()\n\nMain function of the program"}, "range": {"start": {"line": 3, "character": 5}, "end": {"line": 3, "character": 9}}}`)
		expectedResult := map[string]interface{}{
			"contents": map[string]interface{}{
				"kind":  "markdown",
				"value": "func main()\n\nMain function of the program",
			},
			"range": map[string]interface{}{
				"start": map[string]interface{}{"line": float64(3), "character": float64(5)},
				"end":   map[string]interface{}{"line": float64(3), "character": float64(9)},
			},
		}
		executeJSONRPCTest(t, gateway, mockClients, LSPMethodHover, params, mockResponse, expectedResult)
	})

	t.Run("textDocument/hover for Python file", func(t *testing.T) {
		params := map[string]interface{}{
			"textDocument": map[string]interface{}{"uri": "file:///test.py"},
			"position":     map[string]interface{}{"line": 7, "character": 4},
		}
		mockResponse := json.RawMessage(`{"contents": {"kind": "plaintext", "value": "def hello_world() -> None\n\nA simple function that prints hello world"}, "range": {"start": {"line": 7, "character": 0}, "end": {"line": 7, "character": 11}}}`)
		expectedResult := map[string]interface{}{
			"contents": map[string]interface{}{
				"kind":  "plaintext",
				"value": "def hello_world() -> None\n\nA simple function that prints hello world",
			},
			"range": map[string]interface{}{
				"start": map[string]interface{}{"line": float64(7), "character": float64(0)},
				"end":   map[string]interface{}{"line": float64(7), "character": float64(11)},
			},
		}
		executeJSONRPCTest(t, gateway, mockClients, LSPMethodHover, params, mockResponse, expectedResult)
	})

	t.Run("textDocument/hover for TypeScript file", func(t *testing.T) {
		params := map[string]interface{}{
			"textDocument": map[string]interface{}{"uri": "file:///test.ts"},
			"position":     map[string]interface{}{"line": 2, "character": 12},
		}
		mockResponse := json.RawMessage(`{"contents": ["function greet(name: string): void", "Greets a person with the given name"], "range": {"start": {"line": 2, "character": 9}, "end": {"line": 2, "character": 14}}}`)
		expectedResult := map[string]interface{}{
			"contents": []interface{}{
				"function greet(name: string): void",
				"Greets a person with the given name",
			},
			"range": map[string]interface{}{
				"start": map[string]interface{}{"line": float64(2), "character": float64(9)},
				"end":   map[string]interface{}{"line": float64(2), "character": float64(14)},
			},
		}
		executeJSONRPCTest(t, gateway, mockClients, LSPMethodHover, params, mockResponse, expectedResult)
	})

	t.Run("textDocument/hover for Java file", func(t *testing.T) {
		params := map[string]interface{}{
			"textDocument": map[string]interface{}{"uri": "file:///test.java"},
			"position":     map[string]interface{}{"line": 5, "character": 16},
		}
		mockResponse := json.RawMessage(`{"contents": {"kind": "markdown", "value": "public static void main(String[] args)\n\nThe main method of the Java application"}, "range": {"start": {"line": 5, "character": 12}, "end": {"line": 5, "character": 16}}}`)
		expectedResult := map[string]interface{}{
			"contents": map[string]interface{}{
				"kind":  "markdown",
				"value": "public static void main(String[] args)\n\nThe main method of the Java application",
			},
			"range": map[string]interface{}{
				"start": map[string]interface{}{"line": float64(5), "character": float64(12)},
				"end":   map[string]interface{}{"line": float64(5), "character": float64(16)},
			},
		}
		executeJSONRPCTest(t, gateway, mockClients, LSPMethodHover, params, mockResponse, expectedResult)
	})
}

func TestHandleJSONRPC_RequestErrors(t *testing.T) {
	t.Parallel()
	gateway, mockClients := createTestGatewayForHandlers(t)

	t.Run("LSP client request error", func(t *testing.T) {
		mockClient := mockClients["gopls"]
		mockClient.SetRequestError(fmt.Errorf("LSP server error"))

		request := JSONRPCRequest{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "textDocument/definition",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file:///test.go",
				},
				"position": map[string]interface{}{
					"line":      0,
					"character": 5,
				},
			},
		}

		body, _ := json.Marshal(request)
		req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		gateway.HandleJSONRPC(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response JSONRPCResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Error == nil {
			t.Fatal("Expected error in response")
		}

		if response.Error.Code != InternalError {
			t.Errorf("Expected error code %d, got %d", InternalError, response.Error.Code)
		}

		mockClient.SetRequestError(nil)
	})
}

func TestHandleJSONRPC_SpecialMethods(t *testing.T) {
	t.Parallel()
	gateway, _ := createTestGatewayForHandlers(t)

	specialMethods := []string{
		"initialize",
		"initialized",
		"shutdown",
		"exit",
		"workspace/symbol",
		"workspace/executeCommand",
	}

	for _, method := range specialMethods {
		t.Run(method, func(t *testing.T) {
			testSpecialMethodSuccess(t, gateway, method)
		})
	}
}

func testSpecialMethodSuccess(t *testing.T, gateway *Gateway, method string) {
	request := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  method,
		Params:  map[string]interface{}{},
	}

	body, _ := json.Marshal(request)
	req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	gateway.HandleJSONRPC(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Error != nil {
		t.Errorf("Unexpected error for method %s: %v", method, response.Error)
	}
}

func TestHandleJSONRPC_ContentTypeHeader(t *testing.T) {
	t.Parallel()
	gateway, _ := createTestGatewayForHandlers(t)

	request := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "textDocument/definition",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///test.go",
			},
			"position": map[string]interface{}{
				"line":      0,
				"character": 5,
			},
		},
	}

	body, _ := json.Marshal(request)
	req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	gateway.HandleJSONRPC(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}
}

func TestHandleJSONRPC_URIExtraction(t *testing.T) {
	t.Parallel()
	gateway, _ := createTestGatewayForHandlers(t)

	tests := []struct {
		name        string
		method      string
		params      interface{}
		shouldError bool
	}{
		{
			name:   "textDocument with uri parameter",
			method: "textDocument/definition",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file:///test.go",
				},
				"position": map[string]interface{}{
					"line":      0,
					"character": 5,
				},
			},
			shouldError: false,
		},
		{
			name:   "direct uri parameter",
			method: "textDocument/publishDiagnostics",
			params: map[string]interface{}{
				"uri":         "file:///test.go",
				"diagnostics": []interface{}{},
			},
			shouldError: false,
		},
		{
			name:        "missing parameters",
			method:      "textDocument/definition",
			params:      nil,
			shouldError: true,
		},
		{
			name:        "invalid parameter structure",
			method:      "textDocument/definition",
			params:      "invalid",
			shouldError: true,
		},
		{
			name:   "missing textDocument",
			method: "textDocument/definition",
			params: map[string]interface{}{
				"position": map[string]interface{}{
					"line":      0,
					"character": 5,
				},
			},
			shouldError: true,
		},
		{
			name:   "textDocument without uri",
			method: "textDocument/definition",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"version": 1,
				},
				"position": map[string]interface{}{
					"line":      0,
					"character": 5,
				},
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := JSONRPCRequest{
				JSONRPC: JSONRPCVersion,
				ID:      1,
				Method:  tt.method,
				Params:  tt.params,
			}

			body, _ := json.Marshal(request)
			req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			gateway.HandleJSONRPC(w, req)

			var response JSONRPCResponse
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if tt.shouldError {
				if response.Error == nil {
					t.Fatal("Expected error in response")
				}
			} else {
				if response.Error != nil {
					t.Fatalf("Unexpected error in response: %v", response.Error)
				}
			}
		})
	}
}
