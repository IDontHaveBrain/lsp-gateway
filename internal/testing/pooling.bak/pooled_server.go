package pooling

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"lsp-gateway/internal/transport"
)

// PooledServer wraps an LSP client with pooling-specific state and management
type PooledServer struct {
	// Core client
	client transport.LSPClient
	
	// Identity and metadata
	id               string
	language         string
	currentWorkspace string
	createdAt        time.Time
	
	// State management
	state        ServerState
	healthStatus HealthStatus
	lastUsed     time.Time
	lastChecked  time.Time
	useCount     int
	failureCount int
	
	// Workspace management
	workspaceHistory   []string
	maxWorkspaces      int
	workspaceSwitcher  *WorkspaceSwitcher
	
	// Synchronization
	mu sync.RWMutex
	
	// Context and lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	
	// Performance metrics
	totalRequestTime time.Duration
	requestCount     int64
	errorCount       int64
	
	// Configuration
	config *LanguagePoolConfig
}

// NewPooledServer creates a new pooled server wrapper around an LSP client
func NewPooledServer(language string, client transport.LSPClient, config *LanguagePoolConfig) *PooledServer {
	ctx, cancel := context.WithCancel(context.Background())
	
	ps := &PooledServer{
		client:           client,
		id:               uuid.New().String(),
		language:         language,
		createdAt:        time.Now(),
		state:            ServerStateStarting,
		healthStatus:     HealthStatusUnknown,
		lastUsed:         time.Now(),
		lastChecked:      time.Now(),
		useCount:         0,
		failureCount:     0,
		workspaceHistory: make([]string, 0),
		maxWorkspaces:    config.MaxWorkspacesPerServer,
		ctx:              ctx,
		cancel:           cancel,
		config:           config,
	}
	
	return ps
}

// Start initializes the pooled server and starts the underlying LSP client
func (ps *PooledServer) Start(ctx context.Context) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	
	if ps.state != ServerStateStarting {
		return fmt.Errorf("server %s is not in starting state: %s", ps.id, ps.state)
	}
	
	// Start the underlying LSP client
	startCtx, cancel := context.WithTimeout(ctx, ps.config.WarmupTimeout)
	defer cancel()
	
	if err := ps.client.Start(startCtx); err != nil {
		ps.state = ServerStateFailed
		ps.healthStatus = HealthStatusUnhealthy
		ps.failureCount++
		return fmt.Errorf("failed to start LSP client for server %s: %w", ps.id, err)
	}
	
	// Verify the client is active
	if !ps.client.IsActive() {
		ps.state = ServerStateFailed
		ps.healthStatus = HealthStatusUnhealthy
		ps.failureCount++
		return fmt.Errorf("LSP client for server %s is not active after start", ps.id)
	}
	
	ps.state = ServerStateReady
	ps.healthStatus = HealthStatusHealthy
	ps.lastChecked = time.Now()
	
	return nil
}

// Stop gracefully stops the pooled server
func (ps *PooledServer) Stop() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	
	if ps.state == ServerStateStopping || ps.state == ServerStateFailed {
		return nil // Already stopping or failed
	}
	
	ps.state = ServerStateStopping
	
	// Cancel the context
	if ps.cancel != nil {
		ps.cancel()
	}
	
	// Stop the underlying client
	if ps.client != nil {
		if err := ps.client.Stop(); err != nil {
			ps.failureCount++
			return fmt.Errorf("failed to stop LSP client for server %s: %w", ps.id, err)
		}
	}
	
	ps.state = ServerStateFailed // Terminal state after stop
	ps.healthStatus = HealthStatusUnhealthy
	
	return nil
}

// Acquire marks the server as busy and returns it for use
func (ps *PooledServer) Acquire(workspace string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	
	if ps.state != ServerStateReady && ps.state != ServerStateIdle {
		return fmt.Errorf("server %s is not available: %s", ps.id, ps.state)
	}
	
	if ps.healthStatus != HealthStatusHealthy {
		return fmt.Errorf("server %s is not healthy: %s", ps.id, ps.healthStatus)
	}
	
	// Handle workspace switching if needed
	if workspace != "" && workspace != ps.currentWorkspace {
		if err := ps.switchWorkspaceUnsafe(workspace); err != nil {
			return fmt.Errorf("failed to switch workspace for server %s: %w", ps.id, err)
		}
	}
	
	ps.state = ServerStateBusy
	ps.lastUsed = time.Now()
	ps.useCount++
	
	return nil
}

// Release marks the server as idle and available for reuse
func (ps *PooledServer) Release() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	
	if ps.state == ServerStateBusy {
		ps.state = ServerStateIdle
	}
}

