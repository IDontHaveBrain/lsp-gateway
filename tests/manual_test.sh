#!/bin/bash
# Manual test script for verifying findReferences works

echo "🔄 Rebuilding..."
make local

echo "🧹 Clearing cache..."
lsp-gateway cache clear

echo "📚 Indexing workspace..."
lsp-gateway cache index

echo "ℹ️ Checking cache info..."
lsp-gateway cache info

echo "🔍 Testing findReferences for NewLSPManager..."
echo '{"jsonrpc": "2.0", "method": "tools/call", "params": {"name": "findReferences", "arguments": {"pattern": "NewLSPManager", "filePattern": "**/*.go", "maxResults": 5}}, "id": 1}' | lsp-gateway mcp | tail -1 | python3 -m json.tool

echo "✅ Test complete"