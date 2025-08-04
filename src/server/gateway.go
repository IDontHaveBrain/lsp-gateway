package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
)

// HTTPGateway provides a simple HTTP JSON-RPC gateway to LSP servers
type HTTPGateway struct {
	lspManager *LSPManager
	server     *http.Server
	mu         sync.RWMutex
}

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// NewHTTPGateway creates a new simple HTTP gateway
func NewHTTPGateway(addr string, cfg *config.Config) (*HTTPGateway, error) {
	lspManager, err := NewLSPManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create LSP manager: %w", err)
	}

	gateway := &HTTPGateway{
		lspManager: lspManager,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/jsonrpc", gateway.handleJSONRPC)
	mux.HandleFunc("/health", gateway.handleHealth)

	gateway.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return gateway, nil
}

// Start starts the HTTP gateway
func (g *HTTPGateway) Start(ctx context.Context) error {
	if err := g.lspManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start LSP manager: %w", err)
	}

	common.GatewayLogger.Info("Starting HTTP gateway on %s", g.server.Addr)

	go func() {
		if err := g.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			common.GatewayLogger.Error("HTTP server error: %v", err)
		}
	}()

	return nil
}

// Stop stops the HTTP gateway
func (g *HTTPGateway) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var lastErr error

	if g.server != nil {
		if err := g.server.Shutdown(ctx); err != nil {
			common.GatewayLogger.Error("HTTP server shutdown error: %v", err)
			lastErr = err
		}
	}

	if err := g.lspManager.Stop(); err != nil {
		common.GatewayLogger.Error("LSP manager stop error: %v", err)
		lastErr = err
	}

	return lastErr
}

// handleJSONRPC handles JSON-RPC requests
func (g *HTTPGateway) handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		g.writeError(w, nil, -32600, "Invalid Request", "Only POST method allowed")
		return
	}

	if r.Header.Get("Content-Type") != "application/json" {
		g.writeError(w, nil, -32600, "Invalid Request", "Content-Type must be application/json")
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, nil, -32700, "Parse error", err.Error())
		return
	}

	if req.JSONRPC != "2.0" {
		g.writeError(w, req.ID, -32600, "Invalid Request", "jsonrpc must be 2.0")
		return
	}

	common.GatewayLogger.Debug("Processing request: %s", req.Method)

	result, err := g.lspManager.ProcessRequest(r.Context(), req.Method, req.Params)
	if err != nil {
		g.writeError(w, req.ID, -32603, "Internal error", err.Error())
		return
	}

	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		common.GatewayLogger.Error("Failed to encode response: %v", err)
	}
}

// handleHealth handles health check requests
func (g *HTTPGateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":      "healthy",
		"lsp_clients": g.lspManager.GetClientStatus(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// writeError writes a JSON-RPC error response
func (g *HTTPGateway) writeError(w http.ResponseWriter, id interface{}, code int, message, data string) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // JSON-RPC errors still return 200
	json.NewEncoder(w).Encode(response)
}
