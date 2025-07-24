package cli

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
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
	configPath               string
	port                     int
	// Project-related flags
	projectPath              string
	autoDetectProject        bool
	generateProjectConfig    bool
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

	rootCmd.AddCommand(serverCmd)
}

// GetServerCmd returns the server command for testing purposes
func GetServerCmd() *cobra.Command {
	return serverCmd
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
				for _, client := range projectGw.Gateway.Clients {
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
			return ValidateFilePath(configPath, "config", "read")
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
	)
	if err == nil {
		return nil
	}
	return err
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
	// Load base configuration
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
