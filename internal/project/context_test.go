package project

import (
	"testing"
	"time"

	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/project/types"
)

func TestNewProjectContext(t *testing.T) {
	tests := []struct {
		name        string
		projectType string
		rootPath    string
	}{
		{
			name:        "go project context",
			projectType: "go",
			rootPath:    "/test/go-project",
		},
		{
			name:        "python project context",
			projectType: "python",
			rootPath:    "/test/python-project",
		},
		{
			name:        "typescript project context",
			projectType: "typescript",
			rootPath:    "/test/ts-project",
		},
		{
			name:        "unknown project context",
			projectType: "unknown",
			rootPath:    "/test/unknown-project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewProjectContext(tt.projectType, tt.rootPath)

			if ctx == nil {
				t.Fatal("NewProjectContext() returned nil")
			}

			if ctx.ProjectType != tt.projectType {
				t.Errorf("ProjectType = %v, want %v", ctx.ProjectType, tt.projectType)
			}

			if ctx.RootPath != tt.rootPath {
				t.Errorf("RootPath = %v, want %v", ctx.RootPath, tt.rootPath)
			}

			if ctx.WorkspaceRoot != tt.rootPath {
				t.Errorf("WorkspaceRoot = %v, want %v", ctx.WorkspaceRoot, tt.rootPath)
			}

			if ctx.Confidence != 0.0 {
				t.Errorf("Confidence = %v, want 0.0", ctx.Confidence)
			}

			if ctx.IsValid {
				t.Error("IsValid should be false for new context")
			}

			if ctx.IsMonorepo {
				t.Error("IsMonorepo should be false for new context")
			}

			if ctx.Languages == nil {
				t.Error("Languages slice should be initialized")
			}

			if ctx.LanguageVersions == nil {
				t.Error("LanguageVersions map should be initialized")
			}

			if ctx.Dependencies == nil {
				t.Error("Dependencies map should be initialized")
			}

			if ctx.DevDependencies == nil {
				t.Error("DevDependencies map should be initialized")
			}

			if ctx.RequiredServers == nil {
				t.Error("RequiredServers slice should be initialized")
			}

			if ctx.Metadata == nil {
				t.Error("Metadata map should be initialized")
			}

			if ctx.Platform != platform.GetCurrentPlatform() {
				t.Errorf("Platform = %v, want %v", ctx.Platform, platform.GetCurrentPlatform())
			}

			if ctx.Architecture != platform.GetCurrentArchitecture() {
				t.Errorf("Architecture = %v, want %v", ctx.Architecture, platform.GetCurrentArchitecture())
			}

			if ctx.DetectedAt.IsZero() {
				t.Error("DetectedAt should be set to current time")
			}
		})
	}
}

