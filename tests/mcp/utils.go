package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// MessageBuilder helps construct MCP messages for testing
type MessageBuilder struct {
	message *MCPMessage
}

// NewMessageBuilder creates a new message builder
func NewMessageBuilder() *MessageBuilder {
	return &MessageBuilder{
		message: &MCPMessage{
			JSONRPC: JSONRPCVersion,
		},
	}
}

// WithID sets the message ID
func (mb *MessageBuilder) WithID(id interface{}) *MessageBuilder {
	mb.message.ID = id
	return mb
}

// WithMethod sets the method name
func (mb *MessageBuilder) WithMethod(method string) *MessageBuilder {
	mb.message.Method = method
	return mb
}

// WithParams sets the parameters
func (mb *MessageBuilder) WithParams(params interface{}) *MessageBuilder {
	mb.message.Params = params
	return mb
}

// WithResult sets the result
func (mb *MessageBuilder) WithResult(result interface{}) *MessageBuilder {
	mb.message.Result = result
	return mb
}

// WithError sets the error
func (mb *MessageBuilder) WithError(code int, message string, data interface{}) *MessageBuilder {
	mb.message.Error = &MCPError{
		Code:    code,
		Message: message,
		Data:    data,
	}
	return mb
}

// Build returns the constructed message
func (mb *MessageBuilder) Build() *MCPMessage {
	return mb.message
}

// BuildRequest builds and returns as MCPRequest
func (mb *MessageBuilder) BuildRequest() (*MCPRequest, error) {
	return mb.message.ToRequest()
}

// BuildResponse builds and returns as MCPResponse
func (mb *MessageBuilder) BuildResponse() (*MCPResponse, error) {
	return mb.message.ToResponse()
}

// CreateInitializeRequest creates an initialize request message
func CreateInitializeRequest(clientInfo map[string]interface{}, capabilities map[string]interface{}) *MCPRequest {
	if clientInfo == nil {
		clientInfo = map[string]interface{}{
			"name":    "test-mcp-client",
			"version": "1.0.0",
		}
	}
	
	if capabilities == nil {
		capabilities = map[string]interface{}{
			"tools": true,
		}
	}
	
	params := InitializeParams{
		ProtocolVersion: ProtocolVersion,
		ClientInfo:      clientInfo,
		Capabilities:    capabilities,
	}
	
	return &MCPRequest{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "initialize",
		Params:  params,
	}
}

// CreateToolCallRequest creates a tool call request
func CreateToolCallRequest(id interface{}, toolName string, params map[string]interface{}) *MCPRequest {
	toolCall := ToolCall{
		Tool:   toolName,
		Params: params,
	}
	
	return &MCPRequest{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Method:  "tools/call",
		Params:  toolCall,
	}
}

// ValidateMessage performs comprehensive validation on an MCP message
func ValidateMessage(msg *MCPMessage) []string {
	var errors []string
	
	// Check JSON-RPC version
	if msg.JSONRPC != JSONRPCVersion {
		errors = append(errors, fmt.Sprintf("invalid JSONRPC version: expected %s, got %s", JSONRPCVersion, msg.JSONRPC))
	}
	
	// Check message type consistency
	if msg.IsRequest() {
		if msg.Result != nil {
			errors = append(errors, "request should not have result field")
		}
		if msg.Error != nil {
			errors = append(errors, "request should not have error field")
		}
		if msg.Method == "" {
			errors = append(errors, "request must have method field")
		}
	} else if msg.IsResponse() {
		if msg.Params != nil {
			errors = append(errors, "response should not have params field")
		}
		if msg.Method != "" {
			errors = append(errors, "response should not have method field")
		}
		if msg.Result == nil && msg.Error == nil {
			errors = append(errors, "response must have either result or error field")
		}
		if msg.Result != nil && msg.Error != nil {
			errors = append(errors, "response cannot have both result and error fields")
		}
	} else if msg.IsNotification() {
		if msg.Result != nil {
			errors = append(errors, "notification should not have result field")
		}
		if msg.Error != nil {
			errors = append(errors, "notification should not have error field")
		}
		if msg.Method == "" {
			errors = append(errors, "notification must have method field")
		}
	} else {
		errors = append(errors, "message must be either request, response, or notification")
	}
	
	// Validate method names
	if msg.Method != "" && !isValidMethodName(msg.Method) {
		errors = append(errors, fmt.Sprintf("invalid method name: %s", msg.Method))
	}
	
	return errors
}

