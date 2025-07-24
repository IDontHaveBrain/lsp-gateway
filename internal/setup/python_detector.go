package setup

import (
	"fmt"
	"lsp-gateway/internal/platform"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type PythonDetector struct {
	Executor       platform.CommandExecutor
	VersionChecker *VersionChecker
	Logger         *SetupLogger
}

func NewPythonDetector() *PythonDetector {
	return &PythonDetector{
		Executor:       platform.NewCommandExecutor(),
		VersionChecker: NewVersionChecker(),
		Logger:         NewSetupLogger(nil),
	}
}

func (pd *PythonDetector) DetectPython() (*RuntimeInfo, error) {
	info := &RuntimeInfo{
		Name:       "python",
		Installed:  false,
		Version:    "",
		Compatible: false,
		Path:       "",
		Issues:     []string{},
	}

	pythonCommands := pd.GetPythonCommands()

	var detectedPython *pythonInstallation
	var detectionErrors []string

	for _, cmd := range pythonCommands {
		python, err := pd.detectPythonInstallation(cmd)
		if err != nil {
			detectionErrors = append(detectionErrors, fmt.Sprintf("%s: %v", cmd, err))
			continue
		}

		if python != nil {
			detectedPython = python
			break
		}
	}

	if detectedPython == nil {
		info.Issues = append(info.Issues, "No Python installation found")
		info.Issues = append(info.Issues, detectionErrors...)
		return info, NewDetectionError("python", "detection", "No valid Python installation found", nil)
	}

	info.Installed = true
	info.Version = detectedPython.Version
	info.Path = detectedPython.Path
	info.Compatible = pd.VersionChecker.IsCompatible("python", detectedPython.Version)

	if !info.Compatible {
		minVersion, _ := pd.VersionChecker.GetMinVersion("python")
		info.Issues = append(info.Issues, fmt.Sprintf("Python version %s is below minimum required version %s", detectedPython.Version, minVersion))
	}

	if err := pd.validatePip(detectedPython, info); err != nil {
		pd.Logger.WithError(err).Warn("Pip validation failed")
		info.Issues = append(info.Issues, fmt.Sprintf("Pip validation failed: %v", err))
	}

	if err := pd.validateVirtualEnvironment(detectedPython, info); err != nil {
		pd.Logger.WithError(err).Warn("Virtual environment validation failed")
		info.Issues = append(info.Issues, fmt.Sprintf("Virtual environment support: %v", err))
	}

	pd.performPlatformSpecificChecks(detectedPython, info)

	return info, nil
}

type pythonInstallation struct {
	Command string
	Path    string
	Version string
	Pip     *pipInstallation
}

type pipInstallation struct {
	Command string
	Version string
	Working bool
}

func (pd *PythonDetector) GetPythonCommands() []string {
	if runtime.GOOS == OS_WINDOWS {
		return []string{"py", "python", "python3"}
	}
	return []string{"python3", "python"}
}

func (pd *PythonDetector) detectPythonInstallation(command string) (*pythonInstallation, error) {
	if !pd.Executor.IsCommandAvailable(command) {
		return nil, fmt.Errorf("command '%s' not found in PATH", command)
	}

	version, err := pd.getPythonVersion(command)
	if err != nil {
		return nil, fmt.Errorf("failed to get version: %w", err)
	}

	path, err := pd.getPythonPath(command)
	if err != nil {
		pd.Logger.WithError(err).Debug("Could not determine Python path")
		path = command // fallback to command name
	}

	installation := &pythonInstallation{
		Command: command,
		Path:    path,
		Version: version,
	}

	pip, err := pd.detectPip(command)
	if err != nil {
		pd.Logger.WithError(err).Debug("Pip detection failed")
	} else {
		installation.Pip = pip
	}

	return installation, nil
}

func (pd *PythonDetector) getPythonVersion(command string) (string, error) {
	result, err := pd.Executor.Execute(command, []string{"--version"}, 10*time.Second)
	if err != nil {
		return "", fmt.Errorf("version command failed: %w", err)
	}

	if result.ExitCode != 0 {
		return "", fmt.Errorf("version command returned exit code %d: %s", result.ExitCode, result.Stderr)
	}

	versionOutput := result.Stdout
	if versionOutput == "" {
		versionOutput = result.Stderr
	}

	version, err := pd.ParseVersionString(versionOutput)
	if err != nil {
		return "", fmt.Errorf("failed to parse version from '%s': %w", versionOutput, err)
	}

	return version, nil
}

func (pd *PythonDetector) ParseVersionString(output string) (string, error) {
	re := regexp.MustCompile(`Python\s+(\d+\.\d+\.\d+)`)
	matches := re.FindStringSubmatch(output)

	if len(matches) < 2 {
		re = regexp.MustCompile(`(\d+\.\d+\.\d+)`)
		matches = re.FindStringSubmatch(output)
		if len(matches) < 2 {
			return "", fmt.Errorf("could not extract version from output: %s", output)
		}
	}

	return matches[1], nil
}

func (pd *PythonDetector) getPythonPath(command string) (string, error) {
	result, err := pd.Executor.Execute(command, []string{"-c", "import sys; print(sys.executable)"}, 5*time.Second)
	if err != nil {
		return "", fmt.Errorf("path detection failed: %w", err)
	}

	if result.ExitCode != 0 {
		return "", fmt.Errorf("path detection returned exit code %d: %s", result.ExitCode, result.Stderr)
	}

	path := strings.TrimSpace(result.Stdout)
	if path == "" {
		return "", fmt.Errorf("empty path returned")
	}

	return path, nil
}

func (pd *PythonDetector) detectPip(pythonCommand string) (*pipInstallation, error) {
	pipCommands := []string{
		fmt.Sprintf("%s -m pip", pythonCommand), // python -m pip (recommended)
		"pip3",                                  // pip3 command
		"pip",                                   // pip command
	}

	for _, pipCmd := range pipCommands {
		pip, err := pd.testPipCommand(pipCmd)
		if err == nil && pip != nil {
			return pip, nil
		}
	}

	return nil, fmt.Errorf("no working pip installation found")
}

func (pd *PythonDetector) testPipCommand(pipCommand string) (*pipInstallation, error) {
	parts := strings.Fields(pipCommand)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty pip command")
	}

	var args []string
	if len(parts) > 1 {
		args = parts[1:]
	}
	args = append(args, "--version")

	result, err := pd.Executor.Execute(parts[0], args, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("pip version command failed: %w", err)
	}

	if result.ExitCode != 0 {
		return nil, fmt.Errorf("pip version command returned exit code %d: %s", result.ExitCode, result.Stderr)
	}

	version, err := pd.ParsePipVersion(result.Stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pip version: %w", err)
	}

	return &pipInstallation{
		Command: pipCommand,
		Version: version,
		Working: true,
	}, nil
}

