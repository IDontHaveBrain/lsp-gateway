package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/models/lsp"
	"lsp-gateway/src/internal/project"
	"lsp-gateway/src/internal/types"
	versionpkg "lsp-gateway/src/internal/version"
	"lsp-gateway/src/server/cache"
)

// MCPServer provides Model Context Protocol bridge to LSP functionality
// Always uses SCIP cache for maximum LLM query performance
type MCPServer struct {
	lspManager *LSPManager
	scipCache  cache.SCIPCache
	ctx        context.Context
	cancel     context.CancelFunc
	// Always operates in enhanced mode
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

func NewMCPServer(cfg *config.Config) (*MCPServer, error) {
	if cfg == nil {
		cfg = config.GetDefaultConfig()
	}

	// Always enable mandatory SCIP cache for MCP server (LLM optimization)
	if !cfg.IsCacheEnabled() {
		cfg.EnableCache()
		// Override with MCP-optimized cache settings
		cfg.Cache.MaxMemoryMB = 512      // Increased memory for LLM workloads
		cfg.Cache.TTLHours = 1           // 1 hour TTL for stable symbols
		cfg.Cache.BackgroundIndex = true // Always enable background indexing
		cfg.Cache.HealthCheckMinutes = 2 // More frequent health checks (every 2 minutes)
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

	return &MCPServer{
		lspManager: lspManager,
		scipCache:  lspManager.GetCache(), // Get cache from LSP manager
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

// Start starts the MCP server with cache warming for optimal LLM performance
func (m *MCPServer) Start() error {
	// Start LSP manager (which handles SCIP cache startup internally)
	if err := m.lspManager.Start(m.ctx); err != nil {
		return fmt.Errorf("failed to start LSP manager: %w", err)
	}

	// Log cache status after LSP manager has started
	if m.scipCache != nil {
		common.LSPLogger.Info("MCP server: SCIP cache started successfully")

		// Perform initial workspace symbol indexing
		go m.performInitialIndexing()
	} else {
		common.LSPLogger.Warn("MCP server: Starting without SCIP cache (cache unavailable)")
	}

	return nil
}

// indexWorkspaceFiles scans workspace files and indexes document symbols with full ranges
func (m *MCPServer) indexWorkspaceFiles(ctx context.Context) {
	common.LSPLogger.Info("MCP server: Scanning workspace files for document symbols")

	// Get workspace directory
	workspaceDir, err := os.Getwd()
	if err != nil {
		workspaceDir = "."
	}

	// Scan for source files based on configured languages
	extensions := m.getFileExtensions()
	common.LSPLogger.Info("MCP server: Using extensions: %v", extensions)
	files := m.scanSourceFiles(workspaceDir, extensions, 50) // Limit to 50 files for initial indexing

	common.LSPLogger.Info("MCP server: Found %d source files to index in %s", len(files), workspaceDir)

	if len(files) == 0 {
		common.LSPLogger.Warn("MCP server: No source files found for indexing")
		return
	}

	// Index each file's document symbols
	indexedCount := 0
	for i, file := range files {
		// Convert to file URI
		absPath, err := filepath.Abs(file)
		if err != nil {
			common.LSPLogger.Debug("Failed to get absolute path for %s: %v", file, err)
			continue
		}
		uri := "file://" + absPath

		// Get document symbols for this file
		params := map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": uri,
			},
		}

		common.LSPLogger.Debug("Getting document symbols for file %d/%d: %s", i+1, len(files), file)
		result, err := m.lspManager.ProcessRequest(ctx, types.MethodTextDocumentDocumentSymbol, params)
		if err != nil {
			common.LSPLogger.Debug("Failed to get document symbols for %s: %v", file, err)
			continue
		}

		// The ProcessRequest will automatically trigger indexing via indexDocumentSymbols
		// But let's log the result to see what we got
		if result != nil {
			common.LSPLogger.Debug("Got document symbols for %s", file)
			indexedCount++
		}
	}

	common.LSPLogger.Info("MCP server: Indexed %d/%d files with document symbols", indexedCount, len(files))
}

// getFileExtensions returns file extensions for configured languages
func (m *MCPServer) getFileExtensions() []string {
	extensions := []string{}
	extMap := map[string][]string{
		"go":         {".go"},
		"python":     {".py"},
		"javascript": {".js", ".jsx", ".mjs"},
		"typescript": {".ts", ".tsx"},
		"java":       {".java"},
	}

	// Get extensions for all configured servers
	for lang := range m.lspManager.GetConfiguredServers() {
		if exts, ok := extMap[lang]; ok {
			extensions = append(extensions, exts...)
		}
	}

	// Default to common extensions if none configured
	if len(extensions) == 0 {
		extensions = []string{".go", ".py", ".js", ".ts", ".java"}
	}

	return extensions
}

// scanSourceFiles scans directory for source files with given extensions
func (m *MCPServer) scanSourceFiles(dir string, extensions []string, maxFiles int) []string {
	var files []string
	count := 0

	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			// Skip hidden directories and common non-source directories
			if d != nil && d.IsDir() {
				name := d.Name()
				if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" ||
					name == "build" || name == "dist" || name == "target" || name == "__pycache__" {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Check if we've reached the limit
		if count >= maxFiles {
			return filepath.SkipDir
		}

		// Check file extension
		ext := filepath.Ext(path)
		for _, validExt := range extensions {
			if ext == validExt {
				files = append(files, path)
				count++
				break
			}
		}

		return nil
	})

	return files
}

// performInitialIndexing performs initial workspace symbol indexing
func (m *MCPServer) performInitialIndexing() {
	// Wait a bit for LSP servers to fully initialize
	time.Sleep(2 * time.Second)

	common.LSPLogger.Info("MCP server: Starting initial workspace symbol indexing")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// First, try to get all workspace symbols using a broad query
	// Use multiple common patterns to get more symbols
	patterns := []string{"", "a", "e", "s", "t", "i", "n", "r", "o"}
	allSymbols := make(map[string]bool) // Track unique symbols by key

	for _, pattern := range patterns {
		query := types.SymbolPatternQuery{
			Pattern:     pattern,
			FilePattern: "**/*",
			MaxResults:  5000,
		}

		result, err := m.lspManager.SearchSymbolPattern(ctx, query)
		if err != nil {
			common.LSPLogger.Debug("MCP server: Pattern '%s' failed: %v", pattern, err)
			continue
		}

		// Track unique symbols
		for _, sym := range result.Symbols {
			key := fmt.Sprintf("%s:%s:%d", sym.FilePath, sym.Name, sym.LineNumber)
			allSymbols[key] = true
		}

		common.LSPLogger.Debug("MCP server: Pattern '%s' found %d symbols", pattern, result.TotalCount)
	}

	common.LSPLogger.Info("MCP server: Initial indexing completed with %d unique symbols", len(allSymbols))

	// Additionally, scan workspace files and index them directly
	// This ensures we get document symbols with full ranges
	common.LSPLogger.Info("MCP server: Starting workspace file indexing")
	m.indexWorkspaceFiles(ctx)
	common.LSPLogger.Info("MCP server: Workspace file indexing completed")

	// Save the index to disk for persistence
	if cacheManager, ok := m.scipCache.(*cache.SimpleCacheManager); ok {
		if err := cacheManager.SaveIndexToDisk(); err != nil {
			common.LSPLogger.Error("MCP server: Failed to save index to disk: %v", err)
		} else {
			common.LSPLogger.Info("MCP server: Index saved to disk successfully")
		}
	}
}

// Stop stops the MCP server and cache systems
func (m *MCPServer) Stop() error {
	m.cancel()

	// Stop LSP manager (which handles SCIP cache shutdown internally)
	lspErr := m.lspManager.Stop()
	if lspErr != nil {
		common.LSPLogger.Error("Error stopping LSP manager: %v", lspErr)
		return lspErr
	}

	return nil
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
		"optimization": "LLM_queries",
	}

	if m.scipCache != nil {
		cacheMetrics := m.scipCache.GetMetrics()
		cacheInfo["enabled"] = true
		if cacheMetrics != nil {
			cacheInfo["health"] = "OK"
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

// handleToolsList returns available enhanced tools (no basic LSP tools)
func (m *MCPServer) handleToolsList(req *MCPRequest) *MCPResponse {
	// MCP server provides enhanced SCIP-based tools with occurrence metadata and role filtering
	tools := []map[string]interface{}{
		{
			"name":        "findSymbols",
			"description": "Find code symbols (functions, classes, variables) in specified files. Returns scored and ranked results matching your pattern.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pattern": map[string]interface{}{
						"type":        "string",
						"description": "Symbol name or regex pattern to search. Use (?i) prefix for case-insensitive. Examples: 'handleRequest', '(?i)test.*', '^get[A-Z]', 'process.*Event$'",
					},
					"filePattern": map[string]interface{}{
						"type":        "string",
						"description": "File filter using directory path, glob pattern, or regex. Examples: '.', 'src/', 'tests/unit/', '*.java', 'src/**/*.py', '(?i)test.*\\.js$', '**/internal/*.go'",
					},
					"symbolKinds": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "number",
						},
						"description": "Filter by symbol kinds. Common: 5=Class, 6=Method, 11=Interface, 12=Function, 13=Variable. Full list: 1=File, 2=Module, 3=Namespace, 4=Package, 7=Property, 8=Field, 9=Constructor, 10=Enum, 14=Constant, 23=Struct",
					},
					"containerPattern": map[string]interface{}{
						"type":        "string",
						"description": "Parent container filter (class/module name). Supports regex with (?i) for case-insensitive. Examples: 'MyClass', '(?i).*controller', 'Test.*'",
					},
					"maxResults": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of results to return (default: 100)",
					},
					"includeCode": map[string]interface{}{
						"type":        "boolean",
						"description": "Include source code for each symbol (default: false)",
					},
					"symbolRoles": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "Filter by symbol roles: 'definition', 'reference', 'import', 'write', 'read', 'generated', 'test'. Can combine multiple roles.",
					},
				},
				"required": []string{"pattern", "filePattern"},
			},
		},
		{
			"name":        "findReferences",
			"description": "Find all references to a specific symbol in the codebase. Returns locations where the symbol is used.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"symbolName": map[string]interface{}{
						"type":        "string",
						"description": "Name of the symbol to find references for. Examples: 'handleRequest', 'UserService', 'calculateTotal'",
					},
					"filePattern": map[string]interface{}{
						"type":        "string",
						"description": "File filter to limit search scope. Examples: 'src/', '*.go', '**/*.ts'. Default: '**/*' (all files)",
					},
					"includeDefinition": map[string]interface{}{
						"type":        "boolean",
						"description": "Include the symbol definition in results (default: false)",
					},
					"maxResults": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of references to return (default: 100)",
					},
					"includeCode": map[string]interface{}{
						"type":        "boolean",
						"description": "Include code context for each reference (default: false)",
					},
					"exactMatch": map[string]interface{}{
						"type":        "boolean",
						"description": "Require exact symbol name match (default: false)",
					},
					"symbolRoles": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "Filter by occurrence roles: 'definition', 'reference', 'import', 'write', 'read', 'generated', 'test'. Leave empty for all roles.",
					},
					"includeRelated": map[string]interface{}{
						"type":        "boolean",
						"description": "Include related symbols through implementation/inheritance relationships (default: false)",
					},
				},
				"required": []string{"symbolName"},
			},
		},
		{
			"name":        "findDefinitions",
			"description": "Find symbol definitions in the codebase. Returns only the definition occurrences of symbols.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"symbolName": map[string]interface{}{
						"type":        "string",
						"description": "Name of the symbol to find definitions for. Examples: 'handleRequest', 'UserService', 'calculateTotal'",
					},
					"filePattern": map[string]interface{}{
						"type":        "string",
						"description": "File filter to limit search scope. Examples: 'src/', '*.go', '**/*.ts'. Default: '**/*' (all files)",
					},
					"exactMatch": map[string]interface{}{
						"type":        "boolean",
						"description": "Require exact symbol name match (default: false)",
					},
					"maxResults": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of definitions to return (default: 100)",
					},
					"includeCode": map[string]interface{}{
						"type":        "boolean",
						"description": "Include code context for each definition (default: false)",
					},
				},
				"required": []string{"symbolName"},
			},
		},
		{
			"name":        "findImplementations",
			"description": "Find implementations of interfaces, abstract methods, or base classes. Returns symbols that implement the specified interface or method.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"symbolName": map[string]interface{}{
						"type":        "string",
						"description": "Name of the interface/abstract symbol to find implementations for. Examples: 'Handler', 'Repository', 'Service'",
					},
					"filePattern": map[string]interface{}{
						"type":        "string",
						"description": "File filter to limit search scope. Examples: 'src/', '*.go', '**/*.ts'. Default: '**/*' (all files)",
					},
					"exactMatch": map[string]interface{}{
						"type":        "boolean",
						"description": "Require exact symbol name match (default: false)",
					},
					"maxResults": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of implementations to return (default: 100)",
					},
					"includeCode": map[string]interface{}{
						"type":        "boolean",
						"description": "Include code context for each implementation (default: false)",
					},
				},
				"required": []string{"symbolName"},
			},
		},
		{
			"name":        "findWriteAccesses",
			"description": "Find write accesses to a symbol. Returns locations where the symbol is modified, assigned, or mutated.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"symbolName": map[string]interface{}{
						"type":        "string",
						"description": "Name of the symbol to find write accesses for. Examples: 'count', 'config', 'userData'",
					},
					"filePattern": map[string]interface{}{
						"type":        "string",
						"description": "File filter to limit search scope. Examples: 'src/', '*.go', '**/*.ts'. Default: '**/*' (all files)",
					},
					"exactMatch": map[string]interface{}{
						"type":        "boolean",
						"description": "Require exact symbol name match (default: false)",
					},
					"maxResults": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of write accesses to return (default: 100)",
					},
					"includeCode": map[string]interface{}{
						"type":        "boolean",
						"description": "Include code context for each write access (default: false)",
					},
				},
				"required": []string{"symbolName"},
			},
		},
		{
			"name":        "getSymbolInfo",
			"description": "Get detailed information about a specific symbol including documentation, relationships, and metadata.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"symbolName": map[string]interface{}{
						"type":        "string",
						"description": "Name of the symbol to get information for. Examples: 'handleRequest', 'UserService', 'calculateTotal'",
					},
					"filePattern": map[string]interface{}{
						"type":        "string",
						"description": "File filter to limit search scope. Examples: 'src/', '*.go', '**/*.ts'. Default: '**/*' (all files)",
					},
					"exactMatch": map[string]interface{}{
						"type":        "boolean",
						"description": "Require exact symbol name match (default: false)",
					},
					"includeRelationships": map[string]interface{}{
						"type":        "boolean",
						"description": "Include symbol relationships (implements, extends, calls, etc.) (default: true)",
					},
					"includeDocumentation": map[string]interface{}{
						"type":        "boolean",
						"description": "Include symbol documentation and signature information (default: true)",
					},
				},
				"required": []string{"symbolName"},
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

	// MCP server only handles enhanced tools, not basic LSP methods
	// Route to appropriate enhanced tool handler
	var result interface{}
	var err error

	switch name {
	case "findSymbols":
		result, err = m.handleFindSymbols(params)
	case "findReferences":
		result, err = m.handleFindSymbolReferences(params)
	case "findDefinitions":
		result, err = m.handleFindDefinitions(params)
	case "findImplementations":
		result, err = m.handleFindImplementations(params)
	case "findWriteAccesses":
		result, err = m.handleFindWriteAccesses(params)
	case "getSymbolInfo":
		result, err = m.handleGetSymbolInfo(params)
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
				Code:    -32603,
				Message: err.Error(),
				Data:    map[string]interface{}{"tool": name},
			},
		}
	}

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
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

	return m.lspManager.ProcessRequest(m.ctx, types.MethodTextDocumentDefinition, lspParams)
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

	return m.lspManager.ProcessRequest(m.ctx, types.MethodTextDocumentReferences, lspParams)
}

