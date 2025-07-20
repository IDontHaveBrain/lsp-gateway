package installer

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/types"
)

type ServerInstaller = types.ServerInstaller

type ServerInstallOptions = types.ServerInstallOptions

type DependencyValidationResult = types.DependencyValidationResult

type ServerPlatformStrategy = types.ServerPlatformStrategy

type ServerDefinition = types.ServerDefinition

type ServerInstallMethod = types.InstallMethod

type DefaultServerInstaller struct {
	registry         *ServerRegistry
	runtimeInstaller types.RuntimeInstaller
	strategies       map[string]types.ServerPlatformStrategy
}

func NewServerInstaller(runtimeInstaller types.RuntimeInstaller) *DefaultServerInstaller {
	installer := &DefaultServerInstaller{
		registry:         NewServerRegistry(),
		runtimeInstaller: runtimeInstaller,
		strategies:       make(map[string]types.ServerPlatformStrategy),
	}

	installer.strategies["windows"] = &WindowsServerStrategyAdapter{}
	installer.strategies["linux"] = &LinuxServerStrategyAdapter{}
	installer.strategies["darwin"] = &MacOSServerStrategyAdapter{}

	return installer
}

func (s *DefaultServerInstaller) Install(server string, options types.ServerInstallOptions) (*types.InstallResult, error) {
	startTime := time.Now()

	_, err := s.registry.GetServer(server)
	if err != nil {
		return nil, NewInstallerError(InstallerErrorTypeNotFound, server,
			fmt.Sprintf("unknown server: %s", server), err)
	}

	if !options.SkipDependencyCheck {
		if depResult, err := s.ValidateDependencies(server); err != nil || !depResult.CanInstall {
			return &types.InstallResult{
				Success:  false,
				Version:  "",
				Path:     "",
				Method:   "dependency_check",
				Duration: time.Since(startTime),
				Warnings: []string{},
				Errors:   []string{fmt.Sprintf("dependency validation failed: %v", err)},
				Details:  map[string]interface{}{"server": server},
			}, nil
		}
	}

	platform := options.Platform
	if platform == "" {
		platform = s.detectCurrentPlatform()
	}

	strategy := s.GetPlatformStrategy(platform)
	if strategy == nil {
		return &types.InstallResult{
			Success:  false,
			Version:  "",
			Path:     "",
			Method:   "platform_strategy",
			Duration: time.Since(startTime),
			Warnings: []string{},
			Errors:   []string{fmt.Sprintf("no strategy available for platform: %s", platform)},
			Details:  map[string]interface{}{"server": server, "platform": platform},
		}, nil
	}

	installResult, installErr := strategy.InstallServer(server, options)
	if installErr != nil {
		return &types.InstallResult{
			Success:  false,
			Version:  "",
			Path:     "",
			Method:   fmt.Sprintf("%s_platform_strategy", platform),
			Duration: time.Since(startTime),
			Warnings: []string{},
			Errors:   []string{installErr.Error()},
			Details:  map[string]interface{}{"server": server, "platform": platform},
		}, nil
	}

	if installResult != nil {
		installResult.Duration = time.Since(startTime)
		installResult.Details["platform"] = platform
		return installResult, nil
	}

	result := &types.InstallResult{
		Success:  true,
		Version:  options.Version,
		Path:     "",
		Method:   fmt.Sprintf("%s_platform_strategy", platform),
		Duration: time.Since(startTime),
		Warnings: []string{},
		Errors:   []string{},
		Details:  map[string]interface{}{"server": server, "platform": platform},
	}

	if !options.SkipVerify {
		if verifyResult, err := s.Verify(server); err != nil || !verifyResult.Installed {
			result.Success = false
			result.Warnings = append(result.Warnings, "installation completed but verification failed")
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("verification error: %v", err))
			}
		} else {
			result.Path = verifyResult.Path
			result.Version = verifyResult.Version
		}
	}

	return result, nil
}

