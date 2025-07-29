//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func main() {
	// Test negative position
	request := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "test-validation",
		Method:  "textDocument/definition",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///tmp/test.go",
			},
			"position": map[string]interface{}{
				"line":      -1,
				"character": 0,
			},
		},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		fmt.Printf("Error marshaling request: %v\n", err)
		return
	}

	// Make request to gateway
	resp, err := http.Post("http://localhost:8080/jsonrpc", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		fmt.Printf("Error making request: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		return
	}

	var jsonResp JSONRPCResponse
	if err := json.Unmarshal(body, &jsonResp); err != nil {
		fmt.Printf("Error unmarshaling response: %v\n", err)
		fmt.Printf("Raw response: %s\n", string(body))
		return
	}

	fmt.Printf("Response: %+v\n", jsonResp)
	if jsonResp.Error != nil {
		fmt.Printf("Error Code: %d\n", jsonResp.Error.Code)
		fmt.Printf("Error Message: %s\n", jsonResp.Error.Message)
	}
	if jsonResp.Result != nil {
		fmt.Printf("Result: %s\n", string(jsonResp.Result))
	}
}