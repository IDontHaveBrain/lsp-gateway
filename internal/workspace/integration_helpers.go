package workspace

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lsp-gateway/internal/project"
	"lsp-gateway/tests/e2e/testutils"
)

// Integration helper functions for workspace testing

// CreateTemporaryWorkspace creates a temporary workspace directory with proper structure
func CreateTemporaryWorkspace(languages []string) (string, error) {
	tempDir, err := os.MkdirTemp("", "lspg-workspace-test-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	
	// Create language-specific subdirectories
	for _, lang := range languages {
		langDir := filepath.Join(tempDir, lang)
		if err := os.MkdirAll(langDir, 0755); err != nil {
			os.RemoveAll(tempDir)
			return "", fmt.Errorf("failed to create language directory %s: %w", lang, err)
		}
		
		// Setup project files for each language
		if err := SetupWorkspaceProjectFiles(langDir, lang); err != nil {
			os.RemoveAll(tempDir)
			return "", fmt.Errorf("failed to setup project files for %s: %w", lang, err)
		}
	}
	
	// Create common workspace files
	if err := createCommonWorkspaceFiles(tempDir, languages); err != nil {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("failed to create common workspace files: %w", err)
	}
	
	return tempDir, nil
}

// SetupWorkspaceProjectFiles creates realistic project files for a specific language
func SetupWorkspaceProjectFiles(workspaceDir string, lang string) error {
	projectFiles := GetTestProjectFiles(lang)
	if len(projectFiles) == 0 {
		return fmt.Errorf("no project files defined for language: %s", lang)
	}
	
	for relativePath, content := range projectFiles {
		fullPath := filepath.Join(workspaceDir, relativePath)
		
		// Ensure parent directory exists
		parentDir := filepath.Dir(fullPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", parentDir, err)
		}
		
		// Write file content
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", fullPath, err)
		}
	}
	
	return nil
}

// WaitForPortAvailability waits for a port to become available with exponential backoff
func WaitForPortAvailability(port int, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	pollConfig := testutils.FastPollingConfig{
		InitialInterval:   100 * time.Millisecond,
		MaxInterval:       2 * time.Second,
		BackoffFactor:     1.5,
		StartupTimeout:    timeout,
		FastFailTimeout:   5 * time.Second,
		LogProgress:       false,
		EnableFastFail:    true,
		AdaptivePolling:   true,
		MinSystemInterval: 50 * time.Millisecond,
	}
	
	interval := pollConfig.InitialInterval
	for {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), time.Second)
		if err != nil {
			// Port is not available (which is what we want)
			return nil
		}
		conn.Close()
		
		// Check for timeout
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for port %d to become available", port)
		default:
		}
		
		// Wait before next attempt with exponential backoff
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for port %d to become available", port)
		case <-time.After(interval):
		}
		
		// Apply exponential backoff
		if pollConfig.BackoffFactor > 1.0 {
			newInterval := time.Duration(float64(interval) * pollConfig.BackoffFactor)
			if newInterval > pollConfig.MaxInterval {
				interval = pollConfig.MaxInterval
			} else if newInterval < pollConfig.MinSystemInterval {
				interval = pollConfig.MinSystemInterval
			} else {
				interval = newInterval
			}
		}
	}
}

// ValidateJSONRPCResponse validates that a response follows JSON-RPC 2.0 specification
func ValidateJSONRPCResponse(response []byte) error {
	if len(response) == 0 {
		return fmt.Errorf("empty response")
	}
	
	// Parse as JSON-RPC message using the common message parser
	msg, err := testutils.ParseMCPMessage(response)
	if err != nil {
		return fmt.Errorf("invalid JSON format: %w", err)
	}
	
	// Validate JSON-RPC version
	if msg.Jsonrpc != "2.0" {
		return fmt.Errorf("invalid JSON-RPC version: expected '2.0', got '%s'", msg.Jsonrpc)
	}
	
	// Must have either result or error, but not both
	hasResult := msg.Result != nil
	hasError := msg.Error != nil
	
	if !hasResult && !hasError {
		return fmt.Errorf("response must have either 'result' or 'error' field")
	}
	
	if hasResult && hasError {
		return fmt.Errorf("response cannot have both 'result' and 'error' fields")
	}
	
	// Validate ID field (should match request ID)
	if msg.ID == nil {
		return fmt.Errorf("response missing 'id' field")
	}
	
	return nil
}

// CreateWorkspaceConfig creates a workspace configuration for testing
func CreateWorkspaceConfig(languages []string) (*WorkspaceConfig, error) {
	if len(languages) == 0 {
		return nil, fmt.Errorf("at least one language must be specified")
	}
	
	// Create temporary workspace for config generation
	tempWorkspace, err := CreateTemporaryWorkspace(languages)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary workspace: %w", err)
	}
	defer os.RemoveAll(tempWorkspace)
	
	// Create project context
	projectContext := &project.ProjectContext{
		ProjectType:    determineProjectType(languages),
		Languages:      languages,
		RootPath:       tempWorkspace,
		ConfigFiles:    make([]string, 0),
		Dependencies:   make(map[string]string),
		BuildSystem:    determineBuildSystem(languages),
		PackageManager: determinePackageManager(languages),
		IsMonorepo:     len(languages) > 1,
	}
	
	// Create config manager and generate configuration
	configManager := NewWorkspaceConfigManager()
	if err := configManager.GenerateWorkspaceConfig(tempWorkspace, projectContext); err != nil {
		return nil, fmt.Errorf("failed to generate workspace config: %w", err)
	}
	
	// Load and return the generated configuration
	workspaceConfig, err := configManager.LoadWorkspaceConfig(tempWorkspace)
	if err != nil {
		return nil, fmt.Errorf("failed to load generated workspace config: %w", err)
	}
	
	return workspaceConfig, nil
}

