package pooling

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lsp-gateway/internal/transport"
)

// WorkspaceSwitchStrategy defines the interface for language-specific workspace switching
type WorkspaceSwitchStrategy interface {
	// CanSwitch determines if this strategy can switch between the given workspaces
	CanSwitch(from, to string) bool
	
	// PreSwitch performs any necessary preparation before switching workspaces
	PreSwitch(client transport.LSPClient, from, to string) error
	
	// ExecuteSwitch performs the actual workspace switch using LSP protocol
	ExecuteSwitch(ctx context.Context, client transport.LSPClient, from, to string) error
	
	// PostSwitch performs any cleanup or verification after switching workspaces
	PostSwitch(client transport.LSPClient, from, to string) error
	
	// ValidateSwitch validates that the workspace switch was successful
	ValidateSwitch(ctx context.Context, client transport.LSPClient, to string) error
}

// LSP Protocol types for workspace management
type WorkspaceFolder struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

type DidChangeWorkspaceFoldersParams struct {
	Event WorkspaceFoldersChangeEvent `json:"event"`
}

type WorkspaceFoldersChangeEvent struct {
	Added   []WorkspaceFolder `json:"added"`
	Removed []WorkspaceFolder `json:"removed"`
}

// Base strategy with common functionality
type baseWorkspaceSwitchStrategy struct {
	logger Logger
}

// pathToURI converts a file path to a URI
func (b *baseWorkspaceSwitchStrategy) pathToURI(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for %s: %w", path, err)
	}
	
	// Convert to file URI
	uri := (&url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(absPath),
	}).String()
	
	return uri, nil
}

// sendWorkspaceFoldersNotification sends the standard LSP workspace/didChangeWorkspaceFolders notification
func (b *baseWorkspaceSwitchStrategy) sendWorkspaceFoldersNotification(
	ctx context.Context,
	client transport.LSPClient,
	from, to string,
) error {
	var removedFolders []WorkspaceFolder
	var addedFolders []WorkspaceFolder
	
	// Handle removal of old workspace
	if from != "" {
		fromURI, err := b.pathToURI(from)
		if err != nil {
			return fmt.Errorf("failed to convert from path to URI: %w", err)
		}
		
		removedFolders = append(removedFolders, WorkspaceFolder{
			URI:  fromURI,
			Name: filepath.Base(from),
		})
	}
	
	// Handle addition of new workspace
	toURI, err := b.pathToURI(to)
	if err != nil {
		return fmt.Errorf("failed to convert to path to URI: %w", err)
	}
	
	addedFolders = append(addedFolders, WorkspaceFolder{
		URI:  toURI,
		Name: filepath.Base(to),
	})
	
	// Create the notification params
	params := DidChangeWorkspaceFoldersParams{
		Event: WorkspaceFoldersChangeEvent{
			Added:   addedFolders,
			Removed: removedFolders,
		},
	}
	
	b.logger.Debug("Sending workspace/didChangeWorkspaceFolders notification: removed=%d, added=%d", 
		len(removedFolders), len(addedFolders))
	
	// Send the notification
	return client.SendNotification(ctx, "workspace/didChangeWorkspaceFolders", params)
}

// Java (JDTLS) Workspace Switch Strategy
type JavaWorkspaceSwitchStrategy struct {
	baseWorkspaceSwitchStrategy
}

func NewJavaWorkspaceSwitchStrategy(logger Logger) *JavaWorkspaceSwitchStrategy {
	return &JavaWorkspaceSwitchStrategy{
		baseWorkspaceSwitchStrategy: baseWorkspaceSwitchStrategy{logger: logger},
	}
}

func (j *JavaWorkspaceSwitchStrategy) CanSwitch(from, to string) bool {
	// Java JDTLS supports full workspace switching
	return j.isJavaProject(to)
}

