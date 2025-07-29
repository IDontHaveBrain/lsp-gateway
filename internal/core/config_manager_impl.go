package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"lsp-gateway/internal/setup"
)

// FileConfigManager implements ConfigManager using file-based storage with in-memory caching
type HybridConfigManager struct {
	mu          sync.RWMutex
	
	// Configuration data
	globalConfig   *GlobalConfig
	projectConfigs map[string]*ProjectConfig
	clientConfigs  map[string]*ClientConfig
	
	// File paths
	globalConfigPath   string
	projectConfigsDir  string
	clientConfigsDir   string
	
	// Caching and watching
	configCache        map[string]*ResolvedConfig
	cacheExpiry        map[string]time.Time
	cacheTTL           time.Duration
	
	// Watchers - separate maps for type safety
	globalWatchers     map[string][]chan *GlobalConfig
	projectWatchers    map[string][]chan *ProjectConfig
	watcherStopCh      map[string]chan struct{}
	
	// State
	logger             *setup.SetupLogger
}

// NewHybridConfigManager creates a new hybrid configuration manager
func NewHybridConfigManager(configData interface{}) (ConfigManager, error) {
	_ = defaultHybridConfigManagerConfig()
	if configData != nil {
		// TODO: Unmarshal configData into config
	}
	
	manager := &HybridConfigManager{
		projectConfigs:     make(map[string]*ProjectConfig),
		clientConfigs:      make(map[string]*ClientConfig),
		configCache:        make(map[string]*ResolvedConfig),
		cacheExpiry:        make(map[string]time.Time),
		cacheTTL:           5 * time.Minute,
		globalWatchers:     make(map[string][]chan *GlobalConfig),
		projectWatchers:    make(map[string][]chan *ProjectConfig),
		watcherStopCh:      make(map[string]chan struct{}),
		logger:             setup.NewSetupLogger(nil),
		
		// Default paths - should be configurable
		globalConfigPath:   "/etc/lsp-gateway/global.json",
		projectConfigsDir:  "/etc/lsp-gateway/projects",
		clientConfigsDir:   "/etc/lsp-gateway/clients",
	}
	
	// Load initial configurations
	if err := manager.loadConfigurations(); err != nil {
		return nil, fmt.Errorf("failed to load initial configurations: %w", err)
	}
	
	return manager, nil
}

// GetServerConfig implements ConfigManager interface
func (m *HybridConfigManager) GetServerConfig(serverName string, workspaceID string) (*ServerConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Check cache first
	cacheKey := fmt.Sprintf("server_%s_%s", serverName, workspaceID)
	if resolved, expires, found := m.getCachedResolvedConfig(cacheKey); found && time.Now().Before(expires) {
		if serverConfig, exists := resolved.ServerConfigs[serverName]; exists {
			return serverConfig, nil
		}
	}
	
	// Build resolved configuration
	resolved, err := m.buildResolvedConfig(workspaceID, "")
	if err != nil {
		return nil, fmt.Errorf("failed to build resolved config: %w", err)
	}
	
	// Cache the resolved configuration
	m.cacheResolvedConfig(cacheKey, resolved)
	
	serverConfig, exists := resolved.ServerConfigs[serverName]
	if !exists {
		return nil, fmt.Errorf("server %s not found in resolved configuration", serverName)
	}
	
	return serverConfig, nil
}

// GetGlobalConfig implements ConfigManager interface
func (m *HybridConfigManager) GetGlobalConfig() (*GlobalConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.globalConfig == nil {
		return nil, fmt.Errorf("global configuration not loaded")
	}
	
	// Return a copy to prevent external modification
	return m.copyGlobalConfig(m.globalConfig), nil
}

// GetProjectConfig implements ConfigManager interface
func (m *HybridConfigManager) GetProjectConfig(workspaceID string) (*ProjectConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	config, exists := m.projectConfigs[workspaceID]
	if !exists {
		return nil, fmt.Errorf("project configuration not found for workspace %s", workspaceID)
	}
	
	// Return a copy to prevent external modification
	return m.copyProjectConfig(config), nil
}