func (pd *PythonDetector) ParsePipVersion(output string) (string, error) {
	re := regexp.MustCompile(`pip\s+(\d+\.\d+(?:\.\d+)?)`)
	matches := re.FindStringSubmatch(output)

	if len(matches) < 2 {
		return "", fmt.Errorf("could not extract pip version from output: %s", output)
	}

	return matches[1], nil
}

func (pd *PythonDetector) validatePip(python *pythonInstallation, info *RuntimeInfo) error {
	if python.Pip == nil {
		return fmt.Errorf("pip not detected")
	}

	if err := pd.testPipList(python.Pip.Command); err != nil {
		return fmt.Errorf("pip list test failed: %w", err)
	}

	if err := pd.testPipInstallDryRun(python.Pip.Command); err != nil {
		return fmt.Errorf("pip install test failed: %w", err)
	}

	return nil
}

func (pd *PythonDetector) testPipList(pipCommand string) error {
	parts := strings.Fields(pipCommand)
	args := append(parts[1:], "list", "--format=freeze")

	result, err := pd.Executor.Execute(parts[0], args, 15*time.Second)
	if err != nil {
		return fmt.Errorf("pip list command failed: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("pip list returned exit code %d: %s", result.ExitCode, result.Stderr)
	}

	return nil
}

func (pd *PythonDetector) testPipInstallDryRun(pipCommand string) error {
	parts := strings.Fields(pipCommand)
	args := append(parts[1:], "install", "--dry-run", "--quiet", "setuptools")

	result, err := pd.Executor.Execute(parts[0], args, 20*time.Second)
	if err != nil {
		return fmt.Errorf("pip install dry-run failed: %w", err)
	}

	if result.ExitCode > 1 {
		return fmt.Errorf("pip install dry-run returned exit code %d: %s", result.ExitCode, result.Stderr)
	}

	return nil
}

func (pd *PythonDetector) validateVirtualEnvironment(python *pythonInstallation, info *RuntimeInfo) error {
	parts := strings.Fields(python.Command)
	args := append(parts[1:], "-m", "venv", "--help")

	result, err := pd.Executor.Execute(parts[0], args, 10*time.Second)
	if err != nil {
		return fmt.Errorf("venv module test failed: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("venv module not available (exit code %d): %s", result.ExitCode, result.Stderr)
	}

	return nil
}

func (pd *PythonDetector) performPlatformSpecificChecks(python *pythonInstallation, info *RuntimeInfo) {
	switch runtime.GOOS {
	case "windows":
		pd.checkWindowsSpecific(python, info)
	case "darwin":
		pd.checkMacOSSpecific(python, info)
	case "linux":
		pd.checkLinuxSpecific(python, info)
	}
}

func (pd *PythonDetector) checkWindowsSpecific(python *pythonInstallation, info *RuntimeInfo) {
	if python.Command == "py" {
		info.Issues = append(info.Issues, "Using Python Launcher (py) - recommended for Windows")
	}

	if python.Command == "python" {
		if pd.Executor.IsCommandAvailable("py") {
			info.Issues = append(info.Issues, "Multiple Python commands available - consider using 'py' launcher")
		}
	}

	if strings.Contains(python.Path, "WindowsApps") {
		info.Issues = append(info.Issues, "Using Windows Store Python - may have limitations for development")
	}
}

func (pd *PythonDetector) checkMacOSSpecific(python *pythonInstallation, info *RuntimeInfo) {
	if strings.Contains(python.Path, "/usr/bin/python") {
		info.Issues = append(info.Issues, "Using system Python - consider using Homebrew Python for development")
	} else if strings.Contains(python.Path, "/opt/homebrew") || strings.Contains(python.Path, "/usr/local") {
		info.Issues = append(info.Issues, "Using Homebrew Python - recommended for development")
	}

	if strings.Contains(python.Path, "/Applications/Xcode.app") {
		info.Issues = append(info.Issues, "Using Xcode Python - consider using Homebrew for better package management")
	}
}

func (pd *PythonDetector) checkLinuxSpecific(python *pythonInstallation, info *RuntimeInfo) {
	if strings.Contains(python.Path, "/usr/bin/python") {
		info.Issues = append(info.Issues, "Using system Python - ensure development packages are installed")
	} else if strings.Contains(python.Path, "/usr/local") {
		info.Issues = append(info.Issues, "Using locally compiled Python")
	}

	if python.Pip != nil {
		parts := strings.Fields(python.Command)
		args := append(parts[1:], "-c", "import sysconfig; print(sysconfig.get_path('include'))")

		result, err := pd.Executor.Execute(parts[0], args, 5*time.Second)
		if err == nil && result.ExitCode == 0 {
			includePath := strings.TrimSpace(result.Stdout)
			info.Issues = append(info.Issues, fmt.Sprintf("Python development headers location: %s", includePath))
		}
	}
}
