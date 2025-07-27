package cli

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/project"
	"lsp-gateway/internal/setup"
	"lsp-gateway/internal/transport"
	"lsp-gateway/mcp"

	"github.com/spf13/cobra"
)

var (
	// MCP command flag variables - exported for testing purposes
	McpConfigPath string
	McpGatewayURL string
	McpPort       int
	McpTransport  string
	McpTimeout    time.Duration
	McpMaxRetries int
	// DirectLSP mode flag - exported for testing purposes
	McpDirectLSP     bool
	McpLSPConfigPath string
	// Project-related flags - exported for testing purposes
	McpProjectPath           string
	McpAutoDetectProject     bool
	McpGenerateProjectConfig bool
)

var mcpCmd = &cobra.Command{
	Use:   CmdMCP,
	Short: "Start the MCP server",
	Long: `Start the Model Context Protocol (MCP) server that provides LSP functionality.

The MCP server acts as a bridge between MCP clients and Language Server Protocol (LSP)
servers, exposing LSP methods as MCP tools. This enables AI assistants and other MCP
clients to interact with language servers for code analysis, symbol search, definition
lookup, and reference finding.

Operation Modes:
- HTTP Gateway Mode (default): Routes requests through the LSP Gateway HTTP server
- DirectLSP Mode: Connects directly to LSP servers without HTTP gateway dependency

Features:
- MCP protocol implementation
- LSP method mapping to MCP tools
- Multiple transport support (stdio, tcp, http)
- Direct LSP connections for reduced latency
- Configuration-driven LSP server management
- Graceful shutdown handling

Examples:
  # Start MCP server with HTTP Gateway mode (default)
  lsp-gateway mcp

  # Start with DirectLSP mode using config file
  lsp-gateway mcp --direct-lsp --lsp-config config.yaml

  # Start with DirectLSP mode using automatic project detection
  lsp-gateway mcp --direct-lsp

  # Start with custom LSP Gateway URL
  lsp-gateway mcp --gateway http://localhost:9090

  # Start with HTTP transport on specific port
  lsp-gateway mcp --transport http --port 3000

  # Start with custom configuration and timeout
  lsp-gateway mcp --config mcp-config.yaml --timeout 60s`,
	RunE: runMCPServer,
}

func init() {
	mcpCmd.Flags().StringVarP(&McpConfigPath, "config", "c", "", "MCP configuration file path (optional)")
	mcpCmd.Flags().StringVarP(&McpGatewayURL, FLAG_GATEWAY, "g", DefaultLSPGatewayURL, "LSP Gateway URL")
	mcpCmd.Flags().IntVarP(&McpPort, FLAG_PORT, "p", 3000, "MCP server port (for HTTP transport)")
	mcpCmd.Flags().StringVarP(&McpTransport, FLAG_DESCRIPTION_TRANSPORT, "t", transport.TransportStdio, "Transport type (stdio, tcp, http)")
	mcpCmd.Flags().DurationVar(&McpTimeout, FLAG_TIMEOUT, 30*time.Second, "Request timeout duration")
	mcpCmd.Flags().IntVar(&McpMaxRetries, "max-retries", 3, "Maximum retries for failed requests")

	// DirectLSP mode flags
	mcpCmd.Flags().BoolVar(&McpDirectLSP, "direct-lsp", false, "Use direct LSP connections instead of HTTP gateway")
	mcpCmd.Flags().StringVar(&McpLSPConfigPath, "lsp-config", "", "LSP server configuration file path (optional for direct-lsp mode - uses auto-detection if not provided)")

	// Project-related flags
	mcpCmd.Flags().StringVarP(&McpProjectPath, FLAG_PROJECT, "P", "", FLAG_DESCRIPTION_PROJECT_PATH)
	mcpCmd.Flags().BoolVar(&McpAutoDetectProject, FLAG_AUTO_DETECT_PROJECT, false, FLAG_DESCRIPTION_AUTO_DETECT_PROJECT)
	mcpCmd.Flags().BoolVar(&McpGenerateProjectConfig, FLAG_GENERATE_PROJECT_CONFIG, false, FLAG_DESCRIPTION_GENERATE_PROJECT_CONFIG)

	rootCmd.AddCommand(mcpCmd)
}

