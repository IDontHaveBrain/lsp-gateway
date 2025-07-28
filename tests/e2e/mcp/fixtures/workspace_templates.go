package fixtures

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"lsp-gateway/tests/e2e/mcp/helpers"
	"lsp-gateway/tests/e2e/mcp/types"
)

// SetupStep represents a single setup operation for workspace configuration
type SetupStep struct {
	Name        string
	Description string
	Action      func(*TestWorkspace) error
	Required    bool
	Timeout     time.Duration
}

// TeardownStep represents a cleanup operation for workspace destruction
type TeardownStep struct {
	Name        string
	Description string  
	Action      func(*TestWorkspace) error
	Required    bool
}

// WorkspaceTemplate defines a complete workspace configuration template
type WorkspaceTemplate struct {
	Name            string
	Description     string
	RepositoryURL   string
	CommitHash      string
	Language        string
	ProjectType     string
	LSPServerConfig *helpers.EnhancedLSPServerConfig
	TestData        *FatihColorTestData
	SetupSteps      []SetupStep
	TeardownSteps   []TeardownStep
	
	// Template metadata
	Version    string
	CreatedAt  time.Time
	Tags       []string
	Maintainer string
}

// TestWorkspace represents an enhanced workspace with template integration
type TestWorkspace struct {
	*types.TestWorkspace
	
	// Template integration
	Template        *WorkspaceTemplate
	TemplateVersion string
	ConfigManager   *WorkspaceConfigManager
	
	// Enhanced test data
	TestDataCache   map[string]interface{}
	ValidationState WorkspaceValidationState
}

// WorkspaceValidationState tracks the validation status of workspace components
type WorkspaceValidationState struct {
	RepositoryValid bool
	LSPConfigValid  bool
	TestDataValid   bool
	EnvironmentReady bool
	LastValidated   time.Time
	ValidationErrors []string
}

// WorkspaceConfigManager manages workspace templates and configurations
type WorkspaceConfigManager struct {
	templates       map[string]*WorkspaceTemplate
	defaultTemplate *WorkspaceTemplate
	configCache     map[string]*helpers.EnhancedLSPServerConfig
	mutex           sync.RWMutex
	logger          *log.Logger
	
	// Component managers
	repositoryCloner *helpers.RepositoryCloner
	workspaceManager *helpers.TestWorkspaceManager
	lspConfigGen     *helpers.LSPConfigGenerator
}

// WorkspaceConfiguration represents exportable workspace configuration
type WorkspaceConfiguration struct {
	WorkspaceID     string                         `json:"workspace_id"`
	Template        string                         `json:"template"`
	LSPConfig       *helpers.EnhancedLSPServerConfig `json:"lsp_config"`
	RepositoryInfo  RepositoryMetadata             `json:"repository_info"`
	TestDataSummary TestDataSummary                `json:"test_data_summary"`
	CreatedAt       time.Time                      `json:"created_at"`
	ValidationState WorkspaceValidationState       `json:"validation_state"`
}

// TestDataSummary provides a summary of available test data
type TestDataSummary struct {
	DocumentSymbolsCount  int      `json:"document_symbols_count"`
	WorkspaceSymbolsCount int      `json:"workspace_symbols_count"`
	ReferencesCount       int      `json:"references_count"`
	HoversCount           int      `json:"hovers_count"`
	CompletionsCount      int      `json:"completions_count"`
	SupportedFiles        []string `json:"supported_files"`
}

// NewWorkspaceConfigManager creates a new workspace configuration manager
func NewWorkspaceConfigManager() (*WorkspaceConfigManager, error) {
	manager := &WorkspaceConfigManager{
		templates:   make(map[string]*WorkspaceTemplate),
		configCache: make(map[string]*helpers.EnhancedLSPServerConfig),
		logger:      log.New(os.Stdout, "[WorkspaceConfigManager] ", log.LstdFlags),
	}
	
	// Initialize component managers
	workspaceManager, err := helpers.NewTestWorkspaceManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace manager: %w", err)
	}
	
	manager.workspaceManager = workspaceManager
	manager.lspConfigGen = helpers.NewLSPConfigGenerator()
	
	// Register default templates
	if err := manager.registerDefaultTemplates(); err != nil {
		return nil, fmt.Errorf("failed to register default templates: %w", err)
	}
	
	manager.logger.Printf("Workspace configuration manager initialized with %d templates", len(manager.templates))
	return manager, nil
}

