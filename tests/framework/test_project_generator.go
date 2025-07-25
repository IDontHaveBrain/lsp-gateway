package framework

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ProjectComplexity defines the complexity of generated projects
type ProjectComplexity string

const (
	ComplexitySimple     ProjectComplexity = "simple"
	ComplexityMedium     ProjectComplexity = "medium"
	ComplexityComplex    ProjectComplexity = "complex"
	ComplexityLarge      ProjectComplexity = "large"
	ComplexityEnterprise ProjectComplexity = "enterprise"
)

// ProjectSize defines the size of generated projects
type ProjectSize string

const (
	SizeSmall  ProjectSize = "small"
	SizeMedium ProjectSize = "medium"
	SizeLarge  ProjectSize = "large"
	SizeXLarge ProjectSize = "xlarge"
)

// ProjectGenerationConfig configures project generation
type ProjectGenerationConfig struct {
	Type         ProjectType
	Languages    []string
	Complexity   ProjectComplexity
	Size         ProjectSize
	BuildSystem  bool
	TestFiles    bool
	Dependencies bool
	Documentation bool
	CI           bool
	Docker       bool
	
	// Customization options
	FileCount         int
	DirectoryDepth    int
	CrossReferences   bool
	RealisticContent  bool
	VersionControl    bool
	
	// Enhanced options for complex scenarios
	MicroserviceCount    int
	DatabaseIntegration  bool
	KubernetesConfig     bool
	APIGateway          bool
	ServiceMesh         bool
	MonitoringStack     bool
	SharedLibraries     bool
	MultiModuleStructure bool
}

// ProjectTemplate represents a template for project generation
type ProjectTemplate struct {
	Name         string
	Type         ProjectType
	Languages    []string
	Structure    map[string]string
	BuildFiles   []string
	Dependencies map[string][]string
	Frameworks   map[string]string
}

// TestProjectGenerator generates realistic test projects
type TestProjectGenerator struct {
	TempDir      string
	Templates    map[string]*ProjectTemplate
	Generated    []*TestProject
	
	// Call tracking
	LoadTemplatesCalls     []context.Context
	GenerateProjectCalls   []ProjectGenerationConfig
	
	// Function fields for behavior customization
	LoadTemplatesFunc      func(ctx context.Context) error
	GenerateProjectFunc    func(config *ProjectGenerationConfig) (*TestProject, error)
	
	mu sync.RWMutex
}

// NewTestProjectGenerator creates a new test project generator
func NewTestProjectGenerator(tempDir string) *TestProjectGenerator {
	return &TestProjectGenerator{
		TempDir:                tempDir,
		Templates:              make(map[string]*ProjectTemplate),
		Generated:              make([]*TestProject, 0),
		LoadTemplatesCalls:     make([]context.Context, 0),
		GenerateProjectCalls:   make([]ProjectGenerationConfig, 0),
	}
}

// LoadTemplates loads project templates for generation
func (g *TestProjectGenerator) LoadTemplates(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	g.LoadTemplatesCalls = append(g.LoadTemplatesCalls, ctx)
	
	if g.LoadTemplatesFunc != nil {
		return g.LoadTemplatesFunc(ctx)
	}
	
	// Load predefined templates
	g.loadGoTemplates()
	g.loadPythonTemplates()
	g.loadTypeScriptTemplates()
	g.loadJavaTemplates()
	g.loadRustTemplates()
	g.loadMultiLanguageTemplates()
	g.loadMonorepoTemplates()
	g.loadMicroservicesTemplates()
	
	// Load enhanced complex scenario templates
	g.loadMonorepoWebAppTemplates()
	g.loadMicroservicesSuiteTemplates()
	g.loadPythonWithGoExtensionsTemplates()
	g.loadFullStackTypeScriptTemplates()
	g.loadJavaSpringWithReactTemplates()
	g.loadEnterprisePlatformTemplates()
	
	return nil
}

// GenerateProject generates a test project based on configuration
func (g *TestProjectGenerator) GenerateProject(config *ProjectGenerationConfig) (*TestProject, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	g.GenerateProjectCalls = append(g.GenerateProjectCalls, *config)
	
	if g.GenerateProjectFunc != nil {
		return g.GenerateProjectFunc(config)
	}
	
	// Create project directory
	projectName := g.generateProjectName(config)
	projectPath := filepath.Join(g.TempDir, "projects", projectName)
	
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create project directory: %w", err)
	}
	
	// Create project structure
	project := &TestProject{
		ID:           fmt.Sprintf("proj-%d", time.Now().UnixNano()),
		Name:         projectName,
		RootPath:     projectPath,
		ProjectType:  config.Type,
		Languages:    config.Languages,
		Structure:    make(map[string]string),
		BuildFiles:   make([]string, 0),
		TestFiles:    make([]string, 0),
		Dependencies: make(map[string][]string),
		CreatedAt:    time.Now(),
	}
	
	// Generate project content based on configuration
	if err := g.generateProjectContent(project, config); err != nil {
		return nil, fmt.Errorf("failed to generate project content: %w", err)
	}
	
	// Write files to disk
	if err := g.writeProjectFiles(project); err != nil {
		return nil, fmt.Errorf("failed to write project files: %w", err)
	}
	
	// Calculate project size
	project.Size = g.calculateProjectSize(project)
	
	// Track generated project
	g.Generated = append(g.Generated, project)
	
	return project, nil
}

// generateProjectName generates a unique project name
func (g *TestProjectGenerator) generateProjectName(config *ProjectGenerationConfig) string {
	timestamp := time.Now().Format("20060102-150405")
	langStr := strings.Join(config.Languages, "-")
	return fmt.Sprintf("%s-%s-%s-%s", config.Type, langStr, config.Complexity, timestamp)
}

// generateProjectContent generates the project content structure
func (g *TestProjectGenerator) generateProjectContent(project *TestProject, config *ProjectGenerationConfig) error {
	// Generate language-specific content
	for _, language := range config.Languages {
		if err := g.generateLanguageContent(project, language, config); err != nil {
			return fmt.Errorf("failed to generate %s content: %w", language, err)
		}
	}
	
	// Generate project type specific structure
	if err := g.generateProjectTypeStructure(project, config); err != nil {
		return fmt.Errorf("failed to generate project type structure: %w", err)
	}
	
	// Generate build system files
	if config.BuildSystem {
		g.generateBuildFiles(project, config)
	}
	
	// Generate test files
	if config.TestFiles {
		g.generateTestFiles(project, config)
	}
	
	// Generate dependencies
	if config.Dependencies {
		g.generateDependencies(project, config)
	}
	
	// Generate documentation
	if config.Documentation {
		g.generateDocumentation(project, config)
	}
	
	// Generate CI configuration
	if config.CI {
		g.generateCIConfiguration(project, config)
	}
	
	// Generate Docker configuration
	if config.Docker {
		g.generateDockerConfiguration(project, config)
	}
	
	return nil
}

// generateLanguageContent generates content for a specific language
func (g *TestProjectGenerator) generateLanguageContent(project *TestProject, language string, config *ProjectGenerationConfig) error {
	switch language {
	case "go":
		return g.generateGoContent(project, config)
	case "python":
		return g.generatePythonContent(project, config)
	case "javascript":
		return g.generateJavaScriptContent(project, config)
	case "typescript":
		return g.generateTypeScriptContent(project, config)
	case "java":
		return g.generateJavaContent(project, config)
	case "rust":
		return g.generateRustContent(project, config)
	case "cpp":
		return g.generateCppContent(project, config)
	case "csharp":
		return g.generateCSharpContent(project, config)
	default:
		return g.generateGenericContent(project, language, config)
	}
}

// generateGoContent generates Go-specific content
func (g *TestProjectGenerator) generateGoContent(project *TestProject, config *ProjectGenerationConfig) error {
	// Generate go.mod
	project.Structure["go.mod"] = g.generateGoMod(project.Name)
	project.BuildFiles = append(project.BuildFiles, "go.mod")
	
	// Generate main.go
	project.Structure["main.go"] = g.generateGoMain()
	
	// Generate package structure based on complexity
	switch config.Complexity {
	case ComplexitySimple:
		project.Structure["utils.go"] = g.generateGoUtils()
	case ComplexityMedium:
		project.Structure["cmd/main.go"] = g.generateGoMain()
		project.Structure["pkg/utils/utils.go"] = g.generateGoUtils()
		project.Structure["internal/service.go"] = g.generateGoService()
	case ComplexityComplex, ComplexityLarge:
		project.Structure["cmd/main.go"] = g.generateGoMain()
		project.Structure["cmd/server/main.go"] = g.generateGoServerMain()
		project.Structure["pkg/utils/utils.go"] = g.generateGoUtils()
		project.Structure["pkg/models/user.go"] = g.generateGoUserModel()
		project.Structure["internal/service/user.go"] = g.generateGoUserService()
		project.Structure["internal/handlers/user.go"] = g.generateGoUserHandler()
		project.Structure["internal/database/connection.go"] = g.generateGoDatabase()
		project.Structure["api/routes.go"] = g.generateGoRoutes()
	}
	
	// Add dependencies
	project.Dependencies["go"] = []string{"github.com/gin-gonic/gin", "gorm.io/gorm"}
	
	return nil
}

// generatePythonContent generates Python-specific content
func (g *TestProjectGenerator) generatePythonContent(project *TestProject, config *ProjectGenerationConfig) error {
	// Generate setup.py or pyproject.toml
	if config.Complexity == ComplexitySimple {
		project.Structure["setup.py"] = g.generatePythonSetup(project.Name)
		project.BuildFiles = append(project.BuildFiles, "setup.py")
	} else {
		project.Structure["pyproject.toml"] = g.generatePythonPyproject(project.Name)
		project.BuildFiles = append(project.BuildFiles, "pyproject.toml")
	}
	
	// Generate requirements.txt
	project.Structure["requirements.txt"] = g.generatePythonRequirements()
	
	// Generate main.py
	project.Structure["main.py"] = g.generatePythonMain()
	
	// Generate package structure based on complexity
	switch config.Complexity {
	case ComplexitySimple:
		project.Structure["utils.py"] = g.generatePythonUtils()
	case ComplexityMedium:
		project.Structure["src/__init__.py"] = ""
		project.Structure["src/main.py"] = g.generatePythonMain()
		project.Structure["src/utils.py"] = g.generatePythonUtils()
		project.Structure["src/models.py"] = g.generatePythonModels()
	case ComplexityComplex, ComplexityLarge:
		project.Structure["src/__init__.py"] = ""
		project.Structure["src/main.py"] = g.generatePythonMain()
		project.Structure["src/utils/__init__.py"] = ""
		project.Structure["src/utils/helpers.py"] = g.generatePythonUtils()
		project.Structure["src/models/__init__.py"] = ""
		project.Structure["src/models/user.py"] = g.generatePythonUserModel()
		project.Structure["src/services/__init__.py"] = ""
		project.Structure["src/services/user_service.py"] = g.generatePythonUserService()
		project.Structure["src/api/__init__.py"] = ""
		project.Structure["src/api/routes.py"] = g.generatePythonRoutes()
		project.Structure["src/database/__init__.py"] = ""
		project.Structure["src/database/connection.py"] = g.generatePythonDatabase()
	}
	
	// Add dependencies
	project.Dependencies["python"] = []string{"flask", "requests", "sqlalchemy"}
	
	return nil
}

// generateJavaScriptContent generates JavaScript-specific content
func (g *TestProjectGenerator) generateJavaScriptContent(project *TestProject, config *ProjectGenerationConfig) error {
	// Generate package.json
	project.Structure["package.json"] = g.generatePackageJson(project.Name, "javascript")
	project.BuildFiles = append(project.BuildFiles, "package.json")
	
	// Generate main JavaScript file
	project.Structure["index.js"] = "console.log('Hello, World!');"
	
	return nil
}

// generateTypeScriptContent generates TypeScript-specific content
func (g *TestProjectGenerator) generateTypeScriptContent(project *TestProject, config *ProjectGenerationConfig) error {
	// Generate package.json
	project.Structure["package.json"] = g.generatePackageJson(project.Name, "typescript")
	project.BuildFiles = append(project.BuildFiles, "package.json")
	
	// Generate tsconfig.json
	project.Structure["tsconfig.json"] = g.generateTsConfig()
	project.BuildFiles = append(project.BuildFiles, "tsconfig.json")
	
	// Generate content based on complexity
	switch config.Complexity {
	case ComplexitySimple:
		project.Structure["index.ts"] = g.generateTypeScriptIndex()
		project.Structure["utils.ts"] = g.generateTypeScriptUtils()
	case ComplexityMedium:
		project.Structure["src/index.ts"] = g.generateTypeScriptIndex()
		project.Structure["src/utils.ts"] = g.generateTypeScriptUtils()
		project.Structure["src/types.ts"] = g.generateTypeScriptTypes()
		project.Structure["src/services/api.ts"] = g.generateTypeScriptAPIService()
	case ComplexityComplex, ComplexityLarge:
		project.Structure["src/index.ts"] = g.generateTypeScriptIndex()
		project.Structure["src/types/user.ts"] = g.generateTypeScriptUserTypes()
		project.Structure["src/services/userService.ts"] = g.generateTypeScriptUserService()
		project.Structure["src/utils/helpers.ts"] = g.generateTypeScriptUtils()
		project.Structure["src/components/UserComponent.tsx"] = g.generateTypeScriptComponent()
		project.Structure["src/api/routes.ts"] = g.generateTypeScriptRoutes()
		project.Structure["src/database/connection.ts"] = g.generateTypeScriptDatabase()
	}
	
	// Add dependencies
	project.Dependencies["typescript"] = []string{"react", "express", "axios"}
	
	return nil
}

// generateJavaContent generates Java-specific content
func (g *TestProjectGenerator) generateJavaContent(project *TestProject, config *ProjectGenerationConfig) error {
	// Generate pom.xml
	project.Structure["pom.xml"] = g.generatePomXml(project.Name)
	project.BuildFiles = append(project.BuildFiles, "pom.xml")
	
	// Generate content based on complexity
	packagePath := "src/main/java/com/example"
	
	switch config.Complexity {
	case ComplexitySimple:
		project.Structure[packagePath+"/Main.java"] = g.generateJavaMain()
		project.Structure[packagePath+"/Utils.java"] = g.generateJavaUtils()
	case ComplexityMedium:
		project.Structure[packagePath+"/Main.java"] = g.generateJavaMain()
		project.Structure[packagePath+"/service/UserService.java"] = g.generateJavaUserService()
		project.Structure[packagePath+"/model/User.java"] = g.generateJavaUserModel()
		project.Structure[packagePath+"/util/Helper.java"] = g.generateJavaUtils()
	case ComplexityComplex, ComplexityLarge:
		project.Structure[packagePath+"/Application.java"] = g.generateJavaSpringBootMain()
		project.Structure[packagePath+"/controller/UserController.java"] = g.generateJavaController()
		project.Structure[packagePath+"/service/UserService.java"] = g.generateJavaUserService()
		project.Structure[packagePath+"/model/User.java"] = g.generateJavaUserModel()
		project.Structure[packagePath+"/repository/UserRepository.java"] = g.generateJavaRepository()
		project.Structure[packagePath+"/config/DatabaseConfig.java"] = g.generateJavaConfig()
		project.Structure["src/main/resources/application.properties"] = g.generateJavaApplicationProperties()
	}
	
	// Add dependencies
	project.Dependencies["java"] = []string{"spring-boot-starter-web", "spring-boot-starter-data-jpa"}
	
	return nil
}

// generateRustContent generates Rust-specific content
func (g *TestProjectGenerator) generateRustContent(project *TestProject, config *ProjectGenerationConfig) error {
	// Generate Cargo.toml
	project.Structure["Cargo.toml"] = g.generateCargoToml(project.Name)
	project.BuildFiles = append(project.BuildFiles, "Cargo.toml")
	
	// Generate content based on complexity
	switch config.Complexity {
	case ComplexitySimple:
		project.Structure["src/main.rs"] = g.generateRustMain()
		project.Structure["src/lib.rs"] = g.generateRustLib()
	case ComplexityMedium:
		project.Structure["src/main.rs"] = g.generateRustMain()
		project.Structure["src/lib.rs"] = g.generateRustLib()
		project.Structure["src/utils.rs"] = g.generateRustUtils()
		project.Structure["src/models.rs"] = g.generateRustModels()
	case ComplexityComplex, ComplexityLarge:
		project.Structure["src/main.rs"] = g.generateRustMain()
		project.Structure["src/lib.rs"] = g.generateRustLib()
		project.Structure["src/models/mod.rs"] = g.generateRustModels()
		project.Structure["src/models/user.rs"] = g.generateRustUserModel()
		project.Structure["src/services/mod.rs"] = g.generateRustServicesModule()
		project.Structure["src/services/user_service.rs"] = g.generateRustUserService()
		project.Structure["src/utils/mod.rs"] = g.generateRustUtilsModule()
		project.Structure["src/utils/helpers.rs"] = g.generateRustUtils()
	}
	
	// Add dependencies
	project.Dependencies["rust"] = []string{"serde", "tokio", "axum"}
	
	return nil
}

// generateCppContent generates C++ content
func (g *TestProjectGenerator) generateCppContent(project *TestProject, config *ProjectGenerationConfig) error {
	// Generate CMakeLists.txt
	project.Structure["CMakeLists.txt"] = g.generateCMakeLists(project.Name)
	project.BuildFiles = append(project.BuildFiles, "CMakeLists.txt")
	
	switch config.Complexity {
	case ComplexitySimple:
		project.Structure["main.cpp"] = g.generateCppMain()
		project.Structure["utils.h"] = g.generateCppUtilsHeader()
		project.Structure["utils.cpp"] = g.generateCppUtils()
	default:
		project.Structure["src/main.cpp"] = g.generateCppMain()
		project.Structure["include/utils.h"] = g.generateCppUtilsHeader()
		project.Structure["src/utils.cpp"] = g.generateCppUtils()
		project.Structure["include/user.h"] = g.generateCppUserHeader()
		project.Structure["src/user.cpp"] = g.generateCppUser()
	}
	
	return nil
}

// generateCSharpContent generates C# content
func (g *TestProjectGenerator) generateCSharpContent(project *TestProject, config *ProjectGenerationConfig) error {
	// Generate .csproj file
	project.Structure[project.Name+".csproj"] = g.generateCsProj(project.Name)
	project.BuildFiles = append(project.BuildFiles, project.Name+".csproj")
	
	switch config.Complexity {
	case ComplexitySimple:
		project.Structure["Program.cs"] = g.generateCSharpMain()
		project.Structure["Utils.cs"] = g.generateCSharpUtils()
	default:
		project.Structure["Program.cs"] = g.generateCSharpMain()
		project.Structure["Models/User.cs"] = g.generateCSharpUserModel()
		project.Structure["Services/UserService.cs"] = g.generateCSharpUserService()
		project.Structure["Controllers/UserController.cs"] = g.generateCSharpController()
		project.Structure["Utils/Helper.cs"] = g.generateCSharpUtils()
	}
	
	return nil
}

// generateGenericContent generates content for unsupported languages
func (g *TestProjectGenerator) generateGenericContent(project *TestProject, language string, config *ProjectGenerationConfig) error {
	// Create basic files for the language
	ext := g.getExtensionForLanguage(language)
	project.Structure[fmt.Sprintf("main%s", ext)] = fmt.Sprintf("// Main file for %s\nprint('Hello from %s!')", language, language)
	project.Structure[fmt.Sprintf("utils%s", ext)] = fmt.Sprintf("// Utility file for %s\nfunction utility() { return 'utility'; }", language)
	
	return nil
}

// getExtensionForLanguage returns the file extension for a language
func (g *TestProjectGenerator) getExtensionForLanguage(language string) string {
	extensions := map[string]string{
		"python":     ".py",
		"javascript": ".js",
		"typescript": ".ts",
		"java":       ".java",
		"go":         ".go",
		"rust":       ".rs",
		"cpp":        ".cpp",
		"csharp":     ".cs",
		"php":        ".php",
		"ruby":       ".rb",
	}
	
	if ext, exists := extensions[language]; exists {
		return ext
	}
	
	return ".txt"
}

// writeProjectFiles writes all project files to disk
func (g *TestProjectGenerator) writeProjectFiles(project *TestProject) error {
	for relPath, content := range project.Structure {
		fullPath := filepath.Join(project.RootPath, relPath)
		
		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", relPath, err)
		}
		
		// Write file
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", relPath, err)
		}
	}
	
	return nil
}

// calculateProjectSize calculates the total size of the project
func (g *TestProjectGenerator) calculateProjectSize(project *TestProject) int64 {
	var totalSize int64
	
	for _, content := range project.Structure {
		totalSize += int64(len(content))
	}
	
	return totalSize
}

// Reset resets the generator state
func (g *TestProjectGenerator) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	// Clean up generated projects
	for _, project := range g.Generated {
		os.RemoveAll(project.RootPath)
	}
	
	g.Generated = make([]*TestProject, 0)
	g.LoadTemplatesCalls = make([]context.Context, 0)
	g.GenerateProjectCalls = make([]ProjectGenerationConfig, 0)
	
	// Reset function fields
	g.LoadTemplatesFunc = nil
	g.GenerateProjectFunc = nil
}

// Template loading methods

// loadGoTemplates loads Go project templates
func (g *TestProjectGenerator) loadGoTemplates() {
	g.Templates["go-simple"] = &ProjectTemplate{
		Name:      "go-simple",
		Type:      ProjectTypeSingleLanguage,
		Languages: []string{"go"},
		Structure: map[string]string{
			"go.mod":  "module {{.ProjectName}}\n\ngo 1.19",
			"main.go": "package main\n\nfunc main() {\n\tprintln(\"Hello, World!\")\n}",
		},
		BuildFiles:   []string{"go.mod"},
		Dependencies: map[string][]string{"go": {"github.com/gin-gonic/gin"}},
	}
}

// loadPythonTemplates loads Python project templates
func (g *TestProjectGenerator) loadPythonTemplates() {
	g.Templates["python-simple"] = &ProjectTemplate{
		Name:      "python-simple",
		Type:      ProjectTypeSingleLanguage,
		Languages: []string{"python"},
		Structure: map[string]string{
			"setup.py":         "from setuptools import setup\n\nsetup(name='{{.ProjectName}}')",
			"main.py":          "print('Hello, World!')",
			"requirements.txt": "flask==2.0.1\nrequests==2.28.0",
		},
		BuildFiles:   []string{"setup.py", "requirements.txt"},
		Dependencies: map[string][]string{"python": {"flask", "requests"}},
	}
}

// loadTypeScriptTemplates loads TypeScript project templates
func (g *TestProjectGenerator) loadTypeScriptTemplates() {
	g.Templates["typescript-simple"] = &ProjectTemplate{
		Name:      "typescript-simple",
		Type:      ProjectTypeSingleLanguage,
		Languages: []string{"typescript"},
		Structure: map[string]string{
			"package.json":   "{\"name\": \"{{.ProjectName}}\", \"dependencies\": {\"typescript\": \"^4.9.0\"}}",
			"tsconfig.json":  "{\"compilerOptions\": {\"target\": \"ES2020\"}}",
			"src/index.ts":   "console.log('Hello, World!');",
		},
		BuildFiles:   []string{"package.json", "tsconfig.json"},
		Dependencies: map[string][]string{"typescript": {"react", "express"}},
	}
}

