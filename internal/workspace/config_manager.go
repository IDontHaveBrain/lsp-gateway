package workspace

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/project"
	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/setup"

	"github.com/goccy/go-yaml"
)

const (
	WorkspaceConfigFileName = "workspace.yaml"
	WorkspaceStateFileName  = "state.json"
	WorkspaceLogDirName     = "logs"
	WorkspaceCacheDirName   = "cache"
	WorkspaceIndexDirName   = "index"
)

type WorkspaceConfigManager interface {
	LoadWorkspaceConfig(workspaceRoot string) (*WorkspaceConfig, error)
	GenerateWorkspaceConfig(workspaceRoot string, projectContext *project.ProjectContext) error
	GenerateSubProjectConfigs(workspaceRoot string, subProjects []*project.ProjectContext) (map[string]*SubProjectInfo, error)
	GetSubProjectConfig(workspaceRoot, subProjectID string) (*SubProjectInfo, error)
	GetWorkspaceConfigPath(workspaceRoot string) string
	GetWorkspaceDirectory(workspaceRoot string) string
	ValidateWorkspaceConfig(config *WorkspaceConfig) error
	ValidateSubProjectConfig(config *SubProjectInfo) error
	LoadWithHierarchy(workspaceRoot string, globalConfigPath string) (*WorkspaceConfig, error)
	CreateWorkspaceDirectories(workspaceRoot string) error
	CleanupWorkspace(workspaceRoot string) error
}

type WorkspaceInfo struct {
	WorkspaceID      string                 `yaml:"workspace_id" json:"workspace_id"`
	Name             string                 `yaml:"name,omitempty" json:"name,omitempty"`
	RootPath         string                 `yaml:"root_path" json:"root_path"`
	ProjectType      string                 `yaml:"project_type" json:"project_type"`
	Languages        []string               `yaml:"languages" json:"languages"`
	CreatedAt        time.Time              `yaml:"created_at" json:"created_at"`
	LastUpdated      time.Time              `yaml:"last_updated" json:"last_updated"`
	Version          string                 `yaml:"version" json:"version"`
	Hash             string                 `yaml:"hash" json:"hash"`
	Metadata         map[string]interface{} `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

type SubProjectInfo struct {
	ID              string            `yaml:"id" json:"id"`
	Name            string            `yaml:"name" json:"name"`
	RelativePath    string            `yaml:"relative_path" json:"relative_path"`
	AbsolutePath    string            `yaml:"absolute_path" json:"absolute_path"`
	ProjectType     string            `yaml:"project_type" json:"project_type"`
	Languages       []string          `yaml:"languages" json:"languages"`
	WorkspaceFolder string            `yaml:"workspace_folder" json:"workspace_folder"`
	ResourceQuota   *ResourceQuota    `yaml:"resource_quota,omitempty" json:"resource_quota,omitempty"`
	MarkerFiles     []string          `yaml:"marker_files,omitempty" json:"marker_files,omitempty"`
	Dependencies    map[string]string `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	RequiredServers []string          `yaml:"required_servers,omitempty" json:"required_servers,omitempty"`
}

type ResourceQuota struct {
	MaxMemoryMB     int `yaml:"max_memory_mb,omitempty" json:"max_memory_mb,omitempty"`
	MaxConcurrency  int `yaml:"max_concurrency,omitempty" json:"max_concurrency,omitempty"`
	MaxCacheSize    int `yaml:"max_cache_size,omitempty" json:"max_cache_size,omitempty"`
}

type WorkspaceConfig struct {
	Workspace    WorkspaceInfo                    `yaml:"workspace" json:"workspace"`
	SubProjects  []*SubProjectInfo               `yaml:"sub_projects,omitempty" json:"sub_projects,omitempty"`
	Servers      map[string]*config.ServerConfig `yaml:"servers" json:"servers"`
	Performance  config.PerformanceConfiguration `yaml:"performance_config" json:"performance_config"`
	Cache        config.CachingConfiguration     `yaml:"cache" json:"cache"`
	Logging      LoggingConfig                   `yaml:"logging" json:"logging"`
	Directories  WorkspaceDirectories            `yaml:"directories" json:"directories"`
}

type LoggingConfig struct {
	Level      string `yaml:"level,omitempty" json:"level,omitempty"`
	OutputFile string `yaml:"output_file,omitempty" json:"output_file,omitempty"`
	MaxSize    int    `yaml:"max_size_mb,omitempty" json:"max_size_mb,omitempty"`
	MaxBackups int    `yaml:"max_backups,omitempty" json:"max_backups,omitempty"`
	MaxAge     int    `yaml:"max_age_days,omitempty" json:"max_age_days,omitempty"`
}

type WorkspaceDirectories struct {
	Root   string `yaml:"root" json:"root"`
	Config string `yaml:"config" json:"config"`
	Cache  string `yaml:"cache" json:"cache"`
	Logs   string `yaml:"logs" json:"logs"`
	Index  string `yaml:"index" json:"index"`
	State  string `yaml:"state" json:"state"`
}

type DefaultWorkspaceConfigManager struct {
	logger           *setup.SetupLogger
	baseConfigDir    string
	configGenerator  *config.ConfigGenerator
	projectDetector  project.ProjectDetector
}

func NewWorkspaceConfigManager() WorkspaceConfigManager {
	return NewWorkspaceConfigManagerWithOptions(&WorkspaceConfigManagerOptions{})
}

type WorkspaceConfigManagerOptions struct {
	BaseConfigDir   string
	Logger          *setup.SetupLogger
	ProjectDetector project.ProjectDetector
}

func NewWorkspaceConfigManagerWithOptions(opts *WorkspaceConfigManagerOptions) WorkspaceConfigManager {
	if opts == nil {
		opts = &WorkspaceConfigManagerOptions{}
	}

	logger := opts.Logger
	if logger == nil {
		logger = setup.NewSetupLogger(&setup.SetupLoggerConfig{
			Component: "workspace-config-manager",
			Level:     setup.LogLevelInfo,
		})
	}

	baseConfigDir := opts.BaseConfigDir
	if baseConfigDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			logger.WithError(err).Warn("Could not determine home directory, using current directory")
			baseConfigDir = "."
		} else {
			baseConfigDir = filepath.Join(homeDir, ".lspg")
		}
	}

	projectDetector := opts.ProjectDetector
	if projectDetector == nil {
		projectDetector = project.NewProjectDetector()
	}

	return &DefaultWorkspaceConfigManager{
		logger:          logger,
		baseConfigDir:   baseConfigDir,
		configGenerator: config.NewConfigGenerator(),
		projectDetector: projectDetector,
	}
}

