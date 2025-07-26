package installer

import (
	"fmt"
	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/types"
	"strings"
)

func (r *DefaultRuntimeInstaller) generateRecommendations(result *types.VerificationResult) {
	recommendations := []string{}

	issuesByCategory := make(map[types.IssueCategory][]types.Issue)
	criticalIssues := 0
	highIssues := 0

	for _, issue := range result.Issues {
		issuesByCategory[issue.Category] = append(issuesByCategory[issue.Category], issue)

		switch issue.Severity {
		case types.IssueSeverityCritical:
			criticalIssues++
		case types.IssueSeverityHigh:
			highIssues++
		}
	}

	if issues, exists := issuesByCategory[types.IssueCategoryInstallation]; exists {
		recommendations = append(recommendations, r.generateInstallationRecommendations(result.Runtime, issues)...)
	}

	if issues, exists := issuesByCategory[types.IssueCategoryVersion]; exists {
		recommendations = append(recommendations, r.generateVersionRecommendations(result.Runtime, issues)...)
	}

	if issues, exists := issuesByCategory[types.IssueCategoryEnvironment]; exists {
		recommendations = append(recommendations, r.generateEnvironmentRecommendations(result.Runtime, issues)...)
	}

	if issues, exists := issuesByCategory[types.IssueCategoryDependencies]; exists {
		recommendations = append(recommendations, r.generateDependencyRecommendations(result.Runtime, issues)...)
	}

	if issues, exists := issuesByCategory[types.IssueCategoryPermissions]; exists {
		recommendations = append(recommendations, r.generatePermissionRecommendations(result.Runtime, issues)...)
	}

	if issues, exists := issuesByCategory[types.IssueCategoryPath]; exists {
		recommendations = append(recommendations, r.generatePathRecommendations(result.Runtime, issues)...)
	}

	if issues, exists := issuesByCategory[types.IssueCategoryConfiguration]; exists {
		recommendations = append(recommendations, r.generateConfigurationRecommendations(result.Runtime, issues)...)
	}

	if issues, exists := issuesByCategory[types.IssueCategoryCorruption]; exists {
		recommendations = append(recommendations, r.generateCorruptionRecommendations(result.Runtime, issues)...)
	}

	if criticalIssues > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("⚠️ CRITICAL: %d critical issues found. Runtime may not function properly.", criticalIssues))
		recommendations = append(recommendations, r.getCriticalIssueActions(result.Runtime)...)
	}

	if highIssues > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("⚠️ HIGH: %d high-priority issues found. Some functionality may be limited.", highIssues))
	}

	if criticalIssues == 0 && result.Installed && len(result.Runtime) > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("✅ %s runtime is installed and functional.", strings.ToUpper(result.Runtime[:1])+result.Runtime[1:]))
	}

	recommendations = append(recommendations, r.getPlatformSpecificRecommendations(result.Runtime)...)

	result.Recommendations = recommendations
}

func (r *DefaultRuntimeInstaller) generateInstallationRecommendations(runtime string, issues []types.Issue) []string {
	recommendations := []string{}

	for _, issue := range issues {
		if issue.Severity == types.IssueSeverityCritical {
			switch runtime {
			case "go":
				recommendations = append(recommendations, RECOMMENDATION_GO_INSTALL)
				if platform.IsLinux() {
					recommendations = append(recommendations, "Or use package manager: sudo apt install golang-go (Ubuntu/Debian) or sudo dnf install golang (Fedora)")
				} else if platform.IsMacOS() {
					recommendations = append(recommendations, "Or use Homebrew: brew install go")
				} else if platform.IsWindows() {
					recommendations = append(recommendations, "Or use package manager: winget install GoLang.Go or choco install golang")
				}

			case RuntimePython:
				recommendations = append(recommendations, "Install Python from https://python.org/downloads/")
				if platform.IsLinux() {
					recommendations = append(recommendations, "Or use package manager: sudo apt install python3 python3-pip (Ubuntu/Debian) or sudo dnf install python3 python3-pip (Fedora)")
				} else if platform.IsMacOS() {
					recommendations = append(recommendations, "Or use Homebrew: brew install python")
				} else if platform.IsWindows() {
					recommendations = append(recommendations, "Or use package manager: winget install Python.Python.3 or choco install python")
				}

			case RuntimeNodeJS:
				recommendations = append(recommendations, "Install Node.js from https://nodejs.org/")
				if platform.IsLinux() {
					recommendations = append(recommendations, "Or use package manager: sudo apt install nodejs npm (Ubuntu/Debian) or sudo dnf install nodejs npm (Fedora)")
				} else if platform.IsMacOS() {
					recommendations = append(recommendations, "Or use Homebrew: brew install node")
				} else if platform.IsWindows() {
					recommendations = append(recommendations, "Or use package manager: winget install OpenJS.NodeJS or choco install nodejs")
				}

			case RuntimeJava:
				recommendations = append(recommendations, "Install OpenJDK from https://openjdk.org/ or Oracle JDK from https://oracle.com/java/")
				if platform.IsLinux() {
					recommendations = append(recommendations, "Or use package manager: sudo apt install openjdk-17-jdk (Ubuntu/Debian) or sudo dnf install java-17-openjdk-devel (Fedora)")
				} else if platform.IsMacOS() {
					recommendations = append(recommendations, "Or use Homebrew: brew install openjdk@17")
				} else if platform.IsWindows() {
					recommendations = append(recommendations, "Or use package manager: winget install Microsoft.OpenJDK.17 or choco install openjdk")
				}
			}
		}
	}

	return recommendations
}

