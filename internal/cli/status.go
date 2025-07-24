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

	"lsp-gateway/internal/installer"
	"lsp-gateway/internal/types"

	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var (
	statusJSON    bool
	statusVerbose bool
	statusTimeout time.Duration
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show system status",
	Long: `Show the status of runtimes, language servers, and system configuration.
	
The status command provides comprehensive information about the current state
of your LSP Gateway installation, including:
- Runtime installations (Go, Python, Node.js, Java)
- Language server availability
- Configuration validity
- System environment details

Examples:
  # Show all status information
  lsp-gateway status
  
  # Show only runtime status
  lsp-gateway status runtimes
  
  # Show specific runtime status
  lsp-gateway status runtime go
  
  # Output as JSON for automation
  lsp-gateway status --json
  
  # Verbose output with detailed information
  lsp-gateway status --verbose`,
	RunE: statusAll,
}

var statusRuntimesCmd = &cobra.Command{
	Use:   "runtimes",
	Short: "Show runtime status",
	Long: `Show the status of all supported runtimes.
	
This command displays information about runtime installations including:
- Installation status (installed/missing)
- Version information
- Compatibility with minimum requirements
- Executable paths
- Any detected issues

Examples:
  # Show all runtime status
  lsp-gateway status runtimes
  
  # JSON output
  lsp-gateway status runtimes --json`,
	RunE: statusRuntimes,
}

var statusRuntimeCmd = &cobra.Command{
	Use:   "runtime <name>",
	Short: "Show specific runtime status",
	Long: `Show detailed status information for a specific runtime.
	
Supported runtimes: go, python, nodejs, java

This command provides detailed information about a single runtime including:
- Installation status and version
- Compatibility check results
- Executable path and permissions
- Configuration requirements
- Troubleshooting suggestions

Examples:
  # Show Go runtime status
  lsp-gateway status runtime go
  
  # Show Python runtime status with verbose output
  lsp-gateway status runtime python --verbose`,
	Args: cobra.ExactArgs(1),
	RunE: statusRuntime,
}

var statusServersCmd = &cobra.Command{
	Use:   "servers",
	Short: "Show language server status",
	Long: `Show the status of all supported language servers.
	
This command displays information about language server installations including:
- Installation status (installed/missing)
- Version information
- Compatibility with runtime requirements
- Health check results
- Functional status
- Any detected issues

Examples:
  # Show all server status
  lsp-gateway status servers
  
  # JSON output
  lsp-gateway status servers --json`,
	RunE: statusServers,
}

var statusServerCmd = &cobra.Command{
	Use:   "server <name>",
	Short: "Show specific server status",
	Long: `Show detailed status information for a specific language server.
	
Supported servers: gopls, pylsp, typescript-language-server, jdtls

This command provides detailed information about a single server including:
- Installation status and version
- Runtime dependency verification
- Health check results
- Functional test results
- Troubleshooting suggestions

Examples:
  # Show gopls status
  lsp-gateway status server gopls
  
  # Show TypeScript language server status with verbose output
  lsp-gateway status server typescript-language-server --verbose`,
	Args: cobra.ExactArgs(1),
	RunE: statusServer,
}

func init() {
	statusCmd.PersistentFlags().BoolVar(&statusJSON, "json", false, "Output in JSON format")
	statusCmd.PersistentFlags().BoolVarP(&statusVerbose, "verbose", "v", false, "Verbose output")
	statusCmd.PersistentFlags().DurationVar(&statusTimeout, "timeout", 30*time.Second, "Detection timeout")

	statusCmd.AddCommand(statusRuntimesCmd)
	statusCmd.AddCommand(statusRuntimeCmd)
	statusCmd.AddCommand(statusServersCmd)
	statusCmd.AddCommand(statusServerCmd)

	rootCmd.AddCommand(statusCmd)
}

// Exported functions for testing
func StatusAll(cmd *cobra.Command, args []string) error {
	return statusAll(cmd, args)
}

func StatusRuntimes(cmd *cobra.Command, args []string) error {
	return statusRuntimes(cmd, args)
}

func StatusRuntime(cmd *cobra.Command, args []string) error {
	return statusRuntime(cmd, args)
}

func StatusServers(cmd *cobra.Command, args []string) error {
	return statusServers(cmd, args)
}

func StatusServer(cmd *cobra.Command, args []string) error {
	return statusServer(cmd, args)
}