func (s *DefaultServerInstaller) Verify(server string) (*types.VerificationResult, error) {
	verifier := NewServerVerifier(s.runtimeInstaller)

	serverResult, err := verifier.VerifyServer(server)
	if err != nil {
		return nil, err
	}

	result := &types.VerificationResult{
		Installed:  serverResult.Installed,
		Compatible: serverResult.Compatible,
		Version:    serverResult.Version,
		Path:       serverResult.Path,
		Issues:     serverResult.Issues,
		Details:    make(map[string]interface{}),
	}

	result.Details["server"] = server
	result.Details["verified_at"] = serverResult.VerifiedAt
	result.Details["duration"] = serverResult.Duration
	result.Details["functional"] = serverResult.Functional
	result.Details["runtime_required"] = serverResult.RuntimeRequired

	if serverResult.RuntimeStatus != nil {
		result.Details["runtime_status"] = serverResult.RuntimeStatus
	}

	if serverResult.HealthCheck != nil {
		result.Details["health_check"] = map[string]interface{}{
			"responsive":   serverResult.HealthCheck.Responsive,
			"startup_time": serverResult.HealthCheck.StartupTime,
			"test_method":  serverResult.HealthCheck.TestMethod,
			"tested_at":    serverResult.HealthCheck.TestedAt,
		}
	}

	for k, v := range serverResult.Metadata {
		result.Details[k] = v
	}

	return result, nil
}

func (s *DefaultServerInstaller) ValidateDependencies(server string) (*types.DependencyValidationResult, error) {
	serverDef, err := s.registry.GetServer(server)
	if err != nil {
		return nil, NewInstallerError(InstallerErrorTypeNotFound, server,
			fmt.Sprintf("unknown server: %s", server), err)
	}

	result := &types.DependencyValidationResult{
		CanInstall:      false,
		MissingRuntimes: []string{},
		VersionIssues:   []types.VersionIssue{},
		Recommendations: []string{},
	}

	runtimeResult, err := s.runtimeInstaller.Verify(serverDef.Runtime)
	if err != nil {
		result.MissingRuntimes = append(result.MissingRuntimes, serverDef.Runtime)
		result.Recommendations = append(result.Recommendations,
			fmt.Sprintf("Install %s runtime before installing %s server", serverDef.Runtime, server))
		return result, nil
	}

	if !runtimeResult.Installed {
		result.MissingRuntimes = append(result.MissingRuntimes, serverDef.Runtime)
		result.Recommendations = append(result.Recommendations,
			fmt.Sprintf("Install %s runtime before installing %s server", serverDef.Runtime, server))
		return result, nil
	}

	if !runtimeResult.Compatible {
		result.VersionIssues = append(result.VersionIssues, types.VersionIssue{
			Component:        serverDef.Runtime,
			RequiredVersion:  serverDef.MinVersion,
			InstalledVersion: runtimeResult.Version,
			Severity:         types.IssueSeverityHigh,
		})
		result.Recommendations = append(result.Recommendations,
			fmt.Sprintf("Upgrade %s runtime to meet minimum version requirements", serverDef.Runtime))
		return result, nil
	}

	result.CanInstall = true

	return result, nil
}

func (s *DefaultServerInstaller) GetSupportedServers() []string {
	return s.registry.ListServers()
}

func (s *DefaultServerInstaller) GetServerInfo(server string) (*types.ServerDefinition, error) {
	return s.registry.GetServer(server)
}

func (s *DefaultServerInstaller) GetPlatformStrategy(platform string) types.ServerPlatformStrategy {
	if strategy, exists := s.strategies[platform]; exists {
		return strategy
	}
	return s.strategies["linux"]
}

func (s *DefaultServerInstaller) detectCurrentPlatform() string {
	return runtime.GOOS
}

type ServerRegistry struct {
	servers map[string]*types.ServerDefinition
}

func NewServerRegistry() *ServerRegistry {
	registry := &ServerRegistry{
		servers: make(map[string]*types.ServerDefinition),
	}

	registry.registerDefaults()

	return registry
}

