package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/installer"

	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var (
	diagnoseJSON               bool
	diagnoseVerbose            bool
	diagnoseTimeout            time.Duration
	diagnoseAll                bool
	// Multi-language diagnostic flags
	diagnoseMultiLanguage      bool
	diagnosePerformance        bool
	diagnoseRouting            bool
	diagnoseResourceLimits     bool
	diagnoseOptimization       bool
	diagnoseProjectPath        string
	diagnoseCheckConsistency   bool
	diagnosePerformanceMode    string
	diagnoseComprehensive      bool
	diagnoseTemplateValidation bool
)

var diagnoseCmd = &cobra.Command{
	Use:   "diagnose",
	Short: "Run comprehensive system diagnostics",
	Long: `Run comprehensive diagnostics to identify and troubleshoot system issues.

The diagnose command performs detailed analysis of your LSP Gateway installation,
runtime environment, and configuration to identify potential problems and provide
actionable recommendations for resolution.

Diagnostic Areas:
- Runtime installations and compatibility
- Language server functionality
- System environment and permissions
- Configuration validation
- Network connectivity (for HTTP mode)
- Common installation issues
- Multi-language project consistency
- Performance settings and optimization
- Smart routing configuration
- Resource usage and limits

Examples:
  # Run all diagnostics
  lsp-gateway diagnose
  
  # Diagnose only runtimes
  lsp-gateway diagnose runtimes
  
  # Verbose diagnostic output
  lsp-gateway diagnose --verbose
  
  # JSON output for automation
  lsp-gateway diagnose --json
  
  # Multi-language diagnostics
  lsp-gateway diagnose --multi-language --project-path /path/to/project
  
  # Performance diagnostics
  lsp-gateway diagnose --performance --performance-mode production
  
  # Routing diagnostics
  lsp-gateway diagnose --routing --check-consistency`,
	RunE: diagnoseSystem,
}

var diagnoseRuntimesCmd = &cobra.Command{
	Use:   "runtimes",
	Short: "Run runtime diagnostics",
	Long: `Run detailed diagnostics for runtime installations.

This command performs comprehensive checks on all supported runtimes including:
- Installation detection and path validation
- Version compatibility analysis
- Permission and accessibility checks
- Common installation issues
- Configuration recommendations

Examples:
  # Diagnose all runtimes
  lsp-gateway diagnose runtimes
  
  # Verbose runtime diagnostics
  lsp-gateway diagnose runtimes --verbose`,
	RunE: diagnoseRuntimes,
}

var diagnoseServersCmd = &cobra.Command{
	Use:   "servers",
	Short: "Run language server diagnostics",
	Long: `Run detailed diagnostics for language server installations.

This command performs comprehensive checks on all supported language servers including:
- Installation detection and path validation
- Runtime dependency verification
- Functional testing and health checks
- Communication and responsiveness tests
- Common installation and configuration issues

Examples:
  # Diagnose all language servers
  lsp-gateway diagnose servers
  
  # Verbose server diagnostics
  lsp-gateway diagnose servers --verbose`,
	RunE: diagnoseServers,
}

var diagnoseConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Run configuration diagnostics",
	Long: `Run detailed diagnostics for configuration files and settings.

This command performs comprehensive checks on configuration including:
- Configuration file existence and accessibility
- YAML syntax validation
- Schema and structure validation
- Server configuration completeness
- Path and runtime consistency checks

Examples:
  # Diagnose configuration
  lsp-gateway diagnose config
  
  # Verbose configuration diagnostics
  lsp-gateway diagnose config --verbose
  
  # Multi-language configuration diagnostics
  lsp-gateway diagnose config --multi-language --project-path /path/to/project
  
  # Performance configuration diagnostics
  lsp-gateway diagnose config --performance --performance-mode production`,
	RunE: diagnoseConfig,
}

var diagnoseOptimizationCmd = &cobra.Command{
	Use:   "optimization",
	Short: "Run optimization diagnostics",
	Long: `Run detailed diagnostics for configuration optimization and performance settings.

This command performs comprehensive checks on optimization including:
- Performance settings validation
- Resource limit analysis
- Optimization mode compatibility
- Memory and CPU usage projections
- Concurrent request handling capacity

Examples:
  # Diagnose optimization settings
  lsp-gateway diagnose optimization
  
  # Check production optimization settings
  lsp-gateway diagnose optimization --performance-mode production
  
  # Comprehensive optimization analysis
  lsp-gateway diagnose optimization --check-consistency --performance`,
	RunE: diagnoseOptimization,
}

func init() {
	diagnoseCmd.PersistentFlags().BoolVar(&diagnoseJSON, "json", false, "Output in JSON format")
	diagnoseCmd.PersistentFlags().BoolVarP(&diagnoseVerbose, "verbose", "v", false, "Verbose diagnostic output")
	diagnoseCmd.PersistentFlags().DurationVar(&diagnoseTimeout, FLAG_TIMEOUT, 60*time.Second, "Diagnostic timeout")
	diagnoseCmd.PersistentFlags().BoolVar(&diagnoseAll, "all", false, "Run all available diagnostics")
	// Multi-language diagnostic flags
	diagnoseCmd.PersistentFlags().BoolVar(&diagnoseMultiLanguage, "multi-language", false, "Enable multi-language diagnostics")
	diagnoseCmd.PersistentFlags().BoolVar(&diagnosePerformance, "performance", false, "Enable performance diagnostics")
	diagnoseCmd.PersistentFlags().BoolVar(&diagnoseRouting, "routing", false, "Enable routing diagnostics")
	diagnoseCmd.PersistentFlags().BoolVar(&diagnoseResourceLimits, "resource-limits", false, "Enable resource limit diagnostics")
	diagnoseCmd.PersistentFlags().BoolVar(&diagnoseOptimization, "optimization", false, "Enable optimization diagnostics")
	diagnoseCmd.PersistentFlags().StringVar(&diagnoseProjectPath, "project-path", "", "Project path for multi-language diagnostics")
	diagnoseCmd.PersistentFlags().BoolVar(&diagnoseCheckConsistency, "check-consistency", false, "Enable consistency checking")
	diagnoseCmd.PersistentFlags().StringVar(&diagnosePerformanceMode, "performance-mode", "development", "Performance mode for diagnostics (development, production, analysis)")
	diagnoseCmd.PersistentFlags().BoolVar(&diagnoseComprehensive, "comprehensive", false, "Enable comprehensive diagnostics with scoring")
	diagnoseCmd.PersistentFlags().BoolVar(&diagnoseTemplateValidation, "template-validation", false, "Enable template adherence validation")

	// Add enhanced multi-language diagnostic commands
	diagnoseMultiLanguageCmd := &cobra.Command{
		Use:   "multi-language",
		Short: "Run multi-language configuration diagnostics",
		Long:  `Run detailed diagnostics for multi-language configurations, cross-language consistency, and project structure validation.`,
		RunE:  diagnoseMultiLanguageConfiguration,
	}

	diagnosePerformanceCmd := &cobra.Command{
		Use:   "performance",
		Short: "Run performance configuration diagnostics",
		Long:  `Run detailed diagnostics for performance settings, resource limits, and optimization modes.`,
		RunE:  diagnosePerformanceConfiguration,
	}

	diagnoseRoutingCmd := &cobra.Command{
		Use:   "routing",
		Short: "Run smart routing diagnostics",
		Long:  `Run detailed diagnostics for smart routing strategies, load balancing, and server selection.`,
		RunE:  diagnoseRoutingStrategy,
	}

	diagnoseCmd.AddCommand(diagnoseRuntimesCmd)
	diagnoseCmd.AddCommand(diagnoseServersCmd)
	diagnoseCmd.AddCommand(diagnoseConfigCmd)
	diagnoseCmd.AddCommand(diagnoseOptimizationCmd)
	diagnoseCmd.AddCommand(diagnoseMultiLanguageCmd)
	diagnoseCmd.AddCommand(diagnosePerformanceCmd)
	diagnoseCmd.AddCommand(diagnoseRoutingCmd)

	rootCmd.AddCommand(diagnoseCmd)
}