// Exported variables for testing
var (
	StatusJSON    = &statusJSON
	StatusVerbose = &statusVerbose
)

// Exported utility functions for testing
func GetStatusIcon(installed bool) string {
	return getStatusIcon(installed)
}

func GetCompatibleText(compatible bool) string {
	return getCompatibleText(compatible)
}

func GetWorkingText(working bool) string {
	return getWorkingText(working)
}

func FormatRuntimeName(name string) string {
	return formatRuntimeName(name)
}

func OutputServersTableHeader() {
	outputServersTableHeader()
}

func InitializeStatusData() map[string]interface{} {
	return initializeStatusData()
}

func BuildRuntimeStatusData(verifyResult *types.VerificationResult, err error, installedCount *int, compatibleCount *int) map[string]interface{} {
	return buildRuntimeStatusData(verifyResult, err, installedCount, compatibleCount)
}

// GetStatusCmd returns the status command for testing purposes
func GetStatusCmd() *cobra.Command {
	return statusCmd
}

func statusAll(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), statusTimeout)
	defer cancel()
	_ = ctx

	if statusJSON {
		return outputComprehensiveStatusJSON()
	}

	return outputComprehensiveStatusHuman()
}

func statusRuntimes(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), statusTimeout)
	defer cancel()
	_ = ctx

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
		} else {
			result.Details["installed"] = verifyResult.Installed
			result.Details["version"] = verifyResult.Version
			result.Details["compatible"] = verifyResult.Compatible
			result.Details["path"] = verifyResult.Path

			if !verifyResult.Installed {
				result.Status = StatusFailed
				result.Message = fmt.Sprintf("%s runtime not installed", strings.ToUpper(runtimeName[:1])+runtimeName[1:])
			} else if !verifyResult.Compatible {
				result.Status = StatusWarning
				result.Message = fmt.Sprintf("%s version %s does not meet requirements", strings.ToUpper(runtimeName[:1])+runtimeName[1:], verifyResult.Version)
			} else {
				result.Status = StatusPassed
				result.Message = fmt.Sprintf("%s runtime %s is properly installed and compatible", strings.ToUpper(runtimeName[:1])+runtimeName[1:], verifyResult.Version)
			}
		}

		results = append(results, result)
	}

	if statusJSON {
		report := &DiagnosticReport{
			Timestamp: time.Now(),
			Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
			Results:   results,
		}
		return outputDiagnoseJSON(report)
	}

	return outputRuntimeDiagnosticsHuman(results)
}

func statusRuntime(cmd *cobra.Command, args []string) error {
	runtimeName := strings.ToLower(args[0])

	ctx, cancel := context.WithTimeout(context.Background(), statusTimeout)
	defer cancel()
	_ = ctx

	if runtimeName == "node" {
		runtimeName = "nodejs"
	}

	runtimeInstaller := installer.NewRuntimeInstaller()
	if runtimeInstaller == nil {
		return NewInstallerNotAvailableError("runtime")
	}

	supportedRuntimes := runtimeInstaller.GetSupportedRuntimes()
	validRuntime := false
	for _, supported := range supportedRuntimes {
		if runtimeName == supported {
			validRuntime = true
			break
		}
	}

	if !validRuntime {
		return NewUnsupportedRuntimeError(runtimeName, supportedRuntimes)
	}

	result, err := runtimeInstaller.Verify(runtimeName)
	if err != nil {
		return &CLIError{
			Type:    ErrorTypeRuntime,
			Message: fmt.Sprintf("Failed to verify %s runtime", runtimeName),
			Cause:   err,
			Suggestions: []string{
				fmt.Sprintf("Install %s runtime", runtimeName),
				fmt.Sprintf("Check if %s is available in your PATH", runtimeName),
				"Run system diagnostics: lsp-gateway diagnose",
				"Try manual installation and verification",
			},
			RelatedCmds: []string{
				fmt.Sprintf("install runtime %s", runtimeName),
				"diagnose",
			},
		}
	}

	if statusJSON {
		if output, err := json.Marshal(result); err == nil {
			fmt.Println(string(output))
		} else {
			return NewJSONMarshalError(LABEL_RUNTIME_STATUS, err)
		}
		return nil
	}

	fmt.Printf("%s Runtime Status:\n", strings.ToUpper(runtimeName[:1])+runtimeName[1:])
	fmt.Printf("========================================\n\n")

	fmt.Printf("Installation Status: %s\n", getStatusIcon(result.Installed))
	fmt.Printf("Version: %s\n", result.Version)
	fmt.Printf(FORMAT_EXECUTABLE_PATH, result.Path)
	fmt.Printf("Compatible: %s\n", getCompatibleText(result.Compatible))

	if len(result.EnvironmentVars) > 0 {
		fmt.Printf("\nEnvironment Variables:\n")
		for k, v := range result.EnvironmentVars {
			fmt.Printf("  %s=%s\n", k, v)
		}
	}

	if len(result.Issues) > 0 {
		fmt.Printf("\nIssues Found:\n")
		for _, issue := range result.Issues {
			fmt.Printf("  [%s] %s\n", strings.ToUpper(string(issue.Severity)), issue.Title)
			fmt.Printf(FORMAT_SPACES_INSTRUCTION, issue.Description)
			fmt.Printf(FORMAT_SOLUTION_INSTRUCTION, issue.Solution)
			fmt.Println()
		}
	}

	if len(result.Recommendations) > 0 {
		fmt.Printf("\nRecommendations:\n")
		for _, rec := range result.Recommendations {
			fmt.Printf("  - %s\n", rec)
		}
	}

	if statusVerbose {
		fmt.Printf("\nDetailed Information:\n")
		fmt.Printf("  Verification Duration: %v\n", result.Duration)
		fmt.Printf(FORMAT_VERIFIED_AT, result.VerifiedAt.Format(time.RFC3339))
		if len(result.AdditionalPaths) > 0 {
			fmt.Printf("  Additional Paths: %s\n", strings.Join(result.AdditionalPaths, ", "))
		}
	}

	return nil
}

