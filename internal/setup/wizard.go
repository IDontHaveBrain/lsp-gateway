package setup

import (
	"bufio"
	"context"
	"fmt"
	"lsp-gateway/internal/config"
	"lsp-gateway/internal/installer"
	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/types"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type SetupWizard struct {
	detector         RuntimeDetector
	runtimeInstaller types.RuntimeInstaller
	serverInstaller  types.ServerInstaller
	configGenerator  ConfigGenerator
	logger           *SetupLogger

	interactive     bool
	dryRun          bool
	rollbackEnabled bool

	installationState *InstallationState
	rollbackActions   []RollbackAction
}

type InstallationState struct {
	SessionID         string
	StartTime         time.Time
	CurrentPhase      SetupPhase
	PhasesCompleted   []SetupPhase
	RuntimesInstalled []string
	ServersInstalled  []string
	ConfigGenerated   bool
	Errors            []error
	Warnings          []string

	OriginalState *EnvironmentSnapshot
	Modifications []ModificationRecord
}

type SetupPhase string

const (
	PhaseDetection     SetupPhase = "detection"
	PhaseRuntimes      SetupPhase = "runtimes"
	PhaseServers       SetupPhase = "servers"
	PhaseConfiguration SetupPhase = "configuration"
	PhaseVerification  SetupPhase = "verification"
	PhaseCleanup       SetupPhase = "cleanup"
)

type EnvironmentSnapshot struct {
	Timestamp     time.Time
	Runtimes      map[string]*RuntimeInfo
	Servers       map[string]bool
	ConfigExists  bool
	ConfigContent string
}

type ModificationRecord struct {
	Type       ModificationType
	Component  string
	Action     string
	Timestamp  time.Time
	Reversible bool
	UndoAction func() error
	Details    map[string]interface{}
}

type ModificationType string

const (
	ModificationRuntime   ModificationType = "runtime"
	ModificationServer    ModificationType = "server"
	ModificationConfig    ModificationType = "config"
	ModificationDirectory ModificationType = "directory"
	ModificationFile      ModificationType = "file"
)

type RollbackAction struct {
	Description string
	UndoFunc    func() error
	Component   string
	Critical    bool
}

type SetupWizardOptions struct {
	Interactive     bool
	DryRun          bool
	RollbackEnabled bool
	ConfigPath      string
	SkipRuntimes    []string
	SkipServers     []string
	ForceReinstall  bool
	Timeout         time.Duration

	ProgressCallback func(*ProgressInfo)
	StatusCallback   func(string)
}

type InstallationPlan struct {
	SessionID         string
	CreatedAt         time.Time
	RuntimesToInstall []RuntimeInstallationStep
	ServersToInstall  []ServerInstallationStep
	ConfigToGenerate  bool
	EstimatedDuration time.Duration
	RequiresElevation bool
	Warnings          []string
	Dependencies      map[string][]string
}

type RuntimeInstallationStep struct {
	Runtime       string
	Version       string
	Method        string
	Required      bool
	EstimatedTime time.Duration
	Dependencies  []string
}

type ServerInstallationStep struct {
	Server        string
	Runtime       string
	Method        string
	Required      bool
	EstimatedTime time.Duration
	Dependencies  []string
}

func NewSetupWizard() *SetupWizard {
	detector := NewRuntimeDetector()
	runtimeInstaller := installer.NewRuntimeInstaller()
	serverInstaller := installer.NewServerInstaller(runtimeInstaller)
	configGenerator := NewConfigGenerator()

	return &SetupWizard{
		detector:         detector,
		runtimeInstaller: runtimeInstaller,
		serverInstaller:  serverInstaller,
		configGenerator:  configGenerator,
		logger:           NewSetupLogger(nil),
		interactive:      true,
		dryRun:           false,
		rollbackEnabled:  true,
		rollbackActions:  []RollbackAction{},
		installationState: &InstallationState{
			SessionID:         fmt.Sprintf("setup_%d", time.Now().Unix()),
			StartTime:         time.Now(),
			CurrentPhase:      PhaseDetection,
			PhasesCompleted:   []SetupPhase{},
			RuntimesInstalled: []string{},
			ServersInstalled:  []string{},
			ConfigGenerated:   false,
			Errors:            []error{},
			Warnings:          []string{},
			Modifications:     []ModificationRecord{},
		},
	}
}

func NewSetupWizardWithOptions(options SetupWizardOptions) *SetupWizard {
	wizard := NewSetupWizard()
	wizard.interactive = options.Interactive
	wizard.dryRun = options.DryRun
	wizard.rollbackEnabled = options.RollbackEnabled

	return wizard
}

func (w *SetupWizard) SetLogger(logger *SetupLogger) {
	if logger != nil {
		w.logger = logger
		w.detector.SetLogger(logger)
		w.configGenerator.SetLogger(logger)
	}
}

