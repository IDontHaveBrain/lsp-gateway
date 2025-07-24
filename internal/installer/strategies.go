package installer

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"lsp-gateway/internal/platform"
)

type PlatformStrategy interface {
	InstallGo(version string) error

	InstallPython(version string) error

	InstallNodejs(version string) error

	InstallJava(version string) error

	GetPlatformInfo() *PlatformInfo

	GetPackageManagers() []string

	IsPackageManagerAvailable(manager string) bool
}

type PlatformInfo struct {
	OS           string
	Architecture string
	Distribution string
	Version      string
}

type WindowsStrategy struct {
	packageManagers []platform.PackageManager
	executor        platform.CommandExecutor
	retryAttempts   int
	retryDelay      time.Duration
}

func NewWindowsStrategy() *WindowsStrategy {
	return &WindowsStrategy{
		packageManagers: []platform.PackageManager{
			platform.NewWingetManager(),
			platform.NewChocolateyManager(),
		},
		executor:      platform.NewCommandExecutor(),
		retryAttempts: 3,
		retryDelay:    5 * time.Second,
	}
}

func (w *WindowsStrategy) InstallGo(version string) error {
	return w.installComponent("go", version, map[string]string{
		"winget":     "GoLang.Go",
		"chocolatey": "golang",
	})
}

func (w *WindowsStrategy) InstallPython(version string) error {
	return w.installComponent("python", version, map[string]string{
		"winget":     "Python.Python.3.11",
		"chocolatey": "python",
	})
}

func (w *WindowsStrategy) InstallNodejs(version string) error {
	return w.installComponent("nodejs", version, map[string]string{
		"winget":     "OpenJS.NodeJS",
		"chocolatey": "nodejs",
	})
}

func (w *WindowsStrategy) InstallJava(version string) error {
	return w.installComponent("java", version, map[string]string{
		"winget":     "Eclipse.Temurin.17.JDK",
		"chocolatey": "openjdk",
	})
}

func (w *WindowsStrategy) installComponent(component, version string, packageNames map[string]string) error {
	var lastErr error

	for _, manager := range w.packageManagers {
		if !manager.IsAvailable() {
			continue
		}

		managerName := manager.GetName()
		packageName, exists := packageNames[managerName]
		if !exists {
			continue
		}

		err := w.installWithRetry(manager, component, packageName)
		if err == nil {
			result, verifyErr := manager.Verify(component)
			if verifyErr != nil {
				lastErr = NewVerificationError(component, "post-install", "verified installation", "verification failed", verifyErr)
				continue
			}

			if !result.Installed {
				lastErr = NewVerificationError(component, "post-install", "installed", "not installed", nil)
				continue
			}

			return nil
		}

		lastErr = err
	}

	if lastErr != nil {
		return NewInstallerError(InstallerErrorTypeInstallation, component,
			"all package managers failed", lastErr)
	}

	return NewInstallerError(InstallerErrorTypeUnsupported, component,
		"no suitable package managers available", nil)
}

func (w *WindowsStrategy) installWithRetry(manager platform.PackageManager, component, packageName string) error {
	var lastErr error

	for attempt := 1; attempt <= w.retryAttempts; attempt++ {
		err := manager.Install(packageName)
		if err == nil {
			return nil
		}

		lastErr = err

		if w.isNonRetryableError(err) {
			break
		}

		if attempt < w.retryAttempts {
			time.Sleep(w.retryDelay)
		}
	}

	return NewInstallationError(component, "install", 0,
		fmt.Sprintf("failed after %d attempts with %s", w.retryAttempts, manager.GetName()), lastErr)
}

func (w *WindowsStrategy) isNonRetryableError(err error) bool {
	errStr := strings.ToLower(err.Error())

	if strings.Contains(errStr, "access denied") ||
		strings.Contains(errStr, "permission") ||
		strings.Contains(errStr, "administrator") {
		return true
	}

	if strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "no packages found") ||
		strings.Contains(errStr, "no package") {
		return true
	}

	if strings.Contains(errStr, "invalid") ||
		strings.Contains(errStr, "bad argument") {
		return true
	}

	return false
}