func statusServers(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), statusTimeout)
	defer cancel()
	_ = ctx

	runtimeInstaller := installer.NewRuntimeInstaller()
	if runtimeInstaller == nil {
		return NewInstallerCreationError("runtime")
	}

	serverInstaller := installer.NewServerInstaller(runtimeInstaller)
	if serverInstaller == nil {
		return NewInstallerCreationError("server")
	}

	supportedServers := serverInstaller.GetSupportedServers()

	if statusJSON {
		return outputServersStatusJSON(serverInstaller, supportedServers)
	}

	return outputServersStatusHuman(serverInstaller, supportedServers)
}

func outputServersStatusJSON(installer *installer.DefaultServerInstaller, servers []string) error {
	type ServerStatus struct {
		Name        string   `json:"name"`
		DisplayName string   `json:"display_name"`
		Runtime     string   `json:"runtime"`
		Installed   bool     `json:"installed"`
		Working     bool     `json:"working"`
		Compatible  bool     `json:"compatible"`
		Version     string   `json:"version,omitempty"`
		Path        string   `json:"path,omitempty"`
		Issues      []string `json:"issues,omitempty"`
	}

	type StatusOutput struct {
		Success bool           `json:"success"`
		Servers []ServerStatus `json:"servers"`
		Summary struct {
			Total      int `json:"total"`
			Installed  int `json:"installed"`
			Working    int `json:"working"`
			Compatible int `json:"compatible"`
		} `json:"summary"`
	}

	output := StatusOutput{
		Success: true,
		Servers: make([]ServerStatus, 0, len(servers)),
	}

	for _, serverName := range servers {
		serverInfo, _ := installer.GetServerInfo(serverName)
		verifyResult, _ := installer.Verify(serverName)

		status := ServerStatus{
			Name:        serverName,
			DisplayName: serverName,
			Runtime:     "",
			Installed:   false,
			Working:     false,
			Compatible:  false,
		}

		if serverInfo != nil {
			status.DisplayName = serverInfo.DisplayName
			status.Runtime = serverInfo.Runtime
		}

		if verifyResult != nil {
			status.Installed = verifyResult.Installed
			status.Working = verifyResult.Compatible
			status.Compatible = verifyResult.Compatible
			status.Version = verifyResult.Version
			status.Path = verifyResult.Path
			for _, issue := range verifyResult.Issues {
				status.Issues = append(status.Issues, issue.Description)
			}
		}

		output.Servers = append(output.Servers, status)

		output.Summary.Total++
		if status.Installed {
			output.Summary.Installed++
		}
		if status.Working {
			output.Summary.Working++
		}
		if status.Compatible {
			output.Summary.Compatible++
		}
	}

	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return NewJSONMarshalError("status information", err)
	}

	fmt.Println(string(jsonData))
	return nil
}