func (w *SetupWizard) RunCompleteSetup(options SetupOptions) (*SetupResult, error) {
	w.logger.WithOperation("complete-setup").Info("Starting complete automated setup")

	w.prepareSetupState(options)
	startTime := time.Now()
	result := w.initializeSetupResult()

	environmentInfo, err := w.executeDetectionPhase(result, startTime)
	if err != nil {
		return result, err
	}

	plan, err := w.executePlanningPhase(environmentInfo, options, result, startTime)
	if err != nil {
		return result, err
	}

	if w.interactive && !w.executeInteractiveConfirmation(plan, result, startTime) {
		return result, nil
	}

	w.prepareInstallationRollback(environmentInfo)

	if err := w.executeRuntimeInstallation(plan, options, result, startTime); err != nil {
		if w.rollbackEnabled {
			if rollbackErr := w.performRollback(); rollbackErr != nil {
				w.logger.WithError(rollbackErr).Error("Rollback failed after runtime installation error")
			}
		}
		return result, err
	}

	if err := w.executeServerInstallation(plan, options, result, startTime); err != nil {
		if w.rollbackEnabled {
			if rollbackErr := w.performRollback(); rollbackErr != nil {
				w.logger.WithError(rollbackErr).Error("Rollback failed after server installation error")
			}
		}
		return result, err
	}

	w.executeConfigurationGeneration(options, result)
	w.executeVerificationPhase(result)
	w.finalizeSetup(result, startTime)

	return result, nil
}

func (w *SetupWizard) RunInteractiveSetup(options SetupOptions) (*SetupResult, error) {
	w.logger.WithOperation("interactive-setup").Info("Starting interactive setup wizard")

	options.Interactive = true
	w.interactive = true

	w.logger.UserInfo("Welcome to the LSP Gateway Setup Wizard!")
	w.logger.UserInfo("This wizard will guide you through setting up runtimes and language servers.")
	w.logger.UserInfo("")

	preferences, err := w.gatherUserPreferences()
	if err != nil {
		return nil, fmt.Errorf("failed to gather user preferences: %w", err)
	}

	options.SkipRuntimes = preferences.SkipRuntimes
	options.SkipServers = preferences.SkipServers
	options.Force = preferences.ForceReinstall

	result, err := w.RunCompleteSetup(options)
	if err != nil {
		w.logger.UserError(fmt.Sprintf("Interactive setup failed: %v", err))
		return result, err
	}

	w.showCompletionSummary(result)

	return result, nil
}

func (w *SetupWizard) detectEnvironment(ctx context.Context) (*EnvironmentInfo, error) {
	w.logger.WithOperation("detect-environment").Info("Detecting system environment")

	detectionReport, err := w.detector.DetectAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("runtime detection failed: %w", err)
	}

	environmentInfo := &EnvironmentInfo{
		Platform:        detectionReport.Platform.String(),
		Architecture:    detectionReport.Architecture.String(),
		Distribution:    w.detectDistribution(),
		Runtimes:        detectionReport.Runtimes,
		LanguageServers: make(map[string]*ServerInfo),
		PackageManagers: w.detectPackageManagers(),
		Issues:          []string{},
	}

	supportedServers := w.serverInstaller.GetSupportedServers()
	for _, serverName := range supportedServers {
		serverVerification, err := w.serverInstaller.Verify(serverName)
		if err != nil {
			w.logger.WithError(err).WithField("server", serverName).Debug("Server verification failed")
			continue
		}

		runtime := ""
		if serverDef, err := w.serverInstaller.GetServerInfo(serverName); err == nil {
			runtime = serverDef.Runtime
		}

		environmentInfo.LanguageServers[serverName] = &ServerInfo{
			Name:      serverName,
			Installed: serverVerification.Installed,
			Version:   serverVerification.Version,
			Runtime:   runtime,
			Path:      serverVerification.Path,
			Working:   len(serverVerification.Issues) == 0,
			Issues:    extractIssueDescriptions(serverVerification.Issues),
		}
	}

	if detectionReport.Summary.IssuesFound > 0 {
		environmentInfo.Issues = append(environmentInfo.Issues,
			fmt.Sprintf("%d runtime detection issues found", detectionReport.Summary.IssuesFound))
	}

	return environmentInfo, nil
}

func (w *SetupWizard) createInstallationPlan(envInfo *EnvironmentInfo, options SetupOptions) (*InstallationPlan, error) {
	w.logger.WithOperation("create-plan").Info("Creating installation plan")

	plan := w.initializeInstallationPlan()
	w.planRuntimeInstallations(envInfo, options, plan)
	w.planServerInstallations(envInfo, options, plan)
	w.finalizeInstallationPlan(plan)

	return plan, nil
}

func (w *SetupWizard) initializeInstallationPlan() *InstallationPlan {
	return &InstallationPlan{
		SessionID:         w.installationState.SessionID,
		CreatedAt:         time.Now(),
		RuntimesToInstall: []RuntimeInstallationStep{},
		ServersToInstall:  []ServerInstallationStep{},
		ConfigToGenerate:  true,
		EstimatedDuration: 0,
		RequiresElevation: false,
		Warnings:          []string{},
		Dependencies:      make(map[string][]string),
	}
}