func (r *DefaultRuntimeInstaller) generateVersionRecommendations(runtime string, issues []types.Issue) []string {
	recommendations := []string{}

	for _, issue := range issues {
		if issue.Severity == types.IssueSeverityHigh || issue.Severity == types.IssueSeverityCritical {
			if currentVersion, exists := issue.Details["current_version"].(string); exists {
				if minVersion, exists := issue.Details["min_version"].(string); exists {
					recommendations = append(recommendations,
						fmt.Sprintf("Upgrade %s from version %s to %s or later", runtime, currentVersion, minVersion))
				}
			}

			switch runtime {
			case "go":
				recommendations = append(recommendations, "Download latest Go from https://golang.org/dl/ and follow upgrade instructions")
			case RuntimePython:
				recommendations = append(recommendations, "Use pyenv to manage Python versions or install latest Python from python.org")
			case RuntimeNodeJS:
				recommendations = append(recommendations, "Use nvm to manage Node.js versions or download latest from nodejs.org")
			case RuntimeJava:
				recommendations = append(recommendations, "Download latest JDK and update JAVA_HOME environment variable")
			}
		}
	}

	return recommendations
}

func (r *DefaultRuntimeInstaller) generateEnvironmentRecommendations(runtime string, issues []types.Issue) []string {
	recommendations := []string{}

	for _, issue := range issues {
		switch runtime {
		case "go":
			if strings.Contains(issue.Description, "GOPATH") {
				recommendations = append(recommendations, "Either create the GOPATH directory or unset GOPATH to use Go modules (recommended)")
			}
			if strings.Contains(issue.Description, "GOROOT") {
				recommendations = append(recommendations, "Unset GOROOT to use the default Go installation path, or set it to a valid Go installation")
			}
		case RuntimePython:
			if strings.Contains(issue.Description, "PYTHONPATH") {
				recommendations = append(recommendations, "Review PYTHONPATH environment variable and ensure all paths exist")
			}
		case RuntimeJava:
			if strings.Contains(issue.Description, "JAVA_HOME") {
				recommendations = append(recommendations, "Set JAVA_HOME to point to your JDK installation directory")
				if platform.IsWindows() {
					recommendations = append(recommendations, "Example: set JAVA_HOME=C:\\Program Files\\Java\\jdk-17")
				} else {
					recommendations = append(recommendations, "Example: export JAVA_HOME=/usr/lib/jvm/java-17-openjdk")
				}
			}
		}
	}

	return recommendations
}

func (r *DefaultRuntimeInstaller) generateDependencyRecommendations(runtime string, issues []types.Issue) []string {
	recommendations := []string{}

	for _, issue := range issues {
		switch runtime {
		case RuntimePython:
			if strings.Contains(issue.Title, "Pip") {
				recommendations = append(recommendations, "Install pip: python -m ensurepip --upgrade")
				if platform.IsLinux() {
					recommendations = append(recommendations, "Or install via package manager: sudo apt install python3-pip")
				}
			}
		case RuntimeNodeJS:
			if strings.Contains(issue.Title, "NPM") {
				recommendations = append(recommendations, "Reinstall Node.js to get npm, or install npm separately")
			}
		case RuntimeJava:
			if strings.Contains(issue.Title, "Compiler") {
				recommendations = append(recommendations, "Install a JDK (not just JRE) to get the Java compiler")
			}
		}
	}

	return recommendations
}

