package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/project"
	versionpkg "lsp-gateway/src/internal/version"
	"lsp-gateway/src/server/cache"
)

// MCPServer provides Model Context Protocol bridge to LSP functionality
// Always uses SCIP cache for maximum LLM query performance
type MCPServer struct {
	lspManager  *LSPManager
	scipCache   cache.SCIPCache
	cacheWarmer *cache.CacheWarmupManager
	ctx         context.Context
	cancel      context.CancelFunc
}

// MCP Protocol Types - JSON-RPC 2.0 Compliant
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Tool content types for structured output
type ToolContent struct {
	Type     string      `json:"type"`
	Text     string      `json:"text,omitempty"`
	Data     interface{} `json:"data,omitempty"`
	MimeType string      `json:"mimeType,omitempty"`
}

// NewMCPServer creates a new MCP server with always-on SCIP cache for optimal LLM performance
func NewMCPServer(cfg *config.Config) (*MCPServer, error) {
	if cfg == nil {
		cfg = config.GetDefaultConfig()
	}

	// Always enable mandatory SCIP cache for MCP server (LLM optimization)
	if !cfg.IsCacheEnabled() {
		cfg.EnableCache()
		// Override with MCP-optimized cache settings
		cfg.Cache.MaxMemoryMB = 512                     // Increased memory for LLM workloads
		cfg.Cache.TTL = 60 * time.Minute                // Longer TTL for stable symbols
		cfg.Cache.BackgroundIndex = true                // Always enable background indexing
		cfg.Cache.HealthCheckInterval = 2 * time.Minute // More frequent health checks
		// Optimize for all supported languages
		cfg.Cache.Languages = []string{"go", "python", "javascript", "typescript", "java"}
		common.LSPLogger.Info("MCP server: Enabled mandatory SCIP cache with LLM-optimized settings")
	}

	// Create LSP manager - cache is now created and initialized internally
	lspManager, err := NewLSPManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create LSP manager: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create cache warmer for proactive LLM query optimization
	cachedLSPManager := cache.NewCachedLSPManager(lspManager)
	cacheWarmer := cache.NewCacheWarmupManager(cachedLSPManager)

	return &MCPServer{
		lspManager:  lspManager,
		scipCache:   lspManager.GetCache(), // Get cache from LSP manager
		cacheWarmer: cacheWarmer,
		ctx:         ctx,
		cancel:      cancel,
	}, nil
}

// Start starts the MCP server with cache warming for optimal LLM performance
func (m *MCPServer) Start() error {
	// Start SCIP cache first (if available)
	if m.scipCache != nil {
		if err := m.scipCache.Start(m.ctx); err != nil {
			common.LSPLogger.Error("Failed to start SCIP cache: %v, continuing without cache", err)
			// Graceful degradation: Continue without cache
			m.scipCache = nil
		} else {
			common.LSPLogger.Info("MCP server: SCIP cache started successfully")
		}
	} else {
		common.LSPLogger.Warn("MCP server: Starting without SCIP cache (cache unavailable)")
	}

	// Start LSP manager with cache integration
	if err := m.lspManager.Start(m.ctx); err != nil {
		return fmt.Errorf("failed to start LSP manager: %w", err)
	}

	// Start cache warmer for proactive optimization (if cache is available)
	if m.cacheWarmer != nil && m.scipCache != nil {
		m.cacheWarmer.Start()
		common.LSPLogger.Info("MCP server: Cache warming started for LLM query optimization")
	}

	return nil
}