// GetFatihColorTemplate returns the comprehensive fatih/color workspace template
func GetFatihColorTemplate() *WorkspaceTemplate {
	testData := GetFatihColorTestData()
	
	template := &WorkspaceTemplate{
		Name:          "fatih-color",
		Description:   "Complete workspace template for fatih/color Go library testing",
		RepositoryURL: "https://github.com/fatih/color.git",
		CommitHash:    "4c05561a8fbfd21922e4908479e63b48b677a61f", 
		Language:      "go",
		ProjectType:   "library",
		TestData:      testData,
		Version:       "1.0.0",
		CreatedAt:     time.Now(),
		Tags:          []string{"go", "library", "color", "terminal"},
		Maintainer:    "MCP E2E Test Suite",
	}
	
	// Setup LSP configuration
	template.LSPServerConfig = createFatihColorLSPConfig()
	
	// Define setup steps
	template.SetupSteps = []SetupStep{
		{
			Name:        "clone-repository",
			Description: "Clone fatih/color repository to workspace",
			Action:      setupCloneRepository,
			Required:    true,
			Timeout:     2 * time.Minute,
		},
		{
			Name:        "configure-lsp",
			Description: "Configure and validate LSP server settings",
			Action:      setupConfigureLSP,
			Required:    true,
			Timeout:     30 * time.Second,
		},
		{
			Name:        "populate-test-data",
			Description: "Load and validate test fixture data",
			Action:      setupPopulateTestData,
			Required:    true,
			Timeout:     10 * time.Second,
		},
		{
			Name:        "validate-environment",
			Description: "Validate Go environment and dependencies",
			Action:      setupValidateEnvironment,
			Required:    true,
			Timeout:     45 * time.Second,
		},
		{
			Name:        "optimize-performance",
			Description: "Apply performance optimizations for testing",
			Action:      setupOptimizePerformance,
			Required:    false,
			Timeout:     15 * time.Second,
		},
	}
	
	// Define teardown steps
	template.TeardownSteps = []TeardownStep{
		{
			Name:        "cleanup-cache",
			Description: "Clean up Go module and build caches",
			Action:      teardownCleanupCache,
			Required:    false,
		},
		{
			Name:        "remove-workspace",
			Description: "Remove workspace directory and files",
			Action:      teardownRemoveWorkspace,
			Required:    true,
		},
	}
	
	return template
}

// NewWorkspaceFromTemplate creates a new workspace instance from a template
func NewWorkspaceFromTemplate(template *WorkspaceTemplate) (*TestWorkspace, error) {
	if template == nil {
		return nil, fmt.Errorf("template cannot be nil")
	}
	
	// Create workspace manager
	manager, err := helpers.NewTestWorkspaceManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace manager: %w", err)
	}
	
	// Generate unique workspace ID
	workspaceID := fmt.Sprintf("%s-%d", template.Name, time.Now().Unix())
	
	// Create base workspace
	baseWorkspace, err := manager.CreateWorkspace(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to create base workspace: %w", err)
	}
	
	// Create enhanced workspace
	workspace := &TestWorkspace{
		TestWorkspace:   baseWorkspace,
		Template:        template,
		TemplateVersion: template.Version,
		TestDataCache:   make(map[string]interface{}),
		ValidationState: WorkspaceValidationState{
			LastValidated: time.Now(),
			ValidationErrors: make([]string, 0),
		},
	}
	
	// Execute setup steps
	for _, step := range template.SetupSteps {
		if err := executeSetupStep(workspace, step); err != nil {
			// Cleanup on failure
			if cleanupErr := workspace.CleanupFunc(); cleanupErr != nil {
				log.Printf("Warning: cleanup failed after setup error: %v", cleanupErr)
			}
			return nil, fmt.Errorf("setup step '%s' failed: %w", step.Name, err)
		}
	}
	
	// Populate test data from template
	workspace.ExpectedSymbols = convertToWorkspaceSymbols(template.TestData.WorkspaceSymbols)
	workspace.ExpectedReferences = convertToWorkspaceReferences(template.TestData.References)
	workspace.ExpectedHovers = convertToWorkspaceHovers(template.TestData.Hovers)
	
	// Cache test data for quick access
	workspace.TestDataCache["document_symbols"] = template.TestData.DocumentSymbols
	workspace.TestDataCache["completions"] = template.TestData.Completions
	workspace.TestDataCache["definitions"] = template.TestData.Definitions
	
	// Mark as initialized
	workspace.IsInitialized = true
	workspace.ValidationState.RepositoryValid = true
	workspace.ValidationState.LSPConfigValid = true
	workspace.ValidationState.TestDataValid = true
	workspace.ValidationState.EnvironmentReady = true
	
	return workspace, nil
}

