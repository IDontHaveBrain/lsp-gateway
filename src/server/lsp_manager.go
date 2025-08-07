package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/constants"
	errorspkg "lsp-gateway/src/internal/errors"
	"lsp-gateway/src/internal/security"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/aggregators"
	"lsp-gateway/src/server/cache"
	"lsp-gateway/src/server/documents"
	"lsp-gateway/src/server/errors"
)

// ClientStatus represents the status of an LSP client
type ClientStatus struct {
	Active    bool
	Error     error
	Available bool // Whether the server command is available on system
}

// LSPManager manages LSP clients for different languages
type LSPManager struct {
	clients             map[string]types.LSPClient
	clientErrors        map[string]error
	config              *config.Config
	ctx                 context.Context
	cancel              context.CancelFunc
	mu                  sync.RWMutex
	documentManager     documents.DocumentManager
	workspaceAggregator aggregators.WorkspaceSymbolAggregator

	// Optional SCIP cache integration - can be nil for simple usage
	scipCache cache.SCIPCache
}

// NewLSPManager creates a new LSP manager with unified cache configuration
func NewLSPManager(cfg *config.Config) (*LSPManager, error) {
	if cfg == nil {
		cfg = config.GetDefaultConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	manager := &LSPManager{
		clients:             make(map[string]types.LSPClient),
		clientErrors:        make(map[string]error),
		config:              cfg,
		ctx:                 ctx,
		cancel:              cancel,
		documentManager:     documents.NewLSPDocumentManager(),
		workspaceAggregator: aggregators.NewWorkspaceSymbolAggregator(),
		scipCache:           nil, // Optional cache - set to nil initially
	}

	// Try to create cache with unified config - graceful degradation if it fails
	if cfg.Cache != nil && cfg.Cache.Enabled {
		scipCache, err := cache.NewSCIPCacheManager(cfg.Cache)
		if err != nil {
			common.LSPLogger.Warn("Failed to create cache (continuing without cache): %v", err)
		} else {
			manager.scipCache = scipCache
		}
	}

	return manager, nil
}

// Start initializes and starts all configured LSP clients
func (m *LSPManager) Start(ctx context.Context) error {
	// Start cache if available - optional integration
	if m.scipCache != nil {
		if err := m.scipCache.Start(ctx); err != nil {
			common.LSPLogger.Warn("Failed to start SCIP cache (continuing without cache): %v", err)
			m.scipCache = nil // Disable cache on start failure
		}
	}

	// Start clients with individual timeouts to prevent hanging
	results := make(chan struct {
		language string
		err      error
	}, len(m.config.Servers))

	// Start each client in a separate goroutine
	for language, serverConfig := range m.config.Servers {
		go func(lang string, cfg *config.ServerConfig) {
			// Individual timeout per server
			clientCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			err := m.startClientWithTimeout(clientCtx, lang, cfg)
			results <- struct {
				language string
				err      error
			}{lang, err}
		}(language, serverConfig)
	}

	// Collect results with overall timeout
	timeout := time.After(constants.DefaultInitializeTimeout)
	completed := 0

	for completed < len(m.config.Servers) {
		select {
		case result := <-results:
			completed++
			if result.err != nil {
				common.LSPLogger.Error("Failed to start %s client: %v", result.language, result.err)
				m.mu.Lock()
				m.clientErrors[result.language] = result.err
				m.mu.Unlock()
			} else {
			}
		case <-timeout:
			common.LSPLogger.Warn("Timeout reached, %d/%d clients started", completed, len(m.config.Servers))
			return nil
		case <-ctx.Done():
			common.LSPLogger.Warn("Context cancelled, %d/%d clients started", completed, len(m.config.Servers))
			return nil
		}
	}

	// Perform workspace indexing if cache is enabled and background indexing is configured
	if m.scipCache != nil && m.config.Cache != nil && m.config.Cache.BackgroundIndex {
		go func() {
			// Wait a bit for LSP servers to fully initialize
			time.Sleep(2 * time.Second)

			// Get working directory
			wd, err := os.Getwd()
			if err != nil {
				common.LSPLogger.Warn("Failed to get working directory for indexing: %v", err)
				return
			}

			// Perform workspace indexing
			indexCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			if cacheManager, ok := m.scipCache.(*cache.SimpleCacheManager); ok {
				if err := cacheManager.PerformWorkspaceIndexing(indexCtx, wd, m); err != nil {
					common.LSPLogger.Warn("Failed to perform workspace indexing: %v", err)
				} else {
				}
			}
		}()
	}

	return nil
}

// Stop stops all LSP clients
func (m *LSPManager) Stop() error {
	m.cancel()

	// Stop cache if available - optional integration
	if m.scipCache != nil {
		if err := m.scipCache.Stop(); err != nil {
			common.LSPLogger.Warn("Failed to stop SCIP cache: %v", err)
		} else {
		}
	}

	m.mu.Lock()
	clients := make(map[string]types.LSPClient)
	for k, v := range m.clients {
		clients[k] = v
	}
	m.clients = make(map[string]types.LSPClient)
	m.mu.Unlock()

	// Stop clients in parallel for faster shutdown
	done := make(chan error, len(clients))
	for language, client := range clients {
		go func(lang string, c types.LSPClient) {
			err := c.Stop()
			if err != nil {
				common.LSPLogger.Error("Error stopping %s client: %v", lang, err)
			}
			done <- err
		}(language, client)
	}

	// Wait for all clients to stop with timeout (allow for graceful shutdown + buffer)
	timeout := time.After(constants.ProcessShutdownTimeout + 1*time.Second)
	completed := 0
	var lastErr error

	for completed < len(clients) {
		select {
		case err := <-done:
			completed++
			if err != nil {
				lastErr = err
			}
		case <-timeout:
			common.LSPLogger.Warn("Timeout stopping LSP clients, %d/%d completed", completed, len(clients))
			return fmt.Errorf("timeout stopping LSP clients")
		}
	}

	return lastErr
}

// CheckServerAvailability checks if LSP server commands are available without starting them
func (m *LSPManager) CheckServerAvailability() map[string]ClientStatus {
	status := make(map[string]ClientStatus)

	for language, serverConfig := range m.config.Servers {
		// Validate command using security module
		if err := security.ValidateCommand(serverConfig.Command, serverConfig.Args); err != nil {
			status[language] = ClientStatus{
				Active:    false,
				Available: false,
				Error:     fmt.Errorf("invalid command: %w", err),
			}
			continue
		}

		// Check if command exists in PATH
		if _, err := exec.LookPath(serverConfig.Command); err != nil {
			status[language] = ClientStatus{
				Active:    false,
				Available: false,
				Error:     fmt.Errorf("command not found: %s", serverConfig.Command),
			}
			continue
		}

		// Command is available but not running
		status[language] = ClientStatus{
			Active:    false,
			Available: true,
			Error:     nil,
		}
	}

	return status
}

// ProcessRequest processes a JSON-RPC request by routing it to the appropriate LSP client
func (m *LSPManager) ProcessRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	// Try cache lookup first if cache is available and method is cacheable
	if m.scipCache != nil && m.isCacheableMethod(method) {
		if result, found, err := m.scipCache.Lookup(method, params); err == nil && found {
			return result, nil
		} else if err != nil {
			// Cache lookup failed, continue with LSP fallback
		}
	}

	// Extract file URI from params to determine language
	uri, err := m.documentManager.ExtractURI(params)
	if err != nil {
		// For methods that don't require URI (like workspace/symbol), try all clients
		if method == types.MethodWorkspaceSymbol {
			m.mu.RLock()
			clients := make(map[string]interface{})
			for k, v := range m.clients {
				clients[k] = v
			}
			m.mu.RUnlock()
			result, err := m.workspaceAggregator.ProcessWorkspaceSymbol(ctx, clients, params)

			// Index and cache the result if successful
			if err == nil && m.scipCache != nil {
				// Index the workspace symbols for each language
				// TODO: Implement indexWorkspaceSymbols method if needed
				// for lang := range clients {
				//	m.indexWorkspaceSymbols(ctx, lang, result)
				// }

				// Cache the result if cacheable
				if m.isCacheableMethod(method) {
					if cacheErr := m.scipCache.Store(method, params, result); cacheErr != nil {
						// Cache store failed
					} else {
						// Cache stored successfully
					}
				}
			}

			return result, err
		}
		return nil, fmt.Errorf("failed to extract URI from params: %w", err)
	}

	language := m.documentManager.DetectLanguage(uri)
	if language == "" {
		return nil, fmt.Errorf("unsupported file type: %s", uri)
	}

	client, err := m.getClient(language)
	if err != nil {
		return nil, fmt.Errorf("no LSP client for language %s: %w", language, err)
	}

	// Check if server supports the requested method
	if !client.Supports(method) {
		errorTranslator := errors.NewLSPErrorTranslator()
		return nil, errorspkg.NewMethodNotSupportedError(
			language,
			method,
			errorTranslator.GetMethodSuggestion(language, method),
		)
	}

	// Send textDocument/didOpen notification if needed for methods that require opened documents
	// Only workspace/symbol doesn't need a specific document open
	needsDidOpen := method != types.MethodWorkspaceSymbol
	if needsDidOpen {
		m.ensureDocumentOpen(client, uri, params)
	}

	// Send request to LSP server
	result, err := client.SendRequest(ctx, method, params)

	// Cache the result and perform SCIP indexing if successful and cache is available
	if err == nil && m.scipCache != nil && m.isCacheableMethod(method) {
		m.scipCache.Store(method, params, result)

		// Perform SCIP indexing for document-related operations
		m.performSCIPIndexing(ctx, method, uri, language, params, result)
	} else {
	}

	return result, err
}

