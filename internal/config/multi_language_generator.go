package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"lsp-gateway/internal/installer"
)

const (
	// Indexing strategies (smart is unique to this file)
	IndexingStrategySmart = "smart"

	// Monorepo strategies
	MonorepoStrategyLanguageSeparated = "language-separated"
	MonorepoStrategyMixed             = "mixed"
	MonorepoStrategyMicroservices     = "microservices"
)

type ServerConfigTemplate struct {
	Name                  string                            `yaml:"name"`
	Command               string                            `yaml:"command"`
	Args                  []string                          `yaml:"args,omitempty"`
	Transport             string                            `yaml:"transport"`
	RootMarkers           []string                          `yaml:"root_markers"`
	Settings              map[string]interface{}            `yaml:"settings,omitempty"`
	LanguageSettings      map[string]map[string]interface{} `yaml:"language_settings,omitempty"`
	ServerType            string                            `yaml:"server_type,omitempty"`
	Priority              int                               `yaml:"priority,omitempty"`
	Weight                float64                           `yaml:"weight,omitempty"`
	Constraints           *ServerConstraints                `yaml:"constraints,omitempty"`
	FrameworkEnhancements map[string]map[string]interface{} `yaml:"framework_enhancements,omitempty"`
	BypassTemplate        *ServerBypassConfig               `yaml:"bypass_template,omitempty"`
}

type ConfigGenerator struct {
	templates          map[string]*ServerConfigTemplate
	frameworkEnhancers map[string]FrameworkEnhancer
	monorepoStrategies map[string]MonorepoStrategy
	optimizationModes  map[string]OptimizationStrategy
	bypassTemplates    map[string]*LanguageBypassConfig
	bypassDefaults     *GlobalBypassConfig
}

type FrameworkEnhancer interface {
	EnhanceConfig(config *ServerConfig, framework *Framework) error
	GetRequiredSettings() map[string]interface{}
	GetRecommendedExtensions() []string
}

type MonorepoStrategy interface {
	GenerateConfig(projectInfo *MultiLanguageProjectInfo) (*MultiLanguageConfig, error)
	GetWorkspaceLayout(layout *MonorepoLayout) map[string]string
	OptimizeForLayout(config *MultiLanguageConfig, layout *MonorepoLayout) error
}

func NewConfigGenerator() *ConfigGenerator {
	generator := &ConfigGenerator{
		templates:          make(map[string]*ServerConfigTemplate),
		frameworkEnhancers: make(map[string]FrameworkEnhancer),
		monorepoStrategies: make(map[string]MonorepoStrategy),
		optimizationModes:  make(map[string]OptimizationStrategy),
		bypassTemplates:    make(map[string]*LanguageBypassConfig),
		bypassDefaults:     nil,
	}

	generator.initializeDefaultTemplates()
	generator.initializeFrameworkEnhancers()
	generator.initializeMonorepoStrategies()
	generator.initializeOptimizationModes()
	generator.initializeBypassTemplates()

	return generator
}

// detectAvailableJavaRuntimes detects available Java runtimes on the current platform
func detectAvailableJavaRuntimes() []map[string]interface{} {
	var runtimes []map[string]interface{}

	// Check JAVA_HOME first
	if javaHome := os.Getenv("JAVA_HOME"); javaHome != "" {
		if _, err := os.Stat(javaHome); err == nil {
			runtimes = append(runtimes, map[string]interface{}{
				"name":    "JAVA_HOME",
				"path":    javaHome,
				"default": true,
			})
		}
	}

	// Platform-specific Java detection
	switch runtime.GOOS {
	case "darwin": // macOS
		macPaths := []string{
			"/Library/Java/JavaVirtualMachines",
			"/opt/homebrew/Cellar/openjdk",
			"/usr/local/Cellar/openjdk",
		}
		for _, basePath := range macPaths {
			if entries, err := os.ReadDir(basePath); err == nil {
				for _, entry := range entries {
					if entry.IsDir() {
						javaPath := filepath.Join(basePath, entry.Name(), "Contents", "Home")
						if _, err := os.Stat(javaPath); err == nil {
							runtimes = append(runtimes, map[string]interface{}{
								"name": "JavaSE-" + extractJavaVersion(entry.Name()),
								"path": javaPath,
							})
						}
					}
				}
			}
		}

	case "linux":
		linuxPaths := []string{
			"/usr/lib/jvm",
			"/opt/java",
			"/usr/local/java",
		}
		for _, basePath := range linuxPaths {
			if entries, err := os.ReadDir(basePath); err == nil {
				for _, entry := range entries {
					if entry.IsDir() {
						javaPath := filepath.Join(basePath, entry.Name())
						if _, err := os.Stat(filepath.Join(javaPath, "bin", "java")); err == nil {
							runtimes = append(runtimes, map[string]interface{}{
								"name": "JavaSE-" + extractJavaVersion(entry.Name()),
								"path": javaPath,
							})
						}
					}
				}
			}
		}

	case "windows":
		windowsPaths := []string{
			"C:\\Program Files\\Java",
			"C:\\Program Files (x86)\\Java",
		}
		for _, basePath := range windowsPaths {
			if entries, err := os.ReadDir(basePath); err == nil {
				for _, entry := range entries {
					if entry.IsDir() {
						javaPath := filepath.Join(basePath, entry.Name())
						if _, err := os.Stat(filepath.Join(javaPath, "bin", "java.exe")); err == nil {
							runtimes = append(runtimes, map[string]interface{}{
								"name": "JavaSE-" + extractJavaVersion(entry.Name()),
								"path": javaPath,
							})
						}
					}
				}
			}
		}
	}

	// If no runtimes found, provide fallback defaults
	if len(runtimes) == 0 {
		runtimes = []map[string]interface{}{
			{
				"name":    "JavaSE-11",
				"path":    "/usr/lib/jvm/java-11-openjdk",
				"default": true,
			},
			{
				"name": "JavaSE-17",
				"path": "/usr/lib/jvm/java-17-openjdk",
			},
		}
	}

	return runtimes
}

// extractJavaVersion extracts Java version from directory name
func extractJavaVersion(dirName string) string {
	// Common patterns: jdk-11, openjdk-17, java-11-openjdk, 11.0.2, etc.
	if strings.Contains(dirName, "11") {
		return "11"
	}
	if strings.Contains(dirName, "17") {
		return "17"
	}
	if strings.Contains(dirName, "21") {
		return "21"
	}
	if strings.Contains(dirName, "8") {
		return "8"
	}
	return "11" // Default fallback
}

