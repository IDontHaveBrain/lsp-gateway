package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"lsp-gateway/tests/integration/config/helpers"
)

func TestCLIHelperBasic(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "cli-helper-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create CLI helper
	cliHelper := helpers.NewCLITestHelper(tempDir)

	// Create simple Go project
	projectStructure := map[string]string{
		"go.mod":  "module test-project\n\ngo 1.21",
		"main.go": "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}",
	}
	
	// Create project structure
	for filePath, content := range projectStructure {
		fullPath := filepath.Join(tempDir, filePath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	// Test CLI command execution
	fmt.Println("Testing CLI command execution...")
	result, err := cliHelper.RunCommand("config", "generate", "--project-path", tempDir)
	if err != nil {
		t.Fatalf("CLI command failed: %v", err)
	}

	fmt.Printf("Exit Code: %d\n", result.ExitCode)
	fmt.Printf("Stdout: %s\n", result.Stdout)
	fmt.Printf("Stderr: %s\n", result.Stderr)

	// Check if it's not a mock response
	if result.Stdout == "Mock output for command: [config generate --project-path "+tempDir+"]" {
		t.Fatal("Still getting mock output instead of real CLI execution")
	}

	// Check if config file was generated
	configPath := filepath.Join(tempDir, "lsp-gateway.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("Config file not created at: %s\n", configPath)
		fmt.Printf("Available files in temp dir:\n")
		entries, _ := os.ReadDir(tempDir)
		for _, entry := range entries {
			fmt.Printf("  - %s\n", entry.Name())
		}
	} else {
		fmt.Printf("Config file successfully created at: %s\n", configPath)
		// Read and display config content
		content, _ := os.ReadFile(configPath)
		fmt.Printf("Config content:\n%s\n", string(content))
	}

	fmt.Println("CLI Helper test completed")
}

func main() {
	testing.Main(
		func(pat, str string) (bool, error) { return true, nil },
		[]testing.InternalTest{
			{"TestCLIHelperBasic", TestCLIHelperBasic},
		},
		nil,
		nil,
	)
}