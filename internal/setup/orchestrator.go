package setup

import (
	"context"
	"fmt"
	"lsp-gateway/internal/config"
	"lsp-gateway/internal/installer"
	"lsp-gateway/internal/types"
	"time"
)

// SetupOrchestrator coordinates existing components for unified auto-setup workflow
type SetupOrchestrator struct {
	runtimeDetector  RuntimeDetector
	runtimeInstaller types.RuntimeInstaller
	serverInstaller  types.ServerInstaller
	configGenerator  ConfigGenerator
	logger           *SetupLogger
	sessionID        string
}

// SetupOptions configures the setup orchestration workflow
type SetupOptions struct {
	// Timeout settings
	Timeout              time.Duration `json:"timeout"`
	RuntimeDetectTimeout time.Duration `json:"runtime_detect_timeout"`
	InstallTimeout       time.Duration `json:"install_timeout"`

	// Control options
	Force                   bool `json:"force"`
	DryRun                  bool `json:"dry_run"`
	SkipRuntimeInstallation bool `json:"skip_runtime_installation"`
	SkipServerInstallation  bool `json:"skip_server_installation"`
	SkipConfigGeneration    bool `json:"skip_config_generation"`
	SkipVerification        bool `json:"skip_verification"`
	EnableParallelInstalls  bool `json:"enable_parallel_installs"`
	ContinueOnError         bool `json:"continue_on_error"`

	// Selective runtime/server installation
	SelectedRuntimes []string `json:"selected_runtimes"`
	SelectedServers  []string `json:"selected_servers"`
	ExcludeRuntimes  []string `json:"exclude_runtimes"`
	ExcludeServers   []string `json:"exclude_servers"`

	// Installation options
	InstallLatest         bool              `json:"install_latest"`
	ForceReinstall        bool              `json:"force_reinstall"`
	RuntimeVersions       map[string]string `json:"runtime_versions"`
	ServerVersions        map[string]string `json:"server_versions"`
	PlatformOverride      string            `json:"platform_override"`
	InstallationDirectory string            `json:"installation_directory"`

	// Configuration options
	ConfigOutputPath  string                 `json:"config_output_path"`
	ConfigPort        int                    `json:"config_port"`
	ConfigOverrides   map[string]interface{} `json:"config_overrides"`
	UseExistingConfig bool                   `json:"use_existing_config"`
	MergeWithExisting bool                   `json:"merge_with_existing"`

	// Progress and logging
	EnableProgressReporting bool `json:"enable_progress_reporting"`
	VerboseLogging          bool `json:"verbose_logging"`
	QuietMode               bool `json:"quiet_mode"`
	EnableMetrics           bool `json:"enable_metrics"`

	// Recovery options
	AutoRetryFailures   bool          `json:"auto_retry_failures"`
	MaxRetryAttempts    int           `json:"max_retry_attempts"`
	RetryDelay          time.Duration `json:"retry_delay"`
	EnableErrorRecovery bool          `json:"enable_error_recovery"`
	RecoveryStrategies  []string      `json:"recovery_strategies"`

	// Advanced options
	WorkingDirectory      string            `json:"working_directory"`
	EnvironmentVariables  map[string]string `json:"environment_variables"`
	CustomCommands        map[string]string `json:"custom_commands"`
	ValidateInstallations bool              `json:"validate_installations"`
	GenerateReport        bool              `json:"generate_report"`
}

