package setup

import (
	"fmt"
	"lsp-gateway/internal/platform"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type NodeJSDetector struct {
	executor       platform.CommandExecutor
	versionChecker *VersionChecker
	logger         *SetupLogger
}

type NodeJSInfo struct {
	*RuntimeInfo
	NodeVersion      string
	NPMVersion       string
	NPMGlobalPath    string
	NPMRegistry      string
	YarnAvailable    bool
	YarnVersion      string
	PnpmAvailable    bool
	PnpmVersion      string
	GlobalPackages   []string
	PermissionIssues []string
}

type PackageManagerInfo struct {
	Name      string
	Available bool
	Version   string
	Command   string
	Issues    []string
}

func NewNodeJSDetector(logger *SetupLogger) *NodeJSDetector {
	return &NodeJSDetector{
		executor:       platform.NewCommandExecutor(),
		versionChecker: NewVersionChecker(),
		logger:         logger,
	}
}

func (d *NodeJSDetector) DetectNodeJS() (*NodeJSInfo, error) {
	info := &NodeJSInfo{
		RuntimeInfo: &RuntimeInfo{
			Name:       "nodejs",
			Installed:  false,
			Version:    "",
			Compatible: false,
			Path:       "",
			Issues:     []string{},
		},
		GlobalPackages:   []string{},
		PermissionIssues: []string{},
	}

	d.logger.Debug("Starting Node.js runtime detection")

	if err := d.detectNodeVersion(info); err != nil {
		d.logger.WithError(err).Debug("Node.js version detection failed")
		info.Issues = append(info.Issues, fmt.Sprintf("Node.js detection failed: %v", err))
		return info, nil // Return info with issues rather than error
	}

	d.validateNodeVersion(info)

	if err := d.detectNPM(info); err != nil {
		d.logger.WithError(err).Debug("npm detection failed")
		info.Issues = append(info.Issues, fmt.Sprintf("npm detection failed: %v", err))
	}

	if info.NPMVersion != "" {
		d.checkGlobalInstallCapability(info)
	}

	d.detectAlternativePackageManagers(info)

	d.validatePackageResolution(info)

	d.logger.WithFields(map[string]interface{}{
		"node_version": info.NodeVersion,
		"npm_version":  info.NPMVersion,
		"compatible":   info.Compatible,
		"issues_count": len(info.Issues),
	}).Debug("Node.js detection completed")

	return info, nil
}

func (d *NodeJSDetector) detectNodeVersion(info *NodeJSInfo) error {
	if !d.executor.IsCommandAvailable("node") {
		return NewDetectionError("nodejs", "command_availability", "node command not found in PATH", nil)
	}

	result, err := d.executor.Execute("node", []string{"--version"}, 5*time.Second)
	if err != nil {
		return NewDetectionError("nodejs", "version_check",
			fmt.Sprintf("failed to execute 'node --version': %s", result.Stderr), err)
	}

	if result.ExitCode != 0 {
		return NewDetectionError("nodejs", "version_check",
			fmt.Sprintf("node --version exited with code %d: %s", result.ExitCode, result.Stderr), nil)
	}

	versionStr := strings.TrimSpace(result.Stdout)
	if !strings.HasPrefix(versionStr, "v") {
		return NewDetectionError("nodejs", "version_parsing",
			fmt.Sprintf("unexpected version format: %s", versionStr), nil)
	}

	cleanVersion := strings.TrimPrefix(versionStr, "v")

	pathResult, err := d.executor.Execute("which", []string{"node"}, 3*time.Second)
	if err == nil && pathResult.ExitCode == 0 {
		info.Path = strings.TrimSpace(pathResult.Stdout)
	}

	info.Installed = true
	info.Version = cleanVersion
	info.NodeVersion = versionStr

	d.logger.WithFields(map[string]interface{}{
		"version": versionStr,
		"path":    info.Path,
	}).Debug("Node.js installation detected")

	return nil
}

func (d *NodeJSDetector) validateNodeVersion(info *NodeJSInfo) {
	if info.Version == "" {
		info.Compatible = false
		return
	}

	info.Compatible = d.versionChecker.IsCompatible("nodejs", info.Version)

	if !info.Compatible {
		minVersion, _ := d.versionChecker.GetMinVersion("nodejs")
		issue := fmt.Sprintf("Node.js version %s does not meet minimum requirement of %s",
			info.NodeVersion, minVersion)
		info.Issues = append(info.Issues, issue)
		d.logger.Warn(issue)
	} else {
		d.logger.Debug(fmt.Sprintf("Node.js version %s is compatible", info.NodeVersion))
	}
}

func (d *NodeJSDetector) detectNPM(info *NodeJSInfo) error {
	if !d.executor.IsCommandAvailable("npm") {
		return NewDetectionError("npm", "command_availability", "npm command not found in PATH", nil)
	}

	result, err := d.executor.Execute("npm", []string{"--version"}, 10*time.Second)
	if err != nil {
		return NewDetectionError("npm", "version_check",
			fmt.Sprintf("failed to execute 'npm --version': %s", result.Stderr), err)
	}

	if result.ExitCode != 0 {
		return NewDetectionError("npm", "version_check",
			fmt.Sprintf("npm --version exited with code %d: %s", result.ExitCode, result.Stderr), nil)
	}

	info.NPMVersion = strings.TrimSpace(result.Stdout)

	if err := d.testNPMFunctionality(info); err != nil {
		info.Issues = append(info.Issues, fmt.Sprintf("npm functionality test failed: %v", err))
	}

	d.logger.WithField("npm_version", info.NPMVersion).Debug("npm detected and functional")
	return nil
}

func (d *NodeJSDetector) testNPMFunctionality(info *NodeJSInfo) error {
	result, err := d.executor.Execute("npm", []string{"config", "get", "registry"}, 10*time.Second)
	if err != nil {
		return fmt.Errorf("npm config test failed: %w", err)
	}

	if result.ExitCode == 0 {
		info.NPMRegistry = strings.TrimSpace(result.Stdout)
		d.logger.WithField("registry", info.NPMRegistry).Debug("npm registry configured")
	}

	prefixResult, err := d.executor.Execute("npm", []string{"config", "get", "prefix"}, 10*time.Second)
	if err == nil && prefixResult.ExitCode == 0 {
		info.NPMGlobalPath = strings.TrimSpace(prefixResult.Stdout)
		d.logger.WithField("global_path", info.NPMGlobalPath).Debug("npm global path detected")
	}

	return nil
}

func (d *NodeJSDetector) checkGlobalInstallCapability(info *NodeJSInfo) {
	if info.NPMGlobalPath == "" {
		info.Issues = append(info.Issues, "npm global path not detected")
		return
	}

	globalNodeModules := filepath.Join(info.NPMGlobalPath, "lib", "node_modules")
	if runtime.GOOS == "windows" {
		globalNodeModules = filepath.Join(info.NPMGlobalPath, "node_modules")
	}

	if _, err := os.Stat(globalNodeModules); os.IsNotExist(err) {
		if err := os.MkdirAll(globalNodeModules, 0755); err != nil {
			info.PermissionIssues = append(info.PermissionIssues,
				fmt.Sprintf("Cannot create global node_modules directory: %v", err))
		}
	}

	if err := d.testWritePermissions(globalNodeModules); err != nil {
		info.PermissionIssues = append(info.PermissionIssues,
			fmt.Sprintf("No write permission to global node_modules: %v", err))
	}

	d.listGlobalPackages(info)
}

func (d *NodeJSDetector) testWritePermissions(dir string) error {
	testFile := filepath.Join(dir, ".lsp-gateway-write-test")

	file, err := os.Create(testFile)
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		if removeErr := os.Remove(testFile); removeErr != nil {
		}
		return err
	}

	if err := os.Remove(testFile); err != nil {
	}
	return nil
}