// StartMockLSPServers starts mock LSP servers for all specified languages
func StartMockLSPServers(languages []string) (map[string]*MockLSPServer, error) {
	servers := make(map[string]*MockLSPServer)
	
	for _, lang := range languages {
		mockServer := NewMockLSPServer(lang)
		
		if err := mockServer.Start(); err != nil {
			// Clean up already started servers
			StopMockLSPServers(servers)
			return nil, fmt.Errorf("failed to start mock LSP server for %s: %w", lang, err)
		}
		
		servers[lang] = mockServer
	}
	
	// Wait a moment for all servers to be ready
	time.Sleep(100 * time.Millisecond)
	
	return servers, nil
}

// StopMockLSPServers stops all mock LSP servers gracefully
func StopMockLSPServers(servers map[string]*MockLSPServer) {
	for lang, server := range servers {
		if server != nil {
			if err := server.Stop(); err != nil {
				fmt.Printf("Warning: failed to stop mock LSP server for %s: %v\n", lang, err)
			}
		}
	}
}

// MeasureWorkspaceStartupTime measures how long it takes for a workspace to start
func MeasureWorkspaceStartupTime(gateway WorkspaceGateway) time.Duration {
	startTime := time.Now()
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Start the gateway
	if err := gateway.Start(ctx); err != nil {
		return 0 // Return 0 duration on error
	}
	
	return time.Since(startTime)
}

// ValidateWorkspaceHealth performs comprehensive health validation of a workspace
func ValidateWorkspaceHealth(gateway WorkspaceGateway) error {
	health := gateway.Health()
	
	// Check overall health status
	if !health.IsHealthy {
		return fmt.Errorf("workspace is not healthy: %v", health.Errors)
	}
	
	// Check that we have active clients
	if health.ActiveClients == 0 {
		return fmt.Errorf("no active LSP clients")
	}
	
	// Validate individual client statuses
	for language, status := range health.ClientStatuses {
		if !status.IsActive {
			return fmt.Errorf("LSP client for %s is not active: %s", language, status.Error)
		}
		
		// Check if client was recently used (within reasonable time)
		if time.Since(status.LastUsed) > 5*time.Minute {
			return fmt.Errorf("LSP client for %s has not been used recently", language)
		}
	}
	
	// Check for any error messages
	if len(health.Errors) > 0 {
		return fmt.Errorf("workspace has errors: %v", health.Errors)
	}
	
	return nil
}

// CreateTestWorkspaceDirectories creates the expected directory structure for workspace testing
func CreateTestWorkspaceDirectories(baseDir string, languages []string) error {
	// Create base directory
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create base directory: %w", err)
	}
	
	// Create language-specific directories
	for _, lang := range languages {
		langDir := filepath.Join(baseDir, lang)
		if err := os.MkdirAll(langDir, 0755); err != nil {
			return fmt.Errorf("failed to create language directory %s: %w", lang, err)
		}
		
		// Create standard subdirectories for each language
		subdirs := getLanguageSubdirectories(lang)
		for _, subdir := range subdirs {
			fullPath := filepath.Join(langDir, subdir)
			if err := os.MkdirAll(fullPath, 0755); err != nil {
				return fmt.Errorf("failed to create subdirectory %s: %w", fullPath, err)
			}
		}
	}
	
	// Create common workspace directories
	commonDirs := []string{"logs", "cache", "tmp", "config"}
	for _, dir := range commonDirs {
		fullPath := filepath.Join(baseDir, dir)
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			return fmt.Errorf("failed to create common directory %s: %w", fullPath, err)
		}
	}
	
	return nil
}

// ValidateWorkspaceStructure ensures the workspace has the expected directory structure
func ValidateWorkspaceStructure(workspaceDir string, expectedLanguages []string) error {
	// Check that workspace directory exists
	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		return fmt.Errorf("workspace directory does not exist: %s", workspaceDir)
	}
	
	// Check language directories
	for _, lang := range expectedLanguages {
		langDir := filepath.Join(workspaceDir, lang)
		if _, err := os.Stat(langDir); os.IsNotExist(err) {
			return fmt.Errorf("language directory does not exist: %s", langDir)
		}
		
		// Check for expected project files
		projectFiles := GetTestProjectFiles(lang)
		for relativePath := range projectFiles {
			fullPath := filepath.Join(langDir, relativePath)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				return fmt.Errorf("expected project file does not exist: %s", fullPath)
			}
		}
	}
	
	return nil
}

