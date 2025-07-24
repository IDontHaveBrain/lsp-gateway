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

// Enhanced WorkspaceManager with Multi-Language Support
//
// This enhanced version provides:
// - Multi-language project detection using ProjectLanguageScanner
// - Language-specific working directories and workspace boundaries
// - Lazy initialization of language-specific LSP clients
// - Cross-language workspace support with reference resolution
// - Backward compatibility with existing single-language workspaces
//
// Key Features:
// - Uses MultiLanguageProjectInfo for comprehensive project analysis
// - Supports language-specific root paths for complex project structures
// - Maintains separate LSP clients for each language with proper lifecycle management
// - Provides cross-language reference resolution for polyglot projects
// - Integrates with the new ProjectLanguageScanner for accurate language detection

const (
	DefaultWorkspaceCleanupInterval = 30 * time.Minute
	DefaultWorkspaceTimeout         = 5 * time.Minute
	MaxConcurrentWorkspaces         = 50
)

// WorkspaceContextImpl implements the WorkspaceContext interface with multi-language support
type WorkspaceContextImpl struct {
	ID              string
	RootPath        string
	ProjectType     string
	ProjectName     string
	Languages       []string
	CreatedAt       time.Time
	LastAccessedAt  time.Time
	ClientInstances map[string]transport.LSPClient
	Config          *config.GatewayConfig
	IsInitialized   bool
	IsActiveFlag    bool

	// Multi-language support fields
	MultiLangInfo  *MultiLanguageProjectInfo
	LanguageRoots  map[string]string
	ActiveLanguage string
	LSPClients     map[string]transport.LSPClient // language -> client mapping

	// New multi-server field
	ServerManager *WorkspaceServerManager `json:"-"` // Don't serialize

	mu sync.RWMutex
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
	workspaces               map[string]*WorkspaceContextImpl
	pathToWorkspace          map[string]string
	config                   *config.GatewayConfig
	router                   *Router
	logger                   *mcp.StructuredLogger
	cleanupInterval          time.Duration
	workspaceTimeout         time.Duration
	maxWorkspaces            int
	globalMultiServerManager *MultiServerManager // Reference to global multi-server manager
	mu                       sync.RWMutex
	ctx                      context.Context
	cancel                   context.CancelFunc
	cleanupTicker            *time.Ticker
}

type WorkspaceDiscoveryResult struct {
	WorkspaceID   string
	RootPath      string
	ProjectType   string
	Languages     []string
	ConfigMarkers []string
	// Multi-language support fields
	MultiLangInfo    *MultiLanguageProjectInfo // Comprehensive project information
	LanguageRoots    map[string]string         // Language -> root path mapping
	DominantLanguage string                    // Primary language if detected
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

// NewWorkspaceManagerWithMultiServer creates a new workspace manager with multi-server support
func NewWorkspaceManagerWithMultiServer(config *config.GatewayConfig, router *Router, logger *mcp.StructuredLogger, globalMultiServerManager *MultiServerManager) *WorkspaceManager {
	ctx, cancel := context.WithCancel(context.Background())

	wm := &WorkspaceManager{
		workspaces:               make(map[string]*WorkspaceContextImpl),
		pathToWorkspace:          make(map[string]string),
		config:                   config,
		router:                   router,
		logger:                   logger,
		cleanupInterval:          DefaultWorkspaceCleanupInterval,
		workspaceTimeout:         DefaultWorkspaceTimeout,
		maxWorkspaces:            MaxConcurrentWorkspaces,
		globalMultiServerManager: globalMultiServerManager,
		ctx:                      ctx,
		cancel:                   cancel,
		cleanupTicker:            time.NewTicker(DefaultWorkspaceCleanupInterval),
	}

	go wm.startCleanupRoutine()

	return wm
}

// createWorkspaceServerManager creates a workspace server manager for a workspace
func (wm *WorkspaceManager) createWorkspaceServerManager(workspaceID string, workspace *WorkspaceContextImpl) error {
	if wm.globalMultiServerManager == nil {
		return fmt.Errorf("global multi-server manager not available")
	}

	serverManager := NewWorkspaceServerManager(workspaceID, wm.config, wm.globalMultiServerManager)

	if err := serverManager.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize workspace server manager: %w", err)
	}

	workspace.ServerManager = serverManager
	wm.logger.Infof("Created workspace server manager for workspace %s", workspaceID)
	return nil
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

	// Create workspace server manager if global multi-server manager is available
	if err := wm.createWorkspaceServerManager(workspace.ID, workspace); err != nil {
		wm.logger.Warnf("Failed to create workspace server manager for %s: %s", workspace.ID, err.Error())
		// Continue without multi-server support for backward compatibility
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

// GetLanguageSpecificClient returns or creates an LSP client for a specific language in a workspace
func (wm *WorkspaceManager) GetLanguageSpecificClient(workspaceID, language string) (transport.LSPClient, error) {
	wm.mu.RLock()
	workspace, exists := wm.workspaces[workspaceID]
	wm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("workspace not found: %s", workspaceID)
	}

	return workspace.getOrCreateLanguageClient(language, wm.config, wm.logger)
}

// SetActiveLanguage sets the active language for a workspace
func (wm *WorkspaceManager) SetActiveLanguage(workspaceID, language string) error {
	wm.mu.RLock()
	workspace, exists := wm.workspaces[workspaceID]
	wm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("workspace not found: %s", workspaceID)
	}

	return workspace.setActiveLanguage(language)
}

