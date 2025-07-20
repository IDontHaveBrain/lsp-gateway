package installer

import (
	"fmt"
	"time"
)

type DependencyRegistry struct {
	dependencies map[string]*ServerDependencyInfo
}

func NewDependencyRegistry() *DependencyRegistry {
	registry := &DependencyRegistry{
		dependencies: make(map[string]*ServerDependencyInfo),
	}

	registry.registerDefaults()

	return registry
}

func (r *DependencyRegistry) registerDefaults() {
	r.dependencies["gopls"] = &ServerDependencyInfo{
		ServerName:  "gopls",
		DisplayName: "Go Language Server",
		RequiredRuntime: &RuntimeRequirement{
			Name:               "go",
			MinVersion:         "1.19.0",
			RecommendedVersion: "1.21.0",
			Required:           true,
			Purpose:            "Required to run Go language server and compile Go code",
			InstallationCmd:    []string{"npm", "run", "runtime-install", "go"},
		},
		OptionalRuntimes: []*RuntimeRequirement{},
		AdditionalPackages: []*PackageRequirement{
			{
				Name:            "gopls",
				Runtime:         "go",
				MinVersion:      "latest",
				InstallCommands: []string{"go", "install", "golang.org/x/tools/gopls@latest"},
				Required:        true,
				Purpose:         "The actual Go language server binary",
			},
		},
		EnvironmentVars: map[string]string{
			"GOPATH": "Go workspace path",
			"GOROOT": "Go installation root",
		},
		PreInstallCommands: []string{
			"go", "env", "GOPATH",
			"go", "env", "GOROOT",
		},
		PostInstallCommands: []string{
			"gopls", "version",
		},
		VerificationSteps: []*VerificationStep{
			{
				Name:           "Check gopls installation",
				Command:        []string{"gopls", "version"},
				ExpectedOutput: "gopls",
				FailureMessage: "gopls is not properly installed or not in PATH",
			},
			{
				Name:           "Verify Go environment",
				Command:        []string{"go", "env", "GOPATH"},
				ExpectedOutput: "",
				FailureMessage: "Go environment is not properly configured",
			},
		},
		ConflictsWith: []string{},
		InstallationNotes: []string{
			"Ensure GOPATH and GOROOT environment variables are properly set",
			"gopls works best with Go modules enabled",
			"For private repositories, configure GOPRIVATE environment variable",
		},
	}

	r.dependencies["pylsp"] = &ServerDependencyInfo{
		ServerName:  "pylsp",
		DisplayName: "Python Language Server",
		RequiredRuntime: &RuntimeRequirement{
			Name:               "python",
			MinVersion:         "3.8.0",
			RecommendedVersion: "3.11.0",
			Required:           true,
			Purpose:            "Required to run Python language server and execute Python code",
			InstallationCmd:    []string{"npm", "run", "runtime-install", "python"},
		},
		OptionalRuntimes: []*RuntimeRequirement{},
		AdditionalPackages: []*PackageRequirement{
			{
				Name:            "pip",
				Runtime:         "python",
				MinVersion:      "20.0.0",
				InstallCommands: []string{"python3", "-m", "ensurepip", "--upgrade"},
				Required:        true,
				Purpose:         "Package installer for Python, required to install pylsp",
			},
			{
				Name:            "python-lsp-server",
				Runtime:         "python",
				MinVersion:      "latest",
				InstallCommands: []string{"pip3", "install", "python-lsp-server[all]"},
				Required:        true,
				Purpose:         "The actual Python language server with all optional dependencies",
			},
		},
		EnvironmentVars: map[string]string{
			"PYTHONPATH": "Python module search path",
		},
		PreInstallCommands: []string{
			"python3", "--version",
			"pip3", "--version",
		},
		PostInstallCommands: []string{
			"pylsp", "--version",
		},
		VerificationSteps: []*VerificationStep{
			{
				Name:           "Check pylsp installation",
				Command:        []string{"pylsp", "--version"},
				ExpectedOutput: "pylsp",
				FailureMessage: "pylsp is not properly installed or not in PATH",
			},
			{
				Name:           "Verify pip installation",
				Command:        []string{"pip3", "--version"},
				ExpectedOutput: "pip",
				FailureMessage: "pip3 is not available for package installation",
			},
			{
				Name:           "Test pylsp startup",
				Command:        []string{"pylsp", "--help"},
				ExpectedOutput: "usage:",
				FailureMessage: "pylsp cannot start properly",
			},
		},
		ConflictsWith: []string{"python-language-server", "pyls"},
		InstallationNotes: []string{
			"Install with [all] extras for full functionality including rope, autopep8, flake8",
			"For better performance, consider installing 'python-lsp-server[all]' in a virtual environment",
			"Some features require additional packages like black, isort, mypy",
			"Virtual environment support requires pylsp-mypy plugin",
		},
	}

	r.dependencies["typescript-language-server"] = &ServerDependencyInfo{
		ServerName:  "typescript-language-server",
		DisplayName: "TypeScript Language Server",
		RequiredRuntime: &RuntimeRequirement{
			Name:               "nodejs",
			MinVersion:         "18.0.0",
			RecommendedVersion: "20.0.0",
			Required:           true,
			Purpose:            "Required to run TypeScript language server",
			InstallationCmd:    []string{"npm", "run", "runtime-install", "node"},
		},
		OptionalRuntimes: []*RuntimeRequirement{},
		AdditionalPackages: []*PackageRequirement{
			{
				Name:            "npm",
				Runtime:         "nodejs",
				MinVersion:      "9.0.0",
				InstallCommands: []string{"npm", "install", "-g", "npm@latest"},
				Required:        true,
				Purpose:         "Node.js package manager, required to install TypeScript language server",
			},
			{
				Name:            "typescript",
				Runtime:         "nodejs",
				MinVersion:      "4.0.0",
				InstallCommands: []string{"npm", "install", "-g", "typescript"},
				Required:        true,
				Purpose:         "TypeScript compiler, required for language server functionality",
			},
			{
				Name:            "typescript-language-server",
				Runtime:         "nodejs",
				MinVersion:      "latest",
				InstallCommands: []string{"npm", "install", "-g", "typescript-language-server"},
				Required:        true,
				Purpose:         "The actual TypeScript language server binary",
			},
		},
		EnvironmentVars: map[string]string{
			"NODE_PATH": "Node.js module search path for global packages",
		},
		PreInstallCommands: []string{
			"node", "--version",
			"npm", "--version",
		},
		PostInstallCommands: []string{
			"typescript-language-server", "--version",
			"tsc", "--version",
		},
		VerificationSteps: []*VerificationStep{
			{
				Name:           "Check typescript-language-server installation",
				Command:        []string{"typescript-language-server", "--version"},
				ExpectedOutput: "typescript-language-server",
				FailureMessage: "typescript-language-server is not properly installed or not in PATH",
			},
			{
				Name:           "Check TypeScript compiler",
				Command:        []string{"tsc", "--version"},
				ExpectedOutput: "Version",
				FailureMessage: "TypeScript compiler (tsc) is not available",
			},
			{
				Name:           "Test language server startup",
				Command:        []string{"typescript-language-server", "--stdio"},
				ExpectedOutput: "",
				FailureMessage: "typescript-language-server cannot start in stdio mode",
			},
		},
		ConflictsWith: []string{},
		InstallationNotes: []string{
			"Global installation recommended for system-wide availability",
			"Local project installations may require additional configuration",
			"For JavaScript projects, TypeScript is still required for language server features",
			"Consider installing @types packages for better type definitions",
		},
	}

	r.dependencies["jdtls"] = &ServerDependencyInfo{
		ServerName:  "jdtls",
		DisplayName: "Eclipse JDT Language Server",
		RequiredRuntime: &RuntimeRequirement{
			Name:               "java",
			MinVersion:         "17.0.0",
			RecommendedVersion: "21.0.0",
			Required:           true,
			Purpose:            "Required to run Eclipse JDT Language Server (Java 17+ required)",
			InstallationCmd:    []string{"npm", "run", "runtime-install", "java"},
		},
		OptionalRuntimes: []*RuntimeRequirement{},
		AdditionalPackages: []*PackageRequirement{
			{
				Name:            "javac",
				Runtime:         "java",
				MinVersion:      "17.0.0",
				InstallCommands: []string{"java", "-version"}, // Platform-specific installation
				Required:        true,
				Purpose:         "Java compiler, typically included with JDK",
			},
		},
		EnvironmentVars: map[string]string{
			"JAVA_HOME": "Java installation directory",
			"PATH":      "Must include Java bin directory",
		},
		PreInstallCommands: []string{
			"java", "-version",
			"javac", "-version",
		},
		PostInstallCommands: []string{},
		VerificationSteps: []*VerificationStep{
			{
				Name:           "Check Java runtime",
				Command:        []string{"java", "-version"},
				ExpectedOutput: "java version",
				FailureMessage: "Java runtime is not available or version is too old",
			},
			{
				Name:           "Check Java compiler",
				Command:        []string{"javac", "-version"},
				ExpectedOutput: "javac",
				FailureMessage: "Java compiler (javac) is not available",
			},
			{
				Name:           "Verify Java version compatibility",
				Command:        []string{"java", "-version"},
				ExpectedOutput: "17",
				FailureMessage: "Java version 17 or higher is required for jdtls",
			},
		},
		ConflictsWith: []string{},
		InstallationNotes: []string{
			"Eclipse JDT Language Server requires manual installation from Eclipse website",
			"Java 17 or higher is mandatory (Java 8/11 not supported)",
			"JDK (not just JRE) is required for full functionality",
			"JAVA_HOME environment variable must point to JDK installation",
			"Download jdt-language-server from: https://download.eclipse.org/jdtls/",
			"Extract and configure the executable path in LSP Gateway configuration",
		},
	}
}