func (g *ConfigGenerator) initializeDefaultTemplates() {
	g.templates = map[string]*ServerConfigTemplate{
		"go": {
			Name:        "gopls",
			Command:     "gopls",
			Transport:   "stdio",
			RootMarkers: []string{"go.mod", "go.work", "go.sum"},
			ServerType:  ServerTypeMulti,
			Priority:    10,
			Weight:      1.0,
			Settings: map[string]interface{}{
				"gopls": map[string]interface{}{
					"analyses": map[string]bool{
						"unusedparams":   true,
						"shadow":         true,
						"unusedwrite":    true,
						"useany":         true,
						"nilness":        true,
						"fieldalignment": false,
					},
					"staticcheck":        true,
					"gofumpt":            true,
					"usePlaceholders":    true,
					"completeUnimported": true,
					"deepCompletion":     true,
					"semanticTokens":     true,
					"codelenses": map[string]bool{
						"gc_details":         true,
						"generate":           true,
						"regenerate_cgo":     true,
						"test":               true,
						"tidy":               true,
						"upgrade_dependency": true,
						"vendor":             true,
					},
				},
			},
			Constraints: &ServerConstraints{
				RequiredMarkers: []string{"go.mod"},
				ProjectTypes:    []string{"go", "monorepo", "multi-language"},
				MinFileCount:    1,
			},
			FrameworkEnhancements: map[string]map[string]interface{}{
				"gin": {
					"gopls": map[string]interface{}{
						"annotations": map[string]bool{
							"bounds": true,
							"escape": false,
						},
					},
				},
				"kubernetes": {
					"gopls": map[string]interface{}{
						"buildFlags": []string{"-tags=ignore_autogenerated"},
					},
				},
			},
		},
		"python": {
			Name:        "python-lsp-server",
			Command:     "python",
			Args:        []string{"-m", "pylsp"},
			Transport:   "stdio",
			RootMarkers: []string{"pyproject.toml", "setup.py", "requirements.txt", "Pipfile", "poetry.lock"},
			ServerType:  ServerTypeMulti,
			Priority:    9,
			Weight:      1.0,
			Settings: map[string]interface{}{
				"pylsp": map[string]interface{}{
					"configurationSources": []string{"pycodestyle", "flake8"},
					"plugins": map[string]interface{}{
						"pycodestyle": map[string]bool{"enabled": true},
						"pyflakes":    map[string]bool{"enabled": true},
						"pylint":      map[string]bool{"enabled": false},
						"rope_completion": map[string]interface{}{
							"enabled": true,
							"eager":   true,
						},
						"rope_autoimport": map[string]interface{}{
							"enabled":      true,
							"completions":  map[string]bool{"enabled": true},
							"code_actions": map[string]bool{"enabled": true},
						},
						"mypy-ls": map[string]interface{}{
							"enabled":   true,
							"live_mode": false,
							"strict":    false,
						},
						"black": map[string]interface{}{
							"enabled":     true,
							"line_length": 88,
							"cache":       "normal",
						},
						"isort":    map[string]bool{"enabled": true},
						"autopep8": map[string]bool{"enabled": false},
						"yapf":     map[string]bool{"enabled": false},
					},
				},
			},
			Constraints: &ServerConstraints{
				RequiredMarkers: []string{}, // No specific markers required, Python detection is based on file existence
				ProjectTypes:    []string{"python", "monorepo", "multi-language"},
				MinFileCount:    1,
			},
			FrameworkEnhancements: map[string]map[string]interface{}{
				"django": {
					"pylsp": map[string]interface{}{
						"plugins": map[string]interface{}{
							"pylsp_django": map[string]bool{"enabled": true},
						},
					},
				},
				"flask": {
					"pylsp": map[string]interface{}{
						"plugins": map[string]interface{}{
							"rope_completion": map[string]interface{}{
								"enabled": true,
								"eager":   true,
							},
						},
					},
				},
				"fastapi": {
					"pylsp": map[string]interface{}{
						"plugins": map[string]interface{}{
							"pydantic": map[string]bool{"enabled": true},
						},
					},
				},
			},
		},
		"typescript": {
			Name:        "typescript-language-server",
			Command:     "typescript-language-server",
			Args:        []string{"--stdio"},
			Transport:   "stdio",
			RootMarkers: []string{"tsconfig.json", "package.json", "tsconfig.build.json"},
			ServerType:  ServerTypeMulti,
			Priority:    8,
			Weight:      1.0,
			Settings: map[string]interface{}{
				"typescript": map[string]interface{}{
					"updateImportsOnFileMove": map[string]string{
						"enabled": "always",
					},
					"suggest": map[string]interface{}{
						"completeFunctionCalls":              true,
						"includeCompletionsForModuleExports": true,
					},
					"preferences": map[string]interface{}{
						"strictNullChecks":      true,
						"strictFunctionTypes":   true,
						"noImplicitReturns":     true,
						"noImplicitAny":         true,
						"importModuleSpecifier": "relative",
					},
					"inlayHints": map[string]interface{}{
						"parameterNames": map[string]string{
							"enabled": "literals",
						},
						"parameterTypes": map[string]bool{
							"enabled": false,
						},
						"variableTypes": map[string]bool{
							"enabled": false,
						},
						"propertyDeclarationTypes": map[string]bool{
							"enabled": false,
						},
						"functionLikeReturnTypes": map[string]bool{
							"enabled": false,
						},
						"enumMemberValues": map[string]bool{
							"enabled": false,
						},
					},
				},
				"javascript": map[string]interface{}{
					"suggest": map[string]interface{}{
						"completeFunctionCalls":              true,
						"includeCompletionsForModuleExports": true,
					},
					"preferences": map[string]interface{}{
						"checkJs":               true,
						"importModuleSpecifier": "relative",
					},
				},
			},
			LanguageSettings: map[string]map[string]interface{}{
				"typescript": {
					"preferences": map[string]interface{}{
						"strictNullChecks":    true,
						"strictFunctionTypes": true,
					},
				},
				"javascript": {
					"preferences": map[string]interface{}{
						"checkJs": true,
					},
				},
			},
			Constraints: &ServerConstraints{
				RequiredMarkers: []string{"package.json"}, // Only require package.json to support both TS and JS projects
				ProjectTypes:    []string{"typescript", "javascript", "monorepo", "multi-language", "frontend-backend"},
				MinFileCount:    1,
			},
			FrameworkEnhancements: map[string]map[string]interface{}{
				"react": {
					"typescript": map[string]interface{}{
						"jsx": "react-jsx",
						"preferences": map[string]interface{}{
							"jsxAttributeCompletionStyle": "auto",
						},
					},
				},
				"vue": {
					"typescript": map[string]interface{}{
						"preferences": map[string]interface{}{
							"disableSuggestions": false,
						},
					},
				},
				"angular": {
					"typescript": map[string]interface{}{
						"preferences": map[string]interface{}{
							"strictTemplates": true,
						},
					},
				},
				"nextjs": {
					"typescript": map[string]interface{}{
						"preferences": map[string]interface{}{
							"jsxAttributeCompletionStyle":   "auto",
							"includePackageJsonAutoImports": "auto",
						},
					},
				},
			},
		},
		"java": {
			Name:        "eclipse-jdtls",
			Command:     installer.GetJDTLSExecutablePath(),
			Args:        installer.GetJDTLSArgs(),
			Transport:   "stdio",
			RootMarkers: []string{"pom.xml", "build.gradle", "build.gradle.kts", ".project", "settings.gradle"},
			ServerType:  ServerTypeWorkspace,
			Priority:    7,
			Weight:      1.0,
			Settings: map[string]interface{}{
				"java": map[string]interface{}{
					"configuration": map[string]interface{}{
						"runtimes": detectAvailableJavaRuntimes(),
					},
					"compile": map[string]interface{}{
						"nullAnalysis": map[string]interface{}{
							"mode": "automatic",
						},
					},
					"completion": map[string]interface{}{
						"enabled":              true,
						"overwrite":            true,
						"guessMethodArguments": true,
						"favoriteStaticMembers": []string{
							"org.junit.Assert.*",
							"org.junit.Assume.*",
							"org.junit.jupiter.api.Assertions.*",
							"org.junit.jupiter.api.Assumptions.*",
							"org.mockito.Mockito.*",
							"org.springframework.test.web.servlet.request.MockMvcRequestBuilders.*",
							"org.springframework.test.web.servlet.result.MockMvcResultMatchers.*",
						},
					},
					"contentProvider": map[string]string{
						"preferred": "fernflower",
					},
					"import": map[string]interface{}{
						"gradle": map[string]interface{}{
							"enabled": true,
							"wrapper": map[string]bool{
								"enabled": true,
							},
						},
						"maven": map[string]interface{}{
							"enabled": true,
						},
						"exclusions": []string{
							"**/node_modules/**",
							"**/.metadata/**",
							"**/archetype-resources/**",
							"**/META-INF/maven/**",
						},
					},
					"maven": map[string]interface{}{
						"downloadSources": true,
						"downloadJavadoc": false,
						"updateSnapshots": false,
					},
					"referencesCodeLens": map[string]bool{
						"enabled": true,
					},
					"signatureHelp": map[string]bool{
						"enabled": true,
					},
					"implementationsCodeLens": map[string]bool{
						"enabled": true,
					},
					"format": map[string]interface{}{
						"enabled": true,
						"settings": map[string]interface{}{
							"url":     "https://raw.githubusercontent.com/google/styleguide/gh-pages/eclipse-java-google-style.xml",
							"profile": "GoogleStyle",
						},
					},
				},
			},
			Constraints: &ServerConstraints{
				RequiredMarkers: []string{"pom.xml", "build.gradle"},
				ProjectTypes:    []string{"java", "monorepo", "multi-language", "microservices"},
				MinFileCount:    1,
			},
			FrameworkEnhancements: map[string]map[string]interface{}{
				"spring-boot": {
					"java": map[string]interface{}{
						"completion": map[string]interface{}{
							"favoriteStaticMembers": []string{
								"org.springframework.test.web.servlet.request.MockMvcRequestBuilders.*",
								"org.springframework.test.web.servlet.result.MockMvcResultMatchers.*",
								"org.springframework.boot.test.context.SpringBootTest.*",
							},
						},
					},
				},
			},
		},
		"rust": {
			Name:        "rust-analyzer",
			Command:     "rust-analyzer",
			Transport:   "stdio",
			RootMarkers: []string{"Cargo.toml", "rust-project.json"},
			ServerType:  ServerTypeWorkspace,
			Priority:    8,
			Weight:      1.0,
			Settings: map[string]interface{}{
				"rust-analyzer": map[string]interface{}{
					"assist": map[string]interface{}{
						"importGranularity": "module",
						"importPrefix":      "by_self",
					},
					"cargo": map[string]interface{}{
						"loadOutDirsFromCheck":           true,
						"runBuildScripts":                true,
						"useRustcWrapperForBuildScripts": true,
					},
					"procMacro": map[string]interface{}{
						"enable": true,
						"ignored": map[string]interface{}{
							"leptos_macro": []string{"component", "server"},
						},
					},
					"checkOnSave": map[string]interface{}{
						"command": "clippy",
					},
					"completion": map[string]interface{}{
						"addCallArgumentSnippets": true,
						"addCallParenthesis":      true,
						"postfix": map[string]bool{
							"enable": true,
						},
						"autoimport": map[string]bool{
							"enable": true,
						},
					},
					"diagnostics": map[string]interface{}{
						"enabled": true,
						"experimental": map[string]bool{
							"enable": false,
						},
					},
					"hover": map[string]interface{}{
						"actions": map[string]bool{
							"enable": true,
						},
						"links": map[string]bool{
							"enable": true,
						},
					},
					"inlayHints": map[string]interface{}{
						"bindingModeHints": map[string]bool{
							"enable": false,
						},
						"chainingHints": map[string]bool{
							"enable": true,
						},
						"closingBraceHints": map[string]interface{}{
							"enable":   true,
							"minLines": 25,
						},
						"closureReturnTypeHints": map[string]string{
							"enable": "never",
						},
						"lifetimeElisionHints": map[string]interface{}{
							"enable":            "never",
							"useParameterNames": false,
						},
						"maxLength": 25,
						"parameterHints": map[string]bool{
							"enable": true,
						},
						"reborrowHints": map[string]string{
							"enable": "never",
						},
						"renderColons": true,
						"typeHints": map[string]interface{}{
							"enable":                    true,
							"hideClosureInitialization": false,
							"hideNamedConstructor":      false,
						},
					},
					"lens": map[string]interface{}{
						"enable": true,
						"implementations": map[string]bool{
							"enable": true,
						},
						"references": map[string]bool{
							"adt.enable":         false,
							"enumVariant.enable": false,
							"method.enable":      false,
							"trait.enable":       false,
						},
						"run": map[string]bool{
							"enable": true,
						},
					},
				},
			},
			Constraints: &ServerConstraints{
				RequiredMarkers: []string{"Cargo.toml"},
				ProjectTypes:    []string{"rust", "monorepo", "multi-language"},
				MinFileCount:    1,
			},
		},
		"cpp": {
			Name:        "clangd",
			Command:     "clangd",
			Args:        []string{"--background-index", "--suggest-missing-includes", "--clang-tidy", "--header-insertion=iwyu"},
			Transport:   "stdio",
			RootMarkers: []string{"compile_commands.json", "compile_flags.txt", ".clangd", "CMakeLists.txt", "Makefile"},
			ServerType:  ServerTypeWorkspace,
			Priority:    6,
			Weight:      1.0,
			Settings: map[string]interface{}{
				"clangd": map[string]interface{}{
					"arguments": []string{
						"--background-index",
						"--suggest-missing-includes",
						"--clang-tidy",
						"--header-insertion=iwyu",
						"--completion-style=detailed",
						"--function-arg-placeholders",
						"--fallback-style=llvm",
					},
				},
			},
			Constraints: &ServerConstraints{
				RequiredMarkers: []string{}, // No specific markers required, C++ detection is based on file existence
				ProjectTypes:    []string{"cpp", "c", "monorepo", "multi-language"},
				MinFileCount:    1,
			},
		},
	}
}