// GetWorkspaceLanguages returns all languages detected in a workspace
func (wm *WorkspaceManager) GetWorkspaceLanguages(workspaceID string) ([]string, error) {
	wm.mu.RLock()
	workspace, exists := wm.workspaces[workspaceID]
	wm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("workspace not found: %s", workspaceID)
	}

	return workspace.GetLanguages(), nil
}

// GetLanguageRoot returns the root path for a specific language in a workspace
func (wm *WorkspaceManager) GetLanguageRoot(workspaceID, language string) (string, error) {
	wm.mu.RLock()
	workspace, exists := wm.workspaces[workspaceID]
	wm.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("workspace not found: %s", workspaceID)
	}

	return workspace.getLanguageRoot(language), nil
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

	// Try comprehensive multi-language analysis first
	scanner := NewProjectLanguageScanner()
	multiLangInfo, err := scanner.ScanProject(rootPath)

	var projectType string
	var languages []string
	var configMarkers []string
	var languageRoots map[string]string
	var dominantLanguage string

	if err == nil && multiLangInfo != nil {
		// Use comprehensive analysis results
		projectType = multiLangInfo.ProjectType
		languages = multiLangInfo.GetPrimaryLanguages()
		if len(languages) == 0 {
			// If no primary languages, get all detected languages
			for lang := range multiLangInfo.Languages {
				languages = append(languages, lang)
			}
		}
		configMarkers = multiLangInfo.BuildFiles
		dominantLanguage = multiLangInfo.DominantLanguage

		// Build language roots map
		languageRoots = make(map[string]string)
		for lang, langCtx := range multiLangInfo.Languages {
			if langCtx.RootPath != "" {
				languageRoots[lang] = langCtx.RootPath
			} else {
				languageRoots[lang] = rootPath
			}
		}
	} else {
		// Fall back to simple analysis
		wm.logger.Warnf("Multi-language analysis failed, using simple analysis: %v", err)
		projectType, languages, configMarkers = wm.analyzeProjectSimple(rootPath)

		// Simple language roots mapping
		languageRoots = make(map[string]string)
		for _, lang := range languages {
			languageRoots[lang] = rootPath
		}
		if len(languages) > 0 {
			dominantLanguage = languages[0]
		}
	}

	workspaceID := fmt.Sprintf("workspace_%s_%d",
		strings.ReplaceAll(filepath.Base(rootPath), " ", "_"),
		time.Now().UnixNano())

	return &WorkspaceDiscoveryResult{
		WorkspaceID:      workspaceID,
		RootPath:         rootPath,
		ProjectType:      projectType,
		Languages:        languages,
		ConfigMarkers:    configMarkers,
		MultiLangInfo:    multiLangInfo,
		LanguageRoots:    languageRoots,
		DominantLanguage: dominantLanguage,
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
	// Try using the new ProjectLanguageScanner for comprehensive analysis
	scanner := NewProjectLanguageScanner()
	multiLangInfo, err := scanner.ScanProject(rootPath)
	if err != nil {
		wm.logger.Warnf("Multi-language scan failed, falling back to simple analysis: %s", err.Error())
		return wm.analyzeProjectSimple(rootPath)
	}

	// Extract information from MultiLanguageProjectInfo
	projectType := multiLangInfo.ProjectType
	languages := multiLangInfo.GetPrimaryLanguages()
	configMarkers := multiLangInfo.BuildFiles

	// If no primary languages found, get all languages
	if len(languages) == 0 {
		for lang := range multiLangInfo.Languages {
			languages = append(languages, lang)
		}
	}

	return projectType, languages, configMarkers
}