// SetupResult contains the comprehensive result of the setup orchestration
type SetupResult struct {
	// Overall status
	Success   bool          `json:"success"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration"`
	SessionID string        `json:"session_id"`

	// Detection results
	DetectionReport   *DetectionReport        `json:"detection_report,omitempty"`
	DetectionSuccess  bool                    `json:"detection_success"`
	DetectionDuration time.Duration           `json:"detection_duration"`
	DetectedRuntimes  map[string]*RuntimeInfo `json:"detected_runtimes"`

	// Installation results
	RuntimeInstallResults map[string]*types.InstallResult `json:"runtime_install_results"`
	ServerInstallResults  map[string]*types.InstallResult `json:"server_install_results"`
	InstallationSummary   InstallationSummary             `json:"installation_summary"`

	// Configuration results
	ConfigGeneration *ConfigGenerationResult `json:"config_generation,omitempty"`
	GeneratedConfig  *config.GatewayConfig   `json:"generated_config,omitempty"`
	ConfigPath       string                  `json:"config_path,omitempty"`

	// Verification results
	RuntimeVerifications map[string]*types.VerificationResult `json:"runtime_verifications"`
	ServerVerifications  map[string]*types.VerificationResult `json:"server_verifications"`

	// Progress and logging
	ProgressUpdates []ProgressUpdate `json:"progress_updates"`
	LogEntries      []string         `json:"log_entries"`
	Metrics         *SetupMetrics    `json:"metrics,omitempty"`

	// Issues and recommendations
	Errors          []SetupError     `json:"errors"`
	Warnings        []string         `json:"warnings"`
	Messages        []string         `json:"messages"`
	Recommendations []string         `json:"recommendations"`
	NextSteps       []string         `json:"next_steps"`
	RecoveryActions []RecoveryAction `json:"recovery_actions"`

	// Metadata
	Options  *SetupOptions          `json:"options"`
	Metadata map[string]interface{} `json:"metadata"`
}

// InstallationSummary provides a high-level overview of installation results
type InstallationSummary struct {
	RuntimesAttempted  int `json:"runtimes_attempted"`
	RuntimesSuccessful int `json:"runtimes_successful"`
	RuntimesFailed     int `json:"runtimes_failed"`
	RuntimesSkipped    int `json:"runtimes_skipped"`

	ServersAttempted  int `json:"servers_attempted"`
	ServersSuccessful int `json:"servers_successful"`
	ServersFailed     int `json:"servers_failed"`
	ServersSkipped    int `json:"servers_skipped"`

	TotalInstallTime   time.Duration `json:"total_install_time"`
	AverageInstallTime time.Duration `json:"average_install_time"`
}

// ProgressUpdate tracks progress throughout the setup process
type ProgressUpdate struct {
	Timestamp time.Time     `json:"timestamp"`
	Stage     string        `json:"stage"`
	Operation string        `json:"operation"`
	Progress  *ProgressInfo `json:"progress,omitempty"`
	Message   string        `json:"message"`
	Component string        `json:"component,omitempty"`
}

// RecoveryAction represents an action taken to recover from an error
type RecoveryAction struct {
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Target      string                 `json:"target"`
	Success     bool                   `json:"success"`
	Details     map[string]interface{} `json:"details"`
	Timestamp   time.Time              `json:"timestamp"`
}

// NewSetupOrchestrator creates a new setup orchestrator with default dependencies
func NewSetupOrchestrator() *SetupOrchestrator {
	logger := NewSetupLogger(&SetupLoggerConfig{
		Level:                  LogLevelInfo,
		Component:              "orchestrator",
		EnableJSON:             false,
		EnableUserMessages:     true,
		EnableProgressTracking: true,
		EnableMetrics:          true,
		SessionID:              generateSessionID(),
	})

	runtimeInstaller := installer.NewRuntimeInstaller()
	serverInstaller := installer.NewServerInstaller(runtimeInstaller)

	orchestrator := &SetupOrchestrator{
		runtimeDetector:  NewRuntimeDetector(),
		runtimeInstaller: runtimeInstaller,
		serverInstaller:  serverInstaller,
		configGenerator:  NewConfigGenerator(),
		logger:           logger,
		sessionID:        logger.config.SessionID,
	}

	// Configure components with logger
	orchestrator.runtimeDetector.SetLogger(logger)
	orchestrator.configGenerator.SetLogger(logger)

	return orchestrator
}

