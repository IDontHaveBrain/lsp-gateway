package common

import (
	"context"
	"fmt"
	"strings"
	"time"

	"lsp-gateway/internal/platform"
)

type VerificationManager struct {
	timeout time.Duration
}

func NewVerificationManager(timeout time.Duration) *VerificationManager {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &VerificationManager{
		timeout: timeout,
	}
}

type ComponentVerificationConfig struct {
	Name           string
	VersionCommand string
	VersionArgs    []string
	TestCommands   []string // Additional commands to test functionality
	MinVersion     string
	VersionParser  func(string) string // Custom version parsing function
}

type VerificationResult struct {
	ComponentName string                 `json:"component_name"`
	Installed     bool                   `json:"installed"`
	Version       string                 `json:"version"`
	Path          string                 `json:"path"`
	Compatible    bool                   `json:"compatible"`
	Issues        []VerificationIssue    `json:"issues"`
	Details       map[string]interface{} `json:"details"`
	Timestamp     time.Time              `json:"timestamp"`
}

type VerificationIssue struct {
	Severity    string `json:"severity"` // "critical", "warning", "info"
	Category    string `json:"category"` // "installation", "version", "functionality", etc.
	Description string `json:"description"`
	Suggestion  string `json:"suggestion,omitempty"`
}

func (vm *VerificationManager) VerifyComponent(ctx context.Context, config ComponentVerificationConfig) *VerificationResult {
	result := &VerificationResult{
		ComponentName: config.Name,
		Installed:     false,
		Compatible:    false,
		Issues:        []VerificationIssue{},
		Details:       make(map[string]interface{}),
		Timestamp:     time.Now(),
	}

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), vm.timeout)
		defer cancel()
	}

	if !platform.IsCommandAvailable(config.VersionCommand) {
		result.addIssue("critical", "installation",
			fmt.Sprintf("%s command not found", config.VersionCommand),
			fmt.Sprintf("Install %s runtime", config.Name))
		return result
	}

	result.Installed = true

	if path, err := platform.GetCommandPath(config.VersionCommand); err == nil {
		result.Path = path
		result.Details["path"] = path
	}

	version, err := vm.getComponentVersion(ctx, config)
	if err != nil {
		result.addIssue("warning", "version",
			fmt.Sprintf("Failed to get %s version: %v", config.Name, err),
			"Check if the command is working properly")
	} else {
		result.Version = version
		result.Details["version"] = version

		if config.MinVersion != "" {
			compatible, err := vm.isVersionCompatible(version, config.MinVersion)
			if err != nil {
				result.addIssue("warning", "version",
					fmt.Sprintf("Failed to compare versions: %v", err), "")
			} else if !compatible {
				result.addIssue("warning", "version",
					fmt.Sprintf("Version %s does not meet minimum requirement %s", version, config.MinVersion),
					fmt.Sprintf("Upgrade %s to version %s or later", config.Name, config.MinVersion))
			} else {
				result.Compatible = true
			}
		} else {
			result.Compatible = true // Assume compatible if no minimum version specified
		}
	}

	if len(config.TestCommands) > 0 {
		testResults := vm.runTestCommands(ctx, config.TestCommands)
		result.Details["test_results"] = testResults

		for _, testResult := range testResults {
			if !testResult.Success {
				result.addIssue("warning", "functionality",
					fmt.Sprintf("Test command failed: %s", testResult.Command),
					"Check component functionality")
			}
		}
	}

	return result
}

