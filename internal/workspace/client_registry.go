package workspace

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"lsp-gateway/internal/transport"
	"lsp-gateway/mcp"
)

// ClientInfo holds comprehensive information about a registered LSP client
type ClientInfo struct {
	Client         transport.LSPClient `json:"-"`
	SubProjectID   string              `json:"sub_project_id"`
	Language       string              `json:"language"`
	WorkspaceRoot  string              `json:"workspace_root"`
	ServerCommand  string              `json:"server_command"`
	Transport      string              `json:"transport"`
	StartedAt      time.Time           `json:"started_at"`
	LastUsed       time.Time           `json:"last_used"`
	RequestCount   int64               `json:"request_count"`
	ErrorCount     int64               `json:"error_count"`
	LastError      string              `json:"last_error,omitempty"`
	IsHealthy      bool                `json:"is_healthy"`
	Priority       int                 `json:"priority"`
	Metadata       map[string]string   `json:"metadata,omitempty"`
}

// ClientRegistryHealth provides health information about the client registry
type ClientRegistryHealth struct {
	TotalClients     int                            `json:"total_clients"`
	ActiveClients    int                            `json:"active_clients"`
	IdleClients      int                            `json:"idle_clients"`
	MemoryUsageMB    int64                          `json:"memory_usage_mb"`
	RegistrySize     int                            `json:"registry_size"`
	ClientsByProject map[string]int                 `json:"clients_by_project"`
	ClientsByLanguage map[string]int                `json:"clients_by_language"`
	ClientsByTransport map[string]int               `json:"clients_by_transport"`
	OldestClient     *time.Time                     `json:"oldest_client,omitempty"`
	NewestClient     *time.Time                     `json:"newest_client,omitempty"`
	LastCleanup      time.Time                      `json:"last_cleanup"`
	CleanupCount     int64                          `json:"cleanup_count"`
}

// ClientRegistry manages multi-level client storage with thread-safe operations
type ClientRegistry struct {
	// Multi-level storage: [SubProjectID][Language] -> ClientInfo
	clients      map[string]map[string]*ClientInfo
	clientsMutex sync.RWMutex
	
	// Quick lookup indexes
	clientsByLanguage  map[string][]*ClientInfo // [Language] -> []ClientInfo
	clientsByTransport map[string][]*ClientInfo // [Transport] -> []ClientInfo
	indexMutex         sync.RWMutex
	
	// Metrics and statistics
	totalClients    int32
	activeClients   int32
	registrySize    int64
	lastCleanup     time.Time
	cleanupCount    int64
	
	// Configuration
	config *ClientManagerConfig
	logger *mcp.StructuredLogger
	
	// Lifecycle management
	ctx           context.Context
	cancel        context.CancelFunc
	isShutdown    int32
	shutdownOnce  sync.Once
}

// NewClientRegistry creates a new client registry instance
func NewClientRegistry(config *ClientManagerConfig, logger *mcp.StructuredLogger) *ClientRegistry {
	ctx, cancel := context.WithCancel(context.Background())
	
	registry := &ClientRegistry{
		clients:            make(map[string]map[string]*ClientInfo),
		clientsByLanguage:  make(map[string][]*ClientInfo),
		clientsByTransport: make(map[string][]*ClientInfo),
		config:             config,
		logger:             logger,
		ctx:                ctx,
		cancel:             cancel,
		lastCleanup:        time.Now(),
	}
	
	if logger != nil {
		logger.Info("ClientRegistry initialized successfully")
	}
	
	return registry
}