func (w *WindowsStrategy) GetPlatformInfo() *PlatformInfo {
	info := &PlatformInfo{
		OS:           "windows",
		Distribution: "windows",
	}

	result, err := w.executor.Execute("wmic", []string{"os", "get", "osarchitecture", "/value"}, 10*time.Second)
	if err == nil && result.ExitCode == 0 {
		if strings.Contains(strings.ToLower(result.Stdout), "64") {
			info.Architecture = "amd64"
		} else {
			info.Architecture = "386"
		}
	} else {
		if arch := getEnvWithExecutor(w.executor, "PROCESSOR_ARCHITECTURE"); arch != "" {
			if strings.Contains(strings.ToUpper(arch), "64") {
				info.Architecture = "amd64"
			} else {
				info.Architecture = "386"
			}
		}
	}

	result, err = w.executor.Execute("ver", []string{}, 10*time.Second)
	if err == nil && result.ExitCode == 0 {
		info.Version = strings.TrimSpace(result.Stdout)
	}

	return info
}

func (w *WindowsStrategy) GetPackageManagers() []string {
	var managers []string
	for _, mgr := range w.packageManagers {
		managers = append(managers, mgr.GetName())
	}
	return managers
}

func (w *WindowsStrategy) IsPackageManagerAvailable(manager string) bool {
	for _, mgr := range w.packageManagers {
		if mgr.GetName() == manager {
			return mgr.IsAvailable()
		}
	}
	return false
}

// Test helper methods for WindowsStrategy

// InstallComponent installs a component using the appropriate package manager (exported for testing)
func (w *WindowsStrategy) InstallComponent(component, version string, packageNames map[string]string) error {
	return w.installComponent(component, version, packageNames)
}

// GetPackageManagerInstances returns the package managers for testing
func (w *WindowsStrategy) GetPackageManagerInstances() []platform.PackageManager {
	return w.packageManagers
}

// SetPackageManagers sets the package managers for testing
func (w *WindowsStrategy) SetPackageManagers(managers []platform.PackageManager) {
	w.packageManagers = managers
}

// GetExecutor returns the command executor for testing
func (w *WindowsStrategy) GetExecutor() platform.CommandExecutor {
	return w.executor
}

// SetExecutor sets the command executor for testing
func (w *WindowsStrategy) SetExecutor(executor platform.CommandExecutor) {
	w.executor = executor
}

// SetRetryAttempts sets the retry attempts for testing
func (w *WindowsStrategy) SetRetryAttempts(attempts int) {
	w.retryAttempts = attempts
}

// SetRetryDelay sets the retry delay for testing
func (w *WindowsStrategy) SetRetryDelay(delay time.Duration) {
	w.retryDelay = delay
}

func getEnvWithExecutor(executor platform.CommandExecutor, varName string) string {
	shell := executor.GetShell()
	var command string

	if shell == CommandCmd {
		command = fmt.Sprintf("echo %%%s%%", varName)
	} else {
		command = fmt.Sprintf("$env:%s", varName)
	}

	args := executor.GetShellArgs(command)
	result, err := executor.Execute(shell, args, 5*time.Second)
	if err != nil || result.ExitCode != 0 {
		return ""
	}

	return strings.TrimSpace(result.Stdout)
}

type MacOSStrategy struct {
	packageManagers []string
	packageMgr      platform.PackageManager
	executor        platform.CommandExecutor
	platformInfo    *PlatformInfo
}

func NewMacOSStrategy() *MacOSStrategy {
	strategy := &MacOSStrategy{
		packageManagers: []string{CommandBrew},
		executor:        platform.NewCommandExecutor(),
	}

	strategy.packageMgr = platform.NewHomebrewManager()

	strategy.platformInfo = &PlatformInfo{
		OS:           "darwin",
		Architecture: platform.GetCurrentArchitecture().String(),
		Distribution: "macos",
		Version:      strategy.getMacOSVersion(),
	}

	return strategy
}