func (m *MCPServer) handleGetHover(params map[string]interface{}) (interface{}, error) {
	if err := validatePositionParams(params); err != nil {
		return nil, err
	}

	lspParams, err := m.buildPositionParams(params)
	if err != nil {
		return nil, err
	}

	return m.lspManager.ProcessRequest(m.ctx, types.MethodTextDocumentHover, lspParams)
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

	return m.lspManager.ProcessRequest(m.ctx, types.MethodTextDocumentDocumentSymbol, lspParams)
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

	return m.lspManager.ProcessRequest(m.ctx, types.MethodWorkspaceSymbol, lspParams)
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

	return m.lspManager.ProcessRequest(m.ctx, types.MethodTextDocumentCompletion, lspParams)
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
				m.lspManager.ProcessRequest(m.ctx, types.MethodTextDocumentDefinition, params)
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
				m.lspManager.ProcessRequest(m.ctx, types.MethodWorkspaceSymbol, p)
			}(params)
		}
	}
}

// handleFindSymbols handles the findSymbols tool for finding code symbols
func (m *MCPServer) handleFindSymbols(params map[string]interface{}) (interface{}, error) {
	common.LSPLogger.Debug("handleFindSymbols called with params: %+v", params)

	arguments, ok := params["arguments"].(map[string]interface{})
	if !ok {
		common.LSPLogger.Error("Failed to get arguments from params: %+v", params)
		return nil, fmt.Errorf("missing or invalid arguments")
	}

	pattern, ok := arguments["pattern"].(string)
	if !ok || pattern == "" {
		return nil, fmt.Errorf("pattern is required and must be a string")
	}

	filePattern, ok := arguments["filePattern"].(string)
	if !ok || filePattern == "" {
		return nil, fmt.Errorf("filePattern is required and must be a string")
	}

	query := types.SymbolPatternQuery{
		Pattern:     pattern,
		FilePattern: filePattern,
	}

	// Parse symbol kinds
	if kinds, ok := arguments["symbolKinds"].([]interface{}); ok {
		for _, k := range kinds {
			if kind, ok := k.(float64); ok {
				query.SymbolKinds = append(query.SymbolKinds, lsp.SymbolKind(kind))
			}
		}
	}

	// Parse max results
	if maxResults, ok := arguments["maxResults"].(float64); ok {
		query.MaxResults = int(maxResults)
	}

	// Parse include code
	if includeCode, ok := arguments["includeCode"].(bool); ok {
		query.IncludeCode = includeCode
	}

	// Parse symbol roles for enhanced filtering
	var roleFilter *types.SymbolRole
	if roles, ok := arguments["symbolRoles"].([]interface{}); ok {
		var combinedRole types.SymbolRole
		for _, r := range roles {
			if roleStr, ok := r.(string); ok {
				role := parseSymbolRole(roleStr)
				if role != 0 {
					combinedRole = combinedRole.AddRole(role)
				}
			}
		}
		if combinedRole != 0 {
			roleFilter = &combinedRole
		}
	}

	// Execute the search
	ctx := context.Background()
	common.LSPLogger.Debug("Calling SearchSymbolPattern with query: %+v", query)
	result, err := m.lspManager.SearchSymbolPattern(ctx, query)
	if err != nil {
		common.LSPLogger.Error("SearchSymbolPattern failed: %v", err)
		return nil, fmt.Errorf("symbol pattern search failed: %w", err)
	}
	common.LSPLogger.Debug("SearchSymbolPattern returned %d symbols", len(result.Symbols))

	// Apply role filtering if specified
	filteredSymbols := result.Symbols
	if roleFilter != nil {
		filteredSymbols = m.filterSymbolsByRole(result.Symbols, *roleFilter)
		common.LSPLogger.Debug("Role filtering reduced symbols from %d to %d", len(result.Symbols), len(filteredSymbols))
	}

	// Format the result for MCP with enhanced occurrence metadata
	formattedResult := map[string]interface{}{
		"symbols":    formatEnhancedSymbolsForMCP(filteredSymbols, roleFilter != nil),
		"totalCount": len(filteredSymbols),
		"truncated":  result.Truncated,
		"searchMetadata": map[string]interface{}{
			"pattern":     pattern,
			"filePattern": filePattern,
			"roleFilter":  formatRoleFilter(roleFilter),
			"searchType":  "enhanced_pattern_search",
		},
	}

	common.LSPLogger.Debug("Returning formatted result with %d filtered symbols", len(filteredSymbols))

	// Wrap result in content array for MCP response
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("Found %d symbols matching pattern '%s'", len(filteredSymbols), pattern),
			},
			{
				"type": "text",
				"text": formatStructuredResult(formattedResult, "findSymbols"),
			},
		},
	}, nil
}

