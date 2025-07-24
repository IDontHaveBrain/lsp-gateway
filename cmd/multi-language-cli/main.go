package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/gateway"
)

func main() {
	var (
		projectPath    = flag.String("project", ".", "Path to the project to analyze")
		command        = flag.String("command", "detect", "Command to run: detect, config, status, symbols, serve")
		outputFormat   = flag.String("format", "json", "Output format: json, yaml, text")
		outputFile     = flag.String("output", "", "Output file (default: stdout)")
		serverPort     = flag.Int("port", 8080, "Server port for serve command")
		symbolQuery    = flag.String("query", "", "Symbol query for symbols command")
		verbose        = flag.Bool("verbose", false, "Enable verbose logging")
	)
	flag.Parse()

	// Setup logging
	logger := log.New(os.Stderr, "[MULTI-LANG-CLI] ", log.LstdFlags)
	if !*verbose {
		logger.SetOutput(os.Stderr)
	}

	// Get absolute project path
	absProjectPath, err := filepath.Abs(*projectPath)
	if err != nil {
		logger.Fatalf("Failed to get absolute path: %v", err)
	}

	// Execute command
	switch *command {
	case "detect":
		err = detectLanguages(absProjectPath, *outputFormat, *outputFile, logger)
	case "config":
		err = generateConfig(absProjectPath, *outputFormat, *outputFile, logger)
	case "status":
		err = showProjectStatus(absProjectPath, *outputFormat, *outputFile, logger)
	case "symbols":
		if *symbolQuery == "" {
			logger.Fatalf("Symbol query is required for symbols command (use -query flag)")
		}
		err = searchSymbols(absProjectPath, *symbolQuery, *outputFormat, *outputFile, logger)
	case "serve":
		err = startServer(absProjectPath, *serverPort, logger)
	default:
		logger.Fatalf("Unknown command: %s. Available commands: detect, config, status, symbols, serve", *command)
	}

	if err != nil {
		logger.Fatalf("Command failed: %v", err)
	}
}

func detectLanguages(projectPath, format, outputFile string, logger *log.Logger) error {
	logger.Printf("Detecting languages in project: %s", projectPath)

	// Initialize scanner
	scanner := gateway.NewProjectLanguageScanner()
	scanner.OptimizeForLargeMonorepos()

	// Scan project
	projectInfo, err := scanner.ScanProjectComprehensive(projectPath)
	if err != nil {
		return fmt.Errorf("failed to scan project: %w", err)
	}

	// Format output
	result := map[string]interface{}{
		"project_path":      projectInfo.RootPath,
		"project_type":      projectInfo.ProjectType,
		"dominant_language": projectInfo.DominantLanguage,
		"total_files":       projectInfo.TotalFileCount,
		"scan_duration":     projectInfo.ScanDuration.String(),
		"languages":         make(map[string]interface{}),
		"detected_at":       projectInfo.DetectedAt.Format(time.RFC3339),
	}

	// Add language details
	for lang, ctx := range projectInfo.Languages {
		result["languages"].(map[string]interface{})[lang] = map[string]interface{}{
			"file_count":     ctx.FileCount,
			"test_files":     ctx.TestFileCount,
			"priority":       ctx.Priority,
			"confidence":     ctx.Confidence,
			"framework":      ctx.Framework,
			"version":        ctx.Version,
			"lsp_server":     ctx.LSPServerName,
			"root_path":      ctx.RootPath,
			"build_files":    ctx.BuildFiles,
			"config_files":   ctx.ConfigFiles,
			"source_paths":   ctx.SourcePaths,
			"test_paths":     ctx.TestPaths,
		}
	}

	return outputResult(result, format, outputFile, logger)
}

func generateConfig(projectPath, format, outputFile string, logger *log.Logger) error {
	logger.Printf("Generating multi-language configuration for: %s", projectPath)

	// Generate config from path
	mlConfig, err := config.AutoGenerateConfigFromPath(projectPath)
	if err != nil {
		return fmt.Errorf("failed to generate config: %w", err)
	}

	// Convert to different formats based on request
	var result interface{}
	switch format {
	case "yaml":
		result = mlConfig
	case "gateway":
		gatewayConfig, err := mlConfig.ToGatewayConfig()
		if err != nil {
			return fmt.Errorf("failed to convert to gateway config: %w", err)
		}
		result = gatewayConfig
	default:
		result = map[string]interface{}{
			"project_info":    mlConfig.ProjectInfo,
			"servers":         mlConfig.ServerConfigs,
			"workspace_roots": mlConfig.WorkspaceRoots,
			"generated_at":    mlConfig.GeneratedAt.Format(time.RFC3339),
			"supported_languages": mlConfig.GetSupportedLanguages(),
		}
	}

	return outputResult(result, format, outputFile, logger)
}

func showProjectStatus(projectPath, format, outputFile string, logger *log.Logger) error {
	logger.Printf("Showing project status for: %s", projectPath)

	// Create integrator
	gatewayConfig := createDefaultConfig()
	integrator := gateway.NewMultiLanguageIntegrator(gatewayConfig, logger)

	// Get project status
	status, err := integrator.GetProjectLanguageStatus(projectPath)
	if err != nil {
		return fmt.Errorf("failed to get project status: %w", err)
	}

	// Get performance metrics
	scanner := gateway.NewProjectLanguageScanner()
	perfMetrics := scanner.GetPerformanceMetrics()

	result := map[string]interface{}{
		"project_status":       status,
		"performance_metrics":  perfMetrics,
		"cache_stats":          scanner.GetCacheStats(),
	}

	return outputResult(result, format, outputFile, logger)
}

