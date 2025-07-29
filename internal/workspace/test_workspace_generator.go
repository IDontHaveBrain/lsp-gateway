package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lsp-gateway/internal/project/types"
)

// TestWorkspaceGenerator provides utilities for generating complex multi-project test workspaces
type TestWorkspaceGenerator interface {
	GenerateMultiProjectWorkspace(config *MultiProjectWorkspaceConfig) (*TestWorkspace, error)
	GenerateSingleProjectWorkspace(projectType string, config *SingleProjectConfig) (*TestWorkspace, error)
	CleanupWorkspace(workspace *TestWorkspace) error
	ValidateWorkspaceStructure(workspace *TestWorkspace) error
	GenerateNestedWorkspace(config *NestedWorkspaceConfig) (*TestWorkspace, error)
	GenerateCustomWorkspace(config *CustomWorkspaceConfig) (*TestWorkspace, error)
}

// DefaultTestWorkspaceGenerator implements TestWorkspaceGenerator
type DefaultTestWorkspaceGenerator struct {
	baseDir           string
	tempDirs          []string
	projectTemplates  map[string]*ProjectTemplate
	mu                sync.RWMutex
	cleanupHandlers   []func() error
	generationOptions *GenerationOptions
}

// MultiProjectWorkspaceConfig configures multi-project workspace generation
type MultiProjectWorkspaceConfig struct {
	Name              string                      `json:"name"`
	BaseDir           string                      `json:"base_dir"`
	Projects          []*ProjectConfig            `json:"projects"`
	SharedDirectories []string                    `json:"shared_directories"`
	CrossReferences   []*CrossReference           `json:"cross_references"`
	WorkspaceFiles    map[string]string           `json:"workspace_files"`
	NestedLevel       int                         `json:"nested_level"`
	GenerateMetadata  bool                        `json:"generate_metadata"`
	EnableGitRepo     bool                        `json:"enable_git_repo"`
	CustomTemplates   map[string]*ProjectTemplate `json:"custom_templates"`
	ValidationLevel   ValidationLevel             `json:"validation_level"`
}

// SingleProjectConfig configures single project generation
type SingleProjectConfig struct {
	Name            string            `json:"name"`
	BaseDir         string            `json:"base_dir"`
	ProjectType     string            `json:"project_type"`
	Languages       []string          `json:"languages"`
	Template        *ProjectTemplate  `json:"template"`
	CustomFiles     map[string]string `json:"custom_files"`
	Dependencies    []string          `json:"dependencies"`
	EnableTests     bool              `json:"enable_tests"`
	EnableBuild     bool              `json:"enable_build"`
	ValidationLevel ValidationLevel   `json:"validation_level"`
}

// NestedWorkspaceConfig configures nested workspace generation
type NestedWorkspaceConfig struct {
	Name          string                         `json:"name"`
	BaseDir       string                         `json:"base_dir"`
	MaxDepth      int                            `json:"max_depth"`
	SubWorkspaces []*MultiProjectWorkspaceConfig `json:"sub_workspaces"`
	CommonFiles   map[string]string              `json:"common_files"`
}

// CustomWorkspaceConfig allows completely custom workspace generation
type CustomWorkspaceConfig struct {
	Name             string                     `json:"name"`
	BaseDir          string                     `json:"base_dir"`
	DirectoryTree    *DirectoryNode             `json:"directory_tree"`
	FileContents     map[string]string          `json:"file_contents"`
	ProjectMarkers   map[string][]string        `json:"project_markers"`
	CustomValidation func(*TestWorkspace) error `json:"-"`
}

// DirectoryNode represents a directory tree structure
type DirectoryNode struct {
	Name        string                    `json:"name"`
	IsFile      bool                      `json:"is_file"`
	Content     string                    `json:"content,omitempty"`
	Children    map[string]*DirectoryNode `json:"children,omitempty"`
	Permissions os.FileMode               `json:"permissions"`
}

// ProjectConfig configures individual project generation
type ProjectConfig struct {
	Name            string            `json:"name"`
	RelativePath    string            `json:"relative_path"`
	ProjectType     string            `json:"project_type"`
	Languages       []string          `json:"languages"`
	Template        *ProjectTemplate  `json:"template"`
	CustomFiles     map[string]string `json:"custom_files"`
	Dependencies    []string          `json:"dependencies"`
	MarkerFiles     []string          `json:"marker_files"`
	SourceDirs      []string          `json:"source_dirs"`
	TestDirs        []string          `json:"test_dirs"`
	EnableBuild     bool              `json:"enable_build"`
	EnableTests     bool              `json:"enable_tests"`
	CrossRefTargets []string          `json:"cross_ref_targets"`
}

// CrossReference defines cross-project references
type CrossReference struct {
	FromProject string   `json:"from_project"`
	ToProject   string   `json:"to_project"`
	RefType     string   `json:"ref_type"` // "import", "dependency", "config"
	Files       []string `json:"files"`
}

