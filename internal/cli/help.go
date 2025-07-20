package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var helpWorkflowsCmd = &cobra.Command{
	Use:   "workflows",
	Short: "Show common workflow examples and usage patterns",
	Long: `Display comprehensive workflow examples for common LSP Gateway usage patterns.

🚀 WORKFLOW EXAMPLES:
This command provides step-by-step guides for common LSP Gateway tasks,
from initial setup to advanced usage scenarios.

📋 AVAILABLE WORKFLOWS:
  • first-time     - Complete first-time setup and verification
  • development    - Daily development workflow
  • maintenance    - System maintenance and troubleshooting
  • integration    - IDE/editor and AI assistant integration
  • troubleshooting - Common problem resolution steps

💻 USAGE EXAMPLES:

  Show all workflows:
    lsp-gateway help workflows
    
  Show specific workflow:
    lsp-gateway help workflows first-time
    lsp-gateway help workflows development
    lsp-gateway help workflows troubleshooting`,
	Args: cobra.MaximumNArgs(1),
	RunE: runHelpWorkflows,
}

func runHelpWorkflows(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		showAllWorkflows()
		return nil
	}

	workflow := args[0]
	switch workflow {
	case "first-time":
		showFirstTimeWorkflow()
	case "development":
		showDevelopmentWorkflow()
	case "maintenance":
		showMaintenanceWorkflow()
	case "integration":
		showIntegrationWorkflow()
	case "troubleshooting":
		showTroubleshootingWorkflow()
	default:
		return fmt.Errorf("unknown workflow: %s (available: first-time, development, maintenance, integration, troubleshooting)", workflow)
	}

	return nil
}

func showAllWorkflows() {
	fmt.Printf(`
🌟 LSP Gateway Workflow Examples
================================

Choose a workflow to see detailed step-by-step instructions:

🚀 FIRST-TIME SETUP:
  lsp-gateway help workflows first-time
  Complete initial setup from scratch to working system

💻 DEVELOPMENT WORKFLOW:
  lsp-gateway help workflows development  
  Daily development tasks and common operations

🔧 MAINTENANCE:
  lsp-gateway help workflows maintenance
  System maintenance, updates, and health checks

🔗 INTEGRATION:
  lsp-gateway help workflows integration
  IDE integration and AI assistant setup

🚨 TROUBLESHOOTING:
  lsp-gateway help workflows troubleshooting
  Common problems and their solutions

For specific workflow details, use:
  lsp-gateway help workflows <workflow-name>
`)
}

func showFirstTimeWorkflow() {
	fmt.Printf(`
🚀 First-Time Setup Workflow
============================

Complete setup from fresh system to working LSP Gateway:

STEP 1: Quick Setup (Recommended)
----------------------------------
# Complete automated setup
lsp-gateway setup all

# Verify installation
lsp-gateway status

# Start server
lsp-gateway server


STEP 2: Manual Setup (Advanced)
--------------------------------
# Generate configuration
lsp-gateway config generate --auto-detect

# Install runtimes (if needed)
lsp-gateway install runtime go
lsp-gateway install runtime python
lsp-gateway install runtime nodejs

# Install language servers
lsp-gateway install servers

# Verify installation
lsp-gateway verify runtime all
lsp-gateway status servers

# Start server
lsp-gateway server --config config.yaml


STEP 3: Verification
--------------------
# Test HTTP Gateway
curl -X POST http://localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "textDocument/hover",
    "params": {
      "textDocument": {"uri": "file:///path/to/file.go"},
      "position": {"line": 0, "character": 0}
    }
  }'

# Test MCP Server (in separate terminal)
lsp-gateway mcp --config config.yaml


STEP 4: Integration
-------------------
# Setup shell completion
lsp-gateway completion bash > ~/.bash_completion.d/lsp-gateway
source ~/.bashrc

# Configure your IDE to use: http://localhost:8080/jsonrpc


TROUBLESHOOTING:
If setup fails, run: lsp-gateway diagnose
`)
}