func (wcm *DefaultWorkspaceConfigManager) LoadWorkspaceConfig(workspaceRoot string) (*WorkspaceConfig, error) {
	configPath := wcm.GetWorkspaceConfigPath(workspaceRoot)
	
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("workspace configuration not found: %s", configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read workspace configuration: %w", err)
	}

	var wsConfig WorkspaceConfig
	if err := yaml.Unmarshal(data, &wsConfig); err != nil {
		return nil, fmt.Errorf("failed to parse workspace configuration: %w", err)
	}

	wcm.logger.WithField("config_path", configPath).Info("Loaded workspace configuration")
	return &wsConfig, nil
}

func (wcm *DefaultWorkspaceConfigManager) LoadWithHierarchy(workspaceRoot string, globalConfigPath string) (*WorkspaceConfig, error) {
	var baseConfig *config.GatewayConfig
	var err error

	if globalConfigPath != "" {
		baseConfig, err = config.LoadConfig(globalConfigPath)
		if err != nil {
			wcm.logger.WithError(err).Warn("Failed to load global configuration, using defaults")
			baseConfig = &config.GatewayConfig{}
		}
	} else {
		baseConfig = &config.GatewayConfig{}
	}

	workspaceConfig, err := wcm.LoadWorkspaceConfig(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to load workspace configuration: %w", err)
	}

	mergedConfig := wcm.mergeConfigurations(baseConfig, workspaceConfig)

	wcm.applyEnvironmentOverrides(mergedConfig)

	if err := wcm.ValidateWorkspaceConfig(mergedConfig); err != nil {
		return nil, fmt.Errorf("workspace configuration validation failed: %w", err)
	}

	return mergedConfig, nil
}

func (wcm *DefaultWorkspaceConfigManager) GenerateWorkspaceConfig(workspaceRoot string, projectContext *project.ProjectContext) error {
	if projectContext == nil {
		return fmt.Errorf("project context cannot be nil")
	}

	workspaceID := wcm.generateWorkspaceID(workspaceRoot)
	workspaceHash := wcm.generateWorkspaceHash(workspaceRoot)
	
	workspaceDirectories := wcm.createWorkspaceDirectoryStructure(workspaceRoot)

	if err := wcm.CreateWorkspaceDirectories(workspaceRoot); err != nil {
		return fmt.Errorf("failed to create workspace directories: %w", err)
	}

	// 1. Detect sub-projects using WorkspaceDetector
	workspaceDetector := NewWorkspaceDetector()
	workspaceContext, err := workspaceDetector.DetectWorkspaceAt(workspaceRoot)
	if err != nil {
		wcm.logger.WithError(err).Warn("Failed to detect sub-projects, falling back to single-project mode")
		workspaceContext = &WorkspaceContext{
			ID:           workspaceID,
			Root:         workspaceRoot,
			SubProjects:  []*DetectedSubProject{},
			ProjectPaths: make(map[string]*DetectedSubProject),
			Languages:    projectContext.Languages,
			Hash:         workspaceHash,
			CreatedAt:    time.Now(),
			ProjectType:  projectContext.ProjectType,
		}
	}

	// 2. Convert DetectedSubProject to SubProject, then to ProjectContext for sub-project configuration
	var subProjectContexts []*project.ProjectContext
	if len(workspaceContext.SubProjects) > 0 {
		// First convert DetectedSubProject to SubProject
		subProjects := wcm.convertDetectedSubProjectsToSubProjects(workspaceContext.SubProjects)
		// Then convert SubProject to ProjectContext
		subProjectContexts = wcm.convertSubProjectsToProjectContexts(subProjects, workspaceRoot)
	}

	// 3. Generate sub-project configurations
	var subProjectConfigs map[string]*SubProjectInfo
	if len(subProjectContexts) > 0 {
		subProjectConfigs, err = wcm.GenerateSubProjectConfigs(workspaceRoot, subProjectContexts)
		if err != nil {
			wcm.logger.WithError(err).Warn("Failed to generate sub-project configurations, using empty sub-projects")
			subProjectConfigs = make(map[string]*SubProjectInfo)
		}
	} else {
		subProjectConfigs = make(map[string]*SubProjectInfo)
	}

	// 4. Aggregate workspace information from all sub-projects
	aggregatedInfo := wcm.aggregateWorkspaceInfo(workspaceContext, projectContext, subProjectContexts)
	
	workspaceInfo := WorkspaceInfo{
		WorkspaceID: workspaceID,
		Name:        filepath.Base(workspaceRoot),
		RootPath:    workspaceRoot,
		ProjectType: aggregatedInfo.ProjectType,
		Languages:   aggregatedInfo.Languages,
		CreatedAt:   time.Now(),
		LastUpdated: time.Now(),
		Version:     "1.0.0",
		Hash:        workspaceHash,
		Metadata:    aggregatedInfo.Metadata,
	}

	// 5. Enhanced server configuration with sub-projects
	serverConfigs, err := wcm.generateServerConfigurations(projectContext, subProjectConfigs)
	if err != nil {
		return fmt.Errorf("failed to generate server configurations: %w", err)
	}

	// 6. Build enhanced WorkspaceConfig with sub-projects
	workspaceConfig := &WorkspaceConfig{
		Workspace:   workspaceInfo,
		SubProjects: wcm.convertSubProjectConfigsToInfoSlice(subProjectConfigs),
		Servers:     serverConfigs,
		Performance: wcm.generatePerformanceConfig(projectContext),
		Cache:       wcm.generateCacheConfig(projectContext),
		Logging:     wcm.generateLoggingConfig(workspaceDirectories),
		Directories: workspaceDirectories,
	}

	configPath := wcm.GetWorkspaceConfigPath(workspaceRoot)
	if err := wcm.saveWorkspaceConfig(workspaceConfig, configPath); err != nil {
		return fmt.Errorf("failed to save workspace configuration: %w", err)
	}

	wcm.logger.WithFields(map[string]interface{}{
		"workspace_root":        workspaceRoot,
		"config_path":           configPath,
		"project_type":          aggregatedInfo.ProjectType,
		"languages":             aggregatedInfo.Languages,
		"total_sub_projects":    len(subProjectConfigs),
		"detected_sub_projects": len(workspaceContext.SubProjects),
	}).Info("Generated workspace configuration with sub-project support")

	return nil
}