func (r *DependencyRegistry) GetServerDependencies(serverName string) (*ServerDependencyInfo, error) {
	if depInfo, exists := r.dependencies[serverName]; exists {
		return depInfo, nil
	}
	return nil, &InstallerError{
		Type:    InstallerErrorTypeNotFound,
		Message: fmt.Sprintf(ERROR_DEPENDENCY_INFO_NOT_FOUND, serverName),
	}
}

func (r *DependencyRegistry) RegisterServerDependencies(serverName string, depInfo *ServerDependencyInfo) {
	r.dependencies[serverName] = depInfo
}

func (r *DependencyRegistry) ListServersWithDependencies() []string {
	names := make([]string, 0, len(r.dependencies))
	for name := range r.dependencies {
		names = append(names, name)
	}
	return names
}

func (r *DependencyRegistry) GetServersByRuntime(runtime string) []*ServerDependencyInfo {
	var servers []*ServerDependencyInfo
	for _, depInfo := range r.dependencies {
		if depInfo.RequiredRuntime != nil && depInfo.RequiredRuntime.Name == runtime {
			servers = append(servers, depInfo)
		}

		for _, optionalRuntime := range depInfo.OptionalRuntimes {
			if optionalRuntime.Name == runtime {
				servers = append(servers, depInfo)
				break
			}
		}
	}
	return servers
}

