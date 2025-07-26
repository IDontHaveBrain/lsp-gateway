package project

import (
	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/project/types"
	"time"
)

type ProjectContext struct {
	// Core identification
	ProjectType   string `json:"project_type"`   // Primary project type (go, python, nodejs, java, typescript, mixed, unknown)
	RootPath      string `json:"root_path"`      // Absolute path to project root
	WorkspaceRoot string `json:"workspace_root"` // Workspace root (may differ from project root in monorepos)

	// Language information
	Languages        []string          `json:"languages"`                   // All detected languages in the project
	PrimaryLanguage  string            `json:"primary_language"`            // Primary/dominant language
	LanguageVersions map[string]string `json:"language_versions,omitempty"` // Language-specific version info

	// Project metadata
	ModuleName  string `json:"module_name,omitempty"`  // Module/package name (e.g., go module name)
	Version     string `json:"version,omitempty"`      // Project version if available
	DisplayName string `json:"display_name,omitempty"` // Human-readable project name
	Description string `json:"description,omitempty"`  // Project description

	// Dependencies and requirements
	Dependencies    map[string]string `json:"dependencies,omitempty"`     // Direct dependencies with versions
	DevDependencies map[string]string `json:"dev_dependencies,omitempty"` // Development dependencies
	RequiredServers []string          `json:"required_servers"`           // LSP servers needed for this project

	// Project structure information
	SourceDirs  []string `json:"source_dirs,omitempty"`  // Source code directories
	TestDirs    []string `json:"test_dirs,omitempty"`    // Test directories
	ConfigFiles []string `json:"config_files,omitempty"` // Configuration files found
	MarkerFiles []string `json:"marker_files"`           // Files used for project type detection
	BuildFiles  []string `json:"build_files,omitempty"`  // Build configuration files

	// Build and tooling information
	BuildSystem    string   `json:"build_system,omitempty"`    // Build system (make, gradle, npm, etc.)
	BuildTargets   []string `json:"build_targets,omitempty"`   // Available build targets
	PackageManager string   `json:"package_manager,omitempty"` // Package manager (npm, yarn, pip, etc.)

	// Git and version control
	VCSType       string `json:"vcs_type,omitempty"`       // Version control system type
	VCSRoot       string `json:"vcs_root,omitempty"`       // Version control root directory
	CurrentBranch string `json:"current_branch,omitempty"` // Current branch name

	// Detection metadata
	DetectedAt      time.Time     `json:"detected_at"`                // When detection was performed
	DetectionTime   time.Duration `json:"detection_time"`             // Time taken for detection
	DetectionMethod string        `json:"detection_method,omitempty"` // Method used for detection
	Confidence      float64       `json:"confidence"`                 // Detection confidence (0-1)

	// Validation status
	IsValid            bool     `json:"is_valid"`                      // Whether project structure is valid
	ValidationErrors   []string `json:"validation_errors,omitempty"`   // Validation error messages
	ValidationWarnings []string `json:"validation_warnings,omitempty"` // Validation warnings

	// Platform and environment
	Platform     platform.Platform     `json:"platform"`     // Target platform
	Architecture platform.Architecture `json:"architecture"` // Target architecture

	// Additional metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"` // Additional project-specific metadata
	Tags     []string               `json:"tags,omitempty"`     // Project tags/labels

	// Performance and resource info
	ProjectSize types.ProjectSize `json:"project_size"` // Project size metrics

	// Multi-project workspace support
	IsMonorepo    bool     `json:"is_monorepo"`              // Whether this is part of a monorepo
	SubProjects   []string `json:"sub_projects,omitempty"`   // Sub-project paths in monorepo
	ParentProject string   `json:"parent_project,omitempty"` // Parent project if this is a sub-project
}

// Language-specific context extensions
type GoProjectContext struct {
	*ProjectContext
	GoVersion string            `json:"go_version,omitempty"`
	GoMod     GoModInfo         `json:"go_mod,omitempty"`
	GoPath    string            `json:"go_path,omitempty"`
	GoRoot    string            `json:"go_root,omitempty"`
	GoEnv     map[string]string `json:"go_env,omitempty"`
}

type GoModInfo struct {
	ModuleName string            `json:"module_name"`
	GoVersion  string            `json:"go_version"`
	Requires   map[string]string `json:"requires,omitempty"`
	Replaces   map[string]string `json:"replaces,omitempty"`
	Excludes   []string          `json:"excludes,omitempty"`
}

type PythonProjectContext struct {
	*ProjectContext
	PythonVersion string        `json:"python_version,omitempty"`
	VirtualEnv    string        `json:"virtual_env,omitempty"`
	Requirements  []string      `json:"requirements,omitempty"`
	SetupPy       SetupPyInfo   `json:"setup_py,omitempty"`
	PyProject     PyProjectInfo `json:"pyproject,omitempty"`
}

type SetupPyInfo struct {
	Name         string   `json:"name,omitempty"`
	Version      string   `json:"version,omitempty"`
	Description  string   `json:"description,omitempty"`
	Author       string   `json:"author,omitempty"`
	Requirements []string `json:"requirements,omitempty"`
	Packages     []string `json:"packages,omitempty"`
}