func outputServersStatusHuman(installer *installer.DefaultServerInstaller, servers []string) error {
	if len(servers) == 0 {
		fmt.Println("No language servers are configured")
		return nil
	}

	if err := outputServersTableHeader(); err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if err := writeServersTableHeaders(w); err != nil {
		return err
	}

	counts, err := writeServersTableRows(w, installer, servers)
	if err != nil {
		return err
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush table output: %w", err)
	}

	outputServersSummary(counts.installed, counts.working, len(servers))

	if statusVerbose {
		return outputServersVerboseDetails(installer, servers)
	}

	return nil
}

func getStatusIcon(installed bool) string {
	if installed {
		return "✓ Installed"
	}
	return "✗ Not Installed"
}

func getCompatibleText(compatible bool) string {
	if compatible {
		return "Yes"
	}
	return "No"
}

type serverCounts struct {
	installed int
	working   int
}

type serverDisplayInfo struct {
	displayName string
	runtime     string
	status      string
	version     string
	path        string
}

func outputServersTableHeader() error {
	fmt.Printf("Language Server Status\n")
	fmt.Printf("======================\n\n")
	return nil
}

func writeServersTableHeaders(w *tabwriter.Writer) error {
	if _, err := fmt.Fprintf(w, "SERVER\tDISPLAY NAME\tRUNTIME\tSTATUS\tVERSION\tPATH\n"); err != nil {
		return fmt.Errorf("failed to write table header: %w", err)
	}
	if _, err := fmt.Fprintf(w, "------\t------------\t-------\t------\t-------\t----\n"); err != nil {
		return fmt.Errorf("failed to write table separator: %w", err)
	}
	return nil
}

func writeServersTableRows(w *tabwriter.Writer, installer *installer.DefaultServerInstaller, servers []string) (serverCounts, error) {
	counts := serverCounts{}

	for _, serverName := range servers {
		info := extractServerDisplayInfo(installer, serverName, &counts)
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			serverName, info.displayName, info.runtime, info.status, info.version, info.path); err != nil {
			return counts, fmt.Errorf("failed to write table row: %w", err)
		}
	}

	return counts, nil
}

func extractServerDisplayInfo(installer *installer.DefaultServerInstaller, serverName string, counts *serverCounts) serverDisplayInfo {
	serverInfo, _ := installer.GetServerInfo(serverName)
	verifyResult, _ := installer.Verify(serverName)

	info := serverDisplayInfo{
		displayName: serverName,
		runtime:     "unknown",
		status:      "✗ Not Installed",
		version:     "N/A",
		path:        "N/A",
	}

	if serverInfo != nil {
		info.displayName = serverInfo.DisplayName
		info.runtime = serverInfo.Runtime
	}

	if verifyResult != nil && verifyResult.Installed {
		counts.installed++
		if verifyResult.Compatible {
			counts.working++
			info.status = "✓ Working"
		} else {
			info.status = "⚠ Installed (Issues)"
		}

		if verifyResult.Version != "" {
			info.version = verifyResult.Version
		}
		if verifyResult.Path != "" {
			info.path = verifyResult.Path
		}
	}

	return info
}

func outputServersSummary(installedCount, workingCount, totalServers int) {
	fmt.Printf("\nSummary:\n")
	fmt.Printf("- Total servers:    %d\n", totalServers)
	fmt.Printf("- Installed:        %d\n", installedCount)
	fmt.Printf("- Working properly: %d\n", workingCount)

	if installedCount < totalServers {
		fmt.Printf("- Missing:          %d\n", totalServers-installedCount)
	}
}

func outputServersVerboseDetails(installer *installer.DefaultServerInstaller, servers []string) error {
	fmt.Printf("\nDetailed Information:\n")
	for _, serverName := range servers {
		if err := outputSingleServerDetails(installer, serverName); err != nil {
			return err
		}
	}
	return nil
}

func outputSingleServerDetails(installer *installer.DefaultServerInstaller, serverName string) error {
	serverInfo, _ := installer.GetServerInfo(serverName)
	verifyResult, _ := installer.Verify(serverName)

	displayName := "Unknown"
	if serverInfo != nil {
		displayName = serverInfo.DisplayName
	}

	fmt.Printf("\n%s (%s):\n", serverName, displayName)

	if serverInfo != nil {
		outputServerInfoDetails(serverInfo)
	}

	if verifyResult != nil && len(verifyResult.Issues) > 0 {
		outputServerIssues(verifyResult.Issues)
	}

	return nil
}