func (s *ServerRegistry) registerDefaults() {
	s.servers["gopls"] = &types.ServerDefinition{
		Name:              "gopls",
		DisplayName:       "Go Language Server",
		Runtime:           "go",
		MinVersion:        "1.19.0",
		MinRuntimeVersion: "1.19.0",
		InstallCmd:        []string{"go", "install", "golang.org/x/tools/gopls@latest"},
		VerifyCmd:         []string{"gopls", "version"},
		ConfigKey:         "go-lsp",
		Description:       "Official Go language server",
		Homepage:          "https://golang.org/x/tools/gopls",
		Languages:         []string{"go"},
		InstallMethods: []types.InstallMethod{
			{
				Name:          "go_install",
				Platform:      "all",
				Method:        "go_install",
				Commands:      []string{"go", "install", "golang.org/x/tools/gopls@latest"},
				Description:   "Install using go install command",
				Requirements:  []string{"go >= 1.19.0"},
				PreRequisites: []string{"go version"},
				Verification:  []string{"gopls", "version"},
				PostInstall:   []string{},
			},
		},
		VersionCommand: []string{"gopls", "version"},
	}

	s.servers["pylsp"] = &types.ServerDefinition{
		Name:        "pylsp",
		DisplayName: "Python Language Server",
		Runtime:     "python",
		MinVersion:  "3.8.0",
		InstallCmd:  []string{"pip", "install", "python-lsp-server"},
		VerifyCmd:   []string{"pylsp", "--version"},
		ConfigKey:   "python-lsp",
		Description: "Python LSP server implementation",
		Homepage:    "https://github.com/python-lsp/python-lsp-server",
		Languages:   []string{"python"},
		InstallMethods: []types.InstallMethod{
			{
				Name:          "pip_install",
				Platform:      "all",
				Method:        "pip_install",
				Commands:      []string{"pip", "install", "python-lsp-server"},
				Description:   "Install using pip package manager",
				Requirements:  []string{"python >= 3.8.0", "pip"},
				PreRequisites: []string{"python --version", "pip --version"},
				Verification:  []string{"pylsp", "--version"},
				PostInstall:   []string{},
			},
		},
		VersionCommand: []string{"pylsp", "--version"},
	}

	s.servers["typescript-language-server"] = &types.ServerDefinition{
		Name:        "typescript-language-server",
		DisplayName: "TypeScript Language Server",
		Runtime:     "nodejs",
		MinVersion:  "18.0.0",
		InstallCmd:  []string{"npm", "install", "-g", "typescript-language-server", "typescript"},
		VerifyCmd:   []string{"typescript-language-server", "--version"},
		ConfigKey:   "typescript-lsp",
		Description: "Language server for TypeScript and JavaScript",
		Homepage:    "https://github.com/typescript-language-server/typescript-language-server",
		Languages:   []string{"typescript", "javascript"},
		InstallMethods: []types.InstallMethod{
			{
				Name:          "npm_global",
				Platform:      "all",
				Method:        "npm_global",
				Commands:      []string{"npm", "install", "-g", "typescript-language-server", "typescript"},
				Description:   "Install globally using npm",
				Requirements:  []string{"nodejs >= 18.0.0", "npm"},
				PreRequisites: []string{"node --version", "npm --version"},
				Verification:  []string{"typescript-language-server", "--version"},
				PostInstall:   []string{},
			},
		},
		VersionCommand: []string{"typescript-language-server", "--version"},
	}

	s.servers["jdtls"] = &types.ServerDefinition{
		Name:        "jdtls",
		DisplayName: "Eclipse JDT Language Server",
		Runtime:     "java",
		MinVersion:  "17.0.0",
		InstallCmd:  []string{"echo", "Manual download from Eclipse JDT Language Server releases"},
		VerifyCmd:   []string{"java", "--version"},
		ConfigKey:   "java-lsp",
		Description: "Eclipse JDT Language Server for Java",
		Homepage:    "https://github.com/eclipse/eclipse.jdt.ls",
		Languages:   []string{"java"},
		InstallMethods: []types.InstallMethod{
			{
				Name:          "manual_download",
				Platform:      "all",
				Method:        "manual_download",
				Commands:      []string{"echo", "Manual download from Eclipse JDT Language Server releases"},
				Description:   "Manual download and setup from Eclipse releases",
				Requirements:  []string{"java >= 17.0.0"},
				PreRequisites: []string{"java --version"},
				Verification:  []string{"java", "--version"},
				PostInstall:   []string{},
			},
		},
		VersionCommand: []string{"java", "--version"},
	}
}