// NewSetupOrchestratorWithDependencies creates orchestrator with custom dependencies for testing
func NewSetupOrchestratorWithDependencies(
	detector RuntimeDetector,
	runtimeInstaller types.RuntimeInstaller,
	serverInstaller types.ServerInstaller,
	configGenerator ConfigGenerator,
	logger *SetupLogger,
) *SetupOrchestrator {
	if logger == nil {
		logger = NewSetupLogger(nil)
	}

	orchestrator := &SetupOrchestrator{
		runtimeDetector:  detector,
		runtimeInstaller: runtimeInstaller,
		serverInstaller:  serverInstaller,
		configGenerator:  configGenerator,
		logger:           logger,
		sessionID:        logger.config.SessionID,
	}

	// Configure components with logger
	if detector != nil {
		detector.SetLogger(logger)
	}
	if configGenerator != nil {
		configGenerator.SetLogger(logger)
	}

	return orchestrator
}

// RunFullSetup orchestrates the complete setup workflow
func (o *SetupOrchestrator) RunFullSetup(ctx context.Context, options *SetupOptions) (*SetupResult, error) {
	startTime := time.Now()

	if options == nil {
		options = DefaultSetupOptions()
	}

	result := o.initializeSetupResult(startTime, options)

	o.logger.WithOperation("full-setup").Info("Starting comprehensive auto-setup workflow")

	// Configure components based on options
	if err := o.configureComponents(options); err != nil {
		return o.finalizeResult(result, false, fmt.Errorf("component configuration failed: %w", err))
	}

	// Stage 1: Runtime Detection
	if err := o.performRuntimeDetection(ctx, options, result); err != nil {
		if !options.ContinueOnError {
			return o.finalizeResult(result, false, fmt.Errorf("runtime detection failed: %w", err))
		}
		o.addError(result, "detection", "Runtime detection failed", err, "warning", true,
			[]string{"Continue with manual runtime configuration", "Check system PATH"})
	}

	// Stage 2: Runtime Installation
	if !options.SkipRuntimeInstallation && !options.DryRun {
		if err := o.performRuntimeInstallation(ctx, options, result); err != nil {
			if !options.ContinueOnError {
				return o.finalizeResult(result, false, fmt.Errorf("runtime installation failed: %w", err))
			}
			o.addError(result, "runtime_installation", "Runtime installation failed", err, "high", true,
				[]string{"Install runtimes manually", "Use package manager", "Check system requirements"})
		}
	}

	// Stage 3: Server Installation
	if !options.SkipServerInstallation && !options.DryRun {
		if err := o.performServerInstallation(ctx, options, result); err != nil {
			if !options.ContinueOnError {
				return o.finalizeResult(result, false, fmt.Errorf("server installation failed: %w", err))
			}
			o.addError(result, "server_installation", "Server installation failed", err, "medium", true,
				[]string{"Install language servers manually", "Check runtime dependencies"})
		}
	}

	// Stage 4: Verification
	if !options.SkipVerification {
		if err := o.performVerification(ctx, options, result); err != nil {
			if !options.ContinueOnError {
				return o.finalizeResult(result, false, fmt.Errorf("verification failed: %w", err))
			}
			o.addError(result, "verification", "Installation verification failed", err, "medium", true,
				[]string{"Check installation paths", "Verify environment variables"})
		}
	}

	// Stage 5: Configuration Generation
	if !options.SkipConfigGeneration {
		if err := o.performConfigGeneration(ctx, options, result); err != nil {
			if !options.ContinueOnError {
				return o.finalizeResult(result, false, fmt.Errorf("configuration generation failed: %w", err))
			}
			o.addError(result, "configuration", "Configuration generation failed", err, "medium", true,
				[]string{"Use default configuration", "Configure manually"})
		}
	}

	// Generate final recommendations and next steps
	o.generateRecommendations(result)

	success := len(result.Errors) == 0 || (options.ContinueOnError && o.hasMinimumRequirements(result))
	return o.finalizeResult(result, success, nil)
}