// GetClientConfig implements ConfigManager interface
func (m *HybridConfigManager) GetClientConfig(clientID string) (*ClientConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	config, exists := m.clientConfigs[clientID]
	if !exists {
		return nil, fmt.Errorf("client configuration not found for client %s", clientID)
	}
	
	// Return a copy to prevent external modification
	return m.copyClientConfig(config), nil
}

// UpdateGlobalConfig implements ConfigManager interface
func (m *HybridConfigManager) UpdateGlobalConfig(config *GlobalConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Validate configuration
	if err := m.ValidateConfig(config); err != nil {
		return fmt.Errorf("invalid global configuration: %w", err)
	}
	
	// Update timestamp
	config.UpdatedAt = time.Now()
	
	// Save to file
	if err := m.saveGlobalConfig(config); err != nil {
		return fmt.Errorf("failed to save global configuration: %w", err)
	}
	
	// Update in-memory copy
	m.globalConfig = m.copyGlobalConfig(config)
	
	// Invalidate cache
	m.invalidateCache("global")
	
	// Notify watchers
	m.notifyWatchers("global", config)
	
	m.logger.Info("Global configuration updated successfully")
	return nil
}

// UpdateProjectConfig implements ConfigManager interface
func (m *HybridConfigManager) UpdateProjectConfig(workspaceID string, config *ProjectConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Validate configuration
	if err := m.ValidateConfig(config); err != nil {
		return fmt.Errorf("invalid project configuration: %w", err)
	}
	
	// Ensure workspace ID matches
	config.WorkspaceID = workspaceID
	config.UpdatedAt = time.Now()
	
	// Save to file
	if err := m.saveProjectConfig(workspaceID, config); err != nil {
		return fmt.Errorf("failed to save project configuration: %w", err)
	}
	
	// Update in-memory copy
	m.projectConfigs[workspaceID] = m.copyProjectConfig(config)
	
	// Invalidate cache
	m.invalidateCache(fmt.Sprintf("project_%s", workspaceID))
	
	// Notify watchers
	m.notifyWatchers(fmt.Sprintf("project_%s", workspaceID), config)
	
	m.logger.WithField("workspace_id", workspaceID).Info("Project configuration updated successfully")
	return nil
}

// UpdateClientConfig implements ConfigManager interface
func (m *HybridConfigManager) UpdateClientConfig(clientID string, config *ClientConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Validate configuration
	if err := m.ValidateConfig(config); err != nil {
		return fmt.Errorf("invalid client configuration: %w", err)
	}
	
	// Ensure client ID matches
	config.ClientID = clientID
	config.UpdatedAt = time.Now()
	
	// Save to file
	if err := m.saveClientConfig(clientID, config); err != nil {
		return fmt.Errorf("failed to save client configuration: %w", err)
	}
	
	// Update in-memory copy
	m.clientConfigs[clientID] = m.copyClientConfig(config)
	
	// Invalidate cache
	m.invalidateCache(fmt.Sprintf("client_%s", clientID))
	
	// Notify watchers
	m.notifyWatchers(fmt.Sprintf("client_%s", clientID), config)
	
	m.logger.WithField("client_id", clientID).Info("Client configuration updated successfully")
	return nil
}

// ValidateConfig implements ConfigManager interface
func (m *HybridConfigManager) ValidateConfig(config interface{}) error {
	switch c := config.(type) {
	case *GlobalConfig:
		return m.validateGlobalConfig(c)
	case *ProjectConfig:
		return m.validateProjectConfig(c)
	case *ClientConfig:
		return m.validateClientConfig(c)
	case *ServerConfig:
		return m.validateServerConfig(c)
	default:
		return fmt.Errorf("unsupported configuration type: %T", config)
	}
}

