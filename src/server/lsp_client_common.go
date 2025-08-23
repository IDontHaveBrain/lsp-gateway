package server

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server/protocol"
)

// deliverResponseCommon reduces duplication across StdioClient and SocketClient response handling.
// It delivers a JSON-RPC response or error to the pending request if present, and logs late/unknown responses.
func deliverResponseCommon(
	id interface{},
	result json.RawMessage,
	rpcErr *protocol.RPCError,
	requests map[string]*pendingRequest,
	reqsMu *sync.RWMutex,
	timeoutsMu *sync.RWMutex,
	recentTimeouts map[string]time.Time,
	stopCh <-chan struct{},
) {
	idStr := fmt.Sprintf("%v", id)

	// Protect concurrent access to requests map
	reqsMu.RLock()
	req, exists := requests[idStr]
	reqsMu.RUnlock()
	if exists {
		var responseData json.RawMessage
		if rpcErr != nil {
			errorData, _ := json.Marshal(rpcErr)
			responseData = errorData
			if !protocol.IsExpectedSuppressibleError(rpcErr) {
				sanitizedError := common.SanitizeErrorForLogging(rpcErr)
				common.LSPLogger.Warn("LSP response contains error: id=%s, error=%s", idStr, sanitizedError)
			}
		} else {
			responseData = result
		}
		select {
		case req.respCh <- responseData:
			// delivered
		case <-req.done:
			common.LSPLogger.Warn("Request already completed when trying to deliver response: id=%s", idStr)
		case <-stopCh:
			common.LSPLogger.Warn("Client stopped when trying to deliver response: id=%s", idStr)
		}
		return
	}

	// Handle late responses after timeout
	timeoutsMu.RLock()
	timeoutTime, wasTimeout := recentTimeouts[idStr]
	timeoutsMu.RUnlock()
	if wasTimeout && time.Since(timeoutTime) < 30*time.Second {
		common.LSPLogger.Debug("Received late response for previously timed-out request: id=%s (timed out %v ago)", idStr, time.Since(timeoutTime))
	} else {
		common.LSPLogger.Warn("No matching request found for response: id=%s", idStr)
	}
}
