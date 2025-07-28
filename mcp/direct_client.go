package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/transport"
)

// DirectLSPManager provides direct LSP server connections for MCP mode
// This replaces the HTTP gateway dependency with direct transport connections
type DirectLSPManager struct {
	clients      map[string]transport.LSPClient // serverName -> LSP client
	languageMap  map[string]string              // language -> serverName
	scipIndexer  transport.SCIPIndexer          // Optional SCIP cache integration
	logger       *log.Logger
	mu           sync.RWMutex
}

// DirectLSPManagerConfig holds configuration for DirectLSPManager
type DirectLSPManagerConfig struct {
	ServerConfigs []*config.ServerConfig
	SCIPIndexer   transport.SCIPIndexer // Optional SCIP cache integration
	Logger        *log.Logger
}

// NewDirectLSPManager creates a new DirectLSPManager with the given configuration
func NewDirectLSPManager(cfg *DirectLSPManagerConfig) (*DirectLSPManager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("DirectLSPManagerConfig cannot be nil")
	}

	if len(cfg.ServerConfigs) == 0 {
		return nil, fmt.Errorf("no server configurations provided")
	}

	logger := cfg.Logger
	if logger == nil {
		logger = log.New(log.Writer(), "[DirectLSPManager] ", log.LstdFlags|log.Lshortfile)
	}

	manager := &DirectLSPManager{
		clients:     make(map[string]transport.LSPClient),
		languageMap: make(map[string]string),
		scipIndexer: cfg.SCIPIndexer,
		logger:      logger,
	}

	// Initialize LSP clients for each server configuration
	for _, serverConfig := range cfg.ServerConfigs {
		if err := serverConfig.Validate(); err != nil {
			return nil, fmt.Errorf("invalid server config for %s: %w", serverConfig.Name, err)
		}

		client, err := manager.createLSPClient(serverConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create LSP client for %s: %w", serverConfig.Name, err)
		}

		manager.clients[serverConfig.Name] = client

		// Map languages to server names for routing
		for _, language := range serverConfig.Languages {
			if existingServer, exists := manager.languageMap[language]; exists {
				manager.logger.Printf("Warning: Language %s already mapped to server %s, overriding with %s", 
					language, existingServer, serverConfig.Name)
			}
			manager.languageMap[language] = serverConfig.Name
		}
	}

	manager.logger.Printf("Initialized DirectLSPManager with %d servers supporting %d languages", 
		len(manager.clients), len(manager.languageMap))

	return manager, nil
}

// createLSPClient creates a transport.LSPClient for the given server configuration
func (dm *DirectLSPManager) createLSPClient(serverConfig *config.ServerConfig) (transport.LSPClient, error) {
	clientConfig := transport.ClientConfig{
		Command:   serverConfig.Command,
		Args:      serverConfig.Args,
		Transport: serverConfig.Transport,
	}

	// Debug log to verify Args are being passed
	dm.logger.Printf("Creating LSP client for %s with command: %s, args: %v", 
		serverConfig.Name, serverConfig.Command, serverConfig.Args)

	// Create client with SCIP integration if available
	if dm.scipIndexer != nil {
		return transport.NewLSPClientWithSCIP(clientConfig, dm.scipIndexer)
	}

	return transport.NewLSPClient(clientConfig)
}

// SendLSPRequest implements the LSPClient interface for MCP tools
// Routes requests to the appropriate LSP server based on context or falls back to first available
func (dm *DirectLSPManager) SendLSPRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	dm.logger.Printf("[DEBUG] SendLSPRequest called: method=%s, params=%+v", method, params)

	// Try to determine target server from request parameters
	serverName, err := dm.determineTargetServer(method, params)
	if err != nil {
		dm.logger.Printf("[ERROR] Failed to determine target server: %v", err)
		return nil, fmt.Errorf("failed to determine target server: %w", err)
	}

	dm.logger.Printf("[DEBUG] Determined target server: %s", serverName)

	client, exists := dm.clients[serverName]
	if !exists {
		dm.logger.Printf("[ERROR] LSP client not found for server: %s", serverName)
		return nil, fmt.Errorf("LSP client not found for server: %s", serverName)
	}

	// Forward request to the appropriate LSP server
	dm.logger.Printf("[DEBUG] Routing %s request to server: %s", method, serverName)
	result, err := client.SendRequest(ctx, method, params)
	if err != nil {
		dm.logger.Printf("[ERROR] LSP server %s returned error: %v", serverName, err)
		return nil, err
	}
	dm.logger.Printf("[DEBUG] LSP server %s returned result: %d bytes", serverName, len(result))
	return result, nil
}

