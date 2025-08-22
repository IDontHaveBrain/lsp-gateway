package server

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/constants"
	errorspkg "lsp-gateway/src/internal/errors"
	"lsp-gateway/src/internal/project"
	"lsp-gateway/src/internal/security"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/aggregators"
	"lsp-gateway/src/server/aggregators/base"
	"lsp-gateway/src/server/cache"
	"lsp-gateway/src/server/documents"
	"lsp-gateway/src/server/errors"
	"lsp-gateway/src/server/watcher"
	"lsp-gateway/src/utils"
	"strings"
)

// ClientStatus represents the status of an LSP client
type ClientStatus struct {
	Active    bool
	Error     error
	Available bool // Whether the server command is available on system
}

// serverConfigWrapper wraps server config to implement LSPClient interface for aggregator compatibility
type serverConfigWrapper struct {
	language string
	config   *config.ServerConfig
}

func (w *serverConfigWrapper) Start(ctx context.Context) error { return nil }
func (w *serverConfigWrapper) Stop() error                     { return nil }
func (w *serverConfigWrapper) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	return nil, nil
}
func (w *serverConfigWrapper) SendNotification(ctx context.Context, method string, params interface{}) error {
	return nil
}
func (w *serverConfigWrapper) Supports(method string) bool { return false }
func (w *serverConfigWrapper) IsActive() bool              { return false }

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

	// Optional SCIP cache integration - managed through CacheIntegrator
	cacheIntegrator *cache.CacheIntegrator
	scipCache       cache.SCIPCache // Convenience accessor, gets cache from integrator

	// Project information for consistent symbol ID generation
	projectInfo *project.PackageInfo

	// File watcher for real-time change detection
	fileWatcher *watcher.FileWatcher
	watcherMu   sync.Mutex

	// hoverMemo removed - hover not needed during indexing

	// Limit concurrent indexing tasks to avoid request-path blocking
	indexLimiter chan struct{}
}