// convertDetectedSubProjectsToSubProjects converts DetectedSubProject to SubProject
func (wcm *DefaultWorkspaceConfigManager) convertDetectedSubProjectsToSubProjects(detectedSubProjects []*DetectedSubProject) []*SubProject {
	subProjects := make([]*SubProject, 0, len(detectedSubProjects))
	
	for _, detected := range detectedSubProjects {
		if detected == nil {
			continue
		}
		
		subProject := &SubProject{
			ID:           detected.ID,
			Name:         detected.Name,
			Root:         detected.AbsolutePath,
			RelativePath: detected.RelativePath,
			ProjectType:  detected.ProjectType,
			DetectedAt:   time.Now(),
			LastModified: time.Now(),
			Languages:    append([]string(nil), detected.Languages...), // Copy slice
			ConfigFiles:  append([]string(nil), detected.MarkerFiles...), // Copy marker files as config files
			LSPClients:   make(map[string]*ClientRef),
		}
		
		// Set primary language from languages if available
		if len(detected.Languages) > 0 {
			subProject.PrimaryLang = detected.Languages[0]
		} else {
			subProject.PrimaryLang = detected.ProjectType
		}
		
		subProjects = append(subProjects, subProject)
	}
	
	wcm.logger.WithFields(map[string]interface{}{
		"input_detected_projects": len(detectedSubProjects),
		"output_sub_projects":     len(subProjects),
	}).Debug("Converted DetectedSubProject to SubProject")
	
	return subProjects
}

// convertSubProjectsToProjectContexts converts sub-projects to project contexts
func (wcm *DefaultWorkspaceConfigManager) convertSubProjectsToProjectContexts(subProjects []*SubProject, workspaceRoot string) []*project.ProjectContext {
	projectContexts := make([]*project.ProjectContext, 0, len(subProjects))
	
	for _, subProject := range subProjects {
		if subProject == nil {
			continue
		}
		
		// Build required servers list based on languages
		requiredServers := make([]string, 0, len(subProject.Languages))
		for _, lang := range subProject.Languages {
			switch lang {
			case "go":
				requiredServers = append(requiredServers, "gopls")
			case "python":
				requiredServers = append(requiredServers, "pyright")
			case "javascript", "typescript":
				requiredServers = append(requiredServers, "typescript-language-server")
			case "java":
				requiredServers = append(requiredServers, "jdtls")
			case "rust":
				requiredServers = append(requiredServers, "rust-analyzer")
			}
		}
		
		// Determine primary language from PrimaryLang or first in languages list
		primaryLanguage := subProject.PrimaryLang
		if primaryLanguage == "" && len(subProject.Languages) > 0 {
			primaryLanguage = subProject.Languages[0]
		}
		if primaryLanguage == "" {
			primaryLanguage = subProject.ProjectType
		}
		
		projectContext := &project.ProjectContext{
			ProjectType:      subProject.ProjectType,
			RootPath:         subProject.Root,
			WorkspaceRoot:    workspaceRoot,
			Languages:        append([]string(nil), subProject.Languages...), // Copy slice
			PrimaryLanguage:  primaryLanguage,
			LanguageVersions: make(map[string]string),
			ModuleName:       subProject.Name,
			DisplayName:      subProject.Name,
			Description:      fmt.Sprintf("%s project at %s", subProject.ProjectType, subProject.RelativePath),
			Dependencies:     make(map[string]string),
			MarkerFiles:      append([]string(nil), subProject.ConfigFiles...), // Use ConfigFiles as MarkerFiles
			RequiredServers:  requiredServers,
			ProjectSize: types.ProjectSize{
				TotalFiles:     100,  // Default values, could be enhanced with actual size detection
				SourceFiles:    80,
				TestFiles:      15,
				ConfigFiles:    5,
				TotalSizeBytes: 1024 * 1024, // 1MB default
			},
		}
		
		projectContexts = append(projectContexts, projectContext)
	}
	
	wcm.logger.WithFields(map[string]interface{}{
		"input_sub_projects":  len(subProjects),
		"output_contexts":     len(projectContexts),
		"workspace_root":      workspaceRoot,
	}).Debug("Converted sub-projects to project contexts")
	
	return projectContexts
}

// AggregatedWorkspaceInfo holds aggregated information about the workspace
type AggregatedWorkspaceInfo struct {
	ProjectType string
	Languages   []string
	Metadata    map[string]interface{}
}

// aggregateWorkspaceInfo aggregates information from workspace context and sub-projects
func (wcm *DefaultWorkspaceConfigManager) aggregateWorkspaceInfo(workspaceContext *WorkspaceContext, projectContext *project.ProjectContext, subProjectContexts []*project.ProjectContext) *AggregatedWorkspaceInfo {
	// Start with the primary project context as base
	aggregatedInfo := &AggregatedWorkspaceInfo{
		ProjectType: projectContext.ProjectType,
		Languages:   append([]string(nil), projectContext.Languages...), // Copy slice
		Metadata:    make(map[string]interface{}),
	}
	
	// If we have sub-projects, aggregate their information
	if len(subProjectContexts) > 0 {
		languageSet := make(map[string]bool)
		projectTypeSet := make(map[string]bool)
		
		// Add languages from primary project
		for _, lang := range projectContext.Languages {
			languageSet[lang] = true
		}
		projectTypeSet[projectContext.ProjectType] = true
		
		// Add languages and project types from sub-projects
		for _, subProjectContext := range subProjectContexts {
			for _, lang := range subProjectContext.Languages {
				languageSet[lang] = true
			}
			projectTypeSet[subProjectContext.ProjectType] = true
		}
		
		// Convert sets back to slices
		aggregatedLanguages := make([]string, 0, len(languageSet))
		for lang := range languageSet {
			aggregatedLanguages = append(aggregatedLanguages, lang)
		}
		aggregatedInfo.Languages = aggregatedLanguages
		
		// Determine workspace project type
		if len(projectTypeSet) > 1 {
			aggregatedInfo.ProjectType = "mixed"
		} else if len(workspaceContext.SubProjects) > 0 {
			// Use workspace context project type if available
			if workspaceContext.ProjectType != "" {
				aggregatedInfo.ProjectType = workspaceContext.ProjectType
			}
		}
		
		// Add metadata about sub-projects
		aggregatedInfo.Metadata["total_sub_projects"] = len(subProjectContexts)
		aggregatedInfo.Metadata["is_monorepo"] = len(subProjectContexts) > 1
		
		// Collect project type distribution
		projectTypeCount := make(map[string]int)
		for projectType := range projectTypeSet {
			projectTypeCount[projectType] = 0
		}
		for _, subProjectContext := range subProjectContexts {
			projectTypeCount[subProjectContext.ProjectType]++
		}
		aggregatedInfo.Metadata["project_type_distribution"] = projectTypeCount
	} else {
		// Single project workspace
		aggregatedInfo.Metadata["total_sub_projects"] = 0
		aggregatedInfo.Metadata["is_monorepo"] = false
	}
	
	wcm.logger.WithFields(map[string]interface{}{
		"aggregated_project_type": aggregatedInfo.ProjectType,
		"aggregated_languages":    aggregatedInfo.Languages,
		"total_sub_projects":      len(subProjectContexts),
	}).Debug("Aggregated workspace information")
	
	return aggregatedInfo
}

