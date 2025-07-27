package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"lsp-gateway/internal/gateway"

	"github.com/spf13/cobra"
)

var (
	bypassJSON    bool
	bypassVerbose bool
	bypassForce   bool
	bypassReason  string
)

var bypassCmd = &cobra.Command{
	Use:   "bypass",
	Short: "Manage server bypass decisions",
	Long: `Manage server bypass decisions for LSP Gateway.

When servers fail repeatedly or encounter issues, they can be bypassed to maintain
overall system functionality while allowing recovery. This command provides
comprehensive bypass management including:

• Bypass servers manually or automatically
• List currently bypassed servers
• Remove bypass decisions to re-enable servers
• Check bypass statistics and recovery status
• Configure fallback strategies for bypassed servers

Examples:
  # List all bypassed servers
  lsp-gateway bypass list

  # Bypass a specific server manually
  lsp-gateway bypass set python-server --reason "Memory issues"

  # Remove bypass for a server (re-enable)
  lsp-gateway bypass clear python-server

  # Show bypass statistics
  lsp-gateway bypass stats

  # Check if a server can attempt recovery
  lsp-gateway bypass recovery python-server`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

var bypassListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all bypassed servers",
	Long: `List all currently bypassed servers with their bypass information.

This command shows:
- Server names and languages
- Bypass reasons and timestamps
- Whether bypass was user-initiated or automatic
- Recovery attempt counts
- Bypass decision types

Examples:
  # List all bypassed servers
  lsp-gateway bypass list

  # List in JSON format for automation
  lsp-gateway bypass list --json

  # Verbose output with full details
  lsp-gateway bypass list --verbose`,
	RunE: bypassList,
}

var bypassSetCmd = &cobra.Command{
	Use:   "set <server-name>",
	Short: "Bypass a server manually",
	Long: `Bypass a server manually with a specific reason.

This command allows you to manually bypass a server, preventing it from
receiving requests until the bypass is cleared. This is useful when:
- A server is experiencing issues
- You want to test alternative servers
- A server needs maintenance

Examples:
  # Bypass a server with a reason
  lsp-gateway bypass set python-server --reason "Under maintenance"

  # Bypass with specific strategy
  lsp-gateway bypass set go-server --reason "Memory leak" --strategy fallback_cache

  # Force bypass even if server is healthy
  lsp-gateway bypass set ts-server --reason "Testing" --force`,
	Args: cobra.ExactArgs(1),
	RunE: bypassSet,
}

var bypassClearCmd = &cobra.Command{
	Use:   "clear <server-name>",
	Short: "Remove bypass for a server (re-enable)",
	Long: `Remove bypass decision for a server, allowing it to receive requests again.

This command re-enables a previously bypassed server by:
- Removing the bypass state
- Resetting circuit breakers
- Clearing failure counters
- Transitioning server to healthy state if active

Examples:
  # Re-enable a bypassed server
  lsp-gateway bypass clear python-server

  # Force clear even if server might not be ready
  lsp-gateway bypass clear go-server --force`,
	Args: cobra.ExactArgs(1),
	RunE: bypassClear,
}

var bypassStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show bypass statistics",
	Long: `Show comprehensive statistics about server bypass decisions.

This command displays:
- Total number of bypassed servers
- Breakdown by bypass reasons and decision types
- Bypass strategy usage statistics
- Fallback performance metrics
- Historical bypass data

Examples:
  # Show bypass statistics
  lsp-gateway bypass stats

  # JSON output for monitoring
  lsp-gateway bypass stats --json`,
	RunE: bypassStats,
}

var bypassRecoveryCmd = &cobra.Command{
	Use:   "recovery <server-name>",
	Short: "Check if server can attempt recovery",
	Long: `Check if a bypassed server is eligible for recovery attempts.

This command shows:
- Current bypass status
- Recovery eligibility
- Cooldown periods
- Previous recovery attempts
- Recommendations for recovery

Examples:
  # Check recovery status for a server
  lsp-gateway bypass recovery python-server

  # Verbose recovery information
  lsp-gateway bypass recovery go-server --verbose`,
	Args: cobra.ExactArgs(1),
	RunE: bypassRecovery,
}