// NewLSPManager creates a new LSP manager with unified cache configuration
func NewLSPManager(cfg *config.Config) (*LSPManager, error) {
	if cfg == nil {
		cfg = config.GetDefaultConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create cache integrator for unified cache management
	cacheIntegrator := cache.NewCacheIntegrator(cfg, common.LSPLogger)

	manager := &LSPManager{
		clients:             make(map[string]types.LSPClient),
		clientErrors:        make(map[string]error),
		config:              cfg,
		ctx:                 ctx,
		cancel:              cancel,
		documentManager:     documents.NewLSPDocumentManager(),
		workspaceAggregator: aggregators.NewWorkspaceSymbolAggregator(),
		cacheIntegrator:     cacheIntegrator,
		scipCache:           cacheIntegrator.GetCache(), // Set convenience accessor
		projectInfo:         nil,                        // Will be initialized when needed
	}

	// Initialize indexing concurrency limiter (tighter on Windows)
	limiterSize := 2
	if runtime.GOOS == "windows" {
		limiterSize = 1
	}
	manager.indexLimiter = make(chan struct{}, limiterSize)

	// Initialize project info for consistent symbol ID generation
	if wd, err := os.Getwd(); err == nil {
		// Try to determine the primary language by looking at project files
		language := manager.detectPrimaryLanguage(wd)
		if projectInfo, err := project.GetPackageInfo(wd, language); err == nil {
			manager.projectInfo = projectInfo
		} else {
			// Fallback to basic project info
			manager.projectInfo = &project.PackageInfo{
				Name:     filepath.Base(wd),
				Version:  "0.0.0",
				Language: language,
			}
		}
	}

	return manager, nil
}

// Start initializes and starts all configured LSP clients
func (m *LSPManager) Start(ctx context.Context) error {
	common.LSPLogger.Debug("[LSPManager.Start] Starting LSP manager, scipCache=%v", m.cacheIntegrator.IsEnabled())

	// Start cache through integrator - handles graceful degradation internally
	if err := m.cacheIntegrator.StartCache(ctx); err != nil {
		return fmt.Errorf("unexpected cache start error: %w", err)
	}
	// Update convenience accessor after potential cache disabling
	m.scipCache = m.cacheIntegrator.GetCache()

	// Detect workspace languages and filter servers to start
	// Only start LSP servers for languages actually present in the current working directory
	languagesToStart := make([]string, 0)
	serversToStart := make(map[string]*config.ServerConfig)

	if wd, err := os.Getwd(); err == nil {
		if detected, derr := project.DetectLanguages(wd); derr == nil && len(detected) > 0 {
			detectedSet := make(map[string]bool, len(detected))
			for _, lang := range detected {
				detectedSet[lang] = true
			}
			for lang, cfg := range m.config.Servers {
				if detectedSet[lang] {
					serversToStart[lang] = cfg
					languagesToStart = append(languagesToStart, lang)
				}
			}
			common.LSPLogger.Info("Detected workspace languages: %v", detected)
			common.LSPLogger.Info("Starting LSP servers for detected languages only: %v", languagesToStart)
		} else {
			// No languages detected; do not start any servers
			common.LSPLogger.Warn("No languages detected in workspace; skipping LSP server startup")
		}
	} else {
		// Unable to get working directory; conservative: skip starting servers
		common.LSPLogger.Warn("Failed to get working directory; skipping LSP server startup")
	}

	// If nothing to start, return after initializing cache and watchers as appropriate
	if len(serversToStart) == 0 {
		// Start file watcher for real-time change detection (still useful for cache indexing triggers)
		if m.config.Cache != nil && m.config.Cache.BackgroundIndex {
			if err := m.startFileWatcher(); err != nil {
				common.LSPLogger.Warn("Failed to start file watcher: %v", err)
			}
		}
		return nil
	}

	// Start clients using ParallelAggregator framework with language-specific timeouts
	// Create timeout manager for initialize operations
	timeoutMgr := base.NewTimeoutManager().ForOperation(base.OperationInitialize)

	// Calculate overall timeout as maximum of all language timeouts for filtered set
	overallTimeout := timeoutMgr.GetOverallTimeout(languagesToStart)

	common.LSPLogger.Debug("[LSPManager.Start] Using overall collection timeout of %v for %d servers", overallTimeout, len(serversToStart))

	// Create aggregator with calculated overall timeout
	aggregator := base.NewParallelAggregator[*config.ServerConfig, error](0, overallTimeout)

	// Convert server configs to clients map for framework compatibility
	serverConfigs := make(map[string]types.LSPClient)
	for lang, cfg := range serversToStart {
		// Use a placeholder client that holds the server config
		serverConfigs[lang] = &serverConfigWrapper{language: lang, config: cfg}
	}

	// Create executor function that calls startClientWithTimeout
	executor := func(ctx context.Context, client types.LSPClient, _ *config.ServerConfig) (error, error) {
		// Extract language and config from wrapper
		wrapper := client.(*serverConfigWrapper)
		err := m.startClientWithTimeout(ctx, wrapper.language, wrapper.config)
		return err, err
	}

	// Execute parallel client startup with language-specific timeouts
	_, errors := aggregator.ExecuteWithLanguageTimeouts(ctx, serverConfigs, nil, executor, timeoutMgr.GetTimeout)

	// Process results and populate clientErrors map to preserve exact behavior
	completed := len(serversToStart) - len(errors)

	// Map framework errors back to the clientErrors structure
	m.mu.Lock()
	for _, err := range errors {
		// Extract language from error string format "language: error"
		errStr := err.Error()
		if colonIndex := strings.Index(errStr, ": "); colonIndex > 0 {
			language := errStr[:colonIndex]
			actualErr := fmt.Errorf("%s", errStr[colonIndex+2:])
			m.clientErrors[language] = actualErr
			common.LSPLogger.Error("Failed to start %s client: %v", language, actualErr)
		}
	}
	m.mu.Unlock()

	// Handle timeout/cancellation cases exactly as before
	if len(errors) > 0 {
		// Check for timeout by examining error messages
		for _, err := range errors {
			errStr := err.Error()
			if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "Overall timeout reached") {
				common.LSPLogger.Warn("Timeout reached, %d/%d clients started", completed, len(serversToStart))
				return nil
			}
		}
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		common.LSPLogger.Warn("Context cancelled, %d/%d clients started", completed, len(serversToStart))
		return nil
	default:
		// Context not cancelled, continue
	}

	// Perform workspace indexing if cache is enabled and background indexing is configured
	if m.scipCache != nil && m.config.Cache != nil && m.config.Cache.BackgroundIndex {
		// Check immediately if cache already has data (loads synchronously)
		if cacheManager, ok := m.scipCache.(*cache.SCIPCacheManager); ok {
			stats := cacheManager.GetIndexStats()
			if stats != nil && (stats.SymbolCount > 0 || stats.ReferenceCount > 0 || stats.DocumentCount > 0) {
				common.LSPLogger.Debug("LSP Manager: Cache already populated with %d symbols, %d references, %d documents - skipping background indexing",
					stats.SymbolCount, stats.ReferenceCount, stats.DocumentCount)
			} else {
				// Only index if cache is truly empty
				go func() {
					// Wait for LSP servers to fully initialize
					time.Sleep(constants.GetBackgroundIndexingDelay())

					// Double-check cache status
					recheckStats := cacheManager.GetIndexStats()
					if recheckStats != nil && (recheckStats.SymbolCount > 0 || recheckStats.ReferenceCount > 0 || recheckStats.DocumentCount > 0) {
						common.LSPLogger.Debug("LSP Manager: Cache was populated while waiting - skipping background indexing")
						return
					}

					// Get working directory
					wd, err := os.Getwd()
					if err != nil {
						common.LSPLogger.Warn("Failed to get working directory for indexing: %v", err)
						return
					}

					// Perform workspace indexing
					common.LSPLogger.Debug("LSP Manager: Performing background workspace indexing")
					indexCtx, cancel := common.CreateContext(5 * time.Minute)
					defer cancel()

					if err := cacheManager.PerformWorkspaceIndexing(indexCtx, wd, m); err != nil {
						common.LSPLogger.Warn("Failed to perform workspace indexing: %v", err)
					}
				}()
			}
		}
	}

	// Start file watcher for real-time change detection
	if m.config.Cache != nil && m.config.Cache.BackgroundIndex {
		if err := m.startFileWatcher(); err != nil {
			common.LSPLogger.Warn("Failed to start file watcher: %v", err)
		}
	}

	return nil
}