// ValidateTemplate validates a workspace template for correctness
func ValidateTemplate(template *WorkspaceTemplate) error {
	if template == nil {
		return fmt.Errorf("template cannot be nil")
	}
	
	// Validate required fields
	if template.Name == "" {
		return fmt.Errorf("template name cannot be empty")
	}
	if template.RepositoryURL == "" {
		return fmt.Errorf("repository URL cannot be empty")
	}
	if template.CommitHash == "" {
		return fmt.Errorf("commit hash cannot be empty")
	}
	if template.Language == "" {
		return fmt.Errorf("language cannot be empty")
	}
	
	// Validate LSP configuration
	if template.LSPServerConfig == nil {
		return fmt.Errorf("LSP server configuration cannot be nil")
	}
	
	// Validate test data
	if template.TestData == nil {
		return fmt.Errorf("test data cannot be nil")
	}
	
	// Validate test data integrity
	if validationErrors := template.TestData.ValidateTestData(); len(validationErrors) > 0 {
		return fmt.Errorf("test data validation failed: %v", validationErrors)
	}
	
	// Validate setup steps
	for i, step := range template.SetupSteps {
		if step.Name == "" {
			return fmt.Errorf("setup step %d has empty name", i)
		}
		if step.Action == nil {
			return fmt.Errorf("setup step %d (%s) has nil action", i, step.Name)
		}
		if step.Timeout <= 0 {
			return fmt.Errorf("setup step %d (%s) has invalid timeout", i, step.Name)
		}
	}
	
	return nil
}

// GetAvailableTemplates returns all registered workspace templates
func (wcm *WorkspaceConfigManager) GetAvailableTemplates() []*WorkspaceTemplate {
	wcm.mutex.RLock()
	defer wcm.mutex.RUnlock()
	
	templates := make([]*WorkspaceTemplate, 0, len(wcm.templates))
	for _, template := range wcm.templates {
		templates = append(templates, template)
	}
	
	return templates
}

// CreateCompleteTestWorkspace creates a fully configured test workspace
func CreateCompleteTestWorkspace(workspaceID string) (*TestWorkspace, error) {
	// Get the fatih/color template
	template := GetFatihColorTemplate()
	
	// Validate template
	if err := ValidateTemplate(template); err != nil {
		return nil, fmt.Errorf("template validation failed: %w", err)
	}
	
	// Create workspace from template
	workspace, err := NewWorkspaceFromTemplate(template)
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace from template: %w", err)
	}
	
	// Update workspace ID if specified
	if workspaceID != "" {
		workspace.ID = workspaceID
	}
	
	return workspace, nil
}

// SetupMCPTestEnvironment creates and configures the complete MCP test environment
func SetupMCPTestEnvironment() (*WorkspaceConfigManager, error) {
	manager, err := NewWorkspaceConfigManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace config manager: %w", err)
	}
	
	// Pre-warm configuration cache
	template := GetFatihColorTemplate()
	if err := manager.CacheTemplateConfig(template); err != nil {
		manager.logger.Printf("Warning: failed to cache template config: %v", err)
	}
	
	manager.logger.Printf("MCP test environment ready with %d templates", len(manager.templates))
	return manager, nil
}

