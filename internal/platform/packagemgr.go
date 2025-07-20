package platform

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type PackageManager interface {
	IsAvailable() bool

	Install(component string) error

	Verify(component string) (*VerificationResult, error)

	GetName() string

	RequiresAdmin() bool

	GetPlatforms() []string
}

type VerificationResult struct {
	Installed bool     `json:"installed"`
	Version   string   `json:"version"`
	Path      string   `json:"path"`
	Issues    []string `json:"issues"`
}

type Component struct {
	Name           string            `json:"name"`
	PackageNames   map[string]string `json:"package_names"`   // package_manager -> package_name
	VerifyCommands []string          `json:"verify_commands"` // commands to verify installation
	PostInstall    []string          `json:"post_install"`    // post-installation commands
	MinVersion     string            `json:"min_version"`     // minimum required version
	VersionCommand string            `json:"version_command"` // command to check version
	VersionPattern string            `json:"version_pattern"` // regex pattern to extract version
}

type InstallationStrategy struct {
	PackageManager string    `json:"package_manager"`
	Component      Component `json:"component"`
	Priority       int       `json:"priority"` // lower number = higher priority
	EstimatedTime  string    `json:"estimated_time"`
}

type BasePackageManager struct {
	name      string
	platforms []string
	adminReq  bool
}

func (b *BasePackageManager) GetName() string {
	return b.name
}

func (b *BasePackageManager) RequiresAdmin() bool {
	return b.adminReq
}

func (b *BasePackageManager) GetPlatforms() []string {
	return b.platforms
}

func execCommand(command string, timeout time.Duration) (string, error) {
	executor := NewCommandExecutor()
	result, err := ExecuteShellCommand(executor, command, timeout)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Stdout), nil
}

func extractVersion(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		patterns := []string{
			"version",
			"Version",
			"VERSION",
		}

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

type HomebrewManager struct {
	BasePackageManager
}

func NewHomebrewManager() *HomebrewManager {
	return &HomebrewManager{
		BasePackageManager: BasePackageManager{
			name:      "homebrew",
			platforms: []string{"darwin"},
			adminReq:  false,
		},
	}
}

func (h *HomebrewManager) IsAvailable() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	return IsCommandAvailable(PACKAGE_MANAGER_BREW)
}

func (h *HomebrewManager) Install(component string) error {
	if !h.IsAvailable() {
		return fmt.Errorf("homebrew is not available")
	}

	packageName := h.getPackageName(component)
	command := fmt.Sprintf("brew install %s", packageName)

	_, err := execCommand(command, 10*time.Minute)
	if err != nil {
		return fmt.Errorf("failed to install %s via homebrew: %w", component, err)
	}

	return nil
}

func (h *HomebrewManager) Verify(component string) (*VerificationResult, error) {
	result := &VerificationResult{
		Installed: false,
		Issues:    []string{},
	}

	verifyCmd := h.getVerifyCommand(component)
	if verifyCmd == "" {
		result.Issues = append(result.Issues, "no verification command available")
		return result, nil
	}

	output, err := execCommand(verifyCmd, 30*time.Second)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("verification command failed: %v", err))
		return result, nil
	}

	result.Installed = true
	result.Version = extractVersion(output)

	cmdName := strings.Fields(verifyCmd)[0]
	if IsCommandAvailable(cmdName) {
		if path, err := exec.LookPath(cmdName); err == nil {
			result.Path = path
		}
	}

	return result, nil
}

func (h *HomebrewManager) getPackageName(component string) string {
	packageMap := map[string]string{
		"go":     "go",
		"python": "python@3.11",
		"nodejs": "node",
		"java":   "openjdk@21",
		"git":    "git",
	}

	if pkg, exists := packageMap[component]; exists {
		return pkg
	}
	return component
}

func (h *HomebrewManager) getVerifyCommand(component string) string {
	cmdMap := map[string]string{
		"go":     "go version",
		"python": "python3 --version",
		"nodejs": "node --version",
		"java":   "java -version",
		"git":    "git --version",
	}

	return cmdMap[component]
}

type AptManager struct {
	BasePackageManager
}

func NewAptManager() *AptManager {
	return &AptManager{
		BasePackageManager: BasePackageManager{
			name:      "apt",
			platforms: []string{"linux"},
			adminReq:  true,
		},
	}
}

func (a *AptManager) IsAvailable() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	return IsCommandAvailable("apt") || IsCommandAvailable("apt-get")
}