// formatSymbolsForMCP formats enhanced symbol info for MCP response
func formatSymbolsForMCP(symbols []types.EnhancedSymbolInfo) []map[string]interface{} {
	formatted := make([]map[string]interface{}, len(symbols))

	for i, sym := range symbols {
		// Build simplified response with only meaningful data for LLMs
		result := map[string]interface{}{
			"name": sym.Name,
			"type": getSymbolKindName(sym.Kind),
		}

		// Format location
		// For symbols with scope (classes, methods, functions, etc.), always show range format
		// For other symbols, show range only if start and end differ
		if hasScope(sym.Kind) || sym.LineNumber != sym.EndLine {
			result["location"] = fmt.Sprintf("%s:%d-%d", sym.FilePath, sym.LineNumber, sym.EndLine)
		} else {
			result["location"] = fmt.Sprintf("%s:%d", sym.FilePath, sym.LineNumber)
		}

		// Only include optional fields if they have meaningful content
		if sym.Container != "" {
			result["container"] = sym.Container
		}
		if sym.Signature != "" {
			result["signature"] = sym.Signature
		}
		if sym.Documentation != "" {
			result["documentation"] = sym.Documentation
		}
		if sym.Code != "" {
			result["code"] = sym.Code
		}

		formatted[i] = result
	}

	return formatted
}