// ProjectTemplate defines template structure for project generation
type ProjectTemplate struct {
	Name          string                 `json:"name"`
	ProjectType   string                 `json:"project_type"`
	Languages     []string               `json:"languages"`
	Files         map[string]string      `json:"files"`
	Directories   []string               `json:"directories"`
	MarkerFiles   []string               `json:"marker_files"`
	Dependencies  map[string]string      `json:"dependencies"`
	BuildCommands []string               `json:"build_commands"`
	TestCommands  []string               `json:"test_commands"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// TestWorkspace represents a generated test workspace
type TestWorkspace struct {
	ID              string                         `json:"id"`
	Name            string                         `json:"name"`
	RootPath        string                         `json:"root_path"`
	Projects        []*DetectedSubProject          `json:"projects"`
	ProjectPaths    map[string]*DetectedSubProject `json:"project_paths"`
	Languages       []string                       `json:"languages"`
	GeneratedFiles  []string                       `json:"generated_files"`
	TempDirs        []string                       `json:"temp_dirs"`
	Metadata        map[string]interface{}         `json:"metadata"`
	ValidationLevel ValidationLevel                `json:"validation_level"`
	CreatedAt       time.Time                      `json:"created_at"`
	cleanupFuncs    []func() error                 `json:"-"`
}

// GenerationOptions configure workspace generation behavior
type GenerationOptions struct {
	EnableLogging        bool            `json:"enable_logging"`
	CreateGitRepo        bool            `json:"create_git_repo"`
	GenerateMetadata     bool            `json:"generate_metadata"`
	ValidationLevel      ValidationLevel `json:"validation_level"`
	MaxDepth             int             `json:"max_depth"`
	MaxFiles             int             `json:"max_files"`
	FileMode             os.FileMode     `json:"file_mode"`
	DirMode              os.FileMode     `json:"dir_mode"`
	PreserveTempDirs     bool            `json:"preserve_temp_dirs"`
	ConcurrentGeneration bool            `json:"concurrent_generation"`
}

// ValidationLevel defines the level of workspace validation
type ValidationLevel int

const (
	ValidationNone ValidationLevel = iota
	ValidationBasic
	ValidationComplete
	ValidationStrict
)

// NewTestWorkspaceGenerator creates a new test workspace generator
func NewTestWorkspaceGenerator() TestWorkspaceGenerator {
	return NewTestWorkspaceGeneratorWithOptions(DefaultGenerationOptions())
}

// NewTestWorkspaceGeneratorWithOptions creates a generator with custom options
func NewTestWorkspaceGeneratorWithOptions(options *GenerationOptions) TestWorkspaceGenerator {
	baseDir := filepath.Join(os.TempDir(), "lspg-test-workspaces")
	os.MkdirAll(baseDir, 0755)

	generator := &DefaultTestWorkspaceGenerator{
		baseDir:           baseDir,
		tempDirs:          make([]string, 0),
		projectTemplates:  createDefaultProjectTemplates(),
		cleanupHandlers:   make([]func() error, 0),
		generationOptions: options,
	}

	return generator
}

// DefaultGenerationOptions returns default generation options
func DefaultGenerationOptions() *GenerationOptions {
	return &GenerationOptions{
		EnableLogging:        true,
		CreateGitRepo:        false,
		GenerateMetadata:     true,
		ValidationLevel:      ValidationBasic,
		MaxDepth:             5,
		MaxFiles:             1000,
		FileMode:             0644,
		DirMode:              0755,
		PreserveTempDirs:     false,
		ConcurrentGeneration: true,
	}
}

// GenerateMultiProjectWorkspace generates a complex multi-project workspace
func (g *DefaultTestWorkspaceGenerator) GenerateMultiProjectWorkspace(config *MultiProjectWorkspaceConfig) (*TestWorkspace, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if config == nil {
		return nil, fmt.Errorf("workspace config cannot be nil")
	}

	// Validate configuration
	if err := g.validateMultiProjectConfig(config); err != nil {
		return nil, fmt.Errorf("invalid workspace config: %w", err)
	}

	// Create workspace root directory
	workspacePath, err := g.createWorkspaceRoot(config.Name, config.BaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace root: %w", err)
	}

	workspace := &TestWorkspace{
		ID:              generateWorkspaceID(),
		Name:            config.Name,
		RootPath:        workspacePath,
		Projects:        make([]*DetectedSubProject, 0),
		ProjectPaths:    make(map[string]*DetectedSubProject),
		Languages:       make([]string, 0),
		GeneratedFiles:  make([]string, 0),
		TempDirs:        []string{workspacePath},
		Metadata:        make(map[string]interface{}),
		ValidationLevel: config.ValidationLevel,
		CreatedAt:       time.Now(),
		cleanupFuncs:    make([]func() error, 0),
	}

	// Track temp directory
	g.tempDirs = append(g.tempDirs, workspacePath)

	// Generate shared directories
	if err := g.generateSharedDirectories(workspace, config.SharedDirectories); err != nil {
		g.cleanupWorkspace(workspace)
		return nil, fmt.Errorf("failed to generate shared directories: %w", err)
	}

	// Generate workspace-level files
	if err := g.generateWorkspaceFiles(workspace, config.WorkspaceFiles); err != nil {
		g.cleanupWorkspace(workspace)
		return nil, fmt.Errorf("failed to generate workspace files: %w", err)
	}

	// Generate projects
	projects, err := g.generateProjects(workspace, config.Projects, config.CustomTemplates)
	if err != nil {
		g.cleanupWorkspace(workspace)
		return nil, fmt.Errorf("failed to generate projects: %w", err)
	}
	workspace.Projects = projects

	// Build project path mapping
	workspace.ProjectPaths = g.buildProjectPathMapping(projects)

	// Determine workspace languages
	workspace.Languages = g.determineWorkspaceLanguages(projects)

	// Generate cross-references
	if err := g.generateCrossReferences(workspace, config.CrossReferences); err != nil {
		g.cleanupWorkspace(workspace)
		return nil, fmt.Errorf("failed to generate cross-references: %w", err)
	}

	// Generate metadata
	if config.GenerateMetadata {
		g.generateWorkspaceMetadata(workspace, config)
	}

	// Initialize git repository if requested
	if config.EnableGitRepo {
		if err := g.initializeGitRepo(workspace); err != nil {
			// Log warning but don't fail
			if g.generationOptions.EnableLogging {
				fmt.Printf("Warning: failed to initialize git repo: %v\n", err)
			}
		}
	}

	// Validate workspace if required
	if config.ValidationLevel > ValidationNone {
		if err := g.ValidateWorkspaceStructure(workspace); err != nil {
			g.cleanupWorkspace(workspace)
			return nil, fmt.Errorf("workspace validation failed: %w", err)
		}
	}

	return workspace, nil
}

// GenerateSingleProjectWorkspace generates a single project workspace
func (g *DefaultTestWorkspaceGenerator) GenerateSingleProjectWorkspace(projectType string, config *SingleProjectConfig) (*TestWorkspace, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if config == nil {
		config = &SingleProjectConfig{
			Name:            fmt.Sprintf("test-%s-project", projectType),
			ProjectType:     projectType,
			Languages:       []string{projectType},
			EnableTests:     true,
			EnableBuild:     true,
			ValidationLevel: ValidationBasic,
		}
	}

	// Create workspace path
	workspacePath, err := g.createWorkspaceRoot(config.Name, config.BaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace root: %w", err)
	}

	workspace := &TestWorkspace{
		ID:              generateWorkspaceID(),
		Name:            config.Name,
		RootPath:        workspacePath,
		Projects:        make([]*DetectedSubProject, 0),
		ProjectPaths:    make(map[string]*DetectedSubProject),
		Languages:       config.Languages,
		GeneratedFiles:  make([]string, 0),
		TempDirs:        []string{workspacePath},
		Metadata:        make(map[string]interface{}),
		ValidationLevel: config.ValidationLevel,
		CreatedAt:       time.Now(),
		cleanupFuncs:    make([]func() error, 0),
	}

	// Track temp directory
	g.tempDirs = append(g.tempDirs, workspacePath)

	// Get or create project template
	template := config.Template
	if template == nil {
		template = g.getProjectTemplate(projectType)
		if template == nil {
			g.cleanupWorkspace(workspace)
			return nil, fmt.Errorf("no template found for project type: %s", projectType)
		}
	}

	// Generate project files
	project, err := g.generateProjectFromTemplate(workspace.RootPath, ".", config.Name, template, config.CustomFiles)
	if err != nil {
		g.cleanupWorkspace(workspace)
		return nil, fmt.Errorf("failed to generate project: %w", err)
	}

	workspace.Projects = []*DetectedSubProject{project}
	workspace.ProjectPaths = map[string]*DetectedSubProject{
		project.AbsolutePath: project,
		project.RelativePath: project,
	}

	// Validate if required
	if config.ValidationLevel > ValidationNone {
		if err := g.ValidateWorkspaceStructure(workspace); err != nil {
			g.cleanupWorkspace(workspace)
			return nil, fmt.Errorf("workspace validation failed: %w", err)
		}
	}

	return workspace, nil
}

// GenerateNestedWorkspace generates a nested workspace structure
func (g *DefaultTestWorkspaceGenerator) GenerateNestedWorkspace(config *NestedWorkspaceConfig) (*TestWorkspace, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if config == nil || config.MaxDepth <= 0 {
		return nil, fmt.Errorf("invalid nested workspace config")
	}

	// Create root workspace
	workspacePath, err := g.createWorkspaceRoot(config.Name, config.BaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create nested workspace root: %w", err)
	}

	workspace := &TestWorkspace{
		ID:              generateWorkspaceID(),
		Name:            config.Name,
		RootPath:        workspacePath,
		Projects:        make([]*DetectedSubProject, 0),
		ProjectPaths:    make(map[string]*DetectedSubProject),
		Languages:       make([]string, 0),
		GeneratedFiles:  make([]string, 0),
		TempDirs:        []string{workspacePath},
		Metadata:        make(map[string]interface{}),
		ValidationLevel: ValidationBasic,
		CreatedAt:       time.Now(),
		cleanupFuncs:    make([]func() error, 0),
	}

	// Track temp directory
	g.tempDirs = append(g.tempDirs, workspacePath)

	// Generate common files at root level
	if err := g.generateCommonFiles(workspace, config.CommonFiles); err != nil {
		g.cleanupWorkspace(workspace)
		return nil, fmt.Errorf("failed to generate common files: %w", err)
	}

	// Generate sub-workspaces recursively
	allProjects := make([]*DetectedSubProject, 0)
	allLanguages := make(map[string]bool)

	for _, subConfig := range config.SubWorkspaces {
		subConfig.BaseDir = workspace.RootPath
		subWorkspace, err := g.GenerateMultiProjectWorkspace(subConfig)
		if err != nil {
			g.cleanupWorkspace(workspace)
			return nil, fmt.Errorf("failed to generate sub-workspace %s: %w", subConfig.Name, err)
		}

		// Merge projects and languages
		allProjects = append(allProjects, subWorkspace.Projects...)
		for _, lang := range subWorkspace.Languages {
			allLanguages[lang] = true
		}

		// Add cleanup function for sub-workspace
		workspace.cleanupFuncs = append(workspace.cleanupFuncs, func() error {
			return g.CleanupWorkspace(subWorkspace)
		})
	}

	workspace.Projects = allProjects
	workspace.ProjectPaths = g.buildProjectPathMapping(allProjects)

	// Convert language map to slice
	languages := make([]string, 0, len(allLanguages))
	for lang := range allLanguages {
		languages = append(languages, lang)
	}
	workspace.Languages = languages

	return workspace, nil
}

// GenerateCustomWorkspace generates a completely custom workspace
func (g *DefaultTestWorkspaceGenerator) GenerateCustomWorkspace(config *CustomWorkspaceConfig) (*TestWorkspace, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if config == nil {
		return nil, fmt.Errorf("custom workspace config cannot be nil")
	}

	// Create workspace root
	workspacePath, err := g.createWorkspaceRoot(config.Name, config.BaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create custom workspace root: %w", err)
	}

	workspace := &TestWorkspace{
		ID:              generateWorkspaceID(),
		Name:            config.Name,
		RootPath:        workspacePath,
		Projects:        make([]*DetectedSubProject, 0),
		ProjectPaths:    make(map[string]*DetectedSubProject),
		Languages:       make([]string, 0),
		GeneratedFiles:  make([]string, 0),
		TempDirs:        []string{workspacePath},
		Metadata:        make(map[string]interface{}),
		ValidationLevel: ValidationBasic,
		CreatedAt:       time.Now(),
		cleanupFuncs:    make([]func() error, 0),
	}

	// Track temp directory
	g.tempDirs = append(g.tempDirs, workspacePath)

	// Generate directory tree
	if config.DirectoryTree != nil {
		if err := g.generateDirectoryTree(workspace.RootPath, config.DirectoryTree); err != nil {
			g.cleanupWorkspace(workspace)
			return nil, fmt.Errorf("failed to generate directory tree: %w", err)
		}
	}

	// Generate file contents
	if err := g.generateFileContents(workspace, config.FileContents); err != nil {
		g.cleanupWorkspace(workspace)
		return nil, fmt.Errorf("failed to generate file contents: %w", err)
	}

	// Detect projects based on markers
	projects, err := g.detectProjectsFromMarkers(workspace, config.ProjectMarkers)
	if err != nil {
		g.cleanupWorkspace(workspace)
		return nil, fmt.Errorf("failed to detect projects from markers: %w", err)
	}
	workspace.Projects = projects
	workspace.ProjectPaths = g.buildProjectPathMapping(projects)
	workspace.Languages = g.determineWorkspaceLanguages(projects)

	// Custom validation if provided
	if config.CustomValidation != nil {
		if err := config.CustomValidation(workspace); err != nil {
			g.cleanupWorkspace(workspace)
			return nil, fmt.Errorf("custom validation failed: %w", err)
		}
	}

	return workspace, nil
}

// CleanupWorkspace cleans up a test workspace and all its resources
func (g *DefaultTestWorkspaceGenerator) CleanupWorkspace(workspace *TestWorkspace) error {
	if workspace == nil {
		return nil
	}

	var errors []error

	// Run custom cleanup functions
	for _, cleanupFunc := range workspace.cleanupFuncs {
		if err := cleanupFunc(); err != nil {
			errors = append(errors, err)
		}
	}

	// Remove workspace directory if it exists
	if workspace.RootPath != "" {
		if err := os.RemoveAll(workspace.RootPath); err != nil {
			errors = append(errors, fmt.Errorf("failed to remove workspace directory %s: %w", workspace.RootPath, err))
		}
	}

	// Remove from tracked temp directories
	g.mu.Lock()
	for i, dir := range g.tempDirs {
		if dir == workspace.RootPath {
			g.tempDirs = append(g.tempDirs[:i], g.tempDirs[i+1:]...)
			break
		}
	}
	g.mu.Unlock()

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}

	return nil
}

// ValidateWorkspaceStructure validates the structure of a generated workspace
func (g *DefaultTestWorkspaceGenerator) ValidateWorkspaceStructure(workspace *TestWorkspace) error {
	if workspace == nil {
		return fmt.Errorf("workspace cannot be nil")
	}

	// Check workspace directory exists
	if _, err := os.Stat(workspace.RootPath); os.IsNotExist(err) {
		return fmt.Errorf("workspace root directory does not exist: %s", workspace.RootPath)
	}

	// Validate each project
	for _, project := range workspace.Projects {
		if err := g.validateProject(project); err != nil {
			return fmt.Errorf("project validation failed for %s: %w", project.Name, err)
		}
	}

	// Validate project path mapping
	if err := g.validateProjectPathMapping(workspace); err != nil {
		return fmt.Errorf("project path mapping validation failed: %w", err)
	}

	// Additional validation based on level
	switch workspace.ValidationLevel {
	case ValidationComplete:
		return g.validateWorkspaceComplete(workspace)
	case ValidationStrict:
		return g.validateWorkspaceStrict(workspace)
	}

	return nil
}

// Helper methods

func (g *DefaultTestWorkspaceGenerator) validateMultiProjectConfig(config *MultiProjectWorkspaceConfig) error {
	if config.Name == "" {
		return fmt.Errorf("workspace name cannot be empty")
	}
	if len(config.Projects) == 0 {
		return fmt.Errorf("workspace must have at least one project")
	}
	for _, project := range config.Projects {
		if err := g.validateProjectConfig(project); err != nil {
			return fmt.Errorf("invalid project config %s: %w", project.Name, err)
		}
	}
	return nil
}

func (g *DefaultTestWorkspaceGenerator) validateProjectConfig(config *ProjectConfig) error {
	if config.Name == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	if config.ProjectType == "" {
		return fmt.Errorf("project type cannot be empty")
	}
	if config.RelativePath == "" {
		return fmt.Errorf("project relative path cannot be empty")
	}
	return nil
}

func (g *DefaultTestWorkspaceGenerator) createWorkspaceRoot(name, baseDir string) (string, error) {
	if baseDir == "" {
		baseDir = g.baseDir
	}

	// Ensure base directory exists
	if err := os.MkdirAll(baseDir, g.generationOptions.DirMode); err != nil {
		return "", err
	}

	// Create unique workspace directory
	timestamp := time.Now().UnixNano()
	workspaceName := fmt.Sprintf("%s-%d", sanitizeName(name), timestamp)
	workspacePath := filepath.Join(baseDir, workspaceName)

	if err := os.MkdirAll(workspacePath, g.generationOptions.DirMode); err != nil {
		return "", err
	}

	return workspacePath, nil
}

func (g *DefaultTestWorkspaceGenerator) generateSharedDirectories(workspace *TestWorkspace, sharedDirs []string) error {
	for _, dir := range sharedDirs {
		dirPath := filepath.Join(workspace.RootPath, dir)
		if err := os.MkdirAll(dirPath, g.generationOptions.DirMode); err != nil {
			return fmt.Errorf("failed to create shared directory %s: %w", dir, err)
		}

		// Create a README in shared directories
		readmePath := filepath.Join(dirPath, "README.md")
		readmeContent := fmt.Sprintf("# %s\n\nShared directory for workspace: %s\n", dir, workspace.Name)
		if err := os.WriteFile(readmePath, []byte(readmeContent), g.generationOptions.FileMode); err != nil {
			return fmt.Errorf("failed to create README in shared directory %s: %w", dir, err)
		}
		workspace.GeneratedFiles = append(workspace.GeneratedFiles, readmePath)
	}
	return nil
}

func (g *DefaultTestWorkspaceGenerator) generateWorkspaceFiles(workspace *TestWorkspace, workspaceFiles map[string]string) error {
	for relativePath, content := range workspaceFiles {
		fullPath := filepath.Join(workspace.RootPath, relativePath)

		// Ensure parent directory exists
		parentDir := filepath.Dir(fullPath)
		if err := os.MkdirAll(parentDir, g.generationOptions.DirMode); err != nil {
			return fmt.Errorf("failed to create parent directory for %s: %w", relativePath, err)
		}

		if err := os.WriteFile(fullPath, []byte(content), g.generationOptions.FileMode); err != nil {
			return fmt.Errorf("failed to write workspace file %s: %w", relativePath, err)
		}
		workspace.GeneratedFiles = append(workspace.GeneratedFiles, fullPath)
	}
	return nil
}

func (g *DefaultTestWorkspaceGenerator) generateProjects(workspace *TestWorkspace, projectConfigs []*ProjectConfig, customTemplates map[string]*ProjectTemplate) ([]*DetectedSubProject, error) {
	projects := make([]*DetectedSubProject, 0, len(projectConfigs))

	// Use concurrent generation if enabled
	if g.generationOptions.ConcurrentGeneration && len(projectConfigs) > 1 {
		return g.generateProjectsConcurrent(workspace, projectConfigs, customTemplates)
	}

	for _, projectConfig := range projectConfigs {
		project, err := g.generateProject(workspace, projectConfig, customTemplates)
		if err != nil {
			return nil, fmt.Errorf("failed to generate project %s: %w", projectConfig.Name, err)
		}
		projects = append(projects, project)
	}

	return projects, nil
}

func (g *DefaultTestWorkspaceGenerator) generateProjectsConcurrent(workspace *TestWorkspace, projectConfigs []*ProjectConfig, customTemplates map[string]*ProjectTemplate) ([]*DetectedSubProject, error) {
	type projectResult struct {
		project *DetectedSubProject
		error   error
		index   int
	}

	results := make(chan projectResult, len(projectConfigs))
	var wg sync.WaitGroup

	// Generate projects concurrently
	for i, projectConfig := range projectConfigs {
		wg.Add(1)
		go func(idx int, config *ProjectConfig) {
			defer wg.Done()
			project, err := g.generateProject(workspace, config, customTemplates)
			results <- projectResult{project: project, error: err, index: idx}
		}(i, projectConfig)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results in order
	projects := make([]*DetectedSubProject, len(projectConfigs))
	for result := range results {
		if result.error != nil {
			return nil, fmt.Errorf("failed to generate project: %w", result.error)
		}
		projects[result.index] = result.project
	}

	return projects, nil
}

func (g *DefaultTestWorkspaceGenerator) generateProject(workspace *TestWorkspace, projectConfig *ProjectConfig, customTemplates map[string]*ProjectTemplate) (*DetectedSubProject, error) {
	// Get template
	var template *ProjectTemplate
	if projectConfig.Template != nil {
		template = projectConfig.Template
	} else if customTemplates != nil {
		if t, exists := customTemplates[projectConfig.ProjectType]; exists {
			template = t
		}
	}

	if template == nil {
		template = g.getProjectTemplate(projectConfig.ProjectType)
		if template == nil {
			return nil, fmt.Errorf("no template found for project type: %s", projectConfig.ProjectType)
		}
	}

	// Create project directory path
	projectPath := filepath.Join(workspace.RootPath, projectConfig.RelativePath)

	// Generate project from template
	project, err := g.generateProjectFromTemplate(projectPath, projectConfig.RelativePath, projectConfig.Name, template, projectConfig.CustomFiles)
	if err != nil {
		return nil, err
	}

	// Override template values with config values
	if len(projectConfig.Languages) > 0 {
		project.Languages = projectConfig.Languages
	}
	if len(projectConfig.MarkerFiles) > 0 {
		project.MarkerFiles = projectConfig.MarkerFiles
	}

	return project, nil
}

func (g *DefaultTestWorkspaceGenerator) generateProjectFromTemplate(projectPath, relativePath, name string, template *ProjectTemplate, customFiles map[string]string) (*DetectedSubProject, error) {
	// Create project directory
	if err := os.MkdirAll(projectPath, g.generationOptions.DirMode); err != nil {
		return nil, fmt.Errorf("failed to create project directory %s: %w", projectPath, err)
	}

	// Create subdirectories
	for _, dir := range template.Directories {
		dirPath := filepath.Join(projectPath, dir)
		if err := os.MkdirAll(dirPath, g.generationOptions.DirMode); err != nil {
			return nil, fmt.Errorf("failed to create project subdirectory %s: %w", dir, err)
		}
	}

	// Generate template files
	generatedFiles := make([]string, 0)
	for filePath, content := range template.Files {
		fullPath := filepath.Join(projectPath, filePath)

		// Ensure parent directory exists
		parentDir := filepath.Dir(fullPath)
		if err := os.MkdirAll(parentDir, g.generationOptions.DirMode); err != nil {
			return nil, fmt.Errorf("failed to create parent directory for %s: %w", filePath, err)
		}

		// Replace template variables
		processedContent := g.processTemplateContent(content, name, template.ProjectType)

		if err := os.WriteFile(fullPath, []byte(processedContent), g.generationOptions.FileMode); err != nil {
			return nil, fmt.Errorf("failed to write template file %s: %w", filePath, err)
		}
		generatedFiles = append(generatedFiles, fullPath)
	}

	// Generate custom files
	for filePath, content := range customFiles {
		fullPath := filepath.Join(projectPath, filePath)

		// Ensure parent directory exists
		parentDir := filepath.Dir(fullPath)
		if err := os.MkdirAll(parentDir, g.generationOptions.DirMode); err != nil {
			return nil, fmt.Errorf("failed to create parent directory for custom file %s: %w", filePath, err)
		}

		processedContent := g.processTemplateContent(content, name, template.ProjectType)

		if err := os.WriteFile(fullPath, []byte(processedContent), g.generationOptions.FileMode); err != nil {
			return nil, fmt.Errorf("failed to write custom file %s: %w", filePath, err)
		}
		generatedFiles = append(generatedFiles, fullPath)
	}

	// Create DetectedSubProject
	project := &DetectedSubProject{
		ID:              generateTestProjectID(projectPath),
		Name:            name,
		RelativePath:    relativePath,
		AbsolutePath:    projectPath,
		ProjectType:     template.ProjectType,
		Languages:       template.Languages,
		MarkerFiles:     template.MarkerFiles,
		WorkspaceFolder: projectPath,
	}

	return project, nil
}

func (g *DefaultTestWorkspaceGenerator) generateCrossReferences(workspace *TestWorkspace, crossRefs []*CrossReference) error {
	for _, crossRef := range crossRefs {
		if err := g.generateCrossReference(workspace, crossRef); err != nil {
			return fmt.Errorf("failed to generate cross-reference from %s to %s: %w", crossRef.FromProject, crossRef.ToProject, err)
		}
	}
	return nil
}

func (g *DefaultTestWorkspaceGenerator) generateCrossReference(workspace *TestWorkspace, crossRef *CrossReference) error {
	// Find projects
	var fromProject, toProject *DetectedSubProject
	for _, project := range workspace.Projects {
		if project.Name == crossRef.FromProject {
			fromProject = project
		}
		if project.Name == crossRef.ToProject {
			toProject = project
		}
	}

	if fromProject == nil {
		return fmt.Errorf("from project not found: %s", crossRef.FromProject)
	}
	if toProject == nil {
		return fmt.Errorf("to project not found: %s", crossRef.ToProject)
	}

	// Generate cross-reference based on type
	switch crossRef.RefType {
	case "import":
		return g.generateImportReference(fromProject, toProject, crossRef.Files)
	case "dependency":
		return g.generateDependencyReference(fromProject, toProject, crossRef.Files)
	case "config":
		return g.generateConfigReference(fromProject, toProject, crossRef.Files)
	default:
		return fmt.Errorf("unsupported cross-reference type: %s", crossRef.RefType)
	}
}

func (g *DefaultTestWorkspaceGenerator) generateImportReference(fromProject, toProject *DetectedSubProject, files []string) error {
	// Implementation depends on project types
	// For simplicity, create a reference file
	refFileName := fmt.Sprintf("reference_to_%s.txt", sanitizeName(toProject.Name))
	refFilePath := filepath.Join(fromProject.AbsolutePath, refFileName)

	refContent := fmt.Sprintf("# Reference to %s\n\nProject: %s\nPath: %s\nType: %s\nLanguages: %v\n",
		toProject.Name, toProject.Name, toProject.RelativePath, toProject.ProjectType, toProject.Languages)

	return os.WriteFile(refFilePath, []byte(refContent), g.generationOptions.FileMode)
}

func (g *DefaultTestWorkspaceGenerator) generateDependencyReference(fromProject, toProject *DetectedSubProject, files []string) error {
	// Create dependency reference based on project types
	depFileName := fmt.Sprintf("dependencies_from_%s.txt", sanitizeName(toProject.Name))
	depFilePath := filepath.Join(fromProject.AbsolutePath, depFileName)

	depContent := fmt.Sprintf("# Dependencies from %s\n\nDependent on: %s\n", toProject.Name, toProject.Name)

	return os.WriteFile(depFilePath, []byte(depContent), g.generationOptions.FileMode)
}

func (g *DefaultTestWorkspaceGenerator) generateConfigReference(fromProject, toProject *DetectedSubProject, files []string) error {
	// Create config reference
	configFileName := "cross_project_config.json"
	configFilePath := filepath.Join(fromProject.AbsolutePath, configFileName)

	configContent := fmt.Sprintf(`{
  "cross_project_references": {
    "target_project": "%s",
    "target_path": "%s",
    "reference_type": "config",
    "files": %v
  }
}`, toProject.Name, toProject.RelativePath, files)

	return os.WriteFile(configFilePath, []byte(configContent), g.generationOptions.FileMode)
}

func (g *DefaultTestWorkspaceGenerator) buildProjectPathMapping(projects []*DetectedSubProject) map[string]*DetectedSubProject {
	projectPaths := make(map[string]*DetectedSubProject)

	// Sort projects by path depth (deepest first) for proper precedence
	sortedProjects := make([]*DetectedSubProject, len(projects))
	copy(sortedProjects, projects)

	for i := 0; i < len(sortedProjects); i++ {
		for j := i + 1; j < len(sortedProjects); j++ {
			depthI := strings.Count(sortedProjects[i].AbsolutePath, string(filepath.Separator))
			depthJ := strings.Count(sortedProjects[j].AbsolutePath, string(filepath.Separator))
			if depthI < depthJ {
				sortedProjects[i], sortedProjects[j] = sortedProjects[j], sortedProjects[i]
			}
		}
	}

	// Build mapping with deepest projects taking precedence
	for _, project := range sortedProjects {
		projectPaths[project.AbsolutePath] = project
		projectPaths[project.RelativePath] = project
	}

	return projectPaths
}

func (g *DefaultTestWorkspaceGenerator) determineWorkspaceLanguages(projects []*DetectedSubProject) []string {
	languageSet := make(map[string]bool)

	for _, project := range projects {
		for _, lang := range project.Languages {
			if lang != types.PROJECT_TYPE_UNKNOWN {
				languageSet[lang] = true
			}
		}
	}

	languages := make([]string, 0, len(languageSet))
	for lang := range languageSet {
		languages = append(languages, lang)
	}

	if len(languages) == 0 {
		languages = []string{types.PROJECT_TYPE_UNKNOWN}
	}

	return languages
}

func (g *DefaultTestWorkspaceGenerator) generateWorkspaceMetadata(workspace *TestWorkspace, config *MultiProjectWorkspaceConfig) {
	workspace.Metadata["generator_version"] = "1.0.0"
	workspace.Metadata["generation_time"] = workspace.CreatedAt
	workspace.Metadata["project_count"] = len(workspace.Projects)
	workspace.Metadata["language_count"] = len(workspace.Languages)
	workspace.Metadata["config_name"] = config.Name
	workspace.Metadata["nested_level"] = config.NestedLevel
	workspace.Metadata["cross_references"] = len(config.CrossReferences)
	workspace.Metadata["shared_directories"] = len(config.SharedDirectories)
}

func (g *DefaultTestWorkspaceGenerator) initializeGitRepo(workspace *TestWorkspace) error {
	// Create basic git repository
	gitDir := filepath.Join(workspace.RootPath, ".git")
	if err := os.MkdirAll(gitDir, g.generationOptions.DirMode); err != nil {
		return err
	}

	// Create basic .gitignore
	gitignorePath := filepath.Join(workspace.RootPath, ".gitignore")
	gitignoreContent := `# Generated workspace files
*.tmp
*.log
.DS_Store
Thumbs.db

# Language-specific ignores
node_modules/
__pycache__/
*.pyc
target/
build/
dist/
*.class
`

	return os.WriteFile(gitignorePath, []byte(gitignoreContent), g.generationOptions.FileMode)
}

func (g *DefaultTestWorkspaceGenerator) generateCommonFiles(workspace *TestWorkspace, commonFiles map[string]string) error {
	for relativePath, content := range commonFiles {
		fullPath := filepath.Join(workspace.RootPath, relativePath)

		parentDir := filepath.Dir(fullPath)
		if err := os.MkdirAll(parentDir, g.generationOptions.DirMode); err != nil {
			return fmt.Errorf("failed to create parent directory for %s: %w", relativePath, err)
		}

		if err := os.WriteFile(fullPath, []byte(content), g.generationOptions.FileMode); err != nil {
			return fmt.Errorf("failed to write common file %s: %w", relativePath, err)
		}
		workspace.GeneratedFiles = append(workspace.GeneratedFiles, fullPath)
	}
	return nil
}

func (g *DefaultTestWorkspaceGenerator) generateDirectoryTree(rootPath string, node *DirectoryNode) error {
	currentPath := filepath.Join(rootPath, node.Name)

	if node.IsFile {
		// Create file
		parentDir := filepath.Dir(currentPath)
		if err := os.MkdirAll(parentDir, g.generationOptions.DirMode); err != nil {
			return err
		}

		content := node.Content
		if content == "" {
			content = fmt.Sprintf("# Generated file: %s\n", node.Name)
		}

		return os.WriteFile(currentPath, []byte(content), node.Permissions)
	} else {
		// Create directory
		if err := os.MkdirAll(currentPath, node.Permissions); err != nil {
			return err
		}

		// Create children
		for _, child := range node.Children {
			if err := g.generateDirectoryTree(currentPath, child); err != nil {
				return err
			}
		}
	}

	return nil
}

func (g *DefaultTestWorkspaceGenerator) generateFileContents(workspace *TestWorkspace, fileContents map[string]string) error {
	for relativePath, content := range fileContents {
		fullPath := filepath.Join(workspace.RootPath, relativePath)

		parentDir := filepath.Dir(fullPath)
		if err := os.MkdirAll(parentDir, g.generationOptions.DirMode); err != nil {
			return fmt.Errorf("failed to create parent directory for %s: %w", relativePath, err)
		}

		if err := os.WriteFile(fullPath, []byte(content), g.generationOptions.FileMode); err != nil {
			return fmt.Errorf("failed to write file content %s: %w", relativePath, err)
		}
		workspace.GeneratedFiles = append(workspace.GeneratedFiles, fullPath)
	}
	return nil
}

func (g *DefaultTestWorkspaceGenerator) detectProjectsFromMarkers(workspace *TestWorkspace, projectMarkers map[string][]string) ([]*DetectedSubProject, error) {
	projects := make([]*DetectedSubProject, 0)

	// Walk through workspace directory to find marker files
	err := filepath.Walk(workspace.RootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		fileName := info.Name()
		for projectType, markers := range projectMarkers {
			for _, marker := range markers {
				if fileName == marker {
					// Found a marker file, create project
					projectDir := filepath.Dir(path)
					relativePath, _ := filepath.Rel(workspace.RootPath, projectDir)
					if relativePath == "." {
						relativePath = ""
					}

					project := &DetectedSubProject{
						ID:              generateTestProjectID(projectDir),
						Name:            filepath.Base(projectDir),
						RelativePath:    relativePath,
						AbsolutePath:    projectDir,
						ProjectType:     projectType,
						Languages:       []string{projectType},
						MarkerFiles:     []string{marker},
						WorkspaceFolder: projectDir,
					}

					projects = append(projects, project)
					return nil
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return projects, nil
}

func (g *DefaultTestWorkspaceGenerator) validateProject(project *DetectedSubProject) error {
	// Check project directory exists
	if _, err := os.Stat(project.AbsolutePath); os.IsNotExist(err) {
		return fmt.Errorf("project directory does not exist: %s", project.AbsolutePath)
	}

	// Check marker files exist
	for _, markerFile := range project.MarkerFiles {
		markerPath := filepath.Join(project.AbsolutePath, markerFile)
		if _, err := os.Stat(markerPath); os.IsNotExist(err) {
			return fmt.Errorf("marker file does not exist: %s", markerPath)
		}
	}

	return nil
}

func (g *DefaultTestWorkspaceGenerator) validateProjectPathMapping(workspace *TestWorkspace) error {
	// Ensure all projects are in the mapping
	for _, project := range workspace.Projects {
		if _, exists := workspace.ProjectPaths[project.AbsolutePath]; !exists {
			return fmt.Errorf("project not found in path mapping: %s", project.AbsolutePath)
		}
	}

	return nil
}

func (g *DefaultTestWorkspaceGenerator) validateWorkspaceComplete(workspace *TestWorkspace) error {
	// Complete validation includes checking file contents, permissions, etc.
	for _, filePath := range workspace.GeneratedFiles {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return fmt.Errorf("generated file does not exist: %s", filePath)
		}
	}

	return nil
}

func (g *DefaultTestWorkspaceGenerator) validateWorkspaceStrict(workspace *TestWorkspace) error {
	// Strict validation includes complete validation plus additional checks
	if err := g.validateWorkspaceComplete(workspace); err != nil {
		return err
	}

	// Additional strict validations
	if len(workspace.Projects) == 0 {
		return fmt.Errorf("workspace has no projects")
	}

	if len(workspace.Languages) == 0 {
		return fmt.Errorf("workspace has no languages")
	}

	return nil
}

func (g *DefaultTestWorkspaceGenerator) cleanupWorkspace(workspace *TestWorkspace) {
	// Helper method for internal cleanup during generation errors
	if workspace != nil && workspace.RootPath != "" {
		os.RemoveAll(workspace.RootPath)
	}
}

func (g *DefaultTestWorkspaceGenerator) getProjectTemplate(projectType string) *ProjectTemplate {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if template, exists := g.projectTemplates[projectType]; exists {
		return template
	}

	return nil
}

func (g *DefaultTestWorkspaceGenerator) processTemplateContent(content, projectName, projectType string) string {
	// Simple template variable replacement
	content = strings.ReplaceAll(content, "{{PROJECT_NAME}}", projectName)
	content = strings.ReplaceAll(content, "{{PROJECT_TYPE}}", projectType)
	content = strings.ReplaceAll(content, "{{TIMESTAMP}}", time.Now().Format("2006-01-02 15:04:05"))

	return content
}

// Helper functions

func generateWorkspaceID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return fmt.Sprintf("workspace_%s", hex.EncodeToString(bytes))
}

func generateTestProjectID(projectPath string) string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return fmt.Sprintf("project_%s", hex.EncodeToString(bytes))
}

func sanitizeName(name string) string {
	// Remove invalid characters for file/directory names
	sanitized := strings.ReplaceAll(name, " ", "_")
	sanitized = strings.ReplaceAll(sanitized, "/", "_")
	sanitized = strings.ReplaceAll(sanitized, "\\", "_")
	sanitized = strings.ReplaceAll(sanitized, ":", "_")
	sanitized = strings.ReplaceAll(sanitized, "*", "_")
	sanitized = strings.ReplaceAll(sanitized, "?", "_")
	sanitized = strings.ReplaceAll(sanitized, "\"", "_")
	sanitized = strings.ReplaceAll(sanitized, "<", "_")
	sanitized = strings.ReplaceAll(sanitized, ">", "_")
	sanitized = strings.ReplaceAll(sanitized, "|", "_")

	return sanitized
}

func createDefaultProjectTemplates() map[string]*ProjectTemplate {
	templates := make(map[string]*ProjectTemplate)

	// Go template
	templates[types.PROJECT_TYPE_GO] = &ProjectTemplate{
		Name:        "Go Project",
		ProjectType: types.PROJECT_TYPE_GO,
		Languages:   []string{types.PROJECT_TYPE_GO},
		Files:       GetTestProjectFiles("go"),
		Directories: []string{"cmd", "internal", "pkg", "test"},
		MarkerFiles: []string{types.MARKER_GO_MOD},
		Dependencies: map[string]string{
			"go": "1.21",
		},
		BuildCommands: []string{"go build", "go test"},
		TestCommands:  []string{"go test ./..."},
	}

	// Python template
	templates[types.PROJECT_TYPE_PYTHON] = &ProjectTemplate{
		Name:        "Python Project",
		ProjectType: types.PROJECT_TYPE_PYTHON,
		Languages:   []string{types.PROJECT_TYPE_PYTHON},
		Files:       GetTestProjectFiles("python"),
		Directories: []string{"src", "tests", "docs"},
		MarkerFiles: []string{types.MARKER_REQUIREMENTS, types.MARKER_SETUP_PY},
		Dependencies: map[string]string{
			"python": ">=3.8",
		},
		BuildCommands: []string{"python setup.py build"},
		TestCommands:  []string{"python -m pytest"},
	}

	// TypeScript template
	templates[types.PROJECT_TYPE_TYPESCRIPT] = &ProjectTemplate{
		Name:        "TypeScript Project",
		ProjectType: types.PROJECT_TYPE_TYPESCRIPT,
		Languages:   []string{types.PROJECT_TYPE_TYPESCRIPT},
		Files:       GetTestProjectFiles("typescript"),
		Directories: []string{"src", "dist", "tests"},
		MarkerFiles: []string{types.MARKER_TSCONFIG, types.MARKER_PACKAGE_JSON},
		Dependencies: map[string]string{
			"node":       ">=16.0.0",
			"typescript": "^5.0.0",
		},
		BuildCommands: []string{"npm run build"},
		TestCommands:  []string{"npm test"},
	}

	// Java template
	templates[types.PROJECT_TYPE_JAVA] = &ProjectTemplate{
		Name:        "Java Project",
		ProjectType: types.PROJECT_TYPE_JAVA,
		Languages:   []string{types.PROJECT_TYPE_JAVA},
		Files:       GetTestProjectFiles("java"),
		Directories: []string{"src/main/java", "src/test/java", "src/main/resources"},
		MarkerFiles: []string{types.MARKER_POM_XML},
		Dependencies: map[string]string{
			"java":  ">=11",
			"maven": ">=3.6.0",
		},
		BuildCommands: []string{"mvn compile"},
		TestCommands:  []string{"mvn test"},
	}

	return templates
}

// Convenience functions for common test scenarios

// CreateSimpleMultiProjectWorkspace creates a simple multi-project workspace for testing
func CreateSimpleMultiProjectWorkspace(name string, languages []string) (*TestWorkspace, error) {
	generator := NewTestWorkspaceGenerator()

	projects := make([]*ProjectConfig, 0, len(languages))
	for _, lang := range languages {
		projects = append(projects, &ProjectConfig{
			Name:         fmt.Sprintf("%s-service", lang),
			RelativePath: fmt.Sprintf("services/%s-service", lang),
			ProjectType:  lang,
			Languages:    []string{lang},
			EnableBuild:  true,
			EnableTests:  true,
		})
	}

	config := &MultiProjectWorkspaceConfig{
		Name:              name,
		Projects:          projects,
		SharedDirectories: []string{"shared", "docs"},
		WorkspaceFiles: map[string]string{
			"README.md": fmt.Sprintf("# %s\n\nMulti-project workspace with: %v\n", name, languages),
			"Makefile":  "# Workspace Makefile\n\nall:\n\t@echo \"Building all projects\"\n",
		},
		GenerateMetadata: true,
		ValidationLevel:  ValidationBasic,
	}

	return generator.GenerateMultiProjectWorkspace(config)
}

// CreateComplexNestedWorkspace creates a complex nested workspace for advanced testing
func CreateComplexNestedWorkspace(name string) (*TestWorkspace, error) {
	generator := NewTestWorkspaceGenerator()

	// Create sub-workspaces
	backendWorkspace := &MultiProjectWorkspaceConfig{
		Name: "backend",
		Projects: []*ProjectConfig{
			{
				Name:         "user-service",
				RelativePath: "user-service",
				ProjectType:  types.PROJECT_TYPE_GO,
				Languages:    []string{types.PROJECT_TYPE_GO},
				EnableBuild:  true,
				EnableTests:  true,
			},
			{
				Name:         "auth-service",
				RelativePath: "auth-service",
				ProjectType:  types.PROJECT_TYPE_JAVA,
				Languages:    []string{types.PROJECT_TYPE_JAVA},
				EnableBuild:  true,
				EnableTests:  true,
			},
		},
		SharedDirectories: []string{"shared", "config"},
		ValidationLevel:   ValidationBasic,
	}

	frontendWorkspace := &MultiProjectWorkspaceConfig{
		Name: "frontend",
		Projects: []*ProjectConfig{
			{
				Name:         "web-app",
				RelativePath: "web-app",
				ProjectType:  types.PROJECT_TYPE_TYPESCRIPT,
				Languages:    []string{types.PROJECT_TYPE_TYPESCRIPT},
				EnableBuild:  true,
				EnableTests:  true,
			},
			{
				Name:         "mobile-app",
				RelativePath: "mobile-app",
				ProjectType:  types.PROJECT_TYPE_TYPESCRIPT,
				Languages:    []string{types.PROJECT_TYPE_TYPESCRIPT},
				EnableBuild:  true,
				EnableTests:  true,
			},
		},
		SharedDirectories: []string{"components", "assets"},
		ValidationLevel:   ValidationBasic,
	}

	config := &NestedWorkspaceConfig{
		Name:          name,
		MaxDepth:      3,
		SubWorkspaces: []*MultiProjectWorkspaceConfig{backendWorkspace, frontendWorkspace},
		CommonFiles: map[string]string{
			"README.md":          fmt.Sprintf("# %s\n\nComplex nested workspace\n", name),
			"docker-compose.yml": "version: '3.8'\nservices:\n  # Services defined here\n",
		},
	}

	return generator.GenerateNestedWorkspace(config)
}

// CreateTestWorkspaceWithCrossReferences creates a workspace with cross-project references
func CreateTestWorkspaceWithCrossReferences(name string) (*TestWorkspace, error) {
	generator := NewTestWorkspaceGenerator()

	config := &MultiProjectWorkspaceConfig{
		Name: name,
		Projects: []*ProjectConfig{
			{
				Name:         "core-lib",
				RelativePath: "libs/core",
				ProjectType:  types.PROJECT_TYPE_GO,
				Languages:    []string{types.PROJECT_TYPE_GO},
				EnableBuild:  true,
				EnableTests:  true,
			},
			{
				Name:         "api-server",
				RelativePath: "services/api",
				ProjectType:  types.PROJECT_TYPE_GO,
				Languages:    []string{types.PROJECT_TYPE_GO},
				EnableBuild:  true,
				EnableTests:  true,
			},
			{
				Name:         "worker",
				RelativePath: "services/worker",
				ProjectType:  types.PROJECT_TYPE_PYTHON,
				Languages:    []string{types.PROJECT_TYPE_PYTHON},
				EnableBuild:  true,
				EnableTests:  true,
			},
		},
		CrossReferences: []*CrossReference{
			{
				FromProject: "api-server",
				ToProject:   "core-lib",
				RefType:     "import",
				Files:       []string{"main.go"},
			},
			{
				FromProject: "worker",
				ToProject:   "core-lib",
				RefType:     "dependency",
				Files:       []string{"requirements.txt"},
			},
		},
		SharedDirectories: []string{"shared", "config"},
		GenerateMetadata:  true,
		ValidationLevel:   ValidationComplete,
	}

	return generator.GenerateMultiProjectWorkspace(config)
}

// CleanupAllWorkspaces cleans up all tracked temporary directories
func (g *DefaultTestWorkspaceGenerator) CleanupAllWorkspaces() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	var errors []error

	// Run all cleanup handlers
	for _, handler := range g.cleanupHandlers {
		if err := handler(); err != nil {
			errors = append(errors, err)
		}
	}

	// Remove all temp directories
	for _, dir := range g.tempDirs {
		if err := os.RemoveAll(dir); err != nil {
			errors = append(errors, fmt.Errorf("failed to remove temp directory %s: %w", dir, err))
		}
	}

	// Clear tracking lists
	g.tempDirs = g.tempDirs[:0]
	g.cleanupHandlers = g.cleanupHandlers[:0]

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}

	return nil
}