// DefaultSetupOptions returns sensible default options for setup orchestration
func DefaultSetupOptions() *SetupOptions {
	return &SetupOptions{
		Timeout:                 30 * time.Minute,
		RuntimeDetectTimeout:    2 * time.Minute,
		InstallTimeout:          10 * time.Minute,
		Force:                   false,
		DryRun:                  false,
		SkipRuntimeInstallation: false,
		SkipServerInstallation:  false,
		SkipConfigGeneration:    false,
		SkipVerification:        false,
		EnableParallelInstalls:  true,
		ContinueOnError:         true,
		SelectedRuntimes:        []string{}, // Empty means all
		SelectedServers:         []string{}, // Empty means all
		ExcludeRuntimes:         []string{},
		ExcludeServers:          []string{},
		InstallLatest:           true,
		ForceReinstall:          false,
		RuntimeVersions:         make(map[string]string),
		ServerVersions:          make(map[string]string),
		ConfigPort:              8080,
		ConfigOverrides:         make(map[string]interface{}),
		UseExistingConfig:       false,
		MergeWithExisting:       true,
		EnableProgressReporting: true,
		VerboseLogging:          false,
		QuietMode:               false,
		EnableMetrics:           true,
		AutoRetryFailures:       true,
		MaxRetryAttempts:        3,
		RetryDelay:              5 * time.Second,
		EnableErrorRecovery:     true,
		RecoveryStrategies:      []string{"retry", "fallback", "skip"},
		EnvironmentVariables:    make(map[string]string),
		CustomCommands:          make(map[string]string),
		ValidateInstallations:   true,
		GenerateReport:          true,
	}
}

// initializeSetupResult creates initial result structure
func (o *SetupOrchestrator) initializeSetupResult(startTime time.Time, options *SetupOptions) *SetupResult {
	return &SetupResult{
		Success:               false,
		StartTime:             startTime,
		SessionID:             o.sessionID,
		DetectedRuntimes:      make(map[string]*RuntimeInfo),
		RuntimeInstallResults: make(map[string]*types.InstallResult),
		ServerInstallResults:  make(map[string]*types.InstallResult),
		RuntimeVerifications:  make(map[string]*types.VerificationResult),
		ServerVerifications:   make(map[string]*types.VerificationResult),
		ProgressUpdates:       []ProgressUpdate{},
		LogEntries:            []string{},
		Errors:                []SetupError{},
		Warnings:              []string{},
		Messages:              []string{},
		Recommendations:       []string{},
		NextSteps:             []string{},
		RecoveryActions:       []RecoveryAction{},
		Options:               options,
		Metadata:              make(map[string]interface{}),
	}
}

// configureComponents configures orchestrator components based on options
func (o *SetupOrchestrator) configureComponents(options *SetupOptions) error {
	// Configure runtime detector timeouts
	if options.RuntimeDetectTimeout > 0 {
		o.runtimeDetector.SetTimeout(options.RuntimeDetectTimeout)
	}

	// Configure custom commands
	if detector, ok := o.runtimeDetector.(*DefaultRuntimeDetector); ok {
		for runtime, command := range options.CustomCommands {
			detector.SetCustomCommand(runtime, command)
		}
	}

	return nil
}