func TestProjectContext_LanguageManagement(t *testing.T) {
	ctx := NewProjectContext("mixed", "/test/project")

	t.Run("AddLanguage", func(t *testing.T) {
		ctx.AddLanguage("go")
		if len(ctx.Languages) != 1 {
			t.Errorf("Expected 1 language, got %d", len(ctx.Languages))
		}
		if ctx.Languages[0] != "go" {
			t.Errorf("Expected 'go', got %v", ctx.Languages[0])
		}
		if ctx.PrimaryLanguage != "go" {
			t.Errorf("PrimaryLanguage should be set to 'go', got %v", ctx.PrimaryLanguage)
		}

		ctx.AddLanguage("python")
		if len(ctx.Languages) != 2 {
			t.Errorf("Expected 2 languages, got %d", len(ctx.Languages))
		}
		if ctx.PrimaryLanguage != "go" {
			t.Errorf("PrimaryLanguage should remain 'go', got %v", ctx.PrimaryLanguage)
		}

		ctx.AddLanguage("go")
		if len(ctx.Languages) != 2 {
			t.Errorf("Adding duplicate language should not increase count, got %d", len(ctx.Languages))
		}
	})

	t.Run("HasLanguage", func(t *testing.T) {
		if !ctx.HasLanguage("go") {
			t.Error("Should have 'go' language")
		}
		if !ctx.HasLanguage("python") {
			t.Error("Should have 'python' language")
		}
		if ctx.HasLanguage("java") {
			t.Error("Should not have 'java' language")
		}
	})

	t.Run("IsMultiLanguage", func(t *testing.T) {
		if !ctx.IsMultiLanguage() {
			t.Error("Should be multi-language project")
		}

		singleLangCtx := NewProjectContext("go", "/test/go-project")
		singleLangCtx.AddLanguage("go")
		if singleLangCtx.IsMultiLanguage() {
			t.Error("Single language project should not be multi-language")
		}
	})

	t.Run("SetLanguageVersion and GetLanguageVersion", func(t *testing.T) {
		ctx.SetLanguageVersion("go", "1.21.0")
		ctx.SetLanguageVersion("python", "3.11.0")

		goVersion, exists := ctx.GetLanguageVersion("go")
		if !exists {
			t.Error("Go version should exist")
		}
		if goVersion != "1.21.0" {
			t.Errorf("Go version = %v, want 1.21.0", goVersion)
		}

		pythonVersion, exists := ctx.GetLanguageVersion("python")
		if !exists {
			t.Error("Python version should exist")
		}
		if pythonVersion != "3.11.0" {
			t.Errorf("Python version = %v, want 3.11.0", pythonVersion)
		}

		_, exists = ctx.GetLanguageVersion("java")
		if exists {
			t.Error("Java version should not exist")
		}
	})
}

func TestProjectContext_ServerManagement(t *testing.T) {
	ctx := NewProjectContext("go", "/test/project")

	t.Run("AddRequiredServer", func(t *testing.T) {
		ctx.AddRequiredServer("gopls")
		if len(ctx.RequiredServers) != 1 {
			t.Errorf("Expected 1 server, got %d", len(ctx.RequiredServers))
		}
		if ctx.RequiredServers[0] != "gopls" {
			t.Errorf("Expected 'gopls', got %v", ctx.RequiredServers[0])
		}

		ctx.AddRequiredServer("pylsp")
		if len(ctx.RequiredServers) != 2 {
			t.Errorf("Expected 2 servers, got %d", len(ctx.RequiredServers))
		}

		ctx.AddRequiredServer("gopls")
		if len(ctx.RequiredServers) != 2 {
			t.Errorf("Adding duplicate server should not increase count, got %d", len(ctx.RequiredServers))
		}
	})

	t.Run("HasServer", func(t *testing.T) {
		if !ctx.HasServer("gopls") {
			t.Error("Should have 'gopls' server")
		}
		if !ctx.HasServer("pylsp") {
			t.Error("Should have 'pylsp' server")
		}
		if ctx.HasServer("jdtls") {
			t.Error("Should not have 'jdtls' server")
		}
	})
}

func TestProjectContext_MarkerFileManagement(t *testing.T) {
	ctx := NewProjectContext("go", "/test/project")

	t.Run("AddMarkerFile", func(t *testing.T) {
		ctx.AddMarkerFile("go.mod")
		if len(ctx.MarkerFiles) != 1 {
			t.Errorf("Expected 1 marker file, got %d", len(ctx.MarkerFiles))
		}
		if ctx.MarkerFiles[0] != "go.mod" {
			t.Errorf("Expected 'go.mod', got %v", ctx.MarkerFiles[0])
		}

		ctx.AddMarkerFile("go.sum")
		if len(ctx.MarkerFiles) != 2 {
			t.Errorf("Expected 2 marker files, got %d", len(ctx.MarkerFiles))
		}

		ctx.AddMarkerFile("go.mod")
		if len(ctx.MarkerFiles) != 2 {
			t.Errorf("Adding duplicate marker file should not increase count, got %d", len(ctx.MarkerFiles))
		}
	})
}