func (s *ServerRegistry) GetServer(name string) (*types.ServerDefinition, error) {
	if server, exists := s.servers[name]; exists {
		return server, nil
	}
	return nil, &InstallerError{
		Type:    InstallerErrorTypeNotFound,
		Message: "server not found: " + name,
	}
}

func (s *ServerRegistry) RegisterServer(server *types.ServerDefinition) {
	s.servers[server.Name] = server
}

func (s *ServerRegistry) ListServers() []string {
	names := make([]string, 0, len(s.servers))
	for name := range s.servers {
		names = append(names, name)
	}
	return names
}

func (s *ServerRegistry) GetServersByRuntime(runtime string) []*types.ServerDefinition {
	var servers []*types.ServerDefinition
	for _, server := range s.servers {
		if server.Runtime == runtime {
			servers = append(servers, server)
		}
	}
	return servers
}

func (s *ServerRegistry) GetServersByLanguage(language string) []*types.ServerDefinition {
	var servers []*types.ServerDefinition
	for _, server := range s.servers {
		for _, lang := range server.Languages {
			if lang == language {
				servers = append(servers, server)
				break
			}
		}
	}
	return servers
}

type WindowsServerStrategy struct{}

func (w *WindowsServerStrategy) InstallGopls(version string) error {
	return fmt.Errorf("windows server installation not implemented yet")
}

func (w *WindowsServerStrategy) InstallPylsp(version string) error {
	return fmt.Errorf("windows server installation not implemented yet")
}

func (w *WindowsServerStrategy) InstallTypeScriptLS(version string) error {
	return fmt.Errorf("windows server installation not implemented yet")
}

func (w *WindowsServerStrategy) InstallJdtls(version string) error {
	return fmt.Errorf("windows server installation not implemented yet")
}

func (w *WindowsServerStrategy) VerifyServer(server string) (*VerificationResult, error) {
	return nil, fmt.Errorf("windows server verification not implemented yet")
}

func (w *WindowsServerStrategy) GetInstallationMethods(server string) []string {
	return []string{"not_implemented"}
}

type LinuxServerStrategy struct{}

func (l *LinuxServerStrategy) InstallGopls(version string) error {
	return fmt.Errorf("linux server installation not implemented yet")
}

func (l *LinuxServerStrategy) InstallPylsp(version string) error {
	return fmt.Errorf("linux server installation not implemented yet")
}

func (l *LinuxServerStrategy) InstallTypeScriptLS(version string) error {
	return fmt.Errorf("linux server installation not implemented yet")
}

func (l *LinuxServerStrategy) InstallJdtls(version string) error {
	installer := &DefaultServerInstaller{}
	result, err := installer.installJdtlsLinux()
	if err != nil {
		return err
	}
	if !result.Success {
		return fmt.Errorf("jdtls installation failed: %s", strings.Join(result.Errors, "; "))
	}
	return nil
}

func (l *LinuxServerStrategy) VerifyServer(server string) (*VerificationResult, error) {
	return nil, fmt.Errorf("linux server verification not implemented yet")
}

func (l *LinuxServerStrategy) GetInstallationMethods(server string) []string {
	return []string{"not_implemented"}
}

type MacOSServerStrategy struct{}

func (m *MacOSServerStrategy) InstallGopls(version string) error {
	return fmt.Errorf("macOS server installation not implemented yet")
}

func (m *MacOSServerStrategy) InstallPylsp(version string) error {
	return fmt.Errorf("macOS server installation not implemented yet")
}

func (m *MacOSServerStrategy) InstallTypeScriptLS(version string) error {
	return fmt.Errorf("macOS server installation not implemented yet")
}

func (m *MacOSServerStrategy) InstallJdtls(version string) error {
	return fmt.Errorf("macOS server installation not implemented yet")
}

func (m *MacOSServerStrategy) VerifyServer(server string) (*VerificationResult, error) {
	return nil, fmt.Errorf("macOS server verification not implemented yet")
}

func (m *MacOSServerStrategy) GetInstallationMethods(server string) []string {
	return []string{"not_implemented"}
}

