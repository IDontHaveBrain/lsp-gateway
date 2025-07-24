package setup

import (
	"context"
	"fmt"
	"lsp-gateway/internal/platform"
	"os/exec"
	"strings"
	"time"
)

type RuntimeDetector interface {
	DetectGo(ctx context.Context) (*RuntimeInfo, error)

	DetectPython(ctx context.Context) (*RuntimeInfo, error)

	DetectNodejs(ctx context.Context) (*RuntimeInfo, error)

	DetectJava(ctx context.Context) (*RuntimeInfo, error)

	DetectAll(ctx context.Context) (*DetectionReport, error)

	SetLogger(logger *SetupLogger)

	SetTimeout(timeout time.Duration)
}

type RuntimeInfo struct {
	Name          string                 // Runtime name (e.g., "go", "python", "nodejs", "java")
	Installed     bool                   // Whether the runtime is installed
	Version       string                 // Detected version (raw output)
	ParsedVersion *Version               // Parsed semantic version
	Compatible    bool                   // Whether the version meets minimum requirements
	MinVersion    string                 // Minimum required version
	Path          string                 // Path to the runtime executable
	WorkingDir    string                 // Working directory used for detection
	DetectionCmd  string                 // Command used for detection
	Issues        []string               // Any issues found during detection
	Warnings      []string               // Non-critical warnings
	Metadata      map[string]interface{} // Additional runtime-specific metadata
	DetectedAt    time.Time              // When detection was performed
	Duration      time.Duration          // Time taken for detection
}

type DetectionReport struct {
	Timestamp    time.Time               // When detection was performed
	Platform     platform.Platform       // Current platform
	Architecture platform.Architecture   // Current architecture
	Runtimes     map[string]*RuntimeInfo // Detection results for each runtime
	Summary      DetectionSummary        // Summary of detection results
	Duration     time.Duration           // Total time taken for detection
	Issues       []string                // Global issues found during detection
	Warnings     []string                // Global warnings
	SessionID    string                  // Unique identifier for this detection session
	Metadata     map[string]interface{}  // Additional metadata
}

type DetectionSummary struct {
	TotalRuntimes      int           // Total number of runtimes checked
	InstalledRuntimes  int           // Number of installed runtimes
	CompatibleRuntimes int           // Number of compatible runtimes
	IssuesFound        int           // Total issues found
	WarningsFound      int           // Total warnings found
	SuccessRate        float64       // Percentage of successful detections
	AverageDetectTime  time.Duration // Average time per runtime detection
}

type DetectionConfig struct {
	Timeout         time.Duration     // Timeout for detection operations
	WorkingDir      string            // Working directory for command execution
	EnableParallel  bool              // Enable parallel detection of runtimes
	CustomCommands  map[string]string // Custom commands for specific runtimes
	EnvironmentVars map[string]string // Additional environment variables
}

type DefaultRuntimeDetector struct {
	versionChecker *VersionChecker
	logger         *SetupLogger
	config         *DetectionConfig
	executor       platform.CommandExecutor
	sessionID      string
}

func NewRuntimeDetector() *DefaultRuntimeDetector {
	return &DefaultRuntimeDetector{
		versionChecker: NewVersionChecker(),
		logger:         NewSetupLogger(nil), // Default logger
		config: &DetectionConfig{
			Timeout:         30 * time.Second,
			WorkingDir:      "",
			EnableParallel:  true,
			CustomCommands:  make(map[string]string),
			EnvironmentVars: make(map[string]string),
		},
		executor:  platform.NewCommandExecutor(),
		sessionID: fmt.Sprintf("detect_%d", time.Now().Unix()),
	}
}

func (d *DefaultRuntimeDetector) SetLogger(logger *SetupLogger) {
	if logger != nil {
		d.logger = logger
	}
}

func (d *DefaultRuntimeDetector) SetTimeout(timeout time.Duration) {
	if timeout > 0 {
		d.config.Timeout = timeout
	}
}

// SetExecutor sets the command executor for testing purposes
func (d *DefaultRuntimeDetector) SetExecutor(executor platform.CommandExecutor) {
	if executor != nil {
		d.executor = executor
	}
}

