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
    lspg help workflows
    
  Show specific workflow:
    lspg help workflows first-time
    lspg help workflows development
    lspg help workflows troubleshooting`,
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
  lspg help workflows first-time
  Complete initial setup from scratch to working system

💻 DEVELOPMENT WORKFLOW:
  lspg help workflows development  
  Daily development tasks and common operations

🔧 MAINTENANCE:
  lspg help workflows maintenance
  System maintenance, updates, and health checks

🔗 INTEGRATION:
  lspg help workflows integration
  IDE integration and AI assistant setup

🚨 TROUBLESHOOTING:
  lspg help workflows troubleshooting
  Common problems and their solutions

For specific workflow details, use:
  lspg help workflows <workflow-name>
`)
}

func showFirstTimeWorkflow() {
	fmt.Printf(`
🚀 First-Time Setup Workflow
============================

Complete setup from fresh system to working LSP Gateway:

STEP 1: Quick Setup (Recommended)
----------------------------------
# Complete automated setup (recommended for first-time users)
lspg setup all

# Verify installation
lspg status

# Start server
lspg server


STEP 2: Manual Setup (Alternative)
-----------------------------------
# For users who prefer step-by-step manual control:

# Generate configuration
lspg config generate

# Install runtimes (if needed)
lspg install runtime go
lspg install runtime python
lspg install runtime nodejs

# Install language servers
lspg install servers

# Verify installation
lspg verify runtime all
lspg status servers

# Start server
lspg server --config config.yaml


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
lspg mcp --config config.yaml


STEP 4: Integration
-------------------
# Setup shell completion
lspg completion bash > ~/.bash_completion.d/lsp-gateway
source ~/.bashrc

# Configure your IDE to use: http://localhost:8080/jsonrpc


TROUBLESHOOTING:
If setup fails:
1. Try automated setup: lspg setup all --force
2. Run diagnostics: lspg diagnose
3. Check individual steps with manual setup (STEP 2)
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
lspg status

# Start HTTP Gateway (for IDE integration)
lspg server &

# Start MCP Server (for AI assistants)
lspg mcp &


CONFIGURATION MANAGEMENT:
-------------------------
# View current configuration
lspg config show

# Validate configuration
lspg config validate

# Regenerate configuration (after installing new runtimes)
lspg config generate --overwrite


RUNTIME MANAGEMENT:
-------------------
# Check runtime status
lspg status runtimes

# Install missing runtime
lspg install runtime python

# Verify runtime installation
lspg verify runtime python


SERVER MANAGEMENT:
------------------
# Check language server status
lspg status servers

# Install missing servers
lspg install servers

# Verify specific server
lspg status server gopls


MAINTENANCE:
------------
# Run system diagnostics
lspg diagnose

# Update language servers (force reinstall)
lspg install servers --force

# Clean restart
pkill lspg
lspg server


PROJECT SWITCHING:
------------------
# Different config per project
cd /path/to/project
lspg config generate --output .lsp-gateway.yaml
lspg server --config .lsp-gateway.yaml
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
lspg diagnose --verbose

# Check all component status
lspg status
lspg status runtimes
lspg status servers

# Verify installations
lspg verify runtime all


UPDATING COMPONENTS:
--------------------
# Update language servers
lspg install servers --force

# Update specific server
lspg install server gopls --force

# Regenerate config after updates
lspg config generate --overwrite


PERFORMANCE OPTIMIZATION:
-------------------------
# Check for issues
lspg diagnose

# Restart with clean configuration
lspg config generate --overwrite
lspg server --config config.yaml


BACKUP AND RESTORE:
-------------------
# Backup configuration
cp config.yaml config.yaml.backup

# Export current setup
lspg config show --json > setup-backup.json
lspg status --json > status-backup.json

# Restore from backup
cp config.yaml.backup config.yaml
lspg config validate


TROUBLESHOOTING WORKFLOW:
-------------------------
1. Run diagnostics:
   lspg diagnose --verbose

2. Check component status:
   lspg status runtimes
   lspg status servers

3. Verify installations:
   lspg verify runtime go
   lspg verify runtime python

4. Regenerate configuration:
   lspg config generate --overwrite

5. Test functionality:
   lspg server --config config.yaml


CLEANUP:
--------
# Remove old configurations
rm -f config.yaml.old config.yaml.backup

# Clean install (if needed)
lspg install runtime all --force
lspg install servers --force
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
lspg server --port 8080

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
lspg mcp --config config.yaml

# For Claude Desktop, add to config:
{
  "mcpServers": {
    "lsp-gateway": {
      "command": "lspg",
      "args": ["mcp", "--config", "config.yaml"]
    }
  }
}

# For other AI assistants using MCP:
lspg mcp --transport http --port 3000


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
lspg diagnose --verbose --all

# Check system status
lspg status --verbose
lspg version


STEP 2: COMMON ISSUES & SOLUTIONS
---------------------------------

"Configuration file not found":
  → lspg setup all
  → lspg config generate (if manual setup preferred)
  → lspg diagnose

"Port already in use":
  → lsof -i :8080
  → lspg server --port 8081
  → sudo kill $(lsof -t -i:8080)

"Language server not found":
  → lspg setup all
  → lspg status servers
  → lspg install servers (if manual setup preferred)

"Server fails to start":
  → lspg setup all --force
  → lspg config validate
  → lspg diagnose

"Runtime not detected":
  → lspg setup all
  → which go python node java
  → lspg install runtime go (if manual setup preferred)

"Permission denied":
  → ls -la config.yaml
  → chmod 644 config.yaml
  → lspg server --port 8080

"Server not responding":
  → curl http://localhost:8080/jsonrpc
  → lspg server --config config.yaml --verbose
  → Check firewall settings


STEP 3: SYSTEMATIC DEBUGGING
-----------------------------
1. Check prerequisites:
   lspg status runtimes

2. Verify configuration:
   lspg config validate

3. Test each component:
   lspg verify runtime go
   lspg verify runtime python

4. Check network connectivity:
   curl -I http://localhost:8080

5. Review logs:
   lspg server --verbose

6. Reset to known good state:
   lspg install runtime all --force
   lspg install servers --force


STEP 4: ADVANCED TROUBLESHOOTING
---------------------------------
# Check process status
ps aux | grep lspg

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
lspg setup all --force
lspg server


GET HELP:
---------
If problems persist:
1. Include output of: lspg version
2. Include output of: lspg diagnose --verbose
3. Include output of: lspg status --verbose
4. Describe exact error messages and steps to reproduce
`)
}

func init() {
	rootCmd.AddCommand(helpWorkflowsCmd)
}