// convertSubProjectConfigsToInfoSlice converts sub-project configs map to slice for WorkspaceConfig
func (wcm *DefaultWorkspaceConfigManager) convertSubProjectConfigsToInfoSlice(subProjectConfigs map[string]*SubProjectInfo) []*SubProjectInfo {
	if len(subProjectConfigs) == 0 {
		return []*SubProjectInfo{}
	}
	
	subProjects := make([]*SubProjectInfo, 0, len(subProjectConfigs))
	for _, subProjectInfo := range subProjectConfigs {
		subProjects = append(subProjects, subProjectInfo)
	}
	
	wcm.logger.WithFields(map[string]interface{}{
		"input_configs":   len(subProjectConfigs),
		"output_projects": len(subProjects),
	}).Debug("Converted sub-project configs to info slice")
	
	return subProjects
}

func (wcm *DefaultWorkspaceConfigManager) GetWorkspaceConfigPath(workspaceRoot string) string {
	workspaceDir := wcm.GetWorkspaceDirectory(workspaceRoot)
	return filepath.Join(workspaceDir, WorkspaceConfigFileName)
}

func (wcm *DefaultWorkspaceConfigManager) GetWorkspaceDirectory(workspaceRoot string) string {
	workspaceHash := wcm.generateWorkspaceHash(workspaceRoot)
	return filepath.Join(wcm.baseConfigDir, "workspaces", workspaceHash)
}

func (wcm *DefaultWorkspaceConfigManager) CreateWorkspaceDirectories(workspaceRoot string) error {
	workspaceDir := wcm.GetWorkspaceDirectory(workspaceRoot)
	
	dirs := []string{
		workspaceDir,
		filepath.Join(workspaceDir, WorkspaceLogDirName),
		filepath.Join(workspaceDir, WorkspaceCacheDirName),
		filepath.Join(workspaceDir, WorkspaceIndexDirName),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	wcm.logger.WithField("workspace_dir", workspaceDir).Debug("Created workspace directories")
	return nil
}

func (wcm *DefaultWorkspaceConfigManager) ValidateWorkspaceConfig(config *WorkspaceConfig) error {
	if config == nil {
		return fmt.Errorf("workspace configuration cannot be nil")
	}

	var validationErrors []string
	var validationWarnings []string

	// 1. Basic workspace validation
	if config.Workspace.WorkspaceID == "" {
		validationErrors = append(validationErrors, "workspace ID cannot be empty")
	}

	if config.Workspace.RootPath == "" {
		validationErrors = append(validationErrors, "workspace root path cannot be empty")
	} else {
		if _, err := os.Stat(config.Workspace.RootPath); os.IsNotExist(err) {
			validationErrors = append(validationErrors, fmt.Sprintf("workspace root path does not exist: %s", config.Workspace.RootPath))
		}
	}

	if len(config.Servers) == 0 {
		validationErrors = append(validationErrors, "workspace configuration must have at least one server")
	}

	// Validate server configurations
	for serverName, serverConfig := range config.Servers {
		if serverConfig.Name == "" {
			validationErrors = append(validationErrors, fmt.Sprintf("server configuration '%s' missing name", serverName))
		}
		if serverConfig.Command == "" {
			validationErrors = append(validationErrors, fmt.Sprintf("server configuration '%s' missing command", serverName))
		}
	}

	// 2. Sub-project validation and aggregation
	subProjectIDs := make(map[string]bool)
	subProjectPaths := make(map[string]string)
	allRequiredServers := make(map[string][]string)

	for _, subProject := range config.SubProjects {
		if subProject == nil {
			validationErrors = append(validationErrors, "sub-project configuration cannot be nil")
			continue
		}

		// Validate individual sub-project
		if err := wcm.ValidateSubProjectConfig(subProject); err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("Sub-project '%s': %s", subProject.ID, err.Error()))
		}

		// Check for duplicate sub-project IDs
		if subProjectIDs[subProject.ID] {
			validationErrors = append(validationErrors, fmt.Sprintf("Duplicate sub-project ID: '%s'", subProject.ID))
		} else {
			subProjectIDs[subProject.ID] = true
		}

		// Check for path conflicts (nested sub-projects)
		if subProject.AbsolutePath != "" {
			for existingPath, existingID := range subProjectPaths {
				if wcm.isPathNested(subProject.AbsolutePath, existingPath) {
					validationErrors = append(validationErrors, fmt.Sprintf("Sub-project '%s' path '%s' conflicts with sub-project '%s' path '%s' (nested paths not allowed)", subProject.ID, subProject.AbsolutePath, existingID, existingPath))
				}
			}
			subProjectPaths[subProject.AbsolutePath] = subProject.ID
		}

		// Collect required servers for cross-validation
		if len(subProject.RequiredServers) > 0 {
			allRequiredServers[subProject.ID] = subProject.RequiredServers
		}
	}

	// 3. Cross-validation between sub-projects and servers
	for subProjectID, requiredServers := range allRequiredServers {
		for _, serverName := range requiredServers {
			if _, exists := config.Servers[serverName]; !exists {
				validationErrors = append(validationErrors, fmt.Sprintf("Sub-project '%s': required server '%s' not found in workspace configuration", subProjectID, serverName))
			}
		}
	}

	// 4. Workspace folder consistency validation
	serverWorkspaceRoots := make(map[string]map[string]string)
	for serverName, serverConfig := range config.Servers {
		if len(serverConfig.WorkspaceRoots) > 0 {
			serverWorkspaceRoots[serverName] = serverConfig.WorkspaceRoots
		}
	}

	for _, subProject := range config.SubProjects {
		if subProject.WorkspaceFolder != "" {
			for _, serverName := range subProject.RequiredServers {
				if workspaceRoots, exists := serverWorkspaceRoots[serverName]; exists {
					found := false
					for _, root := range workspaceRoots {
						if strings.HasPrefix(subProject.AbsolutePath, root) || root == subProject.WorkspaceFolder {
							found = true
							break
						}
					}
					if !found {
						validationWarnings = append(validationWarnings, fmt.Sprintf("Sub-project '%s' workspace folder '%s' may not be properly mapped to server '%s' workspace roots", subProject.ID, subProject.WorkspaceFolder, serverName))
					}
				}
			}
		}
	}

	// 5. Resource quota validation (aggregate limits)
	totalMaxMemory := 0
	totalMaxConcurrency := 0
	totalMaxCacheSize := 0

	for _, subProject := range config.SubProjects {
		if subProject.ResourceQuota != nil {
			totalMaxMemory += subProject.ResourceQuota.MaxMemoryMB
			totalMaxConcurrency += subProject.ResourceQuota.MaxConcurrency
			totalMaxCacheSize += subProject.ResourceQuota.MaxCacheSize
		}
	}

	// Check for reasonable limits (these are heuristic limits)
	const maxReasonableMemoryMB = 32768    // 32GB
	const maxReasonableConcurrency = 100   // 100 concurrent operations
	const maxReasonableCacheSize = 10240   // 10GB

	if totalMaxMemory > maxReasonableMemoryMB {
		validationWarnings = append(validationWarnings, fmt.Sprintf("Total resource quota memory (%d MB) exceeds reasonable limit (%d MB)", totalMaxMemory, maxReasonableMemoryMB))
	}
	if totalMaxConcurrency > maxReasonableConcurrency {
		validationWarnings = append(validationWarnings, fmt.Sprintf("Total resource quota concurrency (%d) exceeds reasonable limit (%d)", totalMaxConcurrency, maxReasonableConcurrency))
	}
	if totalMaxCacheSize > maxReasonableCacheSize {
		validationWarnings = append(validationWarnings, fmt.Sprintf("Total resource quota cache size (%d MB) exceeds reasonable limit (%d MB)", totalMaxCacheSize, maxReasonableCacheSize))
	}

	// 6. Log warnings if any
	if len(validationWarnings) > 0 {
		wcm.logger.WithFields(map[string]interface{}{
			"workspace_id": config.Workspace.WorkspaceID,
			"warnings":     validationWarnings,
		}).Warn("Workspace configuration validation warnings")
	}

	// 7. Return aggregated validation errors
	if len(validationErrors) > 0 {
		errorMsg := fmt.Sprintf("Workspace validation failed with %d errors:", len(validationErrors))
		for _, err := range validationErrors {
			errorMsg += fmt.Sprintf("\n- %s", err)
		}
		return fmt.Errorf(errorMsg)
	}

	return nil
}

