package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"lsp-gateway/internal/installer"

	"github.com/spf13/cobra"
)

var (
	verifyJSON    bool
	verifyVerbose bool
	verifyAll     bool
)

var verifyCmd = &cobra.Command{
	Use:   CmdVerify,
	Short: "Verify runtime installations and system requirements",
	Long: `Verify command checks the installation status and functionality of
programming language runtimes and their associated tools.

Available verify targets:
- runtime <name>: Verify specific runtime installation
- runtime all:    Verify all supported runtimes

The verification process includes:
1. Check if the runtime is installed and accessible
2. Verify the runtime version meets minimum requirements
3. Test basic functionality (compilation, execution)
4. Check for common configuration issues
5. Validate compatibility with LSP Gateway

Examples:
  lsp-gateway verify runtime go       # Verify Go installation
  lsp-gateway verify runtime python   # Verify Python installation
  lsp-gateway verify runtime all      # Verify all runtimes
  lsp-gateway verify runtime go --verbose  # Detailed verification output`,
}

var verifyRuntimeCmd = &cobra.Command{
	Use:   "runtime <name|all>",
	Short: "Verify programming language runtime installations",
	Long: `Verify that programming language runtimes are properly installed and functional.

Supported runtimes for verification:
- go:     Go programming language installation and tools
- python: Python installation, pip, and essential packages
- nodejs: Node.js runtime and npm package manager
- java:   Java Development Kit and runtime environment
- all:    Verify all supported runtimes

Verification checks:
- Runtime accessibility (PATH configuration)
- Version compatibility with LSP Gateway requirements
- Basic functionality (compile/execute simple programs)
- Associated tools availability (package managers, etc.)
- Common configuration issues

Exit codes:
- 0: All verifications passed
- 1: Some verifications failed
- 2: Critical verification failures`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"go", "python", "nodejs", "java", "all"},
	RunE:      runVerifyRuntime,
}

func init() {
	verifyRuntimeCmd.Flags().BoolVar(&verifyJSON, "json", false, "Output results in JSON format")
	verifyRuntimeCmd.Flags().BoolVarP(&verifyVerbose, "verbose", "v", false, "Show detailed verification information")
	verifyRuntimeCmd.Flags().BoolVar(&verifyAll, "all", false, "Verify all runtimes (equivalent to 'runtime all')")

	verifyCmd.AddCommand(verifyRuntimeCmd)

	rootCmd.AddCommand(verifyCmd)
}

// GetVerifyRuntimeCmd returns the verify runtime command for testing purposes
func GetVerifyRuntimeCmd() *cobra.Command {
	return verifyRuntimeCmd
}

func runVerifyRuntime(cmd *cobra.Command, args []string) error {
	runtimeName := args[0]

	if verifyAll && runtimeName != allRuntimesKeyword {
		runtimeName = allRuntimesKeyword
	}

	runtimeInstaller := installer.NewRuntimeInstaller()
	if runtimeInstaller == nil {
		return fmt.Errorf("failed to create runtime installer")
	}

	var results []RuntimeVerificationResult
	var verifyError error

	if runtimeName == "all" {
		results, verifyError = verifyAllRuntimes(runtimeInstaller)
	} else {
		result, err := verifySingleRuntime(runtimeInstaller, runtimeName)
		if result.Result != nil {
			results = []RuntimeVerificationResult{result}
		}
		verifyError = err
	}

	if verifyJSON {
		return outputVerifyResultsJSON(results, verifyError)
	} else {
		return outputVerifyResultsHuman(results, verifyError)
	}
}