// loadJavaTemplates loads Java project templates
func (g *TestProjectGenerator) loadJavaTemplates() {
	g.Templates["java-simple"] = &ProjectTemplate{
		Name:      "java-simple",
		Type:      ProjectTypeSingleLanguage,
		Languages: []string{"java"},
		Structure: map[string]string{
			"pom.xml": "<?xml version=\"1.0\"?>\n<project><modelVersion>4.0.0</modelVersion><groupId>com.example</groupId><artifactId>{{.ProjectName}}</artifactId><version>1.0.0</version></project>",
			"src/main/java/Main.java": "public class Main {\n    public static void main(String[] args) {\n        System.out.println(\"Hello, World!\");\n    }\n}",
		},
		BuildFiles:   []string{"pom.xml"},
		Dependencies: map[string][]string{"java": {"spring-boot-starter-web"}},
	}
}

// loadRustTemplates loads Rust project templates
func (g *TestProjectGenerator) loadRustTemplates() {
	g.Templates["rust-simple"] = &ProjectTemplate{
		Name:      "rust-simple",
		Type:      ProjectTypeSingleLanguage,
		Languages: []string{"rust"},
		Structure: map[string]string{
			"Cargo.toml":   "[package]\nname = \"{{.ProjectName}}\"\nversion = \"0.1.0\"\nedition = \"2021\"",
			"src/main.rs":  "fn main() {\n    println!(\"Hello, world!\");\n}",
		},
		BuildFiles:   []string{"Cargo.toml"},
		Dependencies: map[string][]string{"rust": {"serde", "tokio"}},
	}
}

// loadMultiLanguageTemplates loads multi-language project templates
func (g *TestProjectGenerator) loadMultiLanguageTemplates() {
	g.Templates["multi-language"] = &ProjectTemplate{
		Name:      "multi-language",
		Type:      ProjectTypeMultiLanguage,
		Languages: []string{"go", "python", "typescript"},
		Structure: map[string]string{
			"backend/go.mod":         "module {{.ProjectName}}-backend\n\ngo 1.19",
			"backend/main.go":        "package main\n\nfunc main() {}",
			"scripts/setup.py":       "from setuptools import setup\n\nsetup(name='{{.ProjectName}}-scripts')",
			"scripts/deploy.py":      "print('Deploying')",
			"frontend/package.json":  "{\"name\": \"{{.ProjectName}}-frontend\"}",
			"frontend/src/index.ts":  "console.log('Frontend');",
		},
		BuildFiles: []string{"backend/go.mod", "scripts/setup.py", "frontend/package.json"},
	}
}

// loadMonorepoTemplates loads monorepo project templates
func (g *TestProjectGenerator) loadMonorepoTemplates() {
	g.Templates["monorepo"] = &ProjectTemplate{
		Name:      "monorepo",
		Type:      ProjectTypeMonorepo,
		Languages: []string{"go", "python", "typescript", "java", "rust"},
		Structure: map[string]string{
			"services/auth-service/go.mod":         "module auth-service\n\ngo 1.19",
			"services/auth-service/main.go":        "package main\n\nfunc main() {}",
			"services/user-service/setup.py":       "from setuptools import setup\n\nsetup(name='user-service')",
			"services/user-service/main.py":        "print('User service')",
			"frontend/package.json":                "{\"name\": \"frontend\"}",
			"frontend/src/App.tsx":                 "import React from 'react';\n\nconst App = () => <div>App</div>;",
			"services/notification/pom.xml":        "<?xml version=\"1.0\"?>\n<project><modelVersion>4.0.0</modelVersion></project>",
			"services/notification/Main.java":      "public class Main { public static void main(String[] args) {} }",
			"services/search/Cargo.toml":           "[package]\nname = \"search\"\nversion = \"0.1.0\"",
			"services/search/src/main.rs":          "fn main() { println!(\"Search service\"); }",
		},
		BuildFiles: []string{"services/auth-service/go.mod", "services/user-service/setup.py", "frontend/package.json", "services/notification/pom.xml", "services/search/Cargo.toml"},
	}
}

// loadMicroservicesTemplates loads microservices project templates
func (g *TestProjectGenerator) loadMicroservicesTemplates() {
	g.Templates["microservices"] = &ProjectTemplate{
		Name:      "microservices",
		Type:      ProjectTypeMicroservices,
		Languages: []string{"go", "python", "typescript", "java"},
		Structure: map[string]string{
			"auth-service/go.mod":           "module auth-service\n\ngo 1.19",
			"auth-service/main.go":          "package main\n\nfunc main() {}",
			"user-service/requirements.txt": "flask==2.0.1",
			"user-service/main.py":          "from flask import Flask\napp = Flask(__name__)",
			"api-gateway/package.json":      "{\"name\": \"api-gateway\"}",
			"api-gateway/server.js":         "const express = require('express');",
			"notification-service/pom.xml":  "<?xml version=\"1.0\"?>\n<project><modelVersion>4.0.0</modelVersion></project>",
			"docker-compose.yml":            "version: '3.8'\nservices:\n  auth:\n    build: ./auth-service",
		},
		BuildFiles: []string{"auth-service/go.mod", "user-service/requirements.txt", "api-gateway/package.json", "notification-service/pom.xml", "docker-compose.yml"},
	}
}

// ==================== ENHANCED COMPLEX SCENARIO TEMPLATE LOADERS ====================

// loadMonorepoWebAppTemplates loads MonorepoWebApp templates
func (g *TestProjectGenerator) loadMonorepoWebAppTemplates() {
	g.Templates["monorepo-webapp"] = &ProjectTemplate{
		Name:      "MonorepoWebApp",
		Type:      ProjectTypeMonorepoWebApp,
		Languages: []string{"typescript", "go", "python"},
		Structure: map[string]string{
			"package.json":     "{\"name\": \"monorepo-webapp\", \"workspaces\": [\"packages/*\", \"services/*\"]}",
			"lerna.json":       "{\"version\": \"0.0.0\", \"npmClient\": \"yarn\", \"useWorkspaces\": true}",
			"workspace.json":   "{\"version\": 2, \"projects\": {}}",
		},
		BuildFiles:   []string{"package.json", "lerna.json", "workspace.json"},
		Dependencies: map[string][]string{
			"typescript": {"react", "@types/react", "vite"},
			"go":         {"github.com/gin-gonic/gin", "gorm.io/gorm"},
			"python":     {"fastapi", "pandas", "numpy"},
		},
		Frameworks: map[string]string{
			"frontend": "React + Vite + TypeScript",
			"backend":  "Go + Gin + GORM",
			"data":     "Python + FastAPI + Pandas",
		},
	}
}

// loadMicroservicesSuiteTemplates loads MicroservicesSuite templates
func (g *TestProjectGenerator) loadMicroservicesSuiteTemplates() {
	g.Templates["microservices-suite"] = &ProjectTemplate{
		Name:      "MicroservicesSuite",
		Type:      "microservices-suite",
		Languages: []string{"typescript", "go", "java", "python", "rust", "csharp"},
		Structure: map[string]string{
			"docker-compose.yml": "version: '3.8'\nservices:\n  api-gateway:\n    build: ./services/api-gateway",
			"Makefile":           "all:\n\t@echo 'Building microservices suite'",
		},
		BuildFiles: []string{"docker-compose.yml", "Makefile"},
		Dependencies: map[string][]string{
			"typescript": {"express", "@types/express", "cors"},
			"go":         {"github.com/gin-gonic/gin", "google.golang.org/grpc"},
			"java":       {"spring-boot-starter-web", "spring-kafka"},
			"python":     {"fastapi", "celery", "redis"},
			"rust":       {"tokio", "axum", "serde"},
			"csharp":     {"Microsoft.AspNetCore", "Microsoft.EntityFrameworkCore"},
		},
		Frameworks: map[string]string{
			"api-gateway":     "Node.js + Express + TypeScript",
			"user-service":    "Go + gRPC + PostgreSQL",
			"order-service":   "Java Spring Boot + Kafka",
			"payment-service": "Python FastAPI + Redis",
			"notification":    "Rust + Axum + Queue",
			"analytics":       "C# .NET + Entity Framework",
		},
	}
}

// loadPythonWithGoExtensionsTemplates loads PythonWithGoExtensions templates
func (g *TestProjectGenerator) loadPythonWithGoExtensionsTemplates() {
	g.Templates["python-with-go-extensions"] = &ProjectTemplate{
		Name:      "PythonWithGoExtensions",
		Type:      "python-with-go-extensions",
		Languages: []string{"python", "go"},
		Structure: map[string]string{
			"pyproject.toml": "[project]\nname = \"python-go-extensions\"\nversion = \"1.0.0\"",
			"setup.py":       "from setuptools import setup\nsetup(name='python-go-extensions')",
			"Makefile":       "build:\n\t@echo 'Building Python with Go extensions'",
		},
		BuildFiles: []string{"pyproject.toml", "setup.py", "Makefile"},
		Dependencies: map[string][]string{
			"python": {"fastapi", "pydantic", "numpy", "cffi"},
			"go":     {"github.com/gin-gonic/gin"},
		},
		Frameworks: map[string]string{
			"main":       "Python FastAPI",
			"extensions": "Go with C bindings",
		},
	}
}

// loadFullStackTypeScriptTemplates loads FullStackTypeScript templates
func (g *TestProjectGenerator) loadFullStackTypeScriptTemplates() {
	g.Templates["fullstack-typescript"] = &ProjectTemplate{
		Name:      "FullStackTypeScript",
		Type:      "fullstack-typescript",
		Languages: []string{"typescript"},
		Structure: map[string]string{
			"package.json":   "{\"name\": \"fullstack-typescript\", \"workspaces\": [\"apps/*\", \"packages/*\"]}",
			"turbo.json":     "{\"pipeline\": {\"build\": {\"outputs\": [\"dist/**\"]}}}",
			"tsconfig.json": "{\"compilerOptions\": {\"target\": \"ES2020\"}}",
		},
		BuildFiles: []string{"package.json", "turbo.json", "tsconfig.json"},
		Dependencies: map[string][]string{
			"typescript": {"react", "express", "@types/node", "@types/react", "vite", "prisma"},
		},
		Frameworks: map[string]string{
			"frontend": "React + Vite + TypeScript",
			"backend":  "Node.js + Express + TypeScript",
			"shared":   "TypeScript + Prisma",
		},
	}
}

// loadJavaSpringWithReactTemplates loads JavaSpringWithReact templates
func (g *TestProjectGenerator) loadJavaSpringWithReactTemplates() {
	g.Templates["java-spring-with-react"] = &ProjectTemplate{
		Name:      "JavaSpringWithReact",
		Type:      "java-spring-with-react",
		Languages: []string{"java", "typescript"},
		Structure: map[string]string{
			"pom.xml": "<?xml version=\"1.0\"?>\n<project><modelVersion>4.0.0</modelVersion><packaging>pom</packaging></project>",
			"mvnw":    "#!/bin/bash\necho 'Maven wrapper'",
		},
		BuildFiles: []string{"pom.xml", "mvnw"},
		Dependencies: map[string][]string{
			"java":       {"spring-boot-starter-web", "spring-boot-starter-data-jpa", "spring-security-core"},
			"typescript": {"react", "@types/react", "vite", "axios"},
		},
		Frameworks: map[string]string{
			"backend":  "Java Spring Boot + JPA + Security",
			"frontend": "React + TypeScript + Vite",
			"shared":   "Maven Multi-module",
		},
	}
}

// loadEnterprisePlatformTemplates loads EnterprisePlatform templates
func (g *TestProjectGenerator) loadEnterprisePlatformTemplates() {
	g.Templates["enterprise-platform"] = &ProjectTemplate{
		Name:      "EnterprisePlatform",
		Type:      "enterprise-platform",
		Languages: []string{"go", "java", "python", "rust", "cpp", "csharp", "typescript", "scala"},
		Structure: map[string]string{
			"platform.yaml":               "version: 1\nplatform:\n  name: enterprise-platform",
			"Makefile":                    "all:\n\t@echo 'Building enterprise platform'",
			"docker-compose.enterprise.yml": "version: '3.8'\nservices:\n  core:\n    build: ./platform/core",
		},
		BuildFiles: []string{"platform.yaml", "Makefile", "docker-compose.enterprise.yml"},
		Dependencies: map[string][]string{
			"go":         {"github.com/gin-gonic/gin", "k8s.io/client-go", "go.uber.org/zap"},
			"java":       {"spring-boot-starter-web", "spring-cloud-starter", "micrometer-core"},
			"python":     {"fastapi", "kafka-python", "prometheus-client", "pandas"},
			"rust":       {"tokio", "axum", "tracing", "serde"},
			"cpp":        {"boost", "grpc++", "protobuf"},
			"csharp":     {"Microsoft.AspNetCore", "Microsoft.Extensions.Hosting", "Serilog"},
			"typescript": {"express", "@prometheus-io/client", "winston"},
			"scala":      {"akka-actor", "akka-stream", "kafka-streams-scala"},
		},
		Frameworks: map[string]string{
			"core":           "Go + Kubernetes + gRPC",
			"auth":           "Java Spring Boot + JWT",
			"data-pipeline":  "Python + Kafka + Pandas",
			"analytics":      "Rust + TimeSeries + gRPC",
			"ml-service":     "C++ + TensorFlow + gRPC",
			"config":         "C# .NET + Configuration",
			"monitoring":     "TypeScript + Prometheus",
			"event-streaming": "Scala + Akka + Kafka",
		},
	}
}

// Content generation methods for different languages

// generateGoMod generates go.mod content
func (g *TestProjectGenerator) generateGoMod(projectName string) string {
	return fmt.Sprintf("module %s\n\ngo 1.19\n\nrequire (\n\tgithub.com/gin-gonic/gin v1.9.1\n)", projectName)
}

// generateGoMain generates main.go content
func (g *TestProjectGenerator) generateGoMain() string {
	return "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}"
}

// generateGoUtils generates Go utility functions
func (g *TestProjectGenerator) generateGoUtils() string {
	return "package main\n\nimport \"fmt\"\n\nfunc Utility() {\n\tfmt.Println(\"Utility function\")\n}"
}

// generateGoService generates Go service code
func (g *TestProjectGenerator) generateGoService() string {
	return "package internal\n\ntype Service struct {\n\tname string\n}\n\nfunc NewService(name string) *Service {\n\treturn &Service{name: name}\n}"
}

// generateGoServerMain generates Go server main
func (g *TestProjectGenerator) generateGoServerMain() string {
	return "package main\n\nimport (\n\t\"log\"\n\t\"net/http\"\n)\n\nfunc main() {\n\thttp.HandleFunc(\"/\", func(w http.ResponseWriter, r *http.Request) {\n\t\tw.Write([]byte(\"Server running\"))\n\t})\n\tlog.Fatal(http.ListenAndServe(\":8080\", nil))\n}"
}

// generateGoUserModel generates Go user model
func (g *TestProjectGenerator) generateGoUserModel() string {
	return "package models\n\ntype User struct {\n\tID   int    `json:\"id\"`\n\tName string `json:\"name\"`\n}"
}

// generateGoUserService generates Go user service
func (g *TestProjectGenerator) generateGoUserService() string {
	return "package service\n\nimport \"../models\"\n\ntype UserService struct {}\n\nfunc (s *UserService) GetUser(id int) *models.User {\n\treturn &models.User{ID: id, Name: \"Test User\"}\n}"
}

// generateGoUserHandler generates Go user handler
func (g *TestProjectGenerator) generateGoUserHandler() string {
	return "package handlers\n\nimport (\n\t\"encoding/json\"\n\t\"net/http\"\n)\n\nfunc GetUser(w http.ResponseWriter, r *http.Request) {\n\tjson.NewEncoder(w).Encode(map[string]string{\"user\": \"test\"})\n}"
}

// generateGoDatabase generates Go database connection
func (g *TestProjectGenerator) generateGoDatabase() string {
	return "package database\n\nimport \"database/sql\"\n\nvar DB *sql.DB\n\nfunc Connect() error {\n\treturn nil\n}"
}

// generateGoRoutes generates Go API routes
func (g *TestProjectGenerator) generateGoRoutes() string {
	return "package api\n\nimport \"net/http\"\n\nfunc SetupRoutes() *http.ServeMux {\n\tmux := http.NewServeMux()\n\tmux.HandleFunc(\"/users\", handleUsers)\n\treturn mux\n}\n\nfunc handleUsers(w http.ResponseWriter, r *http.Request) {}"
}

// Python content generation methods

// generatePythonSetup generates setup.py content
func (g *TestProjectGenerator) generatePythonSetup(projectName string) string {
	return fmt.Sprintf("from setuptools import setup\n\nsetup(\n    name='%s',\n    version='1.0.0',\n    packages=['src'],\n)", projectName)
}

// generatePythonPyproject generates pyproject.toml content
func (g *TestProjectGenerator) generatePythonPyproject(projectName string) string {
	return fmt.Sprintf("[build-system]\nrequires = [\"setuptools\", \"wheel\"]\n\n[project]\nname = \"%s\"\nversion = \"1.0.0\"", projectName)
}

// generatePythonDataProcessorPyproject generates pyproject.toml content for data processor service
func (g *TestProjectGenerator) generatePythonDataProcessorPyproject(projectName string) string {
	return fmt.Sprintf(`[build-system]
requires = ["setuptools", "wheel"]

[project]
name = "%s-data-processor"
version = "1.0.0"
dependencies = [
    "pandas>=1.5.0",
    "numpy>=1.24.0",
    "sqlalchemy>=1.4.0",
    "psycopg2-binary>=2.9.0"
]`, projectName)
}

// generatePythonRequirements generates requirements.txt content
func (g *TestProjectGenerator) generatePythonRequirements() string {
	return "flask==2.0.1\nrequests==2.28.0\nsqlalchemy==1.4.0"
}

// generatePythonMain generates main.py content
func (g *TestProjectGenerator) generatePythonMain() string {
	return "#!/usr/bin/env python3\n\ndef main():\n    print('Hello, World!')\n\nif __name__ == '__main__':\n    main()"
}

// generatePythonUtils generates Python utility functions
func (g *TestProjectGenerator) generatePythonUtils() string {
	return "def utility_function():\n    \"\"\"A utility function\"\"\"\n    return 'utility'"
}

// generatePythonModels generates Python models
func (g *TestProjectGenerator) generatePythonModels() string {
	return "class BaseModel:\n    def __init__(self):\n        pass\n\nclass User(BaseModel):\n    def __init__(self, name):\n        super().__init__()\n        self.name = name"
}

// generatePythonUserModel generates Python user model
func (g *TestProjectGenerator) generatePythonUserModel() string {
	return "from dataclasses import dataclass\n\n@dataclass\nclass User:\n    id: int\n    name: str"
}

// generatePythonUserService generates Python user service
func (g *TestProjectGenerator) generatePythonUserService() string {
	return "from .user import User\n\nclass UserService:\n    def get_user(self, user_id: int) -> User:\n        return User(id=user_id, name='Test User')"
}

// generatePythonRoutes generates Python API routes
func (g *TestProjectGenerator) generatePythonRoutes() string {
	return "from flask import Flask, jsonify\n\napp = Flask(__name__)\n\n@app.route('/users')\ndef get_users():\n    return jsonify({'users': []})"
}

// generatePythonDatabase generates Python database connection
func (g *TestProjectGenerator) generatePythonDatabase() string {
	return "import sqlite3\n\nclass Database:\n    def __init__(self):\n        self.connection = None\n    \n    def connect(self):\n        self.connection = sqlite3.connect('database.db')"
}

// TypeScript content generation methods

// generatePackageJson generates package.json content
func (g *TestProjectGenerator) generatePackageJson(projectName, projectType string) string {
	deps := map[string]string{
		"typescript": "\"typescript\": \"^4.9.0\", \"@types/node\": \"^18.0.0\"",
		"javascript": "\"express\": \"^4.18.0\", \"lodash\": \"^4.17.0\"",
	}
	
	dep := deps[projectType]
	if dep == "" {
		dep = "\"typescript\": \"^4.9.0\""
	}
	
	return fmt.Sprintf("{\n  \"name\": \"%s\",\n  \"version\": \"1.0.0\",\n  \"dependencies\": {\n    %s\n  }\n}", projectName, dep)
}

// generateTsConfig generates tsconfig.json content
func (g *TestProjectGenerator) generateTsConfig() string {
	return "{\n  \"compilerOptions\": {\n    \"target\": \"ES2020\",\n    \"module\": \"commonjs\",\n    \"outDir\": \"./dist\",\n    \"rootDir\": \"./src\"\n  }\n}"
}

// generateTypeScriptIndex generates TypeScript index file
func (g *TestProjectGenerator) generateTypeScriptIndex() string {
	return "console.log('Hello, TypeScript!');\n\nclass Application {\n  start() {\n    console.log('Application started');\n  }\n}\n\nconst app = new Application();\napp.start();"
}

// generateTypeScriptUtils generates TypeScript utilities
func (g *TestProjectGenerator) generateTypeScriptUtils() string {
	return "export function utility(message: string): string {\n  return `Utility: ${message}`;\n}\n\nexport class Helper {\n  static format(text: string): string {\n    return text.toUpperCase();\n  }\n}"
}

// generateTypeScriptTypes generates TypeScript type definitions
func (g *TestProjectGenerator) generateTypeScriptTypes() string {
	return "export interface User {\n  id: number;\n  name: string;\n  email?: string;\n}\n\nexport type ApiResponse<T> = {\n  data: T;\n  status: 'success' | 'error';\n};"
}

// generateTypeScriptAPIService generates TypeScript API service
func (g *TestProjectGenerator) generateTypeScriptAPIService() string {
	return "import { User, ApiResponse } from '../types';\n\nexport class ApiService {\n  async getUser(id: number): Promise<ApiResponse<User>> {\n    // Mock implementation\n    return {\n      data: { id, name: 'Test User' },\n      status: 'success'\n    };\n  }\n}"
}

// generateTypeScriptUserTypes generates TypeScript user types
func (g *TestProjectGenerator) generateTypeScriptUserTypes() string {
	return "export interface User {\n  id: number;\n  name: string;\n  email: string;\n  createdAt: Date;\n}\n\nexport interface CreateUserRequest {\n  name: string;\n  email: string;\n}"
}

// generateTypeScriptUserService generates TypeScript user service
func (g *TestProjectGenerator) generateTypeScriptUserService() string {
	return "import { User, CreateUserRequest } from '../types/user';\n\nexport class UserService {\n  async createUser(request: CreateUserRequest): Promise<User> {\n    return {\n      id: 1,\n      name: request.name,\n      email: request.email,\n      createdAt: new Date()\n    };\n  }\n}"
}

// generateTypeScriptComponent generates TypeScript React component
func (g *TestProjectGenerator) generateTypeScriptComponent() string {
	return "import React from 'react';\nimport { User } from '../types/user';\n\ninterface Props {\n  user: User;\n}\n\nexport const UserComponent: React.FC<Props> = ({ user }) => {\n  return (\n    <div>\n      <h2>{user.name}</h2>\n      <p>{user.email}</p>\n    </div>\n  );\n};"
}