// isPathNested checks if path1 is nested within path2 or vice versa
func (wcm *DefaultWorkspaceConfigManager) isPathNested(path1, path2 string) bool {
	// Clean and make absolute paths for comparison
	cleanPath1 := filepath.Clean(path1)
	cleanPath2 := filepath.Clean(path2)
	
	// Check if either path is a parent of the other
	rel1, err1 := filepath.Rel(cleanPath1, cleanPath2)
	rel2, err2 := filepath.Rel(cleanPath2, cleanPath1)
	
	// If one path is relative to the other and doesn't start with "..", it's nested
	if err1 == nil && !strings.HasPrefix(rel1, "..") && rel1 != "." {
		return true
	}
	if err2 == nil && !strings.HasPrefix(rel2, "..") && rel2 != "." {
		return true
	}
	
	return false
}

func (wcm *DefaultWorkspaceConfigManager) CleanupWorkspace(workspaceRoot string) error {
	workspaceDir := wcm.GetWorkspaceDirectory(workspaceRoot)
	
	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		return nil
	}

	if err := os.RemoveAll(workspaceDir); err != nil {
		return fmt.Errorf("failed to cleanup workspace directory: %w", err)
	}

	wcm.logger.WithField("workspace_dir", workspaceDir).Info("Cleaned up workspace directory")
	return nil
}

func (wcm *DefaultWorkspaceConfigManager) generateWorkspaceID(workspaceRoot string) string {
	return fmt.Sprintf("ws_%d", time.Now().Unix())
}

func (wcm *DefaultWorkspaceConfigManager) generateWorkspaceHash(workspaceRoot string) string {
	absPath, err := filepath.Abs(workspaceRoot)
	if err != nil {
		absPath = workspaceRoot
	}
	
	hash := sha256.Sum256([]byte(absPath))
	return fmt.Sprintf("%x", hash[:8])
}

func (wcm *DefaultWorkspaceConfigManager) createWorkspaceDirectoryStructure(workspaceRoot string) WorkspaceDirectories {
	workspaceDir := wcm.GetWorkspaceDirectory(workspaceRoot)
	
	return WorkspaceDirectories{
		Root:   workspaceDir,
		Config: workspaceDir,
		Cache:  filepath.Join(workspaceDir, WorkspaceCacheDirName),
		Logs:   filepath.Join(workspaceDir, WorkspaceLogDirName),
		Index:  filepath.Join(workspaceDir, WorkspaceIndexDirName),
		State:  filepath.Join(workspaceDir, WorkspaceStateFileName),
	}
}

