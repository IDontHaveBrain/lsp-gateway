package main

import (
	"fmt"
	"os"

	"lsp-gateway/internal/cli"
)

// runMain executes the main application logic and returns the exit code
// This function is extracted for testing purposes
func runMain() int {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

func main() {
	exitCode := runMain()
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}