// performRuntimeDetection executes runtime detection phase
func (o *SetupOrchestrator) performRuntimeDetection(ctx context.Context, options *SetupOptions, result *SetupResult) error {
	progress := NewProgressInfo("Runtime Detection", 1)
	o.updateProgress(result, "detection", "runtime-detection", progress, "Detecting installed runtimes")

	o.logger.UserProgress("Detecting runtimes", progress)

	detectionCtx := ctx
	if options.RuntimeDetectTimeout > 0 {
		var cancel context.CancelFunc
		detectionCtx, cancel = context.WithTimeout(ctx, options.RuntimeDetectTimeout)
		defer cancel()
	}

	detectStart := time.Now()
	detectionReport, err := o.runtimeDetector.DetectAll(detectionCtx)
	result.DetectionDuration = time.Since(detectStart)

	if err != nil {
		result.DetectionSuccess = false
		return fmt.Errorf("runtime detection failed: %w", err)
	}

	result.DetectionReport = detectionReport
	result.DetectionSuccess = true
	result.DetectedRuntimes = detectionReport.Runtimes

	progress.Update(1, "Detection complete")
	o.updateProgress(result, "detection", "runtime-detection", progress, fmt.Sprintf("Detected %d runtimes", len(detectionReport.Runtimes)))

	// Filter runtimes based on options
	filteredRuntimes := o.filterRuntimes(detectionReport.Runtimes, options)
	result.Metadata["filtered_runtimes"] = filteredRuntimes

	o.logger.UserSuccess(fmt.Sprintf("Runtime detection completed: %d/%d runtimes available",
		detectionReport.Summary.InstalledRuntimes, detectionReport.Summary.TotalRuntimes))

	return nil
}

// performRuntimeInstallation executes runtime installation phase
func (o *SetupOrchestrator) performRuntimeInstallation(ctx context.Context, options *SetupOptions, result *SetupResult) error {
	if result.DetectionReport == nil {
		return fmt.Errorf("runtime detection must be completed before installation")
	}

	runtimesToInstall := o.determineRuntimesToInstall(result.DetectedRuntimes, options)
	if len(runtimesToInstall) == 0 {
		o.logger.UserInfo("No runtimes need installation")
		return nil
	}

	progress := NewProgressInfo("Runtime Installation", int64(len(runtimesToInstall)))
	o.updateProgress(result, "installation", "runtime-installation", progress, fmt.Sprintf("Installing %d runtimes", len(runtimesToInstall)))

	installStart := time.Now()
	completed := int64(0)

	for _, runtime := range runtimesToInstall {
		if err := ctx.Err(); err != nil {
			return err
		}

		o.logger.UserProgress(fmt.Sprintf("Installing %s runtime", runtime), progress)

		installOptions := o.buildRuntimeInstallOptions(runtime, options)
		installResult, err := o.runtimeInstaller.Install(runtime, installOptions)

		result.RuntimeInstallResults[runtime] = installResult
		completed++
		progress.Update(completed, runtime)

		if err != nil {
			o.addError(result, fmt.Sprintf("%s_runtime", runtime), fmt.Sprintf("Failed to install %s runtime", runtime), err, "high", true,
				[]string{fmt.Sprintf("Install %s manually", runtime), "Check system requirements"})

			if !options.ContinueOnError {
				return fmt.Errorf("runtime installation failed for %s: %w", runtime, err)
			}
			result.InstallationSummary.RuntimesFailed++
		} else if installResult != nil && installResult.Success {
			result.InstallationSummary.RuntimesSuccessful++
			o.logger.UserSuccess(fmt.Sprintf("Successfully installed %s runtime", runtime))
		} else {
			result.InstallationSummary.RuntimesFailed++
		}

		result.InstallationSummary.RuntimesAttempted++
	}

	result.InstallationSummary.TotalInstallTime = time.Since(installStart)
	if result.InstallationSummary.RuntimesAttempted > 0 {
		result.InstallationSummary.AverageInstallTime = result.InstallationSummary.TotalInstallTime /
			time.Duration(result.InstallationSummary.RuntimesAttempted)
	}

	o.updateProgress(result, "installation", "runtime-installation", progress, "Runtime installation complete")

	return nil
}