// analyzeProjectSimple provides fallback analysis using simple heuristics
func (wm *WorkspaceManager) analyzeProjectSimple(rootPath string) (string, []string, []string) {
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

		// Initialize multi-language support fields
		MultiLangInfo:  discovery.MultiLangInfo,
		LanguageRoots:  discovery.LanguageRoots,
		ActiveLanguage: discovery.DominantLanguage,
		LSPClients:     make(map[string]transport.LSPClient),
	}

	// Ensure language roots is initialized
	if workspace.LanguageRoots == nil {
		workspace.LanguageRoots = make(map[string]string)
		for _, lang := range workspace.Languages {
			workspace.LanguageRoots[lang] = discovery.RootPath
		}
	}

	// Ensure active language is set
	if workspace.ActiveLanguage == "" && len(workspace.Languages) > 0 {
		workspace.ActiveLanguage = workspace.Languages[0]
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
		if stopErr := client.Stop(); stopErr != nil {
			logger.Warnf("Failed to stop LSP client during cleanup: %v", stopErr)
		}
		return nil, fmt.Errorf("failed to initialize LSP client for language %s: %w", language, err)
	}

	wc.ClientInstances[language] = client

	logger.Infof("Created LSP client for workspace %s (language: %s) at %s", wc.ID, language, wc.RootPath)

	return client, nil
}

func (wc *WorkspaceContextImpl) initializeClient(client transport.LSPClient, logger *mcp.StructuredLogger) error {
	return wc.initializeClientWithRoot(client, wc.RootPath, logger)
}