func (r *DefaultRuntimeInstaller) generatePermissionRecommendations(runtime string, issues []types.Issue) []string {
	recommendations := []string{}

	for _, issue := range issues {
		if strings.Contains(issue.Description, "execute permissions") {
			if path, exists := issue.Details["path"].(string); exists {
				recommendations = append(recommendations, fmt.Sprintf("Fix permissions: chmod +x %s", path))
			}
		}

		if strings.Contains(issue.Description, "not writable") {
			switch runtime {
			case RuntimePython:
				recommendations = append(recommendations, "Use virtual environments: python -m venv myenv && source myenv/bin/activate")
			case RuntimeNodeJS:
				recommendations = append(recommendations, "Configure npm prefix: npm config set prefix ~/.npm-global")
				recommendations = append(recommendations, "Add ~/.npm-global/bin to your PATH")
			}
		}
	}

	return recommendations
}

func (r *DefaultRuntimeInstaller) generatePathRecommendations(runtime string, issues []types.Issue) []string {
	recommendations := []string{}

	for _, issue := range issues {
		if strings.Contains(issue.Description, "not found in PATH") {
			recommendations = append(recommendations, fmt.Sprintf("Add %s to your system PATH", runtime))

			if platform.IsWindows() {
				recommendations = append(recommendations, "Use System Properties > Environment Variables to add to PATH")
			} else {
				recommendations = append(recommendations, "Add to ~/.bashrc or ~/.zshrc: export PATH=$PATH:/path/to/runtime/bin")
			}
		}
	}

	return recommendations
}

func (r *DefaultRuntimeInstaller) generateConfigurationRecommendations(_ string, issues []types.Issue) []string {
	recommendations := []string{}

	for _, issue := range issues {
		if strings.Contains(issue.Title, "Version Mismatch") {
			recommendations = append(recommendations, "Ensure all tools are from the same installation")
			recommendations = append(recommendations, "Consider reinstalling to get a consistent toolchain")
		}
	}

	return recommendations
}

func (r *DefaultRuntimeInstaller) generateCorruptionRecommendations(runtime string, _ []types.Issue) []string {
	recommendations := []string{}

	recommendations = append(recommendations, fmt.Sprintf("Reinstall %s to fix corruption issues", runtime))
	recommendations = append(recommendations, "Clear any cached files or configurations")

	switch runtime {
	case RuntimePython:
		recommendations = append(recommendations, "Clear pip cache: pip cache purge")
	case RuntimeNodeJS:
		recommendations = append(recommendations, "Clear npm cache: npm cache clean --force")
	}

	return recommendations
}

func (r *DefaultRuntimeInstaller) getCriticalIssueActions(runtime string) []string {
	actions := []string{
		"🔧 Immediate action required to fix critical issues",
		fmt.Sprintf("1. Verify %s installation", runtime),
		"2. Check PATH environment variable",
		"3. Test basic functionality",
	}

	return actions
}

func (r *DefaultRuntimeInstaller) getPlatformSpecificRecommendations(_ string) []string {
	recommendations := []string{}

	currentPlatform := platform.GetCurrentPlatform()

	switch currentPlatform {
	case platform.PlatformWindows:
		recommendations = append(recommendations, "💡 Windows users: Consider using Windows Package Manager (winget) or Chocolatey for easier installations")
		recommendations = append(recommendations, "💡 Add installation directories to PATH via System Properties > Environment Variables")

	case platform.PlatformMacOS:
		recommendations = append(recommendations, "💡 macOS users: Consider using Homebrew for easier package management")
		recommendations = append(recommendations, "💡 Add installation directories to PATH in ~/.zshrc or ~/.bash_profile")

	case platform.PlatformLinux:
		recommendations = append(recommendations, "💡 Linux users: Use your distribution's package manager when possible")
		recommendations = append(recommendations, "💡 Add installation directories to PATH in ~/.bashrc or ~/.profile")
	}

	return recommendations
}