// Stop stops the MCP server and cache systems
func (m *MCPServer) Stop() error {
	m.cancel()

	// Stop cache warmer first (if available)
	if m.cacheWarmer != nil {
		m.cacheWarmer.Stop()
		common.LSPLogger.Info("MCP server: Cache warmer stopped")
	}

	// Stop LSP manager
	lspErr := m.lspManager.Stop()
	if lspErr != nil {
		common.LSPLogger.Error("Error stopping LSP manager: %v", lspErr)
	}

	// Stop SCIP cache (if available)
	var cacheErr error
	if m.scipCache != nil {
		cacheErr = m.scipCache.Stop()
		if cacheErr != nil {
			common.LSPLogger.Error("Error stopping SCIP cache: %v", cacheErr)
		}
	}

	// Return the first error encountered
	if lspErr != nil {
		return lspErr
	}
	return cacheErr
}

// Run runs the MCP server with STDIO transport
func (m *MCPServer) Run(input io.Reader, output io.Writer) error {
	defer m.cancel()

	if err := m.Start(); err != nil {
		return fmt.Errorf("failed to start LSP manager: %w", err)
	}
	defer m.Stop()

	// Use line-based I/O as required by MCP STDIO transport spec
	scanner := bufio.NewScanner(input)

	for scanner.Scan() {
		select {
		case <-m.ctx.Done():
			return m.ctx.Err()
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		var req MCPRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			common.LSPLogger.Error("decode error: %v", err)
			continue
		}

		response := m.handleRequest(&req)

		// Encode response as single line JSON (no embedded newlines)
		responseBytes, err := json.Marshal(response)
		if err != nil {
			common.LSPLogger.Error("encode error: %v", err)
			continue
		}

		// Write response followed by newline as required by spec
		if _, err := fmt.Fprintf(output, "%s\n", string(responseBytes)); err != nil {
			common.LSPLogger.Error("write error: %v", err)
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("input scan error: %w", err)
	}

	return nil
}

// handleRequest processes MCP requests
func (m *MCPServer) handleRequest(req *MCPRequest) *MCPResponse {
	switch req.Method {
	case "initialize":
		return m.handleInitialize(req)
	case "tools/list":
		return m.handleToolsList(req)
	case "tools/call":
		return m.handleToolCall(req)
	default:
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: fmt.Sprintf("method not found: %s", req.Method),
				Data:    map[string]interface{}{"method": req.Method},
			},
		}
	}
}

// handleInitialize handles MCP initialize request with cache performance info
func (m *MCPServer) handleInitialize(req *MCPRequest) *MCPResponse {
	// Get current cache metrics (with graceful handling)
	cacheInfo := map[string]interface{}{
		"optimization":    "LLM_queries",
		"background_warm": m.cacheWarmer != nil,
	}

	if m.scipCache != nil {
		cacheMetrics := m.scipCache.GetMetrics()
		cacheInfo["enabled"] = true
		if cacheMetrics != nil {
			cacheInfo["health"] = cacheMetrics.HealthStatus
			cacheInfo["entries"] = cacheMetrics.EntryCount
		} else {
			cacheInfo["status"] = "metrics_unavailable"
		}
	} else {
		cacheInfo["enabled"] = false
		cacheInfo["fallback"] = "direct_LSP"
	}

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"protocolVersion": "2025-06-18",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{
					"listChanged": true,
				},
				"logging": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "lsp-gateway-mcp-cached",
				"version": versionpkg.GetVersion(),
				"title":   "LSP Gateway MCP Server (SCIP Cache Enabled)",
			},
			"_meta": map[string]interface{}{
				"lsp-gateway": map[string]interface{}{
					"supportedLanguages": []string{"go", "python", "javascript", "typescript", "java"},
					"lspFeatures":        []string{"definition", "references", "hover", "documentSymbol", "workspaceSymbol", "completion"},
					"cache":              cacheInfo,
					"performance_target": "sub_millisecond_cached_responses",
				},
			},
		},
	}
}