// initializeClientWithRoot initializes an LSP client with a specific root path
func (wc *WorkspaceContextImpl) initializeClientWithRoot(client transport.LSPClient, rootPath string, logger *mcp.StructuredLogger) error {
	initParams := map[string]interface{}{
		"processId": nil,
		"rootPath":  rootPath,
		"rootUri":   fmt.Sprintf("file://%s", rootPath),
		"capabilities": map[string]interface{}{
			"workspace": map[string]interface{}{
				"applyEdit":              true,
				"workspaceEdit":          map[string]interface{}{"documentChanges": true},
				"didChangeConfiguration": map[string]interface{}{"dynamicRegistration": true},
				"didChangeWatchedFiles":  map[string]interface{}{"dynamicRegistration": true},
				"symbol":                 map[string]interface{}{"dynamicRegistration": true},
				"executeCommand":         map[string]interface{}{"dynamicRegistration": true},
			},
			"textDocument": map[string]interface{}{
				"publishDiagnostics": map[string]interface{}{"relatedInformation": true},
				"synchronization":    map[string]interface{}{"dynamicRegistration": true, "willSave": true, "willSaveWaitUntil": true, "didSave": true},
				"completion":         map[string]interface{}{"dynamicRegistration": true, "contextSupport": true},
				"hover":              map[string]interface{}{"dynamicRegistration": true, "contentFormat": []string{"markdown", "plaintext"}},
				"signatureHelp":      map[string]interface{}{"dynamicRegistration": true},
				"definition":         map[string]interface{}{"dynamicRegistration": true},
				"references":         map[string]interface{}{"dynamicRegistration": true},
				"documentHighlight":  map[string]interface{}{"dynamicRegistration": true},
				"documentSymbol":     map[string]interface{}{"dynamicRegistration": true},
				"codeAction":         map[string]interface{}{"dynamicRegistration": true},
				"codeLens":           map[string]interface{}{"dynamicRegistration": true},
				"formatting":         map[string]interface{}{"dynamicRegistration": true},
				"rangeFormatting":    map[string]interface{}{"dynamicRegistration": true},
				"onTypeFormatting":   map[string]interface{}{"dynamicRegistration": true},
				"rename":             map[string]interface{}{"dynamicRegistration": true},
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

	// Clean up workspace server manager first
	if wc.ServerManager != nil {
		if err := wc.ServerManager.Shutdown(); err != nil {
			errors = append(errors, fmt.Sprintf("ServerManager: %v", err))
		}
		wc.ServerManager = nil
	}

	// Clean up ClientInstances (legacy)
	for language, client := range wc.ClientInstances {
		if client.IsActive() {
			if err := client.Stop(); err != nil {
				errors = append(errors, fmt.Sprintf("ClientInstances[%s]: %v", language, err))
			}
		}
	}

	// Clean up LSPClients (new multi-language support)
	for language, client := range wc.LSPClients {
		if client != nil && client.IsActive() {
			if err := client.Stop(); err != nil {
				errors = append(errors, fmt.Sprintf("LSPClients[%s]: %v", language, err))
			}
		}
	}

	// Clear all client maps
	wc.ClientInstances = make(map[string]transport.LSPClient)
	wc.LSPClients = make(map[string]transport.LSPClient)

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

	// Include multi-language specific information
	info := map[string]interface{}{
		"id":              wc.ID,
		"root_path":       wc.RootPath,
		"project_type":    wc.ProjectType,
		"languages":       wc.Languages,
		"created_at":      wc.CreatedAt,
		"last_accessed":   wc.LastAccessedAt,
		"active_clients":  activeClients,
		"is_initialized":  wc.IsInitialized,
		"active_language": wc.ActiveLanguage,
		"language_roots":  wc.LanguageRoots,
	}

	// Add multi-language project info if available
	if wc.MultiLangInfo != nil {
		info["project_info"] = map[string]interface{}{
			"dominant_language": wc.MultiLangInfo.DominantLanguage,
			"total_files":       wc.MultiLangInfo.TotalFileCount,
			"scan_duration":     wc.MultiLangInfo.ScanDuration,
			"detected_at":       wc.MultiLangInfo.DetectedAt,
		}
	}

	return info
}

// Multi-language workspace context methods

// setActiveLanguage sets the active language for the workspace
func (wc *WorkspaceContextImpl) setActiveLanguage(language string) error {
	wc.mu.Lock()
	defer wc.mu.Unlock()

	// Check if language is supported
	supported := false
	for _, lang := range wc.Languages {
		if lang == language {
			supported = true
			break
		}
	}

	if !supported {
		return fmt.Errorf("language %s is not supported in this workspace", language)
	}

	wc.ActiveLanguage = language
	return nil
}

// getLanguageRoot returns the root path for a specific language
func (wc *WorkspaceContextImpl) getLanguageRoot(language string) string {
	wc.mu.RLock()
	defer wc.mu.RUnlock()

	if root, exists := wc.LanguageRoots[language]; exists {
		return root
	}
	return wc.RootPath // fallback to workspace root
}

// getOrCreateLanguageClient returns an existing language client or creates a new one
func (wc *WorkspaceContextImpl) getOrCreateLanguageClient(language string, config *config.GatewayConfig, logger *mcp.StructuredLogger) (transport.LSPClient, error) {
	wc.mu.Lock()
	defer wc.mu.Unlock()

	// Check if client already exists and is active
	if client, exists := wc.LSPClients[language]; exists && client.IsActive() {
		return client, nil
	}

	// Try ClientInstances for backward compatibility
	if client, exists := wc.ClientInstances[language]; exists && client.IsActive() {
		return client, nil
	}

	// Create new client
	client, err := wc.createLanguageSpecificClient(language, config, logger)
	if err != nil {
		return nil, err
	}

	// Store in both maps for compatibility
	wc.LSPClients[language] = client
	wc.ClientInstances[language] = client

	return client, nil
}

// createLanguageSpecificClient creates an LSP client for a specific language
func (wc *WorkspaceContextImpl) createLanguageSpecificClient(language string, config *config.GatewayConfig, logger *mcp.StructuredLogger) (transport.LSPClient, error) {
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

	// Initialize with language-specific root if available
	languageRoot := wc.getLanguageRoot(language)
	if err := wc.initializeClientWithRoot(client, languageRoot, logger); err != nil {
		client.Stop()
		return nil, fmt.Errorf("failed to initialize LSP client for language %s: %w", language, err)
	}

	logger.Infof("Created language-specific LSP client for workspace %s (language: %s, root: %s)", wc.ID, language, languageRoot)

	return client, nil
}

// ResolveCrossLanguageReference handles references that cross language boundaries
func (wc *WorkspaceContextImpl) ResolveCrossLanguageReference(fromLang, toLang, reference string) (string, error) {
	wc.mu.RLock()
	defer wc.mu.RUnlock()

	// For now, return a simple path resolution
	// This could be enhanced with more sophisticated cross-language reference resolution
	fromRoot := wc.getLanguageRoot(fromLang)
	toRoot := wc.getLanguageRoot(toLang)

	if fromRoot == toRoot {
		// Same workspace root, return reference as-is
		return reference, nil
	}

	// Try to resolve relative path from one language root to another
	relPath, err := filepath.Rel(fromRoot, filepath.Join(toRoot, reference))
	if err != nil {
		return reference, fmt.Errorf("failed to resolve cross-language reference from %s to %s: %w", fromLang, toLang, err)
	}

	return relPath, nil
}

// HasLanguage checks if the workspace supports a specific language
func (wc *WorkspaceContextImpl) HasLanguage(language string) bool {
	wc.mu.RLock()
	defer wc.mu.RUnlock()

	for _, lang := range wc.Languages {
		if lang == language {
			return true
		}
	}
	return false
}

// GetActiveLanguage returns the currently active language
func (wc *WorkspaceContextImpl) GetActiveLanguage() string {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	return wc.ActiveLanguage
}

// GetMultiLanguageInfo returns the multi-language project information
func (wc *WorkspaceContextImpl) GetMultiLanguageInfo() *MultiLanguageProjectInfo {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	return wc.MultiLangInfo
}

// IsMultiLanguage returns true if the workspace contains multiple languages
func (wc *WorkspaceContextImpl) IsMultiLanguage() bool {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	return len(wc.Languages) > 1
}

// ValidateMultiLanguageSupport validates that multi-language support is working correctly
func (wm *WorkspaceManager) ValidateMultiLanguageSupport() error {
	// Basic validation that new features are available
	testWorkspace := &WorkspaceContextImpl{
		ID:            "test",
		LanguageRoots: make(map[string]string),
		LSPClients:    make(map[string]transport.LSPClient),
		Languages:     []string{"go", "python"},
	}

	// Test language root functionality
	testWorkspace.LanguageRoots["go"] = "/test/go"
	testWorkspace.LanguageRoots["python"] = "/test/python"

	if root := testWorkspace.getLanguageRoot("go"); root != "/test/go" {
		return fmt.Errorf("language root retrieval failed: expected /test/go, got %s", root)
	}

	// Test language support check
	if !testWorkspace.HasLanguage("python") {
		return fmt.Errorf("language support check failed for python")
	}

	if testWorkspace.HasLanguage("java") {
		return fmt.Errorf("language support check should have failed for java")
	}

	// Test multi-language detection
	if !testWorkspace.IsMultiLanguage() {
		return fmt.Errorf("multi-language detection failed")
	}

	wm.logger.Info("Multi-language workspace support validation passed")
	return nil
}
