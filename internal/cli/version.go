package cli

import (
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

var (
	Version     = "0.0.1"
	GitCommit   = ERROR_UNKNOWN_VALUE
	GitBranch   = ERROR_UNKNOWN_VALUE
	BuildTime   = ERROR_UNKNOWN_VALUE
	BuildUser   = ERROR_UNKNOWN_VALUE
	VersionJSON bool
)

type VersionInfo struct {
	Version      string `json:"version"`
	GitCommit    string `json:"git_commit"`
	GitBranch    string `json:"git_branch"`
	BuildTime    string `json:"build_time"`
	BuildUser    string `json:"build_user"`
	GoVersion    string `json:"go_version"`
	Platform     string `json:"platform"`
	Architecture string `json:"architecture"`
	Compiler     string `json:"compiler"`
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version and build information",
	Long: `Display comprehensive version and build information for LSP Gateway.

📦 VERSION INFORMATION:
This command shows detailed version information including:
- LSP Gateway version and build details
- Git commit and branch information  
- Build environment and timestamp
- Go runtime and platform details
- Architecture and compiler information

💻 USAGE EXAMPLES:

  Basic version info:
    lsp-gateway version                    # Human-readable version information
    
  JSON output for automation:
    lsp-gateway version --json             # Machine-readable JSON format
    
  Integration examples:
    VERSION=$(lsp-gateway version --json | jq -r '.version')
    lsp-gateway version | grep "Version:"  # Extract specific information

🔧 BUILD INFORMATION:
Version information can be customized at build time using:
  go build -ldflags "-X 'lsp-gateway/internal/cli.Version=v1.0.0'"

🚀 QUICK CHECKS:
  lsp-gateway version && lsp-gateway status  # Version + system status
  lsp-gateway version --json | jq           # Pretty-print JSON version info

For troubleshooting, include version output when reporting issues.`,
	RunE: runVersion,
}

func runVersion(_ *cobra.Command, args []string) error {
	versionInfo := VersionInfo{
		Version:      Version,
		GitCommit:    GitCommit,
		GitBranch:    GitBranch,
		BuildTime:    BuildTime,
		BuildUser:    BuildUser,
		GoVersion:    runtime.Version(),
		Platform:     runtime.GOOS,
		Architecture: runtime.GOARCH,
		Compiler:     runtime.Compiler,
	}

	if VersionJSON {
		jsonData, err := json.MarshalIndent(versionInfo, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal version information: %w", err)
		}
		fmt.Println(string(jsonData))
		return nil
	}

	fmt.Printf("LSP Gateway Version Information\n")
	fmt.Printf("===============================\n\n")
	fmt.Printf("Version:      %s\n", versionInfo.Version)

	if GitCommit != ERROR_UNKNOWN_VALUE {
		if len(GitCommit) > 7 {
			fmt.Printf("Git Commit:   %s (%s)\n", GitCommit[:7], GitCommit)
		} else {
			fmt.Printf("Git Commit:   %s\n", GitCommit)
		}
	}

	if GitBranch != ERROR_UNKNOWN_VALUE {
		fmt.Printf("Git Branch:   %s\n", GitBranch)
	}

	if BuildTime != ERROR_UNKNOWN_VALUE {
		if t, err := time.Parse(time.RFC3339, BuildTime); err == nil {
			fmt.Printf("Build Time:   %s\n", t.Format("2006-01-02 15:04:05 UTC"))
		} else {
			fmt.Printf("Build Time:   %s\n", BuildTime)
		}
	}

	if BuildUser != ERROR_UNKNOWN_VALUE {
		fmt.Printf("Build User:   %s\n", BuildUser)
	}

	fmt.Printf("\nRuntime Information:\n")
	fmt.Printf("Go Version:   %s\n", versionInfo.GoVersion)
	fmt.Printf("Platform:     %s\n", versionInfo.Platform)
	fmt.Printf("Architecture: %s\n", versionInfo.Architecture)
	fmt.Printf("Compiler:     %s\n", versionInfo.Compiler)

	fmt.Printf("\nFor support, include this version information when reporting issues.\n")
	return nil
}

func init() {
	versionCmd.Flags().BoolVar(&VersionJSON, "json", false, "Output version information in JSON format")

	rootCmd.AddCommand(versionCmd)
}