// handleToolsList returns available LSP tools
func (m *MCPServer) handleToolsList(req *MCPRequest) *MCPResponse {
	tools := []map[string]interface{}{
		{
			"name":        "goto_definition",
			"title":       "Go to Definition",
			"description": "Navigate to the definition of a symbol at a specific position in a file",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"uri": map[string]interface{}{
						"type":        "string",
						"description": "File URI (e.g., file:///path/to/file.go)",
						"pattern":     "^file://",
					},
					"line": map[string]interface{}{
						"type":        "integer",
						"description": "Line number (0-based)",
						"minimum":     0,
					},
					"character": map[string]interface{}{
						"type":        "integer",
						"description": "Character position (0-based)",
						"minimum":     0,
					},
				},
				"required":             []string{"uri", "line", "character"},
				"additionalProperties": false,
			},
		},
		{
			"name":        "find_references",
			"title":       "Find References",
			"description": "Find all references to a symbol at a specific position in a file",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"uri": map[string]interface{}{
						"type":        "string",
						"description": "File URI (e.g., file:///path/to/file.go)",
					},
					"line": map[string]interface{}{
						"type":        "integer",
						"description": "Line number (0-based)",
					},
					"character": map[string]interface{}{
						"type":        "integer",
						"description": "Character position (0-based)",
					},
					"includeDeclaration": map[string]interface{}{
						"type":        "boolean",
						"description": "Include the declaration in the results",
						"default":     true,
					},
				},
				"required": []string{"uri", "line", "character"},
			},
		},
		{
			"name":        "get_hover_info",
			"title":       "Get Hover Information",
			"description": "Get hover information for a symbol at a specific position in a file",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"uri": map[string]interface{}{
						"type":        "string",
						"description": "File URI (e.g., file:///path/to/file.go)",
					},
					"line": map[string]interface{}{
						"type":        "integer",
						"description": "Line number (0-based)",
					},
					"character": map[string]interface{}{
						"type":        "integer",
						"description": "Character position (0-based)",
					},
				},
				"required": []string{"uri", "line", "character"},
			},
		},
		{
			"name":        "get_document_symbols",
			"title":       "Get Document Symbols",
			"description": "Get all symbols in a document",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"uri": map[string]interface{}{
						"type":        "string",
						"description": "Document URI (e.g., file:///path/to/file.go)",
					},
				},
				"required": []string{"uri"},
			},
		},
		{
			"name":        "search_workspace_symbols",
			"title":       "Search Workspace Symbols",
			"description": "Search for symbols in the workspace with pagination support",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Search query for symbol names",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "get_completion",
			"title":       "Get Code Completion",
			"description": "Get code completion suggestions at a specific position in a file",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"uri": map[string]interface{}{
						"type":        "string",
						"description": "File URI (e.g., file:///path/to/file.go)",
					},
					"line": map[string]interface{}{
						"type":        "integer",
						"description": "Line number (0-based)",
					},
					"character": map[string]interface{}{
						"type":        "integer",
						"description": "Character position (0-based)",
					},
					"triggerKind": map[string]interface{}{
						"type":        "integer",
						"description": "Completion trigger kind (1=invoked, 2=triggerCharacter, 3=incomplete)",
						"default":     1,
					},
					"triggerCharacter": map[string]interface{}{
						"type":        "string",
						"description": "Character that triggered completion (when triggerKind=2)",
					},
				},
				"required": []string{"uri", "line", "character"},
			},
		},
	}

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"tools": tools,
		},
	}
}