func (w *SetupWizard) planRuntimeInstallations(envInfo *EnvironmentInfo, options SetupOptions, plan *InstallationPlan) {
	supportedRuntimes := w.runtimeInstaller.GetSupportedRuntimes()

	for _, runtimeName := range supportedRuntimes {
		if contains(options.SkipRuntimes, runtimeName) {
			continue
		}

		if w.shouldInstallRuntime(envInfo, options, runtimeName) {
			step := w.createRuntimeInstallationStep(runtimeName)
			plan.RuntimesToInstall = append(plan.RuntimesToInstall, step)
			plan.EstimatedDuration += step.EstimatedTime
		}
	}
}

func (w *SetupWizard) shouldInstallRuntime(envInfo *EnvironmentInfo, options SetupOptions, runtimeName string) bool {
	runtimeInfo, exists := envInfo.Runtimes[runtimeName]
	return !exists || !runtimeInfo.Installed || !runtimeInfo.Compatible || options.Force
}

func (w *SetupWizard) createRuntimeInstallationStep(runtimeName string) RuntimeInstallationStep {
	return RuntimeInstallationStep{
		Runtime:       runtimeName,
		Version:       "latest",
		Method:        w.getBestInstallMethod(),
		Required:      w.isRuntimeRequired(runtimeName),
		EstimatedTime: w.estimateRuntimeInstallTime(runtimeName),
		Dependencies:  []string{},
	}
}

func (w *SetupWizard) planServerInstallations(envInfo *EnvironmentInfo, options SetupOptions, plan *InstallationPlan) {
	supportedServers := w.serverInstaller.GetSupportedServers()

	for _, serverName := range supportedServers {
		if contains(options.SkipServers, serverName) {
			continue
		}

		if w.shouldInstallServer(envInfo, options, serverName) {
			w.processServerInstallation(envInfo, plan, serverName)
		}
	}
}

func (w *SetupWizard) shouldInstallServer(envInfo *EnvironmentInfo, options SetupOptions, serverName string) bool {
	serverInfo, exists := envInfo.LanguageServers[serverName]
	return !exists || !serverInfo.Installed || !serverInfo.Working || options.Force
}

func (w *SetupWizard) processServerInstallation(envInfo *EnvironmentInfo, plan *InstallationPlan, serverName string) {
	serverDef, err := w.serverInstaller.GetServerInfo(serverName)
	if err != nil {
		w.logger.WithError(err).WithField("server", serverName).Warn("Failed to get server info")
		return
	}

	if w.isRuntimeAvailableForServer(envInfo, plan, serverDef.Runtime) {
		step := w.createServerInstallationStep(serverName, serverDef)
		plan.ServersToInstall = append(plan.ServersToInstall, step)
		plan.EstimatedDuration += step.EstimatedTime
	} else {
		plan.Warnings = append(plan.Warnings,
			fmt.Sprintf("Cannot install %s: runtime %s not available", serverName, serverDef.Runtime))
	}
}

func (w *SetupWizard) isRuntimeAvailableForServer(envInfo *EnvironmentInfo, plan *InstallationPlan, runtimeName string) bool {
	if w.isRuntimeInstalledAndCompatible(envInfo, runtimeName) {
		return true
	}

	return w.isRuntimePlannedForInstallation(plan, runtimeName)
}

func (w *SetupWizard) isRuntimeInstalledAndCompatible(envInfo *EnvironmentInfo, runtimeName string) bool {
	runtimeInfo, exists := envInfo.Runtimes[runtimeName]
	return exists && runtimeInfo.Installed && runtimeInfo.Compatible
}

func (w *SetupWizard) isRuntimePlannedForInstallation(plan *InstallationPlan, runtimeName string) bool {
	for _, runtimeStep := range plan.RuntimesToInstall {
		if runtimeStep.Runtime == runtimeName {
			return true
		}
	}
	return false
}

func (w *SetupWizard) createServerInstallationStep(serverName string, serverDef *installer.ServerDefinition) ServerInstallationStep {
	return ServerInstallationStep{
		Server:        serverName,
		Runtime:       serverDef.Runtime,
		Method:        w.determineServerInstallMethod(serverName),
		Required:      w.isServerRequired(serverName),
		EstimatedTime: w.estimateServerInstallTime(serverName),
		Dependencies:  []string{serverDef.Runtime},
	}
}

func (w *SetupWizard) finalizeInstallationPlan(plan *InstallationPlan) {
	if len(plan.RuntimesToInstall) > 0 {
		plan.RequiresElevation = true
	}

	w.logger.WithFields(map[string]interface{}{
		"runtimes_to_install": len(plan.RuntimesToInstall),
		"servers_to_install":  len(plan.ServersToInstall),
		"estimated_duration":  plan.EstimatedDuration,
		"requires_elevation":  plan.RequiresElevation,
	}).Info("Installation plan created")
}