func (r *DependencyRegistry) GetMissingRuntimesForServers(availableRuntimes []string) map[string][]string {
	missingRuntimes := make(map[string][]string)

	for serverName, depInfo := range r.dependencies {
		if depInfo.RequiredRuntime != nil {
			runtime := depInfo.RequiredRuntime.Name
			if !containsString(availableRuntimes, runtime) {
				if _, exists := missingRuntimes[runtime]; !exists {
					missingRuntimes[runtime] = []string{}
				}
				missingRuntimes[runtime] = append(missingRuntimes[runtime], serverName)
			}
		}
	}

	return missingRuntimes
}

func (r *DependencyRegistry) ValidateServerDependencyDefinition(depInfo *ServerDependencyInfo) []string {
	issues := []string{}

	if depInfo.ServerName == "" {
		issues = append(issues, "server name is required")
	}

	if depInfo.DisplayName == "" {
		issues = append(issues, "display name is required")
	}

	if depInfo.RequiredRuntime == nil {
		issues = append(issues, "required runtime is missing")
	} else {
		if depInfo.RequiredRuntime.Name == "" {
			issues = append(issues, "required runtime name is missing")
		}
		if depInfo.RequiredRuntime.MinVersion == "" {
			issues = append(issues, "required runtime minimum version is missing")
		}
		if len(depInfo.RequiredRuntime.InstallationCmd) == 0 {
			issues = append(issues, "required runtime installation command is missing")
		}
	}

	if len(depInfo.VerificationSteps) == 0 {
		issues = append(issues, "verification steps are missing")
	}

	return issues
}