// generateTypeScriptRoutes generates TypeScript API routes
func (g *TestProjectGenerator) generateTypeScriptRoutes() string {
	return "import express from 'express';\nimport { UserService } from '../services/userService';\n\nconst router = express.Router();\nconst userService = new UserService();\n\nrouter.get('/users/:id', async (req, res) => {\n  // Implementation\n});\n\nexport default router;"
}

// generateTypeScriptDatabase generates TypeScript database connection
func (g *TestProjectGenerator) generateTypeScriptDatabase() string {
	return "import { Pool } from 'pg';\n\nexport class Database {\n  private pool: Pool;\n\n  constructor() {\n    this.pool = new Pool({\n      connectionString: process.env.DATABASE_URL\n    });\n  }\n\n  async query(text: string, params?: any[]) {\n    return this.pool.query(text, params);\n  }\n}"
}

// Java content generation methods

// generatePomXml generates pom.xml content
func (g *TestProjectGenerator) generatePomXml(projectName string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>%s</artifactId>
    <version>1.0.0</version>
    <properties>
        <maven.compiler.source>11</maven.compiler.source>
        <maven.compiler.target>11</maven.compiler.target>
    </properties>
    <dependencies>
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-web</artifactId>
            <version>2.7.0</version>
        </dependency>
    </dependencies>
</project>`, projectName)
}

// generateJavaMain generates Java main class
func (g *TestProjectGenerator) generateJavaMain() string {
	return "public class Main {\n    public static void main(String[] args) {\n        System.out.println(\"Hello, World!\");\n    }\n}"
}

// generateJavaUtils generates Java utility class
func (g *TestProjectGenerator) generateJavaUtils() string {
	return "public class Utils {\n    public static String utility(String message) {\n        return \"Utility: \" + message;\n    }\n}"
}

// generateJavaSpringBootMain generates Java Spring Boot main class
func (g *TestProjectGenerator) generateJavaSpringBootMain() string {
	return "import org.springframework.boot.SpringApplication;\nimport org.springframework.boot.autoconfigure.SpringBootApplication;\n\n@SpringBootApplication\npublic class Application {\n    public static void main(String[] args) {\n        SpringApplication.run(Application.class, args);\n    }\n}"
}

// generateJavaController generates Java controller
func (g *TestProjectGenerator) generateJavaController() string {
	return "import org.springframework.web.bind.annotation.*;\n\n@RestController\n@RequestMapping(\"/api/users\")\npublic class UserController {\n    \n    @GetMapping(\"/{id}\")\n    public User getUser(@PathVariable Long id) {\n        return new User(id, \"Test User\");\n    }\n}"
}

// generateJavaUserService generates Java user service
func (g *TestProjectGenerator) generateJavaUserService() string {
	return "import org.springframework.stereotype.Service;\n\n@Service\npublic class UserService {\n    \n    public User findById(Long id) {\n        return new User(id, \"Test User\");\n    }\n}"
}

// generateJavaUserModel generates Java user model
func (g *TestProjectGenerator) generateJavaUserModel() string {
	return "public class User {\n    private Long id;\n    private String name;\n    \n    public User(Long id, String name) {\n        this.id = id;\n        this.name = name;\n    }\n    \n    // Getters and setters\n    public Long getId() { return id; }\n    public void setId(Long id) { this.id = id; }\n    public String getName() { return name; }\n    public void setName(String name) { this.name = name; }\n}"
}

// generateJavaRepository generates Java repository
func (g *TestProjectGenerator) generateJavaRepository() string {
	return "import org.springframework.data.jpa.repository.JpaRepository;\nimport org.springframework.stereotype.Repository;\n\n@Repository\npublic interface UserRepository extends JpaRepository<User, Long> {\n    User findByName(String name);\n}"
}

// generateJavaConfig generates Java configuration
func (g *TestProjectGenerator) generateJavaConfig() string {
	return "import org.springframework.context.annotation.Configuration;\nimport org.springframework.context.annotation.Bean;\nimport javax.sql.DataSource;\n\n@Configuration\npublic class DatabaseConfig {\n    \n    @Bean\n    public DataSource dataSource() {\n        // Configuration\n        return null;\n    }\n}"
}

// generateJavaApplicationProperties generates application.properties
func (g *TestProjectGenerator) generateJavaApplicationProperties() string {
	return "server.port=8080\nspring.datasource.url=jdbc:h2:mem:testdb\nspring.jpa.hibernate.ddl-auto=create-drop"
}

// Rust content generation methods

// generateCargoToml generates Cargo.toml content
func (g *TestProjectGenerator) generateCargoToml(projectName string) string {
	return fmt.Sprintf("[package]\nname = \"%s\"\nversion = \"0.1.0\"\nedition = \"2021\"\n\n[dependencies]\nserde = { version = \"1.0\", features = [\"derive\"] }\ntokio = { version = \"1.0\", features = [\"full\"] }", projectName)
}

// generateRustMain generates Rust main.rs
func (g *TestProjectGenerator) generateRustMain() string {
	return "fn main() {\n    println!(\"Hello, world!\");\n}"
}

// generateRustLib generates Rust lib.rs
func (g *TestProjectGenerator) generateRustLib() string {
	return "pub mod utils;\npub mod models;\n\npub fn hello() {\n    println!(\"Hello from lib!\");\n}"
}

// generateRustUtils generates Rust utilities
func (g *TestProjectGenerator) generateRustUtils() string {
	return "pub fn utility_function(message: &str) -> String {\n    format!(\"Utility: {}\", message)\n}"
}

// generateRustModels generates Rust models
func (g *TestProjectGenerator) generateRustModels() string {
	return "use serde::{Deserialize, Serialize};\n\n#[derive(Debug, Serialize, Deserialize)]\npub struct User {\n    pub id: u64,\n    pub name: String,\n}"
}

// generateRustUserModel generates Rust user model
func (g *TestProjectGenerator) generateRustUserModel() string {
	return "use serde::{Deserialize, Serialize};\n\n#[derive(Debug, Clone, Serialize, Deserialize)]\npub struct User {\n    pub id: u64,\n    pub name: String,\n    pub email: Option<String>,\n}"
}

// generateRustServicesModule generates Rust services module
func (g *TestProjectGenerator) generateRustServicesModule() string {
	return "pub mod user_service;\n\npub use user_service::UserService;"
}

// generateRustUserService generates Rust user service
func (g *TestProjectGenerator) generateRustUserService() string {
	return "use crate::models::User;\n\npub struct UserService;\n\nimpl UserService {\n    pub fn new() -> Self {\n        UserService\n    }\n    \n    pub async fn get_user(&self, id: u64) -> Option<User> {\n        Some(User {\n            id,\n            name: \"Test User\".to_string(),\n            email: None,\n        })\n    }\n}"
}

// generateRustUtilsModule generates Rust utils module
func (g *TestProjectGenerator) generateRustUtilsModule() string {
	return "pub mod helpers;\n\npub use helpers::*;"
}

// C++ content generation methods

// generateCMakeLists generates CMakeLists.txt
func (g *TestProjectGenerator) generateCMakeLists(projectName string) string {
	return fmt.Sprintf("cmake_minimum_required(VERSION 3.10)\nproject(%s)\n\nset(CMAKE_CXX_STANDARD 17)\n\nadd_executable(%s main.cpp)", projectName, projectName)
}

// generateCppMain generates C++ main
func (g *TestProjectGenerator) generateCppMain() string {
	return "#include <iostream>\n\nint main() {\n    std::cout << \"Hello, World!\" << std::endl;\n    return 0;\n}"
}

// generateCppUtilsHeader generates C++ utils header
func (g *TestProjectGenerator) generateCppUtilsHeader() string {
	return "#pragma once\n#include <string>\n\nclass Utils {\npublic:\n    static std::string utility(const std::string& message);\n};"
}

// generateCppUtils generates C++ utils implementation
func (g *TestProjectGenerator) generateCppUtils() string {
	return "#include \"utils.h\"\n\nstd::string Utils::utility(const std::string& message) {\n    return \"Utility: \" + message;\n}"
}

// generateCppUserHeader generates C++ user header
func (g *TestProjectGenerator) generateCppUserHeader() string {
	return "#pragma once\n#include <string>\n\nclass User {\nprivate:\n    int id;\n    std::string name;\npublic:\n    User(int id, const std::string& name);\n    int getId() const;\n    std::string getName() const;\n};"
}

// generateCppUser generates C++ user implementation
func (g *TestProjectGenerator) generateCppUser() string {
	return "#include \"user.h\"\n\nUser::User(int id, const std::string& name) : id(id), name(name) {}\n\nint User::getId() const { return id; }\nstd::string User::getName() const { return name; }"
}

// C# content generation methods

// generateCsProj generates .csproj file
func (g *TestProjectGenerator) generateCsProj(projectName string) string {
	return fmt.Sprintf("<Project Sdk=\"Microsoft.NET.Sdk\">\n  <PropertyGroup>\n    <OutputType>Exe</OutputType>\n    <TargetFramework>net6.0</TargetFramework>\n  </PropertyGroup>\n</Project>")
}

// generateCSharpMain generates C# main
func (g *TestProjectGenerator) generateCSharpMain() string {
	return "using System;\n\nclass Program\n{\n    static void Main(string[] args)\n    {\n        Console.WriteLine(\"Hello, World!\");\n    }\n}"
}

// generateCSharpUtils generates C# utilities
func (g *TestProjectGenerator) generateCSharpUtils() string {
	return "using System;\n\npublic static class Utils\n{\n    public static string Utility(string message)\n    {\n        return $\"Utility: {message}\";\n    }\n}"
}

// generateCSharpUserModel generates C# user model
func (g *TestProjectGenerator) generateCSharpUserModel() string {
	return "public class User\n{\n    public int Id { get; set; }\n    public string Name { get; set; }\n    \n    public User(int id, string name)\n    {\n        Id = id;\n        Name = name;\n    }\n}"
}

// generateCSharpUserService generates C# user service
func (g *TestProjectGenerator) generateCSharpUserService() string {
	return "using Models;\n\npublic class UserService\n{\n    public User GetUser(int id)\n    {\n        return new User(id, \"Test User\");\n    }\n}"
}

// generateCSharpController generates C# controller
func (g *TestProjectGenerator) generateCSharpController() string {
	return "using Microsoft.AspNetCore.Mvc;\nusing Services;\n\n[ApiController]\n[Route(\"api/[controller]\")]\npublic class UserController : ControllerBase\n{\n    private readonly UserService _userService;\n    \n    public UserController(UserService userService)\n    {\n        _userService = userService;\n    }\n    \n    [HttpGet(\"{id}\")]\n    public IActionResult GetUser(int id)\n    {\n        var user = _userService.GetUser(id);\n        return Ok(user);\n    }\n}"
}

// Project type structure generation methods

// generateProjectTypeStructure generates structure based on project type
func (g *TestProjectGenerator) generateProjectTypeStructure(project *TestProject, config *ProjectGenerationConfig) error {
	switch config.Type {
	case ProjectTypeMonorepo:
		return g.generateMonorepoStructure(project, config)
	case ProjectTypeMicroservices:
		return g.generateMicroservicesStructure(project, config)
	case ProjectTypeFrontendBackend:
		return g.generateFrontendBackendStructure(project, config)
	case ProjectTypeWorkspace:
		return g.generateWorkspaceStructure(project, config)
	case ProjectTypePolyglot:
		return g.generatePolyglotStructure(project, config)
	case ProjectTypeMonorepoWebApp:
		return g.generateMonorepoWebAppStructure(project, config)
	case ProjectTypeMicroservicesSuite:
		return g.generateMicroservicesSuiteStructure(project, config)
	case ProjectTypePythonWithGoExtensions:
		return g.generatePythonWithGoExtensionsStructure(project, config)
	case "fullstack-typescript":
		return g.generateFullStackTypeScriptStructure(project, config)
	case "java-spring-with-react":
		return g.generateJavaSpringWithReactStructure(project, config)
	case "enterprise-platform":
		return g.generateEnterprisePlatformStructure(project, config)
	}
	return nil
}

// generateMonorepoStructure generates monorepo structure
func (g *TestProjectGenerator) generateMonorepoStructure(project *TestProject, config *ProjectGenerationConfig) error {
	// Add common monorepo files
	project.Structure["Makefile"] = "build-all:\n\t@echo 'Building monorepo'\n\ntest-all:\n\t@echo 'Testing monorepo'"
	project.Structure["docker-compose.yml"] = "version: '3.8'\nservices:\n  app:\n    build: ."
	project.Structure["README.md"] = "# Monorepo Project\n\nThis is a monorepo containing multiple services."
	project.BuildFiles = append(project.BuildFiles, "Makefile", "docker-compose.yml")
	return nil
}

// generateMicroservicesStructure generates microservices structure
func (g *TestProjectGenerator) generateMicroservicesStructure(project *TestProject, config *ProjectGenerationConfig) error {
	// Add microservices-specific files
	project.Structure["docker-compose.yml"] = "version: '3.8'\nservices:\n  service1:\n    build: ./service1\n  service2:\n    build: ./service2"
	project.Structure["kubernetes/deployment.yaml"] = "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: microservices"
	project.BuildFiles = append(project.BuildFiles, "docker-compose.yml")
	return nil
}

// generateFrontendBackendStructure generates frontend-backend structure
func (g *TestProjectGenerator) generateFrontendBackendStructure(project *TestProject, config *ProjectGenerationConfig) error {
	// Add frontend-backend separation
	project.Structure["frontend/README.md"] = "# Frontend\n\nFrontend application"
	project.Structure["backend/README.md"] = "# Backend\n\nBackend API"
	project.Structure["shared/types.json"] = "{\"User\": {\"id\": \"number\", \"name\": \"string\"}}"
	return nil
}

// generateWorkspaceStructure generates workspace structure
func (g *TestProjectGenerator) generateWorkspaceStructure(project *TestProject, config *ProjectGenerationConfig) error {
	// Add workspace files
	project.Structure["workspace.json"] = "{\"projects\": [], \"version\": 1}"
	project.Structure[".gitignore"] = "node_modules/\n*.log\ndist/"
	project.BuildFiles = append(project.BuildFiles, "workspace.json")
	return nil
}

// generatePolyglotStructure generates polyglot structure
func (g *TestProjectGenerator) generatePolyglotStructure(project *TestProject, config *ProjectGenerationConfig) error {
	// Add polyglot-specific structure
	project.Structure["scripts/build-all.sh"] = "#!/bin/bash\necho 'Building all languages'"
	project.Structure["docs/ARCHITECTURE.md"] = "# Architecture\n\nMulti-language architecture"
	return nil
}

// generateBuildFiles generates build system files
func (g *TestProjectGenerator) generateBuildFiles(project *TestProject, config *ProjectGenerationConfig) {
	// Already handled in language-specific generation
}

// generateTestFiles generates test files
func (g *TestProjectGenerator) generateTestFiles(project *TestProject, config *ProjectGenerationConfig) {
	for _, lang := range project.Languages {
		switch lang {
		case "go":
			project.Structure["main_test.go"] = "package main\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) {}"
			project.TestFiles = append(project.TestFiles, "main_test.go")
		case "python":
			project.Structure["test_main.py"] = "import unittest\n\nclass TestMain(unittest.TestCase):\n    def test_main(self):\n        pass"
			project.TestFiles = append(project.TestFiles, "test_main.py")
		case "typescript":
			project.Structure["src/__tests__/index.test.ts"] = "import { expect, test } from '@jest/globals';\n\ntest('sample test', () => {\n  expect(1).toBe(1);\n});"
			project.TestFiles = append(project.TestFiles, "src/__tests__/index.test.ts")
		}
	}
}

// generateDependencies generates dependency files
func (g *TestProjectGenerator) generateDependencies(project *TestProject, config *ProjectGenerationConfig) {
	// Dependencies are already handled in language-specific generation
}

// generateDocumentation generates documentation files
func (g *TestProjectGenerator) generateDocumentation(project *TestProject, config *ProjectGenerationConfig) {
	project.Structure["docs/README.md"] = "# Documentation\n\nProject documentation"
	project.Structure["docs/API.md"] = "# API Documentation\n\nAPI endpoints and usage"
}

// generateCIConfiguration generates CI/CD configuration
func (g *TestProjectGenerator) generateCIConfiguration(project *TestProject, config *ProjectGenerationConfig) {
	project.Structure[".github/workflows/ci.yml"] = `name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - name: Run tests
        run: make test`
}

// generateDockerConfiguration generates Docker configuration
func (g *TestProjectGenerator) generateDockerConfiguration(project *TestProject, config *ProjectGenerationConfig) {
	project.Structure["Dockerfile"] = "FROM alpine:latest\nRUN echo 'Docker container'\nCMD ['echo', 'Hello from Docker']"
	project.Structure[".dockerignore"] = "node_modules\n*.log\n.git"
}

// ==================== ENHANCED COMPLEX SCENARIO GENERATORS ====================

// generateMonorepoWebAppStructure generates React frontend + Go backend + Python data processing + TypeScript shared libs
func (g *TestProjectGenerator) generateMonorepoWebAppStructure(project *TestProject, config *ProjectGenerationConfig) error {
	// Workspace root configuration
	project.Structure["package.json"] = g.generateMonorepoRootPackageJson(project.Name)
	project.Structure["lerna.json"] = g.generateLernaConfig()
	project.Structure["workspace.json"] = g.generateNxWorkspaceConfig()
	project.Structure["Makefile"] = g.generateMonorepoMakefile()
	project.Structure["docker-compose.yml"] = g.generateMonorepoDockerCompose()
	
	// Frontend (React)
	project.Structure["packages/frontend/package.json"] = g.generateReactPackageJson("frontend")
	project.Structure["packages/frontend/tsconfig.json"] = g.generateReactTsConfig()
	project.Structure["packages/frontend/src/App.tsx"] = g.generateReactApp()
	project.Structure["packages/frontend/src/index.tsx"] = g.generateReactIndex()
	project.Structure["packages/frontend/src/components/UserList.tsx"] = g.generateReactUserListComponent()
	project.Structure["packages/frontend/src/services/api.ts"] = g.generateReactAPIService()
	project.Structure["packages/frontend/src/types/api.ts"] = g.generateReactAPITypes()
	project.Structure["packages/frontend/public/index.html"] = g.generateReactIndexHTML()
	project.Structure["packages/frontend/.env.example"] = g.generateReactEnvExample()
	
	// Backend (Go)
	project.Structure["services/backend/go.mod"] = g.generateGoBackendMod(project.Name)
	project.Structure["services/backend/main.go"] = g.generateGoBackendMain()
	project.Structure["services/backend/internal/handlers/user.go"] = g.generateGoUserAPIHandler()
	project.Structure["services/backend/internal/services/user.go"] = g.generateGoUserBusinessService()
	project.Structure["services/backend/internal/models/user.go"] = g.generateGoUserDomainModel()
	project.Structure["services/backend/internal/database/postgres.go"] = g.generateGoPostgresConnection()
	project.Structure["services/backend/pkg/middleware/cors.go"] = g.generateGoCORSMiddleware()
	project.Structure["services/backend/pkg/middleware/auth.go"] = g.generateGoAuthMiddleware()
	project.Structure["services/backend/config/config.go"] = g.generateGoConfig()
	project.Structure["services/backend/Dockerfile"] = g.generateGoDockerfile()
	
	// Data Processing (Python)
	project.Structure["services/data-processor/pyproject.toml"] = g.generatePythonDataProcessorPyproject(project.Name)
	project.Structure["services/data-processor/src/main.py"] = g.generatePythonDataProcessorMain()
	project.Structure["services/data-processor/src/processors/user_analytics.py"] = g.generatePythonUserAnalytics()
	project.Structure["services/data-processor/src/processors/report_generator.py"] = g.generatePythonReportGenerator()
	project.Structure["services/data-processor/src/database/connection.py"] = g.generatePythonDatabaseConnection()
	project.Structure["services/data-processor/src/models/analytics.py"] = g.generatePythonAnalyticsModels()
	project.Structure["services/data-processor/src/utils/data_transformers.py"] = g.generatePythonDataTransformers()
	project.Structure["services/data-processor/requirements.txt"] = g.generatePythonDataProcessorRequirements()
	project.Structure["services/data-processor/Dockerfile"] = g.generatePythonDockerfile()
	
	// Shared Libraries (TypeScript)
	project.Structure["packages/shared-types/package.json"] = g.generateSharedTypesPackageJson("shared-types")
	project.Structure["packages/shared-types/src/index.ts"] = g.generateSharedTypesIndex()
	project.Structure["packages/shared-types/src/user.ts"] = g.generateSharedUserTypes()
	project.Structure["packages/shared-types/src/api.ts"] = g.generateSharedAPITypes()
	project.Structure["packages/shared-types/src/events.ts"] = g.generateSharedEventTypes()
	project.Structure["packages/shared-types/tsconfig.json"] = g.generateSharedTypesTsConfig()
	
	project.Structure["packages/shared-utils/package.json"] = g.generateSharedUtilsPackageJson("shared-utils")
	project.Structure["packages/shared-utils/src/index.ts"] = g.generateSharedUtilsIndex()
	project.Structure["packages/shared-utils/src/validation.ts"] = g.generateSharedValidationUtils()
	project.Structure["packages/shared-utils/src/formatting.ts"] = g.generateSharedFormattingUtils()
	project.Structure["packages/shared-utils/src/constants.ts"] = g.generateSharedConstants()
	
	// Configuration and Infrastructure
	project.Structure[".github/workflows/ci.yml"] = g.generateMonorepoGitHubActions()
	project.Structure["infrastructure/docker-compose.dev.yml"] = g.generateDevDockerCompose()
	project.Structure["infrastructure/docker-compose.prod.yml"] = g.generateProdDockerCompose()
	project.Structure["infrastructure/nginx.conf"] = g.generateNginxConfig()
	project.Structure["scripts/setup.sh"] = g.generateMonorepoSetupScript()
	project.Structure["scripts/test-all.sh"] = g.generateMonorepoTestScript()
	project.Structure["scripts/deploy.sh"] = g.generateMonorepoDeployScript()
	
	// Documentation
	project.Structure["README.md"] = g.generateMonorepoWebAppReadme(project.Name)
	project.Structure["docs/ARCHITECTURE.md"] = g.generateMonorepoArchitectureDoc()
	project.Structure["docs/API.md"] = g.generateMonorepoAPIDoc()
	project.Structure["docs/DEPLOYMENT.md"] = g.generateMonorepoDeploymentDoc()
	
	project.BuildFiles = append(project.BuildFiles, "package.json", "lerna.json", "Makefile", "docker-compose.yml")
	return nil
}

// generateMicroservicesSuiteStructure generates 5+ services in different languages with inter-service dependencies
func (g *TestProjectGenerator) generateMicroservicesSuiteStructure(project *TestProject, config *ProjectGenerationConfig) error {
	microserviceCount := config.MicroserviceCount
	if microserviceCount < 5 {
		microserviceCount = 6 // Default to 6 services
	}
	
	// Root configuration
	project.Structure["docker-compose.yml"] = g.generateMicroservicesDockerCompose(microserviceCount)
	project.Structure["Makefile"] = g.generateMicroservicesMakefile()
	project.Structure["scripts/setup-all.sh"] = g.generateMicroservicesSetupScript()
	project.Structure["monitoring/prometheus.yml"] = g.generatePrometheusConfig()
	project.Structure["monitoring/grafana/dashboards/services.json"] = g.generateGrafanaDashboard()
	
	// API Gateway (Node.js/TypeScript)
	project.Structure["services/api-gateway/package.json"] = g.generateAPIGatewayPackageJson()
	project.Structure["services/api-gateway/src/index.ts"] = g.generateAPIGatewayMain()
	project.Structure["services/api-gateway/src/routes/index.ts"] = g.generateAPIGatewayRoutes()
	project.Structure["services/api-gateway/src/middleware/auth.ts"] = g.generateAPIGatewayAuthMiddleware()
	project.Structure["services/api-gateway/src/middleware/rate-limit.ts"] = g.generateAPIGatewayRateLimit()
	project.Structure["services/api-gateway/src/proxy/service-proxy.ts"] = g.generateServiceProxy()
	project.Structure["services/api-gateway/config/services.json"] = g.generateServiceDiscoveryConfig()
	
	// User Service (Go)
	project.Structure["services/user-service/go.mod"] = g.generateUserServiceGoMod()
	project.Structure["services/user-service/main.go"] = g.generateUserServiceMain()
	project.Structure["services/user-service/internal/handlers/grpc.go"] = g.generateUserServiceGRPCHandlers()
	project.Structure["services/user-service/internal/handlers/http.go"] = g.generateUserServiceHTTPHandlers()
	project.Structure["services/user-service/internal/service/user.go"] = g.generateUserServiceBusiness()
	project.Structure["services/user-service/internal/repository/postgres.go"] = g.generateUserServiceRepository()
	project.Structure["services/user-service/proto/user.proto"] = g.generateUserServiceProto()
	project.Structure["services/user-service/migrations/001_create_users.sql"] = g.generateUserServiceMigrations()
	
	// Order Service (Java Spring Boot)
	project.Structure["services/order-service/pom.xml"] = g.generateOrderServicePom()
	project.Structure["services/order-service/src/main/java/com/example/orders/Application.java"] = g.generateOrderServiceApplication()
	project.Structure["services/order-service/src/main/java/com/example/orders/controller/OrderController.java"] = g.generateOrderController()
	project.Structure["services/order-service/src/main/java/com/example/orders/service/OrderService.java"] = g.generateOrderServiceBusiness()
	project.Structure["services/order-service/src/main/java/com/example/orders/repository/OrderRepository.java"] = g.generateOrderRepository()
	project.Structure["services/order-service/src/main/java/com/example/orders/model/Order.java"] = g.generateOrderModel()
	project.Structure["services/order-service/src/main/java/com/example/orders/config/KafkaConfig.java"] = g.generateOrderKafkaConfig()
	project.Structure["services/order-service/src/main/resources/application.yml"] = g.generateOrderServiceConfig()
	
	// Payment Service (Python FastAPI)
	project.Structure["services/payment-service/pyproject.toml"] = g.generatePaymentServicePyproject(project.Name)
	project.Structure["services/payment-service/main.py"] = g.generatePaymentServiceMain()
	project.Structure["services/payment-service/app/routers/payments.py"] = g.generatePaymentRouters()
	project.Structure["services/payment-service/app/services/payment_processor.py"] = g.generatePaymentProcessor()
	project.Structure["services/payment-service/app/services/stripe_integration.py"] = g.generateStripeIntegration()
	project.Structure["services/payment-service/app/models/payment.py"] = g.generatePaymentModels()
	project.Structure["services/payment-service/alembic/versions/001_create_payments.py"] = g.generatePaymentMigrations()
	
	// Notification Service (Rust)
	project.Structure["services/notification-service/Cargo.toml"] = g.generateNotificationServiceCargo()
	project.Structure["services/notification-service/src/main.rs"] = g.generateNotificationServiceMain()
	project.Structure["services/notification-service/src/handlers/mod.rs"] = g.generateNotificationHandlers()
	project.Structure["services/notification-service/src/services/email.rs"] = g.generateEmailService()
	project.Structure["services/notification-service/src/services/sms.rs"] = g.generateSMSService()
	project.Structure["services/notification-service/src/models/notification.rs"] = g.generateNotificationModels()
	project.Structure["services/notification-service/src/queue/redis.rs"] = g.generateRedisQueue()
	
	// Analytics Service (C#/.NET)
	project.Structure["services/analytics-service/Analytics.Service.csproj"] = g.generateAnalyticsServiceCsproj()
	project.Structure["services/analytics-service/Program.cs"] = g.generateAnalyticsServiceProgram()
	project.Structure["services/analytics-service/Controllers/AnalyticsController.cs"] = g.generateAnalyticsController()
	project.Structure["services/analytics-service/Services/EventProcessor.cs"] = g.generateEventProcessor()
	project.Structure["services/analytics-service/Services/MetricsCollector.cs"] = g.generateMetricsCollector()
	project.Structure["services/analytics-service/Models/Event.cs"] = g.generateAnalyticsEventModel()
	project.Structure["services/analytics-service/appsettings.json"] = g.generateAnalyticsServiceConfig()
	
	// Infrastructure
	project.Structure["infrastructure/kubernetes/namespace.yaml"] = g.generateK8sNamespace()
	project.Structure["infrastructure/kubernetes/ingress.yaml"] = g.generateK8sIngress()
	project.Structure["infrastructure/kubernetes/configmap.yaml"] = g.generateK8sConfigMap()
	project.Structure["infrastructure/terraform/main.tf"] = g.generateTerraformMain()
	project.Structure["infrastructure/terraform/variables.tf"] = g.generateTerraformVariables()
	project.Structure["infrastructure/helm/Chart.yaml"] = g.generateHelmChart()
	
	// Service mesh (Istio)
	project.Structure["service-mesh/virtual-service.yaml"] = g.generateIstioVirtualService()
	project.Structure["service-mesh/destination-rule.yaml"] = g.generateIstioDestinationRule()
	project.Structure["service-mesh/gateway.yaml"] = g.generateIstioGateway()
	
	project.BuildFiles = append(project.BuildFiles, "docker-compose.yml", "Makefile")
	return nil
}

// generatePythonWithGoExtensionsStructure generates Python main project with Go performance-critical modules
func (g *TestProjectGenerator) generatePythonWithGoExtensionsStructure(project *TestProject, config *ProjectGenerationConfig) error {
	// Python main project
	project.Structure["pyproject.toml"] = g.generatePythonGoExtensionsPyproject(project.Name)
	project.Structure["setup.py"] = g.generatePythonGoExtensionsSetup(project.Name)
	project.Structure["requirements.txt"] = g.generatePythonGoExtensionsRequirements()
	project.Structure["requirements-dev.txt"] = g.generatePythonGoExtensionsDevRequirements()
	
	// Python application
	project.Structure["src/main.py"] = g.generatePythonGoExtensionsMain()
	project.Structure["src/app/__init__.py"] = ""
	project.Structure["src/app/core/engine.py"] = g.generatePythonCoreEngine()
	project.Structure["src/app/core/processor.py"] = g.generatePythonProcessor()
	project.Structure["src/app/api/routes.py"] = g.generatePythonAPIRoutes()
	project.Structure["src/app/models/data.py"] = g.generatePythonDataModels()
	project.Structure["src/app/services/calculation.py"] = g.generatePythonCalculationService()
	project.Structure["src/app/utils/performance.py"] = g.generatePythonPerformanceUtils()
	
	// Go extensions for performance-critical operations
	project.Structure["extensions/go.mod"] = g.generateGoExtensionsMod(project.Name)
	project.Structure["extensions/main.go"] = g.generateGoExtensionsMain()
	project.Structure["extensions/calculator/calculator.go"] = g.generateGoCalculatorExtension()
	project.Structure["extensions/parser/parser.go"] = g.generateGoParserExtension()
	project.Structure["extensions/crypto/hasher.go"] = g.generateGoCryptoExtension()
	project.Structure["extensions/compression/compressor.go"] = g.generateGoCompressionExtension()
	project.Structure["extensions/c-bridge/bridge.go"] = g.generateGoCBridge()
	project.Structure["extensions/c-bridge/bridge.h"] = g.generateGoCBridgeHeader()
	
	// Python bindings and wrappers
	project.Structure["src/extensions/__init__.py"] = ""
	project.Structure["src/extensions/go_calculator.py"] = g.generatePythonGoCalculatorWrapper()
	project.Structure["src/extensions/go_parser.py"] = g.generatePythonGoParserWrapper()
	project.Structure["src/extensions/go_crypto.py"] = g.generatePythonGoCryptoWrapper()
	project.Structure["src/extensions/go_compression.py"] = g.generatePythonGoCompressionWrapper()
	
	// Build and configuration
	project.Structure["Makefile"] = g.generatePythonGoExtensionsMakefile()
	project.Structure["build.py"] = g.generatePythonGoExtensionsBuildScript()
	project.Structure["scripts/build-extensions.sh"] = g.generateBuildExtensionsScript()
	project.Structure["scripts/test-performance.py"] = g.generatePerformanceTestScript()
	
	// Configuration
	project.Structure["config/settings.py"] = g.generatePythonGoExtensionsSettings()
	project.Structure["config/logging.conf"] = g.generatePythonLoggingConfig()
	
	// Tests
	project.Structure["tests/test_extensions.py"] = g.generatePythonExtensionsTests()
	project.Structure["tests/test_performance.py"] = g.generatePythonPerformanceTests()
	project.Structure["tests/benchmarks/compare_implementations.py"] = g.generateImplementationComparison()
	
	project.BuildFiles = append(project.BuildFiles, "pyproject.toml", "setup.py", "Makefile")
	return nil
}

// generateFullStackTypeScriptStructure generates Node.js backend + React frontend + TypeScript everywhere
func (g *TestProjectGenerator) generateFullStackTypeScriptStructure(project *TestProject, config *ProjectGenerationConfig) error {
	// Root workspace configuration
	project.Structure["package.json"] = g.generateFullStackTSRootPackageJson(project.Name)
	project.Structure["tsconfig.json"] = g.generateFullStackTSRootTsConfig()
	project.Structure["yarn.lock"] = "# Yarn lockfile\n"
	project.Structure[".yarnrc.yml"] = g.generateYarnConfig()
	
	// Backend (Node.js + Express + TypeScript)
	project.Structure["apps/backend/package.json"] = g.generateFullStackBackendPackageJson("backend")
	project.Structure["apps/backend/tsconfig.json"] = g.generateFullStackBackendTsConfig()
	project.Structure["apps/backend/src/index.ts"] = g.generateFullStackBackendMain()
	project.Structure["apps/backend/src/app.ts"] = g.generateFullStackBackendApp()
	project.Structure["apps/backend/src/routes/users.ts"] = g.generateFullStackBackendUserRoutes()
	project.Structure["apps/backend/src/routes/auth.ts"] = g.generateFullStackBackendAuthRoutes()
	project.Structure["apps/backend/src/controllers/UserController.ts"] = g.generateFullStackUserController()
	project.Structure["apps/backend/src/controllers/AuthController.ts"] = g.generateFullStackAuthController()
	project.Structure["apps/backend/src/services/UserService.ts"] = g.generateFullStackUserService()
	project.Structure["apps/backend/src/services/AuthService.ts"] = g.generateFullStackAuthService()
	project.Structure["apps/backend/src/models/User.ts"] = g.generateFullStackUserModel()
	project.Structure["apps/backend/src/models/Auth.ts"] = g.generateFullStackAuthModel()
	project.Structure["apps/backend/src/database/connection.ts"] = g.generateFullStackDatabaseConnection()
	project.Structure["apps/backend/src/database/migrations/001-create-users.ts"] = g.generateFullStackUserMigration()
	project.Structure["apps/backend/src/middleware/auth.ts"] = g.generateFullStackBackendAuthMiddleware()
	project.Structure["apps/backend/src/middleware/validation.ts"] = g.generateFullStackValidationMiddleware()
	project.Structure["apps/backend/src/utils/logger.ts"] = g.generateFullStackLogger()
	project.Structure["apps/backend/src/utils/response.ts"] = g.generateFullStackResponseUtils()
	project.Structure["apps/backend/.env.example"] = g.generateFullStackBackendEnvExample()
	
	// Frontend (React + TypeScript)
	project.Structure["apps/frontend/package.json"] = g.generateFullStackFrontendPackageJson("frontend")
	project.Structure["apps/frontend/tsconfig.json"] = g.generateFullStackFrontendTsConfig()
	project.Structure["apps/frontend/vite.config.ts"] = g.generateViteConfig()
	project.Structure["apps/frontend/index.html"] = g.generateFullStackFrontendIndexHTML()
	project.Structure["apps/frontend/src/main.tsx"] = g.generateFullStackFrontendMain()
	project.Structure["apps/frontend/src/App.tsx"] = g.generateFullStackFrontendApp()
	project.Structure["apps/frontend/src/pages/HomePage.tsx"] = g.generateFullStackHomePage()
	project.Structure["apps/frontend/src/pages/LoginPage.tsx"] = g.generateFullStackLoginPage()
	project.Structure["apps/frontend/src/pages/DashboardPage.tsx"] = g.generateFullStackDashboardPage()
	project.Structure["apps/frontend/src/components/common/Header.tsx"] = g.generateFullStackHeader()
	project.Structure["apps/frontend/src/components/common/Footer.tsx"] = g.generateFullStackFooter()
	project.Structure["apps/frontend/src/components/user/UserList.tsx"] = g.generateFullStackUserListComponent()
	project.Structure["apps/frontend/src/components/user/UserForm.tsx"] = g.generateFullStackUserFormComponent()
	project.Structure["apps/frontend/src/services/api.ts"] = g.generateFullStackFrontendAPIService()
	project.Structure["apps/frontend/src/services/auth.ts"] = g.generateFullStackFrontendAuthService()
	project.Structure["apps/frontend/src/hooks/useAuth.ts"] = g.generateFullStackAuthHook()
	project.Structure["apps/frontend/src/hooks/useUsers.ts"] = g.generateFullStackUsersHook()
	project.Structure["apps/frontend/src/store/auth.ts"] = g.generateFullStackAuthStore()
	project.Structure["apps/frontend/src/store/users.ts"] = g.generateFullStackUsersStore()
	project.Structure["apps/frontend/src/utils/api.ts"] = g.generateFullStackFrontendAPIUtils()
	project.Structure["apps/frontend/src/utils/validation.ts"] = g.generateFullStackFrontendValidationUtils()
	project.Structure["apps/frontend/src/styles/globals.css"] = g.generateFullStackGlobalCSS()
	project.Structure["apps/frontend/.env.example"] = g.generateFullStackFrontendEnvExample()
	
	// Shared libraries
	project.Structure["packages/shared/package.json"] = g.generateFullStackSharedPackageJson("shared")
	project.Structure["packages/shared/tsconfig.json"] = g.generateFullStackSharedTsConfig()
	project.Structure["packages/shared/src/index.ts"] = g.generateFullStackSharedIndex()
	project.Structure["packages/shared/src/types/user.ts"] = g.generateFullStackSharedUserTypes()
	project.Structure["packages/shared/src/types/auth.ts"] = g.generateFullStackSharedAuthTypes()
	project.Structure["packages/shared/src/types/api.ts"] = g.generateFullStackSharedAPITypes()
	project.Structure["packages/shared/src/validation/user.ts"] = g.generateFullStackSharedUserValidation()
	project.Structure["packages/shared/src/validation/auth.ts"] = g.generateFullStackSharedAuthValidation()
	project.Structure["packages/shared/src/constants/index.ts"] = g.generateFullStackSharedConstants()
	project.Structure["packages/shared/src/utils/date.ts"] = g.generateFullStackSharedDateUtils()
	project.Structure["packages/shared/src/utils/string.ts"] = g.generateFullStackSharedStringUtils()
	
	// Development and build tools
	project.Structure["turbo.json"] = g.generateTurboConfig()
	project.Structure[".eslintrc.js"] = g.generateFullStackESLintConfig()
	project.Structure["prettier.config.js"] = g.generateFullStackPrettierConfig()
	project.Structure["jest.config.js"] = g.generateFullStackJestConfig()
	
	// Docker
	project.Structure["apps/backend/Dockerfile"] = g.generateFullStackBackendDockerfile()
	project.Structure["apps/frontend/Dockerfile"] = g.generateFullStackFrontendDockerfile()
	project.Structure["docker-compose.yml"] = g.generateFullStackDockerCompose()
	project.Structure["docker-compose.dev.yml"] = g.generateFullStackDevDockerCompose()
	
	// CI/CD
	project.Structure[".github/workflows/ci.yml"] = g.generateFullStackGitHubActions()
	project.Structure[".github/workflows/deploy.yml"] = g.generateFullStackDeployActions()
	
	project.BuildFiles = append(project.BuildFiles, "package.json", "turbo.json")
	return nil
}

// generateJavaSpringWithReactStructure generates Java Spring Boot + React frontend + Maven multi-module setup
func (g *TestProjectGenerator) generateJavaSpringWithReactStructure(project *TestProject, config *ProjectGenerationConfig) error {
	// Root Maven configuration
	project.Structure["pom.xml"] = g.generateJavaSpringReactRootPom(project.Name)
	project.Structure[".mvn/wrapper/maven-wrapper.properties"] = g.generateMavenWrapperProperties()
	project.Structure["mvnw"] = g.generateMavenWrapper()
	
	// Backend (Spring Boot)
	project.Structure["backend/pom.xml"] = g.generateJavaSpringBackendPom()
	project.Structure["backend/src/main/java/com/example/app/Application.java"] = g.generateJavaSpringApplication()
	project.Structure["backend/src/main/java/com/example/app/config/WebConfig.java"] = g.generateJavaSpringWebConfig()
	project.Structure["backend/src/main/java/com/example/app/config/SecurityConfig.java"] = g.generateJavaSpringSecurityConfig()
	project.Structure["backend/src/main/java/com/example/app/config/DatabaseConfig.java"] = g.generateJavaSpringDatabaseConfig()
	project.Structure["backend/src/main/java/com/example/app/controller/UserController.java"] = g.generateJavaSpringUserController()
	project.Structure["backend/src/main/java/com/example/app/controller/AuthController.java"] = g.generateJavaSpringAuthController()
	project.Structure["backend/src/main/java/com/example/app/service/UserService.java"] = g.generateJavaSpringUserService()
	project.Structure["backend/src/main/java/com/example/app/service/AuthService.java"] = g.generateJavaSpringAuthService()
	project.Structure["backend/src/main/java/com/example/app/repository/UserRepository.java"] = g.generateJavaSpringUserRepository()
	project.Structure["backend/src/main/java/com/example/app/model/User.java"] = g.generateJavaSpringUserModel()
	project.Structure["backend/src/main/java/com/example/app/model/Role.java"] = g.generateJavaSpringRoleModel()
	project.Structure["backend/src/main/java/com/example/app/dto/UserDTO.java"] = g.generateJavaSpringUserDTO()
	project.Structure["backend/src/main/java/com/example/app/dto/AuthDTO.java"] = g.generateJavaSpringAuthDTO()
	project.Structure["backend/src/main/java/com/example/app/exception/GlobalExceptionHandler.java"] = g.generateJavaSpringExceptionHandler()
	project.Structure["backend/src/main/java/com/example/app/security/JwtTokenProvider.java"] = g.generateJavaSpringJwtProvider()
	project.Structure["backend/src/main/java/com/example/app/security/JwtAuthenticationFilter.java"] = g.generateJavaSpringJwtFilter()
	project.Structure["backend/src/main/resources/application.yml"] = g.generateJavaSpringApplicationYml()
	project.Structure["backend/src/main/resources/application-dev.yml"] = g.generateJavaSpringDevApplicationYml()
	project.Structure["backend/src/main/resources/application-prod.yml"] = g.generateJavaSpringProdApplicationYml()
	project.Structure["backend/src/main/resources/db/migration/V1__Create_users_table.sql"] = g.generateJavaSpringUserMigration()
	project.Structure["backend/src/main/resources/db/migration/V2__Create_roles_table.sql"] = g.generateJavaSpringRoleMigration()
	
	// Frontend (React + TypeScript)
	project.Structure["frontend/package.json"] = g.generateJavaSpringReactFrontendPackageJson()
	project.Structure["frontend/tsconfig.json"] = g.generateJavaSpringReactTsConfig()
	project.Structure["frontend/vite.config.ts"] = g.generateJavaSpringReactViteConfig()
	project.Structure["frontend/index.html"] = g.generateJavaSpringReactIndexHTML()
	project.Structure["frontend/src/main.tsx"] = g.generateJavaSpringReactMain()
	project.Structure["frontend/src/App.tsx"] = g.generateJavaSpringReactApp()
	project.Structure["frontend/src/pages/LoginPage.tsx"] = g.generateJavaSpringReactLoginPage()
	project.Structure["frontend/src/pages/DashboardPage.tsx"] = g.generateJavaSpringReactDashboardPage()
	project.Structure["frontend/src/pages/UsersPage.tsx"] = g.generateJavaSpringReactUsersPage()
	project.Structure["frontend/src/components/Layout.tsx"] = g.generateJavaSpringReactLayout()
	project.Structure["frontend/src/components/UserTable.tsx"] = g.generateJavaSpringReactUserTable()
	project.Structure["frontend/src/components/UserForm.tsx"] = g.generateJavaSpringReactUserForm()
	project.Structure["frontend/src/services/api.ts"] = g.generateJavaSpringReactAPIService()
	project.Structure["frontend/src/services/auth.ts"] = g.generateJavaSpringReactAuthService()
	project.Structure["frontend/src/hooks/useAuth.ts"] = g.generateJavaSpringReactAuthHook()
	project.Structure["frontend/src/types/api.ts"] = g.generateJavaSpringReactAPITypes()
	project.Structure["frontend/src/types/user.ts"] = g.generateJavaSpringReactUserTypes()
	project.Structure["frontend/src/utils/http.ts"] = g.generateJavaSpringReactHTTPUtils()
	
	// Shared module
	project.Structure["shared/pom.xml"] = g.generateJavaSpringSharedPom()
	project.Structure["shared/src/main/java/com/example/shared/dto/BaseDTO.java"] = g.generateJavaSpringBaseDTO()
	project.Structure["shared/src/main/java/com/example/shared/exception/BusinessException.java"] = g.generateJavaSpringBusinessException()
	project.Structure["shared/src/main/java/com/example/shared/util/DateUtils.java"] = g.generateJavaSpringDateUtils()
	project.Structure["shared/src/main/java/com/example/shared/util/ValidationUtils.java"] = g.generateJavaSpringValidationUtils()
	
	// Build and deployment
	project.Structure["Dockerfile.backend"] = g.generateJavaSpringBackendDockerfile()
	project.Structure["Dockerfile.frontend"] = g.generateJavaSpringFrontendDockerfile()
	project.Structure["docker-compose.yml"] = g.generateJavaSpringDockerCompose()
	project.Structure["scripts/build.sh"] = g.generateJavaSpringBuildScript()
	project.Structure["scripts/deploy.sh"] = g.generateJavaSpringDeployScript()
	
	project.BuildFiles = append(project.BuildFiles, "pom.xml", "backend/pom.xml", "shared/pom.xml")
	return nil
}

// generateEnterprisePlatformStructure generates large-scale platform with 8+ languages and complex dependencies
func (g *TestProjectGenerator) generateEnterprisePlatformStructure(project *TestProject, config *ProjectGenerationConfig) error {
	// Platform root configuration
	project.Structure["platform.yaml"] = g.generateEnterprisePlatformConfig(project.Name)
	project.Structure["Makefile"] = g.generateEnterprisePlatformMakefile()
	project.Structure["docker-compose.enterprise.yml"] = g.generateEnterpriseDockerCompose()
	project.Structure["scripts/bootstrap.sh"] = g.generateEnterpriseBootstrapScript()
	project.Structure["scripts/deploy-all.sh"] = g.generateEnterpriseDeployScript()
	
	// Core Platform Services (Go)
	project.Structure["platform/core/go.mod"] = g.generateEnterpriseCoreGoMod(project.Name)
	project.Structure["platform/core/cmd/platform/main.go"] = g.generateEnterpriseCoreMain()
	project.Structure["platform/core/internal/orchestrator/orchestrator.go"] = g.generatePlatformOrchestrator()
	project.Structure["platform/core/internal/registry/service_registry.go"] = g.generateServiceRegistry()
	project.Structure["platform/core/internal/config/platform_config.go"] = g.generatePlatformConfig()
	project.Structure["platform/core/internal/health/health_checker.go"] = g.generatePlatformHealthChecker()
	project.Structure["platform/core/pkg/middleware/logging.go"] = g.generatePlatformLoggingMiddleware()
	project.Structure["platform/core/pkg/middleware/metrics.go"] = g.generatePlatformMetricsMiddleware()
	
	// Authentication Service (Java Spring Boot)
	project.Structure["services/auth-service/pom.xml"] = g.generateEnterpriseAuthServicePom()
	project.Structure["services/auth-service/src/main/java/com/enterprise/auth/AuthApplication.java"] = g.generateEnterpriseAuthApplication()
	project.Structure["services/auth-service/src/main/java/com/enterprise/auth/controller/AuthController.java"] = g.generateEnterpriseAuthController()
	project.Structure["services/auth-service/src/main/java/com/enterprise/auth/service/TokenService.java"] = g.generateEnterpriseTokenService()
	project.Structure["services/auth-service/src/main/java/com/enterprise/auth/security/JwtManager.java"] = g.generateEnterpriseJwtManager()
	
	// Data Processing Pipeline (Python)
	project.Structure["services/data-pipeline/pyproject.toml"] = g.generateEnterpriseDataPipelinePyproject()
	project.Structure["services/data-pipeline/src/pipeline/orchestrator.py"] = g.generateDataPipelineOrchestrator()
	project.Structure["services/data-pipeline/src/processors/stream_processor.py"] = g.generateStreamProcessor()
	project.Structure["services/data-pipeline/src/processors/batch_processor.py"] = g.generateBatchProcessor()
	project.Structure["services/data-pipeline/src/connectors/kafka_connector.py"] = g.generateKafkaConnector()
	
	// Real-time Analytics (Rust)
	project.Structure["services/analytics-engine/Cargo.toml"] = g.generateEnterpriseAnalyticsEngineCargo()
	project.Structure["services/analytics-engine/src/main.rs"] = g.generateEnterpriseAnalyticsEngineMain()
	project.Structure["services/analytics-engine/src/engine/query_engine.rs"] = g.generateQueryEngine()
	project.Structure["services/analytics-engine/src/engine/aggregator.rs"] = g.generateDataAggregator()
	project.Structure["services/analytics-engine/src/storage/time_series.rs"] = g.generateTimeSeriesStorage()
	
	// Machine Learning Service (C++)
	project.Structure["services/ml-service/CMakeLists.txt"] = g.generateEnterpriseMLServiceCMake()
	project.Structure["services/ml-service/src/main.cpp"] = g.generateEnterpriseMLServiceMain()
	project.Structure["services/ml-service/src/inference/model_server.cpp"] = g.generateMLModelServer()
	project.Structure["services/ml-service/src/inference/model_server.h"] = g.generateMLModelServerHeader()
	project.Structure["services/ml-service/src/training/trainer.cpp"] = g.generateMLTrainer()
	project.Structure["services/ml-service/src/training/trainer.h"] = g.generateMLTrainerHeader()
	
	// Configuration Service (C#/.NET)
	project.Structure["services/config-service/ConfigService.csproj"] = g.generateEnterpriseConfigServiceCsproj()
	project.Structure["services/config-service/Program.cs"] = g.generateEnterpriseConfigServiceProgram()
	project.Structure["services/config-service/Controllers/ConfigController.cs"] = g.generateEnterpriseConfigController()
	project.Structure["services/config-service/Services/ConfigManager.cs"] = g.generateEnterpriseConfigManager()
	
	// Monitoring and Observability (TypeScript/Node.js)
	project.Structure["services/monitoring/package.json"] = g.generateEnterpriseMonitoringPackageJson()
	project.Structure["services/monitoring/src/index.ts"] = g.generateEnterpriseMonitoringMain()
	project.Structure["services/monitoring/src/collectors/metrics_collector.ts"] = g.generateMetricsCollectorTS()
	project.Structure["services/monitoring/src/collectors/trace_collector.ts"] = g.generateTraceCollector()
	project.Structure["services/monitoring/src/exporters/prometheus_exporter.ts"] = g.generatePrometheusExporter()
	
	// Event Streaming (Scala)
	project.Structure["services/event-streaming/build.sbt"] = g.generateEnterpriseEventStreamingBuild()
	project.Structure["services/event-streaming/src/main/scala/com/enterprise/streaming/StreamingApp.scala"] = g.generateEventStreamingApp()
	project.Structure["services/event-streaming/src/main/scala/com/enterprise/streaming/processors/EventProcessor.scala"] = g.generateEventProcessor()
	
	// Admin Dashboard (React + TypeScript)
	project.Structure["admin-dashboard/package.json"] = g.generateEnterpriseAdminDashboardPackageJson()
	project.Structure["admin-dashboard/src/App.tsx"] = g.generateEnterpriseAdminDashboardApp()
	project.Structure["admin-dashboard/src/pages/ServicesPage.tsx"] = g.generateAdminServicesPage()
	project.Structure["admin-dashboard/src/pages/MetricsPage.tsx"] = g.generateAdminMetricsPage()
	project.Structure["admin-dashboard/src/components/ServiceCard.tsx"] = g.generateAdminServiceCard()
	
	// Infrastructure as Code
	project.Structure["infrastructure/terraform/main.tf"] = g.generateEnterpriseTerraformMain()
	project.Structure["infrastructure/terraform/modules/vpc/main.tf"] = g.generateTerraformVPCModule()
	project.Structure["infrastructure/terraform/modules/eks/main.tf"] = g.generateTerraformEKSModule()
	project.Structure["infrastructure/terraform/modules/rds/main.tf"] = g.generateTerraformRDSModule()
	
	// Kubernetes manifests
	project.Structure["k8s/namespace.yaml"] = g.generateEnterpriseK8sNamespace()
	project.Structure["k8s/ingress/nginx-ingress.yaml"] = g.generateEnterpriseNginxIngress()
	project.Structure["k8s/monitoring/prometheus.yaml"] = g.generateEnterprisePrometheusConfig()
	project.Structure["k8s/monitoring/grafana.yaml"] = g.generateEnterpriseGrafanaConfig()
	project.Structure["k8s/logging/fluentd.yaml"] = g.generateEnterpriseFluentdConfig()
	
	// Service mesh configuration
	project.Structure["service-mesh/istio/gateway.yaml"] = g.generateEnterpriseIstioGateway()
	project.Structure["service-mesh/istio/virtual-services.yaml"] = g.generateEnterpriseIstioVirtualServices()
	project.Structure["service-mesh/istio/destination-rules.yaml"] = g.generateEnterpriseIstioDestinationRules()
	
	// CI/CD pipelines
	project.Structure[".github/workflows/platform-ci.yml"] = g.generateEnterprisePlatformCI()
	project.Structure[".github/workflows/service-deployment.yml"] = g.generateEnterpriseServiceDeployment()
	project.Structure["jenkins/Jenkinsfile"] = g.generateEnterpriseJenkinsfile()
	
	// Documentation
	project.Structure["docs/README.md"] = g.generateEnterprisePlatformReadme(project.Name)
	project.Structure["docs/ARCHITECTURE.md"] = g.generateEnterprisePlatformArchitecture()
	project.Structure["docs/API.md"] = g.generateEnterprisePlatformAPI()
	project.Structure["docs/DEPLOYMENT.md"] = g.generateEnterprisePlatformDeployment()
	project.Structure["docs/MONITORING.md"] = g.generateEnterprisePlatformMonitoring()
	
	project.BuildFiles = append(project.BuildFiles, "platform.yaml", "Makefile", "docker-compose.enterprise.yml")
	return nil
}

// ==================== MONOREPO WEBAPP HELPER METHODS ====================

// generateMonorepoRootPackageJson generates the root package.json for monorepo webapp
func (g *TestProjectGenerator) generateMonorepoRootPackageJson(projectName string) string {
	return fmt.Sprintf(`{
  "name": "%s",
  "version": "1.0.0",
  "private": true,
  "workspaces": [
    "packages/*",
    "services/*"
  ],
  "scripts": {
    "build": "lerna run build",
    "start": "concurrently \"npm run start:frontend\" \"npm run start:backend\" \"npm run start:data-processor\"",
    "start:frontend": "cd packages/frontend && npm start",
    "start:backend": "cd services/backend && go run main.go",
    "start:data-processor": "cd services/data-processor && python src/main.py",
    "test": "lerna run test",
    "lint": "lerna run lint",
    "clean": "lerna clean && rimraf node_modules"
  },
  "devDependencies": {
    "lerna": "^6.4.1",
    "concurrently": "^7.6.0",
    "rimraf": "^4.1.2"
  }
}`, projectName)
}

// generateLernaConfig generates lerna.json configuration
func (g *TestProjectGenerator) generateLernaConfig() string {
	return `{
  "version": "independent",
  "npmClient": "npm",
  "useWorkspaces": true,
  "command": {
    "publish": {
      "conventionalCommits": true,
      "message": "chore(release): publish"
    },
    "version": {
      "allowBranch": ["master", "main"],
      "conventionalCommits": true,
      "createRelease": "github"
    }
  }
}`
}

// generateNxWorkspaceConfig generates workspace.json for Nx
func (g *TestProjectGenerator) generateNxWorkspaceConfig() string {
	return `{
  "version": 2,
  "projects": {
    "frontend": "packages/frontend",
    "shared-types": "packages/shared-types",
    "shared-utils": "packages/shared-utils",
    "backend": "services/backend",
    "data-processor": "services/data-processor"
  }
}`
}

// generateMonorepoMakefile generates Makefile for monorepo management
func (g *TestProjectGenerator) generateMonorepoMakefile() string {
	return `# Monorepo WebApp Makefile

.PHONY: install build test clean start stop

# Install all dependencies
install:
	npm install
	cd services/backend && go mod download
	cd services/data-processor && pip install -r requirements.txt

# Build all projects
build:
	npm run build
	cd services/backend && go build -o bin/backend main.go
	cd services/data-processor && python -m py_compile src/main.py

# Run tests
test:
	npm run test
	cd services/backend && go test ./...
	cd services/data-processor && python -m pytest

# Clean build artifacts
clean:
	npm run clean
	cd services/backend && rm -rf bin/
	cd services/data-processor && find . -name "*.pyc" -delete

# Start all services
start:
	docker-compose up -d
	npm run start

# Stop all services
stop:
	docker-compose down
	pkill -f "npm start"
	pkill -f "go run"
	pkill -f "python src/main.py"`
}

// generateMonorepoDockerCompose generates docker-compose.yml for development
func (g *TestProjectGenerator) generateMonorepoDockerCompose() string {
	return `version: '3.8'

services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: webapp_db
      POSTGRES_USER: webapp_user
      POSTGRES_PASSWORD: webapp_pass
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    command: redis-server --appendonly yes
    volumes:
      - redis_data:/data

  frontend:
    build:
      context: ./packages/frontend
      dockerfile: Dockerfile
    ports:
      - "3000:3000"
    environment:
      - REACT_APP_API_URL=http://localhost:8080
    depends_on:
      - backend

  backend:
    build:
      context: ./services/backend
      dockerfile: Dockerfile
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgres://webapp_user:webapp_pass@postgres:5432/webapp_db
      - REDIS_URL=redis://redis:6379
    depends_on:
      - postgres
      - redis

  data-processor:
    build:
      context: ./services/data-processor
      dockerfile: Dockerfile
    environment:
      - DATABASE_URL=postgres://webapp_user:webapp_pass@postgres:5432/webapp_db
      - REDIS_URL=redis://redis:6379
    depends_on:
      - postgres
      - redis

volumes:
  postgres_data:
  redis_data:`
}

// generateReactPackageJson generates package.json for React frontend
func (g *TestProjectGenerator) generateReactPackageJson(packageName string) string {
	return fmt.Sprintf(`{
  "name": "%s",
  "version": "1.0.0",
  "private": true,
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0",
    "react-router-dom": "^6.8.1",
    "axios": "^1.3.4",
    "react-query": "^3.39.3",
    "@mui/material": "^5.11.10",
    "@mui/icons-material": "^5.11.9",
    "@emotion/react": "^11.10.5",
    "@emotion/styled": "^11.10.5"
  },
  "scripts": {
    "start": "react-scripts start",
    "build": "react-scripts build",
    "test": "react-scripts test",
    "eject": "react-scripts eject",
    "lint": "eslint src --ext .js,.jsx,.ts,.tsx"
  },
  "devDependencies": {
    "react-scripts": "5.0.1",
    "@types/react": "^18.0.28",
    "@types/react-dom": "^18.0.11",
    "typescript": "^4.9.5",
    "@typescript-eslint/eslint-plugin": "^5.54.0",
    "@typescript-eslint/parser": "^5.54.0",
    "eslint": "^8.35.0"
  },
  "browserslist": {
    "production": [
      ">0.2%%",
      "not dead",
      "not op_mini all"
    ],
    "development": [
      "last 1 chrome version",
      "last 1 firefox version",
      "last 1 safari version"
    ]
  }
}`, packageName)
}

// generateReactTsConfig generates tsconfig.json for React
func (g *TestProjectGenerator) generateReactTsConfig() string {
	return `{
  "compilerOptions": {
    "target": "es5",
    "lib": [
      "dom",
      "dom.iterable",
      "es6"
    ],
    "allowJs": true,
    "skipLibCheck": true,
    "esModuleInterop": true,
    "allowSyntheticDefaultImports": true,
    "strict": true,
    "forceConsistentCasingInFileNames": true,
    "noFallthroughCasesInSwitch": true,
    "module": "esnext",
    "moduleResolution": "node",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx"
  },
  "include": [
    "src"
  ]
}`
}

// generateReactApp generates React App.tsx component
func (g *TestProjectGenerator) generateReactApp() string {
	return `import React from 'react';
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from 'react-query';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import CssBaseline from '@mui/material/CssBaseline';
import AppBar from '@mui/material/AppBar';
import Toolbar from '@mui/material/Toolbar';
import Typography from '@mui/material/Typography';
import Container from '@mui/material/Container';
import UserList from './components/UserList';