func showDevelopmentWorkflow() {
	fmt.Printf(`
💻 Development Workflow
=======================

Daily development tasks and common operations:

DAILY STARTUP:
--------------
# Check system status
lsp-gateway status

# Start HTTP Gateway (for IDE integration)
lsp-gateway server &

# Start MCP Server (for AI assistants)
lsp-gateway mcp &


CONFIGURATION MANAGEMENT:
-------------------------
# View current configuration
lsp-gateway config show

# Validate configuration
lsp-gateway config validate

# Regenerate configuration (after installing new runtimes)
lsp-gateway config generate --auto-detect --overwrite


RUNTIME MANAGEMENT:
-------------------
# Check runtime status
lsp-gateway status runtimes

# Install missing runtime
lsp-gateway install runtime python

# Verify runtime installation
lsp-gateway verify runtime python


SERVER MANAGEMENT:
------------------
# Check language server status
lsp-gateway status servers

# Install missing servers
lsp-gateway install servers

# Verify specific server
lsp-gateway status server gopls


MAINTENANCE:
------------
# Run system diagnostics
lsp-gateway diagnose

# Update language servers (force reinstall)
lsp-gateway install servers --force

# Clean restart
pkill lsp-gateway
lsp-gateway server


PROJECT SWITCHING:
------------------
# Different config per project
cd /path/to/project
lsp-gateway config generate --auto-detect --output .lsp-gateway.yaml
lsp-gateway server --config .lsp-gateway.yaml
`)
}

func showMaintenanceWorkflow() {
	fmt.Printf(`
🔧 Maintenance Workflow
=======================

System maintenance, updates, and health checks:

REGULAR HEALTH CHECKS:
----------------------
# Full system diagnostic
lsp-gateway diagnose --verbose

# Check all component status
lsp-gateway status
lsp-gateway status runtimes
lsp-gateway status servers

# Verify installations
lsp-gateway verify runtime all


UPDATING COMPONENTS:
--------------------
# Update language servers
lsp-gateway install servers --force

# Update specific server
lsp-gateway install server gopls --force

# Regenerate config after updates
lsp-gateway config generate --auto-detect --overwrite


PERFORMANCE OPTIMIZATION:
-------------------------
# Check for issues
lsp-gateway diagnose

# Restart with clean configuration
lsp-gateway config generate --auto-detect --overwrite
lsp-gateway server --config config.yaml


BACKUP AND RESTORE:
-------------------
# Backup configuration
cp config.yaml config.yaml.backup

# Export current setup
lsp-gateway config show --json > setup-backup.json
lsp-gateway status --json > status-backup.json

# Restore from backup
cp config.yaml.backup config.yaml
lsp-gateway config validate


TROUBLESHOOTING WORKFLOW:
-------------------------
1. Run diagnostics:
   lsp-gateway diagnose --verbose

2. Check component status:
   lsp-gateway status runtimes
   lsp-gateway status servers

3. Verify installations:
   lsp-gateway verify runtime go
   lsp-gateway verify runtime python

4. Regenerate configuration:
   lsp-gateway config generate --auto-detect --overwrite

5. Test functionality:
   lsp-gateway server --config config.yaml


CLEANUP:
--------
# Remove old configurations
rm -f config.yaml.old config.yaml.backup

# Clean install (if needed)
lsp-gateway install runtime all --force
lsp-gateway install servers --force
`)
}