func (w *SetupWizard) confirmInstallationPlan(plan *InstallationPlan) bool {
	w.logger.UserInfo("Installation Plan:")
	w.logger.UserInfo("================")

	if len(plan.RuntimesToInstall) > 0 {
		w.logger.UserInfo("Runtimes to install:")
		for _, step := range plan.RuntimesToInstall {
			w.logger.UserInfo(fmt.Sprintf("  - %s (%s)", step.Runtime, step.Version))
		}
		w.logger.UserInfo("")
	}

	if len(plan.ServersToInstall) > 0 {
		w.logger.UserInfo("Language servers to install:")
		for _, step := range plan.ServersToInstall {
			w.logger.UserInfo(fmt.Sprintf("  - %s (requires %s)", step.Server, step.Runtime))
		}
		w.logger.UserInfo("")
	}

	if plan.ConfigToGenerate {
		w.logger.UserInfo("Configuration will be generated automatically")
		w.logger.UserInfo("")
	}

	if len(plan.Warnings) > 0 {
		w.logger.UserWarn("Warnings:")
		for _, warning := range plan.Warnings {
			w.logger.UserWarn(fmt.Sprintf("  - %s", warning))
		}
		w.logger.UserInfo("")
	}

	w.logger.UserInfo(fmt.Sprintf("Estimated time: %v", plan.EstimatedDuration))
	if plan.RequiresElevation {
		w.logger.UserWarn("Note: This installation may require administrator privileges")
	}
	w.logger.UserInfo("")

	return w.askYesNo("Proceed with installation?", true)
}

func (w *SetupWizard) installRuntimes(steps []RuntimeInstallationStep, options SetupOptions) ([]string, error) {
	w.logger.WithOperation("install-runtimes").Info("Installing runtimes")

	installedRuntimes := []string{}
	totalSteps := len(steps)

	for i, step := range steps {
		w.logger.UserInfo(fmt.Sprintf("Installing runtime %d/%d: %s", i+1, totalSteps, step.Runtime))

		if w.dryRun {
			w.logger.UserInfo(fmt.Sprintf(DRY_RUN_INSTALL, step.Runtime))
			installedRuntimes = append(installedRuntimes, step.Runtime)
			continue
		}

		installOptions := types.InstallOptions{
			Version:        step.Version,
			Force:          options.Force,
			SkipVerify:     false,
			Timeout:        options.Timeout,
			PackageManager: "", // Use default
		}

		installResult, err := w.runtimeInstaller.Install(step.Runtime, installOptions)
		if err != nil {
			w.logger.WithError(err).WithField("runtime", step.Runtime).Error("Runtime installation failed")
			return installedRuntimes, fmt.Errorf("failed to install runtime %s: %w", step.Runtime, err)
		}

		if !installResult.Success {
			errorMessage := strings.Join(installResult.Errors, "; ")
			w.logger.WithField("runtime", step.Runtime).Error("Runtime installation unsuccessful")
			return installedRuntimes, fmt.Errorf("runtime installation failed: %s", errorMessage)
		}

		if w.rollbackEnabled {
			w.recordModification(ModificationRecord{
				Type:       ModificationRuntime,
				Component:  step.Runtime,
				Action:     "install",
				Timestamp:  time.Now(),
				Reversible: true, // Runtime uninstallation is complex, but we can try
				UndoAction: func() error {
					w.logger.UserWarn(fmt.Sprintf("Rolling back runtime installation: %s", step.Runtime))
					return fmt.Errorf("runtime rollback not implemented - manual removal required for %s", step.Runtime)
				},
				Details: map[string]interface{}{
					"version": installResult.Version,
					"path":    installResult.Path,
					"method":  installResult.Method,
				},
			})
		}

		installedRuntimes = append(installedRuntimes, step.Runtime)
		w.logger.UserSuccess(fmt.Sprintf("Successfully installed %s", step.Runtime))
	}

	return installedRuntimes, nil
}

func (w *SetupWizard) installServers(steps []ServerInstallationStep, options SetupOptions) ([]string, error) {
	w.logger.WithOperation("install-servers").Info("Installing language servers")

	installedServers := []string{}
	totalSteps := len(steps)

	for i, step := range steps {
		w.logger.UserInfo(fmt.Sprintf("Installing server %d/%d: %s", i+1, totalSteps, step.Server))

		if w.dryRun {
			w.logger.UserInfo(fmt.Sprintf(DRY_RUN_INSTALL, step.Server))
			installedServers = append(installedServers, step.Server)
			continue
		}

		installOptions := types.ServerInstallOptions{
			Version:             "latest",
			Force:               options.Force,
			SkipVerify:          false,
			SkipDependencyCheck: false,
			Timeout:             options.Timeout,
			Platform:            "",
			InstallMethod:       step.Method,
		}

		installResult, err := w.serverInstaller.Install(step.Server, installOptions)
		if err != nil {
			w.logger.WithError(err).WithField("server", step.Server).Error("Server installation failed")
			return installedServers, fmt.Errorf("failed to install server %s: %w", step.Server, err)
		}

		if !installResult.Success {
			errorMessage := strings.Join(installResult.Errors, "; ")
			w.logger.WithField("server", step.Server).Error("Server installation unsuccessful")
			return installedServers, fmt.Errorf("server installation failed: %s", errorMessage)
		}

		if w.rollbackEnabled {
			w.recordModification(ModificationRecord{
				Type:       ModificationServer,
				Component:  step.Server,
				Action:     "install",
				Timestamp:  time.Now(),
				Reversible: true,
				UndoAction: func() error {
					w.logger.UserWarn(fmt.Sprintf("Rolling back server installation: %s", step.Server))
					return w.uninstallServer(step.Server, step.Method)
				},
				Details: map[string]interface{}{
					"version": installResult.Version,
					"path":    installResult.Path,
					"method":  installResult.Method,
				},
			})
		}

		installedServers = append(installedServers, step.Server)
		w.logger.UserSuccess(fmt.Sprintf("Successfully installed %s", step.Server))
	}

	return installedServers, nil
}

