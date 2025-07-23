package main

import (
	"fmt"
	"os"

	"lsp-gateway/internal/cli"
)

// RunMain executes the main application logic and returns the exit code
// This function is exported for testing purposes
func RunMain() int {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

func main() {
	exitCode := RunMain()
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}