func outputServerInfoDetails(serverInfo *types.ServerDefinition) {
	fmt.Printf("  Required runtime: %s (min version: %s)\n", serverInfo.Runtime, serverInfo.MinRuntimeVersion)
	fmt.Printf("  Install command:  %s\n", serverInfo.InstallCmd)
	fmt.Printf("  Verify command:   %s\n", serverInfo.VerifyCmd)
	fmt.Printf("  Languages:        %s\n", strings.Join(serverInfo.Languages, ", "))
	fmt.Printf("  File extensions:  %s\n", strings.Join(serverInfo.Extensions, ", "))
}

func outputServerIssues(issues []types.Issue) {
	fmt.Printf("  Issues:\n")
	for _, issue := range issues {
		fmt.Printf("    - %s\n", issue)
	}
}

func getWorkingText(working bool) string {
	if working {
		return "Yes"
	}
	return "No"
}

func statusServer(cmd *cobra.Command, args []string) error {
	serverName := strings.ToLower(args[0])

	ctx, cancel := context.WithTimeout(context.Background(), statusTimeout)
	defer cancel()
	_ = ctx

	runtimeInstaller := installer.NewRuntimeInstaller()
	serverInstaller := installer.NewServerInstaller(runtimeInstaller)

	supportedServers := serverInstaller.GetSupportedServers()
	validServer := false
	for _, supported := range supportedServers {
		if serverName == supported {
			validServer = true
			break
		}
	}

	if !validServer {
		return NewServerNotFoundError(serverName)
	}

	result, err := serverInstaller.Verify(serverName)
	if err != nil {
		return &CLIError{
			Type:    ErrorTypeServer,
			Message: fmt.Sprintf("Failed to verify %s language server", serverName),
			Cause:   err,
			Suggestions: []string{
				fmt.Sprintf("Install %s: lsp-gateway install server %s", serverName, serverName),
				"Check runtime dependencies: lsp-gateway status runtimes",
				"Run system diagnostics: lsp-gateway diagnose",
				"Verify the server is in your PATH environment variable",
			},
			RelatedCmds: []string{
				fmt.Sprintf("install server %s", serverName),
				"status runtimes",
				"diagnose",
			},
		}
	}

	if statusJSON {
		if output, err := json.Marshal(result); err == nil {
			fmt.Println(string(output))
		} else {
			return NewJSONMarshalError("server status", err)
		}
		return nil
	}

	fmt.Printf("%s Language Server Status:\n", strings.ToUpper(serverName[:1])+serverName[1:])
	fmt.Printf("========================================\n\n")

	fmt.Printf("Installation Status: %s\n", getStatusIcon(result.Installed))
	fmt.Printf("Version: %s\n", result.Version)
	fmt.Printf(FORMAT_EXECUTABLE_PATH, result.Path)
	fmt.Printf("Compatible: %s\n", getCompatibleText(result.Compatible))

	if functional, ok := result.Metadata["functional"].(bool); ok {
		fmt.Printf("Functional: %s\n", getWorkingText(functional))
	}

	if runtimeRequired, ok := result.Metadata["runtime_required"].(string); ok {
		fmt.Printf("Runtime Required: %s\n", runtimeRequired)
	}

	if healthCheck, ok := result.Metadata["health_check"].(map[string]interface{}); ok {
		fmt.Printf("\nHealth Check:\n")
		if responsive, ok := healthCheck["responsive"].(bool); ok {
			fmt.Printf("  Responsive: %s\n", getWorkingText(responsive))
		}
		if testMethod, ok := healthCheck["test_method"].(string); ok {
			fmt.Printf("  Test Method: %s\n", testMethod)
		}
		if startupTime, ok := healthCheck["startup_time"].(time.Duration); ok {
			fmt.Printf("  Startup Time: %v\n", startupTime)
		}
	}

	if len(result.EnvironmentVars) > 0 {
		fmt.Printf("\nEnvironment Variables:\n")
		for k, v := range result.EnvironmentVars {
			fmt.Printf("  %s=%s\n", k, v)
		}
	}

	if len(result.Issues) > 0 {
		fmt.Printf("\nIssues Found:\n")
		for _, issue := range result.Issues {
			fmt.Printf("  [%s] %s\n", strings.ToUpper(string(issue.Severity)), issue.Title)
			fmt.Printf(FORMAT_SPACES_INSTRUCTION, issue.Description)
			fmt.Printf(FORMAT_SOLUTION_INSTRUCTION, issue.Solution)
			fmt.Println()
		}
	}

	if len(result.Recommendations) > 0 {
		fmt.Printf("\nRecommendations:\n")
		for _, rec := range result.Recommendations {
			fmt.Printf("  - %s\n", rec)
		}
	}

	if statusVerbose {
		fmt.Printf("\nDetailed Information:\n")
		fmt.Printf("  Verification Duration: %v\n", result.Duration)
		fmt.Printf(FORMAT_VERIFIED_AT, result.VerifiedAt.Format(time.RFC3339))
		if len(result.AdditionalPaths) > 0 {
			fmt.Printf("  Additional Paths: %s\n", strings.Join(result.AdditionalPaths, ", "))
		}
	}

	return nil
}