// RunWorkspaceHealthCheck performs comprehensive health validation
func RunWorkspaceHealthCheck(workspace *TestWorkspace) error {
	if workspace == nil {
		return fmt.Errorf("workspace cannot be nil")
	}
	
	var errors []string
	
	// Check base workspace health
	if workspace.TestWorkspace == nil {
		errors = append(errors, "base workspace is nil")
	} else {
		// Check workspace directory exists
		if _, err := os.Stat(workspace.RootPath); err != nil {
			errors = append(errors, fmt.Sprintf("workspace root path inaccessible: %v", err))
		}
		
		// Check repository structure
		if err := validateRepositoryStructure(workspace.RootPath); err != nil {
			errors = append(errors, fmt.Sprintf("repository structure invalid: %v", err))
		}
	}
	
	// Check template integration
	if workspace.Template == nil {
		errors = append(errors, "workspace template is nil")
	}
	
	// Check LSP configuration
	if workspace.LSPConfig == nil {
		errors = append(errors, "LSP configuration is nil")
	}
	
	// Check test data availability
	if len(workspace.ExpectedSymbols) == 0 {
		errors = append(errors, "no expected symbols loaded")
	}
	
	if len(errors) > 0 {
		// Update validation state
		workspace.ValidationState.ValidationErrors = errors
		workspace.ValidationState.LastValidated = time.Now()
		workspace.ValidationState.RepositoryValid = false
		workspace.ValidationState.LSPConfigValid = false
		workspace.ValidationState.TestDataValid = false
		workspace.ValidationState.EnvironmentReady = false
		
		return fmt.Errorf("workspace health check failed: %v", errors)
	}
	
	// Update validation state on success
	workspace.ValidationState.ValidationErrors = []string{}
	workspace.ValidationState.LastValidated = time.Now()
	workspace.ValidationState.RepositoryValid = true
	workspace.ValidationState.LSPConfigValid = true
	workspace.ValidationState.TestDataValid = true
	workspace.ValidationState.EnvironmentReady = true
	
	return nil
}

// ExportWorkspaceConfig exports workspace configuration to JSON
func ExportWorkspaceConfig(workspace *TestWorkspace) (string, error) {
	if workspace == nil {
		return "", fmt.Errorf("workspace cannot be nil")
	}
	
	// Create test data summary
	summary := TestDataSummary{
		DocumentSymbolsCount:  len(workspace.Template.TestData.DocumentSymbols),
		WorkspaceSymbolsCount: len(workspace.Template.TestData.WorkspaceSymbols),
		ReferencesCount:       len(workspace.Template.TestData.References),
		HoversCount:           len(workspace.Template.TestData.Hovers),
		CompletionsCount:      len(workspace.Template.TestData.Completions),
		SupportedFiles:        workspace.Template.TestData.RepositoryInfo.Files,
	}
	
	config := WorkspaceConfiguration{
		WorkspaceID:     workspace.ID,
		Template:        workspace.Template.Name,
		LSPConfig:       workspace.Template.LSPServerConfig,
		RepositoryInfo:  workspace.Template.TestData.RepositoryInfo,
		TestDataSummary: summary,
		CreatedAt:       workspace.CreatedAt,
		ValidationState: workspace.ValidationState,
	}
	
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal workspace config: %w", err)
	}
	
	return string(data), nil
}

// RunFullWorkspaceValidation performs comprehensive workspace validation
func RunFullWorkspaceValidation(workspace *TestWorkspace) error {
	if workspace == nil {
		return fmt.Errorf("workspace cannot be nil")
	}
	
	// Step 1: Health check
	if err := RunWorkspaceHealthCheck(workspace); err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	
	// Step 2: LSP server connection test
	if err := TestLSPServerConnection(workspace); err != nil {
		return fmt.Errorf("LSP server connection test failed: %w", err)
	}
	
	// Step 3: Test data integrity verification
	if err := VerifyTestDataIntegrity(workspace); err != nil {
		return fmt.Errorf("test data integrity verification failed: %w", err)
	}
	
	// Step 4: Validate workspace setup
	if err := ValidateWorkspaceSetup(workspace); err != nil {
		return fmt.Errorf("workspace setup validation failed: %w", err)
	}
	
	return nil
}