// RegisterClient registers a new LSP client in the registry
func (r *ClientRegistry) RegisterClient(clientInfo *ClientInfo) error {
	if atomic.LoadInt32(&r.isShutdown) != 0 {
		return fmt.Errorf("registry is shutdown")
	}
	
	if clientInfo == nil || clientInfo.Client == nil {
		return fmt.Errorf("invalid client info provided")
	}
	
	if clientInfo.SubProjectID == "" || clientInfo.Language == "" {
		return fmt.Errorf("sub-project ID and language are required")
	}
	
	r.clientsMutex.Lock()
	defer r.clientsMutex.Unlock()
	
	// Check if client already exists
	if subProjectClients, exists := r.clients[clientInfo.SubProjectID]; exists {
		if _, exists := subProjectClients[clientInfo.Language]; exists {
			return fmt.Errorf("client already registered for sub-project %s, language %s", 
				clientInfo.SubProjectID, clientInfo.Language)
		}
	}
	
	// Initialize sub-project map if needed
	if r.clients[clientInfo.SubProjectID] == nil {
		r.clients[clientInfo.SubProjectID] = make(map[string]*ClientInfo)
	}
	
	// Store client info
	r.clients[clientInfo.SubProjectID][clientInfo.Language] = clientInfo
	
	// Update indexes
	r.updateIndexesLocked(clientInfo, true)
	
	// Update metrics
	atomic.AddInt32(&r.totalClients, 1)
	if clientInfo.Client.IsActive() {
		atomic.AddInt32(&r.activeClients, 1)
	}
	
	// Update registry size estimation
	atomic.AddInt64(&r.registrySize, r.estimateClientInfoSize(clientInfo))
	
	if r.logger != nil {
		r.logger.WithFields(map[string]interface{}{
			"sub_project_id": clientInfo.SubProjectID,
			"language":       clientInfo.Language,
			"workspace_root": clientInfo.WorkspaceRoot,
			"transport":      clientInfo.Transport,
		}).Info("Client registered successfully")
	}
	
	return nil
}

// UnregisterClient removes an LSP client from the registry
func (r *ClientRegistry) UnregisterClient(subProjectID, language string) error {
	if atomic.LoadInt32(&r.isShutdown) != 0 {
		return fmt.Errorf("registry is shutdown")
	}
	
	r.clientsMutex.Lock()
	defer r.clientsMutex.Unlock()
	
	subProjectClients, exists := r.clients[subProjectID]
	if !exists {
		return fmt.Errorf("sub-project not found: %s", subProjectID)
	}
	
	clientInfo, exists := subProjectClients[language]
	if !exists {
		return fmt.Errorf("client not found for language: %s", language)
	}
	
	// Remove from storage
	delete(subProjectClients, language)
	if len(subProjectClients) == 0 {
		delete(r.clients, subProjectID)
	}
	
	// Update indexes  
	r.updateIndexesLocked(clientInfo, false)
	
	// Update metrics
	atomic.AddInt32(&r.totalClients, -1)
	if clientInfo.Client.IsActive() {
		atomic.AddInt32(&r.activeClients, -1)
	}
	
	// Update registry size estimation
	atomic.AddInt64(&r.registrySize, -r.estimateClientInfoSize(clientInfo))
	
	if r.logger != nil {
		r.logger.WithFields(map[string]interface{}{
			"sub_project_id": subProjectID,
			"language":       language,
		}).Info("Client unregistered successfully")
	}
	
	return nil
}

// GetClient retrieves an LSP client with O(1) lookup time
func (r *ClientRegistry) GetClient(subProjectID, language string) (transport.LSPClient, error) {
	if atomic.LoadInt32(&r.isShutdown) != 0 {
		return nil, fmt.Errorf("registry is shutdown")
	}
	
	r.clientsMutex.RLock()
	defer r.clientsMutex.RUnlock()
	
	subProjectClients, exists := r.clients[subProjectID]
	if !exists {
		return nil, fmt.Errorf("sub-project not found: %s", subProjectID)
	}
	
	clientInfo, exists := subProjectClients[language]
	if !exists {
		return nil, fmt.Errorf("client not found for language: %s", language)
	}
	
	return clientInfo.Client, nil
}

// GetClientInfo retrieves detailed information about a client
func (r *ClientRegistry) GetClientInfo(subProjectID, language string) *ClientInfo {
	if atomic.LoadInt32(&r.isShutdown) != 0 {
		return nil
	}
	
	r.clientsMutex.RLock()
	defer r.clientsMutex.RUnlock()
	
	if subProjectClients, exists := r.clients[subProjectID]; exists {
		if clientInfo, exists := subProjectClients[language]; exists {
			// Return a copy to prevent external modifications
			infoCopy := *clientInfo
			return &infoCopy
		}
	}
	
	return nil
}

// GetAllClients returns all registered clients organized by sub-project and language
func (r *ClientRegistry) GetAllClients() map[string]map[string]transport.LSPClient {
	if atomic.LoadInt32(&r.isShutdown) != 0 {
		return make(map[string]map[string]transport.LSPClient)
	}
	
	r.clientsMutex.RLock()
	defer r.clientsMutex.RUnlock()
	
	result := make(map[string]map[string]transport.LSPClient)
	
	for subProjectID, subProjectClients := range r.clients {
		result[subProjectID] = make(map[string]transport.LSPClient)
		for language, clientInfo := range subProjectClients {
			result[subProjectID][language] = clientInfo.Client
		}
	}
	
	return result
}