type DiagnosticResult struct {
	Name        string                 `json:"name"`
	Status      string                 `json:"status"` // "passed", "failed", "warning", "skipped"
	Message     string                 `json:"message"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Suggestions []string               `json:"suggestions,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

type DiagnosticReport struct {
	Timestamp      time.Time          `json:"timestamp"`
	Platform       string             `json:"platform"`
	GoVersion      string             `json:"go_version"`
	LSPGatewayPath string             `json:"lsp_gateway_path"`
	Results        []DiagnosticResult `json:"results"`
	Summary        DiagnosticSummary  `json:"summary"`
}

type DiagnosticSummary struct {
	TotalChecks   int     `json:"total_checks"`
	Passed        int     `json:"passed"`
	Failed        int     `json:"failed"`
	Warnings      int     `json:"warnings"`
	Skipped       int     `json:"skipped"`
	OverallStatus string  `json:"overall_status"`
	Score         float64 `json:"score,omitempty"`
	Grade         string  `json:"grade,omitempty"`
}

// Enhanced diagnostic scoring and grading
type DiagnosticScoring struct {
	ConfigurationHealth  float64 `json:"configuration_health"`
	PerformanceRating    float64 `json:"performance_rating"`
	ConsistencyScore     float64 `json:"consistency_score"`
	OptimizationLevel    float64 `json:"optimization_level"`
	MultiLanguageHealth  float64 `json:"multi_language_health"`
	RoutingEfficiency    float64 `json:"routing_efficiency"`
}

func diagnoseSystem(cmd *cobra.Command, args []string) error {
	if err := validateDiagnoseParams(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), diagnoseTimeout)
	defer cancel()

	if !diagnoseJSON {
		printDiagnoseHeader("LSP Gateway System Diagnostics")
	}

	report := &DiagnosticReport{
		Timestamp:      time.Now(),
		Platform:       fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		GoVersion:      runtime.Version(),
		LSPGatewayPath: os.Args[0],
		Results:        []DiagnosticResult{},
	}

	var errors []error

	if err := runSystemDiagnostics(ctx, report); err != nil {
		errors = append(errors, &CLIError{
			Type:    ErrorTypeGeneral,
			Message: "System diagnostics encountered issues",
			Cause:   err,
			Suggestions: []string{
				"Check if system utilities are available (lsof, ps, df, etc.)",
				"Verify system permissions for diagnostic tools",
				"Try running with elevated privileges if needed",
			},
		})
	}

	if err := runRuntimeDiagnostics(ctx, report); err != nil {
		errors = append(errors, &CLIError{
			Type:    ErrorTypeRuntime,
			Message: "Runtime diagnostics encountered issues",
			Cause:   err,
			Suggestions: []string{
				"Check if runtimes are properly installed (go, python, node, java)",
				"Verify runtime versions meet minimum requirements",
				"Update or reinstall problematic runtimes",
			},
			RelatedCmds: []string{
				"status runtimes",
				"install runtime <name>",
			},
		})
	}

	if err := runServerDiagnostics(ctx, report); err != nil {
		errors = append(errors, &CLIError{
			Type:    ErrorTypeServer,
			Message: "Language server diagnostics encountered issues",
			Cause:   err,
			Suggestions: []string{
				"Check if language servers are properly installed",
				"Verify servers are accessible in your PATH",
				"Install missing servers: lsp-gateway install servers",
			},
			RelatedCmds: []string{
				"status servers",
				"install servers",
				"install server <name>",
			},
		})
	}

	if err := runConfigDiagnostics(ctx, report); err != nil {
		errors = append(errors, &CLIError{
			Type:    ErrorTypeConfig,
			Message: "Configuration diagnostics encountered issues",
			Cause:   err,
			Suggestions: []string{
				"Validate configuration file: lsp-gateway config validate",
				"Generate a fresh config: lsp-gateway config generate --overwrite",
				"Check config file permissions and syntax",
			},
			RelatedCmds: []string{
				"config validate",
				"config generate",
				"config show",
			},
		})
	}

	calculateSummary(report)

	// Run enhanced multi-language diagnostics if enabled
	if diagnoseMultiLanguage {
		if err := runMultiLanguageDiagnostics(ctx, report); err != nil {
			errors = append(errors, &CLIError{
				Type:    ErrorTypeConfig,
				Message: "Multi-language diagnostics encountered issues",
				Cause:   err,
				Suggestions: []string{
					"Verify project path is accessible",
					"Ensure multi-language configuration is valid",
					"Check language detection and consistency",
				},
				RelatedCmds: []string{
					"diagnose multi-language",
					"config validate",
				},
			})
		}
	}

	// Run performance diagnostics if enabled
	if diagnosePerformance {
		if err := runPerformanceDiagnostics(ctx, report); err != nil {
			errors = append(errors, &CLIError{
				Type:    ErrorTypeConfig,
				Message: "Performance diagnostics encountered issues",
				Cause:   err,
				Suggestions: []string{
					"Review performance configuration settings",
					"Check resource limits and constraints",
					"Validate optimization mode compatibility",
				},
				RelatedCmds: []string{
					"diagnose performance",
					"diagnose optimization",
				},
			})
		}
	}

	// Run routing diagnostics if enabled
	if diagnoseRouting {
		if err := runRoutingDiagnostics(ctx, report); err != nil {
			errors = append(errors, &CLIError{
				Type:    ErrorTypeConfig,
				Message: "Routing diagnostics encountered issues",
				Cause:   err,
				Suggestions: []string{
					"Review smart routing configuration",
					"Check for language routing conflicts",
					"Validate load balancing settings",
				},
				RelatedCmds: []string{
					"diagnose routing",
					"config validate",
				},
			})
		}
	}

	// Add enhanced diagnostic summary information
	if diagnoseMultiLanguage || diagnosePerformance || diagnoseRouting || diagnoseResourceLimits {
		addEnhancedDiagnosticSummary(report)
	}

	// Calculate comprehensive scoring if enabled
	if diagnoseComprehensive {
		addComprehensiveDiagnosticScoring(report)
	}

	if diagnoseJSON {
		return outputDiagnoseJSON(report)
	}

	if err := outputDiagnoseHuman(report); err != nil {
		return &CLIError{
			Type:    ErrorTypeGeneral,
			Message: "Failed to display diagnostic report",
			Cause:   err,
			Suggestions: []string{
				"Check terminal output capabilities",
				"Try redirecting output: lsp-gateway diagnose > diagnostics.txt",
				"Report this as a bug if it persists",
			},
		}
	}

	if len(errors) > 0 {
		fmt.Printf("\nErrors encountered during diagnostics:\n")
		for _, err := range errors {
			fmt.Printf("  - %v\n", err)
		}
	}

	return nil
}

func diagnoseRuntimes(cmd *cobra.Command, args []string) error {
	_, cancel := context.WithTimeout(context.Background(), diagnoseTimeout)
	defer cancel()

	if !diagnoseJSON {
		printDiagnoseHeader("Runtime Diagnostics")
	}

	runtimeInstaller := installer.NewRuntimeInstaller()
	if runtimeInstaller == nil {
		return NewInstallerNotAvailableError("runtime")
	}

	supportedRuntimes := runtimeInstaller.GetSupportedRuntimes()
	results := []DiagnosticResult{}

	for _, runtimeName := range supportedRuntimes {
		result := DiagnosticResult{
			Name:      fmt.Sprintf("Runtime: %s", cases.Title(language.English).String(runtimeName)),
			Timestamp: time.Now(),
			Details:   make(map[string]interface{}),
		}

		verifyResult, err := runtimeInstaller.Verify(runtimeName)
		if err != nil {
			result.Status = StatusFailed
			result.Message = fmt.Sprintf("Failed to verify %s runtime: %v", runtimeName, err)
			result.Suggestions = []string{
				fmt.Sprintf("Check if %s is installed", runtimeName),
				fmt.Sprintf("Try: ./lsp-gateway install runtime %s", runtimeName),
			}
		} else {
			result.Details["installed"] = verifyResult.Installed
			result.Details["version"] = verifyResult.Version
			result.Details["compatible"] = verifyResult.Compatible
			result.Details["path"] = verifyResult.Path

			if !verifyResult.Installed {
				result.Status = StatusFailed
				result.Message = fmt.Sprintf("%s runtime not installed", cases.Title(language.English).String(runtimeName))
				result.Suggestions = []string{
					fmt.Sprintf("Install %s runtime", runtimeName),
					fmt.Sprintf("Use automatic installation: ./lsp-gateway install runtime %s", runtimeName),
				}
			} else if !verifyResult.Compatible {
				result.Status = StatusWarning
				result.Message = fmt.Sprintf("%s version %s does not meet requirements", cases.Title(language.English).String(runtimeName), verifyResult.Version)
				result.Suggestions = append(result.Suggestions, verifyResult.Recommendations...)
			} else {
				result.Status = StatusPassed
				result.Message = fmt.Sprintf("%s runtime %s is properly installed and compatible", cases.Title(language.English).String(runtimeName), verifyResult.Version)
			}

			if len(verifyResult.Issues) > 0 {
				if result.Status == StatusPassed {
					result.Status = StatusWarning
				}
				for _, issue := range verifyResult.Issues {
					result.Suggestions = append(result.Suggestions, "Resolve issue: "+issue.Description)
				}
			}
		}

		results = append(results, result)
	}

	if diagnoseJSON {
		report := &DiagnosticReport{
			Timestamp: time.Now(),
			Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
			Results:   results,
		}
		calculateSummary(report)
		return outputDiagnoseJSON(report)
	}

	return outputRuntimeDiagnosticsHuman(results)
}