func outputComprehensiveStatusJSON() error {
	status := initializeStatusData()

	runtimeInstaller := installer.NewRuntimeInstaller()
	if runtimeInstaller != nil {
		runtimeData := collectRuntimeStatusJSON(runtimeInstaller)
		status["runtimes"] = runtimeData

		serverData := collectServerStatusJSON(runtimeInstaller)
		if serverData != nil {
			status["servers"] = serverData
		}
	}

	return outputJSONStatus(status)
}

func initializeStatusData() map[string]interface{} {
	return map[string]interface{}{
		"timestamp": time.Now(),
		"success":   true,
	}
}

func collectRuntimeStatusJSON(runtimeInstaller *installer.DefaultRuntimeInstaller) map[string]interface{} {
	supportedRuntimes := runtimeInstaller.GetSupportedRuntimes()
	runtimeStatus := make(map[string]interface{})
	installedRuntimes := 0
	compatibleRuntimes := 0

	for _, runtime := range supportedRuntimes {
		verifyResult, err := runtimeInstaller.Verify(runtime)
		runtimeData := buildRuntimeStatusData(verifyResult, err, &installedRuntimes, &compatibleRuntimes)
		runtimeStatus[runtime] = runtimeData
	}

	return map[string]interface{}{
		"details": runtimeStatus,
		"summary": map[string]interface{}{
			"total":      len(supportedRuntimes),
			"installed":  installedRuntimes,
			"compatible": compatibleRuntimes,
		},
	}
}

func buildRuntimeStatusData(verifyResult *types.VerificationResult, err error, installedCount *int, compatibleCount *int) map[string]interface{} {
	if err != nil {
		return map[string]interface{}{
			"installed":  false,
			"compatible": false,
			"error":      err.Error(),
		}
	}

	if verifyResult.Installed {
		(*installedCount)++
	}
	if verifyResult.Compatible {
		(*compatibleCount)++
	}

	return map[string]interface{}{
		"installed":  verifyResult.Installed,
		"compatible": verifyResult.Compatible,
		"version":    verifyResult.Version,
		"path":       verifyResult.Path,
	}
}

func collectServerStatusJSON(runtimeInstaller *installer.DefaultRuntimeInstaller) map[string]interface{} {
	serverInstaller := installer.NewServerInstaller(runtimeInstaller)
	if serverInstaller == nil {
		return nil
	}

	supportedServers := serverInstaller.GetSupportedServers()
	serverStatus := make(map[string]interface{})
	installedServers := 0
	workingServers := 0

	for _, server := range supportedServers {
		verifyResult, err := serverInstaller.Verify(server)
		serverData := buildServerStatusData(verifyResult, err, &installedServers, &workingServers)
		serverStatus[server] = serverData
	}

	return map[string]interface{}{
		"details": serverStatus,
		"summary": map[string]interface{}{
			"total":     len(supportedServers),
			"installed": installedServers,
			"working":   workingServers,
		},
	}
}

func buildServerStatusData(verifyResult *types.VerificationResult, err error, installedCount *int, workingCount *int) map[string]interface{} {
	if err != nil {
		return map[string]interface{}{
			"installed": false,
			"working":   false,
			"error":     err.Error(),
		}
	}

	if verifyResult.Installed {
		(*installedCount)++
	}
	if verifyResult.Compatible {
		(*workingCount)++
	}

	return map[string]interface{}{
		"installed": verifyResult.Installed,
		"working":   verifyResult.Compatible,
		"version":   verifyResult.Version,
		"path":      verifyResult.Path,
	}
}