// handleToolCall executes LSP tool calls with cache performance tracking
func (m *MCPServer) handleToolCall(req *MCPRequest) *MCPResponse {
	params, ok := req.Params.(map[string]interface{})
	if !ok {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32602,
				Message: "Invalid params: expected object",
				Data:    map[string]interface{}{"received": fmt.Sprintf("%T", req.Params)},
			},
		}
	}

	name, ok := params["name"].(string)
	if !ok {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32602,
				Message: "Missing required parameter: name",
				Data:    map[string]interface{}{"parameter": "name"},
			},
		}
	}

	args, ok := params["arguments"].(map[string]interface{})
	if !ok {
		args = make(map[string]interface{})
	}

	var result interface{}
	var err error

	// Record start time for performance tracking
	startTime := time.Now()

	switch name {
	case "goto_definition":
		result, err = m.handleGotoDefinition(args)
	case "find_references":
		result, err = m.handleFindReferences(args)
	case "get_hover_info":
		result, err = m.handleGetHover(args)
	case "get_document_symbols":
		result, err = m.handleGetDocumentSymbols(args)
	case "search_workspace_symbols":
		result, err = m.handleSearchWorkspaceSymbols(args)
	case "get_completion":
		result, err = m.handleGetCompletion(args)
	default:
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: fmt.Sprintf("Tool not found: %s", name),
				Data:    map[string]interface{}{"tool": name},
			},
		}
	}

	if err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32000,
				Message: fmt.Sprintf("Tool execution failed: %s", err.Error()),
				Data:    map[string]interface{}{"tool": name, "error": err.Error()},
			},
		}
	}

	// Calculate response time
	responseTime := time.Since(startTime)

	// Get cache metrics for performance reporting (with graceful handling)
	cacheStats := make(map[string]interface{})
	if m.scipCache != nil {
		cacheMetrics := m.scipCache.GetMetrics()
		if cacheMetrics != nil {
			hitRatio := float64(0)
			if cacheMetrics.HitCount+cacheMetrics.MissCount > 0 {
				hitRatio = float64(cacheMetrics.HitCount) / float64(cacheMetrics.HitCount+cacheMetrics.MissCount)
			}
			cacheStats = map[string]interface{}{
				"enabled":      true,
				"hit_ratio":    fmt.Sprintf("%.2f%%", hitRatio*100),
				"total_hits":   cacheMetrics.HitCount,
				"total_misses": cacheMetrics.MissCount,
				"health":       cacheMetrics.HealthStatus,
				"entry_count":  cacheMetrics.EntryCount,
			}
		} else {
			cacheStats = map[string]interface{}{
				"enabled": true,
				"status":  "metrics_unavailable",
			}
		}
	} else {
		cacheStats = map[string]interface{}{
			"enabled": false,
			"reason":  "cache_initialization_failed",
		}
	}

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": formatStructuredResult(result, name),
				},
			},
			"isError": false,
			"_meta": map[string]interface{}{
				"tool":          name,
				"timestamp":     fmt.Sprintf("%d", time.Now().Unix()),
				"response_time": fmt.Sprintf("%.2fms", float64(responseTime.Nanoseconds())/1000000),
				"cache":         cacheStats,
				"optimized_for": "LLM_queries",
			},
		},
	}
}

// LSP Tool Handlers - Optimized for cache utilization
func (m *MCPServer) handleGotoDefinition(params map[string]interface{}) (interface{}, error) {
	if err := validatePositionParams(params); err != nil {
		return nil, err
	}

	lspParams, err := m.buildPositionParams(params)
	if err != nil {
		return nil, err
	}

	// Pre-warm cache for adjacent positions (LLM usage pattern optimization)
	go m.preWarmAdjacentPositions(lspParams)

	return m.lspManager.ProcessRequest(m.ctx, "textDocument/definition", lspParams)
}

func (m *MCPServer) handleFindReferences(params map[string]interface{}) (interface{}, error) {
	if err := validatePositionParams(params); err != nil {
		return nil, err
	}

	lspParams, err := m.buildPositionParams(params)
	if err != nil {
		return nil, err
	}

	// Add includeDeclaration parameter
	if includeDecl, ok := params["includeDeclaration"].(bool); ok {
		lspParams["context"] = map[string]interface{}{
			"includeDeclaration": includeDecl,
		}
	} else {
		lspParams["context"] = map[string]interface{}{
			"includeDeclaration": true,
		}
	}

	return m.lspManager.ProcessRequest(m.ctx, "textDocument/references", lspParams)
}

func (m *MCPServer) handleGetHover(params map[string]interface{}) (interface{}, error) {
	if err := validatePositionParams(params); err != nil {
		return nil, err
	}

	lspParams, err := m.buildPositionParams(params)
	if err != nil {
		return nil, err
	}

	return m.lspManager.ProcessRequest(m.ctx, "textDocument/hover", lspParams)
}