// Stop stops all LSP clients
func (m *LSPManager) Stop() error {
	m.cancel()

	// Stop file watcher
	if m.fileWatcher != nil {
		if err := m.fileWatcher.Stop(); err != nil {
			common.LSPLogger.Warn("Failed to stop file watcher: %v", err)
		}
	}

	// Stop cache through integrator
	if err := m.cacheIntegrator.StopCache(); err != nil {
		common.LSPLogger.Warn("Failed to stop SCIP cache: %v", err)
	}

	m.mu.Lock()
	clients := make(map[string]types.LSPClient)
	for k, v := range m.clients {
		clients[k] = v
	}
	m.clients = make(map[string]types.LSPClient)
	m.mu.Unlock()

	// If no clients to stop, return successfully
	if len(clients) == 0 {
		return nil
	}

	// Stop clients in parallel using ParallelAggregator framework
	// Use more generous timeouts for shutdown to allow LSP servers to exit gracefully
	individualTimeout := constants.ProcessShutdownTimeout * 3 // 15 seconds per client
	overallTimeout := individualTimeout + 5*time.Second       // 20 seconds overall

	aggregator := base.NewParallelAggregator[struct{}, error](individualTimeout, overallTimeout)

	ctx := context.Background()
	results, errors := aggregator.Execute(ctx, clients, struct{}{}, func(ctx context.Context, client types.LSPClient, _ struct{}) (error, error) {
		err := client.Stop()
		return err, err
	})

	// Check for timeout - if we got fewer responses than clients, it likely timed out
	totalResponses := len(results) + len(errors)
	if totalResponses < len(clients) {
		common.LSPLogger.Warn("Timeout stopping LSP clients, %d/%d completed", totalResponses, len(clients))
		return fmt.Errorf("timeout stopping LSP clients")
	}

	// Return last error to preserve original behavior
	if len(errors) > 0 {
		return errors[len(errors)-1]
	}

	return nil
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

	result, err := m.sendRequestWithRetry(ctx, client, method, params, uri, language)

	// Normalize LSP results for better client/test compatibility
	if err == nil {
		// For textDocument/references, always return an array (not null)
		if method == types.MethodTextDocumentReferences {
			if len(result) == 0 || strings.TrimSpace(string(result)) == "null" {
				result = json.RawMessage("[]")
			}
		}
	}

	// Cache the result and perform/schedule SCIP indexing if successful
	if err == nil && m.scipCache != nil && m.isCacheableMethod(method) {
		m.scipCache.Store(method, params, result)

		// For document symbols, index synchronously to ensure workspace indexing correctness
		if method == types.MethodTextDocumentDocumentSymbol {
			timeout := constants.AdjustDurationForWindows(12*time.Second, 1.5)
			idxCtx, cancel := common.CreateContext(timeout)
			defer cancel()
			m.performSCIPIndexing(idxCtx, method, uri, language, params, result)
		} else {
			// Schedule non-blocking indexing for other methods
			m.scheduleIndexing(method, uri, language, params, result)
		}
	}

	return result, err
}

