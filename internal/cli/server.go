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
	"lsp-gateway/internal/project"
	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/workspace"

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

	workspaceCtx, workspaceConfig, gw, err := initializeWorkspaceGateway(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanupWorkspaceGateway(gw)

	log.Printf("[INFO] Workspace initialized: ID=%s, Root=%s, Type=%s, Languages=%v\n",
		workspaceCtx.ID, workspaceCtx.Root, workspaceCtx.ProjectType, workspaceCtx.Languages)

	server := createHTTPServer(cfg, gw, workspaceConfig)
	return runServerLifecycle(ctx, server, cfg.Port)
}

func setupServerConfiguration() (*config.GatewayConfig, error) {
	if err := validateServerParams(); err != nil {
		log.Printf("[ERROR] Server parameter validation failed: %v\n", err)
		return nil, err
	}

	// Workspace detection step (simplified from project detection)
	workspaceRoot, err := os.Getwd()
	if err != nil {
		log.Printf("[ERROR] Failed to get current working directory: %v\n", err)
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}

	log.Printf("[INFO] Using workspace root: %s\n", workspaceRoot)

	log.Printf("[DEBUG] Loading configuration\n")
	cfg, err := loadConfiguration()
	if err != nil {
		log.Printf("[ERROR] Failed to load configuration from %s: %v\n", configPath, err)
		return nil, HandleConfigError(err, configPath)
	}

	log.Printf("[INFO] Configuration loaded from %s\n", configPath)

	// Initialize port manager for dynamic port allocation
	var allocatedPort int
	if port != DefaultServerPort {
		log.Printf("[INFO] Port override specified: default_port=%d, override_port=%d\n", DefaultServerPort, port)
		allocatedPort = port
	} else {
		// Use workspace port manager for dynamic allocation
		portManager, err := workspace.NewWorkspacePortManager()
		if err != nil {
			log.Printf("[WARN] Failed to initialize port manager, using default port: %v\n", err)
			allocatedPort = DefaultServerPort
		} else {
			allocatedPort, err = portManager.AllocatePort(workspaceRoot)
			if err != nil {
				log.Printf("[WARN] Port allocation failed, using default port: %v\n", err)
				allocatedPort = DefaultServerPort
			} else {
				log.Printf("[INFO] Allocated port %d for workspace %s\n", allocatedPort, workspaceRoot)
			}
		}
	}

	cfg.Port = allocatedPort

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

func initializeWorkspaceGateway(ctx context.Context, cfg *config.GatewayConfig) (*workspace.WorkspaceContext, *workspace.WorkspaceConfig, workspace.WorkspaceGateway, error) {
	log.Printf("[DEBUG] Initializing workspace-based gateway\n")

	// Create workspace detector
	workspaceDetector := workspace.NewWorkspaceDetector()

	// Detect workspace context
	workspaceCtx, err := workspaceDetector.DetectWorkspace()
	if err != nil {
		log.Printf("[ERROR] Workspace detection failed: %v\n", err)
		return nil, nil, nil, fmt.Errorf("workspace detection failed: %w", err)
	}

	log.Printf("[INFO] Workspace detected: ID=%s, Type=%s, Languages=%v\n",
		workspaceCtx.ID, workspaceCtx.ProjectType, workspaceCtx.Languages)

	// Create workspace config manager
	configManager := workspace.NewWorkspaceConfigManager()

	// Generate or load workspace configuration
	workspaceConfig, err := loadOrGenerateWorkspaceConfig(configManager, workspaceCtx)
	if err != nil {
		log.Printf("[ERROR] Failed to load/generate workspace configuration: %v\n", err)
		return nil, nil, nil, fmt.Errorf("workspace configuration failed: %w", err)
	}

	// Create workspace gateway
	gw := workspace.NewWorkspaceGateway()

	// Initialize gateway with workspace configuration
	gatewayConfig := &workspace.WorkspaceGatewayConfig{
		WorkspaceRoot:    workspaceCtx.Root,
		ExtensionMapping: createDefaultExtensionMapping(),
		Timeout:          30 * time.Second,
		EnableLogging:    true,
	}

	if err := gw.Initialize(ctx, workspaceConfig, gatewayConfig); err != nil {
		log.Printf("[ERROR] Failed to initialize workspace gateway: %v\n", err)
		return nil, nil, nil, fmt.Errorf("gateway initialization failed: %w", err)
	}

	log.Printf("[INFO] Starting workspace gateway\n")
	if err := gw.Start(ctx); err != nil {
		log.Printf("[ERROR] Failed to start workspace gateway: %v\n", err)
		
		// Check if gateway can start with partial functionality
		health := gw.Health()
		if health.ActiveClients > 0 {
			log.Printf("[INFO] Gateway starting with %d active clients (some servers may have failed)\n", health.ActiveClients)
			return workspaceCtx, workspaceConfig, gw, nil
		}
		
		return nil, nil, nil, fmt.Errorf("gateway startup failed: %w", err)
	}

	log.Printf("[INFO] Workspace gateway started successfully with %d clients\n", gw.Health().ActiveClients)
	return workspaceCtx, workspaceConfig, gw, nil
}

func cleanupWorkspaceGateway(gw workspace.WorkspaceGateway) {
	if gw != nil {
		if err := gw.Stop(); err != nil {
			log.Printf("[WARN] Error stopping workspace gateway during shutdown: %v\n", err)
		}
	}
}

// loadOrGenerateWorkspaceConfig loads existing workspace config or generates new one
func loadOrGenerateWorkspaceConfig(configManager workspace.WorkspaceConfigManager, workspaceCtx *workspace.WorkspaceContext) (*workspace.WorkspaceConfig, error) {
	// Try to load existing workspace configuration
	workspaceConfig, err := configManager.LoadWorkspaceConfig(workspaceCtx.Root)
	if err == nil {
		log.Printf("[INFO] Loaded existing workspace configuration\n")
		return workspaceConfig, nil
	}

	log.Printf("[INFO] Generating new workspace configuration\n")

	// Create project context for config generation
	projectCtx := &project.ProjectContext{
		ProjectType: workspaceCtx.ProjectType,
		RootPath:    workspaceCtx.Root,
		Languages:   workspaceCtx.Languages,
		Metadata:    make(map[string]interface{}),
	}

	// Generate new workspace configuration
	if err := configManager.GenerateWorkspaceConfig(workspaceCtx.Root, projectCtx); err != nil {
		return nil, fmt.Errorf("failed to generate workspace config: %w", err)
	}

	// Load the newly generated configuration
	workspaceConfig, err = configManager.LoadWorkspaceConfig(workspaceCtx.Root)
	if err != nil {
		return nil, fmt.Errorf("failed to load generated workspace config: %w", err)
	}

	log.Printf("[INFO] Generated and loaded workspace configuration with %d servers\n", len(workspaceConfig.Servers))
	return workspaceConfig, nil
}

// createDefaultExtensionMapping creates default file extension to language mappings
func createDefaultExtensionMapping() map[string]string {
	return map[string]string{
		".go":   types.PROJECT_TYPE_GO,
		".mod":  types.PROJECT_TYPE_GO,
		".py":   types.PROJECT_TYPE_PYTHON,
		".pyx":  types.PROJECT_TYPE_PYTHON,
		".pyi":  types.PROJECT_TYPE_PYTHON,
		".js":   types.PROJECT_TYPE_NODEJS,
		".jsx":  types.PROJECT_TYPE_NODEJS,
		".mjs":  types.PROJECT_TYPE_NODEJS,
		".cjs":  types.PROJECT_TYPE_NODEJS,
		".ts":   types.PROJECT_TYPE_TYPESCRIPT,
		".tsx":  types.PROJECT_TYPE_TYPESCRIPT,
		".java": types.PROJECT_TYPE_JAVA,
	}
}

// loadConfiguration loads configuration without project-specific logic
func loadConfiguration() (*config.GatewayConfig, error) {
	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Printf("[WARN] Configuration file not found: %s\n", configPath)
		
		// Try to auto-generate configuration
		log.Printf("[INFO] Attempting to auto-generate configuration...\n")
		
		// Use current working directory for auto-generation
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
		
		// Auto-generate multi-language configuration
		multiLangConfig, err := config.AutoGenerateConfigFromPath(wd)
		if err != nil {
			return nil, fmt.Errorf("configuration file not found and auto-generation failed: %w", err)
		}
		
		// Convert to gateway configuration
		cfg, err := multiLangConfig.ToGatewayConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to convert auto-generated configuration: %w", err)
		}
		
		log.Printf("[INFO] Successfully auto-generated configuration with %d servers\n", len(cfg.Servers))
		return cfg, nil
	}
	
	// Load base configuration from existing file
	return config.LoadConfig(configPath)
}

