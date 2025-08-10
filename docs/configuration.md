# Configuration Guide

LSP Gateway provides flexible configuration for Language Server Protocol integration with intelligent auto-detection and comprehensive caching capabilities.

## Overview

LSP Gateway supports three configuration methods:
- **Auto-detection** (recommended): Automatically detects languages and available LSP servers
- **YAML configuration**: Complete control over server settings and cache configuration
- **Template-based**: Use provided templates as starting points

The configuration system features:
- **6 supported languages**: Go, Python, JavaScript, TypeScript, Java, Rust
- **SCIP cache integration**: Sub-millisecond symbol lookups (enabled by default)
- **Security validation**: Whitelist-based LSP server command validation
- **Project isolation**: Auto-generated project-specific cache paths

## Quick Start

### Auto-Detection (MCP Mode)
```bash
lsp-gateway mcp                    # Auto-detects languages in current directory
lsp-gateway status                 # Check which LSP servers are available
```

### Basic YAML Configuration
```yaml
servers:
  go:
    command: "gopls"
    args: ["serve"]

cache:
  enabled: true
  storage_path: ".lsp-gateway/scip-cache"
  max_memory_mb: 512
  ttl_hours: 24
```

## Configuration Reference

### Server Configuration

Configure LSP servers for each supported language:

```yaml
servers:
  go:
    command: "gopls"                    # Required: LSP server executable
    args: ["serve"]                     # Optional: Command arguments
    working_dir: ""                     # Optional: Working directory
    initialization_options: {}          # Optional: LSP initialization options
```

#### Supported Languages and Default Servers

| Language   | Command                        | Default Args     | Purpose |
|------------|--------------------------------|------------------|---------|
| `go`       | `gopls`                        | `["serve"]`      | Go language server |
| `python`   | `pylsp`                        | `[]`             | Python LSP server |
| `javascript` | `typescript-language-server` | `["--stdio"]`    | JavaScript support |
| `typescript` | `typescript-language-server` | `["--stdio"]`    | TypeScript support |
| `java`     | `jdtls`                        | `[]`             | Java language server |
| `rust`     | `rust-analyzer`                | `[]`             | Rust language server |

#### Server Configuration Fields

- **`command`** (required): LSP server executable name or path
  - Supports `~` expansion for home directory
  - Must pass security validation (whitelist-based)
- **`args`** (optional): Command-line arguments for the LSP server
- **`working_dir`** (optional): Working directory for the LSP server process
  - Supports `~` expansion
- **`initialization_options`** (optional): LSP-specific initialization parameters

### SCIP Cache Configuration

SCIP cache provides sub-millisecond symbol lookups optimized for AI assistant usage:

```yaml
cache:
  enabled: true                           # Default: true
  storage_path: ".lsp-gateway/scip-cache" # Default: ~/.lsp-gateway/scip-cache
  max_memory_mb: 512                      # Default: 512 MB
  ttl_hours: 24                           # Default: 24 hours
  languages: ["*"]                        # Default: "*" (all supported languages)
  background_index: true                  # Default: true
  health_check_minutes: 5                 # Default: 5 minutes
  eviction_policy: "lru"                  # Default: "lru" (or "simple")
  disk_cache: true                        # Default: true (persistent cache)
```

#### Language Configuration Examples

```yaml
# Cache all supported languages (recommended)
languages: ["*"]

# Cache specific languages only
languages: ["go", "python"]

# Cache single language
languages: ["typescript"]
```

#### Cache Configuration Fields

- **`enabled`** (boolean): Enable/disable SCIP cache
  - **Default**: `true`
  - Always enabled for HTTP gateway and MCP server modes

- **`storage_path`** (string): Cache storage directory
  - **Default**: `~/.lsp-gateway/scip-cache`
  - Auto-generates project-specific paths using MD5 hash
  - Format: `{base-path}/{project-name}-{hash8}`

- **`max_memory_mb`** (integer): Memory limit in megabytes
  - **Default**: `512` MB
  - **MCP mode**: Always uses 512 MB
  - **Validation**: Must be > 0, warns if > 50% of system memory

- **`ttl_hours`** (integer): Cache time-to-live in hours
  - **Default**: `24` hours
  - **MCP mode**: Overridden to 1 hour
  - **Validation**: Must be ≥ 1 hour

