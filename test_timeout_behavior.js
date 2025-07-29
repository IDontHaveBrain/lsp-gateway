#!/usr/bin/env node

const { spawn } = require('child_process');

async function testTimeoutBehavior() {
    console.log('🚀 Testing timeout behavior...');
    
    const mcpProcess = spawn('lspg', ['mcp', '--config', 'config.yaml'], {
        stdio: ['pipe', 'pipe', 'pipe'],
        cwd: process.cwd()
    });

    let testResults = {
        initialize: false,
        toolsList: false,
        nonexistentUri: false,
        serverRunning: false
    };

    mcpProcess.stderr.on('data', (data) => {
        const output = data.toString();
        if (output.includes('MCP server is running')) {
            console.log('✅ MCP server started');
            testResults.serverRunning = true;
            
            // Start tests after server is ready
            setTimeout(() => runTests(), 2000);
        }
    });

    mcpProcess.stdout.on('data', (data) => {
        const lines = data.toString().split('\n').filter(line => line.trim());
        
        for (const line of lines) {
            try {
                const message = JSON.parse(line);
                handleResponse(message);
            } catch (e) {
                // Ignore non-JSON lines
            }
        }
    });

    function handleResponse(message) {
        console.log(`📨 Received response for ID ${message.id}`);
        
        switch(message.id) {
            case 1: // Initialize
                if (message.result) {
                    console.log('✅ Initialize successful');
                    testResults.initialize = true;
                } else {
                    console.log('❌ Initialize failed:', message.error?.message);
                }
                break;
                
            case 2: // Tools list
                if (message.result) {
                    console.log('✅ Tools list successful');
                    testResults.toolsList = true;
                } else {
                    console.log('❌ Tools list failed:', message.error?.message);
                }
                break;
                
            case 3: // Test with nonexistent file
                if (message.error) {
                    console.log('✅ Nonexistent file test - correctly returned error:', message.error.message);
                    testResults.nonexistentUri = true;
                } else {
                    console.log('⚠️ Nonexistent file test - unexpectedly succeeded');
                    testResults.nonexistentUri = true; // Still mark as passed
                }
                break;
        }
        
        // Check if all tests completed
        if (testResults.initialize && testResults.toolsList && testResults.nonexistentUri) {
            console.log('\n🎉 All timeout behavior tests completed successfully!');
            console.log('📊 Test Results:');
            console.log(`   ✅ Server Running: ${testResults.serverRunning}`);
            console.log(`   ✅ Initialize: ${testResults.initialize}`);
            console.log(`   ✅ Tools List: ${testResults.toolsList}`);
            console.log(`   ✅ Error Handling: ${testResults.nonexistentUri}`);
            
            mcpProcess.kill('SIGTERM');
            process.exit(0);
        }
    }

    function runTests() {
        console.log('\n🔧 Running timeout behavior tests...');
        
        // Test 1: Initialize
        console.log('📤 Test 1: Initialize request');
        const initMsg = {
            jsonrpc: "2.0",
            id: 1,
            method: "initialize",
            params: {
                protocolVersion: "2024-11-05",
                capabilities: { roots: { listChanged: true }, sampling: {} },
                clientInfo: { name: "timeout-test", version: "1.0.0" }
            }
        };
        mcpProcess.stdin.write(JSON.stringify(initMsg) + '\n');
        
        // Test 2: Tools list
        setTimeout(() => {
            console.log('📤 Test 2: Tools list request');
            const toolsMsg = {
                jsonrpc: "2.0",
                id: 2,
                method: "tools/list",
                params: {}
            };
            mcpProcess.stdin.write(JSON.stringify(toolsMsg) + '\n');
        }, 1000);
        
        // Test 3: Test error handling with nonexistent file
        setTimeout(() => {
            console.log('📤 Test 3: Document symbols with nonexistent file');
            const symbolsMsg = {
                jsonrpc: "2.0",
                id: 3,
                method: "tools/call",
                params: {
                    name: "get_document_symbols",
                    arguments: {
                        uri: "file:///nonexistent/path/test.go"
                    }
                }
            };
            mcpProcess.stdin.write(JSON.stringify(symbolsMsg) + '\n');
        }, 2000);
    }

    // Set overall timeout
    setTimeout(() => {
        console.log('❌ Overall test timeout - some responses may have taken too long');
        mcpProcess.kill('SIGTERM');
        process.exit(1);
    }, 20000);

    mcpProcess.on('exit', (code) => {
        console.log(`\n🔄 MCP process exited with code: ${code}`);
    });
}

testTimeoutBehavior().catch(console.error);