func (g *ConfigGenerator) initializeFrameworkEnhancers() {
	g.frameworkEnhancers["react"] = &ReactEnhancer{}
	g.frameworkEnhancers["django"] = &DjangoEnhancer{}
	g.frameworkEnhancers["spring-boot"] = &SpringBootEnhancer{}
	g.frameworkEnhancers["kubernetes"] = &KubernetesEnhancer{}
}

func (g *ConfigGenerator) initializeMonorepoStrategies() {
	g.monorepoStrategies[MonorepoStrategyLanguageSeparated] = &LanguageSeparatedStrategy{}
	g.monorepoStrategies[MonorepoStrategyMixed] = &MixedStrategy{}
	g.monorepoStrategies[MonorepoStrategyMicroservices] = &MicroservicesStrategy{}
}

func (g *ConfigGenerator) initializeOptimizationModes() {
	g.optimizationModes[PerformanceProfileDevelopment] = NewDevelopmentOptimization()
	g.optimizationModes[PerformanceProfileProduction] = NewProductionOptimization()
	g.optimizationModes[PerformanceProfileAnalysis] = NewAnalysisOptimization()
}

func (g *ConfigGenerator) initializeBypassTemplates() {
	// Load bypass defaults from template
	g.bypassDefaults = g.loadGlobalBypassDefaults()

	// Load language-specific bypass templates
	g.bypassTemplates["go"] = g.loadLanguageBypassTemplate("go")
	g.bypassTemplates["python"] = g.loadLanguageBypassTemplate("python")
	g.bypassTemplates["typescript"] = g.loadLanguageBypassTemplate("typescript")
	g.bypassTemplates["javascript"] = g.loadLanguageBypassTemplate("javascript")
	g.bypassTemplates["java"] = g.loadLanguageBypassTemplate("java")
}