func (m *MacOSStrategy) InstallGo(version string) error {
	if err := m.ensureHomebrew(); err != nil {
		return NewInstallerError(InstallerErrorTypeInstallation, "go", "homebrew setup failed", err)
	}

	packageName := "go"
	if version != "" && version != VersionLatest {
		versionedPackage := fmt.Sprintf("go@%s", m.normalizeGoVersion(version))
		if m.checkBrewFormulaExists(versionedPackage) {
			packageName = versionedPackage
		}
	}

	return m.installWithRetry("go", packageName)
}

func (m *MacOSStrategy) InstallPython(version string) error {
	if err := m.ensureHomebrew(); err != nil {
		return NewInstallerError(InstallerErrorTypeInstallation, "python", "homebrew setup failed", err)
	}

	packageName := m.getPythonPackageName(version)

	if err := m.installWithRetry("python", packageName); err != nil {
		return err
	}

	return m.setupPythonEnvironment()
}

func (m *MacOSStrategy) InstallNodejs(version string) error {
	if err := m.ensureHomebrew(); err != nil {
		return NewInstallerError(InstallerErrorTypeInstallation, "nodejs", "homebrew setup failed", err)
	}

	packageName := CommandNode
	if version != "" && version != VersionLatest {
		majorVersion := m.extractMajorVersion(version)
		versionedPackage := fmt.Sprintf("node@%s", majorVersion)
		if m.checkBrewFormulaExists(versionedPackage) {
			packageName = versionedPackage
		}
	}

	return m.installWithRetry("nodejs", packageName)
}

func (m *MacOSStrategy) InstallJava(version string) error {
	if err := m.ensureHomebrew(); err != nil {
		return NewInstallerError(InstallerErrorTypeInstallation, "java", "homebrew setup failed", err)
	}

	packageName := m.getJavaPackageName(version)

	if err := m.installWithRetry("java", packageName); err != nil {
		return err
	}

	return m.setupJavaEnvironment(version)
}

func (m *MacOSStrategy) GetPlatformInfo() *PlatformInfo {
	return m.platformInfo
}

func (m *MacOSStrategy) GetPackageManagers() []string {
	return m.packageManagers
}

func (m *MacOSStrategy) IsPackageManagerAvailable(manager string) bool {
	switch manager {
	case CommandBrew, PackageManagerHomebrew:
		return m.packageMgr.IsAvailable()
	default:
		return false
	}
}

func (m *MacOSStrategy) ensureHomebrew() error {
	if m.packageMgr.IsAvailable() {
		return nil
	}

	return m.installHomebrew()
}

func (m *MacOSStrategy) installHomebrew() error {
	installScript := `/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`

	result, err := platform.ExecuteShellCommand(m.executor, installScript, 10*time.Minute)
	if err != nil {
		return NewInstallationError("homebrew", "install", result.ExitCode, result.Stderr, err)
	}

	return m.updateHomebrewPath()
}

func (m *MacOSStrategy) updateHomebrewPath() error {
	arch := platform.GetCurrentArchitecture()
	var brewPath string

	switch arch {
	case platform.ArchARM64:
		brewPath = "/opt/homebrew/bin"
	case platform.ArchAMD64:
		brewPath = PathUsrLocalBin
	default:
		brewPath = PathUsrLocalBin // fallback
	}

	currentPath := os.Getenv(ENV_VAR_PATH)
	if !strings.Contains(currentPath, brewPath) {
		newPath := brewPath + ":" + currentPath
		if err := os.Setenv(ENV_VAR_PATH, newPath); err != nil {
			return fmt.Errorf("failed to set PATH environment variable: %w", err)
		}
	}

	return nil
}

func (m *MacOSStrategy) installWithRetry(component, packageName string) error {
	const maxRetries = 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := m.packageMgr.Install(packageName)
		if err == nil {
			if result, verifyErr := m.packageMgr.Verify(component); verifyErr == nil && result.Installed {
				return nil
			}
		}

		lastErr = err
		if attempt < maxRetries {
			waitTime := time.Duration(attempt*attempt) * time.Second
			time.Sleep(waitTime)

			m.updateHomebrew()
		}
	}

	return NewInstallerError(InstallerErrorTypeInstallation, component,
		fmt.Sprintf("failed after %d attempts", maxRetries), lastErr)
}