const queryClient = new QueryClient();

const theme = createTheme({
  palette: {
    primary: {
      main: '#1976d2',
    },
    secondary: {
      main: '#dc004e',
    },
  },
});

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <Router>
          <AppBar position="sticky">
            <Toolbar>
              <Typography variant="h6" component="div" sx={{ flexGrow: 1 }}>
                Monorepo WebApp
              </Typography>
            </Toolbar>
          </AppBar>
          <Container maxWidth="lg" sx={{ mt: 4, mb: 4 }}>
            <Routes>
              <Route path="/" element={<UserList />} />
              <Route path="/users" element={<UserList />} />
            </Routes>
          </Container>
        </Router>
      </ThemeProvider>
    </QueryClientProvider>
  );
}

export default App;`
}

// generateReactIndex generates React index.tsx
func (g *TestProjectGenerator) generateReactIndex() string {
	return `import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';

const root = ReactDOM.createRoot(
  document.getElementById('root') as HTMLElement
);
root.render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);`
}

// generateReactUserListComponent generates UserList React component
func (g *TestProjectGenerator) generateReactUserListComponent() string {
	return `import React from 'react';
import { useQuery } from 'react-query';
import {
  Card,
  CardContent,
  Typography,
  Grid,
  CircularProgress,
  Alert,
  Button,
  Box
} from '@mui/material';
import { Person, Email, Phone } from '@mui/icons-material';
import { fetchUsers } from '../services/api';
import { User } from '../types/api';