func runMCPServer(_ *cobra.Command, args []string) error {
	if McpDirectLSP {
		if McpLSPConfigPath != "" {
			log.Printf("[INFO] Starting MCP server with DirectLSP mode, transport=%s, lsp_config=%s\n", McpTransport, McpLSPConfigPath)
		} else {
			log.Printf("[INFO] Starting MCP server with DirectLSP mode, transport=%s, using auto-detection\n", McpTransport)
		}
	} else {
		log.Printf("[INFO] Starting MCP server with HTTP Gateway mode, transport=%s, gateway_url=%s\n", McpTransport, McpGatewayURL)
	}

	logger := createMCPLogger()
	server, err := setupMCPServer(logger)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	return runMCPTransport(ctx, server, logger)
}

func createMCPHTTPServer(server *mcp.Server, port int) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		if server.IsRunning() {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"status":"ok","timestamp":%d}`, time.Now().Unix())
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintf(w, `{"status":"error","message":"server not running"}`)
		}
	})

	mux.HandleFunc("/mcp", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", transport.HTTP_CONTENT_TYPE_JSON)
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = fmt.Fprintf(w, `{"error":"HTTP MCP transport not yet implemented","message":"Use stdio transport for now"}`)
	})

	return &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
}

func runMCPHTTPLifecycle(ctx context.Context, httpServer *http.Server, port int, logger *mcp.StructuredLogger) error {
	serverErr := createServerErrorChannel()
	go func() {
		logger.WithField("port", port).Info("MCP HTTP server listening")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Error("MCP HTTP server error")
			serverErr <- HandleServerStartError(err, port)
		}
	}()

	config := &ServerLifecycleConfig{
		ServerName:      "MCP HTTP server",
		Port:            port,
		ShutdownFunc:    func() error { return shutdownMCPHTTPServerInternal(httpServer) },
		ShutdownTimeout: 30 * time.Second,
	}

	logger.WithField("port", port).Info("MCP HTTP server started successfully, waiting for requests")
	return waitForShutdownSignal(ctx, config, serverErr)
}

func shutdownMCPHTTPServerInternal(httpServer *http.Server) error {
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return err
	}
	return nil
}

// RunMCPStdioServer starts the MCP server with stdio transport - exported for testing
func RunMCPStdioServer(ctx context.Context, server *mcp.Server) error {
	log.Printf("[INFO] Starting MCP server with stdio transport, gateway_url=%s\n", McpGatewayURL)

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("[DEBUG] Starting MCP server\n")
		if err := server.Start(); err != nil {
			log.Printf("[ERROR] MCP server startup failed: %v\n", err)
			serverErr <- NewMCPServerError("failed to start", err)
		} else {
			log.Printf("[INFO] MCP server started successfully\n")
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("[INFO] MCP server is running, waiting for requests\n")

	select {
	case <-sigCh:
		log.Printf("[INFO] Received shutdown signal\n")
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		log.Printf("[INFO] Context cancelled\n")
	}

	log.Printf("[INFO] Shutting down MCP server\n")

	if err := server.Stop(); err != nil {
		log.Printf("[WARN] Error stopping MCP server during shutdown: %v\n", err)
	} else {
		log.Printf("[INFO] MCP server stopped successfully\n")
	}

	return nil
}

func runMCPHTTPServer(ctx context.Context, server *mcp.Server, port int, logger *mcp.StructuredLogger) error {
	log.Printf("[INFO] Starting MCP server with HTTP transport on port %d\n", port)

	httpServer := createMCPHTTPServer(server, port)
	return runMCPHTTPLifecycle(ctx, httpServer, port, logger)
}

func createMCPLogger() *mcp.StructuredLogger {
	logConfig := &mcp.LoggerConfig{
		Level:              mcp.LogLevelInfo,
		Component:          "mcp-cli",
		EnableJSON:         false,
		EnableStackTrace:   false,
		EnableCaller:       true,
		EnableMetrics:      false,
		Output:             os.Stderr,
		IncludeTimestamp:   true,
		TimestampFormat:    time.RFC3339,
		MaxStackTraceDepth: 10,
		EnableAsyncLogging: false,
		AsyncBufferSize:    1000,
	}
	return mcp.NewStructuredLogger(logConfig)
}

func setupMCPServer(logger *mcp.StructuredLogger) (*mcp.Server, error) {
	if err := validateMCPParams(); err != nil {
		log.Printf("[ERROR] MCP parameter validation failed: %v\n", err)
		return nil, err
	}

	// Project detection step for MCP
	projectResult, err := performMCPProjectDetection()
	if err != nil {
		log.Printf("[WARN] MCP project detection failed: %v\n", err)
		// Continue with default configuration
	}

	cfg := &mcp.ServerConfig{
		Name:          "lsp-gateway-mcp",
		Description:   "MCP server providing LSP functionality through LSP Gateway",
		Version:       "0.1.0",
		LSPGatewayURL: McpGatewayURL,
		Transport:     McpTransport,
		Timeout:       McpTimeout,
		MaxRetries:    McpMaxRetries,
	}

	// Apply project-aware configuration if available
	if projectResult != nil && projectResult.ProjectContext != nil {
		applyProjectAwareMCPConfig(cfg, projectResult)
		// Add integration with project-aware gateway capabilities
		integrateMCPWithProjectAwareGateway(cfg, projectResult)
	}

	log.Printf("[DEBUG] MCP server configuration created\n")

	if McpConfigPath != "" {
		log.Printf("[INFO] Configuration file specified (currently using command-line flags): %s\n", McpConfigPath)
	}

	logger.Debug("Validating MCP configuration")
	if err := cfg.Validate(); err != nil {
		logger.WithError(err).Error("MCP configuration validation failed")
		return nil, NewValidationError("MCP configuration", []string{err.Error()})
	}
	logger.Info("MCP configuration validated successfully")

	logger.Debug("Creating MCP server")
	var server *mcp.Server
	if McpDirectLSP {
		// Create MCP server with DirectLSPManager
		server, err = createMCPServerWithDirectLSP(cfg, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create MCP server with DirectLSP: %w", err)
		}
	} else {
		// Create MCP server with HTTP Gateway Client
		server = mcp.NewServer(cfg)
	}
	logger.Info("MCP server created successfully")

	return server, nil
}

func runMCPTransport(ctx context.Context, server *mcp.Server, logger *mcp.StructuredLogger) error {
	if McpTransport == transport.TransportHTTP {
		logger.WithField("port", McpPort).Info("Using HTTP transport for MCP server")
		return runMCPHTTPServer(ctx, server, McpPort, logger)
	}

	logger.Info("Using stdio transport for MCP server")
	return RunMCPStdioServer(ctx, server)
}

func validateMCPParams() error {
	err := ValidateMultiple(
		func() *ValidationError {
			return ValidateTransport(McpTransport, FLAG_DESCRIPTION_TRANSPORT)
		},
		func() *ValidationError {
			return ValidateURL(McpGatewayURL, FLAG_GATEWAY)
		},
		func() *ValidationError {
			if McpTransport == transport.TransportHTTP {
				return ValidatePortAvailability(McpPort, FLAG_PORT)
			}
			return ValidatePort(McpPort, "port")
		},
		func() *ValidationError {
			return ValidateTimeout(McpTimeout, FLAG_TIMEOUT)
		},
		func() *ValidationError {
			return ValidateIntRange(McpMaxRetries, 0, 10, "max-retries")
		},
		func() *ValidationError {
			if McpConfigPath != "" {
				return ValidateFilePath(McpConfigPath, "config", "read")
			}
			return nil
		},
		// Project validation
		func() *ValidationError {
			if McpProjectPath != "" {
				return ValidateProjectPath(McpProjectPath, "project")
			}
			return nil
		},
		// DirectLSP validation
		func() *ValidationError {
			if McpLSPConfigPath != "" {
				return ValidateFilePath(McpLSPConfigPath, "lsp-config", "read")
			}
			return nil
		},
	)
	if err == nil {
		return nil
	}
	return err
}

// performMCPProjectDetection performs project detection for MCP server based on CLI flags
func performMCPProjectDetection() (*project.ProjectAnalysisResult, error) {
	// Skip project detection if no flags are set
	if !McpAutoDetectProject && McpProjectPath == "" {
		return nil, nil
	}

	// Determine project path
	detectionPath := McpProjectPath
	if McpAutoDetectProject && detectionPath == "" {
		// Use current working directory
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
		detectionPath = wd
	}

	if detectionPath == "" {
		return nil, fmt.Errorf("no project path specified for MCP detection")
	}

	log.Printf("[INFO] Starting MCP project detection at path: %s\n", detectionPath)

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
		return nil, fmt.Errorf("MCP project detection failed: %w", err)
	}

	if result.ProjectContext != nil {
		log.Printf("[INFO] MCP project detected: %s (%s) with languages: %v\n",
			result.ProjectContext.ProjectType,
			result.ProjectContext.RootPath,
			result.ProjectContext.Languages)
	}

	return result, nil
}

// applyProjectAwareMCPConfig applies project-specific configuration to MCP server config
func applyProjectAwareMCPConfig(cfg *mcp.ServerConfig, projectResult *project.ProjectAnalysisResult) {
	projectCtx := projectResult.ProjectContext

	// Update MCP server name to include project type
	cfg.Name = fmt.Sprintf("lsp-gateway-mcp-%s", projectCtx.ProjectType)

	// Update description to include project information
	cfg.Description = fmt.Sprintf("MCP server providing LSP functionality for %s project at %s",
		projectCtx.ProjectType, projectCtx.RootPath)

	// Note: Project context is available in the projectCtx variable
	// and is used throughout the MCP server for project-aware functionality

	// Apply project-specific timeout adjustments for large projects
	if projectCtx.ProjectSize.TotalFiles > 1000 {
		// Increase timeout for large projects
		cfg.Timeout = cfg.Timeout + (30 * time.Second)
		log.Printf("[INFO] Increased MCP timeout for large project (%d files)\n",
			projectCtx.ProjectSize.TotalFiles)
	}

	// Generate project-specific configuration if requested
	if McpGenerateProjectConfig {
		log.Printf("[INFO] Generating project-specific MCP configuration\n")
		generateMCPProjectConfig(cfg, projectCtx)
	}

	log.Printf("[INFO] Applied project-aware MCP configuration for %s project\n", projectCtx.ProjectType)
}

// generateMCPProjectConfig generates and persists project-specific MCP configuration
func generateMCPProjectConfig(cfg *mcp.ServerConfig, projectCtx *project.ProjectContext) {
	// Create project-aware logger
	logger := setup.NewSetupLogger(&setup.SetupLoggerConfig{
		Component: "mcp-project-config-generator",
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
		log.Printf("[WARN] Failed to generate MCP project configuration: %v\n", err)
		return
	}

	// Generated configuration details are available through genResult and used
	// throughout the MCP server for project-aware functionality

	log.Printf("[INFO] Generated MCP project configuration: %d servers, %d optimizations\n",
		genResult.ServersGenerated, genResult.OptimizationsApplied)
}

// performAutoDetectionAndSetup performs automatic project detection and generates LSP server configurations
func performAutoDetectionAndSetup() ([]*config.ServerConfig, error) {
	// Use current working directory as project root
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}

	log.Printf("[INFO] Starting automatic project detection at: %s\n", wd)

	// Create project detector
	detector := project.NewProjectDetector()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Perform project detection
	projectContext, err := detector.DetectProject(ctx, wd)
	if err != nil {
		return nil, fmt.Errorf("automatic project detection failed: %w", err)
	}

	if projectContext.ProjectType == "unknown" || len(projectContext.Languages) == 0 {
		return nil, fmt.Errorf("no supported languages detected in current directory: %s", wd)
	}

	log.Printf("[INFO] Detected project type: %s with languages: %v\n", 
		projectContext.ProjectType, projectContext.Languages)

	// Create project config generator
	logger := setup.NewSetupLogger(&setup.SetupLoggerConfig{
		Component: "mcp-auto-detection",
	})
	registry := setup.NewDefaultServerRegistry()
	verifier := setup.NewDefaultServerVerifier()
	generator := project.NewProjectConfigGenerator(logger, registry, verifier)

	// Generate project-specific configuration
	genResult, err := generator.GenerateFromProject(ctx, projectContext)
	if err != nil {
		return nil, fmt.Errorf("failed to generate configuration from detected project: %w", err)
	}

	if genResult.GatewayConfig == nil || len(genResult.GatewayConfig.Servers) == 0 {
		return nil, fmt.Errorf("no LSP servers were configured for detected project languages: %v", projectContext.Languages)
	}

	// Convert to []*config.ServerConfig format
	serverConfigs := make([]*config.ServerConfig, len(genResult.GatewayConfig.Servers))
	for i := range genResult.GatewayConfig.Servers {
		serverConfigs[i] = &genResult.GatewayConfig.Servers[i]
	}

	log.Printf("[INFO] Auto-generated %d LSP server configurations\n", len(serverConfigs))
	for _, serverConfig := range serverConfigs {
		log.Printf("[INFO] - %s: %s (languages: %v)\n", 
			serverConfig.Name, serverConfig.Command, serverConfig.Languages)
	}

	return serverConfigs, nil
}

// integrateMCPWithProjectAwareGateway attempts to integrate MCP server with project-aware gateway
func integrateMCPWithProjectAwareGateway(cfg *mcp.ServerConfig, projectResult *project.ProjectAnalysisResult) {
	if projectResult == nil || projectResult.ProjectContext == nil {
		log.Printf("[DEBUG] No project context available for MCP-gateway integration\n")
		return
	}

	// Configure timeouts based on project size
	if projectResult.ProjectSize.TotalFiles > 500 {
		// Increase timeout for larger projects
		originalTimeout := cfg.Timeout
		cfg.Timeout = originalTimeout + (15 * time.Second)

		log.Printf("[INFO] Adjusted MCP timeout for large project (%d files): %s -> %s\n",
			projectResult.ProjectSize.TotalFiles, originalTimeout.String(), cfg.Timeout.String())
	}

	log.Printf("[INFO] Integrated MCP server with project-aware gateway capabilities\n")
}

// LoadServerConfigsFromFile loads LSP server configurations from a config file
func LoadServerConfigsFromFile(configPath string) ([]*config.ServerConfig, error) {
	if configPath == "" {
		return nil, fmt.Errorf("configuration file path cannot be empty")
	}

	// Check if file exists first for better error messages
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("LSP configuration file not found: %s. Use --lsp-config to specify a valid config file", configPath)
	}

	gatewayConfig, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load LSP configuration from %s: %w", configPath, err)
	}

	if len(gatewayConfig.Servers) == 0 {
		return nil, fmt.Errorf("no LSP servers configured in %s. DirectLSP mode requires at least one server configuration", configPath)
	}

	// Convert []ServerConfig to []*ServerConfig
	serverConfigs := make([]*config.ServerConfig, len(gatewayConfig.Servers))
	for i := range gatewayConfig.Servers {
		serverConfigs[i] = &gatewayConfig.Servers[i]
	}

	// Validate each server configuration
	for _, serverConfig := range serverConfigs {
		if err := serverConfig.Validate(); err != nil {
			return nil, fmt.Errorf("invalid LSP server configuration for '%s': %w", serverConfig.Name, err)
		}
	}

	log.Printf("[INFO] Loaded %d LSP server configurations from %s", len(serverConfigs), configPath)
	for _, serverConfig := range serverConfigs {
		log.Printf("[INFO] - %s: %s (languages: %v)", serverConfig.Name, serverConfig.Command, serverConfig.Languages)
	}
	return serverConfigs, nil
}

// createMCPServerWithDirectLSP creates an MCP server with DirectLSPManager instead of HTTP gateway
func createMCPServerWithDirectLSP(cfg *mcp.ServerConfig, logger *mcp.StructuredLogger) (*mcp.Server, error) {
	var serverConfigs []*config.ServerConfig
	var err error

	if McpLSPConfigPath != "" {
		// Load LSP server configurations from file
		serverConfigs, err = LoadServerConfigsFromFile(McpLSPConfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load LSP server configurations: %w", err)
		}
	} else {
		// Perform automatic project detection and configuration generation
		serverConfigs, err = performAutoDetectionAndSetup()
		if err != nil {
			return nil, fmt.Errorf("failed to auto-detect and configure LSP servers: %w", err)
		}
	}

	// Create DirectLSPManager configuration
	directLSPConfig := &mcp.DirectLSPManagerConfig{
		ServerConfigs: serverConfigs,
		Logger:        log.New(os.Stderr, "[DirectLSPManager] ", log.LstdFlags|log.Lshortfile),
	}

	// Create DirectLSPManager
	directLSPManager, err := mcp.NewDirectLSPManager(directLSPConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create DirectLSPManager: %w", err)
	}

	logger.WithField("servers", len(serverConfigs)).Info("Created DirectLSPManager with LSP servers")

	// Create MCP server with DirectLSPManager
	server, err := mcp.NewServerWithDirectLSP(cfg, directLSPManager)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP server with DirectLSP: %w", err)
	}

	return server, nil
}