// SetCustomCommand sets a custom command for a specific runtime
func (d *DefaultRuntimeDetector) SetCustomCommand(runtime, command string) {
	if runtime != "" && command != "" {
		d.config.CustomCommands[runtime] = command
	}
}

func (d *DefaultRuntimeDetector) DetectGo(ctx context.Context) (*RuntimeInfo, error) {
	return d.detectRuntime(ctx, "go", []string{"go", "version"})
}

func (d *DefaultRuntimeDetector) DetectPython(ctx context.Context) (*RuntimeInfo, error) {
	info, err := d.detectRuntime(ctx, "python", []string{"python3", "--version"})
	if err != nil || !info.Installed {
		return d.detectRuntime(ctx, "python", []string{"python", "--version"})
	}
	return info, err
}

func (d *DefaultRuntimeDetector) DetectNodejs(ctx context.Context) (*RuntimeInfo, error) {
	return d.detectRuntime(ctx, "nodejs", []string{"node", "--version"})
}

func (d *DefaultRuntimeDetector) DetectJava(ctx context.Context) (*RuntimeInfo, error) {
	return d.detectRuntime(ctx, "java", []string{"java", "-version"})
}

func (d *DefaultRuntimeDetector) DetectAll(ctx context.Context) (*DetectionReport, error) {
	startTime := time.Now()

	d.logger.WithOperation("detect-all-runtimes").Info("Starting runtime detection for all supported runtimes")

	report := &DetectionReport{
		Timestamp:    startTime,
		Platform:     platform.GetCurrentPlatform(),
		Architecture: platform.GetCurrentArchitecture(),
		Runtimes:     make(map[string]*RuntimeInfo),
		SessionID:    d.sessionID,
		Metadata:     make(map[string]interface{}),
		Issues:       []string{},
		Warnings:     []string{},
	}

	type runtimeDetector struct {
		name   string
		detect func(context.Context) (*RuntimeInfo, error)
	}

	detectors := []runtimeDetector{
		{"go", d.DetectGo},
		{"python", d.DetectPython},
		{"nodejs", d.DetectNodejs},
		{"java", d.DetectJava},
	}

	var totalDetectionTime time.Duration
	successfulDetections := 0

	for i, detector := range detectors {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		d.logger.UserInfof("Detecting runtime %d/%d: %s", i+1, len(detectors), detector.name)
		d.logger.WithField("runtime", detector.name).Debug("Detecting runtime")

		runtime, err := detector.detect(ctx)
		if err != nil {
			d.logger.WithError(err).WithField("runtime", detector.name).Warn("Runtime detection failed")
			report.Issues = append(report.Issues, fmt.Sprintf("Failed to detect %s: %v", detector.name, err))
			runtime = &RuntimeInfo{
				Name:       detector.name,
				Installed:  false,
				Version:    "",
				Compatible: false,
				Path:       "",
				Issues:     []string{err.Error()},
				Warnings:   []string{},
				Metadata:   make(map[string]interface{}),
				DetectedAt: time.Now(),
				Duration:   0,
			}
		} else {
			successfulDetections++
		}

		totalDetectionTime += runtime.Duration
		report.Runtimes[runtime.Name] = runtime

		d.logger.WithFields(map[string]interface{}{
			"runtime":   runtime.Name,
			"installed": runtime.Installed,
			"version":   runtime.Version,
			"duration":  runtime.Duration,
		}).Info(RUNTIME_DETECTION_COMPLETED)
	}

	summary := DetectionSummary{
		TotalRuntimes:     len(report.Runtimes),
		AverageDetectTime: totalDetectionTime / time.Duration(len(detectors)),
	}

	for _, runtime := range report.Runtimes {
		if runtime.Installed {
			summary.InstalledRuntimes++
		}
		if runtime.Compatible {
			summary.CompatibleRuntimes++
		}
		summary.IssuesFound += len(runtime.Issues)
		summary.WarningsFound += len(runtime.Warnings)
	}

	if summary.TotalRuntimes > 0 {
		summary.SuccessRate = float64(successfulDetections) / float64(summary.TotalRuntimes) * 100.0
	}

	report.Summary = summary
	report.Duration = time.Since(startTime)

	d.logger.WithFields(map[string]interface{}{
		"total_duration":      report.Duration,
		"runtimes_detected":   summary.TotalRuntimes,
		"runtimes_installed":  summary.InstalledRuntimes,
		"runtimes_compatible": summary.CompatibleRuntimes,
		"success_rate":        summary.SuccessRate,
	}).Info(RUNTIME_DETECTION_COMPLETED)

	d.logger.UserSuccess(fmt.Sprintf("Runtime detection completed: %d/%d runtimes installed",
		summary.InstalledRuntimes, summary.TotalRuntimes))

	return report, nil
}