func (m *MacOSStrategy) updateHomebrew() {
	if _, err := platform.ExecuteShellCommand(m.executor, "brew update", 5*time.Minute); err != nil {
	}
}

func (m *MacOSStrategy) checkBrewFormulaExists(formula string) bool {
	result, err := platform.ExecuteShellCommand(m.executor, fmt.Sprintf("brew search --formula %s", formula), 30*time.Second)
	return err == nil && strings.Contains(result.Stdout, formula)
}

func (m *MacOSStrategy) getPythonPackageName(version string) string {
	if version == "" || version == VersionLatest {
		return HOMEBREW_PYTHON_3_11 // Default to stable version
	}

	majorMinor := m.extractMajorMinorVersion(version)
	packageName := fmt.Sprintf("python@%s", majorMinor)

	if m.checkBrewFormulaExists(packageName) {
		return packageName
	}

	return "python@3.11" // fallback
}

func (m *MacOSStrategy) getJavaPackageName(version string) string {
	if version == "" || version == VersionLatest {
		return HOMEBREW_OPENJDK_21 // Default to LTS version
	}

	majorVersion := m.extractMajorVersion(version)
	packageName := fmt.Sprintf("openjdk@%s", majorVersion)

	if m.checkBrewFormulaExists(packageName) {
		return packageName
	}

	return HOMEBREW_OPENJDK_21 // fallback
}

func (m *MacOSStrategy) setupPythonEnvironment() error {
	commands := []string{
		"python3 -m ensurepip --upgrade",
		"python3 -m pip install --upgrade pip",
	}

	for _, cmd := range commands {
		if _, err := platform.ExecuteShellCommand(m.executor, cmd, 2*time.Minute); err != nil {
			continue
		}
	}

	return nil
}

func (m *MacOSStrategy) setupJavaEnvironment(version string) error {
	majorVersion := m.extractMajorVersion(version)
	if majorVersion == "" {
		majorVersion = "21" // fallback
	}

	result, err := platform.ExecuteShellCommand(m.executor,
		fmt.Sprintf("brew --prefix openjdk@%s", majorVersion), 30*time.Second)
	if err != nil {
		return nil // Don't fail for JAVA_HOME setup issues
	}

	javaHome := strings.TrimSpace(result.Stdout)
	if javaHome != "" {
		if err := os.Setenv(ENV_VAR_JAVA_HOME, javaHome); err != nil {
			// Log error but don't fail completely since JAVA_HOME is not critical
			log.Printf("Warning: Failed to set JAVA_HOME environment variable: %v", err)
		}

		javaBin := filepath.Join(javaHome, "bin")
		currentPath := os.Getenv(ENV_VAR_PATH)
		if !strings.Contains(currentPath, javaBin) {
			newPath := javaBin + ":" + currentPath
			if err := os.Setenv(ENV_VAR_PATH, newPath); err != nil {
				// Log error but don't fail completely since PATH update is not critical for basic Java installation
				log.Printf("Warning: Failed to update PATH environment variable: %v", err)
			}
		}
	}

	return nil
}

func (m *MacOSStrategy) getMacOSVersion() string {
	result, err := platform.ExecuteShellCommand(m.executor, "sw_vers -productVersion", 10*time.Second)
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(result.Stdout)
}

func (m *MacOSStrategy) normalizeGoVersion(version string) string {
	version = strings.TrimPrefix(version, "go")
	return m.extractMajorMinorVersion(version)
}

