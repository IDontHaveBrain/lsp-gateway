package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/transport"
)

// MultiLanguageIntegrator provides unified interface for multi-language LSP operations
type MultiLanguageIntegrator struct {
	multiServerManager *MultiServerManager
	smartRouter        *SmartRouterImpl
	projectDetector    *ProjectLanguageScanner
	workspaceManager   *WorkspaceManager
	
	// Configuration
	config *config.GatewayConfig
	logger *log.Logger
	
	// Runtime state
	activeProjects   map[string]*MultiLanguageProjectInfo
	projectMutex     sync.RWMutex
	initialized      bool
}

// NewMultiLanguageIntegrator creates a new multi-language integrator
func NewMultiLanguageIntegrator(gatewayConfig *config.GatewayConfig, logger *log.Logger) *MultiLanguageIntegrator {
	integrator := &MultiLanguageIntegrator{
		config:         gatewayConfig,
		logger:         logger,
		activeProjects: make(map[string]*MultiLanguageProjectInfo),
		initialized:    false,
	}
	
	// Initialize components
	integrator.projectDetector = NewProjectLanguageScanner()
	integrator.projectDetector.OptimizeForLargeMonorepos()
	
	integrator.workspaceManager = NewWorkspaceManager(gatewayConfig, nil, logger)
	
	integrator.multiServerManager = NewMultiServerManager(gatewayConfig, logger)
	
	// Initialize smart router with project-aware routing if available
	if projectRouter, err := NewProjectAwareRouter(NewRouter(gatewayConfig.Servers), gatewayConfig, integrator.workspaceManager, logger); err == nil {
		integrator.smartRouter = NewSmartRouter(projectRouter, gatewayConfig, integrator.workspaceManager, &mcp.NullLogger{})
	}
	
	return integrator
}

// Initialize initializes all components of the multi-language system
func (mli *MultiLanguageIntegrator) Initialize(ctx context.Context) error {
	if mli.initialized {
		return nil
	}
	
	mli.logger.Printf("Initializing multi-language LSP Gateway system...")
	
	// Initialize multi-server manager
	if err := mli.multiServerManager.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize multi-server manager: %w", err)
	}
	
	// Start multi-server manager
	if err := mli.multiServerManager.Start(); err != nil {
		return fmt.Errorf("failed to start multi-server manager: %w", err)
	}
	
	// Initialize workspace manager
	if err := mli.workspaceManager.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize workspace manager: %w", err)
	}
	
	mli.initialized = true
	mli.logger.Printf("Multi-language LSP Gateway system initialized successfully")
	
	return nil
}

// DetectAndConfigureProject detects languages in a project and configures LSP servers
func (mli *MultiLanguageIntegrator) DetectAndConfigureProject(projectPath string) (*MultiLanguageProjectInfo, error) {
	mli.logger.Printf("Detecting languages in project: %s", projectPath)
	
	// Scan project for languages
	projectInfo, err := mli.projectDetector.ScanProjectComprehensive(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect project languages: %w", err)
	}
	
	mli.logger.Printf("Detected project type: %s with %d languages", 
		projectInfo.ProjectType, len(projectInfo.Languages))
	
	// Log detected languages
	for lang, ctx := range projectInfo.Languages {
		mli.logger.Printf("  - %s: %d files, priority %d, confidence %.2f", 
			lang, ctx.FileCount, ctx.Priority, ctx.Confidence)
	}
	
	// Store project info
	mli.projectMutex.Lock()
	mli.activeProjects[projectPath] = projectInfo
	mli.projectMutex.Unlock()
	
	// Configure workspace for multi-language support
	if err := mli.configureWorkspaceForProject(projectInfo); err != nil {
		mli.logger.Printf("Warning: failed to configure workspace for project %s: %v", projectPath, err)
	}
	
	return projectInfo, nil
}

