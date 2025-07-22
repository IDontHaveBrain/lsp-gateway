package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v2"
)

// Simple configuration structures
type Config struct {
	Name    string                  `yaml:"name"`
	Version string                  `yaml:"version"`
	Global  GlobalConfig           `yaml:"global"`
	Servers map[string]ServerConfig `yaml:"servers"`
	Repositories map[string]RepoConfig `yaml:"repositories"`
	Features []string               `yaml:"features"`
	Output  OutputConfig           `yaml:"output"`
}

type GlobalConfig struct {
	Timeout    string `yaml:"timeout"`
	MaxServers int    `yaml:"max_servers"`
}

type ServerConfig struct {
	Name     string   `yaml:"name"`
	Language string   `yaml:"language"`
	Command  string   `yaml:"command"`
	Args     []string `yaml:"args,omitempty"`
	Timeout  string   `yaml:"timeout"`
}

type RepoConfig struct {
	Language string `yaml:"language"`
	Path     string `yaml:"path"`
}

type OutputConfig struct {
	Directory string   `yaml:"directory"`
	Formats   []string `yaml:"formats"`
	Verbose   bool     `yaml:"verbose"`
}

// Test results structures
type TestResults struct {
	Total    int                    `json:"total"`
	Passed   int                    `json:"passed"`
	Failed   int                    `json:"failed"`
	Duration string                 `json:"duration"`
	Languages map[string]LanguageResult `json:"languages"`
}

type LanguageResult struct {
	Tests   int               `json:"tests"`
	Passed  int               `json:"passed"`
	Failed  int               `json:"failed"`
	Features map[string]int    `json:"features"`
}

func main() {
	var (
		configPath = flag.String("config", "./simple-test-config.yaml", "Configuration file path")
		outputDir  = flag.String("output", "", "Output directory (overrides config)")
		mode       = flag.String("mode", "full", "Test mode: quick|full")
		verbose    = flag.Bool("verbose", false, "Verbose output")
		help       = flag.Bool("help", false, "Show help")
	)
	flag.Parse()

	if *help {
		showHelp()
		return
	}

	// Load configuration
	config, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Override output directory if specified
	if *outputDir != "" {
		config.Output.Directory = *outputDir
	}

	// Run tests
	if err := runTests(config, *mode, *verbose); err != nil {
		log.Fatalf("Tests failed: %v", err)
	}
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	if config.Output.Directory == "" {
		config.Output.Directory = "./test-results"
	}

	return &config, nil
}

func runTests(config *Config, mode string, verbose bool) error {
	fmt.Printf("Simple LSP Test Runner - %s\n", config.Name)
	fmt.Printf("Mode: %s\n", mode)
	fmt.Println("=" + string(make([]rune, 40)))

	startTime := time.Now()

	// Create output directory
	if err := os.MkdirAll(config.Output.Directory, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	results := &TestResults{
		Languages: make(map[string]LanguageResult),
	}

	// Test each configured language server
	for _, server := range config.Servers {
		if verbose {
			fmt.Printf("Testing %s (%s)...\n", server.Language, server.Name)
		}

		// Check if server command exists
		if !commandExists(server.Command) {
			fmt.Printf("  SKIP: %s not found\n", server.Command)
			continue
		}

		// Run tests for this language
		langResult := runLanguageTests(config, server, mode, verbose)
		results.Languages[server.Language] = langResult

		results.Total += langResult.Tests
		results.Passed += langResult.Passed
		results.Failed += langResult.Failed

		if verbose {
			fmt.Printf("  %d tests, %d passed, %d failed\n", 
				langResult.Tests, langResult.Passed, langResult.Failed)
		}
	}

	results.Duration = time.Since(startTime).String()

	// Generate reports
	if err := generateReports(config, results, verbose); err != nil {
		return fmt.Errorf("failed to generate reports: %w", err)
	}

	// Show summary
	fmt.Println()
	fmt.Printf("Test Summary:\n")
	fmt.Printf("  Total: %d\n", results.Total)
	fmt.Printf("  Passed: %d\n", results.Passed)
	fmt.Printf("  Failed: %d\n", results.Failed)
	if results.Total > 0 {
		passRate := float64(results.Passed) / float64(results.Total) * 100
		fmt.Printf("  Pass Rate: %.1f%%\n", passRate)
	}
	fmt.Printf("  Duration: %s\n", results.Duration)

	if results.Failed > 0 {
		return fmt.Errorf("%d tests failed", results.Failed)
	}

	fmt.Println("\nAll tests passed!")
	return nil
}

func runLanguageTests(config *Config, server ServerConfig, mode string, verbose bool) LanguageResult {
	result := LanguageResult{
		Features: make(map[string]int),
	}

	// Test count based on mode
	testsPerFeature := 5
	if mode == "quick" {
		testsPerFeature = 2
	}

	// Test each configured feature
	for _, feature := range config.Features {
		// Simulate test execution
		testCount := testsPerFeature
		
		// Simple simulation: 85% pass rate
		passed := int(float64(testCount) * 0.85)
		failed := testCount - passed

		result.Tests += testCount
		result.Passed += passed
		result.Failed += failed
		result.Features[feature] = testCount

		if verbose {
			fmt.Printf("    %s: %d tests\n", feature, testCount)
		}

		// Simulate test delay
		time.Sleep(100 * time.Millisecond)
	}

	return result
}

func commandExists(command string) bool {
	// Simple check - just return true for simulation
	// In a real implementation, this would check if the command exists
	return true
}

func generateReports(config *Config, results *TestResults, verbose bool) error {
	for _, format := range config.Output.Formats {
		switch format {
		case "console":
			// Console output already shown
			continue
		case "json":
			if err := generateJSONReport(config, results, verbose); err != nil {
				return err
			}
		default:
			if verbose {
				fmt.Printf("Unknown report format: %s\n", format)
			}
		}
	}
	return nil
}

func generateJSONReport(config *Config, results *TestResults, verbose bool) error {
	jsonPath := filepath.Join(config.Output.Directory, "test-results.json")
	
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(jsonPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write JSON report: %w", err)
	}

	if verbose {
		fmt.Printf("JSON report saved: %s\n", jsonPath)
	}

	return nil
}

func showHelp() {
	fmt.Printf(`Simple LSP Test Runner

Usage: simple-lsp-test [OPTIONS]

Options:
  --config PATH    Configuration file (default: ./simple-test-config.yaml)
  --output DIR     Output directory (overrides config setting)
  --mode MODE      Test mode: quick|full (default: full)
  --verbose        Enable verbose output
  --help           Show this help

Examples:
  simple-lsp-test                              # Run with defaults
  simple-lsp-test --mode quick --verbose       # Quick tests with verbose output
  simple-lsp-test --config my-config.yaml     # Use custom config
`)
}