// GetAllClientsForProject returns all clients for a specific sub-project
func (r *ClientRegistry) GetAllClientsForProject(subProjectID string) map[string]transport.LSPClient {
	if atomic.LoadInt32(&r.isShutdown) != 0 {
		return make(map[string]transport.LSPClient)
	}
	
	r.clientsMutex.RLock()
	defer r.clientsMutex.RUnlock()
	
	result := make(map[string]transport.LSPClient)
	
	if subProjectClients, exists := r.clients[subProjectID]; exists {
		for language, clientInfo := range subProjectClients {
			result[language] = clientInfo.Client
		}
	}
	
	return result
}

// GetClientsByLanguage returns all clients for a specific language
func (r *ClientRegistry) GetClientsByLanguage(language string) []*ClientInfo {
	if atomic.LoadInt32(&r.isShutdown) != 0 {
		return nil
	}
	
	r.indexMutex.RLock()
	defer r.indexMutex.RUnlock()
	
	clients := r.clientsByLanguage[language]
	if clients == nil {
		return nil
	}
	
	// Return a copy to prevent external modifications
	result := make([]*ClientInfo, len(clients))
	for i, clientInfo := range clients {
		infoCopy := *clientInfo
		result[i] = &infoCopy
	}
	
	return result
}

// GetClientsByTransport returns all clients using a specific transport
func (r *ClientRegistry) GetClientsByTransport(transport string) []*ClientInfo {
	if atomic.LoadInt32(&r.isShutdown) != 0 {
		return nil
	}
	
	r.indexMutex.RLock()
	defer r.indexMutex.RUnlock()
	
	clients := r.clientsByTransport[transport]
	if clients == nil {
		return nil
	}
	
	// Return a copy to prevent external modifications
	result := make([]*ClientInfo, len(clients))
	for i, clientInfo := range clients {
		infoCopy := *clientInfo
		result[i] = &infoCopy
	}
	
	return result
}

// UpdateClientUsage updates the last used time for a client
func (r *ClientRegistry) UpdateClientUsage(subProjectID, language string) {
	if atomic.LoadInt32(&r.isShutdown) != 0 {
		return
	}
	
	r.clientsMutex.Lock()
	defer r.clientsMutex.Unlock()
	
	if subProjectClients, exists := r.clients[subProjectID]; exists {
		if clientInfo, exists := subProjectClients[language]; exists {
			clientInfo.LastUsed = time.Now()
			atomic.AddInt64(&clientInfo.RequestCount, 1)
		}
	}
}

// UpdateClientError updates the error information for a client
func (r *ClientRegistry) UpdateClientError(subProjectID, language, errorMsg string) {
	if atomic.LoadInt32(&r.isShutdown) != 0 {
		return
	}
	
	r.clientsMutex.Lock()
	defer r.clientsMutex.Unlock()
	
	if subProjectClients, exists := r.clients[subProjectID]; exists {
		if clientInfo, exists := subProjectClients[language]; exists {
			clientInfo.LastError = errorMsg
			atomic.AddInt64(&clientInfo.ErrorCount, 1)
			clientInfo.IsHealthy = false
		}
	}
}

// UpdateClientHealth updates the health status of a client
func (r *ClientRegistry) UpdateClientHealth(subProjectID, language string, isHealthy bool) {
	if atomic.LoadInt32(&r.isShutdown) != 0 {
		return
	}
	
	r.clientsMutex.Lock()
	defer r.clientsMutex.Unlock()
	
	if subProjectClients, exists := r.clients[subProjectID]; exists {
		if clientInfo, exists := subProjectClients[language]; exists {
			clientInfo.IsHealthy = isHealthy
			if isHealthy {
				clientInfo.LastError = ""
			}
		}
	}
}

// GetIdleClients returns clients that haven't been used since the specified cutoff time
func (r *ClientRegistry) GetIdleClients(cutoff time.Time) []*ClientInfo {
	if atomic.LoadInt32(&r.isShutdown) != 0 {
		return nil
	}
	
	r.clientsMutex.RLock()
	defer r.clientsMutex.RUnlock()
	
	var idleClients []*ClientInfo
	
	for _, subProjectClients := range r.clients {
		for _, clientInfo := range subProjectClients {
			if clientInfo.LastUsed.Before(cutoff) {
				infoCopy := *clientInfo
				idleClients = append(idleClients, &infoCopy)
			}
		}
	}
	
	return idleClients
}

