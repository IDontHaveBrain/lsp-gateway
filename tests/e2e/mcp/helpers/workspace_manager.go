package e2e_test

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"lsp-gateway/tests/e2e/mcp/types"
)


// TestWorkspaceManager manages multiple isolated test workspaces
type TestWorkspaceManager struct {
	workspaces       map[string]*types.TestWorkspace
	tempBaseDir      string
	cleanupFuncs     []func() error
	mutex            sync.RWMutex
	repositoryCloner *RepositoryCloner
	logger           *log.Logger
}

// NewTestWorkspaceManager creates a new workspace manager
func NewTestWorkspaceManager() (*TestWorkspaceManager, error) {
	tempBaseDir, err := os.MkdirTemp("", "mcp-e2e-workspaces-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create base temp directory: %w", err)
	}

	// Create repository cloner for fatih/color project
	cloner := NewRepositoryCloner(
		"https://github.com/fatih/color.git",
		"4c05561a8fbfd21922e4908479e63b48b677a61f",
	)

	manager := &TestWorkspaceManager{
		workspaces:       make(map[string]*types.TestWorkspace),
		tempBaseDir:      tempBaseDir,
		cleanupFuncs:     make([]func() error, 0),
		repositoryCloner: cloner,
		logger:           log.New(os.Stdout, "[WorkspaceManager] ", log.LstdFlags),
	}

	// Add cleanup for base directory
	manager.cleanupFuncs = append(manager.cleanupFuncs, func() error {
		return os.RemoveAll(tempBaseDir)
	})

	return manager, nil
}

// CreateWorkspace creates a new isolated test workspace
func (wm *TestWorkspaceManager) CreateWorkspace(workspaceID string) (*types.TestWorkspace, error) {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	// Check if workspace already exists
	if _, exists := wm.workspaces[workspaceID]; exists {
		return nil, fmt.Errorf("workspace %s already exists", workspaceID)
	}

	wm.logger.Printf("Creating workspace %s", workspaceID)

	// Create unique workspace directory
	workspaceDir := filepath.Join(wm.tempBaseDir, workspaceID)
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace directory: %w", err)
	}

	// Clone repository to workspace
	cloneResult, err := wm.repositoryCloner.CloneToWorkspace(workspaceDir)
	if err != nil {
		// Cleanup on failure
		os.RemoveAll(workspaceDir)
		return nil, fmt.Errorf("failed to clone repository to workspace: %w", err)
	}

	// Create workspace instance
	workspace := &types.TestWorkspace{
		ID:            workspaceID,
		Language:      "go",
		ProjectType:   "library",
		RootPath:      cloneResult.WorkspacePath,
		RepositoryURL: cloneResult.RepositoryURL,
		CommitHash:    cloneResult.CommitHash,
		CreatedAt:     time.Now(),
		IsInitialized: false,

		// Initialize empty test data (to be populated by fixtures)
		ExpectedSymbols:    make([]types.ExpectedSymbol, 0),
		ExpectedReferences: make([]types.ExpectedReference, 0),
		ExpectedHovers:     make([]types.ExpectedHover, 0),
	}

	// Setup LSP configuration for Go
	workspace.LSPConfig = &types.LSPServerConfig{
		ServerPath:   "gopls",
		Args:         []string{"serve"},
		InitOptions:  make(map[string]interface{}),
		WorkspaceDir: workspace.RootPath,
		Language:     "go",
	}

	// Set up workspace cleanup function
	workspace.CleanupFunc = func() error {
		wm.logger.Printf("Cleaning up workspace %s at %s", workspaceID, workspace.RootPath)
		if err := os.RemoveAll(workspaceDir); err != nil {
			return fmt.Errorf("failed to remove workspace directory: %w", err)
		}
		return nil
	}

	// Validate the workspace
	if err := wm.ValidateWorkspace(workspace); err != nil {
		workspace.CleanupFunc()
		return nil, fmt.Errorf("workspace validation failed: %w", err)
	}

	// Mark as initialized
	workspace.IsInitialized = true

	// Store workspace
	wm.workspaces[workspaceID] = workspace

	// Log any validation warnings
	if len(cloneResult.ValidationErrors) > 0 {
		wm.logger.Printf("Workspace %s created with validation warnings: %v", workspaceID, cloneResult.ValidationErrors)
	} else {
		wm.logger.Printf("Workspace %s created successfully at %s", workspaceID, workspace.RootPath)
	}

	return workspace, nil
}

// GetWorkspace retrieves a workspace by ID
func (wm *TestWorkspaceManager) GetWorkspace(workspaceID string) (*types.TestWorkspace, bool) {
	wm.mutex.RLock()
	defer wm.mutex.RUnlock()

	workspace, exists := wm.workspaces[workspaceID]
	return workspace, exists
}

// ListWorkspaces returns all active workspaces
func (wm *TestWorkspaceManager) ListWorkspaces() []*types.TestWorkspace {
	wm.mutex.RLock()
	defer wm.mutex.RUnlock()

	workspaces := make([]*types.TestWorkspace, 0, len(wm.workspaces))
	for _, workspace := range wm.workspaces {
		workspaces = append(workspaces, workspace)
	}

	return workspaces
}

