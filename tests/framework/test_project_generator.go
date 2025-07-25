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
	Type          ProjectType
	Languages     []string
	Complexity    ProjectComplexity
	Size          ProjectSize
	BuildSystem   bool
	TestFiles     bool
	Dependencies  bool
	Documentation bool
	CI            bool
	Docker        bool

	// Customization options
	FileCount        int
	DirectoryDepth   int
	CrossReferences  bool
	RealisticContent bool
	VersionControl   bool

	// Enhanced options for complex scenarios
	MicroserviceCount    int
	DatabaseIntegration  bool
	KubernetesConfig     bool
	APIGateway           bool
	ServiceMesh          bool
	MonitoringStack      bool
	SharedLibraries      bool
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
	TempDir   string
	Templates map[string]*ProjectTemplate
	Generated []*TestProject

	// Call tracking
	LoadTemplatesCalls   []context.Context
	GenerateProjectCalls []ProjectGenerationConfig

	// Function fields for behavior customization
	LoadTemplatesFunc   func(ctx context.Context) error
	GenerateProjectFunc func(config *ProjectGenerationConfig) (*TestProject, error)

	mu sync.RWMutex
}

// NewTestProjectGenerator creates a new test project generator
func NewTestProjectGenerator(tempDir string) *TestProjectGenerator {
	return &TestProjectGenerator{
		TempDir:              tempDir,
		Templates:            make(map[string]*ProjectTemplate),
		Generated:            make([]*TestProject, 0),
		LoadTemplatesCalls:   make([]context.Context, 0),
		GenerateProjectCalls: make([]ProjectGenerationConfig, 0),
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

	// Load enhanced complex scenario templates (TODO: implement missing methods)
	// g.loadMonorepoWebAppTemplates()
	// g.loadMicroservicesSuiteTemplates()
	// g.loadPythonWithGoExtensionsTemplates()
	// g.loadFullStackTypeScriptTemplates()
	// g.loadJavaSpringWithReactTemplates()
	// g.loadEnterprisePlatformTemplates()

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
		return g.generateGenericContent(project, "javascript", config)
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
		project.Structure["src/types.ts"] = "// TypeScript types\nexport interface User { id: number; name: string; }"
		project.Structure["src/services/api.ts"] = "// API service\nexport const apiCall = () => console.log('API call');"
	case ComplexityComplex, ComplexityLarge:
		project.Structure["src/index.ts"] = g.generateTypeScriptIndex()
		project.Structure["src/types/user.ts"] = "// User types\nexport interface User { id: number; name: string; email: string; }"
		project.Structure["src/services/userService.ts"] = "// User service\nexport class UserService { getUser() { return null; } }"
		project.Structure["src/utils/helpers.ts"] = g.generateTypeScriptUtils()
		project.Structure["src/components/UserComponent.tsx"] = "// React component\nexport const UserComponent = () => <div>User</div>;"
		project.Structure["src/api/routes.ts"] = "// API routes\nexport const routes = [];"
		project.Structure["src/database/connection.ts"] = "// Database connection\nexport const db = null;"
	}

	// Add dependencies
	project.Dependencies["typescript"] = []string{"react", "express", "axios"}

	return nil
}