func (g *ConfigGenerator) GenerateMultiLanguageConfig(projectInfo *MultiLanguageProjectInfo) (*MultiLanguageConfig, error) {
	if projectInfo == nil {
		return nil, fmt.Errorf("project info cannot be nil")
	}

	config := &MultiLanguageConfig{
		ProjectInfo:   projectInfo,
		ServerConfigs: []*ServerConfig{},
		WorkspaceConfig: &WorkspaceConfig{
			MultiRoot:               len(projectInfo.LanguageContexts) > 1,
			LanguageRoots:           make(map[string]string),
			SharedSettings:          make(map[string]interface{}),
			CrossLanguageReferences: true,
		},
		OptimizedFor: PerformanceProfileDevelopment,
		GeneratedAt:  time.Now(),
		Version:      "1.0",
		Metadata:     make(map[string]interface{}),
	}

	// Generate server configurations for each language context
	for _, langCtx := range projectInfo.LanguageContexts {
		serverConfig, err := g.GenerateServerConfig(langCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to generate server config for %s: %w", langCtx.Language, err)
		}

		if serverConfig != nil {
			config.ServerConfigs = append(config.ServerConfigs, serverConfig)
			config.WorkspaceConfig.LanguageRoots[langCtx.Language] = langCtx.RootPath
		}
	}

	// Apply framework-specific enhancements
	if err := g.applyFrameworkEnhancements(config, projectInfo.Frameworks); err != nil {
		return nil, fmt.Errorf("failed to apply framework enhancements: %w", err)
	}

	// Apply monorepo-specific configurations
	if projectInfo.MonorepoLayout != nil {
		if err := g.applyMonorepoStrategy(config, projectInfo.MonorepoLayout); err != nil {
			return nil, fmt.Errorf("failed to apply monorepo strategy: %w", err)
		}
	}

	// Optimize configuration based on project type
	if err := g.OptimizeForProjectType(config, projectInfo.ProjectType); err != nil {
		return nil, fmt.Errorf("failed to optimize for project type: %w", err)
	}

	return config, nil
}

// GenerateDefaultMultiLanguageConfig generates a default multi-language configuration
// with support for common languages when project detection fails
func (g *ConfigGenerator) GenerateDefaultMultiLanguageConfig() (*MultiLanguageConfig, error) {
	// Get current working directory as absolute path
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Create default project info with common languages
	defaultProjectInfo := &MultiLanguageProjectInfo{
		ProjectType:   ProjectTypeMulti,
		RootDirectory: currentDir,
		WorkspaceRoot: currentDir,
		LanguageContexts: []*LanguageContext{
			// Java configuration
			{
				Language:     "java",
				FilePatterns: []string{"*.java"},
				RootMarkers:  []string{"pom.xml", "build.gradle", "build.gradle.kts"},
				RootPath:     currentDir,
				FileCount:    10,
			},
			// Python configuration
			{
				Language:     "python",
				FilePatterns: []string{"*.py"},
				RootMarkers:  []string{"pyproject.toml", "requirements.txt", "setup.py"},
				RootPath:     currentDir,
				FileCount:    10,
			},
			// Go configuration
			{
				Language:     "go",
				FilePatterns: []string{"*.go"},
				RootMarkers:  []string{"go.mod", "go.sum"},
				RootPath:     currentDir,
				FileCount:    10,
			},
			// TypeScript/JavaScript configuration
			{
				Language:     "typescript",
				FilePatterns: []string{"*.ts", "*.tsx", "*.js", "*.jsx"},
				RootMarkers:  []string{"tsconfig.json", "package.json"},
				RootPath:     currentDir,
				FileCount:    10,
			},
		},
		DetectedAt: time.Now(),
		Metadata:   make(map[string]interface{}),
	}

	return g.GenerateMultiLanguageConfig(defaultProjectInfo)
}

func (g *ConfigGenerator) GenerateServerConfig(langCtx *LanguageContext) (*ServerConfig, error) {
	// Normalize language name to match template keys
	normalizedLanguage := g.normalizeLanguageName(langCtx.Language)

	template, exists := g.templates[normalizedLanguage]
	if !exists {
		return nil, fmt.Errorf("no template found for language: %s (normalized from: %s)", normalizedLanguage, langCtx.Language)
	}

	// Map language name for Languages field (for request routing)
	mappedLanguage := g.mapLanguageForRouting(langCtx.Language)

	// Create server config from template
	serverConfig := &ServerConfig{
		Name:             template.Name,
		Languages:        []string{mappedLanguage},
		Command:          template.Command,
		Args:             make([]string, len(template.Args)),
		Transport:        template.Transport,
		RootMarkers:      make([]string, len(template.RootMarkers)),
		Settings:         deepCopyMap(template.Settings),
		LanguageSettings: deepCopyLanguageSettingsMap(template.LanguageSettings),
		Priority:         template.Priority,
		Weight:           template.Weight,
		ServerType:       template.ServerType,
		Constraints:      deepCopyServerConstraints(template.Constraints),
		Frameworks:       make([]string, len(langCtx.Frameworks)),
		Version:          langCtx.Version,
		WorkspaceRoots:   map[string]string{langCtx.Language: langCtx.RootPath},
	}

	copy(serverConfig.Args, template.Args)
	copy(serverConfig.RootMarkers, template.RootMarkers)
	copy(serverConfig.Frameworks, langCtx.Frameworks)

	// Apply project-specific workspace configuration for workspace-type servers
	if err := g.applyProjectSpecificWorkspaceConfig(serverConfig, langCtx); err != nil {
		return nil, fmt.Errorf("failed to apply project-specific workspace config: %w", err)
	}

	// Apply language context specific settings
	if err := g.applyLanguageContextSettings(serverConfig, langCtx); err != nil {
		return nil, fmt.Errorf("failed to apply language context settings: %w", err)
	}

	// Apply bypass configuration
	if err := g.applyBypassConfiguration(serverConfig, langCtx); err != nil {
		return nil, fmt.Errorf("failed to apply bypass configuration: %w", err)
	}

	// Validate constraints against language context
	if !g.validateConstraints(serverConfig.Constraints, langCtx) {
		return nil, fmt.Errorf("server constraints not satisfied for language %s", langCtx.Language)
	}

	return serverConfig, nil
}