// ProcessMultiLanguageRequest processes an LSP request with multi-language support
func (mli *MultiLanguageIntegrator) ProcessMultiLanguageRequest(ctx context.Context, request *LSPRequest) (*AggregatedResponse, error) {
	if !mli.initialized {
		return nil, fmt.Errorf("multi-language integrator not initialized")
	}
	
	// Determine project context
	projectPath := mli.extractProjectPath(request.URI)
	if projectPath == "" {
		return nil, fmt.Errorf("unable to determine project path from request")
	}
	
	// Get or detect project info
	projectInfo, err := mli.getProjectInfo(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get project info: %w", err)
	}
	
	// Route request using smart router
	if mli.smartRouter != nil {
		return mli.smartRouter.AggregateBroadcast(request)
	}
	
	// Fallback to direct server management
	return mli.processRequestDirect(ctx, request, projectInfo)
}

// ProcessCrossLanguageSymbolSearch performs symbol search across all languages in a project
func (mli *MultiLanguageIntegrator) ProcessCrossLanguageSymbolSearch(ctx context.Context, projectPath, query string) (*CrossLanguageSymbolResult, error) {
	projectInfo, err := mli.getProjectInfo(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get project info: %w", err)
	}
	
	result := &CrossLanguageSymbolResult{
		Query:     query,
		Languages: make(map[string]*LanguageSymbolResult),
		TotalSymbols: 0,
		ProcessingTime: 0,
	}
	
	startTime := time.Now()
	
	// Search symbols in parallel across all languages
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	for language, langCtx := range projectInfo.Languages {
		wg.Add(1)
		go func(lang string, ctx *LanguageContext) {
			defer wg.Done()
			
			if langResult, err := mli.searchSymbolsInLanguage(lang, query, ctx); err == nil {
				mu.Lock()
				result.Languages[lang] = langResult
				result.TotalSymbols += len(langResult.Symbols)
				mu.Unlock()
			}
		}(language, langCtx)
	}
	
	wg.Wait()
	result.ProcessingTime = time.Since(startTime)
	
	return result, nil
}

// GetProjectLanguageStatus returns status of all languages in a project
func (mli *MultiLanguageIntegrator) GetProjectLanguageStatus(projectPath string) (*ProjectLanguageStatus, error) {
	projectInfo, err := mli.getProjectInfo(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get project info: %w", err)
	}
	
	status := &ProjectLanguageStatus{
		ProjectPath:   projectPath,
		ProjectType:   projectInfo.ProjectType,
		TotalLanguages: len(projectInfo.Languages),
		Languages:     make(map[string]*LanguageStatus),
		LastUpdated:   time.Now(),
	}
	
	// Get status for each language
	for language, langCtx := range projectInfo.Languages {
		langStatus := &LanguageStatus{
			Language:      language,
			FileCount:     langCtx.FileCount,
			Priority:      langCtx.Priority,
			Confidence:    langCtx.Confidence,
			Framework:     langCtx.Framework,
			LSPServerName: langCtx.LSPServerName,
			ServerHealthy: false,
		}
		
		// Check server health
		if servers, err := mli.multiServerManager.GetHealthyServers(language); err == nil {
			langStatus.ServerHealthy = len(servers) > 0
			langStatus.ActiveServers = len(servers)
		}
		
		status.Languages[language] = langStatus
	}
	
	return status, nil
}

// GetMultiLanguageMetrics returns comprehensive metrics for multi-language operations
func (mli *MultiLanguageIntegrator) GetMultiLanguageMetrics() (*MultiLanguageMetrics, error) {
	if !mli.initialized {
		return nil, fmt.Errorf("integrator not initialized")
	}
	
	metrics := &MultiLanguageMetrics{
		TotalProjects:    len(mli.activeProjects),
		ActiveLanguages:  make(map[string]int),
		ServerMetrics:    mli.multiServerManager.GetMetrics(),
		RoutingMetrics:   nil,
		LastUpdated:      time.Now(),
	}
	
	if mli.smartRouter != nil {
		metrics.RoutingMetrics = mli.smartRouter.GetRoutingMetrics()
	}
	
	// Count languages across all projects
	mli.projectMutex.RLock()
	for _, projectInfo := range mli.activeProjects {
		for language := range projectInfo.Languages {
			metrics.ActiveLanguages[language]++
		}
	}
	mli.projectMutex.RUnlock()
	
	return metrics, nil
}