- **`languages`** (array): Languages to cache
  - **Default**: `["*"]` (wildcard for all supported languages)
  - **Wildcard**: Use `["*"]` to include all supported languages
  - **Explicit**: Specify individual languages: `["go", "python", "javascript", "typescript", "java", "rust"]`
  - **Validation**: Only supported languages allowed

- **`background_index`** (boolean): Enable background cache optimization
  - **Default**: `true`
  - **MCP mode**: Always enabled

- **`health_check_minutes`** (integer): Health monitoring frequency in minutes
  - **Default**: `5` minutes
  - **MCP mode**: Overridden to 2 minutes
  - **Validation**: Must be ≥ 1 minute

- **`eviction_policy`** (string): Cache eviction strategy
  - **Default**: `"lru"` (Least Recently Used)
  - **Options**: `"lru"` or `"simple"`

- **`disk_cache`** (boolean): Enable persistent disk cache
  - **Default**: `true`
  - Persists cache to JSON files for faster startup

#### Cache Architecture

- **Three-tier storage**: Hot memory cache, warm LRU cache, cold disk storage
- **Project isolation**: Auto-generated unique cache paths prevent conflicts
- **Smart invalidation**: File system monitoring with cascade dependency updates
- **Performance optimization**: 90%+ cache hit rates for typical LLM usage patterns

### MCP Configuration

MCP (Model Context Protocol) server mode automatically runs in enhanced mode with optimized settings:

#### MCP Mode Behavior

- Always runs in **enhanced mode** (AI-optimized)
- Automatically overrides cache settings:
  - `max_memory_mb`: 512 MB
  - `ttl_hours`: 1 hour (for stable symbols)
  - `health_check_minutes`: 2 minutes
  - `languages`: All 6 supported languages
  - `background_index`: Always enabled

## Configuration Templates

Use provided templates as starting points:

### Multi-Language Template
```bash
cp config-templates/multi-language-template.yaml config.yaml
```

Complete setup with all 6 supported languages and comprehensive cache configuration.

### Single-Language Template
```bash
cp config-templates/patterns/single-language.yaml config.yaml
```

Focused single-language configuration with examples for each supported language.

## Usage Examples

### HTTP Gateway Integration

Start HTTP gateway with configuration:
```bash
lsp-gateway server --config config.yaml
```

Test with curl:
```bash
curl -X POST http://localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "textDocument/hover",
    "params": {
      "textDocument": {"uri": "file:///path/to/file.go"},
      "position": {"line": 10, "character": 5}
    }
  }'
```

Health check:
```bash
curl http://localhost:8080/health
```

### MCP Server Integration

Auto-detect languages and start MCP server:
```bash
lsp-gateway mcp                           # Auto-detection mode
lsp-gateway mcp --config config.yaml     # Explicit configuration
```

Available MCP tools:
- `goto_definition`: Navigate to symbol definition
- `find_references`: Find all symbol references
- `get_hover_info`: Show documentation on hover
- `get_document_symbols`: Document outline/symbols
- `search_workspace_symbols`: Workspace-wide symbol search
- `get_completion`: Code completion suggestions

### Claude Desktop Integration

