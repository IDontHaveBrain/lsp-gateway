package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/constants"
	versionpkg "lsp-gateway/src/internal/version"
	"lsp-gateway/src/server/protocol"
	"lsp-gateway/src/utils/configloader"
)

// MCPServer provides Model Context Protocol bridge to LSP functionality
// Always uses SCIP cache for maximum LLM query performance
type MCPServer struct {
	lspManager *LSPManager
	ctx        context.Context
	cancel     context.CancelFunc
	// Always operates in enhanced mode
}

func NewMCPServer(cfg *config.Config) (*MCPServer, error) {
	if cfg == nil {
		cfg = config.GetDefaultConfig()
	}

	// Always enable mandatory SCIP cache for MCP server (LLM optimization)
	if !cfg.IsCacheEnabled() {
		cfg.EnableCache()
		// Override with MCP-optimized cache settings
		cfg.Cache.MaxMemoryMB = constants.MCPCacheMemoryMB             // Increased memory for LLM workloads
		cfg.Cache.TTLHours = constants.MCPCacheTTLHours                // 1 hour TTL for stable symbols
		cfg.Cache.BackgroundIndex = true                               // Always enable background indexing
		cfg.Cache.HealthCheckMinutes = constants.MCPHealthCheckMinutes // More frequent health checks
		// Optimize for all supported languages
		cfg.Cache.Languages = config.GetAllSupportedLanguages()
	}

	// Create LSP manager - cache is now created and initialized internally
	lspManager, err := NewLSPManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create LSP manager: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &MCPServer{
		lspManager: lspManager,
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

// Start starts the MCP server with cache warming for optimal LLM performance
func (m *MCPServer) Start() error {
	cache := m.lspManager.GetCache()
	// MCP server starting with cache optimization

	// Start LSP manager (which handles SCIP cache startup internally)
	if err := m.lspManager.Start(m.ctx); err != nil {
		return fmt.Errorf("failed to start LSP manager: %w", err)
	}

	// Check cache status and perform indexing if needed
	if cache != nil {
		// Check immediately if cache has data (it loads synchronously in Start())
		stats := cache.GetIndexStats()
		if stats != nil && (stats.SymbolCount > 0 || stats.ReferenceCount > 0 || stats.DocumentCount > 0) {
			common.LSPLogger.Debug("MCP server: Using existing cache with %d symbols, %d references, %d documents",
				stats.SymbolCount, stats.ReferenceCount, stats.DocumentCount)
			// Don't perform any indexing - use existing cache
		} else {
			// Only perform initial indexing if cache is truly empty
			// Wait for LSP servers to be ready before indexing
			go func() {
				// Wait for LSP servers to fully initialize
				time.Sleep(3 * time.Second)

				// Double-check that cache is still empty (in case something else indexed it)
				currentCache := m.lspManager.GetCache()
				if currentCache == nil {
					return
				}
				recheckStats := currentCache.GetIndexStats()
				if recheckStats != nil && (recheckStats.SymbolCount > 0 || recheckStats.ReferenceCount > 0 || recheckStats.DocumentCount > 0) {
					common.LSPLogger.Debug("MCP server: Cache was populated while waiting (symbols=%d, refs=%d, docs=%d), skipping indexing",
						recheckStats.SymbolCount, recheckStats.ReferenceCount, recheckStats.DocumentCount)
					return
				}

				common.LSPLogger.Debug("MCP server: Cache is empty, performing initial indexing")
				m.performInitialIndexing()
			}()
		}
	} else {
		common.LSPLogger.Warn("MCP server: Starting without SCIP cache (cache unavailable)")
	}

	return nil
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
		return m.delegateToolCall(req)
	default:
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    protocol.MethodNotFound,
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

	cache := m.lspManager.GetCache()
	if cache != nil {
		cacheMetrics := cache.GetMetrics()
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
					"supportedLanguages": config.GetAllSupportedLanguages(),
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
					"containerPattern": map[string]interface{}{
						"type":        "string",
						"description": "Parent container filter (class/module name). Supports regex with (?i) for case-insensitive. Examples: 'MyClass', '(?i).*controller', 'Test.*'",
					},
					"symbolKinds": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "number",
						},
						"description": "Filter by symbol kinds. Common: 5=Class, 6=Method, 11=Interface, 12=Function, 13=Variable. Full list: 1=File, 2=Module, 3=Namespace, 4=Package, 7=Property, 8=Field, 9=Constructor, 10=Enum, 14=Constant, 23=Struct",
					},
					"symbolRoles": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "Filter by symbol roles: 'definition', 'reference', 'import', 'write', 'read', 'generated', 'test'. Can combine multiple roles.",
					},
					"maxResults": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of results to return (default: 100)",
					},
					"includeCode": map[string]interface{}{
						"type":        "boolean",
						"description": "Include source code for each symbol (default: false)",
					},
				},
				"required": []string{"pattern", "filePattern"},
			},
		},
		{
			"name":        "findReferences",
			"description": "Find all references to symbols matching a pattern in the codebase. Returns locations where matching symbols are used.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pattern": map[string]interface{}{
						"type":        "string",
						"description": "Symbol name or regex pattern to search. Use (?i) prefix for case-insensitive. Examples: 'handleRequest', '(?i)test.*', '^get[A-Z]', 'process.*Event$'",
					},
					"filePattern": map[string]interface{}{
						"type":        "string",
						"description": "File filter using directory path, glob pattern, or regex. Default: '**/*' (all files). Examples: '.', 'src/', 'tests/unit/', '*.java', 'src/**/*.py', '(?i)test.*\\.js$', '**/internal/*.go'",
					},
					"maxResults": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of references to return (default: 100)",
					},
				},
				"required": []string{"pattern"},
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

// RunMCPServer starts an MCP server with the specified configuration
func RunMCPServer(configPath string) error {
	cfg := configloader.LoadOrAuto(configPath)

	// Ensure cache path is project-specific so MCP shares the same cache as CLI
	if cfg != nil && cfg.Cache != nil {
		if wd, err := os.Getwd(); err == nil {
			projectPath := config.GetProjectSpecificCachePath(wd)
			cfg.SetCacheStoragePath(projectPath)
		}
	}

	server, err := NewMCPServer(cfg)
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	return server.Run(os.Stdin, os.Stdout)
}