func diagnoseServers(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), diagnoseTimeout)
	defer cancel()
	_ = ctx

	if !diagnoseJSON {
		printDiagnoseHeader("Language Server Diagnostics")
	}

	runtimeInstaller := installer.NewRuntimeInstaller()
	if runtimeInstaller == nil {
		return NewInstallerNotAvailableError("runtime")
	}

	serverInstaller := installer.NewServerInstaller(runtimeInstaller)
	if serverInstaller == nil {
		return NewInstallerNotAvailableError("server")
	}

	supportedServers := serverInstaller.GetSupportedServers()
	results := []DiagnosticResult{}

	for _, serverName := range supportedServers {
		result := DiagnosticResult{
			Name:      fmt.Sprintf("Server: %s", serverName),
			Timestamp: time.Now(),
			Details:   make(map[string]interface{}),
		}

		serverInfo, _ := serverInstaller.GetServerInfo(serverName)
		if serverInfo != nil {
			result.Details["display_name"] = serverInfo.DisplayName
			result.Details["runtime"] = serverInfo.Runtime
			result.Details["languages"] = serverInfo.Languages
		}

		verifyResult, err := serverInstaller.Verify(serverName)
		if err != nil {
			result.Status = StatusFailed
			result.Message = fmt.Sprintf("Failed to verify %s server: %v", serverName, err)
			result.Suggestions = []string{
				fmt.Sprintf("Check if %s is installed", serverName),
				fmt.Sprintf("Try: ./lsp-gateway install server %s", serverName),
			}
		} else {
			result.Details["installed"] = verifyResult.Installed
			result.Details["version"] = verifyResult.Version
			result.Details["compatible"] = verifyResult.Compatible
			result.Details["path"] = verifyResult.Path

			if !verifyResult.Installed {
				result.Status = StatusFailed
				result.Message = fmt.Sprintf("%s server not installed", serverName)
				result.Suggestions = []string{
					fmt.Sprintf("Install %s server", serverName),
					fmt.Sprintf("Use automatic installation: ./lsp-gateway install server %s", serverName),
				}
			} else if !verifyResult.Compatible {
				result.Status = StatusWarning
				result.Message = fmt.Sprintf("%s server installed but has compatibility issues", serverName)
				result.Suggestions = append(result.Suggestions, verifyResult.Recommendations...)
			} else {
				result.Status = StatusPassed
				result.Message = fmt.Sprintf("%s server is properly installed and functional", serverName)
			}

			if len(verifyResult.Issues) > 0 {
				if result.Status == StatusPassed {
					result.Status = StatusWarning
				}
				for _, issue := range verifyResult.Issues {
					result.Suggestions = append(result.Suggestions, "Resolve issue: "+issue.Description)
				}
			}
		}

		results = append(results, result)
	}

	if diagnoseJSON {
		report := &DiagnosticReport{
			Timestamp: time.Now(),
			Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
			Results:   results,
		}
		calculateSummary(report)
		return outputDiagnoseJSON(report)
	}

	return outputServerDiagnosticsHuman(results)
}

func diagnoseConfig(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), diagnoseTimeout)
	defer cancel()
	_ = ctx

	if !diagnoseJSON {
		printDiagnoseHeader("Configuration Diagnostics")
	}

	results := []DiagnosticResult{}

	configResult := DiagnosticResult{
		Name:      "Configuration File",
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		configResult.Status = StatusFailed
		configResult.Message = fmt.Sprintf("Failed to load configuration: %v", err)
		configResult.Suggestions = []string{
			"Generate a new configuration file: ./lsp-gateway config generate",
			"Check that config.yaml exists and has correct syntax",
			"Ensure the file has proper permissions",
		}
		configResult.Details["file_exists"] = false
		configResult.Details["syntax_valid"] = false
	} else {
		configResult.Details["file_exists"] = true
		configResult.Details["syntax_valid"] = true
		configResult.Details["port"] = cfg.Port
		configResult.Details["server_count"] = len(cfg.Servers)

		if validationErr := config.ValidateConfig(cfg); validationErr != nil {
			configResult.Status = StatusWarning
			configResult.Message = fmt.Sprintf("Configuration is valid but has issues: %v", validationErr)
			configResult.Suggestions = []string{
				"Fix configuration validation issues",
				"Review server configurations",
				"Regenerate config: ./lsp-gateway config generate",
			}
		} else {
			configResult.Status = StatusPassed
			configResult.Message = "Configuration file is valid and complete"
		}

		if len(cfg.Servers) == 0 {
			configResult.Status = StatusWarning
			configResult.Message = "Configuration file is valid but no servers are configured"
			configResult.Suggestions = append(configResult.Suggestions, "Add language server configurations")
		}
	}

	results = append(results, configResult)

	if diagnoseJSON {
		report := &DiagnosticReport{
			Timestamp: time.Now(),
			Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
			Results:   results,
		}
		calculateSummary(report)
		return outputDiagnoseJSON(report)
	}

	return outputConfigDiagnosticsHuman(results)
}

func runSystemDiagnostics(_ context.Context, report *DiagnosticReport) error {
	systemResult := DiagnosticResult{
		Name:      "System Environment",
		Status:    "passed",
		Message:   fmt.Sprintf("Successfully detected %s/%s environment", runtime.GOOS, runtime.GOARCH),
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"platform":     runtime.GOOS,
			"architecture": runtime.GOARCH,
			"go_version":   runtime.Version(),
		},
	}
	report.Results = append(report.Results, systemResult)

	binaryResult := DiagnosticResult{
		Name:      "LSP Gateway Binary",
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	if stat, err := os.Stat(os.Args[0]); err != nil {
		binaryResult.Status = StatusWarning
		binaryResult.Message = "Cannot access LSP Gateway binary"
		binaryResult.Suggestions = []string{
			"Ensure the binary has proper permissions",
			"Verify the binary path is correct",
		}
	} else {
		binaryResult.Status = StatusPassed
		binaryResult.Message = "LSP Gateway binary is accessible"
		binaryResult.Details["path"] = os.Args[0]
		binaryResult.Details["size"] = stat.Size()
		binaryResult.Details["mode"] = stat.Mode().String()
	}
	report.Results = append(report.Results, binaryResult)

	return nil
}

func runRuntimeDiagnostics(_ context.Context, report *DiagnosticReport) error {
	runtimeInstaller := installer.NewRuntimeInstaller()
	if runtimeInstaller == nil {
		report.Results = append(report.Results, DiagnosticResult{
			Name:      "Runtime Detection",
			Status:    "failed",
			Message:   "Failed to create runtime installer",
			Timestamp: time.Now(),
		})
		return nil
	}

	supportedRuntimes := runtimeInstaller.GetSupportedRuntimes()
	installedCount := 0
	compatibleCount := 0

	for _, name := range supportedRuntimes {
		result := DiagnosticResult{
			Name:      fmt.Sprintf("Runtime: %s", cases.Title(language.English).String(name)),
			Timestamp: time.Now(),
			Details:   make(map[string]interface{}),
		}

		verifyResult, err := runtimeInstaller.Verify(name)
		if err != nil {
			result.Status = StatusFailed
			result.Message = fmt.Sprintf("Failed to verify %s runtime: %v", name, err)
			result.Suggestions = []string{
				fmt.Sprintf("Check if %s is installed", name),
				fmt.Sprintf("Try: ./lsp-gateway install runtime %s", name),
			}
		} else {
			result.Details["installed"] = verifyResult.Installed
			result.Details["version"] = verifyResult.Version
			result.Details["compatible"] = verifyResult.Compatible
			result.Details["path"] = verifyResult.Path

			if verifyResult.Installed {
				installedCount++
			}
			if verifyResult.Compatible {
				compatibleCount++
			}

			if !verifyResult.Installed {
				result.Status = StatusFailed
				result.Message = fmt.Sprintf("%s runtime not installed", cases.Title(language.English).String(name))
				result.Suggestions = []string{
					fmt.Sprintf("Install %s runtime", name),
					fmt.Sprintf("Use automatic installation: ./lsp-gateway install runtime %s", name),
				}
			} else if !verifyResult.Compatible {
				result.Status = StatusWarning
				result.Message = fmt.Sprintf("%s version %s does not meet minimum requirements", cases.Title(language.English).String(name), verifyResult.Version)
				result.Suggestions = append(result.Suggestions, verifyResult.Recommendations...)
			} else {
				result.Status = StatusPassed
				result.Message = fmt.Sprintf("%s runtime %s is properly installed and compatible", cases.Title(language.English).String(name), verifyResult.Version)
			}

			if len(verifyResult.Issues) > 0 {
				if result.Status == StatusPassed {
					result.Status = StatusWarning
				}
				for _, issue := range verifyResult.Issues {
					result.Suggestions = append(result.Suggestions, "Resolve issue: "+issue.Description)
				}
			}
		}

		report.Results = append(report.Results, result)
	}

	if installedCount == 0 {
		report.Results = append(report.Results, DiagnosticResult{
			Name:      "Runtime Summary",
			Status:    "failed",
			Message:   "No runtimes are installed",
			Timestamp: time.Now(),
			Suggestions: []string{
				"Install at least one runtime (Go is recommended)",
				"Use: ./lsp-gateway install runtime go",
				"LSP Gateway requires runtimes to support language servers",
			},
		})
	} else if compatibleCount < installedCount {
		report.Results = append(report.Results, DiagnosticResult{
			Name:      "Runtime Summary",
			Status:    "warning",
			Message:   fmt.Sprintf("%d/%d runtimes meet compatibility requirements", compatibleCount, installedCount),
			Timestamp: time.Now(),
			Suggestions: []string{
				"Upgrade incompatible runtimes to supported versions",
				"Check minimum version requirements",
			},
		})
	} else {
		report.Results = append(report.Results, DiagnosticResult{
			Name:      "Runtime Summary",
			Status:    "passed",
			Message:   fmt.Sprintf("All %d installed runtimes are compatible", installedCount),
			Timestamp: time.Now(),
		})
	}

	return nil
}

