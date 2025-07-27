package cli

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/gateway"
	"lsp-gateway/internal/project"
	"lsp-gateway/internal/setup"

	"github.com/spf13/cobra"
)

const (
	DefaultServerPort    = 8080
	DefaultLSPGatewayURL = "http://localhost:8080"
)

var (
	configPath string
	port       int
	// Project-related flags
	projectPath           string
	autoDetectProject     bool
	generateProjectConfig bool
	
	// Bypass-related flags
	bypassEnabled              bool
	bypassStrategy             string
	bypassAutoConsecutive      bool
	bypassAutoCircuitBreaker   bool
	bypassConsecutiveThreshold int
	bypassRecoveryAttempts     int
	bypassInteractive          bool
	bypassQuiet                bool
)

var serverCmd = &cobra.Command{
	Use:   CmdServer,
	Short: "Start the LSP Gateway server",
	Long:  `Start the LSP Gateway server with the specified configuration.`,
	RunE:  runServer,
}

func init() {
	serverCmd.Flags().StringVarP(&configPath, "config", "c", DefaultConfigFile, "Configuration file path")
	serverCmd.Flags().IntVarP(&port, FLAG_PORT, "p", DefaultServerPort, FLAG_DESCRIPTION_SERVER_PORT)

	// Project-related flags
	serverCmd.Flags().StringVarP(&projectPath, FLAG_PROJECT, "P", "", FLAG_DESCRIPTION_PROJECT_PATH)
	serverCmd.Flags().BoolVar(&autoDetectProject, FLAG_AUTO_DETECT_PROJECT, false, FLAG_DESCRIPTION_AUTO_DETECT_PROJECT)
	serverCmd.Flags().BoolVar(&generateProjectConfig, FLAG_GENERATE_PROJECT_CONFIG, false, FLAG_DESCRIPTION_GENERATE_PROJECT_CONFIG)

	// Bypass-related flags
	serverCmd.Flags().BoolVar(&bypassEnabled, FLAG_BYPASS_ENABLED, false, FLAG_DESCRIPTION_BYPASS_ENABLED)
	serverCmd.Flags().StringVar(&bypassStrategy, FLAG_BYPASS_STRATEGY, "fail_gracefully", FLAG_DESCRIPTION_BYPASS_STRATEGY)
	serverCmd.Flags().BoolVar(&bypassAutoConsecutive, FLAG_BYPASS_AUTO_CONSECUTIVE, false, FLAG_DESCRIPTION_BYPASS_AUTO_CONSECUTIVE)
	serverCmd.Flags().BoolVar(&bypassAutoCircuitBreaker, FLAG_BYPASS_AUTO_CIRCUIT_BREAKER, false, FLAG_DESCRIPTION_BYPASS_AUTO_CIRCUIT_BREAKER)
	serverCmd.Flags().IntVar(&bypassConsecutiveThreshold, FLAG_BYPASS_CONSECUTIVE_THRESHOLD, 3, FLAG_DESCRIPTION_BYPASS_CONSECUTIVE_THRESHOLD)
	serverCmd.Flags().IntVar(&bypassRecoveryAttempts, FLAG_BYPASS_RECOVERY_ATTEMPTS, 3, FLAG_DESCRIPTION_BYPASS_RECOVERY_ATTEMPTS)
	serverCmd.Flags().BoolVar(&bypassInteractive, FLAG_BYPASS_INTERACTIVE, false, FLAG_DESCRIPTION_BYPASS_INTERACTIVE)
	serverCmd.Flags().BoolVar(&bypassQuiet, FLAG_BYPASS_QUIET, false, FLAG_DESCRIPTION_BYPASS_QUIET)

	rootCmd.AddCommand(serverCmd)
}

func runServer(cmd *cobra.Command, args []string) error {
	return runServerWithContext(cmd.Context(), cmd, args)
}

