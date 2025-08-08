#!/bin/bash
# Test script to check if findReferences works

echo "=== Testing findReferences ==="
echo ""

echo "1. Clearing cache..."
node bin/lsp-gateway.js cache clear >/dev/null 2>&1

echo "2. Indexing workspace..."
node bin/lsp-gateway.js cache index >/dev/null 2>&1 || { echo "Indexing failed"; exit 1; }

echo "3. Checking cache info..."
node bin/lsp-gateway.js cache info 2>&1 | grep -A 5 "Index Statistics"

echo ""
echo "4. Testing findReferences via MCP..."
RESULT=$(echo '{"jsonrpc": "2.0", "method": "tools/call", "params": {"name": "findReferences", "arguments": {"pattern": "NewLSPManager", "filePattern": "**/*.go", "maxResults": 5}}, "id": 1}' | node bin/lsp-gateway.js mcp 2>/dev/null | tail -1)

echo "Result:"
echo "$RESULT" | python3 -c "
import json, sys
data = json.load(sys.stdin)
refs = json.loads(data['result']['content'][0]['text'])['references']
print(f'Found {len(refs)} references')
if len(refs) > 0:
    print('✅ SUCCESS - References found!')
    for i, ref in enumerate(refs[:3]):
        print(f'  {i+1}. {ref}')
else:
    print('❌ FAILURE - No references found')
"