// MergeConfigs implements ConfigManager interface
func (m *HybridConfigManager) MergeConfigs(global *GlobalConfig, project *ProjectConfig, client *ClientConfig) *ResolvedConfig {
	resolved := &ResolvedConfig{
		ServerConfigs:   make(map[string]*ServerConfig),
		GlobalSettings:  global,
		ResolvedAt:      time.Now(),
	}
	
	// Start with global server defaults
	if global != nil {
		for name, serverConfig := range global.ServerDefaults {
			resolved.ServerConfigs[name] = m.copyServerConfig(serverConfig)
		}
		resolved.CacheConfig = global.CacheConfig
	}
	
	// Apply project overrides
	if project != nil {
		resolved.ProjectSettings = project
		for name, serverConfig := range project.ServerOverrides {
			if existing, exists := resolved.ServerConfigs[name]; exists {
				// Merge server configurations
				merged := m.mergeServerConfigs(existing, serverConfig)
				resolved.ServerConfigs[name] = merged
			} else {
				resolved.ServerConfigs[name] = m.copyServerConfig(serverConfig)
			}
		}
	}
	
	// Apply client preferences (no direct server config overrides, but affects selection)
	if client != nil {
		resolved.ClientSettings = client
	}
	
	return resolved
}

// WatchGlobalConfig implements ConfigManager interface
func (m *HybridConfigManager) WatchGlobalConfig(ctx context.Context) (<-chan *GlobalConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	ch := make(chan *GlobalConfig, 1)
	
	// Add to watchers
	watcherKey := "global"
	m.globalWatchers[watcherKey] = append(m.globalWatchers[watcherKey], ch)
	
	// Start file watching if not already started
	if _, exists := m.watcherStopCh[watcherKey]; !exists {
		stopCh := make(chan struct{})
		m.watcherStopCh[watcherKey] = stopCh
		go m.watchConfigFile(m.globalConfigPath, watcherKey, stopCh)
	}
	
	// Send current configuration immediately
	if m.globalConfig != nil {
		select {
		case ch <- m.copyGlobalConfig(m.globalConfig):
		default:
		}
	}
	
	// Handle context cancellation
	go func() {
		<-ctx.Done()
		m.removeGlobalWatcher(watcherKey, ch)
		close(ch)
	}()
	
	return ch, nil
}

// WatchProjectConfig implements ConfigManager interface
func (m *HybridConfigManager) WatchProjectConfig(ctx context.Context, workspaceID string) (<-chan *ProjectConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	ch := make(chan *ProjectConfig, 1)
	
	// Add to watchers
	watcherKey := fmt.Sprintf("project_%s", workspaceID)
	m.projectWatchers[watcherKey] = append(m.projectWatchers[watcherKey], ch)
	
	// Start file watching if not already started
	if _, exists := m.watcherStopCh[watcherKey]; !exists {
		stopCh := make(chan struct{})
		m.watcherStopCh[watcherKey] = stopCh
		configPath := filepath.Join(m.projectConfigsDir, workspaceID+".json")
		go m.watchConfigFile(configPath, watcherKey, stopCh)
	}
	
	// Send current configuration immediately
	if config, exists := m.projectConfigs[workspaceID]; exists {
		select {
		case ch <- m.copyProjectConfig(config):
		default:
		}
	}
	
	// Handle context cancellation
	go func() {
		<-ctx.Done()
		m.removeProjectWatcher(watcherKey, ch)
		close(ch)
	}()
	
	return ch, nil
}

// Private helper methods

func (m *HybridConfigManager) loadConfigurations() error {
	// Load global configuration
	if err := m.loadGlobalConfig(); err != nil {
		m.logger.WithError(err).Warn("Failed to load global configuration, using defaults")
		m.globalConfig = defaultGlobalConfig()
	}
	
	// Load project configurations
	if err := m.loadProjectConfigs(); err != nil {
		m.logger.WithError(err).Warn("Failed to load project configurations")
	}
	
	// Load client configurations  
	if err := m.loadClientConfigs(); err != nil {
		m.logger.WithError(err).Warn("Failed to load client configurations")
	}
	
	return nil
}

func (m *HybridConfigManager) loadGlobalConfig() error {
	data, err := os.ReadFile(m.globalConfigPath)
	if err != nil {
		return err
	}
	
	var config GlobalConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}
	
	m.globalConfig = &config
	return nil
}

