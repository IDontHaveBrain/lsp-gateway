package cli

import (
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"lsp-gateway/internal/version"
)

var (
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
    lspg version                    # Human-readable version information
    
  JSON output for automation:
    lspg version --json             # Machine-readable JSON format
    
  Integration examples:
    VERSION=$(lspg version --json | jq -r '.version')
    lspg version | grep "Version:"  # Extract specific information

🔧 BUILD INFORMATION:
Version information can be customized at build time using:
  go build -ldflags "-X 'lsp-gateway/internal/cli.Version=v1.0.0'"

🚀 QUICK CHECKS:
  lspg version && lspg status  # Version + system status
  lspg version --json | jq           # Pretty-print JSON version info

For troubleshooting, include version output when reporting issues.`,
	RunE: runVersion,
}

func runVersion(_ *cobra.Command, args []string) error {
	versionInfo := VersionInfo{
		Version:      version.Version,
		GitCommit:    version.GitCommit,
		GitBranch:    version.GitBranch,
		BuildTime:    version.BuildTime,
		BuildUser:    version.BuildUser,
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

	if version.GitCommit != "unknown" {
		if len(version.GitCommit) > 7 {
			fmt.Printf("Git Commit:   %s (%s)\n", version.GitCommit[:7], version.GitCommit)
		} else {
			fmt.Printf("Git Commit:   %s\n", version.GitCommit)
		}
	}

	if version.GitBranch != "unknown" {
		fmt.Printf("Git Branch:   %s\n", version.GitBranch)
	}

	if version.BuildTime != "unknown" {
		if t, err := time.Parse(time.RFC3339, version.BuildTime); err == nil {
			fmt.Printf("Build Time:   %s\n", t.Format("2006-01-02 15:04:05 UTC"))
		} else {
			fmt.Printf("Build Time:   %s\n", version.BuildTime)
		}
	}

	if version.BuildUser != "unknown" {
		fmt.Printf("Build User:   %s\n", version.BuildUser)
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
