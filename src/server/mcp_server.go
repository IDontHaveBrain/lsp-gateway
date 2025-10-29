package server

import (
	"context"
	"fmt"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/types"
	versionpkg "lsp-gateway/src/internal/version"
)

// MCPSymbolService defines the contract required by MCP tools to discover symbols
// and references. The default implementation is backed by the LSP manager/SCIP cache,
// but tests can substitute custom implementations.
type MCPSymbolService interface {
	FindSymbols(ctx context.Context, query SymbolSearchQuery) (*SymbolSearchResult, error)
	FindReferences(ctx context.Context, query SymbolReferenceQuery) (*SymbolReferenceResult, error)
}

// SymbolSearchQuery wraps the public SymbolPatternQuery with defaults already applied.
type SymbolSearchQuery struct {
	Pattern     string
	FilePattern string
	MaxResults  int
	IncludeCode bool
}

// SymbolSearchResult is a convenience alias for SymbolPatternResult to avoid
// leaking additional metadata in tool handlers.
type SymbolSearchResult struct {
	Symbols    []types.EnhancedSymbolInfo
	TotalCount int
	Truncated  bool
}

// MCPServer exposes the Model Context Protocol stdio server powered by go-sdk.
type MCPServer struct {
	cfg             *config.Config
	lspManager      *LSPManager
	symbolService   MCPSymbolService
	impl            *mcp.Implementation
	server          *mcp.Server
	capabilities    mcp.ServerCapabilities
	toolDefinitions map[string]*mcp.Tool
	toolOrder       []string

	ctx    context.Context
	cancel context.CancelFunc
	start  sync.Once
}

// NewMCPServer constructs a go-sdk backed MCP server that reuses the shared LSP manager.
func NewMCPServer(cfg *config.Config) (*MCPServer, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration required for MCP server")
	}

	// Cache configuration already applied by config.Load() with ModeMCPServer
	lspManager, err := NewLSPManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create LSP manager: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	impl := &mcp.Implementation{
		Name:    "lsp-gateway",
		Title:   "LSP Gateway MCP Server",
		Version: versionpkg.GetVersion(),
	}

	s := &MCPServer{
		cfg:             cfg,
		lspManager:      lspManager,
		symbolService:   newDefaultSymbolService(lspManager),
		impl:            impl,
		toolDefinitions: make(map[string]*mcp.Tool),
		ctx:             ctx,
		cancel:          cancel,
	}

	s.server = mcp.NewServer(impl, &mcp.ServerOptions{HasTools: true})

	if err := s.registerTools(); err != nil {
		return nil, err
	}

	s.capabilities = mcp.ServerCapabilities{
		Logging: &mcp.LoggingCapabilities{},
		Tools:   &mcp.ToolCapabilities{ListChanged: true},
	}

	return s, nil
}

// Start initialises the LSP manager and background indexing routines.
func (s *MCPServer) Start() error {
	var startErr error
	s.start.Do(func() {
		if err := s.lspManager.Start(s.ctx); err != nil {
			startErr = fmt.Errorf("failed to start LSP manager: %w", err)
			return
		}

		cache := s.lspManager.GetCache()
		if cache == nil {
			common.LSPLogger.Warn("MCP server: Starting without SCIP cache (cache unavailable)")
			return
		}

		stats := cache.GetIndexStats()
		if stats != nil && (stats.SymbolCount > 0 || stats.ReferenceCount > 0 || stats.DocumentCount > 0) {
			common.LSPLogger.Debug("MCP server: Using existing cache with %d symbols, %d references, %d documents",
				stats.SymbolCount, stats.ReferenceCount, stats.DocumentCount)
			return
		}

		go s.performInitialIndexing()
	})
	return startErr
}

// Stop cancels the MCP server context and shuts down the shared LSP manager.
func (s *MCPServer) Stop() error {
	s.cancel()
	if err := s.lspManager.Stop(); err != nil {
		common.LSPLogger.Error("Error stopping LSP manager: %v", err)
		return err
	}
	return nil
}

// Run starts the MCP server using the provided transport. When transport is nil, a StdioTransport is used.
func (s *MCPServer) Run(ctx context.Context, transport mcp.Transport) error {
	if ctx == nil {
		ctx = s.ctx
	}

	if err := s.Start(); err != nil {
		return err
	}
	defer func() { _ = s.Stop() }()

	if transport == nil {
		transport = &mcp.StdioTransport{}
	}

	return s.server.Run(ctx, transport)
}

// Implementation exposes the underlying go-sdk implementation descriptor.
func (s *MCPServer) Implementation() *mcp.Implementation {
	return s.impl
}

// GoSDKServer returns the underlying go-sdk server instance.
func (s *MCPServer) GoSDKServer() *mcp.Server {
	return s.server
}

// Capabilities reports the currently advertised MCP capabilities.
func (s *MCPServer) Capabilities() mcp.ServerCapabilities {
	return s.capabilities
}

// RegisteredToolNames returns a copy of the tool registration order.
func (s *MCPServer) RegisteredToolNames() []string {
	names := make([]string, len(s.toolOrder))
	copy(names, s.toolOrder)
	return names
}

// ToolDefinition returns the go-sdk tool definition for the supplied name.
func (s *MCPServer) ToolDefinition(name string) *mcp.Tool {
	return s.toolDefinitions[name]
}

// RunMCPServer launches the stdio server using process stdio handles.
func RunMCPServer(configPath string) error {
	cfg, err := config.Load(configPath, config.ModeMCPServer)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	server, err := NewMCPServer(cfg)
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	return server.Run(context.Background(), &mcp.StdioTransport{})
}