func (j *JavaWorkspaceSwitchStrategy) isJavaProject(path string) bool {
	// Check for common Java project markers
	markers := []string{"pom.xml", "build.gradle", "build.gradle.kts", ".project", "src/main/java"}
	
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(path, marker)); err == nil {
			return true
		}
	}
	
	return false
}

func (j *JavaWorkspaceSwitchStrategy) PreSwitch(client transport.LSPClient, from, to string) error {
	j.logger.Debug("Java pre-switch: preparing to switch from %s to %s", from, to)
	
	// For JDTLS, we might want to clean up any in-progress compilation
	// This is optional and depends on the specific JDTLS version
	return nil
}

func (j *JavaWorkspaceSwitchStrategy) ExecuteSwitch(ctx context.Context, client transport.LSPClient, from, to string) error {
	j.logger.Info("Executing Java workspace switch: %s -> %s", from, to)
	
	// JDTLS fully supports workspace/didChangeWorkspaceFolders
	return j.sendWorkspaceFoldersNotification(ctx, client, from, to)
}

func (j *JavaWorkspaceSwitchStrategy) PostSwitch(client transport.LSPClient, from, to string) error {
	j.logger.Debug("Java post-switch: completed switch to %s", to)
	
	// JDTLS handles workspace switching internally, no additional steps needed
	return nil
}

func (j *JavaWorkspaceSwitchStrategy) ValidateSwitch(ctx context.Context, client transport.LSPClient, to string) error {
	j.logger.Debug("Validating Java workspace switch to %s", to)
	
	// Give JDTLS some time to process the workspace change
	select {
	case <-time.After(2 * time.Second):
		// Continue with validation
	case <-ctx.Done():
		return fmt.Errorf("validation cancelled: %w", ctx.Err())
	}
	
	// Try a simple workspace/symbol request to verify the workspace is loaded
	params := map[string]interface{}{
		"query": "class",
	}
	
	validationCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	
	_, err := client.SendRequest(validationCtx, "workspace/symbol", params)
	if err != nil {
		j.logger.Warn("Java workspace validation failed, but continuing: %v", err)
		// Don't fail the switch for validation errors, as the workspace might still be loading
	}
	
	return nil
}

// TypeScript Workspace Switch Strategy
type TypeScriptWorkspaceSwitchStrategy struct {
	baseWorkspaceSwitchStrategy
}

func NewTypeScriptWorkspaceSwitchStrategy(logger Logger) *TypeScriptWorkspaceSwitchStrategy {
	return &TypeScriptWorkspaceSwitchStrategy{
		baseWorkspaceSwitchStrategy: baseWorkspaceSwitchStrategy{logger: logger},
	}
}

func (t *TypeScriptWorkspaceSwitchStrategy) CanSwitch(from, to string) bool {
	// TypeScript language server supports workspace switching with some limitations
	return t.isTypeScriptProject(to)
}

func (t *TypeScriptWorkspaceSwitchStrategy) isTypeScriptProject(path string) bool {
	// Check for TypeScript/JavaScript project markers
	markers := []string{"tsconfig.json", "jsconfig.json", "package.json", "node_modules"}
	
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(path, marker)); err == nil {
			return true
		}
	}
	
	return false
}

func (t *TypeScriptWorkspaceSwitchStrategy) PreSwitch(client transport.LSPClient, from, to string) error {
	t.logger.Debug("TypeScript pre-switch: preparing to switch from %s to %s", from, to)
	
	// TypeScript server may benefit from clearing previous project state
	// Send a workspace/didChangeConfiguration to reset any cached configurations
	return nil
}

func (t *TypeScriptWorkspaceSwitchStrategy) ExecuteSwitch(ctx context.Context, client transport.LSPClient, from, to string) error {
	t.logger.Info("Executing TypeScript workspace switch: %s -> %s", from, to)
	
	// Try workspace/didChangeWorkspaceFolders first
	if err := t.sendWorkspaceFoldersNotification(ctx, client, from, to); err != nil {
		t.logger.Warn("TypeScript workspace/didChangeWorkspaceFolders failed, trying fallback: %v", err)
		
		// Fallback: Send workspace/didChangeConfiguration
		return t.sendConfigurationChangeNotification(ctx, client, to)
	}
	
	return nil
}