// SendRequest forwards a request to the underlying LSP client with metrics tracking
func (ps *PooledServer) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	start := time.Now()
	
	// Ensure server is acquired
	ps.mu.RLock()
	if ps.state != ServerStateBusy {
		ps.mu.RUnlock()
		return nil, fmt.Errorf("server %s is not acquired: %s", ps.id, ps.state)
	}
	client := ps.client
	ps.mu.RUnlock()
	
	// Send the request
	result, err := client.SendRequest(ctx, method, params)
	
	// Update metrics
	ps.mu.Lock()
	ps.requestCount++
	ps.totalRequestTime += time.Since(start)
	if err != nil {
		ps.errorCount++
		ps.failureCount++
		
		// Check if we should mark the server as unhealthy
		if ps.failureCount >= ps.config.FailureThreshold {
			ps.healthStatus = HealthStatusUnhealthy
		} else if ps.failureCount > 0 {
			ps.healthStatus = HealthStatusDegraded
		}
	}
	ps.mu.Unlock()
	
	return result, err
}

// SendNotification forwards a notification to the underlying LSP client
func (ps *PooledServer) SendNotification(ctx context.Context, method string, params interface{}) error {
	ps.mu.RLock()
	if ps.state != ServerStateBusy {
		ps.mu.RUnlock()
		return fmt.Errorf("server %s is not acquired: %s", ps.id, ps.state)
	}
	client := ps.client
	ps.mu.RUnlock()
	
	err := client.SendNotification(ctx, method, params)
	
	if err != nil {
		ps.mu.Lock()
		ps.failureCount++
		if ps.failureCount >= ps.config.FailureThreshold {
			ps.healthStatus = HealthStatusUnhealthy
		}
		ps.mu.Unlock()
	}
	
	return err
}

// IsActive returns whether the underlying LSP client is active
func (ps *PooledServer) IsActive() bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	
	if ps.client == nil {
		return false
	}
	
	return ps.client.IsActive() && (ps.state == ServerStateReady || ps.state == ServerStateIdle || ps.state == ServerStateBusy)
}

// HealthCheck performs a health check on the server
func (ps *PooledServer) HealthCheck(ctx context.Context) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	
	ps.lastChecked = time.Now()
	
	// Check if the underlying client is active
	if ps.client == nil || !ps.client.IsActive() {
		ps.healthStatus = HealthStatusUnhealthy
		return fmt.Errorf("underlying LSP client is not active")
	}
	
	// Check failure count
	if ps.failureCount >= ps.config.FailureThreshold {
		ps.healthStatus = HealthStatusUnhealthy
		return fmt.Errorf("failure threshold exceeded: %d", ps.failureCount)
	}
	
	// Check server age
	if ps.config.MaxServerAge > 0 && time.Since(ps.createdAt) > ps.config.MaxServerAge {
		ps.healthStatus = HealthStatusDegraded
		return fmt.Errorf("server age limit exceeded")
	}
	
	// Check use count
	if ps.config.MaxUseCount > 0 && ps.useCount >= ps.config.MaxUseCount {
		ps.healthStatus = HealthStatusDegraded
		return fmt.Errorf("use count limit exceeded")
	}
	
	// Server is healthy
	if ps.failureCount == 0 {
		ps.healthStatus = HealthStatusHealthy
	} else {
		ps.healthStatus = HealthStatusDegraded
	}
	
	return nil
}

// ShouldReplace determines if this server should be replaced
func (ps *PooledServer) ShouldReplace() bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	
	// Replace if unhealthy
	if ps.healthStatus == HealthStatusUnhealthy {
		return true
	}
	
	// Replace if too old
	if ps.config.MaxServerAge > 0 && time.Since(ps.createdAt) > ps.config.MaxServerAge {
		return true
	}
	
	// Replace if used too much
	if ps.config.MaxUseCount > 0 && ps.useCount >= ps.config.MaxUseCount {
		return true
	}
	
	// Replace if failure threshold exceeded
	if ps.failureCount >= ps.config.RestartThreshold {
		return true
	}
	
	return false
}

// GetInfo returns information about the pooled server
func (ps *PooledServer) GetInfo() *PooledServerInfo {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	
	var avgRequestTime time.Duration
	if ps.requestCount > 0 {
		avgRequestTime = time.Duration(ps.totalRequestTime.Nanoseconds() / ps.requestCount)
	}
	
	return &PooledServerInfo{
		ID:               ps.id,
		Language:         ps.language,
		CurrentWorkspace: ps.currentWorkspace,
		State:            ps.state,
		HealthStatus:     ps.healthStatus,
		CreatedAt:        ps.createdAt,
		LastUsed:         ps.lastUsed,
		LastChecked:      ps.lastChecked,
		UseCount:         ps.useCount,
		FailureCount:     ps.failureCount,
		RequestCount:     ps.requestCount,
		ErrorCount:       ps.errorCount,
		AvgRequestTime:   avgRequestTime,
		WorkspaceCount:   len(ps.workspaceHistory),
		IsActive:         ps.IsActive(),
	}
}

