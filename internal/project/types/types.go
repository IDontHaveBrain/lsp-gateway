package types

// LanguageDetectionResult contains language-specific detection results
type LanguageDetectionResult struct {
	Language        string                 `json:"language"`
	Confidence      float64                `json:"confidence"`
	Version         string                 `json:"version,omitempty"`
	MarkerFiles     []string               `json:"marker_files"`
	ConfigFiles     []string               `json:"config_files"`
	SourceDirs      []string               `json:"source_dirs"`
	TestDirs        []string               `json:"test_dirs"`
	Dependencies    map[string]string      `json:"dependencies,omitempty"`
	DevDependencies map[string]string      `json:"dev_dependencies,omitempty"`
	BuildFiles      []string               `json:"build_files"`
	RequiredServers []string               `json:"required_servers"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// ProjectSize contains metrics about project size
type ProjectSize struct {
	TotalFiles     int   `json:"total_files"`
	SourceFiles    int   `json:"source_files"`
	TestFiles      int   `json:"test_files"`
	ConfigFiles    int   `json:"config_files"`
	TotalSizeBytes int64 `json:"total_size_bytes"`
}

// GoDependencyInfo contains Go-specific dependency information
type GoDependencyInfo struct {
	Version      string   `json:"version"`
	Indirect     bool     `json:"indirect"`
	IsPrerelease bool     `json:"is_prerelease"`
	Tags         []string `json:"tags,omitempty"`
}

// PythonDependencyInfo contains Python-specific dependency information
type PythonDependencyInfo struct {
	Name        string   `json:"name"`
	Version     string   `json:"version,omitempty"`
	Specifier   string   `json:"specifier,omitempty"`
	Extras      []string `json:"extras,omitempty"`
	Environment string   `json:"environment,omitempty"`
	Source      string   `json:"source,omitempty"` // requirements.txt, pyproject.toml, etc.
}

// TypeScriptDependencyInfo contains TypeScript/JavaScript-specific dependency information
type TypeScriptDependencyInfo struct {
	Name            string   `json:"name"`
	Version         string   `json:"version"`
	Type            string   `json:"type"` // "dependency", "devDependency", "peerDependency", "optionalDependency"
	IsFramework     bool     `json:"is_framework"`
	IsBuildTool     bool     `json:"is_build_tool"`
	IsTestFramework bool     `json:"is_test_framework"`
	IsLinter        bool     `json:"is_linter"`
	Issues          []string `json:"issues,omitempty"`
}

// MavenInfo contains Maven-specific project information
type MavenInfo struct {
	GroupId      string            `json:"group_id,omitempty"`
	ArtifactId   string            `json:"artifact_id,omitempty"`
	Version      string            `json:"version,omitempty"`
	Packaging    string            `json:"packaging,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	Plugins      []string          `json:"plugins,omitempty"`
}

// GradleInfo contains Gradle-specific project information
type GradleInfo struct {
	GroupId      string            `json:"group_id,omitempty"`
	Version      string            `json:"version,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	Plugins      []string          `json:"plugins,omitempty"`
	Tasks        []string          `json:"tasks,omitempty"`
}

// LanguageInfo contains comprehensive information about a detected programming language
type LanguageInfo struct {
	Name           string                 `json:"name"`               // Language name (e.g., "go", "python")
	DisplayName    string                 `json:"display_name"`       // Human-readable name
	Version        string                 `json:"version,omitempty"`  // Detected version
	MinVersion     string                 `json:"min_version"`        // Minimum required version
	MaxVersion     string                 `json:"max_version"`        // Maximum supported version
	BuildTools     []string               `json:"build_tools"`        // Available build tools
	PackageManager string                 `json:"package_manager"`    // Default package manager
	TestFrameworks []string               `json:"test_frameworks"`    // Common test frameworks
	LintTools      []string               `json:"lint_tools"`         // Available linting tools
	FormatTools    []string               `json:"format_tools"`       // Code formatting tools
	LSPServers     []string               `json:"lsp_servers"`        // Compatible LSP servers
	FileExtensions []string               `json:"file_extensions"`    // Associated file extensions
	Capabilities   []string               `json:"capabilities"`       // Language-specific capabilities
	Metadata       map[string]interface{} `json:"metadata,omitempty"` // Additional language metadata
}