// generateJavaContent generates Java-specific content
func (g *TestProjectGenerator) generateJavaContent(project *TestProject, config *ProjectGenerationConfig) error {
	// Generate pom.xml
	project.Structure["pom.xml"] = `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>` + project.Name + `</artifactId>
    <version>1.0.0</version>
    <properties>
        <maven.compiler.source>11</maven.compiler.source>
        <maven.compiler.target>11</maven.compiler.target>
    </properties>
</project>`
	project.BuildFiles = append(project.BuildFiles, "pom.xml")

	// Generate content based on complexity
	packagePath := "src/main/java/com/example"

	switch config.Complexity {
	case ComplexitySimple:
		project.Structure[packagePath+"/Main.java"] = "package com.example;\npublic class Main {\n    public static void main(String[] args) {\n        System.out.println(\"Hello, World!\");\n    }\n}"
		project.Structure[packagePath+"/Utils.java"] = "package com.example;\npublic class Utils {\n    public static String helper() { return \"utility\"; }\n}"
	case ComplexityMedium:
		project.Structure[packagePath+"/Main.java"] = "package com.example;\npublic class Main {\n    public static void main(String[] args) {\n        System.out.println(\"Hello, World!\");\n    }\n}"
		project.Structure[packagePath+"/service/UserService.java"] = "package com.example.service;\npublic class UserService {\n    public void getUser() {}\n}"
		project.Structure[packagePath+"/model/User.java"] = "package com.example.model;\npublic class User {\n    private String name;\n    public String getName() { return name; }\n}"
		project.Structure[packagePath+"/util/Helper.java"] = "package com.example.util;\npublic class Helper {\n    public static String help() { return \"help\"; }\n}"
	case ComplexityComplex, ComplexityLarge:
		project.Structure[packagePath+"/Application.java"] = "package com.example;\n@SpringBootApplication\npublic class Application {\n    public static void main(String[] args) {\n        SpringApplication.run(Application.class, args);\n    }\n}"
		project.Structure[packagePath+"/controller/UserController.java"] = "package com.example.controller;\n@RestController\npublic class UserController {\n    @GetMapping(\"/users\")\n    public String getUsers() { return \"users\"; }\n}"
		project.Structure[packagePath+"/service/UserService.java"] = "package com.example.service;\n@Service\npublic class UserService {\n    public void getUser() {}\n}"
		project.Structure[packagePath+"/model/User.java"] = "package com.example.model;\n@Entity\npublic class User {\n    @Id\n    private Long id;\n    private String name;\n}"
		project.Structure[packagePath+"/repository/UserRepository.java"] = "package com.example.repository;\npublic interface UserRepository extends JpaRepository<User, Long> {}"
		project.Structure[packagePath+"/config/DatabaseConfig.java"] = "package com.example.config;\n@Configuration\npublic class DatabaseConfig {}"
		project.Structure["src/main/resources/application.properties"] = "server.port=8080\nspring.datasource.url=jdbc:h2:mem:testdb"
	}

	// Add dependencies
	project.Dependencies["java"] = []string{"spring-boot-starter-web", "spring-boot-starter-data-jpa"}

	return nil
}

// generateRustContent generates Rust-specific content
func (g *TestProjectGenerator) generateRustContent(project *TestProject, config *ProjectGenerationConfig) error {
	// Generate Cargo.toml
	project.Structure["Cargo.toml"] = `[package]
name = "` + project.Name + `"
version = "0.1.0"
edition = "2021"

[dependencies]
serde = "1.0"
tokio = "1.0"
axum = "0.6"`
	project.BuildFiles = append(project.BuildFiles, "Cargo.toml")

	// Generate content based on complexity
	switch config.Complexity {
	case ComplexitySimple:
		project.Structure["src/main.rs"] = "fn main() {\n    println!(\"Hello, world!\");\n}"
		project.Structure["src/lib.rs"] = "pub fn add(left: usize, right: usize) -> usize {\n    left + right\n}"
	case ComplexityMedium:
		project.Structure["src/main.rs"] = "fn main() {\n    println!(\"Hello, world!\");\n}"
		project.Structure["src/lib.rs"] = "pub fn add(left: usize, right: usize) -> usize {\n    left + right\n}"
		project.Structure["src/utils.rs"] = "pub fn utility() -> String {\n    \"utility\".to_string()\n}"
		project.Structure["src/models.rs"] = "#[derive(Debug)]\npub struct User {\n    pub name: String,\n}"
	case ComplexityComplex, ComplexityLarge:
		project.Structure["src/main.rs"] = "fn main() {\n    println!(\"Hello, world!\");\n}"
		project.Structure["src/lib.rs"] = "pub fn add(left: usize, right: usize) -> usize {\n    left + right\n}"
		project.Structure["src/models/mod.rs"] = "pub mod user;"
		project.Structure["src/models/user.rs"] = "#[derive(Debug, serde::Serialize)]\npub struct User {\n    pub id: u64,\n    pub name: String,\n}"
		project.Structure["src/services/mod.rs"] = "pub mod user_service;"
		project.Structure["src/services/user_service.rs"] = "use crate::models::user::User;\n\npub struct UserService;\n\nimpl UserService {\n    pub fn get_user(&self) -> Option<User> { None }\n}"
		project.Structure["src/utils/mod.rs"] = "pub mod helpers;"
		project.Structure["src/utils/helpers.rs"] = "pub fn utility() -> String {\n    \"utility\".to_string()\n}"
	}

	// Add dependencies
	project.Dependencies["rust"] = []string{"serde", "tokio", "axum"}

	return nil
}

