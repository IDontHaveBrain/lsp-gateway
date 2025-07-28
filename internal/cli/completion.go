package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for LSP Gateway commands and flags.

🐚 SHELL COMPLETION:
Shell completion provides auto-complete functionality for commands, subcommands,
and flags when using LSP Gateway in your terminal. This greatly improves the
user experience by reducing typing and helping discover available options.

📋 SUPPORTED SHELLS:
  • bash      - Bash shell completion
  • zsh       - Zsh shell completion  
  • fish      - Fish shell completion
  • powershell - PowerShell completion

💻 USAGE EXAMPLES:

  Generate completion for your shell:
    lspg completion bash     # Generate bash completion
    lspg completion zsh      # Generate zsh completion
    lspg completion fish     # Generate fish completion
    lspg completion powershell # Generate PowerShell completion

🔧 INSTALLATION:

  Bash (add to ~/.bashrc):
    source <(lspg completion bash)
    
  Zsh (add to ~/.zshrc):
    source <(lspg completion zsh)
    
  Fish (save to ~/.config/fish/completions/):
    lspg completion fish > ~/.config/fish/completions/lsp-gateway.fish
    
  PowerShell (add to profile):
    lspg completion powershell | Out-String | Invoke-Expression

📝 PERSISTENT INSTALLATION:

  Save completion script to file:
    lspg completion bash > /etc/bash_completion.d/lsp-gateway
    lspg completion zsh > /usr/local/share/zsh/site-functions/_lsp-gateway
    
  For user-specific installation:
    mkdir -p ~/.local/share/bash-completion/completions
    lspg completion bash > ~/.local/share/bash-completion/completions/lsp-gateway

🚀 FEATURES:
  • Command and subcommand completion
  • Flag name completion
  • Runtime name completion (go, python, nodejs, java)
  • Server name completion (gopls, pylsp, etc.)
  • File path completion for config files
  • Context-aware suggestions

After installation, restart your shell or source your shell configuration.`,
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE:      runCompletion,
}

func runCompletion(cmd *cobra.Command, args []string) error {
	shell := args[0]

	switch shell {
	case "bash":
		return rootCmd.GenBashCompletion(os.Stdout)
	case "zsh":
		return rootCmd.GenZshCompletion(os.Stdout)
	case "fish":
		return rootCmd.GenFishCompletion(os.Stdout, true)
	case "powershell":
		return rootCmd.GenPowerShellCompletion(os.Stdout)
	default:
		return fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish, powershell)", shell)
	}
}

func setupCompletions() {
	configFileCompletion := func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"yaml", "yml"}, cobra.ShellCompDirectiveFilterFileExt
	}

	// Register flag completion functions
	// These will be called after all commands are initialized
	if serverCmd != nil {
		_ = serverCmd.RegisterFlagCompletionFunc("config", configFileCompletion)
	}

	if mcpCmd != nil {
		_ = mcpCmd.RegisterFlagCompletionFunc("config", configFileCompletion)
	}

	if ConfigCmd != nil {
		_ = ConfigCmd.RegisterFlagCompletionFunc("config", configFileCompletion)
	}
}

func init() {
	setupCompletions()

	rootCmd.AddCommand(completionCmd)
}