func (a *AptManager) Install(component string) error {
	if !a.IsAvailable() {
		return fmt.Errorf("apt is not available")
	}

	packageName := a.getPackageName(component)

	updateCmd := "sudo apt update"
	if _, err := execCommand(updateCmd, 5*time.Minute); err != nil {
		return fmt.Errorf("failed to update package list: %w", err)
	}

	installCmd := fmt.Sprintf("sudo apt install -y %s", packageName)
	_, err := execCommand(installCmd, 15*time.Minute)
	if err != nil {
		return fmt.Errorf("failed to install %s via apt: %w", component, err)
	}

	return nil
}

func (a *AptManager) Verify(component string) (*VerificationResult, error) {
	result := &VerificationResult{
		Installed: false,
		Issues:    []string{},
	}

	verifyCmd := a.getVerifyCommand(component)
	if verifyCmd == "" {
		result.Issues = append(result.Issues, "no verification command available")
		return result, nil
	}

	output, err := execCommand(verifyCmd, 30*time.Second)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("verification command failed: %v", err))
		return result, nil
	}

	result.Installed = true
	result.Version = extractVersion(output)

	cmdName := strings.Fields(verifyCmd)[0]
	if IsCommandAvailable(cmdName) {
		if path, err := exec.LookPath(cmdName); err == nil {
			result.Path = path
		}
	}

	return result, nil
}

func (a *AptManager) getPackageName(component string) string {
	packageMap := map[string]string{
		"go":     "golang-go",
		"python": "python3 python3-pip python3-venv",
		"nodejs": "nodejs npm",
		"java":   "openjdk-21-jdk",
		"git":    "git",
	}

	if pkg, exists := packageMap[component]; exists {
		return pkg
	}
	return component
}

func (a *AptManager) getVerifyCommand(component string) string {
	cmdMap := map[string]string{
		"go":     "go version",
		"python": "python3 --version",
		"nodejs": "node --version",
		"java":   "java -version",
		"git":    "git --version",
	}

	return cmdMap[component]
}

type WingetManager struct {
	BasePackageManager
}

func NewWingetManager() *WingetManager {
	return &WingetManager{
		BasePackageManager: BasePackageManager{
			name:      "winget",
			platforms: []string{"windows"},
			adminReq:  false,
		},
	}
}

func (w *WingetManager) IsAvailable() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	return IsCommandAvailable(PACKAGE_MANAGER_WINGET)
}

func (w *WingetManager) Install(component string) error {
	if !w.IsAvailable() {
		return fmt.Errorf("winget is not available")
	}

	packageName := w.getPackageName(component)
	command := fmt.Sprintf("winget install %s --silent --accept-source-agreements --accept-package-agreements", packageName)

	_, err := execCommand(command, 15*time.Minute)
	if err != nil {
		return fmt.Errorf("failed to install %s via winget: %w", component, err)
	}

	return nil
}

func (w *WingetManager) Verify(component string) (*VerificationResult, error) {
	result := &VerificationResult{
		Installed: false,
		Issues:    []string{},
	}

	verifyCmd := w.getVerifyCommand(component)
	if verifyCmd == "" {
		result.Issues = append(result.Issues, "no verification command available")
		return result, nil
	}

	output, err := execCommand(verifyCmd, 30*time.Second)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("verification command failed: %v", err))
		return result, nil
	}

	result.Installed = true
	result.Version = extractVersion(output)

	cmdName := strings.Fields(verifyCmd)[0]
	if IsCommandAvailable(cmdName) {
		if path, err := exec.LookPath(cmdName); err == nil {
			result.Path = path
		}
	}

	return result, nil
}

func (w *WingetManager) getPackageName(component string) string {
	packageMap := map[string]string{
		"go":     "GoLang.Go",
		"python": "Python.Python.3.11",
		"nodejs": "OpenJS.NodeJS",
		"java":   "Eclipse.Temurin.21.JDK",
		"git":    "Git.Git",
	}

	if pkg, exists := packageMap[component]; exists {
		return pkg
	}
	return component
}

func (w *WingetManager) getVerifyCommand(component string) string {
	cmdMap := map[string]string{
		"go":     "go version",
		"python": "python --version",
		"nodejs": "node --version",
		"java":   "java -version",
		"git":    "git --version",
	}

	return cmdMap[component]
}

type ChocolateyManager struct {
	BasePackageManager
}

func NewChocolateyManager() *ChocolateyManager {
	return &ChocolateyManager{
		BasePackageManager: BasePackageManager{
			name:      "chocolatey",
			platforms: []string{"windows"},
			adminReq:  true,
		},
	}
}

func (c *ChocolateyManager) IsAvailable() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	return IsCommandAvailable("choco")
}