// performServerInstallation executes server installation phase
func (o *SetupOrchestrator) performServerInstallation(ctx context.Context, options *SetupOptions, result *SetupResult) error {
	serversToInstall := o.determineServersToInstall(result.DetectedRuntimes, options)
	if len(serversToInstall) == 0 {
		o.logger.UserInfo("No language servers need installation")
		return nil
	}

	progress := NewProgressInfo("Server Installation", int64(len(serversToInstall)))
	o.updateProgress(result, "installation", "server-installation", progress, fmt.Sprintf("Installing %d language servers", len(serversToInstall)))

	completed := int64(0)

	for _, server := range serversToInstall {
		if err := ctx.Err(); err != nil {
			return err
		}

		o.logger.UserProgress(fmt.Sprintf("Installing %s server", server), progress)

		installOptions := o.buildServerInstallOptions(server, options)
		installResult, err := o.serverInstaller.Install(server, installOptions)

		result.ServerInstallResults[server] = installResult
		completed++
		progress.Update(completed, server)

		if err != nil {
			o.addError(result, fmt.Sprintf("%s_server", server), fmt.Sprintf("Failed to install %s server", server), err, "medium", true,
				[]string{fmt.Sprintf("Install %s server manually", server), "Check runtime dependencies"})

			if !options.ContinueOnError {
				return fmt.Errorf("server installation failed for %s: %w", server, err)
			}
			result.InstallationSummary.ServersFailed++
		} else if installResult != nil && installResult.Success {
			result.InstallationSummary.ServersSuccessful++
			o.logger.UserSuccess(fmt.Sprintf("Successfully installed %s server", server))
		} else {
			result.InstallationSummary.ServersFailed++
		}

		result.InstallationSummary.ServersAttempted++
	}

	o.updateProgress(result, "installation", "server-installation", progress, "Server installation complete")

	return nil
}

// performVerification executes verification phase
func (o *SetupOrchestrator) performVerification(ctx context.Context, options *SetupOptions, result *SetupResult) error {
	if !options.ValidateInstallations {
		return nil
	}

	// Verify installed runtimes
	for runtime := range result.RuntimeInstallResults {
		if err := ctx.Err(); err != nil {
			return err
		}

		verifyResult, err := o.runtimeInstaller.Verify(runtime)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Runtime verification failed for %s: %v", runtime, err))
		} else {
			result.RuntimeVerifications[runtime] = verifyResult
			if !verifyResult.Installed || !verifyResult.Compatible {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Runtime %s verification issues found", runtime))
			}
		}
	}

	// Verify installed servers
	for server := range result.ServerInstallResults {
		if err := ctx.Err(); err != nil {
			return err
		}

		verifyResult, err := o.serverInstaller.Verify(server)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Server verification failed for %s: %v", server, err))
		} else {
			result.ServerVerifications[server] = verifyResult
			if !verifyResult.Installed || !verifyResult.Compatible {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Server %s verification issues found", server))
			}
		}
	}

	return nil
}

// performConfigGeneration executes configuration generation phase
func (o *SetupOrchestrator) performConfigGeneration(ctx context.Context, options *SetupOptions, result *SetupResult) error {
	progress := NewProgressInfo("Configuration Generation", 1)
	o.updateProgress(result, "configuration", "config-generation", progress, "Generating gateway configuration")

	o.logger.UserProgress("Generating configuration", progress)

	var configResult *ConfigGenerationResult
	var err error

	if options.UseExistingConfig && options.ConfigOutputPath != "" {
		// Use existing configuration logic would go here
		configResult, err = o.configGenerator.GenerateDefault()
	} else {
		// Generate from detected runtimes
		configResult, err = o.configGenerator.GenerateFromDetected(ctx)
	}

	if err != nil {
		return fmt.Errorf("configuration generation failed: %w", err)
	}

	result.ConfigGeneration = configResult
	result.GeneratedConfig = configResult.Config

	// Apply overrides if specified
	if len(options.ConfigOverrides) > 0 {
		updates := ConfigUpdates{
			Port:     options.ConfigPort,
			Metadata: options.ConfigOverrides,
		}

		updateResult, err := o.configGenerator.UpdateConfig(configResult.Config, updates)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to apply config overrides: %v", err))
		} else {
			result.GeneratedConfig = updateResult.Config
		}
	}

	progress.Update(1, "Configuration generated")
	o.updateProgress(result, "configuration", "config-generation", progress, fmt.Sprintf("Generated config with %d servers", len(result.GeneratedConfig.Servers)))

	o.logger.UserSuccess(fmt.Sprintf("Configuration generated with %d language servers", len(result.GeneratedConfig.Servers)))

	return nil
}