// scheduleIndexing runs performSCIPIndexing asynchronously with bounded concurrency and timeouts
func (m *LSPManager) scheduleIndexing(method, uri, language string, params, result interface{}) {
	// Always enqueue work in a goroutine; limiter bounds concurrency without dropping tasks
	go func() {
		// Acquire slot (blocks until available to avoid silent drops)
		m.indexLimiter <- struct{}{}
		defer func() { <-m.indexLimiter }()

		// Derive timeout per method
		timeout := 10 * time.Second
		switch method {
		case types.MethodWorkspaceSymbol:
			timeout = 20 * time.Second
		case types.MethodTextDocumentReferences:
			timeout = 15 * time.Second
		case types.MethodTextDocumentDocumentSymbol:
			timeout = 12 * time.Second
		case types.MethodTextDocumentDefinition, types.MethodTextDocumentCompletion, types.MethodTextDocumentHover:
			timeout = 10 * time.Second
		}
		timeout = constants.AdjustDurationForWindows(timeout, 1.5)

		idxCtx, cancel := common.CreateContext(timeout)
		defer cancel()
		m.performSCIPIndexing(idxCtx, method, uri, language, params, result)
	}()
}

// sendRequestWithRetry sends a request and retries with exponential backoff on transient "no views" errors (common on Windows/gopls)
func (m *LSPManager) sendRequestWithRetry(ctx context.Context, client types.LSPClient, method string, params interface{}, uri string, language string) (json.RawMessage, error) {
	maxRetries := 3
	baseDelay := 200 * time.Millisecond
	if runtime.GOOS == "windows" {
		baseDelay = 500 * time.Millisecond
		maxRetries = 4 // Extra retry on Windows due to slower view establishment
	}

	var lastRes json.RawMessage
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		res, err := client.SendRequest(ctx, method, params)
		lastRes = res
		lastErr = err

		if err != nil {
			return res, err
		}

		// Retry on transient content-modified errors commonly returned by some servers (e.g., OmniSharp)
		if isContentModifiedRPCError(res) {
			if uri == "" || attempt == maxRetries-1 {
				return res, nil
			}
			delay := time.Duration(attempt+1) * baseDelay
			time.Sleep(delay)
			// Re-ensure document open to stabilize
			m.ensureDocumentOpen(client, uri, params)
			continue
		}

		if !isNoViewsRPCError(res) {
			// Success - no "no views" error
			return res, nil
		}

		// Don't retry if no URI provided or this is the last attempt
		if uri == "" || attempt == maxRetries-1 {
			if attempt == maxRetries-1 && uri != "" {
				common.LSPLogger.Warn("'no views' error persisted after %d retries for %s (language: %s, method: %s)",
					maxRetries, uri, language, method)
			}
			return res, nil
		}

		// Log retry attempt
		if attempt == 0 {
			common.LSPLogger.Debug("Encountered 'no views' error for %s, retrying with exponential backoff...", uri)
		}

		// Re-establish workspace folder and document
		m.ensureDocumentOpen(client, uri, params)

		// Exponential backoff with jitter
		delay := time.Duration(attempt+1) * baseDelay
		// Add small jitter to prevent synchronized retries
		jitter := time.Duration(rand.Intn(100)) * time.Millisecond
		time.Sleep(delay + jitter)
	}

	return lastRes, lastErr
}