// generateCppContent generates C++ content
func (g *TestProjectGenerator) generateCppContent(project *TestProject, config *ProjectGenerationConfig) error {
	// Generate CMakeLists.txt
	project.Structure["CMakeLists.txt"] = `cmake_minimum_required(VERSION 3.10)
project(` + project.Name + `)

set(CMAKE_CXX_STANDARD 17)

add_executable(${PROJECT_NAME} src/main.cpp)`
	project.BuildFiles = append(project.BuildFiles, "CMakeLists.txt")

	switch config.Complexity {
	case ComplexitySimple:
		project.Structure["main.cpp"] = "#include <iostream>\n\nint main() {\n    std::cout << \"Hello, World!\" << std::endl;\n    return 0;\n}"
		project.Structure["utils.h"] = "#pragma once\n#include <string>\n\nstd::string utility();"
		project.Structure["utils.cpp"] = "#include \"utils.h\"\n\nstd::string utility() {\n    return \"utility\";\n}"
	default:
		project.Structure["src/main.cpp"] = "#include <iostream>\n\nint main() {\n    std::cout << \"Hello, World!\" << std::endl;\n    return 0;\n}"
		project.Structure["include/utils.h"] = "#pragma once\n#include <string>\n\nstd::string utility();"
		project.Structure["src/utils.cpp"] = "#include \"utils.h\"\n\nstd::string utility() {\n    return \"utility\";\n}"
		project.Structure["include/user.h"] = "#pragma once\n#include <string>\n\nclass User {\npublic:\n    std::string name;\n};"
		project.Structure["src/user.cpp"] = "#include \"user.h\""
	}

	return nil
}

// generateCSharpContent generates C# content
func (g *TestProjectGenerator) generateCSharpContent(project *TestProject, config *ProjectGenerationConfig) error {
	// Generate .csproj file
	project.Structure[project.Name+".csproj"] = `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <OutputType>Exe</OutputType>
    <TargetFramework>net6.0</TargetFramework>
  </PropertyGroup>
</Project>`
	project.BuildFiles = append(project.BuildFiles, project.Name+".csproj")

	switch config.Complexity {
	case ComplexitySimple:
		project.Structure["Program.cs"] = "using System;\n\nnamespace " + project.Name + "\n{\n    class Program\n    {\n        static void Main(string[] args)\n        {\n            Console.WriteLine(\"Hello, World!\");\n        }\n    }\n}"
		project.Structure["Utils.cs"] = "namespace " + project.Name + "\n{\n    public static class Utils\n    {\n        public static string Helper() => \"utility\";\n    }\n}"
	default:
		project.Structure["Program.cs"] = "using System;\n\nnamespace " + project.Name + "\n{\n    class Program\n    {\n        static void Main(string[] args)\n        {\n            Console.WriteLine(\"Hello, World!\");\n        }\n    }\n}"
		project.Structure["Models/User.cs"] = "namespace " + project.Name + ".Models\n{\n    public class User\n    {\n        public string Name { get; set; }\n    }\n}"
		project.Structure["Services/UserService.cs"] = "using " + project.Name + ".Models;\n\nnamespace " + project.Name + ".Services\n{\n    public class UserService\n    {\n        public User GetUser() => new User();\n    }\n}"
		project.Structure["Controllers/UserController.cs"] = "using " + project.Name + ".Services;\n\nnamespace " + project.Name + ".Controllers\n{\n    public class UserController\n    {\n        private readonly UserService _userService;\n    }\n}"
		project.Structure["Utils/Helper.cs"] = "namespace " + project.Name + ".Utils\n{\n    public static class Helper\n    {\n        public static string Help() => \"help\";\n    }\n}"
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