// determineTargetServer determines which LSP server should handle the request
// This implementation uses a simple heuristic based on file extensions or defaults to the first server
func (dm *DirectLSPManager) determineTargetServer(method string, params interface{}) (string, error) {
	dm.logger.Printf("[DEBUG] determineTargetServer: method=%s, paramsType=%T", method, params)
	
	// Try to extract file information from parameters to determine language
	if paramsMap, ok := params.(map[string]interface{}); ok {
		dm.logger.Printf("[DEBUG] Params is map, checking for textDocument")
		if textDoc, exists := paramsMap["textDocument"]; exists {
			dm.logger.Printf("[DEBUG] Found textDocument: %+v", textDoc)
			if textDocMap, ok := textDoc.(map[string]interface{}); ok {
				if uri, exists := textDocMap["uri"]; exists {
					if uriStr, ok := uri.(string); ok {
						dm.logger.Printf("[DEBUG] Extracted URI: %s", uriStr)
						language := dm.inferLanguageFromURI(uriStr)
						dm.logger.Printf("[DEBUG] Inferred language: %s", language)
						if serverName, exists := dm.languageMap[language]; exists {
							dm.logger.Printf("[DEBUG] Found server for language %s: %s", language, serverName)
							return serverName, nil
						}
						dm.logger.Printf("[DEBUG] No server found for language: %s", language)
					}
				}
			}
		}

		// Check for direct URI parameter
		if uri, exists := paramsMap["uri"]; exists {
			if uriStr, ok := uri.(string); ok {
				dm.logger.Printf("[DEBUG] Found direct URI: %s", uriStr)
				language := dm.inferLanguageFromURI(uriStr)
				dm.logger.Printf("[DEBUG] Inferred language from direct URI: %s", language)
				if serverName, exists := dm.languageMap[language]; exists {
					dm.logger.Printf("[DEBUG] Found server for language %s: %s", language, serverName)
					return serverName, nil
				}
			}
		}
	}

	// Fallback to first available server
	for serverName := range dm.clients {
		dm.logger.Printf("Using fallback server %s for method %s", serverName, method)
		return serverName, nil
	}

	return "", fmt.Errorf("no LSP servers available")
}

// inferLanguageFromURI attempts to determine programming language from file URI
func (dm *DirectLSPManager) inferLanguageFromURI(uri string) string {
	// Basic file extension to language mapping
	extensionMap := map[string]string{
		".go":   "go",
		".py":   "python",
		".ts":   "typescript",
		".tsx":  "typescript",
		".js":   "javascript",
		".jsx":  "javascript",
		".java": "java",
		".kt":   "kotlin",
		".rs":   "rust",
		".cpp":  "cpp",
		".c":    "c",
		".cs":   "csharp",
		".php":  "php",
		".rb":   "ruby",
		".swift": "swift",
	}

	// Extract file extension from URI
	for ext, lang := range extensionMap {
		if len(uri) > len(ext) && uri[len(uri)-len(ext):] == ext {
			return lang
		}
	}

	return "unknown"
}

// Start initializes all LSP server connections
func (dm *DirectLSPManager) Start(ctx context.Context) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.logger.Printf("Starting %d LSP servers", len(dm.clients))

	for serverName, client := range dm.clients {
		if err := client.Start(ctx); err != nil {
			dm.logger.Printf("Failed to start LSP server %s: %v", serverName, err)
			// Continue starting other servers even if one fails
			continue
		}
		dm.logger.Printf("Successfully started LSP server: %s", serverName)
	}

	return nil
}

// Stop gracefully shuts down all LSP server connections
func (dm *DirectLSPManager) Stop() error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.logger.Printf("Stopping %d LSP servers", len(dm.clients))

	var lastErr error
	for serverName, client := range dm.clients {
		if err := client.Stop(); err != nil {
			dm.logger.Printf("Error stopping LSP server %s: %v", serverName, err)
			lastErr = err
		} else {
			dm.logger.Printf("Successfully stopped LSP server: %s", serverName)
		}
	}

	return lastErr
}

// IsActive checks if at least one LSP server is active
func (dm *DirectLSPManager) IsActive() bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	for _, client := range dm.clients {
		if client.IsActive() {
			return true
		}
	}
	return false
}

// GetAvailableLanguages returns the list of supported languages
func (dm *DirectLSPManager) GetAvailableLanguages() []string {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	languages := make([]string, 0, len(dm.languageMap))
	for language := range dm.languageMap {
		languages = append(languages, language)
	}
	return languages
}

// GetServerNames returns the list of configured server names
func (dm *DirectLSPManager) GetServerNames() []string {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	names := make([]string, 0, len(dm.clients))
	for name := range dm.clients {
		names = append(names, name)
	}
	return names
}

// GetServerForLanguage returns the server name for a given language
func (dm *DirectLSPManager) GetServerForLanguage(language string) (string, bool) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	serverName, exists := dm.languageMap[language]
	return serverName, exists
}