Add to Claude Desktop configuration (`~/.claude/claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "lsp-gateway": {
      "command": "lsp-gateway",
      "args": ["mcp"]
    }
  }
}
```

### IDE Integration

Point your IDE's LSP client to the HTTP gateway:
```
LSP Server URL: http://localhost:8080/jsonrpc
```

After starting: `lsp-gateway server --config config.yaml`

## Auto-Detection System

### Language Detection Rules

LSP Gateway automatically detects languages based on:

#### File Extensions
- **Go**: `.go`
- **Python**: `.py`
- **JavaScript**: `.js`, `.jsx`
- **TypeScript**: `.ts`, `.tsx`
- **Java**: `.java`
- **Rust**: `.rs`

#### Project Files (Higher Confidence)
- **Go**: `go.mod`, `go.sum`
- **Python**: `setup.py`, `requirements.txt`, `pyproject.toml`
- **JavaScript/TypeScript**: `package.json`, `tsconfig.json`
- **Java**: `pom.xml`, `build.gradle`
 - **Rust**: `Cargo.toml`, `Cargo.lock`

#### Detection Priority
1. **Go** (priority 4)
2. **TypeScript** (priority 3)
3. **JavaScript** (priority 2)
4. **Python** (priority 1)
5. **Java** (priority 0)

### LSP Server Availability

Auto-detection only includes languages with:
- Available LSP server executables (checked via `exec.LookPath()`)
- Security validation passing (whitelist-based)

Check availability:
```bash
lsp-gateway status    # Shows available vs configured servers
```

## Configuration Loading Hierarchy

LSP Gateway loads configuration in this order:

1. **Explicit config**: `--config path/to/config.yaml`
2. **Default config file**: `~/.lsp-gateway/config.yaml` (if exists)
3. **Auto-detection**: Language detection + LSP server availability
4. **Default fallback**: All supported languages with defaults

When config loading fails, auto-detection with project-specific cache paths is used.

## Cache Management Commands

Monitor and manage SCIP cache:

```bash
lsp-gateway cache info       # Cache status and statistics
lsp-gateway cache clear      # Clear cache contents
lsp-gateway cache index      # Proactively index files
```

Cache provides:
- **Sub-millisecond responses** (< 1ms vs 10-100ms LSP calls)
- **90%+ cache hit rates** for typical usage patterns
- **Intelligent invalidation** based on file changes
- **Performance monitoring** with detailed metrics

## Troubleshooting

### Configuration Issues

**Check configuration validity:**
```bash
lsp-gateway status           # Verify server availability
lsp-gateway test            # Test LSP connections
```

**Common problems:**
- **Missing LSP servers**: Install required language servers
- **Invalid commands**: Check security validation and paths
- **Path issues**: Use absolute paths or `~` for home directory
- **Memory limits**: Adjust `max_memory_mb` if experiencing issues

### Cache Issues

**Cache diagnostics:**
```bash
lsp-gateway cache info       # Cache information and statistics
```

**Cache problems:**
- **Low hit rates**: Check `ttl_hours` settings and file modification patterns
- **Memory issues**: Adjust `max_memory_mb` limit
- **Storage issues**: Verify `storage_path` permissions and disk space
- **Performance**: Enable `background_index` for optimization

### LSP Server Issues

**Server availability:**
```bash
which gopls                  # Check if server is installed
lsp-gateway status          # Show all server statuses
```

**Connection problems:**
- **Command not found**: Install missing LSP servers
- **Security validation**: Ensure commands are whitelisted
- **Working directory**: Check `working_dir` exists and is accessible
- **Initialization**: Verify `initialization_options` format

### Port Conflicts

**Use different port:**
```bash
lsp-gateway server --port 8081    # HTTP gateway on port 8081
```

**Check port usage:**
```bash
lsof -i :8080                     # Check what's using port 8080
```

## Security Considerations

LSP Gateway implements security best practices:

- **Command validation**: All LSP server commands validated through whitelist
- **Path restrictions**: Limited to safe executable paths
- **No arbitrary execution**: Prevents shell injection and malicious commands
- **Project isolation**: Cache paths prevent cross-project interference

Whitelist validation prevents execution of:
- Shell commands or scripts
- Paths outside expected LSP server locations
- Commands with suspicious arguments or paths

## Performance Optimization

### Cache Optimization
- **Enable background indexing**: `background_index: true`
- **Tune memory limits**: Balance between performance and system resources
- **Adjust TTL**: Match your development workflow patterns
- **Monitor metrics**: Use cache health and stats commands

### Multi-Language Projects
- **Selective language enablement**: Only enable languages you use
- **Project-specific configs**: Use different configs for different projects
- **Memory allocation**: Increase `max_memory_mb` for large projects

## Migration Guide

### From Previous Versions
1. **Check cache settings**: Cache is now enabled by default
2. **Update server commands**: Verify command paths and arguments
3. **Review MCP mode**: Choose between `lsp` and `enhanced` modes
4. **Test configuration**: Use `lsp-gateway status` to verify setup

### From Auto-Detection to Explicit Config
To create an explicit configuration file, start with the templates in `config-templates/` directory and customize based on your project's needs.