func runServerDiagnostics(_ context.Context, report *DiagnosticReport) error {
	runtimeInstaller := installer.NewRuntimeInstaller()
	serverInstaller := installer.NewServerInstaller(runtimeInstaller)

	if serverInstaller == nil {
		report.Results = append(report.Results, DiagnosticResult{
			Name:      "Server Detection",
			Status:    "failed",
			Message:   "Failed to create server installer",
			Timestamp: time.Now(),
		})
		return nil
	}

	supportedServers := serverInstaller.GetSupportedServers()
	installedCount := 0
	workingCount := 0

	for _, serverName := range supportedServers {
		result := DiagnosticResult{
			Name:      fmt.Sprintf("Server: %s", serverName),
			Timestamp: time.Now(),
			Details:   make(map[string]interface{}),
		}

		verifyResult, err := serverInstaller.Verify(serverName)
		if err != nil {
			result.Status = StatusFailed
			result.Message = fmt.Sprintf("Failed to verify %s server: %v", serverName, err)
			result.Suggestions = []string{
				fmt.Sprintf("Try: ./lsp-gateway install server %s", serverName),
			}
		} else {
			result.Details["installed"] = verifyResult.Installed
			result.Details["version"] = verifyResult.Version
			result.Details["compatible"] = verifyResult.Compatible

			if verifyResult.Installed {
				installedCount++
			}
			if verifyResult.Compatible {
				workingCount++
			}

			if !verifyResult.Installed {
				result.Status = StatusFailed
				result.Message = fmt.Sprintf("%s server not installed", serverName)
				result.Suggestions = []string{
					fmt.Sprintf("Install %s server", serverName),
					fmt.Sprintf("Use: ./lsp-gateway install server %s", serverName),
				}
			} else if !verifyResult.Compatible {
				result.Status = StatusWarning
				result.Message = fmt.Sprintf("%s server installed but has compatibility issues", serverName)
				result.Suggestions = append(result.Suggestions, verifyResult.Recommendations...)
			} else {
				result.Status = StatusPassed
				result.Message = fmt.Sprintf("%s server is properly installed and functional", serverName)
			}
		}

		report.Results = append(report.Results, result)
	}

	if installedCount == 0 {
		report.Results = append(report.Results, DiagnosticResult{
			Name:      "Server Summary",
			Status:    "warning",
			Message:   "No language servers are installed",
			Timestamp: time.Now(),
			Suggestions: []string{
				"Install language servers for your development needs",
				"Use: ./lsp-gateway install server gopls (for Go)",
			},
		})
	} else {
		report.Results = append(report.Results, DiagnosticResult{
			Name:      "Server Summary",
			Status:    "passed",
			Message:   fmt.Sprintf("%d servers installed, %d working properly", installedCount, workingCount),
			Timestamp: time.Now(),
		})
	}

	return nil
}

func runConfigDiagnostics(ctx context.Context, report *DiagnosticReport) error {
	result := DiagnosticResult{
		Name:      "Configuration Check",
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		result.Status = StatusFailed
		result.Message = fmt.Sprintf("Failed to load configuration: %v", err)
		result.Suggestions = []string{
			"Generate a new configuration file: ./lsp-gateway config generate",
			"Check that config.yaml exists and has correct syntax",
		}
		result.Details["file_exists"] = false
	} else {
		result.Details["file_exists"] = true
		result.Details["port"] = cfg.Port
		result.Details["server_count"] = len(cfg.Servers)

		if validationErr := config.ValidateConfig(cfg); validationErr != nil {
			result.Status = StatusWarning
			result.Message = fmt.Sprintf("Configuration has validation issues: %v", validationErr)
			result.Suggestions = []string{
				"Fix configuration validation issues",
				"Regenerate config: ./lsp-gateway config generate",
			}
		} else {
			result.Status = StatusPassed
			result.Message = "Configuration is valid and complete"
		}

		if len(cfg.Servers) == 0 {
			result.Status = StatusWarning
			result.Message = "Configuration is valid but no servers are configured"
			result.Suggestions = append(result.Suggestions, "Add language server configurations")
		}
	}

	report.Results = append(report.Results, result)
	return nil
}

func calculateSummary(report *DiagnosticReport) {
	summary := DiagnosticSummary{
		TotalChecks: len(report.Results),
	}

	for _, result := range report.Results {
		switch result.Status {
		case StatusPassed:
			summary.Passed++
		case StatusFailed:
			summary.Failed++
		case StatusWarning:
			summary.Warnings++
		case "skipped":
			summary.Skipped++
		}
	}

	if summary.Failed > 0 {
		summary.OverallStatus = "failed"
	} else if summary.Warnings > 0 {
		summary.OverallStatus = StatusWarning
	} else if summary.Passed > 0 {
		summary.OverallStatus = "passed"
	} else {
		summary.OverallStatus = "unknown"
	}

	report.Summary = summary
}

func outputDiagnoseJSON(report *DiagnosticReport) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func outputDiagnoseHuman(report *DiagnosticReport) error {
	fmt.Printf("LSP Gateway Diagnostic Report\n")
	fmt.Printf("============================\n\n")

	fmt.Printf("System Information:\n")
	fmt.Printf("  Timestamp: %s\n", report.Timestamp.Format(time.RFC3339))
	fmt.Printf("  Platform: %s\n", report.Platform)
	fmt.Printf("  Go Version: %s\n", report.GoVersion)
	fmt.Printf("  LSP Gateway: %s\n", report.LSPGatewayPath)
	fmt.Printf("\n")

	fmt.Printf("Diagnostic Summary:\n")
	fmt.Printf("  Total Checks: %d\n", report.Summary.TotalChecks)
	fmt.Printf("  Passed: %s\n", getColoredCount(report.Summary.Passed))
	fmt.Printf("  Failed: %s\n", getColoredCount(report.Summary.Failed))
	fmt.Printf("  Warnings: %s\n", getColoredCount(report.Summary.Warnings))
	fmt.Printf("  Skipped: %s\n", getColoredCount(report.Summary.Skipped))
	fmt.Printf("  Overall Status: %s\n", getColoredStatus(report.Summary.OverallStatus))
	fmt.Printf("\n")

	fmt.Printf("Detailed Results:\n")
	outputDiagnosticTable(report.Results)

	allSuggestions := []string{}
	for _, result := range report.Results {
		if result.Status == "failed" || result.Status == "warning" {
			allSuggestions = append(allSuggestions, result.Suggestions...)
		}
	}

	if len(allSuggestions) > 0 {
		fmt.Printf("\nRecommendations:\n")
		for i, suggestion := range allSuggestions {
			fmt.Printf("  %d. %s\n", i+1, suggestion)
		}
	}

	if diagnoseVerbose {
		fmt.Printf("\nVerbose Details:\n")
		for _, result := range report.Results {
			if len(result.Details) > 0 {
				fmt.Printf("\n%s:\n", result.Name)
				for key, value := range result.Details {
					fmt.Printf("  %s: %v\n", key, value)
				}
			}
		}
	}

	return nil
}

func outputDiagnosticTable(results []DiagnosticResult) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if _, err := fmt.Fprintln(w, "CHECK\tSTATUS\tMESSAGE"); err != nil {
		return
	}
	if _, err := fmt.Fprintln(w, "-----\t------\t-------"); err != nil {
		return
	}

	for _, result := range results {
		status := getColoredStatus(result.Status)
		message := result.Message
		if len(message) > 60 {
			message = message[:57] + "..."
		}

		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", result.Name, status, message); err != nil {
			continue
		}
	}

	if err := w.Flush(); err != nil {
		// Ignore flush errors for diagnostic output
		_ = err
	}
}

func outputRuntimeDiagnosticsHuman(results []DiagnosticResult) error {
	fmt.Printf("Runtime Diagnostics Results\n")
	fmt.Printf("===========================\n\n")

	installedCount := 0
	compatibleCount := 0

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintf(w, "RUNTIME\tSTATUS\tVERSION\tPATH\n"); err != nil {
		return fmt.Errorf("failed to write table header: %w", err)
	}
	if _, err := fmt.Fprintf(w, "-------\t------\t-------\t----\n"); err != nil {
		return fmt.Errorf("failed to write table separator: %w", err)
	}

	for _, result := range results {
		if strings.HasPrefix(result.Name, "Runtime:") {
			runtimeName := strings.TrimPrefix(result.Name, "Runtime: ")
			status := getColoredStatus(result.Status)
			version := VALUE_N_A
			path := "N/A"

			if result.Details["version"] != nil {
				version = fmt.Sprintf("%v", result.Details["version"])
			}
			if result.Details["path"] != nil {
				path = fmt.Sprintf("%v", result.Details["path"])
			}

			if installed, ok := result.Details["installed"].(bool); ok && installed {
				installedCount++
			}
			if compatible, ok := result.Details["compatible"].(bool); ok && compatible {
				compatibleCount++
			}

			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", runtimeName, status, version, path); err != nil {
				return fmt.Errorf("failed to write table row: %w", err)
			}
		}
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush table output: %w", err)
	}

	fmt.Printf("\nSummary:\n")
	fmt.Printf("- Installed runtimes: %d\n", installedCount)
	fmt.Printf("- Compatible runtimes: %d\n", compatibleCount)

	return nil
}

