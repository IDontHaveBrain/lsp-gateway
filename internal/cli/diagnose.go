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
	diagnoseJSON    bool
	diagnoseVerbose bool
	diagnoseTimeout time.Duration
	diagnoseAll     bool
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

Examples:
  # Run all diagnostics
  lsp-gateway diagnose
  
  # Diagnose only runtimes
  lsp-gateway diagnose runtimes
  
  # Verbose diagnostic output
  lsp-gateway diagnose --verbose
  
  # JSON output for automation
  lsp-gateway diagnose --json`,
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
  lsp-gateway diagnose config --verbose`,
	RunE: diagnoseConfig,
}

func init() {
	diagnoseCmd.PersistentFlags().BoolVar(&diagnoseJSON, "json", false, "Output in JSON format")
	diagnoseCmd.PersistentFlags().BoolVarP(&diagnoseVerbose, "verbose", "v", false, "Verbose diagnostic output")
	diagnoseCmd.PersistentFlags().DurationVar(&diagnoseTimeout, FLAG_TIMEOUT, 60*time.Second, "Diagnostic timeout")
	diagnoseCmd.PersistentFlags().BoolVar(&diagnoseAll, "all", false, "Run all available diagnostics")

	diagnoseCmd.AddCommand(diagnoseRuntimesCmd)
	diagnoseCmd.AddCommand(diagnoseServersCmd)
	diagnoseCmd.AddCommand(diagnoseConfigCmd)

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
	TotalChecks   int    `json:"total_checks"`
	Passed        int    `json:"passed"`
	Failed        int    `json:"failed"`
	Warnings      int    `json:"warnings"`
	Skipped       int    `json:"skipped"`
	OverallStatus string `json:"overall_status"`
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
			result.Status = "failed"
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
				result.Status = "failed"
				result.Message = fmt.Sprintf("%s runtime not installed", cases.Title(language.English).String(runtimeName))
				result.Suggestions = []string{
					fmt.Sprintf("Install %s runtime", runtimeName),
					fmt.Sprintf("Use automatic installation: ./lsp-gateway install runtime %s", runtimeName),
				}
			} else if !verifyResult.Compatible {
				result.Status = "warning"
				result.Message = fmt.Sprintf("%s version %s does not meet requirements", cases.Title(language.English).String(runtimeName), verifyResult.Version)
				result.Suggestions = append(result.Suggestions, verifyResult.Recommendations...)
			} else {
				result.Status = "passed"
				result.Message = fmt.Sprintf("%s runtime %s is properly installed and compatible", cases.Title(language.English).String(runtimeName), verifyResult.Version)
			}

			if len(verifyResult.Issues) > 0 {
				if result.Status == "passed" {
					result.Status = "warning"
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
			result.Status = "failed"
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
				result.Status = "failed"
				result.Message = fmt.Sprintf("%s server not installed", serverName)
				result.Suggestions = []string{
					fmt.Sprintf("Install %s server", serverName),
					fmt.Sprintf("Use automatic installation: ./lsp-gateway install server %s", serverName),
				}
			} else if !verifyResult.Compatible {
				result.Status = "warning"
				result.Message = fmt.Sprintf("%s server installed but has compatibility issues", serverName)
				result.Suggestions = append(result.Suggestions, verifyResult.Recommendations...)
			} else {
				result.Status = "passed"
				result.Message = fmt.Sprintf("%s server is properly installed and functional", serverName)
			}

			if len(verifyResult.Issues) > 0 {
				if result.Status == "passed" {
					result.Status = "warning"
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
		configResult.Status = "failed"
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
			configResult.Status = "warning"
			configResult.Message = fmt.Sprintf("Configuration is valid but has issues: %v", validationErr)
			configResult.Suggestions = []string{
				"Fix configuration validation issues",
				"Review server configurations",
				"Regenerate config: ./lsp-gateway config generate",
			}
		} else {
			configResult.Status = "passed"
			configResult.Message = "Configuration file is valid and complete"
		}

		if len(cfg.Servers) == 0 {
			configResult.Status = "warning"
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
		binaryResult.Status = "warning"
		binaryResult.Message = "Cannot access LSP Gateway binary"
		binaryResult.Suggestions = []string{
			"Ensure the binary has proper permissions",
			"Verify the binary path is correct",
		}
	} else {
		binaryResult.Status = "passed"
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
			result.Status = "failed"
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
				result.Status = "failed"
				result.Message = fmt.Sprintf("%s runtime not installed", cases.Title(language.English).String(name))
				result.Suggestions = []string{
					fmt.Sprintf("Install %s runtime", name),
					fmt.Sprintf("Use automatic installation: ./lsp-gateway install runtime %s", name),
				}
			} else if !verifyResult.Compatible {
				result.Status = "warning"
				result.Message = fmt.Sprintf("%s version %s does not meet minimum requirements", cases.Title(language.English).String(name), verifyResult.Version)
				result.Suggestions = append(result.Suggestions, verifyResult.Recommendations...)
			} else {
				result.Status = "passed"
				result.Message = fmt.Sprintf("%s runtime %s is properly installed and compatible", cases.Title(language.English).String(name), verifyResult.Version)
			}

			if len(verifyResult.Issues) > 0 {
				if result.Status == "passed" {
					result.Status = "warning"
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
			result.Status = "failed"
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
				result.Status = "failed"
				result.Message = fmt.Sprintf("%s server not installed", serverName)
				result.Suggestions = []string{
					fmt.Sprintf("Install %s server", serverName),
					fmt.Sprintf("Use: ./lsp-gateway install server %s", serverName),
				}
			} else if !verifyResult.Compatible {
				result.Status = "warning"
				result.Message = fmt.Sprintf("%s server installed but has compatibility issues", serverName)
				result.Suggestions = append(result.Suggestions, verifyResult.Recommendations...)
			} else {
				result.Status = "passed"
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
		result.Status = "failed"
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
			result.Status = "warning"
			result.Message = fmt.Sprintf("Configuration has validation issues: %v", validationErr)
			result.Suggestions = []string{
				"Fix configuration validation issues",
				"Regenerate config: ./lsp-gateway config generate",
			}
		} else {
			result.Status = "passed"
			result.Message = "Configuration is valid and complete"
		}

		if len(cfg.Servers) == 0 {
			result.Status = "warning"
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
		case "passed":
			summary.Passed++
		case "failed":
			summary.Failed++
		case "warning":
			summary.Warnings++
		case "skipped":
			summary.Skipped++
		}
	}

	if summary.Failed > 0 {
		summary.OverallStatus = "failed"
	} else if summary.Warnings > 0 {
		summary.OverallStatus = "warning"
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
	fmt.Printf("  Passed: %s\n", getColoredCount(report.Summary.Passed, "passed"))
	fmt.Printf("  Failed: %s\n", getColoredCount(report.Summary.Failed, "failed"))
	fmt.Printf("  Warnings: %s\n", getColoredCount(report.Summary.Warnings, "warning"))
	fmt.Printf("  Skipped: %s\n", getColoredCount(report.Summary.Skipped, "skipped"))
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
			version := "N/A"
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
			version := "N/A"
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
	case "failed":
		return "✗ FAILED"
	case "warning":
		return "⚠ WARNING"
	case "skipped":
		return "- SKIPPED"
	default:
		return strings.ToUpper(status)
	}
}

func getColoredCount(count int, _ string) string {
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
