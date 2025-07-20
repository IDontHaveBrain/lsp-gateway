package common

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"lsp-gateway/internal/platform"
)

type PackageManagerTemplate struct {
	name      string
	platforms []string
	adminReq  bool
	config    PackageManagerConfig
}

type PackageManagerConfig struct {
	Name             string
	Platforms        []string
	RequiresAdmin    bool
	Command          string
	InstallArgs      []string
	UpdateArgs       []string
	VerifyCommands   map[string]string // component -> verify command
	PackageNames     map[string]string // component -> package name
	InstallTimeout   time.Duration
	VerifyTimeout    time.Duration
	PreInstallSteps  []string // commands to run before installation
	PostInstallSteps []string // commands to run after installation
}

func NewPackageManagerTemplate(config PackageManagerConfig) *PackageManagerTemplate {
	if config.InstallTimeout == 0 {
		config.InstallTimeout = 10 * time.Minute
	}
	if config.VerifyTimeout == 0 {
		config.VerifyTimeout = 30 * time.Second
	}

	return &PackageManagerTemplate{
		name:      config.Name,
		platforms: config.Platforms,
		adminReq:  config.RequiresAdmin,
		config:    config,
	}
}

func (pm *PackageManagerTemplate) IsAvailable() bool {
	currentPlatform := runtime.GOOS
	platformSupported := false
	for _, platform := range pm.config.Platforms {
		if platform == currentPlatform {
			platformSupported = true
			break
		}
	}
	if !platformSupported {
		return false
	}

	return platform.IsCommandAvailable(pm.config.Command)
}

func (pm *PackageManagerTemplate) GetName() string {
	return pm.name
}

func (pm *PackageManagerTemplate) RequiresAdmin() bool {
	return pm.adminReq
}

func (pm *PackageManagerTemplate) GetPlatforms() []string {
	return pm.platforms
}

func (pm *PackageManagerTemplate) Install(component string) error {
	if !pm.IsAvailable() {
		return fmt.Errorf("%s is not available", pm.config.Name)
	}

	packageName := pm.getPackageName(component)

	if err := pm.runSteps(pm.config.PreInstallSteps, "pre-install"); err != nil {
		return fmt.Errorf("pre-install steps failed: %w", err)
	}

	args := append(pm.config.InstallArgs, packageName)
	command := fmt.Sprintf("%s %s", pm.config.Command, strings.Join(args, " "))

	executor := platform.NewCommandExecutor()
	result, err := platform.ExecuteShellCommand(executor, command, pm.config.InstallTimeout)
	if err != nil {
		return fmt.Errorf("failed to install %s via %s: %w (output: %s)",
			component, pm.config.Name, err, result.Stderr)
	}

	if err := pm.runSteps(pm.config.PostInstallSteps, "post-install"); err != nil {
		return fmt.Errorf("post-install steps failed: %w", err)
	}

	return nil
}

func (pm *PackageManagerTemplate) Verify(component string) (*platform.VerificationResult, error) {
	result := &platform.VerificationResult{
		Installed: false,
		Issues:    []string{},
	}

	verifyCmd := pm.getVerifyCommand(component)
	if verifyCmd == "" {
		result.Issues = append(result.Issues, "no verification command available")
		return result, nil
	}

	executor := platform.NewCommandExecutor()
	execResult, err := platform.ExecuteShellCommand(executor, verifyCmd, pm.config.VerifyTimeout)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("verification command failed: %v", err))
		return result, nil
	}

	result.Installed = true
	result.Version = pm.extractVersion(execResult.Stdout)

	cmdName := strings.Fields(verifyCmd)[0]
	if platform.IsCommandAvailable(cmdName) {
		if path, err := exec.LookPath(cmdName); err == nil {
			result.Path = path
		}
	}

	return result, nil
}

func (pm *PackageManagerTemplate) getPackageName(component string) string {
	if packageName, exists := pm.config.PackageNames[component]; exists {
		return packageName
	}
	return component
}

