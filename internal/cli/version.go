package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Show version information for LSP Gateway.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("LSP Gateway MVP\n")
		fmt.Printf("Go Version: %s\n", runtime.Version())
		fmt.Printf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		return nil
	},
}

func init() {
	// Add version command to root
	rootCmd.AddCommand(versionCmd)
}