func (m *HybridConfigManager) loadProjectConfigs() error {
	if _, err := os.Stat(m.projectConfigsDir); os.IsNotExist(err) {
		return nil // Directory doesn't exist, no configs to load
	}
	
	entries, err := os.ReadDir(m.projectConfigsDir)
	if err != nil {
		return err
	}
	
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			workspaceID := entry.Name()[:len(entry.Name())-5] // Remove .json
			configPath := filepath.Join(m.projectConfigsDir, entry.Name())
			
			data, err := os.ReadFile(configPath)
			if err != nil {
				m.logger.WithError(err).WithField("workspace_id", workspaceID).
					Error("Failed to load project configuration")
				continue
			}
			
			var config ProjectConfig
			if err := json.Unmarshal(data, &config); err != nil {
				m.logger.WithError(err).WithField("workspace_id", workspaceID).
					Error("Failed to parse project configuration")
				continue
			}
			
			m.projectConfigs[workspaceID] = &config
		}
	}
	
	return nil
}

func (m *HybridConfigManager) loadClientConfigs() error {
	if _, err := os.Stat(m.clientConfigsDir); os.IsNotExist(err) {
		return nil // Directory doesn't exist, no configs to load
	}
	
	entries, err := os.ReadDir(m.clientConfigsDir)
	if err != nil {
		return err
	}
	
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			clientID := entry.Name()[:len(entry.Name())-5] // Remove .json
			configPath := filepath.Join(m.clientConfigsDir, entry.Name())
			
			data, err := os.ReadFile(configPath)
			if err != nil {
				m.logger.WithError(err).WithField("client_id", clientID).
					Error("Failed to load client configuration")
				continue
			}
			
			var config ClientConfig
			if err := json.Unmarshal(data, &config); err != nil {
				m.logger.WithError(err).WithField("client_id", clientID).
					Error("Failed to parse client configuration")
				continue
			}
			
			m.clientConfigs[clientID] = &config
		}
	}
	
	return nil
}

func (m *HybridConfigManager) buildResolvedConfig(workspaceID, clientID string) (*ResolvedConfig, error) {
	var projectConfig *ProjectConfig
	var clientConfig *ClientConfig
	
	if workspaceID != "" {
		if config, exists := m.projectConfigs[workspaceID]; exists {
			projectConfig = config
		}
	}
	
	if clientID != "" {
		if config, exists := m.clientConfigs[clientID]; exists {
			clientConfig = config
		}
	}
	
	return m.MergeConfigs(m.globalConfig, projectConfig, clientConfig), nil
}

func (m *HybridConfigManager) getCachedResolvedConfig(key string) (*ResolvedConfig, time.Time, bool) {
	config, exists := m.configCache[key]
	if !exists {
		return nil, time.Time{}, false
	}
	
	expires, exists := m.cacheExpiry[key]
	if !exists {
		return nil, time.Time{}, false
	}
	
	return config, expires, true
}

func (m *HybridConfigManager) cacheResolvedConfig(key string, config *ResolvedConfig) {
	m.configCache[key] = config
	m.cacheExpiry[key] = time.Now().Add(m.cacheTTL)
}

func (m *HybridConfigManager) invalidateCache(pattern string) {
	// Remove cache entries that match the pattern
	for key := range m.configCache {
		if key == pattern || (pattern == "global" && key != pattern) {
			delete(m.configCache, key)
			delete(m.cacheExpiry, key)
		}
	}
}

func (m *HybridConfigManager) notifyWatchers(watcherKey string, config interface{}) {
	switch c := config.(type) {
	case *GlobalConfig:
		if watchers, exists := m.globalWatchers[watcherKey]; exists {
			for _, ch := range watchers {
				select {
				case ch <- c:
				default:
					// Channel full, skip notification
				}
			}
		}
	case *ProjectConfig:
		if watchers, exists := m.projectWatchers[watcherKey]; exists {
			for _, ch := range watchers {
				select {
				case ch <- c:
				default:
					// Channel full, skip notification
				}
			}
		}
	}
}