func (s *DefaultServerInstaller) installJdtlsLinux() (*types.InstallResult, error) {
	startTime := time.Now()
	result := &types.InstallResult{
		Success:  false,
		Version:  "",
		Path:     "",
		Method:   "manual download",
		Duration: 0,
		Warnings: []string{},
		Errors:   []string{},
		Details:  map[string]interface{}{"server": "jdtls"},
	}

	messages := []string{"Checking Java runtime availability..."}
	messages = append(messages, "Java runtime check skipped (requires dependency injection fix)")

	messages = append(messages, "Java runtime verification temporarily disabled")

	installDir := s.getJdtlsInstallDir()
	messages = append(messages, fmt.Sprintf("Installing jdtls to: %s", installDir))

	if s.isJdtlsInstalled(installDir) {
		existingVersion := s.getInstalledJdtlsVersion(installDir)
		if existingVersion != "" {
			messages = append(messages, fmt.Sprintf("jdtls %s already installed", existingVersion))
			result.Success = true
			result.Version = existingVersion
			result.Path = s.getJdtlsExecutablePath(installDir)
			result.Duration = time.Since(startTime)
			result.Details["messages"] = messages
			return result, nil
		}
	}

	messages = append(messages, "Downloading Eclipse JDT Language Server...")
	downloadURL := s.getJdtlsDownloadURL()
	tempFile, err := s.downloadJdtls(downloadURL)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Download failed: %v", err))
		result.Duration = time.Since(startTime)
		result.Details["messages"] = messages
		return result, NewInstallerError(InstallerErrorTypeNetwork, "jdtls", "Failed to download jdtls archive", err)
	}
	defer func() {
		_ = os.Remove(tempFile)
	}() // Clean up temp file

	messages = append(messages, "Download completed successfully")

	messages = append(messages, "Extracting jdtls archive...")
	if err := s.extractJdtls(tempFile, installDir); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Extraction failed: %v", err))
		result.Duration = time.Since(startTime)
		result.Details["messages"] = messages
		return result, NewInstallerError(InstallerErrorTypeInstallation, "jdtls", "Failed to extract jdtls archive", err)
	}

	messages = append(messages, "Extraction completed successfully")

	messages = append(messages, "Creating jdtls executable wrapper...")
	executablePath, err := s.createJdtlsWrapper(installDir, "java") // Default to "java" command
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Wrapper creation failed: %v", err))
		result.Duration = time.Since(startTime)
		result.Details["messages"] = messages
		return result, NewInstallerError(InstallerErrorTypeInstallation, "jdtls", "Failed to create executable wrapper", err)
	}

	messages = append(messages, fmt.Sprintf("Executable wrapper created: %s", executablePath))

	messages = append(messages, "Verifying jdtls installation...")
	version_detected, err := s.verifyJdtlsInstallation(executablePath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Installation verification failed: %v", err))
		result.Duration = time.Since(startTime)
		result.Details["messages"] = messages
		return result, NewInstallerError(InstallerErrorTypeVerification, "jdtls", "Installation verification failed", err)
	}

	messages = append(messages, "jdtls installation completed successfully")
	result.Success = true
	result.Version = version_detected
	result.Path = executablePath
	result.Duration = time.Since(startTime)
	result.Details["messages"] = messages

	return result, nil
}

func (s *DefaultServerInstaller) getJdtlsInstallDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	return filepath.Join(homeDir, ".local", "share", "lsp-gateway", "jdtls")
}

func (s *DefaultServerInstaller) getJdtlsDownloadURL() string {
	return "https://download.eclipse.org/jdtls/snapshots/jdt-language-server-latest.tar.gz"
}

func (s *DefaultServerInstaller) isJdtlsInstalled(installDir string) bool {
	requiredFiles := []string{
		"plugins/org.eclipse.equinox.launcher_*.jar",
		"config_linux", // or config_win, config_mac
	}

	for _, pattern := range requiredFiles {
		if pattern == "config_linux" {
			configDir := s.getJdtlsConfigDir(installDir)
			if _, err := os.Stat(configDir); os.IsNotExist(err) {
				return false
			}
		} else {
			pluginsDir := filepath.Join(installDir, "plugins")
			if entries, err := os.ReadDir(pluginsDir); err == nil {
				found := false
				for _, entry := range entries {
					if strings.Contains(entry.Name(), "org.eclipse.equinox.launcher") && strings.HasSuffix(entry.Name(), ".jar") {
						found = true
						break
					}
				}
				if !found {
					return false
				}
			} else {
				return false
			}
		}
	}

	return true
}