func (c *ChocolateyManager) Install(component string) error {
	if !c.IsAvailable() {
		return fmt.Errorf("chocolatey is not available")
	}

	packageName := c.getPackageName(component)
	command := fmt.Sprintf("choco install %s -y", packageName)

	_, err := execCommand(command, 15*time.Minute)
	if err != nil {
		return fmt.Errorf("failed to install %s via chocolatey: %w", component, err)
	}

	return nil
}

func (c *ChocolateyManager) Verify(component string) (*VerificationResult, error) {
	result := &VerificationResult{
		Installed: false,
		Issues:    []string{},
	}

	verifyCmd := c.getVerifyCommand(component)
	if verifyCmd == "" {
		result.Issues = append(result.Issues, "no verification command available")
		return result, nil
	}

	output, err := execCommand(verifyCmd, 30*time.Second)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("verification command failed: %v", err))
		return result, nil
	}

	result.Installed = true
	result.Version = extractVersion(output)

	cmdName := strings.Fields(verifyCmd)[0]
	if IsCommandAvailable(cmdName) {
		if path, err := exec.LookPath(cmdName); err == nil {
			result.Path = path
		}
	}

	return result, nil
}

func (c *ChocolateyManager) getPackageName(component string) string {
	packageMap := map[string]string{
		"go":     "golang",
		"python": "python",
		"nodejs": "nodejs",
		"java":   "openjdk",
		"git":    "git",
	}

	if pkg, exists := packageMap[component]; exists {
		return pkg
	}
	return component
}

func (c *ChocolateyManager) getVerifyCommand(component string) string {
	cmdMap := map[string]string{
		"go":     "go version",
		"python": "python --version",
		"nodejs": "node --version",
		"java":   "java -version",
		"git":    "git --version",
	}

	return cmdMap[component]
}

type YumManager struct {
	BasePackageManager
}

func NewYumManager() *YumManager {
	return &YumManager{
		BasePackageManager: BasePackageManager{
			name:      "yum",
			platforms: []string{"linux"},
			adminReq:  true,
		},
	}
}

func (y *YumManager) IsAvailable() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	return IsCommandAvailable("yum")
}

func (y *YumManager) Install(component string) error {
	if !y.IsAvailable() {
		return fmt.Errorf("yum is not available")
	}

	packageName := y.getPackageName(component)

	updateCmd := "sudo yum makecache"
	_, _ = execCommand(updateCmd, 2*time.Minute) // Update cache, ignore errors

	installCmd := fmt.Sprintf("sudo yum install -y %s", packageName)
	_, err := execCommand(installCmd, 15*time.Minute)
	if err != nil {
		return fmt.Errorf("failed to install %s via yum: %w", component, err)
	}

	return nil
}

func (y *YumManager) Verify(component string) (*VerificationResult, error) {
	result := &VerificationResult{
		Installed: false,
		Issues:    []string{},
	}

	verifyCmd := y.getVerifyCommand(component)
	if verifyCmd == "" {
		result.Issues = append(result.Issues, "no verification command available")
		return result, nil
	}

	output, err := execCommand(verifyCmd, 30*time.Second)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("verification command failed: %v", err))
		return result, nil
	}

	result.Installed = true
	result.Version = extractVersion(output)

	cmdName := strings.Fields(verifyCmd)[0]
	if IsCommandAvailable(cmdName) {
		if path, err := exec.LookPath(cmdName); err == nil {
			result.Path = path
		}
	}

	return result, nil
}

func (y *YumManager) getPackageName(component string) string {
	packageMap := map[string]string{
		"go":     "golang",
		"python": "python3 python3-pip python3-venv",
		"nodejs": "nodejs npm",
		"java":   "java-21-openjdk java-21-openjdk-devel",
		"git":    "git",
	}

	if pkg, exists := packageMap[component]; exists {
		return pkg
	}
	return component
}

func (y *YumManager) getVerifyCommand(component string) string {
	cmdMap := map[string]string{
		"go":     "go version",
		"python": "python3 --version",
		"nodejs": "node --version",
		"java":   "java -version",
		"git":    "git --version",
	}

	return cmdMap[component]
}

type DnfManager struct {
	BasePackageManager
}

func NewDnfManager() *DnfManager {
	return &DnfManager{
		BasePackageManager: BasePackageManager{
			name:      PACKAGE_MANAGER_DNF,
			platforms: []string{"linux"},
			adminReq:  true,
		},
	}
}

func (d *DnfManager) IsAvailable() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	return IsCommandAvailable(PACKAGE_MANAGER_DNF)
}

func (d *DnfManager) Install(component string) error {
	if !d.IsAvailable() {
		return fmt.Errorf("dnf is not available")
	}

	packageName := d.getPackageName(component)

	updateCmd := "sudo dnf makecache"
	_, _ = execCommand(updateCmd, 2*time.Minute) // Update cache, ignore errors

	installCmd := fmt.Sprintf("sudo dnf install -y %s", packageName)
	_, err := execCommand(installCmd, 15*time.Minute)
	if err != nil {
		return fmt.Errorf("failed to install %s via dnf: %w", component, err)
	}

	return nil
}