const UserList: React.FC = () => {
  const { data: users, isLoading, error, refetch } = useQuery<User[]>(
    'users',
    fetchUsers,
    {
      refetchInterval: 30000, // Refetch every 30 seconds
    }
  );

  if (isLoading) {
    return (
      <Box display="flex" justifyContent="center" mt={4}>
        <CircularProgress />
      </Box>
    );
  }

  if (error) {
    return (
      <Alert severity="error" action={
        <Button color="inherit" size="small" onClick={() => refetch()}>
          Retry
        </Button>
      }>
        Failed to load users. Please try again.
      </Alert>
    );
  }

  return (
    <div>
      <Typography variant="h4" component="h1" gutterBottom>
        Users
      </Typography>
      <Grid container spacing={3}>
        {users?.map((user) => (
          <Grid item xs={12} sm={6} md={4} key={user.id}>
            <Card>
              <CardContent>
                <Box display="flex" alignItems="center" mb={1}>
                  <Person color="primary" sx={{ mr: 1 }} />
                  <Typography variant="h6">{user.name}</Typography>
                </Box>
                <Box display="flex" alignItems="center" mb={1}>
                  <Email color="action" sx={{ mr: 1 }} />
                  <Typography variant="body2">{user.email}</Typography>
                </Box>
                {user.phone && (
                  <Box display="flex" alignItems="center">
                    <Phone color="action" sx={{ mr: 1 }} />
                    <Typography variant="body2">{user.phone}</Typography>
                  </Box>
                )}
              </CardContent>
            </Card>
          </Grid>
        ))}
      </Grid>
    </div>
  );
};

export default UserList;`
}

// generateReactAPIService generates API service for React
func (g *TestProjectGenerator) generateReactAPIService() string {
	return `import axios from 'axios';
import { User, CreateUserRequest, UpdateUserRequest } from '../types/api';

const API_BASE_URL = process.env.REACT_APP_API_URL || 'http://localhost:8080';