// applyProjectSpecificWorkspaceConfig applies project-specific workspace configuration
// for workspace-type servers that require isolated workspace directories
func (g *ConfigGenerator) applyProjectSpecificWorkspaceConfig(serverConfig *ServerConfig, langCtx *LanguageContext) error {
	// Only apply to workspace-type servers
	if serverConfig.ServerType != ServerTypeWorkspace {
		return nil
	}

	// Apply project-specific configuration based on server name
	switch serverConfig.Name {
	case "eclipse-jdtls":
		// Replace static JDTLS args with project-specific workspace
		if langCtx.RootPath != "" {
			serverConfig.Args = installer.GetJDTLSArgsForProject(langCtx.RootPath)
		} else {
			// Fallback to legacy behavior if no project root available
			serverConfig.Args = installer.GetJDTLSArgs()
		}

	// Future workspace-type servers can be added here
	// case "rust-analyzer":
	//     // Apply Rust-specific workspace configuration
	// case "omnisharp":
	//     // Apply C#-specific workspace configuration

	default:
		// For other workspace-type servers, keep existing args
	}

	return nil
}

func (g *ConfigGenerator) applyLanguageContextSettings(serverConfig *ServerConfig, langCtx *LanguageContext) error {
	// Apply build system specific settings
	if langCtx.BuildSystem != "" {
		if err := g.applyBuildSystemSettings(serverConfig, langCtx.BuildSystem, langCtx.Language); err != nil {
			return fmt.Errorf("failed to apply build system settings: %w", err)
		}
	}

	// Apply package manager specific settings
	if langCtx.PackageManager != "" {
		if err := g.applyPackageManagerSettings(serverConfig, langCtx.PackageManager, langCtx.Language); err != nil {
			return fmt.Errorf("failed to apply package manager settings: %w", err)
		}
	}

	// Apply complexity-based optimizations
	if langCtx.Complexity != nil {
		if err := g.applyComplexityOptimizations(serverConfig, langCtx.Complexity); err != nil {
			return fmt.Errorf("failed to apply complexity optimizations: %w", err)
		}
	}

	return nil
}

func (g *ConfigGenerator) applyBuildSystemSettings(serverConfig *ServerConfig, buildSystem, language string) error {
	switch language {
	case "go":
		if buildSystem == "bazel" {
			if gopls, ok := serverConfig.Settings["gopls"].(map[string]interface{}); ok {
				gopls["buildFlags"] = []string{"-tags=bazel"}
			}
		}
	case "java":
		switch buildSystem {
		case "gradle":
			if java, ok := serverConfig.Settings["java"].(map[string]interface{}); ok {
				if importSettings, ok := java["import"].(map[string]interface{}); ok {
					if gradle, ok := importSettings["gradle"].(map[string]interface{}); ok {
						gradle["enabled"] = true
						gradle["wrapper"] = map[string]bool{"enabled": true}
					}
				}
			}
		case "maven":
			if java, ok := serverConfig.Settings["java"].(map[string]interface{}); ok {
				if importSettings, ok := java["import"].(map[string]interface{}); ok {
					if maven, ok := importSettings["maven"].(map[string]interface{}); ok {
						maven["enabled"] = true
					}
				}
			}
		}
	case "rust":
		if buildSystem == "cargo" {
			if rust, ok := serverConfig.Settings["rust-analyzer"].(map[string]interface{}); ok {
				if cargo, ok := rust["cargo"].(map[string]interface{}); ok {
					cargo["loadOutDirsFromCheck"] = true
					cargo["runBuildScripts"] = true
				}
			}
		}
	}
	return nil
}

func (g *ConfigGenerator) applyPackageManagerSettings(serverConfig *ServerConfig, packageManager, language string) error {
	switch language {
	case "python":
		if packageManager == "poetry" {
			if pylsp, ok := serverConfig.Settings["pylsp"].(map[string]interface{}); ok {
				if plugins, ok := pylsp["plugins"].(map[string]interface{}); ok {
					plugins["poetry"] = map[string]bool{"enabled": true}
				}
			}
		}
	case "typescript", "javascript":
		if packageManager == "yarn" || packageManager == "pnpm" {
			if ts, ok := serverConfig.Settings["typescript"].(map[string]interface{}); ok {
				if preferences, ok := ts["preferences"].(map[string]interface{}); ok {
					preferences["includePackageJsonAutoImports"] = "auto"
				}
			}
		}
	}
	return nil
}

func (g *ConfigGenerator) applyComplexityOptimizations(serverConfig *ServerConfig, complexity *LanguageComplexity) error {
	if complexity.LinesOfCode > 100000 {
		// Large codebase optimizations
		serverConfig.Settings = g.applyLargeCodebaseOptimizations(serverConfig.Settings, serverConfig.Languages[0])
	}

	if complexity.DependencyCount > 100 {
		// High dependency count optimizations
		serverConfig.Settings = g.applyHighDependencyOptimizations(serverConfig.Settings, serverConfig.Languages[0])
	}

	return nil
}

func (g *ConfigGenerator) applyLargeCodebaseOptimizations(settings map[string]interface{}, language string) map[string]interface{} {
	switch language {
	case "go":
		if gopls, ok := settings["gopls"].(map[string]interface{}); ok {
			gopls["memoryMode"] = "DegradeClosed"
			gopls["symbolMatcher"] = "FastFuzzy"
		}
	case "typescript":
		if ts, ok := settings["typescript"].(map[string]interface{}); ok {
			ts["disableAutomaticTypeAcquisition"] = true
			if preferences, ok := ts["preferences"].(map[string]interface{}); ok {
				preferences["includeCompletionsForModuleExports"] = false
			}
		}
	}
	return settings
}

func (g *ConfigGenerator) applyHighDependencyOptimizations(settings map[string]interface{}, language string) map[string]interface{} {
	switch language {
	case "java":
		if java, ok := settings["java"].(map[string]interface{}); ok {
			if maven, ok := java["maven"].(map[string]interface{}); ok {
				maven["downloadSources"] = false
				maven["downloadJavadoc"] = false
			}
		}
	case "python":
		if pylsp, ok := settings["pylsp"].(map[string]interface{}); ok {
			if plugins, ok := pylsp["plugins"].(map[string]interface{}); ok {
				if rope, ok := plugins["rope_completion"].(map[string]interface{}); ok {
					rope["eager"] = false
				}
			}
		}
	}
	return settings
}

