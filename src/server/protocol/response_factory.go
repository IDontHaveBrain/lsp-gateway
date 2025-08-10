package protocol

import "encoding/json"

// ResponseFactory provides a centralized way to create JSON-RPC responses
// eliminating duplication and ensuring consistent response formatting
type ResponseFactory struct{}

// NewResponseFactory creates a new ResponseFactory instance
func NewResponseFactory() *ResponseFactory {
	return &ResponseFactory{}
}

// CreateSuccess creates a successful JSON-RPC response with result data
func (rf *ResponseFactory) CreateSuccess(id interface{}, result interface{}) JSONRPCResponse {
	// Ensure result field is always present (use explicit null when empty)
	if result == nil {
		result = json.RawMessage("null")
	} else if raw, ok := result.(json.RawMessage); ok {
		if len(raw) == 0 {
			result = json.RawMessage("null")
		}
	}
	return JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  result,
	}
}

// CreateError creates a JSON-RPC error response with custom error code and message
func (rf *ResponseFactory) CreateError(id interface{}, code int, message string) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error:   NewRPCError(code, message, nil),
	}
}

// CreateInternalError creates a JSON-RPC response for internal errors (-32603)
func (rf *ResponseFactory) CreateInternalError(id interface{}, err error) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error:   NewInternalError(err.Error()),
	}
}

// CreateParseError creates a JSON-RPC response for parse errors (-32700)
func (rf *ResponseFactory) CreateParseError(id interface{}) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error:   NewParseError(nil),
	}
}

// CreateInvalidRequest creates a JSON-RPC response for invalid request errors (-32600)
func (rf *ResponseFactory) CreateInvalidRequest(id interface{}, message string) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error:   NewInvalidRequestError(message),
	}
}

// CreateMethodNotFound creates a JSON-RPC response for method not found errors (-32601)
func (rf *ResponseFactory) CreateMethodNotFound(id interface{}, message string) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error:   NewMethodNotFoundError(message),
	}
}

// CreateInvalidParams creates a JSON-RPC response for invalid params errors (-32602)
func (rf *ResponseFactory) CreateInvalidParams(id interface{}, message string) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error:   NewRPCError(InvalidParams, "Invalid params", message),
	}
}

// CreateCustomError creates a JSON-RPC response with a pre-built RPCError
func (rf *ResponseFactory) CreateCustomError(id interface{}, rpcErr *RPCError) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error:   rpcErr,
	}
}