type TestCommandResult struct {
	Command string `json:"command"`
	Success bool   `json:"success"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
}

func (vm *VerificationManager) getComponentVersion(ctx context.Context, config ComponentVerificationConfig) (string, error) {
	executor := platform.NewCommandExecutor()

	args := append([]string{config.VersionCommand}, config.VersionArgs...)
	command := strings.Join(args, " ")

	result, err := platform.ExecuteShellCommandWithContext(executor, ctx, command)
	if err != nil {
		return "", err
	}

	if config.VersionParser != nil {
		return config.VersionParser(result.Stdout), nil
	}

	return vm.parseVersion(result.Stdout), nil
}

func (vm *VerificationManager) parseVersion(output string) string {
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
					if vm.looksLikeVersion(field) {
						return field
					}
				}
			}
		}
	}

	for _, line := range lines {
		fields := strings.Fields(strings.TrimSpace(line))
		for _, field := range fields {
			if vm.looksLikeVersion(field) {
				return field
			}
		}
	}

	return "unknown"
}

func (vm *VerificationManager) looksLikeVersion(s string) bool {
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")

	if strings.Contains(s, ".") {
		parts := strings.Split(s, ".")
		if len(parts) >= 2 {
			for i := 0; i < 2 && i < len(parts); i++ {
				if len(parts[i]) == 0 {
					return false
				}
				hasDigit := false
				for _, r := range parts[i] {
					if r >= '0' && r <= '9' {
						hasDigit = true
						break
					}
				}
				if !hasDigit {
					return false
				}
			}
			return true
		}
	}

	return false
}

func (vm *VerificationManager) runTestCommands(ctx context.Context, commands []string) []TestCommandResult {
	results := make([]TestCommandResult, len(commands))
	executor := platform.NewCommandExecutor()

	for i, command := range commands {
		result := TestCommandResult{
			Command: command,
			Success: false,
		}

		execResult, err := platform.ExecuteShellCommandWithContext(executor, ctx, command)
		if err != nil {
			result.Error = err.Error()
			if execResult != nil {
				result.Output = execResult.Stderr
			}
		} else {
			result.Success = true
			if execResult != nil {
				result.Output = execResult.Stdout
			}
		}

		results[i] = result
	}

	return results
}

func (vm *VerificationManager) isVersionCompatible(version, minVersion string) (bool, error) {
	normalizedVersion := vm.normalizeVersion(version)
	normalizedMinVersion := vm.normalizeVersion(minVersion)

	if normalizedVersion == "" || normalizedMinVersion == "" {
		return true, nil
	}

	return strings.Compare(normalizedVersion, normalizedMinVersion) >= 0, nil
}

func (vm *VerificationManager) normalizeVersion(version string) string {
	version = strings.TrimPrefix(version, "v")
	version = strings.TrimPrefix(version, "V")

	parts := strings.Fields(version)
	if len(parts) > 0 {
		version = parts[0]
	}

	return version
}

func (vr *VerificationResult) addIssue(severity, category, description, suggestion string) {
	vr.Issues = append(vr.Issues, VerificationIssue{
		Severity:    severity,
		Category:    category,
		Description: description,
		Suggestion:  suggestion,
	})
}

func (vr *VerificationResult) HasCriticalIssues() bool {
	for _, issue := range vr.Issues {
		if issue.Severity == "critical" {
			return true
		}
	}
	return false
}

func (vr *VerificationResult) GetSuggestions() []string {
	var suggestions []string
	for _, issue := range vr.Issues {
		if issue.Suggestion != "" {
			suggestions = append(suggestions, issue.Suggestion)
		}
	}
	return suggestions
}

var DefaultVerificationConfigs = map[string]ComponentVerificationConfig{
	"go": {
		Name:           "Go",
		VersionCommand: "go",
		VersionArgs:    []string{"version"},
		TestCommands:   []string{"go env GOROOT", "go env GOPATH"},
		MinVersion:     "1.20",
	},
	"python": {
		Name:           "Python",
		VersionCommand: "python3",
		VersionArgs:    []string{"--version"},
		TestCommands:   []string{"python3 -c \"import sys; print(sys.version)\""},
		MinVersion:     "3.8",
	},
	"nodejs": {
		Name:           "Node.js",
		VersionCommand: "node",
		VersionArgs:    []string{"--version"},
		TestCommands:   []string{"npm --version"},
		MinVersion:     "16.0",
	},
	"java": {
		Name:           "Java",
		VersionCommand: "java",
		VersionArgs:    []string{"-version"},
		TestCommands:   []string{"javac -version"},
		MinVersion:     "11",
	},
}