// GetClientStatus returns the status of all LSP clients
func (m *LSPManager) GetClientStatus() map[string]ClientStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]ClientStatus)

	// Add all configured servers to status
	for language := range m.config.Servers {
		if client, exists := m.clients[language]; exists {
			// Client exists, check if it's active
			if activeClient, ok := client.(interface{ IsActive() bool }); ok {
				status[language] = ClientStatus{
					Active:    activeClient.IsActive(),
					Available: true, // If client exists, command was available
					Error:     nil,
				}
			} else {
				status[language] = ClientStatus{
					Active:    true, // Assume active if we can't check
					Available: true,
					Error:     nil,
				}
			}
		} else {
			// Client doesn't exist, check if there's an error
			if err, hasError := m.clientErrors[language]; hasError {
				status[language] = ClientStatus{
					Active:    false,
					Available: false,
					Error:     err,
				}
			} else {
				status[language] = ClientStatus{
					Active:    false,
					Available: false,
					Error:     fmt.Errorf("client not started"),
				}
			}
		}
	}

	return status
}

// getClientActiveWaitIterations returns language-specific wait iterations for client to become active
func (m *LSPManager) getClientActiveWaitIterations(language string) int {
	switch language {
	case "java":
		// Java LSP server needs up to 15 seconds (150 iterations * 100ms)
		return 150
	case "python":
		// Python LSP server needs moderate time (50 iterations * 100ms = 5s)
		return 50
	default:
		// Default 3 seconds (30 iterations * 100ms)
		return 30
	}
}