func (d *NodeJSDetector) listGlobalPackages(info *NodeJSInfo) {
	result, err := d.executor.Execute("npm", []string{"list", "-g", "--depth=0", "--json"}, 15*time.Second)
	if err != nil || result.ExitCode != 0 {
		result, err = d.executor.Execute("npm", []string{"list", "-g", "--depth=0"}, 15*time.Second)
		if err != nil || result.ExitCode != 0 {
			info.Issues = append(info.Issues, "Could not list global packages")
			return
		}
	}

	lines := strings.Split(result.Stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "@") && !strings.HasPrefix(line, "npm") {
			parts := strings.Fields(line)
			if len(parts) > 1 {
				packageInfo := parts[1]
				if strings.Contains(packageInfo, "@") {
					packageName := strings.Split(packageInfo, "@")[0]
					if packageName != "" && packageName != "npm" {
						info.GlobalPackages = append(info.GlobalPackages, packageName)
					}
				}
			}
		}
	}

	d.logger.WithField("global_packages", info.GlobalPackages).Debug("Global packages detected")
}

func (d *NodeJSDetector) detectAlternativePackageManagers(info *NodeJSInfo) {
	if d.executor.IsCommandAvailable(YARN_COMMAND) {
		result, err := d.executor.Execute(YARN_COMMAND, []string{"--version"}, 5*time.Second)
		if err == nil && result.ExitCode == 0 {
			info.YarnAvailable = true
			info.YarnVersion = strings.TrimSpace(result.Stdout)
			d.logger.WithField("yarn_version", info.YarnVersion).Debug("Yarn detected")
		}
	}

	if d.executor.IsCommandAvailable("pnpm") {
		result, err := d.executor.Execute("pnpm", []string{"--version"}, 5*time.Second)
		if err == nil && result.ExitCode == 0 {
			info.PnpmAvailable = true
			info.PnpmVersion = strings.TrimSpace(result.Stdout)
			d.logger.WithField("pnpm_version", info.PnpmVersion).Debug("pnpm detected")
		}
	}
}