// hasScope returns true if the symbol kind typically has a scope (multiple lines)
func hasScope(kind lsp.SymbolKind) bool {
	switch kind {
	case lsp.Class, lsp.Method, lsp.Function, lsp.Constructor,
		lsp.Interface, lsp.Namespace, lsp.Module, lsp.Struct, lsp.Enum:
		return true
	default:
		return false
	}
}

// getSymbolKindName returns the string representation of a symbol kind
func getSymbolKindName(kind lsp.SymbolKind) string {
	switch kind {
	case lsp.File:
		return "File"
	case lsp.Module:
		return "Module"
	case lsp.Namespace:
		return "Namespace"
	case lsp.Package:
		return "Package"
	case lsp.Class:
		return "Class"
	case lsp.Method:
		return "Method"
	case lsp.Property:
		return "Property"
	case lsp.Field:
		return "Field"
	case lsp.Constructor:
		return "Constructor"
	case lsp.Enum:
		return "Enum"
	case lsp.Interface:
		return "Interface"
	case lsp.Function:
		return "Function"
	case lsp.Variable:
		return "Variable"
	case lsp.Constant:
		return "Constant"
	case lsp.String:
		return "String"
	case lsp.Number:
		return "Number"
	case lsp.Boolean:
		return "Boolean"
	case lsp.Array:
		return "Array"
	case lsp.Object:
		return "Object"
	case lsp.Key:
		return "Key"
	case lsp.Null:
		return "Null"
	case lsp.EnumMember:
		return "EnumMember"
	case lsp.Struct:
		return "Struct"
	case lsp.Event:
		return "Event"
	case lsp.Operator:
		return "Operator"
	case lsp.TypeParameter:
		return "TypeParameter"
	default:
		return "Unknown"
	}
}

