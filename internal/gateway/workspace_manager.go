package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/transport"
	"lsp-gateway/mcp"
)

const (
	DefaultWorkspaceCleanupInterval = 30 * time.Minute
	DefaultWorkspaceTimeout         = 5 * time.Minute
	MaxConcurrentWorkspaces        = 50
)

// WorkspaceContextImpl implements the WorkspaceContext interface
type WorkspaceContextImpl struct {
	ID               string
	RootPath         string
	ProjectType      string
	ProjectName      string
	Languages        []string
	CreatedAt        time.Time
	LastAccessedAt   time.Time
	ClientInstances  map[string]transport.LSPClient
	Config           *config.GatewayConfig
	IsInitialized    bool
	IsActiveFlag     bool
	mu               sync.RWMutex
}

// Interface methods for WorkspaceContext interface
func (wc *WorkspaceContextImpl) GetID() string {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	return wc.ID
}

func (wc *WorkspaceContextImpl) GetRootPath() string {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	return wc.RootPath
}

func (wc *WorkspaceContextImpl) GetProjectType() string {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	return wc.ProjectType
}

func (wc *WorkspaceContextImpl) GetProjectName() string {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	return wc.ProjectName
}

func (wc *WorkspaceContextImpl) GetLanguages() []string {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	return wc.Languages
}

func (wc *WorkspaceContextImpl) IsActive() bool {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	return wc.IsActiveFlag
}

type WorkspaceManager struct {
	workspaces      map[string]*WorkspaceContextImpl
	pathToWorkspace map[string]string
	config          *config.GatewayConfig
	router          *Router
	logger          *mcp.StructuredLogger
	cleanupInterval time.Duration
	workspaceTimeout time.Duration
	maxWorkspaces   int
	mu              sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
	cleanupTicker   *time.Ticker
}

type WorkspaceDiscoveryResult struct {
	WorkspaceID   string
	RootPath      string
	ProjectType   string
	Languages     []string
	ConfigMarkers []string
}

func NewWorkspaceManager(config *config.GatewayConfig, router *Router, logger *mcp.StructuredLogger) *WorkspaceManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	wm := &WorkspaceManager{
		workspaces:       make(map[string]*WorkspaceContextImpl),
		pathToWorkspace:  make(map[string]string),
		config:           config,
		router:           router,
		logger:           logger,
		cleanupInterval:  DefaultWorkspaceCleanupInterval,
		workspaceTimeout: DefaultWorkspaceTimeout,
		maxWorkspaces:    MaxConcurrentWorkspaces,
		ctx:              ctx,
		cancel:           cancel,
		cleanupTicker:    time.NewTicker(DefaultWorkspaceCleanupInterval),
	}

	go wm.startCleanupRoutine()
	
	return wm
}

func (wm *WorkspaceManager) GetOrCreateWorkspace(fileURI string) (*WorkspaceContextImpl, error) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	parsedURI, err := url.Parse(fileURI)
	if err != nil {
		return nil, fmt.Errorf("invalid file URI: %w", err)
	}

	filePath := parsedURI.Path
	if strings.HasPrefix(parsedURI.Scheme, "file") {
		filePath = parsedURI.Path
	}

	workspaceID := wm.findExistingWorkspace(filePath)
	if workspaceID != "" {
		workspace := wm.workspaces[workspaceID]
		workspace.updateLastAccessed()
		return workspace, nil
	}

	discovery, err := wm.discoverWorkspace(filePath)
	if err != nil {
		return nil, fmt.Errorf("workspace discovery failed: %w", err)
	}

	if len(wm.workspaces) >= wm.maxWorkspaces {
		if err := wm.evictOldestWorkspace(); err != nil {
			wm.logger.Warnf("Failed to evict oldest workspace: %s", err.Error())
		}
	}

	workspace, err := wm.createWorkspace(discovery)
	if err != nil {
		return nil, fmt.Errorf("workspace creation failed: %w", err)
	}

	wm.workspaces[workspace.ID] = workspace
	wm.pathToWorkspace[workspace.RootPath] = workspace.ID

	wm.logger.Infof("Created new workspace: %s at %s (type: %s)", workspace.ID, workspace.RootPath, workspace.ProjectType)

	return workspace, nil
}