func runServerWithContext(ctx context.Context, _ *cobra.Command, args []string) error {
	log.Printf("[INFO] Starting LSP Gateway server with config_path=%s\n", configPath)

	if ctx == nil {
		ctx = context.Background()
	}

	cfg, err := setupServerConfiguration()
	if err != nil {
		return err
	}

	gw, err := initializeGateway(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanupGateway(gw)

	server := createHTTPServer(cfg, gw)
	return runServerLifecycle(ctx, server, cfg.Port)
}

func setupServerConfiguration() (*config.GatewayConfig, error) {
	if err := validateServerParams(); err != nil {
		log.Printf("[ERROR] Server parameter validation failed: %v\n", err)
		return nil, err
	}

	// Project detection step
	projectConfig, err := performProjectDetection()
	if err != nil {
		log.Printf("[WARN] Project detection failed: %v\n", err)
		// Continue with default configuration loading
	}

	log.Printf("[DEBUG] Loading configuration\n")
	cfg, err := loadConfigurationWithProject(projectConfig)
	if err != nil {
		log.Printf("[ERROR] Failed to load configuration from %s: %v\n", configPath, err)
		return nil, HandleConfigError(err, configPath)
	}

	log.Printf("[INFO] Configuration loaded from %s\n", configPath)

	if port != DefaultServerPort {
		log.Printf("[INFO] Port override specified: default_port=%d, override_port=%d\n", DefaultServerPort, port)
		cfg.Port = port
	}

	// Apply bypass configuration overrides from CLI flags
	if err := applyBypassOverrides(cfg); err != nil {
		log.Printf("[ERROR] Failed to apply bypass overrides: %v\n", err)
		return nil, err
	}

	log.Printf("[DEBUG] Validating configuration\n")
	if err := config.ValidateConfig(cfg); err != nil {
		log.Printf("[ERROR] Configuration validation failed: %v\n", err)
		return nil, NewValidationError("configuration", []string{err.Error()})
	}
	log.Printf("[INFO] Configuration validated successfully\n")

	log.Printf("[DEBUG] Checking port availability on %d\n", cfg.Port)
	if err := ValidatePortAvailability(cfg.Port, "port"); err != nil {
		log.Printf("[ERROR] Port %d not available: %v\n", cfg.Port, err)
		return nil, ToValidationError(err)
	}

	return cfg, nil
}

func initializeGateway(ctx context.Context, cfg *config.GatewayConfig) (gateway.GatewayInterface, error) {
	log.Printf("[DEBUG] Creating LSP gateway (project_aware=%t)\n", cfg.ProjectAware)

	// Create failure handler for startup failures
	failureHandler := createFailureHandler(cfg)

	var gw gateway.GatewayInterface
	var err error

	if cfg.ProjectAware {
		gw, err = gateway.NewProjectAwareGateway(cfg)
		if err != nil {
			log.Printf("[WARN] Failed to create project-aware gateway, falling back to traditional: %v\n", err)
			// Fallback to traditional gateway
			gw, err = gateway.NewGateway(cfg)
			if err != nil {
				log.Printf("[ERROR] Failed to create fallback gateway: %v\n", err)
				return nil, NewGatewayStartupError(err)
			}
			log.Printf("[INFO] Traditional gateway created as fallback\n")
		} else {
			log.Printf("[INFO] Project-aware gateway created successfully\n")
		}
	} else {
		gw, err = gateway.NewGateway(cfg)
		if err != nil {
			log.Printf("[ERROR] Failed to create gateway: %v\n", err)
			return nil, NewGatewayStartupError(err)
		}
		log.Printf("[INFO] Traditional gateway created successfully\n")
	}

	log.Printf("[INFO] Starting gateway\n")
	if err := gw.Start(ctx); err != nil {
		log.Printf("[ERROR] Failed to start gateway: %v\n", err)
		
		// Try to extract specific server failures from the gateway startup error
		if startupFailures := extractStartupFailures(err, cfg); len(startupFailures) > 0 {
			log.Printf("[INFO] Detected %d server startup failures, handling with failure notification system\n", len(startupFailures))
			
			// Handle startup failures with user interaction
			if handlerErr := failureHandler.HandleGatewayStartupFailures(startupFailures); handlerErr != nil {
				log.Printf("[ERROR] Failure handler returned error: %v\n", handlerErr)
				return nil, handlerErr
			}
			
			// Check if gateway can start with some failures bypassed
			if canStartWithBypassedServers(gw) {
				log.Printf("[INFO] Gateway starting with some servers bypassed\n")
				return gw, nil
			}
		}
		
		return nil, NewGatewayStartupError(err)
	}

	return gw, nil
}

func cleanupGateway(gw gateway.GatewayInterface) {
	if gw != nil {
		if err := gw.Stop(); err != nil {
			log.Printf("[WARN] Error stopping gateway during shutdown: %v\n", err)
		}
	}
}

func createHTTPServer(cfg *config.GatewayConfig, gw gateway.GatewayInterface) *http.Server {
	log.Printf("[DEBUG] Setting up HTTP server\n")

	// Create a new ServeMux to avoid global state conflicts in tests
	mux := http.NewServeMux()
	mux.HandleFunc("/jsonrpc", gw.HandleJSONRPC)

	// Add health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// Check if gateway is properly initialized
		if gw == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintf(w, `{"status":"error","message":"gateway not initialized","timestamp":%d}`, time.Now().Unix())
			return
		}

		// Check health based on gateway type
		hasActiveClient := false
		clientCount := 0
		isProjectAware := false

		// Try to cast to ProjectAwareGateway first
		if projectGw, ok := gw.(*gateway.ProjectAwareGateway); ok {
			isProjectAware = true
			// Check workspace clients
			workspaces := projectGw.GetAllWorkspaces()
			for _, workspace := range workspaces {
				if workspace.IsActive() {
					hasActiveClient = true
					clientCount++
				}
			}
			// Also check traditional clients as fallback
			if !hasActiveClient && projectGw.Gateway != nil {
				for _, client := range projectGw.Clients {
					if client.IsActive() {
						hasActiveClient = true
						clientCount++
					}
				}
			}
		} else if traditionalGw, ok := gw.(*gateway.Gateway); ok {
			// Traditional gateway health check
			if len(traditionalGw.Clients) > 0 {
				for _, client := range traditionalGw.Clients {
					if client.IsActive() {
						hasActiveClient = true
						clientCount++
					}
				}
			}
		}

		if hasActiveClient {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"status":"ok","active_clients":%d,"project_aware":%t,"timestamp":%d}`,
				clientCount, isProjectAware, time.Now().Unix())
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintf(w, `{"status":"starting","message":"no active LSP clients yet","project_aware":%t,"timestamp":%d}`,
				isProjectAware, time.Now().Unix())
		}
	})

	return &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
}

func runServerLifecycle(ctx context.Context, server *http.Server, port int) error {
	serverErr := createServerErrorChannel()
	go func() {
		logServerStart("LSP Gateway HTTP server", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logServerError("HTTP server", err)
			serverErr <- HandleServerStartError(err, port)
		}
	}()

	config := &ServerLifecycleConfig{
		ServerName:      "LSP Gateway server",
		Port:            port,
		ShutdownFunc:    func() error { return shutdownHTTPServer(server) },
		ShutdownTimeout: 30 * time.Second,
	}

	return waitForShutdownSignal(ctx, config, serverErr)
}

func shutdownHTTPServer(server *http.Server) error {
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}
	return nil
}

func validateServerParams() error {
	err := ValidateMultiple(
		func() *ValidationError {
			// Don't validate config file existence here - it will be handled later
			// with auto-generation if needed
			return nil
		},
		func() *ValidationError {
			return ValidatePort(port, "port")
		},
		// Project validation
		func() *ValidationError {
			if projectPath != "" {
				return ValidateProjectPath(projectPath, "project")
			}
			return nil
		},
		// Bypass validation
		func() *ValidationError {
			return validateBypassFlags()
		},
	)
	if err == nil {
		return nil
	}
	return err
}

// validateBypassFlags validates bypass-related CLI flags
func validateBypassFlags() *ValidationError {
	// Validate bypass strategy
	validStrategies := map[string]bool{
		"fail_gracefully":   true,
		"fallback_server":   true,
		"cache_response":    true,
		"circuit_breaker":   true,
		"retry_with_backoff": true,
	}
	
	if bypassStrategy != "" && !validStrategies[bypassStrategy] {
		return &ValidationError{
			Field:   "bypass-strategy",
			Value:   bypassStrategy,
			Message: fmt.Sprintf("invalid bypass strategy '%s'", bypassStrategy),
			Suggestions: []string{
				"Valid strategies: fail_gracefully, fallback_server, cache_response, circuit_breaker, retry_with_backoff",
			},
		}
	}
	
	// Validate consecutive threshold
	if bypassConsecutiveThreshold < 1 {
		return &ValidationError{
			Field:   "bypass-consecutive-threshold",
			Value:   bypassConsecutiveThreshold,
			Message: "consecutive failure threshold must be at least 1",
		}
	}
	
	// Validate recovery attempts
	if bypassRecoveryAttempts < 0 {
		return &ValidationError{
			Field:   "bypass-recovery-attempts",
			Value:   bypassRecoveryAttempts,
			Message: "recovery attempts cannot be negative",
		}
	}
	
	return nil
}

// applyBypassOverrides applies CLI flag values to bypass configuration
func applyBypassOverrides(cfg *config.GatewayConfig) error {
	// Only apply overrides if any bypass flags were explicitly set
	if !hasAnyBypassFlagsSet() {
		return nil
	}
	
	log.Printf("[INFO] Applying bypass configuration overrides from CLI flags\n")
	
	// Initialize bypass configuration if not present
	if cfg.BypassConfig == nil {
		cfg.BypassConfig = config.DefaultBypassConfiguration()
	}
	
	// Initialize global bypass config if not present
	if cfg.BypassConfig.GlobalBypass == nil {
		cfg.BypassConfig.GlobalBypass = &config.GlobalBypassConfig{}
	}
	
	// Apply flag overrides
	if bypassEnabled {
		cfg.BypassConfig.Enabled = true
		log.Printf("[INFO] Bypass functionality enabled via CLI flag\n")
	}
	
	if bypassAutoConsecutive {
		cfg.BypassConfig.GlobalBypass.AutoBypassOnConsecutive = true
		log.Printf("[INFO] Auto bypass on consecutive failures enabled via CLI flag\n")
	}
	
	if bypassAutoCircuitBreaker {
		cfg.BypassConfig.GlobalBypass.AutoBypassOnCircuitBreaker = true
		log.Printf("[INFO] Auto bypass on circuit breaker enabled via CLI flag\n")
	}
	
	if bypassConsecutiveThreshold > 0 {
		cfg.BypassConfig.GlobalBypass.ConsecutiveFailureThreshold = bypassConsecutiveThreshold
		log.Printf("[INFO] Consecutive failure threshold set to %d via CLI flag\n", bypassConsecutiveThreshold)
	}
	
	if bypassRecoveryAttempts >= 0 {
		cfg.BypassConfig.GlobalBypass.MaxRecoveryAttempts = bypassRecoveryAttempts
		log.Printf("[INFO] Max recovery attempts set to %d via CLI flag\n", bypassRecoveryAttempts)
	}
	
	if bypassInteractive {
		cfg.BypassConfig.GlobalBypass.UserConfirmationRequired = true
		log.Printf("[INFO] Interactive bypass mode enabled via CLI flag\n")
	}
	
	// Handle bypass strategy - apply to global default if specified
	if bypassStrategy != "fail_gracefully" {
		// This would typically require a default strategy field in global config
		// For now, we'll log it as this may need additional configuration structure
		log.Printf("[INFO] Default bypass strategy set to '%s' via CLI flag\n", bypassStrategy)
	}
	
	return nil
}

// hasAnyBypassFlagsSet checks if any bypass-related flags were explicitly set
func hasAnyBypassFlagsSet() bool {
	// Note: This is a simple check. For more sophisticated detection,
	// we could track which flags were actually set vs using defaults
	return bypassEnabled || 
		   bypassAutoConsecutive || 
		   bypassAutoCircuitBreaker || 
		   bypassConsecutiveThreshold != 3 || // default value
		   bypassRecoveryAttempts != 3 ||     // default value
		   bypassInteractive || 
		   bypassQuiet ||
		   bypassStrategy != "fail_gracefully" // default value
}

// performProjectDetection performs project detection based on CLI flags
func performProjectDetection() (*project.ProjectAnalysisResult, error) {
	// Skip project detection if no flags are set
	if !autoDetectProject && projectPath == "" {
		return nil, nil
	}

	// Determine project path
	detectionPath := projectPath
	if autoDetectProject && detectionPath == "" {
		// Use current working directory
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
		detectionPath = wd
	}

	if detectionPath == "" {
		return nil, fmt.Errorf("no project path specified for detection")
	}

	log.Printf("[INFO] Starting project detection at path: %s\n", detectionPath)

	// Create project integration
	integration, err := project.NewProjectIntegration(project.DefaultIntegrationConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create project integration: %w", err)
	}

	// Perform detection and analysis
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := integration.DetectAndAnalyzeProject(ctx, detectionPath)
	if err != nil {
		return nil, fmt.Errorf("project detection failed: %w", err)
	}

	if result.ProjectContext != nil {
		log.Printf("[INFO] Project detected: %s (%s) with languages: %v\n",
			result.ProjectContext.ProjectType,
			result.ProjectContext.RootPath,
			result.ProjectContext.Languages)
	}

	return result, nil
}

// loadConfigurationWithProject loads configuration with optional project-specific overrides
func loadConfigurationWithProject(projectResult *project.ProjectAnalysisResult) (*config.GatewayConfig, error) {
	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Printf("[WARN] Configuration file not found: %s\n", configPath)
		
		// Try to auto-generate configuration
		log.Printf("[INFO] Attempting to auto-generate configuration...\n")
		
		// Determine project path for auto-generation
		projectPath := "."
		if projectResult != nil && projectResult.ProjectContext != nil && projectResult.ProjectContext.RootPath != "" {
			projectPath = projectResult.ProjectContext.RootPath
		}
		
		// Auto-generate multi-language configuration
		multiLangConfig, err := config.AutoGenerateConfigFromPath(projectPath)
		if err != nil {
			log.Printf("[ERROR] Failed to auto-generate configuration: %v\n", err)
			return nil, fmt.Errorf("configuration file not found and auto-generation failed: %w", err)
		}
		
		// Convert to gateway configuration
		cfg, err := multiLangConfig.ToGatewayConfig()
		if err != nil {
			log.Printf("[ERROR] Failed to convert auto-generated configuration: %v\n", err)
			return nil, fmt.Errorf("failed to convert auto-generated configuration: %w", err)
		}
		
		log.Printf("[INFO] Successfully auto-generated configuration with %d servers\n", len(cfg.Servers))
		
		// Apply project-specific configuration if available and requested
		if projectResult != nil && projectResult.ProjectContext != nil && generateProjectConfig {
			log.Printf("[INFO] Applying project-specific configuration enhancements\n")
			if err := applyProjectConfiguration(cfg, projectResult); err != nil {
				log.Printf("[WARN] Failed to apply project configuration: %v\n", err)
				// Continue with auto-generated configuration
			}
		}
		
		return cfg, nil
	}
	
	// Load base configuration from existing file
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, err
	}

	// Apply project-specific configuration if available and requested
	if projectResult != nil && projectResult.ProjectContext != nil && generateProjectConfig {
		log.Printf("[INFO] Generating project-specific configuration\n")

		if err := applyProjectConfiguration(cfg, projectResult); err != nil {
			log.Printf("[WARN] Failed to apply project configuration: %v\n", err)
			// Continue with base configuration
		}
	}

	return cfg, nil
}

// applyProjectConfiguration applies project-specific configuration to the gateway config
func applyProjectConfiguration(cfg *config.GatewayConfig, projectResult *project.ProjectAnalysisResult) error {
	projectCtx := projectResult.ProjectContext

	// Create project-aware logger
	logger := setup.NewSetupLogger(&setup.SetupLoggerConfig{
		Component: "project-config-generator",
	})

	// Create config generator with proper registry and verifier
	registry := setup.NewDefaultServerRegistry()
	verifier := setup.NewDefaultServerVerifier()
	generator := project.NewProjectConfigGenerator(logger, registry, verifier)

	// Generate project-specific configuration
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	genResult, err := generator.GenerateFromProject(ctx, projectCtx)
	if err != nil {
		return fmt.Errorf("failed to generate project configuration: %w", err)
	}

	// Apply the generated configuration to the existing config
	if genResult.GatewayConfig != nil {
		// Merge project-aware settings
		if genResult.GatewayConfig.ProjectAware {
			cfg.ProjectAware = true
			cfg.ProjectContext = genResult.GatewayConfig.ProjectContext
		}

		// Add project-specific servers to the existing servers
		cfg.Servers = append(cfg.Servers, genResult.GatewayConfig.Servers...)

		// Apply project configuration if available
		if genResult.GatewayConfig.ProjectConfig != nil {
			cfg.ProjectConfig = genResult.GatewayConfig.ProjectConfig
		}

		log.Printf("[INFO] Applied project configuration: %d additional servers configured\n",
			len(genResult.GatewayConfig.Servers))
	}

	return nil
}

// createFailureHandler creates a failure handler based on CLI configuration
func createFailureHandler(cfg *config.GatewayConfig) *CLIFailureHandler {
	// Create failure handler configuration from CLI flags and gateway config
	failureConfig := &FailureHandlerConfig{
		Interactive:        !bypassQuiet && (bypassInteractive || isTerminalInteractive()),
		MaxRetryAttempts:   3,
		RetryDelay:         2 * time.Second,
		BatchThreshold:     3,
		LogFailures:        true,
		LogLevel:           "warn",
		AutoBypassPolicies: make(map[gateway.FailureCategory]bool),
	}

	// Configure auto-bypass policies based on CLI flags and config
	if bypassAutoConsecutive || (cfg.BypassConfig != nil && cfg.BypassConfig.GlobalBypass != nil && cfg.BypassConfig.GlobalBypass.AutoBypassOnConsecutive) {
		failureConfig.AutoBypassPolicies[gateway.FailureCategoryRuntime] = true
	}
	
	if bypassAutoCircuitBreaker || (cfg.BypassConfig != nil && cfg.BypassConfig.GlobalBypass != nil && cfg.BypassConfig.GlobalBypass.AutoBypassOnCircuitBreaker) {
		failureConfig.AutoBypassPolicies[gateway.FailureCategoryTransport] = true
	}

	// Set severity filter
	if bypassQuiet {
		failureConfig.SeverityFilter = gateway.FailureSeverityHigh
	} else {
		failureConfig.SeverityFilter = gateway.FailureSeverityLow
	}

	// Set retry attempts from config
	if cfg.BypassConfig != nil && cfg.BypassConfig.GlobalBypass != nil && cfg.BypassConfig.GlobalBypass.MaxRecoveryAttempts > 0 {
		failureConfig.MaxRetryAttempts = cfg.BypassConfig.GlobalBypass.MaxRecoveryAttempts
	}

	// Create logger for failure handler
	logger := log.New(os.Stderr, "[FAILURE] ", log.LstdFlags)

	// Configure for non-interactive mode if needed
	handler := NewCLIFailureHandler(failureConfig, logger)
	if !failureConfig.Interactive {
		handler.SetNonInteractiveMode(bypassAutoConsecutive || bypassAutoCircuitBreaker)
	}

	// Configure JSON mode if environment variable is set
	if os.Getenv("LSP_GATEWAY_JSON_OUTPUT") == "true" {
		handler.SetJSONMode()
	}

	return handler
}

// extractStartupFailures attempts to extract individual server failures from gateway startup errors
func extractStartupFailures(err error, cfg *config.GatewayConfig) []gateway.FailureNotification {
	var failures []gateway.FailureNotification

	if err == nil {
		return failures
	}

	errStr := err.Error()
	
	// Try to extract server-specific failures from error message
	// This is a heuristic approach - in practice, the gateway should provide structured failure information
	
	// Look for common failure patterns in the error message
	for _, server := range cfg.Servers {
		serverName := server.Name
		language := ""
		if len(server.Languages) > 0 {
			language = server.Languages[0]
		}
		
		// Check if this server is mentioned in the error
		if strings.Contains(strings.ToLower(errStr), strings.ToLower(serverName)) {
			category := gateway.FailureCategoryStartup
			severity := gateway.FailureSeverityHigh
			
			// Determine specific failure type from error content
			if strings.Contains(strings.ToLower(errStr), "connection") || 
			   strings.Contains(strings.ToLower(errStr), "transport") {
				category = gateway.FailureCategoryTransport
			} else if strings.Contains(strings.ToLower(errStr), "config") ||
			          strings.Contains(strings.ToLower(errStr), "invalid") {
				category = gateway.FailureCategoryConfiguration
			} else if strings.Contains(strings.ToLower(errStr), "memory") ||
			          strings.Contains(strings.ToLower(errStr), "resource") {
				category = gateway.FailureCategoryResource
				severity = gateway.FailureSeverityCritical
			}

			// Generate recommendations based on category
			recommendations := generateStartupRecommendations(category, serverName, language)

			failure := gateway.CreateFailureNotification(
				serverName,
				language,
				category,
				severity,
				fmt.Sprintf("Failed to start during gateway initialization: %s", extractServerError(errStr, serverName)),
				err,
				recommendations,
			)

			// Add startup context
			failure.Context = &gateway.FailureContext{
				ServerName:  serverName,
				Language:    language,
				ProjectPath: getCurrentProjectPath(),
				Metadata: map[string]string{
					"config_file":  getCurrentConfigFile(),
					"startup_time": "0s", // This would be properly calculated
				},
			}

			failure.BypassAvailable = true
			failures = append(failures, failure)
		}
	}

	// If no specific server failures found, create a generic failure
	if len(failures) == 0 && len(cfg.Servers) > 0 {
		// Create a generic failure notification
		failure := gateway.CreateFailureNotification(
			"gateway",
			"multi",
			gateway.FailureCategoryStartup,
			gateway.FailureSeverityHigh,
			fmt.Sprintf("Gateway startup failed: %s", err.Error()),
			err,
			generateStartupRecommendations(gateway.FailureCategoryStartup, "gateway", "multi"),
		)
		failure.BypassAvailable = false // Cannot bypass entire gateway
		failures = append(failures, failure)
	}

	return failures
}

// generateStartupRecommendations generates recommendations for startup failures
func generateStartupRecommendations(category gateway.FailureCategory, serverName, language string) []gateway.RecoveryRecommendation {
	var recommendations []gateway.RecoveryRecommendation

	switch category {
	case gateway.FailureCategoryStartup:
		recommendations = []gateway.RecoveryRecommendation{
			{
				Action:      "check_installation",
				Description: fmt.Sprintf("Verify %s is properly installed for %s", serverName, language),
				Commands:    []string{"status servers", fmt.Sprintf("install server %s", serverName)},
				Priority:    1,
			},
			{
				Action:      "check_runtime",
				Description: fmt.Sprintf("Verify %s runtime is available", language),
				Commands:    []string{"status runtimes", fmt.Sprintf("install runtime %s", language)},
				Priority:    2,
			},
			{
				Action:      "diagnose",
				Description: "Run full system diagnostics",
				Commands:    []string{"diagnose"},
				Priority:    3,
			},
		}
		
	case gateway.FailureCategoryConfiguration:
		recommendations = []gateway.RecoveryRecommendation{
			{
				Action:      "validate_config",
				Description: "Validate configuration file",
				Commands:    []string{"config validate"},
				Priority:    1,
			},
			{
				Action:      "regenerate_config",
				Description: "Generate new configuration",
				Commands:    []string{"config generate --auto-detect"},
				Priority:    2,
			},
			{
				Action:      "setup_all",
				Description: "Run complete setup",
				Commands:    []string{"setup all"},
				Priority:    3,
			},
		}
		
	case gateway.FailureCategoryTransport:
		recommendations = []gateway.RecoveryRecommendation{
			{
				Action:      "check_connectivity",
				Description: "Check network connectivity and ports",
				Commands:    []string{"diagnose"},
				Priority:    1,
			},
			{
				Action:      "change_port",
				Description: "Try different port",
				Commands:    []string{"server --port 8081"},
				Priority:    2,
			},
		}
		
	case gateway.FailureCategoryResource:
		recommendations = []gateway.RecoveryRecommendation{
			{
				Action:      "check_resources",
				Description: "Check system resources (memory, CPU)",
				Commands:    []string{"diagnose"},
				Priority:    1,
			},
			{
				Action:      "free_memory",
				Description: "Close unnecessary applications",
				Commands:    []string{},
				Priority:    2,
			},
		}
	}

	return recommendations
}

// extractServerError extracts server-specific error from error message
func extractServerError(errStr, serverName string) string {
	// Simple heuristic to extract relevant error portion
	lines := strings.Split(errStr, "\n")
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), strings.ToLower(serverName)) {
			return strings.TrimSpace(line)
		}
	}
	return errStr
}

// canStartWithBypassedServers checks if gateway can operate with some servers bypassed
func canStartWithBypassedServers(gw gateway.GatewayInterface) bool {
	if gw == nil {
		return false
	}

	// Check if gateway has any active clients
	// This is a simplified check - in practice, this would query the gateway's internal state
	
	// For project-aware gateways
	if projectGw, ok := gw.(*gateway.ProjectAwareGateway); ok {
		workspaces := projectGw.GetAllWorkspaces()
		for _, workspace := range workspaces {
			if workspace.IsActive() {
				return true
			}
		}
		// Check traditional clients as fallback
		if projectGw.Gateway != nil {
			for _, client := range projectGw.Gateway.Clients {
				if client.IsActive() {
					return true
				}
			}
		}
	}

	// For traditional gateways
	if traditionalGw, ok := gw.(*gateway.Gateway); ok {
		for _, client := range traditionalGw.Clients {
			if client.IsActive() {
				return true
			}
		}
	}

	return false
}

// isTerminalInteractive checks if we're running in an interactive terminal
func isTerminalInteractive() bool {
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