// handleFindSymbolReferences handles the findReferences tool for finding all references to a symbol
func (m *MCPServer) handleFindSymbolReferences(params map[string]interface{}) (interface{}, error) {
	common.LSPLogger.Debug("handleFindSymbolReferences called with params: %+v", params)

	arguments, ok := params["arguments"].(map[string]interface{})
	if !ok {
		common.LSPLogger.Error("Failed to get arguments from params: %+v", params)
		return nil, fmt.Errorf("missing or invalid arguments")
	}

	symbolName, ok := arguments["symbolName"].(string)
	if !ok || symbolName == "" {
		return nil, fmt.Errorf("symbolName is required and must be a string")
	}

	query := SymbolReferenceQuery{
		SymbolName: symbolName,
	}

	// Parse file pattern
	if filePattern, ok := arguments["filePattern"].(string); ok {
		query.FilePattern = filePattern
	} else {
		query.FilePattern = "**/*"
	}

	// Parse include definition
	if includeDefinition, ok := arguments["includeDefinition"].(bool); ok {
		query.IncludeDefinition = includeDefinition
	}

	// Parse max results
	if maxResults, ok := arguments["maxResults"].(float64); ok {
		query.MaxResults = int(maxResults)
	} else {
		query.MaxResults = 100
	}

	// Parse include code
	if includeCode, ok := arguments["includeCode"].(bool); ok {
		query.IncludeCode = includeCode
	}

	// Parse exact match
	if exactMatch, ok := arguments["exactMatch"].(bool); ok {
		query.ExactMatch = exactMatch
	}

	// Parse include related
	if includeRelated, ok := arguments["includeRelated"].(bool); ok {
		query.IncludeRelated = includeRelated
	}

	// Parse symbol roles for enhanced filtering
	if roles, ok := arguments["symbolRoles"].([]interface{}); ok {
		var combinedRole types.SymbolRole
		for _, r := range roles {
			if roleStr, ok := r.(string); ok {
				role := parseSymbolRole(roleStr)
				if role != 0 {
					combinedRole = combinedRole.AddRole(role)
				}
			}
		}
		if combinedRole != 0 {
			query.FilterByRole = &combinedRole
		}
	}

	// Execute the search
	ctx := context.Background()
	common.LSPLogger.Debug("Calling SearchSymbolReferences with query: %+v", query)
	result, err := m.lspManager.SearchSymbolReferences(ctx, query)
	if err != nil {
		common.LSPLogger.Error("SearchSymbolReferences failed: %v", err)
		return nil, fmt.Errorf("symbol references search failed: %w", err)
	}
	common.LSPLogger.Debug("SearchSymbolReferences returned %d references", len(result.References))

	// Format the result for MCP with enhanced metadata
	formattedResult := map[string]interface{}{
		"references":       formatEnhancedReferencesForMCP(result.References),
		"totalCount":       result.TotalCount,
		"truncated":        result.Truncated,
		"definitionCount":  result.DefinitionCount,
		"readAccessCount":  result.ReadAccessCount,
		"writeAccessCount": result.WriteAccessCount,
		"implementations":  formatEnhancedReferencesForMCP(result.Implementations),
		"relatedSymbols":   result.RelatedSymbols,
		"searchMetadata":   result.SearchMetadata,
	}

	common.LSPLogger.Debug("Returning formatted result with %d references", result.TotalCount)

	// Wrap result in content array for MCP response
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("Found %d references to symbol '%s' (%d definitions, %d reads, %d writes)",
					result.TotalCount, symbolName, result.DefinitionCount, result.ReadAccessCount, result.WriteAccessCount),
			},
			{
				"type": "text",
				"text": formatStructuredResult(formattedResult, "findReferences"),
			},
		},
	}, nil
}

// formatReferencesForMCP formats reference info for MCP response
func formatReferencesForMCP(references []ReferenceInfo) []map[string]interface{} {
	formatted := make([]map[string]interface{}, len(references))

	for i, ref := range references {
		result := map[string]interface{}{
			"location": fmt.Sprintf("%s:%d:%d", ref.FilePath, ref.LineNumber, ref.Column),
		}

		// Only include optional fields if they have meaningful content
		if ref.Text != "" {
			result["text"] = ref.Text
		}
		if ref.Code != "" {
			result["code"] = ref.Code
		}
		if ref.Context != "" {
			result["context"] = ref.Context
		}

		formatted[i] = result
	}

	return formatted
}