func outputServerDiagnosticsHuman(results []DiagnosticResult) error {
	fmt.Printf("Language Server Diagnostics Results\n")
	fmt.Printf("===================================\n\n")

	installedCount := 0
	workingCount := 0

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "SERVER\tSTATUS\tVERSION\tPATH\n")
	_, _ = fmt.Fprintf(w, "------\t------\t-------\t----\n")

	for _, result := range results {
		if strings.HasPrefix(result.Name, "Server:") {
			serverName := strings.TrimPrefix(result.Name, "Server: ")
			status := getColoredStatus(result.Status)
			version := VALUE_N_A
			path := "N/A"

			if result.Details["version"] != nil {
				version = fmt.Sprintf("%v", result.Details["version"])
			}
			if result.Details["path"] != nil {
				path = fmt.Sprintf("%v", result.Details["path"])
			}

			if installed, ok := result.Details["installed"].(bool); ok && installed {
				installedCount++
			}
			if compatible, ok := result.Details["compatible"].(bool); ok && compatible {
				workingCount++
			}

			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", serverName, status, version, path); err != nil {
				return fmt.Errorf("failed to write server table row: %w", err)
			}
		}
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush server table output: %w", err)
	}

	fmt.Printf("\nSummary:\n")
	fmt.Printf("- Installed servers: %d\n", installedCount)
	fmt.Printf("- Working servers: %d\n", workingCount)

	return nil
}

func outputConfigDiagnosticsHuman(results []DiagnosticResult) error {
	fmt.Printf("Configuration Diagnostics Results\n")
	fmt.Printf("=================================\n\n")

	for _, result := range results {
		fmt.Printf("%s: %s\n", result.Name, getColoredStatus(result.Status))
		fmt.Printf("  Message: %s\n", result.Message)

		if len(result.Details) > 0 {
			fmt.Printf("  Details:\n")
			for key, value := range result.Details {
				fmt.Printf("    %s: %v\n", key, value)
			}
		}

		if len(result.Suggestions) > 0 {
			fmt.Printf("  Suggestions:\n")
			for _, suggestion := range result.Suggestions {
				fmt.Printf("    - %s\n", suggestion)
			}
		}
		fmt.Printf("\n")
	}

	return nil
}

func getColoredStatus(status string) string {
	switch status {
	case "passed":
		return "✓ PASSED"
	case StatusFailed:
		return "✗ FAILED"
	case "warning":
		return "⚠ WARNING"
	case "skipped":
		return "- SKIPPED"
	default:
		return strings.ToUpper(status)
	}
}

func getColoredCount(count int) string {
	if count == 0 {
		return "0"
	}
	return fmt.Sprintf("%d", count)
}

func printDiagnoseHeader(title string) {
	fmt.Printf("\n%s\n", title)
	fmt.Printf("%s\n\n", strings.Repeat("=", len(title)))
}

func validateDiagnoseParams() error {
	result := ValidateMultiple(
		func() *ValidationError {
			return ValidateTimeout(diagnoseTimeout, FLAG_TIMEOUT)
		},
	)
	if result == nil {
		return nil
	}
	return result
}

// Enhanced diagnostic command implementations

// New enhanced diagnostic command implementations

func diagnoseMultiLanguageConfiguration(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), diagnoseTimeout)
	defer cancel()

	if !diagnoseJSON {
		printDiagnoseHeader("Multi-Language Configuration Diagnostics")
	}

	report := &DiagnosticReport{
		Timestamp:      time.Now(),
		Platform:       fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		GoVersion:      runtime.Version(),
		LSPGatewayPath: os.Args[0],
		Results:        []DiagnosticResult{},
	}

	var errors []error

	// Force multi-language diagnostics
	diagnoseMultiLanguage = true

	// Run multi-language specific diagnostics
	if err := runMultiLanguageDiagnostics(ctx, report); err != nil {
		errors = append(errors, err)
	}

	// Also run consistency checking
	diagnoseCheckConsistency = true
	if err := runConfigDiagnostics(ctx, report); err != nil {
		errors = append(errors, err)
	}

	// Add template validation if enabled
	if diagnoseTemplateValidation {
		if err := runTemplateValidationDiagnostics(ctx, report); err != nil {
			errors = append(errors, err)
		}
	}

	calculateSummary(report)
	addEnhancedDiagnosticSummary(report)

	if diagnoseComprehensive {
		addComprehensiveDiagnosticScoring(report)
	}

	if diagnoseJSON {
		return outputDiagnoseJSON(report)
	}

	if err := outputDiagnoseHuman(report); err != nil {
		return err
	}

	if len(errors) > 0 {
		fmt.Printf("\nErrors encountered during multi-language diagnostics:\n")
		for _, err := range errors {
			fmt.Printf("  - %v\n", err)
		}
	}

	return nil
}

func diagnosePerformanceConfiguration(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), diagnoseTimeout)
	defer cancel()

	if !diagnoseJSON {
		printDiagnoseHeader("Performance Configuration Diagnostics")
	}

	report := &DiagnosticReport{
		Timestamp:      time.Now(),
		Platform:       fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		GoVersion:      runtime.Version(),
		LSPGatewayPath: os.Args[0],
		Results:        []DiagnosticResult{},
	}

	var errors []error

	// Force performance diagnostics
	diagnosePerformance = true
	diagnoseResourceLimits = true

	// Run performance specific diagnostics
	if err := runPerformanceDiagnostics(ctx, report); err != nil {
		errors = append(errors, err)
	}

	// Also run resource limit diagnostics
	if err := runResourceLimitDiagnostics(ctx, report); err != nil {
		errors = append(errors, err)
	}

	// Run optimization diagnostics
	if err := runOptimizationDiagnostics(ctx, report); err != nil {
		errors = append(errors, err)
	}

	calculateSummary(report)
	addEnhancedDiagnosticSummary(report)

	if diagnoseComprehensive {
		addComprehensiveDiagnosticScoring(report)
	}

	if diagnoseJSON {
		return outputDiagnoseJSON(report)
	}

	if err := outputDiagnoseHuman(report); err != nil {
		return err
	}

	if len(errors) > 0 {
		fmt.Printf("\nErrors encountered during performance diagnostics:\n")
		for _, err := range errors {
			fmt.Printf("  - %v\n", err)
		}
	}

	return nil
}

func diagnoseRoutingStrategy(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), diagnoseTimeout)
	defer cancel()

	if !diagnoseJSON {
		printDiagnoseHeader("Smart Routing Strategy Diagnostics")
	}

	report := &DiagnosticReport{
		Timestamp:      time.Now(),
		Platform:       fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		GoVersion:      runtime.Version(),
		LSPGatewayPath: os.Args[0],
		Results:        []DiagnosticResult{},
	}

	var errors []error

	// Force routing diagnostics
	diagnoseRouting = true
	diagnoseCheckConsistency = true

	// Run routing specific diagnostics
	if err := runRoutingDiagnostics(ctx, report); err != nil {
		errors = append(errors, err)
	}

	// Also run server diagnostics for routing validation
	if err := runServerDiagnostics(ctx, report); err != nil {
		errors = append(errors, err)
	}

	calculateSummary(report)
	addEnhancedDiagnosticSummary(report)

	if diagnoseComprehensive {
		addComprehensiveDiagnosticScoring(report)
	}

	if diagnoseJSON {
		return outputDiagnoseJSON(report)
	}

	if err := outputDiagnoseHuman(report); err != nil {
		return err
	}

	if len(errors) > 0 {
		fmt.Printf("\nErrors encountered during routing diagnostics:\n")
		for _, err := range errors {
			fmt.Printf("  - %v\n", err)
		}
	}

	return nil
}

// Template validation diagnostics
func runTemplateValidationDiagnostics(ctx context.Context, report *DiagnosticReport) error {
	result := DiagnosticResult{
		Name:      "Template Adherence Validation",
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Load current configuration
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		result.Status = "failed"
		result.Message = "Cannot load configuration for template validation"
		result.Suggestions = []string{
			"Ensure configuration file exists",
			"Generate configuration: lsp-gateway config generate",
		}
	} else {
		// Validate against built-in templates
		generator := config.NewConfigGenerator()
		templateIssues := []string{}
		templateSuggestions := []string{}

		// Check each server against its language template
		for _, server := range cfg.Servers {
			if len(server.Languages) > 0 {
				language := server.Languages[0]
				
				// Compare with template expectations
				switch language {
				case "go":
					if server.Command != "gopls" {
						templateIssues = append(templateIssues, fmt.Sprintf("Go server uses non-standard command: %s", server.Command))
						templateSuggestions = append(templateSuggestions, "Consider using 'gopls' for Go language server")
					}
				case "python":
					if server.Command != "python" || len(server.Args) == 0 || server.Args[0] != "-m" || server.Args[1] != "pylsp" {
						templateIssues = append(templateIssues, "Python server does not follow standard python-lsp-server template")
						templateSuggestions = append(templateSuggestions, "Use 'python -m pylsp' for Python language server")
					}
				case "typescript", "javascript":
					if server.Command != "typescript-language-server" {
						templateIssues = append(templateIssues, fmt.Sprintf("TypeScript server uses non-standard command: %s", server.Command))
						templateSuggestions = append(templateSuggestions, "Consider using 'typescript-language-server' for TypeScript/JavaScript")
					}
				case "java":
					if server.Command != "jdtls" {
						templateIssues = append(templateIssues, fmt.Sprintf("Java server uses non-standard command: %s", server.Command))
						templateSuggestions = append(templateSuggestions, "Consider using 'jdtls' for Java language server")
					}
				case "rust":
					if server.Command != "rust-analyzer" {
						templateIssues = append(templateIssues, fmt.Sprintf("Rust server uses non-standard command: %s", server.Command))
						templateSuggestions = append(templateSuggestions, "Consider using 'rust-analyzer' for Rust language server")
					}
				}
			}
		}

		result.Details["template_issues"] = templateIssues
		result.Details["servers_validated"] = len(cfg.Servers)
		result.Details["generator_available"] = generator != nil

		if len(templateIssues) > 0 {
			result.Status = "warning"
			result.Message = fmt.Sprintf("Found %d template adherence issues", len(templateIssues))
			result.Suggestions = templateSuggestions
		} else {
			result.Status = "passed"
			result.Message = "Configuration adheres to standard language server templates"
		}
	}

	report.Results = append(report.Results, result)
	return nil
}