// Support functions for template operations

func createFatihColorLSPConfig() *helpers.EnhancedLSPServerConfig {
	config := &helpers.EnhancedLSPServerConfig{
		ServerName:    "gopls-fatih-color",
		Language:      "go",
		Command:       "gopls",
		Args:          []string{"serve"},
		Transport:     "stdio",
		RootMarkers:   []string{"go.mod", "go.sum", ".git"},
		GoplsSettings: make(map[string]interface{}),
		BuildFlags:    []string{"-tags=test", "-mod=readonly"},
		Environment: map[string]string{
			"GO111MODULE": "on",
			"GOPROXY":     "direct",
			"CGO_ENABLED": "1",
		},
		InitTimeout:     30 * time.Second,
		ResponseTimeout: 10 * time.Second,
		MemoryLimit:     512 * 1024 * 1024, // 512MB
		TestMode:        true,
		DisableFeatures: []string{"diagnostics", "codelens"},
		EnableFeatures:  []string{"definition", "references", "hover", "completion", "documentsymbol"},
	}
	
	// Apply test-specific optimizations
	config.GoplsSettings["memoryMode"] = "DegradeClosed"
	config.GoplsSettings["diagnosticsDelay"] = "5s"
	config.GoplsSettings["completionBudget"] = "200ms"
	config.GoplsSettings["staticcheck"] = false
	
	return config
}

func executeSetupStep(workspace *TestWorkspace, step SetupStep) error {
	ctx, cancel := context.WithTimeout(context.Background(), step.Timeout)
	defer cancel()
	
	log.Printf("Executing setup step: %s", step.Name)
	
	done := make(chan error, 1)
	go func() {
		done <- step.Action(workspace)
	}()
	
	select {
	case err := <-done:
		if err != nil && step.Required {
			return fmt.Errorf("required setup step failed: %w", err)
		}
		if err != nil {
			log.Printf("Warning: optional setup step '%s' failed: %v", step.Name, err)
		}
		return nil
	case <-ctx.Done():
		if step.Required {
			return fmt.Errorf("required setup step timed out after %v", step.Timeout)
		}
		log.Printf("Warning: optional setup step '%s' timed out", step.Name)
		return nil
	}
}

// Setup step implementations

func setupCloneRepository(workspace *TestWorkspace) error {
	// Repository cloning is handled by the base workspace creation
	// This step validates the cloning was successful
	if _, err := os.Stat(filepath.Join(workspace.RootPath, "go.mod")); err != nil {
		return fmt.Errorf("go.mod not found, repository cloning may have failed: %w", err)
	}
	
	if _, err := os.Stat(filepath.Join(workspace.RootPath, "color.go")); err != nil {
		return fmt.Errorf("color.go not found, repository cloning may have failed: %w", err)
	}
	
	return nil
}

func setupConfigureLSP(workspace *TestWorkspace) error {
	if workspace.Template == nil || workspace.Template.LSPServerConfig == nil {
		return fmt.Errorf("template LSP configuration is nil")
	}
	
	// Update workspace root in LSP config
	workspace.Template.LSPServerConfig.WorkspaceRoot = workspace.RootPath
	
	// Create workspace-specific cache directories
	goCacheDir := filepath.Join(workspace.RootPath, ".gocache")
	goModCacheDir := filepath.Join(workspace.RootPath, ".gomodcache")
	
	if err := os.MkdirAll(goCacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create GOCACHE directory: %w", err)
	}
	if err := os.MkdirAll(goModCacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create GOMODCACHE directory: %w", err)
	}
	
	workspace.Template.LSPServerConfig.Environment["GOCACHE"] = goCacheDir
	workspace.Template.LSPServerConfig.Environment["GOMODCACHE"] = goModCacheDir
	
	return nil
}