func (wcm *DefaultWorkspaceConfigManager) generateServerConfigurations(projectContext *project.ProjectContext, subProjects map[string]*SubProjectInfo) (map[string]*config.ServerConfig, error) {
	servers := make(map[string]*config.ServerConfig)

	// Handle single-project scenario (backward compatibility)
	if subProjects == nil || len(subProjects) == 0 {
		return wcm.generateSingleProjectServerConfigurations(projectContext)
	}

	// Multi-project scenario: group sub-projects by required servers
	serverToSubProjects := wcm.groupSubProjectsByServer(subProjects)
	
	// Generate server configurations for each server
	for serverName, subProjectList := range serverToSubProjects {
		serverConfig, err := wcm.createMultiProjectServerConfig(serverName, subProjectList, projectContext)
		if err != nil {
			wcm.logger.WithFields(map[string]interface{}{
				"server": serverName,
				"error":  err,
			}).Warn("Failed to create server configuration")
			continue
		}
		
		if serverConfig != nil {
			servers[serverName] = serverConfig
		}
	}

	// Also handle languages from the main project context that might not be covered by sub-projects
	mainProjectServers, err := wcm.generateSingleProjectServerConfigurations(projectContext)
	if err != nil {
		return nil, fmt.Errorf("failed to generate main project server configurations: %w", err)
	}

	// Merge main project servers with sub-project servers
	for serverName, mainServerConfig := range mainProjectServers {
		if existingServer, exists := servers[serverName]; exists {
			// Merge workspace roots and languages
			wcm.mergeServerConfigurations(existingServer, mainServerConfig, projectContext.WorkspaceRoot)
		} else {
			// Add workspace root for main project
			if mainServerConfig.WorkspaceRoots == nil {
				mainServerConfig.WorkspaceRoots = make(map[string]string)
			}
			for _, lang := range mainServerConfig.Languages {
				mainServerConfig.WorkspaceRoots[lang] = projectContext.WorkspaceRoot
			}
			servers[serverName] = mainServerConfig
		}
	}

	return servers, nil
}

func (wcm *DefaultWorkspaceConfigManager) generateSingleProjectServerConfigurations(projectContext *project.ProjectContext) (map[string]*config.ServerConfig, error) {
	servers := make(map[string]*config.ServerConfig)

	for _, language := range projectContext.Languages {
		normalizedLang := wcm.normalizeLanguageForTemplate(language)
		template, exists := wcm.configGenerator.GetTemplate(normalizedLang)
		if !exists {
			wcm.logger.WithField("language", language).Warn("No server template found for language")
			continue
		}

		serverConfig := &config.ServerConfig{
			Name:         template.Name,
			Languages:    []string{language},
			Command:      template.Command,
			Args:         template.Args,
			Transport:    template.Transport,
			RootMarkers:  template.RootMarkers,
			Settings:     template.Settings,
			ServerType:   config.ServerTypeWorkspace,
			Priority:     template.Priority,
			Weight:       template.Weight,
			Constraints:  template.Constraints,
			WorkspaceRoots: map[string]string{
				language: projectContext.WorkspaceRoot,
			},
		}

		if template.LanguageSettings != nil {
			serverConfig.LanguageSettings = make(map[string]map[string]interface{})
			for lang, settings := range template.LanguageSettings {
				serverConfig.LanguageSettings[lang] = settings
			}
		}

		servers[template.Name] = serverConfig
	}

	return servers, nil
}

func (wcm *DefaultWorkspaceConfigManager) groupSubProjectsByServer(subProjects map[string]*SubProjectInfo) map[string][]*SubProjectInfo {
	serverToSubProjects := make(map[string][]*SubProjectInfo)
	
	for _, subProject := range subProjects {
		// If sub-project specifies required servers, use those
		if len(subProject.RequiredServers) > 0 {
			for _, serverName := range subProject.RequiredServers {
				serverToSubProjects[serverName] = append(serverToSubProjects[serverName], subProject)
			}
		} else {
			// Otherwise, determine servers based on languages
			for _, language := range subProject.Languages {
				normalizedLang := wcm.normalizeLanguageForTemplate(language)
				template, exists := wcm.configGenerator.GetTemplate(normalizedLang)
				if exists {
					serverToSubProjects[template.Name] = append(serverToSubProjects[template.Name], subProject)
				}
			}
		}
	}
	
	return serverToSubProjects
}

func (wcm *DefaultWorkspaceConfigManager) createMultiProjectServerConfig(serverName string, subProjectList []*SubProjectInfo, projectContext *project.ProjectContext) (*config.ServerConfig, error) {
	if len(subProjectList) == 0 {
		return nil, nil
	}

	// Collect all languages across sub-projects and find a template
	languageSet := make(map[string]bool)
	workspaceRoots := make(map[string]string)
	var serverConfig *config.ServerConfig
	
	for _, subProject := range subProjectList {
		for _, language := range subProject.Languages {
			languageSet[language] = true
			
			// Use workspace folder if specified, otherwise use absolute path
			workspacePath := subProject.AbsolutePath
			if subProject.WorkspaceFolder != "" {
				workspacePath = subProject.WorkspaceFolder
			}
			workspaceRoots[language] = workspacePath
			
			// Create server config based on first valid template found
			if serverConfig == nil {
				normalizedLang := wcm.normalizeLanguageForTemplate(language)
				template, exists := wcm.configGenerator.GetTemplate(normalizedLang)
				if exists && template.Name == serverName {
					serverConfig = &config.ServerConfig{
						Name:         template.Name,
						Languages:    []string{}, // Will be populated later
						Command:      template.Command,
						Args:         template.Args,
						Transport:    template.Transport,
						RootMarkers:  template.RootMarkers,
						Settings:     template.Settings,
						ServerType:   config.ServerTypeWorkspace,
						Priority:     template.Priority,
						Weight:       template.Weight,
						Constraints:  template.Constraints,
						WorkspaceRoots: make(map[string]string),
					}
					
					if template.LanguageSettings != nil {
						serverConfig.LanguageSettings = make(map[string]map[string]interface{})
						for lang, settings := range template.LanguageSettings {
							serverConfig.LanguageSettings[lang] = settings
						}
					}
				}
			}
		}
	}

	if serverConfig == nil {
		return nil, fmt.Errorf("no template found for server %s", serverName)
	}

	// Convert language set to slice and populate workspace roots
	languages := make([]string, 0, len(languageSet))
	for lang := range languageSet {
		languages = append(languages, lang)
	}
	serverConfig.Languages = languages
	serverConfig.WorkspaceRoots = workspaceRoots

	return serverConfig, nil
}