func diagnoseOptimization(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), diagnoseTimeout)
	defer cancel()

	if !diagnoseJSON {
		printDiagnoseHeader("Optimization Diagnostics")
	}

	report := &DiagnosticReport{
		Timestamp:      time.Now(),
		Platform:       fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		GoVersion:      runtime.Version(),
		LSPGatewayPath: os.Args[0],
		Results:        []DiagnosticResult{},
	}

	var errors []error

	// Run optimization-specific diagnostics
	if err := runOptimizationDiagnostics(ctx, report); err != nil {
		errors = append(errors, err)
	}

	// Run performance diagnostics if enabled
	if diagnosePerformance {
		if err := runPerformanceDiagnostics(ctx, report); err != nil {
			errors = append(errors, err)
		}
	}

	// Run resource limit diagnostics if enabled  
	if diagnoseResourceLimits {
		if err := runResourceLimitDiagnostics(ctx, report); err != nil {
			errors = append(errors, err)
		}
	}

	// Run template validation if enabled
	if diagnoseTemplateValidation {
		if err := runTemplateValidationDiagnostics(ctx, report); err != nil {
			errors = append(errors, err)
		}
	}

	calculateSummary(report)
	addEnhancedDiagnosticSummary(report)

	if diagnoseJSON {
		return outputDiagnoseJSON(report)
	}

	if err := outputDiagnoseHuman(report); err != nil {
		return err
	}

	if len(errors) > 0 {
		fmt.Printf("\nErrors encountered during optimization diagnostics:\n")
		for _, err := range errors {
			fmt.Printf("  - %v\n", err)
		}
	}

	return nil
}

// Enhanced diagnostic helper functions