// handleFindDefinitions handles the findDefinitions tool for finding symbol definitions
func (m *MCPServer) handleFindDefinitions(params map[string]interface{}) (interface{}, error) {
	common.LSPLogger.Debug("handleFindDefinitions called with params: %+v", params)

	arguments, ok := params["arguments"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid arguments")
	}

	symbolName, ok := arguments["symbolName"].(string)
	if !ok || symbolName == "" {
		return nil, fmt.Errorf("symbolName is required and must be a string")
	}

	// Create a reference query that only includes definitions
	query := SymbolReferenceQuery{
		SymbolName:        symbolName,
		FilePattern:       "**/*",
		IncludeDefinition: true,
		MaxResults:        100,
		ExactMatch:        false,
	}

	// Set file pattern if provided
	if filePattern, ok := arguments["filePattern"].(string); ok {
		query.FilePattern = filePattern
	}

	// Set exact match if provided
	if exactMatch, ok := arguments["exactMatch"].(bool); ok {
		query.ExactMatch = exactMatch
	}

	// Set max results if provided
	if maxResults, ok := arguments["maxResults"].(float64); ok {
		query.MaxResults = int(maxResults)
	}

	// Set include code if provided
	if includeCode, ok := arguments["includeCode"].(bool); ok {
		query.IncludeCode = includeCode
	}

	// Filter only definition roles
	definitionRole := types.SymbolRoleDefinition
	query.FilterByRole = &definitionRole

	// Execute the search
	ctx := context.Background()
	result, err := m.lspManager.SearchSymbolReferences(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("symbol definitions search failed: %w", err)
	}

	// Filter to only definitions (extra safety)
	definitions := []ReferenceInfo{}
	for _, ref := range result.References {
		if ref.IsDefinition {
			definitions = append(definitions, ref)
		}
	}

	formattedResult := map[string]interface{}{
		"definitions": formatEnhancedReferencesForMCP(definitions),
		"totalCount":  len(definitions),
		"truncated":   result.Truncated,
		"searchMetadata": map[string]interface{}{
			"symbolName": symbolName,
			"searchType": "definitions_only",
			"exactMatch": query.ExactMatch,
		},
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("Found %d definitions for symbol '%s'", len(definitions), symbolName),
			},
			{
				"type": "text",
				"text": formatStructuredResult(formattedResult, "findDefinitions"),
			},
		},
	}, nil
}

// handleFindImplementations handles the findImplementations tool for finding symbol implementations
func (m *MCPServer) handleFindImplementations(params map[string]interface{}) (interface{}, error) {
	common.LSPLogger.Debug("handleFindImplementations called with params: %+v", params)

	arguments, ok := params["arguments"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid arguments")
	}

	symbolName, ok := arguments["symbolName"].(string)
	if !ok || symbolName == "" {
		return nil, fmt.Errorf("symbolName is required and must be a string")
	}

	// Create a reference query that includes related symbols (implementations)
	query := SymbolReferenceQuery{
		SymbolName:        symbolName,
		FilePattern:       "**/*",
		IncludeDefinition: false,
		IncludeRelated:    true, // This will include implementations
		MaxResults:        100,
		ExactMatch:        false,
	}

	// Set file pattern if provided
	if filePattern, ok := arguments["filePattern"].(string); ok {
		query.FilePattern = filePattern
	}

	// Set exact match if provided
	if exactMatch, ok := arguments["exactMatch"].(bool); ok {
		query.ExactMatch = exactMatch
	}

	// Set max results if provided
	if maxResults, ok := arguments["maxResults"].(float64); ok {
		query.MaxResults = int(maxResults)
	}

	// Set include code if provided
	if includeCode, ok := arguments["includeCode"].(bool); ok {
		query.IncludeCode = includeCode
	}

	// Execute the search
	ctx := context.Background()
	result, err := m.lspManager.SearchSymbolReferences(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("symbol implementations search failed: %w", err)
	}

	// Use the implementations from the result
	implementations := result.Implementations
	if implementations == nil {
		implementations = []ReferenceInfo{}
	}

	formattedResult := map[string]interface{}{
		"implementations": formatEnhancedReferencesForMCP(implementations),
		"totalCount":      len(implementations),
		"relatedSymbols":  result.RelatedSymbols,
		"searchMetadata": map[string]interface{}{
			"symbolName": symbolName,
			"searchType": "implementations_only",
			"exactMatch": query.ExactMatch,
		},
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("Found %d implementations for symbol '%s'", len(implementations), symbolName),
			},
			{
				"type": "text",
				"text": formatStructuredResult(formattedResult, "findImplementations"),
			},
		},
	}, nil
}

// handleFindWriteAccesses handles the findWriteAccesses tool for finding symbol write accesses
func (m *MCPServer) handleFindWriteAccesses(params map[string]interface{}) (interface{}, error) {
	common.LSPLogger.Debug("handleFindWriteAccesses called with params: %+v", params)

	arguments, ok := params["arguments"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid arguments")
	}

	symbolName, ok := arguments["symbolName"].(string)
	if !ok || symbolName == "" {
		return nil, fmt.Errorf("symbolName is required and must be a string")
	}

	// Create a reference query that only includes write accesses
	query := SymbolReferenceQuery{
		SymbolName:      symbolName,
		FilePattern:     "**/*",
		OnlyWriteAccess: true, // Filter only write accesses
		MaxResults:      100,
		ExactMatch:      false,
	}

	// Set file pattern if provided
	if filePattern, ok := arguments["filePattern"].(string); ok {
		query.FilePattern = filePattern
	}

	// Set exact match if provided
	if exactMatch, ok := arguments["exactMatch"].(bool); ok {
		query.ExactMatch = exactMatch
	}

	// Set max results if provided
	if maxResults, ok := arguments["maxResults"].(float64); ok {
		query.MaxResults = int(maxResults)
	}

	// Set include code if provided
	if includeCode, ok := arguments["includeCode"].(bool); ok {
		query.IncludeCode = includeCode
	}

	// Also set role filter for extra safety
	writeRole := types.SymbolRoleWriteAccess
	query.FilterByRole = &writeRole

	// Execute the search
	ctx := context.Background()
	result, err := m.lspManager.SearchSymbolReferences(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("symbol write accesses search failed: %w", err)
	}

	// Filter to only write accesses (extra safety)
	writeAccesses := []ReferenceInfo{}
	for _, ref := range result.References {
		if ref.IsWriteAccess {
			writeAccesses = append(writeAccesses, ref)
		}
	}

	formattedResult := map[string]interface{}{
		"writeAccesses": formatEnhancedReferencesForMCP(writeAccesses),
		"totalCount":    len(writeAccesses),
		"truncated":     result.Truncated,
		"searchMetadata": map[string]interface{}{
			"symbolName": symbolName,
			"searchType": "write_accesses_only",
			"exactMatch": query.ExactMatch,
		},
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("Found %d write accesses for symbol '%s'", len(writeAccesses), symbolName),
			},
			{
				"type": "text",
				"text": formatStructuredResult(formattedResult, "findWriteAccesses"),
			},
		},
	}, nil
}

