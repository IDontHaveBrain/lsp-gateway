package testutils

import (
	"time"
)

// PredefinedLanguageConfigs provides ready-to-use configurations for common test repositories

// GetPythonLanguageConfig returns configuration for Python testing using faif/python-patterns
func GetPythonLanguageConfig() LanguageConfig {
	return LanguageConfig{
		Language:     "python",
		RepoURL:      "https://github.com/faif/python-patterns.git",
		TestPaths:    []string{"patterns"},
		FilePatterns: []string{"*.py"},
		RootMarkers:  []string{"README.md", ".git"},
		ExcludePaths: []string{"__pycache__", ".git", ".pytest_cache"},
		RepoSubDir:   "python-patterns",
		CustomVariables: map[string]string{
			"lsp_server": "pylsp",
			"transport":  "stdio",
		},
	}
}

// GetGoLanguageConfig returns configuration for Go testing using a typical Go repository
func GetGoLanguageConfig() LanguageConfig {
	return LanguageConfig{
		Language:     "go",
		RepoURL:      "https://github.com/golang/example.git",
		TestPaths:    []string{".", "cmd", "pkg", "internal"},
		FilePatterns: []string{"*.go"},
		RootMarkers:  []string{"go.mod", "go.sum"},
		ExcludePaths: []string{"vendor", ".git", "testdata"},
		RepoSubDir:   "example",
		CustomVariables: map[string]string{
			"lsp_server": "gopls",
			"transport":  "stdio",
		},
	}
}

// GetJavaScriptLanguageConfig returns configuration for JavaScript/TypeScript testing using chalk repository with fixed commit
func GetJavaScriptLanguageConfig() LanguageConfig {
	return LanguageConfig{
		Language:     "javascript",
		RepoURL:      "https://github.com/chalk/chalk.git",
		TestPaths:    []string{"source", "test"},
		FilePatterns: []string{"*.js", "*.ts", "*.mjs", "*.jsx", "*.tsx"},
		RootMarkers:  []string{"package.json", "tsconfig.json"},
		ExcludePaths: []string{"node_modules", ".git", "dist", "build", "coverage"},
		RepoSubDir:   "chalk",
		CustomVariables: map[string]string{
			"lsp_server":   "typescript-language-server",
			"transport":    "stdio",
			"commit_hash":  "5dbc1e2", // Fixed commit hash for v5.4.1
		},
	}
}

// GetJavaLanguageConfig returns configuration for Java testing
func GetJavaLanguageConfig() LanguageConfig {
	return LanguageConfig{
		Language:     "java",
		RepoURL:      "https://github.com/spring-projects/spring-boot.git",
		TestPaths:    []string{"spring-boot-project/spring-boot/src/main/java"},
		FilePatterns: []string{"*.java"},
		RootMarkers:  []string{"pom.xml", "build.gradle"},
		ExcludePaths: []string{"target", ".git", ".m2"},
		RepoSubDir:   "spring-boot",
		CustomVariables: map[string]string{
			"lsp_server": "jdtls",
			"transport":  "stdio",
		},
	}
}

// GetRustLanguageConfig returns configuration for Rust testing
func GetRustLanguageConfig() LanguageConfig {
	return LanguageConfig{
		Language:     "rust",
		RepoURL:      "https://github.com/rust-lang/cargo.git",
		TestPaths:    []string{"src"},
		FilePatterns: []string{"*.rs"},
		RootMarkers:  []string{"Cargo.toml", "Cargo.lock"},
		ExcludePaths: []string{"target", ".git"},
		RepoSubDir:   "cargo",
		CustomVariables: map[string]string{
			"lsp_server": "rust-analyzer",
			"transport":  "stdio",
		},
	}
}

// Language-specific repository manager factory functions

// NewPythonRepositoryManager creates a repository manager configured for Python testing
func NewPythonRepositoryManager(customConfig ...GenericRepoConfig) *GenericRepoManager {
	var config GenericRepoConfig
	if len(customConfig) > 0 {
		config = customConfig[0]
	} else {
		config = GenericRepoConfig{
			CloneTimeout:   300 * time.Second,
			EnableLogging:  true,
			ForceClean:     false,
			PreserveGitDir: false,
		}
	}
	
	config.LanguageConfig = GetPythonLanguageConfig()
	return NewGenericRepoManager(config)
}

// NewGoRepositoryManager creates a repository manager configured for Go testing
func NewGoRepositoryManager(customConfig ...GenericRepoConfig) *GenericRepoManager {
	var config GenericRepoConfig
	if len(customConfig) > 0 {
		config = customConfig[0]
	} else {
		config = GenericRepoConfig{
			CloneTimeout:   300 * time.Second,
			EnableLogging:  true,
			ForceClean:     false,
			PreserveGitDir: false,
		}
	}
	
	config.LanguageConfig = GetGoLanguageConfig()
	return NewGenericRepoManager(config)
}