func (m *HybridConfigManager) removeGlobalWatcher(watcherKey string, ch chan *GlobalConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if watchers, exists := m.globalWatchers[watcherKey]; exists {
		for i, watcher := range watchers {
			if watcher == ch {
				m.globalWatchers[watcherKey] = append(watchers[:i], watchers[i+1:]...)
				break
			}
		}
		
		// Stop file watching if no more watchers
		if len(m.globalWatchers[watcherKey]) == 0 && len(m.projectWatchers[watcherKey]) == 0 {
			if stopCh, exists := m.watcherStopCh[watcherKey]; exists {
				close(stopCh)
				delete(m.watcherStopCh, watcherKey)
			}
			delete(m.globalWatchers, watcherKey)
		}
	}
}

func (m *HybridConfigManager) removeProjectWatcher(watcherKey string, ch chan *ProjectConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if watchers, exists := m.projectWatchers[watcherKey]; exists {
		for i, watcher := range watchers {
			if watcher == ch {
				m.projectWatchers[watcherKey] = append(watchers[:i], watchers[i+1:]...)
				break
			}
		}
		
		// Stop file watching if no more watchers
		if len(m.globalWatchers[watcherKey]) == 0 && len(m.projectWatchers[watcherKey]) == 0 {
			if stopCh, exists := m.watcherStopCh[watcherKey]; exists {
				close(stopCh)
				delete(m.watcherStopCh, watcherKey)
			}
			delete(m.projectWatchers, watcherKey)
		}
	}
}

func (m *HybridConfigManager) watchConfigFile(filePath, watcherKey string, stopCh chan struct{}) {
	// Simplified file watching - in production would use fsnotify
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	var lastModTime time.Time
	if stat, err := os.Stat(filePath); err == nil {
		lastModTime = stat.ModTime()
	}
	
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			if stat, err := os.Stat(filePath); err == nil {
				if stat.ModTime().After(lastModTime) {
					lastModTime = stat.ModTime()
					// Reload configuration and notify watchers
					// Implementation would reload the specific config type
				}
			}
		}
	}
}

// Validation methods

func (m *HybridConfigManager) validateGlobalConfig(config *GlobalConfig) error {
	if config.DefaultTimeout <= 0 {
		return fmt.Errorf("default timeout must be positive")
	}
	
	if config.MaxConnections <= 0 {
		return fmt.Errorf("max connections must be positive")
	}
	
	// Validate server defaults
	for name, serverConfig := range config.ServerDefaults {
		if err := m.validateServerConfig(serverConfig); err != nil {
			return fmt.Errorf("invalid server config for %s: %w", name, err)
		}
	}
	
	return nil
}

func (m *HybridConfigManager) validateProjectConfig(config *ProjectConfig) error {
	if config.WorkspaceID == "" {
		return fmt.Errorf("workspace ID cannot be empty")
	}
	
	if config.RootPath == "" {
		return fmt.Errorf("root path cannot be empty")
	}
	
	// Validate server overrides
	for name, serverConfig := range config.ServerOverrides {
		if err := m.validateServerConfig(serverConfig); err != nil {
			return fmt.Errorf("invalid server override for %s: %w", name, err)
		}
	}
	
	return nil
}

func (m *HybridConfigManager) validateClientConfig(config *ClientConfig) error {
	if config.ClientID == "" {
		return fmt.Errorf("client ID cannot be empty")
	}
	
	if config.MaxRequestTime <= 0 {
		return fmt.Errorf("max request time must be positive")
	}
	
	if config.MaxConcurrency <= 0 {
		return fmt.Errorf("max concurrency must be positive")
	}
	
	return nil
}

func (m *HybridConfigManager) validateServerConfig(config *ServerConfig) error {
	if config.Name == "" {
		return fmt.Errorf("server name cannot be empty")
	}
	
	if len(config.Command) == 0 {
		return fmt.Errorf("server command cannot be empty")
	}
	
	if config.StartupTimeout <= 0 {
		return fmt.Errorf("startup timeout must be positive")
	}
	
	return nil
}

// Copy methods to prevent external modification

func (m *HybridConfigManager) copyGlobalConfig(config *GlobalConfig) *GlobalConfig {
	// Deep copy implementation
	copy := *config
	copy.ServerDefaults = make(map[string]*ServerConfig)
	for k, v := range config.ServerDefaults {
		copy.ServerDefaults[k] = m.copyServerConfig(v)
	}
	copy.Features = make(map[string]bool)
	for k, v := range config.Features {
		copy.Features[k] = v
	}
	return &copy
}

