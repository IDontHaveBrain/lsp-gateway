package pooling

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"lsp-gateway/internal/transport"
)

// Global pool manager instance
var (
	globalPoolManager *ServerPoolManager
	globalPoolMutex   sync.RWMutex
)

// InitializeGlobalPoolManager initializes the global pool manager
func InitializeGlobalPoolManager(config *PoolConfig) error {
	globalPoolMutex.Lock()
	defer globalPoolMutex.Unlock()
	
	if globalPoolManager != nil {
		return fmt.Errorf("global pool manager is already initialized")
	}
	
	globalPoolManager = NewServerPoolManager(config)
	return nil
}

// GetGlobalPoolManager returns the global pool manager
func GetGlobalPoolManager() *ServerPoolManager {
	globalPoolMutex.RLock()
	defer globalPoolMutex.RUnlock()
	
	return globalPoolManager
}

// ShutdownGlobalPoolManager shuts down the global pool manager
func ShutdownGlobalPoolManager() error {
	globalPoolMutex.Lock()
	defer globalPoolMutex.Unlock()
	
	if globalPoolManager != nil {
		err := globalPoolManager.Stop()
		globalPoolManager = nil
		return err
	}
	
	return nil
}

// PoolEnabledLSPClient wraps a pooled server to provide the standard LSPClient interface
type PoolEnabledLSPClient struct {
	language    string
	workspace   string
	server      *PooledServer
	poolManager *ServerPoolManager
	isAcquired  bool
	mu          sync.Mutex
}

// NewPoolEnabledLSPClient creates a new pool-enabled LSP client
func NewPoolEnabledLSPClient(language string, workspace string, config ClientConfig) (transport.LSPClient, error) {
	poolManager := GetGlobalPoolManager()
	
	// If no pool manager or pooling disabled, fallback to regular client
	if poolManager == nil || !poolManager.IsRunning() {
		return createFallbackClient(config)
	}
	
	client := &PoolEnabledLSPClient{
		language:    language,
		workspace:   workspace,
		poolManager: poolManager,
		isAcquired:  false,
	}
	
	return client, nil
}

// Start allocates a server from the pool and starts it
func (pec *PoolEnabledLSPClient) Start(ctx context.Context) error {
	pec.mu.Lock()
	defer pec.mu.Unlock()
	
	if pec.isAcquired {
		return fmt.Errorf("client is already started")
	}
	
	// Try to get server from pool
	server, err := pec.poolManager.GetServerWithContext(ctx, pec.language, pec.workspace)
	if err != nil {
		return fmt.Errorf("failed to acquire server from pool: %w", err)
	}
	
	pec.server = server
	pec.isAcquired = true
	
	return nil
}

// Stop returns the server to the pool
func (pec *PoolEnabledLSPClient) Stop() error {
	pec.mu.Lock()
	defer pec.mu.Unlock()
	
	if !pec.isAcquired || pec.server == nil {
		return nil // Nothing to stop
	}
	
	// Return server to pool
	err := pec.poolManager.ReturnServer(pec.server)
	pec.server = nil
	pec.isAcquired = false
	
	return err
}

// SendRequest forwards the request to the pooled server
func (pec *PoolEnabledLSPClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	pec.mu.Lock()
	server := pec.server
	pec.mu.Unlock()
	
	if server == nil {
		return nil, fmt.Errorf("client is not started")
	}
	
	return server.SendRequest(ctx, method, params)
}

// SendNotification forwards the notification to the pooled server
func (pec *PoolEnabledLSPClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	pec.mu.Lock()
	server := pec.server
	pec.mu.Unlock()
	
	if server == nil {
		return fmt.Errorf("client is not started")
	}
	
	return server.SendNotification(ctx, method, params)
}

// IsActive returns whether the pooled server is active
func (pec *PoolEnabledLSPClient) IsActive() bool {
	pec.mu.Lock()
	server := pec.server
	pec.mu.Unlock()
	
	if server == nil {
		return false
	}
	
	return server.IsActive()
}

// ClientConfig represents configuration for creating LSP clients
type ClientConfig struct {
	Language    string
	Command     string
	Args        []string
	Transport   string
	Workspace   string
	Environment map[string]string
}

// CreateLSPClientFactory creates a factory function for a specific language that can work with pooling
func CreateLSPClientFactory(language string, baseConfig transport.ClientConfig) func() (transport.LSPClient, error) {
	return func() (transport.LSPClient, error) {
		// Create a regular transport client
		return transport.NewLSPClient(baseConfig)
	}
}

// RegisterLanguageClientFactory registers a client factory for a language with the global pool manager
func RegisterLanguageClientFactory(language string, baseConfig transport.ClientConfig) error {
	poolManager := GetGlobalPoolManager()
	if poolManager == nil {
		return fmt.Errorf("global pool manager is not initialized")
	}
	
	factory := CreateLSPClientFactory(language, baseConfig)
	return poolManager.RegisterClientFactory(language, factory)
}