func showIntegrationWorkflow() {
	fmt.Printf(`
🔗 Integration Workflow
=======================

IDE integration and AI assistant setup:

IDE/EDITOR INTEGRATION:
-----------------------
# Start HTTP Gateway
lsp-gateway server --port 8080

# Configure your IDE LSP client to use:
# HTTP endpoint: http://localhost:8080/jsonrpc
# Supported methods:
#   - textDocument/definition
#   - textDocument/references  
#   - textDocument/documentSymbol
#   - workspace/symbol
#   - textDocument/hover


VISUAL STUDIO CODE:
-------------------
# Install LSP client extension
# Configure settings.json:
{
  "lsp-gateway.serverUrl": "http://localhost:8080/jsonrpc",
  "lsp-gateway.languages": ["go", "python", "typescript", "java"]
}


VIM/NEOVIM:
-----------
# Using vim-lsp or coc.nvim
let g:lsp_settings = {
\   'lsp-gateway': {
\     'cmd': ['curl', '-X', 'POST', 'http://localhost:8080/jsonrpc'],
\     'allowlist': ['go', 'python', 'typescript', 'java']
\   }
\ }


AI ASSISTANT INTEGRATION (MCP):
-------------------------------
# Start MCP Server
lsp-gateway mcp --config config.yaml

# For Claude Desktop, add to config:
{
  "mcpServers": {
    "lsp-gateway": {
      "command": "lsp-gateway",
      "args": ["mcp", "--config", "config.yaml"]
    }
  }
}

# For other AI assistants using MCP:
lsp-gateway mcp --transport http --port 3000


HTTP API INTEGRATION:
---------------------
# Test connection
curl -X POST http://localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "workspace/symbol",
    "params": {"query": "function"}
  }'

# Get file symbols
curl -X POST http://localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0", 
    "id": 2,
    "method": "textDocument/documentSymbol",
    "params": {
      "textDocument": {"uri": "file:///path/to/file.go"}
    }
  }'


CUSTOM INTEGRATION:
-------------------
# Use as library in your application
import "lsp-gateway/internal/gateway"

cfg := &config.GatewayConfig{...}
gw, err := gateway.NewGateway(cfg)
gw.Start(ctx)
`)
}

func showTroubleshootingWorkflow() {
	fmt.Printf(`
🚨 Troubleshooting Workflow
===========================

Common problems and their systematic resolution:

STEP 1: IDENTIFY THE PROBLEM
----------------------------
# Run comprehensive diagnostics
lsp-gateway diagnose --verbose --all

# Check system status
lsp-gateway status --verbose
lsp-gateway version


STEP 2: COMMON ISSUES & SOLUTIONS
---------------------------------

"Configuration file not found":
  → lsp-gateway config generate
  → lsp-gateway setup all

"Port already in use":
  → lsof -i :8080
  → lsp-gateway server --port 8081
  → sudo kill $(lsof -t -i:8080)

"Language server not found":
  → lsp-gateway status servers
  → lsp-gateway install servers
  → lsp-gateway verify runtime go

"Server fails to start":
  → lsp-gateway config validate
  → lsp-gateway diagnose
  → lsp-gateway status runtimes

"Runtime not detected":
  → which go python node java
  → lsp-gateway install runtime go
  → echo $PATH

"Permission denied":
  → ls -la config.yaml
  → chmod 644 config.yaml
  → lsp-gateway server --port 8080

"Server not responding":
  → curl http://localhost:8080/jsonrpc
  → lsp-gateway server --config config.yaml --verbose
  → Check firewall settings


STEP 3: SYSTEMATIC DEBUGGING
-----------------------------
1. Check prerequisites:
   lsp-gateway status runtimes

2. Verify configuration:
   lsp-gateway config validate

3. Test each component:
   lsp-gateway verify runtime go
   lsp-gateway verify runtime python

4. Check network connectivity:
   curl -I http://localhost:8080

5. Review logs:
   lsp-gateway server --verbose

6. Reset to known good state:
   lsp-gateway setup all --force


STEP 4: ADVANCED TROUBLESHOOTING
---------------------------------
# Check process status
ps aux | grep lsp-gateway

# Check listening ports
netstat -tulpn | grep :8080

# Check file permissions
ls -la config.yaml ~/.lsp-gateway/

# Check environment variables
env | grep -E "(PATH|GO|PYTHON|NODE|JAVA)"

# Test individual language servers
gopls version
pylsp --help
typescript-language-server --version
jdtls --version


STEP 5: RESET AND REINSTALL
----------------------------
# Complete reset (if all else fails)
rm -f config.yaml
lsp-gateway install runtime all --force
lsp-gateway install servers --force
lsp-gateway setup all
lsp-gateway server


GET HELP:
---------
If problems persist:
1. Include output of: lsp-gateway version
2. Include output of: lsp-gateway diagnose --verbose
3. Include output of: lsp-gateway status --verbose
4. Describe exact error messages and steps to reproduce
`)
}

func init() {
	rootCmd.AddCommand(helpWorkflowsCmd)
}