func TestProjectContext_MetadataOperations(t *testing.T) {
	ctx := NewProjectContext("go", "/test/project")

	t.Run("SetMetadata and GetMetadata", func(t *testing.T) {
		ctx.SetMetadata("module_name", "example.com/myproject")
		ctx.SetMetadata("go_version", "1.21.0")
		ctx.SetMetadata("features", []string{"web", "api"})

		moduleName, exists := ctx.GetMetadata("module_name")
		if !exists {
			t.Error("module_name should exist")
		}
		if moduleName != "example.com/myproject" {
			t.Errorf("module_name = %v, want example.com/myproject", moduleName)
		}

		goVersion, exists := ctx.GetMetadata("go_version")
		if !exists {
			t.Error("go_version should exist")
		}
		if goVersion != "1.21.0" {
			t.Errorf("go_version = %v, want 1.21.0", goVersion)
		}

		features, exists := ctx.GetMetadata("features")
		if !exists {
			t.Error("features should exist")
		}
		featuresSlice, ok := features.([]string)
		if !ok {
			t.Error("features should be []string type")
		}
		if len(featuresSlice) != 2 {
			t.Errorf("Expected 2 features, got %d", len(featuresSlice))
		}

		_, exists = ctx.GetMetadata("non_existent")
		if exists {
			t.Error("non_existent key should not exist")
		}
	})

	t.Run("Metadata initialization", func(t *testing.T) {
		newCtx := &ProjectContext{}
		newCtx.SetMetadata("test", "value")
		
		if newCtx.Metadata == nil {
			t.Error("Metadata should be initialized")
		}
		
		value, exists := newCtx.GetMetadata("test")
		if !exists || value != "value" {
			t.Error("Metadata should be properly set after initialization")
		}
	})
}

func TestProjectContext_ValidationHelpers(t *testing.T) {
	ctx := NewProjectContext("go", "/test/project")

	t.Run("AddValidationError", func(t *testing.T) {
		if ctx.IsValid {
			t.Error("Context should be invalid initially")
		}

		ctx.AddValidationError("Missing go.mod file")
		if len(ctx.ValidationErrors) != 1 {
			t.Errorf("Expected 1 validation error, got %d", len(ctx.ValidationErrors))
		}
		if ctx.ValidationErrors[0] != "Missing go.mod file" {
			t.Errorf("Expected 'Missing go.mod file', got %v", ctx.ValidationErrors[0])
		}
		if ctx.IsValid {
			t.Error("Context should remain invalid after adding error")
		}

		ctx.AddValidationError("Invalid module name")
		if len(ctx.ValidationErrors) != 2 {
			t.Errorf("Expected 2 validation errors, got %d", len(ctx.ValidationErrors))
		}
	})

	t.Run("AddValidationWarning", func(t *testing.T) {
		ctx.AddValidationWarning("Large project detected")
		if len(ctx.ValidationWarnings) != 1 {
			t.Errorf("Expected 1 validation warning, got %d", len(ctx.ValidationWarnings))
		}
		if ctx.ValidationWarnings[0] != "Large project detected" {
			t.Errorf("Expected 'Large project detected', got %v", ctx.ValidationWarnings[0])
		}

		ctx.AddValidationWarning("Outdated dependencies")
		if len(ctx.ValidationWarnings) != 2 {
			t.Errorf("Expected 2 validation warnings, got %d", len(ctx.ValidationWarnings))
		}
	})
}

func TestProjectContext_ConfidenceScoring(t *testing.T) {
	ctx := NewProjectContext("go", "/test/project")

	tests := []struct {
		name       string
		input      float64
		expected   float64
	}{
		{
			name:     "normal confidence (0.85)",
			input:    0.85,
			expected: 0.85,
		},
		{
			name:     "high confidence (0.95)",
			input:    0.95,
			expected: 0.95,
		},
		{
			name:     "low confidence (0.25)",
			input:    0.25,
			expected: 0.25,
		},
		{
			name:     "zero confidence",
			input:    0.0,
			expected: 0.0,
		},
		{
			name:     "perfect confidence",
			input:    1.0,
			expected: 1.0,
		},
		{
			name:     "negative confidence (clamped to 0)",
			input:    -0.5,
			expected: 0.0,
		},
		{
			name:     "over confidence (clamped to 1)",
			input:    1.5,
			expected: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx.SetConfidence(tt.input)
			if ctx.Confidence != tt.expected {
				t.Errorf("SetConfidence(%v) = %v, want %v", tt.input, ctx.Confidence, tt.expected)
			}
		})
	}
}