// GetHealth returns comprehensive health information about the registry
func (r *ClientRegistry) GetHealth() *ClientRegistryHealth {
	if atomic.LoadInt32(&r.isShutdown) != 0 {
		return &ClientRegistryHealth{
			LastCleanup: r.lastCleanup,
		}
	}
	
	r.clientsMutex.RLock()
	defer r.clientsMutex.RUnlock()
	
	health := &ClientRegistryHealth{
		TotalClients:       int(atomic.LoadInt32(&r.totalClients)),
		ActiveClients:      int(atomic.LoadInt32(&r.activeClients)),
		MemoryUsageMB:      atomic.LoadInt64(&r.registrySize) / (1024 * 1024),
		RegistrySize:       len(r.clients),
		ClientsByProject:   make(map[string]int),
		ClientsByLanguage:  make(map[string]int),
		ClientsByTransport: make(map[string]int),
		LastCleanup:        r.lastCleanup,
		CleanupCount:       atomic.LoadInt64(&r.cleanupCount),
	}
	
	var oldestTime, newestTime *time.Time
	idleCount := 0
	idleCutoff := time.Now().Add(-5 * time.Minute) // Consider 5+ minutes as idle
	
	// Collect detailed statistics
	for subProjectID, subProjectClients := range r.clients {
		health.ClientsByProject[subProjectID] = len(subProjectClients)
		
		for language, clientInfo := range subProjectClients {
			health.ClientsByLanguage[language]++
			health.ClientsByTransport[clientInfo.Transport]++
			
			// Track oldest and newest clients
			if oldestTime == nil || clientInfo.StartedAt.Before(*oldestTime) {
				oldestTime = &clientInfo.StartedAt
			}
			if newestTime == nil || clientInfo.StartedAt.After(*newestTime) {
				newestTime = &clientInfo.StartedAt
			}
			
			// Count idle clients
			if clientInfo.LastUsed.Before(idleCutoff) {
				idleCount++
			}
		}
	}
	
	health.IdleClients = idleCount
	health.OldestClient = oldestTime
	health.NewestClient = newestTime
	
	// Add memory usage from Go runtime
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	health.MemoryUsageMB += int64(memStats.Alloc) / (1024 * 1024)
	
	return health
}

// Cleanup removes clients based on various criteria
func (r *ClientRegistry) Cleanup(maxAge time.Duration, maxIdleTime time.Duration) int {
	if atomic.LoadInt32(&r.isShutdown) != 0 {
		return 0
	}
	
	r.clientsMutex.Lock()
	defer r.clientsMutex.Unlock()
	
	now := time.Now()
	ageCutoff := now.Add(-maxAge)
	idleCutoff := now.Add(-maxIdleTime)
	removedCount := 0
	
	for subProjectID, subProjectClients := range r.clients {
		for language, clientInfo := range subProjectClients {
			shouldRemove := false
			
			// Check age-based removal
			if maxAge > 0 && clientInfo.StartedAt.Before(ageCutoff) {
				shouldRemove = true
			}
			
			// Check idle-based removal
			if maxIdleTime > 0 && clientInfo.LastUsed.Before(idleCutoff) {
				shouldRemove = true
			}
			
			// Check if client is inactive
			if !clientInfo.Client.IsActive() {
				shouldRemove = true
			}
			
			if shouldRemove {
				// Stop the client
				if err := clientInfo.Client.Stop(); err != nil && r.logger != nil {
					r.logger.WithError(err).WithFields(map[string]interface{}{
						"sub_project_id": subProjectID,
						"language":       language,
					}).Warn("Error stopping client during cleanup")
				}
				
				// Remove from storage and indexes
				delete(subProjectClients, language)
				r.updateIndexesLocked(clientInfo, false)
				
				// Update metrics
				atomic.AddInt32(&r.totalClients, -1)
				if clientInfo.Client.IsActive() {
					atomic.AddInt32(&r.activeClients, -1)
				}
				atomic.AddInt64(&r.registrySize, -r.estimateClientInfoSize(clientInfo))
				
				removedCount++
			}
		}
		
		// Remove empty sub-project maps
		if len(subProjectClients) == 0 {
			delete(r.clients, subProjectID)
		}
	}
	
	r.lastCleanup = now
	atomic.AddInt64(&r.cleanupCount, int64(removedCount))
	
	if r.logger != nil && removedCount > 0 {
		r.logger.WithFields(map[string]interface{}{
			"removed_count": removedCount,
			"max_age":       maxAge.String(),
			"max_idle_time": maxIdleTime.String(),
		}).Info("Registry cleanup completed")
	}
	
	return removedCount
}

