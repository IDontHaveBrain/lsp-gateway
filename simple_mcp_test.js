#!/usr/bin/env node

const { spawn } = require('child_process');
const fs = require('fs');

async function testMCPSimple() {
    console.log('🚀 Starting simple MCP test...');
    
    const mcpProcess = spawn('lspg', ['mcp', '--config', 'config.yaml'], {
        stdio: ['pipe', 'pipe', 'pipe'],
        cwd: process.cwd()
    });

    let responseReceived = false;
    let timeout;
    
    // Set timeout to ensure we don't wait infinitely
    timeout = setTimeout(() => {
        if (!responseReceived) {
            console.log('❌ TIMEOUT: No response within 10 seconds - infinite waiting detected!');
            mcpProcess.kill('SIGTERM');
            process.exit(1);
        }
    }, 10000);

    mcpProcess.stdout.on('data', (data) => {
        const lines = data.toString().split('\n').filter(line => line.trim());
        
        for (const line of lines) {
            try {
                const message = JSON.parse(line);
                if (message.result) {
                    console.log('✅ SUCCESS: Received MCP response, no infinite waiting!');
                    console.log(`📊 Response type: ${message.result.tools ? 'tools list' : 'other'}`);
                    if (message.result.tools) {
                        console.log(`📊 Tools count: ${message.result.tools.length}`);
                    }
                    responseReceived = true;
                    clearTimeout(timeout);
                    mcpProcess.kill('SIGTERM');
                    return;
                }
            } catch (e) {
                // Ignore non-JSON lines
            }
        }
    });

    mcpProcess.stderr.on('data', (data) => {
        const output = data.toString();
        if (output.includes('MCP server is running')) {
            console.log('✅ MCP server started successfully');
            
            // Wait a moment then send initialize request
            setTimeout(() => {
                const initMsg = {
                    jsonrpc: "2.0",
                    id: 1,
                    method: "initialize",
                    params: {
                        protocolVersion: "2024-11-05",
                        capabilities: { roots: { listChanged: true }, sampling: {} },
                        clientInfo: { name: "simple-test", version: "1.0.0" }
                    }
                };
                
                console.log('📤 Sending initialize request...');
                mcpProcess.stdin.write(JSON.stringify(initMsg) + '\n');
                
                // Then send tools/list request
                setTimeout(() => {
                    const toolsMsg = {
                        jsonrpc: "2.0",
                        id: 2,
                        method: "tools/list",
                        params: {}
                    };
                    
                    console.log('📤 Sending tools/list request...');
                    mcpProcess.stdin.write(JSON.stringify(toolsMsg) + '\n');
                }, 1000);
            }, 2000);
        }
    });

    mcpProcess.on('exit', (code) => {
        if (responseReceived) {
            console.log('🎉 Test completed successfully - no infinite waiting detected!');
            process.exit(0);
        } else {
            console.log('❌ Test failed or timed out');
            process.exit(1);
        }
    });
}

testMCPSimple().catch(console.error);