func TestProjectContext_DetectionTime(t *testing.T) {
	ctx := NewProjectContext("go", "/test/project")

	duration := 5 * time.Second
	ctx.SetDetectionTime(duration)

	if ctx.DetectionTime != duration {
		t.Errorf("DetectionTime = %v, want %v", ctx.DetectionTime, duration)
	}
}

func TestProjectContext_DependencyManagement(t *testing.T) {
	ctx := NewProjectContext("go", "/test/project")

	t.Run("Dependencies management", func(t *testing.T) {
		if ctx.Dependencies == nil {
			t.Error("Dependencies map should be initialized")
		}

		ctx.Dependencies["github.com/gin-gonic/gin"] = "v1.9.1"
		ctx.Dependencies["github.com/stretchr/testify"] = "v1.8.4"

		if len(ctx.Dependencies) != 2 {
			t.Errorf("Expected 2 dependencies, got %d", len(ctx.Dependencies))
		}

		ginVersion, exists := ctx.Dependencies["github.com/gin-gonic/gin"]
		if !exists {
			t.Error("gin dependency should exist")
		}
		if ginVersion != "v1.9.1" {
			t.Errorf("gin version = %v, want v1.9.1", ginVersion)
		}
	})

	t.Run("DevDependencies management", func(t *testing.T) {
		if ctx.DevDependencies == nil {
			t.Error("DevDependencies map should be initialized")
		}

		ctx.DevDependencies["golangci-lint"] = "v1.54.2"
		ctx.DevDependencies["air"] = "v1.45.0"

		if len(ctx.DevDependencies) != 2 {
			t.Errorf("Expected 2 dev dependencies, got %d", len(ctx.DevDependencies))
		}

		lintVersion, exists := ctx.DevDependencies["golangci-lint"]
		if !exists {
			t.Error("golangci-lint dev dependency should exist")
		}
		if lintVersion != "v1.54.2" {
			t.Errorf("golangci-lint version = %v, want v1.54.2", lintVersion)
		}
	})
}

func TestGoProjectContext_Structure(t *testing.T) {
	baseCtx := NewProjectContext("go", "/test/go-project")
	goCtx := &GoProjectContext{
		ProjectContext: baseCtx,
		GoVersion:      "1.21.0",
		GoMod: GoModInfo{
			ModuleName: "example.com/myproject",
			GoVersion:  "1.21",
			Requires: map[string]string{
				"github.com/gin-gonic/gin": "v1.9.1",
			},
		},
		GoPath: "/home/user/go",
		GoRoot: "/usr/local/go",
		GoEnv: map[string]string{
			"GOPROXY": "https://proxy.golang.org",
		},
	}

	if goCtx.GoVersion != "1.21.0" {
		t.Errorf("GoVersion = %v, want 1.21.0", goCtx.GoVersion)
	}
	if goCtx.GoMod.ModuleName != "example.com/myproject" {
		t.Errorf("ModuleName = %v, want example.com/myproject", goCtx.GoMod.ModuleName)
	}
	if len(goCtx.GoMod.Requires) != 1 {
		t.Errorf("Expected 1 require, got %d", len(goCtx.GoMod.Requires))
	}
	if goCtx.GoPath != "/home/user/go" {
		t.Errorf("GoPath = %v, want /home/user/go", goCtx.GoPath)
	}
}