func (g *ConfigGenerator) applyFrameworkEnhancements(config *MultiLanguageConfig, frameworks []*Framework) error {
	for _, framework := range frameworks {
		// Find the server config for this framework's language
		var serverConfig *ServerConfig
		for _, sc := range config.ServerConfigs {
			for _, lang := range sc.Languages {
				if lang == framework.Language {
					serverConfig = sc
					break
				}
			}
			if serverConfig != nil {
				break
			}
		}

		if serverConfig == nil {
			continue
		}

		// Apply template-based framework enhancements
		if template, exists := g.templates[framework.Language]; exists {
			if enhancements, exists := template.FrameworkEnhancements[framework.Name]; exists {
				if err := g.mergeFrameworkEnhancements(serverConfig, enhancements); err != nil {
					return fmt.Errorf("failed to merge framework enhancements for %s-%s: %w", framework.Language, framework.Name, err)
				}
			}
		}

		// Apply custom framework enhancer if available
		if enhancer, exists := g.frameworkEnhancers[framework.Name]; exists {
			if err := enhancer.EnhanceConfig(serverConfig, framework); err != nil {
				return fmt.Errorf("failed to apply framework enhancer for %s: %w", framework.Name, err)
			}
		}
	}

	config.Metadata["enhanced_frameworks"] = len(frameworks)
	return nil
}

func (g *ConfigGenerator) mergeFrameworkEnhancements(serverConfig *ServerConfig, enhancements map[string]interface{}) error {
	for key, value := range enhancements {
		if existing, exists := serverConfig.Settings[key]; exists {
			if existingMap, ok := existing.(map[string]interface{}); ok {
				if valueMap, ok := value.(map[string]interface{}); ok {
					merged := deepMergeMap(existingMap, valueMap)
					serverConfig.Settings[key] = merged
				} else {
					serverConfig.Settings[key] = value
				}
			} else {
				serverConfig.Settings[key] = value
			}
		} else {
			serverConfig.Settings[key] = value
		}
	}
	return nil
}

func (g *ConfigGenerator) applyMonorepoStrategy(config *MultiLanguageConfig, layout *MonorepoLayout) error {
	strategy, exists := g.monorepoStrategies[layout.Strategy]
	if !exists {
		return fmt.Errorf("unknown monorepo strategy: %s", layout.Strategy)
	}

	return strategy.OptimizeForLayout(config, layout)
}

func (g *ConfigGenerator) OptimizeForProjectType(config *MultiLanguageConfig, projectType string) error {
	switch projectType {
	case ProjectTypeMonorepo:
		return g.optimizeForMonorepo(config)
	case ProjectTypeMicroservices:
		return g.optimizeForMicroservices(config)
	case ProjectTypeFrontendBackend:
		return g.optimizeForFrontendBackend(config)
	case ProjectTypePolyglot:
		return g.optimizeForPolyglot(config)
	default:
		return g.optimizeForGeneral(config)
	}
}

func (g *ConfigGenerator) optimizeForMonorepo(config *MultiLanguageConfig) error {
	config.WorkspaceConfig.CrossLanguageReferences = true
	config.WorkspaceConfig.IndexingStrategy = IndexingStrategySmart
	config.OptimizedFor = PerformanceProfileDevelopment

	// Enable shared settings for better cross-language integration
	config.WorkspaceConfig.SharedSettings["enable_cross_language_navigation"] = true
	config.WorkspaceConfig.SharedSettings["shared_symbol_index"] = true

	return nil
}

func (g *ConfigGenerator) optimizeForMicroservices(config *MultiLanguageConfig) error {
	config.WorkspaceConfig.CrossLanguageReferences = false
	config.WorkspaceConfig.IndexingStrategy = IndexingStrategyIncremental

	// Optimize for isolated services
	for _, serverConfig := range config.ServerConfigs {
		serverConfig.ServerType = ServerTypeSingle
		if serverConfig.MaxConcurrentRequests == 0 {
			serverConfig.MaxConcurrentRequests = 25 // Lower for microservices
		}
	}

	return nil
}

func (g *ConfigGenerator) optimizeForFrontendBackend(config *MultiLanguageConfig) error {
	config.WorkspaceConfig.CrossLanguageReferences = true

	// Separate frontend and backend optimizations
	for _, serverConfig := range config.ServerConfigs {
		language := serverConfig.Languages[0]
		if g.isFrontendLanguage(language) {
			serverConfig.Priority += 2 // Higher priority for frontend
		} else if g.isBackendLanguage(language) {
			serverConfig.Weight += 0.5 // Higher weight for backend
		}
	}

	return nil
}

func (g *ConfigGenerator) optimizeForPolyglot(config *MultiLanguageConfig) error {
	config.WorkspaceConfig.CrossLanguageReferences = true
	config.WorkspaceConfig.IndexingStrategy = IndexingStrategyFull

	// Enable maximum compatibility features
	config.WorkspaceConfig.SharedSettings["polyglot_mode"] = true
	config.WorkspaceConfig.SharedSettings["cross_language_completion"] = true

	return nil
}

func (g *ConfigGenerator) optimizeForGeneral(config *MultiLanguageConfig) error {
	// Apply balanced optimizations
	config.OptimizedFor = PerformanceProfileDevelopment
	config.WorkspaceConfig.IndexingStrategy = IndexingStrategyIncremental

	return nil
}

func (g *ConfigGenerator) isFrontendLanguage(language string) bool {
	frontendLanguages := map[string]bool{
		"typescript": true,
		"javascript": true,
		"html":       true,
		"css":        true,
		"scss":       true,
		"vue":        true,
		"svelte":     true,
	}
	return frontendLanguages[language]
}

func (g *ConfigGenerator) isBackendLanguage(language string) bool {
	backendLanguages := map[string]bool{
		"go":     true,
		"python": true,
		"java":   true,
		"rust":   true,
		"cpp":    true,
		"csharp": true,
		"php":    true,
	}
	return backendLanguages[language]
}