func (t *TypeScriptWorkspaceSwitchStrategy) sendConfigurationChangeNotification(ctx context.Context, client transport.LSPClient, to string) error {
	// Send a configuration change to help TypeScript server recognize the new workspace
	params := map[string]interface{}{
		"settings": map[string]interface{}{
			"typescript": map[string]interface{}{
				"preferences": map[string]interface{}{
					"includePackageJsonAutoImports": "on",
				},
			},
		},
	}
	
	return client.SendNotification(ctx, "workspace/didChangeConfiguration", params)
}

func (t *TypeScriptWorkspaceSwitchStrategy) PostSwitch(client transport.LSPClient, from, to string) error {
	t.logger.Debug("TypeScript post-switch: completed switch to %s", to)
	return nil
}

func (t *TypeScriptWorkspaceSwitchStrategy) ValidateSwitch(ctx context.Context, client transport.LSPClient, to string) error {
	t.logger.Debug("Validating TypeScript workspace switch to %s", to)
	
	// Give TypeScript server time to process the change
	select {
	case <-time.After(1 * time.Second):
		// Continue with validation
	case <-ctx.Done():
		return fmt.Errorf("validation cancelled: %w", ctx.Err())
	}
	
	// Try a workspace/symbol request
	params := map[string]interface{}{
		"query": "function",
	}
	
	validationCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	
	_, err := client.SendRequest(validationCtx, "workspace/symbol", params)
	if err != nil {
		t.logger.Warn("TypeScript workspace validation failed, but continuing: %v", err)
	}
	
	return nil
}

// Go (gopls) Workspace Switch Strategy
type GoWorkspaceSwitchStrategy struct {
	baseWorkspaceSwitchStrategy
}

func NewGoWorkspaceSwitchStrategy(logger Logger) *GoWorkspaceSwitchStrategy {
	return &GoWorkspaceSwitchStrategy{
		baseWorkspaceSwitchStrategy: baseWorkspaceSwitchStrategy{logger: logger},
	}
}

func (g *GoWorkspaceSwitchStrategy) CanSwitch(from, to string) bool {
	// gopls supports workspace switching for Go modules
	return g.isGoProject(to)
}

func (g *GoWorkspaceSwitchStrategy) isGoProject(path string) bool {
	// Check for Go project markers
	markers := []string{"go.mod", "go.sum", "main.go"}
	
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(path, marker)); err == nil {
			return true
		}
	}
	
	// Also check for any .go files in the directory
	if entries, err := os.ReadDir(path); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
				return true
			}
		}
	}
	
	return false
}

func (g *GoWorkspaceSwitchStrategy) PreSwitch(client transport.LSPClient, from, to string) error {
	g.logger.Debug("Go pre-switch: preparing to switch from %s to %s", from, to)
	return nil
}

func (g *GoWorkspaceSwitchStrategy) ExecuteSwitch(ctx context.Context, client transport.LSPClient, from, to string) error {
	g.logger.Info("Executing Go workspace switch: %s -> %s", from, to)
	
	// gopls supports workspace/didChangeWorkspaceFolders
	return g.sendWorkspaceFoldersNotification(ctx, client, from, to)
}

func (g *GoWorkspaceSwitchStrategy) PostSwitch(client transport.LSPClient, from, to string) error {
	g.logger.Debug("Go post-switch: completed switch to %s", to)
	return nil
}