func (m *MacOSStrategy) extractMajorVersion(version string) string {
	re := regexp.MustCompile(`^(\d+)`)
	matches := re.FindStringSubmatch(version)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func (m *MacOSStrategy) extractMajorMinorVersion(version string) string {
	re := regexp.MustCompile(`^(\d+\.\d+)`)
	matches := re.FindStringSubmatch(version)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// Test helper methods for MacOSStrategy

// GetExecutor returns the command executor for testing
func (m *MacOSStrategy) GetExecutor() platform.CommandExecutor {
	return m.executor
}

// SetExecutor sets the command executor for testing
func (m *MacOSStrategy) SetExecutor(executor platform.CommandExecutor) {
	m.executor = executor
}

// GetPackageManagerInstance returns the package manager for testing (singular)
func (m *MacOSStrategy) GetPackageManagerInstance() platform.PackageManager {
	return m.packageMgr
}

// SetPackageManager sets the package manager for testing
func (m *MacOSStrategy) SetPackageManager(mgr platform.PackageManager) {
	m.packageMgr = mgr
}

// InstallWithRetry exposes the internal installWithRetry method for testing
func (m *MacOSStrategy) InstallWithRetry(component, packageName string) error {
	return m.installWithRetry(component, packageName)
}

// ExtractMajorVersion exposes the internal extractMajorVersion method for testing
func (m *MacOSStrategy) ExtractMajorVersion(version string) string {
	return m.extractMajorVersion(version)
}

// ExtractMajorMinorVersion exposes the internal extractMajorMinorVersion method for testing
func (m *MacOSStrategy) ExtractMajorMinorVersion(version string) string {
	return m.extractMajorMinorVersion(version)
}

// NormalizeGoVersion exposes the internal normalizeGoVersion method for testing
func (m *MacOSStrategy) NormalizeGoVersion(version string) string {
	return m.normalizeGoVersion(version)
}

// GetPythonPackageName exposes the internal getPythonPackageName method for testing
func (m *MacOSStrategy) GetPythonPackageName(version string) string {
	return m.getPythonPackageName(version)
}

// SetupJavaEnvironment exposes the internal setupJavaEnvironment method for testing
func (m *MacOSStrategy) SetupJavaEnvironment(version string) error {
	return m.setupJavaEnvironment(version)
}

// UpdateHomebrewPath exposes the internal updateHomebrewPath method for testing
func (m *MacOSStrategy) UpdateHomebrewPath() error {
	return m.updateHomebrewPath()
}

type LinuxStrategy struct {
	linuxInfo       *platform.LinuxInfo
	packageManagers map[string]platform.PackageManager
	executor        platform.CommandExecutor
	retryAttempts   int
	retryDelay      time.Duration
}

func NewLinuxStrategy() (*LinuxStrategy, error) {
	linuxInfo, err := platform.DetectLinuxDistribution()
	if err != nil {
		return nil, NewInstallerError(InstallerErrorTypeUnsupported, "linux",
			"failed to detect Linux distribution", err)
	}

	packageManagers := make(map[string]platform.PackageManager)

	managers := []platform.PackageManager{
		platform.NewAptManager(),
		platform.NewYumManager(),
		platform.NewDnfManager(),
	}

	for _, mgr := range managers {
		if mgr.IsAvailable() {
			packageManagers[mgr.GetName()] = mgr
		}
	}

	if len(packageManagers) == 0 {
		return nil, NewInstallerError(InstallerErrorTypeUnsupported, "linux",
			"no supported package managers found", nil)
	}

	return &LinuxStrategy{
		linuxInfo:       linuxInfo,
		packageManagers: packageManagers,
		executor:        platform.NewCommandExecutor(),
		retryAttempts:   3,
		retryDelay:      2 * time.Second,
	}, nil
}

func (l *LinuxStrategy) InstallGo(version string) error {
	return l.installComponent("go", version)
}

func (l *LinuxStrategy) InstallPython(version string) error {
	return l.installComponent("python", version)
}

func (l *LinuxStrategy) InstallNodejs(version string) error {
	return l.installComponent("nodejs", version)
}

func (l *LinuxStrategy) InstallJava(version string) error {
	return l.installComponent("java", version)
}

func (l *LinuxStrategy) installComponent(component, version string) error {
	preferredManagers := platform.GetPreferredPackageManagers(l.linuxInfo.Distribution)

	var lastErr error

	for _, managerName := range preferredManagers {
		if mgr, exists := l.packageManagers[managerName]; exists {
			if err := l.installWithRetry(mgr, component, version); err != nil {
				lastErr = err
				continue
			}
			return nil
		}
	}

	for _, mgr := range l.packageManagers {
		if err := l.installWithRetry(mgr, component, version); err != nil {
			lastErr = err
			continue
		}
		return nil
	}

	if lastErr != nil {
		return NewInstallerError(InstallerErrorTypeInstallation, component,
			fmt.Sprintf("all package managers failed to install %s", component), lastErr)
	}

	return NewInstallerError(InstallerErrorTypeInstallation, component,
		"no package managers available", nil)
}

func (l *LinuxStrategy) installWithRetry(mgr platform.PackageManager, component, version string) error {
	var lastErr error

	for attempt := 0; attempt < l.retryAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(l.retryDelay)
		}

		if result, err := mgr.Verify(component); err == nil && result.Installed {
			if version != "" && version != VersionLatest {
				if !l.isVersionCompatible(result.Version, version) {
				} else {
					return nil // Compatible version already installed
				}
			} else {
				return nil // Any version is acceptable
			}
		}

		err := mgr.Install(component)
		if err == nil {
			if result, verifyErr := mgr.Verify(component); verifyErr == nil && result.Installed {
				return nil
			} else {
				lastErr = NewVerificationError(component, "post-install", "installed", "not found", verifyErr)
			}
		} else {
			lastErr = err
		}
	}

	return NewInstallationError(component, "install", -1,
		fmt.Sprintf("failed after %d attempts with %s", l.retryAttempts, mgr.GetName()), lastErr)
}