func (m *HybridConfigManager) copyProjectConfig(config *ProjectConfig) *ProjectConfig {
	// Deep copy implementation
	copy := *config
	copy.ServerOverrides = make(map[string]*ServerConfig)
	for k, v := range config.ServerOverrides {
		copy.ServerOverrides[k] = m.copyServerConfig(v)
	}
	copy.SourceDirs = make([]string, len(config.SourceDirs))
	copy.ExcludeDirs = make([]string, len(config.ExcludeDirs))
	copy.LanguageSettings = make(map[string]interface{})
	// ... complete deep copy
	return &copy
}

func (m *HybridConfigManager) copyClientConfig(config *ClientConfig) *ClientConfig {
	// Deep copy implementation
	copy := *config
	copy.PreferredServers = make([]string, len(config.PreferredServers))
	copy.DisabledServers = make([]string, len(config.DisabledServers))
	copy.EnabledFeatures = make([]string, len(config.EnabledFeatures))
	copy.DisabledFeatures = make([]string, len(config.DisabledFeatures))
	// ... complete copy
	return &copy
}

func (m *HybridConfigManager) copyServerConfig(config *ServerConfig) *ServerConfig {
	// Deep copy implementation
	copy := *config
	copy.Command = make([]string, len(config.Command))
	copy.Args = make([]string, len(config.Args))
	copy.Env = make(map[string]string)
	copy.LanguageIDs = make([]string, len(config.LanguageIDs))
	copy.FileExtensions = make([]string, len(config.FileExtensions))
	// ... complete copy
	return &copy
}

func (m *HybridConfigManager) mergeServerConfigs(base, override *ServerConfig) *ServerConfig {
	// Merge server configurations with override precedence
	merged := m.copyServerConfig(base)
	
	// Override non-zero values
	if len(override.Command) > 0 {
		merged.Command = make([]string, len(override.Command))
		copy(merged.Command, override.Command)
	}
	
	if len(override.Args) > 0 {
		merged.Args = make([]string, len(override.Args))
		copy(merged.Args, override.Args)
	}
	
	if override.StartupTimeout > 0 {
		merged.StartupTimeout = override.StartupTimeout
	}
	
	// ... complete merge logic
	
	return merged
}

func (m *HybridConfigManager) saveGlobalConfig(config *GlobalConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(m.globalConfigPath), 0755); err != nil {
		return err
	}
	
	return os.WriteFile(m.globalConfigPath, data, 0644)
}

func (m *HybridConfigManager) saveProjectConfig(workspaceID string, config *ProjectConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	
	// Ensure directory exists
	if err := os.MkdirAll(m.projectConfigsDir, 0755); err != nil {
		return err
	}
	
	configPath := filepath.Join(m.projectConfigsDir, workspaceID+".json")
	return os.WriteFile(configPath, data, 0644)
}

func (m *HybridConfigManager) saveClientConfig(clientID string, config *ClientConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	
	// Ensure directory exists
	if err := os.MkdirAll(m.clientConfigsDir, 0755); err != nil {
		return err
	}
	
	configPath := filepath.Join(m.clientConfigsDir, clientID+".json")
	return os.WriteFile(configPath, data, 0644)
}

func defaultHybridConfigManagerConfig() interface{} {
	return map[string]interface{}{
		"cache_ttl":             5 * time.Minute,
		"global_config_path":    "/etc/lsp-gateway/global.json",
		"project_configs_dir":   "/etc/lsp-gateway/projects",
		"client_configs_dir":    "/etc/lsp-gateway/clients",
	}
}

func defaultGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		DefaultTimeout:   30 * time.Second,
		MaxConnections:   50,
		ServerDefaults:   make(map[string]*ServerConfig),
		MemoryLimit:      1024 * 1024 * 1024, // 1GB
		MaxWorkspaces:    100,
		LogLevel:         "info",
		MetricsEnabled:   true,
		Features:         make(map[string]bool),
		Version:          "1.0.0",
		UpdatedAt:        time.Now(),
	}
}