// Helper methods

func (o *SetupOrchestrator) filterRuntimes(runtimes map[string]*RuntimeInfo, options *SetupOptions) map[string]*RuntimeInfo {
	filtered := make(map[string]*RuntimeInfo)

	for name, info := range runtimes {
		// Skip excluded runtimes
		if o.contains(options.ExcludeRuntimes, name) {
			continue
		}

		// If specific runtimes selected, only include those
		if len(options.SelectedRuntimes) > 0 && !o.contains(options.SelectedRuntimes, name) {
			continue
		}

		filtered[name] = info
	}

	return filtered
}

func (o *SetupOrchestrator) determineRuntimesToInstall(runtimes map[string]*RuntimeInfo, options *SetupOptions) []string {
	var toInstall []string

	for name, info := range runtimes {
		if o.contains(options.ExcludeRuntimes, name) {
			continue
		}

		if len(options.SelectedRuntimes) > 0 && !o.contains(options.SelectedRuntimes, name) {
			continue
		}

		if !info.Installed || options.ForceReinstall {
			toInstall = append(toInstall, name)
		}
	}

	return toInstall
}

func (o *SetupOrchestrator) determineServersToInstall(runtimes map[string]*RuntimeInfo, options *SetupOptions) []string {
	var toInstall []string
	serverMap := map[string]string{
		"go":     SERVER_GOPLS,
		"python": SERVER_PYLSP,
		"nodejs": "typescript-language-server",
		"java":   SERVER_JDTLS,
	}

	for runtimeName, info := range runtimes {
		if !info.Installed || !info.Compatible {
			continue
		}

		if server, exists := serverMap[runtimeName]; exists {
			if o.contains(options.ExcludeServers, server) {
				continue
			}

			if len(options.SelectedServers) > 0 && !o.contains(options.SelectedServers, server) {
				continue
			}

			toInstall = append(toInstall, server)
		}
	}

	return toInstall
}

func (o *SetupOrchestrator) buildRuntimeInstallOptions(runtime string, options *SetupOptions) types.InstallOptions {
	installOptions := types.InstallOptions{
		Force:    options.Force || options.ForceReinstall,
		Timeout:  options.InstallTimeout,
		Platform: options.PlatformOverride,
	}

	if version, exists := options.RuntimeVersions[runtime]; exists {
		installOptions.Version = version
	}

	return installOptions
}

func (o *SetupOrchestrator) buildServerInstallOptions(server string, options *SetupOptions) types.ServerInstallOptions {
	installOptions := types.ServerInstallOptions{
		Force:               options.Force || options.ForceReinstall,
		Timeout:             options.InstallTimeout,
		Platform:            options.PlatformOverride,
		SkipDependencyCheck: false,
		SkipVerify:          options.SkipVerification,
	}

	if version, exists := options.ServerVersions[server]; exists {
		installOptions.Version = version
	}

	return installOptions
}

func (o *SetupOrchestrator) updateProgress(result *SetupResult, stage, operation string, progress *ProgressInfo, message string) {
	update := ProgressUpdate{
		Timestamp: time.Now(),
		Stage:     stage,
		Operation: operation,
		Progress:  progress,
		Message:   message,
	}

	result.ProgressUpdates = append(result.ProgressUpdates, update)

	if o.logger != nil && progress != nil {
		o.logger.WithProgress(progress).WithStage(stage).Info(message)
	}
}