type PyProjectInfo struct {
	Name         string            `json:"name,omitempty"`
	Version      string            `json:"version,omitempty"`
	Description  string            `json:"description,omitempty"`
	Authors      []string          `json:"authors,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	BuildSystem  string            `json:"build_system,omitempty"`
}

type NodeJSProjectContext struct {
	*ProjectContext
	NodeVersion    string          `json:"node_version,omitempty"`
	NPMVersion     string          `json:"npm_version,omitempty"`
	PackageJSON    PackageJSONInfo `json:"package_json,omitempty"`
	PackageManager string          `json:"package_manager,omitempty"` // npm, yarn, pnpm
	LockFile       string          `json:"lock_file,omitempty"`
}

type PackageJSONInfo struct {
	Name            string            `json:"name,omitempty"`
	Version         string            `json:"version,omitempty"`
	Description     string            `json:"description,omitempty"`
	Main            string            `json:"main,omitempty"`
	Scripts         map[string]string `json:"scripts,omitempty"`
	Dependencies    map[string]string `json:"dependencies,omitempty"`
	DevDependencies map[string]string `json:"dev_dependencies,omitempty"`
	Engines         map[string]string `json:"engines,omitempty"`
}

type JavaProjectContext struct {
	*ProjectContext
	JavaVersion string           `json:"java_version,omitempty"`
	BuildSystem string           `json:"build_system,omitempty"` // maven, gradle
	JavaHome    string           `json:"java_home,omitempty"`
	MavenInfo   types.MavenInfo  `json:"maven_info,omitempty"`
	GradleInfo  types.GradleInfo `json:"gradle_info,omitempty"`
}

type TypeScriptProjectContext struct {
	*ProjectContext
	TypeScriptVersion string                `json:"typescript_version,omitempty"`
	TSConfig          TSConfigInfo          `json:"tsconfig,omitempty"`
	NodeJSContext     *NodeJSProjectContext `json:"nodejs_context,omitempty"` // TypeScript often builds on Node.js
}

type TSConfigInfo struct {
	CompilerOptions map[string]interface{} `json:"compiler_options,omitempty"`
	Include         []string               `json:"include,omitempty"`
	Exclude         []string               `json:"exclude,omitempty"`
	Files           []string               `json:"files,omitempty"`
	Extends         string                 `json:"extends,omitempty"`
}

// Helper methods for ProjectContext

func NewProjectContext(projectType, rootPath string) *ProjectContext {
	return &ProjectContext{
		ProjectType:        projectType,
		RootPath:           rootPath,
		WorkspaceRoot:      rootPath,
		Languages:          make([]string, 0),
		LanguageVersions:   make(map[string]string),
		Dependencies:       make(map[string]string),
		DevDependencies:    make(map[string]string),
		RequiredServers:    make([]string, 0),
		SourceDirs:         make([]string, 0),
		TestDirs:           make([]string, 0),
		ConfigFiles:        make([]string, 0),
		MarkerFiles:        make([]string, 0),
		BuildFiles:         make([]string, 0),
		BuildTargets:       make([]string, 0),
		DetectedAt:         time.Now(),
		Confidence:         0.0,
		IsValid:            false,
		ValidationErrors:   make([]string, 0),
		ValidationWarnings: make([]string, 0),
		Platform:           platform.GetCurrentPlatform(),
		Architecture:       platform.GetCurrentArchitecture(),
		Metadata:           make(map[string]interface{}),
		Tags:               make([]string, 0),
		ProjectSize:        types.ProjectSize{},
		IsMonorepo:         false,
		SubProjects:        make([]string, 0),
	}
}

func (ctx *ProjectContext) AddLanguage(language string) {
	for _, existing := range ctx.Languages {
		if existing == language {
			return
		}
	}
	ctx.Languages = append(ctx.Languages, language)

	// Set primary language if not set
	if ctx.PrimaryLanguage == "" {
		ctx.PrimaryLanguage = language
	}
}

func (ctx *ProjectContext) AddRequiredServer(server string) {
	for _, existing := range ctx.RequiredServers {
		if existing == server {
			return
		}
	}
	ctx.RequiredServers = append(ctx.RequiredServers, server)
}

func (ctx *ProjectContext) AddMarkerFile(file string) {
	for _, existing := range ctx.MarkerFiles {
		if existing == file {
			return
		}
	}
	ctx.MarkerFiles = append(ctx.MarkerFiles, file)
}

func (ctx *ProjectContext) SetDetectionTime(duration time.Duration) {
	ctx.DetectionTime = duration
}

func (ctx *ProjectContext) SetConfidence(confidence float64) {
	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1 {
		confidence = 1
	}
	ctx.Confidence = confidence
}

func (ctx *ProjectContext) AddValidationError(error string) {
	ctx.ValidationErrors = append(ctx.ValidationErrors, error)
	ctx.IsValid = false
}

func (ctx *ProjectContext) AddValidationWarning(warning string) {
	ctx.ValidationWarnings = append(ctx.ValidationWarnings, warning)
}

func (ctx *ProjectContext) IsMultiLanguage() bool {
	return len(ctx.Languages) > 1
}

func (ctx *ProjectContext) HasLanguage(language string) bool {
	for _, lang := range ctx.Languages {
		if lang == language {
			return true
		}
	}
	return false
}

func (ctx *ProjectContext) HasServer(server string) bool {
	for _, srv := range ctx.RequiredServers {
		if srv == server {
			return true
		}
	}
	return false
}

func (ctx *ProjectContext) GetLanguageVersion(language string) (string, bool) {
	version, exists := ctx.LanguageVersions[language]
	return version, exists
}

func (ctx *ProjectContext) SetLanguageVersion(language, version string) {
	if ctx.LanguageVersions == nil {
		ctx.LanguageVersions = make(map[string]string)
	}
	ctx.LanguageVersions[language] = version
}

func (ctx *ProjectContext) GetMetadata(key string) (interface{}, bool) {
	value, exists := ctx.Metadata[key]
	return value, exists
}

func (ctx *ProjectContext) SetMetadata(key string, value interface{}) {
	if ctx.Metadata == nil {
		ctx.Metadata = make(map[string]interface{})
	}
	ctx.Metadata[key] = value
}