func verifyAllRuntimes(installer *installer.DefaultRuntimeInstaller) ([]RuntimeVerificationResult, error) {
	supportedRuntimes := installer.GetSupportedRuntimes()
	results := make([]RuntimeVerificationResult, 0, len(supportedRuntimes))
	var firstError error

	if !verifyJSON {
		fmt.Printf("Verifying all runtime installations...\n\n")
	}

	for i, runtimeName := range supportedRuntimes {
		if !verifyJSON && verifyVerbose {
			fmt.Printf("[%d/%d] Verifying %s runtime...\n", i+1, len(supportedRuntimes), runtimeName)
		}

		result, err := installer.Verify(runtimeName)

		runtimeResult := RuntimeVerificationResult{
			Runtime: runtimeName,
			Result:  result,
		}
		results = append(results, runtimeResult)

		if err != nil && firstError == nil {
			firstError = err
		}

		if !verifyJSON {
			displaySingleVerificationResult(runtimeName, result, err)
		}
	}

	if !verifyJSON {
		displayVerificationSummary(results)
	}

	return results, firstError
}

func verifySingleRuntime(runtimeInstaller *installer.DefaultRuntimeInstaller, runtimeName string) (RuntimeVerificationResult, error) {
	supportedRuntimes := runtimeInstaller.GetSupportedRuntimes()
	isSupported := false
	for _, supported := range supportedRuntimes {
		if supported == runtimeName {
			isSupported = true
			break
		}
	}

	if !isSupported {
		return RuntimeVerificationResult{}, installer.NewInstallerError(
			installer.InstallerErrorTypeNotFound,
			runtimeName,
			fmt.Sprintf("unsupported runtime '%s'. Supported runtimes: %s", runtimeName, strings.Join(supportedRuntimes, ", ")),
			nil,
		)
	}

	if !verifyJSON {
		fmt.Printf("Verifying %s runtime installation...\n", runtimeName)
	}

	result, err := runtimeInstaller.Verify(runtimeName)

	if !verifyJSON {
		displaySingleVerificationResult(runtimeName, result, err)
	}

	return RuntimeVerificationResult{
		Runtime: runtimeName,
		Result:  result,
	}, err
}

func displaySingleVerificationResult(runtimeName string, result *installer.VerificationResult, err error) {
	if err != nil {
		fmt.Printf("✗ %s: Verification failed - %v\n", runtimeName, err)
		return
	}

	if result == nil {
		fmt.Printf("✗ %s: No verification result available\n", runtimeName)
		return
	}

	if result.Installed && result.Compatible {
		fmt.Printf("✓ %s: Installed and working correctly\n", runtimeName)
	} else if result.Installed {
		if !result.Compatible {
			fmt.Printf("⚠ %s: Installed but version incompatible\n", runtimeName)
		} else {
			fmt.Printf("⚠ %s: Installed with issues\n", runtimeName)
		}
	} else {
		fmt.Printf("✗ %s: Not installed\n", runtimeName)
	}

	if verifyVerbose {
		if result.Version != "" {
			fmt.Printf("  Version: %s\n", result.Version)
		}
		if result.Path != "" {
			fmt.Printf("  Path: %s\n", result.Path)
		}

		if len(result.Issues) > 0 {
			fmt.Printf("  Issues:\n")
			for _, issue := range result.Issues {
				fmt.Printf("    - %s\n", issue)
			}
		}
	}

	fmt.Println()
}

func displayVerificationSummary(results []RuntimeVerificationResult) {
	fmt.Printf("Verification Summary:\n")
	fmt.Printf("====================\n")

	workingCount := 0
	installedCount := 0
	compatibleCount := 0

	for _, runtimeResult := range results {
		if runtimeResult.Result != nil {
			if runtimeResult.Result.Installed {
				installedCount++
			}
			if runtimeResult.Result.Compatible {
				workingCount++
			}
			if runtimeResult.Result.Compatible {
				compatibleCount++
			}
		}
	}

	total := len(results)
	fmt.Printf("Total runtimes checked: %d\n", total)
	fmt.Printf("Installed: %d/%d\n", installedCount, total)
	fmt.Printf("Working: %d/%d\n", workingCount, total)
	fmt.Printf("Compatible: %d/%d\n", compatibleCount, total)

	if workingCount == total {
		fmt.Printf("\n✓ All runtimes are working correctly!\n")
	} else if installedCount == total {
		fmt.Printf("\n⚠ All runtimes are installed but some have issues\n")
	} else {
		fmt.Printf("\n✗ Some runtimes are missing or have issues\n")
		fmt.Printf("Run 'lsp-gateway install runtime all' to install missing runtimes\n")
	}

	hasIssues := false
	for _, runtimeResult := range results {
		if runtimeResult.Result != nil && len(runtimeResult.Result.Issues) > 0 {
			if !hasIssues {
				fmt.Printf("\nIssues found:\n")
				hasIssues = true
			}
			issueStrings := make([]string, len(runtimeResult.Result.Issues))
			for i, issue := range runtimeResult.Result.Issues {
				issueStrings[i] = issue.Description
			}
			fmt.Printf("- %s: %s\n", runtimeResult.Runtime, strings.Join(issueStrings, ", "))
		}
	}
}