func setupPopulateTestData(workspace *TestWorkspace) error {
	if workspace.Template == nil || workspace.Template.TestData == nil {
		return fmt.Errorf("template test data is nil")
	}
	
	// Validate test data
	if errors := workspace.Template.TestData.ValidateTestData(); len(errors) > 0 {
		return fmt.Errorf("test data validation failed: %v", errors)
	}
	
	// Cache test data for performance
	workspace.TestDataCache["test_data"] = workspace.Template.TestData
	
	return nil
}

func setupValidateEnvironment(workspace *TestWorkspace) error {
	// Validate Go environment
	// Check if workspace can build
	return helpers.NewRepositoryCloner("", "").VerifyGoModule(workspace.RootPath)
}

func setupOptimizePerformance(workspace *TestWorkspace) error {
	// Apply performance optimizations
	if workspace.Template != nil && workspace.Template.LSPServerConfig != nil {
		workspace.Template.LSPServerConfig.GoplsSettings["memoryMode"] = "DegradeClosed"
		workspace.Template.LSPServerConfig.GoplsSettings["diagnosticsDelay"] = "10s"
	}
	return nil
}

// Teardown step implementations

func teardownCleanupCache(workspace *TestWorkspace) error {
	// Clean up Go caches
	goCacheDir := filepath.Join(workspace.RootPath, ".gocache")
	goModCacheDir := filepath.Join(workspace.RootPath, ".gomodcache")
	
	if err := os.RemoveAll(goCacheDir); err != nil {
		log.Printf("Warning: failed to remove GOCACHE: %v", err)
	}
	if err := os.RemoveAll(goModCacheDir); err != nil {
		log.Printf("Warning: failed to remove GOMODCACHE: %v", err)
	}
	
	return nil
}

func teardownRemoveWorkspace(workspace *TestWorkspace) error {
	if workspace.CleanupFunc != nil {
		return workspace.CleanupFunc()
	}
	return nil
}

// Validation helper functions

func ValidateWorkspaceSetup(workspace *TestWorkspace) error {
	if workspace == nil {
		return fmt.Errorf("workspace is nil")
	}
	
	// Check if workspace directory exists and is accessible
	if _, err := os.Stat(workspace.RootPath); err != nil {
		return fmt.Errorf("workspace root path not accessible: %w", err)
	}
	
	// Check if required files exist
	requiredFiles := []string{"go.mod", "color.go"}
	for _, file := range requiredFiles {
		filePath := filepath.Join(workspace.RootPath, file)
		if _, err := os.Stat(filePath); err != nil {
			return fmt.Errorf("required file not found: %s", file)
		}
	}
	
	return nil
}

func TestLSPServerConnection(workspace *TestWorkspace) error {
	// This is a placeholder for LSP server connection testing
	// In a real implementation, you would:
	// 1. Start an LSP server instance
	// 2. Send initialization request
	// 3. Validate response
	// 4. Clean up connection
	
	if workspace.Template == nil || workspace.Template.LSPServerConfig == nil {
		return fmt.Errorf("LSP configuration not available")
	}
	
	// For now, just validate the configuration
	gen := helpers.NewLSPConfigGenerator()
	return gen.ValidateLSPConfig(workspace.Template.LSPServerConfig)
}

func VerifyTestDataIntegrity(workspace *TestWorkspace) error {
	if workspace.Template == nil || workspace.Template.TestData == nil {
		return fmt.Errorf("test data not available")
	}
	
	// Validate test data structure
	testData := workspace.Template.TestData
	if len(testData.DocumentSymbols) == 0 {
		return fmt.Errorf("no document symbols in test data")
	}
	
	if len(testData.WorkspaceSymbols) == 0 {
		return fmt.Errorf("no workspace symbols in test data")
	}
	
	// Run built-in validation
	if errors := testData.ValidateTestData(); len(errors) > 0 {
		return fmt.Errorf("test data validation failed: %v", errors)
	}
	
	return nil
}

