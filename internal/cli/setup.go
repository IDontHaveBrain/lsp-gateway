package cli

import (
	"time"

	"github.com/spf13/cobra"
)

var (
	setupForce        bool
	setupInteractive  bool
	setupTimeout      time.Duration
	setupSkipRuntimes []string
	setupSkipServers  []string
	setupConfigPath   string
)

var setupCmd = &cobra.Command{
	Use:   CmdSetup,
	Short: "Setup and configure LSP Gateway",
	Long: `Manual setup guidance for the LSP Gateway.

The setup commands have been replaced with individual install and configuration commands
for more granular control. Use the following working commands instead:

MANUAL SETUP PROCESS:

1. Install Programming Language Runtimes:
   lsp-gateway install runtime go          # Install Go runtime
   lsp-gateway install runtime python      # Install Python runtime  
   lsp-gateway install runtime nodejs      # Install Node.js runtime
   lsp-gateway install runtime java        # Install Java runtime
   lsp-gateway install runtime all         # Install all runtimes

2. Install Language Servers:
   lsp-gateway install server gopls                    # Go language server
   lsp-gateway install server pylsp                    # Python language server
   lsp-gateway install server typescript-language-server  # TypeScript/JavaScript
   lsp-gateway install server jdtls                    # Java language server
   lsp-gateway install servers                         # Install all servers

3. Verify Installations:
   lsp-gateway verify runtime all          # Verify all runtimes
   lsp-gateway config validate             # Validate configuration

4. Start the Gateway:
   lsp-gateway server --config config.yaml # Start HTTP Gateway

For troubleshooting: lsp-gateway diagnose
For system status: lsp-gateway status`,
	RunE: setupAll, // Default to setupAll when no subcommand specified
}

var setupAllCmd = &cobra.Command{
	Use:   "all",
	Short: "Complete automated setup",
	Long: `Manual installation guidance instead of automated setup.

Automated setup has been replaced with granular install commands.
Use these working commands for complete installation:

1. INSTALL RUNTIMES:
   lsp-gateway install runtime all         # Install all runtimes
   # OR install specific runtimes:
   lsp-gateway install runtime go
   lsp-gateway install runtime python
   lsp-gateway install runtime nodejs
   lsp-gateway install runtime java

2. INSTALL LANGUAGE SERVERS:
   lsp-gateway install servers              # Install all language servers
   # OR install specific servers:
   lsp-gateway install server gopls
   lsp-gateway install server pylsp
   lsp-gateway install server typescript-language-server
   lsp-gateway install server jdtls

3. VERIFY INSTALLATION:
   lsp-gateway verify runtime all          # Verify runtime installations
   lsp-gateway config validate             # Validate configuration

4. START GATEWAY:
   lsp-gateway server --config config.yaml # Start HTTP Gateway

Use --force flag with install commands for reinstallation.
For help: lsp-gateway install --help`,
	RunE: setupAll,
}

var setupWizardCmd = &cobra.Command{
	Use:   "wizard",
	Short: "Interactive setup workflow",
	Long: `Manual configuration guidance instead of interactive wizard.

The interactive wizard has been replaced with individual commands.
Follow this step-by-step manual process:

STEP-BY-STEP SETUP GUIDE:

1. CHECK SYSTEM STATUS:
   lsp-gateway diagnose                     # System diagnostics
   lsp-gateway status runtimes              # Check installed runtimes
   lsp-gateway status servers               # Check language servers

2. INSTALL MISSING COMPONENTS:
   lsp-gateway install runtime <name>       # Install specific runtime
   lsp-gateway install server <name>        # Install specific server

3. VERIFY INSTALLATIONS:
   lsp-gateway verify runtime all           # Verify all runtimes work
   lsp-gateway config validate              # Validate configuration

4. TEST GATEWAY:
   lsp-gateway server --config config.yaml # Start HTTP Gateway

SUPPORTED RUNTIMES: go, python, nodejs, java
SUPPORTED SERVERS: gopls, pylsp, typescript-language-server, jdtls

For detailed help: lsp-gateway <command> --help
For troubleshooting: lsp-gateway diagnose`,
	RunE: setupWizard,
}