// Shutdown gracefully shuts down the registry
func (r *ClientRegistry) Shutdown(ctx context.Context) error {
	var shutdownErr error
	
	r.shutdownOnce.Do(func() {
		atomic.StoreInt32(&r.isShutdown, 1)
		
		if r.logger != nil {
			r.logger.Info("Shutting down ClientRegistry")
		}
		
		r.cancel()
		
		r.clientsMutex.Lock()
		defer r.clientsMutex.Unlock()
		
		// Stop all clients
		var errors []error
		for subProjectID, subProjectClients := range r.clients {
			for language, clientInfo := range subProjectClients {
				if err := clientInfo.Client.Stop(); err != nil {
					errors = append(errors, fmt.Errorf("failed to stop client %s:%s: %w", subProjectID, language, err))
				}
			}
		}
		
		// Clear all storage
		r.clients = make(map[string]map[string]*ClientInfo)
		r.clientsByLanguage = make(map[string][]*ClientInfo)
		r.clientsByTransport = make(map[string][]*ClientInfo)
		
		// Reset metrics
		atomic.StoreInt32(&r.totalClients, 0)
		atomic.StoreInt32(&r.activeClients, 0)
		atomic.StoreInt64(&r.registrySize, 0)
		
		if len(errors) > 0 {
			shutdownErr = fmt.Errorf("errors during registry shutdown: %v", errors)
		}
		
		if r.logger != nil {
			if shutdownErr != nil {
				r.logger.WithError(shutdownErr).Error("ClientRegistry shutdown completed with errors")
			} else {
				r.logger.Info("ClientRegistry shutdown completed successfully")
			}
		}
	})
	
	return shutdownErr
}

// Private helper methods

// updateIndexesLocked updates the language and transport indexes (must be called with lock held)
func (r *ClientRegistry) updateIndexesLocked(clientInfo *ClientInfo, add bool) {
	r.indexMutex.Lock()
	defer r.indexMutex.Unlock()
	
	if add {
		// Add to language index
		r.clientsByLanguage[clientInfo.Language] = append(r.clientsByLanguage[clientInfo.Language], clientInfo)
		
		// Add to transport index
		r.clientsByTransport[clientInfo.Transport] = append(r.clientsByTransport[clientInfo.Transport], clientInfo)
	} else {
		// Remove from language index
		if clients, exists := r.clientsByLanguage[clientInfo.Language]; exists {
			for i, client := range clients {
				if client.SubProjectID == clientInfo.SubProjectID && client.Language == clientInfo.Language {
					r.clientsByLanguage[clientInfo.Language] = append(clients[:i], clients[i+1:]...)
					break
				}
			}
			if len(r.clientsByLanguage[clientInfo.Language]) == 0 {
				delete(r.clientsByLanguage, clientInfo.Language)
			}
		}
		
		// Remove from transport index
		if clients, exists := r.clientsByTransport[clientInfo.Transport]; exists {
			for i, client := range clients {
				if client.SubProjectID == clientInfo.SubProjectID && client.Language == clientInfo.Language {
					r.clientsByTransport[clientInfo.Transport] = append(clients[:i], clients[i+1:]...)
					break
				}
			}
			if len(r.clientsByTransport[clientInfo.Transport]) == 0 {
				delete(r.clientsByTransport, clientInfo.Transport)
			}
		}
	}
}

// estimateClientInfoSize estimates the memory footprint of a ClientInfo structure
func (r *ClientRegistry) estimateClientInfoSize(clientInfo *ClientInfo) int64 {
	size := int64(200) // Base struct size estimate
	
	// Add string field sizes
	size += int64(len(clientInfo.SubProjectID))
	size += int64(len(clientInfo.Language))
	size += int64(len(clientInfo.WorkspaceRoot))
	size += int64(len(clientInfo.ServerCommand))
	size += int64(len(clientInfo.Transport))
	size += int64(len(clientInfo.LastError))
	
	// Add metadata map size estimate
	if clientInfo.Metadata != nil {
		for key, value := range clientInfo.Metadata {
			size += int64(len(key) + len(value) + 16) // Include map overhead
		}
	}
	
	return size
}