func (r *DependencyRegistry) GetInstallationComplexity(serverName string) (*InstallationComplexity, error) {
	depInfo, err := r.GetServerDependencies(serverName)
	if err != nil {
		return nil, err
	}

	complexity := &InstallationComplexity{
		ServerName:        serverName,
		Difficulty:        "easy",
		EstimatedTime:     5 * time.Minute,
		StepsRequired:     1,
		ManualSteps:       0,
		PlatformSpecific:  false,
		RequiresDownloads: false,
	}

	requiredPackages := 0
	for _, pkg := range depInfo.AdditionalPackages {
		if pkg.Required {
			requiredPackages++
			complexity.StepsRequired++
		}
	}

	for _, note := range depInfo.InstallationNotes {
		if containsString([]string{"manual", "download", "extract", "configure"}, note) {
			complexity.ManualSteps++
			complexity.RequiresDownloads = true
		}
	}

	if complexity.ManualSteps > 0 {
		complexity.Difficulty = "advanced"
		complexity.EstimatedTime = 15 * time.Minute
	} else if requiredPackages > 2 {
		complexity.Difficulty = "medium"
		complexity.EstimatedTime = 10 * time.Minute
	}

	if serverName == "jdtls" {
		complexity.Difficulty = "advanced"
		complexity.EstimatedTime = 20 * time.Minute
		complexity.ManualSteps = 3
		complexity.RequiresDownloads = true
		complexity.PlatformSpecific = true
	}

	return complexity, nil
}

type InstallationComplexity struct {
	ServerName        string        // Server name
	Difficulty        string        // "easy", "medium", "advanced"
	EstimatedTime     time.Duration // Estimated installation time
	StepsRequired     int           // Number of installation steps
	ManualSteps       int           // Number of manual steps required
	PlatformSpecific  bool          // Whether installation is platform-specific
	RequiresDownloads bool          // Whether manual downloads are required
}

func (r *DependencyRegistry) GetQuickInstallGuide(serverName string) (*QuickInstallGuide, error) {
	depInfo, err := r.GetServerDependencies(serverName)
	if err != nil {
		return nil, err
	}

	complexity, err := r.GetInstallationComplexity(serverName)
	if err != nil {
		return nil, err
	}

	guide := &QuickInstallGuide{
		ServerName:      serverName,
		DisplayName:     depInfo.DisplayName,
		Complexity:      complexity,
		Prerequisites:   []string{},
		InstallSteps:    []string{},
		VerificationCmd: []string{},
		Notes:           depInfo.InstallationNotes,
	}

	if depInfo.RequiredRuntime != nil {
		guide.Prerequisites = append(guide.Prerequisites,
			fmt.Sprintf("%s %s or later", depInfo.RequiredRuntime.Name, depInfo.RequiredRuntime.MinVersion))
	}

	guide.InstallSteps = append(guide.InstallSteps, "1. Ensure runtime dependencies are installed:")
	if depInfo.RequiredRuntime != nil {
		guide.InstallSteps = append(guide.InstallSteps,
			fmt.Sprintf("   npm run runtime-install %s", depInfo.RequiredRuntime.Name))
	}

	guide.InstallSteps = append(guide.InstallSteps, "2. Install language server:")
	for i, pkg := range depInfo.AdditionalPackages {
		if pkg.Required && len(pkg.InstallCommands) > 0 {
			guide.InstallSteps = append(guide.InstallSteps,
				fmt.Sprintf("   %s", pkg.InstallCommands[0]))
			if i == 0 { // Only show first command to keep it simple
				break
			}
		}
	}

	guide.InstallSteps = append(guide.InstallSteps, "3. Verify installation:")
	if len(depInfo.VerificationSteps) > 0 {
		guide.VerificationCmd = depInfo.VerificationSteps[0].Command
		guide.InstallSteps = append(guide.InstallSteps,
			fmt.Sprintf("   %s", depInfo.VerificationSteps[0].Command[0]))
	}

	return guide, nil
}

type QuickInstallGuide struct {
	ServerName      string                  // Server name
	DisplayName     string                  // Human-readable name
	Complexity      *InstallationComplexity // Installation complexity
	Prerequisites   []string                // List of prerequisites
	InstallSteps    []string                // Installation steps
	VerificationCmd []string                // Command to verify installation
	Notes           []string                // Additional notes
}