// Shutdown gracefully shuts down all components
func (mli *MultiLanguageIntegrator) Shutdown() error {
	mli.logger.Printf("Shutting down multi-language LSP Gateway system...")
	
	var errors []error
	
	// Stop multi-server manager
	if mli.multiServerManager != nil {
		if err := mli.multiServerManager.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop multi-server manager: %w", err))
		}
	}
	
	// Shutdown project detector
	if mli.projectDetector != nil {
		mli.projectDetector.Shutdown()
	}
	
	mli.initialized = false
	
	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors: %v", errors)
	}
	
	mli.logger.Printf("Multi-language LSP Gateway system shut down successfully")
	return nil
}

// Private helper methods

func (mli *MultiLanguageIntegrator) configureWorkspaceForProject(projectInfo *MultiLanguageProjectInfo) error {
	// Create workspace configuration for multi-language project
	workspaceConfig := &WorkspaceConfig{
		ID:           fmt.Sprintf("ml-%d", time.Now().Unix()),
		RootPath:     projectInfo.RootPath,
		ProjectType:  projectInfo.ProjectType,
		Languages:    make([]string, 0, len(projectInfo.Languages)),
		CreatedAt:    time.Now(),
	}
	
	for language := range projectInfo.Languages {
		workspaceConfig.Languages = append(workspaceConfig.Languages, language)
	}
	
	// Register workspace
	return mli.workspaceManager.CreateWorkspace(workspaceConfig)
}

func (mli *MultiLanguageIntegrator) getProjectInfo(projectPath string) (*MultiLanguageProjectInfo, error) {
	mli.projectMutex.RLock()
	projectInfo, exists := mli.activeProjects[projectPath]
	mli.projectMutex.RUnlock()
	
	if !exists {
		// Detect project on-demand
		return mli.DetectAndConfigureProject(projectPath)
	}
	
	return projectInfo, nil
}

func (mli *MultiLanguageIntegrator) extractProjectPath(fileURI string) string {
	if !strings.HasPrefix(fileURI, "file://") {
		return ""
	}
	
	path := strings.TrimPrefix(fileURI, "file://")
	
	// Find project root by traversing up directories
	for {
		dir := filepath.Dir(path)
		if dir == path { // Reached root
			break
		}
		
		// Check for common project markers
		markers := []string{
			"go.mod", "Cargo.toml", "package.json", "pom.xml", 
			"pyproject.toml", "setup.py", ".git",
		}
		
		for _, marker := range markers {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
		}
		
		path = dir
	}
	
	return filepath.Dir(strings.TrimPrefix(fileURI, "file://"))
}

func (mli *MultiLanguageIntegrator) processRequestDirect(ctx context.Context, request *LSPRequest, projectInfo *MultiLanguageProjectInfo) (*AggregatedResponse, error) {
	// Direct processing fallback when smart router is not available
	language := request.Language
	if language == "" {
		if lang, err := mli.extractLanguageFromURI(request.URI); err == nil {
			language = lang
		}
	}
	
	if language == "" {
		return nil, fmt.Errorf("unable to determine language for request")
	}
	
	// Get server for language
	servers, err := mli.multiServerManager.GetHealthyServers(language)
	if err != nil || len(servers) == 0 {
		return nil, fmt.Errorf("no healthy servers available for language %s", language)
	}
	
	// Execute request on first healthy server
	server := servers[0]
	startTime := time.Now()
	
	result, err := server.SendRequest(ctx, request.Method, request.Params)
	processingTime := time.Since(startTime)
	
	// Update server metrics
	mli.multiServerManager.UpdateServerMetrics(server.config.Name, processingTime, err == nil)
	
	return &AggregatedResponse{
		PrimaryResult:  result,
		Strategy:       SingleTargetWithFallback,
		ProcessingTime: processingTime,
		ServerCount:    1,
		Metadata: map[string]interface{}{
			"server_name":  server.config.Name,
			"language":     language,
			"success":      err == nil,
		},
	}, err
}