func runMultiLanguageDiagnostics(ctx context.Context, report *DiagnosticReport) error {
	result := DiagnosticResult{
		Name:      "Multi-language Configuration",
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Check if project path is provided for multi-language diagnostics
	if diagnoseProjectPath == "" {
		// Use current directory as fallback
		diagnoseProjectPath = "."
	}

	result.Details["project_path"] = diagnoseProjectPath

	// Attempt to auto-generate multi-language config for analysis
	multiLangConfig, err := config.AutoGenerateConfigFromPath(diagnoseProjectPath)
	if err != nil {
		result.Status = "failed"
		result.Message = fmt.Sprintf("Failed to analyze multi-language project: %v", err)
		result.Suggestions = []string{
			"Ensure project path contains supported language files",
			"Check directory permissions and accessibility",
			"Verify project structure contains recognizable language patterns",
		}
	} else {
		// Validate multi-language configuration
		if validationErr := multiLangConfig.Validate(); validationErr != nil {
			result.Status = "warning"
			result.Message = fmt.Sprintf("Multi-language configuration has issues: %v", validationErr)
			result.Suggestions = []string{
				"Review detected language configurations",
				"Check project structure consistency",
				"Verify language server compatibility",
			}
		} else {
			result.Status = "passed"
			result.Message = "Multi-language configuration is valid and consistent"
		}

		// Add detailed analysis
		result.Details["languages_detected"] = multiLangConfig.GetSupportedLanguages()
		result.Details["project_type"] = multiLangConfig.ProjectInfo.ProjectType
		result.Details["server_count"] = len(multiLangConfig.ServerConfigs)
		result.Details["consistency_check"] = diagnoseCheckConsistency
		result.Details["generated_at"] = multiLangConfig.GeneratedAt

		// Check language coverage and conflicts
		languageCoverage := make(map[string]int)
		for _, serverConfig := range multiLangConfig.ServerConfigs {
			for _, lang := range serverConfig.Languages {
				languageCoverage[lang]++
			}
		}
		result.Details["language_coverage"] = languageCoverage

		// Check for language conflicts
		conflicts := 0
		for _, count := range languageCoverage {
			if count > 1 {
				conflicts++
			}
		}
		if conflicts > 0 {
			if result.Status == "passed" {
				result.Status = "warning"
			}
			result.Message += fmt.Sprintf("; found %d language routing conflicts", conflicts)
			result.Suggestions = append(result.Suggestions, "Consider smart routing to resolve language conflicts")
		}

		// Perform consistency checking if enabled
		if diagnoseCheckConsistency {
			gwConfig, convertErr := multiLangConfig.ToGatewayConfig()
			if convertErr == nil {
				if consistencyErr := gwConfig.ValidateConsistency(); consistencyErr != nil {
					if result.Status == "passed" {
						result.Status = "warning"
					}
					result.Message += fmt.Sprintf("; consistency issues: %v", consistencyErr)
					result.Suggestions = append(result.Suggestions, "Review configuration consistency across languages")
				}
				result.Details["consistency_validated"] = true
			} else {
				result.Details["consistency_validated"] = false
				result.Details["consistency_error"] = convertErr.Error()
			}
		}
	}

	report.Results = append(report.Results, result)
	return nil
}

func runPerformanceDiagnostics(ctx context.Context, report *DiagnosticReport) error {
	result := DiagnosticResult{
		Name:      "Performance Configuration",
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	result.Details["performance_mode"] = diagnosePerformanceMode

	// Load and analyze configuration for performance
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		result.Status = "failed"
		result.Message = "Cannot load configuration for performance analysis"
		result.Suggestions = []string{
			"Ensure configuration file exists",
			"Generate configuration: lsp-gateway config generate",
		}
	} else {
		// Enhanced performance analysis with optimization manager
		optManager := config.NewOptimizationManager()
		strategy, strategyErr := optManager.GetStrategy(diagnosePerformanceMode)

		result.Details["max_concurrent_requests"] = cfg.MaxConcurrentRequests
		result.Details["timeout"] = cfg.Timeout
		result.Details["server_count"] = len(cfg.Servers)
		result.Details["project_aware"] = cfg.ProjectAware
		result.Details["enable_concurrent_servers"] = cfg.EnableConcurrentServers

		if strategyErr == nil {
			result.Details["optimization_strategy"] = strategy.GetOptimizationName()
			result.Details["performance_settings"] = strategy.GetPerformanceSettings()
			result.Details["memory_settings"] = strategy.GetMemorySettings()
			result.Details["resource_limits"] = strategy.GetResourceLimits()
		}

		// Analyze performance settings against current mode
		performanceIssues := []string{}
		performanceSuggestions := []string{}

		if cfg.MaxConcurrentRequests <= 0 {
			performanceIssues = append(performanceIssues, "Invalid MaxConcurrentRequests setting")
			performanceSuggestions = append(performanceSuggestions, "Set MaxConcurrentRequests to a positive value")
		} else {
			// Mode-specific performance validation
			switch diagnosePerformanceMode {
			case "production":
				if cfg.MaxConcurrentRequests < 100 {
					performanceIssues = append(performanceIssues, "MaxConcurrentRequests too low for production")
					performanceSuggestions = append(performanceSuggestions, "Increase MaxConcurrentRequests to 100-200 for production")
				}
				if cfg.MaxConcurrentRequests > 500 {
					performanceIssues = append(performanceIssues, "MaxConcurrentRequests may be too high for production stability")
					performanceSuggestions = append(performanceSuggestions, "Consider reducing MaxConcurrentRequests for production stability")
				}
			case "development":
				if cfg.MaxConcurrentRequests > 100 {
					performanceIssues = append(performanceIssues, "MaxConcurrentRequests may be too high for development")
					performanceSuggestions = append(performanceSuggestions, "Consider reducing MaxConcurrentRequests to 50-100 for development")
				}
			case "analysis":
				if cfg.MaxConcurrentRequests > 50 {
					performanceIssues = append(performanceIssues, "MaxConcurrentRequests may be too high for analysis workloads")
					performanceSuggestions = append(performanceSuggestions, "Consider reducing MaxConcurrentRequests to 25-50 for analysis mode")
				}
				if cfg.Timeout == "30s" {
					performanceIssues = append(performanceIssues, "Timeout may be too short for analysis workloads")
					performanceSuggestions = append(performanceSuggestions, "Consider increasing timeout to 60-120s for analysis mode")
				}
			}
		}

		// Validate resource limits for multi-server configurations
		if cfg.EnableConcurrentServers {
			if cfg.MaxConcurrentServersPerLanguage <= 0 {
				performanceIssues = append(performanceIssues, "MaxConcurrentServersPerLanguage not set for concurrent server mode")
				performanceSuggestions = append(performanceSuggestions, "Set MaxConcurrentServersPerLanguage to appropriate value (2-5 recommended)")
			} else if cfg.MaxConcurrentServersPerLanguage > 5 {
				performanceIssues = append(performanceIssues, "MaxConcurrentServersPerLanguage too high, may cause resource contention")
				performanceSuggestions = append(performanceSuggestions, "Consider reducing MaxConcurrentServersPerLanguage to 2-5")
			}
		}

		// Check language pool performance settings
		if len(cfg.LanguagePools) > 0 {
			for _, pool := range cfg.LanguagePools {
				if pool.ResourceLimits != nil {
					if err := pool.ResourceLimits.Validate(); err != nil {
						performanceIssues = append(performanceIssues, fmt.Sprintf("Invalid resource limits for %s pool: %v", pool.Language, err))
						performanceSuggestions = append(performanceSuggestions, fmt.Sprintf("Review resource limits for %s language pool", pool.Language))
					}
				}
			}
		}

		result.Details["performance_issues"] = performanceIssues
		result.Details["current_timeout"] = cfg.Timeout
		result.Details["current_max_concurrent"] = cfg.MaxConcurrentRequests

		if len(performanceIssues) > 0 {
			result.Status = "warning"
			result.Message = fmt.Sprintf("Found %d performance optimization opportunities", len(performanceIssues))
			result.Suggestions = performanceSuggestions
		} else {
			result.Status = "passed"
			result.Message = fmt.Sprintf("Performance configuration is well-optimized for %s mode", diagnosePerformanceMode)
		}
	}

	report.Results = append(report.Results, result)
	return nil
}

func runRoutingDiagnostics(ctx context.Context, report *DiagnosticReport) error {
	result := DiagnosticResult{
		Name:      "Smart Routing Configuration",
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Load configuration and analyze routing
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		result.Status = "failed"
		result.Message = "Cannot load configuration for routing analysis"
		result.Suggestions = []string{
			"Ensure configuration file exists",
			"Generate configuration: lsp-gateway config generate",
		}
	} else {
		// Enhanced routing analysis
		routingIssues := []string{}
		routingSuggestions := []string{}

		// Analyze traditional server configuration
		languageCoverage := make(map[string][]string)
		serversByLanguage := make(map[string][]*config.ServerConfig)
		for i, server := range cfg.Servers {
			for _, lang := range server.Languages {
				languageCoverage[lang] = append(languageCoverage[lang], server.Name)
				serversByLanguage[lang] = append(serversByLanguage[lang], &cfg.Servers[i])
			}
		}

		result.Details["language_coverage"] = languageCoverage
		result.Details["total_servers"] = len(cfg.Servers)
		result.Details["enable_concurrent_servers"] = cfg.EnableConcurrentServers
		result.Details["max_concurrent_servers_per_language"] = cfg.MaxConcurrentServersPerLanguage

		// Check for routing conflicts in traditional configuration
		traditionalConflicts := 0
		for lang, servers := range languageCoverage {
			if len(servers) > 1 {
				traditionalConflicts++
				if !cfg.EnableConcurrentServers {
					routingIssues = append(routingIssues, fmt.Sprintf("Language %s has multiple servers but concurrent servers not enabled", lang))
					routingSuggestions = append(routingSuggestions, "Enable concurrent servers or remove duplicate language assignments")
				}
			}
		}

		// Analyze language pool configuration (enhanced multi-server)
		poolConflicts := 0
		poolCoverage := make(map[string]string)
		if len(cfg.LanguagePools) > 0 {
			for _, pool := range cfg.LanguagePools {
				if existingPool, exists := poolCoverage[pool.Language]; exists {
					poolConflicts++
					routingIssues = append(routingIssues, fmt.Sprintf("Duplicate language pool for %s (conflicts with %s)", pool.Language, existingPool))
				} else {
					poolCoverage[pool.Language] = pool.Language
				}

				// Validate pool's load balancing configuration
				if pool.LoadBalancingConfig != nil {
					if err := pool.LoadBalancingConfig.Validate(); err != nil {
						routingIssues = append(routingIssues, fmt.Sprintf("Invalid load balancing config for %s pool: %v", pool.Language, err))
						routingSuggestions = append(routingSuggestions, fmt.Sprintf("Fix load balancing configuration for %s language pool", pool.Language))
					}
				}

				// Check server transport compatibility within pools
				if err := pool.ValidateServerTransportCompatibility(); err != nil {
					routingIssues = append(routingIssues, fmt.Sprintf("Transport compatibility issue in %s pool: %v", pool.Language, err))
					routingSuggestions = append(routingSuggestions, fmt.Sprintf("Review transport configuration for %s language pool", pool.Language))
				}
			}

			result.Details["language_pools"] = len(cfg.LanguagePools)
			result.Details["pool_coverage"] = poolCoverage
			result.Details["pool_conflicts"] = poolConflicts
		}

		// Check for conflicts between traditional servers and language pools
		crossConfigConflicts := 0
		if len(cfg.Servers) > 0 && len(cfg.LanguagePools) > 0 {
			for lang := range languageCoverage {
				if _, existsInPools := poolCoverage[lang]; existsInPools {
					crossConfigConflicts++
					routingIssues = append(routingIssues, fmt.Sprintf("Language %s configured in both servers and language pools", lang))
				}
			}

			if crossConfigConflicts > 0 {
				routingSuggestions = append(routingSuggestions, "Use either traditional servers OR language pools, not both")
				routingSuggestions = append(routingSuggestions, "Migrate to language pools for enhanced multi-server capabilities")
			}
		}

		// Analyze global multi-server configuration
		if cfg.GlobalMultiServerConfig != nil {
			if err := cfg.GlobalMultiServerConfig.Validate(); err != nil {
				routingIssues = append(routingIssues, fmt.Sprintf("Invalid global multi-server config: %v", err))
				routingSuggestions = append(routingSuggestions, "Review global multi-server configuration settings")
			}

			// Check for circular dependencies
			if err := cfg.GlobalMultiServerConfig.ValidateCircularDependencies(); err != nil {
				routingIssues = append(routingIssues, fmt.Sprintf("Circular dependency in multi-server config: %v", err))
				routingSuggestions = append(routingSuggestions, "Remove circular references in server configurations")
			}

			result.Details["global_multi_server_config"] = map[string]interface{}{
				"selection_strategy": cfg.GlobalMultiServerConfig.SelectionStrategy,
				"concurrent_limit":   cfg.GlobalMultiServerConfig.ConcurrentLimit,
				"health_check_interval": cfg.GlobalMultiServerConfig.HealthCheckInterval,
				"max_retries":        cfg.GlobalMultiServerConfig.MaxRetries,
			}
		}

		// Check consistency across all routing configurations
		if diagnoseCheckConsistency {
			if err := cfg.ValidateConsistency(); err != nil {
				routingIssues = append(routingIssues, fmt.Sprintf("Configuration consistency issue: %v", err))
				routingSuggestions = append(routingSuggestions, "Review and fix configuration consistency issues")
			}
		}

		result.Details["routing_issues"] = routingIssues
		result.Details["traditional_conflicts"] = traditionalConflicts
		result.Details["cross_config_conflicts"] = crossConfigConflicts

		totalIssues := len(routingIssues)
		if totalIssues > 0 {
			result.Status = "warning"
			result.Message = fmt.Sprintf("Found %d routing configuration issues", totalIssues)
			result.Suggestions = routingSuggestions
		} else {
			result.Status = "passed"
			result.Message = "Smart routing configuration is optimal"
			if cfg.EnableConcurrentServers {
				result.Message += " with concurrent server support"
			}
		}
	}

	report.Results = append(report.Results, result)
	return nil
}

func runResourceLimitDiagnostics(ctx context.Context, report *DiagnosticReport) error {
	result := DiagnosticResult{
		Name:      "Resource Limits",
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Load configuration and analyze resource requirements
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		result.Status = "failed"
		result.Message = "Cannot load configuration for resource analysis"
	} else {
		// Estimate resource requirements
		estimatedMemoryMB := len(cfg.Servers) * 50 // 50MB per server estimate
		if cfg.MaxConcurrentRequests > 0 {
			estimatedMemoryMB += cfg.MaxConcurrentRequests * 2 // 2MB per request
		}

		result.Details["estimated_memory_mb"] = estimatedMemoryMB
		result.Details["server_count"] = len(cfg.Servers)
		result.Details["max_concurrent_requests"] = cfg.MaxConcurrentRequests

		if estimatedMemoryMB > 2000 {
			result.Status = "warning"
			result.Message = fmt.Sprintf("High estimated memory usage: %dMB", estimatedMemoryMB)
			result.Suggestions = []string{
				"Ensure system has sufficient memory",
				"Consider reducing MaxConcurrentRequests or server count",
				"Monitor actual memory usage in production",
			}
		} else {
			result.Status = "passed"
			result.Message = fmt.Sprintf("Resource requirements look reasonable: ~%dMB", estimatedMemoryMB)
		}
	}

	report.Results = append(report.Results, result)
	return nil
}

func runOptimizationDiagnostics(ctx context.Context, report *DiagnosticReport) error {
	result := DiagnosticResult{
		Name:      "Optimization Analysis",
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	result.Details["performance_mode"] = diagnosePerformanceMode

	// Load configuration and analyze optimization potential
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		result.Status = "failed"
		result.Message = "Cannot load configuration for optimization analysis"
	} else {
		// Analyze current settings vs optimal settings for the performance mode
		optimizationIssues := []string{}
		optimizationSuggestions := []string{}

		// Check optimization based on performance mode
		switch diagnosePerformanceMode {
		case "production":
			if cfg.MaxConcurrentRequests < 100 {
				optimizationIssues = append(optimizationIssues, "MaxConcurrentRequests too low for production")
				optimizationSuggestions = append(optimizationSuggestions, "Increase MaxConcurrentRequests to 100-200 for production")
			}
		case "development":
			if cfg.MaxConcurrentRequests > 100 {
				optimizationIssues = append(optimizationIssues, "MaxConcurrentRequests may be too high for development")
				optimizationSuggestions = append(optimizationSuggestions, "Consider reducing MaxConcurrentRequests to 50-100 for development")
			}
		case "analysis":
			// Analysis mode might need different timeout settings
			if cfg.Timeout == "30s" {
				optimizationIssues = append(optimizationIssues, "Timeout may be too short for analysis workloads")
				optimizationSuggestions = append(optimizationSuggestions, "Consider increasing timeout to 60-120s for analysis mode")
			}
		}

		result.Details["optimization_issues"] = optimizationIssues
		result.Details["current_max_concurrent"] = cfg.MaxConcurrentRequests
		result.Details["current_timeout"] = cfg.Timeout

		if len(optimizationIssues) > 0 {
			result.Status = "warning"
			result.Message = fmt.Sprintf("Found %d optimization opportunities", len(optimizationIssues))
			result.Suggestions = optimizationSuggestions
		} else {
			result.Status = "passed"
			result.Message = fmt.Sprintf("Configuration is well-optimized for %s mode", diagnosePerformanceMode)
		}
	}

	report.Results = append(report.Results, result)
	return nil
}

func addEnhancedDiagnosticSummary(report *DiagnosticReport) {
	// Add summary information about enhanced diagnostics
	enhancedResult := DiagnosticResult{
		Name:      "Enhanced Diagnostics Summary",
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	enhancedFeatures := []string{}
	if diagnoseMultiLanguage {
		enhancedFeatures = append(enhancedFeatures, "multi-language")
	}
	if diagnosePerformance {
		enhancedFeatures = append(enhancedFeatures, "performance")
	}
	if diagnoseRouting {
		enhancedFeatures = append(enhancedFeatures, "routing")
	}
	if diagnoseResourceLimits {
		enhancedFeatures = append(enhancedFeatures, "resource-limits")
	}
	if diagnoseOptimization {
		enhancedFeatures = append(enhancedFeatures, "optimization")
	}
	if diagnoseComprehensive {
		enhancedFeatures = append(enhancedFeatures, "comprehensive-scoring")
	}
	if diagnoseTemplateValidation {
		enhancedFeatures = append(enhancedFeatures, "template-validation")
	}

	enhancedResult.Details["enhanced_features_enabled"] = enhancedFeatures
	enhancedResult.Details["project_path"] = diagnoseProjectPath
	enhancedResult.Details["performance_mode"] = diagnosePerformanceMode
	enhancedResult.Details["consistency_checking"] = diagnoseCheckConsistency
	enhancedResult.Details["comprehensive_mode"] = diagnoseComprehensive
	enhancedResult.Status = "passed"
	enhancedResult.Message = fmt.Sprintf("Enhanced diagnostics completed with %d additional features", len(enhancedFeatures))

	report.Results = append(report.Results, enhancedResult)
}

// Add comprehensive diagnostic scoring
func addComprehensiveDiagnosticScoring(report *DiagnosticReport) {
	scoringResult := DiagnosticResult{
		Name:      "Comprehensive Diagnostic Scoring",
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Calculate individual scores
	scoring := DiagnosticScoring{}
	totalResults := len(report.Results)
	passedResults := report.Summary.Passed
	failedResults := report.Summary.Failed
	warningResults := report.Summary.Warnings

	if totalResults > 0 {
		// Configuration health (0-100)
		scoring.ConfigurationHealth = float64(passedResults) / float64(totalResults) * 100
		if failedResults > 0 {
			scoring.ConfigurationHealth -= float64(failedResults) * 10 // Penalty for failures
		}
		if warningResults > 0 {
			scoring.ConfigurationHealth -= float64(warningResults) * 5 // Penalty for warnings
		}

		// Performance rating based on optimization and resource efficiency
		scoring.PerformanceRating = scoring.ConfigurationHealth
		if diagnosePerformance {
			// Boost score if performance diagnostics were run and passed
			for _, result := range report.Results {
				if result.Name == "Performance Configuration" && result.Status == "passed" {
					scoring.PerformanceRating += 10
				}
			}
		}

		// Consistency score based on multi-language and routing validation
		scoring.ConsistencyScore = scoring.ConfigurationHealth
		if diagnoseCheckConsistency {
			for _, result := range report.Results {
				if strings.Contains(result.Name, "Multi-language") && result.Status == "passed" {
					scoring.ConsistencyScore += 15
				}
				if strings.Contains(result.Name, "Routing") && result.Status == "passed" {
					scoring.ConsistencyScore += 10
				}
			}
		}

		// Optimization level based on current mode and settings
		scoring.OptimizationLevel = 50.0 // Base level
		switch diagnosePerformanceMode {
		case "production":
			scoring.OptimizationLevel = 85.0
		case "analysis":
			scoring.OptimizationLevel = 95.0
		case "development":
			scoring.OptimizationLevel = 70.0
		}

		// Multi-language health
		scoring.MultiLanguageHealth = scoring.ConfigurationHealth
		if diagnoseMultiLanguage {
			for _, result := range report.Results {
				if result.Name == "Multi-language Configuration" {
					if result.Status == "passed" {
						scoring.MultiLanguageHealth += 20
					} else if result.Status == "warning" {
						scoring.MultiLanguageHealth += 5
					}
				}
			}
		}

		// Routing efficiency
		scoring.RoutingEfficiency = scoring.ConfigurationHealth
		if diagnoseRouting {
			for _, result := range report.Results {
				if result.Name == "Smart Routing Configuration" {
					if result.Status == "passed" {
						scoring.RoutingEfficiency += 25
					} else if result.Status == "warning" {
						scoring.RoutingEfficiency += 10
					}
				}
			}
		}

		// Ensure scores don't exceed 100
		if scoring.ConfigurationHealth > 100 {
			scoring.ConfigurationHealth = 100
		}
		if scoring.PerformanceRating > 100 {
			scoring.PerformanceRating = 100
		}
		if scoring.ConsistencyScore > 100 {
			scoring.ConsistencyScore = 100
		}
		if scoring.OptimizationLevel > 100 {
			scoring.OptimizationLevel = 100
		}
		if scoring.MultiLanguageHealth > 100 {
			scoring.MultiLanguageHealth = 100
		}
		if scoring.RoutingEfficiency > 100 {
			scoring.RoutingEfficiency = 100
		}

		// Calculate overall score as weighted average
		overallScore := (scoring.ConfigurationHealth*0.3 + scoring.PerformanceRating*0.2 + 
			scoring.ConsistencyScore*0.2 + scoring.OptimizationLevel*0.15 + 
			scoring.MultiLanguageHealth*0.1 + scoring.RoutingEfficiency*0.05)

		// Determine grade
		var grade string
		switch {
		case overallScore >= 90:
			grade = "A"
		case overallScore >= 80:
			grade = "B"
		case overallScore >= 70:
			grade = "C"
		case overallScore >= 60:
			grade = "D"
		default:
			grade = "F"
		}

		// Update report summary with scoring
		report.Summary.Score = overallScore
		report.Summary.Grade = grade

		// Add scoring details
		scoringResult.Details["scoring"] = scoring
		scoringResult.Details["overall_score"] = overallScore
		scoringResult.Details["grade"] = grade
		scoringResult.Details["total_results"] = totalResults
		scoringResult.Details["passed_results"] = passedResults
		scoringResult.Details["failed_results"] = failedResults
		scoringResult.Details["warning_results"] = warningResults

		if overallScore >= 80 {
			scoringResult.Status = "passed"
			scoringResult.Message = fmt.Sprintf("Excellent configuration health with score %.1f (%s grade)", overallScore, grade)
		} else if overallScore >= 60 {
			scoringResult.Status = "warning"
			scoringResult.Message = fmt.Sprintf("Configuration needs improvement with score %.1f (%s grade)", overallScore, grade)
			scoringResult.Suggestions = []string{
				"Review failed and warning diagnostic results",
				"Consider running optimization diagnostics",
				"Enable comprehensive consistency checking",
			}
		} else {
			scoringResult.Status = "failed"
			scoringResult.Message = fmt.Sprintf("Configuration requires immediate attention with score %.1f (%s grade)", overallScore, grade)
			scoringResult.Suggestions = []string{
				"Address all failed diagnostic results immediately",
				"Run comprehensive diagnostics to identify issues",
				"Consider regenerating configuration from scratch",
				"Review project structure and language server setup",
			}
		}
	}

	report.Results = append(report.Results, scoringResult)
}