func bypassList(cmd *cobra.Command, args []string) error {
	// Create bypass state manager
	stateDir := "./data"
	if stateDir == "" {
		stateDir = "/tmp/lsp-gateway"
	}

	bsm := gateway.NewBypassStateManager(stateDir, nil)
	bypassedServers := bsm.GetAllBypassedServers()

	if bypassJSON {
		return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
			"bypassed_servers": bypassedServers,
			"total_count":      len(bypassedServers),
			"timestamp":        time.Now(),
		})
	}

	if len(bypassedServers) == 0 {
		fmt.Println("No servers are currently bypassed.")
		return nil
	}

	fmt.Printf("Bypassed Servers (%d total):\n\n", len(bypassedServers))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SERVER\tLANGUAGE\tREASON\tTYPE\tBYPASSED\tRECOVERY ATTEMPTS")

	for _, entry := range bypassedServers {
		bypassedAgo := time.Since(entry.BypassTimestamp)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\n",
			entry.ServerName,
			entry.Language,
			entry.BypassReason,
			entry.BypassDecisionType,
			formatDuration(bypassedAgo),
			entry.RecoveryAttempts,
		)
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush table output: %w", err)
	}

	if bypassVerbose {
		fmt.Println("\nDetailed Information:")
		for _, entry := range bypassedServers {
			fmt.Printf("\n%s:\n", entry.ServerName)
			fmt.Printf("  Language: %s\n", entry.Language)
			fmt.Printf("  Bypass Reason: %s\n", entry.BypassReason)
			fmt.Printf("  Decision Type: %s\n", entry.BypassDecisionType)
			fmt.Printf("  User Decision: %t\n", entry.UserBypassDecision)
			fmt.Printf("  Bypassed At: %s\n", entry.BypassTimestamp.Format(time.RFC3339))
			fmt.Printf("  Recovery Attempts: %d\n", entry.RecoveryAttempts)
			if !entry.LastRecoveryTime.IsZero() {
				fmt.Printf("  Last Recovery Try: %s\n", entry.LastRecoveryTime.Format(time.RFC3339))
			}
		}
	}

	return nil
}

func bypassSet(cmd *cobra.Command, args []string) error {
	serverName := args[0]

	if bypassReason == "" {
		return fmt.Errorf("bypass reason is required (use --reason flag)")
	}

	// Create bypass state manager
	stateDir := "./data"
	if stateDir == "" {
		stateDir = "/tmp/lsp-gateway"
	}

	bsm := gateway.NewBypassStateManager(stateDir, nil)

	// Check if server is already bypassed
	if bsm.IsBypassed(serverName) && !bypassForce {
		return fmt.Errorf("server %s is already bypassed (use --force to override)", serverName)
	}

	// Determine bypass strategy
	strategy := gateway.BypassDecisionUserManual
	if bypassStrategy != "" {
		switch strings.ToLower(bypassStrategy) {
		case "user_manual":
			strategy = gateway.BypassDecisionUserManual
		case "auto_consecutive_failures":
			strategy = gateway.BypassDecisionAutoConsecutive
		case "auto_circuit_breaker":
			strategy = gateway.BypassDecisionAutoCircuitBreaker
		case "auto_health_degraded":
			strategy = gateway.BypassDecisionAutoHealthDegraded
		default:
			return fmt.Errorf("invalid bypass strategy: %s", bypassStrategy)
		}
	}

	// Set bypass state
	err := bsm.SetBypassState(serverName, "unknown", bypassReason, strategy, true)
	if err != nil {
		return fmt.Errorf("failed to set bypass state: %w", err)
	}

	fmt.Printf("✓ Server '%s' has been bypassed\n", serverName)
	fmt.Printf("  Reason: %s\n", bypassReason)
	fmt.Printf("  Strategy: %s\n", strategy)
	fmt.Printf("  Timestamp: %s\n", time.Now().Format(time.RFC3339))

	return nil
}

func bypassClear(cmd *cobra.Command, args []string) error {
	serverName := args[0]

	// Create bypass state manager
	stateDir := "./data"
	if stateDir == "" {
		stateDir = "/tmp/lsp-gateway"
	}

	bsm := gateway.NewBypassStateManager(stateDir, nil)

	// Check if server is bypassed
	if !bsm.IsBypassed(serverName) {
		fmt.Printf("Server '%s' is not currently bypassed.\n", serverName)
		return nil
	}

	// Get bypass info before clearing
	bypassInfo := bsm.GetBypassInfo(serverName)

	// Clear bypass state
	err := bsm.ClearBypassState(serverName)
	if err != nil {
		return fmt.Errorf("failed to clear bypass state: %w", err)
	}

	fmt.Printf("✓ Bypass cleared for server '%s'\n", serverName)
	if bypassInfo != nil {
		fmt.Printf("  Was bypassed for: %s\n", formatDuration(time.Since(bypassInfo.BypassTimestamp)))
		fmt.Printf("  Original reason: %s\n", bypassInfo.BypassReason)
	}

	return nil
}