// startClientWithTimeout starts a single LSP client with timeout
func (m *LSPManager) startClientWithTimeout(ctx context.Context, language string, cfg *config.ServerConfig) error {
	// Validate LSP server command for security
	if err := security.ValidateCommand(cfg.Command, cfg.Args); err != nil {
		return fmt.Errorf("invalid LSP server command for %s: %w", language, err)
	}

	// Check if LSP server executable is available
	if _, err := exec.LookPath(cfg.Command); err != nil {
		return fmt.Errorf("LSP server executable not found for %s: %s", language, cfg.Command)
	}

	clientConfig := types.ClientConfig{
		Command: cfg.Command,
		Args:    cfg.Args,
	}

	client, err := NewStdioClient(clientConfig, language)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if err := client.Start(ctx); err != nil {
		return fmt.Errorf("failed to start client: %w", err)
	}

	m.mu.Lock()
	m.clients[language] = client
	m.mu.Unlock()

	// Wait for client to become active with timeout
	if activeClient, ok := client.(interface{ IsActive() bool }); ok {
		maxWaitIterations := m.getClientActiveWaitIterations(language)
		for i := 0; i < maxWaitIterations; i++ {
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled while waiting for client to become active")
			default:
				if activeClient.IsActive() {
					return nil
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
		return fmt.Errorf("client did not become active within timeout")
	}

	return nil
}

// getClient returns the LSP client for a given language
func (m *LSPManager) getClient(language string) (types.LSPClient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, exists := m.clients[language]
	if !exists {
		return nil, fmt.Errorf("no client for language: %s", language)
	}
	return client, nil
}

// GetClient returns the LSP client for a given language (public method for testing)
func (m *LSPManager) GetClient(language string) (types.LSPClient, error) {
	return m.getClient(language)
}

// GetConfiguredServers returns the map of configured servers
func (m *LSPManager) GetConfiguredServers() map[string]*config.ServerConfig {
	return m.config.Servers
}
