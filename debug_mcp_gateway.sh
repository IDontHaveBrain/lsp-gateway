#!/bin/bash

LOG_FILE="/home/skawn/work/lsp-gateway/mcp_gateway_debug_$(date +%s).log"
echo "=== MCP Gateway Debug Started at $(date) ===" >> "$LOG_FILE"
echo "Working directory: $(pwd)" >> "$LOG_FILE"
echo "Command: lspg mcp" >> "$LOG_FILE"
echo "" >> "$LOG_FILE"

# Run MCP server with debug output
exec lspg mcp 2>&1 | tee -a "$LOG_FILE"