func (d *DefaultRuntimeDetector) detectRuntime(ctx context.Context, runtimeName string, cmd []string) (*RuntimeInfo, error) {
	startTime := time.Now()
	info := d.initializeRuntimeInfo(runtimeName, startTime)

	select {
	case <-ctx.Done():
		return info, ctx.Err()
	default:
	}

	cmd = d.resolveDetectionCommand(runtimeName, cmd)
	if err := d.validateDetectionCommand(cmd, info); err != nil {
		info.Duration = time.Since(startTime)
		return info, err
	}

	if !d.isCommandAvailable(cmd, info, startTime) {
		return info, nil
	}

	result, err := d.executeDetectionCommand(ctx, cmd, info, startTime)
	if err != nil {
		return info, err
	}

	d.processDetectionResult(result, cmd, info)
	d.validateVersionCompatibility(runtimeName, info)
	d.populateRuntimeMetadata(result, info)
	d.logDetectionCompletion(runtimeName, info)

	return info, nil
}

func (d *DefaultRuntimeDetector) initializeRuntimeInfo(runtimeName string, startTime time.Time) *RuntimeInfo {
	return &RuntimeInfo{
		Name:       runtimeName,
		Installed:  false,
		Version:    "",
		Compatible: false,
		Path:       "",
		Issues:     []string{},
		Warnings:   []string{},
		Metadata:   make(map[string]interface{}),
		DetectedAt: startTime,
		WorkingDir: d.config.WorkingDir,
	}
}

func (d *DefaultRuntimeDetector) resolveDetectionCommand(runtimeName string, cmd []string) []string {
	if customCmd, exists := d.config.CustomCommands[runtimeName]; exists {
		parts := strings.Fields(customCmd)
		if len(parts) > 0 {
			return parts
		}
	}
	return cmd
}

func (d *DefaultRuntimeDetector) validateDetectionCommand(cmd []string, info *RuntimeInfo) error {
	if len(cmd) == 0 {
		info.Issues = append(info.Issues, "No detection command specified")
		return fmt.Errorf("no detection command for runtime %s", info.Name)
	}

	info.DetectionCmd = strings.Join(cmd, " ")
	d.logger.WithFields(map[string]interface{}{
		"runtime": info.Name,
		"command": info.DetectionCmd,
		"timeout": d.config.Timeout,
	}).Debug("Executing runtime detection command")

	return nil
}

func (d *DefaultRuntimeDetector) isCommandAvailable(cmd []string, info *RuntimeInfo, startTime time.Time) bool {
	if !platform.IsCommandAvailable(cmd[0]) {
		info.Issues = append(info.Issues, fmt.Sprintf("Command '%s' not found in PATH", cmd[0]))
		info.Duration = time.Since(startTime)
		d.logger.WithField("command", cmd[0]).Debug("Command not found in PATH")
		return false
	}
	return true
}

func (d *DefaultRuntimeDetector) executeDetectionCommand(ctx context.Context, cmd []string, info *RuntimeInfo, startTime time.Time) (*platform.Result, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	result, err := d.executor.ExecuteWithEnv(cmd[0], cmd[1:], d.config.EnvironmentVars, d.config.Timeout)
	info.Duration = time.Since(startTime)

	if err != nil {
		return d.handleExecutionError(timeoutCtx, result, cmd, info, err)
	}

	return result, nil
}

func (d *DefaultRuntimeDetector) handleExecutionError(timeoutCtx context.Context, result *platform.Result, cmd []string, info *RuntimeInfo, err error) (*platform.Result, error) {
	select {
	case <-timeoutCtx.Done():
		info.Issues = append(info.Issues, fmt.Sprintf("Detection command timed out after %v", d.config.Timeout))
		d.logger.WithField("timeout", d.config.Timeout).Warn("Runtime detection timed out")
		return result, nil
	default:
	}

	if result != nil && result.ExitCode != 0 {
		return d.handleNonZeroExit(result, cmd, info)
	}

	info.Issues = append(info.Issues, fmt.Sprintf("Failed to execute detection command: %v", err))
	return result, err
}

