#!/usr/bin/env node

const { spawn } = require('child_process');
const readline = require('readline');

function testMCPServer() {
    console.log('Starting MCP server test...');
    
    const server = spawn('lspg', ['mcp', '--config', 'config.yaml'], {
        stdio: ['pipe', 'pipe', 'pipe']
    });
    
    let responseCount = 0;
    
    // Setup readline to read JSON responses line by line
    const rl = readline.createInterface({
        input: server.stdout,
        crlfDelay: Infinity
    });
    
    rl.on('line', (line) => {
        if (line.trim().startsWith('{')) {
            console.log(`Response ${++responseCount}:`, line);
            
            // After initialization, call tools/list
            if (responseCount === 1) {
                setTimeout(() => {
                    console.log('Sending tools/list request...');
                    const toolsListRequest = JSON.stringify({
                        "jsonrpc": "2.0",
                        "id": 2,
                        "method": "tools/list",
                        "params": {}
                    }) + '\n';
                    server.stdin.write(toolsListRequest);
                }, 100);
            }
            
            // After tools/list, call a tool
            if (responseCount === 2) {
                setTimeout(() => {
                    console.log('Sending get_document_symbols tool call...');
                    const toolCallRequest = JSON.stringify({
                        "jsonrpc": "2.0",
                        "id": 3,
                        "method": "tools/call",
                        "params": {
                            "name": "get_document_symbols",
                            "arguments": {
                                "uri": "file:///home/skawn/work/lsp-gateway/test_validation.go"
                            }
                        }
                    }) + '\n';
                    server.stdin.write(toolCallRequest);
                }, 100);
            }
            
            // Close after tool response
            if (responseCount === 3) {
                setTimeout(() => {
                    console.log('Test completed, closing server...');
                    server.kill('SIGTERM');
                }, 1000);
            }
        }
    });
    
    server.stderr.on('data', (data) => {
        console.log('Server output:', data.toString());
    });
    
    server.on('close', (code) => {
        console.log(`Server process exited with code ${code}`);
    });
    
    // Send initial initialize request
    console.log('Sending initialize request...');
    const initRequest = JSON.stringify({
        "jsonrpc": "2.0",
        "id": 1,
        "method": "initialize",
        "params": {
            "processId": process.pid,
            "clientInfo": {
                "name": "test-client",
                "version": "1.0.0"
            },
            "capabilities": {}
        }
    }) + '\n';
    
    server.stdin.write(initRequest);
}

testMCPServer();