// handleGetSymbolInfo handles the getSymbolInfo tool for getting detailed symbol information
func (m *MCPServer) handleGetSymbolInfo(params map[string]interface{}) (interface{}, error) {
	common.LSPLogger.Debug("handleGetSymbolInfo called with params: %+v", params)

	arguments, ok := params["arguments"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid arguments")
	}

	symbolName, ok := arguments["symbolName"].(string)
	if !ok || symbolName == "" {
		return nil, fmt.Errorf("symbolName is required and must be a string")
	}

	// Parse options
	includeRelationships := true
	if incRel, ok := arguments["includeRelationships"].(bool); ok {
		includeRelationships = incRel
	}

	includeDocumentation := true
	if incDoc, ok := arguments["includeDocumentation"].(bool); ok {
		includeDocumentation = incDoc
	}

	filePattern := "**/*"
	if fp, ok := arguments["filePattern"].(string); ok {
		filePattern = fp
	}

	exactMatch := false
	if em, ok := arguments["exactMatch"].(bool); ok {
		exactMatch = em
	}

	ctx := context.Background()

	// First, find the symbol using SearchSymbolPattern to get detailed info
	symbolQuery := types.SymbolPatternQuery{
		Pattern:     symbolName,
		FilePattern: filePattern,
		MaxResults:  1,
		IncludeCode: true,
	}

	if exactMatch {
		symbolQuery.Pattern = "^" + symbolName + "$"
	}

	symbolResult, err := m.lspManager.SearchSymbolPattern(ctx, symbolQuery)
	if err != nil || len(symbolResult.Symbols) == 0 {
		return nil, fmt.Errorf("symbol not found: %s", symbolName)
	}

	symbol := symbolResult.Symbols[0]

	// Get references if relationships are requested
	var references []ReferenceInfo
	var implementations []ReferenceInfo
	var relatedSymbols []string

	if includeRelationships {
		refQuery := SymbolReferenceQuery{
			SymbolName:        symbolName,
			FilePattern:       filePattern,
			IncludeDefinition: true,
			IncludeRelated:    true,
			MaxResults:        50, // Limit for detailed info
			ExactMatch:        exactMatch,
		}

		if refResult, err := m.lspManager.SearchSymbolReferences(ctx, refQuery); err == nil {
			references = refResult.References
			implementations = refResult.Implementations
			relatedSymbols = refResult.RelatedSymbols
		}
	}

	// Build detailed symbol info
	symbolInfo := map[string]interface{}{
		"name":       symbol.Name,
		"kind":       getSymbolKindName(symbol.Kind),
		"location":   fmt.Sprintf("%s:%d-%d", symbol.FilePath, symbol.LineNumber, symbol.EndLine),
		"container":  symbol.Container,
		"filePath":   symbol.FilePath,
		"lineNumber": symbol.LineNumber,
		"endLine":    symbol.EndLine,
	}

	if includeDocumentation {
		if symbol.Signature != "" {
			symbolInfo["signature"] = symbol.Signature
		}
		if symbol.Documentation != "" {
			symbolInfo["documentation"] = symbol.Documentation
		}
		if symbol.Code != "" {
			symbolInfo["code"] = symbol.Code
		}
	}

	if includeRelationships {
		symbolInfo["referencesCount"] = len(references)
		symbolInfo["implementationsCount"] = len(implementations)
		symbolInfo["relatedSymbols"] = relatedSymbols

		// Include sample references (up to 5)
		if len(references) > 0 {
			maxSamples := 5
			if len(references) < maxSamples {
				maxSamples = len(references)
			}
			symbolInfo["sampleReferences"] = formatEnhancedReferencesForMCP(references[:maxSamples])
		}

		// Include sample implementations (up to 3)
		if len(implementations) > 0 {
			maxImpls := 3
			if len(implementations) < maxImpls {
				maxImpls = len(implementations)
			}
			symbolInfo["sampleImplementations"] = formatEnhancedReferencesForMCP(implementations[:maxImpls])
		}
	}

	formattedResult := map[string]interface{}{
		"symbolInfo": symbolInfo,
		"searchMetadata": map[string]interface{}{
			"symbolName":           symbolName,
			"searchType":           "detailed_symbol_info",
			"includeRelationships": includeRelationships,
			"includeDocumentation": includeDocumentation,
			"exactMatch":           exactMatch,
		},
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("Symbol information for '%s' (%s)", symbolName, getSymbolKindName(symbol.Kind)),
			},
			{
				"type": "text",
				"text": formatStructuredResult(formattedResult, "getSymbolInfo"),
			},
		},
	}, nil
}

// parseSymbolRole converts a string role name to SymbolRole bitflag
func parseSymbolRole(roleStr string) types.SymbolRole {
	switch strings.ToLower(roleStr) {
	case "definition":
		return types.SymbolRoleDefinition
	case "reference":
		return types.SymbolRoleReadAccess // Default references are read access
	case "import":
		return types.SymbolRoleImport
	case "write":
		return types.SymbolRoleWriteAccess
	case "read":
		return types.SymbolRoleReadAccess
	case "generated":
		return types.SymbolRoleGenerated
	case "test":
		return types.SymbolRoleTest
	case "forward":
		return types.SymbolRoleForwardDefinition
	default:
		return 0 // Unknown role
	}
}

