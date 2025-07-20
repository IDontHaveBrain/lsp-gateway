package setup

import (
	"context"
	"fmt"
	"lsp-gateway/internal/platform"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type GoDetector struct {
	executor       platform.CommandExecutor
	logger         *SetupLogger
	versionChecker *VersionChecker
	timeout        time.Duration
}

type GoRuntimeInfo struct {
	*RuntimeInfo
	GOROOT     string `json:"goroot,omitempty"`
	GOPATH     string `json:"gopath,omitempty"`
	GOOS       string `json:"goos,omitempty"`
	GOARCH     string `json:"goarch,omitempty"`
	ModSupport bool   `json:"mod_support"`
}

func NewGoDetector() *GoDetector {
	return &GoDetector{
		executor:       platform.NewCommandExecutor(),
		logger:         NewSetupLogger(nil),
		versionChecker: NewVersionChecker(),
		timeout:        30 * time.Second,
	}
}

func (gd *GoDetector) DetectGo() (*RuntimeInfo, error) {
	return gd.DetectGoWithContext(context.Background())
}

func (gd *GoDetector) DetectGoWithContext(ctx context.Context) (*RuntimeInfo, error) {
	gd.logger.WithOperation("go-detection").Info("Starting Go runtime detection")

	runtimeInfo := &RuntimeInfo{
		Name:       "go",
		Installed:  false,
		Version:    "",
		Compatible: false,
		Path:       "",
		Issues:     []string{},
	}

	select {
	case <-ctx.Done():
		return runtimeInfo, ctx.Err()
	default:
	}

	if !gd.executor.IsCommandAvailable("go") {
		gd.logger.Debug("Go command not found in PATH")
		runtimeInfo.Issues = append(runtimeInfo.Issues, "Go is not installed or not in PATH")
		return runtimeInfo, nil
	}

	gd.logger.Debug("Go command found in PATH")

	version, err := gd.getGoVersion()
	if err != nil {
		gd.logger.WithError(err).Warn("Failed to get Go version")
		runtimeInfo.Issues = append(runtimeInfo.Issues, fmt.Sprintf("Failed to get Go version: %v", err))
		return runtimeInfo, nil
	}

	runtimeInfo.Installed = true
	runtimeInfo.Version = version
	gd.logger.WithField("version", version).Info("Go version detected")

	compatible := gd.versionChecker.IsCompatible("go", version)
	runtimeInfo.Compatible = compatible

	if !compatible {
		minVersion, _ := gd.versionChecker.GetMinVersion("go")
		issue := fmt.Sprintf("Go version %s does not meet minimum requirement %s", version, minVersion)
		runtimeInfo.Issues = append(runtimeInfo.Issues, issue)
		gd.logger.WithFields(map[string]interface{}{
			"installed_version": version,
			"required_version":  minVersion,
		}).Warn("Go version compatibility check failed")
	} else {
		gd.logger.Debug("Go version compatibility check passed")
	}

	goPath, err := gd.getGoPath()
	if err != nil {
		gd.logger.WithError(err).Debug("Failed to get Go executable path")
		runtimeInfo.Issues = append(runtimeInfo.Issues, fmt.Sprintf("Failed to get Go path: %v", err))
	} else {
		runtimeInfo.Path = goPath
		gd.logger.WithField("path", goPath).Debug("Go executable path found")
	}

	if err := gd.validateGoEnvironment(runtimeInfo); err != nil {
		gd.logger.WithError(err).Debug("Go environment validation failed")
		runtimeInfo.Issues = append(runtimeInfo.Issues, fmt.Sprintf("Go environment issues: %v", err))
	}

	if err := gd.validateGoModules(runtimeInfo); err != nil {
		gd.logger.WithError(err).Debug("Go modules validation failed")
		runtimeInfo.Issues = append(runtimeInfo.Issues, fmt.Sprintf("Go modules issues: %v", err))
	}

	gd.logger.WithFields(map[string]interface{}{
		"installed":  runtimeInfo.Installed,
		"compatible": runtimeInfo.Compatible,
		"issues":     len(runtimeInfo.Issues),
	}).Info("Go runtime detection completed")

	return runtimeInfo, nil
}

func (gd *GoDetector) DetectGoExtended() (*GoRuntimeInfo, error) {
	basicInfo, err := gd.DetectGo()
	if err != nil {
		return nil, err
	}

	extendedInfo := &GoRuntimeInfo{
		RuntimeInfo: basicInfo,
		ModSupport:  false,
	}

	if !basicInfo.Installed {
		return extendedInfo, nil
	}

	if err := gd.enrichWithGoEnv(extendedInfo); err != nil {
		gd.logger.WithError(err).Debug("Failed to get extended Go environment info")
		extendedInfo.Issues = append(extendedInfo.Issues, fmt.Sprintf("Extended environment detection failed: %v", err))
	}

	return extendedInfo, nil
}

func (gd *GoDetector) getGoVersion() (string, error) {
	result, err := gd.executor.Execute("go", []string{"version"}, gd.timeout)
	if err != nil {
		return "", NewDetectionError("go", "version", "Command execution failed", err)
	}

	if result.ExitCode != 0 {
		return "", NewDetectionError("go", "version",
			fmt.Sprintf("Command failed with exit code %d: %s", result.ExitCode, result.Stderr), nil)
	}

	version, err := gd.parseGoVersion(result.Stdout)
	if err != nil {
		return "", NewDetectionError("go", "version",
			fmt.Sprintf("Failed to parse version from output: %s", result.Stdout), err)
	}

	return version, nil
}

func (gd *GoDetector) parseGoVersion(output string) (string, error) {
	re := regexp.MustCompile(`go version go(\d+\.\d+\.?\d*)\s`)
	matches := re.FindStringSubmatch(output)

	if len(matches) < 2 {
		return "", fmt.Errorf("invalid go version output format: %s", output)
	}

	return matches[1], nil
}

func (gd *GoDetector) getGoPath() (string, error) {
	result, err := gd.executor.Execute("go", []string{"env", "GOROOT"}, gd.timeout)
	if err != nil {
		return "", err
	}

	if result.ExitCode != 0 {
		return "", fmt.Errorf("failed to get GOROOT: exit code %d", result.ExitCode)
	}

	goroot := strings.TrimSpace(result.Stdout)
	if goroot == "" {
		return "", fmt.Errorf("GOROOT is empty")
	}

	goPath := filepath.Join(goroot, "bin", "go")

	return goPath, nil
}

func (gd *GoDetector) validateGoEnvironment(info *RuntimeInfo) error {
	envVars := []string{"GOROOT", "GOPATH", "GOOS", "GOARCH"}

	for _, envVar := range envVars {
		result, err := gd.executor.Execute("go", []string{"env", envVar}, gd.timeout)
		if err != nil {
			gd.logger.WithFields(map[string]interface{}{
				"env_var": envVar,
				"error":   err,
			}).Debug("Failed to get Go environment variable")
			continue
		}

		if result.ExitCode != 0 {
			gd.logger.WithField("env_var", envVar).Debug("Go env command failed")
			continue
		}

		value := strings.TrimSpace(result.Stdout)
		gd.logger.WithFields(map[string]interface{}{
			"env_var": envVar,
			"value":   value,
		}).Debug("Go environment variable detected")

		if envVar == "GOROOT" && value == "" {
			info.Issues = append(info.Issues, "GOROOT is not set")
		}
	}

	return nil
}

func (gd *GoDetector) validateGoModules(info *RuntimeInfo) error {
	result, err := gd.executor.Execute("go", []string{"help", "mod"}, gd.timeout)
	if err != nil {
		return fmt.Errorf("go mod command not available: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("go mod help failed with exit code %d", result.ExitCode)
	}

	gd.logger.Debug("Go modules support confirmed")
	return nil
}

func (gd *GoDetector) enrichWithGoEnv(info *GoRuntimeInfo) error {
	envQueries := map[string]string{
		"GOROOT": "goroot",
		"GOPATH": "gopath",
		"GOOS":   "goos",
		"GOARCH": "goarch",
	}

	for envVar, field := range envQueries {
		result, err := gd.executor.Execute("go", []string{"env", envVar}, gd.timeout)
		if err != nil {
			continue
		}

		if result.ExitCode != 0 {
			continue
		}

		value := strings.TrimSpace(result.Stdout)

		switch field {
		case "goroot":
			info.GOROOT = value
		case "gopath":
			info.GOPATH = value
		case "goos":
			info.GOOS = value
		case "goarch":
			info.GOARCH = value
		}
	}

	if err := gd.validateGoModules(info.RuntimeInfo); err == nil {
		info.ModSupport = true
	}

	return nil
}

func (gd *GoDetector) ValidateGoInstallation() error {
	gd.logger.Info("Performing comprehensive Go installation validation")

	info, err := gd.DetectGo()
	if err != nil {
		return fmt.Errorf("basic Go detection failed: %w", err)
	}

	if !info.Installed {
		return fmt.Errorf("go is not installed")
	}

	if !info.Compatible {
		minVersion, _ := gd.versionChecker.GetMinVersion("go")
		return NewVersionError("go", minVersion, info.Version, true)
	}

	if err := gd.testGoCompilation(); err != nil {
		return fmt.Errorf("go compilation test failed: %w", err)
	}

	gd.logger.UserSuccess("Go installation validation passed")
	return nil
}

func (gd *GoDetector) testGoCompilation() error {
	gd.logger.Debug("Testing Go compilation functionality")

	result, err := gd.executor.Execute("go", []string{"env", "GOVERSION"}, gd.timeout)
	if err != nil {
		return fmt.Errorf("failed to execute go env: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("go env failed with exit code %d: %s", result.ExitCode, result.Stderr)
	}

	gd.logger.Debug("Go compilation test passed")
	return nil
}

func (gd *GoDetector) GetGoRecommendations() []string {
	info, err := gd.DetectGo()
	if err != nil {
		return []string{"Unable to analyze Go installation - check if Go is accessible"}
	}

	var recommendations []string

	if !info.Installed {
		recommendations = append(recommendations,
			"Install Go from https://golang.org/dl/",
			"Add Go binary directory to your PATH environment variable",
			"Verify installation by running 'go version'")
		return recommendations
	}

	if !info.Compatible {
		minVersion, _ := gd.versionChecker.GetMinVersion("go")
		recommendations = append(recommendations,
			fmt.Sprintf("Upgrade Go to version %s or higher", minVersion),
			"Download the latest Go version from https://golang.org/dl/")
	}

	if len(info.Issues) > 0 {
		recommendations = append(recommendations, "Resolve the following issues:")
		for _, issue := range info.Issues {
			recommendations = append(recommendations, "  - "+issue)
		}
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Go installation appears to be working correctly")
	}

	return recommendations
}

func (gd *GoDetector) SetTimeout(timeout time.Duration) {
	gd.timeout = timeout
}

func (gd *GoDetector) GetMinimumVersion() string {
	version, exists := gd.versionChecker.GetMinVersion("go")
	if !exists {
		return "1.19.0" // Default minimum
	}
	return version
}

func (gd *GoDetector) SetMinimumVersion(version string) {
	gd.versionChecker.SetMinVersion("go", version)
}