func (m *MCPServer) handleGetDocumentSymbols(params map[string]interface{}) (interface{}, error) {
	uri, ok := params["uri"].(string)
	if !ok || uri == "" {
		return nil, fmt.Errorf("missing or invalid uri parameter")
	}
	if !isValidFileURI(uri) {
		return nil, fmt.Errorf("uri must be a valid file:// URI")
	}

	lspParams := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
	}

	return m.lspManager.ProcessRequest(m.ctx, "textDocument/documentSymbol", lspParams)
}

func (m *MCPServer) handleSearchWorkspaceSymbols(params map[string]interface{}) (interface{}, error) {
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("missing or invalid query parameter")
	}
	if len(query) > 100 {
		return nil, fmt.Errorf("query too long (max 100 characters)")
	}

	// Normalize query for better cache hit rates (common LLM usage patterns)
	normalizedQuery := m.normalizeSymbolQuery(query)

	lspParams := map[string]interface{}{
		"query": normalizedQuery,
	}

	// Pre-warm cache with common variations (async)
	go m.preWarmSymbolVariations(normalizedQuery)

	return m.lspManager.ProcessRequest(m.ctx, "workspace/symbol", lspParams)
}

func (m *MCPServer) handleGetCompletion(params map[string]interface{}) (interface{}, error) {
	if err := validatePositionParams(params); err != nil {
		return nil, err
	}

	lspParams, err := m.buildPositionParams(params)
	if err != nil {
		return nil, err
	}

	// Add completion context
	context := map[string]interface{}{
		"triggerKind": 1, // Default: invoked
	}

	if triggerKind, ok := params["triggerKind"].(float64); ok {
		context["triggerKind"] = int(triggerKind)
	}

	if triggerChar, ok := params["triggerCharacter"].(string); ok {
		context["triggerCharacter"] = triggerChar
	}

	lspParams["context"] = context

	return m.lspManager.ProcessRequest(m.ctx, "textDocument/completion", lspParams)
}

// Helper Functions
func (m *MCPServer) buildPositionParams(params map[string]interface{}) (map[string]interface{}, error) {
	uri, ok := params["uri"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid uri parameter")
	}

	line, ok := params["line"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid line parameter")
	}

	character, ok := params["character"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid character parameter")
	}

	return map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      int(line),
			"character": int(character),
		},
	}, nil
}

func formatStructuredResult(result interface{}, toolName string) string {
	if result == nil {
		return fmt.Sprintf("Tool '%s' returned no results", toolName)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting result for tool '%s': %v", toolName, err)
	}

	return string(data)
}

// Input validation for MCP tools
func validatePositionParams(params map[string]interface{}) error {
	uri, ok := params["uri"].(string)
	if !ok || uri == "" {
		return fmt.Errorf("missing or invalid uri parameter")
	}
	if !isValidFileURI(uri) {
		return fmt.Errorf("uri must be a valid file:// URI")
	}

	line, ok := params["line"].(float64)
	if !ok || line < 0 {
		return fmt.Errorf("missing or invalid line parameter (must be >= 0)")
	}

	character, ok := params["character"].(float64)
	if !ok || character < 0 {
		return fmt.Errorf("missing or invalid character parameter (must be >= 0)")
	}

	return nil
}

func isValidFileURI(uri string) bool {
	return len(uri) > 7 && uri[:7] == "file://"
}

// Cache optimization helpers for LLM usage patterns