func outputJSONStatus(status map[string]interface{}) error {
	jsonData, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return NewJSONMarshalError("comprehensive status", err)
	}

	fmt.Println(string(jsonData))
	return nil
}

func outputComprehensiveStatusHuman() error {
	printStatusHeader()

	runtimeInstaller := installer.NewRuntimeInstaller()
	if runtimeInstaller != nil {
		outputRuntimeStatusHuman(runtimeInstaller)
		outputServerStatusHuman(runtimeInstaller)
	}

	printQuickActions()
	return nil
}

func printStatusHeader() {
	fmt.Printf("LSP Gateway System Status\n")
	fmt.Printf("========================\n\n")
	fmt.Printf("Timestamp: %s\n\n", time.Now().Format(time.RFC3339))
}

func outputRuntimeStatusHuman(runtimeInstaller *installer.DefaultRuntimeInstaller) {
	supportedRuntimes := runtimeInstaller.GetSupportedRuntimes()
	installedRuntimes := 0
	compatibleRuntimes := 0

	fmt.Printf("Runtimes (%d supported):\n", len(supportedRuntimes))
	fmt.Printf("------------------------\n")

	for _, runtime := range supportedRuntimes {
		verifyResult, err := runtimeInstaller.Verify(runtime)
		status, version := getRuntimeStatusAndVersion(verifyResult, err, &installedRuntimes, &compatibleRuntimes)
		fmt.Printf("  %-10s %s %s\n", formatRuntimeName(runtime), status, version)
	}

	fmt.Printf("\nRuntime Summary: %d/%d installed, %d/%d compatible\n\n",
		installedRuntimes, len(supportedRuntimes), compatibleRuntimes, installedRuntimes)
}

func getRuntimeStatusAndVersion(verifyResult *types.VerificationResult, err error, installedCount *int, compatibleCount *int) (string, string) {
	status := "✗ Not Installed"
	version := "N/A"

	if err != nil {
		status = "✗ Error"
	} else if verifyResult.Installed {
		(*installedCount)++
		if verifyResult.Compatible {
			(*compatibleCount)++
			status = "✓ Working"
		} else {
			status = "⚠ Issues"
		}
		if verifyResult.Version != "" {
			version = verifyResult.Version
		}
	}

	return status, version
}

func formatRuntimeName(runtime string) string {
	return strings.ToUpper(runtime[:1]) + runtime[1:] + ":"
}

func outputServerStatusHuman(runtimeInstaller *installer.DefaultRuntimeInstaller) {
	serverInstaller := installer.NewServerInstaller(runtimeInstaller)
	if serverInstaller == nil {
		return
	}

	supportedServers := serverInstaller.GetSupportedServers()
	installedServers := 0
	workingServers := 0

	fmt.Printf("Language Servers (%d supported):\n", len(supportedServers))
	fmt.Printf("---------------------------------\n")

	for _, server := range supportedServers {
		verifyResult, err := serverInstaller.Verify(server)
		status, version := getServerStatusAndVersion(verifyResult, err, &installedServers, &workingServers)
		fmt.Printf("  %-25s %s %s\n", server+":", status, version)
	}

	fmt.Printf("\nServer Summary: %d/%d installed, %d/%d working\n\n",
		installedServers, len(supportedServers), workingServers, installedServers)
}

func getServerStatusAndVersion(verifyResult *types.VerificationResult, err error, installedCount *int, workingCount *int) (string, string) {
	status := "✗ Not Installed"
	version := "N/A"

	if err != nil {
		status = "✗ Error"
	} else if verifyResult.Installed {
		(*installedCount)++
		if verifyResult.Compatible {
			(*workingCount)++
			status = "✓ Working"
		} else {
			status = "⚠ Issues"
		}
		if verifyResult.Version != "" {
			version = verifyResult.Version
		}
	}

	return status, version
}

func printQuickActions() {
	fmt.Printf("Quick Actions:\n")
	fmt.Printf("--------------\n")
	fmt.Printf("• Install all dependencies: lsp-gateway install servers\n")
	fmt.Printf("• Run full diagnostics:     lsp-gateway diagnose\n")
	fmt.Printf("• Start HTTP Gateway:       lsp-gateway server\n")
	fmt.Printf("• Start MCP Server:         lsp-gateway mcp\n")
	fmt.Printf("• Generate config:          lsp-gateway config generate\n")
}