func (mli *MultiLanguageIntegrator) searchSymbolsInLanguage(language, query string, langCtx *LanguageContext) (*LanguageSymbolResult, error) {
	// Get servers for language
	servers, err := mli.multiServerManager.GetHealthyServers(language)
	if err != nil || len(servers) == 0 {
		return nil, fmt.Errorf("no healthy servers for language %s", language)
	}
	
	// Prepare workspace/symbol request
	symbolRequest := map[string]interface{}{
		"query": query,
	}
	
	// Execute request
	server := servers[0]
	result, err := server.SendRequest(context.Background(), "workspace/symbol", symbolRequest)
	if err != nil {
		return nil, err
	}
	
	// Parse symbols from result
	var symbols []json.RawMessage
	if err := json.Unmarshal(result, &symbols); err != nil {
		return nil, fmt.Errorf("failed to parse symbol results: %w", err)
	}
	
	return &LanguageSymbolResult{
		Language: language,
		Symbols:  symbols,
		Count:    len(symbols),
	}, nil
}

func (mli *MultiLanguageIntegrator) extractLanguageFromURI(fileURI string) (string, error) {
	if !strings.HasPrefix(fileURI, "file://") {
		return "", fmt.Errorf("invalid file URI")
	}
	
	path := strings.TrimPrefix(fileURI, "file://")
	ext := filepath.Ext(path)
	
	extToLang := map[string]string{
		".go":   "go",
		".py":   "python", 
		".js":   "javascript",
		".ts":   "typescript",
		".jsx":  "javascript",
		".tsx":  "typescript",
		".java": "java",
		".rs":   "rust",
	}
	
	if lang, exists := extToLang[ext]; exists {
		return lang, nil
	}
	
	return "", fmt.Errorf("unable to determine language from URI: %s", fileURI)
}

// Data structures for multi-language operations

type CrossLanguageSymbolResult struct {
	Query          string                            `json:"query"`
	Languages      map[string]*LanguageSymbolResult  `json:"languages"`
	TotalSymbols   int                              `json:"total_symbols"`
	ProcessingTime time.Duration                    `json:"processing_time"`
}

type LanguageSymbolResult struct {
	Language string            `json:"language"`
	Symbols  []json.RawMessage `json:"symbols"`
	Count    int               `json:"count"`
}

type ProjectLanguageStatus struct {
	ProjectPath    string                     `json:"project_path"`
	ProjectType    string                     `json:"project_type"`
	TotalLanguages int                        `json:"total_languages"`
	Languages      map[string]*LanguageStatus `json:"languages"`
	LastUpdated    time.Time                  `json:"last_updated"`
}

type LanguageStatus struct {
	Language      string  `json:"language"`
	FileCount     int     `json:"file_count"`
	Priority      int     `json:"priority"`
	Confidence    float64 `json:"confidence"`
	Framework     string  `json:"framework,omitempty"`
	LSPServerName string  `json:"lsp_server_name,omitempty"`
	ServerHealthy bool    `json:"server_healthy"`
	ActiveServers int     `json:"active_servers"`
}

type MultiLanguageMetrics struct {
	TotalProjects   int                    `json:"total_projects"`
	ActiveLanguages map[string]int         `json:"active_languages"`
	ServerMetrics   *ManagerMetrics        `json:"server_metrics"`
	RoutingMetrics  *RoutingMetrics        `json:"routing_metrics,omitempty"`
	LastUpdated     time.Time              `json:"last_updated"`
}

type WorkspaceConfig struct {
	ID          string    `json:"id"`
	RootPath    string    `json:"root_path"`
	ProjectType string    `json:"project_type"`
	Languages   []string  `json:"languages"`
	CreatedAt   time.Time `json:"created_at"`
}

// NullLogger implements a no-op logger for MCP integration
type NullLogger struct{}

func (nl *NullLogger) Debugf(format string, args ...interface{}) {}
func (nl *NullLogger) Printf(format string, args ...interface{}) {}
func (nl *NullLogger) Errorf(format string, args ...interface{}) {}