// Helper functions

// createCommonWorkspaceFiles creates files that are common across all workspace types
func createCommonWorkspaceFiles(workspaceDir string, languages []string) error {
	// Create README.md
	readmeContent := fmt.Sprintf("# Test Workspace\n\nThis is a test workspace containing the following languages:\n\n%s\n", 
		formatLanguageList(languages))
	
	readmePath := filepath.Join(workspaceDir, "README.md")
	if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
		return fmt.Errorf("failed to create README.md: %w", err)
	}
	
	// Create .gitignore
	gitignoreContent := generateGitignoreContent(languages)
	gitignorePath := filepath.Join(workspaceDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
		return fmt.Errorf("failed to create .gitignore: %w", err)
	}
	
	return nil
}

// formatLanguageList formats a list of languages for documentation
func formatLanguageList(languages []string) string {
	var formatted []string
	for _, lang := range languages {
		formatted = append(formatted, fmt.Sprintf("- %s", strings.Title(lang)))
	}
	return strings.Join(formatted, "\n")
}

// generateGitignoreContent generates appropriate .gitignore content for the languages
func generateGitignoreContent(languages []string) string {
	gitignoreRules := []string{
		"# General",
		".DS_Store",
		"Thumbs.db",
		"*.log",
		"*.tmp",
		"",
		"# LSP Gateway",
		".lspg/",
		".lspg-port",
		"",
	}
	
	// Add language-specific ignore rules
	for _, lang := range languages {
		rules := getLanguageGitignoreRules(lang)
		if len(rules) > 0 {
			gitignoreRules = append(gitignoreRules, fmt.Sprintf("# %s", strings.Title(lang)))
			gitignoreRules = append(gitignoreRules, rules...)
			gitignoreRules = append(gitignoreRules, "")
		}
	}
	
	return strings.Join(gitignoreRules, "\n")
}

// getLanguageGitignoreRules returns gitignore rules for a specific language
func getLanguageGitignoreRules(language string) []string {
	rules := map[string][]string{
		"go": {
			"*.exe",
			"*.exe~",
			"*.dll",
			"*.so",
			"*.dylib",
			"vendor/",
			"go.work",
		},
		"python": {
			"__pycache__/",
			"*.py[cod]",
			"*$py.class",
			"*.so",
			".Python",
			"venv/",
			"env/",
			".env",
		},
		"javascript": {
			"node_modules/",
			"npm-debug.log*",
			"yarn-debug.log*",
			"yarn-error.log*",
			".npm",
		},
		"typescript": {
			"node_modules/",
			"*.js.map",
			"*.d.ts.map",
			"dist/",
			"build/",
		},
		"java": {
			"*.class",
			"*.jar",
			"*.war",
			"*.ear",
			"target/",
			".m2/",
		},
		"rust": {
			"target/",
			"Cargo.lock",
			"*.pdb",
		},
	}
	
	if langRules, exists := rules[language]; exists {
		return langRules
	}
	return []string{}
}

// getLanguageSubdirectories returns standard subdirectories for a language
func getLanguageSubdirectories(language string) []string {
	subdirs := map[string][]string{
		"go": {"cmd", "pkg", "internal", "vendor"},
		"python": {"src", "tests", "docs"},
		"javascript": {"src", "test", "dist", "node_modules"},
		"typescript": {"src", "test", "dist", "node_modules"},
		"java": {"src/main/java", "src/test/java", "target"},
		"rust": {"src", "tests", "target"},
		"c": {"src", "include", "build"},
		"cpp": {"src", "include", "build"},
	}
	
	if dirs, exists := subdirs[language]; exists {
		return dirs
	}
	return []string{"src", "test"}
}

// determineProjectType determines the project type based on languages
func determineProjectType(languages []string) string {
	if len(languages) == 1 {
		return fmt.Sprintf("single-%s", languages[0])
	} else if len(languages) <= 3 {
		return "multi-language"
	} else {
		return "polyglot"
	}
}

// determineBuildSystems determines build systems based on languages
func determineBuildSystem(languages []string) string {
	buildSystems := make(map[string]bool)
	
	for _, lang := range languages {
		switch lang {
		case "go":
			buildSystems["go"] = true
		case "python":
			buildSystems["pip"] = true
		case "javascript", "typescript":
			buildSystems["npm"] = true
		case "java":
			buildSystems["maven"] = true
		case "rust":
			buildSystems["cargo"] = true
		case "c", "cpp":
			buildSystems["make"] = true
		}
	}
	
	// Return the first build system found, or empty string if none
	for system := range buildSystems {
		return system
	}
	
	return ""
}

// determinePackageManager determines package manager based on languages
func determinePackageManager(languages []string) string {
	managers := make(map[string]bool)
	
	for _, lang := range languages {
		switch lang {
		case "go":
			managers["go mod"] = true
		case "python":
			managers["pip"] = true
		case "javascript", "typescript":
			managers["npm"] = true
		case "java":
			managers["maven"] = true
		case "rust":
			managers["cargo"] = true
		}
	}
	
	// Return the first package manager found, or empty string if none
	for manager := range managers {
		return manager
	}
	
	return ""
}