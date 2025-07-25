package framework

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TestWorkspaceManager manages test workspaces and their lifecycle
type TestWorkspaceManager struct {
	// Configuration
	TempDir       string
	MaxWorkspaces int
	CleanupOnExit bool

	// State tracking
	workspaces    map[string]*TestWorkspace
	nextID        int
	isInitialized bool

	// Call tracking
	InitializeCalls      []context.Context
	CreateWorkspaceCalls []WorkspaceConfig
	CleanupCalls         []interface{}

	// Function fields for behavior customization
	InitializeFunc      func(ctx context.Context) error
	CreateWorkspaceFunc func(config *WorkspaceConfig) (*TestWorkspace, error)
	CleanupFunc         func() error

	mu sync.RWMutex
}

// TestWorkspace represents a single test workspace
type TestWorkspace struct {
	ID        string
	Name      string
	RootPath  string
	Config    *WorkspaceConfig
	Projects  []*TestProject
	CreatedAt time.Time
	LastUsed  time.Time
	IsActive  bool
	TempFiles []string

	// Workspace-specific state
	mutex sync.RWMutex
}

// WorkspaceConfig defines workspace creation parameters
type WorkspaceConfig struct {
	Name        string
	Type        WorkspaceType
	Projects    []ProjectConfig
	SharedFiles map[string]string
	Environment map[string]string
	TempDir     string
	AutoCleanup bool
}

// WorkspaceType defines different types of test workspaces
type WorkspaceType string

const (
	WorkspaceTypeSingle      WorkspaceType = "single"
	WorkspaceTypeMulti       WorkspaceType = "multi"
	WorkspaceTypeMonorepo    WorkspaceType = "monorepo"
	WorkspaceTypeDistributed WorkspaceType = "distributed"
)

// ProjectConfig defines individual project configuration within a workspace
type ProjectConfig struct {
	Name         string
	Language     string
	Path         string
	Dependencies []string
	BuildFiles   []string
}

// NewTestWorkspaceManager creates a new workspace manager
func NewTestWorkspaceManager(tempDir string) *TestWorkspaceManager {
	return &TestWorkspaceManager{
		TempDir:       tempDir,
		MaxWorkspaces: 50,
		CleanupOnExit: true,
		workspaces:    make(map[string]*TestWorkspace),
		nextID:        1,

		// Initialize call tracking
		InitializeCalls:      make([]context.Context, 0),
		CreateWorkspaceCalls: make([]WorkspaceConfig, 0),
		CleanupCalls:         make([]interface{}, 0),
	}
}

// Initialize sets up the workspace manager
func (w *TestWorkspaceManager) Initialize(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.InitializeCalls = append(w.InitializeCalls, ctx)

	if w.InitializeFunc != nil {
		return w.InitializeFunc(ctx)
	}

	if w.isInitialized {
		return fmt.Errorf("workspace manager is already initialized")
	}

	// Create base workspace directory
	workspaceDir := filepath.Join(w.TempDir, "workspaces")
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return fmt.Errorf("failed to create workspaces directory: %w", err)
	}

	// Create shared resources directory
	sharedDir := filepath.Join(w.TempDir, "shared")
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		return fmt.Errorf("failed to create shared directory: %w", err)
	}

	w.isInitialized = true

	return nil
}

// CreateWorkspace creates a new test workspace
func (w *TestWorkspaceManager) CreateWorkspace(config *WorkspaceConfig) (*TestWorkspace, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.CreateWorkspaceCalls = append(w.CreateWorkspaceCalls, *config)

	if w.CreateWorkspaceFunc != nil {
		return w.CreateWorkspaceFunc(config)
	}

	if !w.isInitialized {
		return nil, fmt.Errorf("workspace manager is not initialized")
	}

	if len(w.workspaces) >= w.MaxWorkspaces {
		return nil, fmt.Errorf("maximum workspaces limit reached")
	}

	// Generate unique workspace ID
	workspaceID := fmt.Sprintf("workspace-%d", w.nextID)
	w.nextID++

	// Create workspace directory
	workspacePath := filepath.Join(w.TempDir, "workspaces", workspaceID)
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace directory: %w", err)
	}

	// Create workspace instance
	workspace := &TestWorkspace{
		ID:        workspaceID,
		Name:      config.Name,
		RootPath:  workspacePath,
		Config:    config,
		Projects:  make([]*TestProject, 0),
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		IsActive:  true,
		TempFiles: make([]string, 0),
	}

	// Create shared files
	if err := w.createSharedFiles(workspace, config.SharedFiles); err != nil {
		return nil, fmt.Errorf("failed to create shared files: %w", err)
	}

	// Setup environment
	if err := w.setupEnvironment(workspace, config.Environment); err != nil {
		return nil, fmt.Errorf("failed to setup environment: %w", err)
	}

	w.workspaces[workspaceID] = workspace

	return workspace, nil
}

// GetWorkspace retrieves a workspace by ID
func (w *TestWorkspaceManager) GetWorkspace(workspaceID string) (*TestWorkspace, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	workspace, exists := w.workspaces[workspaceID]
	if exists {
		workspace.mutex.Lock()
		workspace.LastUsed = time.Now()
		workspace.mutex.Unlock()
	}

	return workspace, exists
}