// isNoViewsRPCError detects a JSON-RPC error payload with "no views" message
func isNoViewsRPCError(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var e struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		return false
	}
	if e.Message == "" {
		return false
	}
	return strings.Contains(strings.ToLower(e.Message), "no views")
}

// isContentModifiedRPCError detects a JSON-RPC error payload with Content Modified (-32801)
func isContentModifiedRPCError(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var e struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		return false
	}
	if e.Message == "" {
		return false
	}
	if e.Code == -32801 {
		return true
	}
	return strings.Contains(strings.ToLower(e.Message), "content modified")
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
	// Calculate max wait time based on language initialization timeout
	maxWaitTime := constants.GetInitializeTimeout(language)

	// Convert to iterations (100ms per iteration)
	maxIterations := int(maxWaitTime.Seconds() * 10) // 100ms = 0.1s, so 10 iterations per second

	// Ensure minimum wait time of 3 seconds
	if maxIterations < 30 {
		maxIterations = 30
	}

	return maxIterations
}

// resolveCommandPath resolves the command path, checking custom installations first
func (m *LSPManager) resolveCommandPath(language, command string) string {
    // Prefer custom installation under ~/.lsp-gateway/tools/{language}
    if root := common.GetLSPToolRoot(language); root != "" {
        if resolved := common.FirstExistingExecutable(root, []string{command}); resolved != "" {
            common.LSPLogger.Debug("Using installed %s command at %s", language, resolved)
            return resolved
        }
    }

    // For Java, check if custom installation exists
    if language == "java" && command == "jdtls" {
        var customPath string
        if runtime.GOOS == "windows" {
            customPath = common.GetLSPToolPath("java", "jdtls.bat")
		} else {
			customPath = common.GetLSPToolPath("java", "jdtls")
		}

		// Check if the custom installation exists
		if common.FileExists(customPath) {
			common.LSPLogger.Debug("Using custom jdtls installation at %s", customPath)
			return customPath
		}
	}

	// Windows: if a full path to jdtls is provided without extension, prefer .bat/.cmd
	if language == "java" && runtime.GOOS == "windows" {
		lower := strings.ToLower(command)
		if strings.HasSuffix(lower, "\\jdtls") || strings.HasSuffix(lower, "/jdtls") || lower == "jdtls" {
			if common.FileExists(command + ".bat") {
				return command + ".bat"
			}
			if common.FileExists(command + ".cmd") {
				return command + ".cmd"
			}
		}
	}

    // Check for other language custom installations
    if command == "gopls" || command == "pylsp" || command == "jedi-language-server" || command == "pyright-langserver" || command == "basedpyright-langserver" || command == "typescript-language-server" || command == "omnisharp" || command == "OmniSharp" || command == "kotlin-lsp" {
        // Map of commands to their languages for path construction
        languageMap := map[string]string{
            "gopls":                      "go",
            "pylsp":                      "python",
            "jedi-language-server":       "python",
            "pyright-langserver":         "python",
            "basedpyright-langserver":    "python",
            "typescript-language-server": "typescript",
            "omnisharp":                  "csharp",
            "OmniSharp":                  "csharp",
            "kotlin-lsp":                 "kotlin",
        }
        if lang, exists := languageMap[command]; exists {
            customPath := common.GetLSPToolPath(lang, command)
            if runtime.GOOS == "windows" {
                // Add extension for platform-specific shims
                if command == "typescript-language-server" || command == "pylsp" || command == "jedi-language-server" {
                    customPath = customPath + ".cmd"
                } else if command == "omnisharp" || command == "OmniSharp" {
                    customPath = customPath + ".exe"
                } else if command == "kotlin-lsp" {
                    // Prefer native .exe on Windows for reliable stdio
                    exePath := common.GetLSPToolPath("kotlin", "kotlin-lsp.exe")
                    if common.FileExists(exePath) {
                        customPath = exePath
                    } else {
                        // Fall back to .cmd if no .exe present
                        customPath = customPath + ".cmd"
                    }
                }
            }

            // Check if the custom installation exists
            if common.FileExists(customPath) {
                common.LSPLogger.Debug("Using custom %s installation at %s", command, customPath)
                return customPath
            }
        }
    }

	// Return the original command (will be resolved via PATH)
	return command
}