func (g *GoWorkspaceSwitchStrategy) ValidateSwitch(ctx context.Context, client transport.LSPClient, to string) error {
	g.logger.Debug("Validating Go workspace switch to %s", to)
	
	// Give gopls time to process the module change
	select {
	case <-time.After(1 * time.Second):
		// Continue with validation
	case <-ctx.Done():
		return fmt.Errorf("validation cancelled: %w", ctx.Err())
	}
	
	// Try a workspace/symbol request to verify Go workspace is loaded
	params := map[string]interface{}{
		"query": "func",
	}
	
	validationCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	_, err := client.SendRequest(validationCtx, "workspace/symbol", params)
	if err != nil {
		g.logger.Warn("Go workspace validation failed, but continuing: %v", err)
	}
	
	return nil
}

// Python (pylsp) Workspace Switch Strategy
type PythonWorkspaceSwitchStrategy struct {
	baseWorkspaceSwitchStrategy
}

func NewPythonWorkspaceSwitchStrategy(logger Logger) *PythonWorkspaceSwitchStrategy {
	return &PythonWorkspaceSwitchStrategy{
		baseWorkspaceSwitchStrategy: baseWorkspaceSwitchStrategy{logger: logger},
	}
}

func (p *PythonWorkspaceSwitchStrategy) CanSwitch(from, to string) bool {
	// pylsp has limited workspace switching support
	return p.isPythonProject(to)
}

func (p *PythonWorkspaceSwitchStrategy) isPythonProject(path string) bool {
	// Check for Python project markers
	markers := []string{"setup.py", "pyproject.toml", "requirements.txt", "Pipfile", "__init__.py"}
	
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(path, marker)); err == nil {
			return true
		}
	}
	
	// Also check for any .py files in the directory
	if entries, err := os.ReadDir(path); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".py") {
				return true
			}
		}
	}
	
	return false
}

func (p *PythonWorkspaceSwitchStrategy) PreSwitch(client transport.LSPClient, from, to string) error {
	p.logger.Debug("Python pre-switch: preparing to switch from %s to %s", from, to)
	return nil
}

func (p *PythonWorkspaceSwitchStrategy) ExecuteSwitch(ctx context.Context, client transport.LSPClient, from, to string) error {
	p.logger.Info("Executing Python workspace switch: %s -> %s", from, to)
	
	// Try workspace/didChangeWorkspaceFolders first
	if err := p.sendWorkspaceFoldersNotification(ctx, client, from, to); err != nil {
		p.logger.Warn("Python workspace/didChangeWorkspaceFolders failed, trying fallback: %v", err)
		
		// Fallback: Send workspace/didChangeConfiguration with Python-specific settings
		return p.sendPythonConfigurationChange(ctx, client, to)
	}
	
	return nil
}

func (p *PythonWorkspaceSwitchStrategy) sendPythonConfigurationChange(ctx context.Context, client transport.LSPClient, to string) error {
	// Send configuration change with Python path information
	params := map[string]interface{}{
		"settings": map[string]interface{}{
			"pylsp": map[string]interface{}{
				"plugins": map[string]interface{}{
					"jedi": map[string]interface{}{
						"environment": to,
					},
				},
			},
		},
	}
	
	return client.SendNotification(ctx, "workspace/didChangeConfiguration", params)
}

func (p *PythonWorkspaceSwitchStrategy) PostSwitch(client transport.LSPClient, from, to string) error {
	p.logger.Debug("Python post-switch: completed switch to %s", to)
	return nil
}

func (p *PythonWorkspaceSwitchStrategy) ValidateSwitch(ctx context.Context, client transport.LSPClient, to string) error {
	p.logger.Debug("Validating Python workspace switch to %s", to)
	
	// Give pylsp time to process the change
	select {
	case <-time.After(1 * time.Second):
		// Continue with validation
	case <-ctx.Done():
		return fmt.Errorf("validation cancelled: %w", ctx.Err())
	}
	
	// Try a simple workspace/symbol request
	params := map[string]interface{}{
		"query": "def",
	}
	
	validationCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	_, err := client.SendRequest(validationCtx, "workspace/symbol", params)
	if err != nil {
		p.logger.Warn("Python workspace validation failed, but continuing: %v", err)
	}
	
	return nil
}