# Workspace Configuration Manager

The WorkspaceConfigManager provides workspace-specific configuration loading and management for LSP Gateway. It supports hierarchical configuration loading, workspace directory management, and automatic configuration generation based on detected project languages.

## Features

- **Configuration Hierarchy**: Merges global → workspace-specific → environment overrides
- **Workspace Detection**: Integrates with existing project detector for language detection  
- **Configuration Generation**: Auto-generates workspace config based on detected languages
- **Directory Management**: Creates and manages workspace directories (`~/.lspg/workspaces/{hash}/`)
- **Validation**: Ensures configuration validity and required fields

## Usage

### Basic Usage

```go
package main

import (
    "context"
    "lsp-gateway/internal/workspace"
    "lsp-gateway/internal/project"
)

func main() {
    // Create workspace config manager
    wcm := workspace.NewWorkspaceConfigManager()
    
    // Detect project structure
    detector := project.NewProjectDetector()
    projectContext, err := detector.DetectProject(context.Background(), "/path/to/project")
    if err != nil {
        panic(err)
    }
    
    // Generate workspace configuration
    err = wcm.GenerateWorkspaceConfig("/path/to/project", projectContext)
    if err != nil {
        panic(err)
    }
    
    // Load workspace configuration
    config, err := wcm.LoadWorkspaceConfig("/path/to/project")
    if err != nil {
        panic(err)
    }
    
    // Validate configuration
    err = wcm.ValidateWorkspaceConfig(config)
    if err != nil {
        panic(err)
    }
}
```

### Hierarchical Configuration Loading

```go
// Load with hierarchy: global → workspace → environment
globalConfigPath := "/path/to/global/config.yaml"
config, err := wcm.LoadWithHierarchy("/path/to/project", globalConfigPath)
if err != nil {
    panic(err)
}
```

### Custom Configuration Manager Options

```go
wcm := workspace.NewWorkspaceConfigManagerWithOptions(&workspace.WorkspaceConfigManagerOptions{
    BaseConfigDir:   "/custom/config/dir",
    Logger:          customLogger,
    ProjectDetector: customDetector,
})
```

## Configuration Structure

### WorkspaceConfig

```yaml
workspace:
  workspace_id: "ws_1640995200"  
  name: "example-project"
  root_path: "/path/to/project"
  project_type: "go"
  languages: ["go"]
  created_at: "2023-01-01T00:00:00Z"
  last_updated: "2023-01-01T00:00:00Z"
  version: "1.0.0"
  hash: "a1b2c3d4"

servers:
  gopls:
    name: "gopls"
    languages: ["go"]
    command: "gopls"
    args: ["serve"]
    transport: "stdio"
    root_markers: ["go.mod", "go.sum"]
    server_type: "workspace"

performance_config:
  enabled: true
  profile: "development"
  auto_tuning: true
  caching:
    enabled: true
    global_ttl: "30m"
    max_memory_usage_mb: 1024
    eviction_strategy: "lru"
  scip:
    enabled: true
    auto_refresh: true
    refresh_interval: "10m"
    fallback_to_lsp: true

cache:
  enabled: true
  global_ttl: "30m"
  max_memory_usage_mb: 512
  eviction_strategy: "lru"

logging:
  level: "info"
  output_file: "/workspace/logs/workspace.log"
  max_size_mb: 10
  max_backups: 3
  max_age_days: 7

directories:
  root: "/home/user/.lspg/workspaces/a1b2c3d4"
  config: "/home/user/.lspg/workspaces/a1b2c3d4"
  cache: "/home/user/.lspg/workspaces/a1b2c3d4/cache"
  logs: "/home/user/.lspg/workspaces/a1b2c3d4/logs"
  index: "/home/user/.lspg/workspaces/a1b2c3d4/index"
  state: "/home/user/.lspg/workspaces/a1b2c3d4/state.json"
```

## Directory Structure

The WorkspaceConfigManager creates the following directory structure:

```
~/.lspg/workspaces/{hash}/
├── workspace.yaml      # Main workspace configuration
├── state.json         # Workspace state file
├── cache/            # Cache directory
├── logs/             # Log files
│   └── workspace.log
└── index/            # SCIP index files
```

The workspace hash is generated from the absolute path of the workspace root, ensuring consistent directory names for the same project.

## Interface

### WorkspaceConfigManager Interface

```go
type WorkspaceConfigManager interface {
    LoadWorkspaceConfig(workspaceRoot string) (*WorkspaceConfig, error)
    GenerateWorkspaceConfig(workspaceRoot string, projectContext *project.ProjectContext) error
    GetWorkspaceConfigPath(workspaceRoot string) string
    GetWorkspaceDirectory(workspaceRoot string) string
    ValidateWorkspaceConfig(config *WorkspaceConfig) error
    LoadWithHierarchy(workspaceRoot string, globalConfigPath string) (*WorkspaceConfig, error)
    CreateWorkspaceDirectories(workspaceRoot string) error
    CleanupWorkspace(workspaceRoot string) error
}
```

## Integration

The WorkspaceConfigManager integrates with:

- **Project Detector** (`internal/project/detector.go`): For language and project type detection
- **Configuration System** (`internal/config/`): For base configuration structures and templates
- **Setup System** (`internal/setup/`): For logging and project setup workflows

## Error Handling

The implementation provides comprehensive error handling with descriptive error messages:

- Configuration file not found
- Invalid YAML syntax
- Validation failures
- Directory creation failures  
- Permission issues

## Supported Languages

The configuration generator supports templates for:

- Go (`gopls`)
- TypeScript/JavaScript (`typescript-language-server`)
- Python (`pylsp`)
- Java (`jdtls`)
- Rust (`rust-analyzer`)

Additional languages can be added by extending the `normalizeLanguageForTemplate` method and ensuring corresponding templates exist in the ConfigGenerator.