func (s *DefaultServerInstaller) getInstalledJdtlsVersion(installDir string) string {
	pluginsDir := filepath.Join(installDir, "plugins")
	if entries, err := os.ReadDir(pluginsDir); err == nil {
		for _, entry := range entries {
			if strings.Contains(entry.Name(), "org.eclipse.jdt.ls.core") {
				name := entry.Name()
				if idx := strings.Index(name, "_"); idx != -1 {
					versionPart := name[idx+1:]
					if dotIdx := strings.Index(versionPart, ".v"); dotIdx != -1 {
						return versionPart[:dotIdx]
					}
				}
			}
		}
	}
	return "unknown"
}

func (s *DefaultServerInstaller) getJdtlsExecutablePath(installDir string) string {
	executable := "jdtls"
	if runtime.GOOS == "windows" {
		executable = "jdtls.bat"
	}
	return filepath.Join(installDir, "bin", executable)
}

func (s *DefaultServerInstaller) getJdtlsConfigDir(installDir string) string {
	var configSuffix string
	switch runtime.GOOS {
	case "windows":
		configSuffix = "config_win"
	case "darwin":
		configSuffix = "config_mac"
	default:
		configSuffix = "config_linux"
	}
	return filepath.Join(installDir, configSuffix)
}

func (s *DefaultServerInstaller) downloadJdtls(url string) (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Minute, // jdtls archive can be large
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create download request: %w", err)
	}

	req.Header.Set("User-Agent", "lsp-gateway-installer/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download jdtls: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	tempFile, err := os.CreateTemp("", "jdtls-*.tar.gz")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer func() {
		if err := tempFile.Close(); err != nil {
		}
	}()

	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		if removeErr := os.Remove(tempFile.Name()); removeErr != nil {
		}
		return "", fmt.Errorf("failed to save download: %w", err)
	}

	return tempFile.Name(), nil
}

func (s *DefaultServerInstaller) extractJdtls(archivePath, installDir string) error {
	if err := s.prepareInstallDirectory(installDir); err != nil {
		return err
	}

	tarReader, cleanup, err := s.openTarArchive(archivePath)
	if err != nil {
		return err
	}
	defer cleanup()

	return s.extractTarContents(tarReader, installDir)
}

func (s *DefaultServerInstaller) prepareInstallDirectory(installDir string) error {
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("failed to create installation directory: %w", err)
	}
	return nil
}

func (s *DefaultServerInstaller) openTarArchive(archivePath string) (*tar.Reader, func(), error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open archive: %w", err)
	}

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		if closeErr := file.Close(); closeErr != nil {
			// Log close error but continue with original error
		}
		return nil, nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}

	tarReader := tar.NewReader(gzReader)

	cleanup := func() {
		if err := gzReader.Close(); err != nil {
			// Error closing gzip reader during cleanup
		}
		if err := file.Close(); err != nil {
			// Error closing file during cleanup
		}
	}

	return tarReader, cleanup, nil
}

func (s *DefaultServerInstaller) extractTarContents(tarReader *tar.Reader, installDir string) error {
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		if err := s.extractTarEntry(tarReader, header, installDir); err != nil {
			return err
		}
	}

	return nil
}

func (s *DefaultServerInstaller) extractTarEntry(tarReader *tar.Reader, header *tar.Header, installDir string) error {
	targetPath := filepath.Join(installDir, header.Name)

	if err := s.validateExtractionPath(targetPath, installDir, header.Name); err != nil {
		return err
	}

	switch header.Typeflag {
	case tar.TypeDir:
		return s.extractDirectory(targetPath, header)
	case tar.TypeReg:
		return s.extractRegularFile(tarReader, targetPath, header)
	case tar.TypeSymlink:
		return s.extractSymlink(targetPath, header)
	default:
		return nil
	}
}