func outputVerifyResultsJSON(results []RuntimeVerificationResult, verifyError error) error {
	total := len(results)
	installedCount := 0
	workingCount := 0
	compatibleCount := 0

	for _, runtimeResult := range results {
		if runtimeResult.Result != nil {
			if runtimeResult.Result.Installed {
				installedCount++
			}
			if runtimeResult.Result.Compatible {
				workingCount++
			}
			if runtimeResult.Result.Compatible {
				compatibleCount++
			}
		}
	}

	jsonResults := make([]VerificationResultJSON, len(results))

	for i, runtimeResult := range results {
		if runtimeResult.Result != nil {
			jsonResults[i] = VerificationResultJSON{
				Runtime:    runtimeResult.Runtime,
				Installed:  runtimeResult.Result.Installed,
				Version:    runtimeResult.Result.Version,
				Path:       runtimeResult.Result.Path,
				Working:    runtimeResult.Result.Compatible,
				Compatible: runtimeResult.Result.Compatible,
				Issues: func() []string {
					issueStrings := make([]string, len(runtimeResult.Result.Issues))
					for i, issue := range runtimeResult.Result.Issues {
						issueStrings[i] = issue.Description
					}
					return issueStrings
				}(),
			}
		} else {
			jsonResults[i] = VerificationResultJSON{
				Runtime:    runtimeResult.Runtime,
				Installed:  false,
				Version:    "",
				Path:       "",
				Working:    false,
				Compatible: false,
				Issues:     []string{"verification failed"},
			}
		}
	}

	output := struct {
		Success bool                     `json:"success"`
		Summary VerificationSummary      `json:"summary"`
		Results []VerificationResultJSON `json:"results"`
		Error   string                   `json:"error,omitempty"`
	}{
		Success: verifyError == nil && workingCount == total,
		Summary: VerificationSummary{
			Total:      total,
			Installed:  installedCount,
			Working:    workingCount,
			Compatible: compatibleCount,
		},
		Results: jsonResults,
	}

	if verifyError != nil {
		output.Error = verifyError.Error()
	}

	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON output: %w", err)
	}

	fmt.Println(string(jsonData))

	if verifyError != nil || workingCount < total {
		return fmt.Errorf("verification failed for some runtimes")
	}

	return nil
}

func outputVerifyResultsHuman(results []RuntimeVerificationResult, verifyError error) error {
	if verifyError != nil {
		return verifyError
	}

	for _, runtimeResult := range results {
		if runtimeResult.Result == nil || !runtimeResult.Result.Compatible {
			return fmt.Errorf("some runtime verifications failed")
		}
	}

	return nil
}

type RuntimeVerificationResult struct {
	Runtime string
	Result  *installer.VerificationResult
}

type VerificationSummary struct {
	Total      int `json:"total"`
	Installed  int `json:"installed"`
	Working    int `json:"working"`
	Compatible int `json:"compatible"`
}

type VerificationResultJSON struct {
	Runtime    string   `json:"runtime"`
	Installed  bool     `json:"installed"`
	Version    string   `json:"version"`
	Path       string   `json:"path"`
	Working    bool     `json:"working"`
	Compatible bool     `json:"compatible"`
	Issues     []string `json:"issues"`
}