func (l *LinuxStrategy) isVersionCompatible(installed, required string) bool {
	if required == "" || required == VersionLatest {
		return true
	}

	return strings.Contains(installed, required)
}

func (l *LinuxStrategy) GetPlatformInfo() *PlatformInfo {
	return &PlatformInfo{
		OS:           "linux",
		Architecture: string(platform.GetCurrentArchitecture()),
		Distribution: string(l.linuxInfo.Distribution),
		Version:      l.linuxInfo.Version,
	}
}

func (l *LinuxStrategy) GetPackageManagers() []string {
	var managers []string
	for name := range l.packageManagers {
		managers = append(managers, name)
	}
	return managers
}

func (l *LinuxStrategy) IsPackageManagerAvailable(manager string) bool {
	_, exists := l.packageManagers[manager]
	return exists
}

// Test helper methods for LinuxStrategy

// GetPackageManagerInstances returns the package managers map for testing
func (l *LinuxStrategy) GetPackageManagerInstances() map[string]platform.PackageManager {
	return l.packageManagers
}

// SetPackageManagers sets the package managers for testing
func (l *LinuxStrategy) SetPackageManagers(managers map[string]platform.PackageManager) {
	l.packageManagers = managers
}

// ClearPackageManagers clears all package managers for testing
func (l *LinuxStrategy) ClearPackageManagers() {
	l.packageManagers = make(map[string]platform.PackageManager)
}

// SetRetryAttempts sets the retry attempts for testing
func (l *LinuxStrategy) SetRetryAttempts(attempts int) {
	l.retryAttempts = attempts
}

// InstallWithRetry exposes the internal installWithRetry method for testing
func (l *LinuxStrategy) InstallWithRetry(mgr platform.PackageManager, component, version string) error {
	return l.installWithRetry(mgr, component, version)
}

// NewTestLinuxStrategy creates a LinuxStrategy for testing with custom dependencies
func NewTestLinuxStrategy(linuxInfo *platform.LinuxInfo, executor platform.CommandExecutor) *LinuxStrategy {
	return &LinuxStrategy{
		linuxInfo:       linuxInfo,
		packageManagers: make(map[string]platform.PackageManager),
		executor:        executor,
		retryAttempts:   3,
		retryDelay:      2 * time.Second,
	}
}