func (d *NodeJSDetector) validatePackageResolution(info *NodeJSInfo) {
	result, err := d.executor.Execute("node", []string{"-e", "console.log(require.resolve('fs'))"}, 5*time.Second)
	if err != nil || result.ExitCode != 0 {
		info.Issues = append(info.Issues, "Node.js module resolution test failed")
		return
	}

	builtinTest := `
try {
  require('fs');
  require('path');
  require('os');
  console.log('OK');
} catch (e) {
  console.log('ERROR: ' + e.message);
}
`
	result, err = d.executor.Execute("node", []string{"-e", builtinTest}, 5*time.Second)
	if err != nil || result.ExitCode != 0 || !strings.Contains(result.Stdout, "OK") {
		info.Issues = append(info.Issues, "Built-in module resolution failed")
	}
}

func (d *NodeJSDetector) GetPackageManagerRecommendation(info *NodeJSInfo) *PackageManagerInfo {
	if info.NPMVersion != "" && len(info.PermissionIssues) == 0 {
		return &PackageManagerInfo{
			Name:      "npm",
			Available: true,
			Version:   info.NPMVersion,
			Command:   "npm",
			Issues:    info.PermissionIssues,
		}
	}

	if info.YarnAvailable {
		return &PackageManagerInfo{
			Name:      YARN_COMMAND,
			Available: true,
			Version:   info.YarnVersion,
			Command:   YARN_COMMAND,
			Issues:    []string{},
		}
	}

	if info.PnpmAvailable {
		return &PackageManagerInfo{
			Name:      "pnpm",
			Available: true,
			Version:   info.PnpmVersion,
			Command:   "pnpm",
			Issues:    []string{},
		}
	}

	return &PackageManagerInfo{
		Name:      "none",
		Available: false,
		Version:   "",
		Command:   "",
		Issues:    []string{"No functional package manager detected"},
	}
}

func (d *NodeJSDetector) ValidateForTypeScriptLS(info *NodeJSInfo) []string {
	var issues []string

	if !info.Installed {
		issues = append(issues, "Node.js is not installed")
		return issues
	}

	version, err := ParseVersion(info.Version)
	if err != nil {
		issues = append(issues, fmt.Sprintf("Cannot parse Node.js version: %v", err))
		return issues
	}

	if version.Major < 14 {
		issues = append(issues, fmt.Sprintf("TypeScript Language Server requires Node.js 14+, found %s", info.NodeVersion))
	}

	recommendation := d.GetPackageManagerRecommendation(info)
	if !recommendation.Available {
		issues = append(issues, "No functional package manager available for installing TypeScript Language Server")
	}

	if len(info.PermissionIssues) > 0 {
		issues = append(issues, "Global package installation may fail due to permission issues")
		for _, permIssue := range info.PermissionIssues {
			issues = append(issues, fmt.Sprintf("  - %s", permIssue))
		}
	}

	for _, pkg := range info.GlobalPackages {
		if pkg == "typescript-language-server" {
			d.logger.Debug("typescript-language-server already installed globally")
			break
		}
	}

	return issues
}