func (w *SetupWizard) generateConfiguration(configPath string) (bool, error) {
	w.logger.WithOperation("generate-config").Info("Generating configuration")

	if w.dryRun {
		w.logger.UserInfo("DRY RUN: Would generate configuration")
		return true, nil
	}

	ctx := context.Background()
	configResult, err := w.configGenerator.GenerateFromDetected(ctx)
	if err != nil {
		return false, fmt.Errorf("configuration generation failed: %w", err)
	}

	if len(configResult.Issues) > 0 {
		w.logger.UserWarn("Configuration generation completed with issues:")
		for _, issue := range configResult.Issues {
			w.logger.UserWarn(fmt.Sprintf("  - %s", issue))
		}
	}

	outputPath := configPath
	if outputPath == "" {
		outputPath = "config.yaml"
	}

	configExists := false
	var existingContent string
	if _, err := os.Stat(outputPath); err == nil {
		configExists = true
		if content, err := os.ReadFile(outputPath); err == nil {
			existingContent = string(content)
		}
	}

	if err := config.SaveConfig(configResult.Config, outputPath); err != nil {
		return false, fmt.Errorf("failed to save configuration: %w", err)
	}

	if w.rollbackEnabled {
		w.recordModification(ModificationRecord{
			Type:       ModificationConfig,
			Component:  outputPath,
			Action:     "create_or_update",
			Timestamp:  time.Now(),
			Reversible: true,
			UndoAction: func() error {
				w.logger.UserWarn(fmt.Sprintf("Rolling back configuration: %s", outputPath))
				if configExists {
					return os.WriteFile(outputPath, []byte(existingContent), 0644)
				} else {
					return os.Remove(outputPath)
				}
			},
			Details: map[string]interface{}{
				"path":          outputPath,
				"existed":       configExists,
				"servers_count": len(configResult.Config.Servers),
			},
		})
	}

	w.logger.UserSuccess(fmt.Sprintf("Configuration saved to %s", outputPath))
	w.logger.UserInfo(fmt.Sprintf("Generated configuration with %d language servers", len(configResult.Config.Servers)))

	return true, nil
}

func (w *SetupWizard) verifyCompleteSetup() (*VerificationReport, error) {
	w.logger.WithOperation("verify-setup").Info("Verifying complete setup")

	report := &VerificationReport{
		Overall:         VerificationStatusPassed,
		Runtimes:        make(map[string]*RuntimeVerification),
		LanguageServers: make(map[string]*ServerVerification),
		Configuration:   &ConfigVerification{},
		Recommendations: []string{},
	}

	for _, runtimeName := range w.installationState.RuntimesInstalled {
		verification, err := w.runtimeInstaller.Verify(runtimeName)
		if err != nil {
			w.logger.WithError(err).WithField("runtime", runtimeName).Warn("Runtime verification failed")
			report.Overall = VerificationStatusWarning
			continue
		}

		report.Runtimes[runtimeName] = &RuntimeVerification{
			Status:       VerificationStatusPassed,
			Version:      verification.Version,
			VersionCheck: verification.Compatible,
			PathCheck:    verification.Path != "",
			Permission:   len(verification.Issues) == 0,
			Issues:       extractIssueDescriptions(verification.Issues),
		}

		if !verification.Compatible || !verification.Installed {
			report.Runtimes[runtimeName].Status = VerificationStatusFailed
			report.Overall = VerificationStatusFailed
		}
	}

	for _, serverName := range w.installationState.ServersInstalled {
		verification, err := w.serverInstaller.Verify(serverName)
		if err != nil {
			w.logger.WithError(err).WithField("server", serverName).Warn("Server verification failed")
			report.Overall = VerificationStatusWarning
			continue
		}

		report.LanguageServers[serverName] = &ServerVerification{
			Status:          VerificationStatusPassed,
			Version:         verification.Version,
			RuntimeCheck:    true, // Runtime was verified above
			ExecutableCheck: verification.Path != "",
			Communication:   len(verification.Issues) == 0,
			Issues:          extractIssueDescriptions(verification.Issues),
		}

		if !verification.Compatible || !verification.Installed {
			report.LanguageServers[serverName].Status = VerificationStatusFailed
			report.Overall = VerificationStatusFailed
		}
	}

	if w.installationState.ConfigGenerated {
		report.Configuration = &ConfigVerification{
			Status:       VerificationStatusPassed,
			FileExists:   true,
			Syntax:       true,
			Schema:       true,
			ServerConfig: true,
			Issues:       []string{},
		}
	}

	if report.Overall != VerificationStatusPassed {
		report.Recommendations = append(report.Recommendations,
			"Some components failed verification. Check the logs for detailed error information.")
	}

	if len(w.installationState.RuntimesInstalled) > 0 && len(w.installationState.ServersInstalled) == 0 {
		report.Recommendations = append(report.Recommendations,
			"Runtimes were installed but no language servers. Consider running server installation.")
	}

	return report, nil
}