func (d *DefaultRuntimeDetector) handleNonZeroExit(result *platform.Result, cmd []string, info *RuntimeInfo) (*platform.Result, error) {
	output := d.extractOutput(result)
	if output != "" {
		info.Installed = true
		info.Version = output
		d.resolveExecutablePath(cmd[0], info)
	} else {
		info.Issues = append(info.Issues, fmt.Sprintf("Command failed with exit code %d: %s", result.ExitCode, result.Stderr))
	}
	return result, nil
}

func (d *DefaultRuntimeDetector) extractOutput(result *platform.Result) string {
	output := strings.TrimSpace(result.Stdout)
	if output == "" {
		output = strings.TrimSpace(result.Stderr)
	}
	return output
}

func (d *DefaultRuntimeDetector) resolveExecutablePath(cmdName string, info *RuntimeInfo) {
	if execPath, err := exec.LookPath(cmdName); err == nil {
		info.Path = execPath
		return
	}

	var pathCmd []string
	if platform.IsWindows() {
		pathCmd = []string{"where", cmdName}
	} else {
		pathCmd = []string{"which", cmdName}
	}

	if pathResult, pathErr := d.executor.Execute(pathCmd[0], pathCmd[1:], d.config.Timeout); pathErr == nil {
		info.Path = strings.TrimSpace(pathResult.Stdout)
	}
}

func (d *DefaultRuntimeDetector) processDetectionResult(result *platform.Result, cmd []string, info *RuntimeInfo) {
	if result != nil && info.Installed {
		return
	}

	info.Installed = true
	info.Version = d.extractOutput(result)

	if execPath, err := exec.LookPath(cmd[0]); err == nil {
		info.Path = execPath
	}
}

func (d *DefaultRuntimeDetector) validateVersionCompatibility(runtimeName string, info *RuntimeInfo) {
	if !info.Installed || info.Version == "" {
		return
	}

	parsedVersion, err := ParseVersion(info.Version)
	if err != nil {
		info.Warnings = append(info.Warnings, fmt.Sprintf("Could not parse version string: %s", info.Version))
		info.Compatible = true
		return
	}

	info.ParsedVersion = parsedVersion
	d.checkMinimumVersionRequirements(runtimeName, info)
}

func (d *DefaultRuntimeDetector) checkMinimumVersionRequirements(runtimeName string, info *RuntimeInfo) {
	minVersion, exists := d.versionChecker.GetMinVersion(runtimeName)
	if !exists {
		info.Compatible = true
		return
	}

	info.MinVersion = minVersion
	info.Compatible = d.versionChecker.IsCompatible(runtimeName, info.Version)

	if !info.Compatible {
		info.Warnings = append(info.Warnings,
			fmt.Sprintf("Version %s is below minimum required version %s",
				info.Version, minVersion))
	}
}

func (d *DefaultRuntimeDetector) populateRuntimeMetadata(result *platform.Result, info *RuntimeInfo) {
	info.Metadata["command_used"] = info.DetectionCmd
	info.Metadata["exit_code"] = 0
	if result != nil {
		info.Metadata["exit_code"] = result.ExitCode
		info.Metadata["stdout"] = result.Stdout
		info.Metadata["stderr"] = result.Stderr
	}
	info.Metadata["platform"] = platform.GetCurrentPlatform().String()
	info.Metadata["architecture"] = platform.GetCurrentArchitecture().String()
}

func (d *DefaultRuntimeDetector) logDetectionCompletion(runtimeName string, info *RuntimeInfo) {
	d.logger.WithFields(map[string]interface{}{
		"runtime":    runtimeName,
		"installed":  info.Installed,
		"version":    info.Version,
		"compatible": info.Compatible,
		"path":       info.Path,
		"duration":   info.Duration,
		"issues":     len(info.Issues),
		"warnings":   len(info.Warnings),
	}).Debug(RUNTIME_DETECTION_COMPLETED)
}