func (pm *PackageManagerTemplate) getVerifyCommand(component string) string {
	return pm.config.VerifyCommands[component]
}

func (pm *PackageManagerTemplate) runSteps(steps []string, stepType string) error {
	executor := platform.NewCommandExecutor()

	for i, step := range steps {
		if step == "" {
			continue
		}

		_, err := platform.ExecuteShellCommand(executor, step, 2*time.Minute)
		if err != nil {
			return fmt.Errorf("%s step %d failed (%s): %w", stepType, i+1, step, err)
		}
	}

	return nil
}

func (pm *PackageManagerTemplate) extractVersion(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		patterns := []string{"version", "Version", "VERSION"}

		for _, pattern := range patterns {
			if strings.Contains(strings.ToLower(line), strings.ToLower(pattern)) {
				fields := strings.Fields(line)
				for _, field := range fields {
					if strings.Contains(field, ".") || strings.HasPrefix(field, "v") {
						return field
					}
				}
			}
		}
	}

	if len(lines) > 0 && lines[0] != "" {
		return lines[0]
	}

	return "unknown"
}

func NewHomebrewManagerConfig() PackageManagerConfig {
	return PackageManagerConfig{
		Name:        "homebrew",
		Platforms:   []string{"darwin"},
		Command:     "brew",
		InstallArgs: []string{"install"},
		PackageNames: map[string]string{
			"go":     "go",
			"python": "python@3.11",
			"nodejs": "node",
			"java":   "openjdk@21",
			"git":    "git",
		},
		VerifyCommands: map[string]string{
			"go":     "go version",
			"python": "python3 --version",
			"nodejs": "node --version",
			"java":   "java -version",
			"git":    "git --version",
		},
	}
}

func NewAptManagerConfig() PackageManagerConfig {
	return PackageManagerConfig{
		Name:            "apt",
		Platforms:       []string{"linux"},
		Command:         "sudo apt",
		RequiresAdmin:   true,
		InstallArgs:     []string{"install", "-y"},
		PreInstallSteps: []string{"sudo apt update"},
		PackageNames: map[string]string{
			"go":     "golang-go",
			"python": "python3 python3-pip python3-venv",
			"nodejs": "nodejs npm",
			"java":   "openjdk-21-jdk",
			"git":    "git",
		},
		VerifyCommands: map[string]string{
			"go":     "go version",
			"python": "python3 --version",
			"nodejs": "node --version",
			"java":   "java -version",
			"git":    "git --version",
		},
		InstallTimeout: 15 * time.Minute,
	}
}

func NewWingetManagerConfig() PackageManagerConfig {
	return PackageManagerConfig{
		Name:        "winget",
		Platforms:   []string{"windows"},
		Command:     "winget",
		InstallArgs: []string{"install", "--silent", "--accept-source-agreements", "--accept-package-agreements"},
		PackageNames: map[string]string{
			"go":     "GoLang.Go",
			"python": "Python.Python.3.11",
			"nodejs": "OpenJS.NodeJS",
			"java":   "Eclipse.Temurin.21.JDK",
			"git":    "Git.Git",
		},
		VerifyCommands: map[string]string{
			"go":     "go version",
			"python": "python --version",
			"nodejs": "node --version",
			"java":   "java -version",
			"git":    "git --version",
		},
		InstallTimeout: 15 * time.Minute,
	}
}

func NewChocolateyManagerConfig() PackageManagerConfig {
	return PackageManagerConfig{
		Name:          "chocolatey",
		Platforms:     []string{"windows"},
		Command:       "choco",
		RequiresAdmin: true,
		InstallArgs:   []string{"install", "-y"},
		PackageNames: map[string]string{
			"go":     "golang",
			"python": "python",
			"nodejs": "nodejs",
			"java":   "openjdk",
			"git":    "git",
		},
		VerifyCommands: map[string]string{
			"go":     "go version",
			"python": "python --version",
			"nodejs": "node --version",
			"java":   "java -version",
			"git":    "git --version",
		},
		InstallTimeout: 15 * time.Minute,
	}
}