// switchWorkspaceUnsafe switches the server to a new workspace (must be called with lock held)
func (ps *PooledServer) switchWorkspaceUnsafe(workspace string) error {
	if !ps.config.EnableWorkspaceSwitching {
		return fmt.Errorf("workspace switching is disabled for language %s", ps.language)
	}
	
	if workspace == ps.currentWorkspace {
		return nil // Already on the correct workspace
	}
	
	// Check workspace limit
	if ps.maxWorkspaces > 0 && len(ps.workspaceHistory) >= ps.maxWorkspaces && !ps.hasWorkspace(workspace) {
		return fmt.Errorf("workspace limit exceeded for server %s", ps.id)
	}
	
	// Create workspace switcher if not already created
	if ps.workspaceSwitcher == nil {
		ps.workspaceSwitcher = NewWorkspaceSwitcher(ps.client, ps.getLogger())
		if ps.currentWorkspace != "" {
			// Set the current workspace in the switcher
			ps.workspaceSwitcher.mu.Lock()
			ps.workspaceSwitcher.currentWorkspace = ps.currentWorkspace
			ps.workspaceSwitcher.mu.Unlock()
		}
	}
	
	// Use timeout from config
	switchTimeout := ps.config.WorkspaceSwitchTimeout
	if switchTimeout == 0 {
		switchTimeout = 30 * time.Second
	}
	
	ctx, cancel := context.WithTimeout(ps.ctx, switchTimeout)
	defer cancel()
	
	// Execute the workspace switch using the dedicated switcher
	if err := ps.workspaceSwitcher.SwitchWorkspace(ctx, workspace, ps.language); err != nil {
		ps.failureCount++
		return fmt.Errorf("workspace switch failed for server %s: %w", ps.id, err)
	}
	
	// Update server state after successful switch
	ps.currentWorkspace = workspace
	
	// Add to history if not already present
	if !ps.hasWorkspace(workspace) {
		ps.workspaceHistory = append(ps.workspaceHistory, workspace)
	}
	
	return nil
}

// hasWorkspace checks if the server has worked with a specific workspace
func (ps *PooledServer) hasWorkspace(workspace string) bool {
	for _, w := range ps.workspaceHistory {
		if w == workspace {
			return true
		}
	}
	return false
}

// getLogger returns a logger for the pooled server
// For now, this creates a simple logger that can be used for workspace switching
func (ps *PooledServer) getLogger() Logger {
	return &pooledServerLogger{serverID: ps.id, language: ps.language}
}

// pooledServerLogger implements the Logger interface for pooled servers
type pooledServerLogger struct {
	serverID string
	language string
}

func (psl *pooledServerLogger) Debug(msg string, args ...interface{}) {
	// In a full implementation, this would integrate with the existing logging system
	// For now, we'll use a simple format that includes server context
	fmt.Printf("[DEBUG][%s][%s] %s\n", psl.serverID[:8], psl.language, fmt.Sprintf(msg, args...))
}

func (psl *pooledServerLogger) Info(msg string, args ...interface{}) {
	fmt.Printf("[INFO][%s][%s] %s\n", psl.serverID[:8], psl.language, fmt.Sprintf(msg, args...))
}

func (psl *pooledServerLogger) Warn(msg string, args ...interface{}) {
	fmt.Printf("[WARN][%s][%s] %s\n", psl.serverID[:8], psl.language, fmt.Sprintf(msg, args...))
}

func (psl *pooledServerLogger) Error(msg string, args ...interface{}) {
	fmt.Printf("[ERROR][%s][%s] %s\n", psl.serverID[:8], psl.language, fmt.Sprintf(msg, args...))
}

// PooledServerInfo provides information about a pooled server
type PooledServerInfo struct {
	ID               string        `json:"id"`
	Language         string        `json:"language"`
	CurrentWorkspace string        `json:"current_workspace"`
	State            ServerState   `json:"state"`
	HealthStatus     HealthStatus  `json:"health_status"`
	CreatedAt        time.Time     `json:"created_at"`
	LastUsed         time.Time     `json:"last_used"`
	LastChecked      time.Time     `json:"last_checked"`
	UseCount         int           `json:"use_count"`
	FailureCount     int           `json:"failure_count"`
	RequestCount     int64         `json:"request_count"`
	ErrorCount       int64         `json:"error_count"`
	AvgRequestTime   time.Duration `json:"avg_request_time"`
	WorkspaceCount   int           `json:"workspace_count"`
	IsActive         bool          `json:"is_active"`
}