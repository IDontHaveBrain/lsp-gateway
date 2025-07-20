package common

import (
	"fmt"
	"runtime"

	"lsp-gateway/internal/platform"
)

type PlatformManager struct{}

func NewPlatformManager() *PlatformManager {
	return &PlatformManager{}
}

type PlatformInfo struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	Platform     string `json:"platform"` // Combined OS/Architecture
	IsWindows    bool   `json:"is_windows"`
	IsLinux      bool   `json:"is_linux"`
	IsMacOS      bool   `json:"is_macos"`
	IsUnix       bool   `json:"is_unix"`
}

func (pm *PlatformManager) GetPlatformInfo() *PlatformInfo {
	currentPlatform := platform.GetCurrentPlatform()

	return &PlatformInfo{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		Platform:     platform.GetPlatformString(),
		IsWindows:    currentPlatform == platform.PlatformWindows,
		IsLinux:      currentPlatform == platform.PlatformLinux,
		IsMacOS:      currentPlatform == platform.PlatformMacOS,
		IsUnix:       platform.IsUnix(),
	}
}

func (pm *PlatformManager) GetAvailablePackageManagers() []platform.PackageManager {
	var managers []platform.PackageManager

	switch runtime.GOOS {
	case PLATFORM_DARWIN:
		homebrew := platform.NewHomebrewManager()
		if homebrew.IsAvailable() {
			managers = append(managers, homebrew)
		}
	case "linux":
		apt := platform.NewAptManager()
		if apt.IsAvailable() {
			managers = append(managers, apt)
		}
	case "windows":
		winget := platform.NewWingetManager()
		if winget.IsAvailable() {
			managers = append(managers, winget)
		}

		choco := platform.NewChocolateyManager()
		if choco.IsAvailable() {
			managers = append(managers, choco)
		}
	}

	return managers
}

func (pm *PlatformManager) GetBestPackageManager() platform.PackageManager {
	managers := pm.GetAvailablePackageManagers()
	if len(managers) == 0 {
		return nil
	}

	return managers[0]
}

type ExecutorConfig struct {
	Timeout      int  `json:"timeout"`
	RequireAdmin bool `json:"require_admin"`
}

func (pm *PlatformManager) GetPlatformExecutor(config ExecutorConfig) platform.CommandExecutor {
	return platform.NewCommandExecutor()
}

type PlatformCapabilities struct {
	SupportsShell       bool     `json:"supports_shell"`
	PackageManagers     []string `json:"package_managers"`
	DefaultShell        string   `json:"default_shell"`
	ExecutableExtension string   `json:"executable_extension"`
	PathSeparator       string   `json:"path_separator"`
	PathListSeparator   string   `json:"path_list_separator"`
	HomeDirectory       string   `json:"home_directory"`
	TempDirectory       string   `json:"temp_directory"`
}

func (pm *PlatformManager) GetPlatformCapabilities() *PlatformCapabilities {
	managers := pm.GetAvailablePackageManagers()
	managerNames := make([]string, len(managers))
	for i, manager := range managers {
		managerNames[i] = manager.GetName()
	}

	executor := platform.NewCommandExecutor()
	defaultShell := executor.GetShell()

	homeDir, err := platform.GetHomeDirectory()
	if err != nil {
		homeDir = ""
	}

	return &PlatformCapabilities{
		SupportsShell:       platform.SupportsShell(defaultShell),
		PackageManagers:     managerNames,
		DefaultShell:        defaultShell,
		ExecutableExtension: platform.GetExecutableExtension(),
		PathSeparator:       platform.GetPathSeparator(),
		PathListSeparator:   platform.GetPathListSeparator(),
		HomeDirectory:       homeDir,
		TempDirectory:       platform.GetTempDirectory(),
	}
}

type InstallationStrategy struct {
	PackageManager   string   `json:"package_manager"`
	InstallCommands  []string `json:"install_commands"`
	VerifyCommands   []string `json:"verify_commands"`
	RequiresAdmin    bool     `json:"requires_admin"`
	EstimatedTime    string   `json:"estimated_time"`
	Prerequisites    []string `json:"prerequisites"`
	PostInstallSteps []string `json:"post_install_steps"`
}

func (pm *PlatformManager) GetInstallationStrategies(component string) []InstallationStrategy {
	var strategies []InstallationStrategy

	managers := pm.GetAvailablePackageManagers()
	for _, manager := range managers {
		strategy := InstallationStrategy{
			PackageManager: manager.GetName(),
			RequiresAdmin:  manager.RequiresAdmin(),
			EstimatedTime:  "5-10 minutes", // Default estimate
		}

		strategies = append(strategies, strategy)
	}

	return strategies
}

func (pm *PlatformManager) ValidatePlatformRequirements() []ValidationIssue {
	var issues []ValidationIssue

	info := pm.GetPlatformInfo()

	supportedPlatforms := []string{PLATFORM_WINDOWS, PLATFORM_LINUX, PLATFORM_DARWIN}
	if !contains(supportedPlatforms, info.OS) {
		issues = append(issues, ValidationIssue{
			Severity:    "critical",
			Category:    "platform",
			Description: fmt.Sprintf("Unsupported platform: %s", info.OS),
			Suggestion:  "Use a supported platform (Windows, Linux, or macOS)",
		})
	}

	managers := pm.GetAvailablePackageManagers()
	if len(managers) == 0 {
		issues = append(issues, ValidationIssue{
			Severity:    "warning",
			Category:    "package_manager",
			Description: "No package managers available",
			Suggestion:  "Install a package manager for easier component installation",
		})
	}

	executor := platform.NewCommandExecutor()
	defaultShell := executor.GetShell()
	if !platform.SupportsShell(defaultShell) {
		issues = append(issues, ValidationIssue{
			Severity:    "warning",
			Category:    "shell",
			Description: "Shell execution not supported",
			Suggestion:  "Some features may not work without shell support",
		})
	}

	return issues
}

type ValidationIssue struct {
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
}

func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}