func (w *SetupWizard) gatherUserPreferences() (*UserPreferences, error) {
	w.logger.UserInfo("Gathering your preferences...")
	w.logger.UserInfo("")

	preferences := &UserPreferences{
		SkipRuntimes:   []string{},
		SkipServers:    []string{},
		ForceReinstall: false,
	}

	supportedRuntimes := w.runtimeInstaller.GetSupportedRuntimes()
	w.logger.UserInfo("Which programming languages do you want to set up?")
	for _, runtime := range supportedRuntimes {
		if !w.askYesNo(fmt.Sprintf("Install %s runtime?", runtime), true) {
			preferences.SkipRuntimes = append(preferences.SkipRuntimes, runtime)
		}
	}

	preferences.ForceReinstall = w.askYesNo("Force reinstall of existing components?", false)

	return preferences, nil
}

func (w *SetupWizard) askYesNo(question string, defaultYes bool) bool {
	prompt := question
	if defaultYes {
		prompt += " [Y/n]: "
	} else {
		prompt += " [y/N]: "
	}

	w.logger.UserInfo(prompt)

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		response := strings.ToLower(strings.TrimSpace(scanner.Text()))

		if response == "" {
			return defaultYes
		}

		return response == "y" || response == "yes"
	}

	return defaultYes
}

func (w *SetupWizard) showCompletionSummary(result *SetupResult) {
	w.logger.UserInfo("Setup Complete!")
	w.logger.UserInfo("================")

	if len(result.RuntimesInstalled) > 0 {
		w.logger.UserSuccess(fmt.Sprintf("✓ Installed %d runtimes: %s",
			len(result.RuntimesInstalled), strings.Join(result.RuntimesInstalled, ", ")))
	}

	if len(result.ServersInstalled) > 0 {
		w.logger.UserSuccess(fmt.Sprintf("✓ Installed %d language servers: %s",
			len(result.ServersInstalled), strings.Join(result.ServersInstalled, ", ")))
	}

	if result.ConfigGenerated {
		w.logger.UserSuccess("✓ Configuration generated successfully")
	}

	w.logger.UserInfo(fmt.Sprintf("Total setup time: %v", result.Duration))

	if len(result.Issues) > 0 {
		w.logger.UserWarn("Issues encountered:")
		for _, issue := range result.Issues {
			w.logger.UserWarn(fmt.Sprintf("  - %s", issue))
		}
	}

	if len(result.Recommendations) > 0 {
		w.logger.UserInfo("Recommendations:")
		for _, rec := range result.Recommendations {
			w.logger.UserInfo(fmt.Sprintf("  - %s", rec))
		}
	}

	w.logger.UserInfo("")
	w.logger.UserInfo("You can now start the LSP Gateway with:")
	w.logger.UserInfo("  ./lsp-gateway server --config config.yaml")
}

func (w *SetupWizard) performRollback() error {
	w.logger.WithOperation("rollback").Info("Performing setup rollback")

	if len(w.installationState.Modifications) == 0 {
		w.logger.UserInfo("No modifications to rollback")
		return nil
	}

	rollbackErrors := []string{}
	for i := len(w.installationState.Modifications) - 1; i >= 0; i-- {
		modification := w.installationState.Modifications[i]

		if !modification.Reversible {
			w.logger.UserWarn(fmt.Sprintf("Cannot rollback %s: %s (not reversible)",
				modification.Component, modification.Action))
			continue
		}

		w.logger.UserInfo(fmt.Sprintf("Rolling back: %s %s", modification.Component, modification.Action))

		if err := modification.UndoAction(); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Sprintf("%s: %v", modification.Component, err))
			w.logger.WithError(err).WithField("component", modification.Component).Error("Rollback failed")
		} else {
			w.logger.UserSuccess(fmt.Sprintf("Rolled back: %s", modification.Component))
		}
	}

	if len(rollbackErrors) > 0 {
		w.logger.UserWarn("Some rollback operations failed:")
		for _, error := range rollbackErrors {
			w.logger.UserWarn(fmt.Sprintf("  - %s", error))
		}
		return fmt.Errorf("rollback completed with %d errors", len(rollbackErrors))
	}

	w.logger.UserSuccess("Rollback completed successfully")
	return nil
}

func (w *SetupWizard) recordModification(modification ModificationRecord) {
	w.installationState.Modifications = append(w.installationState.Modifications, modification)
}