func TestPythonProjectContext_Structure(t *testing.T) {
	baseCtx := NewProjectContext("python", "/test/python-project")
	pythonCtx := &PythonProjectContext{
		ProjectContext: baseCtx,
		PythonVersion:  "3.11.0",
		VirtualEnv:     "/test/python-project/.venv",
		Requirements:   []string{"requirements.txt", "requirements-dev.txt"},
		SetupPy: SetupPyInfo{
			Name:         "myproject",
			Version:      "1.0.0",
			Description:  "My Python project",
			Author:       "John Doe",
			Requirements: []string{"requests>=2.28.0", "fastapi>=0.100.0"},
			Packages:     []string{"myproject", "myproject.utils"},
		},
		PyProject: PyProjectInfo{
			Name:        "myproject",
			Version:     "1.0.0",
			Description: "My Python project",
			Authors:     []string{"John Doe <john@example.com>"},
			Dependencies: map[string]string{
				"requests": ">=2.28.0",
				"fastapi":  ">=0.100.0",
			},
			BuildSystem: "poetry",
		},
	}

	if pythonCtx.PythonVersion != "3.11.0" {
		t.Errorf("PythonVersion = %v, want 3.11.0", pythonCtx.PythonVersion)
	}
	if len(pythonCtx.Requirements) != 2 {
		t.Errorf("Expected 2 requirements files, got %d", len(pythonCtx.Requirements))
	}
	if pythonCtx.SetupPy.Name != "myproject" {
		t.Errorf("SetupPy.Name = %v, want myproject", pythonCtx.SetupPy.Name)
	}
	if pythonCtx.PyProject.BuildSystem != "poetry" {
		t.Errorf("PyProject.BuildSystem = %v, want poetry", pythonCtx.PyProject.BuildSystem)
	}
}

func TestNodeJSProjectContext_Structure(t *testing.T) {
	baseCtx := NewProjectContext("nodejs", "/test/nodejs-project")
	nodeCtx := &NodeJSProjectContext{
		ProjectContext: baseCtx,
		NodeVersion:    "18.17.0",
		NPMVersion:     "9.6.7",
		PackageManager: "npm",
		LockFile:       "package-lock.json",
		PackageJSON: PackageJSONInfo{
			Name:        "my-node-project",
			Version:     "1.0.0",
			Description: "My Node.js project",
			Main:        "index.js",
			Scripts: map[string]string{
				"start": "node index.js",
				"test":  "jest",
				"build": "webpack",
			},
			Dependencies: map[string]string{
				"express": "^4.18.0",
				"lodash":  "^4.17.21",
			},
			DevDependencies: map[string]string{
				"jest":    "^29.0.0",
				"webpack": "^5.0.0",
			},
			Engines: map[string]string{
				"node": ">=18.0.0",
				"npm":  ">=9.0.0",
			},
		},
	}

	if nodeCtx.NodeVersion != "18.17.0" {
		t.Errorf("NodeVersion = %v, want 18.17.0", nodeCtx.NodeVersion)
	}
	if nodeCtx.PackageManager != "npm" {
		t.Errorf("PackageManager = %v, want npm", nodeCtx.PackageManager)
	}
	if nodeCtx.PackageJSON.Name != "my-node-project" {
		t.Errorf("PackageJSON.Name = %v, want my-node-project", nodeCtx.PackageJSON.Name)
	}
	if len(nodeCtx.PackageJSON.Scripts) != 3 {
		t.Errorf("Expected 3 scripts, got %d", len(nodeCtx.PackageJSON.Scripts))
	}
}