// isValidMethodName checks if a method name is valid
func isValidMethodName(method string) bool {
	// Method names should contain only alphanumeric, /, _, and -
	for _, char := range method {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '/' || char == '_' || char == '-') {
			return false
		}
	}
	
	// Should not start or end with /
	if strings.HasPrefix(method, "/") || strings.HasSuffix(method, "/") {
		return false
	}
	
	// Should not contain consecutive /
	if strings.Contains(method, "//") {
		return false
	}
	
	return true
}

// FormatMessage formats an MCP message for logging
func FormatMessage(msg *MCPMessage) string {
	if msg == nil {
		return "<nil>"
	}
	
	var msgType string
	if msg.IsRequest() {
		msgType = "REQUEST"
	} else if msg.IsResponse() {
		msgType = "RESPONSE"
	} else if msg.IsNotification() {
		msgType = "NOTIFICATION"
	} else {
		msgType = "UNKNOWN"
	}
	
	parts := []string{
		fmt.Sprintf("Type: %s", msgType),
		fmt.Sprintf("ID: %v", msg.ID),
	}
	
	if msg.Method != "" {
		parts = append(parts, fmt.Sprintf("Method: %s", msg.Method))
	}
	
	if msg.Params != nil {
		paramsJSON, _ := json.Marshal(msg.Params)
		parts = append(parts, fmt.Sprintf("Params: %s", string(paramsJSON)))
	}
	
	if msg.Result != nil {
		resultJSON, _ := json.Marshal(msg.Result)
		parts = append(parts, fmt.Sprintf("Result: %s", string(resultJSON)))
	}
	
	if msg.Error != nil {
		parts = append(parts, fmt.Sprintf("Error: [%d] %s", msg.Error.Code, msg.Error.Message))
	}
	
	return strings.Join(parts, ", ")
}

// ResponseMatcher helps match responses to requests
type ResponseMatcher struct {
	pendingRequests map[string]time.Time
	mu              sync.RWMutex
}

// NewResponseMatcher creates a new response matcher
func NewResponseMatcher() *ResponseMatcher {
	return &ResponseMatcher{
		pendingRequests: make(map[string]time.Time),
	}
}

// RegisterRequest registers a sent request
func (rm *ResponseMatcher) RegisterRequest(id interface{}) {
	idStr := fmt.Sprintf("%v", id)
	
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	rm.pendingRequests[idStr] = time.Now()
}

// MatchResponse matches a response to a request and returns the latency
func (rm *ResponseMatcher) MatchResponse(id interface{}) (time.Duration, bool) {
	idStr := fmt.Sprintf("%v", id)
	
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	if sentTime, exists := rm.pendingRequests[idStr]; exists {
		delete(rm.pendingRequests, idStr)
		return time.Since(sentTime), true
	}
	
	return 0, false
}

// GetPendingCount returns the number of pending requests
func (rm *ResponseMatcher) GetPendingCount() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	return len(rm.pendingRequests)
}

// CleanupOldRequests removes requests older than the specified duration
func (rm *ResponseMatcher) CleanupOldRequests(maxAge time.Duration) int {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	cutoff := time.Now().Add(-maxAge)
	removed := 0
	
	for id, sentTime := range rm.pendingRequests {
		if sentTime.Before(cutoff) {
			delete(rm.pendingRequests, id)
			removed++
		}
	}
	
	return removed
}