// ConfigurePoolingForLanguage configures pooling for a specific language
func ConfigurePoolingForLanguage(language string, command string, args []string, transport string) error {
	baseConfig := transport.ClientConfig{
		Command:   command,
		Args:      args,
		Transport: transport,
	}
	
	return RegisterLanguageClientFactory(language, baseConfig)
}

// SetupPoolingForCommonLanguages sets up pooling for commonly used languages
func SetupPoolingForCommonLanguages() error {
	languageConfigs := map[string]transport.ClientConfig{
		"java": {
			Command:   "jdtls",
			Args:      []string{},
			Transport: "stdio",
		},
		"typescript": {
			Command:   "typescript-language-server",
			Args:      []string{"--stdio"},
			Transport: "stdio",
		},
		"go": {
			Command:   "gopls",
			Args:      []string{},
			Transport: "stdio",
		},
		"python": {
			Command:   "pylsp",
			Args:      []string{},
			Transport: "stdio",
		},
	}
	
	for language, config := range languageConfigs {
		if err := RegisterLanguageClientFactory(language, config); err != nil {
			return fmt.Errorf("failed to register factory for %s: %w", language, err)
		}
	}
	
	return nil
}

// createFallbackClient creates a regular LSP client when pooling is not available
func createFallbackClient(config ClientConfig) (transport.LSPClient, error) {
	baseConfig := transport.ClientConfig{
		Command:   config.Command,
		Args:      config.Args,
		Transport: config.Transport,
	}
	
	return transport.NewLSPClient(baseConfig)
}

// PoolStats provides statistics about the current state of pooling
type PoolStats struct {
	IsEnabled      bool                         `json:"is_enabled"`
	IsRunning      bool                         `json:"is_running"`
	Languages      []string                     `json:"languages"`
	TotalServers   int                          `json:"total_servers"`
	ActiveServers  int                          `json:"active_servers"`
	HealthyServers int                          `json:"healthy_servers"`
	PoolHealth     *PoolHealth                  `json:"pool_health,omitempty"`
	PoolMetrics    *PoolMetrics                 `json:"pool_metrics,omitempty"`
	LanguageStats  map[string]*LanguagePoolStats `json:"language_stats,omitempty"`
}

// LanguagePoolStats provides statistics for a specific language pool
type LanguagePoolStats struct {
	Language      string                 `json:"language"`
	TotalServers  int                    `json:"total_servers"`
	ActiveServers int                    `json:"active_servers"`
	IdleServers   int                    `json:"idle_servers"`
	FailedServers int                    `json:"failed_servers"`
	ServerInfo    []*PooledServerInfo    `json:"server_info,omitempty"`
}

// GetPoolStats returns comprehensive statistics about the current pooling state
func GetPoolStats() *PoolStats {
	poolManager := GetGlobalPoolManager()
	
	stats := &PoolStats{
		IsEnabled:     false,
		IsRunning:     false,
		Languages:     []string{},
		LanguageStats: make(map[string]*LanguagePoolStats),
	}
	
	if poolManager == nil {
		return stats
	}
	
	stats.IsEnabled = true
	stats.IsRunning = poolManager.IsRunning()
	
	if !stats.IsRunning {
		return stats
	}
	
	// Get overall health
	health := poolManager.Health()
	if health != nil {
		stats.PoolHealth = health
		stats.TotalServers = health.TotalServers
		stats.ActiveServers = health.ActiveServers
		stats.HealthyServers = health.HealthyServers
		
		// Collect language statistics
		for language, langHealth := range health.Languages {
			stats.Languages = append(stats.Languages, language)
			
			serverInfo, _ := poolManager.GetLanguagePoolInfo(language)
			
			stats.LanguageStats[language] = &LanguagePoolStats{
				Language:      language,
				TotalServers:  langHealth.TotalServers,
				ActiveServers: langHealth.ActiveServers,
				IdleServers:   langHealth.IdleServers,
				FailedServers: langHealth.FailedServers,
				ServerInfo:    serverInfo,
			}
		}
	}
	
	// Get metrics
	stats.PoolMetrics = poolManager.GetMetrics()
	
	return stats
}

// IsPoolingEnabled returns whether pooling is currently enabled and running
func IsPoolingEnabled() bool {
	poolManager := GetGlobalPoolManager()
	return poolManager != nil && poolManager.IsRunning()
}

// StartPooling starts the global pool manager with default configuration
func StartPooling(ctx context.Context) error {
	poolManager := GetGlobalPoolManager()
	if poolManager == nil {
		// Initialize with default config
		if err := InitializeGlobalPoolManager(DefaultPoolConfig()); err != nil {
			return err
		}
		poolManager = GetGlobalPoolManager()
	}
	
	// Set up common language factories
	if err := SetupPoolingForCommonLanguages(); err != nil {
		return fmt.Errorf("failed to setup common language factories: %w", err)
	}
	
	// Start the pool manager
	return poolManager.Start(ctx)
}

// StopPooling stops the global pool manager
func StopPooling() error {
	return ShutdownGlobalPoolManager()
}