func (wcm *DefaultWorkspaceConfigManager) mergeServerConfigurations(existingServer, mainServer *config.ServerConfig, mainWorkspaceRoot string) {
	// Merge languages
	languageSet := make(map[string]bool)
	for _, lang := range existingServer.Languages {
		languageSet[lang] = true
	}
	for _, lang := range mainServer.Languages {
		if !languageSet[lang] {
			existingServer.Languages = append(existingServer.Languages, lang)
			languageSet[lang] = true
		}
	}

	// Merge workspace roots
	if existingServer.WorkspaceRoots == nil {
		existingServer.WorkspaceRoots = make(map[string]string)
	}
	for _, lang := range mainServer.Languages {
		if _, exists := existingServer.WorkspaceRoots[lang]; !exists {
			existingServer.WorkspaceRoots[lang] = mainWorkspaceRoot
		}
	}

	// Merge language settings
	if mainServer.LanguageSettings != nil {
		if existingServer.LanguageSettings == nil {
			existingServer.LanguageSettings = make(map[string]map[string]interface{})
		}
		for lang, settings := range mainServer.LanguageSettings {
			if _, exists := existingServer.LanguageSettings[lang]; !exists {
				existingServer.LanguageSettings[lang] = settings
			}
		}
	}
}

func (wcm *DefaultWorkspaceConfigManager) generatePerformanceConfig(projectContext *project.ProjectContext) config.PerformanceConfiguration {
	performanceConfig := config.PerformanceConfiguration{
		Enabled:    true,
		Profile:    config.PerformanceProfileDevelopment,
		AutoTuning: true,
		Version:    "1.0.0",
	}

	performanceConfig.Caching = &config.CachingConfiguration{
		Enabled:          true,
		GlobalTTL:        config.DefaultCacheTTL,
		MaxMemoryUsage:   1024,
		EvictionStrategy: config.EvictionStrategyLRU,
		ResponseCache: &config.CacheConfig{
			Enabled: true,
			TTL:     15 * time.Minute,
			MaxSize: 100,
		},
		SemanticCache: &config.CacheConfig{
			Enabled: true,
			TTL:     30 * time.Minute,
			MaxSize: 500,
		},
	}

	performanceConfig.Timeouts = &config.TimeoutConfiguration{
		GlobalTimeout:     5 * time.Minute,
		DefaultTimeout:    30 * time.Second,
		ConnectionTimeout: 10 * time.Second,
	}

	performanceConfig.SCIP = &config.SCIPConfiguration{
		Enabled:         true,
		AutoRefresh:     true,
		RefreshInterval: 10 * time.Minute,
		FallbackToLSP:   true,
		CacheConfig: config.CacheConfig{
			Enabled: true,
			TTL:     1 * time.Hour,
			MaxSize: 1000,
		},
	}

	return performanceConfig
}

func (wcm *DefaultWorkspaceConfigManager) generateCacheConfig(projectContext *project.ProjectContext) config.CachingConfiguration {
	return config.CachingConfiguration{
		Enabled:          true,
		GlobalTTL:        config.DefaultCacheTTL,
		MaxMemoryUsage:   512,
		EvictionStrategy: config.EvictionStrategyLRU,
		ResponseCache: &config.CacheConfig{
			Enabled: true,
			TTL:     15 * time.Minute,
			MaxSize: 100,
		},
		ProjectCache: &config.CacheConfig{
			Enabled: true,
			TTL:     1 * time.Hour,
			MaxSize: 50,
		},
	}
}

func (wcm *DefaultWorkspaceConfigManager) generateLoggingConfig(directories WorkspaceDirectories) LoggingConfig {
	return LoggingConfig{
		Level:      "info",
		OutputFile: filepath.Join(directories.Logs, "workspace.log"),
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     7,
	}
}

func (wcm *DefaultWorkspaceConfigManager) mergeConfigurations(globalConfig *config.GatewayConfig, workspaceConfig *WorkspaceConfig) *WorkspaceConfig {
	if globalConfig.PerformanceConfig != nil && workspaceConfig.Performance.Caching == nil {
		workspaceConfig.Performance.Caching = globalConfig.PerformanceConfig.Caching
	}

	if globalConfig.PerformanceConfig != nil && workspaceConfig.Performance.SCIP == nil {
		workspaceConfig.Performance.SCIP = globalConfig.PerformanceConfig.SCIP
	}

	for _, globalServer := range globalConfig.Servers {
		if workspaceServer, exists := workspaceConfig.Servers[globalServer.Name]; exists {
			if workspaceServer.Settings == nil && globalServer.Settings != nil {
				workspaceServer.Settings = globalServer.Settings
			}
			if workspaceServer.LanguageSettings == nil && globalServer.LanguageSettings != nil {
				workspaceServer.LanguageSettings = globalServer.LanguageSettings
			}
		}
	}

	return workspaceConfig
}

func (wcm *DefaultWorkspaceConfigManager) applyEnvironmentOverrides(config *WorkspaceConfig) {
	if config.Performance.SCIP != nil {
		config.Performance.SCIP.ApplyEnvironmentDefaults()
	}
}

func (wcm *DefaultWorkspaceConfigManager) normalizeLanguageForTemplate(language string) string {
	switch language {
	case "typescript", "ts":
		return "typescript"
	case "javascript", "js", "node", "nodejs":
		return "javascript"
	case "python", "py":
		return "python"
	case "go", "golang":
		return "go"
	case "java":
		return "java"
	case "rust", "rs":
		return "rust"
	default:
		return language
	}
}

func (wcm *DefaultWorkspaceConfigManager) saveWorkspaceConfig(config *WorkspaceConfig, configPath string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal workspace configuration: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write workspace configuration: %w", err)
	}

	return nil
}