const apiClient = axios.create({
  baseURL: API_BASE_URL,
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Request interceptor for auth
apiClient.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('authToken');
    if (token) {
      config.headers.Authorization = 'Bearer ' + token;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// Response interceptor for error handling
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('authToken');
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

export const fetchUsers = async (): Promise<User[]> => {
  const response = await apiClient.get<User[]>('/api/users');
  return response.data;
};

export const fetchUser = async (id: number): Promise<User> => {
  const response = await apiClient.get<User>('/api/users/' + id);
  return response.data;
};

export const createUser = async (userData: CreateUserRequest): Promise<User> => {
  const response = await apiClient.post<User>('/api/users', userData);
  return response.data;
};

export const updateUser = async (id: number, userData: UpdateUserRequest): Promise<User> => {
  const response = await apiClient.put<User>('/api/users/' + id, userData);
  return response.data;
};

export const deleteUser = async (id: number): Promise<void> => {
  await apiClient.delete('/api/users/' + id);
};`
}

// generateReactAPITypes generates TypeScript types for API
func (g *TestProjectGenerator) generateReactAPITypes() string {
	return `export interface User {
  id: number;
  name: string;
  email: string;
  phone?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateUserRequest {
  name: string;
  email: string;
  phone?: string;
}

export interface UpdateUserRequest {
  name?: string;
  email?: string;
  phone?: string;
}

export interface APIResponse<T> {
  data: T;
  message: string;
  status: string;
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
}

export interface ErrorResponse {
  error: string;
  message: string;
  details?: Record<string, string>;
}`
}

// generateReactIndexHTML generates public/index.html
func (g *TestProjectGenerator) generateReactIndexHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <link rel="icon" href="%PUBLIC_URL%/favicon.ico" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <meta name="theme-color" content="#000000" />
    <meta name="description" content="Monorepo WebApp - Full Stack Application" />
    <title>Monorepo WebApp</title>
  </head>
  <body>
    <noscript>You need to enable JavaScript to run this app.</noscript>
    <div id="root"></div>
  </body>
</html>`
}

// generateReactEnvExample generates .env.example
func (g *TestProjectGenerator) generateReactEnvExample() string {
	return `# API Configuration
REACT_APP_API_URL=http://localhost:8080

# Feature Flags
REACT_APP_ENABLE_ANALYTICS=true
REACT_APP_ENABLE_DEBUG=false

# Third-party Services
REACT_APP_SENTRY_DSN=
REACT_APP_GOOGLE_ANALYTICS_ID=`
}

// generateGoBackendMod generates go.mod for Go backend
func (g *TestProjectGenerator) generateGoBackendMod(projectName string) string {
	return fmt.Sprintf(`module %s-backend

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/lib/pq v1.10.7
	github.com/redis/go-redis/v9 v9.0.2
	github.com/golang-migrate/migrate/v4 v4.15.2
	github.com/joho/godotenv v1.4.0
	github.com/stretchr/testify v1.8.2
)

require (
	github.com/bytedance/sonic v1.8.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/chenzhuoyu/base64x v0.0.0-20221115062448-fe3a3abad311 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/gabriel-vasile/mimetype v1.4.2 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.11.2 // indirect
	github.com/goccy/go-json v0.10.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	github.com/leodido/go-urn v1.2.1 // indirect
	github.com/mattn/go-isatty v0.0.17 // indirect
	github.com/modern-go/concurrent v0.0.0-20180228061459-e0a39a4cb421 // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.0.6 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.9 // indirect
	golang.org/x/arch v0.0.0-20210923205945-b76863e36670 // indirect
	golang.org/x/crypto v0.5.0 // indirect
	golang.org/x/net v0.7.0 // indirect
	golang.org/x/sys v0.5.0 // indirect
	golang.org/x/text v0.7.0 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
`, projectName)
}

// generateGoBackendMain generates main.go for Go backend
func (g *TestProjectGenerator) generateGoBackendMain() string {
	return `package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"./config"
	"./internal/database"
	"./internal/handlers"
	"./pkg/middleware"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Initialize configuration
	cfg := config.Load()

	// Initialize database
	db, err := database.Initialize(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()

	// Initialize Redis
	redis := database.InitializeRedis(cfg.RedisURL)
	defer redis.Close()

	// Set up Gin router
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	// Add middleware
	router.Use(middleware.CORS())
	router.Use(middleware.Logger())
	router.Use(middleware.ErrorHandler())

	// Add routes
	api := router.Group("/api")
	{
		userHandler := handlers.NewUserHandler(db, redis)
		api.GET("/users", userHandler.GetUsers)
		api.GET("/users/:id", userHandler.GetUser)
		api.POST("/users", userHandler.CreateUser)
		api.PUT("/users/:id", userHandler.UpdateUser)
		api.DELETE("/users/:id", userHandler.DeleteUser)
	}

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})

	log.Printf("Server starting on port %s", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}`
}

// generateGoUserAPIHandler generates user API handler
func (g *TestProjectGenerator) generateGoUserAPIHandler() string {
	return `package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"database/sql"

	"../models"
	"../services"
)

type UserHandler struct {
	userService *services.UserService
}

func NewUserHandler(db *sql.DB, redis *redis.Client) *UserHandler {
	return &UserHandler{
		userService: services.NewUserService(db, redis),
	}
}

func (h *UserHandler) GetUsers(c *gin.Context) {
	users, err := h.userService.GetAllUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, users)
}

func (h *UserHandler) GetUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	user, err := h.userService.GetUserByID(c.Request.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) CreateUser(c *gin.Context) {
	var req models.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.userService.CreateUser(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, user)
}

func (h *UserHandler) UpdateUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req models.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.userService.UpdateUser(c.Request.Context(), id, &req)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) DeleteUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	err = h.userService.DeleteUser(c.Request.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}`
}

// generateGoUserBusinessService generates user business service
func (g *TestProjectGenerator) generateGoUserBusinessService() string {
	return `package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"../models"
)

type UserService struct {
	db    *sql.DB
	redis *redis.Client
}

func NewUserService(db *sql.DB, redis *redis.Client) *UserService {
	return &UserService{
		db:    db,
		redis: redis,
	}
}

func (s *UserService) GetAllUsers(ctx context.Context) ([]*models.User, error) {
	// Try to get from cache first
	cacheKey := "users:all"
	cached, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		var users []*models.User
		if err := json.Unmarshal([]byte(cached), &users); err == nil {
			return users, nil
		}
	}

	// Get from database
	query := "SELECT id, name, email, phone, created_at, updated_at FROM users ORDER BY created_at DESC"
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		user := &models.User{}
		err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.Phone, &user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	// Cache the result
	if data, err := json.Marshal(users); err == nil {
		s.redis.Set(ctx, cacheKey, data, 5*time.Minute)
	}

	return users, nil
}

func (s *UserService) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("user:%d", id)
	cached, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		var user models.User
		if err := json.Unmarshal([]byte(cached), &user); err == nil {
			return &user, nil
		}
	}

	// Get from database
	query := "SELECT id, name, email, phone, created_at, updated_at FROM users WHERE id = $1"
	user := &models.User{}
	err = s.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Name, &user.Email, &user.Phone, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if data, err := json.Marshal(user); err == nil {
		s.redis.Set(ctx, cacheKey, data, 10*time.Minute)
	}

	return user, nil
}

func (s *UserService) CreateUser(ctx context.Context, req *models.CreateUserRequest) (*models.User, error) {
	query := "INSERT INTO users (name, email, phone) VALUES ($1, $2, $3) RETURNING id, created_at, updated_at"
	user := &models.User{
		Name:  req.Name,
		Email: req.Email,
		Phone: req.Phone,
	}

	err := s.db.QueryRowContext(ctx, query, user.Name, user.Email, user.Phone).Scan(
		&user.ID, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Invalidate cache
	s.redis.Del(ctx, "users:all")

	return user, nil
}

func (s *UserService) UpdateUser(ctx context.Context, id int64, req *models.UpdateUserRequest) (*models.User, error) {
	query := "UPDATE users SET name = COALESCE($2, name), email = COALESCE($3, email), phone = COALESCE($4, phone), updated_at = NOW() WHERE id = $1 RETURNING name, email, phone, created_at, updated_at"
	user := &models.User{ID: id}

	err := s.db.QueryRowContext(ctx, query, id, req.Name, req.Email, req.Phone).Scan(
		&user.Name, &user.Email, &user.Phone, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Invalidate cache
	s.redis.Del(ctx, "users:all", fmt.Sprintf("user:%d", id))

	return user, nil
}

func (s *UserService) DeleteUser(ctx context.Context, id int64) error {
	query := "DELETE FROM users WHERE id = $1"
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	// Invalidate cache
	s.redis.Del(ctx, "users:all", fmt.Sprintf("user:%d", id))

	return nil
}`
}

// generateGoUserDomainModel generates user domain model
func (g *TestProjectGenerator) generateGoUserDomainModel() string {
	return `package models

import (
	"time"
)

type User struct {
	ID        int64     'json:"id" db:"id"'
	Name      string    'json:"name" db:"name"'
	Email     string    'json:"email" db:"email"'
	Phone     *string   'json:"phone,omitempty" db:"phone"'
	CreatedAt time.Time 'json:"created_at" db:"created_at"'
	UpdatedAt time.Time 'json:"updated_at" db:"updated_at"'
}

type CreateUserRequest struct {
	Name  string  'json:"name" binding:"required,min=2,max=100"'
	Email string  'json:"email" binding:"required,email"'
	Phone *string 'json:"phone,omitempty" binding:"omitempty,min=10,max=20"'
}

type UpdateUserRequest struct {
	Name  *string 'json:"name,omitempty" binding:"omitempty,min=2,max=100"'
	Email *string 'json:"email,omitempty" binding:"omitempty,email"'
	Phone *string 'json:"phone,omitempty" binding:"omitempty,min=10,max=20"'
}`
}

// generateGoPostgresConnection generates PostgreSQL connection
func (g *TestProjectGenerator) generateGoPostgresConnection() string {
	return `package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/redis/go-redis/v9"
)

func Initialize(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Run migrations
	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

func InitializeRedis(redisURL string) *redis.Client {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		panic(fmt.Sprintf("failed to parse Redis URL: %v", err))
	}

	client := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		panic(fmt.Sprintf("failed to connect to Redis: %v", err))
	}

	return client
}

func runMigrations(db *sql.DB) error {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations",
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}`
}

// generateGoCORSMiddleware generates CORS middleware
func (g *TestProjectGenerator) generateGoCORSMiddleware() string {
	return `package middleware

import (
	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})
}`
}

// generateGoAuthMiddleware generates auth middleware
func (g *TestProjectGenerator) generateGoAuthMiddleware() string {
	return `package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func Logger() gin.HandlerFunc {
	return gin.Logger()
}

func ErrorHandler() gin.HandlerFunc {
	return gin.Recovery()
}

func AuthRequired() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Extract token from Bearer format
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format"})
			c.Abort()
			return
		}

		token := tokenParts[1]
		
		// Validate token (implement your token validation logic here)
		if !validateToken(token) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		c.Next()
	})
}

func validateToken(token string) bool {
	// Implement your token validation logic here
	// This is a placeholder implementation
	return token != ""
}`
}

// generateGoConfig generates configuration
func (g *TestProjectGenerator) generateGoConfig() string {
	return `package config

import (
	"os"
)

type Config struct {
	Environment string
	Port        string
	DatabaseURL string
	RedisURL    string
}

func Load() *Config {
	return &Config{
		Environment: getEnv("ENVIRONMENT", "development"),
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://webapp_user:webapp_pass@localhost:5432/webapp_db?sslmode=disable"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6379"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}`
}

// generateGoDockerfile generates Dockerfile for Go backend
func (g *TestProjectGenerator) generateGoDockerfile() string {
	return `FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/main .
COPY --from=builder /app/migrations ./migrations

EXPOSE 8080
CMD ["./main"]`
}

// ==================== MICROSERVICES SUITE HELPER METHODS ====================

// generateMicroservicesDockerCompose generates docker-compose for microservices
func (g *TestProjectGenerator) generateMicroservicesDockerCompose(serviceCount int) string {
	return `version: '3.8'

services:
  # Infrastructure Services
  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: microservices_db
      POSTGRES_USER: microservices_user
      POSTGRES_PASSWORD: microservices_pass
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

  kafka:
    image: confluentinc/cp-kafka:latest
    environment:
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://localhost:9092
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
    ports:
      - "9092:9092"
    depends_on:
      - zookeeper

  zookeeper:
    image: confluentinc/cp-zookeeper:latest
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181
      ZOOKEEPER_TICK_TIME: 2000
    ports:
      - "2181:2181"

  # Application Services
  api-gateway:
    build:
      context: ./services/api-gateway
      dockerfile: Dockerfile
    ports:
      - "8080:8080"
    environment:
      - NODE_ENV=development
      - REDIS_URL=redis://redis:6379
    depends_on:
      - redis
      - user-service
      - order-service
      - payment-service

  user-service:
    build:
      context: ./services/user-service
      dockerfile: Dockerfile
    ports:
      - "8081:8081"
    environment:
      - DATABASE_URL=postgres://microservices_user:microservices_pass@postgres:5432/microservices_db
      - REDIS_URL=redis://redis:6379
    depends_on:
      - postgres
      - redis

  order-service:
    build:
      context: ./services/order-service
      dockerfile: Dockerfile
    ports:
      - "8082:8082"
    environment:
      - DATABASE_URL=postgres://microservices_user:microservices_pass@postgres:5432/microservices_db
      - KAFKA_BROKERS=kafka:9092
    depends_on:
      - postgres
      - kafka

  payment-service:
    build:
      context: ./services/payment-service
      dockerfile: Dockerfile
    ports:
      - "8083:8083"
    environment:
      - DATABASE_URL=postgres://microservices_user:microservices_pass@postgres:5432/microservices_db
    depends_on:
      - postgres

  notification-service:
    build:
      context: ./services/notification-service
      dockerfile: Dockerfile
    ports:
      - "8084:8084"
    environment:
      - REDIS_URL=redis://redis:6379
      - KAFKA_BROKERS=kafka:9092
    depends_on:
      - redis
      - kafka

  analytics-service:
    build:
      context: ./services/analytics-service
      dockerfile: Dockerfile
    ports:
      - "8085:8085"
    environment:
      - DATABASE_URL=postgres://microservices_user:microservices_pass@postgres:5432/microservices_db
    depends_on:
      - postgres

  # Monitoring
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - grafana_data:/var/lib/grafana

volumes:
  postgres_data:
  redis_data:
  grafana_data:`
}

// generateMicroservicesMakefile generates Makefile for microservices
func (g *TestProjectGenerator) generateMicroservicesMakefile() string {
	return `# Microservices Suite Makefile

.PHONY: build start stop test clean logs

# Build all services
build:
	docker-compose build

# Start all services
start:
	docker-compose up -d
	@echo "Services starting..."
	@echo "API Gateway: http://localhost:8080"
	@echo "Grafana: http://localhost:3000 (admin/admin)"
	@echo "Prometheus: http://localhost:9090"

# Stop all services
stop:
	docker-compose down

# Run tests for all services
test:
	cd services/api-gateway && npm test
	cd services/user-service && go test ./...
	cd services/order-service && mvn test
	cd services/payment-service && python -m pytest
	cd services/notification-service && cargo test
	cd services/analytics-service && dotnet test

# Clean all services
clean:
	docker-compose down -v
	docker system prune -f

# View logs
logs:
	docker-compose logs -f

# Setup development environment
setup:
	./scripts/setup-all.sh

# Health check all services
health:
	@echo "Checking service health..."
	@curl -f http://localhost:8080/health || echo "API Gateway: DOWN"
	@curl -f http://localhost:8081/health || echo "User Service: DOWN"
	@curl -f http://localhost:8082/health || echo "Order Service: DOWN"
	@curl -f http://localhost:8083/health || echo "Payment Service: DOWN"
	@curl -f http://localhost:8084/health || echo "Notification Service: DOWN"
	@curl -f http://localhost:8085/health || echo "Analytics Service: DOWN"`
}

// generateMicroservicesSetupScript generates setup script
func (g *TestProjectGenerator) generateMicroservicesSetupScript() string {
	return `#!/bin/bash

# Microservices Suite Setup Script

set -e

echo "Setting up Microservices Suite..."

# Check for required tools
command -v docker >/dev/null 2>&1 || { echo "Docker is required but not installed. Aborting." >&2; exit 1; }
command -v docker-compose >/dev/null 2>&1 || { echo "Docker Compose is required but not installed. Aborting." >&2; exit 1; }
command -v node >/dev/null 2>&1 || { echo "Node.js is required but not installed. Aborting." >&2; exit 1; }
command -v go >/dev/null 2>&1 || { echo "Go is required but not installed. Aborting." >&2; exit 1; }
command -v mvn >/dev/null 2>&1 || { echo "Maven is required but not installed. Aborting." >&2; exit 1; }
command -v python >/dev/null 2>&1 || { echo "Python is required but not installed. Aborting." >&2; exit 1; }
command -v cargo >/dev/null 2>&1 || { echo "Rust/Cargo is required but not installed. Aborting." >&2; exit 1; }
command -v dotnet >/dev/null 2>&1 || { echo ".NET is required but not installed. Aborting." >&2; exit 1; }

# Install dependencies for each service
echo "Installing dependencies..."

# API Gateway (Node.js/TypeScript)
cd services/api-gateway
npm install
cd ../..

# User Service (Go)
cd services/user-service
go mod download
cd ../..

# Order Service (Java/Spring Boot)
cd services/order-service
mvn dependency:resolve
cd ../..

# Payment Service (Python/FastAPI)
cd services/payment-service
pip install -r requirements.txt
cd ../..

# Notification Service (Rust)
cd services/notification-service
cargo fetch
cd ../..

# Analytics Service (C#/.NET)
cd services/analytics-service
dotnet restore
cd ../..

echo "Setup complete!"
echo "Run 'make start' to start all services"
echo "Run 'make health' to check service status"`
}

// generateAPIGatewayPackageJson generates package.json for API Gateway
func (g *TestProjectGenerator) generateAPIGatewayPackageJson() string {
	return `{
  "name": "api-gateway",
  "version": "1.0.0",
  "description": "API Gateway for Microservices Suite",
  "main": "dist/index.js",
  "scripts": {
    "start": "node dist/index.js",
    "dev": "ts-node-dev src/index.ts",
    "build": "tsc",
    "test": "jest",
    "lint": "eslint src --ext .ts",
    "docker:build": "docker build -t api-gateway .",
    "docker:run": "docker run -p 8080:8080 api-gateway"
  },
  "dependencies": {
    "express": "^4.18.2",
    "http-proxy-middleware": "^2.0.6",
    "redis": "^4.6.5",
    "express-rate-limit": "^6.7.0",
    "helmet": "^6.0.1",
    "cors": "^2.8.5",
    "jsonwebtoken": "^9.0.0",
    "morgan": "^1.10.0",
    "compression": "^1.7.4",
    "dotenv": "^16.0.3"
  },
  "devDependencies": {
    "@types/node": "^18.15.0",
    "@types/express": "^4.17.17",
    "@types/cors": "^2.8.13",
    "@types/morgan": "^1.9.4",
    "@types/compression": "^1.7.2",
    "@types/jsonwebtoken": "^9.0.1",
    "typescript": "^4.9.5",
    "ts-node-dev": "^2.0.0",
    "jest": "^29.5.0",
    "@types/jest": "^29.5.0",
    "eslint": "^8.36.0",
    "@typescript-eslint/eslint-plugin": "^5.55.0",
    "@typescript-eslint/parser": "^5.55.0"
  }
}`
}

// generateAPIGatewayMain generates main entry point for API Gateway
func (g *TestProjectGenerator) generateAPIGatewayMain() string {
	return `import express from 'express';
import cors from 'cors';
import helmet from 'helmet';
import compression from 'compression';
import morgan from 'morgan';
import dotenv from 'dotenv';
import { createClient } from 'redis';

import routes from './routes';
import { authMiddleware } from './middleware/auth';
import { rateLimitMiddleware } from './middleware/rate-limit';
import { errorHandler } from './middleware/error-handler';
import { serviceHealthCheck } from './health/service-health';

dotenv.config();

const app = express();
const PORT = process.env.PORT || 8080;

// Redis client for caching and rate limiting
export const redisClient = createClient({
  url: process.env.REDIS_URL || 'redis://localhost:6379'
});

async function startServer() {
  try {
    // Connect to Redis
    await redisClient.connect();
    console.log('Connected to Redis');

    // Middleware
    app.use(helmet());
    app.use(cors());
    app.use(compression());
    app.use(morgan('combined'));
    app.use(express.json({ limit: '10mb' }));
    app.use(express.urlencoded({ extended: true }));

    // Rate limiting
    app.use(rateLimitMiddleware);

    // Health check
    app.get('/health', (req, res) => {
      res.json({ 
        status: 'healthy', 
        timestamp: new Date().toISOString(),
        version: process.env.npm_package_version || '1.0.0'
      });
    });

    // Service health check
    app.get('/health/services', async (req, res) => {
      const healthStatus = await serviceHealthCheck();
      res.json(healthStatus);
    });

    // Authentication middleware for protected routes
    app.use('/api', authMiddleware);

    // Routes
    app.use('/api', routes);

    // Error handling
    app.use(errorHandler);

    // Start server
    app.listen(PORT, () => {
      console.log('API Gateway started on port', PORT);
      console.log('Environment:', process.env.NODE_ENV || 'development');
    });

  } catch (error) {
    console.error('Failed to start server:', error);
    process.exit(1);
  }
}

// Graceful shutdown
process.on('SIGTERM', async () => {
  console.log('SIGTERM received, shutting down gracefully');
  await redisClient.quit();
  process.exit(0);
});

process.on('SIGINT', async () => {
  console.log('SIGINT received, shutting down gracefully');
  await redisClient.quit();
  process.exit(0);
});

startServer();`
}

// generateUserServiceGoMod generates go.mod for User Service
func (g *TestProjectGenerator) generateUserServiceGoMod() string {
	return `module user-service

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/lib/pq v1.10.7
	github.com/redis/go-redis/v9 v9.0.2
	google.golang.org/grpc v1.53.0
	google.golang.org/protobuf v1.28.1
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.15.2
	github.com/golang/protobuf v1.5.2
	github.com/stretchr/testify v1.8.2
	github.com/joho/godotenv v1.4.0
)

require (
	github.com/bytedance/sonic v1.8.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/chenzhuoyu/base64x v0.0.0-20221115062448-fe3a3abad311 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/gabriel-vasile/mimetype v1.4.2 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.11.2 // indirect
	github.com/goccy/go-json v0.10.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	github.com/leodido/go-urn v1.2.1 // indirect
	github.com/mattn/go-isatty v0.0.17 // indirect
	github.com/modern-go/concurrent v0.0.0-20180228061459-e0a39a4cb421 // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.0.6 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.9 // indirect
	golang.org/x/arch v0.0.0-20210923205945-b76863e36670 // indirect
	golang.org/x/crypto v0.5.0 // indirect
	golang.org/x/net v0.7.0 // indirect
	golang.org/x/sys v0.5.0 // indirect
	golang.org/x/text v0.7.0 // indirect
	google.golang.org/genproto v0.0.0-20230223222841-637eb2293923 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)`
}

// generateUserServiceMain generates main.go for User Service
func (g *TestProjectGenerator) generateUserServiceMain() string {
	return `package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"

	"./internal/handlers"
	"./internal/repository"
	"./internal/service"
	"./proto"
	"./config"
	"./internal/database"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	cfg := config.Load()

	// Initialize database
	db, err := database.Initialize(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()

	// Initialize Redis
	redis := database.InitializeRedis(cfg.RedisURL)
	defer redis.Close()

	// Initialize repository and service
	userRepo := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepo, redis)

	// Start gRPC server
	go startGRPCServer(userService, cfg.GRPCPort)

	// Start HTTP server
	startHTTPServer(userService, cfg.HTTPPort)
}

func startGRPCServer(userService *service.UserService, port string) {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatal("Failed to listen for gRPC:", err)
	}

	grpcServer := grpc.NewServer()
	proto.RegisterUserServiceServer(grpcServer, handlers.NewGRPCUserHandler(userService))

	log.Printf("gRPC server listening on port %s", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal("Failed to serve gRPC:", err)
	}
}

func startHTTPServer(userService *service.UserService, port string) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// Middleware
	router.Use(gin.Recovery())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "user-service",
			"timestamp": time.Now().Unix(),
		})
	})

	// HTTP handlers
	httpHandler := handlers.NewHTTPUserHandler(userService)
	api := router.Group("/api/v1")
	{
		api.GET("/users", httpHandler.GetUsers)
		api.GET("/users/:id", httpHandler.GetUser)
		api.POST("/users", httpHandler.CreateUser)
		api.PUT("/users/:id", httpHandler.UpdateUser)
		api.DELETE("/users/:id", httpHandler.DeleteUser)
	}

	// Start server with graceful shutdown
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	go func() {
		log.Printf("HTTP server listening on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start HTTP server:", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exited")
}`
}

// ==================== PYTHON WITH GO EXTENSIONS HELPER METHODS ====================

// generatePythonGoExtensionsPyproject generates pyproject.toml for Python-Go project
func (g *TestProjectGenerator) generatePythonGoExtensionsPyproject(projectName string) string {
	return fmt.Sprintf(`[project]
name = "%s"
version = "1.0.0"
description = "Python application with Go performance extensions"
authors = [
    {name = "Developer", email = "dev@example.com"}
]
readme = "README.md"
license = {text = "MIT"}
requires-python = ">=3.8"
classifiers = [
    "Development Status :: 4 - Beta",
    "Intended Audience :: Developers",
    "License :: OSI Approved :: MIT License",
    "Programming Language :: Python :: 3",
    "Programming Language :: Python :: 3.8",
    "Programming Language :: Python :: 3.9",
    "Programming Language :: Python :: 3.10",
    "Programming Language :: Python :: 3.11",
]

dependencies = [
    "fastapi>=0.95.0",
    "uvicorn[standard]>=0.21.0",
    "pydantic>=1.10.0",
    "requests>=2.28.0",
    "numpy>=1.24.0",
    "pandas>=2.0.0",
    "psutil>=5.9.0",
    "redis>=4.5.0",
    "sqlalchemy>=2.0.0",
    "alembic>=1.10.0",
]

[project.optional-dependencies]
dev = [
    "pytest>=7.2.0",
    "pytest-asyncio>=0.21.0",
    "pytest-benchmark>=4.0.0",
    "black>=23.1.0",
    "ruff>=0.0.254",
    "mypy>=1.1.0",
    "coverage>=7.2.0",
]

[project.scripts]
run-server = "%s.main:main"
benchmark = "%s.benchmarks:run_benchmarks"

[build-system]
requires = ["setuptools>=65.0", "wheel"]
build-backend = "setuptools.build_meta"

[tool.setuptools.packages.find]
where = ["src"]

[tool.black]
line-length = 88
target-version = ['py38']

[tool.ruff]
line-length = 88
target-version = "py38"

[tool.mypy]
python_version = "3.8"
warn_return_any = true
warn_unused_configs = true
disallow_untyped_defs = true

[tool.pytest.ini_options]
testpaths = ["tests"]
python_files = ["test_*.py"]
python_classes = ["Test*"]
python_functions = ["test_*"]
addopts = "-v --tb=short"
`, projectName, projectName, projectName)
}

// generatePythonGoExtensionsSetup generates setup.py for Python-Go project
func (g *TestProjectGenerator) generatePythonGoExtensionsSetup(projectName string) string {
	return fmt.Sprintf(`#!/usr/bin/env python3

import os
import subprocess
import sys
from setuptools import setup, find_packages, Extension
from setuptools.command.build_ext import build_ext


class GoExtension(Extension):
    def __init__(self, name, sourcedir=''):
        Extension.__init__(self, name, sources=[])
        self.sourcedir = os.path.abspath(sourcedir)


class BuildGoExt(build_ext):
    def run(self):
        try:
            subprocess.check_output(['go', 'version'])
        except OSError:
            raise RuntimeError("Go must be installed to build Go extensions")

        for ext in self.extensions:
            self.build_go_extension(ext)

    def build_go_extension(self, ext):
        extdir = os.path.abspath(os.path.dirname(self.get_ext_fullpath(ext.name)))
        
        # Build Go shared library
        go_build_cmd = [
            'go', 'build',
            '-buildmode=c-shared',
            '-o', os.path.join(extdir, 'extensions', ext.name + '.so'),
            './extensions/' + ext.name.split('.')[-1]
        ]
        
        subprocess.check_call(go_build_cmd, cwd=self.build_temp or '.')


if __name__ == '__main__':
    setup(
        name='%s',
        version='1.0.0',
        author='Developer',
        author_email='dev@example.com',
        description='Python application with Go performance extensions',
        long_description=open('README.md').read() if os.path.exists('README.md') else '',
        long_description_content_type='text/markdown',
        url='https://github.com/example/%s',
        packages=find_packages(where='src'),
        package_dir={'': 'src'},
        ext_modules=[
            GoExtension('extensions.calculator'),
            GoExtension('extensions.parser'),
            GoExtension('extensions.crypto'),
            GoExtension('extensions.compression'),
        ],
        cmdclass={
            'build_ext': BuildGoExt,
        },
        python_requires='>=3.8',
        install_requires=[
            'fastapi>=0.95.0',
            'uvicorn[standard]>=0.21.0',
            'pydantic>=1.10.0',
            'requests>=2.28.0',
            'numpy>=1.24.0',
            'pandas>=2.0.0',
            'psutil>=5.9.0',
            'redis>=4.5.0',
            'sqlalchemy>=2.0.0',
            'alembic>=1.10.0',
        ],
        extras_require={
            'dev': [
                'pytest>=7.2.0',
                'pytest-asyncio>=0.21.0',
                'pytest-benchmark>=4.0.0',
                'black>=23.1.0',
                'ruff>=0.0.254',
                'mypy>=1.1.0',
                'coverage>=7.2.0',
            ],
        },
        classifiers=[
            'Development Status :: 4 - Beta',
            'Intended Audience :: Developers',
            'License :: OSI Approved :: MIT License',
            'Programming Language :: Python :: 3',
            'Programming Language :: Python :: 3.8',
            'Programming Language :: Python :: 3.9',
            'Programming Language :: Python :: 3.10',
            'Programming Language :: Python :: 3.11',
        ],
    )
`, projectName, projectName)
}

// generatePythonGoExtensionsRequirements generates requirements.txt
func (g *TestProjectGenerator) generatePythonGoExtensionsRequirements() string {
	return `# Core dependencies
fastapi>=0.95.0
uvicorn[standard]>=0.21.0
pydantic>=1.10.0
requests>=2.28.0

# Data processing
numpy>=1.24.0
pandas>=2.0.0

# System utilities
psutil>=5.9.0

# Caching and databases
redis>=4.5.0
sqlalchemy>=2.0.0
alembic>=1.10.0

# Additional utilities
python-multipart>=0.0.6
python-jose[cryptography]>=3.3.0
passlib[bcrypt]>=1.7.4
python-dotenv>=1.0.0`
}

// generatePythonGoExtensionsDevRequirements generates requirements-dev.txt
func (g *TestProjectGenerator) generatePythonGoExtensionsDevRequirements() string {
	return `# Testing
pytest>=7.2.0
pytest-asyncio>=0.21.0
pytest-benchmark>=4.0.0
pytest-cov>=4.0.0
pytest-mock>=3.10.0

# Code quality
black>=23.1.0
ruff>=0.0.254
mypy>=1.1.0
pre-commit>=3.2.0

# Documentation
sphinx>=6.1.0
sphinx-rtd-theme>=1.2.0

# Development tools
ipython>=8.11.0
jupyter>=1.0.0
memory-profiler>=0.60.0
line-profiler>=4.0.0

# HTTP testing
httpx>=0.24.0`
}

// generatePythonGoExtensionsMain generates main.py
func (g *TestProjectGenerator) generatePythonGoExtensionsMain() string {
	return `#!/usr/bin/env python3

import asyncio
import logging
import time
from contextlib import asynccontextmanager
from typing import Dict, Any

import uvicorn
from fastapi import FastAPI, HTTPException, Depends
from fastapi.middleware.cors import CORSMiddleware
from fastapi.middleware.gzip import GZipMiddleware

from app.core.engine import ProcessingEngine
from app.api.routes import router as api_router
from app.utils.performance import PerformanceMonitor
from extensions import go_calculator, go_parser, go_crypto, go_compression

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s"
)
logger = logging.getLogger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Manage application lifespan."""
    # Startup
    logger.info("Starting Python-Go Extensions Application")
    
    # Initialize performance monitor
    monitor = PerformanceMonitor()
    monitor.start()
    
    # Initialize processing engine
    engine = ProcessingEngine()
    await engine.initialize()
    
    # Store in app state
    app.state.monitor = monitor
    app.state.engine = engine
    
    # Warm up Go extensions
    try:
        # Test calculator extension
        result = go_calculator.add(10, 20)
        logger.info(f"Calculator extension test: 10 + 20 = {result}")
        
        # Test parser extension
        parsed = go_parser.parse_json('{"test": true}')
        logger.info(f"Parser extension test: {parsed}")
        
        # Test crypto extension
        hash_result = go_crypto.hash_sha256("test string")
        logger.info(f"Crypto extension test: {hash_result[:16]}...")
        
        # Test compression extension
        compressed = go_compression.compress("test data for compression")
        logger.info(f"Compression extension test: {len(compressed)} bytes")
        
    except Exception as e:
        logger.warning(f"Go extension warmup failed: {e}")
    
    yield
    
    # Shutdown
    logger.info("Shutting down Python-Go Extensions Application")
    monitor.stop()
    await engine.cleanup()


def create_app() -> FastAPI:
    """Create and configure FastAPI application."""
    app = FastAPI(
        title="Python-Go Extensions Application",
        description="High-performance Python app with Go extensions",
        version="1.0.0",
        lifespan=lifespan
    )
    
    # Middleware
    app.add_middleware(
        CORSMiddleware,
        allow_origins=["*"],
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )
    app.add_middleware(GZipMiddleware, minimum_size=1000)
    
    # Routes
    app.include_router(api_router, prefix="/api/v1")
    
    @app.get("/")
    async def root():
        return {
            "message": "Python-Go Extensions Application",
            "version": "1.0.0",
            "extensions": ["calculator", "parser", "crypto", "compression"]
        }
    
    @app.get("/health")
    async def health_check():
        return {
            "status": "healthy",
            "timestamp": time.time(),
            "extensions_available": True
        }
    
    @app.get("/performance")
    async def performance_stats(monitor: PerformanceMonitor = Depends(lambda: app.state.monitor)):
        return monitor.get_stats()
    
    return app


def main():
    """Main entry point."""
    app = create_app()
    
    uvicorn.run(
        app,
        host="0.0.0.0",
        port=8000,
        log_level="info",
        access_log=True,
        reload=False,  # Disable in production
    )


if __name__ == "__main__":
    main()`
}

// generatePythonCoreEngine generates core engine
func (g *TestProjectGenerator) generatePythonCoreEngine() string {
	return `import asyncio
import logging
from typing import Dict, List, Any, Optional
from datetime import datetime
import time

import numpy as np
import pandas as pd
from sqlalchemy.ext.asyncio import AsyncSession, create_async_engine
from redis.asyncio import Redis

from ..models.data import ProcessingJob, ProcessingResult
from ..services.calculation import CalculationService
from extensions import go_calculator, go_parser, go_crypto

logger = logging.getLogger(__name__)


class ProcessingEngine:
    """High-performance processing engine with Go extensions."""
    
    def __init__(self):
        self.engine = None
        self.redis: Optional[Redis] = None
        self.calculation_service = CalculationService()
        self.is_running = False
        self.stats = {
            "jobs_processed": 0,
            "avg_processing_time": 0.0,
            "go_extension_calls": 0,
            "python_processing_calls": 0
        }
    
    async def initialize(self):
        """Initialize the processing engine."""
        logger.info("Initializing processing engine...")
        
        # Initialize database
        self.engine = create_async_engine(
            "sqlite+aiosqlite:///./app.db",
            echo=False
        )
        
        # Initialize Redis
        self.redis = Redis.from_url("redis://localhost:6379", decode_responses=True)
        
        # Test connections
        try:
            await self.redis.ping()
            logger.info("Redis connection established")
        except Exception as e:
            logger.warning(f"Redis connection failed: {e}")
        
        self.is_running = True
        logger.info("Processing engine initialized successfully")
    
    async def cleanup(self):
        """Cleanup resources."""
        self.is_running = False
        
        if self.redis:
            await self.redis.close()
        
        if self.engine:
            await self.engine.dispose()
        
        logger.info("Processing engine cleaned up")
    
    async def process_data(self, data: Dict[str, Any], use_go_extensions: bool = True) -> ProcessingResult:
        """Process data using Python and Go extensions."""
        start_time = time.time()
        
        try:
            job = ProcessingJob(
                id=str(int(time.time() * 1000)),
                data=data,
                created_at=datetime.utcnow(),
                use_go_extensions=use_go_extensions
            )
            
            if use_go_extensions:
                result = await self._process_with_go_extensions(job)
                self.stats["go_extension_calls"] += 1
            else:
                result = await self._process_with_python(job)
                self.stats["python_processing_calls"] += 1
            
            # Update statistics
            processing_time = time.time() - start_time
            self.stats["jobs_processed"] += 1
            self.stats["avg_processing_time"] = (
                (self.stats["avg_processing_time"] * (self.stats["jobs_processed"] - 1) + processing_time) /
                self.stats["jobs_processed"]
            )
            
            return result
            
        except Exception as e:
            logger.error(f"Processing failed: {e}")
            raise
    
    async def _process_with_go_extensions(self, job: ProcessingJob) -> ProcessingResult:
        """Process using Go extensions for performance-critical operations."""
        data = job.data
        results = {}
        
        # Numerical calculations using Go
        if "numbers" in data:
            numbers = data["numbers"]
            if isinstance(numbers, list) and len(numbers) >= 2:
                # Use Go calculator for basic operations
                results["sum"] = go_calculator.sum_array(numbers)
                results["mean"] = go_calculator.calculate_mean(numbers)
                results["std"] = go_calculator.calculate_std(numbers)
        
        # JSON parsing using Go
        if "json_data" in data:
            try:
                parsed = go_parser.parse_json(str(data["json_data"]))
                results["parsed_json"] = parsed
            except Exception as e:
                logger.warning(f"Go JSON parsing failed: {e}")
                results["parsed_json"] = data["json_data"]
        
        # Hashing using Go crypto
        if "hash_input" in data:
            hash_result = go_crypto.hash_sha256(str(data["hash_input"]))
            results["hash"] = hash_result
        
        # Complex analysis using Python + NumPy
        if "matrix_data" in data:
            matrix = np.array(data["matrix_data"])
            results["matrix_analysis"] = {
                "shape": matrix.shape,
                "dtype": str(matrix.dtype),
                "mean": float(np.mean(matrix)),
                "std": float(np.std(matrix)),
                "eigenvalues": np.linalg.eigvals(matrix).tolist() if matrix.ndim == 2 and matrix.shape[0] == matrix.shape[1] else None
            }
        
        return ProcessingResult(
            job_id=job.id,
            results=results,
            processing_time=time.time() - time.time(),
            method="go_extensions",
            success=True
        )
    
    async def _process_with_python(self, job: ProcessingJob) -> ProcessingResult:
        """Process using pure Python implementations."""
        data = job.data
        results = {}
        
        # Numerical calculations using Python
        if "numbers" in data:
            numbers = data["numbers"]
            if isinstance(numbers, list) and len(numbers) >= 2:
                results["sum"] = sum(numbers)
                results["mean"] = sum(numbers) / len(numbers)
                results["std"] = (sum((x - results["mean"]) ** 2 for x in numbers) / len(numbers)) ** 0.5
        
        # JSON parsing using Python
        if "json_data" in data:
            import json
            try:
                if isinstance(data["json_data"], str):
                    parsed = json.loads(data["json_data"])
                else:
                    parsed = data["json_data"]
                results["parsed_json"] = parsed
            except Exception as e:
                logger.warning(f"Python JSON parsing failed: {e}")
                results["parsed_json"] = data["json_data"]
        
        # Hashing using Python
        if "hash_input" in data:
            import hashlib
            hash_result = hashlib.sha256(str(data["hash_input"]).encode()).hexdigest()
            results["hash"] = hash_result
        
        # Matrix analysis using NumPy
        if "matrix_data" in data:
            matrix = np.array(data["matrix_data"])
            results["matrix_analysis"] = {
                "shape": matrix.shape,
                "dtype": str(matrix.dtype),
                "mean": float(np.mean(matrix)),
                "std": float(np.std(matrix)),
                "eigenvalues": np.linalg.eigvals(matrix).tolist() if matrix.ndim == 2 and matrix.shape[0] == matrix.shape[1] else None
            }
        
        return ProcessingResult(
            job_id=job.id,
            results=results,
            processing_time=time.time() - time.time(),
            method="python",
            success=True
        )
    
    def get_stats(self) -> Dict[str, Any]:
        """Get processing statistics."""
        return {
            **self.stats,
            "is_running": self.is_running,
            "uptime": time.time()
        }`
}

// generateGoExtensionsMod generates go.mod for Go extensions
func (g *TestProjectGenerator) generateGoExtensionsMod(projectName string) string {
	return fmt.Sprintf(`module %s-extensions

go 1.21

require (
	github.com/json-iterator/go v1.1.12
	golang.org/x/crypto v0.8.0
)

require (
	github.com/modern-go/concurrent v0.0.0-20180228061459-e0a39a4cb421 // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
)
`, projectName)
}

// generateGoExtensionsMain generates main.go for Go extensions
func (g *TestProjectGenerator) generateGoExtensionsMain() string {
	return `package main

import "C"

// This is required for building shared libraries
func main() {}

//export test_connection
func test_connection() *C.char {
	return C.CString("Go extensions loaded successfully")
}
`
}

// generateGoCalculatorExtension generates Go calculator extension
func (g *TestProjectGenerator) generateGoCalculatorExtension() string {
	return `package main

import (
	"C"
	"math"
	"unsafe"
)

//export add
func add(a, b C.double) C.double {
	return C.double(float64(a) + float64(b))
}

//export multiply
func multiply(a, b C.double) C.double {
	return C.double(float64(a) * float64(b))
}

//export sum_array
func sum_array(arr *C.double, length C.int) C.double {
	slice := (*[1 << 30]C.double)(unsafe.Pointer(arr))[:length:length]
	var sum float64
	for i := 0; i < int(length); i++ {
		sum += float64(slice[i])
	}
	return C.double(sum)
}

//export calculate_mean
func calculate_mean(arr *C.double, length C.int) C.double {
	if length == 0 {
		return 0
	}
	sum := sum_array(arr, length)
	return C.double(float64(sum) / float64(length))
}

//export calculate_std
func calculate_std(arr *C.double, length C.int) C.double {
	if length <= 1 {
		return 0
	}
	
	slice := (*[1 << 30]C.double)(unsafe.Pointer(arr))[:length:length]
	mean := float64(calculate_mean(arr, length))
	
	var variance float64
	for i := 0; i < int(length); i++ {
		diff := float64(slice[i]) - mean
		variance += diff * diff
	}
	variance /= float64(length - 1)
	
	return C.double(math.Sqrt(variance))
}

//export factorial
func factorial(n C.int) C.longlong {
	if n <= 1 {
		return 1
	}
	
	result := int64(1)
	for i := 2; i <= int(n); i++ {
		result *= int64(i)
	}
	return C.longlong(result)
}

//export fibonacci
func fibonacci(n C.int) C.longlong {
	if n <= 1 {
		return C.longlong(n)
	}
	
	a, b := int64(0), int64(1)
	for i := 2; i <= int(n); i++ {
		a, b = b, a+b
	}
	return C.longlong(b)
}

func main() {}
`
}

// Missing Python method implementations

func (g *TestProjectGenerator) generatePythonDataProcessorMain() string {
	return "#!/usr/bin/env python3\n" +
		"\"\"\"Main entry point for data processor service.\"\"\"\n\n" +
		"def main():\n" +
		"    print(\"Data processor service started\")\n\n" +
		"if __name__ == \"__main__\":\n" +
		"    main()"
}

func (g *TestProjectGenerator) generatePythonUserAnalytics() string {
	return "\"\"\"User analytics processing module.\"\"\"\n\n" +
		"def process_user_analytics(user_data):\n" +
		"    \"\"\"Process user analytics data.\"\"\"\n" +
		"    return {\"processed\": True, \"user_count\": len(user_data)}"
}

func (g *TestProjectGenerator) generatePythonReportGenerator() string {
	return "\"\"\"Report generation module.\"\"\"\n\n" +
		"def generate_report(data):\n" +
		"    \"\"\"Generate reports from processed data.\"\"\"\n" +
		"    return {\"report\": \"Generated report\", \"timestamp\": \"2024-01-01\"}"
}

func (g *TestProjectGenerator) generatePythonDatabaseConnection() string {
	return "\"\"\"Database connection utilities.\"\"\"\n" +
		"import sqlalchemy as db\n\n" +
		"def get_connection():\n" +
		"    \"\"\"Get database connection.\"\"\"\n" +
		"    engine = db.create_engine(\"postgresql://user:pass@localhost/db\")\n" +
		"    return engine.connect()"
}

func (g *TestProjectGenerator) generatePythonAnalyticsModels() string {
	return "\"\"\"Analytics data models.\"\"\"\n\n" +
		"class UserAnalytics:\n" +
		"    def __init__(self, user_id, metrics):\n" +
		"        self.user_id = user_id\n" +
		"        self.metrics = metrics"
}

func (g *TestProjectGenerator) generatePythonDataTransformers() string {
	return "\"\"\"Data transformation utilities.\"\"\"\n\n" +
		"def transform_user_data(raw_data):\n" +
		"    \"\"\"Transform raw user data.\"\"\"\n" +
		"    return {\"transformed\": raw_data, \"processed_at\": \"2024-01-01\"}"
}

func (g *TestProjectGenerator) generatePythonDataProcessorRequirements() string {
	return "pandas>=1.5.0\n" +
		"numpy>=1.24.0\n" +
		"sqlalchemy>=1.4.0\n" +
		"psycopg2-binary>=2.9.0\n" +
		"pytest>=7.0.0"
}

func (g *TestProjectGenerator) generatePythonDockerfile() string {
	return "FROM python:3.11-slim\n\n" +
		"WORKDIR /app\n" +
		"COPY requirements.txt .\n" +
		"RUN pip install -r requirements.txt\n\n" +
		"COPY . .\n" +
		"CMD [\"python\", \"src/main.py\"]"
}

// Additional missing methods for shared utilities

func (g *TestProjectGenerator) generateSharedTypesPackageJson(projectName string) string {
	return fmt.Sprintf("{\"name\": \"%s-shared-types\", \"version\": \"1.0.0\", \"main\": \"index.ts\"}", projectName)
}

func (g *TestProjectGenerator) generateSharedTypesIndex() string {
	return "export * from './user';\nexport * from './api';\nexport * from './events';"
}

func (g *TestProjectGenerator) generateSharedUserTypes() string {
	return "export interface User { id: string; name: string; email: string; }"
}

func (g *TestProjectGenerator) generateSharedAPITypes() string {
	return "export interface APIResponse<T> { data: T; success: boolean; message?: string; }"
}

func (g *TestProjectGenerator) generateSharedEventTypes() string {
	return "export interface Event { id: string; type: string; timestamp: Date; payload: any; }"
}

func (g *TestProjectGenerator) generateSharedTypesTsConfig() string {
	return "{\"compilerOptions\": {\"target\": \"ES2020\", \"module\": \"commonjs\", \"declaration\": true}}"
}

func (g *TestProjectGenerator) generateSharedUtilsPackageJson(projectName string) string {
	return fmt.Sprintf("{\"name\": \"%s-shared-utils\", \"version\": \"1.0.0\", \"main\": \"index.ts\"}", projectName)
}

func (g *TestProjectGenerator) generateSharedUtilsIndex() string {
	return "export * from './validation';\nexport * from './formatting';"
}

func (g *TestProjectGenerator) generateSharedValidationUtils() string {
	return "export const validateEmail = (email: string): boolean => /\\S+@\\S+\\.\\S+/.test(email);"
}

func (g *TestProjectGenerator) generateSharedFormattingUtils() string {
	return "export const formatDate = (date: Date): string => date.toISOString().split('T')[0];"
}

func (g *TestProjectGenerator) generateSharedConstants() string {
	return "export const API_BASE_URL = 'http://localhost:3000/api';\nexport const DEFAULT_TIMEOUT = 5000;"
}

func (g *TestProjectGenerator) generateMonorepoGitHubActions() string {
	return "name: CI\non: [push, pull_request]\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@v2"
}

func (g *TestProjectGenerator) generateDevDockerCompose() string {
	return "version: '3.8'\nservices:\n  app:\n    build: .\n    ports:\n      - '3000:3000'\n    volumes:\n      - .:/app"
}

func (g *TestProjectGenerator) generateProdDockerCompose() string {
	return "version: '3.8'\nservices:\n  app:\n    build:\n      context: .\n      dockerfile: Dockerfile.prod\n    ports:\n      - '80:80'"
}

func (g *TestProjectGenerator) generateNginxConfig() string {
	return "server {\n    listen 80;\n    location / {\n        proxy_pass http://localhost:3000;\n    }\n}"
}

func (g *TestProjectGenerator) generateMonorepoSetupScript() string {
	return "#!/bin/bash\necho 'Setting up monorepo...'\nnpm install\nnpm run build"
}

func (g *TestProjectGenerator) generateMonorepoTestScript() string {
	return "#!/bin/bash\necho 'Running monorepo tests...'\nnpm test"
}

func (g *TestProjectGenerator) generateMonorepoDeployScript() string {
	return "#!/bin/bash\necho 'Deploying monorepo...'\nnpm run build && npm run deploy"
}

func (g *TestProjectGenerator) generateMonorepoWebAppReadme(projectName string) string {
	return fmt.Sprintf("# %s Web App\n\nThis is the web application component of the monorepo.\n\n## Getting Started\n\n```bash\nnpm install\nnpm start\n```", projectName)
}

func (g *TestProjectGenerator) generateMonorepoArchitectureDoc() string {
	return "# Architecture\n\nThis monorepo follows a microservices architecture with:\n\n- Frontend: React\n- Backend: Go\n- Database: PostgreSQL"
}

func (g *TestProjectGenerator) generateMonorepoAPIDoc() string {
	return "# API Documentation\n\n## Endpoints\n\n- GET /api/users - Get all users\n- POST /api/users - Create user"
}

func (g *TestProjectGenerator) generateMonorepoDeploymentDoc() string {
	return "# Deployment\n\n## Production\n\n```bash\ndocker-compose -f docker-compose.prod.yml up\n```"
}

func (g *TestProjectGenerator) generatePrometheusConfig() string {
	return "global:\n  scrape_interval: 15s\nscrape_configs:\n  - job_name: 'prometheus'\n    static_configs:\n      - targets: ['localhost:9090']"
}

func (g *TestProjectGenerator) generateGrafanaDashboard() string {
	return "{\"dashboard\": {\"title\": \"Application Metrics\", \"panels\": []}}"
}

func (g *TestProjectGenerator) generateAPIGatewayRoutes() string {
	return "const routes = [\n  { path: '/api/users', target: 'http://user-service:3000' },\n  { path: '/api/auth', target: 'http://auth-service:3001' }\n];"
}

func (g *TestProjectGenerator) generateAPIGatewayAuthMiddleware() string {
	return "const authMiddleware = (req, res, next) => {\n  const token = req.headers.authorization;\n  if (!token) return res.status(401).send('Unauthorized');\n  next();\n};"
}

func (g *TestProjectGenerator) generateAPIGatewayRateLimit() string {
	return "const rateLimit = require('express-rate-limit');\nconst limiter = rateLimit({ windowMs: 15 * 60 * 1000, max: 100 });"
}

func (g *TestProjectGenerator) generateServiceProxy() string {
	return "const httpProxy = require('http-proxy-middleware');\nconst proxy = httpProxy({ target: 'http://localhost:3000', changeOrigin: true });"
}

func (g *TestProjectGenerator) generateServiceDiscoveryConfig() string {
	return "services:\n  user-service:\n    host: localhost\n    port: 3000\n  auth-service:\n    host: localhost\n    port: 3001"
}

func (g *TestProjectGenerator) generateUserServiceGRPCHandlers() string {
	return "package handlers\n\nimport \"context\"\n\nfunc (s *Server) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {\n\treturn &pb.User{Id: req.Id, Name: \"User\"}, nil\n}"
}

func (g *TestProjectGenerator) generateUserServiceHTTPHandlers() string {
	return "package handlers\n\nimport \"net/http\"\n\nfunc GetUser(w http.ResponseWriter, r *http.Request) {\n\tw.Write([]byte(`{\"id\": 1, \"name\": \"User\"}`))\n}"
}

func (g *TestProjectGenerator) generateUserServiceBusiness() string {
	return "package business\n\ntype UserService struct{}\n\nfunc (s *UserService) GetUser(id string) (*User, error) {\n\treturn &User{ID: id, Name: \"User\"}, nil\n}"
}

func (g *TestProjectGenerator) generateUserServiceRepository() string {
	return "package repository\n\nimport \"database/sql\"\n\ntype UserRepository struct {\n\tdb *sql.DB\n}\n\nfunc (r *UserRepository) GetUser(id string) (*User, error) {\n\treturn &User{ID: id}, nil\n}"
}

func (g *TestProjectGenerator) generateUserServiceProto() string {
	return "syntax = \"proto3\";\n\npackage user;\n\nservice UserService {\n  rpc GetUser(GetUserRequest) returns (User);\n}\n\nmessage GetUserRequest {\n  string id = 1;\n}\n\nmessage User {\n  string id = 1;\n  string name = 2;\n}"
}

func (g *TestProjectGenerator) generateUserServiceMigrations() string {
	return "CREATE TABLE users (\n  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),\n  name VARCHAR(255) NOT NULL,\n  email VARCHAR(255) UNIQUE NOT NULL,\n  created_at TIMESTAMP DEFAULT NOW()\n);"
}

func (g *TestProjectGenerator) generateOrderServicePom() string {
	return "<?xml version=\"1.0\"?>\n<project>\n  <modelVersion>4.0.0</modelVersion>\n  <groupId>com.example</groupId>\n  <artifactId>order-service</artifactId>\n  <version>1.0.0</version>\n</project>"
}

func (g *TestProjectGenerator) generateOrderServiceApplication() string {
	return "package com.example.order;\n\nimport org.springframework.boot.SpringApplication;\nimport org.springframework.boot.autoconfigure.SpringBootApplication;\n\n@SpringBootApplication\npublic class OrderServiceApplication {\n    public static void main(String[] args) {\n        SpringApplication.run(OrderServiceApplication.class, args);\n    }\n}"
}

func (g *TestProjectGenerator) generateOrderController() string {
	return "package com.example.order.controller;\n\nimport org.springframework.web.bind.annotation.*;\n\n@RestController\n@RequestMapping(\"/api/orders\")\npublic class OrderController {\n    @GetMapping\n    public String getOrders() {\n        return \"Orders\";\n    }\n}"
}

func (g *TestProjectGenerator) generateOrderKafkaConfig() string {
	return "package com.example.order.config;\n\nimport org.springframework.kafka.annotation.EnableKafka;\nimport org.springframework.context.annotation.Configuration;\n\n@EnableKafka\n@Configuration\npublic class KafkaConfig {}"
}

func (g *TestProjectGenerator) generateOrderModel() string {
	return "package com.example.order.model;\n\nimport javax.persistence.*;\n\n@Entity\npublic class Order {\n    @Id\n    @GeneratedValue(strategy = GenerationType.IDENTITY)\n    private Long id;\n    private String product;\n    // getters and setters\n}"
}

func (g *TestProjectGenerator) generateOrderRepository() string {
	return "package com.example.order.repository;\n\nimport org.springframework.data.jpa.repository.JpaRepository;\nimport com.example.order.model.Order;\n\npublic interface OrderRepository extends JpaRepository<Order, Long> {}"
}

func (g *TestProjectGenerator) generateOrderServiceBusiness() string {
	return "package com.example.order.service;\n\nimport org.springframework.stereotype.Service;\n\n@Service\npublic class OrderService {\n    public String processOrder(String orderId) {\n        return \"Order processed: \" + orderId;\n    }\n}"
}

func (g *TestProjectGenerator) generateOrderServiceConfig() string {
	return "server:\n  port: 8082\nspring:\n  application:\n    name: order-service\n  datasource:\n    url: jdbc:postgresql://localhost:5432/orders\n    username: user\n    password: password"
}

func (g *TestProjectGenerator) generatePaymentProcessor() string {
	return "\"\"\"Payment processing service.\"\"\"\n\nclass PaymentProcessor:\n    def process_payment(self, amount, currency):\n        return {\"status\": \"success\", \"amount\": amount, \"currency\": currency}"
}

func (g *TestProjectGenerator) generatePaymentRouters() string {
	return "\"\"\"Payment service routes.\"\"\"\nfrom fastapi import APIRouter\n\nrouter = APIRouter()\n\n@router.post(\"/process\")\nasync def process_payment(payment_data: dict):\n    return {\"status\": \"processed\"}"
}

func (g *TestProjectGenerator) generatePaymentServiceMain() string {
	return "\"\"\"Payment service main entry point.\"\"\"\nfrom fastapi import FastAPI\n\napp = FastAPI(title=\"Payment Service\")\n\n@app.get(\"/health\")\nasync def health():\n    return {\"status\": \"healthy\"}"
}

func (g *TestProjectGenerator) generatePaymentServicePyproject(projectName string) string {
	return fmt.Sprintf(`[build-system]
requires = ["setuptools", "wheel"]

[project]
name = "%s-payment-service"
version = "1.0.0"
dependencies = [
    "fastapi>=0.68.0",
    "uvicorn>=0.15.0",
    "stripe>=3.0.0"
]`, projectName)
}

func (g *TestProjectGenerator) generateStripeIntegration() string {
	return "\"\"\"Stripe payment integration.\"\"\"\nimport stripe\n\nclass StripeIntegration:\n    def __init__(self, api_key):\n        stripe.api_key = api_key\n    \n    def create_payment_intent(self, amount, currency=\"usd\"):\n        return stripe.PaymentIntent.create(amount=amount, currency=currency)"
}

func (g *TestProjectGenerator) generatePaymentModels() string {
	return "\"\"\"Payment data models.\"\"\"\nfrom sqlalchemy import Column, Integer, String, Float\nfrom sqlalchemy.ext.declarative import declarative_base\n\nBase = declarative_base()\n\nclass Payment(Base):\n    __tablename__ = 'payments'\n    id = Column(Integer, primary_key=True)\n    amount = Column(Float)\n    currency = Column(String(3))"
}

func (g *TestProjectGenerator) generatePaymentMigrations() string {
	return "CREATE TABLE payments (\n  id SERIAL PRIMARY KEY,\n  amount DECIMAL(10,2) NOT NULL,\n  currency VARCHAR(3) NOT NULL DEFAULT 'USD',\n  status VARCHAR(20) NOT NULL DEFAULT 'pending',\n  created_at TIMESTAMP DEFAULT NOW()\n);"
}

func (g *TestProjectGenerator) generateNotificationServiceCargo() string {
	return "[package]\nname = \"notification-service\"\nversion = \"0.1.0\"\nedition = \"2021\"\n\n[dependencies]\ntokio = { version = \"1.0\", features = [\"full\"] }\nserde = { version = \"1.0\", features = [\"derive\"] }"
}

func (g *TestProjectGenerator) generateNotificationServiceMain() string {
	return "use tokio::main;\n\n#[main]\nasync fn main() {\n    println!(\"Notification service started\");\n    // Start HTTP server\n}"
}

func (g *TestProjectGenerator) generateNotificationHandlers() string {
	return "use serde::{Deserialize, Serialize};\n\n#[derive(Serialize, Deserialize)]\npub struct Notification {\n    pub id: String,\n    pub message: String,\n}\n\npub async fn send_notification(notification: Notification) -> Result<(), Box<dyn std::error::Error>> {\n    println!(\"Sending notification: {}\", notification.message);\n    Ok(())\n}"
}

func (g *TestProjectGenerator) generateEmailService() string {
	return "pub struct EmailService;\n\nimpl EmailService {\n    pub async fn send_email(&self, to: &str, subject: &str, body: &str) -> Result<(), Box<dyn std::error::Error>> {\n        println!(\"Sending email to: {}, Subject: {}\", to, subject);\n        Ok(())\n    }\n}"
}

func (g *TestProjectGenerator) generateSMSService() string {
	return "pub struct SMSService;\n\nimpl SMSService {\n    pub async fn send_sms(&self, to: &str, message: &str) -> Result<(), Box<dyn std::error::Error>> {\n        println!(\"Sending SMS to: {}, Message: {}\", to, message);\n        Ok(())\n    }\n}"
}

func (g *TestProjectGenerator) generateNotificationModels() string {
	return "use serde::{Deserialize, Serialize};\n\n#[derive(Serialize, Deserialize, Clone)]\npub struct NotificationRequest {\n    pub recipient: String,\n    pub message: String,\n    pub notification_type: NotificationType,\n}\n\n#[derive(Serialize, Deserialize, Clone)]\npub enum NotificationType {\n    Email,\n    SMS,\n    Push,\n}"
}

func (g *TestProjectGenerator) generateRedisQueue() string {
	return "use redis::{Client, Commands};\n\npub struct RedisQueue {\n    client: Client,\n}\n\nimpl RedisQueue {\n    pub fn new(redis_url: &str) -> Result<Self, redis::RedisError> {\n        let client = Client::open(redis_url)?;\n        Ok(RedisQueue { client })\n    }\n    \n    pub fn push(&self, queue: &str, item: &str) -> Result<(), redis::RedisError> {\n        let mut con = self.client.get_connection()?;\n        con.rpush(queue, item)?;\n        Ok(())\n    }\n}"
}

func (g *TestProjectGenerator) generateAnalyticsServiceCsproj() string {
	return "<Project Sdk=\"Microsoft.NET.Sdk.Web\">\n  <PropertyGroup>\n    <TargetFramework>net6.0</TargetFramework>\n  </PropertyGroup>\n  <ItemGroup>\n    <PackageReference Include=\"Microsoft.EntityFrameworkCore\" Version=\"6.0.0\" />\n  </ItemGroup>\n</Project>"
}

func (g *TestProjectGenerator) generateAnalyticsController() string {
	return "using Microsoft.AspNetCore.Mvc;\n\nnamespace AnalyticsService.Controllers\n{\n    [ApiController]\n    [Route(\"api/[controller]\")]\n    public class AnalyticsController : ControllerBase\n    {\n        [HttpGet]\n        public IActionResult GetAnalytics()\n        {\n            return Ok(new { message = \"Analytics data\" });\n        }\n    }\n}"
}

func (g *TestProjectGenerator) generateAnalyticsEventModel() string {
	return "using System;\n\nnamespace AnalyticsService.Models\n{\n    public class AnalyticsEvent\n    {\n        public Guid Id { get; set; }\n        public string EventType { get; set; }\n        public string UserId { get; set; }\n        public DateTime Timestamp { get; set; }\n        public string Data { get; set; }\n    }\n}"
}

func (g *TestProjectGenerator) generateAnalyticsServiceConfig() string {
	return "{\n  \"Logging\": {\n    \"LogLevel\": {\n      \"Default\": \"Information\",\n      \"Microsoft\": \"Warning\"\n    }\n  },\n  \"ConnectionStrings\": {\n    \"DefaultConnection\": \"Server=localhost;Database=Analytics;Trusted_Connection=true;\"\n  }\n}"
}

func (g *TestProjectGenerator) generateAnalyticsServiceProgram() string {
	return "using Microsoft.AspNetCore.Hosting;\nusing Microsoft.Extensions.Hosting;\n\nnamespace AnalyticsService\n{\n    public class Program\n    {\n        public static void Main(string[] args)\n        {\n            CreateHostBuilder(args).Build().Run();\n        }\n\n        public static IHostBuilder CreateHostBuilder(string[] args) =>\n            Host.CreateDefaultBuilder(args)\n                .ConfigureWebHostDefaults(webBuilder =>\n                {\n                    webBuilder.UseStartup<Startup>();\n                });\n    }\n}"
}

func (g *TestProjectGenerator) generateEventProcessor() string {
	return "using System.Threading.Tasks;\n\nnamespace AnalyticsService.Services\n{\n    public class EventProcessor\n    {\n        public async Task ProcessEvent(AnalyticsEvent analyticsEvent)\n        {\n            // Process the analytics event\n            await Task.Delay(100); // Simulate processing\n        }\n    }\n}"
}

func (g *TestProjectGenerator) generateK8sConfigMap() string {
	return "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: app-config\n  namespace: default\ndata:\n  database_url: \"postgresql://localhost:5432/app\"\n  redis_url: \"redis://localhost:6379\""
}

func (g *TestProjectGenerator) generateK8sIngress() string {
	return "apiVersion: networking.k8s.io/v1\nkind: Ingress\nmetadata:\n  name: app-ingress\n  namespace: default\nspec:\n  rules:\n  - host: app.example.com\n    http:\n      paths:\n      - path: /\n        pathType: Prefix\n        backend:\n          service:\n            name: app-service\n            port:\n              number: 80"
}

func (g *TestProjectGenerator) generateK8sNamespace() string {
	return "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: app-namespace\n  labels:\n    name: app-namespace"
}

func (g *TestProjectGenerator) generateMetricsCollector() string {
	return "using System.Diagnostics.Metrics;\n\nnamespace AnalyticsService.Metrics\n{\n    public class MetricsCollector\n    {\n        private readonly Counter<int> _requestCounter;\n        private readonly Histogram<double> _requestDuration;\n\n        public MetricsCollector()\n        {\n            var meter = new Meter(\"AnalyticsService\");\n            _requestCounter = meter.CreateCounter<int>(\"requests_total\");\n            _requestDuration = meter.CreateHistogram<double>(\"request_duration_ms\");\n        }\n\n        public void IncrementRequestCount() => _requestCounter.Add(1);\n        public void RecordRequestDuration(double duration) => _requestDuration.Record(duration);\n    }\n}"
}

func (g *TestProjectGenerator) generateTerraformMain() string {
	return "terraform {\n  required_providers {\n    aws = {\n      source  = \"hashicorp/aws\"\n      version = \"~> 4.0\"\n    }\n  }\n}\n\nprovider \"aws\" {\n  region = var.aws_region\n}\n\nresource \"aws_instance\" \"app\" {\n  ami           = \"ami-0c02fb55956c7d316\"\n  instance_type = \"t3.micro\"\n  \n  tags = {\n    Name = \"AppServer\"\n  }\n}"
}

func (g *TestProjectGenerator) generateTerraformVariables() string {
	return "variable \"aws_region\" {\n  description = \"AWS region for resources\"\n  type        = string\n  default     = \"us-west-2\"\n}\n\nvariable \"instance_type\" {\n  description = \"EC2 instance type\"\n  type        = string\n  default     = \"t3.micro\"\n}"
}

func (g *TestProjectGenerator) generateHelmChart() string {
	return "apiVersion: v2\nname: app\ndescription: A Helm chart for Kubernetes application\ntype: application\nversion: 0.1.0\nappVersion: \"1.0.0\""
}

func (g *TestProjectGenerator) generateIstioVirtualService() string {
	return "apiVersion: networking.istio.io/v1beta1\nkind: VirtualService\nmetadata:\n  name: app-vs\nspec:\n  hosts:\n  - app.example.com\n  http:\n  - route:\n    - destination:\n        host: app-service\n        port:\n          number: 80"
}

func (g *TestProjectGenerator) generateIstioDestinationRule() string {
	return "apiVersion: networking.istio.io/v1beta1\nkind: DestinationRule\nmetadata:\n  name: app-dr\nspec:\n  host: app-service\n  trafficPolicy:\n    loadBalancer:\n      simple: LEAST_CONN"
}

func (g *TestProjectGenerator) generateIstioGateway() string {
	return "apiVersion: networking.istio.io/v1beta1\nkind: Gateway\nmetadata:\n  name: app-gateway\nspec:\n  selector:\n    istio: ingressgateway\n  servers:\n  - port:\n      number: 80\n      name: http\n      protocol: HTTP\n    hosts:\n    - app.example.com"
}

func (g *TestProjectGenerator) generatePythonProcessor() string {
	return "\"\"\"Core data processor.\"\"\"\nimport asyncio\nfrom typing import Dict, Any\n\nclass DataProcessor:\n    def __init__(self):\n        self.processed_count = 0\n    \n    async def process_data(self, data: Dict[str, Any]) -> Dict[str, Any]:\n        \"\"\"Process incoming data.\"\"\"\n        self.processed_count += 1\n        return {\"processed\": True, \"data\": data, \"count\": self.processed_count}"
}

func (g *TestProjectGenerator) generatePythonAPIRoutes() string {
	return "\"\"\"API routes for data processing.\"\"\"\nfrom fastapi import APIRouter\nfrom .processor import DataProcessor\n\nrouter = APIRouter()\nprocessor = DataProcessor()\n\n@router.post(\"/process\")\nasync def process_data(data: dict):\n    result = await processor.process_data(data)\n    return result\n\n@router.get(\"/status\")\nasync def get_status():\n    return {\"status\": \"running\", \"processed\": processor.processed_count}"
}

func (g *TestProjectGenerator) generatePythonDataModels() string {
	return "\"\"\"Data models for processing.\"\"\"\nfrom pydantic import BaseModel\nfrom typing import Optional, Dict, Any\n\nclass ProcessingRequest(BaseModel):\n    data: Dict[str, Any]\n    priority: Optional[int] = 1\n    \nclass ProcessingResult(BaseModel):\n    success: bool\n    result: Optional[Dict[str, Any]] = None\n    error: Optional[str] = None"
}

func (g *TestProjectGenerator) generatePythonCalculationService() string {
	return "\"\"\"Mathematical calculation service.\"\"\"\nimport math\nfrom typing import List, Union\n\nclass CalculationService:\n    @staticmethod\n    def mean(values: List[Union[int, float]]) -> float:\n        return sum(values) / len(values) if values else 0.0\n    \n    @staticmethod\n    def std_dev(values: List[Union[int, float]]) -> float:\n        if len(values) < 2:\n            return 0.0\n        mean_val = CalculationService.mean(values)\n        variance = sum((x - mean_val) ** 2 for x in values) / (len(values) - 1)\n        return math.sqrt(variance)"
}

func (g *TestProjectGenerator) generatePythonPerformanceUtils() string {
	return "\"\"\"Performance monitoring utilities.\"\"\"\nimport time\nimport psutil\nfrom typing import Dict, Any\n\nclass PerformanceMonitor:\n    def __init__(self):\n        self.start_time = time.time()\n    \n    def get_metrics(self) -> Dict[str, Any]:\n        return {\n            \"uptime\": time.time() - self.start_time,\n            \"cpu_percent\": psutil.cpu_percent(),\n            \"memory_percent\": psutil.virtual_memory().percent,\n            \"disk_usage\": psutil.disk_usage('/').percent\n        }"
}
// Go Extension Methods
func (g *TestProjectGenerator) generateGoParserExtension() string {
	return `package parser
import "fmt"
func Parse(data string) string { return fmt.Sprintf("parsed: %s", data) }`
}

func (g *TestProjectGenerator) generateGoCryptoExtension() string {
	return `package crypto
import "crypto/sha256"
func Hash(data []byte) []byte { h := sha256.Sum256(data); return h[:] }`
}

func (g *TestProjectGenerator) generateGoCompressionExtension() string {
	return `package compression
import "compress/gzip"
func Compress(data []byte) []byte { return data }`
}

func (g *TestProjectGenerator) generateGoCBridge() string {
	return `package main
/*
#include "bridge.h"
*/
import "C"
func main() {}`
}

func (g *TestProjectGenerator) generateGoCBridgeHeader() string {
	return `#ifndef BRIDGE_H
#define BRIDGE_H
#endif`
}

// Python Go Wrapper Methods
func (g *TestProjectGenerator) generatePythonGoCalculatorWrapper() string {
	return `import ctypes
class GoCalculator:
    def calculate(self, x, y): return x + y`
}

func (g *TestProjectGenerator) generatePythonGoParserWrapper() string {
	return `import ctypes
class GoParser:
    def parse(self, data): return f"parsed: {data}"`
}

func (g *TestProjectGenerator) generatePythonGoCryptoWrapper() string {
	return `import hashlib
class GoCrypto:
    def hash(self, data): return hashlib.sha256(data).hexdigest()`
}

func (g *TestProjectGenerator) generatePythonGoCompressionWrapper() string {
	return `import gzip
class GoCompression:
    def compress(self, data): return gzip.compress(data)`
}

func (g *TestProjectGenerator) generatePythonGoExtensionsMakefile() string {
	return `all:
	go build -buildmode=c-shared -o extensions.so extensions/main.go
clean:
	rm -f extensions.so`
}