func (g *ConfigGenerator) validateConstraints(constraints *ServerConstraints, langCtx *LanguageContext) bool {
	if constraints == nil {
		return true
	}

	// Check file count constraints
	if constraints.MinFileCount > 0 && langCtx.FileCount < constraints.MinFileCount {
		return false
	}
	if constraints.MaxFileCount > 0 && langCtx.FileCount > constraints.MaxFileCount {
		return false
	}

	// Check required markers
	if len(constraints.RequiredMarkers) > 0 {
		for _, required := range constraints.RequiredMarkers {
			found := false
			for _, marker := range langCtx.RootMarkers {
				if marker == required || strings.Contains(marker, required) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	// Check excluded markers
	for _, excluded := range constraints.ExcludedMarkers {
		for _, marker := range langCtx.RootMarkers {
			if marker == excluded || strings.Contains(marker, excluded) {
				return false
			}
		}
	}

	return true
}

func (g *ConfigGenerator) GetAvailableLanguages() []string {
	var languages []string
	for lang := range g.templates {
		languages = append(languages, lang)
	}
	sort.Strings(languages)
	return languages
}

func (g *ConfigGenerator) GetAvailableFrameworks() []string {
	var frameworks []string
	for framework := range g.frameworkEnhancers {
		frameworks = append(frameworks, framework)
	}
	sort.Strings(frameworks)
	return frameworks
}

func (g *ConfigGenerator) GetTemplate(language string) (*ServerConfigTemplate, bool) {
	template, exists := g.templates[language]
	return template, exists
}

func (g *ConfigGenerator) AddTemplate(language string, template *ServerConfigTemplate) {
	g.templates[language] = template
}

func (g *ConfigGenerator) AddFrameworkEnhancer(framework string, enhancer FrameworkEnhancer) {
	g.frameworkEnhancers[framework] = enhancer
}

// Utility functions

func deepCopyMap(original map[string]interface{}) map[string]interface{} {
	if original == nil {
		return nil
	}

	result := make(map[string]interface{})
	for key, value := range original {
		switch v := value.(type) {
		case map[string]interface{}:
			result[key] = deepCopyMap(v)
		case []interface{}:
			copySlice := make([]interface{}, len(v))
			for i, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					copySlice[i] = deepCopyMap(itemMap)
				} else {
					copySlice[i] = item
				}
			}
			result[key] = copySlice
		case []string:
			copySlice := make([]string, len(v))
			copy(copySlice, v)
			result[key] = copySlice
		default:
			result[key] = v
		}
	}
	return result
}

func deepCopyLanguageSettingsMap(original map[string]map[string]interface{}) map[string]map[string]interface{} {
	if original == nil {
		return nil
	}

	copy := make(map[string]map[string]interface{})
	for lang, settings := range original {
		copy[lang] = deepCopyMap(settings)
	}
	return copy
}

func deepCopyServerConstraints(original *ServerConstraints) *ServerConstraints {
	if original == nil {
		return nil
	}

	result := &ServerConstraints{
		MinFileCount: original.MinFileCount,
		MaxFileCount: original.MaxFileCount,
		MinVersion:   original.MinVersion,
	}

	if original.RequiredMarkers != nil {
		result.RequiredMarkers = make([]string, len(original.RequiredMarkers))
		copy(result.RequiredMarkers, original.RequiredMarkers)
	}

	if original.ExcludedMarkers != nil {
		result.ExcludedMarkers = make([]string, len(original.ExcludedMarkers))
		copy(result.ExcludedMarkers, original.ExcludedMarkers)
	}

	if original.ProjectTypes != nil {
		result.ProjectTypes = make([]string, len(original.ProjectTypes))
		copy(result.ProjectTypes, original.ProjectTypes)
	}

	return result
}

func deepMergeMap(dst, src map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy destination first
	for key, value := range dst {
		result[key] = value
	}

	// Merge source
	for key, srcValue := range src {
		if dstValue, exists := result[key]; exists {
			if dstMap, dstOk := dstValue.(map[string]interface{}); dstOk {
				if srcMap, srcOk := srcValue.(map[string]interface{}); srcOk {
					result[key] = deepMergeMap(dstMap, srcMap)
					continue
				}
			}
		}
		result[key] = srcValue
	}

	return result
}

// normalizeLanguageName maps project detection language names to LSP server template keys
func (g *ConfigGenerator) normalizeLanguageName(language string) string {
	languageMap := map[string]string{
		"nodejs":     "typescript", // Node.js projects use TypeScript language server
		"javascript": "typescript", // JavaScript projects also use TypeScript language server
		"typescript": "typescript", // TypeScript projects (no change)
		"js":         "typescript", // Common JS abbreviation
		"ts":         "typescript", // Common TS abbreviation
	}

	if normalized, exists := languageMap[language]; exists {
		return normalized
	}

	// Return original language if no mapping exists
	return language
}

// mapLanguageForRouting maps project detection language names to LSP server language names for request routing
func (g *ConfigGenerator) mapLanguageForRouting(language string) string {
	routingMap := map[string]string{
		"nodejs":     "typescript", // Node.js projects route to typescript
		"javascript": "typescript", // JavaScript projects route to typescript
		"typescript": "typescript", // TypeScript projects (no change)
		"js":         "typescript", // Common JS abbreviation
		"ts":         "typescript", // Common TS abbreviation
	}

	if mapped, exists := routingMap[language]; exists {
		return mapped
	}

	// Return original language if no mapping exists
	return language
}

func (g *ConfigGenerator) loadGlobalBypassDefaults() *GlobalBypassConfig {
	return &GlobalBypassConfig{}
}

func (g *ConfigGenerator) loadLanguageBypassTemplate(language string) *LanguageBypassConfig {
	switch language {
	case "go":
		return &LanguageBypassConfig{
			Language:   "go",
			Strategy:   "auto",
			Conditions: []string{"consecutive_failures", "circuit_breaker"},
			Timeout:    "30s",
			PerformanceThresholds: &LanguagePerformanceThresholds{
				MaxResponseTime: "5s",
				MaxMemoryUsage:  "512MB",
				MaxCPUUsage:     80.0,
			},
			Recovery: &LanguageRecoveryConfig{
				Enabled:           true,
				MaxAttempts:       3,
				Cooldown:          "30s",
				HealthCheckMethod: "ping",
			},
		}
	case "python":
		return &LanguageBypassConfig{
			Language:   "python",
			Strategy:   "auto",
			Conditions: []string{"consecutive_failures", "circuit_breaker"},
			Timeout:    "30s",
			PerformanceThresholds: &LanguagePerformanceThresholds{
				MaxResponseTime: "10s",
				MaxMemoryUsage:  "1GB",
				MaxCPUUsage:     85.0,
			},
			Recovery: &LanguageRecoveryConfig{
				Enabled:           true,
				MaxAttempts:       3,
				Cooldown:          "30s",
				HealthCheckMethod: "ping",
			},
		}
	case "typescript", "javascript":
		return &LanguageBypassConfig{
			Language:   language,
			Strategy:   "auto",
			Conditions: []string{"consecutive_failures", "circuit_breaker"},
			Timeout:    "20s",
			PerformanceThresholds: &LanguagePerformanceThresholds{
				MaxResponseTime: "8s",
				MaxMemoryUsage:  "800MB",
				MaxCPUUsage:     75.0,
			},
			Recovery: &LanguageRecoveryConfig{
				Enabled:           true,
				MaxAttempts:       3,
				Cooldown:          "30s",
				HealthCheckMethod: "ping",
			},
		}
	case "java":
		return &LanguageBypassConfig{
			Language:   "java",
			Strategy:   "manual", // Java servers are slower to start, require manual intervention
			Conditions: []string{"consecutive_failures"},
			Timeout:    "60s",
			PerformanceThresholds: &LanguagePerformanceThresholds{
				MaxResponseTime: "15s",
				MaxMemoryUsage:  "2GB",
				MaxCPUUsage:     90.0,
			},
			Recovery: &LanguageRecoveryConfig{
				Enabled:           true,
				MaxAttempts:       5,
				Cooldown:          "60s",
				HealthCheckMethod: "workspace_health",
			},
		}
	default:
		return &LanguageBypassConfig{
			Language:   language,
			Strategy:   "auto",
			Conditions: []string{"consecutive_failures", "circuit_breaker"},
			Timeout:    "30s",
			PerformanceThresholds: &LanguagePerformanceThresholds{
				MaxResponseTime: "10s",
				MaxMemoryUsage:  "1GB",
				MaxCPUUsage:     80.0,
			},
			Recovery: &LanguageRecoveryConfig{
				Enabled:           true,
				MaxAttempts:       3,
				Cooldown:          "30s",
				HealthCheckMethod: "ping",
			},
		}
	}
}

// applyBypassConfiguration applies bypass configuration to the server config
func (g *ConfigGenerator) applyBypassConfiguration(serverConfig *ServerConfig, langCtx *LanguageContext) error {
	// Get language-specific bypass template
	normalizedLanguage := g.normalizeLanguageName(langCtx.Language)
	bypassTemplate, exists := g.bypassTemplates[normalizedLanguage]
	if !exists {
		// Use default bypass template if specific language template doesn't exist
		bypassTemplate = g.loadLanguageBypassTemplate("default")
	}

	// Create bypass configuration based on template
	bypassConfig := &ServerBypassConfig{
		Enabled:          true,
		BypassStrategy:   bypassTemplate.Strategy,
		BypassConditions: make([]string, len(bypassTemplate.Conditions)),
		RecoveryAttempts: bypassTemplate.Recovery.MaxAttempts,
		CooldownPeriod:   bypassTemplate.Recovery.Cooldown,
		Timeouts: &BypassTimeouts{
			Request: bypassTemplate.Timeout,
		},
		FailureThresholds: &BypassFailureThresholds{
			ConsecutiveFailures: 3,  // Default value
			ErrorRatePercent:    25, // Default value
		},
	}

	// Copy conditions
	copy(bypassConfig.BypassConditions, bypassTemplate.Conditions)

	// Apply performance thresholds if available
	if bypassTemplate.PerformanceThresholds != nil {
		if bypassTemplate.PerformanceThresholds.MaxResponseTime != "" {
			bypassConfig.Timeouts.Request = bypassTemplate.PerformanceThresholds.MaxResponseTime
		}
		// Convert memory usage to MB if needed (simple parsing)
		if bypassTemplate.PerformanceThresholds.MaxMemoryUsage != "" {
			if bypassTemplate.PerformanceThresholds.MaxMemoryUsage == "512MB" {
				bypassConfig.FailureThresholds.MemoryUsageMB = 512
			} else if bypassTemplate.PerformanceThresholds.MaxMemoryUsage == "1GB" {
				bypassConfig.FailureThresholds.MemoryUsageMB = 1024
			} else if bypassTemplate.PerformanceThresholds.MaxMemoryUsage == "2GB" {
				bypassConfig.FailureThresholds.MemoryUsageMB = 2048
			} else if bypassTemplate.PerformanceThresholds.MaxMemoryUsage == "800MB" {
				bypassConfig.FailureThresholds.MemoryUsageMB = 800
			} else {
				bypassConfig.FailureThresholds.MemoryUsageMB = 1024 // Default
			}
		}
		bypassConfig.FailureThresholds.CPUUsagePercent = bypassTemplate.PerformanceThresholds.MaxCPUUsage
	}

	// Apply project-specific optimizations
	if err := g.applyProjectSpecificBypassOptimizations(bypassConfig, langCtx); err != nil {
		return fmt.Errorf("failed to apply project-specific bypass optimizations: %w", err)
	}

	// Set the bypass configuration on the server config
	serverConfig.BypassConfig = bypassConfig

	return nil
}

// applyProjectSpecificBypassOptimizations applies project-specific bypass optimizations
func (g *ConfigGenerator) applyProjectSpecificBypassOptimizations(bypassConfig *ServerBypassConfig, langCtx *LanguageContext) error {
	// Apply complexity-based optimizations
	if langCtx.Complexity != nil {
		if langCtx.Complexity.LinesOfCode > 100000 {
			// Large codebase - be more conservative with bypass
			bypassConfig.FailureThresholds.ConsecutiveFailures = 5
			bypassConfig.RecoveryAttempts = 2
			bypassConfig.CooldownPeriod = "5m"
		}

		if langCtx.Complexity.DependencyCount > 100 {
			// High dependency count - longer timeouts
			if bypassConfig.Timeouts != nil {
				bypassConfig.Timeouts.Startup = "30s"
				bypassConfig.Timeouts.Request = "20s"
			}
		}
	}

	// Apply build system specific optimizations
	if langCtx.BuildSystem != "" {
		switch langCtx.BuildSystem {
		case "gradle", "maven":
			// Java build systems - longer startup timeouts
			if bypassConfig.Timeouts != nil {
				bypassConfig.Timeouts.Startup = "45s"
			}
		case "cargo":
			// Rust build system - moderate timeouts
			if bypassConfig.Timeouts != nil {
				bypassConfig.Timeouts.Startup = "20s"
			}
		}
	}

	// Apply framework-specific optimizations
	for _, framework := range langCtx.Frameworks {
		switch framework {
		case "spring-boot":
			// Spring Boot takes longer to start
			if bypassConfig.Timeouts != nil {
				bypassConfig.Timeouts.Startup = "60s"
			}
		case "django":
			// Django projects may have complex database migrations
			if bypassConfig.Timeouts != nil {
				bypassConfig.Timeouts.Startup = "30s"
			}
		case "react", "angular", "vue":
			// Frontend frameworks - faster recovery
			bypassConfig.RecoveryAttempts = 5
			bypassConfig.CooldownPeriod = "1m"
		}
	}

	return nil
}

// generateBypassConfigForLanguage generates bypass configuration for a specific language and project type
func (g *ConfigGenerator) generateBypassConfigForLanguage(language, projectType string) *ServerBypassConfig {
	// Get language-specific bypass template
	normalizedLanguage := g.normalizeLanguageName(language)
	bypassTemplate, exists := g.bypassTemplates[normalizedLanguage]
	if !exists {
		bypassTemplate = g.loadLanguageBypassTemplate("default")
	}

	// Create base bypass configuration
	baseConfig := &ServerBypassConfig{
		Enabled:          true,
		BypassStrategy:   bypassTemplate.Strategy,
		BypassConditions: make([]string, len(bypassTemplate.Conditions)),
		RecoveryAttempts: bypassTemplate.Recovery.MaxAttempts,
		CooldownPeriod:   bypassTemplate.Recovery.Cooldown,
		Timeouts: &BypassTimeouts{
			Request: bypassTemplate.Timeout,
		},
		FailureThresholds: &BypassFailureThresholds{
			ConsecutiveFailures: 3,
			ErrorRatePercent:    25,
		},
	}

	copy(baseConfig.BypassConditions, bypassTemplate.Conditions)

	// Apply project type specific optimizations
	switch projectType {
	case ProjectTypeMonorepo:
		baseConfig.FailureThresholds.ConsecutiveFailures = 5 // More tolerant for complex monorepos
		baseConfig.CooldownPeriod = "3m"
	case ProjectTypeMicroservices:
		baseConfig.RecoveryAttempts = 2 // Fail fast for microservices
		baseConfig.CooldownPeriod = "30s"
	case ProjectTypeFrontendBackend:
		if g.isFrontendLanguage(language) {
			baseConfig.RecoveryAttempts = 5 // More attempts for frontend
		} else if g.isBackendLanguage(language) {
			baseConfig.FailureThresholds.ConsecutiveFailures = 4 // Backend services should be stable
		}
	}

	return baseConfig
}