// startClientWithTimeout starts a single LSP client with timeout
func (m *LSPManager) startClientWithTimeout(ctx context.Context, language string, cfg *config.ServerConfig) error {
	resolvedCommand := m.resolveCommandPath(language, cfg.Command)

	argsToUse := cfg.Args

	// Validate initial command/args
	if err := security.ValidateCommand(resolvedCommand, argsToUse); err != nil {
		return fmt.Errorf("invalid LSP server command for %s: %w", language, err)
	}

	// Ensure executable exists; if not, try language-specific fallbacks
	if _, err := exec.LookPath(resolvedCommand); err != nil {
		// Python: attempt alternative servers if the configured one is missing
		if language == "python" {
			type candidate struct {
				cmd  string
				args []string
			}
			// Build candidate list, de-duplicated, preferring modern servers
			seen := map[string]bool{}
			candidates := []candidate{}
			add := func(cmd string, args []string) {
				if cmd == "" || seen[cmd] {
					return
				}
				seen[cmd] = true
				candidates = append(candidates, candidate{cmd: cmd, args: args})
			}
			add(cfg.Command, cfg.Args)
			add("basedpyright-langserver", []string{"--stdio"})
			add("pyright-langserver", []string{"--stdio"})
			add("pylsp", []string{})
			add("jedi-language-server", []string{})

			found := false
			for _, c := range candidates {
				rc := m.resolveCommandPath(language, c.cmd)
				// Re-validate command with candidate args
				if vErr := security.ValidateCommand(rc, c.args); vErr != nil {
					continue
				}
				if _, lErr := exec.LookPath(rc); lErr == nil {
					resolvedCommand = rc
					argsToUse = c.args
					found = true
					common.LSPLogger.Info("Using fallback Python LSP server: %s", rc)
					break
				}
			}
			if !found {
				return fmt.Errorf("LSP server executable not found for %s: %s", language, resolvedCommand)
			}
		} else {
			return fmt.Errorf("LSP server executable not found for %s: %s", language, resolvedCommand)
		}
	}

	clientConfig := types.ClientConfig{
		Command:               resolvedCommand,
		Args:                  argsToUse,
		WorkingDir:            cfg.WorkingDir,
		InitializationOptions: cfg.InitializationOptions,
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

// detectPrimaryLanguage detects the primary language of a project directory
func (m *LSPManager) detectPrimaryLanguage(workingDir string) string {
	// Check for project marker files in order of preference
	projectMarkers := []struct {
		files    []string
		language string
	}{
		{[]string{"go.mod", "go.work"}, "go"},
		{[]string{"package.json", "tsconfig.json"}, "typescript"},
		{[]string{"package.json"}, "javascript"},
		{[]string{"pyproject.toml", "setup.py", "requirements.txt"}, "python"},
		{[]string{"pom.xml", "build.gradle", "build.gradle.kts"}, "java"},
		{[]string{"Cargo.toml", "Cargo.lock"}, "rust"},
	}

	for _, marker := range projectMarkers {
		for _, file := range marker.files {
			if common.FileExists(filepath.Join(workingDir, file)) {
				return marker.language
			}
		}
	}

	// Fallback: look at file extensions in directory
	if files, err := os.ReadDir(workingDir); err == nil {
		langCounts := make(map[string]int)
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			ext := filepath.Ext(file.Name())
			switch ext {
			case ".go":
				langCounts["go"]++
			case ".py":
				langCounts["python"]++
			case ".js", ".mjs":
				langCounts["javascript"]++
			case ".ts":
				langCounts["typescript"]++
			case ".java":
				langCounts["java"]++
			case ".rs":
				langCounts["rust"]++
			}
		}

		// Return the language with the most files
		maxCount := 0
		var primaryLang string
		for lang, count := range langCounts {
			if count > maxCount {
				maxCount = count
				primaryLang = lang
			}
		}
		if primaryLang != "" {
			return primaryLang
		}
	}

	// Default fallback
	return "unknown"
}

// startFileWatcher initializes and starts the file watcher
func (m *LSPManager) startFileWatcher() error {
	m.watcherMu.Lock()
	defer m.watcherMu.Unlock()

	// Get supported extensions
	extensions := constants.GetAllSupportedExtensions()

	// Create file watcher
	fw, err := watcher.NewFileWatcher(extensions, m.handleFileChanges)
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Get working directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Add working directory to watch
	if err := fw.AddPath(wd); err != nil {
		return fmt.Errorf("failed to add watch path: %w", err)
	}

	// Start watching
	fw.Start()
	m.fileWatcher = fw

	common.LSPLogger.Info("File watcher started for real-time change detection")
	return nil
}

// handleFileChanges processes file change events and triggers incremental indexing
func (m *LSPManager) handleFileChanges(events []watcher.FileChangeEvent) {
	if len(events) == 0 {
		return
	}

	// Check if cache is available
	if m.scipCache == nil {
		return
	}

	// Group events by operation
	var modifiedFiles []string
	var deletedFiles []string

	for _, event := range events {
		switch event.Operation {
		case "write", "create":
			modifiedFiles = append(modifiedFiles, event.Path)
		case "remove":
			deletedFiles = append(deletedFiles, event.Path)
		}
	}

	common.LSPLogger.Debug("File changes detected: %d modified, %d deleted",
		len(modifiedFiles), len(deletedFiles))

	// Handle deleted files
	for _, path := range deletedFiles {
		uri := utils.FilePathToURI(path)
		if err := m.scipCache.InvalidateDocument(uri); err != nil {
			common.LSPLogger.Warn("Failed to invalidate deleted document %s: %v", uri, err)
		}
	}

	// Trigger incremental indexing for modified files
	if len(modifiedFiles) > 0 {
		go m.performIncrementalReindex(modifiedFiles)
	}
}

// performIncrementalReindex performs incremental reindexing for changed files
func (m *LSPManager) performIncrementalReindex(files []string) {
	// Bound concurrency with shared indexing limiter without dropping tasks
	m.indexLimiter <- struct{}{}
	defer func() { <-m.indexLimiter }()

	// Check cache manager type
	cacheManager, ok := m.scipCache.(*cache.SCIPCacheManager)
	if !ok {
		common.LSPLogger.Warn("Cache manager doesn't support incremental indexing")
		return
	}

	ctx, cancel := common.CreateContext(2 * time.Minute)
	defer cancel()

	// Get working directory
	wd, err := os.Getwd()
	if err != nil {
		common.LSPLogger.Error("Failed to get working directory: %v", err)
		return
	}

	common.LSPLogger.Info("Performing incremental reindex for %d changed files", len(files))

	// Perform incremental indexing
	if err := cacheManager.PerformIncrementalIndexing(ctx, wd, m); err != nil {
		common.LSPLogger.Error("Incremental reindexing failed: %v", err)
	} else {
		common.LSPLogger.Info("Incremental reindex completed successfully")

		// Log updated cache stats
		if stats := cacheManager.GetIndexStats(); stats != nil {
			common.LSPLogger.Debug("Cache stats after reindex: %d symbols, %d documents",
				stats.SymbolCount, stats.DocumentCount)
		}
	}
}
