package client

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"lsp-gateway/internal/transport"
	"lsp-gateway/tests/utils/framework/config"
)

// LSPServerManager manages LSP server instances for testing
type LSPServerManager struct {
	mu      sync.RWMutex
	servers map[string]*ManagedLSPServer
	config  *config.LSPTestConfig
}

// ManagedLSPServer wraps an LSP client with additional test management features
type ManagedLSPServer struct {
	Name         string
	Language     string
	Config       *config.ServerConfig
	Client       transport.LSPClient
	WorkspaceURI string

	mu           sync.RWMutex
	initialized  bool
	starting     bool
	lastActivity time.Time
	requestCount int
	errorCount   int

	// Test-specific state
	PreWarmedUp  bool
	Capabilities *ServerCapabilities
}

// ServerCapabilities represents LSP server capabilities relevant to testing
type ServerCapabilities struct {
	DefinitionProvider      bool `json:"definitionProvider"`
	ReferencesProvider      bool `json:"referencesProvider"`
	HoverProvider           bool `json:"hoverProvider"`
	DocumentSymbolProvider  bool `json:"documentSymbolProvider"`
	WorkspaceSymbolProvider bool `json:"workspaceSymbolProvider"`

	// Additional capabilities for completeness
	CompletionProvider    *CompletionOptions    `json:"completionProvider,omitempty"`
	SignatureHelpProvider *SignatureHelpOptions `json:"signatureHelpProvider,omitempty"`

	// Raw capabilities for future extensibility
	Raw map[string]interface{} `json:"-"`
}

type CompletionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

type SignatureHelpOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

// InitializeParams represents LSP initialize request parameters
type InitializeParams struct {
	ProcessID        *int                   `json:"processId,omitempty"`
	RootPath         *string                `json:"rootPath,omitempty"`
	RootURI          *string                `json:"rootUri,omitempty"`
	Capabilities     *ClientCapabilities    `json:"capabilities"`
	InitOptions      map[string]interface{} `json:"initializationOptions,omitempty"`
	WorkspaceFolders []WorkspaceFolder      `json:"workspaceFolders,omitempty"`
}

type ClientCapabilities struct {
	TextDocument *TextDocumentClientCapabilities `json:"textDocument,omitempty"`
	Workspace    *WorkspaceClientCapabilities    `json:"workspace,omitempty"`
}

type TextDocumentClientCapabilities struct {
	Definition     *DefinitionClientCapabilities     `json:"definition,omitempty"`
	References     *ReferencesClientCapabilities     `json:"references,omitempty"`
	Hover          *HoverClientCapabilities          `json:"hover,omitempty"`
	DocumentSymbol *DocumentSymbolClientCapabilities `json:"documentSymbol,omitempty"`
}

type WorkspaceClientCapabilities struct {
	Symbol *WorkspaceSymbolClientCapabilities `json:"symbol,omitempty"`
}

type DefinitionClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	LinkSupport         bool `json:"linkSupport,omitempty"`
}

type ReferencesClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type HoverClientCapabilities struct {
	DynamicRegistration bool     `json:"dynamicRegistration,omitempty"`
	ContentFormat       []string `json:"contentFormat,omitempty"`
}

type DocumentSymbolClientCapabilities struct {
	DynamicRegistration               bool `json:"dynamicRegistration,omitempty"`
	HierarchicalDocumentSymbolSupport bool `json:"hierarchicalDocumentSymbolSupport,omitempty"`
}

type WorkspaceSymbolClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type WorkspaceFolder struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