func validateRepositoryStructure(rootPath string) error {
	// Check for basic Go project structure
	requiredPaths := []string{
		filepath.Join(rootPath, "go.mod"),
		filepath.Join(rootPath, "color.go"),
	}
	
	for _, path := range requiredPaths {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("required path not found: %s", path)
		}
	}
	
	return nil
}

// Conversion helpers for test data

func convertToWorkspaceSymbols(symbols []ExpectedSymbol) []types.ExpectedSymbol {
	result := make([]types.ExpectedSymbol, len(symbols))
	for i, symbol := range symbols {
		result[i] = types.ExpectedSymbol{
			Name:      symbol.Name,
			Kind:      fmt.Sprintf("%d", symbol.Kind),
			Location:  symbol.File,
			Line:      symbol.Line,
			Character: symbol.Character,
		}
	}
	return result
}

func convertToWorkspaceReferences(references map[string][]ExpectedReference) []types.ExpectedReference {
	var result []types.ExpectedReference
	for symbol, refs := range references {
		for _, ref := range refs {
			for _, loc := range ref.Locations {
				result = append(result, types.ExpectedReference{
					Symbol:    symbol,
					FilePath:  ref.File,
					Line:      loc.Line,
					Character: loc.Character,
				})
			}
		}
	}
	return result
}

func convertToWorkspaceHovers(hovers map[string][]ExpectedHover) []types.ExpectedHover {
	var result []types.ExpectedHover
	for _, hoverList := range hovers {
		for _, hover := range hoverList {
			result = append(result, types.ExpectedHover{
				Position: types.LSPPosition{
					Line:      hover.Position.Line,
					Character: hover.Position.Character,
				},
				ExpectedText: hover.Content,
				FilePath:     hover.File,
			})
		}
	}
	return result
}

// Manager helper methods

func (wcm *WorkspaceConfigManager) registerDefaultTemplates() error {
	// Register fatih/color template
	template := GetFatihColorTemplate()
	if err := ValidateTemplate(template); err != nil {
		return fmt.Errorf("default template validation failed: %w", err)
	}
	
	wcm.templates[template.Name] = template
	wcm.defaultTemplate = template
	
	wcm.logger.Printf("Registered default template: %s", template.Name)
	return nil
}

func (wcm *WorkspaceConfigManager) CacheTemplateConfig(template *WorkspaceTemplate) error {
	if template == nil || template.LSPServerConfig == nil {
		return fmt.Errorf("invalid template for caching")
	}
	
	wcm.mutex.Lock()
	defer wcm.mutex.Unlock()
	
	wcm.configCache[template.Name] = template.LSPServerConfig
	wcm.logger.Printf("Cached LSP configuration for template: %s", template.Name)
	return nil
}

// ExampleMCPWorkspaceSetup demonstrates complete integration usage
func ExampleMCPWorkspaceSetup() (*TestWorkspace, error) {
	// Step 1: Get template
	template := GetFatihColorTemplate()
	
	// Step 2: Validate template
	if err := ValidateTemplate(template); err != nil {
		return nil, fmt.Errorf("template validation failed: %w", err)
	}
	
	// Step 3: Create workspace from template
	workspace, err := NewWorkspaceFromTemplate(template)
	if err != nil {
		return nil, fmt.Errorf("workspace creation failed: %w", err)
	}
	
	// Step 4: Populate with test data
	workspace.ExpectedSymbols = convertToWorkspaceSymbols(template.TestData.WorkspaceSymbols)
	workspace.ExpectedReferences = convertToWorkspaceReferences(template.TestData.References)
	workspace.ExpectedHovers = convertToWorkspaceHovers(template.TestData.Hovers)
	
	// Step 5: Final validation
	if err := RunFullWorkspaceValidation(workspace); err != nil {
		// Clean up on failure
		if workspace.CleanupFunc != nil {
			workspace.CleanupFunc()
		}
		return nil, fmt.Errorf("workspace validation failed: %w", err)
	}
	
	return workspace, nil
}