func init() {
	setupCmd.PersistentFlags().BoolVar(&setupForce, FLAG_FORCE, false, "Force reinstallation of existing components")
	setupCmd.PersistentFlags().BoolVar(&setupInteractive, "interactive", true, "Enable interactive prompts (can be disabled)")
	setupCmd.PersistentFlags().DurationVar(&setupTimeout, FLAG_TIMEOUT, 10*time.Minute, "Setup operation timeout")
	setupCmd.PersistentFlags().StringSliceVar(&setupSkipRuntimes, "skip-runtimes", []string{}, "Runtimes to skip during setup (go,python,nodejs,java)")
	setupCmd.PersistentFlags().StringSliceVar(&setupSkipServers, "skip-servers", []string{}, "Language servers to skip during setup")
	setupCmd.PersistentFlags().StringVarP(&setupConfigPath, "config", "c", DefaultConfigFile, "Configuration file path")

	setupWizardCmd.Flags().BoolVar(&setupInteractive, "no-interactive", false, "Disable interactive prompts (use defaults)")

	setupCmd.AddCommand(setupAllCmd)
	setupCmd.AddCommand(setupWizardCmd)

	rootCmd.AddCommand(setupCmd)
}

func setupAll(cmd *cobra.Command, args []string) error {

	cmd.Println("=======================================================")
	cmd.Println("  LSP Gateway Manual Setup Guide")
	cmd.Println("=======================================================")
	cmd.Println()
	cmd.Println("Automated setup has been replaced with individual commands.")
	cmd.Println("Follow these steps for complete installation:")
	cmd.Println()
	cmd.Println("1. INSTALL RUNTIMES:")
	cmd.Println("   lsp-gateway install runtime all         # Install all runtimes")
	cmd.Println("   # OR install specific: go, python, nodejs, java")
	cmd.Println()
	cmd.Println("2. INSTALL LANGUAGE SERVERS:")
	cmd.Println("   lsp-gateway install servers              # Install all servers")
	cmd.Println("   # OR install specific: gopls, pylsp, typescript-language-server, jdtls")
	cmd.Println()
	cmd.Println("3. VERIFY INSTALLATION:")
	cmd.Println("   lsp-gateway verify runtime all          # Verify runtimes")
	cmd.Println("   lsp-gateway config validate             # Validate config")
	cmd.Println()
	cmd.Println("4. START GATEWAY:")
	cmd.Println("   lsp-gateway server --config config.yaml # Start HTTP Gateway")
	cmd.Println()
	cmd.Printf("Configuration path: %s\n", setupConfigPath)

	if len(setupSkipRuntimes) > 0 {
		cmd.Printf("Note: Skip runtimes functionality replaced by selective install commands\n")
	}
	if len(setupSkipServers) > 0 {
		cmd.Printf("Note: Skip servers functionality replaced by selective install commands\n")
	}

	cmd.Println()
	cmd.Println("For help with any command: lsp-gateway <command> --help")
	cmd.Println("For troubleshooting: lsp-gateway diagnose")

	return nil
}

func setupWizard(cmd *cobra.Command, args []string) error {

	if cmd.Flags().Changed("no-interactive") {
		setupInteractive = false
	}

	cmd.Println("=======================================================")
	cmd.Println("  LSP Gateway Manual Setup Guide")
	cmd.Println("=======================================================")
	cmd.Println()
	cmd.Println("The interactive wizard has been replaced with manual commands.")
	cmd.Println("Follow this step-by-step process:")
	cmd.Println()

	cmd.Println("STEP 1: CHECK SYSTEM STATUS")
	cmd.Println("  lsp-gateway diagnose                     # System diagnostics")
	cmd.Println("  lsp-gateway status runtimes              # Check runtimes")
	cmd.Println("  lsp-gateway status servers               # Check servers")
	cmd.Println()

	cmd.Println("STEP 2: INSTALL MISSING COMPONENTS")
	cmd.Println("  Runtimes: lsp-gateway install runtime <go|python|nodejs|java|all>")
	cmd.Println("  Servers:  lsp-gateway install server <gopls|pylsp|typescript-language-server|jdtls>")
	cmd.Println("  Or:       lsp-gateway install servers   # Install all servers")
	cmd.Println()

	cmd.Println("STEP 3: VERIFY INSTALLATION")
	cmd.Println("  lsp-gateway verify runtime all          # Verify runtimes")
	cmd.Println("  lsp-gateway config validate              # Validate config")
	cmd.Println()

	cmd.Println("STEP 4: START GATEWAY")
	cmd.Println("  lsp-gateway server --config config.yaml # Start HTTP Gateway")
	cmd.Println()

	cmd.Printf("Configuration path: %s\n", setupConfigPath)
	cmd.Println()
	cmd.Println("For detailed help: lsp-gateway <command> --help")
	cmd.Println("For troubleshooting: lsp-gateway diagnose")

	return nil
}