// InitializeResult represents the response from initialize request
type InitializeResult struct {
	Capabilities *ServerCapabilities `json:"capabilities"`
	ServerInfo   *ServerInfo         `json:"serverInfo,omitempty"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// NewLSPServerManager creates a new LSP server manager
func NewLSPServerManager(config *config.LSPTestConfig) *LSPServerManager {
	return &LSPServerManager{
		servers: make(map[string]*ManagedLSPServer),
		config:  config,
	}
}

// GetOrCreateServer gets an existing server or creates a new one
func (m *LSPServerManager) GetOrCreateServer(ctx context.Context, serverName string, workspaceURI string) (*ManagedLSPServer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	serverKey := fmt.Sprintf("%s:%s", serverName, workspaceURI)

	if server, exists := m.servers[serverKey]; exists {
		if server.Client.IsActive() {
			server.lastActivity = time.Now()
			return server, nil
		}
		// Server exists but is not active, remove it
		delete(m.servers, serverKey)
	}

	// Create new server
	serverConfig, exists := m.config.Servers[serverName]
	if !exists {
		return nil, fmt.Errorf("server configuration not found: %s", serverName)
	}

	server, err := m.createServer(ctx, serverConfig, workspaceURI)
	if err != nil {
		return nil, fmt.Errorf("failed to create server %s: %w", serverName, err)
	}

	m.servers[serverKey] = server
	return server, nil
}

// createServer creates and initializes a new managed LSP server
func (m *LSPServerManager) createServer(ctx context.Context, serverConfig *config.ServerConfig, workspaceURI string) (*ManagedLSPServer, error) {
	// Create transport client config
	clientConfig := transport.ClientConfig{
		Command:   serverConfig.Command,
		Args:      serverConfig.Args,
		Transport: serverConfig.Transport,
	}

	// Create LSP client
	client, err := transport.NewLSPClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create LSP client: %w", err)
	}

	server := &ManagedLSPServer{
		Name:         serverConfig.Name,
		Language:     serverConfig.Language,
		Config:       serverConfig,
		Client:       client,
		WorkspaceURI: workspaceURI,
		lastActivity: time.Now(),
	}

	// Start the server
	if err := m.startServer(ctx, server); err != nil {
		return nil, fmt.Errorf("failed to start server: %w", err)
	}

	return server, nil
}

// startServer starts and initializes an LSP server
func (m *LSPServerManager) startServer(ctx context.Context, server *ManagedLSPServer) error {
	server.mu.Lock()
	defer server.mu.Unlock()

	if server.starting {
		return fmt.Errorf("server is already starting")
	}

	server.starting = true
	defer func() {
		server.starting = false
	}()

	// Start the LSP client with timeout
	startCtx, cancel := context.WithTimeout(ctx, server.Config.StartTimeout)
	defer cancel()

	if err := server.Client.Start(startCtx); err != nil {
		return fmt.Errorf("failed to start LSP client: %w", err)
	}

	// Initialize the server
	if err := m.initializeServer(ctx, server); err != nil {
		_ = server.Client.Stop() // Clean up on initialization failure
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	// Pre-warmup if configured
	if server.Config.PreWarmup {
		if err := m.warmupServer(ctx, server); err != nil {
			// Log warning but don't fail - warmup is optional
			// TODO: Use logger when available
		}
	}

	server.initialized = true
	return nil
}

// initializeServer sends the initialize request and processes the response
func (m *LSPServerManager) initializeServer(ctx context.Context, server *ManagedLSPServer) error {
	// Build workspace folder
	workspaceFolder := WorkspaceFolder{
		URI:  server.WorkspaceURI,
		Name: filepath.Base(server.WorkspaceURI),
	}

	// Build initialize parameters
	params := InitializeParams{
		RootURI: &server.WorkspaceURI,
		Capabilities: &ClientCapabilities{
			TextDocument: &TextDocumentClientCapabilities{
				Definition: &DefinitionClientCapabilities{
					DynamicRegistration: false,
					LinkSupport:         true,
				},
				References: &ReferencesClientCapabilities{
					DynamicRegistration: false,
				},
				Hover: &HoverClientCapabilities{
					DynamicRegistration: false,
					ContentFormat:       []string{"markdown", "plaintext"},
				},
				DocumentSymbol: &DocumentSymbolClientCapabilities{
					DynamicRegistration:               false,
					HierarchicalDocumentSymbolSupport: true,
				},
			},
			Workspace: &WorkspaceClientCapabilities{
				Symbol: &WorkspaceSymbolClientCapabilities{
					DynamicRegistration: false,
				},
			},
		},
		InitOptions:      server.Config.InitOptions,
		WorkspaceFolders: []WorkspaceFolder{workspaceFolder},
	}

	// Send initialize request
	response, err := server.Client.SendRequest(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}

	// Parse initialize result
	var result InitializeResult
	if err := json.Unmarshal(response, &result); err != nil {
		return fmt.Errorf("failed to parse initialize result: %w", err)
	}

	server.Capabilities = result.Capabilities

	// Send initialized notification
	if err := server.Client.SendNotification(ctx, "initialized", map[string]interface{}{}); err != nil {
		return fmt.Errorf("initialized notification failed: %w", err)
	}

	// Wait for initialize delay if configured
	if server.Config.InitializeDelay > 0 {
		time.Sleep(server.Config.InitializeDelay)
	}

	return nil
}

// warmupServer performs pre-warmup operations
func (m *LSPServerManager) warmupServer(ctx context.Context, server *ManagedLSPServer) error {
	// This could include operations like:
	// - Opening a dummy file
	// - Requesting workspace symbols
	// - Other operations to "warm up" the server

	server.PreWarmedUp = true
	return nil
}

// SendRequest sends a request to the managed server
func (m *ManagedLSPServer) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.initialized {
		return nil, fmt.Errorf("server not initialized")
	}

	if !m.Client.IsActive() {
		return nil, fmt.Errorf("server not active")
	}

	m.requestCount++
	m.lastActivity = time.Now()

	response, err := m.Client.SendRequest(ctx, method, params)
	if err != nil {
		m.errorCount++
		return nil, err
	}

	return response, nil
}

// SendNotification sends a notification to the managed server
func (m *ManagedLSPServer) SendNotification(ctx context.Context, method string, params interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.initialized {
		return fmt.Errorf("server not initialized")
	}

	if !m.Client.IsActive() {
		return fmt.Errorf("server not active")
	}

	m.lastActivity = time.Now()

	return m.Client.SendNotification(ctx, method, params)
}

// GetCapabilities returns the server capabilities
func (m *ManagedLSPServer) GetCapabilities() *ServerCapabilities {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Capabilities
}

// Shutdown gracefully shuts down the managed server
func (m *ManagedLSPServer) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.initialized {
		return nil
	}

	// Send shutdown request
	_, err := m.Client.SendRequest(ctx, "shutdown", nil)
	if err != nil {
		// Log error but continue with shutdown
	}

	// Send exit notification
	_ = m.Client.SendNotification(ctx, "exit", nil)

	// Stop the client
	if stopErr := m.Client.Stop(); stopErr != nil {
		if err == nil {
			err = stopErr
		}
	}

	m.initialized = false
	return err
}

// ShutdownAll shuts down all managed servers
func (m *LSPServerManager) ShutdownAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error

	for key, server := range m.servers {
		if err := server.Shutdown(ctx); err != nil {
			lastErr = err
		}
		delete(m.servers, key)
	}

	return lastErr
}