// preWarmAdjacentPositions pre-warms cache for positions adjacent to current query
// This optimizes for LLM's tendency to query nearby code locations
func (m *MCPServer) preWarmAdjacentPositions(lspParams map[string]interface{}) {
	// Skip cache warming if cache is not available
	if m.scipCache == nil {
		return
	}

	// Extract position information
	position, ok := lspParams["position"].(map[string]interface{})
	if !ok {
		return
	}

	line, lineOk := position["line"].(int)
	character, charOk := position["character"].(int)
	if !lineOk || !charOk {
		return
	}

	// Pre-warm cache for adjacent lines (common LLM pattern)
	adjacents := []struct{ lineOffset, charOffset int }{
		{-1, 0}, {1, 0}, {0, -5}, {0, 5}, // Adjacent lines and characters
	}

	for _, adj := range adjacents {
		if line+adj.lineOffset >= 0 && character+adj.charOffset >= 0 {
			adjParams := make(map[string]interface{})
			for k, v := range lspParams {
				adjParams[k] = v
			}
			adjParams["position"] = map[string]interface{}{
				"line":      line + adj.lineOffset,
				"character": character + adj.charOffset,
			}

			// Fire and forget - cache warming
			go func(params map[string]interface{}) {
				m.lspManager.ProcessRequest(m.ctx, "textDocument/definition", params)
			}(adjParams)
		}
	}
}

// normalizeSymbolQuery normalizes symbol queries for better cache hit rates
func (m *MCPServer) normalizeSymbolQuery(query string) string {
	// Simple normalization: trim whitespace, convert to lowercase for better cache hits
	query = strings.TrimSpace(query)
	query = strings.ToLower(query)
	return query
}

// preWarmSymbolVariations pre-warms cache with common symbol query variations
func (m *MCPServer) preWarmSymbolVariations(query string) {
	// Skip cache warming if cache is not available
	if m.scipCache == nil || len(query) < 2 {
		return
	}

	// Common LLM patterns: prefixes and partial matches
	variations := []string{
		query[:len(query)/2], // Half query
		query + "*",          // Wildcard
	}

	for _, variation := range variations {
		if variation != query && len(variation) > 1 {
			params := map[string]interface{}{"query": variation}
			// Fire and forget - cache warming
			go func(p map[string]interface{}) {
				m.lspManager.ProcessRequest(m.ctx, "workspace/symbol", p)
			}(params)
		}
	}
}

// RunMCPServer starts an MCP server with the specified configuration
func RunMCPServer(configPath string) error {
	var cfg *config.Config
	if configPath != "" {
		loadedConfig, err := config.LoadConfig(configPath)
		if err != nil {
			common.LSPLogger.Warn("Failed to load config from %s, using defaults: %v", configPath, err)
			cfg = config.GetDefaultConfig()
		} else {
			cfg = loadedConfig
		}
	} else {
		// Auto-detect languages in current directory
		wd, err := os.Getwd()
		if err != nil {
			common.LSPLogger.Warn("Failed to get working directory, using defaults: %v", err)
			cfg = config.GetDefaultConfig()
		} else {
			common.LSPLogger.Info("Auto-detecting languages in: %s", wd)
			cfg = config.GenerateAutoConfig(wd, project.GetAvailableLanguages)
			if cfg == nil || len(cfg.Servers) == 0 {
				common.LSPLogger.Warn("No languages detected or LSP servers unavailable, using defaults")
				cfg = config.GetDefaultConfig()
			} else {
				languages := make([]string, 0, len(cfg.Servers))
				for lang := range cfg.Servers {
					languages = append(languages, lang)
				}
				common.LSPLogger.Info("Auto-detected languages: %v", languages)
			}
		}
	}

	server, err := NewMCPServer(cfg)
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	common.LSPLogger.Info("MCP Server started")
	common.LSPLogger.Info("Available LSP tools: goto_definition, find_references, get_hover_info, get_document_symbols, search_workspace_symbols, get_completion")
	common.LSPLogger.Info("Protocol: Model Context Protocol v2025-06-18 (STDIO)")
	common.LSPLogger.Info("Enhanced with: JSON-RPC 2.0 compliance, input validation, structured output")

	return server.Run(os.Stdin, os.Stdout)
}