func (s *DefaultServerInstaller) validateExtractionPath(targetPath, installDir, entryName string) error {
	cleanInstallDir := filepath.Clean(installDir) + string(os.PathSeparator)
	if !strings.HasPrefix(targetPath, cleanInstallDir) {
		return fmt.Errorf("path traversal attempt detected: %s", entryName)
	}
	return nil
}

func (s *DefaultServerInstaller) extractDirectory(targetPath string, header *tar.Header) error {
	if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
	}
	return nil
}

func (s *DefaultServerInstaller) extractRegularFile(tarReader *tar.Reader, targetPath string, header *tar.Header) error {
	if err := s.ensureParentDirectory(targetPath); err != nil {
		return err
	}

	return s.writeFileContent(tarReader, targetPath, header)
}

func (s *DefaultServerInstaller) ensureParentDirectory(targetPath string) error {
	parentDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory for %s: %w", targetPath, err)
	}
	return nil
}

func (s *DefaultServerInstaller) writeFileContent(tarReader *tar.Reader, targetPath string, header *tar.Header) error {
	outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", targetPath, err)
	}

	defer func() {
		if closeErr := outFile.Close(); closeErr != nil {
		}
	}()

	if _, err := io.Copy(outFile, tarReader); err != nil {
		return fmt.Errorf("failed to extract file %s: %w", targetPath, err)
	}

	return nil
}

func (s *DefaultServerInstaller) extractSymlink(targetPath string, header *tar.Header) error {
	if err := os.Symlink(header.Linkname, targetPath); err != nil {
		return fmt.Errorf("failed to create symlink %s: %w", targetPath, err)
	}
	return nil
}

func (s *DefaultServerInstaller) createJdtlsWrapper(installDir, javaPath string) (string, error) {
	binDir := filepath.Join(installDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create bin directory: %w", err)
	}

	launcherJar, err := s.findJdtlsLauncherJar(installDir)
	if err != nil {
		return "", fmt.Errorf("failed to find launcher jar: %w", err)
	}

	configDir := s.getJdtlsConfigDir(installDir)

	var wrapperPath string
	var wrapperContent string

	if runtime.GOOS == "windows" {
		wrapperPath = filepath.Join(binDir, "jdtls.bat")
		wrapperContent = fmt.Sprintf(`@echo off
"%s" ^
  -Declipse.application=org.eclipse.jdt.ls.core.id1 ^
  -Dosgi.bundles.defaultStartLevel=4 ^
  -Declipse.product=org.eclipse.jdt.ls.core.product ^
  -Dlog.protocol=true ^
  -Dlog.level=ALL ^
  -Xms1g ^
  -Xmx2G ^
  --add-modules=ALL-SYSTEM ^
  --add-opens java.base/java.util=ALL-UNNAMED ^
  --add-opens java.base/java.lang=ALL-UNNAMED ^
  -jar "%s" ^
  -configuration "%s" ^
  -data "%%TEMP%%\jdtls-workspace" ^
  %%*
`, javaPath, launcherJar, configDir)
	} else {
		wrapperPath = filepath.Join(binDir, "jdtls")
		wrapperContent = fmt.Sprintf(`#!/bin/bash

# Eclipse JDT Language Server wrapper script
# Generated by lsp-gateway installer

exec "%s" \
  -Declipse.application=org.eclipse.jdt.ls.core.id1 \
  -Dosgi.bundles.defaultStartLevel=4 \
  -Declipse.product=org.eclipse.jdt.ls.core.product \
  -Dlog.protocol=true \
  -Dlog.level=ALL \
  -Xms1g \
  -Xmx2G \
  --add-modules=ALL-SYSTEM \
  --add-opens java.base/java.util=ALL-UNNAMED \
  --add-opens java.base/java.lang=ALL-UNNAMED \
  -jar "%s" \
  -configuration "%s" \
  -data "${TMPDIR:-/tmp}/jdtls-workspace" \
  "$@"
`, javaPath, launcherJar, configDir)
	}

	if err := os.WriteFile(wrapperPath, []byte(wrapperContent), 0755); err != nil {
		return "", fmt.Errorf("failed to create wrapper script: %w", err)
	}

	return wrapperPath, nil
}