func (wm *WorkspaceManager) GetWorkspaceByID(workspaceID string) (*WorkspaceContextImpl, bool) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	workspace, exists := wm.workspaces[workspaceID]
	if exists {
		workspace.updateLastAccessed()
	}
	return workspace, exists
}

func (wm *WorkspaceManager) GetLSPClient(workspaceID, language string) (transport.LSPClient, error) {
	wm.mu.RLock()
	workspace, exists := wm.workspaces[workspaceID]
	wm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("workspace not found: %s", workspaceID)
	}

	return workspace.getLSPClient(language, wm.config, wm.logger)
}

func (wm *WorkspaceManager) CleanupWorkspace(workspaceID string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	workspace, exists := wm.workspaces[workspaceID]
	if !exists {
		return fmt.Errorf("workspace not found: %s", workspaceID)
	}

	if err := workspace.cleanup(); err != nil {
		wm.logger.Errorf("Error cleaning up workspace %s: %s", workspaceID, err.Error())
		return err
	}

	delete(wm.workspaces, workspaceID)
	delete(wm.pathToWorkspace, workspace.RootPath)

	wm.logger.Infof("Cleaned up workspace: %s at %s", workspaceID, workspace.RootPath)

	return nil
}

func (wm *WorkspaceManager) GetAllWorkspaces() []*WorkspaceContextImpl {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	workspaces := make([]*WorkspaceContextImpl, 0, len(wm.workspaces))
	for _, workspace := range wm.workspaces {
		workspaces = append(workspaces, workspace)
	}
	return workspaces
}

func (wm *WorkspaceManager) Shutdown() error {
	wm.cancel()
	wm.cleanupTicker.Stop()

	wm.mu.Lock()
	defer wm.mu.Unlock()

	var errors []string
	for workspaceID := range wm.workspaces {
		if err := wm.workspaces[workspaceID].cleanup(); err != nil {
			errors = append(errors, fmt.Sprintf("workspace %s: %v", workspaceID, err))
		}
	}

	wm.workspaces = make(map[string]*WorkspaceContextImpl)
	wm.pathToWorkspace = make(map[string]string)

	if len(errors) > 0 {
		return fmt.Errorf("errors during shutdown: %s", strings.Join(errors, "; "))
	}

	wm.logger.Info("WorkspaceManager shutdown completed")
	return nil
}

func (wm *WorkspaceManager) findExistingWorkspace(filePath string) string {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return ""
	}

	for rootPath, workspaceID := range wm.pathToWorkspace {
		if strings.HasPrefix(absPath, rootPath) {
			return workspaceID
		}
	}

	return ""
}

func (wm *WorkspaceManager) discoverWorkspace(filePath string) (*WorkspaceDiscoveryResult, error) {
	rootPath := wm.findProjectRoot(filePath)
	if rootPath == "" {
		rootPath = filepath.Dir(filePath)
	}

	projectType, languages, configMarkers := wm.analyzeProject(rootPath)
	
	workspaceID := fmt.Sprintf("workspace_%s_%d", 
		strings.ReplaceAll(filepath.Base(rootPath), " ", "_"), 
		time.Now().UnixNano())

	return &WorkspaceDiscoveryResult{
		WorkspaceID:   workspaceID,
		RootPath:      rootPath,
		ProjectType:   projectType,
		Languages:     languages,
		ConfigMarkers: configMarkers,
	}, nil
}

func (wm *WorkspaceManager) findProjectRoot(filePath string) string {
	rootMarkers := []string{
		".git", ".hg", ".svn",
		"go.mod", "package.json", "pyproject.toml", "setup.py",
		"pom.xml", "build.gradle", "Cargo.toml",
		"tsconfig.json", ".vscode", ".idea",
	}

	currentDir := filepath.Dir(filePath)
	for {
		for _, marker := range rootMarkers {
			markerPath := filepath.Join(currentDir, marker)
			if wm.pathExists(markerPath) {
				return currentDir
			}
		}

		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			break
		}
		currentDir = parent
	}

	return ""
}