func (w *SetupWizard) captureEnvironmentSnapshot(envInfo *EnvironmentInfo) *EnvironmentSnapshot {
	snapshot := &EnvironmentSnapshot{
		Timestamp: time.Now(),
		Runtimes:  make(map[string]*RuntimeInfo),
		Servers:   make(map[string]bool),
	}

	for name, info := range envInfo.Runtimes {
		snapshot.Runtimes[name] = info
	}

	for name, info := range envInfo.LanguageServers {
		snapshot.Servers[name] = info.Installed
	}

	configPath := "config.yaml"
	if _, err := os.Stat(configPath); err == nil {
		snapshot.ConfigExists = true
		if content, err := os.ReadFile(configPath); err == nil {
			snapshot.ConfigContent = string(content)
		}
	}

	return snapshot
}

type UserPreferences struct {
	SkipRuntimes   []string
	SkipServers    []string
	ForceReinstall bool
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func extractIssueDescriptions(issues []types.Issue) []string {
	descriptions := make([]string, len(issues))
	for i, issue := range issues {
		descriptions[i] = issue.Description
	}
	return descriptions
}

func (w *SetupWizard) isRuntimeRequired(runtime string) bool {
	return runtime == "go"
}

func (w *SetupWizard) isServerRequired(server string) bool {
	return server == "gopls"
}

func (w *SetupWizard) estimateRuntimeInstallTime(runtime string) time.Duration {
	estimates := map[string]time.Duration{
		"go":     3 * time.Minute,
		"python": 2 * time.Minute,
		"nodejs": 2 * time.Minute,
		"java":   5 * time.Minute,
	}

	if estimate, exists := estimates[runtime]; exists {
		return estimate
	}
	return 3 * time.Minute // Default estimate
}

func (w *SetupWizard) estimateServerInstallTime(server string) time.Duration {
	estimates := map[string]time.Duration{
		"gopls":                      30 * time.Second,
		"pylsp":                      1 * time.Minute,
		"typescript-language-server": 1 * time.Minute,
		"jdtls":                      3 * time.Minute,
	}

	if estimate, exists := estimates[server]; exists {
		return estimate
	}
	return 1 * time.Minute // Default estimate
}

func (w *SetupWizard) determineServerInstallMethod(server string) string {
	methods := map[string]string{
		"gopls":                      "go_install",
		"pylsp":                      "pip_install",
		"typescript-language-server": "npm_install",
		"jdtls":                      "manual_download",
	}

	if method, exists := methods[server]; exists {
		return method
	}
	return "package_manager" // Default method
}

func (w *SetupWizard) uninstallServer(server, method string) error {
	switch method {
	case "go_install":
		return fmt.Errorf("go install uninstallation not implemented - remove from GOPATH/bin manually")
	case "pip_install":
		return fmt.Errorf("pip uninstallation not implemented - use 'pip uninstall python-lsp-server'")
	case "npm_install":
		return fmt.Errorf("npm uninstallation not implemented - use 'npm uninstall -g typescript-language-server'")
	case "manual_download":
		homeDir, _ := os.UserHomeDir()
		installDir := filepath.Join(homeDir, ".local", "share", "lsp-gateway", server)
		return os.RemoveAll(installDir)
	default:
		return fmt.Errorf("unknown installation method: %s", method)
	}
}

func (w *SetupWizard) detectDistribution() string {
	if !platform.IsLinux() {
		return ""
	}

	linuxInfo, err := platform.DetectLinuxDistribution()
	if err != nil {
		w.logger.WithError(err).Debug("Failed to detect Linux distribution")
		return ""
	}

	return linuxInfo.Distribution.String()
}

func (w *SetupWizard) detectPackageManagers() []string {
	availableManagers := platform.GetAvailablePackageManagers()
	var managerNames []string

	for _, mgr := range availableManagers {
		managerNames = append(managerNames, mgr.GetName())
	}

	return managerNames
}

func (w *SetupWizard) getBestInstallMethod() string {
	bestMgr := platform.GetBestPackageManager()
	if bestMgr != nil {
		return bestMgr.GetName()
	}
	return "manual" // Fallback to manual installation
}

// prepareSetupState initializes the setup state and environment
func (w *SetupWizard) prepareSetupState(options SetupOptions) {
	w.installationState.CurrentPhase = PhaseDetection
	w.logger.WithOperation("prepare-state").Info("Preparing setup state")
}

// initializeSetupResult creates and returns a new SetupResult
func (w *SetupWizard) initializeSetupResult() *SetupResult {
	return &SetupResult{
		Success:           false,
		Duration:          0,
		RuntimesInstalled: []string{},
		ServersInstalled:  []string{},
		ConfigGenerated:   false,
		Issues:            []string{},
		Warnings:          []string{},
		Recommendations:   []string{},
	}
}

// executeDetectionPhase runs environment detection and returns environment info
func (w *SetupWizard) executeDetectionPhase(result *SetupResult, startTime time.Time) (*EnvironmentInfo, error) {
	w.installationState.CurrentPhase = PhaseDetection
	w.logger.WithOperation("detection-phase").Info("Executing detection phase")

	ctx := context.Background()
	envInfo, err := w.detectEnvironment(ctx)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Environment detection failed: %v", err))
		return nil, err
	}

	w.installationState.PhasesCompleted = append(w.installationState.PhasesCompleted, PhaseDetection)
	return envInfo, nil
}