// DestroyWorkspace removes and cleans up a specific workspace
func (wm *TestWorkspaceManager) DestroyWorkspace(workspaceID string) error {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	workspace, exists := wm.workspaces[workspaceID]
	if !exists {
		return fmt.Errorf("workspace %s does not exist", workspaceID)
	}

	// Call workspace cleanup function
	if workspace.CleanupFunc != nil {
		if err := workspace.CleanupFunc(); err != nil {
			wm.logger.Printf("Warning: workspace cleanup failed for %s: %v", workspaceID, err)
			// Continue with removal even if cleanup fails
		}
	}

	// Remove from manager
	delete(wm.workspaces, workspaceID)

	wm.logger.Printf("Workspace %s destroyed", workspaceID)
	return nil
}

// CleanupAll cleans up all workspaces and the manager itself
func (wm *TestWorkspaceManager) CleanupAll() error {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	var errors []error

	// Clean up all workspaces
	for workspaceID, workspace := range wm.workspaces {
		if workspace.CleanupFunc != nil {
			if err := workspace.CleanupFunc(); err != nil {
				errors = append(errors, fmt.Errorf("failed to cleanup workspace %s: %w", workspaceID, err))
			}
		}
	}

	// Clear workspaces map
	wm.workspaces = make(map[string]*types.TestWorkspace)

	// Execute additional cleanup functions
	for _, cleanupFunc := range wm.cleanupFuncs {
		if err := cleanupFunc(); err != nil {
			errors = append(errors, fmt.Errorf("cleanup function failed: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup completed with %d errors: %v", len(errors), errors)
	}

	wm.logger.Printf("All workspaces cleaned up successfully")
	return nil
}

// ValidateWorkspace validates a workspace's health and structure
func (wm *TestWorkspaceManager) ValidateWorkspace(workspace *types.TestWorkspace) error {
	if workspace == nil {
		return fmt.Errorf("workspace is nil")
	}

	// Check workspace directory exists
	if _, err := os.Stat(workspace.RootPath); err != nil {
		return fmt.Errorf("workspace root path does not exist: %w", err)
	}

	// Check repository structure using cloner validation
	if err := wm.repositoryCloner.ValidateRepository(workspace.RootPath); err != nil {
		return fmt.Errorf("repository validation failed: %w", err)
	}

	// Check Go module structure
	if err := wm.repositoryCloner.VerifyGoModule(workspace.RootPath); err != nil {
		return fmt.Errorf("Go module verification failed: %w", err)
	}

	// Validate LSP configuration
	if workspace.LSPConfig == nil {
		return fmt.Errorf("LSP configuration is nil")
	}

	if workspace.LSPConfig.WorkspaceDir != workspace.RootPath {
		return fmt.Errorf("LSP workspace directory mismatch: expected %s, got %s",
			workspace.RootPath, workspace.LSPConfig.WorkspaceDir)
	}

	// Check if LSP server binary is available (gopls)
	if _, err := exec.LookPath(workspace.LSPConfig.ServerPath); err != nil {
		return fmt.Errorf("LSP server binary not found: %w", err)
	}

	return nil
}

// IsWorkspaceHealthy checks if a workspace is in a healthy state
func (wm *TestWorkspaceManager) IsWorkspaceHealthy(workspaceID string) bool {
	workspace, exists := wm.GetWorkspace(workspaceID)
	if !exists {
		return false
	}

	return wm.ValidateWorkspace(workspace) == nil
}

// GetWorkspaceCount returns the number of active workspaces
func (wm *TestWorkspaceManager) GetWorkspaceCount() int {
	wm.mutex.RLock()
	defer wm.mutex.RUnlock()
	return len(wm.workspaces)
}

// SetLogger allows customizing the logger used by the manager
func (wm *TestWorkspaceManager) SetLogger(logger *log.Logger) {
	wm.logger = logger
	wm.repositoryCloner.SetLogger(logger)
}

// GetTempBaseDirectory returns the base temporary directory used by the manager
func (wm *TestWorkspaceManager) GetTempBaseDirectory() string {
	return wm.tempBaseDir
}

// ForceCleanupWorkspace attempts to forcefully clean up a workspace, ignoring some errors
func (wm *TestWorkspaceManager) ForceCleanupWorkspace(workspaceID string) error {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	workspace, exists := wm.workspaces[workspaceID]
	if !exists {
		return fmt.Errorf("workspace %s does not exist", workspaceID)
	}

	// Attempt cleanup with best effort
	if workspace.CleanupFunc != nil {
		workspace.CleanupFunc() // Ignore error for forced cleanup
	}

	// Force remove directory if it still exists
	if _, err := os.Stat(workspace.RootPath); err == nil {
		os.RemoveAll(workspace.RootPath) // Ignore error for forced cleanup
	}

	// Remove from manager
	delete(wm.workspaces, workspaceID)

	wm.logger.Printf("Workspace %s force-cleaned", workspaceID)
	return nil
}

// RefreshWorkspace re-validates and refreshes workspace state
func (wm *TestWorkspaceManager) RefreshWorkspace(workspaceID string) error {
	workspace, exists := wm.GetWorkspace(workspaceID)
	if !exists {
		return fmt.Errorf("workspace %s does not exist", workspaceID)
	}

	// Re-validate workspace
	if err := wm.ValidateWorkspace(workspace); err != nil {
		workspace.IsInitialized = false
		return fmt.Errorf("workspace validation failed during refresh: %w", err)
	}

	workspace.IsInitialized = true
	wm.logger.Printf("Workspace %s refreshed successfully", workspaceID)
	return nil
}