func searchSymbols(projectPath, query, format, outputFile string, logger *log.Logger) error {
	logger.Printf("Searching symbols in project: %s, query: %s", projectPath, query)

	// Create integrator
	gatewayConfig := createDefaultConfig()
	integrator := gateway.NewMultiLanguageIntegrator(gatewayConfig, logger)

	// Search symbols
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	symbolResult, err := integrator.ProcessCrossLanguageSymbolSearch(ctx, projectPath, query)
	if err != nil {
		return fmt.Errorf("failed to search symbols: %w", err)
	}

	result := map[string]interface{}{
		"query":           symbolResult.Query,
		"total_symbols":   symbolResult.TotalSymbols,
		"processing_time": symbolResult.ProcessingTime.String(),
		"languages":       symbolResult.Languages,
	}

	return outputResult(result, format, outputFile, logger)
}

func startServer(projectPath string, port int, logger *log.Logger) error {
	logger.Printf("Starting multi-language LSP server on port %d for project: %s", port, projectPath)

	// Create configuration
	gatewayConfig := createDefaultConfig()
	gatewayConfig.Port = port

	// Create integrator
	integrator := gateway.NewMultiLanguageIntegrator(gatewayConfig, logger)

	// Initialize integrator
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := integrator.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize integrator: %w", err)
	}

	// Detect and configure project
	_, err := integrator.DetectAndConfigureProject(projectPath)
	if err != nil {
		return fmt.Errorf("failed to configure project: %w", err)
	}

	logger.Printf("Multi-language LSP server started successfully!")
	logger.Printf("Project configured with multi-language support")
	logger.Printf("Server listening on http://localhost:%d/jsonrpc", port)

	// Keep server running
	select {}
}

func outputResult(result interface{}, format, outputFile string, logger *log.Logger) error {
	var output []byte
	var err error

	switch format {
	case "json":
		output, err = json.MarshalIndent(result, "", "  ")
	case "yaml":
		// For YAML output, we'll use a simple representation
		output, err = json.MarshalIndent(result, "", "  ")
		if err == nil {
			output = []byte(fmt.Sprintf("# YAML format requested, but using JSON\n%s", string(output)))
		}
	case "text":
		output = []byte(formatAsText(result))
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}

	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	// Write output
	if outputFile == "" {
		fmt.Print(string(output))
	} else {
		if err := os.WriteFile(outputFile, output, 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		logger.Printf("Output written to: %s", outputFile)
	}

	return nil
}

func formatAsText(data interface{}) string {
	var builder strings.Builder

	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			builder.WriteString(fmt.Sprintf("%s: ", key))
			
			switch val := value.(type) {
			case map[string]interface{}:
				builder.WriteString("\n")
				for subKey, subVal := range val {
					builder.WriteString(fmt.Sprintf("  %s: %v\n", subKey, subVal))
				}
			case []interface{}:
				builder.WriteString(fmt.Sprintf("[%d items]\n", len(val)))
			default:
				builder.WriteString(fmt.Sprintf("%v\n", val))
			}
		}
	default:
		builder.WriteString(fmt.Sprintf("%v\n", v))
	}

	return builder.String()
}

func createDefaultConfig() *config.GatewayConfig {
	return &config.GatewayConfig{
		Port:                            8080,
		Timeout:                         "30s",
		MaxConcurrentRequests:           100,
		ProjectAware:                    true,
		EnableConcurrentServers:         true,
		MaxConcurrentServersPerLanguage: 3,
		Servers: []config.ServerConfig{
			{
				Name:        "gopls",
				Languages:   []string{"go"},
				Command:     "gopls",
				Transport:   "stdio",
				RootMarkers: []string{"go.mod", "go.sum"},
				Priority:    5,
				Weight:      1.0,
			},
			{
				Name:        "python-lsp-server",
				Languages:   []string{"python"},
				Command:     "python",
				Args:        []string{"-m", "pylsp"},
				Transport:   "stdio",
				RootMarkers: []string{"pyproject.toml", "setup.py"},
				Priority:    4,
				Weight:      1.0,
			},
			{
				Name:        "typescript-language-server",
				Languages:   []string{"typescript", "javascript"},
				Command:     "typescript-language-server",
				Args:        []string{"--stdio"},
				Transport:   "stdio",
				RootMarkers: []string{"tsconfig.json", "package.json"},
				Priority:    4,
				Weight:      1.0,
			},
			{
				Name:        "jdtls",
				Languages:   []string{"java"},
				Command:     "jdtls",
				Transport:   "stdio",
				RootMarkers: []string{"pom.xml", "build.gradle"},
				Priority:    3,
				Weight:      1.0,
			},
			{
				Name:        "rust-analyzer",
				Languages:   []string{"rust"},
				Command:     "rust-analyzer", 
				Transport:   "stdio",
				RootMarkers: []string{"Cargo.toml"},
				Priority:    3,
				Weight:      1.0,
			},
		},
		LanguagePools: []config.LanguageServerPool{},
		GlobalMultiServerConfig: config.DefaultMultiServerConfig(),
	}
}