func (wm *WorkspaceManager) analyzeProject(rootPath string) (string, []string, []string) {
	var projectType string
	var languages []string
	var configMarkers []string

	checks := map[string]struct {
		markers   []string
		languages []string
	}{
		"go":         {[]string{"go.mod", "go.sum"}, []string{"go"}},
		"node":       {[]string{"package.json", "node_modules"}, []string{"javascript", "typescript"}},
		"python":     {[]string{"pyproject.toml", "setup.py", "requirements.txt"}, []string{"python"}},
		"java":       {[]string{"pom.xml", "build.gradle", "gradlew"}, []string{"java"}},
		"typescript": {[]string{"tsconfig.json"}, []string{"typescript"}},
	}

	for pType, check := range checks {
		for _, marker := range check.markers {
			markerPath := filepath.Join(rootPath, marker)
			if wm.pathExists(markerPath) {
				if projectType == "" {
					projectType = pType
				}
				configMarkers = append(configMarkers, marker)
				for _, lang := range check.languages {
					if !wm.containsString(languages, lang) {
						languages = append(languages, lang)
					}
				}
				break
			}
		}
	}

	if projectType == "" {
		projectType = "generic"
	}
	if len(languages) == 0 {
		languages = []string{"text"}
	}

	return projectType, languages, configMarkers
}

func (wm *WorkspaceManager) createWorkspace(discovery *WorkspaceDiscoveryResult) (*WorkspaceContextImpl, error) {
	workspace := &WorkspaceContextImpl{
		ID:              discovery.WorkspaceID,
		RootPath:        discovery.RootPath,
		ProjectType:     discovery.ProjectType,
		ProjectName:     filepath.Base(discovery.RootPath),
		Languages:       discovery.Languages,
		CreatedAt:       time.Now(),
		LastAccessedAt:  time.Now(),
		ClientInstances: make(map[string]transport.LSPClient),
		Config:          wm.config,
		IsInitialized:   true,
		IsActiveFlag:    true,
	}

	return workspace, nil
}

func (wm *WorkspaceManager) evictOldestWorkspace() error {
	var oldestWorkspace *WorkspaceContextImpl
	var oldestID string

	for id, workspace := range wm.workspaces {
		if oldestWorkspace == nil || workspace.LastAccessedAt.Before(oldestWorkspace.LastAccessedAt) {
			oldestWorkspace = workspace
			oldestID = id
		}
	}

	if oldestID != "" {
		return wm.cleanupWorkspaceUnsafe(oldestID)
	}

	return nil
}

func (wm *WorkspaceManager) cleanupWorkspaceUnsafe(workspaceID string) error {
	workspace, exists := wm.workspaces[workspaceID]
	if !exists {
		return fmt.Errorf("workspace not found: %s", workspaceID)
	}

	if err := workspace.cleanup(); err != nil {
		return err
	}

	delete(wm.workspaces, workspaceID)
	delete(wm.pathToWorkspace, workspace.RootPath)
	return nil
}

func (wm *WorkspaceManager) startCleanupRoutine() {
	for {
		select {
		case <-wm.ctx.Done():
			return
		case <-wm.cleanupTicker.C:
			wm.performPeriodicCleanup()
		}
	}
}

func (wm *WorkspaceManager) performPeriodicCleanup() {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	cutoff := time.Now().Add(-wm.workspaceTimeout)
	var toCleanup []string

	for id, workspace := range wm.workspaces {
		if workspace.LastAccessedAt.Before(cutoff) {
			toCleanup = append(toCleanup, id)
		}
	}

	for _, id := range toCleanup {
		if err := wm.cleanupWorkspaceUnsafe(id); err != nil {
			wm.logger.Warnf("Failed to cleanup workspace %s during periodic cleanup: %s", id, err.Error())
		} else {
			wm.logger.Debugf("Cleaned up inactive workspace: %s", id)
		}
	}
}

func (wm *WorkspaceManager) pathExists(path string) bool {
	_, err := filepath.Abs(path)
	return err == nil
}

func (wm *WorkspaceManager) containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func (wc *WorkspaceContextImpl) updateLastAccessed() {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	wc.LastAccessedAt = time.Now()
}

func (wc *WorkspaceContextImpl) getLSPClient(language string, config *config.GatewayConfig, logger *mcp.StructuredLogger) (transport.LSPClient, error) {
	wc.mu.Lock()
	defer wc.mu.Unlock()

	if client, exists := wc.ClientInstances[language]; exists && client.IsActive() {
		return client, nil
	}

	serverConfig, err := config.GetServerByLanguage(language)
	if err != nil || serverConfig == nil {
		return nil, fmt.Errorf("no server configuration found for language: %s", language)
	}

	clientConfig := transport.ClientConfig{
		Command:   serverConfig.Command,
		Args:      serverConfig.Args,
		Transport: serverConfig.Transport,
	}

	client, err := transport.NewLSPClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create LSP client for language %s: %w", language, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start LSP client for language %s: %w", language, err)
	}

	if err := wc.initializeClient(client, logger); err != nil {
		client.Stop()
		return nil, fmt.Errorf("failed to initialize LSP client for language %s: %w", language, err)
	}

	wc.ClientInstances[language] = client

	logger.Infof("Created LSP client for workspace %s (language: %s) at %s", wc.ID, language, wc.RootPath)

	return client, nil
}