// NewJavaScriptRepositoryManager creates a repository manager configured for JavaScript/TypeScript testing with chalk repository
func NewJavaScriptRepositoryManager(customConfig ...GenericRepoConfig) *GenericRepoManager {
	var config GenericRepoConfig
	if len(customConfig) > 0 {
		config = customConfig[0]
	} else {
		config = GenericRepoConfig{
			CloneTimeout:   300 * time.Second,
			EnableLogging:  true,
			ForceClean:     false,
			PreserveGitDir: true,  // Keep .git for commit checkout
		}
	}
	
	langConfig := GetJavaScriptLanguageConfig()
	config.LanguageConfig = langConfig
	
	// Extract commit hash from language config if not already set
	if config.CommitHash == "" {
		if commitHash, exists := langConfig.CustomVariables["commit_hash"]; exists {
			config.CommitHash = commitHash
		}
	}
	
	return NewGenericRepoManager(config)
}

// NewJavaRepositoryManager creates a repository manager configured for Java testing
func NewJavaRepositoryManager(customConfig ...GenericRepoConfig) *GenericRepoManager {
	var config GenericRepoConfig
	if len(customConfig) > 0 {
		config = customConfig[0]
	} else {
		config = GenericRepoConfig{
			CloneTimeout:   300 * time.Second,
			EnableLogging:  true,
			ForceClean:     false,
			PreserveGitDir: false,
		}
	}
	
	config.LanguageConfig = GetJavaLanguageConfig()
	return NewGenericRepoManager(config)
}

// NewRustRepositoryManager creates a repository manager configured for Rust testing
func NewRustRepositoryManager(customConfig ...GenericRepoConfig) *GenericRepoManager {
	var config GenericRepoConfig
	if len(customConfig) > 0 {
		config = customConfig[0]
	} else {
		config = GenericRepoConfig{
			CloneTimeout:   300 * time.Second,
			EnableLogging:  true,
			ForceClean:     false,
			PreserveGitDir: false,
		}
	}
	
	config.LanguageConfig = GetRustLanguageConfig()
	return NewGenericRepoManager(config)
}

// Custom configuration builders

// CustomLanguageConfig allows creating a custom language configuration
type CustomLanguageConfig struct {
	language        string
	repoURL         string
	testPaths       []string
	filePatterns    []string
	rootMarkers     []string
	excludePaths    []string
	repoSubDir      string
	customVariables map[string]string
}

// NewCustomLanguageConfig creates a new custom language configuration builder
func NewCustomLanguageConfig(language, repoURL string) *CustomLanguageConfig {
	return &CustomLanguageConfig{
		language:        language,
		repoURL:         repoURL,
		testPaths:       []string{"."},
		filePatterns:    []string{"*"},
		rootMarkers:     []string{},
		excludePaths:    []string{".git"},
		customVariables: make(map[string]string),
	}
}

// WithTestPaths sets the test paths for the custom configuration
func (clc *CustomLanguageConfig) WithTestPaths(paths ...string) *CustomLanguageConfig {
	clc.testPaths = paths
	return clc
}

// WithFilePatterns sets the file patterns for the custom configuration
func (clc *CustomLanguageConfig) WithFilePatterns(patterns ...string) *CustomLanguageConfig {
	clc.filePatterns = patterns
	return clc
}

// WithRootMarkers sets the root markers for the custom configuration
func (clc *CustomLanguageConfig) WithRootMarkers(markers ...string) *CustomLanguageConfig {
	clc.rootMarkers = markers
	return clc
}

// WithExcludePaths sets the exclude paths for the custom configuration
func (clc *CustomLanguageConfig) WithExcludePaths(paths ...string) *CustomLanguageConfig {
	clc.excludePaths = paths
	return clc
}

// WithRepoSubDir sets the repository subdirectory for the custom configuration
func (clc *CustomLanguageConfig) WithRepoSubDir(subDir string) *CustomLanguageConfig {
	clc.repoSubDir = subDir
	return clc
}

// WithCustomVariable adds a custom variable to the configuration
func (clc *CustomLanguageConfig) WithCustomVariable(key, value string) *CustomLanguageConfig {
	if clc.customVariables == nil {
		clc.customVariables = make(map[string]string)
	}
	clc.customVariables[key] = value
	return clc
}

// Build creates the final LanguageConfig from the builder
func (clc *CustomLanguageConfig) Build() LanguageConfig {
	return LanguageConfig{
		Language:        clc.language,
		RepoURL:         clc.repoURL,
		TestPaths:       clc.testPaths,
		FilePatterns:    clc.filePatterns,
		RootMarkers:     clc.rootMarkers,
		ExcludePaths:    clc.excludePaths,
		RepoSubDir:      clc.repoSubDir,
		CustomVariables: clc.customVariables,
	}
}

// NewCustomRepositoryManager creates a repository manager with custom configuration
func NewCustomRepositoryManager(langConfig LanguageConfig, repoConfig ...GenericRepoConfig) *GenericRepoManager {
	var config GenericRepoConfig
	if len(repoConfig) > 0 {
		config = repoConfig[0]
	} else {
		config = GenericRepoConfig{
			CloneTimeout:   300 * time.Second,
			EnableLogging:  true,
			ForceClean:     false,
			PreserveGitDir: false,
		}
	}
	
	config.LanguageConfig = langConfig
	return NewGenericRepoManager(config)
}