func TestJavaProjectContext_Structure(t *testing.T) {
	baseCtx := NewProjectContext("java", "/test/java-project")
	javaCtx := &JavaProjectContext{
		ProjectContext: baseCtx,
		JavaVersion:    "17.0.7",
		BuildSystem:    "maven",
		JavaHome:       "/usr/lib/jvm/java-17-openjdk",
		MavenInfo: types.MavenInfo{
			GroupId:    "com.example",
			ArtifactId: "my-java-app",
			Version:    "1.0.0",
			Packaging:  "jar",
			Dependencies: map[string]string{
				"org.springframework.boot:spring-boot-starter": "2.7.0",
			},
			Plugins: []string{"maven-compiler-plugin", "spring-boot-maven-plugin"},
		},
		GradleInfo: types.GradleInfo{
			GroupId: "com.example",
			Version: "1.0.0",
			Dependencies: map[string]string{
				"org.springframework.boot:spring-boot-starter": "2.7.0",
			},
			Plugins: []string{"java", "spring-boot"},
			Tasks:   []string{"build", "test", "bootJar"},
		},
	}

	if javaCtx.JavaVersion != "17.0.7" {
		t.Errorf("JavaVersion = %v, want 17.0.7", javaCtx.JavaVersion)
	}
	if javaCtx.BuildSystem != "maven" {
		t.Errorf("BuildSystem = %v, want maven", javaCtx.BuildSystem)
	}
	if javaCtx.MavenInfo.ArtifactId != "my-java-app" {
		t.Errorf("MavenInfo.ArtifactId = %v, want my-java-app", javaCtx.MavenInfo.ArtifactId)
	}
	if len(javaCtx.GradleInfo.Tasks) != 3 {
		t.Errorf("Expected 3 Gradle tasks, got %d", len(javaCtx.GradleInfo.Tasks))
	}
}

func TestTypeScriptProjectContext_Structure(t *testing.T) {
	baseNodeCtx := NewProjectContext("nodejs", "/test/ts-project")
	nodeCtx := &NodeJSProjectContext{
		ProjectContext: baseNodeCtx,
		NodeVersion:    "18.17.0",
		PackageManager: "npm",
	}

	baseCtx := NewProjectContext("typescript", "/test/ts-project")
	tsCtx := &TypeScriptProjectContext{
		ProjectContext:    baseCtx,
		TypeScriptVersion: "5.1.6",
		NodeJSContext:     nodeCtx,
		TSConfig: TSConfigInfo{
			CompilerOptions: map[string]interface{}{
				"target":    "ES2020",
				"module":    "commonjs",
				"strict":    true,
				"esModuleInterop": true,
			},
			Include: []string{"src/**/*", "tests/**/*"},
			Exclude: []string{"node_modules", "dist"},
			Files:   []string{"src/index.ts"},
			Extends: "@tsconfig/node18/tsconfig.json",
		},
	}

	if tsCtx.TypeScriptVersion != "5.1.6" {
		t.Errorf("TypeScriptVersion = %v, want 5.1.6", tsCtx.TypeScriptVersion)
	}
	if tsCtx.NodeJSContext == nil {
		t.Error("NodeJSContext should not be nil")
	}
	if tsCtx.NodeJSContext.NodeVersion != "18.17.0" {
		t.Errorf("NodeJSContext.NodeVersion = %v, want 18.17.0", tsCtx.NodeJSContext.NodeVersion)
	}
	if len(tsCtx.TSConfig.Include) != 2 {
		t.Errorf("Expected 2 include patterns, got %d", len(tsCtx.TSConfig.Include))
	}
	if tsCtx.TSConfig.Extends != "@tsconfig/node18/tsconfig.json" {
		t.Errorf("TSConfig.Extends = %v, want @tsconfig/node18/tsconfig.json", tsCtx.TSConfig.Extends)
	}
}

func TestProjectContext_ProjectSize(t *testing.T) {
	ctx := NewProjectContext("go", "/test/project")

	ctx.ProjectSize = types.ProjectSize{
		TotalFiles:     1500,
		SourceFiles:    1200,
		TestFiles:      200,
		ConfigFiles:    100,
		TotalSizeBytes: 1024 * 1024 * 50, // 50 MB
	}

	if ctx.ProjectSize.TotalFiles != 1500 {
		t.Errorf("ProjectSize.TotalFiles = %v, want 1500", ctx.ProjectSize.TotalFiles)
	}
	if ctx.ProjectSize.SourceFiles != 1200 {
		t.Errorf("ProjectSize.SourceFiles = %v, want 1200", ctx.ProjectSize.SourceFiles)
	}
	if ctx.ProjectSize.TotalSizeBytes != 1024*1024*50 {
		t.Errorf("ProjectSize.TotalSizeBytes = %v, want %v", ctx.ProjectSize.TotalSizeBytes, 1024*1024*50)
	}
}