func (wc *WorkspaceContextImpl) initializeClient(client transport.LSPClient, logger *mcp.StructuredLogger) error {
	initParams := map[string]interface{}{
		"processId": nil,
		"rootPath":  wc.RootPath,
		"rootUri":   fmt.Sprintf("file://%s", wc.RootPath),
		"capabilities": map[string]interface{}{
			"workspace": map[string]interface{}{
				"applyEdit":         true,
				"workspaceEdit":     map[string]interface{}{"documentChanges": true},
				"didChangeConfiguration": map[string]interface{}{"dynamicRegistration": true},
				"didChangeWatchedFiles":  map[string]interface{}{"dynamicRegistration": true},
				"symbol":                 map[string]interface{}{"dynamicRegistration": true},
				"executeCommand":         map[string]interface{}{"dynamicRegistration": true},
			},
			"textDocument": map[string]interface{}{
				"publishDiagnostics": map[string]interface{}{"relatedInformation": true},
				"synchronization":     map[string]interface{}{"dynamicRegistration": true, "willSave": true, "willSaveWaitUntil": true, "didSave": true},
				"completion":          map[string]interface{}{"dynamicRegistration": true, "contextSupport": true},
				"hover":               map[string]interface{}{"dynamicRegistration": true, "contentFormat": []string{"markdown", "plaintext"}},
				"signatureHelp":       map[string]interface{}{"dynamicRegistration": true},
				"definition":          map[string]interface{}{"dynamicRegistration": true},
				"references":          map[string]interface{}{"dynamicRegistration": true},
				"documentHighlight":   map[string]interface{}{"dynamicRegistration": true},
				"documentSymbol":      map[string]interface{}{"dynamicRegistration": true},
				"codeAction":          map[string]interface{}{"dynamicRegistration": true},
				"codeLens":            map[string]interface{}{"dynamicRegistration": true},
				"formatting":          map[string]interface{}{"dynamicRegistration": true},
				"rangeFormatting":     map[string]interface{}{"dynamicRegistration": true},
				"onTypeFormatting":    map[string]interface{}{"dynamicRegistration": true},
				"rename":              map[string]interface{}{"dynamicRegistration": true},
			},
		},
		"trace": "off",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	initResponse, err := client.SendRequest(ctx, "initialize", initParams)
	if err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}

	var initResult map[string]interface{}
	if err := json.Unmarshal(initResponse, &initResult); err != nil {
		logger.Warnf("Failed to parse initialize response: %s", err.Error())
	}

	if err := client.SendNotification(ctx, "initialized", map[string]interface{}{}); err != nil {
		return fmt.Errorf("initialized notification failed: %w", err)
	}

	return nil
}

func (wc *WorkspaceContextImpl) cleanup() error {
	wc.mu.Lock()
	defer wc.mu.Unlock()

	var errors []string

	for language, client := range wc.ClientInstances {
		if client.IsActive() {
			if err := client.Stop(); err != nil {
				errors = append(errors, fmt.Sprintf("language %s: %v", language, err))
			}
		}
	}

	wc.ClientInstances = make(map[string]transport.LSPClient)

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

func (wc *WorkspaceContextImpl) GetInfo() map[string]interface{} {
	wc.mu.RLock()
	defer wc.mu.RUnlock()

	activeClients := make([]string, 0, len(wc.ClientInstances))
	for language, client := range wc.ClientInstances {
		if client.IsActive() {
			activeClients = append(activeClients, language)
		}
	}

	return map[string]interface{}{
		"id":              wc.ID,
		"root_path":       wc.RootPath,
		"project_type":    wc.ProjectType,
		"languages":       wc.Languages,
		"created_at":      wc.CreatedAt,
		"last_accessed":   wc.LastAccessedAt,
		"active_clients":  activeClients,
		"is_initialized":  wc.IsInitialized,
	}
}