func (o *SetupOrchestrator) addError(result *SetupResult, stage, message string, err error, severity string, recoverable bool, suggestions []string) {
	// Create detailed message incorporating error details
	detailedMessage := message
	if err != nil {
		detailedMessage = fmt.Sprintf("%s: %v", message, err)
	}

	// Create SetupError using the existing structure from errors.go
	setupError := NewSetupError(
		SetupErrorTypeInstallation, // Default type, could be made configurable
		stage,                      // Use stage as component
		detailedMessage,            // Detailed message including error
		err,                        // Original error as cause
	)

	// Map suggestions to alternatives
	setupError.Alternatives = suggestions

	// Add severity and recovery info to metadata
	setupError = setupError.WithMetadata("severity", severity)
	setupError = setupError.WithMetadata("recoverable", recoverable)
	setupError = setupError.WithMetadata("timestamp", time.Now())
	setupError = setupError.WithMetadata("original_stage", stage)

	result.Errors = append(result.Errors, *setupError)

	if o.logger != nil {
		o.logger.WithError(err).WithField("stage", stage).Error(message)
	}
}

func (o *SetupOrchestrator) generateRecommendations(result *SetupResult) {
	// Generate recommendations based on results
	if result.InstallationSummary.RuntimesSuccessful > 0 {
		result.NextSteps = append(result.NextSteps, "Start the LSP Gateway server with: ./lsp-gateway server")
	}

	if result.GeneratedConfig != nil {
		result.NextSteps = append(result.NextSteps, "Configuration saved - ready to use")
	}

	if len(result.Errors) > 0 {
		result.Recommendations = append(result.Recommendations, "Review errors and warnings above")
		result.Recommendations = append(result.Recommendations, "Run with --continue-on-error to skip failed components")
	}

	if result.InstallationSummary.ServersSuccessful == 0 {
		result.Recommendations = append(result.Recommendations, "Install language servers manually if auto-installation failed")
	}
}

func (o *SetupOrchestrator) hasMinimumRequirements(result *SetupResult) bool {
	// Check if we have minimum requirements met
	return result.InstallationSummary.RuntimesSuccessful > 0 || result.InstallationSummary.ServersSuccessful > 0
}

func (o *SetupOrchestrator) finalizeResult(result *SetupResult, success bool, err error) (*SetupResult, error) {
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Success = success

	if o.logger != nil {
		result.Metrics = o.logger.GetMetrics()
		o.logger.LogOperationComplete("full-setup", success)

		if success {
			o.logger.UserSuccess(fmt.Sprintf("Setup completed successfully in %v", result.Duration))
		} else {
			o.logger.UserError(fmt.Sprintf("Setup completed with errors in %v", result.Duration))
		}
	}

	return result, err
}

func (o *SetupOrchestrator) contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// SetLogger configures the logger for the orchestrator and its components
func (o *SetupOrchestrator) SetLogger(logger *SetupLogger) {
	if logger != nil {
		o.logger = logger
		if o.runtimeDetector != nil {
			o.runtimeDetector.SetLogger(logger)
		}
		if o.configGenerator != nil {
			o.configGenerator.SetLogger(logger)
		}
	}
}

// GetSupportedRuntimes returns list of supported runtimes
func (o *SetupOrchestrator) GetSupportedRuntimes() []string {
	if o.runtimeInstaller != nil {
		return o.runtimeInstaller.GetSupportedRuntimes()
	}
	return []string{"go", "python", "nodejs", "java"}
}

// GetSupportedServers returns list of supported servers
func (o *SetupOrchestrator) GetSupportedServers() []string {
	if o.serverInstaller != nil {
		return o.serverInstaller.GetSupportedServers()
	}
	return []string{SERVER_GOPLS, SERVER_PYLSP, "typescript-language-server", SERVER_JDTLS}
}