func TestProjectContext_MonorepoSupport(t *testing.T) {
	ctx := NewProjectContext("mixed", "/test/monorepo")

	ctx.IsMonorepo = true
	ctx.SubProjects = []string{
		"/test/monorepo/frontend",
		"/test/monorepo/backend",
		"/test/monorepo/shared",
	}

	if !ctx.IsMonorepo {
		t.Error("Should be marked as monorepo")
	}
	if len(ctx.SubProjects) != 3 {
		t.Errorf("Expected 3 sub-projects, got %d", len(ctx.SubProjects))
	}

	subProject := NewProjectContext("react", "/test/monorepo/frontend")
	subProject.ParentProject = "/test/monorepo"

	if subProject.ParentProject != "/test/monorepo" {
		t.Errorf("ParentProject = %v, want /test/monorepo", subProject.ParentProject)
	}
}

func TestProjectContext_CompleteWorkflow(t *testing.T) {
	ctx := NewProjectContext("go", "/test/go-project")

	ctx.AddLanguage("go")
	ctx.SetLanguageVersion("go", "1.21.0")
	ctx.AddRequiredServer("gopls")
	ctx.AddMarkerFile("go.mod")
	ctx.AddMarkerFile("go.sum")
	ctx.SetMetadata("module_name", "example.com/myproject")
	ctx.SetConfidence(0.95)
	ctx.SetDetectionTime(2 * time.Second)

	ctx.SourceDirs = []string{"cmd", "internal", "pkg"}
	ctx.TestDirs = []string{"test"}
	ctx.ConfigFiles = []string{".golangci.yml", "Dockerfile"}
	ctx.BuildFiles = []string{"Makefile", "Dockerfile"}
	ctx.BuildSystem = "make"
	ctx.PackageManager = "go mod"

	ctx.Dependencies["github.com/gin-gonic/gin"] = "v1.9.1"
	ctx.DevDependencies["golangci-lint"] = "v1.54.2"

	ctx.ProjectSize = types.ProjectSize{
		TotalFiles:     500,
		SourceFiles:    400,
		TestFiles:      80,
		ConfigFiles:    20,
		TotalSizeBytes: 1024 * 1024 * 10, // 10 MB
	}

	ctx.VCSType = "git"
	ctx.VCSRoot = "/test/go-project"
	ctx.CurrentBranch = "main"

	ctx.Tags = []string{"api", "microservice", "go"}

	if !ctx.HasLanguage("go") {
		t.Error("Should have Go language")
	}
	if !ctx.HasServer("gopls") {
		t.Error("Should have gopls server")
	}
	if ctx.Confidence != 0.95 {
		t.Errorf("Confidence = %v, want 0.95", ctx.Confidence)
	}
	if len(ctx.SourceDirs) != 3 {
		t.Errorf("Expected 3 source dirs, got %d", len(ctx.SourceDirs))
	}
	if len(ctx.Tags) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(ctx.Tags))
	}

	moduleName, exists := ctx.GetMetadata("module_name")
	if !exists || moduleName != "example.com/myproject" {
		t.Error("Module name metadata should be set correctly")
	}

	goVersion, exists := ctx.GetLanguageVersion("go")
	if !exists || goVersion != "1.21.0" {
		t.Error("Go version should be set correctly")
	}

	if ctx.DetectionTime != 2*time.Second {
		t.Errorf("DetectionTime = %v, want 2s", ctx.DetectionTime)
	}
}