func bypassStats(cmd *cobra.Command, args []string) error {
	// Create bypass state manager
	stateDir := "./data"
	if stateDir == "" {
		stateDir = "/tmp/lsp-gateway"
	}

	bsm := gateway.NewBypassStateManager(stateDir, nil)
	stats := bsm.GetBypassStatistics()

	if bypassJSON {
		return json.NewEncoder(os.Stdout).Encode(stats)
	}

	fmt.Println("Bypass Statistics:")
	fmt.Printf("  Total Bypassed Servers: %d\n", stats["total_bypassed"])
	fmt.Printf("  State File: %s\n", stats["state_file"])
	fmt.Printf("  Last Update: %s\n", stats["last_update"])

	if decisionTypes, ok := stats["by_decision_type"].(map[gateway.BypassDecisionType]int); ok && len(decisionTypes) > 0 {
		fmt.Println("\nBypass Reasons:")
		for decisionType, count := range decisionTypes {
			fmt.Printf("  %s: %d\n", decisionType, count)
		}
	}

	if languages, ok := stats["by_language"].(map[string]int); ok && len(languages) > 0 {
		fmt.Println("\nBy Language:")
		for language, count := range languages {
			fmt.Printf("  %s: %d\n", language, count)
		}
	}

	if totalRecoveryAttempts, ok := stats["total_recovery_attempts"].(int); ok && totalRecoveryAttempts > 0 {
		fmt.Printf("\nTotal Recovery Attempts: %d\n", totalRecoveryAttempts)
	}

	return nil
}

func bypassRecovery(cmd *cobra.Command, args []string) error {
	serverName := args[0]

	// Create bypass state manager
	stateDir := "./data"
	if stateDir == "" {
		stateDir = "/tmp/lsp-gateway"
	}

	bsm := gateway.NewBypassStateManager(stateDir, nil)

	// Check if server is bypassed
	if !bsm.IsBypassed(serverName) {
		fmt.Printf("Server '%s' is not currently bypassed.\n", serverName)
		return nil
	}

	bypassInfo := bsm.GetBypassInfo(serverName)
	if bypassInfo == nil {
		return fmt.Errorf("could not get bypass information for server %s", serverName)
	}

	canAttempt := bsm.CanAttemptRecovery(serverName)

	fmt.Printf("Recovery Status for '%s':\n", serverName)
	fmt.Printf("  Bypassed: %t\n", true)
	fmt.Printf("  Can Attempt Recovery: %t\n", canAttempt)
	fmt.Printf("  Bypass Reason: %s\n", bypassInfo.BypassReason)
	fmt.Printf("  Decision Type: %s\n", bypassInfo.BypassDecisionType)
	fmt.Printf("  User Decision: %t\n", bypassInfo.UserBypassDecision)
	fmt.Printf("  Recovery Attempts: %d\n", bypassInfo.RecoveryAttempts)

	if !bypassInfo.LastRecoveryTime.IsZero() {
		fmt.Printf("  Last Recovery Try: %s\n", bypassInfo.LastRecoveryTime.Format(time.RFC3339))
		fmt.Printf("  Time Since Last Try: %s\n", formatDuration(time.Since(bypassInfo.LastRecoveryTime)))
	}

	if bypassVerbose {
		fmt.Printf("\nRecommendations:\n")
		if bypassInfo.UserBypassDecision {
			fmt.Printf("  - This is a manual bypass; use 'lsp-gateway bypass clear %s' to re-enable\n", serverName)
		} else if canAttempt {
			fmt.Printf("  - Server is eligible for automatic recovery\n")
			fmt.Printf("  - Recovery will be attempted on next health check\n")
		} else {
			fmt.Printf("  - Server is in cooldown period after previous recovery attempts\n")
			fmt.Printf("  - Wait before attempting recovery again\n")
		}
	}

	return nil
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	} else {
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func init() {
	// Set up flags
	bypassCmd.PersistentFlags().BoolVar(&bypassJSON, "json", false, "Output in JSON format")
	bypassCmd.PersistentFlags().BoolVarP(&bypassVerbose, "verbose", "v", false, "Verbose output")

	bypassSetCmd.Flags().BoolVar(&bypassForce, "force", false, "Force bypass even if server is healthy")
	bypassSetCmd.Flags().StringVar(&bypassReason, "reason", "", "Reason for bypassing the server (required)")
	bypassSetCmd.Flags().StringVar(&bypassStrategy, "strategy", "", "Bypass strategy (user_manual, auto_consecutive_failures, auto_circuit_breaker, auto_health_degraded)")

	bypassClearCmd.Flags().BoolVar(&bypassForce, "force", false, "Force clear even if server might not be ready")

	// Add subcommands
	bypassCmd.AddCommand(bypassListCmd)
	bypassCmd.AddCommand(bypassSetCmd)
	bypassCmd.AddCommand(bypassClearCmd)
	bypassCmd.AddCommand(bypassStatsCmd)
	bypassCmd.AddCommand(bypassRecoveryCmd)

	// Register with root command
	rootCmd.AddCommand(bypassCmd)
}