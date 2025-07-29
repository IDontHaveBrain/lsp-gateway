#!/bin/bash

# MCP Debug Wrapper Script
# Captures all logs from MCP server to a file

LOG_FILE="/home/skawn/work/lsp-gateway/mcp_claude_debug_$(date +%s).log"

echo "=== MCP Debug Wrapper Started at $(date) ===" >> "$LOG_FILE"
echo "Working Directory: $(pwd)" >> "$LOG_FILE"
echo "Command: lspg mcp --config config.yaml" >> "$LOG_FILE"
echo "Environment Variables:" >> "$LOG_FILE"
env | grep -E "(CLAUDE|PATH|HOME)" >> "$LOG_FILE"
echo "=====================================" >> "$LOG_FILE"

# Execute the actual MCP server and capture all output
exec lspg mcp --config config.yaml 2>&1 | tee -a "$LOG_FILE"