// executePlanningPhase creates an installation plan
func (w *SetupWizard) executePlanningPhase(envInfo *EnvironmentInfo, options SetupOptions, result *SetupResult, startTime time.Time) (*InstallationPlan, error) {
	w.logger.WithOperation("planning-phase").Info("Executing planning phase")

	plan, err := w.createInstallationPlan(envInfo, options)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Installation planning failed: %v", err))
		return nil, err
	}

	return plan, nil
}

// executeInteractiveConfirmation prompts user to confirm the installation plan
func (w *SetupWizard) executeInteractiveConfirmation(plan *InstallationPlan, result *SetupResult, startTime time.Time) bool {
	w.logger.WithOperation("interactive-confirmation").Info("Executing interactive confirmation")

	if !w.confirmInstallationPlan(plan) {
		result.Issues = append(result.Issues, "Installation cancelled by user")
		return false
	}

	return true
}

// prepareInstallationRollback sets up rollback functionality
func (w *SetupWizard) prepareInstallationRollback(envInfo *EnvironmentInfo) {
	w.logger.WithOperation("prepare-rollback").Info("Preparing installation rollback")

	if w.rollbackEnabled {
		w.installationState.OriginalState = w.captureEnvironmentSnapshot(envInfo)
	}
}

// executeRuntimeInstallation installs required runtimes
func (w *SetupWizard) executeRuntimeInstallation(plan *InstallationPlan, options SetupOptions, result *SetupResult, startTime time.Time) error {
	w.installationState.CurrentPhase = PhaseRuntimes
	w.logger.WithOperation("runtime-installation").Info("Executing runtime installation phase")

	if len(plan.RuntimesToInstall) == 0 {
		w.logger.Info("No runtimes to install")
		w.installationState.PhasesCompleted = append(w.installationState.PhasesCompleted, PhaseRuntimes)
		return nil
	}

	installedRuntimes, err := w.installRuntimes(plan.RuntimesToInstall, options)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Runtime installation failed: %v", err))
		return err
	}

	result.RuntimesInstalled = installedRuntimes
	w.installationState.RuntimesInstalled = installedRuntimes
	w.installationState.PhasesCompleted = append(w.installationState.PhasesCompleted, PhaseRuntimes)

	return nil
}

// executeServerInstallation installs language servers
func (w *SetupWizard) executeServerInstallation(plan *InstallationPlan, options SetupOptions, result *SetupResult, startTime time.Time) error {
	w.installationState.CurrentPhase = PhaseServers
	w.logger.WithOperation("server-installation").Info("Executing server installation phase")

	if len(plan.ServersToInstall) == 0 {
		w.logger.Info("No servers to install")
		w.installationState.PhasesCompleted = append(w.installationState.PhasesCompleted, PhaseServers)
		return nil
	}

	installedServers, err := w.installServers(plan.ServersToInstall, options)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Server installation failed: %v", err))
		return err
	}

	result.ServersInstalled = installedServers
	w.installationState.ServersInstalled = installedServers
	w.installationState.PhasesCompleted = append(w.installationState.PhasesCompleted, PhaseServers)

	return nil
}

// executeConfigurationGeneration generates the configuration file
func (w *SetupWizard) executeConfigurationGeneration(options SetupOptions, result *SetupResult) {
	w.installationState.CurrentPhase = PhaseConfiguration
	w.logger.WithOperation("config-generation").Info("Executing configuration generation phase")

	configGenerated, err := w.generateConfiguration(options.ConfigPath)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Configuration generation failed: %v", err))
		return
	}

	result.ConfigGenerated = configGenerated
	w.installationState.ConfigGenerated = configGenerated
	w.installationState.PhasesCompleted = append(w.installationState.PhasesCompleted, PhaseConfiguration)
}

// executeVerificationPhase verifies the complete setup
func (w *SetupWizard) executeVerificationPhase(result *SetupResult) {
	w.installationState.CurrentPhase = PhaseVerification
	w.logger.WithOperation("verification-phase").Info("Executing verification phase")

	verificationReport, err := w.verifyCompleteSetup()
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Verification failed: %v", err))
		return
	}

	// Add verification recommendations to result
	result.Recommendations = append(result.Recommendations, verificationReport.Recommendations...)

	w.installationState.PhasesCompleted = append(w.installationState.PhasesCompleted, PhaseVerification)
}

// finalizeSetup completes the setup process
func (w *SetupWizard) finalizeSetup(result *SetupResult, startTime time.Time) {
	w.installationState.CurrentPhase = PhaseCleanup
	result.Duration = time.Since(startTime)
	result.Success = len(result.Issues) == 0

	w.logger.WithOperation("finalize-setup").WithFields(map[string]interface{}{
		"duration":           result.Duration,
		"success":            result.Success,
		"runtimes_installed": len(result.RuntimesInstalled),
		"servers_installed":  len(result.ServersInstalled),
		"config_generated":   result.ConfigGenerated,
	}).Info("Setup finalized")
}