func (wcm *DefaultWorkspaceConfigManager) GenerateSubProjectConfigs(workspaceRoot string, subProjects []*project.ProjectContext) (map[string]*SubProjectInfo, error) {
	if len(subProjects) == 0 {
		return make(map[string]*SubProjectInfo), nil
	}

	subProjectInfos := make(map[string]*SubProjectInfo)
	
	for _, projectContext := range subProjects {
		if projectContext == nil {
			wcm.logger.Warn("Skipping nil project context in sub-project generation")
			continue
		}

		relativePath, err := filepath.Rel(workspaceRoot, projectContext.RootPath)
		if err != nil {
			wcm.logger.WithError(err).WithFields(map[string]interface{}{
				"workspace_root": workspaceRoot,
				"project_root":   projectContext.RootPath,
			}).Warn("Failed to calculate relative path for sub-project, using absolute path")
			relativePath = projectContext.RootPath
		}

		subProjectID := wcm.generateSubProjectID(projectContext.ProjectType, relativePath)

		resourceQuota := wcm.generateResourceQuota(projectContext.ProjectType, projectContext.ProjectSize)

		requiredServers := make([]string, len(projectContext.RequiredServers))
		copy(requiredServers, projectContext.RequiredServers)

		markerFiles := make([]string, len(projectContext.MarkerFiles))
		copy(markerFiles, projectContext.MarkerFiles)

		dependencies := make(map[string]string)
		for key, value := range projectContext.Dependencies {
			dependencies[key] = value
		}

		subProjectInfo := &SubProjectInfo{
			ID:              subProjectID,
			Name:            wcm.generateSubProjectName(projectContext),
			RelativePath:    relativePath,
			AbsolutePath:    projectContext.RootPath,
			ProjectType:     projectContext.ProjectType,
			Languages:       append([]string(nil), projectContext.Languages...),
			WorkspaceFolder: projectContext.RootPath,
			ResourceQuota:   resourceQuota,
			MarkerFiles:     markerFiles,
			Dependencies:    dependencies,
			RequiredServers: requiredServers,
		}

		subProjectInfos[subProjectID] = subProjectInfo

		wcm.logger.WithFields(map[string]interface{}{
			"sub_project_id":   subProjectID,
			"project_type":     projectContext.ProjectType,
			"relative_path":    relativePath,
			"languages":        projectContext.Languages,
			"required_servers": requiredServers,
		}).Debug("Generated sub-project configuration")
	}

	wcm.logger.WithFields(map[string]interface{}{
		"workspace_root":       workspaceRoot,
		"total_sub_projects":   len(subProjectInfos),
		"generated_project_ids": func() []string {
			ids := make([]string, 0, len(subProjectInfos))
			for id := range subProjectInfos {
				ids = append(ids, id)
			}
			return ids
		}(),
	}).Info("Generated sub-project configurations")

	return subProjectInfos, nil
}

func (wcm *DefaultWorkspaceConfigManager) GetSubProjectConfig(workspaceRoot, subProjectID string) (*SubProjectInfo, error) {
	if subProjectID == "" {
		return nil, fmt.Errorf("sub-project ID cannot be empty")
	}

	workspaceConfig, err := wcm.LoadWorkspaceConfig(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to load workspace configuration: %w", err)
	}

	for _, subProject := range workspaceConfig.SubProjects {
		if subProject.ID == subProjectID {
			wcm.logger.WithFields(map[string]interface{}{
				"sub_project_id": subProjectID,
				"project_type":   subProject.ProjectType,
				"absolute_path":  subProject.AbsolutePath,
			}).Debug("Found sub-project configuration")
			return subProject, nil
		}
	}

	return nil, fmt.Errorf("sub-project with ID '%s' not found in workspace configuration", subProjectID)
}

func (wcm *DefaultWorkspaceConfigManager) ValidateSubProjectConfig(config *SubProjectInfo) error {
	if config == nil {
		return fmt.Errorf("sub-project configuration cannot be nil")
	}

	if config.ID == "" {
		return fmt.Errorf("sub-project ID cannot be empty")
	}

	if config.AbsolutePath == "" {
		return fmt.Errorf("sub-project absolute path cannot be empty")
	}

	if config.ProjectType == "" {
		return fmt.Errorf("sub-project type cannot be empty")
	}

	if _, err := os.Stat(config.AbsolutePath); os.IsNotExist(err) {
		return fmt.Errorf("sub-project path does not exist: %s", config.AbsolutePath)
	}

	if !filepath.IsAbs(config.AbsolutePath) {
		return fmt.Errorf("sub-project path must be absolute: %s", config.AbsolutePath)
	}

	if len(config.Languages) == 0 {
		return fmt.Errorf("sub-project must have at least one language")
	}

	if config.ResourceQuota != nil {
		if config.ResourceQuota.MaxMemoryMB < 0 {
			return fmt.Errorf("sub-project resource quota max memory cannot be negative: %d", config.ResourceQuota.MaxMemoryMB)
		}
		if config.ResourceQuota.MaxConcurrency < 0 {
			return fmt.Errorf("sub-project resource quota max concurrency cannot be negative: %d", config.ResourceQuota.MaxConcurrency)
		}
		if config.ResourceQuota.MaxCacheSize < 0 {
			return fmt.Errorf("sub-project resource quota max cache size cannot be negative: %d", config.ResourceQuota.MaxCacheSize)
		}
	}

	for _, serverName := range config.RequiredServers {
		if serverName == "" {
			return fmt.Errorf("sub-project required server names cannot be empty")
		}
	}

	wcm.logger.WithFields(map[string]interface{}{
		"sub_project_id": config.ID,
		"project_type":   config.ProjectType,
		"absolute_path":  config.AbsolutePath,
		"languages":      config.Languages,
	}).Debug("Sub-project configuration validation passed")

	return nil
}

func (wcm *DefaultWorkspaceConfigManager) generateSubProjectID(projectType, relativePath string) string {
	timestamp := time.Now().Unix()
	pathHash := sha256.Sum256([]byte(relativePath))
	pathHashStr := fmt.Sprintf("%x", pathHash[:4])
	return fmt.Sprintf("%s_%s_%d", projectType, pathHashStr, timestamp)
}

func (wcm *DefaultWorkspaceConfigManager) generateSubProjectName(projectContext *project.ProjectContext) string {
	if projectContext.DisplayName != "" {
		return projectContext.DisplayName
	}
	if projectContext.ModuleName != "" {
		return projectContext.ModuleName
	}
	return filepath.Base(projectContext.RootPath)
}

func (wcm *DefaultWorkspaceConfigManager) generateResourceQuota(projectType string, projectSize interface{}) *ResourceQuota {
	quota := &ResourceQuota{}

	switch projectType {
	case "go", "golang":
		quota.MaxMemoryMB = 512
		quota.MaxConcurrency = 8
		quota.MaxCacheSize = 100
	case "python", "py":
		quota.MaxMemoryMB = 256
		quota.MaxConcurrency = 4
		quota.MaxCacheSize = 75
	case "typescript", "javascript", "nodejs", "node":
		quota.MaxMemoryMB = 384
		quota.MaxConcurrency = 6
		quota.MaxCacheSize = 80
	case "java":
		quota.MaxMemoryMB = 768
		quota.MaxConcurrency = 10
		quota.MaxCacheSize = 150
	case "rust", "rs":
		quota.MaxMemoryMB = 640
		quota.MaxConcurrency = 8
		quota.MaxCacheSize = 120
	default:
		quota.MaxMemoryMB = 256
		quota.MaxConcurrency = 4
		quota.MaxCacheSize = 50
	}

	return quota
}