func createHTTPServer(cfg *config.GatewayConfig, gw workspace.WorkspaceGateway, workspaceConfig *workspace.WorkspaceConfig) *http.Server {
	log.Printf("[DEBUG] Setting up HTTP server\n")

	// Create a new ServeMux to avoid global state conflicts in tests
	mux := http.NewServeMux()
	mux.HandleFunc("/jsonrpc", gw.HandleJSONRPC)

	// Add simplified health check endpoint using WorkspaceGateway.Health()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// Check if gateway is properly initialized
		if gw == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintf(w, `{"status":"error","message":"workspace gateway not initialized","timestamp":%d}`, time.Now().Unix())
			return
		}

		// Get health status from workspace gateway
		health := gw.Health()

		// Determine HTTP status based on health
		statusCode := http.StatusServiceUnavailable
		if health.IsHealthy {
			statusCode = http.StatusOK
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)

		// Create simplified health response
		healthResponse := map[string]interface{}{
			"status":         getHealthStatus(health.IsHealthy),
			"workspace_root": health.WorkspaceRoot,
			"active_clients": health.ActiveClients,
			"workspace_type": "single",
			"timestamp":      time.Now().Unix(),
		}

		// Add client details if available
		if len(health.ClientStatuses) > 0 {
			healthResponse["client_statuses"] = health.ClientStatuses
		}

		// Add errors if any
		if len(health.Errors) > 0 {
			healthResponse["errors"] = health.Errors
		}

		// Add workspace information
		if workspaceConfig != nil {
			healthResponse["workspace_id"] = workspaceConfig.Workspace.WorkspaceID
			healthResponse["project_type"] = workspaceConfig.Workspace.ProjectType
			healthResponse["languages"] = workspaceConfig.Workspace.Languages
		}

		if _, err := fmt.Fprintf(w, "%s", formatHealthResponse(healthResponse)); err != nil {
			log.Printf("[WARN] Failed to write health response: %v\n", err)
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
		// Workspace validation (simplified from project validation)
		func() *ValidationError {
			if projectPath != "" {
				// Validate workspace path if specified
				if _, err := os.Stat(projectPath); os.IsNotExist(err) {
					return &ValidationError{
						Field:   "project",
						Value:   projectPath,
						Message: fmt.Sprintf("workspace path does not exist: %s", projectPath),
					}
				}
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

// Legacy function - replaced by workspace detection approach
// Kept for backward compatibility with CLI flags but simplified
func performWorkspaceDetection() (*workspace.WorkspaceContext, error) {
	// Use workspace detector instead of complex project detection
	detector := workspace.NewWorkspaceDetector()
	
	// Determine detection path from CLI flags or use current directory
	detectionPath := projectPath
	if autoDetectProject && detectionPath == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
		detectionPath = wd
	}

	if detectionPath == "" {
		// Use current directory as default
		return detector.DetectWorkspace()
	}

	log.Printf("[INFO] Starting workspace detection at path: %s\n", detectionPath)
	return detector.DetectWorkspaceAt(detectionPath)
}







// canStartWithBypassedServers checks if workspace gateway can operate with some servers bypassed
func canStartWithBypassedServers(gw workspace.WorkspaceGateway) bool {
	if gw == nil {
		return false
	}

	// Use workspace gateway health check for simplified validation
	health := gw.Health()
	return health.IsHealthy && health.ActiveClients > 0
}

// getHealthStatus returns string status based on health boolean
func getHealthStatus(isHealthy bool) string {
	if isHealthy {
		return "ok"
	}
	return "unhealthy"
}

// formatHealthResponse formats health response as JSON string
func formatHealthResponse(response map[string]interface{}) string {
	// Simple JSON formatting without external dependencies
	var parts []string
	
	for key, value := range response {
		switch v := value.(type) {
		case string:
			parts = append(parts, fmt.Sprintf(`"%s":"%s"`, key, v))
		case int:
			parts = append(parts, fmt.Sprintf(`"%s":%d`, key, v))
		case int64:
			parts = append(parts, fmt.Sprintf(`"%s":%d`, key, v))
		case bool:
			parts = append(parts, fmt.Sprintf(`"%s":%t`, key, v))
		case []string:
			if len(v) > 0 {
				var items []string
				for _, item := range v {
					items = append(items, fmt.Sprintf(`"%s"`, item))
				}
				parts = append(parts, fmt.Sprintf(`"%s":[%s]`, key, strings.Join(items, ",")))
			} else {
				parts = append(parts, fmt.Sprintf(`"%s":[]`, key))
			}
		default:
			// Skip complex types for now
		}
	}
	
	return fmt.Sprintf("{%s}", strings.Join(parts, ","))
}

// isTerminalInteractive checks if we're running in an interactive terminal
func isTerminalInteractive() bool {
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