func (d *NodeJSDetector) InstallTypeScriptLS(info *NodeJSInfo) error {
	recommendation := d.GetPackageManagerRecommendation(info)
	if !recommendation.Available {
		return fmt.Errorf("no functional package manager available")
	}

	d.logger.WithField("package_manager", recommendation.Name).Info("Installing TypeScript Language Server")

	var args []string
	switch recommendation.Name {
	case "npm":
		args = []string{"install", "-g", "typescript-language-server", "typescript"}
	case YARN_COMMAND:
		args = []string{"global", "add", "typescript-language-server", "typescript"}
	case "pnpm":
		args = []string{"add", "-g", "typescript-language-server", "typescript"}
	default:
		return fmt.Errorf("unsupported package manager: %s", recommendation.Name)
	}

	result, err := d.executor.Execute(recommendation.Command, args, 60*time.Second)
	if err != nil {
		return fmt.Errorf("failed to install TypeScript Language Server: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("TypeScript Language Server installation failed with exit code %d: %s",
			result.ExitCode, result.Stderr)
	}

	verifyResult, err := d.executor.Execute("typescript-language-server", []string{"--version"}, 10*time.Second)
	if err != nil || verifyResult.ExitCode != 0 {
		return fmt.Errorf("TypeScript Language Server installation verification failed")
	}

	d.logger.UserSuccess("TypeScript Language Server installed successfully")
	return nil
}

func (d *NodeJSDetector) GetInstallationGuidance(info *NodeJSInfo) []string {
	var guidance []string

	if !info.Installed {
		guidance = append(guidance, "Node.js is not installed. Install Node.js 18.0.0 or later:")

		switch runtime.GOOS {
		case "windows":
			guidance = append(guidance, "  1. Download from https://nodejs.org")
			guidance = append(guidance, "  2. Or use winget: winget install OpenJS.NodeJS")
			guidance = append(guidance, "  3. Or use chocolatey: choco install nodejs")
		case "darwin":
			guidance = append(guidance, "  1. Download from https://nodejs.org")
			guidance = append(guidance, "  2. Or use homebrew: brew install node")
			guidance = append(guidance, "  3. Or use macports: sudo port install nodejs18")
		case "linux":
			guidance = append(guidance, "  1. Use package manager (Ubuntu/Debian): sudo apt install nodejs npm")
			guidance = append(guidance, "  2. Use package manager (RHEL/CentOS): sudo yum install nodejs npm")
			guidance = append(guidance, "  3. Use NodeSource repository for latest version")
			guidance = append(guidance, "  4. Download from https://nodejs.org")
		}

		return guidance
	}

	if !info.Compatible {
		minVersion, _ := d.versionChecker.GetMinVersion("nodejs")
		guidance = append(guidance, fmt.Sprintf("Node.js version %s is too old. Update to %s or later.",
			info.NodeVersion, minVersion))
	}

	if info.NPMVersion == "" {
		guidance = append(guidance, "npm is not available. Reinstall Node.js or install npm separately:")
		guidance = append(guidance, "  - npm is typically bundled with Node.js")
		guidance = append(guidance, "  - Try reinstalling Node.js from https://nodejs.org")
	}

	if len(info.PermissionIssues) > 0 {
		guidance = append(guidance, "Global package installation has permission issues:")
		for _, issue := range info.PermissionIssues {
			guidance = append(guidance, fmt.Sprintf("  - %s", issue))
		}
		guidance = append(guidance, "Solutions:")
		guidance = append(guidance, "  1. Use npm config to change global directory: npm config set prefix ~/.npm-global")
		guidance = append(guidance, "  2. Use a Node.js version manager (nvm, fnm)")
		guidance = append(guidance, "  3. Fix permissions: https://docs.npmjs.com/resolving-eacces-permissions-errors")
	}

	return guidance
}