func (d *DnfManager) Verify(component string) (*VerificationResult, error) {
	result := &VerificationResult{
		Installed: false,
		Issues:    []string{},
	}

	verifyCmd := d.getVerifyCommand(component)
	if verifyCmd == "" {
		result.Issues = append(result.Issues, "no verification command available")
		return result, nil
	}

	output, err := execCommand(verifyCmd, 30*time.Second)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("verification command failed: %v", err))
		return result, nil
	}

	result.Installed = true
	result.Version = extractVersion(output)

	cmdName := strings.Fields(verifyCmd)[0]
	if IsCommandAvailable(cmdName) {
		if path, err := exec.LookPath(cmdName); err == nil {
			result.Path = path
		}
	}

	return result, nil
}

func (d *DnfManager) getPackageName(component string) string {
	packageMap := map[string]string{
		"go":     "golang",
		"python": "python3 python3-pip python3-venv",
		"nodejs": "nodejs npm",
		"java":   "java-21-openjdk java-21-openjdk-devel",
		"git":    "git",
	}

	if pkg, exists := packageMap[component]; exists {
		return pkg
	}
	return component
}

func (d *DnfManager) getVerifyCommand(component string) string {
	cmdMap := map[string]string{
		"go":     "go version",
		"python": "python3 --version",
		"nodejs": "node --version",
		"java":   "java -version",
		"git":    "git --version",
	}

	return cmdMap[component]
}

func GetAvailablePackageManagers() []PackageManager {
	managers := []PackageManager{
		NewHomebrewManager(),
		NewAptManager(),
		NewYumManager(),
		NewDnfManager(),
		NewWingetManager(),
		NewChocolateyManager(),
	}

	var available []PackageManager
	for _, mgr := range managers {
		if mgr.IsAvailable() {
			available = append(available, mgr)
		}
	}

	return available
}

func GetBestPackageManager() PackageManager {
	available := GetAvailablePackageManagers()
	if len(available) == 0 {
		return nil
	}

	switch runtime.GOOS {
	case "darwin":
		return getBestDarwinPackageManager(available)
	case "linux":
		return getBestLinuxPackageManager(available)
	case "windows":
		return getBestWindowsPackageManager(available)
	default:
		return available[0]
	}
}

func getBestDarwinPackageManager(available []PackageManager) PackageManager {
	if mgr := findPackageManagerByName(available, "homebrew"); mgr != nil {
		return mgr
	}
	return available[0]
}

func getBestLinuxPackageManager(available []PackageManager) PackageManager {
	preferredLinuxManagers := []string{"apt", PACKAGE_MANAGER_DNF, "yum"}

	for _, preferred := range preferredLinuxManagers {
		if mgr := findPackageManagerByName(available, preferred); mgr != nil {
			return mgr
		}
	}

	return available[0]
}

func getBestWindowsPackageManager(available []PackageManager) PackageManager {
	preferredWindowsManagers := []string{PACKAGE_MANAGER_WINGET, "chocolatey"}

	for _, preferred := range preferredWindowsManagers {
		if mgr := findPackageManagerByName(available, preferred); mgr != nil {
			return mgr
		}
	}

	return available[0]
}

func findPackageManagerByName(available []PackageManager, name string) PackageManager {
	for _, mgr := range available {
		if mgr.GetName() == name {
			return mgr
		}
	}
	return nil
}

func InstallComponent(component string) error {
	mgr := GetBestPackageManager()
	if mgr == nil {
		return fmt.Errorf("no package managers available on platform %s", runtime.GOOS)
	}

	if mgr.RequiresAdmin() {
		fmt.Fprintf(os.Stderr, "Warning: %s requires administrative privileges\n", mgr.GetName())
	}

	return mgr.Install(component)
}

func VerifyComponent(component string) (*VerificationResult, error) {
	managers := GetAvailablePackageManagers()
	if len(managers) == 0 {
		return nil, fmt.Errorf("no package managers available on platform %s", runtime.GOOS)
	}

	var lastErr error
	for _, mgr := range managers {
		result, err := mgr.Verify(component)
		if err != nil {
			lastErr = err
			continue
		}

		if result.Installed {
			return result, nil
		}
	}

	result := &VerificationResult{
		Installed: false,
		Issues:    []string{"component not found with any available package manager"},
	}

	if lastErr != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("last error: %v", lastErr))
	}

	return result, nil
}