// formatRoleFilter formats a role filter for metadata display
func formatRoleFilter(roleFilter *types.SymbolRole) []string {
	if roleFilter == nil {
		return []string{}
	}

	var roles []string
	if roleFilter.HasRole(types.SymbolRoleDefinition) {
		roles = append(roles, "definition")
	}
	if roleFilter.HasRole(types.SymbolRoleImport) {
		roles = append(roles, "import")
	}
	if roleFilter.HasRole(types.SymbolRoleWriteAccess) {
		roles = append(roles, "write")
	}
	if roleFilter.HasRole(types.SymbolRoleReadAccess) {
		roles = append(roles, "read")
	}
	if roleFilter.HasRole(types.SymbolRoleGenerated) {
		roles = append(roles, "generated")
	}
	if roleFilter.HasRole(types.SymbolRoleTest) {
		roles = append(roles, "test")
	}
	if roleFilter.HasRole(types.SymbolRoleForwardDefinition) {
		roles = append(roles, "forward")
	}
	return roles
}

// filterSymbolsByRole filters symbols based on role (placeholder implementation)
// Note: This is a placeholder since EnhancedSymbolInfo doesn't currently have role information
// In a full implementation, we would need occurrence data to filter by roles
func (m *MCPServer) filterSymbolsByRole(symbols []types.EnhancedSymbolInfo, roleFilter types.SymbolRole) []types.EnhancedSymbolInfo {
	// For now, return all symbols since we don't have role data in EnhancedSymbolInfo
	// This would need to be enhanced with actual occurrence-based filtering
	common.LSPLogger.Debug("Role filtering not fully implemented for symbol search - returning all symbols")
	return symbols
}

// formatEnhancedSymbolsForMCP formats symbols with enhanced metadata including occurrence roles
func formatEnhancedSymbolsForMCP(symbols []types.EnhancedSymbolInfo, hasRoleFilter bool) []map[string]interface{} {
	formatted := make([]map[string]interface{}, len(symbols))

	for i, sym := range symbols {
		// Build enhanced response with SCIP-style occurrence metadata
		result := map[string]interface{}{
			"name":     sym.Name,
			"kind":     getSymbolKindName(sym.Kind),
			"location": fmt.Sprintf("%s:%d-%d", sym.FilePath, sym.LineNumber, sym.EndLine),
		}

		// Add optional fields
		if sym.Container != "" {
			result["container"] = sym.Container
		}
		if sym.Signature != "" {
			result["signature"] = sym.Signature
		}
		if sym.Documentation != "" {
			result["documentation"] = sym.Documentation
		}
		if sym.Code != "" {
			result["code"] = sym.Code
		}

		// Add enhanced metadata for SCIP features (placeholder)
		result["occurrenceMetadata"] = map[string]interface{}{
			"filePath":      sym.FilePath,
			"lineNumber":    sym.LineNumber,
			"endLine":       sym.EndLine,
			"hasRoleFilter": hasRoleFilter,
		}

		formatted[i] = result
	}

	return formatted
}

// formatEnhancedReferencesForMCP formats references with enhanced SCIP occurrence metadata
func formatEnhancedReferencesForMCP(references []ReferenceInfo) []map[string]interface{} {
	formatted := make([]map[string]interface{}, len(references))

	for i, ref := range references {
		result := map[string]interface{}{
			"location": fmt.Sprintf("%s:%d:%d", ref.FilePath, ref.LineNumber, ref.Column),
		}

		// Include basic text fields if available
		if ref.Text != "" {
			result["text"] = ref.Text
		}
		if ref.Code != "" {
			result["code"] = ref.Code
		}
		if ref.Context != "" {
			result["context"] = ref.Context
		}

		// Enhanced SCIP occurrence metadata
		occurrenceMetadata := map[string]interface{}{
			"symbolId": ref.SymbolID,
		}

		// Role-based metadata
		var roles []string
		if ref.IsDefinition {
			roles = append(roles, "definition")
		}
		if ref.IsReadAccess {
			roles = append(roles, "read")
		}
		if ref.IsWriteAccess {
			roles = append(roles, "write")
		}
		if ref.IsImport {
			roles = append(roles, "import")
		}
		if ref.IsGenerated {
			roles = append(roles, "generated")
		}
		if ref.IsTest {
			roles = append(roles, "test")
		}

		if len(roles) > 0 {
			occurrenceMetadata["roles"] = roles
		}

		// Role indicator flags for easier filtering
		occurrenceMetadata["isDefinition"] = ref.IsDefinition
		occurrenceMetadata["isReadAccess"] = ref.IsReadAccess
		occurrenceMetadata["isWriteAccess"] = ref.IsWriteAccess
		occurrenceMetadata["isImport"] = ref.IsImport
		occurrenceMetadata["isGenerated"] = ref.IsGenerated
		occurrenceMetadata["isTest"] = ref.IsTest

		// Add relationships if available
		if len(ref.Relationships) > 0 {
			occurrenceMetadata["relationships"] = ref.Relationships
		}

		// Add documentation if available
		if len(ref.Documentation) > 0 {
			occurrenceMetadata["documentation"] = ref.Documentation
		}

		// Add range information if available
		if ref.Range != nil {
			occurrenceMetadata["range"] = map[string]interface{}{
				"start": map[string]interface{}{
					"line":      ref.Range.Start.Line,
					"character": ref.Range.Start.Character,
				},
				"end": map[string]interface{}{
					"line":      ref.Range.End.Line,
					"character": ref.Range.End.Character,
				},
			}
		}

		result["occurrenceMetadata"] = occurrenceMetadata
		formatted[i] = result
	}

	return formatted
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

	common.LSPLogger.Info("Starting MCP server in enhanced mode")

	server, err := NewMCPServer(cfg)
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	common.LSPLogger.Info("MCP Server started (Enhanced mode)")
	common.LSPLogger.Info("Protocol: Model Context Protocol v2025-06-18 (STDIO)")
	common.LSPLogger.Info("Mode: Enhanced tools only (no basic LSP functions)")
	common.LSPLogger.Info("Note: Use 'lsp-gateway server' for basic LSP functionality")

	return server.Run(os.Stdin, os.Stdout)
}