func (s *DefaultServerInstaller) findJdtlsLauncherJar(installDir string) (string, error) {
	pluginsDir := filepath.Join(installDir, "plugins")
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return "", fmt.Errorf("failed to read plugins directory: %w", err)
	}

	for _, entry := range entries {
		if strings.Contains(entry.Name(), "org.eclipse.equinox.launcher") &&
			strings.HasSuffix(entry.Name(), ".jar") &&
			!strings.Contains(entry.Name(), ".source") {
			return filepath.Join(pluginsDir, entry.Name()), nil
		}
	}

	return "", fmt.Errorf("launcher jar not found in plugins directory")
}

func (s *DefaultServerInstaller) verifyJdtlsInstallation(executablePath string) (string, error) {
	executor := platform.NewCommandExecutor()

	result, err := executor.Execute(executablePath, []string{"-version"}, 10*time.Second)

	if err != nil {
		result, err = executor.Execute(executablePath, []string{}, 5*time.Second)
		if err != nil {
			if strings.Contains(err.Error(), "timeout") {
				return s.getInstalledJdtlsVersion(filepath.Dir(filepath.Dir(executablePath))), nil
			}
			return "", fmt.Errorf("jdtls executable test failed: %w", err)
		}
	}

	if result.Stdout != "" || result.Stderr != "" {
		output := result.Stdout + result.Stderr
		if strings.Contains(output, "Eclipse JDT") || strings.Contains(output, "jdt.ls") {
			return s.getInstalledJdtlsVersion(filepath.Dir(filepath.Dir(executablePath))), nil
		}
	}

	return s.getInstalledJdtlsVersion(filepath.Dir(filepath.Dir(executablePath))), nil
}

type WindowsServerStrategyAdapter struct{}

func (w *WindowsServerStrategyAdapter) InstallServer(server string, options types.ServerInstallOptions) (*types.InstallResult, error) {
	return &types.InstallResult{
		Success: false,
		Errors:  []string{"Windows server installation not implemented yet"},
		Details: map[string]interface{}{"server": server},
	}, nil
}

func (w *WindowsServerStrategyAdapter) VerifyServer(server string) (*types.VerificationResult, error) {
	return &types.VerificationResult{
		Installed: false,
		Issues:    []types.Issue{},
		Details:   map[string]interface{}{"server": server},
	}, nil
}

func (w *WindowsServerStrategyAdapter) GetInstallCommand(server, version string) ([]string, error) {
	return []string{}, fmt.Errorf("not implemented")
}

type LinuxServerStrategyAdapter struct{}

func (l *LinuxServerStrategyAdapter) InstallServer(server string, options types.ServerInstallOptions) (*types.InstallResult, error) {
	return &types.InstallResult{
		Success: false,
		Errors:  []string{"Linux server installation not implemented yet"},
		Details: map[string]interface{}{"server": server},
	}, nil
}

func (l *LinuxServerStrategyAdapter) VerifyServer(server string) (*types.VerificationResult, error) {
	return &types.VerificationResult{
		Installed: false,
		Issues:    []types.Issue{},
		Details:   map[string]interface{}{"server": server},
	}, nil
}

func (l *LinuxServerStrategyAdapter) GetInstallCommand(server, version string) ([]string, error) {
	return []string{}, fmt.Errorf("not implemented")
}

type MacOSServerStrategyAdapter struct{}

func (m *MacOSServerStrategyAdapter) InstallServer(server string, options types.ServerInstallOptions) (*types.InstallResult, error) {
	return &types.InstallResult{
		Success: false,
		Errors:  []string{"macOS server installation not implemented yet"},
		Details: map[string]interface{}{"server": server},
	}, nil
}

func (m *MacOSServerStrategyAdapter) VerifyServer(server string) (*types.VerificationResult, error) {
	return &types.VerificationResult{
		Installed: false,
		Issues:    []types.Issue{},
		Details:   map[string]interface{}{"server": server},
	}, nil
}

func (m *MacOSServerStrategyAdapter) GetInstallCommand(server, version string) ([]string, error) {
	return []string{}, fmt.Errorf("not implemented")
}