// ListWorkspaces returns all active workspaces
func (w *TestWorkspaceManager) ListWorkspaces() []*TestWorkspace {
	w.mu.RLock()
	defer w.mu.RUnlock()

	workspaces := make([]*TestWorkspace, 0, len(w.workspaces))
	for _, workspace := range w.workspaces {
		if workspace.IsActive {
			workspaces = append(workspaces, workspace)
		}
	}

	return workspaces
}

// DestroyWorkspace removes a workspace and cleans up its resources
func (w *TestWorkspaceManager) DestroyWorkspace(workspaceID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	workspace, exists := w.workspaces[workspaceID]
	if !exists {
		return fmt.Errorf("workspace %s not found", workspaceID)
	}

	// Mark as inactive
	workspace.mutex.Lock()
	workspace.IsActive = false
	workspace.mutex.Unlock()

	// Clean up workspace files
	if err := os.RemoveAll(workspace.RootPath); err != nil {
		return fmt.Errorf("failed to remove workspace directory: %w", err)
	}

	// Remove from tracking
	delete(w.workspaces, workspaceID)

	return nil
}

// Cleanup destroys all workspaces and cleans up resources
func (w *TestWorkspaceManager) Cleanup() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.CleanupCalls = append(w.CleanupCalls, struct{}{})

	if w.CleanupFunc != nil {
		return w.CleanupFunc()
	}

	var errors []error

	// Destroy all workspaces
	for workspaceID := range w.workspaces {
		if err := w.destroyWorkspaceInternal(workspaceID); err != nil {
			errors = append(errors, fmt.Errorf("failed to destroy workspace %s: %w", workspaceID, err))
		}
	}

	// Clear state
	w.workspaces = make(map[string]*TestWorkspace)
	w.isInitialized = false
	w.nextID = 1

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}

	return nil
}

// Reset clears all tracking data and workspaces
func (w *TestWorkspaceManager) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Destroy all workspaces
	for workspaceID := range w.workspaces {
		w.destroyWorkspaceInternal(workspaceID)
	}

	// Reset state
	w.workspaces = make(map[string]*TestWorkspace)
	w.isInitialized = false
	w.nextID = 1

	// Clear call tracking
	w.InitializeCalls = make([]context.Context, 0)
	w.CreateWorkspaceCalls = make([]WorkspaceConfig, 0)
	w.CleanupCalls = make([]interface{}, 0)

	// Reset function fields
	w.InitializeFunc = nil
	w.CreateWorkspaceFunc = nil
	w.CleanupFunc = nil
}

// IsInitialized returns whether the manager is initialized
func (w *TestWorkspaceManager) IsInitialized() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.isInitialized
}

// GetWorkspaceCount returns the number of active workspaces
func (w *TestWorkspaceManager) GetWorkspaceCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()

	count := 0
	for _, workspace := range w.workspaces {
		if workspace.IsActive {
			count++
		}
	}

	return count
}

// destroyWorkspaceInternal handles internal workspace destruction logic
func (w *TestWorkspaceManager) destroyWorkspaceInternal(workspaceID string) error {
	workspace, exists := w.workspaces[workspaceID]
	if !exists {
		return nil // Already destroyed
	}

	workspace.mutex.Lock()
	workspace.IsActive = false
	workspace.mutex.Unlock()

	// Clean up workspace files
	if err := os.RemoveAll(workspace.RootPath); err != nil {
		return fmt.Errorf("failed to remove workspace directory: %w", err)
	}

	delete(w.workspaces, workspaceID)

	return nil
}

// createSharedFiles creates shared files within the workspace
func (w *TestWorkspaceManager) createSharedFiles(workspace *TestWorkspace, sharedFiles map[string]string) error {
	for relativePath, content := range sharedFiles {
		fullPath := filepath.Join(workspace.RootPath, relativePath)

		// Create directory if needed
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", relativePath, err)
		}

		// Write file
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", relativePath, err)
		}

		workspace.TempFiles = append(workspace.TempFiles, fullPath)
	}

	return nil
}

// setupEnvironment configures environment variables for the workspace
func (w *TestWorkspaceManager) setupEnvironment(workspace *TestWorkspace, environment map[string]string) error {
	// Create environment file
	envFile := filepath.Join(workspace.RootPath, ".env")
	var envContent string

	for key, value := range environment {
		envContent += fmt.Sprintf("%s=%s\n", key, value)
	}

	if envContent != "" {
		if err := os.WriteFile(envFile, []byte(envContent), 0644); err != nil {
			return fmt.Errorf("failed to write environment file: %w", err)
		}

		workspace.TempFiles = append(workspace.TempFiles, envFile)
	}

	return nil
}

// Workspace methods

// AddProject adds a project to the workspace
func (ws *TestWorkspace) AddProject(project *TestProject) {
	ws.mutex.Lock()
	defer ws.mutex.Unlock()

	ws.Projects = append(ws.Projects, project)
	ws.LastUsed = time.Now()
}

// GetProjects returns all projects in the workspace
func (ws *TestWorkspace) GetProjects() []*TestProject {
	ws.mutex.RLock()
	defer ws.mutex.RUnlock()

	projects := make([]*TestProject, len(ws.Projects))
	copy(projects, ws.Projects)

	return projects
}

// GetProject retrieves a project by name
func (ws *TestWorkspace) GetProject(name string) (*TestProject, bool) {
	ws.mutex.RLock()
	defer ws.mutex.RUnlock()

	for _, project := range ws.Projects {
		if project.Name == name {
			return project, true
		}
	}

	return nil, false
}
